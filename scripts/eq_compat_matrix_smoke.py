#!/usr/bin/env python3
"""Data-driven Plugin Grabber EQ compatibility capture and smoke runner.

Capture mode is intentionally read-only with respect to plug-in parameters. It
creates a disposable project, loads each configured plug-in, and records the
complete parameter/probe response plus the Plugin Grabber structural summary.

Smoke mode uses the same entry point and is extended by the Go implementation
tests in this task; it must never select production behaviour by fixture name.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

import zmq


def request_json(method: str, url: str, payload: dict[str, Any] | None,
                 timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(
        payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {exc.code} {url}: {body[:1000]}") from exc
    parsed = json.loads(body)
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float,
           confirmed: bool = False) -> dict[str, Any]:
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool,
        "args": args,
        "confirmed": confirmed,
        "source": "eq_compat_matrix_smoke",
    }, timeout)


def result_of(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result", response)
    return result if isinstance(result, dict) else {}


def require_ok(response: dict[str, Any], label: str) -> dict[str, Any]:
    status = str(response.get("status", "")).lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(
            f"{label}: status={status!r} error={response.get('error', '')!r}")
    result = result_of(response)
    inner_status = str(result.get("status", "ok")).lower()
    if inner_status in {"error", "failed", "partial_failure"}:
        raise RuntimeError(
            f"{label}: inner status={inner_status!r} error={result.get('error', '')!r}")
    return result


def first_text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None:
            text = str(value).strip()
            if text:
                return text
    return ""


def kernel_command(command: dict[str, Any], timeout: float) -> dict[str, Any]:
    context = zmq.Context.instance()
    socket = context.socket(zmq.REQ)
    timeout_ms = max(1000, int(timeout * 1000))
    socket.setsockopt(zmq.RCVTIMEO, timeout_ms)
    socket.setsockopt(zmq.SNDTIMEO, timeout_ms)
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect("tcp://127.0.0.1:5555")
    try:
        socket.send_json(command)
        reply = socket.recv_json()
    finally:
        socket.close()
    if not isinstance(reply, dict):
        raise RuntimeError(f"kernel {command.get('cmd')} returned non-object JSON")
    return reply


def normalized_plugin_path(value: str) -> str:
    if not value:
        return ""
    return os.path.normcase(os.path.normpath(value.strip()))


def plugin_path_matches(configured: str, candidate: str) -> bool:
    configured_path = normalized_plugin_path(configured)
    candidate_path = normalized_plugin_path(candidate)
    if not configured_path or not candidate_path:
        return False
    if configured_path == candidate_path:
        return True
    # JUCE may index a VST3 bundle by its platform binary below the configured
    # .vst3 directory. That is the same plug-in location, not a fuzzy match.
    return candidate_path.startswith(configured_path + os.sep)


def resolve_plugin_identifier(case: dict[str, Any], timeout: float,
                              output_dir: Path) -> tuple[str, dict[str, Any]]:
    configured = first_text(case, "plugin_identifier")
    if configured:
        return configured, {"source": "fixture", "identifier": configured}
    plugin_name = first_text(case, "plugin_name")
    reply = kernel_command({"cmd": "plugin_search", "query": plugin_name, "limit": 32}, timeout)
    case_id = first_text(case, "id")
    (output_dir / f"{case_id}.plugin_search.json").write_text(
        json.dumps(reply, ensure_ascii=False, indent=2), encoding="utf-8")
    status = str(reply.get("status", "")).lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(
            f"plugin_search {plugin_name!r}: status={status!r} "
            f"error={reply.get('error', reply.get('message', ''))!r}")
    rows: list[dict[str, Any]] = []
    for key in ("plugins", "entries", "results"):
        value = reply.get(key)
        if isinstance(value, list):
            rows = [row for row in value if isinstance(row, dict)]
            break
    exact = [row for row in rows
             if first_text(row, "name", "plugin_name").casefold() == plugin_name.casefold()]
    configured_path = first_text(case, "plugin_path")
    path_exact = [row for row in exact if normalized_plugin_path(first_text(
        row, "file_or_identifier", "plugin_path", "path")) ==
        normalized_plugin_path(configured_path)]
    if not path_exact:
        path_exact = [row for row in exact if plugin_path_matches(
            configured_path,
            first_text(row, "file_or_identifier", "plugin_path", "path"))]

    # The known plug-in list can contain duplicate rows after repeated scans.
    # Collapse only rows that identify the exact same logical plug-in; distinct
    # identifiers remain an ambiguity and must fail closed.
    by_identifier: dict[str, dict[str, Any]] = {}
    for row in path_exact:
        identifier = first_text(row, "identifier", "plugin_identifier")
        if identifier:
            by_identifier.setdefault(identifier, row)
    candidates = list(by_identifier.values())
    if len(candidates) != 1:
        details = [{
            "name": first_text(row, "name", "plugin_name"),
            "identifier": first_text(row, "identifier", "plugin_identifier"),
            "path": first_text(row, "file_or_identifier", "plugin_path", "path"),
            "uid": first_text(row, "uid", "deprecated_uid"),
        } for row in exact]
        raise RuntimeError(
            f"plugin_search did not uniquely resolve {plugin_name!r}; "
            f"exact_name={len(exact)} exact_path={len(path_exact)} "
            f"logical_candidates={len(candidates)} candidates={details}")
    selected = candidates[0]
    identifier = first_text(selected, "identifier", "plugin_identifier")
    if not identifier:
        raise RuntimeError(f"plugin_search result for {plugin_name!r} omitted identifier")
    return identifier, selected


def load_case(base: str, case: dict[str, Any], timeout: float,
              output_dir: Path) -> tuple[str, str, str, dict[str, Any]]:
    case_id = first_text(case, "id")
    plugin_name = first_text(case, "plugin_name")
    plugin_path = first_text(case, "plugin_path")
    track = require_ok(invoke(
        base, "track.add_audio", {"name": f"EQ matrix {case_id}"}, timeout, True),
        f"{case_id} track.add_audio")
    track_id = first_text(track, "track_id", "id")
    if not track_id:
        raise RuntimeError(f"{case_id}: track.add_audio returned no track_id")

    load_args: dict[str, Any] = {
        "track_id": track_id,
        "plugin_path": plugin_path,
        "plugin_name": plugin_name,
    }
    identifier, resolution = resolve_plugin_identifier(case, timeout, output_dir)
    if identifier:
        load_args["plugin_identifier"] = identifier
    loaded = require_ok(invoke(
        base, "plugin.load_to_rack", load_args, timeout, True),
        f"{case_id} plugin.load_to_rack")
    plugin_id = first_text(loaded, "plugin_id", "node_id", "id")
    if not plugin_id:
        raise RuntimeError(f"{case_id}: plugin.load_to_rack returned no plugin_id")
    return track_id, plugin_id, identifier, resolution


def capture_case(base: str, case: dict[str, Any], timeout: float,
                 output_dir: Path, mode: str,
                 defaults: dict[str, Any]) -> dict[str, Any]:
    case_id = first_text(case, "id")
    path = Path(first_text(case, "plugin_path"))
    if not path.exists():
        raise RuntimeError(f"configured plugin path does not exist: {path}")

    track_id, plugin_id, plugin_identifier, plugin_resolution = load_case(
        base, case, timeout, output_dir)
    time.sleep(0.6)
    parameters = require_ok(invoke(base, "plugin.get_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": plugin_identifier,
        "include_parameters": True,
    }, timeout), f"{case_id} plugin.get_parameters")
    explain = require_ok(invoke(base, "plugin_grabber.explain_controls", {
        "track_id": track_id,
        "plugin_id": plugin_id,
    }, timeout), f"{case_id} plugin_grabber.explain_controls")

    smoke_result: dict[str, Any] | None = None
    if mode == "smoke":
        smoke_result = smoke_case(base, case, track_id, plugin_id,
                                  parameters, explain, defaults, timeout)

    payload = {
        "case": case,
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": plugin_identifier,
        "plugin_resolution": plugin_resolution,
        "parameters": parameters,
        "explain": explain,
        "smoke": smoke_result,
    }
    (output_dir / f"{case_id}.json").write_text(
        json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")

    rows = [row for row in parameters.get("parameters", [])
            if isinstance(row, dict)]
    summary = explain.get("eq_band_summary")
    summary = summary if isinstance(summary, dict) else {}
    return {
        "id": case_id,
        "plugin_name": first_text(case, "plugin_name"),
        "track_id": track_id,
        "plugin_id": plugin_id,
        "parameter_count": len(rows),
        "eq_model": summary.get("eq_model"),
        "band_count": summary.get("band_count", summary.get("active_band_count")),
        "available_slot_count": summary.get("available_slot_count"),
        "set_eq_point_supported": summary.get("set_eq_point_supported"),
        "mapping_source": summary.get("mapping_source"),
        "status": "captured",
    }


def parameter_rows(parameters: dict[str, Any]) -> list[dict[str, Any]]:
    return [row for row in parameters.get("parameters", [])
            if isinstance(row, dict)]


def param_id(row: dict[str, Any]) -> str:
    return first_text(row, "param_id", "parameter_id", "id")


def normalized_value(row: dict[str, Any]) -> float:
    value = row.get("normalized_value", row.get("normalised_value"))
    if not isinstance(value, (int, float)):
        raise RuntimeError(f"parameter {param_id(row)} has no normalized value")
    return float(value)


def smoke_case(base: str, case: dict[str, Any], track_id: str, plugin_id: str,
               before: dict[str, Any], explain: dict[str, Any],
               defaults: dict[str, Any], timeout: float) -> dict[str, Any]:
    summary = explain.get("eq_band_summary")
    summary = summary if isinstance(summary, dict) else None
    expected_eq = case.get("expected_eq", True)
    expected_supported = case.get("expected_supported", True)
    if not expected_eq:
        if summary is not None:
            raise RuntimeError("negative case was recognised as EQ")
        return {"status": "expected_non_eq"}
    if summary is None:
        raise RuntimeError("expected EQ structural summary is absent")
    if case.get("expected_model") and summary.get("eq_model") != case["expected_model"]:
        raise RuntimeError(
            f"model={summary.get('eq_model')!r}, expected={case['expected_model']!r}")
    expected_bands = case.get("expected_band_count")
    if expected_bands is not None and summary.get("band_count") != expected_bands:
        raise RuntimeError(
            f"band_count={summary.get('band_count')!r}, expected={expected_bands!r}")
    expected_parameters = case.get("expected_parameter_count")
    if expected_parameters is not None and len(parameter_rows(before)) != expected_parameters:
        raise RuntimeError(
            f"parameter_count={len(parameter_rows(before))}, expected={expected_parameters}")
    if summary.get("mapping_source") != "generic_structural":
        raise RuntimeError(f"unexpected generic mapping source: {summary.get('mapping_source')!r}")
    expected_kinds = {str(value).casefold() for value in
                      case.get("expected_filter_kinds", [])}
    actual_kinds = {str(value).casefold() for value in
                    (summary.get("supported_filter_kinds") or [])}
    if expected_kinds and not expected_kinds.issubset(actual_kinds):
        raise RuntimeError(
            f"supported_filter_kinds={sorted(actual_kinds)!r}, "
            f"missing={sorted(expected_kinds-actual_kinds)!r}")
    completeness = summary.get("completeness")
    if expected_supported and (
            not isinstance(completeness, dict) or not completeness.get("complete")):
        raise RuntimeError(f"EQ structure is incomplete: {completeness!r}")
    expected_channels = sorted(str(value).casefold() for value in
                               case.get("expected_channels", []))
    actual_channels = sorted(str(value).casefold() for value in
                             (summary.get("channel_bindings") or []))
    if expected_channels and actual_channels != expected_channels:
        raise RuntimeError(
            f"channel_bindings={actual_channels!r}, expected={expected_channels!r}")
    actual_supported = bool(summary.get("set_eq_point_supported"))
    if actual_supported != bool(expected_supported):
        raise RuntimeError(
            f"set_eq_point_supported={actual_supported}, expected={expected_supported}")
    target = dict(defaults.get("target", {}))
    if case.get("request_q") is False:
        target.pop("q", None)
    case_target = case.get("target")
    if isinstance(case_target, dict):
        target.update(case_target)
    if not expected_supported:
        reason = str(summary.get("reason", ""))
        expected_reason = str(case.get("expected_reason", ""))
        if expected_reason and expected_reason not in reason:
            raise RuntimeError(f"unsupported reason={reason!r}, expected {expected_reason!r}")
        try:
            rejected = invoke(base, "plugin_grabber.set_eq_point", {
                "track_id": track_id, "plugin_id": plugin_id, **target,
            }, timeout, True)
        except RuntimeError as exc:
            if expected_reason and expected_reason not in str(exc):
                raise RuntimeError(f"execution rejection omitted expected reason: {exc}") from exc
        else:
            if str(rejected.get("status", "")).lower() in {"ok", "success", "completed"}:
                raise RuntimeError("unsupported EQ unexpectedly entered execution")
        return {"status": "expected_unsupported", "reason": reason}

    before_rows = parameter_rows(before)
    snapshot = {param_id(row): normalized_value(row) for row in before_rows}
    response = invoke(base, "plugin_grabber.set_eq_point", {
        "track_id": track_id, "plugin_id": plugin_id, **target,
    }, timeout, True)
    applied = require_ok(response, f"{case.get('id')} plugin_grabber.set_eq_point")
    writes = [row for row in applied.get("writes", []) if isinstance(row, dict)]
    touched = {first_text(row, "param_id") for row in writes}
    touched.discard("")
    if not touched:
        raise RuntimeError("set_eq_point returned no touched parameters")
    requested_shape = str(target.get("shape", "")).casefold()
    if requested_shape:
        selected = applied.get("selected_section")
        if not isinstance(selected, dict) or str(selected.get("shape", "")).casefold() != requested_shape:
            raise RuntimeError(
                f"selected_section did not confirm requested shape {requested_shape!r}: {selected!r}")
        expected_partial = bool(case.get("expected_partial", False))
        if bool(applied.get("partial")) != expected_partial:
            raise RuntimeError(
                f"partial={applied.get('partial')!r}, expected={expected_partial!r}; "
                f"limitations={applied.get('limitations')!r}")
    expected_quantized = {str(value).casefold() for value in
                          case.get("expect_quantized_roles", [])}
    actual_quantized = {first_text(row, "role").casefold() for row in writes
                        if bool(row.get("quantized"))}
    if expected_quantized and not expected_quantized.issubset(actual_quantized):
        raise RuntimeError(
            f"quantized_roles={sorted(actual_quantized)!r}, "
            f"expected at least={sorted(expected_quantized)!r}")
    activation_indices = [i for i, row in enumerate(writes)
                          if first_text(row, "role") == "used"]
    if activation_indices and activation_indices != list(
            range(len(writes)-len(activation_indices), len(writes))):
        raise RuntimeError("activation writes were not last")
    actual_by_id = {first_text(row, "param_id"): row for row in
                    applied.get("actual_readback", []) if isinstance(row, dict)}
    if set(actual_by_id) != touched:
        raise RuntimeError(
            f"actual_readback ids={sorted(actual_by_id)!r}, touched={sorted(touched)!r}")
    inactive_labels = {"unused", "disabled", "off", "out", "bypass", "bypassed"}
    for index in activation_indices:
        pid = first_text(writes[index], "param_id")
        label = first_text(actual_by_id.get(pid, {}), "value_text").casefold()
        if not label or label in inactive_labels:
            raise RuntimeError(
                f"activation parameter {pid} did not read back active: {label!r}")
    validation_error: Exception | None = None
    try:
        forbidden = [str(token).casefold() for token in
                     case.get("forbidden_write_name_tokens", [])]
        by_id = {param_id(row): row for row in before_rows}
        for touched_id in touched:
            name = first_text(by_id.get(touched_id, {}), "name", "raw_param_name")
            if any(token in name.casefold() for token in forbidden):
                raise RuntimeError(f"forbidden parameter was touched: {name} ({touched_id})")

        after = require_ok(invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id,
            "include_parameters": True,
        }, timeout), f"{case.get('id')} post-write readback")
        after_snapshot = {param_id(row): normalized_value(row)
                          for row in parameter_rows(after)}
        unrelated = [pid for pid, value in snapshot.items()
                     if pid not in touched and abs(after_snapshot.get(pid, value)-value) > 1e-4]
        if unrelated:
            raise RuntimeError(f"unrelated parameters changed: {unrelated[:12]}")
    except Exception as exc:
        validation_error = exc
    finally:
        restore_errors: list[str] = []
        roles = {first_text(row, "param_id"): first_text(row, "role") for row in writes}
        restore_order = sorted(touched, key=lambda pid: (roles.get(pid) != "used", pid))
        for pid in restore_order:
            try:
                require_ok(invoke(base, "plugin.set_parameter", {
                    "track_id": track_id, "plugin_id": plugin_id,
                    "param_id": pid, "value": snapshot[pid],
                }, timeout, True), f"{case.get('id')} restore {pid}")
            except Exception as exc:
                restore_errors.append(str(exc))
        restored = require_ok(invoke(base, "plugin.get_parameters", {
            "track_id": track_id, "plugin_id": plugin_id,
            "include_parameters": True,
        }, timeout), f"{case.get('id')} restore readback")
        restored_snapshot = {param_id(row): normalized_value(row)
                             for row in parameter_rows(restored)}
        mismatches = [pid for pid, value in snapshot.items()
                      if abs(restored_snapshot.get(pid, value)-value) > 1e-4]
        if restore_errors or mismatches:
            raise RuntimeError(
                f"state restoration failed errors={restore_errors} mismatches={mismatches[:12]}")
    if validation_error is not None:
        raise validation_error
    return {"status": "ok", "touched_parameter_ids": sorted(touched),
            "result": applied}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--mode", choices=("capture", "smoke"), default="capture")
    parser.add_argument("--only", default="",
                        help="comma-separated case ids; default runs all")
    args = parser.parse_args()

    config_path = Path(args.config)
    config = json.loads(config_path.read_text(encoding="utf-8"))
    cases = [case for case in config.get("cases", []) if isinstance(case, dict)]
    wanted = {value.strip() for value in args.only.split(",") if value.strip()}
    if wanted:
        cases = [case for case in cases if first_text(case, "id") in wanted]
    if not cases:
        raise RuntimeError("compatibility matrix selected no cases")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    base = args.agent_http.rstrip("/")
    health = request_json("GET", base + "/health", None, 10)
    if str(health.get("status", "")).lower() not in {"ok", "ready"}:
        raise RuntimeError(f"agent health check failed: {health}")

    require_ok(invoke(base, "project.new", {}, args.timeout_sec, True), "project.new")
    time.sleep(0.5)

    results: list[dict[str, Any]] = []
    failures: list[dict[str, str]] = []
    for index, case in enumerate(cases, 1):
        case_id = first_text(case, "id")
        print(f"[{index}/{len(cases)}] {case_id}: {first_text(case, 'plugin_name')}", flush=True)
        try:
            result = capture_case(base, case, args.timeout_sec, output_dir,
                                  args.mode, config.get("defaults", {}))
            results.append(result)
            print(
                f"  captured params={result['parameter_count']} model={result['eq_model']!r} "
                f"bands={result['band_count']!r}", flush=True)
        except Exception as exc:  # keep the complete matrix evidence in one run
            failures.append({"id": case_id, "error": str(exc)})
            print(f"  ERROR: {exc}", file=sys.stderr, flush=True)

    report = {
        "schema_version": "plugin_grabber.eq_compat_matrix_report.v1",
        "mode": args.mode,
        "config": str(config_path.resolve()),
        "results": results,
        "failures": failures,
        "status": "failed" if failures else "ok",
    }
    (output_dir / "summary.json").write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"report: {output_dir / 'summary.json'}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
