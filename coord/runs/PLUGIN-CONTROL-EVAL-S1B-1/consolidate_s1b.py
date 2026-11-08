#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-S1B-1 consolidation (card 312, offline).

Reads the frozen driver's round records from <run-dir>/rounds (task rounds
T1..T6 x repeat 1..3; probe T1_r0 excluded by construction), the run's LLM
payload capture (retrieval ledger source), the frozen P-baseline segment
summary, the 307 A-arm summary and the 310 A-pure summary, then writes
arm_b_summary.json:

  - 18-row B score table (arm relabeled "B" per RUNBOOK: the frozen 300
    driver hardcodes arm="P" in raw records, preserved verbatim -- same
    relabel ruling as 310's A-pure);
  - usage totals vs plan segment_caps.B_star (caps carried verbatim from
    plan.json B_star: 18 rounds / 216 calls / 8.64M in / 1.728M out);
  - retrieval ledger per round (plan section 5): executed retrieval calls
    from the agent-log delta + per-conversation payload extraction of
    queries / URLs / timestamps / result-content hashes / bytes entering
    context, with per-round counts vs per_round_caps
    max_retrieval_searches_B_C=6 / max_retrieval_fetches_B_C=10;
  - retrieval behavior analysis (card deliverable): per case -- did the
    model initiate retrieval, what was queried, and did retrieval convert
    into a correct value encoding (write landed / verdict pass) -- with the
    plan section 5 failure split (no query initiated / no applicable
    material / tool or network refused / found but not extracted / applied
    wrong) and a repo-contamination watchlist (URLs/queries touching this
    repository's own docs -> flagged for supervisor adjudication);
  - FOUR-WAY per-case comparison B vs A-pure(310) vs A(307) vs P -- the
    card's core deliverable ("can retrieval fill the bare-face knowledge
    gap");
  - typed-adapter device gate and seam-corpus flags inherited from the 310
    consolidation (S5 zero-executed-typed requirement + leak conversation
    mapping; verdicts are NOT rewritten).

Offline only; never talks to the agent/kernel. Exit 0 on complete
consolidation, 1 if records are missing/incomplete, 2 if any round shows an
executed typed adapter call (device invalid; partial output still written).
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

CASES = ["T1", "T2", "T3", "T4", "T5", "T6"]
CAPS_B = {"rounds": 18, "max_llm_calls": 216,
          "max_input_tokens": 8640000, "max_output_tokens": 1728000}
RETRIEVAL_CAPS = {"max_retrieval_searches": 6, "max_retrieval_fetches": 10}

HARNESS_TOOL_RE = re.compile(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)")
LOOP_TOOL_RE = re.compile(r"message_loop\.tool ms=\d+ goal=\S+ tool=([A-Za-z0-9_.\-]+)")
TYPED_ADAPTER_RE = re.compile(
    r"^(plugin_grabber[._](inspect|apply|explain)|apply_(reverb|delay|eq|compressor|"
    r"limiter|de_esser|gate_expander|transient_shaper|multiband))")
BARE_WRITE_RE = re.compile(r"^(plugin\.set_parameter|set_plugin_param)$")
BARE_READ_RE = re.compile(r"^(plugin\.get_parameters|get_plugin_parameters)$")
RETRIEVAL_TOOL_RE = re.compile(
    r"^(web\.search|web\.fetch|browser\.search|browser\.fetch)$")
# per-conversation tool-catalog route face (amendment 2026-11-08): the
# ordinary loop discloses the FULL catalog (web pair included; 310 form 209
# lines) only on zero-capability-match turns; capability matches route to
# filtered packs (42/51 lines, no web pair; 310: T1/T4/T5/T6 pack,
# T2/T3 full). Recorded per round -- the retrieval treatment is delivered
# exactly where the product routing delivers it.
CATALOG_LINE_ANY_RE = re.compile(r"[A-Za-z_0-9]+ tool=[A-Za-z0-9_.\-]+ risk=\w+")
CATALOG_WEB_PAIR_RE = re.compile(
    r"web_search tool=web\.search risk=\w+[\s\S]{0,4000}?web_fetch tool=web\.fetch risk=\w+|"
    r"web_fetch tool=web\.fetch risk=\w+[\s\S]{0,4000}?web_search tool=web\.search risk=\w+")
FULL_CATALOG_MIN_LINES = 150
# repo-contamination watch needles: retrieval that reaches THIS project's own
# documentation would be the model googling the experiment (card constraint:
# record the URL list and hand to the supervisor)
CONTAMINATION_NEEDLES = ("vit_daw", "vit-daw", "zty-vit-daw", "plugin-control-eval")


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
        "retrieval_calls": [c for c in all_calls if RETRIEVAL_TOOL_RE.match(c)],
        "write_path": ("typed_adapter" if any(TYPED_ADAPTER_RE.match(c) and "apply" in c
                                              for c in all_calls)
                       else ("typed_adapter_nonapply" if any(TYPED_ADAPTER_RE.match(c)
                                                             for c in all_calls)
                             else ("bare" if any(BARE_WRITE_RE.match(c) for c in all_calls)
                                   else "none_observed"))),
    }


def _short_hash(text: str) -> str:
    return hashlib.sha256((text or "").encode("utf-8", errors="replace")).hexdigest()[:16]


def retrieval_ledger_from_capture(capture_path: Path, conversation_id: str) -> dict:
    """Per-round retrieval evidence from the payload capture (plan section 5).

    Walks the LLM request messages of the round's conversation: assistant
    tool_calls entries for the retrieval tool family give the query / url
    arguments and the request timestamp; tool-role reply messages give the
    tool result content that entered the model context (hashed + length).
    Also records the conversation's tool-catalog route face (full catalog
    with web pair vs filtered pack) -- the product's bounded disclosure
    decides whether the retrieval treatment is even visible in the round.
    """
    ledger: dict = {"conversation_id": conversation_id, "attempts": [],
                    "search_count": 0, "fetch_count": 0,
                    "browser_search_count": 0, "browser_fetch_count": 0,
                    "catalog_route_face": {"max_catalog_lines": 0,
                                           "web_pair_present": False,
                                           "form": "no_catalog_lines"}}
    if not capture_path.exists() or not conversation_id:
        ledger["error"] = "capture missing"
        return ledger
    if not capture_path.exists() or not conversation_id:
        ledger["error"] = "capture missing"
        return ledger
    events: list[dict] = []
    for line in capture_path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            continue
        if str(row.get("conversation_id") or "") != conversation_id:
            continue
        req = row.get("request") or {}
        messages = req.get("messages") or []
        try:
            blob = json.dumps(req, ensure_ascii=False)
        except Exception:
            blob = str(req)
        n_lines = len(CATALOG_LINE_ANY_RE.findall(blob))
        face = ledger["catalog_route_face"]
        face["max_catalog_lines"] = max(face["max_catalog_lines"], n_lines)
        if CATALOG_WEB_PAIR_RE.search(blob):
            face["web_pair_present"] = True
        face["form"] = ("full" if face["max_catalog_lines"] >= FULL_CATALOG_MIN_LINES
                        else ("pack" if face["max_catalog_lines"] > 0
                              else "no_catalog_lines"))
        for msg in messages:
            if not isinstance(msg, dict):
                continue
            calls = msg.get("tool_calls") or []
            if msg.get("role") == "assistant" and isinstance(calls, list):
                for call in calls:
                    if not isinstance(call, dict):
                        continue
                    fn = (call.get("function") or {})
                    name = str(fn.get("name") or "")
                    if not RETRIEVAL_TOOL_RE.match(name):
                        continue
                    args_raw = str(fn.get("arguments") or "")
                    try:
                        args = json.loads(args_raw) if args_raw else {}
                    except json.JSONDecodeError:
                        args = {"_raw": args_raw[:400]}
                    events.append({
                        "kind": "call", "tool": name,
                        "query": args.get("query"),
                        "url": args.get("url"),
                        "captured_at": row.get("created_at"),
                    })
            if msg.get("role") == "tool" and isinstance(calls, list) is False:
                pass
        # tool results: role=tool messages carry the retrieval result content
        # that entered the model context; hash + length only (bounded here,
        # full text stays in the capture artifact)
        for msg in messages:
            if not isinstance(msg, dict) or msg.get("role") != "tool":
                continue
            name = str(msg.get("name") or msg.get("tool_call_name") or "")
            if not RETRIEVAL_TOOL_RE.match(name):
                continue
            content = msg.get("content")
            text = content if isinstance(content, str) else json.dumps(
                content, ensure_ascii=False)
            failed = ("联网搜索失败" in text) or ("error" in text[:200].lower())
            events.append({
                "kind": "result", "tool": name,
                "content_sha256_16": _short_hash(text),
                "context_chars": len(text or ""),
                "looks_failed": failed,
            })
    for ev in events:
        if ev["kind"] == "call":
            ledger["attempts"].append(ev)
            if ev["tool"] == "web.search":
                ledger["search_count"] += 1
            elif ev["tool"] == "web.fetch":
                ledger["fetch_count"] += 1
            elif ev["tool"] == "browser.search":
                ledger["browser_search_count"] += 1
            elif ev["tool"] == "browser.fetch":
                ledger["browser_fetch_count"] += 1
        else:
            ledger.setdefault("results", []).append(ev)
    # pair results to attempts by tool order (best effort; hashes + lengths
    # are the evidence, exact pairing is not load-bearing)
    return ledger


def retrieval_round_classification(faces: dict, ledger: dict, rec: dict) -> str:
    """Plan section 5 retrieval failure split for one round (route aware)."""
    executed = faces["retrieval_calls"]
    attempted = ledger["attempts"]
    results = ledger.get("results") or []
    tags = set(rec.get("verdict", {}).get("failure_tags") or [])
    face = ledger.get("catalog_route_face") or {}
    web_visible = bool(face.get("web_pair_present"))
    if not executed and not attempted:
        if not web_visible:
            return "retrieval_not_disclosed_pack_routed"
        return "no_query_initiated_despite_disclosure"
    if results and all(r.get("looks_failed") for r in results):
        return "tool_or_network_refused"
    if executed and not any(
            r.get("looks_failed") for r in results) and not results:
        return "query_initiated_result_shape_unresolved"
    if "value_encoding_error" in tags or "model_task_failure" in tags:
        return "retrieval_did_not_close_the_gap"
    if rec.get("verdict", {}).get("final_verdict") == "pass":
        return "retrieval_present_round_passed"
    return "retrieval_present_outcome_other"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--run-dir", required=True, type=Path)
    ap.add_argument("--p-summary", required=True, type=Path)
    ap.add_argument("--a307-summary", required=True, type=Path)
    ap.add_argument("--apure-summary", required=True, type=Path)
    ap.add_argument("--capture", required=True, type=Path,
                    help="run's agent_llm_payload_capture.jsonl (retrieval ledger)")
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
    typed_device_invalid = []
    retrieval_cap_breaches = []
    contamination_hits = []
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
            ledger = retrieval_ledger_from_capture(
                args.capture, str(rec.get("conversation_id") or ""))
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
            total_searches = ledger["search_count"] + ledger["browser_search_count"]
            total_fetches = ledger["fetch_count"] + ledger["browser_fetch_count"]
            cap_flags = []
            if total_searches > RETRIEVAL_CAPS["max_retrieval_searches"]:
                cap_flags.append(f"retrieval_searches_over_cap:{total_searches}")
            if total_fetches > RETRIEVAL_CAPS["max_retrieval_fetches"]:
                cap_flags.append(f"retrieval_fetches_over_cap:{total_fetches}")
            if cap_flags:
                retrieval_cap_breaches.append(f"{case}_r{repeat}:{','.join(cap_flags)}")
            for att in ledger["attempts"]:
                blob = json.dumps(att, ensure_ascii=False).lower()
                if any(n in blob for n in CONTAMINATION_NEEDLES):
                    contamination_hits.append({
                        "round": f"{case}_r{repeat}", "attempt": att})
            rows.append({
                "case": case, "arm": "B", "repeat": repeat,
                "raw_arm_field": rec.get("arm"),
                "conversation_id": rec.get("conversation_id"),
                "first_action": rec.get("first_action"),
                "tool_faces": faces,
                "device_flags": device_flags,
                "retrieval_ledger": ledger,
                "retrieval_classification": retrieval_round_classification(
                    faces, ledger, rec),
                "retrieval_cap_flags": cap_flags,
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
    within_caps = (totals["llm_calls"] <= CAPS_B["max_llm_calls"]
                   and totals["input_tokens"] <= CAPS_B["max_input_tokens"]
                   and totals["output_tokens"] <= CAPS_B["max_output_tokens"])

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
            "retrieval_rounds": sum(1 for r in c_rows if r["tool_faces"]["retrieval_calls"]),
            "retrieval_visible_rounds": sum(
                1 for r in c_rows
                if (r["retrieval_ledger"].get("catalog_route_face") or {})
                .get("web_pair_present")),
            "catalog_route_forms": [
                (r["retrieval_ledger"].get("catalog_route_face") or {}).get("form")
                for r in c_rows],
            "retrieval_classifications": [r["retrieval_classification"] for r in c_rows],
            "retrieval_queries": sorted({
                str(a.get("query")) for r in c_rows
                for a in r["retrieval_ledger"]["attempts"] if a.get("query")}),
            "retrieval_urls": sorted({
                str(a.get("url")) for r in c_rows
                for a in r["retrieval_ledger"]["attempts"] if a.get("url")}),
            "llm_calls": sum(r["llm_calls"] for r in c_rows),
        }
        if case == "T4":
            entry["t4_linked_pair_note"] = (
                "307 acceptance ruling in force (carried by 310, unchanged): "
                "unexpected_mutation failures of the deliberate dual-channel-"
                "write form are a suspected frozen-rule defect (stereo linked "
                "pair), verdicts preserved as evaluated; rule fix deferred to "
                "before C batch -- analysis layer flag only, criteria unchanged")
        per_case[case] = entry

    # four-way comparison B vs A-pure(310) vs A(307) vs P
    comparison = []
    p_summary = {}
    a307_summary = {}
    apure_summary = {}
    if args.p_summary.exists():
        p_summary = json.loads(args.p_summary.read_text(encoding="utf-8"))
    if args.a307_summary.exists():
        a307_summary = json.loads(args.a307_summary.read_text(encoding="utf-8"))
    if args.apure_summary.exists():
        apure_summary = json.loads(args.apure_summary.read_text(encoding="utf-8"))
    p_rounds = {(r.get("case_id")): r for r in p_summary.get("rounds", [])
                if r.get("final_verdict")}
    a307_rows = {(r.get("case")): [x for x in a307_summary.get("score_table", [])
                                   if x.get("case") == r.get("case")]
                 for r in a307_summary.get("score_table", [])}
    apure_score = apure_summary.get("score_table") or []
    for case in CASES:
        b_rows = [r for r in rows if r["case"] == case]
        b_pass = per_case[case]["pass_count"]
        p_row = p_rounds.get(case) or {}
        a307_case_rows = a307_rows.get(case) or []
        a307_pass = sum(1 for r in a307_case_rows if r.get("final_verdict") == "pass")
        apure_case_rows = [r for r in apure_score if r.get("case") == case]
        apure_pass = sum(1 for r in apure_case_rows if r.get("final_verdict") == "pass")
        comparison.append({
            "case": case,
            "B_pass": f"{b_pass}/{len(b_rows)}" if b_rows else "n/a",
            "B_verdicts": [r["final_verdict"] for r in b_rows],
            "B_failure_tags": sorted({t for r in b_rows for t in r["failure_tags"]}),
            "B_write_paths": [r["tool_faces"]["write_path"] for r in b_rows],
            "B_retrieval_rounds": f"{per_case[case]['retrieval_rounds']}/{len(b_rows)}",
            "B_retrieval_classifications": per_case[case]["retrieval_classifications"],
            "Apure_pass": f"{apure_pass}/{len(apure_case_rows)}" if apure_case_rows else "n/a",
            "Apure_verdicts": [r.get("final_verdict") for r in apure_case_rows],
            "Apure_failure_tags": sorted({t for r in apure_case_rows
                                          for t in (r.get("failure_tags") or [])}),
            "Apure_write_paths": [r.get("tool_faces", {}).get("write_path")
                                  for r in apure_case_rows],
            "A307_pass": f"{a307_pass}/{len(a307_case_rows)}" if a307_case_rows else "n/a",
            "A307_verdicts": [r.get("final_verdict") for r in a307_case_rows],
            "A307_failure_tags": sorted({t for r in a307_case_rows
                                         for t in (r.get("failure_tags") or [])}),
            "A307_write_paths": [r.get("tool_faces", {}).get("write_path") for r in a307_case_rows],
            "P_verdict": p_row.get("final_verdict"),
            "P_failure_tags": p_row.get("failure_tags") or [],
            "P_llm_calls": p_row.get("llm_calls"),
            "B_llm_calls_total": per_case[case]["llm_calls"],
            "delta_note": ("B = A-pure face (VIT_PLUGIN_CONTROL_EVAL_SEAM=1) PLUS "
                           "VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL=1 (web.search/"
                           "web.fetch direct, no confirmation card; 312 seam); "
                           "A-pure(310) = seam set 305 S1-S4 + 308 S5/S2b; "
                           "A(307) = S1-S4 only (typed adapters selectable); "
                           "P = production seams all closed; single P repeat per "
                           "plan run_counts.P"),
        })

    # retrieval behavior analysis (card deliverable core)
    retrieval_analysis = {
        "rounds_with_retrieval": sum(1 for r in rows if r["tool_faces"]["retrieval_calls"]),
        "rounds_total": len(rows),
        "classification_counts": {},
        "per_case": {case: {
            "retrieval_rounds": per_case[case]["retrieval_rounds"],
            "classifications": per_case[case]["retrieval_classifications"],
            "queries": per_case[case]["retrieval_queries"],
            "urls": per_case[case]["retrieval_urls"],
        } for case in CASES},
        "cap_breaches": retrieval_cap_breaches,
        "repo_contamination_watchlist": contamination_hits,
        "conversion_question": (
            "did retrieval convert value_encoding_error-type failures (310 "
            "A-pure: T1/T3/T4 all rounds) into successful bare writes? -- see "
            "four_way_comparison rows B_write_paths/B_failure_tags vs "
            "Apure_failure_tags"),
    }
    for r in rows:
        c = r["retrieval_classification"]
        retrieval_analysis["classification_counts"][c] = \
            retrieval_analysis["classification_counts"].get(c, 0) + 1

    summary = {
        "schema_version": "plugin_control_eval_segment_b_summary.v1",
        "card": "PLUGIN-CONTROL-EVAL-S1B-1",
        "run_dir": str(args.run_dir.resolve()),
        "arm": ("B (A-pure seam VIT_PLUGIN_CONTROL_EVAL_SEAM=1 + B retrieval seam "
                "VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL=1; relabel per RUNBOOK: "
                "raw driver records keep arm=\"P\")"),
        "model": "deepseek-flash (product default config)",
        "score_table": rows,
        "totals": totals, "caps": CAPS_B, "within_caps": within_caps,
        "per_case": per_case,
        "four_way_comparison": comparison,
        "retrieval_analysis": retrieval_analysis,
        "p_baseline_source": str(args.p_summary.resolve()),
        "a307_source": str(args.a307_summary.resolve()),
        "apure_source": str(args.apure_summary.resolve()),
        "device_gate": {
            "typed_adapter_invalid_rounds": typed_device_invalid,
            "note": ("after S5 any executed typed adapter call = device invalid "
                     "(card stop condition); empty list required for a clean "
                     "B dataset"),
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
            "retrieval ledger: attempts from payload-capture tool_calls, "
            "executed calls from agent-log deltas; browser.search/browser.fetch "
            "(RiskDirect siblings) are counted separately as retrieval-family "
            "evidence",
        ],
    }

    args.out.write_text(json.dumps(summary, ensure_ascii=False, indent=2),
                        encoding="utf-8")

    # console table
    hdr = (f"{'case':4} {'rep':3} {'verdict':8} {'write_path':20} {'retr':12} "
           f"{'fail_tags':28} {'mut':3} {'calls':5} {'in_tok':8} {'out_tok':7} {'stop':24}")
    print(hdr)
    for r in rows:
        retr = f"{len(r['tool_faces']['retrieval_calls'])}c/{r['retrieval_classification'][:9]}"
        print(f"{r['case']:4} {r['repeat']:<3} {str(r['final_verdict']):8} "
              f"{r['tool_faces']['write_path']:20} "
              f"{retr:12} "
              f"{','.join(r['failure_tags'])[:28]:28} "
              f"{str(r['mutation_count']):3} {r['llm_calls']:<5} "
              f"{r['input_tokens']:<8} {r['output_tokens']:<7} "
              f"{str(r['stop_reason'])[:24]:24}")
    print(f"totals: {totals} within_caps={within_caps}")
    print("B vs A-pure(310) vs A(307) vs P:")
    for c in comparison:
        print(f"  {c['case']}: B={c['B_pass']} (retr {c['B_retrieval_rounds']}) "
              f"vs Apure={c['Apure_pass']} vs A307={c['A307_pass']} "
              f"vs P={c['P_verdict']}")
    print(f"retrieval classification counts: {retrieval_analysis['classification_counts']}")
    if contamination_hits:
        print(f"REPO CONTAMINATION WATCHLIST ({len(contamination_hits)} hits) -- supervisor adjudicates")
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
