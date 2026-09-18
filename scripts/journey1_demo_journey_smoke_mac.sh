#!/usr/bin/env bash
# journey1_demo_journey_smoke_mac.sh — PORT-JOURNEY-1-MAC demo journey driver (mac).
#
# Mac port of scripts/journey1_demo_journey_smoke.ps1 (2026-09-13, the PC-side
# authoritative demo-journey driver). One script, one demo journey over the real
# two-process stack (VitApp kernel + Go agent; the Godot frontend is out of
# scope — the agent HTTP face drives everything, same as the ps1):
#
#   copy project (912.vit shape: 6 stem tracks, bass = empty track without EQ)
#     -> load it (isolated workspace)
#     -> authority = full access
#     -> send "帮低音轨做个均衡实验，然后让我试听" (real LLM turn)
#     -> five explicit segment assertions (card PORT-JOURNEY-1-MAC framing):
#        S1 project open      open_project via /agent/invoke -> ok + tracks
#                            visible + the bass track present
#        S2 authority         /agent/authority echoes full_project_access
#        S3 experiment        chat chain runs, delivered text is substantive
#                            AND carries no direction-asking (ps1 A1)
#        S4 load              deterministic rack_add_node probe of a promoted
#                            PCA subject is not denied, returns plugin_id +
#                            plugin_instance_ready, and the UI projection
#                            shows the plugin (ps1 A2)
#        S5 audition          audition.*/mix_tick event or audition_session_id
#                            seen on the conversation surface (ps1 A3)
#     -> plus the ps1 assertion 4 control: manual save point, stack teardown,
#        reopen — the live reopened conversation must NOT carry pre-save-point
#        history and must leave no continuation-stall WARN (ps1 A4).
#
# ps1 ↔ mac segment mapping (load-path reconnaissance identical — kernel
# open_project command driven through the agent tool face; see the ps1 header
# for the harness.go anchors, unchanged on mac):
#
#   | ps1 (journey1_demo_journey_smoke.ps1)      | mac (this script)                    |
#   |--------------------------------------------|--------------------------------------|
#   | param(...) block                           | flag parsing below                   |
#   | From-B64 / [char] code-point strings       | plain UTF-8 literals (no ANSI        |
#   | (PS 5.1 code-page workaround)              | code page on macOS)                  |
#   | Get-NetTCPConnection listener checks       | lsof port_listener_pid               |
#   | Invoke-RestMethod + UTF-8 body bytes       | curl + python json.dumps bodies      |
#   | Get-FileHash SHA256                        | shasum -a 256                        |
#   | Get-TreeFingerprint (PS StringBuilder)     | python3 tree walker                  |
#   | Start-Process kernel (VIT_PROJECT_XML,     | ( cd kernel_root; VIT_PROJECT_XML=   |
#   |   Hidden, WorkingDirectory)                |   ... exec kernel ) &                |
#   | Start-Process agent (VIT_HISTORY_DRAFT_    | ( cd agent_cwd; VIT_HISTORY_DRAFT_   |
#   |   ROOT, VIT_DAW_DEV_ROOT, -last-log-path)  |   ROOT/... exec agent ) &            |
#   |   -keep-last-log-lines 8000                |   + VIT_ORCHESTRATION_STORE_PATH     |
#   |                                            |     isolation, -vsp-hub-url ""       |
#   |   (both A5/C2 mac harness additions, jour- |
#   |    ney-neutral: fresh orchestration store  |
#   |    per run, no VSP hub in this topology)   |
#   | Stop-Process + 30s port release wait       | stop_process TERM->KILL + wait       |
#   |                                            |   (A1 finding: SIGTERM exit 143)     |
#   | KernelExe candidates (PC build dirs)       | cmake+make build in the run workdir  |
#   |                                            |   (A5) or --kernel-bin reuse         |
#   | go build VitAgent.journey1.exe             | go build -o workdir/bin/vitagent     |
#   | ProjectSource D:\Godot\...912.vit          | ~/Documents/vit-daw-frontend/912.vit |
#   | EqPluginIdentifier VST3-bx_hybrid V2-...   | promoted mac PCA subject (default    |
#   |   (PC-promoted static EQ)                  |   C1 comp Mono, --probe-plugin-      |
#   |                                            |   identifier; U2: mac is Waves-only, |
#   |                                            |   no EQ family certified — the       |
#   |                                            |   probe's ps1 semantics is "a        |
#   |                                            |   promoted processor lands", the     |
#   |                                            |   family is machine-local state)     |
#   | A1..A4 assertions + JOURNEY1_VERDICT       | identical A1..A4 + S1..S5 rows +     |
#   |                                            |   JOURNEY1_MAC_VERDICT               |
#   | exit 0/1/2 (green/red|inconclusive/env)    | identical                            |
#
# Isolation contract (AGENTS.md §10, same as ps1):
#   * the user project (912.vit) and its .vit_history are opened READ-ONLY and
#     fingerprinted before/after; only a byte copy inside the run workdir is
#     operated on
#   * the kernel default project XML is redirected with VIT_PROJECT_XML to a
#     copy inside the run workdir; the kernel additionally runs under a fake
#     VitApp root in the workdir (A5/C2 climbToVitAppRoot markers), so the
#     repo tree is never written
#   * agent history drafts are redirected with VIT_HISTORY_DRAFT_ROOT and the
#     orchestration store with VIT_ORCHESTRATION_STORE_PATH
#   * every run uses a fresh artifact directory (mktemp; AGENTS §9)
#
# LLM precondition (card PORT-JOURNEY-1-MAC): the experiment segment is a real
# LLM turn. The script preflights the engine config (~/.vit/config.json or
# VIT_AGENT_LLM_*/OPENAI_* env; agent/internal/config/config.go) and refuses to
# run (env failure, exit 2) when it is incomplete — no stub replies. The API
# key is never printed, logged, or written into artifacts: only a boolean
# has_api_key plus the model name and base-url host are recorded.
#
# §8 probabilistic discipline (pre-declared on the card, not re-derived here):
# at most 3 valid runs; success = one run exiting 0 with all five segment
# assertions green (explicit per-segment verdicts, not log stage names); two
# consecutive failures at the same deterministic breakpoint stop the reruns.
#
# Usage:
#   ./scripts/journey1_demo_journey_smoke_mac.sh                          # full run
#   ./scripts/journey1_demo_journey_smoke_mac.sh --kernel-bin <path>/VitApp
#   ./scripts/journey1_demo_journey_smoke_mac.sh --skip-agent-build --agent-bin <p>
#   ./scripts/journey1_demo_journey_smoke_mac.sh --workdir PATH           # reuse a workdir
#   ./scripts/journey1_demo_journey_smoke_mac.sh --skip-reopen            # skip phase B
#
# Exit codes (ps1 parity):
#   0  all assertions green (the journey gate)
#   1  journey ran, at least one assertion RED (or inconclusive)
#   2  environment / load-path failure (no assertion verdict claimed)
#
# All artifacts land under the run workdir (run meta, prereq.txt, per-step
# JSON under phase_a/ and phase_b/, kernel/agent logs, both binary sha256s,
# journey1_report.json), printed to stderr.

set -uo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
PROJECT_SOURCE="$HOME/Documents/vit-daw-frontend/912.vit"
PROBE_PLUGIN_IDENTIFIER="VST3-C1 comp Mono-10456661-65e94c5e"
BASS_TRACK_NAME="bass"
ZONE_ID="Z3"
TURN_BUDGET_SECONDS=480
MAX_NUDGES=4
POLL_SECONDS=5
STARTUP_TIMEOUT_SECONDS=90
KERNEL_DWELL_SECONDS=15
REOPEN_DWELL_SECONDS=20
STOP_GRACE_SECONDS=10
SKIP_REOPEN=0
WORKDIR_ARG=""
TRACKTION_DIR=""
CMAKE_PROXY=""
FETCH_SRC_SPECS=""
OUTPUT_PATH=""

KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

# Journey literals — the same texts the ps1 carries in base64 / code points
# (decoded there only to survive the PS 5.1 ANSI code page; macOS needs no
# such dance, the literals ride UTF-8).
PROMPT_TEXT="帮低音轨做个均衡实验，然后让我试听"
NUDGE_TEXT="可以执行"
PLACEHOLDER_TEXT="我还在继续处理这个任务，完成后再向你汇报。"
# A1 direction-asking vocabulary (ps1 $AskWords + markers).
ASK_WORDS=("哪个" "方向" "选择" "请问" "你希望" "倾向")
ASK_WORD_OR="还是"
FULLWIDTH_QUESTION="？"

usage() {
  cat <<'EOF'
journey1_demo_journey_smoke_mac.sh — PORT-JOURNEY-1-MAC demo journey driver.

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --agent-http URL        Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --skip-agent-build      With --agent-bin: do not build the agent
  --agent-bin PATH        Agent binary to start (only with --skip-agent-build)
  --project-source PATH   Demo project to copy (default
                          ~/Documents/vit-daw-frontend/912.vit)
  --probe-plugin-identifier ID
                          PCA-promoted plugin for the S4 deterministic load
                          probe (default "VST3-C1 comp Mono-10456661-65e94c5e";
                          must be a promoted subject in the machine-local PCA
                          stores ~/.vit/processor_control_attestations.*)
  --bass-track-name NAME  Journey target track (default "bass")
  --turn-budget-seconds N Chat turn budget (default 480)
  --max-nudges N          Max nudge slices after the prompt (default 4)
  --poll-seconds N        Goal poll interval inside waiting_continue (default 5)
  --startup-timeout SECONDS  Kernel/agent startup timeout (default 90)
  --kernel-dwell SECONDS  Kernel warm-up dwell after start (default 15)
  --reopen-dwell SECONDS  Phase B reopen dwell (default 20)
  --stop-grace SECONDS    SIGTERM grace before SIGKILL (default 10)
  --skip-reopen           Skip phase B (A4 becomes not_observable)
  --workdir PATH          Reuse PATH as the run workdir instead of a fresh
                          mktemp directory (never overwrites other runs)
  --tracktion-dir PATH    tracktion_engine source dir (kernel build only)
  --cmake-proxy URL       HTTP(S) proxy for kernel cmake FetchContent only
  --fetch-src NAME=PATH   Offline FetchContent source override (repeatable)
  --output PATH           Also write the report JSON to PATH
  -h, --help              Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --project-source) PROJECT_SOURCE="$2"; shift 2 ;;
    --probe-plugin-identifier) PROBE_PLUGIN_IDENTIFIER="$2"; shift 2 ;;
    --bass-track-name) BASS_TRACK_NAME="$2"; shift 2 ;;
    --turn-budget-seconds) TURN_BUDGET_SECONDS="$2"; shift 2 ;;
    --max-nudges) MAX_NUDGES="$2"; shift 2 ;;
    --poll-seconds) POLL_SECONDS="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --kernel-dwell) KERNEL_DWELL_SECONDS="$2"; shift 2 ;;
    --reopen-dwell) REOPEN_DWELL_SECONDS="$2"; shift 2 ;;
    --stop-grace) STOP_GRACE_SECONDS="$2"; shift 2 ;;
    --skip-reopen) SKIP_REOPEN=1; shift ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    --cmake-proxy) CMAKE_PROXY="$2"; shift 2 ;;
    --fetch-src) FETCH_SRC_SPECS+="${2}"$'\n'; shift 2 ;;
    --output) OUTPUT_PATH="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }

for tool in go cmake make python3 lsof pgrep shasum curl; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

RUN_ID="journey1_mac_$(date '+%Y%m%d-%H%M%S')"
WORKDIR="${WORKDIR_ARG:-$(mktemp -d "${TMPDIR:-/tmp}/journey1_mac.XXXXXXXX")}"
mkdir -p "$WORKDIR"/{bin,http,bodies,phase_a,phase_b,logs,build}
PROJECT_DIR="$WORKDIR/project"
AGENT_DRAFTS="$WORKDIR/agent_drafts"
KERNEL_WORKSPACE="$WORKDIR/kernel_workspace"
mkdir -p "$PROJECT_DIR" "$AGENT_DRAFTS" "$KERNEL_WORKSPACE"/{Settings,Logs}
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"
AGENT_STATE_DIR="$WORKDIR/agent_state"
mkdir -p "$AGENT_STATE_DIR" "$WORKDIR/agent_cwd"

COPY_PROJECT="$PROJECT_DIR/journey1_912.vit"
PREREQ_FILE="$WORKDIR/prereq.txt"
REPORT_PATH="$WORKDIR/journey1_report.json"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
REPO_SETTINGS_XML="$REPO_ROOT/VitApp/Workspace/Settings/Settings.xml"
USER_HISTORY_DIR="$(dirname "$PROJECT_SOURCE")/.vit_history"

KERNEL_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
OUTCOME="env_failure"
FATAL_MSG=""
EXIT_CODE=2
KERNEL_BIN=""
AGENT_BIN=""
KERNEL_SHA=""
AGENT_SHA=""

log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" >&2; }
bad()  { echo "RED: $*" >&2; }
info() { echo "info: $*" >&2; }

prereq() {
  echo "$1" | tee -a "$PREREQ_FILE" >&2
}

fatal_env() {
  FATAL_MSG="$1"
  bad "fatal: $FATAL_MSG"
  OUTCOME="env_failure"
  exit 2
}

file_sha256() {
  if [[ -f "$1" ]]; then shasum -a 256 "$1" | cut -d' ' -f1; else echo "absent"; fi
}

json_field() {
  # json_field <file> <python expr against d> — small JSON extraction helper.
  python3 - "$1" "$2" <<'PY'
import json, sys
with open(sys.argv[1], "r", encoding="utf-8") as f:
    d = json.load(f)
print(eval(sys.argv[2], {"d": d}))
PY
}

# ------------------------------------------------- stack teardown (early-bound)
# Defined before the first possible fatal_env so the EXIT trap below can always
# run teardown + fingerprints + report, even when the journey aborts in
# preflight (LLM guard) or mid-phase.

port_listener_pid() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

stop_process() {
  # stop_process <pid> — SIGTERM + grace, then SIGKILL; fills STOP_RECORD.
  local pid="$1" start elapsed code signal="SIGTERM"
  kill -TERM "$pid" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -KILL "$pid" 2>/dev/null || true; signal="SIGTERM+SIGKILL"; break; }
    sleep 1
  done
  wait "$pid" 2>/dev/null
  code=$?
  elapsed=$((SECONDS - start))
  STOP_RECORD="${signal}:${code}:${elapsed}"
}
# (no errexit anywhere: journey assertions must not abort the run — verdicts and
# teardown always run; fatal_env is the only early exit path)

stop_stack() {
  # stop_stack <phase-tag> — agent first, then kernel, then port release wait.
  local tag="$1" port busy
  [[ -z "$AGENT_PID" ]] || {
    log "stopping agent (pid $AGENT_PID, phase $tag)..."
    stop_process "$AGENT_PID"
    AGENT_STOP_RECORD="$STOP_RECORD"
    log "agent stop: $AGENT_STOP_RECORD"
  }
  AGENT_PID=""
  [[ -z "$KERNEL_PID" ]] || {
    log "stopping kernel (pid $KERNEL_PID, phase $tag)..."
    stop_process "$KERNEL_PID"
    KERNEL_STOP_RECORD="$STOP_RECORD"
    log "kernel stop: $KERNEL_STOP_RECORD (A1 finding: SIGTERM exit 143 expected)"
  }
  KERNEL_PID=""
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    busy=""
    for port in "$AGENT_HTTP_PORT" "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy="$busy $port"
    done
    [[ -z "$busy" ]] && break
    sleep 1
  done
  for port in "$AGENT_HTTP_PORT" "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    [[ -n "$(port_listener_pid "$port")" ]] && prereq "port_still_busy_after_${tag}=$port"
  done
}

cleanup() {
  local rc=$?
  trap - EXIT INT TERM
  step "Teardown"
  stop_stack "final"
  prereq "repo_default_project_sha256_after=$(file_sha256 "$REPO_DEFAULT_PROJECT")"
  prereq "repo_settings_sha256_after=$(file_sha256 "$REPO_SETTINGS_XML")"
  prereq "user_project_sha256_after=$(file_sha256 "$PROJECT_SOURCE")"
  prereq "user_history_fingerprint_after=$(python3 - "$USER_HISTORY_DIR" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
if not os.path.isdir(root):
    print("absent"); raise SystemExit
h = hashlib.sha256()
for dirpath, dirnames, filenames in sorted(os.walk(root)):
    dirnames.sort()
    for name in sorted(filenames):
        p = os.path.join(dirpath, name)
        st = os.stat(p)
        h.update(("%s:%d:%s;" % (p[len(root):], st.st_size, st.st_mtime)).encode("utf-8"))
print(h.hexdigest())
PY
)"
  prereq "finished=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  python3 - "$REPORT_PATH" "$WORKDIR" "$RUN_ID" "$PROJECT_SOURCE" "$COPY_PROJECT" \
    "$KERNEL_BIN" "$AGENT_BIN" "$PROMPT_TEXT" "$OUTCOME" "$FATAL_MSG" \
    "$KERNEL_STOP_RECORD" "$AGENT_STOP_RECORD" "$KERNEL_SHA" "$AGENT_SHA" <<'PY'
import json, os, sys

(out, workdir, run_id, project_source, copy_project, kernel_bin, agent_bin,
 prompt, outcome, fatal_msg, kernel_stop, agent_stop, kernel_sha, agent_sha) = sys.argv[1:15]

def load(path, default=None):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    except Exception:
        return default

def loadl(path):
    try:
        with open(path, "r", encoding="utf-8") as f:
            return [json.loads(line) for line in f if line.strip()]
    except Exception:
        return []

def frag(name):
    for sub in ("phase_a", "phase_b", ""):
        p = os.path.join(workdir, sub, name) if sub else os.path.join(workdir, name)
        if os.path.isfile(p):
            return load(p)
    return None

prereq = []
if os.path.isfile(os.path.join(workdir, "prereq.txt")):
    prereq = [l.rstrip("\n") for l in open(os.path.join(workdir, "prereq.txt"), encoding="utf-8") if l.strip()]

report = {
    "schema_version": "vit_demo_journey_driver.mac.v1",
    "card": "PORT-JOURNEY-1-MAC",
    "run_root": workdir,
    "run_id": run_id,
    "project_source": project_source,
    "copy_project": copy_project,
    "kernel_exe": kernel_bin,
    "kernel_sha256": kernel_sha,
    "agent_binary": agent_bin,
    "agent_sha256": agent_sha,
    "prompt": prompt,
    "verdict": outcome,
    "fatal": fatal_msg,
    "assertions": frag("assertions.json") or {},
    "five_segments": (frag("assertions.json") or {}).get("five_segments", {}),
    "phase_a": {
        "load": frag("step_load.json"),
        "authority": frag("authority_response.json"),
        "chat_slices": loadl(os.path.join(workdir, "phase_a", "slices.jsonl")),
        "a1": frag("a1_analysis.json"),
        "a2_probe": frag("a2_probe.json"),
        "a2_projection": frag("a2_projection.json"),
        "a3": frag("a3_audition.json"),
        "save_chain": frag("save_chain.json"),
        "sessions_after_load": frag("sessions_after_load.json"),
        "sessions_after_turn": frag("sessions_after_turn.json"),
    },
    "phase_b": frag("reopen.json"),
    "kernel_stop": {"record": kernel_stop},
    "agent_stop": {"record": agent_stop},
    "prereq": prereq,
}
try:
    with open(out, "w", encoding="utf-8") as f:
        json.dump(report, f, ensure_ascii=False, indent=2)
    print("report written: %s" % out, file=sys.stderr)
except Exception as e:
    print("report write failed: %s" % e, file=sys.stderr)
PY
  [[ -n "$OUTPUT_PATH" ]] && { mkdir -p "$(dirname "$OUTPUT_PATH")"; cp "$REPORT_PATH" "$OUTPUT_PATH" 2>/dev/null || true; }
  log "workdir: $WORKDIR"
  case "$OUTCOME" in
    all_green) EXIT_CODE=0 ;;
    assertions_red|journey_inconclusive) EXIT_CODE=1 ;;
    *) EXIT_CODE=2 ;;
  esac
  echo "" >&2
  echo "JOURNEY1_MAC_VERDICT $OUTCOME" >&2
  exit "$EXIT_CODE"
}
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------- preflight
AGENT_HTTP_ADDR="${AGENT_HTTP#http://}"
AGENT_HTTP_ADDR="${AGENT_HTTP_ADDR#https://}"
AGENT_HTTP_PORT="${AGENT_HTTP_ADDR##*:}"

step "Preflight: LLM engine config (presence only — the key never reaches logs)"
LLM_CFG_SUMMARY="$(python3 - <<'PY'
import json, os, sys
def env(name):
    v = os.environ.get(name, "")
    return v.strip()
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
)" || fatal_env "LLM config preflight crashed"
info "llm_config=$LLM_CFG_SUMMARY"
python3 -c 'import json,sys; sys.exit(0 if json.loads(sys.argv[1])["complete"] else 1)' "$LLM_CFG_SUMMARY" \
  || fatal_env "LLM engine config incomplete (card stop condition: 环境缺失 → blocked，不得以桩应答替代): $LLM_CFG_SUMMARY"

step "Preflight: run root + repo fingerprints"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | tr '\n' ';' | sed 's/;$//')"
  echo "agent_http=$AGENT_HTTP"
  echo "project_source=$PROJECT_SOURCE"
  echo "probe_plugin_identifier=$PROBE_PLUGIN_IDENTIFIER"
  echo "bass_track_name=$BASS_TRACK_NAME"
  echo "turn_budget_seconds=$TURN_BUDGET_SECONDS max_nudges=$MAX_NUDGES"
  echo "skip_reopen=$SKIP_REOPEN"
  echo "llm_config=$LLM_CFG_SUMMARY"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$PROJECT_SOURCE" ]] || fatal_env "project source not found: $PROJECT_SOURCE"
[[ -f "$REPO_DEFAULT_PROJECT" ]] || fatal_env "repo default project not found: $REPO_DEFAULT_PROJECT"

USER_HISTORY_FP_BEFORE="$(python3 - "$USER_HISTORY_DIR" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
if not os.path.isdir(root):
    print("absent"); raise SystemExit
h = hashlib.sha256()
for dirpath, dirnames, filenames in sorted(os.walk(root)):
    dirnames.sort()
    for name in sorted(filenames):
        p = os.path.join(dirpath, name)
        st = os.stat(p)
        h.update(("%s:%d:%s;" % (p[len(root):], st.st_size, st.st_mtime)).encode("utf-8"))
print(h.hexdigest())
PY
)"
prereq "user_project_sha256_before=$(file_sha256 "$PROJECT_SOURCE")"
prereq "user_history_fingerprint_before=$USER_HISTORY_FP_BEFORE"
prereq "repo_default_project_sha256_before=$(file_sha256 "$REPO_DEFAULT_PROJECT")"
prereq "repo_settings_sha256_before=$(file_sha256 "$REPO_SETTINGS_XML")"

step "Preflight: ports $AGENT_HTTP_PORT/$KERNEL_PORT_REQ/$ZMQ_PUB_PORT/$ZMQ_LOG_PORT must be free (stack ownership)"
for port in "$AGENT_HTTP_PORT" "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  pid="$(port_listener_pid "$port")"
  [[ -z "$pid" ]] || fatal_env "port $port already has a listener (pid $pid); AGENTS §9 single-owner rule"
  prereq "port_free=$port"
done

step "Isolate: copy the project and the kernel default XML into the run dir"
cp "$PROJECT_SOURCE" "$COPY_PROJECT"
prereq "copy_project=$COPY_PROJECT"
prereq "copy_project_sha256=$(file_sha256 "$COPY_PROJECT")"
cp "$REPO_DEFAULT_PROJECT" "$KERNEL_WORKSPACE/default_project.xml"

# ---------------------------------------------------------------- binaries
KERNEL_BIN=""
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fatal_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  log "using provided kernel binary: $KERNEL_BIN (sha256/mtime recorded)"
else
  if [[ -z "$TRACKTION_DIR" ]]; then
    TRACKTION_DIR="$REPO_ROOT/tracktion_engine"
  fi
  [[ -f "$TRACKTION_DIR/CMakeLists.txt" ]] \
    || fatal_env "tracktion_engine not usable at $TRACKTION_DIR (submodule not checked out? pass --tracktion-dir or --kernel-bin)"
  CMAKE_ENV=()
  if [[ -n "$CMAKE_PROXY" ]]; then
    CMAKE_ENV=("HTTPS_PROXY=$CMAKE_PROXY" "HTTP_PROXY=$CMAKE_PROXY" "https_proxy=$CMAKE_PROXY" "http_proxy=$CMAKE_PROXY")
    log "kernel build downloads routed through $CMAKE_PROXY"
  fi
  FETCH_CONTENT_DEFINES=()
  while IFS= read -r spec; do
    [[ -n "$spec" ]] || continue
    name="${spec%%=*}"; path="${spec#*=}"
    [[ "$name" != "$spec" && -n "$name" && -n "$path" ]] || fatal_env "--fetch-src expects NAME=PATH, got: $spec"
    [[ -d "$path" ]] || fatal_env "--fetch-src source dir does not exist: $path"
    FETCH_CONTENT_DEFINES+=("-DFETCHCONTENT_SOURCE_DIR_${name}=${path}")
    log "fetch-src override: FETCHCONTENT_SOURCE_DIR_${name} -> $path"
  done <<< "$FETCH_SRC_SPECS"
  log "building VitApp kernel (cmake+make, sources at $REPO_ROOT/VitApp, tracktion at $TRACKTION_DIR)..."
  if ! env ${CMAKE_ENV[@]+"${CMAKE_ENV[@]}"} cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" \
        -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug \
        -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        ${FETCH_CONTENT_DEFINES[@]+"${FETCH_CONTENT_DEFINES[@]}"} \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fatal_env "kernel cmake configure failed (see $WORKDIR/logs/kernel_configure.log)"
  fi
  if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp \
        > "$WORKDIR/logs/kernel_build.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_build.log" >&2
    fatal_env "kernel make failed (see $WORKDIR/logs/kernel_build.log)"
  fi
  KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  [[ -x "$KERNEL_BIN" ]] || fatal_env "kernel binary not found after build: $KERNEL_BIN"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "kernel built: $KERNEL_BIN"
fi
KERNEL_SHA="$(cut -d' ' -f1 "$WORKDIR/kernel_bin.sha256")"

if [[ "$SKIP_AGENT_BUILD" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" ]] || fatal_env "--skip-agent-build requires --agent-bin"
  [[ -x "$AGENT_BIN_ARG" ]] || fatal_env "agent binary is not executable: $AGENT_BIN_ARG"
  AGENT_BIN="$AGENT_BIN_ARG"
else
  [[ -z "$AGENT_BIN_ARG" ]] || fatal_env "--agent-bin is only valid together with --skip-agent-build"
  AGENT_BIN="$WORKDIR/bin/vitagent"
  log "building agent (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$AGENT_BIN" ./cmd/vitagent) \
    || fatal_env "go build agent failed"
fi
shasum -a 256 "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256"
AGENT_SHA="$(cut -d' ' -f1 "$WORKDIR/agent_bin.sha256")"
prereq "kernel_binary=$KERNEL_BIN sha256=$KERNEL_SHA mtime=$(stat -f %Sm "$KERNEL_BIN")"
prereq "agent_binary=$AGENT_BIN sha256=$AGENT_SHA mtime=$(stat -f %Sm "$AGENT_BIN")"

# ---------------------------------------------------------------- http helpers
http_json() {
  # http_json <method> <url> <body-file-or-""> <out-file> [timeout]
  # Transport failure -> return 1; otherwise print the HTTP status code.
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

# ---------------------------------------------------------------- stack control
start_stack() {
  # start_stack <phase-tag-A-or-B> <phase-dir>: starts kernel+agent.
  local tag="$1" phase_dir="$2" tag_lower
  case "$tag" in A) tag_lower="a" ;; B) tag_lower="b" ;; *) tag_lower="$(echo "$tag" | tr '[:upper:]' '[:lower:]')" ;; esac
  step "Start stack (phase $tag): kernel + agent"
  (
    cd "$KERNEL_ROOT"
    exec env VIT_PROJECT_XML="$KERNEL_WORKSPACE/default_project.xml" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             "$KERNEL_BIN"
  ) > "$WORKDIR/logs/kernel_stdout_${tag}.log" 2>&1 &
  KERNEL_PID=$!
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || {
      tail -30 "$WORKDIR/logs/kernel_stdout_${tag}.log" >&2
      fatal_env "kernel process exited during startup (phase $tag)"
    }
    [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
    sleep 1
  done
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] \
    || fatal_env "kernel ZMQ REQ port $KERNEL_PORT_REQ not listening within ${STARTUP_TIMEOUT_SECONDS}s (phase $tag)"
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    listener="$(port_listener_pid "$port")"
    [[ "$listener" == "$KERNEL_PID" ]] \
      || fatal_env "port $port listener pid $listener != kernel pid $KERNEL_PID (phase $tag)"
  done
  log "kernel up (pid $KERNEL_PID, ports $KERNEL_PORT_REQ/$ZMQ_PUB_PORT/$ZMQ_LOG_PORT)"

  (
    cd "$WORKDIR/agent_cwd"
    exec env VIT_HISTORY_DRAFT_ROOT="$AGENT_DRAFTS" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             VIT_ORCHESTRATION_STORE_PATH="$AGENT_STATE_DIR/orchestration_v1.json" \
             "$AGENT_BIN" \
               -http "$AGENT_HTTP_ADDR" \
               -last-log-path "$phase_dir/agent_last.log" \
               -keep-last-log-lines 8000 \
               -vsp-hub-url ""
  ) > "$WORKDIR/logs/agent_stdout_${tag}.log" 2>&1 &
  AGENT_PID=$!
  deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$AGENT_PID" 2>/dev/null || {
      tail -30 "$WORKDIR/logs/agent_stdout_${tag}.log" >&2
      fatal_env "agent process exited during startup (phase $tag)"
    }
    curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && break
    sleep 1
  done
  curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null \
    || fatal_env "agent HTTP did not become ready within ${STARTUP_TIMEOUT_SECONDS}s (phase $tag)"
  [[ "$(port_listener_pid "$AGENT_HTTP_PORT")" == "$AGENT_PID" ]] \
    || fatal_env "agent HTTP port owner is not the agent pid (phase $tag)"
  prereq "phase_${tag_lower}_kernel_pid=$KERNEL_PID agent_pid=$AGENT_PID"
  log "agent up (pid $AGENT_PID, $AGENT_HTTP)"
}

# ================================================================ PHASE A
start_stack "A" "$WORKDIR/phase_a"

step "PHASE A bootstrap: GET /agent/state"
code="$(http_json GET "$AGENT_HTTP/agent/state" "" "$WORKDIR/phase_a/state_bootstrap.json" 30)" \
  || fatal_env "GET /agent/state failed (bootstrap)"
[[ "$code" =~ ^2 ]] || fatal_env "GET /agent/state returned HTTP $code (bootstrap)"
prereq "phase_a_shadow_initialized=$(json_field "$WORKDIR/phase_a/state_bootstrap.json" 'str(d.get("shadow",{}).get("initialized",""))') track_count=$(json_field "$WORKDIR/phase_a/state_bootstrap.json" 'd.get("shadow",{}).get("track_count","")')"

if (( KERNEL_DWELL_SECONDS > 0 )); then
  step "Kernel warm-up dwell ${KERNEL_DWELL_SECONDS}s"
  sleep "$KERNEL_DWELL_SECONDS"
fi

# ---- S1: project open ------------------------------------------------------
step "PHASE A / S1 project open: open_project via agent tool face"
python3 - "$COPY_PROJECT" > "$WORKDIR/bodies/open_project.json" <<'PY'
import json, sys
print(json.dumps({"tool": "open_project",
                  "command": {"cmd": "open_project", "file_path": sys.argv[1]},
                  "source": "journey1_mac_driver", "confirmed": True}, ensure_ascii=False))
PY
LOAD_STATUS=""
invoke_project_load() {
  # invoke_project_load <out-prefix>: POST open_project, honor requires_confirmation.
  local prefix="$1" requires
  code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/open_project.json" "${prefix}_response.json" 180)" \
    || fatal_env "open_project invoke transport failed"
  [[ "$code" =~ ^2 ]] || fatal_env "open_project invoke returned HTTP $code (see ${prefix}_response.json)"
  requires="$(json_field "${prefix}_response.json" 'str(d.get("requires_confirmation","")).lower()')"
  if [[ "$requires" == "true" ]]; then
    code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/open_project.json" "${prefix}_response_confirmed.json" 180)" \
      || fatal_env "confirmed open_project invoke transport failed"
    [[ "$code" =~ ^2 ]] || fatal_env "confirmed open_project invoke returned HTTP $code"
  fi
}
invoke_project_load "$WORKDIR/phase_a/load_invoke"
LOAD_STATUS="$(json_field "$WORKDIR/phase_a/load_invoke_response.json" 'str(d.get("status",""))')"
# the confirmed variant (if produced) is the operative reply
if [[ -f "$WORKDIR/phase_a/load_invoke_response_confirmed.json" ]]; then
  LOAD_STATUS="$(json_field "$WORKDIR/phase_a/load_invoke_response_confirmed.json" 'str(d.get("status",""))')"
fi
LOAD_COMMAND="$(json_field "$WORKDIR/phase_a/load_invoke_response.json" 'str(d.get("command_name",""))')"
LOAD_ERROR="$(json_field "$WORKDIR/phase_a/load_invoke_response.json" 'str(d.get("error",""))')"
prereq "phase_a_load_status=$LOAD_STATUS command=$LOAD_COMMAND error=$LOAD_ERROR"
[[ "$LOAD_STATUS" == "ok" ]] \
  || fatal_env "project load path failed: status=$LOAD_STATUS error=$LOAD_ERROR (see load_invoke_response.json)"

sleep 6
code="$(http_json GET "$AGENT_HTTP/agent/ui/state" "" "$WORKDIR/phase_a/ui_state_after_load.json" 60)" \
  || fatal_env "GET /agent/ui/state failed after load"
[[ "$code" =~ ^2 ]] || fatal_env "GET /agent/ui/state returned HTTP $code after load"

python3 - "$WORKDIR/phase_a/ui_state_after_load.json" "$BASS_TRACK_NAME" "$WORKDIR/phase_a/step_load.json" <<'PY'
import json, os, sys
ui, bass_name, out = sys.argv[1], sys.argv[2], sys.argv[3]
d = json.load(open(ui, encoding="utf-8"))
tracks = d.get("tracks") or []
if isinstance(tracks, dict):
    tracks = list(tracks.values())
names = []
bass = None
for t in tracks:
    if not isinstance(t, dict):
        continue
    n = str(t.get("name", t.get("track_name", "")))
    if n:
        names.append(n)
    if n == bass_name:
        bass = t
def plugin_count(t):
    if not isinstance(t, dict):
        return 0
    v = t.get("plugin_count")
    if isinstance(v, (int, float)):
        return int(v)
    rows = 0
    for key in ("plugins", "processors", "nodes", "rack_nodes", "effects"):
        val = t.get(key)
        if isinstance(val, list):
            rows += len(val)
        elif isinstance(val, dict):
            rows += len(val)
    return rows
project = d.get("project") or {}
uuid = str(project.get("project_uuid", "")) or str((d.get("project_history") or {}).get("project_uuid", ""))
json.dump({
    "track_names": names,
    "track_count": len(names),
    "bass_present": bass is not None,
    "bass_plugin_count_before": plugin_count(bass),
    "project_uuid": uuid,
    "project_path": str(project.get("project_path", "")),
}, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
TRACK_NAMES="$(json_field "$WORKDIR/phase_a/step_load.json" '",".join(d["track_names"])')"
BASS_PRESENT="$(json_field "$WORKDIR/phase_a/step_load.json" 'd["bass_present"]')"
TRACK_COUNT="$(json_field "$WORKDIR/phase_a/step_load.json" 'd["track_count"]')"
PROJECT_UUID="$(json_field "$WORKDIR/phase_a/step_load.json" 'd["project_uuid"]')"
prereq "phase_a_tracks=$TRACK_NAMES"
prereq "phase_a_bass_present=$BASS_PRESENT project_uuid=$PROJECT_UUID"

python3 - "$COPY_PROJECT" "$WORKDIR/phase_a/sessions_after_load.json" <<'PY' >/dev/null
import json, os, sys
project, out = sys.argv[1], sys.argv[2]
root = os.path.join(os.path.dirname(project), ".vit_history", ".sessions")
rows = []
if os.path.isdir(root):
    for uuid_dir in sorted(os.listdir(root)):
        u = os.path.join(root, uuid_dir)
        if not os.path.isdir(u):
            continue
        for sess in sorted(os.listdir(u)):
            s = os.path.join(u, sess)
            if not os.path.isdir(s):
                continue
            newest, count = 0, 0
            for dirpath, dirnames, filenames in os.walk(s):
                for name in filenames:
                    p = os.path.join(dirpath, name)
                    count += 1
                    newest = max(newest, os.stat(p).st_mtime)
            rows.append({"uuid_dir": uuid_dir, "session": sess, "path": s,
                         "last_write_epoch": newest, "file_count": count,
                         "session_json": os.path.isfile(os.path.join(s, "session.json")),
                         "has_workspace": os.path.isdir(os.path.join(s, "workspace"))})
json.dump(rows, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY

# ---- S2: authority ----------------------------------------------------------
step "PHASE A / S2 authority: full project access"
printf '{"authority_mode":"full_project_access"}' > "$WORKDIR/bodies/authority.json"
code="$(http_json POST "$AGENT_HTTP/agent/authority" "$WORKDIR/bodies/authority.json" "$WORKDIR/phase_a/authority_response.json" 30)" \
  || fatal_env "authority switch transport failed"
[[ "$code" =~ ^2 ]] || fatal_env "authority switch refused HTTP $code: $(head -c 300 "$WORKDIR/phase_a/authority_response.json")"
AUTHORITY_MODE="$(json_field "$WORKDIR/phase_a/authority_response.json" 'str(d.get("authority_mode",""))')"
prereq "phase_a_authority_mode=$AUTHORITY_MODE"
[[ "$AUTHORITY_MODE" == "full_project_access" ]] \
  || fatal_env "authority switch refused: $(cat "$WORKDIR/phase_a/authority_response.json")"

# ---- S3: experiment (real LLM turn) -----------------------------------------
step "PHASE A / S3 experiment: send the demo prompt (real LLM turn, budget ${TURN_BUDGET_SECONDS}s)"
CONVERSATION_A="journey1_mac_a_$(date '+%Y%m%d_%H%M%S')"
prereq "phase_a_conversation_id=$CONVERSATION_A"
: > "$WORKDIR/phase_a/slices.jsonl"
SLICE_INDEX=0
CHAT_TRANSPORT_OK=1
TURN_DEADLINE=$((SECONDS + TURN_BUDGET_SECONDS))
while (( SLICE_INDEX <= MAX_NUDGES && SECONDS < TURN_DEADLINE )); do
  if (( SLICE_INDEX == 0 )); then MESSAGE="$PROMPT_TEXT"; else MESSAGE="$NUDGE_TEXT"; fi
  python3 - "$CONVERSATION_A" "$MESSAGE" > "$WORKDIR/bodies/chat.json" <<'PY'
import json, sys
print(json.dumps({"conversation_id": sys.argv[1], "message": sys.argv[2]}, ensure_ascii=False))
PY
  slice_start=$SECONDS
  code="$(http_json POST "$AGENT_HTTP/agent/chat" "$WORKDIR/bodies/chat.json" "$WORKDIR/phase_a/chat_response_${SLICE_INDEX}.json" 300)" \
    || { CHAT_TRANSPORT_OK=0; break; }
  [[ "$code" =~ ^2 ]] || { CHAT_TRANSPORT_OK=0; break; }
  slice_ms=$(( (SECONDS - slice_start) * 1000 ))
  python3 - "$WORKDIR/phase_a/chat_response_${SLICE_INDEX}.json" "$SLICE_INDEX" "$slice_ms" >> "$WORKDIR/phase_a/slices.jsonl" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
json.dump({"index": int(sys.argv[2]), "goal_status": str(d.get("goal_status", "")),
           "stop_reason": str(d.get("stop_reason", "")), "ms": int(sys.argv[3]),
           "reply": str(d.get("reply", ""))}, sys.stdout, ensure_ascii=False)
print()
PY
  GOAL_STATUS="$(json_field "$WORKDIR/phase_a/chat_response_${SLICE_INDEX}.json" 'str(d.get("goal_status",""))')"
  STOP_REASON="$(json_field "$WORKDIR/phase_a/chat_response_${SLICE_INDEX}.json" 'str(d.get("stop_reason",""))')"
  prereq "phase_a_slice_${SLICE_INDEX}_status=$GOAL_STATUS stop=$STOP_REASON"
  ok "slice $SLICE_INDEX: goal_status=$GOAL_STATUS stop=$STOP_REASON"
  if [[ "$GOAL_STATUS" == "waiting_continue" ]]; then
    poll_deadline=$((SECONDS + (TURN_BUDGET_SECONDS < 120 ? TURN_BUDGET_SECONDS : 120)))
    (( poll_deadline > TURN_DEADLINE )) && poll_deadline=$TURN_DEADLINE
    while (( SECONDS < poll_deadline && SECONDS < TURN_DEADLINE )); do
      sleep "$POLL_SECONDS"
      code="$(http_json GET "$AGENT_HTTP/agent/runtime/status" "" "$WORKDIR/bodies/runtime_poll.json" 30)" || { sleep 1; continue; }
      [[ "$code" =~ ^2 ]] || { sleep 1; continue; }
      polled="$(json_field "$WORKDIR/bodies/runtime_poll.json" 'str(d.get("goal",{}).get("status",""))')"
      info "poll goal=$polled"
      case "$polled" in
        completed|failed|stopped|cancelled) break ;;
      esac
    done
  fi
  case "$GOAL_STATUS" in
    waiting_clarification|waiting_confirmation) SLICE_INDEX=$((SLICE_INDEX + 1)); continue ;;
  esac
  break
done
[[ "$CHAT_TRANSPORT_OK" -eq 1 ]] || fatal_env "chat slice $SLICE_INDEX transport/HTTP failure (see phase_a/chat_response_${SLICE_INDEX}.json)"
cp "$WORKDIR/phase_a/chat_response_${SLICE_INDEX}.json" "$WORKDIR/phase_a/chat_response_final.json" 2>/dev/null || true
sleep 5
code="$(http_json GET "$AGENT_HTTP/agent/runtime/status" "" "$WORKDIR/phase_a/runtime_status.json" 30)" || true
[[ "${code:-}" =~ ^2 ]] || info "runtime status fetch after turn failed (non-fatal)"

step "PHASE A evidence capture (ui state / events / actions / runtime state)"
code="$(http_json GET "$AGENT_HTTP/agent/ui/state" "" "$WORKDIR/phase_a/ui_state_after_turn.json" 60)" || true
[[ "${code:-}" =~ ^2 ]] || bad "GET /agent/ui/state after turn failed"
code="$(http_json GET "$AGENT_HTTP/agent/events?conversation_id=${CONVERSATION_A}&since=0&limit=500" "" "$WORKDIR/phase_a/events.json" 60)" || true
[[ "${code:-}" =~ ^2 ]] || bad "GET /agent/events failed"
code="$(http_json GET "$AGENT_HTTP/agent/actions" "" "$WORKDIR/phase_a/actions.json" 60)" || true
[[ "${code:-}" =~ ^2 ]] || bad "GET /agent/actions failed"

RUNTIME_STATE_PATH="$(find "$AGENT_DRAFTS" "$(dirname "$COPY_PROJECT")" -name agent_runtime_state.json -type f -print0 2>/dev/null | xargs -0 stat -f '%m %N' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)"
if [[ -n "$RUNTIME_STATE_PATH" && -f "$RUNTIME_STATE_PATH" ]]; then
  cp "$RUNTIME_STATE_PATH" "$WORKDIR/phase_a/agent_runtime_state_snapshot.json"
  prereq "phase_a_runtime_state_path=$RUNTIME_STATE_PATH"
fi
cp "$WORKDIR/phase_a/agent_last.log" "$WORKDIR/phase_a/agent_last.phase_a.log" 2>/dev/null || true

# A1 analysis: no direction asking in the text the user actually receives.
python3 - "$WORKDIR/phase_a/slices.jsonl" "$WORKDIR/phase_a/events.json" "$PLACEHOLDER_TEXT" "$ASK_WORD_OR" "$FULLWIDTH_QUESTION" "${ASK_WORDS[@]}" > /dev/null <<'PY'
import json, re, sys

slices_path, events_path = sys.argv[1], sys.argv[2]
placeholder, ask_or, fullwidth_q = sys.argv[3], sys.argv[4], sys.argv[5]
ask_words = sys.argv[6:]

delivered = []
try:
    with open(slices_path, encoding="utf-8") as f:
        for line in f:
            if not line.strip():
                continue
            s = json.loads(line)
            text = s.get("reply", "")
            if text.strip():
                delivered.append({"source": "chat_slice_%d" % s["index"], "text": text,
                                  "placeholder": placeholder in text})
except FileNotFoundError:
    pass
try:
    events = json.load(open(events_path, encoding="utf-8"))
    for ev in (events.get("events") or []):
        parts = [str(ev.get(k, "")) for k in ("title", "body", "summary") if str(ev.get(k, "")).strip()]
        if parts:
            delivered.append({"source": "event_%s_%s" % (ev.get("type", ""), ev.get("seq", "")),
                              "text": " | ".join(parts), "placeholder": False})
except FileNotFoundError:
    pass

substantive = [row for row in delivered if not row["placeholder"]]
option_marker = re.compile(r"(?:^|[\s;:.,()\[\]])([A-D])[.\u3001\uFF0E)\uFF09]")
option_marker_count = 0
ask_hits = []
has_question_mark = False
for row in substantive:
    text = row["text"]
    option_marker_count += len(option_marker.findall(text))
    for word in ask_words:
        if word in text and word not in ask_hits:
            ask_hits.append(word)
    if fullwidth_q in text or "?" in text:
        has_question_mark = True
ask_or_hit = any(ask_or in row["text"] for row in substantive)
a1_red = (option_marker_count >= 2) or (has_question_mark and (len(ask_hits) >= 1 or ask_or_hit))
if a1_red:
    state = "red"
elif not substantive:
    state = "not_observable"
else:
    state = "green"
first_reply = ""
try:
    with open(slices_path, encoding="utf-8") as f:
        for line in f:
            if line.strip():
                first_reply = json.loads(line).get("reply", "")
                break
except FileNotFoundError:
    pass
json.dump({
    "a1_first_reply": first_reply,
    "a1_first_reply_is_placeholder": placeholder in first_reply,
    "a1_delivered_texts": delivered,
    "a1_substantive_text_count": len(substantive),
    "a1_option_marker_count": option_marker_count,
    "a1_ask_word_hits": ask_hits,
    "a1_question_mark": has_question_mark,
    "a1_ask_or_hit": ask_or_hit,
    "a1_state": state,
}, open(sys.argv[1].replace("slices.jsonl", "") + "a1_analysis.json", "w", encoding="utf-8"),
   ensure_ascii=False, indent=2)
PY
A1_STATE="$(json_field "$WORKDIR/phase_a/a1_analysis.json" 'd["a1_state"]')"
SUBSTANTIVE_COUNT="$(json_field "$WORKDIR/phase_a/a1_analysis.json" 'd["a1_substantive_text_count"]')"
prereq "a1_state=$A1_STATE substantive_texts=$SUBSTANTIVE_COUNT option_markers=$(json_field "$WORKDIR/phase_a/a1_analysis.json" 'd["a1_option_marker_count"]') ask_words=$(json_field "$WORKDIR/phase_a/a1_analysis.json" '",".join(d["a1_ask_word_hits"])') question_mark=$(json_field "$WORKDIR/phase_a/a1_analysis.json" 'd["a1_question_mark"]')"

# ---- S4: load (deterministic probe, ps1 A2) ---------------------------------
step "PHASE A / S4 load: deterministic rack_add_node probe ($PROBE_PLUGIN_IDENTIFIER)"
GATE_PATTERN="stage=pca_processor_load_gate"
count_gate() {
  # count_gate <log> — occurrences of the gate stage line; 0 on any miss.
  local n
  n="$(grep -c "$GATE_PATTERN" "$1" 2>/dev/null)"
  echo "${n:-0}"
}
GATE_DENIED_JOURNEY="$(count_gate "$WORKDIR/phase_a/agent_last.log")"
prereq "a2_pca_load_gate_denials_journey_turn=$GATE_DENIED_JOURNEY"

BASS_TRACK_ID="$(python3 - "$WORKDIR/phase_a/ui_state_after_turn.json" "$BASS_TRACK_NAME" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
tracks = d.get("tracks") or []
if isinstance(tracks, dict):
    tracks = list(tracks.values())
for t in tracks:
    if isinstance(t, dict) and str(t.get("name", t.get("track_name", ""))) == sys.argv[2]:
        print(str(t.get("track_id", t.get("id", ""))))
        break
PY
)"
PROBE_BEFORE="$(count_gate "$WORKDIR/phase_a/agent_last.log")"
PROBE_STATUS=""
PROBE_ERROR_BODY=""
if [[ -n "$BASS_TRACK_ID" ]]; then
  python3 - "$PROBE_PLUGIN_IDENTIFIER" "$BASS_TRACK_ID" "$ZONE_ID" > "$WORKDIR/bodies/probe.json" <<'PY'
import json, sys
print(json.dumps({"tool": "rack_add_node",
                  "command": {"cmd": "rack_add_node", "plugin_identifier": sys.argv[1],
                              "track_id": sys.argv[2], "x": 0, "y": 0, "zone_id": sys.argv[3]},
                  "source": "journey1_mac_driver", "confirmed": True}, ensure_ascii=False))
PY
  if code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/probe.json" "$WORKDIR/phase_a/a2_direct_rack_add_node_response.json" 180)"; then
    [[ "$code" =~ ^2 ]] || PROBE_ERROR_BODY="$(head -c 4000 "$WORKDIR/phase_a/a2_direct_rack_add_node_response.json")"
  else
    PROBE_ERROR_BODY="transport failure"
  fi
  [[ -n "$PROBE_ERROR_BODY" ]] && printf '%s' "$PROBE_ERROR_BODY" > "$WORKDIR/phase_a/a2_direct_rack_add_node_error.json"
  PROBE_STATUS="$(json_field "$WORKDIR/phase_a/a2_direct_rack_add_node_response.json" 'str(d.get("status",""))' 2>/dev/null || echo "")"
else
  bad "bass track id not resolvable from ui_state_after_turn — probe cannot run"
fi
PROBE_AFTER="$(count_gate "$WORKDIR/phase_a/agent_last.log")"
python3 - "$WORKDIR/phase_a" "$PROBE_STATUS" "$PROBE_ERROR_BODY" "$PROBE_PLUGIN_IDENTIFIER" "$BASS_TRACK_ID" "$PROBE_BEFORE" "$PROBE_AFTER" "$GATE_DENIED_JOURNEY" "$BASS_TRACK_NAME" > /dev/null <<'PY'
import json, os, sys
phase_dir, status, error_body, identifier, track_id, before, after, journey_denials, bass_name = sys.argv[1:11]
before, after = int(before), int(after)
plugin_id, instance_ready, graph_diff = "", False, ""
try:
    d = json.load(open(os.path.join(phase_dir, "a2_direct_rack_add_node_response.json"), encoding="utf-8"))
    result = d.get("result") or {}
    plugin_id = str(result.get("plugin_id", ""))
    instance_ready = bool(result.get("plugin_instance_ready", False))
    diff = [str(result.get(k, "")) for k in ("graph_last_diff_kind", "graph_last_diff_summary") if str(result.get(k, "")).strip()]
    graph_diff = "|".join(diff)
except Exception:
    pass
denied = (after > before) or (status == "error")
landed = bool(plugin_id) and instance_ready

# journey-turn half (model-branch-dependent, reported separately — ps1
# a2_journey_turn_plugin_landed): bass plugin growth or rack rows after the
# turn, read from the pre-probe ui state.
journey_bass_before = 0
journey_bass_after = 0
journey_rack_rows = 0
try:
    ui = json.load(open(os.path.join(phase_dir, "ui_state_after_turn.json"), encoding="utf-8"))
    load = json.load(open(os.path.join(phase_dir, "step_load.json"), encoding="utf-8"))
    journey_bass_before = int(load.get("bass_plugin_count_before", 0))
    tracks = ui.get("tracks") or []
    if isinstance(tracks, dict):
        tracks = list(tracks.values())
    bass = None
    for t in tracks:
        if isinstance(t, dict) and str(t.get("name", t.get("track_name", ""))) == bass_name:
            bass = t
            break
    def plugin_count(t):
        if not isinstance(t, dict):
            return 0
        v = t.get("plugin_count")
        if isinstance(v, (int, float)):
            return int(v)
        rows = 0
        for key in ("plugins", "processors", "nodes", "rack_nodes", "effects"):
            val = t.get(key)
            if isinstance(val, (list, dict)):
                rows += len(val)
        return rows
    journey_bass_after = plugin_count(bass)
    rack = ui.get("plugin_rack") or {}
    plugins = rack.get("plugins")
    journey_rack_rows = len(plugins) if isinstance(plugins, (list, dict)) else 0
except Exception:
    pass
json.dump({
    "a2_direct_probe_track_id": track_id,
    "a2_direct_probe_plugin": identifier,
    "a2_direct_probe_status": status,
    "a2_direct_probe_error_body": error_body[:2000],
    "a2_direct_probe_plugin_id": plugin_id,
    "a2_direct_probe_plugin_instance_ready": instance_ready,
    "a2_direct_probe_graph_diff": graph_diff,
    "a2_direct_probe_gate_denial_delta": after - before,
    "a2_direct_probe_denied": denied,
    "a2_direct_probe_plugin_landed": landed,
    "a2_pca_load_gate_denials_journey_turn": int(journey_denials),
    "a2_bass_plugin_count_before_turn": journey_bass_before,
    "a2_bass_plugin_count_after_turn": journey_bass_after,
    "a2_plugin_rack_plugin_rows_after_turn": journey_rack_rows,
    "a2_journey_turn_plugin_landed": journey_bass_after > journey_bass_before or journey_rack_rows > 0,
}, open(os.path.join(phase_dir, "a2_probe.json"), "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
PROBE_DENIED="$(json_field "$WORKDIR/phase_a/a2_probe.json" 'd["a2_direct_probe_denied"]')"
PROBE_LANDED="$(json_field "$WORKDIR/phase_a/a2_probe.json" 'd["a2_direct_probe_plugin_landed"]')"
prereq "a2_direct_probe_landed=$PROBE_LANDED plugin_id=$(json_field "$WORKDIR/phase_a/a2_probe.json" 'd["a2_direct_probe_plugin_id"]') instance_ready=$(json_field "$WORKDIR/phase_a/a2_probe.json" 'd["a2_direct_probe_plugin_instance_ready"]') denied=$PROBE_DENIED gate_delta=$(json_field "$WORKDIR/phase_a/a2_probe.json" 'd["a2_direct_probe_gate_denial_delta"]')"

# post-probe UI projection (UI-PLUGIN-COUNT-1 semantics): select the bass track
# through /agent/ui/context (the endpoint the Godot frontend uses), then read
# the projection.
printf '{"selected_track_name":"%s"}' "$BASS_TRACK_NAME" > "$WORKDIR/bodies/uicontext.json"
code="$(http_json POST "$AGENT_HTTP/agent/ui/context" "$WORKDIR/bodies/uicontext.json" "$WORKDIR/bodies/uicontext_reply.json" 30)" || true
code="$(http_json GET "$AGENT_HTTP/agent/ui/state" "" "$WORKDIR/phase_a/ui_state_post_probe.json" 60)" || true
python3 - "$WORKDIR/phase_a/ui_state_post_probe.json" "$BASS_TRACK_NAME" > /dev/null <<'PY'
import json, os, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
tracks = d.get("tracks") or []
if isinstance(tracks, dict):
    tracks = list(tracks.values())
bass = None
for t in tracks:
    if isinstance(t, dict) and str(t.get("name", t.get("track_name", ""))) == sys.argv[2]:
        bass = t
        break
def plugin_count(t):
    if not isinstance(t, dict):
        return 0
    v = t.get("plugin_count")
    if isinstance(v, (int, float)):
        return int(v)
    rows = 0
    for key in ("plugins", "processors", "nodes", "rack_nodes", "effects"):
        val = t.get(key)
        if isinstance(val, (list, dict)):
            rows += len(val)
    return rows
rack = d.get("plugin_rack") or {}
rack_plugins = len(rack.get("plugins") or []) if isinstance(rack.get("plugins"), (list, dict)) else 0
rack_rows = len(rack.get("rack") or []) if isinstance(rack.get("rack"), (list, dict)) else 0
bass_count = plugin_count(bass)
out_path = os.path.join(os.path.dirname(os.path.abspath(sys.argv[1])), "a2_projection.json")
json.dump({
    "a2_bass_plugin_count_after": bass_count,
    "a2_plugin_rack_plugin_rows": rack_plugins,
    "a2_plugin_rack_rows": rack_rows,
    "a2_ui_projection_landed": bass_count >= 1 and rack_plugins >= 1,
}, open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
A2_PROJECTION="$(json_field "$WORKDIR/phase_a/a2_projection.json" 'd["a2_ui_projection_landed"]')"
prereq "a2_ui_projection_post_probe bass_count=$(json_field "$WORKDIR/phase_a/a2_projection.json" 'd["a2_bass_plugin_count_after"]') plugin_rows=$(json_field "$WORKDIR/phase_a/a2_projection.json" 'd["a2_plugin_rack_plugin_rows"]') landed=$A2_PROJECTION"
grep "$GATE_PATTERN" "$WORKDIR/phase_a/agent_last.log" 2>/dev/null | sed 's/^[[:space:]]*//' > "$WORKDIR/phase_a/a2_pca_load_gate_log_lines.txt" || true

# A2 verdict: deterministic half carries the assertion (ps1 semantics — the
# journey turn is model-branch-dependent and stays reported separately).
if [[ "$PROBE_DENIED" == "True" ]] || [[ "$PROBE_LANDED" != "True" ]] || { [[ "$PROBE_LANDED" == "True" ]] && [[ "$A2_PROJECTION" != "True" ]]; }; then
  A2_RED=1
else
  A2_RED=0
fi
prereq "a2_state=$( (( A2_RED )) && echo RED || echo GREEN ) probe_denied=$PROBE_DENIED probe_landed=$PROBE_LANDED ui_projection=$A2_PROJECTION"

# ---- S5: audition (ps1 A3) ---------------------------------------------------
step "PHASE A / S5 audition: A/B card on the conversation surface"
python3 - "$WORKDIR/phase_a/events.json" "$WORKDIR/phase_a/actions.json" > /dev/null <<'PY'
import json, sys
event_types = []
try:
    events = json.load(open(sys.argv[1], encoding="utf-8"))
    event_types = [str(e.get("type", "")) for e in (events.get("events") or []) if str(e.get("type", "")).strip()]
except Exception:
    pass
audition_events = sorted({t for t in event_types if t.startswith("audition.")})
mix_tick_events = sorted({t for t in event_types if "mix_tick" in t})
def raw(path):
    try:
        return open(path, encoding="utf-8").read()
    except Exception:
        return ""
audition_session_seen = ("audition_session_id" in raw(sys.argv[1])) or ("audition_session_id" in raw(sys.argv[2]))
proposal_applied = any(("intervention" in t or "applied" in t) for t in event_types)
card_mounted = bool(audition_events) or audition_session_seen or bool(mix_tick_events)
json.dump({
    "a3_event_types": event_types,
    "a3_audition_events": audition_events,
    "a3_mix_tick_events": mix_tick_events,
    "a3_audition_session_seen": audition_session_seen,
    "a3_proposal_applied_signal": proposal_applied,
    "a3_audition_card_mounted": card_mounted,
}, open(sys.argv[2].replace("actions.json", "") + "a3_audition.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
A3_MOUNTED="$(json_field "$WORKDIR/phase_a/a3_audition.json" 'd["a3_audition_card_mounted"]')"
prereq "a3_audition_events=$(json_field "$WORKDIR/phase_a/a3_audition.json" '",".join(d["a3_audition_events"])') mix_tick=$(json_field "$WORKDIR/phase_a/a3_audition.json" '",".join(d["a3_mix_tick_events"])') session_seen=$(json_field "$WORKDIR/phase_a/a3_audition.json" 'd["a3_audition_session_seen"]') card_mounted=$A3_MOUNTED"

# ---- save point (ps1 A step 4; the A4 contract needs a real manual save) ----
step "PHASE A save point: manual save (prepare -> kernel save -> adopt generation)"
SAVE_PREPARE_ID=""
SAVE_GENERATION_ID=""
SAVE_PREPARE_STATUS="none"
SAVE_PROJECT_STATUS="none"
SAVE_FINAL_STATUS="none"
save_invoke() {
  # save_invoke <tool> <json-body-file> <out-file>: POST and record status.
  local tool="$1" body="$2" out="$3" code status
  if code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$body" "$out" 300)"; then
    [[ "$code" =~ ^2 ]] || status="http_$code" 
  else
    status="transport_failed"
  fi
  status="${status:-$(json_field "$out" 'str(d.get("status",""))' 2>/dev/null || echo unknown)}"
  printf '%s' "$status"
}
python3 - "$COPY_PROJECT" "$PROJECT_UUID" > "$WORKDIR/bodies/save_prepare.json" <<'PY'
import json, sys
print(json.dumps({"tool": "version_project_save_prepare",
                  "command": {"cmd": "version_project_save_prepare", "project_path": sys.argv[1],
                              "project_uuid": sys.argv[2], "save_kind": "manual"},
                  "source": "journey1_mac_driver", "confirmed": True}, ensure_ascii=False))
PY
SAVE_PREPARE_STATUS="$(save_invoke version_project_save_prepare "$WORKDIR/bodies/save_prepare.json" "$WORKDIR/phase_a/save_prepare_response.json")"
SAVE_PREPARE_ID="$(json_field "$WORKDIR/phase_a/save_prepare_response.json" 'str(d.get("result",{}).get("prepare_id",""))' 2>/dev/null || echo "")"
SAVE_GENERATION_ID="$(json_field "$WORKDIR/phase_a/save_prepare_response.json" 'str(d.get("result",{}).get("agent_history_generation",""))' 2>/dev/null || echo "")"
prereq "phase_a_save_prepare status=$SAVE_PREPARE_STATUS prepare_id=$SAVE_PREPARE_ID generation=$SAVE_GENERATION_ID"
if [[ -n "$SAVE_PREPARE_ID" ]]; then
  python3 - "$COPY_PROJECT" "$SAVE_PREPARE_ID" "$SAVE_GENERATION_ID" > "$WORKDIR/bodies/save_project.json" <<'PY'
import json, sys
print(json.dumps({"tool": "save_project",
                  "command": {"cmd": "save_project", "file_path": sys.argv[1],
                              "history_prepare_id": sys.argv[2], "agent_history_generation": sys.argv[3]},
                  "source": "journey1_mac_driver", "confirmed": True}, ensure_ascii=False))
PY
  SAVE_PROJECT_STATUS="$(save_invoke save_project "$WORKDIR/bodies/save_project.json" "$WORKDIR/phase_a/save_project_response.json")"
  prereq "phase_a_save_project status=$SAVE_PROJECT_STATUS"
  python3 - "$COPY_PROJECT" "$PROJECT_UUID" "$SAVE_PREPARE_ID" "$SAVE_GENERATION_ID" > "$WORKDIR/bodies/save_saved.json" <<'PY'
import json, sys
print(json.dumps({"tool": "version_project_saved",
                  "command": {"cmd": "version_project_saved", "project_path": sys.argv[1],
                              "project_uuid": sys.argv[2], "history_prepare_id": sys.argv[3],
                              "agent_history_generation": sys.argv[4], "save_kind": "manual"},
                  "source": "journey1_mac_driver", "confirmed": True}, ensure_ascii=False))
PY
  SAVE_FINAL_STATUS="$(save_invoke version_project_saved "$WORKDIR/bodies/save_saved.json" "$WORKDIR/phase_a/version_project_saved_response.json")"
  prereq "phase_a_version_project_saved status=$SAVE_FINAL_STATUS"
fi
prereq "phase_a_copy_project_sha256_after_save=$(file_sha256 "$COPY_PROJECT")"

python3 - "$COPY_PROJECT" "$WORKDIR/phase_a/sessions_after_turn.json" <<'PY' >/dev/null
import json, os, sys
project, out = sys.argv[1], sys.argv[2]
root = os.path.join(os.path.dirname(project), ".vit_history", ".sessions")
rows = []
if os.path.isdir(root):
    for uuid_dir in sorted(os.listdir(root)):
        u = os.path.join(root, uuid_dir)
        if not os.path.isdir(u):
            continue
        for sess in sorted(os.listdir(u)):
            s = os.path.join(u, sess)
            if not os.path.isdir(s):
                continue
            newest, count = 0, 0
            for dirpath, dirnames, filenames in os.walk(s):
                for name in filenames:
                    p = os.path.join(dirpath, name)
                    count += 1
                    newest = max(newest, os.stat(p).st_mtime)
            rows.append({"uuid_dir": uuid_dir, "session": sess, "path": s,
                         "last_write_epoch": newest, "file_count": count,
                         "session_json": os.path.isfile(os.path.join(s, "session.json")),
                         "has_workspace": os.path.isdir(os.path.join(s, "workspace"))})
json.dump(rows, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY

# ================================================================ PHASE B
A4_STATE="not_observable"
PHASE_B_JSON_PATH="$WORKDIR/phase_b/reopen.json"
if [[ "$SKIP_REOPEN" -eq 1 ]]; then
  prereq "phase_b=skipped (--skip-reopen)"
else
  step "PHASE B: teardown, then reopen the same copy project (session-cleanliness control)"
  stop_stack "A"
  python3 - "$COPY_PROJECT" "$WORKDIR/phase_b/sessions_before_reopen.json" <<'PY' >/dev/null
import json, os, sys
project, out = sys.argv[1], sys.argv[2]
root = os.path.join(os.path.dirname(project), ".vit_history", ".sessions")
rows = []
if os.path.isdir(root):
    for uuid_dir in sorted(os.listdir(root)):
        u = os.path.join(root, uuid_dir)
        if not os.path.isdir(u):
            continue
        for sess in sorted(os.listdir(u)):
            s = os.path.join(u, sess)
            if not os.path.isdir(s):
                continue
            newest, count = 0, 0
            for dirpath, dirnames, filenames in os.walk(s):
                for name in filenames:
                    p = os.path.join(dirpath, name)
                    count += 1
                    newest = max(newest, os.stat(p).st_mtime)
            rows.append({"uuid_dir": uuid_dir, "session": sess, "path": s,
                         "last_write_epoch": newest, "file_count": count,
                         "session_json": os.path.isfile(os.path.join(s, "session.json")),
                         "has_workspace": os.path.isdir(os.path.join(s, "workspace"))})
json.dump(rows, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
  REOPEN_EPOCH="$(date +%s)"
  start_stack "B" "$WORKDIR/phase_b"
  invoke_project_load "$WORKDIR/phase_b/load_invoke"
  LOAD_STATUS_B="$(json_field "$WORKDIR/phase_b/load_invoke_response.json" 'str(d.get("status",""))')"
  if [[ -f "$WORKDIR/phase_b/load_invoke_response_confirmed.json" ]]; then
    LOAD_STATUS_B="$(json_field "$WORKDIR/phase_b/load_invoke_response_confirmed.json" 'str(d.get("status",""))')"
  fi
  prereq "phase_b_load_status=$LOAD_STATUS_B"
  [[ "$LOAD_STATUS_B" == "ok" ]] || fatal_env "phase B project load failed (see phase_b/load_invoke_response.json)"

  step "PHASE B dwell ${REOPEN_DWELL_SECONDS}s so the reopen path can restore or not restore"
  sleep "$REOPEN_DWELL_SECONDS"
  code="$(http_json GET "$AGENT_HTTP/agent/ui/state" "" "$WORKDIR/phase_b/ui_state_after_reopen.json" 60)" || true
  code="$(http_json GET "$AGENT_HTTP/agent/state?detail=full" "" "$WORKDIR/phase_b/state_after_reopen_full.json" 60)" || true
  code="$(http_json GET "$AGENT_HTTP/agent/events?conversation_id=${CONVERSATION_A}&since=0&limit=500" "" "$WORKDIR/phase_b/events_for_phase_a_conversation.json" 60)" || true

  python3 - "$COPY_PROJECT" "$REOPEN_EPOCH" "$WORKDIR/phase_b/sessions_before_reopen.json" \
    "$WORKDIR/phase_b/state_after_reopen_full.json" "$WORKDIR/phase_b/events_for_phase_a_conversation.json" \
    "$CONVERSATION_A" "$WORKDIR/phase_b/agent_last.log" "$PHASE_B_JSON_PATH" <<'PY'
import datetime, json, os, re, sys

project, reopen_epoch, before_path, state_path, events_path, conv_id, agent_log, out = sys.argv[1:9]
reopen_epoch = int(reopen_epoch)

def sessions(project):
    root = os.path.join(os.path.dirname(project), ".vit_history", ".sessions")
    rows = []
    if os.path.isdir(root):
        for uuid_dir in sorted(os.listdir(root)):
            u = os.path.join(root, uuid_dir)
            if not os.path.isdir(u):
                continue
            for sess in sorted(os.listdir(u)):
                s = os.path.join(u, sess)
                if not os.path.isdir(s):
                    continue
                newest, count = 0, 0
                for dirpath, dirnames, filenames in os.walk(s):
                    for name in filenames:
                        p = os.path.join(dirpath, name)
                        count += 1
                        newest = max(newest, os.stat(p).st_mtime)
                rows.append({"uuid_dir": uuid_dir, "session": sess, "path": s,
                             "last_write_epoch": newest, "file_count": count})
    return rows

after = sessions(project)
try:
    before = json.load(open(before_path, encoding="utf-8"))
except Exception:
    before = []
before_names = [r["session"] for r in before if r.get("session")]

resumed, fresh, rewritten = [], [], []
for row in after:
    if row["last_write_epoch"] < reopen_epoch:
        continue
    if row["session"] in before_names:
        resumed.append(row["session"])
        for dirpath, dirnames, filenames in os.walk(row["path"]):
            for name in filenames:
                p = os.path.join(dirpath, name)
                st = os.stat(p)
                if st.st_mtime >= reopen_epoch:
                    rewritten.append({"session": row["session"], "file": p[len(row["path"]):],
                                      "bytes": st.st_size,
                                      "written": datetime.datetime.utcfromtimestamp(st.st_mtime).isoformat() + "Z"})
    else:
        fresh.append(row["session"])

try:
    state = json.load(open(state_path, encoding="utf-8"))
    state_text = json.dumps(state, ensure_ascii=False)
except Exception:
    state, state_text = {}, ""
conv_in_state = conv_id in state_text
ph = state.get("project_history") or {}
graph = ph.get("conversation_graph") or {}
nodes = graph.get("nodes") or []
messages = ph.get("conversation_messages") or []
history_dir = str(ph.get("history_dir", ""))
commit_count = ph.get("commit_count")

stall_warn = 0
try:
    with open(agent_log, encoding="utf-8") as f:
        stall_warn = sum(1 for line in f if re.search(r"\[continuation\.stall\]", line))
except FileNotFoundError:
    pass

carried = bool(nodes) or bool(messages)
a4_red = carried or stall_warn > 0
json.dump({
    "reopen_started_epoch": reopen_epoch,
    "resumed_session_dirs": resumed,
    "fresh_session_dirs": fresh,
    "phase_a_session_files_rewritten_on_reopen": rewritten,
    "a4_stall_warn_count": stall_warn,
    "a4_phase_a_conversation_id_in_state": conv_in_state,
    "a4_live_history_dir": history_dir,
    "a4_live_session_name": os.path.basename(os.path.dirname(history_dir)) if history_dir else "",
    "a4_live_conversation_graph_nodes": len(nodes) if isinstance(nodes, list) else 0,
    "a4_live_conversation_messages": len(messages) if isinstance(messages, list) else 0,
    "a4_live_commit_count": commit_count,
    "a4_pre_savepoint_history_carried": carried,
    "a4_state": "red" if a4_red else "green",
}, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
PY
  A4_STATE="$(json_field "$PHASE_B_JSON_PATH" 'd["a4_state"]')"
  prereq "a4_state=$A4_STATE live_session=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_live_session_name"]') graph_nodes=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_live_conversation_graph_nodes"]') messages=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_live_conversation_messages"]') commit_count=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_live_commit_count"]') pre_savepoint_carried=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_pre_savepoint_history_carried"]') stall_warn=$(json_field "$PHASE_B_JSON_PATH" 'd["a4_stall_warn_count"]') resumed=$(json_field "$PHASE_B_JSON_PATH" '",".join(d["resumed_session_dirs"])') fresh=$(json_field "$PHASE_B_JSON_PATH" '",".join(d["fresh_session_dirs"])')"
fi

# ================================================================ assertions
step "Assertions assembly"
python3 - "$WORKDIR" "$A1_STATE" "$SUBSTANTIVE_COUNT" "$A2_RED" "$A3_MOUNTED" "$A4_STATE" \
  "$TRACK_COUNT" "$BASS_PRESENT" "$AUTHORITY_MODE" > "$WORKDIR/assertions_summary.txt" <<'PY'
import json, sys
workdir = sys.argv[1]
a1_state, substantive, a2_red, a3_mounted, a4_state = sys.argv[2:7]
track_count, bass_present, authority_mode = sys.argv[7:10]
a2_red = a2_red == "1"
a3_mounted = a3_mounted == "True"
bass_present = bass_present == "True"

def state_of(flag):
    return "green" if flag else "red"

# five explicit segments (card PORT-JOURNEY-1-MAC framing)
s1 = state_of(int(track_count) > 0 and bass_present)
s2 = state_of(authority_mode == "full_project_access")
if a1_state == "red":
    s3 = "red"
elif a1_state == "not_observable" or int(substantive) == 0:
    s3 = "not_observable"
else:
    s3 = "green"
s4 = state_of(not a2_red)
s5 = state_of(a3_mounted)

a2_state = "red" if a2_red else "green"
a3_state = "red" if not a3_mounted else "green"

assertions = {
    "a1_no_direction_asking": {"state": a1_state,
        "basis": "delivered text (chat slices + event title/body): option-marker count / ask vocabulary"},
    "a2_load_path_works": {"state": a2_state,
        "basis": "deterministic half: the direct rack_add_node probe is not denied by the PCA load gate AND the Kernel's own reply carries a ready plugin instance (plugin_id + plugin_instance_ready) AND the UI projection shows the plugin; journey-turn half reported separately (a2_pca_load_gate_denials_journey_turn)"},
    "a3_experiment_chain": {"state": a3_state,
        "basis": "proposal -> applied -> readback -> mix_tick/audition card on the conversation event stream"},
    "a4_session_clean": {"state": a4_state,
        "basis": "phase B reopen after a phase A save point: the live reopened conversation must not carry pre-save-point history (graph nodes / messages) and no continuation-stall WARN"},
}
five_segments = {
    "s1_project_open": {"state": s1,
        "basis": "open_project via /agent/invoke -> status=ok (env gate) + /agent/ui/state track_count>0 + bass track present"},
    "s2_authority": {"state": s2,
        "basis": "/agent/authority echoes full_project_access"},
    "s3_experiment": {"state": s3,
        "basis": "chat chain delivered substantive text AND a1 (no direction asking) green"},
    "s4_load": {"state": s4,
        "basis": "promoted-processor rack_add_node probe lands + UI projection (ps1 A2)"},
    "s5_audition": {"state": s5,
        "basis": "audition.*/mix_tick event or audition_session_id on the conversation surface (ps1 A3)"},
}
all_states = [v["state"] for v in assertions.values()] + list(five_segments.values())
red = sum(1 for s in all_states if s == "red")
not_obs = sum(1 for s in all_states if s == "not_observable")
json.dump({"assertions": assertions, "five_segments": five_segments,
           "red_count": red, "not_observable_count": not_obs},
          open(workdir + "/assertions.json", "w", encoding="utf-8"), ensure_ascii=False, indent=2)
for name, row in list(assertions.items()) + list(five_segments.items()):
    print("%-24s %s" % (name, row["state"]))
PY
RED_COUNT="$(json_field "$WORKDIR/assertions.json" 'd["red_count"]')"
NOTOBS_COUNT="$(json_field "$WORKDIR/assertions.json" 'd["not_observable_count"]')"
if (( RED_COUNT == 0 && NOTOBS_COUNT == 0 )); then
  OUTCOME="all_green"
elif (( RED_COUNT == 0 )); then
  OUTCOME="journey_inconclusive"
else
  OUTCOME="assertions_red"
fi
prereq "verdict=$OUTCOME red=$RED_COUNT not_observable=$NOTOBS_COUNT"
cat "$WORKDIR/assertions_summary.txt" >&2 || true

stop_stack "journey_end"
exit 0
