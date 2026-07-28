#!/usr/bin/env python3
"""Frozen external holdout smoke for the generic static-EQ executor.

The cases and edits below are fixed before parameter inspection.  The runner
never learns from a result: an executable topology is applied and undone;
an unsupported topology is recorded as a safe rejection with zero drift.
"""
from __future__ import annotations

import argparse
import json
import math
from collections import Counter
from pathlib import Path
from typing import Any

import waves_eq_topology_census as census
import waves_static_eq_phase3_smoke as phase3


CASES: tuple[dict[str, Any], ...] = (
    {
        "id": "jeesonic_eq_pro",
        "plugin_name": "Jeesonic EQ Pro",
        "vendor": "jeesonic",
        "category": "Fx|EQ",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Jeesonic EQ Pro.vst3",
        "instruction": "将 3400 Hz 降低 3 dB，Q 值为 0.5",
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                  "gain_db": -3.0, "q": 0.5}],
    },
    {
        "id": "freeeq8",
        "plugin_name": "FreeEQ8",
        "vendor": "TizWildinEntertainment",
        "category": "Fx",
        "plugin_path": r"C:\Program Files\Common Files\VST3\FreeEQ8.vst3\Contents\x86_64-win\FreeEQ8.vst3",
        "instruction": "将 3400 Hz 降低 3 dB，Q 值为 0.5",
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                  "gain_db": -3.0, "q": 0.5}],
    },
    {
        "id": "voxengo_marvel_geq",
        "plugin_name": "Marvel GEQ",
        "vendor": "Voxengo",
        "category": "Fx|EQ",
        "plugin_path": r"C:\Program Files\Common Files\VST3\Marvel GEQ.vst3",
        "instruction": "将最接近 3400 Hz 的频段降低 3 dB",
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                  "gain_db": -3.0}],
    },
    {
        "id": "fabfilter_pro_q_3",
        "plugin_name": "Pro-Q 3",
        "vendor": "FabFilter",
        "category": "Fx|EQ",
        "plugin_path": r"C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3",
        "instruction": "原子批量打开 80 Hz 低切，并在 10 kHz 提升 2 dB 高架",
        "edits": [
            {"action": "upsert", "shape": "low_cut", "frequency_hz": 80.0},
            {"action": "upsert", "shape": "high_shelf", "frequency_hz": 10000.0,
             "gain_db": 2.0},
        ],
    },
    {
        "id": "tdr_nova",
        "plugin_name": "TDR Nova",
        "vendor": "Tokyo Dawn Labs",
        "category": "Fx|EQ",
        "plugin_path": r"C:\Program Files\Common Files\VST3\TDR Nova.vst3",
        "instruction": "将 3400 Hz 降低 3 dB，Q 值为 0.5",
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                  "gain_db": -3.0, "q": 0.5}],
    },
)


class Audit:
    def __init__(self) -> None:
        self.agent_tools: Counter[str] = Counter()
        self.kernel_commands: Counter[str] = Counter()
        self.events: list[dict[str, Any]] = []

    def agent(self, tool: str, args: dict[str, Any]) -> None:
        allowed = {
            "track.add_audio", "plugin.load_to_rack", "plugin.get_parameters",
            "plugin_grabber.explain_controls", "plugin_grabber.apply_eq_edits",
        }
        if tool not in allowed:
            raise RuntimeError(f"tool outside holdout allowlist: {tool}")
        self.agent_tools[tool] += 1
        self.events.append({"route": "agent", "name": tool,
                            "argument_keys": sorted(args)})

    def kernel(self, command: str) -> None:
        if command != "plugin_search":
            raise RuntimeError(f"kernel command outside holdout allowlist: {command}")
        self.kernel_commands[command] += 1
        self.events.append({"route": "kernel", "name": command})

    def summary(self) -> dict[str, Any]:
        return {
            "agent_tools": dict(sorted(self.agent_tools.items())),
            "kernel_commands": dict(sorted(self.kernel_commands.items())),
            "external_holdout_instance_count": len(CASES),
            "audio_probe_count": 0,
            "learning_call_count": 0,
            "profile_call_count": 0,
            "spal_call_count": 0,
            "b4_call_count": 0,
            "plugin_alliance_parameter_read_count": 0,
            "event_count": len(self.events),
        }


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float,
           audit: Audit) -> tuple[int, dict[str, Any]]:
    audit.agent(tool, args)
    return phase3.request_json_allow_error("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": True,
        "source": "static_eq_external_holdout_smoke",
    }, timeout)


def resolve(case: dict[str, Any], timeout: float, audit: Audit,
            output_dir: Path) -> tuple[str, dict[str, Any]]:
    reply = census.kernel_command({
        "cmd": "plugin_search", "query": case["plugin_name"], "limit": 64,
    }, timeout, audit)
    write_json(output_dir / "search" / f"{case['id']}.json", reply)
    exact: dict[str, dict[str, Any]] = {}
    for row in census.search_rows(reply):
        if census.first_text(row, "name", "plugin_name").casefold() != case["plugin_name"].casefold():
            continue
        if census.first_text(row, "manufacturer").casefold() != case["vendor"].casefold():
            continue
        if census.first_text(row, "format", "plugin_format").casefold() != "vst3":
            continue
        if not census.same_plugin_path(case["plugin_path"], census.first_text(
                row, "file_or_identifier", "plugin_path", "path")):
            continue
        identifier = census.first_text(row, "identifier", "plugin_identifier")
        if identifier:
            exact.setdefault(identifier, row)
    if len(exact) != 1:
        raise RuntimeError(f"{case['id']}: exact resolution count={len(exact)} ids={sorted(exact)}")
    identifier, row = next(iter(exact.items()))
    if census.first_text(row, "category").casefold() != case["category"].casefold():
        raise RuntimeError(f"{case['id']}: category drift: {census.first_text(row, 'category')!r}")
    return identifier, row


def load(base: str, case: dict[str, Any], timeout: float, audit: Audit,
         output_dir: Path) -> tuple[str, str, str, dict[str, Any]]:
    identifier, resolution = resolve(case, timeout, audit, output_dir)
    status, response = invoke(base, "track.add_audio", {
        "name": f"External EQ holdout {case['id']}",
    }, timeout, audit)
    track = phase3.require_invoke_ok(status, response, f"{case['id']} add track")
    track_id = census.first_text(track, "track_id", "id")
    status, response = invoke(base, "plugin.load_to_rack", {
        "track_id": track_id, "plugin_path": case["plugin_path"],
        "plugin_name": case["plugin_name"], "plugin_identifier": identifier,
    }, timeout, audit)
    loaded = phase3.require_invoke_ok(status, response, f"{case['id']} load")
    plugin_id = census.first_text(loaded, "plugin_id", "node_id", "id")
    if not track_id or not plugin_id:
        raise RuntimeError(f"{case['id']}: load omitted track/plugin id")
    return track_id, plugin_id, identifier, resolution


def snapshot(base: str, case: dict[str, Any], track_id: str, plugin_id: str,
             identifier: str, timeout: float, audit: Audit, output_dir: Path,
             stage: str) -> dict[str, float]:
    parameters, pagination = census.collect_paged_parameters(
        base, f"{case['id']}_{stage}", track_id, plugin_id, identifier,
        128, timeout, audit, output_dir)
    if not pagination.get("complete"):
        raise RuntimeError(f"{case['id']} {stage}: incomplete pagination")
    values: dict[str, float] = {}
    for row in parameters:
        param_id = census.parameter_id(row)
        value = row.get("normalized_value")
        if param_id and isinstance(value, (int, float)) and math.isfinite(float(value)):
            values[param_id] = float(value)
    return values


def changed(before: dict[str, float], after: dict[str, float], tolerance: float = 1e-4) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for param_id in sorted(set(before) | set(after)):
        left, right = before.get(param_id), after.get(param_id)
        if left is None or right is None or abs(left - right) > tolerance:
            rows.append({"param_id": param_id, "before": left, "after": right})
    return rows


def run_case(base: str, case: dict[str, Any], timeout: float,
             audit: Audit, output_dir: Path) -> dict[str, Any]:
    track_id, plugin_id, identifier, resolution = load(
        base, case, timeout, audit, output_dir)
    before = snapshot(base, case, track_id, plugin_id, identifier,
                      timeout, audit, output_dir, "before")
    explain_status, explain_response = invoke(base, "plugin_grabber.explain_controls", {
        "track_id": track_id, "plugin_id": plugin_id,
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{case['id']}.explain.json", explain_response)

    apply_status, apply_response = invoke(base, "plugin_grabber.apply_eq_edits", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True,
        "edits": case["edits"],
    }, timeout, audit)
    write_json(output_dir / "responses" / f"{case['id']}.apply.json", apply_response)
    result = apply_response.get("result") if isinstance(apply_response.get("result"), dict) else {}
    outcome = str(result.get("status", apply_response.get("status", ""))).casefold()
    executable = apply_status == 200 and outcome in {"exact", "quantized"}
    record: dict[str, Any] = {
        "case_id": case["id"], "plugin_name": case["plugin_name"],
        "instruction": case["instruction"], "requested_edits": case["edits"],
        "track_id": track_id, "plugin_id": plugin_id, "identifier": identifier,
        "resolution": resolution, "explain_http_status": explain_status,
        "apply_http_status": apply_status, "apply_outcome": outcome,
        "executable": executable,
    }
    if executable:
        operation_ref, control_refs, touched = phase3.validate_apply_result(
            result, len(case["edits"]), f"{case['id']} apply")
        applied = snapshot(base, case, track_id, plugin_id, identifier,
                           timeout, audit, output_dir, "applied")
        if not any(param_id in touched for param_id in (set(before) | set(applied))):
            raise RuntimeError(f"{case['id']}: apply reported no observable touched parameter")
        undo_result = phase3.undo(base, track_id, plugin_id, operation_ref,
                                  timeout, audit, f"{case['id']} undo")
        after = snapshot(base, case, track_id, plugin_id, identifier,
                         timeout, audit, output_dir, "after")
        restoration_drift = [row for row in changed(before, after)
                             if row["param_id"] in touched]
        if restoration_drift:
            raise RuntimeError(f"{case['id']}: undo drift: {restoration_drift}")
        record.update({
            "status": outcome, "operation_ref": operation_ref,
            "control_refs": control_refs, "touched_parameter_ids": sorted(touched),
            "undo": undo_result, "restored": True, "restoration_drift": [],
        })
    else:
        after = snapshot(base, case, track_id, plugin_id, identifier,
                         timeout, audit, output_dir, "after_rejection")
        rejection_drift = changed(before, after)
        if rejection_drift:
            raise RuntimeError(f"{case['id']}: rejected request changed parameters: {rejection_drift}")
        error_obj = result.get("error") if isinstance(result.get("error"), dict) else {}
        record.update({
            "status": "safely_rejected", "rejection_code": census.first_text(
                error_obj, "code") or census.first_text(result, "code", "error_code")
                or census.first_text(apply_response, "code", "error"),
            "rejection_message": census.first_text(error_obj, "message")
                or census.first_text(result, "message") or census.first_text(apply_response, "message"),
            "parameter_write_count": 0, "restored": True, "rejection_drift": [],
        })
    write_json(output_dir / "cases" / f"{case['id']}.json", record)
    return record


def run(args: argparse.Namespace) -> int:
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    audit = Audit()
    tool_status, tool_response = phase3.request_json_allow_error(
        "GET", args.agent_http.rstrip("/") + "/agent/tools", None, args.timeout_sec)
    if tool_status != 200 or "plugin_grabber.apply_eq_edits" not in json.dumps(tool_response):
        raise RuntimeError("running Agent does not advertise apply_eq_edits")
    results: list[dict[str, Any]] = []
    for case in CASES:
        print(f"holdout: {case['plugin_name']} :: {case['instruction']}", flush=True)
        results.append(run_case(args.agent_http, case, args.timeout_sec, audit, output_dir))
    summary = {
        "schema_version": "static_eq.external_holdout_live_smoke.v1",
        "status": "passed",
        "executable_count": sum(bool(row["executable"]) for row in results),
        "safe_rejection_count": sum(not bool(row["executable"]) for row in results),
        "results": results, "audit": audit.summary(), "events": audit.events,
    }
    write_json(output_dir / "summary.json", summary)
    print(f"PASS: executable={summary['executable_count']} safe_rejection={summary['safe_rejection_count']}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    return run(parser.parse_args())


if __name__ == "__main__":
    raise SystemExit(main())
