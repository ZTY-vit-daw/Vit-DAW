from __future__ import annotations

import argparse
import json
import math
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


FORBIDDEN_RAW_KEYS = {
    "aligned_envelope_frames", "input_event_candidates", "raw_samples",
    "raw_waveform", "render_file_path", "shared_memory", "pcm",
}


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float) -> dict[str, Any]:
    payload = json.dumps({"tool": tool, "args": args, "confirmed": False,
                          "source": "com5_product_path_smoke"}).encode("utf-8")
    request = urllib.request.Request(base.rstrip("/") + "/agent/invoke", data=payload,
                                     headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {error.code}: {body[:2000]}") from error
    parsed = json.loads(body)
    if str(parsed.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {parsed}")
    result = parsed.get("result", parsed)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned a non-object result")
    return result


def compact_source_snapshot(artifact: dict[str, Any], clip_id: str) -> dict[str, Any]:
    conditions = artifact["conditions"]
    frames = artifact["aligned_envelope_frames"]
    sample_rate = float(conditions["sample_rate"])
    block_count = min(12, len(frames))
    block_size = max(1, math.ceil(len(frames) / block_count))
    segments: list[dict[str, Any]] = []
    for index in range(0, len(frames), block_size):
        block = frames[index:index + block_size]
        if not block:
            continue
        rms_values = [value for row in block for value in row["input_rms_dbfs"]]
        peak_values = [max(row["input_peak_dbfs"]) for row in block]
        mean_power = sum(10 ** (value / 10) for value in rms_values) / len(rms_values)
        rms = 10 * math.log10(max(mean_power, 1e-16))
        peak = max(peak_values)
        segments.append({
            "start_seconds": block[0]["start_sample"] / sample_rate,
            "end_seconds": block[-1]["end_sample"] / sample_rate,
            "rms_dbfs": rms, "peak_dbfs": peak, "crest_db": peak-rms,
            "energy_state": "active" if rms > -80 else "silent",
        })
    quality = artifact["quality_evidence"]["input"]
    waveform = {
        "schema_version": "dad.com.source_projection_input.v1", "feature_type": "waveform_envelope",
        "status": "ready", "freshness": "fresh", "track_id": artifact["processor_scope"]["track_id"],
        "clip_id": clip_id,
        "source_revision": conditions["source_revision"], "clip_revision": conditions["clip_revision"],
        "sample_rate": sample_rate, "channel_count": conditions["channel_count"],
        "channel_layout": conditions["channel_layout"],
        "duration_seconds": (conditions["end_sample"]-conditions["start_sample"])/sample_rate,
        "analyzed_sample_count": conditions["end_sample"]-conditions["start_sample"],
        "nonzero_count": quality["nonzero_samples"], "coverage_ratio": quality["coverage"],
        "rms_dbfs": quality["rms_dbfs"], "peak_dbfs": quality["peak_dbfs"],
        "crest_db": quality["peak_dbfs"]-quality["rms_dbfs"], "quality_status": "ready",
        "evidence_ref": artifact["evidence_ref"] + ":input_source",
        "analyzer_version": artifact["analyzer_version"], "time_segments": segments,
    }
    return {"schema_version": "mixboard_feature_snapshot.v1", "waveform_envelope": waveform}


def projection(result: dict[str, Any]) -> dict[str, Any]:
    value = result.get("com_projection")
    if not isinstance(value, dict):
        observation = result.get("observation")
        value = observation.get("com_projection") if isinstance(observation, dict) else None
    if not isinstance(value, dict):
        raise RuntimeError("mix.observe omitted com_projection")
    return value


def catalog_entry(result: dict[str, Any]) -> dict[str, Any]:
    catalog = result.get("catalog") or (result.get("observation") or {}).get("catalog") or {}
    for row in catalog.get("entries") or []:
        if row.get("key") == "observation.com_projection":
            return row
    raise RuntimeError("catalog omitted observation.com_projection")


def raw_leaks(value: Any, path: str = "") -> list[str]:
    leaks: list[str] = []
    if isinstance(value, dict):
        for key, child in value.items():
            next_path = f"{path}.{key}" if path else key
            if key.lower() in FORBIDDEN_RAW_KEYS:
                leaks.append(next_path)
            leaks.extend(raw_leaks(child, next_path))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            leaks.extend(raw_leaks(child, f"{path}[{index}]"))
    return leaks


def check_projection(mode: str, result: dict[str, Any], allowed: set[str]) -> list[dict[str, Any]]:
    proj = projection(result)
    context = proj.get("llm_context") or {}
    entry = catalog_entry(result)
    return [
        {"gate": f"{mode}_projection_mode", "pass": proj.get("mode") == mode},
        {"gate": f"{mode}_projection_status", "pass": proj.get("status") in allowed},
        {"gate": f"{mode}_identity", "pass": bool(proj.get("projection_id") and proj.get("observation_id") == result.get("observation_id"))},
        {"gate": f"{mode}_catalog", "pass": entry.get("freshness") in {"fresh", "partial"}},
        {"gate": f"{mode}_compact_context", "pass": context.get("do_not_include_raw_package") is True and len(context.get("compact_facts") or []) <= 24},
        {"gate": f"{mode}_response_raw_isolation", "pass": not raw_leaks(result)},
    ]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--com4-report", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-sec", type=float, default=180)
    args = parser.parse_args()
    com4 = json.loads(Path(args.com4_report).read_text(encoding="utf-8"))
    if com4.get("status") != "ok":
        raise RuntimeError("COM-4 prerequisite smoke is not ok")
    before_path = Path(com4["before"]["artifact"]).resolve()
    after_path = Path(com4["after"]["artifact"]).resolve()
    before = json.loads(before_path.read_text(encoding="utf-8"))
    scope, conditions = before["processor_scope"], before["conditions"]
    common = {"scope": "selected_track", "project_context": True, "observation_only": True,
              "track_id": scope["track_id"], "plugin_id": scope["plugin_instance_id"]}

    clip_id = str(com4["before"]["receipt"]["clip_id"])
    source_result = invoke(args.agent_http, "mix.observe", {**common, "mix_session_id": "com5_source_only",
        "com_mode": "source_only", "clip_id": clip_id,
        "feature_snapshot": compact_source_snapshot(before, clip_id)}, args.timeout_sec)
    paired_result = invoke(args.agent_http, "mix.observe", {**common, "mix_session_id": "com5_paired_io",
        "com_mode": "paired_io", "clip_id": clip_id,
        "topology_class": scope["topology_class"], "topology_generation": scope["topology_generation"],
        "start_sample": conditions["start_sample"], "end_sample": conditions["end_sample"]}, args.timeout_sec)
    change_result = invoke(args.agent_http, "mix.observe", {**common, "mix_session_id": "com5_change_delta",
        "com_mode": "change_delta", "com_before_artifact_path": str(before_path),
        "com_after_artifact_path": str(after_path)}, args.timeout_sec)

    results = {"source_only": source_result, "paired_io": paired_result, "change_delta": change_result}
    reads: dict[str, Any] = {}
    gates: list[dict[str, Any]] = []
    for mode, result in results.items():
        gates.extend(check_projection(mode, result, {"ready", "partial"}))
        reads[mode] = invoke(args.agent_http, "mix.read", {
            "mix_session_id": result["mix_session_id"], "observation_id": result["observation_id"],
            "keys": ["observation.com_projection", "observation.catalog"]}, args.timeout_sec)
        read_proj = ((reads[mode].get("items") or {}).get("observation.com_projection") or {})
        gates.append({"gate": f"{mode}_persisted_read_identity", "pass": read_proj.get("projection_id") == projection(result).get("projection_id")})
        for key in ("observation_path", "context_pack_path"):
            path = Path(result[key])
            content = json.loads(path.read_text(encoding="utf-8")) if path.is_file() else {}
            gates.append({"gate": f"{mode}_{key}_exists", "pass": path.is_file()})
            gates.append({"gate": f"{mode}_{key}_raw_isolation", "pass": not raw_leaks(content)})
            gates.append({"gate": f"{mode}_{key}_artifact_path_isolation", "pass": str(before_path) not in json.dumps(content) and str(after_path) not in json.dumps(content)})

    paired_proj = projection(paired_result)
    capture = paired_result.get("com_evidence_capture") or {}
    gates.extend([
        {"gate": "paired_live_capture", "pass": capture.get("status") == "ready" and bool(capture.get("pair_id"))},
        {"gate": "paired_capture_identity", "pass": (paired_proj.get("evidence_inputs") or {}).get("source_evidence_id") == capture.get("pair_id")},
        {"gate": "change_input_equivalent", "pass": (projection(change_result).get("trust_quality") or {}).get("input_equivalent") is True},
        {"gate": "change_child_ids", "pass": bool((projection(change_result).get("behavior_change") or {}).get("before_projection_id") and (projection(change_result).get("behavior_change") or {}).get("after_projection_id"))},
        {"gate": "com4_restore_verified", "pass": next((row.get("pass") for row in com4.get("gates") or [] if row.get("gate") == "restore_verified"), False) is True},
    ])
    status = "ok" if all(row["pass"] for row in gates) else "failed"
    report = {"schema_version": "com.product_path_smoke.v1", "status": status,
              "godot_started_agent": True, "com4_prerequisite": args.com4_report,
              "projections": {mode: projection(result) for mode, result in results.items()},
              "captures": {"paired_io": capture}, "reads": reads, "gates": gates}
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"status": status, "report": str(output), "failed_gates": [row["gate"] for row in gates if not row["pass"]]}, ensure_ascii=False, indent=2))
    return 0 if status == "ok" else 1


if __name__ == "__main__":
    raise SystemExit(main())
