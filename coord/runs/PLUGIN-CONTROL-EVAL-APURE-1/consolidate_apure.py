#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-APURE-1 consolidation (card 310, offline).

Reads the frozen driver's round records from <run-dir>/rounds (task rounds
T1..T6 x repeat 1..3; probe T1_r0 excluded by construction -- it lives in
<run-dir>/probe), the frozen P-baseline segment summary, and the 307 A-arm
summary, then writes arm_apure_summary.json:

  - 18-row A-pure score table (arm relabeled "A-pure" per RUNBOOK: the
    frozen 300 driver hardcodes arm="P" in raw records, preserved verbatim);
  - usage totals vs plan segment_caps.A_star (carried verbatim);
  - THREE-WAY per-case comparison A-pure vs A(307) vs P -- the card's core
    deliverable;
  - typed-adapter device gate: after seam S5 the plugin_grabber family is
    not in the model catalog; ANY executed typed adapter call (or
    write_path=typed_adapter) in ANY round = device invalid flag
    (card clause: "write_path typed_adapter = S5 failure"); verdicts are
    NOT rewritten, the flag escalates to the supervisor;
  - T4 linked-pair annotation per 307 acceptance ruling: verdicts stay as
    evaluated (frozen rules); an analysis-layer flag notes the suspected
    stereo-linked-pair rule defect -- rule fix is deferred to before the
    B/C batches, NOT this card.

Offline only; never talks to the agent/kernel. Exit 0 on complete
consolidation, 1 if records are missing/incomplete, 2 if any round shows an
executed typed adapter call (device invalid; partial output still written).
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
        "bare_write_calls": [c for c in all_calls if BARE_WRITE_RE.match(c)],
        "bare_read_calls": [c for c in all_calls if BARE_READ_RE.match(c)],
        "write_path": ("typed_adapter" if any(TYPED_ADAPTER_RE.match(c) and "apply" in c
                                              for c in all_calls)
                       else ("typed_adapter_nonapply" if any(TYPED_ADAPTER_RE.match(c)
                                                             for c in all_calls)
                             else ("bare" if any(BARE_WRITE_RE.match(c) for c in all_calls)
                                   else "none_observed"))),
    }


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True, type=Path)
    ap.add_argument("--p-summary", required=True, type=Path)
    ap.add_argument("--a307-summary", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    ap.add_argument("--seam-corpus", default="", type=str,
                    help="path to seam_verify_corpus.json; rounds whose "
                         "conversation appears in leaking payload rows get a "
                         "device flag (verdicts are NOT rewritten)")
    args = ap.parse_args()

    corpus: dict = {}
    if args.seam_corpus and Path(args.seam_corpus).exists():
        corpus = json.loads(Path(args.seam_corpus).read_text(encoding="utf-8"))
    leak_convs: set = set()
    if corpus:
        cap = Path(corpus.get("a_capture") or "")
        if cap.exists():
            for line in cap.read_text(encoding="utf-8", errors="replace").splitlines():
                line = line.strip()
                if not line:
                    continue
                try:
                    row = json.loads(line)
                except json.JSONDecodeError:
                    continue
                req = json.dumps(row.get("request") or {}, ensure_ascii=False)
                if any(n in req for n in ("normalized_role", "template_role", '"alias"',
                                          "display_group", "control_relevance")) or \
                   ('"control_shell"' in req and
                        re.search(r'"control_shell"\s*:\s*\{', req)):
                    leak_convs.add(str(row.get("conversation_id") or ""))

    rounds_dir = args.run_dir / "rounds"
    rows = []
    missing = []
    incomplete = False
    typed_device_invalid = []
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
            device_flags = []
            if rec.get("conversation_id") in leak_convs:
                device_flags.append(
                    "seam_leakage: semantic overlay / content-form control_shell "
                    "entered model context (see seam_verify_corpus.json)")
            if faces["typed_adapter_calls"]:
                device_flags.append(
                    "device_invalid_typed_adapter: executed typed adapter call(s) "
                    f"{faces['typed_adapter_calls']} -- after S5 the plugin_grabber "
                    "family must not be callable on the ordinary loop (catalog trim failed)")
                typed_device_invalid.append(f"{case}_r{repeat}")
            rows.append({
                "case": case, "arm": "A-pure", "repeat": repeat,
                "raw_arm_field": rec.get("arm"),
                "conversation_id": rec.get("conversation_id"),
                "first_action": rec.get("first_action"),
                "tool_faces": faces,
                "device_flags": device_flags,
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
        entry = {
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
        if case == "T4":
            entry["t4_linked_pair_note"] = (
                "307 acceptance ruling in force: unexpected_mutation failures of "
                "the deliberate dual-channel-write form are a suspected frozen-"
                "rule defect (stereo linked pair), verdicts preserved as "
                "evaluated; rule fix deferred to before B/C batches -- analysis "
                "layer flag only, criteria unchanged")
        per_case[case] = entry

    # three-way comparison A-pure vs A(307) vs P
    comparison = []
    p_summary = {}
    a307_summary = {}
    if args.p_summary.exists():
        p_summary = json.loads(args.p_summary.read_text(encoding="utf-8"))
    if args.a307_summary.exists():
        a307_summary = json.loads(args.a307_summary.read_text(encoding="utf-8"))
    p_rounds = {(r.get("case_id")): r for r in p_summary.get("rounds", [])
                if r.get("final_verdict")}
    a307_rows = {(r.get("case")): [x for x in a307_summary.get("score_table", [])
                                   if x.get("case") == r.get("case")]
                 for r in a307_summary.get("score_table", [])}
    for case in CASES:
        ap_rows = [r for r in rows if r["case"] == case]
        ap_pass = per_case[case]["pass_count"]
        p_row = p_rounds.get(case) or {}
        a307_case_rows = a307_rows.get(case) or []
        a307_pass = sum(1 for r in a307_case_rows if r.get("final_verdict") == "pass")
        a307_write = [r.get("tool_faces", {}).get("write_path") for r in a307_case_rows]
        comparison.append({
            "case": case,
            "Apure_pass": f"{ap_pass}/{len(ap_rows)}" if ap_rows else "n/a",
            "Apure_verdicts": [r["final_verdict"] for r in ap_rows],
            "Apure_failure_tags": sorted({t for r in ap_rows for t in r["failure_tags"]}),
            "Apure_write_paths": [r["tool_faces"]["write_path"] for r in ap_rows],
            "A307_pass": f"{a307_pass}/{len(a307_case_rows)}" if a307_case_rows else "n/a",
            "A307_verdicts": [r.get("final_verdict") for r in a307_case_rows],
            "A307_failure_tags": sorted({t for r in a307_case_rows
                                         for t in (r.get("failure_tags") or [])}),
            "A307_write_paths": a307_write,
            "P_verdict": p_row.get("final_verdict"),
            "P_failure_tags": p_row.get("failure_tags") or [],
            "P_llm_calls": p_row.get("llm_calls"),
            "Apure_llm_calls_total": per_case[case]["llm_calls"],
            "delta_note": ("A-pure = seam set 305 S1-S4 + 308 S5/S2b (no typed "
                           "tools in catalog, render surface stripped); "
                           "A(307) = S1-S4 only (typed adapters selectable, "
                           "10/18 rounds used them); P = production seams all "
                           "closed; single P repeat per plan run_counts.P"),
        })

    summary = {
        "schema_version": "plugin_control_eval_segment_apure_summary.v1",
        "card": "PLUGIN-CONTROL-EVAL-APURE-1",
        "run_dir": str(args.run_dir.resolve()),
        "arm": ("A-pure (seam open VIT_PLUGIN_CONTROL_EVAL_SEAM=1 = 305 S1-S4 + "
                "308 S5 catalog trim + S2b render strip; relabel per RUNBOOK: "
                "raw driver records keep arm=\"P\")"),
        "model": "deepseek-flash (product default config)",
        "score_table": rows,
        "totals": totals, "caps": CAPS_A, "within_caps": within_caps,
        "per_case": per_case,
        "three_way_comparison": comparison,
        "p_baseline_source": str(args.p_summary.resolve()),
        "a307_source": str(args.a307_summary.resolve()),
        "device_gate": {
            "typed_adapter_invalid_rounds": typed_device_invalid,
            "note": ("after S5 any executed typed adapter call = device invalid "
                     "(card stop condition); empty list required for a clean "
                     "A-pure dataset"),
        },
        "seam_corpus_check": {
            "source": corpus.get("a_capture") if corpus else None,
            "verdict": corpus.get("verdict") if corpus else None,
            "leaking_conversations": sorted(leak_convs),
            "note": "affected rounds carry device_flags; verdicts preserved "
                    "as evaluated (supervisor adjudicates arm validity)",
        } if corpus else None,
        "missing_rounds": missing,
        "notes": [
            "probe round (T1 repeat 0, device verification carrier) excluded from scores",
            "cost fields remain missing pending plan budgets.monetary price snapshot",
            "single round = single sample; 3 repeats do not constitute a stability claim (plan section 6)",
            "tool-face: S5 trims the plugin_grabber family from the model "
            "catalog (all three catalog exits); expected write_path domain = "
            "bare | none_observed",
        ],
    }
    if missing:
        incomplete = True

    args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2),
                        encoding="utf-8")

    # console table
    hdr = (f"{'case':4} {'rep':3} {'verdict':8} {'write_path':20} {'fail_tags':28} "
           f"{'mut':3} {'calls':5} {'in_tok':8} {'out_tok':7} {'stop':24}")
    print(hdr)
    for r in rows:
        print(f"{r['case']:4} {r['repeat']:<3} {str(r['final_verdict']):8} "
              f"{r['tool_faces']['write_path']:20} "
              f"{','.join(r['failure_tags'])[:28]:28} "
              f"{str(r['mutation_count']):3} {r['llm_calls']:<5} "
              f"{r['input_tokens']:<8} {r['output_tokens']:<7} "
              f"{str(r['stop_reason'])[:24]:24}")
    print(f"totals: {totals} within_caps={within_caps}")
    print("A-pure vs A(307) vs P:")
    for c in comparison:
        print(f"  {c['case']}: A-pure={c['Apure_pass']} vs A307={c['A307_pass']} "
              f"vs P={c['P_verdict']} (Apure_tags={c['Apure_failure_tags'] or '-'})")
    if typed_device_invalid:
        print(f"DEVICE INVALID (typed adapter executed): {typed_device_invalid}")
    if missing:
        print(f"MISSING ROUNDS: {missing}")
        return 1
    if typed_device_invalid:
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
