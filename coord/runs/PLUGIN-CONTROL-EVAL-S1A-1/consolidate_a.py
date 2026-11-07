#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-S1A-1 consolidation (card 307, offline).

Reads the frozen driver's round records from <run-dir>/rounds (task rounds
T1..T6 x repeat 1..3; probe T1_r0 excluded by construction -- it lives in
<run-dir>/probe) and the frozen P-baseline segment summary, then writes
arm_a_summary.json:

  - 18-row A-arm score table (arm relabeled A per RUNBOOK declaration: the
    frozen 300 driver hardcodes arm="P" in raw records, which are preserved
    verbatim and never rewritten);
  - usage totals vs plan segment_caps.A_star;
  - per-case A-vs-P comparison (P = 300 segment-p, single repeat);
  - failure-taxonomy and behavior-tag tallies.

Offline only; never talks to the agent/kernel. Exit 0 on complete
consolidation, 1 if records are missing/incomplete (partial output still
written for forensics).
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

CASES = ["T1", "T2", "T3", "T4", "T5", "T6"]
CAPS_A = {"rounds": 18, "max_llm_calls": 216,
          "max_input_tokens": 8640000, "max_output_tokens": 1728000}

# tool-face extraction (device finding 2026-11-07: the seam set (S1-S4) does
# not remove typed plugin_grabber adapter tools from the ordinary catalog;
# per-round tool faces are recorded so each round's adapter-vs-bare choice
# is on the record for supervisor interpretation)
HARNESS_TOOL_RE = re.compile(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)")
LOOP_TOOL_RE = re.compile(r"message_loop\.tool ms=\d+ goal=\S+ tool=([A-Za-z0-9_.\-]+)")
TYPED_ADAPTER_RE = re.compile(
    r"^(plugin_grabber[._](inspect|apply|explain)|apply_(reverb|delay|eq|compressor|"
    r"limiter|de_esser|gate_expander|transient_shaper|multiband))")
BARE_WRITE_RE = re.compile(r"^(plugin\.set_parameter|set_plugin_param)$")
BARE_READ_RE = re.compile(r"^(plugin\.get_parameters|get_plugin_parameters)$")


def tool_faces(delta: str) -> dict:
    loop_calls = LOOP_TOOL_RE.findall(delta or "")
    harness_calls = HARNESS_TOOL_RE.findall(delta or "")
    all_calls = loop_calls + harness_calls
    return {
        "loop_tool_sequence": loop_calls,
        "harness_tool_sequence": harness_calls,
        "typed_adapter_calls": [c for c in all_calls if TYPED_ADAPTER_RE.match(c)],
        "typed_apply_calls": [c for c in all_calls
                              if TYPED_ADAPTER_RE.match(c) and
                              ("apply" in c or c.startswith("plugin_grabber.apply"))],
        "bare_write_calls": [c for c in all_calls if BARE_WRITE_RE.match(c)],
        "bare_read_calls": [c for c in all_calls if BARE_READ_RE.match(c)],
        "write_path": ("typed_adapter" if any(TYPED_ADAPTER_RE.match(c) and "apply" in c
                                              for c in all_calls)
                       else ("bare" if any(BARE_WRITE_RE.match(c) for c in all_calls)
                             else "none_observed")),
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True, type=Path)
    ap.add_argument("--p-summary", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()

    rounds_dir = args.run_dir / "rounds"
    rows = []
    missing = []
    incomplete = False
    for case in CASES:
        for repeat in (1, 2, 3):
            rec_path = rounds_dir / f"{case}_r{repeat}" / "round_record.json"
            if not rec_path.exists():
                missing.append(f"{case}_r{repeat}")
                continue
            rec = json.loads(rec_path.read_text(encoding="utf-8"))
            u = rec.get("usage") or {}
            v = rec.get("verdict") or {}
            faces = tool_faces(rec.get("agent_log_delta"))
            rows.append({
                "case": case, "arm": "A", "repeat": repeat,
                "raw_arm_field": rec.get("arm"),
                "conversation_id": rec.get("conversation_id"),
                "first_action": rec.get("first_action"),
                "tool_faces": faces,
                "final_verdict": v.get("final_verdict"),
                "failure_tags": v.get("failure_tags") or [],
                "failure_detail": v.get("detail"),
                "behavior_tags": rec.get("behavior_tags") or [],
                "mutation_count": v.get("mutation_count"),
                "mutation_audit": v.get("mutation_audit"),
                "llm_calls": int(u.get("llm_calls") or 0),
                "input_tokens": int(u.get("input_tokens") or 0),
                "output_tokens": int(u.get("output_tokens") or 0),
                "cache_read_tokens": int(u.get("cache_read_tokens") or 0),
                "cost": "missing (price snapshot pending, plan budgets.monetary)",
                "stop_reason": rec.get("stop_reason"),
                "round_dir": str(rec_path.parent.resolve()),
            })

    totals = {"llm_calls": sum(r["llm_calls"] for r in rows),
              "input_tokens": sum(r["input_tokens"] for r in rows),
              "output_tokens": sum(r["output_tokens"] for r in rows),
              "cache_read_tokens": sum(r["cache_read_tokens"] for r in rows)}
    within_caps = (totals["llm_calls"] <= CAPS_A["max_llm_calls"]
                   and totals["input_tokens"] <= CAPS_A["max_input_tokens"]
                   and totals["output_tokens"] <= CAPS_A["max_output_tokens"])

    # per-case tallies
    per_case = {}
    for case in CASES:
        c_rows = [r for r in rows if r["case"] == case]
        verdicts = [r["final_verdict"] for r in c_rows]
        per_case[case] = {
            "repeats": len(c_rows),
            "pass_count": sum(1 for v in verdicts if v == "pass"),
            "verdicts": verdicts,
            "failure_tags": sorted({t for r in c_rows for t in r["failure_tags"]}),
            "behavior_tags": sorted({t for r in c_rows for t in r["behavior_tags"]}),
            "write_paths": [r["tool_faces"]["write_path"] for r in c_rows],
            "typed_adapter_used_rounds": sum(1 for r in c_rows
                                             if r["tool_faces"]["typed_adapter_calls"]),
            "bare_write_used_rounds": sum(1 for r in c_rows
                                          if r["tool_faces"]["bare_write_calls"]),
            "llm_calls": sum(r["llm_calls"] for r in c_rows),
        }

    # A vs P comparison
    comparison = []
    p_summary = {}
    if args.p_summary.exists():
        p_summary = json.loads(args.p_summary.read_text(encoding="utf-8"))
        p_rounds = {(r.get("case_id")): r for r in p_summary.get("rounds", [])
                    if r.get("final_verdict")}
    else:
        p_rounds = {}
    for case in CASES:
        a_rows = [r for r in rows if r["case"] == case]
        a_pass = per_case[case]["pass_count"]
        p_row = p_rounds.get(case) or {}
        comparison.append({
            "case": case,
            "A_pass": f"{a_pass}/{len(a_rows)}" if a_rows else "n/a",
            "A_verdicts": [r["final_verdict"] for r in a_rows],
            "A_failure_tags": sorted({t for r in a_rows for t in r["failure_tags"]}),
            "A_write_paths": [r["tool_faces"]["write_path"] for r in a_rows],
            "P_verdict": p_row.get("final_verdict"),
            "P_failure_tags": p_row.get("failure_tags") or [],
            "P_llm_calls": p_row.get("llm_calls"),
            "A_llm_calls_total": per_case[case]["llm_calls"],
            "delta_note": ("A repeat-level variance visible in A_verdicts; "
                           "single P repeat per plan run_counts.P"),
        })

    summary = {
        "schema_version": "plugin_control_eval_segment_a_summary.v1",
        "card": "PLUGIN-CONTROL-EVAL-S1A-1",
        "run_dir": str(args.run_dir.resolve()),
        "arm": "A (seam open VIT_PLUGIN_CONTROL_EVAL_SEAM=1; relabel per RUNBOOK: raw driver records keep arm=\"P\")",
        "model": "deepseek-flash (product default config)",
        "score_table": rows,
        "totals": totals, "caps": CAPS_A, "within_caps": within_caps,
        "per_case": per_case,
        "a_vs_p": comparison,
        "p_baseline_source": str(args.p_summary.resolve()),
        "missing_rounds": missing,
        "notes": [
            "probe round (T1 repeat 0, device verification carrier) excluded from scores",
            "cost fields remain missing pending plan budgets.monetary price snapshot",
            "single round = single sample; 3 repeats do not constitute a stability claim (plan section 6)",
        ],
    }
    if missing:
        incomplete = True

    args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2),
                        encoding="utf-8")

    # console table
    hdr = (f"{'case':4} {'rep':3} {'verdict':8} {'write_path':13} {'fail_tags':28} "
           f"{'mut':3} {'calls':5} {'in_tok':8} {'out_tok':7} {'stop':24}")
    print(hdr)
    for r in rows:
        print(f"{r['case']:4} {r['repeat']:<3} {str(r['final_verdict']):8} "
              f"{r['tool_faces']['write_path']:13} "
              f"{','.join(r['failure_tags'])[:28]:28} "
              f"{str(r['mutation_count']):3} {r['llm_calls']:<5} "
              f"{r['input_tokens']:<8} {r['output_tokens']:<7} "
              f"{str(r['stop_reason'])[:24]:24}")
    print(f"totals: {totals} within_caps={within_caps}")
    print("A vs P:")
    for c in comparison:
        print(f"  {c['case']}: A={c['A_pass']} vs P={c['P_verdict']} "
              f"(A_tags={c['A_failure_tags'] or '-'})")
    if missing:
        print(f"MISSING ROUNDS: {missing}")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
