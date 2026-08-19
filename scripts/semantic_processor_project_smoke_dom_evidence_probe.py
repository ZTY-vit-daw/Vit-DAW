#!/usr/bin/env python3
"""Read-only DAD fine-evidence readiness probe for the product path.

This probe reads the canonical feature_snapshot evidence object and verifies
the four DOM source views plus the measurement conditions required for
cross-track comparability. It does not start an Agent, open sealed truth, or
mutate a DAW project.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


FINE_KEYS = ("noise_floor_evidence", "frequency_time_events", "transient_events", "band_dynamics")


def load_evidence(path: Path) -> dict[str, Any]:
    data = json.loads(path.read_text(encoding="utf-8"))
    content = data.get("content", data)
    if not isinstance(content, dict):
        raise ValueError("evidence content is not an object")
    snapshot = content.get("feature_snapshot", content)
    if not isinstance(snapshot, dict):
        raise ValueError("feature_snapshot is not an object")
    return snapshot


def evidence_rows(snapshot: dict[str, Any]) -> list[dict[str, Any]]:
    rows = snapshot.get("band_energy_summaries")
    if isinstance(rows, list) and rows:
        return [row for row in rows if isinstance(row, dict)]
    row = snapshot.get("band_energy_summary")
    if isinstance(row, dict):
        return [row]
    return []


def fine_status(row: dict[str, Any], key: str) -> str:
    value = row.get(key)
    if not isinstance(value, dict):
        return "missing"
    return str(value.get("status", "missing"))


def event_count(row: dict[str, Any], key: str) -> int:
    value = row.get(key)
    if not isinstance(value, dict):
        return 0
    events = value.get("events")
    return len(events) if isinstance(events, list) else 0


def measurement_ready(row: dict[str, Any]) -> tuple[bool, list[str]]:
    missing = []
    for key in ("source_revision", "clip_revision", "sample_rate", "channel_count"):
        value = row.get(key)
        if value is None or value == "" or value == 0:
            missing.append(key)
    if not row.get("coverage_seconds") and not row.get("analyzed_range"):
        missing.append("coverage_seconds_or_analyzed_range")
    return len(missing) == 0, missing


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--evidence", required=True, type=Path, help="feature_snapshot evidence object")
    parser.add_argument("--output", type=Path, help="optional JSON receipt path")
    args = parser.parse_args()
    if not args.evidence.exists():
        print(json.dumps({"status": "blocked", "reason": "evidence_missing"}, ensure_ascii=False))
        return 2
    snapshot = load_evidence(args.evidence)
    rows = evidence_rows(snapshot)
    tracks: list[dict[str, Any]] = []
    errors: list[str] = []
    for row in rows:
        track_id = str(row.get("track_id", "unknown"))
        statuses = {key: fine_status(row, key) for key in FINE_KEYS}
        counts = {key: event_count(row, key) for key in ("frequency_time_events", "transient_events")}
        ready, missing_conditions = measurement_ready(row)
        track_errors = [f"{track_id}:{key}={status}" for key, status in statuses.items() if status != "ready"]
        if counts["frequency_time_events"] <= 0:
            track_errors.append(f"{track_id}:frequency_time_events.empty")
        if counts["transient_events"] <= 0:
            track_errors.append(f"{track_id}:transient_events.empty")
        if not ready:
            track_errors.append(f"{track_id}:measurement_conditions.missing={','.join(missing_conditions)}")
        errors.extend(track_errors)
        tracks.append({
            "track_id": track_id,
            "fine_status": statuses,
            "event_counts": counts,
            "measurement_conditions_complete": ready,
            "measurement_condition_missing": missing_conditions,
            "ready": len(track_errors) == 0,
        })
    result = {
        "schema_version": "semantic_processor_project_dom_evidence_probe.v1",
        "status": "passed" if rows and not errors else "blocked",
        "track_count": len(rows),
        "tracks": tracks,
        "errors": errors,
    }
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["status"] == "passed" else 1


if __name__ == "__main__":
    sys.exit(main())