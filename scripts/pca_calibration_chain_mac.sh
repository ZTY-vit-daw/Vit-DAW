#!/usr/bin/env bash
# pca_calibration_chain_mac.sh — PORT-C2 one-key mac calibration chain.
#
# Mac equivalent of the PC-side calibration chain (the stack-driving pattern of
# scripts/b1_2_source_calibration_agent_smoke.py / b1_2_strict_reference_calibration_agent_smoke.py,
# reusing the PORT-A5 stack harness scripts/dev_agent_smoke_mac.sh):
#
#   1. stack up      real VitApp kernel + Go agent (kernel+agent two processes;
#                    Godot frontend out of scope), agent HTTP /health ready,
#                    project.state agent->kernel ZMQ round-trip
#   2. probe         one agent invoke plugin.semantic_build_index over the VST3
#                    dir (kernel child-process scan, A5-proven on this stack)
#                    -> machine-local semantic index + full inventory
#   3. whitelist     select the U2-narrowed Waves promoted subjects from the
#      draft         inventory (12 families x plain Mono/Stereo variants), emit
#                    whitelist_draft.json aligned to the PC-side whitelist
#                    schemas (plugin_semantics Entry + processor certification
#                    candidate) with probe conclusions and a comparison note
#   4. PCA           per subject: POST /agent/processor-certification/start
#      re-certify    (consent token) -> poll job -> receipts land under
#                    ~/.vit/pca_certifications/, attestations import into the
#                    machine-local PCA stores (~/.vit/processor_control_
#                    attestations.v{1,2}.json) with promoted status
#   5. readonly      C4/A5 whitelist GETs + GET /agent/processor-certification/
#      smoke         candidates?family=all asserting every subject promoted for
#                    its manifest family, plus a read-only parse of the PCA
#                    stores. No further mutations.
#
# U2 narrowing (2026-09-16 user ruling): mac is the demo machine, Waves-only;
# the 5 Plugin Alliance and 3 FabFilter subjects are NOT installed. The 12
# plain Waves families enumerate to 24 Mono/Stereo bodies on this machine
# (U2 records "23 subjects"; see whitelist_draft.json -> reconciliation_note —
# the superset covers any PC-side 23-subset, PA/FabFilter excluded by U2).
#
# The kernel writes all of its state under a fake VitApp root inside the run
# workdir (VitPaths.h climbs to a directory containing CMakeLists.txt +
# Source/), so the source tree is never touched (AGENTS §10). Machine-local
# calibration state (~/.vit: semantic index, PCA stores, certification
# receipts) is the designed landing location for this chain (card acceptance
# ③); pre/post hashes are recorded in the artifacts. The pluginprobe native
# observation host is NOT used: PluginProbe/native-host is Windows-only today
# (module_win32.cpp + bcrypt), and the certification runner itself performs the
# real load + typed inspect/apply/restore on a disposable track, which is the
# load evidence this chain needs.
#
# Usage:
#   ./scripts/pca_calibration_chain_mac.sh                    # full run
#   ./scripts/pca_calibration_chain_mac.sh --kernel-bin PATH  # reuse a kernel
#   ./scripts/pca_calibration_chain_mac.sh --skip-agent-build --agent-bin PATH
#   ./scripts/pca_calibration_chain_mac.sh --workdir PATH     # reuse a workdir
#
# Exit code 0 requires every gate to pass (see summary.json "gates"). Failure
# classification (AGENTS §8): env (tool/ports/build) vs functional_subject_load
# (a promoted subject failed to instantiate/load — R2 stop condition, evidence
# preserved for blocked) vs functional (enumeration/draft/certification/readonly
# assertion failures). All artifacts land under the run workdir, printed to
# stderr.
#
# PORT-C2-PCR platform branches (2026-09-18, single script, both ends): the
# chain also runs on PC Git Bash (MINGW64). Platform differences are isolated
# to branches guarded by uname (never shared-path rewrites): tool set
# (sha256sum/netstat vs shasum/lsof), mixed-form paths (D:/...) for every
# argument handed to a native binary, the MSVC multi-config kernel build
# (default Visual Studio generator + cmake --build), the ZMQ port-owner
# assertion via the MSYS winpid mapping, and the Windows selection semantics:
# several WaveShell generations coexist in the PC VST3 dir, so same-name
# inventory entries are deduplicated (newest shell wins) and a family with a
# single Mono-or-Stereo variant is a recorded machine fact instead of a
# selection failure (darwin keeps the strict Mono+Stereo pair requirement).

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
VST3_DIR=""
EXPECTED_SUBJECTS=23
SCAN_TIMEOUT_SECONDS=900
CERTIFY_TIMEOUT_SECONDS=420
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
WORKDIR_ARG=""
OUTPUT_PATH=""
TRACKTION_DIR=""
CMAKE_PROXY=""
FETCH_SRC_SPECS=""

ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555
CONVERSATION_ID="pca-calibration-chain-mac-probe"

ALLOWED_GET_PATHS=(
  "/health"
  "/agent/runtime/status"
  "/agent/state"
  "/agent/events"
)

usage() {
  cat <<'EOF'
pca_calibration_chain_mac.sh — PORT-C2 mac calibration chain (probe -> whitelist
draft -> PCA re-certification -> readonly smoke).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-agent-build      With --agent-bin: do not build the agent
  --agent-bin PATH        Agent binary to start (only with --skip-agent-build)
  --vst3-dir PATH         VST3 search path for the probe scan
                          (platform default: /Library/Audio/Plug-Ins/VST3 on
                          darwin, C:/Program Files/Common Files/VST3 on
                          Windows/Git Bash)
  --expected-subjects N   Minimum matched Waves subjects for the probe gate
                          (default 23, U2 narrowed scope)
  --scan-timeout SECONDS  Probe scan + semantic index build timeout (default 900)
  --certify-timeout SECONDS  Per-subject certification job timeout (default 420)
  --startup-timeout SECONDS  Kernel/agent startup timeout (default 90)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --workdir PATH          Reuse PATH as the run workdir instead of a fresh
                          mktemp directory (never overwrites other runs)
  --tracktion-dir PATH    tracktion_engine source dir (default: the repo's
                          submodule checkout; in an isolated git worktree the
                          submodule may be empty — point at any checkout of
                          the same pinned commit)
  --cmake-proxy URL       HTTP(S) proxy for the kernel cmake FetchContent
                          downloads only (e.g. http://127.0.0.1:7890 when
                          github tarballs are unreachable directly)
  --fetch-src NAME=PATH   Use PATH as the FetchContent source dir for NAME
                          (e.g. --fetch-src LIBZMQ=D:/cache/libzmq-src);
                          repeatable; offline override — no download is
                          attempted for that dependency (same pinned version
                          is the caller's responsibility)
  --output PATH           Also write the summary JSON to PATH
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --vst3-dir) VST3_DIR="$2"; shift 2 ;;
    --expected-subjects) EXPECTED_SUBJECTS="$2"; shift 2 ;;
    --scan-timeout) SCAN_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --certify-timeout) CERTIFY_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    --cmake-proxy) CMAKE_PROXY="$2"; shift 2 ;;
    --fetch-src) FETCH_SRC_SPECS+="${2}"$'\n'; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

fail_env() {
  echo "ERROR[env]: $*" >&2
  exit 3
}
fail_functional() {
  echo "ERROR[functional]: $*" >&2
  exit 1
}

# --------------------------------------------------------- platform branches
PLATFORM="$(uname -s)"
case "$PLATFORM" in
  Darwin)
    PLATFORM_TAG="mac"
    PY="python3"
    SHASUM=(shasum -a 256)
    ;;
  MINGW*|MSYS*|CYGWIN*)
    PLATFORM_TAG="win"
    # Real interpreter only: the WindowsApps python3 alias is an unreliable
    # stub on this class of machines (intermittent rc=49, no output).
    PY="python"
    SHASUM=(sha256sum)
    # Native binaries (cmake/go/python/VitApp/VitAgent) cannot resolve MSYS
    # /d/-style paths; every path crossing the bash/native boundary stays in
    # mixed form (D:/...) via cygpath -m.
    REPO_ROOT="$(cygpath -m "$REPO_ROOT")"
    ;;
  *)
    fail_env "unsupported platform: $PLATFORM (supported: Darwin, MINGW/MSYS Git Bash)"
    ;;
esac
[[ -n "$VST3_DIR" ]] || VST3_DIR="$(
  if [[ "$PLATFORM_TAG" == "mac" ]]; then
    echo "/Library/Audio/Plug-Ins/VST3"
  else
    echo "C:/Program Files/Common Files/VST3"
  fi
)"

if [[ "$PLATFORM_TAG" == "mac" ]]; then
  for tool in go cmake make python3 lsof pgrep shasum curl; do
    command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
  done
else
  for tool in go cmake python curl sha256sum netstat cygpath; do
    command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
  done
fi

RUN_ID="pca_calibration_chain_${PLATFORM_TAG}_$(date '+%Y%m%d-%H%M%S')"
if [[ "$PLATFORM_TAG" == "mac" ]]; then
  WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/pca_calibration_chain_mac.XXXXXXXX")}"
else
  # MSYS /tmp is invisible to native binaries; default into the Windows temp.
  WORKDIR="${WORKDIR_ARG:-$(mktemp -d "$(cygpath -m "${TEMP:-/tmp}")/pca_calibration_chain_win.XXXXXXXX")}"
fi
mkdir -p "$WORKDIR"/{build,bin,http,logs,probe,cert}
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT"
# Fake VitApp root markers: VitPaths.h climbToVitAppRoot() anchors the kernel's
# Workspace/{Logs,Settings,Cache} + default_project.xml here (AGENTS §10).
: > "$KERNEL_ROOT/CMakeLists.txt"
mkdir -p "$KERNEL_ROOT/Source"
AGENT_STATE_DIR="$WORKDIR/agent_state"
mkdir -p "$AGENT_STATE_DIR" "$WORKDIR/agent_cwd"

KERNEL_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
FAIL_CLASSIFICATION=""

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

cleanup() {
  local rc=$?
  # Never leave processes this run started; evidence is already on disk.
  for pid_sig in "$AGENT_PID:TERM" "$KERNEL_PID:TERM"; do
    local pid="${pid_sig%%:*}" sig="${pid_sig##*:}"
    [[ -n "$pid" ]] || continue
    if kill -0 "$pid" 2>/dev/null; then
      kill -"$sig" "$pid" 2>/dev/null || true
      local i
      for i in 1 2 3 4 5 6 7 8 9 10; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 1
      done
      kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
  exit $rc
}
trap cleanup EXIT INT TERM

json_field() {
  # json_field <file> <python expr against d> — small JSON extraction helper.
  "$PY" - "$1" "$2" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
print(eval(sys.argv[2], {"d": d}))
PY
}

if [[ "$PLATFORM_TAG" == "mac" ]]; then
  port_listener_pid() {
    # lsof exits 1 when no listener matches; with pipefail that must not kill
    # the harness (empty output == "no listener" is the expected free-port case).
    lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
  }
else
  port_listener_pid() {
    # Windows PID (for the owner assertion via the MSYS winpid mapping); the
    # LISTENING state keyword is not localized on any Windows locale.
    netstat -ano | awk -v p="$1" '$1=="TCP" && $4=="LISTENING" { n=split($2,a,":"); if (a[n]==p) { print $5; exit } }'
  }
fi

assert_ports_free() {
  local port
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "$AGENT_HTTP_PORT"; do
    local pid
    pid="$(port_listener_pid "$port")"
    if [[ -n "$pid" ]]; then
      fail_env "port $port already has a listener (pid $pid); this harness never stops processes it did not start (AGENTS §9 single-owner rule)"
    fi
  done
}

AGENT_HTTP_ADDR="${AGENT_HTTP#http://}"
AGENT_HTTP_ADDR="${AGENT_HTTP_ADDR#https://}"
AGENT_HTTP_PORT="${AGENT_HTTP_ADDR##*:}"

assert_ports_free

[[ -d "$VST3_DIR" ]] || fail_env "VST3 scan directory does not exist: $VST3_DIR"

# Machine-local calibration state landing dirs (card acceptance ③): record the
# pre-run state so the receipt can show exactly what this run created.
VIT_HOME="$HOME/.vit"
if [[ "$PLATFORM_TAG" != "mac" ]]; then
  VIT_HOME="$(cygpath -m "$VIT_HOME")"
fi
snapshot_vit_state() {
  local out="$1"
  {
    echo "snapshot_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
    echo "platform=$PLATFORM_TAG"
    if [[ -d "$VIT_HOME" ]]; then
      (cd "$VIT_HOME" && find . -maxdepth 5 -type f | sort | while read -r f; do
        printf '%s  %s\n' "$("${SHASUM[@]}" "$f" | cut -d' ' -f1)" "$f"
      done)
    else
      echo "absent: $VIT_HOME did not exist before this run"
    fi
  } > "$out" 2>/dev/null || echo "snapshot failed" > "$out"
}
snapshot_vit_state "$WORKDIR/vit_state_before.txt"

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "vst3_dir=$VST3_DIR"
  echo "expected_subjects=$EXPECTED_SUBJECTS"
} > "$WORKDIR/run_meta.txt"

# ---------------------------------------------------------------- kernel build
KERNEL_BIN=""
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  "${SHASUM[@]}" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  if [[ "$PLATFORM_TAG" == "mac" ]]; then
    stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  else
    stat -c "kernel_bin_mtime=%y kernel_bin_size=%s" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  fi
  log "using provided kernel binary: $KERNEL_BIN (sha256/mtime recorded)"
else
  if [[ -z "$TRACKTION_DIR" ]]; then
    TRACKTION_DIR="$REPO_ROOT/tracktion_engine"
  fi
  if [[ "$PLATFORM_TAG" != "mac" ]]; then
    TRACKTION_DIR="$(cygpath -m "$TRACKTION_DIR")"
  fi
  if [[ ! -f "$TRACKTION_DIR/CMakeLists.txt" ]]; then
    fail_env "tracktion_engine not usable at $TRACKTION_DIR (submodule not checked out? pass --tracktion-dir pointing at a checkout of the pinned commit)"
  fi
  CMAKE_ENV=()
  if [[ -n "$CMAKE_PROXY" ]]; then
    # Scoped to the kernel build only: FetchContent tarball downloads honor
    # the libcurl proxy env vars; the rest of the chain stays proxy-free.
    CMAKE_ENV=("HTTPS_PROXY=$CMAKE_PROXY" "HTTP_PROXY=$CMAKE_PROXY" "https_proxy=$CMAKE_PROXY" "http_proxy=$CMAKE_PROXY")
    log "kernel build downloads routed through $CMAKE_PROXY"
  fi
  if [[ "$PLATFORM_TAG" == "mac" ]]; then
    CMAKE_CONFIGURE_ARGS=(-G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug)
  else
    # MSVC default generator (multi-config, auto-selects the installed Visual
    # Studio); CMAKE_BUILD_TYPE is meaningless there — config picked at build.
    CMAKE_CONFIGURE_ARGS=()
  fi
  FETCH_CONTENT_DEFINES=()
  while IFS= read -r spec; do
    [[ -n "$spec" ]] || continue
    name="${spec%%=*}"
    path="${spec#*=}"
    [[ "$name" != "$spec" && -n "$name" && -n "$path" ]] \
      || fail_env "--fetch-src expects NAME=PATH, got: $spec"
    [[ -d "$path" ]] || fail_env "--fetch-src source dir does not exist: $path"
    if [[ "$PLATFORM_TAG" != "mac" ]]; then
      path="$(cygpath -m "$path")"
    fi
    FETCH_CONTENT_DEFINES+=("-DFETCHCONTENT_SOURCE_DIR_${name}=${path}")
    log "fetch-src override: FETCHCONTENT_SOURCE_DIR_${name} -> $path"
  done <<< "$FETCH_SRC_SPECS"
  log "building VitApp kernel (cmake, sources at $REPO_ROOT/VitApp, tracktion at $TRACKTION_DIR)..."
  if ! env ${CMAKE_ENV[@]+"${CMAKE_ENV[@]}"} cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" \
        ${CMAKE_CONFIGURE_ARGS[@]+"${CMAKE_CONFIGURE_ARGS[@]}"} \
        ${FETCH_CONTENT_DEFINES[@]+"${FETCH_CONTENT_DEFINES[@]}"} \
        -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fail_env "kernel cmake configure failed (see $WORKDIR/logs/kernel_configure.log)"
  fi
  if [[ "$PLATFORM_TAG" == "mac" ]]; then
    if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp \
          > "$WORKDIR/logs/kernel_build.log" 2>&1; then
      tail -20 "$WORKDIR/logs/kernel_build.log" >&2
      fail_env "kernel make failed (see $WORKDIR/logs/kernel_build.log)"
    fi
    KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  else
    if ! cmake --build "$WORKDIR/build/vitapp" --config Debug --target VitApp -j "$(nproc)" \
          > "$WORKDIR/logs/kernel_build.log" 2>&1; then
      tail -20 "$WORKDIR/logs/kernel_build.log" >&2
      fail_env "kernel cmake --build failed (see $WORKDIR/logs/kernel_build.log)"
    fi
    KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp.exe"
  fi
  [[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary not found after build: $KERNEL_BIN"
  "${SHASUM[@]}" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "kernel built: $KERNEL_BIN"
fi

# ---------------------------------------------------------------- agent build
if [[ "$SKIP_AGENT_BUILD" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" ]] || fail_env "--skip-agent-build requires --agent-bin"
  [[ -x "$AGENT_BIN_ARG" ]] || fail_env "agent binary is not executable: $AGENT_BIN_ARG"
  AGENT_BIN="$AGENT_BIN_ARG"
else
  [[ -z "$AGENT_BIN_ARG" ]] || fail_env "--agent-bin is only valid together with --skip-agent-build"
  AGENT_BIN="$WORKDIR/bin/vitagent"
  [[ "$PLATFORM_TAG" != "mac" ]] && AGENT_BIN="$WORKDIR/bin/vitagent.exe"
  log "building agent (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$AGENT_BIN" ./cmd/vitagent) \
    || fail_env "go build agent failed"
fi
"${SHASUM[@]}" "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256" || true

# ---------------------------------------------------------------- start kernel
log "starting kernel (cwd=$KERNEL_ROOT)..."
(
  cd "$KERNEL_ROOT"
  exec "$KERNEL_BIN"
) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
KERNEL_PID=$!
log "kernel pid=$KERNEL_PID"

kernel_dead() { ! kill -0 "$KERNEL_PID" 2>/dev/null; }

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  if kernel_dead; then
    tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2
    fail_functional "kernel process exited during startup (see $WORKDIR/logs/kernel_stdout.log)"
  fi
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
  sleep 1
done
[[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] || fail_functional "kernel ZMQ REQ port $KERNEL_PORT_REQ not listening within ${STARTUP_TIMEOUT_SECONDS}s"
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  listener_pid="$(port_listener_pid "$port")"
  if [[ "$PLATFORM_TAG" == "mac" ]]; then
    [[ "$listener_pid" == "$KERNEL_PID" ]] || fail_functional "port $port listener pid $listener_pid != kernel pid $KERNEL_PID"
  else
    # $! is an MSYS pid while netstat reports Windows pids; compare through
    # the /proc winpid mapping of the kernel process.
    KERNEL_WINPID="$(cat "/proc/$KERNEL_PID/winpid" 2>/dev/null || true)"
    [[ -n "$KERNEL_WINPID" ]] || fail_env "cannot resolve winpid for kernel pid $KERNEL_PID (/proc/$KERNEL_PID/winpid missing)"
    [[ "$listener_pid" == "$KERNEL_WINPID" ]] || fail_functional "port $port listener pid $listener_pid != kernel winpid $KERNEL_WINPID (msys pid $KERNEL_PID)"
  fi
done
log "kernel ZMQ ports listening (REQ $KERNEL_PORT_REQ / PUB $ZMQ_PUB_PORT / log $ZMQ_LOG_PORT), owner pid $KERNEL_PID"

# ---------------------------------------------------------------- start agent
log "starting agent (state isolated under $AGENT_STATE_DIR)..."
(
  cd "$WORKDIR/agent_cwd"
  VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
  exec "$AGENT_BIN" \
    -http "$AGENT_HTTP_ADDR" \
    -last-log-path "$AGENT_STATE_DIR/agent_last.log" \
    -vsp-hub-url ""
) > "$WORKDIR/logs/agent_stdout.log" 2>&1 &
AGENT_PID=$!
log "agent pid=$AGENT_PID"

agent_dead() { ! kill -0 "$AGENT_PID" 2>/dev/null; }

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  if agent_dead; then
    tail -30 "$WORKDIR/logs/agent_stdout.log" >&2
    fail_functional "agent process exited during startup (see $WORKDIR/logs/agent_stdout.log)"
  fi
  if curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null; then
    break
  fi
  sleep 1
done
curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null \
  || fail_functional "agent /health not ready within ${STARTUP_TIMEOUT_SECONDS}s"
log "agent /health ready"

# ------------------------------------------------- agent->kernel round-trip
log "agent->kernel round-trip (POST /agent/invoke project.state)..."
INVOKE_BODY='{"tool":"project.state","args":{},"source":"pca_calibration_chain_mac"}'
HTTP_CODE="$(curl -sS --max-time 120 -o "$WORKDIR/http/invoke_project_state.json" \
  -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data "$INVOKE_BODY" \
  "$AGENT_HTTP/agent/invoke")" \
  || fail_functional "POST /agent/invoke project.state request failed"
[[ "$HTTP_CODE" =~ ^2 ]] || fail_functional "POST /agent/invoke project.state returned HTTP $HTTP_CODE"
[[ "$(json_field "$WORKDIR/http/invoke_project_state.json" 'str(d.get("status",""))')" == "ok" ]] \
  || fail_functional "project.state via agent invoke not ok: $(head -c 400 "$WORKDIR/http/invoke_project_state.json")"
log "project.state via agent invoke -> ok"

# ---------------------------------------------------------------- probe (scan)
# One agent-surface invoke does the whole probe: kernel child-process scan of
# the VST3 dir (A5-proven) + inventory rows -> Go-side Classify -> machine-local
# semantic index saved to ~/.vit/plugin_semantics.json (the certification entry
# resolves exact identifiers against this index).
log "probe: building semantic index (plugin.semantic_build_index paths=[$VST3_DIR], kernel scan may take minutes)..."
INDEX_BODY="$(printf '{"tool":"plugin.semantic_build_index","args":{"paths":["%s"]},"confirmed":true,"source":"pca_calibration_chain_mac"}' "$VST3_DIR")"
HTTP_CODE="$(curl -sS --max-time "$((SCAN_TIMEOUT_SECONDS + 120))" -o "$WORKDIR/http/semantic_build_index.json" \
  -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data "$INDEX_BODY" \
  "$AGENT_HTTP/agent/invoke")" \
  || fail_functional "plugin.semantic_build_index request failed (scan timeout? see $WORKDIR/logs/kernel_stdout.log)"
[[ "$HTTP_CODE" =~ ^2 ]] || fail_functional "plugin.semantic_build_index returned HTTP $HTTP_CODE: $(head -c 400 "$WORKDIR/http/semantic_build_index.json")"
[[ "$(json_field "$WORKDIR/http/semantic_build_index.json" 'str(d.get("status",""))')" == "ok" ]] \
  || fail_functional "plugin.semantic_build_index not ok: $(head -c 400 "$WORKDIR/http/semantic_build_index.json")"
PLUGIN_COUNT="$(json_field "$WORKDIR/http/semantic_build_index.json" 'int(d.get("result",{}).get("plugin_count",0))')"
INDEX_PATH="$(json_field "$WORKDIR/http/semantic_build_index.json" 'str(d.get("result",{}).get("index_path",""))')"
if [[ "$PLATFORM_TAG" != "mac" ]]; then
  # The native agent reports a Windows path; normalize for the bash-side
  # existence check and copy below.
  INDEX_PATH="$(cygpath -u "$INDEX_PATH" 2>/dev/null || echo "$INDEX_PATH")"
fi
log "semantic index built: plugin_count=$PLUGIN_COUNT index_path=$INDEX_PATH"
[[ -f "$INDEX_PATH" ]] || fail_functional "semantic index file missing at $INDEX_PATH"
cp "$INDEX_PATH" "$WORKDIR/probe/plugin_semantics_index.json"
if (( PLUGIN_COUNT < EXPECTED_SUBJECTS )); then
  fail_functional "probe enumerated only $PLUGIN_COUNT plugins (expected >= $EXPECTED_SUBJECTS waves subjects)"
fi

# ------------------------------------------------------- subject selection
# U2 narrowed scope: 12 plain Waves families x Mono/Stereo variants. Selection
# is by exact kernel inventory name (A5 enumeration evidence); each family must
# match exactly one Mono + one Stereo body. Excluded product variants
# (C1 comp-gate / comp-sc, C6-SideChain, Sibilance-Live, TransX Multi) are not
# promoted subjects; the U2 "23" vs the 24 enumerated bodies reconciliation is
# recorded in the draft (superset covers any PC-side 23-subset).
log "selecting promoted subjects from the semantic index (U2 narrowed scope)..."
"$PY" - "$WORKDIR/probe/plugin_semantics_index.json" "$WORKDIR/probe/selected_subjects.json" <<'PY'
import json, re, sys

index_path, out_path = sys.argv[1], sys.argv[2]
index = json.load(open(index_path, encoding="utf-8"))
IS_WIN = sys.platform.startswith("win")

def shell_rank(entry):
    # Windows: several WaveShell generations coexist in the VST3 dir; prefer
    # the newest (WaveShell1-VST3 17.1 > 16.7 > ...). Non-shell paths rank lowest.
    m = re.search(r"WaveShell\d?-VST3\S*\s(\d+(?:\.\d+)?)", str(entry.get("plugin_path", "")))
    return float(m.group(1)) if m else -1.0

# family key -> (exact-name regex, processor family for PCA certification)
MANIFEST = [
    ("C1",         r"^C1 comp (Mono|Stereo)$",        "broadband_compressor"),
    ("C4",         r"^C4 (Mono|Stereo)$",             "multiband_dynamics"),
    ("C6",         r"^C6 (Mono|Stereo)$",             "multiband_dynamics"),
    ("L1",         r"^L1 limiter (Mono|Stereo)$",     "limiter"),
    ("L2",         r"^L2 (Mono|Stereo)$",             "limiter"),
    ("LinMB",      r"^LinMB (Mono|Stereo)$",          "multiband_dynamics"),
    ("DeEsser",    r"^DeEsser (Mono|Stereo)$",        "de_esser"),
    ("RDeEsser",   r"^RDeEsser (Mono|Stereo)$",       "de_esser"),
    ("Sibilance",  r"^Sibilance (Mono|Stereo)$",      "de_esser"),
    ("PSE",        r"^PSE (Mono|Stereo)$",            "gate_expander"),
    ("SmackAttack",r"^Smack Attack (Mono|Stereo)$",   "transient_shaper"),
    ("TransXWide", r"^TransX Wide (Mono|Stereo)$",    "transient_shaper"),
]

subjects = []
problems = []
notes = []
for key, pattern, family in MANIFEST:
    hits = [e for e in index.get("entries", []) if re.match(pattern, str(e.get("name", "")))]
    if IS_WIN and hits:
        # Dedup same-name duplicates coming from coexisting WaveShell
        # generations: keep one body per inventory name, newest shell wins.
        by_name = {}
        for e in hits:
            name = str(e.get("name", ""))
            rank = (shell_rank(e), str(e.get("plugin_path", "")))
            if name not in by_name:
                by_name[name] = (rank, e)
            elif rank > by_name[name][0]:
                notes.append(f"family {key}: {name}: kept {e.get('plugin_path')} (dropped {by_name[name][1].get('plugin_path')})")
                by_name[name] = (rank, e)
            else:
                notes.append(f"family {key}: {name}: kept {by_name[name][1].get('plugin_path')} (dropped {e.get('plugin_path')})")
        hits = [entry for _, entry in sorted(by_name.values(), key=lambda t: str(t[1].get("name", "")))]
    variants = sorted(str(e.get("name", "")) for e in hits)
    if IS_WIN:
        # A family enumerating a single Mono-or-Stereo variant is a machine
        # fact to record (candidate source of the U2-recorded 23-subject PC
        # count), not a selection failure; darwin keeps the strict pair rule.
        ok = 1 <= len(hits) <= 2 and all(v.endswith((" Mono", " Stereo")) for v in variants)
        if ok and len(hits) == 1:
            notes.append(f"family {key}: single-variant machine fact — only {variants[0]} enumerated on this machine")
    else:
        ok = len(hits) == 2 and any(v.endswith(" Mono") for v in variants) and any(v.endswith(" Stereo") for v in variants)
    if not ok:
        problems.append(f"family {key}: expected plain Mono/Stereo variant bodies, got {variants}")
        continue
    for entry in hits:
        subjects.append({
            "family_key": key,
            "processor_family": family,
            # PC plugin_semantics.json Entry schema fields (name/path/identity)
            "id": entry.get("id", ""),
            "name": entry.get("name", ""),
            "descriptive_name": entry.get("descriptive_name", ""),
            "manufacturer": entry.get("manufacturer", ""),
            "format": entry.get("format", ""),
            "category": entry.get("category", ""),
            "identifier": entry.get("identifier", ""),
            "plugin_path": entry.get("plugin_path", ""),
            "is_instrument": bool(entry.get("is_instrument", False)),
        })
subjects.sort(key=lambda s: (s["processor_family"], s["name"]))
json.dump({
    "schema_version": "pca.calibration_chain.selected_subjects.v1",
    "platform": sys.platform,
    "manifest_families": len(MANIFEST),
    "selected_count": len(subjects),
    "problems": problems,
    "enumeration_notes": notes,
    "subjects": subjects,
}, open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(f"selected={len(subjects)} problems={len(problems)} notes={len(notes)}")
for p in problems:
    print("PROBLEM:", p, file=sys.stderr)
PY
SELECTED_COUNT="$(json_field "$WORKDIR/probe/selected_subjects.json" 'int(d.get("selected_count",0))')"
PROBLEM_COUNT="$(json_field "$WORKDIR/probe/selected_subjects.json" 'len(d.get("problems",[]))')"
if (( PROBLEM_COUNT > 0 )); then
  FAIL_CLASSIFICATION="functional_enumeration"
  fail_functional "subject selection found family mismatches (see $WORKDIR/probe/selected_subjects.json)"
fi
if (( SELECTED_COUNT < EXPECTED_SUBJECTS )); then
  FAIL_CLASSIFICATION="functional_enumeration"
  fail_functional "only $SELECTED_COUNT subjects selected (expected >= $EXPECTED_SUBJECTS)"
fi
log "subject selection: $SELECTED_COUNT subjects across 12 U2 families"

# ----------------------------------------------------------- whitelist draft
log "writing whitelist draft (PC-schema aligned)..."
"$PY" - "$WORKDIR/probe/selected_subjects.json" "$WORKDIR/whitelist_draft.json" "$WORKDIR/probe/plugin_semantics_index.json" <<'PY'
import json, sys

subjects_path, draft_path, index_path = sys.argv[1], sys.argv[2], sys.argv[3]
selection = json.load(open(subjects_path, encoding="utf-8"))
index = json.load(open(index_path, encoding="utf-8"))
subjects = selection["subjects"]

rows = []
for s in subjects:
    rows.append({
        # PC processor_certification_entry.v2 candidate schema fields
        "id": s["id"],
        "name": s["name"],
        "manufacturer": s["manufacturer"],
        "format": s["format"],
        "identifier": s["identifier"],
        "plugin_path": s["plugin_path"],
        "family": s["processor_family"],
        # probe conclusions (this chain's evidence)
        "probe": {
            "enumerated": True,
            "enumeration_source": "kernel scan_plugins via agent plugin.semantic_build_index",
            "semantic_index_entry_id": s["id"],
            "category": s["category"],
            "is_instrument": s["is_instrument"],
            "family_key": s["family_key"],
        },
        # filled after PCA re-certification (see whitelist_draft_final.json)
        "pca": None,
    })

draft = {
    "schema_version": "pca.calibration_chain.whitelist_draft.v1",
    "draft_status": "draft_for_decision_side_adoption",
    "platform": selection.get("platform", "unknown"),
    "subject_count": len(rows),
    "u2_scope": {
        "narrowing": "2026-09-16 user ruling: mac is the Waves-only demo machine; 5 PA + 3 FabFilter subjects are not installed",
        "families": 12,
        "superset_source": f"machine semantic index ({len(index.get('entries', []))} entries) built by plugin.semantic_build_index",
    },
    "enumeration_notes": selection.get("enumeration_notes", []),
    "reconciliation_note": (
        "U2 records the mac calibration face as 23 Waves subjects described as the 12 families' "
        "Mono/Stereo variants. This machine's kernel inventory resolves those 12 plain families to "
        f"{len(rows)} bodies (12 x Mono/Stereo). The draft keeps all {len(rows)} so any PC-side 23-subset "
        "is covered; trimming to exactly 23 is a decision-side adoption action. Product variants "
        "(C1 comp-gate / C1 comp-sc / C6-SideChain / Sibilance-Live / TransX Multi) are excluded "
        "as non-promoted variants."
    ),
    "schema_alignment": {
        "primary": "rows follow the PC-side processor certification candidate schema (agent/internal/chat/processor_certification_entry.go: id/name/manufacturer/format/identifier/plugin_path/family) extended with probe/pca evidence fields",
        "pc_semantic_index": "identity fields (id/name/descriptive_name/manufacturer/format/category/identifier/plugin_path/is_instrument) match the PC ~/.vit/plugin_semantics.json Entry schema (agent/internal/pluginsemantics/index.go)",
        "pc_experiment_whitelist": "the domain-pin view (~/.vit/free_state_experiment_plugins.json, vit.free_state_experiment_plugins.v5) pins one plugin + param ids per domain on PC; adopting mac section pins from these subjects is a follow-up decision, not part of this draft",
        "pca_stores": "attestation records land in ~/.vit/processor_control_attestations.v1.json (broadband_compressor) and .v2.json (limiter/gate_expander/de_esser/transient_shaper/multiband_dynamics) with promoted status",
    },
    "family_assignments": {
        "broadband_compressor": "C1 comp Mono/Stereo",
        "multiband_dynamics": "C4, C6, LinMB Mono/Stereo",
        "limiter": "L1 limiter, L2 Mono/Stereo",
        "de_esser": "DeEsser, RDeEsser, Sibilance Mono/Stereo",
        "gate_expander": "PSE Mono/Stereo",
        "transient_shaper": "Smack Attack, TransX Wide Mono/Stereo",
    },
    "subjects": rows,
}
json.dump(draft, open(draft_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(f"draft_subjects={len(rows)}")
PY
DRAFT_COUNT="$(json_field "$WORKDIR/whitelist_draft.json" 'int(d.get("subject_count",0))')"
[[ "$DRAFT_COUNT" -eq "$SELECTED_COUNT" ]] \
  || fail_functional "whitelist draft subject_count $DRAFT_COUNT != selected $SELECTED_COUNT"
log "whitelist draft written: $WORKDIR/whitelist_draft.json ($DRAFT_COUNT subjects)"

# --------------------------------------------------------- PCA re-certify
# Per subject: consented certification job on a disposable track (real load +
# typed inspect/apply/readback/restore + full snapshot restore + track delete),
# receipts -> ~/.vit/pca_certifications/<job>/, attestation import -> PCA
# stores. A load-stage failure is the R2 stop condition (evidence preserved).
log "PCA re-certification: $SELECTED_COUNT subjects (per-subject timeout ${CERTIFY_TIMEOUT_SECONDS}s)..."
CERT_EXIT=0
"$PY" - "$WORKDIR/probe/selected_subjects.json" "$WORKDIR/cert" "$AGENT_HTTP" "$CERTIFY_TIMEOUT_SECONDS" <<'PY' || CERT_EXIT=$?
import json, sys, time, urllib.error, urllib.request

subjects_path, cert_dir, agent_http, timeout_s = sys.argv[1], sys.argv[2], sys.argv[3], int(sys.argv[4])
selection = json.load(open(subjects_path, encoding="utf-8"))
subjects = selection["subjects"]
CONSENT = "temporary_track_apply_readback_restore"
LOAD_FAILURE_MARKERS = ("pca_load_gate", "plugin.load_to_rack", "instantiate")

def request(url, method="GET", body=None, timeout=60):
    data = None if body is None else json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, json.loads(resp.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as error:
        try:
            return error.code, json.loads(error.read().decode("utf-8", errors="replace"))
        except Exception:
            return error.code, {"status": "error", "error": str(error)}

def is_load_failure(job):
    error = str(job.get("error", "")) + " " + json.dumps(job.get("certification", {}))
    lowered = error.lower()
    return any(marker in lowered for marker in LOAD_FAILURE_MARKERS) and "passed" not in lowered

results = []
aborted = False
for index, subject in enumerate(subjects, 1):
    name, family, identifier = subject["name"], subject["processor_family"], subject["identifier"]
    if aborted:
        results.append({"name": name, "family": family, "identifier": identifier,
                        "status": "skipped_after_load_failure"})
        continue
    started = time.time()
    status, reply = request(
        agent_http.rstrip("/") + "/agent/processor-certification/start", "POST",
        {"identifier": identifier, "family": family, "confirmed": True, "consent": CONSENT}, 60)
    record = {"name": name, "family": family, "identifier": identifier,
              "start_http_status": status, "start_reply": reply}
    if status != 202 or str(reply.get("status", "")) != "accepted":
        record["status"] = "start_failed"
        record["failure_stage"] = "load" if is_load_failure(reply) else "start"
        results.append(record)
        if record["failure_stage"] == "load":
            aborted = True
        continue
    job_id = str(reply.get("job", {}).get("job_id", ""))
    record["job_id"] = job_id
    final = None
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        code, poll = request(agent_http.rstrip("/") + "/agent/processor-certification/status?job_id=" + job_id, "GET", None, 30)
        if code != 200:
            final = {"poll_http_status": code, "poll_reply": poll}
            break
        job = poll.get("job", {})
        state = str(job.get("status", ""))
        if state in ("completed", "failed"):
            final = job
            break
        time.sleep(2)
    if final is None:
        record["status"] = "timeout"
        record["failure_stage"] = "job"
        results.append(record)
        continue
    record["job"] = final
    cert = final.get("certification", {}) if isinstance(final, dict) else {}
    if str(final.get("status", "")) == "completed" and str(cert.get("status", "")) == "passed":
        record["status"] = "certified"
        record["summary_path"] = str(cert.get("summary_path", ""))
        record["evidence_path"] = str(cert.get("evidence_path", ""))
        # Both import fields always serialize (Go struct omitempty is a no-op);
        # the not-applicable generation carries "promoted": null.
        imp_v2 = final.get("import") if isinstance(final.get("import"), dict) else {}
        imp_v1 = final.get("import_v1") if isinstance(final.get("import_v1"), dict) else {}
        record["promoted_count"] = len((imp_v2.get("promoted") or []) + (imp_v1.get("promoted") or []))
    else:
        record["status"] = "failed"
        record["failure_stage"] = "load" if is_load_failure(final) else "verdict"
        if record["failure_stage"] == "load":
            aborted = True
    record["elapsed_seconds"] = round(time.time() - started, 1)
    results.append(record)
    print(f"[{index}/{len(subjects)}] {name} ({family}) -> {record['status']} ({record.get('elapsed_seconds','?')}s)", flush=True)

certified = sum(1 for r in results if r["status"] == "certified")
load_failures = [r for r in results if r.get("failure_stage") == "load"]
summary = {
    "schema_version": "pca.calibration_chain.certification_summary.v1",
    "total": len(subjects),
    "certified": certified,
    "failed": sum(1 for r in results if r["status"] == "failed"),
    "timeouts": sum(1 for r in results if r["status"] == "timeout"),
    "start_failures": sum(1 for r in results if r["status"] == "start_failed"),
    "skipped_after_load_failure": sum(1 for r in results if r["status"] == "skipped_after_load_failure"),
    "load_failure_subjects": [r["name"] for r in load_failures],
    "failure_kind": "subject_load_failure" if load_failures else (None if certified == len(subjects) else "certification"),
    "results": results,
}
json.dump(summary, open(cert_dir + "/certification_summary.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
for r in results:
    json.dump(r, open(f"{cert_dir}/subject_{r['name'].replace(' ', '_')}.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(f"certified={certified}/{len(subjects)} load_failures={len(load_failures)}")
sys.exit(0 if certified == len(subjects) else 1)
PY
if [[ -f "$WORKDIR/cert/certification_summary.json" ]]; then
  CERTIFIED_COUNT="$(json_field "$WORKDIR/cert/certification_summary.json" 'int(d.get("certified",0))')"
  CERT_FAILURE_KIND="$(json_field "$WORKDIR/cert/certification_summary.json" 'str(d.get("failure_kind","none") or "none")')"
else
  CERTIFIED_COUNT=0
  CERT_FAILURE_KIND="crashed"
fi
if [[ "$CERT_FAILURE_KIND" == "subject_load_failure" ]]; then
  FAIL_CLASSIFICATION="functional_subject_load"
elif [[ "$CERT_EXIT" -ne 0 ]]; then
  FAIL_CLASSIFICATION="functional_certification"
fi
log "PCA re-certification: $CERTIFIED_COUNT/$SELECTED_COUNT certified (failure_kind=$CERT_FAILURE_KIND)"

# ------------------------------------------------------ finalize the draft
log "finalizing whitelist draft with PCA conclusions..."
"$PY" - "$WORKDIR/whitelist_draft.json" "$WORKDIR/cert/certification_summary.json" "$WORKDIR/whitelist_draft_final.json" <<'PY'
import json, sys

draft_path, cert_path, final_path = sys.argv[1], sys.argv[2], sys.argv[3]
draft = json.load(open(draft_path, encoding="utf-8"))
cert = json.load(open(cert_path, encoding="utf-8"))
by_name = {r["name"]: r for r in cert.get("results", [])}

for subject in draft["subjects"]:
    record = by_name.get(subject["name"], {})
    subject["pca"] = {
        "certification_status": record.get("status", "missing"),
        "failure_stage": record.get("failure_stage"),
        "job_id": record.get("job_id"),
        "receipt_summary_path": record.get("summary_path"),
        "receipt_evidence_path": record.get("evidence_path"),
        "promoted_count": record.get("promoted_count", 0),
    }
draft["finalized_at"] = __import__("time").strftime("%Y-%m-%dT%H:%M:%SZ", __import__("time").gmtime())
draft["certified_count"] = sum(1 for s in draft["subjects"] if s["pca"]["certification_status"] == "certified")
json.dump(draft, open(final_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(f"final_subjects={len(draft['subjects'])} certified={draft['certified_count']}")
PY
log "final whitelist draft: $WORKDIR/whitelist_draft_final.json"

# ------------------------------------------------------------ readonly smoke
# C4/A5 whitelist GETs only, plus the read-only certification candidates view
# and a read-only parse of the machine-local PCA stores. No mutations.
readonly_get() {
  local path="$1" out_file="$2" matched=0 allowed
  for allowed in "${ALLOWED_GET_PATHS[@]}"; do
    [[ "$allowed" == "$path" ]] && { matched=1; break; }
  done
  [[ "$matched" -eq 1 ]] || fail_functional "GET helper rejected non-whitelisted path: $path"
  local uri="${AGENT_HTTP%/}${path}"
  if [[ "$path" == "/agent/events" ]]; then
    uri="${uri}?conversation_id=${CONVERSATION_ID}&limit=200"
  fi
  local status
  status="$(curl -sS --max-time 30 -o "$out_file" -w '%{http_code}' "$uri")" \
    || fail_functional "GET $path request failed"
  [[ "$status" =~ ^2 ]] || fail_functional "GET $path returned HTTP $status"
}

log "readonly smoke: whitelist GETs..."
for path in "/health" "/agent/runtime/status" "/agent/state" "/agent/events"; do
  out="$WORKDIR/http/${path//\//_}.json"
  readonly_get "$path" "$out"
  log "GET $path -> 2xx ($(wc -c < "$out" | tr -d ' ') bytes)"
done

log "readonly smoke: certification candidates (promoted assertion) + PCA store parse..."
READONLY_OK=0
"$PY" - "$WORKDIR/probe/selected_subjects.json" "$WORKDIR/http" "$AGENT_HTTP" "$VIT_HOME" "$WORKDIR/readonly_verification.json" <<'PY' || READONLY_OK=$?
import json, os, sys

subjects_path, http_dir, agent_http, vit_home, out_path = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5]
selection = json.load(open(subjects_path, encoding="utf-8"))
subjects = selection["subjects"]

with open(os.path.join(http_dir, "candidates_all.json"), "w", encoding="utf-8") as f:
    import urllib.request
    with urllib.request.urlopen(agent_http.rstrip("/") + "/agent/processor-certification/candidates?family=all", timeout=120) as resp:
        f.write(resp.read().decode("utf-8", errors="replace"))
candidates_raw = json.load(open(os.path.join(http_dir, "candidates_all.json"), encoding="utf-8"))
by_identifier = {}
for c in candidates_raw.get("candidates", []):
    by_identifier[c.get("identifier", "")] = c

missing, not_promoted, promoted = [], [], 0
for s in subjects:
    candidate = by_identifier.get(s["identifier"])
    if candidate is None:
        missing.append(s["name"])
        continue
    capability = next((cap for cap in candidate.get("capabilities", []) if cap.get("family") == s["processor_family"]), None)
    if capability is None:
        not_promoted.append(f"{s['name']}: no {s['processor_family']} capability")
    elif str(capability.get("status", "")) != "promoted":
        not_promoted.append(f"{s['name']}: {s['processor_family']}={capability.get('status')} ({capability.get('reason')})")
    else:
        promoted += 1

stores = {}
for store_name in ("processor_control_attestations.v1.json", "processor_control_attestations.v2.json"):
    path = os.path.join(vit_home, store_name)
    entry = {"path": path, "exists": os.path.isfile(path)}
    if entry["exists"]:
        library = json.load(open(path, encoding="utf-8"))
        attestations = library.get("attestations", [])
        entry["schema_version"] = library.get("schema_version")
        entry["attestation_count"] = len(attestations)
        entry["statuses"] = {}
        for attestation in attestations:
            status = str(attestation.get("status", ""))
            entry["statuses"][status] = entry["statuses"].get(status, 0) + 1
        entry["families"] = sorted({str(a.get("processor_family", "")) for a in attestations})
        entry["subjects"] = sorted({str(a.get("subject", {}).get("name", "")) for a in attestations})
    stores[store_name] = entry

receipts_dir = os.path.join(vit_home, "pca_certifications")
receipt_jobs = sorted(os.listdir(receipts_dir)) if os.path.isdir(receipts_dir) else []

report = {
    "schema_version": "pca.calibration_chain.readonly_verification.v1",
    "subjects_expected_promoted": len(subjects),
    "subjects_promoted": promoted,
    "subjects_missing_from_candidates": missing,
    "subjects_not_promoted": not_promoted,
    "pca_stores": stores,
    "receipt_jobs": receipt_jobs,
    "pass": promoted == len(subjects) and not missing and not not_promoted,
}
json.dump(report, open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(f"promoted={promoted}/{len(subjects)} missing={len(missing)} not_promoted={len(not_promoted)}")
for name in missing:
    print("MISSING:", name, file=sys.stderr)
for name in not_promoted:
    print("NOT_PROMOTED:", name, file=sys.stderr)
sys.exit(0 if report["pass"] else 1)
PY
if [[ "$READONLY_OK" -ne 0 && -z "$FAIL_CLASSIFICATION" ]]; then
  FAIL_CLASSIFICATION="functional_readonly"
fi

# ---------------------------------------------------------------- stop stack
# Runs in the MAIN shell (not under $()): wait must reap the parent shell's own
# children to capture the real exit code (A1 finding: kernel SIGTERM -> 143
# without JUCE shutdown).
STOP_RECORD=""
stop_process() {
  local pid="$1" start elapsed code signal="SIGTERM"
  kill -TERM "$pid" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -KILL "$pid" 2>/dev/null || true; signal="SIGTERM+SIGKILL"; break; }
    sleep 1
  done
  set +e
  wait "$pid" 2>/dev/null
  code=$?
  set -e
  elapsed=$((SECONDS - start))
  STOP_RECORD="${signal}:${code}:${elapsed}"
}

log "stopping agent (pid $AGENT_PID)..."
stop_process "$AGENT_PID"
AGENT_STOP_RECORD="$STOP_RECORD"
log "agent stop: $AGENT_STOP_RECORD"

log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "kernel stop: $KERNEL_STOP_RECORD (A1 finding: SIGTERM expected to yield 143)"
KERNEL_PID=""
AGENT_PID=""

# ------------------------------------------------- machine-local post-state
snapshot_vit_state "$WORKDIR/vit_state_after.txt"

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
"$PY" - "$SUMMARY_FILE" "$RUN_ID" "$WORKDIR" \
  "$KERNEL_STOP_RECORD" "$AGENT_STOP_RECORD" \
  "$PLUGIN_COUNT" "$SELECTED_COUNT" "$DRAFT_COUNT" "$CERTIFIED_COUNT" "$CERT_FAILURE_KIND" \
  "$READONLY_OK" "$FAIL_CLASSIFICATION" "$PLATFORM_TAG" <<'PY'
import json, sys

(out, run_id, workdir, kernel_stop, agent_stop, plugin_count, selected_count,
 draft_count, certified_count, cert_failure_kind, readonly_ok, fail_class,
 platform_tag) = sys.argv[1:14]

readonly_report = {}
try:
    readonly_report = json.load(open(workdir + "/readonly_verification.json", encoding="utf-8"))
except Exception:
    pass

gates = {
    "stack_start_and_roundtrip": True,
    "probe_enumeration": int(plugin_count) >= 23,
    "whitelist_draft_written": int(draft_count) == int(selected_count) and int(draft_count) >= 23,
    "pca_certification_all": int(certified_count) == int(selected_count),
    "pca_store_landed": bool(
        readonly_report.get("pca_stores", {}).get("processor_control_attestations.v2.json", {}).get("exists")
        or readonly_report.get("pca_stores", {}).get("processor_control_attestations.v1.json", {}).get("exists")
    ),
    "readonly_smoke": int(readonly_ok) == 0,
}
summary = {
    "schema_version": "pca.calibration_chain.mac.v1",
    "platform": platform_tag,
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": fail_class or ("none" if all(gates.values()) else "functional"),
    "u2_scope": {"families": 12, "selected_subjects": int(selected_count),
                 "note": "12 plain Waves families enumerate to 24 Mono/Stereo bodies; U2 records 23 — see whitelist_draft.json reconciliation_note"},
    "kernel_stop": {"record": kernel_stop},
    "agent_stop": {"record": agent_stop},
    "probe": {"semantic_index_plugin_count": int(plugin_count)},
    "whitelist_draft": {"subjects": int(draft_count), "path": workdir + "/whitelist_draft_final.json"},
    "pca_certification": {"certified": int(certified_count), "total": int(selected_count),
                          "failure_kind": cert_failure_kind},
    "readonly_verification": readonly_report,
    "gates": gates,
    "artifacts_dir": workdir,
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY

cat "$SUMMARY_FILE"
[[ -n "$OUTPUT_PATH" ]] && { mkdir -p "$(dirname "$OUTPUT_PATH")"; cp "$SUMMARY_FILE" "$OUTPUT_PATH"; }
log "workdir: $WORKDIR"

if [[ -n "$FAIL_CLASSIFICATION" ]] || ! "$PY" -c '
import json, sys
d = json.load(open(sys.argv[1]))
sys.exit(0 if all(d["gates"].values()) else 1)' "$SUMMARY_FILE"; then
  exit 1
fi
exit 0
