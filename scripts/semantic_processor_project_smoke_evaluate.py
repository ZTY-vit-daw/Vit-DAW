#!/usr/bin/env python3
"""Evaluate a closed smoke run against sealed truth.

This is intentionally separate from the runner.  It is the only component in
this preparation package that opens ``sealed_truth.json``.
"""

from __future__ import annotations

import argparse
import json
import os
import time
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "semantic_processor_agent_project_smoke_evaluation.v1"
EXPECTED_FAMILIES = {
    "static_eq", "broadband_compressor", "limiter", "gate_expander",
    "de_esser", "transient_shaper", "multiband_dynamics",
}
EVIDENCE_FIELDS = (
    "model_observation_receipts", "semantic_intent_artifacts", "candidate_sets",
    "model_selected_identifiers", "pca_preload_receipts", "pca_postload_receipts",
    "typed_controller_receipts", "confirmation_receipts", "transaction_receipts",
    "parameter_readbacks", "snapshot_verifications", "rollback_receipts",
    "post_action_observation_receipts", "ab_results",
)
FAMILY_ALIASES = {
    "eq": "static_eq", "static_eq": "static_eq",
    "compressor": "broadband_compressor", "broadband_compressor": "broadband_compressor",
    "limiter": "limiter",
    "gate": "gate_expander", "expander": "gate_expander", "gate_expander": "gate_expander",
    "deesser": "de_esser", "de-esser": "de_esser", "de_esser": "de_esser",
    "transient": "transient_shaper", "transient_shaper": "transient_shaper",
    "multiband": "multiband_dynamics", "multiband_dynamics": "multiband_dynamics",
}


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def atomic_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def load(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError(f"expected JSON object: {path}")
    return value


def map_value(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def row_values(value: Any) -> list[dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    if isinstance(value, dict):
        return [value]
    return []


def first_text(value: Any, *keys: str) -> str:
    row = map_value(value)
    for key in keys:
        item = row.get(key)
        if isinstance(item, str) and item.strip():
            return item.strip()
    return ""


def normalize_family(value: Any) -> str:
    token = str(value or "").strip().lower()
    return FAMILY_ALIASES.get(token, token if token in EXPECTED_FAMILIES else "")


def decision_records(response: dict[str, Any]) -> list[dict[str, str]]:
    """Extract only model decision surfaces, never receipt payloads."""
    roots: list[dict[str, Any]] = [response]
    for key in ("free_state", "decision", "latest_decision", "semantic_processor_intent"):
        row = map_value(response.get(key))
        if row:
            roots.append(row)
    workflow = map_value(response.get("workflow_data"))
    loop = map_value(workflow.get("free_state_reasoning_loop"))
    latest = map_value(loop.get("latest_decision"))
    if latest:
        roots.append(latest)
    records: list[dict[str, str]] = []
    seen: set[tuple[str, str]] = set()
    for root in roots:
        intent = map_value(root.get("semantic_processor_intent"))
        family = normalize_family(first_text(intent, "family", "processor_family", "selected_family", "processor_type"))
        if not family:
            family = normalize_family(first_text(root, "family", "processor_family", "selected_family", "processor_type"))
        target_row = map_value(root.get("target_ref"))
        target = first_text(root, "target_track", "track_id", "target") or first_text(target_row, "id", "track_id", "target_track")
        if not target:
            target = first_text(intent, "target_track", "track_id", "target")
        if not family and not target:
            continue
        key = (family, target)
        if key in seen:
            continue
        seen.add(key)
        records.append({"family": family, "target": target})
    return records


def count_value(value: Any) -> int:
    if isinstance(value, list):
        return len(value)
    if isinstance(value, dict):
        return 1
    if isinstance(value, (int, float)) and value >= 0:
        return int(value)
    return 0


def evidence_counts(response: dict[str, Any]) -> dict[str, int]:
    sources = [response, map_value(response.get("execution_evidence")), map_value(response.get("free_state"))]
    workflow = map_value(response.get("workflow_data"))
    loop = map_value(workflow.get("free_state_reasoning_loop"))
    latest = map_value(loop.get("latest_decision"))
    if latest:
        sources.append(latest)
    counts = {key: 0 for key in EVIDENCE_FIELDS}
    for key in EVIDENCE_FIELDS:
        for source in sources:
            if key in source:
                counts[key] = max(counts[key], count_value(source[key]))
            nested_counts = map_value(source.get("counts"))
            if key in nested_counts:
                counts[key] = max(counts[key], count_value(nested_counts[key]))
    replies = response.get("executed_kernel_reply")
    if isinstance(replies, list):
        for receipt in replies:
            if not isinstance(receipt, dict) or str(receipt.get("status", "")).strip().lower() not in {"ok", "success", "completed"}:
                continue
            tool = str(receipt.get("tool") or receipt.get("command_name") or "").strip().lower()
            if tool in {"ccb.observation_request", "ccb_observation_request"}:
                counts["model_observation_receipts"] += 1
            elif "candidate" in tool:
                counts["candidate_sets"] += 1
            elif "pca" in tool:
                counts["pca_preload_receipts"] += 1
            elif "confirm" in tool:
                counts["confirmation_receipts"] += 1
            elif "readback" in tool:
                counts["parameter_readbacks"] += 1
            elif "snapshot" in tool or "verify" in tool:
                counts["snapshot_verifications"] += 1
            elif "rollback" in tool or "undo" in tool:
                counts["rollback_receipts"] += 1
            elif "apply" in tool or "transaction" in tool or "control" in tool:
                counts["transaction_receipts"] += 1
    return counts


def receipt_rows(response: dict[str, Any], field: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for source in (response, map_value(response.get("execution_evidence")), map_value(response.get("free_state"))):
        value = source.get(field)
        rows.extend(row_values(value))
    return rows


def receipt_validation_failures(response: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    failed_statuses = {"failed", "error", "rejected", "blocked", "not_eligible", "ineligible"}
    for field in EVIDENCE_FIELDS:
        for row in receipt_rows(response, field):
            status = first_text(row, "status", "result", "outcome").lower()
            if status in failed_statuses:
                failures.append(f"{field}:{status}")
            for boolean_key in ("ok", "passed", "eligible", "qualified", "confirmed", "applied", "readback_ok", "snapshot_ok", "restored", "same_tap_render_mode"):
                if boolean_key in row and row[boolean_key] is False:
                    failures.append(f"{field}:{boolean_key}=false")
            if field == "ab_results":
                before = first_text(row, "before_render_revision", "before_revision")
                after = first_text(row, "after_render_revision", "after_revision")
                if before and after and before == after:
                    failures.append("ab_results:render_revision_unchanged")
    # Candidate disclosure and exact model selection are separate evidence
    # surfaces. If both expose identifiers, membership must be byte-exact.
    candidate_ids: set[str] = set()
    selected_ids: set[str] = set()
    for row in receipt_rows(response, "candidate_sets"):
        for key in ("identifier", "candidate_identifier", "plugin_id", "instance_id", "id"):
            value = row.get(key)
            if isinstance(value, str) and value.strip():
                candidate_ids.add(value)
        for nested in row_values(row.get("candidates")):
            for key in ("identifier", "candidate_identifier", "plugin_id", "instance_id", "id"):
                value = nested.get(key)
                if isinstance(value, str) and value.strip():
                    candidate_ids.add(value)
    for row in receipt_rows(response, "model_selected_identifiers"):
        for key in ("identifier", "candidate_identifier", "plugin_id", "instance_id", "id", "selected_identifier"):
            value = row.get(key)
            if isinstance(value, str) and value.strip():
                selected_ids.add(value)
    if candidate_ids and selected_ids and not selected_ids.issubset(candidate_ids):
        failures.append("model_selected_identifiers:not_a_disclosed_candidate")
    return failures


def load_response_rows(project: dict[str, Any]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    evidence = map_value(project.get("execution_evidence"))
    for raw_path in evidence.get("response_artifacts", []):
        try:
            path = Path(str(raw_path)).resolve()
            value = load(path)
            rows.append(value)
        except (OSError, ValueError, json.JSONDecodeError):
            continue
    return rows


def checkpoint_state_audit(project: dict[str, Any]) -> tuple[bool, list[str]]:
    path_text = str(project.get("checkpoint_history") or "").strip()
    if not path_text:
        return False, ["checkpoint_history_missing"]
    try:
        checkpoint = load(Path(path_text).resolve())
    except (OSError, ValueError, json.JSONDecodeError):
        return False, ["checkpoint_history_unreadable"]
    history = checkpoint.get("history")
    states = [str(row.get("state")) for row in history if isinstance(row, dict) and str(row.get("state", "")).strip()] if isinstance(history, list) else []
    required = (
        "model_observing", "candidate_query", "model_identifier_selection",
        "preload_pca_recheck", "postload_qualification", "control_planning",
        "parameter_confirmation", "typed_execution", "readback_and_snapshot_verification",
        "model_post_action_observation", "model_outcome", "project_terminal",
    )
    cursor = 0
    missing: list[str] = []
    for wanted in required:
        try:
            cursor = states.index(wanted, cursor) + 1
        except ValueError:
            missing.append(wanted)
    return not missing, missing


def project_evidence(project: dict[str, Any], responses: list[dict[str, Any]]) -> dict[str, int]:
    counts = {key: 0 for key in EVIDENCE_FIELDS}
    for response in responses:
        current = evidence_counts(response)
        for key in EVIDENCE_FIELDS:
            counts[key] += current[key]
    report_counts = map_value(project.get("execution_evidence")).get("counts")
    if isinstance(report_counts, dict):
        for key in EVIDENCE_FIELDS:
            counts[key] = max(counts[key], count_value(report_counts.get(key)))
    return counts


def has_governed_full_path(counts: dict[str, int], state_chain_ok: bool) -> bool:
    return all(counts.get(key, 0) > 0 for key in EVIDENCE_FIELDS) and state_chain_ok


def infrastructure_failure_kind(run: dict[str, Any]) -> str:
    """Classify an infrastructure stop from the runner's explicit scope."""
    failures = run.get("infrastructure_failures")
    if not isinstance(failures, list):
        failures = []
    scopes = {str(row.get("scope", "")).strip().lower() for row in failures if isinstance(row, dict)}
    if "agent_model_service" in scopes:
        return "agent_model_service_failure"
    if "transport" in scopes:
        return "transport_failure"
    if "project_setup" in scopes:
        return "project_setup_failure"
    return "infrastructure_failure"


def explicit_conformance_violations(project: dict[str, Any], responses: list[dict[str, Any]]) -> list[Any]:
    violations: list[Any] = []
    for source in [project, *responses]:
        for key in ("conformance_violations", "agent_conformance_failures", "violations"):
            value = source.get(key)
            if isinstance(value, list):
                violations.extend(value)
            elif value:
                violations.append(value)
    for response in responses:
        violations.extend(receipt_validation_failures(response))
    return violations


def evaluate(args: argparse.Namespace) -> dict[str, Any]:
    report_path = Path(args.run_report).resolve()
    sealed_path = Path(args.sealed_truth).resolve()
    run = load(report_path)
    if run.get("schema_version") != "semantic_processor_agent_project_smoke_run_report.v1":
        raise ValueError("run report schema mismatch")
    if not run.get("ended_at"):
        raise ValueError("run report is not atomically closed")
    if run.get("sealed_truth_opened_by_runner") is not False:
        raise ValueError("runner sealed-truth boundary was violated")
    sealed = load(sealed_path)
    if sealed.get("schema_version") != "semantic_processor_agent_project_smoke_sealed_truth.v1":
        raise ValueError("sealed truth schema mismatch")
    if sealed.get("fixture_set_id") != run.get("fixture_set_id"):
        raise ValueError("fixture set mismatch between run and sealed truth")

    project_rows = {str(row.get("public_case_id")): row for row in run.get("projects", []) if isinstance(row, dict)}
    issues: list[dict[str, Any]] = []
    issue_projects: dict[str, str] = {}
    preflight_blocked = str(run.get("status")) == "preflight_blocked"
    infrastructure_blocked = str(run.get("status")) == "infrastructure_failure"
    infrastructure_kind = infrastructure_failure_kind(run) if infrastructure_blocked else ""
    not_executed = str(run.get("status")) in {"preflight_blocked", "preflight_passed_not_executed"}
    for case in sealed.get("cases", []):
        if not isinstance(case, dict):
            continue
        case_id = str(case.get("public_case_id"))
        project = project_rows.get(case_id, {})
        responses = load_response_rows(project)
        decisions = [record for response in responses for record in decision_records(response)]
        counts = project_evidence(project, responses)
        state_chain_ok, state_chain_missing = checkpoint_state_audit(project)
        violations = explicit_conformance_violations(project, responses)
        project_outcome = str((project.get("model_outcomes") or ["inconclusive"])[-1])
        for issue in case.get("issue_assignments", []):
            if not isinstance(issue, dict):
                continue
            issue_id = str(issue.get("issue_id"))
            expected = normalize_family(issue.get("expected_family"))
            expected_target = str(issue.get("expected_target_track"))
            issue_projects[issue_id] = case_id
            target_decisions = [row for row in decisions if row.get("target") == expected_target]
            selected_families = sorted({row.get("family") for row in target_decisions if row.get("family")})
            selected_targets = sorted({row.get("target") for row in target_decisions if row.get("target")})
            governed = has_governed_full_path(counts, state_chain_ok) and not violations
            if preflight_blocked or infrastructure_blocked:
                classification = "infrastructure_failure"
                model_outcome = "inconclusive"
                conformance = "unobservable"
                failure_taxonomy = {
                    "kind": infrastructure_kind if infrastructure_blocked else "preflight_blocked",
                    "evidence": run.get("infrastructure_failures", []) if infrastructure_blocked else run.get("preflight", {}).get("blockers", []),
                }
            elif not_executed:
                classification = "not_exercised_by_model"
                model_outcome = "inconclusive"
                conformance = "unobservable"
                failure_taxonomy = {"kind": "formal_run_not_requested", "preflight_status": run.get("preflight", {}).get("status", "unknown")}
            elif not target_decisions:
                model_outcome = project_outcome
                if model_outcome == "model_no_op":
                    classification = "model_no_op"
                    failure_taxonomy = {"kind": "no_mutation_after_model_observation"}
                elif model_outcome == "model_blocked":
                    classification = "model_blocked"
                    failure_taxonomy = {"kind": "governed_or_evidence_block", "evidence": project.get("execution_evidence", {})}
                else:
                    classification = "not_exercised_by_model"
                    failure_taxonomy = {"kind": "expected_target_not_selected", "expected_target_track": expected_target}
                conformance = "pass" if model_outcome in {"model_no_op", "model_blocked"} and not violations else "unobservable"
            elif project_outcome == "model_blocked" and not governed:
                model_outcome = "model_blocked"
                classification = "model_blocked"
                conformance = "pass" if not violations else "fail"
                failure_taxonomy = {"kind": "governed_or_evidence_block", "evidence": project.get("execution_evidence", {}), "violations": violations}
            elif not governed:
                model_outcome = project_outcome if project_outcome in {"satisfied", "model_no_op", "model_blocked"} else "inconclusive"
                classification = "agent_conformance_failure"
                conformance = "fail"
                failure_taxonomy = {"kind": "required_execution_evidence_missing_or_invalid", "missing": [key for key in EVIDENCE_FIELDS if counts.get(key, 0) <= 0], "state_chain_missing": state_chain_missing, "violations": violations}
            else:
                model_outcome = project_outcome if project_outcome in {"satisfied", "model_no_op", "model_blocked"} else "satisfied"
                conformance = "pass"
                if expected in selected_families:
                    classification = "matched_expected_family"
                    failure_taxonomy = {"kind": "governed_full_path"}
                else:
                    classification = "evaluator_disagreement"
                    failure_taxonomy = {"kind": "model_selected_alternative_family", "selected_families": selected_families}
            issues.append({
                "issue_id": issue_id,
                "expected_family": expected,
                "expected_target_track": expected_target,
                "model_selected_families": selected_families,
                "model_selected_targets": selected_targets,
                "model_outcome": model_outcome,
                "agent_conformance": conformance,
                "classification": classification,
                "execution_evidence_summary": dict(counts),
                "failure_taxonomy": failure_taxonomy,
            })

    family_counts = {family: sum(1 for issue in issues if issue["expected_family"] == family and issue["classification"] == "matched_expected_family") for family in sorted(EXPECTED_FAMILIES)}
    classifications = {}
    for issue in issues:
        classifications[issue["classification"]] = classifications.get(issue["classification"], 0) + 1
    agent_pass = all(issue["agent_conformance"] == "pass" for issue in issues) and not preflight_blocked and not infrastructure_blocked
    project_reports = []
    for case_id, project in project_rows.items():
        project_reports.append({
            "public_case_id": case_id,
            "issues": [issue for issue in issues if issue_projects.get(issue.get("issue_id")) == case_id],
            "response_artifact_count": len(load_response_rows(project)),
        })
    evaluation = {
        "schema_version": SCHEMA_VERSION,
        "contract_id": run.get("contract_id"),
        "fixture_set_id": run.get("fixture_set_id"),
        "run_id": run.get("run_id"),
        "status": "preflight_blocked" if preflight_blocked else ("infrastructure_failure" if infrastructure_blocked else ("not_executed" if not_executed else "evaluated")),
        "evaluated_at": now_iso(),
        "agent_conformance_summary": {"status": "unobservable" if not_executed or infrastructure_blocked else ("pass" if agent_pass else "fail"), "passed": agent_pass, "reason": "formal run did not start" if not_executed else (infrastructure_kind if infrastructure_blocked else "derived from execution evidence")},
        "model_outcome_summary": {"inconclusive": len(issues)} if not_executed or infrastructure_blocked else {},
        "family_coverage": {"status": "none_exercised", "expected_family_counts": family_counts, "full_path_families": []},
        "case_classification_counts": classifications,
        "projects": project_reports,
        "issues": issues,
        "mixing_layer_readiness": {"status": "blocked", "reasons": ["preflight_blocked", "no family full paths"] if preflight_blocked else ([infrastructure_kind, "no family full paths"] if infrastructure_blocked else ["family full paths incomplete"])},
        "sealed_truth_access": {"evaluator_opened_after_closed_report": True, "runner_opened": False, "sealed_truth_path": str(sealed_path)},
    }
    output = Path(args.output or report_path.with_name("evaluation_report.json")).resolve()
    atomic_json(output, evaluation)
    return {"status": evaluation["status"], "output": str(output), "case_classification_counts": classifications, "agent_conformance": evaluation["agent_conformance_summary"]}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-report", required=True)
    parser.add_argument("--sealed-truth", required=True)
    parser.add_argument("--output", default="")
    args = parser.parse_args()
    try:
        print(json.dumps(evaluate(args), ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
