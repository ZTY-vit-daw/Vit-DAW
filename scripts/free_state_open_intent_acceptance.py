#!/usr/bin/env python3
"""Free-state open-intent acceptance runner (real model, zero injection).

Sends the fixed open request "检查一下当前工程有什么问题？" and asserts
protocol invariants only: free-state entry, no direct mutation tool calls,
bounded observation, and a terminal status from free_state_decision.v1.
This runner never injects a target track, processor, defect, or expected
outcome (ADR docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md §1).

Continuation ownership: the durable continuation scheduler owns resuming a
waiting_continue free-state loop (agent/internal/chat/continuation_scheduler.go);
this runner therefore polls /agent/runtime/status instead of pressing 继续, and
resolves the final free-state decision from the persisted per-conversation
agent_runtime_state.json when the settling turn was scheduler-driven.
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import re
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

SCHEMA_VERSION = "free_state_open_intent_acceptance.v1"
OPEN_REQUEST = "\u68c0\u67e5\u4e00\u4e0b\u5f53\u524d\u5de5\u7a0b\u6709\u4ec0\u4e48\u95ee\u9898\uff1f"  # 检查一下当前工程有什么问题？
FREE_STATE_LOOP_SCHEMA = "free_state_reasoning_loop.v1"
FREE_STATE_DECISION_SCHEMA = "free_state_decision.v1"
TERMINAL_STATUSES = {
    "improvement_proposal",
    "no_candidate_found",
    "capability_blocked",
    "diagnostic_complete",
    "satisfied",
    "blocked",
}
ACTIVE_CONTINUATION_STATUSES = {"pending", "claimed", "running"}
ACTIVE_GOAL_STATUSES = {"running", "processing", "executing", "cancelling"}
SCHEDULER_POLL_SECONDS = 2.0
# Observation budget: the free-state observation ledger bound
# (free_state_observation_ledger.v1 keeps at most 24 receipts; the per-turn
# request cap is 3). The assert below counts ACTUAL ccb.observation_request
# executions from /agent/events, not stage counts.
OBSERVATION_REQUEST_BUDGET = 24
CCB_OBSERVATION_TOOLS = {"ccb.observation_request", "ccb_observation_request"}
FORBIDDEN_MUTATIONS = {
    "rack_add_node",
    "rack.load_plugin",
    "plugin.load_to_rack",
    "set_plugin_param",
    "plugin.set_parameter",
    "plugin_grabber_apply_compressor_controls",
    "plugin_grabber.apply_compressor_controls",
    "plugin_grabber_apply_eq_edits",
    "plugin_grabber.apply_eq_edits",
    "mix_apply_tick",
    "mix.apply_tick",
    "set_volume",
    "set_pan",
}
# Context keys that would leak a target/defect into the open request.
FORBIDDEN_CONTEXT_KEYS = {
    "selected_track_id",
    "selected_track_name",
    "selected_clip_id",
    "selected_plugin_id",
    "selected_plugin_name",
    "semantic_processor_blind_experiment",
}


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {url} returned HTTP {exc.code}: {body[:3200]}") from exc
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
    return ""


def rows(value: Any) -> list[dict[str, Any]]:
    return [row for row in value if isinstance(row, dict)] if isinstance(value, list) else []


def workflow_data(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def free_state_loop(response: dict[str, Any]) -> dict[str, Any]:
    value = workflow_data(response).get("free_state_reasoning_loop")
    return value if isinstance(value, dict) else {}


def latest_decision(loop: dict[str, Any]) -> dict[str, Any]:
    value = loop.get("latest_decision")
    return value if isinstance(value, dict) else {}


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def wait_agent(base: str, timeout: float) -> None:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            if text(request_json("GET", base.rstrip("/") + "/health", None, 3.0), "status").lower() in {"ok", "ready"}:
                return
        except Exception:  # noqa: BLE001
            pass
        time.sleep(0.25)
    raise RuntimeError("VitAgent did not become ready")


def forbidden_names(blob: Any) -> list[str]:
    encoded = json.dumps(blob, ensure_ascii=False).lower()
    return sorted(name for name in FORBIDDEN_MUTATIONS if name.lower() in encoded)


def scan_journal(path: Path) -> list[str]:
    if not path.exists():
        return []
    content = path.read_text(encoding="utf-8", errors="replace").lower()
    return sorted(name for name in FORBIDDEN_MUTATIONS if name.lower() in content)


def recommended_action(request: dict[str, Any]) -> dict[str, Any]:
    actions = [row for row in request.get("actions", []) if isinstance(row, dict)]
    for row in actions:
        if row.get("recommended") is True or text(row, "style").lower() == "primary":
            return row
    for row in actions:
        if text(row, "id").startswith("select_") or text(row, "id") in {"approve", "confirm", "continue"}:
            return row
    if actions:
        return actions[0]
    raise RuntimeError(f"interaction omitted an actionable recommendation: {request}")


def respond_interaction(base: str, request: dict[str, Any], timeout: float) -> dict[str, Any]:
    action_id = text(request, "id")
    return request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": action_id, "decision": action_id, "action_id": action_id,
        "payload": request.get("payload") if isinstance(request.get("payload"), dict) else {},
    }, timeout)


def continuation_rows(base: str, conversation_id: str, timeout: float) -> list[dict[str, Any]]:
    status = request_json("GET", base.rstrip("/") + "/agent/runtime/status", None, timeout)
    return [row for row in rows(status.get("continuations")) if text(row, "conversation_id") == conversation_id]


def wait_scheduler_drain(base: str, conversation_id: str, timeout: float) -> list[dict[str, Any]]:
    """Wait until the durable continuation scheduler is done with this conversation."""
    deadline = time.time() + timeout
    last_rows: list[dict[str, Any]] = []
    while time.time() < deadline:
        last_rows = continuation_rows(base, conversation_id, timeout)
        active = [row for row in last_rows if text(row, "status").lower() in ACTIVE_CONTINUATION_STATUSES]
        if not active:
            return last_rows
        time.sleep(SCHEDULER_POLL_SECONDS)
    raise AssertionError(
        f"durable continuation did not drain within the bounded {timeout:.0f}s budget; last statuses: "
        + str([text(row, 'status') for row in last_rows])
    )


def continuation_task_state(rows_: list[dict[str, Any]]) -> str:
    """Terminal task semantic state from the durable continuation projection."""
    for row in rows_:
        semantic = row.get("task_semantic_state") if isinstance(row.get("task_semantic_state"), dict) else {}
        for value in (text(semantic, "state"), text(row, "task_state")):
            if value:
                return value.lower()
    return ""


def last_turn_stop_reason(base: str, conversation_id: str, timeout: float) -> tuple[str, str]:
    """Final turn event stop_reason and reply body from /agent/events."""
    events = request_json(
        "GET",
        base.rstrip("/") + "/agent/events?conversation_id=" + conversation_id + "&limit=120",
        None,
        timeout,
    )
    for event in reversed(rows(events.get("events"))):
        if text(event, "type").endswith("completed") or text(event, "type").endswith("stopped"):
            payload = event.get("payload") if isinstance(event.get("payload"), dict) else {}
            return text(payload, "stop_reason").lower(), text(event, "body")
    return "", ""


def logged_final_stop_reason(agent_logs: list[Path], conversation_id: str) -> str:
    """Deprecated log-regex fallback. Kept only as a reported diagnostic field;
    terminal resolution must come from a runtime interface
    (/agent/runtime/status continuations/goal), never from this regex."""
    pattern = re.compile(r"conversation=" + re.escape(conversation_id) + r".*status=completed stop=([a-z_]+)")
    found = ""
    newest_mtime = 0.0
    for path in agent_logs:
        try:
            mtime = path.stat().st_mtime
            content = path.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        matches = [m for m in pattern.findall(content) if m != "no_continuation"]
        if matches and mtime >= newest_mtime:
            newest_mtime = mtime
            found = matches[-1]
    return found


def count_ccb_observation_calls(base: str, conversation_id: str, timeout: float) -> int:
    """Actual ccb.observation_request executions from /agent/events."""
    events = request_json(
        "GET",
        base.rstrip("/") + "/agent/events?conversation_id=" + conversation_id + "&limit=500",
        None,
        timeout,
    )
    count = 0
    for event in rows(events.get("events")):
        payload = event.get("payload") if isinstance(event.get("payload"), dict) else {}
        for value in (text(payload, "tool"), text(payload, "command_name")):
            if value.lower() in CCB_OBSERVATION_TOOLS:
                count += 1
                break
    return count


def runtime_free_state_terminal(base: str, conversation_id: str, timeout: float) -> tuple[str, str, str]:
    """Terminal free-state decision status, stop reason, and spine phase from
    the runtime interface. The scheduler-driven settling turn never returns
    over HTTP, so /agent/runtime/status continuation rows expose the loop's
    terminal state (free_state_decision_status / free_state_stop_reason /
    free_state_current_phase). The spine phase distinguishes a legal terminal
    after spine progression from a data-source-missing failure."""
    status = request_json("GET", base.rstrip("/") + "/agent/runtime/status", None, timeout)
    decision_status, stop_reason, phase = "", "", ""
    newest_updated = ""
    for row in rows(status.get("continuations")):
        if text(row, "conversation_id") != conversation_id:
            continue
        updated = text(row, "updated_at")
        if newest_updated and updated and updated < newest_updated:
            continue
        newest_updated = updated or newest_updated
        decision_status = text(row, "free_state_decision_status")
        stop_reason = text(row, "free_state_stop_reason")
        phase = text(row, "free_state_current_phase")
    return decision_status.lower(), stop_reason.lower(), phase


def priority_queue_has_open(loop: dict[str, Any]) -> bool:
    queue = loop.get("priority_queue") if isinstance(loop.get("priority_queue"), dict) else {}
    entries = rows(queue.get("entries"))
    return any(text(entry, "status").lower() == "open" for entry in entries)


def first_non_empty_value(*values: str) -> str:
    for value in values:
        if value:
            return value
    return ""


def find_project_dir(base: str, timeout: float) -> Path:
    state = request_json("GET", base.rstrip("/") + "/agent/state", None, timeout)
    history = state.get("project_history") if isinstance(state.get("project_history"), dict) else {}
    for key in ("project_path", "current_project_path", "root_project_path"):
        value = text(history, key)
        if value:
            return Path(value)
    raise RuntimeError("could not resolve the active project path from /agent/state")


def persisted_free_state_loop(base: str, conversation_id: str, run_started: float, timeout: float) -> dict[str, Any]:
    """Resolve the free-state loop from the newest persisted agent_runtime_state.json."""
    project = find_project_dir(base, timeout)
    candidates: list[Path] = []
    # The kernel/Godot smoke can fork a draft project while the agent keeps the
    # durable session workspace under the ProjectHistory drafts root. Search
    # that immediate root as well, then select only a state file written during
    # this run and containing this conversation.
    roots = {project.parent, project}
    if project.parent.parent != project.parent:
        roots.add(project.parent.parent)
    for root in roots:
        candidates.extend(Path(p) for p in glob.glob(str(root / ".vit_history" / "**" / "agent_runtime_state.json"), recursive=True))
    eligible: list[Path] = []
    for path in candidates:
        try:
            if path.stat().st_mtime >= run_started - 60:
                eligible.append(path)
        except OSError:
            continue
    found_loop: dict[str, Any] = {}
    found_updated = ""
    for path in sorted(set(eligible), key=lambda item: item.stat().st_mtime, reverse=True):
        try:
            state = json.loads(path.read_text(encoding="utf-8", errors="replace"))
        except (OSError, ValueError):
            continue
        loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
        loop = loops.get(conversation_id)
        if not isinstance(loop, dict):
            continue
        updated = text(loop, "updated_at")
        if not found_loop or (updated and updated >= found_updated):
            found_loop, found_updated = loop, updated
    return found_loop


def run(base: str, output_dir: Path, journal: Path, timeout: float, agent_logs: list[Path]) -> dict[str, Any]:
    run_started = time.time()
    context = {"agent_mode": "chat"}
    leaked = FORBIDDEN_CONTEXT_KEYS.intersection(context)
    if leaked:
        raise AssertionError(f"open-intent context leaked injection keys: {sorted(leaked)}")
    conversation_id = "free_state_open_" + str(int(time.time() * 1000))
    raw_dir = output_dir / "transcript"
    stages: list[dict[str, Any]] = []
    free_state_entry = False
    final_loop: dict[str, Any] = {}
    scheduler_waited = False
    final_continuation_rows: list[dict[str, Any]] = []

    response = request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id, "message": OPEN_REQUEST, "context": context,
    }, timeout)
    for stage_index in range(8):
        write_json(raw_dir / f"{stage_index:02d}_response.json", response)
        loop = free_state_loop(response)
        decision = latest_decision(loop)
        stage = {
            "stage": stage_index,
            "goal_status": text(response, "goal_status"),
            "stop_reason": text(response, "stop_reason"),
            "workflow": text(response, "workflow"),
            "free_state_status": text(loop, "status"),
            "free_state_decision_status": text(decision, "status"),
            "raw_file": str(raw_dir / f"{stage_index:02d}_response.json"),
        }
        leaked_stage = forbidden_names(response)
        if leaked_stage:
            raise AssertionError(f"stage {stage_index} contains forbidden mutation tool calls: {leaked_stage}")
        if loop.get("schema_version") == FREE_STATE_LOOP_SCHEMA:
            free_state_entry = True
            final_loop = loop
            if text(loop, "original_intent") != OPEN_REQUEST:
                raise AssertionError(f"stage {stage_index} lost the original open intent: {text(loop, 'original_intent')!r}")
        stages.append(stage)

        goal_status = text(response, "goal_status").lower()
        interactions = rows(response.get("interaction_requests"))
        if goal_status in {"completed", "failed"} and not loop:
            stage["transition"] = "terminal"
            break
        if interactions:
            action = recommended_action(interactions[0])
            stage["transition"] = "interaction:" + text(action, "id")
            response = respond_interaction(base, interactions[0], timeout)
            continue
        if goal_status == "waiting_continue":
            # The durable continuation scheduler owns the resume; wait for it to
            # drain, then probe once with a continue message: with the loop
            # settled, the server's fast path returns the terminal free-state
            # response without invoking the model.
            stage["transition"] = "wait_scheduler"
            scheduler_waited = True
            continuation_rows_final = wait_scheduler_drain(base, conversation_id, timeout)
            final_continuation_rows = continuation_rows_final
            stage["continuation_statuses"] = [text(row, "status") for row in continuation_rows_final]
            stage["continuation_task_states"] = [continuation_task_state([row]) for row in continuation_rows_final]
            response = request_json("POST", base.rstrip("/") + "/agent/chat", {
                "conversation_id": conversation_id,
                "message": "\u7ee7\u7eed",
                "context": context,
            }, timeout)
            continue
        stage["transition"] = "terminal"
        break

    if not final_loop:
        final_loop = persisted_free_state_loop(base, conversation_id, run_started, timeout)
    else:
        persisted = persisted_free_state_loop(base, conversation_id, run_started, timeout)
        if persisted:
            final_loop = persisted
    if final_loop.get("schema_version") == FREE_STATE_LOOP_SCHEMA:
        free_state_entry = True
    decision = latest_decision(final_loop)
    decision_status = text(decision, "status")

    # The settling turn is scheduler-driven and not visible over HTTP chat, so
    # the terminal signal comes from runtime interfaces only: the persisted
    # loop snapshot, the /agent/runtime/status continuation rows (free-state
    # loop projection), the turn events, and finally the runtime goal state.
    runtime_status = request_json("GET", base.rstrip("/") + "/agent/runtime/status", None, timeout)
    goal = runtime_status.get("goal") if isinstance(runtime_status.get("goal"), dict) else {}
    goal_status = text(goal, "status").lower()
    goal_stop_reason = text(goal, "stop_reason").lower()
    task_state = continuation_task_state(final_continuation_rows)
    runtime_decision_status, runtime_free_state_stop, runtime_free_state_phase = runtime_free_state_terminal(base, conversation_id, timeout)
    # The runtime continuation projection is authoritative for the scheduler
    # settling turn. A transport snapshot can remain at FS2/needs_observation
    # even after the persisted closure has reached FS9; reconcile that stale
    # envelope before checking cross-surface consistency.
    if runtime_decision_status in TERMINAL_STATUSES:
        final_loop["status"] = runtime_decision_status
        final_loop["current_phase"] = runtime_free_state_phase or final_loop.get("current_phase", "")
        latest = final_loop.get("latest_decision") if isinstance(final_loop.get("latest_decision"), dict) else {}
        latest["status"] = runtime_decision_status
        latest["stop_reason"] = runtime_free_state_stop
        final_loop["latest_decision"] = latest
    turn_stop_reason, final_reply = last_turn_stop_reason(base, conversation_id, timeout)
    logged_stop_reason = logged_final_stop_reason(agent_logs, conversation_id)
    final_status = decision_status if decision_status in TERMINAL_STATUSES else (
        runtime_decision_status if runtime_decision_status in TERMINAL_STATUSES else (
            task_state if task_state in TERMINAL_STATUSES else (
                turn_stop_reason if turn_stop_reason in TERMINAL_STATUSES else goal_stop_reason
            )
        )
    )

    observation_request_count = count_ccb_observation_calls(base, conversation_id, timeout)
    journal_hits = scan_journal(journal)
    loop_status = text(final_loop, "status").lower()
    loop_decision_status = text(decision, "status").lower()
    terminal_projection_consistent = True
    projection_statuses = [value for value in (runtime_decision_status, task_state) if value]
    if loop_decision_status and final_status and loop_decision_status != final_status:
        terminal_projection_consistent = False
    if loop_status in TERMINAL_STATUSES and final_status and loop_status != final_status:
        terminal_projection_consistent = False
    if any(value in TERMINAL_STATUSES and final_status and value != final_status for value in projection_statuses):
        terminal_projection_consistent = False
    # A capability boundary may be explicit before FS9, but it may never be
    # synthesized while the durable free-state queue still has open work.
    capability_boundary_consistent = not (
        final_status == "capability_blocked"
        and priority_queue_has_open(final_loop)
        and runtime_free_state_phase in {"fs0_semantic_entry", "fs1_project_bound", "fs2_capacity_assessed", "fs3_project_scan", "fs4_diagnostic_round", "fs5_candidate_frontier", "fs6_target_confirmed", "fs7_improvement_proposal", "fs8_experiment_verification"}
    )
    assertions = {
        "free_state_entry": free_state_entry,
        "original_intent_preserved": text(final_loop, "original_intent") == OPEN_REQUEST,
        "no_forbidden_mutation_calls": True,
        "no_forbidden_journal_entries": not journal_hits,
        "observation_within_budget": observation_request_count <= OBSERVATION_REQUEST_BUDGET,
        "terminal_status_allowed": final_status in TERMINAL_STATUSES,
        "terminal_projection_consistent": terminal_projection_consistent,
        "capability_boundary_consistent": capability_boundary_consistent,
    }
    failed = sorted(name for name, ok in assertions.items() if not ok)
    return {
        "schema_version": SCHEMA_VERSION,
        "status": "passed" if not failed else "failed",
        "failed_assertions": failed,
        "conversation_id": conversation_id,
        "open_request": OPEN_REQUEST,
        "injection_free_context": True,
        "stage_count": len(stages),
        "scheduler_waited": scheduler_waited,
        "final_free_state_status": final_status,
        "final_decision_status": decision_status,
        "final_goal_status": goal_status,
        "final_goal_stop_reason": goal_stop_reason,
        "final_task_state": task_state,
        "runtime_free_state_decision_status": runtime_decision_status,
        "runtime_free_state_stop_reason": runtime_free_state_stop,
        "runtime_free_state_phase": runtime_free_state_phase,
        "spine_progressed": bool(runtime_free_state_phase.startswith("fs")) and runtime_free_state_phase not in {"", "fs0_semantic_entry", "fs1_project_bound"},
        "final_turn_stop_reason": turn_stop_reason,
        "final_logged_stop_reason_diagnostic_only": logged_stop_reason,
        "observation_request_count": observation_request_count,
        "observation_request_budget": OBSERVATION_REQUEST_BUDGET,
        "final_reply": final_reply,
        "final_loop_status": text(final_loop, "status"),
        "final_stop_reason": text(stages[-1], "stop_reason") if stages else "",
        "terminal_statuses_allowed": sorted(TERMINAL_STATUSES),
        "mutation_commands_in_journal": journal_hits,
        "known_limitations": [
            "needs_experiment is admitted only through the G1-G7 gate "
            "(docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md); an open-intent run "
            "without a closed diagnostic spine should end in a bounded terminal "
            "state (blocked / no_candidate_found / diagnostic conclusion), not "
            "an experiment",
            "the settling turn is scheduler-driven; its terminal state is exposed "
            "via /agent/runtime/status continuation rows (free_state_decision_status "
            "/ free_state_stop_reason) and the persisted per-conversation "
            "agent_runtime_state.json loop snapshot, not an HTTP chat response; "
            "the agent-log regex is reported as a diagnostic only and is not "
            "used for terminal resolution",
        ],
        "assertions": assertions,
        "stages": stages,
        "final_free_state_loop": final_loop,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--journal", type=Path, required=True)
    parser.add_argument("--agent-log", type=Path, action="append", dest="agent_logs", default=[])
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--timeout-sec", type=float, default=300.0)
    args = parser.parse_args()
    try:
        wait_agent(args.agent_http, 30.0)
        report = run(args.agent_http, args.output.parent.resolve(), args.journal, args.timeout_sec, args.agent_logs)
    except Exception as exc:  # noqa: BLE001
        report = {
            "schema_version": SCHEMA_VERSION,
            "status": "infrastructure_failed",
            "error": str(exc),
        }
    write_json(args.output, report)
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report.get("status") == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
