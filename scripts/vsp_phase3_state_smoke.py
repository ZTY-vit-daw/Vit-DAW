#!/usr/bin/env python3
"""Smoke-test the VSP Phase 3 state channel over the existing ZMQ endpoint."""

from __future__ import annotations

import argparse
import json
import os
import sys
import tempfile
import uuid
import wave
from datetime import datetime, timezone
from typing import Any

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


CLIENT_ID = "smoke.vsp.phase3.state"
KERNEL_INTERNAL_TIMELINE_TRACK_IDS = {"1002", "1003", "1004", "1005", "1006"}


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def envelope(
    *,
    channel: str,
    message_type: str,
    schema: str,
    payload: dict[str, Any],
    session_id: str = "session_pending",
    request_id: str | None = None,
    transaction_id: str | None = None,
    base_revision: int | None = None,
    timeout_ms: int = 60000,
) -> dict[str, Any]:
    msg: dict[str, Any] = {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": new_id("msg"),
        "session_id": session_id,
        "client_id": CLIENT_ID,
        "role": "test",
        "channel": channel,
        "type": message_type,
        "created_at": utc_now(),
        "trace_id": new_id("trace"),
        "payload": payload,
        "command_timeout_ms": timeout_ms,
    }
    if request_id:
        msg["request_id"] = request_id
    if transaction_id:
        msg["transaction_id"] = transaction_id
    if base_revision is not None:
        msg["base_revision"] = base_revision
    return msg


def send(socket: Any, payload: dict[str, Any]) -> dict[str, Any]:
    socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = socket.recv_string()
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"reply was not JSON: {raw[:500]}") from exc
    if not isinstance(parsed, dict):
        raise AssertionError(f"reply was not a JSON object: {type(parsed)!r}")
    return parsed


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def track_ids(rows: Any) -> list[str]:
    out: list[str] = []
    for row in rows or []:
        if isinstance(row, dict):
            track_id = str(row.get("track_id") or row.get("id") or "").strip()
            if track_id:
                out.append(track_id)
    return out


def require_old_ok(reply: dict[str, Any], label: str) -> None:
    require(reply.get("status") == "ok", f"{label}: expected legacy status ok, got {reply!r}")


def command_request(
    socket: Any,
    *,
    session_id: str,
    command: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="command",
            message_type="command.request",
            schema="vsp.command.request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            transaction_id=new_id("tx"),
            payload={
                "command": command,
                "args": args or {},
            },
            timeout_ms=timeout_ms,
        ),
    )


def legacy_command_request(
    socket: Any,
    *,
    session_id: str,
    cmd: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="command",
            message_type="command.request",
            schema="vsp.command.request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            transaction_id=new_id("tx"),
            payload={
                "command": "legacy.command",
                "legacy": {
                    "cmd": cmd,
                    "args": args or {},
                },
            },
            timeout_ms=timeout_ms,
        ),
    )


def state_request(
    socket: Any,
    *,
    session_id: str,
    message_type: str,
    schema: str,
    base_revision: int | None = None,
    timeout_ms: int,
) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="state",
            message_type=message_type,
            schema=schema,
            session_id=session_id,
            request_id=new_id("req"),
            base_revision=base_revision,
            payload={
                "scope": "project.timeline",
            },
            timeout_ms=timeout_ms,
        ),
    )


def find_track_op(reply: dict[str, Any], op: str, track_id: str) -> bool:
    for item in ((reply.get("payload") or {}).get("ops") or []):
        if item.get("op") == op and item.get("track_id") == track_id:
            return True
    return False


def find_clip_op(reply: dict[str, Any], op: str, clip_id: str) -> bool:
    for item in ((reply.get("payload") or {}).get("ops") or []):
        if item.get("op") == op and item.get("clip_id") == clip_id:
            return True
    return False


def make_temp_wav() -> str:
    fd, path = tempfile.mkstemp(prefix="vsp_phase3_state_", suffix=".wav")
    os.close(fd)
    sample_rate = 44100
    with wave.open(path, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(b"\x00\x00" * sample_rate)
    return path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=120000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    socket = ctx.socket(zmq.REQ)
    socket.setsockopt(zmq.RCVTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.SNDTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect(args.url)

    created_track_id: str | None = None
    imported_clip_id: str | None = None
    temp_wav_path: str | None = None
    deleted_created_track = False
    removed_imported_clip = False
    summary: dict[str, Any] = {
        "schema_version": "vsp_phase3_state_smoke.v1",
        "url": args.url,
        "checks": [],
    }

    try:
        legacy_state = send(socket, {"cmd": "get_project_state"})
        require_old_ok(legacy_state, "legacy get_project_state")
        require(isinstance(legacy_state.get("tracks"), list), "legacy get_project_state: tracks must be an array")
        summary["checks"].append("legacy_get_project_state_compatible")

        hello = send(
            socket,
            envelope(
                channel="session",
                message_type="session.hello",
                schema="vsp.session.hello.v1",
                payload={
                    "client_name": "VSP Phase 3 state smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["project.read", "project.write", "state.subscribe"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: expected session.hello_ack, got {hello!r}")
        flags = hello.get("feature_flags") or {}
        require(flags.get("state.snapshot.scoped") is True, f"hello: state.snapshot.scoped flag missing {flags!r}")
        require(flags.get("state.delta") is True, f"hello: state.delta flag missing {flags!r}")
        require(flags.get("state.resync") is True, f"hello: state.resync flag missing {flags!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, "hello: session_id missing")
        summary["session_id"] = session_id
        summary["checks"].append("vsp_session_hello_state_flags")

        snapshot = state_request(
            socket,
            session_id=session_id,
            message_type="state.snapshot_request",
            schema="vsp.state.snapshot_request.v1",
            timeout_ms=args.timeout_ms,
        )
        require(snapshot.get("type") == "state.snapshot", f"snapshot: bad type {snapshot!r}")
        revision = int(snapshot.get("revision") or 0)
        require(revision > 0, f"snapshot: revision missing {snapshot!r}")
        snap_payload = snapshot.get("payload") or {}
        require(isinstance(snap_payload.get("tracks"), list), "snapshot: payload.tracks must be an array")
        visible_track_ids = track_ids(snap_payload.get("tracks"))
        leaked_internal_ids = sorted(KERNEL_INTERNAL_TIMELINE_TRACK_IDS.intersection(visible_track_ids))
        require(not leaked_internal_ids, f"snapshot: project.timeline leaked internal track ids {leaked_internal_ids}")
        for track in snap_payload.get("tracks") or []:
            require("track_id" in track, f"snapshot: track_id missing in {track!r}")
            require(isinstance(track.get("clips"), list), f"snapshot: clips must be an array in {track!r}")
        summary["initial_track_ids"] = visible_track_ids
        summary["initial_revision"] = revision
        summary["checks"].append("state_snapshot_project_track_clip_shape")
        summary["checks"].append("state_snapshot_project_timeline_hides_internal_tracks")

        no_op_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision,
            timeout_ms=args.timeout_ms,
        )
        require(no_op_delta.get("type") == "state.delta", f"no-op delta: bad type {no_op_delta!r}")
        require((no_op_delta.get("ack") or {}).get("stage") == "no_op", f"no-op delta: bad ack {no_op_delta!r}")
        require((no_op_delta.get("payload") or {}).get("ops") == [], f"no-op delta: ops not empty {no_op_delta!r}")
        summary["checks"].append("state_delta_no_op")

        bad_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=max(0, revision - 1),
            timeout_ms=args.timeout_ms,
        )
        require(bad_delta.get("type") == "state.resync_required", f"bad delta: expected resync_required {bad_delta!r}")
        require((bad_delta.get("payload") or {}).get("resync_hint") is True, f"bad delta: resync hint missing {bad_delta!r}")
        summary["checks"].append("state_revision_gap_resync_required")

        create_track = command_request(
            socket,
            session_id=session_id,
            command="track.create",
            timeout_ms=args.timeout_ms,
        )
        require(create_track.get("type") == "command.response", f"track.create: bad type {create_track!r}")
        created_track_id = str(((create_track.get("payload") or {}).get("legacy_reply") or {}).get("track_id") or "").strip()
        require(created_track_id, f"track.create: track_id missing {create_track!r}")
        summary["created_track_id"] = created_track_id
        summary["checks"].append("command_track_create")

        add_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision,
            timeout_ms=args.timeout_ms,
        )
        require(add_delta.get("type") == "state.delta", f"add delta: bad type {add_delta!r}")
        require(find_track_op(add_delta, "add", created_track_id), f"add delta: add op missing {add_delta!r}")
        revision_after_add = int(add_delta.get("revision") or 0)
        require(revision_after_add > revision, f"add delta: revision did not advance {add_delta!r}")
        summary["revision_after_add"] = revision_after_add
        summary["checks"].append("state_delta_track_add")

        temp_wav_path = make_temp_wav()
        import_clip = legacy_command_request(
            socket,
            session_id=session_id,
            cmd="import_audio",
            args={
                "track_id": created_track_id,
                "file_path": temp_wav_path,
                "offset_time": 0.0,
            },
            timeout_ms=args.timeout_ms,
        )
        require(import_clip.get("type") == "command.response", f"import_audio: bad type {import_clip!r}")
        import_reply = (import_clip.get("payload") or {}).get("legacy_reply") or {}
        require(import_reply.get("status") == "ok", f"import_audio: legacy reply not ok {import_clip!r}")
        imported_clip_id = str(import_reply.get("clip_id") or "").strip()
        require(imported_clip_id, f"import_audio: clip_id missing {import_clip!r}")
        summary["imported_clip_id"] = imported_clip_id
        summary["checks"].append("legacy_import_audio_clip")

        clip_add_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision_after_add,
            timeout_ms=args.timeout_ms,
        )
        require(clip_add_delta.get("type") == "state.delta", f"clip add delta: bad type {clip_add_delta!r}")
        require(find_clip_op(clip_add_delta, "add", imported_clip_id), f"clip add delta: add op missing {clip_add_delta!r}")
        revision_after_clip_add = int(clip_add_delta.get("revision") or 0)
        require(revision_after_clip_add > revision_after_add, f"clip add delta: revision did not advance {clip_add_delta!r}")
        summary["revision_after_clip_add"] = revision_after_clip_add
        summary["checks"].append("state_delta_clip_add")

        move_clip = command_request(
            socket,
            session_id=session_id,
            command="clip.move",
            args={
                "source_track_id": created_track_id,
                "target_track_id": created_track_id,
                "clip_id": imported_clip_id,
                "new_start_seconds": 0.25,
            },
            timeout_ms=args.timeout_ms,
        )
        require(move_clip.get("type") == "command.response", f"clip.move: bad type {move_clip!r}")
        require(((move_clip.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok",
                f"clip.move: legacy reply not ok {move_clip!r}")
        summary["checks"].append("command_clip_move")

        clip_move_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision_after_clip_add,
            timeout_ms=args.timeout_ms,
        )
        require(clip_move_delta.get("type") == "state.delta", f"clip move delta: bad type {clip_move_delta!r}")
        require(find_clip_op(clip_move_delta, "replace", imported_clip_id),
                f"clip move delta: replace op missing {clip_move_delta!r}")
        revision_after_clip_move = int(clip_move_delta.get("revision") or 0)
        require(revision_after_clip_move > revision_after_clip_add,
                f"clip move delta: revision did not advance {clip_move_delta!r}")
        summary["revision_after_clip_move"] = revision_after_clip_move
        summary["checks"].append("state_delta_clip_replace")

        remove_clip = legacy_command_request(
            socket,
            session_id=session_id,
            cmd="remove_clips",
            args={
                "clip_ids": [imported_clip_id],
            },
            timeout_ms=args.timeout_ms,
        )
        require(remove_clip.get("type") == "command.response", f"remove_clips: bad type {remove_clip!r}")
        require(((remove_clip.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok",
                f"remove_clips: legacy reply not ok {remove_clip!r}")
        removed_imported_clip = True
        summary["checks"].append("legacy_remove_clip_cleanup")

        clip_remove_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision_after_clip_move,
            timeout_ms=args.timeout_ms,
        )
        require(clip_remove_delta.get("type") == "state.delta", f"clip remove delta: bad type {clip_remove_delta!r}")
        require(find_clip_op(clip_remove_delta, "remove", imported_clip_id),
                f"clip remove delta: remove op missing {clip_remove_delta!r}")
        revision_after_clip_remove = int(clip_remove_delta.get("revision") or 0)
        require(revision_after_clip_remove > revision_after_clip_move,
                f"clip remove delta: revision did not advance {clip_remove_delta!r}")
        summary["revision_after_clip_remove"] = revision_after_clip_remove
        summary["checks"].append("state_delta_clip_remove")

        delete_track = command_request(
            socket,
            session_id=session_id,
            command="track.delete",
            args={"track_id": created_track_id},
            timeout_ms=args.timeout_ms,
        )
        require(delete_track.get("type") == "command.response", f"track.delete: bad type {delete_track!r}")
        require(((delete_track.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok",
                f"track.delete: legacy reply not ok {delete_track!r}")
        deleted_created_track = True
        summary["checks"].append("command_track_delete_cleanup")

        remove_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision_after_clip_remove,
            timeout_ms=args.timeout_ms,
        )
        require(remove_delta.get("type") == "state.delta", f"remove delta: bad type {remove_delta!r}")
        require(find_track_op(remove_delta, "remove", created_track_id), f"remove delta: remove op missing {remove_delta!r}")
        summary["revision_after_remove"] = int(remove_delta.get("revision") or 0)
        summary["checks"].append("state_delta_track_remove")

        resync = state_request(
            socket,
            session_id=session_id,
            message_type="state.resync_request",
            schema="vsp.state.resync_request.v1",
            timeout_ms=args.timeout_ms,
        )
        require(resync.get("type") == "state.snapshot", f"resync: bad type {resync!r}")
        require((resync.get("payload") or {}).get("resync") is True, f"resync: marker missing {resync!r}")
        summary["checks"].append("state_resync_request_snapshot")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if created_track_id and not deleted_created_track:
            try:
                if imported_clip_id and not removed_imported_clip:
                    legacy_command_request(
                        socket,
                        session_id=str(summary.get("session_id") or "session_pending"),
                        cmd="remove_clips",
                        args={"clip_ids": [imported_clip_id]},
                        timeout_ms=args.timeout_ms,
                    )
                command_request(
                    socket,
                    session_id=str(summary.get("session_id") or "session_pending"),
                    command="track.delete",
                    args={"track_id": created_track_id},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover - best-effort cleanup path
                print(f"cleanup failed for track {created_track_id}: {cleanup_error}", file=sys.stderr)
        if temp_wav_path and os.path.exists(temp_wav_path):
            try:
                os.remove(temp_wav_path)
            except OSError as cleanup_error:
                print(f"temp wav cleanup failed for {temp_wav_path}: {cleanup_error}", file=sys.stderr)
        socket.close(0)


if __name__ == "__main__":
    main()
