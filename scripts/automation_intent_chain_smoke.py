#!/usr/bin/env python3
"""Natural-language automation intent chain smoke (M3-C4-AUTO-INTENT-1).

Same stack shape as automation_agent_chain_smoke.py (217): kernel + agent
berth via dev_agent_smoke.ps1 -StartKernel -RestartAgent, no UI, isolated
project (VIT_PROJECT_XML). The setup face uses the kernel ZMQ command socket
(project + tracks); every assertion leg drives the real agent /agent/chat
HTTP face — the deterministic pre-model intent surface this card certifies
for the first time. The whole chain is LLM-free: the choreography routing is
vocabulary-driven and takes over before any model turn, so no LLM config is
required or used (AGENTS §8 declaration: zero LLM participation).

Legs:

A. positive chain (agent chat): the time-varying phrase "0 到 30 秒，让这条
   轨的存在感慢慢强起来" must produce the confirmation card (workflow
   section_automation_choreo, waiting_confirmation, editable fields
   shape/start_value/end_value/curve/region_start/region_end, zero points
   written before confirmation) -> "确认写入" executes the governed
   envelope write (mutation_performed, written == 2) with per-point readback
   verified and the C4 ledger evidence (mixboard_decision_status recorded +
   workflow_queue_c4 in_progress) -> kernel get_curve confirms the exact
   default fade-in points (0.30 -> 0.60 over 0-30s).
B. readback entry (agent chat): "读回刚才那条线" answers with the live curve
   and the per-point consistency verdict.
C. negative boundary leg 1 (no time-varying word): "把这条轨的存在感调强一
   点" must NOT arm the choreography (no section_automation workflow, curve
   point count unchanged) — the static treatment path keeps the phrase.
D. negative boundary leg 2 (no region, no marker): "让这条轨的存在感慢慢强
   起来" must answer with the clarification ask (never a guess, zero writes).
E. category boundary leg (space class, no plugin on the track): "让这条轨的
   空间感慢慢铺进来" must answer with the honest no-plugin boundary (nothing
   fabricated, zero writes).

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

CHAT_TIMEOUT_SEC = 60.0


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
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {body[:800]}") from exc


def chat(base_url: str, conversation: str, message: str, context: Dict[str, Any]) -> Dict[str, Any]:
    return post_json(base_url, "/agent/chat", {
        "conversation_id": conversation,
        "message": message,
        "context": context,
    }, CHAT_TIMEOUT_SEC)


def invoke(base_url: str, tool: str, args: Optional[Dict[str, Any]] = None, *, confirmed: bool = True, timeout: float = 120.0) -> Dict[str, Any]:
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


def find_track_volume_plugin(state: Dict[str, Any], track_id: str) -> Optional[str]:
    """196/217 precedent: the track's built-in Volume & Pan plugin row."""
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


def kernel_curve(kernel_zmq: str, plugin_id: str, param_id: str) -> Dict[str, Any]:
    reply = zmq_command(kernel_zmq, {"cmd": "automation.get_curve", "plugin_id": plugin_id, "param_id": param_id, "command_timeout_ms": 60000})
    require(str(reply.get("status", "")).lower() in {"ok", "success"},
            f"kernel automation.get_curve failed: {json.dumps(reply, ensure_ascii=False)[:500]}")
    return reply


def kernel_point_count(kernel_zmq: str, plugin_id: str, param_id: str) -> int:
    curve = kernel_curve(kernel_zmq, plugin_id, param_id)
    points = curve.get("points")
    require(isinstance(points, list), f"kernel curve points not a list: {points!r}")
    return len(points)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-sec", type=int, default=240)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    import tempfile
    project_path = Path(args.project_path).resolve() if args.project_path else Path(tempfile.gettempdir()) / f"automation_intent_chain_{int(time.time())}.vit"

    legs: List[str] = []
    summary: Dict[str, Any] = {
        "scenario": "M3-C4-AUTO-INTENT-1 natural-language automation intent chain",
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
        "project_path": str(project_path),
        "llm_participation": "none (deterministic vocabulary routing takes over before any model turn)",
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

        # ---- setup (kernel ZMQ face: isolated project + two tracks) -------
        project_path.parent.mkdir(parents=True, exist_ok=True)
        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"save_as_project failed: {reply}")

        track_ids: List[str] = []
        for _ in range(2):
            reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
            track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
            require(track_id, f"add_audio_track returned no track_id: {reply}")
            track_ids.append(track_id)
        ride_track_id = track_ids[1]

        kernel_state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        ride_plugin_id = find_track_volume_plugin(kernel_state, ride_track_id)
        require(bool(ride_plugin_id),
                f"no built-in Volume & Pan plugin on track {ride_track_id}: {json.dumps(kernel_state.get('tracks'), ensure_ascii=False)[:300]}")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 0,
                "setup baseline: the volume curve must start empty")
        record("setup", {"ride_track_id": ride_track_id, "ride_volume_plugin_id": ride_plugin_id})

        conv_main = "automation_intent_smoke_main"
        chat_context = {"agent_mode": "chat", "selected_track_id": ride_track_id}

        # ---- LEG A: positive phrase -> confirmation card -> execute -------
        proposed = chat(args.agent_http, conv_main, "0 到 30 秒，让这条轨的存在感慢慢强起来", chat_context)
        require(proposed.get("workflow") == "section_automation_choreo",
                f"LEG A proposal workflow = {proposed.get('workflow')!r}, want section_automation_choreo: {json.dumps(proposed, ensure_ascii=False)[:800]}")
        require(str(proposed.get("goal_status", "")).lower() in {"waiting_confirmation", "waiting_confirm"},
                f"LEG A proposal goal_status = {proposed.get('goal_status')!r}")
        require(proposed.get("stop_reason") == "automation_choreo_awaiting_confirmation",
                f"LEG A proposal stop_reason = {proposed.get('stop_reason')!r}")
        interactions = proposed.get("interaction_requests") or []
        card = next((row for row in interactions if isinstance(row, dict) and row.get("kind") == "automation_choreo_confirmation"), None)
        require(card is not None, f"LEG A: no automation_choreo_confirmation card: {json.dumps(interactions, ensure_ascii=False)[:600]}")
        field_ids = sorted(str(field.get("id") or "") for field in (card.get("fields") or []) if isinstance(field, dict))
        require(field_ids == ["curve", "end_value", "region_end", "region_start", "shape", "start_value"],
                f"LEG A card fields = {field_ids}")
        shape_options = [str(option.get("id") or "") for option in (card.get("fields") or [{}])[0].get("options") or [] if isinstance(option, dict)]
        require(sorted(shape_options) == ["fade_in", "fade_out", "hold", "push"],
                f"LEG A shape options = {shape_options}")
        workflow_data = proposed.get("workflow_data") or {}
        require(workflow_data.get("mutation_performed") is False, "LEG A: confirmation card must carry mutation_performed=false")
        require(approx((workflow_data.get("region_start_s")), 0.0) and approx(workflow_data.get("region_end_s"), 30.0),
                f"LEG A disclosed region = {workflow_data.get('region_start_s')}-{workflow_data.get('region_end_s')}")
        require(workflow_data.get("param_id") == "volume" and bool(workflow_data.get("plugin_id")),
                f"LEG A disclosed target = {workflow_data.get('plugin_id')}/{workflow_data.get('param_id')}")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 0,
                "LEG A: zero-write before confirmation — the curve must stay empty while waiting")

        confirmed = chat(args.agent_http, conv_main, "确认写入", chat_context)
        require(confirmed.get("stop_reason") == "automation_choreo_settled",
                f"LEG A confirm stop_reason = {confirmed.get('stop_reason')!r}: {json.dumps(confirmed, ensure_ascii=False)[:1200]}")
        settle_data = confirmed.get("workflow_data") or {}
        require(settle_data.get("mutation_performed") is True, "LEG A: settle must carry mutation_performed=true")
        require(settle_data.get("written") == 2, f"LEG A written = {settle_data.get('written')!r}, want 2")
        require(settle_data.get("readback_tolerance") is not None, "LEG A: settle must disclose the readback tolerance")
        require(settle_data.get("mixboard_decision_status") == "recorded" and bool(settle_data.get("mixboard_decision_record_id")),
                f"LEG A decision ledger = {settle_data.get('mixboard_decision_status')}/{settle_data.get('mixboard_decision_record_id')}")
        queue_c4 = settle_data.get("workflow_queue_c4") or {}
        require(queue_c4.get("node_id") == "C4" and queue_c4.get("state") == "in_progress",
                f"LEG A C4 queue node = {json.dumps(queue_c4, ensure_ascii=False)[:300]}")
        kernel_curve_reply = kernel_curve(args.kernel_zmq, ride_plugin_id, "volume")
        points = kernel_curve_reply.get("points") or []
        require(len(points) == 2, f"LEG A kernel curve points = {len(points)}, want 2")
        require(approx(points[0].get("t"), 0.0) and approx(points[0].get("v"), 0.30),
                f"LEG A point#0 = {points[0]!r}, want t=0 v=0.30 (default fade-in)")
        require(approx(points[1].get("t"), 30.0) and approx(points[1].get("v"), 0.60),
                f"LEG A point#1 = {points[1]!r}, want t=30 v=0.60")
        record("leg_a_positive_chain", {
            "confirmation_card_fields": field_ids,
            "written": settle_data.get("written"),
            "decision_record_id": settle_data.get("mixboard_decision_record_id"),
            "c4_state": queue_c4.get("state"),
            "kernel_points": points,
        })

        # ---- LEG B: readback entry ---------------------------------------
        readback = chat(args.agent_http, conv_main, "读回刚才那条线", chat_context)
        require(readback.get("stop_reason") == "automation_choreo_readback_ok",
                f"LEG B stop_reason = {readback.get('stop_reason')!r}: {json.dumps(readback, ensure_ascii=False)[:800]}")
        readback_data = readback.get("workflow_data") or {}
        require(readback_data.get("point_count") == 2 and readback_data.get("matches_written") is True,
                f"LEG B readback data = {json.dumps(readback_data, ensure_ascii=False)[:400]}")
        record("leg_b_readback_entry", {"point_count": readback_data.get("point_count"), "matches_written": readback_data.get("matches_written")})

        # ---- LEG C: negative boundary 1 — no time-varying word -----------
        conv_static = "automation_intent_smoke_static"
        static_reply = chat(args.agent_http, conv_static, "把这条轨的存在感调强一点", chat_context)
        require(static_reply.get("workflow") != "section_automation_choreo",
                f"LEG C static phrase armed the choreography: {json.dumps(static_reply, ensure_ascii=False)[:600]}")
        static_data = static_reply.get("workflow_data") or {}
        require(static_data.get("mutation_performed") is not True,
                "LEG C static phrase must not report a choreography mutation")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 2,
                "LEG C: the curve must stay untouched by the static phrase")
        record("leg_c_static_boundary", {"workflow": static_reply.get("workflow"), "curve_points": 2})

        # ---- LEG D: negative boundary 2 — no region, no marker ------------
        conv_clarify = "automation_intent_smoke_clarify"
        clarify_reply = chat(args.agent_http, conv_clarify, "让这条轨的存在感慢慢强起来", chat_context)
        require(str(clarify_reply.get("stop_reason", "")).startswith("automation_choreo_clarify"),
                f"LEG D stop_reason = {clarify_reply.get('stop_reason')!r}: {json.dumps(clarify_reply, ensure_ascii=False)[:800]}")
        require("哪一秒" in str(clarify_reply.get("reply") or clarify_reply.get("message") or "") or "区间" in str(clarify_reply.get("reply") or ""),
                f"LEG D reply is not a region ask: {clarify_reply.get('reply')!r}")
        clarify_data = clarify_reply.get("workflow_data") or {}
        require(clarify_data.get("mutation_performed") is False, "LEG D: clarification must not mutate")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 2,
                "LEG D: the curve must stay untouched by the clarification ask")
        record("leg_d_region_clarification", {"stop_reason": clarify_reply.get("stop_reason"), "curve_points": 2})

        # ---- LEG E: category boundary — space class without a plugin ------
        conv_space = "automation_intent_smoke_space"
        space_reply = chat(args.agent_http, conv_space, "让这条轨的空间感慢慢铺进来", chat_context)
        require(space_reply.get("stop_reason") == "automation_category_boundary_no_plugin",
                f"LEG E stop_reason = {space_reply.get('stop_reason')!r}: {json.dumps(space_reply, ensure_ascii=False)[:800]}")
        space_data = space_reply.get("workflow_data") or {}
        require(space_data.get("boundary") == "no_plugin_on_track",
                f"LEG E boundary marker = {space_data.get('boundary')!r}")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 2,
                "LEG E: the boundary answer must not touch the curve")
        record("leg_e_space_boundary", {"stop_reason": space_reply.get("stop_reason"), "curve_points": 2})

        # ---- LEG F: C4 progress via mix_report (queue fold) ---------------
        report = invoke_ok(args.agent_http, "mix_report", {})
        queue_view = report.get("mix_workflow_queue")
        require(isinstance(queue_view, dict), f"LEG F mix_report carries no mix_workflow_queue fold: {json.dumps(report, ensure_ascii=False)[:600]}")
        nodes = queue_view.get("nodes") or queue_view.get("nodes_by_id") or {}
        c4_row = None
        if isinstance(nodes, list):
            c4_row = next((row for row in nodes if isinstance(row, dict) and row.get("node_id") == "C4"), None)
        elif isinstance(nodes, dict):
            c4_row = nodes.get("C4")
        require(c4_row is not None, f"LEG F queue view has no C4 node: {json.dumps(queue_view, ensure_ascii=False)[:500]}")
        c4_state = str(c4_row.get("state") or c4_row.get("status") or "")
        require(c4_state.lower() == "in_progress",
                f"LEG F C4 node state = {c4_state!r}, want in_progress: {json.dumps(c4_row, ensure_ascii=False)[:300]}")
        record("leg_f_mix_report_c4", {"c4_state": c4_state})

        summary["status"] = "PASS"
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
    except Exception as exc:  # noqa: BLE001 - smoke failure reporting.
        summary["status"] = "FAIL"
        summary["error"] = str(exc)
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print(f"[FAIL] {exc}", file=sys.stderr)
        Path(args.output).parent.mkdir(parents=True, exist_ok=True)
        Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        sys.exit(1)

    Path(args.output).parent.mkdir(parents=True, exist_ok=True)
    Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print("[PASS] automation intent chain smoke: all legs green")


if __name__ == "__main__":
    main()
