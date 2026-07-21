"""Smoke-test Vit track folders and folder bus routing over ZMQ."""

from __future__ import annotations

import argparse
import json
import sys
import time
from typing import Any, Dict, List

import zmq


def send_command(sock: zmq.Socket, payload: Dict[str, Any]) -> Dict[str, Any]:
    cmd = payload.get("cmd")
    try:
        sock.send_string(json.dumps(payload))
        raw = sock.recv()
    except zmq.Again as exc:
        raise RuntimeError(f"ZMQ timeout while waiting for {cmd}") from exc
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8", errors="replace")
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON reply for {payload.get('cmd')}: {raw!r}") from exc
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object reply for {payload.get('cmd')}: {reply!r}")
    return reply


def require_ok(reply: Dict[str, Any], label: str) -> Dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label} failed: {json.dumps(reply, ensure_ascii=False)}")
    return reply


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


def wait_for_ping(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    timeout_s = timeout_ms / 1000.0
    deadline = time.time() + timeout_s
    last_error: Exception | None = None
    while time.time() < deadline:
        sock = make_socket(ctx, url, timeout_ms)
        try:
            require_ok(send_command(sock, {"cmd": "ping"}), "ping")
            return sock
        except Exception as exc:  # noqa: BLE001 - diagnostic retry loop.
            last_error = exc
            sock.close(0)
            time.sleep(0.25)
    raise RuntimeError(f"kernel did not answer ping within {timeout_s:.1f}s: {last_error}")


def user_track_rows(reply: Dict[str, Any]) -> List[Dict[str, Any]]:
    tracks = reply.get("tracks", [])
    if not isinstance(tracks, list):
        return []
    out: List[Dict[str, Any]] = []
    for raw in tracks:
        if not isinstance(raw, dict):
            continue
        track_id = str(raw.get("track_id", raw.get("id", ""))).strip()
        track_type = str(raw.get("track_type", raw.get("type", ""))).strip().lower()
        if not track_id or track_type == "master":
            continue
        out.append(raw)
    return out


def track_id(row: Dict[str, Any]) -> str:
    return str(row.get("track_id", row.get("id", ""))).strip()


def ensure_audio_tracks(sock: zmq.Socket, count: int) -> List[str]:
    state = require_ok(send_command(sock, {"cmd": "list_tracks"}), "list_tracks")
    rows = [
        row for row in user_track_rows(state)
        if bool(row.get("is_audio_track", row.get("is_audio", False)))
    ]
    while len(rows) < count:
        require_ok(send_command(sock, {"cmd": "add_audio_track"}), "add_audio_track")
        state = require_ok(send_command(sock, {"cmd": "list_tracks"}), "list_tracks")
        rows = [
            row for row in user_track_rows(state)
            if bool(row.get("is_audio_track", row.get("is_audio", False)))
        ]
    return [track_id(row) for row in rows[:count]]


def find_track(sock: zmq.Socket, wanted_id: str) -> Dict[str, Any]:
    state = require_ok(send_command(sock, {"cmd": "list_tracks"}), "list_tracks")
    for row in user_track_rows(state):
        if track_id(row) == wanted_id:
            return row
    raise RuntimeError(f"track not found in state: {wanted_id}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=8000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    sock = wait_for_ping(ctx, args.url, args.timeout_ms)
    require_ok(send_command(sock, {"cmd": "new_project"}), "new_project")
    track_ids = ensure_audio_tracks(sock, 3)
    apply_reply = require_ok(
        send_command(
            sock,
            {
                "cmd": "project.apply_track_organization",
                "groups": [
                    {
                        "folder_name": "Smoke Folder",
                        "track_ids": track_ids[:2],
                    }
                ],
            },
        ),
        "project.apply_track_organization",
    )
    folder_ids = apply_reply.get("created_folder_ids", [])
    if not isinstance(folder_ids, list) or not folder_ids:
        raise RuntimeError(f"missing created_folder_ids: {apply_reply}")
    folder_id = str(folder_ids[0]).strip()
    folder = find_track(sock, folder_id)
    if str(folder.get("track_type", "")).strip().lower() != "folder":
        raise RuntimeError(f"created row is not a folder: {folder}")
    if bool(folder.get("routing_bus_enabled", False)):
        raise RuntimeError(f"folder bus should be disabled by default: {folder}")
    child = find_track(sock, track_ids[0])
    if str(child.get("parent_track_id", "")).strip() != folder_id:
        raise RuntimeError(f"child parent_track_id mismatch: {child}")

    require_ok(
        send_command(
            sock,
            {
                "cmd": "folder_track.set_routing_bus_enabled",
                "folder_track_id": folder_id,
                "routing_bus_enabled": True,
            },
        ),
        "enable folder bus",
    )
    folder = find_track(sock, folder_id)
    if not bool(folder.get("routing_bus_enabled", False)):
        raise RuntimeError(f"folder bus did not enable: {folder}")
    require_ok(
        send_command(
            sock,
            {
                "cmd": "folder_track.set_routing_bus_enabled",
                "folder_track_id": folder_id,
                "routing_bus_enabled": False,
            },
        ),
        "disable folder bus",
    )
    folder = find_track(sock, folder_id)
    if bool(folder.get("routing_bus_enabled", False)):
        raise RuntimeError(f"folder bus did not disable: {folder}")

    print(json.dumps({
        "status": "ok",
        "folder_track_id": folder_id,
        "moved_track_ids": track_ids[:2],
        "checked": [
            "default_container_folder",
            "child_parent_track_id",
            "routing_bus_toggle_on",
            "routing_bus_toggle_off",
        ],
    }, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001 - smoke script top-level diagnostic.
        print(f"track_folder_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
