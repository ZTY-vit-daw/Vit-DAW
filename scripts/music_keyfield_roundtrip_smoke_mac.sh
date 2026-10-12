#!/usr/bin/env bash
# music_keyfield_roundtrip_smoke_mac.sh — GEN-CTX-KEYFIELD-1: project key /
# style-note field persistence smoke on the mac real kernel stack (VitApp
# headless kernel, direct ZMQ REQ on tcp://127.0.0.1:5555 — the same command
# surface the agent consumes internally via kernel.SendCommand; the Godot
# frontend and the agent HTTP face are out of scope).
#
# Verifies the full chain of the two GEN-CTX-KEYFIELD-1 commands against the
# pre-change kernel baseline (the .vit files that matter are created by the
# BASELINE binary, so "old project" genuinely predates the new fields):
#
#   P0  preflight    ports free (single-owner), both kernel binaries hashed
#                    into the run workdir (AGENTS §9)
#   P1  phase A      BASELINE kernel: new_project, save_as into the run
#                    workdir (the OLD project — saved by a binary that knows
#                    nothing about vit_music_*), get_project_state (baseline
#                    disclosure carries null/"" placeholders), teardown
#   P2  phase B      CANDIDATE kernel, §11 round-trip trilogy + negatives:
#                    R1 old-project default fidelity: open the old .vit →
#                       key_signature null / style_note "" / no debug keys →
#                       save_project in place → reopen → defaults survive
#                       (no migration write, no property fabrication);
#                    R2 set → auto-persist → reload: set_project_key
#                       {root:"A", scale:"minor"} + set_project_style_note →
#                       disclosed music_context values equal the set-time
#                       echo (deep compare incl. set_at) → clear+reopen →
#                       byte-fidelity through the auto-persist save path;
#                    R3 save_as new path: save_as_project copy.vit → clear →
#                       open copy → same fidelity through the .vit container;
#                    NEG fail-closed probes: scale "ionian" (legal control)
#                       lands; scale "dorian23" / root 150 / root "H" /
#                       501-char style note are rejected with status=error
#                       AND leave the stored state unchanged;
#                    CLR disclosure linkage: clear key → null with NO debug
#                       keys, clear style → "", idempotent re-clear ok;
#                    tempo family regression guard (DISCLOSE-1 keys intact).
#   P3  close        summary JSON (schema music.keyfield.roundtrip.mac.v1),
#                    exit 0 iff every gate is green
#
# Read-side debug branches (key_signature_raw on unknown scale,
# key_signature_parse_error on corrupt JSON) are defensive against
# hand-edited state files and cannot be reached through the command surface
# (the write side is fail-closed), so they are out of scope for this probe.
#
# Usage:
#   ./scripts/music_keyfield_roundtrip_smoke_mac.sh \
#        --baseline-kernel PATH --kernel-bin PATH
#
# The kernels are NOT built by this script — pass the pre-change baseline
# binary and the rebuilt candidate binary (sha256 recorded per AGENTS §9).

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
RUN_TAG="music_keyfield_mac.GEN-CTX-KEYFIELD-1"

usage() {
  cat <<'EOF'
music_keyfield_roundtrip_smoke_mac.sh — key/style field persistence smoke
(mac headless kernel via direct ZMQ, GEN-CTX-KEYFIELD-1).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --baseline-kernel PATH  PRE-change kernel binary (required; creates the old
                          project the §11 default-fidelity leg reopens)
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
RUN_ID="music_keyfield_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/music_keyfield_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{zmq,logs,project}
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT"
# Fake VitApp root markers: VitPaths.h climbToVitAppRoot() anchors the kernel's
# Workspace/{Logs,Settings,Cache} + default_project.xml here (AGENTS §10).
# Both phases deliberately share this root so state isolation holds.
: > "$KERNEL_ROOT/CMakeLists.txt"
mkdir -p "$KERNEL_ROOT/Source"
PROJECT_FILE="$WORKDIR/project/keyfield_probe.vit"
COPY_FILE="$WORKDIR/project/keyfield_copy.vit"

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
  # gate <name> <0|1> <detail>
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
[[ -n "$BASELINE_KERNEL" ]] || fail_env "--baseline-kernel is required (the pre-change binary that authors the old project)"
[[ -x "$BASELINE_KERNEL" ]] || fail_env "baseline kernel binary is not executable: $BASELINE_KERNEL"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
[[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN (pass --kernel-bin; the kernel is NOT built by this smoke)"
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
  # kquery <tag> <timeout-sec> <payload-json>
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
    v = eval(sys.argv[2], {"r": r, "json": json})
except Exception:
    v = None
print(json.dumps(v) if isinstance(v, (dict, list)) else v)
PY
}

run_gate() {
  # run_gate <gate-name> <tag> <timeout-sec> <payload-json> <expected-status>
  kquery "$2" "$3" "$4"
  gate "$1" "$( [[ "$K_STATUS" == "$5" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want $5)"
}

mc_key_json() {
  # mc_key_json <verdict-json-line> — canonical dump of music_context.key_signature
  python3 - "$1" <<'PY'
import json, sys
try:
    d = json.loads(sys.argv[1])
    mc = d["json"]["music_context"]
    ks = mc.get("key_signature")
    print(json.dumps(ks, sort_keys=True, ensure_ascii=False) if ks is not None else "None")
except Exception:
    print("<absent>")
PY
}

# ---------------------------------------------------------------- P1 phase A
log "P1: phase A — baseline kernel ($BASELINE_KERNEL) authors the OLD project..."
start_kernel "$BASELINE_KERNEL" phaseA

run_gate phaseA_new_project_ok a01_new 30 '{"cmd":"new_project"}' ok

rm -f "$PROJECT_FILE"
run_gate phaseA_save_as_ok a02_save_as 60 "{\"cmd\":\"save_as_project\",\"file_path\":\"$PROJECT_FILE\"}" ok
if [[ -f "$PROJECT_FILE" && -s "$PROJECT_FILE" ]]; then
  gate phaseA_save_as_file 1 "file $(wc -c < "$PROJECT_FILE" | tr -d ' ')B"
else
  gate phaseA_save_as_file 0 "project file missing/empty at $PROJECT_FILE"
fi

kquery a03_state 30 '{"cmd":"get_project_state"}'
gate phaseA_state_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
A_MC_KEY="$(kfield "$K_JSON" 'r["music_context"].get("key_signature") if isinstance(r.get("music_context"), dict) else "<absent>"')"
A_MC_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseA_baseline_placeholders "$( [[ "$A_MC_KEY" == "None" && "$A_MC_STYLE" == "" ]] && echo 1 || echo 0 )" "baseline key_signature=$A_MC_KEY style_note='$A_MC_STYLE'"

stop_kernel phaseA
export PHASEA_KERNEL_STOP="$phaseA_KERNEL_STOP"

# ---------------------------------------------------------------- P2 phase B
log "P2: phase B — candidate kernel ($KERNEL_BIN): §11 round-trip trilogy + negatives..."
start_kernel "$KERNEL_BIN" phaseB

run_gate phaseB_clear_slate b00_clear 60 '{"cmd":"clear_project"}' ok

# ---- R1: old project (no vit_music_* properties) default fidelity --------
run_gate phaseB_open_old_ok b01_open_old 60 "{\"cmd\":\"open_project\",\"file_path\":\"$PROJECT_FILE\"}" ok

kquery b02_state_old 30 '{"cmd":"get_project_state"}'
gate phaseB_state_old_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b02_state_old.json" "$WORKDIR/zmq/B_r1_open1.json"
R1_KEY="$(mc_key_json "$K_JSON")"
R1_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
R1_DEBUG="$(kfield "$K_JSON" 'sorted(k for k in r["music_context"] if k.startswith("key_signature_")) if isinstance(r.get("music_context"), dict) else ["<absent>"]')"
gate phaseB_r1_defaults "$( [[ "$R1_KEY" == "None" && "$R1_STYLE" == "" && "$R1_DEBUG" == "[]" ]] && echo 1 || echo 0 )" "key=$R1_KEY style='$R1_STYLE' debug_keys=$R1_DEBUG"

run_gate phaseB_r1_save_in_place_ok b03_save 60 '{"cmd":"save_project"}' ok
run_gate phaseB_r1_close_ok b04_clear 60 '{"cmd":"clear_project"}' ok
run_gate phaseB_r1_reopen_ok b05_reopen 60 "{\"cmd\":\"open_project\",\"file_path\":\"$PROJECT_FILE\"}" ok

kquery b06_state_reopen 30 '{"cmd":"get_project_state"}'
gate phaseB_r1_state_reopen_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b06_state_reopen.json" "$WORKDIR/zmq/B_r1_open2.json"
R1B_KEY="$(mc_key_json "$K_JSON")"
R1B_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
R1B_DEBUG="$(kfield "$K_JSON" 'sorted(k for k in r["music_context"] if k.startswith("key_signature_")) if isinstance(r.get("music_context"), dict) else ["<absent>"]')"
gate phaseB_r1_defaults_survive_save "$( [[ "$R1B_KEY" == "None" && "$R1B_STYLE" == "" && "$R1B_DEBUG" == "[]" ]] && echo 1 || echo 0 )" "after save+reopen: key=$R1B_KEY style='$R1B_STYLE' debug_keys=$R1B_DEBUG"

# ---- R2: set → auto-persist → reload (byte fidelity) ----------------------
STYLE_TEXT='温暖模拟流行：侧链泵动的垫底 + 干净的顶线 (warm analog pop, sidechain-pumped pads)'
kquery b07_set_key 30 '{"cmd":"set_project_key","root":"A","scale":"minor"}'
SET_KEY_ECHO="$(kfield "$K_JSON" 'json.dumps(r.get("key_signature"), sort_keys=True, ensure_ascii=False) if isinstance(r.get("key_signature"), dict) else "None"')"
SET_KEY_FIELDS_OK="$(kfield "$K_JSON" '1 if r.get("key_signature", {}).get("root_pitch") == 9 and r.get("key_signature", {}).get("root_name") == "A" and r.get("key_signature", {}).get("scale") == "minor" and r.get("key_signature", {}).get("set_by") == "user" and isinstance(r.get("key_signature", {}).get("set_at"), str) and r["key_signature"]["set_at"] else 0')"
gate phaseB_r2_set_key_ok "$( [[ "$K_STATUS" == "ok" && "$SET_KEY_FIELDS_OK" == "1" ]] && echo 1 || echo 0 )" "status=$K_STATUS echo=$SET_KEY_ECHO"

kquery b08_set_style 30 "$(python3 -c 'import json,sys; print(json.dumps({"cmd":"set_project_style_note","text":sys.argv[1]}))' "$STYLE_TEXT")"
SET_STYLE_ECHO="$(kfield "$K_JSON" 'r.get("style_note") if isinstance(r.get("style_note"), str) else "None"')"
gate phaseB_r2_set_style_ok "$( [[ "$K_STATUS" == "ok" && "$SET_STYLE_ECHO" == "$STYLE_TEXT" ]] && echo 1 || echo 0 )" "status=$K_STATUS echo='$SET_STYLE_ECHO'"

kquery b09_state_set 30 '{"cmd":"get_project_state"}'
gate phaseB_r2_state_set_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
R2_DISCLOSE_KEY="$(mc_key_json "$K_JSON")"
R2_DISCLOSE_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_r2_disclose_matches_set "$( [[ "$R2_DISCLOSE_KEY" == "$SET_KEY_ECHO" && "$R2_DISCLOSE_STYLE" == "$STYLE_TEXT" ]] && echo 1 || echo 0 )" "disclosed_key=$R2_DISCLOSE_KEY echo_key=$SET_KEY_ECHO style_match=$([[ "$R2_DISCLOSE_STYLE" == "$STYLE_TEXT" ]] && echo yes || echo no)"
cp "$WORKDIR/zmq/b09_state_set.json" "$WORKDIR/zmq/B_r2_set.json"

run_gate phaseB_r2_close_ok b10_clear 60 '{"cmd":"clear_project"}' ok
run_gate phaseB_r2_reopen_ok b11_reopen 60 "{\"cmd\":\"open_project\",\"file_path\":\"$PROJECT_FILE\"}" ok

kquery b12_state_reload 30 '{"cmd":"get_project_state"}'
gate phaseB_r2_state_reload_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b12_state_reload.json" "$WORKDIR/zmq/B_r2_reload.json"
R2R_KEY="$(mc_key_json "$K_JSON")"
R2R_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_r2_autopersist_fidelity "$( [[ "$R2R_KEY" == "$SET_KEY_ECHO" && "$R2R_STYLE" == "$STYLE_TEXT" ]] && echo 1 || echo 0 )" "after auto-persist reload: key=$R2R_KEY (echo=$SET_KEY_ECHO) style_match=$([[ "$R2R_STYLE" == "$STYLE_TEXT" ]] && echo yes || echo no)"

# ---- R3: save_as new path (.vit container) roundtrip ----------------------
rm -f "$COPY_FILE"
run_gate phaseB_r3_save_as_ok b13_save_as 60 "{\"cmd\":\"save_as_project\",\"file_path\":\"$COPY_FILE\"}" ok
if [[ -f "$COPY_FILE" && -s "$COPY_FILE" ]]; then
  gate phaseB_r3_copy_file 1 "file $(wc -c < "$COPY_FILE" | tr -d ' ')B"
else
  gate phaseB_r3_copy_file 0 "copy file missing/empty at $COPY_FILE"
fi
run_gate phaseB_r3_close_ok b14_clear 60 '{"cmd":"clear_project"}' ok
run_gate phaseB_r3_open_copy_ok b15_open_copy 60 "{\"cmd\":\"open_project\",\"file_path\":\"$COPY_FILE\"}" ok

kquery b16_state_copy 30 '{"cmd":"get_project_state"}'
gate phaseB_r3_state_copy_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b16_state_copy.json" "$WORKDIR/zmq/B_r3_copy.json"
R3_KEY="$(mc_key_json "$K_JSON")"
R3_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_r3_save_as_fidelity "$( [[ "$R3_KEY" == "$SET_KEY_ECHO" && "$R3_STYLE" == "$STYLE_TEXT" ]] && echo 1 || echo 0 )" "after save_as reload: key=$R3_KEY (echo=$SET_KEY_ECHO) style_match=$([[ "$R3_STYLE" == "$STYLE_TEXT" ]] && echo yes || echo no)"

# ---- NEG: fail-closed probes (state currently A/minor + style text) -------
kquery b17_neg_ionian 30 '{"cmd":"set_project_key","root":"D","scale":"ionian"}'
IONIAN_ECHO="$(kfield "$K_JSON" 'json.dumps(r.get("key_signature"), sort_keys=True, ensure_ascii=False) if isinstance(r.get("key_signature"), dict) else "None"')"
gate phaseB_neg_ionian_passes "$( [[ "$K_STATUS" == "ok" ]] && kfield "$K_JSON" '1 if r.get("key_signature", {}).get("root_pitch") == 2 and r["key_signature"].get("scale") == "ionian" else 0' | grep -q 1 && echo 1 || echo 0 )" "status=$K_STATUS echo=$IONIAN_ECHO (legal control)"

kquery b18_neg_scale 30 '{"cmd":"set_project_key","root":"E","scale":"dorian23"}'
gate phaseB_neg_scale_dorian23_error "$( [[ "$K_STATUS" == "error" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want error)"
kquery b19_neg_root_hi 30 '{"cmd":"set_project_key","root":150,"scale":"dorian"}'
gate phaseB_neg_root_150_error "$( [[ "$K_STATUS" == "error" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want error)"
kquery b20_neg_root_name 30 '{"cmd":"set_project_key","root":"H","scale":"major"}'
gate phaseB_neg_root_H_error "$( [[ "$K_STATUS" == "error" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want error)"
OVERFLOW_TEXT="$(python3 -c 'print("x" * 501)')"
kquery b21_neg_style_len 30 "$(python3 -c 'import json,sys; print(json.dumps({"cmd":"set_project_style_note","text":sys.argv[1]}))' "$OVERFLOW_TEXT")"
gate phaseB_neg_style_501_error "$( [[ "$K_STATUS" == "error" ]] && echo 1 || echo 0 )" "status=$K_STATUS (want error, 501 chars)"

kquery b22_state_after_negs 30 '{"cmd":"get_project_state"}'
gate phaseB_neg_state_unchanged_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
cp "$WORKDIR/zmq/b22_state_after_negs.json" "$WORKDIR/zmq/B_after_negs.json"
NEG_KEY="$(mc_key_json "$K_JSON")"
NEG_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_neg_state_unchanged "$( [[ "$NEG_KEY" == "$IONIAN_ECHO" && "$NEG_STYLE" == "$STYLE_TEXT" ]] && echo 1 || echo 0 )" "key=$NEG_KEY (want ionian echo=$IONIAN_ECHO) style_match=$([[ "$NEG_STYLE" == "$STYLE_TEXT" ]] && echo yes || echo no)"

# ---- CLR: clear linkage + idempotence --------------------------------------
kquery b23_clear_key 30 '{"cmd":"set_project_key","clear":true}'
gate phaseB_clr_key_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
kquery b24_state_clr_key 30 '{"cmd":"get_project_state"}'
CLR_KEY="$(mc_key_json "$K_JSON")"
CLR_DEBUG="$(kfield "$K_JSON" 'sorted(k for k in r["music_context"] if k.startswith("key_signature_")) if isinstance(r.get("music_context"), dict) else ["<absent>"]')"
gate phaseB_clr_key_null "$( [[ "$K_STATUS" == "ok" && "$CLR_KEY" == "None" && "$CLR_DEBUG" == "[]" ]] && echo 1 || echo 0 )" "key=$CLR_KEY debug_keys=$CLR_DEBUG"

kquery b25_clear_style 30 '{"cmd":"set_project_style_note","clear":true}'
gate phaseB_clr_style_ok "$( [[ "$K_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$K_STATUS"
kquery b26_state_clr_style 30 '{"cmd":"get_project_state"}'
CLR_STYLE="$(kfield "$K_JSON" 'r["music_context"].get("style_note") if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_clr_style_empty "$( [[ "$K_STATUS" == "ok" && "$CLR_STYLE" == "" ]] && echo 1 || echo 0 )" "style='$CLR_STYLE'"

run_gate phaseB_clr_key_idempotent_ok b27_clear_key2 30 '{"cmd":"set_project_key","clear":true}' ok

# tempo family regression guard (DISCLOSE-1 keys must stay intact)
kquery b28_state_final 30 '{"cmd":"get_project_state"}'
cp "$WORKDIR/zmq/b28_state_final.json" "$WORKDIR/zmq/B_final.json"
T_TEMPO="$(kfield "$K_JSON" 'r["music_context"].get("tempo_bpm") if isinstance(r.get("music_context"), dict) else "<absent>"')"
T_TS="$(kfield "$K_JSON" 'r["music_context"].get("time_signature") if isinstance(r.get("music_context"), dict) else "<absent>"')"
T_COUNTS="$(kfield "$K_JSON" '(r["music_context"].get("tempo_setting_count"), r["music_context"].get("time_sig_setting_count")) if isinstance(r.get("music_context"), dict) else "<absent>"')"
gate phaseB_tempo_family_intact "$( python3 -c 'import sys; t,ts,c=sys.argv[1],sys.argv[2],sys.argv[3];
sys.exit(0 if isinstance(float(t), float) and ts=="4/4" and "1" in c else 1)' "$T_TEMPO" "$T_TS" "$T_COUNTS" && echo 1 || echo 0 )" "tempo_bpm=$T_TEMPO time_signature=$T_TS counts=$T_COUNTS"

stop_kernel phaseB
export PHASEB_KERNEL_STOP="$phaseB_KERNEL_STOP"

# ---------------------------------------------------------------- P3 summary
SUMMARY_FILE="$WORKDIR/summary.json"
GATE_NAMES="phaseA_kernel_start_ports phaseA_new_project_ok phaseA_save_as_ok phaseA_save_as_file phaseA_state_ok phaseA_baseline_placeholders \
phaseB_kernel_start_ports phaseB_clear_slate \
phaseB_open_old_ok phaseB_state_old_ok phaseB_r1_defaults phaseB_r1_save_in_place_ok phaseB_r1_close_ok phaseB_r1_reopen_ok phaseB_r1_state_reopen_ok phaseB_r1_defaults_survive_save \
phaseB_r2_set_key_ok phaseB_r2_set_style_ok phaseB_r2_state_set_ok phaseB_r2_disclose_matches_set phaseB_r2_close_ok phaseB_r2_reopen_ok phaseB_r2_state_reload_ok phaseB_r2_autopersist_fidelity \
phaseB_r3_save_as_ok phaseB_r3_copy_file phaseB_r3_close_ok phaseB_r3_open_copy_ok phaseB_r3_state_copy_ok phaseB_r3_save_as_fidelity \
phaseB_neg_ionian_passes phaseB_neg_scale_dorian23_error phaseB_neg_root_150_error phaseB_neg_root_H_error phaseB_neg_style_501_error phaseB_neg_state_unchanged_ok phaseB_neg_state_unchanged \
phaseB_clr_key_ok phaseB_clr_key_null phaseB_clr_style_ok phaseB_clr_style_empty phaseB_clr_key_idempotent_ok phaseB_tempo_family_intact"
export WORKDIR SUMMARY_FILE RUN_ID GATE_NAMES
export PHASEA_KERNEL_STOP PHASEB_KERNEL_STOP
export FAIL_CLASSIFICATION
export BASELINE_KERNEL_SHA KERNEL_SHA REPO_ROOT
export SET_KEY_ECHO SET_STYLE_ECHO IONIAN_ECHO
export R1_KEY R2_DISCLOSE_KEY R2R_KEY R3_KEY NEG_KEY
export PROJECT_FILE COPY_FILE STYLE_TEXT
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

summary = {
    "schema_version": "music.keyfield.roundtrip.mac.v1",
    "card": "GEN-CTX-KEYFIELD-1",
    "run_id": os.environ["RUN_ID"],
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": os.environ.get("FAIL_CLASSIFICATION") or ("none" if all(gates.values()) else "functional"),
    "repo": {"root": os.environ["REPO_ROOT"]},
    "binaries": {
        "baseline_kernel_sha256": os.environ["BASELINE_KERNEL_SHA"],
        "kernel_sha256": os.environ["KERNEL_SHA"],
    },
    "probe_surface": "kernel direct ZMQ REQ tcp://127.0.0.1:5555 (§11 roundtrip trilogy: baseline-authored old project, auto-persist reload, save_as copy)",
    "gates": gates,
    "gate_failure_details": fail_details,
    "roundtrip_evidence": {
        "set_time_echo_key": os.environ.get("SET_KEY_ECHO", ""),
        "set_time_echo_style": os.environ.get("SET_STYLE_ECHO", ""),
        "r1_old_project_key": os.environ.get("R1_KEY", ""),
        "r2_disclosed_key_after_set": os.environ.get("R2_DISCLOSE_KEY", ""),
        "r2_key_after_autopersist_reload": os.environ.get("R2R_KEY", ""),
        "r3_key_after_save_as_reload": os.environ.get("R3_KEY", ""),
        "neg_control_ionian_echo": os.environ.get("IONIAN_ECHO", ""),
        "neg_after_key": os.environ.get("NEG_KEY", ""),
    },
    "style_note_probe_text": os.environ.get("STYLE_TEXT", ""),
    "project_files": {
        "old_project": os.environ.get("PROJECT_FILE", ""),
        "save_as_copy": os.environ.get("COPY_FILE", ""),
    },
    "teardown": {
        "phaseA_kernel": os.environ.get("PHASEA_KERNEL_STOP", ""),
        "phaseB_kernel": os.environ.get("PHASEB_KERNEL_STOP", ""),
    },
}

path = os.environ["SUMMARY_FILE"]
with open(path, "w", encoding="utf-8") as f:
    json.dump(summary, f, indent=2, sort_keys=True, ensure_ascii=False)
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
