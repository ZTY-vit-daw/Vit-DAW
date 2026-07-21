#!/usr/bin/env python3
"""Smoke-test VSP Phase 4 realtime, asset, and event reference channels."""

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


CLIENT_ID = "smoke.vsp.phase4.realtime_asset"


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


def make_temp_wav() -> str:
    fd, path = tempfile.mkstemp(prefix="vsp_phase4_asset_", suffix=".wav")
    os.close(fd)
    sample_rate = 44100
    with wave.open(path, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(b"\x00\x00" * sample_rate)
    return path


def assert_stream_budget(stream: dict[str, Any], expected_track_id: str) -> None:
    require(stream.get("mode") == "latest_only", f"stream mode must be latest_only: {stream!r}")
    require(stream.get("drop_old") is True, f"stream drop_old missing: {stream!r}")
    require(1 <= int(stream.get("max_hz") or 0) <= 60, f"stream max_hz outside budget: {stream!r}")
    if stream.get("stream") in {"meters.visible_tracks", "spectrum.visible_tracks"}:
        require(stream.get("visible_tracks_only") is True, f"visible stream must be visible only: {stream!r}")
        require(stream.get("track_ids") == [expected_track_id], f"visible stream track_ids not limited: {stream!r}")
        budget = stream.get("track_budget") or {}
        require(budget.get("max_visible_tracks") == 17, f"visible budget missing: {stream!r}")


def assert_asset_reference(reply: dict[str, Any], *, kind: str, clip_id: str) -> None:
    require(reply.get("type") == "asset.reference", f"asset: bad type {reply!r}")
    payload = reply.get("payload") or {}
    asset = payload.get("asset") or {}
    bindings = payload.get("bindings") or []
    require(payload.get("inline_payload") is False, f"asset: inline payload must be false {reply!r}")
    require(payload.get("no_big_json_payload") is True, f"asset: big payload guard missing {reply!r}")
    require(asset.get("kind") == kind, f"asset: wrong kind {asset!r}")
    require(asset.get("owner") == clip_id, f"asset: wrong owner {asset!r}")
    require(str(asset.get("uri") or "").startswith("vit-cache://project_current/clips/"), f"asset: uri not neutral {asset!r}")
    require(asset.get("revision"), f"asset: revision missing {asset!r}")
    require(asset.get("asset_id"), f"asset: asset_id missing {asset!r}")
    require(asset.get("format"), f"asset: format missing {asset!r}")
    require(isinstance(bindings, list) and bindings, f"asset: bindings missing {reply!r}")
    forbidden = {"samples", "peaks", "pixels", "bins", "raw_bytes", "waveform_data", "spectrum_data"}
    serialized = json.dumps(payload, ensure_ascii=False)
    for key in forbidden:
        require(f'"{key}"' not in serialized, f"asset: forbidden inline key {key} present")


def assert_asset_manifest(reply: dict[str, Any], *, kind: str, track_id: str, clip_id: str) -> None:
    require(reply.get("type") == "asset.manifest", f"asset manifest: bad type {reply!r}")
    payload = reply.get("payload") or {}
    references = payload.get("references") or payload.get("assets") or []
    require(payload.get("status") == "ok", f"asset manifest: status not ok {reply!r}")
    require(payload.get("kind") == kind, f"asset manifest: wrong kind {reply!r}")
    require(payload.get("platform_neutral") is True, f"asset manifest: platform_neutral missing {reply!r}")
    require(payload.get("inline_payload") is False, f"asset manifest: inline payload must be false {reply!r}")
    require(payload.get("no_big_json_payload") is True, f"asset manifest: big payload guard missing {reply!r}")
    require(isinstance(references, list) and references, f"asset manifest: references missing {reply!r}")
    require(int(payload.get("asset_count") or 0) == len(references), f"asset manifest: asset_count mismatch {reply!r}")

    matching = [
        ref for ref in references
        if isinstance(ref, dict)
        and str(ref.get("clip_id") or "") == clip_id
        and str(ref.get("track_id") or ref.get("kernel_track_id") or "") == track_id
        and str(ref.get("kind") or "") == kind
    ]
    require(matching, f"asset manifest: imported clip reference missing {reply!r}")

    ref = matching[0]
    asset = ref.get("asset") or {}
    require(ref.get("source") == "vsp.asset.manifest", f"asset manifest: wrong source {ref!r}")
    require(ref.get("inline_payload") is False, f"asset manifest: ref inline payload must be false {ref!r}")
    require(ref.get("no_big_json_payload") is True, f"asset manifest: ref big payload guard missing {ref!r}")
    require(asset.get("owner") == clip_id, f"asset manifest: wrong asset owner {ref!r}")
    require(asset.get("kind") == kind, f"asset manifest: wrong asset kind {ref!r}")
    require(str(asset.get("uri") or "").startswith("vit-cache://project_current/clips/"), f"asset manifest: uri not neutral {ref!r}")
    require(asset.get("revision"), f"asset manifest: revision missing {ref!r}")
    require(asset.get("asset_id"), f"asset manifest: asset_id missing {ref!r}")

    forbidden = {"samples", "peaks", "pixels", "bins", "raw_bytes", "waveform_data", "spectrum_data"}
    serialized = json.dumps(payload, ensure_ascii=False)
    for key in forbidden:
        require(f'"{key}"' not in serialized, f"asset manifest: forbidden inline key {key} present")


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
    session_id = "session_pending"
    summary: dict[str, Any] = {
        "schema_version": "vsp_phase4_realtime_asset_smoke.v1",
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
                    "client_name": "VSP Phase 4 realtime/asset smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["realtime.subscribe", "asset.reference", "asset.manifest", "event.subscribe"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: bad type {hello!r}")
        flags = hello.get("feature_flags") or {}
        require(flags.get("realtime.visible_tracks") is True, f"hello: realtime.visible_tracks missing {flags!r}")
        require(flags.get("realtime.latest_only") is True, f"hello: realtime.latest_only missing {flags!r}")
        require(flags.get("asset.reference") is True, f"hello: asset.reference missing {flags!r}")
        require(flags.get("asset.manifest") is True, f"hello: asset.manifest missing {flags!r}")
        require(flags.get("event.job_progress") is True, f"hello: event.job_progress missing {flags!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, "hello: session_id missing")
        summary["session_id"] = session_id
        summary["checks"].append("vsp_session_hello_phase4_flags")

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
        summary["checks"].append("command_track_create_for_visible_scope")

        rt_sub = send(
            socket,
            envelope(
                channel="realtime",
                message_type="realtime.subscribe",
                schema="vsp.realtime.subscribe.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={
                    "streams": [
                        {"stream": "transport.playhead", "max_hz": 30, "mode": "latest_only"},
                        {
                            "stream": "meters.visible_tracks",
                            "track_ids": [created_track_id],
                            "max_hz": 20,
                            "mode": "latest_only",
                            "visible_range": {"start_track": 0, "track_count": 1},
                        },
                        {
                            "stream": "spectrum.visible_tracks",
                            "track_ids": [created_track_id],
                            "max_hz": 12,
                            "mode": "latest_only",
                            "visible_range": {"start_track": 0, "track_count": 1},
                        },
                    ]
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(rt_sub.get("type") == "realtime.stream_status", f"realtime.subscribe: bad type {rt_sub!r}")
        rt_payload = rt_sub.get("payload") or {}
        subscription_id = str(rt_payload.get("subscription_id") or "").strip()
        streams = rt_payload.get("streams") or []
        require(subscription_id, f"realtime.subscribe: subscription_id missing {rt_sub!r}")
        require(len(streams) == 3, f"realtime.subscribe: expected 3 streams {rt_sub!r}")
        for stream in streams:
            assert_stream_budget(stream, created_track_id)
        summary["realtime_subscription_id"] = subscription_id
        summary["checks"].append("realtime_subscribe_latest_only_visible_budget")

        stream_id = str(streams[0].get("stream_id") or "").strip()
        frame = send(
            socket,
            envelope(
                channel="realtime",
                message_type="realtime.frame_request",
                schema="vsp.realtime.frame_request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"subscription_id": subscription_id, "stream_id": stream_id},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(frame.get("type") == "realtime.frame", f"realtime.frame_request: bad type {frame!r}")
        frame_payload = frame.get("payload") or {}
        require(frame_payload.get("stream_id") == stream_id, f"realtime.frame: stream_id mismatch {frame!r}")
        require(int(frame_payload.get("frame_index") or 0) > 0, f"realtime.frame: frame_index missing {frame!r}")
        transport_data = frame_payload.get("data") or {}
        require(transport_data.get("source") != "reference_sample", f"realtime.transport: still using reference sample {frame!r}")
        require("position_seconds" in transport_data, f"realtime.transport: position missing {frame!r}")
        summary["realtime_transport_source"] = transport_data.get("source")
        summary["checks"].append("realtime_latest_frame_pull")

        meter_stream_id = str(streams[1].get("stream_id") or "").strip()
        meter_frame = send(
            socket,
            envelope(
                channel="realtime",
                message_type="realtime.frame_request",
                schema="vsp.realtime.frame_request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"subscription_id": subscription_id, "stream_id": meter_stream_id},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(meter_frame.get("type") == "realtime.frame", f"realtime.meter: bad type {meter_frame!r}")
        meter_payload = meter_frame.get("payload") or {}
        meter_data = meter_payload.get("data") or {}
        meter_tracks = meter_data.get("tracks") or []
        require(meter_data.get("source") != "reference_sample", f"realtime.meter: still using reference sample {meter_frame!r}")
        require(isinstance(meter_tracks, list) and len(meter_tracks) == 1, f"realtime.meter: expected one track {meter_frame!r}")
        meter_track = meter_tracks[0] or {}
        require(meter_track.get("track_id") == created_track_id, f"realtime.meter: wrong track id {meter_frame!r}")
        require(meter_track.get("source") == "engine_level_meter", f"realtime.meter: wrong source {meter_frame!r}")
        require("peak_db" in meter_track and "left_level_db" in meter_track, f"realtime.meter: level fields missing {meter_frame!r}")
        summary["realtime_meter_source"] = meter_track.get("source")
        summary["checks"].append("realtime_meter_engine_frame_pull")

        rt_unsub = send(
            socket,
            envelope(
                channel="realtime",
                message_type="realtime.unsubscribe",
                schema="vsp.realtime.unsubscribe.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"subscription_id": subscription_id},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(rt_unsub.get("type") == "realtime.stream_status", f"realtime.unsubscribe: bad type {rt_unsub!r}")
        require((rt_unsub.get("payload") or {}).get("status") == "unsubscribed", f"realtime.unsubscribe: not unsubscribed {rt_unsub!r}")
        summary["checks"].append("realtime_unsubscribe")

        temp_wav_path = make_temp_wav()
        import_clip = legacy_command_request(
            socket,
            session_id=session_id,
            cmd="import_audio",
            args={"track_id": created_track_id, "file_path": temp_wav_path, "offset_time": 0.0},
            timeout_ms=args.timeout_ms,
        )
        require(import_clip.get("type") == "command.response", f"import_audio: bad type {import_clip!r}")
        import_reply = (import_clip.get("payload") or {}).get("legacy_reply") or {}
        require(import_reply.get("status") == "ok", f"import_audio: legacy reply not ok {import_clip!r}")
        imported_clip_id = str(import_reply.get("clip_id") or "").strip()
        require(imported_clip_id, f"import_audio: clip_id missing {import_clip!r}")
        summary["imported_clip_id"] = imported_clip_id
        summary["checks"].append("legacy_import_audio_for_asset_reference")

        for kind in ("waveform_peak", "spectrum_tile"):
            asset = send(
                socket,
                envelope(
                    channel="asset",
                    message_type="asset.request",
                    schema="vsp.asset.request.v1",
                    session_id=session_id,
                    request_id=new_id("req"),
                    payload={"clip_id": imported_clip_id, "kind": kind},
                    timeout_ms=args.timeout_ms,
                ),
            )
            assert_asset_reference(asset, kind=kind, clip_id=imported_clip_id)
        summary["checks"].append("asset_references_waveform_and_spectrum_no_big_json")

        asset_manifest = send(
            socket,
            envelope(
                channel="asset",
                message_type="asset.manifest_request",
                schema="vsp.asset.manifest_request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={
                    "kind": "waveform_peak",
                    "track_ids": [created_track_id],
                    "clip_ids": [imported_clip_id],
                    "priority": "visible_manifest",
                    "max_assets": 128,
                    "visible_range": {"start_track": 0, "track_count": 1},
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        assert_asset_manifest(
            asset_manifest,
            kind="waveform_peak",
            track_id=created_track_id,
            clip_id=imported_clip_id,
        )
        summary["checks"].append("asset_manifest_visible_waveform_no_big_json")

        evt_sub = send(
            socket,
            envelope(
                channel="event",
                message_type="event.subscribe",
                schema="vsp.event.subscribe.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"topics": ["import.audio", "asset.bake", "render"]},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(evt_sub.get("type") == "event.notification", f"event.subscribe: bad type {evt_sub!r}")
        event_subscription_id = str((evt_sub.get("payload") or {}).get("subscription_id") or "").strip()
        require(event_subscription_id, f"event.subscribe: subscription_id missing {evt_sub!r}")
        summary["event_subscription_id"] = event_subscription_id
        summary["checks"].append("event_subscribe_topics")

        evt_poll = send(
            socket,
            envelope(
                channel="event",
                message_type="event.poll",
                schema="vsp.event.poll.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"subscription_id": event_subscription_id},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(evt_poll.get("type") in {"event.progress", "event.notification"}, f"event.poll: bad type {evt_poll!r}")
        evt_payload = evt_poll.get("payload") or {}
        require(evt_payload.get("event_id"), f"event.poll: event_id missing {evt_poll!r}")
        require(int(evt_payload.get("sequence") or 0) > 0, f"event.poll: sequence missing {evt_poll!r}")
        if evt_poll.get("type") == "event.progress":
            require(evt_payload.get("job_id"), f"event.progress: job_id missing {evt_poll!r}")
            require(0.0 <= float(evt_payload.get("progress") or 0.0) <= 1.0, f"event.progress: invalid progress {evt_poll!r}")
        else:
            require(evt_payload.get("status") == "no_op", f"event.notification: expected no_op {evt_poll!r}")
        summary["checks"].append("event_poll_progress_or_noop_shape")

        remove_clip = legacy_command_request(
            socket,
            session_id=session_id,
            cmd="remove_clips",
            args={"clip_ids": [imported_clip_id]},
            timeout_ms=args.timeout_ms,
        )
        require(remove_clip.get("type") == "command.response", f"remove_clips: bad type {remove_clip!r}")
        require(((remove_clip.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok", f"remove_clips: legacy reply not ok {remove_clip!r}")
        removed_imported_clip = True
        summary["checks"].append("legacy_remove_clip_cleanup")

        delete_track = command_request(
            socket,
            session_id=session_id,
            command="track.delete",
            args={"track_id": created_track_id},
            timeout_ms=args.timeout_ms,
        )
        require(delete_track.get("type") == "command.response", f"track.delete: bad type {delete_track!r}")
        require(((delete_track.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok", f"track.delete: legacy reply not ok {delete_track!r}")
        deleted_created_track = True
        summary["checks"].append("command_track_delete_cleanup")

        legacy_state_after = send(socket, {"cmd": "get_project_state"})
        require(legacy_state_after.get("status") == "ok", f"legacy get_project_state after failed: {legacy_state_after!r}")
        summary["checks"].append("legacy_get_project_state_compatible_after")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if created_track_id and not deleted_created_track:
            try:
                if imported_clip_id and not removed_imported_clip:
                    legacy_command_request(
                        socket,
                        session_id=session_id,
                        cmd="remove_clips",
                        args={"clip_ids": [imported_clip_id]},
                        timeout_ms=args.timeout_ms,
                    )
                command_request(
                    socket,
                    session_id=session_id,
                    command="track.delete",
                    args={"track_id": created_track_id},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for track {created_track_id}: {cleanup_error}", file=sys.stderr)
        if temp_wav_path and os.path.exists(temp_wav_path):
            try:
                os.remove(temp_wav_path)
            except OSError as cleanup_error:
                print(f"temp wav cleanup failed for {temp_wav_path}: {cleanup_error}", file=sys.stderr)
        socket.close(0)


if __name__ == "__main__":
    main()
