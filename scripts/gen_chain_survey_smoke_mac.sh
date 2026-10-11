#!/usr/bin/env bash
# gen_chain_survey_smoke_mac.sh — GEN-MAC-BASELINE-1: generation-chain first
# survey on the mac real stack (VitApp kernel + Go agent two-piece; the Godot
# frontend is out of scope, same berth as dev_agent_smoke_mac.sh).
#
# Survey surface (card: project.new/save_as + assets.generated.* three
# commands + midi.* family, dispatched through the agent tool chain — the
# surface an agent-orchestrated generation line actually rides on):
#
#   P0  fixtures     3-note C-major .mid + 2.2s 440Hz sine .wav, both
#                    synthesised into the run workdir (AGENTS §10)
#   P1  project      clear_project slate sweep; project.new;
#                    project.save_as into the run workdir (file-exists gate)
#   P2  midi family  track.add host; midi.import_file (3 fixture notes);
#                    midi.read_clip_notes read-back == fixture;
#                    midi.insert_clip (second clip); midi.add_notes;
#                    midi.add_notes_bulk; midi.write_clip_notes mixed-op
#                    patch (insert+transpose+delete -> apply_midi_note_patch);
#                    midi.mutate_notes region mutate; midi.delete_notes by
#                    note id; midi.read_clip_data (shape recorded);
#                    final read-back mathematical consistency (hard gate)
#   P3  render leg B MIDI-only edit: render.start must reply synchronously
#                    status=error reason=no_renderable_audio_content with no
#                    job_id (K1 fix reply semantics, mac first verification),
#                    command surface stays alive, idle render.cancel replies ok
#   P4  assets       assets.register_job; assets.ingest_generated_asset x2
#                    onto one audio track with a shared take_stack_id (clip,
#                    asset_ref, take stack snapshot gates);
#                    assets.set_async_ghost_state on the ingested clip;
#                    assets.switch_take back to take 1 (gated on the ok reply)
#   P5  render leg A with the ingested sine stem: render.start -> job_id;
#                    output wav polled to completion (watchdog window);
#                    RIFF header + non-zero size; command surface alive after
#                    (the K1 freeze regression check); render.profile.bind
#                    telemetry probe (survey-recorded, RLM-PROFILE-2 face)
#   P6  close        project.save; negative probe: ingest with a missing
#                    file_path must fail closed with an error reply
#
# Exit 0 = survey-complete: stack lifecycle + every probe dispatched with its
# reply recorded + all hard data-level gates green (the write/read-back
# substrate the generation line needs, GEN-CAP-RECON-1 §2.1). Per-command
# outcomes land in summary.json (schema gen.chain.survey.mac.v1) and
# per-probe JSON under http/. Failures are classified env (ports, toolchain,
# transport) vs functional (reply assertions) per AGENTS §8.
#
# Usage:
#   ./scripts/gen_chain_survey_smoke_mac.sh --kernel-bin PATH --agent-bin PATH
#   ./scripts/gen_chain_survey_smoke_mac.sh                 # repo default bins
#   ./scripts/gen_chain_survey_smoke_mac.sh --workdir PATH  # reuse a run workdir
#
# The kernel writes all of its state under a fake VitApp root inside the run
# workdir (VitPaths.h climbs to a directory containing CMakeLists.txt +
# Source/), so the source tree is never touched. This script never stops
# processes it did not start (AGENTS §9 single-owner rule).

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_DEFAULT=""
AGENT_BIN_ARG=""
WORKDIR_ARG=""
OUTPUT_PATH=""
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
RENDER_WAIT_SECONDS=300
INVOKE_TIMEOUT_DEFAULT=60

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555
SOURCE_TAG="gen_chain_survey_mac.GEN-MAC-BASELINE-1"

usage() {
  cat <<'EOF'
gen_chain_survey_smoke_mac.sh — generation-chain five-command first survey
(mac kernel+agent two-piece, GEN-MAC-BASELINE-1).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Kernel binary (default: repo
                          VitApp/build/VitApp_artefacts/Debug/VitApp)
  --agent-bin PATH        Prebuilt agent binary (skips the go build)
  --workdir PATH          Reuse PATH as the run workdir instead of a fresh
                          mktemp directory (never overwrites other runs)
  --startup-timeout SECONDS  Kernel/agent startup timeout (default 90)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --render-wait SECONDS   render leg A output completion watchdog (default 300)
  --output PATH           Also write the summary JSON to PATH
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --render-wait) RENDER_WAIT_SECONDS="$2"; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
[[ -d "$REPO_ROOT" ]] || { echo "repo root not found: $REPO_ROOT" >&2; exit 2; }

# ---------------------------------------------------------------- run workdir
RUN_ID="gen_chain_survey_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/gen_chain_survey_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{http,logs,fixtures,project}
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
CONFIRMATION_CARDS=0
GATE_FAIL_LOG="$WORKDIR/gate_failures.txt"
: > "$GATE_FAIL_LOG"

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

cleanup() {
  local rc=$?
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

json_field() {
  # json_field <file> <python expr against d> — small JSON extraction helper.
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8") as f:
        d = json.load(f)
    print(eval(sys.argv[2], {"d": d}))
except Exception as exc:  # missing file / invalid json -> empty output
    print(f"")
    sys.stderr.write(f"json_field({sys.argv[1]}): {exc}\n")
PY
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

REPLY_STATUS=""
REPLY_FILE=""
invoke_tool() {
  # invoke_tool <tag> <timeout-sec> <tool> <args-json>
  # Sets REPLY_STATUS (final envelope status) and REPLY_FILE (reply path).
  # Transport/HTTP failure -> return 9 (env-class by caller decision).
  local tag="$1" tmo="$2" tool="$3" args="$4" body code st
  REPLY_FILE="$WORKDIR/http/${tag}.json"
  body="$(printf '{"tool":"%s","args":%s,"source":"%s"}' "$tool" "$args" "$SOURCE_TAG")"
  code="$(curl -sS --max-time "$tmo" -o "$REPLY_FILE" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' --data "$body" \
    "$AGENT_HTTP/agent/invoke")" || return 9
  [[ "$code" =~ ^2 ]] || return 9
  st="$(json_field "$REPLY_FILE" 'str(d.get("status",""))')"
  if [[ "$st" == "needs_confirmation" ]]; then
    cp "$REPLY_FILE" "$WORKDIR/http/${tag}_card.json"
    CONFIRMATION_CARDS=$((CONFIRMATION_CARDS+1))
    log "confirmation card surfaced for $tool (action $(json_field "$REPLY_FILE" 'str(d.get("agent_action_id",""))')) — re-invoking confirmed"
    body="$(printf '{"tool":"%s","args":%s,"source":"%s","confirmed":true}' "$tool" "$args" "$SOURCE_TAG")"
    code="$(curl -sS --max-time "$tmo" -o "$REPLY_FILE" -w '%{http_code}' \
      -X POST -H 'Content-Type: application/json' --data "$body" \
      "$AGENT_HTTP/agent/invoke")" || return 9
    [[ "$code" =~ ^2 ]] || return 9
    st="$(json_field "$REPLY_FILE" 'str(d.get("status",""))')"
  fi
  REPLY_STATUS="$st"
  return 0
}

final_status() {
  # final_status <file> — kernel-level status: result.status wins over the
  # agent envelope status (render leg B replies status=error inside result).
  python3 - "$1" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
except Exception:
    print("")
    raise SystemExit
r = d.get("result")
if isinstance(r, dict) and str(r.get("status", "")) != "":
    print(str(r.get("status")))
else:
    print(str(d.get("status", "")))
PY
}

result_field() {
  # result_field <file> <pyexpr against r=result dict, d=full reply>
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
except Exception:
    print("")
    raise SystemExit
r = d.get("result") if isinstance(d.get("result"), dict) else {}
try:
    print(eval(sys.argv[2], {"r": r, "d": d}))
except Exception:
    print("")
PY
}

assert_ports_free() {
  local port pid
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "${AGENT_HTTP##*:}"; do
    pid="$(port_listener_pid "$port")"
    if [[ -n "$pid" ]]; then
      fail_env "port $port already has a listener (pid $pid); this harness never stops processes it did not start (AGENTS §9 single-owner rule)"
    fi
  done
}
assert_ports_free

# ---------------------------------------------------------------- binaries
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
[[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN (pass --kernel-bin; the kernel is NOT built by this survey script — GEN-MAC-BASELINE-1 rebuilds it out-of-band and records the hash)"
shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
KERNEL_SHA="$(cut -d' ' -f1 "$WORKDIR/kernel_bin.sha256")"
stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
log "kernel binary: $KERNEL_BIN (sha256 $KERNEL_SHA)"

if [[ -n "$AGENT_BIN_ARG" ]]; then
  AGENT_BIN="$AGENT_BIN_ARG"
  [[ -x "$AGENT_BIN" ]] || fail_env "agent binary is not executable: $AGENT_BIN"
else
  log "building agent (go build)..."
  mkdir -p "$WORKDIR/bin"
  (cd "$REPO_ROOT/agent" && go build -o "$WORKDIR/bin/vitagent" ./cmd/vitagent) \
    || fail_env "go build agent failed"
  AGENT_BIN="$WORKDIR/bin/vitagent"
fi
shasum -a 256 "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256"
AGENT_SHA="$(cut -d' ' -f1 "$WORKDIR/agent_bin.sha256")"
log "agent binary: $AGENT_BIN (sha256 $AGENT_SHA)"

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$KERNEL_SHA"
  echo "agent_bin=$AGENT_BIN"
  echo "agent_sha256=$AGENT_SHA"
  echo "agent_http=$AGENT_HTTP"
} > "$WORKDIR/run_meta.txt"

# ---------------------------------------------------------------- fixtures
MIDI_FIXTURE="$WORKDIR/fixtures/gen_survey.mid"
WAV_FIXTURE="$WORKDIR/fixtures/gen_survey_sine.wav"
python3 - "$MIDI_FIXTURE" "$WAV_FIXTURE" <<'PY'
import math, struct, sys

# MIDI: format 0, one track, 480 PPQ, 120 bpm implicit; three notes
# C4(60,v90) E4(64,v80) G4(67,v70), each 1 beat, back to back — byte-for-byte
# the archived green fixture from KERNEL-RENDER-FREEZE-FIX-1
# (coord/runs/KERNEL-RENDER-FREEZE-FIX-1/smoke_midi_register_regress/
# midi_register_fixture.mid). NB: the MTrk chunk id is case-sensitive
# ("MTrk", 0x4b6B lowercase k); an uppercase-K variant is silently skipped
# by the SMF parser as an unknown chunk and imports zero notes.
midi = bytes([
    0x4D,0x54,0x68,0x64, 0x00,0x00,0x00,0x06, 0x00,0x00, 0x00,0x01, 0x01,0xE0,
    0x4D,0x54,0x72,0x6B, 0x00,0x00,0x00,0x1F,
    0x00,0x90,0x3C,0x5A,
    0x83,0x60,0x80,0x3C,0x00,
    0x00,0x90,0x40,0x50,
    0x83,0x60,0x80,0x40,0x00,
    0x00,0x90,0x43,0x46,
    0x83,0x60,0x80,0x43,0x00,
    0x00,0xFF,0x2F,0x00,
])
with open(sys.argv[1], "wb") as f:
    f.write(midi)

# WAV: mono 16-bit 44.1kHz, 2.2s, 440Hz sine at amplitude 0.5 — sustained
# tone so the stem is renderable audio content (Write-LeaseSmokeStemWav form).
rate, dur, amp, freq = 44100, 2.2, 0.5, 440.0
samples = int(rate * dur)
data = b"".join(
    struct.pack("<h", int(round(amp * math.sin(2.0 * math.pi * freq * i / rate) * 32767.0)))
    for i in range(samples)
)
hdr = b"RIFF" + struct.pack("<I", 36 + len(data)) + b"WAVEfmt " + struct.pack(
    "<IHHIIHH", 16, 1, 1, rate, rate * 2, 2, 16
) + b"data" + struct.pack("<I", len(data))
with open(sys.argv[2], "wb") as f:
    f.write(hdr + data)
PY
[[ -s "$MIDI_FIXTURE" ]] || fail_env "midi fixture write failed"
[[ -s "$WAV_FIXTURE" ]] || fail_env "wav fixture write failed"
log "fixtures ready: $MIDI_FIXTURE ($(wc -c < "$MIDI_FIXTURE" | tr -d ' ')B), $WAV_FIXTURE ($(wc -c < "$WAV_FIXTURE" | tr -d ' ')B)"

# ---------------------------------------------------------------- start kernel
log "starting kernel (cwd=$KERNEL_ROOT, state isolated under the run workdir)..."
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
lsof -nP -iTCP:"$KERNEL_PORT_REQ" -iTCP:"$ZMQ_PUB_PORT" -iTCP:"$ZMQ_LOG_PORT" -sTCP:LISTEN \
  > "$WORKDIR/lsof_kernel_ports.txt" 2>&1 || true
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  listener_pid="$(port_listener_pid "$port")"
  [[ "$listener_pid" == "$KERNEL_PID" ]] || fail_functional "port $port listener pid $listener_pid != kernel pid $KERNEL_PID"
done
GATE_kernel_start_ports=1
log "kernel ZMQ ports listening: $KERNEL_PORT_REQ (REQ) $ZMQ_PUB_PORT (PUB) $ZMQ_LOG_PORT (log) — owned by pid $KERNEL_PID"

# ---------------------------------------------------------------- start agent
log "starting agent (state isolated under $AGENT_STATE_DIR)..."
(
  cd "$WORKDIR/agent_cwd"
  VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
    exec "$AGENT_BIN" \
    -http "${AGENT_HTTP#http://}" \
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
curl -sS --max-time 2 -o "$WORKDIR/http/health.json" "$AGENT_HTTP/health" 2>/dev/null \
  || fail_functional "agent /health not ready within ${STARTUP_TIMEOUT_SECONDS}s"
GATE_agent_start_health=1
log "agent /health ready"

# ---------------------------------------------------------------- P1 project
log "P1: clear_project slate sweep..."
if invoke_tool p01_clear 60 "clear_project" '{}'; then
  gate clear_project_slate "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate clear_project_slate 0 "transport/http failure"
fi

log "P1: project.new..."
if invoke_tool p02_project_new 60 "project.new" '{}'; then
  gate project_new_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate project_new_ok 0 "transport/http failure"
fi

SAVE_AS_PATH="$WORKDIR/project/gen_survey_saveas.vit"
rm -f "$SAVE_AS_PATH"
log "P1: project.save_as -> $SAVE_AS_PATH ..."
if invoke_tool p03_save_as 90 "project.save_as" "{\"file_path\":\"$SAVE_AS_PATH\"}"; then
  if [[ "$REPLY_STATUS" == "ok" && -f "$SAVE_AS_PATH" && -s "$SAVE_AS_PATH" ]]; then
    gate project_save_as_ok 1 "file $(wc -c < "$SAVE_AS_PATH" | tr -d ' ')B"
  else
    gate project_save_as_ok 0 "status=$REPLY_STATUS file_exists=$([[ -f "$SAVE_AS_PATH" ]] && echo yes || echo no)"
  fi
else
  gate project_save_as_ok 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P2 midi
log "P2: track.add (midi host)..."
MIDI_TRACK_ID=""
if invoke_tool p04_track_add 60 "track.add" '{}'; then
  MIDI_TRACK_ID="$(result_field "$REPLY_FILE" 'str(r.get("track_id",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$MIDI_TRACK_ID" ]]; then
    gate midi_host_track_ok 1 "track_id=$MIDI_TRACK_ID"
  else
    gate midi_host_track_ok 0 "status=$REPLY_STATUS track_id=$MIDI_TRACK_ID"
  fi
else
  gate midi_host_track_ok 0 "transport/http failure"
fi

log "P2: midi.import_file (3 fixture notes)..."
MIDI_CLIP_ID=""
if invoke_tool p05_midi_import 180 "midi.import_file" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"file_path\":\"$MIDI_FIXTURE\",\"start_time_beats\":0}"; then
  MIDI_CLIP_ID="$(result_field "$REPLY_FILE" 'str(r.get("clip_id",""))')"
  NOTE_COUNT="$(result_field "$REPLY_FILE" 'str(r.get("note_count",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$MIDI_CLIP_ID" && "$NOTE_COUNT" == "3" ]]; then
    gate midi_import_ok 1 "clip=$MIDI_CLIP_ID note_count=3"
  else
    gate midi_import_ok 0 "status=$REPLY_STATUS clip=$MIDI_CLIP_ID note_count=$NOTE_COUNT"
  fi
else
  gate midi_import_ok 0 "transport/http failure"
fi

log "P2: midi.read_clip_notes read-back after import..."
if invoke_tool p06_readback_import 60 "midi.read_clip_notes" "{\"clip_id\":\"$MIDI_CLIP_ID\"}"; then
  VERDICT="$(python3 - "$REPLY_FILE" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
r = d.get("result") if isinstance(d.get("result"), dict) else {}
notes = r.get("notes")
if not isinstance(notes, list):
    print("no_notes_field")
    raise SystemExit
want = {(60,0.0,1.0,90):1,(64,1.0,1.0,80):1,(67,2.0,1.0,70):1}
got = {}
ok = True
for n in notes:
    key = (int(n.get("pitch",-1)), round(float(n.get("start",-1)),3), round(float(n.get("length", n.get("duration",-1))),3), int(n.get("velocity",-1)))
    got[key] = got.get(key,0)+1
if got != want or len(notes) != 3:
    ok = False
print("consistent" if ok else f"mismatch got={got}")
PY
)"
  if [[ "$REPLY_STATUS" == "ok" && "$VERDICT" == "consistent" ]]; then
    gate midi_readback_import_consistent 1 "3 notes exact (pitch/start/length/velocity)"
  else
    gate midi_readback_import_consistent 0 "status=$REPLY_STATUS verdict=$VERDICT"
  fi
else
  gate midi_readback_import_consistent 0 "transport/http failure"
fi

log "P2: midi.insert_clip (second clip, beat 8 len 4)..."
MIDI_CLIP2_ID=""
if invoke_tool p07_insert_clip 90 "midi.insert_clip" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"start\":8.0,\"length\":4.0}"; then
  MIDI_CLIP2_ID="$(result_field "$REPLY_FILE" 'str(r.get("clip_id",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$MIDI_CLIP2_ID" ]]; then
    gate midi_insert_clip_ok 1 "clip2=$MIDI_CLIP2_ID"
  else
    gate midi_insert_clip_ok 0 "status=$REPLY_STATUS clip2=$MIDI_CLIP2_ID"
  fi
else
  gate midi_insert_clip_ok 0 "transport/http failure"
fi

log "P2: midi.add_notes (2 notes into clip2)..."
if invoke_tool p08_add_notes 90 "midi.add_notes" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"clip_id\":\"$MIDI_CLIP2_ID\",\"time_unit\":\"beats\",\"notes\":[{\"pitch\":72,\"start\":0.0,\"length\":0.5,\"velocity\":100},{\"pitch\":74,\"start\":1.0,\"length\":0.5,\"velocity\":90}]}"; then
  gate midi_add_notes_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate midi_add_notes_ok 0 "transport/http failure"
fi

log "P2: midi.add_notes_bulk (3 notes into clip2)..."
if invoke_tool p09_add_notes_bulk 90 "midi.add_notes_bulk" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"clip_id\":\"$MIDI_CLIP2_ID\",\"time_unit\":\"beats\",\"notes\":[{\"pitch\":76,\"start\":2.0,\"length\":0.5,\"velocity\":80},{\"pitch\":79,\"start\":3.0,\"length\":0.5,\"velocity\":70},{\"pitch\":83,\"start\":3.5,\"length\":0.25,\"velocity\":60}]}"; then
  gate midi_add_notes_bulk_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate midi_add_notes_bulk_ok 0 "transport/http failure"
fi

log "P2: midi.write_clip_notes mixed-op patch (insert+transpose+delete)..."
if invoke_tool p10_apply_patch 120 "midi.write_clip_notes" \
  "{\"clip_id\":\"$MIDI_CLIP_ID\",\"time_unit\":\"beats\",\"operations\":[{\"op\":\"insert_note\",\"pitch\":72,\"start\":3.0,\"length\":1.0,\"velocity\":64},{\"op\":\"transpose_note\",\"semitones\":5,\"start\":0.0,\"length\":1.0},{\"op\":\"delete_note\",\"start\":1.0,\"length\":1.0}]}"; then
  gate midi_apply_patch_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate midi_apply_patch_ok 0 "transport/http failure"
fi

log "P2: midi.mutate_notes (by note id — the kernel requires ids, region form is rejected)..."
MUTATE_NOTE_ID=""
if invoke_tool p10b_pre_mutate_read 60 "midi.read_clip_notes" "{\"clip_id\":\"$MIDI_CLIP_ID\"}"; then
  MUTATE_NOTE_ID="$(python3 - "$REPLY_FILE" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    notes = (d.get("result") or {}).get("notes") or []
    for n in notes:
        if int(n.get("pitch", -1)) == 67:
            print(str(n.get("id", n.get("note_id", ""))))
            break
    else:
        print("")
except Exception:
    print("")
PY
)"
  log "mutate target note id (pitch 67): ${MUTATE_NOTE_ID:-<none found>}"
fi
if [[ -n "$MUTATE_NOTE_ID" ]] && invoke_tool p11_mutate 90 "midi.mutate_notes" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"clip_id\":\"$MIDI_CLIP_ID\",\"notes\":[{\"id\":\"$MUTATE_NOTE_ID\",\"pitch\":74,\"velocity\":55}]}"; then
  gate midi_mutate_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  if [[ -z "$MUTATE_NOTE_ID" ]]; then
    gate midi_mutate_ok 0 "no pitch-67 note id surfaced by read-back (p10b)"
  else
    gate midi_mutate_ok 0 "transport/http failure"
  fi
fi

log "P2: midi.delete_notes (by note id of the inserted 72)..."
DEL_NOTE_ID=""
if invoke_tool p12_pre_delete_read 60 "midi.read_clip_notes" "{\"clip_id\":\"$MIDI_CLIP_ID\"}"; then
  DEL_NOTE_ID="$(python3 - "$REPLY_FILE" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    notes = (d.get("result") or {}).get("notes") or []
    for n in notes:
        if int(n.get("pitch", -1)) == 72:
            print(str(n.get("id", n.get("note_id", ""))))
            break
    else:
        print("")
except Exception:
    print("")
PY
)"
  log "delete target note id: ${DEL_NOTE_ID:-<none found>}"
fi
if [[ -n "$DEL_NOTE_ID" ]] && invoke_tool p13_delete_notes 90 "midi.delete_notes_legacy" \
  "{\"track_id\":\"$MIDI_TRACK_ID\",\"clip_id\":\"$MIDI_CLIP_ID\",\"note_ids\":[\"$DEL_NOTE_ID\"]}"; then
  gate midi_delete_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  if [[ -z "$DEL_NOTE_ID" ]]; then
    gate midi_delete_ok 0 "no note id surfaced by read-back (p12)"
  else
    gate midi_delete_ok 0 "transport/http failure"
  fi
fi

log "P2: midi.read_clip_data (shape survey)..."
CLIP_DATA_SHAPE="not-dispatched"
if invoke_tool p14_clip_data 60 "midi.read_clip_data" "{\"clip_id\":\"$MIDI_CLIP_ID\"}"; then
  CLIP_DATA_SHAPE="$(python3 - "$REPLY_FILE" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    r = d.get("result") if isinstance(d.get("result"), dict) else {}
    keys = sorted(r.keys())
    print(",".join(keys) if keys else str(d.get("status", "")))
except Exception as exc:
    print(f"unreadable: {exc}")
PY
)"
  log "clip_data result keys: $CLIP_DATA_SHAPE"
fi

log "P2: final read-back consistency..."
if invoke_tool p15_readback_final 60 "midi.read_clip_notes" "{\"clip_id\":\"$MIDI_CLIP_ID\"}"; then
  FINAL_VERDICT="$(python3 - "$REPLY_FILE" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
r = d.get("result") if isinstance(d.get("result"), dict) else {}
notes = r.get("notes")
if not isinstance(notes, list):
    print("no_notes_field")
    raise SystemExit
# clip1 expected final state (beats, clip starts at 0):
#   65@0.0 len1.0 v90  (60 transposed +5)
#   74@2.0 len1.0 v55  (67 region-mutated)
#   (64 deleted by patch; 72 inserted by patch then deleted by note id)
want = {(65,0.0,1.0,90):1,(74,2.0,1.0,55):1}
got = {}
for n in notes:
    key = (int(n.get("pitch",-1)), round(float(n.get("start",-1)),3), round(float(n.get("length", n.get("duration",-1))),3), int(n.get("velocity",-1)))
    got[key] = got.get(key,0)+1
print("consistent" if got == want else f"mismatch got={got}")
PY
)"
  if [[ "$REPLY_STATUS" == "ok" && "$FINAL_VERDICT" == "consistent" ]]; then
    gate midi_readback_final_consistent 1 "clip1 final == {65@0 v90, 74@2 v55}"
  else
    gate midi_readback_final_consistent 0 "status=$REPLY_STATUS verdict=$FINAL_VERDICT"
  fi
else
  gate midi_readback_final_consistent 0 "transport/http failure"
fi

if invoke_tool p15b_readback_clip2 60 "midi.read_clip_notes" "{\"clip_id\":\"$MIDI_CLIP2_ID\"}"; then
  CLIP2_COUNT="$(result_field "$REPLY_FILE" 'len(r.get("notes") or [])')"
  if [[ "$REPLY_STATUS" == "ok" && "$CLIP2_COUNT" == "5" ]]; then
    gate midi_readback_clip2_consistent 1 "clip2 holds 5 notes (2 add + 3 bulk)"
  else
    gate midi_readback_clip2_consistent 0 "status=$REPLY_STATUS count=$CLIP2_COUNT"
  fi
else
  gate midi_readback_clip2_consistent 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P3 render leg B
log "P3: render leg B — MIDI-only edit must be rejected synchronously..."
LEGB_PATH="$WORKDIR/fixtures/gen_survey_midi_only.wav"
if invoke_tool p16_render_legB 60 "render.start" "{\"file_path\":\"$LEGB_PATH\"}"; then
  LEGB_STATUS="$(final_status "$REPLY_FILE")"
  LEGB_REASON="$(result_field "$REPLY_FILE" 'str(r.get("reason",""))')"
  LEGB_JOB="$(result_field "$REPLY_FILE" 'str(r.get("job_id",""))')"
  if [[ "$LEGB_STATUS" == "error" && "$LEGB_REASON" == "no_renderable_audio_content" && -z "$LEGB_JOB" ]]; then
    gate render_legB_sync_rejection 1 "status=error reason=no_renderable_audio_content no job"
  else
    gate render_legB_sync_rejection 0 "status=$LEGB_STATUS reason=$LEGB_REASON job=$LEGB_JOB"
  fi
else
  gate render_legB_sync_rejection 0 "transport/http failure"
fi

if invoke_tool p17_legB_alive 30 "project.state" '{}'; then
  gate render_legB_surface_alive "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate render_legB_surface_alive 0 "transport/http failure"
fi

if invoke_tool p18_legB_cancel 30 "render.cancel" '{}'; then
  gate render_legB_idle_cancel_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate render_legB_idle_cancel_ok 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P4 assets
log "P4: track.add (audio host)..."
AUDIO_TRACK_ID=""
if invoke_tool p19_audio_track 60 "track.add" '{}'; then
  AUDIO_TRACK_ID="$(result_field "$REPLY_FILE" 'str(r.get("track_id",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$AUDIO_TRACK_ID" ]]; then
    gate assets_host_track_ok 1 "track_id=$AUDIO_TRACK_ID"
  else
    gate assets_host_track_ok 0 "status=$REPLY_STATUS track_id=$AUDIO_TRACK_ID"
  fi
else
  gate assets_host_track_ok 0 "transport/http failure"
fi

log "P4: assets.register_job..."
GEN_JOB_ID="genbase1-job-1"
if invoke_tool p20_register_job 60 "assets.register_job" \
  "{\"job_id\":\"$GEN_JOB_ID\",\"job_state\":\"generating\",\"status_message\":\"gen chain survey fixture job\",\"source\":\"gen_chain_survey_mac\"}"; then
  gate assets_register_job_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate assets_register_job_ok 0 "transport/http failure"
fi

log "P4: assets.ingest_generated_asset #1 (sine stem, shared take stack)..."
INGEST1_CLIP=""; TAKE1_ID=""; ASSET1_REF=""
if invoke_tool p21_ingest1 180 "assets.ingest_generated_asset" \
  "{\"file_path\":\"$WAV_FIXTURE\",\"track_id\":\"$AUDIO_TRACK_ID\",\"start_time\":0.0,\"job_id\":\"$GEN_JOB_ID\",\"bucket\":\"generated\",\"take_stack_id\":\"genbase1-stack\"}"; then
  INGEST1_CLIP="$(result_field "$REPLY_FILE" 'str(r.get("clip_id",""))')"
  ASSET1_REF="$(result_field "$REPLY_FILE" 'str(r.get("asset_ref",""))')"
  TAKE1_ID="$(result_field "$REPLY_FILE" 'str(r.get("active_take_id",""))')"
  STACKS="$(result_field "$REPLY_FILE" 'len(r.get("take_histories") or [])')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$INGEST1_CLIP" && -n "$ASSET1_REF" && "$STACKS" -ge 1 ]]; then
    gate assets_ingest1_ok 1 "clip=$INGEST1_CLIP asset_ref=${ASSET1_REF:0:24}… take=$TAKE1_ID stacks=$STACKS"
  else
    gate assets_ingest1_ok 0 "status=$REPLY_STATUS clip=$INGEST1_CLIP ref=$ASSET1_REF stacks=$STACKS"
  fi
else
  gate assets_ingest1_ok 0 "transport/http failure"
fi

log "P4: assets.ingest_generated_asset #2 (second take on the same stack)..."
INGEST2_CLIP=""; TAKE2_ID=""
if invoke_tool p22_ingest2 180 "assets.ingest_generated_asset" \
  "{\"file_path\":\"$WAV_FIXTURE\",\"track_id\":\"$AUDIO_TRACK_ID\",\"start_time\":2.5,\"take_stack_id\":\"genbase1-stack\"}"; then
  INGEST2_CLIP="$(result_field "$REPLY_FILE" 'str(r.get("clip_id",""))')"
  TAKE2_ID="$(result_field "$REPLY_FILE" 'str(r.get("active_take_id",""))')"
  STACK_TAKES="$(result_field "$REPLY_FILE" 'sum(len(s.get("takes") or []) for s in (r.get("take_histories") or []))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$INGEST2_CLIP" && "$STACK_TAKES" -ge 2 ]]; then
    gate assets_ingest2_ok 1 "clip2=$INGEST2_CLIP take2=$TAKE2_ID stack_takes=$STACK_TAKES"
  else
    gate assets_ingest2_ok 0 "status=$REPLY_STATUS clip2=$INGEST2_CLIP stack_takes=$STACK_TAKES"
  fi
else
  gate assets_ingest2_ok 0 "transport/http failure"
fi

log "P4: assets.set_async_ghost_state (mute on clip1)..."
if invoke_tool p23_ghost 60 "assets.set_async_ghost_state" \
  "{\"clip_id\":\"$INGEST1_CLIP\",\"ghost_state\":\"mute\"}"; then
  gate assets_ghost_state_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate assets_ghost_state_ok 0 "transport/http failure"
fi

log "P4: assets.switch_take (clip2 -> take1)..."
SWITCH_TAKE_VERDICT="not-dispatched"
if [[ -n "$TAKE1_ID" && -n "$INGEST2_CLIP" ]] && invoke_tool p24_switch_take 90 "assets.switch_take" \
  "{\"clip_id\":\"$INGEST2_CLIP\",\"take_id\":\"$TAKE1_ID\"}"; then
  ACTIVE_AFTER="$(result_field "$REPLY_FILE" 'str(r.get("active_take_id", r.get("take_id","")))')"
  if [[ "$REPLY_STATUS" == "ok" ]]; then
    gate assets_switch_take_ok 1 "active_take after=$ACTIVE_AFTER (expect $TAKE1_ID)"
  else
    gate assets_switch_take_ok 0 "status=$REPLY_STATUS"
  fi
  SWITCH_TAKE_VERDICT="status=$REPLY_STATUS active_after=$ACTIVE_AFTER"
else
  if [[ -z "$TAKE1_ID" || -z "$INGEST2_CLIP" ]]; then
    gate assets_switch_take_ok 0 "prerequisite missing (take1=$TAKE1_ID clip2=$INGEST2_CLIP)"
    SWITCH_TAKE_VERDICT="prerequisite-missing"
  else
    gate assets_switch_take_ok 0 "transport/http failure"
    SWITCH_TAKE_VERDICT="transport-failure"
  fi
fi

# ---------------------------------------------------------------- P5 render leg A
log "P5: render leg A — with the ingested sine stem..."
RENDER_OUT="$WORKDIR/fixtures/gen_survey_render_out.wav"
rm -f "$RENDER_OUT"
RENDER_JOB_ID=""
if invoke_tool p25_render_legA 90 "render.start" "{\"file_path\":\"$RENDER_OUT\"}"; then
  RENDER_JOB_ID="$(result_field "$REPLY_FILE" 'str(r.get("job_id",""))')"
  RENDER_START_STATUS="$(final_status "$REPLY_FILE")"
  if [[ "$REPLY_STATUS" == "ok" && -n "$RENDER_JOB_ID" ]]; then
    gate render_legA_job_started 1 "job_id=$RENDER_JOB_ID status=$RENDER_START_STATUS"
  else
    gate render_legA_job_started 0 "envelope=$REPLY_STATUS job=$RENDER_JOB_ID kernel_status=$RENDER_START_STATUS"
  fi
else
  gate render_legA_job_started 0 "transport/http failure"
fi

log "P5: polling render output (watchdog ${RENDER_WAIT_SECONDS}s)..."
RENDER_FILE_STATE="missing"
deadline=$((SECONDS + RENDER_WAIT_SECONDS))
while (( SECONDS < deadline )); do
  if [[ -s "$RENDER_OUT" ]]; then
    RENDER_FILE_STATE="written"
    break
  fi
  if kernel_dead; then
    RENDER_FILE_STATE="kernel-dead"
    break
  fi
  sleep 2
done
if [[ "$RENDER_FILE_STATE" == "written" ]]; then
  RIFF_HEAD="$(dd if="$RENDER_OUT" bs=1 count=4 2>/dev/null || true)"
  RENDER_SIZE="$(wc -c < "$RENDER_OUT" | tr -d ' ')"
  if [[ "$RIFF_HEAD" == "RIFF" && "$RENDER_SIZE" -gt 44 ]]; then
    gate render_legA_file_completed 1 "RIFF ok, ${RENDER_SIZE}B"
  else
    gate render_legA_file_completed 0 "head='$RIFF_HEAD' size=$RENDER_SIZE"
  fi
else
  gate render_legA_file_completed 0 "state=$RENDER_FILE_STATE within ${RENDER_WAIT_SECONDS}s"
fi

if invoke_tool p26_legA_alive 30 "project.state" '{}'; then
  gate render_legA_surface_alive "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS (K1 freeze regression check)"
else
  gate render_legA_surface_alive 0 "transport/http failure (K1 freeze regression check)"
fi

log "P5: render.profile.bind telemetry probe (survey-recorded)..."
PROFILE_BIND_VERDICT="not-dispatched"
if [[ -n "$RENDER_JOB_ID" ]] && invoke_tool p27_profile_bind 60 "render.profile.bind" \
  "{\"render_id\":\"$RENDER_JOB_ID\",\"profile_id\":\"builtin:apple_music\"}"; then
  PROFILE_BIND_VERDICT="status=$REPLY_STATUS $(result_field "$REPLY_FILE" 'str(r.get("error", r.get("message","")))[:120]')"
  log "render.profile.bind: $PROFILE_BIND_VERDICT"
elif [[ -z "$RENDER_JOB_ID" ]]; then
  PROFILE_BIND_VERDICT="no-render-job"
fi

# ---------------------------------------------------------------- P6 close
log "P6: project.save..."
if invoke_tool p28_save 90 "project.save" '{}'; then
  gate project_save_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate project_save_ok 0 "transport/http failure"
fi

log "P6: negative probe — ingest with a missing file must fail closed..."
if invoke_tool p29_ingest_badpath 60 "assets.ingest_generated_asset" \
  "{\"file_path\":\"$WORKDIR/fixtures/does_not_exist.wav\",\"track_id\":\"$AUDIO_TRACK_ID\",\"start_time\":0.0}"; then
  BADPATH_STATUS="$(final_status "$REPLY_FILE")"
  if [[ "$BADPATH_STATUS" == "error" ]]; then
    gate ingest_failclosed_badpath 1 "error reply: $(result_field "$REPLY_FILE" 'str(r.get("message", r.get("error","")))[:80]')"
  else
    gate ingest_failclosed_badpath 0 "status=$BADPATH_STATUS (must be error)"
  fi
else
  gate ingest_failclosed_badpath 0 "transport/http failure"
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

log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "kernel stop: $KERNEL_STOP_RECORD (signal:exit_code:elapsed — A1 finding: SIGTERM expected to yield 143 without JUCE shutdown)"
KERNEL_PID=""
AGENT_PID=""

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
GATE_NAMES="kernel_start_ports agent_start_health clear_project_slate project_new_ok project_save_as_ok \
midi_host_track_ok midi_import_ok midi_readback_import_consistent midi_insert_clip_ok midi_add_notes_ok \
midi_add_notes_bulk_ok midi_apply_patch_ok midi_mutate_ok midi_delete_ok midi_readback_final_consistent \
midi_readback_clip2_consistent render_legB_sync_rejection render_legB_surface_alive render_legB_idle_cancel_ok \
assets_host_track_ok assets_register_job_ok assets_ingest1_ok assets_ingest2_ok assets_ghost_state_ok \
assets_switch_take_ok render_legA_job_started render_legA_file_completed render_legA_surface_alive \
project_save_ok ingest_failclosed_badpath"
export WORKDIR SUMMARY_FILE RUN_ID GATE_NAMES
export KERNEL_STOP_RECORD AGENT_STOP_RECORD FAIL_CLASSIFICATION CONFIRMATION_CARDS
export RENDER_JOB_ID CLIP_DATA_SHAPE SWITCH_TAKE_VERDICT PROFILE_BIND_VERDICT
export MIDI_CLIP_ID MIDI_CLIP2_ID INGEST1_CLIP INGEST2_CLIP TAKE1_ID TAKE2_ID ASSET1_REF
export LEGB_STATUS RENDER_FILE_STATE AGENT_SHA KERNEL_SHA REPO_ROOT
python3 - <<'PY'
import json, os, sys

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
    "schema_version": "gen.chain.survey.mac.v1",
    "card": "GEN-MAC-BASELINE-1",
    "run_id": os.environ["RUN_ID"],
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": os.environ.get("FAIL_CLASSIFICATION") or ("none" if all(gates.values()) else "functional"),
    "repo": {
        "root": os.environ["REPO_ROOT"],
    },
    "binaries": {
        "kernel_sha256": os.environ["KERNEL_SHA"],
        "agent_sha256": os.environ["AGENT_SHA"],
    },
    "gates": gates,
    "gate_failure_details": fail_details,
    "survey_recorded": {
        "confirmation_cards_surfaced": int(os.environ.get("CONFIRMATION_CARDS", "0")),
        "midi_clip_data_result_keys": os.environ.get("CLIP_DATA_SHAPE", ""),
        "switch_asset_take": os.environ.get("SWITCH_TAKE_VERDICT", ""),
        "render_profile_bind": os.environ.get("PROFILE_BIND_VERDICT", ""),
        "render_legA": {
            "job_id": os.environ.get("RENDER_JOB_ID", ""),
            "file_state": os.environ.get("RENDER_FILE_STATE", ""),
        },
        "render_legB_reply_status": os.environ.get("LEGB_STATUS", ""),
        "ids": {
            "midi_clip_import": os.environ.get("MIDI_CLIP_ID", ""),
            "midi_clip_inserted": os.environ.get("MIDI_CLIP2_ID", ""),
            "ingest_clips": [os.environ.get("INGEST1_CLIP", ""), os.environ.get("INGEST2_CLIP", "")],
            "takes": [os.environ.get("TAKE1_ID", ""), os.environ.get("TAKE2_ID", "")],
            "asset_ref_1": os.environ.get("ASSET1_REF", ""),
        },
    },
    "kernel_stop": {"record": os.environ.get("KERNEL_STOP_RECORD", "")},
    "agent_stop": {"record": os.environ.get("AGENT_STOP_RECORD", "")},
    "artifacts_dir": workdir,
}
meta = os.path.join(workdir, "run_meta.txt")
if os.path.exists(meta):
    summary["run_meta"] = dict(
        line.split("=", 1) for line in open(meta, encoding="utf-8").read().splitlines() if "=" in line
    )
with open(os.environ["SUMMARY_FILE"], "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY

[[ -n "$OUTPUT_PATH" ]] && { mkdir -p "$(dirname "$OUTPUT_PATH")"; cp "$SUMMARY_FILE" "$OUTPUT_PATH"; }
log "workdir: $WORKDIR"

if ! python3 -c '
import json, sys
d = json.load(open(sys.argv[1]))
sys.exit(0 if all(d["gates"].values()) else 1)' "$SUMMARY_FILE"; then
  log "SURVEY INCOMPLETE: failing gates: $(cut -d: -f1 "$GATE_FAIL_LOG" | tr '\n' ' ')"
  exit 1
fi
log "SURVEY COMPLETE: all gates green (exit 0)"
exit 0
