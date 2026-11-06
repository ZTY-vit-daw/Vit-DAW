# free_state_d1_smoke.py — SMOKE-SCRIPT-HYGIENE-1 (260) item (4) constructed
# stub for scripts/free_state_d1_smoke.py (the D1 python runner).
#
# Zero stack, zero LLM: writes the runner report the launcher's phase-gate
# branch detection reads, with the terminal branch selected by the probe via
# the environment (the launcher's argument surface has no branch selector —
# the model branch is a real-run property):
#   VIT260_STUB_BRANCH=zero_mutation -> validation.branch="zero_mutation_boundary"
#                                     (256-legalized zero-mutation terminal form)
#   anything else                    -> validation without a branch key
#                                     (applied / 1-mutation form)
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
    branch = os.environ.get("VIT260_STUB_BRANCH", "applied").strip().lower()
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
    report = {
        "status": "pass",
        "selected_domains": ["reverb"],
        "stub": True,
        "validation": validation,
    }
    with open(known.output, "w", encoding="utf-8") as fh:
        json.dump(report, fh, indent=2)
    print("stub D1 runner done (VIT260_STUB_BRANCH=%s, output=%s)" % (branch, known.output))
    return 0


if __name__ == "__main__":
    sys.exit(main())
