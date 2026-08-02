#!/usr/bin/env python3
"""Create six small Vit projects from the opaque goal-5 fixture cases.

This script requires a running VitAgent and VitApp kernel.  It imports only the
case ``stems`` directory exposed by ``fixture_manifest.json``; sealed acoustic
truth is deliberately never opened or passed through the product path.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any


SCHEMA_VERSION = "semantic_mix_goal5_projects.v1"


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else {}


def invoke(
    base_url: str,
    tool: str,
    args: dict[str, Any],
    timeout: float,
    *,
    confirmed: bool = False,
) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {
            "tool": tool,
            "args": args,
            "confirmed": confirmed,
            "source": "semantic_mix_goal5_projects",
        },
        timeout,
    )
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(response, ensure_ascii=False)[:2600]}")
    return response


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retained for timeout diagnostics.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready: {last_error}")


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def strings(value: Any) -> list[str]:
    if not isinstance(value, list):
        return []
    return [str(item) for item in value if str(item).strip()]


def wait_analysis(
    base_url: str,
    job_id: str,
    request_timeout: float,
    analysis_timeout: float,
) -> dict[str, Any]:
    invoke(
        base_url,
        "project.audio_analysis_start",
        {"analysis_job_id": job_id, "interval_ms": 10},
        request_timeout,
    )
    deadline = time.time() + analysis_timeout
    latest: dict[str, Any] = {}
    while time.time() < deadline:
        status = invoke(
            base_url,
            "project.audio_analysis_status",
            {"analysis_job_id": job_id, "latest": True},
            request_timeout,
        )
        latest = result_map(status)
        job = latest.get("analysis_job") if isinstance(latest.get("analysis_job"), dict) else latest
        ready = int(job.get("dad_fact_ready_count") or 0)
        total = int(job.get("dad_fact_total_count") or 0)
        state = str(job.get("dad_fact_status", "")).lower()
        waveform_rows = job.get("track_waveform_envelopes")
        waveforms = len(waveform_rows) if isinstance(waveform_rows, list) else 0
        if state == "ready" and total > 0 and ready >= total and waveforms >= total:
            return {
                "analysis_job_id": job_id,
                "dad_fact_status": state,
                "dad_fact_ready_count": ready,
                "dad_fact_total_count": total,
                "track_waveform_envelope_count": waveforms,
            }
        time.sleep(0.75)
    raise RuntimeError(f"analysis did not become ready for {job_id}: {json.dumps(latest, ensure_ascii=False)[:3200]}")


def project_tracks(state_response: dict[str, Any]) -> list[dict[str, Any]]:
    result = result_map(state_response)
    value = result.get("tracks")
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def build_case(
    base_url: str,
    case: dict[str, Any],
    request_timeout: float,
    analysis_timeout: float,
) -> dict[str, Any]:
    case_id = str(case["case_id"])
    stems_directory = Path(case["stems_directory"]).resolve()
    project_path = Path(case["project_path"]).resolve()
    if not stems_directory.is_dir():
        raise FileNotFoundError(f"stems directory missing for {case_id}: {stems_directory}")
    expected_tracks = [str(item) for item in case["track_order"]]

    invoke(base_url, "project.new", {}, request_timeout, confirmed=True)
    # New-project implementations have differed over time in whether they keep
    # one empty UI track. Clearing here guarantees the fixture is exactly six
    # imported stems without deleting any user project (this script runs only
    # in the dedicated generated fixture session).
    invoke(base_url, "project.clear", {}, request_timeout, confirmed=True)
    invoke(
        base_url,
        "project.save_as",
        {"file_path": str(project_path)},
        request_timeout,
        confirmed=True,
    )
    imported = invoke(
        base_url,
        "project.import_folder_as_stems",
        {
            "folder_path": str(stems_directory),
            "recursive": False,
            "target_policy": "create_tracks",
            "start_time_seconds": 0.0,
            "skip_unreadable": False,
            "command_timeout_ms": int(request_timeout * 1000),
        },
        request_timeout,
        confirmed=True,
    )
    imported_result = result_map(imported)
    created_track_ids = strings(
        imported_result.get("created_track_ids") or imported_result.get("created_track_ids_preview")
    )
    created_clip_ids = strings(
        imported_result.get("created_clip_ids") or imported_result.get("created_clip_ids_preview")
    )
    if len(created_track_ids) != len(expected_tracks) or len(created_clip_ids) != len(expected_tracks):
        raise RuntimeError(
            f"{case_id} import returned {len(created_track_ids)} tracks/{len(created_clip_ids)} clips, "
            f"expected {len(expected_tracks)}: {json.dumps(imported_result, ensure_ascii=False)[:2600]}"
        )
    job_value = imported_result.get("analysis_job")
    job = job_value if isinstance(job_value, dict) else {}
    job_id = str(imported_result.get("analysis_job_id") or job.get("analysis_job_id") or job.get("job_id") or "")
    if not job_id:
        raise RuntimeError(f"{case_id} import omitted analysis_job_id")
    analysis = wait_analysis(base_url, job_id, request_timeout, analysis_timeout)
    invoke(base_url, "project.save", {}, request_timeout, confirmed=True)

    state_response = invoke(base_url, "project.state", {}, request_timeout)
    tracks = project_tracks(state_response)
    by_id = {
        str(row.get("track_id") or row.get("id") or ""): row
        for row in tracks
        if str(row.get("track_id") or row.get("id") or "")
    }
    missing = [track_id for track_id in created_track_ids if track_id not in by_id]
    if missing:
        raise RuntimeError(f"{case_id} created tracks missing after save: {missing}")
    unexpected = [track_id for track_id in by_id if track_id not in set(created_track_ids)]
    if unexpected:
        raise RuntimeError(f"{case_id} contains unexpected tracks after clear/import: {unexpected}")

    track_rows: list[dict[str, Any]] = []
    for track_id in created_track_ids:
        row = by_id[track_id]
        clips = row.get("clips") if isinstance(row.get("clips"), list) else []
        if len(clips) != 1:
            raise RuntimeError(f"{case_id} track {track_id} has {len(clips)} clips, expected one")
        track_rows.append(
            {
                "track_id": track_id,
                "track_name": str(row.get("name") or row.get("track_name") or ""),
                "clip_id": str(clips[0].get("clip_id") or clips[0].get("id") or ""),
                "source_path": str(
                    clips[0].get("file_path")
                    or clips[0].get("source_path")
                    or clips[0].get("current_source_path")
                    or ""
                ),
            }
        )
    if sorted(row["track_name"].lower() for row in track_rows) != sorted(expected_tracks):
        raise RuntimeError(f"{case_id} track names do not match fixture roles: {track_rows}")
    if not project_path.is_file():
        raise RuntimeError(f"{case_id} project was not saved: {project_path}")
    return {
        "case_id": case_id,
        "project_path": str(project_path),
        "project_size_bytes": project_path.stat().st_size,
        "tracks": track_rows,
        "analysis": analysis,
    }


def default_manifest() -> str:
    local = os.environ.get("LOCALAPPDATA")
    if not local:
        raise RuntimeError("LOCALAPPDATA is unavailable; pass --fixture-manifest")
    pointer = Path(local) / "Vit" / "Goal5Fixtures" / "current.json"
    if not pointer.is_file():
        raise FileNotFoundError(f"fixture pointer missing: {pointer}")
    current = json.loads(pointer.read_text(encoding="utf-8"))
    return str(current["fixture_manifest"])


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--fixture-manifest", default="")
    parser.add_argument("--case", action="append", dest="cases", default=[])
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--analysis-timeout-sec", type=float, default=240.0)
    args = parser.parse_args()
    try:
        health = wait_agent(args.agent_http, min(args.timeout_sec, 30.0))
        manifest_path = Path(args.fixture_manifest or default_manifest()).resolve()
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        all_cases = [item for item in manifest.get("cases", []) if isinstance(item, dict)]
        selected = set(args.cases)
        cases = [item for item in all_cases if not selected or str(item.get("case_id")) in selected]
        if selected - {str(item.get("case_id")) for item in cases}:
            raise ValueError(f"unknown case IDs: {sorted(selected - {str(item.get('case_id')) for item in cases})}")
        reports: list[dict[str, Any]] = []
        for case in cases:
            reports.append(build_case(args.agent_http, case, args.timeout_sec, args.analysis_timeout_sec))
            print(json.dumps({"status": "case_ready", "case_id": case["case_id"]}, ensure_ascii=False), flush=True)
        report = {
            "schema_version": SCHEMA_VERSION,
            "created_at": now_iso(),
            "status": "passed",
            "fixture_set_id": manifest.get("set_id"),
            "fixture_manifest": str(manifest_path),
            "agent_health": health,
            "case_count": len(reports),
            "projects": reports,
        }
        report_path = manifest_path.parent / "project_build_report.json"
        write_json_atomic(report_path, report)
        print(json.dumps({**report, "projects": reports}, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
