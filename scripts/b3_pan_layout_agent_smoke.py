"""Godot-product-path smoke for B3 static pan-layout planning and execution."""

from __future__ import annotations

import argparse
import json
import time
import urllib.request
from pathlib import Path
from typing import Any


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        result = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(result, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return result


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retry details belong in smoke artifacts.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready at {base_url}: {last_error}")


def execution_tools(response: dict[str, Any]) -> list[str]:
    tools: list[str] = []
    for row in response.get("executed_kernel_reply", []):
        if isinstance(row, dict):
            name = str(row.get("tool", row.get("command_name", ""))).strip()
            if name:
                tools.append(name)
    return tools


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result")
    return result if isinstance(result, dict) else {}


def invoke_tool(
    base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "b3_pan_layout_agent_smoke.fixture",
        "confirmed": confirmed,
    }
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:2200]}")
    return response


def track_pans(base_url: str, timeout: float) -> dict[str, float]:
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    pans: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = row.get("pan", row.get("pan_value"))
        if track_id and isinstance(value, (int, float)):
            pans[track_id] = float(value)
    return pans


def prepare_fixture(base_url: str, project_path: Path, audio_path: Path, timeout: float) -> dict[str, Any]:
    invoke_tool(base_url, "project.new", {}, timeout, confirmed=True)
    invoke_tool(base_url, "project.save_as", {"file_path": str(project_path)}, timeout, confirmed=True)
    created: list[dict[str, str]] = []
    for name in (
        "Lead Vocal",
        "Guitar Double 1",
        "Guitar Double 2",
        "Backing Vocal 1",
        "Backing Vocal 2",
        "Percussion 1",
        "Percussion 2",
    ):
        response = invoke_tool(base_url, "track.add_audio", {"name": name}, timeout, confirmed=True)
        result = result_map(response)
        track_id = str(result.get("track_id") or result.get("id") or result.get("item_id") or "").strip()
        if not track_id:
            raise RuntimeError(f"track.add_audio did not return track_id for {name}: {json.dumps(response, ensure_ascii=False)[:1200]}")
        invoke_tool(
            base_url,
            "track.rename",
            {"track_id": track_id, "name": name, "track_name": name},
            timeout,
            confirmed=True,
        )
        invoke_tool(
            base_url,
            "clip.import_audio",
            {"track_id": track_id, "file_path": str(audio_path), "offset_time": 0.0},
            timeout,
            confirmed=True,
        )
        created.append({"track_id": track_id, "track_name": name})
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    return {"project_path": str(project_path), "created": created, "track_count": len(state.get("tracks", []))}


def request_project_l3_preflight(base_url: str, timeout: float) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {
            "tool": "mix.observe",
            "confirmed": True,
            "source": "b3_pan_layout_agent_smoke.fixture_l3_preflight",
            "args": {
                "scope": "full_project",
                "project_context": True,
                "observation_only": True,
                "observation_ready_gate": True,
                "disclosure": "digest_catalog",
                "mom_intent": "action_preflight_observation",
                "mix_session_id": "b3_pan_layout_fixture_l3_preflight",
                "goal_text": "Prepare isolated B3 fixture L3 stereo evidence",
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


def wait_mom_relationships(base_url: str, timeout: float) -> dict[str, Any]:
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
                "mix_session_id": "b3_pan_layout_fixture_readiness",
                "goal_text": "B3 fixture relationship readiness",
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
        stereo = relation.get("stereo_distribution") if isinstance(relation.get("stereo_distribution"), dict) else {}
        relation_status = str(relation.get("status", "")).strip().lower()
        stereo_status = str(stereo.get("status", "")).strip().lower()
        if (
            str(result.get("observation_id", "")).strip()
            and projection
            and relation_status == "ready"
            and stereo_status == "ready"
        ):
            return {
                "observation_id": result.get("observation_id"),
                "multitrack_relation_status": relation_status,
                "stereo_distribution_status": stereo_status,
                "track_count": relation.get("track_count"),
                "missing_track_count": relation.get("missing_track_count"),
            }
        time.sleep(0.5)
    raise RuntimeError(
        "B3 fixture MOM stereo relationships did not become ready: "
        + json.dumps(result_map(latest), ensure_ascii=False)[:4000]
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    parser.add_argument("--prepare-stems-folder", default="")
    parser.add_argument("--skip-fixture", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b3_pan_layout_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)
    fixture = None
    if args.prepare_stems_folder:
        stems_folder = Path(args.prepare_stems_folder).resolve()
        if not stems_folder.is_dir():
            raise RuntimeError(f"fixture stems folder not found: {stems_folder}")
        from b2_static_balance_agent_smoke import prepare_stems_fixture

        project_path = artifact_dir / "fixture_project.vit"
        fixture = prepare_stems_fixture(
            args.agent_http,
            stems_folder,
            project_path,
            args.timeout_sec,
            args.dad_timeout_sec,
        )
        fixture["project_path"] = str(project_path)
        fixture["stems_folder"] = str(stems_folder)
        fixture["l3_preflight"] = request_project_l3_preflight(args.agent_http, args.timeout_sec)
        fixture["mom_readiness"] = wait_mom_relationships(args.agent_http, args.timeout_sec)
    elif not args.skip_fixture:
        audio_path = repo / "test_target_3s.wav"
        if not audio_path.is_file():
            raise RuntimeError(f"fixture audio not found: {audio_path}")
        fixture = prepare_fixture(args.agent_http, artifact_dir / "fixture_project.vit", audio_path, args.timeout_sec)

    before = track_pans(args.agent_http, args.timeout_sec)
    conversation_id = f"b3_pan_layout_smoke_{run_stamp}"
    pending = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": "请按现代流行风格执行 B3 全工程静态声像布局",
            "context": {"agent_mode": "chat"},
        },
        args.timeout_sec,
    )
    (artifact_dir / "pending_response.json").write_text(
        json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    preflight_tools = execution_tools(pending)
    workflow_data = pending.get("workflow_data") if isinstance(pending.get("workflow_data"), dict) else {}
    presentation = (
        workflow_data.get("proposal_presentation")
        if isinstance(workflow_data.get("proposal_presentation"), dict)
        else {}
    )
    actions = presentation.get("actions") if isinstance(presentation.get("actions"), list) else workflow_data.get("actions")
    if not isinstance(actions, list):
        actions = []
    interaction_kinds = [
        str(row.get("kind", ""))
        for row in pending.get("interaction_requests", [])
        if isinstance(row, dict)
    ]
    interaction = next(
        (
            row
            for row in pending.get("interaction_requests", [])
            if isinstance(row, dict)
            and str(row.get("kind", "")) in {"proposal_approval", "pan_layout_confirmation"}
        ),
        None,
    )
    current_runtime = str(pending.get("workflow", "")) == "capability_runtime_v1"
    evidence_refs = {str(value) for value in presentation.get("evidence_refs", [])}
    required_evidence = {"mix.observe", "project.state"}
    legacy_required_preflight = {"mix.observe", "project.state"}
    missing_preflight = sorted(legacy_required_preflight.difference(preflight_tools)) if not current_runtime else []
    forbidden_preflight = sorted(
        {"mix.apply_pan_layout_batch", "mix.apply_tick", "track.pan", "clip.gain.set"}.intersection(preflight_tools)
    )
    runtime_binding_ok = (
        str(workflow_data.get("canary_stage", "")) == "proposal"
        and str(workflow_data.get("capability_id", "")) == "static_mix.pan_layout.v0"
        and str(workflow_data.get("proposal_id", "")).strip()
        and int(workflow_data.get("proposal_revision", 0) or 0) > 0
        and str(workflow_data.get("action_set_hash", "")).strip()
        and required_evidence.issubset(evidence_refs)
    )
    legacy_binding_ok = (
        str(pending.get("workflow", "")) == "pan_layout"
        and str(pending.get("stop_reason", "")) == "needs_confirmation"
        and "pan_layout_confirmation" in interaction_kinds
        and not missing_preflight
    )
    after_pending = track_pans(args.agent_http, args.timeout_sec)
    changed_before_confirmation = [
        track_id for track_id, value in before.items() if abs(after_pending.get(track_id, value) - value) > 0.001
    ]
    pending_ok = (
        pending.get("needs_confirmation") is True
        and isinstance(interaction, dict)
        and len(actions) > 0
        and not forbidden_preflight
        and not changed_before_confirmation
        and (runtime_binding_ok if current_runtime else legacy_binding_ok)
    )
    if not pending_ok:
        raise RuntimeError(
            "B3 did not produce a read-only governed pending pan-layout proposal: "
            + json.dumps(
                {
                    "workflow": pending.get("workflow"),
                    "stop_reason": pending.get("stop_reason"),
                    "needs_confirmation": pending.get("needs_confirmation"),
                    "canary_stage": workflow_data.get("canary_stage"),
                    "capability_id": workflow_data.get("capability_id"),
                    "proposal_id": workflow_data.get("proposal_id"),
                    "proposal_revision": workflow_data.get("proposal_revision"),
                    "action_set_hash": workflow_data.get("action_set_hash"),
                    "action_count": len(actions),
                    "interaction_kinds": interaction_kinds,
                    "preflight_tools": preflight_tools,
                    "missing_preflight": missing_preflight,
                    "forbidden_preflight": forbidden_preflight,
                    "evidence_refs": sorted(evidence_refs),
                    "changed_before_confirmation": changed_before_confirmation,
                    "reply": pending.get("reply"),
                },
                ensure_ascii=False,
            )
        )

    decision = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/interaction/respond",
        {
            "interaction_id": str(interaction.get("id", "")),
            "action_id": "approve",
            "decision": "approve",
            "payload": {},
        },
        args.timeout_sec,
    )
    (artifact_dir / "approve_response.json").write_text(
        json.dumps(decision, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    decision_tools = execution_tools(decision)
    decision_workflow = decision.get("workflow_data") if isinstance(decision.get("workflow_data"), dict) else {}
    if current_runtime:
        verification_result = decision_workflow.get("verification_result")
        verification_status = (
            str(verification_result.get("status", ""))
            if isinstance(verification_result, dict)
            else str(verification_result or "")
        )
        execution_ok = (
            str(decision.get("workflow", "")) == "capability_runtime_v1"
            and str(decision_workflow.get("canary_stage", "")) == "executed_verified"
            and verification_status == "pass"
            and int(decision_workflow.get("receipt_count", 0) or 0) > 0
            and str(decision.get("goal_status", "")) == "completed"
        )
    else:
        execution_ok = (
            decision_tools == ["mix.apply_pan_layout_batch", "project.state", "mix.observe"]
            and str(decision.get("stop_reason", "")) == "pan_layout_applied_verified"
        )
    if not execution_ok:
        raise RuntimeError(
            "B3 approval did not complete through governed execution and verification: "
            + json.dumps(
                {
                    "workflow": decision.get("workflow"),
                    "goal_status": decision.get("goal_status"),
                    "canary_stage": decision_workflow.get("canary_stage"),
                    "execution_status": decision_workflow.get("execution_status"),
                    "verification_result": decision_workflow.get("verification_result"),
                    "receipt_count": decision_workflow.get("receipt_count"),
                    "tools": decision_tools,
                    "stop_reason": decision.get("stop_reason"),
                    "reply": decision.get("reply"),
                },
                ensure_ascii=False,
            )[:4000]
        )

    targets = {
        str(row.get("track_id", "")): float(row.get("target", row.get("target_pan")))
        for row in actions
        if isinstance(row, dict)
        and str(row.get("track_id", "")).strip()
        and isinstance(row.get("target", row.get("target_pan")), (int, float))
    }
    after = track_pans(args.agent_http, args.timeout_sec)
    mismatched = [
        track_id for track_id, target in targets.items() if track_id not in after or abs(after[track_id] - target) > 0.01
    ]
    changed_non_targets = [
        track_id
        for track_id, value in before.items()
        if track_id not in targets and abs(after.get(track_id, value) - value) > 0.001
    ]
    if len(targets) != len(actions) or mismatched or changed_non_targets:
        raise RuntimeError(
            f"B3 pan verification failed: targets={len(targets)}/{len(actions)} "
            f"mismatched={mismatched} changed_non_targets={changed_non_targets}"
        )

    history_messages = (decision.get("project_history") or {}).get("conversation_messages") or []
    if (
        len(history_messages) < 2
        or not isinstance(history_messages[-2], dict)
        or not isinstance(history_messages[-1], dict)
        or str(history_messages[-2].get("role", "")) != "user"
        or str(history_messages[-1].get("role", "")) != "assistant"
        or str(history_messages[-1].get("content", "")) != str(decision.get("reply", ""))
    ):
        raise RuntimeError(
            "B3 confirmation was not appended to project conversation history: "
            + json.dumps(history_messages[-4:], ensure_ascii=False)[:2400]
        )

    project_path = Path(str((fixture or {}).get("project_path", "")))
    reopen_pans: dict[str, float] = {}
    reopen_history: list[Any] = []
    if project_path.is_file():
        invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
        invoke_tool(args.agent_http, "project.new", {}, args.timeout_sec, confirmed=True)
        invoke_tool(
            args.agent_http,
            "project.open",
            {"file_path": str(project_path), "project_path": str(project_path)},
            args.timeout_sec,
            confirmed=True,
        )
        reopen_pans = track_pans(args.agent_http, args.timeout_sec)
        reopen_mismatched = [
            track_id
            for track_id, target in targets.items()
            if track_id not in reopen_pans or abs(reopen_pans[track_id] - target) > 0.01
        ]
        agent_state = request_json("GET", args.agent_http.rstrip("/") + "/agent/state", None, args.timeout_sec)
        (artifact_dir / "reopened_agent_state.json").write_text(
            json.dumps(agent_state, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        project_history = agent_state.get("project_history") if isinstance(agent_state.get("project_history"), dict) else {}
        reopen_history = project_history.get("conversation_messages") if isinstance(project_history.get("conversation_messages"), list) else []
        if (
            reopen_mismatched
            or len(reopen_history) < 2
            or not isinstance(reopen_history[-1], dict)
            or str(reopen_history[-1].get("role", "")) != "assistant"
            or str(reopen_history[-1].get("content", "")) != str(decision.get("reply", ""))
        ):
            raise RuntimeError(
                "B3 save/reopen verification failed: "
                + json.dumps(
                    {
                        "pan_mismatches": reopen_mismatched,
                        "history_tail": reopen_history[-4:],
                        "expected_reply": decision.get("reply"),
                    },
                    ensure_ascii=False,
                )[:3200]
            )

    summary = {
        "schema_version": "b3_pan_layout_agent_smoke.v2",
        "status": "ok",
        "health": health,
        "fixture": fixture,
        "conversation_id": conversation_id,
        "preflight_tools": preflight_tools,
        "workflow": pending.get("workflow"),
        "capability_id": workflow_data.get("capability_id"),
        "proposal_id": workflow_data.get("proposal_id"),
        "proposal_revision": workflow_data.get("proposal_revision"),
        "action_count": len(actions),
        "changed_before_confirmation": changed_before_confirmation,
        "decision_tools": decision_tools,
        "decision_stop_reason": decision.get("stop_reason"),
        "target_mismatches": mismatched,
        "changed_non_targets": changed_non_targets,
        "history_tail_roles": [str(row.get("role", "")) for row in history_messages[-2:] if isinstance(row, dict)],
        "save_reopen_verified": bool(reopen_pans),
        "reopen_target_count": len(targets),
        "reopen_history_tail_roles": [
            str(row.get("role", "")) for row in reopen_history[-2:] if isinstance(row, dict)
        ],
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
