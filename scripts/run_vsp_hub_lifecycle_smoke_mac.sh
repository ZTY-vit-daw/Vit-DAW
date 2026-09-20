#!/usr/bin/env bash
# VSP Hub product lifecycle smoke, mac equivalent — PORT-SMOKE-MAC-3
# (piece 5/5).
#
# Mac port of scripts/run_vsp_hub_lifecycle_smoke.ps1 (PC-only original,
# zero changes there) on the card's three-piece stack basis: the PC script
# starts the stack THROUGH the Godot start_page autostart (kernel + hub +
# agent as Godot children) and then verifies the product lifecycle; on mac
# the stack start/stop follows the PORT-VSPHUB-1 recipe (harness-owned
# kernel + vsphub + agent, same fixed ports, same required registration),
# and every post-start lifecycle gate is ported 1:1.
#
#   | ps1 (run_vsp_hub_lifecycle_smoke.ps1)  | mac (this script)             |
#   |----------------------------------------|-------------------------------|
#   | build VitAgent.exe + VspHub.exe into   | go build agent+vsphub into    |
#   |   agent\bin (product path)             |   the run workdir; sha256 of  |
#   |                                        |   agent/bin/{vitagent,vsphub} |
#   |                                        |   product-path copies also    |
#   |                                        |   recorded (never written)    |
#   | kernel exe resolved from VitApp\build  | --kernel-bin (default: repo's |
#   |   release artefacts                    |   VitApp/build/…/Debug/VitApp |
#   |                                        |   if present, else cmake      |
#   |                                        |   build; sha256+mtime recorded)|
#   | Start Godot project runtime; autostart | VSPHUB-1 recipe stack start:  |
#   |   Kernel+Hub+Agent as Godot children   |   kernel (fake root) → agent  |
#   | Wait lifecycle evidence (autostart     |   BEFORE hub with required    |
#   |   log lines + ports + ready logs)      |   registration (retry window  |
#   |                                        |   evidenced) → hub health →   |
#   |                                        |   registration success        |
#   | Verify process ownership: hub 8787 /   | same: listener pid == started |
#   |   agent 7878 / kernel 5555+5556 owner  |   pid; running exe path +     |
#   |   path == expected exe                 |   sha256 == expected binary   |
#   | Verify running Hub/Agent binary hashes | same (sha256 of the running   |
#   |   match expected                       |   exe from lsof)              |
#   | listeners snapshot (ports.json)        | same (lsof evidence file)     |
#   | GET /health: service=VspHub status=ok  | same                          |
#   | GET /vsp/status: vsp.hub.http +        | same                          |
#   |   vit.agent.official role=agent        |                               |
#   | Godot headless realtime ws probe:      | same probe, VSPHUB-1 recipe   |
#   |   subscribe vsp.hub.websocket, frames  |   (ENABLE=1 + timeout env;    |
#   |   without HTTP fallback                |   --skip-ws-probe to skip)    |
#   | run vsp_phase4_hub_http_asset_smoke.py | same py UNTOUCHED against the |
#   |   against the live hub                 |   live hub (--skip-hub-http-  |
#   |                                        |   asset-smoke to skip)        |
#   | summary.json in repo Workspace         | summary.json in the isolated  |
#   |   Artifacts                            |   run workdir (AGENTS §10)    |
#   | teardown: stop Godot/agent/hub/kernel  | stop records + port-drain     |
#   |   by owner-path match                  |   assert (never stops foreign |
#   |                                        |   processes)                  |
#   | optional: 60-track playback, GUI       | out of mac three-piece scope  |
#   |   health/architecture probes, -Reuse*  | (documented in SMOKE_TESTS)   |
#
# §8 discipline (pre-declared on the card): deterministic lifecycle gates —
# at most 3 valid runs, success = single authoritative run exit 0 with every
# gate green; same-breakpoint two-failure stop-loss.

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
HUB_HTTP="http://127.0.0.1:8787"
KERNEL_BIN_ARG=""
SKIP_BUILDS=0
AGENT_BIN_ARG=""
HUB_BIN_ARG=""
WORKDIR_ARG=""
ARTIFACT_ROOT_ARG=""
OUTPUT_PATH=""
TRACKTION_DIR=""
SKIP_WS_PROBE=0
SKIP_HUB_HTTP_ASSET_SMOKE=0
GODOT_BIN="/Users/timozty/Applications/Godot.app/Contents/MacOS/Godot"
FRONTEND_ROOT="$HOME/Documents/vit-daw-frontend"
UDP_TO_GODOT=4444
UDP_FROM_GODOT=4445
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555

usage() {
  cat <<'EOF'
run_vsp_hub_lifecycle_smoke_mac.sh — VSP Hub product lifecycle smoke on the
mac three-piece stack (kernel + vsphub + agent) — PORT-SMOKE-MAC-3 piece 5/5,
mac port of run_vsp_hub_lifecycle_smoke.ps1.

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --hub-http URL          VSP Hub HTTP base (default http://127.0.0.1:8787)
  --kernel-bin PATH       Kernel binary (default: the repo's
                          VitApp/build/VitApp_artefacts/Debug/VitApp when it
                          exists, else a cmake build in the run workdir;
                          sha256+mtime recorded either way)
  --skip-builds           With --agent-bin/--hub-bin: do not build either
  --agent-bin PATH        Agent binary to start (only with --skip-builds)
  --hub-bin PATH          vsphub binary to start (only with --skip-builds)
  --skip-ws-probe         Skip the Godot headless realtime ws consumption
                          probe (PC -skip equivalent; default runs it)
  --skip-hub-http-asset-smoke  Skip the vsp_phase4_hub_http_asset_smoke.py
                          step (ps1 -SkipHubHttpAssetSmoke; default runs it)
  --godot-bin PATH        Godot binary for the ws probe
  --frontend-root PATH    Frontend repo root for the ws probe
  --udp-to-godot N        Agent telemetry UDP destination port (default 4444)
  --udp-from-godot N      Agent command UDP listen port (default 4445)
  --startup-timeout SECONDS  Kernel/agent/hub startup timeout (default 90)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --workdir PATH          Reuse PATH as the run workdir (never overwrites)
  --artifact-root DIR     Artifact root (default
                          ~/Documents/vit-smoke-mac3-artifacts; each run gets
                          a fresh <root>/<run_id>/ dir; ignored with --workdir)
  --tracktion-dir PATH    tracktion_engine source dir (default: the repo's
                          submodule checkout)
  --output PATH           Also write the summary JSON to PATH
  -h, --help              Show this help

Exit codes: 0 = all gates passed; 1 = functional failure; 3 = env failure.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --hub-http) HUB_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-builds) SKIP_BUILDS=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --hub-bin) HUB_BIN_ARG="$2"; shift 2 ;;
    --skip-ws-probe) SKIP_WS_PROBE=1; shift ;;
    --skip-hub-http-asset-smoke) SKIP_HUB_HTTP_ASSET_SMOKE=1; shift ;;
    --godot-bin) GODOT_BIN="$2"; shift 2 ;;
    --frontend-root) FRONTEND_ROOT="$2"; shift 2 ;;
    --udp-to-godot) UDP_TO_GODOT="$2"; shift 2 ;;
    --udp-from-godot) UDP_FROM_GODOT="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --artifact-root) ARTIFACT_ROOT_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 3 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 3; }

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

for tool in go cmake make python3 lsof shasum curl git; do
  command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
done

RUN_ID="vsp_hub_lifecycle_mac_$(date '+%Y%m%d-%H%M%S')"
if [[ -n "$WORKDIR_ARG" ]]; then
  WORKDIR="$WORKDIR_ARG"
else
  ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac3-artifacts}"
  WORKDIR="$ARTIFACT_ROOT/$RUN_ID"
fi
mkdir -p "$WORKDIR"/{build,bin,http,logs,hub} || fail_env "cannot create workdir: $WORKDIR"
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT"
# Fake VitApp root markers: VitPaths.h climbToVitAppRoot() anchors the kernel's
# Workspace/{Logs,Settings,Cache} + default_project.xml here (AGENTS §10).
: > "$KERNEL_ROOT/CMakeLists.txt"
mkdir -p "$KERNEL_ROOT/Source"
AGENT_STATE_DIR="$WORKDIR/agent_state"
mkdir -p "$AGENT_STATE_DIR"

KERNEL_PID=""
HUB_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
HUB_STOP_RECORD=""
AGENT_STOP_RECORD=""
GATES_FILE="$WORKDIR/gates.env"
: > "$GATES_FILE"
gate() { printf '%s=%s\n' "$1" "$2" >> "$GATES_FILE"; }

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

cleanup() {
  local rc=$?
  for pid_sig in "$AGENT_PID:TERM" "$HUB_PID:TERM" "$KERNEL_PID:TERM"; do
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

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

exe_path_of() {
  lsof -nP -p "$1" 2>/dev/null | awk '$4=="txt" && $5=="REG" {print $9; exit}' || true
}

same_file() {
  python3 -c '
import os, sys
try:
    a = os.stat(sys.argv[1])
    b = os.stat(sys.argv[2])
except OSError:
    raise SystemExit(1)
raise SystemExit(0 if (a.st_dev, a.st_ino) == (b.st_dev, b.st_ino) else 1)' "$1" "$2"
}

AGENT_HTTP_ADDR="${AGENT_HTTP#http://}"
AGENT_HTTP_ADDR="${AGENT_HTTP_ADDR#https://}"
AGENT_HTTP_PORT="${AGENT_HTTP_ADDR##*:}"
HUB_HTTP_ADDR="${HUB_HTTP#http://}"
HUB_HTTP_ADDR="${HUB_HTTP_ADDR#https://}"
HUB_HTTP_PORT="${HUB_HTTP_ADDR##*:}"
HUB_VSP_URL="${HUB_HTTP%/}/vsp"
HUB_STATUS_URL="${HUB_HTTP%/}/vsp/status"

assert_ports_free() {
  local port pid
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "$AGENT_HTTP_PORT" "$HUB_HTTP_PORT"; do
    pid="$(port_listener_pid "$port")"
    if [[ -n "$pid" ]]; then
      fail_env "port $port already has a listener (pid $pid); this harness never stops processes it did not start (AGENTS §9 single-owner rule)"
    fi
  done
}

assert_ports_free

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_vsp_hub_lifecycle_smoke_mac.sh (mac port of run_vsp_hub_lifecycle_smoke.ps1, PORT-SMOKE-MAC-3 piece 5/5)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "hub_http=$HUB_HTTP"
  echo "skip_ws_probe=$SKIP_WS_PROBE"
  echo "skip_hub_http_asset_smoke=$SKIP_HUB_HTTP_ASSET_SMOKE"
  echo "godot_bin=$GODOT_BIN"
  echo "frontend_root=$FRONTEND_ROOT"
  echo "udp_to_godot=$UDP_TO_GODOT udp_from_godot=$UDP_FROM_GODOT"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

# ---------------------------------------------------------------- binaries (prepare + evidence)
KERNEL_BIN=""
REPO_KERNEL_DEFAULT="$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp"
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
elif [[ -x "$REPO_KERNEL_DEFAULT" ]]; then
  KERNEL_BIN="$REPO_KERNEL_DEFAULT"
  log "reusing the repo's built kernel binary: $KERNEL_BIN"
else
  KERNEL_BIN=""
fi
if [[ -z "$KERNEL_BIN" ]]; then
  if [[ -z "$TRACKTION_DIR" ]]; then
    TRACKTION_DIR="$REPO_ROOT/tracktion_engine"
  fi
  if [[ ! -f "$TRACKTION_DIR/CMakeLists.txt" ]]; then
    fail_env "tracktion_engine not usable at $TRACKTION_DIR (submodule not checked out? pass --tracktion-dir pointing at a checkout of the pinned commit)"
  fi
  log "building VitApp kernel (cmake+make)..."
  if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug \
        -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fail_env "kernel cmake configure failed (see $WORKDIR/logs/kernel_configure.log)"
  fi
  if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp \
        > "$WORKDIR/logs/kernel_build.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_build.log" >&2
    fail_env "kernel make failed (see $WORKDIR/logs/kernel_build.log)"
  fi
  KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  [[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary not found after build: $KERNEL_BIN"
  log "kernel built: $KERNEL_BIN"
fi
{
  shasum -a 256 "$KERNEL_BIN"
  stat -f "mtime=%Sm" -t "%Y-%m-%dT%H:%M:%S%z" "$KERNEL_BIN"
} > "$WORKDIR/kernel_bin.sha256"

if [[ "$SKIP_BUILDS" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" && -n "$HUB_BIN_ARG" ]] \
    || fail_env "--skip-builds requires both --agent-bin and --hub-bin"
  [[ -x "$AGENT_BIN_ARG" ]] || fail_env "agent binary is not executable: $AGENT_BIN_ARG"
  [[ -x "$HUB_BIN_ARG" ]] || fail_env "hub binary is not executable: $HUB_BIN_ARG"
  AGENT_BIN="$AGENT_BIN_ARG"
  HUB_BIN="$HUB_BIN_ARG"
else
  [[ -z "$AGENT_BIN_ARG" && -z "$HUB_BIN_ARG" ]] \
    || fail_env "--agent-bin/--hub-bin are only valid together with --skip-builds"
  log "building agent + vsphub (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$WORKDIR/bin/vitagent" ./cmd/vitagent) \
    || fail_env "go build agent failed"
  (cd "$REPO_ROOT/agent" && go build -o "$WORKDIR/bin/vsphub" ./cmd/vsphub) \
    || fail_env "go build vsphub failed"
  AGENT_BIN="$WORKDIR/bin/vitagent"
  HUB_BIN="$WORKDIR/bin/vsphub"
fi
shasum -a 256 "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256"
shasum -a 256 "$HUB_BIN" > "$WORKDIR/hub_bin.sha256"

# Product-path binary evidence (never written by this run; VSPHUB-1 keeps
# agent/bin/vsphub as the start_page autostart candidate).
PRODUCT_AGENT_BIN="$REPO_ROOT/agent/bin/vitagent"
PRODUCT_HUB_BIN="$REPO_ROOT/agent/bin/vsphub"
{
  echo "agent_workdir_sha256=$(cut -d' ' -f1 < "$WORKDIR/agent_bin.sha256")"
  echo "hub_workdir_sha256=$(cut -d' ' -f1 < "$WORKDIR/hub_bin.sha256")"
  if [[ -x "$PRODUCT_AGENT_BIN" ]]; then
    echo "agent_product_path=$PRODUCT_AGENT_BIN"
    echo "agent_product_sha256=$(shasum -a 256 "$PRODUCT_AGENT_BIN" | cut -d' ' -f1)"
  else
    echo "agent_product_path=(absent)"
  fi
  if [[ -x "$PRODUCT_HUB_BIN" ]]; then
    echo "hub_product_path=$PRODUCT_HUB_BIN"
    echo "hub_product_sha256=$(shasum -a 256 "$PRODUCT_HUB_BIN" | cut -d' ' -f1)"
  else
    echo "hub_product_path=(absent)"
  fi
} > "$WORKDIR/binary_evidence.txt"
gate binaries_prepared true
log "binaries: agent=$(cut -c1-16 "$WORKDIR/agent_bin.sha256")... hub=$(cut -c1-16 "$WORKDIR/hub_bin.sha256")... kernel=$(cut -c1-16 "$WORKDIR/kernel_bin.sha256")..."

# ---------------------------------------------------------------- start kernel (stack: piece 1/3)
log "starting kernel (cwd=$KERNEL_ROOT, VIT_ENABLE_SHARED_MEMORY_TEST=1)..."
(
  cd "$KERNEL_ROOT"
  VIT_ENABLE_SHARED_MEMORY_TEST=1 exec "$KERNEL_BIN"
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
lsof -nP -iTCP:"$KERNEL_PORT_REQ" -iTCP:"$ZMQ_PUB_PORT" -iTCP:"$ZMQ_LOG_PORT" -sTCP:LISTEN \
  > "$WORKDIR/lsof_kernel_ports.txt" 2>&1 || true
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  listener_pid="$(port_listener_pid "$port")"
  [[ "$listener_pid" == "$KERNEL_PID" ]] || fail_functional "port $port listener pid $listener_pid != kernel pid $KERNEL_PID"
done
gate kernel_start_ports true
log "kernel ZMQ ports listening (owned by pid $KERNEL_PID)"

# ------------------------------------------------ stack: agent BEFORE hub (retry evidence)
log "starting agent with required registration (hub not up yet; retry window opens)..."
(
  cd "$WORKDIR"
  VIT_AGENT_VSP_HUB_REQUIRED=true \
  VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
  exec "$AGENT_BIN" \
    -http "$AGENT_HTTP_ADDR" \
    -last-log-path "$AGENT_STATE_DIR/agent_last.log" \
    -udp-to-godot "$UDP_TO_GODOT" \
    -udp-from-godot "$UDP_FROM_GODOT" \
    -vsp-hub-url "$HUB_VSP_URL"
) > "$WORKDIR/logs/agent_stdout.log" 2>&1 &
AGENT_PID=$!
log "agent pid=$AGENT_PID"

AGENT_LOG="$AGENT_STATE_DIR/agent_last.log"
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
gate agent_start_health true
log "agent /health ready (hub still down)"

retry_pending_count=0
deadline=$((SECONDS + 20))
while (( SECONDS < deadline )); do
  retry_pending_count="$(grep -c "VSP Hub registration pending" "$AGENT_LOG" 2>/dev/null || true)"
  (( retry_pending_count >= 1 )) && break
  agent_dead && fail_functional "agent exited while waiting for the hub retry window (see $AGENT_LOG)"
  sleep 1
done
[[ "$retry_pending_count" -ge 1 ]] \
  || fail_functional "no 'VSP Hub registration pending' line observed while the hub was down (retry loop not running?)"
gate registration_retry_pending true
log "retry window evidenced: $retry_pending_count pending line(s) before hub start"

# ---------------------------------------------------------------- stack: hub (piece 3/3)
log "starting vsphub (kernel defaults 5555/5556; log isolated in workdir)..."
(
  cd "$KERNEL_ROOT"
  VIT_VSP_HUB_ADDR="127.0.0.1:$HUB_HTTP_PORT" \
  VIT_VSP_HUB_KERNEL_REQ_URL="$ZMQ_REQ_ENDPOINT" \
  VIT_VSP_HUB_KERNEL_SUB_URL="tcp://127.0.0.1:$ZMQ_PUB_PORT" \
  VIT_VSP_HUB_LAST_LOG_PATH="$WORKDIR/hub/vsp_hub_last.log" \
  exec "$HUB_BIN"
) > "$WORKDIR/logs/hub_stdout.log" 2>&1 &
HUB_PID=$!
log "hub pid=$HUB_PID"

hub_dead() { ! kill -0 "$HUB_PID" 2>/dev/null; }

hub_health_ok() {
  curl -sS --max-time 3 "$HUB_HTTP/health" 2>/dev/null \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); sys.exit(0 if d.get("status")=="ok" and d.get("service")=="VspHub" else 1)' 2>/dev/null
}

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  if hub_dead; then
    tail -30 "$WORKDIR/logs/hub_stdout.log" >&2
    fail_functional "vsphub process exited during startup (see $WORKDIR/logs/hub_stdout.log)"
  fi
  hub_health_ok && break
  sleep 1
done
hub_health_ok || fail_functional "hub /health not ok within ${STARTUP_TIMEOUT_SECONDS}s"
listener_pid="$(port_listener_pid "$HUB_HTTP_PORT")"
[[ "$listener_pid" == "$HUB_PID" ]] \
  || fail_functional "hub HTTP port $HUB_HTTP_PORT listener pid $listener_pid != hub pid $HUB_PID"
curl -sS --max-time 5 "$HUB_HTTP/health" > "$WORKDIR/hub/health.json" || true
gate hub_start_health true
log "hub /health ok (port $HUB_HTTP_PORT owned by pid $HUB_PID)"

# ---------------------------------------------------------------- registration success
wait_hub_lists_agent() {
  local deadline=$((SECONDS + 30)) status_json
  while (( SECONDS < deadline )); do
    status_json="$(curl -sS --max-time 5 "$HUB_STATUS_URL" 2>/dev/null || true)"
    if [[ -n "$status_json" ]] && printf '%s' "$status_json" | python3 -c '
import json, sys
d = json.load(sys.stdin)
ok = any(s.get("role") == "agent" and s.get("client_id") == "vit.agent.official"
         for s in d.get("sessions", []))
sys.exit(0 if ok else 1)' 2>/dev/null; then
      printf '%s' "$status_json" > "$WORKDIR/hub/status_with_agent.json"
      return 0
    fi
    sleep 1
  done
  return 1
}

wait_hub_lists_agent \
  || fail_functional "hub /vsp/status never listed the agent session (role=agent client_id=vit.agent.official)"
grep "VitAgent registered with VSP Hub" "$AGENT_LOG" > "$WORKDIR/registration_success_line.txt" 2>/dev/null \
  || fail_functional "agent log has no 'VitAgent registered with VSP Hub' line (see $AGENT_LOG)"
gate registration_success true
log "agent registered: $(cat "$WORKDIR/registration_success_line.txt")"

# ---------------------------------------------------------------- process ownership + binary hash (ps1 "Verify process ownership")
verify_port_owner() {
  local port="$1" label="$2" expected_bin="$3"
  local pid exe running_sha expected_sha
  pid="$(port_listener_pid "$port")"
  [[ -n "$pid" ]] || fail_functional "$label port $port is not listening"
  exe="$(exe_path_of "$pid")"
  [[ -n "$exe" ]] || fail_functional "$label: no executable path resolved for pid $pid (port $port)"
  same_file "$exe" "$expected_bin" \
    || fail_functional "$label owner path mismatch (port $port): expected=$expected_bin actual=$exe pid=$pid"
  running_sha="$(shasum -a 256 "$exe" | cut -d' ' -f1)"
  expected_sha="$(shasum -a 256 "$expected_bin" | cut -d' ' -f1)"
  [[ "$running_sha" == "$expected_sha" ]] \
    || fail_functional "$label running binary sha256 mismatch (port $port): expected=$expected_sha actual=$running_sha"
  printf '%s|%s|%s|%s|%s\n' "$port" "$label" "$pid" "$exe" "$running_sha" >> "$WORKDIR/ownership.txt"
}

: > "$WORKDIR/ownership.txt"
log "verifying process ownership + running binary hashes..."
verify_port_owner "$HUB_HTTP_PORT" "VSP Hub HTTP" "$HUB_BIN"
verify_port_owner "$AGENT_HTTP_PORT" "agent HTTP" "$AGENT_BIN"
verify_port_owner "$KERNEL_PORT_REQ" "kernel command" "$KERNEL_BIN"
verify_port_owner "$ZMQ_PUB_PORT" "kernel event" "$KERNEL_BIN"
gate port_ownership_and_hashes true
log "ownership verified: hub/agent/kernel ports owned by this run's binaries (see $WORKDIR/ownership.txt)"

# Listeners snapshot (ps1 Get-ListenersSnapshot equivalent).
{
  for port in "$AGENT_HTTP_PORT" "$HUB_HTTP_PORT" "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT"; do
    pid="$(port_listener_pid "$port")"
    if [[ -n "$pid" ]]; then
      echo "port=$port listening=true pid=$pid path=$(exe_path_of "$pid")"
    else
      echo "port=$port listening=false"
    fi
  done
} > "$WORKDIR/ports.txt"

# ---------------------------------------------------------------- hub health + status + agent session (ps1 "Verify Hub health and status")
log "verifying hub health + status (VitAgent session, transports)..."
python3 - "$HUB_HTTP/health" "$HUB_STATUS_URL" "$WORKDIR/hub" <<'PY' \
  || fail_functional "hub health/status verification failed (see $WORKDIR/hub)"
import json, sys, urllib.request

health_url, status_url, hub_dir = sys.argv[1:4]

with urllib.request.urlopen(health_url, timeout=10) as r:
    health = json.loads(r.read().decode("utf-8"))
json.dump(health, open(hub_dir + "/health_verify.json", "w", encoding="utf-8"),
          ensure_ascii=False, indent=2)
if health.get("status") != "ok" or health.get("service") != "VspHub":
    raise SystemExit(1)

with urllib.request.urlopen(status_url, timeout=10) as r:
    status = json.loads(r.read().decode("utf-8"))
json.dump(status, open(hub_dir + "/status_verify.json", "w", encoding="utf-8"),
          ensure_ascii=False, indent=2)
transports = status.get("transports") or []
if "vsp.hub.http" not in transports:
    print("PROBE_FAIL: vsp.hub.http not advertised: %r" % transports, file=sys.stderr)
    raise SystemExit(1)
ok = any(s.get("role") == "agent" and s.get("client_id") == "vit.agent.official"
         for s in status.get("sessions", []))
if not ok:
    print("PROBE_FAIL: no vit.agent.official agent session in /vsp/status", file=sys.stderr)
    raise SystemExit(1)
print("hub status ok: transports=%s session_count=%s" %
      (transports, status.get("session_count")))
PY
gate hub_health_status_agent_session true
log "VSP Hub health/status passed with VitAgent session"

# ---------------------------------------------------------------- Godot realtime ws consumption (ps1 step, VSPHUB-1 recipe)
WS_PROBE_SUMMARY="skipped"
if [[ "$SKIP_WS_PROBE" -eq 0 ]]; then
  [[ -x "$GODOT_BIN" ]] || fail_env "--ws-probe step: Godot binary not executable: $GODOT_BIN (pass --godot-bin or --skip-ws-probe)"
  [[ -f "$FRONTEND_ROOT/project.godot" ]] || fail_env "--ws-probe step: frontend project not found: $FRONTEND_ROOT (pass --frontend-root or --skip-ws-probe)"
  [[ -f "$FRONTEND_ROOT/tools/diagnostics/vsp_realtime_ws_live_probe.gd" ]] \
    || fail_env "--ws-probe step: frontend ws live probe script not found under $FRONTEND_ROOT"
  log "ws consumption probe: Godot headless vsp_realtime_ws_live_probe against the live hub (ENABLE=1)..."
  set +e
  (
    cd "$FRONTEND_ROOT"
    VIT_GUI_VSP_REALTIME_WS_ENABLE=1 \
    VIT_WS_LIVE_PROBE_TIMEOUT_SEC=30 \
    exec "$GODOT_BIN" --headless --path "$FRONTEND_ROOT" \
      --script res://tools/diagnostics/vsp_realtime_ws_live_probe.gd
  ) > "$WORKDIR/logs/ws_probe_stdout.log" 2>&1
  WS_EXIT=$?
  set -e
  cp "$WORKDIR/logs/ws_probe_stdout.log" "$WORKDIR/http/ws_probe_stdout.log" 2>/dev/null || true
  if [[ "$WS_EXIT" -ne 0 ]]; then
    tail -40 "$WORKDIR/logs/ws_probe_stdout.log" >&2
    fail_functional "ws live probe exited $WS_EXIT (see $WORKDIR/logs/ws_probe_stdout.log)"
  fi
  python3 - "$WORKDIR/logs/ws_probe_stdout.log" <<'PY' || fail_functional "ws live probe exit 0 but summary JSON missing/invalid (see $WORKDIR/logs/ws_probe_stdout.log)"
import json, sys
lines = [l for l in open(sys.argv[1], encoding="utf-8", errors="replace") if l.strip().startswith("{")]
if not lines:
    raise SystemExit(1)
d = json.loads(lines[-1])
if d.get("status") != "ok":
    raise SystemExit(1)
ws = d.get("websocket", {})
if str(ws.get("state", "")) != "open":
    raise SystemExit(1)
if str(d.get("subscription_transport", "")) != "vsp.hub.websocket":
    raise SystemExit(1)
PY
  gate ws_consumption_probe true
  WS_PROBE_SUMMARY="pass"
  log "ws consumption probe PASS (adapter subscribed over hub websocket; see logs/ws_probe_stdout.log)"
fi

# ---------------------------------------------------------------- hub HTTP asset smoke (ps1 step, untouched py)
ASSET_SUMMARY="skipped"
if [[ "$SKIP_HUB_HTTP_ASSET_SMOKE" -eq 0 ]]; then
  ASSET_PY="$REPO_ROOT/scripts/vsp_phase4_hub_http_asset_smoke.py"
  [[ -f "$ASSET_PY" ]] || fail_env "asset smoke py not found: $ASSET_PY"
  ASSET_TIMEOUT_SEC=60
  ASSET_TIMEOUT_MS=60000
  log "hub HTTP asset smoke (untouched py) against the live hub..."
  set +e
  python3 "$ASSET_PY" \
    --hub-url "$HUB_VSP_URL" \
    --timeout-ms "$ASSET_TIMEOUT_MS" \
    --timeout-sec "$ASSET_TIMEOUT_SEC" \
    > "$WORKDIR/http/asset_smoke_stdout.log" 2> "$WORKDIR/http/asset_smoke_stderr.log"
  ASSET_EXIT=$?
  set -e
  if [[ "$ASSET_EXIT" -ne 0 ]]; then
    tail -40 "$WORKDIR/http/asset_smoke_stderr.log" >&2 || true
    tail -20 "$WORKDIR/http/asset_smoke_stdout.log" >&2 || true
    fail_functional "vsp_phase4_hub_http_asset_smoke.py exited $ASSET_EXIT (logs: $WORKDIR/http/asset_smoke_{stdout,stderr}.log)"
  fi
  python3 - "$WORKDIR/http/asset_smoke_stdout.log" "$WORKDIR/http/asset_smoke.json" <<'PY' \
    || fail_functional "asset smoke exited 0 but its stdout summary was missing/invalid (see $WORKDIR/http/asset_smoke_stdout.log)"
import json, sys
raw = open(sys.argv[1], encoding="utf-8", errors="replace").read().strip()
if not raw:
    raise SystemExit(1)
d = json.loads(raw)
if d.get("schema_version") != "vsp_phase4_hub_http_asset_smoke.v1":
    raise SystemExit(1)
if not (d.get("checks") or []):
    raise SystemExit(1)
json.dump(d, open(sys.argv[2], "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
  gate hub_http_asset_smoke true
  ASSET_SUMMARY="pass"
  log "hub HTTP asset smoke PASS ($(python3 -c 'import json; d=json.load(open("'"$WORKDIR"'/http/asset_smoke.json")); print(len(d["checks"]), "checks")'))"
fi

# ---------------------------------------------------------------- stop stack
STOP_RECORD=""
stop_process() {
  local pid="$1"
  local start elapsed code signal="SIGTERM"
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
AGENT_PID=""

log "stopping hub (pid $HUB_PID)..."
stop_process "$HUB_PID"
HUB_STOP_RECORD="$STOP_RECORD"
log "hub stop: $HUB_STOP_RECORD"
HUB_PID=""

log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "kernel stop: $KERNEL_STOP_RECORD (signal:exit_code:elapsed)"
KERNEL_PID=""

sleep 1
lsof -nP -iTCP:"$KERNEL_PORT_REQ" -iTCP:"$ZMQ_PUB_PORT" -iTCP:"$ZMQ_LOG_PORT" -iTCP:"$AGENT_HTTP_PORT" -iTCP:"$HUB_HTTP_PORT" -sTCP:LISTEN \
  > "$WORKDIR/lsof_after_stop.txt" 2>&1 || true
if [[ -s "$WORKDIR/lsof_after_stop.txt" ]]; then
  cat "$WORKDIR/lsof_after_stop.txt" >&2
  fail_functional "some stack port still has a listener after teardown (see $WORKDIR/lsof_after_stop.txt)"
fi
gate stack_stopped_clean true

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
python3 - "$SUMMARY_FILE" "$RUN_ID" "$WORKDIR" \
  "$KERNEL_STOP_RECORD" "$HUB_STOP_RECORD" "$AGENT_STOP_RECORD" \
  "$GATES_FILE" "$WS_PROBE_SUMMARY" "$ASSET_SUMMARY" <<'PY'
import json, os, sys

(out, run_id, workdir, kernel_stop, hub_stop, agent_stop,
 gates_path, ws_probe, asset_smoke) = sys.argv[1:10]

gates = {}
for line in open(gates_path, encoding="utf-8"):
    line = line.strip()
    if "=" in line:
        k, v = line.split("=", 1)
        gates[k] = (v == "true")

binary_evidence = {}
for line in open(os.path.join(workdir, "binary_evidence.txt"), encoding="utf-8"):
    if "=" in line:
        k, v = line.strip().split("=", 1)
        binary_evidence[k] = v

ownership = [l.strip() for l in open(os.path.join(workdir, "ownership.txt"), encoding="utf-8") if l.strip()]

def load_json(relpath):
    path = os.path.join(workdir, relpath)
    if os.path.exists(path):
        return json.load(open(path, encoding="utf-8"))
    return None

summary = {
    "schema_version": "vsp.hub.lifecycle.smoke.mac.v1",
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "gates": gates,
    "binary_evidence": binary_evidence,
    "ownership": ownership,
    "hub_health": load_json("hub/health_verify.json"),
    "hub_status": load_json("hub/status_verify.json"),
    "ws_probe": ws_probe,
    "asset_smoke": asset_smoke,
    "stops": {
        "kernel": kernel_stop,
        "hub": hub_stop,
        "agent": agent_stop,
    },
    "artifacts_dir": workdir,
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY

cat "$SUMMARY_FILE"
[[ -n "$OUTPUT_PATH" ]] && { mkdir -p "$(dirname "$OUTPUT_PATH")"; cp "$SUMMARY_FILE" "$OUTPUT_PATH"; }
log "workdir: $WORKDIR"

python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
sys.exit(0 if all(d["gates"].values()) else 1)' "$SUMMARY_FILE" || exit 1
exit 0
