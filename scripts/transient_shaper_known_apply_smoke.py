#!/usr/bin/env python3
"""Exercise typed transient-shaper apply/readback/restore on disclosed Waves cases."""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def result_of(response: dict[str, Any], label: str) -> dict[str, Any]:
    return matrix.require_ok(response, label)


def binding_for_role(inspect: dict[str, Any], role: str) -> dict[str, Any]:
    stage = inspect.get("transient_shaper_stage") or {}
    for section in ("envelope_action", "detector", "timing", "shape", "mode", "output"):
        for row in stage.get(section) or []:
            if isinstance(row, dict) and matrix.first_text(row, "role") == role:
                return row
    raise RuntimeError(f"inspect omitted role {role}")


def normalized_snapshot(parameters: dict[str, Any]) -> dict[str, float]:
    result: dict[str, float] = {}
    for row in parameters.get("parameters") or []:
        if not isinstance(row, dict):
            continue
        param_id = matrix.first_text(row, "param_id", "id")
        value = row.get("normalized_value")
        if param_id and isinstance(value, (int, float)):
            result[param_id] = float(value)
    return result


def run_case(base: str, case: dict[str, Any], timeout: float, output: Path) -> dict[str, Any]:
    track_id = ""
    evidence: dict[str, Any] = {"case": case}
    try:
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        track = result_of(matrix.invoke(base, "track.add_audio", {
            "name": f"Transient apply {case['id']}"}, timeout, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        loaded = result_of(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id, "plugin_path": case["plugin_path"],
            "plugin_name": case["plugin_name"], "plugin_identifier": identifier,
        }, timeout, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        before_response = matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True}, timeout)
        before = result_of(before_response, "parameters.before")
        inspect_response = matrix.invoke(base, "plugin_grabber.inspect_transient_shaper", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout)
        inspect = result_of(inspect_response, "inspect")
        if case["id"] == "waves_smack_attack_stereo":
            bindings = [binding_for_role(inspect, "attack_amount"),
                        binding_for_role(inspect, "sustain_amount")]
            controls = [{"control_ref": bindings[0]["control_ref"], "percent": 25.0},
                        {"control_ref": bindings[1]["control_ref"], "percent": -20.0}]
        else:
            bindings = [binding_for_role(inspect, "transient_range")]
            controls = [{"control_ref": bindings[0]["control_ref"], "value_db": 3.0}]
        apply_response = matrix.invoke(base, "plugin_grabber.apply_transient_shaper_controls", {
            "track_id": track_id, "plugin_id": plugin_id, "atomic": True,
            "controls": controls}, timeout, True)
        applied = result_of(apply_response, "apply")
        if matrix.first_text(applied, "status") not in {"exact", "quantized"}:
            raise RuntimeError(f"unexpected apply status {applied.get('status')!r}")
        restore_ref = matrix.first_text(applied, "restore_ref")
        if not restore_ref:
            raise RuntimeError("apply omitted restore_ref")
        restore_response = matrix.invoke(base, "plugin_grabber.apply_transient_shaper_controls", {
            "track_id": track_id, "plugin_id": plugin_id, "atomic": True,
            "restore_ref": restore_ref}, timeout, True)
        restored = result_of(restore_response, "restore")
        after_response = matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True}, timeout)
        after = result_of(after_response, "parameters.after")
        before_values, after_values = normalized_snapshot(before), normalized_snapshot(after)
        drift = {key: [value, after_values.get(key)] for key, value in before_values.items()
                 if key not in after_values or abs(value - after_values[key]) > 1e-6}
        if drift:
            raise RuntimeError(f"restore drift: {drift}")
        result = {"id": case["id"], "status": "passed", "classification": inspect.get("classification"),
                  "roles": [binding.get("role") for binding in bindings], "apply_status": applied.get("status"),
                  "restore_status": restored.get("status"), "full_snapshot_restored": True}
        evidence.update({"plugin_identifier": identifier, "plugin_resolution": resolution,
                         "before_response": before_response, "inspect_response": inspect_response,
                         "apply_response": apply_response, "restore_response": restore_response,
                         "after_response": after_response, "result": result})
        write_json(output, evidence)
        return result
    except Exception as error:
        result = {"id": case["id"], "status": "failed", "error": str(error)}
        evidence["result"] = result
        write_json(output, evidence)
        return result
    finally:
        if track_id:
            matrix.invoke(base, "track.delete", {"track_id": track_id}, timeout, True)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=Path(__file__).with_name("transient_shaper_control_matrix.json"))
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.resolve().read_text(encoding="utf-8"))
    cases = manifest.get("training_cases") or []
    results = [run_case(args.agent_http, case, args.timeout_sec,
                        args.output_dir / "cases" / f"{index:02d}_{case['id']}.json")
               for index, case in enumerate(cases, 1)]
    summary = {"schema_version": "plugin_grabber.transient_shaper_known_apply_report.v1",
               "results": results,
               "verdict": "passed" if len(results) == 2 and all(row["status"] == "passed" for row in results) else "failed"}
    write_json(args.output_dir / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
