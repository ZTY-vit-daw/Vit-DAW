#!/usr/bin/env python3
"""Run the disclosed Plugin Alliance round-1 cohort as a v1.1 regression set."""
from __future__ import annotations

import argparse
import json
import math
import time
import traceback
from pathlib import Path
from typing import Any

import compressor_apply_smoke as apply_smoke
import compressor_compat_matrix_smoke as matrix
import compressor_plugin_alliance_blind_v1 as blind


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("compressor_plugin_alliance_blind_v1.json")
DEFAULT_RESOLUTION_FREEZE = (
    REPO_ROOT / "VitApp" / "Workspace" / "Artifacts" / "smoke" /
    "product_path_20260804_081349" / "compressor_control" /
    "plugin_alliance_blind_v1" / "freeze.json"
)
TRACK_PREFIX = "PA regression v1.1"

EXPECTED_BOUNDARIES = {
    "bx_limiter_true_peak_negative": "unsupported_limiter",
    "lindell_mbc_negative": "unsupported_multiband_compressor",
    "spl_de_esser_dual_band_negative": "unsupported_de_esser",
    "spl_transient_designer_plus_negative": "not_compressor",
    "unfiltered_audio_g8_negative": "unsupported_gate_expander",
    "pro_audio_dsp_dsm_v3_negative": "unsupported_spectral_dynamics",
}


def fallback_target(binding: dict[str, Any], current: float) -> float:
    role = matrix.first_text(binding, "role")
    if role in {"threshold", "reduction_amount", "low_level_amount", "high_level_amount"}:
        return current - 6.0
    if role in {"input_drive", "makeup_gain", "output_gain", "wet_gain", "dry_gain"}:
        return current + 3.0
    if role in {"ratio", "direction_curve"}:
        return round(current + max(0.5, abs(current) * 0.2), 1)
    if role in {"attack", "release", "recovery", "time_constant", "pdr_time", "lookahead", "hold"}:
        return current + max(1.0, abs(current) * 0.2)
    if role == "mix":
        return current - 10.0 if current >= 50.0 else current + 10.0
    return current + max(1.0, abs(current) * 0.1)


def control_for_binding(binding: dict[str, Any]) -> dict[str, Any]:
    control_ref = matrix.first_text(binding, "control_ref")
    role = matrix.first_text(binding, "role")
    if not control_ref:
        raise RuntimeError(f"{role} omitted control_ref")
    if binding.get("transactional_probe_required") is True:
        current = binding.get("current_physical")
        if not isinstance(current, (int, float)) or not math.isfinite(float(current)):
            raise RuntimeError(f"{role} omitted transactional probe seed")
        return {
            "control_ref": control_ref,
            "display_value": fallback_target(binding, float(current)),
        }
    forward, _ = blind.control_pair(binding)
    return forward


def select_controls(inspect: dict[str, Any], policy: dict[str, Any]) -> tuple[list[dict[str, Any]], list[str]]:
    bindings = apply_smoke.compressor_bindings(inspect)
    priorities = [str(value) for value in policy.get("apply_role_priority") or []]
    maximum = int(policy.get("max_apply_controls_per_positive") or 1)
    controls: list[dict[str, Any]] = []
    roles: list[str] = []
    used: set[str] = set()
    for role in priorities:
        for binding in bindings:
            if matrix.first_text(binding, "role") != role:
                continue
            param_id = matrix.first_text(binding, "param_id")
            if not param_id or param_id in used:
                continue
            try:
                control = control_for_binding(binding)
            except RuntimeError:
                continue
            controls.append(control)
            roles.append(role)
            used.add(param_id)
            break
        if len(controls) >= maximum:
            break
    if not controls:
        raise RuntimeError("no preferred compressor binding had a reversible v1.1 target")
    return controls, roles


def run_case(base: str, case: dict[str, Any], resolution: dict[str, Any],
             policy: dict[str, Any], timeout: float, case_dir: Path) -> dict[str, Any]:
    case_id = matrix.first_text(case, "id")
    expectation = matrix.first_text(case, "expectation")
    identifier = matrix.first_text(resolution, "plugin_identifier")
    tolerance = float(policy.get("snapshot_tolerance") or 0.0001)
    evidence: dict[str, Any] = {
        "schema_version": "plugin_grabber.compressor_regression_case.v1.1",
        "case": case,
        "disclosed_round1_resolution": resolution,
        "started_at": blind.utc_now(),
    }
    result: dict[str, Any] = {
        "id": case_id,
        "plugin_name": matrix.first_text(case, "plugin_name"),
        "expectation": expectation,
        "status": "failed",
        "temporary_track_deleted": False,
    }
    track_id = ""
    try:
        evidence["track_add_response"] = matrix.invoke(base, "track.add_audio", {
            "name": f"{TRACK_PREFIX} {case_id}"}, timeout, True)
        track = matrix.require_ok(evidence["track_add_response"], "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        evidence["plugin_load_response"] = matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "plugin_identifier": identifier,
        }, timeout, True)
        loaded = matrix.require_ok(evidence["plugin_load_response"], "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        evidence["before_parameters_response"] = blind.get_parameters(
            base, track_id, plugin_id, identifier, timeout)
        before = blind.snapshot_from_response(evidence["before_parameters_response"])
        result["parameter_count"] = len(before)
        evidence["inspect_response_1"] = matrix.invoke(
            base, "plugin_grabber.inspect_compressor", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout)

        if expectation == "compressor":
            inspect_1 = matrix.require_ok(evidence["inspect_response_1"], "inspect first pass")
            inspect_summary = matrix.validate_inspect(inspect_1)
            evidence["inspect_response_2"] = matrix.invoke(
                base, "plugin_grabber.inspect_compressor", {
                    "track_id": track_id, "plugin_id": plugin_id}, timeout)
            inspect_2 = matrix.require_ok(evidence["inspect_response_2"], "inspect second pass")
            matrix.validate_inspect(inspect_2)
            generation_1 = blind.inspect_generation(inspect_1)
            generation_2 = blind.inspect_generation(inspect_2)
            if not generation_1 or generation_1 != generation_2:
                raise RuntimeError(f"topology generation drifted: {generation_1!r} -> {generation_2!r}")
            controls, roles = select_controls(inspect_2, policy)
            evidence["selected_controls"] = controls
            evidence["apply_response"] = matrix.invoke(
                base, "plugin_grabber.apply_compressor_controls", {
                    "track_id": track_id, "plugin_id": plugin_id,
                    "atomic": True, "controls": controls,
                }, timeout, True)
            applied = matrix.require_ok(evidence["apply_response"], "typed apply")
            applied_controls = [row for row in applied.get("controls") or [] if isinstance(row, dict)]
            if len(applied_controls) != len(controls) or not all(row.get("actual_readback") for row in applied_controls):
                raise RuntimeError("typed apply omitted requested control readback")
            restore_ref = matrix.first_text(applied, "restore_ref")
            if not restore_ref:
                raise RuntimeError("typed apply omitted exact normalized restore_ref")
            evidence["restore_response"] = matrix.invoke(
                base, "plugin_grabber.apply_compressor_controls", {
                    "track_id": track_id, "plugin_id": plugin_id,
                    "atomic": True, "restore_ref": restore_ref,
                }, timeout, True)
            restored = matrix.require_ok(evidence["restore_response"], "typed restore")
            evidence["after_parameters_response"] = blind.get_parameters(
                base, track_id, plugin_id, identifier, timeout)
            after = blind.snapshot_from_response(evidence["after_parameters_response"])
            mismatches = blind.snapshot_mismatches(before, after, tolerance)
            evidence["snapshot_mismatches"] = mismatches
            if mismatches:
                raise RuntimeError(f"complete snapshot restore failed for {len(mismatches)} parameters")
            result.update({
                **inspect_summary,
                "status": "passed",
                "topology_generation_stable": True,
                "selected_roles": roles,
                "apply_status": applied.get("status"),
                "restore_status": restored.get("status"),
                "write_count": len(applied.get("writes") or []),
                "full_snapshot_restored": True,
            })
        else:
            code = blind.rejection_code(evidence["inspect_response_1"])
            expected = EXPECTED_BOUNDARIES[case_id]
            if code != expected:
                raise RuntimeError(f"negative inspect rejection={code!r}, expected {expected!r}")
            evidence["after_parameters_response"] = blind.get_parameters(
                base, track_id, plugin_id, identifier, timeout)
            after = blind.snapshot_from_response(evidence["after_parameters_response"])
            mismatches = blind.snapshot_mismatches(before, after, tolerance)
            evidence["snapshot_mismatches"] = mismatches
            if mismatches:
                raise RuntimeError(f"negative inspect changed {len(mismatches)} parameters")
            result.update({
                "status": "passed",
                "rejection_code": code,
                "parameters_unchanged": True,
                "write_count": 0,
            })
    except Exception as error:
        result["error"] = str(error)
        evidence["exception"] = {
            "type": type(error).__name__,
            "message": str(error),
            "traceback": traceback.format_exc(),
        }
    finally:
        if track_id:
            try:
                evidence["track_delete_response"] = matrix.invoke(
                    base, "track.delete", {"track_id": track_id}, timeout, True)
                matrix.require_ok(evidence["track_delete_response"], "track.delete")
                result["temporary_track_deleted"] = True
            except Exception as cleanup_error:
                result["status"] = "failed"
                result["cleanup_error"] = str(cleanup_error)
        evidence["completed_at"] = blind.utc_now()
        evidence["result"] = result
        blind.write_json(case_dir / "evidence.json", evidence)
    return result


def cleanup_audit(base: str, timeout: float) -> dict[str, Any]:
    response = matrix.invoke(base, "track.list", {}, timeout)
    rows = blind.result_of(response)
    tracks: list[dict[str, Any]] = []
    for key in ("tracks", "items", "entries"):
        if isinstance(rows.get(key), list):
            tracks = [row for row in rows[key] if isinstance(row, dict)]
            break
    leftovers = [row for row in tracks if matrix.first_text(row, "name", "track_name").startswith(TRACK_PREFIX)]
    return {"clean": not leftovers, "leftover_tracks": leftovers, "track_count": len(tracks)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", default=str(DEFAULT_MANIFEST))
    parser.add_argument("--resolution-freeze", default=str(DEFAULT_RESOLUTION_FREEZE))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    manifest = blind.load_manifest(Path(args.manifest).resolve())
    freeze = json.loads(Path(args.resolution_freeze).resolve().read_text(encoding="utf-8"))
    resolutions = {
        matrix.first_text(row, "id"): row
        for row in freeze.get("identity_only_preflight") or [] if isinstance(row, dict)
    }
    output = Path(args.output_dir).resolve()
    if output.exists() and any(output.iterdir()):
        raise RuntimeError(f"regression output directory must be empty: {output}")
    output.mkdir(parents=True, exist_ok=True)
    health = blind.health_check(args.agent_http)
    cases = [case for case in manifest.get("cases") or [] if isinstance(case, dict)]
    results = []
    for index, case in enumerate(cases, 1):
        case_id = matrix.first_text(case, "id")
        print(f"[{index}/{len(cases)}] {case_id}", flush=True)
        resolution = resolutions.get(case_id)
        if resolution is None:
            result = {"id": case_id, "status": "failed", "error": "disclosed resolution missing"}
        else:
            result = run_case(args.agent_http, case, resolution, manifest.get("policy") or {},
                              args.timeout_sec, output / "cases" / f"{index:02d}_{case_id}")
        results.append(result)
        print(f"  {result.get('status')} {result.get('error', '')}", flush=True)
    cleanup = cleanup_audit(args.agent_http, args.timeout_sec)
    passed = [row for row in results if row.get("status") == "passed"]
    summary = {
        "schema_version": "plugin_grabber.compressor_plugin_alliance_regression.v1.1",
        "source_set": matrix.first_text(manifest, "set_id"),
        "blind_claim": False,
        "sealed_reserve_parameters_read": 0,
        "agent_health": health,
        "completed_at": blind.utc_now(),
        "verdict": "passed" if len(passed) == len(results) and cleanup["clean"] else "failed",
        "counts": {"total": len(results), "passed": len(passed), "failed": len(results) - len(passed)},
        "cleanup_audit": cleanup,
        "expected_boundaries": EXPECTED_BOUNDARIES,
        "results": results,
    }
    blind.write_json(output / "summary.json", summary)
    print(json.dumps({"verdict": summary["verdict"], "counts": summary["counts"]}, ensure_ascii=False))
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
