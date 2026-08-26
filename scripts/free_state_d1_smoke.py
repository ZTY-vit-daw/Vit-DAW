#!/usr/bin/env python3
"""Public-only real-stack smoke for the Vit-DAW D1-S1 track-gain experiment."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import shutil
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any


PUBLIC_SCHEMA = "semantic_processor_agent_project_smoke_public_manifest.v1"
DEFAULT_CASE_ID = "spv1_p01"
OPEN_PROMPT = "检查一下当前工程有什么问题？"
NOT_EXERCISED_EXIT = 3
ACTIVE_CONTINUATION_STATUSES = {"pending", "claimed", "running"}
FORBIDDEN_SELECTION_KEYS = {
    "selected_track_id",
    "selected_track_name",
    "selected_clip_id",
    "selected_clip_ids",
    "selected_clip_track_id",
    "selected_clip_ranges",
    "selected_clip_range",
}


class SchedulerDrainTimeout(RuntimeError):
    def __init__(self, message: str, timeline: list[dict[str, Any]], runtime_status: dict[str, Any], continuations: list[dict[str, Any]]):
        super().__init__(message)
        self.timeline = timeline
        self.runtime_status = runtime_status
        self.continuations = continuations


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as http_error:
        body = ""
        try:
            body = http_error.read().decode("utf-8", errors="replace")[:500]
        except Exception:  # noqa: BLE001 - body is diagnostic only.
            pass
        raise RuntimeError(f"{method} {url} -> HTTP {http_error.code}: {body}") from http_error
    if not isinstance(value, dict):
        raise RuntimeError(f"non-object response from {url}")
    return value


def invoke(base_url: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    envelope = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "source": "free_state_d1_smoke.setup", "confirmed": confirmed},
        timeout,
    )
    if str(envelope.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(envelope, ensure_ascii=False)[:2000]}")
    result = envelope.get("result", envelope)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned no object result")
    return result


def load_public_case(manifest_path: Path, case_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    resolved = manifest_path.resolve()
    if "sealed" in {part.lower() for part in resolved.parts}:
        raise ValueError("public manifest path must not contain a sealed segment")
    manifest = json.loads(resolved.read_text(encoding="utf-8-sig"))
    if not isinstance(manifest, dict) or manifest.get("schema_version") != PUBLIC_SCHEMA:
        raise ValueError("public manifest schema mismatch")
    blindness = manifest.get("blindness_contract")
    required = (
        "sealed_truth_not_in_agent_context",
        "case_ids_are_semantically_opaque",
        "runner_reads_public_manifest_only",
        "same_neutral_prompt_for_all_projects",
        "preselected_track_forbidden",
        "preselected_plugin_forbidden",
    )
    if not isinstance(blindness, dict) or any(blindness.get(key) is not True for key in required):
        raise ValueError("public manifest blindness contract is incomplete")
    cases = [row for row in manifest.get("cases", []) if isinstance(row, dict) and row.get("public_case_id") == case_id]
    if len(cases) != 1:
        raise ValueError(f"public manifest must contain exactly one requested case {case_id}")
    case = cases[0]
    project_path = Path(str(case.get("project_path", ""))).resolve()
    if "sealed" in {part.lower() for part in project_path.parts}:
        raise ValueError("public project path must not contain a sealed segment")
    if not project_path.is_file():
        raise ValueError(f"public project is missing: {project_path}")
    return manifest, case


def materialize_public_case(case: dict[str, Any], workdir: Path) -> dict[str, Any]:
    source_project = Path(str(case["project_path"])).resolve()
    destination = workdir.resolve()
    if destination.exists():
        raise ValueError(f"D1 project workdir already exists: {destination}")
    shutil.copytree(source_project.parent, destination, ignore=shutil.ignore_patterns(".vit_history"))
    copied_project = destination / source_project.name
    if not copied_project.is_file():
        raise RuntimeError(f"copied public project is missing: {copied_project}")
    copied_case = dict(case)
    copied_case["project_path"] = str(copied_project)
    return copied_case


def first_text(*values: Any) -> str:
    for value in values:
        if value is None:
            continue
        text = str(value).strip()
        if text and text != "<nil>":
            return text
    return ""


def rows(value: Any) -> list[dict[str, Any]]:
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def walk(value: Any):
    yield value
    if isinstance(value, dict):
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def dicts(value: Any) -> list[dict[str, Any]]:
    return [item for item in walk(value) if isinstance(item, dict)]


def values_for_key(value: Any, key: str) -> list[Any]:
    return [item[key] for item in dicts(value) if key in item]


def project_path_from_state(state: dict[str, Any]) -> str:
    for item in dicts(state):
        path = first_text(item.get("project_path"), item.get("current_project_path"))
        if path:
            return str(Path(path).resolve())
    return ""


def project_revision(state: dict[str, Any]) -> str:
    for key in ("project_revision", "revision"):
        for value in values_for_key(state, key):
            text = first_text(value)
            if text.isdigit():
                return text
    return ""


def continuation_rows(base_url: str, conversation_id: str, timeout: float) -> list[dict[str, Any]]:
    status = request_json("GET", base_url.rstrip("/") + "/agent/runtime/status", None, timeout)
    return [item for item in rows(status.get("continuations")) if first_text(item.get("conversation_id")) == conversation_id]


def wait_scheduler_drain(base_url: str, conversation_id: str, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    latest: list[dict[str, Any]] = []
    latest_status: dict[str, Any] = {}
    timeline: list[dict[str, Any]] = []
    seen_continuation = False
    while time.monotonic() < deadline:
        latest_status = request_json("GET", base_url.rstrip("/") + "/agent/runtime/status", None, min(timeout, 30))
        latest = [item for item in rows(latest_status.get("continuations")) if first_text(item.get("conversation_id")) == conversation_id]
        timeline.append({
            "observed_at": dt.datetime.now(dt.timezone.utc).isoformat(),
            "continuations": [
                {key: item.get(key) for key in ("continuation_id", "status", "attempt", "updated_at", "last_error", "free_state_status", "free_state_current_phase", "free_state_continuation_budget", "free_state_continuation_used", "free_state_decision_status", "free_state_stop_reason") if key in item}
                for item in latest
            ],
        })
        if latest:
            seen_continuation = True
        active = [item for item in latest if first_text(item.get("status")).lower() in ACTIVE_CONTINUATION_STATUSES]
        if not active and seen_continuation:
            causes = [first_text(item.get("free_state_stop_reason"), item.get("last_error"), item.get("status")) for item in latest]
            return {"continuations": latest, "timeline": timeline, "runtime_status": latest_status, "terminal_causes": [cause for cause in causes if cause]}
        time.sleep(2)
    reason = "durable_continuation_missing_after_waiting_continue" if not seen_continuation else "D1 durable continuation did not drain within the smoke timeout"
    raise SchedulerDrainTimeout(reason, timeline, latest_status, latest)


def continuation_interaction_requests(continuations: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Project a durable waiting interaction back into the chat response shape.

    Scheduler-drained responses historically retained only the free-state loop,
    which made the smoke skip the second mix-tick confirmation and incorrectly
    assert mutation cardinality before the approved action could run.
    """
    requests: list[dict[str, Any]] = []
    for item in continuations:
        pending = item.get("pending_interaction")
        if not isinstance(pending, dict):
            continue
        raw_requests = pending.get("requests")
        candidates = raw_requests if isinstance(raw_requests, list) else [pending]
        for candidate in candidates:
            if not isinstance(candidate, dict):
                continue
            request = dict(candidate)
            if not first_text(request.get("id"), request.get("interaction_id")):
                interaction_id = first_text(pending.get("interaction_id"))
                if interaction_id:
                    request["id"] = interaction_id
            if not first_text(request.get("kind"), request.get("type")):
                request["kind"] = first_text(pending.get("kind"))
            if not first_text(request.get("status")):
                request["status"] = "waiting_for_user"
            request.setdefault("workflow", first_text(pending.get("workflow"), request.get("kind")))
            request.setdefault("goal_id", first_text(pending.get("goal_id")))
            request.setdefault("run_id", first_text(pending.get("run_id")))
            request.setdefault("conversation_id", first_text(pending.get("conversation_id")))
            requests.append(request)
    return requests


def test_continuation_interaction_requests() -> None:
    rows = continuation_interaction_requests([{
        "pending_interaction": {
            "interaction_id": "interaction-mix",
            "kind": "mix_tick_confirmation",
            "requests": [{"id": "interaction-mix", "kind": "mix_tick_confirmation", "status": "waiting_for_user"}],
        }
    }])
    assert len(rows) == 1 and rows[0]["id"] == "interaction-mix" and rows[0]["kind"] == "mix_tick_confirmation"


def newest_agent_runtime_state_for_conversation(project_path: str, conversation_id: str, started_at: float) -> dict[str, Any]:
    project = Path(project_path)
    roots = {project.parent, project}
    if project.parent.parent != project.parent:
        roots.add(project.parent.parent)
    candidates: list[Path] = []
    for root in roots:
        history_root = root / ".vit_history"
        if history_root.is_dir():
            candidates.extend(history_root.rglob("agent_runtime_state.json"))
    found: dict[str, Any] = {}
    found_updated = ""
    for path in sorted(set(candidates), key=lambda item: item.stat().st_mtime, reverse=True):
        try:
            if path.stat().st_mtime < started_at - 60:
                continue
            state = json.loads(path.read_text(encoding="utf-8", errors="replace"))
        except (OSError, ValueError):
            continue
        loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
        loop = loops.get(conversation_id)
        if not isinstance(loop, dict):
            continue
        updated = first_text(loop.get("updated_at"))
        if not found or (updated and updated >= found_updated):
            found, found_updated = state, updated
    return found


def persisted_free_state_loop(project_path: str, conversation_id: str, started_at: float) -> dict[str, Any]:
    state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, started_at)
    loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
    loop = loops.get(conversation_id)
    return loop if isinstance(loop, dict) else {}


def persisted_task_semantic_state(state: dict[str, Any], conversation_id: str) -> dict[str, Any]:
    goal_runtime = state.get("goal_runtime") if isinstance(state.get("goal_runtime"), dict) else {}
    goals = rows(goal_runtime.get("goals"))
    # The conversation -> goal binding is authoritative in conversation_goals;
    # the goal row's own conversation_id field is not always populated.
    conversation_goals = state.get("conversation_goals") if isinstance(state.get("conversation_goals"), dict) else {}
    bound_goal_id = first_text(conversation_goals.get(conversation_id))
    for goal in goals:
        if bound_goal_id and first_text(goal.get("goal_id")) == bound_goal_id:
            task = goal.get("task") if isinstance(goal.get("task"), dict) else {}
            semantic = task.get("semantic_state") if isinstance(task.get("semantic_state"), dict) else {}
            if semantic:
                return semantic
    for goal in goals:
        if first_text(goal.get("conversation_id")) != conversation_id:
            continue
        task = goal.get("task") if isinstance(goal.get("task"), dict) else {}
        semantic = task.get("semantic_state") if isinstance(task.get("semantic_state"), dict) else {}
        if semantic:
            return semantic
    return {}


def prepare_project(base_url: str, case: dict[str, Any], timeout: float) -> dict[str, Any]:
    project_path = str(Path(str(case["project_path"])).resolve())
    invoke(base_url, "project.open", {"file_path": project_path, "project_path": project_path}, timeout, confirmed=True)
    state = invoke(base_url, "project.state", {}, timeout)
    if project_path_from_state(state).lower() != project_path.lower():
        raise RuntimeError("live project binding does not match the public p01 project")

    deadline = time.monotonic() + timeout
    latest: dict[str, Any] = {}
    started = False
    while time.monotonic() < deadline:
        latest = invoke(base_url, "project.audio_analysis_status", {"latest": True}, timeout)
        candidates = [item for item in dicts(latest) if "dad_fact_status" in item]
        analysis = candidates[0] if candidates else latest
        status = first_text(analysis.get("dad_fact_status")).lower()
        ready = int(analysis.get("dad_fact_ready_count", 0) or 0)
        total = int(analysis.get("dad_fact_total_count", 0) or 0)
        waveforms = rows(analysis.get("track_waveform_envelopes"))
        if status == "ready" and total > 0 and ready == total and len(waveforms) >= total:
            return {"project_path": project_path, "dad_fact_status": status, "dad_fact_ready_count": ready, "dad_fact_total_count": total}
        queue_status = first_text(analysis.get("analysis_queue_status")).lower()
        if not started and (status not in {"ready", "building", "queued", "pending", "running"} or queue_status == "missing"):
            invoke(base_url, "project.audio_analysis_start", {"retry_missing": True, "rebuild_from_project": True, "interval_ms": 50}, timeout)
            started = True
        time.sleep(1)
    raise RuntimeError("public p01 DAD evidence did not become ready: " + json.dumps(latest, ensure_ascii=False)[:2000])


def recommended_interaction(response: dict[str, Any], track_gain_selected: bool) -> dict[str, Any] | None:
    interactions = rows(response.get("interaction_requests"))
    for interaction in interactions:
        payload = interaction.get("payload") if isinstance(interaction.get("payload"), dict) else {}
        domains = {first_text(value).lower() for value in values_for_key(payload, "action_domain")}
        if not track_gain_selected and "track_gain" not in domains:
            continue
        eligible = []
        for action in rows(interaction.get("actions")):
            action_id = first_text(action.get("id"), action.get("action_id")).lower()
            if action_id and action_id not in {"cancel", "reject", "decline", "abort"}:
                eligible.append(action)
        selected = next((item for item in eligible if item.get("recommended") is True), None)
        if selected is None:
            selected = next((item for item in eligible if first_text(item.get("style")).lower() in {"primary", "confirm"}), None)
        if selected is None:
            continue
        action_id = first_text(selected.get("id"), selected.get("action_id"))
        interaction_id = first_text(interaction.get("id"), interaction.get("interaction_id"))
        durable_payload = dict(payload)
        durable_payload.setdefault("workflow", first_text(interaction.get("workflow"), interaction.get("kind"), interaction.get("type")))
        durable_payload.setdefault("kind", first_text(interaction.get("kind"), interaction.get("type")))
        durable_payload.setdefault("type", first_text(interaction.get("type"), interaction.get("kind")))
        durable_payload.setdefault("conversation_id", first_text(interaction.get("conversation_id")))
        durable_payload.setdefault("goal_id", first_text(interaction.get("goal_id")))
        durable_payload.setdefault("run_id", first_text(interaction.get("run_id")))
        return {"interaction_id": interaction_id, "decision": action_id, "action_id": action_id, "payload": durable_payload}
    return None


def find_d1_loop(responses: list[dict[str, Any]]) -> dict[str, Any] | None:
    found = None
    for response in responses:
        for item in dicts(response):
            if item.get("schema_version") == "free_state_reasoning_loop.v1" and isinstance(item.get("experiment"), dict):
                admission = item["experiment"].get("admission")
                if isinstance(admission, dict) and first_text(admission.get("typed_action", {}).get("action_domain") if isinstance(admission.get("typed_action"), dict) else "").lower() == "track_gain":
                    found = item
    return found


def find_admission_boundary(responses: list[dict[str, Any]]) -> dict[str, Any] | None:
    """Return the first durable FS6/FS7 admission projection.

    Admission-only smoke must stop before interaction confirmation. A receipt
    is authoritative when present; the phase/status fallback keeps the smoke
    explainable for terminal closures produced by older response envelopes.
    """
    found: dict[str, Any] | None = None
    for item in responses:
        for row in dicts(item):
            if row.get("schema_version") != "free_state_reasoning_loop.v1":
                continue
            receipt = row.get("admission_receipt") if isinstance(row.get("admission_receipt"), dict) else {}
            phase = first_text(row.get("current_phase")).lower()
            status = first_text(row.get("status")).lower()
            if receipt or phase == "fs7_improvement_proposal" or (phase == "fs9_terminal" and status in {"blocked", "capability_blocked"}):
                found = {
                    "phase": phase,
                    "status": status,
                    "admission_receipt": receipt,
                    "latest_decision": row.get("latest_decision") if isinstance(row.get("latest_decision"), dict) else {},
                }
    return found


def validate_admission_only_boundary(boundary: dict[str, Any]) -> None:
    phase = first_text(boundary.get("phase")).lower()
    status = first_text(boundary.get("status")).lower()
    require(phase in {"fs7_improvement_proposal", "fs9_terminal"}, f"admission-only ended at unexpected phase {phase}")
    require(status != "no_candidate_found", "admission-only fabricated no_candidate_found")
    receipt = boundary.get("admission_receipt") if isinstance(boundary.get("admission_receipt"), dict) else {}
    if phase == "fs7_improvement_proposal":
        proposal = boundary.get("latest_decision", {}).get("improvement_proposal")
        require(isinstance(proposal, dict), "FS7 admission-only boundary omitted improvement_proposal")
        require(receipt.get("proposal_present") is True and receipt.get("proposal_valid") is True, "FS7 admission receipt is not valid")
        require(not receipt.get("failed_gate_ids"), "FS7 admission receipt contains failed gates")
    else:
        require(status in {"blocked", "capability_blocked"}, f"FS9 admission-only ended with status {status}")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def validate_d1(base_url: str, conversation_id: str, responses: list[dict[str, Any]], timeout: float, case_id: str) -> dict[str, Any]:
    loop = find_d1_loop(responses)
    require(loop is not None, "D1 loop projection is missing")
    assert loop is not None
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    admission = experiment.get("admission") if isinstance(experiment.get("admission"), dict) else {}
    typed = admission.get("typed_action") if isinstance(admission.get("typed_action"), dict) else {}
    require(first_text(typed.get("action_domain")).lower() == "track_gain", "D1 action_domain mismatch")
    require(first_text(typed.get("action_kind")).lower() == "track_gain_adjust", "D1 action_kind mismatch")
    require(int(admission.get("experiment_budget", 0) or 0) == 1, "D1 experiment_budget must equal one")
    for key in ("diagnostic_dose_bounds", "retained_dose_bounds"):
        bounds = admission.get(key) if isinstance(admission.get(key), dict) else {}
        require(int(bounds.get("max_action_attempts", 0) or 0) == 1, f"D1 {key}.max_action_attempts must equal one")

    experiment_rounds = rows(experiment.get("rounds"))
    require(len(experiment_rounds) == 1, "D1 must contain exactly one experiment round")
    round_row = experiment_rounds[0]
    interventions = rows(round_row.get("interventions"))
    require(len(interventions) == 1, "D1 must contain exactly one forward mutation")
    intervention = interventions[0]
    receipt = intervention.get("receipt") if isinstance(intervention.get("receipt"), dict) else {}
    before_revision = first_text(receipt.get("before_revision"))
    after_revision = first_text(receipt.get("after_revision"), receipt.get("applied_revision"))
    require(before_revision and after_revision and before_revision != after_revision, "D1 receipt requires distinct before/after revisions")
    for key in ("transaction_id", "idempotency_key", "actual_readback_db"):
        require(receipt.get(key) not in (None, ""), f"D1 execution receipt missing {key}")
    require(receipt.get("readback_verified") is True, "D1 actual readback was not verified")

    post_observations = [item for item in rows(round_row.get("observations")) if item.get("post_action") is True]
    require(len(post_observations) == 1, "D1 requires one post-action CCB observation")
    post = post_observations[0]
    require(post.get("fresh") is True and first_text(post.get("project_revision")) == after_revision, "post-action CCB is not fresh and revision-bound")
    require(round_row.get("materiality") is not None, "acoustic materiality record is missing")
    require(round_row.get("target_response") is not None, "target response record is missing")

    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    required_flags = ("parameter_applied", "readback_verified", "evaluation_ready", "human_audition_ready", "human_confirmed", "ambiguous", "rolled_back", "settled")
    require(all(flag in d1_receipt and isinstance(d1_receipt[flag], bool) for flag in required_flags), "D1 receipt does not express all required states")
    require(d1_receipt.get("parameter_applied") is True and d1_receipt.get("readback_verified") is True and d1_receipt.get("evaluation_ready") is True, "D1 technical/evaluation gates are incomplete")
    require(d1_receipt.get("human_audition_ready") is True, "D1 did not reach human_audition_ready")
    require(d1_receipt.get("human_confirmed") is False and d1_receipt.get("settled") is False, "smoke must not fabricate human confirmation or settlement")
    require(int(d1_receipt.get("forward_mutation_count", 0) or 0) == 1, "D1 receipt forward mutation cardinality mismatch")
    require("validation_error" not in d1_receipt, "D1 receipt schema validation failed: " + first_text(d1_receipt.get("validation_error")))

    session = loop.get("audition_session_snapshot") if isinstance(loop.get("audition_session_snapshot"), dict) else {}
    session_id = first_text(loop.get("audition_session_id"), session.get("session_id"))
    candidates = rows(session.get("candidates"))
    require(session_id and first_text(session.get("status")).lower() in {"ready", "playing", "stopped"}, "D1 audition session is not ready")
    require(len(candidates) == 2, "D1 audition requires exactly two candidates")
    by_id = {first_text(item.get("id")): item for item in candidates}
    require(set(by_id) == {"candidate-a", "candidate-b"}, "D1 audition candidate identity mismatch")
    for candidate in by_id.values():
        require(first_text(candidate.get("source_kind")) == "audio_file", "D1 audition candidates must use real audio_file sources")
        require(Path(first_text(candidate.get("source_ref"))).is_file(), "D1 audition WAV is missing")
        require(first_text(candidate.get("render_revision")) and first_text(candidate.get("preview_revision")), "D1 audition candidate provenance is incomplete")
    for key in ("source_ref", "project_revision", "render_revision", "preview_revision"):
        require(first_text(by_id["candidate-a"].get(key)) != first_text(by_id["candidate-b"].get(key)), f"D1 A/B candidates share {key}")

    actions_before = request_json("GET", base_url.rstrip("/") + "/agent/actions?limit=200", None, timeout)
    d1_actions_before = [item for item in rows(actions_before.get("actions")) if first_text(item.get("source")) == "free_state_d1_s1"]
    require(len(d1_actions_before) == 1, "D1 journal must contain exactly one forward mutation")
    state_before = invoke(base_url, "project.state", {}, timeout)
    revision_before_select = project_revision(state_before)
    for candidate_id in ("candidate-a", "candidate-b"):
        selected = request_json("POST", base_url.rstrip("/") + "/agent/audition/select", {"conversation_id": conversation_id, "session_id": session_id, "candidate_id": candidate_id}, timeout)
        require(first_text(selected.get("status")).lower() == "ok", f"audition.select failed for {candidate_id}")
    request_json("POST", base_url.rstrip("/") + "/agent/audition/stop", {"conversation_id": conversation_id, "session_id": session_id}, timeout)
    state_after = invoke(base_url, "project.state", {}, timeout)
    actions_after = request_json("GET", base_url.rstrip("/") + "/agent/actions?limit=200", None, timeout)
    d1_actions_after = [item for item in rows(actions_after.get("actions")) if first_text(item.get("source")) == "free_state_d1_s1"]
    require(project_revision(state_after) == revision_before_select, "audition.select changed the project revision")
    require(len(d1_actions_after) == len(d1_actions_before) == 1, "audition.select changed journal mutation cardinality")

    return {
        "status": "pass",
        "public_case_id": case_id,
        "conversation_id": conversation_id,
        "turn_id": first_text(experiment.get("turn_id")),
        "round_id": first_text(round_row.get("round_id")),
        "before_revision": before_revision,
        "after_revision": after_revision,
        "transaction_id": receipt["transaction_id"],
        "idempotency_key": receipt["idempotency_key"],
        "actual_readback_db": receipt["actual_readback_db"],
        "post_action_observation_id": post.get("observation_id"),
        "forward_mutation_count": 1,
        "audition_session_id": session_id,
        "human_audition_ready": True,
        "human_confirmed": False,
    }


SETTLEMENT_PROBE_TAG = "smoke_settlement_probe"
SETTLEMENT_PROBE_FREE_TEXT = "machine-originated settlement probe; not a human judgment"
SETTLEMENT_PROBE_ANSWERS = {
    "retain": {"heard_difference": "yes", "preference": "b"},
    "rollback": {"heard_difference": "yes", "preference": "a"},
    "ambiguous": {"heard_difference": "unsure", "preference": "unsure"},
}
SETTLEMENT_EXPECTATIONS = {
    "retain": {"outcome": "improved", "human_confirmed": True, "ambiguous": False, "rolled_back": False, "disposition": "retain"},
    "rollback": {"outcome": "rolled_back", "human_confirmed": True, "ambiguous": False, "rolled_back": True, "disposition": "rollback"},
    "ambiguous": {"outcome": "needs_user_judgment", "human_confirmed": False, "ambiguous": True, "rolled_back": False, "disposition": "request_audition"},
}
TERMINAL_CONTINUATION_STATUSES = {"completed", "cancelled", "failed"}


def probe_tags(evidence: dict[str, Any]) -> set[str]:
    return {first_text(tag) for tag in (evidence.get("reason_tags") or [])}


def settled_projection(state: dict[str, Any], conversation_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
    loop = loops.get(conversation_id) if isinstance(loops.get(conversation_id), dict) else {}
    return loop, persisted_task_semantic_state(state, conversation_id)


def assert_settled_projection(loop: dict[str, Any], semantic: dict[str, Any], disposition: str, evidence_id: str, expected_outcome: str) -> None:
    expected = SETTLEMENT_EXPECTATIONS[disposition]
    require(bool(loop), "settled free-state loop was not persisted")
    require(first_text(loop.get("status")).lower() == "completed", "settled loop status mismatch: " + first_text(loop.get("status")))
    # NOTE: loop.last_error may legitimately carry the boundary-park message
    # ("experiment round is waiting for the human judgment boundary") -- that
    # is the designed stop point, not a settlement defect.
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    require(first_text(experiment.get("status")).lower() == "settled", "experiment status mismatch: " + first_text(experiment.get("status")))
    require(first_text(experiment.get("outcome")).lower() == expected_outcome, f"experiment outcome mismatch for {disposition}: " + first_text(experiment.get("outcome")))
    rounds = rows(experiment.get("rounds"))
    require(len(rounds) == 1, "settlement must not open a second experiment round")
    judgments = rows(rounds[0].get("user_judgment_evidence"))
    require(judgments and first_text(judgments[-1].get("id")) == evidence_id, "persisted judgment evidence identity mismatch")
    require(SETTLEMENT_PROBE_TAG in probe_tags(judgments[-1]), "persisted judgment lost the machine-origin probe marker")
    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    require(d1_receipt.get("settled") is True, "d1 receipt did not record settlement")
    for flag in ("human_confirmed", "ambiguous", "rolled_back"):
        require(d1_receipt.get(flag) is expected[flag], f"d1 receipt {flag} mismatch for {disposition}: {d1_receipt.get(flag)!r}")
    require(first_text(d1_receipt.get("disposition")).lower() == expected["disposition"], "d1 receipt disposition mismatch: " + first_text(d1_receipt.get("disposition")))
    require(int(d1_receipt.get("forward_mutation_count", 0) or 0) == 1, "settlement must not add forward mutations")
    require(int(d1_receipt.get("rollback_compensation_count", -1) or 0) == (1 if disposition == "rollback" else 0), "rollback compensation cardinality mismatch for " + disposition)
    layers = d1_receipt.get("layers") if isinstance(d1_receipt.get("layers"), dict) else {}
    human_ab = layers.get("human_ab") if isinstance(layers.get("human_ab"), dict) else {}
    require(first_text(human_ab.get("status")).lower() == "decided", "human A/B layer was not decided")
    require(first_text(semantic.get("state")).lower() == "settled", "canonical task semantic state is not settled: " + first_text(semantic.get("state")))
    require(semantic.get("terminal") is True, "canonical task semantic state is not terminal")


def run_settlement_probe(base_url: str, conversation_id: str, project_path: str, validation: dict[str, Any], disposition: str, timeout: float, run_started: float) -> dict[str, Any]:
    answers = SETTLEMENT_PROBE_ANSWERS[disposition]
    payload = {
        "conversation_id": conversation_id,
        "turn_id": first_text(validation.get("turn_id")),
        "round_id": first_text(validation.get("round_id")),
        "audition_session_id": first_text(validation.get("audition_session_id")),
        "project_revision": first_text(validation.get("after_revision")),
        "heard_difference": answers["heard_difference"],
        "preference": answers["preference"],
        "reason_tags": [SETTLEMENT_PROBE_TAG],
        "free_text": SETTLEMENT_PROBE_FREE_TEXT,
    }
    for key in ("turn_id", "round_id", "audition_session_id", "project_revision"):
        require(payload[key], f"settlement probe is missing {key} from the D1 validation receipt")
    # The agent establishes the durable judgment boundary asynchronously after
    # audition.ready. The WebUI waits for trajectory.user_judgment.requested;
    # the probe accepts the equivalent durable state: the round binding, or
    # (task semantic human_judgment_required + round decision user_judgment_pending)
    # -- the handler re-establishes a dropped round binding through the guarded
    # judgment-request API.
    boundary_deadline = time.monotonic() + 60
    boundary_bound = False
    while not boundary_bound and time.monotonic() < boundary_deadline:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, run_started)
        loop_now, semantic_now = settled_projection(state, conversation_id)
        rounds_now = rows((loop_now.get("experiment") or {}).get("rounds")) if isinstance(loop_now.get("experiment"), dict) else []
        if rounds_now:
            round_now = rounds_now[0]
            bound_to_session = round_now.get("user_judgment_requested") is True and first_text(round_now.get("audition_session_id")) == payload["audition_session_id"]
            parked_at_boundary = first_text(semantic_now.get("state")).lower() == "human_judgment_required" and first_text(round_now.get("decision")).lower() == "user_judgment_pending"
            boundary_bound = bound_to_session or parked_at_boundary
        if not boundary_bound:
            time.sleep(1)
    require(boundary_bound, "durable judgment boundary was not established before the probe judgment")
    judged = request_json("POST", base_url.rstrip("/") + "/agent/audition/judgment", payload, timeout)
    require(first_text(judged.get("status")).lower() == "ok", "settlement probe judgment was rejected: " + first_text(judged.get("error")))
    evidence = judged.get("evidence") if isinstance(judged.get("evidence"), dict) else {}
    evidence_id = first_text(evidence.get("id"))
    require(evidence_id, "settlement probe judgment returned no evidence identity")
    require(SETTLEMENT_PROBE_TAG in probe_tags(evidence), "settlement probe marker was not retained on the judgment response")

    # The judgment settles and persists synchronously before the HTTP response
    # returns; the retry window only tolerates a slow disk flush.
    deadline = time.monotonic() + 15
    loop: dict[str, Any] = {}
    semantic: dict[str, Any] = {}
    while True:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, run_started)
        loop, semantic = settled_projection(state, conversation_id)
        if (first_text(loop.get("status")).lower() == "completed" and semantic) or time.monotonic() >= deadline:
            break
        time.sleep(1)
    assert_settled_projection(loop, semantic, disposition, evidence_id, SETTLEMENT_EXPECTATIONS[disposition]["outcome"])

    state_now = invoke(base_url, "project.state", {}, timeout)
    revision_now = project_revision(state_now)
    if disposition == "rollback":
        require(revision_now != payload["project_revision"], "rollback did not move the project revision off the treatment")
    else:
        require(revision_now == payload["project_revision"], disposition + " settlement changed the project revision")
    return {
        "disposition": disposition,
        "probe_origin": True,
        "evidence_id": evidence_id,
        "task_semantic_state": first_text(semantic.get("state")),
        "experiment_status": first_text((loop.get("experiment") or {}).get("status")) if isinstance(loop.get("experiment"), dict) else "",
        "experiment_outcome": SETTLEMENT_EXPECTATIONS[disposition]["outcome"],
        "project_revision": revision_now,
    }


def verify_settled_after_restart(base_url: str, report_path: Path, timeout: float) -> dict[str, Any]:
    prior = json.loads(report_path.read_text(encoding="utf-8"))
    probe = prior.get("settlement_probe") if isinstance(prior.get("settlement_probe"), dict) else {}
    require(probe.get("probe_origin") is True, "report has no settlement probe section to verify")
    disposition = first_text(probe.get("disposition"))
    expected_outcome = SETTLEMENT_EXPECTATIONS.get(disposition, {}).get("outcome", "")
    evidence_id = first_text(probe.get("evidence_id"))
    conversation_id = first_text(prior.get("conversation_id"))
    setup = prior.get("project_setup") if isinstance(prior.get("project_setup"), dict) else {}
    project_path = first_text(setup.get("project_path"))
    started = float(prior.get("started_at_epoch") or 0)
    require(disposition and expected_outcome and evidence_id and conversation_id and project_path and started > 0, "prior report is missing settlement identity")

    # Reactivating the workspace drives the restart reconciliation path
    # (state restore + semantic projection reconcile) before asserting.
    invoke(base_url, "project.open", {"file_path": project_path, "project_path": project_path}, timeout, confirmed=True)
    deadline = time.monotonic() + 15
    loop: dict[str, Any] = {}
    semantic: dict[str, Any] = {}
    while True:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, started)
        loop, semantic = settled_projection(state, conversation_id)
        if (loop and semantic) or time.monotonic() >= deadline:
            break
        time.sleep(1)
    assert_settled_projection(loop, semantic, disposition, evidence_id, expected_outcome)

    continuations = continuation_rows(base_url, conversation_id, min(timeout, 30))
    resurrected = [item for item in continuations if first_text(item.get("status")).lower() not in TERMINAL_CONTINUATION_STATUSES]
    require(not resurrected, "restart resurrected non-terminal continuations: " + json.dumps([{ "status": item.get("status"), "error": item.get("last_error")} for item in resurrected], ensure_ascii=False))
    return {
        "verified_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "disposition": disposition,
        "task_semantic_state": first_text(semantic.get("state")),
        "experiment_outcome": expected_outcome,
        "continuations_total": len(continuations),
    }


def write_report(path: Path, report: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--public-manifest", required=True)
    parser.add_argument("--public-case-id", default=DEFAULT_CASE_ID)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=600)
    parser.add_argument("--project-workdir", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--admission-only", action="store_true", help="stop at FS7/admission boundary before confirmation or mutation")
    parser.add_argument("--settlement-probe", choices=sorted(SETTLEMENT_PROBE_ANSWERS), default="", help="after the boundary validation, submit a machine-origin judgment and machine-check the settlement receipt; the default mode without this flag keeps asserting settled is False")
    parser.add_argument("--verify-settled", default="", help="verify a previously settled probe report after an agent restart (path to d1_smoke_report.json)")
    args = parser.parse_args()
    output = Path(args.output).resolve()
    if args.verify_settled:
        verification = verify_settled_after_restart(args.agent_http, Path(args.verify_settled).resolve(), args.timeout_sec)
        prior_path = Path(args.verify_settled).resolve()
        prior = json.loads(prior_path.read_text(encoding="utf-8"))
        prior["restart_verification"] = verification
        write_report(prior_path, prior)
        print(f"D1-S1 SETTLEMENT RESTART VERIFY PASS: disposition={verification['disposition']} report={prior_path}")
        return 0
    report: dict[str, Any] = {"schema_version": "vit.free_state_d1_smoke.v1", "started_at": dt.datetime.now(dt.timezone.utc).isoformat(), "public_case_id": args.public_case_id}
    responses: list[dict[str, Any]] = []
    try:
        _, public_case = load_public_case(Path(args.public_manifest), args.public_case_id)
        case = materialize_public_case(public_case, Path(args.project_workdir))
        report["public_source_project"] = str(Path(str(public_case["project_path"])).resolve())
        report["project_setup"] = prepare_project(args.agent_http, case, args.timeout_sec)
        report["started_at_epoch"] = time.time()
        ui_context_response = request_json("GET", args.agent_http.rstrip("/") + "/agent/ui/context", None, min(args.timeout_sec, 30))
        ui_context = ui_context_response.get("context") if isinstance(ui_context_response.get("context"), dict) else {}
        leaked_selection = FORBIDDEN_SELECTION_KEYS.intersection(ui_context)
        require(not leaked_selection, "D1 open-intent preflight inherited selected DAW targets: " + str(sorted(leaked_selection)))
        require("dev_smoke_" not in json.dumps(ui_context, ensure_ascii=False).lower(), "D1 open-intent preflight inherited dev smoke context")
        report["ui_context_preflight"] = {"status": "clean", "keys": sorted(ui_context)}
        conversation_id = "d1_s1_" + dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%S%f")
        # Persist the identity before the model request so a transport or
        # scheduler timeout still leaves a traceable open-intent attempt.
        report["conversation_id"] = conversation_id
        write_report(output, report)
        run_started = float(report["started_at_epoch"])
        response = request_json("POST", args.agent_http.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": OPEN_PROMPT, "context": {"agent_mode": "chat"}}, args.timeout_sec)
        responses.append(response)
        report["responses"] = responses
        write_report(output, report)
        track_gain_selected = False
        for _ in range(8):
            require("dev_smoke_" not in json.dumps(response, ensure_ascii=False).lower(), "D1 response inherited dev smoke target context")
            domains = {first_text(value).lower() for value in values_for_key(response, "action_domain")}
            kinds = {first_text(value).lower() for value in values_for_key(response, "action_kind")}
            if "track_gain" in domains:
                track_gain_selected = True
                require("track_gain_adjust" in kinds, "model selected track_gain without track_gain_adjust")
            if args.admission_only:
                boundary = find_admission_boundary(responses)
                if boundary is not None:
                    validate_admission_only_boundary(boundary)
                    report.update({
                        "status": "admission_only",
                        "admission": boundary,
                        "mutation_performed": False,
                        "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                        "responses": responses,
                    })
                    write_report(output, report)
                    print(f"D1-S1 ADMISSION_ONLY: report={output}")
                    return 0
            if first_text(response.get("workflow")).lower() == "free_state_d1_s1" or find_d1_loop(responses) is not None and any(bool(item.get("human_audition_ready")) for item in dicts(response)):
                break
            interaction = recommended_interaction(response, track_gain_selected)
            if interaction is not None:
                response = request_json("POST", args.agent_http.rstrip("/") + "/agent/interaction/respond", interaction, args.timeout_sec)
                responses.append(response)
                report["responses"] = responses
                write_report(output, report)
                continue
            if first_text(response.get("goal_status")).lower() == "waiting_continue":
                drain = wait_scheduler_drain(args.agent_http, conversation_id, args.timeout_sec)
                continuation_state = drain["continuations"]
                report["continuation_timeline"] = drain["timeline"]
                report["last_runtime_status"] = drain["runtime_status"]
                report["continuations"] = continuation_state
                report["terminal_causes"] = drain["terminal_causes"]
                write_report(output, report)
                failed_continuations = [item for item in continuation_state if first_text(item.get("status")).lower() == "failed" or first_text(item.get("last_error"))]
                if failed_continuations:
                    raise RuntimeError("D1 durable continuation failed: " + first_text(failed_continuations[0].get("last_error")))
                loop = persisted_free_state_loop(report["project_setup"]["project_path"], conversation_id, run_started)
                require(bool(loop), "D1 scheduler drained without a persisted free-state loop")
                response = {
                    "goal_status": "scheduler_drained",
                    "workflow_data": {"free_state_reasoning_loop": loop},
                    "continuation_statuses": [first_text(item.get("status")) for item in continuation_state],
                }
                interaction_requests = continuation_interaction_requests(continuation_state)
                if interaction_requests:
                    response["interaction_requests"] = interaction_requests
                responses.append(response)
                continue
            break

        report["responses"] = responses
        if args.admission_only:
            boundary = find_admission_boundary(responses)
            if boundary is not None:
                validate_admission_only_boundary(boundary)
                report.update({
                    "status": "admission_only",
                    "admission": boundary,
                    "mutation_performed": False,
                    "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                })
                write_report(output, report)
                print(f"D1-S1 ADMISSION_ONLY: report={output}")
                return 0
        if not track_gain_selected:
            report.update({"status": "not_exercised", "reason": f"{args.public_case_id} open run did not autonomously select track_gain"})
            write_report(output, report)
            print(f"D1-S1 NOT_EXERCISED: report={output}")
            return NOT_EXERCISED_EXIT
        report["validation"] = validate_d1(args.agent_http, conversation_id, responses, args.timeout_sec, args.public_case_id)
        if args.settlement_probe:
            # The probe judgment is machine-originated and permanently marked
            # as such; it exercises the settlement machinery on this temporary
            # engineering copy only and never claims a human decision.
            report["settlement_probe"] = run_settlement_probe(args.agent_http, conversation_id, report["project_setup"]["project_path"], report["validation"], args.settlement_probe, args.timeout_sec, run_started)
            report["status"] = "settlement_probe_pass"
        else:
            report["status"] = "pass"
        report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        write_report(output, report)
        if args.settlement_probe:
            print(f"D1-S1 SETTLEMENT({args.settlement_probe}) PASS: report={output}")
        else:
            print(f"D1-S1 PASS: report={output}")
        return 0
    except KeyboardInterrupt:
        report.update({"status": "interrupted", "reason": "operator_interrupted", "responses": responses, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
        if report.get("project_setup") and report.get("conversation_id"):
            try:
                report["last_runtime_status"] = request_json("GET", args.agent_http.rstrip("/") + "/agent/runtime/status", None, 10)
                report["continuations"] = continuation_rows(args.agent_http, str(report["conversation_id"]), 10)
                report["persisted_loop"] = persisted_free_state_loop(report["project_setup"]["project_path"], str(report["conversation_id"]), float(report.get("started_at_epoch", time.time())))
            except Exception as snapshot_error:
                report["snapshot_error"] = str(snapshot_error)
        write_report(output, report)
        print(f"D1-S1 INTERRUPTED: report={output}", file=sys.stderr)
        return 130
    except Exception as exc:  # noqa: BLE001 - smoke reports fail closed.
        report.update({"status": "fail", "error": str(exc), "responses": responses, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
        if isinstance(exc, SchedulerDrainTimeout):
            report["continuation_timeline"] = exc.timeline
            report["last_runtime_status"] = exc.runtime_status
            report["continuations"] = exc.continuations
            report["terminal_causes"] = [first_text(item.get("free_state_stop_reason"), item.get("last_error"), item.get("status")) for item in exc.continuations]
        if report.get("project_setup"):
            persisted = persisted_free_state_loop(report["project_setup"]["project_path"], str(report.get("conversation_id", "")), float(report.get("started_at_epoch", time.time())))
            if persisted:
                report["persisted_loop"] = persisted
        write_report(output, report)
        print(f"D1-S1 FAIL: {exc}; report={output}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
