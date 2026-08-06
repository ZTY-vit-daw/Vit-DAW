from __future__ import annotations

import argparse
import hashlib
import json
import subprocess
import sys
from pathlib import Path
from typing import Any


REQUIRED_DIMENSIONS = {
    "source_dynamics",
    "gain_action",
    "transient_response",
    "recovery_motion",
    "level_effect",
    "stereo_behavior",
    "trigger_relation",
}


def gate(name: str, passed: bool, detail: Any = None) -> dict[str, Any]:
    row: dict[str, Any] = {"gate": name, "pass": bool(passed)}
    if detail is not None:
        row["detail"] = detail
    return row


def contains_forbidden_raw(value: Any) -> str | None:
    forbidden_keys = {
        "aligned_envelope_frames", "input_event_candidates", "frames", "event_list",
        "frame_trace", "gain_trace", "raw_samples", "raw_waveform", "shared_memory",
        "render_file_path", "render_path", "file_path",
    }
    if isinstance(value, dict):
        for key, child in value.items():
            if key.lower() in forbidden_keys:
                return key
            found = contains_forbidden_raw(child)
            if found:
                return found
    elif isinstance(value, list):
        for child in value:
            found = contains_forbidden_raw(child)
            if found:
                return found
    return None


def main() -> int:
    parser = argparse.ArgumentParser(description="COM-3 real artifact derivation smoke")
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--artifact", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    artifact_path = Path(args.artifact).resolve()
    output_path = Path(args.output).resolve()
    artifact_raw = artifact_path.read_bytes()
    artifact = json.loads(artifact_raw)

    process = subprocess.run(
        ["go", "run", "./cmd/comderive", "-artifact", str(artifact_path)],
        cwd=repo / "agent", capture_output=True, text=True, encoding="utf-8",
    )
    projection: dict[str, Any] = {}
    decode_error = ""
    if process.stdout.strip():
        try:
            projection = json.loads(process.stdout)
        except json.JSONDecodeError as exc:
            decode_error = str(exc)

    identifiability = {
        str(row.get("dimension")): row
        for row in projection.get("identifiability", {}).get("dimensions", [])
        if isinstance(row, dict)
    }
    typed = {
        name for name in REQUIRED_DIMENSIONS
        if name == "source_dynamics" or isinstance(projection.get(name), dict)
    }
    forbidden_parameter_keys = {
        "threshold_db", "ratio", "knee", "attack_ms", "release_ms", "lookahead_ms",
        "makeup_gain_db", "mix", "channel_link", "sidechain_filter_hz",
    }
    projection_text = json.dumps(projection, ensure_ascii=False).lower()
    inferred_parameter = next(
        (key for key in sorted(forbidden_parameter_keys) if f'"{key}":' in projection_text),
        None,
    )
    facts = projection.get("llm_context", {}).get("compact_facts", [])
    gates = [
        gate("command_exit", process.returncode == 0, process.stderr.strip() or None),
        gate("projection_json", bool(projection) and not decode_error, decode_error or None),
        gate("paired_mode_ready", projection.get("mode") == "paired_io" and projection.get("status") in {"ready", "partial"}),
        gate("projection_identity", str(projection.get("projection_id", "")).startswith("com_")),
        gate("evidence_identity", projection.get("evidence_refs") == [artifact.get("evidence_ref")]),
        gate("scope_revision", projection.get("processor_scope", {}).get("scope_revision") == artifact.get("processor_scope", {}).get("scope_revision")),
        gate("render_revision", projection.get("conditions", {}).get("render_revision") == artifact.get("conditions", {}).get("render_revision")),
        gate("typed_dimensions", typed == REQUIRED_DIMENSIONS, sorted(REQUIRED_DIMENSIONS - typed)),
        gate("identifiability_rows", REQUIRED_DIMENSIONS <= set(identifiability), sorted(REQUIRED_DIMENSIONS - set(identifiability))),
        gate("gain_action_identifiable", identifiability.get("gain_action", {}).get("status") in {"identified", "bounded"}),
        gate("transient_identifiable", identifiability.get("transient_response", {}).get("status") in {"identified", "bounded"}),
        gate("recovery_identifiable", identifiability.get("recovery_motion", {}).get("status") in {"identified", "bounded"}),
        gate("trigger_not_overclaimed", identifiability.get("trigger_relation", {}).get("status") == "not_identifiable"),
        gate("raw_trace_excluded", contains_forbidden_raw(projection) is None, contains_forbidden_raw(projection)),
        gate("numeric_controls_not_inferred", inferred_parameter is None, inferred_parameter),
        gate("compact_context_bound", len(facts) <= 24 and projection.get("llm_context", {}).get("do_not_include_raw_package") is True, len(facts)),
    ]
    report = {
        "schema_version": "com.behavior_derivation_smoke.v1",
        "status": "ok" if all(row["pass"] for row in gates) else "failed",
        "artifact_sha256": hashlib.sha256(artifact_raw).hexdigest(),
        "pair_id": artifact.get("pair_id"),
        "projection_id": projection.get("projection_id"),
        "projection_status": projection.get("status"),
        "behavior_summary": {
            name: projection.get(name) for name in sorted(REQUIRED_DIMENSIONS - {"source_dynamics"})
        },
        "gates": gates,
    }
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"status": report["status"], "report": str(output_path), "projection_id": report["projection_id"], "gates": gates}, ensure_ascii=False, indent=2))
    return 0 if report["status"] == "ok" else 1


if __name__ == "__main__":
    sys.exit(main())
