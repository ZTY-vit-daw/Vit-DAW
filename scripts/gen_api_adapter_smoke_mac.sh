#!/usr/bin/env bash
# gen_api_adapter_smoke_mac.sh — GEN-API-ADAPTER-1: unified generation-API
# adapter end-to-end smoke on the mac real stack (VitApp kernel + Go agent
# two-piece; the Godot frontend and webui are out of scope, same berth as
# gen_chain_survey_smoke_mac.sh).
#
# Chain under test (card acceptance: 用户配置→生成→产物 ingest 落轨→clip 可
# 寻址, data-level assertions, exit 0):
#
#   P0  startup      provider simulator (self-hosted generation endpoint
#                    fixture: sync_binary + suno_relay + a 401 surface) on a
#                    loopback port; kernel+agent two-piece with
#                    VIT_GENAPI_CONFIG pointing at a run-scoped provider
#                    config; user "fills the config" exactly like a real
#                    user would (file + one env credential override)
#   P1  gen.providers       configured providers are listed with key
#                           presence only — the secret must NOT appear
#   P2  unified template    track.add -> gen.generate (sync_binary dialect,
#                           self-hosted MusicGen型) -> artifact on disk with
#                           RIFF magic -> existing assets.register_job +
#                           assets.ingest_generated_asset path -> clip_id /
#                           asset_ref / active_take_id -> clip visible in
#                           project.state (addressable)
#   P3  suno relay型        gen.generate on the suno_relay dialect (relay
#                           submit/poll/download surface) -> two candidate
#                           clips on one shared take stack ->
#                           assets.switch_take flips the active candidate
#   P4  fail-closed         unknown provider -> config_error; 401 endpoint
#                           -> auth_error; missing prompt -> config_error;
#                           command surface stays alive afterwards
#   P5  close               teardown, summary.json (schema
#                           genapi.adapter.smoke.mac.v1)
#
# Boundary declaration (AGENTS §5/§8, honest labeling): the generation
# provider here is a LOCAL simulator standing in for the self-hosted
# provider type. It exercises the full adapter chain (request construction,
# param normalization, polling, download, sniff, ingest) over real HTTP and
# the real kernel. No external provider key is available in this
# environment, so the external-provider leg (real Suno relay / Stability /
# MusicGPT account) is declared as an environment blocker in the card
# receipt rather than faked here.
#
# Exit 0 = all gates green.
#
# Usage:
#   ./scripts/gen_api_adapter_smoke_mac.sh --kernel-bin PATH
#   ./scripts/gen_api_adapter_smoke_mac.sh                 # repo default bins
#   ./scripts/gen_api_adapter_smoke_mac.sh --workdir PATH  # reuse a run workdir

set -euo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_DEFAULT=""
AGENT_BIN_ARG=""
WORKDIR_ARG=""
OUTPUT_PATH=""
STARTUP_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
INVOKE_TIMEOUT_DEFAULT=120

ZMQ_REQ_ENDPOINT="tcp://127.0.0.1:5555"
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
KERNEL_PORT_REQ=5555
SOURCE_TAG="gen_api_adapter_smoke_mac.GEN-API-ADAPTER-1"
RELAY_ENV_KEY="smoke-relay-key-7f3a"

usage() {
  cat <<'EOF'
gen_api_adapter_smoke_mac.sh — unified generation-API adapter end-to-end
smoke (mac kernel+agent two-piece, GEN-API-ADAPTER-1).

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
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
[[ -d "$REPO_ROOT" ]] || { echo "repo root not found: $REPO_ROOT" >&2; exit 2; }

# ---------------------------------------------------------------- run workdir
RUN_ID="gen_api_adapter_smoke_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/gen_api_adapter_smoke.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{http,logs,project,config,cache}
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT"
: > "$KERNEL_ROOT/CMakeLists.txt"
mkdir -p "$KERNEL_ROOT/Source"
AGENT_STATE_DIR="$WORKDIR/agent_state"
mkdir -p "$AGENT_STATE_DIR" "$WORKDIR/agent_cwd"

KERNEL_PID=""
AGENT_PID=""
SIM_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
FAIL_CLASSIFICATION=""
CONFIRMATION_CARDS=0
GATE_FAIL_LOG="$WORKDIR/gate_failures.txt"
: > "$GATE_FAIL_LOG"

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

cleanup() {
  local rc=$?
  for pid_sig in "$SIM_PID:TERM" "$AGENT_PID:TERM" "$KERNEL_PID:TERM"; do
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
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8") as f:
        d = json.load(f)
    print(eval(sys.argv[2], {"d": d}))
except Exception as exc:
    print(f"")
    sys.stderr.write(f"json_field({sys.argv[1]}): {exc}\n")
PY
}

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

gate() {
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
  # The agent answers tool-level failures with HTTP 400 + a JSON error
  # envelope (harness error path); that is a REPLIED failure, not a transport
  # failure — only curl errors / unparseable bodies count as transport here.
  local tag="$1" tmo="$2" tool="$3" args="$4" body code st
  REPLY_FILE="$WORKDIR/http/${tag}.json"
  body="$(printf '{"tool":"%s","args":%s,"source":"%s"}' "$tool" "$args" "$SOURCE_TAG")"
  code="$(curl -sS --max-time "$tmo" -o "$REPLY_FILE" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' --data "$body" \
    "$AGENT_HTTP/agent/invoke")" || return 9
  st="$(json_field "$REPLY_FILE" 'str(d.get("status",""))')"
  [[ -n "$st" ]] || return 9
  if [[ "$st" == "needs_confirmation" ]]; then
    cp "$REPLY_FILE" "$WORKDIR/http/${tag}_card.json"
    CONFIRMATION_CARDS=$((CONFIRMATION_CARDS+1))
    log "confirmation card surfaced for $tool — re-invoking confirmed"
    body="$(printf '{"tool":"%s","args":%s,"source":"%s","confirmed":true}' "$tool" "$args" "$SOURCE_TAG")"
    code="$(curl -sS --max-time "$tmo" -o "$REPLY_FILE" -w '%{http_code}' \
      -X POST -H 'Content-Type: application/json' --data "$body" \
      "$AGENT_HTTP/agent/invoke")" || return 9
    st="$(json_field "$REPLY_FILE" 'str(d.get("status",""))')"
    [[ -n "$st" ]] || return 9
  fi
  REPLY_STATUS="$st"
  return 0
}

final_status() {
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
[[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN (pass --kernel-bin; the kernel is NOT built by this script — reuse the GEN-MAC-BASELINE-1 rebuild or rebuild out-of-band and record the hash)"
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

# ---------------------------------------------------------------- P0 provider simulator
SIM_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
SIM_EVIDENCE="$WORKDIR/sim_evidence.json"
SIM_LOG="$WORKDIR/logs/provider_simulator.log"
python3 - "$SIM_PORT" "$SIM_EVIDENCE" > "$SIM_LOG" 2>&1 <<'PY' &
import json, math, struct, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port, evidence_path = int(sys.argv[1]), sys.argv[2]
evidence = {"sync_submit": [], "suno_submit": [], "suno_polls": 0, "suno_submit_auth": [],
            "downloads": [], "auth_fail_hits": 0}
poll_state = {"n": 0}

def sine_wav(freq, seconds, rate=8000):
    n = int(seconds * rate)
    data = b"".join(struct.pack("<h", int(3200 * math.sin(2 * math.pi * freq * i / rate))) for i in range(n))
    hdr = b"RIFF" + struct.pack("<I", 36 + len(data)) + b"WAVEfmt " + struct.pack("<IHHIIHH",
        16, 1, 1, rate, rate * 2, 2, 16) + b"data" + struct.pack("<I", len(data))
    return hdr + data

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        sys.stderr.write("sim: " + fmt % args + "\n")
    def _json(self, obj, code=200):
        raw = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)
    def _wav(self, raw):
        self.send_response(200)
        self.send_header("Content-Type", "audio/wav")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)
    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        body = json.loads(self.rfile.read(length) or b"{}")
        if self.path == "/generate":
            evidence["sync_submit"].append(body)
            self._wav(sine_wav(440.0, 0.5))
        elif self.path == "/suno/submit/music":
            evidence["suno_submit"].append(body)
            evidence["suno_submit_auth"].append(self.headers.get("Authorization", ""))
            self._json({"code": "success", "message": "", "data": "task-sim-1"})
        elif self.path == "/v1/make":
            evidence["auth_fail_hits"] += 1
            self._json({"error": {"message": "invalid api key"}}, 401)
        else:
            self._json({"error": "not found"}, 404)
    def do_GET(self):
        if self.path.startswith("/suno/fetch/"):
            evidence["suno_polls"] += 1
            if poll_state["n"] == 0:
                poll_state["n"] = 1
                self._json({"task_id": "task-sim-1", "status": "IN_PROGRESS"})
            else:
                base = f"http://127.0.0.1:{port}"
                self._json({"task_id": "task-sim-1", "status": "SUCCESS", "data": [
                    {"id": "clip-a", "title": "Candidate A", "audio_url": base + "/dl/a.wav",
                     "metadata": {"tags": "ambient", "duration": 61.0}, "model_name": "chirp-v3-5"},
                    {"id": "clip-b", "title": "Candidate B", "audio_url": base + "/dl/b.wav",
                     "metadata": {"tags": "ambient", "duration": 59.0}, "model_name": "chirp-v3-5"},
                ]})
        elif self.path == "/dl/a.wav":
            evidence["downloads"].append("a")
            self._wav(sine_wav(523.25, 0.5))
        elif self.path == "/dl/b.wav":
            evidence["downloads"].append("b")
            self._wav(sine_wav(659.25, 0.5))
        elif self.path == "/stats":
            self._json(evidence)
        else:
            self._json({"error": "not found"}, 404)

def dump():
    with open(evidence_path, "w", encoding="utf-8") as f:
        json.dump(evidence, f, indent=2)

import atexit
atexit.register(dump)

ThreadingHTTPServer(("127.0.0.1", port), H).serve_forever()
PY
SIM_PID=$!
log "provider simulator pid=$SIM_PID port=$SIM_PORT"

deadline=$((SECONDS + 30))
until curl -sS --max-time 2 -o /dev/null "http://127.0.0.1:$SIM_PORT/stats" 2>/dev/null; do
  kill -0 "$SIM_PID" 2>/dev/null || fail_env "provider simulator exited during startup (see $SIM_LOG)"
  (( SECONDS < deadline )) || fail_env "provider simulator not ready within 30s"
  sleep 1
done
export GATE_simulator_up=1

# user-facing config surface: the file a real user fills in. The relay
# credential arrives via the per-provider env override to exercise that path
# on the real stack; everything else is plain file config.
PROVIDER_CONFIG="$WORKDIR/config/genapi_providers.json"
cat > "$PROVIDER_CONFIG" <<EOF
{
  "providers": [
    {
      "id": "local-musicgen",
      "dialect": "sync_binary",
      "base_url": "http://127.0.0.1:$SIM_PORT",
      "generate_path": "/generate",
      "api_key": "",
      "model": "simulator/musicgen",
      "download_dir": "$WORKDIR/cache/local"
    },
    {
      "id": "relay-sim",
      "dialect": "suno_relay",
      "base_url": "http://127.0.0.1:$SIM_PORT",
      "api_key": "",
      "model": "chirp-v3-5",
      "poll_interval_seconds": 1,
      "timeout_seconds": 120,
      "download_dir": "$WORKDIR/cache/relay"
    },
    {
      "id": "broken-auth",
      "dialect": "sync_json_url",
      "base_url": "http://127.0.0.1:$SIM_PORT",
      "generate_path": "/v1/make",
      "api_key": "wrong-key",
      "download_dir": "$WORKDIR/cache/broken"
    }
  ]
}
EOF
log "user provider config: $PROVIDER_CONFIG (relay credential via env override)"

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
  echo "provider_sim_port=$SIM_PORT"
  echo "provider_config=$PROVIDER_CONFIG"
} > "$WORKDIR/run_meta.txt"

# ---------------------------------------------------------------- start kernel
log "starting kernel..."
(
  cd "$KERNEL_ROOT"
  exec "$KERNEL_BIN"
) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
KERNEL_PID=$!
log "kernel pid=$KERNEL_PID"

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  kill -0 "$KERNEL_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2; fail_functional "kernel exited during startup"; }
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
  sleep 1
done
[[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] || fail_functional "kernel ZMQ REQ port $KERNEL_PORT_REQ not listening within ${STARTUP_TIMEOUT_SECONDS}s"
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  listener_pid="$(port_listener_pid "$port")"
  [[ "$listener_pid" == "$KERNEL_PID" ]] || fail_functional "port $port listener pid $listener_pid != kernel pid $KERNEL_PID"
done
export GATE_kernel_start_ports=1
log "kernel ZMQ ports listening (owned by pid $KERNEL_PID)"

# ---------------------------------------------------------------- start agent
log "starting agent (state isolated under $AGENT_STATE_DIR; genapi config via env)..."
(
  cd "$WORKDIR/agent_cwd"
  VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
  VIT_GENAPI_CONFIG="$PROVIDER_CONFIG" \
  VIT_GENAPI_KEY_RELAY_SIM="$RELAY_ENV_KEY" \
    exec "$AGENT_BIN" \
    -http "${AGENT_HTTP#http://}" \
    -last-log-path "$AGENT_STATE_DIR/agent_last.log" \
    -vsp-hub-url ""
) > "$WORKDIR/logs/agent_stdout.log" 2>&1 &
AGENT_PID=$!
log "agent pid=$AGENT_PID"

deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  kill -0 "$AGENT_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/agent_stdout.log" >&2; fail_functional "agent exited during startup"; }
  curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && break
  sleep 1
done
curl -sS --max-time 2 -o "$WORKDIR/http/health.json" "$AGENT_HTTP/health" 2>/dev/null \
  || fail_functional "agent /health not ready within ${STARTUP_TIMEOUT_SECONDS}s"
export GATE_agent_start_health=1
log "agent /health ready"

# ---------------------------------------------------------------- P1 project + providers
log "P1: clear_project + project.new..."
invoke_tool p01_clear 60 "clear_project" '{}' \
  && gate clear_project_slate "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS" \
  || gate clear_project_slate 0 "transport/http failure"
invoke_tool p02_project_new 60 "project.new" '{}' \
  && gate project_new_ok "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS" \
  || gate project_new_ok 0 "transport/http failure"

log "P1: gen.providers — config listing without secrets..."
if invoke_tool p03_providers 60 "gen.providers" '{}'; then
  PROVIDER_COUNT="$(result_field "$REPLY_FILE" 'str(r.get("count",""))')"
  SECRET_LEAK="$(grep -c "$RELAY_ENV_KEY" "$REPLY_FILE" || true)"
  HAS_RELAY_KEY="$(result_field "$REPLY_FILE" 'str([p for p in (r.get("providers") or []) if p.get("id")=="relay-sim"][0].get("has_api_key"))')"
  if [[ "$REPLY_STATUS" == "ok" && "$PROVIDER_COUNT" == "3" && "$SECRET_LEAK" == "0" && "$HAS_RELAY_KEY" == "True" ]]; then
    gate providers_listed_secretfree 1 "count=$PROVIDER_COUNT relay has_api_key=env secret_leak=0"
  else
    gate providers_listed_secretfree 0 "status=$REPLY_STATUS count=$PROVIDER_COUNT leak=$SECRET_LEAK has_key=$HAS_RELAY_KEY"
  fi
else
  gate providers_listed_secretfree 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P2 unified template E2E
log "P2: track.add (audio host)..."
AUDIO_TRACK_ID=""
if invoke_tool p04_track_add 60 "track.add" '{}'; then
  AUDIO_TRACK_ID="$(result_field "$REPLY_FILE" 'str(r.get("track_id",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$AUDIO_TRACK_ID" ]]; then
    gate unified_host_track_ok 1 "track_id=$AUDIO_TRACK_ID"
  else
    gate unified_host_track_ok 0 "status=$REPLY_STATUS track_id=$AUDIO_TRACK_ID"
  fi
else
  gate unified_host_track_ok 0 "transport/http failure"
fi

log "P2: gen.generate (sync_binary unified template) with auto-ingest..."
INGEST1_CLIP=""; ASSET1_REF=""; TAKE1_ID=""; ART1_PATH=""
if invoke_tool p05_gen_sync 180 "gen.generate" \
  "{\"provider_id\":\"local-musicgen\",\"prompt\":\"steady test tone\",\"duration_seconds\":0.5,\"track_id\":\"$AUDIO_TRACK_ID\",\"start_time\":0.0,\"take_stack_id\":\"genapi-smoke-unified\"}"; then
  INGEST1_CLIP="$(result_field "$REPLY_FILE" 'str(((r.get("ingest") or [{}])[0]).get("clip_id",""))')"
  ASSET1_REF="$(result_field "$REPLY_FILE" 'str(((r.get("ingest") or [{}])[0]).get("asset_ref",""))')"
  TAKE1_ID="$(result_field "$REPLY_FILE" 'str(((r.get("ingest") or [{}])[0]).get("active_take_id",""))')"
  ART1_PATH="$(result_field "$REPLY_FILE" 'str(((r.get("artifacts") or [{}])[0]).get("local_path",""))')"
  ART1_FMT="$(result_field "$REPLY_FILE" 'str(((r.get("artifacts") or [{}])[0]).get("format",""))')"
  if [[ "$REPLY_STATUS" == "ok" && -n "$INGEST1_CLIP" && -n "$ASSET1_REF" && -n "$TAKE1_ID" && "$ART1_FMT" == "wav" ]]; then
    gate unified_generate_ingest_ok 1 "clip=$INGEST1_CLIP ref=${ASSET1_REF:0:20}… take=$TAKE1_ID fmt=$ART1_FMT"
  else
    gate unified_generate_ingest_ok 0 "status=$REPLY_STATUS clip=$INGEST1_CLIP ref=$ASSET1_REF take=$TAKE1_ID fmt=$ART1_FMT"
  fi
  if [[ -n "$ART1_PATH" && -f "$ART1_PATH" && "$(head -c 4 "$ART1_PATH")" == "RIFF" ]]; then
    gate unified_artifact_ondisk 1 "$(wc -c < "$ART1_PATH" | tr -d ' ')B $(basename "$ART1_PATH")"
  else
    gate unified_artifact_ondisk 0 "path=$ART1_PATH exists=$([[ -n "$ART1_PATH" && -f "$ART1_PATH" ]] && echo yes || echo no)"
  fi
else
  gate unified_generate_ingest_ok 0 "transport/http failure"
  gate unified_artifact_ondisk 0 "transport/http failure"
fi

log "P2: generated clip is addressable in project.state..."
if invoke_tool p06_state_addr 60 "project.state" '{}'; then
  if [[ -n "$INGEST1_CLIP" ]] && grep -q "\"$INGEST1_CLIP\"" "$REPLY_FILE"; then
    gate unified_clip_addressable 1 "clip $INGEST1_CLIP present in project.state"
  else
    gate unified_clip_addressable 0 "clip=$INGEST1_CLIP not found in project.state reply"
  fi
else
  gate unified_clip_addressable 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P3 suno relay型
log "P3: gen.generate on suno_relay dialect (submit/poll/download, 2 candidates)..."
SUNO_INGEST_COUNT=""; SUNO_TAKE_A=""; SUNO_TAKE_B=""; SUNO_CLIP_B=""
if invoke_tool p07_gen_suno 240 "gen.generate" \
  "{\"provider_id\":\"relay-sim\",\"prompt\":\"ambient pad over sea\",\"style_tags\":\"ambient\",\"track_id\":\"$AUDIO_TRACK_ID\",\"start_time\":2.0,\"take_stack_id\":\"genapi-smoke-suno\"}"; then
  SUNO_INGEST_COUNT="$(result_field "$REPLY_FILE" 'len([x for x in (r.get("ingest") or []) if isinstance(x, dict) and x.get("clip_id")])')"
  SUNO_TAKE_A="$(result_field "$REPLY_FILE" 'str(((r.get("ingest") or [{}])[0]).get("active_take_id",""))')"
  SUNO_CLIP_B="$(result_field "$REPLY_FILE" 'str(((r.get("ingest") or [{}])[1]).get("clip_id",""))')"
  SUNO_ARTIFACTS="$(result_field "$REPLY_FILE" 'len(r.get("artifacts") or [])')"
  STATUS_TRAIL="$(result_field "$REPLY_FILE" '",".join(r.get("status_seen") or [])')"
  if [[ "$REPLY_STATUS" == "ok" && "$SUNO_INGEST_COUNT" == "2" && -n "$SUNO_TAKE_A" && -n "$SUNO_CLIP_B" ]]; then
    gate suno_relay_two_candidates_ingested 1 "artifacts=$SUNO_ARTIFACTS ingested=$SUNO_INGEST_COUNT trail=$STATUS_TRAIL"
  else
    gate suno_relay_two_candidates_ingested 0 "status=$REPLY_STATUS artifacts=$SUNO_ARTIFACTS ingested=$SUNO_INGEST_COUNT trail=$STATUS_TRAIL"
  fi
else
  gate suno_relay_two_candidates_ingested 0 "transport/http failure"
fi

log "P3: switch_asset_take flips candidate B's clip back to candidate A's take..."
if [[ -n "$SUNO_CLIP_B" ]] && invoke_tool p08_switch_take 90 "assets.switch_take" \
  "{\"clip_id\":\"$SUNO_CLIP_B\",\"take_id\":\"$SUNO_TAKE_A\"}"; then
  ACTIVE_AFTER="$(result_field "$REPLY_FILE" 'str(r.get("active_take_id", r.get("take_id","")))')"
  if [[ "$REPLY_STATUS" == "ok" && "$ACTIVE_AFTER" == "$SUNO_TAKE_A" ]]; then
    gate suno_candidate_stack_switchable 1 "active_take=$ACTIVE_AFTER"
  else
    gate suno_candidate_stack_switchable 0 "status=$REPLY_STATUS active=$ACTIVE_AFTER want=$SUNO_TAKE_A"
  fi
else
  gate suno_candidate_stack_switchable 0 "transport/http failure (clip=$SUNO_CLIP_B)"
fi

log "P3: simulator request-shape evidence (param normalization + relay auth)..."
# Pull the live evidence snapshot over HTTP before teardown: the simulator's
# atexit dump does not run under SIGTERM.
curl -sS --max-time 5 -o "$SIM_EVIDENCE" "http://127.0.0.1:$SIM_PORT/stats" || true
python3 - "$SIM_EVIDENCE" "$RELAY_ENV_KEY" > "$WORKDIR/sim_shape_verdict.txt" <<'PY' || true
import json, sys
ev = json.load(open(sys.argv[1], encoding="utf-8"))
key = sys.argv[2]
checks = {
    "sync_prompt_normalized": bool(ev["sync_submit"]) and ev["sync_submit"][0].get("prompt") == "steady test tone",
    "sync_duration_mapped": bool(ev["sync_submit"]) and ev["sync_submit"][0].get("duration") == 0.5,
    "suno_inspiration_mode": bool(ev["suno_submit"]) and ev["suno_submit"][0].get("gpt_description_prompt") == "ambient pad over sea",
    # tags is a custom-mode-only field on the relay surface; in inspiration
    # mode the style signal belongs to the description, so the adapter must
    # NOT leak a stray tags field (dialect-correct field discipline).
    "suno_inspiration_no_stray_tags": bool(ev["suno_submit"]) and "tags" not in ev["suno_submit"][0],
    "suno_mv_from_model": bool(ev["suno_submit"]) and ev["suno_submit"][0].get("mv") == "chirp-v3-5",
    "suno_bearer_env_key": bool(ev["suno_submit_auth"]) and ev["suno_submit_auth"][0] == "Bearer " + key,
    "suno_polled_to_success": ev["suno_polls"] >= 2,
    "both_candidates_downloaded": set(ev["downloads"]) == {"a", "b"},
}
verdict = all(checks.values())
print("PASS" if verdict else "FAIL")
for k, v in checks.items():
    print(f"  {k}: {'ok' if v else 'MISS'}")
sys.exit(0 if verdict else 1)
PY
if [[ "$(head -1 "$WORKDIR/sim_shape_verdict.txt")" == "PASS" ]]; then
  gate adapter_request_shapes_normalized 1 "$(tail -n +2 "$WORKDIR/sim_shape_verdict.txt" | grep -c ok)/8 shape checks ok"
else
  gate adapter_request_shapes_normalized 0 "$(cat "$WORKDIR/sim_shape_verdict.txt" | tr '\n' ' ')"
fi

# ---------------------------------------------------------------- P4 fail-closed
log "P4: negative probes — unknown provider / 401 / missing prompt..."
if invoke_tool p09_unknown_provider 60 "gen.generate" \
  "{\"provider_id\":\"ghost\",\"prompt\":\"x\",\"track_id\":\"$AUDIO_TRACK_ID\"}"; then
  ERR_CLASS="$(result_field "$REPLY_FILE" 'str(r.get("result",{}).get("error_class",""))')"
  if [[ "$(final_status "$REPLY_FILE")" == "error" && "$ERR_CLASS" == "config_error" ]]; then
    gate failclosed_unknown_provider 1 "error_class=config_error"
  else
    gate failclosed_unknown_provider 0 "status=$(final_status "$REPLY_FILE") class=$ERR_CLASS"
  fi
else
  gate failclosed_unknown_provider 0 "transport/http failure"
fi

if invoke_tool p10_auth_fail 60 "gen.generate" \
  "{\"provider_id\":\"broken-auth\",\"prompt\":\"x\",\"track_id\":\"$AUDIO_TRACK_ID\"}"; then
  ERR_CLASS="$(result_field "$REPLY_FILE" 'str(r.get("result",{}).get("error_class",""))')"
  ERR_HTTP="$(result_field "$REPLY_FILE" 'str(r.get("result",{}).get("http_status",""))')"
  if [[ "$(final_status "$REPLY_FILE")" == "error" && "$ERR_CLASS" == "auth_error" && "$ERR_HTTP" == "401" ]]; then
    gate failclosed_auth_classified 1 "error_class=auth_error http=401"
  else
    gate failclosed_auth_classified 0 "status=$(final_status "$REPLY_FILE") class=$ERR_CLASS http=$ERR_HTTP"
  fi
else
  gate failclosed_auth_classified 0 "transport/http failure"
fi

if invoke_tool p11_no_prompt 60 "gen.generate" \
  "{\"provider_id\":\"local-musicgen\",\"track_id\":\"$AUDIO_TRACK_ID\"}"; then
  ERR_CLASS="$(result_field "$REPLY_FILE" 'str(r.get("result",{}).get("error_class",""))')"
  if [[ "$(final_status "$REPLY_FILE")" == "error" && "$ERR_CLASS" == "config_error" ]]; then
    gate failclosed_empty_request 1 "error_class=config_error (pre-HTTP)"
  else
    gate failclosed_empty_request 0 "status=$(final_status "$REPLY_FILE") class=$ERR_CLASS"
  fi
else
  gate failclosed_empty_request 0 "transport/http failure"
fi

log "P4: command surface alive after failures..."
if invoke_tool p12_alive 30 "project.state" '{}'; then
  gate surface_alive_after_failures "$( [[ "$REPLY_STATUS" == "ok" ]] && echo 1 || echo 0 )" "status=$REPLY_STATUS"
else
  gate surface_alive_after_failures 0 "transport/http failure"
fi

# ---------------------------------------------------------------- P5 close
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
log "stopping kernel (pid $KERNEL_PID)..."
stop_process "$KERNEL_PID"
KERNEL_STOP_RECORD="$STOP_RECORD"
log "stopping provider simulator (pid $SIM_PID)..."
stop_process "$SIM_PID"
KERNEL_PID=""; AGENT_PID=""; SIM_PID=""

# ---------------------------------------------------------------- summary
SUMMARY_FILE="$WORKDIR/summary.json"
GATE_NAMES="simulator_up kernel_start_ports agent_start_health clear_project_slate project_new_ok \
providers_listed_secretfree unified_host_track_ok unified_generate_ingest_ok unified_artifact_ondisk \
unified_clip_addressable suno_relay_two_candidates_ingested suno_candidate_stack_switchable \
adapter_request_shapes_normalized failclosed_unknown_provider failclosed_auth_classified \
failclosed_empty_request surface_alive_after_failures"
export WORKDIR SUMMARY_FILE RUN_ID GATE_NAMES
export KERNEL_STOP_RECORD AGENT_STOP_RECORD FAIL_CLASSIFICATION CONFIRMATION_CARDS
export INGEST1_CLIP ASSET1_REF TAKE1_ID SUNO_CLIP_B SUNO_TAKE_A AGENT_SHA KERNEL_SHA REPO_ROOT
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
    "schema_version": "genapi.adapter.smoke.mac.v1",
    "card": "GEN-API-ADAPTER-1",
    "run_id": os.environ["RUN_ID"],
    "overall_status": "PASS" if all(gates.values()) else "FAIL",
    "failure_kind": os.environ.get("FAIL_CLASSIFICATION") or ("none" if all(gates.values()) else "functional"),
    "boundary_declaration": "generation provider = local self-hosted-type simulator over real HTTP; no external provider key was available in this environment (external leg declared as environment blocker in the card receipt, not faked)",
    "repo": {"root": os.environ["REPO_ROOT"]},
    "binaries": {
        "kernel_sha256": os.environ["KERNEL_SHA"],
        "agent_sha256": os.environ["AGENT_SHA"],
    },
    "gates": gates,
    "gate_failure_details": fail_details,
    "recorded": {
        "confirmation_cards_surfaced": int(os.environ.get("CONFIRMATION_CARDS", "0")),
        "ids": {
            "unified_ingest_clip": os.environ.get("INGEST1_CLIP", ""),
            "unified_asset_ref": os.environ.get("ASSET1_REF", ""),
            "unified_take": os.environ.get("TAKE1_ID", ""),
            "suno_candidate_b_clip": os.environ.get("SUNO_CLIP_B", ""),
            "suno_candidate_a_take": os.environ.get("SUNO_TAKE_A", ""),
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
  log "SMOKE INCOMPLETE: failing gates: $(cut -d: -f1 "$GATE_FAIL_LOG" | tr '\n' ' ')"
  exit 1
fi
log "SMOKE COMPLETE: all gates green (exit 0)"
exit 0
