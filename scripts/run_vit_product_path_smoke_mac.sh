#!/usr/bin/env bash
# run_vit_product_path_smoke mac equivalent — PORT-SMOKE-MAC-1 (script 5/7).
#
# Mac port of scripts/run_vit_product_path_smoke.ps1 (3088 lines) restricted
# to the ps1's DEFAULT path (no *AgentOnly switches): the product-path
# lifecycle + mix smoke — fixture project through agent + live kernel, clip
# fade/gain agent closed-loop, read-only product observe, Chinese multitrack
# MOM observation, no-pending confirmation guard, vocal focus relationship
# observation, vocal clarification loop.
#
# Declared mac adaptation (card: "两件套栈即可，不等 hub"): the ps1 launches a
# GODOT-owned lifecycle (Godot project autostarts kernel+VSP Hub+agent, then
# asserts hub health + Godot-owned ports + binary hashes). The VSP Hub is not
# ported to mac yet (PORT-VSPHUB-1 is building it in parallel), so this mac
# script starts the two-piece stack DIRECTLY (kernel+agent, A5/C2/JOURNEY
# pattern: fake VitApp root, VIT_PROJECT_XML copy, draft-root/orchestration
# store isolation, -vsp-hub-url "") and skips the hub/Godot-lifecycle
# assertions. The chat context reports the truthful mac interaction path.
# Everything downstream of stack start is assertion-for-assertion identical.
#
#   | ps1 (run_vit_product_path_smoke.ps1)      | mac (this script)             |
#   |--------------------------------------------|-------------------------------|
#   | param block (RepoRoot/Godot*/UI/Kernel/    | flags below; Godot/UI/Hub     |
#   |   Agent/VspHub + many -Only switches)      | params dropped (adaptation)   |
#   | Build agent + VspHub (repo agent\bin)      | go build into the run workdir |
#   | Start Godot project → autostart children   | direct kernel+agent start     |
#   |   + hub health + Godot-owned port asserts  |   under isolated roots (§10); |
#   |   + binary hash parity (hub/agent)         |   port ownership + agent      |
#   |                                            |   /health instead             |
#   | $AgentLog = repo Workspace\Logs\           | -last-log-path into the run   |
#   |   agent_last.log                           | workdir (same patterns)       |
#   | mixboard feature snapshot at repo          | VIT_MIXBOARD_ROOT=$run/mix-   |
#   |   VitApp\Workspace\Artifacts\ (authorita-  | board → snapshot at $run/     |
#   |   tive) + Godot split-staleness check      | mixboard_feature_snapshot.json|
#   |                                            | (authoritative; the Godot     |
#   |                                            | split check is N/A without a |
#   |                                            | Godot-owned lifecycle)        |
#   | chat context interaction_path=             | interaction_path=agent_http_  |
#   |   agent_http_after_godot_project_lifecycle |   after_direct_stack_lifecycle|
#   |   product_lifecycle="godot_project"        |   product_lifecycle=direct_   |
#   |                                            |   two_piece_stack (only       |
#   |                                            |   "blind_project_smoke" is    |
#   |                                            |   keyed on in the agent)      |
#   | Join-UnicodeChars code-point strings       | plain UTF-8 literals          |
#   | Invoke-WebRequest / Get-NetTCPListener     | curl / lsof                   |
#   | Track1/2 = repo root fixture wavs          | same (tracked in git)         |
#   | Reset-FixtureProject (new+clear)           | identical                     |
#   | Clip fade/gain loop (ui context, 4 turns,  | identical bodies/needles/     |
#   |   clip.fade.read/gain.read/set + asserts)  | route+value asserts           |
#   | Observe turn (read-only assert + bridge    | identical + snapshot assert   |
#   |   readiness) + authoritative snapshot      |   at the mac mixboard root    |
#   | Multitrack MOM observe (v1.5 intent,       | identical (same JSON fields)  |
#   |   coverage>=2, llm_context guard)          |                               |
#   | Confirm-tick block behind pending_candidate| identical (unreachable in the |
#   |   null check                               | default path on both sides)   |
#   | No-pending guard / vocal focus / vocal     | identical stop_reason sets,   |
#   |   clarification loop (ask/answer/confirm,  | route alias sets, log         |
#   |   AB Result needle, events checks)         | patterns, event scans         |
#   | summary.json vit_product_path_smoke.v1     | .mac.v1 + run meta + §8 note  |
#
# LLM precondition: the product-path turns are real LLM turns — the same
# config preflight as JOURNEY-1-MAC (~/.vit/config.json or env); the key
# never reaches logs/artifacts.
#
# §8 discipline (pre-declared on the card): ≤3 valid runs; success = single
# run exit 0 with every turn assertion green; failures classified; two
# failures at the same deterministic breakpoint stop the reruns.

set -uo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
WAIT_SECONDS=60
CHAT_TIMEOUT_SEC=240
CHAT_SETTLE_SECONDS=300
KEEP_PROCESSES=0
WORKDIR_ARG=""
TRACKTION_DIR=""
ARTIFACT_ROOT_ARG=""

KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

# Journey literals — ps1 code-point arrays decoded once (UTF-8 rides directly).
READ_CLIP_STATE_MESSAGE="读取当前选中 clip 的 fade 和 gain 状态"
FADE_SET_MESSAGE="把当前选中 clip 的 fade in 设置为 0.15 秒，fade out 设置为 0.25 秒"
OBSERVE_MESSAGE="帮我看整体混音"
MULTITRACK_MESSAGE="比较一下各轨频段占用和声像关系，不要修改。"
CONFIRM_MESSAGE="可以执行"
VOCAL_FOCUS_MESSAGE="make the lead vocal more forward"
VOCAL_FORWARD_MESSAGE="让主唱更靠前"
VOCAL_ANSWER_MESSAGE="Track 1 是主唱"

usage() {
  cat <<'EOF'
run_vit_product_path_smoke_mac.sh — mac port of run_vit_product_path_smoke.ps1
default path (PORT-SMOKE-MAC-1; two-piece stack, hub/Godot lifecycle adapted
out until PORT-VSPHUB-1 lands).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary
  --skip-agent-build      With --agent-bin: do not build the agent
  --agent-bin PATH        Agent binary to start (only with --skip-agent-build)
  --wait-seconds N        Startup wait (default 60)
  --chat-timeout-sec N    /agent/chat timeout (default 240)
  --chat-settle-seconds N Budget for waiting a sliced-out turn's durable
                          continuation to settle (default 300)
  --keep-processes        Do not stop the stack this run started
  --workdir PATH          Reuse PATH as the run artifact dir
  --tracktion-dir PATH    tracktion_engine source dir (kernel build only)
  --artifact-root DIR     Artifact root (default ~/Documents/vit-smoke-mac1-artifacts)
  -h, --help              Show this help

Exit codes: 0 = product-path smoke passed; 1 = a turn assertion failed;
2 = environment failure.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --wait-seconds) WAIT_SECONDS="$2"; shift 2 ;;
    --chat-timeout-sec) CHAT_TIMEOUT_SEC="$2"; shift 2 ;;
    --chat-settle-seconds) CHAT_SETTLE_SECONDS="$2"; shift 2 ;;
    --keep-processes) KEEP_PROCESSES=1; shift ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    --artifact-root) ARTIFACT_ROOT_ARG="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in go cmake make python3 lsof shasum curl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" >&2; }
warn() { echo "warn: $*" >&2; }
fail_env()       { OUTCOME="env_failure"; echo "ERROR[env]: $*" >&2; exit 2; }
fail_functional(){ OUTCOME="assertion_failed"; echo "ERROR[functional]: $*" >&2; exit 1; }

json_field() {
  python3 - "$1" "$2" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
print(eval(sys.argv[2], {"d": d}))
PY
}

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

RUN_ID="vit_product_path_mac_$(date '+%Y%m%d-%H%M%S')"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac1-artifacts}"
WORKDIR="${WORKDIR_ARG:-$ARTIFACT_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR"/{http,bodies,logs} || fail_env "cannot create artifact dir: $WORKDIR"
AGENT_LOG="$WORKDIR/agent_last.log"
STOP_REASONS_ENV="$WORKDIR/stop_reasons.env"
: > "$STOP_REASONS_ENV"

KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"
KERNEL_WORKSPACE="$WORKDIR/kernel_workspace"
mkdir -p "$KERNEL_WORKSPACE"
AGENT_DRAFTS="$WORKDIR/agent_drafts"
AGENT_STATE_DIR="$WORKDIR/agent_state"
AGENT_CWD="$WORKDIR/agent_cwd"
MIXBOARD_ROOT="$WORKDIR/mixboard"
mkdir -p "$AGENT_DRAFTS" "$AGENT_STATE_DIR" "$AGENT_CWD" "$MIXBOARD_ROOT"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
# mixboard.DefaultRoot(): snapshot lands at Dir(VIT_MIXBOARD_ROOT)/
# mixboard_feature_snapshot.json — the mac authoritative-snapshot location.
FEATURE_SNAPSHOT_PATH="$WORKDIR/mixboard_feature_snapshot.json"

KERNEL_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
OUTCOME="env_failure"
KERNEL_BIN=""
AGENT_BIN=""

record_stop_reason() { echo "$1=$2" >> "$STOP_REASONS_ENV"; }

stop_process() {
  local pid="$1" start elapsed code signal="SIGTERM"
  kill -TERM "$pid" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS - start < 10 )) || { kill -KILL "$pid" 2>/dev/null || true; signal="SIGTERM+SIGKILL"; break; }
    sleep 1
  done
  wait "$pid" 2>/dev/null
  code=$?
  elapsed=$((SECONDS - start))
  STOP_RECORD="${signal}:${code}:${elapsed}"
}

cleanup() {
  local rc=$?
  trap - EXIT INT TERM
  if [[ "$KEEP_PROCESSES" -ne 1 ]]; then
    [[ -z "$AGENT_PID" ]] || {
      log "stopping agent (pid $AGENT_PID)..."
      stop_process "$AGENT_PID"; AGENT_STOP_RECORD="$STOP_RECORD"; log "agent stop: $AGENT_STOP_RECORD"
    }
    [[ -z "$KERNEL_PID" ]] || {
      log "stopping kernel (pid $KERNEL_PID)..."
      stop_process "$KERNEL_PID"; KERNEL_STOP_RECORD="$STOP_RECORD"; log "kernel stop: $KERNEL_STOP_RECORD"
    }
  fi
  AGENT_PID=""; KERNEL_PID=""
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    local busy=""
    for port in "$AGENT_HTTP_PORT" "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy="$busy $port"
    done
    [[ -z "$busy" ]] && break
    sleep 1
  done
  [[ -f "$AGENT_LOG" ]] && tail -120 "$AGENT_LOG" > "$WORKDIR/agent_log_tail.txt" 2>/dev/null
  python3 - "$WORKDIR/summary.json" "$RUN_ID" "$WORKDIR" "$OUTCOME" \
    "$KERNEL_STOP_RECORD" "$AGENT_STOP_RECORD" "$KERNEL_BIN" "$AGENT_BIN" "$STOP_REASONS_ENV" <<'PY' 2>/dev/null || true
import hashlib, json, os, sys
out, run_id, workdir, outcome, kernel_stop, agent_stop, kernel_bin, agent_bin, stop_env = sys.argv[1:10]
def sha(path):
    try:
        return hashlib.sha256(open(path, "rb").read()).hexdigest()
    except Exception:
        return ""
stop_reasons = {}
if os.path.isfile(stop_env):
    for line in open(stop_env, encoding="utf-8"):
        line = line.strip()
        if "=" in line:
            k, v = line.split("=", 1)
            stop_reasons[k] = v
summary = {
    "schema_version": "vit_product_path_smoke.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_vit_product_path_smoke.ps1",
    "mac_adaptation": "two-piece direct stack (hub/Godot lifecycle pending PORT-VSPHUB-1); default-path assertions otherwise identical",
    "run_id": run_id,
    "overall_status": "PASS" if outcome == "all_green" else outcome,
    "outcome": outcome,
    "interaction_path": "agent_http_after_direct_stack_lifecycle",
    "product_lifecycle": "direct_two_piece_stack",
    "kernel_exe": kernel_bin, "kernel_sha256": sha(kernel_bin) if kernel_bin else "",
    "agent_binary": agent_bin, "agent_sha256": sha(agent_bin) if agent_bin else "",
    "kernel_stop": {"record": kernel_stop}, "agent_stop": {"record": agent_stop},
    "stop_reasons": stop_reasons,
    "full_responses": sorted(os.path.basename(p) for p in os.listdir(os.path.join(workdir, "http")) if p.endswith(".json")),
    "agent_log_tail": "agent_log_tail.txt",
    "artifacts_dir": workdir,
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
PY
  log "workdir: $WORKDIR"
  case "$OUTCOME" in
    all_green) exit 0 ;;
    assertion_failed) exit 1 ;;
    *) exit 2 ;;
  esac
}
trap cleanup EXIT INT TERM

AGENT_HTTP_ADDR="${AGENT_HTTP#http://}"
AGENT_HTTP_ADDR="${AGENT_HTTP_ADDR#https://}"
AGENT_HTTP_PORT="${AGENT_HTTP_ADDR##*:}"

# ---------------------------------------------------------------- preflight
step "Preflight: LLM engine config (presence only — the key never reaches logs)"
LLM_CFG_SUMMARY="$(python3 - <<'PY'
import json, os
def env(name):
    return os.environ.get(name, "").strip()
cfg = {}
path = os.path.expanduser("~/.vit/config.json")
if os.path.isfile(path):
    try:
        cfg = json.load(open(path, encoding="utf-8"))
    except Exception as e:
        print(json.dumps({"complete": False, "reason": "config parse error: %s" % e}))
        raise SystemExit
base = str(cfg.get("baseUrl", "")).strip() or env("VIT_AGENT_LLM_BASE_URL") or env("OPENAI_BASE_URL")
key  = str(cfg.get("apiKey", "")).strip()  or env("VIT_AGENT_LLM_API_KEY") or env("OPENAI_API_KEY")
model= str(cfg.get("defaultModel", "")).strip() or env("VIT_AGENT_LLM_MODEL") or env("OPENAI_MODEL")
host = base.split("//")[-1].split("/")[0] if base else ""
print(json.dumps({
    "config_file_exists": os.path.isfile(path), "has_api_key": bool(key),
    "model": model, "base_url_host": host, "complete": bool(base and key and model),
}))
PY
)" || fail_env "LLM config preflight crashed"
log "llm_config=$LLM_CFG_SUMMARY"
python3 -c 'import json,sys; sys.exit(0 if json.loads(sys.argv[1])["complete"] else 1)' "$LLM_CFG_SUMMARY" \
  || fail_env "LLM engine config incomplete (card stop condition): $LLM_CFG_SUMMARY"

step "Preflight: run root + repo fingerprints + ports"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_vit_product_path_smoke_mac.sh (mac port of run_vit_product_path_smoke.ps1 default path)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "interaction_path=agent_http_after_direct_stack_lifecycle"
  echo "llm_config=$LLM_CFG_SUMMARY"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$REPO_DEFAULT_PROJECT" ]] || fail_env "repo default project missing: $REPO_DEFAULT_PROJECT"
TRACK1_PATH="$REPO_ROOT/test_100hz_10s.wav"
TRACK2_PATH="$REPO_ROOT/test_target_3s.wav"
[[ -f "$TRACK1_PATH" ]] || fail_env "track 1 fixture missing: $TRACK1_PATH"
[[ -f "$TRACK2_PATH" ]] || fail_env "track 2 fixture missing: $TRACK2_PATH"

for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "$AGENT_HTTP_PORT"; do
  pid_busy="$(port_listener_pid "$port")"
  [[ -z "$pid_busy" ]] || fail_env "port $port already has a listener (pid $pid_busy); AGENTS §9 single-owner rule"
done

# ---------------------------------------------------------------- binaries
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  log "using provided kernel binary: $KERNEL_BIN (sha256/mtime recorded)"
else
  if [[ -z "$TRACKTION_DIR" ]]; then TRACKTION_DIR="$REPO_ROOT/tracktion_engine"; fi
  [[ -f "$TRACKTION_DIR/CMakeLists.txt" ]] \
    || fail_env "tracktion_engine not usable at $TRACKTION_DIR (pass --tracktion-dir or --kernel-bin)"
  log "building VitApp kernel (cmake+make)..."
  if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fail_env "kernel cmake configure failed"
  fi
  if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp > "$WORKDIR/logs/kernel_build.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_build.log" >&2
    fail_env "kernel make failed"
  fi
  KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  [[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary not found after build"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "kernel built: $KERNEL_BIN"
fi

if [[ "$SKIP_AGENT_BUILD" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" ]] || fail_env "--skip-agent-build requires --agent-bin"
  [[ -x "$AGENT_BIN_ARG" ]] || fail_env "agent binary is not executable: $AGENT_BIN_ARG"
  AGENT_BIN="$AGENT_BIN_ARG"
else
  [[ -z "$AGENT_BIN_ARG" ]] || fail_env "--agent-bin is only valid together with --skip-agent-build"
  AGENT_BIN="$WORKDIR/bin/vitagent"
  mkdir -p "$WORKDIR/bin"
  log "building agent (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$AGENT_BIN" ./cmd/vitagent) || fail_env "go build agent failed"
fi
shasum -a 256 "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256"

# ---------------------------------------------------------------- http helpers
http_json() {
  local method="$1" url="$2" body_file="$3" out="$4" timeout="${5:-60}" code
  if [[ -n "$body_file" ]]; then
    code="$(curl -sS --max-time "$timeout" -o "$out" -w '%{http_code}' \
      -X "$method" -H 'Content-Type: application/json; charset=utf-8' \
      --data-binary "@$body_file" "$url" 2>/dev/null)" || return 1
  else
    code="$(curl -sS --max-time "$timeout" -o "$out" -w '%{http_code}' \
      -X "$method" "$url" 2>/dev/null)" || return 1
  fi
  printf '%s' "$code"
}

BUSY_RETRY_SECONDS=5
BUSY_RETRY_MAX=12

invoke_json_busy_retry() {
  # mac machine fact (JOURNEY-1-MAC): offline renders hold the engine; retry
  # the 60s window on "Engine is busy rendering" replies. Assertions unchanged.
  local method="$1" url="$2" body="$3" out="$4" timeout="${5:-120}" code attempt
  for attempt in $(seq 0 "$BUSY_RETRY_MAX"); do
    if code="$(http_json "$method" "$url" "$body" "$out" "$timeout")"; then
      if grep -q 'Engine is busy rendering' "$out" 2>/dev/null; then
        if (( attempt < BUSY_RETRY_MAX )); then
          log "kernel busy rendering — retry $((attempt + 1))/$BUSY_RETRY_MAX in ${BUSY_RETRY_SECONDS}s"
          sleep "$BUSY_RETRY_SECONDS"
          continue
        fi
      fi
      printf '%s' "$code"
      return 0
    else
      return 1
    fi
  done
}

invoke_tool() {
  # invoke_tool <tool> <args-json-file-or-'-'> <confirmed 0|1> <out-prefix> [timeout]
  local tool="$1" args_path="$2" confirmed="$3" prefix="$4" timeout="${5:-120}" code
  python3 - "$tool" "$args_path" "$confirmed" "$WORKDIR/bodies/tool_invoke.json" <<'PY'
import json, sys
tool, args_path, confirmed, out = sys.argv[1:5]
args = json.load(open(args_path, encoding="utf-8")) if args_path != "-" else {}
json.dump({"tool": tool, "args": args, "confirmed": confirmed == "1",
           "source": "vit_product_path_smoke"}, open(out, "w", encoding="utf-8"), ensure_ascii=False)
PY
  code="$(invoke_json_busy_retry POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/tool_invoke.json" "${prefix}.json" "$timeout")" \
    || fail_env "invoke $tool transport failed"
  [[ "$code" =~ ^2 ]] || fail_env "invoke $tool returned HTTP $code (see ${prefix}.json)"
  json_field "${prefix}.json" 'str(d.get("status",""))'
}

agent_chat_ctx() {
  # agent_chat_ctx <conversation-id> <message> <extra-context-json-or-'-'> <out-prefix>
  # Posts the turn, then (mac chat_settle anchor, JOURNEY-1-MAC
  # waiting_continue precedent) waits for a sliced-out turn's durable
  # continuation to settle and reads the settled reply / effective
  # stop_reason + executed route from the conversation events surface.
  # Assertions read <prefix>_settled.json; the raw response is preserved.
  local conv="$1" message="$2" extra="$3" prefix="$4" code
  python3 - "$conv" "$message" "$extra" "$WORKDIR/bodies/chat.json" <<'PY'
import json, sys
context = {
    "agent_mode": "chat",
    "interaction_path": "agent_http_after_direct_stack_lifecycle",
    "product_path_smoke": True,
    "product_lifecycle": "direct_two_piece_stack",
}
if sys.argv[3] != "-":
    context.update(json.loads(sys.argv[3]))
json.dump({"conversation_id": sys.argv[1], "message": sys.argv[2], "context": context},
          open(sys.argv[4], "w", encoding="utf-8"), ensure_ascii=False)
PY
  code="$(http_json POST "$AGENT_HTTP/agent/chat" "$WORKDIR/bodies/chat.json" "${prefix}.json" "$CHAT_TIMEOUT_SEC")" \
    || fail_env "chat transport failed (conversation $conv)"
  [[ "$code" =~ ^2 ]] || fail_env "chat returned HTTP $code (conversation $conv)"
  local goal_status
  goal_status="$(json_field "${prefix}.json" 'str(d.get("goal_status",""))')"
  if [[ "$goal_status" != "waiting_continue" ]]; then
    cp "${prefix}.json" "${prefix}_settled.json"
    return 0
  fi
  log "turn sliced out (waiting_continue/limit_reached) — waiting for the durable continuation to settle (budget ${CHAT_SETTLE_SECONDS}s)"
  local deadline=$((SECONDS + CHAT_SETTLE_SECONDS)) polled=""
  while (( SECONDS < deadline )); do
    sleep 5
    local pcode
    pcode="$(http_json GET "$AGENT_HTTP/agent/runtime/status" "" "$WORKDIR/bodies/settle_poll.json" 30)" || { sleep 1; continue; }
    [[ "$pcode" =~ ^2 ]] || { sleep 1; continue; }
    polled="$(json_field "$WORKDIR/bodies/settle_poll.json" 'str((d.get("goal") or {}).get("status",""))')"
    log "settle poll: goal=$polled"
    case "$polled" in
      completed|failed|stopped|cancelled|waiting_confirmation|waiting_clarification) break ;;
    esac
  done
  printf '%s' "$polled" > "${prefix}_settled_goal.txt"
  http_json GET "$AGENT_HTTP/agent/events?conversation_id=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$conv")&since=0&limit=500" "" "${prefix}_events.json" 60 >/dev/null || true
  python3 - "$prefix" "$polled" <<'SETTLE_PY'
import json, sys
prefix, settled_goal = sys.argv[1:3]
placeholder = "我还在继续处理这个任务，完成后再向你汇报。"
raw = json.load(open(f"{prefix}.json", encoding="utf-8"))
events = {}
try:
    events = json.load(open(f"{prefix}_events.json", encoding="utf-8"))
except Exception:
    pass
delivered, tool_rows, turn_completed_texts = [], [], []
for ev in (events.get("events") or []):
    parts = [str(ev.get(k, "")) for k in ("title", "body", "summary") if str(ev.get(k, "")).strip()]
    if parts:
        delivered.append(" | ".join(parts))
    if str(ev.get("type", "")) == "turn.completed":
        text = " ".join(str(ev.get(k, "")) for k in ("title", "body") if str(ev.get(k, "")).strip())
        if text:
            turn_completed_texts.append(text)
    sources = [ev]
    payload = ev.get("payload")
    if isinstance(payload, dict):
        sources.append(payload)
    for src in sources:
        if not isinstance(src, dict):
            continue
        name = str(src.get("tool") or src.get("command_name") or src.get("command") or "")
        if name and {"tool": name} not in tool_rows:
            tool_rows.append({"tool": name})
reply_candidates = [t for t in delivered if placeholder not in t] \
                   + [t for t in turn_completed_texts if placeholder not in t]
settled_reply = reply_candidates[-1] if reply_candidates else str(raw.get("reply", ""))
mapping = {
    "waiting_confirmation": "needs_confirmation",
    "waiting_clarification": "needs_clarification",
    "completed": "done",
}
out = dict(raw)
out["stop_reason"] = mapping.get(settled_goal, str(raw.get("stop_reason", "")))
out["goal_status"] = settled_goal or "settle_timeout"
out["reply"] = settled_reply
out["settled_from"] = "continuation+events (mac chat_settle anchor)"
out["raw_stop_reason"] = raw.get("stop_reason", "")
out["raw_reply"] = raw.get("reply", "")
out["delivered_event_count"] = len(delivered)
if not out.get("executed_kernel_reply") and tool_rows:
    out["executed_kernel_reply"] = tool_rows
json.dump(out, open(f"{prefix}_settled.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
SETTLE_PY
}

events_get() {
  # events_get <conversation-id> <since> <limit> <out-file>
  local conv="$1" since="$2" limit="$3" out="$4" code
  code="$(http_json GET "$AGENT_HTTP/agent/events?conversation_id=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$conv")&since=$since&limit=$limit" "" "$out" 30)" \
    || fail_env "GET /agent/events failed (conversation $conv)"
  [[ "$code" =~ ^2 ]] || fail_env "GET /agent/events returned HTTP $code (conversation $conv)"
}

set_ui_context() {
  # set_ui_context <context-json-file> <out-file>
  local code
  code="$(http_json POST "$AGENT_HTTP/agent/ui/context" "$1" "$2" 10)" \
    || fail_env "POST /agent/ui/context transport failed"
  [[ "$code" =~ ^2 ]] || fail_env "agent ui context update refused HTTP $code"
  [[ "$(json_field "$2" 'str(d.get("status",""))')" == "ok" ]] \
    || fail_functional "agent ui context update failed: $(head -c 300 "$2")"
}

wait_log_pattern() {
  local pattern="$1" timeout="$2" deadline=$((SECONDS + timeout)) line
  while (( SECONDS < deadline )); do
    if [[ -f "$AGENT_LOG" ]]; then
      line="$(grep -F "$pattern" "$AGENT_LOG" 2>/dev/null | head -1)"
      [[ -n "$line" ]] && { printf '%s' "$line"; return 0; }
    fi
    sleep 0.25
  done
  fail_functional "Timed out waiting for log pattern: $pattern"
}

route_helpers() {
  : # (reserved: shared route-assert helpers live in route_counts below)
}

route_counts() {
  # route_counts <response.json> — prints {"propose":n,"apply":n,"observe":n,
  # "daw_invoke":n,"track_volume":n,"derive":n,"names":[...]}
  python3 - "$1" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
rows = d.get("executed_kernel_reply") or []
if isinstance(rows, dict):
    rows = [rows]
names = []
for row in rows:
    if isinstance(row, str):
        names.append(row)
        continue
    for key in ("tool", "command_name", "command"):
        name = str(row.get(key) or "")
        if name:
            names.append(name)
def count(*aliases):
    return sum(1 for n in names if n in aliases)
print(json.dumps({
    "names": names,
    "propose": count("mix.propose_tick", "mix_propose_tick"),
    "apply": count("mix.apply_tick", "mix_apply_tick"),
    "observe": count("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation", "ccb.observation_catalog", "ccb_observation_catalog"),
    "derive": count("mix.derive", "mix_derive"),
    "daw_invoke": count("daw.invoke", "daw_invoke"),
    "track_volume": count("track.volume", "track_volume"),
}))
PY
}

assert_observation_shape() {
  # assert_observation_shape <response.json> <mode: bridge|mom> [expected-goal-text]
  python3 - "$1" "$2" "${3:-}" <<'PY'
import json, sys

path, mode, expected_goal = sys.argv[1:4]
d = json.load(open(path, encoding="utf-8"))
rows = d.get("executed_kernel_reply") or []
if isinstance(rows, dict):
    rows = [rows]
obs = None
for row in rows:
    if isinstance(row, dict):
        result = row.get("result") or {}
        if isinstance(result.get("observation"), dict):
            obs = result["observation"]
            break
if obs is None:
    print(f"FAIL: {path} did not expose a mix observation result", file=sys.stderr)
    sys.exit(1)

def num(value):
    try:
        return float(str(value).strip())
    except Exception:
        return None

def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)

def bridge_snapshot_check(snapshot, label):
    latest = (snapshot or {}).get("latest_request") or {}
    request_id = str(latest.get("request_id") or "")
    if not request_id:
        fail(f"{label} feature_snapshot.latest_request.request_id missing")
    requested = {(r or {}).get("feature_type") for r in (latest.get("requested_features") or []) if isinstance(r, dict)}
    for feat in ("waveform_envelope", "spectral_field"):
        if feat not in requested:
            fail(f"{label} latest_request.requested_features missing {feat}")
    for name in ("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"):
        row = (snapshot or {}).get(name) or {}
        status = str(row.get("status") or "")
        if not status:
            fail(f"{label} acoustic feature {name} missing status")
        row_req = str(row.get("request_id") or "")
        if request_id and row_req and row_req != request_id:
            fail(f"{label} acoustic feature {name} request_id mismatch: got={row_req} expected={request_id}")
        if status in ("ready", "partial"):
            if not row_req:
                fail(f"{label} ready acoustic feature {name} missing request_id")
            if not str(row.get("source") or ""):
                fail(f"{label} ready acoustic feature {name} missing source")
            if name == "spectrogram_tiles":
                if str(row.get("feature_type") or "") != "spectral_field":
                    fail(f"{label} spectrogram_tiles feature_type is not spectral_field")
                if str(row.get("source") or "") not in ("kernel_tile_ready_direct_collector", "kernel_tile_ready_godot_bridge"):
                    fail(f"{label} spectrogram_tiles source is not accepted: {row.get('source')}")
                seen, expected = num(row.get("tile_count_seen")), num(row.get("tile_count_expected"))
                if seen is None or seen < 1 or expected is None:
                    fail(f"{label} spectrogram_tiles missing tile counts")
            continue
        if status in ("building", "requested", "pending", "missing", "blocked", "unavailable", "invalid", "stale", "suspect", "deferred", "failed"):
            reason = str(row.get("reason") or "")
            progress_reason = str((row.get("progress") or {}).get("reason") or "")
            if not reason and not progress_reason:
                fail(f"{label} acoustic feature {name} status {status} missing explicit reason/progress")
            continue
        fail(f"{label} acoustic feature {name} has unexpected status {status}")

if mode == "bridge":
    snapshot = ((obs.get("global_summary") or {}).get("feature_snapshot") or {})
    bridge_snapshot_check(snapshot, "product observe")
    mix_package = obs.get("mix_package") or {}
    current_metrics = mix_package.get("current_metrics") or {}
    for name in ("band_energy", "stereo_relation"):
        row = current_metrics.get(name) or {}
        if str(row.get("status") or "") == "missing" and not str(row.get("reason") or ""):
            fail(f"product observe current_metrics.{name} missing explicit reason")
    print("ok: acoustic bridge readiness assertions passed")
    sys.exit(0)

# mode == "mom"
target = obs.get("target_ref") or {}
if str(target.get("kind")) != "project" or str(target.get("id")) != "current":
    fail(f"expected project target, got kind={target.get('kind')} id={target.get('id')}")
listen_mode = str((((obs.get("listen_scope") or {}).get("source")) or {}).get("mode") or "")
if listen_mode != "full_project":
    fail(f"expected listen_scope.source.mode=full_project, got {listen_mode}")
mom = obs.get("mom_projection") or {}
intent = str((mom.get("intent_policy") or {}).get("name") or mom.get("intent") or "")
if intent != "project_multitrack_relation_observation":
    fail(f"expected MOM project_multitrack_relation_observation, got {intent}")
relation = mom.get("multitrack_relation") or {}
if str(relation.get("status") or "") == "not_applicable_single_track":
    fail("unexpectedly downgraded to not_applicable_single_track")
compared = num((mom.get("trust_quality") or {}).get("coverage", {}).get("compared_track_count")) \
    if isinstance((mom.get("trust_quality") or {}).get("coverage"), dict) else None
if compared is None or compared < 2:
    fail(f"expected compared_track_count>=2, got {compared}")
tracks = obs.get("project_package") or {}
project_tracks = tracks.get("tracks") or []
if len(project_tracks) < 2:
    fail(f"expected project_package.tracks>=2, got {len(project_tracks)}")
llm_context = mom.get("llm_context") or {}
if llm_context.get("do_not_include_raw_package") is not True:
    fail("MOM llm_context did not block raw package")
if expected_goal:
    goal = str((obs.get("mix_package") or {}).get("goal_text") or "")
    if goal != expected_goal:
        fail(f"goal_text mismatch: got={goal} expected={expected_goal}")
print("ok: MOM multitrack observation assertions passed")
sys.exit(0)
PY
}

assert_feature_snapshot_file() {
  # assert_feature_snapshot_file <snapshot.json> — authoritative snapshot check
  python3 - "$1" <<'PY'
import json, sys
path = sys.argv[1]
try:
    d = json.load(open(path, encoding="utf-8"))
except Exception as exc:
    print(f"FAIL: authoritative snapshot unreadable: {path} ({exc})", file=sys.stderr)
    sys.exit(1)
if str(d.get("schema_version") or "") != "mixboard_feature_snapshot.v1":
    print(f"FAIL: authoritative snapshot schema mismatch: {d.get('schema_version')}", file=sys.stderr)
    sys.exit(1)
print("ok: authoritative mixboard feature snapshot present (schema v1)")
PY
}

import_audio_fixture() {
  local track_id="$1" file_path="$2" prefix="$3" status
  python3 - "$track_id" "$file_path" "$WORKDIR/bodies/import_args.json" <<'PY'
import json, sys
json.dump({"track_id": sys.argv[1], "file_path": sys.argv[2], "start_time": 0,
           "media_type": "audio", "mode": "non_destructive"},
          open(sys.argv[3], "w", encoding="utf-8"), ensure_ascii=False)
PY
  status="$(invoke_tool clip.import_media_to_track \
    "$WORKDIR/bodies/import_args.json" 1 "$prefix")"
  if [[ "$status" != "ok" ]]; then
    printf '%s' "{\"track_id\":\"$track_id\",\"file_path\":\"$file_path\",\"offset_time\":0}" > "$WORKDIR/bodies/import_args.json"
    status="$(invoke_tool clip.import_audio "$WORKDIR/bodies/import_args.json" 1 "$prefix")"
  fi
  [[ "$status" == "ok" ]] || fail_functional "audio fixture import failed: status=$status"
}

# ---------------------------------------------------------------- start stack
step "Start product-path stack (two-piece direct start; hub/Godot lifecycle pending PORT-VSPHUB-1)"
cp "$REPO_DEFAULT_PROJECT" "$KERNEL_WORKSPACE/default_project.xml"
(
  cd "$KERNEL_ROOT"
  exec env VIT_PROJECT_XML="$KERNEL_WORKSPACE/default_project.xml" \
           VIT_DAW_DEV_ROOT="$REPO_ROOT" \
           "$KERNEL_BIN"
) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
KERNEL_PID=$!
deadline=$((SECONDS + WAIT_SECONDS))
while (( SECONDS < deadline )); do
  kill -0 "$KERNEL_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2; fail_env "kernel exited during startup"; }
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
  sleep 1
done
[[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] || fail_env "kernel ZMQ REQ port $KERNEL_PORT_REQ not listening within ${WAIT_SECONDS}s"
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  owner="$(port_listener_pid "$port")"
  [[ "$owner" == "$KERNEL_PID" ]] || fail_env "port $port listener pid $owner != kernel pid $KERNEL_PID"
done
ok "kernel up (pid $KERNEL_PID, ports $KERNEL_PORT_REQ/$ZMQ_PUB_PORT/$ZMQ_LOG_PORT)"

(
  cd "$AGENT_CWD"
  exec env VIT_HISTORY_DRAFT_ROOT="$AGENT_DRAFTS" \
           VIT_DAW_DEV_ROOT="$REPO_ROOT" \
           VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
           VIT_MIXBOARD_ROOT="$MIXBOARD_ROOT" \
           "$AGENT_BIN" \
             -http "$AGENT_HTTP_ADDR" \
             -last-log-path "$AGENT_LOG" \
             -keep-last-log-lines 8000 \
             -vsp-hub-url ""
) > "$WORKDIR/logs/agent_stdout.log" 2>&1 &
AGENT_PID=$!
deadline=$((SECONDS + WAIT_SECONDS))
while (( SECONDS < deadline )); do
  kill -0 "$AGENT_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/agent_stdout.log" >&2; fail_env "agent exited during startup"; }
  curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && break
  sleep 1
done
curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null || fail_env "agent /health not ready within ${WAIT_SECONDS}s"
[[ "$(port_listener_pid "$AGENT_HTTP_PORT")" == "$AGENT_PID" ]] || fail_env "agent HTTP port owner is not the agent pid"
ok "agent up (pid $AGENT_PID, $AGENT_HTTP)"

# ---------------------------------------------------------------- health checks
step "Health checks"
code="$(http_json GET "$AGENT_HTTP/agent/state" "" "$WORKDIR/http/agent_state.json" 10)" \
  || fail_env "GET /agent/state failed"
[[ "$code" =~ ^2 ]] || fail_env "GET /agent/state returned HTTP $code"
[[ "$(json_field "$WORKDIR/http/agent_state.json" 'str(d.get("status",""))')" == "ok" ]] \
  || fail_functional "GET /agent/state did not return ok"
printf '{}' > "$WORKDIR/bodies/none.json"
[[ "$(invoke_tool project.state "$WORKDIR/bodies/none.json" 0 "$WORKDIR/http/project_state")" == "ok" ]] \
  || fail_functional "project.state failed"
http_json GET "$AGENT_HTTP/agent/runtime/status" "" "$WORKDIR/http/runtime_status.json" 10 >/dev/null \
  || fail_env "GET /agent/runtime/status failed"
ok "agent health/state/project.state passed"

# ---------------------------------------------------------------- fixture
step "Create fixture project through agent + live kernel"
PROJECT_NEW_STATUS="$(invoke_tool project.new "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_new")"
if [[ "$PROJECT_NEW_STATUS" == "ok" ]]; then
  ok "project.new reset completed"
  sleep 0.5
else
  warn "project.new unavailable: $(json_field "$WORKDIR/http/project_new.json" 'str(d.get("error",""))')"
fi
[[ "$(invoke_tool project.clear "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_clear")" == "ok" ]] \
  || fail_functional "project.clear failed"

printf '{"name":"Lead Vocal"}' > "$WORKDIR/bodies/track_args.json"
[[ "$(invoke_tool track.add_audio "$WORKDIR/bodies/track_args.json" 1 "$WORKDIR/http/track1_add")" == "ok" ]] \
  || fail_functional "track.add_audio Lead Vocal failed"
TRACK1_ID="$(json_field "$WORKDIR/http/track1_add.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
printf '{"name":"Track 2"}' > "$WORKDIR/bodies/track_args.json"
[[ "$(invoke_tool track.add_audio "$WORKDIR/bodies/track_args.json" 1 "$WORKDIR/http/track2_add")" == "ok" ]] \
  || fail_functional "track.add_audio Track 2 failed"
TRACK2_ID="$(json_field "$WORKDIR/http/track2_add.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
[[ -n "$TRACK1_ID" && -n "$TRACK2_ID" ]] || fail_functional "could not resolve fixture track IDs: $TRACK1_ID / $TRACK2_ID"

import_audio_fixture "$TRACK1_ID" "$TRACK1_PATH" "$WORKDIR/http/import1"
import_audio_fixture "$TRACK2_ID" "$TRACK2_PATH" "$WORKDIR/http/import2"
CLIP1_ID="$(json_field "$WORKDIR/http/import1.json" 'str((d.get("result") or {}).get("clip_id") or (d.get("result") or {}).get("id") or d.get("clip_id") or "")')"
CLIP2_ID="$(json_field "$WORKDIR/http/import2.json" 'str((d.get("result") or {}).get("clip_id") or (d.get("result") or {}).get("id") or d.get("clip_id") or "")')"
[[ -n "$CLIP1_ID" ]] || fail_functional "could not resolve imported clip ID for Track 1"
python3 - "$WORKDIR/fixture.json" "$TRACK1_ID" "$TRACK2_ID" "$CLIP1_ID" "$CLIP2_ID" "$TRACK1_PATH" "$TRACK2_PATH" \
  "$WORKDIR/http/import1.json" "$WORKDIR/http/import2.json" <<'PY'
import json, sys
out = sys.argv[1]
t1, t2, c1, c2, p1, p2, i1, i2 = sys.argv[2:10]
json.dump({"track1_id": t1, "track2_id": t2, "clip1_id": c1, "clip2_id": c2,
           "track1_name": "Lead Vocal", "track1_path": p1, "track2_path": p2,
           "import1": json.load(open(i1, encoding="utf-8")),
           "import2": json.load(open(i2, encoding="utf-8"))},
          open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
ok "fixture ready: Track 1=$TRACK1_ID (clip $CLIP1_ID) Track 2=$TRACK2_ID"
sleep 0.75

# ---------------------------------------------------------------- clip fade/gain
step "Clip fade/gain agent closed-loop smoke"
python3 - "$TRACK1_ID" "$CLIP1_ID" "$WORKDIR/bodies/ui_context.json" <<'PY'
import json, sys
json.dump({
    "selected_track_id": sys.argv[1],
    "selected_track_name": "Lead Vocal",
    "selected_clip_id": sys.argv[2],
    "selected_clip_ids": [sys.argv[2]],
    "selected_clip_track_id": sys.argv[1],
    "selected_clip_name": "Lead Vocal clip",
    "current_playhead_seconds": 0,
}, open(sys.argv[3], "w", encoding="utf-8"))
PY
set_ui_context "$WORKDIR/bodies/ui_context.json" "$WORKDIR/http/clip_fade_gain_ui_context.json"

UI_CONTEXT_JSON="$(cat "$WORKDIR/bodies/ui_context.json")"

CLIP_CONVERSATION_ID="product_path_clip_fade_gain_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$CLIP_CONVERSATION_ID" "set current clip gain to -3 dB" "$UI_CONTEXT_JSON" "$WORKDIR/http/chat_clip_gain_pending"
SET_STOP="$(json_field "$WORKDIR/http/chat_clip_gain_pending_settled.json" 'str(d.get("stop_reason",""))')"
SET_NEEDS="$(json_field "$WORKDIR/http/chat_clip_gain_pending_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
route_counts "$WORKDIR/http/chat_clip_gain_pending_settled.json" > "$WORKDIR/bodies/set_counts.json"
if [[ "$SET_NEEDS" != "true" ]]; then
  fail_functional "clip gain set setup did not create a confirmation. stop_reason=$SET_STOP tools=$(json_field "$WORKDIR/bodies/set_counts.json" '",".join(d["names"])')"
fi
python3 - "$WORKDIR/bodies/set_counts.json" <<'PY' || fail_functional "mix/track gain route during clip gain setup: $(cat "$WORKDIR/bodies/set_counts.json")"
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
forbidden = ("mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume")
sys.exit(1 if any(n in forbidden for n in d["names"]) else 0)
PY

agent_chat_ctx "$CLIP_CONVERSATION_ID" "$READ_CLIP_STATE_MESSAGE" "$UI_CONTEXT_JSON" "$WORKDIR/http/chat_clip_fade_gain_read"
READ_STOP="$(json_field "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" 'str(d.get("stop_reason",""))')"
READ_NEEDS="$(json_field "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
READ_GOAL="$(json_field "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" 'str(d.get("goal_status",""))')"
READ_PLAN="$(json_field "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" 'str(d.get("plan_id",""))')"
if [[ "$READ_NEEDS" == "true" || "$READ_GOAL" == "waiting_confirmation" || -n "$READ_PLAN" ]]; then
  fail_functional "clip fade/gain read was intercepted by stale confirmation (see chat_clip_fade_gain_read.json)"
fi
[[ "$READ_STOP" == "done" ]] \
  || fail_functional "clip fade/gain read stop_reason=$READ_STOP reply=$(json_field "$WORKDIR/http/chat_clip_fade_gain_read.json" 'str(d.get("reply",""))')"
route_counts "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" > "$WORKDIR/bodies/read_counts.json"
python3 - "$WORKDIR/bodies/read_counts.json" <<'PY' || fail_functional "clip read route assertions failed (see bodies/read_counts.json)"
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
names = d["names"]
print("route:", " -> ".join(names), file=sys.stderr)
if not any(n in ("clip.fade.read", "clip_fade_read") for n in names):
    sys.exit("FAIL: clip.fade.read not in route")
if not any(n in ("clip.gain.read", "clip_gain_read") for n in names):
    sys.exit("FAIL: clip.gain.read not in route")
for forbidden in ("clip.gain.set", "clip_gain_set", "mix.propose_tick", "mix_propose_tick",
                  "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume"):
    if forbidden in names:
        sys.exit(f"FAIL: stale write or mix route during clip read: {forbidden}")
sys.exit(0)
PY
# clip_id + reply needles on the fade/gain read result
python3 - "$WORKDIR/http/chat_clip_fade_gain_read_settled.json" "$CLIP1_ID" <<'PY' || fail_functional "clip fade/gain read result assertions failed"
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
clip1 = sys.argv[2]
rows = d.get("executed_kernel_reply") or []
if isinstance(rows, dict):
    rows = [rows]
def find(*aliases):
    for row in rows:
        if not isinstance(row, dict):
            continue
        for key in ("tool", "command_name", "command"):
            if str(row.get(key) or "") in aliases:
                return row.get("result") or {}
    return None
fade = find("clip.fade.read", "clip_fade_read")
gain = find("clip.gain.read", "clip_gain_read")
if str((fade or {}).get("clip_id") or "") != clip1:
    sys.exit(f"FAIL: clip.fade.read used wrong clip_id: {fade}")
if str((gain or {}).get("clip_id") or "") != clip1:
    sys.exit(f"FAIL: clip.gain.read used wrong clip_id: {gain}")
reply = str(d.get("reply") or "")
if "Fade" not in reply or "Clip gain" not in reply:
    sys.exit(f"FAIL: clip fade/gain read reply did not expose fade/gain state: {reply}")
sys.exit(0)
PY

agent_chat_ctx "$CLIP_CONVERSATION_ID" "$FADE_SET_MESSAGE" "$UI_CONTEXT_JSON" "$WORKDIR/http/chat_clip_fade_set_pending"
FADE_SET_NEEDS="$(json_field "$WORKDIR/http/chat_clip_fade_set_pending_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
[[ "$FADE_SET_NEEDS" == "true" ]] \
  || fail_functional "clip fade set did not request confirmation (see chat_clip_fade_set_pending.json)"
route_counts "$WORKDIR/http/chat_clip_fade_set_pending_settled.json" > "$WORKDIR/bodies/fade_set_counts.json"
python3 - "$WORKDIR/bodies/fade_set_counts.json" <<'PY' || fail_functional "clip fade set pending route assertions failed"
import json, sys
names = json.load(open(sys.argv[1], encoding="utf-8"))["names"]
if not any(n in ("clip.fade.set", "clip_fade_set") for n in names):
    sys.exit("FAIL: clip.fade.set pending not in route")
for forbidden in ("clip.fade.read", "clip_fade_read", "clip.gain.read", "clip_gain_read",
                  "mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick",
                  "track.volume", "track_volume", "set_volume"):
    if forbidden in names:
        sys.exit(f"FAIL: read or mix route during clip fade set: {forbidden}")
sys.exit(0)
PY

agent_chat_ctx "$CLIP_CONVERSATION_ID" "$CONFIRM_MESSAGE" "$UI_CONTEXT_JSON" "$WORKDIR/http/chat_clip_fade_set_confirm"
FADE_CONFIRM_NEEDS="$(json_field "$WORKDIR/http/chat_clip_fade_set_confirm_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
[[ "$FADE_CONFIRM_NEEDS" != "true" ]] \
  || fail_functional "clip fade set confirmation still needs confirmation (see chat_clip_fade_set_confirm.json)"
route_counts "$WORKDIR/http/chat_clip_fade_set_confirm_settled.json" > "$WORKDIR/bodies/fade_confirm_counts.json"
python3 - "$WORKDIR/bodies/fade_confirm_counts.json" <<'PY' || fail_functional "confirmed clip.fade.set route assertions failed"
import json, sys
names = json.load(open(sys.argv[1], encoding="utf-8"))["names"]
if not any(n in ("clip.fade.set", "clip_fade_set") for n in names):
    sys.exit("FAIL: confirmed clip.fade.set not in route")
for forbidden in ("mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick",
                  "track.volume", "track_volume", "set_volume"):
    if forbidden in names:
        sys.exit(f"FAIL: mix route during confirmed clip fade set: {forbidden}")
sys.exit(0)
PY

agent_chat_ctx "$CLIP_CONVERSATION_ID" "$READ_CLIP_STATE_MESSAGE" "$UI_CONTEXT_JSON" "$WORKDIR/http/chat_clip_fade_gain_read_after_set"
route_counts "$WORKDIR/http/chat_clip_fade_gain_read_after_set_settled.json" > "$WORKDIR/bodies/read_after_counts.json"
python3 - "$WORKDIR/http/chat_clip_fade_gain_read_after_set_settled.json" <<'PY' || fail_functional "fade round-trip value assertions failed"
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
rows = d.get("executed_kernel_reply") or []
if isinstance(rows, dict):
    rows = [rows]
def find(*aliases):
    for row in rows:
        if not isinstance(row, dict):
            continue
        for key in ("tool", "command_name", "command"):
            if str(row.get(key) or "") in aliases:
                return row.get("result") or {}
    return None
def near(value, expected, tol=0.001):
    try:
        return abs(float(str(value).strip()) - expected) <= tol
    except Exception:
        return False
fade = find("clip.fade.read", "clip_fade_read")
gain = find("clip.gain.read", "clip_gain_read")
names_found = fade is not None and gain is not None
if not names_found:
    sys.exit("FAIL: clip.fade.read/gain.read after fade set not in route")
if not near((fade or {}).get("fade_in_seconds"), 0.15):
    sys.exit(f"FAIL: fade_in_seconds after agent fade set: {fade.get('fade_in_seconds')}")
if not near((fade or {}).get("fade_out_seconds"), 0.25):
    sys.exit(f"FAIL: fade_out_seconds after agent fade set: {fade.get('fade_out_seconds')}")
sys.exit(0)
PY
ok "clip fade/gain agent read cleared stale confirmation and fade set round-tripped through typed clip tools"

# ---------------------------------------------------------------- observe turn
step "Ask product-path mix question through agent HTTP"
CONVERSATION_ID="product_path_mix_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$CONVERSATION_ID" "$OBSERVE_MESSAGE" "-" "$WORKDIR/http/chat_observe"
OBSERVE_STOP="$(json_field "$WORKDIR/http/chat_observe_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason observe "$OBSERVE_STOP"
[[ "$OBSERVE_STOP" == "done" ]] || fail_functional "observe turn stop_reason=$OBSERVE_STOP"
events_get "$CONVERSATION_ID" 0 20 "$WORKDIR/http/events_after_observe.json"
OBSERVE_EVENT_SEQ="$(json_field "$WORKDIR/http/events_after_observe.json" 'int(d.get("next_seq", 0))')"
OBSERVE_PENDING_COUNT="$(json_field "$WORKDIR/http/events_after_observe.json" 'sum(1 for e in (d.get("events") or []) if str(e.get("type","")) == "mix_tick.pending")')"
OBSERVE_NEEDS="$(json_field "$WORKDIR/http/chat_observe_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
[[ "$OBSERVE_PENDING_COUNT" == "0" ]] || fail_functional "read-only product observe emitted mix_tick.pending"
[[ "$OBSERVE_NEEDS" != "true" ]] || fail_functional "read-only product observe requested confirmation"
ok "product observe stayed read-only with no pending event or confirmation"
assert_observation_shape "$WORKDIR/http/chat_observe_settled.json" bridge \
  || fail_functional "product observe acoustic bridge readiness failed: $(head -c 400 "$WORKDIR/http/chat_observe.json")"
[[ -f "$FEATURE_SNAPSHOT_PATH" ]] || fail_functional "authoritative mixboard feature snapshot missing: $FEATURE_SNAPSHOT_PATH"
assert_feature_snapshot_file "$FEATURE_SNAPSHOT_PATH"
cp "$FEATURE_SNAPSHOT_PATH" "$WORKDIR/http/authoritative_mixboard_feature_snapshot.json"
python3 - "$FEATURE_SNAPSHOT_PATH" > "$WORKDIR/http/snapshot_after_observe.json" <<'PY'
import datetime, json, os, sys
path = sys.argv[1]
json.dump({"authoritative_path": path,
           "authoritative_last_write_utc": datetime.datetime.utcfromtimestamp(os.stat(path).st_mtime).isoformat() + "Z",
           "godot_project_split_check": "n/a_without_godot_lifecycle (mac two-piece adaptation)"},
          sys.stdout, ensure_ascii=False)
PY
ok "authoritative feature snapshot asserted at the mac mixboard root"

# ---------------------------------------------------------------- multitrack MOM
step "Chinese multitrack MOM observation scope"
MULTITRACK_CONVERSATION_ID="product_path_multitrack_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$MULTITRACK_CONVERSATION_ID" "$MULTITRACK_MESSAGE" "-" "$WORKDIR/http/chat_multitrack_observe"
MULTITRACK_STOP="$(json_field "$WORKDIR/http/chat_multitrack_observe_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason multitrack_observe "$MULTITRACK_STOP"
[[ "$MULTITRACK_STOP" == "done" ]] || fail_functional "multitrack observe turn stop_reason=$MULTITRACK_STOP"
assert_observation_shape "$WORKDIR/http/chat_multitrack_observe_settled.json" mom "$MULTITRACK_MESSAGE" \
  || fail_functional "product Chinese multitrack observe MOM assertions failed"
events_get "$MULTITRACK_CONVERSATION_ID" 0 20 "$WORKDIR/http/events_after_multitrack_observe.json"
MULTITRACK_PENDING_COUNT="$(json_field "$WORKDIR/http/events_after_multitrack_observe.json" 'sum(1 for e in (d.get("events") or []) if str(e.get("type","")) == "mix_tick.pending")')"
MULTITRACK_NEEDS="$(json_field "$WORKDIR/http/chat_multitrack_observe_settled.json" 'str(d.get("needs_confirmation","")).lower()')"
[[ "$MULTITRACK_PENDING_COUNT" == "0" ]] || fail_functional "read-only multitrack observe emitted mix_tick.pending"
[[ "$MULTITRACK_NEEDS" != "true" ]] || fail_functional "read-only multitrack observe requested confirmation"
ok "Chinese multitrack observe used project MOM scope with no pending event or confirmation"

# ------------------------------------------------- confirm-tick (ps1 parity)
# ps1: this block runs only when a pending candidate exists; the default
# path's read-only observe assertions above guarantee PENDING_CANDIDATE stays
# empty on BOTH platforms — kept for parity, not reachable here.
PENDING_CANDIDATE=""
if [[ -n "$PENDING_CANDIDATE" ]]; then
  fail_functional "unreachable: pending candidate asserted absent above (ps1 parity block)"
fi

# ---------------------------------------------------------------- no-pending guard
step "No-pending confirmation guard"
GUARD_CONVERSATION_ID="product_path_no_pending_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$GUARD_CONVERSATION_ID" "$CONFIRM_MESSAGE" "-" "$WORKDIR/http/chat_no_pending"
GUARD_STOP="$(json_field "$WORKDIR/http/chat_no_pending_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason no_pending "$GUARD_STOP"
[[ "$GUARD_STOP" == "no_pending_mix_tick_candidate" ]] \
  || fail_functional "no-pending guard stop_reason=$GUARD_STOP"

# ---------------------------------------------------------------- vocal focus
step "Vocal focus relationship observation"
FOCUS_CONVERSATION_ID="product_path_vocal_focus_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$FOCUS_CONVERSATION_ID" "$VOCAL_FOCUS_MESSAGE" "-" "$WORKDIR/http/chat_vocal_focus"
FOCUS_STOP="$(json_field "$WORKDIR/http/chat_vocal_focus_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason vocal_focus "$FOCUS_STOP"
case "$FOCUS_STOP" in
  done|needs_clarification|needs_confirmation) ;;
  *) fail_functional "vocal focus turn stop_reason=$FOCUS_STOP" ;;
esac
route_counts "$WORKDIR/http/chat_vocal_focus_settled.json" > "$WORKDIR/bodies/focus_counts.json"
python3 - "$WORKDIR/http/chat_vocal_focus_settled.json" "$WORKDIR/bodies/focus_counts.json" "$FOCUS_STOP" <<'PY' || fail_functional "vocal focus route assertions failed (route: $(cat "$WORKDIR/bodies/focus_counts.json"))"
import json, sys
resp = json.load(open(sys.argv[1], encoding="utf-8"))
counts = json.load(open(sys.argv[2], encoding="utf-8"))
stop_reason = sys.argv[3]
if counts["observe"] < 1:
    sys.exit("FAIL: expected vocal focus route to include mix.observe/ccb.observation_catalog")
if stop_reason == "needs_confirmation":
    if resp.get("needs_confirmation") is not True:
        sys.exit("FAIL: needs_confirmation stop reason without needs_confirmation=true")
    pending_events = sum(1 for e in (resp.get("typed_events") or [])
                         if isinstance(e, dict) and str(e.get("event_type")) == "PendingCandidate")
    if pending_events < 1:
        sys.exit("FAIL: needs_confirmation did not include a PendingCandidate typed event")
    if counts["derive"] < 1:
        sys.exit("FAIL: expected pending route to include mix.derive")
if stop_reason == "done" and counts["derive"] < 1:
    reply = str(resp.get("reply") or "")
    safe_noop = any(k in reply for k in ("can't", "cannot", "not reliably", "No mix action",
                                         "no mix action", "not safe", "not identified", "partial"))
    if not safe_noop:
        sys.exit("FAIL: completed route without mix.derive and without an explicit no-op reply")
if counts["apply"] != 0 or counts["daw_invoke"] != 0 or counts["track_volume"] != 0:
    sys.exit(f"FAIL: vocal focus route mutated the project unexpectedly: {counts}")
sys.exit(0)
PY
ok "vocal focus relationship observation passed"

# ------------------------------------------------------- vocal clarification
step "Vocal clarification loop"
PROJECT_NEW2_STATUS="$(invoke_tool project.new "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_new2")"
[[ "$PROJECT_NEW2_STATUS" == "ok" ]] || warn "project.new unavailable (loop reset)"
[[ "$(invoke_tool project.clear "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_clear2")" == "ok" ]] \
  || fail_functional "project.clear (vocal clarification reset) failed"
printf '{"name":"Track 1"}' > "$WORKDIR/bodies/track_args.json"
[[ "$(invoke_tool track.add_audio "$WORKDIR/bodies/track_args.json" 1 "$WORKDIR/http/generic_track1")" == "ok" ]] \
  || fail_functional "track.add_audio generic Track 1 failed"
GENERIC_TRACK1_ID="$(json_field "$WORKDIR/http/generic_track1.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
printf '{"name":"Track 2"}' > "$WORKDIR/bodies/track_args.json"
[[ "$(invoke_tool track.add_audio "$WORKDIR/bodies/track_args.json" 1 "$WORKDIR/http/generic_track2")" == "ok" ]] \
  || fail_functional "track.add_audio generic Track 2 failed"
GENERIC_TRACK2_ID="$(json_field "$WORKDIR/http/generic_track2.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
[[ -n "$GENERIC_TRACK1_ID" && -n "$GENERIC_TRACK2_ID" ]] \
  || fail_functional "could not resolve generic vocal clarification track IDs: $GENERIC_TRACK1_ID / $GENERIC_TRACK2_ID"
import_audio_fixture "$GENERIC_TRACK1_ID" "$TRACK1_PATH" "$WORKDIR/http/generic_import1"
import_audio_fixture "$GENERIC_TRACK2_ID" "$TRACK2_PATH" "$WORKDIR/http/generic_import2"
sleep 0.75

CLARIFY_CONVERSATION_ID="product_path_vocal_clarify_$(date '+%Y%m%d_%H%M%S')"
agent_chat_ctx "$CLARIFY_CONVERSATION_ID" "$VOCAL_FORWARD_MESSAGE" "-" "$WORKDIR/http/chat_vocal_clarify_ask"
CLARIFY_ASK_STOP="$(json_field "$WORKDIR/http/chat_vocal_clarify_ask_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason vocal_clarification_ask "$CLARIFY_ASK_STOP"
[[ "$CLARIFY_ASK_STOP" == "needs_clarification" ]] \
  || fail_functional "vocal clarification ask stop_reason=$CLARIFY_ASK_STOP reply=$(json_field "$WORKDIR/http/chat_vocal_clarify_ask.json" 'str(d.get("reply",""))')"
CLARIFY_ASK_REPLY="$(json_field "$WORKDIR/http/chat_vocal_clarify_ask_settled.json" 'str(d.get("reply",""))')"
CLARIFY_ASK_REPLY_LC="$(python3 -c 'import sys; print(sys.argv[1].lower())' "$CLARIFY_ASK_REPLY")"
if [[ "$CLARIFY_ASK_REPLY" != *"哪条"* && "$CLARIFY_ASK_REPLY" != *"哪一条"* && "$CLARIFY_ASK_REPLY" != *"哪一轨"* && "$CLARIFY_ASK_REPLY_LC" != *"which track"* ]]; then
  fail_functional "vocal clarification ask did not ask which track is vocal: $CLARIFY_ASK_REPLY"
fi
events_get "$CLARIFY_CONVERSATION_ID" 0 20 "$WORKDIR/http/events_vocal_clarify_after_ask.json"
CLARIFY_ASK_PENDING="$(json_field "$WORKDIR/http/events_vocal_clarify_after_ask.json" 'sum(1 for e in (d.get("events") or []) if str(e.get("type","")) == "mix_tick.pending")')"
[[ "$CLARIFY_ASK_PENDING" == "0" ]] \
  || fail_functional "vocal clarification ask stored pending before the vocal track was identified"

agent_chat_ctx "$CLARIFY_CONVERSATION_ID" "$VOCAL_ANSWER_MESSAGE" "-" "$WORKDIR/http/chat_vocal_clarify_answer"
CLARIFY_ANSWER_STOP="$(json_field "$WORKDIR/http/chat_vocal_clarify_answer_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason vocal_clarification_answer "$CLARIFY_ANSWER_STOP"
case "$CLARIFY_ANSWER_STOP" in
  done|needs_confirmation) ;;
  *) fail_functional "vocal clarification answer stop_reason=$CLARIFY_ANSWER_STOP reply=$(json_field "$WORKDIR/http/chat_vocal_clarify_answer.json" 'str(d.get("reply",""))')" ;;
esac
if [[ "$CLARIFY_ANSWER_STOP" == "needs_confirmation" ]]; then
  [[ "$(json_field "$WORKDIR/http/chat_vocal_clarify_answer_settled.json" 'str(d.get("needs_confirmation","")).lower()')" == "true" ]] \
    || fail_functional "vocal clarification answer returned needs_confirmation stop reason without needs_confirmation=true"
fi
wait_log_pattern "[mix.tick.pending] stored conversation=$CLARIFY_CONVERSATION_ID" 15 >/dev/null
events_get "$CLARIFY_CONVERSATION_ID" 0 40 "$WORKDIR/http/events_vocal_clarify_after_answer.json"
CLARIFY_PENDING_COUNT="$(json_field "$WORKDIR/http/events_vocal_clarify_after_answer.json" 'sum(1 for e in (d.get("events") or []) if str(e.get("type","")) == "mix_tick.pending")')"
[[ "$CLARIFY_PENDING_COUNT" != "0" ]] \
  || fail_functional "vocal clarification answer did not produce a pending mix tick"

agent_chat_ctx "$CLARIFY_CONVERSATION_ID" "$CONFIRM_MESSAGE" "-" "$WORKDIR/http/chat_vocal_clarify_confirm"
CLARIFY_CONFIRM_STOP="$(json_field "$WORKDIR/http/chat_vocal_clarify_confirm_settled.json" 'str(d.get("stop_reason",""))')"
record_stop_reason vocal_clarification_confirm "$CLARIFY_CONFIRM_STOP"
[[ "$CLARIFY_CONFIRM_STOP" == "mix_tick_applied_reobserved" ]] \
  || fail_functional "vocal clarification confirm stop_reason=$CLARIFY_CONFIRM_STOP"
route_counts "$WORKDIR/http/chat_vocal_clarify_confirm_settled.json" > "$WORKDIR/bodies/clarify_confirm_counts.json"
python3 - "$WORKDIR/bodies/clarify_confirm_counts.json" <<'PY' || fail_functional "vocal clarification confirm route assertions failed: $(cat "$WORKDIR/bodies/clarify_confirm_counts.json")"
import json, sys
c = json.load(open(sys.argv[1], encoding="utf-8"))
if c["propose"] < 1: sys.exit("FAIL: mix.propose_tick missing")
if c["apply"] < 1: sys.exit("FAIL: mix.apply_tick missing")
if c["observe"] < 1: sys.exit("FAIL: reobserve missing")
if c["daw_invoke"] != 0: sys.exit("FAIL: daw.invoke present")
if c["track_volume"] != 0: sys.exit("FAIL: track.volume present")
sys.exit(0)
PY
CLARIFY_CONFIRM_REPLY="$(json_field "$WORKDIR/http/chat_vocal_clarify_confirm_settled.json" 'str(d.get("reply",""))')"
[[ "$CLARIFY_CONFIRM_REPLY" == *"AB Result"* ]] \
  || fail_functional "vocal clarification confirmation reply did not include AB Result: $CLARIFY_CONFIRM_REPLY"
ok "vocal clarification loop (ask -> answer -> confirm with AB Result) passed"

OUTCOME="all_green"
ok "product-path lifecycle + mix smoke passed (mac two-piece adaptation)"
exit 0
