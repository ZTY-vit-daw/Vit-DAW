#!/usr/bin/env python3
"""Smoke-test clip fade/gain commands, snapshot exposure, and persistence."""

from __future__ import annotations

import argparse
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


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def vsp_envelope(
    session_id: str,
    payload: dict[str, Any],
    timeout_ms: int,
    *,
    channel: str = "command",
    message_type: str = "command.request",
    schema: str = "vsp.command.request.v1",
) -> dict[str, Any]:
    return {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": new_id("msg"),
        "session_id": session_id,
        "client_id": "smoke.clip_fade_gain",
        "role": "test",
        "channel": channel,
        "type": message_type,
        "created_at": utc_now(),
        "trace_id": new_id("trace"),
        "request_id": new_id("req"),
        "transaction_id": new_id("tx"),
        "command_timeout_ms": timeout_ms,
        "payload": payload,
    }


def vsp_hello(sock: zmq.Socket, timeout_ms: int) -> str:
    reply = send_command(
        sock,
        {
            "vsp_version": "1.0",
            "schema": "vsp.session.hello.v1",
            "message_id": new_id("msg"),
            "session_id": "session_pending",
            "client_id": "smoke.clip_fade_gain",
            "role": "test",
            "channel": "session",
            "type": "session.hello",
            "created_at": utc_now(),
            "trace_id": new_id("trace"),
            "payload": {
                "client_name": "Clip fade/gain smoke",
                "client_version": "0.1.0",
                "protocol_min": "1.0",
                "protocol_max": "1.0",
                "wants": ["command.request", "project.read"],
                "transport_bindings": ["legacy.zmq_reqrep"],
            },
            "command_timeout_ms": timeout_ms,
        },
    )
    require(reply.get("type") == "session.hello_ack", f"VSP hello failed: {reply}")
    session_id = str(reply.get("session_id") or "").strip()
    require(bool(session_id), f"VSP hello did not return session_id: {reply}")
    return session_id


def send_vsp_command(sock: zmq.Socket, session_id: str, command: str, args: dict[str, Any], timeout_ms: int) -> dict[str, Any]:
    reply = send_command(
        sock,
        vsp_envelope(
            session_id,
            {
                "command": command,
                "args": args,
            },
            timeout_ms,
        ),
    )
    require(reply.get("type") == "command.response", f"VSP {command} returned unexpected reply: {reply}")
    payload = reply.get("payload")
    require(isinstance(payload, dict), f"VSP {command} payload missing: {reply}")
    require(payload.get("legacy_command") == command, f"VSP {command} mapped to unexpected legacy command: {payload}")
    legacy_reply = payload.get("legacy_reply")
    require(isinstance(legacy_reply, dict), f"VSP {command} legacy_reply missing: {payload}")
    return require_ok(f"VSP {command}", legacy_reply)


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
        sock = make_socket(ctx, url, min(timeout_ms, 5000))
        try:
            require_ok("ping", send_command(sock, {"cmd": "ping"}))
            return sock
        except Exception as exc:  # noqa: BLE001 - top-level smoke retry diagnostics.
            last_error = exc
            sock.close(0)
            time.sleep(0.25)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def approx(actual: Any, expected: float, tolerance: float = 0.001) -> bool:
    try:
        return abs(float(actual) - expected) <= tolerance
    except (TypeError, ValueError):
        return False


def find_clip(state: dict[str, Any], clip_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    tracks = state.get("tracks")
    require(isinstance(tracks, list), "get_project_state tracks must be an array")
    for raw_track in tracks:
        if not isinstance(raw_track, dict):
            continue
        clips = raw_track.get("clips")
        if not isinstance(clips, list):
            continue
        for raw_clip in clips:
            if not isinstance(raw_clip, dict):
                continue
            candidate = str(raw_clip.get("id") or raw_clip.get("clip_id") or "").strip()
            if candidate == clip_id:
                return raw_track, raw_clip
    raise RuntimeError(f"clip {clip_id} not found in project snapshot")


def assert_fade(label: str, row: dict[str, Any], fade_in: float, fade_out: float) -> None:
    require(approx(row.get("fade_in_seconds"), fade_in), f"{label}: fade_in_seconds mismatch: {row}")
    require(approx(row.get("fade_out_seconds"), fade_out), f"{label}: fade_out_seconds mismatch: {row}")
    require("fade_in_curve" in row, f"{label}: fade_in_curve missing: {row}")
    require("fade_out_curve" in row, f"{label}: fade_out_curve missing: {row}")
    require("auto_crossfade" in row, f"{label}: auto_crossfade missing: {row}")


def assert_gain(label: str, row: dict[str, Any], gain_db: float) -> None:
    require(approx(row.get("gain_db"), gain_db), f"{label}: gain_db mismatch: {row}")
    require(approx(row.get("clip_gain_db"), gain_db), f"{label}: clip_gain_db mismatch: {row}")
    require("pan" in row and "clip_pan" in row, f"{label}: pan fields missing: {row}")
    require("mute" in row and "clip_mute" in row, f"{label}: mute fields missing: {row}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=30000)
    parser.add_argument("--audio-file", default=r"D:\Vit_DAW\test_target_3s.wav")
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", default="")
    args = parser.parse_args()

    audio_file = Path(args.audio_file).resolve()
    require(audio_file.is_file(), f"audio file does not exist: {audio_file}")

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    else:
        temp_dir = Path(tempfile.gettempdir()) / "vit_daw_clip_fade_gain_smoke"
        temp_dir.mkdir(parents=True, exist_ok=True)
        project_path = temp_dir / f"clip_fade_gain_{int(time.time())}.vit"

    fade_in = 0.123
    fade_out = 0.234
    gain_db = -3.5
    command_timeout_ms = max(args.timeout_ms, 30000)

    ctx = zmq.Context.instance()
    sock = wait_for_kernel(ctx, args.url, args.timeout_ms)
    summary: dict[str, Any] = {
        "status": "running",
        "url": args.url,
        "audio_file": str(audio_file),
        "project_path": str(project_path),
        "checks": [],
    }

    try:
        require_ok("new_project", send_command(sock, {"cmd": "new_project", "command_timeout_ms": command_timeout_ms}))
        summary["checks"].append("new_project")

        require_ok(
            "save_as_project_before_import",
            send_command(sock, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )
        summary["checks"].append("save_as_project_before_import")

        track_reply = require_ok("add_audio_track", send_command(sock, {"cmd": "add_audio_track"}))
        track_id = str(track_reply.get("track_id") or "").strip()
        require(track_id, f"add_audio_track did not return track_id: {track_reply}")
        summary["track_id"] = track_id
        summary["checks"].append("add_audio_track")

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
        clip_id = str(import_reply.get("clip_id") or "").strip()
        require(clip_id, f"import_media_to_track did not return clip_id: {import_reply}")
        summary["clip_id"] = clip_id
        summary["checks"].append("import_media_to_track")

        initial_fade = require_ok("clip.fade.read initial", send_command(sock, {"cmd": "clip.fade.read", "clip_id": clip_id}))
        require("fade_in_seconds" in initial_fade and "fade_out_seconds" in initial_fade, f"initial fade fields missing: {initial_fade}")
        summary["checks"].append("clip.fade.read.initial")

        set_fade = require_ok(
            "clip.fade.set",
            send_command(
                sock,
                {
                    "cmd": "clip.fade.set",
                    "clip_id": clip_id,
                    "fade_in_seconds": fade_in,
                    "fade_out_seconds": fade_out,
                    "fade_in_curve": "linear",
                    "fade_out_curve": "convex",
                    "auto_crossfade": False,
                    "command_timeout_ms": command_timeout_ms,
                },
            ),
        )
        assert_fade("clip.fade.set reply", set_fade, fade_in, fade_out)
        summary["checks"].append("clip.fade.set")

        read_fade = require_ok("clip.fade.read after set", send_command(sock, {"cmd": "clip.fade.read", "clip_id": clip_id}))
        assert_fade("clip.fade.read after set", read_fade, fade_in, fade_out)
        summary["checks"].append("clip.fade.read.after_set")

        set_gain = require_ok(
            "clip.gain.set",
            send_command(
                sock,
                {
                    "cmd": "clip.gain.set",
                    "clip_id": clip_id,
                    "gain_db": gain_db,
                    "command_timeout_ms": command_timeout_ms,
                },
            ),
        )
        assert_gain("clip.gain.set reply", set_gain, gain_db)
        summary["checks"].append("clip.gain.set")

        read_gain = require_ok("clip.gain.read after set", send_command(sock, {"cmd": "clip.gain.read", "clip_id": clip_id}))
        assert_gain("clip.gain.read after set", read_gain, gain_db)
        summary["checks"].append("clip.gain.read.after_set")

        state_after_set = require_ok("get_project_state after set", send_command(sock, {"cmd": "get_project_state"}))
        _, clip_row = find_clip(state_after_set, clip_id)
        assert_fade("snapshot after set", clip_row, fade_in, fade_out)
        assert_gain("snapshot after set", clip_row, gain_db)
        summary["checks"].append("snapshot.after_set")

        require_ok("save_project", send_command(sock, {"cmd": "save_project", "command_timeout_ms": command_timeout_ms}))
        summary["checks"].append("save_project")

        require_ok(
            "open_project",
            send_command(sock, {"cmd": "open_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )
        summary["checks"].append("open_project")

        reopen_fade = require_ok("clip.fade.read after reopen", send_command(sock, {"cmd": "clip.fade.read", "clip_id": clip_id}))
        assert_fade("clip.fade.read after reopen", reopen_fade, fade_in, fade_out)
        summary["checks"].append("clip.fade.read.after_reopen")

        reopen_gain = require_ok("clip.gain.read after reopen", send_command(sock, {"cmd": "clip.gain.read", "clip_id": clip_id}))
        assert_gain("clip.gain.read after reopen", reopen_gain, gain_db)
        summary["checks"].append("clip.gain.read.after_reopen")

        session_id = vsp_hello(sock, command_timeout_ms)
        summary["vsp_session_id"] = session_id
        vsp_fade_in = 0.111
        vsp_fade_out = 0.222
        vsp_gain_db = -4.25
        vsp_set_fade = send_vsp_command(
            sock,
            session_id,
            "clip.fade.set",
            {
                "clip_id": clip_id,
                "fade_in_seconds": vsp_fade_in,
                "fade_out_seconds": vsp_fade_out,
                "fade_in_curve": "linear",
                "fade_out_curve": "linear",
            },
            command_timeout_ms,
        )
        assert_fade("VSP clip.fade.set reply", vsp_set_fade, vsp_fade_in, vsp_fade_out)
        summary["checks"].append("vsp.clip.fade.set")

        vsp_read_fade = send_vsp_command(sock, session_id, "clip.fade.read", {"clip_id": clip_id}, command_timeout_ms)
        assert_fade("VSP clip.fade.read reply", vsp_read_fade, vsp_fade_in, vsp_fade_out)
        summary["checks"].append("vsp.clip.fade.read")

        vsp_set_gain = send_vsp_command(sock, session_id, "clip.gain.set", {"clip_id": clip_id, "gain_db": vsp_gain_db}, command_timeout_ms)
        assert_gain("VSP clip.gain.set reply", vsp_set_gain, vsp_gain_db)
        summary["checks"].append("vsp.clip.gain.set")

        vsp_read_gain = send_vsp_command(sock, session_id, "clip.gain.read", {"clip_id": clip_id}, command_timeout_ms)
        assert_gain("VSP clip.gain.read reply", vsp_read_gain, vsp_gain_db)
        summary["checks"].append("vsp.clip.gain.read")

        require_ok("save_project after VSP commands", send_command(sock, {"cmd": "save_project", "command_timeout_ms": command_timeout_ms}))
        require_ok(
            "open_project after VSP commands",
            send_command(sock, {"cmd": "open_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )
        state_after_vsp_reopen = require_ok("get_project_state after VSP reopen", send_command(sock, {"cmd": "get_project_state"}))
        _, vsp_reopened_clip_row = find_clip(state_after_vsp_reopen, clip_id)
        assert_fade("snapshot after VSP reopen", vsp_reopened_clip_row, vsp_fade_in, vsp_fade_out)
        assert_gain("snapshot after VSP reopen", vsp_reopened_clip_row, vsp_gain_db)
        summary["checks"].append("snapshot.after_vsp_reopen")

        state_after_reopen = require_ok("get_project_state after reopen", send_command(sock, {"cmd": "get_project_state"}))
        _, reopened_clip_row = find_clip(state_after_reopen, clip_id)
        assert_fade("snapshot after reopen", reopened_clip_row, vsp_fade_in, vsp_fade_out)
        assert_gain("snapshot after reopen", reopened_clip_row, vsp_gain_db)
        summary["checks"].append("snapshot.after_reopen")

        summary["status"] = "passed"
        summary["expected"] = {
            "legacy_fade_in_seconds": fade_in,
            "legacy_fade_out_seconds": fade_out,
            "legacy_gain_db": gain_db,
            "vsp_fade_in_seconds": vsp_fade_in,
            "vsp_fade_out_seconds": vsp_fade_out,
            "vsp_gain_db": vsp_gain_db,
        }
        summary["snapshot_after_reopen"] = {
            key: reopened_clip_row.get(key)
            for key in (
                "fade_in_seconds",
                "fade_out_seconds",
                "fade_in_curve",
                "fade_out_curve",
                "auto_crossfade",
                "gain_db",
                "clip_gain_db",
                "pan",
                "mute",
            )
        }
    finally:
        sock.close(0)

    if args.output:
        output_path = Path(args.output).resolve()
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001 - smoke script top-level diagnostic.
        print(f"clip_fade_gain_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
