#!/usr/bin/env python3
"""Smoke-test kernel edit-state change registration (KERNEL-SETTEMPO-PERSIST-1).

Legs (against the live kernel ZMQ REQ face, same pattern as
automation_curve_smoke.py / clip_fade_gain_smoke.py):

1.  set_tempo open-form roundtrip: open_project an existing .vit, set_tempo
    96 (runAutoPersist saves in the same synchronous dispatch), reopen the
    project from disk and require the persisted tempo to be exactly 96 —
    the byte-fidelity analogue of the GEN-CTX-KEYFIELD-1 set_at check: the
    value must survive a real disk roundtrip, not an in-memory echo. The
    project file hash must also change across the write (auto-persist
    really touched the disk).
2.  clear_project roundtrip: with a clip imported and saved, clear_project
    (its own runAutoPersist, no explicit save in between) followed by a
    reopen must show zero clips — before the fix the non-forced save
    silently skipped the write and the clip came back from disk.
3.  undo roundtrip: set_tempo 133, undo (back to 96), explicit
    save_project, reopen -> 96 (the undo effect reached the disk).
4.  redo roundtrip: set_tempo 144, undo, redo (back to 144), explicit
    save_project, reopen -> 144 (the redo effect reached the disk).

Exit code 0 only if every leg passes.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import tempfile
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


def make_socket(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    sock = ctx.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    if hasattr(zmq, "REQ_RELAXED"):
        sock.setsockopt(zmq.REQ_RELAXED, 1)
    if hasattr(zmq, "REQ_CORRELATE"):
        sock.setsockopt(zmq.REQ_CORRELATE, 1)
    sock.connect(url)
    return sock


def send_command(sock: zmq.Socket, payload: dict[str, Any]) -> dict[str, Any]:
    cmd = payload.get("cmd")
    sock.send_string(json.dumps(payload, ensure_ascii=False))
    raw = sock.recv()
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8", errors="replace")
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON reply for {cmd}: {raw[:1000]!r}") from exc
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object reply for {cmd}: {reply!r}")
    return reply


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_ok(label: str, reply: dict[str, Any]) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status in {"ok", "success", "completed"}, f"{label} failed: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def wait_for_kernel(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    deadline = time.time() + (timeout_ms / 1000.0)
    last_error: Exception | None = None
    while time.time() < deadline:
        sock = make_socket(ctx, url, timeout_ms)
        try:
            reply = send_command(sock, {"cmd": "ping"})
            if str(reply.get("status", "")).lower() == "ok":
                return sock
        except Exception as exc:  # noqa: BLE001 - probe loop
            last_error = exc
        sock.close(0)
        time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def disclosed_tempo(state: dict[str, Any]) -> float:
    music_context = state.get("music_context")
    require(isinstance(music_context, dict), f"music_context missing in get_project_state: {sorted(state)}")
    tempo = music_context.get("tempo_bpm")
    require(isinstance(tempo, (int, float)), f"music_context.tempo_bpm missing/not numeric: {music_context}")
    return float(tempo)


def total_clips(state: dict[str, Any]) -> int:
    tracks = state.get("tracks")
    require(isinstance(tracks, list), "get_project_state did not return a tracks array")
    count = 0
    for track in tracks:
        if not isinstance(track, dict):
            continue
        clips = track.get("clips")
        if isinstance(clips, list):
            count += len(clips)
    return count


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=30000)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--audio-file", default="")
    parser.add_argument("--output", default="")
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parents[1]

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    else:
        temp_dir = Path(tempfile.gettempdir()) / "vit_daw_settempo_persist_smoke"
        temp_dir.mkdir(parents=True, exist_ok=True)
        project_path = temp_dir / f"settempo_persist_{int(time.time())}.vit"

    audio_file = Path(args.audio_file) if args.audio_file else repo_root / "test_target_3s.wav"
    require(audio_file.is_file(), f"audio fixture missing: {audio_file}")

    command_timeout_ms = max(args.timeout_ms, 30000)

    ctx = zmq.Context.instance()
    sock = wait_for_kernel(ctx, args.url, args.timeout_ms)
    run_id = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ") + "_" + uuid.uuid4().hex[:8]
    summary: dict[str, Any] = {
        "status": "running",
        "run_id": run_id,
        "url": args.url,
        "project_path": str(project_path),
        "audio_file": str(audio_file),
        "checks": [],
    }

    def open_project() -> dict[str, Any]:
        return require_ok(
            "open_project",
            send_command(sock, {"cmd": "open_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )

    def get_state() -> dict[str, Any]:
        return require_ok("get_project_state", send_command(sock, {"cmd": "get_project_state"}))

    try:
        # Fresh project on disk (new default tempo 120), then enter the
        # open-form: every leg below starts from open_project so the save
        # path inside runAutoPersist is the non-forced
        # EditFileOperations::save (the silent-skip gate this card fixes).
        require_ok("new_project", send_command(sock, {"cmd": "new_project", "command_timeout_ms": command_timeout_ms}))
        summary["checks"].append("new_project")
        require_ok(
            "save_as_project",
            send_command(sock, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )
        summary["checks"].append("save_as_project")

        # ---- LEG 1: set_tempo open-form roundtrip (byte fidelity) --------
        # File-hash gate comes FIRST so a pre-fix kernel fails right here
        # (silent skip: auto-persist reports success but the file never
        # changes) even though it predates the music_context disclosure.
        open_project()
        hash_before = file_sha256(project_path)
        set_reply = require_ok("set_tempo 96", send_command(sock, {"cmd": "set_tempo", "bpm": 96, "command_timeout_ms": command_timeout_ms}))
        require(abs(float(set_reply.get("bpm", -1)) - 96.0) < 1e-9, f"set_tempo must echo bpm 96: {set_reply}")
        hash_after = file_sha256(project_path)
        require(hash_after != hash_before, "set_tempo open-form must write the project file (auto-persist silent skip suspected: hash unchanged)")
        summary["hash_before_set_tempo"] = hash_before
        summary["hash_after_set_tempo"] = hash_after
        summary["checks"].append("set_tempo.open_form.file_written")

        baseline_tempo = disclosed_tempo(get_state())
        require(abs(baseline_tempo - 96.0) < 1e-9, f"in-memory tempo must be 96 right after set_tempo, got {baseline_tempo}")
        summary["tempo_in_memory_bpm"] = baseline_tempo

        open_project()
        reloaded_tempo = disclosed_tempo(get_state())
        require(abs(reloaded_tempo - 96.0) < 1e-9, f"set_tempo 96 did not survive save/reload roundtrip: disclosed {reloaded_tempo}")
        summary["tempo_after_reopen_bpm"] = reloaded_tempo
        summary["checks"].append("set_tempo.open_form.roundtrip_96")

        # ---- LEG 2: clear_project roundtrip -------------------------------
        track_reply = require_ok("add_audio_track", send_command(sock, {"cmd": "add_audio_track"}))
        track_id = str(track_reply.get("track_id") or "").strip()
        require(track_id, f"add_audio_track did not return track_id: {track_reply}")
        import_reply = require_ok(
            "import_media_to_track",
            send_command(
                sock,
                {
                    "cmd": "import_media_to_track",
                    "track_id": track_id,
                    "file_path": str(audio_file),
                    "media_type": "audio",
                    "mode": "non_destructive",
                    "start_time": 0.0,
                    "command_timeout_ms": command_timeout_ms,
                },
            ),
        )
        require(str(import_reply.get("clip_id") or "").strip(), f"import_media_to_track did not return clip_id: {import_reply}")
        require_ok(
            "save_project (persist imported clip)",
            send_command(sock, {"cmd": "save_project", "command_timeout_ms": command_timeout_ms}),
        )

        open_project()
        clips_before_clear = total_clips(get_state())
        require(clips_before_clear >= 1, f"clip must be on disk after save_project (found {clips_before_clear}) — precondition failed")
        summary["clips_before_clear"] = clips_before_clear
        summary["checks"].append("clear_project.precondition_clip_on_disk")

        require_ok("clear_project", send_command(sock, {"cmd": "clear_project", "command_timeout_ms": command_timeout_ms}))
        open_project()
        clips_after_reopen = total_clips(get_state())
        require(clips_after_reopen == 0, f"clear_project did not survive save/reload roundtrip: {clips_after_reopen} clips came back from disk")
        summary["clips_after_clear_reopen"] = clips_after_reopen
        summary["checks"].append("clear_project.roundtrip_zero_clips")

        # ---- LEG 3: undo roundtrip ----------------------------------------
        open_project()
        require_ok("set_tempo 133", send_command(sock, {"cmd": "set_tempo", "bpm": 133, "command_timeout_ms": command_timeout_ms}))
        require_ok("undo", send_command(sock, {"cmd": "undo", "command_timeout_ms": command_timeout_ms}))
        undone_tempo = disclosed_tempo(get_state())
        require(abs(undone_tempo - 96.0) < 1e-9, f"undo must restore 96 bpm in memory, got {undone_tempo}")
        require_ok(
            "save_project (persist undo)",
            send_command(sock, {"cmd": "save_project", "command_timeout_ms": command_timeout_ms}),
        )
        open_project()
        after_undo_reload = disclosed_tempo(get_state())
        require(abs(after_undo_reload - 96.0) < 1e-9, f"undo effect did not survive save/reload roundtrip: disclosed {after_undo_reload}")
        summary["undo_roundtrip_tempo_bpm"] = after_undo_reload
        summary["checks"].append("undo.roundtrip_96")

        # ---- LEG 4: redo roundtrip ----------------------------------------
        open_project()
        require_ok("set_tempo 144", send_command(sock, {"cmd": "set_tempo", "bpm": 144, "command_timeout_ms": command_timeout_ms}))
        require_ok("undo", send_command(sock, {"cmd": "undo", "command_timeout_ms": command_timeout_ms}))
        require_ok("redo", send_command(sock, {"cmd": "redo", "command_timeout_ms": command_timeout_ms}))
        redone_tempo = disclosed_tempo(get_state())
        require(abs(redone_tempo - 144.0) < 1e-9, f"redo must restore 144 bpm in memory, got {redone_tempo}")
        require_ok(
            "save_project (persist redo)",
            send_command(sock, {"cmd": "save_project", "command_timeout_ms": command_timeout_ms}),
        )
        open_project()
        after_redo_reload = disclosed_tempo(get_state())
        require(abs(after_redo_reload - 144.0) < 1e-9, f"redo effect did not survive save/reload roundtrip: disclosed {after_redo_reload}")
        summary["redo_roundtrip_tempo_bpm"] = after_redo_reload
        summary["checks"].append("redo.roundtrip_144")

        summary["status"] = "pass"
        summary["verdict"] = "PASS"
        print("settempo persist smoke: PASS")
        print(json.dumps({"checks": summary["checks"]}, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001 - top-level smoke guard
        summary["status"] = "fail"
        summary["verdict"] = "FAIL"
        summary["error"] = str(exc)
        print(f"settempo persist smoke: FAIL: {exc}", file=sys.stderr)
        return 1
    finally:
        if args.output:
            output_path = Path(args.output)
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        else:
            print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(main())
