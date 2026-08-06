#!/usr/bin/env python3
"""Exercise compressor physical apply/readback and restore on disposable tracks."""
from __future__ import annotations

import argparse
import json
import math
import time
from pathlib import Path
from typing import Any

import compressor_compat_matrix_smoke as matrix


SMOKE_ROLES = {
    "api_2500": ("threshold", "ratio"),
    "cla_2a": ("reduction_amount",),
    "cla_76": ("input_drive",),
    "mv2": ("low_level_amount", "high_level_amount"),
}


def parameter_snapshot(parameters: dict[str, Any]) -> dict[str, float]:
    snapshot: dict[str, float] = {}
    for row in parameters.get("parameters") or []:
        if not isinstance(row, dict):
            continue
        param_id = matrix.first_text(row, "param_id", "id")
        value = row.get("normalized_value")
        if param_id and isinstance(value, (int, float)) and math.isfinite(float(value)):
            snapshot[param_id] = float(value)
    return snapshot


def compressor_bindings(inspect: dict[str, Any]) -> list[dict[str, Any]]:
    stage = inspect.get("compressor_stage")
    if not isinstance(stage, dict):
        return []
    bindings: list[dict[str, Any]] = []
    for path in stage.get("control_paths") or []:
        if not isinstance(path, dict):
            continue
        for section in ("detector", "operating_point", "transfer", "timing", "gain_action"):
            bindings.extend(row for row in path.get(section) or [] if isinstance(row, dict))
    bindings.extend(row for row in stage.get("output") or [] if isinstance(row, dict))
    return bindings


def alternate_physical(binding: dict[str, Any], current: float) -> float:
    values: list[float] = []
    for point in binding.get("curve") or []:
        if isinstance(point, list) and len(point) == 2 and isinstance(point[1], (int, float)):
            value = float(point[1])
            if math.isfinite(value):
                values.append(value)
    for row in binding.get("reachable_values") or []:
        if isinstance(row, dict) and isinstance(row.get("physical"), (int, float)):
            value = float(row["physical"])
            if math.isfinite(value):
                values.append(value)
    domain = binding.get("domain")
    if isinstance(domain, dict) and float(domain.get("confidence") or 0) >= 0.80:
        for key in ("min", "max"):
            value = domain.get(key)
            if isinstance(value, (int, float)) and math.isfinite(float(value)):
                values.append(float(value))
    candidates = [value for value in values if abs(value-current) > max(1e-4, abs(current)*1e-4)]
    if not candidates:
        raise RuntimeError(
            f"{binding.get('role')} {binding.get('name')} has no alternate measured physical value")
    return max(candidates, key=lambda value: abs(value-current))


def controls_for_roles(inspect: dict[str, Any], roles: tuple[str, ...]) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    by_role: dict[str, list[dict[str, Any]]] = {}
    for binding in compressor_bindings(inspect):
        by_role.setdefault(matrix.first_text(binding, "role"), []).append(binding)
    forward: list[dict[str, Any]] = []
    restore: list[dict[str, Any]] = []
    for role in roles:
        candidates = by_role.get(role) or []
        if not candidates:
            raise RuntimeError(f"inspect omitted requested smoke role {role}")
        binding = candidates[0]
        domain = binding.get("domain")
        domain_unit = matrix.first_text(domain, "unit").lower() if isinstance(domain, dict) else ""
        control_ref = matrix.first_text(binding, "control_ref")
        if domain_unit in {"enum", "toggle"}:
            current_label = matrix.first_text(binding, "current_text")
            labels = [matrix.first_text(row, "label")
                      for row in binding.get("reachable_values") or []
                      if isinstance(row, dict)]
            alternate = next((label for label in labels
                              if label and label.casefold() != current_label.casefold()), "")
            if not alternate or not current_label:
                raise RuntimeError(f"{role} omitted reversible enum labels")
            forward.append({"control_ref": control_ref, "enum_label": alternate})
            restore.append({"control_ref": control_ref, "enum_label": current_label})
            continue
        current = binding.get("current_physical")
        if not isinstance(current, (int, float)) or not math.isfinite(float(current)):
            raise RuntimeError(f"{role} omitted finite current_physical")
        current_value = float(current)
        target = alternate_physical(binding, current_value)
        forward.append({"control_ref": control_ref, "display_value": target})
        restore.append({"control_ref": control_ref, "display_value": current_value})
    return forward, restore


def get_parameters(base: str, track_id: str, plugin_id: str, identifier: str,
                   timeout: float, label: str) -> dict[str, Any]:
    return matrix.require_ok(matrix.invoke(base, "plugin.get_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": identifier,
        "include_parameters": True,
    }, timeout), label)


def run_case(base: str, case: dict[str, Any], roles: tuple[str, ...], timeout: float) -> dict[str, Any]:
    case_id = matrix.first_text(case, "id")
    track_id = ""
    try:
        track = matrix.require_ok(matrix.invoke(base, "track.add_audio", {
            "name": f"Compressor apply {case_id}"}, timeout, True), f"{case_id} track.add_audio")
        track_id = matrix.first_text(track, "track_id", "id")
        identifier, _ = matrix.resolve_identifier(case, timeout)
        loaded = matrix.require_ok(matrix.invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": matrix.first_text(case, "plugin_path"),
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "plugin_identifier": identifier,
        }, timeout, True), f"{case_id} plugin.load_to_rack")
        plugin_id = matrix.first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        before = get_parameters(base, track_id, plugin_id, identifier, timeout, f"{case_id} before")
        before_snapshot = parameter_snapshot(before)
        inspect = matrix.require_ok(matrix.invoke(base, "plugin_grabber.inspect_compressor", {
            "track_id": track_id, "plugin_id": plugin_id,
        }, timeout), f"{case_id} inspect")
        forward, _ = controls_for_roles(inspect, roles)

        applied = matrix.require_ok(matrix.invoke(base, "plugin_grabber.apply_compressor_controls", {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "atomic": True,
            "controls": forward,
        }, timeout, True), f"{case_id} apply")
        if len(applied.get("controls") or []) != len(forward):
            raise RuntimeError("apply result omitted requested controls")
        if not all((row.get("actual_readback") for row in applied.get("controls") or [])):
            raise RuntimeError("apply result omitted physical readback")
        restore_ref = matrix.first_text(applied, "restore_ref")
        if not restore_ref:
            raise RuntimeError("apply result omitted exact normalized restore_ref")

        restored = matrix.require_ok(matrix.invoke(base, "plugin_grabber.apply_compressor_controls", {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "atomic": True,
            "restore_ref": restore_ref,
        }, timeout, True), f"{case_id} restore")
        after = get_parameters(base, track_id, plugin_id, identifier, timeout, f"{case_id} after restore")
        after_snapshot = parameter_snapshot(after)
        mismatches = [param_id for param_id, value in before_snapshot.items()
                      if abs(after_snapshot.get(param_id, value)-value) > 1e-4]
        if mismatches:
            raise RuntimeError(f"full snapshot did not restore: {mismatches[:12]}")
        return {
            "id": case_id,
            "plugin_name": matrix.first_text(case, "plugin_name"),
            "roles": list(roles),
            "apply_status": applied.get("status"),
            "restore_status": restored.get("status"),
            "write_count": len(applied.get("writes") or []),
            "full_snapshot_restored": True,
        }
    finally:
        if track_id:
            response = matrix.invoke(base, "track.delete", {"track_id": track_id}, timeout, True)
            matrix.require_ok(response, f"{case_id} track.delete")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7879")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--only", default="")
    args = parser.parse_args()

    config = json.loads(Path(args.config).read_text(encoding="utf-8"))
    cases = {matrix.first_text(case, "id"): case for case in config.get("cases") or []
             if isinstance(case, dict)}
    wanted = {value.strip() for value in args.only.split(",") if value.strip()}
    selected_roles = {case_id: roles for case_id, roles in SMOKE_ROLES.items()
                      if not wanted or case_id in wanted}
    unknown = wanted.difference(SMOKE_ROLES)
    if unknown:
        raise RuntimeError(f"unknown apply smoke cases: {', '.join(sorted(unknown))}")
    if not selected_roles:
        raise RuntimeError("apply smoke selected no cases")
    results: list[dict[str, Any]] = []
    for case_id, roles in selected_roles.items():
        case = cases.get(case_id)
        if case is None:
            raise RuntimeError(f"matrix omitted smoke case {case_id}")
        print(f"{case_id}: {', '.join(roles)}", flush=True)
        result = run_case(args.agent_http, case, roles, args.timeout_sec)
        results.append(result)
        print(
            f"  apply={result['apply_status']} restore={result['restore_status']} "
            f"writes={result['write_count']}", flush=True)
    report = {
        "schema_version": "plugin_grabber.compressor_apply_smoke_report.v1",
        "status": "ok",
        "results": results,
    }
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"report: {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
