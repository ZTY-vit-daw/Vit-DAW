#!/usr/bin/env bash
# run_observation_v1_acceptance_smoke mac equivalent — PORT-SMOKE-MAC-1
# (script 7/7).
#
# Mac port of scripts/run_observation_v1_acceptance_smoke.ps1: the
# observation-v1 acceptance ORCHESTRATOR. Same step chain, same fail-fast
# semantics (a failed step aborts the run with exit 1, exactly like
# Invoke-LoggedStep + throw in the ps1):
#
#   | ps1 (run_observation_v1_acceptance_       | mac (this script)            |
#   |        smoke.ps1)                         |                              |
#   |--------------------------------------------|------------------------------|
#   | param($RepoRoot, $GodotProjectRoot =      | flags below; mac defaults:   |
#   |   D:\Godot\project\vit-daw-frontend,       |   ~/Documents/vit-daw-front- |
#   |   $GodotExe = D:\Godot\…console.exe,       |   end and ~/Applications/    |
#   |   $KernelExe, $MaterialPath, $PythonExe,   |   Godot.app/Contents/MacOS/  |
#   |   -SkipGodotHeadless, -SkipGoTests,        |   Godot                      |
#   |   -SkipKernelSmokes, -SkipProductPath,     | same skip flags              |
#   |   -SkipBuild, -Reuse*, $TimeoutSeconds=60) |                              |
#   | Artifacts under repo VitApp\Workspace\     | artifacts under a fresh run  |
#   |   Artifacts\smoke\observation_v1_…         |   dir (mac isolation, §10)   |
#   | summary observation_v1_acceptance_         | .mac.v1 + per-step exit      |
#   |   smoke.v1, steps[] with logs + timings    |   codes + timings            |
#   | Step "Godot headless parse": & Godot       | $GODOT_BIN --headless        |
#   |   --headless --path … --quit               |   --path … --quit            |
#   | Step "Observation Go regression set":      | identical package set +      |
#   |   go test 6 packages -count=1              |   -count=1                   |
#   | Step "DAD L3 package smoke": runs          | dad_probe.py against a mac-  |
#   |   run_dad_l3_package_smoke.ps1 (kernel +   | started isolated kernel with |
#   |   dad_probe.py, features                   |   the same features string   |
#   |   waveform_envelope,spectral_field,        |   waveform_envelope,         |
#   |   l3_acoustic_summary) + summary asserts   |   spectral_field,            |
#   |   (status ready/suspect, observed L3       |   l3_acoustic_summary) + the |
#   |   features + evidence_refs + bounded       |   same summary asserts       |
#   |   producer facts)                          |                              |
#   | Step "L2 render probe smoke": dad_probe    | same, features waveform_     |
#   |   features …,l2_render_probe + validate    |   envelope,spectral_field,   |
#   |   l2_render_probe ready/suspect + no       |   l2_render_probe + same     |
#   |   raw_leak_keys                            |   validation                 |
#   | Step "L2 realtime observation smoke":      | seed snapshot + env + Godot  |
#   |   seed snapshot JSON + env + Godot         |   --headless --script (the   |
#   |   --headless --script probe.gd + the full  |   .gd probe is cross-        |
#   |   ready/live/deferred matrix asserts       |   platform) + same asserts   |
#   | Step "AB result smoke": run_ab_result_     | sibling run_ab_result_smoke_ |
#   |   smoke.ps1                                |   mac.sh                     |
#   | Step "Godot product-path lifecycle smoke": | sibling run_vit_product_path |
#   |   run_vit_product_path_smoke.ps1           |   _smoke_mac.sh (two-piece   |
#   |                                            |   adaptation, declared on    |
#   |                                            |   that script)               |
#
# KNOWN mac interface gap (declared on the card, evidence-first): the two DAD
# steps require scripts/dad_probe.py to READ kernel shared memory, and its
# reader uses Windows-only `mmap(tagname=...)` (Python raises TypeError on
# macOS). The probe catches the exception and marks the feature
# failed("shared_memory_read_failed") — so the DAD steps are EXPECTED to fail
# on mac until a POSIX-shm reader lands (out of this card's file domain:
# scripts/*.py changes are not among the ≤7 new _mac.sh files). This script
# still runs them once as-is for evidence; use --skip-kernel-smokes to
# exercise the remaining steps.
#
# §8 discipline: deterministic chain (the product-path sub-step carries its
# own LLM budget declared on its script) — ≤3 valid runs; success = single
# run exit 0 (all steps green).

set -uo pipefail

REPO_ROOT_DEFAULT=""
GODOT_PROJECT_ROOT="$HOME/Documents/vit-daw-frontend"
GODOT_BIN="$HOME/Applications/Godot.app/Contents/MacOS/Godot"
KERNEL_BIN_ARG=""
MATERIAL_PATH=""
PYTHON_BIN="python3"
SKIP_GODOT_HEADLESS=0
SKIP_GO_TESTS=0
SKIP_KERNEL_SMOKES=0
SKIP_DAD_SMOKES=0
SKIP_PRODUCT_PATH=0
TIMEOUT_SECONDS=60
WORKDIR_ARG=""
TRACKTION_DIR=""
ARTIFACT_ROOT_ARG=""

ZMQ_REQ_PORT=5555
ZMQ_SUB_PORT=5556
ZMQ_LOG_PORT=5557

usage() {
  cat <<'EOF'
run_observation_v1_acceptance_smoke_mac.sh — mac port of
run_observation_v1_acceptance_smoke.ps1 (PORT-SMOKE-MAC-1). Orchestrates:
godot headless parse -> observation go regression -> DAD L3 package smoke ->
L2 render probe smoke -> L2 realtime observation smoke -> AB result smoke ->
product-path smoke. Fail-fast like the ps1.

Options:
  --repo-root PATH          Repository root (default: parent of this script's dir)
  --godot-project-root P    Godot project root (default ~/Documents/vit-daw-frontend)
  --godot-bin PATH          Godot binary (default ~/Applications/Godot.app/.../Godot)
  --kernel-bin PATH         Reuse an existing VitApp kernel binary (DAD steps)
  --material-path PATH      Probe material (default: repo Paper Crown.mp3 if
                            present, else test_100hz_10s.wav — same fallback
                            order as the ps1)
  --python PATH             Python interpreter (default python3; needs pyzmq)
  --skip-godot-headless     Skip the godot headless parse step
  --skip-go-tests           Skip the observation go regression step
  --skip-kernel-smokes      Skip the DAD L3 + L2 render probe + L2 realtime steps
  --skip-dad-smokes         (mac evidence granularity, declared addition) skip
                            only the two shm-dependent DAD steps, keep the
                            L2 realtime Godot step
  --skip-product-path       Skip the product-path step
  --timeout-seconds N       Startup/REQ timeouts (default 60)
  --workdir PATH            Reuse PATH as the run artifact dir
  --tracktion-dir PATH      tracktion_engine source dir (kernel build only)
  --artifact-root DIR       Artifact root (default ~/Documents/vit-smoke-mac1-artifacts)
  -h, --help                Show this help

Exit codes: 0 = all steps passed; 1 = a step failed; 2 = environment failure.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --godot-project-root) GODOT_PROJECT_ROOT="$2"; shift 2 ;;
    --godot-bin) GODOT_BIN="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --material-path) MATERIAL_PATH="$2"; shift 2 ;;
    --python) PYTHON_BIN="$2"; shift 2 ;;
    --skip-godot-headless) SKIP_GODOT_HEADLESS=1; shift ;;
    --skip-go-tests) SKIP_GO_TESTS=1; shift ;;
    --skip-kernel-smokes) SKIP_KERNEL_SMOKES=1; shift ;;
    --skip-dad-smokes) SKIP_DAD_SMOKES=1; shift ;;
    --skip-product-path) SKIP_PRODUCT_PATH=1; shift ;;
    --timeout-seconds) TIMEOUT_SECONDS="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    --artifact-root) ARTIFACT_ROOT_ARG="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in go cmake make python3 lsof shasum curl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" >&2; }
fail_env()       { OUTCOME="env_failure"; echo "ERROR[env]: $*" >&2; exit 2; }
fail_functional(){ OUTCOME="assertion_failed"; echo "ERROR[functional]: $*" >&2; exit 1; }

json_field() {
  python3 - "$1" "$2" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
print(eval(sys.argv[2], {"d": d}))
PY
}

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

RUN_ID="observation_v1_accept_mac_$(date '+%Y%m%d-%H%M%S')"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac1-artifacts}"
WORKDIR="${WORKDIR_ARG:-$ARTIFACT_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR/logs" || fail_env "cannot create artifact dir: $WORKDIR"
SUMMARY_PATH="$WORKDIR/summary.json"
STEPS_ENV="$WORKDIR/steps.env"
: > "$STEPS_ENV"

KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"
KERNEL_WORKSPACE="$WORKDIR/kernel_workspace"
mkdir -p "$KERNEL_WORKSPACE"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"

KERNEL_PID=""
KERNEL_BIN=""
OUTCOME="env_failure"

record_step() {
  # record_step <name> <status> <exit_code|-> [extra key=value]...
  local name="$1" status="$2" exit_code="${3:--}" line="name=$1|status=$2|exit_code=$3"
  shift 3 || true
  for kv in "$@"; do line="$line|$kv"; done
  echo "$line" >> "$STEPS_ENV"
}

write_summary() {
  python3 - "$SUMMARY_PATH" "$RUN_ID" "$WORKDIR" "$1" "${2:-}" "$STEPS_ENV" "$REPO_ROOT" <<'PY'
import json, os, sys
out, run_id, workdir, status, error, steps_env, repo_root = sys.argv[1:8]
steps = []
for line in open(steps_env, encoding="utf-8"):
    line = line.strip()
    if not line:
        continue
    kv = {}
    for part in line.split("|"):
        if "=" in part:
            k, v = part.split("=", 1)
            kv[k] = v
    steps.append(kv)
summary = {
    "schema_version": "observation_v1_acceptance_smoke.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_observation_v1_acceptance_smoke.ps1",
    "run_id": run_id,
    "status": status,
    "error": error,
    "repo_root": repo_root,
    "repo_head": os.popen(f"git -C {repo_root} rev-parse HEAD 2>/dev/null").read().strip() or "unknown",
    "artifact_dir": workdir,
    "steps": steps,
    "artifacts": {
        "godot_headless_parse_log": "logs/godot_headless_parse.log",
        "go_observation_regression_log": "logs/go_observation_regression.log",
        "dad_l3_summary": "dad_l3/dad_probe_summary.json",
        "l2_render_probe_summary": "l2_render_probe/dad_probe_summary.json",
        "l2_realtime_observation_dir": "l2_realtime/",
        "ab_result_log": "logs/ab_result_smoke.log",
        "product_path_log": "logs/product_path_smoke.log",
    },
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
PY
}

cleanup() {
  local rc=$?
  trap - EXIT INT TERM
  if [[ -n "$KERNEL_PID" ]]; then
    log "stopping kernel (pid $KERNEL_PID)..."
    kill -TERM "$KERNEL_PID" 2>/dev/null || true
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do kill -0 "$KERNEL_PID" 2>/dev/null || break; sleep 1; done
    kill -0 "$KERNEL_PID" 2>/dev/null && kill -KILL "$KERNEL_PID" 2>/dev/null || true
    wait "$KERNEL_PID" 2>/dev/null || true
    KERNEL_PID=""
  fi
  write_summary "$([[ "$OUTCOME" == "all_green" ]] && echo passed || echo failed)" "$FATAL_MSG"
  log "summary: $SUMMARY_PATH"
  log "workdir: $WORKDIR"
  case "$OUTCOME" in
    all_green) exit 0 ;;
    assertion_failed) exit 1 ;;
    *) exit 2 ;;
  esac
}
trap cleanup EXIT INT TERM
FATAL_MSG=""

# ---------------------------------------------------------------- preflight
step "Preflight"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_observation_v1_acceptance_smoke_mac.sh (mac port of run_observation_v1_acceptance_smoke.ps1)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "godot_bin=$GODOT_BIN"
  echo "godot_project_root=$GODOT_PROJECT_ROOT"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$REPO_ROOT/agent/go.mod" ]] || fail_env "agent go.mod not found under $REPO_ROOT/agent"
[[ -f "$REPO_ROOT/scripts/dad_probe.py" ]] || fail_env "dad_probe.py missing"
"$PYTHON_BIN" -c "import zmq" 2>/dev/null || fail_env "$PYTHON_BIN cannot import zmq (pyzmq)"

# ---------------------------------------------------------------- step 1
if [[ "$SKIP_GODOT_HEADLESS" -ne 1 ]]; then
  step "Godot headless parse"
  [[ -x "$GODOT_BIN" ]] || fail_env "Godot executable not found: $GODOT_BIN"
  [[ -f "$GODOT_PROJECT_ROOT/project.godot" ]] || fail_env "Godot project root not found: $GODOT_PROJECT_ROOT"
  set +e
  "$GODOT_BIN" --headless --path "$GODOT_PROJECT_ROOT" --quit > "$WORKDIR/logs/godot_headless_parse.log" 2>&1
  EXIT_CODE=$?
  set -e
  record_step "Godot headless parse" "$([[ $EXIT_CODE -eq 0 ]] && echo passed || echo failed)" "$EXIT_CODE" "log_path=logs/godot_headless_parse.log"
  [[ "$EXIT_CODE" -eq 0 ]] || { FATAL_MSG="godot headless parse failed with exit code $EXIT_CODE"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
  ok "godot headless parse passed"
fi

# ---------------------------------------------------------------- step 2
if [[ "$SKIP_GO_TESTS" -ne 1 ]]; then
  step "Observation Go regression set"
  set +e
  (cd "$REPO_ROOT/agent" && go test ./internal/acousticpackage ./internal/mom ./internal/tim ./internal/mixboard ./internal/agentloop ./internal/chat -count=1) \
    > "$WORKDIR/logs/go_observation_regression.log" 2>&1
  EXIT_CODE=$?
  set -e
  record_step "Observation Go regression set" "$([[ $EXIT_CODE -eq 0 ]] && echo passed || echo failed)" "$EXIT_CODE" "log_path=logs/go_observation_regression.log"
  [[ "$EXIT_CODE" -eq 0 ]] || { FATAL_MSG="observation go regression failed with exit code $EXIT_CODE"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
  ok "observation go regression passed"
fi

# ---------------------------------------------------------------- kernel helper
ensure_kernel() {
  # Starts (once) the isolated kernel the DAD steps probe. Prints nothing;
  # sets KERNEL_PID / KERNEL_BIN.
  [[ -z "$KERNEL_PID" ]] || return 0
  step "Prepare VitApp kernel (isolated; DAD steps)"
  for port in "$ZMQ_REQ_PORT" "$ZMQ_SUB_PORT" "$ZMQ_LOG_PORT"; do
    pid_busy="$(port_listener_pid "$port")"
    [[ -z "$pid_busy" ]] || fail_env "port $port already has a listener (pid $pid_busy); AGENTS §9 single-owner rule"
  done
  if [[ -n "$KERNEL_BIN_ARG" ]]; then
    [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
    KERNEL_BIN="$KERNEL_BIN_ARG"
    shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  else
    local tracktion="${TRACKTION_DIR:-$REPO_ROOT/tracktion_engine}"
    [[ -f "$tracktion/CMakeLists.txt" ]] || fail_env "tracktion_engine not usable at $tracktion (pass --tracktion-dir or --kernel-bin)"
    log "building VitApp kernel (cmake+make)..."
    if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
          -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR="$tracktion" \
          > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
      tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
      fail_env "kernel cmake configure failed"
    fi
    if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp > "$WORKDIR/logs/kernel_build.log" 2>&1; then
      tail -20 "$WORKDIR/logs/kernel_build.log" >&2
      fail_env "kernel make failed"
    fi
    KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
    shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  fi
  cp "$REPO_DEFAULT_PROJECT" "$KERNEL_WORKSPACE/default_project.xml"
  (
    cd "$KERNEL_ROOT"
    exec env VIT_PROJECT_XML="$KERNEL_WORKSPACE/default_project.xml" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             "$KERNEL_BIN"
  ) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
  KERNEL_PID=$!
  local deadline=$((SECONDS + TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2; fail_env "kernel exited during startup"; }
    [[ -n "$(port_listener_pid "$ZMQ_REQ_PORT")" ]] && break
    sleep 1
  done
  [[ -n "$(port_listener_pid "$ZMQ_REQ_PORT")" ]] || fail_env "kernel command port $ZMQ_REQ_PORT did not become ready"
  ok "kernel started pid=$KERNEL_PID"
}

resolve_material() {
  if [[ -z "$MATERIAL_PATH" ]]; then
    if [[ -f "$REPO_ROOT/Paper Crown.mp3" ]]; then
      MATERIAL_PATH="$REPO_ROOT/Paper Crown.mp3"
    else
      MATERIAL_PATH="$REPO_ROOT/test_100hz_10s.wav"
    fi
  fi
  [[ -f "$MATERIAL_PATH" ]] || fail_env "probe material not found: $MATERIAL_PATH"
}

run_dad_probe() {
  # run_dad_probe <features> <out-dir> — starts kernel if needed, runs
  # dad_probe.py, fails the run when the probe exits nonzero.
  local features="$1" out_dir="$2"
  ensure_kernel
  resolve_material
  mkdir -p "$out_dir"
  log "running dad_probe.py features=[$features] material=$MATERIAL_PATH"
  set +e
  "$PYTHON_BIN" "$REPO_ROOT/scripts/dad_probe.py" \
    --req-url "tcp://127.0.0.1:$ZMQ_REQ_PORT" \
    --sub-url "tcp://127.0.0.1:$ZMQ_SUB_PORT" \
    --material-path "$MATERIAL_PATH" \
    --features "$features" \
    --timeout-sec "$TIMEOUT_SECONDS" \
    --output "$out_dir/dad_probe_summary.json" 2>&1 | tee "$out_dir/dad_probe_stdout.log"
  local probe_exit="${PIPESTATUS[0]}"
  set -e
  if [[ "$probe_exit" -ne 0 ]]; then
    echo "ERROR[functional]: dad_probe.py failed with exit code $probe_exit (log: $out_dir/dad_probe_stdout.log)" >&2
    if grep -q "shared_memory_read_failed" "$out_dir/dad_probe_summary.json" 2>/dev/null; then
      echo "NOTE: mac interface gap — dad_probe.py reads kernel shm via Windows-only mmap(tagname=...); POSIX-shm reader is out of this card's file domain (see script header)" >&2
    fi
    return "$probe_exit"
  fi
  return 0
}

if [[ "$SKIP_KERNEL_SMOKES" -ne 1 && "$SKIP_DAD_SMOKES" -ne 1 ]]; then
  # ------------------------------------------------------------ step 3
  step "DAD L3 package smoke"
  DAD_L3_DIR="$WORKDIR/dad_l3"
  if run_dad_probe "waveform_envelope,spectral_field,l3_acoustic_summary" "$DAD_L3_DIR"; then
    record_step "DAD L3 package smoke" passed 0 "summary_path=$DAD_L3_DIR/dad_probe_summary.json"
  else
    EC=$?
    record_step "DAD L3 package smoke" failed "$EC" "summary_path=$DAD_L3_DIR/dad_probe_summary.json"
    FATAL_MSG="DAD L3 package smoke failed with exit code $EC (mac shm reader gap — see script header)"
    OUTCOME="assertion_failed"
    fail_functional "$FATAL_MSG"
  fi
  python3 - "$DAD_L3_DIR/dad_probe_summary.json" <<'PY' || { FATAL_MSG="DAD L3 summary validation failed"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
rows = [f for f in report.get("features", []) if f.get("feature_type") == "l3_acoustic_summary"]
if not rows:
    sys.exit("l3_acoustic_summary feature report missing")
row = rows[0]
if row.get("status") not in ("ready", "suspect"):
    sys.exit(f"l3_acoustic_summary status is not ready/suspect: {row.get('status')} reason={row.get('reason')}")
observed = set(row.get("observed_features") or [])
per_feature = row.get("per_feature") or {}
for required in ("band_energy_summary", "stereo_relation_summary", "loudness_summary"):
    if required not in observed:
        sys.exit(f"missing L3 feature {required}")
    feature = per_feature.get(required) or {}
    if not str(feature.get("evidence_ref") or ""):
        sys.exit(f"missing evidence_ref for {required}")
band_events = [e.get("event") for e in (row.get("events") or [])
               if (e.get("event") or {}).get("feature_type") == "band_energy_summary"]
if not band_events:
    sys.exit("no band_energy_summary event payload was retained")
band_event = band_events[0]
for bounded in ("noise_floor_evidence", "frequency_time_events", "transient_events", "band_dynamics"):
    if bounded not in band_event:
        sys.exit(f"missing bounded producer fact {bounded}")
sys.exit(0)
PY
  ok "DAD L3 package smoke passed"

fi
if [[ "$SKIP_KERNEL_SMOKES" -ne 1 && "$SKIP_DAD_SMOKES" -ne 1 ]]; then
  # ------------------------------------------------------------ step 4
  step "L2 render probe smoke"
  L2_RP_DIR="$WORKDIR/l2_render_probe"
  if run_dad_probe "waveform_envelope,spectral_field,l2_render_probe" "$L2_RP_DIR"; then
    record_step "L2 render probe smoke" passed 0 "summary_path=$L2_RP_DIR/dad_probe_summary.json"
  else
    EC=$?
    record_step "L2 render probe smoke" failed "$EC" "summary_path=$L2_RP_DIR/dad_probe_summary.json"
    FATAL_MSG="L2 render probe smoke failed with exit code $EC (mac shm reader gap — see script header)"
    OUTCOME="assertion_failed"
    fail_functional "$FATAL_MSG"
  fi
  python3 - "$L2_RP_DIR/dad_probe_summary.json" <<'PY' || { FATAL_MSG="L2 render probe summary validation failed"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
rows = [f for f in report.get("features", []) if f.get("feature_type") == "l2_render_probe"]
if not rows:
    sys.exit("l2_render_probe feature report missing")
row = rows[0]
if row.get("status") not in ("ready", "suspect"):
    sys.exit(f"l2_render_probe status is not ready/suspect: {row.get('status')} reason={row.get('reason')}")
if row.get("raw_leak_keys"):
    sys.exit(f"l2_render_probe raw payload leaked: {row.get('raw_leak_keys')}")
sys.exit(0)
PY
  ok "L2 render probe smoke passed"
fi
if [[ "$SKIP_KERNEL_SMOKES" -ne 1 ]]; then
  # ------------------------------------------------------------ step 5
  step "L2 realtime observation smoke"
  L2_RT_DIR="$WORKDIR/l2_realtime"
  mkdir -p "$L2_RT_DIR"
  resolve_material
  python3 - "$L2_RT_DIR/seed_snapshot.json" "$MATERIAL_PATH" <<'PY'
import json, sys
out, material = sys.argv[1:3]
seed = {
    "schema_version": "mixboard_feature_snapshot.v1",
    "updated_at": "2026-06-23T00:00:00Z",
    "latest_request": {
        "schema_version": "mixboard_feature_request.v1",
        "request_id": "l2_probe_req",
        "resolved_target": {
            "track_id": "track_1", "clip_id": "clip_1",
            "source_path": material, "file_path": material,
            "source_revision": "rev_prepared", "clip_revision": "clip_rev_prepared",
            "duration_seconds": 10.0,
        },
        "requested_features": [
            {"feature_type": "waveform_envelope", "request_id": "l2_probe_req", "track_id": "track_1", "clip_id": "clip_1"},
            {"feature_type": "spectral_field", "request_id": "l2_probe_req", "track_id": "track_1", "clip_id": "clip_1"},
        ],
        "updated_at": "2026-06-23T00:00:00Z",
    },
    "waveform_envelope": {
        "status": "ready", "track_id": "track_1", "clip_id": "clip_1",
        "request_id": "l2_probe_req", "source": "kernel_audio_feature_data_ready",
        "source_revision": "rev_prepared", "clip_revision": "clip_rev_prepared",
        "rms": 0.2, "peak_abs": 0.7, "float_count": 128,
    },
    "spectrogram_tiles": {
        "status": "ready", "feature_type": "spectral_field", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "kernel_prepared_telemetry", "source_revision": "rev_prepared",
        "clip_revision": "clip_rev_prepared",
        "tile_count_seen": 2, "tile_count_expected": 2, "tile_count_parsed": 2,
        "coverage_seconds": 10.0,
    },
    "spectrogram_tile_rows": [{
        "status": "ready", "feature_type": "spectral_field", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "kernel_prepared_telemetry", "source_revision": "rev_prepared",
        "tile_count_seen": 2, "tile_count_expected": 2, "coverage_seconds": 10.0,
    }],
    "band_energy_summary": {
        "status": "ready", "feature_type": "band_energy_summary", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "spectral_tile_derived", "materialized_by": "kernel_prepared_telemetry",
        "source_revision": "rev_prepared", "clip_revision": "clip_rev_prepared",
        "tile_count_parsed": 2, "coverage_seconds": 10.0,
        "bands": {"bass": {"unit_energy": 0.25, "energy_db": -12.041, "min_hz": 60, "max_hz": 160},
                  "air": {"unit_energy": 0.02, "energy_db": -33.979, "min_hz": 6000, "max_hz": 16000}},
    },
    "band_energy_summaries": [{
        "status": "ready", "feature_type": "band_energy_summary", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "spectral_tile_derived", "materialized_by": "kernel_prepared_telemetry",
        "source_revision": "rev_prepared", "tile_count_parsed": 2, "coverage_seconds": 10.0,
        "bands": {"bass": {"unit_energy": 0.25, "energy_db": -12.041}},
    }],
    "stereo_relation_summary": {
        "status": "ready", "feature_type": "stereo_relation_summary", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "spectral_tile_derived", "materialized_by": "kernel_prepared_telemetry",
        "source_revision": "rev_prepared", "clip_revision": "clip_rev_prepared",
        "tile_count_parsed": 2, "coverage_seconds": 10.0,
        "balance_db": 0.1, "balance_state": "centered",
        "correlation_estimate": 0.82, "correlation_state": "stable",
    },
    "stereo_relation_summaries": [{
        "status": "ready", "feature_type": "stereo_relation_summary", "track_id": "track_1",
        "clip_id": "clip_1", "request_id": "l2_probe_req",
        "source": "spectral_tile_derived", "materialized_by": "kernel_prepared_telemetry",
        "source_revision": "rev_prepared", "tile_count_parsed": 2, "coverage_seconds": 10.0,
        "balance_state": "centered", "correlation_estimate": 0.82,
    }],
}
json.dump(seed, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
  DEV_ROOT="$L2_RT_DIR/dev_root"
  SNAPSHOT_DIR="$DEV_ROOT/VitApp/Workspace/Artifacts"
  mkdir -p "$SNAPSHOT_DIR"
  cp "$L2_RT_DIR/seed_snapshot.json" "$SNAPSHOT_DIR/mixboard_feature_snapshot.json"
  READY_SNAPSHOT="$L2_RT_DIR/l2_ready_snapshot.json"
  set +e
  env VIT_DAW_DEV_ROOT="$DEV_ROOT" \
      VIT_DAW_L2_PROBE_SOURCE="$MATERIAL_PATH" \
      VIT_DAW_L2_READY_SNAPSHOT_PATH="$READY_SNAPSHOT" \
      "$GODOT_BIN" --headless --path "$GODOT_PROJECT_ROOT" \
      --script "$REPO_ROOT/scripts/l2_realtime_observation_probe.gd" \
      > "$L2_RT_DIR/godot_l2_probe.log" 2>&1
  GODOT_EXIT=$?
  set -e
  record_step "L2 realtime observation smoke" "$([[ $GODOT_EXIT -eq 0 ]] && echo passed || echo failed)" "$GODOT_EXIT" "artifact_path=$L2_RT_DIR"
  [[ "$GODOT_EXIT" -eq 0 ]] || { FATAL_MSG="Godot L2 probe exited with code $GODOT_EXIT (log: $L2_RT_DIR/godot_l2_probe.log)"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }

  python3 - "$SNAPSHOT_DIR/mixboard_feature_snapshot.json" "$READY_SNAPSHOT" <<'PY' || { FATAL_MSG="L2 realtime snapshot validation failed"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
import json, sys
snapshot = json.load(open(sys.argv[1], encoding="utf-8"))
ready = json.load(open(sys.argv[2], encoding="utf-8"))
def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr); sys.exit(1)
def num(v):
    try:
        return float(v)
    except Exception:
        return None
ready_band = (ready.get("band_energy_summary") or {})
ready_stereo = (ready.get("stereo_relation_summary") or {})
if str(ready_band.get("source")) != "spectral_tile_derived":
    fail("L3 band summary was overwritten during ready phase")
if str(ready_stereo.get("source")) != "spectral_tile_derived":
    fail("L3 stereo summary was overwritten during ready phase")
if str(ready_band.get("source_revision")) != "rev_prepared":
    fail("L3 band source_revision was not preserved")
live_band_rows = ready.get("realtime_band_energy_summaries") or []
live_stereo_rows = ready.get("realtime_stereo_relation_summaries") or []
def first_ready(rows):
    for row in rows:
        if str(row.get("request_id")) == "l2_probe_req" and str(row.get("status")) == "ready":
            return row
    return None
band_row = first_ready(live_band_rows)
stereo_row = first_ready(live_stereo_rows)
if band_row is None or str(band_row.get("source")) != "live_level_meter_spectrum":
    fail("L2 realtime ready band summary missing from matrix rows")
if stereo_row is None or str(stereo_row.get("source")) != "live_level_meter_stereo":
    fail("L2 realtime ready stereo summary missing from matrix rows")
for row in (band_row, stereo_row):
    if str(row.get("capture_mode")) != "realtime_playback":
        fail("L2 realtime row missing capture_mode=realtime_playback")
    if str(row.get("tap_point")) != "unknown_live_meter":
        fail("L2 realtime row must not claim post_fx/post_fader tap point")
    qe = row.get("quality_evidence") or {}
    if qe.get("post_fx_verified") is not False or qe.get("post_fader_verified") is not False:
        fail("L2 realtime row missing negative post-FX/post-fader evidence")
live_bass = num(((band_row.get("bands") or {}).get("bass") or {}).get("unit_energy"))
live_air = num(((band_row.get("bands") or {}).get("air") or {}).get("unit_energy"))
if live_bass is None or live_air is None or live_bass <= live_air:
    fail(f"L2 band values do not show the injected bass energy. bass={live_bass} air={live_air}")
live_band_top = snapshot.get("realtime_band_energy_summary") or {}
live_stereo_top = snapshot.get("realtime_stereo_relation_summary") or {}
if str(live_band_top.get("status")) == "ready" or str(live_stereo_top.get("status")) == "ready":
    fail("L2 realtime top-level rows stayed ready after playback stopped/source changed")
if str((snapshot.get("band_energy_summary") or {}).get("status")) == "ready" \
        or str((snapshot.get("stereo_relation_summary") or {}).get("status")) == "ready":
    fail("L3 top-level rows stayed ready after source changed without new prepared capture")
for row in (live_band_top, live_stereo_top):
    if str(row.get("status")) != "deferred":
        fail("L2 realtime top-level row should be deferred after stopped playback")
    if str(row.get("source_revision")) != "rev_second_material":
        fail("L2 realtime top-level row did not follow new source identity")
    if str(row.get("tap_point")) != "unknown_live_meter":
        fail("deferred L2 row missing explicit tap_point")
for name in ("spectrogram_tile_rows", "band_energy_summaries", "stereo_relation_summaries",
             "realtime_band_energy_summaries", "realtime_stereo_relation_summaries"):
    rows = snapshot.get(name)
    if not rows or len(rows) < 1:
        fail(f"{name} was not preserved/written")
sys.exit(0)
PY
  ok "L2 realtime observation smoke passed"
fi

# ---------------------------------------------------------------- step 6
step "AB result smoke"
set +e
bash "$SCRIPT_DIR/run_ab_result_smoke_mac.sh" --repo-root "$REPO_ROOT" \
  --artifact-root "$ARTIFACT_ROOT" > "$WORKDIR/logs/ab_result_smoke.log" 2>&1
AB_EXIT=$?
set -e
record_step "AB result smoke" "$([[ $AB_EXIT -eq 0 ]] && echo passed || echo failed)" "$AB_EXIT" "log_path=logs/ab_result_smoke.log"
[[ "$AB_EXIT" -eq 0 ]] || { FATAL_MSG="AB result smoke failed with exit code $AB_EXIT"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
ok "AB result smoke passed"

# ---------------------------------------------------------------- step 7
if [[ "$SKIP_PRODUCT_PATH" -ne 1 ]]; then
  step "Product-path smoke (mac two-piece adaptation of the ps1 lifecycle step)"
  set +e
  bash "$SCRIPT_DIR/run_vit_product_path_smoke_mac.sh" --repo-root "$REPO_ROOT" \
    --artifact-root "$ARTIFACT_ROOT" \
    ${KERNEL_BIN_ARG:+--kernel-bin "$KERNEL_BIN_ARG"} \
    > "$WORKDIR/logs/product_path_smoke.log" 2>&1
  PP_EXIT=$?
  set -e
  record_step "Product-path smoke" "$([[ $PP_EXIT -eq 0 ]] && echo passed || echo failed)" "$PP_EXIT" "log_path=logs/product_path_smoke.log"
  [[ "$PP_EXIT" -eq 0 ]] || { FATAL_MSG="product-path smoke failed with exit code $PP_EXIT"; OUTCOME="assertion_failed"; fail_functional "$FATAL_MSG"; }
  ok "product-path smoke passed"
fi

OUTCOME="all_green"
ok "Observation v1 acceptance smoke passed; summary=$SUMMARY_PATH"
exit 0
