#!/usr/bin/env python3
"""Capture the deterministic generic compressor topology across local products.

The runner never writes plug-in parameters. It creates one disposable track per
case, captures the complete observed parameter surface and inspect result, then
deletes that track even when validation fails.
"""
from __future__ import annotations

import argparse
import json
import os
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
    request = urllib.request.Request(
        url, data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
        except json.JSONDecodeError as decode_error:
            raise RuntimeError(f"HTTP {error.code} {url}: {body[:1000]}") from decode_error
        if isinstance(parsed, dict):
            return parsed
        raise RuntimeError(f"HTTP {error.code} {url}: {body[:1000]}")
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
        "source": "compressor_compat_matrix_smoke",
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
    if str(result.get("status", "ok")).lower() in {"error", "failed", "rejected"}:
        raise RuntimeError(
            f"{label}: result={result.get('status')!r} "
            f"error={result.get('message', result.get('error', ''))!r}")
    return result


def first_text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
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


def normalized_path(value: str) -> str:
    return os.path.normcase(os.path.normpath(value.strip())) if value else ""


def path_matches(configured: str, candidate: str) -> bool:
    expected, actual = normalized_path(configured), normalized_path(candidate)
    return bool(expected and actual and
                (expected == actual or actual.startswith(expected + os.sep)))


def resolve_identifier(case: dict[str, Any], timeout: float) -> tuple[str, dict[str, Any]]:
    name = first_text(case, "plugin_name")
    configured_path = first_text(case, "plugin_path")
    reply = kernel_command({"cmd": "plugin_search", "query": name, "limit": 32}, timeout)
    rows: list[dict[str, Any]] = []
    for key in ("plugins", "entries", "results"):
        if isinstance(reply.get(key), list):
            rows = [row for row in reply[key] if isinstance(row, dict)]
            break
    named = [row for row in rows
             if first_text(row, "name", "plugin_name").casefold() == name.casefold()]
    exact_path = [row for row in named
                  if normalized_path(first_text(
                      row, "file_or_identifier", "plugin_path", "path")) ==
                  normalized_path(configured_path)]
    exact = exact_path or [
        row for row in named
        if path_matches(configured_path,
                        first_text(row, "file_or_identifier", "plugin_path", "path"))]
    unique: dict[str, dict[str, Any]] = {}
    for row in exact:
        identifier = first_text(row, "identifier", "plugin_identifier")
        if identifier:
            unique.setdefault(identifier, row)
    if len(unique) != 1:
        raise RuntimeError(
            f"plugin_search did not uniquely resolve {name!r}: "
            f"exact_path={len(exact)} logical_candidates={len(unique)}")
    identifier, row = next(iter(unique.items()))
    return identifier, row


def binding_count(stage: dict[str, Any]) -> int:
    total = len(stage.get("output") or [])
    for path in stage.get("control_paths") or []:
        if not isinstance(path, dict):
            continue
        for section in ("detector", "operating_point", "transfer", "timing", "gain_action"):
            total += len(path.get(section) or [])
    return total


def validate_inspect(result: dict[str, Any]) -> dict[str, Any]:
    if result.get("mapping_source") != "generic_structural":
        raise RuntimeError(f"mapping_source={result.get('mapping_source')!r}")
    topology = result.get("control_topology")
    stage = result.get("compressor_stage")
    if not isinstance(topology, dict) or not first_text(topology, "generation"):
        raise RuntimeError("inspect omitted topology generation")
    if not isinstance(stage, dict) or not stage.get("control_paths"):
        raise RuntimeError("inspect omitted compressor control paths")
    refs = 0
    for path in stage.get("control_paths") or []:
        for section in ("detector", "operating_point", "transfer", "timing", "gain_action"):
            for binding in path.get(section) or []:
                if not first_text(binding, "control_ref"):
                    raise RuntimeError(f"{section} binding omitted control_ref")
                refs += 1
    for binding in stage.get("output") or []:
        if not first_text(binding, "control_ref"):
            raise RuntimeError("output binding omitted control_ref")
        refs += 1
    if refs != binding_count(stage):
        raise RuntimeError("control_ref coverage does not match binding count")
    return {
        "classification": result.get("classification"),
        "confidence": result.get("confidence"),
        "path_count": len(stage.get("control_paths") or []),
        "binding_count": refs,
        "generation": topology.get("generation"),
    }


def capture_case(base: str, case: dict[str, Any], timeout: float,
                 output_dir: Path) -> dict[str, Any]:
    case_id = first_text(case, "id")
    name = first_text(case, "plugin_name")
    plugin_path = first_text(case, "plugin_path")
    if not Path(plugin_path).exists():
        raise RuntimeError(f"configured plugin path does not exist: {plugin_path}")
    track_id = ""
    try:
        track = require_ok(invoke(base, "track.add_audio", {
            "name": f"Compressor matrix {case_id}"}, timeout, True), f"{case_id} track.add_audio")
        track_id = first_text(track, "track_id", "id")
        identifier, resolution = resolve_identifier(case, timeout)
        loaded = require_ok(invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": plugin_path,
            "plugin_name": name,
            "plugin_identifier": identifier,
        }, timeout, True), f"{case_id} plugin.load_to_rack")
        plugin_id = first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        parameters = require_ok(invoke(base, "plugin.get_parameters", {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "plugin_identifier": identifier,
            "include_parameters": True,
        }, timeout), f"{case_id} plugin.get_parameters")
        evidence = {
            "case": case,
            "plugin_identifier": identifier,
            "plugin_resolution": resolution,
            "parameters": parameters,
        }
        (output_dir / f"{case_id}.parameters.json").write_text(
            json.dumps(evidence, ensure_ascii=False, indent=2), encoding="utf-8")
        inspect = require_ok(invoke(base, "plugin_grabber.inspect_compressor", {
            "track_id": track_id,
            "plugin_id": plugin_id,
        }, timeout), f"{case_id} inspect_compressor")
        summary = validate_inspect(inspect)
        payload = {**evidence, "inspect": inspect, "summary": summary}
        (output_dir / f"{case_id}.json").write_text(
            json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        return {"id": case_id, "plugin_name": name, "status": "captured", **summary}
    finally:
        if track_id:
            response = invoke(base, "track.delete", {"track_id": track_id}, timeout, True)
            require_ok(response, f"{case_id} track.delete")


def capture_negative_case(base: str, case: dict[str, Any], timeout: float,
                          output_dir: Path) -> dict[str, Any]:
    case_id = first_text(case, "id")
    name = first_text(case, "plugin_name")
    plugin_path = first_text(case, "plugin_path")
    track_id = ""
    try:
        track = require_ok(invoke(base, "track.add_audio", {
            "name": f"Compressor negative {case_id}"}, timeout, True), f"{case_id} track.add_audio")
        track_id = first_text(track, "track_id", "id")
        identifier, resolution = resolve_identifier(case, timeout)
        loaded = require_ok(invoke(base, "plugin.load_to_rack", {
            "track_id": track_id,
            "plugin_path": plugin_path,
            "plugin_name": name,
            "plugin_identifier": identifier,
        }, timeout, True), f"{case_id} plugin.load_to_rack")
        plugin_id = first_text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        parameters = require_ok(invoke(base, "plugin.get_parameters", {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "plugin_identifier": identifier,
            "include_parameters": True,
        }, timeout), f"{case_id} plugin.get_parameters")
        preliminary = {
            "case": case,
            "plugin_identifier": identifier,
            "plugin_resolution": resolution,
            "parameters": parameters,
        }
        (output_dir / f"{case_id}.parameters.json").write_text(
            json.dumps(preliminary, ensure_ascii=False, indent=2), encoding="utf-8")
        inspect_response = invoke(base, "plugin_grabber.inspect_compressor", {
            "track_id": track_id,
            "plugin_id": plugin_id,
        }, timeout)
        status = str(inspect_response.get("status", "")).lower()
        result = result_of(inspect_response)
        rejection = first_text(result, "code", "rejection_code")
        error = first_text(inspect_response, "error")
        expected = first_text(case, "expected_rejection")
        if status in {"ok", "success", "completed"}:
            raise RuntimeError(f"negative {case.get('negative_kind')} was recognized as compressor")
        if expected and rejection != expected and expected not in error:
            raise RuntimeError(
                f"negative rejection={rejection!r} error={error!r}, expected={expected!r}")
        payload = {**preliminary, "inspect_response": inspect_response}
        (output_dir / f"{case_id}.json").write_text(
            json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        return {
            "id": case_id,
            "plugin_name": name,
            "status": "expected_rejection",
            "negative_kind": case.get("negative_kind"),
            "rejection": expected,
        }
    finally:
        if track_id:
            response = invoke(base, "track.delete", {"track_id": track_id}, timeout, True)
            require_ok(response, f"{case_id} track.delete")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7879")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--only", default="")
    args = parser.parse_args()

    config_path = Path(args.config)
    config = json.loads(config_path.read_text(encoding="utf-8"))
    positive_cases = [case for case in config.get("cases", []) if isinstance(case, dict)]
    negative_cases = [case for case in config.get("negative_cases", []) if isinstance(case, dict)]
    cases = positive_cases + negative_cases
    wanted = {value.strip() for value in args.only.split(",") if value.strip()}
    if wanted:
        cases = [case for case in cases if first_text(case, "id") in wanted]
    if not cases:
        raise RuntimeError("compatibility matrix selected no cases")
    if any("plugin alliance" in first_text(case, "plugin_name").casefold()
           for case in cases):
        raise RuntimeError("Plugin Alliance is reserved for the post-freeze blind set")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    health = request_json("GET", args.agent_http.rstrip("/") + "/health", None, 10)
    if str(health.get("status", "")).lower() not in {"ok", "ready"}:
        raise RuntimeError(f"agent health check failed: {health}")

    results: list[dict[str, Any]] = []
    failures: list[dict[str, str]] = []
    for index, case in enumerate(cases, 1):
        case_id = first_text(case, "id")
        print(f"[{index}/{len(cases)}] {case_id}: {first_text(case, 'plugin_name')}", flush=True)
        try:
            if case.get("expected_compressor", True):
                result = capture_case(args.agent_http, case, args.timeout_sec, output_dir)
            else:
                result = capture_negative_case(
                    args.agent_http, case, args.timeout_sec, output_dir)
            results.append(result)
            if result["status"] == "expected_rejection":
                print(f"  rejected as {result['negative_kind']}", flush=True)
            else:
                print(
                    f"  {result['classification']} paths={result['path_count']} "
                    f"bindings={result['binding_count']}", flush=True)
        except Exception as error:  # Continue to retain evidence for every product.
            failures.append({"id": case_id, "error": str(error)})
            print(f"  ERROR: {error}", flush=True)

    report = {
        "schema_version": "plugin_grabber.compressor_compat_matrix_report.v1",
        "config": str(config_path.resolve()),
        "positive_case_count": len(positive_cases),
        "negative_case_count": len(negative_cases),
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
