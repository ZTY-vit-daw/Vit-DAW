#!/usr/bin/env python3
"""SegmentationPrimitives real-stack probe (card L2-2-SEG-SMOKE-1).

Runs against a LIVE three-piece stack (VitApp kernel + Godot UI + Go agent,
brought up by seg_primitives_smoke.ps1 via dev_agent_smoke). It imports real
stems through the same ZMQ command chain as project_stems_import_probe.py
(new_project -> save_as_project -> project.import_preflight ->
project.import_folder_as_stems), then requests two segmentation_primitives
bakes of the same imported stem file and asserts:

  A1 each round publishes exactly one dad_l3_segmentation_primitives.v1
     payload on the kernel publish socket (the agent-side L3 whitelist does
     not persist this feature type, so the probe is the persistence leg);
  A2 the three sub-structures are legal: onset_events carries
     detected_count/published_count/truncated, onset_density frames are a
     non-empty array, energy_novelty frames are a non-empty array plus
     max_novelty/mean_novelty summaries;
  A3 the two payloads are byte-identical after dropping the wall-clock
     updated_at stamp and the probe-generated request_id (real-stack version
     of the SEG-1 unit test T1 replay check);
  A4 exit 0 (this probe's own exit code, propagated by the ps1 wrapper).
"""

from __future__ import annotations

import argparse
import copy
import glob
import json
import os
import time
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

PROBE_VERSION = "seg_primitives_probe.v1"
SCHEMA_NAME = "dad_l3_segmentation_primitives.v1"
FEATURE_TYPE = "segmentation_primitives"
ONSET_PUBLISH_LIMIT = 1024
# Fields that differ between two runs by construction (wall clock, probe-side
# request id); everything else must match byte for byte.
DETERMINISM_EXCLUDED_FIELDS = ("updated_at", "request_id")


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime()) + "Z"


def send_command(req_socket: Any, payload: Dict[str, Any]) -> Dict[str, Any]:
    req_socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = req_socket.recv_string()
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {"status": "error", "raw_reply": raw}


def require_ok(label: str, reply: Dict[str, Any]) -> None:
    if str(reply.get("status") or "").lower() != "ok":
        raise RuntimeError(f"{label} failed: {json.dumps(reply, ensure_ascii=False)[:2000]}")


def rows(value: Any) -> List[Dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    return []


def strings(value: Any) -> List[str]:
    if isinstance(value, list):
        return [str(item) for item in value if item]
    return []


def is_int(value: Any) -> bool:
    return isinstance(value, int) and not isinstance(value, bool)


def canonical_payload(payload: Dict[str, Any]) -> str:
    trimmed = copy.deepcopy(payload)
    for field in DETERMINISM_EXCLUDED_FIELDS:
        trimmed.pop(field, None)
    return json.dumps(trimmed, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def first_determinism_difference(left: Dict[str, Any], right: Dict[str, Any]) -> List[str]:
    """Best-effort list of top-level fields that differ (for A3 forensics)."""
    differences: List[str] = []
    keys = sorted(set(list(left.keys()) + list(right.keys())))
    for key in keys:
        if key in DETERMINISM_EXCLUDED_FIELDS:
            continue
        left_json = json.dumps(left.get(key, "<absent>"), sort_keys=True, ensure_ascii=False)
        right_json = json.dumps(right.get(key, "<absent>"), sort_keys=True, ensure_ascii=False)
        if left_json != right_json:
            differences.append(key)
        if len(differences) >= 16:
            break
    return differences


def stage_import_subset(training_folder: Path, max_files: int, staging_dir: Path) -> Path:
    """Copy the first N wavs of the training folder into the run-local staging
    dir and return it. The real 61-stem folder queues one serialized L3 bake
    per clip (the pool is deliberately single-threaded for determinism), which
    starves the probe's own bakes for tens of minutes; a small subset keeps
    the same import command chain and the same source material while keeping
    the queue depth bounded."""
    wavs = sorted(glob.glob(str(training_folder / "*.wav")))
    if not wavs:
        raise RuntimeError(f"no wav files found in training folder: {training_folder}")
    if max_files <= 0 or len(wavs) <= max_files:
        return training_folder
    staging_dir.mkdir(parents=True, exist_ok=True)
    import shutil

    staged = []
    for source in wavs[:max_files]:
        target = staging_dir / Path(source).name
        shutil.copy2(source, target)
        staged.append(target)
    return staging_dir


def resolve_bake_file(import_reply: Dict[str, Any], training_folder: Path) -> str:
    # Prefer the file the kernel itself associated with the first created
    # track (track_plan rows carry the source path); fall back to the first
    # wav of the imported folder, which is the same material the import used.
    for row in rows(import_reply.get("track_plan")):
        for key in ("file_path", "source_path", "source_file", "media_path"):
            value = str(row.get(key) or "").strip()
            if value and os.path.isfile(value):
                return value
    wavs = sorted(glob.glob(str(training_folder / "*.wav")))
    if not wavs:
        raise RuntimeError(f"no wav files found in training folder: {training_folder}")
    return wavs[0]


L3_SUMMARY_FEATURES = ("band_energy_summary", "stereo_relation_summary", "loudness_summary")


def sanitize_counter_key(key: str) -> str:
    # PS 5.1 ConvertFrom-Json rejects empty-string property names; keep every
    # observed-traffic key non-empty and property-name safe.
    cleaned = "".join(ch if (ch.isalnum() or ch in "_.:-") else "_" for ch in key)
    return cleaned if cleaned else "<empty>"


class SubscriptionDrain:
    """Collects segmentation_primitives publish events while counting the
    background traffic (the stems-import-triggered L3 analysis) as evidence
    that the real pipeline kept running."""

    def __init__(self, sub_socket: Any):
        self.sub_socket = sub_socket
        self.observed_events: Dict[str, int] = {}
        self.l3_summary_count = 0

    def _note_event(self, event: Dict[str, Any]) -> bool:
        command = str(event.get("command") or event.get("cmd") or "")
        feature_type = str(event.get("feature_type") or "")
        counter_key = f"{command}:{feature_type}" if feature_type else command
        counter_key = sanitize_counter_key(counter_key)
        self.observed_events[counter_key] = self.observed_events.get(counter_key, 0) + 1
        if command == "audio_feature_data_ready" and feature_type in L3_SUMMARY_FEATURES:
            self.l3_summary_count += 1
        return command == "audio_feature_data_ready" and feature_type == FEATURE_TYPE

    def _recv_note(self, exclude_request_ids: Tuple[str, ...]) -> Tuple[Optional[Dict[str, Any]], bool]:
        """Drain one available event; returns (matched payload, duplicate)."""
        import zmq  # local import keeps --help usable without pyzmq

        try:
            raw = self.sub_socket.recv_string(zmq.NOBLOCK)
        except zmq.Again:
            return None, False
        except Exception:
            return None, False
        try:
            event = json.loads(raw)
        except json.JSONDecodeError:
            self.observed_events["<unparsable>"] = self.observed_events.get("<unparsable>", 0) + 1
            return None, False
        if not isinstance(event, dict):
            return None, False
        if self._note_event(event):
            if str(event.get("request_id") or "") not in exclude_request_ids:
                return event, False
            return None, True
        return None, False

    def pump_until(self, duration_seconds: float, stop_condition=None) -> bool:
        """Pump the subscription for a bounded time (or until stop_condition
        is true), keeping the socket drained and the counters moving. Returns
        True when the condition was met (or none was given), False when the
        duration expired first."""
        import zmq  # local import keeps --help usable without pyzmq

        poller = zmq.Poller()
        poller.register(self.sub_socket, zmq.POLLIN)
        deadline = time.monotonic() + duration_seconds
        while time.monotonic() < deadline:
            if stop_condition is not None and stop_condition():
                return self._drain_buffered(poller)
            socks = dict(poller.poll(timeout=500))
            if self.sub_socket in socks:
                self._recv_note(())
        return self._drain_buffered(poller) if stop_condition is not None and stop_condition() else (stop_condition is None)

    def _drain_buffered(self, poller: Any) -> bool:
        while True:
            event, _ = self._recv_note(())
            if event is None and not self._pending(poller):
                return True

    def _pending(self, poller: Any) -> bool:
        import zmq

        socks = dict(poller.poll(timeout=0))
        return self.sub_socket in socks

    def wait_for_payload(self, timeout_seconds: float, exclude_request_ids: Tuple[str, ...] = ()) -> Tuple[Optional[Dict[str, Any]], int]:
        import zmq  # local import keeps --help usable without pyzmq

        poller = zmq.Poller()
        poller.register(self.sub_socket, zmq.POLLIN)
        deadline = time.monotonic() + timeout_seconds
        duplicates = 0
        while time.monotonic() < deadline:
            socks = dict(poller.poll(timeout=500))
            if self.sub_socket not in socks:
                continue
            event, is_duplicate = self._recv_note(exclude_request_ids)
            if event is not None:
                return event, duplicates
            if is_duplicate:
                duplicates += 1
        return None, duplicates


def assert_structure(payload: Dict[str, Any], failures: List[str]) -> Dict[str, Any]:
    """A2: the three sub-structures must be legal."""
    details: Dict[str, Any] = {}

    onset_events = payload.get("onset_events")
    if not isinstance(onset_events, dict):
        failures.append("A2 onset_events object missing")
    else:
        detected = onset_events.get("detected_count")
        published = onset_events.get("published_count")
        truncated = onset_events.get("truncated")
        if not is_int(detected) or detected < 0:
            failures.append(f"A2 onset_events.detected_count not a non-negative int: {detected!r}")
        if not is_int(published) or published < 0:
            failures.append(f"A2 onset_events.published_count not a non-negative int: {published!r}")
        if not isinstance(truncated, bool):
            failures.append(f"A2 onset_events.truncated not a bool: {truncated!r}")
        if is_int(detected) and is_int(published):
            expected_published = min(detected, ONSET_PUBLISH_LIMIT)
            if published != expected_published:
                failures.append(
                    f"A2 onset_events.published_count={published} != min(detected_count, {ONSET_PUBLISH_LIMIT})={expected_published}"
                )
            if truncated != (detected > ONSET_PUBLISH_LIMIT):
                failures.append(f"A2 onset_events.truncated={truncated} disagrees with detected_count={detected}")
        details["onset_events"] = {
            "detected_count": detected,
            "published_count": published,
            "truncated": truncated,
            "method": onset_events.get("method"),
        }

    onset_density = payload.get("onset_density")
    if not isinstance(onset_density, dict):
        failures.append("A2 onset_density object missing")
    else:
        density_frames = onset_density.get("frames")
        if not isinstance(density_frames, list) or len(density_frames) == 0:
            failures.append("A2 onset_density.frames not a non-empty array")
        else:
            details["onset_density_frames"] = len(density_frames)

    energy_novelty = payload.get("energy_novelty")
    if not isinstance(energy_novelty, dict):
        failures.append("A2 energy_novelty object missing")
    else:
        novelty_frames = energy_novelty.get("frames")
        if not isinstance(novelty_frames, list) or len(novelty_frames) == 0:
            failures.append("A2 energy_novelty.frames not a non-empty array")
        else:
            details["energy_novelty_frames"] = len(novelty_frames)
        max_novelty = energy_novelty.get("max_novelty")
        mean_novelty = energy_novelty.get("mean_novelty")
        if not isinstance(max_novelty, (int, float)) or isinstance(max_novelty, bool):
            failures.append(f"A2 energy_novelty.max_novelty not numeric: {max_novelty!r}")
        if not isinstance(mean_novelty, (int, float)) or isinstance(mean_novelty, bool):
            failures.append(f"A2 energy_novelty.mean_novelty not numeric: {mean_novelty!r}")
        details["energy_novelty_summary"] = {"max_novelty": max_novelty, "mean_novelty": mean_novelty}

    return details


def run_probe(args: argparse.Namespace) -> int:
    import zmq

    output_path = Path(args.output).resolve()
    output_path.parent.mkdir(parents=True, exist_ok=True)
    project_path = Path(args.project_path).resolve()
    project_path.parent.mkdir(parents=True, exist_ok=True)
    training_folder = Path(args.training_folder).resolve()
    if not training_folder.is_dir():
        raise RuntimeError(f"training folder does not exist: {training_folder}")
    import_folder = stage_import_subset(training_folder, args.max_import_files, output_path.parent / "stems_staging")
    summary_import_folder = str(import_folder)
    bake_fallback_folder = import_folder

    summary: Dict[str, Any] = {
        "schema_version": PROBE_VERSION,
        "status": "running",
        "started_at": now_iso(),
        "req_url": args.req_url,
        "sub_url": args.sub_url,
        "training_folder": str(training_folder),
        "import_folder": summary_import_folder,
        "import_folder_staged": str(import_folder) != str(training_folder),
        "project_path": str(project_path),
        "commands": [],
        "assertions": {
            "A1_payload_present": None,
            "A2_structure_legal": None,
            "A3_determinism": None,
        },
        "failures": [],
    }
    failures: List[str] = summary["failures"]

    def call(label: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        reply = send_command(req, payload)
        summary["commands"].append({"label": label, "cmd": payload.get("cmd"), "status": reply.get("status")})
        return reply

    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.SNDTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(args.req_url)

    sub = context.socket(zmq.SUB)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.setsockopt(zmq.LINGER, 0)
    sub.connect(args.sub_url)
    drain = SubscriptionDrain(sub)
    # Give the SUB pipe a moment to establish before the first bake.
    time.sleep(1.0)

    payloads: List[Dict[str, Any]] = []
    try:
        reply = call("ping", {"cmd": "ping"})
        require_ok("ping", reply)

        reply = call("new_project", {"cmd": "new_project"})
        require_ok("new_project", reply)

        reply = call("save_as_project", {"cmd": "save_as_project", "file_path": str(project_path)})
        require_ok("save_as_project", reply)

        preflight = call("stems_import_preflight", {
            "cmd": "project.import_preflight",
            "folder_path": str(import_folder),
            "recursive": False,
            "media_kinds": ["audio"],
            "intended_mode": "stems_folder",
            "target_policy": "create_tracks",
            "start_time_seconds": 0,
        })
        require_ok("project.import_preflight", preflight)
        preflight_summary = preflight.get("summary")
        readable_count = int(preflight_summary.get("readable_file_count") or 0) if isinstance(preflight_summary, dict) else 0
        if readable_count <= 0:
            raise RuntimeError("project.import_preflight found no readable audio files")
        summary["readable_file_count"] = readable_count

        imported = call("stems_import_folder", {
            "cmd": "project.import_folder_as_stems",
            "folder_path": str(import_folder),
            "recursive": False,
            "target_policy": "create_tracks",
            "start_time_seconds": 0,
            "confirmed": True,
            "skip_unreadable": False,
        })
        require_ok("project.import_folder_as_stems", imported)
        import_summary = imported.get("summary")
        tracks_created = int(import_summary.get("tracks_created") or 0) if isinstance(import_summary, dict) else 0
        clips_created = int(import_summary.get("clips_created") or 0) if isinstance(import_summary, dict) else 0
        created_track_ids = strings(imported.get("created_track_ids"))
        if tracks_created != readable_count or clips_created != readable_count or not created_track_ids:
            raise RuntimeError(
                f"stems import created tracks/clips {tracks_created}/{clips_created} "
                f"(readable={readable_count}, track_ids={len(created_track_ids)})"
            )
        summary["tracks_created"] = tracks_created
        summary["clips_created"] = clips_created

        # The stems import queues background L3 analysis for every clip. The
        # segmentation bake shares the same serialized L3 pool, so wait for
        # that queue to drain (bounded) before baking - otherwise the bake
        # latency is dominated by jobs we do not assert on.
        analysis_job_id = str(imported.get("analysis_job_id") or "")
        background: Dict[str, Any] = {"analysis_job_id": analysis_job_id, "polls": []}
        if analysis_job_id:
            stable_rounds = 0
            last_signature = None
            background_deadline = time.monotonic() + args.background_analysis_timeout_sec
            while time.monotonic() < background_deadline:
                time.sleep(5.0)
                status_reply = send_command(req, {"cmd": "project.audio_analysis_status", "analysis_job_id": analysis_job_id})
                job = status_reply.get("analysis_job") if isinstance(status_reply.get("analysis_job"), dict) else {}
                if not job:
                    background["polls"].append({"error": str(status_reply.get("status") or "no analysis_job in reply")})
                    stable_rounds += 1
                    if stable_rounds >= 6:
                        break
                    continue
                pending = int(job.get("pending_clips") or 0)
                submitted_clips = int(job.get("submitted_clips") or 0)
                submitted_features = int(job.get("submitted_feature_jobs") or 0)
                signature = (pending, submitted_clips, submitted_features)
                if len(background["polls"]) < 32:
                    background["polls"].append({
                        "pending_clips": pending,
                        "submitted_clips": submitted_clips,
                        "submitted_feature_jobs": submitted_features,
                        "analysis_queue_status": job.get("analysis_queue_status"),
                    })
                if pending == 0 and signature == last_signature:
                    stable_rounds += 1
                    if stable_rounds >= 3:
                        break
                else:
                    stable_rounds = 0
                last_signature = signature
            background["final_signature"] = list(last_signature) if last_signature else None
        summary["background_analysis"] = background

        bake_file = resolve_bake_file(imported, bake_fallback_folder)
        bake_track_id = created_track_ids[0]
        summary["bake"] = {
            "file_path": bake_file,
            "track_id": bake_track_id,
            "rounds": [],
        }

        excluded_request_ids: Tuple[str, ...] = ()
        expected_summaries = 3 * tracks_created
        for round_index in (1, 2):
            if round_index > 1:
                # AudioFeatureService::requestBake merges identical bake
                # requests inside a 30s window (merge key ignores request_id;
                # same-priority re-requests are dropped silently). Round 2
                # must be issued after that window expires, or it never
                # enters the L3 pool at all - the exact failure of runs
                # 20260926_224515/230125. While waiting, keep the SUB pumped
                # (background analysis keeps publishing) and let the queued
                # background L3 bakes finish so round 2 does not wait behind
                # them inside the single-threaded pool.
                merge_wait = args.merge_window_sec + 5.0
                drain.pump_until(merge_wait)
                summary_deadline_hit = False
                if expected_summaries > 0:
                    summary_deadline_hit = not drain.pump_until(
                        args.background_quiet_timeout_sec,
                        stop_condition=lambda: drain.l3_summary_count >= expected_summaries,
                    )
                summary["bake"]["round_gap"] = {
                    "merge_window_seconds": merge_wait,
                    "expected_l3_summary_events": expected_summaries,
                    "observed_l3_summary_events": drain.l3_summary_count,
                    "background_quiet_timeout_hit": summary_deadline_hit,
                }
            request_id = f"seg_primitives_smoke_r{round_index}_{int(time.time() * 1000)}"
            bake_reply = call(f"warm_bake_round{round_index}", {
                "cmd": "warm_waveform_bake",
                "track_id": bake_track_id,
                "file_path": bake_file,
                "allow_file_source": True,
                "feature_type": FEATURE_TYPE,
                "priority": "on_demand",
                "source_id": bake_file,
                "request_id": request_id,
            })
            require_ok(f"warm_waveform_bake round {round_index}", bake_reply)
            payload, duplicates = drain.wait_for_payload(
                timeout_seconds=args.bake_timeout_sec,
                exclude_request_ids=excluded_request_ids,
            )
            excluded_request_ids = excluded_request_ids + (request_id,)
            round_entry: Dict[str, Any] = {
                "round": round_index,
                "request_id": request_id,
                "bake_feature_type": bake_reply.get("feature_type"),
                "received": payload is not None,
                "duplicate_events_skipped": duplicates,
            }
            if payload is None:
                failures.append(
                    f"A1 round {round_index}: no {FEATURE_TYPE} payload on the publish socket within "
                    f"{args.bake_timeout_sec}s (L3 pipeline did not emit the feature - card stop condition)"
                )
                summary["bake"]["rounds"].append(round_entry)
                break
            payload_path = output_path.parent / f"payload_round{round_index}.json"
            payload_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
            round_entry["payload_path"] = str(payload_path)
            round_entry["schema_version"] = payload.get("schema_version")
            round_entry["status"] = payload.get("status")
            round_entry["quality_status"] = payload.get("quality_status")
            if payload.get("schema_version") != SCHEMA_NAME:
                failures.append(
                    f"A1 round {round_index}: schema_version {payload.get('schema_version')!r} != {SCHEMA_NAME!r}"
                )
            payloads.append(payload)
            summary["bake"]["rounds"].append(round_entry)

        summary["observed_publish_traffic"] = drain.observed_events

        # A1 covers payload presence and schema name only; A2/A3 own their own
        # failure entries, so a structure failure does not demote A1.
        summary["assertions"]["A1_payload_present"] = len(payloads) == 2 and all(
            p.get("schema_version") == SCHEMA_NAME for p in payloads
        )

        if payloads:
            failures_before = len(failures)
            structure_details = assert_structure(payloads[0], failures)
            summary["structure_details"] = structure_details
            summary["assertions"]["A2_structure_legal"] = len(failures) == failures_before
        else:
            summary["assertions"]["A2_structure_legal"] = False

        if len(payloads) == 2:
            canonical_left = canonical_payload(payloads[0])
            canonical_right = canonical_payload(payloads[1])
            payload_left_path = output_path.parent / "canonical_round1.json"
            payload_right_path = output_path.parent / "canonical_round2.json"
            payload_left_path.write_text(canonical_left, encoding="utf-8")
            payload_right_path.write_text(canonical_right, encoding="utf-8")
            if canonical_left == canonical_right:
                summary["assertions"]["A3_determinism"] = True
                summary["canonical_bytes"] = len(canonical_left.encode("utf-8"))
            else:
                summary["assertions"]["A3_determinism"] = False
                summary["determinism_difference_fields"] = first_determinism_difference(payloads[0], payloads[1])
                failures.append(
                    "A3 two-round payloads differ after excluding "
                    f"{list(DETERMINISM_EXCLUDED_FIELDS)} (functional red line - card stop condition)"
                )
        else:
            summary["assertions"]["A3_determinism"] = False
    finally:
        try:
            req.close(linger=0)
        except Exception:
            pass
        try:
            sub.close(linger=0)
        except Exception:
            pass
        context.term()

    summary["finished_at"] = now_iso()
    summary["status"] = "passed" if not failures else "failed"
    output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({
        "status": summary["status"],
        "assertions": summary["assertions"],
        "failures": failures,
        "output": str(output_path),
    }, ensure_ascii=False, indent=2))
    return 0 if not failures else 1


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--req-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--sub-url", default="tcp://127.0.0.1:5556")
    parser.add_argument("--training-folder", required=True)
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--bake-timeout-sec", type=int, default=900)
    parser.add_argument("--background-analysis-timeout-sec", type=int, default=600)
    parser.add_argument("--merge-window-sec", type=int, default=30,
                        help="requestBake merges identical requests within 30s (merge key ignores request_id); round 2 waits past this window")
    parser.add_argument("--background-quiet-timeout-sec", type=int, default=300,
                        help="round-2 waits until the import-triggered L3 summary events reach 3 per clip, bounded by this budget")
    parser.add_argument("--max-import-files", type=int, default=0,
                        help="0 imports the whole folder; N>0 stages the first N wavs into the run dir and imports that subset (keeps the serialized L3 queue bounded)")
    parser.add_argument("--req-timeout-ms", type=int, default=300000)
    args = parser.parse_args()
    try:
        return run_probe(args)
    except Exception as exc:  # surface a structured failure artifact as well
        try:
            output_path = Path(args.output).resolve()
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_text(json.dumps({
                "schema_version": PROBE_VERSION,
                "status": "failed",
                "error": f"{type(exc).__name__}: {exc}",
                "finished_at": now_iso(),
            }, ensure_ascii=False, indent=2), encoding="utf-8")
        except Exception:
            pass
        print(f"seg_primitives_probe failed: {type(exc).__name__}: {exc}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
