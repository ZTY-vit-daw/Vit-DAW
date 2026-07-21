#!/usr/bin/env python3
"""VSP Hub 60-track playback acceptance smoke.

This test exercises the long-lived Hub boundary under a realistic multitrack
shape: batch import through command, visible-priority waveform manifest through
asset, canonical transport play/stop commands, and realtime cache reads.
"""

from __future__ import annotations

import argparse
import contextlib
import json
import math
import os
import tempfile
import time
import uuid
import wave
from datetime import datetime, timezone
from typing import Any
from urllib import error, request


CLIENT_ID = "smoke.vsp.hub_60_track_playback"


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def envelope(
    *,
    channel: str,
    message_type: str,
    schema: str,
    payload: dict[str, Any],
    session_id: str = "session_pending",
    request_id: str | None = None,
    transaction_id: str | None = None,
    timeout_ms: int = 120000,
) -> dict[str, Any]:
    msg: dict[str, Any] = {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": new_id("msg"),
        "session_id": session_id,
        "client_id": CLIENT_ID,
        "role": "gui",
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


def http_get_json(url: str, timeout_sec: int) -> dict[str, Any]:
    with request.urlopen(url, timeout=timeout_sec) as resp:
        parsed = json.loads(resp.read().decode("utf-8"))
    require(isinstance(parsed, dict), f"GET {url}: reply was not a JSON object")
    return parsed


def post_vsp(url: str, payload: dict[str, Any], timeout_sec: int) -> dict[str, Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method="POST",
    )
    try:
        with request.urlopen(req, timeout=timeout_sec) as resp:
            raw = resp.read().decode("utf-8")
    except error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        raise AssertionError(f"POST {url}: HTTP {exc.code}: {raw}") from exc
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"POST {url}: reply was not JSON: {raw[:500]}") from exc
    require(isinstance(parsed, dict), f"POST {url}: reply was not a JSON object")
    return parsed


def command_request(
    hub_url: str,
    *,
    session_id: str,
    command: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
    timeout_sec: int,
) -> tuple[dict[str, Any], float]:
    start = time.perf_counter()
    reply = post_vsp(
        hub_url,
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
        timeout_sec,
    )
    return reply, (time.perf_counter() - start) * 1000.0


def legacy_command_request(
    hub_url: str,
    *,
    session_id: str,
    cmd: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
    timeout_sec: int,
) -> tuple[dict[str, Any], float]:
    start = time.perf_counter()
    reply = post_vsp(
        hub_url,
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
        timeout_sec,
    )
    return reply, (time.perf_counter() - start) * 1000.0


def state_snapshot(
    hub_url: str,
    *,
    session_id: str,
    timeout_ms: int,
    timeout_sec: int,
) -> dict[str, Any]:
    return post_vsp(
        hub_url,
        envelope(
            channel="state",
            message_type="state.snapshot_request",
            schema="vsp.state.snapshot_request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            payload={"scope": "project.timeline"},
            timeout_ms=timeout_ms,
        ),
        timeout_sec,
    )


def realtime_request(
    hub_url: str,
    *,
    session_id: str,
    message_type: str,
    schema: str,
    payload: dict[str, Any],
    timeout_ms: int,
    timeout_sec: int,
) -> dict[str, Any]:
    return post_vsp(
        hub_url,
        envelope(
            channel="realtime",
            message_type=message_type,
            schema=schema,
            session_id=session_id,
            request_id=new_id("req"),
            payload=payload,
            timeout_ms=timeout_ms,
        ),
        timeout_sec,
    )


def asset_manifest_request(
    hub_url: str,
    *,
    session_id: str,
    track_ids: list[str],
    clip_ids: list[str],
    timeout_ms: int,
    timeout_sec: int,
) -> tuple[dict[str, Any], float]:
    start = time.perf_counter()
    reply = post_vsp(
        hub_url,
        envelope(
            channel="asset",
            message_type="asset.manifest_request",
            schema="vsp.asset.manifest_request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            payload={
                "kind": "waveform_peak",
                "track_ids": track_ids,
                "clip_ids": clip_ids,
                "priority": "visible_manifest",
                "max_assets": max(128, len(track_ids)),
                "visible_range": {"start_track": 0, "track_count": len(track_ids)},
            },
            timeout_ms=timeout_ms,
        ),
        timeout_sec,
    )
    return reply, (time.perf_counter() - start) * 1000.0


def make_temp_wavs(directory: str, count: int, duration_sec: float, sample_rate: int = 44100) -> list[str]:
    paths: list[str] = []
    total_samples = max(1, int(duration_sec * sample_rate))
    for i in range(count):
        path = os.path.join(directory, f"vsp_hub_60_track_{i:03d}.wav")
        freq = 110.0 + float(i % 24) * 17.5
        amp = 0.18
        with wave.open(path, "wb") as wav:
            wav.setnchannels(1)
            wav.setsampwidth(2)
            wav.setframerate(sample_rate)
            frames = bytearray()
            for n in range(total_samples):
                value = int(max(-1.0, min(1.0, math.sin(2.0 * math.pi * freq * n / sample_rate) * amp)) * 32767.0)
                frames += int(value).to_bytes(2, byteorder="little", signed=True)
            wav.writeframes(bytes(frames))
        paths.append(path)
    return paths


def legacy_reply(reply: dict[str, Any]) -> dict[str, Any]:
    payload = reply.get("payload") or {}
    lr = payload.get("legacy_reply") or {}
    require(isinstance(lr, dict), f"legacy reply missing {reply!r}")
    return lr


def assert_command_ok(reply: dict[str, Any], label: str) -> dict[str, Any]:
    require(reply.get("type") == "command.response", f"{label}: bad type {reply!r}")
    lr = legacy_reply(reply)
    require(str(lr.get("status") or "").lower() in {"ok", "success", ""}, f"{label}: legacy status not ok {reply!r}")
    return lr


def assert_manifest_ok(reply: dict[str, Any], expected_min_refs: int) -> dict[str, Any]:
    require(reply.get("type") == "asset.manifest", f"asset.manifest: bad type {reply!r}")
    payload = reply.get("payload") or {}
    refs = payload.get("references") or payload.get("assets") or []
    require(payload.get("status") == "ok", f"asset.manifest: status not ok {reply!r}")
    require(payload.get("kind") == "waveform_peak", f"asset.manifest: wrong kind {reply!r}")
    require(payload.get("inline_payload") is False, f"asset.manifest: inline payload present {reply!r}")
    require(payload.get("no_big_json_payload") is True, f"asset.manifest: big JSON guard missing {reply!r}")
    require(isinstance(refs, list) and len(refs) >= expected_min_refs, f"asset.manifest: too few refs {reply!r}")
    require(payload.get("prepare_requested") is True, f"asset.manifest: prepare_requested missing {reply!r}")
    require(int(payload.get("prepare_count") or 0) >= expected_min_refs, f"asset.manifest: prepare_count too small {reply!r}")
    require(int(payload.get("prepare_error_count") or 0) == 0, f"asset.manifest: prepare errors {reply!r}")
    forbidden = {"samples", "peaks", "pixels", "bins", "raw_bytes", "waveform_data", "spectrum_data"}
    serialized = json.dumps(payload, ensure_ascii=False)
    for key in forbidden:
        require(f'"{key}"' not in serialized, f"asset.manifest: forbidden inline key {key} present")
    return payload


def assert_realtime_frame(reply: dict[str, Any], stream_id: str, label: str) -> dict[str, Any]:
    require(reply.get("type") == "realtime.frame", f"{label}: bad type {reply!r}")
    payload = reply.get("payload") or {}
    require(payload.get("stream_id") == stream_id, f"{label}: stream_id mismatch {reply!r}")
    require(int(payload.get("frame_index") or 0) > 0, f"{label}: frame_index missing {reply!r}")
    data = payload.get("data") or {}
    require(data.get("source") != "reference_sample", f"{label}: still using reference sample {reply!r}")
    return data


def numeric_track_id(value: str) -> int | None:
    text = str(value).strip()
    return int(text) if text.isdigit() else None


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--hub-url", default="http://127.0.0.1:8787/vsp")
    parser.add_argument("--track-count", type=int, default=60)
    parser.add_argument("--visible-count", type=int, default=16)
    parser.add_argument("--duration-sec", type=float, default=1.5)
    parser.add_argument("--timeout-ms", type=int, default=180000)
    parser.add_argument("--timeout-sec", type=int, default=180)
    parser.add_argument("--max-control-ms", type=float, default=3000.0)
    parser.add_argument("--keep-imported-tracks", action="store_true")
    parser.add_argument("--media-dir", default="")
    args = parser.parse_args()

    hub_url = args.hub_url.rstrip("/")
    base_url = hub_url.removesuffix("/vsp")
    summary: dict[str, Any] = {
        "schema_version": "vsp_hub_60_track_playback_smoke.v1",
        "hub_url": hub_url,
        "track_count": args.track_count,
        "visible_count": args.visible_count,
        "keep_imported_tracks": bool(args.keep_imported_tracks),
        "checks": [],
        "timings_ms": {},
    }

    session_id = ""
    created_track_ids: list[str] = []
    created_clip_ids: list[str] = []
    smoke_succeeded = False

    tmp_context: Any
    if args.keep_imported_tracks:
        media_dir = os.path.abspath(args.media_dir) if args.media_dir else tempfile.mkdtemp(prefix="vsp_hub_60_track_playback_")
        os.makedirs(media_dir, exist_ok=True)
        tmp_context = contextlib.nullcontext(media_dir)
    else:
        tmp_context = tempfile.TemporaryDirectory(prefix="vsp_hub_60_track_playback_")

    with tmp_context as tmp:
        summary["media_dir"] = tmp
        try:
            health = http_get_json(f"{base_url}/health", args.timeout_sec)
            require(health.get("status") == "ok", f"hub health not ok {health!r}")
            status = http_get_json(f"{base_url}/vsp/status", args.timeout_sec)
            require("vsp.hub.http" in (status.get("transports") or []), f"hub status missing http transport {status!r}")
            summary["checks"].append("hub_health_status_ok")

            hello = post_vsp(
                hub_url,
                envelope(
                    channel="session",
                    message_type="session.hello",
                    schema="vsp.session.hello.v1",
                    payload={
                        "client_id": CLIENT_ID,
                        "client_name": "VSP Hub 60-track playback smoke",
                        "role": "gui",
                        "wants": ["command.request", "realtime.subscribe", "asset.manifest"],
                        "transport_bindings": ["vsp.hub.http"],
                    },
                    timeout_ms=args.timeout_ms,
                ),
                args.timeout_sec,
            )
            require(hello.get("type") == "session.hello_ack", f"hello: bad type {hello!r}")
            flags = hello.get("feature_flags") or {}
            require(flags.get("realtime.visible_tracks") is True, f"hello: realtime.visible_tracks missing {hello!r}")
            require(flags.get("asset.manifest") is True, f"hello: asset.manifest missing {hello!r}")
            session_id = str(hello.get("session_id") or "").strip()
            require(session_id, f"hello: session_id missing {hello!r}")
            summary["session_id"] = session_id
            summary["checks"].append("session_hello_caps")

            wav_paths = make_temp_wavs(tmp, args.track_count, args.duration_sec)
            import_reply, import_ms = command_request(
                hub_url,
                session_id=session_id,
                command="project.import_audio_files",
                args={
                    "file_paths": wav_paths,
                    "start_time_seconds": 0.0,
                    "target_policy": "create_tracks",
                    "defer_audio_analysis": True,
                    "start_audio_analysis": False,
                    "skip_unreadable": False,
                    "confirmation": True,
                    "confirmed": True,
                },
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            lr = assert_command_ok(import_reply, "project.import_audio_files")
            created_track_ids = [str(v).strip() for v in (lr.get("created_track_ids") or []) if str(v).strip()]
            created_clip_ids = [str(v).strip() for v in (lr.get("created_clip_ids") or []) if str(v).strip()]
            require(len(created_track_ids) >= args.track_count, f"import: expected {args.track_count} tracks, got {created_track_ids!r}")
            require(len(created_clip_ids) >= args.track_count, f"import: expected {args.track_count} clips, got {created_clip_ids!r}")
            numeric_ids = [numeric_track_id(v) for v in created_track_ids]
            if numeric_ids and all(v is not None for v in numeric_ids):
                require(min(v for v in numeric_ids if v is not None) >= 1007, f"import: user track id below 1007 {created_track_ids!r}")
            summary["created_track_count"] = len(created_track_ids)
            summary["created_clip_count"] = len(created_clip_ids)
            summary["first_track_id"] = created_track_ids[0]
            summary["created_track_ids"] = created_track_ids
            summary["created_clip_ids"] = created_clip_ids
            summary["timings_ms"]["import_audio_files"] = round(import_ms, 2)
            summary["checks"].append("batch_import_60_tracks")

            snapshot = state_snapshot(
                hub_url,
                session_id=session_id,
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            require(snapshot.get("type") == "state.snapshot", f"state.snapshot: bad type {snapshot!r}")
            summary["checks"].append("state_snapshot_after_import")

            visible_count = min(max(args.visible_count, 1), len(created_track_ids), 17)
            visible_track_ids = created_track_ids[:visible_count]
            visible_clip_ids = created_clip_ids[:visible_count]

            manifest, manifest_ms = asset_manifest_request(
                hub_url,
                session_id=session_id,
                track_ids=visible_track_ids,
                clip_ids=visible_clip_ids,
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            manifest_payload = assert_manifest_ok(manifest, expected_min_refs=visible_count)
            summary["timings_ms"]["asset_manifest_visible"] = round(manifest_ms, 2)
            summary["manifest_prepare_count"] = int(manifest_payload.get("prepare_count") or 0)
            summary["manifest_reference_count"] = int(manifest_payload.get("asset_count") or 0)
            summary["checks"].append("visible_asset_manifest_prepares_waveforms")

            rt_sub = realtime_request(
                hub_url,
                session_id=session_id,
                message_type="realtime.subscribe",
                schema="vsp.realtime.subscribe.v1",
                payload={
                    "streams": [
                        {"stream": "transport.playhead", "max_hz": 30, "mode": "latest_only"},
                        {
                            "stream": "meters.visible_tracks",
                            "track_ids": visible_track_ids,
                            "max_hz": 60,
                            "mode": "latest_only",
                            "visible_range": {"start_track": 0, "track_count": visible_count},
                        },
                    ]
                },
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            require(rt_sub.get("type") == "realtime.stream_status", f"realtime.subscribe: bad type {rt_sub!r}")
            rt_payload = rt_sub.get("payload") or {}
            subscription_id = str(rt_payload.get("subscription_id") or "").strip()
            streams = rt_payload.get("streams") or []
            require(subscription_id, f"realtime.subscribe: subscription_id missing {rt_sub!r}")
            require(isinstance(streams, list) and len(streams) == 2, f"realtime.subscribe: expected two streams {rt_sub!r}")
            for stream in streams:
                require(stream.get("mode") == "latest_only", f"realtime stream not latest_only {stream!r}")
                require(stream.get("drop_old") is True, f"realtime stream missing drop_old {stream!r}")
                require(1 <= int(stream.get("max_hz") or 0) <= 60, f"realtime max_hz outside budget {stream!r}")
            transport_stream_id = str(streams[0].get("stream_id") or "").strip()
            meter_stream_id = str(streams[1].get("stream_id") or "").strip()
            require(transport_stream_id and meter_stream_id, f"stream_id missing {rt_sub!r}")
            summary["realtime_subscription_id"] = subscription_id
            summary["checks"].append("realtime_subscribe_visible_budget")

            play_reply, play_ms = command_request(
                hub_url,
                session_id=session_id,
                command="transport.play",
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            assert_command_ok(play_reply, "transport.play")
            require(play_ms <= args.max_control_ms, f"transport.play too slow: {play_ms:.2f}ms")
            summary["timings_ms"]["transport_play"] = round(play_ms, 2)
            summary["checks"].append("canonical_transport_play_responsive")

            time.sleep(1.0)
            transport_frame = realtime_request(
                hub_url,
                session_id=session_id,
                message_type="realtime.frame_request",
                schema="vsp.realtime.frame_request.v1",
                payload={"subscription_id": subscription_id, "stream_id": transport_stream_id},
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            transport_data = assert_realtime_frame(transport_frame, transport_stream_id, "transport.frame")
            require("position_seconds" in transport_data, f"transport.frame: position missing {transport_frame!r}")
            summary["realtime_transport_source"] = transport_data.get("source")

            meter_frame = realtime_request(
                hub_url,
                session_id=session_id,
                message_type="realtime.frame_request",
                schema="vsp.realtime.frame_request.v1",
                payload={"subscription_id": subscription_id, "stream_id": meter_stream_id},
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            meter_data = assert_realtime_frame(meter_frame, meter_stream_id, "meter.frame")
            meter_tracks = meter_data.get("tracks") or []
            require(isinstance(meter_tracks, list) and meter_tracks, f"meter.frame: tracks missing {meter_frame!r}")
            require(len(meter_tracks) <= 17, f"meter.frame: visible budget exceeded {len(meter_tracks)}")
            require(all(str(t.get("track_id") or "") in visible_track_ids for t in meter_tracks if isinstance(t, dict)), f"meter.frame: leaked non-visible tracks {meter_frame!r}")
            require(all("peak_db" in t and "left_level_db" in t for t in meter_tracks if isinstance(t, dict)), f"meter.frame: missing level fields {meter_frame!r}")
            summary["realtime_meter_track_count"] = len(meter_tracks)
            summary["checks"].append("realtime_cache_visible_meter_frame")

            stop_reply, stop_ms = command_request(
                hub_url,
                session_id=session_id,
                command="transport.stop",
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            assert_command_ok(stop_reply, "transport.stop")
            require(stop_ms <= args.max_control_ms, f"transport.stop too slow: {stop_ms:.2f}ms")
            summary["timings_ms"]["transport_stop"] = round(stop_ms, 2)
            summary["checks"].append("canonical_transport_stop_responsive")

            rt_unsub = realtime_request(
                hub_url,
                session_id=session_id,
                message_type="realtime.unsubscribe",
                schema="vsp.realtime.unsubscribe.v1",
                payload={"subscription_id": subscription_id},
                timeout_ms=args.timeout_ms,
                timeout_sec=args.timeout_sec,
            )
            require(rt_unsub.get("type") == "realtime.stream_status", f"realtime.unsubscribe: bad type {rt_unsub!r}")
            require((rt_unsub.get("payload") or {}).get("status") == "unsubscribed", f"realtime.unsubscribe: not unsubscribed {rt_unsub!r}")
            summary["checks"].append("realtime_unsubscribe")

            cleanup_mode = "defer_to_lifecycle" if args.keep_imported_tracks else "cleanup_in_finally"
            summary["cleanup"] = {
                "mode": cleanup_mode,
                "track_count": len(created_track_ids),
                "clip_count": len(created_clip_ids),
            }
            if args.keep_imported_tracks:
                summary["checks"].append("imported_tracks_retained_for_followup_probe")
            summary["status"] = "ok"
            smoke_succeeded = True
            print(json.dumps(summary, ensure_ascii=False, indent=2))
        finally:
            if session_id and not (args.keep_imported_tracks and smoke_succeeded):
                if created_clip_ids:
                    try:
                        legacy_command_request(
                            hub_url,
                            session_id=session_id,
                            cmd="remove_clips",
                            args={"clip_ids": created_clip_ids},
                            timeout_ms=args.timeout_ms,
                            timeout_sec=args.timeout_sec,
                        )
                    except Exception:
                        pass
                for track_id in created_track_ids:
                    try:
                        command_request(
                            hub_url,
                            session_id=session_id,
                            command="track.delete",
                            args={"track_id": track_id},
                            timeout_ms=args.timeout_ms,
                            timeout_sec=args.timeout_sec,
                        )
                    except Exception:
                        pass


if __name__ == "__main__":
    main()
