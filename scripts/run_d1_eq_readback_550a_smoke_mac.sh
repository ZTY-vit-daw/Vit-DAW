#!/usr/bin/env bash
# run_d1_eq_readback_550a_smoke_mac.sh — D1-EQ-READBACK-550A-1 真栈回归烟测（mac）。
#
# 目标：在真实栈（VitApp 内核 + Go agent，两件套——agent HTTP 面驱动全程，
# Godot 面外，同 journey1 mac 先例）上锁定 API-550A Stereo 场景跑一轮完整
# d1 自由态实验（观察→假设→准入→装载→写参→读回→A/B→人耳边界），断言
# readback_verified=True（scripts/free_state_d1_smoke.py 的既有断言面，
# exit 0 为准）。
#
# 550A 锁定机制：机器本地实验白名单 ~/.vit/free_state_experiment_plugins.json
# 的 static_eq 段临时收窄为仅 API-550A Stereo（其它族逐字节不动），运行结束
# （含失败/中断）恢复原文件。白名单是机器本地态（PORT-WL-EQ-1 先例），本
# 脚本对它只做 备份→收窄→恢复 三步，不落仓。
#
# 工程素材：spv1_p01 桌面 stems（与 PC d1 fixture 同源素材）经内核
# project_stems_import_probe.py 建成 .vit 工程 + 合成 public manifest
# （schema+blindness 六键+case 行），供跨平台 evaluator 消费。
#
# §8 纪律：LLM 参与运行——最多 3 个有效轮；成功=单轮 exit 0（evaluator
# 断言全绿）；exit 3=NOT_EXERCISED（模型未选 static_eq 域，记录性结果，
# 消耗一轮）；同一确定性断点连续 2 次失败止损。
#
# Usage:
#   ./scripts/run_d1_eq_readback_550a_smoke_mac.sh [--kernel-bin PATH]
#        [--stems-dir PATH] [--vst3-dir PATH] [--agent-http URL]
#        [--timeout-sec N] [--runs N] [--workdir PATH] [--skip-agent-build --agent-bin P]
#
# Exit codes: 0=PASS；1=断言失败/止损；2=环境失败；3=NOT_EXERCISED（全轮）。
set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_ARG=""
STEMS_DIR="$HOME/Desktop/cases/spv1_p01/stems"
VST3_DIR="/Library/Audio/Plug-Ins/VST3"
AGENT_HTTP="http://127.0.0.1:7878"
AGENT_HTTP_ADDR="127.0.0.1:7878"
TIMEOUT_SECONDS=600
MAX_RUNS=3
WORKDIR_ARG=""
SKIP_AGENT_BUILD=0
AGENT_BIN_ARG=""
TRACKTION_DIR=""

KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

usage() {
  cat <<'EOF'
run_d1_eq_readback_550a_smoke_mac.sh — locked-550A real-stack d1 regression (mac).

Options:
  --kernel-bin PATH    Reuse a built VitApp kernel (records sha256; default builds Debug via cmake)
  --stems-dir PATH     spv1_p01 stems folder (default ~/Desktop/cases/spv1_p01/stems)
  --vst3-dir PATH      VST3 search path (default /Library/Audio/Plug-Ins/VST3)
  --agent-http URL     Agent HTTP base (default http://127.0.0.1:7878)
  --timeout-sec N      Evaluator timeout per run (default 600)
  --runs N             Max valid LLM rounds (§8, default 3)
  --workdir PATH       Reuse a run workdir
  --skip-agent-build   With --agent-bin: do not build the agent
  --agent-bin PATH     Agent binary (only with --skip-agent-build)
  --tracktion-dir PATH tracktion_engine source (default: repo submodule checkout)
  -h, --help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --stems-dir) STEMS_DIR="$2"; shift 2 ;;
    --vst3-dir) VST3_DIR="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; AGENT_HTTP_ADDR="${2#http://}"; shift 2 ;;
    --timeout-sec) TIMEOUT_SECONDS="$2"; shift 2 ;;
    --runs) MAX_RUNS="$2"; shift 2 ;;
    --workdir) WORKDIR_ARG="$2"; shift 2 ;;
    --skip-agent-build) SKIP_AGENT_BUILD=1; shift ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --tracktion-dir) TRACKTION_DIR="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

SCRIPT_PATH="${BASH_SOURCE[0]}"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$(dirname "$SCRIPT_PATH")/.." && pwd)}"

fail_env() { echo "ERROR[env]: $*" >&2; exit 2; }
fail_run() { echo "ERROR[run]: $*" >&2; exit 1; }
log() { echo "[$(date '+%H:%M:%S')] $*" >&2; }

for tool in go cmake make python3 lsof shasum curl jq; do
  command -v "$tool" >/dev/null 2>&1 || fail_env "required tool not found: $tool"
done

python3 -c "import zmq" 2>/dev/null || fail_env "pyzmq missing (probe needs it)"
python3 -c "import soundfile, numpy" 2>/dev/null || fail_env "soundfile/numpy missing (evaluator material gates need them)"

[[ -d "$STEMS_DIR" ]] || fail_env "stems folder missing: $STEMS_DIR"
WHITELIST="$HOME/.vit/free_state_experiment_plugins.json"
[[ -f "$WHITELIST" ]] || fail_env "machine whitelist missing: $WHITELIST"
python3 - "$WHITELIST" <<'PY' || fail_env "LLM engine config incomplete (~/.vit/config.json or VIT_AGENT_LLM_*/OPENAI_* env)"
import json, os, sys
cfg = {}
p = os.path.expanduser("~/.vit/config.json")
if os.path.isfile(p):
    cfg = json.load(open(p))
env = os.environ.get
ok = (str(cfg.get("apiKey", "")).strip() or env("VIT_AGENT_LLM_API_KEY") or env("OPENAI_API_KEY")) and \
     (str(cfg.get("defaultModel", "")).strip() or env("VIT_AGENT_LLM_MODEL") or env("OPENAI_MODEL")) and \
     (str(cfg.get("baseUrl", "")).strip() or env("VIT_AGENT_LLM_BASE_URL") or env("OPENAI_BASE_URL"))
sys.exit(0 if ok else 1)
PY

# -- run identity --------------------------------------------------------------
RUN_ID="d1_550a_$(date '+%Y%m%d_%H%M%S')"
ART_ROOT="$HOME/Documents/vit-d1eq550a-artifacts"
WORKDIR="${WORKDIR_ARG:-$ART_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR"/{bin,logs,zmq,bodies,public_case,agent_state,agent_cwd}
cd "$WORKDIR"

{
  echo "run_id=$RUN_ID"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD)"
  echo "repo_branch=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short | wc -l | tr -d ' ') entries"
  echo "agent_http=$AGENT_HTTP stems_dir=$STEMS_DIR vst3_dir=$VST3_DIR"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
} > "$WORKDIR/run_meta.txt"

KERNEL_PID=""; AGENT_PID=""
WHITELIST_BACKUP="$WORKDIR/whitelist_backup.json"
restore_whitelist() {
  if [[ -f "$WHITELIST_BACKUP" ]]; then
    cp "$WHITELIST_BACKUP" "$WHITELIST" && log "whitelist restored"
  fi
}
cleanup() {
  local rc=$?
  restore_whitelist
  for pid_sig in "$AGENT_PID:TERM" "$KERNEL_PID:TERM"; do
    local pid="${pid_sig%%:*}"
    [[ -n "$pid" ]] || continue
    if kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null || true
      local i; for i in 1 2 3 4 5 6 7 8 9 10; do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
      kill -0 "$pid" 2>/dev/null && kill -KILL "$pid" 2>/dev/null || true
    fi
  done
  wait 2>/dev/null || true
  exit $rc
}
trap cleanup EXIT INT TERM

port_listener_pid() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true; }

for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "$AGENT_HTTP_ADDR"; do
  pid="$(port_listener_pid "${port##*:}")"
  [[ -z "$pid" ]] || fail_env "port $port owned by pid $pid — stagger parallel real-stack work (AGENTS §9)"
done

# -- kernel ---------------------------------------------------------------------
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
else
  if [[ -z "$TRACKTION_DIR" ]]; then TRACKTION_DIR="$REPO_ROOT/tracktion_engine"; fi
  [[ -f "$TRACKTION_DIR/CMakeLists.txt" ]] || fail_env "tracktion_engine unusable at $TRACKTION_DIR"
  log "building kernel (cmake+make Debug)..."
  cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1 || fail_env "kernel cmake configure failed"
  make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp \
       > "$WORKDIR/logs/kernel_build.log" 2>&1 || fail_env "kernel make failed"
  KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
fi
log "kernel: $KERNEL_BIN ($(cut -c1-16 "$WORKDIR/kernel_bin.sha256"))"

# -- agent ----------------------------------------------------------------------
if [[ "$SKIP_AGENT_BUILD" -eq 1 ]]; then
  [[ -n "$AGENT_BIN_ARG" && -x "$AGENT_BIN_ARG" ]] || fail_env "--skip-agent-build requires --agent-bin"
  AGENT_BIN="$AGENT_BIN_ARG"
else
  AGENT_BIN="$WORKDIR/bin/vitagent"
  (cd "$REPO_ROOT/agent" && go build -o "$AGENT_BIN" ./cmd/vitagent) \
    || fail_env "agent build failed"
fi
shasum -a 256 "$AGENT_BIN" > "$WORKDIR/agent_bin.sha256"
log "agent: $AGENT_BIN ($(cut -c 1-16 "$WORKDIR/agent_bin.sha256"))"

# -- stack up -------------------------------------------------------------------
KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source" "$KERNEL_ROOT/Workspace"
: > "$KERNEL_ROOT/CMakeLists.txt"

log "starting kernel (cwd=$KERNEL_ROOT)..."
( cd "$KERNEL_ROOT" && exec "$KERNEL_BIN" ) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
KERNEL_PID=$!
deadline=$((SECONDS + 90))
while (( SECONDS < deadline )); do
  kill -0 "$KERNEL_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2; fail_env "kernel exited during startup"; }
  [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
  sleep 1
done
for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
  listener="$(port_listener_pid "$port")"
  [[ "$listener" == "$KERNEL_PID" ]] || fail_env "port $port listener $listener != kernel $KERNEL_PID"
done
log "kernel up (pid $KERNEL_PID)"

log "starting agent..."
(
  cd "$WORKDIR/agent_cwd"
  exec env VIT_HISTORY_DRAFT_ROOT="$WORKDIR/agent_drafts" \
           VIT_DAW_DEV_ROOT="$REPO_ROOT" \
           VIT_ORCHESTRATION_STORE_PATH="$WORKDIR/agent_state/orchestration_v1.json" \
           "$AGENT_BIN" \
             -http "$AGENT_HTTP_ADDR" \
             -last-log-path "$WORKDIR/agent_state/agent_last.log" \
             -keep-last-log-lines 8000 \
             -vsp-hub-url ""
) > "$WORKDIR/logs/agent_stdout.log" 2>&1 &
AGENT_PID=$!
deadline=$((SECONDS + 90))
while (( SECONDS < deadline )); do
  kill -0 "$AGENT_PID" 2>/dev/null || { tail -30 "$WORKDIR/logs/agent_stdout.log" >&2; fail_env "agent exited during startup"; }
  curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && break
  sleep 1
done
curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null || fail_env "agent health not reachable"
log "agent up (pid $AGENT_PID)"

http_json() { # http_json <method> <url> <body-file> <out-file> <timeout-s> -> http code
  curl -sS -X "$1" -H "Content-Type: application/json" --data-binary "@$3" \
       --max-time "$5" -o "$4" -w "%{http_code}" "$2"
}

# -- warm-up scan (cold fake root cannot resolve identifiers) -------------------
log "warm-up: plugin.semantic_build_index over $VST3_DIR (may take minutes)..."
python3 - "$VST3_DIR" > "$WORKDIR/bodies/scan.json" <<'PY'
import json, sys
print(json.dumps({"tool": "plugin.semantic_build_index", "args": {"paths": [sys.argv[1]]},
                  "confirmed": True, "source": "d1_550a_smoke_mac"}, ensure_ascii=False))
PY
code="$(http_json POST "$AGENT_HTTP/agent/invoke" "$WORKDIR/bodies/scan.json" "$WORKDIR/scan_reply.json" 960)" \
  || fail_env "scan invoke transport failed"
[[ "$code" =~ ^2 ]] || fail_env "scan invoke HTTP $code: $(head -c 300 "$WORKDIR/scan_reply.json")"
SCAN_STATUS="$(jq -r '.status // ""' "$WORKDIR/scan_reply.json")"
SCAN_COUNT="$(jq -r '.result.plugin_count // 0' "$WORKDIR/scan_reply.json")"
log "scan: status=$SCAN_COUNT plugins=$SCAN_COUNT"
[[ "$SCAN_STATUS" == "ok" && "$SCAN_COUNT" -gt 0 ]] || fail_env "scan not ok (status=$SCAN_STATUS count=$SCAN_COUNT)"

# -- authority ------------------------------------------------------------------
printf '{"authority_mode":"full_project_access"}' > "$WORKDIR/bodies/authority.json"
code="$(http_json POST "$AGENT_HTTP/agent/authority" "$WORKDIR/bodies/authority.json" "$WORKDIR/authority_reply.json" 30)" \
  || fail_env "authority transport failed"
[[ "$code" =~ ^2 ]] || fail_env "authority refused HTTP $code"
[[ "$(jq -r '.authority_mode // ""' "$WORKDIR/authority_reply.json")" == "full_project_access" ]] \
  || fail_env "authority refused: $(head -c 200 "$WORKDIR/authority_reply.json")"
log "authority: full_project_access"

# -- build the spv1_p01 project from desktop stems (kernel ZMQ, deterministic) --
PROJECT_VIT="$WORKDIR/public_case/spv1_p01.vit"
log "building .vit project from $STEMS_DIR (stems import probe)..."
python3 "$REPO_ROOT/scripts/project_stems_import_probe.py" \
  --req-url "tcp://127.0.0.1:$KERNEL_PORT_REQ" \
  --project-path "$PROJECT_VIT" \
  --training-folder "$STEMS_DIR" \
  --output "$WORKDIR/stems_probe_report.json" \
  > "$WORKDIR/logs/stems_probe.log" 2>&1 || { tail -20 "$WORKDIR/logs/stems_probe.log" >&2; fail_env "stems import probe failed"; }
[[ "$(jq -r '.status' "$WORKDIR/stems_probe_report.json")" == "passed" ]] \
  || fail_env "stems probe status != passed: $(jq -r '.error' "$WORKDIR/stems_probe_report.json")"
[[ -f "$PROJECT_VIT" ]] || fail_env "project file missing after probe: $PROJECT_VIT"
log "project built: $PROJECT_VIT"

# -- synthetic public manifest (evaluator face) ----------------------------------
MANIFEST="$WORKDIR/public_manifest.json"
python3 - "$MANIFEST" "$PROJECT_VIT" "$STEMS_DIR" <<'PY'
import hashlib, json, sys
manifest_path, project_vit, stems_dir = sys.argv[1:4]
from pathlib import Path
stems = []
for wav in sorted(Path(stems_dir).glob("*.wav")):
    stems.append({
        "track": wav.stem,
        "file": str(wav),
        "sha256": hashlib.sha256(wav.read_bytes()).hexdigest(),
    })
if not stems:
    raise SystemExit("no stems found")
json.dump({
    "schema_version": "semantic_processor_agent_project_smoke_public_manifest.v1",
    "blindness_contract": {
        "sealed_truth_not_in_agent_context": True,
        "case_ids_are_semantically_opaque": True,
        "runner_reads_public_manifest_only": True,
        "same_neutral_prompt_for_all_projects": True,
        "preselected_track_forbidden": True,
        "preselected_plugin_forbidden": True,
    },
    "cases": [{
        "public_case_id": "spv1_p01",
        "project_path": project_vit,
        "stem_files": stems,
    }],
}, open(manifest_path, "w"), indent=1)
PY
[[ -f "$MANIFEST" ]] || fail_env "manifest synthesis failed"
log "manifest: $MANIFEST"

# -- narrow the static_eq whitelist to API-550A Stereo (backup + trap restore) ---
cp "$WHITELIST" "$WHITELIST_BACKUP"
python3 - "$WHITELIST" <<'PY' || { restore_whitelist; fail_env "whitelist narrowing failed"; }
import json, sys
path = sys.argv[1]
data = json.load(open(path))
eq = [row for row in data.get("static_eq", [])
      if row.get("plugin_name") == "API-550A Stereo"]
if len(eq) != 1:
    raise SystemExit(f"expected exactly one API-550A Stereo entry, found {len(eq)}")
data["static_eq"] = eq
json.dump(data, open(path, "w"), indent=1, ensure_ascii=False)
PY
log "whitelist narrowed: static_eq = [API-550A Stereo] (backup at $WHITELIST_BACKUP)"

# -- §8 rounds -------------------------------------------------------------------
FINAL_EXIT=1
LAST_BREAKPOINT=""
declare -a ROUND_RECORDS=()
for round in $(seq 1 "$MAX_RUNS"); do
  ROUND_DIR="$WORKDIR/round_$round"
  mkdir -p "$ROUND_DIR"
  log "== round $round/$MAX_RUNS: free_state_d1_smoke.py (frequency flavor, expect static_eq) =="
  python3 "$REPO_ROOT/scripts/free_state_d1_smoke.py" \
    --public-manifest "$MANIFEST" \
    --public-case-id spv1_p01 \
    --agent-http "$AGENT_HTTP" \
    --timeout-sec "$TIMEOUT_SECONDS" \
    --project-workdir "$ROUND_DIR/project" \
    --output "$ROUND_DIR/d1_smoke_report.json" \
    --prompt-flavor frequency \
    --expect-domain static_eq \
    > "$ROUND_DIR/runner_stdout.log" 2>&1
  rc=$?
  ROUND_RECORDS+=("round $round exit=$rc report=$ROUND_DIR/d1_smoke_report.json")
  cp "$WORKDIR/agent_state/agent_last.log" "$ROUND_DIR/agent_last.log" 2>/dev/null || true
  if [[ $rc -eq 0 ]]; then
    log "round $round PASS (exit 0) — readback verified on the locked 550A scenario"
    FINAL_EXIT=0
    break
  fi
  if [[ $rc -eq 3 ]]; then
    log "round $round NOT_EXERCISED (model did not select static_eq) — recorded, consuming a round"
    LAST_BREAKPOINT="not_exercised"
    continue
  fi
  # deterministic failure: classify the breakpoint from the report for stop-loss
  BP="$(jq -r '.status // .reason // "unknown"' "$ROUND_DIR/d1_smoke_report.json" 2>/dev/null | head -c 80)"
  log "round $round FAILED (exit $rc, breakpoint=$BP)"
  if [[ -n "$LAST_BREAKPOINT" && "$BP" == "$LAST_BREAKPOINT" && "$BP" != "not_exercised" ]]; then
    log "stop-loss: same deterministic breakpoint twice ($BP) — no third identical retry (§8)"
    break
  fi
  LAST_BREAKPOINT="$BP"
done

{
  echo "final_exit=$FINAL_EXIT"
  echo "rounds:"; printf '  %s\n' "${ROUND_RECORDS[@]:-}"
  echo "finished_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
} > "$WORKDIR/summary.txt"
cat "$WORKDIR/summary.txt" >&2

[[ "$FINAL_EXIT" -eq 0 ]] || fail_run "locked-550A d1 smoke did not pass in $MAX_RUNS rounds (see $WORKDIR)"
log "D1-550A LOCKED SMOKE PASS: artifacts in $WORKDIR"
exit 0
