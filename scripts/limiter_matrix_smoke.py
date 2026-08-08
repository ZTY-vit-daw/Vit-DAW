#!/usr/bin/env python3
"""Capture Limiter V1 training/regression evidence on disposable tracks without writes."""
from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix
import limiter_blind_smoke as blind


SCRIPT_PATH = Path(__file__).resolve()
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("limiter_control_matrix.json")


def rejection_code(response: dict[str, Any]) -> str:
    result = matrix.result_of(response)
    return matrix.first_text(result, "code", "rejection_code")


def run_case(base: str, case: dict[str, Any], timeout: float, output: Path) -> dict[str, Any]:
    if matrix.first_text(case, "fixture"):
        return {"id": case["id"], "status": "covered_by_unit_fixture",
                "expectation": case["expectation"], "expected_rejection": case.get("expected_rejection"),
                "fixture": case["fixture"]}
    track_id = ""
    evidence: dict[str, Any] = {"case": case}
    try:
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {"name": f"Limiter matrix {case['id']}"}, timeout, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id, "plugin_path": case["plugin_path"], "plugin_name": case["plugin_name"],
            "plugin_identifier": identifier}, timeout, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(.35)
        parameters = matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True}, timeout), "plugin.get_parameters")
        inspect_one_response = matrix.invoke(base, "plugin_grabber.inspect_limiter", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout)
        inspect_two_response = matrix.invoke(base, "plugin_grabber.inspect_limiter", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout)
        evidence.update({"plugin_identifier": identifier, "plugin_resolution": resolution,
                         "parameters": parameters, "inspect_one_response": inspect_one_response,
                         "inspect_two_response": inspect_two_response})
        expectation = case["expectation"]
        if expectation == "not_limiter":
            first_code, second_code = rejection_code(inspect_one_response), rejection_code(inspect_two_response)
            expected = case["expected_rejection"]
            if first_code != expected or second_code != expected:
                raise RuntimeError(f"rejection codes {first_code!r}/{second_code!r}, expected {expected!r}")
            result = {"id": case["id"], "plugin_name": case["plugin_name"], "status": "passed",
                      "expectation": expectation, "rejection_code": first_code, "stable_two_reads": True}
        else:
            inspect_one = matrix.require_ok(inspect_one_response, "inspect.one")
            inspect_two = matrix.require_ok(inspect_two_response, "inspect.two")
            generation_one, generation_two = blind.validate_inspect(inspect_one), blind.validate_inspect(inspect_two)
            if generation_one != generation_two:
                raise RuntimeError("topology generation changed across identical reads")
            expected_class = expectation if expectation == "indexed_multi_stage_limiter" else ""
            if expected_class and inspect_one.get("classification") != expected_class:
                raise RuntimeError(f"classification={inspect_one.get('classification')!r}, expected {expected_class!r}")
            result = {"id": case["id"], "plugin_name": case["plugin_name"], "status": "passed",
                      "expectation": expectation, "classification": inspect_one.get("classification"),
                      "generation": generation_one, "stage_count": len(inspect_one.get("limiter_stages") or []),
                      "binding_count": len(blind.all_bindings(inspect_one)), "stable_two_reads": True}
        evidence["result"] = result
        blind.write_json(output, evidence)
        return result
    except Exception as error:
        result = {"id": case["id"], "plugin_name": case.get("plugin_name"), "status": "failed", "error": str(error)}
        evidence["result"] = result
        blind.write_json(output, evidence)
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
    args = parser.parse_args()
    manifest = blind.load_manifest(args.manifest.resolve())
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
                              args.output_dir / "cases" / f"{group}_{index:02d}_{case['id']}.json")
            result["set"] = group
            results.append(result)
            print(json.dumps(result, ensure_ascii=False), flush=True)
    summary = {"schema_version": "plugin_grabber.limiter_matrix_report.v1", "results": results,
               "verdict": "passed" if all(row["status"] in {"passed", "covered_by_unit_fixture"} for row in results) else "failed"}
    blind.write_json(args.output_dir / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
