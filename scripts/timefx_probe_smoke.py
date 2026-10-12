"""M2-TIMEFX-2 reverb/delay paired probe + FXM differential smoke.

Real-stack driver (started by run_timefx_probe_smoke.ps1 against the
Godot-launched VitApp/VitAgent/VspHub trio). Legs:

  A. stack setup over kernel ZMQ: probe track + imported probe audio + one
     reverb (Pro-R) or delay (Timeless 3) rack node;
  B. semantic write leg: agent HTTP inspect_{family} -> apply_{family}_controls;
     asserts the automatic post-write FXM differential probe attached a ready
     fxm_probe with real non-zero numbers (bypass != processed);
  C. direct kernel probe roundtrip: timefx_dual_tap_probe command -> terminal
     _ready event on the pub socket -> artifact on disk with matching sha256.

Exit 0 only when every gate passes. The probe track is deleted in finally so
the project copy under the run dir stays inspectable.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import time
import urllib.request
from pathlib import Path
from typing import Any

import zmq

from compressor_compat_matrix_smoke import first_text, invoke, require_ok
from compressor_dual_tap_smoke import kernel_command


FAMILY_TOOLS = {
    "reverb": ("plugin_grabber.inspect_reverb", "plugin_grabber.apply_reverb_controls"),
    "delay": ("plugin_grabber.inspect_delay", "plugin_grabber.apply_delay_controls"),
}
# role preference per family: the first role with a disclosed binding wins;
# value shape follows chat validate{Family}RequestUnit.
FAMILY_WRITE = {
    "reverb": [("decay", "value_s", 2.4), ("size", "value_s", 2.9), ("wet", "percent", 42), ("mix", "percent", 42)],
    "delay": [("time", "value_ms", 375.0), ("feedback", "percent", 35), ("mix", "percent", 40), ("wet", "percent", 40)],
}
PROBE_END_SAMPLE = 480000  # 10 s @ 48 kHz, matching the probe material


def request_json(method: str, url: str, payload: dict[str, Any], timeout: float) -> dict[str, Any]:
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method=method)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def require(condition: bool, message: str, report: list[dict[str, Any]], gate: str) -> None:
    report.append({"gate": gate, "pass": bool(condition), "detail": message})
    if not condition:
        raise RuntimeError(f"gate {gate} failed: {message}")


def stage_bindings(stage: dict[str, Any]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for section in ("tail", "timing", "character", "output", "feedback"):
        for row in stage.get(section) or []:
            if isinstance(row, dict):
                rows.append(row)
    return rows


def pick_write(inspect: dict[str, Any], family: str) -> tuple[dict[str, Any], str, float]:
    stage = inspect.get(f"{family}_stage")
    if not isinstance(stage, dict):
        stage = inspect.get("reverb_stage") if family == "reverb" else inspect.get("delay_stage")
    if not isinstance(stage, dict):
        raise RuntimeError(f"inspect_{family} did not disclose a {family} stage")
    bindings = {str(row.get("role")): row for row in stage_bindings(stage) if row.get("control_ref")}
    for role, key, value in FAMILY_WRITE[family]:
        if role in bindings:
            return bindings[role], key, value
    raise RuntimeError(f"no writable {family} control binding disclosed: {sorted(bindings)}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", required=True, type=Path)
    parser.add_argument("--family", default="reverb", choices=["reverb", "delay"])
    parser.add_argument("--plugin-path", required=True)
    parser.add_argument("--material", required=True, type=Path)
    parser.add_argument("--workspace", required=True, type=Path)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    report: list[dict[str, Any]] = []
    context = zmq.Context.instance()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, int(args.timeout_sec * 1000))
    req.setsockopt(zmq.LINGER, 0)
    req.connect("tcp://127.0.0.1:5555")
    sub = context.socket(zmq.SUB)
    sub.setsockopt(zmq.SUBSCRIBE, b"")
    sub.setsockopt(zmq.RCVTIMEO, 500)
    sub.setsockopt(zmq.LINGER, 0)
    sub.connect("tcp://127.0.0.1:5556")

    inspect_tool, apply_tool = FAMILY_TOOLS[args.family]
    track_id = ""
    try:
        # --- leg A: probe track + material + processor -----------------------
        added = kernel_command(req, {"cmd": "add_track"})
        require(added.get("status") == "ok", f"add_track reply: {added}", report, "A1.add_track")
        track_id = str(added.get("track_id") or "")
        require(bool(track_id), f"add_track track_id missing: {added}", report, "A1.track_id")

        imported = kernel_command(req, {"cmd": "import_audio", "track_id": track_id,
                                        "file_path": str(args.material), "offset_time": 0})
        require(imported.get("status") == "ok", f"import_audio reply: {imported}", report, "A2.import_audio")

        node = kernel_command(req, {"cmd": "rack_add_node", "track_id": track_id,
                                    "plugin_path": args.plugin_path, "auto_connect": True})
        require(node.get("status") == "ok", f"rack_add_node reply: {node}", report, "A3.rack_add_node")
        plugin_id = str(node.get("plugin_id") or node.get("plugin_item_id") or "")
        require(bool(plugin_id), f"rack_add_node plugin id missing: {node}", report, "A3.plugin_id")

        # wait until the live parameter surface is readable
        deadline = time.time() + 60.0
        params: dict[str, Any] = {}
        while time.time() < deadline:
            params = kernel_command(req, {"cmd": "get_plugin_parameters",
                                          "track_id": track_id, "plugin_id": plugin_id,
                                          "include_parameters": True})
            rows = params.get("parameters")
            if params.get("status") == "ok" and isinstance(rows, list) and rows:
                break
            time.sleep(0.8)
        require(params.get("status") == "ok" and isinstance(params.get("parameters"), list) and bool(params["parameters"]),
                f"plugin parameters unavailable: {str(params)[:400]}", report, "A4.parameters")

        # --- leg B: semantic write -> automatic FXM differential --------------
        inspect = require_ok(invoke(args.agent_http, inspect_tool, {
            "track_id": track_id, "plugin_id": plugin_id}, args.timeout_sec), f"{inspect_tool}")
        classification = first_text(inspect, "classification")
        generation = first_text(inspect.get("control_topology") or {}, "generation")
        require(bool(classification) and bool(generation),
                f"recognizer identity missing: class={classification!r} gen={generation!r}", report, "B1.recognizer")

        binding, value_key, value = pick_write(inspect, args.family)
        control_ref = str(binding.get("control_ref"))
        role = str(binding.get("role"))
        applied = require_ok(invoke(args.agent_http, apply_tool, {
            "track_id": track_id, "plugin_id": plugin_id,
            "controls": [{"control_ref": control_ref, value_key: value}],
        }, args.timeout_sec), f"{apply_tool}({role})")
        require(str(applied.get("status")) in {"exact", "quantized"},
                f"apply status: {applied.get('status')}", report, "B2.apply_status")

        probe = applied.get("fxm_probe")
        require(isinstance(probe, dict), f"apply result carried no fxm_probe: {str(applied)[:600]}", report, "B3.fxm_probe_attached")
        probe = probe if isinstance(probe, dict) else {}
        require(probe.get("status") == "ready", f"fxm probe status: {probe.get('status')} reason={probe.get('reason')}", report, "B3.probe_ready")
        require(probe.get("fxm_status") in {"ready", "partial"},
                f"fxm projection status: {probe.get('fxm_status')}", report, "B3.fxm_status")

        projection = probe.get("fxm_projection") or {}
        trust = projection.get("trust_quality") or {}
        require(trust.get("same_source") and trust.get("same_window") and trust.get("same_format"),
                f"identity gates: {trust}", report, "B4.identity_gates")

        deltas = {key: probe.get(key) for key in ("rms_delta_db", "lufs_delta_db", "peak_delta_db", "crest_delta_db")}
        require(all(isinstance(v, (int, float)) and math.isfinite(float(v)) for v in deltas.values()),
                f"delta values not real numbers: {deltas}", report, "B5.deltas_finite")
        require(abs(float(deltas["rms_delta_db"])) > 0.01,
                f"bypass equals processed (rms delta {deltas['rms_delta_db']}): no transformation measured", report, "B5.bypass_ne_processed")
        fxm_delta = projection.get("effect_delta") or {}
        require(fxm_delta.get("rms_delta_db") is not None and fxm_delta.get("lufs_delta_db") is not None
                and fxm_delta.get("peak_delta_db") is not None and fxm_delta.get("crest_delta_db") is not None,
                f"fxm projection effect_delta incomplete: {fxm_delta}", report, "B5.effect_delta_complete")

        # --- leg C: direct kernel probe roundtrip ------------------------------
        request_id = f"timefx_smoke_direct_{int(time.time())}"
        reply = kernel_command(req, {
            "cmd": "timefx_dual_tap_probe", "request_id": request_id,
            "track_id": track_id, "plugin_id": plugin_id,
            "family": args.family, "topology_class": classification,
            "topology_generation": generation,
            "start_sample": 0, "end_sample": PROBE_END_SAMPLE,
            "deterministic": True,
        })
        require(reply.get("status") == "ok", f"probe command rejected: {str(reply)[:400]}", report, "C1.probe_accepted")
        pair_id = str(reply.get("pair_id") or "")
        require(pair_id.startswith("tfx2_"), f"pair id shape: {pair_id}", report, "C1.pair_id")
        evidence_ref = str(reply.get("evidence_ref") or "")
        require(evidence_ref.startswith("vit://dad.timefx_dual_tap/"), f"evidence ref: {evidence_ref}", report, "C1.evidence_ref")

        receipt: dict[str, Any] = {}
        events: list[dict[str, Any]] = []
        deadline = time.time() + args.timeout_sec
        while time.time() < deadline and not receipt:
            try:
                event = sub.recv_json()
            except zmq.Again:
                continue
            if isinstance(event, dict) and event.get("pair_id") == pair_id:
                events.append(event)
                if event.get("command") == "timefx_dual_tap_probe_ready":
                    receipt = event
        require(bool(receipt), f"no terminal probe event for {pair_id} (events seen: {len(events)})", report, "C2.terminal_event")
        require(receipt.get("status") == "ready", f"probe receipt status: {receipt.get('status')} reason={receipt.get('reason')}", report, "C2.receipt_ready")

        metrics = receipt.get("tap_metrics") or {}
        baseline = metrics.get("baseline") or {}
        processed = metrics.get("processed") or {}
        require(baseline.get("rms_dbfs") is not None and processed.get("rms_dbfs") is not None,
                f"tap metrics missing: {metrics}", report, "C3.tap_metrics")
        require(abs(float(baseline.get("rms_dbfs", 0.0)) - float(processed.get("rms_dbfs", 0.0))) > 0.01,
                f"kernel taps equal: baseline={baseline.get('rms_dbfs')} processed={processed.get('rms_dbfs')}", report, "C3.bypass_ne_processed")
        for row, label in ((baseline, "baseline"), (processed, "processed")):
            require(all(row.get(key) is not None and math.isfinite(float(row[key])) for key in ("peak_dbfs", "rms_dbfs", "approximate_lufs", "crest_db")),
                    f"{label} metrics incomplete: {row}", report, "C3.metrics_complete")

        artifact_path = args.workspace / "Artifacts" / "timefx_evidence" / pair_id / f"{pair_id}.json"
        require(artifact_path.is_file(), f"artifact missing: {artifact_path}", report, "C4.artifact_on_disk")
        if artifact_path.is_file():
            raw = artifact_path.read_bytes()
            digest = hashlib.sha256(raw).hexdigest()
            require(digest == str(receipt.get("artifact_sha256") or ""),
                    f"artifact sha mismatch: disk={digest} receipt={receipt.get('artifact_sha256')}", report, "C4.artifact_sha")
            artifact = json.loads(raw.decode("utf-8"))
            require(artifact.get("schema_version") == "dad.timefx_dual_tap_evidence.v1",
                    f"artifact schema: {artifact.get('schema_version')}", report, "C4.artifact_schema")
            require(artifact.get("conditions", {}).get("tail_policy") == "same_window_pair",
                    f"artifact tail policy: {artifact.get('conditions', {}).get('tail_policy')}", report, "C4.tail_policy")

        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps({
            "family": args.family, "track_id": track_id, "plugin_id": plugin_id,
            "classification": classification, "generation": generation,
            "write": {"role": role, "key": value_key, "value": value},
            "fxm_probe": probe, "direct_receipt": receipt, "gates": report,
        }, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"timefx probe smoke PASS ({args.family}); report: {args.output}")
        return 0
    finally:
        if track_id:
            try:
                kernel_command(req, {"cmd": "delete_track", "track_id": track_id})
            except Exception:
                pass
        req.close()
        sub.close()


if __name__ == "__main__":
    raise SystemExit(main())
