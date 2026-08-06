#!/usr/bin/env python3
"""Validate compressor handoffs from Vit's open semantic treatment route."""
from __future__ import annotations

import argparse
import json
import math
import time
from pathlib import Path
from typing import Any, Callable

import compressor_compat_matrix_smoke as matrix
import compressor_semantic_planning_smoke as planning


MESSAGE = "Use compression to make this vocal more stable and forward while preserving transients."
PARAMETER_MUTATIONS = {
    "set_plugin_param",
    "plugin.set_params_batch",
    "plugin_grabber_apply_compressor_controls",
    "plugin_grabber.apply_compressor_controls",
}


def rows(value: Any) -> list[dict[str, Any]]:
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def workflow_data(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def interaction(response: dict[str, Any], workflow: str) -> dict[str, Any]:
    for item in rows(response.get("interaction_requests")):
        if matrix.first_text(item, "workflow") == workflow:
            return item
    raise AssertionError(f"response omitted {workflow} interaction")


def respond(base: str, request: dict[str, Any], action: dict[str, Any], timeout: float) -> dict[str, Any]:
    action_id = matrix.first_text(action, "id")
    return matrix.request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": matrix.first_text(request, "id"),
        "decision": action_id,
        "action_id": action_id,
        "payload": request.get("payload") if isinstance(request.get("payload"), dict) else {},
    }, timeout)


def select_action(request: dict[str, Any], predicate: Callable[[dict[str, Any]], bool]) -> dict[str, Any]:
    actions = [item for item in rows(request.get("actions"))
               if matrix.first_text(item, "id").startswith("select_")]
    for action in actions:
        if predicate(action):
            return action
    raise AssertionError(f"no matching selection action: {actions}")


def mutation_names(response: dict[str, Any]) -> set[str]:
    found: set[str] = set()
    for reply in rows(response.get("executed_kernel_reply")):
        for key in ("tool", "command_name"):
            value = matrix.first_text(reply, key).lower()
            if value:
                found.add(value)
        result = reply.get("result")
        if isinstance(result, dict):
            text = json.dumps(result, ensure_ascii=False).lower()
            for name in PARAMETER_MUTATIONS | {"rack_add_node"}:
                if name in text:
                    found.add(name)
    return found


def same_params(before: dict[str, float], after: dict[str, float]) -> bool:
    return set(before) == set(after) and all(
        math.isclose(before[key], after[key], rel_tol=0, abs_tol=1e-12) for key in before)


def feature_snapshot(material: Path, track_id: str, clip_id: str) -> tuple[dict[str, Any], int]:
    snapshot, sample_count = planning.source_snapshot(material, track_id, clip_id)
    waveform = snapshot.get("waveform_envelope")
    if isinstance(waveform, dict):
        fingerprint = matrix.first_text(waveform, "source_revision").removeprefix("sha256:")
        waveform["evidence_ref"] = "compressor_open_routing_smoke:source:" + fingerprint
        waveform["analyzer_version"] = "compressor_open_routing_smoke.source_snapshot.v1"
    return snapshot, sample_count


def create_audio_fixture(base: str, name: str, material: Path, timeout: float) -> dict[str, Any]:
    track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {"name": name}, timeout, True), "track.add_audio")
    track_id = matrix.first_text(track, "track_id", "id")
    imported = matrix.require_ok(matrix.invoke(base, "clip.import_media_to_track", {
        "track_id": track_id,
        "file_path": str(material),
        "start_time": 0,
        "time_unit": "seconds",
        "media_type": "audio",
        "mode": "non_destructive",
    }, timeout, True), "clip.import_media_to_track")
    clip_id = matrix.first_text(imported, "clip_id", "id")
    snapshot, sample_count = feature_snapshot(material, track_id, clip_id)
    if not track_id or not clip_id or sample_count <= 0:
        raise RuntimeError("audio fixture omitted track, clip, or sample identity")
    return {
        "track_id": track_id,
        "track_name": name,
        "clip_id": clip_id,
        "feature_snapshot": snapshot,
        "sample_count": sample_count,
    }


def chat_context(fixture: dict[str, Any]) -> dict[str, Any]:
    return {
        "agent_mode": "chat",
        "interaction_path": "agent_http_after_godot_project_lifecycle",
        "product_path_smoke": True,
        "product_lifecycle": "godot_project",
        "selected_track_id": fixture["track_id"],
        "selected_track_name": fixture["track_name"],
        "selected_clip_id": fixture["clip_id"],
        "clip_id": fixture["clip_id"],
        "feature_snapshot": fixture["feature_snapshot"],
        "start_sample": 0,
        "end_sample": fixture["sample_count"],
    }


def open_chat(base: str, conversation_id: str, fixture: dict[str, Any], timeout: float) -> dict[str, Any]:
    context = chat_context(fixture)
    if "selected_plugin_id" in context or "selected_plugin_track_id" in context:
        raise AssertionError("open-route smoke must not bind a selected plugin")
    return matrix.request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id,
        "message": MESSAGE,
        "context": context,
    }, timeout)


def choose_treatment(base: str, response: dict[str, Any], target_mode: str,
                     timeout: float) -> dict[str, Any]:
    if response.get("workflow") != "semantic_treatment_strategy":
        return response
    data = workflow_data(response)
    request = interaction(response, "semantic_treatment_strategy")
    choices = {matrix.first_text(item, "choice_key"): item for item in rows(data.get("choices"))}

    def matches(action: dict[str, Any]) -> bool:
        choice = choices.get(matrix.first_text(action, "id").removeprefix("select_"), {})
        return (matrix.first_text(choice, "processor_type") == "compressor"
                and matrix.first_text(choice, "target_mode") == target_mode)

    return respond(base, request, select_action(request, matches), timeout)


def assert_compressor_proposal(response: dict[str, Any], post_load: bool) -> dict[str, Any]:
    data = workflow_data(response)
    if response.get("workflow") != "semantic_compressor_execution":
        raise AssertionError(f"open route did not reach compressor execution proposal: {response}")
    if response.get("needs_confirmation") is not True or data.get("status") != "waiting_confirmation":
        raise AssertionError(f"compressor proposal is not waiting for independent approval: {data}")
    if data.get("mutation_authorized") is not False or data.get("mutation_performed") is not False:
        raise AssertionError(f"compressor proposal leaked mutation authority: {data}")
    card = data.get("processor_identity_card") if isinstance(data.get("processor_identity_card"), dict) else {}
    if card.get("schema_version") != "audio_processor.semantic_identity_card.v1":
        raise AssertionError(f"compressor proposal omitted the identity card: {card}")
    qualification = data.get("post_load_qualification") if isinstance(data.get("post_load_qualification"), dict) else {}
    if post_load and (qualification.get("status") != "qualified"
                      or qualification.get("processor_type") != "compressor"):
        raise AssertionError(f"loaded compressor was not re-qualified: {qualification}")
    return data


def execute_proposal(base: str, proposal: dict[str, Any], before: dict[str, float],
                     track_id: str, plugin_id: str, timeout: float) -> tuple[dict[str, Any], dict[str, float]]:
    request = interaction(proposal, "semantic_compressor_execution")
    approve = next((item for item in rows(request.get("actions"))
                    if matrix.first_text(item, "id") == "approve"), None)
    if approve is None:
        raise AssertionError("compressor proposal omitted approve action")
    executed = respond(base, request, approve, timeout)
    data = workflow_data(executed)
    after = planning.params(base, track_id, plugin_id, timeout)
    receipt = data.get("execution_receipt") if isinstance(data.get("execution_receipt"), dict) else {}
    audit = receipt.get("parameter_audit") if isinstance(receipt.get("parameter_audit"), dict) else {}
    if executed.get("workflow") != "semantic_compressor_execution" or data.get("status") != "executed":
        raise AssertionError(f"compressor execution did not complete: {executed}")
    if data.get("mutation_authorized") is not True or data.get("mutation_performed") is not True:
        raise AssertionError(f"approved compressor execution omitted mutation authority: {data}")
    if same_params(before, after):
        raise AssertionError("approved compressor execution did not change any parameter")
    if audit.get("status") != "pass" or audit.get("non_target_parameters_unchanged") is not True:
        raise AssertionError(f"compressor execution did not pass parameter audit: {audit}")
    return executed, after


def resolve_case(config_path: Path, case_id: str) -> dict[str, Any]:
    config = json.loads(config_path.read_text(encoding="utf-8"))
    return next(item for item in config.get("cases", []) if item.get("id") == case_id)


def load_fixture_compressor(base: str, fixture: dict[str, Any], case: dict[str, Any],
                            timeout: float) -> str:
    identifier, _ = matrix.resolve_identifier(case, timeout)
    loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
        "track_id": fixture["track_id"],
        "plugin_path": matrix.first_text(case, "plugin_path"),
        "plugin_name": matrix.first_text(case, "plugin_name"),
        "plugin_identifier": identifier,
    }, timeout, True), "plugin.load_to_rack")
    plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
    if not plugin_id:
        raise RuntimeError("plugin load omitted plugin identity")
    return plugin_id


def existing_instance_case(base: str, material: Path, case: dict[str, Any], timeout: float) -> dict[str, Any]:
    fixture = create_audio_fixture(base, "Open routing existing compressor", material, timeout)
    try:
        plugin_id = load_fixture_compressor(base, fixture, case, timeout)
        time.sleep(0.35)
        before = planning.params(base, fixture["track_id"], plugin_id, timeout)
        conversation_id = f"compressor_open_existing_{int(time.time() * 1000)}"
        first = open_chat(base, conversation_id, fixture, timeout)
        proposal = choose_treatment(base, first, "existing_plugin", timeout)
        assert_compressor_proposal(proposal, False)
        after_proposal = planning.params(base, fixture["track_id"], plugin_id, timeout)
        if not same_params(before, after_proposal):
            raise AssertionError("existing-instance open routing changed parameters before approval")
        executed, after = execute_proposal(base, proposal, before, fixture["track_id"], plugin_id, timeout)
        return {
            "status": "ok",
            "conversation_id": conversation_id,
            "track_id": fixture["track_id"],
            "plugin_id": plugin_id,
            "strategy_interaction_required": first.get("workflow") == "semantic_treatment_strategy",
            "proposal_zero_write": True,
            "execution_status": workflow_data(executed).get("status"),
            "changed_parameter_count": sum(1 for key in before if key in after and not math.isclose(
                before[key], after[key], rel_tol=0, abs_tol=1e-12)),
        }
    finally:
        matrix.require_ok(matrix.invoke(base, "track.delete", {
            "track_id": fixture["track_id"],
        }, timeout, True), "track.delete")


def recommended_candidate_action(request: dict[str, Any]) -> dict[str, Any]:
    preferred = ("pro-c 2", "pro c 2", "zl compressor", "rcompressor", "cla-2a", "api-2500")
    actions = [item for item in rows(request.get("actions"))
               if matrix.first_text(item, "id").startswith("select_")]
    for token in preferred:
        for action in actions:
            if token in json.dumps(action, ensure_ascii=False).lower():
                return action
    if actions:
        return actions[0]
    raise AssertionError("compressor recommendation omitted selection actions")


def post_load_case(base: str, material: Path, timeout: float) -> dict[str, Any]:
    fixture = create_audio_fixture(base, "Open routing compressor post-load", material, timeout)
    try:
        conversation_id = f"compressor_open_post_load_{int(time.time() * 1000)}"
        first = open_chat(base, conversation_id, fixture, timeout)
        recommendation = choose_treatment(base, first, "load_required", timeout)
        rec_data = workflow_data(recommendation)
        if recommendation.get("workflow") != "plugin_recommendation_selection":
            raise AssertionError(f"open route did not enter compressor recommendation: {recommendation}")
        if rec_data.get("processor_type") != "compressor" or rec_data.get("post_load_planner") != "semantic_compressor":
            raise AssertionError(f"compressor recommendation lost its post-load handoff: {rec_data}")
        rec_request = interaction(recommendation, "plugin_recommendation_selection")
        load_proposal = respond(base, rec_request, recommended_candidate_action(rec_request), timeout)
        if load_proposal.get("workflow") != "plugin_grabber_load_and_get_params" or load_proposal.get("needs_confirmation") is not True:
            raise AssertionError(f"compressor selection did not create load confirmation: {load_proposal}")
        loaded = matrix.request_json("POST", base.rstrip("/") + "/agent/confirm", {
            "plan_id": load_proposal.get("plan_id"), "decision": "approve",
        }, timeout)
        proposal_data = assert_compressor_proposal(loaded, True)
        mutations = mutation_names(loaded)
        if "rack_add_node" not in mutations or PARAMETER_MUTATIONS.intersection(mutations):
            raise AssertionError(f"load approval did not remain separate from parameter writes: {mutations}")
        qualification = proposal_data["post_load_qualification"]
        plugin_id = matrix.first_text(qualification, "plugin_id")
        before = planning.params(base, fixture["track_id"], plugin_id, timeout)
        executed, after = execute_proposal(base, loaded, before, fixture["track_id"], plugin_id, timeout)
        return {
            "status": "ok",
            "conversation_id": conversation_id,
            "track_id": fixture["track_id"],
            "plugin_id": plugin_id,
            "plugin_name": matrix.first_text(qualification, "plugin_name"),
            "strategy_interaction_required": first.get("workflow") == "semantic_treatment_strategy",
            "recommendation_post_load_planner": rec_data.get("post_load_planner"),
            "load_and_parameter_approvals_separate": True,
            "post_load_qualification": qualification,
            "execution_status": workflow_data(executed).get("status"),
            "changed_parameter_count": sum(1 for key in before if key in after and not math.isclose(
                before[key], after[key], rel_tol=0, abs_tol=1e-12)),
        }
    finally:
        matrix.require_ok(matrix.invoke(base, "track.delete", {
            "track_id": fixture["track_id"],
        }, timeout, True), "track.delete")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--config", default=str(Path(__file__).with_name("compressor_compat_matrix.json")))
    parser.add_argument("--case-id", default="pro_c_2")
    parser.add_argument("--material", default=str(Path(__file__).parents[1] / "PluginProbe" / "assets" /
                                                   "probe_audio_v1" / "transient_burst_48k_10s.wav"))
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-sec", type=float, default=360.0)
    args = parser.parse_args()

    material = Path(args.material).resolve()
    case = resolve_case(Path(args.config), args.case_id)
    report: dict[str, Any] = {
        "schema_version": "semantic_compressor.open_routing_product_smoke.v1",
        "message": MESSAGE,
        "selected_plugin_disclosed_to_agent": False,
    }
    try:
        report["existing_instance"] = existing_instance_case(args.agent_http, material, case, args.timeout_sec)
        report["post_load"] = post_load_case(args.agent_http, material, args.timeout_sec)
        report["status"] = "ok"
    except Exception as exc:
        report["status"] = "failed"
        report["error"] = str(exc)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report.get("status") == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
