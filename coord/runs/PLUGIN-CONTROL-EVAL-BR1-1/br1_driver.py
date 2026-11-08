#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-BR1-1 B-R1 arm round wrapper (card 317).

Byte-identical behavior to the 312 arm_b_driver.py wrapper: the frozen
300/307/310 runner trio in runner_frozen/ is reused BYTE-FROZEN (blob-
verified against the 310 snapshot: plan 259d6b4d / cases d48ca7cf /
driver 57abac09) and this wrapper adds exactly ONE behavior the frozen
driver does not carry -- per-round retrieval cap enforcement (plan.json
per_round_caps max_retrieval_searches_B_C=6 / max_retrieval_fetches_B_C=10)
via module-level wrapping of the frozen driver's count_tool_calls +
budget_breached hooks.

The B-R1 thin prompt itself is NOT injected here: the injection lives in
the agent (test-only seam VIT_PLUGIN_CONTROL_EVAL_SEAM_BR1_PROMPT=1 appends
the frozen card-face segment to the ordinary-family system skeleton; see
agent/internal/agentloop/message_loop.go and the paired unit test). The
wrapper-level option was surveyed and rejected in the RUNBOOK (zero-source
system-prompt injection has no external channel; user-message prepending
would contaminate the semantic-entry classifier input -- the 312 finding
made tool-catalog routing a first-class confound, so the classifier input
must stay byte-identical to arm B).

Usage: python br1_driver.py <frozen driver subcommand and args...>
"""

import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / "runner_frozen"))

import plugin_control_eval_driver as frozen  # noqa: E402

# retrieval caps live in the frozen plan.json budgets.per_round_caps (the
# cases module's PER_ROUND_CAPS carries only the five generic caps)
_plan = json.loads((Path(__file__).resolve().parent / "runner_frozen" / "plan.json")
                   .read_text(encoding="utf-8"))
_prc = _plan["budgets"]["per_round_caps"]
RETRIEVAL_SEARCH_CAP = int(_prc["max_retrieval_searches_B_C"])
RETRIEVAL_FETCH_CAP = int(_prc["max_retrieval_fetches_B_C"])

RETRIEVAL_CALL_RE = re.compile(
    r"harness\.invoke total_ms=\d+ tool=(web\.search|web\.fetch)\b")

# per-poll stash: latest counts seen by count_tool_calls for the round that
# is currently running (single-threaded driver; one round per process).
_latest = {"web.search": 0, "web.fetch": 0}


def _count_retrieval(log_text: str) -> None:
    _latest["web.search"] = 0
    _latest["web.fetch"] = 0
    for name in RETRIEVAL_CALL_RE.findall(log_text or ""):
        _latest[name] = _latest.get(name, 0) + 1


_orig_count_tool_calls = frozen.count_tool_calls


def _patched_count_tool_calls(log_text: str) -> int:
    _count_retrieval(log_text)
    return _orig_count_tool_calls(log_text)


_orig_budget_breached = frozen.budget_breached


def _patched_budget_breached(usage, tool_calls):
    breach = _orig_budget_breached(usage, tool_calls)
    if breach:
        return breach
    if _latest["web.search"] > RETRIEVAL_SEARCH_CAP:
        return "retrieval_searches_over_cap"
    if _latest["web.fetch"] > RETRIEVAL_FETCH_CAP:
        return "retrieval_fetches_over_cap"
    return None


frozen.count_tool_calls = _patched_count_tool_calls
frozen.budget_breached = _patched_budget_breached

if __name__ == "__main__":
    sys.exit(frozen.main())
