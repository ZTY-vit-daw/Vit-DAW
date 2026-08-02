#!/usr/bin/env python3
"""Kernel contract smoke for Agent store v2 save/copy semantics."""

from __future__ import annotations

import argparse
import json
import math
import shutil
import struct
import tempfile
import wave
from pathlib import Path
from typing import Any, Iterable

import zmq


def send(sock: zmq.Socket, payload: dict[str, Any]) -> dict[str, Any]:
    sock.send_json(payload)
    raw = sock.recv()
    value = json.loads(raw.decode("utf-8"))
    if not isinstance(value, dict):
        raise RuntimeError(f"non-object reply: {value!r}")
    return value


def require_ok(label: str, reply: dict[str, Any]) -> dict[str, Any]:
    if str(reply.get("status", "")).lower() not in {"ok", "ready", "success", "media_preflight"}:
        raise RuntimeError(f"{label} failed: {reply}")
    return reply


def write_tone(path: Path, seconds: float = 0.25, rate: int = 48000) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    count = int(seconds * rate)
    with wave.open(str(path), "wb") as output:
        output.setnchannels(1)
        output.setsampwidth(2)
        output.setframerate(rate)
        frames = bytearray()
        for index in range(count):
            value = int(0.2 * 32767 * math.sin(2.0 * math.pi * 440.0 * index / rate))
            frames.extend(struct.pack("<h", value))
        output.writeframes(frames)


def all_strings(value: Any) -> Iterable[str]:
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for child in value.values():
            yield from all_strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from all_strings(child)


def same_path(left: str, right: str) -> bool:
    return Path(left).resolve() == Path(right).resolve()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--root", type=Path)
    parser.add_argument("--timeout-ms", type=int, default=15000)
    args = parser.parse_args()

    root = args.root.resolve() if args.root else Path(tempfile.mkdtemp(prefix="vit-v2-kernel-smoke-"))
    root.mkdir(parents=True, exist_ok=True)
    audio = root / "source_audio" / "tone.wav"
    source_project = root / "source" / "Song.vit"
    reference_project = root / "reference_export" / "Song.vit"
    copied_project = root / "copied_export" / "Song.vit"
    save_as_project = root / "normal_save_as" / "SongCopy.vit"
    write_tone(audio)

    context = zmq.Context.instance()
    sock = context.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, args.timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, args.timeout_ms)
    sock.connect(args.url)

    checks: list[str] = []
    require_ok("ping", send(sock, {"cmd": "ping"}))
    require_ok("new_project", send(sock, {"cmd": "new_project"}))
    saved = require_ok("save_as_project", send(sock, {"cmd": "save_as_project", "file_path": str(source_project)}))
    source_uuid = str(saved.get("project_uuid") or "")
    if not source_uuid:
        raise RuntimeError(f"save_as_project omitted UUID: {saved}")
    track = require_ok("add_audio_track", send(sock, {"cmd": "add_audio_track"}))
    imported = require_ok(
        "import_media_to_track",
        send(sock, {
            "cmd": "import_media_to_track", "track_id": track["track_id"],
            "file_path": str(audio), "media_type": "audio", "mode": "non_destructive", "start_time": 0.0,
        }),
    )
    if not imported.get("clip_id"):
        raise RuntimeError(f"import omitted clip id: {imported}")
    require_ok("save_project", send(sock, {"cmd": "save_project"}))
    source_state = require_ok("source state", send(sock, {"cmd": "get_project_state"}))
    checks.append("source_project_saved")

    preflight = send(sock, {"cmd": "save_project_copy", "file_path": str(reference_project), "preflight_only": True})
    if str(preflight.get("status", "")).lower() != "require_media_policy" or int(preflight.get("referenced_audio_count", 0)) != 1:
        raise RuntimeError(f"media policy preflight contract failed: {preflight}")
    checks.append("media_policy_required")

    reference = require_ok(
        "reference_only copy",
        send(sock, {"cmd": "save_project_copy", "file_path": str(reference_project), "media_policy": "reference_only"}),
    )
    copied = require_ok(
        "copy_referenced_audio copy",
        send(sock, {"cmd": "save_project_copy", "file_path": str(copied_project), "media_policy": "copy_referenced_audio"}),
    )
    for label, reply in (("reference", reference), ("copied", copied)):
        if not reply.get("active_project_unchanged") or reply.get("active_project_uuid") != source_uuid:
            raise RuntimeError(f"{label} export changed active identity: {reply}")
        if not same_path(str(reply.get("active_project_path", "")), str(source_project)):
            raise RuntimeError(f"{label} export changed active path: {reply}")
        if reply.get("project_uuid") == source_uuid:
            raise RuntimeError(f"{label} export reused source UUID: {reply}")
    checks.append("active_identity_unchanged")

    copied_rows = copied.get("media") or []
    if len(copied_rows) != 1:
        raise RuntimeError(f"copied export media rows invalid: {copied_rows}")
    copy_path = Path(str(copied_rows[0].get("copy_path", "")))
    copy_path.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(audio, copy_path)
    if copy_path.read_bytes() != audio.read_bytes():
        raise RuntimeError("copied audio differs from source")

    require_ok("open reference export", send(sock, {"cmd": "open_project", "file_path": str(reference_project)}))
    reference_state = require_ok("reference state", send(sock, {"cmd": "get_project_state"}))
    if not any(same_path(text, str(audio)) for text in all_strings(reference_state) if text.lower().endswith(".wav")):
        raise RuntimeError(f"reference export did not retain original audio reference: {reference_state}")
    checks.append("reference_only_reopens")

    require_ok("open copied export", send(sock, {"cmd": "open_project", "file_path": str(copied_project)}))
    copied_state = require_ok("copied state", send(sock, {"cmd": "get_project_state"}))
    if not any(same_path(text, str(copy_path)) for text in all_strings(copied_state) if text.lower().endswith(".wav")):
        raise RuntimeError(f"copied export did not resolve package audio: {copied_state}")
    checks.append("copied_audio_reopens")

    reopened = require_ok("reopen source", send(sock, {"cmd": "open_project", "file_path": str(source_project)}))
    if reopened.get("project_uuid") != source_uuid:
        raise RuntimeError(f"source identity changed after exports: {reopened}")

    normal_save_as = require_ok("normal Save As", send(sock, {"cmd": "save_as_project", "file_path": str(save_as_project)}))
    if normal_save_as.get("project_uuid") == source_uuid:
        raise RuntimeError(f"normal Save As reused source UUID: {normal_save_as}")
    save_as_state = require_ok("normal Save As state", send(sock, {"cmd": "get_project_state"}))
    if not any(same_path(text, str(audio)) for text in all_strings(save_as_state) if text.lower().endswith(".wav")):
        raise RuntimeError(f"normal Save As broke the original media reference: {save_as_state}")
    if (save_as_project.parent / "SongCopy_Media").exists():
        raise RuntimeError("normal Save As unexpectedly copied media")
    checks.append("normal_save_as_rebinds_without_media_copy")

    reopened = require_ok("reopen source after Save As", send(sock, {"cmd": "open_project", "file_path": str(source_project)}))
    if reopened.get("project_uuid") != source_uuid:
        raise RuntimeError(f"source identity changed after normal Save As: {reopened}")
    if source_project.read_bytes() == reference_project.read_bytes() and source_state == copied_state:
        raise RuntimeError("export snapshots unexpectedly collapsed to the source identity")
    checks.append("source_reopens_unchanged")

    print(json.dumps({"status": "ok", "root": str(root), "source_uuid": source_uuid, "checks": checks}, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
