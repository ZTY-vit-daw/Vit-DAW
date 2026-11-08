#!/usr/bin/env python3
r"""PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 seam device verification (card 319).

T1-completion leg of the barefree line: same seam set as card 318
(VIT_PLUGIN_CONTROL_EVAL_SEAM=1 A-pure face PLUS
VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1) with the seam key now ALSO
covering the THIRD layer -- the harness broadMixObserveFirstWriteGuard
(the pre-journal stage that blocked all three 318 T1 rounds). Gate script
run BEFORE the 3 paid T1 task rounds (probe round T1 repeat 0 is the
carrier) and re-run post-corpus as an evidence face. Faces:

  S1/S2/S2b/S3/S4/S5/CS  the 310 six faces unchanged (bare face intact);
  W0  (8th, gating)      A-pure retrieval equivalence: ZERO executed
                         web.search/web.fetch/browser.* calls anywhere
                         (no B seam; any hit = seam residue or behavior
                         break, halt for adjudication);
  P0  (9th, gating)      BR1 thin-prompt segment must NOT appear anywhere
                         (no 317 seam; a byte hit = residue = invalid);
  GW  (10th, gating)     guard-permission corpus face: ZERO typed
                         generic-write guard rejections and ZERO T1
                         write-gate blocks in any request payload (318
                         halves live on every recorded turn);
  GWH (advisory)         harness broad-mix guard error-text counter
                         ("ordinary acoustic mixing requests must run
                         mix.request_observation"): with the 319 seam open
                         this text may still legitimately appear for
                         NON-parameter broad-mix writes (volume/pan/load,
                         which the seam intentionally still blocks), so
                         the count is REPORTED for adjudication, not
                         gating; per-round mechanism attribution lives in
                         consolidate_t1.py (guard_blocked_write);
  GW-LIVE (11th, gating file reference) 318 halves live probe
                         (guard_live_probe.json: T3 victim bare write
                         lands open / rejected again off);
  GW-LIVE-T1 (12th, gating file reference) 319 third-layer live probe
                         (guard_live_probe_t1.json: the exact 318 T1
                         victim -- Pro-R param 6 write WITH the natural-mix
                         task prompt in context -- must LAND open and be
                         rejected again after unset+agent restart).

Exit codes: 0 = all faces verified; 4 = seam leakage / device invalid;
1 = harness problem (missing capture/record files).

--selftest builds synthetic clean/contaminated corpora in a temp dir and
proves detection power in BOTH directions. This script never talks to the
agent or kernel; it only reads artifacts.
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
# route classification (informational here -- catalog web-line visibility
# follows the product's routed bounded disclosure, exactly like A-pure)
CATALOG_LINE_ANY_RE = re.compile(r"[A-Za-z_0-9]+ tool=[A-Za-z0-9_.\-]+ risk=\w+")
CATALOG_WEB_SEARCH_LINE_RE = re.compile(r"web_search tool=web\.search risk=\w+")
CATALOG_WEB_FETCH_LINE_RE = re.compile(r"web_fetch tool=web\.fetch risk=\w+")
FULL_CATALOG_MIN_LINES = 150
# W0: ANY executed retrieval call (direct execution would mean the 312 B
# seam residue; A-pure face parks retrievals on needs_confirmation and the
# four preceding arms show zero initiation)
RETRIEVAL_EXEC_RE = re.compile(
    r"harness\.invoke total_ms=\d+ tool=(web\.search|web\.fetch|browser\.search|browser\.fetch)\b")
# P0: the 317 frozen thin-prompt segment must be absent byte-for-byte
BR1_PROMPT_SEGMENT = ("你有 web.search/web.fetch 可查插件手册与参数文档；"
                      "遇到不确定的参数编码（param_id/量纲/取值范围）先查再写，"
                      "查过的不重复查")
BR1_PROMPT_SEGMENT_SHA256 = hashlib.sha256(
    BR1_PROMPT_SEGMENT.encode("utf-8")).hexdigest()
# GW: typed generic-write guard rejection texts (B-R1 payload dissection,
# T3/T4) and the T1 write-gate tool_gate reason (T1). Their presence in any
# request payload means the barewrite seam is not live on that path.
TYPED_GUARD_REJECTION_RE = re.compile(
    r"typed_(transient_shaper|compressor|limiter|gate_expander|de_esser|eq|multiband)"
    r"_(surface_unavailable|control_required)")
T1_WRITE_GATE_REASON = "broad mixing requests must stop here"
# GWH (319): the harness third-layer guard error text (318 T1 payload
# dissection: pre_journal_validation_failed stage=broad_mix_observe_first_
# write_guard). Advisory counter -- see docstring.
BROAD_MIX_GUARD_TEXT = "ordinary acoustic mixing requests must run mix.request_observation"


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
    texts = []
    for row in rows:
        req = row.get("request")
        if req is None:
            continue
        try:
            texts.append(json.dumps(req, ensure_ascii=False))
        except Exception:
            texts.append(str(req))
    return texts


def count_and_contexts(texts: list[str], needle: str, ctx: int = 60):
    count = 0
    contexts = []
    for text in texts:
        idx = text.find(needle)
        while idx != -1:
            count += 1
            contexts.append(text[max(0, idx - ctx): idx + ctx])
            idx = text.find(needle, idx + len(needle))
    return count, contexts


def regex_rows(texts: list[str], pat: re.Pattern, ctx: int = 60):
    rows = 0
    hits = 0
    contexts = []
    for text in texts:
        found = pat.findall(text)
        if found:
            rows += 1
            hits += len(found)
            m = pat.search(text)
            if m:
                contexts.append(text[max(0, m.start() - ctx): m.end() + ctx])
    return rows, hits, contexts


def verify(a_capture: Path, probe_dir: Path, p_capture: Path,
           t3r1_capture: Path, gw_live_probe: Path | None = None,
           gw_live_t1_probe: Path | None = None) -> dict:
    """Run all check faces; returns the result dict (verdict filled)."""
    a_rows = load_capture(a_capture)
    p_rows = load_capture(p_capture) if p_capture.exists() else []
    a_reqs = request_texts(a_rows)
    p_reqs = request_texts(p_rows)

    result: dict = {"schema_version": "plugin_control_eval_barefree_seam_verify.v1",
                    "a_capture": str(a_capture),
                    "a_rows": len(a_rows), "a_request_payloads": len(a_reqs),
                    "p_rows": len(p_rows), "checks": {}, "verdict": ""}
    failures = []

    # ---- S2: five semantic fields absent from BAREFREE request payloads
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
                failures.append(f"S2 alias key-form present {a_count}x in BAREFREE requests")
        else:
            a_count, a_ctx = count_and_contexts(a_reqs, field)
            p_count, _ = count_and_contexts(p_reqs, field)
            s2[field] = {"a_count": a_count, "p_control_count": p_count,
                         "a_contexts": a_ctx[:10]}
            if a_count > 0:
                failures.append(f"S2 {field} present {a_count}x in BAREFREE requests")
    weak = [f for f in FIVE_FIELDS if f != "alias" and s2[f]["p_control_count"] == 0]
    result["checks"]["S2_five_fields_absent"] = {"detail": s2, "method_weak_fields": weak}

    # ---- S2b: render-surface negative anchor detection power + BAREFREE counts
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
                       "design; presence is informational -- the gating "
                       "assertion is S2's key-form zero-hit")
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
        "note": "bare_count>0 proves live-stack bare disclosure; absence is "
                "informational (bounded disclosure may omit the tool)"}
    if a_tail > 0:
        failures.append(f"S3 production description tail present {a_tail}x in BAREFREE requests")

    # ---- S4: typed routing rules trimmed from skeleton
    a_rule, a_rule_ctx = count_and_contexts(a_reqs, TYPED_RULE_FIRST)
    p_rule, _ = count_and_contexts(p_reqs, TYPED_RULE_FIRST)
    result["checks"]["S4_typed_routing_rules_absent"] = {
        "a_count": a_rule,
        "a_contexts": a_rule_ctx[:10], "p_control_count": p_rule}
    if a_rule > 0:
        failures.append(f"S4 typed routing rule present {a_rule}x in BAREFREE requests")
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
                  "disclosure form; plain-string hits are informational "
                  "(known permitted forms per 308 ruling); the bare "
                  "read/write pair MUST be disclosed in catalog-line form "
                  "or the arm has no bare write face at all")
    if grab_hits > 0:
        failures.append(f"S5 plugin_grabber catalog line present {grab_hits}x "
                        f"in BAREFREE requests ({grab_rows} rows)")
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
        "note": ("content form = device invalid per 308 acceptance ruling; "
                 "key-name list form = declared residual, counted informationally")}
    if cs_hits > 0:
        failures.append(f"CS control_shell content form present {cs_hits}x "
                        f"in BAREFREE requests ({cs_rows} rows)")

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

    # ---- W0 (8th face, gating): A-pure retrieval equivalence -- zero
    # executed retrieval calls in probe delta AND across all request payloads
    # (the capture text is scanned too because harness.invoke lines echo into
    # later request payloads; direct execution anywhere means the 312 B seam
    # residue or a behavior break -- halt for adjudication either way)
    w0: dict = {}
    probe_exec: dict = {}
    if record_path.exists():
        delta = str((json.loads(record_path.read_text(encoding="utf-8"))
                     .get("agent_log_delta")) or "")
        for c in RETRIEVAL_EXEC_RE.findall(delta):
            probe_exec[c] = probe_exec.get(c, 0) + 1
    corpus_rows, corpus_hits, corpus_ctx = regex_rows(a_reqs, RETRIEVAL_EXEC_RE)
    ws_rows, ws_hits, _ = regex_rows(a_reqs, CATALOG_WEB_SEARCH_LINE_RE)
    wf_rows, wf_hits, _ = regex_rows(a_reqs, CATALOG_WEB_FETCH_LINE_RE)
    w0["probe_executed_retrieval_calls"] = probe_exec
    w0["corpus_exec_rows"] = corpus_rows
    w0["corpus_exec_hits"] = corpus_hits
    w0["corpus_exec_contexts"] = corpus_ctx[:5]
    w0["catalog_web_search_line_rows"] = ws_rows  # informational (routed disclosure)
    w0["catalog_web_fetch_line_rows"] = wf_rows   # informational
    w0["note"] = ("BAREFREE has NO B_RETRIEVAL seam: retrieval visibility "
                  "follows the product's routed bounded disclosure exactly "
                  "like A-pure (informational counts); EXECUTION of any "
                  "web/browser retrieval is a gating failure (seam residue "
                  "or behavior break)")
    if probe_exec or corpus_hits > 0:
        failures.append(f"W0 executed retrieval calls present: probe={probe_exec} "
                        f"corpus_rows={corpus_rows} (B seam residue / behavior break)")
    result["checks"]["W0_retrieval_equivalence"] = w0

    # ---- P0 (9th face, gating): BR1 thin-prompt segment absent everywhere
    p0: dict = {"segment_sha256": BR1_PROMPT_SEGMENT_SHA256,
                "segment_length_utf8_chars": len(BR1_PROMPT_SEGMENT)}
    seg_rows, seg_hits, seg_ctx = regex_rows(a_reqs, re.compile(re.escape(BR1_PROMPT_SEGMENT)))
    p0["rows_with_segment"] = seg_rows
    p0["segment_hits"] = seg_hits
    p0["contexts"] = seg_ctx[:5]
    if seg_hits > 0:
        failures.append(f"P0 317 thin-prompt segment present {seg_hits}x in "
                        f"BAREFREE requests ({seg_rows} rows) -- 317 seam residue")
    result["checks"]["P0_br1_segment_absent"] = p0

    # ---- GW (10th face, gating): guard-permission corpus face -- zero typed
    # generic-write guard rejections and zero T1 write-gate blocks in any
    # request payload (their presence = barewrite seam not live)
    gw: dict = {}
    rej_rows, rej_hits, rej_ctx = regex_rows(a_reqs, TYPED_GUARD_REJECTION_RE)
    gw["typed_guard_rejection_rows"] = rej_rows
    gw["typed_guard_rejection_hits"] = rej_hits
    gw["typed_guard_rejection_contexts"] = rej_ctx[:5]
    gate_hits, gate_ctx = count_and_contexts(a_reqs, T1_WRITE_GATE_REASON)
    gate_rows = sum(1 for t in a_reqs if T1_WRITE_GATE_REASON in t)
    gw["t1_write_gate_rows"] = gate_rows
    gw["t1_write_gate_hits"] = gate_hits
    gw["t1_write_gate_contexts"] = gate_ctx[:5]
    gw["note"] = ("B-R1 dissection anchors: T3/T4 rejections were "
                  "typed_transient_shaper_surface_unavailable tool_results "
                  "(chat guard face); T1 was the tool_gate reason text "
                  "(agentloop write-gate face). Zero of both = both halves "
                  "of the barewrite seam live on every recorded turn")
    if rej_hits > 0:
        failures.append(f"GW typed guard rejection present {rej_hits}x "
                        f"({rej_rows} rows) -- chat-side seam NOT live")
    if gate_hits > 0:
        failures.append(f"GW T1 write-gate reason present {gate_hits}x "
                        f"({gate_rows} rows) -- agentloop-side seam NOT live")
    # GWH (advisory, card 319): harness broad-mix guard error-text count.
    # NOT gating: the 319 seam intentionally still blocks non-parameter
    # broad-mix writes, so this text can legitimately appear when the model
    # tries e.g. set_volume on a natural-mix turn. Mechanism attribution
    # per round (guard_blocked_write) is consolidate_t1.py's job.
    gwh_rows, gwh_hits, gwh_ctx = regex_rows(a_reqs, re.compile(re.escape(BROAD_MIX_GUARD_TEXT)))
    gw["broadmix_guard_error_rows"] = gwh_rows
    gw["broadmix_guard_error_hits"] = gwh_hits
    gw["broadmix_guard_error_contexts"] = gwh_ctx[:5]
    gw["broadmix_counter_note"] = ("advisory (card 319): non-parameter "
                                   "broad-mix writes stay blocked by design; "
                                   "hits are adjudicated via consolidate_t1 "
                                   "guard_blocked_write per round, not here")
    result["checks"]["GW_guard_permission_corpus"] = gw

    # ---- GW-LIVE (11th face, gating file reference): the orchestrator's
    # zero-LLM live probe must prove seam-open landing (parameters_changed +
    # readback delta) AND seam-closed rejection (typed guard text back)
    gwl: dict = {"probe_path": str(gw_live_probe) if gw_live_probe else ""}
    if gw_live_probe and Path(gw_live_probe).exists():
        try:
            # utf-8-sig: the orchestrator merges this file with PowerShell
            # Set-Content -Encoding UTF8 (BOM on Windows PowerShell 5.1)
            live = json.loads(Path(gw_live_probe).read_text(encoding="utf-8-sig"))
        except json.JSONDecodeError:
            live = {"_unparseable": True}
        bare = live.get("barewrite_probe") or {}
        off = live.get("gw_off_probe") or {}
        bare_ok = bool(bare.get("ok"))
        off_ok = bool(off.get("ok"))
        gwl["barewrite_probe_ok"] = bare_ok
        gwl["barewrite_probe_detail"] = bare
        gwl["gw_off_probe_ok"] = off_ok
        gwl["gw_off_probe_detail"] = off
        if not bare_ok:
            failures.append("GW-LIVE barewrite probe did not prove seam-open "
                            "landing + readback change (see guard_live_probe.json)")
        if not off_ok:
            failures.append("GW-LIVE seam-closed regression probe did not prove "
                            "guard rejection restored (see guard_live_probe.json)")
    else:
        gwl["error"] = "guard_live_probe.json missing"
        failures.append("GW-LIVE guard_live_probe.json missing (orchestrator "
                        "live legs must run before the gate)")
    result["checks"]["GW_LIVE_guard_permission_probe"] = gwl

    # ---- GW-LIVE-T1 (12th face, gating file reference, card 319): the
    # orchestrator's zero-LLM third-layer probe artifacts
    # (guard_live_probe_t1.json) must prove (a) the exact 318 T1 victim
    # write -- Pro-R param 6 WITH the natural-mix task prompt in context --
    # LANDED with a changed readback while the seam is open and (b) after
    # unset+agent restart the SAME write is guard-rejected again.
    gwt1: dict = {"probe_path": str(gw_live_t1_probe) if gw_live_t1_probe else ""}
    if gw_live_t1_probe and Path(gw_live_t1_probe).exists():
        try:
            # utf-8-sig: the orchestrator merges this file with PowerShell
            # Set-Content -Encoding UTF8 (BOM on Windows PowerShell 5.1)
            live_t1 = json.loads(Path(gw_live_t1_probe).read_text(encoding="utf-8-sig"))
        except json.JSONDecodeError:
            live_t1 = {"_unparseable": True}
        t1_open = live_t1.get("t1_open_probe") or {}
        t1_off = live_t1.get("t1_off_probe") or {}
        t1_open_ok = bool(t1_open.get("ok"))
        t1_off_ok = bool(t1_off.get("ok"))
        gwt1["t1_open_probe_ok"] = t1_open_ok
        gwt1["t1_open_probe_detail"] = t1_open
        gwt1["t1_off_probe_ok"] = t1_off_ok
        gwt1["t1_off_probe_detail"] = t1_off
        if not t1_open_ok:
            failures.append("GW-LIVE-T1 third-layer open probe did not prove "
                            "the T1 victim write lands with the natural-mix "
                            "context (see guard_live_probe_t1.json)")
        if not t1_off_ok:
            failures.append("GW-LIVE-T1 seam-closed regression probe did not "
                            "prove harness-guard rejection restored (see "
                            "guard_live_probe_t1.json)")
    else:
        gwt1["error"] = "guard_live_probe_t1.json missing"
        failures.append("GW-LIVE-T1 guard_live_probe_t1.json missing "
                        "(orchestrator T1 live legs must run before the gate)")
    result["checks"]["GW_LIVE_T1_third_layer_probe"] = gwt1

    result["failures"] = failures
    result["verdict"] = "pass" if not failures else "fail"
    return result


def write_row(path: Path, source: str, conv: str, request, response_model="deepseek-flash"):
    row = {"source": source, "conversation_id": conv,
           "request": request, "response": {"model": response_model}}
    with open(path, "a", encoding="utf-8") as f:
        f.write(json.dumps(row, ensure_ascii=False) + "\n")


def selftest() -> int:
    """Bidirectional synthetic detection-power proof (zero LLM, offline)."""
    tmp = Path(tempfile.mkdtemp(prefix="barefree_seam_selftest_"))
    expectations = []  # (label, expected_exit_class) -- "pass" or "fail"

    # synthetic healthy GW-LIVE probe file (used by clean cases)
    gw_live_ok = tmp / "gw_live_ok.json"
    gw_live_ok.write_text(json.dumps({
        "barewrite_probe": {"ok": True, "wrote_param": "1", "value_before": 0.2,
                            "value_after": 0.35, "status": "ok",
                            "parameters_changed": True},
        "gw_off_probe": {"ok": True, "status": "rejected",
                         "error_contains": "typed_transient_shaper_surface_unavailable"}},
        ensure_ascii=False), encoding="utf-8")

    # synthetic healthy GW-LIVE-T1 probe file (card 319 third layer)
    gw_live_t1_ok = tmp / "gw_live_t1_ok.json"
    gw_live_t1_ok.write_text(json.dumps({
        "t1_open_probe": {"ok": True, "case": "T1", "param_id": "6",
                          "value_before": 1.0, "value_after": 0.35,
                          "write_status": "ok", "broad_mix_text_present": False},
        "t1_off_probe": {"ok": True, "case": "T1", "param_id": "6",
                         "write_status": "error",
                         "broad_mix_text_present": True,
                         "value_unchanged": True}},
        ensure_ascii=False), encoding="utf-8")

    def run_case(label: str, capture_rows, probe_delta: str | None,
                 anchor: Path | None, expect: str,
                 gw_live: Path | None = None,
                 gw_live_t1: Path | None = None) -> None:
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
        gw = gw_live if gw_live is not None else gw_live_ok
        gw1 = gw_live_t1 if gw_live_t1 is not None else gw_live_t1_ok
        res = verify(cap, probe, tmp / "p_missing.jsonl", anchor_path, gw, gw1)
        out = tmp / f"{label}_result.json"
        out.write_text(json.dumps(res, ensure_ascii=False, indent=2), encoding="utf-8")
        got = "pass" if res["verdict"] == "pass" else "fail"
        status = "OK " if got == expect else "MISMATCH"
        print(f"[selftest] {status} {label}: verdict={res['verdict']} (expected {expect})")
        if got != expect:
            for fl in res["failures"]:
                print(f"    failure: {fl}")
        expectations.append((label, got, expect))

    # minimal healthy BAREFREE corpus skeleton: ordinary loop sources, bare
    # catalog lines, NO web pair required, NO BR1 segment, no contaminants
    def healthy_messages(extra_text: str = "") -> dict:
        catalog = ("plugin_get_parameters tool=plugin.get_parameters risk=direct\n"
                   "plugin_set_parameter tool=plugin.set_parameter risk=undoable\n"
                   "plugin_scan tool=plugin.scan risk=direct")
        system = catalog + extra_text
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": system}],
                "prefer_json": True}

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

    # synthetic anchor replicating the real T3_r1 shape
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

    # 1. clean BAREFREE corpus (bare pair, no segment, no web, no guard
    #    rejections, healthy GW-LIVE file) -> pass
    run_case("clean_barefree", [("ordinary_agent_semantic_entry", "conv_clean_barefree",
                                 {"messages": [{"role": "user", "content": "case task"},
                                               {"role": "system", "content": "entry arbiter"}],
                                  "prefer_json": True}),
                                ("message_loop", "conv_clean_barefree", healthy_messages())],
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
    #    the true historical contaminated corpus)
    real_anchor = (Path(__file__).resolve().parent.parent
                   / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "a307_reference"
                   / "t3_r1_contaminated_capture.jsonl")
    if real_anchor.exists():
        probe7 = tmp / "anchor_as_a_probe"
        probe7.mkdir(exist_ok=True)
        first = load_capture(real_anchor)[0]
        rec7 = {"conversation_id": str(first.get("conversation_id") or "x"),
                "agent_log_delta": "", "verdict": {"final_verdict": "pass"},
                "first_action": None}
        (probe7 / "round_record.json").write_text(
            json.dumps(rec7, ensure_ascii=False), encoding="utf-8")
        res7 = verify(real_anchor, probe7, tmp / "p_missing.jsonl", real_anchor,
                      gw_live_ok, gw_live_t1_ok)
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

    # 9. BAREFREE-specific: executed web.search in probe delta -> fail (W0)
    web_delta = ("message_loop.tool ms=12 goal=g1 tool=plugin.get_parameters\n"
                 "harness.invoke total_ms=950 tool=web.search\n")
    run_case("executed_web_search_w0",
             [("ordinary_agent_semantic_entry", "conv_executed_web_search_w0",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_executed_web_search_w0", healthy_messages())],
             web_delta, anchor_syn, "fail")

    # 10. BAREFREE-specific: executed browser.fetch echoed into a later
    #     request payload -> fail (W0 corpus face)
    browser_echo = healthy_messages(
        " harness.invoke total_ms=310 tool=browser.fetch")
    run_case("browser_exec_echo_w0",
             [("message_loop", "conv_browser_exec_echo_w0", browser_echo)],
             clean_delta, anchor_syn, "fail")

    # 11. BAREFREE-specific: BR1 segment residue in the system face -> fail (P0)
    seg_payload = healthy_messages("\n" + BR1_PROMPT_SEGMENT)
    run_case("br1_segment_present_p0",
             [("message_loop", "conv_br1_segment_present_p0", seg_payload)],
             clean_delta, anchor_syn, "fail")

    # 12. BAREFREE-specific: typed guard rejection tool_result echoed into a
    #     later request payload (the exact B-R1 T3/T4 shape) -> fail (GW)
    guard_payload = healthy_messages(
        ' <tool_result>{"execution":{"command_name":"set_plugin_param",'
        '"error":"typed_transient_shaper_surface_unavailable: cannot prove '
        'this parameter is outside the typed transient-shaper surface"}} </tool_result>')
    run_case("typed_guard_rejection_gw",
             [("message_loop", "conv_typed_guard_rejection_gw", guard_payload)],
             clean_delta, anchor_syn, "fail")

    # 13. BAREFREE-specific: T1 write-gate tool_gate reason echoed into a
    #     later request payload (the exact B-R1 T1 shape) -> fail (GW)
    gate_payload = healthy_messages(
        ' <tool_gate>{"blocked_tool":"plugin.get_parameters","reason":"mix.observe '
        'is complete; broad mixing requests must stop here, summarize the '
        'observation, propose one concrete next move, and wait for explicit '
        'user confirmation"}</tool_gate>')
    run_case("t1_write_gate_gw",
             [("message_loop", "conv_t1_write_gate_gw", gate_payload)],
             clean_delta, anchor_syn, "fail")

    # 14. BAREFREE-specific: GW-LIVE probe file missing -> fail (GW-LIVE)
    run_case("gw_live_probe_missing",
             [("ordinary_agent_semantic_entry", "conv_gw_live_probe_missing",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_gw_live_probe_missing", healthy_messages())],
             clean_delta, anchor_syn, "fail",
             gw_live=tmp / "gw_live_missing.json")

    # 15. BAREFREE-specific: GW-LIVE probe with barewrite leg not ok -> fail
    gw_live_bad = tmp / "gw_live_bad.json"
    gw_live_bad.write_text(json.dumps({
        "barewrite_probe": {"ok": False, "status": "rejected",
                            "error_contains": "typed_transient_shaper_surface_unavailable"},
        "gw_off_probe": {"ok": True, "status": "rejected"}}, ensure_ascii=False),
        encoding="utf-8")
    run_case("gw_live_barewrite_rejected",
             [("ordinary_agent_semantic_entry", "conv_gw_live_barewrite_rejected",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_gw_live_barewrite_rejected", healthy_messages())],
             clean_delta, anchor_syn, "fail",
             gw_live=gw_live_bad)

    # 16. clean with repair rows present (repair face healthy) -> pass
    run_case("clean_barefree_with_repair_rows",
             [("ordinary_agent_semantic_entry", "conv_clean_barefree_with_repair_rows",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_clean_barefree_with_repair_rows", healthy_messages()),
              ("message_loop_repair", "conv_clean_barefree_with_repair_rows",
               {"messages": [{"role": "user", "content": "repair payload"},
                             {"role": "system", "content": "You repair outputs."}],
                "prefer_json": True})],
             clean_delta, anchor_syn, "pass")

    # 17. web pair VISIBLE in catalog lines but never executed -> pass
    #     (routed bounded disclosure is the A-pure shape; only execution gates)
    visible_web = healthy_messages(
        "\nweb_search tool=web.search risk=confirm\nweb_fetch tool=web.fetch risk=confirm")
    run_case("web_pair_visible_never_executed",
             [("ordinary_agent_semantic_entry", "conv_web_pair_visible_never_executed",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_web_pair_visible_never_executed", visible_web)],
             clean_delta, anchor_syn, "pass")

    # 18. card 319: GW-LIVE-T1 probe file missing -> fail (GW-LIVE-T1)
    run_case("gw_live_t1_probe_missing",
             [("ordinary_agent_semantic_entry", "conv_gw_live_t1_probe_missing",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_gw_live_t1_probe_missing", healthy_messages())],
             clean_delta, anchor_syn, "fail",
             gw_live_t1=tmp / "gw_live_t1_missing.json")

    # 19. card 319: GW-LIVE-T1 open leg not ok (T1 victim write did not
    #     land with the natural-mix context) -> fail (GW-LIVE-T1)
    gw_live_t1_bad_open = tmp / "gw_live_t1_bad_open.json"
    gw_live_t1_bad_open.write_text(json.dumps({
        "t1_open_probe": {"ok": False, "write_status": "error",
                          "broad_mix_text_present": True},
        "t1_off_probe": {"ok": True}}, ensure_ascii=False), encoding="utf-8")
    run_case("gw_live_t1_open_not_ok",
             [("ordinary_agent_semantic_entry", "conv_gw_live_t1_open_not_ok",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_gw_live_t1_open_not_ok", healthy_messages())],
             clean_delta, anchor_syn, "fail",
             gw_live_t1=gw_live_t1_bad_open)

    # 20. card 319: harness broad-mix guard error text echoed into a
    #     payload (the exact 318 T1 stage shape) -> advisory GWH counter
    #     records it but the verdict STAYS pass (non-gating by design;
    #     per-round attribution belongs to consolidate_t1)
    broadmix_payload = healthy_messages(
        ' <tool_result>{"execution":{"command_name":"set_volume","error":"'
        + BROAD_MIX_GUARD_TEXT + '"}} </tool_result>')
    run_case("broadmix_text_advisory_gwh",
             [("ordinary_agent_semantic_entry", "conv_broadmix_text_advisory_gwh",
               {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": "entry arbiter"}],
                "prefer_json": True}),
              ("message_loop", "conv_broadmix_text_advisory_gwh", broadmix_payload)],
             clean_delta, anchor_syn, "pass")
    gwh_result = json.loads((tmp / "broadmix_text_advisory_gwh_result.json").read_text(encoding="utf-8"))
    gwh_hits = (gwh_result.get("checks", {}).get("GW_guard_permission_corpus", {})
                .get("broadmix_guard_error_hits"))
    if gwh_hits:
        print(f"[selftest] OK  broadmix_advisory_counter: hits={gwh_hits} (recorded, verdict unchanged)")
        expectations.append(("broadmix_advisory_counter_recorded", "pass", "pass"))
    else:
        print("[selftest] MISMATCH broadmix_advisory_counter: GWH face did not record the hit")
        expectations.append(("broadmix_advisory_counter_recorded", "fail", "pass"))

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
    ap.add_argument("--gw-live-probe", type=Path, default=None,
                    help="orchestrator guard_live_probe.json (GW-LIVE face)")
    ap.add_argument("--gw-live-t1-probe", type=Path, default=None,
                    help="orchestrator guard_live_probe_t1.json (GW-LIVE-T1 face, card 319)")
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
        print(f"[verify] FAIL: BAREFREE capture missing: {args.a_capture}")
        args.out.write_text(json.dumps({"verdict": "harness_invalid",
                                        "error": "a capture missing"}), encoding="utf-8")
        return 1
    p_capture = args.p_capture or (here.parent / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "p_baseline" / "agent_llm_payload_capture.jsonl")
    t3r1 = args.t3r1_capture or (here.parent / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "a307_reference" / "t3_r1_contaminated_capture.jsonl")
    gw = args.gw_live_probe or (here / "guard_live_probe.json")
    gw1 = args.gw_live_t1_probe or (here / "guard_live_probe_t1.json")

    result = verify(args.a_capture, args.probe_dir, p_capture, t3r1, gw, gw1)
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
