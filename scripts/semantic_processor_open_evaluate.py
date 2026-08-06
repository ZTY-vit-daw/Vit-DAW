#!/usr/bin/env python3
"""Evaluate a completed blind semantic-processor run against sealed truth."""

from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "semantic_processor_open_evaluation.v1"


def rows(value: Any) -> list[dict[str, Any]]:
    return [row for row in value if isinstance(row, dict)] if isinstance(value, list) else []


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def find_key(value: Any, key: str) -> list[Any]:
    found: list[Any] = []
    if isinstance(value, dict):
        for child_key, child in value.items():
            if child_key == key:
                found.append(child)
            found.extend(find_key(child, key))
    elif isinstance(value, list):
        for child in value:
            found.extend(find_key(child, key))
    return found


def acoustic_summary(case: dict[str, Any]) -> dict[str, Any]:
    com_statuses: list[str] = []
    com_modes: list[str] = []
    eq_acoustic: list[str] = []
    parameter_audits: list[str] = []
    for stage in rows(case.get("stages")):
        path = Path(str(stage.get("raw_file", "")))
        if not path.is_file():
            continue
        raw = json.loads(path.read_text(encoding="utf-8"))
        for value in find_key(raw, "com_evaluation"):
            if isinstance(value, dict):
                if value.get("status"):
                    com_statuses.append(str(value["status"]))
                if value.get("mode"):
                    com_modes.append(str(value["mode"]))
        for value in find_key(raw, "verification"):
            if isinstance(value, dict) and value.get("acoustic"):
                eq_acoustic.append(str(value["acoustic"]))
        for value in find_key(raw, "parameter_audit"):
            if isinstance(value, dict) and value.get("status"):
                parameter_audits.append(str(value["status"]))
    return {
        "com_statuses": list(dict.fromkeys(com_statuses)),
        "com_modes": list(dict.fromkeys(com_modes)),
        "eq_acoustic_statuses": list(dict.fromkeys(eq_acoustic)),
        "parameter_audit_statuses": list(dict.fromkeys(parameter_audits)),
        "has_post_action_acoustic_evidence": bool(com_statuses or eq_acoustic),
    }


def execution_summary(case: dict[str, Any]) -> dict[str, Any]:
    statuses: list[str] = []
    stop_reasons: list[str] = []
    failure_codes: list[str] = []
    errors: list[str] = []
    rollback_restored = False
    for stage in rows(case.get("stages")):
        path = Path(str(stage.get("raw_file", "")))
        if not path.is_file():
            continue
        raw = json.loads(path.read_text(encoding="utf-8"))
        data = raw.get("workflow_data") if isinstance(raw.get("workflow_data"), dict) else {}
        for value in (data.get("execution_status"), data.get("status"), raw.get("goal_status")):
            if value and str(value).strip():
                statuses.append(str(value).strip().lower())
        if raw.get("stop_reason") and str(raw["stop_reason"]).strip():
            stop_reasons.append(str(raw["stop_reason"]).strip().lower())
        for value in find_key(data, "failure_code"):
            if value and str(value).strip():
                failure_codes.append(str(value).strip().lower())
        if raw.get("error") and str(raw["error"]).strip():
            errors.append(str(raw["error"]).strip())
        for rollback in find_key(raw, "rollback"):
            if isinstance(rollback, dict) and str(rollback.get("status", "")).lower() == "restored":
                rollback_restored = True
            elif str(rollback).lower() == "restored":
                rollback_restored = True
    materialization_rejected = "materialization_rejected" in statuses or "execution_materialization_rejected" in failure_codes
    model_service_error = any("http error 5" in error.lower() for error in errors)
    observation_only = any("observation_only" in reason for reason in stop_reasons)
    failed = materialization_rejected or any(status in {"failed", "partial_failure", "rejected", "error"} for status in statuses)
    return {
        "statuses": list(dict.fromkeys(statuses)),
        "stop_reasons": list(dict.fromkeys(stop_reasons)),
        "failure_codes": list(dict.fromkeys(failure_codes)),
        "failed": failed,
        "materialization_rejected": materialization_rejected,
        "model_service_error": model_service_error,
        "observation_only_resolution": observation_only,
        "rollback_restored": rollback_restored,
        "errors": list(dict.fromkeys(errors)),
    }


def evaluate_case(case: dict[str, Any], truth: dict[str, Any]) -> dict[str, Any]:
    loop = case.get("free_state_reasoning_loop") if isinstance(case.get("free_state_reasoning_loop"), dict) else {}
    applied_actions = [
        str(row.get("processor_type", "")).lower()
        for row in rows(loop.get("actions"))
        if str(row.get("status", "")).lower() == "applied" and str(row.get("processor_type", "")).strip()
    ]
    planned = [str(value).lower() for value in case.get("processor_sequence", []) if str(value).strip()]
    actual = applied_actions or planned
    expected_first = [str(value).lower() for value in truth.get("expected_first_processors", [])]
    expected_eventual = [str(value).lower() for value in truth.get("expected_eventual_processors", [])]
    first = actual[0] if actual else "none"
    execution = execution_summary(case)
    route_observable = bool(actual) or not execution["model_service_error"]
    first_correct = (first in expected_first) if route_observable else None
    required_eventual = {value for value in expected_eventual if value != "none"}
    eventual_coverage = required_eventual.issubset(set(actual))
    expected_mutation = first != "none"
    load_count = int(case.get("load_confirmation_count") or 0)
    parameter_count = int(case.get("parameter_confirmation_count") or 0)
    action_count = int(case.get("action_confirmation_count") or 0)
    if not expected_mutation:
        confirmation_boundary = True
    elif first in {"eq", "compressor"}:
        confirmation_boundary = load_count >= 1 and parameter_count >= 1
    else:
        confirmation_boundary = action_count >= 1
    acoustic = acoustic_summary(case)
    failures = []
    if first_correct is False:
        failures.append("route_mismatch")
    if not route_observable:
        failures.append("route_unobservable_due_model_service_error")
    if not eventual_coverage:
        failures.append("incomplete_eventual_processor_coverage")
    if not confirmation_boundary:
        failures.append("confirmation_boundary_missing")
    if expected_mutation and not acoustic["has_post_action_acoustic_evidence"]:
        failures.append("post_action_acoustic_evidence_missing")
    if expected_mutation and execution["failed"]:
        failures.append("processor_execution_failed")
    if execution["materialization_rejected"]:
        failures.append("materialization_rejected")
    if execution["observation_only_resolution"]:
        failures.append("execution_resolution_observation_only")
    if execution["rollback_restored"]:
        failures.append("rollback_restored")
    if str(truth.get("issue_kind", "")).lower() == "control" and first != "none":
        failures.append("control_false_positive")
    original_intent_retained = str(loop.get("original_intent", "")).strip() == str(case.get("prompt", "")).strip()
    if not original_intent_retained:
        failures.append("free_state_original_intent_lost")
    if str(loop.get("status", "")).lower() not in {"completed", "blocked"}:
        failures.append("free_state_loop_not_terminal")
    return {
        "case_id": truth["case_id"],
        "suite": truth["suite"],
        "issue_kind": truth["issue_kind"],
        "prompt": truth["blind_prompt"],
        "expected_first_processors": expected_first,
        "actual_processor_sequence": actual,
        "planned_processor_sequence": planned,
        "free_state_status": str(loop.get("status", "")),
        "free_state_cycle_count": int(loop.get("cycle") or 0),
        "free_state_original_intent_retained": original_intent_retained,
        "first_route_observable": route_observable,
        "first_route_correct": first_correct,
        "expected_eventual_processors": expected_eventual,
        "eventual_processor_coverage": eventual_coverage,
        "load_confirmation_count": load_count,
        "parameter_confirmation_count": parameter_count,
        "action_confirmation_count": action_count,
        "confirmation_boundary_pass": confirmation_boundary,
        "acoustic_result": acoustic,
        "execution_result": execution,
        "failure_taxonomy": failures,
        "result": "pass" if not failures else "observed_failure",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-report", required=True)
    parser.add_argument("--sealed-truth", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    output = Path(args.output).resolve()
    try:
        run = json.loads(Path(args.run_report).resolve().read_text(encoding="utf-8"))
        truth = json.loads(Path(args.sealed_truth).resolve().read_text(encoding="utf-8"))
        if run.get("status") != "completed":
            raise RuntimeError("blind experiment did not complete")
        if run.get("fixture_set_id") != truth.get("set_id"):
            raise RuntimeError("run report and sealed truth fixture IDs differ")
        by_case = {str(row.get("case_id")): row for row in rows(run.get("cases"))}
        results = [evaluate_case(by_case[str(row["case_id"])], row) for row in rows(truth.get("cases")) if str(row.get("case_id")) in by_case]
        taxonomy: dict[str, int] = {}
        for result in results:
            for failure in result["failure_taxonomy"]:
                taxonomy[failure] = taxonomy.get(failure, 0) + 1
        report = {
            "schema_version": SCHEMA_VERSION,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "status": "completed",
            "fixture_set_id": truth.get("set_id"),
            "case_count": len(results),
            "pass_count": sum(result["result"] == "pass" for result in results),
            "observed_failure_count": sum(result["result"] != "pass" for result in results),
            "failure_taxonomy": taxonomy,
            "cases": results,
        }
        write_json(output, report)
        print(json.dumps({key: report[key] for key in ("status", "case_count", "pass_count", "observed_failure_count", "failure_taxonomy")}, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        write_json(output, {"schema_version": SCHEMA_VERSION, "status": "failed", "error": str(exc)})
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
