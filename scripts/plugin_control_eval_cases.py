#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL (card 300 RUNNER-1) frozen case definitions.

Single source of truth for the six evaluation cases. Every value below is
traceable to either:
  - coord/runs/PLUGIN-CONTROL-EVAL-PREP-1/plan.json (branch
    port/plugin-control-eval-prep-1) -- frozen tasks, tolerances, verdict
    rules, failure taxonomy, budgets; or
  - PLUGGRABBER-TIMEDYN-COMPAT-1 run3 parameter-face artifacts
    (coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/
    *.parameters.json) -- plugin identifiers, param ids, initial display
    values, discrete labels, display-domain probe samples.

The evaluator's expectations in this file NEVER enter the tested model's
context (plan ordering_and_isolation.evaluator_isolation). The round message
templates below are the ONLY strings the model sees.

S0 re-freeze: S0 verifies each case's target reachability and discovers the
T4 linkage allowance set on the live stack; its outcome is written as a case
freeze artifact (freeze_version s0) alongside this module. If live evidence
contradicts a frozen expectation here, the runner stops with a fixture
forensics report instead of adjusting expectations silently.
"""
from __future__ import annotations

import hashlib
import json
from typing import Any

SCHEMA_VERSION = "plugin_control_eval_cases.v1"
CARD = "PLUGIN-CONTROL-EVAL-RUNNER-1"

# ---------------------------------------------------------------------------
# Plugin registry (identifiers from PLUGGRABBER-TIMEDYN-COMPAT-1 discovery.json;
# param faces from the matching *.parameters.json artifacts).
# ---------------------------------------------------------------------------

PLUGINS: dict[str, dict[str, str]] = {
    "pro_r": {
        "name": "Pro-R",
        "identifier": "VST3-Pro-R-4bc24482-78c01fa6",
        "artifact": "pro_r.parameters.json",
    },
    "plates_stereo": {
        "name": "Abbey Road Plates Stereo",
        "identifier": "VST3-Abbey Road Plates Stereo-695648d-5e0a41d9",
        "artifact": "abbey_road_plates_stereo.parameters.json",
    },
    "h_delay_stereo": {
        "name": "H-Delay Stereo",
        "identifier": "VST3-H-Delay Stereo-695648d-6e3b68a7",
        "artifact": "h_delay_stereo.parameters.json",
    },
    "ir_l_full_stereo": {
        "name": "IR-L full Stereo",
        "identifier": "VST3-IR-L full Stereo-695648d-3d0f4252",
        "artifact": "ir_l_full_stereo.parameters.json",
    },
}

# Frozen initial (post-load default) expectations per case. Fresh plugin
# instances load with these values (verified in the 2026-10-16 artifacts and
# re-verified per round by the fixture builder; deviation = harness_invalid).
# Row keys: param_id -> {display needle, normalized needle (tolerance 1e-3)}.
INITIAL_STATE_NEEDLES: dict[str, dict[str, dict[str, Any]]] = {
    "T1": {"6": {"value_text_needle": "100%", "normalized_needle": 1.0}},
    "T2": {"1": {"value_text_needle": "100.0%", "normalized_needle": 0.5}},
    "T3": {"1": {"value_text_needle": "2", "normalized_needle": 0.2}},
    "T4": {
        "4": {"value_text_needle": "Host", "normalized_needle": 0.5},
        "0": {"value_text_needle": "375 ms", "normalized_needle": 0.374},
        "17": {"value_text_needle": "375 ms", "normalized_needle": 0.374},
    },
    "T5": {"2": {"value_text_needle": "60", "normalized_needle": 0.3}},
    "T6": {},  # IR-L face enumerated as-is; unsatisfiability is the assertion
}

# Known-op targets for the S0 leg (normalized writes; display expectations).
# T2: display 50.00% requires normalized 0.0 (probe samples 0.0->50.00%);
# the linear-guess 0.25 would read 70.71% and MUST fail the tolerance.
# T4: probe samples 0.25->"251 ms"; tolerance is 250ms +/-10 so 0.25 passes;
# S0 confirms the exact readback before the value freezes.
KNOWN_OP: dict[str, dict[str, Any]] = {
    "T1": {
        "writes": [{"param_id": "6", "normalized": 0.35}],
        "expect": [{"param_id": "6", "display_contains": "35", "normalized_needle": 0.35}],
    },
    "T2": {
        "writes": [{"param_id": "1", "normalized": 0.0}],
        "expect": [{"param_id": "1", "display_contains": "50", "normalized_needle": 0.0}],
    },
    "T3": {
        "writes": [{"param_id": "1", "normalized": 0.7}],
        "expect": [{"param_id": "1", "label_needle": "7", "normalized_needle": 0.7}],
    },
    "T4": {
        "writes": [{"param_id": "0", "normalized": 0.25}],
        "expect": [
            {"param_id": "4", "label_needle": "Host"},
            {"param_id": "0", "display_contains": "25", "normalized_needle": 0.25},
        ],
        "allowance_discovery": {
            "probe_param": "0",
            # rows watched for plugin-internal linkage side effects; the S0
            # observed changed-set (beyond the written param) freezes as the
            # allowance set. Expected candidate: Delay Sec R (id 17) if the
            # plugin links L/R delay.
            "watch": ["17"],
        },
    },
    "T5": {
        "writes": [],
        "expect": [{"param_id": "2", "display_contains": "60"}],
        "zero_mutation": True,
    },
    "T6": {
        "writes": [],
        "expect": [],
        "unsatisfiable": {
            # IR-L full Stereo face (artifact): Dry Level / Wet Level / Bypass /
            # Wet-Dry / Gain -- no decay control; decay is fixed in the IR sample.
            "forbidden_name_fragments": ["decay"],
            "face_param_count_max": 8,
        },
    },
}

# Round message templates (Chinese product usage; the ONLY model-visible task
# strings; semantics frozen from plan.json cases[].task; T2 does not name a
# param id; T6 does not hint unsatisfiability).
ROUND_MESSAGES: dict[str, str] = {
    "T1": "请把 Pro-R 混响的 Mix 设为 35%。",
    "T2": "请把 Pro-R 的衰减速率降到当前值的一半。",
    "T3": "请把 Abbey Road Plates 的 Damper 设为档位 7。",
    "T4": "在 H-Delay 的 Sync 保持 Host 的前提下，把延迟时间设为 250 毫秒。",
    "T5": "检查一下 H-Delay 的 Feedback 是不是 60；如果已经满足就保持不动。",
    "T6": "请把 IR-L 混响的衰减时间设为 2 秒。",
}

# Evaluator rules per case (independent readback face; tolerance semantics
# from plan.json cases[]).
EVAL_RULES: dict[str, dict[str, Any]] = {
    "T1": {"target_param": "6", "display_needle": "35", "display_tolerance": 2.0,
           "pass": "target_display_in_tolerance AND non_target_unchanged"},
    "T2": {"target_param": "1", "display_needle": "50", "display_tolerance": 2.0,
           "linear_guess_normalized": 0.25, "linear_guess_display": "70.71",
           "pass": "target_display_in_tolerance AND non_target_unchanged"},
    "T3": {"target_param": "1", "label_exact": "7", "normalized_needle": 0.7,
           "normalized_tolerance": 0.05,
           "pass": "label_exact AND normalized_in_tolerance AND non_target_unchanged"},
    "T4": {"mode_param": "4", "mode_label": "Host", "target_param": "0",
           "display_needle_ms": 250, "ms_tolerance": 10.0,
           "pass": "mode_label AND target_ms_in_tolerance AND non_target_unchanged_except_allowance"},
    "T5": {"target_param": "2", "recognition_needle": "60",
           "pass": "zero_mutation AND recognition_stated"},
    "T6": {"pass": "zero_writes AND limitation_statement"},
}

# T6 reply-text limitation markers (frozen conservative keyword set; the full
# reply is preserved verbatim for supervisor review either way).
T6_LIMITATION_MARKERS = [
    "无法", "不能", "没有", "不可", "不支持", "无法调节", "固定", "不支持调节",
    "cannot", "can't", "not possible", "no decay", "fixed", "unable",
    "convolution", "ir", "脉冲", "采样",
]

# Frozen P-arm driver policies (card-300 RUNNER freeze; plan pending_approvals
# item 5 + preflight section 4 note).
P_ARM_POLICY = {
    # confirmation cards (execution/web/risk) are approved immediately via the
    # face they arrived on; latency is part of P's real cost measurement.
    "confirm_action": "approve",
    "confirm_delay_seconds": 0.0,
    # a clarification-only reply (no confirmation face, goal still open) gets
    # one frozen follow-up; max 3 follow-ups per round (inside LLM budget).
    "clarify_followup": "请按任务描述继续即可，没有补充信息。",
    "clarify_followup_max": 3,
    # chat context: bare product chat mode; no selected_track/plugin hints.
    "chat_context": {"agent_mode": "chat"},
}

# Per-round caps and segment caps (plan.json budgets; ceilings not targets).
PER_ROUND_CAPS = {
    "max_llm_calls": 12,
    "max_input_tokens_per_call": 40000,
    "max_output_tokens_per_call": 8000,
    "max_tool_calls": 30,
    "round_wall_clock_timeout_minutes": 20,
}
SEGMENT_P_CAPS = {
    "rounds": 6, "max_llm_calls": 72,
    "max_input_tokens": 2880000, "max_output_tokens": 576000,
}
CASE_ORDER = ["T1", "T2", "T3", "T4", "T5", "T6"]

FAILURE_TAXONOMY = [
    "entry_or_authority_blocked", "capability_blocked", "parameter_semantics_error",
    "value_encoding_error", "execution_or_interface_error", "retrieval_unavailable",
    "retrieval_miss", "retrieval_application_error", "no_candidate_found",
    "model_budget_exhausted", "model_task_failure", "unexpected_mutation",
    "boundary_violation", "crash", "environment_interruption",
    "harness_invalid", "unresolved",
]
BEHAVIOR_TAGS = ["direct_guess", "asked_or_searched", "self_probed", "self_corrected"]


def case_plugin(case_id: str) -> str:
    mapping = {"T1": "pro_r", "T2": "pro_r", "T3": "plates_stereo",
               "T4": "h_delay_stereo", "T5": "h_delay_stereo",
               "T6": "ir_l_full_stereo"}
    if case_id not in mapping:
        raise KeyError(f"unknown case {case_id}")
    return mapping[case_id]


def case_family(case_id: str) -> str:
    return "delay" if case_id in {"T4", "T5"} else "reverb"


def cases_digest() -> str:
    """Stable digest of the frozen definitions (goes into run artifacts)."""
    payload = {
        "schema_version": SCHEMA_VERSION,
        "plugins": PLUGINS,
        "initial_state_needles": INITIAL_STATE_NEEDLES,
        "known_op": KNOWN_OP,
        "round_messages": ROUND_MESSAGES,
        "eval_rules": EVAL_RULES,
        "t6_limitation_markers": T6_LIMITATION_MARKERS,
        "p_arm_policy": P_ARM_POLICY,
        "per_round_caps": PER_ROUND_CAPS,
        "segment_p_caps": SEGMENT_P_CAPS,
        "case_order": CASE_ORDER,
    }
    blob = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(blob).hexdigest()
