#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""282 EDU-ASK-ANCHOR-1 stack leg: chat-driven transport.set_loop / clear_loop.

Phase B of the frozen RUNBOOK (same directory). Drives the real agent
/agent/chat HTTP face with two real LLM turns (deepseek-flash), then asserts
on three layers:
  1. daw_action event-stream traces (real tool invocations, not turn-body
     text echoes — 278 run6 precedent),
  2. the kernel reply embedded in the tool item payload (loop_enabled /
     loop_start_seconds / loop_end_seconds),
  3. an independent kernel ZMQ `stop` read-back (idempotent; buildTransportReply
     carries the full transport state).

Exit codes (RUNBOOK §8):
  0 = PASS   2 = environment   3 = LLM behavior (no tool call)   4 = harness/link
"""

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

import zmq

TERMINAL_GOALS = {"completed", "failed", "stopped", "cancelled"}
CHAT_TIMEOUT_SEC = 300.0
ROUND_BUDGET_SEC = 300.0
APPROVE_HOPS_MAX = 2

SET_MESSAGE = (
    "请设置循环播放：把循环区（loop region）设为从 10 秒到 15 秒，"
    "我想反复听这一段。只要设置循环区就好，先不要开始播放。"
)
CLEAR_MESSAGE = "请清除循环播放：取消刚才设置的循环区，把循环关掉。"


def log(message: str) -> None:
    print("[t282] %s" % message, flush=True)


def post_json(base_url: str, path: str, payload: dict, timeout: float) -> dict:
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
        raise RuntimeError("%s returned HTTP %s: %s" % (path, exc.code, body[:800])) from exc


def get_json(base_url: str, path: str, timeout: float = 30.0) -> dict:
    with urllib.request.urlopen(base_url.rstrip("/") + path, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def zmq_command(url: str, payload: dict, timeout_ms: int = 60000) -> dict:
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
    last_error = None
    while time.time() < deadline:
        try:
            health = get_json(base_url, "/health", timeout=5.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # noqa: BLE001 - retry diagnostics.
            last_error = exc
        time.sleep(1.0)
    raise RuntimeError("agent HTTP not ready: %s" % last_error)


def wait_kernel_zmq(url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error = None
    while time.time() < deadline:
        try:
            reply = zmq_command(url, {"cmd": "ping"}, timeout_ms=5000)
            if str(reply.get("status", "")).lower() in {"ok", "success"}:
                return
            last_error = RuntimeError("ping replied %r" % reply.get("status"))
        except Exception as exc:  # noqa: BLE001 - retry diagnostics.
            last_error = exc
        time.sleep(1.0)
    raise RuntimeError("kernel ZMQ not ready: %s" % last_error)


def kernel_transport_readback(kernel_zmq: str) -> dict:
    """Idempotent `stop`: never touches the loop state, returns the full
    buildTransportReply body (loop_enabled / loop_start_seconds / ...)."""
    reply = zmq_command(kernel_zmq, {"cmd": "stop"})
    if str(reply.get("status", "")).lower() != "ok":
        raise RuntimeError("kernel stop read-back failed: %s" % json.dumps(reply, ensure_ascii)[:400])
    return reply


def ensure_active_edit(kernel_zmq: str) -> str:
    """Guarantee an active edit exists so transport commands cannot fail with
    'No active edit loaded'. Uses stop as the probe."""
    probe = zmq_command(kernel_zmq, {"cmd": "stop"})
    if str(probe.get("status", "")).lower() == "ok":
        return "already_loaded"
    reply = zmq_command(kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
    if str(reply.get("status", "")).lower() not in {"ok", "success"}:
        raise RuntimeError("new_project failed: %s" % json.dumps(reply, ensure_ascii)[:400])
    return "new_project_created"


def fetch_events(agent_http: str, conversation: str) -> list:
    try:
        doc = get_json(
            agent_http,
            "/agent/events?conversation_id=%s&since=0&limit=400" % conversation,
            timeout=30.0,
        )
        events = doc.get("events")
        return events if isinstance(events, list) else []
    except Exception as exc:  # noqa: BLE001 - recorded, retried by caller.
        log("events fetch error: %s" % exc)
        return []


def runtime_goal_status(agent_http: str) -> str:
    try:
        doc = get_json(agent_http, "/agent/runtime/status", timeout=30.0)
        goal = doc.get("goal")
        return str((goal or {}).get("status") or "")
    except Exception:  # noqa: BLE001 - best-effort poll.
        return ""


def pending_card(resp: dict) -> dict:
    """Return the first interaction_requests entry worth an approve hop."""
    rows = resp.get("interaction_requests")
    if not isinstance(rows, list):
        return {}
    for row in rows:
        if isinstance(row, dict) and row.get("interaction_id"):
            return row
    return {}


def run_chat_round(agent_http: str, label: str, message: str, run_root: Path) -> dict:
    """One chat turn: POST, settle to a terminal goal, approve at most
    APPROVE_HOPS_MAX governance cards (282 handoff: r2-shape confirmations are
    driven the same way as 278/306 approve hops)."""
    conversation = "t282_%s_%s" % (label, time.strftime("%Y%m%d_%H%M%S"))
    out = {"conversation_id": conversation, "label": label}
    started = time.time()

    resp = post_json(agent_http, "/agent/chat", {
        "conversation_id": conversation,
        "message": message,
    }, CHAT_TIMEOUT_SEC)
    out["chat_response"] = resp
    (run_root / ("%s_chat_response.json" % label)).write_text(
        json.dumps(resp, ensure_ascii=False, indent=1), encoding="utf-8")

    goal_status = str(resp.get("goal_status") or "")
    hops = 0
    deadline = time.time() + ROUND_BUDGET_SEC
    while time.time() < deadline:
        card = pending_card(resp)
        if card and hops < APPROVE_HOPS_MAX:
            hops += 1
            log("%s: approve hop %d on %s" % (label, hops, card.get("interaction_id")))
            resp = post_json(agent_http, "/agent/interaction/respond", {
                "interaction_id": card.get("interaction_id"),
                "action_id": "approve",
                "decision": "approve",
                "payload": {},
            }, CHAT_TIMEOUT_SEC)
            out.setdefault("approve_hops", []).append({
                "interaction_id": card.get("interaction_id"),
                "kind": card.get("kind") or card.get("interaction_kind"),
                "stop_reason": resp.get("stop_reason"),
                "goal_status": resp.get("goal_status"),
            })
            goal_status = str(resp.get("goal_status") or goal_status)
            deadline = max(deadline, time.time() + ROUND_BUDGET_SEC)
            continue
        if goal_status in TERMINAL_GOALS:
            break
        polled = runtime_goal_status(agent_http)
        if polled in TERMINAL_GOALS:
            goal_status = polled
            # chat response goal_status remains authoritative (yue run6).
            chat_status = str((out.get("chat_response") or {}).get("goal_status") or "")
            if goal_status == "idle" and chat_status in TERMINAL_GOALS:
                goal_status = chat_status
            if goal_status in TERMINAL_GOALS:
                break
        time.sleep(4.0)

    out["goal_status"] = goal_status
    out["stop_reason"] = str(resp.get("stop_reason") or "")
    out["wall_seconds"] = int(time.time() - started)
    out["approve_hop_count"] = hops

    deadline = time.time() + 20.0
    events = []
    while time.time() < deadline:
        events = fetch_events(agent_http, conversation)
        if events:
            break
        time.sleep(2.0)
    out["event_count"] = len(events)
    (run_root / ("%s_events.json" % label)).write_text(
        json.dumps(events, ensure_ascii=False, indent=1), encoding="utf-8")
    out["_events"] = events
    return out


def tool_traces(events: list, tool_name: str) -> list:
    """Completed daw_action items that carried a real invocation of tool_name
    (item_id / payload.tool / payload.command_name — 278 run6 precedent)."""
    hits = []
    for ev in events:
        if not isinstance(ev, dict) or ev.get("item_type") != "daw_action":
            continue
        if str(ev.get("type") or "") not in {"item.completed", "approval.requested", "item.started"}:
            continue
        payload = ev.get("payload") if isinstance(ev.get("payload"), dict) else {}
        texts = [
            str(ev.get("item_id") or ""),
            str(payload.get("tool") or ""),
            str(payload.get("command_name") or ""),
        ]
        if not any(tool_name in text for text in texts):
            continue
        hits.append({
            "type": ev.get("type"),
            "status": ev.get("status"),
            "tool": payload.get("tool"),
            "command_name": payload.get("command_name"),
            "requires_confirmation": payload.get("requires_confirmation"),
            "error": payload.get("error"),
            "result": payload.get("result"),
        })
    return hits


def number_or_none(value):
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return float(value)
    return None


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--run-root", required=True)
    args = parser.parse_args()
    run_root = Path(args.run_root)
    run_root.mkdir(parents=True, exist_ok=True)

    summary = {
        "leg": "282 transport loop chat-driven stack leg",
        "started_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
    }

    def finish(code: int, outcome: str) -> int:
        summary["outcome"] = outcome
        summary["exit_code"] = code
        summary["finished_at"] = time.strftime("%Y-%m-%d %H:%M:%S")
        (run_root / "summary.json").write_text(
            json.dumps(summary, ensure_ascii=False, indent=1, default=str), encoding="utf-8")
        log("outcome=%s exit=%d" % (outcome, code))
        return code

    # ---- phase 0: stack readiness + active edit -----------------------------
    try:
        wait_agent_http(args.agent_http, timeout_s=60.0)
        wait_kernel_zmq(args.kernel_zmq, timeout_s=60.0)
        edit_state = ensure_active_edit(args.kernel_zmq)
        summary["active_edit_state"] = edit_state
    except Exception as exc:  # noqa: BLE001 - environment class.
        summary["environment_error"] = str(exc)
        return finish(2, "environment_stack_not_ready")

    baseline = kernel_transport_readback(args.kernel_zmq)
    summary["baseline_transport"] = baseline
    if bool(baseline.get("loop_enabled")):
        log("baseline loop_enabled=true; clearing first for a clean baseline")
        zmq_command(args.kernel_zmq, {"cmd": "transport_clear_loop"})
        baseline = kernel_transport_readback(args.kernel_zmq)

    # ---- phase 1: chat set_loop round ---------------------------------------
    set_round = run_chat_round(args.agent_http, "set", SET_MESSAGE, run_root)
    summary["set_round"] = {k: v for k, v in set_round.items() if k != "_events"}
    set_events = set_round.pop("_events", [])
    if set_round["goal_status"] not in TERMINAL_GOALS:
        return finish(3, "set_round_no_terminal_goal")
    if set_round["goal_status"] != "completed":
        summary["set_round_failed_status"] = set_round["goal_status"]
        return finish(3, "set_round_goal_%s" % set_round["goal_status"])

    set_traces = tool_traces(set_events, "transport.set_loop")
    summary["set_loop_traces"] = set_traces
    if not set_traces:
        return finish(3, "set_round_no_tool_call")

    completed = [t for t in set_traces if t.get("type") == "item.completed"]
    if not completed:
        summary["set_loop_no_completed_item"] = True
        return finish(4, "set_loop_never_completed")
    set_item = completed[-1]
    set_result = set_item.get("result") if isinstance(set_item.get("result"), dict) else {}
    summary["set_loop_kernel_reply"] = set_result
    if str(set_result.get("status", "")).lower() != "ok":
        return finish(4, "set_loop_kernel_reply_not_ok")

    reply_start = number_or_none(set_result.get("loop_start_seconds"))
    reply_end = number_or_none(set_result.get("loop_end_seconds"))
    if reply_start is None or reply_end is None:
        return finish(4, "set_loop_reply_missing_loop_range")
    summary["set_loop_reply_range"] = [reply_start, reply_end]
    summary["soft_param_exact_10_15"] = (
        abs(reply_start - 10.0) < 1e-6 and abs(reply_end - 15.0) < 1e-6
    )

    after_set = kernel_transport_readback(args.kernel_zmq)
    summary["after_set_readback"] = after_set
    if not bool(after_set.get("loop_enabled")):
        return finish(4, "after_set_loop_not_enabled")
    live_start = number_or_none(after_set.get("loop_start_seconds"))
    live_end = number_or_none(after_set.get("loop_end_seconds"))
    if live_start is None or live_end is None:
        return finish(4, "after_set_readback_missing_range")
    if abs(live_start - reply_start) > 1e-6 or abs(live_end - reply_end) > 1e-6:
        summary["readback_mismatch"] = {
            "reply": [reply_start, reply_end],
            "live": [live_start, live_end],
        }
        return finish(4, "after_set_range_mismatch")
    log("set round verified: loop=[%s, %s] enabled=True (reply==live)"
        % (live_start, live_end))

    # ---- phase 2: chat clear_loop round -------------------------------------
    clear_round = run_chat_round(args.agent_http, "clear", CLEAR_MESSAGE, run_root)
    summary["clear_round"] = {k: v for k, v in clear_round.items() if k != "_events"}
    clear_events = clear_round.pop("_events", [])
    if clear_round["goal_status"] not in TERMINAL_GOALS:
        return finish(3, "clear_round_no_terminal_goal")
    if clear_round["goal_status"] != "completed":
        summary["clear_round_failed_status"] = clear_round["goal_status"]
        return finish(3, "clear_round_goal_%s" % clear_round["goal_status"])

    clear_traces = tool_traces(clear_events, "transport.clear_loop")
    summary["clear_loop_traces"] = clear_traces
    if not clear_traces:
        return finish(3, "clear_round_no_tool_call")
    clear_completed = [t for t in clear_traces if t.get("type") == "item.completed"]
    if not clear_completed:
        summary["clear_loop_no_completed_item"] = True
        return finish(4, "clear_loop_never_completed")
    clear_item = clear_completed[-1]
    clear_result = clear_item.get("result") if isinstance(clear_item.get("result"), dict) else {}
    summary["clear_loop_kernel_reply"] = clear_result
    if str(clear_result.get("status", "")).lower() != "ok":
        return finish(4, "clear_loop_kernel_reply_not_ok")

    after_clear = kernel_transport_readback(args.kernel_zmq)
    summary["after_clear_readback"] = after_clear
    if bool(after_clear.get("loop_enabled")):
        return finish(4, "after_clear_loop_still_enabled")
    log("clear round verified: loop_enabled=False")

    return finish(0, "pass")


if __name__ == "__main__":
    sys.exit(main())
