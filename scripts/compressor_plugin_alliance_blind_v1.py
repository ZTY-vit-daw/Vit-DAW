#!/usr/bin/env python3
"""Freeze and execute the Plugin Alliance compressor-control v1 blind set."""
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


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("compressor_plugin_alliance_blind_v1.json")
FROZEN_FILES = (
    "agent/bin/VitAgent.exe",
    "agent/internal/workflows/plugingrabber/compressor_topology.go",
    "agent/internal/workflows/plugingrabber/display_domain.go",
    "agent/internal/workflows/plugingrabber/display_probe.go",
    "agent/internal/workflows/plugingrabber/observation.go",
    "agent/internal/workflows/plugingrabber/types.go",
    "agent/internal/chat/plugin_compressor_inspect.go",
    "agent/internal/chat/plugin_compressor_apply.go",
    "agent/internal/chat/plugin_eq_edits.go",
    "agent/internal/chat/server.go",
    "agent/internal/harness/plugin_param_guard.go",
    "scripts/compressor_apply_smoke.py",
    "scripts/compressor_compat_matrix_smoke.py",
    "scripts/compressor_compat_matrix.json",
)
TRACK_PREFIX = "PA blind v1"


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
    positives = [case for case in cases if case.get("expectation") == "compressor"]
    negatives = [case for case in cases if case.get("expectation") == "not_compressor"]
    policy = manifest.get("policy") or {}
    if len(cases) != 18 or len(positives) != 12 or len(negatives) != 6:
        raise RuntimeError(
            f"blind manifest composition is {len(positives)} positive / "
            f"{len(negatives)} negative, expected 12 / 6")
    if int(policy.get("sample_count") or 0) != len(cases):
        raise RuntimeError("blind manifest policy sample_count does not match cases")
    if any("bx_townhouse" in matrix.first_text(case, "plugin_name").casefold()
           for case in cases):
        raise RuntimeError("pilot bx_townhouse must not be scored in the blind set")
    return manifest


def health_check(base: str) -> dict[str, Any]:
    health = matrix.request_json("GET", base.rstrip("/") + "/health", None, 10)
    if matrix.first_text(health, "status").lower() not in {"ok", "ready"}:
        raise RuntimeError(f"agent health check failed: {health}")
    return health


def identity_only_preflight(manifest: dict[str, Any], timeout: float) -> list[dict[str, Any]]:
    resolutions: list[dict[str, Any]] = []
    for index, case in enumerate(manifest.get("cases") or [], 1):
        case_id = matrix.first_text(case, "id")
        plugin_path = Path(matrix.first_text(case, "plugin_path"))
        if not plugin_path.exists():
            raise RuntimeError(f"{case_id}: plugin file is missing: {plugin_path}")
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        resolved_path = matrix.first_text(
            resolution, "file_or_identifier", "plugin_path", "path")
        if not matrix.path_matches(str(plugin_path), resolved_path):
            raise RuntimeError(
                f"{case_id}: resolved path {resolved_path!r} does not match {plugin_path}")
        resolutions.append({
            "index": index,
            "id": case_id,
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "expectation": matrix.first_text(case, "expectation"),
            "plugin_identifier": identifier,
            "resolved_path": resolved_path,
        })
        print(f"[{index}/18] preflight {case_id}: unique identity", flush=True)
    return resolutions


def freeze(manifest_path: Path, output: Path, base: str, timeout: float) -> dict[str, Any]:
    manifest = load_manifest(manifest_path)
    health = health_check(base)
    resolutions = identity_only_preflight(manifest, timeout)
    hashes = file_hashes()
    record = {
        "schema_version": "plugin_grabber.compressor_blind_freeze.v1",
        "frozen_at": utc_now(),
        "set_id": matrix.first_text(manifest, "set_id"),
        "manifest_path": str(manifest_path.resolve()),
        "manifest_sha256": sha256_file(manifest_path),
        "runner_path": str(SCRIPT_PATH),
        "runner_sha256": sha256_file(SCRIPT_PATH),
        "agent_http": base.rstrip("/"),
        "agent_health": health,
        "agent_sha256": hashes["agent/bin/VitAgent.exe"],
        "frozen_file_sha256": hashes,
        "recognizer_control_composite_sha256": source_composite(hashes),
        "documented_training_freeze_sha256":
            "9b0a6dc28e306b5cdf6708a60edb1489e58a94c44032a22d9a8ff0e59120c830",
        "identity_only_preflight": resolutions,
        "parameter_surfaces_read": 0,
        "sealed_reserve_case_count": len(manifest.get("sealed_reserve_cases") or []),
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
            mismatches.append({
                "path": relative,
                "expected": expected_files.get(relative),
                "actual": actual_files.get(relative),
            })
    checks = {
        "manifest_sha256_expected": freeze_record.get("manifest_sha256"),
        "manifest_sha256_actual": sha256_file(manifest_path),
        "runner_sha256_expected": freeze_record.get("runner_sha256"),
        "runner_sha256_actual": sha256_file(SCRIPT_PATH),
        "agent_sha256_expected": freeze_record.get("agent_sha256"),
        "agent_sha256_actual": actual_files.get("agent/bin/VitAgent.exe"),
        "composite_sha256_expected": freeze_record.get(
            "recognizer_control_composite_sha256"),
        "composite_sha256_actual": source_composite(actual_files),
        "file_mismatches": mismatches,
    }
    checks["intact"] = (
        checks["manifest_sha256_expected"] == checks["manifest_sha256_actual"]
        and checks["runner_sha256_expected"] == checks["runner_sha256_actual"]
        and checks["agent_sha256_expected"] == checks["agent_sha256_actual"]
        and checks["composite_sha256_expected"] == checks["composite_sha256_actual"]
        and not mismatches
    )
    return checks


def result_of(response: dict[str, Any]) -> dict[str, Any]:
    return matrix.result_of(response)


def get_parameters(base: str, track_id: str, plugin_id: str, identifier: str,
                   timeout: float) -> dict[str, Any]:
    response = matrix.invoke(base, "plugin.get_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": identifier,
        "include_parameters": True,
    }, timeout)
    matrix.require_ok(response, "plugin.get_parameters")
    return response


def snapshot_from_response(response: dict[str, Any]) -> dict[str, float]:
    return apply_smoke.parameter_snapshot(result_of(response))


def snapshot_mismatches(before: dict[str, float], after: dict[str, float],
                        tolerance: float) -> list[dict[str, Any]]:
    mismatches: list[dict[str, Any]] = []
    for param_id in sorted(set(before) | set(after)):
        left, right = before.get(param_id), after.get(param_id)
        if left is None or right is None or abs(left - right) > tolerance:
            mismatches.append({"param_id": param_id, "before": left, "after": right})
    return mismatches


def control_pair(binding: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any]]:
    role = matrix.first_text(binding, "role")
    control_ref = matrix.first_text(binding, "control_ref")
    if not control_ref:
        raise RuntimeError(f"{role} binding omitted control_ref")
    domain = binding.get("domain")
    unit = matrix.first_text(domain, "unit").lower() if isinstance(domain, dict) else ""
    if unit in {"enum", "toggle"}:
        current_label = matrix.first_text(binding, "current_text")
        labels = [matrix.first_text(row, "label")
                  for row in binding.get("reachable_values") or []
                  if isinstance(row, dict)]
        alternate = next((label for label in labels
                          if label and label.casefold() != current_label.casefold()), "")
        if not current_label or not alternate:
            raise RuntimeError(f"{role} has no reversible enum target")
        return (
            {"control_ref": control_ref, "enum_label": alternate},
            {"control_ref": control_ref, "enum_label": current_label},
        )
    current = binding.get("current_physical")
    if not isinstance(current, (int, float)) or not math.isfinite(float(current)):
        raise RuntimeError(f"{role} omitted finite current_physical")
    current_value = float(current)
    alternate = apply_smoke.alternate_physical(binding, current_value)
    return (
        {"control_ref": control_ref, "display_value": alternate},
        {"control_ref": control_ref, "display_value": current_value},
    )


def select_controls(inspect: dict[str, Any], policy: dict[str, Any]) -> tuple[
        list[dict[str, Any]], list[dict[str, Any]], list[str]]:
    bindings = apply_smoke.compressor_bindings(inspect)
    priorities = [str(role) for role in policy.get("apply_role_priority") or []]
    max_controls = int(policy.get("max_apply_controls_per_positive") or 1)
    forward: list[dict[str, Any]] = []
    restore: list[dict[str, Any]] = []
    roles: list[str] = []
    used_params: set[str] = set()
    for role in priorities:
        for binding in bindings:
            if matrix.first_text(binding, "role") != role:
                continue
            param_id = matrix.first_text(binding, "param_id")
            if not param_id or param_id in used_params:
                continue
            try:
                next_control, prior_control = control_pair(binding)
            except RuntimeError:
                continue
            forward.append(next_control)
            restore.append(prior_control)
            roles.append(role)
            used_params.add(param_id)
            break
        if len(forward) >= max_controls:
            break
    if not forward:
        raise RuntimeError("no preferred compressor binding had a reversible physical target")
    return forward, restore, roles


def inspect_generation(inspect: dict[str, Any]) -> str:
    topology = inspect.get("control_topology")
    return matrix.first_text(topology, "generation") if isinstance(topology, dict) else ""


def rejection_code(response: dict[str, Any]) -> str:
    result = result_of(response)
    code = matrix.first_text(result, "rejection_code", "code")
    if code:
        return code
    error = matrix.first_text(response, "error")
    return "not_compressor" if "not_compressor" in error else ""


def run_case(base: str, case: dict[str, Any], resolution: dict[str, Any],
             policy: dict[str, Any], timeout: float, case_dir: Path) -> dict[str, Any]:
    case_id = matrix.first_text(case, "id")
    expectation = matrix.first_text(case, "expectation")
    identifier = matrix.first_text(resolution, "plugin_identifier")
    tolerance = float(policy.get("snapshot_tolerance") or 0.0001)
    evidence: dict[str, Any] = {
        "schema_version": "plugin_grabber.compressor_blind_case_evidence.v1",
        "case": case,
        "frozen_resolution": resolution,
        "started_at": utc_now(),
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
        result["track_id"] = track_id
        evidence["plugin_load_response"] = matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "plugin_identifier": identifier,
        }, timeout, True)
        loaded = matrix.require_ok(evidence["plugin_load_response"], "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        result["plugin_id"] = plugin_id
        time.sleep(0.35)
        evidence["before_parameters_response"] = get_parameters(
            base, track_id, plugin_id, identifier, timeout)
        before = snapshot_from_response(evidence["before_parameters_response"])
        result["parameter_count"] = len(before)

        evidence["inspect_response_1"] = matrix.invoke(
            base, "plugin_grabber.inspect_compressor", {
                "track_id": track_id, "plugin_id": plugin_id}, timeout)
        if expectation == "compressor":
            inspect_1 = matrix.require_ok(
                evidence["inspect_response_1"], "inspect_compressor first pass")
            inspect_summary = matrix.validate_inspect(inspect_1)
            evidence["inspect_response_2"] = matrix.invoke(
                base, "plugin_grabber.inspect_compressor", {
                    "track_id": track_id, "plugin_id": plugin_id}, timeout)
            inspect_2 = matrix.require_ok(
                evidence["inspect_response_2"], "inspect_compressor second pass")
            matrix.validate_inspect(inspect_2)
            generation_1 = inspect_generation(inspect_1)
            generation_2 = inspect_generation(inspect_2)
            if not generation_1 or generation_1 != generation_2:
                raise RuntimeError(
                    f"topology generation drifted: {generation_1!r} -> {generation_2!r}")
            forward, restore, roles = select_controls(inspect_2, policy)
            evidence["selected_forward_controls"] = forward
            evidence["selected_restore_controls"] = restore
            evidence["apply_response"] = matrix.invoke(
                base, "plugin_grabber.apply_compressor_controls", {
                    "track_id": track_id,
                    "plugin_id": plugin_id,
                    "atomic": True,
                    "controls": forward,
                }, timeout, True)
            applied = matrix.require_ok(evidence["apply_response"], "typed apply")
            controls = [row for row in applied.get("controls") or []
                        if isinstance(row, dict)]
            if len(controls) != len(forward) or not all(
                    row.get("actual_readback") for row in controls):
                raise RuntimeError("typed apply omitted requested control readback")
            evidence["restore_response"] = matrix.invoke(
                base, "plugin_grabber.apply_compressor_controls", {
                    "track_id": track_id,
                    "plugin_id": plugin_id,
                    "atomic": True,
                    "controls": restore,
                }, timeout, True)
            restored = matrix.require_ok(evidence["restore_response"], "typed restore")
            evidence["after_parameters_response"] = get_parameters(
                base, track_id, plugin_id, identifier, timeout)
            after = snapshot_from_response(evidence["after_parameters_response"])
            mismatches = snapshot_mismatches(before, after, tolerance)
            evidence["snapshot_mismatches"] = mismatches
            if mismatches:
                raise RuntimeError(
                    f"complete snapshot restore failed for {len(mismatches)} parameters")
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
            code = rejection_code(evidence["inspect_response_1"])
            if code != "not_compressor":
                raise RuntimeError(
                    f"negative inspect rejection={code!r}, expected 'not_compressor'")
            evidence["after_parameters_response"] = get_parameters(
                base, track_id, plugin_id, identifier, timeout)
            after = snapshot_from_response(evidence["after_parameters_response"])
            mismatches = snapshot_mismatches(before, after, tolerance)
            evidence["snapshot_mismatches"] = mismatches
            if mismatches:
                raise RuntimeError(
                    f"negative inspect changed {len(mismatches)} parameters")
            result.update({
                "status": "passed",
                "rejection_code": code,
                "parameters_unchanged": True,
                "write_count": 0,
            })
    except Exception as error:  # Every failure is evidence; the cohort must continue.
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
                evidence["cleanup_exception"] = {
                    "type": type(cleanup_error).__name__,
                    "message": str(cleanup_error),
                    "traceback": traceback.format_exc(),
                }
        evidence["completed_at"] = utc_now()
        evidence["result"] = result
        write_json(case_dir / "evidence.json", evidence)
    return result


def track_cleanup_audit(base: str, timeout: float) -> dict[str, Any]:
    response = matrix.invoke(base, "track.list", {}, timeout)
    tracks: list[dict[str, Any]] = []
    result = result_of(response)
    for key in ("tracks", "items", "entries"):
        if isinstance(result.get(key), list):
            tracks = [row for row in result[key] if isinstance(row, dict)]
            break
    leftovers = [row for row in tracks
                 if matrix.first_text(row, "name", "track_name").startswith(TRACK_PREFIX)]
    return {
        "response_status": response.get("status"),
        "track_count": len(tracks),
        "leftover_blind_tracks": leftovers,
        "clean": not leftovers,
    }


def execute(manifest_path: Path, freeze_path: Path, output_dir: Path,
            base: str, timeout: float) -> dict[str, Any]:
    manifest = load_manifest(manifest_path)
    freeze_record = json.loads(freeze_path.read_text(encoding="utf-8"))
    if output_dir.exists() and any(output_dir.iterdir()):
        raise RuntimeError(f"blind output directory must be empty: {output_dir}")
    output_dir.mkdir(parents=True, exist_ok=True)
    health = health_check(base)
    before_verification = verify_freeze(freeze_record, manifest_path)
    write_json(output_dir / "freeze_verification_before.json", before_verification)
    if not before_verification["intact"]:
        raise RuntimeError("blind freeze verification failed before execution")
    resolutions = {
        matrix.first_text(row, "id"): row
        for row in freeze_record.get("identity_only_preflight") or []
        if isinstance(row, dict)
    }
    results: list[dict[str, Any]] = []
    cases = [case for case in manifest.get("cases") or [] if isinstance(case, dict)]
    for index, case in enumerate(cases, 1):
        case_id = matrix.first_text(case, "id")
        expectation = matrix.first_text(case, "expectation")
        print(f"[{index}/18] {case_id}: expected {expectation}", flush=True)
        resolution = resolutions.get(case_id)
        if resolution is None:
            result = {
                "id": case_id,
                "plugin_name": matrix.first_text(case, "plugin_name"),
                "expectation": expectation,
                "status": "failed",
                "error": "frozen identity resolution is missing",
                "temporary_track_deleted": True,
            }
        else:
            result = run_case(
                base, case, resolution, manifest.get("policy") or {}, timeout,
                output_dir / "cases" / f"{index:02d}_{case_id}")
        results.append(result)
        print(
            f"  {result['status']}"
            + (f" classification={result.get('classification')}" if result.get("classification") else "")
            + (f" error={result.get('error')}" if result.get("error") else ""),
            flush=True,
        )
    cleanup = track_cleanup_audit(base, timeout)
    after_verification = verify_freeze(freeze_record, manifest_path)
    write_json(output_dir / "freeze_verification_after.json", after_verification)
    passed = [row for row in results if row.get("status") == "passed"]
    failed = [row for row in results if row.get("status") != "passed"]
    positive = [row for row in results if row.get("expectation") == "compressor"]
    negative = [row for row in results if row.get("expectation") == "not_compressor"]
    summary = {
        "schema_version": "plugin_grabber.compressor_plugin_alliance_blind_report.v1",
        "set_id": matrix.first_text(manifest, "set_id"),
        "started_from_freeze": str(freeze_path.resolve()),
        "completed_at": utc_now(),
        "run_complete": len(results) == len(cases),
        "verdict": "passed" if not failed and cleanup["clean"] and after_verification["intact"] else "failed",
        "training_rule_feedback": "forbidden",
        "agent_health": health,
        "agent_sha256": freeze_record.get("agent_sha256"),
        "manifest_sha256": freeze_record.get("manifest_sha256"),
        "recognizer_control_composite_sha256": freeze_record.get(
            "recognizer_control_composite_sha256"),
        "freeze_intact_before": before_verification["intact"],
        "freeze_intact_after": after_verification["intact"],
        "sealed_reserve_parameters_read": 0,
        "counts": {
            "total": len(results),
            "passed": len(passed),
            "failed": len(failed),
            "positive_total": len(positive),
            "positive_passed": len([row for row in positive if row.get("status") == "passed"]),
            "negative_total": len(negative),
            "negative_passed": len([row for row in negative if row.get("status") == "passed"]),
        },
        "cleanup_audit": cleanup,
        "results": results,
    }
    write_json(output_dir / "summary.json", summary)
    print(f"report: {output_dir / 'summary.json'}")
    print(json.dumps({"verdict": summary["verdict"], "counts": summary["counts"]},
                     ensure_ascii=False))
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
        freeze(manifest_path, Path(args.freeze_output).resolve(),
               args.agent_http, args.timeout_sec)
        return 0
    if not args.output_dir:
        parser.error("--output-dir is required with --execute-freeze")
    summary = execute(
        manifest_path, Path(args.execute_freeze).resolve(),
        Path(args.output_dir).resolve(), args.agent_http, args.timeout_sec)
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
