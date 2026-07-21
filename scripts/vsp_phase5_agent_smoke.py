#!/usr/bin/env python3
"""Smoke-test the Phase 5 Agent-facing VSP observe/execute/reobserve contract."""

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


CLIENT_ID = "smoke.vsp.phase5.agent"


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
        "role": "agent",
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
            payload={"command": command, "args": args or {}},
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
            payload={"command": "legacy.command", "legacy": {"cmd": cmd, "args": args or {}}},
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
            payload={"scope": "project.timeline"},
            timeout_ms=timeout_ms,
        ),
    )


def event_poll(socket: Any, *, session_id: str, timeout_ms: int) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="event",
            message_type="event.poll",
            schema="vsp.event.poll.v1",
            session_id=session_id,
            request_id=new_id("req"),
            payload={"topics": ["job.progress", "import.progress"], "limit": 4},
            timeout_ms=timeout_ms,
        ),
    )


def make_temp_wav() -> str:
    fd, path = tempfile.mkstemp(prefix="vsp_phase5_agent_", suffix=".wav")
    os.close(fd)
    sample_rate = 44100
    with wave.open(path, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(b"\x00\x00" * sample_rate)
    return path


def legacy_reply(reply: dict[str, Any]) -> dict[str, Any]:
    payload = reply.get("payload") or {}
    legacy = payload.get("legacy_reply") or {}
    require(isinstance(legacy, dict), f"legacy_reply missing object: {reply!r}")
    return legacy


def has_track_op(reply: dict[str, Any], op: str, track_ids: set[str]) -> bool:
    for item in ((reply.get("payload") or {}).get("ops") or []):
        if item.get("op") == op and str(item.get("track_id") or "") in track_ids:
            return True
    return False


def has_clip_op(reply: dict[str, Any], op: str, clip_ids: set[str]) -> bool:
    for item in ((reply.get("payload") or {}).get("ops") or []):
        if item.get("op") == op and str(item.get("clip_id") or "") in clip_ids:
            return True
    return False


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

    temp_wav_path: str | None = None
    created_track_ids: set[str] = set()
    created_clip_ids: set[str] = set()
    removed_clips = False
    deleted_tracks: set[str] = set()
    session_id = "session_pending"
    summary: dict[str, Any] = {
        "schema_version": "vsp_phase5_agent_smoke.v1",
        "url": args.url,
        "checks": [],
    }

    try:
        legacy_state = send(socket, {"cmd": "get_project_state"})
        require(legacy_state.get("status") == "ok", f"legacy get_project_state failed: {legacy_state!r}")
        summary["checks"].append("legacy_get_project_state_compatible_before")

        hello = send(
            socket,
            envelope(
                channel="session",
                message_type="session.hello",
                schema="vsp.session.hello.v1",
                payload={
                    "client_name": "VSP Phase 5 Agent smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["command.request", "state.snapshot", "state.delta", "state.resync", "event.progress"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: bad type {hello!r}")
        flags = hello.get("feature_flags") or {}
        for flag in ("state.snapshot.scoped", "state.delta", "state.resync", "event.job_progress", "legacy.ipc_adapter"):
            require(flags.get(flag) is True, f"hello: feature flag {flag} missing: {hello!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, f"hello: session_id missing {hello!r}")
        summary["session_id"] = session_id
        summary["checks"].append("session_hello_agent_capabilities")

        snapshot = state_request(
            socket,
            session_id=session_id,
            message_type="state.snapshot_request",
            schema="vsp.state.snapshot_request.v1",
            timeout_ms=args.timeout_ms,
        )
        require(snapshot.get("type") == "state.snapshot", f"snapshot: bad type {snapshot!r}")
        payload = snapshot.get("payload") or {}
        require(payload.get("snapshot_hash"), f"snapshot: hash missing {snapshot!r}")
        require(isinstance(payload.get("project"), dict), f"snapshot: project compact package missing {snapshot!r}")
        require(isinstance(payload.get("tracks"), list), f"snapshot: tracks compact package missing {snapshot!r}")
        require("tiles" not in json.dumps(payload, ensure_ascii=False).lower(), "snapshot leaked GUI tile payload")
        base_revision = int(snapshot.get("revision") or 0)
        require(base_revision > 0, f"snapshot: revision missing {snapshot!r}")
        summary["base_revision"] = base_revision
        summary["checks"].append("project_observe_compact_snapshot")

        create_track = command_request(
            socket,
            session_id=session_id,
            command="track.create",
            timeout_ms=args.timeout_ms,
        )
        require(create_track.get("type") == "command.response", f"track.create: bad type {create_track!r}")
        create_payload = create_track.get("payload") or {}
        require((create_track.get("ack") or {}).get("stage") == "completed", f"track.create: ack not completed {create_track!r}")
        require(create_track.get("transaction_id"), f"track.create: transaction_id missing {create_track!r}")
        create_reply = legacy_reply(create_track)
        require(create_reply.get("status") == "ok", f"track.create: legacy status not ok {create_track!r}")
        created_track_id = str(create_reply.get("track_id") or "").strip()
        require(created_track_id, f"track.create: track_id missing {create_track!r}")
        created_track_ids.add(created_track_id)
        require(create_payload.get("resync_hint") is True, f"track.create: legacy revision resync hint missing {create_track!r}")
        summary["created_track_ids"] = sorted(created_track_ids)
        summary["checks"].append("command_execute_ack_resync_hint")

        add_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=base_revision,
            timeout_ms=args.timeout_ms,
        )
        require(add_delta.get("type") == "state.delta", f"track add delta: bad type {add_delta!r}")
        require(has_track_op(add_delta, "add", created_track_ids), f"track add delta: op missing {add_delta!r}")
        revision_after_add = int(add_delta.get("revision") or 0)
        require(revision_after_add > base_revision, f"track add delta: revision did not advance {add_delta!r}")
        summary["revision_after_add"] = revision_after_add
        summary["checks"].append("command_reobserve_delta_track_add")

        temp_wav_path = make_temp_wav()
        import_audio = command_request(
            socket,
            session_id=session_id,
            command="project.import_audio_files",
            args={
                "file_paths": [temp_wav_path],
                "start_time_seconds": 0.0,
                "target_policy": "create_tracks",
                "defer_audio_analysis": True,
                "start_audio_analysis": False,
                "skip_unreadable": False,
                "confirmation": True,
                "confirmed": True,
            },
            timeout_ms=args.timeout_ms,
        )
        require(import_audio.get("type") == "command.response", f"project.import_audio_files: bad type {import_audio!r}")
        import_reply = legacy_reply(import_audio)
        require(import_reply.get("status") == "ok", f"project.import_audio_files: legacy status not ok {import_audio!r}")
        for value in import_reply.get("created_track_ids") or []:
            if str(value).strip():
                created_track_ids.add(str(value).strip())
        for value in import_reply.get("created_clip_ids") or []:
            if str(value).strip():
                created_clip_ids.add(str(value).strip())
        require(created_track_ids, f"project.import_audio_files: no created tracks seen {import_audio!r}")
        require(created_clip_ids, f"project.import_audio_files: no created clips seen {import_audio!r}")
        summary["created_track_ids"] = sorted(created_track_ids)
        summary["created_clip_ids"] = sorted(created_clip_ids)
        summary["checks"].append("import_command_ack_readback")

        import_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=revision_after_add,
            timeout_ms=args.timeout_ms,
        )
        require(import_delta.get("type") == "state.delta", f"import delta: bad type {import_delta!r}")
        require(
            has_track_op(import_delta, "add", created_track_ids) or has_clip_op(import_delta, "add", created_clip_ids),
            f"import delta: expected track or clip add op {import_delta!r}",
        )
        revision_after_import = int(import_delta.get("revision") or 0)
        require(revision_after_import > revision_after_add, f"import delta: revision did not advance {import_delta!r}")
        summary["revision_after_import"] = revision_after_import
        summary["checks"].append("import_reobserve_delta")

        progress = event_poll(socket, session_id=session_id, timeout_ms=args.timeout_ms)
        require(progress.get("type") in {"event.progress", "event.notification"}, f"event.poll: bad type {progress!r}")
        progress_payload = progress.get("payload") or {}
        require(progress_payload.get("event_id"), f"event.poll: event_id missing {progress!r}")
        require(int(progress_payload.get("sequence") or 0) > 0, f"event.poll: sequence missing {progress!r}")
        summary["checks"].append("import_progress_event_observe_shape")

        remove_clips = legacy_command_request(
            socket,
            session_id=session_id,
            cmd="remove_clips",
            args={"clip_ids": sorted(created_clip_ids)},
            timeout_ms=args.timeout_ms,
        )
        require(remove_clips.get("type") == "command.response", f"remove_clips cleanup: bad type {remove_clips!r}")
        require(legacy_reply(remove_clips).get("status") == "ok", f"remove_clips cleanup failed {remove_clips!r}")
        removed_clips = True
        summary["checks"].append("legacy_remove_imported_clips_cleanup")

        cleanup_base_revision = revision_after_import
        cleanup_delta = state_request(
            socket,
            session_id=session_id,
            message_type="state.delta_request",
            schema="vsp.state.delta_request.v1",
            base_revision=cleanup_base_revision,
            timeout_ms=args.timeout_ms,
        )
        if cleanup_delta.get("type") == "state.delta":
            cleanup_base_revision = int(cleanup_delta.get("revision") or cleanup_base_revision)

        for track_id in sorted(created_track_ids):
            delete_track = command_request(
                socket,
                session_id=session_id,
                command="track.delete",
                args={"track_id": track_id},
                timeout_ms=args.timeout_ms,
            )
            require(delete_track.get("type") == "command.response", f"track.delete cleanup: bad type {delete_track!r}")
            require(legacy_reply(delete_track).get("status") == "ok", f"track.delete cleanup failed {delete_track!r}")
            deleted_tracks.add(track_id)
        summary["checks"].append("command_track_delete_cleanup")

        final_resync = state_request(
            socket,
            session_id=session_id,
            message_type="state.resync_request",
            schema="vsp.state.resync_request.v1",
            timeout_ms=args.timeout_ms,
        )
        require(final_resync.get("type") == "state.snapshot", f"final resync: bad type {final_resync!r}")
        require((final_resync.get("payload") or {}).get("resync") is True, f"final resync: marker missing {final_resync!r}")
        summary["checks"].append("state_resync_snapshot_after_cleanup")

        legacy_state_after = send(socket, {"cmd": "get_project_state"})
        require(legacy_state_after.get("status") == "ok", f"legacy get_project_state after failed: {legacy_state_after!r}")
        summary["checks"].append("legacy_get_project_state_compatible_after")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if created_clip_ids and not removed_clips:
            try:
                legacy_command_request(
                    socket,
                    session_id=session_id,
                    cmd="remove_clips",
                    args={"clip_ids": sorted(created_clip_ids)},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for clips {sorted(created_clip_ids)}: {cleanup_error}", file=sys.stderr)
        for track_id in sorted(created_track_ids - deleted_tracks):
            try:
                command_request(
                    socket,
                    session_id=session_id,
                    command="track.delete",
                    args={"track_id": track_id},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for track {track_id}: {cleanup_error}", file=sys.stderr)
        if temp_wav_path:
            try:
                os.remove(temp_wav_path)
            except OSError as cleanup_error:
                print(f"temp wav cleanup failed for {temp_wav_path}: {cleanup_error}", file=sys.stderr)
        socket.close(linger=0)


if __name__ == "__main__":
    main()
