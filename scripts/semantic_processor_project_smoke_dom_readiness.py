#!/usr/bin/env python3
"""Project final smoke targets through the current DOM implementation.

This evaluator-side tool reconstructs source-only evidence from the exact
contract excerpts, invokes agent/cmd/domreadiness, and optionally freezes the
result. It does not call the product Agent or mutate a project.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf

from semantic_processor_project_smoke_analyze import window_metrics


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def load_source_evidence(source_root: Path, project: dict[str, Any], track: str) -> dict[str, Any]:
    name = str(project["source_project"])
    path = source_root / name / f"{name}_{track}.wav"
    with sf.SoundFile(path) as handle:
        sample_rate = int(handle.samplerate)
        handle.seek(int(round(float(project["start_seconds"]) * sample_rate)))
        frames = int(round((float(project["end_seconds"]) - float(project["start_seconds"])) * sample_rate))
        audio = handle.read(frames, dtype="float32", always_2d=True)
    require(len(audio) == frames, f"short source excerpt: {path}")
    mono = np.mean(audio, axis=1, dtype=np.float64)
    analysis_rate = 11025
    step = max(1, int(round(sample_rate / analysis_rate)))
    evidence = window_metrics(mono[::step], int(round(sample_rate / step)))["dom_source_evidence"]
    evidence["evidence_refs"] = [f"offline_contract_material:{project['public_case_id']}:{track}"]
    return evidence


def build_request(contract: dict[str, Any]) -> dict[str, Any]:
    plan = contract["material_plan"]
    source_root = Path(plan["source_root"]).resolve()
    cases: list[dict[str, Any]] = []
    for project in plan["projects"]:
        for issue in project["sealed_issue_assignments"]:
            cases.append({
                "case_id": issue["issue_id"],
                "project": project["public_case_id"],
                "target": issue["target_track"],
                "start_seconds": project["start_seconds"],
                "end_seconds": project["end_seconds"],
                "source": load_source_evidence(source_root, project, issue["target_track"]),
            })
    require(len(cases) == 7, "DOM readiness requires seven issue targets")
    return {"cases": cases}


def run_dom(repo_root: Path, request: dict[str, Any]) -> dict[str, Any]:
    process = subprocess.run(
        ["go", "run", "./cmd/domreadiness"],
        cwd=repo_root / "agent",
        input=json.dumps(request, ensure_ascii=True),
        text=True,
        capture_output=True,
        timeout=120,
        check=False,
    )
    require(process.returncode == 0, f"domreadiness failed: {process.stderr.strip()}")
    result = json.loads(process.stdout)
    require(result.get("schema_version") == "semantic_processor_project_dom_readiness.v1", "unexpected DOM readiness schema")
    require(len(result.get("cases", [])) == 7, "DOM readiness result count mismatch")
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", default=str(Path(__file__).with_name("semantic_processor_project_smoke_contract.json")))
    parser.add_argument("--output", default="")
    args = parser.parse_args()
    try:
        contract_path = Path(args.contract).resolve()
        repo_root = contract_path.parent.parent
        contract = json.loads(contract_path.read_text(encoding="utf-8"))
        result = run_dom(repo_root, build_request(contract))
        if args.output:
            write_json(Path(args.output).resolve(), result)
        summary = []
        for row in result["cases"]:
            projection = row["projection"]
            summary.append({
                "case_id": row["case_id"],
                "target": row["target"],
                "root_status": projection["status"],
                "can_support_family_selection": projection["trust_quality"]["can_support_family_selection"],
                "can_support_post_action_evaluation": projection["trust_quality"]["can_support_post_action_evaluation"],
                "peak_structure": projection["peak_structure"]["status"],
                "activity_structure": projection["activity_structure"]["status"],
                "frequency_time_events": projection["frequency_time_events"]["status"],
                "transient_structure": projection["transient_structure"]["status"],
                "band_dynamics": projection["band_dynamics"]["status"],
            })
        print(json.dumps({"status": "passed", "output": args.output or None, "cases": summary}, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
