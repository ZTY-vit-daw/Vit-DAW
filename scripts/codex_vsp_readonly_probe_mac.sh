#!/usr/bin/env bash
# Codex VSP read-only probe, mac equivalent — PORT-SMOKE-MAC-3 (piece 2/5).
#
# Mac port of scripts/codex_vsp_readonly_probe.ps1 (PC-only original, zero
# changes there). The ps1 is a pure probe against a RUNNING hub; on mac the
# card mandates the three-piece stack basis (kernel + vsphub + agent, PORT-
# VSPHUB-1 recipe), so this script starts the stack itself, runs the probe
# gates against the live hub, then tears everything down. VSPHUB-1 already
# exercised this probe's non-HubOnly semantics as a 10-check inline subset
# (vsp_hub_three_piece_smoke_mac.sh ④); this is the standalone equivalent.
#
#   | ps1 (codex_vsp_readonly_probe.ps1)     | mac (this script)                |
#   |----------------------------------------|----------------------------------|
#   | param($HubUrl = …:8787/vsp)            | --hub-http (default …:8787); the |
#   |                                        | stack is started on that port    |
#   | param($StatusUrl = "")                 | derived from --hub-http          |
#   | param($ClientId = codex.agent.local)   | --client-id (same default)       |
#   | param($ClientVersion = …probe-v1)      | --client-version (same default)  |
#   | param($Scope = project.timeline)       | --scope (same default)           |
#   | [int]$TimeoutSeconds = 20              | --timeout (same default)         |
#   | [switch]$HubOnly                       | --hub-only flag                  |
#   | health: status=ok service=VspHub       | same                             |
#   | /vsp/status caps: extension advertises | same                             |
#   |   state.snapshot+event.poll, NOT       |                                  |
#   |   command.request                      |                                  |
#   | session.hello (read_only=true) → ack   | same (skipped in --hub-only)     |
#   | extension.register → ack + status      | same                             |
#   |   lists codex session                  |                                  |
#   | state.snapshot (scope): payload ok +   | same; internal track-id leak     |
#   |   no internal track-id leak            |   check only for project.timeline|
#   | event.poll topics → event.progress/    | same (skipped in --hub-only)     |
#   |   event.notification with event_id     |                                  |
#   | extension.unregister in finally        | same                             |
#   | status removes codex session           | same                             |
#   | summary JSON to stdout                 | summary JSON to workdir artifact |
#   |                                        | + stdout, exit 0                 |
#
# Mac additions (stack wrapper per the card): VSPHUB-1 three-piece start,
# port-ownership asserts, teardown with stop records + port-drain assert,
# run artifacts. Failures classified env (exit 3) vs functional (exit 1).
#
# §8 discipline (pre-declared on the card): deterministic protocol probe —
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
CLIENT_ID="codex.agent.local"
CLIENT_VERSION="codex-vsp-readonly-probe-v1"
SCOPE="project.timeline"
TIMEOUT_SECONDS=20
HUB_ONLY=0
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
codex_vsp_readonly_probe_mac.sh — Codex VSP read-only probe on the mac
three-piece stack (kernel + vsphub + agent) — PORT-SMOKE-MAC-3 piece 2/5,
mac port of codex_vsp_readonly_probe.ps1.

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --hub-http URL          VSP Hub HTTP base (default http://127.0.0.1:8787)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-builds           With --agent-bin/--hub-bin: do not build either
  --agent-bin PATH        Agent binary to start (only with --skip-builds)
  --hub-bin PATH          vsphub binary to start (only with --skip-builds)
  --client-id ID          Probe client_id (default codex.agent.local)
  --client-version VERS   Probe client_version (default
                          codex-vsp-readonly-probe-v1)
  --scope SCOPE           state.snapshot scope (default project.timeline)
  --timeout SECONDS       Probe HTTP timeout (default 20)
  --hub-only              Skip kernel-routed reads (hello + snapshot +
                          event.poll); ps1 -HubOnly
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
    --client-id) CLIENT_ID="$2"; shift 2 ;;
    --client-version) CLIENT_VERSION="$2"; shift 2 ;;
    --scope) SCOPE="$2"; shift 2 ;;
    --timeout) TIMEOUT_SECONDS="$2"; shift 2 ;;
    --hub-only) HUB_ONLY=1; shift ;;
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

RUN_ID="codex_vsp_probe_mac_$(date '+%Y%m%d-%H%M%S')"
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
  echo "script=codex_vsp_readonly_probe_mac.sh (mac port of codex_vsp_readonly_probe.ps1, PORT-SMOKE-MAC-3 piece 2/5)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "hub_http=$HUB_HTTP"
  echo "client_id=$CLIENT_ID client_version=$CLIENT_VERSION"
  echo "scope=$SCOPE timeout=$TIMEOUT_SECONDS hub_only=$HUB_ONLY"
  echo "udp_to_godot=$UDP_TO_GODOT udp_from_godot=$UDP_FROM_GODOT"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

# ---------------------------------------------------------------- builds
KERNEL_BIN=""
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  {
    shasum -a 256 "$KERNEL_BIN"
    stat -f "mtime=%Sm" -t "%Y-%m-%dT%H:%M:%S%z" "$KERNEL_BIN"
  } > "$WORKDIR/kernel_bin.sha256"
  log "using provided kernel binary: $KERNEL_BIN (sha256+mtime recorded)"
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
log "binaries: agent=$(shasum -a 256 "$AGENT_BIN" | cut -c1-16)... hub=$(shasum -a 256 "$HUB_BIN" | cut -c1-16)... kernel=$(cut -c1-16 "$WORKDIR/kernel_bin.sha256")..."

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
gate kernel_start_ports true
log "kernel ZMQ ports listening (owned by pid $KERNEL_PID)"

# ------------------------------------------------ start agent BEFORE hub (retry evidence)
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

# ---------------------------------------------------------------- codex read-only probe
# Python port of the ps1 probe body: same envelopes, same checks, same check
# names (codex_*), unregister in finally, post-unregister status cleanup.
log "codex VSP read-only probe (ps1 semantics)..."
python3 - "$HUB_VSP_URL" "$HUB_STATUS_URL" "$CLIENT_ID" "$CLIENT_VERSION" \
  "$SCOPE" "$TIMEOUT_SECONDS" "$HUB_ONLY" \
  "$WORKDIR/http/codex_vsp_readonly_probe.json" <<'PY' \
  || fail_functional "codex read-only probe failed (see $WORKDIR/http/codex_vsp_readonly_probe.json and prior log lines)"
import json, sys, time, uuid, urllib.request, urllib.error

(hub_url, status_url, client_id, client_version, scope,
 timeout_seconds, hub_only, out_path) = sys.argv[1:9]
timeout_seconds = int(timeout_seconds)
hub_only = hub_only == "1"
KERNEL_INTERNAL_TIMELINE_TRACK_IDS = {"1002", "1003", "1004", "1005", "1006"}

def fail(msg):
    print("PROBE_FAIL: " + msg, file=sys.stderr)
    raise SystemExit(1)

def get(url):
    with urllib.request.urlopen(url, timeout=timeout_seconds) as r:
        return json.loads(r.read().decode("utf-8"))

def post(env):
    body = json.dumps(env).encode("utf-8")
    req = urllib.request.Request(hub_url, data=body, method="POST",
                                 headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout_seconds) as r:
            return r.status, json.loads(r.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            parsed = json.loads(raw)
        except Exception:
            parsed = {"raw": raw}
        return e.code, parsed

def envelope(session_id, channel, msg_type, payload):
    return {
        "vsp_version": "1.0",
        "schema": "vsp.%s.v1" % msg_type,
        "message_id": "msg_codex_probe_" + uuid.uuid4().hex,
        "session_id": session_id,
        "client_id": client_id,
        "role": "extension",
        "channel": channel,
        "type": msg_type,
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "trace_id": "trace_codex_probe_" + uuid.uuid4().hex,
        "payload": payload,
    }

def require_reply(reply, status, expected, label):
    if status != 200:
        fail("%s returned HTTP %s: %s" % (label, status, json.dumps(reply)[:400]))
    if reply.get("type") not in expected:
        fail("%s returned unexpected type %s: %s" % (label, reply.get("type"), json.dumps(reply)[:400]))

def status_has_session(session_id):
    status = get(status_url)
    for s in status.get("sessions", []) or []:
        if (str(s.get("session_id")) == session_id
                and str(s.get("client_id")) == client_id
                and str(s.get("role")) == "extension"):
            return True
    return False

checks = []
session_id = "sess_codex_probe_%d" % int(time.time() * 1000)
registered = False

health_url = status_url.replace("/vsp/status", "/health", 1) if status_url.endswith("/vsp/status") \
    else status_url[: status_url.rfind("/vsp")] + "/health"
health = get(health_url)
if health.get("status") != "ok" or health.get("service") != "VspHub":
    fail("health endpoint was not ok: " + json.dumps(health)[:400])
checks.append("hub_health_ok")

status = get(status_url)
ext_caps = set((status.get("capabilities") or {}).get("extension") or [])
if "state.snapshot" not in ext_caps or "event.poll" not in ext_caps:
    fail("extension role does not advertise state.snapshot+event.poll")
if "command.request" in ext_caps:
    fail("extension role unexpectedly advertises command.request")
checks.append("status_advertises_readonly_extension_caps")

try:
    if not hub_only:
        hello = envelope("session_pending", "session", "session.hello", {
            "client_name": "Codex VSP Read-only Probe",
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
        checks.append("codex_session_hello_ack")
    else:
        checks.append("hub_only_skipped_kernel_hello")

    register = envelope(session_id, "extension", "extension.register", {
        "name": "Codex VSP Read-only Probe",
        "version": client_version,
        "wants": ["state.snapshot", "event.poll"],
        "transport_bindings": ["vsp.hub.http"],
        "read_only": True,
    })
    status_code, reply = post(register)
    require_reply(reply, status_code, ["extension.register_ack"], "extension.register")
    registered = True
    checks.append("codex_extension_register_ack")

    if not status_has_session(session_id):
        fail("codex extension session was not visible in /vsp/status")
    checks.append("status_lists_codex_session")

    if not hub_only:
        snapshot = envelope(session_id, "state", "state.snapshot_request", {
            "scope": scope,
        })
        status_code, reply = post(snapshot)
        require_reply(reply, status_code, ["state.snapshot"], "state.snapshot_request")
        if str((reply.get("payload") or {}).get("status")) != "ok":
            fail("state.snapshot payload was not ok: " + json.dumps(reply)[:400])
        if scope == "project.timeline":
            track_ids = []
            for row in (reply.get("payload") or {}).get("tracks") or []:
                tid = str(row.get("track_id", row.get("id", "")))
                if tid.strip():
                    track_ids.append(tid.strip())
            leaked = sorted(set(track_ids) & KERNEL_INTERNAL_TIMELINE_TRACK_IDS)
            if leaked:
                fail("state.snapshot project.timeline leaked internal track ids: " + ",".join(leaked))
            checks.append("codex_state_snapshot_hides_internal_tracks")
        checks.append("codex_state_snapshot_read")

        event_poll = envelope(session_id, "event", "event.poll", {
            "topics": ["project", "import", "bake", "render"],
        })
        status_code, reply = post(event_poll)
        require_reply(reply, status_code, ["event.progress", "event.notification"], "event.poll")
        if not str((reply.get("payload") or {}).get("event_id", "")).strip():
            fail("event.poll did not return an event_id: " + json.dumps(reply)[:400])
        checks.append("codex_event_poll_read")
    else:
        checks.append("hub_only_skipped_kernel_reads")
finally:
    if registered:
        unreg = envelope(session_id, "extension", "extension.unregister", {})
        status_code, reply = post(unreg)
        if status_code == 200 and reply.get("type") == "extension.unregister_ack":
            checks.append("codex_extension_unregister_ack")

if status_has_session(session_id):
    fail("codex extension session remained visible after unregister")
checks.append("status_removes_codex_session")

summary = {
    "status": "ok",
    "schema_version": "codex_vsp_readonly_probe.v1",
    "mac_port": True,
    "hub_url": hub_url,
    "status_url": status_url,
    "client_id": client_id,
    "role": "extension",
    "session_id": session_id,
    "read_only": True,
    "hub_only": hub_only,
    "mutation_channels_sent": [],
    "checks": checks,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print("PROBE_OK checks=" + ",".join(checks))
PY
gate codex_readonly_probe true
log "codex read-only probe PASS ($(python3 -c 'import json; d=json.load(open("'"$WORKDIR"'/http/codex_vsp_readonly_probe.json")); print(len(d["checks"]), "checks")'))"

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
  "$KERNEL_STOP_RECORD" "$HUB_STOP_RECORD" "$AGENT_STOP_RECORD" "$GATES_FILE" <<'PY'
import json, os, sys

(out, run_id, workdir, kernel_stop, hub_stop, agent_stop, gates_path) = sys.argv[1:8]

gates = {}
for line in open(gates_path, encoding="utf-8"):
    line = line.strip()
    if "=" in line:
        k, v = line.split("=", 1)
        gates[k] = (v == "true")

probe_path = os.path.join(workdir, "http", "codex_vsp_readonly_probe.json")
probe = json.load(open(probe_path, encoding="utf-8")) if os.path.exists(probe_path) else {}

summary = {
    "schema_version": "codex.vsp.readonly.probe.mac.v1",
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "gates": gates,
    "stops": {
        "kernel": kernel_stop,
        "hub": hub_stop,
        "agent": agent_stop,
    },
    "codex_readonly_probe": probe,
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
