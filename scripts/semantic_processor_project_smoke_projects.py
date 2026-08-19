#!/usr/bin/env python3
"""Build the two isolated .vit projects from the public smoke manifest.

This module only consumes the public manifest.  It delegates the existing
product-path stems import and DAD wait, and never opens sealed truth.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

from semantic_mix_goal5_projects import build_case, wait_agent


SCHEMA_VERSION = "semantic_processor_agent_project_smoke_project_build.v1"
REQUIRED_PRODUCT_FINE_FIELDS = (
    "noise_floor_evidence",
    "frequency_time_events",
    "transient_events",
    "band_dynamics",
)


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def product_fine_evidence(project_path: Path, track_rows: list[dict[str, Any]]) -> dict[str, Any]:
    """Summarize product-path bounded DAD facts without opening sealed truth."""
    candidates = list(project_path.parent.glob(".vit_derived/*/l3_feature_log.jsonl"))
    log_files = sorted(
        candidates,
        key=lambda path: path.stat().st_mtime_ns if path.is_file() else 0,
        reverse=True,
    )[:1]
    events: list[dict[str, Any]] = []
    for log_path in log_files:
        try:
            for line in log_path.read_text(encoding="utf-8", errors="replace").splitlines():
                if not line.strip():
                    continue
                value = json.loads(line)
                if isinstance(value, dict) and str(value.get("feature_type", "")) == "band_energy_summary":
                    events.append(value)
        except (OSError, ValueError, json.JSONDecodeError):
            continue

    rows: list[dict[str, Any]] = []
    for track in track_rows:
        source_path = str(track.get("source_path", ""))
        candidates = [event for event in events if str(event.get("source_path") or event.get("file_path") or "") == source_path]
        fields = sorted({field for event in candidates for field in REQUIRED_PRODUCT_FINE_FIELDS if field in event})
        missing = [field for field in REQUIRED_PRODUCT_FINE_FIELDS if field not in fields]
        rows.append({
            "track_id": str(track.get("track_id", "")),
            "track_name": str(track.get("track_name", "")),
            "source_path": source_path,
            "event_count": len(candidates),
            "required_fields_present": fields,
            "missing_fields": missing,
            "status": "passed" if candidates and not missing else "blocked",
        })
    return {
        "status": "passed" if rows and all(row["status"] == "passed" for row in rows) else "blocked",
        "required_fields": list(REQUIRED_PRODUCT_FINE_FIELDS),
        "log_files": [str(path) for path in log_files],
        "tracks": rows,
    }


def wait_product_fine_evidence(project_path: Path, track_rows: list[dict[str, Any]], timeout_seconds: float) -> dict[str, Any]:
    """Wait for asynchronous product DAD publication to settle before reporting."""
    deadline = time.time() + max(0.0, timeout_seconds)
    latest = product_fine_evidence(project_path, track_rows)
    while latest.get("status") != "passed" and time.time() < deadline:
        time.sleep(0.75)
        latest = product_fine_evidence(project_path, track_rows)
    return latest


def build(args: argparse.Namespace) -> dict[str, Any]:
    manifest_path = Path(args.fixture_manifest).resolve()
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schema_version") != "semantic_processor_agent_project_smoke_public_manifest.v1":
        raise ValueError("fixture manifest schema mismatch")
    cases = [row for row in manifest.get("cases", []) if isinstance(row, dict)]
    selected = set(args.case or [])
    known = {str(row.get("public_case_id")) for row in cases}
    unknown = selected - known
    if unknown:
        raise ValueError(f"unknown public case IDs: {sorted(unknown)}")
    cases = [row for row in cases if not selected or str(row.get("public_case_id")) in selected]
    health = wait_agent(args.agent_http, min(args.timeout_sec, 30.0))
    reports: list[dict[str, Any]] = []
    for case in cases:
        # The reused product-path helper only needs public fields.  The adapter
        # intentionally renames public_case_id to its generic case_id input.
        public_case = {
            "case_id": str(case["public_case_id"]),
            "stems_directory": case["stems_directory"],
            "project_path": case["project_path"],
            "track_order": case["track_order"],
        }
        report = build_case(args.agent_http, public_case, args.timeout_sec, args.analysis_timeout_sec)
        report["analysis"]["product_fine_evidence"] = wait_product_fine_evidence(
            Path(report["project_path"]), report.get("tracks", []), args.analysis_timeout_sec
        )
        reports.append(report)
    output = manifest_path.parent / "project_build_report.json"
    report = {
        "schema_version": SCHEMA_VERSION,
        "created_at": now_iso(),
        "status": "passed",
        "contract_id": manifest.get("contract_id"),
        "fixture_set_id": manifest.get("fixture_set_id"),
        "fixture_manifest": str(manifest_path),
        "agent_health": health,
        "projects": reports,
        "project_count": len(reports),
    }
    write_json_atomic(output, report)
    return {**report, "output": str(output)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fixture-manifest", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--case", action="append", default=[])
    parser.add_argument("--timeout-sec", type=float, default=600.0)
    parser.add_argument("--analysis-timeout-sec", type=float, default=600.0)
    args = parser.parse_args()
    try:
        print(json.dumps(build(args), ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
