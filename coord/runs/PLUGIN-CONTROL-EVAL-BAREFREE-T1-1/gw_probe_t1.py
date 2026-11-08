#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 third-layer guard live probe (card 319).

Zero-LLM direct /agent/invoke legs proving the HARNESS half of the
barewrite seam on the exact 318 T1 guard victim (FabFilter Pro-R, param_id
"6" = Mix, the parameter write the 318 corpus recorded three times as
pre_journal_validation_failed stage=broad_mix_observe_first_write_guard).
Unlike the 318 GW probe (T3 typed-guard victim, no user text in context),
this probe carries the T1 task prompt as invoke context so the harness
broad-mix observe-first guard actually engages (broadMixNaturalRequest
matches "mix"; the prompt has no ExplicitPluginRequest verb):

  --mode open (agent env has VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1):
      build a fresh T1 fixture (initial Mix=100%/1.0 frozen needle), read
      the baseline, invoke set_plugin_param(6, 0.35) WITH the natural-mix
      user_message context -- it must NOT be guard-rejected and the
      independent readback must show normalized ~0.35 (third-layer
      seam-open landing + changed readback);

  --mode off (agent restarted WITHOUT the seam env): same build+invoke --
      it MUST be rejected by a guard (the 318-stage broad-mix text and/or
      the chat typed-guard text; which layer answered first is recorded)
      and the value must be unchanged.

The frozen driver trio in ../PLUGIN-CONTROL-EVAL-BAREFREE-1/runner_frozen
(blob 259d6b4d/d48ca7cf/57abac09) is imported, not modified:
build_fixture/teardown_fixture/readback_parameters/invoke/request_json are
reused as-is. Fixture is per-probe disposable.
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

_HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(_HERE.parent / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen"))

import plugin_control_eval_driver as frozen  # noqa: E402

CASE = "T1"                 # FabFilter Pro-R (318 T1 harness-guard victim)
PARAM_ID = "6"              # Mix
WRITE_VALUE = 0.35          # the T1 target encoding (display 35%)
TOLERANCE = 0.02
# The exact T1 task prompt (frozen cases PROMPTS.T1). Engages the harness
# broad-mix guard: contains "mix" (natural request), no explicit-plugin
# verb ("设为" is not in the verb list), so seam-closed state must block.
USER_MESSAGE = "请把 Pro-R 混响的 Mix 设为 35%。"
BROAD_MIX_GUARD_TEXT = "mix.request_observation"


def invoke_with_context(base: str, tool: str, args: dict, context: dict,
                        timeout: float) -> dict:
    """frozen.invoke without context passing; local variant keeps the frozen
    transport (request_json) and only adds the request context field."""
    return frozen.request_json("POST", base.rstrip("/") + "/agent/invoke", {
        "tool": tool, "args": args, "confirmed": False,
        "source": frozen.SOURCE, "context": context,
    }, timeout)


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--mode", required=True, choices=["open", "off"])
    ap.add_argument("--agent-http", default="http://127.0.0.1:7878")
    ap.add_argument("--timeout", type=float, default=30.0)
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()

    record: dict = {"mode": args.mode, "case": CASE, "param_id": PARAM_ID,
                    "value": WRITE_VALUE, "user_message": USER_MESSAGE,
                    "probed_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
                    "ok": False}
    fixture: dict = {}
    try:
        fixture = frozen.build_fixture(CASE, args.agent_http, args.timeout,
                                       f"GT1{args.mode.upper()}")
        record["track_id"] = fixture.get("track_id")
        record["plugin_id"] = fixture.get("plugin_id")
        before = frozen.readback_parameters(fixture, args.timeout, args.agent_http)
        before_row = frozen.row_by_id(before, PARAM_ID) or {}
        record["value_before"] = frozen._num(before_row.get("normalized_value"))
        resp = invoke_with_context(args.agent_http, "set_plugin_param", {
            "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
            "param_id": PARAM_ID, "value": WRITE_VALUE},
            {"user_message": USER_MESSAGE}, args.timeout)
        result = frozen.result_of(resp)
        if isinstance(result, dict):
            preview = result.get("result_preview") if isinstance(result.get("result_preview"), dict) else {}
            record["parameters_changed"] = preview.get("parameters_changed")
            record["rejection_code"] = preview.get("rejection_code")
        record["write_status"] = str(resp.get("status") or "")
        record["write_error"] = str(resp.get("error") or "")[:400]
        resp_text = json.dumps(resp, ensure_ascii=False)
        record["broad_mix_text_present"] = BROAD_MIX_GUARD_TEXT in resp_text
        record["typed_text_present"] = "typed_" in str(record.get("rejection_code") or "") or \
            "typed_transient_shaper_surface_unavailable" in resp_text or "typed_" in record["write_error"]
        guard_hit = record["broad_mix_text_present"] or record["typed_text_present"] \
            or frozen.governance_blocked(resp_text)
        record["guard_rejected"] = guard_hit
        after = frozen.readback_parameters(fixture, args.timeout, args.agent_http)
        after_row = frozen.row_by_id(after, PARAM_ID) or {}
        record["value_after"] = frozen._num(after_row.get("normalized_value"))
        record["label_after"] = str(after_row.get("value_text") or "")
        if args.mode == "open":
            # third-layer seam open: the T1-type write (natural-mix context)
            # must land (no guard rejection, no error) and the independent
            # readback must show the changed value
            landed = (record["write_status"] == "ok" and not guard_hit
                      and record["value_after"] is not None
                      and abs(record["value_after"] - WRITE_VALUE) <= TOLERANCE
                      and record["value_before"] is not None
                      and abs(record["value_after"] - record["value_before"]) > TOLERANCE)
            record["ok"] = bool(landed)
            record["criteria"] = {
                "write_status_ok": record["write_status"] == "ok",
                "not_guard_rejected": not guard_hit,
                "no_broad_mix_text": not record["broad_mix_text_present"],
                "readback_at_target": (record["value_after"] is not None
                                       and abs(record["value_after"] - WRITE_VALUE) <= TOLERANCE),
                "readback_changed": (record["value_after"] is not None
                                     and record["value_before"] is not None
                                     and abs(record["value_after"] - record["value_before"]) > TOLERANCE),
            }
        else:
            # seam closed: the same write must be guard-rejected and the
            # value must be unchanged (interception restored)
            rejected = (guard_hit and record["write_status"] != "ok"
                        and record["value_after"] is not None
                        and record["value_before"] is not None
                        and abs(record["value_after"] - record["value_before"]) <= TOLERANCE)
            record["ok"] = bool(rejected)
            record["criteria"] = {
                "guard_rejected": guard_hit,
                "write_status_not_ok": record["write_status"] != "ok",
                "broad_mix_text_present": record["broad_mix_text_present"],
                "typed_text_present": record["typed_text_present"],
                "value_unchanged": (record["value_after"] is not None
                                    and record["value_before"] is not None
                                    and abs(record["value_after"] - record["value_before"]) <= TOLERANCE),
            }
    except Exception as error:  # FixtureError / transport / harness shape
        record["exception"] = str(error)[:500]
        record["ok"] = False
    finally:
        try:
            frozen.teardown_fixture(args.agent_http, fixture, args.timeout)
            record["fixture_torn_down"] = True
        except Exception as error:
            record["fixture_torn_down"] = False
            record["teardown_error"] = str(error)[:200]
    args.out.write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"mode": record["mode"], "ok": record["ok"],
                      "out": str(args.out)}, ensure_ascii=False))
    return 0 if record["ok"] else 4


if __name__ == "__main__":
    sys.exit(main())
