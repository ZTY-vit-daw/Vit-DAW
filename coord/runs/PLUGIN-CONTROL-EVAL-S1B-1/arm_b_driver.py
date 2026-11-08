#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-S1B-1 B-arm round wrapper (card 312).

The frozen 300/307/310 runner trio in runner_frozen/ is reused BYTE-FROZEN
(blob-verified against the 310 snapshot: plan 259d6b4d / cases d48ca7cf /
driver 57abac09). This wrapper adds exactly ONE behavior the frozen driver
does not carry: per-round retrieval cap enforcement for the B arm
(plan.json per_round_caps max_retrieval_searches_B_C=6 /
max_retrieval_fetches_B_C=10), implemented by module-level wrapping of the
frozen driver's count_tool_calls + budget_breached hooks (Python module
globals are looked up at call time, so run_p_round picks these up without a
single byte changed in the frozen files):

  - patched count_tool_calls: counts web.search / web.fetch calls in the
    same agent-log delta text and stashes them per poll (true tool-call
    count still returned unchanged);
  - patched budget_breached: after the frozen checks, a retrieval breach
    returns "retrieval_searches_over_cap" / "retrieval_fetches_over_cap",
    which run_p_round records verbatim as
    stop_reason="model_budget_exhausted:retrieval_*" (same channel as the
    frozen budget stops).

Everything else (s0 / dryround / round / selftest faces, fixtures,
evaluator, records, arm label "P" as the frozen driver writes it) is the
frozen driver unchanged; the B relabeling is an analysis-layer concern
(consolidate_s1b.py), mirroring the 310 A-pure relabel ruling.

Usage: python arm_b_driver.py <frozen driver subcommand and args...>
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
