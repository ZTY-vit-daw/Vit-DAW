#!/usr/bin/env python3
"""Freeze and execute the independent Plugin Alliance compressor blind v2 set."""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import math
import time
import traceback
from pathlib import Path
from typing import Any

import compressor_apply_smoke as apply_smoke
import compressor_compat_matrix_smoke as matrix
import compressor_plugin_alliance_blind_v1 as blind_v1
import compressor_plugin_alliance_regression_v11 as regression_v11


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("compressor_plugin_alliance_blind_v2.json")
SOURCE_RESERVE_MANIFEST = SCRIPT_PATH.with_name("compressor_plugin_alliance_blind_v1.json")
TRACK_PREFIX = "PA blind v2"
SUCCESS_STATUSES = {"ok", "success", "completed"}
REJECTION_CODES = {
    "not_compressor",
    "unsupported_limiter",
    "unsupported_multiband_compressor",
    "unsupported_gate_expander",
    "unsupported_de_esser",
    "unsupported_spectral_dynamics",
}

# The production binary is authoritative; source hashes make the tested closure auditable.
FROZEN_FILES = tuple(dict.fromkeys(
    blind_v1.FROZEN_FILES
    + (
        "agent/internal/chat/plugin_eq_control.go",
        "agent/internal/vst3host/worker.go",
        "scripts/compressor_plugin_alliance_blind_v1.py",
        "scripts/compressor_plugin_alliance_regression_v11.py",
    )
))


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def file_hashes() -> dict[str, str]:
    hashes: dict[str, str] = {}
    for relative in FROZEN_FILES:
        path = REPO_ROOT / relative
        if not path.is_file():
            raise RuntimeError(f"frozen file is missing: {path}")
        hashes[relative] = sha256_file(path)
    return hashes


def source_composite(hashes: dict[str, str]) -> str:
    rows = [f"{path}\0{digest}\n" for path, digest in sorted(hashes.items())]
    return hashlib.sha256("".join(rows).encode("utf-8")).hexdigest()


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    cases = [case for case in manifest.get("cases") or [] if isinstance(case, dict)]
    reserve = [case for case in manifest.get("sealed_reserve_cases") or []
               if isinstance(case, dict)]
    policy = manifest.get("policy") or {}
    positives = [case for case in cases if case.get("expectation") == "compressor"]
    negatives = [case for case in cases if case.get("expectation") == "not_compressor"]
    hybrids = [case for case in cases if case.get("expectation") == "hybrid_boundary"]
    if len(cases) != 8 or len(positives) != 5 or len(negatives) != 1 or len(hybrids) != 2:
        raise RuntimeError(
            f"v2 composition is {len(positives)} positive / {len(negatives)} negative / "
            f"{len(hybrids)} hybrid, expected 5 / 1 / 2")
    if len(reserve) != 4:
        raise RuntimeError(f"v2 sealed reserve must contain 4 cases, got {len(reserve)}")
    if int(policy.get("sample_count") or 0) != len(cases):
        raise RuntimeError("v2 policy sample_count does not match cases")
    if len({matrix.first_text(case, "id") for case in cases + reserve}) != 12:
        raise RuntimeError("v2 case and reserve ids must be unique")
    if int(policy.get("hybrid_boundary_count") or 0) != len(hybrids):
        raise RuntimeError("v2 policy hybrid count does not match cases")
    for case in cases + reserve:
        if not matrix.first_text(case, "id") or not matrix.first_text(case, "plugin_name"):
            raise RuntimeError("v2 case omitted id or plugin_name")
        if not Path(matrix.first_text(case, "plugin_path")).is_file():
            raise RuntimeError(f"plugin file is missing: {matrix.first_text(case, 'plugin_path')}")
    source = json.loads(SOURCE_RESERVE_MANIFEST.read_text(encoding="utf-8"))
    source_cases = {
        matrix.first_text(case, "id"): matrix.first_text(case, "plugin_name")
        for case in source.get("sealed_reserve_cases") or [] if isinstance(case, dict)
    }
    v2_reserve = {matrix.first_text(case, "id"): matrix.first_text(case, "plugin_name")
                  for case in cases + reserve}
    if v2_reserve != source_cases:
        raise RuntimeError("v2 cases must be an exact partition of the v1 sealed reserve")
    return manifest


def health_check(base: str) -> dict[str, Any]:
    health = matrix.request_json("GET", base.rstrip("/") + "/health", None, 10)
    if matrix.first_text(health, "status").lower() not in SUCCESS_STATUSES:
        raise RuntimeError(f"agent health check failed: {health}")
    return health


def identity_only_preflight(manifest: dict[str, Any], timeout: float) -> list[dict[str, Any]]:
    resolutions: list[dict[str, Any]] = []
    cases = [case for case in manifest.get("cases") or [] if isinstance(case, dict)]
    for index, case in enumerate(cases, 1):
        case_id = matrix.first_text(case, "id")
        plugin_path = Path(matrix.first_text(case, "plugin_path"))
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        resolved_path = matrix.first_text(resolution, "file_or_identifier", "plugin_path", "path")
        if not matrix.path_matches(str(plugin_path), resolved_path):
            raise RuntimeError(f"{case_id}: resolved path {resolved_path!r} does not match {plugin_path}")
        resolutions.append({
            "index": index,
            "id": case_id,
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "expectation": matrix.first_text(case, "expectation"),
            "plugin_identifier": identifier,
            "resolved_path": resolved_path,
        })
        print(f"[{index}/{len(cases)}] preflight {case_id}: unique identity", flush=True)
    return resolutions


def freeze(manifest_path: Path, output: Path, base: str, timeout: float) -> dict[str, Any]:
    manifest = load_manifest(manifest_path)
    health = health_check(base)
    resolutions = identity_only_preflight(manifest, timeout)
    hashes = file_hashes()
    reserve = [case for case in manifest.get("sealed_reserve_cases") or [] if isinstance(case, dict)]
    record = {
        "schema_version": "plugin_grabber.compressor_blind_freeze.v2",
        "frozen_at": utc_now(),
        "set_id": matrix.first_text(manifest, "set_id"),
        "manifest_path": str(manifest_path.resolve()),
        "manifest_sha256": sha256_file(manifest_path),
        "source_reserve_manifest_path": str(SOURCE_RESERVE_MANIFEST.resolve()),
        "source_reserve_manifest_sha256": sha256_file(SOURCE_RESERVE_MANIFEST),
        "runner_path": str(SCRIPT_PATH),
        "runner_sha256": sha256_file(SCRIPT_PATH),
        "agent_http": base.rstrip("/"),
        "agent_health": health,
        "agent_sha256": hashes["agent/bin/VitAgent.exe"],
        "frozen_file_sha256": hashes,
        "recognizer_control_composite_sha256": source_composite(hashes),
        "identity_only_preflight": resolutions,
        "parameter_surfaces_read": 0,
        "sealed_reserve_case_count": len(reserve),
        "sealed_reserve_case_ids": [matrix.first_text(case, "id") for case in reserve],
        "sealed_reserve_parameters_read": 0,
        "policy": manifest.get("policy"),
    }
    write_json(output, record)
    print(f"freeze: {output}")
    print(f"manifest_sha256={record['manifest_sha256']}")
    print(f"agent_sha256={record['agent_sha256']}")
    print(f"composite_sha256={record['recognizer_control_composite_sha256']}")
    return record


def verify_freeze(freeze_record: dict[str, Any], manifest_path: Path) -> dict[str, Any]:
    expected_files = freeze_record.get("frozen_file_sha256") or {}
    actual_files = file_hashes()
    mismatches = []
    for relative in sorted(set(expected_files) | set(actual_files)):
        if expected_files.get(relative) != actual_files.get(relative):
            mismatches.append({"path": relative, "expected": expected_files.get(relative),
                               "actual": actual_files.get(relative)})
    checks = {
        "manifest_sha256_expected": freeze_record.get("manifest_sha256"),
        "manifest_sha256_actual": sha256_file(manifest_path),
        "source_reserve_manifest_sha256_expected": freeze_record.get("source_reserve_manifest_sha256"),
        "source_reserve_manifest_sha256_actual": sha256_file(SOURCE_RESERVE_MANIFEST),
        "runner_sha256_expected": freeze_record.get("runner_sha256"),
        "runner_sha256_actual": sha256_file(SCRIPT_PATH),
        "agent_sha256_expected": freeze_record.get("agent_sha256"),
        "agent_sha256_actual": actual_files.get("agent/bin/VitAgent.exe"),
        "composite_sha256_expected": freeze_record.get("recognizer_control_composite_sha256"),
        "composite_sha256_actual": source_composite(actual_files),
        "file_mismatches": mismatches,
    }
    checks["intact"] = (
        checks["manifest_sha256_expected"] == checks["manifest_sha256_actual"]
        and checks["source_reserve_manifest_sha256_expected"] == checks["source_reserve_manifest_sha256_actual"]
        and checks["runner_sha256_expected"] == checks["runner_sha256_actual"]
        and checks["agent_sha256_expected"] == checks["agent_sha256_actual"]
        and checks["composite_sha256_expected"] == checks["composite_sha256_actual"]
        and not mismatches
    )
    return checks


def response_succeeded(response: dict[str, Any]) -> bool:
    outer = matrix.first_text(response, "status").lower()
    inner = matrix.first_text(matrix.result_of(response), "status").lower()
    return outer in SUCCESS_STATUSES and inner not in {"error", "failed", "rejected"}


def rejection_code(response: dict[str, Any]) -> str:
    result = matrix.result_of(response)
    code = matrix.first_text(result, "rejection_code", "code")
    if code:
        return code
    error = matrix.first_text(response, "error")
    for candidate in REJECTION_CODES:
        if candidate in error:
            return candidate
    return ""


def snapshot(base: str, track_id: str, plugin_id: str, identifier: str,
             timeout: float, evidence: dict[str, Any], evidence_key: str) -> dict[str, float]:
    response = matrix.invoke(base, "plugin.get_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": identifier,
        "include_parameters": True,
    }, timeout)
    matrix.require_ok(response, "plugin.get_parameters")
    evidence[evidence_key] = response
    return apply_smoke.parameter_snapshot(matrix.result_of(response))


def snapshot_mismatches(before: dict[str, float], after: dict[str, float],
                        tolerance: float) -> list[dict[str, Any]]:
    mismatches: list[dict[str, Any]] = []
    for param_id in sorted(set(before) | set(after)):
        left, right = before.get(param_id), after.get(param_id)
        if left is None or right is None or abs(left - right) > tolerance:
            mismatches.append({"param_id": param_id, "before": left, "after": right})
    return mismatches


def run_positive_flow(base: str, track_id: str, plugin_id: str, inspect: dict[str, Any],
                      before: dict[str, float], policy: dict[str, Any], identifier: str,
                      timeout: float, evidence: dict[str, Any]) -> dict[str, Any]:
    inspect_summary = matrix.validate_inspect(inspect)
    controls, roles = regression_v11.select_controls(inspect, policy)
    evidence["selected_controls"] = controls
    evidence["selected_roles"] = roles
    applied_response = matrix.invoke(base, "plugin_grabber.apply_compressor_controls", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "controls": controls,
    }, timeout, True)
    evidence["apply_response"] = applied_response
    applied = matrix.require_ok(applied_response, "typed apply")
    applied_controls = [row for row in applied.get("controls") or [] if isinstance(row, dict)]
    if len(applied_controls) != len(controls) or not all(row.get("actual_readback") for row in applied_controls):
        raise RuntimeError("typed apply omitted requested control readback")
    restore_ref = matrix.first_text(applied, "restore_ref")
    if not restore_ref:
        raise RuntimeError("typed apply omitted exact normalized restore_ref")
    evidence["restore_ref"] = restore_ref
    restored_response = matrix.invoke(base, "plugin_grabber.apply_compressor_controls", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "restore_ref": restore_ref,
    }, timeout, True)
    evidence["restore_response"] = restored_response
    restored = matrix.require_ok(restored_response, "typed restore")
    after = snapshot(base, track_id, plugin_id, identifier, timeout, evidence,
                     "after_parameters_response")
    mismatches = snapshot_mismatches(before, after, float(policy.get("snapshot_tolerance") or 0.0001))
    evidence["snapshot_mismatches"] = mismatches
    if mismatches:
        raise RuntimeError(f"complete snapshot restore failed for {len(mismatches)} parameters")
    return {
        **inspect_summary,
        "status": "passed",
        "topology_generation_stable": True,
        "apply_status": applied.get("status"),
        "restore_status": restored.get("status"),
        "write_count": len(applied.get("writes") or []),
        "full_snapshot_restored": True,
    }


def run_rejection_flow(base: str, track_id: str, plugin_id: str, response: dict[str, Any],
                       before: dict[str, float], identifier: str, policy: dict[str, Any],
                       timeout: float, evidence: dict[str, Any]) -> dict[str, Any]:
    code = rejection_code(response)
    allowed = set(policy.get("allowed_rejection_codes") or REJECTION_CODES)
    if code not in allowed:
        raise RuntimeError(f"inspect rejection={code!r} is outside v2 boundary set")
    after = snapshot(base, track_id, plugin_id, identifier, timeout, evidence,
                     "after_parameters_response")
    mismatches = snapshot_mismatches(before, after, float(policy.get("snapshot_tolerance") or 0.0001))
    evidence["snapshot_mismatches"] = mismatches
    if mismatches:
        raise RuntimeError(f"rejected inspect changed {len(mismatches)} parameters")
    return {
        "status": "passed",
        "branch": "safe_rejection",
        "rejection_code": code,
        "parameters_unchanged": True,
        "write_count": 0,
    }


def run_case(base: str, case: dict[str, Any], resolution: dict[str, Any],
             policy: dict[str, Any], timeout: float, case_dir: Path) -> dict[str, Any]:
    case_id = matrix.first_text(case, "id")
    expectation = matrix.first_text(case, "expectation")
    identifier = matrix.first_text(resolution, "plugin_identifier")
    evidence: dict[str, Any] = {
        "schema_version": "plugin_grabber.compressor_blind_case_evidence.v2",
        "case": case,
        "frozen_resolution": resolution,
        "started_at": blind_v1.utc_now(),
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
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {
            "name": f"{TRACK_PREFIX} {case_id}"}, timeout, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        evidence["plugin_load_response"] = matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "plugin_identifier": identifier,
        }, timeout, True)
        loaded = matrix.require_ok(evidence["plugin_load_response"], "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        evidence["plugin_id"] = plugin_id
        time.sleep(0.35)
        before = snapshot(base, track_id, plugin_id, identifier, timeout, evidence,
                          "before_parameters_response")
        evidence["before_parameter_count"] = len(before)
        evidence["inspect_response_1"] = matrix.invoke(base, "plugin_grabber.inspect_compressor", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout)
        accepted = response_succeeded(evidence["inspect_response_1"])
        if expectation == "compressor":
            if not accepted:
                raise RuntimeError(f"positive inspect rejected: {rejection_code(evidence['inspect_response_1'])}")
            inspect_1 = matrix.require_ok(evidence["inspect_response_1"], "inspect first pass")
            matrix.validate_inspect(inspect_1)
            evidence["inspect_response_2"] = matrix.invoke(base, "plugin_grabber.inspect_compressor", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout)
            inspect_2 = matrix.require_ok(evidence["inspect_response_2"], "inspect second pass")
            if matrix.first_text(inspect_1.get("control_topology") or {}, "generation") != matrix.first_text(inspect_2.get("control_topology") or {}, "generation"):
                raise RuntimeError("topology generation drifted between inspect passes")
            result.update(run_positive_flow(base, track_id, plugin_id, inspect_2, before, policy, identifier, timeout, evidence))
        elif expectation == "not_compressor":
            if accepted:
                raise RuntimeError("strict negative was recognized as compressor")
            result.update(run_rejection_flow(base, track_id, plugin_id, evidence["inspect_response_1"], before, identifier, policy, timeout, evidence))
        elif expectation == "hybrid_boundary":
            if accepted:
                inspect_1 = matrix.require_ok(evidence["inspect_response_1"], "hybrid inspect")
                matrix.validate_inspect(inspect_1)
                evidence["inspect_response_2"] = matrix.invoke(base, "plugin_grabber.inspect_compressor", {
                    "track_id": track_id, "plugin_id": plugin_id}, timeout)
                inspect_2 = matrix.require_ok(evidence["inspect_response_2"], "hybrid second inspect")
                if matrix.first_text(inspect_1.get("control_topology") or {}, "generation") != matrix.first_text(inspect_2.get("control_topology") or {}, "generation"):
                    raise RuntimeError("hybrid topology generation drifted")
                hybrid_policy = dict(policy)
                hybrid_policy["max_apply_controls_per_positive"] = int(
                    policy.get("max_apply_controls_per_hybrid") or
                    policy.get("max_apply_controls_per_positive") or 1)
                result.update(run_positive_flow(base, track_id, plugin_id, inspect_2, before, hybrid_policy, identifier, timeout, evidence))
                result["branch"] = "accepted_broadband_stage"
            else:
                result.update(run_rejection_flow(base, track_id, plugin_id, evidence["inspect_response_1"], before, identifier, policy, timeout, evidence))
        else:
            raise RuntimeError(f"unknown v2 expectation {expectation!r}")
        result["status"] = "passed"
    except Exception as error:
        result["status"] = "failed"
        result["error"] = str(error)
        evidence["exception"] = {
            "type": type(error).__name__,
            "message": str(error),
            "traceback": traceback.format_exc(),
        }
    finally:
        if track_id:
            try:
                evidence["track_delete_response"] = matrix.invoke(base, "track.delete", {
                    "track_id": track_id}, timeout, True)
                matrix.require_ok(evidence["track_delete_response"], "track.delete")
                result["temporary_track_deleted"] = True
            except Exception as error:
                result["status"] = "failed"
                result["cleanup_error"] = str(error)
        evidence["completed_at"] = blind_v1.utc_now()
        evidence["result"] = result
        blind_v1.write_json(case_dir / "evidence.json", evidence)
    return result


def track_cleanup_audit(base: str, timeout: float) -> dict[str, Any]:
    response = matrix.invoke(base, "track.list", {}, timeout)
    result = matrix.result_of(response)
    tracks: list[dict[str, Any]] = []
    for key in ("tracks", "items", "entries"):
        if isinstance(result.get(key), list):
            tracks = [row for row in result[key] if isinstance(row, dict)]
            break
    leftovers = [row for row in tracks if matrix.first_text(row, "name", "track_name").startswith(TRACK_PREFIX)]
    return {"clean": not leftovers, "leftover_tracks": leftovers, "track_count": len(tracks)}


def execute(manifest_path: Path, freeze_path: Path, output_dir: Path,
            base: str, timeout: float) -> dict[str, Any]:
    manifest = load_manifest(manifest_path)
    freeze_record = json.loads(freeze_path.read_text(encoding="utf-8"))
    if output_dir.exists() and any(output_dir.iterdir()):
        raise RuntimeError(f"v2 output directory must be empty: {output_dir}")
    output_dir.mkdir(parents=True, exist_ok=True)
    health = health_check(base)
    before = verify_freeze(freeze_record, manifest_path)
    blind_v1.write_json(output_dir / "freeze_verification_before.json", before)
    if not before["intact"]:
        raise RuntimeError("v2 freeze verification failed before execution")
    resolutions = {matrix.first_text(row, "id"): row for row in freeze_record.get("identity_only_preflight") or [] if isinstance(row, dict)}
    cases = [case for case in manifest.get("cases") or [] if isinstance(case, dict)]
    results: list[dict[str, Any]] = []
    for index, case in enumerate(cases, 1):
        case_id = matrix.first_text(case, "id")
        print(f"[{index}/{len(cases)}] {case_id}: expected {matrix.first_text(case, 'expectation')}", flush=True)
        resolution = resolutions.get(case_id)
        if resolution is None:
            result = {"id": case_id, "status": "failed", "error": "missing frozen identity", "temporary_track_deleted": True}
        else:
            result = run_case(base, case, resolution, manifest.get("policy") or {}, timeout, output_dir / "cases" / f"{index:02d}_{case_id}")
        results.append(result)
        print(f"  {result.get('status')} branch={result.get('branch', '')} error={result.get('error', '')}", flush=True)
    cleanup = track_cleanup_audit(base, timeout)
    after = verify_freeze(freeze_record, manifest_path)
    blind_v1.write_json(output_dir / "freeze_verification_after.json", after)
    passed = [row for row in results if row.get("status") == "passed"]
    failed = [row for row in results if row.get("status") != "passed"]
    positives = [row for row in results if row.get("expectation") == "compressor"]
    negatives = [row for row in results if row.get("expectation") == "not_compressor"]
    hybrids = [row for row in results if row.get("expectation") == "hybrid_boundary"]
    summary = {
        "schema_version": "plugin_grabber.compressor_plugin_alliance_blind_report.v2",
        "set_id": matrix.first_text(manifest, "set_id"),
        "started_from_freeze": str(freeze_path.resolve()),
        "completed_at": utc_now(),
        "run_complete": len(results) == len(cases),
        "verdict": "passed" if not failed and cleanup["clean"] and after["intact"] else "failed",
        "blind_claim": True,
        "training_rule_feedback": "forbidden",
        "agent_health": health,
        "agent_sha256": freeze_record.get("agent_sha256"),
        "manifest_sha256": freeze_record.get("manifest_sha256"),
        "recognizer_control_composite_sha256": freeze_record.get("recognizer_control_composite_sha256"),
        "freeze_intact_before": before["intact"],
        "freeze_intact_after": after["intact"],
        "sealed_reserve_parameters_read": 0,
        "counts": {
            "total": len(results),
            "passed": len(passed),
            "failed": len(failed),
            "positive_total": len(positives),
            "positive_passed": len([row for row in positives if row.get("status") == "passed"]),
            "negative_total": len(negatives),
            "negative_passed": len([row for row in negatives if row.get("status") == "passed"]),
            "hybrid_total": len(hybrids),
            "hybrid_passed": len([row for row in hybrids if row.get("status") == "passed"]),
        },
        "cleanup_audit": cleanup,
        "results": results,
    }
    blind_v1.write_json(output_dir / "summary.json", summary)
    print(f"report: {output_dir / 'summary.json'}")
    print(json.dumps({"verdict": summary["verdict"], "counts": summary["counts"]}, ensure_ascii=False))
    return summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", default=str(DEFAULT_MANIFEST))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--freeze-output")
    mode.add_argument("--execute-freeze")
    parser.add_argument("--output-dir")
    args = parser.parse_args()
    manifest_path = Path(args.manifest).resolve()
    if args.freeze_output:
        freeze(manifest_path, Path(args.freeze_output).resolve(), args.agent_http, args.timeout_sec)
        return 0
    if not args.output_dir:
        parser.error("--output-dir is required with --execute-freeze")
    summary = execute(manifest_path, Path(args.execute_freeze).resolve(), Path(args.output_dir).resolve(), args.agent_http, args.timeout_sec)
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
