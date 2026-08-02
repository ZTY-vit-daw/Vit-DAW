"""Godot product-path smoke for direct C1 frequency cleanup on a temporary stems project.

The caller owns the Godot/Hub/Kernel/Agent lifecycle. This probe uses only the
public Agent HTTP surface, never confirms the returned proposal, and fails if
C1 readiness performs observation/render repair or mutates the project.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import time
import urllib.request
from pathlib import Path
from typing import Any


C1_CAPABILITY_ID = "fine_mix.frequency_cleanup.v1"
C1_MESSAGE = "进行C1处理"
SUCCESS_STAGES = {
    "analysis",
    "no_static_eq_required",
    "plugin_selection_required",
    "proposal",
    "plugin_load_proposal",
    "eq_proposal",
    "target_preflight_blocked",
}


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        parsed = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result")
    return result if isinstance(result, dict) else {}


def invoke_tool(
    base_url: str,
    tool: str,
    args: dict[str, Any],
    timeout: float,
    *,
    confirmed: bool = False,
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "c1_frequency_cleanup_agent_smoke",
        "confirmed": confirmed,
    }
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    require(
        str(response.get("status", "")).lower() in {"ok", "success", "completed"},
        f"{tool} failed: {json.dumps(response, ensure_ascii=False)[:2400]}",
    )
    return response


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retained in smoke diagnostics.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready at {base_url}: {last_error}")


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def sha256_file(path: Path) -> str:
    if not path.is_file():
        return ""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


def normalized_path(value: Any) -> str:
    text = str(value or "").strip()
    if not text:
        return ""
    try:
        return str(Path(text).resolve()).casefold()
    except OSError:
        return text.replace("/", "\\").casefold()


def usable_l3_band_row(row: dict[str, Any]) -> bool:
    status = str(row.get("status") or row.get("quality_status") or "").lower()
    bands = row.get("bands") if isinstance(row.get("bands"), dict) else {}
    if status in {"ready", "partial"}:
        return bool(bands)
    reason = str(row.get("quality_reason") or row.get("reason") or "").lower()
    try:
        coverage = float(row.get("coverage_ratio") or 0.0)
    except (TypeError, ValueError):
        coverage = 0.0
    return status == "suspect" and reason in {"all_zero_audio", "all_zero_source", "input_all_zero"} and coverage >= 0.999 and bool(bands)


def ready_l3_band_materials(feature_path: Path, acoustic_path: Path) -> set[tuple[str, str]]:
    ready: set[tuple[str, str]] = set()
    if feature_path.is_file():
        try:
            payload = json.loads(feature_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            payload = {}
        if isinstance(payload, dict):
            rows = payload.get("band_energy_summaries", [])
            if not isinstance(rows, list):
                rows = []
            latest = payload.get("band_energy_summary")
            if isinstance(latest, dict):
                rows = [*rows, latest]
            for row in rows:
                if not isinstance(row, dict) or not usable_l3_band_row(row):
                    continue
                source_path = row.get("source_path") or row.get("file_path")
                source_revision = str(row.get("source_revision") or row.get("source_fingerprint") or "").strip()
                if normalized_path(source_path) and source_revision:
                    ready.add((normalized_path(source_path), source_revision.casefold()))

    if not acoustic_path.is_file():
        return ready
    try:
        payload = json.loads(acoustic_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return ready
    rows = payload.get("packages", []) if isinstance(payload, dict) else []
    for row in rows:
        if not isinstance(row, dict):
            continue
        layers = row.get("package_layers") if isinstance(row.get("package_layers"), dict) else {}
        l3 = layers.get("l3_deep") if isinstance(layers.get("l3_deep"), dict) else {}
        features = l3.get("features") if isinstance(l3.get("features"), dict) else {}
        band = features.get("band_energy_summary") if isinstance(features.get("band_energy_summary"), dict) else {}
        ref = band.get("ref") if isinstance(band.get("ref"), dict) else {}
        candidate = dict(ref)
        candidate["status"] = band.get("status")
        candidate["reason"] = band.get("reason") or ref.get("reason")
        if not usable_l3_band_row(candidate):
            continue
        source_path = row.get("source_path") or ref.get("source_path") or ref.get("file_path")
        source_revision = str(row.get("source_revision") or row.get("source_fingerprint") or ref.get("source_revision") or "").strip()
        if normalized_path(source_path) and source_revision:
            ready.add((normalized_path(source_path), source_revision.casefold()))
    return ready


def wait_for_portable_l3(feature_path: Path, acoustic_path: Path, imported_refs: list[dict[str, Any]], timeout: float) -> dict[str, Any]:
    wanted: dict[tuple[str, str], str] = {}
    for row in imported_refs:
        source = Path(str(row.get("source_file_path") or row.get("imported_file_path") or "")).resolve()
        duration = float(row.get("duration_seconds") or 0.0)
        stat = source.stat()
        source_revision = f"{source}|size={stat.st_size}|mtime={stat.st_mtime_ns // 1_000_000}|length={duration:.4f}"
        wanted[(normalized_path(source), source_revision.casefold())] = source.name
    deadline = time.monotonic() + timeout
    latest: set[tuple[str, str]] = set()
    while time.monotonic() < deadline:
        latest = ready_l3_band_materials(feature_path, acoustic_path)
        matched = wanted.keys() & latest
        if len(matched) == len(wanted):
            return {
                "status": "ready",
                "ready_count": len(matched),
                "total_count": len(wanted),
                "missing_files": [],
            }
        time.sleep(5.0)
    missing = [name for key, name in wanted.items() if key not in latest]
    return {
        "status": "timeout",
        "ready_count": len(wanted) - len(missing),
        "total_count": len(wanted),
        "missing_files": sorted(missing),
    }


def track_signature(state: dict[str, Any]) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for track in state.get("tracks", []):
        if not isinstance(track, dict):
            continue
        track_id = str(track.get("track_id") or track.get("id") or "").strip()
        if not track_id:
            continue
        plugins: list[dict[str, Any]] = []
        for plugin in track.get("plugins") or []:
            if not isinstance(plugin, dict):
                continue
            plugins.append(
                {
                    "id": str(plugin.get("plugin_id") or plugin.get("plugin_item_id") or plugin.get("item_id") or plugin.get("id") or ""),
                    "name": str(plugin.get("name") or plugin.get("plugin_name") or ""),
                    "identifier": str(plugin.get("identifier") or plugin.get("plugin_identifier") or ""),
                    "enabled": plugin.get("enabled"),
                    "bypassed": plugin.get("bypassed", plugin.get("is_bypassed")),
                }
            )
        clips: list[dict[str, Any]] = []
        for clip in track.get("clips") or []:
            if isinstance(clip, dict):
                clips.append(
                    {
                        "id": str(clip.get("clip_id") or clip.get("item_id") or clip.get("id") or ""),
                        "source_path": str(
                            clip.get("current_source_path")
                            or clip.get("source_path")
                            or clip.get("file_path")
                            or clip.get("media_path")
                            or ""
                        ),
                    }
                )
        rows.append(
            {
                "track_id": track_id,
                "track_name": str(track.get("track_name") or track.get("name") or ""),
                "volume_db": track.get("volume_db", track.get("fader_db", track.get("gain_db"))),
                "pan": track.get("pan", track.get("pan_value")),
                "mute": track.get("mute", track.get("is_muted")),
                "solo": track.get("solo", track.get("is_solo")),
                "plugins": plugins,
                "clips": clips,
            }
        )
    return sorted(rows, key=lambda row: row["track_id"])


def execution_tools(response: dict[str, Any]) -> list[str]:
    tools: list[str] = []
    for row in response.get("executed_kernel_reply", []):
        if isinstance(row, dict):
            name = str(row.get("tool") or row.get("command_name") or row.get("cmd") or "").strip()
            if name:
                tools.append(name)
    return tools


def int_value(value: Any) -> int:
    try:
        return int(value or 0)
    except (TypeError, ValueError):
        return 0


def run(args: argparse.Namespace) -> dict[str, Any]:
    started = time.monotonic()
    repo_root = Path(args.repo_root).resolve()
    stems_folder = Path(args.stems_folder).resolve()
    project_path = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    project_path.parent.mkdir(parents=True, exist_ok=True)
    require(stems_folder.is_dir(), f"stems folder not found: {stems_folder}")
    wav_files = sorted(stems_folder.glob("*.wav"))
    require(len(wav_files) >= 2, f"expected a multitrack WAV folder, got {len(wav_files)} files")

    acoustic_path = repo_root / "VitApp" / "Workspace" / "Artifacts" / "acoustic_package_status.json"
    feature_path = repo_root / "VitApp" / "Workspace" / "Artifacts" / "mixboard_feature_snapshot.json"
    summary: dict[str, Any] = {
        "schema_version": "c1_frequency_cleanup_product_path_smoke.v1",
        "status": "running",
        "repo_root": str(repo_root),
        "stems_folder": str(stems_folder),
        "wav_count": len(wav_files),
        "wav_total_bytes": sum(path.stat().st_size for path in wav_files),
        "project_path": str(project_path),
        "acoustic_package_path": str(acoustic_path),
        "feature_snapshot_path": str(feature_path),
        "acoustic_package_sha256_before": sha256_file(acoustic_path),
    }

    try:
        summary["health"] = wait_agent(args.agent_http, min(args.timeout_sec, 60.0))
        invoke_tool(args.agent_http, "project.new", {}, args.timeout_sec, confirmed=True)
        invoke_tool(
            args.agent_http,
            "project.save_as",
            {"file_path": str(project_path)},
            args.timeout_sec,
            confirmed=True,
        )
        imported = invoke_tool(
            args.agent_http,
            "project.import_folder_as_stems",
            {
                "folder_path": str(stems_folder),
                "recursive": False,
                "target_policy": "create_tracks",
                "start_time_seconds": 0.0,
                "skip_unreadable": False,
                "command_timeout_ms": int(args.timeout_sec * 1000),
            },
            args.timeout_sec,
            confirmed=True,
        )
        write_json(artifact_dir / "import_response.json", imported)
        imported_result = result_map(imported)
        import_summary = imported_result.get("summary") if isinstance(imported_result.get("summary"), dict) else imported_result
        imported_refs = imported_result.get("imported_track_refs", [])
        if not isinstance(imported_refs, list):
            imported_refs = []
        imported_refs = [row for row in imported_refs if isinstance(row, dict)]
        created_track_ids = imported_result.get("created_track_ids", [])
        created_clip_ids = imported_result.get("created_clip_ids", [])
        if not isinstance(created_track_ids, list) or not created_track_ids:
            created_track_ids = [str(row.get("track_id") or "") for row in imported_refs if isinstance(row, dict)]
        if not isinstance(created_clip_ids, list) or not created_clip_ids:
            created_clip_ids = [str(row.get("clip_id") or "") for row in imported_refs if isinstance(row, dict)]
        created_track_ids = [value for value in created_track_ids if str(value).strip()]
        created_clip_ids = [value for value in created_clip_ids if str(value).strip()]
        require(len(created_track_ids) == len(wav_files), f"imported track count {len(created_track_ids)} != {len(wav_files)}")
        require(len(created_clip_ids) == len(wav_files), f"imported clip count {len(created_clip_ids)} != {len(wav_files)}")
        summary["import"] = {
            "created_track_count": len(created_track_ids),
            "created_clip_count": len(created_clip_ids),
            "analysis_deferred": imported_result.get("analysis_deferred", import_summary.get("analysis_deferred")),
            "analysis_queue_status": imported_result.get("analysis_queue_status", import_summary.get("analysis_queue_status")),
        }

        l3_wait_started = time.monotonic()
        l3_status = wait_for_portable_l3(feature_path, acoustic_path, imported_refs, args.timeout_sec)
        l3_status["elapsed_ms"] = int((time.monotonic() - l3_wait_started) * 1000)
        summary["portable_l3_preparation"] = l3_status
        write_json(artifact_dir / "portable_l3_preparation.json", l3_status)
        require(
            l3_status["status"] == "ready",
            f"background observation did not prepare portable L3 for the full project: {l3_status['ready_count']}/{l3_status['total_count']}",
        )

        before_response = invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec)
        before_state = result_map(before_response)
        before_signature = track_signature(before_state)
        write_json(artifact_dir / "project_state_before_c1.json", before_response)
        write_json(artifact_dir / "project_signature_before_c1.json", before_signature)

        conversation_id = "c1_product_path_smoke_" + time.strftime("%Y%m%d_%H%M%S")
        summary["conversation_id"] = conversation_id
        chat_started = time.monotonic()
        chat = request_json(
            "POST",
            args.agent_http.rstrip("/") + "/agent/chat",
            {
                "conversation_id": conversation_id,
                "message": C1_MESSAGE,
                "context": {
                    "agent_mode": "chat",
                    "capability_id": C1_CAPABILITY_ID,
                    "interaction_mode": "propose",
                    "c1_execute": True,
                    "smoke_contract": "c1_existing_evidence_zero_observation.v1",
                },
            },
            args.timeout_sec,
        )
        summary["chat_elapsed_ms"] = int((time.monotonic() - chat_started) * 1000)
        write_json(artifact_dir / "chat_c1.json", chat)

        after_response = invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec)
        after_state = result_map(after_response)
        after_signature = track_signature(after_state)
        write_json(artifact_dir / "project_state_after_c1.json", after_response)
        write_json(artifact_dir / "project_signature_after_c1.json", after_signature)

        workflow_data = chat.get("workflow_data") if isinstance(chat.get("workflow_data"), dict) else {}
        timing = workflow_data.get("timing") if isinstance(workflow_data.get("timing"), dict) else {}
        coverage = workflow_data.get("coverage") if isinstance(workflow_data.get("coverage"), dict) else {}
        stage = str(workflow_data.get("canary_stage") or "").strip()
        capability_id = str(workflow_data.get("capability_id") or "").strip()
        tools = execution_tools(chat)
        forbidden_tools = [name for name in tools if name in {"mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation"}]
        diagnosis_fill_count = int_value(timing.get("diagnosis_fill_requested_track_count"))
        diagnosis_render_count = int_value(timing.get("diagnosis_fill_render_count"))
        assembly_ms = int_value(timing.get("existing_evidence_assembly_ms"))
        ccb_ms = int_value(timing.get("ccb_ms"))
        projected_count = int_value(coverage.get("eligible_track_count", coverage.get("analyzed_track_count")))
        project_count = int_value(coverage.get("project_track_count"))

        summary.update(
            {
                "reply": str(chat.get("reply") or ""),
                "stop_reason": str(chat.get("stop_reason") or ""),
                "goal_status": str(chat.get("goal_status") or ""),
                "capability_id": capability_id,
                "canary_stage": stage,
                "coverage": coverage,
                "timing": timing,
                "tool_route": tools,
                "forbidden_observation_tools": forbidden_tools,
                "diagnosis_fill_requested_track_count": diagnosis_fill_count,
                "diagnosis_fill_render_count": diagnosis_render_count,
                "project_mutation_detected": before_signature != after_signature,
                "acoustic_package_sha256_after": sha256_file(acoustic_path),
            }
        )

        require(capability_id == C1_CAPABILITY_ID, f"chat did not route to C1: capability_id={capability_id!r}")
        require(stage != "readiness_blocked", f"C1 readiness blocked: {summary['reply']}")
        require(stage in SUCCESS_STAGES or bool(chat.get("proposal")), f"unexpected C1 stage: {stage!r}")
        require(not forbidden_tools, f"C1 readiness invoked observation tools: {forbidden_tools}")
        require(diagnosis_fill_count == 0, f"C1 readiness requested diagnosis fill for {diagnosis_fill_count} tracks")
        require(diagnosis_render_count == 0, f"C1 readiness rendered {diagnosis_render_count} tracks")
        require(assembly_ms + ccb_ms <= 30_000, f"C1 existing-evidence assembly/CCB took {assembly_ms + ccb_ms} ms")
        require(before_signature == after_signature, "C1 changed track/plugin state before confirmation")
        require(project_count == len(wav_files), f"C1 coverage project_track_count={project_count}, expected {len(wav_files)}")
        require(projected_count == len(wav_files), f"C1 eligible/analyzed coverage={projected_count}, expected {len(wav_files)}")

        summary["status"] = "passed"
        summary["total_elapsed_ms"] = int((time.monotonic() - started) * 1000)
        write_json(artifact_dir / "summary.json", summary)
        return summary
    except Exception as exc:  # noqa: BLE001 - complete artifact is required on smoke failure.
        summary["status"] = "failed"
        summary["error"] = str(exc)
        summary["total_elapsed_ms"] = int((time.monotonic() - started) * 1000)
        summary["acoustic_package_sha256_after"] = sha256_file(acoustic_path)
        write_json(artifact_dir / "summary.json", summary)
        raise


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=r"D:\Vit_DAW")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--stems-folder", required=True)
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--timeout-sec", type=float, default=360.0)
    args = parser.parse_args()
    result = run(args)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
