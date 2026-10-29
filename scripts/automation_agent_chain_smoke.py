#!/usr/bin/env python3
"""Agent-side automation chain smoke (M3-C4-AUTO-DISCLOSE-1).

Same stack shape as sendroute_agent_smoke.py: kernel + agent berth
(dev_agent_smoke.ps1 -StartKernel -RestartAgent, no UI). The setup face uses
the kernel ZMQ command socket (project + tracks + external plugin load via
rack_add_node, the pgtdc probe precedent — never the PCA-gated agent load
tool); every assertion leg drives the real agent /agent/invoke HTTP face,
which is exactly the authority face this card certifies for the first time
(M3-KERNEL-1 proved the kernel command face only).

Legs:

A. external plugin parameter leg (agent invoke): get_plugin_parameters reads
   the target row's value range and envelope_point_count baseline ->
   automation.set_points with confirmed absent must hang at
   needs_confirmation (RiskConfirm face) with zero points written ->
   confirmed set_points writes three in-range points (written == 3) ->
   automation.get_curve reads back point_count == 3 with per-point t/v/c
   match -> descriptor envelope_point_count follows (3) ->
   automation.clear_region removes all three -> curve and descriptor back
   to 0.
B. track-level volume ride leg (agent invoke, the card's mandatory ride
   form): the track's built-in Volume & Pan plugin answers get_plugin_parameters
   with exactly the volume/pan rows; volume ride points (0.2 -> 0.5 -> 0.8,
   range [0,1] per card 201) go through the same write -> read-back ->
   clear chain on the volume parameter.
C. undo leg (agent invoke): set_points writes three points, project.undo
   rolls the write back, get_curve must report zero points again.
D. negative leg (agent invoke): set_points with one out-of-range value must
   reject the whole command (fail-closed) — the verbatim rejection receipt is
   kept in the summary artifact — and the curve must stay unchanged (zero
   points written).

Exit code 0 only if every leg passes; summary JSON artifact at --output.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


DISCOVERY_QUERIES = ["Supermassive", "Timeless", "Pro-R", "H-Delay", "Reverb", "Delay"]


def post_json(base_url: str, path: str, payload: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        # Fail-closed command rejections ride HTTP 400 with a JSON body; the
        # caller asserts on that body, so surface it instead of raising.
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {body[:800]}") from exc


def invoke(base_url: str, tool: str, args: Optional[Dict[str, Any]] = None, *, confirmed: bool = True, timeout: float = 120.0) -> Dict[str, Any]:
    """Full response (not just result) — the needs_confirmation and rejected
    shapes are themselves assertion targets."""
    payload: Dict[str, Any] = {"tool": tool, "args": args or {}}
    if confirmed:
        payload["confirmed"] = True
    return post_json(base_url, "/agent/invoke", payload, timeout)


def invoke_ok(base_url: str, tool: str, args: Optional[Dict[str, Any]] = None, *, timeout: float = 120.0) -> Dict[str, Any]:
    reply = invoke(base_url, tool, args, confirmed=True, timeout=timeout)
    if reply.get("status") != "ok":
        raise RuntimeError(f"{tool} failed: {json.dumps(reply, ensure_ascii=False)[:1200]}")
    return reply.get("result") or {}


def zmq_command(url: str, payload: Dict[str, Any], timeout_ms: int = 60000) -> Dict[str, Any]:
    context = zmq.Context.instance()
    sock = context.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    try:
        sock.connect(url)
        sock.send_json(payload)
        return json.loads(sock.recv_string())
    finally:
        sock.close(0)


def wait_agent_http(base_url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(base_url.rstrip("/") + "/health", timeout=2.0) as response:
                health = json.loads(response.read().decode("utf-8"))
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"agent HTTP did not become ready at {base_url}: {last_error}")


def wait_kernel_zmq(url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            reply = zmq_command(url, {"cmd": "ping"}, timeout_ms=5000)
            if str(reply.get("status", "")).lower() in {"ok", "success"}:
                return
            last_error = RuntimeError(f"ping replied {reply.get('status')!r}")
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def approx(actual: Any, expected: float, tolerance: float = 1e-6) -> bool:
    try:
        return abs(float(actual) - expected) <= tolerance
    except (TypeError, ValueError):
        return False


def parameter_rows(reply_result: Dict[str, Any]) -> List[Dict[str, Any]]:
    rows = reply_result.get("parameters")
    require(isinstance(rows, list) and bool(rows), f"get_plugin_parameters returned no parameters: {json.dumps(reply_result, ensure_ascii=False)[:600]}")
    return [row for row in rows if isinstance(row, dict)]


def kernel_parameters(kernel_zmq: str, track_id: str, plugin_id: str) -> List[Dict[str, Any]]:
    """Kernel-face parameter rows: the raw value-range authority (top-level
    id/min/max/envelope_point_count — the 201 descriptor face). The agent
    /agent/invoke projection answers param_id + display_domain_candidate and
    carries no envelope_point_count column; both faces are recorded, raw
    domains and descriptor counts are read from the kernel face."""
    reply = zmq_command(kernel_zmq, {"cmd": "get_plugin_parameters", "track_id": track_id, "plugin_id": plugin_id, "command_timeout_ms": 60000})
    require(str(reply.get("status", "")).lower() in {"ok", "success"},
            f"kernel get_plugin_parameters failed: {json.dumps(reply, ensure_ascii=False)[:500]}")
    return parameter_rows(reply)


def kernel_row(rows: List[Dict[str, Any]], param_id: str) -> Dict[str, Any]:
    for row in rows:
        if str(row.get("id") or "").strip().lower() == param_id.lower():
            return row
    raise RuntimeError(f"kernel parameter row {param_id!r} not found among {[row.get('id') for row in rows]}")


def kernel_envelope_count(kernel_zmq: str, track_id: str, plugin_id: str, param_id: str) -> int:
    row = kernel_row(kernel_parameters(kernel_zmq, track_id, plugin_id), param_id)
    raw = row.get("envelope_point_count")
    require(isinstance(raw, int) and not isinstance(raw, bool), f"envelope_point_count must be an int, got {raw!r}")
    return raw


def numeric_value_range(row: Dict[str, Any]) -> Optional[List[float]]:
    """Kernel-face row value range as [min, max]; None when the row carries no
    numeric min/max pair (non-automation targets are skipped by the caller)."""
    values: List[float] = []
    for key in ("min", "max"):
        raw = row.get(key)
        if isinstance(raw, bool) or not isinstance(raw, (int, float)):
            return None
        values.append(float(raw))
    return values if values[0] < values[1] else None


def pick_automation_target(rows: List[Dict[str, Any]]) -> Dict[str, Any]:
    """First kernel-face row with a numeric value range — same spirit as the
    190 probe: the range is read from the live descriptor, never invented."""
    for index, row in enumerate(rows):
        value_range = numeric_value_range(row)
        if value_range and str(row.get("id") or "").strip():
            return {"index": index, "param_id": str(row["id"]).strip(), "value_range": value_range}
    raise RuntimeError(f"no kernel parameter row carries a numeric value range: {json.dumps(rows[:3], ensure_ascii=False)[:600]}")


def points_inside(value_range: List[float]) -> List[Dict[str, Any]]:
    low, high = value_range
    span = high - low
    return [
        {"t": 0.0, "v": low + 0.8 * span, "c": 0.0},
        {"t": 1.5, "v": low + 0.5 * span, "c": 0.25},
        {"t": 3.0, "v": low + 0.2 * span, "c": 0.0},
    ]


def ride_points(low: float, high: float) -> List[Dict[str, Any]]:
    span = high - low
    return [
        {"t": 0.0, "v": low + 0.2 * span, "c": 0.0},
        {"t": 2.0, "v": low + 0.5 * span, "c": 0.0},
        {"t": 4.0, "v": low + 0.8 * span, "c": 0.0},
    ]


def assert_curve_points(label: str, curve: Dict[str, Any], expected: List[Dict[str, Any]]) -> None:
    points = curve.get("points")
    require(isinstance(points, list) and len(points) == len(expected),
            f"{label}: point list mismatch, got {points!r}")
    for point, want in zip(points, expected):
        require(isinstance(point, dict), f"{label}: non-object point: {point!r}")
        require(approx(point.get("t"), want["t"]) and approx(point.get("v"), want["v"]) and approx(point.get("c"), want["c"]),
                f"{label}: point mismatch got {point!r} want {want!r}")


def curve_point_count(base_url: str, plugin_id: str, param_id: str) -> int:
    curve = invoke_ok(base_url, "automation.get_curve", {"plugin_id": plugin_id, "param_id": param_id})
    raw = curve.get("point_count")
    require(isinstance(raw, int) and not isinstance(raw, bool), f"get_curve point_count must be an int, got {raw!r}")
    require(isinstance(curve.get("points"), list) and len(curve["points"]) == raw,
            f"get_curve points/point_count disagree: {json.dumps(curve, ensure_ascii=False)[:400]}")
    return raw


def envelope_count(kernel_zmq: str, track_id: str, plugin_id: str, param_id: str) -> int:
    """Descriptor envelope_point_count — kernel face (the agent invoke
    projection carries no envelope_point_count column; recorded boundary)."""
    return kernel_envelope_count(kernel_zmq, track_id, plugin_id, param_id)


def get_parameters_with_rows(base_url: str, track_id: str, plugin_id: str) -> Dict[str, Any]:
    """get_plugin_parameters over the agent invoke face with full parameter
    rows: without include_parameters=true the agent face returns a compact
    summary (display_probe_summary, no parameters array) — the routing
    guidance itself instructs include_parameters:true. The agent projection
    identifies rows by param_id and carries display_domain_candidate."""
    result = invoke_ok(base_url, "get_plugin_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "include_parameters": True,
    })
    parameter_rows(result)
    return result


def find_track_volume_plugin(state: Dict[str, Any], track_id: str) -> Optional[str]:
    """Same shape as automation_curve_smoke.find_track_volume_plugin (196
    precedent): the track's built-in Volume & Pan plugin row (name/type
    containing "volume"). Returns None when the state face carries no plugin
    rows for the track — the caller then falls back to the kernel state."""
    tracks = state.get("tracks") if isinstance(state.get("tracks"), list) else []
    for raw_track in tracks:
        if not isinstance(raw_track, dict):
            continue
        candidate = str(raw_track.get("track_id") or raw_track.get("id") or raw_track.get("item_id") or "").strip()
        if candidate != track_id:
            continue
        plugins = raw_track.get("plugins")
        if not isinstance(plugins, list):
            return None
        for raw_plugin in plugins:
            if not isinstance(raw_plugin, dict):
                continue
            kind = (str(raw_plugin.get("name") or "") + "/" + str(raw_plugin.get("type") or "")).lower()
            if "volume" not in kind:
                continue
            plugin_id = str(raw_plugin.get("id") or raw_plugin.get("plugin_item_id") or raw_plugin.get("plugin_id") or "").strip()
            if plugin_id:
                return plugin_id
        return None
    return None


def resolve_ride_plugin_id(agent_http: str, kernel_zmq: str, ride_track_id: str) -> str:
    """Resolve the ride track's built-in Volume & Pan plugin id. The agent
    project.state face is tried first; the kernel get_project_state face is
    the fallback (the agent state projection may sanitize plugin rows)."""
    state = invoke_ok(agent_http, "project.state", {})
    plugin_id = find_track_volume_plugin(state, ride_track_id)
    if plugin_id:
        return plugin_id
    kernel_state = zmq_command(kernel_zmq, {"cmd": "get_project_state"})
    require(str(kernel_state.get("status", "")).lower() in {"ok", "success"},
            f"kernel get_project_state failed: {json.dumps(kernel_state, ensure_ascii=False)[:400]}")
    plugin_id = find_track_volume_plugin(kernel_state, ride_track_id)
    require(bool(plugin_id),
            "no built-in Volume & Pan plugin found on track "
            f"{ride_track_id} (agent state plugins: {json.dumps(state.get('tracks'), ensure_ascii=False)[:200]})")
    return plugin_id


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-sec", type=int, default=240)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--plugin-query", action="append", default=[])
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    else:
        project_path = Path(tempfile_dir()) / f"automation_agent_chain_{int(time.time())}.vit"

    legs: List[str] = []
    summary: Dict[str, Any] = {
        "scenario": "M3-C4-AUTO-DISCLOSE-1 agent-side automation chain",
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
        "project_path": str(project_path),
        "started_at": datetime.now(timezone.utc).isoformat(),
        "legs": legs,
    }

    def record(name: str, detail: Dict[str, Any]) -> None:
        legs.append(name)
        summary[name] = detail
        print(f"[ok] {name}: {json.dumps(detail, ensure_ascii=False)[:400]}")

    try:
        wait_agent_http(args.agent_http, args.timeout_sec)
        wait_kernel_zmq(args.kernel_zmq, args.timeout_sec)

        # ---- setup (kernel ZMQ face: project + tracks + external plugin) --
        project_path.parent.mkdir(parents=True, exist_ok=True)
        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"save_as_project failed: {reply}")

        queries = args.plugin_query or DISCOVERY_QUERIES
        plugin = None
        for query in queries:
            reply = zmq_command(args.kernel_zmq, {"cmd": "plugin_search", "query": query, "limit": 8, "command_timeout_ms": 60000})
            batch = []
            for key in ("plugins", "entries", "results"):
                if isinstance(reply.get(key), list):
                    batch = [row for row in reply[key] if isinstance(row, dict)]
                    break
            for row in batch:
                identifier = str(row.get("identifier") or row.get("plugin_identifier") or "").strip()
                path = str(row.get("file_or_identifier") or row.get("plugin_path") or row.get("path") or "").strip()
                name = str(row.get("name") or row.get("plugin_name") or "").strip()
                if identifier and path and name:
                    plugin = {"query": query, "name": name, "identifier": identifier, "path": path}
                    break
            if plugin:
                break
        require(plugin is not None, f"plugin_search found no loadable external plugin for queries: {queries}")

        track_ids: List[str] = []
        for _ in range(2):
            reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
            track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
            require(track_id, f"add_audio_track returned no track_id: {reply}")
            track_ids.append(track_id)
        fx_track_id, ride_track_id = track_ids[0], track_ids[1]

        reply = zmq_command(args.kernel_zmq, {
            "cmd": "rack_add_node",
            "track_id": fx_track_id,
            "plugin_path": plugin["path"],
            "plugin_identifier": plugin["identifier"],
            "auto_connect": True,
            "x": 0.32,
            "y": 0.32,
            "command_timeout_ms": 60000,
        })
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"rack_add_node failed: {reply}")
        external_plugin_id = str(reply.get("plugin_id") or reply.get("plugin_item_id") or reply.get("id") or "").strip()
        require(external_plugin_id, f"rack_add_node returned no plugin id: {reply}")

        ride_plugin_id = resolve_ride_plugin_id(args.agent_http, args.kernel_zmq, ride_track_id)
        record("setup", {
            "plugin": plugin["name"],
            "external_plugin_id": external_plugin_id,
            "fx_track_id": fx_track_id,
            "ride_track_id": ride_track_id,
            "ride_volume_plugin_id": ride_plugin_id,
        })

        # ---- LEG A: external plugin parameter chain over /agent/invoke ----
        # Agent-face projection (param_id/display_domain_candidate) is read
        # over /agent/invoke as part of the chain; the raw value range the
        # write validates against is read from the kernel face.
        agent_params = get_parameters_with_rows(args.agent_http, fx_track_id, external_plugin_id)
        agent_param_ids = sorted(str(row.get("param_id") or "").strip() for row in parameter_rows(agent_params) if str(row.get("param_id") or "").strip())
        kernel_rows = kernel_parameters(args.kernel_zmq, fx_track_id, external_plugin_id)
        target = pick_automation_target(kernel_rows)
        require(target["param_id"] in [pid.lower() for pid in agent_param_ids],
                f"LEG A target {target['param_id']!r} not visible in the agent parameter projection: {agent_param_ids}")
        require(envelope_count(args.kernel_zmq, fx_track_id, external_plugin_id, target["param_id"]) == 0,
                "LEG A baseline: target envelope_point_count must start at 0")

        pending = invoke(args.agent_http, "automation.set_points", {
            "plugin_id": external_plugin_id,
            "param_id": target["param_id"],
            "points": points_inside(target["value_range"]),
        }, confirmed=False)
        require(str(pending.get("status", "")).lower() == "needs_confirmation",
                f"LEG A unconfirmed set_points must hang at needs_confirmation, got: {json.dumps(pending, ensure_ascii=False)[:800]}")
        require(bool(pending.get("requires_confirmation")), "LEG A pending reply must set requires_confirmation=true")
        require(str(pending.get("preview") or "").strip() != "", "LEG A pending reply must carry a preview")
        require(curve_point_count(args.agent_http, external_plugin_id, target["param_id"]) == 0,
                "LEG A unconfirmed set_points must not write any point")

        written = invoke_ok(args.agent_http, "automation.set_points", {
            "plugin_id": external_plugin_id,
            "param_id": target["param_id"],
            "points": points_inside(target["value_range"]),
        })
        require(int(written.get("written", -1)) == 3, f"LEG A written must be 3: {written}")

        curve = invoke_ok(args.agent_http, "automation.get_curve", {"plugin_id": external_plugin_id, "param_id": target["param_id"]})
        require(int(curve.get("point_count", -1)) == 3, f"LEG A get_curve point_count must be 3: {curve}")
        assert_curve_points("LEG A readback", curve, points_inside(target["value_range"]))
        require(envelope_count(args.kernel_zmq, fx_track_id, external_plugin_id, target["param_id"]) == 3,
                "LEG A descriptor envelope_point_count must follow the write (3)")

        cleared = invoke_ok(args.agent_http, "automation.clear_region", {
            "plugin_id": external_plugin_id,
            "param_id": target["param_id"],
            "region": {"start": 0.0, "end": 5.0},
        })
        require(int(cleared.get("removed_points", -1)) == 3, f"LEG A removed_points must be 3: {cleared}")
        require(curve_point_count(args.agent_http, external_plugin_id, target["param_id"]) == 0,
                "LEG A curve must be empty after clear_region")
        require(envelope_count(args.kernel_zmq, fx_track_id, external_plugin_id, target["param_id"]) == 0,
                "LEG A descriptor envelope_point_count must return to 0")
        record("leg_a_external_plugin_param_chain", {
            "param_id": target["param_id"],
            "raw_value_range_kernel_face": target["value_range"],
            "agent_projection_param_count": len(agent_param_ids),
            "agent_projection_param_ids_sample": agent_param_ids[:8],
            "checks": [
                "agent_invoke_get_plugin_parameters_rows_visible",
                "value_range_read_via_kernel_face_get_plugin_parameters",
                "unconfirmed_set_points_needs_confirmation_zero_write",
                "confirmed_set_points_written_3",
                "get_curve_readback_pointwise_match",
                "descriptor_envelope_count_follows_3",
                "clear_region_removed_3_curve_and_descriptor_zero",
            ],
        })

        # ---- LEG B: track-level volume ride chain over /agent/invoke ------
        # Exactly volume/pan rows with raw range [0,1]/[-1,1] is asserted on
        # the kernel face (the 201 descriptor mirror); the agent projection
        # must show the same two parameters by param_id.
        ride_params = get_parameters_with_rows(args.agent_http, ride_track_id, ride_plugin_id)
        agent_ride_ids = sorted(str(row.get("param_id") or "").strip().lower() for row in parameter_rows(ride_params) if str(row.get("param_id") or "").strip())
        require(agent_ride_ids == ["pan", "volume"], f"LEG B agent projection must show exactly volume/pan params, got: {agent_ride_ids}")
        ride_rows = kernel_parameters(args.kernel_zmq, ride_track_id, ride_plugin_id)
        ride_ids = sorted(str(row.get("id") or "").strip().lower() for row in ride_rows)
        require(ride_ids == ["pan", "volume"], f"LEG B track volume plugin must answer exactly volume/pan rows, got: {ride_ids}")
        volume_row = kernel_row(ride_rows, "volume")
        volume_range = numeric_value_range(volume_row)
        require(volume_range is not None and volume_range == [0.0, 1.0],
                f"LEG B volume row value range must be [0, 1] (card 201), got {volume_range}")
        pan_range = numeric_value_range(kernel_row(ride_rows, "pan"))
        require(pan_range is not None and pan_range == [-1.0, 1.0],
                f"LEG B pan row value range must be [-1, 1] (card 201), got {pan_range}")

        ride = ride_points(*volume_range)
        written = invoke_ok(args.agent_http, "automation.set_points", {
            "track_id": ride_track_id,
            "plugin_id": ride_plugin_id,
            "param_id": "volume",
            "points": ride,
        })
        require(int(written.get("written", -1)) == 3, f"LEG B written must be 3: {written}")
        curve = invoke_ok(args.agent_http, "automation.get_curve", {"plugin_id": ride_plugin_id, "param_id": "volume"})
        require(int(curve.get("point_count", -1)) == 3, f"LEG B get_curve point_count must be 3: {curve}")
        assert_curve_points("LEG B readback", curve, ride)
        require(envelope_count(args.kernel_zmq, ride_track_id, ride_plugin_id, "volume") == 3,
                "LEG B descriptor envelope_point_count must follow the ride write (3)")

        cleared = invoke_ok(args.agent_http, "automation.clear_region", {
            "plugin_id": ride_plugin_id,
            "param_id": "volume",
            "region": {"start": 0.0, "end": 5.0},
        })
        require(int(cleared.get("removed_points", -1)) == 3, f"LEG B removed_points must be 3: {cleared}")
        require(curve_point_count(args.agent_http, ride_plugin_id, "volume") == 0,
                "LEG B curve must be empty after clear_region")
        require(envelope_count(args.kernel_zmq, ride_track_id, ride_plugin_id, "volume") == 0,
                "LEG B descriptor envelope_point_count must return to 0")
        record("leg_b_track_volume_ride_chain", {
            "plugin_id": ride_plugin_id,
            "value_range": volume_range,
            "checks": [
                "volume_pan_rows_exactly",
                "volume_range_0_1",
                "ride_set_points_written_3",
                "get_curve_readback_pointwise_match",
                "descriptor_envelope_count_follows_3",
                "clear_region_removed_3_curve_and_descriptor_zero",
            ],
        })

        # ---- LEG C: undo leg over /agent/invoke ----------------------------
        undo_points = ride_points(*volume_range)
        written = invoke_ok(args.agent_http, "automation.set_points", {
            "plugin_id": ride_plugin_id,
            "param_id": "volume",
            "points": undo_points,
        })
        require(int(written.get("written", -1)) == 3, f"LEG C written must be 3: {written}")
        require(curve_point_count(args.agent_http, ride_plugin_id, "volume") == 3,
                "LEG C curve must hold 3 points before undo")
        undo = invoke_ok(args.agent_http, "project.undo", {})
        require(curve_point_count(args.agent_http, ride_plugin_id, "volume") == 0,
                f"LEG C get_curve must report 0 points after project.undo (undo result: {json.dumps(undo, ensure_ascii=False)[:300]})")
        record("leg_c_undo", {
            "undo_result": {key: undo[key] for key in ("status", "undone", "undo_label", "message") if key in undo},
            "checks": ["set_points_3_before_undo", "project_undo_ok", "get_curve_zero_after_undo"],
        })

        # ---- LEG D: negative leg (fail-closed whole-command rejection) -----
        low, high = volume_range
        negative_points = [
            {"t": 0.0, "v": low + 0.5 * (high - low), "c": 0.0},
            {"t": 1.0, "v": high + 1.0, "c": 0.0},
        ]
        rejected = invoke(args.agent_http, "automation.set_points", {
            "plugin_id": ride_plugin_id,
            "param_id": "volume",
            "points": negative_points,
        })
        rejected_status = str(rejected.get("status", "")).lower()
        require(rejected_status in {"error", "kernel_error"},
                f"LEG D out-of-range set_points must reject the whole command, got: {json.dumps(rejected, ensure_ascii=False)[:800]}")
        rejection_text = str(rejected.get("error") or "")
        if not rejection_text.strip():
            result = rejected.get("result")
            if isinstance(result, dict):
                ack = result.get("command_ack")
                if isinstance(ack, dict):
                    err = ack.get("error")
                    if isinstance(err, dict):
                        rejection_text = str(err.get("message") or "")
                    rejection_text = rejection_text or str(ack.get("message") or "")
        require(rejection_text.strip() != "", "LEG D rejection receipt must carry an error message")
        require("outside the parameter value range" in rejection_text,
                f"LEG D rejection receipt must disclose the value-range violation: {rejection_text[:300]}")
        require(curve_point_count(args.agent_http, ride_plugin_id, "volume") == 0,
                "LEG D rejected command must leave the curve unchanged (zero writes)")
        record("leg_d_fail_closed_out_of_range", {
            "rejection_receipt_verbatim": {"status": rejected.get("status"), "error": rejection_text,
                                           "command_name": rejected.get("command_name"), "risk_level": rejected.get("risk_level")},
            "checks": ["whole_command_rejected", "rejection_receipt_in_artifact", "curve_unchanged_zero_write"],
        })

        summary["status"] = "ok"
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print("automation agent chain smoke: PASS")
        for leg in legs:
            print(f"  - {leg}")
        return 0
    except Exception as exc:  # noqa: BLE001 - top-level smoke failure reporting.
        summary["status"] = "failed"
        summary["error"] = str(exc)
        print(f"automation agent chain smoke: FAIL: {exc}", file=sys.stderr)
        return 1
    finally:
        output_path = Path(args.output).resolve()
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"summary written: {output_path}")


def tempfile_dir() -> Path:
    import tempfile
    path = Path(tempfile.gettempdir()) / "vit_daw_automation_agent_chain_smoke"
    path.mkdir(parents=True, exist_ok=True)
    return path


if __name__ == "__main__":
    sys.exit(main())
