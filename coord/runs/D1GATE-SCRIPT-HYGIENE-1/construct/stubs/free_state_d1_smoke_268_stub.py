# free_state_d1_smoke_268_stub.py — D1GATE-SCRIPT-HYGIENE-1 (268) constructed
# stub for scripts/free_state_d1_smoke.py (the D1 python runner).
#
# Zero stack, zero LLM. Two duties:
#
# 1. Runner report for the launcher's phase-gate branch detection. The report
#    is written BOM-less UTF-8 with ensure_ascii=False and a Chinese notes
#    field (the real runner's write_report shape — the ANSI-default launcher
#    read is exactly what gate-family fix 4 addresses). Branch selected via
#    the environment (the launcher has no branch selector — the model branch
#    is a real-run property):
#      VIT268_STUB_BRANCH=zero_mutation -> validation.branch="zero_mutation_boundary"
#      anything else                    -> validation without a branch key
#    VIT268_STUB_REPORT_ASCII=1 swaps the notes field to ASCII (the control
#    leg that isolates the encoding variable).
#
# 2. The loudness_watch verdict store the settle gate B reads. On applied
#    branches the store is created under the project workdir (.vit_agent
#    subdir, the real store's home) as BOM-less UTF-8 with a Chinese summary
#    flush against the closing quote (the exact byte shape that made the
#    ANSI-default read throw on the 265 run legs). On the zero_mutation
#    branch no store is written (structurally faithful: no applied action ->
#    no watch-hook trigger -> no store, 260's skip-with-reason rationale).
#
# Accepts and ignores every other flag the launcher passes; always exits 0.
import argparse
import json
import os
import sys


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--output", required=True)
    ap.add_argument("--public-manifest")
    ap.add_argument("--public-case-id")
    ap.add_argument("--agent-http")
    ap.add_argument("--timeout-sec")
    ap.add_argument("--project-workdir")
    known, _unknown = ap.parse_known_args()
    branch = os.environ.get("VIT268_STUB_BRANCH", "applied").strip().lower()
    ascii_report = os.environ.get("VIT268_STUB_REPORT_ASCII", "").strip() == "1"

    validation = {"status": "pass", "action_domain": "reverb", "forward_mutation_count": 1}
    if branch == "zero_mutation":
        validation = {
            "status": "pass",
            "branch": "zero_mutation_boundary",
            "action_domain": "reverb",
            "boundary_stop_reason": "capability_blocked",
            "forward_mutation_count": 0,
            "journal_mutation_count": 0,
        }

    notes = "stub report (ascii control leg)" if ascii_report else "构造桩报告：D1 settle 门组卫生修补验证。"
    report = {
        "status": "pass",
        "selected_domains": ["reverb"],
        "stub": True,
        "notes": notes,
        "validation": validation,
    }
    with open(known.output, "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=2, ensure_ascii=False)

    if branch != "zero_mutation" and known.project_workdir:
        store_dir = os.path.join(known.project_workdir, ".vit_agent")
        os.makedirs(store_dir, exist_ok=True)
        store = {
            "schema_version": "loudness_watch_verdicts.v0",
            "verdicts": [
                {
                    "trigger_kind": "d1_settle",
                    "verdict": "pass",
                    "project_revision": "2",
                    "summary": "全局响度检查：读数未越限，判断面如实落账。",
                }
            ],
        }
        with open(os.path.join(store_dir, "loudness_watch_verdicts.json"), "w", encoding="utf-8") as fh:
            json.dump(store, fh, ensure_ascii=False, indent=1)

    print("stub D1 runner done (VIT268_STUB_BRANCH=%s, output=%s)" % (branch, known.output))
    return 0


if __name__ == "__main__":
    sys.exit(main())
