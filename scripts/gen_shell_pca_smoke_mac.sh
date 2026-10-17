#!/usr/bin/env bash
# gen_shell_pca_smoke_mac.sh — GENSHELL-PCA-1 真栈烟测（mac，修订版·方案 B）
#
# registry 驱动 native 永久准入的全链断言：agent 启动幂等 seed 一条永久 v2
# attestation（绑定 GenShell.vst3 bundle 指纹），装载门/admission/audit 全走
# 现有查询路径，三道现行门（chat 认证入口/token 通道/typed coverage）零改动。
#
# 腿结构（一次运行全部执行，exit 0 = 全绿；§8 纪律：确定性断言，无 LLM）：
#
#   LEG A  seed 断言    — agent 启动后：v2 store 出现 generation_shell promoted
#                         记录（指纹=对部署 bundle 现算 sha256、identifier=
#                         内核枚举身份）；agent log 含 [pca-native-seed] seeded=true
#   LEG B  agent 装载   — 内核 scan 发现 GenShell → add_track → authority=
#                         full_project_access → POST /agent/invoke rack_add_node
#                         成功（agent_action_id/audit 留痕 + 内核 plugin 状态）
#   LEG C  revoke 即拒  — 运行时把 seed 记录改 revoked（store 文件）→ 同一
#                         agent 进程再装载 → pca_load_gate 错误串拒绝
#                         （token fallback 因隔离语义索引无匹配而拒——token
#                          通道本身语义不动）
#   LEG D  UI 路径回归  — revoke 态下直接内核命令 rack_add_node（不经 agent
#                         装载门）→ ok（UI/C++ 路径不受影响）
#   LEG E  重启 re-seed — 杀 agent → 重启 → store 出现新 promoted 记录（≠被
#                         revoke 的 id）+ log seeded=true → authority 重激活 →
#                         agent 装载恢复成功（native 家族会话级 revoke 语义）
#
# 隔离（AGENTS §10）：
#   - VIT_PROCESSOR_ATTESTATIONS_V2_PATH → run 目录（不碰 ~/.vit 真实 store）
#   - VIT_PLUGIN_SEMANTICS_PATH → run 目录空索引（token fallback 匹配不成功；
#     full-access 主路径不经语义索引）
#   - VIT_PROJECT_XML → run 目录工程副本；内核 cwd=仓库根（Settings 留在 checkout）
#   - VIT_ORCHESTRATION_STORE_PATH → run 目录
#
# Usage:
#   ./scripts/gen_shell_pca_smoke_mac.sh                    # 全部 leg
#   ./scripts/gen_shell_pca_smoke_mac.sh --leg A,B
#   ./scripts/gen_shell_pca_smoke_mac.sh --kernel-bin PATH
#   ./scripts/gen_shell_pca_smoke_mac.sh --vst3-bundle PATH
#                                                 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
#   ./scripts/gen_shell_pca_smoke_mac.sh --artifacts-root DIR
#                                                 (default ~/Documents/vit-genshell-artifacts)

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
VST3_BUNDLE="$HOME/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
ARTIFACTS_ROOT_DEFAULT="$HOME/Documents/vit-genshell-artifacts"
LEGS="A,B,C,D,E"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
AGENT_HTTP_ADDR="127.0.0.1:7878"
AGENT_HTTP="http://127.0.0.1:7878"
STARTUP_TIMEOUT_SECONDS=240
STOP_GRACE_SECONDS=10
SCAN_TIMEOUT_SECONDS=900

usage() {
  cat <<'EOF'
gen_shell_pca_smoke_mac.sh — GENSHELL-PCA-1 (方案 B) mac real-stack smoke.

Options:
  --repo-root PATH      Repository root (default: parent of this script's dir)
  --kernel-bin PATH     Kernel binary (default: repo VitApp/build/VitApp_artefacts/Debug/VitApp)
  --vst3-bundle PATH    Deployed GenShell.vst3 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
  --artifacts-root DIR  Artifacts root (default ~/Documents/vit-genshell-artifacts)
  --leg LIST            Comma subset of A,B,C,D,E (default all)
  -h, --help            Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --vst3-bundle) VST3_BUNDLE="$2"; shift 2 ;;
    --artifacts-root) ARTIFACTS_ROOT_DEFAULT="$2"; shift 2 ;;
    --leg) LEGS="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in python3 curl lsof shasum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done
python3 -c "import zmq" 2>/dev/null || { echo "ERROR[env]: python3 pyzmq not available" >&2; exit 2; }

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
ARTIFACTS_ROOT="$ARTIFACTS_ROOT_DEFAULT"

[[ -x "$KERNEL_BIN" ]] || { echo "ERROR[env]: kernel binary not executable: $KERNEL_BIN (pass --kernel-bin)" >&2; exit 2; }
[[ -d "$VST3_BUNDLE" ]] || { echo "ERROR[env]: GenShell.vst3 bundle not found: $VST3_BUNDLE (pass --vst3-bundle)" >&2; exit 2; }
command -v go >/dev/null 2>&1 || { echo "ERROR[env]: go toolchain not found" >&2; exit 2; }

RUN_ID="genshell_pca_mac_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/pca/$RUN_ID"
mkdir -p "$RUN_DIR"/{replies,logs,state,project,http}
PREREQ_FILE="$RUN_DIR/prereq.txt"

REAL_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REAL_DEFAULT_PROJECT" ]] || { echo "ERROR[env]: default project not found: $REAL_DEFAULT_PROJECT" >&2; exit 2; }

KERNEL_PID=""
AGENT_PID=""
OVERALL_PASS=1
SEED_ATTESTATION_ID=""
IDENTIFIER="VST3-GenShell-15b5622d-b818da1c"
ATTEST_STORE="$RUN_DIR/state/attestations.v2.json"
SEMANTIC_INDEX="$RUN_DIR/state/plugin_semantics_empty.json"
AGENT_LOG="$RUN_DIR/state/agent_last.log"

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" | tee -a "$PREREQ_FILE" >&2; }
bad()  { echo "RED: $*" | tee -a "$PREREQ_FILE" >&2; OVERALL_PASS=0; }
info() { echo "info: $*" | tee -a "$PREREQ_FILE" >&2; }
prereq() { echo "$1" | tee -a "$PREREQ_FILE" >&2; }

sha() { shasum -a 256 "$1" 2>/dev/null | cut -d' ' -f1; }

port_listener_pid() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true; }

zmq_send() {
  # zmq_send <json-file> <out-file> [timeout-ms] — one REQ socket per command
  local body="$1" out="$2" timeout_ms="${3:-30000}"
  python3 - "$body" "$out" "$KERNEL_PORT_REQ" "$timeout_ms" <<'PY'
import json, sys, zmq
body_path, out_path, port, timeout_ms = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
cmd = json.load(open(body_path, encoding="utf-8"))
ctx = zmq.Context.instance()
s = ctx.socket(zmq.REQ)
s.setsockopt(zmq.RCVTIMEO, timeout_ms)
s.setsockopt(zmq.LINGER, 0)
s.connect("tcp://127.0.0.1:%d" % port)
try:
    s.send_string(json.dumps(cmd, ensure_ascii=False))
    reply = s.recv_string()
    open(out_path, "w", encoding="utf-8").write(reply)
    print(reply[:400])
except Exception as e:
    open(out_path, "w", encoding="utf-8").write(json.dumps({"status": "error", "message": "zmq: %s" % e}))
    print("ZMQ_FAIL: %s" % e, file=sys.stderr)
    sys.exit(1)
finally:
    s.close()
PY
}

jfield() {
  python3 - "$1" "$2" <<'PY'
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8", errors="replace") as f:
        d = json.load(f)
    print(eval(sys.argv[2], {"d": d}))
except Exception:
    print("")
PY
}

wait_ports_free() {
  local deadline=$((SECONDS + 30)) busy
  while (( SECONDS < deadline )); do
    busy=""
    for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "${AGENT_HTTP_ADDR##*:}"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy="$busy $port"
    done
    [[ -z "$busy" ]] && return 0
    sleep 1
  done
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "${AGENT_HTTP_ADDR##*:}"; do
    [[ -n "$(port_listener_pid "$port")" ]] && prereq "port_still_busy=$port"
  done
  return 1
}

start_kernel() {
  wait_ports_free || { bad "ports busy before kernel start"; return 1; }
  (
    cd "$REPO_ROOT"
    exec env VIT_PROJECT_XML="$RUN_DIR/project/default_project.xml" "$KERNEL_BIN"
  ) > "$RUN_DIR/logs/kernel_main.log" 2>&1 &
  KERNEL_PID=$!
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || { bad "kernel exited during startup"; tail -5 "$RUN_DIR/logs/kernel_main.log" >&2; KERNEL_PID=""; return 1; }
    [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
    sleep 1
  done
  if [[ -z "$(port_listener_pid "$KERNEL_PORT_REQ")" ]]; then
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    wait "$KERNEL_PID" 2>/dev/null
    bad "kernel REQ port not up in ${STARTUP_TIMEOUT_SECONDS}s (child reaped)"
    KERNEL_PID=""
    return 1
  fi
  prereq "kernel_pid=$KERNEL_PID log=$RUN_DIR/logs/kernel_main.log"
  return 0
}

build_agent() {
  (cd "$REPO_ROOT/agent" && go build -o "$RUN_DIR/state/vitagent" ./cmd/vitagent) || { bad "go build agent failed"; return 1; }
  prereq "agent_bin=$RUN_DIR/state/vitagent sha256=$(sha "$RUN_DIR/state/vitagent")"
  return 0
}

start_agent() {
  (
    cd "$RUN_DIR/state"
    VIT_PROCESSOR_ATTESTATIONS_V2_PATH="$ATTEST_STORE" \
    VIT_PLUGIN_SEMANTICS_PATH="$SEMANTIC_INDEX" \
    VIT_ORCHESTRATION_STORE_PATH="$RUN_DIR/state/orchestration_v1.json" \
    exec "$RUN_DIR/state/vitagent" \
      -http "$AGENT_HTTP_ADDR" \
      -last-log-path "$AGENT_LOG" \
      -vsp-hub-url ""
  ) > "$RUN_DIR/logs/agent_stdout.log" 2>&1 &
  AGENT_PID=$!
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$AGENT_PID" 2>/dev/null || { bad "agent exited during startup"; tail -10 "$AGENT_LOG" "$RUN_DIR/logs/agent_stdout.log" >&2; AGENT_PID=""; return 1; }
    curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && break
    sleep 1
  done
  curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null || { bad "agent /health not ready"; AGENT_PID=""; return 1; }
  prereq "agent_pid=$AGENT_PID log=$AGENT_LOG"
  return 0
}

stop_agent() {
  [[ -n "$AGENT_PID" ]] || return 0
  local start=$SECONDS
  kill -TERM "$AGENT_PID" 2>/dev/null || true
  while kill -0 "$AGENT_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$AGENT_PID" 2>/dev/null || true; break; }
    sleep 1
  done
  wait "$AGENT_PID" 2>/dev/null
  prereq "agent_stop=SIGTERM pid_reaped"
  AGENT_PID=""
  return 0
}

stop_kernel() {
  [[ -n "$KERNEL_PID" ]] || return 0
  local start=$SECONDS
  kill -TERM "$KERNEL_PID" 2>/dev/null || true
  while kill -0 "$KERNEL_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$KERNEL_PID" 2>/dev/null || true; break; }
    sleep 1
  done
  wait "$KERNEL_PID" 2>/dev/null
  prereq "kernel_stop=SIGTERM elapsed=$((SECONDS - start))s"
  KERNEL_PID=""
  wait_ports_free || prereq "ports_busy_after_stop"
  return 0
}

emergency_teardown() {
  if [[ -n "$AGENT_PID" ]] && kill -0 "$AGENT_PID" 2>/dev/null; then
    kill -9 "$AGENT_PID" 2>/dev/null || true
    echo "EMERGENCY: agent $AGENT_PID SIGKILLed by EXIT trap" | tee -a "$PREREQ_FILE" >&2
  fi
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    echo "EMERGENCY: kernel $KERNEL_PID SIGKILLed by EXIT trap" | tee -a "$PREREQ_FILE" >&2
  fi
  AGENT_PID=""; KERNEL_PID=""
}
trap emergency_teardown EXIT INT TERM

has_leg() { [[ ",$LEGS," == *",$1,"* ]]; }

# agent 装载探针：add_track → authority full → invoke rack_add_node。
# 期望形态由 expect 控制：ok（装载成功，plugin 落轨）| gate（被 pca_load_gate 拒）。
agent_load_probe() {
  # agent_load_probe <tag> <expect ok|gate>
  local tag="$1" expect="$2"
  printf '{"cmd":"add_track"}' > "$RUN_DIR/replies/${tag}_add_track_req.json"
  zmq_send "$RUN_DIR/replies/${tag}_add_track_req.json" "$RUN_DIR/replies/${tag}_add_track.json" 60000 >/dev/null || true
  local track_id
  track_id="$(jfield "$RUN_DIR/replies/${tag}_add_track.json" 'str(d.get("track_id",""))')"
  prereq "${tag}_track_id=$track_id"
  if [[ -z "$track_id" ]]; then
    bad "$tag: add_track failed"
    return 1
  fi
  python3 - "$track_id" "$IDENTIFIER" > "$RUN_DIR/http/${tag}_invoke_req.json" <<'PY'
import json, sys
print(json.dumps({
    "command": {"cmd": "rack_add_node", "track_id": sys.argv[1], "plugin_identifier": sys.argv[2], "x": 0, "y": 0},
    "source": "gen_shell_pca_smoke_mac",
}))
PY
  local http_code
  http_code="$(curl -sS --max-time 180 -o "$RUN_DIR/http/${tag}_invoke.json" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' --data-binary @"$RUN_DIR/http/${tag}_invoke_req.json" \
    "$AGENT_HTTP/agent/invoke")" || true
  prereq "${tag}_invoke_http=$http_code"
  local status error_text action_id
  status="$(jfield "$RUN_DIR/http/${tag}_invoke.json" 'str(d.get("status",""))')"
  error_text="$(jfield "$RUN_DIR/http/${tag}_invoke.json" 'str(d.get("error",""))')"
  action_id="$(jfield "$RUN_DIR/http/${tag}_invoke.json" 'str(d.get("agent_action_id",""))')"
  prereq "${tag}_invoke status=$status action_id=$action_id error=$error_text"
  if [[ "$expect" == "ok" ]]; then
    if [[ "$status" != "error" && -n "$action_id" ]]; then
      ok "$tag: agent-path rack_add_node admitted (status=$status agent_action_id=$action_id — journal/audit face)"
      return 0
    fi
    bad "$tag: agent-path load did not admit: status=$status error=$error_text"
    return 1
  fi
  if [[ "$status" == "error" && "$error_text" == *"pca_load_gate"* ]]; then
    ok "$tag: agent-path load rejected by pca_load_gate (error=$(printf '%.120s' "$error_text"))"
    return 0
  fi
  bad "$tag: expected pca_load_gate rejection, got status=$status error=$error_text"
  return 1
}

activate_full_access() {
  local http_code body
  http_code="$(curl -sS --max-time 10 -o "$RUN_DIR/http/authority_activate.json" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' --data '{"authority_mode":"full_project_access"}' \
    "$AGENT_HTTP/agent/authority")" || true
  body="$(jfield "$RUN_DIR/http/authority_activate.json" 'str(d.get("authority_mode", d.get("error","")))')"
  prereq "authority_activate http=$http_code body=$body"
  [[ "$http_code" == 200 && "$body" == "full_project_access" ]]
}

seeded_promoted_id() {
  # prints the promoted generation_shell attestation id from the isolated store
  python3 - "$ATTEST_STORE" <<'PY'
import json, sys
try:
    lib = json.load(open(sys.argv[1], encoding="utf-8"))
    for a in lib.get("attestations", []):
        if a.get("processor_family") == "generation_shell" and a.get("status") == "promoted":
            print(a.get("attestation_id", ""))
            break
except Exception:
    pass
PY
}

# ================================================================ preflight
step "Preflight"
if pgrep -f "$KERNEL_BIN" >/dev/null 2>&1; then
  echo "ERROR[env]: a kernel of this binary is already running (pgrep -f $KERNEL_BIN) — single-owner rule, aborting" >&2
  exit 2
fi
prereq "no_running_kernel_at_start=confirmed"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%F %T')"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ')"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$(sha "$KERNEL_BIN")"
  echo "kernel_mtime=$(stat -f %Sm "$KERNEL_BIN")"
  echo "vst3_bundle=$VST3_BUNDLE"
  echo "vst3_bundle_binary_sha256=$(sha "$VST3_BUNDLE/Contents/MacOS/GenShell" 2>/dev/null || echo missing)"
  echo "legs=$LEGS"
} > "$RUN_DIR/run_meta.txt"
cat "$RUN_DIR/run_meta.txt" >&2

cp "$REAL_DEFAULT_PROJECT" "$RUN_DIR/project/default_project.xml"
prereq "project_copy_sha256=$(sha "$RUN_DIR/project/default_project.xml") (source project never written; VIT_PROJECT_XML redirect)"

# 隔离语义索引（空 entries）：token fallback 在此环境下无法解析唯一身份 → 拒；
# full-access 主路径不经语义索引。token 通道自身语义零改动。
printf '{"schema_version":"plugin_semantics.v1","built_at":"2026-10-16T00:00:00Z","source":"gen_shell_pca_smoke_isolation","entries":[],"summary":{}}' > "$SEMANTIC_INDEX"
prereq "semantic_index_isolated=$SEMANTIC_INDEX (empty entries — token fallback probe isolation)"

build_agent || exit 2

step "Kernel + agent start"
KERNEL_OK=1
start_kernel || KERNEL_OK=0
if [[ "$KERNEL_OK" -eq 1 ]]; then
  sleep 5  # edit settle
  start_agent || KERNEL_OK=0
fi
[[ "$KERNEL_OK" -eq 1 ]] || { stop_kernel; exit 1; }

# ================================================================ LEG A
if has_leg A; then
  step "LEG A: startup seed -> promoted v2 attestation bound to bundle fingerprint"
  sleep 2  # seed runs at process start; give the log a moment
  SEED_ATTESTATION_ID="$(seeded_promoted_id)"
  BUNDLE_FINGERPRINT="$(cd "$REPO_ROOT/agent" && go run ./cmd/pcactl fingerprint -path "$VST3_BUNDLE" 2>/dev/null | jfield /dev/stdin 'str(d.get("binary_fingerprint",""))' 2>/dev/null || true)"
  if [[ -z "$BUNDLE_FINGERPRINT" ]]; then
    BUNDLE_FINGERPRINT="$(python3 - "$VST3_BUNDLE" <<'PY'
import hashlib, os, sys
root = sys.argv[1]
files = []
for base, _dirs, names in os.walk(root):
    for n in names:
        p = os.path.join(base, n)
        if os.path.islink(p):
            continue
        if os.path.isfile(p):
            files.append(p)
files.sort()
h = hashlib.sha256()
h.update(b"vit-pca-bundle-v1\x00")
for p in files:
    rel = os.path.relpath(p, root).replace(os.sep, "/")
    data = open(p, "rb").read()
    h.update(len(rel.encode()).to_bytes(8, "big"))
    h.update(rel.encode())
    h.update(len(data).to_bytes(8, "big"))
    h.update(data)
print("sha256:" + h.hexdigest())
PY
)"
  fi
  prereq "bundle_fingerprint=$BUNDLE_FINGERPRINT"
  STORE_CHECK="$(python3 - "$ATTEST_STORE" "$BUNDLE_FINGERPRINT" "$IDENTIFIER" <<'PY'
import json, sys
lib = json.load(open(sys.argv[1], encoding="utf-8"))
fp, ident = sys.argv[2], sys.argv[3]
for a in lib.get("attestations", []):
    if a.get("processor_family") != "generation_shell":
        continue
    subject = a.get("subject", {})
    print("%s|%s|%s|%s|%s" % (
        a.get("attestation_id", ""), a.get("status", ""), a.get("binary_fingerprint", ""),
        subject.get("identifier", ""), subject.get("installed_path", "")))
    _ = fp, ident
PY
)"
  IFS='|' read -r STORE_ID STORE_STATUS STORE_FP STORE_IDENT STORE_PATH <<< "$STORE_CHECK"
  prereq "seed_store id=$STORE_ID status=$STORE_STATUS fingerprint=$STORE_FP identifier=$STORE_IDENT"
  A_OK=1
  [[ -n "$STORE_ID" ]] || { bad "LEG A: no generation_shell attestation in isolated store"; A_OK=0; }
  [[ "$STORE_STATUS" == "promoted" ]] || { bad "LEG A: seed record status=$STORE_STATUS (expected promoted)"; A_OK=0; }
  [[ "$STORE_FP" == "$BUNDLE_FINGERPRINT" ]] || { bad "LEG A: seed fingerprint mismatch store=$STORE_FP computed=$BUNDLE_FINGERPRINT"; A_OK=0; }
  [[ "$STORE_IDENT" == "$IDENTIFIER" ]] || { bad "LEG A: seed identifier=$STORE_IDENT (expected $IDENTIFIER)"; A_OK=0; }
  SEED_ATTESTATION_ID="$STORE_ID"
  if grep -q "\[pca-native-seed\] family=generation_shell .*seeded=true" "$AGENT_LOG" 2>/dev/null; then
    ok "LEG A: agent log carries [pca-native-seed] seeded=true ($(grep -o '\[pca-native-seed\] family=generation_shell[^ ]* identifier=[^ ]* seeded=true' "$AGENT_LOG" | head -1))"
  else
    bad "LEG A: [pca-native-seed] seeded=true line missing from agent log"
    A_OK=0
  fi
  [[ "$A_OK" -eq 1 ]] && ok "LEG A: permanent native admission seeded and bound to deployed bundle (id=$STORE_ID)"
fi

# ================================================================ 内核发现（B/C/E 的前置）
step "Kernel scan: discover GenShell"
USER_VST3_DIR="$HOME/Library/Audio/Plug-Ins/VST3"
printf '{"cmd":"scan_plugins","paths":["%s"]}' "$USER_VST3_DIR" > "$RUN_DIR/replies/scan_req.json"
zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/scan_start.json" 60000 || true
SCAN_ID="$(jfield "$RUN_DIR/replies/scan_start.json" 'str(d.get("scan_id",""))')"
STATE=""
deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
while (( SECONDS < deadline )); do
  printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/status_req.json"
  zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/scan_status.json" 30000 >/dev/null || true
  STATE="$(jfield "$RUN_DIR/replies/scan_status.json" 'str(d.get("status",""))')"
  [[ "$STATE" != "scanning" && "$STATE" != "cancelling" ]] && break
  sleep 3
done
prereq "scan_terminal=$STATE"
printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/list_req.json"
zmq_send "$RUN_DIR/replies/list_req.json" "$RUN_DIR/replies/list.json" 60000 || true
KERNEL_IDENT="$(python3 - "$RUN_DIR/replies/list.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("plugins") or []:
        if p.get("name") == "GenShell":
            print(p.get("identifier", ""))
            break
except Exception:
    pass
PY
)"
if [[ "$KERNEL_IDENT" == *"GenShell"* ]]; then
  ok "scan: GenShell discovered by kernel (identifier=$KERNEL_IDENT)"
  if [[ "$KERNEL_IDENT" != "$IDENTIFIER" ]]; then
    bad "kernel identifier drifted from registry declaration: kernel=$KERNEL_IDENT registry=$IDENTIFIER"
  fi
else
  bad "scan: GenShell not discovered by kernel"
fi

# ================================================================ LEG B
if has_leg B; then
  step "LEG B: agent-path rack_add_node admitted via seeded PCA admission"
  if activate_full_access; then
    ok "authority: full_project_access activated"
  else
    bad "authority: full_project_access activation failed"
  fi
  agent_load_probe B ok || true
fi

# ================================================================ LEG C
if has_leg C; then
  step "LEG C: runtime revoke -> same-process agent load rejected (pca_load_gate)"
  if [[ -z "$SEED_ATTESTATION_ID" ]]; then
    SEED_ATTESTATION_ID="$(seeded_promoted_id)"
  fi
  if [[ -n "$SEED_ATTESTATION_ID" ]]; then
    python3 - "$ATTEST_STORE" "$SEED_ATTESTATION_ID" <<'PY'
import datetime, json, sys
path, att_id = sys.argv[1], sys.argv[2]
lib = json.load(open(path, encoding="utf-8"))
for a in lib.get("attestations", []):
    if a.get("attestation_id") == att_id:
        a["status"] = "revoked"
        a["status_reason"] = "smoke_runtime_revoke"
        a["revoked_at"] = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
json.dump(lib, open(path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
open(path, "a", encoding="utf-8").write("\n")
print("revoked", att_id)
PY
    ok "revoke: seed record $SEED_ATTESTATION_ID flipped to revoked in isolated store"
    agent_load_probe C gate || true
  else
    bad "revoke: no promoted seed id known — LEG C prerequisite missing"
  fi
fi

# ================================================================ LEG D
if has_leg D; then
  step "LEG D: UI path (direct kernel command) unaffected while agent gate is closed"
  printf '{"cmd":"add_track"}' > "$RUN_DIR/replies/D_add_track_req.json"
  zmq_send "$RUN_DIR/replies/D_add_track_req.json" "$RUN_DIR/replies/D_add_track.json" 60000 || true
  D_TRACK="$(jfield "$RUN_DIR/replies/D_add_track.json" 'str(d.get("track_id",""))')"
  if [[ -n "$D_TRACK" ]]; then
    python3 - "$D_TRACK" "$IDENTIFIER" > "$RUN_DIR/replies/D_rack_add_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node",
                  "track_id": sys.argv[1], "plugin_identifier": sys.argv[2],
                  "x": 0, "y": 0}))
PY
    zmq_send "$RUN_DIR/replies/D_rack_add_req.json" "$RUN_DIR/replies/D_rack_add.json" 180000 || true
    D_STATUS="$(jfield "$RUN_DIR/replies/D_rack_add.json" 'str(d.get("status",""))')"
    D_PLUGIN="$(jfield "$RUN_DIR/replies/D_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
    prereq "D_rack_add status=$D_STATUS plugin_id=$D_PLUGIN"
    if [[ "$D_STATUS" == "ok" && -n "$D_PLUGIN" ]]; then
      ok "LEG D: UI-path rack_add_node ok under revoked agent admission (plugin_id=$D_PLUGIN — UI/C++ face independent of Harness gate)"
    else
      bad "LEG D: UI-path rack_add_node failed: status=$D_STATUS"
    fi
  else
    bad "LEG D: add_track failed"
  fi
fi

# ================================================================ LEG E
if has_leg E; then
  step "LEG E: agent restart -> re-seed -> load recovers (session-scoped revoke)"
  stop_agent
  start_agent || { bad "LEG E: agent restart failed"; }
  if [[ -n "$AGENT_PID" ]]; then
    sleep 2
    RESEEDED_ID="$(seeded_promoted_id)"
    prereq "reseeded_id=$RESEEDED_ID (revoked=$SEED_ATTESTATION_ID)"
    E_OK=1
    [[ -n "$RESEEDED_ID" ]] || { bad "LEG E: no promoted record after restart"; E_OK=0; }
    [[ "$RESEEDED_ID" != "$SEED_ATTESTATION_ID" ]] || { bad "LEG E: re-seed reused the revoked record id"; E_OK=0; }
    if grep -q "\[pca-native-seed\] family=generation_shell .*seeded=true" "$AGENT_LOG" 2>/dev/null; then
      ok "LEG E: restart log carries [pca-native-seed] seeded=true"
    else
      bad "LEG E: restart log missing seeded=true (store state=$(seeded_promoted_id))"
      E_OK=0
    fi
    if activate_full_access; then
      ok "authority: full_project_access re-activated after restart"
    else
      bad "authority: re-activation failed after restart"
      E_OK=0
    fi
    agent_load_probe E ok || E_OK=0
    [[ "$E_OK" -eq 1 ]] && ok "LEG E: native admission re-seeded and load recovered (id=$RESEEDED_ID)"
  fi
fi

# ================================================================ teardown
step "Teardown"
stop_agent
stop_kernel

# ================================================================ summary
step "Summary"
python3 - "$RUN_DIR" "$RUN_ID" "$LEGS" <<'PY'
import json, os, sys
run_dir, run_id, legs = sys.argv[1], sys.argv[2], sys.argv[3]
ok_lines, red_lines = [], []
p = os.path.join(run_dir, "prereq.txt")
if os.path.isfile(p):
    for line in open(p, encoding="utf-8", errors="replace"):
        line = line.rstrip("\n")
        if line.startswith("ok: "):
            ok_lines.append(line[4:])
        elif line.startswith("RED: "):
            red_lines.append(line[5:])
report = {
    "schema_version": "vit_gen_shell_pca_smoke.mac.v1",
    "card": "GENSHELL-PCA-1",
    "run_id": run_id,
    "run_root": run_dir,
    "legs_requested": legs,
    "ok_count": len(ok_lines),
    "red_count": len(red_lines),
    "ok": ok_lines,
    "red": red_lines,
    "verdict": "all_green" if not red_lines else "assertions_red",
}
out = os.path.join(run_dir, "pca_report.json")
json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("report: %s" % out, file=sys.stderr)
print("verdict: %s (ok=%d red=%d)" % (report["verdict"], len(ok_lines), len(red_lines)), file=sys.stderr)
PY
prereq "finished=$(date '+%F %T')"
log "workdir: $RUN_DIR"
if [[ "$OVERALL_PASS" -eq 1 ]]; then
  echo "GENSHELL_PCA_MAC_VERDICT all_green" >&2
  exit 0
else
  echo "GENSHELL_PCA_MAC_VERDICT assertions_red" >&2
  exit 1
fi
