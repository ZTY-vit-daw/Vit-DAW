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

Probe mode (M3-ENVELOPE-TRACK-SOURCE-1, --probe-tracks N > 0) probes the
track-level parameter family addressing instead of running the assertion
legs above: add N audio tracks, then for every discovered built-in track
plugin row and every candidate param_id, record verbatim:
automation.get_curve empty disclosure -> automation.set_points three points
(one value-range retry parsed from the verbatim rejection, if out of range)
-> readback point-by-point -> cross-track isolation check ->
automation.clear_region -> read-back empty. Hard assertions anchor only the
189-proven volume param (two tracks, full cycle, cross-track isolation);
every other candidate is recorded as-is (an unaddressable param is a
legitimate probe outcome, not a failure). get_plugin_parameters is also
disclosed verbatim per track volume plugin (known: the descriptor face is
external-plugins-only, so envelope_point_count cannot cover track params).
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


# ----------------------------------------------------------------------------
# Probe mode (M3-ENVELOPE-TRACK-SOURCE-1): track-level parameter family
# addressing disclosure. All replies are recorded verbatim; hard assertions
# anchor only the 189-proven volume param.
# ----------------------------------------------------------------------------

DEFAULT_PROBE_PARAMS = ["volume", "pan", "master volume", "master pan", "trim", "mute", "gain"]


def is_error_reply(reply: dict[str, Any]) -> bool:
    return str(reply.get("status", "")).strip().lower() == "error"


def reply_error_verbatim(reply: dict[str, Any]) -> str:
    return str(reply.get("message") or reply.get("error") or json.dumps(reply, ensure_ascii=False))


def find_track_plugins(state: dict[str, Any], track_id: str) -> list[dict[str, Any]]:
    """Return the raw plugin rows of one track from a get_project_state reply."""
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
        return [p for p in plugins if isinstance(p, dict)]
    raise RuntimeError(f"track {track_id} not found in project snapshot")


def plugin_row_id(plugin_row: dict[str, Any]) -> str:
    plugin_id = str(plugin_row.get("id") or plugin_row.get("plugin_item_id") or plugin_row.get("plugin_id") or "").strip()
    require(bool(plugin_id), f"plugin row carries no id: {plugin_row}")
    return plugin_id


def plugin_row_kind(plugin_row: dict[str, Any]) -> str:
    return str(plugin_row.get("name") or "") + "/" + str(plugin_row.get("type") or "")


def is_volume_plugin_row(plugin_row: dict[str, Any]) -> bool:
    kind = plugin_row_kind(plugin_row).lower()
    return "volume" in kind


def find_sibling_plugin_row(sibling_plugins: list[dict[str, Any]] | None, plugin_row: dict[str, Any]) -> dict[str, Any] | None:
    """Pick the sibling track's same-kind plugin row for the isolation check."""
    if not sibling_plugins:
        return None
    kind = plugin_row_kind(plugin_row)
    for candidate in sibling_plugins:
        if plugin_row_kind(candidate) == kind:
            return candidate
    return None


def probe_initial_points(param_id: str) -> list[dict[str, Any]]:
    lowered = param_id.lower()
    if "pan" in lowered:
        # Symmetric values inside the expected [-1, 1] pan range.
        return [{"t": 0.0, "v": -0.4, "c": 0.0}, {"t": 1.5, "v": 0.0, "c": 0.25}, {"t": 3.0, "v": 0.4, "c": 0.0}]
    if "volume" in lowered:
        return [{"t": 0.0, "v": 0.8, "c": 0.0}, {"t": 1.5, "v": 0.5, "c": 0.25}, {"t": 3.0, "v": 0.2, "c": 0.0}]
    # Unknown family: conservative small positive values; the value-range
    # retry below recovers from a fail-closed rejection by disclosing the
    # actual range verbatim.
    return [{"t": 0.0, "v": 0.5, "c": 0.0}, {"t": 1.5, "v": 0.4, "c": 0.25}, {"t": 3.0, "v": 0.3, "c": 0.0}]


def parse_disclosed_value_range(message: str) -> tuple[float, float] | None:
    """Extract (start, end) from "v outside the parameter value range [a, b]"."""
    marker = "the parameter value range ["
    index = message.find(marker)
    if index < 0:
        return None
    tail = message[index + len(marker):]
    end_bracket = tail.find("]")
    if end_bracket < 0:
        return None
    parts = tail[:end_bracket].split(",")
    if len(parts) != 2:
        return None
    try:
        return float(parts[0].strip()), float(parts[1].strip())
    except ValueError:
        return None


def points_inside_range(value_range: tuple[float, float]) -> list[dict[str, Any]]:
    start, end = value_range
    span = end - start
    return [
        {"t": 0.0, "v": start + 0.8 * span, "c": 0.0},
        {"t": 1.5, "v": start + 0.5 * span, "c": 0.25},
        {"t": 3.0, "v": start + 0.2 * span, "c": 0.0},
    ]


def curve_readback_match(curve: dict[str, Any], expected: list[dict[str, Any]]) -> bool:
    points = curve.get("points")
    if not isinstance(points, list) or len(points) != len(expected):
        return False
    for point, want in zip(points, expected):
        if not isinstance(point, dict):
            return False
        if not approx(point.get("t"), want["t"]) or not approx(point.get("v"), want["v"]) or not approx(point.get("c"), want["c"]):
            return False
    return True


def probe_one_param(
    sock: zmq.Socket,
    command_timeout_ms: int,
    track_index: int,
    track_id: str,
    plugin_row: dict[str, Any],
    param_id: str,
    sibling_plugin_row: dict[str, Any] | None,
) -> dict[str, Any]:
    """Probe one (plugin, param) pair; record everything verbatim, assert nothing."""
    row: dict[str, Any] = {
        "track_index": track_index,
        "track_id": track_id,
        "plugin_kind": plugin_row_kind(plugin_row),
        "plugin_id": plugin_row_id(plugin_row),
        "param_id": param_id,
        "addressable": False,
        "anchor_volume": False,
    }
    curve_args = {"plugin_id": row["plugin_id"], "param_id": param_id}

    empty_reply = send_command(sock, {"cmd": "automation.get_curve", **curve_args})
    row["get_curve_empty_verbatim"] = empty_reply
    if is_error_reply(empty_reply):
        row["errors_verbatim"] = [reply_error_verbatim(empty_reply)]
        return row

    row["addressable"] = True
    row["anchor_volume"] = param_id.strip().lower() == "volume" and is_volume_plugin_row(plugin_row)
    if row["anchor_volume"]:
        # 189 anchor: fresh track volume curve must be empty with disclosures.
        require(int(empty_reply.get("point_count", -1)) == 0, f"fresh volume curve must be empty: {empty_reply}")
        require(empty_reply.get("points") == [], f"fresh volume curve points must be an empty array: {empty_reply}")

    points = probe_initial_points(param_id)
    write_reply = send_command(sock, {"cmd": "automation.set_points", **curve_args, "points": points, "command_timeout_ms": command_timeout_ms})
    row["set_points_first_verbatim"] = write_reply
    if is_error_reply(write_reply) and parse_disclosed_value_range(reply_error_verbatim(write_reply)) is not None:
        # Out-of-range v: the kernel disclosed the real value range verbatim;
        # retry once inside it so the readback/clear legs still get probed.
        value_range = parse_disclosed_value_range(reply_error_verbatim(write_reply)) or (0.0, 1.0)
        points = points_inside_range(value_range)
        row["points_used"] = points
        write_reply = send_command(sock, {"cmd": "automation.set_points", **curve_args, "points": points, "command_timeout_ms": command_timeout_ms})
        row["set_points_retry_verbatim"] = write_reply

    if is_error_reply(write_reply):
        row.setdefault("errors_verbatim", []).append(reply_error_verbatim(write_reply))
        return row

    written = int(write_reply.get("written", -1))
    if row["anchor_volume"]:
        require(written == 3 and int(write_reply.get("point_count", -1)) == 3, f"volume set_points must write 3 points: {write_reply}")

    readback = send_command(sock, {"cmd": "automation.get_curve", **curve_args})
    row["get_curve_readback_verbatim"] = readback
    row["readback_match"] = curve_readback_match(readback, points)
    if row["anchor_volume"]:
        require(not is_error_reply(readback), f"volume readback must succeed: {readback}")
        require(int(readback.get("point_count", -1)) == 3, f"volume readback point_count must be 3: {readback}")
        require(row["readback_match"], f"volume readback must match point-by-point: {readback} want {points}")

    if sibling_plugin_row is not None and param_id.strip().lower() in {"volume", "pan"}:
        # plugin_id addressing has no track_id: prove the sibling track's own
        # plugin still reads empty while this track holds three points.
        sibling_reply = send_command(sock, {"cmd": "automation.get_curve", "plugin_id": plugin_row_id(sibling_plugin_row), "param_id": param_id})
        row["crosscheck_sibling_get_curve_verbatim"] = sibling_reply
        row["crosscheck_sibling_empty"] = not is_error_reply(sibling_reply) and int(sibling_reply.get("point_count", -1)) == 0
        if row["anchor_volume"]:
            require(row["crosscheck_sibling_empty"], f"sibling track volume curve must stay empty (plugin_id isolation): {sibling_reply}")

    clear_reply = send_command(sock, {"cmd": "automation.clear_region", **curve_args, "region": {"start": 0.0, "end": 5.0}, "command_timeout_ms": command_timeout_ms})
    row["clear_region_verbatim"] = clear_reply
    if is_error_reply(clear_reply):
        row.setdefault("errors_verbatim", []).append(reply_error_verbatim(clear_reply))
        return row
    if row["anchor_volume"]:
        require(int(clear_reply.get("removed_points", -1)) == 3, f"volume clear_region must remove 3 points: {clear_reply}")

    after_clear = send_command(sock, {"cmd": "automation.get_curve", **curve_args})
    row["get_curve_after_clear_verbatim"] = after_clear
    row["empty_after_clear"] = not is_error_reply(after_clear) and int(after_clear.get("point_count", -1)) == 0
    if row["anchor_volume"]:
        require(row["empty_after_clear"], f"volume curve must read back empty after clear: {after_clear}")

    return row


def probe_track_sources(sock: zmq.Socket, project_path: Path, probe_tracks: int, probe_params: list[str], command_timeout_ms: int) -> dict[str, Any]:
    """M3-ENVELOPE-TRACK-SOURCE-1 probe body; returns the summary dict."""
    summary: dict[str, Any] = {
        "status": "running",
        "mode": "track_source_probe",
        "project_path": str(project_path),
        "probe_tracks": probe_tracks,
        "probe_params": probe_params,
        "tracks": [],
        "plugin_parameter_descriptor_probes": [],
        "probe_rows": [],
    }

    require_ok("new_project", send_command(sock, {"cmd": "new_project", "command_timeout_ms": command_timeout_ms}))
    require_ok(
        "save_as_project",
        send_command(sock, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": command_timeout_ms}),
    )

    track_ids: list[str] = []
    for index in range(probe_tracks):
        reply = require_ok(f"add_audio_track #{index + 1}", send_command(sock, {"cmd": "add_audio_track"}))
        track_id = str(reply.get("track_id") or "").strip()
        require(track_id, f"add_audio_track #{index + 1} did not return track_id: {reply}")
        track_ids.append(track_id)

    state = require_ok("get_project_state", send_command(sock, {"cmd": "get_project_state"}))
    tracks_out: list[dict[str, Any]] = []
    plugins_per_track: list[list[dict[str, Any]]] = []
    for index, track_id in enumerate(track_ids):
        plugins = find_track_plugins(state, track_id)
        require(bool(plugins), f"track {track_id} (index {index}) exposes no plugin rows: {state}")
        tracks_out.append({"track_index": index, "track_id": track_id, "plugins": plugins})
        plugins_per_track.append(plugins)
    summary["tracks"] = tracks_out

    # Known descriptor-face boundary, re-disclosed verbatim per track volume
    # plugin (189 anchor: external plugins only, so envelope_point_count
    # cannot cover track-level params on this face).
    for index, plugins in enumerate(plugins_per_track):
        volume_rows = [p for p in plugins if is_volume_plugin_row(p)]
        require(bool(volume_rows), f"track index {index} has no volume plugin row (189 anchor): {plugins}")
        descriptor_reply = send_command(sock, {"cmd": "get_plugin_parameters", "track_id": track_ids[index], "plugin_id": plugin_row_id(volume_rows[0])})
        summary["plugin_parameter_descriptor_probes"].append(
            {
                "track_index": index,
                "track_id": track_ids[index],
                "plugin_id": plugin_row_id(volume_rows[0]),
                "reply_verbatim": descriptor_reply,
                "is_error": is_error_reply(descriptor_reply),
                "error_verbatim": reply_error_verbatim(descriptor_reply) if is_error_reply(descriptor_reply) else None,
            }
        )
        if is_error_reply(descriptor_reply):
            # Anchor expectation: the descriptor face rejects track-level
            # built-ins; the exact wording is recorded, only error-ness is
            # asserted (kernel wording may evolve without breaking this gap).
            require(
                "external" in reply_error_verbatim(descriptor_reply).lower(),
                f"descriptor-face rejection wording diverged from the 189 anchor: {descriptor_reply}",
            )

    # Pair each track with the next track's plugin list (mod N) for the
    # cross-track isolation check; with a single track the sibling is None.
    for index, plugins in enumerate(plugins_per_track):
        sibling_plugins = plugins_per_track[(index + 1) % probe_tracks] if probe_tracks > 1 else None
        for plugin_row in plugins:
            for param_id in probe_params:
                row = probe_one_param(
                    sock,
                    command_timeout_ms,
                    index,
                    track_ids[index],
                    plugin_row,
                    param_id,
                    find_sibling_plugin_row(sibling_plugins, plugin_row),
                )
                summary["probe_rows"].append(row)
                marker = "OK" if row["addressable"] else "n/a"
                print(f"[probe] track#{index} {plugin_row_kind(plugin_row)} plugin={row['plugin_id']} param={param_id!r}: addressable={marker}"
                      + (f" readback_match={row.get('readback_match')}" if row["addressable"] else ""))

    # Anchor roll-up: both tracks must have run the full volume cycle.
    anchor_rows = [r for r in summary["probe_rows"] if r["anchor_volume"]]
    anchor_full_cycles = [r for r in anchor_rows if r.get("readback_match") and r.get("empty_after_clear")]
    anchor_crosschecks = [r for r in anchor_rows if r.get("crosscheck_sibling_empty")]
    summary["anchor_assertions"] = {
        "volume_anchor_rows": len(anchor_rows),
        "volume_full_cycles": len(anchor_full_cycles),
        "volume_crosschecks": len(anchor_crosschecks),
        "expected_anchor_rows": probe_tracks,
    }
    require(len(anchor_rows) == probe_tracks, f"volume anchor must cover every track: {len(anchor_rows)}/{probe_tracks}")
    require(len(anchor_full_cycles) == probe_tracks, f"volume anchor full cycle must pass on every track: {anchor_full_cycles}")
    require(len(anchor_crosschecks) == probe_tracks, f"volume anchor cross-track isolation must pass on every track: {anchor_crosschecks}")
    summary["status"] = "ok"
    return summary


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=30000)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", default="")
    parser.add_argument("--probe-tracks", type=int, default=0,
                        help="M3-ENVELOPE-TRACK-SOURCE-1 probe mode: add N audio tracks and probe the "
                             "track-level parameter family; 0 keeps the M3-KERNEL-1 assertion legs")
    parser.add_argument("--probe-params", default=",".join(DEFAULT_PROBE_PARAMS),
                        help="comma-separated candidate param_id list for probe mode")
    args = parser.parse_args()

    if args.probe_tracks > 0:
        probe_params = [p.strip() for p in args.probe_params.split(",") if p.strip()]
        require(bool(probe_params), "--probe-params must carry at least one candidate param_id")

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    elif args.probe_tracks > 0:
        temp_dir = Path(tempfile.gettempdir()) / "vit_daw_track_source_probe"
        temp_dir.mkdir(parents=True, exist_ok=True)
        project_path = temp_dir / f"track_source_probe_{int(time.time())}.vit"
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

    if args.probe_tracks > 0:
        label = "track source probe"
        try:
            summary = probe_track_sources(sock, project_path, args.probe_tracks, probe_params, command_timeout_ms)
            print("track source probe: PASS")
            for assertion, value in summary["anchor_assertions"].items():
                print(f"  - {assertion}: {value}")
            return 0
        except Exception as exc:  # noqa: BLE001 - top-level probe failure reporting.
            summary = {
                "status": "failed",
                "mode": "track_source_probe",
                "project_path": str(project_path),
                "probe_params": probe_params,
                "error": str(exc),
            }
            print(f"track source probe: FAIL: {exc}", file=sys.stderr)
            return 1
        finally:
            sock.close(0)
            write_summary_output(args, summary)

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
        write_summary_output(args, summary)


def write_summary_output(args: argparse.Namespace, summary: dict[str, Any]) -> None:
    if args.output:
        output_path = Path(args.output).resolve()
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"summary written: {output_path}")
    else:
        print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(main())
