#!/usr/bin/env python3
"""UX-AB-AUDITION-COVERAGE-1 (card 177) real-stack reproduction leg, 2026-10-23.

Adapted from aba174_driver.py (card 174; 146/170 driver family) with the
assertion faces swapped to this card's target: the manual-session 场次⑤ form
(bare utterance "给人声加点空间感", full access, cards auto-answered) must,
when the chain dispatches the direct bus-topology legs (track.send.connect
auto_create + rack_add_node -- the forensically-confirmed non-ticket effect
mutation path), mount an equivalent audition session in the
audition:dynamic:topo_* family (the 174 bracket only covered the semantic
ticket parameter write). Capture only; assertions live in assert_uxab177.py.

Run budget (pre-declared): <=3 rounds; retries only for env interrupts or a
round that never exercises the direct-legs form (no track.send.connect in the
executed surface). Success = assert script exit 0. LLM turns follow the
stack's configured model (deepseek-flash family per 174/175 legs).

Exit codes: 0 = round captured (classification happens in the assert script);
2 = environment-level abort (fixture / authority / transport), exit-2 shape.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import shutil
import struct
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

PROMPT = "给人声加点空间感"
NUDGE = "可以执行"
FIXTURE_SOURCE_170 = Path(
    r"D:\Vit_DAW\coord\runs\FS-NAMED-DISCLOSE-SUPPLY-1\fs_disclose_chat_20261022_214407\driver\fixture"
)

STEM_SPECS = [
    ("bass.wav", 0.35, 110.0),
    ("drums.wav", 0.45, 60.0),
    ("guitar.wav", 0.40, 330.0),
    ("other.wav", 0.30, 220.0),
    ("piano.wav", 0.40, 440.0),
    ("vocals.wav", 0.50, 660.0),
]

TERMINAL_GOALS = ["completed", "failed", "stopped", "cancelled"]
USER_GOALS = ["waiting_confirmation", "waiting_clarification"]


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


def post_json(base_url: str, path: str, payload: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8", errors="replace"))


def get_json(base_url: str, path: str, timeout: float) -> Optional[Dict[str, Any]]:
    try:
        with urllib.request.urlopen(base_url.rstrip("/") + path, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8", errors="replace"))
    except Exception:  # noqa: BLE001 — sampling must survive transient errors
        return None


def invoke(base_url: str, tool: str, args: Optional[Dict[str, Any]], timeout: float,
           source: str = "uxab177_driver") -> Dict[str, Any]:
    return post_json(base_url, "/agent/invoke", {
        "tool": tool,
        "args": args or {},
        "confirmed": True,
        "source": source,
    }, timeout)


def post_judgment(base_url: str, payload: Dict[str, Any], timeout: float = 60.0) -> Dict[str, Any]:
    """POST /agent/audition/judgment without raising on 4xx: capture status+body."""
    request = urllib.request.Request(
        base_url.rstrip("/") + "/agent/audition/judgment",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return {"http_status": response.status,
                    "body": json.loads(response.read().decode("utf-8", errors="replace"))}
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        try:
            body: Dict[str, Any] = json.loads(raw)
        except Exception:  # noqa: BLE001
            body = {"raw": raw}
        return {"http_status": exc.code, "body": body}


def write_stem(path: Path, amplitude: float, frequency: float) -> None:
    sample_rate = 44100
    duration = 2.2
    samples = int(sample_rate * duration)
    frames = bytearray()
    for i in range(samples):
        t = i / sample_rate
        value = amplitude * math.sin(2.0 * math.pi * frequency * t)
        pcm = int(max(-1.0, min(1.0, value)) * 32767.0)
        frames += struct.pack("<h", pcm)
    data_bytes = len(frames)
    header = b"RIFF" + struct.pack("<I", 36 + data_bytes) + b"WAVEfmt " + struct.pack(
        "<IHHIIHH", 16, 1, 1, sample_rate, sample_rate * 2, 2, 16
    ) + b"data" + struct.pack("<I", data_bytes)
    path.write_bytes(header + bytes(frames))


class EnvAbort(RuntimeError):
    pass


def build_fixture(base_url: str, fixture_dir: Path, logs: List[str]) -> Path:
    stems_dir = fixture_dir / "fixture_stems"
    stems_dir.mkdir(parents=True, exist_ok=True)
    for name, amplitude, frequency in STEM_SPECS:
        write_stem(stems_dir / name, amplitude, frequency)
    fixture_path = fixture_dir / "softguide_fixture.vit"
    invoke(base_url, "project.new", {}, 60)
    imported = invoke(base_url, "project.import_folder_as_stems", {
        "folder_path": str(stems_dir),
        "recursive": False,
        "target_policy": "create_tracks",
        "start_time_seconds": 0.0,
        "skip_unreadable": False,
        "command_timeout_ms": 60000,
    }, 120)
    if imported.get("status") != "ok":
        raise EnvAbort("fixture import failed: " + json.dumps(imported, ensure_ascii=False)[:800])
    result = imported.get("result") or {}
    job_id = result.get("analysis_job_id")
    if not job_id:
        job = result.get("analysis_job") or {}
        job_id = job.get("analysis_job_id") or job.get("job_id")
    if not job_id:
        raise EnvAbort("fixture import returned no analysis job id")
    invoke(base_url, "project.audio_analysis_start", {"analysis_job_id": job_id, "interval_ms": 10}, 60)
    deadline = time.monotonic() + 240
    total = 0
    while time.monotonic() < deadline:
        status = invoke(base_url, "project.audio_analysis_status", {"analysis_job_id": job_id, "latest": True}, 60)
        job = (status.get("result") or {}).get("analysis_job") or {}
        total = int(job.get("dad_fact_total_count") or job.get("dad_fact_total") or 0)
        ready = int(job.get("dad_fact_ready_count") or 0)
        state = str(job.get("dad_fact_status") or "")
        if total > 0 and ready >= total and state.lower() == "ready":
            break
        time.sleep(1.0)
    else:
        raise EnvAbort("fixture DAD analysis did not become ready within 240s")
    logs.append(f"fixture DAD ready (tracks={total})")
    saved = invoke(base_url, "project.save_as", {"file_path": str(fixture_path)}, 60)
    if saved.get("status") != "ok":
        raise EnvAbort("fixture save_as failed: " + json.dumps(saved, ensure_ascii=False)[:400])
    invoke(base_url, "project.new", {}, 60)
    return fixture_path


def prepare_fixture(base_url: str, fixture_dir: Path, logs: List[str]) -> Path:
    """Reuse the 170 run's pristine fixture by whole-directory copy; rebuild
    deterministically only when the copy is missing or fails to open."""
    fixture_path = fixture_dir / "softguide_fixture.vit"
    if not fixture_path.exists():
        source_vit = FIXTURE_SOURCE_170 / "softguide_fixture.vit"
        source_stems = FIXTURE_SOURCE_170 / "fixture_stems"
        if source_vit.exists() and source_stems.exists():
            shutil.copytree(source_stems, fixture_dir / "fixture_stems", dirs_exist_ok=True)
            shutil.copy2(source_vit, fixture_path)
            logs.append(f"fixture copied from 170 run: {source_vit}")
        else:
            logs.append("170 fixture source missing; rebuilding deterministically")
            fixture_path = build_fixture(base_url, fixture_dir, logs)
            logs.append(f"fixture built: {fixture_path}")
    else:
        logs.append(f"fixture present: {fixture_path}")
    return fixture_path


def open_fixture(base_url: str, fixture_path: Path, logs: List[str]) -> List[Dict[str, Any]]:
    opened = invoke(base_url, "project.open", {"file_path": str(fixture_path)}, 180)
    if opened.get("status") != "ok":
        raise EnvAbort("project.open failed: " + json.dumps(opened, ensure_ascii=False)[:400])
    time.sleep(4.0)
    state = invoke(base_url, "project.state", {}, 60)
    tracks = (state.get("result") or {}).get("tracks") or []
    if len(tracks) < 6:
        raise EnvAbort(
            "fixture open restored fewer than 6 tracks: "
            + json.dumps([t.get("track_name") or t.get("name") for t in tracks], ensure_ascii=False)
        )
    logs.append("fixture open ok: " + ",".join(str(t.get("track_name") or t.get("name")) for t in tracks[:8]))
    return tracks


def send_topology(base_url: str) -> Dict[str, Any]:
    reply = invoke(base_url, "track.send.list", {}, 60)
    return reply.get("result") or reply


def recommended_action(row: Dict[str, Any]) -> str:
    """The card's own primary/recommended action id (never a cancel); empty
    when the card carries no actions (fall back to the generic approve)."""
    for action in row.get("actions") or []:
        if not isinstance(action, dict):
            continue
        action_id = str(action.get("id") or "")
        if not action_id or action_id == "cancel":
            continue
        if action.get("recommended") is True or action.get("style") == "primary":
            return action_id
    return ""


def find_card(response: Dict[str, Any]) -> Dict[str, Any]:
    for row in response.get("interaction_requests") or []:
        if not isinstance(row, dict):
            continue
        kind = str(row.get("kind") or row.get("type") or "")
        if ("confirmation" in kind.lower()
                or kind == "plugin_recommendation_question"
                or kind == "plugin_recommendation_selection"):
            card_id = str(row.get("id") or row.get("interaction_id") or "")
            if card_id:
                return {"face": "interaction", "id": card_id, "kind": kind,
                        "action": recommended_action(row)}
    if response.get("needs_confirmation") and response.get("plan_id"):
        return {"face": "plan", "id": str(response["plan_id"]), "kind": str(response.get("workflow") or "plan")}
    return {}


def walk_pending_card(base_url: str) -> Dict[str, Any]:
    runtime = get_json(base_url, "/agent/runtime/status", 30)
    if not runtime:
        return {}
    for row in runtime.get("continuations") or []:
        pending = row.get("pending_interaction")
        if not isinstance(pending, dict):
            continue
        kind = str(pending.get("kind") or "")
        if ("confirmation" not in kind.lower()
                and kind != "plugin_recommendation_question"
                and kind != "plugin_recommendation_selection"):
            continue
        card_id = str(pending.get("interaction_id") or pending.get("id") or "")
        if card_id:
            return {"face": "interaction", "id": card_id, "kind": kind}
    return {}


def answer_card(base_url: str, card: Dict[str, Any], turn_seconds: float) -> Dict[str, Any]:
    if card["face"] == "plan":
        return post_json(base_url, "/agent/confirm", {"plan_id": card["id"], "decision": "approve"}, turn_seconds)
    # Prefer the card's own recommended action (plugin-recommendation selects
    # carry candidate ids like select_plugin_candidate_1; plain confirmations
    # carry approve): the server reads decision = firstNonEmpty(decision,
    # action_id) and matches the interaction's own action vocabulary.
    action = str(card.get("action") or "approve")
    return post_json(base_url, "/agent/interaction/respond", {
        "interaction_id": card["id"],
        "action_id": action,
        "decision": action,
        "payload": {},
    }, turn_seconds)


def pull_events(base_url: str, conversation_id: str, attempts: int = 5) -> Optional[Dict[str, Any]]:
    path = f"/agent/events?conversation_id={urllib.request.quote(conversation_id)}&since=0&limit=1000"
    for _ in range(attempts):
        events = get_json(base_url, path, 60)
        if events and events.get("events"):
            return events
        time.sleep(2.0)
    return events


def run_round(base_url: str, round_name: str, prompt: str, fixture_master: Path,
              out_dir: Path, args: argparse.Namespace, session_log: List[str]) -> Dict[str, Any]:
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "prompt.txt").write_text(prompt, encoding="utf-8")
    # Each round opens its own copy of the pristine fixture (authority-input
    # protection: the master .vit in driver/fixture/ is never mutated).
    round_fixture = out_dir / "fixture_open.vit"
    shutil.copy2(fixture_master, round_fixture)

    timeline: List[str] = []
    logs: List[str] = []
    started = time.monotonic()
    signals: Dict[str, Any] = {
        "round": round_name,
        "prompt": prompt,
        "conversation_id": "",
        "started_at": now_stamp(),
    }

    tracks_before = open_fixture(base_url, round_fixture, logs)
    signals["disclosed_tracks"] = [
        {"id": str(t.get("track_id") or t.get("id")), "name": str(t.get("track_name") or t.get("name"))}
        for t in tracks_before
    ]
    sends_before = send_topology(base_url)
    (out_dir / "send_list_before.json").write_text(json.dumps(sends_before, ensure_ascii=False, indent=1), encoding="utf-8")
    signals["send_edges_before"] = len(sends_before.get("sends") or [])

    conversation_id = f"uxab177_{round_name}_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    signals["conversation_id"] = conversation_id

    responses: List[Dict[str, Any]] = []
    stop_reasons: List[str] = []
    goal_statuses: List[str] = []
    executed_commands: List[str] = []
    card_hops: List[Dict[str, Any]] = []
    turns_sent = 0
    nudges_sent = 0
    send_pending = True
    settle_reason = "budget_exhausted"
    goal_status = ""
    final_stop = ""
    transport_errors = 0
    runtime_final: Optional[Dict[str, Any]] = None
    settle_deadline = time.monotonic() + args.settle_seconds

    while time.monotonic() < settle_deadline:
        if send_pending:
            send_pending = False
            turns_sent += 1
            is_nudge = turns_sent > 1
            if is_nudge:
                nudges_sent += 1
            message = NUDGE if is_nudge else prompt
            timeline.append(f"turn_{turns_sent}{'_nudge' if is_nudge else ''} send")
            try:
                response = post_json(base_url, "/agent/chat", {
                    "conversation_id": conversation_id,
                    "message": message,
                    "context": {"agent_mode": "chat"},
                }, args.turn_seconds)
            except Exception as exc:  # noqa: BLE001
                transport_errors += 1
                (out_dir / f"chat_{turns_sent}_error.txt").write_text(str(exc), encoding="utf-8")
                timeline.append(f"turn_{turns_sent} transport_error={exc}")
                if turns_sent == 1:
                    settle_reason = "initial_transport_error"
                    break
                response = None
            if response is None:
                if transport_errors >= 2:
                    settle_reason = "transport_error"
                    break
                send_pending = True
                continue
            responses.append(response)
            (out_dir / f"chat_{turns_sent}.json").write_text(json.dumps(response, ensure_ascii=False, indent=1), encoding="utf-8")
            stop = str(response.get("stop_reason") or "")
            goal = str(response.get("goal_status") or "")
            if stop:
                stop_reasons.append(stop)
                final_stop = stop
            if goal:
                goal_statuses.append(goal)
                goal_status = goal
            for cmd in response.get("executed_kernel_reply") or []:
                if isinstance(cmd, dict):
                    name = str(cmd.get("command_name") or "")
                    if name:
                        executed_commands.append(name)
            timeline.append(f"turn_{turns_sent} stop={stop} goal={goal} needs_confirm={bool(response.get('needs_confirmation'))}")
            card = find_card(response)
            hops = 0
            while card and hops < args.max_hops:
                hops += 1
                try:
                    hop_response = answer_card(base_url, card, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    card_hops.append({"index": hops, "card": card, "error": str(exc)})
                    timeline.append(f"hop_{hops} error={exc}")
                    break
                responses.append(hop_response)
                (out_dir / f"hop_{turns_sent}_{hops}.json").write_text(json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                hop_stop = str(hop_response.get("stop_reason") or "")
                hop_goal = str(hop_response.get("goal_status") or "")
                if hop_stop:
                    stop_reasons.append(hop_stop)
                    final_stop = hop_stop
                if hop_goal:
                    goal_statuses.append(hop_goal)
                    goal_status = hop_goal
                for cmd in hop_response.get("executed_kernel_reply") or []:
                    if isinstance(cmd, dict):
                        name = str(cmd.get("command_name") or "")
                        if name:
                            executed_commands.append(name)
                card_hops.append({"index": hops, "card": card, "stop_reason": hop_stop, "goal_status": hop_goal})
                timeline.append(f"hop_{hops} kind={card.get('kind')} stop={hop_stop} goal={hop_goal}")
                if hop_goal in TERMINAL_GOALS:
                    card = {}
                    break
                card = find_card(hop_response)
            if goal_status in TERMINAL_GOALS or final_stop in ("done", "failed", "cancelled", "user_stop"):
                settle_reason = "goal_terminal"
                break
            if goal_status in USER_GOALS:
                settle_reason = "awaiting_user"
                break
            if final_stop == "observation_budget_exhausted":
                settle_reason = "budget_exhausted"
                break
        time.sleep(5.0)
        runtime = get_json(base_url, "/agent/runtime/status", 30)
        if runtime:
            runtime_final = runtime
        walked = walk_pending_card(base_url)
        if walked:
            hops = 0
            card = walked
            while card and hops < args.max_hops:
                hops += 1
                try:
                    hop_response = answer_card(base_url, card, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    card_hops.append({"index": hops, "card": card, "error": str(exc), "source": "runtime_walk"})
                    break
                responses.append(hop_response)
                (out_dir / f"walkhop_{turns_sent}_{hops}.json").write_text(json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                hop_goal = str(hop_response.get("goal_status") or "")
                if hop_goal:
                    goal_statuses.append(hop_goal)
                    goal_status = hop_goal
                card_hops.append({"index": hops, "card": card, "goal_status": hop_goal, "source": "runtime_walk"})
                timeline.append(f"walkhop_{hops} kind={card.get('kind')} goal={hop_goal}")
                if hop_goal in TERMINAL_GOALS:
                    break
                card = find_card(hop_response)
            if goal_status in TERMINAL_GOALS:
                settle_reason = "goal_terminal"
                break
        if runtime:
            goal = str((runtime.get("goal") or {}).get("status") or "")
            if goal in TERMINAL_GOALS or goal in USER_GOALS:
                goal_status = goal
                goal_statuses.append(goal)
                settle_reason = "goal_terminal" if goal in TERMINAL_GOALS else "awaiting_user"
                timeline.append(f"settle runtime_goal={goal}")
                break
            if goal == "waiting_continue":
                continuation_active = any(
                    str(row.get("status") or "") in ("pending", "claimed", "running")
                    for row in (runtime.get("continuations") or [])
                )
                if not continuation_active:
                    if nudges_sent < args.max_nudges:
                        timeline.append(f"boundary: waiting_continue no continuation -> nudge {nudges_sent + 1}/{args.max_nudges}")
                        send_pending = True
                    else:
                        settle_reason = "boundary_no_nudges_left"
                        timeline.append("boundary: no nudges left; chain parked without a card")
                        break

    signals.update({
        "turns_sent": turns_sent,
        "nudges_sent": nudges_sent,
        "settle_reason": settle_reason,
        "goal_status_final": goal_status,
        "stop_reasons": stop_reasons,
        "final_stop_reason": final_stop,
        "goal_statuses_seen": sorted(set(goal_statuses)),
        "executed_commands": executed_commands,
        "card_hops": card_hops,
        "transport_errors": transport_errors,
        "timeline": timeline,
        "elapsed_s": round(time.monotonic() - started, 1),
    })

    events = pull_events(base_url, conversation_id)
    if events:
        (out_dir / "events.json").write_text(json.dumps(events, ensure_ascii=False, indent=1), encoding="utf-8")
        signals["events_count"] = len(events.get("events") or [])

    time.sleep(2.0)
    sends_after = send_topology(base_url)
    (out_dir / "send_list_after.json").write_text(json.dumps(sends_after, ensure_ascii=False, indent=1), encoding="utf-8")
    signals["send_edges_after"] = len(sends_after.get("sends") or [])

    state_after = invoke(base_url, "project.state", {}, 60)
    (out_dir / "project_state_after.json").write_text(json.dumps(state_after, ensure_ascii=False, indent=1), encoding="utf-8")
    if runtime_final:
        (out_dir / "runtime_status_final.json").write_text(json.dumps(runtime_final, ensure_ascii=False, indent=1), encoding="utf-8")

    # ---- second stage: topology-face extraction + probes (card 177) -------
    # The round's assertion faces live in assert_uxab177.py; this stage only
    # captures: every audition:dynamic:* session mounted this round (topo_* =
    # the direct bus legs' bracket, dyn_* = the ticket bracket), the executed
    # command surface (connect/load legs), and a judgment battery against the
    # topo session (identity refusal + correct POST; the answerable-record
    # slot is one-per-conversation by 174 design, so a later ticket mount may
    # have replaced the topo card -- both outcomes recorded verbatim).
    judgment_stage: Dict[str, Any] = {"conversation_id": conversation_id}
    dynamic_sessions: List[Dict[str, Any]] = []
    if events:
        for event in events.get("events") or []:
            if not isinstance(event, dict) or event.get("type") != "audition.ready":
                continue
            payload = event.get("payload") or {}
            session = payload.get("session") or {}
            sid = str(session.get("session_id") or "")
            if sid.startswith("audition:dynamic:"):
                dynamic_sessions.append({"session": session, "event_seq": event.get("seq")})
    topo_rows = [row for row in dynamic_sessions if str((row["session"] or {}).get("session_id") or "").startswith("audition:dynamic:topo_")]
    ticket_rows = [row for row in dynamic_sessions if row not in topo_rows]
    judgment_stage["dynamic_sessions"] = [
        {"session_id": str((row["session"] or {}).get("session_id") or ""), "event_seq": row.get("event_seq")}
        for row in dynamic_sessions
    ]
    judgment_stage["topo_session_found"] = bool(topo_rows)
    judgment_stage["ticket_session_found"] = bool(ticket_rows)
    (out_dir / "audition_sessions_all.json").write_text(json.dumps(dynamic_sessions, ensure_ascii=False, indent=1), encoding="utf-8")

    executed_command_names = sorted(set(executed_commands))
    judgment_stage["executed_commands"] = executed_command_names
    judgment_stage["connect_leg_dispatched"] = "track.send.connect" in executed_command_names
    judgment_stage["load_leg_dispatched"] = "rack_add_node" in executed_command_names or "rack.add_node" in executed_command_names

    if topo_rows:
        session_payload = topo_rows[-1]["session"]
        session_event_seq = topo_rows[-1].get("event_seq")
        (out_dir / "topo_audition_session.json").write_text(json.dumps(topo_rows, ensure_ascii=False, indent=1), encoding="utf-8")
        turn_id = str(session_payload.get("turn_id") or "")
        round_id = str(session_payload.get("round_id") or "")
        session_id = str(session_payload.get("session_id") or "")
        revision = str(session_payload.get("project_revision") or "")
        identity = {
            "conversation_id": conversation_id,
            "turn_id": turn_id,
            "round_id": round_id,
            "audition_session_id": session_id,
            "project_revision": revision,
        }
        judgment_stage["topo_identity"] = identity

        # (1) wrong turn_id against the topo session -> expect 409.
        wrong_turn = dict(identity)
        wrong_turn["turn_id"] = turn_id + "__wrong"
        wrong_turn["heard_difference"] = "yes"
        wrong_turn["preference"] = "b"
        probe1 = post_judgment(base_url, wrong_turn)
        (out_dir / "judgment_409_identity.json").write_text(json.dumps(
            {"request": wrong_turn, "http_status": probe1["http_status"], "body": probe1["body"]},
            ensure_ascii=False, indent=1), encoding="utf-8")
        judgment_stage["probe_wrong_turn"] = {"expect": 409, "got": probe1["http_status"]}

        # (2) correct-identity topo judgment -> expect 200 while the topo card
        # is the conversation's answerable record; 409 if a later ticket mount
        # replaced it (coexistence boundary, both recorded; the assert script
        # accepts 200-or-409 and requires the 409 body to name the mismatch).
        correct = dict(identity)
        correct["heard_difference"] = "yes"
        correct["preference"] = "b"
        correct["free_text"] = "判定探针：topo 通道选 B，保留这次装载"
        probe2 = post_judgment(base_url, correct)
        (out_dir / "judgment_topo_ok.json").write_text(json.dumps(
            {"request": correct, "http_status": probe2["http_status"], "body": probe2["body"]},
            ensure_ascii=False, indent=1), encoding="utf-8")
        judgment_stage["probe_topo_correct"] = {"expect": "200 or 409(replaced)", "got": probe2["http_status"]}
        outcome = (probe2["body"] or {}).get("semantic_dynamic_audition") or {}
        judgment_stage["topo_judgment_outcome"] = outcome

        # (3) candidate render assets: resolve the topo session's candidate
        # source refs on disk (the payload carries absolute file paths).
        assets = []
        for candidate in session_payload.get("candidates") or []:
            if not isinstance(candidate, dict):
                continue
            ref = str(candidate.get("source_ref") or "")
            row = {"label": str(candidate.get("label") or ""), "source_ref": ref}
            if ref:
                p = Path(ref)
                row["exists"] = p.exists()
                row["size"] = p.stat().st_size if p.exists() else 0
            assets.append(row)
        judgment_stage["topo_candidate_assets"] = assets
        (out_dir / "topo_candidate_assets.json").write_text(json.dumps(assets, ensure_ascii=False, indent=1), encoding="utf-8")

        time.sleep(2.0)
        events_after = pull_events(base_url, conversation_id)
        if events_after:
            (out_dir / "events_after_judgment.json").write_text(
                json.dumps(events_after, ensure_ascii=False, indent=1), encoding="utf-8")
            judgment_stage["events_after_count"] = len(events_after.get("events") or [])

    signals["judgment_stage"] = judgment_stage
    signals["goal_status_final"] = goal_status
    signals["settle_reason"] = settle_reason

    for line in timeline:
        session_log.append(f"[{round_name}] {line}")
    for line in logs:
        session_log.append(f"[{round_name}] {line}")
    for key in ("topo_session_found", "ticket_session_found", "connect_leg_dispatched",
                "load_leg_dispatched", "probe_wrong_turn", "probe_topo_correct"):
        if key in judgment_stage:
            session_log.append(f"[{round_name}] judgment.{key}={json.dumps(judgment_stage[key], ensure_ascii=False)}")
    (out_dir / "variant_summary.json").write_text(json.dumps(signals, ensure_ascii=False, indent=1), encoding="utf-8")
    return signals


def sha256_of(path: Path) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def runtime_goal_status(runtime: Optional[Dict[str, Any]]) -> str:
    goal = (runtime or {}).get("goal")
    if isinstance(goal, dict):
        return str(goal.get("status") or "")
    if isinstance(goal, str):
        return goal
    return ""


def bind_authority(base_url: str, round_dir: Path, session_log: List[str]) -> None:
    """Full-access two-step: release any stale uxab177 goal that blocks the
    switch (a parked round leaves goal waiting_* and the switch answers 409
    authority_mode_change_blocked), then switch + input-chain probe."""
    last_error = ""
    for attempt in range(8):
        runtime = get_json(base_url, "/agent/runtime/status", 30)
        if not runtime:
            raise EnvAbort("runtime status unavailable before authority bind")
        mode = str(runtime.get("authority_mode") or "")
        goal = runtime.get("goal") or {}
        goal_status = str(goal.get("status") or "")
        goal_conversation = str(goal.get("conversation_id") or "")
        blocking = goal_status not in ("", "idle", "completed", "failed", "cancelled", "stopped", "stable")
        if mode == "full_project_access" and not blocking:
            break
        if blocking:
            # The bare runtime projection can lose goal.conversation_id; the
            # task detail (and the continuation rows) still carry it.
            goal_conversation = ""
            detail = get_json(base_url, f"/agent/runtime/status?goal_id={urllib.request.quote(str(goal.get('goal_id') or ''))}", 30)
            task = (detail or {}).get("goal") or {}
            if isinstance(task, dict):
                inner = task.get("task")
                if isinstance(inner, dict):
                    goal_conversation = str(inner.get("conversation_id") or "")
            if not goal_conversation:
                for row in runtime.get("continuations") or []:
                    if isinstance(row, dict) and str(row.get("conversation_id") or "").startswith("uxab177_"):
                        goal_conversation = str(row.get("conversation_id"))
                        break
            if not goal_conversation.startswith("uxab177_"):
                raise EnvAbort(f"goal {goal.get('goal_id')} (conversation={goal_conversation!r}) is blocking the authority switch; refusing to stop a foreign goal")
            stop = post_json(base_url, "/agent/turn/stop", {
                "conversation_id": goal_conversation,
                "reason": "uxab177 round hygiene: release prior round goal before authority switch",
            }, 60)
            session_log.append(f"stale goal released: {goal.get('goal_id')} status={goal_status} conv={goal_conversation} resp=" + json.dumps(stop, ensure_ascii=False)[:160])
            time.sleep(5.0)
            continue
        try:
            switch = post_json(base_url, "/agent/authority", {"authority_mode": "full_project_access"}, 30)
        except urllib.error.HTTPError as exc:
            last_error = f"authority switch http {exc.code}"
            time.sleep(5.0)
            continue
        (round_dir / "authority_switch.json").write_text(json.dumps(switch, ensure_ascii=False, indent=1), encoding="utf-8")
        if str(switch.get("authority_mode")) != "full_project_access":
            last_error = "authority switch refused: " + json.dumps(switch, ensure_ascii=False)[:400]
            time.sleep(5.0)
            continue
        break
    else:
        raise EnvAbort("authority bind failed: " + last_error)

    try:
        probe = post_json(base_url, "/agent/chat", {
            "conversation_id": f"uxab177_authority_{datetime.now().strftime('%Y%m%d_%H%M%S')}",
            "message": "/smoke authority",
            "context": {"agent_mode": "chat"},
        }, 60)
    except Exception as exc:  # noqa: BLE001 — authority face dead = env abort
        raise EnvAbort(f"authority probe transport failure: {exc}") from exc
    (round_dir / "authority_probe.json").write_text(json.dumps(probe, ensure_ascii=False, indent=1), encoding="utf-8")
    if str(probe.get("stop_reason")) != "authority_smoke_ok" or "bound=full_project_access" not in str(probe.get("reply") or ""):
        raise EnvAbort("authority input-chain probe did not bind full access: " + json.dumps(probe, ensure_ascii=False)[:400])
    session_log.append("authority bound full_project_access")


def resume_round(base_url: str, round_name: str, conversation_id: str,
                 out_dir: Path, args: argparse.Namespace, session_log: List[str]) -> Dict[str, Any]:
    """Resume a crashed round's conversation: keep every artifact already on
    disk, continue the settle loop (cards + nudge + experiment-judgment
    fallback), then rebuild variant_summary.json from the primary artifacts."""
    out_dir.mkdir(parents=True, exist_ok=True)
    timeline: List[str] = ["resume start"]
    started = time.monotonic()
    turns_sent = 0
    nudges_sent = 0
    card_hops: List[Dict[str, Any]] = []
    stop_reasons: List[str] = []
    goal_statuses: List[str] = []
    goal_status = ""
    settle_reason = "budget_exhausted"
    runtime_final: Optional[Dict[str, Any]] = None
    settle_deadline = time.monotonic() + args.settle_seconds
    experiment_judgment_sent = False

    while time.monotonic() < settle_deadline:
        walked = walk_pending_card(base_url)
        if walked:
            hops = 0
            card = walked
            while card and hops < args.max_hops:
                hops += 1
                try:
                    hop_response = answer_card(base_url, card, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    card_hops.append({"index": hops, "card": card, "error": str(exc), "source": "resume_walk"})
                    timeline.append(f"resume hop error={exc}")
                    break
                (out_dir / f"resumehop_{hops}.json").write_text(json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                hop_goal = str(hop_response.get("goal_status") or "")
                if hop_goal:
                    goal_statuses.append(hop_goal)
                    goal_status = hop_goal
                card_hops.append({"index": hops, "card": card, "goal_status": hop_goal, "source": "resume_walk"})
                timeline.append(f"resume hop_{hops} kind={card.get('kind')} goal={hop_goal}")
                if hop_goal in TERMINAL_GOALS:
                    break
                card = find_card(hop_response)
        runtime = get_json(base_url, "/agent/runtime/status", 30)
        if runtime:
            runtime_final = runtime
        goal = runtime_goal_status(runtime)
        if goal:
            goal_status = goal
            goal_statuses.append(goal)
        if goal in TERMINAL_GOALS or goal in USER_GOALS:
            settle_reason = "goal_terminal" if goal in TERMINAL_GOALS else "awaiting_user"
            timeline.append(f"resume settle runtime_goal={goal}")
            break
        continuation_active = any(
            str(row.get("status") or "") in ("pending", "claimed", "running", "waiting_interaction")
            for row in ((runtime or {}).get("continuations") or [])
        )
        if continuation_active:
            time.sleep(6.0)
            continue
        if goal == "waiting_continue" or goal == "":
            # No live continuation and no card: if the experiment judgment
            # boundary is pending (experiment audition session ready, judgment
            # requested, no recorded evidence yet) answer it once (retain),
            # else send a bounded nudge.
            events = pull_events(base_url, conversation_id, attempts=2) or {"events": []}
            rows = [e for e in events.get("events") or [] if isinstance(e, dict)]
            experiment_session = None
            for event in reversed(rows):
                if event.get("type") != "audition.ready":
                    continue
                session = (event.get("payload") or {}).get("session") or {}
                sid = str(session.get("session_id") or "")
                if sid and not sid.startswith("audition:dynamic:") and not sid.startswith("audition:mix_tick:"):
                    experiment_session = session
                    break
            recorded_experiment = any(
                e.get("type") == "trajectory.user_judgment.recorded"
                and not str((e.get("payload") or {}).get("turn_id") or "").startswith("dynamic_turn:")
                for e in rows
            )
            if experiment_session and not recorded_experiment and not experiment_judgment_sent:
                experiment_judgment_sent = True
                payload = {
                    "conversation_id": conversation_id,
                    "turn_id": str(experiment_session.get("turn_id") or ""),
                    "round_id": str(experiment_session.get("round_id") or ""),
                    "audition_session_id": str(experiment_session.get("session_id") or ""),
                    "project_revision": str(experiment_session.get("project_revision") or ""),
                    "heard_difference": "yes",
                    "preference": "b",
                    "free_text": "恢复收口：选 B 保留",
                }
                fallback = post_judgment(base_url, payload)
                (out_dir / "experiment_judgment_fallback.json").write_text(json.dumps(
                    {"request": payload, "http_status": fallback["http_status"], "body": fallback["body"]},
                    ensure_ascii=False, indent=1), encoding="utf-8")
                timeline.append(f"resume experiment judgment status={fallback['http_status']}")
                time.sleep(6.0)
                continue
            if nudges_sent < args.max_nudges:
                nudges_sent += 1
                turns_sent += 1
                timeline.append(f"resume nudge {nudges_sent}")
                try:
                    response = post_json(base_url, "/agent/chat", {
                        "conversation_id": conversation_id,
                        "message": NUDGE,
                        "context": {"agent_mode": "chat"},
                    }, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    timeline.append(f"resume nudge error={exc}")
                    continue
                (out_dir / f"chat_{turns_sent + 1}_resume.json").write_text(
                    json.dumps(response, ensure_ascii=False, indent=1), encoding="utf-8")
                stop = str(response.get("stop_reason") or "")
                goal2 = str(response.get("goal_status") or "")
                if stop:
                    stop_reasons.append(stop)
                if goal2:
                    goal_statuses.append(goal2)
                    goal_status = goal2
                timeline.append(f"resume nudge stop={stop} goal={goal2}")
                card = find_card(response)
                hops = 0
                while card and hops < args.max_hops:
                    hops += 1
                    try:
                        hop_response = answer_card(base_url, card, args.turn_seconds)
                    except Exception as exc:  # noqa: BLE001
                        card_hops.append({"index": hops, "card": card, "error": str(exc), "source": "resume_nudge"})
                        break
                    (out_dir / f"resumehop_{turns_sent}_{hops}.json").write_text(
                        json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                    hop_goal = str(hop_response.get("goal_status") or "")
                    if hop_goal:
                        goal_statuses.append(hop_goal)
                        goal_status = hop_goal
                    card_hops.append({"index": hops, "card": card, "goal_status": hop_goal, "source": "resume_nudge"})
                    if hop_goal in TERMINAL_GOALS:
                        break
                    card = find_card(hop_response)
                if goal_status in TERMINAL_GOALS:
                    settle_reason = "goal_terminal"
                    break
            else:
                settle_reason = "resume_boundary_no_nudges_left"
                break
        time.sleep(5.0)

    events = pull_events(base_url, conversation_id)
    if events:
        (out_dir / "events_after_judgment.json").write_text(
            json.dumps(events, ensure_ascii=False, indent=1), encoding="utf-8")
    if runtime_final:
        (out_dir / "runtime_status_final.json").write_text(
            json.dumps(runtime_final, ensure_ascii=False, indent=1), encoding="utf-8")

    signals: Dict[str, Any] = {
        "round": round_name,
        "resumed": True,
        "conversation_id": conversation_id,
        "prompt": PROMPT,
        "settle_reason": settle_reason,
        "goal_status_final": goal_status,
        "stop_reasons": stop_reasons,
        "goal_statuses_seen": sorted(set(goal_statuses)),
        "card_hops": card_hops,
        "resume_timeline": timeline,
        "resume_elapsed_s": round(time.monotonic() - started, 1),
        "started_at": now_stamp(),
    }
    for line in timeline:
        session_log.append(f"[{round_name}][resume] {line}")
    (out_dir / "variant_summary.json").write_text(json.dumps(signals, ensure_ascii=False, indent=1), encoding="utf-8")
    return signals


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--round", default="round1")
    parser.add_argument("--out-root", required=True, help="driver dir of this run (round dir created beneath)")
    parser.add_argument("--resume-conversation", default="",
                        help="resume an existing round conversation (skip fixture/authority; keep existing artifacts)")
    parser.add_argument("--settle-seconds", type=int, default=720)
    parser.add_argument("--turn-seconds", type=int, default=480)
    parser.add_argument("--max-nudges", type=int, default=2)
    parser.add_argument("--max-hops", type=int, default=6)
    args = parser.parse_args()

    out_root = Path(args.out_root).resolve()
    out_root.mkdir(parents=True, exist_ok=True)
    fixture_dir = out_root / "fixture"
    round_dir = out_root / args.round
    round_dir.mkdir(parents=True, exist_ok=True)
    session_log: List[str] = []
    session_summary: Dict[str, Any] = {
        "driver": "uxab177_driver.py",
        "card": "UX-AB-AUDITION-COVERAGE-1",
        "round": args.round,
        "prompt": PROMPT,
        "nudge": NUDGE,
        "started_at": now_stamp(),
    }

    try:
        if args.resume_conversation:
            session_summary["resumed_conversation"] = args.resume_conversation
            result = resume_round(args.agent_http, args.round, args.resume_conversation,
                                  round_dir, args, session_log)
            session_summary["round_result"] = result
            session_summary["status"] = "captured"
            session_summary["finished_at"] = now_stamp()
            (out_root / "session_summary.json").write_text(json.dumps(session_summary, ensure_ascii=False, indent=1), encoding="utf-8")
            (out_root / "session_log.txt").write_text("\n".join(session_log) + "\n", encoding="utf-8")
            print(f"[{args.round}][resume] settle={result['settle_reason']} goal={result['goal_status_final']}", flush=True)
            return 0

        fixture_master = prepare_fixture(args.agent_http, fixture_dir, session_log)

        bind_authority(args.agent_http, round_dir, session_log)

        result = run_round(args.agent_http, args.round, PROMPT, fixture_master, round_dir, args, session_log)
        session_summary["round_result"] = result
        session_summary["status"] = "captured"
        session_summary["finished_at"] = now_stamp()
        (out_root / "session_summary.json").write_text(json.dumps(session_summary, ensure_ascii=False, indent=1), encoding="utf-8")
        (out_root / "session_log.txt").write_text("\n".join(session_log) + "\n", encoding="utf-8")
        print(f"[{args.round}] settle={result['settle_reason']} goal={result['goal_status_final']} "
              f"stop={result['final_stop_reason']} turns={result['turns_sent']} hops={len(result['card_hops'])} "
              f"dynamic_session={result['judgment_stage'].get('dynamic_session_found')}", flush=True)
        return 0
    except EnvAbort as exc:
        session_summary["status"] = "env_abort"
        session_summary["error"] = str(exc)
        session_summary["finished_at"] = now_stamp()
        (out_root / "session_summary.json").write_text(json.dumps(session_summary, ensure_ascii=False, indent=1), encoding="utf-8")
        (out_root / "session_log.txt").write_text("\n".join(session_log) + "\n", encoding="utf-8")
        print(f"ENV_ABORT: {exc}", file=sys.stderr, flush=True)
        return 2


if __name__ == "__main__":
    sys.exit(main())
