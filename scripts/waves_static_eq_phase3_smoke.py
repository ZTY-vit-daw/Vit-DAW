#!/usr/bin/env python3
"""Bounded live smoke for the production generic static-EQ tool.

The stack must have been launched by Godot. Five positive Waves instances are
mutated through the Agent and then transactionally undone. Three exceptional
instances are read only. The allowlist deliberately excludes audio probes,
learning/profile routes, SPAL/B4, and every non-Waves external-test plug-in.
"""
from __future__ import annotations

import argparse
import json
import math
import time
import urllib.error
import urllib.request
from collections import Counter
from pathlib import Path
from typing import Any

import waves_eq_topology_census as census


ALLOWED_TOOLS = {
    "track.add_audio",
    "plugin.load_to_rack",
    "plugin.get_parameters",
    "plugin_grabber.explain_controls",
    "plugin_grabber.apply_eq_edits",
}
POSITIVE_CASES = {
    "q10_stereo": {
        "nl": "控制当前效果器将3400Hz降低3dB，Q值为0.5",
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                   "gain_db": -3.0, "q": 0.5}],
        "exercise_ref_actions": True,
    },
    "api_550a_stereo": {
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3000.0,
                   "gain_db": -3.0}],
    },
    "api_560_stereo": {
        "edits": [{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0,
                   "gain_db": -3.0}],
    },
    "emo_f2_stereo": {
        "edits": [{"action": "upsert", "shape": "low_cut", "frequency_hz": 80.0}],
    },
    "ssl_ev2_channel_stereo": {
        "edits": [
            {"action": "upsert", "shape": "low_cut", "frequency_hz": 80.0},
            {"action": "upsert", "shape": "high_shelf", "frequency_hz": 10000.0,
             "gain_db": 2.0},
        ],
    },
}
READ_ONLY_CASES = {
    "f6_stereo": "dynamic_sections_excluded_static_cuts_retained",
    "puigtec_eqp1a_stereo": "coupled_analog_network",
    "q_clone_stereo": "stateful_capture_surface",
}


class Audit:
    def __init__(self) -> None:
        self.agent_tools: Counter[str] = Counter()
        self.kernel_commands: Counter[str] = Counter()
        self.chat_calls = 0
        self.events: list[dict[str, Any]] = []

    def agent(self, tool: str, args: dict[str, Any]) -> None:
        if tool not in ALLOWED_TOOLS:
            raise RuntimeError(f"tool is outside phase-3 smoke allowlist: {tool}")
        self.agent_tools[tool] += 1
        self.events.append({"route": "agent", "name": tool,
                            "argument_keys": sorted(args)})

    def kernel(self, command: str) -> None:
        if command != "plugin_search":
            raise RuntimeError(f"kernel command is outside phase-3 smoke allowlist: {command}")
        self.kernel_commands[command] += 1
        self.events.append({"route": "kernel", "name": command})

    def chat(self) -> None:
        self.chat_calls += 1
        self.events.append({"route": "agent", "name": "agent.chat"})

    def summary(self) -> dict[str, Any]:
        return {
            "agent_tools": dict(sorted(self.agent_tools.items())),
            "kernel_commands": dict(sorted(self.kernel_commands.items())),
            "natural_language_chat_count": self.chat_calls,
            "positive_instance_count": len(POSITIVE_CASES),
            "read_only_rejection_instance_count": len(READ_ONLY_CASES),
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


def request_json_allow_error(method: str, url: str, payload: dict[str, Any] | None,
                             timeout: float) -> tuple[int, dict[str, Any]]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"},
        method=method)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            status = response.status
            body = response.read().decode("utf-8", errors="strict")
    except urllib.error.HTTPError as exc:
        status = exc.code
        body = exc.read().decode("utf-8", errors="replace")
    parsed = json.loads(body)
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return status, parsed


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float,
           audit: Audit) -> tuple[int, dict[str, Any]]:
    audit.agent(tool, args)
    return request_json_allow_error("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": True,
        "source": "waves_static_eq_phase3_smoke",
    }, timeout)


def require_invoke_ok(status: int, response: dict[str, Any], label: str) -> dict[str, Any]:
    if status != 200 or str(response.get("status", "")).casefold() not in {"ok", "success"}:
        raise RuntimeError(f"{label}: HTTP {status} response={json.dumps(response, ensure_ascii=False)[:1600]}")
    result = response.get("result")
    if not isinstance(result, dict):
        raise RuntimeError(f"{label}: response omitted result")
    if str(result.get("status", "")).casefold() in {"error", "failed", "rejected"}:
        raise RuntimeError(f"{label}: result rejected: {result}")
    return result


def eq_summary(response: dict[str, Any]) -> dict[str, Any] | None:
    result = response.get("result")
    if not isinstance(result, dict):
        return None
    summary = result.get("eq_band_summary")
    return summary if isinstance(summary, dict) else None


def capability(summary: dict[str, Any], shape: str, action: str) -> bool:
    topology = summary.get("control_topology")
    if not isinstance(topology, dict):
        return False
    for row in topology.get("shape_capabilities", []):
        if isinstance(row, dict) and str(row.get("shape", "")) == shape:
            actions = row.get("actions")
            return bool(actions.get(action)) if isinstance(actions, dict) else False
    return False


def parameter_snapshot(base: str, case_id: str, track_id: str, plugin_id: str,
                       identifier: str, timeout: float, audit: Audit,
                       output_dir: Path, stage: str) -> dict[str, float]:
    parameters, pagination = census.collect_paged_parameters(
        base, f"{case_id}_{stage}", track_id, plugin_id, identifier,
        128, timeout, audit, output_dir)
    if not pagination.get("complete"):
        raise RuntimeError(f"{case_id} {stage}: incomplete parameter pagination")
    out: dict[str, float] = {}
    for row in parameters:
        param_id = census.parameter_id(row)
        value = row.get("normalized_value")
        if param_id and isinstance(value, (int, float)) and math.isfinite(float(value)):
            out[param_id] = float(value)
    return out


def apply_edits(base: str, track_id: str, plugin_id: str, edits: list[dict[str, Any]],
                timeout: float, audit: Audit, label: str) -> tuple[dict[str, Any], dict[str, Any]]:
    status, response = invoke(base, "plugin_grabber.apply_eq_edits", {
        "track_id": track_id, "plugin_id": plugin_id, "atomic": True, "edits": edits,
    }, timeout, audit)
    return require_invoke_ok(status, response, label), response


def extract_executed_apply(turn: dict[str, Any]) -> dict[str, Any] | None:
    for row in turn.get("executed_kernel_reply", []):
        if not isinstance(row, dict):
            continue
        name = str(row.get("command_name", row.get("tool", ""))).casefold()
        if "apply_eq_edits" not in name:
            continue
        result = row.get("result")
        if isinstance(result, dict):
            return result
    return None


def natural_language_apply(base: str, track_id: str, plugin_id: str, plugin_name: str,
                           message: str, timeout: float, audit: Audit) -> tuple[dict[str, Any], dict[str, Any]]:
    audit.chat()
    _, turn = request_json_allow_error("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": f"waves-eq-phase3-{int(time.time() * 1000)}",
        "message": message,
        "context": {"agent_mode": "chat", "selected_track_id": track_id,
                    "selected_plugin_id": plugin_id, "selected_plugin_name": plugin_name},
    }, timeout)
    executed = [str(row.get("command_name", row.get("tool", ""))).strip()
                for row in turn.get("executed_kernel_reply", []) if isinstance(row, dict)]
    allowed_executed = {"plugin_grabber_explain_controls", "plugin_grabber_apply_eq_edits"}
    unexpected = [name for name in executed if name not in allowed_executed]
    if unexpected:
        raise RuntimeError(f"natural-language request escaped the EQ smoke allowlist: {unexpected}")
    result = extract_executed_apply(turn)
    if result is None:
        raise RuntimeError(f"natural-language request did not execute apply_eq_edits; executed={executed} "
                           f"reply={str(turn.get('reply', ''))[:500]}")
    if str(result.get("status", "")).casefold() not in {"exact", "quantized"}:
        raise RuntimeError(f"natural-language apply result is not executable: {result}")
    return result, turn


def validate_apply_result(result: dict[str, Any], expected_edits: int, label: str) -> tuple[str, list[str], set[str]]:
    operation_ref = str(result.get("operation_ref", "")).strip()
    rows = result.get("edits")
    if not operation_ref or not isinstance(rows, list) or len(rows) != expected_edits:
        raise RuntimeError(f"{label}: missing refs or edit rows: {result}")
    control_refs: list[str] = []
    for row in rows:
        ref = str(row.get("control_ref", "")).strip() if isinstance(row, dict) else ""
        if not ref:
            raise RuntimeError(f"{label}: edit omitted control_ref: {row}")
        control_refs.append(ref)
    touched = {str(row.get("param_id", "")).strip() for row in result.get("writes", [])
               if isinstance(row, dict) and str(row.get("param_id", "")).strip()}
    if not touched:
        raise RuntimeError(f"{label}: no touched parameters")
    return operation_ref, control_refs, touched


def undo(base: str, track_id: str, plugin_id: str, operation_ref: str,
         timeout: float, audit: Audit, label: str) -> dict[str, Any]:
    result, _ = apply_edits(base, track_id, plugin_id,
                            [{"action": "undo", "operation_ref": operation_ref}],
                            timeout, audit, label)
    if result.get("action") != "undo" or not (result.get("rollback") or {}).get("verified"):
        raise RuntimeError(f"{label}: undo was not verified: {result}")
    return result


def run_positive(base: str, case: dict[str, Any], spec: dict[str, Any], config: dict[str, Any],
                 timeout: float, audit: Audit, output_dir: Path) -> dict[str, Any]:
    case_id = census.first_text(case, "id")
    plugin_name = census.first_text(case, "plugin_name")
    track_id, plugin_id, identifier, resolution = census.load_plugin(
        base, case, config["plugin_path"], config["vendor"], config["format"],
        timeout, audit, output_dir)
    before = parameter_snapshot(base, case_id, track_id, plugin_id, identifier,
                                timeout, audit, output_dir, "before")
    explain_status, explain_response = invoke(base, "plugin_grabber.explain_controls",
                                               {"track_id": track_id, "plugin_id": plugin_id},
                                               timeout, audit)
    require_invoke_ok(explain_status, explain_response, f"{case_id} explain")
    write_json(output_dir / "responses" / f"{case_id}.explain.json", explain_response)

    if spec.get("nl"):
        applied, apply_response = natural_language_apply(
            base, track_id, plugin_id, plugin_name, str(spec["nl"]), timeout, audit)
        write_json(output_dir / "responses" / f"{case_id}.natural_language.json", apply_response)
    else:
        applied, apply_response = apply_edits(base, track_id, plugin_id, spec["edits"],
                                               timeout, audit, f"{case_id} apply")
        write_json(output_dir / "responses" / f"{case_id}.apply.json", apply_response)
    base_operation, control_refs, touched = validate_apply_result(
        applied, len(spec["edits"]), f"{case_id} apply")
    extra_actions: list[dict[str, Any]] = []

    if spec.get("exercise_ref_actions"):
        modified, _ = apply_edits(base, track_id, plugin_id,
                                  [{"action": "modify", "control_ref": control_refs[0], "gain_db": -4.0}],
                                  timeout, audit, f"{case_id} modify")
        modify_op, _, modify_touched = validate_apply_result(modified, 1, f"{case_id} modify")
        touched.update(modify_touched)
        extra_actions.append({"action": "modify", "result": modified})
        extra_actions.append({"action": "undo_modify", "result": undo(
            base, track_id, plugin_id, modify_op, timeout, audit, f"{case_id} undo modify")})

        disabled, _ = apply_edits(base, track_id, plugin_id,
                                  [{"action": "disable", "control_ref": control_refs[0]}],
                                  timeout, audit, f"{case_id} disable")
        disable_op, _, disable_touched = validate_apply_result(disabled, 1, f"{case_id} disable")
        touched.update(disable_touched)
        extra_actions.append({"action": "disable", "result": disabled})
        extra_actions.append({"action": "undo_disable", "result": undo(
            base, track_id, plugin_id, disable_op, timeout, audit, f"{case_id} undo disable")})

    undo_result = undo(base, track_id, plugin_id, base_operation, timeout, audit,
                       f"{case_id} undo base")
    after = parameter_snapshot(base, case_id, track_id, plugin_id, identifier,
                               timeout, audit, output_dir, "after")
    mismatches = []
    for param_id in sorted(touched):
        if param_id not in before or param_id not in after or abs(before[param_id] - after[param_id]) > 1e-4:
            mismatches.append({"param_id": param_id, "before": before.get(param_id), "after": after.get(param_id)})
    if mismatches:
        raise RuntimeError(f"{case_id}: touched parameters were not restored: {mismatches}")
    record = {
        "case_id": case_id, "plugin_name": plugin_name, "mode": "positive",
        "track_id": track_id, "plugin_id": plugin_id, "identifier": identifier,
        "resolution": resolution, "requested_edits": spec["edits"],
        "status": applied.get("status"), "operation_ref": base_operation,
        "control_refs": control_refs, "touched_parameter_ids": sorted(touched),
        "undo": undo_result, "extra_actions": extra_actions,
        "restored": True, "restoration_mismatches": [],
    }
    write_json(output_dir / "cases" / f"{case_id}.json", record)
    return record


def run_read_only(base: str, case: dict[str, Any], reason: str, config: dict[str, Any],
                  timeout: float, audit: Audit, output_dir: Path) -> dict[str, Any]:
    case_id = census.first_text(case, "id")
    track_id, plugin_id, identifier, resolution = census.load_plugin(
        base, case, config["plugin_path"], config["vendor"], config["format"],
        timeout, audit, output_dir)
    parameter_snapshot(base, case_id, track_id, plugin_id, identifier,
                       timeout, audit, output_dir, "readonly")
    status, response = invoke(base, "plugin_grabber.explain_controls",
                              {"track_id": track_id, "plugin_id": plugin_id}, timeout, audit)
    write_json(output_dir / "responses" / f"{case_id}.explain.json", response)
    summary = eq_summary(response)
    if case_id == "f6_stereo":
        if status != 200 or summary is None or capability(summary, "bell", "upsert") or \
                not capability(summary, "low_cut", "upsert"):
            raise RuntimeError("F6 must reject dynamic Bell while retaining independent static Cut")
        rejection_code = "dynamic_section_excluded"
    else:
        # explain_controls is a general read-only context tool: an unsupported
        # EQ is represented by a successful context pack with no EQ topology.
        # The mutating apply_eq_edits route would return not_static_eq, but this
        # rejection smoke intentionally does not invoke any mutation route.
        if status != 200 or summary is not None:
            raise RuntimeError(f"{case_id} must expose no generic EQ topology: HTTP {status} {response}")
        rejection_code = "not_static_eq"
    record = {
        "case_id": case_id, "plugin_name": census.first_text(case, "plugin_name"),
        "mode": "read_only_rejection", "reason": reason,
        "track_id": track_id, "plugin_id": plugin_id, "identifier": identifier,
        "resolution": resolution, "http_status": status,
        "rejection_code": rejection_code, "parameter_write_count": 0,
    }
    write_json(output_dir / "cases" / f"{case_id}.json", record)
    return record


def run(args: argparse.Namespace) -> int:
    output_dir = Path(args.output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    config = json.loads(Path(args.config).read_text(encoding="utf-8-sig"))
    cases = {census.first_text(row, "id"): row for row in config.get("cases", [])}
    required = set(POSITIVE_CASES) | set(READ_ONLY_CASES)
    missing = sorted(required - set(cases))
    if missing:
        raise RuntimeError(f"smoke cases missing from config: {missing}")
    if str(config.get("vendor", "")).casefold() != "waves":
        raise RuntimeError("vendor guard requires Waves")
    audit = Audit()
    tools_status, tools_response = request_json_allow_error(
        "GET", args.agent_http.rstrip("/") + "/agent/tools", None, args.timeout_sec)
    if tools_status != 200 or "plugin_grabber.apply_eq_edits" not in json.dumps(tools_response):
        raise RuntimeError("running Agent does not advertise plugin_grabber.apply_eq_edits")
    results: list[dict[str, Any]] = []
    for case_id, spec in POSITIVE_CASES.items():
        print(f"positive: {case_id}", flush=True)
        results.append(run_positive(args.agent_http, cases[case_id], spec, config,
                                    args.timeout_sec, audit, output_dir))
    for case_id, reason in READ_ONLY_CASES.items():
        print(f"read-only rejection: {case_id}", flush=True)
        results.append(run_read_only(args.agent_http, cases[case_id], reason, config,
                                     args.timeout_sec, audit, output_dir))
    summary = {
        "schema_version": "waves.static_eq.phase3_live_smoke.v1",
        "status": "passed", "positive_count": len(POSITIVE_CASES),
        "read_only_rejection_count": len(READ_ONLY_CASES),
        "results": results, "audit": audit.summary(), "events": audit.events,
    }
    write_json(output_dir / "summary.json", summary)
    print(f"PASS: positives={len(POSITIVE_CASES)} read_only_rejections={len(READ_ONLY_CASES)}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    return run(parser.parse_args())


if __name__ == "__main__":
    raise SystemExit(main())
