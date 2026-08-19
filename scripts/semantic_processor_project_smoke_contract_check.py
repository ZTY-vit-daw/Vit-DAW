#!/usr/bin/env python3
"""Validate the frozen seven-family project smoke design.

This checker is evaluator-side. It validates contract invariants and the local
source inventory; it does not build fixtures, call the Agent, or mutate a DAW
project.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf

import semantic_processor_project_smoke_qualify as qualifier
import semantic_processor_project_smoke_dom_readiness as dom_readiness


EXPECTED_FAMILIES = {
    "static_eq",
    "broadband_compressor",
    "limiter",
    "gate_expander",
    "de_esser",
    "transient_shaper",
    "multiband_dynamics",
}
EXPECTED_TRACKS = ("bass", "drums", "guitar", "other", "piano", "vocals")
FINE_GRAINED_DOM_VIEWS = {
    "track.frequency_time_events",
    "track.transient_structure",
    "track.band_dynamics",
}


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def rows(value: Any) -> list[dict[str, Any]]:
    return [row for row in value if isinstance(row, dict)] if isinstance(value, list) else []


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def block_rms(audio: np.ndarray, frames: int) -> np.ndarray:
    usable = len(audio) // frames * frames
    if usable <= 0:
        return np.empty(0, dtype=np.float64)
    blocks = audio[:usable].reshape(-1, frames)
    return np.sqrt(np.mean(np.square(blocks, dtype=np.float64), axis=1) + 1e-30)


def active_ratio_100ms(audio: np.ndarray, sample_rate: int) -> float:
    values = block_rms(audio, max(1, int(round(sample_rate * 0.1))))
    require(len(values) > 0, "excerpt is too short for a 100 ms activity check")
    values_db = 20.0 * np.log10(np.maximum(values, 1e-30))
    active_floor = max(float(np.percentile(values_db, 90)) - 38.0, -72.0)
    return float(np.mean(values_db > active_floor))


def excerpt_metrics(path: Path, start_seconds: float, end_seconds: float) -> dict[str, float]:
    with sf.SoundFile(path) as handle:
        start_frame = int(round(start_seconds * handle.samplerate))
        frame_count = int(round((end_seconds - start_seconds) * handle.samplerate))
        require(start_frame >= 0 and start_frame + frame_count <= handle.frames, f"excerpt outside source: {path}")
        handle.seek(start_frame)
        audio = handle.read(frame_count, dtype="float32", always_2d=True)
        sample_rate = int(handle.samplerate)
    mono = np.mean(audio, axis=1, dtype=np.float64)
    rms = float(np.sqrt(np.mean(np.square(mono)) + 1e-30))
    return {
        "rms_dbfs": -180.0 if rms <= 0 else 20.0 * math.log10(rms),
        "active_ratio_100ms": active_ratio_100ms(mono, sample_rate),
    }


def validate_schema(contract: dict[str, Any]) -> None:
    require(contract.get("schema_version") == "semantic_processor_agent_project_smoke_contract.v1", "unexpected schema_version")
    require(contract.get("status") == "frozen_design", "contract is not frozen_design")
    scope = contract.get("scope") if isinstance(contract.get("scope"), dict) else {}
    require(set(scope.get("executable_families", [])) == EXPECTED_FAMILIES, "seven-family scope mismatch")
    require(scope.get("inspect_only_families") == ["spectral_dynamics"], "Spectral Dynamics boundary changed")
    require(scope.get("forbidden_substitutes") == ["clipper_as_limiter"], "Clipper boundary changed")

    blindness = contract.get("blindness_contract") if isinstance(contract.get("blindness_contract"), dict) else {}
    for key in (
        "runner_reads_public_manifest_only",
        "sealed_truth_not_in_agent_context",
        "case_ids_are_semantically_opaque",
        "same_neutral_prompt_for_all_projects",
        "server_must_not_add_or_replace_model_view_ids",
    ):
        require(blindness.get(key) is True, f"blindness gate {key} is not true")
    forbidden = set(blindness.get("agent_context_forbidden_fields", []))
    for key in ("expected_family", "expected_target_track", "view_ids", "required_coverage", "candidate_identifier", "parameter_id"):
        require(key in forbidden, f"Agent context does not forbid {key}")

    agent_input = contract.get("agent_input") if isinstance(contract.get("agent_input"), dict) else {}
    require(agent_input.get("preselected_track_forbidden") is True, "preselected track is not forbidden")
    require(agent_input.get("preselected_plugin_forbidden") is True, "preselected plugin is not forbidden")
    prompt = str(agent_input.get("initial_prompt", ""))
    for family in EXPECTED_FAMILIES | {"spectral_dynamics", "clipper"}:
        require(family not in prompt.lower(), f"neutral prompt leaks family {family}")

    result = contract.get("result_contract") if isinstance(contract.get("result_contract"), dict) else {}
    classifications = set(result.get("case_classifications", []))
    for value in ("evaluator_disagreement", "model_no_op", "model_blocked", "not_exercised_by_model"):
        require(value in classifications, f"result classification missing {value}")
    evidence = set(result.get("execution_evidence_required_fields", []))
    for value in ("pca_preload_receipts", "pca_postload_receipts", "typed_controller_receipts", "transaction_receipts", "parameter_readbacks", "snapshot_verifications", "post_action_observation_receipts"):
        require(value in evidence, f"execution evidence field missing {value}")

    schemas = contract.get("artifact_schemas") if isinstance(contract.get("artifact_schemas"), dict) else {}
    for name in ("public_manifest", "sealed_truth", "run_checkpoint", "run_report", "evaluation_report"):
        require(isinstance(schemas.get(name), dict), f"artifact schema missing {name}")
        require(str(schemas[name].get("schema_version", "")).endswith(".v1"), f"artifact schema version missing for {name}")
    require(schemas["run_report"].get("sealed_truth_opened_by_runner_must_equal") is False, "run report does not freeze sealed truth boundary")


def validate_materials(contract: dict[str, Any], verify_hashes: bool) -> list[dict[str, Any]]:
    plan = contract.get("material_plan") if isinstance(contract.get("material_plan"), dict) else {}
    projects = rows(plan.get("projects"))
    require(plan.get("project_count") == 2 and len(projects) == 2, "material plan must contain exactly two projects")
    require(tuple(plan.get("track_order", [])) == EXPECTED_TRACKS, "track order mismatch")
    source_root = Path(str(plan.get("source_root", ""))).resolve()
    require(source_root.is_dir(), f"source root missing: {source_root}")
    gate = plan.get("context_validity_gate") if isinstance(plan.get("context_validity_gate"), dict) else {}
    rms_gate = float(gate.get("per_track_rms_dbfs_greater_than", -45.0))
    active_gate = float(gate.get("per_track_active_ratio_100ms_at_least", 0.15))

    seen_cases: set[str] = set()
    seen_issues: set[str] = set()
    families: set[str] = set()
    measurements: list[dict[str, Any]] = []
    for project in projects:
        case_id = str(project.get("public_case_id", ""))
        require(case_id and case_id not in seen_cases, f"duplicate or empty public_case_id {case_id!r}")
        seen_cases.add(case_id)
        source_project = str(project.get("source_project", ""))
        source_dir = source_root / source_project
        require(source_dir.is_dir(), f"source project missing: {source_dir}")
        start = float(project.get("start_seconds", -1))
        end = float(project.get("end_seconds", -1))
        require(abs((end - start) - 20.0) < 1e-9, f"{case_id} must be exactly 20 seconds")

        source_files = rows(project.get("source_files"))
        require({str(row.get("track")) for row in source_files} == set(EXPECTED_TRACKS), f"{case_id} source track set mismatch")
        formats: set[tuple[int, int, str]] = set()
        for source in source_files:
            track = str(source["track"])
            path = source_dir / f"{source_project}_{track}.wav"
            require(path.is_file(), f"source file missing: {path}")
            if verify_hashes:
                require(sha256_file(path) == source.get("sha256"), f"source hash mismatch: {path}")
            info = sf.info(path)
            formats.add((int(info.samplerate), int(info.channels), str(info.subtype)))
            metric = excerpt_metrics(path, start, end)
            require(metric["rms_dbfs"] > rms_gate, f"{case_id}:{track} fails RMS context gate: {metric}")
            require(metric["active_ratio_100ms"] >= active_gate, f"{case_id}:{track} fails activity context gate: {metric}")
            measurements.append({"public_case_id": case_id, "track": track, **{key: round(value, 4) for key, value in metric.items()}})
        require(formats == {(44100, 2, "PCM_16")}, f"{case_id} source format mismatch: {formats}")

        issues = rows(project.get("sealed_issue_assignments"))
        targets: set[str] = set()
        for issue in issues:
            issue_id = str(issue.get("issue_id", ""))
            target = str(issue.get("target_track", ""))
            family = str(issue.get("expected_family", ""))
            require(issue_id and issue_id not in seen_issues, f"duplicate or empty issue_id {issue_id!r}")
            require(target in EXPECTED_TRACKS and target not in targets, f"{case_id} repeats or has invalid target {target}")
            require(family in EXPECTED_FAMILIES and family not in families, f"duplicate or invalid expected family {family}")
            require(isinstance(issue.get("fault_recipe"), dict) and issue["fault_recipe"].get("kind"), f"{issue_id} has no fault recipe")
            require(str(issue.get("detectability_assertion", "")).strip(), f"{issue_id} has no detectability assertion")
            seen_issues.add(issue_id)
            targets.add(target)
            families.add(family)
    require(len(seen_issues) == 7 and families == EXPECTED_FAMILIES, "sealed assignments do not cover each family exactly once")
    return measurements


def validate_readiness(contract: dict[str, Any]) -> None:
    readiness = contract.get("observation_readiness") if isinstance(contract.get("observation_readiness"), dict) else {}
    result = readiness.get("final_material_dom_result") if isinstance(readiness.get("final_material_dom_result"), dict) else {}
    require(readiness.get("verified_with_current_dom_build") is True, "final material was not verified with current DOM Build")
    require(result.get("root_status") == "partial", "frozen DOM root status must remain partial")
    require(result.get("can_support_family_selection") is True, "DOM cannot support family selection")
    require(result.get("can_support_post_action_evaluation") is False, "DOM v1 must not claim post-action evaluation")
    dimensions = result.get("dimensions") if isinstance(result.get("dimensions"), dict) else {}
    require(dimensions.get("track.peak_structure") == "ready", "peak structure readiness changed")
    require(dimensions.get("track.activity_structure") == "ready", "activity structure readiness changed")
    for view in FINE_GRAINED_DOM_VIEWS:
        require(dimensions.get(view) == "partial", f"{view} must be truthfully frozen as partial")

    matrix = {str(row.get("view_id")): row for row in rows(readiness.get("view_matrix"))}
    for view in FINE_GRAINED_DOM_VIEWS:
        require(matrix.get(view, {}).get("formal_run_gate") == "must become ready before seven-family signoff", f"{view} launch gate missing")
    blockers = set(readiness.get("formal_run_launch_blockers_at_freeze", []))
    require(len(blockers) >= 4, "formal launch blockers are incomplete")

    preflight = contract.get("runtime_preflight") if isinstance(contract.get("runtime_preflight"), dict) else {}
    checks = set(preflight.get("hard_fail_checks", []))
    for phrase in (
        "track.frequency_time_events is ready on the de-esser target",
        "track.transient_structure is ready on the transient target",
        "track.band_dynamics is ready on the multiband target",
        "track.activity_structure exposes a usable noise-floor fact on the Gate target",
    ):
        require(phrase in checks, f"runtime preflight missing: {phrase}")


def without_generated_at(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: without_generated_at(child) for key, child in value.items() if key != "generated_at"}
    if isinstance(value, list):
        return [without_generated_at(child) for child in value]
    return value


def validate_dom_artifact(contract: dict[str, Any], contract_path: Path) -> dict[str, Any]:
    readiness = contract["observation_readiness"]
    artifact_path = contract_path.parent.parent / str(readiness.get("dom_readiness_artifact", ""))
    require(artifact_path.is_file(), f"DOM readiness artifact missing: {artifact_path}")
    artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
    require(artifact.get("schema_version") == readiness.get("dom_readiness_artifact_schema"), "DOM readiness artifact schema mismatch")
    repo_root = contract_path.parent.parent
    recomputed = dom_readiness.run_dom(repo_root, dom_readiness.build_request(contract))
    require(without_generated_at(recomputed) == without_generated_at(artifact), "DOM readiness artifact does not match current DOM Build")
    expected_targets = {
        (str(issue["issue_id"]), str(project["public_case_id"]), str(issue["target_track"]))
        for project in contract["material_plan"]["projects"]
        for issue in project["sealed_issue_assignments"]
    }
    actual_targets = {(str(row["case_id"]), str(row["project"]), str(row["target"])) for row in artifact.get("cases", [])}
    require(actual_targets == expected_targets, "DOM readiness target set mismatch")
    return {"artifact": str(artifact_path), "case_count": len(actual_targets), "matches_current_dom_build": True}


def validate_qualification(contract: dict[str, Any], contract_path: Path) -> dict[str, Any]:
    plan = contract["material_plan"]
    frozen = plan.get("offline_recipe_qualification") if isinstance(plan.get("offline_recipe_qualification"), dict) else {}
    require(frozen.get("status") == "qualified_in_memory", "recipe qualification is not frozen as qualified")
    artifact_path = contract_path.parent.parent / str(frozen.get("qualification_artifact", ""))
    require(artifact_path.is_file(), f"qualification artifact missing: {artifact_path}")
    artifact = json.loads(artifact_path.read_text(encoding="utf-8"))
    require(artifact.get("schema_version") == frozen.get("qualification_artifact_schema"), "qualification artifact schema mismatch")
    require(artifact.get("contract_id") == contract.get("contract_id"), "qualification contract id mismatch")
    require(artifact.get("status") == "qualified" and artifact.get("audio_written") is False, "qualification artifact is not a read-only success")
    source_root = Path(plan["source_root"]).resolve()
    peak_ceiling = float(plan["interference_gates"]["clean_and_problem_mix_peak_ceiling_dbfs"])
    recomputed_projects = [qualifier.qualify_project(source_root, project, peak_ceiling) for project in plan["projects"]]
    recomputed = {
        "schema_version": "semantic_processor_project_smoke_recipe_qualification.v1",
        "contract_id": contract["contract_id"],
        "status": "qualified",
        "audio_written": False,
        "project_count": len(recomputed_projects),
        "issue_count": sum(project["issue_count"] for project in recomputed_projects),
        "projects": recomputed_projects,
    }
    require(recomputed == artifact, "qualification artifact does not match recomputation from exact sources")
    return {
        "artifact": str(artifact_path),
        "project_count": int(artifact["project_count"]),
        "issue_count": int(artifact["issue_count"]),
        "audio_written": bool(artifact["audio_written"]),
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--contract",
        default=str(Path(__file__).with_name("semantic_processor_project_smoke_contract.json")),
    )
    parser.add_argument("--skip-source-hashes", action="store_true")
    args = parser.parse_args()
    try:
        path = Path(args.contract).resolve()
        contract = json.loads(path.read_text(encoding="utf-8"))
        require(isinstance(contract, dict), "contract root must be an object")
        validate_schema(contract)
        measurements = validate_materials(contract, not args.skip_source_hashes)
        validate_readiness(contract)
        dom_artifact = validate_dom_artifact(contract, path)
        qualification = validate_qualification(contract, path)
        print(json.dumps({
            "status": "passed",
            "contract": str(path),
            "project_count": 2,
            "family_count": 7,
            "source_hashes_verified": not args.skip_source_hashes,
            "context_measurements": measurements,
            "dom_readiness": dom_artifact,
            "recipe_qualification": qualification,
            "formal_run_readiness": contract.get("formal_run_readiness"),
            "formal_run_launch_blocker_count": len(contract["observation_readiness"]["formal_run_launch_blockers_at_freeze"]),
        }, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
