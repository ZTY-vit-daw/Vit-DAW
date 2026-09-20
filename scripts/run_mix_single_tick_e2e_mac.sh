#!/usr/bin/env bash
# run_mix_single_tick_e2e mac equivalent — PORT-SMOKE-MAC-1 (script 4/7).
#
# Mac port of scripts/run_mix_single_tick_e2e.ps1: the mix single-tick
# E2E — build a two-track fixture project through the agent tool face, then
# three real LLM chat turns (observe -> vocal clarification guard -> explicit
# confirmation) with stop_reason / route / agent-log assertions.
#
#   | ps1 (run_mix_single_tick_e2e.ps1)         | mac (this script)              |
#   |--------------------------------------------|--------------------------------|
#   | param($RepoRoot, $AgentHttp, $ZmqReqPort,  | flags below (same names)       |
#   |   $KernelExe, $Track1Path, $Track2Path,    |                                |
#   |   -SkipBuild, -Reuse*, $WaitSeconds=30,    |                                |
#   |   $ChatTimeoutSec=240)                     |                                |
#   | Stack via dev_agent_smoke.ps1 (restart     | stack start in-run (A5/JOURNEY |
#   |   agent + optional kernel start; agent cwd | pattern): kernel under a fake  |
#   |   = repo agent dir, log in repo Workspace) | VitApp root + VIT_PROJECT_XML  |
#   |                                            | copy; agent with draft-root /  |
#   |                                            | orchestration-store isolation  |
#   |                                            | and -vsp-hub-url "" — repo     |
#   |                                            | tree never written (§10)       |
#   | Resolve-KernelExe PC Release candidates    | cmake+make Debug build in the  |
#   |                                            | run workdir or --kernel-bin    |
#   | go build VitAgent (in repo bin)            | go build into the run workdir  |
#   | Get-NetTCPListener port checks             | lsof port_listener_pid         |
#   | Stop-Process existing kernel (ps1 stops a  | ports must be FREE (env fail)  |
#   |   busy port owner!)                        | — AGENTS §9 single-owner rule  |
#   | Join-UnicodeChars code-point strings       | plain UTF-8 literals           |
#   | Invoke-WebRequest + UTF-8 body bytes       | curl + python json bodies      |
#   | AgentLog = repo Workspace\Logs\            | -last-log-path into the run    |
#   |   agent_last.log                           | workdir (same [mix.tick.*]     |
#   |                                            | patterns)                      |
#   | Track1/Track2 = repo root                  | same two repo-root fixture     |
#   |   test_100hz_10s.wav / test_target_3s.wav  | wavs (tracked in git)          |
#   | Import clip.import_media_to_track          | identical + fallback           |
#   |   (fallback clip.import_audio)             | clip.import_audio              |
#   | (not in ps1)                               | busy-retry 5s x 12 on "Engine  |
#   |                                            | is busy rendering" replies     |
#   |                                            | (mac machine fact, JOURNEY)    |
#   | (not in ps1)                               | chat_settle anchor: with this  |
#   |                                            | mac LLM a chat turn may end    |
#   |                                            | limit_reached/waiting_continue |
#   |                                            | (durable continuation armed);  |
#   |                                            | the driver waits for the goal  |
#   |                                            | to settle (runtime status poll)|
#   |                                            | and reads the settled reply +  |
#   |                                            | effective stop_reason from the |
#   |                                            | conversation events surface —  |
#   |                                            | JOURNEY-1-MAC waiting_continue |
#   |                                            | precedent; assertions and      |
#   |                                            | needles unchanged              |
#   | observe turn / vocal guard / confirm turn  | identical messages, stop_     |
#   | assertions (stop_reason, reply needles,    | reason checks, reply needles,  |
#   |   route aliases, [mix.tick.pending] log    | route aliases, log patterns    |
#   |   patterns, L3-incomplete read-only branch)| , L3 branch                    |
#   | No summary json (console only)             | summary.json + full artifacts  |
#   |                                            | under the run workdir          |
#
# LLM precondition (card): the chat turns are real LLM turns. The script
# preflights the engine config (~/.vit/config.json or VIT_AGENT_LLM_*/OPENAI_*
# env; agent/internal/config/config.go) and refuses to run (env failure,
# exit 2) when incomplete — no stub replies. The API key never reaches logs
# or artifacts (boolean has_api_key + model + base host only).
#
# §8 discipline (pre-declared): ≤3 valid runs; success = single run exit 0
# with all turn assertions green; failure classes recorded separately;
# same-breakpoint two-failure stop-loss before any rerun.

set -uo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
TRACK1_PATH=""
TRACK2_PATH=""
WAIT_SECONDS=30
CHAT_TIMEOUT_SEC=240
CHAT_SETTLE_SECONDS=300
KEEP_PROCESSES=0
WORKDIR_ARG=""
TRACKTION_DIR=""
ARTIFACT_ROOT_ARG=""

KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

# Journey literals — the ps1 carries these as code-point arrays only to
# survive the PS 5.1 ANSI code page; macOS rides UTF-8 directly.
OBSERVE_MESSAGE="帮我看整体混音，只建议一个小幅音量调整，先等我确认，不要用插件"
VOCAL_MESSAGE="让主唱更靠前"
CONFIRM_MESSAGE="可以执行"
# Confirmation-request wording needle group (PORT-PS1-SYNC-2): the flash
# engine has been observed asking with plain "确认" (先等你确认/请确认/待确认)
# without ever writing 执行/继续, so the semantic group is {执行, 继续, 确认}.
EXECUTE_NEEDLE="执行"
CONTINUE_NEEDLE="继续"
CONFIRM_NEEDLE="确认"

usage() {
  cat <<'EOF'
run_mix_single_tick_e2e_mac.sh — mac port of run_mix_single_tick_e2e.ps1
(PORT-SMOKE-MAC-1). Two-track fixture + three LLM chat turns (observe /
vocal clarification guard / confirmation) over the real kernel+agent stack.

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary
  --skip-agent-build      With --agent-bin: do not build the agent
  --agent-bin PATH        Agent binary to start (only with --skip-agent-build)
  --track1-path PATH      Fixture wav for Track 1 (default <repo>/test_100hz_10s.wav)
  --track2-path PATH      Fixture wav for Track 2 (default <repo>/test_target_3s.wav)
  --wait-seconds N        Startup wait (default 30)
  --chat-timeout-sec N    /agent/chat timeout (default 240)
  --chat-settle-seconds N Budget for waiting a sliced-out turn's durable
                          continuation to settle (default 300)
  --keep-processes        Do not stop the stack this run started
  --workdir PATH          Reuse PATH as the run artifact dir
  --tracktion-dir PATH    tracktion_engine source dir (kernel build only)
  --artifact-root DIR     Artifact root (default ~/Documents/vit-smoke-mac1-artifacts)
  -h, --help              Show this help

Exit codes: 0 = E2E passed; 1 = a turn assertion failed; 2 = environment.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --track1-path) TRACK1_PATH="$2"; shift 2 ;;
    --track2-path) TRACK2_PATH="$2"; shift 2 ;;
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

RUN_ID="mix_single_tick_e2e_mac_$(date '+%Y%m%d-%H%M%S')"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac1-artifacts}"
WORKDIR="${WORKDIR_ARG:-$ARTIFACT_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR"/{http,bodies,logs} || fail_env "cannot create artifact dir: $WORKDIR"
AGENT_LOG="$WORKDIR/agent_last.log"

KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"
KERNEL_WORKSPACE="$WORKDIR/kernel_workspace"
mkdir -p "$KERNEL_WORKSPACE"
AGENT_DRAFTS="$WORKDIR/agent_drafts"
AGENT_STATE_DIR="$WORKDIR/agent_state"
AGENT_CWD="$WORKDIR/agent_cwd"
mkdir -p "$AGENT_DRAFTS" "$AGENT_STATE_DIR" "$AGENT_CWD"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"

KERNEL_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
KERNEL_BIN=""
AGENT_BIN=""
OUTCOME="env_failure"

stop_process() {
  local pid="$1" start elapsed code signal="SIGTERM"
  kill -TERM "$pid" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS - start < 10 )) || { kill -KILL "$pid" 2>/dev/null || true; signal="SIGTERM+SIGKILL"; break; }
    sleep 1
  done
  code=0
  wait "$pid" 2>/dev/null || code=$?
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
  python3 - "$WORKDIR/summary.json" "$RUN_ID" "$WORKDIR" "$OUTCOME" \
    "$KERNEL_STOP_RECORD" "$AGENT_STOP_RECORD" "$KERNEL_BIN" "$AGENT_BIN" "$AGENT_LOG" <<'PY' 2>/dev/null || true
import glob, json, os, sys
out, run_id, workdir, outcome, kernel_stop, agent_stop, kernel_bin, agent_bin, agent_log = sys.argv[1:10]
def sha(path):
    try:
        import hashlib
        return hashlib.sha256(open(path, "rb").read()).hexdigest()
    except Exception:
        return ""
summary = {
    "schema_version": "mix_single_tick_e2e.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_mix_single_tick_e2e.ps1",
    "run_id": run_id,
    "overall_status": "PASS" if outcome == "all_green" else outcome,
    "outcome": outcome,
    "kernel_exe": kernel_bin, "kernel_sha256": sha(kernel_bin) if kernel_bin else "",
    "agent_binary": agent_bin, "agent_sha256": sha(agent_bin) if agent_bin else "",
    "kernel_stop": {"record": kernel_stop}, "agent_stop": {"record": agent_stop},
    "conversations": sorted(os.path.basename(p) for p in glob.glob(os.path.join(workdir, "http", "chat_*.json"))),
    "agent_log": agent_log,
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
import json, os, sys
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
    "config_file_exists": os.path.isfile(path),
    "has_api_key": bool(key),
    "model": model,
    "base_url_host": host,
    "complete": bool(base and key and model),
}))
PY
)" || fail_env "LLM config preflight crashed"
log "llm_config=$LLM_CFG_SUMMARY"
python3 -c 'import json,sys; sys.exit(0 if json.loads(sys.argv[1])["complete"] else 1)' "$LLM_CFG_SUMMARY" \
  || fail_env "LLM engine config incomplete (card stop condition: 环境缺失 → blocked，不得以桩应答替代): $LLM_CFG_SUMMARY"

step "Preflight: run root + repo fingerprints + ports"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_mix_single_tick_e2e_mac.sh (mac port of run_mix_single_tick_e2e.ps1)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
  echo "llm_config=$LLM_CFG_SUMMARY"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$REPO_DEFAULT_PROJECT" ]] || fail_env "repo default project missing: $REPO_DEFAULT_PROJECT"
[[ -z "$TRACK1_PATH" ]] && TRACK1_PATH="$REPO_ROOT/test_100hz_10s.wav"
[[ -z "$TRACK2_PATH" ]] && TRACK2_PATH="$REPO_ROOT/test_target_3s.wav"
[[ -f "$TRACK1_PATH" ]] || fail_env "track 1 fixture missing: $TRACK1_PATH"
[[ -f "$TRACK2_PATH" ]] || fail_env "track 2 fixture missing: $TRACK2_PATH"

for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "$AGENT_HTTP_PORT"; do
  pid_free="$(port_listener_pid "$port")"
  [[ -z "$pid_free" ]] || fail_env "port $port already has a listener (pid $pid_free); AGENTS §9 single-owner rule"
done

# ---------------------------------------------------------------- binaries
KERNEL_BIN=""
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
  # http_json <method> <url> <body-file-or-""> <out-file> [timeout]
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
  # Retries (5s x 12) while the reply is a kernel "Engine is busy rendering"
  # rejection (mac machine fact: offline renders hold the engine; JOURNEY-1-MAC
  # precedent — assertions unchanged, only the wait window added).
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

invoke_tool_body() {
  # invoke_tool_body <tool> <args-json-file> <confirmed> <out-body-file>
  python3 - "$1" "$2" "$3" "$4" <<'PY'
import json, sys
tool, args_path, confirmed, out = sys.argv[1:5]
args = json.load(open(args_path, encoding="utf-8")) if args_path != "-" else {}
json.dump({"tool": tool, "args": args, "confirmed": confirmed in ("true", "True", "1"),
           "source": "mix_single_tick_e2e"}, open(out, "w", encoding="utf-8"), ensure_ascii=False)
PY
}

invoke_tool() {
  # invoke_tool <tool> <args-json> <confirmed:true|false> <out-prefix> [timeout]
  local tool="$1" args_json="$2" confirmed="$3" prefix="$4" timeout="${5:-120}" code
  printf '%s' "$args_json" > "$WORKDIR/bodies/tool_args.json"
  invoke_tool_body "$tool" "$WORKDIR/bodies/tool_args.json" "$confirmed" "$WORKDIR/bodies/tool_invoke.json"
  code="$(invoke_json_busy_retry POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/tool_invoke.json" "${prefix}.json" "$timeout")" \
    || fail_env "invoke $tool transport failed"
  [[ "$code" =~ ^2 ]] || fail_env "invoke $tool returned HTTP $code (see ${prefix}.json)"
  json_field "${prefix}.json" 'str(d.get("status",""))'
}

chat_body() {
  # chat_body <conversation-id> <message> <out-file>
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys
json.dump({"conversation_id": sys.argv[1], "message": sys.argv[2],
           "context": {"agent_mode": "chat"}},
          open(sys.argv[3], "w", encoding="utf-8"), ensure_ascii=False)
PY
}

agent_chat() {
  # agent_chat <conversation-id> <message> <out-prefix>
  local conv="$1" message="$2" prefix="$3" code
  chat_body "$conv" "$message" "$WORKDIR/bodies/chat.json"
  code="$(http_json POST "$AGENT_HTTP/agent/chat" "$WORKDIR/bodies/chat.json" "${prefix}.json" "$CHAT_TIMEOUT_SEC")" \
    || fail_env "chat transport failed (conversation $conv)"
  [[ "$code" =~ ^2 ]] || fail_env "chat returned HTTP $code (conversation $conv)"
}

PLACEHOLDER_REPLY_NEEDLE="我还在继续处理这个任务，完成后再向你汇报。"

agent_chat_settled() {
  # agent_chat_settled <conversation-id> <message> <out-prefix>
  # mac anchor (JOURNEY-1-MAC waiting_continue precedent): a chat POST may end
  # limit_reached with the continuation placeholder while the agent's durable
  # continuation finishes the turn asynchronously. Wait for the goal to settle
  # and read the settled reply / effective stop_reason from the events surface;
  # assertions run against <prefix>_settled.json (raw response preserved).
  local conv="$1" message="$2" prefix="$3"
  agent_chat "$conv" "$message" "$prefix"
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
    local code
    code="$(http_json GET "$AGENT_HTTP/agent/runtime/status" "" "$WORKDIR/bodies/settle_poll.json" 30)" || { sleep 1; continue; }
    [[ "$code" =~ ^2 ]] || { sleep 1; continue; }
    polled="$(json_field "$WORKDIR/bodies/settle_poll.json" 'str((d.get("goal") or {}).get("status",""))')"
    log "settle poll: goal=$polled"
    case "$polled" in
      completed|failed|stopped|cancelled|waiting_confirmation|waiting_clarification) break ;;
    esac
  done
  printf '%s' "$polled" > "${prefix}_settled_goal.txt"
  local events_code
  events_code="$(http_json GET "$AGENT_HTTP/agent/events?conversation_id=$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$conv")&since=0&limit=500" "" "${prefix}_events.json" 60)" || true
  python3 - "$prefix" "$polled" "$PLACEHOLDER_REPLY_NEEDLE" <<'SETTLE_PY'
import json, sys
prefix, settled_goal, placeholder = sys.argv[1:4]
raw = json.load(open(f"{prefix}.json", encoding="utf-8"))
events = {}
try:
    events = json.load(open(f"{prefix}_events.json", encoding="utf-8"))
except Exception:
    pass
delivered, turn_completed_texts = [], []
for ev in (events.get("events") or []):
    parts = [str(ev.get(k, "")) for k in ("title", "body", "summary") if str(ev.get(k, "")).strip()]
    if parts:
        delivered.append(" | ".join(parts))
    if str(ev.get("type", "")) == "turn.completed":
        text = " ".join(str(ev.get(k, "")) for k in ("title", "body") if str(ev.get(k, "")).strip())
        if text:
            turn_completed_texts.append(text)
# the model's substantive reply rides the turn.completed event (title=terminal
# state label, body=the reply text); lifecycle/trajectory lines are not replies
reply_candidates = [t for t in delivered if placeholder not in t]                    + [t for t in turn_completed_texts if placeholder not in t]
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
json.dump(out, open(f"{prefix}_settled.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
SETTLE_PY
}

wait_log_pattern() {
  # wait_log_pattern <pattern> <timeout-sec> — prints the first matching line
  # (split declaration: bash 3.2 + set -u expands a later assignment in one
  # `local` line before earlier locals in that line exist)
  local pattern="$1" timeout="$2" line
  local deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    if [[ -f "$AGENT_LOG" ]]; then
      line="$(grep -F "$pattern" "$AGENT_LOG" 2>/dev/null | head -1)"
      [[ -n "$line" ]] && { printf '%s' "$line"; return 0; }
    fi
    sleep 0.25
  done
  fail_functional "Timed out waiting for log pattern: $pattern"
}

tool_names_of() {
  # tool_names_of <response.json> — prints the executed route tool names
  python3 - "$1" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
rows = d.get("executed_kernel_reply") or []
if isinstance(rows, dict):
    rows = [rows]
out = []
for row in rows:
    if isinstance(row, str):
        out.append(row)
        continue
    for key in ("tool", "command_name", "command"):
        name = str(row.get(key) or "")
        if name:
            out.append(name)
print("\n".join(out))
PY
}

assert_status_ok() {
  # assert_status_ok <response.json> <label>
  local status
  status="$(json_field "$1" 'str(d.get("status",""))')"
  [[ "$status" == "ok" ]] || fail_functional "$2 failed: status=$status error=$(json_field "$1" 'str(d.get("error",""))')"
}
# (kept for parity with the ps1 Assert-StatusOk used on probe replies)

# ---------------------------------------------------------------- start stack
step "Prepare agent and kernel"
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
ok "kernel command port listening: $KERNEL_PORT_REQ pid=$KERNEL_PID"

(
  cd "$AGENT_CWD"
  exec env VIT_HISTORY_DRAFT_ROOT="$AGENT_DRAFTS" \
           VIT_DAW_DEV_ROOT="$REPO_ROOT" \
           VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
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
ok "agent ready at $AGENT_HTTP (pid $AGENT_PID)"

# ---------------------------------------------------------------- fixture
step "Create two-track fixture"

# Reset-FixtureProject (ps1): project.new, project.clear, delete every audio
# track but the lowest-indexed one, return the remaining audio tracks sorted.
PROJECT_NEW_STATUS="$(invoke_tool project.new '{}' true "$WORKDIR/http/project_new")"
if [[ "$PROJECT_NEW_STATUS" == "ok" ]]; then
  ok "project.new reset completed"
  sleep 0.5
else
  warn "project.new unavailable; falling back to project.clear/delete"
fi
[[ "$(invoke_tool project.clear '{}' true "$WORKDIR/http/project_clear")" == "ok" ]] \
  || fail_functional "project.clear before fixture reset failed"

state_status="$(invoke_tool project.state '{}' false "$WORKDIR/http/state_before_reset")"
[[ "$state_status" == "ok" ]] || fail_functional "project.state before fixture reset failed"

python3 - "$WORKDIR/http/state_before_reset.json" "$WORKDIR/bodies/to_delete.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
tracks = (d.get("result") or {}).get("tracks") or []
audio = [t for t in tracks if isinstance(t, dict) and str(t.get("type", "")) == "audio"]
def idx(t):
    v = t.get("user_track_index", t.get("track_index"))
    try:
        return int(v)
    except Exception:
        return 0
audio.sort(key=idx)
keep, drop = (audio[:1] if audio else []), audio[1:]
rows = [{"track_id": t.get("track_id") or t.get("id") or "", "track_index": idx(t)} for t in drop]
json.dump(rows, open(sys.argv[2], "w", encoding="utf-8"))
json.dump([{"track_id": t.get("track_id") or t.get("id") or ""} for t in keep],
          open(sys.argv[2] + ".keep", "w", encoding="utf-8"))
PY
DELETED=0
while IFS= read -r row; do
  track_id="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["track_id"])' "$row")"
  track_index="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1]).get("track_index",""))' "$row")"
  [[ -n "$track_id" ]] || continue
  ARGS="{\"track_id\":\"$track_id\"}"
  [[ -n "$track_index" ]] && ARGS="{\"track_id\":\"$track_id\",\"track_index\":$track_index}"
  [[ "$(invoke_tool track.delete "$ARGS" true "$WORKDIR/http/track_delete_$DELETED")" == "ok" ]] \
    || fail_functional "track.delete $track_id failed"
  DELETED=$((DELETED + 1))
done < <(python3 -c 'import json,sys; [print(json.dumps(r)) for r in json.load(open(sys.argv[1]))]' "$WORKDIR/bodies/to_delete.json" 2>/dev/null)
if (( DELETED > 0 )); then ok "deleted existing audio tracks: $DELETED"; else ok "no existing audio tracks to delete"; fi

REMAINING="$(python3 -c 'import json; print(len(json.load(open(sys.argv[1]))))' "$WORKDIR/bodies/to_delete.json.keep" 2>/dev/null || echo 0)"
TRACK1_ID=""
if (( REMAINING == 1 )); then
  TRACK1_ID="$(python3 -c 'import json; print(json.load(open(sys.argv[1]))[0]["track_id"])' "$WORKDIR/bodies/to_delete.json.keep")"
  [[ -n "$TRACK1_ID" ]] || fail_functional "fixture reset left one audio track, but its track ID could not be resolved"
  ok "reusing remaining audio track as Track 1: $TRACK1_ID"
else
  [[ "$(invoke_tool track.add_audio '{"name":"Track 1"}' true "$WORKDIR/http/track_add_1")" == "ok" ]] \
    || fail_functional "track.add_audio Track 1 failed"
  TRACK1_ID="$(json_field "$WORKDIR/http/track_add_1.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
fi
[[ "$(invoke_tool track.add_audio '{"name":"Track 2"}' true "$WORKDIR/http/track_add_2")" == "ok" ]] \
  || fail_functional "track.add_audio Track 2 failed"
TRACK2_ID="$(json_field "$WORKDIR/http/track_add_2.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
[[ -n "$TRACK1_ID" && -n "$TRACK2_ID" ]] || fail_functional "could not resolve fixture track IDs: track1=$TRACK1_ID track2=$TRACK2_ID"

import_audio_fixture() {
  # import_audio_fixture <track-id> <file-path> <label> <out-prefix>
  local track_id="$1" file_path="$2" label="$3" prefix="$4" status
  status="$(invoke_tool clip.import_media_to_track \
    "{\"track_id\":\"$track_id\",\"file_path\":\"$file_path\",\"start_time\":0,\"media_type\":\"audio\",\"mode\":\"non_destructive\"}" \
    true "$prefix")"
  if [[ "$status" != "ok" ]]; then
    error="$(json_field "${prefix}.json" 'str(d.get("error",""))')"
    if [[ "$error" == *"Unknown command: import_media_to_track"* ]]; then
      warn "$label: kernel does not support import_media_to_track; falling back to clip.import_audio"
      status="$(invoke_tool clip.import_audio \
        "{\"track_id\":\"$track_id\",\"file_path\":\"$file_path\",\"offset_time\":0}" true "$prefix")"
    fi
  fi
  [[ "$status" == "ok" ]] || fail_functional "import $label audio failed: status=$status"
}

import_audio_fixture "$TRACK1_ID" "$TRACK1_PATH" "Track 1" "$WORKDIR/http/import_track1"
import_audio_fixture "$TRACK2_ID" "$TRACK2_PATH" "Track 2" "$WORKDIR/http/import_track2"
ok "fixture ready: Track 1=$TRACK1_ID Track 2=$TRACK2_ID"
sleep 0.75

# ---------------------------------------------------------------- observe turn
step "Run chat observe turn"
CONVERSATION_ID="mix_single_tick_e2e_$(date '+%Y%m%d_%H%M%S')"
agent_chat_settled "$CONVERSATION_ID" "$OBSERVE_MESSAGE" "$WORKDIR/http/chat_observe"
OBSERVE_STOP="$(json_field "$WORKDIR/http/chat_observe_settled.json" 'str(d.get("stop_reason",""))')"
case "$OBSERVE_STOP" in
  done|needs_confirmation) ;;
  *) fail_functional "observe turn stop_reason: got '$OBSERVE_STOP', want 'done' or 'needs_confirmation' (settled_from=$(json_field "$WORKDIR/http/chat_observe_settled.json" 'str(d.get("settled_from",""))'))" ;;
esac
OBSERVE_REPLY="$(json_field "$WORKDIR/http/chat_observe_settled.json" 'str(d.get("reply",""))')"
READONLY_DUE_TO_INCOMPLETE_L3=0
if [[ "$OBSERVE_REPLY" != *"$EXECUTE_NEEDLE"* && "$OBSERVE_REPLY" != *"$CONTINUE_NEEDLE"* && "$OBSERVE_REPLY" != *"$CONFIRM_NEEDLE"* ]]; then
  if python3 - "$WORKDIR/http/chat_observe_settled.json" <<'PY'
import json, sys
reply = str(json.load(open(sys.argv[1], encoding="utf-8")).get("reply", ""))
first = any(k in reply for k in ("L3", "深度", "spectrogram"))
second = any(k in reply for k in ("building", "partial", "未完整", "未完成", "不可靠", "还在构建", "正在构建"))
sys.exit(0 if (first and second) else 1)
PY
  then
    READONLY_DUE_TO_INCOMPLETE_L3=1
    ok "observe stayed read-only while L3 acoustic package was incomplete"
  else
    fail_functional "observe reply did not ask for execution confirmation: $OBSERVE_REPLY"
  fi
fi
if [[ "$READONLY_DUE_TO_INCOMPLETE_L3" -eq 0 ]]; then
  STORED_LINE="$(wait_log_pattern "[mix.tick.pending] stored conversation=$CONVERSATION_ID" 10)"
  if [[ "$STORED_LINE" != *"track=$TRACK2_ID"* && "$STORED_LINE" != *"track $TRACK2_ID"* && "$STORED_LINE" != *"target=track:$TRACK2_ID"* ]]; then
    fail_functional "pending candidate did not target Track 2. line=$STORED_LINE"
  fi
  ok "pending candidate stored: $STORED_LINE"
fi

# ---------------------------------------------------------------- vocal guard
step "Run unresolved vocal clarification guard"
VOCAL_CONVERSATION_ID="mix_single_tick_vocal_clarify_$(date '+%Y%m%d_%H%M%S')"
agent_chat_settled "$VOCAL_CONVERSATION_ID" "$VOCAL_MESSAGE" "$WORKDIR/http/chat_vocal_guard"
VOCAL_STOP="$(json_field "$WORKDIR/http/chat_vocal_guard_settled.json" 'str(d.get("stop_reason",""))')"
[[ "$VOCAL_STOP" == "needs_clarification" ]] \
  || fail_functional "unresolved vocal stop_reason: got '$VOCAL_STOP', want 'needs_clarification'"
VOCAL_REPLY="$(json_field "$WORKDIR/http/chat_vocal_guard_settled.json" 'str(d.get("reply",""))')"
VOCAL_REPLY_LC="$(python3 -c 'import sys; print(sys.argv[1].lower())' "$VOCAL_REPLY")"
if [[ "$VOCAL_REPLY" != *"哪条"* && "$VOCAL_REPLY" != *"哪一条"* && "$VOCAL_REPLY" != *"哪一轨"* && "$VOCAL_REPLY_LC" != *"which track"* ]]; then
  fail_functional "unresolved vocal reply did not ask which track is vocal: $VOCAL_REPLY"
fi
if [[ -f "$AGENT_LOG" ]] && grep -Fq "[mix.tick.pending] stored conversation=$VOCAL_CONVERSATION_ID" "$AGENT_LOG" 2>/dev/null; then
  fail_functional "unresolved vocal clarification stored pending unexpectedly: $(grep -F "[mix.tick.pending] stored conversation=$VOCAL_CONVERSATION_ID" "$AGENT_LOG" | head -1)"
fi
ok "unresolved vocal asks clarification without pending: $VOCAL_REPLY"

# ---------------------------------------------------------------- confirm turn
step "Run confirmation turn"
agent_chat_settled "$CONVERSATION_ID" "$CONFIRM_MESSAGE" "$WORKDIR/http/chat_confirm"
CONFIRM_STOP="$(json_field "$WORKDIR/http/chat_confirm_settled.json" 'str(d.get("stop_reason",""))')"
if [[ "$READONLY_DUE_TO_INCOMPLETE_L3" -eq 1 ]]; then
  [[ "$CONFIRM_STOP" == "no_pending_mix_tick_candidate" ]] \
    || fail_functional "confirmation stop_reason: got '$CONFIRM_STOP', want 'no_pending_mix_tick_candidate'"
  ok "confirmation correctly found no pending tick after incomplete L3 read-only observe"
  ROUTE=""
else
  [[ "$CONFIRM_STOP" == "mix_tick_applied_reobserved" ]] \
    || fail_functional "confirmation stop_reason: got '$CONFIRM_STOP', want 'mix_tick_applied_reobserved' (settled_from=$(json_field "$WORKDIR/http/chat_confirm_settled.json" 'str(d.get("settled_from",""))'))"
  tool_names_of "$WORKDIR/http/chat_confirm_settled.json" > "$WORKDIR/bodies/confirm_route.txt"
  ROUTE="$(python3 -c 'print(" -> ".join(l.rstrip("\n") for l in open(sys.argv[1]) if l.strip()))' "$WORKDIR/bodies/confirm_route.txt")"
  assert_any_present() {
    local label="$1"; shift
    local alias found=0
    for alias in "$@"; do grep -qx "$alias" "$WORKDIR/bodies/confirm_route.txt" && { found=1; break; }; done
    (( found )) || fail_functional "Expected $label in executed route. route=$ROUTE"
  }
  assert_any_absent() {
    local label="$1"; shift
    local alias
    for alias in "$@"; do grep -qx "$alias" "$WORKDIR/bodies/confirm_route.txt" && fail_functional "Unexpected $label in executed route. route=$ROUTE"; done
  }
  assert_any_present "mix.propose_tick" mix.propose_tick mix_propose_tick
  assert_any_present "mix.apply_tick" mix.apply_tick mix_apply_tick
  assert_any_present "re-observation tool" mix.observe mix_observe mix.request_observation mix_request_observation
  assert_any_absent "daw.invoke" daw.invoke daw_invoke
  assert_any_absent "track.volume" track.volume track_volume
  wait_log_pattern "[mix.tick.pending] explicit confirmation routed conversation=$CONVERSATION_ID" 10 >/dev/null
  wait_log_pattern "[mix.tick.pending] applied and reobserved conversation=$CONVERSATION_ID" 10 >/dev/null
  ok "confirmation routed + applied and reobserved (route: $ROUTE)"
fi

step "Summary"
{
  echo "conversation: $CONVERSATION_ID"
  echo "route: $ROUTE"
  echo "observe reply: $OBSERVE_REPLY"
  echo "confirm reply: $(json_field "$WORKDIR/http/chat_confirm_settled.json" 'str(d.get("reply",""))')"
} >&2
OUTCOME="all_green"
ok "mix single tick E2E passed"
exit 0
