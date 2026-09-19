#!/usr/bin/env bash
# VSP Hub three-piece stack smoke (kernel + vsphub + agent) — PORT-VSPHUB-1.
#
# Extends the PORT-A5 mac harness pattern (scripts/dev_agent_smoke_mac.sh) with
# the vsphub runtime in the middle, per the PORT-VSPHUB-1 card:
#
#   ① build/place: vsphub built from agent/cmd/vsphub (arm64) into the run
#      workdir; sha256 recorded (the product-path copy at <repo>/agent/bin/
#      vsphub is the start_page autostart candidate and is NOT written by this
#      script — pass --hub-bin to smoke the placed product-path binary).
#   ② three-piece stack: kernel under a fake VitApp root (A5 pattern, source
#      tree never touched, AGENTS §10) + vsphub (127.0.0.1:8787, kernel ZMQ
#      5555/5556 defaults) + agent with -vsp-hub-url + VIT_AGENT_VSP_HUB_REQUIRED
#      =true; every port's listener pid asserted against the owning process.
#      With --external-kernel: no kernel is built/started/stopped by this run —
#      the hub and the agent connect to an ALREADY-RUNNING kernel on 5555/5556
#      (single-owner rule, AGENTS §9: the running stack is observed and
#      recorded, never touched); the kernel gate becomes an observation of the
#      external owner. Use --agent-http/--udp-* to avoid the running stack's
#      agent ports in that mode.
#   ③ registration: agent starts BEFORE the hub, so bounded-retry "pending"
#      log lines are captured, then the hub comes up and registration must
#      succeed (agent log line + hub /vsp/status session role=agent
#      client_id=vit.agent.official). A separate negative pre-phase starts an
#      agent with required=true and NO hub and asserts it self-stops after the
#      documented 30x2s bound with the "required VSP Hub registration failed"
#      log line (bounded-retry semantics, both sides).
#   ④ read-only VSP roundtrip: bash/python port of codex_vsp_readonly_probe
#      .ps1 (health, capability advertisement, session.hello role=extension
#      read_only, extension.register, /vsp/status listing, state.snapshot
#      through the hub to the kernel, event.poll, unregister, status cleanup).
#   ⑤ optional --ws-probe: drives the frontend repo's Godot headless probe
#      (tools/diagnostics/vsp_realtime_ws_live_probe.gd) against the live hub
#      with VIT_GUI_VSP_REALTIME_WS_ENABLE=1 (the fix5 强开 switch) — the
#      adapter must subscribe over the hub websocket and receive realtime
#      frames with zero HTTP fallback.
#
# Exit code 0 requires every enabled gate to pass. Failures are classified
# env (ports busy, build/toolchain) vs functional (replies, registration,
# probe checks) in the error line (AGENTS §8). All artifacts land under the
# run workdir (run ID, logs, per-probe JSON, lsof evidence, summary.json),
# printed to stderr.

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
HUB_HTTP="http://127.0.0.1:8787"
KERNEL_BIN_ARG=""
SKIP_BUILDS=0
AGENT_BIN_ARG=""
HUB_BIN_ARG=""
WORKDIR_ARG=""
OUTPUT_PATH=""
TRACKTION_DIR=""
SKIP_BOUNDED_FAIL=0
WS_PROBE=0
EXTERNAL_KERNEL=0
UDP_TO_GODOT=4444
UDP_FROM_GODOT=4445
GODOT_BIN="/Users/timozty/Applications/Godot.app/Contents/MacOS/Godot"
FRONTEND_ROOT="$HOME/Documents/vit-daw-frontend"
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
BOUNDED_FAIL_TIMEOUT_SECONDS=120

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555

usage() {
  cat <<'EOF'
vsp_hub_three_piece_smoke_mac.sh — kernel + vsphub + agent stack smoke (PORT-VSPHUB-1).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --hub-http URL          VSP Hub HTTP base (default http://127.0.0.1:8787)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-builds           With --agent-bin/--hub-bin: do not build either
  --agent-bin PATH        Agent binary to start (only with --skip-builds)
  --hub-bin PATH          vsphub binary to start (only with --skip-builds);
                          pass <repo>/agent/bin/vsphub to smoke the placed
                          product-path binary
  --skip-bounded-fail     Skip the negative required-mode bounded-fail
                          pre-phase (~70s: agent with required=true and no
                          hub must self-stop after the 30x2s bound)
  --external-kernel       Do not build/start/stop a kernel: connect the hub
                          and agent to an already-running kernel on
                          5555/5556 (observed + recorded, never touched);
                          typically with --agent-http/--udp-to-godot/
                          --udp-from-godot on alternate ports
  --udp-to-godot N        Agent telemetry UDP destination port (default 4444)
  --udp-from-godot N      Agent command UDP listen port (default 4445)
  --ws-probe              Also drive the frontend Godot ws live probe against
                          the live hub (fix5 强开 verification)
  --godot-bin PATH        Godot binary for --ws-probe
  --frontend-root PATH    Frontend repo root for --ws-probe
  --startup-timeout SECONDS  Kernel/agent/hub startup timeout (default 90)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --workdir PATH          Reuse PATH as the run workdir (never overwrites)
  --tracktion-dir PATH    tracktion_engine source dir (default: the repo's
                          submodule checkout)
  --output PATH           Also write the summary JSON to PATH
  -h, --help              Show this help
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
    --skip-bounded-fail) SKIP_BOUNDED_FAIL=1; shift ;;
    --external-kernel) EXTERNAL_KERNEL=1; shift ;;
    --udp-to-godot) UDP_TO_GODOT="$2"; shift 2 ;;
    --udp-from-godot) UDP_FROM_GODOT="$2"; shift 2 ;;
    --ws-probe) WS_PROBE=1; shift ;;
    --godot-bin) GODOT_BIN="$2"; shift 2 ;;
    --frontend-root) FRONTEND_ROOT="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
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

for tool in go cmake make python3 lsof shasum curl; do
  command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
done

RUN_ID="vsp_hub_three_piece_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/vsp_hub_three_piece_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{build,bin,http,logs,probe,hub}
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
BOUNDED_EXIT_SENTINEL="skipped"
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

AGENT_HTTP_ADDR="${AGENT_HTTP#http://}"
AGENT_HTTP_ADDR="${AGENT_HTTP_ADDR#https://}"
AGENT_HTTP_PORT="${AGENT_HTTP_ADDR##*:}"
HUB_HTTP_ADDR="${HUB_HTTP#http://}"
HUB_HTTP_ADDR="${HUB_HTTP_ADDR#https://}"
HUB_HTTP_PORT="${HUB_HTTP_ADDR##*:}"
HUB_VSP_URL="${HUB_HTTP%/}/vsp"
HUB_STATUS_URL="${HUB_HTTP%/}/vsp/status"

assert_ports_free() {
  local port
  local kernel_ports=("$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT")
  if [[ "$EXTERNAL_KERNEL" -eq 1 ]]; then
    # The external kernel MUST be up on the fixed ZMQ ports; only the ports
    # this run owns (agent HTTP/UDP, hub HTTP) must be free.
    for port in "${kernel_ports[@]}"; do
      local pid
      pid="$(port_listener_pid "$port")"
      [[ -n "$pid" ]] || fail_env "--external-kernel: no listener on kernel port $port (no kernel running?)"
    done
    for port in "$AGENT_HTTP_PORT" "$HUB_HTTP_PORT"; do
      local pid
      pid="$(port_listener_pid "$port")"
      if [[ -n "$pid" ]]; then
        fail_env "port $port already has a listener (pid $pid); this harness never stops processes it did not start (AGENTS §9 single-owner rule)"
      fi
    done
    return
  fi
  for port in "${kernel_ports[@]}" "$AGENT_HTTP_PORT" "$HUB_HTTP_PORT"; do
    local pid
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
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "hub_http=$HUB_HTTP"
  echo "ws_probe=$WS_PROBE"
  echo "external_kernel=$EXTERNAL_KERNEL"
  echo "udp_to_godot=$UDP_TO_GODOT udp_from_godot=$UDP_FROM_GODOT"
} > "$WORKDIR/run_meta.txt"

# ---------------------------------------------------------------- builds
KERNEL_BIN=""
EXTERNAL_KERNEL_PID=""
EXTERNAL_KERNEL_PATH=""
if [[ "$EXTERNAL_KERNEL" -eq 1 ]]; then
  log "--external-kernel: no kernel build/start/stop in this run (already-running kernel is observed only)"
elif [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "using provided kernel binary: $KERNEL_BIN (sha256 recorded)"
else
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
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "kernel built: $KERNEL_BIN"
fi

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
log "binaries: agent=$(shasum -a 256 "$AGENT_BIN" | cut -c1-16)... hub=$(shasum -a 256 "$HUB_BIN" | cut -c1-16)..."

# ------------------------------------------- phase A: required-mode bounded fail
# Negative pre-phase (skip with --skip-bounded-fail): required=true and NO hub
# anywhere → the agent must keep retrying (pending log lines) and then stop
# itself at the documented bound (30 attempts x 2s, cmd/vitagent/main.go).
if [[ "$SKIP_BOUNDED_FAIL" -eq 0 ]]; then
  log "phase A: required-mode bounded-fail (no hub; expect self-stop after ~30x2s)..."
  BOUNDED_STATE_DIR="$WORKDIR/agent_state_bounded"
  mkdir -p "$BOUNDED_STATE_DIR"
  (
    cd "$WORKDIR"
    VIT_AGENT_VSP_HUB_REQUIRED=true \
    VIT_ORCHESTRATION_STORE_PATH="$BOUNDED_STATE_DIR/orchestration_v1.json" \
    exec "$AGENT_BIN" \
      -http "$AGENT_HTTP_ADDR" \
      -last-log-path "$BOUNDED_STATE_DIR/agent_last.log" \
      -udp-to-godot "$UDP_TO_GODOT" \
      -udp-from-godot "$UDP_FROM_GODOT" \
      -vsp-hub-url "$HUB_VSP_URL"
  ) > "$WORKDIR/logs/agent_bounded_stdout.log" 2>&1 &
  BOUNDED_AGENT_PID=$!
  log "bounded-fail agent pid=$BOUNDED_AGENT_PID"
  bounded_start=$SECONDS
  bounded_exit=""
  while (( SECONDS - bounded_start < BOUNDED_FAIL_TIMEOUT_SECONDS )); do
    if ! kill -0 "$BOUNDED_AGENT_PID" 2>/dev/null; then
      set +e; wait "$BOUNDED_AGENT_PID" 2>/dev/null; bounded_exit=$?; set -e
      break
    fi
    sleep 2
  done
  if [[ -z "$bounded_exit" ]]; then
    kill -TERM "$BOUNDED_AGENT_PID" 2>/dev/null || true
    sleep 2
    kill -0 "$BOUNDED_AGENT_PID" 2>/dev/null && kill -KILL "$BOUNDED_AGENT_PID" 2>/dev/null || true
    wait "$BOUNDED_AGENT_PID" 2>/dev/null || true
    fail_functional "required-mode agent did NOT self-stop within ${BOUNDED_FAIL_TIMEOUT_SECONDS}s with no hub (bounded retry broken?)"
  fi
  echo "bounded_fail_exit_code=$bounded_exit elapsed=$((SECONDS - bounded_start))s" >> "$WORKDIR/run_meta.txt"
  BOUNDED_LOG="$BOUNDED_STATE_DIR/agent_last.log"
  grep -c "VSP Hub registration pending" "$BOUNDED_LOG" > "$WORKDIR/bounded_pending_count.txt" 2>/dev/null || echo 0 > "$WORKDIR/bounded_pending_count.txt"
  grep "required VSP Hub registration failed" "$BOUNDED_LOG" > "$WORKDIR/bounded_fail_line.txt" 2>/dev/null || true
  PENDING_COUNT="$(cat "$WORKDIR/bounded_pending_count.txt")"
  [[ "$PENDING_COUNT" -ge 1 ]] \
    || fail_functional "bounded-fail agent log shows no retry 'pending' lines (see $BOUNDED_LOG)"
  [[ -s "$WORKDIR/bounded_fail_line.txt" ]] \
    || fail_functional "bounded-fail agent stopped without the 'required VSP Hub registration failed' log line (see $BOUNDED_LOG)"
  log "phase A PASS: self-stopped after $((SECONDS - bounded_start))s (exit $bounded_exit), $PENDING_COUNT pending line(s), bound line: $(cat "$WORKDIR/bounded_fail_line.txt")"
  gate bounded_fail_self_stop true
  BOUNDED_EXIT_SENTINEL="self_stop:exit=$bounded_exit:elapsed=$((SECONDS - bounded_start))s:pending=$PENDING_COUNT"
  # Wait for the HTTP port to actually drain before the next agent instance.
  for i in $(seq 1 15); do
    [[ -z "$(port_listener_pid "$AGENT_HTTP_PORT")" ]] && break
    sleep 1
  done
  [[ -z "$(port_listener_pid "$AGENT_HTTP_PORT")" ]] \
    || fail_env "bounded-fail agent HTTP port $AGENT_HTTP_PORT still occupied after exit"
fi

# ---------------------------------------------------------------- start kernel
if [[ "$EXTERNAL_KERNEL" -eq 1 ]]; then
  EXTERNAL_KERNEL_PID="$(port_listener_pid "$KERNEL_PORT_REQ")"
  EXTERNAL_KERNEL_PATH="$(lsof -nP -p "$EXTERNAL_KERNEL_PID" 2>/dev/null | awk '$4=="txt" && $9!="" {print $9; exit}')"
  [[ -n "$EXTERNAL_KERNEL_PATH" ]] || EXTERNAL_KERNEL_PATH="(path unavailable)"
  {
    echo "external_kernel_pid=$EXTERNAL_KERNEL_PID"
    echo "external_kernel_path=$EXTERNAL_KERNEL_PATH"
    echo "external_kernel_started=$(ps -o lstart= -p "$EXTERNAL_KERNEL_PID" 2>/dev/null || echo unknown)"
    lsof -nP -iTCP:"$KERNEL_PORT_REQ" -iTCP:"$ZMQ_PUB_PORT" -iTCP:"$ZMQ_LOG_PORT" -sTCP:LISTEN 2>/dev/null || true
  } > "$WORKDIR/external_kernel_evidence.txt"
  if [[ -x "$EXTERNAL_KERNEL_PATH" ]]; then
    shasum -a 256 "$EXTERNAL_KERNEL_PATH" > "$WORKDIR/kernel_bin.sha256"
  fi
  gate external_kernel_observed true
  log "external kernel observed: pid=$EXTERNAL_KERNEL_PID path=$EXTERNAL_KERNEL_PATH (never touched by this run)"
else
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
fi

# ------------------------------------------------ start agent BEFORE hub (retry evidence)
# required=true with the hub URL set but the hub not yet up: the registration
# loop must log retryable 'pending' lines; the hub is started only after at
# least one pending line is on disk, evidencing retry-then-success.
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

# Capture at least one retry-pending line before the hub appears.
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

# ---------------------------------------------------------------- start hub
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
log "hub /vsp/status lists agent session (evidence: hub/status_with_agent.json)"

# ---------------------------------------------------------------- read-only VSP probe
# Python port of codex_vsp_readonly_probe.ps1 (same envelope shapes, same
# checks; extension role, read_only, unregister in finally).
log "read-only VSP roundtrip through the hub (codex probe semantics)..."
python3 - "$HUB_VSP_URL" "$HUB_STATUS_URL" "$HUB_HTTP/health" "$WORKDIR/http/vsp_readonly_probe.json" <<'PY' \
  || fail_functional "read-only VSP probe failed (see $WORKDIR/http/vsp_readonly_probe.json and prior log lines)"
import json, sys, time, urllib.request, urllib.error, uuid

hub_url, status_url, health_url, out_path = sys.argv[1:5]
KERNEL_INTERNAL_TIMELINE_TRACK_IDS = {"1002", "1003", "1004", "1005", "1006"}
client_id = "vsphub1.mac.readonly.probe"
client_version = "vsphub1-readonly-probe-v1"

def fail(msg):
    print("PROBE_FAIL: " + msg, file=sys.stderr)
    raise SystemExit(1)

def get(url):
    with urllib.request.urlopen(url, timeout=20) as r:
        return json.loads(r.read().decode("utf-8"))

def post(env):
    body = json.dumps(env).encode("utf-8")
    req = urllib.request.Request(hub_url, data=body, method="POST",
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return r.status, json.loads(r.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            parsed = json.loads(raw)
        except Exception:
            parsed = {"raw": raw}
        return e.code, parsed

def envelope(session_id, channel, msg_type, payload, role="extension"):
    return {
        "vsp_version": "1.0",
        "schema": "vsp.%s.v1" % msg_type,
        "message_id": "msg_vsphub1_probe_" + uuid.uuid4().hex,
        "session_id": session_id,
        "client_id": client_id,
        "role": role,
        "channel": channel,
        "type": msg_type,
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "trace_id": "trace_vsphub1_probe_" + uuid.uuid4().hex,
        "payload": payload,
    }

def require_reply(reply, status, expected, label):
    if status != 200:
        fail("%s returned HTTP %s: %s" % (label, status, json.dumps(reply)[:400]))
    if reply.get("type") not in expected:
        fail("%s returned unexpected type %s: %s" % (label, reply.get("type"), json.dumps(reply)[:400]))

checks = []
session_id = "sess_vsphub1_probe_%d" % int(time.time() * 1000)
registered = False

health = get(health_url)
if health.get("status") != "ok" or health.get("service") != "VspHub":
    fail("health endpoint was not ok: " + json.dumps(health)[:400])
checks.append("hub_health_ok")

status = get(status_url)
ext_caps = set(status.get("capabilities", {}).get("extension", []))
if "state.snapshot" not in ext_caps or "event.poll" not in ext_caps:
    fail("extension role does not advertise state.snapshot+event.poll")
if "command.request" in ext_caps:
    fail("extension role unexpectedly advertises command.request")
checks.append("status_advertises_readonly_extension_caps")

try:
    hello = envelope("session_pending", "session", "session.hello", {
        "client_name": "VSPHUB1 Mac Read-only Probe",
        "client_version": client_version,
        "protocol_min": "1.0",
        "protocol_max": "1.0",
        "wants": ["state.snapshot", "event.poll"],
        "transport_bindings": ["vsp.hub.http"],
        "read_only": True,
    })
    status_code, reply = post(hello)
    require_reply(reply, status_code, ["session.hello_ack"], "session.hello")
    session_id = str(reply.get("session_id", ""))
    if not session_id.strip():
        fail("session.hello did not return a session_id")
    checks.append("session_hello_ack")

    reg = envelope(session_id, "extension", "extension.register", {
        "name": "VSPHUB1 Mac Read-only Probe",
        "version": client_version,
        "wants": ["state.snapshot", "event.poll"],
        "transport_bindings": ["vsp.hub.http"],
        "read_only": True,
    })
    status_code, reply = post(reg)
    require_reply(reply, status_code, ["extension.register_ack"], "extension.register")
    registered = True
    checks.append("extension_register_ack")

    status = get(status_url)
    listed = any(str(s.get("session_id")) == session_id
                 and str(s.get("client_id")) == client_id
                 and str(s.get("role")) == "extension"
                 for s in status.get("sessions", []))
    if not listed:
        fail("probe extension session was not visible in /vsp/status")
    checks.append("status_lists_probe_session")

    snap = envelope(session_id, "state", "state.snapshot_request", {"scope": "project.timeline"})
    status_code, reply = post(snap)
    require_reply(reply, status_code, ["state.snapshot"], "state.snapshot_request")
    payload = reply.get("payload", {})
    if payload.get("status") != "ok":
        fail("state.snapshot payload was not ok: " + json.dumps(reply)[:400])
    track_ids = []
    for row in payload.get("tracks", []) or []:
        tid = str(row.get("track_id", row.get("id", "")))
        if tid.strip():
            track_ids.append(tid.strip())
    leaked = sorted(set(track_ids) & KERNEL_INTERNAL_TIMELINE_TRACK_IDS)
    if leaked:
        fail("state.snapshot project.timeline leaked internal track ids: " + ",".join(leaked))
    checks.append("state_snapshot_hides_internal_tracks")
    checks.append("state_snapshot_read")

    poll = envelope(session_id, "event", "event.poll", {
        "topics": ["project", "import", "bake", "render"],
    })
    status_code, reply = post(poll)
    require_reply(reply, status_code, ["event.progress", "event.notification"], "event.poll")
    if not str(reply.get("payload", {}).get("event_id", "")).strip():
        fail("event.poll did not return an event_id: " + json.dumps(reply)[:400])
    checks.append("event_poll_read")
finally:
    if registered:
        unreg = envelope(session_id, "extension", "extension.unregister", {})
        status_code, reply = post(unreg)
        if status_code == 200 and reply.get("type") == "extension.unregister_ack":
            checks.append("extension_unregister_ack")

status = get(status_url)
still = any(str(s.get("session_id")) == session_id for s in status.get("sessions", []))
if still:
    fail("probe session remained visible after unregister")
checks.append("status_removes_probe_session")

summary = {
    "status": "ok",
    "schema_version": "vsphub1.mac.readonly.probe.v1",
    "hub_url": hub_url,
    "client_id": client_id,
    "role": "extension",
    "session_id": session_id,
    "read_only": True,
    "mutation_channels_sent": [],
    "checks": checks,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print("PROBE_OK checks=" + ",".join(checks))
PY
gate vsp_readonly_probe true
log "read-only VSP probe PASS ($(python3 -c 'import json; d=json.load(open("'"$WORKDIR"'/http/vsp_readonly_probe.json")); print(len(d["checks"]), "checks")'))"

# ---------------------------------------------------------------- optional ws probe
WS_PROBE_SUMMARY="skipped"
if [[ "$WS_PROBE" -eq 1 ]]; then
  [[ -x "$GODOT_BIN" ]] || fail_env "--ws-probe: Godot binary not executable: $GODOT_BIN"
  [[ -f "$FRONTEND_ROOT/project.godot" ]] || fail_env "--ws-probe: frontend project not found: $FRONTEND_ROOT"
  [[ -f "$FRONTEND_ROOT/tools/diagnostics/vsp_realtime_ws_live_probe.gd" ]] \
    || fail_env "--ws-probe: frontend ws live probe script not found under $FRONTEND_ROOT"
  log "ws probe: Godot headless vsp_realtime_ws_live_probe against the live hub (ENABLE=1 强开)..."
  # The stack (hub) is live; the probe instantiates the real adapter which
  # connects to ws://127.0.0.1:8787/vsp/stream. fix5's mac default-off must be
  # overridden via VIT_GUI_VSP_REALTIME_WS_ENABLE=1 in the launching shell.
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
  gate ws_probe true
  WS_PROBE_SUMMARY="pass"
  log "ws probe PASS (adapter subscribed over hub websocket; see logs/ws_probe_stdout.log)"
fi

# ---------------------------------------------------------------- stop stack
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
AGENT_PID=""

log "stopping hub (pid $HUB_PID)..."
stop_process "$HUB_PID"
HUB_STOP_RECORD="$STOP_RECORD"
log "hub stop: $HUB_STOP_RECORD"
HUB_PID=""

if [[ "$EXTERNAL_KERNEL" -eq 1 ]]; then
  # The external kernel was never ours: assert it is still alive and still
  # owning its ports after our teardown (untouched evidence, AGENTS §9).
  kill -0 "$EXTERNAL_KERNEL_PID" 2>/dev/null \
    || fail_functional "external kernel pid $EXTERNAL_KERNEL_PID died during this run (not started by this harness — investigate before rerunning)"
  [[ "$(port_listener_pid "$KERNEL_PORT_REQ")" == "$EXTERNAL_KERNEL_PID" ]] \
    || fail_functional "external kernel no longer owns port $KERNEL_PORT_REQ after teardown"
  log "external kernel untouched after teardown (pid $EXTERNAL_KERNEL_PID still owns $KERNEL_PORT_REQ)"
  gate external_kernel_untouched true
else
log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "kernel stop: $KERNEL_STOP_RECORD (signal:exit_code:elapsed)"
KERNEL_PID=""
fi

# Port-drain evidence: nothing we started is still listening.
sleep 1
if [[ "$EXTERNAL_KERNEL" -eq 1 ]]; then
  lsof -nP -iTCP:"$AGENT_HTTP_PORT" -iTCP:"$HUB_HTTP_PORT" -sTCP:LISTEN \
    > "$WORKDIR/lsof_after_stop.txt" 2>&1 || true
else
  lsof -nP -iTCP:"$KERNEL_PORT_REQ" -iTCP:"$ZMQ_PUB_PORT" -iTCP:"$ZMQ_LOG_PORT" -iTCP:"$AGENT_HTTP_PORT" -iTCP:"$HUB_HTTP_PORT" -sTCP:LISTEN \
    > "$WORKDIR/lsof_after_stop.txt" 2>&1 || true
fi
if [[ -s "$WORKDIR/lsof_after_stop.txt" ]]; then
  cat "$WORKDIR/lsof_after_stop.txt" >&2
  fail_functional "some stack port still has a listener after teardown (see $WORKDIR/lsof_after_stop.txt)"
fi
gate stack_stopped_clean true

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
python3 - "$SUMMARY_FILE" "$RUN_ID" "$WORKDIR" \
  "$KERNEL_STOP_RECORD" "$HUB_STOP_RECORD" "$AGENT_STOP_RECORD" \
  "$BOUNDED_EXIT_SENTINEL" "$WS_PROBE_SUMMARY" "$GATES_FILE" <<'PY'
import json, os, sys

(out, run_id, workdir, kernel_stop, hub_stop, agent_stop,
 bounded, ws_probe, gates_path) = sys.argv[1:10]

gates = {}
for line in open(gates_path, encoding="utf-8"):
    line = line.strip()
    if "=" in line:
        k, v = line.split("=", 1)
        gates[k] = (v == "true")

probe = {}
probe_path = os.path.join(workdir, "http", "vsp_readonly_probe.json")
if os.path.exists(probe_path):
    probe = json.load(open(probe_path, encoding="utf-8"))

summary = {
    "schema_version": "vsp.hub.three_piece.smoke.mac.v1",
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "gates": gates,
    "stops": {
        "kernel": kernel_stop,
        "hub": hub_stop,
        "agent": agent_stop,
    },
    "bounded_fail_prephase": bounded,
    "vsp_readonly_probe": probe,
    "ws_probe": ws_probe,
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
