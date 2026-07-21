#!/usr/bin/env python3
"""Smoke-test asset reference and manifest routing through VspHub HTTP."""

from __future__ import annotations

import argparse
import json
import os
import tempfile
import uuid
import wave
from datetime import datetime, timezone
from typing import Any
from urllib import error, request


CLIENT_ID = "smoke.vsp.phase4.hub_http_asset"


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
        payload = resp.read().decode("utf-8")
    parsed = json.loads(payload)
    if not isinstance(parsed, dict):
        raise AssertionError(f"GET {url}: reply was not a JSON object")
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
    if not isinstance(parsed, dict):
        raise AssertionError(f"POST {url}: reply was not a JSON object: {type(parsed)!r}")
    return parsed


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def command_request(
    hub_url: str,
    *,
    session_id: str,
    command: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
    timeout_sec: int,
) -> dict[str, Any]:
    return post_vsp(
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


def legacy_command_request(
    hub_url: str,
    *,
    session_id: str,
    cmd: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
    timeout_sec: int,
) -> dict[str, Any]:
    return post_vsp(
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


def make_temp_wav() -> str:
    fd, path = tempfile.mkstemp(prefix="vsp_phase4_hub_asset_", suffix=".wav")
    os.close(fd)
    sample_rate = 44100
    with wave.open(path, "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        wav.writeframes(b"\x00\x00" * sample_rate)
    return path


def assert_asset_reference(reply: dict[str, Any], *, kind: str, clip_id: str) -> None:
    require(reply.get("type") == "asset.reference", f"asset reference: bad type {reply!r}")
    payload = reply.get("payload") or {}
    asset = payload.get("asset") or {}
    require(payload.get("inline_payload") is False, f"asset reference: inline payload must be false {reply!r}")
    require(payload.get("no_big_json_payload") is True, f"asset reference: big payload guard missing {reply!r}")
    require(asset.get("kind") == kind, f"asset reference: wrong kind {reply!r}")
    require(asset.get("owner") == clip_id, f"asset reference: wrong owner {reply!r}")
    require(str(asset.get("uri") or "").startswith("vit-cache://project_current/clips/"), f"asset reference: uri not neutral {reply!r}")
    assert_no_big_inline_payload(payload, "asset reference")


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
    require(asset.get("owner") == clip_id, f"asset manifest: wrong asset owner {ref!r}")
    require(asset.get("kind") == kind, f"asset manifest: wrong asset kind {ref!r}")
    require(str(asset.get("uri") or "").startswith("vit-cache://project_current/clips/"), f"asset manifest: uri not neutral {ref!r}")
    assert_no_big_inline_payload(payload, "asset manifest")


def assert_no_big_inline_payload(payload: dict[str, Any], label: str) -> None:
    forbidden = {"samples", "peaks", "pixels", "bins", "raw_bytes", "waveform_data", "spectrum_data"}
    serialized = json.dumps(payload, ensure_ascii=False)
    for key in forbidden:
        require(f'"{key}"' not in serialized, f"{label}: forbidden inline key {key} present")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--hub-url", default="http://127.0.0.1:8787/vsp")
    parser.add_argument("--timeout-ms", type=int, default=120000)
    parser.add_argument("--timeout-sec", type=int, default=120)
    args = parser.parse_args()

    hub_url = args.hub_url.rstrip("/")
    status_url = hub_url.removesuffix("/vsp") + "/vsp/status"
    health_url = hub_url.removesuffix("/vsp") + "/health"

    created_track_id: str | None = None
    imported_clip_id: str | None = None
    temp_wav_path: str | None = None
    deleted_created_track = False
    removed_imported_clip = False
    session_id = "session_pending"
    summary: dict[str, Any] = {
        "schema_version": "vsp_phase4_hub_http_asset_smoke.v1",
        "hub_url": hub_url,
        "checks": [],
    }

    try:
        health = http_get_json(health_url, args.timeout_sec)
        require(health.get("status") == "ok" and health.get("service") == "VspHub", f"health not ok {health!r}")
        summary["checks"].append("hub_health_ok")

        status = http_get_json(status_url, args.timeout_sec)
        gui_caps = (status.get("capabilities") or {}).get("gui") or []
        ext_caps = (status.get("capabilities") or {}).get("extension") or []
        require("asset.manifest_request" in gui_caps, f"gui caps missing asset.manifest_request {gui_caps!r}")
        require("asset.manifest_request" in ext_caps, f"extension caps missing asset.manifest_request {ext_caps!r}")
        require("command.request" not in ext_caps, f"extension caps unexpectedly allow command.request {ext_caps!r}")
        summary["checks"].append("hub_status_advertises_asset_manifest_caps")

        hello = post_vsp(
            hub_url,
            envelope(
                channel="session",
                message_type="session.hello",
                schema="vsp.session.hello.v1",
                payload={
                    "client_name": "VSP Phase 4 Hub HTTP Asset Smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["command.request", "asset.reference", "asset.manifest"],
                    "transport_bindings": ["vsp.hub.http"],
                },
                timeout_ms=args.timeout_ms,
            ),
            args.timeout_sec,
        )
        require(hello.get("type") == "session.hello_ack", f"hello: bad type {hello!r}")
        flags = hello.get("feature_flags") or {}
        hub = hello.get("hub") or {}
        require(flags.get("asset.reference") is True, f"hello: asset.reference missing {flags!r}")
        require(flags.get("asset.manifest") is True, f"hello: asset.manifest missing {flags!r}")
        require("asset.manifest_request" in (hub.get("capabilities") or []), f"hello: hub caps missing asset.manifest_request {hello!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, "hello: session_id missing")
        summary["session_id"] = session_id
        summary["checks"].append("hub_http_session_hello_asset_caps")

        create_track = command_request(
            hub_url,
            session_id=session_id,
            command="track.create",
            timeout_ms=args.timeout_ms,
            timeout_sec=args.timeout_sec,
        )
        require(create_track.get("type") == "command.response", f"track.create: bad type {create_track!r}")
        created_track_id = str(((create_track.get("payload") or {}).get("legacy_reply") or {}).get("track_id") or "").strip()
        require(created_track_id, f"track.create: track_id missing {create_track!r}")
        summary["created_track_id"] = created_track_id
        summary["checks"].append("hub_http_command_track_create")

        temp_wav_path = make_temp_wav()
        import_clip = legacy_command_request(
            hub_url,
            session_id=session_id,
            cmd="import_audio",
            args={"track_id": created_track_id, "file_path": temp_wav_path, "offset_time": 0.0},
            timeout_ms=args.timeout_ms,
            timeout_sec=args.timeout_sec,
        )
        require(import_clip.get("type") == "command.response", f"import_audio: bad type {import_clip!r}")
        import_reply = (import_clip.get("payload") or {}).get("legacy_reply") or {}
        require(import_reply.get("status") == "ok", f"import_audio: legacy reply not ok {import_clip!r}")
        imported_clip_id = str(import_reply.get("clip_id") or "").strip()
        require(imported_clip_id, f"import_audio: clip_id missing {import_clip!r}")
        summary["imported_clip_id"] = imported_clip_id
        summary["checks"].append("hub_http_import_audio_for_asset")

        asset = post_vsp(
            hub_url,
            envelope(
                channel="asset",
                message_type="asset.request",
                schema="vsp.asset.request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={"clip_id": imported_clip_id, "kind": "waveform_peak"},
                timeout_ms=args.timeout_ms,
            ),
            args.timeout_sec,
        )
        assert_asset_reference(asset, kind="waveform_peak", clip_id=imported_clip_id)
        summary["checks"].append("hub_http_asset_reference_no_big_json")

        manifest = post_vsp(
            hub_url,
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
            args.timeout_sec,
        )
        assert_asset_manifest(manifest, kind="waveform_peak", track_id=created_track_id, clip_id=imported_clip_id)
        summary["checks"].append("hub_http_asset_manifest_visible_no_big_json")

        remove_clip = legacy_command_request(
            hub_url,
            session_id=session_id,
            cmd="remove_clips",
            args={"clip_ids": [imported_clip_id]},
            timeout_ms=args.timeout_ms,
            timeout_sec=args.timeout_sec,
        )
        require(remove_clip.get("type") == "command.response", f"remove_clips: bad type {remove_clip!r}")
        require(((remove_clip.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok", f"remove_clips: legacy reply not ok {remove_clip!r}")
        removed_imported_clip = True
        summary["checks"].append("hub_http_remove_clip_cleanup")

        delete_track = command_request(
            hub_url,
            session_id=session_id,
            command="track.delete",
            args={"track_id": created_track_id},
            timeout_ms=args.timeout_ms,
            timeout_sec=args.timeout_sec,
        )
        require(delete_track.get("type") == "command.response", f"track.delete: bad type {delete_track!r}")
        require(((delete_track.get("payload") or {}).get("legacy_reply") or {}).get("status") == "ok", f"track.delete: legacy reply not ok {delete_track!r}")
        deleted_created_track = True
        summary["checks"].append("hub_http_track_delete_cleanup")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if created_track_id and not deleted_created_track:
            try:
                if imported_clip_id and not removed_imported_clip:
                    legacy_command_request(
                        hub_url,
                        session_id=session_id,
                        cmd="remove_clips",
                        args={"clip_ids": [imported_clip_id]},
                        timeout_ms=args.timeout_ms,
                        timeout_sec=args.timeout_sec,
                    )
                command_request(
                    hub_url,
                    session_id=session_id,
                    command="track.delete",
                    args={"track_id": created_track_id},
                    timeout_ms=args.timeout_ms,
                    timeout_sec=args.timeout_sec,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for track {created_track_id}: {cleanup_error}", file=os.sys.stderr)
        if temp_wav_path and os.path.exists(temp_wav_path):
            try:
                os.remove(temp_wav_path)
            except OSError as cleanup_error:
                print(f"temp wav cleanup failed for {temp_wav_path}: {cleanup_error}", file=os.sys.stderr)


if __name__ == "__main__":
    main()
