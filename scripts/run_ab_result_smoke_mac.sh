#!/usr/bin/env bash
# run_ab_result_smoke mac equivalent — PORT-SMOKE-MAC-1 (script 1/7, the anchor).
#
# Mac port of scripts/run_ab_result_smoke.ps1 (the AGENTS §6 health-check
# core: MOM v1.4 AB result smoke). The ps1 is a thin `go test` wrapper, and so
# is this script — no stack is started:
#
#   | ps1 (run_ab_result_smoke.ps1)         | mac (this script)                   |
#   |----------------------------------------|-------------------------------------|
#   | param($RepoRoot = "D:\Vit_DAW")        | --repo-root (default: parent of     |
#   |                                        |   this script's dir)                |
#   | Resolve-Path + go.mod existence check  | same checks in bash                 |
#   | $runPattern = "TestRequestObservation… | identical pattern string (copied    |
#   |   …WorkerConfirmationWritesAndReobse…" |   verbatim, 9 test names)           |
#   | & go test ./internal/mixboard          | go test (same 3 packages, same      |
#   |   ./internal/mom ./internal/chat       |   -run pattern, -count=1)           |
#   |   -run $runPattern -count=1            |                                     |
#   | Fail on $LASTEXITCODE -ne 0            | exit with go test's exit code       |
#
# Mac additions (uniform for the PORT-SMOKE-MAC-1 suite; the ps1 wrote no
# artifacts): each run keeps a fresh artifact dir (run meta with HEAD+dirty,
# full go test log, summary.json). Artifact root default
# ~/Documents/vit-smoke-mac1-artifacts (overridable) — the PC suite wrote into
# the repo Workspace; mac isolates per AGENTS §10.
#
# §8 discipline (pre-declared on the card): deterministic go test — at most 3
# valid runs, success = single run exit 0 with all selected tests passing;
# same-breakpoint two-failure stop-loss applies.

set -uo pipefail

REPO_ROOT_DEFAULT=""

usage() {
  cat <<'EOF'
run_ab_result_smoke_mac.sh — MOM v1.4 AB result smoke (mac, PORT-SMOKE-MAC-1).

Options:
  --repo-root PATH    Repository root (default: parent of this script's dir)
  --artifact-root DIR Artifact root (default ~/Documents/vit-smoke-mac1-artifacts;
                      each run gets a fresh <root>/<run_id>/ dir)
  --workdir PATH      Reuse PATH as this run's artifact dir
  -h, --help          Show this help

Exit codes: 0 = tests passed; 1 = go test failed; 2 = environment failure.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --artifact-root) ARTIFACT_ROOT_ARG="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
command -v go >/dev/null 2>&1 || { echo "ERROR[env]: go not found" >&2; exit 2; }

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

fail_env() { echo "ERROR[env]: $*" >&2; exit 2; }

[[ -f "$REPO_ROOT/agent/go.mod" ]] || fail_env "agent go.mod not found under $REPO_ROOT/agent"

RUN_ID="ab_result_mac_$(date '+%Y%m%d-%H%M%S')"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac1-artifacts}"
WORKDIR="${WORKDIR_ARG:-$ARTIFACT_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR/logs" || fail_env "cannot create artifact dir: $WORKDIR"

# The exact ps1 pattern (9 tests, verbatim).
RUN_PATTERN='TestRequestObservationComputesABResultFromL2RenderProbeAcrossRounds|TestRequestObservationComputesABResultFromExplicitPreviousObservationAcrossSessions|TestRequestObservationMarksABResultStaleWhenRenderRevisionIsReused|TestABResultLayerUsesCompactRenderProbeResult|TestPendingMixTickReportIncludesReadyABResult|TestPendingMixTickReportDoesNotTrustMissingABResult|TestProjectResultCardIncludesReadyABResult|TestProjectResultCardIncludesMissingABResult|TestPluginPrepWorkerConfirmationWritesAndReobserves'

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_ab_result_smoke_mac.sh (mac port of run_ab_result_smoke.ps1)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "go_packages=./internal/mixboard ./internal/mom ./internal/chat"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

echo "" >&2
echo "== Run MOM v1.4 AB result smoke" >&2
set +e
(cd "$REPO_ROOT/agent" && go test ./internal/mixboard ./internal/mom ./internal/chat -run "$RUN_PATTERN" -count=1) \
  2>&1 | tee "$WORKDIR/logs/go_test.log"
GO_EXIT="${PIPESTATUS[0]}"
set -e

SUMMARY_FILE="$WORKDIR/summary.json"
python3 - "$SUMMARY_FILE" "$RUN_ID" "$WORKDIR" "$GO_EXIT" <<'PY'
import json, sys
out, run_id, workdir, go_exit = sys.argv[1:5]
go_exit = int(go_exit)
summary = {
    "schema_version": "ab_result_smoke.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_ab_result_smoke.ps1",
    "run_id": run_id,
    "overall_status": "PASS" if go_exit == 0 else "FAIL",
    "go_test_exit_code": go_exit,
    "gates": {"mom_ab_result_go_tests": go_exit == 0},
    "artifacts_dir": workdir,
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
PY

if [[ "$GO_EXIT" -ne 0 ]]; then
  echo "ERROR[functional]: AB result smoke failed with exit code $GO_EXIT (log: $WORKDIR/logs/go_test.log)" >&2
  exit 1
fi
echo "ok: AB result smoke passed" >&2
echo "summary: $SUMMARY_FILE" >&2
exit 0
