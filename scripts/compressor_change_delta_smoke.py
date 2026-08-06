from __future__ import annotations

import argparse
import json
import math
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

import zmq

import compressor_apply_smoke as apply_smoke
import compressor_compat_matrix_smoke as matrix
from compressor_dual_tap_smoke import (
    artifact_path,
    find_plugin,
    find_track,
    first_clip,
    inspect_compressor,
    kernel_command,
    validate_receipt,
)


def capture(req: zmq.Socket, sub: zmq.Socket, workspace: Path, agent_http: str,
            track_id: str, plugin_id: str, material: Path, timeout: float,
            request_id: str) -> tuple[dict[str, Any], Path, list[dict[str, Any]]]:
    state = kernel_command(req, {"cmd": "get_project_state"})
    track = find_track(state, track_id)
    find_plugin(track, plugin_id)
    clip_id = first_clip(track)
    if not clip_id:
        imported = kernel_command(req, {"cmd": "import_audio", "track_id": track_id,
                                        "file_path": str(material), "offset_time": 0})
        if imported.get("status") != "ok":
            raise RuntimeError(f"import_audio failed: {imported}")
        clip_id = str(imported.get("clip_id") or "")
    classification, generation = inspect_compressor(agent_http, track_id, plugin_id, timeout)
    reply = kernel_command(req, {
        "cmd": "compressor_dual_tap_probe", "request_id": request_id,
        "track_id": track_id, "clip_id": clip_id, "plugin_id": plugin_id,
        "topology_class": classification, "topology_generation": generation,
        "support_class": "single_band_broadband", "start_sample": 0,
        "end_sample": 480000, "deterministic": True,
    })
    if reply.get("status") != "ok":
        raise RuntimeError(f"dual tap request rejected: {reply}")
    pair_id = str(reply.get("pair_id") or "")
    receipt: dict[str, Any] = {}
    events: list[dict[str, Any]] = []
    deadline = time.time() + timeout
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
        raise RuntimeError(f"no terminal event for {request_id}")
    artifact = artifact_path(workspace, pair_id)
    gates = validate_receipt(receipt, pair_id, artifact)
    if not all(row["pass"] for row in gates):
        raise RuntimeError(f"{request_id} capture gates failed: {gates}")
    return receipt, artifact, events


def inspect_and_controls(agent_http: str, track_id: str, plugin_id: str,
                         timeout: float) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    inspect = matrix.require_ok(matrix.invoke(agent_http, "plugin_grabber.inspect_compressor", {
        "track_id": track_id, "plugin_id": plugin_id,
    }, timeout), "inspect compressor")
    bindings = [binding for binding in apply_smoke.compressor_bindings(inspect)
                if matrix.first_text(binding, "role") == "reduction_amount"]
    if not bindings:
        raise RuntimeError("inspect omitted requested smoke role reduction_amount")
    binding = bindings[0]
    current = binding.get("current_physical")
    if not isinstance(current, (int, float)) or not math.isfinite(float(current)):
        raise RuntimeError("reduction_amount omitted finite current_physical")
    return inspect, [{
        "control_ref": matrix.first_text(binding, "control_ref"),
        "display_value": moderate_physical(binding, float(current)),
    }]


def moderate_physical(binding: dict[str, Any], current: float) -> float:
    """Choose a material, interior state change for the reversible COM-4 smoke.

    The generic apply smoke intentionally exercises domain endpoints.  COM-4 instead
    needs a changed processor state that remains observable by COM-2, so it moves by
    roughly one quarter of a trusted continuous domain and keeps endpoint headroom.
    """
    domain = binding.get("domain")
    if isinstance(domain, dict) and float(domain.get("confidence") or 0) >= 0.80:
        lo, hi = domain.get("min"), domain.get("max")
        if all(isinstance(value, (int, float)) and math.isfinite(float(value))
               for value in (lo, hi)):
            lower, upper = sorted((float(lo), float(hi)))
            span = upper - lower
            if span > 0:
                guard = 0.10 * span
                step = 0.25 * span
                candidates = [current + step, current - step]
                interior = [value for value in candidates
                            if lower + guard <= value <= upper - guard]
                if interior:
                    return interior[0]

    measured: list[float] = []
    for point in binding.get("curve") or []:
        if (isinstance(point, list) and len(point) == 2 and
                isinstance(point[1], (int, float)) and math.isfinite(float(point[1]))):
            measured.append(float(point[1]))
    for row in binding.get("reachable_values") or []:
        value = row.get("physical") if isinstance(row, dict) else None
        if isinstance(value, (int, float)) and math.isfinite(float(value)):
            measured.append(float(value))
    if len(set(measured)) >= 2:
        lower, upper = min(measured), max(measured)
        span = upper - lower
        candidates = [value for value in set(measured)
                      if lower + 0.10 * span <= value <= upper - 0.10 * span and
                      abs(value - current) >= 0.08 * span]
        if candidates:
            return min(candidates, key=lambda value: abs(abs(value-current) - 0.20*span))
    raise RuntimeError("reduction_amount has no moderate measured physical target")


def snapshot(agent_http: str, track_id: str, plugin_id: str, timeout: float) -> dict[str, float]:
    response = matrix.require_ok(matrix.invoke(agent_http, "plugin.get_parameters", {
        "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True,
    }, timeout), "plugin parameters")
    return apply_smoke.parameter_snapshot(response)


def derive_change(repo: Path, before: Path, after: Path) -> dict[str, Any]:
    process = subprocess.run([
        "go", "run", "./cmd/comderive", "-before-artifact", str(before),
        "-after-artifact", str(after),
    ], cwd=repo / "agent", capture_output=True, text=True, encoding="utf-8")
    if process.returncode != 0:
        raise RuntimeError(f"comderive failed: {process.stderr.strip()} {process.stdout[:1000]}")
    return json.loads(process.stdout)


def main() -> int:
    parser = argparse.ArgumentParser(description="COM-4 real state-change attribution smoke")
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    parser.add_argument("--material", required=True)
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=120)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    repo, workspace = Path(args.repo_root).resolve(), Path(args.workspace).resolve()
    material, output = Path(args.material).resolve(), Path(args.output).resolve()

    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, int(args.timeout_sec * 1000))
    req.setsockopt(zmq.SNDTIMEO, int(args.timeout_sec * 1000))
    req.setsockopt(zmq.LINGER, 0)
    req.connect("tcp://127.0.0.1:5555")
    sub = context.socket(zmq.SUB)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.setsockopt(zmq.RCVTIMEO, 1000)
    sub.setsockopt(zmq.LINGER, 0)
    sub.connect("tcp://127.0.0.1:5556")
    time.sleep(.4)

    before_snapshot: dict[str, float] = {}
    before_receipt: dict[str, Any] = {}
    after_receipt: dict[str, Any] = {}
    before_artifact: Path | None = None
    after_artifact: Path | None = None
    applied: dict[str, Any] = {}
    restored: dict[str, Any] = {}
    restore_attempted = False
    restore_verified = False
    delta: dict[str, Any] = {}
    failure = ""
    try:
        before_snapshot = snapshot(args.agent_http, args.track_id, args.plugin_id, args.timeout_sec)
        _, controls = inspect_and_controls(args.agent_http, args.track_id, args.plugin_id, args.timeout_sec)
        before_receipt, before_artifact, _ = capture(
            req, sub, workspace, args.agent_http, args.track_id, args.plugin_id,
            material, args.timeout_sec, "com4_before")
        applied = matrix.require_ok(matrix.invoke(args.agent_http,
            "plugin_grabber.apply_compressor_controls", {
                "track_id": args.track_id, "plugin_id": args.plugin_id,
                "atomic": True, "controls": controls,
            }, args.timeout_sec, True), "apply compressor state change")
        if not matrix.first_text(applied, "restore_ref"):
            raise RuntimeError("apply omitted restore_ref")
        if not all(row.get("actual_readback") for row in applied.get("controls") or []):
            raise RuntimeError("apply omitted actual_readback")
        after_receipt, after_artifact, _ = capture(
            req, sub, workspace, args.agent_http, args.track_id, args.plugin_id,
            material, args.timeout_sec, "com4_after")
        delta = derive_change(repo, before_artifact, after_artifact)
    except Exception as exc:  # restoration still runs below
        failure = str(exc)
    finally:
        restore_ref = matrix.first_text(applied, "restore_ref")
        if restore_ref:
            restore_attempted = True
            try:
                restored = matrix.require_ok(matrix.invoke(args.agent_http,
                    "plugin_grabber.apply_compressor_controls", {
                        "track_id": args.track_id, "plugin_id": args.plugin_id,
                        "atomic": True, "restore_ref": restore_ref,
                    }, args.timeout_sec, True), "restore compressor state")
                after_restore = snapshot(args.agent_http, args.track_id, args.plugin_id, args.timeout_sec)
                restore_verified = all(
                    abs(after_restore.get(param_id, value) - value) <= 1e-4
                    for param_id, value in before_snapshot.items()
                )
                if not restore_verified and not failure:
                    failure = "full parameter snapshot did not restore"
            except Exception as exc:
                if not failure:
                    failure = f"restore failed: {exc}"
        req.close(); sub.close(); context.term()

    behavior = delta.get("behavior_change") if isinstance(delta, dict) else {}
    dimensions = {row.get("dimension"): row for row in (behavior or {}).get("dimensions", []) if isinstance(row, dict)}
    same_input = (delta.get("trust_quality") or {}).get("input_equivalent") is True
    state_changed = bool(before_receipt and after_receipt and
        before_receipt.get("processor_state_hash") != after_receipt.get("processor_state_hash") and
        before_receipt.get("render_revision") != after_receipt.get("render_revision"))
    gates = [
        {"gate": "before_capture", "pass": before_artifact is not None},
        {"gate": "typed_apply", "pass": bool(applied and matrix.first_text(applied, "restore_ref"))},
        {"gate": "after_capture", "pass": after_artifact is not None},
        {"gate": "state_revision_changed", "pass": state_changed},
        {"gate": "input_equivalent", "pass": same_input},
        {"gate": "delta_ready_or_partial", "pass": delta.get("mode") == "change_delta" and delta.get("status") in {"ready", "partial"}},
        {"gate": "child_ids_preserved", "pass": bool((behavior or {}).get("before_projection_id") and (behavior or {}).get("after_projection_id"))},
        {"gate": "typed_dimensions", "pass": {"gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation"} <= set(dimensions)},
        {"gate": "restore_attempted", "pass": restore_attempted},
        {"gate": "restore_verified", "pass": restore_verified},
    ]
    status = "ok" if not failure and all(row["pass"] for row in gates) else "failed"
    report = {
        "schema_version": "com.change_delta_smoke.v1", "status": status, "failure": failure,
        "target": {"track_id": args.track_id, "plugin_id": args.plugin_id},
        "before": {"artifact": str(before_artifact) if before_artifact else "", "receipt": before_receipt},
        "after": {"artifact": str(after_artifact) if after_artifact else "", "receipt": after_receipt},
        "apply": applied, "restore": restored,
        "change_delta": delta, "gates": gates,
    }
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"status": status, "report": str(output), "failure": failure,
                      "projection_id": delta.get("projection_id"), "gates": gates}, ensure_ascii=False, indent=2))
    return 0 if status == "ok" else 1


if __name__ == "__main__":
    sys.exit(main())
