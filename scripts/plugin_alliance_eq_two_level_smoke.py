#!/usr/bin/env python3
"""Two-level Plugin Alliance static-EQ holdout smoke.

Each case runs two independent transactions: numeric Bell control, then a
non-Bell shape control. Every successful transaction is undone through the
formal operation_ref path and checked against a complete paginated snapshot.
Unsupported shapes are safe rejections only when they produce zero drift.
"""
from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import static_eq_external_holdout_smoke as holdout
import waves_static_eq_phase3_smoke as phase3


def load_config(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict) or not isinstance(value.get("cases"), list):
        raise RuntimeError("two-level config must contain a cases list")
    return value


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def result_of(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else {}


def outcome_of(status: int, response: dict[str, Any]) -> str:
    result = result_of(response)
    return str(result.get("status", response.get("status", ""))).casefold()


def zero_drift(before: dict[str, float], after: dict[str, float], label: str) -> None:
    drift = holdout.changed(before, after)
    if drift:
        raise RuntimeError(f"{label}: parameter drift: {drift[:20]}")


def shape_from_result(result: dict[str, Any]) -> list[str]:
    shapes: list[str] = []
    selected = result.get("selected_section")
    if isinstance(selected, dict):
        shape = str(selected.get("shape", "")).casefold()
        if shape:
            shapes.append(shape)
    edits = result.get("edits")
    if isinstance(edits, list):
        for edit in edits:
            if isinstance(edit, dict):
                shape = str(edit.get("shape", "")).casefold()
                if shape:
                    shapes.append(shape)
    return shapes


def run_stage(base: str, case: dict[str, Any], stage: dict[str, Any],
              track_id: str, plugin_id: str, identifier: str,
              before: dict[str, float], timeout: float,
              audit: holdout.Audit, output_dir: Path) -> dict[str, Any]:
    stage_id = f"{case['id']}_{stage['level']}"
    status, response = holdout.invoke(base, "plugin_grabber.apply_eq_edits", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True,
        "edits": stage["edits"],
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{stage_id}.apply.json", response)
    result = result_of(response)
    outcome = outcome_of(status, response)
    record: dict[str, Any] = {
        "level": stage["level"],
        "instruction": stage.get("instruction", ""),
        "requested_edits": stage["edits"],
        "apply_http_status": status,
        "apply_outcome": outcome,
    }
    executable = status == 200 and outcome in {"exact", "quantized"}
    if not executable:
        after = holdout.snapshot(base, case, track_id, plugin_id, identifier,
                                 timeout, audit, output_dir, f"{stage['level']}_rejected")
        zero_drift(before, after, f"{stage_id} safe rejection")
        record.update({"status": "safely_rejected", "restored": True,
                       "rejection_drift": []})
        return record

    operation_ref, control_refs, touched = phase3.validate_apply_result(
        result, len(stage["edits"]), f"{stage_id} apply")
    applied = holdout.snapshot(base, case, track_id, plugin_id, identifier,
                               timeout, audit, output_dir, f"{stage['level']}_applied")
    if not touched or not any(param_id in touched for param_id in set(before) | set(applied)):
        raise RuntimeError(f"{stage_id}: apply reported no observable touched parameter")
    requested_shapes = [str(edit.get("shape", "")).casefold()
                        for edit in stage["edits"] if isinstance(edit, dict)]
    actual_shapes = shape_from_result(result)
    if requested_shapes and not all(shape in actual_shapes for shape in requested_shapes):
        raise RuntimeError(f"{stage_id}: requested shapes={requested_shapes} actual={actual_shapes}")

    undo_result = phase3.undo(base, track_id, plugin_id, operation_ref,
                              timeout, audit, f"{stage_id} undo")
    after = holdout.snapshot(base, case, track_id, plugin_id, identifier,
                             timeout, audit, output_dir, f"{stage['level']}_after_undo")
    zero_drift(before, after, f"{stage_id} undo")
    record.update({
        "status": outcome,
        "operation_ref": operation_ref,
        "control_refs": control_refs,
        "touched_parameter_ids": sorted(touched),
        "undo": undo_result,
        "restored": True,
        "restoration_drift": [],
        "actual_shapes": actual_shapes,
    })
    return record


def run_case(base: str, case: dict[str, Any], timeout: float,
             audit: holdout.Audit, output_dir: Path) -> dict[str, Any]:
    track_id, plugin_id, identifier, resolution = holdout.load(
        base, case, timeout, audit, output_dir)
    before = holdout.snapshot(base, case, track_id, plugin_id, identifier,
                              timeout, audit, output_dir, "initial")
    explain_status, explain_response = holdout.invoke(base, "plugin_grabber.explain_controls", {
        "track_id": track_id, "plugin_id": plugin_id,
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{case['id']}.explain.json", explain_response)
    if explain_status != 200:
        raise RuntimeError(f"{case['id']}: explain HTTP {explain_status}")
    stages = case.get("tests")
    if not isinstance(stages, list) or len(stages) != 2:
        raise RuntimeError(f"{case['id']}: exactly two test stages are required")
    results = [run_stage(base, case, stage, track_id, plugin_id, identifier,
                         before, timeout, audit, output_dir) for stage in stages]
    final = holdout.snapshot(base, case, track_id, plugin_id, identifier,
                             timeout, audit, output_dir, "final")
    zero_drift(before, final, f"{case['id']} final")
    return {
        "case_id": case["id"], "plugin_name": case["plugin_name"],
        "track_id": track_id, "plugin_id": plugin_id,
        "identifier": identifier, "resolution": resolution,
        "explain_http_status": explain_status,
        "stages": results, "restored": True, "final_drift": [],
    }


def run(args: argparse.Namespace) -> int:
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    config = load_config(Path(args.config).resolve())
    audit = holdout.Audit()
    results: list[dict[str, Any]] = []
    for case in config["cases"]:
        print(f"two-level holdout: {case['plugin_name']}", flush=True)
        results.append(run_case(args.agent_http, case, args.timeout_sec,
                                audit, output_dir))
    summary = {
        "schema_version": "plugin_alliance.eq_two_level_live_smoke.v1",
        "status": "passed",
        "case_count": len(results),
        "stage_count": sum(len(row["stages"]) for row in results),
        "results": results,
        "audit": {
            **audit.summary(),
            "external_holdout_instance_count": len(results),
            "plugin_alliance_parameter_read_count": len(results),
        },
    }
    write_json(output_dir / "summary.json", summary)
    print(f"PASS: cases={summary['case_count']} stages={summary['stage_count']}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    return run(parser.parse_args())


if __name__ == "__main__":
    raise SystemExit(main())
