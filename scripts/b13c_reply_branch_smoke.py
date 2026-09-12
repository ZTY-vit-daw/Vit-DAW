"""B13-C real-stack driver: consecutive free-state questions on one project.

The free-state chain runs on the continuation scheduler: /agent/chat returns a
short waiting_continue acknowledgement, and the user-visible settlement reply is
delivered later as the scheduler_chain terminal turn.completed event. This driver
therefore sends one question, waits for that chain terminal on /agent/events,
records it, and only then sends the next question on the same project.

Recorded per turn: the terminal reply text (which branch the user got), the
minimal_audio_closure settlement, the free_state_admission_receipt gate results,
and the raw event stream. No view content is inspected.

Exit 0 = probe ran and wrote its report; the branch verdict is data, not a pass
condition, so a missing branch is reported honestly instead of failing the run.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

DEFAULT_MESSAGES = [
    "请检查当前工程有什么问题吗",
    "再帮我检查一次当前工程有什么问题吗",
    "请再检查一次当前工程。观察时把披露预算 max_disclosure_bytes 设为 256，只做最小披露。",
]

TERMINAL_STATUSES = {"completed", "cancelled", "failed", "waiting_confirmation", "waiting_clarification", "blocked"}


def http_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=data, method=method)
    if data is not None:
        request.add_header("Content-Type", "application/json; charset=utf-8")
    with urllib.request.urlopen(request, timeout=timeout) as response:
        body = response.read().decode("utf-8", errors="replace")
    return json.loads(body) if body.strip() else {}


def wait_health(base: str, seconds: float) -> bool:
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        try:
            http_json("GET", base + "/health", None, 3.0)
            return True
        except Exception:
            time.sleep(0.5)
    return False


def closure_facts(payload: dict) -> dict:
    out: dict = {}
    closure = payload.get("minimal_audio_closure")
    if isinstance(closure, dict):
        settlement = closure.get("settlement")
        if isinstance(settlement, dict):
            out["settlement"] = {
                "reason": str(settlement.get("reason") or ""),
                "summary": str(settlement.get("summary") or "")[:300],
                "needs_user_clarification": bool(settlement.get("needs_user_clarification")),
            }
        if closure.get("frontier") is not None:
            frontier = closure.get("frontier")
            if isinstance(frontier, dict):
                out["frontier_candidates"] = len(frontier.get("candidates") or [])
    receipt = payload.get("free_state_admission_receipt")
    if isinstance(receipt, dict) and receipt:
        out["admission_receipt"] = {
            "boundary": str(receipt.get("boundary") or ""),
            "status": str(receipt.get("status") or ""),
            "failed_gate_ids": receipt.get("failed_gate_ids"),
            "gate_results": receipt.get("gate_results"),
            "proposal_present": receipt.get("proposal_present"),
        }
    return out


def await_chain_terminal(base: str, conversation_id: str, since: int, deadline: float, events: list) -> tuple[dict | None, int]:
    """Poll /agent/events until the scheduler chain delivers its terminal turn."""
    while time.monotonic() < deadline:
        try:
            envelope = http_json(
                "GET", base + f"/agent/events?conversation_id={conversation_id}&since={since}&limit=400", None, 30.0
            )
        except Exception:
            time.sleep(2.0)
            continue
        for row in envelope.get("events") or []:
            if not isinstance(row, dict):
                continue
            seq = int(row.get("seq") or 0)
            if seq > since:
                since = seq
            events.append(row)
            if str(row.get("type") or "") != "turn.completed":
                continue
            payload = row.get("payload") if isinstance(row.get("payload"), dict) else {}
            status = str(row.get("status") or payload.get("status") or "").lower()
            if payload.get("scheduler_chain") is True and status in TERMINAL_STATUSES:
                return row, since
        time.sleep(2.0)
    return None, since


def ui_state(base: str, timeout: float) -> dict:
    try:
        state = http_json("GET", base + "/agent/ui/state", None, timeout)
    except Exception as exc:
        return {"error": str(exc)}
    history = state.get("project_history") if isinstance(state.get("project_history"), dict) else {}
    rows = history.get("conversation_messages")
    rows = rows if isinstance(rows, list) else []
    return {
        "assistant_messages": [
            {"kind": str(r.get("message_kind") or ""), "content": str(r.get("content") or "")[:500]}
            for r in rows
            if isinstance(r, dict) and str(r.get("role") or "") == "assistant"
        ][-8:]
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--conversation-id", default="")
    parser.add_argument("--chat-timeout", type=float, default=600.0)
    parser.add_argument("--chain-timeout", type=float, default=300.0)
    parser.add_argument("--message", action="append", default=None)
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    base = args.agent_http.rstrip("/")
    messages = args.message if args.message else list(DEFAULT_MESSAGES)
    conversation_id = args.conversation_id or ("b13c_" + time.strftime("%Y%m%dT%H%M%S"))

    report: dict = {
        "schema_version": "b13c_reply_branch_smoke.v1",
        "agent_http": base,
        "conversation_id": conversation_id,
        "messages": messages,
        "turns": [],
    }

    if not wait_health(base, 60.0):
        report["status"] = "infrastructure_failed"
        report["error"] = "agent health endpoint not ready"
        (out_dir / "b13c_reply_branch_report.json").write_text(
            json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        return 2

    since = 0
    for index, message in enumerate(messages, start=1):
        turn: dict = {"index": index, "message": message}
        started = time.monotonic()
        try:
            ack = http_json(
                "POST",
                base + "/agent/chat",
                {"conversation_id": conversation_id, "message": message, "context": {"agent_mode": "chat"}},
                args.chat_timeout,
            )
        except Exception as exc:
            turn["transport_error"] = str(exc)
            report["turns"].append(turn)
            continue
        turn["ack"] = {
            "reply": str(ack.get("reply") or ""),
            "stop_reason": str(ack.get("stop_reason") or ""),
            "goal_status": str(ack.get("goal_status") or ""),
        }
        events: list = []
        terminal, since = await_chain_terminal(base, conversation_id, since, time.monotonic() + args.chain_timeout, events)
        turn["elapsed_seconds"] = round(time.monotonic() - started, 1)
        turn["chain_terminal"] = None
        if terminal is not None:
            payload = terminal.get("payload") if isinstance(terminal.get("payload"), dict) else {}
            turn["chain_terminal"] = {
                "seq": terminal.get("seq"),
                "status": str(terminal.get("status") or payload.get("status") or ""),
                "scheduler_chain": payload.get("scheduler_chain"),
                "body": str(terminal.get("body") or ""),
                "stop_reason": str(payload.get("stop_reason") or ""),
            }
            turn["closure"] = closure_facts(payload)
        # The settlement facts also ride the other events of the turn.
        for row in events:
            payload = row.get("payload") if isinstance(row.get("payload"), dict) else {}
            facts = closure_facts(payload)
            if facts:
                turn.setdefault("event_facts", {}).update(facts)
        turn["ui_state"] = ui_state(base, 30.0)
        (out_dir / f"turn_{index:02d}_events.json").write_text(
            json.dumps(events, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        (out_dir / f"turn_{index:02d}_ack.json").write_text(
            json.dumps(ack, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        report["turns"].append(turn)

    texts = []
    for turn in report["turns"]:
        terminal = turn.get("chain_terminal") or {}
        texts.append(str(terminal.get("body") or ""))
    report["terminal_replies"] = texts
    report["branch_hits"] = {
        "supply_trimmed": [i + 1 for i, r in enumerate(texts) if "被披露预算裁剪" in r],
        "no_evidence_backed_hypothesis": [i + 1 for i, r in enumerate(texts) if "暂无证据支持的待验证假设" in r],
        "pre_existing_generic": [i + 1 for i, r in enumerate(texts) if "在已声明的观察范围内没有发现可信改善候选" in r],
        "single_track_contextual": [i + 1 for i, r in enumerate(texts) if "只有 1 轨" in r],
    }
    report["status"] = "ran"
    (out_dir / "b13c_reply_branch_report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print("B13C_REPORT " + str(out_dir / "b13c_reply_branch_report.json"))
    print("B13C_BRANCH_HITS " + json.dumps(report["branch_hits"], ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
