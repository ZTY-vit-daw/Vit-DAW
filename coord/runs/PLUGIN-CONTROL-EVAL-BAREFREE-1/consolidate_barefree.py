#!/usr/bin/env python3
r"""PLUGIN-CONTROL-EVAL-BAREFREE-1 consolidation (card 318, BAREFREE arm).

Offline consolidation of the 18 task rounds. Core deliverables:

  1. score_table with the barewrite ledger per round (plugin parameter
     read/write tool calls the model issued, their tool_result statuses,
     guard rejection shapes) -- attempts vs landed writes are the arm's
     central observable;
  2. guard_blocked_write ANALYSIS classification: rounds whose failure
     involves a governance-guard rejection (typed_*_surface_unavailable /
     typed_*_control_required tool_results, or the T1 write-gate tool_gate
     reason) get the analysis label guard_blocked_write. The frozen
     evaluator's verdict/failure_tags are NEVER rewritten (terminal-state
     labels stay as evaluated); the analysis label is presented alongside
     so a guard interception can never be silently misread as
     value_encoding_error -- the exact mislabel the B-R1 dissection found
     on all 9 write-failure rounds;
  3. six-way comparison BAREFREE vs B-R1(317) vs B(312) vs A-pure(310) vs
     A(307) vs P(300);
  4. barewrite_core_answer: T1/T3/T4 pass rates under guard permission =
     the true bare-write capability measurement; per-round readback
     encoding evidence (verdict checks label_exact / normalized) for
     every landed write.

Never talks to the agent or kernel; reads run artifacts only.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

CASES = ["T1", "T2", "T3", "T4", "T5", "T6"]
CAPS = {"rounds": 18, "max_llm_calls": 216, "max_input_tokens": 8640000,
        "max_output_tokens": 1728000}  # plan segment_caps.A_star verbatim
WRITE_CASES = {"T1", "T3", "T4"}      # the 0/9 guard-blocked trio under test

TOOL_CALL_RE = re.compile(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)")
LOOP_TOOL_RE = re.compile(r"message_loop\.tool ms=\d+ goal=\S+ tool=([A-Za-z0-9_.\-]+)")
TYPED_ADAPTER_RE = re.compile(
    r"^(plugin_grabber[._](inspect|apply|explain)|apply_(reverb|delay|eq|compressor|"
    r"limiter|de_esser|gate_expander|transient_shaper|multiband))")
RETRIEVAL_EXEC_RE = re.compile(
    r"harness\.invoke total_ms=\d+ tool=(web\.search|web\.fetch|browser\.search|browser\.fetch)\b")
BARE_WRITE_TOOL_RE = re.compile(r"^(plugin\.set_parameter|plugin_set_parameter|set_plugin_param)$")
BARE_READ_TOOL_RE = re.compile(r"^(plugin\.get_parameters|plugin_get_parameters|get_plugin_parameters)$")
TYPED_GUARD_REJECTION_RE = re.compile(
    r"typed_(transient_shaper|compressor|limiter|gate_expander|de_esser|eq|multiband)"
    r"_(surface_unavailable|control_required)")
T1_WRITE_GATE_REASON = "broad mixing requests must stop here"
GUARD_CODES = ["typed_transient_shaper_surface_unavailable",
               "typed_compressor_surface_unavailable",
               "typed_limiter_surface_unavailable",
               "typed_gate_expander_surface_unavailable",
               "typed_de_esser_surface_unavailable",
               "typed_eq_surface_unavailable",
               "typed_multiband_surface_unavailable",
               "typed_transient_shaper_control_required",
               "typed_compressor_control_required",
               "typed_limiter_control_required",
               "typed_gate_expander_control_required",
               "typed_de_esser_control_required",
               "typed_eq_control_required",
               "typed_multiband_control_required"]


def tool_faces(delta: str) -> dict:
    calls = TOOL_CALL_RE.findall(delta or "")
    loop_calls = LOOP_TOOL_RE.findall(delta or "")
    all_calls = loop_calls + calls
    retrieval = [c for c in all_calls if RETRIEVAL_EXEC_RE.fullmatch(c)]
    bare_writes = [c for c in all_calls if BARE_WRITE_TOOL_RE.match(c)]
    bare_reads = [c for c in all_calls if BARE_READ_TOOL_RE.match(c)]
    typed = [c for c in all_calls if TYPED_ADAPTER_RE.match(c)]
    return {
        "all_tool_calls": all_calls,
        "retrieval_calls": retrieval,
        "bare_write_calls": bare_writes,
        "bare_read_calls": bare_reads,
        "typed_adapter_calls": typed,
        "write_path": ("typed" if typed else
                        "bare" if bare_writes else
                        "none_observed"),
    }


def load_capture_rows(path: Path, conversation_id: str) -> list[dict]:
    rows = []
    if not path.exists():
        return rows
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if str(row.get("conversation_id") or "") == conversation_id:
            rows.append(row)
    return rows


def barewrite_ledger_from_capture(capture_path: Path, conversation_id: str) -> dict:
    """Best-effort pairing of plugin parameter read/write attempts with the
    tool_result statuses that later request payloads echo (evidence body =
    statuses and rejection texts; exact pairing carries no conclusion)."""
    ledger: dict = {"attempts": [], "attempt_counts": {"read": 0, "write": 0},
                    "result_statuses": [], "guard_rejections": [],
                    "tool_gate_blocks": [], "conversation_id": conversation_id}
    rows = load_capture_rows(capture_path, conversation_id)
    for row in rows:
        req = row.get("request") or {}
        messages = req.get("messages") or []
        for m in messages:
            if not isinstance(m, dict):
                continue
            content = m.get("content")
            if not isinstance(content, str):
                continue
            role = m.get("role")
            if role == "assistant":
                # model-issued tool_calls for plugin parameter read/write
                try:
                    parsed = json.loads(content)
                except (json.JSONDecodeError, ValueError):
                    parsed = None
                calls = []
                if isinstance(parsed, dict):
                    calls = parsed.get("tool_calls") or []
                for call in calls:
                    if not isinstance(call, dict):
                        continue
                    tool = str(call.get("tool") or "")
                    if BARE_WRITE_TOOL_RE.match(tool):
                        ledger["attempt_counts"]["write"] += 1
                        ledger["attempts"].append({
                            "tool": tool, "kind": "write",
                            "args": call.get("args"),
                            "reason": str(call.get("reason") or "")[:200],
                        })
                    elif BARE_READ_TOOL_RE.match(tool):
                        ledger["attempt_counts"]["read"] += 1
                        ledger["attempts"].append({
                            "tool": tool, "kind": "read",
                            "args": call.get("args"),
                            "reason": str(call.get("reason") or "")[:200],
                        })
            elif role == "user" and content.startswith("<tool_result>"):
                try:
                    payload = json.loads(content[len("<tool_result>"):])
                except (json.JSONDecodeError, ValueError):
                    continue
                exec_block = payload.get("execution") or {}
                tool = str(exec_block.get("tool") or payload.get("tool") or "")
                if not (BARE_WRITE_TOOL_RE.match(tool) or BARE_READ_TOOL_RE.match(tool)):
                    continue
                status = str(exec_block.get("status") or payload.get("status") or "")
                err = str(exec_block.get("error") or "")
                summary = exec_block.get("result_summary") or {}
                preview = (summary.get("result_preview") if isinstance(summary, dict) else {}) or {}
                rejection_code = str((preview.get("rejection_code") if isinstance(preview, dict) else "") or "")
                ledger["result_statuses"].append({
                    "tool": tool, "status": status or str(payload.get("status") or ""),
                    "rejection_code": rejection_code,
                    "parameters_changed": preview.get("parameters_changed")
                        if isinstance(preview, dict) else None,
                    "error_head": err[:160],
                })
                m2 = TYPED_GUARD_REJECTION_RE.search(err) or \
                    (TYPED_GUARD_REJECTION_RE.search(rejection_code) if rejection_code else None)
                if m2:
                    ledger["guard_rejections"].append({
                        "tool": tool, "code": m2.group(0),
                        "error_head": err[:200]})
            elif role == "user" and content.startswith("<tool_gate>"):
                if T1_WRITE_GATE_REASON in content:
                    try:
                        gate = json.loads(content[len("<tool_gate>"):content.rfind("</tool_gate>")])
                    except (json.JSONDecodeError, ValueError):
                        gate = {"raw": content[:200]}
                    ledger["tool_gate_blocks"].append({
                        "blocked_tool": gate.get("blocked_tool"),
                        "reason_head": str(gate.get("reason") or "")[:160]})
    return ledger


def guard_shape_hits(text: str) -> dict:
    hits: dict = {"typed_guard_codes": [], "t1_write_gate": 0}
    for code in GUARD_CODES:
        if code in text:
            hits["typed_guard_codes"].append(code)
    hits["t1_write_gate"] = text.count(T1_WRITE_GATE_REASON)
    return hits


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True, type=Path)
    ap.add_argument("--p-summary", required=True, type=Path)
    ap.add_argument("--a307-summary", required=True, type=Path)
    ap.add_argument("--apure-summary", required=True, type=Path)
    ap.add_argument("--b312-summary", required=True, type=Path,
                    help="312 arm_b_summary.json")
    ap.add_argument("--br1-summary", required=True, type=Path,
                    help="317 arm_br1_summary.json")
    ap.add_argument("--capture", required=True, type=Path,
                    help="run's agent_llm_payload_capture.jsonl (barewrite ledger)")
    ap.add_argument("--gw-live-probe", default="", type=Path,
                    help="guard_live_probe.json (echoed into device_gate)")
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()

    gw_live = {}
    gw_path = Path(args.gw_live_probe) if args.gw_live_probe else (args.run_dir / "guard_live_probe.json")
    if gw_path.is_file():
        try:
            gw_live = json.loads(gw_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            gw_live = {"_unparseable": True}

    rounds_dir = args.run_dir / "rounds"
    rows = []
    missing = []
    typed_device_invalid = []
    guard_blocked_rounds = []
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
            ledger = barewrite_ledger_from_capture(
                args.capture, str(rec.get("conversation_id") or ""))
            # guard-shape scan over the whole conversation payload text
            conv_text = ""
            for row in load_capture_rows(args.capture, str(rec.get("conversation_id") or "")):
                try:
                    conv_text += json.dumps(row.get("request") or {}, ensure_ascii=False)
                except Exception:
                    pass
            shapes = guard_shape_hits(conv_text + "\n" + str(rec.get("agent_log_delta") or ""))
            device_flags = []
            if faces["typed_adapter_calls"]:
                device_flags.append(
                    "device_invalid_typed_adapter: executed typed adapter call(s) "
                    f"{faces['typed_adapter_calls']}")
                typed_device_invalid.append(f"{case}_r{repeat}")
            if faces["retrieval_calls"]:
                device_flags.append(
                    "w0_retrieval_execution: retrieval call(s) executed "
                    f"{faces['retrieval_calls']} (no B seam in this arm)")
            if shapes["typed_guard_codes"] or shapes["t1_write_gate"] or \
                    ledger["guard_rejections"] or ledger["tool_gate_blocks"]:
                device_flags.append(
                    "gw_guard_rejection_observed: guard rejection shape(s) present "
                    f"despite the barewrite seam -- {shapes['typed_guard_codes'][:3]} "
                    f"t1_gate={shapes['t1_write_gate']}")
            failed = v.get("final_verdict") != "pass"
            analysis = ""
            if failed and (shapes["typed_guard_codes"] or shapes["t1_write_gate"]):
                analysis = "guard_blocked_write"
                guard_blocked_rounds.append(f"{case}_r{repeat}")
            elif failed and v.get("final_verdict") is not None:
                analysis = "evaluator_terminal_label:" + ",".join(v.get("failure_tags") or [])
            rows.append({
                "case": case, "arm": "BAREFREE", "repeat": repeat,
                "raw_arm_field": rec.get("arm"),
                "conversation_id": rec.get("conversation_id"),
                "first_action": rec.get("first_action"),
                "tool_faces": faces,
                "device_flags": device_flags,
                "barewrite_ledger": ledger,
                "guard_shapes": shapes,
                "analysis_classification": analysis,
                "final_verdict": v.get("final_verdict"),
                "failure_tags": v.get("failure_tags") or [],
                "failure_detail": v.get("detail"),
                "verdict_checks": v.get("checks"),
                "mutation_count": v.get("mutation_count"),
                "mutation_audit": rec.get("mutation_audit"),
                "behavior_tags": rec.get("behavior_tags") or [],
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
    within_caps = (totals["llm_calls"] <= CAPS["max_llm_calls"]
                   and totals["input_tokens"] <= CAPS["max_input_tokens"]
                   and totals["output_tokens"] <= CAPS["max_output_tokens"])

    per_case = {}
    for case in CASES:
        c_rows = [r for r in rows if r["case"] == case]
        verdicts = [r["final_verdict"] for r in c_rows]
        entry = {
            "repeats": len(c_rows),
            "pass_count": sum(1 for v2 in verdicts if v2 == "pass"),
            "verdicts": verdicts,
            "failure_tags": sorted({t for r in c_rows for t in r["failure_tags"]}),
            "analysis_classifications": [r["analysis_classification"] for r in c_rows],
            "write_paths": [r["tool_faces"]["write_path"] for r in c_rows],
            "typed_adapter_used_rounds": sum(1 for r in c_rows
                                             if r["tool_faces"]["typed_adapter_calls"]),
            "bare_write_attempt_rounds": sum(1 for r in c_rows
                                             if r["tool_faces"]["bare_write_calls"]),
            "bare_write_attempt_counts": [r["barewrite_ledger"]["attempt_counts"]["write"]
                                          for r in c_rows],
            "guard_rejection_rounds": sum(
                1 for r in c_rows
                if r["guard_shapes"]["typed_guard_codes"] or r["guard_shapes"]["t1_write_gate"]),
            "landed_write_rounds": sum(1 for r in c_rows if (r["mutation_count"] or 0) > 0),
            "llm_calls": sum(r["llm_calls"] for r in c_rows),
        }
        if case == "T4":
            entry["t4_linked_pair_note"] = (
                "307 acceptance ruling in force (carried by 310/312/317, "
                "unchanged): unexpected_mutation failures of the deliberate "
                "dual-channel-write form are a suspected frozen-rule defect "
                "(stereo linked pair), verdicts preserved as evaluated; "
                "analysis layer flag only, criteria unchanged")
        per_case[case] = entry

    # six-way comparison BAREFREE vs BR1(317) vs B(312) vs A-pure(310) vs A(307) vs P
    comparison = []
    summaries = {}
    for key, path in (("p", args.p_summary), ("a307", args.a307_summary),
                      ("apure", args.apure_summary), ("b312", args.b312_summary),
                      ("br1", args.br1_summary)):
        if Path(path).exists():
            summaries[key] = json.loads(Path(path).read_text(encoding="utf-8"))
    p_summary = summaries.get("p") or {}
    p_rounds = {r.get("case_id"): r for r in p_summary.get("rounds", [])
                if r.get("final_verdict")}
    a307_score = (summaries.get("a307") or {}).get("score_table") or []
    apure_score = (summaries.get("apure") or {}).get("score_table") or []
    b312_score = (summaries.get("b312") or {}).get("score_table") or []
    br1_score = (summaries.get("br1") or {}).get("score_table") or []
    for case in CASES:
        bf_rows = [r for r in rows if r["case"] == case]
        bf_pass = per_case[case]["pass_count"]

        def arm_case(score, case):
            cr = [r for r in score if r.get("case") == case]
            p = sum(1 for r in cr if r.get("final_verdict") == "pass")
            return cr, p

        a307_c, a307_p = arm_case(a307_score, case)
        apure_c, apure_p = arm_case(apure_score, case)
        b312_c, b312_p = arm_case(b312_score, case)
        br1_c, br1_p = arm_case(br1_score, case)
        p_row = p_rounds.get(case) or {}
        comparison.append({
            "case": case,
            "BAREFREE_pass": f"{bf_pass}/{len(bf_rows)}" if bf_rows else "n/a",
            "BAREFREE_verdicts": [r["final_verdict"] for r in bf_rows],
            "BAREFREE_failure_tags": sorted({t for r in bf_rows for t in r["failure_tags"]}),
            "BAREFREE_analysis_classifications": per_case[case]["analysis_classifications"],
            "BAREFREE_write_paths": per_case[case]["write_paths"],
            "BAREFREE_landed_write_rounds": per_case[case]["landed_write_rounds"],
            "BR1_pass": f"{br1_p}/{len(br1_c)}" if br1_c else "n/a",
            "BR1_verdicts": [r.get("final_verdict") for r in br1_c],
            "BR1_failure_tags": sorted({t for r in br1_c for t in (r.get("failure_tags") or [])}),
            "B312_pass": f"{b312_p}/{len(b312_c)}" if b312_c else "n/a",
            "B312_verdicts": [r.get("final_verdict") for r in b312_c],
            "B312_failure_tags": sorted({t for r in b312_c for t in (r.get("failure_tags") or [])}),
            "Apure_pass": f"{apure_p}/{len(apure_c)}" if apure_c else "n/a",
            "Apure_verdicts": [r.get("final_verdict") for r in apure_c],
            "Apure_failure_tags": sorted({t for r in apure_c for t in (r.get("failure_tags") or [])}),
            "A307_pass": f"{a307_p}/{len(a307_c)}" if a307_c else "n/a",
            "A307_verdicts": [r.get("final_verdict") for r in a307_c],
            "A307_failure_tags": sorted({t for r in a307_c for t in (r.get("failure_tags") or [])}),
            "P_verdict": p_row.get("final_verdict"),
            "P_failure_tags": p_row.get("failure_tags") or [],
            "delta_note": ("BAREFREE = A-pure face (VIT_PLUGIN_CONTROL_EVAL_SEAM=1: "
                           "305 S1-S4 + 308 S5/S2b, NO retrieval seam, NO prompt "
                           "seam) PLUS VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 "
                           "(card 318 guard-permission seam: T1 write gate exempts "
                           "plugin parameter read/write; chat typed generic-write "
                           "guards pass bare writes). BAREFREE vs A-pure is the "
                           "same-face guard on/off contrast = the true bare-write "
                           "capability measurement"),
        })

    # barewrite core answer: the write-trio pass rate under guard permission
    write_trio_rows = [r for r in rows if r["case"] in WRITE_CASES]
    write_trio_pass = sum(1 for r in write_trio_rows if r["final_verdict"] == "pass")
    landed = [r for r in write_trio_rows if (r["mutation_count"] or 0) > 0]
    readback_evidence = []
    for r in write_trio_rows:
        checks = r.get("verdict_checks") or {}
        readback_evidence.append({
            "round": f"{r['case']}_r{r['repeat']}",
            "final_verdict": r["final_verdict"],
            "failure_tags": r["failure_tags"],
            "analysis_classification": r["analysis_classification"],
            "mutation_count": r["mutation_count"],
            "label_exact_check": checks.get("label_exact"),
            "normalized_check": checks.get("normalized_in_tolerance"),
            "write_attempts": r["barewrite_ledger"]["attempt_counts"]["write"],
            "guard_rejections": len(r["barewrite_ledger"]["guard_rejections"]),
            "t1_gate_blocks": len(r["barewrite_ledger"]["tool_gate_blocks"]),
        })
    apure_write_rows = []
    for r in apure_score:
        if r.get("case") in WRITE_CASES:
            apure_write_rows.append(r)
    apure_write_pass = sum(1 for r in apure_write_rows
                           if r.get("final_verdict") == "pass")
    core_answer = {
        "question": ("with the governance guards permissioned (seam open), can "
                     "the model actually bare-write guarded-family plugin "
                     "parameters? -- pass rate of the guard-blocked trio "
                     "T1/T3/T4 = true bare-write capability"),
        "barefree_write_trio_pass": f"{write_trio_pass}/{len(write_trio_rows)}",
        "apure_write_trio_pass": f"{apure_write_pass}/{len(apure_write_rows)}",
        "same_face_contrast": (
            "BAREFREE vs A-pure differ ONLY in guard permission (single "
            "variable); A-pure 0/9 was device-determined guard blocking per "
            "the 318 dissection, so any BAREFREE conversion isolates model "
            "bare-write capability"),
        "write_trio_landed_rounds": len(landed),
        "readback_encoding_evidence": readback_evidence,
        "guard_blocked_write_rounds": guard_blocked_rounds,
        "mislabel_guard_note": (
            "evaluator failure_tags are terminal-state labels; rounds listed "
            "in guard_blocked_write_rounds carry guard rejection shapes in "
            "their tool_results -- with the seam open their presence means "
            "either seam failure (see device_flags/gw faces) or a guard "
            "surface outside the seam's seven guards; report as-is, never "
            "read them as value_encoding_error"),
    }

    summary = {
        "schema_version": "plugin_control_eval_segment_barefree_summary.v1",
        "card": "PLUGIN-CONTROL-EVAL-BAREFREE-1",
        "run_dir": str(args.run_dir.resolve()),
        "arm": ("BAREFREE (A-pure face VIT_PLUGIN_CONTROL_EVAL_SEAM=1 PLUS guard "
                "seam VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1; relabel per "
                "RUNBOOK: raw driver records keep arm=\"P\")"),
        "model": "deepseek-flash (product default config)",
        "score_table": rows,
        "totals": totals, "caps": CAPS, "within_caps": within_caps,
        "per_case": per_case,
        "six_way_comparison": comparison,
        "barewrite_core_answer": core_answer,
        "guard_blocked_write_analysis": {
            "classified_rounds": guard_blocked_rounds,
            "taxonomy_note": ("new analysis class per card 318 / conclusion-file "
                              "todo-2: tool_result guard-error shape recognition "
                              "(typed_* codes + T1 write-gate reason); evaluator "
                              "verdicts untouched"),
            "device_flag_rounds": [f"{r['case']}_r{r['repeat']}" for r in rows
                                   if r["device_flags"]],
        },
        "device_gate": {"gw_live_probe": gw_live},
        "missing_rounds": missing,
        "typed_device_invalid_rounds": typed_device_invalid,
        "notes": [
            "arm label residual: frozen driver records arm=P + pce_p_ prefix "
            "(300 precedent); BAREFREE relabel here, raw records untouched",
            "single-run single-sample per round; 3 repeats are not a stability "
            "claim (plan section 6)",
            "browser.* sibling tools: same retrieval family for W0 accounting",
        ],
    }

    args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({
        "out": str(args.out),
        "rounds": len(rows),
        "missing": missing,
        "within_caps": within_caps,
        "write_trio_pass": core_answer["barefree_write_trio_pass"],
        "guard_blocked_write_rounds": guard_blocked_rounds,
    }, ensure_ascii=False, indent=2))
    if missing:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
