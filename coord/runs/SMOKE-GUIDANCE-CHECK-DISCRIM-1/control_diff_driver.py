#!/usr/bin/env python3
"""Card 252 control-diff driver: byte-level equivalence check of the new
three-state guidance verdict against the REAL pre-change module (extracted
from git at the claim baseline), over a fixture matrix.

For every (render-file state) x (telemetry state) combination the driver
compares:
  - the summary fragment the caller would write, and
  - the stop-condition-2 decision (exit 2 fires or not),
between old check_prompt_guidance (bool) and the new check_prompt_guidance +
apply_prompt_guidance_summary pair. Every combination must be identical
("same") except exactly one: render file expected-but-absent + all-arbitration
telemetry = the intended fix ("intended_change", guidance n/a, no exit 2).
Any other divergence is a regression and fails the run (exit 1).

Usage (from this run directory):
  python control_diff_driver.py --old-module <extracted_baseline.py> \
      --new-module <worktree scripts/automation_llm_route_smoke.py> \
      --output control_diff_report.json
"""

import argparse
import importlib.util
import json
import sys
from pathlib import Path


def load_module(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--old-module", required=True)
    parser.add_argument("--new-module", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    here = Path(__file__).resolve().parent
    fixtures = here / "fixtures"
    old = load_module("smoke_baseline_pre252", Path(args.old_module).resolve())
    new = load_module("smoke_candidate_252", Path(args.new_module).resolve())

    render_cases = [
        ("needle_present", fixtures / "render_needle_present.txt"),
        ("needle_absent", fixtures / "render_needle_absent.txt"),
        ("file_absent", fixtures / "agent_prompt_render_never_created.jsonl"),
        ("not_configured", None),
    ]
    telemetry_cases = [
        ("all_arbitration_251_archive", fixtures / "telemetry_251_g3run1_all_arbitration.jsonl"),
        ("mixed_with_message_loop", fixtures / "telemetry_mixed_with_message_loop.jsonl"),
        ("telemetry_absent", fixtures / "telemetry_absent.jsonl"),
        ("not_configured", None),
    ]

    rows = []
    for render_name, render_path in render_cases:
        for telemetry_name, telemetry_path in telemetry_cases:
            old_rendered = old.check_prompt_guidance(render_path)
            old_summary = {"prompt_guidance_rendered": old_rendered}
            old_stop = (not old_rendered) if render_path is not None else None

            guidance = new.check_prompt_guidance(render_path, telemetry_path)
            new_summary = {}
            new_stop_fires = new.apply_prompt_guidance_summary(new_summary, guidance)
            new_stop = new_stop_fires if render_path is not None else None

            same = (new_summary == old_summary) and (new_stop == old_stop)
            intended_change = (
                guidance["state"] == "n_a_all_arbitration"
                and render_name == "file_absent"
                and telemetry_name == "all_arbitration_251_archive"
            )
            if not same and not intended_change:
                verdict = "REGRESSION"
            elif intended_change:
                verdict = "intended_change"
            else:
                verdict = "same"
            rows.append({
                "render": render_name, "telemetry": telemetry_name,
                "old_rendered": old_rendered, "old_stop_condition_2": old_stop,
                "new_state": guidance["state"],
                "new_summary_fragment": new_summary,
                "new_stop_condition_2": new_stop,
                "verdict": verdict,
            })

    regressions = [row for row in rows if row["verdict"] == "REGRESSION"]
    intended = [row for row in rows if row["verdict"] == "intended_change"]
    same_rows = [row for row in rows if row["verdict"] == "same"]
    report = {
        "scenario": "SMOKE-GUIDANCE-CHECK-DISCRIM-1 control diff (real pre-252 module vs candidate)",
        "old_module": str(Path(args.old_module).resolve()),
        "new_module": str(Path(args.new_module).resolve()),
        "combinations": len(rows),
        "same": len(same_rows),
        "intended_change": len(intended),
        "regressions": len(regressions),
        "rows": rows,
    }
    Path(args.output).resolve().write_text(
        json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"combinations={len(rows)} same={len(same_rows)} "
          f"intended_change={len(intended)} regressions={len(regressions)}")
    for row in intended:
        print(f"intended_change: {row['render']} + {row['telemetry']} -> "
              f"{row['new_state']} summary={json.dumps(row['new_summary_fragment'], ensure_ascii=False)}")
    for row in regressions:
        print(f"REGRESSION: {row['render']} + {row['telemetry']} -> {json.dumps(row, ensure_ascii=False)}")
    if regressions or len(intended) != 1:
        print("[FAIL] control diff found regressions or the intended-change "
              "combination count is not exactly 1")
        return 1
    print("[PASS] all non-all-arbitration combinations byte-equivalent to the "
          "pre-252 module; exactly one intended change (all-arbitration n/a)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
