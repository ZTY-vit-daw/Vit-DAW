#!/usr/bin/env python3
"""Requirement-by-requirement acceptance audit for the Waves topology census."""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import subprocess
from pathlib import Path
from typing import Any


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{path} is not an object")
    return value


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":"))


def hash_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def forbidden_identity_keys(value: Any, path: str = "$") -> list[str]:
    forbidden = {"plugin_name", "manufacturer", "plugin_identifier", "identifier"}
    found: list[str] = []
    if isinstance(value, dict):
        for key, nested in value.items():
            child = f"{path}.{key}"
            if key.casefold() in forbidden:
                found.append(child)
            found.extend(forbidden_identity_keys(nested, child))
    elif isinstance(value, list):
        for index, nested in enumerate(value):
            found.extend(forbidden_identity_keys(nested, f"{path}[{index}]"))
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--run-dir", required=True)
    parser.add_argument("--fixture", required=True)
    parser.add_argument("--report", required=True)
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_dir = Path(args.run_dir).resolve()
    raw = run_dir / "raw"
    analysis = run_dir / "analysis"
    fixture = read_json(Path(args.fixture))
    summary = read_json(raw / "census_summary.json")
    manifest = read_json(run_dir / "run_manifest.json")
    analysis_summary = read_json(analysis / "analysis_summary.json")
    clusters = read_json(analysis / "clusters.json")
    report_path = Path(args.report).resolve()
    report_text = report_path.read_text(encoding="utf-8")
    audit = Audit()

    cases = [row for row in fixture.get("cases", []) if isinstance(row, dict)]
    results = [row for row in summary.get("results", []) if isinstance(row, dict)]
    audit.check("fixture scope is exactly 38 EQ + 12 Channel Strip Stereo cases",
                len(cases) == 50
                and sum(row.get("surface_kind") == "eq" for row in cases) == 38
                and sum(row.get("surface_kind") == "channel_strip" for row in cases) == 12,
                {"total": len(cases),
                 "eq": sum(row.get("surface_kind") == "eq" for row in cases),
                 "channel_strip": sum(row.get("surface_kind") == "channel_strip" for row in cases)})
    audit.check("all 50 targets have an explicit successful capture state",
                len(results) == 50 and all(row.get("status") == "captured" for row in results),
                {"result_count": len(results),
                 "captured": sum(row.get("status") == "captured" for row in results),
                 "failed": sum(row.get("status") != "captured" for row in results)})

    capture_issues: list[dict[str, Any]] = []
    manufacturer_counts: dict[str, int] = {}
    total_parameters = 0
    total_pages = 0
    for case in cases:
        case_id = str(case["id"])
        path = raw / "captures" / f"{case_id}.json"
        if not path.exists():
            capture_issues.append({"case_id": case_id, "issue": "capture missing"})
            continue
        capture = read_json(path)
        pagination = capture.get("pagination") or {}
        resolution = capture.get("plugin_resolution") or {}
        manufacturer = str(resolution.get("manufacturer", ""))
        manufacturer_counts[manufacturer] = manufacturer_counts.get(manufacturer, 0) + 1
        actual = len(capture.get("parameters", []))
        expected = pagination.get("expected_total")
        total_parameters += actual
        total_pages += int(pagination.get("page_count", 0))
        if not (pagination.get("complete") is True
                and pagination.get("stable_total") is True
                and pagination.get("order_contiguous") is True
                and actual == expected == pagination.get("actual_count")
                and not pagination.get("duplicate_parameter_ids")
                and not pagination.get("empty_parameter_id_indices")):
            capture_issues.append({
                "case_id": case_id, "issue": "pagination", "pagination": pagination,
            })
        if manufacturer != "Waves":
            capture_issues.append({
                "case_id": case_id, "issue": "manufacturer", "actual": manufacturer,
            })
        if str(resolution.get("category", "")) != str(case.get("expected_category", "")):
            capture_issues.append({
                "case_id": case_id, "issue": "category",
                "actual": resolution.get("category"),
            })
        if (capture.get("capture_policy") or {}).get("plugin_parameter_writes") != 0:
            capture_issues.append({"case_id": case_id, "issue": "parameter writes"})
        if not (capture.get("metadata_merge") or {}).get("complete"):
            capture_issues.append({"case_id": case_id, "issue": "metadata incomplete"})
    audit.check("every captured surface is fully paged with stable total and unique IDs",
                not capture_issues,
                {"issues": capture_issues, "total_parameters": total_parameters,
                 "total_pages": total_pages})
    audit.check("all runtime-resolved plug-ins are Waves and no Plugin Alliance surface was read",
                manufacturer_counts == {"Waves": 50}
                and (summary.get("audit") or {}).get("plugin_alliance_parameter_read_count") == 0,
                {"manufacturer_counts": manufacturer_counts,
                 "plugin_alliance_parameter_read_count":
                     (summary.get("audit") or {}).get("plugin_alliance_parameter_read_count")})

    runtime_audit = summary.get("audit") or {}
    expected_tools = {"plugin.get_parameters", "plugin.load_to_rack", "track.add_audio"}
    actual_tools = set((runtime_audit.get("agent_tools") or {}).keys())
    zero_fields = [
        "plugin_parameter_write_count", "audio_probe_count", "learning_call_count",
        "profile_call_count", "spal_call_count", "b4_call_count",
        "plugin_alliance_parameter_read_count",
    ]
    audit.check("runtime used only the read-census allowlist and all prohibited counters are zero",
                actual_tools == expected_tools
                and set((runtime_audit.get("kernel_commands") or {}).keys()) == {"plugin_search"}
                and all(runtime_audit.get(field) == 0 for field in zero_fields),
                runtime_audit)

    signature_files = sorted((analysis / "signatures").glob("*.json"))
    signature_issues: list[dict[str, Any]] = []
    classifications: dict[str, int] = {}
    execution_states: dict[str, int] = {}
    for path in signature_files:
        item = read_json(path)
        canonical = item.get("canonical_topology")
        forbidden = forbidden_identity_keys(canonical)
        if forbidden:
            signature_issues.append({"case_id": item.get("case_id"),
                                     "issue": "identity keys", "paths": forbidden})
        if hash_json(canonical) != item.get("topology_signature_sha256"):
            signature_issues.append({"case_id": item.get("case_id"),
                                     "issue": "topology hash mismatch"})
        policy = item.get("inference_policy") or {}
        if policy.get("plugin_identity_used_as_feature") is not False \
                or policy.get("manufacturer_used_as_feature") is not False \
                or policy.get("production_classifier_used") is not False:
            signature_issues.append({"case_id": item.get("case_id"),
                                     "issue": "inference policy"})
        classification = item.get("public_classification") or "unresolved"
        classifications[classification] = classifications.get(classification, 0) + 1
        execution = item.get("execution") or {}
        status = str(execution.get("status", ""))
        execution_states[status] = execution_states.get(status, 0) + 1
        if classification not in {
                "fixed_slot_adjustable", "fixed_freq", "free_floating", "unresolved"}:
            signature_issues.append({"case_id": item.get("case_id"),
                                     "issue": "invalid public projection"})
        if status == "unsupported" and not execution.get("reasons"):
            signature_issues.append({"case_id": item.get("case_id"),
                                     "issue": "unsupported without reason"})
    audit.check("50 vendor-neutral topology signatures are reproducible and identity-free",
                len(signature_files) == 50 and not signature_issues,
                {"signature_count": len(signature_files), "issues": signature_issues,
                 "classifications": classifications,
                 "execution_states": execution_states})

    cluster_rows = [row for row in clusters.get("clusters", []) if isinstance(row, dict)]
    clustered_ids = [case_id for row in cluster_rows for case_id in row.get("member_case_ids", [])]
    expected_ids = sorted(str(case["id"]) for case in cases)
    audit.check("data-driven clusters cover every case exactly once without identity features",
                clusters.get("identity_features_used") is False
                and sorted(clustered_ids) == expected_ids
                and len(clustered_ids) == len(set(clustered_ids)) == 50,
                {"method": clusters.get("method"), "cluster_count": len(cluster_rows),
                 "member_count": len(clustered_ids),
                 "identity_features_used": clusters.get("identity_features_used")})

    with (analysis / "capability_matrix.csv").open(
            "r", encoding="utf-8-sig", newline="") as handle:
        matrix_rows = list(csv.DictReader(handle))
    audit.check("capability/rejection matrix has one complete row per Waves model",
                len(matrix_rows) == 50
                and {row["case_id"] for row in matrix_rows} == set(expected_ids)
                and all(row["execution_status"] != "unsupported" or row["reasons"]
                        for row in matrix_rows),
                {"row_count": len(matrix_rows),
                 "unsupported_count": sum(row["execution_status"] == "unsupported"
                                          for row in matrix_rows)})
    missing_report_names = [str(case["plugin_name"]) for case in cases
                            if str(case["plugin_name"]) not in report_text]
    audit.check("Markdown report contains all 50 model assignments and the stop boundary",
                not missing_report_names
                and "第一阶段到此结束" in report_text
                and "EQControlTopology" in report_text
                and "buildMarvelGEQSummary" in report_text,
                {"report": str(report_path), "missing_model_names": missing_report_names})

    audit.check("protected runtime and production EQ source hashes remained unchanged during capture",
                not manifest.get("protected_runtime_changed")
                and not manifest.get("production_eq_changed")
                and not manifest.get("run_error"),
                {"protected_runtime_changed": manifest.get("protected_runtime_changed"),
                 "production_eq_changed": manifest.get("production_eq_changed"),
                 "run_error": manifest.get("run_error")})
    production_diff = subprocess.run(
        ["git", "-C", str(repo), "diff", "--name-only", "--", "agent", "VitApp/Source"],
        check=True, capture_output=True, text=True).stdout.splitlines()
    audit.check("current worktree has no uncommitted production Agent/Kernel source edits",
                not production_diff, {"production_diff": production_diff})
    audit.check("analysis summary agrees with the raw census scope",
                analysis_summary.get("analyzed_case_count") == 50
                and analysis_summary.get("failed_case_count") == 0,
                analysis_summary)

    payload = {
        "schema_version": "waves.eq_topology_acceptance.v1",
        "status": "passed" if audit.passed else "failed",
        "requirement_count": len(audit.rows),
        "passed_count": sum(row["status"] == "passed" for row in audit.rows),
        "failed_count": sum(row["status"] == "failed" for row in audit.rows),
        "requirements": audit.rows,
    }
    output = run_dir / "acceptance.json"
    output.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"acceptance={payload['status']} passed={payload['passed_count']}/{payload['requirement_count']}")
    print(f"evidence={output}")
    return 0 if audit.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
