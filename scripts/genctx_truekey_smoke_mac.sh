#!/usr/bin/env bash
# genctx_truekey_smoke_mac.sh — GEN-CTX-INJECT-1 true-key smoke (mac).
#
# Assembles a MusicContextSnapshot from a REAL kernel get_project_state
# response and asserts the tempo / time-signature linkage through
# agent/internal/genctx (env-gated test TestTrueKeySnapshotFromStateFile):
# kernel-direct ZMQ → set_tempo 96 (engine write) → get_project_state →
# snapshot assembly + prompt render + pre-KEYFIELD null-key semantics.
#
# Single kernel (the DISCLOSE candidate carries the music_context
# disclosure); state fully isolated under the run workdir; never touches
# processes it did not start (AGENTS §9/§10).
#
# Usage:
#   ./scripts/genctx_truekey_smoke_mac.sh --kernel-bin PATH [--workdir PATH]
#
# Exit 0 = all gates green.

set -euo pipefail

KERNEL_BIN_ARG=""
WORKDIR_ARG=""
STARTUP_TIMEOUT_SECONDS=90
ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

usage() { sed -n '2,16p' "$0"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
[[ -d "$REPO_ROOT/agent" ]] || { echo "repo root not found: $REPO_ROOT" >&2; exit 2; }
[[ -n "$KERNEL_BIN_ARG" && -x "$KERNEL_BIN_ARG" ]] || { echo "--kernel-bin PATH required (executable)" >&2; exit 2; }

RUN_ID="genctx_truekey_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/genctx_truekey_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{zmq,logs,project}
ARTIFACTS="$REPO_ROOT/coord/runs/GEN-CTX-INJECT-1"
mkdir -p "$ARTIFACTS"

KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"   # fake VitApp root (VitPaths.h climb)

KERNEL_PID=""
GATES="$WORKDIR/gates.txt"; : > "$GATES"

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }
gate() { printf '%s=%s\n' "$1" "$2" >> "$GATES"; [[ "$2" == "1" ]] || { log "GATE FAIL: $1"; exit 1; }; }

cleanup() {
  local rc=$?
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    kill -TERM "$KERNEL_PID" 2>/dev/null || true
    local i; for i in $(seq 1 10); do kill -0 "$KERNEL_PID" 2>/dev/null || break; sleep 1; done
    kill -0 "$KERNEL_PID" 2>/dev/null && kill -KILL "$KERNEL_PID" 2>/dev/null || true
  fi
  wait 2>/dev/null || true
  log "teardown done (rc=$rc)"
}
trap cleanup EXIT

port_listener_pid() { lsof -ti tcp:"$1" -sTCP:LISTEN 2>/dev/null | head -1; }

kzmq() { # kzmq <tag> <timeout-sec> <payload-json>
  local tag="$1" tmo="$2" payload="$3"
  python3 - "$WORKDIR/zmq/${tag}.json" "$ZMQ_REQ_ENDPOINT" "$tmo" "$payload" <<'PY'
import json, sys, zmq
out_path, endpoint, tmo, payload = sys.argv[1], sys.argv[2], int(sys.argv[3]), sys.argv[4]
ctx = zmq.Context.instance(); sock = ctx.socket(zmq.REQ)
sock.setsockopt(zmq.LINGER, 0); sock.setsockopt(zmq.RCVTIMEO, tmo*1000); sock.setsockopt(zmq.SNDTIMEO, tmo*1000)
sock.connect(endpoint)
try:
    sock.send_string(payload); raw = sock.recv_string()
except Exception as exc:
    sock.close(); print(json.dumps({"status":"transport_error","error":str(exc)})); raise SystemExit(0)
sock.close()
open(out_path,"w",encoding="utf-8").write(raw)
try: parsed = json.loads(raw)
except Exception: parsed = None
status = parsed.get("status","") if isinstance(parsed, dict) else ""
print(json.dumps({"status": status, "json": parsed}))
PY
}

# --- startup ---------------------------------------------------------------
for p in 7878 5555 5556 5557; do
  [[ -z "$(port_listener_pid "$p")" ]] || { echo "port $p busy — stack not free (MAC-RUNTIME-STACK single-owner)" >&2; exit 2; }
done

(
  cd "$KERNEL_ROOT"
  exec "$KERNEL_BIN_ARG"
) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
KERNEL_PID=$!

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  kill -0 "$KERNEL_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2; echo "kernel exited during startup" >&2; exit 2; }
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
  sleep 1
done
[[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] || { echo "kernel REQ port not up in ${STARTUP_TIMEOUT_SECONDS}s" >&2; exit 2; }
log "kernel up (pid $KERNEL_PID)"

# --- probes ----------------------------------------------------------------
R=$(kzmq s01_clear 60 '{"cmd":"clear_project"}')
gate kernel_clear_ok "$( [[ "$(echo "$R" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')" == "ok" ]] && echo 1 || echo 0 )"

R=$(kzmq s02_set_tempo_96 30 '{"cmd":"set_tempo","bpm":96}')
gate set_tempo_96_ok "$( [[ "$(echo "$R" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')" == "ok" ]] && echo 1 || echo 0 )"

R=$(kzmq s03_state 30 '{"cmd":"get_project_state"}')
gate state_ok "$( [[ "$(echo "$R" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("status",""))')" == "ok" ]] && echo 1 || echo 0 )"

# direct pre-assert: music_context carries the engine truth we just wrote
gate music_context_tempo_96 "$( python3 - "$WORKDIR/zmq/s03_state.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
mc=d.get("music_context") or {}
print(1 if abs(float(mc.get("tempo_bpm",-1))-96.0)<1e-9 else 0)
PY
)"

# --- env-gated true-key test through the package ---------------------------
export GENCTX_SMOKE_STATE="$WORKDIR/zmq/s03_state.json"
export GENCTX_SMOKE_TEMPO="96"
export GENCTX_SMOKE_TIMESIG="4/4"
(cd "$REPO_ROOT/agent" && go test ./internal/genctx -run TestTrueKeySnapshotFromStateFile -count=1 > "$WORKDIR/logs/truekey_go_test.log" 2>&1)
unset GENCTX_SMOKE_STATE GENCTX_SMOKE_TEMPO GENCTX_SMOKE_TIMESIG
gate truekey_go_test_ok 1
log "true-key go test PASS"

# --- verdict ---------------------------------------------------------------
PASS_DIR="$ARTIFACTS/${RUN_ID}-pass"
mkdir -p "$PASS_DIR"
cp "$WORKDIR"/zmq/*.json "$PASS_DIR/"
cp "$GATES" "$PASS_DIR/gates.txt"
cp "$WORKDIR/logs/truekey_go_test.log" "$PASS_DIR/" 2>/dev/null || true
{
  echo "run_id=$RUN_ID"
  echo "kernel_bin=$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN_ARG" | awk '{print "kernel_sha256="$1}'
  echo "repo_root=$REPO_ROOT"
  echo "gates=$(wc -l < "$GATES" | tr -d ' ') all_pass=1"
} > "$PASS_DIR/run_meta.txt"
log "SMOKE PASS — $RUN_ID (artifacts: $PASS_DIR)"
exit 0
