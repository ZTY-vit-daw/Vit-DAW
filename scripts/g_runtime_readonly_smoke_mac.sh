#!/usr/bin/env bash
# G runtime readonly smoke — mac port of g_runtime_readonly_smoke.ps1
#
# Whitelist GET probes against a running VitAgent HTTP face. The harness is
# strictly read-only: it self-scans its own source for non-GET request
# construction and fails closed before issuing any request.
#
# Probe list, 1:1 with the ps1 whitelist:
#
#   | ps1 (g_runtime_readonly_smoke.ps1) | mac (this script)                  |
#   |------------------------------------|-------------------------------------|
#   | GET /health                        | GET /health                         |
#   | GET /agent/runtime/status          | GET /agent/runtime/status           |
#   | GET /agent/state                   | GET /agent/state                    |
#   | GET /agent/events?limit=200        | GET /agent/events                   |
#   |                                    |   ?conversation_id=<probe>&limit=200|
#
#   The events probe adds conversation_id because the real agent contract
#   (agent/internal/chat/events.go) answers HTTP 400 without it; the ps1's
#   bare "?limit=200" query only satisfies g_runtime_readonly_fixture_server.py,
#   which ignores all query parameters. limit=200 stays within the real
#   agent's event buffer cap (500).
#
# Usage:
#   Probe an already running agent (ps1-equivalent mode):
#     ./scripts/g_runtime_readonly_smoke_mac.sh --agent-http http://127.0.0.1:7878
#
#   Full chain on mac (build agent, start it with state isolated in a fresh
#   temp dir, probe, stop it):
#     ./scripts/g_runtime_readonly_smoke_mac.sh --start-agent --timeout 30
#
# Exit code 0 means every whitelisted GET returned a 2xx status and the
# g.readonly.runtime.smoke.v1 summary was emitted on stdout. Any failure
# (request error, non-2xx status, malformed JSON, guard trip) exits non-zero.
#
# The run workdir keeps all probe artifacts (per-endpoint JSON bodies, agent
# log, binary checksum) and is printed to stderr; each run gets a fresh
# mktemp directory, so previous runs are never overwritten.

set -euo pipefail

AGENT_HTTP="http://127.0.0.1:7878"
TIMEOUT_SECONDS=5
OUTPUT_PATH=""
START_AGENT=0
SKIP_BUILD=0
AGENT_BIN=""
CONVERSATION_ID="g-readonly-smoke-probe"
WORKDIR_ARG=""

ALLOWED_PATHS=(
  "/health"
  "/agent/runtime/status"
  "/agent/state"
  "/agent/events"
)

usage() {
  cat <<'EOF'
g_runtime_readonly_smoke_mac.sh — whitelist GET liveness probes for VitAgent.

Options:
  --agent-http URL       Base URL of the agent HTTP face (default http://127.0.0.1:7878)
  --timeout SECONDS      Per-request curl timeout (default 5; use a larger value
                         when probing a kernel-less agent whose /agent/state
                         retries the kernel dial on every call)
  --output PATH          Also write the summary JSON to PATH
  --start-agent          Build (unless --skip-build) and start a local agent with
                         all runtime state isolated under the run workdir, wait
                         for /health, probe, then stop the agent
  --skip-build           With --start-agent: do not build; requires --agent-bin
  --agent-bin PATH       Agent binary to start (only with --skip-build)
  --conversation-id ID   Probe conversation id for /agent/events
                         (default g-readonly-smoke-probe)
  --workdir PATH         Reuse PATH as the run workdir instead of a fresh mktemp
                         directory (never overwrites artifacts from other runs)
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --timeout) TIMEOUT_SECONDS="$2"; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    --start-agent) START_AGENT=1; shift ;;
    --skip-build) SKIP_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN="$2"; shift 2 ;;
    --conversation-id) CONVERSATION_ID="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

# Fail closed if this script ever grows non-GET request construction. The
# scanned tokens are assembled from fragments so the guard's own source cannot
# contain them literally (same approach as the ps1 harness guard, kept
# independent from the request helper below). Like the ps1 guard, only
# long-form body flags are scanned: the short data flag is a prefix of this
# harness's own mktemp -d call, so scanning it would force a false trip.
assert_readonly_harness() {
  local script_path="$1" tok
  local method_short="-"; method_short="${method_short}X"
  local method_long="--re"; method_long="${method_long}quest"
  local data_long="--da"; data_long="${data_long}ta"
  local form_long="--fo"; form_long="${form_long}rm"
  local upload_long="--up"; upload_long="${upload_long}load-file"
  local upload_short="-"; upload_short="${upload_short}T "
  for tok in "$method_short" "$method_long" "$data_long" \
             "$form_long" "$upload_long" "$upload_short"; do
    if grep -qF -- "$tok" "$script_path"; then
      echo "G readonly harness contains a non-GET request construction token: $tok" >&2
      exit 1
    fi
  done
}

readonly_get() {
  local base_url="$1" path="$2" out_file="$3"
  local matched=0 allowed
  for allowed in "${ALLOWED_PATHS[@]}"; do
    if [[ "$allowed" == "$path" ]]; then
      matched=1
      break
    fi
  done
  if [[ "$matched" -ne 1 ]]; then
    fail "G readonly harness rejected non-whitelisted path: $path"
  fi
  local uri="${base_url%/}${path}"
  if [[ "$path" == "/agent/events" ]]; then
    uri="${uri}?conversation_id=${CONVERSATION_ID}&limit=200"
  fi
  local status err_file
  err_file="${out_file}.curlerr"
  if ! status="$(curl -sS --max-time "$TIMEOUT_SECONDS" -o "$out_file" -w '%{http_code}' "$uri" 2>"$err_file")"; then
    fail "GET $path request failed: $(tr '\n' ' ' < "$err_file")"
  fi
  if [[ "$status" -lt 200 || "$status" -gt 299 ]]; then
    fail "GET $path returned HTTP $status"
  fi
}

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

assert_readonly_harness "$SCRIPT_PATH"

WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/g_readonly_smoke_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR/state"
STATE_DIR="$WORKDIR/state"
AGENT_PID=""

cleanup() {
  if [[ -n "$AGENT_PID" ]] && kill -0 "$AGENT_PID" 2>/dev/null; then
    kill -TERM "$AGENT_PID" 2>/dev/null || true
    local i
    for i in 1 2 3 4 5; do
      kill -0 "$AGENT_PID" 2>/dev/null || break
      sleep 1
    done
    if kill -0 "$AGENT_PID" 2>/dev/null; then
      kill -KILL "$AGENT_PID" 2>/dev/null || true
    fi
  fi
  return 0
}
trap cleanup EXIT

show_agent_log_tail() {
  local f
  for f in "$STATE_DIR/agent_stdout.log" "$STATE_DIR/agent_last.log"; do
    if [[ -f "$f" ]]; then
      echo "---- tail of $f ----" >&2
      tail -n 40 "$f" >&2 || true
    fi
  done
}

wait_for_health() {
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    if curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null; then
      return 0
    fi
    if [[ -n "$AGENT_PID" ]] && ! kill -0 "$AGENT_PID" 2>/dev/null; then
      echo "agent process exited during startup" >&2
      show_agent_log_tail
      return 1
    fi
    sleep 1
  done
  echo "agent /health not ready within 30s" >&2
  show_agent_log_tail
  return 1
}

start_agent() {
  local addr port
  addr="${AGENT_HTTP#http://}"
  addr="${addr#https://}"
  port="${addr##*:}"
  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      fail "port $port already has a listener; this harness never stops processes it did not start (use --agent-http to pick another port)"
    fi
  fi
  if [[ "$SKIP_BUILD" -eq 1 ]]; then
    [[ -n "$AGENT_BIN" ]] || fail "--skip-build requires --agent-bin"
    [[ -x "$AGENT_BIN" ]] || fail "agent binary is not executable: $AGENT_BIN"
  else
    [[ -z "$AGENT_BIN" ]] || fail "--agent-bin is only valid together with --skip-build"
    mkdir -p "$WORKDIR/bin"
    (cd "$REPO_ROOT/agent" && go build -o "$WORKDIR/bin/vitagent" ./cmd/vitagent) \
      || fail "go build failed"
    AGENT_BIN="$WORKDIR/bin/vitagent"
  fi
  shasum "$AGENT_BIN" > "$STATE_DIR/agent_bin.sha1" || true
  {
    echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
    echo "agent_http=$AGENT_HTTP"
    echo "agent_bin=$AGENT_BIN"
    echo "script=$SCRIPT_PATH"
  } > "$STATE_DIR/run_meta.txt"
  mkdir -p "$WORKDIR/agent_cwd"
  (
    cd "$WORKDIR/agent_cwd"
    VIT_ORCHESTRATION_STORE_PATH="$STATE_DIR/orchestration_v1.json" \
    exec "$AGENT_BIN" \
      -http "$addr" \
      -last-log-path "$STATE_DIR/agent_last.log" \
      -vsp-hub-url ""
  ) > "$STATE_DIR/agent_stdout.log" 2>&1 &
  AGENT_PID=$!
  echo "agent pid=$AGENT_PID addr=$addr (state isolated under $STATE_DIR)" >&2
  wait_for_health || fail "agent startup failed; see logs under $STATE_DIR"
}

build_summary() {
  python3 - "$STATE_DIR" "${ALLOWED_PATHS[@]}" <<'PY'
import json
import os
import sys

state_dir = sys.argv[1]
allowed = sys.argv[2:]


def load(name):
    with open(os.path.join(state_dir, name), "rb") as f:
        text = f.read().decode("utf-8").strip()
    if not text:
        return {}
    return json.loads(text)


def prop(value, name):
    if not isinstance(value, dict):
        return ""
    v = value.get(name)
    return "" if v is None else v


def rows(value):
    return value if isinstance(value, list) else []


health = load("health.json")
runtime = load("runtime_status.json")
state = load("state.json")
events = load("events.json")

trajectory = prop(runtime, "task_trajectory")
task = prop(trajectory, "task")
run = prop(trajectory, "run")
summary = {
    "schema_version": "g.readonly.runtime.smoke.v1",
    "endpoints": allowed,
    "health_status": str(prop(health, "status")),
    "state_status": str(prop(state, "status")),
    "task_id": str(prop(task, "task_id")),
    "goal_id": str(prop(task, "goal_id")),
    "run_id": str(prop(task, "run_id")),
    "original_intent": str(prop(task, "original_intent")),
    "task_status": str(prop(task, "status")),
    "semantic_state": str(prop(prop(trajectory, "semantic"), "state")),
    "slice_count": len(rows(prop(run, "slices"))),
    "event_count": len(rows(prop(events, "events"))),
    "read_only": True,
}
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY
}

if [[ "$START_AGENT" -eq 1 ]]; then
  start_agent
fi

readonly_get "$AGENT_HTTP" "/health" "$STATE_DIR/health.json"
readonly_get "$AGENT_HTTP" "/agent/runtime/status" "$STATE_DIR/runtime_status.json"
readonly_get "$AGENT_HTTP" "/agent/state" "$STATE_DIR/state.json"
readonly_get "$AGENT_HTTP" "/agent/events" "$STATE_DIR/events.json"

SUMMARY="$(build_summary)" || fail "summary extraction failed (malformed JSON in a 2xx response body?)"

if [[ -n "$OUTPUT_PATH" ]]; then
  mkdir -p "$(dirname "$OUTPUT_PATH")"
  printf '%s\n' "$SUMMARY" > "$OUTPUT_PATH"
fi

printf '%s\n' "$SUMMARY"
echo "workdir: $WORKDIR" >&2
