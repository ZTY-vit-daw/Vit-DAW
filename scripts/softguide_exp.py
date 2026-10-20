#!/usr/bin/env python3
"""SEMANTIC-TIMEFX-SOFTGUIDE-EXP-1 driver (card 146 experiment leg, 2026-10-20).

Drives the real agent HTTP face (the b9 / FS-LARGEPROJECT-SMOKE nl_chat family)
for the 146 soft-guide experiment: per variant, ONE fresh conversation on the
M2 six-stem fixture (spv1_p06 shape) under full_project_access, with the
observation-prefixed utterance from the pre-declared protocol
(coord/reports/2026-10-20-146-timefx-design-and-experiment.md §一.C).

The driver is evidence capture + signal extraction only: it never asserts
reply semantics. Per-variant artifacts: prompt, chat turns, hop responses,
events, runtime status, track.send.list before/after, project.state after,
conversation graph copy, and a signals summary the classifier reads.
Interaction cards under full access are auto-answered (approve / select-first)
as a FALLBACK and every hop is recorded (147 merged: the three confirmation
surfaces should not card under full access).

Exit code: 0 = every planned variant produced a captured round (even a
classified-failure one); 2 = environment-level abort before/at a variant
(fixture failure, authority failure, transport dead).
"""

from __future__ import annotations

import argparse
import json
import math
import struct
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

PREFIX = "请先观察工程，再给一个有界的小步改进建议："
NUDGE = "可以执行"

VARIANTS: Dict[str, str] = {
    "A1": "给人声轨加一点混响，衰减短些，不要动整体增益或 EQ",
    "A2": "给钢琴加一点混响",
    "A3": "给鼓组加一点房间感（room）",
    "A4": "给这条吉他轨加一个轻微的延迟",
    "A5": "给人声加一点板式混响",
    "B1": "给人声加点空间感（M2 场次原话术）",
    "B2": "这条钢琴听着太干了，帮我弄润一点",
    "B3": "让鼓组听起来更有空间",
    "B4": "人声有点贴脸，往后放一点",
    "B5": "让这条吉他有点远近层次",
}

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
           source: str = "softguide_exp") -> Dict[str, Any]:
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


def run_variant(base_url: str, arm: str, variant: str, prompt: str, fixture_path: Path,
                out_dir: Path, args: argparse.Namespace, session_log: List[str]) -> Dict[str, Any]:
    out_dir.mkdir(parents=True, exist_ok=True)
    (out_dir / "prompt.txt").write_text(prompt, encoding="utf-8")
    timeline: List[str] = []
    logs: List[str] = []
    started = time.monotonic()
    signals: Dict[str, Any] = {
        "arm": arm,
        "variant": variant,
        "prompt": prompt,
        "conversation_id": "",
        "started_at": now_stamp(),
        "wallclock_start": datetime.now().astimezone().isoformat(),
    }

    tracks_before = open_fixture(base_url, fixture_path, logs)
    signals["disclosed_tracks"] = [
        {"id": str(t.get("track_id") or t.get("id")), "name": str(t.get("track_name") or t.get("name"))}
        for t in tracks_before
    ]
    sends_before = send_topology(base_url)
    (out_dir / "send_list_before.json").write_text(json.dumps(sends_before, ensure_ascii=False, indent=1), encoding="utf-8")
    signals["send_edges_before"] = len(sends_before.get("sends") or [])
    signals["fx_tracks_before"] = len(sends_before.get("fx_tracks") or [])

    conversation_id = f"softguide_{arm.lower()}_{variant}_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    signals["conversation_id"] = conversation_id

    responses: List[Dict[str, Any]] = []
    stop_reasons: List[str] = []
    goal_statuses: List[str] = []
    workflows: List[str] = []
    planned_commands: List[str] = []
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
            workflow = response.get("workflow")
            if workflow:
                workflows.append(str(workflow))
            for cmd in response.get("commands") or []:
                if isinstance(cmd, dict):
                    name = str(cmd.get("name") or (cmd.get("command") or {}).get("cmd") or "")
                    if name:
                        planned_commands.append(name)
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
                for cmd in hop_response.get("commands") or []:
                    if isinstance(cmd, dict):
                        name = str(cmd.get("name") or (cmd.get("command") or {}).get("cmd") or "")
                        if name:
                            planned_commands.append(name)
                hop_workflow = hop_response.get("workflow")
                if hop_workflow:
                    workflows.append(str(hop_workflow))
                card_hops.append({
                    "index": hops,
                    "card": card,
                    "stop_reason": hop_stop,
                    "goal_status": hop_goal,
                    "auto_authorized": bool((hop_response.get("workflow_data") or {}).get("full_access_auto_authorized")),
                    "mutation_performed": bool((hop_response.get("workflow_data") or {}).get("mutation_performed")),
                })
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
            # waiting_continue / limit_reached / transient_llm_error from a turn
            # response are NOT nudge triggers on their own: the chain usually
            # keeps running on its durable continuation. Fall through to the
            # runtime poll; only the poll side nudges after checking that no
            # continuation is live (the fs_largeproject / fullaccess fa shape).
        time.sleep(5.0)
        runtime = get_json(base_url, "/agent/runtime/status", 30)
        if runtime:
            runtime_final = runtime
        walked = walk_pending_card(base_url)
        if walked:
            # Fallback auto-answer for a card parked on the durable continuation.
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
                (out_dir / f"walkhop_{hops}.json").write_text(json.dumps(hop_response, ensure_ascii=False, indent=1), encoding="utf-8")
                hop_goal = str(hop_response.get("goal_status") or "")
                if hop_goal:
                    goal_statuses.append(hop_goal)
                    goal_status = hop_goal
                for cmd in hop_response.get("executed_kernel_reply") or []:
                    if isinstance(cmd, dict):
                        name = str(cmd.get("command_name") or "")
                        if name:
                            executed_commands.append(name)
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
        "workflows": workflows,
        "planned_commands": planned_commands,
        "executed_commands": executed_commands,
        "card_hops": card_hops,
        "transport_errors": transport_errors,
        "timeline": timeline,
        "elapsed_s": round(time.monotonic() - started, 1),
        "wallclock_end": datetime.now().astimezone().isoformat(),
    })

    events = get_json(base_url, f"/agent/events?conversation_id={urllib.request.quote(conversation_id)}&since=0&limit=1000", 60)
    if events:
        (out_dir / "events.json").write_text(json.dumps(events, ensure_ascii=False, indent=1), encoding="utf-8")
        signals["events_count"] = len(events.get("events") or [])

    time.sleep(2.0)
    sends_after = send_topology(base_url)
    (out_dir / "send_list_after.json").write_text(json.dumps(sends_after, ensure_ascii=False, indent=1), encoding="utf-8")
    signals["send_edges_after"] = len(sends_after.get("sends") or [])
    signals["fx_tracks_after"] = len(sends_after.get("fx_tracks") or [])
    signals["fx_track_rows_after"] = [
        {"id": str(t.get("track_id")), "name": str(t.get("name"))} for t in (sends_after.get("fx_tracks") or [])
    ]
    signals["send_rows_after"] = sends_after.get("sends") or []

    state_after = invoke(base_url, "project.state", {}, 60)
    (out_dir / "project_state_after.json").write_text(json.dumps(state_after, ensure_ascii=False, indent=1), encoding="utf-8")
    tracks_after = (state_after.get("result") or {}).get("tracks") or []
    rack_rows = []
    for t in tracks_after:
        rack = t.get("rack") or {}
        nodes = rack.get("nodes") or []
        if nodes:
            rack_rows.append({
                "track": str(t.get("track_name") or t.get("name")),
                "track_id": str(t.get("track_id") or t.get("id")),
                "nodes": [
                    {"plugin": str(n.get("plugin_name") or n.get("name") or ""), "plugin_id": n.get("plugin_id")}
                    for n in nodes if isinstance(n, dict)
                ],
            })
    signals["rack_state_after"] = rack_rows
    signals["tracks_after"] = [
        {"id": str(t.get("track_id") or t.get("id")), "name": str(t.get("track_name") or t.get("name"))}
        for t in tracks_after
    ]
    if runtime_final:
        (out_dir / "runtime_status_final.json").write_text(json.dumps(runtime_final, ensure_ascii=False, indent=1), encoding="utf-8")

    for line in timeline:
        session_log.append(f"[{variant}] {line}")
    for line in logs:
        session_log.append(f"[{variant}] {line}")
    (out_dir / "variant_summary.json").write_text(json.dumps(signals, ensure_ascii=False, indent=1), encoding="utf-8")
    return signals


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--arm", required=True, choices=["B2", "B1"])
    parser.add_argument("--variants", required=True, help="comma-separated variant ids, e.g. A1,A2")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--fixture-dir", required=True)
    parser.add_argument("--settle-seconds", type=int, default=720)
    parser.add_argument("--turn-seconds", type=int, default=480)
    parser.add_argument("--max-nudges", type=int, default=2)
    parser.add_argument("--max-hops", type=int, default=6)
    args = parser.parse_args()

    out_dir = Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    fixture_dir = Path(args.fixture_dir).resolve()
    fixture_dir.mkdir(parents=True, exist_ok=True)
    session_log: List[str] = []
    session_summary: Dict[str, Any] = {
        "driver": "softguide_exp.py",
        "experiment": "SEMANTIC-TIMEFX-SOFTGUIDE-EXP-1",
        "arm": args.arm,
        "variants": args.variants.split(","),
        "started_at": now_stamp(),
        "prefix": PREFIX,
        "nudge": NUDGE,
    }

    fixture_path = fixture_dir / "softguide_fixture.vit"
    try:
        if not fixture_path.exists():
            fixture_path = build_fixture(args.agent_http, fixture_dir, session_log)
            session_log.append(f"fixture built: {fixture_path}")
        else:
            session_log.append(f"fixture reused: {fixture_path}")

        switch = post_json(args.agent_http, "/agent/authority", {"authority_mode": "full_project_access"}, 30)
        (out_dir / "authority_switch.json").write_text(json.dumps(switch, ensure_ascii=False, indent=1), encoding="utf-8")
        if str(switch.get("authority_mode")) != "full_project_access":
            raise EnvAbort("authority switch refused: " + json.dumps(switch, ensure_ascii=False)[:400])
        probe = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": f"softguide_{args.arm.lower()}_authority_{datetime.now().strftime('%Y%m%d_%H%M%S')}",
            "message": "/smoke authority",
            "context": {"agent_mode": "chat"},
        }, max(30, 30))
        (out_dir / "authority_probe.json").write_text(json.dumps(probe, ensure_ascii=False, indent=1), encoding="utf-8")
        if str(probe.get("stop_reason")) != "authority_smoke_ok" or "bound=full_project_access" not in str(probe.get("reply") or ""):
            raise EnvAbort("authority input-chain probe did not bind full access: " + json.dumps(probe, ensure_ascii=False)[:400])
        session_log.append("authority bound full_project_access")

        variant_results: List[Dict[str, Any]] = []
        for variant in [v.strip() for v in args.variants.split(",") if v.strip()]:
            prompt = PREFIX + VARIANTS[variant]
            result = run_variant(args.agent_http, args.arm, variant, prompt, fixture_path,
                                 out_dir / "arm" / variant, args, session_log)
            variant_results.append(result)
            print(f"[variant {variant}] settle={result['settle_reason']} goal={result['goal_status_final']} "
                  f"stop={result['final_stop_reason']} sends={result['send_edges_before']}->{result['send_edges_after']} "
                  f"fx={result['fx_tracks_before']}->{result['fx_tracks_after']} hops={len(result['card_hops'])}", flush=True)

        session_summary["variants_done"] = [r["variant"] for r in variant_results]
        session_summary["results"] = variant_results
        session_summary["status"] = "captured"
        session_summary["finished_at"] = now_stamp()
        (out_dir / "session_summary.json").write_text(json.dumps(session_summary, ensure_ascii=False, indent=1), encoding="utf-8")
        (out_dir / "session_log.txt").write_text("\n".join(session_log) + "\n", encoding="utf-8")
        return 0
    except EnvAbort as exc:
        session_summary["status"] = "env_abort"
        session_summary["error"] = str(exc)
        session_summary["finished_at"] = now_stamp()
        (out_dir / "session_summary.json").write_text(json.dumps(session_summary, ensure_ascii=False, indent=1), encoding="utf-8")
        (out_dir / "session_log.txt").write_text("\n".join(session_log) + "\n", encoding="utf-8")
        print(f"ENV_ABORT: {exc}", file=sys.stderr, flush=True)
        return 2


if __name__ == "__main__":
    sys.exit(main())
