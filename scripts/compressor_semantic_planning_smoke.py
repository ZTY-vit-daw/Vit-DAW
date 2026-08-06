#!/usr/bin/env python3
"""Validate COM-6 planning-only semantics through the real Vit chat path."""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import time
import urllib.parse
import wave
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


def params(base: str, track_id: str, plugin_id: str, timeout: float) -> dict[str, float]:
    result = matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
        "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True,
    }, timeout, False), "plugin.parameters")
    rows = result.get("parameters") or []
    return {matrix.first_text(row, "id", "param_id"): float(row["normalized_value"])
            for row in rows if isinstance(row, dict) and matrix.first_text(row, "id", "param_id")
            and isinstance(row.get("normalized_value"), (int, float))}


def chat(base: str, conversation_id: str, message: str, track_id: str,
         plugin_id: str, plugin_name: str, clip_id: str, sample_count: int,
         feature_snapshot: dict[str, Any], force_source_only_fallback: bool,
         timeout: float) -> dict[str, Any]:
    context = {
        "agent_mode": "chat",
        "interaction_path": "agent_http_after_godot_project_lifecycle",
        "product_path_smoke": True,
        "product_lifecycle": "godot_project",
        "semantic_compressor_planning_only": True,
        "selected_plugin_track_id": track_id,
        "selected_track_id": track_id,
        "selected_plugin_id": plugin_id,
        "plugin_id": plugin_id,
        "selected_plugin_name": plugin_name,
        "selected_clip_id": clip_id,
        "clip_id": clip_id,
        "feature_snapshot": feature_snapshot,
    }
    if not force_source_only_fallback:
        context["start_sample"] = 0
        context["end_sample"] = sample_count
    return matrix.request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id,
        "message": message,
        "context": context,
    }, timeout)


def events(base: str, conversation_id: str, timeout: float) -> list[dict[str, Any]]:
    query = urllib.parse.urlencode({"conversation_id": conversation_id, "since": 0, "limit": 200})
    result = matrix.request_json("GET", base.rstrip("/") + "/agent/events?" + query, None, timeout)
    return [row for row in result.get("events") or [] if isinstance(row, dict)]


def validate(response: dict[str, Any], before: dict[str, float], after: dict[str, float],
             event_rows: list[dict[str, Any]], force_source_only_fallback: bool) -> list[dict[str, Any]]:
    gates: list[dict[str, Any]] = []
    workflow = response.get("workflow_data") or {}
    card = workflow.get("processor_identity_card") or {}
    intent = workflow.get("intent_plan") or {}
    observation = workflow.get("com_observation") or {}
    brief = workflow.get("control_brief") or {}
    plan = workflow.get("compressor_plan") or {}
    controls = plan.get("controls") or []
    card_text, brief_text = json.dumps(card, ensure_ascii=False), json.dumps(brief, ensure_ascii=False)
    event_tools = [matrix.first_text(row.get("payload") or {}, "tool") for row in event_rows
                   if str(row.get("type", "")).startswith("item.")]
    gates.extend([
        {"gate":"workflow", "pass": response.get("workflow") == "semantic_compressor_planning"},
        {"gate":"status", "pass": workflow.get("status") == "planned"},
        {"gate":"planning_only", "pass": workflow.get("planning_only") is True and plan.get("planning_only") is True},
        {"gate":"no_mutation_authority", "pass": workflow.get("mutation_authorized") is False and plan.get("mutation_authorized") is False},
        {"gate":"no_mutation_performed", "pass": workflow.get("mutation_performed") is False},
        {"gate":"identity_card_schema", "pass": card.get("schema_version") == "audio_processor.semantic_identity_card.v1"},
        {"gate":"identity_card_budget", "pass": len(card_text.encode("utf-8")) <= 2048},
        {"gate":"identity_card_no_execution_detail", "pass": not any(k in card_text for k in ("control_ref", "param_id", "normalized", "curve"))},
        {"gate":"intent_schema", "pass": intent.get("schema_version") == "semantic_effect.compressor_intent_plan.v1"},
        {"gate":"com_context_present", "pass": bool(observation.get("mode") or observation.get("schema_version"))},
        {"gate":"com_evidence_ready", "pass": observation.get("mode") in {"paired_io", "source_only"} and observation.get("status") in {"ready", "partial"}},
        {"gate":"brief_selected_axes", "pass": brief.get("selected_axes") == intent.get("selected_axes")},
        {"gate":"brief_no_execution_detail", "pass": not any(k in brief_text for k in ("control_ref", "param_id", "current_normalized"))},
        {"gate":"plan_schema", "pass": plan.get("schema_version") == "semantic_effect.compressor_plan.v1"},
        {"gate":"plan_has_absolute_controls", "pass": bool(controls) and all(isinstance(c.get("target"), dict) and len(c["target"]) == 1 for c in controls if isinstance(c, dict))},
        {"gate":"evaluation_frozen", "pass": (plan.get("evaluation_contract") or {}).get("no_auto_iteration") is True},
        {"gate":"no_tool_execution_events", "pass": not [x for x in event_tools if x]},
        {"gate":"parameter_count_unchanged", "pass": set(before) == set(after)},
        {"gate":"parameters_unchanged", "pass": set(before) == set(after) and all(math.isclose(before[k], after[k], rel_tol=0, abs_tol=1e-12) for k in before)},
    ])
    if force_source_only_fallback:
        gates.append({"gate":"source_only_fallback", "pass": observation.get("requested_mode") == "paired_io"
                      and observation.get("mode") == "source_only" and bool(observation.get("fallback_reason"))})
    return gates


def source_snapshot(material: Path, track_id: str, clip_id: str) -> tuple[dict[str, Any], int]:
    with wave.open(str(material), "rb") as reader:
        channels, width, rate, frames = reader.getnchannels(), reader.getsampwidth(), reader.getframerate(), reader.getnframes()
        raw = reader.readframes(frames)
    if width != 3 or channels <= 0 or rate <= 0 or frames <= 0:
        raise RuntimeError(f"unsupported smoke WAV layout channels={channels} width={width} rate={rate} frames={frames}")
    scale = float(1 << 23)
    block_frames = rate
    total_sq = 0.0
    total_count = 0
    total_peak = 0.0
    nonzero = 0
    block_sq = block_count = 0
    block_peak = 0.0
    segments: list[dict[str, Any]] = []
    for frame in range(frames):
        for channel in range(channels):
            offset = (frame * channels + channel) * width
            value = int.from_bytes(raw[offset:offset + 3], "little", signed=True) / scale
            magnitude = abs(value)
            total_sq += value * value
            total_count += 1
            total_peak = max(total_peak, magnitude)
            nonzero += int(value != 0)
            block_sq += value * value
            block_count += 1
            block_peak = max(block_peak, magnitude)
        if (frame + 1) % block_frames == 0 or frame + 1 == frames:
            rms = math.sqrt(block_sq / max(1, block_count))
            rms_db = 20 * math.log10(max(rms, 1e-12))
            peak_db = 20 * math.log10(max(block_peak, 1e-12))
            start = frame + 1 - min(block_frames, frame + 1)
            segments.append({"start_seconds": start / rate, "end_seconds": (frame + 1) / rate,
                             "rms_dbfs": rms_db, "peak_dbfs": peak_db, "crest_db": peak_db-rms_db,
                             "energy_state": "active" if rms_db > -80 else "silent"})
            block_sq = block_count = 0
            block_peak = 0.0
    rms = math.sqrt(total_sq / max(1, total_count))
    rms_db = 20 * math.log10(max(rms, 1e-12))
    peak_db = 20 * math.log10(max(total_peak, 1e-12))
    fingerprint = hashlib.sha256(raw).hexdigest()[:20]
    waveform = {"schema_version":"dad.com.source_projection_input.v1", "feature_type":"waveform_envelope",
                "status":"ready", "freshness":"fresh", "track_id":track_id, "clip_id":clip_id,
                "source_revision":"sha256:"+fingerprint, "clip_revision":"import:"+clip_id,
                "sample_rate":rate, "channel_count":channels, "channel_layout":"stereo" if channels == 2 else "mono",
                "duration_seconds":frames/rate, "analyzed_sample_count":frames, "nonzero_count":nonzero,
                "coverage_ratio":1.0, "rms_dbfs":rms_db, "peak_dbfs":peak_db, "crest_db":peak_db-rms_db,
                "quality_status":"ready", "evidence_ref":"com6_smoke:source:"+fingerprint,
                "analyzer_version":"com6.source_snapshot.v1", "time_segments":segments}
    return {"schema_version":"mixboard_feature_snapshot.v1", "waveform_envelope":waveform}, frames


def run(args: argparse.Namespace) -> dict[str, Any]:
    config = json.loads(Path(args.config).read_text(encoding="utf-8"))
    case = next(row for row in config["cases"] if row.get("id") == args.case_id)
    track_id = ""
    report: dict[str, Any] = {}
    try:
        track = matrix.require_ok(matrix.invoke(args.agent_http, "track.add_audio", {"name":"COM-6 semantic planning smoke"}, args.timeout_sec, True), "track.add_audio")
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
        feature_snapshot, sample_count = source_snapshot(material, track_id, clip_id)
        if not clip_id or sample_count <= 0:
            raise RuntimeError("material import omitted clip identity or sample window")
        time.sleep(.35)
        before = params(args.agent_http, track_id, plugin_id, args.timeout_sec)
        conversation_id = args.conversation_id or f"com6_semantic_{int(time.time())}"
        response = chat(args.agent_http, conversation_id, args.message, track_id, plugin_id, plugin_name,
                        clip_id, sample_count, feature_snapshot, args.force_source_only_fallback,
                        args.timeout_sec)
        after = params(args.agent_http, track_id, plugin_id, args.timeout_sec)
        event_rows = events(args.agent_http, conversation_id, args.timeout_sec)
        gates = validate(response, before, after, event_rows, args.force_source_only_fallback)
        failed = [g["gate"] for g in gates if not g["pass"]]
        report = {"schema_version":"semantic_compressor.product_path_smoke.v1", "status":"ok" if not failed else "failed",
                  "conversation_id":conversation_id, "message":args.message, "plugin_name":plugin_name,
                  "track_id":track_id, "plugin_id":plugin_id, "clip_id":clip_id, "sample_count":sample_count,
                  "force_source_only_fallback":args.force_source_only_fallback,
                  "response":response, "gates":gates, "failed_gates":failed}
        return report
    finally:
        if track_id:
            matrix.require_ok(matrix.invoke(args.agent_http, "track.delete", {"track_id":track_id}, args.timeout_sec, True), "track.delete")
            report["temporary_track_deleted"] = True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default=str(Path(__file__).with_name("compressor_compat_matrix.json")))
    parser.add_argument("--case-id", default="pro_c_2")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--message", default="压得更稳一点，但尽量保留瞬态，不要因为响度变大产生错觉。")
    parser.add_argument("--material", default=str(Path(__file__).parents[1] / "PluginProbe" / "assets" / "probe_audio_v1" / "transient_burst_48k_10s.wav"))
    parser.add_argument("--conversation-id", default="")
    parser.add_argument("--force-source-only-fallback", action="store_true")
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-sec", type=float, default=300.0)
    args = parser.parse_args()
    report: dict[str, Any] = {}
    report = run(args)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if report.get("status") == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
