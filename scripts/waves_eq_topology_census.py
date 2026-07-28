#!/usr/bin/env python3
"""Read-only Waves EQ/Channel Strip parameter topology census.

The runtime is launched by Godot.  This script resolves one exact Waves VST3
shell member, loads it into the disposable project, and obtains its complete
parameter surface through paged plugin.get_parameters calls.  It never writes
plugin parameters and never invokes a learning, profile, or production EQ
summary path.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
from collections import Counter
from pathlib import Path
from typing import Any

import zmq


ALLOWED_AGENT_TOOLS = {
    "track.add_audio",
    "plugin.load_to_rack",
    "plugin.get_parameters",
}


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":"))


def sha256_json(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2),
                    encoding="utf-8")


def request_json(method: str, url: str, payload: dict[str, Any] | None,
                 timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(
        payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8", errors="strict")
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {exc.code} {url}: {body[:1200]}") from exc
    parsed = json.loads(body)
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def result_of(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result", response)
    return result if isinstance(result, dict) else {}


def require_ok(response: dict[str, Any], label: str) -> dict[str, Any]:
    status = str(response.get("status", "")).casefold()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(
            f"{label}: status={status!r} error={response.get('error', '')!r}")
    result = result_of(response)
    inner = str(result.get("status", "ok")).casefold()
    if inner in {"error", "failed", "partial_failure"}:
        raise RuntimeError(
            f"{label}: inner_status={inner!r} error={result.get('error', '')!r}")
    return result


def first_text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
    return ""


class Audit:
    def __init__(self) -> None:
        self.agent_tools: Counter[str] = Counter()
        self.kernel_commands: Counter[str] = Counter()
        self.events: list[dict[str, Any]] = []

    def agent(self, tool: str, args: dict[str, Any]) -> None:
        if tool not in ALLOWED_AGENT_TOOLS:
            raise RuntimeError(f"tool is outside census allowlist: {tool}")
        self.agent_tools[tool] += 1
        self.events.append({
            "route": "agent",
            "name": tool,
            "argument_keys": sorted(args),
        })

    def kernel(self, command: str) -> None:
        if command != "plugin_search":
            raise RuntimeError(
                f"kernel command is outside census allowlist: {command}")
        self.kernel_commands[command] += 1
        self.events.append({"route": "kernel", "name": command})

    def summary(self) -> dict[str, Any]:
        return {
            "agent_tools": dict(sorted(self.agent_tools.items())),
            "kernel_commands": dict(sorted(self.kernel_commands.items())),
            "plugin_parameter_write_count": 0,
            "audio_probe_count": 0,
            "learning_call_count": 0,
            "profile_call_count": 0,
            "spal_call_count": 0,
            "b4_call_count": 0,
            "plugin_alliance_parameter_read_count": 0,
            "event_count": len(self.events),
        }


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float,
           audit: Audit, confirmed: bool = False) -> dict[str, Any]:
    audit.agent(tool, args)
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool,
        "args": args,
        "confirmed": confirmed,
        "source": "waves_eq_topology_census",
    }, timeout)


def kernel_command(command: dict[str, Any], timeout: float,
                   audit: Audit) -> dict[str, Any]:
    name = first_text(command, "cmd")
    audit.kernel(name)
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
        raise RuntimeError(f"kernel {name} returned non-object JSON")
    return reply


def normalized_path(value: str) -> str:
    if not value:
        return ""
    return os.path.normcase(os.path.normpath(value.strip()))


def same_plugin_path(configured: str, candidate: str) -> bool:
    expected = normalized_path(configured)
    actual = normalized_path(candidate)
    return bool(expected and actual and (
        expected == actual or actual.startswith(expected + os.sep)))


def search_rows(reply: dict[str, Any]) -> list[dict[str, Any]]:
    for key in ("plugins", "entries", "results"):
        value = reply.get(key)
        if isinstance(value, list):
            return [row for row in value if isinstance(row, dict)]
    return []


def resolve_plugin(case: dict[str, Any], plugin_path: str, vendor: str,
                   plugin_format: str, timeout: float, audit: Audit,
                   output_dir: Path) -> tuple[str, dict[str, Any]]:
    plugin_name = first_text(case, "plugin_name")
    case_id = first_text(case, "id")
    reply = kernel_command({
        "cmd": "plugin_search", "query": plugin_name, "limit": 64,
    }, timeout, audit)
    write_json(output_dir / "search" / f"{case_id}.json", reply)
    status = str(reply.get("status", "")).casefold()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(
            f"plugin_search failed status={status!r}: "
            f"{reply.get('error', reply.get('message', ''))}")

    exact = [row for row in search_rows(reply)
             if first_text(row, "name", "plugin_name").casefold()
             == plugin_name.casefold()]
    exact = [row for row in exact if same_plugin_path(
        plugin_path, first_text(row, "file_or_identifier", "plugin_path", "path"))]
    by_identifier: dict[str, dict[str, Any]] = {}
    for row in exact:
        identifier = first_text(row, "identifier", "plugin_identifier")
        if identifier:
            by_identifier.setdefault(identifier, row)
    candidates = list(by_identifier.values())
    if len(candidates) != 1:
        raise RuntimeError(
            f"exact Waves shell member resolution is ambiguous: "
            f"rows={len(exact)} identifiers={sorted(by_identifier)}")
    selected = candidates[0]
    manufacturer = first_text(selected, "manufacturer")
    actual_format = first_text(selected, "format", "plugin_format")
    category = first_text(selected, "category")
    expected_category = first_text(case, "expected_category")
    if manufacturer.casefold() != vendor.casefold():
        raise RuntimeError(
            f"manufacturer guard rejected {manufacturer!r}; expected {vendor!r}")
    if actual_format.casefold() != plugin_format.casefold():
        raise RuntimeError(
            f"format guard rejected {actual_format!r}; expected {plugin_format!r}")
    if expected_category and category.casefold() != expected_category.casefold():
        raise RuntimeError(
            f"category guard rejected {category!r}; expected {expected_category!r}")
    if not plugin_name.casefold().endswith("stereo"):
        raise RuntimeError(f"Stereo representative guard rejected {plugin_name!r}")
    identifier = first_text(selected, "identifier", "plugin_identifier")
    return identifier, selected


def load_plugin(base: str, case: dict[str, Any], plugin_path: str,
                vendor: str, plugin_format: str, timeout: float, audit: Audit,
                output_dir: Path) -> tuple[str, str, str, dict[str, Any]]:
    case_id = first_text(case, "id")
    plugin_name = first_text(case, "plugin_name")
    identifier, resolution = resolve_plugin(
        case, plugin_path, vendor, plugin_format, timeout, audit, output_dir)
    track = require_ok(invoke(base, "track.add_audio", {
        "name": f"Waves topology {case_id}",
    }, timeout, audit, True), f"{case_id} track.add_audio")
    track_id = first_text(track, "track_id", "id")
    if not track_id:
        raise RuntimeError("track.add_audio returned no track_id")
    loaded = require_ok(invoke(base, "plugin.load_to_rack", {
        "track_id": track_id,
        "plugin_path": plugin_path,
        "plugin_name": plugin_name,
        "plugin_identifier": identifier,
    }, timeout, audit, True), f"{case_id} plugin.load_to_rack")
    plugin_id = first_text(loaded, "plugin_id", "node_id", "id")
    if not plugin_id:
        raise RuntimeError("plugin.load_to_rack returned no plugin_id")
    return track_id, plugin_id, identifier, resolution


def parameter_id(row: dict[str, Any]) -> str:
    return first_text(row, "param_id", "parameter_id", "id")


def host_metadata_row(row: dict[str, Any]) -> dict[str, Any]:
    allowed = {
        "id", "param_id", "parameter_id", "name", "raw_param_name", "alias",
        "label", "display_group", "normalized_role", "normalized_value", "value",
        "value_text", "min", "max", "is_boolean", "is_discrete", "num_steps",
        "numSteps", "all_labels", "allLabels", "labels", "host_controllable",
        "display_probe", "display_domain_candidate",
    }
    return {key: value for key, value in row.items() if key in allowed}


def collect_paged_parameters(base: str, case_id: str, track_id: str,
                             plugin_id: str, identifier: str, page_size: int,
                             timeout: float, audit: Audit,
                             output_dir: Path) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    parameters: list[dict[str, Any]] = []
    pages: list[dict[str, Any]] = []
    totals: list[int] = []
    offset = 0
    page_index = 0
    while True:
        result = require_ok(invoke(base, "plugin.get_parameters", {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "plugin_identifier": identifier,
            "include_parameters": True,
            "offset": offset,
            "limit": page_size,
        }, timeout, audit), f"{case_id} page {page_index}")
        rows = [row for row in result.get("parameters", [])
                if isinstance(row, dict)]
        page = result.get("parameter_page")
        if not isinstance(page, dict):
            raise RuntimeError("plugin.get_parameters omitted parameter_page")
        reported_offset = int(page.get("offset", -1))
        reported_limit = int(page.get("limit", -1))
        reported_total = int(page.get("total", -1))
        if reported_offset != offset:
            raise RuntimeError(
                f"page offset drift: requested={offset} reported={reported_offset}")
        if reported_limit != page_size:
            raise RuntimeError(
                f"page limit drift: requested={page_size} reported={reported_limit}")
        if reported_total < 0:
            raise RuntimeError(f"invalid reported total: {reported_total}")
        totals.append(reported_total)
        page_record = {
            "index": page_index,
            "offset": reported_offset,
            "limit": reported_limit,
            "total": reported_total,
            "row_count": len(rows),
            "parameter_ids": [parameter_id(row) for row in rows],
        }
        pages.append(page_record)
        write_json(output_dir / "pages" / case_id /
                   f"page_{page_index:04d}.json", {
                       "page": page_record,
                       "parameters": rows,
                   })
        parameters.extend(rows)
        offset += len(rows)
        page_index += 1
        if offset >= reported_total:
            break
        if not rows:
            raise RuntimeError(
                f"empty page before total: offset={offset} total={reported_total}")
        if page_index > 10000:
            raise RuntimeError("pagination safety limit exceeded")

    unique_totals = sorted(set(totals))
    ids = [parameter_id(row) for row in parameters]
    empty_ids = [index for index, value in enumerate(ids) if not value]
    duplicate_ids = sorted(
        value for value, count in Counter(ids).items() if value and count > 1)
    expected_total = totals[-1] if totals else 0
    complete = (
        len(unique_totals) == 1
        and len(parameters) == expected_total
        and not empty_ids
        and not duplicate_ids
    )
    pagination = {
        "page_size": page_size,
        "page_count": len(pages),
        "reported_totals": unique_totals,
        "expected_total": expected_total,
        "actual_count": len(parameters),
        "empty_parameter_id_indices": empty_ids,
        "duplicate_parameter_ids": duplicate_ids,
        "stable_total": len(unique_totals) == 1,
        "order_contiguous": all(
            page["offset"] == index * page_size
            for index, page in enumerate(pages)),
        "complete": complete,
        "pages": pages,
    }
    if not complete:
        raise RuntimeError(f"pagination completeness failed: {pagination}")
    return parameters, pagination


def collect_host_metadata(base: str, case_id: str, track_id: str,
                          plugin_id: str, identifier: str, timeout: float,
                          audit: Audit) -> list[dict[str, Any]]:
    """Obtain exact host discreteness metadata through the same read tool.

    The conformance-surface flag changes only response compacting. It does not
    invoke a profile/runtime or write plugin state. Paged rows remain the
    authoritative completeness proof.
    """
    result = require_ok(invoke(base, "plugin.get_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": identifier,
        "include_parameters": True,
        "include_vps_v3_surface": True,
    }, timeout, audit), f"{case_id} host metadata")
    return [host_metadata_row(row) for row in result.get("parameters", [])
            if isinstance(row, dict)]


def merge_host_metadata(parameters: list[dict[str, Any]],
                        metadata: list[dict[str, Any]]) -> dict[str, Any]:
    by_id = {parameter_id(row): row for row in metadata if parameter_id(row)}
    missing: list[str] = []
    for row in parameters:
        pid = parameter_id(row)
        source = by_id.get(pid)
        if source is None:
            missing.append(pid)
            continue
        for key in ("is_boolean", "is_discrete", "num_steps", "numSteps",
                    "all_labels", "allLabels", "labels", "host_controllable"):
            if key in source:
                row[key] = source[key]
        probe = row.get("display_probe")
        if isinstance(probe, dict):
            labels = probe.get("discrete_labels")
            if isinstance(labels, list):
                row["derived_step_count"] = len(labels)
                row["derived_all_labels"] = [
                    str(item.get("label", "")) for item in labels
                    if isinstance(item, dict)
                ]
                row["step_count_source"] = "display_probe.discrete_labels"
        for key in ("num_steps", "numSteps"):
            if key in row:
                row["step_count_source"] = f"host.{key}"
                break
    return {
        "metadata_count": len(metadata),
        "paged_parameter_count": len(parameters),
        "missing_metadata_parameter_ids": missing,
        "complete": not missing and len(metadata) == len(parameters),
    }


def capture_case(base: str, case: dict[str, Any], config: dict[str, Any],
                 output_dir: Path, timeout: float, audit: Audit) -> dict[str, Any]:
    case_id = first_text(case, "id")
    plugin_path = first_text(case, "plugin_path") or first_text(config, "plugin_path")
    vendor = first_text(config, "vendor")
    plugin_format = first_text(config, "format")
    page_size = int(case.get("page_size", config.get("page_size", 128)))
    if not Path(plugin_path).exists():
        raise RuntimeError(f"WaveShell path does not exist: {plugin_path}")
    track_id, plugin_id, identifier, resolution = load_plugin(
        base, case, plugin_path, vendor, plugin_format, timeout, audit, output_dir)
    time.sleep(0.35)
    parameters, pagination = collect_paged_parameters(
        base, case_id, track_id, plugin_id, identifier, page_size,
        timeout, audit, output_dir)
    metadata = collect_host_metadata(
        base, case_id, track_id, plugin_id, identifier, timeout, audit)
    metadata_merge = merge_host_metadata(parameters, metadata)
    if not metadata_merge["complete"]:
        raise RuntimeError(f"host metadata merge failed: {metadata_merge}")

    structural_surface = [{
        "ordinal": index,
        "param_id": parameter_id(row),
        "name": first_text(row, "name", "raw_param_name", "alias"),
        "display_group": first_text(row, "display_group"),
        "domain": row.get("display_domain_candidate"),
        "probe": row.get("display_probe"),
        "is_boolean": row.get("is_boolean"),
        "is_discrete": row.get("is_discrete"),
        "step_count": row.get("num_steps", row.get(
            "numSteps", row.get("derived_step_count"))),
        "all_labels": row.get("all_labels", row.get(
            "allLabels", row.get("derived_all_labels"))),
    } for index, row in enumerate(parameters)]
    payload = {
        "schema_version": "waves.eq_topology_capture.v1",
        "status": "captured",
        "case": case,
        "plugin_path": plugin_path,
        "track_id": track_id,
        "plugin_id": plugin_id,
        "plugin_identifier": identifier,
        "plugin_resolution": resolution,
        "pagination": pagination,
        "metadata_merge": metadata_merge,
        "parameter_count": len(parameters),
        "parameters": parameters,
        "surface_signature_sha256": sha256_json(structural_surface),
        "capture_policy": {
            "plugin_parameter_writes": 0,
            "audio_active_probes": 0,
            "parameter_domain_probe": "read_only_value_to_string",
            "production_eq_summary_invoked": False,
        },
    }
    write_json(output_dir / "captures" / f"{case_id}.json", payload)
    return {
        "id": case_id,
        "plugin_name": first_text(case, "plugin_name"),
        "surface_kind": first_text(case, "surface_kind"),
        "status": "captured",
        "plugin_identifier": identifier,
        "parameter_count": len(parameters),
        "page_count": pagination["page_count"],
        "pagination_complete": pagination["complete"],
        "surface_signature_sha256": payload["surface_signature_sha256"],
    }


def validate_config(config: dict[str, Any]) -> list[dict[str, Any]]:
    if config.get("schema_version") != "waves.eq_topology_census.v1":
        raise RuntimeError("unsupported census fixture schema")
    cases = [case for case in config.get("cases", [])
             if isinstance(case, dict)]
    if len(cases) != 50:
        raise RuntimeError(f"fixture must contain exactly 50 cases, got {len(cases)}")
    ids = [first_text(case, "id") for case in cases]
    names = [first_text(case, "plugin_name") for case in cases]
    if len(set(ids)) != len(ids) or len(set(names)) != len(names):
        raise RuntimeError("fixture IDs and plugin names must be unique")
    kinds = Counter(first_text(case, "surface_kind") for case in cases)
    if kinds != Counter({"eq": 38, "channel_strip": 12}):
        raise RuntimeError(f"fixture scope drift: {dict(kinds)}")
    return cases


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--only", default="")
    args = parser.parse_args()

    config_path = Path(args.config)
    config = json.loads(config_path.read_text(encoding="utf-8"))
    cases = validate_config(config)
    wanted = {item.strip() for item in args.only.split(",") if item.strip()}
    if wanted:
        known = {first_text(case, "id") for case in cases}
        unknown = sorted(wanted - known)
        if unknown:
            raise RuntimeError(f"unknown --only case IDs: {unknown}")
        cases = [case for case in cases if first_text(case, "id") in wanted]

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    base = args.agent_http.rstrip("/")
    health = request_json("GET", base + "/health", None, 10)
    if str(health.get("status", "")).casefold() not in {"ok", "ready"}:
        raise RuntimeError(f"agent health check failed: {health}")

    audit = Audit()
    rows: list[dict[str, Any]] = []
    started = time.time()
    for index, case in enumerate(cases, start=1):
        case_id = first_text(case, "id")
        plugin_name = first_text(case, "plugin_name")
        print(f"[{index:02d}/{len(cases):02d}] {plugin_name}", flush=True)
        try:
            row = capture_case(
                base, case, config, output_dir, args.timeout_sec, audit)
            print(
                f"  captured params={row['parameter_count']} "
                f"pages={row['page_count']}", flush=True)
        except Exception as exc:  # continue: every target needs an explicit state
            row = {
                "id": case_id,
                "plugin_name": plugin_name,
                "surface_kind": first_text(case, "surface_kind"),
                "status": "failed",
                "failure_stage": "resolve_load_or_parameter_capture",
                "error": str(exc),
            }
            write_json(output_dir / "captures" / f"{case_id}.error.json", row)
            print(f"  FAILED: {exc}", flush=True)
        rows.append(row)
        write_json(output_dir / "census_progress.json", {
            "schema_version": "waves.eq_topology_progress.v1",
            "selected_case_count": len(cases),
            "completed_case_count": len(rows),
            "results": rows,
            "audit": audit.summary(),
        })

    captured = sum(row.get("status") == "captured" for row in rows)
    failed = len(rows) - captured
    summary = {
        "schema_version": "waves.eq_topology_census_summary.v1",
        "fixture_sha256": hashlib.sha256(config_path.read_bytes()).hexdigest(),
        "selected_case_count": len(cases),
        "captured_case_count": captured,
        "failed_case_count": failed,
        "duration_seconds": round(time.time() - started, 3),
        "results": rows,
        "audit": audit.summary(),
        "policy": {
            "vendor": first_text(config, "vendor"),
            "format": first_text(config, "format"),
            "plugin_parameter_writes": 0,
            "plugin_alliance_parameter_reads": 0,
            "audio_active_probes": 0,
        },
    }
    write_json(output_dir / "census_summary.json", summary)
    print(
        f"completed captured={captured} failed={failed} "
        f"output={output_dir}", flush=True)
    # Individual failures are reportable census outcomes. A nonzero exit is
    # reserved for infrastructure/configuration failures that prevent a census.
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise
    except Exception as exc:
        print(f"FATAL: {exc}", file=sys.stderr)
        raise SystemExit(1)
