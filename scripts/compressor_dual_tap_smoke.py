#!/usr/bin/env python3
"""Run the COM-2 real-kernel paired-evidence acceptance smoke.

The script is read-only with respect to compressor parameters. It imports the
selected fixture only when the target track has no clip, asks the existing
compressor recognizer for the live topology generation, and then invokes the
kernel dual-tap command. Raw trace data remains in the kernel artifact store.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import time
import urllib.request
from pathlib import Path
from typing import Any

import zmq


FORBIDDEN_RECEIPT_KEYS = {
    "shared_memory", "time_segments", "spectral_tiles", "raw_ranges",
    "raw_waveform", "raw_samples", "render_file_path", "render_path",
    "file_path", "aligned_envelope_frames", "input_event_candidates",
    "frames", "events",
}


def http_json(url: str, payload: dict[str, Any], timeout: float) -> dict[str, Any]:
    request = urllib.request.Request(
        url,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json; charset=utf-8"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError("agent returned non-object JSON")
    return value


def kernel_command(socket: Any, payload: dict[str, Any]) -> dict[str, Any]:
    socket.send_json(payload)
    value = socket.recv_json()
    if not isinstance(value, dict):
        raise RuntimeError("kernel returned non-object JSON")
    return value


def tracks(state: dict[str, Any]) -> list[dict[str, Any]]:
    rows = state.get("tracks") or state.get("project_tracks") or []
    return [row for row in rows if isinstance(row, dict)]


def find_track(state: dict[str, Any], requested: str) -> dict[str, Any]:
    for row in tracks(state):
        if str(row.get("track_id") or row.get("id")) == requested:
            return row
    raise RuntimeError(f"track {requested!r} not found")


def find_plugin(track: dict[str, Any], requested: str) -> dict[str, Any]:
    rack = track.get("rack") if isinstance(track.get("rack"), dict) else {}
    rows = list(rack.get("nodes") or []) + list(track.get("plugins") or [])
    for row in rows:
        if isinstance(row, dict) and str(row.get("plugin_id") or row.get("id")) == requested:
            return row
    raise RuntimeError(f"plugin {requested!r} not found on target track")


def first_clip(track: dict[str, Any]) -> str:
    for row in track.get("clips") or []:
        if isinstance(row, dict):
            value = str(row.get("clip_id") or row.get("id") or "").strip()
            if value:
                return value
    return ""


def inspect_compressor(agent_http: str, track_id: str, plugin_id: str,
                       timeout: float) -> tuple[str, str]:
    response = http_json(agent_http.rstrip("/") + "/agent/invoke", {
        "tool": "plugin_grabber.inspect_compressor",
        "args": {"track_id": track_id, "plugin_id": plugin_id},
        "confirmed": False,
        "source": "compressor_dual_tap_smoke",
    }, timeout)
    if str(response.get("status", "")).lower() != "ok":
        raise RuntimeError(f"inspect_compressor failed: {response}")
    result = response.get("result") if isinstance(response.get("result"), dict) else {}
    topology = result.get("control_topology") if isinstance(result.get("control_topology"), dict) else {}
    generation = str(topology.get("generation") or "").strip()
    classification = str(result.get("classification") or "").strip()
    if not generation or not classification:
        raise RuntimeError("inspect_compressor omitted classification or generation")
    return classification, generation


def recursively_forbidden(value: Any) -> str:
    if isinstance(value, dict):
        for key, child in value.items():
            if str(key).strip().lower() in FORBIDDEN_RECEIPT_KEYS:
                return str(key)
            reason = recursively_forbidden(child)
            if reason:
                return reason
    elif isinstance(value, list):
        for child in value:
            reason = recursively_forbidden(child)
            if reason:
                return reason
    elif isinstance(value, str):
        text = value.strip()
        if len(text) >= 3 and text[1] == ":" and text[2] in "\\/":
            return "absolute_path"
        if text.startswith("/"):
            return "absolute_path"
    return ""


def artifact_path(workspace: Path, pair_id: str) -> Path:
    return workspace / "Artifacts" / "com_evidence" / pair_id / f"{pair_id}.json"


def validate_receipt(receipt: dict[str, Any], pair_id: str,
                     artifact: Path) -> list[dict[str, Any]]:
    alignment = receipt.get("latency_alignment") if isinstance(receipt.get("latency_alignment"), dict) else {}
    quality = receipt.get("quality_evidence") if isinstance(receipt.get("quality_evidence"), dict) else {}
    checks: list[tuple[str, bool]] = [
        ("terminal_ready", receipt.get("status") == "ready" and receipt.get("reason") == "ok"),
        ("pair_identity", receipt.get("pair_id") == pair_id and receipt.get("evidence_ref") == f"dad.compressor_dual_tap:{pair_id}"),
        ("tap_order", receipt.get("input_tap") == "compressor_input" and receipt.get("output_tap") == "compressor_output"),
        ("exact_window", int(receipt.get("end_sample") or 0) > int(receipt.get("start_sample") or -1)),
        ("scope_identity", all(str(receipt.get(key) or "").strip() for key in ("plugin_instance_id", "plugin_position", "chain_hash", "processor_state_hash", "scope_revision", "topology_generation"))),
        ("fresh_revisions", all(str(receipt.get(key) or "").strip() for key in ("source_revision", "clip_revision", "render_revision"))),
        ("format_identity", float(receipt.get("sample_rate") or 0) > 7000 and int(receipt.get("channel_count") or 0) > 0),
        ("latency_alignment", alignment.get("status") == "ready" and abs(int(alignment.get("residual_error_samples") or 0)) <= 1),
        ("determinism", receipt.get("determinism_proof_status") == "ready" and float(receipt.get("determinism_correlation") or 0) >= .999 and abs(float(receipt.get("determinism_peak_db_delta") or 0)) <= .10 and abs(float(receipt.get("determinism_rms_db_delta") or 0)) <= .10),
        ("nonblank_quality", all(isinstance(quality.get(tap), dict) and int(quality[tap].get("nonzero_samples") or 0) > 0 and int(quality[tap].get("nan_inf_samples") or 0) == 0 and float(quality[tap].get("coverage") or 0) >= .999 for tap in ("input", "output"))),
        ("fine_trace_external", int(receipt.get("envelope_frame_count") or 0) > 0 and artifact.is_file()),
        ("receipt_no_raw_leak", recursively_forbidden(receipt) == ""),
    ]
    if artifact.is_file():
        payload = artifact.read_bytes()
        checks += [
            ("artifact_hash", hashlib.sha256(payload).hexdigest() == receipt.get("artifact_sha256")),
            ("artifact_size", len(payload) == int(receipt.get("artifact_bytes") or 0)),
            ("artifact_has_raw_trace", b'"aligned_envelope_frames"' in payload),
        ]
    return [{"gate": name, "pass": passed} for name, passed in checks]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    parser.add_argument("--material", required=True)
    parser.add_argument("--workspace", default=r"D:\Vit_DAW\VitApp\Workspace")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--req-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--sub-url", default="tcp://127.0.0.1:5556")
    parser.add_argument("--timeout-sec", type=float, default=60)
    parser.add_argument("--output", default="")
    args = parser.parse_args()

    material = Path(args.material).resolve()
    if not material.is_file():
        raise RuntimeError(f"material missing: {material}")
    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, int(args.timeout_sec * 1000))
    req.setsockopt(zmq.SNDTIMEO, int(args.timeout_sec * 1000))
    req.connect(args.req_url)
    sub = context.socket(zmq.SUB)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.setsockopt(zmq.RCVTIMEO, 1000)
    sub.connect(args.sub_url)
    time.sleep(.4)

    state = kernel_command(req, {"cmd": "get_project_state"})
    track = find_track(state, args.track_id)
    plugin = find_plugin(track, args.plugin_id)
    clip_id = first_clip(track)
    if not clip_id:
        imported = kernel_command(req, {"cmd": "import_audio", "track_id": args.track_id, "file_path": str(material), "offset_time": 0})
        if imported.get("status") != "ok":
            raise RuntimeError(f"import_audio failed: {imported}")
        clip_id = str(imported.get("clip_id") or "")
    classification, generation = inspect_compressor(args.agent_http, args.track_id, args.plugin_id, args.timeout_sec)
    command = {
        "cmd": "compressor_dual_tap_probe", "request_id": "com2_product_smoke",
        "track_id": args.track_id, "clip_id": clip_id, "plugin_id": args.plugin_id,
        "topology_class": classification, "topology_generation": generation,
        "support_class": "single_band_broadband", "start_sample": 0,
        "end_sample": 480000, "deterministic": True,
    }
    reply = kernel_command(req, command)
    if reply.get("status") != "ok":
        raise RuntimeError(f"dual tap request rejected: {reply}")
    pair_id = str(reply.get("pair_id") or "")
    receipt: dict[str, Any] = {}
    deadline = time.time() + args.timeout_sec
    events: list[dict[str, Any]] = []
    while time.time() < deadline:
        try:
            event = sub.recv_json()
        except zmq.Again:
            continue
        if isinstance(event, dict) and event.get("pair_id") == pair_id:
            events.append(event)
            if event.get("command") == "compressor_dual_tap_probe_ready":
                receipt = event
                break
    if not receipt:
        raise RuntimeError("no terminal compressor_dual_tap_probe_ready event")
    artifact = artifact_path(Path(args.workspace).resolve(), pair_id)
    gates = validate_receipt(receipt, pair_id, artifact)
    status = "ok" if all(row["pass"] for row in gates) else "failed"
    report = {
        "schema_version": "com2.product_smoke.v1", "status": status,
        "target": {"track_id": args.track_id, "plugin_id": args.plugin_id, "plugin_name": plugin.get("name")},
        "pair_id": pair_id, "evidence_ref": receipt.get("evidence_ref"),
        "artifact_path": str(artifact), "receipt": receipt, "gates": gates,
        "event_count": len(events),
    }
    output = Path(args.output).resolve() if args.output else artifact.parent / "smoke_report.json"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"status": status, "report": str(output), "pair_id": pair_id, "gates": gates}, ensure_ascii=False, indent=2))
    req.close(); sub.close(); context.term()
    return 0 if status == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
