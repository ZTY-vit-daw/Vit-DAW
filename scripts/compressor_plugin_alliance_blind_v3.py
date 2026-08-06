#!/usr/bin/env python3
"""Freeze and execute the final four-case Plugin Alliance compressor blind set."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix
import compressor_plugin_alliance_blind_v1 as blind_v1
import compressor_plugin_alliance_blind_v2 as blind_v2


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("compressor_plugin_alliance_blind_v3.json")
TRACK_PREFIX = "PA blind v3"
FROZEN_FILES = tuple(dict.fromkeys(blind_v2.FROZEN_FILES + (
    "scripts/compressor_plugin_alliance_blind_v2.py",
    "scripts/compressor_plugin_alliance_blind_v3.py",
)))


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    cases = [row for row in manifest.get("cases") or [] if isinstance(row, dict)]
    policy = manifest.get("policy") or {}
    positives = [row for row in cases if row.get("expectation") == "compressor"]
    negatives = [row for row in cases if row.get("expectation") == "not_compressor"]
    if len(cases) != 4 or len(positives) != 1 or len(negatives) != 3:
        raise RuntimeError(
            f"v3 composition is {len(positives)} positive / {len(negatives)} negative, expected 1 / 3")
    if int(policy.get("sample_count") or 0) != 4 or int(policy.get("strict_pass_count") or 0) != 4:
        raise RuntimeError("v3 policy must require all four cases")
    expected_ids = {
        "shadow_hills_optomax", "hum_audio_devices_laal",
        "elysia_nvelope_negative", "lindell_354e_negative",
    }
    if {matrix.first_text(row, "id") for row in cases} != expected_ids:
        raise RuntimeError("v3 cases must be the exact remaining four-case reserve")
    allowed = set(policy.get("allowed_rejection_codes") or [])
    for case in cases:
        if not Path(matrix.first_text(case, "plugin_path")).is_file():
            raise RuntimeError(f"v3 plugin file is missing: {matrix.first_text(case, 'plugin_path')}")
        if case.get("expectation") == "not_compressor":
            expected = matrix.first_text(case, "expected_rejection")
            if not expected or expected not in allowed:
                raise RuntimeError(f"{matrix.first_text(case, 'id')} has invalid expected rejection {expected!r}")
    correction = (manifest.get("oracle_policy") or {}).get("legacy_correction") or {}
    if correction.get("case_id") != "hum_audio_devices_laal" or correction.get("expected_rejection") != "unsupported_limiter":
        raise RuntimeError("v3 must freeze the documented LAAL limiter oracle correction")
    return manifest


def file_hashes() -> dict[str, str]:
    hashes: dict[str, str] = {}
    for relative in FROZEN_FILES:
        path = REPO_ROOT / relative
        if not path.is_file():
            raise RuntimeError(f"frozen file is missing: {path}")
        hashes[relative] = blind_v1.sha256_file(path)
    return hashes


def source_composite(hashes: dict[str, str]) -> str:
    rows = [f"{path}\0{digest}\n" for path, digest in sorted(hashes.items())]
    return hashlib.sha256("".join(rows).encode("utf-8")).hexdigest()


def identity_only_preflight(manifest: dict[str, Any], timeout: float) -> list[dict[str, Any]]:
    cases = [row for row in manifest.get("cases") or [] if isinstance(row, dict)]
    resolutions: list[dict[str, Any]] = []
    for index, case in enumerate(cases, 1):
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        configured = matrix.first_text(case, "plugin_path")
        resolved = matrix.first_text(resolution, "file_or_identifier", "plugin_path", "path")
        if not matrix.path_matches(configured, resolved):
            raise RuntimeError(f"{matrix.first_text(case, 'id')}: resolved path mismatch {resolved!r}")
        resolutions.append({
            "index": index,
            "id": matrix.first_text(case, "id"),
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "expectation": matrix.first_text(case, "expectation"),
            "expected_rejection": matrix.first_text(case, "expected_rejection"),
            "plugin_identifier": identifier,
            "resolved_path": resolved,
        })
        print(f"[{index}/4] preflight {matrix.first_text(case, 'id')}: unique identity", flush=True)
    return resolutions


def freeze(manifest_path: Path, output: Path, base: str, timeout: float) -> dict[str, Any]:
    if output.exists():
        raise RuntimeError(f"v3 freeze output already exists: {output}")
    manifest = load_manifest(manifest_path)
    health = blind_v2.health_check(base)
    resolutions = identity_only_preflight(manifest, timeout)
    hashes = file_hashes()
    record = {
        "schema_version": "plugin_grabber.compressor_blind_freeze.v3",
        "frozen_at": blind_v2.utc_now(),
        "set_id": matrix.first_text(manifest, "set_id"),
        "manifest_path": str(manifest_path.resolve()),
        "manifest_sha256": blind_v1.sha256_file(manifest_path),
        "runner_path": str(SCRIPT_PATH),
        "runner_sha256": blind_v1.sha256_file(SCRIPT_PATH),
        "agent_http": base.rstrip("/"),
        "agent_health": health,
        "agent_sha256": hashes["agent/bin/VitAgent.exe"],
        "frozen_file_sha256": hashes,
        "recognizer_control_composite_sha256": source_composite(hashes),
        "identity_only_preflight": resolutions,
        "parameter_surfaces_read": 0,
        "case_count": 4,
        "execution_count": 0,
        "oracle_policy": manifest.get("oracle_policy"),
        "policy": manifest.get("policy"),
    }
    blind_v1.write_json(output, record)
    print(f"freeze: {output}")
    print(f"manifest_sha256={record['manifest_sha256']}")
    print(f"agent_sha256={record['agent_sha256']}")
    print(f"composite_sha256={record['recognizer_control_composite_sha256']}")
    return record


def verify_freeze(record: dict[str, Any], manifest_path: Path) -> dict[str, Any]:
    actual_files = file_hashes()
    expected_files = record.get("frozen_file_sha256") or {}
    mismatches = [
        {"path": path, "expected": expected_files.get(path), "actual": actual_files.get(path)}
        for path in sorted(set(expected_files) | set(actual_files))
        if expected_files.get(path) != actual_files.get(path)
    ]
    checks = {
        "manifest_sha256_expected": record.get("manifest_sha256"),
        "manifest_sha256_actual": blind_v1.sha256_file(manifest_path),
        "runner_sha256_expected": record.get("runner_sha256"),
        "runner_sha256_actual": blind_v1.sha256_file(SCRIPT_PATH),
        "agent_sha256_expected": record.get("agent_sha256"),
        "agent_sha256_actual": actual_files.get("agent/bin/VitAgent.exe"),
        "composite_sha256_expected": record.get("recognizer_control_composite_sha256"),
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


def run_case(base: str, case: dict[str, Any], resolution: dict[str, Any],
             policy: dict[str, Any], timeout: float, case_dir: Path) -> dict[str, Any]:
    result = blind_v2.run_case(base, case, resolution, policy, timeout, case_dir)
    expected = matrix.first_text(case, "expected_rejection")
    if result.get("status") == "passed" and expected and result.get("rejection_code") != expected:
        result["status"] = "failed"
        result["error"] = f"rejection={result.get('rejection_code')!r}, expected {expected!r}"
        evidence_path = case_dir / "evidence.json"
        evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
        evidence["result"] = result
        evidence["strict_expected_rejection"] = expected
        blind_v1.write_json(evidence_path, evidence)
    return result


def execute(manifest_path: Path, freeze_path: Path, output: Path,
            base: str, timeout: float) -> dict[str, Any]:
    manifest = load_manifest(manifest_path)
    record = json.loads(freeze_path.read_text(encoding="utf-8"))
    if output.exists() and any(output.iterdir()):
        raise RuntimeError(f"v3 run output must be empty: {output}")
    output.mkdir(parents=True, exist_ok=True)
    before = verify_freeze(record, manifest_path)
    blind_v1.write_json(output / "freeze_verification_before.json", before)
    if not before["intact"]:
        raise RuntimeError("v3 freeze verification failed before execution")

    blind_v2.TRACK_PREFIX = TRACK_PREFIX
    blind_v2.REJECTION_CODES.update({
        "unsupported_limiter", "unsupported_multiband_compressor", "not_compressor",
    })
    health = blind_v2.health_check(base)
    resolutions = {
        matrix.first_text(row, "id"): row
        for row in record.get("identity_only_preflight") or [] if isinstance(row, dict)
    }
    cases = [row for row in manifest.get("cases") or [] if isinstance(row, dict)]
    results: list[dict[str, Any]] = []
    for index, case in enumerate(cases, 1):
        case_id = matrix.first_text(case, "id")
        print(f"[{index}/4] {case_id}: frozen expectation={matrix.first_text(case, 'expectation')}", flush=True)
        resolution = resolutions.get(case_id)
        if resolution is None:
            result = {"id": case_id, "status": "failed", "error": "missing frozen identity"}
        else:
            result = run_case(base, case, resolution, manifest.get("policy") or {}, timeout,
                              output / "cases" / f"{index:02d}_{case_id}")
        results.append(result)
        print(f"  {result.get('status')} error={result.get('error', '')}", flush=True)

    cleanup = blind_v2.track_cleanup_audit(base, timeout)
    after = verify_freeze(record, manifest_path)
    blind_v1.write_json(output / "freeze_verification_after.json", after)
    passed = [row for row in results if row.get("status") == "passed"]
    positives = [row for row in results if row.get("expectation") == "compressor"]
    negatives = [row for row in results if row.get("expectation") == "not_compressor"]
    summary = {
        "schema_version": "plugin_grabber.compressor_plugin_alliance_blind_report.v3",
        "set_id": matrix.first_text(manifest, "set_id"),
        "blind_claim": True,
        "run_complete": len(results) == 4,
        "execution_count": 1,
        "completed_at": blind_v2.utc_now(),
        "verdict": "passed" if len(passed) == 4 and cleanup["clean"] and after["intact"] else "failed",
        "agent_health": health,
        "agent_sha256": record.get("agent_sha256"),
        "manifest_sha256": record.get("manifest_sha256"),
        "recognizer_control_composite_sha256": record.get("recognizer_control_composite_sha256"),
        "freeze_intact_before": before["intact"],
        "freeze_intact_after": after["intact"],
        "parameter_surfaces_read_before_freeze": 0,
        "oracle_policy": manifest.get("oracle_policy"),
        "counts": {
            "total": 4,
            "passed": len(passed),
            "failed": 4 - len(passed),
            "positive_total": len(positives),
            "positive_passed": len([row for row in positives if row.get("status") == "passed"]),
            "negative_total": len(negatives),
            "negative_passed": len([row for row in negatives if row.get("status") == "passed"]),
        },
        "cleanup_audit": cleanup,
        "results": results,
    }
    blind_v1.write_json(output / "summary.json", summary)
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
    manifest = Path(args.manifest).resolve()
    if args.freeze_output:
        freeze(manifest, Path(args.freeze_output).resolve(), args.agent_http, args.timeout_sec)
        return 0
    if not args.output_dir:
        parser.error("--output-dir is required with --execute-freeze")
    summary = execute(manifest, Path(args.execute_freeze).resolve(),
                      Path(args.output_dir).resolve(), args.agent_http, args.timeout_sec)
    return 0 if summary["verdict"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
