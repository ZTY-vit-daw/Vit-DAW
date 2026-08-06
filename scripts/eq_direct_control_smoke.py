#!/usr/bin/env python3
"""End-to-end smoke test for direct NL plugin control via set_plugin_param.

Tests the deterministic typed EQ path against live parameter observations:
  user NL message → LLM reads all_parameters + display_domain_candidate
  → LLM picks param_id and computes normalized value
  → set_plugin_param executes
  → readback confirms normalized_value changed in the expected direction

Usage:
    python scripts/eq_direct_control_smoke.py --track-id 1007 --plugin-id 1013
    python scripts/eq_direct_control_smoke.py --agent-http http://127.0.0.1:7878 \\
        --track-id 1007 --plugin-id 1013 --timeout-sec 120
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

# The NL request we test.  Deliberately concrete so we don't need
# the acoustic observation layer (which isn't the target of this test).
NL_REQUEST = "用TDR Nova把3.4kHz降3dB"


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
        {"tool": tool, "args": args, "source": "eq_direct_control_smoke"},
        timeout,
    )


def chat(base: str, conversation_id: str, message: str, context: dict, timeout: float) -> dict:
    return request_json(
        "POST", base.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation_id, "message": message, "context": context},
        timeout,
    )


# ---------------------------------------------------------------------------
# Step helpers
# ---------------------------------------------------------------------------

def step_verify_read_channel(base: str, track_id: str, plugin_id: str, timeout: float) -> dict:
    """
    Verify that A3 landed: explain_controls must return all_parameters with
    display_domain_candidate for most parameters.
    Returns a summary dict.
    """
    resp = invoke_tool(base, "plugin_grabber.explain_controls",
                       {"track_id": track_id, "plugin_id": plugin_id}, timeout)
    result = resp.get("result", resp)
    if not isinstance(result, dict):
        raise RuntimeError(f"explain_controls unexpected response shape: {str(resp)[:400]}")

    status = str(result.get("status", "")).lower()
    if status not in {"ok", "success", ""}:
        raise RuntimeError(f"explain_controls failed: status={status} msg={result.get('message','')}")

    all_params = result.get("all_parameters")
    if not isinstance(all_params, list) or len(all_params) == 0:
        raise RuntimeError(
            "explain_controls returned no all_parameters — A3 read-channel change did not reach the LLM. "
            f"Keys present: {sorted(result.keys())}"
        )

    note = result.get("all_parameters_note", "")
    if not note:
        raise RuntimeError("all_parameters_note missing — context pack change not applied")

    with_domain = sum(1 for p in all_params if isinstance(p, dict) and p.get("display_domain_candidate"))
    coverage = with_domain / len(all_params)

    return {
        "parameter_count": len(all_params),
        "with_domain_candidate": with_domain,
        "domain_coverage_pct": round(coverage * 100, 1),
    }


def step_snapshot_params(base: str, track_id: str, plugin_id: str, timeout: float) -> dict[str, float]:
    """
    Read current normalized_value for every parameter.
    Returns {param_id: normalized_value}.
    """
    resp = invoke_tool(base, "get_plugin_parameters", {
        "track_id": track_id,
        "plugin_id": plugin_id,
        "include_parameters": True,
    }, timeout)
    result = resp.get("result", resp)
    params = result.get("parameters") if isinstance(result, dict) else None
    if not isinstance(params, list) or len(params) == 0:
        raise RuntimeError(
            f"get_plugin_parameters (include_parameters=true) returned no parameters. "
            f"result keys: {sorted(result.keys()) if isinstance(result, dict) else type(result)}"
        )
    snapshot: dict[str, float] = {}
    for p in params:
        if not isinstance(p, dict):
            continue
        pid = str(p.get("param_id", p.get("id", ""))).strip()
        nv = p.get("normalized_value")
        if pid and nv is not None:
            try:
                snapshot[pid] = float(nv)
            except (TypeError, ValueError):
                pass
    if not snapshot:
        raise RuntimeError("get_plugin_parameters: could not extract any param_id/normalized_value pairs")
    return snapshot


def step_nl_request(
    base: str,
    track_id: str,
    plugin_id: str,
    plugin_name: str,
    nl_message: str,
    timeout: float,
    conv_id: str,
) -> tuple[bool, str, dict]:
    """
    Send the NL message and, if a proposal is returned, confirm it.
    Returns (succeeded: bool, detail: str, last_resp: dict).
    """
    context: dict[str, Any] = {
        "agent_mode": "chat",
        "selected_track_id": track_id,
        "selected_plugin_id": plugin_id,
        "selected_plugin_name": plugin_name,
    }

    print(f"  turn 1: \"{nl_message}\"")
    turn1 = chat(base, conv_id, nl_message, context, timeout)

    if os.environ.get("EQ_DIRECT_SMOKE_DEBUG"):
        import tempfile
        dump_path = os.path.join(tempfile.gettempdir(), f"eq_direct_turn1_{conv_id}.json")
        with open(dump_path, "w", encoding="utf-8") as fh:
            json.dump(turn1, fh, ensure_ascii=False, indent=2)
        print(f"  [debug] turn1 saved to {dump_path}")

    goal_status1 = str(turn1.get("goal_status", "")).lower()
    error1 = str(turn1.get("error", ""))
    reply1 = str(turn1.get("reply", ""))

    # Check if execution already happened in turn1 (no proposal needed)
    if goal_status1 in {"done", "completed", "succeeded", "ok", "success"}:
        print(f"  turn 1 executed directly (goal_status={goal_status1})")
        return True, f"executed in turn1 goal_status={goal_status1}", turn1

    # Check for proposal (B4 or mix_treatment)
    needs_confirm = bool(turn1.get("needs_confirmation"))
    wf_data = turn1.get("workflow_data") or {}
    is_b4 = isinstance(wf_data, dict) and bool(wf_data.get("session_id"))
    workflow_name = str(turn1.get("workflow", "")).lower()
    is_mix_treatment = workflow_name == "mix_treatment" and needs_confirm

    if needs_confirm or is_b4 or is_mix_treatment:
        print(f"  turn 1 → proposal (b4={is_b4} mix_treatment={is_mix_treatment})")

        if is_b4 and isinstance(wf_data, dict):
            session_id = str(wf_data.get("session_id", ""))
            proposal_id = str(wf_data.get("proposal_id", ""))
            proposal_revision = int(wf_data.get("proposal_revision", 1))
            action_set_hash = str(wf_data.get("action_set_hash", ""))
            project_cut_hash = str(wf_data.get("project_cut_hash", ""))
            confirm_context: dict[str, Any] = {
                "agent_mode": "chat",
                "capability_session_id": session_id,
                "capability_approval_decision": {
                    "schema_version": "approval_decision.v1",
                    "kind": "approve",
                    "proposal_id": proposal_id,
                    "proposal_revision": proposal_revision,
                    "action_set_hash": action_set_hash,
                    "project_cut_hash": project_cut_hash,
                    "user_text": "确认",
                    "confidence": "explicit",
                },
            }
        else:
            confirm_context = {"agent_mode": "chat"}

        print("  turn 2: 确认，执行这个方案")
        turn2 = chat(base, conv_id, "确认，执行这个方案", confirm_context, timeout)

        if os.environ.get("EQ_DIRECT_SMOKE_DEBUG"):
            import tempfile
            dump_path = os.path.join(tempfile.gettempdir(), f"eq_direct_turn2_{conv_id}.json")
            with open(dump_path, "w", encoding="utf-8") as fh:
                json.dump(turn2, fh, ensure_ascii=False, indent=2)
            print(f"  [debug] turn2 saved to {dump_path}")

        goal_status2 = str(turn2.get("goal_status", "")).lower()
        error2 = str(turn2.get("error", ""))
        reply2 = str(turn2.get("reply", ""))

        if goal_status2 in {"done", "completed", "succeeded", "ok", "success"}:
            return True, f"executed after confirm goal_status={goal_status2}", turn2

        reply_lower = reply2.lower()
        if any(kw in reply_lower for kw in ["已执行", "已应用", "applied", "executed", "完成", "成功"]):
            return True, f"execution indicated in reply: {reply2[:120]}", turn2

        if goal_status2 in {"failed", "error", "blocked"} or error2:
            return False, f"failed after confirm: goal_status={goal_status2} error={error2[:200]}", turn2

        # ambiguous — let readback decide
        return True, f"confirm turn ambiguous goal_status={goal_status2!r}; checking readback", turn2

    # Turn 1 was neither success nor proposal
    if goal_status1 in {"failed", "error", "blocked"} or error1:
        return False, f"blocked in turn1: goal_status={goal_status1} error={error1[:200]} reply={reply1[:200]}", turn1

    reply_lower1 = reply1.lower()
    if any(kw in reply_lower1 for kw in ["已执行", "已应用", "applied", "executed", "完成", "成功"]):
        return True, f"execution indicated in turn1 reply: {reply1[:120]}", turn1

    # Let readback decide
    return True, f"turn1 ambiguous goal_status={goal_status1!r}; checking readback", turn1


def step_verify_change(
    before: dict[str, float],
    after: dict[str, float],
    expect_decrease: bool = True,
) -> tuple[bool, str, list[dict]]:
    """
    Compare before/after snapshots.
    Returns (changed: bool, detail: str, changed_params: list).
    At least one parameter must have changed by more than 0.005 normalized.
    """
    MIN_DELTA = 0.005
    changed = []
    for pid, before_val in before.items():
        after_val = after.get(pid)
        if after_val is None:
            continue
        delta = after_val - before_val
        if abs(delta) >= MIN_DELTA:
            changed.append({
                "param_id": pid,
                "before": round(before_val, 6),
                "after": round(after_val, 6),
                "delta": round(delta, 6),
                "direction": "down" if delta < 0 else "up",
            })

    if not changed:
        unchanged_count = len([p for p in before if p in after])
        return False, (
            f"No parameter changed by ≥{MIN_DELTA} normalized. "
            f"Compared {unchanged_count} parameters — the NL instruction did not write to the plugin."
        ), []

    if expect_decrease:
        # For "降3dB" we expect at least one param went down
        decreasing = [c for c in changed if c["delta"] < 0]
        if not decreasing:
            return False, (
                f"Parameters changed but none decreased — "
                f"expected a gain reduction. Changed: {changed}"
            ), changed

    return True, f"{len(changed)} parameter(s) changed", changed


# ---------------------------------------------------------------------------
# Main smoke
# ---------------------------------------------------------------------------

def run(args: argparse.Namespace) -> int:
    base = args.agent_http.rstrip("/")
    timeout = float(args.timeout_sec)
    track_id = args.track_id
    plugin_id = args.plugin_id
    plugin_name = args.plugin_name or "TDR Nova"

    print("== EQ Direct Control Smoke (new path: all_parameters + set_plugin_param)")
    print(f"agent : {base}")
    print(f"track : {track_id}  plugin: {plugin_id}")
    print()

    failures: list[str] = []

    # ---- wait for agent ----
    try:
        wait_agent(base, min(timeout, 30.0))
        print("ok: agent healthy")
    except RuntimeError as exc:
        print(f"FAIL: {exc}")
        return 1

    # ---- Step 1: verify A3 read channel ----
    print("\n[Step 1] Verify read channel (all_parameters via explain_controls)")
    try:
        rc = step_verify_read_channel(base, track_id, plugin_id, timeout)
        print(f"  ok: {rc['parameter_count']} parameters, "
              f"{rc['with_domain_candidate']} with display_domain_candidate "
              f"({rc['domain_coverage_pct']}%)")
    except RuntimeError as exc:
        msg = f"Read channel check FAILED: {exc}"
        print(f"  FAIL: {msg}")
        failures.append(msg)
        # Not fatal — continue to see if chat still works

    # ---- Step 2: write-before snapshot ----
    print("\n[Step 2] Snapshot parameters (before)")
    try:
        before = step_snapshot_params(base, track_id, plugin_id, timeout)
        print(f"  ok: captured {len(before)} normalized values")
    except RuntimeError as exc:
        print(f"  FAIL: {exc}")
        failures.append(f"Before-snapshot failed: {exc}")
        print("\nCannot continue without before snapshot.")
        _print_summary(failures, 0)
        return 1

    # ---- Step 3: NL request ----
    print(f"\n[Step 3] NL request: \"{NL_REQUEST}\"")
    conv_id = f"eq_direct_smoke_{int(time.time())}"
    last_resp: dict = {}
    try:
        chat_ok, chat_detail, last_resp = step_nl_request(
            base, track_id, plugin_id, plugin_name,
            NL_REQUEST, timeout, conv_id,
        )
        print(f"  {'ok' if chat_ok else 'FAIL'}: {chat_detail}")
        if not chat_ok:
            failures.append(f"NL chat failed: {chat_detail}")
    except Exception as exc:
        print(f"  FAIL (exception): {exc}")
        failures.append(f"NL chat exception: {exc}")
        chat_ok = False

    # ---- Step 3b: verify D3 (no success card on failure) ----
    if not chat_ok and isinstance(last_resp, dict):
        cards = last_resp.get("project_result_cards")
        if cards:
            msg = f"D3 regression: failed turn emitted {len(cards)} project_result_card(s)"
            print(f"  FAIL: {msg}")
            failures.append(msg)
        else:
            print("  ok: no spurious project_result_cards on failed turn (D3)")

    # ---- Step 4: write-after snapshot ----
    print("\n[Step 4] Snapshot parameters (after)")
    time.sleep(0.4)  # let the kernel settle
    try:
        after = step_snapshot_params(base, track_id, plugin_id, timeout)
        print(f"  ok: captured {len(after)} normalized values")
    except RuntimeError as exc:
        print(f"  FAIL: {exc}")
        failures.append(f"After-snapshot failed: {exc}")
        _print_summary(failures, 0)
        return 1

    # ---- Step 5: verify change ----
    print("\n[Step 5] Verify parameter changed (readback)")
    changed_ok, change_detail, changed_params = step_verify_change(before, after, expect_decrease=True)
    if changed_ok:
        print(f"  ok: {change_detail}")
        for cp in changed_params:
            print(f"    param_id={cp['param_id']}  {cp['before']:.4f} → {cp['after']:.4f}  "
                  f"(Δ{cp['delta']:+.4f} {cp['direction']})")
    else:
        print(f"  FAIL: {change_detail}")
        failures.append(f"Parameter readback check failed: {change_detail}")

    print()
    return _print_summary(failures, len(changed_params) if changed_ok else 0)


def _print_summary(failures: list[str], changed_count: int) -> int:
    if not failures:
        print(f"PASS — EQ direct control smoke passed. {changed_count} parameter(s) changed.")
        return 0
    print(f"FAIL — {len(failures)} check(s) failed:")
    for f in failures:
        print(f"  - {f}")
    return 1


def main() -> None:
    parser = argparse.ArgumentParser(description="EQ direct control end-to-end smoke test")
    parser.add_argument("--agent-http", default=AGENT_HTTP_DEFAULT)
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    parser.add_argument("--plugin-name", default="TDR Nova")
    parser.add_argument("--timeout-sec", type=float, default=120.0)
    sys.exit(run(parser.parse_args()))


if __name__ == "__main__":
    main()
