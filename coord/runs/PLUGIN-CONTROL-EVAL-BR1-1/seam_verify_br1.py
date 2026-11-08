#!/usr/bin/env python3
r"""PLUGIN-CONTROL-EVAL-BR1-1 seam device verification (card 317, B-R1 arm).

Gate script run BEFORE the 18 paid task rounds (probe round T2 repeat 0 is
the carrier; 312-amendment ruling: T2 routes to the FULL catalog so the R1
retrieval face is verifiable) and re-run post-corpus as an evidence face.
B-R1 seam set = the full 312 B-arm face (VIT_PLUGIN_CONTROL_EVAL_SEAM=1 =
305 S1-S4 + 308 S5 catalog trim + S2b render strip, PLUS
VIT_PLUGIN_CONTROL_EVAL_SEAM_B_RETRIEVAL=1 = web.search/web.fetch direct
execution) PLUS the card-317 thin-prompt seam
(VIT_PLUGIN_CONTROL_EVAL_SEAM_BR1_PROMPT=1 = frozen guidance segment
appended to the ordinary-family system skeleton). Faces 1-7 are the 312
faces unchanged; the B-R1-specific face:

  R1P (8th face, gating) thin-prompt injection: the frozen segment
      (sha256 51c403d8f8901d586ae1180cdc699ff308dbbc5a870e3fe3db7c2cf68c5a202c)
      must be present verbatim in the system-role face of EVERY message_loop
      request in the capture (the skeleton is static and the env is set for
      the agent process lifetime, so persistence across turns is part of the
      device contract), including the probe carrier's FIRST message_loop
      request (the card's "R1 亲证" -- prompt segment present in the model's
      first request system/prompt face). The segment must NOT appear in any
      ordinary_agent_semantic_entry request (system or user face -- the
      classifier input stays byte-identical to arm B; a hit there is a
      single-variable-design confound = device invalid) nor in any
      message_loop_repair request (seam scope boundary; the 312 corpus
      proves repair faces never carry skeleton text).

Exit codes: 0 = all faces verified; 4 = seam leakage / device invalid;
1 = harness problem (missing capture/record files).

--selftest builds synthetic clean/contaminated corpora in a temp dir and
proves detection power in BOTH directions (the 312 twelve legs plus
B-R1-specific legs). Exit 0 only if every expectation holds.

This script never talks to the agent or kernel; it only reads artifacts.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
import tempfile
from pathlib import Path

FIVE_FIELDS = ["normalized_role", "display_group", "control_relevance",
               "template_role", "alias"]
S2B_ANCHOR_FIELDS = ["normalized_role", "template_role", "binding_targets"]
S2B_ANCHOR_EXPECTED_ROWS = 7  # 307 receipt: T3_r1 contaminated 7 request rows
PROD_DESC_TAIL = "display_domain_candidate from explain_controls all_parameters"
BARE_DESC = "Set one explicit plugin parameter by normalized value (0.0-1.0)."
TYPED_RULE_FIRST = "- Before freezing an actionable generic EQ semantic_action"
TYPED_APPLY_RE = re.compile(
    r"^(plugin_grabber_apply|apply_(reverb|delay|eq|compressor|limiter|"
    r"de_esser|gate_expander|transient_shaper|multiband))")
TOOL_CALL_RE = re.compile(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)")
LOOP_TOOL_RE = re.compile(r"message_loop\.tool ms=\d+ goal=\S+ tool=([A-Za-z0-9_.\-]+)")
TYPED_ADAPTER_RE = re.compile(
    r"^(plugin_grabber[._](inspect|apply|explain)|apply_(reverb|delay|eq|compressor|"
    r"limiter|de_esser|gate_expander|transient_shaper|multiband))")
CATALOG_GRABBER_LINE_RE = re.compile(r"tool=plugin_grabber\.[A-Za-z0-9_.\-]+")
CATALOG_BARE_READ_RE = re.compile(r"tool=plugin\.get_parameters\b")
CATALOG_BARE_WRITE_RE = re.compile(r"tool=plugin\.set_parameter\b")
CONTROL_SHELL_CONTENT_RE = re.compile(r'"control_shell"\s*:\s*\{')
# R1: B-arm retrieval catalog-line forms (ModelSummary disclosure shape)
CATALOG_WEB_SEARCH_LINE_RE = re.compile(r"web_search tool=web\.search risk=\w+")
CATALOG_WEB_FETCH_LINE_RE = re.compile(r"web_fetch tool=web\.fetch risk=\w+")
# R1 carrier classification: the ordinary loop's tool disclosure is ROUTED
# (agentLoopToolContext): zero capability matches -> FULL ModelCatalogSummary
# (310 form: 209 catalog lines, web pair included); capability matches ->
# filtered packs (310 forms: 42/51 lines, web pair absent). Threshold 150
# separates the two forms robustly (310 corpus: 42/51/209 observed).
CATALOG_LINE_ANY_RE = re.compile(r"[A-Za-z_0-9]+ tool=[A-Za-z0-9_.\-]+ risk=\w+")
FULL_CATALOG_MIN_LINES = 150
# R2: executed retrieval calls in agent-log deltas (both tool families are
# network retrieval; browser.* is the RiskDirect sibling already disclosed
# in the 310 A-pure corpus and never executed there)
RETRIEVAL_EXEC_RE = re.compile(
    r"harness\.invoke total_ms=\d+ tool=(web\.search|web\.fetch|browser\.search|browser\.fetch)\b")
# R1P: the card-317 frozen thin-prompt segment (byte-frozen; the paired Go
# unit test plugin_control_eval_seam_br1_prompt_test.go pins the same bytes)
BR1_PROMPT_SEGMENT = ("你有 web.search/web.fetch 可查插件手册与参数文档；"
                      "遇到不确定的参数编码（param_id/量纲/取值范围）先查再写，"
                      "查过的不重复查")
BR1_PROMPT_SEGMENT_SHA256 = hashlib.sha256(
    BR1_PROMPT_SEGMENT.encode("utf-8")).hexdigest()


def load_capture(path: Path) -> list[dict]:
    rows = []
    if not path.exists():
        return rows
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except json.JSONDecodeError:
            rows.append({"_unparseable": line[:200]})
    return rows


def request_texts(rows: list[dict]) -> list[str]:
    out = []
    for row in rows:
        req = row.get("request")
        if req is None:
            continue
        try:
            out.append(json.dumps(req, ensure_ascii=False))
        except Exception:
            out.append(str(req))
    return out


def count_and_contexts(texts: list[str], needle: str, ctx: int = 60):
    total = 0
    contexts = []
    for text in texts:
        start = 0
        while True:
            idx = text.find(needle, start)
            if idx < 0:
                break
            total += 1
            if len(contexts) < 20:
                contexts.append(text[max(0, idx - ctx):idx + len(needle) + ctx])
            start = idx + len(needle)
    return total, contexts


def regex_rows(texts: list[str], pat: re.Pattern, ctx: int = 60):
    rows = 0
    total = 0
    contexts = []
    for text in texts:
        hits = pat.findall(text)
        if hits:
            rows += 1
            total += len(hits)
            if len(contexts) < 20:
                m = pat.search(text)
                contexts.append(text[max(0, m.start() - ctx):m.end() + ctx])
    return rows, total, contexts


def verify(a_capture: Path, probe_dir: Path, p_capture: Path,
           t3r1_capture: Path) -> dict:
    """Run all check faces; returns the result dict (verdict filled)."""
    a_rows = load_capture(a_capture)
    p_rows = load_capture(p_capture) if p_capture.exists() else []
    a_reqs = request_texts(a_rows)
    p_reqs = request_texts(p_rows)

    result: dict = {"schema_version": "plugin_control_eval_br1_seam_verify.v1",
                    "a_capture": str(a_capture),
                    "a_rows": len(a_rows), "a_request_payloads": len(a_reqs),
                    "p_rows": len(p_rows), "checks": {}, "verdict": ""}
    failures = []

    # ---- S2: five semantic fields absent from B-R1 request payloads
    s2 = {}
    for field in FIVE_FIELDS:
        if field == "alias":
            a_count, a_ctx = count_and_contexts(a_reqs, '"alias"')
            plain_count, plain_ctx = count_and_contexts(a_reqs, "alias")
            p_count, _ = count_and_contexts(p_reqs, '"alias"')
            s2[field] = {"a_keyform_count": a_count, "a_plain_count": plain_count,
                         "p_keyform_count": p_count,
                         "a_plain_contexts": plain_ctx[:10]}
            if a_count > 0:
                failures.append(f"S2 alias key-form present {a_count}x in B-R1 requests")
        else:
            a_count, a_ctx = count_and_contexts(a_reqs, field)
            p_count, _ = count_and_contexts(p_reqs, field)
            s2[field] = {"a_count": a_count, "p_control_count": p_count,
                         "a_contexts": a_ctx[:10]}
            if a_count > 0:
                failures.append(f"S2 {field} present {a_count}x in B-R1 requests")
    weak = [f for f in FIVE_FIELDS if f != "alias" and s2[f]["p_control_count"] == 0]
    result["checks"]["S2_five_fields_absent"] = {"detail": s2, "method_weak_fields": weak}

    # ---- S2b: render-surface negative anchor detection power + B-R1 counts
    s2b: dict = {"anchor_capture": str(t3r1_capture)}
    if t3r1_capture.exists():
        anchor_rows = load_capture(t3r1_capture)
        anchor_reqs = request_texts(anchor_rows)
        anchor_detail = {}
        for field in S2B_ANCHOR_FIELDS:
            n_rows = sum(1 for t in anchor_reqs if field in t)
            anchor_detail[field] = {"anchor_request_rows_hit": n_rows,
                                    "expected_rows": S2B_ANCHOR_EXPECTED_ROWS}
            if n_rows < S2B_ANCHOR_EXPECTED_ROWS:
                failures.append(
                    f"S2b anchor detection power insufficient for {field}: "
                    f"{n_rows}<{S2B_ANCHOR_EXPECTED_ROWS} rows (method broken?)")
        bt_rows, bt_total, _ = regex_rows(a_reqs, re.compile(r'"binding_targets"'))
        key_hits = sum(1 for t in a_reqs if "binding_targets" in t)
        s2b["anchor_detection"] = anchor_detail
        s2b["a_binding_targets_rows"] = bt_rows
        s2b["a_binding_targets_hits"] = key_hits
        s2b["note"] = ("binding_targets is a native key that SURVIVES S2b by "
                       "design (only alias/normalized_role/template_role rows "
                       "are stripped); its presence is informational -- the "
                       "gating assertion is S2's three-key zero-hit")
    else:
        s2b["error"] = "anchor capture missing"
        failures.append("S2b anchor capture missing (detection power unproven)")
    result["checks"]["S2b_render_surface_anchor"] = s2b

    # ---- S3: bare tool description only
    a_tail, a_tail_ctx = count_and_contexts(a_reqs, PROD_DESC_TAIL)
    a_bare, _ = count_and_contexts(a_reqs, BARE_DESC)
    p_tail, _ = count_and_contexts(p_reqs, PROD_DESC_TAIL)
    result["checks"]["S3_bare_tool_description"] = {
        "a_prod_tail_count": a_tail, "a_prod_tail_contexts": a_tail_ctx[:10],
        "a_bare_count": a_bare, "p_prod_tail_count": p_tail,
        "note": "bare_count>0 proves live-stack bare disclosure when the tool "
                "is disclosed; absence is only informational (bounded "
                "disclosure may omit the tool)"}
    if a_tail > 0:
        failures.append(f"S3 production description tail present {a_tail}x in B-R1 requests")

    # ---- S4: typed routing rules trimmed from skeleton
    a_rule, a_rule_ctx = count_and_contexts(a_reqs, TYPED_RULE_FIRST)
    p_rule, _ = count_and_contexts(p_reqs, TYPED_RULE_FIRST)
    result["checks"]["S4_typed_routing_rules_absent"] = {
        "a_count": a_rule,
        "a_contexts": a_rule_ctx[:10], "p_control_count": p_rule}
    if a_rule > 0:
        failures.append(f"S4 typed routing rule present {a_rule}x in B-R1 requests")
    if p_rule == 0:
        result["checks"]["S4_typed_routing_rules_absent"]["method_weak"] = True

    # ---- S5: tool-catalog face
    s5: dict = {}
    grab_rows, grab_hits, grab_ctx = regex_rows(a_reqs, CATALOG_GRABBER_LINE_RE)
    s5["catalog_grabber_line_rows"] = grab_rows
    s5["catalog_grabber_line_hits"] = grab_hits
    s5["catalog_grabber_line_contexts"] = grab_ctx[:10]
    bare_plain, bare_ctx = count_and_contexts(a_reqs, "plugin_grabber")
    read_rows, read_hits, _ = regex_rows(a_reqs, CATALOG_BARE_READ_RE)
    write_rows, write_hits, _ = regex_rows(a_reqs, CATALOG_BARE_WRITE_RE)
    s5["bare_plain_string_hits"] = bare_plain
    s5["bare_plain_contexts"] = bare_ctx[:10]
    s5["catalog_bare_read_rows"] = read_rows
    s5["catalog_bare_write_rows"] = write_rows
    s5["note"] = ("catalog-line form 'tool=plugin_grabber.*' is the 307 "
                  "disclosure form (5666 hits there); plain-string hits are "
                  "informational (known permitted forms: repair-echo of model "
                  "hallucination, goalrunner EQ guard text -- 308 ruling); "
                  "the bare read/write pair MUST be disclosed in catalog-line "
                  "form or the arm has no bare write face at all")
    if grab_hits > 0:
        failures.append(f"S5 plugin_grabber catalog line present {grab_hits}x "
                        f"in B-R1 requests ({grab_rows} rows)")
    if read_hits == 0 or write_hits == 0:
        failures.append(f"S5 bare plugin tools missing from catalog lines: "
                        f"get_parameters_rows={read_hits} set_parameter_rows={write_hits}")
    result["checks"]["S5_catalog_trim"] = s5

    # ---- CS: content-form control_shell backstop
    cs_rows, cs_hits, cs_ctx = regex_rows(a_reqs, CONTROL_SHELL_CONTENT_RE)
    keyname_hits, _ = count_and_contexts(a_reqs, '"control_shell"')
    result["checks"]["CS_control_shell_content_form"] = {
        "content_form_rows": cs_rows, "content_form_hits": cs_hits,
        "content_form_contexts": cs_ctx[:10],
        "keyname_form_hits": keyname_hits,
        "note": ("content form (object value) = device invalid per 308 "
                 "acceptance ruling; key-name list form (result_keys etc.) "
                 "= declared residual, permitted, counted informationally")}
    if cs_hits > 0:
        failures.append(f"CS control_shell content form present {cs_hits}x "
                        f"in B-R1 requests ({cs_rows} rows)")

    # ---- S1 + typed gate: ordinary loop engaged; zero executed typed calls
    record_path = probe_dir / "round_record.json"
    s1: dict = {}
    probe_conversation = ""
    if record_path.exists():
        record = json.loads(record_path.read_text(encoding="utf-8"))
        conv = str(record.get("conversation_id") or "")
        probe_conversation = conv
        delta = str(record.get("agent_log_delta") or "")
        calls = TOOL_CALL_RE.findall(delta)
        loop_calls = LOOP_TOOL_RE.findall(delta)
        all_calls = loop_calls + calls
        typed_apply = [c for c in all_calls if TYPED_APPLY_RE.match(c)]
        typed_any = [c for c in all_calls if TYPED_ADAPTER_RE.match(c)]
        conv_rows = [r for r in a_rows if str(r.get("conversation_id") or "") == conv]
        sources = {}
        for r in conv_rows:
            sources[str(r.get("source") or "?")] = sources.get(str(r.get("source") or "?"), 0) + 1
        s1 = {"conversation_id": conv, "tool_calls": calls,
              "typed_apply_calls": typed_apply,
              "typed_any_calls": typed_any,
              "loop_tool_sequence": loop_calls,
              "capture_sources": sources,
              "semantic_entry_present": sources.get("ordinary_agent_semantic_entry", 0) > 0,
              "message_loop_present": (sources.get("message_loop", 0) +
                                       sources.get("message_loop_repair", 0)) > 0,
              "first_action": record.get("first_action"),
              "final_verdict": (record.get("verdict") or {}).get("final_verdict")}
        if typed_any:
            failures.append(f"S5/S1 executed typed adapter calls present in probe log "
                            f"(catalog trim failed): {sorted(set(typed_any))}")
        if not s1["semantic_entry_present"]:
            failures.append("S1 semantic entry LLM call not found in capture for probe conversation")
        if not s1["message_loop_present"]:
            failures.append("S1 ordinary message-loop LLM call not found in capture for probe conversation")
    else:
        s1 = {"error": "probe round_record.json missing"}
        failures.append("S1 probe round_record.json missing")
    result["checks"]["S1_no_typed_takeover"] = s1

    # ---- R1/R2: retrieval face (312 seventh face, route aware, unchanged)
    r7: dict = {}
    ws_rows, ws_hits, ws_ctx = regex_rows(a_reqs, CATALOG_WEB_SEARCH_LINE_RE)
    wf_rows, wf_hits, wf_ctx = regex_rows(a_reqs, CATALOG_WEB_FETCH_LINE_RE)
    r7["catalog_web_search_line_rows"] = ws_rows
    r7["catalog_web_search_line_hits"] = ws_hits
    r7["catalog_web_search_line_contexts"] = ws_ctx[:5]
    r7["catalog_web_fetch_line_rows"] = wf_rows
    r7["catalog_web_fetch_line_hits"] = wf_hits
    r7["catalog_web_fetch_line_contexts"] = wf_ctx[:5]
    # per-conversation route classification
    conv_face: dict = {}
    for row in a_rows:
        req = row.get("request")
        if req is None:
            continue
        conv = str(row.get("conversation_id") or "")
        try:
            blob = json.dumps(req, ensure_ascii=False)
        except Exception:
            blob = str(req)
        n_lines = len(CATALOG_LINE_ANY_RE.findall(blob))
        has_web = bool(CATALOG_WEB_SEARCH_LINE_RE.search(blob) and
                       CATALOG_WEB_FETCH_LINE_RE.search(blob))
        entry = conv_face.setdefault(conv, {"max_catalog_lines": 0,
                                            "web_pair_present": False})
        entry["max_catalog_lines"] = max(entry["max_catalog_lines"], n_lines)
        entry["web_pair_present"] = entry["web_pair_present"] or has_web
    full_no_web = []
    pack_convs = []
    full_convs = []
    for conv, entry in conv_face.items():
        if entry["max_catalog_lines"] >= FULL_CATALOG_MIN_LINES:
            full_convs.append(conv)
            if not entry["web_pair_present"]:
                full_no_web.append(conv)
        elif entry["max_catalog_lines"] > 0:
            pack_convs.append(conv)
    r7["conversation_route_face"] = {
        "full_catalog_conversations": sorted(full_convs),
        "pack_catalog_conversations": sorted(pack_convs),
        "full_without_web_pair": sorted(full_no_web),
        "threshold": FULL_CATALOG_MIN_LINES,
    }
    if ws_hits == 0 or wf_hits == 0:
        failures.append(f"R1 retrieval catalog lines missing from B-R1 payloads: "
                        f"web.search_rows={ws_rows} web.fetch_rows={wf_rows} "
                        f"(probe carrier must be a full-catalog round; "
                        f"orchestrator probe = T2 repeat 0)")
    if full_no_web:
        failures.append(f"R1c full-catalog conversation(s) WITHOUT the web "
                        f"pair: {sorted(full_no_web)} (device removed the "
                        f"retrieval face from the full disclosure)")
    if record_path.exists():
        delta = str((json.loads(record_path.read_text(encoding="utf-8"))
                     .get("agent_log_delta")) or "")
        exec_calls = RETRIEVAL_EXEC_RE.findall(delta)
        counts: dict = {}
        for c in exec_calls:
            counts[c] = counts.get(c, 0) + 1
        r7["probe_executed_retrieval_calls"] = counts
        r7["note"] = ("retrieval execution is model-initiated: zero calls is a "
                      "legitimate recorded outcome, not a device failure; "
                      "pack-routed rounds cannot see the web pair at all "
                      "(product bounded disclosure, recorded per conversation)")
    result["checks"]["R_retrieval_face"] = r7

    # ---- R1P (8th face, B-R1 thin-prompt injection; card 317)
    # Contract (from the seam implementation + the 312 corpus shape proof):
    #   (a) EVERY message_loop request carries the frozen segment verbatim in
    #       a system-role message (static skeleton + env set for process
    #       lifetime -> per-turn persistence is part of the device contract);
    #   (b) the probe carrier's FIRST message_loop request carries it (the
    #       card's R1 亲证: segment present in the model's first request
    #       system/prompt face);
    #   (c) NO semantic_entry request contains the segment anywhere (system
    #       or user face) -- classifier input byte-identical to arm B; a hit
    #       is a single-variable-design confound = device invalid;
    #   (d) NO message_loop_repair request contains the segment (seam scope
    #       boundary; 312 corpus: repair faces never carry skeleton text).
    r8: dict = {"segment_sha256": BR1_PROMPT_SEGMENT_SHA256,
                "segment_length_utf8_chars": len(BR1_PROMPT_SEGMENT)}
    ml_rows_with_seg = 0
    ml_rows_total = 0
    ml_missing: dict = {}  # conversation -> count of message_loop rows missing segment
    conv_first_ml: dict = {}  # conversation -> (first message_loop row has segment in system face)
    se_rows_with_seg = 0
    repair_rows_with_seg = 0
    for row in a_rows:
        req = row.get("request")
        if req is None:
            continue
        src = str(row.get("source") or "")
        conv = str(row.get("conversation_id") or "")
        messages = req.get("messages") or []
        system_blob = "\n".join(
            str(m.get("content") or "") for m in messages
            if isinstance(m, dict) and m.get("role") == "system")
        full_blob = ""
        try:
            full_blob = json.dumps(req, ensure_ascii=False)
        except Exception:
            full_blob = str(req)
        if src == "message_loop":
            ml_rows_total += 1
            has_seg = BR1_PROMPT_SEGMENT in system_blob
            if has_seg:
                ml_rows_with_seg += 1
            else:
                ml_missing[conv] = ml_missing.get(conv, 0) + 1
            if conv not in conv_first_ml:
                conv_first_ml[conv] = has_seg
        elif src == "ordinary_agent_semantic_entry":
            if BR1_PROMPT_SEGMENT in full_blob:
                se_rows_with_seg += 1
        elif src == "message_loop_repair":
            if BR1_PROMPT_SEGMENT in full_blob:
                repair_rows_with_seg += 1
    r8["message_loop_rows_total"] = ml_rows_total
    r8["message_loop_rows_with_segment_in_system"] = ml_rows_with_seg
    r8["message_loop_rows_missing_segment_by_conversation"] = ml_missing
    r8["conversations_first_message_loop_without_segment"] = sorted(
        [c for c, ok in conv_first_ml.items() if not ok])
    r8["semantic_entry_rows_with_segment"] = se_rows_with_seg
    r8["message_loop_repair_rows_with_segment"] = repair_rows_with_seg
    r8["probe_carrier"] = probe_conversation
    if probe_conversation and probe_conversation in conv_first_ml and not conv_first_ml[probe_conversation]:
        failures.append("R1P probe carrier's FIRST message_loop request does NOT "
                        "carry the frozen segment in its system face (seam not "
                        "live on the running agent)")
    if ml_missing:
        failures.append(f"R1P message_loop request(s) missing the frozen segment "
                        f"(persistence break / seam not live): {ml_missing}")
    if ml_rows_total == 0:
        failures.append("R1P no message_loop requests in capture (cannot verify "
                        "thin-prompt injection)")
    if se_rows_with_seg > 0:
        failures.append(f"R1P frozen segment found in {se_rows_with_seg} "
                        f"semantic_entry request(s) -- classifier input "
                        f"contaminated (single-variable design broken)")
    if repair_rows_with_seg > 0:
        failures.append(f"R1P frozen segment found in {repair_rows_with_seg} "
                        f"message_loop_repair request(s) -- seam leaked beyond "
                        f"the ordinary-family skeleton scope")
    result["checks"]["R1P_thin_prompt_face"] = r8

    # ---- model face (record only)
    models = {}
    for r in a_rows:
        resp = r.get("response")
        model = None
        if isinstance(resp, dict):
            model = str(resp.get("model") or "") or None
        if model:
            models[model] = models.get(model, 0) + 1
    result["checks"]["model_face"] = {"response_models": models,
                                      "note": "deepseek-flash expected (product default)"}

    result["failures"] = failures
    result["verdict"] = "pass" if not failures else "seam_leakage"
    return result


def write_row(path: Path, source: str, conv: str, request, response_model="deepseek-flash"):
    row = {"source": source, "conversation_id": conv,
           "request": request, "response": {"model": response_model}}
    with open(path, "a", encoding="utf-8") as f:
        f.write(json.dumps(row, ensure_ascii=False) + "\n")


def selftest() -> int:
    """Bidirectional synthetic detection-power proof (zero LLM, offline)."""
    tmp = Path(tempfile.mkdtemp(prefix="br1_seam_selftest_"))
    expectations = []  # (label, expected_exit_class) -- "pass" or "fail"

    def run_case(label: str, capture_rows, probe_delta: str | None,
                 anchor: Path | None, expect: str) -> None:
        cap = tmp / f"{label}_capture.jsonl"
        for src, conv, req in capture_rows:
            write_row(cap, src, conv, req)
        probe = tmp / f"{label}_probe"
        probe.mkdir(exist_ok=True)
        rec = {"conversation_id": "conv_" + label,
               "agent_log_delta": probe_delta or "",
               "verdict": {"final_verdict": "pass"},
               "first_action": None}
        (probe / "round_record.json").write_text(
            json.dumps(rec, ensure_ascii=False), encoding="utf-8")
        anchor_path = anchor if anchor is not None else tmp / "anchor_missing.jsonl"
        out = tmp / f"{label}_result.json"
        res = verify(cap, probe, tmp / "p_missing.jsonl", anchor_path)
        out.write_text(json.dumps(res, ensure_ascii=False, indent=2), encoding="utf-8")
        got = "pass" if res["verdict"] == "pass" else "fail"
        status = "OK " if got == expect else "MISMATCH"
        print(f"[selftest] {status} {label}: verdict={res['verdict']} (expected {expect})")
        if got != expect:
            for fl in res["failures"]:
                print(f"    failure: {fl}")
        expectations.append((label, got, expect))

    # minimal healthy B-R1 corpus skeleton shared by clean cases: ordinary
    # loop sources, bare catalog lines, bare tools, the web pair, the frozen
    # segment in the message_loop system face, no contaminants
    def healthy_messages(extra_text: str = "", with_segment: bool = True) -> dict:
        catalog = ("plugin_get_parameters tool=plugin.get_parameters risk=direct\n"
                   "plugin_set_parameter tool=plugin.set_parameter risk=undoable\n"
                   "plugin_scan tool=plugin.scan risk=direct\n"
                   "web_search tool=web.search risk=confirm\n"
                   "web_fetch tool=web.fetch risk=confirm")
        system = catalog + extra_text
        if with_segment:
            system = system + "\n" + BR1_PROMPT_SEGMENT
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": system}],
                "prefer_json": True}

    # P capture surrogate for control faces (used only in the P-control case)
    def p_surrogate_messages() -> dict:
        return {"messages": [
            {"role": "user", "content": (
                "normalized_role here; display_group here; control_relevance here; "
                "template_role here; \"alias\" key form here; "
                "- Before freezing an actionable generic EQ semantic_action first; "
                "display_domain_candidate from explain_controls all_parameters tail")}],
            "prefer_json": True}

    clean_delta = ("message_loop.tool ms=12 goal=g1 tool=plugin.get_parameters\n"
                   "message_loop.tool ms=13 goal=g1 tool=plugin.set_parameter\n")

    # synthetic anchor replicating the real T3_r1 shape: 10 rows, 7 request
    # rows carrying the contaminated binding_targets rendering, 3 clean
    anchor_syn = tmp / "anchor_synthetic.jsonl"
    for _ in range(7):
        write_row(anchor_syn, "message_loop", "conv_anchor_1",
                  {"messages": [{"role": "user", "content": (
                      'binding_targets row: {"param_id":"reverb_mix","alias":"Mix",'
                      '"normalized_role":"mix","template_role":"other"} ...')}],
                   "prefer_json": True})
    for _ in range(3):
        write_row(anchor_syn, "message_loop", "conv_anchor_1",
                  {"messages": [{"role": "user", "content": "clean follow-up"}],
                   "prefer_json": True})

    # 1. clean B-R1 corpus (web pair + segment in message_loop system) -> pass
    run_case("clean_br1", [("ordinary_agent_semantic_entry", "conv_clean_br1",
                            {"messages": [{"role": "user", "content": "case task"},
                                          {"role": "system", "content": "entry arbiter"}],
                             "prefer_json": True}),
                           ("message_loop", "conv_clean_br1", healthy_messages())],
             clean_delta, anchor_syn, "pass")

    # 2. binding_targets semantic-row contamination (307 T3_r1 form) -> fail (S2)
    contaminated = healthy_messages(
        ' result: {"binding_targets":[{"param_id":"x","alias":"A",'
        '"normalized_role":"n","template_role":"t"}]}')
    run_case("binding_targets_semantic_row",
             [("message_loop", "conv_binding_targets_semantic_row", contaminated)],
             clean_delta, anchor_syn, "fail")

    # 3. content-form control_shell -> fail (CS)
    cs_payload = healthy_messages(
        ' result: {"control_shell":{"template_role":"other","hint":"..."}}')
    run_case("control_shell_content_form",
             [("message_loop", "conv_control_shell_content_form", cs_payload)],
             clean_delta, anchor_syn, "fail")

    # 4. plugin_grabber catalog line -> fail (S5)
    grab_catalog = healthy_messages(
        "\nplugin_grabber_apply_reverb_controls tool=plugin_grabber.apply_reverb_controls risk=undoable")
    run_case("grabber_catalog_line",
             [("message_loop", "conv_grabber_catalog_line", grab_catalog)],
             clean_delta, anchor_syn, "fail")

    # 5. executed typed adapter call in probe delta -> fail (S5/S1)
    typed_delta = ("message_loop.tool ms=11 goal=g1 tool=plugin_grabber.apply_reverb_controls\n")
    run_case("executed_typed_call",
             [("ordinary_agent_semantic_entry", "conv_executed_typed_call",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_executed_typed_call", healthy_messages())],
             typed_delta, anchor_syn, "fail")

    # 6. key-name form control_shell (result_keys list) -> pass (declared residual)
    keyname = healthy_messages(
        ' result_summary: {"result_keys":["plugin_id","control_shell","entries"]}')
    run_case("control_shell_keyname_form",
             [("ordinary_agent_semantic_entry", "conv_control_shell_keyname_form",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_control_shell_keyname_form", keyname)],
             clean_delta, anchor_syn, "pass")

    # 7. real 307 T3_r1 anchor used as B-input -> fail (S2 detection power on
    #    the true historical contaminated corpus; also lacks web pair + segment)
    real_anchor = Path(__file__).resolve().parent / "a307_reference" / "t3_r1_contaminated_capture.jsonl"
    if real_anchor.exists():
        probe7 = tmp / "anchor_as_a_probe"
        probe7.mkdir(exist_ok=True)
        first = load_capture(real_anchor)[0]
        rec7 = {"conversation_id": str(first.get("conversation_id") or "x"),
                "agent_log_delta": "", "verdict": {"final_verdict": "pass"},
                "first_action": None}
        (probe7 / "round_record.json").write_text(
            json.dumps(rec7, ensure_ascii=False), encoding="utf-8")
        res7 = verify(real_anchor, probe7, tmp / "p_missing.jsonl", real_anchor)
        out7 = tmp / "anchor_as_a_result.json"
        out7.write_text(json.dumps(res7, ensure_ascii=False, indent=2), encoding="utf-8")
        got = "pass" if res7["verdict"] == "pass" else "fail"
        print(f"[selftest] {'OK ' if got == 'fail' else 'MISMATCH'} real_t3r1_as_binput: "
              f"verdict={res7['verdict']} (expected fail)")
        for fl in res7["failures"][:6]:
            print(f"    failure: {fl}")
        expectations.append(("real_t3r1_as_binput", got, "fail"))
    else:
        print("[selftest] SKIP real_t3r1_as_binput (anchor file not found)")

    # 8. P surrogate as B-input -> fail (S2/S3/S4 detection power)
    run_case("p_surrogate_as_binput",
             [("message_loop", "conv_p_surrogate_as_binput", p_surrogate_messages())],
             clean_delta, anchor_syn, "fail")

    # 9. web pair stripped from the catalog -> fail (R1) -- the B-R1 corpus
    #    must not silently degrade to A-pure
    def a_pure_messages() -> dict:
        catalog = ("plugin_get_parameters tool=plugin.get_parameters risk=direct\n"
                   "plugin_set_parameter tool=plugin.set_parameter risk=undoable")
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": catalog + "\n" + BR1_PROMPT_SEGMENT}],
                "prefer_json": True}
    run_case("web_pair_stripped_r1",
             [("ordinary_agent_semantic_entry", "conv_web_pair_stripped_r1",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_web_pair_stripped_r1", a_pure_messages())],
             clean_delta, anchor_syn, "fail")

    # 10. executed web.search retrieval recorded, clean corpus -> still pass
    retrieval_delta = ("message_loop.tool ms=12 goal=g1 tool=web.search\n"
                       "harness.invoke total_ms=950 tool=web.search\n"
                       "harness.invoke total_ms=310 tool=web.fetch\n")
    run_case("executed_retrieval_clean",
             [("ordinary_agent_semantic_entry", "conv_executed_retrieval_clean",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_executed_retrieval_clean", healthy_messages())],
             retrieval_delta, anchor_syn, "pass")

    # 11. 312-amendment legs: FULL-catalog conversation (>=150 catalog lines)
    #     WITHOUT the web pair must fail R1c (with the segment present so the
    #     failure isolates to R1c)
    def full_catalog_messages(with_web: bool) -> dict:
        lines = [f"tool_{i} tool=dummy.tool_{i} risk=direct" for i in range(160)]
        lines.append("plugin_get_parameters tool=plugin.get_parameters risk=direct")
        lines.append("plugin_set_parameter tool=plugin.set_parameter risk=undoable")
        if with_web:
            lines.append("web_search tool=web.search risk=confirm")
            lines.append("web_fetch tool=web.fetch risk=confirm")
        system = "\n".join(lines) + "\n" + BR1_PROMPT_SEGMENT
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": system}],
                "prefer_json": True}
    run_case("full_catalog_without_web_r1c",
             [("ordinary_agent_semantic_entry", "conv_full_catalog_without_web_r1c",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_full_catalog_without_web_r1c",
               full_catalog_messages(with_web=False)),
              # a second conversation carries the web pair so the R1 CARRIER
              # check passes and the failure isolates to R1c (probe
              # conversation is full-form without the web pair)
              ("message_loop", "conv_other_carrier_r1c", healthy_messages())],
             clean_delta, anchor_syn, "fail")
    run_case("full_catalog_with_web_r1c",
             [("ordinary_agent_semantic_entry", "conv_full_catalog_with_web_r1c",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_full_catalog_with_web_r1c",
               full_catalog_messages(with_web=True))],
             clean_delta, anchor_syn, "pass")

    # 12. B-R1-specific: segment stripped from the message_loop system face
    #     (the arm silently degrading to B -- the exact failure R1P must
    #     catch: seam env not set / agent built without the seam)
    run_case("segment_stripped_r1p",
             [("ordinary_agent_semantic_entry", "conv_segment_stripped_r1p",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_segment_stripped_r1p",
               healthy_messages(with_segment=False))],
             clean_delta, anchor_syn, "fail")

    # 13. B-R1-specific: segment leaked into the semantic_entry request
    #     (classifier input contaminated -- single-variable design broken)
    run_case("segment_in_semantic_entry",
             [("ordinary_agent_semantic_entry", "conv_segment_in_semantic_entry",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter " + BR1_PROMPT_SEGMENT}],
                "prefer_json": True}),
              ("message_loop", "conv_segment_in_semantic_entry", healthy_messages())],
             clean_delta, anchor_syn, "fail")

    # 14. B-R1-specific: persistence break -- second message_loop turn of the
    #     probe conversation missing the segment (seam armed then disarmed)
    run_case("segment_persistence_break",
             [("ordinary_agent_semantic_entry", "conv_segment_persistence_break",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_segment_persistence_break", healthy_messages()),
              ("message_loop", "conv_segment_persistence_break",
               healthy_messages(with_segment=False))],
             clean_delta, anchor_syn, "fail")

    # 15. B-R1-specific: segment leaked into a message_loop_repair request
    #     (seam beyond authorized scope)
    run_case("segment_in_repair_request",
             [("ordinary_agent_semantic_entry", "conv_segment_in_repair_request",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_segment_in_repair_request", healthy_messages()),
              ("message_loop_repair", "conv_segment_in_repair_request",
               {"messages": [{"role": "user", "content": "repair payload " + BR1_PROMPT_SEGMENT},
                             {"role": "system", "content": "You repair outputs."}],
                "prefer_json": True})],
             clean_delta, anchor_syn, "fail")

    # 16. B-R1 clean with repair rows present (repair face WITHOUT the
    #     segment) -> pass (repair absence is the healthy shape)
    run_case("clean_br1_with_repair_rows",
             [("ordinary_agent_semantic_entry", "conv_clean_br1_with_repair_rows",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_clean_br1_with_repair_rows", healthy_messages()),
              ("message_loop_repair", "conv_clean_br1_with_repair_rows",
               {"messages": [{"role": "user", "content": "repair payload"},
                             {"role": "system", "content": "You repair outputs."}],
                "prefer_json": True})],
             clean_delta, anchor_syn, "pass")

    bad = [e for e in expectations if e[1] != e[2]]
    print(f"[selftest] {len(expectations) - len(bad)}/{len(expectations)} expectations met")
    print(f"[selftest] tmp evidence dir: {tmp}")
    return 0 if not bad else 1


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--a-capture", type=Path, default=None)
    ap.add_argument("--probe-dir", type=Path, default=None)
    ap.add_argument("--p-capture", type=Path, default=None)
    ap.add_argument("--t3r1-capture", type=Path, default=None)
    ap.add_argument("--out", type=Path, default=None)
    ap.add_argument("--selftest", action="store_true")
    args = ap.parse_args()

    if args.selftest:
        return selftest()

    here = Path(__file__).resolve().parent
    if args.a_capture is None or args.probe_dir is None or args.out is None:
        print("[verify] --a-capture/--probe-dir/--out required (or --selftest)")
        return 1
    if not args.a_capture.exists():
        print(f"[verify] FAIL: B-R1 capture missing: {args.a_capture}")
        args.out.write_text(json.dumps({"verdict": "harness_invalid",
                                        "error": "a capture missing"}), encoding="utf-8")
        return 1
    p_capture = args.p_capture or (here / "p_baseline" / "agent_llm_payload_capture.jsonl")
    t3r1 = args.t3r1_capture or (here / "a307_reference" / "t3_r1_contaminated_capture.jsonl")

    result = verify(args.a_capture, args.probe_dir, p_capture, t3r1)
    args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[verify] verdict={result['verdict']} a_request_payloads={result['a_request_payloads']}", flush=True)
    for f in result["failures"]:
        print(f"  FAIL: {f}", flush=True)
    for name, chk in result["checks"].items():
        if isinstance(chk, dict) and not chk.get("error"):
            print(f"  {name}: recorded", flush=True)
    return 0 if result["verdict"] == "pass" else 4


if __name__ == "__main__":
    sys.exit(main())
