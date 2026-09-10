"""E2E-1 layer A driver: chat-chain HTTP contract bed (S1-S12 + layer C stand-in).

Drives the full user-loop workflow over the same HTTP endpoints the WebUI
uses (natural improvement request -> two confirmation cards -> chain terminal
-> cold-read recovery -> stand-in A/B judgment -> settle/adoption -> project
write audit), asserting each contract segment and recording every response,
event frame, and latency. Read-only against production code; writes only to
--out-dir.

RED baseline expectations live in run_chat_chain_e2e_smoke.ps1; this driver
just records segment outcomes plus the evidence needed to compare against the
B1-DIAG four-cause attribution (terminal event emission, terminal message in
conversation graph, cold read, judgment servability, orphan-close audit).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any

from b1_diag_two_card_forensics import EventRecorder, brief, find_interaction, request_json, save_json

TERMINAL_EVENT_TYPES = {"turn.completed", "turn.failed", "turn.stopped"}
NON_TERMINAL_STATUSES = {"waiting_continue"}


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


def request_json_status(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> tuple[int, dict[str, Any] | list[Any] | None, str]:
    """Request that returns (http_status, parsed_body_or_raw_text, error)."""
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            raw = response.read().decode("utf-8", errors="replace")
            status = response.status
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        status = exc.code
    except Exception as exc:  # noqa: BLE001 - transport failure must be recorded, not crash the bed.
        return -1, None, f"{type(exc).__name__}: {exc}"
    try:
        return status, json.loads(raw), ""
    except json.JSONDecodeError:
        return status, None, raw


def file_fingerprint(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {"exists": False}
    data = path.read_bytes()
    stat = path.stat()
    return {
        "exists": True,
        "size": stat.st_size,
        "mtime": datetime.fromtimestamp(stat.st_mtime).astimezone().isoformat(timespec="milliseconds"),
        "sha256": hashlib.sha256(data).hexdigest(),
    }


def project_revision_from_runtime(runtime: dict[str, Any]) -> str:
    for route in runtime.get("capability_routes") or []:
        assessment = (route or {}).get("capacity_assessment") or {}
        facts = assessment.get("observed_facts") or {}
        revision = facts.get("project_revision")
        if revision:
            return str(revision)
    goal = runtime.get("goal") or {}
    history = goal.get("project_history") or {}
    return str(history.get("head") or "")


def draft_project_path_from_ui(ui_state: dict[str, Any]) -> str:
    history = (ui_state.get("agent_plan") or {}).get("project_history") or {}
    return str(history.get("project_path") or "")


def terminal_events_from(recorder: EventRecorder, after_wallclock: str = "") -> list[dict[str, Any]]:
    """Chain-terminal candidates, excluding slice-level completions.

    Slice-level turn.completed fires on every respond return (chat ack,
    approve ack) with goal status waiting_continue. The chain terminal is a
    turn.* event emitted after the second approve whose goal status has left
    waiting_continue (judgment park waiting_confirmation, completed, failed)
    or any scheduler_chain result event.
    """
    found: list[dict[str, Any]] = []
    with recorder._lock:
        for event in recorder.events:
            if str(event.get("type")) not in TERMINAL_EVENT_TYPES:
                continue
            body = str(event.get("body") or "")
            payload = event.get("payload") or {}
            scheduler_chain = bool(payload.get("scheduler_chain")) if isinstance(payload, dict) else False
            status = str(event.get("status") or "")
            created_at = str(event.get("created_at") or "")
            if after_wallclock and created_at and created_at < after_wallclock:
                continue
            if not scheduler_chain and (not body.strip() or status in NON_TERMINAL_STATUSES):
                continue
            found.append(
                {
                    "seq": event.get("seq"),
                    "type": event.get("type"),
                    "status": status,
                    "scheduler_chain": scheduler_chain,
                    "body_head": body[:200],
                    "created_at": created_at,
                }
            )
    return found


def audition_sessions_from(recorder: EventRecorder) -> list[dict[str, Any]]:
    """Complete audition sessions only. The audition.ready event is emitted
    twice (prepare path + kernel telemetry path); the telemetry twin carries a
    thinner session without turn_id/round_id/project_revision, and a judgment
    payload built from it fails field validation before the semantic check."""
    found: list[dict[str, Any]] = []
    with recorder._lock:
        for event in recorder.events:
            if str(event.get("type")) not in {"audition.ready", "audition.prepare", "audition.failed"}:
                continue
            payload = event.get("payload") or {}
            session = payload.get("session") if isinstance(payload, dict) else None
            if not isinstance(session, dict):
                continue
            session_id = str(session.get("session_id") or "")
            turn_id = str(session.get("turn_id") or "")
            round_id = str(session.get("round_id") or "")
            project_revision = str(session.get("project_revision") or "")
            if not (session_id and turn_id and round_id and project_revision):
                continue
            found.append(
                {
                    "seq": event.get("seq"),
                    "type": event.get("type"),
                    "session_id": session_id,
                    "turn_id": turn_id,
                    "round_id": round_id,
                    "project_revision": project_revision,
                    "status": session.get("status"),
                }
            )
    return found


def semantic_history_audit(runtime: dict[str, Any]) -> dict[str, Any]:
    task = ((runtime.get("goal") or {}).get("task") or {})
    semantic = task.get("semantic_state") or {}
    history = semantic.get("history") or []
    orphan_at_judgment = [
        row for row in history
        if row.get("event") == "owner_turn_closed" and row.get("from") == "human_judgment_required"
    ]
    orphan_closes = [row for row in history if row.get("event") == "owner_turn_closed"]
    governed = [row for row in history if str(row.get("reason") or "").find("governed") >= 0]
    return {
        "state": semantic.get("state"),
        "revision": semantic.get("revision"),
        "transition_reason": semantic.get("transition_reason"),
        "history_len": len(history),
        "history": [
            {
                "revision": row.get("revision"),
                "event": row.get("event"),
                "from": row.get("from"),
                "to": row.get("to"),
                "reason": (str(row.get("reason") or "") or "")[:160],
            }
            for row in history
        ],
        "owner_turn_closed_at_human_judgment_required": len(orphan_at_judgment),
        "owner_turn_closed_total": len(orphan_closes),
        "ungoverned_close_markers": len(orphan_closes) - len(governed),
    }


def actionable_interaction_ids(runtime: dict[str, Any]) -> list[str]:
    ids: list[str] = []
    for continuation in runtime.get("continuations") or []:
        pending = (continuation or {}).get("pending_interaction") or {}
        interaction_id = str(pending.get("id") or pending.get("interaction_id") or "")
        if interaction_id:
            ids.append(interaction_id)
    return ids


CARD_EVENT_TYPES = {"interaction.pending", "mix_tick.pending", "mix_treatment.pending"}


def pending_card_from_events(recorder: EventRecorder, kinds: set[str] | None) -> dict[str, Any] | None:
    with recorder._lock:
        for event in recorder.events:
            if str(event.get("type")) not in CARD_EVENT_TYPES:
                continue
            payload = event.get("payload") or {}
            interaction = payload if payload.get("interaction_id") or payload.get("id") else (payload.get("interaction") or {} if isinstance(payload.get("interaction"), dict) else {})
            interaction_id = str(interaction.get("interaction_id") or interaction.get("id") or payload.get("interaction_id") or "")
            kind = str(interaction.get("kind") or interaction.get("type") or payload.get("kind") or "")
            if interaction_id and (kinds is None or kind in kinds or not kinds):
                return {"id": interaction_id, "kind": kind}
    return None


def pending_card_from_runtime(base: str, kinds: set[str] | None, goal_id: str = "") -> dict[str, Any] | None:
    """Goal-scoped: the runtime continuations list is global, so a fresh
    conversation must not adopt a previous conversation's stale pending card
    (the RED park leaves one behind; E2E1 official run 20260909_213930)."""
    runtime = request_json("GET", base + "/agent/runtime/status", None, 15.0)
    for continuation in runtime.get("continuations") or []:
        if str((continuation or {}).get("status")) != "waiting_interaction":
            continue
        pending = (continuation or {}).get("pending_interaction") or {}
        interaction_id = str(pending.get("interaction_id") or pending.get("id") or "")
        kind = str(pending.get("kind") or pending.get("type") or "")
        if goal_id and str(pending.get("goal_id") or "") != goal_id:
            continue
        if interaction_id and (kinds is None or not kinds or kind in kinds):
            return {"id": interaction_id, "kind": kind}
    return None


def wait_for_card(base: str, recorder: EventRecorder, kinds: set[str] | None, deadline: float, goal_id: str = "") -> dict[str, Any] | None:
    """Card delivery has three surfaces: HTTP response body (checked by the
    caller), card events (conversation-scoped), and the runtime continuation
    pending_interaction (goal-scoped; the WebUI's own fallback source)."""
    next_runtime_poll = 0.0
    while time.monotonic() < deadline:
        card = pending_card_from_events(recorder, kinds)
        if card:
            return card
        if time.monotonic() >= next_runtime_poll:
            next_runtime_poll = time.monotonic() + 2.0
            try:
                card = pending_card_from_runtime(base, kinds, goal_id)
            except Exception:  # noqa: BLE001 - transient runtime poll failures must not kill the wait
                card = None
            if card:
                return card
        time.sleep(0.4)
    return None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--message", default="请检查当前工程是否有什么问题吗")
    parser.add_argument("--branch", choices=["A", "B"], default="A", help="stand-in judgment branch (layer C)")
    parser.add_argument("--chat-budget", type=float, default=90.0, help="S2")
    parser.add_argument("--approve-budget", type=float, default=120.0, help="S3/S4")
    parser.add_argument("--terminal-event-budget", type=float, default=180.0, help="S5")
    parser.add_argument("--graph-budget", type=float, default=30.0, help="S6/S7 polling")
    parser.add_argument("--judgment-budget", type=float, default=10.0, help="S8")
    parser.add_argument("--settle-budget", type=float, default=60.0, help="S9")
    parser.add_argument("--terminal-idle-sec", type=float, default=25.0)
    parser.add_argument("--approve-pacing", type=float, default=5.0, help="human-scale pause between card visibility and approve POST (engine drain window); latency windows start at the POST")
    parser.add_argument("--poll-interval", type=float, default=0.25)
    parser.add_argument("--kernel-project", default="", help="isolated kernel default_project.xml (pollution guard, hashed before/after)")
    parser.add_argument("--expect-red", action="store_true", help="evaluate segment shape against the RED baseline expectation")
    parser.add_argument("--f1-fixed", action="store_true", help="evaluate segment shape against the post-F1 expectation: S8 judgment servable + S11 zero orphan closes are green, S9/S10 observed either way, F2/F3-owned segments keep their red state")
    parser.add_argument("--f2-fixed", action="store_true", help="evaluate segment shape against the post-F2 expectation (implies the F1 shape): S5/S6/S7 scheduled-variant terminal delivery + graph persistence + cold-read recovery are green on top of S8/S11; S9/S10 stay honestly observed")
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    base = args.agent_http.rstrip("/")
    started_at = now_stamp()
    timeline: list[dict[str, Any]] = []
    segments: dict[str, dict[str, Any]] = {}
    latencies: dict[str, Any] = {}

    def mark(label: str, **fields: Any) -> None:
        row = {"wallclock": now_stamp(), "label": label, **fields}
        timeline.append(row)
        print(f"[{row['wallclock']}] {label} " + json.dumps({k: v for k, v in fields.items()}, ensure_ascii=False), flush=True)

    def record(segment: str, ok: bool, expected: str, **evidence: Any) -> None:
        row = {"segment": segment, "ok": ok, "expected": expected, "wallclock": now_stamp(), **evidence}
        segments[segment] = row
        mark("segment_" + segment, ok=ok, expected=expected, **{k: v for k, v in evidence.items() if k != "history"})

    # --- pre-state snapshots -------------------------------------------------
    kernel_project = Path(args.kernel_project) if args.kernel_project else None
    kernel_before = file_fingerprint(kernel_project) if kernel_project else {"exists": False, "note": "not provided"}

    request_json("GET", base + "/health", None, 5.0)
    agent_state_before = request_json("GET", base + "/agent/state", None, 30.0)
    save_json(out_dir / "00_agent_state_before.json", agent_state_before)
    shadow = agent_state_before.get("shadow") or {}
    runtime_before = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    save_json(out_dir / "00_runtime_status_before.json", runtime_before)
    revision_before = project_revision_from_runtime(runtime_before)
    record("S1", bool(shadow.get("initialized")), "green", shadow_initialized=shadow.get("initialized"), track_count=shadow.get("track_count"), project_revision_before=revision_before)

    conversation_id = f"e2e1_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    recorder = EventRecorder(base, conversation_id, args.poll_interval, out_dir / "event_recording.json")
    recorder.start()
    mark("recorder_started", conversation=conversation_id)

    # --- S2: natural-language instruction accepted, first card present -------
    t0 = time.monotonic()
    chat = request_json(
        "POST",
        base + "/agent/chat",
        {"conversation_id": conversation_id, "message": args.message, "context": {"agent_mode": "chat"}},
        args.chat_budget,
    )
    latencies["S2_chat_ms"] = round((time.monotonic() - t0) * 1000, 1)
    save_json(out_dir / "01_chat_response.json", chat)
    mark("chat_done", ms=latencies["S2_chat_ms"], **brief(chat))
    card1 = find_interaction(chat, "improvement_proposal_confirmation") or find_interaction(chat)
    # The first slice may return at limit_reached while the proposal is still
    # forming in the background; the card then surfaces via card events or the
    # runtime continuation pending_interaction (goal-scoped, WebUI fallback).
    conversation_goal_id = str(chat.get("goal_id") or "")
    card1_wait_deadline = time.monotonic() + max(0.0, args.chat_budget - latencies["S2_chat_ms"] / 1000.0)
    if card1 is None:
        card1 = wait_for_card(base, recorder, None, card1_wait_deadline, conversation_goal_id)
    latencies["S2_card1_wait_ms"] = round((time.monotonic() - t0) * 1000, 1)
    if card1 is None:
        record("S2", False, "green", note="no interaction card in chat response, card events, or runtime pending within budget", goal_status=chat.get("goal_status"), stop_reason=chat.get("stop_reason"))
        recorder.stop()
        recorder.dump()
        save_json(out_dir / "timeline.json", {"rows": timeline})
        save_json(out_dir / "e2e_layer_a_report.json", build_report(started_at, conversation_id, segments, latencies, timeline, args.expect_red, f1_fixed=args.f1_fixed, f2_fixed=args.f2_fixed))
        return 2
    card1_id = str(card1.get("id"))
    record("S2", True, "green", interaction=card1_id, kind=card1.get("kind", card1.get("type")), chat_ms=latencies["S2_chat_ms"], card1_wait_ms=latencies["S2_card1_wait_ms"])

    # Human-scale pacing before clicking approve: the observation phase's
    # render/waveform work can still hold the kernel engine when the card
    # surfaces; a human does not click in milliseconds (dbg2 evidence:
    # "start D1-S1 before render: Engine is busy rendering" collapsed the
    # experiment at a 130ms approve). Latency windows below start at the POST.
    if args.approve_pacing > 0:
        time.sleep(args.approve_pacing)

    # --- S3: approve card 1 ---------------------------------------------------
    t0 = time.monotonic()
    approve1 = request_json(
        "POST",
        base + "/agent/interaction/respond",
        {"interaction_id": card1_id, "action_id": "approve", "decision": "approve", "payload": {}},
        args.approve_budget,
    )
    latencies["S3_approve1_ms"] = round((time.monotonic() - t0) * 1000, 1)
    save_json(out_dir / "02_approve_card1_response.json", approve1)
    mark("approve1_done", ms=latencies["S3_approve1_ms"], interaction=card1_id, **brief(approve1))
    record("S3", True, "green", interaction=card1_id, approve_ms=latencies["S3_approve1_ms"], stop_reason=approve1.get("stop_reason"))

    # --- S4: approve card 2, chain continues ----------------------------------
    card2 = find_interaction(approve1, "mix_tick_confirmation") or find_interaction(approve1, "mix_treatment_confirmation")
    if card2 is None:
        card2 = wait_for_card(base, recorder, {"mix_tick_confirmation", "mix_treatment_confirmation"}, time.monotonic() + 45.0, conversation_goal_id)
    if card2 is None:
        candidate = wait_for_card(base, recorder, None, time.monotonic() + 15.0, conversation_goal_id)
        if candidate and str(candidate.get("id")) != card1_id:
            card2 = candidate
    if card2 is None:
        record("S4", False, "green", note="no mix_tick card after approve1")
        recorder.stop()
        recorder.dump()
        save_json(out_dir / "timeline.json", {"rows": timeline})
        save_json(out_dir / "e2e_layer_a_report.json", build_report(started_at, conversation_id, segments, latencies, timeline, args.expect_red, f1_fixed=args.f1_fixed, f2_fixed=args.f2_fixed))
        return 3
    card2_id = str(card2.get("id"))
    if args.approve_pacing > 0:
        time.sleep(args.approve_pacing)
    # Terminal-event time anchor: taken BEFORE the approve2 POST. In the inline
    # variant the terminal turn.completed fires while the approve2 respond is
    # still executing (before the client sees the response), so a post-response
    # anchor would exclude it.
    approve2_anchor_wallclock = now_stamp()
    t0 = time.monotonic()
    approve2 = request_json(
        "POST",
        base + "/agent/interaction/respond",
        {"interaction_id": card2_id, "action_id": "approve", "decision": "approve", "payload": {}},
        args.approve_budget,
    )
    latencies["S4_approve2_ms"] = round((time.monotonic() - t0) * 1000, 1)
    approve2_done = time.monotonic()
    save_json(out_dir / "03_approve_card2_response.json", approve2)
    mark("approve2_done", ms=latencies["S4_approve2_ms"], interaction=card2_id, **brief(approve2))
    record("S4", True, "green", interaction=card2_id, approve_ms=latencies["S4_approve2_ms"], stop_reason=approve2.get("stop_reason"), note="inline terminal if stop=done")

    # --- S5: terminal event reaches the stream --------------------------------
    terminal_seen: list[dict[str, Any]] = []
    deadline = time.monotonic() + args.terminal_event_budget
    while time.monotonic() < deadline:
        terminal_seen = terminal_events_from(recorder, after_wallclock=approve2_anchor_wallclock)
        if terminal_seen:
            break
        time.sleep(0.5)
    latencies["S5_terminal_event_wait_ms"] = round((time.monotonic() - approve2_done) * 1000, 1)
    record(
        "S5",
        bool(terminal_seen),
        "red",
        budget_s=args.terminal_event_budget,
        waited_ms=latencies["S5_terminal_event_wait_ms"],
        terminal_events=terminal_seen,
        note="inline variant emits a non-scheduler_chain turn.completed; scheduled variant emits nothing (B1 Q1)",
    )

    # Let the background settle so the settle slice / orphan close lands.
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

    # --- S6: terminal message in the conversation graph -----------------------
    ui_state = request_json("GET", base + "/agent/ui/state", None, 30.0)
    save_json(out_dir / "04_ui_state_after_terminal.json", ui_state)
    messages = ((ui_state.get("project_history") or {}).get("conversation_messages")) or []
    terminal_body = str((terminal_seen[0] or {}).get("body_head") or "") if terminal_seen else ""
    approve2_reply = str(approve2.get("reply") or "")
    graph_has_terminal = False
    approve2_was_inline_terminal = str(approve2.get("stop_reason") or "") == "done"
    for row in messages:
        content = str(row.get("content") or "")
        if not content:
            continue
        if terminal_body and terminal_body[:60] and terminal_body[:60] in content:
            graph_has_terminal = True
            break
        # Only trust the respond body as the terminal when the inline variant
        # actually finished the chain inside approve2 (stop=done); the scheduled
        # variant's body is a "still working" ack slice, not the terminal.
        if approve2_was_inline_terminal and approve2_reply and approve2_reply[:60] and approve2_reply[:60] in content and str(row.get("role")) == "assistant":
            graph_has_terminal = True
            break
    record(
        "S6",
        graph_has_terminal,
        "red",
        budget_s=args.graph_budget,
        message_rows=[{"node_id": r.get("node_id"), "role": r.get("role"), "kind": r.get("message_kind"), "head": str(r.get("content") or "")[:80]} for r in messages],
        note="settle slice writes no vit node (B1 Q3)",
    )

    # --- S7: cold read recovery ----------------------------------------------
    cold_ui = request_json("GET", base + "/agent/ui/state", None, 30.0)
    save_json(out_dir / "05_ui_state_cold_read.json", cold_ui)
    cold_runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    save_json(out_dir / "06_runtime_status_after.json", cold_runtime)
    actionable = actionable_interaction_ids(cold_runtime)
    consumed_still_actionable = [cid for cid in (card1_id, card2_id) if cid in actionable]
    status, replay_body, replay_error = request_json_status(
        "POST",
        base + "/agent/interaction/respond",
        {"interaction_id": card2_id, "action_id": "approve", "decision": "approve", "payload": {}},
        args.approve_budget,
    )
    save_json(out_dir / "07_card2_replay_response.json", {"status": status, "body": replay_body, "error": replay_error})
    replay_rejected = status == 200 and "已处理" in str((replay_body or {}).get("reply") or "")
    record(
        "S7",
        graph_has_terminal and not consumed_still_actionable and replay_rejected,
        "red",
        consumed_still_actionable=consumed_still_actionable,
        replay_http_status=status,
        replay_rejected=replay_rejected,
        note="terminal must survive cold read; consumed cards must not be actionable",
    )

    # --- S8: A/B judgment servable --------------------------------------------
    sessions = audition_sessions_from(recorder)
    mark("audition_sessions_from_events", sessions=sessions[-2:])
    ui_state_path_for_draft = draft_project_path_from_ui(ui_state)
    draft_before_judgment = file_fingerprint(Path(ui_state_path_for_draft)) if ui_state_path_for_draft else {"exists": False, "note": "draft path unavailable"}
    if sessions:
        session = sessions[-1]
        judgment_payload = {
            "conversation_id": conversation_id,
            "turn_id": session.get("turn_id"),
            "round_id": session.get("round_id"),
            "audition_session_id": session.get("session_id"),
            "project_revision": str(session.get("project_revision") or ""),
            # The stand-in statement must ride the legal enum (yes/no/unsure):
            # the free-text stand-in string never passed evidence validation and
            # was only masked while the F1 orphan close killed the POST earlier
            # in the bind path (20260910_091340: 409 unsupported
            # heard_difference once S8 started reaching the validator).
            "heard_difference": "yes",
            "preference": args.branch.lower(),
            "reason_tags": ["stand_in_e2e"],
            "free_text": "E2E stand-in judgment (machine walks the state machine; human-ear verdict is out of bed scope)",
        }
        t0 = time.monotonic()
        judgment_status, judgment_body, judgment_error = request_json_status(
            "POST", base + "/agent/audition/judgment", judgment_payload, args.judgment_budget
        )
        latencies["S8_judgment_ms"] = round((time.monotonic() - t0) * 1000, 1)
        save_json(out_dir / "08_judgment_response.json", {"status": judgment_status, "body": judgment_body, "error": judgment_error, "payload": judgment_payload})
        record(
            "S8",
            judgment_status == 200,
            "red",
            http_status=judgment_status,
            error_head=str((judgment_body or {}).get("error") or judgment_error or "")[:200],
            judgment_ms=latencies["S8_judgment_ms"],
        )
    else:
        judgment_status = -1
        record("S8", False, "red", note="no audition.ready/prepare event captured; judgment identity unavailable", http_status=None)

    # --- S9 (layer C): settle + adoption (parameterized branch) ----------------
    if judgment_status == 200 and sessions:
        session = sessions[-1]
        candidate_id = "candidate-a" if args.branch == "A" else "candidate-b"
        t0 = time.monotonic()
        apply_status, apply_body, apply_error = request_json_status(
            "POST",
            base + "/agent/audition/apply_candidate",
            {"conversation_id": conversation_id, "audition_session_id": session.get("session_id"), "candidate_id": candidate_id},
            args.settle_budget,
        )
        latencies["S9_apply_ms"] = round((time.monotonic() - t0) * 1000, 1)
        save_json(out_dir / "09_apply_candidate_response.json", {"status": apply_status, "body": apply_body, "error": apply_error})
        settled_runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
        save_json(out_dir / "10_runtime_status_after_settle.json", settled_runtime)
        semantic = ((settled_runtime.get("goal") or {}).get("task") or {}).get("semantic_state") or {}
        honest_close = args.branch == "B"
        record(
            "S9",
            apply_status == 200 and (semantic.get("terminal") is True),
            "red",
            branch=args.branch,
            candidate_id=candidate_id,
            http_status=apply_status,
            apply_ms=latencies.get("S9_apply_ms"),
            task_state=semantic.get("state"),
            terminal=semantic.get("terminal"),
            note="branch B keeps treatment then closes honestly; branch A rolls back to baseline checkpoint",
        )
    else:
        record("S9", False, "red", note="blocked: judgment not servable (S8 red) - settle/adoption unreachable", branch=args.branch)

    # --- S10: project write (mtime/revision advance) ---------------------------
    revision_after = project_revision_from_runtime(cold_runtime)
    ui_state_path = draft_project_path_from_ui(ui_state)
    draft_path = Path(ui_state_path) if ui_state_path else None
    draft_fingerprint = file_fingerprint(draft_path) if draft_path else {"exists": False, "note": "draft path unavailable"}
    draft_advanced_after_judgment = (
        draft_fingerprint.get("exists")
        and draft_before_judgment.get("exists")
        and draft_fingerprint.get("sha256") != draft_before_judgment.get("sha256")
    )
    kernel_after = file_fingerprint(kernel_project) if kernel_project else {"exists": False, "note": "not provided"}
    wrote = (revision_before != revision_after) or draft_advanced_after_judgment
    if judgment_status != 200:
        # Adoption is gated behind the judgment; without it no project write may
        # be credited (kernel autosave / conversation nodes are not adoption).
        wrote = False
    record(
        "S10",
        bool(wrote),
        "red",
        revision_before=revision_before,
        revision_after=revision_after,
        draft_project=str(draft_path),
        draft_before_judgment=draft_before_judgment,
        draft_after=draft_fingerprint,
        kernel_isolated_fingerprint=kernel_after,
        note="adoption gated behind judgment; blocked when S8 red",
    )

    # --- S11: semantic history conservation audit ------------------------------
    audit = semantic_history_audit(cold_runtime)
    record(
        "S11",
        audit["owner_turn_closed_at_human_judgment_required"] == 0 and audit["owner_turn_closed_total"] == 0,
        "red",
        audit=audit,
        note="zero owner_turn_closed@human_judgment_required and zero orphan closes expected (B1 Q2)",
    )

    save_json(out_dir / "timeline.json", {"started_at": started_at, "finished_at": now_stamp(), "rows": timeline})
    record("S12", True, "green", artifacts=str(out_dir), note="event frame recording + responses + report written")
    chain_variant = "inline" if str(approve2.get("stop_reason") or "") == "done" else "scheduled"
    report = build_report(started_at, conversation_id, segments, latencies, timeline, args.expect_red, chain_variant, f1_fixed=args.f1_fixed, f2_fixed=args.f2_fixed)
    save_json(out_dir / "e2e_layer_a_report.json", report)
    print("LAYER_A_DONE " + json.dumps(report.get("shape_summary"), ensure_ascii=False), flush=True)
    return 0 if report["shape_summary"]["matches_expectation"] else 1


def build_report(started_at: str, conversation_id: str, segments: dict[str, dict[str, Any]], latencies: dict[str, Any], timeline: list[dict[str, Any]], expect_red: bool, variant: str = "scheduled", f1_fixed: bool = False, f2_fixed: bool = False) -> dict[str, Any]:
    # Card table (written from the B1 scheduled-variant repro): S5-S8 red.
    # The inline variant (approve2 stop=done) legitimately delivers the
    # terminal through the respond body + finalize node (B1 Q1 documents both
    # variants), so S5/S6/S7 may be green there while the fourth cause (S8
    # judgment dead + S11 orphan close) stays red in both.
    card_expected_green = {"S1", "S2", "S3", "S4", "S12"}
    card_expected_red = {"S5", "S6", "S7", "S8", "S9", "S10", "S11"}
    if variant == "inline":
        variant_green = card_expected_green | {"S5", "S6", "S7"}
        variant_red = {"S8", "S9", "S10", "S11"}
    else:
        variant_green = card_expected_green
        variant_red = card_expected_red
    # Post-F1 shape (task card 2026-09-10-F1): the judgment-boundary exemption
    # turns S8 (judgment POST servable) and S11 (zero owner_turn_closed) green
    # in both variants. S9/S10 ride the settle/adoption path behind the
    # judgment and are recorded honestly either way ("应随之或部分绿，如实记录");
    # the F2-owned terminal delivery segments (S5/S6/S7 scheduled-variant)
    # keep their red state until B1-F2 lands.
    f1_green = set()
    f1_red = set()
    f1_either = set()
    if f1_fixed:
        f1_green = {"S8", "S11"}
        f1_either = {"S9", "S10"}
        if variant == "inline":
            f1_green = f1_green | card_expected_green | {"S5", "S6", "S7"}
        else:
            f1_green = f1_green | card_expected_green
            f1_red = {"S5", "S6", "S7"}
    # Post-F2 shape (task card 2026-09-10-F2): the scheduler delivery gate also
    # delivers the answerable waiting park and persists the terminal reply as a
    # conversation-graph vit node, so S5 (terminal event), S6 (graph message)
    # and S7 (cold-read recovery) turn green on the scheduled variant too.
    # Builds on the F1 shape: S8/S11 stay required-green; S9/S10 (settle/
    # adoption behind the judgment, D1-S1 correct-semantics S9 red documented
    # in the F1 receipt) stay honestly observed either way.
    f2_green = set()
    f2_either = set()
    if f2_fixed:
        f2_green = {"S1", "S2", "S3", "S4", "S12", "S8", "S11", "S5", "S6", "S7"}
        f2_either = {"S9", "S10"}
    card_mismatches: list[str] = []
    variant_mismatches: list[str] = []
    # B3 预授权（任务卡 2026-09-09-MANTEST-2-B3）：默认档对齐 -ExpectF2Fixed
    # 档口径——S9/S10（判定后结算/采纳路径）受 D1-S1 正确语义支配（判定即结
    # 算，apply=非法二次变更 409；F1 收到单 run4 实证红），默认档如实记录两可，
    # 不作为门。
    default_either = {"S9", "S10"}
    for name, row in segments.items():
        if f2_fixed:
            # The F2 shape table covers all twelve segments: S1-S8+S11+S12
            # required green, S9/S10 honestly observed either way.
            if name in f2_green and not row["ok"]:
                variant_mismatches.append(f"{name} expected green (f2_fixed/{variant}), got red")
            continue
        if f1_fixed:
            if name in f1_green and not row["ok"]:
                variant_mismatches.append(f"{name} expected green (f1_fixed/{variant}), got red")
            elif name in f1_red and row["ok"]:
                variant_mismatches.append(f"{name} expected red (f1_fixed/{variant}), got green")
            continue
        if not expect_red:
            if name in default_either:
                continue
            if not row["ok"]:
                variant_mismatches.append(f"{name} expected green, got red")
        else:
            if name in variant_green and not row["ok"]:
                variant_mismatches.append(f"{name} expected green ({variant}), got red")
            elif name in variant_red and row["ok"]:
                variant_mismatches.append(f"{name} expected red ({variant}), got green")
            if name in card_expected_green and not row["ok"]:
                card_mismatches.append(f"{name} expected green, got red")
            elif name in card_expected_red and row["ok"]:
                card_mismatches.append(f"{name} expected red (card table), got green")
    shape_note = "inline variant delivers S5/S6/S7 through the respond body per B1 Q1; scheduled variant matches the card table directly"
    if f2_fixed:
        observed_either = {name: bool(segments[name]["ok"]) for name in sorted(f2_either) if name in segments}
        shape_note = (
            "post-F2 shape: S5 terminal event + S6 graph persistence + S7 cold-read recovery green on the "
            f"scheduled variant, on top of the F1 shape (S8/S11); S9/S10 observed honestly ({observed_either})"
        )
    elif f1_fixed:
        observed_either = {name: bool(segments[name]["ok"]) for name in sorted(f1_either) if name in segments}
        shape_note = (
            "post-F1 shape: S8 judgment servable + S11 zero orphan closes required green; "
            f"S9/S10 observed honestly ({observed_either}); "
            "F2-owned terminal delivery keeps its pre-F2 state"
        )
    return {
        "layer": "A",
        "started_at": started_at,
        "finished_at": now_stamp(),
        "conversation_id": conversation_id,
        "chain_variant": variant,
        "segments": segments,
        "latencies_ms": latencies,
        "shape_summary": {
            "expect_red": expect_red,
            "f1_fixed": f1_fixed,
            "f2_fixed": f2_fixed,
            "variant": variant,
            "card_table_mismatches": card_mismatches,
            "variant_aware_mismatches": variant_mismatches,
            "matches_expectation": (not variant_mismatches),
            "card_table_exact_match": (not card_mismatches) and variant == "scheduled" and not f1_fixed and not f2_fixed,
            "note": shape_note,
        },
        "timeline_rows": len(timeline),
    }


if __name__ == "__main__":
    raise SystemExit(main())
