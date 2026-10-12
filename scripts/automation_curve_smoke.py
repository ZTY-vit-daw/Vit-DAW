#!/usr/bin/env python3
"""Smoke-test the kernel automation curve command family (M3-KERNEL-1).

Legs (against the live kernel ZMQ REQ face, same pattern as
clip_fade_gain_smoke.py):

1.  automation.get_curve on a fresh track volume param -> empty curve with
    time_base/bypass disclosure.
2.  Out-of-domain fail-closed (kernel side): negative t, curve c outside
    [-1, 1], and an empty points array must all be rejected without writing.
3.  automation.set_points writes three points; automation.get_curve reads
    them back point-by-point (t, v, c).
4.  automation.clear_region removes the region; automation.get_curve reads
    the curve back empty.
5.  CAS counter-example over a VSP envelope: a write with a wrong
    base_revision is rejected (stale_project_cut) and leaves the curve
    untouched, while the same write without base_revision succeeds.
"""

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
        "client_id": "smoke.automation_curve",
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
            "client_id": "smoke.automation_curve",
            "role": "test",
            "channel": "session",
            "type": "session.hello",
            "created_at": utc_now(),
            "trace_id": new_id("trace"),
            "payload": {
                "client_name": "Automation curve smoke",
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


def send_vsp_command_expect_rejected(
    sock: zmq.Socket,
    session_id: str,
    command: str,
    args: dict[str, Any],
    timeout_ms: int,
    error_code_pattern: str,
) -> dict[str, Any]:
    reply = send_command(
        sock,
        vsp_envelope(session_id, {"command": command, "args": args}, timeout_ms),
    )
    require(reply.get("type") == "command.error", f"VSP {command} CAS rejection expected command.error, got: {reply}")
    error = reply.get("error")
    require(isinstance(error, dict), f"VSP {command} rejection carries no error object: {reply}")
    code = str(error.get("code") or "")
    message = str(error.get("message") or "")
    require(
        error_code_pattern in code or error_code_pattern in message,
        f"VSP {command} rejection reason mismatch: code={code!r} message={message!r}",
    )
    return reply


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_ok(label: str, reply: dict[str, Any]) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status in {"ok", "success", "completed"}, f"{label} failed: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def require_error(label: str, reply: dict[str, Any], message_pattern: str) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status == "error", f"{label} must be rejected fail-closed, got: {json.dumps(reply, ensure_ascii=False)}")
    message = str(reply.get("message") or "")
    require(message_pattern in message, f"{label} rejection reason mismatch: expected [{message_pattern}] got [{message}]")
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


def approx(actual: Any, expected: float, tolerance: float = 1e-6) -> bool:
    try:
        return abs(float(actual) - expected) <= tolerance
    except (TypeError, ValueError):
        return False


def find_track_volume_plugin(state: dict[str, Any], track_id: str) -> str:
    tracks = state.get("tracks")
    require(isinstance(tracks, list), "get_project_state tracks must be an array")
    for raw_track in tracks:
        if not isinstance(raw_track, dict):
            continue
        candidate = str(raw_track.get("track_id") or raw_track.get("id") or raw_track.get("item_id") or "").strip()
        if candidate != track_id:
            continue
        plugins = raw_track.get("plugins")
        require(isinstance(plugins, list), f"track {track_id} carries no plugins array: {raw_track}")
        for raw_plugin in plugins:
            if not isinstance(raw_plugin, dict):
                continue
            name = str(raw_plugin.get("name") or "")
            plugin_type = str(raw_plugin.get("type") or "")
            if "volume" in name.lower() or "volume" in plugin_type.lower():
                plugin_id = str(raw_plugin.get("id") or raw_plugin.get("plugin_item_id") or "").strip()
                require(bool(plugin_id), f"volume plugin row carries no id: {raw_plugin}")
                return plugin_id
        raise RuntimeError(f"track {track_id} has no volume/pan plugin in its plugin list: {plugins}")
    raise RuntimeError(f"track {track_id} not found in project snapshot")


def assert_curve_points(label: str, curve: dict[str, Any], expected: list[dict[str, Any]]) -> None:
    points = curve.get("points")
    require(isinstance(points, list), f"{label}: points must be an array: {curve}")
    require(len(points) == len(expected), f"{label}: point_count={len(points)} want {len(expected)}: {curve}")
    for index, (point, want) in enumerate(zip(points, expected)):
        require(isinstance(point, dict), f"{label}: point #{index} not an object: {point}")
        require(approx(point.get("t"), want["t"]), f"{label}: point #{index} t={point.get('t')!r} want {want['t']}")
        require(approx(point.get("v"), want["v"]), f"{label}: point #{index} v={point.get('v')!r} want {want['v']}")
        require(approx(point.get("c"), want["c"]), f"{label}: point #{index} c={point.get('c')!r} want {want['c']}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=30000)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", default="")
    args = parser.parse_args()

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    else:
        temp_dir = Path(tempfile.gettempdir()) / "vit_daw_automation_curve_smoke"
        temp_dir.mkdir(parents=True, exist_ok=True)
        project_path = temp_dir / f"automation_curve_{int(time.time())}.vit"

    command_timeout_ms = max(args.timeout_ms, 30000)
    points = [
        {"t": 0.0, "v": 0.8, "c": 0.0},
        {"t": 1.5, "v": 0.5, "c": 0.25},
        {"t": 3.0, "v": 0.2, "c": 0.0},
    ]

    ctx = zmq.Context.instance()
    sock = wait_for_kernel(ctx, args.url, args.timeout_ms)
    summary: dict[str, Any] = {
        "status": "running",
        "url": args.url,
        "project_path": str(project_path),
        "checks": [],
    }

    try:
        require_ok("new_project", send_command(sock, {"cmd": "new_project", "command_timeout_ms": command_timeout_ms}))
        summary["checks"].append("new_project")

        require_ok(
            "save_as_project",
            send_command(sock, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
        )
        summary["checks"].append("save_as_project")

        track_reply = require_ok("add_audio_track", send_command(sock, {"cmd": "add_audio_track"}))
        track_id = str(track_reply.get("track_id") or "").strip()
        require(track_id, f"add_audio_track did not return track_id: {track_reply}")
        summary["track_id"] = track_id
        summary["checks"].append("add_audio_track")

        state = require_ok("get_project_state", send_command(sock, {"cmd": "get_project_state"}))
        plugin_id = find_track_volume_plugin(state, track_id)
        summary["plugin_id"] = plugin_id
        summary["checks"].append("volume_plugin_discovered")

        get_curve_args = {"plugin_id": plugin_id, "param_id": "volume"}

        # LEG 1: empty curve with disclosure fields.
        empty_curve = require_ok("automation.get_curve empty", send_command(sock, {"cmd": "automation.get_curve", **get_curve_args}))
        require(int(empty_curve.get("point_count", -1)) == 0, f"fresh curve must be empty: {empty_curve}")
        require(empty_curve.get("points") == [], f"fresh curve points must be an empty array: {empty_curve}")
        require(str(empty_curve.get("time_base") or "") in {"time", "beats"}, f"time_base disclosure missing: {empty_curve}")
        require(isinstance(empty_curve.get("bypass"), bool), f"bypass disclosure missing: {empty_curve}")
        summary["checks"].append("automation.get_curve.empty")
        summary["time_base"] = empty_curve.get("time_base")

        # LEG 2: out-of-domain writes are rejected fail-closed without writing.
        require_error(
            "automation.set_points negative t",
            send_command(sock, {"cmd": "automation.set_points", **get_curve_args, "points": [{"t": -0.5, "v": 0.5}]}),
            "t < 0",
        )
        require_error(
            "automation.set_points curve c out of range",
            send_command(sock, {"cmd": "automation.set_points", **get_curve_args, "points": [{"t": 0.5, "v": 0.5, "c": 1.5}]}),
            "curve c outside",
        )
        require_error(
            "automation.set_points empty points",
            send_command(sock, {"cmd": "automation.set_points", **get_curve_args, "points": []}),
            "non-empty points array",
        )
        still_empty = require_ok("automation.get_curve after rejections", send_command(sock, {"cmd": "automation.get_curve", **get_curve_args}))
        require(int(still_empty.get("point_count", -1)) == 0, f"rejected writes must not touch the curve: {still_empty}")
        summary["checks"].append("automation.set_points.fail_closed")

        # LEG 3: write three points, read back point-by-point.
        written = require_ok(
            "automation.set_points",
            send_command(
                sock,
                {"cmd": "automation.set_points", **get_curve_args, "points": points, "command_timeout_ms": command_timeout_ms},
            ),
        )
        require(int(written.get("written", -1)) == 3, f"written must echo the point count: {written}")
        require(int(written.get("point_count", -1)) == 3, f"point_count after write must be 3: {written}")
        require(str(written.get("mode") or "") == "full", f"mode must disclose full replacement: {written}")
        summary["checks"].append("automation.set_points.write3")

        read_curve = require_ok("automation.get_curve after write", send_command(sock, {"cmd": "automation.get_curve", **get_curve_args}))
        require(int(read_curve.get("point_count", -1)) == 3, f"readback point_count must be 3: {read_curve}")
        assert_curve_points("automation.get_curve readback", read_curve, points)
        summary["checks"].append("automation.get_curve.readback3")

        # LEG 4: clear the region, read back empty.
        cleared = require_ok(
            "automation.clear_region",
            send_command(
                sock,
                {
                    "cmd": "automation.clear_region",
                    **get_curve_args,
                    "region": {"start": 0.0, "end": 5.0},
                    "command_timeout_ms": command_timeout_ms,
                },
            ),
        )
        require(int(cleared.get("removed_points", -1)) == 3, f"removed_points must be 3: {cleared}")
        require(int(cleared.get("point_count", -1)) == 0, f"point_count after clear must be 0: {cleared}")
        final_empty = require_ok("automation.get_curve after clear", send_command(sock, {"cmd": "automation.get_curve", **get_curve_args}))
        require(int(final_empty.get("point_count", -1)) == 0, f"curve must read back empty after clear: {final_empty}")
        summary["checks"].append("automation.clear_region")

        # LEG 5: CAS counter-example over a VSP envelope. The same write with
        # no base_revision succeeds; a wrong base_revision is rejected as a
        # stale cut and leaves the curve untouched.
        session_id = vsp_hello(sock, args.timeout_ms)
        summary["vsp_session_id"] = session_id

        vsp_written = send_vsp_command(
            sock,
            session_id,
            "automation.set_points",
            {**get_curve_args, "points": points},
            command_timeout_ms,
        )
        require(int(vsp_written.get("written", -1)) == 3, f"VSP write without base_revision must succeed: {vsp_written}")
        summary["checks"].append("vsp.automation.set_points.no_cas")

        send_vsp_command_expect_rejected(
            sock,
            session_id,
            "automation.set_points",
            {**get_curve_args, "points": points, "base_revision": 987654321},
            command_timeout_ms,
            "stale_project_cut",
        )
        summary["checks"].append("vsp.automation.set_points.cas_rejected")

        after_rejection = require_ok("automation.get_curve after CAS rejection", send_command(sock, {"cmd": "automation.get_curve", **get_curve_args}))
        require(int(after_rejection.get("point_count", -1)) == 3, f"CAS-rejected write must not touch the curve: {after_rejection}")
        summary["checks"].append("automation.get_curve.after_cas_rejection")

        # Leave the project clean.
        send_vsp_command(
            sock,
            session_id,
            "automation.clear_region",
            {**get_curve_args, "region": {"start": 0.0, "end": 5.0}},
            command_timeout_ms,
        )
        summary["checks"].append("vsp.automation.clear_region.cleanup")

        summary["status"] = "ok"
        print("automation curve smoke: PASS")
        for check in summary["checks"]:
            print(f"  - {check}")
        return 0
    except Exception as exc:  # noqa: BLE001 - top-level smoke failure reporting.
        summary["status"] = "failed"
        summary["error"] = str(exc)
        print(f"automation curve smoke: FAIL: {exc}", file=sys.stderr)
        return 1
    finally:
        sock.close(0)
        if args.output:
            output_path = Path(args.output).resolve()
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
            print(f"summary written: {output_path}")
        else:
            print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(main())
