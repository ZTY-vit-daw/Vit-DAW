#!/usr/bin/env python3
"""PLUGIN-CONTROL-EVAL-S1A-1 seam device verification (card 307).

Gate script run BEFORE the 18 paid task rounds (probe round T1 repeat 0 is
the carrier) and re-run post-corpus as an evidence face. Verifies, from the
per-run LLM payload capture (request payloads = what entered the tested
model's context) plus the probe round record:

  S2  the five semantic overlay fields (normalized_role / display_group /
      control_relevance / template_role / alias) are absent from every
      REQUEST payload; positive control = the frozen P-baseline capture
      (300 segment-p) grepped with the same method must contain the four
      unambiguous fields (method detection power proven first);
  S3  no request carries the production set_plugin_param description tail
      ("display_domain_candidate from explain_controls all_parameters");
      any disclosed set_plugin_param description must be the bare sentence;
  S4  no request carries the typed-routing first rule
      ("- Before freezing an actionable generic EQ semantic_action")
      (P-baseline positive control: 24 hits);
  S1  the probe conversation ran the ordinary loop (semantic entry + message
      loop LLM calls present) with zero typed apply faces in the agent log
      delta (plugin_grabber_apply* / apply_<family>_controls tool calls).

Exit codes: 0 = all faces verified; 4 = seam leakage / device invalid;
1 = harness problem (missing capture/record files).

This script never talks to the agent or kernel; it only reads artifacts.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

FIVE_FIELDS = ["normalized_role", "display_group", "control_relevance",
               "template_role", "alias"]
PROD_DESC_TAIL = "display_domain_candidate from explain_controls all_parameters"
BARE_DESC = "Set one explicit plugin parameter by normalized value (0.0-1.0)."
TYPED_RULE_FIRST = "- Before freezing an actionable generic EQ semantic_action"
TYPED_APPLY_RE = re.compile(
    r"^(plugin_grabber_apply|apply_(reverb|delay|eq|compressor|limiter|"
    r"de_esser|gate_expander|transient_shaper|multiband))")
TOOL_CALL_RE = re.compile(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)")


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


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--a-capture", required=True, type=Path)
    ap.add_argument("--probe-dir", required=True, type=Path)
    ap.add_argument("--p-capture", required=True, type=Path)
    ap.add_argument("--out", required=True, type=Path)
    args = ap.parse_args()

    if not args.a_capture.exists():
        print(f"[verify] FAIL: A capture missing: {args.a_capture}", flush=True)
        args.out.write_text(json.dumps({"verdict": "harness_invalid",
                                        "error": "a capture missing"}), encoding="utf-8")
        return 1
    a_rows = load_capture(args.a_capture)
    p_rows = load_capture(args.p_capture) if args.p_capture.exists() else []
    a_reqs = request_texts(a_rows)
    p_reqs = request_texts(p_rows)

    result: dict = {"schema_version": "plugin_control_eval_s1a_seam_verify.v1",
                    "a_capture": str(args.a_capture),
                    "a_rows": len(a_rows), "a_request_payloads": len(a_reqs),
                    "p_rows": len(p_rows), "checks": {}, "verdict": ""}
    failures = []

    # ---- S2: five semantic fields absent from A request payloads
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
                failures.append(f"S2 alias key-form present {a_count}x in A requests")
            # plain-word hits are reviewed by human (recorded verbatim), not auto-failed
        else:
            a_count, a_ctx = count_and_contexts(a_reqs, field)
            p_count, _ = count_and_contexts(p_reqs, field)
            s2[field] = {"a_count": a_count, "p_control_count": p_count,
                         "a_contexts": a_ctx[:10]}
            if a_count > 0:
                failures.append(f"S2 {field} present {a_count}x in A requests")
    # method detection-power control: the four unambiguous fields must have
    # been findable in P (>=1 each). If not, flag method weakness honestly.
    weak = [f for f in FIVE_FIELDS if f != "alias" and s2[f]["p_control_count"] == 0]
    result["checks"]["S2_five_fields_absent"] = {"detail": s2, "method_weak_fields": weak}

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
        failures.append(f"S3 production description tail present {a_tail}x in A requests")

    # ---- S4: typed routing rules trimmed from skeleton
    a_rule, a_rule_ctx = count_and_contexts(a_reqs, TYPED_RULE_FIRST)
    p_rule, _ = count_and_contexts(p_reqs, TYPED_RULE_FIRST)
    result["checks"]["S4_typed_routing_rules_absent"] = {
        "a_count": a_rule,
        "a_contexts": a_rule_ctx[:10], "p_control_count": p_rule}
    if a_rule > 0:
        failures.append(f"S4 typed routing rule present {a_rule}x in A requests")
    if p_rule == 0:
        result["checks"]["S4_typed_routing_rules_absent"]["method_weak"] = True

    # ---- S1: ordinary loop engaged, no typed apply faces in the probe log
    record_path = args.probe_dir / "round_record.json"
    s1: dict = {}
    if record_path.exists():
        record = json.loads(record_path.read_text(encoding="utf-8"))
        conv = str(record.get("conversation_id") or "")
        delta = str(record.get("agent_log_delta") or "")
        calls = TOOL_CALL_RE.findall(delta)
        typed = [c for c in calls if TYPED_APPLY_RE.match(c)]
        conv_rows = [r for r in a_rows if str(r.get("conversation_id") or "") == conv]
        sources = {}
        for r in conv_rows:
            sources[str(r.get("source") or "?")] = sources.get(str(r.get("source") or "?"), 0) + 1
        s1 = {"conversation_id": conv, "tool_calls": calls,
              "typed_apply_calls": typed, "capture_sources": sources,
              "semantic_entry_present": sources.get("ordinary_agent_semantic_entry", 0) > 0,
              "message_loop_present": (sources.get("message_loop", 0) +
                                       sources.get("message_loop_repair", 0)) > 0,
              "first_action": record.get("first_action"),
              "final_verdict": (record.get("verdict") or {}).get("final_verdict")}
        if typed:
            failures.append(f"S1 typed apply faces present in probe log: {typed}")
        if not s1["semantic_entry_present"]:
            failures.append("S1 semantic entry LLM call not found in capture for probe conversation")
        if not s1["message_loop_present"]:
            failures.append("S1 ordinary message-loop LLM call not found in capture for probe conversation")
    else:
        s1 = {"error": "probe round_record.json missing"}
        failures.append("S1 probe round_record.json missing")
    result["checks"]["S1_no_typed_takeover"] = s1

    # ---- model face (record only): response models observed in A capture
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
    args.out.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[verify] verdict={result['verdict']} a_request_payloads={len(a_reqs)}", flush=True)
    for f in failures:
        print(f"  FAIL: {f}", flush=True)
    for name, chk in result["checks"].items():
        if isinstance(chk, dict) and not chk.get("error"):
            print(f"  {name}: recorded", flush=True)
    return 0 if not failures else 4


if __name__ == "__main__":
    sys.exit(main())
