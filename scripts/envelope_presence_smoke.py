#!/usr/bin/env python3
"""Smoke-test the envelope_point_count parameter descriptor column (M3-ENVELOPE-PRESENCE-1).

Legs (against the live kernel ZMQ REQ face, same pattern as
automation_curve_smoke.py; plugin discovery/load follows the pgtdc probe
precedent: kernel rack_add_node, never the PCA-gated agent load tool):

A.  Load one external plugin onto a disposable track, then take two
    get_plugin_parameters snapshots S0/S1 with no envelope written: every
    parameter row must carry envelope_point_count == 0 and the two
    snapshots must be fully identical (query stability baseline).
B.  automation.set_points writes three flat points (v == the parameter's
    current value, so the curve cannot move the value fields) on one
    parameter; get_plugin_parameters must then report envelope_point_count
    == 3 on that row and 0 on every untouched row.
C.  Diff-level existing-field zero-change assertion: snapshot S2 (after the
    write) may differ from S1 only in the target row's envelope_point_count
    and -- disclosed, never silently passed -- the curve-driven value keys
    (value/normalized_value/value_text) of the target row. Any other drift
    (other rows, top-level fields) fails the run.
D.  automation.clear_region removes the region; the final snapshot must
    report envelope_point_count == 0 everywhere and match S0 (return to the
    no-envelope baseline; value-field drift is disclosed like leg C).
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


DISCOVERY_QUERIES = [
    "Supermassive",
    "Timeless",
    "Pro-R",
    "H-Delay",
    "Reverb",
    "Delay",
]

VALUE_KEYS = ("value", "normalized_value", "value_text")

TOP_LEVEL_KEYS = (
    "status",
    "track_id",
    "plugin_id",
    "plugin_item_id",
    "template_role",
    "supports_param_grabber",
    "current_param_signature_hash",
    "binding_targets",
    "recommended_groups",
    "quick_controls",
    "control_shell",
    "capability_manifest",
)


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


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_ok(label: str, reply: dict[str, Any]) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status in {"ok", "success", "completed"}, f"{label} failed: {json.dumps(reply, ensure_ascii=False)[:1500]}")
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


def first_text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if isinstance(value, str) and value.strip():
            return value.strip()
    return ""


def discover_plugin(sock: zmq.Socket, queries: list[str], timeout_ms: int) -> dict[str, str]:
    for query in queries:
        reply = require_ok("plugin_search", send_command(sock, {"cmd": "plugin_search", "query": query, "limit": 8, "command_timeout_ms": timeout_ms}))
        batch: list[dict[str, Any]] = []
        for key in ("plugins", "entries", "results"):
            if isinstance(reply.get(key), list):
                batch = [row for row in reply[key] if isinstance(row, dict)]
                break
        for row in batch:
            identifier = first_text(row, "identifier", "plugin_identifier")
            path = first_text(row, "file_or_identifier", "plugin_path", "path")
            name = first_text(row, "name", "plugin_name")
            if identifier and path and name:
                return {"query": query, "name": name, "identifier": identifier, "path": path}
    raise RuntimeError(f"plugin_search found no loadable external plugin for queries: {queries}")


def get_parameters(sock: zmq.Socket, track_id: str, plugin_id: str, timeout_ms: int) -> dict[str, Any]:
    reply = require_ok(
        "get_plugin_parameters",
        send_command(sock, {"cmd": "get_plugin_parameters", "track_id": track_id, "plugin_id": plugin_id, "command_timeout_ms": timeout_ms}),
    )
    rows = reply.get("parameters")
    require(isinstance(rows, list) and rows, f"get_plugin_parameters returned no parameters: {json.dumps(reply)[:600]}")
    return reply


def wait_parameters_ready(sock: zmq.Socket, track_id: str, plugin_id: str, timeout_s: float, timeout_ms: int) -> dict[str, Any]:
    deadline = time.time() + timeout_s
    last_reply: dict[str, Any] = {}
    while time.time() < deadline:
        try:
            return get_parameters(sock, track_id, plugin_id, timeout_ms)
        except RuntimeError as exc:
            last_reply = {"error": str(exc)}
            time.sleep(0.8)
    raise RuntimeError(f"external plugin parameters never became ready: {json.dumps(last_reply)[:400]}")


def row_key(row: dict[str, Any]) -> str:
    return first_text(row, "raw_param_id", "param_id", "id")


def strip_envelope_column(reply: dict[str, Any]) -> dict[str, Any]:
    stripped = {key: value for key, value in reply.items() if key != "parameters"}
    rows = []
    for row in reply.get("parameters", []):
        if isinstance(row, dict):
            rows.append({key: value for key, value in row.items() if key != "envelope_point_count"})
        else:
            rows.append(row)
    stripped["parameters"] = rows
    return stripped


def assert_column(rows: list[dict[str, Any]], expect: dict[str, int], summary: dict[str, Any]) -> None:
    seen: dict[str, int] = {}
    for index, row in enumerate(rows):
        require(isinstance(row, dict), f"parameter row #{index} is not an object: {row!r}")
        require("envelope_point_count" in row, f"parameter row #{index} carries no envelope_point_count key: {sorted(row)}")
        raw = row.get("envelope_point_count")
        require(isinstance(raw, int) and not isinstance(raw, bool), f"envelope_point_count must be an int, got {raw!r}")
        seen[str(row_key(row))] = raw
    for param, want in expect.items():
        require(seen.get(param) == want, f"envelope_point_count for {param}: got {seen.get(param)!r} want {want} (all: {seen})")
    summary["envelope_counts"] = seen


def diff_replies(before: dict[str, Any], after: dict[str, Any], allowed_row: str, allowed_keys: set[str], summary: dict[str, Any]) -> None:
    """Fail unless `after` differs from `before` only inside allowed_row / allowed_keys."""
    for key in TOP_LEVEL_KEYS:
        require(before.get(key) == after.get(key), f"existing top-level field drifted: {key}")
    before_rows = {row_key(row): row for row in before.get("parameters", []) if isinstance(row, dict)}
    after_rows = {row_key(row): row for row in after.get("parameters", []) if isinstance(row, dict)}
    require(set(before_rows) == set(after_rows), "parameter id set drifted")
    drift: dict[str, dict[str, Any]] = {}
    for param in sorted(before_rows):
        old_row = before_rows[param]
        new_row = after_rows[param]
        changed = {key: {"before": old_row.get(key), "after": new_row.get(key)}
                   for key in sorted(set(old_row) | set(new_row))
                   if old_row.get(key) != new_row.get(key)}
        if not changed:
            continue
        if param == allowed_row:
            outside = set(changed) - allowed_keys
            require(not outside, f"target row drifted outside the allowed keys {sorted(allowed_keys)}: {sorted(outside)}")
            drift[param] = changed
        else:
            raise RuntimeError(f"untouched row {param} drifted: {changed}")
    if drift:
        summary.setdefault("disclosed_curve_driven_drift", {})[str(after.get("plugin_id", ""))] = drift
        print(f"  disclosed curve-driven drift on target row: {json.dumps(drift, ensure_ascii=False)}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=30000)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", default="")
    parser.add_argument("--param-index", type=int, default=0)
    parser.add_argument("--plugin-query", action="append", default=[])
    args = parser.parse_args()

    if args.project_path:
        project_path = Path(args.project_path).resolve()
    else:
        temp_dir = Path(tempfile.gettempdir()) / "vit_daw_envelope_presence_smoke"
        temp_dir.mkdir(parents=True, exist_ok=True)
        project_path = temp_dir / f"envelope_presence_{int(time.time())}.vit"

    command_timeout_ms = max(args.timeout_ms, 30000)
    queries = args.plugin_query or DISCOVERY_QUERIES

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

        plugin = discover_plugin(sock, queries, command_timeout_ms)
        summary["plugin"] = plugin
        summary["checks"].append(f"plugin_search:{plugin['name']}")

        track_reply = require_ok("add_audio_track", send_command(sock, {"cmd": "add_audio_track"}))
        track_id = first_text(track_reply, "track_id", "id")
        require(track_id, f"add_audio_track did not return track_id: {track_reply}")
        summary["track_id"] = track_id
        summary["checks"].append("add_audio_track")

        node = require_ok(
            "rack_add_node",
            send_command(
                sock,
                {
                    "cmd": "rack_add_node",
                    "track_id": track_id,
                    "plugin_path": plugin["path"],
                    "plugin_identifier": plugin["identifier"],
                    "auto_connect": True,
                    "x": 0.32,
                    "y": 0.32,
                    "command_timeout_ms": command_timeout_ms,
                },
            ),
        )
        plugin_id = first_text(node, "plugin_id", "plugin_item_id", "id")
        require(plugin_id, f"rack_add_node returned no plugin id: {str(node)[:300]}")
        summary["plugin_id"] = plugin_id
        summary["checks"].append("rack_add_node")

        # LEG A: no-envelope baseline, twice.
        snapshot0 = wait_parameters_ready(sock, track_id, plugin_id, 60.0, command_timeout_ms)
        rows0 = snapshot0.get("parameters", [])
        assert_column(rows0, {row_key(row): 0 for row in rows0 if isinstance(row, dict)}, summary)
        summary["checks"].append("baseline.envelope_point_count.all_zero")
        snapshot1 = get_parameters(sock, track_id, plugin_id, command_timeout_ms)
        require(strip_envelope_column(snapshot0) == strip_envelope_column(snapshot1),
                "two no-envelope get_plugin_parameters snapshots differ (query instability)")
        summary["checks"].append("baseline.snapshot_stability")

        require(0 <= args.param_index < len(rows0), f"--param-index {args.param_index} out of range (rows={len(rows0)})")
        target_row = rows0[args.param_index]
        target_param = row_key(target_row)
        require(target_param, f"target row carries no param id: {target_row}")
        summary["target_param"] = target_param
        target_value = target_row.get("value")
        require(isinstance(target_value, (int, float)) and not isinstance(target_value, bool),
                f"target row carries no numeric current value: {target_value!r}")
        summary["target_value"] = target_value

        # LEG B: three flat points at the parameter's current value.
        points = [
            {"t": 0.0, "v": float(target_value), "c": 0.0},
            {"t": 1.5, "v": float(target_value), "c": 0.0},
            {"t": 3.0, "v": float(target_value), "c": 0.0},
        ]
        written = require_ok(
            "automation.set_points",
            send_command(
                sock,
                {"cmd": "automation.set_points", "plugin_id": plugin_id, "param_id": target_param,
                 "points": points, "command_timeout_ms": command_timeout_ms},
            ),
        )
        require(int(written.get("written", -1)) == 3, f"written must be 3: {written}")
        summary["checks"].append("automation.set_points.write3_flat")

        snapshot2 = get_parameters(sock, track_id, plugin_id, command_timeout_ms)
        rows2 = snapshot2.get("parameters", [])
        assert_column(rows2, {row_key(row): 0 for row in rows2 if isinstance(row, dict)} | {target_param: 3}, summary)
        summary["checks"].append("envelope_point_count.target_is_3")

        # LEG C: existing fields unchanged apart from the target row envelope
        # column (curve-driven value drift on that row is disclosed, not
        # silently swallowed).
        diff_replies(snapshot1, snapshot2, target_param, {"envelope_point_count", *VALUE_KEYS}, summary)
        summary["checks"].append("existing_fields.zero_change.diff_level")

        # LEG D: clear the region, return to the no-envelope baseline.
        cleared = require_ok(
            "automation.clear_region",
            send_command(
                sock,
                {"cmd": "automation.clear_region", "plugin_id": plugin_id, "param_id": target_param,
                 "region": {"start": 0.0, "end": 5.0}, "command_timeout_ms": command_timeout_ms},
            ),
        )
        require(int(cleared.get("removed_points", -1)) == 3, f"removed_points must be 3: {cleared}")
        snapshot3 = get_parameters(sock, track_id, plugin_id, command_timeout_ms)
        rows3 = snapshot3.get("parameters", [])
        assert_column(rows3, {row_key(row): 0 for row in rows3 if isinstance(row, dict)}, summary)
        summary["checks"].append("envelope_point_count.after_clear_zero")
        diff_replies(snapshot0, snapshot3, target_param, set(VALUE_KEYS), summary)
        summary["checks"].append("after_clear.matches_baseline")

        summary["status"] = "ok"
        print("envelope presence smoke: PASS")
        for check in summary["checks"]:
            print(f"  - {check}")
        return 0
    except Exception as exc:  # noqa: BLE001 - top-level smoke failure reporting.
        summary["status"] = "failed"
        summary["error"] = str(exc)
        print(f"envelope presence smoke: FAIL: {exc}", file=sys.stderr)
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
