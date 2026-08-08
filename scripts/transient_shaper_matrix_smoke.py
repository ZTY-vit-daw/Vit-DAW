#!/usr/bin/env python3
"""Capture and validate transient-shaper topology on disposable tracks."""
from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


SCRIPT_PATH = Path(__file__).resolve()
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("transient_shaper_control_matrix.json")


def load_manifest(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError("Transient-shaper manifest must be a JSON object")
    return value


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def rejection_code(response: dict[str, Any]) -> str:
    return matrix.first_text(matrix.result_of(response), "code", "rejection_code")


def all_bindings(result: dict[str, Any]) -> list[dict[str, Any]]:
    stage = result.get("transient_shaper_stage") or {}
    bindings: list[dict[str, Any]] = []
    for section in ("envelope_action", "detector", "timing", "shape", "mode", "output"):
        bindings.extend(row for row in stage.get(section) or [] if isinstance(row, dict))
    return bindings


def validate_inspect(result: dict[str, Any], case: dict[str, Any]) -> tuple[str, int]:
    if result.get("mapping_source") != "generic_structural":
        raise RuntimeError(f"mapping_source={result.get('mapping_source')!r}")
    topology = result.get("control_topology")
    if not isinstance(topology, dict) or not matrix.first_text(topology, "generation"):
        raise RuntimeError("inspect omitted topology generation")
    bindings = all_bindings(result)
    if not bindings:
        raise RuntimeError("inspect omitted transient-shaper bindings")
    for binding in bindings:
        if not matrix.first_text(binding, "control_ref"):
            raise RuntimeError("transient-shaper binding omitted control_ref")
    owned_ids = {matrix.first_text(binding, "param_id") for binding in bindings}
    for auxiliary in result.get("auxiliary_stages") or []:
        if not isinstance(auxiliary, dict):
            continue
        kind = matrix.first_text(auxiliary, "kind")
        if kind not in {"limiter", "clipper", "limiter_or_clipper"}:
            continue
        overlap = owned_ids.intersection(str(value) for value in auxiliary.get("param_ids") or [])
        if overlap:
            raise RuntimeError(f"auxiliary {kind} leaked transient control refs: {sorted(overlap)}")
    roles = {matrix.first_text(binding, "role") for binding in bindings}
    missing_roles = set(case.get("expected_roles") or []).difference(roles)
    if missing_roles:
        raise RuntimeError(f"inspect omitted expected roles: {sorted(missing_roles)}")
    minimum = int(case.get("expected_min_binding_count") or 0)
    if len(bindings) < minimum:
        raise RuntimeError(f"binding_count={len(bindings)}, expected at least {minimum}")
    return matrix.first_text(topology, "generation"), len(bindings)


def run_case(base: str, case: dict[str, Any], timeout: float, output: Path,
             capture_only: bool) -> dict[str, Any]:
    if matrix.first_text(case, "fixture"):
        return {"id": case["id"], "status": "covered_by_unit_fixture",
                "expectation": case["expectation"], "fixture": case["fixture"]}
    track_id = ""
    evidence: dict[str, Any] = {"case": case, "capture_only": capture_only}
    try:
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {
            "name": f"Transient-shaper matrix {case['id']}"}, timeout, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id, "plugin_path": case["plugin_path"],
            "plugin_name": case["plugin_name"], "plugin_identifier": identifier,
        }, timeout, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(.35)
        parameters = matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True,
        }, timeout), "plugin.get_parameters")
        evidence.update({"plugin_identifier": identifier, "plugin_resolution": resolution,
                         "parameters": parameters})
        if capture_only:
            result = {"id": case["id"], "plugin_name": case["plugin_name"],
                      "status": "captured", "parameter_count": parameters.get("parameter_count", 0)}
        else:
            responses = [matrix.invoke(base, "plugin_grabber.inspect_transient_shaper", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout) for _ in range(2)]
            evidence["inspect_responses"] = responses
            if case["expectation"] == "not_transient_shaper":
                expected = case["expected_rejection"]
                codes = [rejection_code(response) for response in responses]
                if codes != [expected, expected]:
                    raise RuntimeError(f"rejection codes {codes!r}, expected {expected!r}")
                result = {"id": case["id"], "plugin_name": case["plugin_name"],
                          "status": "passed", "rejection_code": expected, "stable_two_reads": True}
            else:
                values = [matrix.require_ok(response, f"inspect.{index}")
                          for index, response in enumerate(responses, 1)]
                facts = [validate_inspect(value, case) for value in values]
                if facts[0] != facts[1]:
                    raise RuntimeError("topology changed across identical reads")
                result = {"id": case["id"], "plugin_name": case["plugin_name"],
                          "status": "passed", "classification": values[0].get("classification"),
                          "generation": facts[0][0], "binding_count": facts[0][1],
                          "stable_two_reads": True}
        evidence["result"] = result
        write_json(output, evidence)
        return result
    except Exception as error:
        result = {"id": case["id"], "plugin_name": case.get("plugin_name"),
                  "status": "failed", "error": str(error)}
        evidence["result"] = result
        write_json(output, evidence)
        return result
    finally:
        if track_id:
            cleanup = matrix.invoke(base, "track.delete", {"track_id": track_id}, timeout, True)
            if str(cleanup.get("status", "")).lower() not in {"ok", "success", "completed"}:
                raise RuntimeError(f"disposable track cleanup failed: {cleanup}")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180)
    parser.add_argument("--sets", choices=("training", "regression", "both"), default="both")
    parser.add_argument("--capture-only", action="store_true")
    args = parser.parse_args()
    manifest = load_manifest(args.manifest.resolve())
    groups = []
    if args.sets in {"training", "both"}:
        groups.append(("training", manifest.get("training_cases") or []))
    if args.sets in {"regression", "both"}:
        groups.append(("regression", manifest.get("regression_cases") or []))
    args.output_dir.mkdir(parents=True, exist_ok=True)
    results = []
    for group, cases in groups:
        for index, case in enumerate(cases, 1):
            result = run_case(args.agent_http, case, args.timeout_sec,
                              args.output_dir / "cases" / f"{group}_{index:02d}_{case['id']}.json",
                              args.capture_only)
            result["set"] = group
            results.append(result)
            print(json.dumps(result, ensure_ascii=False), flush=True)
    summary = {"schema_version": "plugin_grabber.transient_shaper_matrix_report.v1",
               "capture_only": args.capture_only, "results": results,
               "verdict": "passed" if all(row["status"] in {"passed", "captured", "covered_by_unit_fixture"}
                                          for row in results) else "failed"}
    write_json(args.output_dir / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
