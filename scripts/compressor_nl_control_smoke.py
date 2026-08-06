#!/usr/bin/env python3
"""Validate explicit compressor control through the real Vit chat path."""
from __future__ import annotations

import argparse
import json
import math
import time
import urllib.parse
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


DEFAULT_MESSAGE = "把 Threshold 设置为 -12 dB，Ratio 设置为 4:1"
EXPECTED_ROUTE = [
    "plugin_grabber.inspect_compressor",
    "plugin_grabber.apply_compressor_controls",
]
FORBIDDEN_TOOLS = {
    "plugin_grabber.explain_controls",
    "plugin.set_parameter",
    "plugin_set_parameter",
    "set_plugin_param",
    "daw.invoke",
    "goal.tick",
}


def find_case(config: dict[str, Any], case_id: str) -> dict[str, Any]:
    for case in config.get("cases") or []:
        if isinstance(case, dict) and matrix.first_text(case, "id") == case_id:
            return case
    raise RuntimeError(f"compressor matrix omitted case {case_id!r}")


def chat(base: str, conversation_id: str, message: str, track_id: str,
         plugin_id: str, plugin_name: str, timeout: float) -> dict[str, Any]:
    return matrix.request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id,
        "message": message,
        "context": {
            "agent_mode": "chat",
            "interaction_path": "agent_http_after_godot_project_lifecycle",
            "product_path_smoke": True,
            "product_lifecycle": "godot_project",
            "selected_plugin_track_id": track_id,
            "selected_track_id": track_id,
            "selected_plugin_id": plugin_id,
            "plugin_id": plugin_id,
            "selected_plugin_name": plugin_name,
        },
    }, timeout)


def conversation_events(base: str, conversation_id: str,
                        timeout: float) -> list[dict[str, Any]]:
    query = urllib.parse.urlencode({
        "conversation_id": conversation_id,
        "since": 0,
        "limit": 100,
    })
    response = matrix.request_json(
        "GET", base.rstrip("/") + "/agent/events?" + query, None, timeout)
    events = response.get("events")
    if not isinstance(events, list):
        raise RuntimeError("agent events response omitted events")
    return [event for event in events if isinstance(event, dict)]


def validate_readback(result: dict[str, Any]) -> dict[str, Any]:
    if matrix.first_text(result, "status").lower() != "exact":
        raise RuntimeError(f"typed apply status was not exact: {result.get('status')!r}")
    expected = {"threshold": -12.0, "ratio": 4.0}
    actual: dict[str, dict[str, Any]] = {}
    for control in result.get("controls") or []:
        if not isinstance(control, dict):
            continue
        role = matrix.first_text(control, "role")
        rows = [row for row in control.get("actual_readback") or []
                if isinstance(row, dict)]
        if role not in expected or not rows:
            continue
        physical = rows[0].get("physical")
        if not isinstance(physical, (int, float)) or not math.isclose(
                float(physical), expected[role], rel_tol=0, abs_tol=0.01):
            raise RuntimeError(
                f"{role} readback={physical!r}, expected={expected[role]!r}")
        actual[role] = {
            "physical": float(physical),
            "value_text": matrix.first_text(rows[0], "value_text"),
            "status": matrix.first_text(control, "status"),
        }
    if set(actual) != set(expected):
        raise RuntimeError(
            f"typed readback roles={sorted(actual)}, expected={sorted(expected)}")
    return actual


def validate_chat(response: dict[str, Any], events: list[dict[str, Any]]) -> dict[str, Any]:
    if matrix.first_text(response, "goal_status").lower() != "completed":
        raise RuntimeError(
            f"chat goal_status={response.get('goal_status')!r} "
            f"reply={response.get('reply')!r}")
    completed = [event for event in events
                 if event.get("type") == "item.completed"]
    route = [matrix.first_text(event.get("payload") or {}, "tool")
             for event in completed]
    if route != EXPECTED_ROUTE:
        raise RuntimeError(f"chat tool route={route!r}, expected={EXPECTED_ROUTE!r}")
    all_tools = {
        matrix.first_text(event.get("payload") or {}, "tool")
        for event in events if str(event.get("type", "")).startswith("item.")
    }
    forbidden = sorted(tool for tool in all_tools if tool in FORBIDDEN_TOOLS)
    if forbidden:
        raise RuntimeError(f"chat used forbidden tools: {forbidden}")
    apply_payload = completed[-1].get("payload") or {}
    apply_result = apply_payload.get("result")
    if not isinstance(apply_result, dict):
        raise RuntimeError("apply completion omitted typed result")
    readback = validate_readback(apply_result)
    reply = matrix.first_text(response, "reply")
    if "-12.00 dB" not in reply or "4.00:1" not in reply:
        raise RuntimeError(f"final reply omitted typed values: {reply!r}")
    return {
        "conversation_id": matrix.first_text(response, "conversation_id"),
        "goal_id": matrix.first_text(response, "goal_id"),
        "goal_status": matrix.first_text(response, "goal_status"),
        "stop_reason": matrix.first_text(response, "stop_reason"),
        "reply": reply,
        "tool_route": route,
        "forbidden_tools_present": forbidden,
        "actual_readback": readback,
    }


def run(base: str, case: dict[str, Any], message: str,
        conversation_id: str, timeout: float) -> dict[str, Any]:
    track_id = ""
    report: dict[str, Any] = {}
    try:
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {
            "name": "Compressor NL control smoke"}, timeout, True),
            "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        identifier, _ = matrix.resolve_identifier(case, timeout)
        plugin_name = matrix.first_text(case, "plugin_name")
        loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": plugin_name,
            "plugin_identifier": identifier,
        }, timeout, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        response = chat(base, conversation_id, message, track_id, plugin_id,
                        plugin_name, timeout)
        events = conversation_events(base, conversation_id, timeout)
        report = validate_chat(response, events)
        report.update({
            "schema_version": "plugin_grabber.compressor_nl_control_smoke.v1",
            "status": "ok",
            "message": message,
            "plugin_name": plugin_name,
            "track_id": track_id,
            "plugin_id": plugin_id,
        })
        return report
    finally:
        if track_id:
            deleted = matrix.invoke(
                base, "track.delete", {"track_id": track_id}, timeout, True)
            matrix.require_ok(deleted, "track.delete")
            report["temporary_track_deleted"] = True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default=str(
        Path(__file__).with_name("compressor_compat_matrix.json")))
    parser.add_argument("--case-id", default="pro_c_2")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--message", default=DEFAULT_MESSAGE)
    parser.add_argument("--conversation-id", default="")
    parser.add_argument("--output", default="")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    args = parser.parse_args()

    config = json.loads(Path(args.config).read_text(encoding="utf-8"))
    case = find_case(config, args.case_id)
    conversation_id = args.conversation_id or f"compressor_nl_smoke_{int(time.time())}"
    health = matrix.request_json(
        "GET", args.agent_http.rstrip("/") + "/health", None, 10)
    if matrix.first_text(health, "status").lower() not in {"ok", "ready"}:
        raise RuntimeError(f"agent health check failed: {health}")
    report = run(args.agent_http, case, args.message, conversation_id,
                 args.timeout_sec)
    rendered = json.dumps(report, ensure_ascii=False, indent=2)
    if args.output:
        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(rendered, encoding="utf-8")
        print(f"report: {output}")
    print(rendered)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
