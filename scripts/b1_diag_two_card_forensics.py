"""B1-DIAG forensics driver: two-card confirmation flow with frame-level event recording.

Reproduces the MANTEST-2 B-segment flow (natural improvement request ->
improvement_proposal_confirmation -> mix_tick_confirmation) over the same HTTP
endpoints the WebUI uses, records the /agent/events stream frame-by-frame, and
captures every response body verbatim for the terminal-boundary forensics
questions (Q1 event emission / Q2 orphan close / Q3 persisted history / Q5
inline respond timing). Read-only against production code; writes only to the
--out-dir artifact directory.
"""

from __future__ import annotations

import argparse
import json
import threading
import time
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as response:
        parsed = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


class EventRecorder:
    """Polls /agent/events with a since cursor, mirroring the WebUI consumer."""

    def __init__(self, base_url: str, conversation_id: str, interval: float, log_path: Path):
        self.url = base_url.rstrip("/") + "/agent/events"
        self.conversation_id = conversation_id
        self.interval = interval
        self.log_path = log_path
        self.since = 0
        self.frames: list[dict[str, Any]] = []
        self.events: list[dict[str, Any]] = []
        self.errors: list[str] = []
        self._stop = threading.Event()
        self._lock = threading.Lock()

    def _poll_once(self) -> None:
        started = time.monotonic()
        query = f"?conversation_id={self.conversation_id}&since={self.since}&limit=120"
        response = request_json("GET", self.url + query, None, 10.0)
        arrived = time.monotonic()
        events = response.get("events") or []
        next_seq = response.get("next_seq")
        frame = {
            "wallclock": now_stamp(),
            "monotonic_ms": round(started * 1000, 1),
            "latency_ms": round((arrived - started) * 1000, 1),
            "since": self.since,
            "next_seq": next_seq,
            "event_count": len(events),
            "events": [
                {
                    "seq": e.get("seq"),
                    "type": e.get("type"),
                    "status": e.get("status"),
                    "item_type": e.get("item_type"),
                    "title": (e.get("title") or "")[:80],
                    "body_head": (e.get("body") or "")[:160],
                    "created_at": e.get("created_at"),
                    "lifecycle": e.get("lifecycle"),
                    "persistence": e.get("persistence"),
                    "message_kind": e.get("message_kind"),
                    "payload_keys": sorted((e.get("payload") or {}).keys()),
                    "scheduler_chain": bool((e.get("payload") or {}).get("scheduler_chain")),
                }
                for e in events
            ],
        }
        with self._lock:
            self.frames.append(frame)
            self.events.extend(events)
            if isinstance(next_seq, (int, float)):
                self.since = max(self.since, int(next_seq))
            else:
                for e in events:
                    self.since = max(self.since, int(e.get("seq") or 0))

    def run(self) -> None:
        while not self._stop.is_set():
            try:
                self._poll_once()
            except Exception as exc:  # noqa: BLE001 - recorder must survive and log errors.
                with self._lock:
                    self.errors.append(f"{now_stamp()} {type(exc).__name__}: {exc}")
            self._stop.wait(self.interval)

    def start(self) -> None:
        self._thread = threading.Thread(target=self.run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._thread.join(timeout=15.0)

    def dump(self) -> None:
        with self._lock:
            payload = {
                "conversation_id": self.conversation_id,
                "final_since": self.since,
                "errors": self.errors,
                "frames": self.frames,
                "raw_events": self.events,
            }
        self.log_path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def find_interaction(response: dict[str, Any], *kinds: str) -> dict[str, Any] | None:
    requests = response.get("interaction_requests")
    if not isinstance(requests, list):
        return None
    for row in requests:
        if not isinstance(row, dict):
            continue
        kind = str(row.get("kind", row.get("type", "")))
        if not kinds or kind in kinds:
            return row
    return None


def save_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def brief(response: dict[str, Any]) -> dict[str, Any]:
    return {
        "goal_status": response.get("goal_status"),
        "stop_reason": response.get("stop_reason"),
        "needs_confirmation": response.get("needs_confirmation"),
        "reply": response.get("reply"),
        "reply_len": len(str(response.get("reply") or "")),
        "workflow": response.get("workflow"),
        "interaction_requests": [
            {"id": r.get("id"), "kind": r.get("kind", r.get("type")), "title": r.get("title")}
            for r in (response.get("interaction_requests") or [])
            if isinstance(r, dict)
        ],
        "project_history_keys": sorted(((response.get("project_history") or {}).keys())),
        "project_history_conversation_messages": (
            response.get("project_history", {}).get("conversation_messages")
        ),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--message", default="请检查当前工程是否有什么问题吗")
    parser.add_argument("--fallback-message", default="混一下当前轨道")
    parser.add_argument("--timeout", type=float, default=180.0)
    parser.add_argument("--card-timeout", type=float, default=240.0)
    parser.add_argument("--terminal-idle-sec", type=float, default=25.0)
    parser.add_argument("--poll-interval", type=float, default=0.25)
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    base = args.agent_http.rstrip("/")
    started_at = now_stamp()
    timeline: list[dict[str, Any]] = []

    def mark(label: str, **fields: Any) -> None:
        row = {"wallclock": now_stamp(), "label": label, **fields}
        timeline.append(row)
        print(f"[{row['wallclock']}] {label} " + json.dumps({k: v for k, v in fields.items()}, ensure_ascii=False), flush=True)

    request_json("GET", base + "/health", None, 5.0)
    mark("stack_healthy")

    conversation_id = f"b1diag_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    recorder = EventRecorder(base, conversation_id, args.poll_interval, out_dir / "event_recording.json")
    recorder.start()
    mark("recorder_started", conversation_id=conversation_id)

    chat_started = time.monotonic()
    chat = request_json(
        "POST",
        base + "/agent/chat",
        {"conversation_id": conversation_id, "message": args.message, "context": {"agent_mode": "chat"}},
        args.timeout,
    )
    chat_ms = round((time.monotonic() - chat_started) * 1000, 1)
    save_json(out_dir / "01_chat_response.json", chat)
    mark("chat_done", ms=chat_ms, **brief(chat))

    card1 = find_interaction(chat, "improvement_proposal_confirmation")
    if card1 is None:
        card1 = find_interaction(chat)
    if card1 is None:
        mark("no_pending_card1", goal_status=chat.get("goal_status"), stop_reason=chat.get("stop_reason"))
        recorder.stop()
        recorder.dump()
        save_json(out_dir / "timeline.json", {"started_at": started_at, "rows": timeline})
        return 2

    t0 = time.monotonic()
    approve1 = request_json(
        "POST",
        base + "/agent/interaction/respond",
        {"interaction_id": str(card1.get("id", "")), "action_id": "approve", "decision": "approve", "payload": {}},
        args.card_timeout,
    )
    approve1_ms = round((time.monotonic() - t0) * 1000, 1)
    save_json(out_dir / "02_approve_card1_response.json", approve1)
    mark("approve1_done", ms=approve1_ms, interaction=str(card1.get("id")), **brief(approve1))

    card2 = find_interaction(approve1, "mix_tick_confirmation")
    if card2 is None:
        # poll events/state briefly for a mix_tick card delivered via stream
        deadline = time.monotonic() + 30.0
        while time.monotonic() < deadline and card2 is None:
            with recorder._lock:
                for e in recorder.events:
                    payload = e.get("payload") or {}
                    cand = payload.get("interaction") if isinstance(payload, dict) else None
                    if isinstance(cand, dict) and str(cand.get("kind", cand.get("type", ""))) == "mix_tick_confirmation":
                        card2 = cand
                        break
            time.sleep(0.5)
    if card2 is None:
        mark("no_pending_card2", note="approve1 response and event stream carried no mix_tick_confirmation card")
        recorder.stop()
        recorder.dump()
        save_json(out_dir / "timeline.json", {"started_at": started_at, "rows": timeline})
        return 3

    t0 = time.monotonic()
    approve2 = request_json(
        "POST",
        base + "/agent/interaction/respond",
        {"interaction_id": str(card2.get("id", "")), "action_id": "approve", "decision": "approve", "payload": {}},
        args.card_timeout,
    )
    approve2_ms = round((time.monotonic() - t0) * 1000, 1)
    approve2_received_wallclock = now_stamp()
    save_json(out_dir / "03_approve_card2_response.json", approve2)
    mark(
        "approve2_done",
        ms=approve2_ms,
        interaction=str(card2.get("id")),
        client_received_at=approve2_received_wallclock,
        **brief(approve2),
    )

    # Let the stream settle past the terminal boundary.
    idle_deadline = time.monotonic() + args.terminal_idle_sec
    last_count = -1
    while time.monotonic() < idle_deadline:
        with recorder._lock:
            count = len(recorder.events)
        if count != last_count:
            last_count = count
            idle_deadline = min(idle_deadline, time.monotonic() + 8.0)
        time.sleep(0.5)

    recorder.stop()
    recorder.dump()
    mark("recorder_stopped", frames=len(recorder.frames), events=len(recorder.events))

    # Post-terminal forensics.
    ui_state = request_json("GET", base + "/agent/ui/state", None, 30.0)
    save_json(out_dir / "04_ui_state_after_terminal.json", ui_state)
    ph = ui_state.get("project_history") or {}
    mark(
        "ui_state_after_terminal",
        conversation_messages=[
            {
                "node_id": r.get("node_id"),
                "role": r.get("role"),
                "kind": r.get("message_kind"),
                "content_head": str(r.get("content") or "")[:100],
            }
            for r in (ph.get("conversation_messages") or [])
        ],
    )

    agent_state = request_json("GET", base + "/agent/state", None, 30.0)
    save_json(out_dir / "05_agent_state_after_terminal.json", agent_state)
    goal = agent_state.get("goal") or {}
    mark(
        "agent_state_after_terminal",
        goal_status=goal.get("status"),
        task_state=(goal.get("task") or {}).get("semantic_state"),
    )

    try:
        runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
        save_json(out_dir / "06_runtime_status_after_terminal.json", runtime)
        mark(
            "runtime_status_after_terminal",
            goal_status=(runtime.get("goal") or {}).get("status"),
            continuations=[
                {"id": c.get("id"), "status": c.get("status")} for c in (runtime.get("continuations") or [])
            ],
        )
    except Exception as exc:  # noqa: BLE001
        mark("runtime_status_failed", error=str(exc))

    try:
        audition = request_json(
            "GET",
            base + "/agent/audition/status?conversation_id=" + conversation_id,
            None,
            30.0,
        )
        save_json(out_dir / "07_audition_status_after_terminal.json", audition)
        sessions = audition.get("sessions") or audition.get("session") or {}
        mark("audition_status_after_terminal", sessions=sessions if isinstance(sessions, dict) else str(sessions)[:400])
    except Exception as exc:  # noqa: BLE001
        mark("audition_status_failed", error=str(exc))

    # Re-fetch of a consumed card emulates the WebUI refresh replay (18:59 form).
    try:
        replay = request_json(
            "POST",
            base + "/agent/interaction/respond",
            {"interaction_id": str(card2.get("id", "")), "action_id": "approve", "decision": "approve", "payload": {}},
            args.card_timeout,
        )
        save_json(out_dir / "08_card2_replay_response.json", replay)
        mark("card2_replay", reply=replay.get("reply"), goal_status=replay.get("goal_status"))
    except Exception as exc:  # noqa: BLE001
        mark("card2_replay_failed", error=str(exc))

    save_json(out_dir / "timeline.json", {"started_at": started_at, "finished_at": now_stamp(), "rows": timeline})
    summary = {
        "conversation_id": conversation_id,
        "chat_ms": chat_ms,
        "approve1_ms": approve1_ms,
        "approve2_ms": approve2_ms,
        "card1_id": card1.get("id"),
        "card2_id": card2.get("id"),
        "approve2_reply": approve2.get("reply"),
        "approve2_goal_status": approve2.get("goal_status"),
        "timeline": timeline,
    }
    save_json(out_dir / "summary.json", summary)
    print("FORENSICS_DONE " + json.dumps(summary, ensure_ascii=False), flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
