#!/usr/bin/env bash
# music_context_disclosure_smoke_mac.sh — GEN-CTX-DISCLOSE-1: music_context
# disclosure-key smoke on the mac real kernel stack (VitApp headless kernel,
# direct ZMQ REQ on tcp://127.0.0.1:5555 — the same command surface the agent
# consumes internally via kernel.SendCommand; the Godot frontend and the agent
# HTTP face are out of scope).
#
# Why direct ZMQ: the agent's user-visible get_project_state tool returns a
# CURATED projection (internal engine state stays hidden — catalog.go
# "project.state" spec), so a zero-agent-diff card asserts the disclosure on
# the kernel reply itself. The card-3 snapshot assembler is the future agent
# consumer and rides the same kernel command surface tested here.
#
# Verifies the get_project_state `music_context` disclosure block
# (tempo_bpm / tempo_setting_count / time_signature /
# time_signature_numerator / time_signature_denominator /
# time_sig_setting_count; key_signature / style_note null placeholders for
# card 2) against the pre-change kernel with a same-file, same-path snapshot
# diff:
#
#   P0  preflight    ports free (single-owner), both kernel binaries hashed
#                    into the run workdir (AGENTS §9)
#   P1  phase A      BASELINE kernel: new_project, save_as into the run
#                    workdir, set_tempo 128 (engine write), get_project_state
#                    (A_create snapshot), clear_project, open_project the
#                    saved file, get_project_state (A_open snapshot), teardown
#   P2  phase B      CANDIDATE kernel: clear_project, open_project the same
#                    saved file, get_project_state (B_open snapshot) — the
#                    six-key family gates run here (persisted tempo 128
#                    discloses, 4/4 family form, integer counts >= 1, null
#                    placeholders); set_tempo 96 then get_project_state again
#                    (B_after) — tempo_bpm must track the live engine write
#                    (the handler reads the same getTempo(0) surface
#                    set_tempo writes, so disclosed == reply == engine truth),
#                    teardown
#   P3  zero-change   deep snapshot diff: A_open vs B_open with music_context
#                    stripped from BOTH sides must be equal (sorted-key JSON
#                    walk); B_open may introduce AT MOST the music_context key
#                    over A_open and must remove nothing (existing keys
#                    unchanged, spec acceptance 2)
#   P4  close         summary JSON (schema music.context.disclosure.mac.v1),
#                    exit 0 iff every gate is green
#
# Both kernels run under the SAME fake VitApp root inside the run workdir
# (state isolation per AGENTS §10; the source tree is never touched) and the
# two phases take the identical clear->open command path on the identical
# project file, so the only difference in the P3 diff is the kernel binary.
# Failures are classified env (ports, toolchain, transport) vs functional
# (reply/snapshot assertions) per AGENTS §8. This script never stops
# processes it did not start (AGENTS §9 single-owner rule).
#
# Usage:
#   ./scripts/music_context_disclosure_smoke_mac.sh \
#        --baseline-kernel PATH --kernel-bin PATH
#
# The kernels are NOT built by this script — pass the pre-change baseline
# binary and the rebuilt candidate binary (sha256 recorded per AGENTS §9).
# music_context is stripped symmetrically from both snapshots before the
# value diff, so the script stays re-runnable post-merge with two same-era
# binaries (the keyset gate then reports an empty delta).

set -euo pipefail

KERNEL_BIN_DEFAULT=""
BASELINE_KERNEL_ARG=""
WORKDIR_ARG=""
OUTPUT_PATH=""
STARTUP_TIMEOUT_SECONDS=120
STOP_GRACE_SECONDS=10
COMMAND_TIMEOUT_DEFAULT=30
PORT_RELEASE_TIMEOUT_SECONDS=30

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555
RUN_TAG="music_ctx_disclose_mac.GEN-CTX-DISCLOSE-1"

usage() {
  cat <<'EOF'
music_context_disclosure_smoke_mac.sh — music_context disclosure-key smoke
(mac headless kernel via direct ZMQ, GEN-CTX-DISCLOSE-1).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --baseline-kernel PATH  PRE-change kernel binary (required; the zero-change
                          snapshot diff compares against this binary)
  --kernel-bin PATH       Candidate kernel binary (default: repo
                          VitApp/build/VitApp_artefacts/Debug/VitApp)
  --workdir PATH          Reuse PATH as the run workdir instead of a fresh
                          mktemp directory (never overwrites other runs)
  --startup-timeout SECONDS  Kernel startup timeout (default 120)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --output PATH           Also write the summary JSON to PATH
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --baseline-kernel) BASELINE_KERNEL_ARG="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
[[ -d "$REPO_ROOT" ]] || { echo "repo root not found: $REPO_ROOT" >&2; exit 2; }

# ---------------------------------------------------------------- run workdir
RUN_ID="music_ctx_disclose_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/music_ctx_disclose_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{zmq,logs,project}
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT"
# Fake VitApp root markers: VitPaths.h climbToVitAppRoot() anchors the kernel's
# Workspace/{Logs,Settings,Cache} + default_project.xml here (AGENTS §10).
# Both phases deliberately share this root so the only variable between the
# A_open and B_open snapshots is the kernel binary.
: > "$KERNEL_ROOT/CMakeLists.txt"
mkdir -p "$KERNEL_ROOT/Source"
PROJECT_FILE="$WORKDIR/project/music_ctx_probe.vit"

KERNEL_PID=""
KERNEL_STOP_RECORD=""
FAIL_CLASSIFICATION=""
GATE_FAIL_LOG="$WORKDIR/gate_failures.txt"
: > "$GATE_FAIL_LOG"

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

cleanup() {
  local rc=$?
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    kill -TERM "$KERNEL_PID" 2>/dev/null || true
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$KERNEL_PID" 2>/dev/null || break
      sleep 1
    done
    kill -0 "$KERNEL_PID" 2>/dev/null && kill -KILL "$KERNEL_PID" 2>/dev/null || true
  fi
  wait 2>/dev/null || true
  exit $rc
}
trap cleanup EXIT INT TERM

fail_env() {
  FAIL_CLASSIFICATION="env"
  log "ENV FAILURE: $*"
  exit 1
}
fail_functional() {
  FAIL_CLASSIFICATION="functional"
  log "FUNCTIONAL FAILURE: $*"
  exit 1
}

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

gate() {
  # gate <name> <0|1> <detail> — record a hard gate outcome (exported so the
  # summary python step can read every GATE_* from the environment).
  local name="$1" val="$2" detail="$3"
  eval "export GATE_${name}=$val"
  if [[ "$val" != "1" ]]; then
    echo "$name: $detail" >> "$GATE_FAIL_LOG"
    log "GATE FAIL: $name — $detail"
  else
    log "gate ok: $name"
  fi
}

assert_ports_free() {
  local port pid
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    pid="$(port_listener_pid "$port")"
    if [[ -n "$pid" ]]; then
      fail_env "port $port already has a listener (pid $pid); this harness never stops processes it did not start (AGENTS §9 single-owner rule)"
    fi
  done
}

wait_ports_released() {
  # After teardown of one phase, wait for the fixed ports to actually leave
  # LISTEN before the next phase binds them.
  local deadline=$((SECONDS + PORT_RELEASE_TIMEOUT_SECONDS)) port busy
  while (( SECONDS < deadline )); do
    busy=0
    for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy=1
    done
    (( busy == 0 )) && return 0
    sleep 1
  done
  fail_env "ports still listening ${PORT_RELEASE_TIMEOUT_SECONDS}s after teardown"
}

# ---------------------------------------------------------------- binaries
assert_ports_free

BASELINE_KERNEL="$BASELINE_KERNEL_ARG"
[[ -n "$BASELINE_KERNEL" ]] || fail_env "--baseline-kernel is required (the pre-change binary the zero-change snapshot diff compares against)"
[[ -x "$BASELINE_KERNEL" ]] || fail_env "baseline kernel binary is not executable: $BASELINE_KERNEL"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
[[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN (pass --kernel-bin; the kernel is NOT built by this smoke — GEN-MAC-BASELINE-1 §二② rebuilds it out-of-band and records the hash)"
shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
KERNEL_SHA="$(cut -d' ' -f1 "$WORKDIR/kernel_bin.sha256")"
shasum -a 256 "$BASELINE_KERNEL" > "$WORKDIR/baseline_kernel.sha256"
BASELINE_KERNEL_SHA="$(cut -d' ' -f1 "$WORKDIR/baseline_kernel.sha256")"
stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
stat -f "baseline_kernel_mtime=%Sm baseline_kernel_size=%z" "$BASELINE_KERNEL" > "$WORKDIR/baseline_kernel.stat"
log "candidate kernel: $KERNEL_BIN (sha256 $KERNEL_SHA)"
log "baseline kernel:  $BASELINE_KERNEL (sha256 $BASELINE_KERNEL_SHA)"
[[ "$KERNEL_SHA" != "$BASELINE_KERNEL_SHA" ]] \
  || fail_env "candidate and baseline kernels are the same binary — nothing to compare"

python3 -c 'import zmq' 2>/dev/null || fail_env "pyzmq not available for the direct kernel ZMQ probes"

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "baseline_kernel_bin=$BASELINE_KERNEL"
  echo "baseline_kernel_sha256=$BASELINE_KERNEL_SHA"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$KERNEL_SHA"
  echo "zmq_req=$ZMQ_REQ_ENDPOINT"
} > "$WORKDIR/run_meta.txt"

# ------------------------------------------------------------ kernel helpers
start_kernel() {
  # start_kernel <kernel-path> <phase-tag>
  local kern="$1" phase="$2"
  assert_ports_free
  log "[$phase] starting kernel (cwd=$KERNEL_ROOT, state isolated under the run workdir): $kern"
  (
    cd "$KERNEL_ROOT"
    exec "$kern"
  ) > "$WORKDIR/logs/${phase}_kernel_stdout.log" 2>&1 &
  KERNEL_PID=$!

  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS)) port listener_pid
  while (( SECONDS < deadline )); do
    if ! kill -0 "$KERNEL_PID" 2>/dev/null; then
      tail -30 "$WORKDIR/logs/${phase}_kernel_stdout.log" >&2
      fail_env "[$phase] kernel process exited during startup (see $WORKDIR/logs/${phase}_kernel_stdout.log)"
    fi
    [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
    sleep 1
  done
  if [[ -z "$(port_listener_pid "$KERNEL_PORT_REQ")" ]]; then
    tail -30 "$WORKDIR/logs/${phase}_kernel_stdout.log" >&2
    fail_env "[$phase] kernel ZMQ REQ port $KERNEL_PORT_REQ not listening within ${STARTUP_TIMEOUT_SECONDS}s (known intermittent: Tracktion device-manager init hang, PORT-SMOKE-MAC-4 env-class precedent — log above)"
  fi
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    listener_pid="$(port_listener_pid "$port")"
    [[ "$listener_pid" == "$KERNEL_PID" ]] \
      || fail_env "[$phase] port $port listener pid $listener_pid != kernel pid $KERNEL_PID"
  done
  export "GATE_${phase}_kernel_start_ports=1"
  log "[$phase] kernel ZMQ ports listening: $KERNEL_PORT_REQ (REQ) $ZMQ_PUB_PORT (PUB) $ZMQ_LOG_PORT (log) — owned by pid $KERNEL_PID"
}

STOP_RECORD=""
stop_kernel() {
  # stop_kernel <phase-tag>
  local phase="$1" start elapsed code signal="SIGTERM"
  log "[$phase] stopping kernel (pid $KERNEL_PID)..."
  kill -TERM "$KERNEL_PID" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$KERNEL_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -KILL "$KERNEL_PID" 2>/dev/null || true; signal="SIGTERM+SIGKILL"; break; }
    sleep 1
  done
  set +e
  wait "$KERNEL_PID" 2>/dev/null
  code=$?
  set -e
  elapsed=$((SECONDS - start))
  STOP_RECORD="${signal}:${code}:${elapsed}"
  export "${phase}_KERNEL_STOP"="$STOP_RECORD"
  log "[$phase] kernel stop: $STOP_RECORD (signal:exit_code:elapsed — SIGTERM expected to yield 143 without JUCE shutdown)"
  KERNEL_PID=""
  wait_ports_released
}

REPLY_FILE=""
kzmq() {
  # kzmq <tag> <timeout-sec> <payload-json> — direct ZMQ REQ to the kernel.
  # Saves the raw reply to $WORKDIR/zmq/<tag>.json and prints a one-line
  # verdict JSON: {"status": ..., "json": <parsed-or-null>}.
  local tag="$1" tmo="$2" payload="$3"
  REPLY_FILE="$WORKDIR/zmq/${tag}.json"
  python3 - "$REPLY_FILE" "$ZMQ_REQ_ENDPOINT" "$tmo" "$payload" <<'PY'
import json, sys, zmq

out_path, endpoint, tmo, payload = sys.argv[1], sys.argv[2], int(sys.argv[3]), sys.argv[4]
ctx = zmq.Context.instance()
sock = ctx.socket(zmq.REQ)
sock.setsockopt(zmq.LINGER, 0)
sock.setsockopt(zmq.RCVTIMEO, tmo * 1000)
sock.setsockopt(zmq.SNDTIMEO, tmo * 1000)
sock.connect(endpoint)
try:
    sock.send_string(payload)
    raw = sock.recv_string()
except Exception as exc:
    sock.close()
    print(json.dumps({"status": "transport_error", "error": str(exc)}))
    raise SystemExit(0)
finally:
    pass
sock.close()
with open(out_path, "w", encoding="utf-8") as f:
    f.write(raw)
try:
    parsed = json.loads(raw)
except Exception:
    parsed = None
status = parsed.get("status", "") if isinstance(parsed, dict) else ""
print(json.dumps({"status": status, "json": parsed}))
PY
}

K_STATUS=""
K_JSON=""
kquery() {
  # kquery <tag> <timeout-sec> <payload-json> — sets K_STATUS (kernel status
  # field) and K_JSON (verdict json line). Transport failure -> K_STATUS=transport_error.
  local line
  line="$(kzmq "$1" "$2" "$3")" || line='{"status": "transport_error"}'
  K_JSON="$line"
  K_STATUS="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("status",""))' "$line")"
}

kfield() {
  # kfield <verdict-json-line> <pyexpr against r=reply dict>
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    d = json.loads(sys.argv[1])
except Exception:
    print("")
    raise SystemExit
r = d.get("json") if isinstance(d.get("json"), dict) else {}
try:
    v = eval(sys.argv[2], {"r": r})
except Exception:
    v = None
print(json.dumps(v) if isinstance(v, (dict, list)) else v)
PY
}

run_gate() {
  # run_gate <gate-name> <tag> <timeout-sec> <payload-json> <expected-status>
  # Invokes kzmq and gates on the kernel status field.
  kquery "$2" "$3" "$4"
  gate "$1" "$( [[ "$K_STATUS" == "$5" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want $5)"
}

# ---------------------------------------------------------------- P1 phase A
log "P1: phase A — baseline kernel ($BASELINE_KERNEL) creates the probe project..."
start_kernel "$BASELINE_KERNEL" phaseA

run_gate phaseA_new_project_ok a01_new 30 '{"cmd":"new_project"}' ok

rm -f "$PROJECT_FILE"
run_gate phaseA_save_as_ok a02_save_as 60 "{\"cmd\":\"save_as_project\",\"file_path\":\"$PROJECT_FILE\"}" ok
if [[ -f "$PROJECT_FILE" && -s "$PROJECT_FILE" ]]; then
  gate phaseA_save_as_file 1 "file $(wc -c < "$PROJECT_FILE" | tr -d ' ')B"
else
  gate phaseA_save_as_file 0 "project file missing/empty at $PROJECT_FILE"
fi

kquery a03_set_tempo_128 30 '{"cmd":"set_tempo","bpm":128}'
A_SET_REPLY_BPM="$(kfield "$K_JSON" 'r.get("bpm")')"
gate phaseA_set_tempo_ok "$( [[ "$K_STATUS" == "ok" ]] && python3 -c 'import sys; sys.exit(0 if float(sys.argv[1])==128.0 else 1)' "${A_SET_REPLY_BPM:-x}" && echo 1 || echo 0 )" "status=$K_STATUS reply_bpm=$A_SET_REPLY_BPM"

kquery a04_state_create 30 '{"cmd":"get_project_state"}'
gate phaseA_state_create_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/a04_state_create.json" "$WORKDIR/zmq/A_create.json"

run_gate phaseA_clear_before_open_ok a05_clear 60 '{"cmd":"clear_project"}' ok
run_gate phaseA_open_project_ok a06_open 60 "{\"cmd\":\"open_project\",\"file_path\":\"$PROJECT_FILE\"}" ok

kquery a07_state_open 30 '{"cmd":"get_project_state"}'
gate phaseA_state_open_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/a07_state_open.json" "$WORKDIR/zmq/A_open.json"

stop_kernel phaseA
export PHASEA_KERNEL_STOP="$phaseA_KERNEL_STOP"

# ---------------------------------------------------------------- P2 phase B
log "P2: phase B — candidate kernel ($KERNEL_BIN) opens the same file..."
start_kernel "$KERNEL_BIN" phaseB

run_gate phaseB_clear_slate b01_clear 60 '{"cmd":"clear_project"}' ok
run_gate phaseB_open_project_ok b02_open 60 "{\"cmd\":\"open_project\",\"file_path\":\"$PROJECT_FILE\"}" ok

kquery b03_state_open 30 '{"cmd":"get_project_state"}'
gate phaseB_state_open_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b03_state_open.json" "$WORKDIR/zmq/B_open.json"

B_MC="$(kfield "$K_JSON" 'r.get("music_context")')"
if [[ -n "$B_MC" && "$B_MC" != "None" ]]; then
  gate phaseB_music_context_present 1 "music_context present in kernel reply"
  export B_MC_JSON="$(python3 -c 'import json,sys; print(json.dumps(json.loads(sys.argv[1])["json"]["music_context"], sort_keys=True))' "$K_JSON")"
  MC_TEMPO_OPEN="$(kfield "$K_JSON" 'r["music_context"].get("tempo_bpm")')"
  MC_TEMPO_COUNT="$(kfield "$K_JSON" 'r["music_context"].get("tempo_setting_count")')"
  MC_TS="$(kfield "$K_JSON" 'r["music_context"].get("time_signature")')"
  MC_NUM="$(kfield "$K_JSON" 'r["music_context"].get("time_signature_numerator")')"
  MC_DEN="$(kfield "$K_JSON" 'r["music_context"].get("time_signature_denominator")')"
  MC_TS_COUNT="$(kfield "$K_JSON" 'r["music_context"].get("time_sig_setting_count")')"
  MC_KEYSIG="$(kfield "$K_JSON" 'r["music_context"].get("key_signature")')"
  MC_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note")')"
  MC_TEMPO_ISNUM="$(kfield "$K_JSON" '1 if isinstance(r["music_context"].get("tempo_bpm"), (int,float)) and not isinstance(r["music_context"].get("tempo_bpm"), bool) else 0')"

  # Persisted engine truth: phase A wrote 128 through set_tempo (auto-persist)
  # and the .vit round-trip must disclose the same numeric value.
  gate phaseB_tempo_persisted_128 "$( python3 -c 'import sys; sys.exit(0 if float(sys.argv[1])==128.0 else 1)' "${MC_TEMPO_OPEN:-x}" && echo 1 || echo 0 )" "tempo_bpm=$MC_TEMPO_OPEN (want 128 from the phase-A persisted write)"
  gate phaseB_tempo_is_number "$MC_TEMPO_ISNUM" "tempo_bpm json type is numeric (serialized form: $MC_TEMPO_OPEN)"
  # Time-signature family form: "4/4" string + 4/4 numerator/denominator.
  gate phaseB_time_signature_form "$( python3 -c 'import sys; ts,n,d=sys.argv[1],sys.argv[2],sys.argv[3];
sys.exit(0 if ts=="4/4" and float(n)==4.0 and float(d)==4.0 else 1)' "$MC_TS" "$MC_NUM" "$MC_DEN" && echo 1 || echo 0 )" "time_signature=$MC_TS numerator=$MC_NUM denominator=$MC_DEN"
  # Automation counts: >= 1 each (TempoSequence auto-inserts a default setting,
  # so a status=ok reply always carries engine truth here).
  gate phaseB_setting_counts "$( python3 -c 'import sys; sys.exit(0 if int(sys.argv[1])>=1 and int(sys.argv[2])>=1 else 1)' "$MC_TEMPO_COUNT" "$MC_TS_COUNT" && echo 1 || echo 0 )" "tempo_setting_count=$MC_TEMPO_COUNT time_sig_setting_count=$MC_TS_COUNT"
  # Card-2 placeholders: key_signature null, style_note "" (unset semantics).
  gate phaseB_placeholders_null "$( [[ "$MC_KEYSIG" == "None" && "$MC_STYLE" == "" ]] && echo 1 || echo 0 )" "key_signature=$MC_KEYSIG style_note='$MC_STYLE'"
else
  gate phaseB_music_context_present 0 "music_context absent from kernel reply"
  gate phaseB_tempo_persisted_128 0 "music_context absent"
  gate phaseB_tempo_is_number 0 "music_context absent"
  gate phaseB_time_signature_form 0 "music_context absent"
  gate phaseB_setting_counts 0 "music_context absent"
  gate phaseB_placeholders_null 0 "music_context absent"
  export B_MC_JSON=""
fi

kquery b04_set_tempo_96 30 '{"cmd":"set_tempo","bpm":96}'
B_SET_REPLY_BPM="$(kfield "$K_JSON" 'r.get("bpm")')"
gate phaseB_set_tempo_ok "$( [[ "$K_STATUS" == "ok" ]] && python3 -c 'import sys; sys.exit(0 if float(sys.argv[1])==96.0 else 1)' "${B_SET_REPLY_BPM:-x}" && echo 1 || echo 0 )" "status=$K_STATUS reply_bpm=$B_SET_REPLY_BPM"

kquery b05_state_after 30 '{"cmd":"get_project_state"}'
gate phaseB_state_after_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b05_state_after.json" "$WORKDIR/zmq/B_after.json"

MC_TEMPO_AFTER="$(kfield "$K_JSON" 'r["music_context"].get("tempo_bpm") if isinstance(r.get("music_context"), dict) else "<absent>"')"
# Live-write linkage: the disclosed tempo must equal the set_tempo reply value
# numerically — both sides read/write the same engine surface (getTempo(0)),
# so equality here is the script-level proof of engine-truth consistency.
gate phaseB_tempo_tracks_set_96 "$( python3 -c 'import sys; sys.exit(0 if float(sys.argv[1])==96.0 and float(sys.argv[2])==96.0 else 1)' "$MC_TEMPO_AFTER" "${B_SET_REPLY_BPM:-x}" && echo 1 || echo 0 )" "tempo_bpm_after=$MC_TEMPO_AFTER set_tempo_reply=$B_SET_REPLY_BPM (want 96/96)"

stop_kernel phaseB
export PHASEB_KERNEL_STOP="$phaseB_KERNEL_STOP"

# ------------------------------------------------- P3 zero-change snapshot diff
log "P3: zero-change snapshot diff (A_open vs B_open, music_context stripped from both)..."
SNAPSHOT_DIFF_VERDICT="$(python3 - "$WORKDIR/zmq/A_open.json" "$WORKDIR/zmq/B_open.json" "$WORKDIR/snapshot_diff.txt" <<'PY'
import json, sys

def kernel_reply(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)

a = kernel_reply(sys.argv[1])
b = kernel_reply(sys.argv[2])
out = open(sys.argv[3], "w", encoding="utf-8")
# Strip symmetrically: the compared surface is every PRE-EXISTING key; the
# new-key presence is asserted by the keyset gate below.
a.pop("music_context", None)
b.pop("music_context", None)
# Documented volatile paths, normalized from BOTH sides before the walk (each
# entry says why it cannot be stable across two kernel launches):
#   $.plugin_list_hygiene.completed_at — TIM-KERNEL-HYGIENE-1 stamps the
#     startup plugin-list cleanup run with the wall clock; it differs per
#     kernel launch by design and carries zero project semantics.
VOLATILE_PATHS = {"$.plugin_list_hygiene.completed_at"}
normalized = []
for path in sorted(VOLATILE_PATHS):
    parts = path[2:].split(".")
    removed = []
    for doc in (a, b):
        cur = doc
        for p in parts[:-1]:
            if isinstance(cur, dict) and p in cur:
                cur = cur[p]
            else:
                cur = None
                break
        if isinstance(cur, dict) and parts[-1] in cur:
            del cur[parts[-1]]
            removed.append(True)
    if any(removed):
        normalized.append(path)

def walk(x, y, path, diffs):
    if isinstance(x, dict) and isinstance(y, dict):
        for k in sorted(set(x) | set(y)):
            if k not in x:
                diffs.append(f"+ {path}.{k} = {json.dumps(y[k], sort_keys=True)}")
            elif k not in y:
                diffs.append(f"- {path}.{k} = {json.dumps(x[k], sort_keys=True)}")
            else:
                walk(x[k], y[k], f"{path}.{k}", diffs)
    elif isinstance(x, list) and isinstance(y, list):
        if len(x) != len(y):
            diffs.append(f"~ {path}: list length {len(x)} -> {len(y)}")
        else:
            for i, (xi, yi) in enumerate(zip(x, y)):
                walk(xi, yi, f"{path}[{i}]", diffs)
    elif x != y:
        diffs.append(f"~ {path}: {json.dumps(x, sort_keys=True)} -> {json.dumps(y, sort_keys=True)}")

diffs = []
walk(a, b, "$", diffs)
for line in diffs:
    out.write(line + "\n")
out.close()

new_keys = sorted(set(b) - set(a))
removed_keys = sorted(set(a) - set(b))
sys.stdout.write(json.dumps({
    "deep_equal": len(diffs) == 0,
    "diff_count": len(diffs),
    "new_top_level_keys": new_keys,
    "removed_top_level_keys": removed_keys,
    "keyset_delta_exact": set(new_keys) <= {"music_context"} and not removed_keys,
    "normalized_volatile_paths": normalized,
}))
PY
)"
echo "snapshot_diff_verdict=$SNAPSHOT_DIFF_VERDICT" | tee "$WORKDIR/snapshot_diff.verdict"
DEEP_EQUAL="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1].split("=",1)[1])["deep_equal"])' "$(cat "$WORKDIR/snapshot_diff.verdict")")"
KEYSET_EXACT="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1].split("=",1)[1])["keyset_delta_exact"])' "$(cat "$WORKDIR/snapshot_diff.verdict")")"
DIFF_COUNT="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1].split("=",1)[1])["diff_count"])' "$(cat "$WORKDIR/snapshot_diff.verdict")")"
gate existing_keys_unchanged "$( [[ "$DEEP_EQUAL" == "True" ]] && echo 1 || echo 0 )" "deep_equal=$DEEP_EQUAL diff_count=$DIFF_COUNT (details: $WORKDIR/snapshot_diff.txt)"
gate keyset_delta_music_context_only "$( [[ "$KEYSET_EXACT" == "True" ]] && echo 1 || echo 0 )" "keyset_delta_exact=$KEYSET_EXACT"

# ---------------------------------------------------------------- P4 summary
SUMMARY_FILE="$WORKDIR/summary.json"
GATE_NAMES="phaseA_kernel_start_ports phaseA_new_project_ok phaseA_save_as_ok phaseA_save_as_file phaseA_set_tempo_ok phaseA_state_create_ok \
phaseA_clear_before_open_ok phaseA_open_project_ok phaseA_state_open_ok \
phaseB_kernel_start_ports phaseB_clear_slate phaseB_open_project_ok phaseB_state_open_ok \
phaseB_music_context_present phaseB_tempo_persisted_128 phaseB_tempo_is_number phaseB_time_signature_form phaseB_setting_counts phaseB_placeholders_null \
phaseB_set_tempo_ok phaseB_state_after_ok phaseB_tempo_tracks_set_96 existing_keys_unchanged keyset_delta_music_context_only"
export WORKDIR SUMMARY_FILE RUN_ID GATE_NAMES
export PHASEA_KERNEL_STOP PHASEB_KERNEL_STOP
export FAIL_CLASSIFICATION
export BASELINE_KERNEL_SHA KERNEL_SHA REPO_ROOT
export SNAPSHOT_DIFF_VERDICT B_MC_JSON MC_TEMPO_OPEN MC_TS MC_NUM MC_DEN MC_TEMPO_COUNT MC_TS_COUNT MC_TEMPO_AFTER
export A_SET_REPLY_BPM B_SET_REPLY_BPM PROJECT_FILE
python3 - <<'PY'
import json, os

workdir = os.environ["WORKDIR"]
gate_names = os.environ["GATE_NAMES"].split()
gates = {}
fail_details = {}
fail_log = os.path.join(workdir, "gate_failures.txt")
if os.path.exists(fail_log):
    for line in open(fail_log, encoding="utf-8"):
        line = line.strip()
        if ":" in line:
            name, detail = line.split(":", 1)
            fail_details[name] = detail
for name in gate_names:
    gates[name] = os.environ.get("GATE_" + name, "0") == "1"

try:
    snapshot_verdict = json.loads(os.environ["SNAPSHOT_DIFF_VERDICT"])
except Exception:
    snapshot_verdict = {"parse_error": os.environ.get("SNAPSHOT_DIFF_VERDICT", "")}
try:
    music_context = json.loads(os.environ["B_MC_JSON"]) if os.environ.get("B_MC_JSON") else None
except Exception:
    music_context = None

summary = {
    "schema_version": "music.context.disclosure.mac.v1",
    "card": "GEN-CTX-DISCLOSE-1",
    "run_id": os.environ["RUN_ID"],
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": os.environ.get("FAIL_CLASSIFICATION") or ("none" if all(gates.values()) else "functional"),
    "repo": {"root": os.environ["REPO_ROOT"]},
    "binaries": {
        "baseline_kernel_sha256": os.environ["BASELINE_KERNEL_SHA"],
        "kernel_sha256": os.environ["KERNEL_SHA"],
    },
    "probe_surface": "kernel direct ZMQ REQ tcp://127.0.0.1:5555 (the agent user-visible tool projects/filters the kernel reply; zero-agent-diff card asserts on the kernel reply)",
    "gates": gates,
    "gate_failure_details": fail_details,
    "music_context_open": music_context,
    "set_tempo_linkage": {
        "phaseA_set_128_reply_bpm": os.environ.get("A_SET_REPLY_BPM", ""),
        "phaseB_open_disclosed_bpm": os.environ.get("MC_TEMPO_OPEN", ""),
        "phaseB_set_96_reply_bpm": os.environ.get("B_SET_REPLY_BPM", ""),
        "phaseB_after_disclosed_bpm": os.environ.get("MC_TEMPO_AFTER", ""),
    },
    "zero_change_snapshot_diff": snapshot_verdict,
    "teardown": {
        "phaseA_kernel": os.environ.get("PHASEA_KERNEL_STOP", ""),
        "phaseB_kernel": os.environ.get("PHASEB_KERNEL_STOP", ""),
    },
    "project_file": os.environ.get("PROJECT_FILE", ""),
}

path = os.environ["SUMMARY_FILE"]
with open(path, "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2, sort_keys=True)
print(json.dumps({"overall_status": summary["overall_status"], "gates": gates}, indent=2, sort_keys=True))
PY

if [[ -n "$OUTPUT_PATH" ]]; then
  cp "$SUMMARY_FILE" "$OUTPUT_PATH"
fi

ALL_GREEN=1
for name in $GATE_NAMES; do
  eval "val=\${GATE_${name}:-0}"
  [[ "$val" == "1" ]] || ALL_GREEN=0
done
if (( ALL_GREEN == 1 )); then
  log "SMOKE PASS — $RUN_ID (summary: $SUMMARY_FILE)"
  exit 0
fi
fail_functional "one or more gates red (summary: $SUMMARY_FILE; details: $WORKDIR/gate_failures.txt)"
