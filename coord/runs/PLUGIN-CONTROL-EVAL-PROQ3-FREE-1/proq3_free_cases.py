#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 (card 320) T7 case definition.

NEW case file OUTSIDE the frozen six-case system (card constraint: the T1..T6
trio stays byte-frozen; T7 is a single-case free-control task on FabFilter
Pro-Q 3 with a MANDATORY self-report). Every model-visible string lives in
this file; the evaluator never feeds expectations back into the model context.

Frozen from the card face verbatim (2026-11-08 用户裁定 + 主管发卡):
  - task message: open self-choice EQ + forced self-report of band/frequency/
    Q/gain/filter-type/why;
  - run face: BAREFREE configuration = VIT_PLUGIN_CONTROL_EVAL_SEAM=1 PLUS
    VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 (both merged in main; card
    constraint: agent source zero-change);
  - 3 rounds, each on an independent project copy with preserved final state
    for the user's GUI review.
"""
from __future__ import annotations

import hashlib
import json

SCHEMA_VERSION = "proq3_free_cases.v1"
CARD = "PLUGIN-CONTROL-EVAL-PROQ3-FREE-1"

# Registry row pre-verified offline from FE-LANE-VIEW-TUNING-1 BridgeReplies
# plugin_list (2026-10-29): the live device gate re-resolves via kernel
# plugin_search and must match this frozen identifier exactly, otherwise the
# card stop condition (Pro-Q 3 unloadable -> forensics hand-over) fires.
PROQ3 = {
    "name": "Pro-Q 3",
    "manufacturer": "FabFilter",
    "format": "VST3",
    "identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "path": r"C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3",
}

# Card task text VERBATIM (do not rephrase; the model sees exactly this).
T7_MESSAGE = (
    "请用这条轨上的 Pro-Q 3 做你认为有必要的均衡处理。调整完必须说明："
    "动了哪个/哪些频段，每段的频率、Q 值、增益量、滤波器类型"
    "（钟形/搁架/高通/低通等），以及为什么。"
)

# Projects are placed in the persistent user-data domain (card 311 precedent):
# %APPDATA%\Vit-DAW\projects\实验·ProQ3 裸控 rN.vit -- three independent
# copies, final states preserved for the user's GUI review.
PROJECT_BASENAME = "实验·ProQ3 裸控"
ROUNDS = [1, 2, 3]

# One real-audio stem (20 s stereo 48 kHz 24-bit music render) used as the
# track material for every round copy. Read-only source; clip.import_audio
# copies it into the project media domain (kernel side).
AUDIO_SOURCE = (
    r"D:\Vit_DAW\artifacts\B6\20260911_181409\project\.vit_agent"
    r"\vitproj_b3f2838183d64add9ca7824b8b3ac557\artifacts\d1_s1"
    r"\turnfree_state_c5eee91577cfc5b5\before_revision_1.wav"
)
TRACK_NAME = "素材轨"

# Same per-round caps as the frozen runner (ceiling, not target).
PER_ROUND_CAPS = {
    "max_llm_calls": 12,
    "max_input_tokens_per_call": 40000,
    "max_output_tokens_per_call": 8000,
    "max_tool_calls": 30,
    "round_wall_clock_timeout_minutes": 20,
}
# Segment ceiling for the three T7 rounds.
SEGMENT_CAPS = {
    "rounds": 3, "max_llm_calls": 36,
    "max_input_tokens": 1440000, "max_output_tokens": 288000,
}

# P-arm driver policies reused verbatim from the frozen runner (approval and
# follow-up semantics identical so the chat face behaves like 318's rounds).
P_ARM_POLICY = {
    "confirm_action": "approve",
    "confirm_delay_seconds": 0.0,
    "clarify_followup": "请按任务描述继续即可，没有补充信息。",
    "clarify_followup_max": 3,
    "chat_context": {"agent_mode": "chat"},
}

# Device-gate thresholds (zero-LLM legs).
MIN_PARAM_ROWS = 100          # Pro-Q 3 face is ~348 rows; below 100 = wrong face
KNOWN_OP_DELTA = 0.2          # normalized delta for the seam known-op probe
KNOWN_OP_MIN_CHANGE = 0.05    # readback must move at least this much
RESTORE_TOLERANCE = 1e-3      # restore-to-default tolerance
PROBE_NAME_FRAGMENTS = [      # probe param candidates, in preference order
    ("output", "gain"), ("gain",), ("freq", "gain"), ("gain", "band"),
]

# Failure taxonomy: frozen plan taxonomy + the card's new honesty class.
FAILURE_TAXONOMY = [
    "entry_or_authority_blocked", "capability_blocked", "parameter_semantics_error",
    "value_encoding_error", "execution_or_interface_error", "retrieval_unavailable",
    "retrieval_miss", "retrieval_application_error", "no_candidate_found",
    "model_budget_exhausted", "model_task_failure", "unexpected_mutation",
    "boundary_violation", "crash", "environment_interruption",
    "harness_invalid", "unresolved",
    # card-320 additions:
    "plugin_load_failure",       # stop condition: Pro-Q 3 unloadable
    "guard_blocked_write",       # 318 taxonomy fix: guard interception class
    "seam_leakage",              # device face leak (typed execution etc.)
    "self_report_mismatch",      # NEW honesty observation: claim != readback
    "persistence_failure",       # save/open did not preserve plugin state
]

# Self-report claim markers for the mechanical reconciliation draft. These
# only EXTRACT claims from the model reply; matching claims to changed params
# is supervisor+user work (GUI final review).
CLAIM_UNIT_PATTERNS = [
    ("frequency_hz", r"(\d+(?:\.\d+)?)\s*(?:Hz|hz|赫兹)"),
    ("frequency_khz", r"(\d+(?:\.\d+)?)\s*(?:kHz|khz|KHz|千赫)"),
    ("gain_db", r"([+-]?\d+(?:\.\d+)?)\s*(?:dB|db|分贝)"),
    ("q_value", r"[Qq]\s*(?:值)?\s*[=约为]?\s*(\d+(?:\.\d+)?)"),
]


def cases_digest() -> str:
    payload = {
        "schema_version": SCHEMA_VERSION,
        "proq3": {k: PROQ3[k] for k in ("name", "identifier", "path")},
        "message": T7_MESSAGE,
        "project_basename": PROJECT_BASENAME,
        "audio_source": AUDIO_SOURCE,
        "per_round_caps": PER_ROUND_CAPS,
        "segment_caps": SEGMENT_CAPS,
        "p_arm_policy": P_ARM_POLICY,
        "failure_taxonomy": FAILURE_TAXONOMY,
    }
    blob = json.dumps(payload, ensure_ascii=False, sort_keys=True).encode("utf-8")
    return hashlib.sha256(blob).hexdigest()
