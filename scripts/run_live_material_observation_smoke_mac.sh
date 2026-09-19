#!/usr/bin/env bash
# run_live_material_observation_smoke mac equivalent — PORT-SMOKE-MAC-1
# (script 6/7).
#
# Mac port of scripts/run_live_material_observation_smoke.ps1: import at
# least two real audio materials through the agent tool face, then drive the
# deterministic mix.observe tool chain (NO LLM chat turns): immediate
# acoustic-package lifecycle capture, retrying full-project observation until
# every imported track reports ready acoustics, MOM v1.4 multitrack
# projection shape, feature-snapshot readiness reasons, L3 coverage
# non-regression, and the loudness/peak/headroom rankings.
#
#   | ps1 (run_live_material_observation_       | mac (this script)            |
#   |        smoke.ps1)                         |                              |
#   |--------------------------------------------|------------------------------|
#   | param($RepoRoot, $AgentHttp, $ZmqReq/     | flags below (same names)     |
#   |   SubPort, $KernelExe, $MaterialPaths,     |                              |
#   |   -SkipBuild, -Reuse*, $WaitSeconds,       |                              |
#   |   $ObservationTimeoutSec)                  |                              |
#   | Agent via dev_agent_smoke.ps1 (Restart)    | direct agent start with      |
#   |                                            | draft-root/orchestration-    |
#   |                                            | store isolation + -vsp-hub-  |
#   |                                            | url "" (A5/JOURNEY pattern)  |
#   | Kernel via Start-TestKernel (stops a busy  | kernel under a fake VitApp   |
#   |   port owner!) + cwd = exe dir (repo       | root + VIT_PROJECT_XML copy; |
#   |   Workspace)                               | ports must be free (§9)      |
#   | mixboard/acoustic status artifacts in the  | VIT_MIXBOARD_ROOT=$run/mix-  |
#   |   repo Workspace Artifacts                 | board → acoustic_package_    |
#   |                                            | status.json at the run root  |
#   | Default materials: test_100hz_10s.wav +    | same candidate list; Paper   |
#   |   Paper Crown.mp3 + repo edm_song.ogg +    | Crown.mp3 is PC-only and     |
#   |   repo-root *.mp3 sweeps                   | mac repo roots carry no mp3s |
#   |                                            | → pass --material-path (may  |
#   |                                            | repeat) for a 2nd material;  |
#   |                                            | ogg candidate also probes a  |
#   |                                            | --tracktion-dir checkout     |
#   |                                            | (worktree submodules empty)  |
#   | Invoke-AgentTool / Import-AudioFixture     | identical tool bodies        |
#   | Immediate + retrying mix.observe loop      | identical (0.75s then 2s     |
#   |                                            | backoff, same deadline       |
#   |                                            | clamp max(10,min(T,120)))    |
#   | MOM v1.4 / track acoustics / readiness     | identical field checks       |
#   |   reasons / coverage / rankings asserts    | (ported 1:1 into python)     |
#   | AgentLog tail into artifacts               | same (agent_last.log)        |
#   | summary vit_live_material_observation_     | .mac.v1 + run meta (§8)      |
#   |   smoke.v1                                 |                              |
#
# §8 discipline: deterministic tool-face chain (no LLM) — ≤3 valid runs;
# success = single run exit 0 with all observation assertions green.

set -uo pipefail

REPO_ROOT_DEFAULT=""
AGENT_HTTP="http://127.0.0.1:7878"
KERNEL_BIN_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
WAIT_SECONDS=60
OBSERVATION_TIMEOUT_SEC=180
KEEP_PROCESSES=0
WORKDIR_ARG=""
TRACKTION_DIR=""
ARTIFACT_ROOT_ARG=""
MATERIAL_PATHS=()

KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

usage() {
  cat <<'EOF'
run_live_material_observation_smoke_mac.sh — mac port of
run_live_material_observation_smoke.ps1 (PORT-SMOKE-MAC-1; deterministic
mix.observe chain, no LLM turns).

Options:
  --repo-root PATH          Repository root (default: parent of this script's dir)
  --agent-http URL          Agent HTTP base (default http://127.0.0.1:7878)
  --kernel-bin PATH         Reuse an existing VitApp kernel binary
  --skip-agent-build        With --agent-bin: do not build the agent
  --agent-bin PATH          Agent binary to start (only with --skip-agent-build)
  --material-path PATH      Audio material to import (repeatable; ps1
                            MaterialPaths). Default candidates: repo
                            test_100hz_10s.wav, repo tracktion edm_song.ogg
                            (probes --tracktion-dir when the worktree
                            submodule is empty); at least two usable
                            materials are required.
  --wait-seconds N          Startup wait (default 60)
  --observation-timeout-sec N  mix.observe timeout (default 180)
  --keep-processes          Do not stop the stack this run started
  --workdir PATH            Reuse PATH as the run artifact dir
  --tracktion-dir PATH      tracktion_engine source dir (kernel build and
                            ogg material candidate)
  --artifact-root DIR       Artifact root (default ~/Documents/vit-smoke-mac1-artifacts)
  -h, --help                Show this help

Exit codes: 0 = observation smoke passed; 1 = assertion failure; 2 = environment.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --material-path) MATERIAL_PATHS+=("$2"); shift 2 ;;
    --wait-seconds) WAIT_SECONDS="$2"; shift 2 ;;
    --observation-timeout-sec) OBSERVATION_TIMEOUT_SEC="$2"; shift 2 ;;
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

RUN_ID="live_material_obs_mac_$(date '+%Y%m%d-%H%M%S')"
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
MIXBOARD_ROOT="$WORKDIR/mixboard"
mkdir -p "$AGENT_DRAFTS" "$AGENT_STATE_DIR" "$AGENT_CWD" "$MIXBOARD_ROOT"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
# acousticpackage/status.go: with VIT_MIXBOARD_ROOT set the status artifact
# lands at Dir(root)/acoustic_package_status.json (the mac equivalent of the
# ps1's repo-Workspace fallback path).
ACOUSTIC_STATUS_FALLBACK="$WORKDIR/acoustic_package_status.json"

KERNEL_PID=""
AGENT_PID=""
KERNEL_STOP_RECORD=""
AGENT_STOP_RECORD=""
OUTCOME="env_failure"
KERNEL_BIN=""
AGENT_BIN=""
TRACKTION_RESOLVED="${TRACKTION_DIR:-}"

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
    [[ -z "$AGENT_PID" ]] || { log "stopping agent..."; stop_process "$AGENT_PID"; AGENT_STOP_RECORD="$STOP_RECORD"; }
    [[ -z "$KERNEL_PID" ]] || { log "stopping kernel..."; stop_process "$KERNEL_PID"; KERNEL_STOP_RECORD="$STOP_RECORD"; }
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
  [[ -f "$AGENT_LOG" ]] && tail -160 "$AGENT_LOG" > "$WORKDIR/agent_log_tail.txt" 2>/dev/null
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
step "Preflight: run root + repo fingerprints + ports"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_live_material_observation_smoke_mac.sh (mac port of run_live_material_observation_smoke.ps1)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$REPO_DEFAULT_PROJECT" ]] || fail_env "repo default project missing: $REPO_DEFAULT_PROJECT"

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
  if [[ -z "$TRACKTION_RESOLVED" ]]; then TRACKTION_RESOLVED="$REPO_ROOT/tracktion_engine"; fi
  [[ -f "$TRACKTION_RESOLVED/CMakeLists.txt" ]] \
    || fail_env "tracktion_engine not usable at $TRACKTION_RESOLVED (pass --tracktion-dir or --kernel-bin)"
  log "building VitApp kernel (cmake+make)..."
  if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_RESOLVED" \
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

# ---------------------------------------------------------------- materials
step "Resolve materials"
python3 - "$REPO_ROOT" "$TRACKTION_RESOLVED" "$WORKDIR/materials.json" "${MATERIAL_PATHS[@]+"${MATERIAL_PATHS[@]}"}" <<'PY'
import json, os, re, sys

repo, tracktion, out = sys.argv[1:4]
explicit = sys.argv[4:]

def label_for(path):
    ext = os.path.splitext(path)[1].lstrip(".").lower()
    name = os.path.splitext(os.path.basename(path))[0]
    if name == "test_100hz_10s":
        return "fixture_100hz_wav"
    if ext == "mp3":
        return "live_mp3_" + re.sub(r"[^A-Za-z0-9]+", "_", name).strip("_")
    return "live_" + ext + "_" + re.sub(r"[^A-Za-z0-9]+", "_", name).strip("_")

rows, skipped, seen = [], [], set()

def add(path, label):
    path = os.path.abspath(path)
    key = path.lower()
    if key in seen:
        return
    seen.add(key)
    rows.append({"label": label, "path": path,
                 "file_name": os.path.basename(path),
                 "extension": os.path.splitext(path)[1].lstrip(".").lower()})

if explicit:
    for path in explicit:
        if os.path.isfile(path):
            add(path, label_for(path))
        else:
            skipped.append({"path": path, "reason": "material_path_missing"})
else:
    defaults = [
        ("fixture_100hz_wav", os.path.join(repo, "test_100hz_10s.wav")),
        ("aigc_song_mp3_paper_crown", os.path.join(repo, "Paper Crown.mp3")),
        ("repo_demo_ogg", os.path.join(repo, "tracktion_engine", "examples", "DemoRunner", "resources", "edm_song.ogg")),
    ]
    if tracktion and os.path.isfile(os.path.join(tracktion, "CMakeLists.txt")):
        defaults.append(("repo_demo_ogg_tracktion_dir", os.path.join(tracktion, "examples", "DemoRunner", "resources", "edm_song.ogg")))
    for label, path in defaults:
        if os.path.isfile(path):
            add(path, label)
        else:
            skipped.append({"label": label, "reason": "default_material_missing"})

json.dump({"materials": rows, "skipped": skipped}, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(json.dumps({"materials": len(rows), "skipped": len(skipped),
                  "labels": [r["label"] for r in rows],
                  "skipped_rows": skipped}, ensure_ascii=False))
PY
cat "$WORKDIR/materials.json" >&2
MATERIAL_COUNT="$(json_field "$WORKDIR/materials.json" 'len(d["materials"])')"
(( MATERIAL_COUNT >= 2 )) || fail_env "need at least two available materials for live observation smoke; found $MATERIAL_COUNT (mac machine fact: Paper Crown.mp3 is PC-only — pass --material-path)"
ok "materials resolved: $MATERIAL_COUNT"

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

invoke_tool() {
  # invoke_tool <tool> <args-json-file-or-'-'> <confirmed 0|1> <out-prefix> [timeout]
  local tool="$1" args_path="$2" confirmed="$3" prefix="$4" timeout="${5:-120}" code attempt
  python3 - "$tool" "$args_path" "$confirmed" "$WORKDIR/bodies/tool_invoke.json" <<'PY'
import json, sys
tool, args_path, confirmed, out = sys.argv[1:5]
args = json.load(open(args_path, encoding="utf-8")) if args_path != "-" else {}
json.dump({"tool": tool, "args": args, "confirmed": confirmed == "1",
           "source": "vit_live_material_observation_smoke"}, open(out, "w", encoding="utf-8"), ensure_ascii=False)
PY
  for attempt in $(seq 0 "$BUSY_RETRY_MAX"); do
    if code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/tool_invoke.json" "${prefix}.json" "$timeout")"; then
      if grep -q 'Engine is busy rendering' "${prefix}.json" 2>/dev/null; then
        if (( attempt < BUSY_RETRY_MAX )); then
          log "kernel busy rendering — retry $((attempt + 1))/$BUSY_RETRY_MAX in ${BUSY_RETRY_SECONDS}s"
          sleep "$BUSY_RETRY_SECONDS"
          continue
        fi
      fi
      [[ "$code" =~ ^2 ]] || fail_env "invoke $tool returned HTTP $code (see ${prefix}.json)"
      json_field "${prefix}.json" 'str(d.get("status",""))'
      return 0
    else
      fail_env "invoke $tool transport failed"
    fi
  done
}

# ---------------------------------------------------------------- start stack
step "Prepare agent + kernel"
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
ok "started kernel pid=$KERNEL_PID"

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
ok "agent ready at $AGENT_HTTP (pid $AGENT_PID)"

# ---------------------------------------------------------------- project + imports
step "Create material project"
printf '{}' > "$WORKDIR/bodies/none.json"
PROJECT_NEW_STATUS="$(invoke_tool project.new "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_new")"
if [[ "$PROJECT_NEW_STATUS" == "ok" ]]; then ok "project.new reset completed"; sleep 0.5; else warn "project.new unavailable"; fi
[[ "$(invoke_tool project.clear "$WORKDIR/bodies/none.json" 1 "$WORKDIR/http/project_clear")" == "ok" ]] \
  || fail_functional "project.clear failed"

: > "$WORKDIR/imports.jsonl"
MATERIAL_INDEX=0
while IFS= read -r material_json; do
  MATERIAL_INDEX=$((MATERIAL_INDEX + 1))
  track_name="$(python3 -c '
import json, os, re, sys
path = json.loads(sys.argv[1])["path"]
name = os.path.splitext(os.path.basename(path))[0]
safe = re.sub(r"[^\w _-]+", "", name, flags=re.UNICODE).strip()
print((safe[:40] if safe else "Audio Material"))' "$material_json")"
  printf '{"name":%s}' "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1], ensure_ascii=False))' "$track_name")" > "$WORKDIR/bodies/track_args.json"
  [[ "$(invoke_tool track.add_audio "$WORKDIR/bodies/track_args.json" 1 "$WORKDIR/http/track_add_$MATERIAL_INDEX")" == "ok" ]] \
    || fail_functional "track.add_audio $track_name failed"
  TRACK_ID="$(json_field "$WORKDIR/http/track_add_$MATERIAL_INDEX.json" 'str((d.get("result") or {}).get("track_id") or (d.get("result") or {}).get("id") or "")')"
  [[ -n "$TRACK_ID" ]] || fail_functional "could not resolve track ID for $track_name"
  FILE_PATH="$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["path"])' "$material_json")"
  printf '{"track_id":%s,"file_path":%s,"start_time":0,"media_type":"audio","mode":"non_destructive"}' \
    "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$TRACK_ID")" \
    "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$FILE_PATH")" > "$WORKDIR/bodies/import_args.json"
  IMPORT_STATUS="$(invoke_tool clip.import_media_to_track "$WORKDIR/bodies/import_args.json" 1 "$WORKDIR/http/import_$MATERIAL_INDEX")"
  if [[ "$IMPORT_STATUS" != "ok" ]]; then
    printf '{"track_id":%s,"file_path":%s,"offset_time":0}' \
      "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$TRACK_ID")" \
      "$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$FILE_PATH")" > "$WORKDIR/bodies/import_args.json"
    IMPORT_STATUS="$(invoke_tool clip.import_audio "$WORKDIR/bodies/import_args.json" 1 "$WORKDIR/http/import_$MATERIAL_INDEX")"
  fi
  [[ "$IMPORT_STATUS" == "ok" ]] || fail_functional "import failed: $FILE_PATH"
  python3 - "$material_json" "$WORKDIR/http/import_$MATERIAL_INDEX.json" "$TRACK_ID" "$track_name" >> "$WORKDIR/imports.jsonl" <<'PY'
import json, sys
material = json.loads(sys.argv[1])
resp = json.load(open(sys.argv[2], encoding="utf-8"))
result = resp.get("result") or {}
print(json.dumps({
    "label": material["label"], "track_id": sys.argv[3], "track_name": sys.argv[4],
    "file_path": material["path"], "file_name": material["file_name"],
    "extension": material["extension"],
    "clip_id": str(result.get("clip_id") or result.get("id") or ""),
    "import_status": str(resp.get("status", "")),
    "import_tool": str(resp.get("tool", "")),
    "import_command_name": str(resp.get("command_name", "")),
}, ensure_ascii=False))
PY
  ok "imported $(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["file_name"])' "$material_json") on track $TRACK_ID"
done < <(python3 -c 'import json,sys; [print(json.dumps(r, ensure_ascii=False)) for r in json.load(open(sys.argv[1]))["materials"]]' "$WORKDIR/materials.json")
python3 -c 'import json,sys; rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]; json.dump(rows, open(sys.argv[2], "w", encoding="utf-8"), ensure_ascii=False, indent=2)' "$WORKDIR/imports.jsonl" "$WORKDIR/imports.json"

# ------------------------------------------------------- immediate lifecycle
step "Observe acoustic package lifecycle immediately after import"
STAMP="$(date '+%Y%m%d_%H%M%S')"
IMMEDIATE_GOAL="了解一下当前工程的频段和声像状态，不要执行任何修改。"
python3 - "$IMMEDIATE_GOAL" "acoustic_package_lifecycle_immediate_$STAMP" "$WORKDIR/bodies/observe_args.json" <<'PY'
import json, sys
json.dump({"mix_session_id": sys.argv[2], "scope": "full_project",
           "goal_text": sys.argv[1], "mixboard_request_id": sys.argv[2]},
          open(sys.argv[3], "w", encoding="utf-8"), ensure_ascii=False)
PY
[[ "$(invoke_tool mix.observe "$WORKDIR/bodies/observe_args.json" 1 "$WORKDIR/http/acoustic_package_lifecycle_immediate_observe" "$OBSERVATION_TIMEOUT_SEC")" == "ok" ]] \
  || fail_functional "mix.observe immediate acoustic package lifecycle failed"

python3 - "$WORKDIR/http/acoustic_package_lifecycle_immediate_observe.json" immediate > "$WORKDIR/http/immediate_status.json" <<'PY'
import json, sys
resp = json.load(open(sys.argv[1], encoding="utf-8"))
result = resp.get("result") or {}
status = result.get("acoustic_package_status") or resp.get("acoustic_package_status")
if status is None:
    print(json.dumps({"error": "missing acoustic_package_status"}))
    sys.exit(0)
print(json.dumps(status, ensure_ascii=False))
PY
python3 - "$WORKDIR/http/immediate_status.json" <<'PY' || fail_functional "immediate acoustic package lifecycle assertions failed"
import json, sys
status = json.load(open(sys.argv[1], encoding="utf-8"))
def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr); sys.exit(1)
if status.get("error"):
    fail(status["error"])
if str(status.get("schema_version")) != "acoustic_package_status.v0":
    fail(f"schema_version is not acoustic_package_status.v0: {status.get('schema_version')}")
allowed = ("ready", "partial", "building", "stale", "missing", "deferred", "failed")
layers = {
    "l1_static": ("waveform_envelope", "peak_rms_summary", "time_energy"),
    "l2_realtime": ("live_meter", "realtime_spectrum", "post_fx_meter", "realtime_stereo_correlation"),
    "l3_deep": ("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "lufs_analysis", "masking_analysis", "reference_match"),
}
def feature(layer, name):
    return ((status.get("package_layers") or {}).get(layer) or {}).get("features", {}).get(name)
for layer, names in layers.items():
    row = (status.get("package_layers") or {}).get(layer)
    if row is None:
        fail(f"missing package layer {layer}")
    if str(row.get("status")) not in allowed:
        fail(f"layer {layer} has invalid status {row.get('status')}")
    for name in names:
        f = feature(layer, name)
        if f is None:
            fail(f"missing feature {layer}.{name}")
        if str(f.get("status")) not in allowed:
            fail(f"feature {name} has invalid status {f.get('status')}")
for future in ("lufs_analysis", "masking_analysis", "reference_match"):
    if str((feature("l3_deep", future) or {}).get("status")) != "deferred":
        fail(f"future feature {future} is not deferred")
def number_present(v):
    try:
        return v is not None and str(v).strip() not in ("", "<nil>", "None")
    except Exception:
        return False
for name in ("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"):
    f = feature("l3_deep", name) or {}
    s = str(f.get("status"))
    if s not in ("ready", "partial", "missing", "stale", "deferred"):
        fail(f"l3_deep.{name} should report a read-model status, got {s}")
    if s in ("partial", "missing", "stale"):
        progress = f.get("progress") or {}
        evidence = any([str(f.get("reason") or ""), str(progress.get("reason") or ""),
                        number_present(progress.get("tile_count_seen")),
                        number_present(progress.get("tile_count_expected")),
                        number_present(progress.get("coverage_seconds"))])
        if not evidence:
            fail(f"l3_deep.{name} {s} missing progress/reason")
print("ok: immediate acoustic package lifecycle assertions passed")
sys.exit(0)
PY

STATUS_PATH="$(json_field "$WORKDIR/http/acoustic_package_lifecycle_immediate_observe.json" 'str((d.get("result") or {}).get("acoustic_package_status_path") or d.get("acoustic_package_status_path") or "")')"
if [[ -z "$STATUS_PATH" ]]; then STATUS_PATH="$ACOUSTIC_STATUS_FALLBACK"; fi
[[ -f "$STATUS_PATH" ]] || fail_functional "acoustic package artifact missing: $STATUS_PATH"
[[ "$(json_field "$STATUS_PATH" 'str(d.get("schema_version",""))')" == "acoustic_package_status.v0" ]] \
  || fail_functional "artifact schema mismatch: $STATUS_PATH"
ok "immediate acoustic_package_status captured at $STATUS_PATH"
cp "$STATUS_PATH" "$WORKDIR/acoustic_package_status_immediate.json"

# ------------------------------------------------------- full project observe
step "Observe full project acoustics"
EXPECTED_TRACK_IDS="$(python3 -c 'import json,sys; print(",".join(r["track_id"] for r in json.load(open(sys.argv[1]))))' "$WORKDIR/imports.json")"
FULL_GOAL="compare the overall mix, multitrack band distribution, and stereo layout. do not modify."
RETRY_DEADLINE=$(( SECONDS + (OBSERVATION_TIMEOUT_SEC < 120 ? (OBSERVATION_TIMEOUT_SEC > 10 ? OBSERVATION_TIMEOUT_SEC : 10) : 120) ))
ATTEMPT=0
OBSERVE_FILE=""
while :; do
  ATTEMPT=$((ATTEMPT + 1))
  if (( ATTEMPT == 1 )); then sleep 0.75; else sleep 2; fi
  python3 - "$FULL_GOAL" "live_material_observation_$STAMP" "$WORKDIR/bodies/observe_args.json" <<'PY'
import json, sys
json.dump({"mix_session_id": sys.argv[2], "scope": "full_project",
           "goal_text": sys.argv[1], "mixboard_request_id": sys.argv[2]},
          open(sys.argv[3], "w", encoding="utf-8"), ensure_ascii=False)
PY
  OBSERVE_FILE="$WORKDIR/http/mix_observe_full_project_attempt_$ATTEMPT.json"
  [[ "$(invoke_tool mix.observe "$WORKDIR/bodies/observe_args.json" 1 "${OBSERVE_FILE%.json}" "$OBSERVATION_TIMEOUT_SEC")" == "ok" ]] \
    || fail_functional "mix.observe full_project attempt $ATTEMPT failed"
  python3 - "$OBSERVE_FILE" "$EXPECTED_TRACK_IDS" > "$WORKDIR/bodies/track_issues.json" <<'PY'
import json, sys
resp = json.load(open(sys.argv[1], encoding="utf-8"))
expected = [t for t in sys.argv[2].split(",") if t]
result = resp.get("result") or {}
obs = result.get("observation") or {}
tracks = ((obs.get("project_package") or {}).get("tracks")) or []
def number_present(v):
    try:
        if v is None: return False
        s = str(v).strip()
        if s in ("", "<nil>", "None"): return False
        float(s); return True
    except Exception:
        return False
issues, seen = [], set()
for t in tracks:
    tid = str(t.get("track_id") or "")
    if tid not in expected:
        continue
    seen.add(tid)
    acoustic = t.get("acoustic") or {}
    if str(acoustic.get("status")) != "ready":
        issues.append(f"track {tid} acoustic status={acoustic.get('status')}")
        continue
    for key in ("peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"):
        if not number_present(acoustic.get(key)):
            issues.append(f"track {tid} missing acoustic metric {key}")
    for key in ("primary_clip_id", "primary_clip_name", "source_path"):
        if not str(acoustic.get(key) or ""):
            issues.append(f"track {tid} missing acoustic source field {key}")
for tid in expected:
    if tid not in seen:
        issues.append(f"imported track {tid} was not present in observed project tracks")
print(json.dumps(issues, ensure_ascii=False))
PY
  ISSUE_COUNT="$(json_field "$WORKDIR/bodies/track_issues.json" 'len(d)')"
  if [[ "$ISSUE_COUNT" == "0" ]]; then break; fi
  warn "full_project acoustic readiness pending attempt $ATTEMPT: $(json_field "$WORKDIR/bodies/track_issues.json" '"; ".join(d)')"
  (( SECONDS < RETRY_DEADLINE )) || break
done
cp "$OBSERVE_FILE" "$WORKDIR/http/mix_observe_full_project.json"

python3 - "$WORKDIR/http/mix_observe_full_project.json" "$EXPECTED_TRACK_IDS" "$MATERIAL_COUNT" > "$WORKDIR/bodies/full_assert.json" <<'PY'
import json, sys
resp = json.load(open(sys.argv[1], encoding="utf-8"))
expected = [t for t in sys.argv[2].split(",") if t]
material_count = int(sys.argv[3])
result = resp.get("result") or {}
obs = result.get("observation") or {}
project = obs.get("project_package") or {}
tracks = project.get("tracks") or []
out = {
    "observation_id": str(result.get("observation_id") or ""),
    "active_acoustic_track_count": int(project.get("active_acoustic_track_count") or 0),
    "track_count": len(tracks),
    "readiness_issues": [],
}
print(json.dumps(out, ensure_ascii=False))
PY
ACTIVE_COUNT="$(json_field "$WORKDIR/bodies/full_assert.json" 'd["active_acoustic_track_count"]')"
TRACKS_COUNT="$(json_field "$WORKDIR/bodies/full_assert.json" 'd["track_count"]')"
(( ACTIVE_COUNT >= MATERIAL_COUNT )) || fail_functional "active_acoustic_track_count $ACTIVE_COUNT < imported material count $MATERIAL_COUNT. issues=$(json_field "$WORKDIR/bodies/track_issues.json" '"; ".join(d)')"
(( TRACKS_COUNT >= MATERIAL_COUNT )) || fail_functional "observed track count $TRACKS_COUNT < imported material count $MATERIAL_COUNT"
[[ "$ISSUE_COUNT" == "0" ]] || fail_functional "full_project acoustic readiness did not complete after $ATTEMPT attempts: $(json_field "$WORKDIR/bodies/track_issues.json" '"; ".join(d)')"

# MOM v1.4 + readiness reasons + coverage + rankings (ported 1:1)
python3 - "$WORKDIR/http/mix_observe_full_project.json" "$WORKDIR/acoustic_package_status_immediate.json" "$EXPECTED_TRACK_IDS" "$MATERIAL_COUNT" <<'PY' || fail_functional "full-project observation assertions failed (see stderr)"
import json, sys

resp = json.load(open(sys.argv[1], encoding="utf-8"))
immediate = json.load(open(sys.argv[2], encoding="utf-8"))
expected = [t for t in sys.argv[3].split(",") if t]
material_count = int(sys.argv[4])

def fail(msg):
    print(f"FAIL: {msg}", file=sys.stderr)
    sys.exit(1)

def num(v):
    try:
        s = str(v).strip()
        if s in ("", "<nil>", "None"):
            return None
        return float(s)
    except Exception:
        return None

result = resp.get("result") or {}
obs = result.get("observation") or {}

# --- Assert-MOMV13MultitrackProjection (checks mom_version v1.4)
projection = obs.get("mom_projection")
if projection is None:
    fail("missing mom_projection")
if str(projection.get("mom_version")) != "v1.4":
    fail(f"unexpected mom_version {projection.get('mom_version')}")
if str(projection.get("intent")) != "project_multitrack_relation_observation":
    fail(f"unexpected MOM intent {projection.get('intent')}")
profile = projection.get("project_mix_profile") or {}
relation = projection.get("multitrack_relation") or {}
if not profile or not relation:
    fail("missing project_mix_profile or multitrack_relation")
if int(profile.get("track_count") or 0) < 2:
    fail("project_mix_profile track_count < 2")
if int(relation.get("track_count") or 0) < 2:
    fail("multitrack_relation track_count < 2")
llm_context = projection.get("llm_context")
if not isinstance(llm_context, dict):
    fail("missing llm_context")
if str(llm_context.get("do_not_include_raw_package")).lower() not in ("true", "1"):
    fail("llm_context.do_not_include_raw_package was not true")
compact = json.dumps(llm_context, ensure_ascii=False)
for forbidden in ('"time_segments":', '"track_waveform_envelopes":', '"spectrogram_tile_rows":'):
    if forbidden in compact:
        fail(f"leaked raw field {forbidden}")

# --- Assert-TrackAcousticsReady
tracks = (obs.get("project_package") or {}).get("tracks") or []
seen = set()
for t in tracks:
    tid = str(t.get("track_id") or "")
    if tid not in expected:
        continue
    seen.add(tid)
    acoustic = t.get("acoustic") or {}
    if str(acoustic.get("status")) != "ready":
        fail(f"track {tid} acoustic status is not ready: {acoustic.get('status')}")
    for key in ("peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"):
        if num(acoustic.get(key)) is None:
            fail(f"track {tid} missing acoustic metric {key}")
    for key in ("primary_clip_id", "primary_clip_name", "source_path"):
        if not str(acoustic.get(key) or ""):
            fail(f"track {tid} missing acoustic source field {key}")
for tid in expected:
    if tid not in seen:
        fail(f"imported track {tid} was not present in observed project tracks")

# --- Build-AcousticReadiness + Assert-AcousticReadinessReasons
readiness = {
    "source_capabilities": obs.get("source_capabilities"),
    "mix_source_capabilities": (obs.get("mix_package") or {}).get("source_capabilities"),
    "deep_source_capabilities": (obs.get("deep_package") or {}).get("source_capabilities"),
    "project_limitations": (obs.get("project_package") or {}).get("limitations") or [],
    "feature_snapshot": ((obs.get("global_summary") or {}).get("feature_snapshot") or {}),
    "current_metrics": {
        "band_energy": ((obs.get("mix_package") or {}).get("current_metrics") or {}).get("band_energy"),
        "stereo_relation": ((obs.get("mix_package") or {}).get("current_metrics") or {}).get("stereo_relation"),
    },
}
snapshot = readiness["feature_snapshot"]
latest = snapshot.get("latest_request") or {}
request_id = str(latest.get("request_id") or "")
if not request_id:
    fail("feature_snapshot.latest_request.request_id missing")

def kernel_prepared(latest):
    return (str(latest.get("status")) == "materialized"
            or str(latest.get("lifecycle")) == "kernel_prepared_materializer"
            or str(latest.get("source_kind")) == "kernel_prepared_telemetry"
            or request_id.startswith("kernel_prepared_"))

if kernel_prepared(latest):
    rows = snapshot.get("track_waveform_envelopes") or []
    if len(rows) < 1:
        fail("kernel-prepared feature_snapshot missing track_waveform_envelopes")
    for row in rows:
        if str(row.get("status")) != "ready":
            fail(f"kernel-prepared track waveform row is not ready: {row}")
        for key in ("track_id", "clip_id", "request_id", "source_revision"):
            if not str(row.get(key) or ""):
                fail(f"kernel-prepared track waveform row missing {key}")
else:
    requested = {(r or {}).get("feature_type") for r in (latest.get("requested_features") or []) if isinstance(r, dict)}
    for feat in ("waveform_envelope", "spectral_field"):
        if feat not in requested:
            fail(f"latest_request.requested_features missing {feat}")

def dad_quality_gate_reason(reason):
    text = str(reason).strip().lower()
    if not text:
        return False
    tokens = ("reader_all_zero", "fft_input_all_zero", "fft_output_all_zero",
              "tile_all_zero_before_write", "shared_memory_all_zero_after_write",
              "nan_or_inf_detected", "empty_coverage", "no_active_frames_above_gate")
    return any(t in text for t in tokens)

def bridge_row(name):
    row = snapshot.get(name) or {}
    status = str(row.get("status") or "")
    if not status:
        fail(f"acoustic feature {name} missing status")
    row_req = str(row.get("request_id") or "")
    if request_id and row_req and row_req != request_id:
        if not str(row.get("source_revision") or "") and not str(row.get("source_hash") or ""):
            fail(f"acoustic feature {name} request_id mismatch without source revision/hash: got={row_req} expected={request_id}")
    if status in ("ready", "partial"):
        if not row_req:
            fail(f"ready acoustic feature {name} missing request_id")
        if not str(row.get("source") or ""):
            fail(f"ready acoustic feature {name} missing source")
        if name == "spectrogram_tiles":
            if str(row.get("feature_type")) != "spectral_field":
                fail("spectrogram_tiles feature_type is not spectral_field")
            if str(row.get("source")) not in ("kernel_tile_ready_direct_collector", "kernel_tile_ready_godot_bridge", "kernel_prepared_telemetry"):
                fail(f"spectrogram_tiles source is not an accepted bridge source: {row.get('source')}")
            seen_n, expected_n = num(row.get("tile_count_seen")), num(row.get("tile_count_expected"))
            if seen_n is None or seen_n < 1:
                fail("spectrogram_tiles ready/partial row missing tile_count_seen")
            if expected_n is None:
                fail("spectrogram_tiles ready/partial row missing tile_count_expected")
        return
    if status in ("requested", "building"):
        return
    if status in ("missing", "stale", "blocked", "unavailable", "invalid"):
        reason = str(row.get("reason") or "")
        if not reason:
            fail(f"acoustic feature {name} status {status} missing explicit reason")
        allowed = ["stale_feature_snapshot_for_current_request", "incomplete_source_identity"]
        if name == "spectrogram_tiles":
            allowed += ["no_spectral_tile_ready_received"]
        elif name == "band_energy_summary":
            allowed += ["spectral_field_missing: no_spectral_tile_ready_received", "no_live_spectrum_payload_for_current_request",
                        "no_live_spectrum_payload", "empty_band_energy_summary", "no_target_match_in_live_levels"]
        elif name == "stereo_relation_summary":
            allowed += ["spectral_field_missing: no_spectral_tile_ready_received", "no_live_stereo_payload_for_current_request",
                        "no_live_stereo_payload", "no_live_spectrum_payload", "no_target_match_in_live_levels"]
        if reason not in allowed and not dad_quality_gate_reason(reason):
            fail(f"acoustic feature {name} has unexpected missing reason {reason}")
        return
    fail(f"acoustic feature {name} has unexpected status {status}")

for name in ("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"):
    bridge_row(name)

for name in ("band_energy", "stereo_relation"):
    row = (readiness["current_metrics"] or {}).get(name) or {}
    if str(row.get("status") or "") == "missing" and not str(row.get("reason") or ""):
        fail(f"current_metrics.{name} missing explicit reason")

deep_caps = readiness["deep_source_capabilities"] or {}
for key in ("masking_analysis", "reference_match", "lufs_analysis"):
    if not str(deep_caps.get(key, "")):
        fail(f"deep acoustic deferred capabilities missing from readiness: {key}")

limits = readiness["project_limitations"]
for required in ("lufs_analysis_deferred_phase_5", "masking_analysis_deferred_phase_5",
                 "reference_match_deferred_phase_5", "post_fx_probe_unavailable_phase_4_1"):
    if required not in limits:
        fail(f"project limitations missing {required}. limitations={limits}")

# --- Assert-AcousticCoverageNotRegressed
def l3_coverage(status):
    feature = ((status.get("package_layers") or {}).get("l3_deep") or {}).get("features", {}).get("spectrogram_tiles") or {}
    progress = feature.get("progress") or {}
    ratio = num(progress.get("coverage_ratio"))
    if ratio is not None:
        return ratio
    seen_n, expected_n = num(progress.get("tile_count_seen")), num(progress.get("tile_count_expected"))
    if seen_n is not None and expected_n is not None and expected_n > 0:
        return seen_n / expected_n
    return None

after_status = result.get("acoustic_package_status") or resp.get("acoustic_package_status")
if after_status is None:
    fail("after-wait acoustic_package_status missing")
if str(after_status.get("schema_version")) != "acoustic_package_status.v0":
    fail("after-wait schema_version is not acoustic_package_status.v0")
before_ratio, after_ratio = l3_coverage(immediate), l3_coverage(after_status)
if before_ratio is not None and after_ratio is not None and after_ratio < (before_ratio - 0.0001):
    fail(f"l3 coverage regressed: before={before_ratio} after={after_ratio}")

# --- rankings
project = obs.get("project_package") or {}
for key in ("loudness_ranking", "peak_ranking", "headroom_risk"):
    rows = project.get(key) or []
    if len(rows) < material_count:
        fail(f"ranking {key} has fewer rows than imported materials: {len(rows)} < {material_count}")

print("ok: MOM v1.4 + readiness reasons + coverage + rankings assertions passed")
sys.exit(0)
PY

STATUS_AFTER_PATH="$(json_field "$WORKDIR/http/mix_observe_full_project.json" 'str((d.get("result") or {}).get("acoustic_package_status_path") or d.get("acoustic_package_status_path") or "")')"
[[ -n "$STATUS_AFTER_PATH" ]] || STATUS_AFTER_PATH="$ACOUSTIC_STATUS_FALLBACK"
[[ -f "$STATUS_AFTER_PATH" ]] && cp "$STATUS_AFTER_PATH" "$WORKDIR/acoustic_package_status_after_wait.json"

python3 - "$WORKDIR/http/mix_observe_full_project.json" > "$WORKDIR/summary_tail.json" <<'PY'
import json, sys
resp = json.load(open(sys.argv[1], encoding="utf-8"))
result = resp.get("result") or {}
obs = result.get("observation") or {}
project = obs.get("project_package") or {}
tracks = project.get("tracks") or []
compact_tracks = []
for t in tracks:
    acoustic = t.get("acoustic") or {}
    row = {"track_id": t.get("track_id"), "track_name": t.get("track_name"),
           "active_state": t.get("active_state"), "acoustic_status": acoustic.get("status")}
    for key in ("primary_clip_id", "primary_clip_name", "source_path", "peak_dbfs",
                "rms_dbfs", "headroom_db", "crest_db", "time_energy_status", "reason"):
        if acoustic.get(key) not in ("", None):
            row[key] = acoustic.get(key)
    compact_tracks.append(row)
print(json.dumps({
    "observation_id": str(result.get("observation_id") or ""),
    "active_acoustic_track_count": int(project.get("active_acoustic_track_count") or 0),
    "track_acoustics": compact_tracks,
    "rankings": {
        "loudness": project.get("loudness_ranking") or [],
        "peak": project.get("peak_ranking") or [],
        "headroom_risk": project.get("headroom_risk") or [],
    },
}, ensure_ascii=False))
PY

python3 - "$WORKDIR/summary.json" "$RUN_ID" "$WORKDIR" "$(json_field "$WORKDIR/materials.json" 'len(d["materials"])')" \
  "$(json_field "$WORKDIR/materials.json" 'len(d["skipped"])')" "$STATUS_PATH" "$KERNEL_BIN" "$AGENT_BIN" <<'PY'
import json, os, sys
out, run_id, workdir, materials, skipped, status_path, kernel_bin, agent_bin = sys.argv[1:9]
tail = json.load(open(os.path.join(workdir, "summary_tail.json"), encoding="utf-8"))
summary = {
    "schema_version": "vit_live_material_observation_smoke.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_live_material_observation_smoke.ps1",
    "run_id": run_id,
    "status": "passed",
    "repo_root_meta": "run_meta.txt",
    "artifact_dir": workdir,
    "kernel_exe": kernel_bin,
    "agent_binary": agent_bin,
    "materials": json.load(open(os.path.join(workdir, "materials.json"), encoding="utf-8"))["materials"],
    "skipped_materials": json.load(open(os.path.join(workdir, "materials.json"), encoding="utf-8"))["skipped"],
    "imports": json.load(open(os.path.join(workdir, "imports.json"), encoding="utf-8")),
    "observation_id": tail["observation_id"],
    "active_acoustic_track_count": tail["active_acoustic_track_count"],
    "track_acoustics": tail["track_acoustics"],
    "rankings": tail["rankings"],
    "acoustic_package_status_path": status_path,
    "immediate_acoustic_package_status": "acoustic_package_status_immediate.json",
    "after_wait_acoustic_package_status": "acoustic_package_status_after_wait.json",
    "artifacts_dir": workdir,
}
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
PY

OUTCOME="all_green"
ok "live material observation smoke passed"
log "summary: $WORKDIR/summary.json"
exit 0
