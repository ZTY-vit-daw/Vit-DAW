#!/usr/bin/env python3
"""Godot-owned full-project product smoke for C2 dynamic-control v1."""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


C2_CAPABILITY_ID = "fine_mix.dynamic_control.v1"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def authoritative_project_change(value: Any) -> bool:
    return (
        isinstance(value, dict)
        and value.get("authoritative") is True
        and isinstance(value.get("changed_entities"), list)
        and len(value["changed_entities"]) > 0
    )


def validate_leaf_receipts(receipts: Any) -> None:
    """Validate the per-target evidence emitted by the C2 typed batch."""
    require(isinstance(receipts, list) and len(receipts) >= 1,
            "C2 batch did not execute at least one observed dynamic leaf")
    for index, receipt in enumerate(receipts, start=1):
        require(isinstance(receipt, dict), f"C2 leaf {index} receipt is not an object")
        require(receipt.get("status") == "executed", f"C2 leaf {index} was not executed")
        require(matrix.first_text(receipt, "track_id"), f"C2 leaf {index} track identity is missing")
        require(matrix.first_text(receipt, "plugin_id"), f"C2 leaf {index} plugin identity is missing")
        controller = receipt.get("controller_result") if isinstance(receipt.get("controller_result"), dict) else {}
        require(controller.get("status") == "exact", f"C2 leaf {index} lacks exact controller readback")
        require(matrix.first_text(controller, "restore_ref"), f"C2 leaf {index} restore_ref is missing")
        audit = receipt.get("parameter_audit")
        if isinstance(audit, dict):
            require(audit.get("status") == "pass", f"C2 leaf {index} parameter audit did not pass")


def validate_post_action_observations(observations: Any) -> None:
    require(isinstance(observations, list) and len(observations) >= 1,
            "C2 post-action CCB observations are missing")
    for index, observation in enumerate(observations, start=1):
        require(isinstance(observation, dict), f"C2 post-action observation {index} is not an object")
        require(observation.get("status") == "observed",
                f"C2 post-action observation {index} was not completed")
        require(observation.get("requires_fresh_observation") is False,
                f"C2 post-action observation {index} is not fresh")
        require(matrix.first_text(observation, "observation_id"),
                f"C2 post-action observation {index} omitted observation_id")
        audit = observation.get("audit_receipt") if isinstance(observation.get("audit_receipt"), dict) else {}
        freshness = audit.get("freshness") if isinstance(audit.get("freshness"), dict) else {}
        require(audit.get("status") in {"executed", "partial"},
                f"C2 post-action observation {index} audit did not execute")
        require(freshness.get("status") == "ready",
                f"C2 post-action observation {index} audit is stale")


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else response


def tracks_from_state(state: dict[str, Any]) -> list[dict[str, Any]]:
    for candidate in (
        state.get("tracks"),
        (state.get("project") or {}).get("tracks") if isinstance(state.get("project"), dict) else None,
        (state.get("project_observe") or {}).get("tracks") if isinstance(state.get("project_observe"), dict) else None,
    ):
        if isinstance(candidate, list):
            return [row for row in candidate if isinstance(row, dict)]
    def walk(value: Any) -> list[dict[str, Any]]:
        if isinstance(value, dict):
            rows = value.get("tracks")
            if isinstance(rows, list) and all(isinstance(row, dict) for row in rows):
                return rows
            for nested in value.values():
                found = walk(nested)
                if found:
                    return found
        elif isinstance(value, list):
            for nested in value:
                found = walk(nested)
                if found:
                    return found
        return []
    return walk(state)


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = True) -> dict[str, Any]:
    return matrix.require_ok(
        matrix.invoke(base, tool, args, timeout, confirmed),
        tool,
    )


def state_signature(state: dict[str, Any]) -> str:
    """Stable enough for the proposal zero-write gate, without raw audio data."""
    rows = tracks_from_state(state)
    compact: list[dict[str, Any]] = []
    for row in rows:
        if not isinstance(row, dict):
            continue
        compact.append({
            "track_id": row.get("track_id", row.get("id")),
            "name": row.get("name", row.get("track_name")),
            "volume": row.get("volume", row.get("volume_db", row.get("fader_db"))),
            "pan": row.get("pan", row.get("pan_value")),
            "plugins": row.get("plugins", row.get("plugin_instances")),
        })
    return json.dumps(compact, ensure_ascii=False, sort_keys=True, default=str)


def first_interaction(response: dict[str, Any], *workflows: str) -> dict[str, Any]:
    rows = response.get("interaction_requests") if isinstance(response.get("interaction_requests"), list) else []
    for row in rows:
        if not isinstance(row, dict):
            continue
        if not workflows or matrix.first_text(row, "workflow") in workflows:
            return row
    return {}


def interaction_decision(interaction: dict[str, Any]) -> str:
    actions = interaction.get("actions") if isinstance(interaction.get("actions"), list) else []
    for action in actions:
        if isinstance(action, dict) and matrix.first_text(action, "id").startswith("select_"):
            return matrix.first_text(action, "id")
    return "approve"


def respond(base: str, interaction: dict[str, Any], decision: str, timeout: float) -> dict[str, Any]:
    interaction_id = matrix.first_text(interaction, "id")
    require(interaction_id, "C2 interaction omitted id")
    return matrix.request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": interaction_id, "action_id": decision, "decision": decision, "payload": {},
    }, timeout)


def isolate_full_project(source: Path, artifact_dir: Path) -> Path:
    """Copy the complete project package, never synthesize or mutate a fixture."""
    require(source.is_file(), f"C2 source project is missing: {source}")
    destination_dir = artifact_dir / "isolated_project_package"
    if destination_dir.exists():
        raise RuntimeError(f"isolated project destination already exists: {destination_dir}")
    shutil.copytree(source.parent, destination_dir)
    destination = destination_dir / source.name
    require(destination.is_file(), "full project copy omitted the .vit project file")
    return destination


def c2_stage(response: dict[str, Any]) -> str:
    data = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    return str(data.get("stage", data.get("status", ""))).strip()


def request_c2_until_observation_ready(base: str, conversation_id: str, message: str, timeout: float) -> dict[str, Any]:
    """Resume the same durable C2 session when cold-start evidence is pending."""
    pending: list[dict[str, Any]] = []
    for attempt in range(5):
        response = matrix.request_json("POST", base.rstrip("/") + "/agent/chat", {
            "conversation_id": conversation_id,
            "message": message,
            "context": {
                "agent_mode": "chat",
                "product_path_smoke": True,
                "product_lifecycle": "godot_project",
                "capability_id": C2_CAPABILITY_ID,
                "interaction_mode": "propose",
                "smoke_contract": "c2_dynamic_control_product_smoke.v2",
            },
        }, timeout)
        if c2_stage(response) != "observation_pending":
            if pending:
                response["c2_observation_resume_attempts"] = pending
            return response
        pending.append({
            "attempt": attempt + 1,
            "reason": (response.get("workflow_data") or {}).get("observation_reason", ""),
        })
        time.sleep(1.0)
    raise RuntimeError(f"C2 observation remained pending after {len(pending)} resume attempts: {pending[-1]}")


def assert_no_mutation(base: str, before: str, timeout: float, label: str) -> None:
    after = state_signature(result_map(invoke(base, "project.state", {}, timeout, confirmed=False)))
    require(after == before, f"{label} mutated the project before its confirmation boundary")


def wait_audio_analysis(base: str, timeout: float, artifact_dir: Path) -> dict[str, Any]:
    latest: dict[str, Any] = {}
    deadline = time.monotonic() + min(timeout, 180.0)
    while time.monotonic() < deadline:
        latest = invoke(base, "project.audio_analysis_status", {"latest": True}, timeout, confirmed=False)
        job = latest.get("analysis_job") if isinstance(latest.get("analysis_job"), dict) else latest
        status = str(job.get("dad_fact_status", "")).strip().lower()
        if status not in {"ready", "building", "queued", "pending", "running"} or str(job.get("analysis_queue_status", "")).lower() == "missing":
            invoke(base, "project.audio_analysis_start", {"retry_missing": True, "rebuild_from_project": True, "interval_ms": 50}, timeout)
            break
        if status == "ready":
            (artifact_dir / "audio_analysis_status.json").write_text(json.dumps(latest, ensure_ascii=False, indent=2), encoding="utf-8")
            return latest
        time.sleep(0.5)
    while time.monotonic() < deadline:
        latest = invoke(base, "project.audio_analysis_status", {"latest": True}, timeout, confirmed=False)
        job = latest.get("analysis_job") if isinstance(latest.get("analysis_job"), dict) else latest
        if str(job.get("dad_fact_status", "")).strip().lower() == "ready":
            (artifact_dir / "audio_analysis_status.json").write_text(json.dumps(latest, ensure_ascii=False, indent=2), encoding="utf-8")
            return latest
        time.sleep(0.5)
    raise RuntimeError("isolated project audio analysis did not become ready")


def run(args: argparse.Namespace) -> dict[str, Any]:
    started = time.monotonic()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    source_project = Path(args.project_path).resolve()
    source_sha256_before = file_sha256(source_project)
    isolated_project = isolate_full_project(source_project, artifact_dir)
    report: dict[str, Any] = {
        "schema_version": "c2_dynamic_control_product_smoke.v2",
        "status": "running",
        "capability_id": C2_CAPABILITY_ID,
        "source_project": str(source_project),
        "isolated_project": str(isolated_project),
        "source_sha256_before": source_sha256_before,
        "c1_referral_supplied": False,
    }
    try:
        invoke(args.agent_http, "project.open", {
            "file_path": str(isolated_project),
            "project_path": str(isolated_project),
        }, args.timeout_sec)
        # C2 owns its observation-readiness gate.  Do not prewarm DAD here or
        # this product-path smoke would miss the cold-start hand-test path.
        state = result_map(invoke(args.agent_http, "project.state", {}, args.timeout_sec, confirmed=False))
        (artifact_dir / "project_state_after_open.json").write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")
        before_state = result_map(invoke(args.agent_http, "project.state", {}, args.timeout_sec, confirmed=False))
        before_signature = state_signature(before_state)
        report.update({"before_signature": before_signature, "source_mutation_forbidden": True})

        conversation_id = f"c2_product_smoke_{int(time.time() * 1000)}"
        report["conversation_id"] = conversation_id
        proposal = request_c2_until_observation_ready(args.agent_http, conversation_id, args.message, args.timeout_sec)
        (artifact_dir / "c2_proposal.json").write_text(json.dumps(proposal, ensure_ascii=False, indent=2), encoding="utf-8")
        proposal_data = proposal.get("workflow_data") if isinstance(proposal.get("workflow_data"), dict) else {}
        c2_meta = proposal_data.get("c2") if isinstance(proposal_data.get("c2"), dict) else {}
        proposal_state = result_map(invoke(args.agent_http, "project.state", {}, args.timeout_sec, confirmed=False))
        proposal_signature = state_signature(proposal_state)
        report.update({"proposal": proposal, "proposal_stage": proposal_data.get("status", proposal_data.get("stage")),
                       "proposal_c2": c2_meta, "proposal_mutated": proposal_signature != before_signature})
        require(proposal_data.get("capability_id") == C2_CAPABILITY_ID, "chat did not remain C2-owned")
        require(not report["proposal_mutated"], "C2 observation/proposal mutated the project")
        require(c2_stage(proposal) not in {"project_treatment_discovery_failed", "pca_resolution_failed"},
                f"C2 did not produce an actionable project treatment: {proposal.get('error', proposal.get('reply'))}")
        require(c2_stage(proposal) != "no_action_needed",
                "B4 completion fixture must exercise C2's dynamic action path; no_action_needed is not an acceptance result")
        require(c2_meta.get("independent") is True and c2_meta.get("c1_referral_present") is False,
                "C2 action proposal was not independent of C1")

        interaction = first_interaction(proposal)
        require(interaction, "C2 action path omitted its first interaction")
        current = proposal
        workflow = matrix.first_text(interaction, "workflow")
        if workflow == "capability_runtime_v1":
            require(c2_stage(current) == "plugin_load_batch_confirmation", "C2 load confirmation had an unexpected stage")
            current = respond(args.agent_http, interaction, "approve", args.timeout_sec)
            (artifact_dir / "c2_plugin_load_batch_execution.json").write_text(json.dumps(current, ensure_ascii=False, indent=2), encoding="utf-8")
            require(bool((current.get("workflow_data") or {}).get("mutation_performed")), "C2 approved plugin load did not report a mutation")
            interaction = first_interaction(current)
            require(interaction, "C2 plugin load did not yield the batch parameter confirmation")
            workflow = matrix.first_text(interaction, "workflow")
        require(workflow == "c2_dynamic_parameter_batch", f"C2 did not reach its project parameter batch: {workflow}")
        execution = respond(args.agent_http, interaction, "approve", args.timeout_sec)
        (artifact_dir / "c2_parameter_batch_execution.json").write_text(json.dumps(execution, ensure_ascii=False, indent=2), encoding="utf-8")
        execution_data = execution.get("workflow_data") if isinstance(execution.get("workflow_data"), dict) else {}
        c2_after = execution_data.get("c2") if isinstance(execution_data.get("c2"), dict) else {}
        after_state = result_map(invoke(args.agent_http, "project.state", {}, args.timeout_sec, confirmed=False))
        (artifact_dir / "project_state_after_execution.json").write_text(json.dumps(after_state, ensure_ascii=False, indent=2), encoding="utf-8")
        after_signature = state_signature(after_state)
        terminal = c2_after.get("terminal_receipt") if isinstance(c2_after.get("terminal_receipt"), dict) else {}
        post_verification = c2_after.get("post_action_verification") if isinstance(c2_after.get("post_action_verification"), list) else []
        project_change = c2_after.get("project_change") if isinstance(c2_after.get("project_change"), dict) else {}
        source_sha256_after = file_sha256(source_project)
        report.update({"execution": execution, "execution_c2": c2_after, "terminal_receipt": terminal,
                       "post_action_verification": post_verification,
                       "project_changed_after_execution": after_signature != before_signature or authoritative_project_change(project_change),
                       "project_change_evidence": "state_signature" if after_signature != before_signature else "authoritative_project_change",
                       "source_sha256_after": source_sha256_after,
                       "source_project_unchanged": source_sha256_after == source_sha256_before,
                       "mixboard_decision_status": execution_data.get("mixboard_decision_status"),
                       "mixboard_decision_record_ref": execution_data.get("mixboard_decision_record_ref")})
        require(execution_data.get("mutation_performed") is True, "C2 did not report a performed mutation")
        require(isinstance(execution_data.get("execution_receipt"), dict), "C2 typed-executor batch receipt is missing")
        validate_leaf_receipts((execution_data.get("execution_receipt") or {}).get("leaf_receipts"))
        require(report["project_changed_after_execution"], "C2 action path did not change the isolated project")
        require(report["source_project_unchanged"], "C2 smoke modified the source B4 completion project")
        validate_post_action_observations(post_verification)
        require(authoritative_project_change(project_change), "C2 post-action project delta is missing or non-authoritative")
        require(str(terminal.get("status")) == "needs_review", "C2 terminal session was not needs_review")
        require(report["mixboard_decision_status"] == "recorded", "C2 terminal Mix Board projection was not recorded")
        require(str(report["mixboard_decision_record_ref"]).startswith("mixboard-decision:"),
                "C2 Mix Board record reference missing")
        report["status"] = "passed"
        return report
    except Exception as exc:  # noqa: BLE001 - preserve complete smoke artifact.
        report["status"] = "failed"
        report["error"] = str(exc)
        raise
    finally:
        report["elapsed_ms"] = int((time.monotonic() - started) * 1000)
        (artifact_dir / "summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--message", default="执行 C2 全工程动态处理：基于固定项目观察识别全部可处理动态目标，批量加载所需 PCA 合格效果器，并统一执行已冻结的动态控制。")
    parser.add_argument("--timeout-sec", type=float, default=360.0)
    args = parser.parse_args()
    try:
        print(json.dumps(run(args), ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
