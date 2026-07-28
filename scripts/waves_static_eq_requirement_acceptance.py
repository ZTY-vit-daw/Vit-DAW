#!/usr/bin/env python3
"""Requirement-level acceptance audit for the static-EQ phase-two design."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any


SHAPES = {"bell", "low_shelf", "high_shelf", "low_cut", "high_cut"}
ACTIONS = {"upsert", "modify", "disable", "remove", "undo"}
STATUSES = {"exact", "quantized", "rejected"}


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{path} is not an object")
    return value


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def hash_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest().upper()


def forbidden_identity_paths(value: Any, path: str = "$") -> list[str]:
    forbidden = {
        "plugin_name", "manufacturer", "plugin_identifier", "identifier",
        "case_id", "section_ref_seed", "structural_key",
    }
    found: list[str] = []
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = f"{path}.{key}"
            if key.casefold() in forbidden:
                found.append(child_path)
            found.extend(forbidden_identity_paths(child, child_path))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            found.extend(forbidden_identity_paths(child, f"{path}[{index}]"))
    return found


class Audit:
    def __init__(self) -> None:
        self.rows: list[dict[str, Any]] = []

    def check(self, requirement: str, passed: bool, evidence: Any) -> None:
        self.rows.append({
            "requirement": requirement,
            "status": "passed" if passed else "failed",
            "evidence": evidence,
        })

    @property
    def passed(self) -> bool:
        return all(row["status"] == "passed" for row in self.rows)


def csv_rows(path: Path) -> list[dict[str, str]]:
    with path.open("r", encoding="utf-8-sig", newline="") as handle:
        return list(csv.DictReader(handle))


def find(rows: list[dict[str, str]], case_id: str, key: str, value: str) -> dict[str, str]:
    for row in rows:
        if row.get("case_id") == case_id and row.get(key) == value:
            return row
    return {}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--report", required=True)
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_dir = Path(args.run_dir).resolve()
    output_dir = run_dir / "analysis" / "static_eq_phase2"
    corpus = read_json(Path(args.corpus).resolve())
    summary = read_json(output_dir / "analysis_summary.json")
    provenance = read_json(output_dir / "analysis_provenance.json")
    partitions = read_json(output_dir / "exact_plan_partitions.json")
    minimality = read_json(output_dir / "addressing_minimality_proof.json")
    catalog = read_json(output_dir / "control_plan_catalog.json")
    rejection_codes = read_json(output_dir / "rejection_codes.json")
    phase1_manifest = read_json(run_dir / "run_manifest.json")
    report_path = Path(args.report).resolve()
    report_text = report_path.read_text(encoding="utf-8")
    matrix = csv_rows(output_dir / "shape_action_matrix.csv")
    intents = csv_rows(output_dir / "canonical_intent_matrix.csv")
    model_files = sorted((output_dir / "models").glob("*.json"))
    models = [read_json(path) for path in model_files]
    audit = Audit()

    scope = corpus.get("scope") or {}
    audit.check(
        "control corpus is exactly five generic static shapes and five actions",
        set(scope.get("included_shapes", [])) == SHAPES
        and set(scope.get("included_actions", [])) == ACTIONS
        and scope.get("atomic_batches") is True
        and scope.get("allow_missing_explicit_fields") is False,
        {"shapes": scope.get("included_shapes"), "actions": scope.get("included_actions"),
         "atomic_batches": scope.get("atomic_batches")})

    runtime_calls = provenance.get("runtime_calls") or {}
    audit.check(
        "phase two is an offline replay of 50 accepted phase-one signatures with zero runtime calls",
        provenance.get("method") == "offline_symbolic_replay_of_phase1_signatures"
        and provenance.get("input_signature_count") == 50
        and provenance.get("phase1_acceptance_status") == "passed"
        and runtime_calls and all(value == 0 for value in runtime_calls.values()),
        {"method": provenance.get("method"),
         "input_signature_count": provenance.get("input_signature_count"),
         "phase1_acceptance_status": provenance.get("phase1_acceptance_status"),
         "runtime_calls": runtime_calls})

    expected_keys = {(str(model["case_id"]), shape, action)
                     for model in models for shape in SHAPES for action in ACTIONS}
    actual_keys = {(row["case_id"], row["shape"], row["action"]) for row in matrix}
    audit.check(
        "50 Waves models have exactly one row for every shape and action",
        len(models) == 50 and len(matrix) == 1250
        and len(actual_keys) == 1250 and actual_keys == expected_keys,
        {"model_count": len(models), "row_count": len(matrix),
         "unique_key_count": len(actual_keys), "missing_count": len(expected_keys - actual_keys)})

    invalid_status = [row for row in matrix if row["status"] not in STATUSES]
    missing_reasons = [row for row in matrix
                       if row["status"] == "rejected" and not row["reasons"]]
    supported_with_reasons = [row for row in matrix
                              if row["status"] != "rejected" and row["reasons"]]
    audit.check(
        "matrix uses only exact/quantized/rejected and every rejection is explicit",
        not invalid_status and not missing_reasons and not supported_with_reasons,
        {"invalid_status_count": len(invalid_status),
         "rejected_without_reason_count": len(missing_reasons),
         "supported_with_rejection_reason_count": len(supported_with_reasons),
         "status_counts": summary.get("shape_action_statuses")})

    expected_intent_count = len(models) * len(corpus.get("canonical_intents", []))
    intent_keys = {(row["case_id"], row["intent_id"]) for row in intents}
    audit.check(
        "every canonical natural-language intent is replayed against every model",
        len(intents) == expected_intent_count == 550 and len(intent_keys) == len(intents),
        {"intent_template_count": len(corpus.get("canonical_intents", [])),
         "row_count": len(intents), "unique_key_count": len(intent_keys),
         "status_counts": summary.get("canonical_intent_statuses")})

    def matrix_row(case_id: str, shape: str, action: str = "upsert") -> dict[str, str]:
        for row in matrix:
            if row["case_id"] == case_id and row["shape"] == shape and row["action"] == action:
                return row
        return {}

    emo_low = matrix_row("emo_f2_stereo", "low_cut")
    emo_high = matrix_row("emo_f2_stereo", "high_cut")
    emo_bell = matrix_row("emo_f2_stereo", "bell")
    audit.check(
        "filter-only EMO-F2 recovers both cuts without inventing a gain band",
        emo_low.get("status") == "exact" and emo_high.get("status") == "exact"
        and emo_bell.get("status") == "rejected"
        and "required_gain_binding_absent" in emo_bell.get("reasons", ""),
        {"low_cut": emo_low, "high_cut": emo_high, "bell": emo_bell})

    special_checks = {
        "f6_dynamic": matrix_row("f6_stereo", "bell").get("reasons", ""),
        "puigtec_coupled": matrix_row("puigtec_eqp1a_stereo", "bell").get("reasons", ""),
        "tract_stateful": matrix_row("tract_stereo", "bell").get("reasons", ""),
        "curves_stateful": matrix_row("curves_aq_stereo", "bell").get("reasons", ""),
        "q_clone": matrix_row("q_clone_stereo", "bell").get("reasons", ""),
    }
    audit.check(
        "dynamic, coupled analog, stateful, and opaque special structures fail closed",
        "dynamic_section_excluded" in special_checks["f6_dynamic"]
        and "coupled_analog_network_excluded" in special_checks["puigtec_coupled"]
        and "stateful_surface_dependency_unresolved" in special_checks["tract_stateful"]
        and "stateful_surface_dependency_unresolved" in special_checks["curves_stateful"]
        and "no_candidate_section" in special_checks["q_clone"],
        special_checks)

    intent_lookup = {(row["case_id"], row["intent_id"]): row for row in intents}
    strict_cases = {
        "q10_bell_q": intent_lookup[("q10_stereo", "bell_q_smoke")],
        "api_bell_q": intent_lookup[("api_550a_stereo", "bell_q_smoke")],
        "q10_cut_slope": intent_lookup[("q10_stereo", "low_cut_slope_smoke")],
        "ssl_ev2_high_cut": intent_lookup[("ssl_ev2_channel_stereo", "high_cut_basic")],
    }
    audit.check(
        "explicit Q/slope and physical ranges are strict while reachable requests succeed",
        strict_cases["q10_bell_q"]["status"] == "exact"
        and strict_cases["api_bell_q"]["status"] == "rejected"
        and "required_q_binding_absent" in strict_cases["api_bell_q"]["reasons"]
        and strict_cases["q10_cut_slope"]["status"] == "rejected"
        and "required_slope_binding_absent" in strict_cases["q10_cut_slope"]["reasons"]
        and strict_cases["ssl_ev2_high_cut"]["status"] == "rejected"
        and "frequency_out_of_reachable_domain" in strict_cases["ssl_ev2_high_cut"]["reasons"],
        strict_cases)

    remove_supported = [row for row in matrix if row["action"] == "remove"
                        and row["status"] != "rejected"]
    action_mismatches: list[dict[str, str]] = []
    for model in models:
        for shape in SHAPES:
            upsert = matrix_row(str(model["case_id"]), shape, "upsert")
            modify = matrix_row(str(model["case_id"]), shape, "modify")
            undo = matrix_row(str(model["case_id"]), shape, "undo")
            if upsert["status"] != modify["status"]:
                action_mismatches.append({"case_id": str(model["case_id"]), "shape": shape,
                                          "upsert": upsert["status"], "modify": modify["status"]})
            expected_undo = "exact" if upsert["status"] != "rejected" else "rejected"
            if undo["status"] != expected_undo:
                action_mismatches.append({"case_id": str(model["case_id"]), "shape": shape,
                                          "upsert": upsert["status"], "undo": undo["status"]})
    audit.check(
        "modify preserves forward capability, undo is journal-conditioned, and no Waves section is removable",
        not action_mismatches and not remove_supported,
        {"action_mismatches": action_mismatches,
         "remove_supported_count": len(remove_supported)})

    witnesses = minimality.get("pairwise_witnesses", [])
    audit.check(
        "anchored/resident/allocatable form three pairwise-distinguishable addressing partitions",
        minimality.get("minimal") is True
        and minimality.get("partition_count") == 3
        and len(witnesses) == 3
        and all(row.get("merge_forbidden") and row.get("distinguishing_observables")
                for row in witnesses)
        and minimality.get("waves_allocatable_observation_count") == 0
        and minimality.get("allocatable_evidence_source") == "identity_free_control_requirement_prototype",
        {"partition_count": minimality.get("partition_count"),
         "minimal": minimality.get("minimal"), "witnesses": witnesses,
         "waves_allocatable_observation_count": minimality.get("waves_allocatable_observation_count")})

    catalog_issues: list[dict[str, Any]] = []
    for row in catalog.get("plans", []):
        plan = row.get("control_plan_signature")
        forbidden = forbidden_identity_paths(plan)
        if forbidden:
            catalog_issues.append({"signature": row.get("control_plan_signature_sha256"),
                                   "issue": "identity keys", "paths": forbidden})
        if hash_json(plan) != row.get("control_plan_signature_sha256"):
            catalog_issues.append({"signature": row.get("control_plan_signature_sha256"),
                                   "issue": "hash mismatch"})
    audit.check(
        "all control-plan signatures are reproducible and identity-free",
        catalog.get("identity_features_used") is False
        and catalog.get("unique_plan_count") == len(catalog.get("plans", []))
        and not catalog_issues,
        {"candidate_program_count": catalog.get("candidate_program_count"),
         "unique_plan_count": catalog.get("unique_plan_count"),
         "issues": catalog_issues})

    final_partition_count = (partitions.get("trace") or [{}])[-1].get("partition_count")
    partition_members = [digest for row in partitions.get("partitions", [])
                         for digest in row.get("member_plan_signatures", [])]
    audit.check(
        "exact partition refinement covers each unique behavior signature once",
        partitions.get("method") == "exact_partition_refinement"
        and partitions.get("identity_features_used") is False
        and final_partition_count == partitions.get("partition_count")
        and len(partition_members) == len(set(partition_members))
        == partitions.get("unique_input_plan_count"),
        {"method": partitions.get("method"), "trace": partitions.get("trace"),
         "partition_count": partitions.get("partition_count"),
         "member_count": len(partition_members)})

    policy_issues = [model["case_id"] for model in models
                     if (model.get("inference_policy") or {}).get("plugin_identity_used_as_feature") is not False
                     or (model.get("inference_policy") or {}).get("production_recognizer_used") is not False
                     or (model.get("inference_policy") or {}).get("parameter_writes_used") is not False]
    audit.check(
        "all 50 model analyses declare section/request inference with no identity, production recognizer, or writes",
        len(model_files) == 50 and not policy_issues,
        {"model_file_count": len(model_files), "policy_issues": policy_issues})

    missing_names = [str(model["plugin_name"]) for model in models
                     if str(model["plugin_name"]) not in report_text]
    artifact_report = output_dir / "WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md"
    audit.check(
        "design report contains every model, the required contracts, and the production stop boundary",
        not missing_names and "control_plan_signature" in report_text
        and "control_ref" in report_text and "operation_ref" in report_text
        and "原子批量事务" in report_text and "停止点" in report_text
        and "未经用户再次确认，不开始生产开发" in report_text
        and artifact_report.read_bytes() == report_path.read_bytes(),
        {"report": str(report_path), "missing_model_names": missing_names,
         "artifact_copy_matches": artifact_report.read_bytes() == report_path.read_bytes()})

    hash_issues: list[dict[str, Any]] = []
    for filename, expected in (phase1_manifest.get("production_eq_after") or {}).items():
        path = Path(filename)
        actual = file_sha256(path)
        if actual != str(expected).upper():
            hash_issues.append({"path": str(path), "expected": expected, "actual": actual})
    production_status = subprocess.run(
        ["git", "-C", str(repo), "status", "--porcelain", "--untracked-files=all",
         "--", "agent", "VitApp/Source"], check=True, capture_output=True,
        text=True).stdout.splitlines()
    audit.check(
        "protected production EQ hashes and all Agent/VitApp production sources remain unchanged",
        not hash_issues and not production_status,
        {"hash_issues": hash_issues, "production_status": production_status})

    required_codes = {
        "shape_not_provably_reachable", "required_q_binding_absent",
        "required_slope_binding_absent", "coupled_analog_network_excluded",
        "dynamic_section_excluded", "stateful_surface_dependency_unresolved",
        "activation_binding_unavailable", "section_not_deallocatable",
        "valid_control_ref_required", "valid_operation_ref_required",
    }
    codes = set((rejection_codes.get("codes") or {}).keys())
    audit.check(
        "stable rejection catalog covers shape, field, special-structure, lifecycle, and reference failures",
        required_codes <= codes and all((rejection_codes.get("codes") or {}).values()),
        {"code_count": len(codes), "missing_required_codes": sorted(required_codes - codes)})

    protection = {
        "schema_version": "static_eq.phase2_source_protection.v1",
        "phase1_expected_hashes": phase1_manifest.get("production_eq_after"),
        "current_hashes": {filename: file_sha256(Path(filename))
                           for filename in (phase1_manifest.get("production_eq_after") or {})},
        "hash_issues": hash_issues,
        "production_status": production_status,
    }
    write_path = output_dir / "production_protection.json"
    write_path.write_text(json.dumps(protection, ensure_ascii=False, indent=2), encoding="utf-8")

    payload = {
        "schema_version": "static_eq.phase2_acceptance.v1",
        "status": "passed" if audit.passed else "failed",
        "requirement_count": len(audit.rows),
        "passed_count": sum(row["status"] == "passed" for row in audit.rows),
        "failed_count": sum(row["status"] == "failed" for row in audit.rows),
        "requirements": audit.rows,
    }
    output = output_dir / "phase2_acceptance.json"
    output.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"acceptance={payload['status']} passed={payload['passed_count']}/{payload['requirement_count']}")
    print(f"evidence={output}")
    return 0 if audit.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
