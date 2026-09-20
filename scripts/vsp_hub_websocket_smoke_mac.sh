#!/usr/bin/env bash
# VSP Hub WebSocket smoke, mac equivalent — PORT-SMOKE-MAC-3 (piece 4/5).
#
# Mac port of scripts/vsp_hub_websocket_smoke.ps1 (PC-only original, zero
# changes there). The ps1 uses .NET's System.Net.WebSockets.ClientWebSocket;
# mac python3 has no websocket library installed, so the probe embeds a
# minimal RFC 6455 client over stdlib sockets (handshake with
# Sec-WebSocket-Accept verification, client-masked text frames, fragment
# reassembly, ping->pong). Same envelopes, same checks, same check names.
# The stack basis is the three-piece form (kernel + vsphub + agent, PORT-
# VSPHUB-1 recipe): this script starts the stack itself, runs the ws probe
# against the live hub ws endpoint, then tears everything down. (The VSPHUB-1
# ws probe pattern — Godot headless adapter — tests the ADAPTER; this piece
# tests the raw hub ws protocol face, like the ps1.)
#
#   | ps1 (vsp_hub_websocket_smoke.ps1)      | mac (this script)                |
#   |----------------------------------------|----------------------------------|
#   | param($StreamUrl = …:8787/vsp/stream)  | --stream-url derived from        |
#   |                                        | --hub-http (default …:8787)      |
#   | param($StatusUrl = …:8787/vsp/status)  | derived from --hub-http          |
#   | [switch]$CheckKernelRoutes             | --check-kernel-routes flag       |
#   | ClientWebSocket connect (20s)          | stdlib RFC6455 handshake (same   |
#   |                                        | 20s timeout, accept-key verify)  |
#   | session.hello over ws (role=tool,      | same (only with kernel routes);  |
#   |   websocket.transport.smoke) → ack     |   correlation_id = message_id;   |
#   |   hub.transport_binding=               |   hub.transport_binding must be   |
#   |   vsp.hub.websocket                    |   vsp.hub.websocket              |
#   | extension.register over ws (client-    | same; ack session_id must equal  |
#   |   supplied sess_ws_ext_smoke_<ms>,     | the client-supplied id           |
#   |   websocket.extension.smoke) → ack     |                                  |
#   | /vsp/status lists session with         | same (transport=vsp.hub.         |
#   |   transport=vsp.hub.websocket          |   websocket via HTTP GET)        |
#   | extension.unregister → ack with        | same                             |
#   |   payload.removed=true                 |                                  |
#   | status removes ws extension            | same                             |
#   | event.notification frames skipped      | same (counted, not asserted)     |
#   |   while waiting (up to 40 messages)    |                                  |
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
CHECK_KERNEL_ROUTES=0
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
vsp_hub_websocket_smoke_mac.sh — VSP Hub WebSocket smoke on the mac
three-piece stack (kernel + vsphub + agent) — PORT-SMOKE-MAC-3 piece 4/5,
mac port of vsp_hub_websocket_smoke.ps1 (embedded stdlib RFC 6455 client).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --hub-http URL          VSP Hub HTTP base (default http://127.0.0.1:8787);
                          the ws stream url is ws://<hub>/vsp/stream
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-builds           With --agent-bin/--hub-bin: do not build either
  --agent-bin PATH        Agent binary to start (only with --skip-builds)
  --hub-bin PATH          vsphub binary to start (only with --skip-builds)
  --check-kernel-routes   Also drive ws session.hello through the hub to the
                          kernel (ps1 -CheckKernelRoutes)
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
    --check-kernel-routes) CHECK_KERNEL_ROUTES=1; shift ;;
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

RUN_ID="vsp_ws_smoke_mac_$(date '+%Y%m%d-%H%M%S')"
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
WS_STREAM_URL="ws://${HUB_HTTP_ADDR%/}/vsp/stream"

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
  echo "script=vsp_hub_websocket_smoke_mac.sh (mac port of vsp_hub_websocket_smoke.ps1, PORT-SMOKE-MAC-3 piece 4/5)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "hub_http=$HUB_HTTP"
  echo "ws_stream_url=$WS_STREAM_URL"
  echo "check_kernel_routes=$CHECK_KERNEL_ROUTES"
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

# ---------------------------------------------------------------- websocket probe
# Stdlib RFC 6455 client + the ps1 probe semantics: envelope face identical,
# checks websocket_*, correlation_id matching, event.notification skipping.
log "VSP Hub websocket probe (ps1 semantics, stdlib RFC6455 client)..."
python3 - "$WS_STREAM_URL" "$HUB_STATUS_URL" "$CHECK_KERNEL_ROUTES" \
  "$WORKDIR/http/vsp_hub_websocket_probe.json" <<'PY' \
  || fail_functional "websocket probe failed (see $WORKDIR/http/vsp_hub_websocket_probe.json and prior log lines)"
import base64
import hashlib
import json
import os
import socket
import struct
import sys
import time
import urllib.request
import uuid

stream_url, status_url, check_kernel_routes, out_path = sys.argv[1:5]
check_kernel_routes = check_kernel_routes == "1"
WS_GUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
TIMEOUT_SECONDS = 20


def fail(msg):
    print("PROBE_FAIL: " + msg, file=sys.stderr)
    raise SystemExit(1)


def parse_ws_url(url):
    if not url.startswith("ws://"):
        fail("only ws:// stream urls are supported: %s" % url)
    rest = url[len("ws://"):]
    slash = rest.find("/")
    if slash < 0:
        hostport, path = rest, "/"
    else:
        hostport, path = rest[:slash], rest[slash:]
    if ":" in hostport:
        host, port = hostport.rsplit(":", 1)
        port = int(port)
    else:
        host, port = hostport, 80
    return host, port, path


class WsClient:
    """Minimal RFC 6455 client: handshake, masked text send, message recv
    with fragment reassembly and ping->pong. Server frames are unmasked."""

    def __init__(self, url):
        self.host, self.port, self.path = parse_ws_url(url)
        self.sock = socket.create_connection((self.host, self.port), timeout=TIMEOUT_SECONDS)
        self.sock.settimeout(TIMEOUT_SECONDS)
        self._buf = b""

    def connect(self):
        key = base64.b64encode(os.urandom(16)).decode("ascii")
        request = (
            "GET %s HTTP/1.1\r\n"
            "Host: %s:%d\r\n"
            "Upgrade: websocket\r\n"
            "Connection: Upgrade\r\n"
            "Sec-WebSocket-Key: %s\r\n"
            "Sec-WebSocket-Version: 13\r\n"
            "\r\n" % (self.path, self.host, self.port, key)
        )
        self.sock.sendall(request.encode("ascii"))
        response = b""
        while b"\r\n\r\n" not in response:
            chunk = self.sock.recv(4096)
            if not chunk:
                fail("websocket handshake: connection closed before response headers")
            response += chunk
        head, _, rest = response.partition(b"\r\n\r\n")
        if rest:
            self._buf = rest
        lines = head.decode("latin-1").split("\r\n")
        if not lines[0].split(" ")[1:] or lines[0].split(" ")[1] != "101":
            fail("websocket handshake: unexpected status line: %s" % lines[0])
        headers = {}
        for line in lines[1:]:
            if ":" in line:
                k, v = line.split(":", 1)
                headers[k.strip().lower()] = v.strip()
        expected_accept = base64.b64encode(
            hashlib.sha1((key + WS_GUID).encode("ascii")).digest()).decode("ascii")
        if headers.get("sec-websocket-accept") != expected_accept:
            fail("websocket handshake: Sec-WebSocket-Accept mismatch (got %r)" %
                 headers.get("sec-websocket-accept"))
        upgrade = headers.get("upgrade", "").lower()
        if upgrade != "websocket":
            fail("websocket handshake: Upgrade header was not websocket: %r" % upgrade)

    def _read_exact(self, n):
        while len(self._buf) < n:
            chunk = self.sock.recv(65536)
            if not chunk:
                raise ConnectionError("websocket closed by peer")
            self._buf += chunk
        out, self._buf = self._buf[:n], self._buf[n:]
        return out

    def send_text(self, text):
        payload = text.encode("utf-8")
        header = bytearray([0x81])  # FIN + text opcode
        mask_bit = 0x80
        length = len(payload)
        if length < 126:
            header.append(mask_bit | length)
        elif length < 65536:
            header.append(mask_bit | 126)
            header.extend(struct.pack(">H", length))
        else:
            header.append(mask_bit | 127)
            header.extend(struct.pack(">Q", length))
        mask = os.urandom(4)
        header.extend(mask)
        masked = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
        self.sock.sendall(bytes(header) + masked)

    def _read_frame(self):
        b1, b2 = self._read_exact(2)
        fin = bool(b1 & 0x80)
        opcode = b1 & 0x0F
        masked = bool(b2 & 0x80)
        length = b2 & 0x7F
        if length == 126:
            length = struct.unpack(">H", self._read_exact(2))[0]
        elif length == 127:
            length = struct.unpack(">Q", self._read_exact(8))[0]
        mask = self._read_exact(4) if masked else None
        payload = self._read_exact(length) if length else b""
        if mask:
            payload = bytes(b ^ mask[i % 4] for i, b in enumerate(payload))
        return fin, opcode, payload

    def recv_message(self):
        """Returns one reassembled data message; answers pings; raises on close."""
        parts = []
        message_opcode = None
        while True:
            fin, opcode, payload = self._read_frame()
            if opcode == 8:  # close
                fail("websocket closed before the expected reply (close payload=%r)" % payload[:120])
            if opcode == 9:  # ping -> pong (client frames must be masked)
                pong = bytearray([0x8A])
                n = len(payload)
                if n < 126:
                    pong.append(0x80 | n)
                else:
                    pong.append(0x80 | 126)
                    pong.extend(struct.pack(">H", n))
                mask = os.urandom(4)
                pong.extend(mask)
                pong.extend(bytes(b ^ mask[i % 4] for i, b in enumerate(payload)))
                self.sock.sendall(bytes(pong))
                continue
            if opcode == 10:  # pong
                continue
            if opcode in (1, 2):
                message_opcode = opcode
                parts.append(payload)
            elif opcode == 0:
                parts.append(payload)
            else:
                continue
            if fin:
                data = b"".join(parts)
                if message_opcode == 1:
                    return data.decode("utf-8", "replace")
                return data.decode("utf-8", "replace")

    def close(self):
        try:
            self.sock.close()
        except OSError:
            pass


def envelope(session_id, client_id, role, channel, msg_type, schema, payload):
    return {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": "msg_ws_smoke_" + uuid.uuid4().hex,
        "session_id": session_id,
        "client_id": client_id,
        "role": role,
        "channel": channel,
        "type": msg_type,
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "trace_id": "trace_ws_smoke_" + uuid.uuid4().hex,
        "payload": payload,
    }


def receive_expected(sock, expected_type, correlation_id=""):
    """ps1 Receive-VspWebSocket: up to 40 messages, skip event.notification
    (counted) and non-matching types/correlations, fail if never seen."""
    event_count = 0
    for _ in range(40):
        text = sock.recv_message().strip()
        if not text:
            continue
        message = json.loads(text)
        if message.get("type") == "event.notification":
            event_count += 1
            continue
        if message.get("type") != expected_type:
            continue
        if correlation_id and message.get("correlation_id") != correlation_id:
            continue
        return message, event_count
    fail("missing %s reply" % expected_type)


def get_status():
    with urllib.request.urlopen(status_url, timeout=TIMEOUT_SECONDS) as r:
        return json.loads(r.read().decode("utf-8"))


def status_sessions():
    return get_status().get("sessions", []) or []


checks = []
extension_session_id = "sess_ws_ext_smoke_%d" % int(time.time() * 1000)

sock = WsClient(stream_url)
try:
    sock.connect()

    if check_kernel_routes:
        hello = envelope("session_pending", "websocket.transport.smoke", "tool", "session",
                         "session.hello", "vsp.session.hello.v1", {
                             "client_name": "VSP Hub WebSocket Smoke",
                             "client_version": "0.1.0",
                             "protocol_min": "1.0",
                             "protocol_max": "1.0",
                             "wants": ["state.snapshot", "event.subscribe"],
                             "transport_bindings": ["vsp.hub.websocket"],
                         })
        sock.send_text(json.dumps(hello))
        hello_reply, _ = receive_expected(sock, "session.hello_ack", hello["message_id"])
        hub_info = hello_reply.get("hub") or {}
        if hub_info.get("transport_binding") != "vsp.hub.websocket":
            fail("session.hello_ack missing websocket transport: " + json.dumps(hello_reply)[:400])
        checks.append("websocket_session_hello_ack")

    register = envelope(extension_session_id, "websocket.extension.smoke", "extension", "extension",
                        "extension.register", "vsp.extension.register.v1", {
                            "name": "WebSocket Extension Smoke",
                            "version": "0.1.0",
                            "wants": ["event.subscribe"],
                            "transport_bindings": ["vsp.hub.websocket"],
                        })
    sock.send_text(json.dumps(register))
    register_reply, _ = receive_expected(sock, "extension.register_ack", register["message_id"])
    if str(register_reply.get("session_id")) != extension_session_id:
        fail("extension.register_ack had unexpected session_id: " + json.dumps(register_reply)[:400])
    checks.append("websocket_extension_register_ack")

    registered = any(
        str(s.get("session_id")) == extension_session_id
        and str(s.get("client_id")) == "websocket.extension.smoke"
        and str(s.get("transport")) == "vsp.hub.websocket"
        for s in status_sessions())
    if not registered:
        fail("websocket extension was not visible in /vsp/status")
    checks.append("status_lists_websocket_extension")

    unregister = envelope(extension_session_id, "websocket.extension.smoke", "extension", "extension",
                          "extension.unregister", "vsp.extension.unregister.v1", {})
    sock.send_text(json.dumps(unregister))
    unregister_reply, _ = receive_expected(sock, "extension.unregister_ack", unregister["message_id"])
    if (unregister_reply.get("payload") or {}).get("removed") is not True:
        fail("extension.unregister_ack did not remove session: " + json.dumps(unregister_reply)[:400])
    checks.append("websocket_extension_unregister_ack")
finally:
    sock.close()

still_registered = any(
    str(s.get("session_id")) == extension_session_id for s in status_sessions())
if still_registered:
    fail("websocket extension session remained visible after unregister")
checks.append("status_removes_websocket_extension")

summary = {
    "status": "ok",
    "schema_version": "vsp_hub_websocket_smoke.v1",
    "mac_port": True,
    "stream_url": stream_url,
    "status_url": status_url,
    "check_kernel_routes": check_kernel_routes,
    "extension_session_id": extension_session_id,
    "checks": checks,
}
with open(out_path, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print("PROBE_OK checks=" + ",".join(checks))
PY
gate websocket_probe true
log "websocket probe PASS ($(python3 -c 'import json; d=json.load(open("'"$WORKDIR"'/http/vsp_hub_websocket_probe.json")); print(len(d["checks"]), "checks")'))"

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

probe_path = os.path.join(workdir, "http", "vsp_hub_websocket_probe.json")
probe = json.load(open(probe_path, encoding="utf-8")) if os.path.exists(probe_path) else {}

summary = {
    "schema_version": "vsp.hub.websocket.smoke.mac.v1",
    "run_id": run_id,
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "gates": gates,
    "stops": {
        "kernel": kernel_stop,
        "hub": hub_stop,
        "agent": agent_stop,
    },
    "websocket_probe": probe,
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
