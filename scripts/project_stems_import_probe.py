import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any, Dict, List, Set


EXPECTED_IMPORT_FEATURES_PER_CLIP = 1


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def send_command(req_socket: Any, payload: Dict[str, Any]) -> Dict[str, Any]:
    req_socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = req_socket.recv_string()
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError:
        return {"status": "error", "raw_reply": raw}
    if isinstance(reply, dict):
        return reply
    return {"status": "error", "raw_reply": reply}


def require_ok(label: str, reply: Dict[str, Any]) -> None:
    if str(reply.get("status", "")).lower() != "ok":
        raise RuntimeError(f"{label} did not return ok: {json.dumps(reply, ensure_ascii=False)[:2000]}")


def rows(value: Any) -> List[Dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def strings(value: Any) -> List[str]:
    if not isinstance(value, list):
        return []
    return [str(item) for item in value if str(item)]


def compact_reply(reply: Dict[str, Any]) -> Dict[str, Any]:
    out: Dict[str, Any] = {"status": reply.get("status"), "command": reply.get("command")}
    for key in (
        "message",
        "action",
        "baking_status",
        "analysis_deferred",
        "analysis_jobs_created",
        "analysis_jobs_queued",
        "analysis_total_clips",
        "analysis_total_feature_jobs",
        "analysis_job_id",
        "analysis_queue_status",
        "background_analysis_status",
        "last_created_track_id",
        "last_created_clip_id",
    ):
        if key in reply:
            out[key] = reply[key]
    for key in ("summary", "audio_settings_snapshot"):
        if isinstance(reply.get(key), dict):
            out[key] = reply[key]
    if isinstance(reply.get("analysis_job"), dict):
        out["analysis_job"] = {
            key: reply["analysis_job"].get(key)
            for key in (
                "analysis_job_id",
                "status",
                "analysis_queue_status",
                "total_clips",
                "submitted_clips",
                "pending_clips",
                "total_feature_jobs",
                "submitted_feature_jobs",
                "pending_feature_jobs",
                "progress_percent",
                "interval_ms",
                "cancel_scope",
                "completion_scope",
            )
            if key in reply["analysis_job"]
        }
    for key, preview_key in (
        ("import_plan", "import_plan"),
        ("imported_tracks", "imported_tracks_preview"),
        ("files", "file_preview"),
        ("sample_rate_mismatches", "sample_rate_mismatch_examples"),
        ("bit_depth_or_format_mismatches", "bit_depth_or_format_mismatch_examples"),
        ("unreadable_files", "unreadable_file_examples"),
    ):
        value = reply.get(key)
        if isinstance(value, list):
            out[preview_key] = value[:8]
        elif isinstance(value, dict):
            compact = dict(value)
            if isinstance(compact.get("track_plan"), list):
                compact["track_plan_preview"] = compact.pop("track_plan")[:8]
            out[preview_key] = compact
    for key in ("created_track_ids", "created_clip_ids"):
        value = strings(reply.get(key))
        if value:
            out[key + "_preview"] = value[:16]
            out[key + "_count"] = len(value)
    return out


def write_json_file(path: Path, value: Dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def track_clip_ids(project_state: Dict[str, Any]) -> Dict[str, Set[str]]:
    out: Dict[str, Set[str]] = {}
    for track in rows(project_state.get("tracks")):
        track_id = str(track.get("track_id") or track.get("id") or "")
        if not track_id:
            continue
        clip_ids: Set[str] = set()
        for clip in rows(track.get("clips")):
            clip_id = str(clip.get("clip_id") or clip.get("id") or "")
            if clip_id:
                clip_ids.add(clip_id)
        out[track_id] = clip_ids
    return out


def assert_import_visible(label: str, project_state: Dict[str, Any], track_ids: List[str], clip_ids: List[str]) -> None:
    by_track = track_clip_ids(project_state)
    missing_tracks = [track_id for track_id in track_ids if track_id not in by_track]
    if missing_tracks:
        raise RuntimeError(f"{label}: imported tracks missing from project_state: {missing_tracks[:8]}")
    visible_clips = set()
    for track_id in track_ids:
        visible_clips.update(by_track.get(track_id, set()))
    missing_clips = [clip_id for clip_id in clip_ids if clip_id not in visible_clips]
    if missing_clips:
        raise RuntimeError(f"{label}: imported clips missing from project_state: {missing_clips[:8]}")


def analysis_job(reply: Dict[str, Any]) -> Dict[str, Any]:
    value = reply.get("analysis_job")
    return value if isinstance(value, dict) else {}


def require_analysis_job(label: str, reply: Dict[str, Any]) -> Dict[str, Any]:
    job = analysis_job(reply)
    if not job:
        raise RuntimeError(f"{label} did not return analysis_job")
    return job


def run_probe(args: argparse.Namespace) -> int:
    import zmq

    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    project_path = Path(args.project_path).resolve()
    project_path.parent.mkdir(parents=True, exist_ok=True)
    training_folder = Path(args.training_folder).resolve()
    if not training_folder.is_dir():
        raise RuntimeError(f"training folder does not exist: {training_folder}")
    sealed_folder = Path(args.sealed_folder).resolve() if args.sealed_folder else None

    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.SNDTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(args.req_url)

    commands: List[Dict[str, Any]] = []
    replies: Dict[str, Dict[str, Any]] = {}
    command_timeout_ms = min(max(args.req_timeout_ms, 5000), 300000)

    def call(label: str, payload: Dict[str, Any], ok: bool = True) -> Dict[str, Any]:
        commands.append(payload)
        reply = send_command(req, payload)
        replies[label] = reply
        if ok:
            require_ok(label, reply)
        return reply

    try:
        call("ping", {"cmd": "ping"})
        call("new_project", {"cmd": "new_project"})
        call("save_as_project_before_import", {"cmd": "save_as_project", "file_path": str(project_path)})

        preflight = call("training_stems_import_preflight", {
            "cmd": "project.import_preflight",
            "folder_path": str(training_folder),
            "recursive": False,
            "media_kinds": ["audio"],
            "intended_mode": "stems_folder",
            "target_policy": "create_tracks",
            "start_time_seconds": 0,
            "command_timeout_ms": command_timeout_ms,
        })
        preflight_summary = preflight.get("summary")
        if not isinstance(preflight_summary, dict):
            raise RuntimeError("project.import_preflight did not return summary")
        readable_count = int(preflight_summary.get("readable_file_count") or 0)
        tracks_to_create = int(preflight_summary.get("tracks_to_create") or 0)
        if readable_count <= 0:
            raise RuntimeError("training preflight found no readable audio files")
        if tracks_to_create != readable_count:
            raise RuntimeError("training preflight tracks_to_create did not match readable_file_count")

        imported = call("training_stems_import_folder_as_stems", {
            "cmd": "project.import_folder_as_stems",
            "folder_path": str(training_folder),
            "recursive": False,
            "target_policy": "create_tracks",
            "start_time_seconds": 0,
            "confirmed": True,
            "skip_unreadable": False,
            "command_timeout_ms": command_timeout_ms,
        })
        import_summary = imported.get("summary")
        if not isinstance(import_summary, dict):
            raise RuntimeError("project.import_folder_as_stems did not return summary")
        tracks_created = int(import_summary.get("tracks_created") or 0)
        clips_created = int(import_summary.get("clips_created") or 0)
        if tracks_created != readable_count or clips_created != readable_count:
            raise RuntimeError(f"import created tracks/clips {tracks_created}/{clips_created}, expected {readable_count}")
        created_track_ids = strings(imported.get("created_track_ids"))
        created_clip_ids = strings(imported.get("created_clip_ids"))
        if len(created_track_ids) != readable_count or len(created_clip_ids) != readable_count:
            raise RuntimeError("created ID counts do not match readable file count")
        if imported.get("analysis_deferred") is not True:
            raise RuntimeError("default stems import should defer audio analysis")
        if str(imported.get("baking_status", "")).lower() != "deferred":
            raise RuntimeError("default stems import should report baking_status=deferred")
        if int(imported.get("analysis_jobs_created") or 0) != 0:
            raise RuntimeError("default stems import should not create analysis jobs")
        if import_summary.get("analysis_deferred") is not True:
            raise RuntimeError("default stems import summary should report analysis_deferred=true")
        if int(import_summary.get("analysis_jobs_created") or 0) != 0:
            raise RuntimeError("default stems import summary should report zero analysis jobs")
        analysis_job_id = str(imported.get("analysis_job_id") or import_summary.get("analysis_job_id") or "")
        if not analysis_job_id:
            raise RuntimeError("default stems import should return analysis_job_id for deferred background analysis")
        if str(imported.get("analysis_queue_status", "")).lower() != "queued":
            raise RuntimeError("default stems import should queue deferred audio analysis")
        if int(imported.get("analysis_jobs_queued") or 0) != readable_count * EXPECTED_IMPORT_FEATURES_PER_CLIP:
            raise RuntimeError(
                "default stems import should queue "
                f"{EXPECTED_IMPORT_FEATURES_PER_CLIP} lightweight analysis job per clip"
            )
        initial_analysis_job = require_analysis_job("training_stems_import_folder_as_stems", imported)
        if str(initial_analysis_job.get("analysis_queue_status", "")).lower() != "queued":
            raise RuntimeError("initial analysis job should be queued")
        if int(initial_analysis_job.get("submitted_feature_jobs") or 0) != 0:
            raise RuntimeError("initial deferred analysis job should not have submitted feature jobs")

        queued_status = call("analysis_status_queued", {
            "cmd": "project.audio_analysis_status",
            "analysis_job_id": analysis_job_id,
        })
        queued_job = require_analysis_job("analysis_status_queued", queued_status)
        if int(queued_job.get("submitted_clips") or 0) != 0:
            raise RuntimeError("queued analysis status should report zero submitted clips")
        if int(queued_job.get("total_clips") or 0) != readable_count:
            raise RuntimeError("queued analysis status total_clips should match imported clips")

        analysis_started = call("analysis_start_throttled", {
            "cmd": "project.audio_analysis_start",
            "analysis_job_id": analysis_job_id,
            "interval_ms": 200,
            "max_submit_clips": 1,
        })
        started_job = require_analysis_job("analysis_start_throttled", analysis_started)
        if str(started_job.get("analysis_queue_status", "")).lower() != "running":
            raise RuntimeError("analysis start should move job to running")

        time.sleep(0.7)
        throttled_status = call("analysis_status_after_throttle_window", {
            "cmd": "project.audio_analysis_status",
            "analysis_job_id": analysis_job_id,
        })
        throttled_job = require_analysis_job("analysis_status_after_throttle_window", throttled_status)
        submitted_clips = int(throttled_job.get("submitted_clips") or 0)
        submitted_feature_jobs = int(throttled_job.get("submitted_feature_jobs") or 0)
        if submitted_clips != 1 or submitted_feature_jobs != EXPECTED_IMPORT_FEATURES_PER_CLIP:
            raise RuntimeError(
                "throttled analysis should submit exactly one clip / "
                f"{EXPECTED_IMPORT_FEATURES_PER_CLIP} feature, got {submitted_clips}/{submitted_feature_jobs}"
            )
        if readable_count > 1 and str(throttled_job.get("analysis_queue_status", "")).lower() != "paused":
            raise RuntimeError("throttled analysis should pause after max_submit_clips rather than draining the full queue")

        cancelled = call("analysis_cancel_remaining", {
            "cmd": "project.audio_analysis_cancel",
            "analysis_job_id": analysis_job_id,
        })
        cancelled_job = require_analysis_job("analysis_cancel_remaining", cancelled)
        if str(cancelled_job.get("analysis_queue_status", "")).lower() != "cancelled":
            raise RuntimeError("analysis cancel should report cancelled")

        state_after_import = call("state_after_import", {"cmd": "get_project_state"})
        assert_import_visible("state_after_import", state_after_import, created_track_ids, created_clip_ids)
        state_after_import_path = output_path.with_name("project_state_after_import.json")
        write_json_file(state_after_import_path, state_after_import)

        call("reopen_imported_project", {
            "cmd": "open_project",
            "file_path": str(project_path),
            "command_timeout_ms": command_timeout_ms,
        })
        state_after_reopen = call("state_after_reopen", {"cmd": "get_project_state"})
        assert_import_visible("state_after_reopen", state_after_reopen, created_track_ids, created_clip_ids)
        state_after_reopen_path = output_path.with_name("project_state_after_reopen.json")
        write_json_file(state_after_reopen_path, state_after_reopen)

        sealed_summary: Dict[str, Any] = {"status": "skipped"}
        if sealed_folder is not None and sealed_folder.is_dir():
            sealed = call("sealed_stems_readonly_preflight", {
                "cmd": "project.import_preflight",
                "folder_path": str(sealed_folder),
                "recursive": False,
                "media_kinds": ["audio"],
                "intended_mode": "stems_folder",
                "target_policy": "create_tracks",
                "start_time_seconds": 0,
                "command_timeout_ms": command_timeout_ms,
            })
            sealed_summary = {
                "status": "passed",
                "folder": str(sealed_folder),
                "summary": sealed.get("summary"),
            }

        status = "passed"
        failure = ""
    except Exception as exc:
        status = "failed"
        failure = str(exc)
        sealed_summary = locals().get("sealed_summary", {"status": "skipped"})
    finally:
        req.close(0)
        context.term()

    report = {
        "schema_version": "project_stems_import_smoke.v1",
        "created_at": now_iso(),
        "status": status,
        "error": failure,
        "req_url": args.req_url,
        "project_path": str(project_path),
        "training_folder": str(training_folder),
        "sealed_folder": str(sealed_folder) if sealed_folder is not None else "",
        "commands": commands,
        "compact": {label: compact_reply(reply) for label, reply in replies.items()},
        "assertions": {
            "training_preflight_readable_file_count": locals().get("readable_count"),
            "training_preflight_tracks_to_create": locals().get("tracks_to_create"),
            "training_import_tracks_created": locals().get("tracks_created"),
            "training_import_clips_created": locals().get("clips_created"),
            "analysis_job_id": locals().get("analysis_job_id"),
            "expected_import_features_per_clip": EXPECTED_IMPORT_FEATURES_PER_CLIP,
            "analysis_throttled_submitted_clips": locals().get("submitted_clips"),
            "analysis_throttled_submitted_feature_jobs": locals().get("submitted_feature_jobs"),
            "state_after_import_visible": "state_after_import" in replies and status == "passed",
            "state_after_reopen_visible": "state_after_reopen" in replies and status == "passed",
            "state_after_import_path": str(locals().get("state_after_import_path", "")),
            "state_after_reopen_path": str(locals().get("state_after_reopen_path", "")),
            "sealed_readonly_preflight": sealed_summary,
        },
    }
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    if status != "passed":
        print(json.dumps({"status": status, "error": failure, "output": str(output_path)}, ensure_ascii=False))
        return 1
    print(json.dumps({
        "status": "passed",
        "output": str(output_path),
        "project_path": str(project_path),
        "assertions": report["assertions"],
    }, ensure_ascii=False))
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--req-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--training-folder", required=True)
    parser.add_argument("--sealed-folder", default="")
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--req-timeout-ms", type=int, default=180000)
    return run_probe(parser.parse_args())


if __name__ == "__main__":
    sys.exit(main())
