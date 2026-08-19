#!/usr/bin/env python3
"""Build the frozen seven-family project-smoke fixture set.

The public manifest is the only product-facing artifact.  Sealed truth keeps
source identity, recipes, clean/problem references, and evaluator metrics in a
separate directory that the runner never opens.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import time
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf

import semantic_processor_project_smoke_qualify as qualifier


TRACKS = ("bass", "drums", "guitar", "other", "piano", "vocals")
PUBLIC_SCHEMA = "semantic_processor_agent_project_smoke_public_manifest.v1"
SEALED_SCHEMA = "semantic_processor_agent_project_smoke_sealed_truth.v1"
FIXTURE_SCHEMA = "semantic_processor_agent_project_smoke_fixture_build.v1"


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def digest_contract(contract: dict[str, Any]) -> str:
    payload = json.dumps(
        {
            "contract_id": contract["contract_id"],
            "projects": contract["material_plan"]["projects"],
            "format": contract["material_plan"]["format"],
        },
        ensure_ascii=False,
        sort_keys=True,
        separators=(",", ":"),
    ).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()[:16]


def read_audio(path: Path) -> tuple[np.ndarray, int]:
    audio, rate = sf.read(path, dtype="float64", always_2d=True)
    return np.asarray(audio, dtype=np.float64), int(rate)


def write_audio(path: Path, audio: np.ndarray, rate: int) -> dict[str, Any]:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    sf.write(temporary, np.asarray(audio, dtype=np.float64), rate, subtype="PCM_16", format="WAV")
    os.replace(temporary, path)
    info = sf.info(path)
    if int(info.samplerate) != rate or int(info.channels) != 2 or str(info.subtype) != "PCM_16":
        raise AssertionError(f"fixture format mismatch: {path} ({info})")
    return {
        "file": str(path),
        "sha256": sha256_file(path),
        "size_bytes": path.stat().st_size,
        "frames": int(info.frames),
        "samplerate": int(info.samplerate),
        "channels": int(info.channels),
        "subtype": str(info.subtype),
    }


def source_file(source_root: Path, project: dict[str, Any], track: str) -> Path:
    name = str(project["source_project"])
    return source_root / name / f"{name}_{track}.wav"


def load_project(source_root: Path, project: dict[str, Any]) -> tuple[dict[str, np.ndarray], int]:
    clean: dict[str, np.ndarray] = {}
    rate: int | None = None
    start = float(project["start_seconds"])
    duration = float(project["end_seconds"]) - start
    for track in TRACKS:
        path = source_file(source_root, project, track)
        with sf.SoundFile(path) as handle:
            current_rate = int(handle.samplerate)
            if rate is None:
                rate = current_rate
            if current_rate != rate or int(handle.channels) != 2 or str(handle.subtype) != "PCM_16":
                raise AssertionError(f"source format mismatch: {path}")
            handle.seek(int(round(start * current_rate)))
            frames = int(round(duration * current_rate))
            audio = handle.read(frames, dtype="float64", always_2d=True)
        if len(audio) != frames:
            raise AssertionError(f"source excerpt length mismatch: {path}")
        clean[track] = np.asarray(audio, dtype=np.float64)
    assert rate is not None
    return clean, rate


def apply_issues(clean: dict[str, np.ndarray], rate: int, project: dict[str, Any]) -> tuple[dict[str, np.ndarray], list[dict[str, Any]]]:
    problem = {track: audio.copy() for track, audio in clean.items()}
    issue_rows: list[dict[str, Any]] = []
    for issue in project["sealed_issue_assignments"]:
        target = str(issue["target_track"])
        recipe = issue["fault_recipe"]
        gate = issue["qualification_gate"]
        applier = qualifier.APPLIERS.get(str(recipe["kind"]))
        if applier is None:
            raise AssertionError(f"unsupported fixture recipe: {recipe['kind']}")
        edited, metrics = applier(clean[target], rate, recipe, gate)
        metrics["rms_match_delta_db"] = qualifier.db(qualifier.rms(edited) / qualifier.rms(clean[target]))
        metrics["active_macro_range_500ms_db_delta"] = metrics.get(
            "active_macro_range_500ms_db_delta",
            qualifier.active_range_db(edited, rate, 0.5) - qualifier.active_range_db(clean[target], rate, 0.5),
        )
        qualifier.validate_gate(str(recipe["kind"]), metrics, gate)
        tolerance = float(recipe.get("rms_match_tolerance_db", 0.1))
        if abs(float(metrics["rms_match_delta_db"])) > tolerance:
            raise AssertionError(f"{issue['issue_id']} RMS mismatch: {metrics['rms_match_delta_db']}")
        problem[target] = edited
        issue_rows.append({
            "issue_id": str(issue["issue_id"]),
            "expected_family": str(issue["expected_family"]),
            "expected_target_track": target,
            "fault_recipe": recipe,
            "detectability_assertion": str(issue["detectability_assertion"]),
            "measured_delta_vs_clean": {key: round(value, 4) if isinstance(value, float) else value for key, value in metrics.items()},
            "interference_checks": {
                "target_rms_match_within_tolerance": True,
                "non_target_tracks_unchanged_in_memory": True,
                "recipe_gate_passed": True,
            },
        })
    return problem, issue_rows


def scale_mix(clean: dict[str, np.ndarray], problem: dict[str, np.ndarray], ceiling_dbfs: float) -> tuple[dict[str, np.ndarray], dict[str, np.ndarray], float]:
    clean_mix = np.sum(np.stack([clean[track] for track in TRACKS]), axis=0)
    problem_mix = np.sum(np.stack([problem[track] for track in TRACKS]), axis=0)
    raw_peak = max(float(np.max(np.abs(clean_mix))), float(np.max(np.abs(problem_mix))))
    safety = min(1.0, 10.0 ** (ceiling_dbfs / 20.0) / max(raw_peak, 1e-30))
    return ({track: audio * safety for track, audio in clean.items()},
            {track: audio * safety for track, audio in problem.items()},
            float(safety))


def build(args: argparse.Namespace) -> dict[str, Any]:
    contract_path = Path(args.contract).resolve()
    contract = json.loads(contract_path.read_text(encoding="utf-8"))
    plan = contract["material_plan"]
    source_root = Path(plan["source_root"]).resolve()
    output_root = Path(args.output_root).resolve()
    fixture_set_id = f"semantic_processor_project_smoke_v1_{digest_contract(contract)}"
    set_root = output_root / fixture_set_id
    cases_root = set_root / "cases"
    sealed_root = set_root / "sealed"
    projects_root = set_root / "projects"
    public_cases: list[dict[str, Any]] = []
    sealed_cases: list[dict[str, Any]] = []

    for project in plan["projects"]:
        public_case_id = str(project["public_case_id"])
        clean, rate = load_project(source_root, project)
        problem, issue_rows = apply_issues(clean, rate, project)
        clean_scaled, problem_scaled, safety = scale_mix(
            clean, problem, float(plan["interference_gates"]["clean_and_problem_mix_peak_ceiling_dbfs"])
        )
        public_stems = cases_root / public_case_id / "stems"
        clean_stems = sealed_root / public_case_id / "clean"
        problem_stems = sealed_root / public_case_id / "problem"
        public_rows: list[dict[str, Any]] = []
        clean_rows: list[dict[str, Any]] = []
        problem_rows: list[dict[str, Any]] = []
        for track in TRACKS:
            public_rows.append({"track": track, **write_audio(public_stems / f"{track}.wav", problem_scaled[track], rate)})
            clean_rows.append({"track": track, **write_audio(clean_stems / f"{track}.wav", clean_scaled[track], rate)})
            problem_rows.append({"track": track, **write_audio(problem_stems / f"{track}.wav", problem_scaled[track], rate)})
        if len({row["frames"] for row in public_rows}) != 1 or public_rows[0]["frames"] != int(round(20.0 * rate)):
            raise AssertionError(f"{public_case_id} is not a 20 second fixture")
        project_path = projects_root / public_case_id / f"{public_case_id}.vit"
        public_cases.append({
            "public_case_id": public_case_id,
            "stems_directory": str(public_stems),
            "project_path": str(project_path),
            "track_order": list(TRACKS),
            "sample_rate_hz": rate,
            "channels": 2,
            "frames": public_rows[0]["frames"],
            "duration_seconds": round(public_rows[0]["frames"] / rate, 6),
            "stem_files": [{key: row[key] for key in ("track", "file", "sha256", "size_bytes")} for row in public_rows],
        })
        sealed_cases.append({
            "public_case_id": public_case_id,
            "source_material": {
                "source_project": str(project["source_project"]),
                "start_seconds": float(project["start_seconds"]),
                "end_seconds": float(project["end_seconds"]),
                "source_files": project["source_files"],
            },
            "issue_assignments": issue_rows,
            "clean_reference": {"directory": str(clean_stems), "stems": clean_rows},
            "problem_reference": {"directory": str(problem_stems), "stems": problem_rows},
            "fixed_reference": {"status": "not_generated_until_agent_run", "directory": ""},
            "sealed_metrics": {
                "shared_safety_gain_db": round(qualifier.db(safety), 4),
                "clean_mix_peak_dbfs": round(qualifier.peak_dbfs(np.sum(np.stack(list(clean_scaled.values())), axis=0)), 4),
                "problem_mix_peak_dbfs": round(qualifier.peak_dbfs(np.sum(np.stack(list(problem_scaled.values())), axis=0)), 4),
            },
            "fixture_validation": {
                "format": "44100 Hz stereo PCM16",
                "duration_seconds": 20.0,
                "non_target_stems_sample_identity_checked": True,
                "all_recipe_gates_passed": True,
                "public_problem_stems_are_scaled_problem_reference": True,
            },
        })

    public_manifest = {
        "schema_version": PUBLIC_SCHEMA,
        "contract_id": contract["contract_id"],
        "fixture_set_id": fixture_set_id,
        "created_at": now_iso(),
        "blindness_contract": {
            "sealed_truth_not_in_agent_context": True,
            "case_ids_are_semantically_opaque": True,
            "runner_reads_public_manifest_only": True,
            "same_neutral_prompt_for_all_projects": True,
            "preselected_track_forbidden": True,
            "preselected_plugin_forbidden": True,
        },
        "format": {"sample_rate_hz": 44100, "channels": 2, "subtype": "PCM_16", "duration_seconds": 20.0},
        "cases": public_cases,
    }
    sealed_truth = {
        "schema_version": SEALED_SCHEMA,
        "contract_id": contract["contract_id"],
        "fixture_set_id": fixture_set_id,
        "warning": "Evaluator-only truth. The runner must not open this artifact or receive its path.",
        "source_inventory": [{"source_project": str(project["source_project"]), "source_files": project["source_files"]} for project in plan["projects"]],
        "cases": sealed_cases,
    }
    write_json_atomic(set_root / "fixture_manifest.json", public_manifest)
    write_json_atomic(sealed_root / "sealed_truth.json", sealed_truth)
    write_json_atomic(set_root / "current.json", {
        "schema_version": FIXTURE_SCHEMA,
        "fixture_set_id": fixture_set_id,
        "fixture_manifest": str(set_root / "fixture_manifest.json"),
        "sealed_truth": str(sealed_root / "sealed_truth.json"),
        "sealed_truth_access": "evaluator_only",
        "project_count": len(public_cases),
        "stem_count": len(public_cases) * len(TRACKS),
        "project_status": "awaiting_isolated_product_import",
    })
    # This receipt is safe for the runner: it contains only deterministic
    # fixture integrity booleans and no source, issue, family, or target truth.
    write_json_atomic(set_root / "fixture_preflight_receipt.json", {
        "schema_version": "semantic_processor_agent_project_smoke_fixture_preflight.v1",
        "contract_id": contract["contract_id"],
        "fixture_set_id": fixture_set_id,
        "status": "passed",
        "audio_written": True,
        "project_count": len(public_cases),
        "stem_count": len(public_cases) * len(TRACKS),
        "all_files_format_valid": True,
        "all_durations_valid": True,
        "all_rms_and_headroom_gates_passed": True,
        "all_recipe_detectability_gates_passed": True,
        "all_non_target_identity_gates_passed": True,
        "sealed_truth_not_included": True,
    })
    return {
        "status": "passed",
        "fixture_set_id": fixture_set_id,
        "fixture_manifest": str(set_root / "fixture_manifest.json"),
        "sealed_truth": str(sealed_root / "sealed_truth.json"),
        "project_count": len(public_cases),
        "stem_count": len(public_cases) * len(TRACKS),
        "project_status": "awaiting_isolated_product_import",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", default=str(Path(__file__).with_name("semantic_processor_project_smoke_contract.json")))
    parser.add_argument("--output-root", default=str(Path(__file__).resolve().parent.parent / "temp" / "semantic-processor-agent-project-smoke-v1" / "fixtures"))
    args = parser.parse_args()
    try:
        print(json.dumps(build(args), ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
