#!/usr/bin/env bash
# dev_agent_smoke mac equivalent (minimal kernel+agent set) — PORT-A5.
#
# This is the mac minimal subset of scripts/dev_agent_smoke.ps1 covering the
# real two-process stack (VitApp kernel + Go agent; the Godot frontend is out
# of scope for this card):
#
#   | ps1 (dev_agent_smoke.ps1)              | mac (this script)                    |
#   |----------------------------------------|--------------------------------------|
#   | Build VitAgent                          | go build into the run workdir        |
#   | StartKernel (Export/staging VitApp.exe) | cmake+make VitApp into the workdir   |
#   |                                         |   (or --kernel-bin to reuse one)     |
#   | StartUI (Godot/exported UI)             | NOT in this card (kernel+agent only) |
#   | Start or reuse agent                    | start agent, state isolated          |
#   | HTTP state (/agent/state)               | C4 whitelist GET probes x4           |
#   | Kernel ports (5555/5556 listeners)      | lsof listeners 5555/5556/5557        |
#   | Project state invoke                    | POST /agent/invoke project.state     |
#   |                                         | + direct kernel ZMQ ping (5555)      |
#   | (not in ps1)                            | scan_plugins child-process scan +    |
#   |                                         |   plugin list / Waves WaveShell R2   |
#   | (kernel stop not in ps1)                | kernel stop method + exit code +     |
#   |                                         |   POSIX shm residue lsof (A1 input) |
#   | Chat / strip-silence / mix / authority  | NOT in this card (agent-internal     |
#   | smokes                                  |   surfaces; minimal kernel set)      |
#
# HTTP probes reuse the C4 whitelist (g_runtime_readonly_smoke_mac.sh):
# /health, /agent/runtime/status, /agent/state, /agent/events (events adds
# conversation_id, matching the real agent contract). Unlike the C4 harness
# this script is not read-only overall: it legitimately POSTs /agent/invoke
# (project.state) and sends kernel ZMQ commands, so the C4 non-GET
# self-scan guard does not apply; the GET helper below still enforces the
# probe whitelist.
#
# The kernel writes all of its state (Workspace/{Logs,Settings,Cache},
# default_project.xml) under a fake VitApp root inside the run workdir
# (VitPaths.h climbs to a directory containing CMakeLists.txt + Source/),
# so the source tree is never touched (AGENTS §10). The kernel is started
# with VIT_ENABLE_SHARED_MEMORY_TEST=1 so the A1 platform finding (no
# graceful shutdown; POSIX shm segment survives SIGTERM) can be evidenced
# with lsof after the kernel is stopped.
#
# Usage:
#   ./scripts/dev_agent_smoke_mac.sh                     # full run: build kernel+agent, smoke, stop
#   ./scripts/dev_agent_smoke_mac.sh --kernel-bin PATH   # reuse a built kernel (sha256+mtime recorded)
#   ./scripts/dev_agent_smoke_mac.sh --skip-agent-build --agent-bin PATH
#   ./scripts/dev_agent_smoke_mac.sh --workdir PATH      # reuse a run workdir (never overwrites other runs)
#
# Exit code 0 requires every gate to pass: kernel start (ZMQ ports bound),
# agent start (/health), whitelist GETs 2xx, agent->kernel project.state ok,
# direct kernel ZMQ ping ok, plugin scan completed via child-process scanner
# with >= --expected-waves Waves plugins enumerated (R2 first verification).
# Failures are classified env (ports busy, build/toolchain/network) vs
# functional (replies, scan, R2) in the summary (AGENTS §8).
#
# All artifacts land under the run workdir (run ID + kernel/agent logs +
# per-probe JSON + lsof evidence + summary.json), printed to stderr.

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
VST3_DIR="/Library/Audio/Plug-Ins/VST3"
EXPECTED_WAVES=23
SCAN_TIMEOUT_SECONDS=600
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
WORKDIR_ARG=""
OUTPUT_PATH=""
TRACKTION_DIR=""

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555
CONVERSATION_ID="dev-agent-smoke-mac-probe"

ALLOWED_GET_PATHS=(
  "/health"
  "/agent/runtime/status"
  "/agent/state"
  "/agent/events"
)

usage() {
  cat <<'EOF'
dev_agent_smoke_mac.sh — mac minimal kernel+agent smoke (PORT-A5).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-agent-build      With --agent-bin: do not build the agent
  --agent-bin PATH        Agent binary to start (only with --skip-agent-build)
  --vst3-dir PATH         VST3 search path for scan_plugins
                          (default /Library/Audio/Plug-Ins/VST3)
  --expected-waves N      Minimum Waves plugin bodies for the R2 gate (default 23)
  --scan-timeout SECONDS  Plugin scan completion timeout (default 600)
  --startup-timeout SECONDS  Kernel/agent startup timeout (default 90)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --workdir PATH          Reuse PATH as the run workdir instead of a fresh
                          mktemp directory (never overwrites other runs)
  --tracktion-dir PATH    tracktion_engine source dir (default: the repo's
                          submodule checkout; in an isolated git worktree the
                          submodule may be empty — point at any checkout of
                          the same pinned commit)
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
    --expected-waves) EXPECTED_WAVES="$2"; shift 2 ;;
    --scan-timeout) SCAN_TIMEOUT_SECONDS="$2"; shift 2 ;;
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

for tool in go cmake make python3 lsof pgrep shasum; do
  command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
done

RUN_ID="dev_agent_smoke_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/dev_agent_smoke_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{build,bin,http,zmq,logs,probe}
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
  # Reap children so the shell does not hang on exit.
  wait 2>/dev/null || true
  exit $rc
}
trap cleanup EXIT INT TERM

json_field() {
  # json_field <file> <python expr against d> — small JSON extraction helper.
  python3 - "$1" "$2" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
print(eval(sys.argv[2], {"d": d}))
PY
}

port_listener_pid() {
  # lsof exits 1 when no listener matches; with pipefail that must not kill
  # the harness (empty output == "no listener" is the expected free-port case).
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

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

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "vst3_dir=$VST3_DIR"
  echo "expected_waves=$EXPECTED_WAVES"
} > "$WORKDIR/run_meta.txt"

# ---------------------------------------------------------------- kernel build
KERNEL_BIN=""
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  log "using provided kernel binary: $KERNEL_BIN (sha256/mtime recorded)"
else
  if [[ -z "$TRACKTION_DIR" ]]; then
    TRACKTION_DIR="$REPO_ROOT/tracktion_engine"
  fi
  if [[ ! -f "$TRACKTION_DIR/CMakeLists.txt" ]]; then
    fail_env "tracktion_engine not usable at $TRACKTION_DIR (submodule not checked out? pass --tracktion-dir pointing at a checkout of the pinned commit)"
  fi
  log "building VitApp kernel (cmake+make, sources at $REPO_ROOT/VitApp, tracktion at $TRACKTION_DIR)..."
  if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug \
        -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fail_env "kernel cmake configure failed (network-dependent FetchContent? see $WORKDIR/logs/kernel_configure.log)"
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

# ---------------------------------------------------------------- agent build
if [[ "$SKIP_AGENT_BUILD" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" ]] || fail_env "--skip-agent-build requires --agent-bin"
  [[ -x "$AGENT_BIN_ARG" ]] || fail_env "agent binary is not executable: $AGENT_BIN_ARG"
  AGENT_BIN="$AGENT_BIN_ARG"
else
  [[ -z "$AGENT_BIN_ARG" ]] || fail_env "--agent-bin is only valid together with --skip-agent-build"
  log "building agent (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$WORKDIR/bin/vitagent" ./cmd/vitagent) \
    || fail_env "go build agent failed"
  AGENT_BIN="$WORKDIR/bin/vitagent"
fi
shasum "$AGENT_BIN" > "$WORKDIR/agent_bin.sha1" || true

# ---------------------------------------------------------------- zmq probe
# Throwaway REQ client (built only in the run workdir; never committed). Uses
# the same pure-Go zmq library as the agent (go-zeromq/zmq4). The go.mod
# mirrors the agent's require set for zmq4's deps and the go.sum is copied
# from the agent module, so the probe builds offline from the local module
# cache (no network, no tidy).
cat > "$WORKDIR/probe/go.mod" <<'EOF'
module zmqprobe

go 1.24.1

require (
	github.com/go-zeromq/zmq4 v0.17.0
	github.com/go-zeromq/goczmq/v4 v4.2.2 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/text v0.15.0 // indirect
)
EOF
cat > "$WORKDIR/probe/main.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-zeromq/zmq4"
)

// zmqprobe <endpoint> <timeout-ms> <json-payload>: one-shot REQ round-trip.
// Prints the reply payload on stdout; non-zero exit on transport failure.
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: zmqprobe <endpoint> <timeout-ms> <json>")
		os.Exit(2)
	}
	timeoutMs, err := strconv.Atoi(os.Args[2])
	if err != nil || timeoutMs <= 0 {
		fmt.Fprintln(os.Stderr, "invalid timeout:", os.Args[2])
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	sock := zmq4.NewReq(ctx, zmq4.WithTimeout(time.Duration(timeoutMs)*time.Millisecond),
		zmq4.WithDialerTimeout(5*time.Second), zmq4.WithDialerMaxRetries(1))
	defer sock.Close()
	if err := sock.Dial(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "dial:", err)
		os.Exit(1)
	}
	if err := sock.Send(zmq4.NewMsgString(os.Args[3])); err != nil {
		fmt.Fprintln(os.Stderr, "send:", err)
		os.Exit(1)
	}
	reply, err := sock.Recv()
	if err != nil {
		fmt.Fprintln(os.Stderr, "recv:", err)
		os.Exit(1)
	}
	payload := ""
	if len(reply.Frames) > 0 {
		payload = string(reply.Frames[len(reply.Frames)-1])
	}
	fmt.Println(payload)
}
EOF
if ! cp "$REPO_ROOT/agent/go.sum" "$WORKDIR/probe/go.sum" \
   || ! (cd "$WORKDIR/probe" && GOPROXY=off GOSUMDB=off go build -o "$WORKDIR/bin/zmqprobe" . > "$WORKDIR/logs/probe_build.log" 2>&1); then
  tail -10 "$WORKDIR/logs/probe_build.log" >&2 || true
  fail_env "zmq probe build failed (module cache incomplete for the zmq4 dep set?)"
fi
zmq_send() {
  # zmq_send <timeout-ms> <json> — reply appended to $WORKDIR/zmq/replies.jsonl
  local timeout_ms="$1" payload="$2" reply
  if ! reply="$("$WORKDIR/bin/zmqprobe" "$ZMQ_REQ_ENDPOINT" "$timeout_ms" "$payload")"; then
    return 1
  fi
  printf '%s\n' "$reply" >> "$WORKDIR/zmq/replies.jsonl"
  printf '%s' "$reply"
}

# ---------------------------------------------------------------- start kernel
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
log "kernel ZMQ ports listening: $KERNEL_PORT_REQ (REQ) $ZMQ_PUB_PORT (PUB) $ZMQ_LOG_PORT (log) — owned by pid $KERNEL_PID"

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

# ---------------------------------------------------------------- HTTP probes
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

for path in "/health" "/agent/runtime/status" "/agent/state" "/agent/events"; do
  out="$WORKDIR/http/${path//\//_}.json"
  readonly_get "$path" "$out"
  log "GET $path -> 2xx ($(wc -c < "$out" | tr -d ' ') bytes)"
done
port_listener_pid "$AGENT_HTTP_PORT" | grep -qx "$AGENT_PID" \
  || fail_functional "agent HTTP port owner is not the agent pid"

# ------------------------------------------------------- ZMQ command surface
log "direct kernel ZMQ ping..."
PING_REPLY="$(zmq_send 10000 '{"cmd":"ping"}')" \
  || fail_functional "kernel ZMQ ping transport failed (endpoint $ZMQ_REQ_ENDPOINT)"
printf '%s\n' "$PING_REPLY" > "$WORKDIR/zmq/ping_reply.json"
[[ "$(json_field "$WORKDIR/zmq/ping_reply.json" 'str(d.get("status",""))')" == "ok" ]] \
  || fail_functional "kernel ZMQ ping reply not ok: $PING_REPLY"
log "kernel ZMQ ping -> $(json_field "$WORKDIR/zmq/ping_reply.json" 'd.get("message","")')"

log "agent->kernel ZMQ round-trip (POST /agent/invoke project.state)..."
INVOKE_BODY='{"tool":"project.state","args":{},"source":"dev_agent_smoke_mac"}'
HTTP_CODE="$(curl -sS --max-time 120 -o "$WORKDIR/http/invoke_project_state.json" \
  -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data "$INVOKE_BODY" \
  "$AGENT_HTTP/agent/invoke")" \
  || fail_functional "POST /agent/invoke project.state request failed"
[[ "$HTTP_CODE" =~ ^2 ]] || fail_functional "POST /agent/invoke project.state returned HTTP $HTTP_CODE"
[[ "$(json_field "$WORKDIR/http/invoke_project_state.json" 'str(d.get("status",""))')" == "ok" ]] \
  || fail_functional "project.state via agent invoke not ok: $(head -c 400 "$WORKDIR/http/invoke_project_state.json")"
log "project.state via agent invoke -> ok"

# ---------------------------------------------------------------- plugin scan
log "starting plugin scan (scan_plugins paths=[$VST3_DIR])..."
SCAN_REPLY="$(zmq_send 30000 "{\"cmd\":\"scan_plugins\",\"paths\":[\"$VST3_DIR\"]}")" \
  || fail_functional "scan_plugins transport failed"
printf '%s\n' "$SCAN_REPLY" > "$WORKDIR/zmq/scan_start_reply.json"
# The scan snapshot carries status=<state> (not "ok"): a started scan reports
# scanning (or completed for an instantly-finished sweep).
SCAN_STATUS="$(json_field "$WORKDIR/zmq/scan_start_reply.json" 'str(d.get("status",""))')"
SCAN_ID="$(json_field "$WORKDIR/zmq/scan_start_reply.json" 'str(d.get("scan_id",""))')"
SCAN_ERROR="$(json_field "$WORKDIR/zmq/scan_start_reply.json" 'str(d.get("error",""))')"
[[ "$SCAN_STATUS" == "scanning" || "$SCAN_STATUS" == "completed" ]] \
  || fail_functional "scan_plugins did not start (status=$SCAN_STATUS error=$SCAN_ERROR): $SCAN_REPLY"
[[ -n "$SCAN_ID" ]] || fail_functional "scan_plugins reply missing scan_id: $SCAN_REPLY"
log "scan started: scan_id=$SCAN_ID status=$SCAN_STATUS out_of_process=$(json_field "$WORKDIR/zmq/scan_start_reply.json" 'str(d.get("out_of_process",""))')"

# Child-process evidence (R9): tracktion spawns the kernel binary itself with
# --PluginScan:<uid> as the scanner child; sample it while scanning.
: > "$WORKDIR/ps_pluginscan_samples.txt"
sample_child_scanners() {
  local out
  out="$(pgrep -lf 'PluginScan' 2>/dev/null || true)"
  if [[ -n "$out" ]]; then
    printf '[%s]\n%s\n' "$(date '+%H:%M:%S')" "$out" >> "$WORKDIR/ps_pluginscan_samples.txt"
  fi
}

: > "$WORKDIR/scan_status_trajectory.jsonl"
deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  sample_child_scanners
  if ! STATUS_REPLY="$(zmq_send 15000 "{\"cmd\":\"plugin_scan_status\",\"scan_id\":\"$SCAN_ID\"}")"; then
    fail_functional "plugin_scan_status transport failed mid-scan"
  fi
  printf '%s\n' "$STATUS_REPLY" >> "$WORKDIR/scan_status_trajectory.jsonl"
  STATE="$(printf '%s' "$STATUS_REPLY" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(str(d.get("status","")))')"
  if [[ "$STATE" != "scanning" && "$STATE" != "cancelling" ]]; then
    break
  fi
  sleep 2
done
printf '%s\n' "$STATUS_REPLY" > "$WORKDIR/zmq/scan_final_status.json"
[[ "$STATE" == "completed" ]] \
  || fail_functional "plugin scan did not complete within ${SCAN_TIMEOUT_SECONDS}s (final state: $STATE; see $WORKDIR/scan_status_trajectory.jsonl)"
scan_total="$(json_field "$WORKDIR/zmq/scan_final_status.json" 'int(d.get("total_files",0))')"
scan_plugin_count="$(json_field "$WORKDIR/zmq/scan_final_status.json" 'int(d.get("plugin_count",0))')"
scan_failed="$(json_field "$WORKDIR/zmq/scan_final_status.json" 'len(d.get("failed_files",[]))')"
scan_out_of_process="$(json_field "$WORKDIR/zmq/scan_final_status.json" 'str(d.get("out_of_process",""))')"
log "scan completed: state=$STATE total_files=$scan_total plugin_count=$scan_plugin_count failed_files=$scan_failed out_of_process=$scan_out_of_process"
sample_child_scanners

# Child-process anchor in kernel logs (R9): the tracktion master logs
# "Launched Plugin Scan Process" when the worker child came up.
KERNEL_LOG_DIR="$KERNEL_ROOT/Workspace/Logs"
grep -r "Plugin Scan Process\|PluginScan" "$KERNEL_LOG_DIR" > "$WORKDIR/kernel_log_pluginscan_anchor.txt" 2>/dev/null || true

log "listing enumerated plugins (plugin_list_available)..."
LIST_REPLY="$(zmq_send 30000 '{"cmd":"plugin_list_available"}')" \
  || fail_functional "plugin_list_available transport failed"
printf '%s\n' "$LIST_REPLY" > "$WORKDIR/zmq/plugin_list_reply.json"
python3 - "$WORKDIR/zmq/plugin_list_reply.json" "$WORKDIR/waves_list.json" <<'PY'
import json, sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
plugins = d.get("plugins") or d.get("result", {}).get("plugins") or []
rows = []
for p in plugins:
    name = str(p.get("name", ""))
    manufacturer = str(p.get("manufacturer", ""))
    path = str(p.get("file_or_identifier", p.get("plugin_path", "")))
    is_waves = ("waves" in manufacturer.lower()) or ("waveshell" in path.lower()) or ("waves" in name.lower())
    rows.append({"name": name, "manufacturer": manufacturer, "path": path, "is_waves": is_waves})
with open(sys.argv[2], "w", encoding="utf-8") as f:
    json.dump(rows, f, ensure_ascii=False, indent=2)
PY
read -r PLUGIN_TOTAL WAVES_COUNT <<<"$(python3 -c '
import json
rows = json.load(open("'"$WORKDIR"'/waves_list.json"))
print(len(rows), sum(1 for r in rows if r["is_waves"]))')"

# R2 first verification: explicit conclusion regardless of count.
if (( WAVES_COUNT >= EXPECTED_WAVES )); then
  R2_CONCLUSION="WaveShell1-VST3 枚举成功：$WAVES_COUNT 个 Waves 主体（期望 >= ${EXPECTED_WAVES}），形态见 waves_list.json"
  log "R2 first verification PASS: $R2_CONCLUSION"
else
  R2_CONCLUSION="WaveShell1-VST3 枚举失败/不完整：仅 $WAVES_COUNT 个 Waves 主体（期望 ${EXPECTED_WAVES}）；保留证据转 blocked（R2 预案：升级 tracktion/JUCE 或补丁另开卡）"
  log "R2 first verification FAIL: $R2_CONCLUSION"
fi
(( WAVES_COUNT >= EXPECTED_WAVES )) \
  || { FAIL_CLASSIFICATION="functional_r2"; }

# ---------------------------------------------------------------- stop stack
# Runs in the MAIN shell (not under $()): wait must reap the parent shell's
# own children to capture the real exit code (signal deaths surface as
# 128+signal, e.g. 143 for SIGTERM without JUCE shutdown — A1 finding).
STOP_RECORD=""
stop_process() {
  # stop_process <pid> — SIGTERM + grace, then SIGKILL; fills STOP_RECORD
  # with "signal:exit_code:elapsed_seconds" for the summary.
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

# Live mapping evidence while the kernel still runs (VIT_ENABLE_SHARED_MEMORY_TEST=1
# published /Vit_Waveform_Test at startup; A1 evidence form: lsof PSXSHM).
lsof -nP -p "$KERNEL_PID" 2>/dev/null | grep PSXSHM > "$WORKDIR/lsof_shm_kernel_alive.txt" || true
log "kernel-alive POSIX shm (lsof PSXSHM): $(grep -c PSXSHM "$WORKDIR/lsof_shm_kernel_alive.txt" || true) segment(s)"

log "stopping agent (pid $AGENT_PID)..."
stop_process "$AGENT_PID"
AGENT_STOP_RECORD="$STOP_RECORD"
log "agent stop: $AGENT_STOP_RECORD"

log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "kernel stop: $KERNEL_STOP_RECORD (signal:exit_code:elapsed — A1 finding: SIGTERM expected to yield 143 without JUCE shutdown)"
KERNEL_PID=""
AGENT_PID=""

# Residue after a non-graceful stop is invisible to lsof (no live mapping), so
# probe with shm_open(O_CREAT|O_EXCL): EEXIST proves the stale segment survived
# the kernel's death (A1 finding: releaseTestMemory never runs); then unlink
# the residue. This mirrors the A1 stale-self-heal reality on the reader side.
python3 - "$WORKDIR" <<'PY' > "$WORKDIR/shm_residue_probe.txt"
import ctypes, os, sys

workdir = sys.argv[1]
libc = ctypes.CDLL(None, use_errno=True)
O_CREAT, O_EXCL, O_RDWR = 0x0200, 0x0800, 0x0002
probe = "/Vit_Waveform_Test"
fd = libc.shm_open(probe.encode(), O_CREAT | O_EXCL | O_RDWR, 0o600)
if fd >= 0:
    os.close(fd)
    libc.shm_unlink(probe.encode())
    print(f"residue=absent shm_open(O_EXCL) succeeded (segment was cleanly removed)")
else:
    err = ctypes.get_errno()
    print(f"residue={'present' if err == 17 else 'unknown'} shm_open(O_EXCL) errno={err} (17=EEXIST -> stale segment survived non-graceful stop)")
    if err == 17:
        rc = libc.shm_unlink(probe.encode())
        print(f"cleanup shm_unlink({probe}) rc={rc}")
PY
cat "$WORKDIR/shm_residue_probe.txt" >&2
SHM_RESIDUE="$(grep -c 'residue=present' "$WORKDIR/shm_residue_probe.txt" || true)"
[[ "$SHM_RESIDUE" -ge 1 ]] \
  && log "A1 finding reproduced: POSIX shm residue survived kernel stop (stale segment present; unlinked now, evidence in shm_residue_probe.txt)" \
  || log "no POSIX shm residue after kernel stop (segment absent — see shm_residue_probe.txt)"

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
python3 - "$SUMMARY_FILE" "$RUN_ID" "$WORKDIR" \
  "$KERNEL_STOP_RECORD" "$AGENT_STOP_RECORD" \
  "$SCAN_ID" "$STATE" "$scan_total" "$scan_plugin_count" "$scan_failed" "$scan_out_of_process" \
  "$PLUGIN_TOTAL" "$WAVES_COUNT" "$EXPECTED_WAVES" "$R2_CONCLUSION" \
  "$SHM_RESIDUE" "$FAIL_CLASSIFICATION" <<'PY'
import json, os, sys

(out, run_id, workdir, kernel_stop, agent_stop, scan_id, scan_state,
 total_files, plugin_count, failed_count, out_of_process, plugin_total,
 waves_count, expected_waves, r2, shm_residue, fail_class) = sys.argv[1:18]

child_samples = 0
ps_path = os.path.join(workdir, "ps_pluginscan_samples.txt")
if os.path.exists(ps_path):
    child_samples = sum(1 for line in open(ps_path, encoding="utf-8") if line.startswith("["))
log_anchor = ""
anchor_path = os.path.join(workdir, "kernel_log_pluginscan_anchor.txt")
if os.path.exists(anchor_path):
    log_anchor = open(anchor_path, encoding="utf-8").read().strip().splitlines()[:1]
    log_anchor = log_anchor[0][:200] if log_anchor else ""

gates = {
    "kernel_start_ports": True,
    "agent_start_health": True,
    "http_whitelist_gets": True,
    "zmq_direct_ping": True,
    "agent_invoke_project_state": True,
    "plugin_scan_completed": scan_state == "completed",
    "r2_waveshell_enumeration": int(waves_count) >= int(expected_waves),
    "r9_child_process_scan": (out_of_process in ("True", "true", "1")) or child_samples > 0 or bool(log_anchor),
}
summary = {
    "schema_version": "dev.agent.smoke.mac.v1",
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": fail_class or ("none" if all(gates.values()) else "functional"),
    "stack": {"kernel_pid_record": True, "agent_pid_record": True},
    "kernel_stop": {"record": kernel_stop},
    "agent_stop": {"record": agent_stop},
    "plugin_scan": {
        "scan_id": scan_id,
        "final_state": scan_state,
        "total_files": int(total_files),
        "plugin_count": int(plugin_count),
        "failed_files": int(failed_count),
        "out_of_process": out_of_process,
    },
    "plugin_list": {"total": int(plugin_total), "waves": int(waves_count), "expected_waves": int(expected_waves)},
    "r2_first_verification": r2,
    "r9_child_process": {
        "kernel_out_of_process_flag": out_of_process == "True" or out_of_process == "true" or out_of_process == "1",
        "ps_samples": child_samples,
        "kernel_log_anchor": log_anchor,
    },
    "shm_residue_segments": int(shm_residue),
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

if [[ -n "$FAIL_CLASSIFICATION" ]] || ! python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
sys.exit(0 if all(d["gates"].values()) else 1)' "$SUMMARY_FILE"; then
  exit 1
fi
exit 0
