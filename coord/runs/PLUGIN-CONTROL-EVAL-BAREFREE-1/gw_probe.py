#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-BAREFREE-1 guard-permission live probe (card 318).

Zero-LLM direct /agent/invoke legs proving BOTH halves of the barewrite
seam on the exact B-R1 T3 guard victim (Abbey Road Plates, param_id "1"
Damper, the parameter whose 0.7 write was rejected three times by
typed_transient_shaper_surface_unavailable in the 317 corpus):

  --mode open (run with VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 in the
      agent env): build a fresh T3 fixture, read the Damper baseline,
      invoke set_plugin_param(1, 0.7) -- it must NOT be guard-rejected and
      the independent readback must show normalized ~0.7 (seam-open
      landing + changed readback);

  --mode off (agent restarted WITHOUT the seam env): same build+invoke --
      it MUST be rejected with the typed guard text (guard restored).

The frozen driver trio (blob 259d6b4d/d48ca7cf/57abac09) is imported, not
modified: build_fixture/teardown_fixture/readback_parameters/invoke/
governance_blocked are reused as-is. Fixture is per-probe disposable.
"""
from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / "runner_frozen"))

import plugin_control_eval_driver as frozen  # noqa: E402

CASE = "T3"          # Abbey Road Plates (B-R1 T3 guard victim)
PARAM_ID = "1"       # Damper
WRITE_VALUE = 0.7    # the T3 target encoding (damper step 7)
TOLERANCE = 0.02


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--mode", required=True, choices=["open", "off"])
    ap.add_argument("--agent-http", default="http://127.0.0.1:7878")
    ap.add_argument("--timeout", type=float, default=30.0)
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()

    record: dict = {"mode": args.mode, "case": CASE, "param_id": PARAM_ID,
                    "value": WRITE_VALUE, "probed_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
                    "ok": False}
    fixture: dict = {}
    try:
        fixture = frozen.build_fixture(CASE, args.agent_http, args.timeout,
                                       f"GW{args.mode.upper()}")
        record["track_id"] = fixture.get("track_id")
        record["plugin_id"] = fixture.get("plugin_id")
        before = frozen.readback_parameters(fixture, args.timeout, args.agent_http)
        before_row = frozen.row_by_id(before, PARAM_ID) or {}
        record["value_before"] = frozen._num(before_row.get("normalized_value"))
        resp = frozen.invoke(args.agent_http, "set_plugin_param", {
            "track_id": fixture["track_id"], "plugin_id": fixture["plugin_id"],
            "param_id": PARAM_ID, "value": WRITE_VALUE}, args.timeout)
        result = frozen.result_of(resp)
        if isinstance(result, dict):
            preview = result.get("result_preview") if isinstance(result.get("result_preview"), dict) else {}
            record["parameters_changed"] = preview.get("parameters_changed")
            record["rejection_code"] = preview.get("rejection_code")
        record["write_status"] = str(resp.get("status") or "")
        record["write_error"] = str(resp.get("error") or "")[:400]
        guard_hit = frozen.governance_blocked(json.dumps(resp, ensure_ascii=False))
        record["guard_rejected"] = guard_hit
        if args.mode == "open":
            # seam open: write must land (no guard rejection, no error) and
            # the independent readback must show the changed value
            after = frozen.readback_parameters(fixture, args.timeout, args.agent_http)
            after_row = frozen.row_by_id(after, PARAM_ID) or {}
            record["value_after"] = frozen._num(after_row.get("normalized_value"))
            record["label_after"] = str(after_row.get("value_text") or "")
            landed = (record["write_status"] == "ok" and not guard_hit
                      and record["value_after"] is not None
                      and abs(record["value_after"] - WRITE_VALUE) <= TOLERANCE
                      and record["value_before"] is not None
                      and abs(record["value_after"] - record["value_before"]) > TOLERANCE)
            record["ok"] = bool(landed)
            record["criteria"] = {
                "write_status_ok": record["write_status"] == "ok",
                "not_guard_rejected": not guard_hit,
                "readback_at_target": (record["value_after"] is not None
                                       and abs(record["value_after"] - WRITE_VALUE) <= TOLERANCE),
                "readback_changed": (record["value_after"] is not None
                                     and record["value_before"] is not None
                                     and abs(record["value_after"] - record["value_before"]) > TOLERANCE),
            }
        else:
            # seam closed: the same write must be guard-rejected (typed guard
            # text present) and the value must be unchanged
            after = frozen.readback_parameters(fixture, args.timeout, args.agent_http)
            after_row = frozen.row_by_id(after, PARAM_ID) or {}
            record["value_after"] = frozen._num(after_row.get("normalized_value"))
            rejected = (guard_hit and record["write_status"] != "ok"
                        and record["value_after"] is not None
                        and record["value_before"] is not None
                        and abs(record["value_after"] - record["value_before"]) <= TOLERANCE)
            record["ok"] = bool(rejected)
            record["criteria"] = {
                "guard_rejected": guard_hit,
                "write_status_not_ok": record["write_status"] != "ok",
                "typed_text_present": "typed_transient_shaper_surface_unavailable"
                                      in json.dumps(resp, ensure_ascii=False)
                                      or "typed_" in str(record.get("rejection_code") or ""),
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
