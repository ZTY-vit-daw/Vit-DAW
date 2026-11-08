#!/usr/bin/env python3
r"""PLUGIN-CONTROL-EVAL-S1B-1 seam device verification (card 312, B arm).

Gate script run BEFORE the 18 paid task rounds (probe round T1 repeat 0 is
the carrier) and re-run post-corpus as an evidence face. B-arm seam set =
the full A-pure face (VIT_PLUGIN_CONTROL_EVAL_SEAM=1 = 305 S1-S4 + 308 S5
catalog trim + S2b render strip; all six 310 faces reused verbatim as faces
1-6) PLUS the B-specific retrieval face:

  R1 (7th face, gating) retrieval disclosure: the model tool catalog in the
      B request payloads MUST carry the web.search / web.fetch catalog-line
      forms (web_search tool=web.search risk=... / web_fetch tool=web.fetch
      risk=...). The B arm is "A-pure + web.search/web.fetch enabled"
      (plan.json B_star: same_as_A_star + retrieval); a missing retrieval
      face means the B arm is not the B arm -> device invalid;
  R2 (7th face, informational + consistency) executed retrieval calls
      (harness.invoke tool=web.search / web.fetch / browser.search /
      browser.fetch in the probe/round log deltas) are counted and recorded;
      retrieval is model-initiated so ZERO executions is a legitimate
      arm outcome (recorded, not a failure); an executed web.* call with a
      zero R1 disclosure would be inconsistent (gating already covers it).

Faces 1-6 are the 310 seam_verify_apure faces unchanged (S2 five-field
absence + P control, S2b 307 T3_r1 negative anchor, S3 bare description,
S4 typed-routing rule absence, S5 catalog trim + bare pair presence, CS
control_shell content-form, S1 semantic-entry/message-loop + zero executed
typed calls). The A-pure zero-regression requirement (card face 3) is
exactly these six faces passing on the B corpus.

Exit codes: 0 = all faces verified; 4 = seam leakage / device invalid;
1 = harness problem (missing capture/record files).

--selftest builds synthetic clean/contaminated corpora in a temp dir and
proves detection power in BOTH directions (310's eight legs plus B-specific
legs: web catalog line present on the clean B corpus passes; a corpus with
the web pair stripped fails R1). Exit 0 only if every expectation holds.

This script never talks to the agent or kernel; it only reads artifacts.
"""
from __future__ import annotations

import argparse
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
# R2: executed retrieval calls in agent-log deltas (both tool families are
# network retrieval; browser.* is the RiskDirect sibling already disclosed
# in the 310 A-pure corpus and never executed there)
RETRIEVAL_EXEC_RE = re.compile(
    r"harness\.invoke total_ms=\d+ tool=(web\.search|web\.fetch|browser\.search|browser\.fetch)\b")


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

    result: dict = {"schema_version": "plugin_control_eval_s1b_seam_verify.v1",
                    "a_capture": str(a_capture),
                    "a_rows": len(a_rows), "a_request_payloads": len(a_reqs),
                    "p_rows": len(p_rows), "checks": {}, "verdict": ""}
    failures = []

    # ---- S2: five semantic fields absent from B request payloads
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
                failures.append(f"S2 alias key-form present {a_count}x in B requests")
        else:
            a_count, a_ctx = count_and_contexts(a_reqs, field)
            p_count, _ = count_and_contexts(p_reqs, field)
            s2[field] = {"a_count": a_count, "p_control_count": p_count,
                         "a_contexts": a_ctx[:10]}
            if a_count > 0:
                failures.append(f"S2 {field} present {a_count}x in B requests")
    weak = [f for f in FIVE_FIELDS if f != "alias" and s2[f]["p_control_count"] == 0]
    result["checks"]["S2_five_fields_absent"] = {"detail": s2, "method_weak_fields": weak}

    # ---- S2b: render-surface negative anchor detection power + B counts
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
        failures.append(f"S3 production description tail present {a_tail}x in B requests")

    # ---- S4: typed routing rules trimmed from skeleton
    a_rule, a_rule_ctx = count_and_contexts(a_reqs, TYPED_RULE_FIRST)
    p_rule, _ = count_and_contexts(p_reqs, TYPED_RULE_FIRST)
    result["checks"]["S4_typed_routing_rules_absent"] = {
        "a_count": a_rule,
        "a_contexts": a_rule_ctx[:10], "p_control_count": p_rule}
    if a_rule > 0:
        failures.append(f"S4 typed routing rule present {a_rule}x in B requests")
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
                        f"in B requests ({grab_rows} rows)")
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
                        f"in B requests ({cs_rows} rows)")

    # ---- S1 + typed gate: ordinary loop engaged; zero executed typed calls
    record_path = probe_dir / "round_record.json"
    s1: dict = {}
    if record_path.exists():
        record = json.loads(record_path.read_text(encoding="utf-8"))
        conv = str(record.get("conversation_id") or "")
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

    # ---- R1/R2: B-arm retrieval face (7th face)
    r7: dict = {}
    ws_rows, ws_hits, ws_ctx = regex_rows(a_reqs, CATALOG_WEB_SEARCH_LINE_RE)
    wf_rows, wf_hits, wf_ctx = regex_rows(a_reqs, CATALOG_WEB_FETCH_LINE_RE)
    r7["catalog_web_search_line_rows"] = ws_rows
    r7["catalog_web_search_line_hits"] = ws_hits
    r7["catalog_web_search_line_contexts"] = ws_ctx[:5]
    r7["catalog_web_fetch_line_rows"] = wf_rows
    r7["catalog_web_fetch_line_hits"] = wf_hits
    r7["catalog_web_fetch_line_contexts"] = wf_ctx[:5]
    if ws_hits == 0 or wf_hits == 0:
        failures.append(f"R1 retrieval catalog lines missing from B payloads: "
                        f"web.search_rows={ws_rows} web.fetch_rows={wf_rows} "
                        f"(B arm requires the retrieval face disclosed)")
    if record_path.exists():
        delta = str((json.loads(record_path.read_text(encoding="utf-8"))
                     .get("agent_log_delta")) or "")
        exec_calls = RETRIEVAL_EXEC_RE.findall(delta)
        counts: dict = {}
        for c in exec_calls:
            counts[c] = counts.get(c, 0) + 1
        r7["probe_executed_retrieval_calls"] = counts
        r7["note"] = ("retrieval execution is model-initiated: zero calls is a "
                      "legitimate recorded outcome, not a device failure")
    result["checks"]["R_retrieval_face"] = r7

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
    tmp = Path(tempfile.mkdtemp(prefix="s1b_seam_selftest_"))
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

    # minimal healthy B-corpus skeleton shared by clean cases: ordinary loop
    # sources, bare catalog lines, bare tools, the web pair, no contaminants
    def healthy_messages(extra_text: str = "") -> dict:
        catalog = ("plugin_get_parameters tool=plugin.get_parameters risk=direct\n"
                   "plugin_set_parameter tool=plugin.set_parameter risk=undoable\n"
                   "plugin_scan tool=plugin.scan risk=direct\n"
                   "web_search tool=web.search risk=confirm\n"
                   "web_fetch tool=web.fetch risk=confirm")
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": catalog + extra_text}],
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

    # 1. clean B corpus (web pair disclosed) -> pass
    run_case("clean_b", [("ordinary_agent_semantic_entry", "conv_clean_b", healthy_messages()),
                         ("message_loop", "conv_clean_b", healthy_messages())],
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
             [("ordinary_agent_semantic_entry", "conv_executed_typed_call", healthy_messages()),
              ("message_loop", "conv_executed_typed_call", healthy_messages())],
             typed_delta, anchor_syn, "fail")

    # 6. key-name form control_shell (result_keys list) -> pass (declared residual)
    keyname = healthy_messages(
        ' result_summary: {"result_keys":["plugin_id","control_shell","entries"]}')
    run_case("control_shell_keyname_form",
             [("ordinary_agent_semantic_entry", "conv_control_shell_keyname_form",
               healthy_messages()),
              ("message_loop", "conv_control_shell_keyname_form", keyname)],
             clean_delta, anchor_syn, "pass")

    # 7. real 307 T3_r1 anchor used as B-input -> fail (S2 detection power on
    #    the true historical contaminated corpus; it also lacks the B web pair
    #    so R1 fires there too -- both faces detect it)
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

    # 9. B-specific: web pair stripped from the catalog -> fail (R1) -- this
    #    is the "A-pure face without retrieval" negative anchor: if the seam
    #    or the catalog ever dropped the web tools the B arm would silently
    #    degrade to A-pure; R1 must catch exactly that
    def a_pure_messages() -> dict:
        catalog = ("plugin_get_parameters tool=plugin.get_parameters risk=direct\n"
                   "plugin_set_parameter tool=plugin.set_parameter risk=undoable")
        return {"messages": [{"role": "user", "content": "case task"},
                             {"role": "system", "content": catalog}],
                "prefer_json": True}
    run_case("web_pair_stripped_r1",
             [("ordinary_agent_semantic_entry", "conv_web_pair_stripped_r1", a_pure_messages()),
              ("message_loop", "conv_web_pair_stripped_r1", a_pure_messages())],
             clean_delta, anchor_syn, "fail")

    # 10. B-specific: executed web.search retrieval recorded, clean corpus ->
    #     still pass (zero-execution is not required; execution is legitimate)
    retrieval_delta = ("message_loop.tool ms=12 goal=g1 tool=web.search\n"
                       "harness.invoke total_ms=950 tool=web.search\n"
                       "harness.invoke total_ms=310 tool=web.fetch\n")
    run_case("executed_retrieval_clean",
             [("ordinary_agent_semantic_entry", "conv_executed_retrieval_clean", healthy_messages()),
              ("message_loop", "conv_executed_retrieval_clean", healthy_messages())],
             retrieval_delta, anchor_syn, "pass")

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
        print(f"[verify] FAIL: B capture missing: {args.a_capture}")
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
