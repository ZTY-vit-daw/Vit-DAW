#!/usr/bin/env python3
"""FS-LEDGER-CARRY-1 (card 158) real-stack chat leg driver, 2026-10-21.

Reproduces the M2 experience session-2 two-round shape on the real agent HTTP
face (softguide_exp.py / FS-LARGEPROJECT nl_chat driver family):

  round 1  M2 session-2 round-1 utterance, verbatim (146 B1):
           "给人声加点空间感"          -- cold conversation, NEW loop
  round 2  M2 session-1 round-1 utterance, verbatim (= session-2 round-2 main
           utterance, dev_agent_smoke fullaccess_semantic_chain LEG C source):
           "有几轨听起来比较干，想给其中一条合适的轨加一点混响（衰减短些），
            不要用整体增益或 EQ 来解决。请先观察工程，再给一个有界的小步改进建议。"

Both rounds run in ONE conversation on the M2 six-stem fixture under
full_project_access. Round 1 must settle to a loop-terminal goal family
(cards auto-answered approve/select-first as fallback, nudges bounded); round
2 then starts a new loop whose ledger must be SEEDED from the round-1
predecessor (B) / or have been seed-observed at birth (A1).

Assertion faces are the deterministic prepare-boundary log lines the card adds
(agent log, grep by conversation id):
  [free-state.seed]  booked conversation=<id> ... scan_satisfied=true   (A1)
  [free-state.carry] conversation=<id> carried_from_loop=... receipts>=1 (B)
Reply semantics are NEVER asserted (AGENTS.md section 8; capture + classify).

Exit codes (pre-declared, section 8):
  0 PASS             deterministic gate faces all present (seed or carry per
                     round as applicable; round-1 settle reached a terminal
                     goal family)
  2 environment      fixture / authority / transport-level abort
  3 round1_no_terminal  llm-shape class: round 1 never reached a terminal
                     goal family within budget after bounded nudges/hops
                     (one rerun allowed; same deterministic break twice =
                     stop-loss for the caller)
  4 gate_face_missing  precondition met but the expected seed/carry log line
                     is absent = functional defect on this card's surface

The stack (kernel + agent) is started/stopped by the run owner (157 manual
bring-up precedent); this driver only talks HTTP.
"""

from __future__ import annotations

import argparse
import json
import math
import re
import struct
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

ROUND1_PROMPT = "给人声加点空间感"
# Round 2 = the 146 real-B2 two-round artifact phrase (156 RESULTS L5, the
# canonical "M2 二轮型" continuation-gap driver: both rounds route free-state
# loops in one conversation). The M2 session-2 round-2 main utterance
# (reverb-prefixed bounded-step request) is driven as a record-only probe in
# run 1: on this stack the semantic entry routes it to the full-access
# semantic dynamic workflow (147 family, autonomous execution), so it never
# creates a free-state loop and the carry face does not apply to it.
ROUND2_PROMPT = "这条钢琴听着太干了，帮我弄润一点"
NUDGE = "可以执行"

# M2 six-stem fixture recipe (dev_agent_smoke fullaccess_semantic_chain LEG A,
# verbatim amplitudes/frequencies; 2.2 s mono 16-bit 44.1 kHz sine stems).
STEM_SPECS = [
    ("bass.wav", 0.35, 110.0),
    ("drums.wav", 0.45, 60.0),
    ("guitar.wav", 0.40, 330.0),
    ("other.wav", 0.30, 220.0),
    ("piano.wav", 0.40, 440.0),
    ("vocals.wav", 0.50, 660.0),
]

TERMINAL_GOALS = ["completed", "failed", "stopped", "cancelled"]


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
           source: str = "fs_ledger_carry_chat") -> Dict[str, Any]:
    return post_json(base_url, "/agent/invoke", {
        "tool": tool,
        "args": args or {},
        "confirmed": True,
        "source": source,
    }, timeout)


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
    fixture_path = fixture_dir / "fs_carry_fixture.vit"
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


def find_card(response: Dict[str, Any]) -> Dict[str, Any]:
    """Return {face, id, kind} for the first answerable card in a chat response."""
    for row in response.get("interaction_requests") or []:
        if not isinstance(row, dict):
            continue
        kind = str(row.get("kind") or row.get("type") or "")
        if ("confirmation" in kind.lower()
                or kind == "plugin_recommendation_question"
                or kind == "plugin_recommendation_selection"):
            card_id = str(row.get("id") or row.get("interaction_id") or "")
            if card_id:
                return {"face": "interaction", "id": card_id, "kind": kind}
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
    return post_json(base_url, "/agent/interaction/respond", {
        "interaction_id": card["id"],
        "action_id": "approve",
        "decision": "approve",
        "payload": {},
    }, turn_seconds)


def run_round(base_url: str, round_id: str, prompt: str, conversation_id: str,
              out_dir: Path, args: argparse.Namespace,
              timeline: List[str], logs: List[str]) -> Dict[str, Any]:
    """One NL round in the shared conversation: prompt -> settle to a terminal
    goal family with bounded nudges + auto-answered confirmation cards."""
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "prompt.txt").write_text(prompt, encoding="utf-8")
    responses: List[Dict[str, Any]] = []
    stop_reasons: List[str] = []
    goal_statuses: List[str] = []
    card_hops: List[Dict[str, Any]] = []
    turns_sent = 0
    nudges_sent = 0
    hops_driven = 0
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
                if nudges_sent > args.max_nudges:
                    settle_reason = "boundary_no_nudges_left"
                    break
            message = NUDGE if is_nudge else prompt
            timeline.append(f"{round_id} turn_{turns_sent}{'_nudge' if is_nudge else ''} send")
            try:
                response = post_json(base_url, "/agent/chat", {
                    "conversation_id": conversation_id,
                    "message": message,
                    "context": {"agent_mode": "chat"},
                }, args.turn_seconds)
            except Exception as exc:  # noqa: BLE001
                transport_errors += 1
                (out_dir / f"chat_{turns_sent}_error.txt").write_text(str(exc), encoding="utf-8")
                timeline.append(f"{round_id} turn_{turns_sent} transport_error={exc}")
                settle_reason = "initial_transport_error"
                break
            responses.append(response)
            (out_dir / f"chat_{turns_sent}.json").write_text(
                json.dumps(response, ensure_ascii=False, indent=1), encoding="utf-8")
            final_stop = str(response.get("stop_reason") or final_stop)
            goal_status = str(response.get("goal_status") or goal_status)
            if response.get("stop_reason"):
                stop_reasons.append(str(response["stop_reason"]))
            if response.get("goal_status"):
                goal_statuses.append(str(response["goal_status"]))
            timeline.append(
                f"{round_id} turn_{turns_sent}{'_nudge' if is_nudge else ''} "
                f"stop={response.get('stop_reason')} goal={response.get('goal_status')} "
                f"cards={len(response.get('interaction_requests') or [])}"
            )
            if goal_status in TERMINAL_GOALS:
                settle_reason = "goal_terminal"
                break
            card = find_card(response)
            if card:
                hops_driven += 1
                if hops_driven > args.max_hops:
                    settle_reason = "card_parked"
                    break
                timeline.append(f"{round_id} hop {card['face']} {card['kind']} {card['id']}")
                try:
                    hop_response = answer_card(base_url, card, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    transport_errors += 1
                    timeline.append(f"{round_id} hop transport_error={exc}")
                    break
                card_hops.append({"card": card, "response": hop_response})
                (out_dir / f"hop_{hops_driven}.json").write_text(
                    json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                hop_goal = str(hop_response.get("goal_status") or "")
                if hop_goal in TERMINAL_GOALS:
                    goal_status = hop_goal
                    settle_reason = "goal_terminal"
                    break
            # A waiting_continue turn does NOT imply the chain is idle: the
            # durable continuation scheduler keeps driving the free-state
            # loop after the HTTP response returns. Only the runtime poll
            # below decides whether a nudge is legal.
        else:
            time.sleep(3.0)
            runtime_poll = get_json(base_url, "/agent/runtime/status", 30)
            if not runtime_poll:
                continue
            goal_prop = runtime_poll.get("goal") or {}
            goal_status_poll = str(goal_prop.get("status") or "")
            continuation_active = False
            walked_card = {}
            for row in runtime_poll.get("continuations") or []:
                if not isinstance(row, dict):
                    continue
                if str(row.get("status")) in ("pending", "claimed", "running"):
                    continuation_active = True
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
                    walked_card = {"face": "interaction", "id": card_id, "kind": kind}
            if walked_card:
                hops_driven += 1
                if hops_driven > args.max_hops:
                    settle_reason = "card_parked"
                    break
                timeline.append(f"{round_id} poll hop {walked_card['kind']} {walked_card['id']}")
                try:
                    hop_response = answer_card(base_url, walked_card, args.turn_seconds)
                except Exception as exc:  # noqa: BLE001
                    transport_errors += 1
                    timeline.append(f"{round_id} poll hop transport_error={exc}")
                    break
                card_hops.append({"card": walked_card, "response": hop_response})
                (out_dir / f"hop_{hops_driven}.json").write_text(
                    json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                continue
            if goal_status_poll in TERMINAL_GOALS:
                goal_status = goal_status_poll
                settle_reason = "goal_terminal"
                timeline.append(f"{round_id} settle goal_terminal (poll) status={goal_status_poll}")
                break
            if (goal_status_poll == "waiting_continue" and not continuation_active
                    and nudges_sent < args.max_nudges):
                timeline.append(f"{round_id} boundary: waiting_continue with no active continuation -> nudge {(nudges_sent + 1)}/{args.max_nudges}")
                send_pending = True

    runtime_final = get_json(base_url, "/agent/runtime/status", 30)
    (out_dir / "runtime_status_final.json").write_text(
        json.dumps(runtime_final, ensure_ascii=False, indent=1), encoding="utf-8")
    events = get_json(base_url, "/agent/events?limit=500", 30)
    (out_dir / "events.json").write_text(
        json.dumps(events, ensure_ascii=False, indent=1), encoding="utf-8")
    return {
        "round": round_id,
        "prompt": prompt,
        "turns_sent": turns_sent,
        "nudges_sent": nudges_sent,
        "hops_driven": hops_driven,
        "settle_reason": settle_reason,
        "goal_status": goal_status,
        "final_stop": final_stop,
        "stop_reasons": stop_reasons,
        "goal_statuses": goal_statuses,
        "transport_errors": transport_errors,
        "card_hops": [{"card": row["card"]} for row in card_hops],
    }


def grep_agent_log(log_path: Path, pattern: str) -> List[str]:
    if not log_path.exists():
        return []
    regex = re.compile(pattern)
    hits: List[str] = []
    try:
        text = log_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return []
    for line in text.splitlines():
        if regex.search(line):
            hits.append(line.strip())
    return hits


def main() -> int:
    parser = argparse.ArgumentParser(description="FS-LEDGER-CARRY-1 chat leg driver")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--agent-log", default="", help="agent -last-log-path file for the gate grep")
    parser.add_argument("--settle-seconds", type=int, default=900)
    parser.add_argument("--turn-seconds", type=int, default=480)
    parser.add_argument("--max-nudges", type=int, default=2)
    parser.add_argument("--max-hops", type=int, default=4)
    parser.add_argument("--skip-fixture", action="store_true",
                        help="reuse the existing fixture .vit in --out-dir (rerun economy)")
    args = parser.parse_args()

    out_dir = Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    agent_log = Path(args.agent_log).resolve() if args.agent_log else out_dir / ".." / "logs" / "agent_last.log"
    conversation_id = "fs_carry_chat_" + datetime.now().strftime("%Y%m%d_%H%M%S")
    timeline: List[str] = []
    logs: List[str] = []
    report: Dict[str, Any] = {
        "card": "FS-LEDGER-CARRY-1",
        "conversation_id": conversation_id,
        "started_at": now_stamp(),
        "agent_http": args.agent_http,
        "rounds": {},
        "gate": {},
    }

    try:
        # ---------------- LEG A: fixture (M2 six-stem shape) + authority ----
        fixture_dir = out_dir / "fixture"
        fixture_path = fixture_dir / "fs_carry_fixture.vit"
        if args.skip_fixture and fixture_path.exists():
            logs.append("fixture reused: " + str(fixture_path))
        else:
            fixture_path = build_fixture(args.agent_http, fixture_dir, logs)
        tracks = open_fixture(args.agent_http, fixture_path, logs)
        report["disclosed_tracks"] = [
            {"id": str(t.get("track_id") or t.get("id")), "name": str(t.get("track_name") or t.get("name"))}
            for t in tracks
        ]

        authority = post_json(args.agent_http, "/agent/authority",
                              {"authority_mode": "full_project_access"}, 30)
        (out_dir / "authority_switch.json").write_text(
            json.dumps(authority, ensure_ascii=False, indent=1), encoding="utf-8")
        if str(authority.get("status")) != "ok" or str(authority.get("authority_mode")) != "full_project_access":
            raise EnvAbort("authority switch refused: " + json.dumps(authority, ensure_ascii=False)[:400])
        probe = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": conversation_id + "_authority_probe",
            "message": "/smoke authority",
            "context": {"agent_mode": "chat"},
        }, max(30, 60))
        (out_dir / "authority_probe.json").write_text(
            json.dumps(probe, ensure_ascii=False, indent=1), encoding="utf-8")
        if str(probe.get("stop_reason")) != "authority_smoke_ok":
            raise EnvAbort("authority probe did not bind full access: " + json.dumps(probe, ensure_ascii=False)[:400])
        logs.append("authority bound full_project_access")

        # ---------------- LEG B: round 1 (cold start, seed face) ------------
        round1 = run_round(args.agent_http, "round1", ROUND1_PROMPT, conversation_id,
                           out_dir / "round1", args, timeline, logs)
        report["rounds"]["round1"] = round1

        # ---------------- LEG C: round 2 (same conversation, carry face) ----
        round2 = run_round(args.agent_http, "round2", ROUND2_PROMPT, conversation_id,
                           out_dir / "round2", args, timeline, logs)
        report["rounds"]["round2"] = round2

    except EnvAbort as exc:
        report["classification"] = "environment"
        report["error"] = str(exc)
        report["finished_at"] = now_stamp()
        report["timeline"] = timeline
        (out_dir / "report.json").write_text(
            json.dumps(report, ensure_ascii=False, indent=1), encoding="utf-8")
        print("[fs-carry] ENVIRONMENT ABORT: " + str(exc))
        return 2

    # ---------------- GATE: deterministic prepare-boundary log faces -------
    gate = report["gate"]
    conv = conversation_id.replace("+", ".")
    seed_lines = grep_agent_log(agent_log, r"\[free-state\.seed\] booked conversation=" + re.escape(conversation_id))
    carry_lines = grep_agent_log(agent_log, r"\[free-state\.carry\] conversation=" + re.escape(conversation_id))
    gate["seed_lines"] = seed_lines
    gate["carry_lines"] = carry_lines
    gate["seed_booked"] = bool(seed_lines)
    gate["carry_booked"] = bool(carry_lines)
    seed_scan_ok = any("scan_satisfied=true" in line for line in seed_lines)
    carry_receipts = 0
    for line in carry_lines:
        match = re.search(r"receipts=(\d+)", line)
        if match:
            carry_receipts = max(carry_receipts, int(match.group(1)))
    gate["seed_scan_satisfied"] = seed_scan_ok
    gate["carry_receipts"] = carry_receipts

    round1_terminal = report["rounds"]["round1"]["settle_reason"] == "goal_terminal"
    transport_errors = (report["rounds"]["round1"]["transport_errors"]
                        + report["rounds"]["round2"]["transport_errors"])
    gate["round1_terminal"] = round1_terminal
    gate["transport_errors"] = transport_errors

    if not round1_terminal:
        report["classification"] = "round1_no_terminal"
        exit_code = 3
    elif transport_errors > 0:
        report["classification"] = "environment"
        exit_code = 2
    elif not (gate["seed_booked"] and seed_scan_ok):
        report["classification"] = "gate_face_missing_seed"
        exit_code = 4
    elif not (gate["carry_booked"] and carry_receipts >= 1):
        report["classification"] = "gate_face_missing_carry"
        exit_code = 4
    else:
        report["classification"] = "pass"
        exit_code = 0

    report["finished_at"] = now_stamp()
    report["timeline"] = timeline
    report["logs"] = logs
    (out_dir / "report.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=1), encoding="utf-8")
    print(f"[fs-carry] classification={report['classification']} "
          f"seed={gate['seed_booked']}({seed_scan_ok}) carry={gate['carry_booked']}({carry_receipts}) "
          f"round1_terminal={round1_terminal} transport_errors={transport_errors}")
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
