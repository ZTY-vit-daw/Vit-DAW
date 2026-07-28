#!/usr/bin/env python3
"""Generic A/B/C smoke for an external EQ VPS Control Graph document."""
from __future__ import annotations

import argparse
import json
import shutil
from pathlib import Path
from typing import Any

import static_eq_external_holdout_smoke as holdout
import waves_static_eq_phase3_smoke as phase3


def read_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise RuntimeError(f"expected JSON object: {path}")
    return value


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def result_of(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else {}


def assert_no_drift(before: dict[str, float], after: dict[str, float], label: str) -> None:
    drift = holdout.changed(before, after)
    if drift:
        raise RuntimeError(f"{label}: parameter drift: {drift[:20]}")


def snapshot(base: str, case: dict[str, Any], track_id: str, plugin_id: str,
             identifier: str, timeout: float, audit: holdout.Audit,
             output_dir: Path, label: str) -> dict[str, float]:
    return holdout.snapshot(base, case, track_id, plugin_id, identifier,
                            timeout, audit, output_dir, label)


def explain(base: str, track_id: str, plugin_id: str, timeout: float,
            audit: holdout.Audit, output_dir: Path, label: str) -> dict[str, Any]:
    status, response = holdout.invoke(base, "plugin_grabber.explain_controls", {
        "track_id": track_id, "plugin_id": plugin_id,
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{label}.explain.json", response)
    if status != 200:
        raise RuntimeError(f"{label}: explain HTTP {status}: {response}")
    return result_of(response)


def apply(base: str, track_id: str, plugin_id: str, edit: dict[str, Any],
          timeout: float, audit: holdout.Audit, output_dir: Path,
          label: str) -> tuple[int, dict[str, Any], dict[str, Any]]:
    status, response = holdout.invoke(base, "plugin_grabber.apply_eq_edits", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "edits": [edit],
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{label}.apply.json", response)
    return status, response, result_of(response)


def topology_digest(explanation: dict[str, Any]) -> dict[str, Any] | None:
    eq = explanation.get("eq_band_summary")
    if not isinstance(eq, dict):
        return None
    topology = eq.get("control_topology") if isinstance(eq.get("control_topology"), dict) else {}
    return {
        "mapping_source": eq.get("mapping_source"),
        "eq_model": eq.get("eq_model"),
        "section_count": len(eq.get("sections", [])) if isinstance(eq.get("sections"), list) else 0,
        "supported_filter_kinds": sorted(str(value) for value in eq.get("supported_filter_kinds", [])),
        "generation": topology.get("generation"),
    }


def run_stage(base: str, case: dict[str, Any], stage: dict[str, Any], phase: str,
              track_id: str, plugin_id: str, identifier: str,
              timeout: float, audit: holdout.Audit, output_dir: Path) -> dict[str, Any]:
    label = f"{phase}_{stage['id']}"
    before = snapshot(base, case, track_id, plugin_id, identifier, timeout, audit, output_dir, f"{label}_before")
    status, response, result = apply(base, track_id, plugin_id, stage["edit"], timeout, audit, output_dir, label)
    if stage["expect"] == "rejected":
        code = str(result.get("rejection_code", ""))
        expected = stage.get("rejection_codes", [])
        if status != 400 or code not in expected:
            raise RuntimeError(f"{label}: expected HTTP 400 {expected}, got HTTP {status} {code}: {response}")
        after = snapshot(base, case, track_id, plugin_id, identifier, timeout, audit, output_dir, f"{label}_after")
        assert_no_drift(before, after, f"{label} rejection")
        return {"edit": stage["edit"], "http_status": status, "status": "rejected",
                "rejection_code": code, "zero_drift": True}

    outcome = str(result.get("status", ""))
    if status != 200 or outcome not in {"exact", "quantized"}:
        raise RuntimeError(f"{label}: expected success, got HTTP {status} {outcome}: {response}")
    operation_ref, control_refs, touched = phase3.validate_apply_result(result, 1, label)
    edit_result = result.get("edits", [{}])[0]
    actual = edit_result.get("actual_readback", []) if isinstance(edit_result, dict) else []
    role_rows = {str(row.get("role")): row for row in actual if isinstance(row, dict)}
    missing = sorted(set(stage.get("expected_roles", [])) - set(role_rows))
    if missing:
        raise RuntimeError(f"{label}: expected readback roles are missing: {missing}")
    for role, expected_label in stage.get("expected_labels", {}).items():
        actual_label = str(role_rows.get(role, {}).get("value_text", ""))
        if actual_label.casefold() != str(expected_label).casefold():
            raise RuntimeError(f"{label}: role {role} label {actual_label!r}, want {expected_label!r}")
    expected_activation = stage.get("expected_activation_strategy")
    if expected_activation:
        selection = edit_result.get("selection", {}) if isinstance(edit_result, dict) else {}
        activation = selection.get("activation", {}) if isinstance(selection, dict) else {}
        if activation.get("strategy") != expected_activation:
            raise RuntimeError(f"{label}: activation strategy {activation.get('strategy')!r}, want {expected_activation!r}")
    applied = snapshot(base, case, track_id, plugin_id, identifier, timeout, audit, output_dir, f"{label}_applied")
    if not holdout.changed(before, applied):
        raise RuntimeError(f"{label}: no live parameter changed")
    undo = phase3.undo(base, track_id, plugin_id, operation_ref, timeout, audit, f"{label} undo")
    after = snapshot(base, case, track_id, plugin_id, identifier, timeout, audit, output_dir, f"{label}_after_undo")
    assert_no_drift(before, after, f"{label} formal undo")
    writes = result.get("writes", [])
    return {"edit": stage["edit"], "http_status": status, "status": outcome,
            "operation_ref": operation_ref, "control_refs": control_refs,
            "touched_parameter_ids": sorted(touched), "roles": sorted(role_rows),
            "actual_readback": actual,
            "quantized_writes": [row for row in writes if isinstance(row, dict) and row.get("quantized") is True],
            "formal_undo": undo, "zero_drift_after_undo": True}


def run(args: argparse.Namespace) -> int:
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    config = read_json(Path(args.config).resolve())
    case = config["case"]
    source_vps = Path(args.vps).resolve()
    source_verification = Path(args.verification).resolve()
    runtime_dir = Path(args.runtime_dir).resolve()
    runtime_dir.mkdir(parents=True, exist_ok=True)
    runtime_vps = runtime_dir / source_vps.name
    runtime_verification = runtime_dir / source_verification.name
    runtime_vps.unlink(missing_ok=True)
    runtime_verification.unlink(missing_ok=True)
    audit = holdout.Audit()

    track_id, plugin_id, identifier, resolution = holdout.load(
        args.agent_http, case, args.timeout_sec, audit, output_dir)
    initial = snapshot(args.agent_http, case, track_id, plugin_id, identifier,
                       args.timeout_sec, audit, output_dir, "initial")

    a_explain = explain(args.agent_http, track_id, plugin_id, args.timeout_sec, audit, output_dir, "A")
    a_topology = topology_digest(a_explain)
    if bool(a_topology is not None) != bool(config["baseline_eq_present"]):
        raise RuntimeError(f"A: baseline EQ presence mismatch: {a_topology}")
    a_records = [run_stage(args.agent_http, case, stage, "A", track_id, plugin_id, identifier,
                           args.timeout_sec, audit, output_dir) for stage in config["baseline_stages"]]

    shutil.copyfile(source_vps, runtime_vps)
    shutil.copyfile(source_verification, runtime_verification)
    b_explain = explain(args.agent_http, track_id, plugin_id, args.timeout_sec, audit, output_dir, "B")
    b_topology = topology_digest(b_explain)
    if not b_topology or b_topology.get("mapping_source") != "vps_control_graph":
        raise RuntimeError(f"B: Control Graph was not loaded: {b_topology}")
    expected_sections = config.get("expected_section_count")
    if expected_sections is not None and b_topology.get("section_count") != expected_sections:
        raise RuntimeError(f"B: section count {b_topology.get('section_count')}, want {expected_sections}")
    b_records = [run_stage(args.agent_http, case, stage, "B", track_id, plugin_id, identifier,
                           args.timeout_sec, audit, output_dir) for stage in config["graph_stages"]]

    runtime_vps.unlink()
    runtime_verification.unlink()
    c_explain = explain(args.agent_http, track_id, plugin_id, args.timeout_sec, audit, output_dir, "C")
    c_topology = topology_digest(c_explain)
    if c_topology != a_topology:
        raise RuntimeError(f"C: baseline topology was not restored: A={a_topology} C={c_topology}")
    c_records = [run_stage(args.agent_http, case, stage, "C", track_id, plugin_id, identifier,
                           args.timeout_sec, audit, output_dir) for stage in config["baseline_stages"]]
    final = snapshot(args.agent_http, case, track_id, plugin_id, identifier,
                     args.timeout_sec, audit, output_dir, "final")
    assert_no_drift(initial, final, "A/B/C final")

    audit_summary = audit.summary()
    for key in ("audio_probe_count", "learning_call_count", "profile_call_count", "spal_call_count", "b4_call_count"):
        if audit_summary.get(key) != 0:
            raise RuntimeError(f"prohibited call count {key}={audit_summary.get(key)}")
    summary = {
        "schema_version": "vit.eq_vps.control_graph.smoke_result.v1", "status": "passed",
        "plugin": case["plugin_name"], "track_id": track_id, "plugin_id": plugin_id,
        "identifier": identifier, "resolution": resolution,
        "A_without_vps": {"topology": a_topology, "tests": a_records},
        "B_with_vps": {"topology": b_topology, "tests": b_records},
        "C_after_unload": {"topology": c_topology, "tests": c_records},
        "final_zero_drift": True, "audit": audit_summary,
    }
    write_json(output_dir / "summary.json", summary)
    print("PASS: EQ VPS Control Graph A/B/C smoke", flush=True)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--vps", required=True)
    parser.add_argument("--verification", required=True)
    parser.add_argument("--runtime-dir", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    return run(parser.parse_args())


if __name__ == "__main__":
    raise SystemExit(main())
