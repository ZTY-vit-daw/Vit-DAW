"""Live product-path acceptance for conversational Capability Proposals.

The caller supplies a running VitAgent/VitApp project whose B2/B3 Readiness is
already satisfied. The script uses only public Chat/interaction/tool endpoints.
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

from b2_static_balance_agent_smoke import request_json, result_map, wait_agent
from capability_runtime_v1_live_smoke import (
    prepare_stems_fixture,
    request_project_l3_preflight,
    wait_mom_relationships,
)


CAPABILITIES = {
    "b2": ("static_mix.static_balance.v0", "请分析当前工程并给出 B2 静态平衡方案", ("volume_db", "fader_db", "gain_db"), "dB"),
    "b3": ("static_mix.pan_layout.v0", "请分析当前工程并给出 B3 声像布局方案", ("pan", "pan_value"), "pan"),
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def invoke(base_url: str, tool: str, args: dict[str, Any], timeout: float) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "source": "capability_proposal_interaction_live_smoke", "confirmed": True},
        timeout,
    )
    require(str(response.get("status", "")).lower() in {"ok", "success", "completed"}, f"{tool} failed: {response}")
    return response


def project_values(base_url: str, keys: tuple[str, ...], timeout: float) -> dict[str, float]:
    state = result_map(invoke(base_url, "project.state", {}, timeout))
    values: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = next((row.get(key) for key in keys if isinstance(row.get(key), (int, float))), None)
        if track_id and isinstance(value, (int, float)):
            values[track_id] = float(value)
    require(len(values) >= 2, f"project does not expose enough comparable tracks for {keys}")
    return values


def chat(base_url: str, conversation_id: str, capability_id: str, message: str, timeout: float) -> dict[str, Any]:
    return request_json(
        "POST",
        base_url.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": message,
            "context": {
                "agent_mode": "chat",
                "capability_runtime_v1": True,
                "capability_id": capability_id,
                "interaction_mode": "propose",
            },
        },
        timeout,
    )


def workflow(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def presentation(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("proposal_presentation")
    if isinstance(value, dict):
        return value
    value = workflow(response).get("proposal_presentation")
    return value if isinstance(value, dict) else {}


def proposal_interaction(response: dict[str, Any]) -> dict[str, Any]:
    for row in response.get("interaction_requests", []):
        if isinstance(row, dict) and str(row.get("kind", "")).lower() == "proposal_approval":
            return row
    return {}


def save(artifact_dir: Path, name: str, value: dict[str, Any]) -> None:
    (artifact_dir / name).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def run(args: argparse.Namespace) -> dict[str, Any]:
    capability_id, initial_message, value_keys, unit = CAPABILITIES[args.capability]
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    wait_agent(args.agent_http, args.timeout_sec)
    fixture: dict[str, Any] | None = None
    if args.stems_folder:
        stems = Path(args.stems_folder).resolve()
        require(stems.is_dir(), f"stems folder not found: {stems}")
        fixture = prepare_stems_fixture(
            args.agent_http,
            stems,
            artifact_dir / "fixture_project.vit",
            args.timeout_sec,
            args.dad_timeout_sec,
        )
        fixture["l3_preflight"] = request_project_l3_preflight(args.agent_http, args.timeout_sec)
        fixture["mom_readiness"] = wait_mom_relationships(args.agent_http, args.timeout_sec, require_ready=True)
    conversation_id = f"proposal_interaction_{args.capability}_{int(time.time() * 1000)}"
    before = project_values(args.agent_http, value_keys, args.timeout_sec)

    proposed = chat(args.agent_http, conversation_id, capability_id, initial_message, args.timeout_sec)
    save(artifact_dir, "01_proposal.json", proposed)
    proposed_flow, proposed_view, r1_interaction = workflow(proposed), presentation(proposed), proposal_interaction(proposed)
    require(
        proposed.get("workflow") == "capability_runtime_v1"
        and proposed.get("needs_confirmation") is True
        and proposed_flow.get("canary_stage") == "proposal"
        and proposed_view.get("schema_version") == "vit.proposal_presentation.v1"
        and int(proposed_view.get("analyzed_tracks") or 0) >= 2
        and int(proposed_view.get("action_count") or 0) > 0
        and len(proposed_view.get("analysis_summary") or []) > 0
        and len(proposed_view.get("change_groups") or []) > 0
        and len(proposed_view.get("actions") or []) > 0
        and r1_interaction,
        "proposal presentation/interaction contract failed",
    )
    require(project_values(args.agent_http, value_keys, args.timeout_sec) == before, "proposal mutated project")

    questioned = chat(args.agent_http, conversation_id, capability_id, "为什么这样调整？", args.timeout_sec)
    save(artifact_dir, "02_question.json", questioned)
    questioned_flow = workflow(questioned)
    require(
        questioned_flow.get("canary_stage") == "proposal_question"
        and questioned_flow.get("proposal_id") == proposed_flow.get("proposal_id")
        and questioned_flow.get("proposal_revision") == proposed_flow.get("proposal_revision")
        and project_values(args.agent_http, value_keys, args.timeout_sec) == before,
        "question authorized, revised, or mutated the proposal",
    )

    revised2 = chat(args.agent_http, conversation_id, capability_id, f"最多调整 0.30 {unit}", args.timeout_sec)
    save(artifact_dir, "03_revision_2.json", revised2)
    revised2_flow, revised2_view, r2_interaction = workflow(revised2), presentation(revised2), proposal_interaction(revised2)
    r2_deltas = [abs(float(row.get("delta") or 0)) for row in revised2_view.get("actions", []) if isinstance(row, dict)]
    require(
        revised2_flow.get("canary_stage") == "proposal_revised"
        and int(revised2_flow.get("proposal_revision") or 0) == int(proposed_flow.get("proposal_revision") or 0) + 1
        and revised2_flow.get("proposal_id") != proposed_flow.get("proposal_id")
        and revised2_flow.get("action_set_hash") != proposed_flow.get("action_set_hash")
        and revised2_flow.get("project_cut_hash") == proposed_flow.get("project_cut_hash")
        and r2_deltas
        and max(r2_deltas) <= 0.30001
        and r2_interaction,
        "revision 2 identity or bounded delta contract failed",
    )
    require(project_values(args.agent_http, value_keys, args.timeout_sec) == before, "revision 2 mutated project")

    revised3 = chat(args.agent_http, conversation_id, capability_id, f"最多调整 0.20 {unit}", args.timeout_sec)
    save(artifact_dir, "04_revision_3.json", revised3)
    revised3_flow = workflow(revised3)
    require(
        int(revised3_flow.get("proposal_revision") or 0) == int(revised2_flow.get("proposal_revision") or 0) + 1
        and revised3_flow.get("proposal_id") != revised2_flow.get("proposal_id")
        and revised3_flow.get("action_set_hash") != revised2_flow.get("action_set_hash"),
        "revision 3 did not invalidate revision 2",
    )

    stale = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/interaction/respond",
        {"interaction_id": str(r2_interaction.get("id", "")), "action_id": "approve", "decision": "approve", "payload": {}},
        args.timeout_sec,
    )
    save(artifact_dir, "05_stale_approval.json", stale)
    require(
        "过期" in str(stale.get("reply", ""))
        and project_values(args.agent_http, value_keys, args.timeout_sec) == before,
        "stale approval was not safely rejected",
    )

    ambiguous = chat(args.agent_http, conversation_id, capability_id, "我再想想", args.timeout_sec)
    save(artifact_dir, "06_ambiguous.json", ambiguous)
    ambiguous_flow = workflow(ambiguous)
    require(
        ambiguous_flow.get("canary_stage") == "proposal_ambiguous"
        and ambiguous_flow.get("proposal_id") == revised3_flow.get("proposal_id")
        and project_values(args.agent_http, value_keys, args.timeout_sec) == before,
        "ambiguous text changed or authorized the proposal",
    )

    executed = chat(args.agent_http, conversation_id, capability_id, "执行这个方案", args.timeout_sec)
    save(artifact_dir, "07_natural_language_execution.json", executed)
    executed_flow = workflow(executed)
    verification = executed_flow.get("verification_result") if isinstance(executed_flow.get("verification_result"), dict) else {}
    decision = executed_flow.get("approval_decision") if isinstance(executed_flow.get("approval_decision"), dict) else {}
    require(
        executed.get("goal_status") == "completed"
        and executed_flow.get("canary_stage") == "executed_verified"
        and executed_flow.get("proposal_id") == revised3_flow.get("proposal_id")
        and executed_flow.get("proposal_revision") == revised3_flow.get("proposal_revision")
        and executed_flow.get("action_set_hash") == revised3_flow.get("action_set_hash")
        and executed_flow.get("project_cut_hash") == revised3_flow.get("project_cut_hash")
        and int(executed_flow.get("receipt_count") or 0) > 0
        and verification.get("status") == "pass"
        and verification.get("structural") == "pass"
        and verification.get("acoustic") == "pass"
        and verification.get("user_acceptance") == "unknown"
        and decision.get("kind") == "approve"
        and decision.get("proposal_id") == revised3_flow.get("proposal_id")
        and decision.get("proposal_revision") == revised3_flow.get("proposal_revision")
        and str(decision.get("source_turn_id", "")).strip()
        and any(str(ref).startswith("project-history:") for ref in executed_flow.get("persistence_refs", [])),
        "natural-language authorization/execution contract failed: " + json.dumps(executed, ensure_ascii=False)[:5000],
    )
    summary = {
        "schema_version": "capability_proposal_interaction_live_smoke.v1",
        "status": "ok",
        "capability_id": capability_id,
        "conversation_id": conversation_id,
        "session_id": executed_flow.get("session_id"),
        "proposal_revisions": [
            proposed_flow.get("proposal_revision"),
            revised2_flow.get("proposal_revision"),
            revised3_flow.get("proposal_revision"),
        ],
        "final_proposal_id": revised3_flow.get("proposal_id"),
        "receipt_count": executed_flow.get("receipt_count"),
        "verification": verification,
        "persistence_refs": executed_flow.get("persistence_refs"),
        "fixture": fixture,
        "checks": {
            "presentation_visible": True,
            "question_read_only": True,
            "revision_invalidates_old_identity": True,
            "stale_button_rejected": True,
            "ambiguous_text_read_only": True,
            "natural_language_exact_approval": True,
        },
    }
    save(artifact_dir, "summary.json", summary)
    return summary


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--capability", choices=tuple(CAPABILITIES), required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    parser.add_argument("--stems-folder", default="")
    args = parser.parse_args()
    print(json.dumps(run(args), ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
