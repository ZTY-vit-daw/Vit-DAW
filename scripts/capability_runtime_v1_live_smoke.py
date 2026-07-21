#!/usr/bin/env python3
"""Live product-path smoke for Capability Runtime v1 B2/B3 sessions.

The caller must start VitApp with an isolated VIT_PROJECT_XML and VitAgent with
an isolated durable VIT_ORCHESTRATION_STORE_PATH. This script deliberately
uses the public Chat and interaction endpoints instead of calling adapters
directly.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any, Callable

from b2_static_balance_agent_smoke import prepare_stems_fixture
from b3_pan_layout_agent_smoke import request_json, result_map, wait_agent


B2 = "static_mix.static_balance.v0"
B3 = "static_mix.pan_layout.v0"


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def invoke_tool(base_url: str, tool: str, args: dict[str, Any], timeout: float) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "source": "capability_runtime_v1_live_smoke", "confirmed": True},
        timeout,
    )
    require(
        str(response.get("status", "")).lower() in {"ok", "success", "completed"},
        f"tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:2400]}",
    )
    return response


def track_values(base_url: str, timeout: float, keys: tuple[str, ...]) -> dict[str, float]:
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    values: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = next((row.get(key) for key in keys if isinstance(row.get(key), (int, float))), None)
        if track_id and isinstance(value, (int, float)):
            values[track_id] = float(value)
    return values


def request_project_l3_preflight(base_url: str, timeout: float) -> dict[str, Any]:
    """Explicitly request the expensive per-track L3 feature fill for a fixture."""
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {
            "tool": "mix.observe",
            "confirmed": True,
            "source": "capability_runtime_v1_live_smoke.fixture_l3_preflight",
            "args": {
                "scope": "full_project",
                "project_context": True,
                "observation_only": True,
                "observation_ready_gate": True,
                "disclosure": "digest_catalog",
                "mom_intent": "action_preflight_observation",
                "mix_session_id": "v1_live_fixture_l3_preflight",
                "goal_text": "Prepare isolated fixture L3 evidence",
            },
        },
        timeout,
    )
    result = result_map(response)
    return {
        "status": response.get("status"),
        "result_status": result.get("status"),
        "reason": result.get("reason"),
        "missing_required_features": result.get("missing_required_features"),
    }


def wait_mom_relationships(base_url: str, timeout: float, *, require_ready: bool = True) -> dict[str, Any]:
    """Wait for the read-only MOM projection to observe the saved fixture."""
    deadline = time.monotonic() + timeout
    latest: dict[str, Any] = {}
    while time.monotonic() < deadline:
        latest = invoke_tool(
            base_url,
            "mix.observe",
            {
                "scope": "full_project",
                "project_context": True,
                "observation_only": True,
                "disclosure": "digest_catalog",
                "mom_intent": "project_multitrack_relation_observation",
                "mix_session_id": "v1_live_fixture_readiness",
                "goal_text": "Capability Runtime v1 live fixture readiness",
            },
            timeout,
        )
        result = result_map(latest)
        projection = result.get("mom_projection") if isinstance(result.get("mom_projection"), dict) else {}
        relation = (
            projection.get("multitrack_relation")
            if isinstance(projection.get("multitrack_relation"), dict)
            else {}
        )
        status = str(relation.get("status", "")).strip().lower()
        acceptable = status == "ready" if require_ready else status not in {"", "missing", "unavailable"}
        if str(result.get("observation_id", "")).strip() and projection and acceptable:
            return {
                "observation_id": result.get("observation_id"),
                "multitrack_relation_status": status,
                "track_count": relation.get("track_count"),
                "missing_track_count": relation.get("missing_track_count"),
            }
        time.sleep(0.5)
    raise RuntimeError(
        "MOM multitrack relationships did not become ready: "
        + json.dumps(result_map(latest).get("mom_projection", {}), ensure_ascii=False)[:4000]
    )


def run_capability(
    *,
    base_url: str,
    capability_id: str,
    message: str,
    timeout: float,
    artifact_dir: Path,
    value_reader: Callable[[], dict[str, float]],
    explicit_canary: bool,
) -> dict[str, Any]:
    before = value_reader()
    conversation_id = f"v1_live_{capability_id.rsplit('.', 2)[-2]}_{int(time.time() * 1000)}"
    request_context: dict[str, Any] = {
        "agent_mode": "chat",
        "capability_id": capability_id,
        "interaction_mode": "propose",
    }
    if explicit_canary:
        request_context["capability_runtime_v1"] = True
    proposal = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": message,
            "context": request_context,
        },
        timeout,
    )
    (artifact_dir / f"{capability_id}.proposal.json").write_text(
        json.dumps(proposal, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    workflow = proposal.get("workflow_data") if isinstance(proposal.get("workflow_data"), dict) else {}
    interactions = [row for row in proposal.get("interaction_requests", []) if isinstance(row, dict)]
    confirmation = next(
        (
            row
            for row in interactions
            if str(row.get("kind", "")).lower() in {"proposal_approval", "confirmation"}
            and str(row.get("workflow", "")).lower() == "capability_runtime_v1"
        ),
        None,
    )
    require(
        proposal.get("needs_confirmation") is True
        and proposal.get("workflow") == "capability_runtime_v1"
        and workflow.get("capability_id") == capability_id
        and workflow.get("canary_stage") == "proposal"
        and str(workflow.get("session_id", "")).strip()
        and str(workflow.get("project_cut_hash", "")).strip()
        and isinstance(confirmation, dict),
        "v1 proposal contract failed: " + json.dumps(proposal, ensure_ascii=False)[:4000],
    )
    after_proposal = value_reader()
    require(after_proposal == before, f"{capability_id} mutated project before authorization")

    decision = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/interaction/respond",
        {
            "interaction_id": str(confirmation.get("id", "")),
            "action_id": "approve",
            "decision": "approve",
            "payload": {},
        },
        timeout,
    )
    (artifact_dir / f"{capability_id}.execution.json").write_text(
        json.dumps(decision, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    executed = decision.get("workflow_data") if isinstance(decision.get("workflow_data"), dict) else {}
    verification = (
        executed.get("verification_result") if isinstance(executed.get("verification_result"), dict) else {}
    )
    persistence_refs = executed.get("persistence_refs") if isinstance(executed.get("persistence_refs"), list) else []
    require(
        decision.get("workflow") == "capability_runtime_v1"
        and decision.get("goal_status") == "completed"
        and executed.get("canary_stage") == "executed_verified"
        and executed.get("execution_status") == "verified"
        and int(executed.get("receipt_count") or 0) > 0
        and executed.get("verification") == "pass"
        and verification.get("status") == "pass"
        and verification.get("structural") == "pass"
        and verification.get("acoustic") == "pass"
        and verification.get("user_acceptance") == "unknown"
        and len(verification.get("evidence_refs") or []) >= 2
        and any(str(ref).startswith("project-history:") for ref in persistence_refs),
        "v1 execution/verification contract failed: " + json.dumps(decision, ensure_ascii=False)[:6000],
    )
    after_execution = value_reader()
    changed = sorted(
        track_id
        for track_id, before_value in before.items()
        if abs(after_execution.get(track_id, before_value) - before_value) > 0.0005
    )
    return {
        "capability_id": capability_id,
        "conversation_id": conversation_id,
        "session_id": workflow.get("session_id"),
        "proposal_id": workflow.get("proposal_id"),
        "receipt_count": executed.get("receipt_count"),
        "verification": verification,
        "persistence_refs": persistence_refs,
        "changed_track_ids": changed,
        "user_acceptance": executed.get("user_acceptance"),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--capability", choices=("b2", "b3", "both"), default="both")
    parser.add_argument("--stems-folder", default="", help="isolated readable stems used to make DAD/MOM verification ready")
    parser.add_argument("--artifact-dir", default="")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    parser.add_argument("--explicit-canary", action="store_true", help="bypass rollout policy for canary-only diagnostics")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = (
        Path(args.artifact_dir).resolve()
        if args.artifact_dir
        else repo / "artifacts" / "orchestration_v1_live_smoke" / f"capabilities_{stamp}"
    )
    artifact_dir.mkdir(parents=True, exist_ok=True)
    health = wait_agent(args.agent_http, args.timeout_sec)
    summaries: list[dict[str, Any]] = []
    stems = Path(args.stems_folder).resolve() if args.stems_folder else None

    if args.capability in {"b3", "both"}:
        require(stems is not None and stems.is_dir(), "B3 live verification requires --stems-folder")
        prepare_stems_fixture(
            args.agent_http,
            stems,
            artifact_dir / "b3_fixture.vit",
            args.timeout_sec,
            args.dad_timeout_sec,
        )
        b3_l3_preflight = request_project_l3_preflight(args.agent_http, args.timeout_sec)
        b3_readiness = wait_mom_relationships(args.agent_http, args.timeout_sec, require_ready=True)
        summaries.append(
            run_capability(
                base_url=args.agent_http,
                capability_id=B3,
                message="请为当前隔离测试工程生成并执行 B3 静态声像布局。",
                timeout=args.timeout_sec,
                artifact_dir=artifact_dir,
                value_reader=lambda: track_values(args.agent_http, args.timeout_sec, ("pan", "pan_value")),
                explicit_canary=args.explicit_canary,
            )
        )
        summaries[-1]["fixture_readiness"] = b3_readiness
        summaries[-1]["fixture_l3_preflight"] = b3_l3_preflight

    if args.capability in {"b2", "both"}:
        require(stems is not None and stems.is_dir(), "B2 requires --stems-folder with at least two readable stems")
        prepare_stems_fixture(
            args.agent_http,
            stems,
            artifact_dir / "b2_fixture.vit",
            args.timeout_sec,
            args.dad_timeout_sec,
        )
        b2_l3_preflight = request_project_l3_preflight(args.agent_http, args.timeout_sec)
        b2_readiness = wait_mom_relationships(args.agent_http, args.timeout_sec, require_ready=True)
        summaries.append(
            run_capability(
                base_url=args.agent_http,
                capability_id=B2,
                message="请为当前隔离测试工程生成并执行 B2 静态平衡。",
                timeout=args.timeout_sec,
                artifact_dir=artifact_dir,
                value_reader=lambda: track_values(
                    args.agent_http, args.timeout_sec, ("volume_db", "fader_db", "gain_db")
                ),
                explicit_canary=args.explicit_canary,
            )
        )
        summaries[-1]["fixture_readiness"] = b2_readiness
        summaries[-1]["fixture_l3_preflight"] = b2_l3_preflight

    status = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {"conversation_id": f"v1_status_{stamp}", "message": "/capability-runtime/status", "context": {}},
        args.timeout_sec,
    )
    authority_report = (
        (status.get("workflow_data") or {}).get("authority_report")
        if isinstance(status.get("workflow_data"), dict)
        else None
    )
    require(isinstance(authority_report, dict), "authority status report missing")
    summary = {
        "schema_version": "capability_runtime_v1_live_smoke.v1",
        "status": "ok",
        "health": health,
        "capabilities": summaries,
        "authority_report": authority_report,
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
