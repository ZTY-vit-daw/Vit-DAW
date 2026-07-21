"""Smoke-test Vit track groups and group volume control over ZMQ."""

from __future__ import annotations

import argparse
import json
import math
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
        raise RuntimeError(f"invalid JSON reply for {cmd}: {raw!r}") from exc
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object reply for {cmd}: {reply!r}")
    return reply


def require_ok(reply: Dict[str, Any], label: str) -> Dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    if status not in {"ok", "success", "completed", "partial"}:
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


def user_audio_track_rows(reply: Dict[str, Any]) -> List[Dict[str, Any]]:
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
        if bool(raw.get("is_audio_track", raw.get("is_audio", False))):
            out.append(raw)
    return out


def track_id(row: Dict[str, Any]) -> str:
    return str(row.get("track_id", row.get("id", ""))).strip()


def read_volume_db(row: Dict[str, Any]) -> float:
    for key in ("volume_db", "fader_db", "gain_db", "db"):
        if key not in row:
            continue
        try:
            value = float(row[key])
        except (TypeError, ValueError):
            continue
        if math.isfinite(value):
            return value
    raise RuntimeError(f"track row has no finite volume db: {row}")


def ensure_audio_tracks(sock: zmq.Socket, count: int) -> List[str]:
    state = require_ok(send_command(sock, {"cmd": "list_tracks"}), "list_tracks")
    rows = user_audio_track_rows(state)
    while len(rows) < count:
        require_ok(send_command(sock, {"cmd": "add_audio_track"}), "add_audio_track")
        state = require_ok(send_command(sock, {"cmd": "list_tracks"}), "list_tracks")
        rows = user_audio_track_rows(state)
    return [track_id(row) for row in rows[:count]]


def tracks_by_id(sock: zmq.Socket) -> Dict[str, Dict[str, Any]]:
    state = require_ok(send_command(sock, {"cmd": "get_project_state"}), "get_project_state")
    return {track_id(row): row for row in user_audio_track_rows(state)}


def assert_track_volume(sock: zmq.Socket, wanted_id: str, expected_db: float) -> None:
    row = tracks_by_id(sock).get(wanted_id)
    if row is None:
        raise RuntimeError(f"track not found in get_project_state: {wanted_id}")
    actual = read_volume_db(row)
    if abs(actual - expected_db) > 0.05:
        raise RuntimeError(f"track {wanted_id} volume mismatch: got {actual:.3f}, want {expected_db:.3f}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=10000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    sock = wait_for_ping(ctx, args.url, args.timeout_ms)
    require_ok(send_command(sock, {"cmd": "new_project"}), "new_project")
    track_ids = ensure_audio_tracks(sock, 3)

    for tid, db in zip(track_ids[:2], (-18.0, -12.0)):
        require_ok(send_command(sock, {"cmd": "set_volume", "track_id": tid, "db": db}), f"set_volume {tid}")
        assert_track_volume(sock, tid, db)

    group = require_ok(
        send_command(
            sock,
            {
                "cmd": "track.group.create",
                "name": "Smoke Group",
                "track_ids": track_ids[:2],
                "origin": "smoke",
                "linked_controls": {"volume": True, "pan": False, "mute": False, "solo": False},
            },
        ),
        "track.group.create",
    )
    group_id = str(group.get("group_id", "")).strip()
    if not group_id:
        raise RuntimeError(f"missing group_id: {group}")

    listed = require_ok(send_command(sock, {"cmd": "track.group.list"}), "track.group.list")
    groups = listed.get("track_groups", [])
    if not isinstance(groups, list) or not any(str(g.get("group_id", g.get("id", ""))) == group_id for g in groups if isinstance(g, dict)):
        raise RuntimeError(f"created group not listed: {listed}")

    state = require_ok(send_command(sock, {"cmd": "get_project_state"}), "get_project_state")
    state_groups = state.get("track_groups", [])
    if not isinstance(state_groups, list) or not any(str(g.get("group_id", g.get("id", ""))) == group_id for g in state_groups if isinstance(g, dict)):
        raise RuntimeError(f"created group not exposed in project.state: {state_groups}")

    applied = require_ok(
        send_command(
            sock,
            {
                "cmd": "track.group.apply_control",
                "group_id": group_id,
                "control": "volume",
                "mode": "absolute",
                "db": 0.0,
            },
        ),
        "track.group.apply_control",
    )
    if int(applied.get("applied_count", 0)) != 2:
        raise RuntimeError(f"expected 2 applied members: {applied}")
    if int(applied.get("verified_count", 0)) != 2:
        raise RuntimeError(f"expected 2 verified members: {applied}")
    for member in applied.get("members", []):
        if not isinstance(member, dict):
            raise RuntimeError(f"bad member result: {member}")
        if abs(float(member.get("after_db", 999.0)) - 0.0) > 0.05:
            raise RuntimeError(f"member did not verify at 0 dB: {member}")

    for tid in track_ids[:2]:
        assert_track_volume(sock, tid, 0.0)

    for tid, db in zip(track_ids[:3], (-60.0, -18.0, -6.0)):
        require_ok(send_command(sock, {"cmd": "set_volume", "track_id": tid, "db": db}), f"set_volume reset-path {tid}")
        assert_track_volume(sock, tid, db)

    reset_group_id = "grp_smoke_b1_reference_reset"
    reset = require_ok(
        send_command(
            sock,
            {
                "cmd": "track.group.apply_control",
                "group_id": reset_group_id,
                "name": "B1 Reference Reset Smoke",
                "track_ids": track_ids[:3],
                "origin": "smoke_b1_gain_staging",
                "linked_controls": {"volume": True, "pan": False, "mute": False, "solo": False},
                "create_group_if_missing": True,
                "replace_members": True,
                "control": "volume",
                "mode": "absolute",
                "db": 0.0,
            },
        ),
        "track.group.apply_control create_group_if_missing",
    )
    if str(reset.get("group_id", "")).strip() != reset_group_id:
        raise RuntimeError(f"reset path returned wrong group_id: {reset}")
    if int(reset.get("applied_count", 0)) != 3:
        raise RuntimeError(f"expected 3 reset-path applied members: {reset}")
    if int(reset.get("verified_count", 0)) != 3:
        raise RuntimeError(f"expected 3 reset-path verified members: {reset}")
    if not bool(reset.get("group_created", False)):
        raise RuntimeError(f"reset path did not create group: {reset}")
    for tid in track_ids[:3]:
        assert_track_volume(sock, tid, 0.0)

    print(json.dumps({
        "status": "ok",
        "group_id": group_id,
        "b1_reset_group_id": reset_group_id,
        "member_track_ids": track_ids[:2],
        "checked": [
            "track_group_create",
            "track_group_list",
            "project_state_track_groups",
            "group_volume_absolute_to_0db",
            "group_apply_control_create_group_if_missing",
            "project_state_member_fader_verification",
        ],
    }, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001 - smoke script top-level diagnostic.
        print(f"track_group_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
