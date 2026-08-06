#!/usr/bin/env python3
"""Run opaque EQ/compressor fixtures through Vit's open semantic route.

This runner reads only the public fixture manifest. It never opens sealed truth
and always follows the Agent's recommended treatment and plugin choices. The
separate evaluator may compare the completed run against sealed expectations.
"""

from __future__ import annotations

import argparse
import json
import os
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

from semantic_mix_goal5_projects import build_case


SCHEMA_VERSION = "semantic_processor_open_experiment.v1"
CASE_INPUTS = {
    "so_01": ("\u4e3b\u5531\u542c\u8d77\u6765\u65f6\u5927\u65f6\u5c0f\u5417\uff1f\u5982\u679c\u786e\u5b9e\u6709\u95ee\u9898\uff0c\u518d\u5e2e\u6211\u8ba9\u5b83\u66f4\u7a33\u5b9a\uff0c\u4fdd\u7559\u81ea\u7136\u8d77\u4f0f\u548c\u54ac\u5b57\u77ac\u6001\u3002", "vocals"),
    "so_02": ("\u8ba9\u4e3b\u5531\u66f4\u7a33\u5b9a\u5730\u9760\u524d\uff0c\u4f46\u4fdd\u7559\u81ea\u7136\u8d77\u4f0f\u548c\u54ac\u5b57\u77ac\u6001\u3002", "vocals"),
    "so_03": ("\u628a\u9f13\u7ec4\u5076\u5c14\u7a81\u51fa\u7684\u5cf0\u503c\u6536\u7a33\u4e00\u4e9b\uff0c\u4f46\u4e0d\u8981\u628a\u51fb\u6253\u611f\u538b\u6241\u3002", "drums"),
    "so_04": ("\u8ba9\u8d1d\u65af\u6bcf\u4e2a\u97f3\u7684\u5b58\u5728\u611f\u66f4\u5747\u5300\uff0c\u4f46\u522b\u8ba9\u4f4e\u9891\u53d8\u8584\u3002", "bass"),
    "so_05": ("\u4e3b\u5531\u6709\u70b9\u53d1\u95f7\u3001\u4f4e\u4e2d\u9891\u5806\u5728\u4e00\u8d77\uff0c\u8ba9\u5b83\u66f4\u6e05\u695a\u5730\u7ad9\u5230\u524d\u9762\uff0c\u4f46\u522b\u628a\u58f0\u97f3\u505a\u8584\u3002", "vocals"),
    "so_06": ("\u8ba9\u4e3b\u5531\u66f4\u6301\u7eed\u5730\u7ad9\u5230\u524d\u9762\uff0c\u4fdd\u7559\u97f3\u8272\u548c\u81ea\u7136\u54ac\u5b57\u3002", "vocals"),
    "so_07": ("\u4e3b\u5531\u6709\u70b9\u7cca\uff0c\u800c\u4e14\u65f6\u5927\u65f6\u5c0f\uff0c\u5e2e\u6211\u6574\u7406\u5f97\u66f4\u6e05\u695a\u7a33\u5b9a\uff0c\u4f46\u522b\u505a\u8584\u6216\u538b\u6b7b\u3002", "vocals"),
    "so_08": ("\u4eba\u58f0\u603b\u662f\u88ab\u4f34\u594f\u76d6\u4f4f\uff0c\u81ea\u5df1\u7684\u5b58\u5728\u611f\u4e5f\u4e0d\u7a33\uff0c\u8ba9\u5b83\u66f4\u9760\u524d\uff0c\u4f46\u522b\u628a\u4f34\u594f\u505a\u8584\u3002", "vocals"),
}
RECOMMENDATION_CONFIRMATION_PROMPT = "\u786e\u8ba4\uff0c\u6309\u4f60\u521a\u624d\u63a8\u8350\u7684\u5904\u7406\u7ee7\u7eed\uff1b\u5982\u679c\u4fe1\u606f\u4e0d\u8db3\u8bf7\u505c\u6b62\uff0c\u4e0d\u8981\u731c\u6d4b\u3002"
CONTINUE_PROMPT = "\u7ee7\u7eed"


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


def rows(value: Any) -> list[dict[str, Any]]:
    return [row for row in value if isinstance(row, dict)] if isinstance(value, list) else []


def text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
    return ""


def workflow_data(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def free_state_loop(response: dict[str, Any]) -> dict[str, Any]:
    data = workflow_data(response)
    value = data.get("free_state_reasoning_loop")
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


def public_manifest_default() -> Path:
    local = os.environ.get("LOCALAPPDATA")
    if not local:
        raise RuntimeError("LOCALAPPDATA is unavailable")
    pointer = json.loads((Path(local) / "Vit" / "SemanticProcessorFixtures" / "current.json").read_text(encoding="utf-8"))
    return Path(pointer["fixture_manifest"]).resolve()


def assert_public_manifest(path: Path, manifest: dict[str, Any]) -> None:
    if "sealed" in {part.lower() for part in path.parts}:
        raise RuntimeError("refusing sealed fixture input")
    contract = manifest.get("blindness_contract") if isinstance(manifest.get("blindness_contract"), dict) else {}
    if contract.get("sealed_truth_not_in_agent_context") is not True:
        raise RuntimeError("manifest does not freeze the sealed-context boundary")
    forbidden = {"issue_kind", "blind_prompt", "expected_first_processors", "dynamic_edits", "eq_edits"}
    for case in rows(manifest.get("cases")):
        if forbidden.intersection(case):
            raise RuntimeError(f"public case {case.get('case_id')} leaks sealed fields")


def focus_from_project(project: dict[str, Any], focus_name: str) -> dict[str, str]:
    for row in rows(project.get("tracks")):
        if text(row, "track_name").lower() == focus_name.lower():
            return {"track_id": text(row, "track_id"), "track_name": text(row, "track_name"), "clip_id": text(row, "clip_id")}
    raise RuntimeError(f"rebuilt project omitted focus track {focus_name}")


def interactions(response: dict[str, Any], workflow: str = "") -> list[dict[str, Any]]:
    found = rows(response.get("interaction_requests"))
    if workflow:
        found = [row for row in found if text(row, "workflow") == workflow]
    return found


def recommended_action(request: dict[str, Any]) -> dict[str, Any]:
    actions = [row for row in rows(request.get("actions")) if text(row, "id") not in {"cancel", "deny"}]
    for row in actions:
        if row.get("recommended") is True or text(row, "style").lower() == "primary":
            return row
    for row in actions:
        if text(row, "id").startswith("select_") or text(row, "id") in {"approve", "confirm", "continue"}:
            return row
    if actions:
        return actions[0]
    raise RuntimeError(f"interaction omitted an actionable recommendation: {request}")


def respond(base: str, request: dict[str, Any], action: dict[str, Any], timeout: float) -> dict[str, Any]:
    action_id = text(action, "id")
    return request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": text(request, "id"), "decision": action_id, "action_id": action_id,
        "payload": request.get("payload") if isinstance(request.get("payload"), dict) else {},
    }, timeout)


def processor_from_response(response: dict[str, Any]) -> str:
    workflow = text(response, "workflow")
    data = workflow_data(response)
    if workflow == "semantic_compressor_execution":
        return "compressor"
    if workflow == "capability_runtime_v1" and isinstance(data.get("semantic_action"), dict):
        return "eq"
    if text(data, "processor_type"):
        return text(data, "processor_type").lower()
    qualification = data.get("post_load_qualification") if isinstance(data.get("post_load_qualification"), dict) else {}
    processor = text(qualification, "processor_type").lower()
    if processor:
        return processor
    typed_state = data.get("typed_state") if isinstance(data.get("typed_state"), dict) else {}
    processor = text(typed_state, "processor_type").lower()
    if processor:
        return processor
    candidate = typed_state.get("candidate_action") if isinstance(typed_state.get("candidate_action"), dict) else {}
    return text(candidate, "processor_type").lower()


def eq_confirmation_context(base_context: dict[str, Any], data: dict[str, Any]) -> dict[str, Any]:
    context = dict(base_context)
    for key in ("session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash"):
        context[{"session_id": "capability_session_id"}.get(key, key)] = data.get(key)
    qualification = data.get("post_load_qualification") if isinstance(data.get("post_load_qualification"), dict) else {}
    action = data.get("semantic_action") if isinstance(data.get("semantic_action"), dict) else {}
    target = action.get("target") if isinstance(action.get("target"), dict) else {}
    context["selected_plugin_track_id"] = text(qualification, "track_id") or text(target, "track_id") or text(base_context, "selected_track_id")
    context["selected_plugin_id"] = text(qualification, "plugin_id") or text(target, "plugin_id")
    context["selected_plugin_name"] = text(qualification, "plugin_name")
    return context


def asks_to_confirm_recommendation(response: dict[str, Any]) -> bool:
    if text(response, "workflow") or interactions(response):
        return False
    if text(response, "goal_status").lower() not in {"waiting_clarification", "completed"}:
        return False
    if text(response, "stop_reason").lower() not in {"needs_clarification", "done"}:
        return False
    reply = text(response, "reply", "message")
    yes_no = any(marker in reply for marker in ("\u662f\u5426", "\u8981\u6211", "\u53ef\u4ee5\u5417"))
    action = any(marker in reply for marker in ("\u786e\u8ba4", "\u5904\u7406", "\u8c03\u6574", "\u65b9\u6848", "\u6267\u884c"))
    return yes_no and action and reply.rstrip().endswith(("?", "\uff1f"))


def advance(base: str, conversation_id: str, context: dict[str, Any], response: dict[str, Any], timeout: float) -> tuple[dict[str, Any] | None, dict[str, Any]]:
    workflow = text(response, "workflow")
    data = workflow_data(response)
    record: dict[str, Any] = {"workflow": workflow, "status": text(data, "status"), "processor": processor_from_response(response)}

    loop_status = text(free_state_loop(response), "status").lower()
    if loop_status in {"blocked", "completed", "cancelled"}:
        record["transition"] = "terminal"
        return None, record

    if (
        text(response, "goal_status").lower() == "waiting_continue"
        and text(response, "stop_reason").lower() == "transient_llm_error"
        and loop_status
    ):
        resume_context = dict(context)
        for key in ("goal_id", "run_id"):
            if text(response, key):
                resume_context[key] = text(response, key)
        record.update({"transition": "continuation", "stop_reason": text(response, "stop_reason")})
        return request_json("POST", base.rstrip("/") + "/agent/chat", {
            "conversation_id": conversation_id,
            "message": CONTINUE_PROMPT,
            "context": resume_context,
        }, timeout), record

    if text(response, "goal_status").lower() == "waiting_continue" and loop_status:
        record.update({"transition": "paused", "stop_reason": text(response, "stop_reason")})
        return None, record

    if workflow in {"semantic_treatment_strategy", "plugin_recommendation_selection", "mix_treatment", "semantic_compressor_execution"}:
        requests = interactions(response, workflow)
        if requests:
            action = recommended_action(requests[0])
            record.update({"transition": "interaction", "action_id": text(action, "id"), "action_label": text(action, "label")})
            return respond(base, requests[0], action, timeout), record

    if workflow == "plugin_grabber_load_and_get_params" and response.get("needs_confirmation") is True and text(response, "plan_id"):
        record.update({"transition": "load_confirmation", "plan_id": text(response, "plan_id")})
        return request_json("POST", base.rstrip("/") + "/agent/confirm", {"plan_id": text(response, "plan_id"), "decision": "approve"}, timeout), record

    if workflow == "capability_runtime_v1" and response.get("needs_confirmation") is True:
        requests = interactions(response, workflow)
        if requests:
            action = recommended_action(requests[0])
            record.update({"transition": "eq_parameter_confirmation", "action_id": text(action, "id"), "action_label": text(action, "label")})
            return respond(base, requests[0], action, timeout), record
        required = ("session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash")
        if all(data.get(key) not in {None, ""} for key in required):
            record["transition"] = "eq_parameter_confirmation"
            return request_json("POST", base.rstrip("/") + "/agent/chat", {
                "conversation_id": conversation_id,
                "message": "\u786e\u8ba4\u6267\u884c\u8fd9\u4e2a\u65b9\u6848",
                "context": eq_confirmation_context(context, data),
            }, timeout), record

    # Open semantic routing may legitimately choose a non-plugin action such
    # as a mix tick. Follow that recommendation instead of terminating early;
    # the evaluator will still score the autonomous route against sealed truth.
    requests = interactions(response)
    if requests:
        action = recommended_action(requests[0])
        transition = "action_confirmation" if response.get("needs_confirmation") is True else "interaction"
        record.update({"transition": transition, "action_id": text(action, "id"), "action_label": text(action, "label")})
        return respond(base, requests[0], action, timeout), record

    if asks_to_confirm_recommendation(response):
        record["transition"] = "recommendation_confirmation"
        return request_json("POST", base.rstrip("/") + "/agent/chat", {
            "conversation_id": conversation_id,
            "message": RECOMMENDATION_CONFIRMATION_PROMPT,
            "context": context,
        }, timeout), record

    record["transition"] = "terminal"
    return None, record


def run_case(base: str, case: dict[str, Any], output: Path, timeout: float, analysis_timeout: float) -> dict[str, Any]:
    case_id = text(case, "case_id")
    prompt, focus_name = CASE_INPUTS[case_id]
    project = build_case(base, case, timeout, analysis_timeout)
    focus = focus_from_project(project, focus_name)
    context = {
        "agent_mode": "chat",
        "interaction_path": "agent_http_after_godot_project_lifecycle",
        "product_lifecycle": "godot_project",
        "semantic_processor_blind_experiment": True,
        "selected_track_id": focus["track_id"],
        "selected_track_name": focus["track_name"],
        "selected_clip_id": focus["clip_id"],
    }
    if "selected_plugin_id" in context:
        raise AssertionError("blind experiment context leaked plugin identity")
    conversation_id = f"semantic_processor_{case_id}_{int(time.time() * 1000)}"
    stages = []
    processor_sequence = []
    load_confirmations = 0
    parameter_confirmations = 0
    action_confirmations = 0
    raw_dir = output / "raw" / case_id
    stage_index = 0
    response = request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id, "message": prompt, "context": context,
    }, timeout)
    for _ in range(24):
        raw_path = raw_dir / f"{stage_index:02d}_{text(response, 'workflow') or 'chat'}.json"
        write_json(raw_path, response)
        next_response, stage = advance(base, conversation_id, context, response, timeout)
        loop = free_state_loop(response)
        stage.update({
            "round": 1,
            "raw_file": str(raw_path),
            "free_state_status": text(loop, "status"),
            "free_state_cycle": loop.get("cycle"),
            "free_state_original_intent_retained": text(loop, "original_intent") == prompt,
        })
        stages.append(stage)
        if stage.get("processor") and (not processor_sequence or processor_sequence[-1] != stage["processor"]):
            processor_sequence.append(stage["processor"])
        load_confirmations += int(stage.get("transition") == "load_confirmation")
        parameter_confirmations += int(stage.get("transition") in {"eq_parameter_confirmation"} or (stage.get("workflow") == "semantic_compressor_execution" and stage.get("transition") == "interaction"))
        action_confirmations += int(stage.get("transition") == "action_confirmation")
        stage_index += 1
        if next_response is None:
            break
        response = next_response
    else:
        raise RuntimeError(f"{case_id} exceeded the autonomous semantic workflow transition limit")

    final_data = workflow_data(response)
    return {
        "case_id": case_id,
        "suite": text(case, "suite"),
        "conversation_id": conversation_id,
        "prompt": prompt,
        "round_count": 1,
        "focus_track": focus_name,
        "sealed_truth_opened": False,
        "initial_plugin_identity_disclosed": False,
        "processor_sequence": processor_sequence,
        "load_confirmation_count": load_confirmations,
        "parameter_confirmation_count": parameter_confirmations,
        "action_confirmation_count": action_confirmations,
        "final_workflow": text(response, "workflow"),
        "final_status": text(final_data, "status", "execution_status") or text(response, "goal_status", "status"),
        "final_stop_reason": text(response, "stop_reason"),
        "final_reply": text(response, "reply", "message"),
        "free_state_reasoning_loop": free_state_loop(response),
        "stages": stages,
        "project": project,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--fixture-manifest", default="")
    parser.add_argument("--case", action="append", dest="cases", default=[])
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-sec", type=float, default=420.0)
    parser.add_argument("--analysis-timeout-sec", type=float, default=300.0)
    args = parser.parse_args()
    output_path = Path(args.output).resolve()
    results: list[dict[str, Any]] = []
    current_case_id = ""
    fixture_set_id = ""
    manifest_path: Path | None = None
    try:
        wait_agent(args.agent_http, 30.0)
        manifest_path = Path(args.fixture_manifest).resolve() if args.fixture_manifest else public_manifest_default()
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        fixture_set_id = str(manifest.get("set_id") or "")
        assert_public_manifest(manifest_path, manifest)
        available = {text(row, "case_id"): row for row in rows(manifest.get("cases")) if text(row, "case_id") in CASE_INPUTS}
        selected = args.cases or list(CASE_INPUTS)
        unknown = [case_id for case_id in selected if case_id not in available]
        if unknown:
            raise ValueError(f"unknown case IDs: {unknown}")
        run_dir = output_path.parent / (output_path.stem + "_artifacts")
        for case_id in selected:
            current_case_id = case_id
            result = run_case(args.agent_http, available[case_id], run_dir, args.timeout_sec, args.analysis_timeout_sec)
            results.append(result)
            print(json.dumps({"case_id": case_id, "processors": result["processor_sequence"], "final": result["final_status"]}, ensure_ascii=False), flush=True)
        report = {
            "schema_version": SCHEMA_VERSION,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "status": "completed",
            "fixture_set_id": manifest.get("set_id"),
            "fixture_manifest": str(manifest_path),
            "sealed_truth_opened": False,
            "case_count": len(results),
            "cases": results,
        }
        write_json(output_path, report)
        print(json.dumps({"status": "completed", "report": str(output_path), "case_count": len(results)}, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001
        failed = {
            "schema_version": SCHEMA_VERSION,
            "status": "infrastructure_failed",
            "error": str(exc),
            "failed_case_id": current_case_id,
            "fixture_set_id": fixture_set_id,
            "fixture_manifest": str(manifest_path) if manifest_path else "",
            "sealed_truth_opened": False,
            "completed_case_count": len(results),
            "cases": results,
        }
        write_json(output_path, failed)
        print(json.dumps(failed, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
