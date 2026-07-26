#!/usr/bin/env python3
"""Three-model Plugin Grabber EQ smoke test.

The fixture contains one isolated plugin instance per EQ model:
  fixed_slot_adjustable: TDR Nova
  fixed_freq:            Voxengo Marvel GEQ
  free_floating:         FabFilter Pro-Q 3

Each case verifies the live model summary, deterministic Go-side band choice,
the expected write roles, parameter readback, and one explicit NL Agent turn.
"""
from __future__ import annotations

import argparse
import json
import math
import sys
import time
import urllib.request
from typing import Any


def request_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    req = urllib.request.Request(url, data=data,
                                 headers={"Content-Type": "application/json; charset=utf-8"},
                                 method=method)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        value = json.loads(resp.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def invoke(base: str, tool: str, args: dict, timeout: float, confirmed: bool = True) -> dict:
    return request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": confirmed,
        "source": "eq_three_models_smoke",
    }, timeout)


def chat(base: str, conversation_id: str, message: str, context: dict, timeout: float) -> dict:
    return request_json("POST", base.rstrip("/") + "/agent/chat", {
        "conversation_id": conversation_id, "message": message, "context": context,
    }, timeout)


def result_map(resp: dict) -> dict:
    value = resp.get("result", resp)
    return value if isinstance(value, dict) else {}


def require_ok(resp: dict, label: str) -> dict:
    if str(resp.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label}: status={resp.get('status')} error={resp.get('error', '')}")
    inner = result_map(resp)
    if str(inner.get("status", "ok")).lower() in {"error", "failed"}:
        raise RuntimeError(f"{label}: inner status={inner.get('status')} error={inner.get('error', '')}")
    return inner


def explain(base: str, case: dict, timeout: float) -> dict:
    res = require_ok(invoke(base, "plugin_grabber.explain_controls", {
        "track_id": case["track_id"], "plugin_id": case["plugin_id"],
    }, timeout, confirmed=False), f"{case['name']} explain_controls")
    summary = res.get("eq_band_summary")
    if not isinstance(summary, dict):
        raise RuntimeError(f"{case['name']}: explain_controls returned no eq_band_summary")
    if summary.get("eq_model") != case["model"]:
        raise RuntimeError(f"{case['name']}: expected model {case['model']!r}, got {summary.get('eq_model')!r}")
    return {"result": res, "summary": summary}


def number(value: Any) -> float | None:
    if value is None:
        return None
    try:
        return float(str(value).strip().split()[0])
    except (TypeError, ValueError):
        return None


def read_parameters(base: str, case: dict, timeout: float) -> dict[str, dict]:
    res = require_ok(invoke(base, "plugin.get_parameters", {
        "track_id": case["track_id"], "plugin_id": case["plugin_id"],
        "include_parameters": True,
    }, timeout, confirmed=False), f"{case['name']} get_plugin_parameters")
    rows = res.get("parameters")
    if not isinstance(rows, list):
        raise RuntimeError(f"{case['name']}: parameter readback returned no parameters")
    out = {}
    for row in rows:
        if not isinstance(row, dict):
            continue
        pid = str(row.get("param_id", row.get("id", ""))).strip()
        if pid:
            out[pid] = row
    return out


def selected_band(eq_result: dict) -> dict:
    value = eq_result.get("selected_band")
    if not isinstance(value, dict) or not str(value.get("band", "")).strip():
        raise RuntimeError(f"set_eq_point returned no selected_band: {eq_result}")
    return value


def writes_by_role(eq_result: dict) -> dict[str, dict]:
    out = {}
    for row in eq_result.get("writes", []):
        if isinstance(row, dict) and row.get("role"):
            out[str(row["role"])] = row
    return out


def assert_display(params: dict[str, dict], pid: str, expected: float, tolerance: float, label: str) -> None:
    row = params.get(str(pid))
    if row is None:
        raise RuntimeError(f"{label}: parameter {pid} missing from readback")
    got = number(row.get("value_text"))
    if got is None or abs(got - expected) > tolerance:
        raise RuntimeError(f"{label}: expected ~{expected}, got {row.get('value_text')!r} (param {pid})")


def direct_case(base: str, case: dict, timeout: float) -> dict:
    info = explain(base, case, timeout)
    summary = info["summary"]
    if case["model"] == "fixed_slot_adjustable":
        bands = summary.get("bands", [])
        target = min(bands, key=lambda b: abs(float(b.get("current_freq_hz", 0)) - 3400.0))
        args = {"track_id": case["track_id"], "plugin_id": case["plugin_id"],
                "freq_hz": 3400.0, "gain_db": -3.0, "q": 0.5}
        expected_roles = {"freq", "gain", "q"}
        expected = {"band": target.get("band"), "freq_pid": target.get("freq_param_id"),
                    "gain_pid": target.get("gain_param_id"), "q_pid": target.get("q_param_id")}
    elif case["model"] == "fixed_freq":
        bands = summary.get("bands", [])
        target = min(bands, key=lambda b: abs(float(b.get("fixed_freq_hz", 0)) - 3400.0))
        args = {"track_id": case["track_id"], "plugin_id": case["plugin_id"],
                "freq_hz": 3400.0, "gain_db": -3.0}
        expected_roles = {"gain"}
        expected = {"band": target.get("band"), "gain_pid": target.get("gain_param_id"),
                    "fixed_freq_hz": float(target.get("fixed_freq_hz", 0))}
    else:
        available = summary.get("available_slots", [])
        if not available:
            raise RuntimeError("free_floating summary has no available slot")
        target = available[0]
        args = {"track_id": case["track_id"], "plugin_id": case["plugin_id"],
                "freq_hz": 3400.0, "gain_db": -3.0, "q": 0.5}
        expected_roles = {"used", "freq", "gain", "q"}
        expected = {"band": target.get("band"), "freq_pid": target.get("freq_param_id"),
                    "gain_pid": target.get("gain_param_id"), "q_pid": target.get("q_param_id"),
                    "used_pid": target.get("used_param_id")}

    if not expected.get("band"):
        raise RuntimeError(f"{case['name']}: selected target band is incomplete: {expected}")
    eq_result = require_ok(invoke(base, "plugin_grabber.set_eq_point", args, timeout),
                           f"{case['name']} set_eq_point")
    actual = selected_band(eq_result)
    if actual.get("band") != expected["band"]:
        raise RuntimeError(f"{case['name']}: expected band {expected['band']}, got {actual.get('band')}")
    roles = set(writes_by_role(eq_result))
    if not expected_roles.issubset(roles):
        raise RuntimeError(f"{case['name']}: expected write roles {sorted(expected_roles)}, got {sorted(roles)}")
    if case["model"] == "fixed_freq" and roles - {"gain"}:
        raise RuntimeError(f"{case['name']}: fixed_freq wrote unexpected roles {sorted(roles - {'gain'})}")

    params = read_parameters(base, case, timeout)
    if case["model"] == "fixed_freq":
        assert_display(params, expected["gain_pid"], -3.0, 1.0, case["name"])
    else:
        assert_display(params, expected["freq_pid"], 3400.0, 550.0, case["name"])
        assert_display(params, expected["gain_pid"], -3.0, 1.0, case["name"])
        assert_display(params, expected["q_pid"], 0.5, 0.12, case["name"])
    return {"band": actual.get("band"), "roles": sorted(roles), "model": summary["eq_model"]}


def find_set_eq_result(resp: dict) -> dict | None:
    for row in resp.get("executed_kernel_reply", []):
        if not isinstance(row, dict):
            continue
        name = str(row.get("command_name", row.get("tool", ""))).lower()
        if "set_eq_point" in name:
            value = row.get("result", row)
            return value if isinstance(value, dict) else None
    return None


def nl_case(base: str, case: dict, timeout: float) -> dict:
    context = {"agent_mode": "chat", "selected_track_id": case["track_id"],
               "selected_plugin_id": case["plugin_id"],
               "selected_plugin_name": case["plugin_name"]}
    response = chat(base, f"eq_three_{case['name']}_{int(time.time())}", case["message"], context, timeout)
    eq_result = find_set_eq_result(response)
    if eq_result is None:
        raise RuntimeError(f"{case['name']}: NL did not execute set_eq_point; status={response.get('goal_status')!r}")
    if str(eq_result.get("eq_model", "")) != case["model"]:
        raise RuntimeError(f"{case['name']}: NL result model={eq_result.get('eq_model')!r}")
    actual = selected_band(eq_result)
    if not actual.get("band"):
        raise RuntimeError(f"{case['name']}: NL result has no selected band")
    if case["model"] != "fixed_freq":
        if number(eq_result.get("q")) is None or abs(float(eq_result["q"]) - 0.5) > 1e-9:
            raise RuntimeError(f"{case['name']}: NL result did not preserve Q=0.5: {eq_result.get('q')!r}")
    return {"band": actual.get("band"), "model": eq_result.get("eq_model")}


def run(args: argparse.Namespace) -> int:
    base = args.agent_http.rstrip("/")
    cases = [
        {"name": "tdr_nova", "plugin_name": "TDR Nova", "track_id": args.tdr_track_id,
         "plugin_id": args.tdr_plugin_id, "model": "fixed_slot_adjustable",
         "message": "用 TDR Nova 将 3400 Hz 降低 3 dB，Q 设置为 0.5。"},
        {"name": "marvel_geq", "plugin_name": "Marvel GEQ", "track_id": args.marvel_track_id,
         "plugin_id": args.marvel_plugin_id, "model": "fixed_freq",
         "message": "用 Marvel GEQ 将最接近 3400 Hz 的固定频段降低 3 dB。"},
        {"name": "pro_q3", "plugin_name": "Pro-Q 3", "track_id": args.proq_track_id,
         "plugin_id": args.proq_plugin_id, "model": "free_floating",
         "message": "Use Pro-Q 3 to cut 3400 Hz by 3 dB with Q 0.5."},
    ]
    failures = []
    print("== three EQ model smoke test")
    try:
        health = request_json("GET", base + "/health", None, 10)
        if str(health.get("status", "")).lower() not in {"ok", "ready"}:
            raise RuntimeError(f"agent health={health}")
    except Exception as exc:
        print(f"FAIL: agent health: {exc}")
        return 1

    for case in cases:
        print(f"\n-- {case['name']} ({case['model']}) --")
        try:
            direct = direct_case(base, case, args.timeout_sec)
            print(f"  direct ok: model={direct['model']} band={direct['band']} roles={direct['roles']}")
        except Exception as exc:
            print(f"  direct FAIL: {exc}")
            failures.append((case["name"] + " direct", str(exc)))
    for case in cases:
        try:
            nl = nl_case(base, case, args.timeout_sec)
            print(f"  NL ok: model={nl['model']} band={nl['band']}")
        except Exception as exc:
            print(f"  NL FAIL: {exc}")
            failures.append((case["name"] + " NL", str(exc)))

    if failures:
        print(f"\n{len(failures)}/6 checks failed:")
        for label, error in failures:
            print(f"  - {label}: {error}")
        return 1
    print("\nAll 6 three-model smoke checks passed.")
    return 0


def main() -> None:
    parser = argparse.ArgumentParser(description="three-model EQ smoke test")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--tdr-track-id", required=True)
    parser.add_argument("--tdr-plugin-id", required=True)
    parser.add_argument("--marvel-track-id", required=True)
    parser.add_argument("--marvel-plugin-id", required=True)
    parser.add_argument("--proq-track-id", required=True)
    parser.add_argument("--proq-plugin-id", required=True)
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    sys.exit(run(parser.parse_args()))


if __name__ == "__main__":
    main()
