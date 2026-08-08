#!/usr/bin/env python3
"""Freeze, then execute the two-case Limiter V1 sealed blind set exactly once."""
from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import math
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


SCRIPT_PATH = Path(__file__).resolve()
REPO_ROOT = SCRIPT_PATH.parent.parent
DEFAULT_MANIFEST = SCRIPT_PATH.with_name("limiter_control_matrix.json")
FROZEN_FILES = (
    "agent/internal/workflows/plugingrabber/limiter_topology.go",
    "agent/internal/chat/plugin_limiter_inspect.go",
    "agent/internal/chat/plugin_limiter_apply.go",
    "agent/internal/chat/plugin_eq_control.go",
    "agent/internal/chat/goalrunner_chat.go",
    "agent/internal/chat/server.go",
    "agent/internal/tools/catalog.go",
    "agent/internal/toolpolicy/plugin_policy.go",
    "agent/bin/VitAgent.exe",
    "scripts/limiter_control_matrix.json",
)


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def load_manifest(path: Path) -> dict[str, Any]:
    manifest = json.loads(path.read_text(encoding="utf-8"))
    sealed = [row for row in manifest.get("sealed_blind_cases") or [] if isinstance(row, dict)]
    if {matrix.first_text(row, "id") for row in sealed} != {
            "fabfilter_pro_l_2_sealed", "waves_l4_ultramaximizer_sealed"}:
        raise RuntimeError("sealed set must contain exactly Pro-L 2 and L4 Ultramaximizer")
    policy = manifest.get("policy") or {}
    if int(policy.get("sealed_parameter_surfaces_before_freeze", -1)) != 0:
        raise RuntimeError("manifest must prohibit sealed parameter reads before freeze")
    if int(policy.get("sealed_execution_limit_per_case", 0)) != 1:
        raise RuntimeError("manifest must limit every sealed case to one execution")
    for case in sealed:
        path_value = Path(matrix.first_text(case, "plugin_path"))
        if not path_value.exists():
            raise RuntimeError(f"sealed plugin path is missing: {path_value}")
    return manifest


def frozen_hashes() -> dict[str, str]:
    hashes: dict[str, str] = {}
    for relative in FROZEN_FILES:
        path = REPO_ROOT / relative
        if not path.is_file():
            raise RuntimeError(f"frozen file is missing: {path}")
        hashes[relative] = sha256_file(path)
    return hashes


def composite_hash(hashes: dict[str, str]) -> str:
    rows = "".join(f"{key}\0{value}\n" for key, value in sorted(hashes.items()))
    return hashlib.sha256(rows.encode("utf-8")).hexdigest()


def plugin_fingerprint(path: Path) -> dict[str, Any]:
    if path.is_file():
        stat = path.stat()
        return {"path": str(path), "sha256": sha256_file(path), "size": stat.st_size,
                "mtime_ns": stat.st_mtime_ns}
    files = sorted(item for item in path.rglob("*") if item.is_file())
    rows = []
    for item in files:
        rows.append({"relative": str(item.relative_to(path)), "sha256": sha256_file(item),
                     "size": item.stat().st_size})
    encoded = json.dumps(rows, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return {"path": str(path), "sha256": hashlib.sha256(encoded).hexdigest(),
            "file_count": len(rows), "files": rows}


def health(base: str) -> dict[str, Any]:
    return matrix.request_json("GET", base.rstrip("/") + "/health", None, 10.0)


def freeze(manifest_path: Path, freeze_path: Path, base: str, timeout: float) -> dict[str, Any]:
    if freeze_path.exists():
        raise RuntimeError(f"freeze output already exists: {freeze_path}")
    manifest = load_manifest(manifest_path)
    hashes = frozen_hashes()
    identities = []
    for case in manifest["sealed_blind_cases"]:
        identifier, resolution = matrix.resolve_identifier(case, timeout)
        configured = matrix.first_text(case, "plugin_path")
        resolved = matrix.first_text(resolution, "file_or_identifier", "plugin_path", "path")
        if not matrix.path_matches(configured, resolved):
            raise RuntimeError(f"{case['id']}: resolved path mismatch {resolved!r}")
        identities.append({"id": case["id"], "plugin_name": case["plugin_name"],
                           "plugin_identifier": identifier, "resolved_path": resolved,
                           "binary_fingerprint": plugin_fingerprint(Path(configured))})
    record = {
        "schema_version": "plugin_grabber.limiter_blind_freeze.v1",
        "frozen_at": utc_now(),
        "set_id": manifest["set_id"],
        "manifest_path": str(manifest_path.resolve()),
        "manifest_sha256": sha256_file(manifest_path),
        "runner_path": str(SCRIPT_PATH),
        "runner_sha256": sha256_file(SCRIPT_PATH),
        "frozen_file_sha256": hashes,
        "recognizer_control_composite_sha256": composite_hash(hashes),
        "agent_health": health(base),
        "identity_only_preflight": identities,
        "parameter_surfaces_read": 0,
        "execution_count": 0,
        "policy": manifest["policy"],
    }
    write_json(freeze_path, record)
    return record


def verify_freeze(record: dict[str, Any], manifest_path: Path) -> dict[str, Any]:
    hashes = frozen_hashes()
    mismatches = [{"path": key, "expected": record.get("frozen_file_sha256", {}).get(key),
                   "actual": hashes.get(key)} for key in sorted(set(hashes) | set(record.get("frozen_file_sha256", {})))
                  if hashes.get(key) != record.get("frozen_file_sha256", {}).get(key)]
    checks = {
        "manifest_expected": record.get("manifest_sha256"),
        "manifest_actual": sha256_file(manifest_path),
        "runner_expected": record.get("runner_sha256"),
        "runner_actual": sha256_file(SCRIPT_PATH),
        "composite_expected": record.get("recognizer_control_composite_sha256"),
        "composite_actual": composite_hash(hashes),
        "file_mismatches": mismatches,
    }
    checks["intact"] = (checks["manifest_expected"] == checks["manifest_actual"] and
                         checks["runner_expected"] == checks["runner_actual"] and
                         checks["composite_expected"] == checks["composite_actual"] and not mismatches)
    return checks


def parameter_rows(result: dict[str, Any]) -> list[dict[str, Any]]:
    return [row for row in result.get("parameters") or [] if isinstance(row, dict)]


def parameter_snapshot(result: dict[str, Any]) -> dict[str, float]:
    snapshot: dict[str, float] = {}
    for row in parameter_rows(result):
        param_id = matrix.first_text(row, "param_id", "parameter_id", "id")
        value = row.get("normalized_value", row.get("value"))
        if param_id and isinstance(value, (int, float)) and math.isfinite(float(value)):
            snapshot[param_id] = float(value)
    return snapshot


def all_bindings(inspect: dict[str, Any]) -> list[dict[str, Any]]:
    bindings: list[dict[str, Any]] = []
    for stage in inspect.get("limiter_stages") or []:
        if not isinstance(stage, dict):
            continue
        for section in ("operating_point", "safety", "timing", "detector", "mode", "output"):
            bindings.extend(row for row in stage.get(section) or [] if isinstance(row, dict))
    bindings.extend(row for row in inspect.get("shared_controls") or [] if isinstance(row, dict))
    return bindings


def choose_control(inspect: dict[str, Any]) -> dict[str, Any]:
    field_for_role = {
        "threshold": "value_db", "input_drive": "value_db", "ceiling": "value_db",
        "output_gain": "value_db", "release": "value_ms", "lookahead": "value_ms",
        "attack": "value_ms", "hold": "value_ms", "channel_link": "percent", "mix": "percent",
    }
    priority = ("threshold", "ceiling", "input_drive", "release", "lookahead", "channel_link", "mix")
    by_role = {matrix.first_text(row, "role"): row for row in all_bindings(inspect)}
    for role in priority:
        binding = by_role.get(role)
        if not binding or not matrix.first_text(binding, "control_ref"):
            continue
        current = binding.get("current_normalized")
        curve = binding.get("curve") or []
        candidates = []
        for pair in curve:
            if isinstance(pair, list) and len(pair) == 2 and all(isinstance(v, (int, float)) for v in pair):
                candidates.append((float(pair[0]), float(pair[1])))
        reachable = binding.get("reachable_values") or []
        for row in reachable:
            if isinstance(row, dict) and isinstance(row.get("normalized"), (int, float)) and isinstance(row.get("physical"), (int, float)):
                candidates.append((float(row["normalized"]), float(row["physical"])))
        if not candidates or not isinstance(current, (int, float)):
            continue
        candidates.sort(key=lambda item: abs(item[0] - float(current)))
        selected = next((item for item in candidates if abs(item[0] - float(current)) >= 0.05), None)
        if selected:
            return {"control_ref": binding["control_ref"], field_for_role[role]: selected[1],
                    "selected_role": role, "selected_normalized": selected[0]}
    raise RuntimeError("no proved numeric limiter binding exposes a safe alternate measured target")


def validate_inspect(result: dict[str, Any]) -> str:
    if result.get("schema_version") != "limiter-control-topology/v1" or result.get("mapping_source") != "generic_structural":
        raise RuntimeError("inspect did not return the generic Limiter V1 schema")
    generation = matrix.first_text(result.get("control_topology") or {}, "generation")
    stages = [row for row in result.get("limiter_stages") or [] if isinstance(row, dict)]
    if not generation or not stages:
        raise RuntimeError("inspect omitted generation or limiter stages")
    for binding in all_bindings(result):
        if not matrix.first_text(binding, "control_ref"):
            raise RuntimeError("inspect limiter binding omitted control_ref")
    return generation


def execute_case(base: str, case: dict[str, Any], identity: dict[str, Any], timeout: float,
                 case_dir: Path) -> dict[str, Any]:
    track_id = ""
    evidence: dict[str, Any] = {"case": case, "frozen_identity": identity}
    try:
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {"name": f"Limiter blind {case['id']}"}, timeout, True), "track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id, "plugin_path": case["plugin_path"], "plugin_name": case["plugin_name"],
            "plugin_identifier": identity["plugin_identifier"]}, timeout, True), "plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(.4)
        before_result = matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True}, timeout), "parameters.before")
        before = parameter_snapshot(before_result)
        evidence["before_parameters"] = before_result
        write_json(case_dir / "evidence.json", evidence)
        inspect_one = matrix.require_ok(matrix.invoke(base, "plugin_grabber.inspect_limiter", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout), "inspect.one")
        generation_one = validate_inspect(inspect_one)
        inspect_two = matrix.require_ok(matrix.invoke(base, "plugin_grabber.inspect_limiter", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout), "inspect.two")
        generation_two = validate_inspect(inspect_two)
        evidence["inspect_one"] = inspect_one
        evidence["inspect_two"] = inspect_two
        write_json(case_dir / "evidence.json", evidence)
        if generation_one != generation_two:
            raise RuntimeError(f"topology generation changed across identical reads: {generation_one} != {generation_two}")
        control = choose_control(inspect_two)
        selected_role = control.pop("selected_role")
        selected_normalized = control.pop("selected_normalized")
        applied = matrix.require_ok(matrix.invoke(base, "plugin_grabber.apply_limiter_controls", {
            "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "controls": [control]}, timeout), "apply")
        restore_ref = matrix.first_text(applied, "restore_ref")
        if not restore_ref:
            raise RuntimeError("apply omitted restore_ref")
        inspect_after = matrix.require_ok(matrix.invoke(base, "plugin_grabber.inspect_limiter", {
            "track_id": track_id, "plugin_id": plugin_id}, timeout), "inspect.after_apply")
        if validate_inspect(inspect_after) != generation_one:
            raise RuntimeError("topology generation changed after a value-only typed apply")
        restored = matrix.require_ok(matrix.invoke(base, "plugin_grabber.apply_limiter_controls", {
            "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "restore_ref": restore_ref}, timeout), "restore")
        final_result = matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id, "include_parameters": True}, timeout), "parameters.final")
        final = parameter_snapshot(final_result)
        differences = {key: {"before": before.get(key), "after": final.get(key)} for key in sorted(set(before) | set(final))
                       if before.get(key) != final.get(key)}
        if differences:
            raise RuntimeError(f"restore did not recover exact normalized preimage: {differences}")
        evidence.update({"before_parameters": before_result, "inspect_one": inspect_one, "inspect_two": inspect_two,
                         "selected_control": {"role": selected_role, "target_normalized": selected_normalized, **control},
                         "apply": applied, "inspect_after_apply": inspect_after, "restore": restored,
                         "final_parameters": final_result, "normalized_differences_after_restore": differences})
        result = {"id": case["id"], "plugin_name": case["plugin_name"], "status": "passed",
                  "classification": inspect_one.get("classification"), "generation": generation_one,
                  "stage_count": len(inspect_one.get("limiter_stages") or []), "control_role": selected_role,
                  "apply_status": applied.get("status"), "exact_preimage_restored": True}
        evidence["result"] = result
        write_json(case_dir / "evidence.json", evidence)
        return result
    except Exception as error:
        result = {"id": case["id"], "plugin_name": case["plugin_name"], "status": "failed", "error": str(error)}
        evidence["result"] = result
        write_json(case_dir / "evidence.json", evidence)
        return result
    finally:
        if track_id:
            response = matrix.invoke(base, "track.delete", {"track_id": track_id}, timeout, True)
            if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
                raise RuntimeError(f"disposable track cleanup failed: {response}")


def execute(manifest_path: Path, freeze_path: Path, output_dir: Path, base: str, timeout: float) -> dict[str, Any]:
    marker = freeze_path.with_suffix(freeze_path.suffix + ".executed.json")
    if marker.exists():
        raise RuntimeError(f"sealed set has already executed: {marker}")
    if output_dir.exists() and any(output_dir.iterdir()):
        raise RuntimeError(f"blind output must be empty: {output_dir}")
    manifest = load_manifest(manifest_path)
    record = json.loads(freeze_path.read_text(encoding="utf-8"))
    before = verify_freeze(record, manifest_path)
    if not before["intact"]:
        raise RuntimeError("freeze verification failed before blind execution")
    output_dir.mkdir(parents=True, exist_ok=True)
    write_json(output_dir / "freeze_verification_before.json", before)
    identities = {row["id"]: row for row in record.get("identity_only_preflight") or []}
    results = []
    for index, case in enumerate(manifest["sealed_blind_cases"], 1):
        identity = identities.get(case["id"])
        if not identity:
            raise RuntimeError(f"missing frozen identity for {case['id']}")
        results.append(execute_case(base, case, identity, timeout, output_dir / "cases" / f"{index:02d}_{case['id']}"))
    after = verify_freeze(record, manifest_path)
    write_json(output_dir / "freeze_verification_after.json", after)
    summary = {
        "schema_version": "plugin_grabber.limiter_blind_report.v1", "set_id": manifest["set_id"],
        "completed_at": utc_now(), "blind_claim": True, "parameter_surfaces_read_before_freeze": 0,
        "execution_count_per_case": 1, "freeze_intact_before": before["intact"],
        "freeze_intact_after": after["intact"], "results": results,
        "verdict": "passed" if after["intact"] and all(row["status"] == "passed" for row in results) else "failed",
    }
    write_json(output_dir / "summary.json", summary)
    write_json(marker, {"executed_at": utc_now(), "output_dir": str(output_dir.resolve()),
                        "case_ids": [row["id"] for row in results], "execution_count_per_case": 1,
                        "summary_sha256": sha256_file(output_dir / "summary.json")})
    return summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    freeze_parser = sub.add_parser("freeze")
    freeze_parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    freeze_parser.add_argument("--output", type=Path, required=True)
    freeze_parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    freeze_parser.add_argument("--timeout-sec", type=float, default=180)
    execute_parser = sub.add_parser("execute")
    execute_parser.add_argument("--manifest", type=Path, default=DEFAULT_MANIFEST)
    execute_parser.add_argument("--freeze", type=Path, required=True)
    execute_parser.add_argument("--output-dir", type=Path, required=True)
    execute_parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    execute_parser.add_argument("--timeout-sec", type=float, default=180)
    args = parser.parse_args()
    if args.command == "freeze":
        result = freeze(args.manifest.resolve(), args.output.resolve(), args.agent_http, args.timeout_sec)
    else:
        result = execute(args.manifest.resolve(), args.freeze.resolve(), args.output_dir.resolve(), args.agent_http, args.timeout_sec)
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
