#!/bin/bash
# run_experiment_chain_py_group_smoke_mac.sh — mac driver for the PC
# experiment-chain py group (PORT-SMOKE-MAC-2 item 1).
#
# PC counterpart paths (scripts/SMOKE_TESTS.md, "B1 Gain Staging Agent
# Smokes" + "B4 Low-End Relation Agent Smoke"): the five py smokes are run
# directly with python against a pre-configured D:\ stack. The mac driver
# keeps the py files untouched and adapts only the stack plumbing:
#
#   | PC (SMOKE_TESTS.md paths)                   | mac (this driver)            |
#   |---------------------------------------------|------------------------------|
#   | repo VitApp/build_release VitApp.exe +      | kernel bin copied into an    |
#   | agent bin VitAgent.exe (py defaults)        | isolated kernel_root per     |
#   |                                             | run dir + worktree-built     |
#   |                                             | vitagent per run (cwd-       |
#   |                                             | isolated)                   |
#   | repo Workspace writes during the smoke      | VIT_PROJECT_XML isolated to  |
#   |                                             | the run dir; agent logs and  |
#   |                                             | smoke artifacts stay inside  |
#   |                                             | the repo worktree           |
#   | 04 a4_multiclip: user-provided              | fixture built deterministi-  |
#   | "post-a4-project.vit" (placeholder path     | cally before the py runs:    |
#   | in the docs)                                | project.new + 2 tracks +     |
#   |                                             | import + split_clip +        |
#   |                                             | save_as_project (clip.split  |
#   |                                             | catalog face)                |
#   | LLM engine from machine config              | ~/.vit/config.json or        |
#   |                                             | VIT_AGENT_LLM_* env,         |
#   |                                             | presence-only preflight      |
#   |                                             | (the key never reaches logs) |
#
# Items (serial; each gets its own stack, ports are waited free between
# items — AGENTS §9 single Mac stack owner):
#   01 b1_group_reset_agent_smoke.py            (self-started stack, 17878)
#   02 b1_2_source_calibration_agent_smoke.py   (self-started stack, 17879)
#   03 b1_3_full_gain_staging_agent_smoke.py    (self-started stack, 17880)
#   04 b1_2_a4_multiclip_agent_smoke.py         (driver stack 7878 + fixture)
#   05 b4_low_end_relation_agent_smoke.py       (driver stack 7878, py builds
#                                                 its own fixture)
#
# §8 declaration (per item): <=3 valid rounds; success = one exit-0 run with
# the py's own assertions; failures classified (environment interruption
# needs raw evidence); same-breakpoint two-failure stop-loss; LLM turns rely
# on the preconfigured engine, key never enters artifacts.
#
# Exit codes: 0 = every selected item passed; 1 = at least one item failed;
# 2 = environment failure before any item ran.

set -euo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_ARG=""
ARTIFACT_ROOT_ARG=""
ITEMS="01,02,03,04,05"
PYTHON_BIN="python3"

usage() {
  cat <<'EOF'
run_experiment_chain_py_group_smoke_mac.sh — mac driver for the
experiment-chain py group (PORT-SMOKE-MAC-2 item 1).

Options:
  --repo-root PATH    Repository root (default: parent of this script's dir)
  --kernel-bin PATH   Reuse an existing VitApp kernel binary (required in
                      practice; sha256 recorded per run)
  --artifact-root DIR Artifact root (default ~/Documents/vit-smoke-mac2-artifacts)
  --items LIST        Comma list of item numbers (default 01,02,03,04,05)
  --python PATH       Python interpreter (default python3; needs pyzmq for
                      the kernel-side probes? — the py group itself is
                      urllib-only, pyzmq not required here)
  -h, --help          Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --artifact-root) ARTIFACT_ROOT_ARG="$2"; shift 2 ;;
    --items) ITEMS="$2"; shift 2 ;;
    --python) PYTHON_BIN="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in go python3 lsof shasum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac2-artifacts}"

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" >&2; }
fail_env()       { echo "ERROR[env]: $*" >&2; exit 2; }

[[ -n "$KERNEL_BIN_ARG" && -x "$KERNEL_BIN_ARG" ]] || fail_env "--kernel-bin pointing at an executable VitApp kernel is required"
KERNEL_BIN_SRC="$(cd "$(dirname "$KERNEL_BIN_ARG")" && pwd)/$(basename "$KERNEL_BIN_ARG")"
KERNEL_SHA256="$(shasum -a 256 "$KERNEL_BIN_SRC" | cut -d' ' -f1)"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REPO_DEFAULT_PROJECT" ]] || fail_env "repo default project template missing: $REPO_DEFAULT_PROJECT"

RUN_STAMP="$(date '+%Y%m%d-%H%M%S')"
mkdir -p "$ARTIFACT_ROOT"
DRIVER_DIR="$ARTIFACT_ROOT/pychain_driver_$RUN_STAMP"
mkdir -p "$DRIVER_DIR"

# ---------------------------------------------------------------- LLM preflight
step "Preflight: LLM engine config (presence only — the key never reaches logs)"
LLM_CFG_SUMMARY="$("$PYTHON_BIN" - <<'PY'
import json, os
path = os.path.expanduser("~/.vit/config.json")
def env(k): return os.environ.get(k, "")
try:
    cfg = json.load(open(path, encoding="utf-8"))
except Exception as e:
    cfg = {}
base = str(cfg.get("baseUrl", "")).strip() or env("VIT_AGENT_LLM_BASE_URL") or env("OPENAI_BASE_URL")
key  = str(cfg.get("apiKey", "")).strip() or env("VIT_AGENT_LLM_API_KEY") or env("OPENAI_API_KEY")
model= str(cfg.get("defaultModel", "")).strip() or env("VIT_AGENT_LLM_MODEL") or env("OPENAI_MODEL")
print(json.dumps({"config_file_exists": os.path.isfile(path), "has_api_key": bool(key),
                  "model_present": bool(model), "base_url_present": bool(base)}))
PY
)" || fail_env "LLM config preflight crashed"
log "llm_config=$LLM_CFG_SUMMARY"
[[ "$(printf '%s' "$LLM_CFG_SUMMARY" | "$PYTHON_BIN" -c 'import json,sys; d=json.load(sys.stdin); print("ok" if d.get("has_api_key") and d.get("model_present") else "missing")')" == "ok" ]] \
  || fail_env "LLM engine config incomplete (card stop condition): $LLM_CFG_SUMMARY"

# ---------------------------------------------------------------- agent build
step "Build agent (worktree go build)"
AGENT_BIN="$DRIVER_DIR/vitagent"
( cd "$REPO_ROOT/agent" && go build -o "$AGENT_BIN" ./cmd/vitagent ) || fail_env "go build agent failed"
AGENT_SHA256="$(shasum -a 256 "$AGENT_BIN" | cut -d' ' -f1)"
log "agent built: $AGENT_BIN (sha256=$AGENT_SHA256)"

port_free() {
  [[ -z "$(lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1)" ]]
}

wait_ports_free() {
  local port deadline=$((SECONDS + 30))
  for port in "$@"; do
    while ! port_free "$port"; do
      (( SECONDS < deadline )) || { log "WARN: port $port still busy after 30s"; return 1; }
      sleep 1
    done
  done
  return 0
}

# prepare_item_env <run-dir>: isolated kernel root + workspace + agent dirs.
# The py smokes start the stack with cwd = binary dir (kernel) / binary
# grandparent (agent), so the copied binaries give every item an isolated
# runtime footprint; env vars ride through the py start_process passthrough.
prepare_item_env() {
  local run="$1"
  mkdir -p "$run/kernel_root/Source" "$run/kernel_workspace" "$run/agent_bin" \
           "$run/agent_drafts" "$run/agent_state" "$run/mixboard"
  : > "$run/kernel_root/CMakeLists.txt"
  cp "$KERNEL_BIN_SRC" "$run/kernel_root/VitApp"
  cp "$AGENT_BIN" "$run/agent_bin/vitagent"
  cp "$REPO_DEFAULT_PROJECT" "$run/kernel_workspace/default_project.xml"
  printf 'kernel_sha256=%s\nagent_sha256=%s\n' "$KERNEL_SHA256" "$AGENT_SHA256" > "$run/bins.sha256"
}

export_stack_env() {
  local run="$1"
  export VIT_PROJECT_XML="$run/kernel_workspace/default_project.xml"
  export VIT_DAW_DEV_ROOT="$REPO_ROOT"
  export VIT_HISTORY_DRAFT_ROOT="$run/agent_drafts"
  export VIT_ORCHESTRATION_STORE_PATH="$run/agent_state/orchestration_v1.json"
  export VIT_MIXBOARD_ROOT="$run/mixboard"
}

# run_selfstarted_item <num> <py-name> <port> <timeout-sec> [extra py args...]
# The py starts and stops its own kernel+agent (its finally block), the driver
# only isolates the runtime footprint and records the artifacts.
run_selfstarted_item() {
  local num="$1" py="$2" port="$3" timeout_sec="$4"; shift 4
  local run="$ARTIFACT_ROOT/pychain_${num}_$(basename "$py" .py)_$RUN_STAMP"
  step "Item $num: $py (self-started stack, agent http $port)"
  prepare_item_env "$run"
  export_stack_env "$run"
  wait_ports_free "$port" 5555 5556 5557
  set +e
  "$PYTHON_BIN" "$REPO_ROOT/scripts/$py" \
    --repo-root "$REPO_ROOT" \
    --kernel-exe "$run/kernel_root/VitApp" \
    --agent-exe "$run/agent_bin/vitagent" \
    --agent-http "http://127.0.0.1:$port" \
    --agent-http-addr "127.0.0.1:$port" \
    --timeout-sec "$timeout_sec" \
    "$@" > "$run/py_stdout.log" 2>&1
  local code=$?
  set -e
  wait_ports_free "$port" 5555 5556 5557 || true
  log "item $num exit=$code (log: $run/py_stdout.log)"
  RESULTS+=("$num|$py|$code|$run")
  return 0
}

# ---------------------------------------------------------------- driver stack (7878)
# For the py smokes that expect an already-running agent (a4 multiclip,
# b4 low-end). Started with the same flag face the self-starting py smokes
# use, so both stack shapes stay comparable.
start_driver_stack() {
  local run="$1"
  (
    cd "$run/kernel_root"
    exec env VIT_PROJECT_XML="$run/kernel_workspace/default_project.xml" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             "$run/kernel_root/VitApp"
  ) > "$run/kernel_stdout.log" 2>&1 &
  DRIVER_KERNEL_PID=$!
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    kill -0 "$DRIVER_KERNEL_PID" 2>/dev/null || { tail -20 "$run/kernel_stdout.log" >&2; return 1; }
    [[ -n "$(lsof -nP -iTCP:5555 -sTCP:LISTEN -t 2>/dev/null | head -1)" ]] && break
    sleep 1
  done
  port_free 5555 && { echo "kernel command port never came up" >&2; return 1; }
  (
    cd "$run"
    exec env VIT_HISTORY_DRAFT_ROOT="$run/agent_drafts" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             VIT_ORCHESTRATION_STORE_PATH="$run/agent_state/orchestration_v1.json" \
             VIT_MIXBOARD_ROOT="$run/mixboard" \
             "$run/agent_bin/vitagent" \
               -http "127.0.0.1:7878" \
               -udp-to-godot 14444 \
               -udp-from-godot 14445 \
               -vsp-hub-url "" \
               -last-log-path "$run/agent_last.log" \
               -keep-last-log-lines 8000
  ) > "$run/agent_stdout.log" 2>&1 &
  DRIVER_AGENT_PID=$!
  deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    kill -0 "$DRIVER_AGENT_PID" 2>/dev/null || { tail -20 "$run/agent_stdout.log" >&2; return 1; }
    curl -sS --max-time 2 -o /dev/null "http://127.0.0.1:7878/health" 2>/dev/null && return 0
    sleep 1
  done
  return 1
}

stop_driver_stack() {
  local pid
  for pid in "$DRIVER_AGENT_PID" "$DRIVER_KERNEL_PID"; do
    [[ -n "$pid" ]] || continue
    kill -TERM "$pid" 2>/dev/null || true
    local start=$SECONDS
    while kill -0 "$pid" 2>/dev/null; do
      (( SECONDS - start < 10 )) || { kill -KILL "$pid" 2>/dev/null || true; break; }
      sleep 1
    done
  done
  DRIVER_AGENT_PID=""; DRIVER_KERNEL_PID=""
}

# invoke7878 <tool> <json-args> [timeout]: /agent/invoke with confirmed=true.
invoke7878() {
  local tool="$1" args="$2" timeout="${3:-60}"
  curl -sS --max-time "$timeout" \
    -H 'Content-Type: application/json' \
    -d "{\"tool\": \"$tool\", \"args\": $args, \"confirmed\": true, \"source\": \"pychain_fixture_builder\"}" \
    "http://127.0.0.1:7878/agent/invoke"
}

# build_a4_fixture <run>: deterministic post-a4 multi-clip project — the PC
# docs take a pre-existing "post-a4-project.vit" from the user; on mac the
# equivalent shape (>=2 tracks, >=1 track with >1 clip) is built through the
# agent tool face (clip.import_audio + clip.split + project.save_as).
build_a4_fixture() {
  local run="$1"
  "$PYTHON_BIN" - "$REPO_ROOT/test_100hz_10s.wav" "$REPO_ROOT/test_target_3s.wav" "$run/post_a4_fixture.vit" > "$run/a4_fixture_build.log" 2>&1 <<'PY' || return 1
import json, sys, urllib.request

wav_a, wav_b, project_path = sys.argv[1:4]
BASE = "http://127.0.0.1:7878"

def invoke(tool, args, timeout=90):
    payload = {"tool": tool, "args": args, "confirmed": True, "source": "pychain_fixture_builder"}
    req = urllib.request.Request(BASE + "/agent/invoke", data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"}, method="POST")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        out = json.loads(resp.read().decode())
    status = str(out.get("status", "")).lower()
    if status not in ("ok", "success", "completed"):
        raise SystemExit(f"{tool} failed: {json.dumps(out, ensure_ascii=False)[:1200]}")
    return out

def result_of(out):
    r = out.get("result")
    return r if isinstance(r, dict) else {}

invoke("project.new", {})
track_a = result_of(invoke("track.add_audio", {"name": "Split Target"})).get("track_id") \
          or result_of(invoke("track.add_audio", {"name": "Split Target"})).get("id")
track_b = result_of(invoke("track.add_audio", {"name": "Secondary"})).get("track_id") \
          or result_of(invoke("track.add_audio", {"name": "Secondary"})).get("id")
if not track_a or not track_b:
    raise SystemExit("track.add_audio did not return track ids")
imported = invoke("import_audio", {"track_id": track_a, "file_path": wav_a, "offset_time": 0})
clip_id = result_of(imported).get("clip_id") or result_of(imported).get("id")
if not clip_id:
    raise SystemExit("import_audio did not return clip id")
split = invoke("split_clip", {"clip_id": clip_id, "track_id": track_a, "split_time": 5.0})
invoke("import_audio", {"track_id": track_b, "file_path": wav_b, "offset_time": 0})
invoke("save_as_project", {"file_path": project_path})
state = invoke("project.state", {})
tracks = [t for t in (result_of(state).get("tracks") or []) if isinstance(t, dict)]
multi = sum(1 for t in tracks if len(t.get("clips") or []) > 1)
print(json.dumps({"saved": project_path, "tracks": len(tracks), "multi_clip_tracks": multi,
                  "split_reply_keys": sorted(result_of(split).keys())}, ensure_ascii=False))
if len(tracks) < 2 or multi < 1:
    raise SystemExit(f"fixture shape wrong: tracks={len(tracks)} multi={multi}")
PY
}

# run_driverstack_item <num> <py-name> <timeout-sec> [extra py args...]
run_driverstack_item() {
  local num="$1" py="$2" timeout_sec="$3"; shift 3
  local run="$ARTIFACT_ROOT/pychain_${num}_$(basename "$py" .py)_$RUN_STAMP"
  step "Item $num: $py (driver stack on 7878)"
  prepare_item_env "$run"
  wait_ports_free 7878 5555 5556 5557 || fail_env "stack ports busy before item $num"
  if ! start_driver_stack "$run"; then
    stop_driver_stack || true
    fail_env "driver stack failed to come up for item $num"
  fi
  local fixture_status="not_needed"
  if [[ "$num" == "04" ]]; then
    if build_a4_fixture "$run"; then
      fixture_status="built"
      ok "a4 fixture built: $run/post_a4_fixture.vit"
    else
      fixture_status="failed"
      log "a4 fixture build failed (see $run/a4_fixture_build.log)"
    fi
  fi
  local code=1
  if [[ "$fixture_status" != "failed" ]]; then
    local py_args=(--repo-root "$REPO_ROOT" --agent-http "http://127.0.0.1:7878" --timeout-sec "$timeout_sec")
    if [[ "$num" == "04" ]]; then py_args+=(--project-path "$run/post_a4_fixture.vit"); fi
    set +e
    "$PYTHON_BIN" "$REPO_ROOT/scripts/$py" "${py_args[@]}" "$@" > "$run/py_stdout.log" 2>&1
    code=$?
    set -e
  fi
  stop_driver_stack || true
  wait_ports_free 7878 5555 5556 5557 || true
  log "item $num exit=$code fixture=$fixture_status (log: $run/py_stdout.log)"
  RESULTS+=("$num|$py|$code|$run")
  return 0
}

RESULTS=()
DRIVER_KERNEL_PID=""
DRIVER_AGENT_PID=""

step "Driver preflight (repo $(cd "$REPO_ROOT" && git rev-parse --short HEAD 2>/dev/null || echo unknown), kernel sha256=$KERNEL_SHA256)"
log "artifact root: $ARTIFACT_ROOT"
printf 'started_at=%s\nrepo_root=%s\nrepo_head=%s\nkernel_bin=%s\nkernel_sha256=%s\nagent_sha256=%s\nllm_config=%s\nitems=%s\n' \
  "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$REPO_ROOT" \
  "$(cd "$REPO_ROOT" && git rev-parse HEAD 2>/dev/null || echo unknown)" \
  "$KERNEL_BIN_SRC" "$KERNEL_SHA256" "$AGENT_SHA256" "$LLM_CFG_SUMMARY" "$ITEMS" \
  > "$DRIVER_DIR/run_meta.txt"

IFS=',' read -r -a ITEM_LIST <<< "$ITEMS"
for num in "${ITEM_LIST[@]}"; do
  num="$(echo "$num" | tr -d '[:space:]')"
  [[ -n "$num" ]] || continue
  case "$num" in
    01) run_selfstarted_item 01 b1_group_reset_agent_smoke.py 17878 150 ;;
    02) run_selfstarted_item 02 b1_2_source_calibration_agent_smoke.py 17879 150 ;;
    03) run_selfstarted_item 03 b1_3_full_gain_staging_agent_smoke.py 17880 240 ;;
    04) run_driverstack_item 04 b1_2_a4_multiclip_agent_smoke.py 300 ;;
    04b) run_driverstack_item 04 b1_2_a4_multiclip_agent_smoke.py 300 --full-b1 ;;
    05) run_driverstack_item 05 b4_low_end_relation_agent_smoke.py 240 --dad-timeout-sec 240 ;;
    *) log "unknown item number skipped: $num" ;;
  esac
done

step "Summary"
"$PYTHON_BIN" - "$DRIVER_DIR/summary.json" "${RESULTS[@]+"${RESULTS[@]}"}" <<'PY'
import json, sys
out_path, rows = sys.argv[1], sys.argv[2:]
items = []
for row in rows:
    num, py, code, run = row.split("|", 3)
    items.append({"item": num, "py": py, "exit_code": int(code), "run_dir": run,
                  "status": "passed" if int(code) == 0 else "failed"})
summary = {
    "schema_version": "experiment_chain_py_group_smoke.mac.v1",
    "card": "PORT-SMOKE-MAC-2",
    "items": items,
    "status": "passed" if items and all(i["status"] == "passed" for i in items) else "failed",
}
json.dump(summary, open(out_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print(json.dumps(summary, ensure_ascii=False, indent=2))
PY
SUMMARY_STATUS="$("$PYTHON_BIN" -c 'import json,sys; print(json.load(open(sys.argv[1]))["status"])' "$DRIVER_DIR/summary.json")"
log "driver summary: $DRIVER_DIR/summary.json (status=$SUMMARY_STATUS)"
[[ "$SUMMARY_STATUS" == "passed" ]] || exit 1
exit 0
