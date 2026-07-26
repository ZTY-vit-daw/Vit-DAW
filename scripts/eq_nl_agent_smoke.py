#!/usr/bin/env python3
"""Smoke test for plugin_grabber.set_eq_point — two parts.

Part 1 — Direct invoke (deterministic, no LLM):
  Calls plugin_grabber.set_eq_point directly and verifies:
  - correct band selected by Go (B3 for 3400 Hz)
  - readback via explain_controls shows frequency/gain/Q updated on that band

Part 2 — NL end-to-end:
  NL message → single-turn chat → LLM calls explain_controls + set_eq_point
  Verifies set_eq_point appeared (not apply_control or bare set_plugin_param).
  If selected_band.band is present in the result, also checks it is correct.

Usage:
    python scripts/eq_nl_agent_smoke.py --track-id 1007 --plugin-id 1015
    python scripts/eq_nl_agent_smoke.py --agent-http http://127.0.0.1:7878 \\
        --track-id 1007 --plugin-id 1015 --timeout-sec 120
"""
from __future__ import annotations
import argparse
import json
import os
import sys
import time
import urllib.request
from typing import Any

AGENT_HTTP_DEFAULT = "http://127.0.0.1:7878"

# TDR Nova default band positions (after fresh project load):
# B1=80 Hz, B2=400 Hz, B3=2200 Hz, B4=6000 Hz
# The smoke intentionally covers the one requested BDD scenario only.

DIRECT_INVOKE_CASES = [
    {"freq_hz": 3400, "gain_db": -3.0, "q": 0.5, "expect_band": "B3",
     "label": "3400 Hz → -3 dB, Q 0.5 on B3 (2200 Hz nearest; dist 1200 vs B4's 2600)"},
]

# NL messages — each explicitly names TDR Nova so the planner takes the
# direct plugin-control path, not the observe-first mixing path.
NL_REQUESTS = [
    {"message": "用 TDR Nova 将 3400 Hz 降低 3 dB，Q 设置为 0.5。", "expect_band": "B3",
     "freq_hz": 3400.0, "gain_db": -3.0, "q": 0.5},
]

# ---------------------------------------------------------------------------
# HTTP helpers
# ---------------------------------------------------------------------------

def request_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode()
    req = urllib.request.Request(
        url, data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8", errors="replace")
    result = json.loads(body)
    if not isinstance(result, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return result


def wait_agent(base: str, timeout: float) -> None:
    deadline = time.time() + timeout
    last_err: Exception | None = None
    while time.time() < deadline:
        try:
            h = request_json("GET", base.rstrip("/") + "/health", None, 3.0)
            if str(h.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:
            last_err = exc
        time.sleep(0.5)
    raise RuntimeError(f"Agent not ready at {base}: {last_err}")


def invoke_tool(base: str, tool: str, args: dict, timeout: float) -> dict:
    return request_json(
        "POST", base.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "confirmed": True, "source": "eq_set_eq_point_smoke"},
        timeout,
    )


def chat(base: str, conversation_id: str, message: str, context: dict, timeout: float) -> dict:
    return request_json(
        "POST", base.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation_id, "message": message, "context": context},
        timeout,
    )


# ---------------------------------------------------------------------------
# Result extraction helpers
# ---------------------------------------------------------------------------

def is_blocked(resp: dict) -> bool:
    status = str(resp.get("goal_status", "")).lower()
    err = str(resp.get("error", "")).lower()
    return status == "failed" or "被拒绝" in err or "blocked" in status


def find_in_executed(resp: dict, *command_name_fragments: str) -> dict | None:
    """Return first executed_kernel_reply row whose command_name matches any fragment."""
    for row in resp.get("executed_kernel_reply", []):
        if not isinstance(row, dict):
            continue
        cn = str(row.get("command_name", row.get("tool", ""))).lower()
        for frag in command_name_fragments:
            if frag.lower() in cn:
                return row
    return None


def extract_set_eq_result(resp: dict) -> dict | None:
    """Return the result map from plugin_grabber.set_eq_point if it ran."""
    row = find_in_executed(resp, "set_eq_point")
    if row is None:
        return None
    result = row.get("result", row)
    return result if isinstance(result, dict) else None


def band_from_eq_result(eq_result: dict) -> str | None:
    sel = eq_result.get("selected_band")
    if isinstance(sel, dict):
        return str(sel.get("band", "")).strip() or None
    return None


# ---------------------------------------------------------------------------
# Part 1 — Direct invoke test
# ---------------------------------------------------------------------------

def run_direct_invoke_test(
    base: str, track_id: str, plugin_id: str,
    freq_hz: float, gain_db: float, q: float, expect_band: str, label: str, timeout: float,
) -> dict[str, Any]:
    result: dict[str, Any] = {"label": label, "pass": False, "error": None}

    resp = invoke_tool(base, "plugin_grabber.set_eq_point", {
        "track_id": track_id, "plugin_id": plugin_id,
        "freq_hz": freq_hz, "gain_db": gain_db, "q": q,
    }, timeout)

    # The invoke response is the InvokeResponse directly; .result is our payload.
    eq_result = resp.get("result", {})
    if not isinstance(eq_result, dict):
        eq_result = {}

    outer_status = str(resp.get("status", "")).lower()
    if outer_status == "error":
        result["error"] = f"invoke returned error: {resp.get('error', '')}"
        return result

    inner_status = str(eq_result.get("status", "ok")).lower()
    if inner_status not in {"ok", "success", ""}:
        result["error"] = f"set_eq_point inner status={inner_status}: {eq_result.get('error', '')}"
        return result

    if "q" not in eq_result:
        result["error"] = "set_eq_point result has no Q target (existing implementation did not handle Q)"
        return result
    try:
        if abs(float(eq_result["q"]) - q) > 1e-9:
            result["error"] = f"set_eq_point Q target mismatch: expected {q}, got {eq_result.get('q')}"
            return result
    except (TypeError, ValueError):
        result["error"] = f"set_eq_point returned non-numeric Q: {eq_result.get('q')!r}"
        return result
    writes = eq_result.get("writes", [])
    if not any(isinstance(w, dict) and w.get("role") == "q" for w in writes):
        result["error"] = "set_eq_point result has no Q write evidence"
        return result

    actual_band = band_from_eq_result(eq_result)
    result["actual_band"] = actual_band

    if actual_band != expect_band:
        result["error"] = (
            f"wrong band selected: expected {expect_band}, got {actual_band!r}. "
            f"selected_band={eq_result.get('selected_band')}"
        )
        return result

    # Readback: call explain_controls and verify the band's freq/gain/Q updated.
    try:
        ec = invoke_tool(base, "plugin_grabber.explain_controls",
                         {"track_id": track_id, "plugin_id": plugin_id}, timeout)
        eq_summary = (ec.get("result") or ec).get("eq_band_summary", {})
        bands = eq_summary.get("bands", []) if isinstance(eq_summary, dict) else []
        for b in bands:
            if str(b.get("band", "")) == expect_band:
                rb_freq = b.get("current_freq_hz")
                rb_gain = b.get("current_gain_db")
                if rb_freq is not None and abs(float(rb_freq) - freq_hz) / max(freq_hz, 1) > 0.15:
                    result["error"] = f"readback freq mismatch: expected ~{freq_hz}, got {rb_freq} on {expect_band}"
                    return result
                if rb_gain is not None and abs(float(rb_gain) - gain_db) > 1.5:
                    result["error"] = f"readback gain mismatch: expected ~{gain_db} dB, got {rb_gain} dB on {expect_band}"
                    return result
                q_param_id = b.get("q_param_id")
                all_parameters = eq_summary.get("all_parameters", []) if isinstance(eq_summary, dict) else []
                if q_param_id and isinstance(ec.get("result"), dict):
                    all_parameters = (ec["result"] or {}).get("all_parameters", all_parameters)
                rb_q = None
                for p in all_parameters if isinstance(all_parameters, list) else []:
                    if str(p.get("param_id", p.get("id", ""))) == str(q_param_id):
                        rb_q = p.get("value_text", p.get("valueText"))
                        break
                if rb_q is None:
                    result["error"] = f"readback Q missing for {expect_band} (q_param_id={q_param_id!r})"
                    return result
                try:
                    rb_q_num = float(str(rb_q).strip().split()[0])
                except (TypeError, ValueError):
                    result["error"] = f"readback Q is not numeric: {rb_q!r} on {expect_band}"
                    return result
                if abs(rb_q_num - q) > 0.08:
                    result["error"] = f"readback Q mismatch: expected ~{q}, got {rb_q} on {expect_band}"
                    return result
                result["readback_freq"] = rb_freq
                result["readback_gain"] = rb_gain
                result["readback_q"] = rb_q
                break
    except Exception as exc:
        result["readback_warn"] = f"readback explain_controls failed: {exc}"

    result["pass"] = True
    return result


# ---------------------------------------------------------------------------
# Part 2 — NL end-to-end test
# ---------------------------------------------------------------------------

def run_one_nl_request(
    base: str, track_id: str, plugin_id: str, plugin_name: str,
    nl_message: str, expect_band: str | None,
    freq_hz: float | None, gain_db: float | None, q: float | None,
    timeout: float, conv_id: str,
) -> dict[str, Any]:
    result: dict[str, Any] = {
        "nl_message": nl_message, "pass": False,
        "expect_band": expect_band, "actual_band": None, "error": None,
    }

    context: dict[str, Any] = {
        "agent_mode": "chat",
        "selected_track_id": track_id,
        "selected_plugin_id": plugin_id,
        "selected_plugin_name": plugin_name,
    }

    if os.environ.get("EQ_NL_SMOKE_DEBUG"):
        import tempfile
        turn = chat(base, conv_id, nl_message, context, timeout)
        dump = os.path.join(tempfile.gettempdir(), f"eq_nl_turn_{conv_id}.json")
        with open(dump, "w", encoding="utf-8") as fh:
            json.dump(turn, fh, ensure_ascii=False, indent=2)
        print(f"  [debug] raw response saved to {dump}")
    else:
        turn = chat(base, conv_id, nl_message, context, timeout)

    if is_blocked(turn):
        result["error"] = f"blocked: {turn.get('error', turn.get('reply', ''))[:300]}"
        return result

    # Preferred path: set_eq_point was called
    eq_result = extract_set_eq_result(turn)
    if eq_result is not None:
        actual_band = band_from_eq_result(eq_result)
        result["actual_band"] = actual_band
        if expect_band and actual_band and actual_band != expect_band:
            result["error"] = f"wrong band: expected {expect_band}, got {actual_band}"
            return result
        if freq_hz is not None and float(eq_result.get("freq_hz", float("nan"))) != freq_hz:
            result["error"] = f"wrong frequency target: expected {freq_hz}, got {eq_result.get('freq_hz')}"
            return result
        if gain_db is not None and float(eq_result.get("gain_db", float("nan"))) != gain_db:
            result["error"] = f"wrong gain target: expected {gain_db}, got {eq_result.get('gain_db')}"
            return result
        if q is not None and float(eq_result.get("q", float("nan"))) != q:
            result["error"] = f"wrong Q target: expected {q}, got {eq_result.get('q')}"
            return result
        result["requested_q"] = q
        result["pass"] = True
        return result

    # Detect old paths that should no longer be used
    if find_in_executed(turn, "apply_control", "effect_control"):
        result["error"] = "LLM used apply_control instead of set_eq_point (old path — profile dependency not eliminated)"
        return result
    if find_in_executed(turn, "set_plugin_param", "set_parameter"):
        result["error"] = "LLM used set_plugin_param directly without set_eq_point (band selection not guaranteed)"
        return result

    reply_snippet = str(turn.get("reply", ""))[:300]
    result["error"] = (
        f"set_eq_point not found in executed_kernel_reply. "
        f"goal_status={turn.get('goal_status')!r} reply={reply_snippet!r}"
    )
    return result


# ---------------------------------------------------------------------------
# Main runner
# ---------------------------------------------------------------------------

def run(args: argparse.Namespace) -> int:
    base = args.agent_http.rstrip("/")
    timeout = float(args.timeout_sec)
    track_id = args.track_id
    plugin_id = args.plugin_id
    plugin_name = args.plugin_name or "TDR Nova"

    print(f"== set_eq_point smoke test")
    print(f"agent:  {base}")
    print(f"track:  {track_id}  plugin: {plugin_id}")

    try:
        wait_agent(base, min(timeout, 30.0))
        print("ok: agent healthy")
    except RuntimeError as exc:
        print(f"FAIL: {exc}")
        return 1

    failures: list[dict] = []

    # ------------------------------------------------------------------
    # Part 1: Direct invoke — deterministic band selection check
    # ------------------------------------------------------------------
    print(f"\n-- Part 1: direct invoke ({len(DIRECT_INVOKE_CASES)} cases) --")
    for case in DIRECT_INVOKE_CASES:
        label = case["label"]
        print(f"  [{label}]")
        r = run_direct_invoke_test(
            base, track_id, plugin_id,
            case["freq_hz"], case["gain_db"], case["q"], case["expect_band"], label, timeout,
        )
        if r["pass"]:
            rb_freq = r.get("readback_freq")
            rb_gain = r.get("readback_gain")
            rb_q = r.get("readback_q")
            print(f"    ok: band={r.get('actual_band')} readback_freq={rb_freq} readback_gain={rb_gain} readback_q={rb_q}")
            if r.get("readback_warn"):
                print(f"    warn: {r['readback_warn']}")
        else:
            print(f"    FAIL: {r.get('error', 'unknown')}")
            failures.append(r)

    # ------------------------------------------------------------------
    # Part 2: NL end-to-end — single-turn, set_eq_point path
    # ------------------------------------------------------------------
    print(f"\n-- Part 2: NL end-to-end ({len(NL_REQUESTS)} requests) --")
    for i, req in enumerate(NL_REQUESTS):
        nl = req["message"]
        expect = req.get("expect_band")
        conv_id = f"eq_smoke_{int(time.time())}_{i}"
        print(f"  [{i+1}/{len(NL_REQUESTS)}] \"{nl}\"")
        r = run_one_nl_request(base, track_id, plugin_id, plugin_name,
                               nl, expect, req.get("freq_hz"), req.get("gain_db"), req.get("q"), timeout, conv_id)
        if r["pass"]:
            print(f"    ok: band={r.get('actual_band')!r} (expected {expect!r})")
        else:
            print(f"    FAIL: {r.get('error', 'unknown')}")
            failures.append({"label": nl, **r})
        time.sleep(0.5)

    # ------------------------------------------------------------------
    # Summary
    # ------------------------------------------------------------------
    total = len(DIRECT_INVOKE_CASES) + len(NL_REQUESTS)
    print()
    if failures:
        print(f"{len(failures)}/{total} test(s) FAILED:")
        for f in failures:
            print(f"  - {f.get('label', f.get('nl_message', '?'))}: {f.get('error', 'unknown')}")
        return 1
    print(f"All {total} tests passed.")
    return 0


def main() -> None:
    parser = argparse.ArgumentParser(description="set_eq_point smoke test")
    parser.add_argument("--agent-http", default=AGENT_HTTP_DEFAULT)
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    parser.add_argument("--plugin-name", default="")
    parser.add_argument("--timeout-sec", type=float, default=120.0)
    sys.exit(run(parser.parse_args()))


if __name__ == "__main__":
    main()
