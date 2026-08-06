#!/usr/bin/env python3
"""Validate COM-7 confirmation and semantic compressor execution through Vit."""
from __future__ import annotations

import argparse
import json
import math
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix
import compressor_semantic_planning_smoke as planning


def semantic_chat(base: str, conversation_id: str, message: str, track_id: str,
                  plugin_id: str, plugin_name: str, clip_id: str, sample_count: int,
                  feature_snapshot: dict[str, Any], force_source_only_fallback: bool,
                  timeout: float) -> dict[str, Any]:
    waveform = feature_snapshot.get("waveform_envelope") or {}
    sample_rate = float(waveform.get("sample_rate") or 48_000)
    duration_seconds = sample_count / sample_rate
    range_start = 200.0 if force_source_only_fallback else 0.0
    range_end = 210.0 if force_source_only_fallback else duration_seconds
    context = {
        "agent_mode": "chat",
        "interaction_path": "agent_http_after_godot_project_lifecycle",
        "product_path_smoke": True,
        "product_lifecycle": "godot_project",
        "selected_plugin_track_id": track_id,
        "selected_track_id": track_id,
        "selected_plugin_id": plugin_id,
        "plugin_id": plugin_id,
        "selected_plugin_name": plugin_name,
        "selected_clip_id": clip_id,
        "clip_id": clip_id,
        "feature_snapshot": feature_snapshot,
        "selected_clip_time_range": {
            "active": True,
            "start_seconds": range_start,
            "end_seconds": range_end,
            "source": "selected_clip",
            "clip_id": clip_id,
        },
    }
    return matrix.request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id, "message": message, "context": context,
    }, timeout)


def approve(base: str, interaction_id: str, timeout: float) -> dict[str, Any]:
    return matrix.request_json("POST", base.rstrip("/") + "/agent/interaction/respond", {
        "interaction_id": interaction_id, "decision": "approve", "action_id": "approve", "payload": {},
    }, timeout)


def same_params(before: dict[str, float], after: dict[str, float]) -> bool:
    return set(before) == set(after) and all(
        math.isclose(before[key], after[key], rel_tol=0, abs_tol=1e-12) for key in before)


def interaction_id(response: dict[str, Any]) -> str:
    for row in response.get("interaction_requests") or []:
        if isinstance(row, dict) and matrix.first_text(row, "workflow") == "semantic_compressor_execution":
            return matrix.first_text(row, "id")
    return ""


def visible_execution_leaks(response: dict[str, Any]) -> list[str]:
    text = json.dumps(response, ensure_ascii=False).lower()
    return [token for token in ("control_ref", "c2cr1_", "param_id", "current_normalized") if token in text]


def run(args: argparse.Namespace) -> dict[str, Any]:
    config = json.loads(Path(args.config).read_text(encoding="utf-8"))
    case = next(row for row in config["cases"] if row.get("id") == args.case_id)
    track_id = ""
    report: dict[str, Any] = {}
    try:
        track = matrix.require_ok(matrix.invoke(args.agent_http, "track.add_audio", {
            "name": "COM-7 semantic execution smoke",
        }, args.timeout_sec, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        identifier, _ = matrix.resolve_identifier(case, args.timeout_sec)
        plugin_name = matrix.first_text(case, "plugin_name")
        loaded = matrix.require_ok(matrix.invoke(args.agent_http, "plugin.load_to_rack", {
            "track_id": track_id, "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": plugin_name, "plugin_identifier": identifier,
        }, args.timeout_sec, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        material = Path(args.material).resolve()
        imported = matrix.require_ok(matrix.invoke(args.agent_http, "clip.import_media_to_track", {
            "track_id": track_id, "file_path": str(material), "start_time": 0,
            "time_unit": "seconds", "media_type": "audio", "mode": "non_destructive",
        }, args.timeout_sec, True), "clip.import_media_to_track")
        clip_id = matrix.first_text(imported, "clip_id", "id")
        feature_snapshot, sample_count = planning.source_snapshot(material, track_id, clip_id)
        time.sleep(.35)
        before = planning.params(args.agent_http, track_id, plugin_id, args.timeout_sec)
        conversation_id = args.conversation_id or f"com7_semantic_{int(time.time())}"
        proposed = semantic_chat(args.agent_http, conversation_id, args.message, track_id, plugin_id,
                                 plugin_name, clip_id, sample_count, feature_snapshot,
                                 args.force_source_only_fallback, args.timeout_sec)
        after_proposal = planning.params(args.agent_http, track_id, plugin_id, args.timeout_sec)
        proposal_workflow = proposed.get("workflow_data") or {}
        iid = interaction_id(proposed)
        gates: list[dict[str, Any]] = [
            {"gate": "workflow", "pass": proposed.get("workflow") == "semantic_compressor_execution"},
            {"gate": "proposal_zero_write", "pass": same_params(before, after_proposal)},
            {"gate": "visible_materialization_hidden", "pass": not visible_execution_leaks(proposed)},
            {"gate": "product_range_context_without_samples", "pass": sample_count > 0},
        ]
        execution: dict[str, Any] | None = None
        after_execution = after_proposal
        if args.force_source_only_fallback:
            gates.extend([
                {"gate": "source_only_execution_rejected", "pass": proposal_workflow.get("status") == "materialization_rejected"},
                {"gate": "source_only_no_confirmation", "pass": proposed.get("needs_confirmation") is not True and not iid},
                {"gate": "source_only_no_mutation_authority", "pass": proposal_workflow.get("mutation_authorized") is False},
            ])
        else:
            gates.extend([
                {"gate": "waiting_confirmation", "pass": proposed.get("needs_confirmation") is True
                 and proposal_workflow.get("status") == "waiting_confirmation" and bool(iid)},
                {"gate": "paired_observation_ready", "pass": (proposal_workflow.get("com_observation") or {}).get("mode") == "paired_io"
                 and (proposal_workflow.get("com_observation") or {}).get("status") == "ready"},
                {"gate": "pending_no_mutation_authority", "pass": proposal_workflow.get("mutation_authorized") is False
                 and proposal_workflow.get("mutation_performed") is False},
            ])
            if iid:
                execution = approve(args.agent_http, iid, args.timeout_sec)
                after_execution = planning.params(args.agent_http, track_id, plugin_id, args.timeout_sec)
            execution_workflow = (execution or {}).get("workflow_data") or {}
            receipt = execution_workflow.get("execution_receipt") or {}
            audit = receipt.get("parameter_audit") or {}
            com_evaluation = receipt.get("com_evaluation") or {}
            gates.extend([
                {"gate": "execution_completed", "pass": (execution or {}).get("workflow") == "semantic_compressor_execution"
                 and execution_workflow.get("status") == "executed"},
                {"gate": "confirmed_mutation", "pass": execution_workflow.get("mutation_authorized") is True
                 and execution_workflow.get("mutation_performed") is True},
                {"gate": "parameters_changed", "pass": set(before) == set(after_execution)
                 and not same_params(before, after_execution)},
                {"gate": "full_parameter_audit", "pass": audit.get("status") == "pass"
                 and audit.get("non_target_parameters_unchanged") is True},
                {"gate": "controller_atomic", "pass": (receipt.get("controller_result") or {}).get("atomic") is True},
                {"gate": "change_delta", "pass": com_evaluation.get("mode") == "change_delta"
                 and com_evaluation.get("status") in {"ready", "partial"}},
                {"gate": "no_auto_iteration", "pass": ((proposal_workflow.get("compressor_plan") or {})
                 .get("evaluation_contract") or {}).get("no_auto_iteration") is True},
            ])
        failed = [row["gate"] for row in gates if not row["pass"]]
        report = {
            "schema_version": "semantic_compressor.execution_product_path_smoke.v1",
            "status": "ok" if not failed else "failed",
            "force_source_only_fallback": args.force_source_only_fallback,
            "conversation_id": conversation_id, "plugin_name": plugin_name,
            "track_id": track_id, "plugin_id": plugin_id, "clip_id": clip_id,
            "sample_count": sample_count, "proposal": proposed, "execution": execution,
            "parameter_count": len(before), "changed_parameter_count": sum(
                1 for key in before if key in after_execution and not math.isclose(
                    before[key], after_execution[key], rel_tol=0, abs_tol=1e-12)),
            "gates": gates, "failed_gates": failed,
        }
        return report
    finally:
        if track_id:
            matrix.require_ok(matrix.invoke(args.agent_http, "track.delete", {
                "track_id": track_id,
            }, args.timeout_sec, True), "track.delete")
            report["temporary_track_deleted"] = True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default=str(Path(__file__).with_name("compressor_compat_matrix.json")))
    parser.add_argument("--case-id", default="pro_c_2")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--message", default="压得更稳一点，但尽量保留瞬态，不要因为响度变大产生错觉。")
    parser.add_argument("--material", default=str(Path(__file__).parents[1] / "PluginProbe" / "assets" /
                                                   "probe_audio_v1" / "transient_burst_48k_10s.wav"))
    parser.add_argument("--conversation-id", default="")
    parser.add_argument("--force-source-only-fallback", action="store_true")
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-sec", type=float, default=300.0)
    args = parser.parse_args()
    report = run(args)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report.get("status") == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
