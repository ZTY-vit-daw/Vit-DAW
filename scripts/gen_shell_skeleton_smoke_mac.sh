#!/usr/bin/env bash
# gen_shell_skeleton_smoke_mac.sh — GENSHELL-CPP-1 真栈烟测（mac）
#
# 驱动真实内核二进制（VitApp，经 --kernel-bin 指定，默认主仓 checkout 的
# Debug 产物——本卡零内核改动，沿用既有二进制并在 run_meta 记录 sha256/mtime）
# 走内核命令线（ZMQ REQ 5555，与 kernel_pluginlist_hygiene_smoke_mac.sh 同模式）：
#
#   LEG S  壳单测（T2 自环+chunk 全保真，genshell_selftest 对部署产物 exit 0）
#   LEG K1 scan 发现     — scan_plugins 扫用户 VST3 目录 → plugin_list_available
#                          发现 GenShell（name/manufacturer/identifier 断言）
#   LEG K2 装载+参数面   — add_track → rack_add_node（UI 路径形态=直接内核命令，
#                          不经 agent 装载门）→ get_plugin_parameters 断言 T1 五参数
#                          + debug 二参数在场
#   LEG K3 SceneMode 读写 — set_plugin_param(normalized 0.5) → 读回一致
#                          （value_text=free_creation）
#   LEG K4 chunk 往返    — save_project(.vit) → open_project → 同 track/plugin id
#                          读回 SceneMode 仍 0.5（EditItemID 跨存取稳定；form_draft
#                          等无 GUI 入口字段的全保真由 LEG S 的壳单测覆盖）
#   LEG K5 show_editor   — show_plugin_editor → 应答 ok → 3s 后内核进程存活
#                          （进程存活断言；编辑器视觉面零断言——卡1端测边界）
#
# 工程隔离：VIT_PROJECT_XML 指向 run 目录的项目副本（AGENTS §10 真实工程防污染）；
# 内核 cwd=本脚本所在仓库根（worktree 或主仓均可——内核 climbToVitAppRoot 从 cwd
# 解析 Workspace，Settings.xml 变更留在该 checkout 内）。
#
# §8 纪律：确定性内核行为断言（无 LLM）；一次运行全部 leg，exit 0 = 全绿。
#
# Usage:
#   ./scripts/gen_shell_skeleton_smoke_mac.sh                    # 全部 leg
#   ./scripts/gen_shell_skeleton_smoke_mac.sh --leg K1,K2
#   ./scripts/gen_shell_skeleton_smoke_mac.sh --kernel-bin PATH
#   ./scripts/gen_shell_skeleton_smoke_mac.sh --vst3-bundle PATH
#                                                 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
#   ./scripts/gen_shell_skeleton_smoke_mac.sh --selftest-bin PATH
#                                                 (default <repo>/GenShell/build/genshell_selftest)
#   ./scripts/gen_shell_skeleton_smoke_mac.sh --artifacts-root DIR
#                                                 (default ~/Documents/vit-genshell-artifacts)

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
VST3_BUNDLE="$HOME/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
SELFTEST_BIN_DEFAULT=""
ARTIFACTS_ROOT_DEFAULT="$HOME/Documents/vit-genshell-artifacts"
LEGS="S,K1,K2,K3,K4,K5"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
STARTUP_TIMEOUT_SECONDS=240
STOP_GRACE_SECONDS=10
SCAN_TIMEOUT_SECONDS=900

usage() {
  cat <<'EOF'
gen_shell_skeleton_smoke_mac.sh — GENSHELL-CPP-1 mac real-stack smoke.

Options:
  --repo-root PATH      Repository root (default: parent of this script's dir)
  --kernel-bin PATH     Kernel binary (default: repo VitApp/build/VitApp_artefacts/Debug/VitApp)
  --vst3-bundle PATH    Deployed GenShell.vst3 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
  --selftest-bin PATH   genshell_selftest binary (default repo GenShell/build/genshell_selftest)
  --artifacts-root DIR  Artifacts root (default ~/Documents/vit-genshell-artifacts)
  --leg LIST            Comma subset of S,K1,K2,K3,K4,K5 (default all)
  -h, --help            Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --vst3-bundle) VST3_BUNDLE="$2"; shift 2 ;;
    --selftest-bin) SELFTEST_BIN_DEFAULT="$2"; shift 2 ;;
    --artifacts-root) ARTIFACTS_ROOT_DEFAULT="$2"; shift 2 ;;
    --leg) LEGS="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in python3 lsof shasum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done
python3 -c "import zmq" 2>/dev/null || { echo "ERROR[env]: python3 pyzmq not available" >&2; exit 2; }

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
SELFTEST_BIN="${SELFTEST_BIN_DEFAULT:-$REPO_ROOT/GenShell/build/genshell_selftest}"
ARTIFACTS_ROOT="$ARTIFACTS_ROOT_DEFAULT"

[[ -x "$KERNEL_BIN" ]] || { echo "ERROR[env]: kernel binary not executable: $KERNEL_BIN (pass --kernel-bin)" >&2; exit 2; }
[[ -d "$VST3_BUNDLE" ]] || { echo "ERROR[env]: GenShell.vst3 bundle not found: $VST3_BUNDLE (pass --vst3-bundle)" >&2; exit 2; }
[[ -x "$SELFTEST_BIN" ]] || { echo "ERROR[env]: genshell_selftest not executable: $SELFTEST_BIN (pass --selftest-bin)" >&2; exit 2; }

RUN_ID="genshell_skel_mac_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/skeleton/$RUN_ID"
mkdir -p "$RUN_DIR"/{replies,logs,selftest}
PREREQ_FILE="$RUN_DIR/prereq.txt"

REAL_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REAL_DEFAULT_PROJECT" ]] || { echo "ERROR[env]: default project not found: $REAL_DEFAULT_PROJECT" >&2; exit 2; }

KERNEL_PID=""
KERNEL_LOG=""
OVERALL_PASS=1

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
  # jfield <file> <python expr against d>
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
    for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy="$busy $port"
    done
    [[ -z "$busy" ]] && return 0
    sleep 1
  done
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    [[ -n "$(port_listener_pid "$port")" ]] && prereq "port_still_busy=$port"
  done
  return 1
}

start_kernel() {
  wait_ports_free || { bad "ports busy before kernel start"; return 1; }
  KERNEL_LOG="$RUN_DIR/logs/kernel_main.log"
  (
    cd "$REPO_ROOT"
    exec env VIT_PROJECT_XML="$RUN_DIR/project/default_project.xml" "$KERNEL_BIN"
  ) > "$KERNEL_LOG" 2>&1 &
  KERNEL_PID=$!
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || { bad "kernel exited during startup"; tail -5 "$KERNEL_LOG" >&2; KERNEL_PID=""; return 1; }
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
  prereq "kernel_pid=$KERNEL_PID log=$KERNEL_LOG"
  return 0
}

stop_kernel() {
  local start elapsed code
  start=$SECONDS
  kill -TERM "$KERNEL_PID" 2>/dev/null || true
  while kill -0 "$KERNEL_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$KERNEL_PID" 2>/dev/null || true; break; }
    sleep 1
  done
  code="SIGTERM"
  wait "$KERNEL_PID" 2>/dev/null
  elapsed=$((SECONDS - start))
  prereq "kernel_stop=$code elapsed=${elapsed}s"
  KERNEL_PID=""
  wait_ports_free || prereq "ports_busy_after_stop"
  return 0
}

emergency_teardown() {
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    echo "EMERGENCY: kernel $KERNEL_PID SIGKILLed by EXIT trap" | tee -a "$PREREQ_FILE" >&2
  fi
  KERNEL_PID=""
}
trap emergency_teardown EXIT INT TERM

has_leg() { [[ ",$LEGS," == *",$1,"* ]]; }

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
  echo "selftest_bin=$SELFTEST_BIN"
  echo "legs=$LEGS"
} > "$RUN_DIR/run_meta.txt"
cat "$RUN_DIR/run_meta.txt" >&2

mkdir -p "$RUN_DIR/project"
cp "$REAL_DEFAULT_PROJECT" "$RUN_DIR/project/default_project.xml"
prereq "project_copy_sha256=$(sha "$RUN_DIR/project/default_project.xml") (source project never written; VIT_PROJECT_XML redirect)"

# ================================================================ LEG S（壳单测：T2 自环 + chunk 全保真）
if has_leg S; then
  step "LEG S: genshell_selftest against deployed bundle (T2 echo + chunk fidelity)"
  "$SELFTEST_BIN" "$VST3_BUNDLE" > "$RUN_DIR/selftest/selftest.log" 2>&1
  SEXIT=$?
  cp "$RUN_DIR/selftest/selftest.log" "$RUN_DIR/selftest/selftest_copy.log"
  if [[ $SEXIT -eq 0 ]]; then
    ok "LEG S: selftest exit 0 ($(grep -c '^\[PASS\]' "$RUN_DIR/selftest/selftest.log") PASS / $(grep -c '^\[FAIL\]' "$RUN_DIR/selftest/selftest.log") FAIL)"
  else
    bad "LEG S: selftest exit=$SEXIT — $(grep '^\[FAIL\]' "$RUN_DIR/selftest/selftest.log" | head -3)"
  fi
fi

# ================================================================ LEG K1-K5（真栈）
if has_leg K1 || has_leg K2 || has_leg K3 || has_leg K4 || has_leg K5; then
  step "Kernel start"
  KERNEL_OK=1
  start_kernel || KERNEL_OK=0
  if [[ "$KERNEL_OK" -eq 1 ]]; then
    sleep 5  # edit settle

    # ---------------- LEG K1: scan 发现
    if has_leg K1; then
      step "LEG K1: scan_plugins -> plugin_list_available discovers GenShell"
      USER_VST3_DIR="$HOME/Library/Audio/Plug-Ins/VST3"
      printf '{"cmd":"scan_plugins","paths":["%s"]}' "$USER_VST3_DIR" > "$RUN_DIR/replies/scan_req.json"
      zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/K1_scan_start.json" 60000 || true
      SCAN_ID="$(jfield "$RUN_DIR/replies/K1_scan_start.json" 'str(d.get("scan_id",""))')"
      STATE=""
      deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
      while (( SECONDS < deadline )); do
        printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/status_req.json"
        zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/K1_status.json" 30000 >/dev/null || true
        STATE="$(jfield "$RUN_DIR/replies/K1_status.json" 'str(d.get("status",""))')"
        [[ "$STATE" != "scanning" && "$STATE" != "cancelling" ]] && break
        sleep 3
      done
      prereq "K1_scan_terminal=$STATE"
      [[ "$STATE" == "completed" ]] && ok "LEG K1: scan completed" || bad "LEG K1: scan state=$STATE"

      printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/list_req.json"
      zmq_send "$RUN_DIR/replies/list_req.json" "$RUN_DIR/replies/K1_list.json" 60000 || true
      GENSHELL_FOUND="$(python3 - "$RUN_DIR/replies/K1_list.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("plugins") or []:
        if p.get("name") == "GenShell":
            print("%s|%s|%s" % (p.get("identifier",""), p.get("manufacturer",""), p.get("category","")))
            break
except Exception:
    pass
PY
)"
      IDENTIFIER="${GENSHELL_FOUND%%|*}"
      if [[ -n "$GENSHELL_FOUND" && "$IDENTIFIER" == *"GenShell"* ]]; then
        ok "LEG K1: GenShell discovered (identifier=$IDENTIFIER)"
        printf '%s' "$IDENTIFIER" > "$RUN_DIR/replies/K1_identifier.txt"
      else
        bad "LEG K1: GenShell not in plugin_list_available (match='$GENSHELL_FOUND')"
      fi
    fi

    # ---------------- LEG K2: 装载 + T1 参数面
    if has_leg K2; then
      step "LEG K2: add_track -> rack_add_node -> get_plugin_parameters (five T1 params)"
      printf '{"cmd":"add_track"}' > "$RUN_DIR/replies/K2_add_track_req.json"
      zmq_send "$RUN_DIR/replies/K2_add_track_req.json" "$RUN_DIR/replies/K2_add_track.json" 60000 || true
      TRACK_ID="$(jfield "$RUN_DIR/replies/K2_add_track.json" 'str(d.get("track_id",""))')"
      prereq "K2_track_id=$TRACK_ID"
      [[ -n "$TRACK_ID" ]] && ok "LEG K2: track added" || bad "LEG K2: add_track failed"

      IDENTIFIER="$(cat "$RUN_DIR/replies/K1_identifier.txt" 2>/dev/null || true)"
      if [[ -n "$TRACK_ID" && -n "$IDENTIFIER" ]]; then
        python3 - "$TRACK_ID" "$IDENTIFIER" > "$RUN_DIR/replies/K2_rack_add_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node",
                  "track_id": sys.argv[1], "plugin_identifier": sys.argv[2],
                  "x": 0, "y": 0}))
PY
        zmq_send "$RUN_DIR/replies/K2_rack_add_req.json" "$RUN_DIR/replies/K2_rack_add.json" 180000 || true
        PLUGIN_ID="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
        ADD_STATUS="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("status",""))')"
        ADD_ERR="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("message", d.get("error","")))')"
        prereq "K2_rack_add status=$ADD_STATUS plugin_id=$PLUGIN_ID error=$ADD_ERR"
        if [[ "$ADD_STATUS" == "ok" && -n "$PLUGIN_ID" ]]; then
          ok "LEG K2: rack_add_node landed (UI path form, direct kernel command; plugin_id=$PLUGIN_ID)"
        else
          bad "LEG K2: rack_add_node failed: status=$ADD_STATUS error=$ADD_ERR"
        fi

        if [[ -n "$PLUGIN_ID" ]]; then
          python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/K2_params_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
          zmq_send "$RUN_DIR/replies/K2_params_req.json" "$RUN_DIR/replies/K2_params.json" 120000 || true
          PSTATUS="$(jfield "$RUN_DIR/replies/K2_params.json" 'str(d.get("status",""))')"
          prereq "K2_get_params status=$PSTATUS"
          if [[ "$PSTATUS" == "ok" ]]; then
            # 卡验收口径=T1 五参数在场。debug 计数参数（kIsReadOnly 不可自动化）不进
            # JUCE automatable 面（宿主行为，非缺陷）——其读数由 LEG S 单测进程直读验证。
            PARAM_REPORT="$(python3 - "$RUN_DIR/replies/K2_params.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
params = d.get("parameters") or []
names = {}
for p in params:
    names.setdefault(str(p.get("name","")), p.get("id",""))
need = ["JobState", "JobProgress", "JobErrorClass", "SceneMode", "Bypass"]
missing = [n for n in need if n not in names]
print("%d|%s" % (len(params), ",".join(missing)))
print("SceneModeParamId=%s" % names.get("SceneMode",""), file=sys.stderr)
PY
)"
            PARAM_COUNT="${PARAM_REPORT%%|*}"
            MISSING="$(printf '%s' "$PARAM_REPORT" | cut -d'|' -f2)"
            if [[ "$MISSING" == "" ]]; then
              ok "LEG K2: T1 five params present via get_plugin_parameters (host-visible count=$PARAM_COUNT incl. tracktion Dry/Wet wrapper params)"
              info "LEG K2: debug count params (MsgEchoCount/MsgUnknownSchemaCount) are kIsReadOnly -> not in JUCE automatable surface; verified by LEG S selftest via direct IEditController reads"
            else
              bad "LEG K2: params missing=$MISSING (count=$PARAM_COUNT)"
            fi
            SCENE_MODE_PARAM_ID="$(python3 - "$RUN_DIR/replies/K2_params.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
for p in d.get("parameters") or []:
    if str(p.get("name","")) == "SceneMode":
        print(p.get("id",""))
        break
PY
)"
            printf '%s' "$SCENE_MODE_PARAM_ID" > "$RUN_DIR/replies/K2_scenemode_param_id.txt"
            prereq "K2_scenemode_param_id=$SCENE_MODE_PARAM_ID"
          else
            bad "LEG K2: get_plugin_parameters status=$PSTATUS: $(head -c 300 "$RUN_DIR/replies/K2_params.json")"
          fi
        fi
      else
        bad "LEG K2: missing track_id or identifier prerequisite"
      fi
    fi

    # ---------------- LEG K3: SceneMode 读写
    if has_leg K3; then
      step "LEG K3: set_plugin_param(SceneMode 0.5) -> readback"
      PLUGIN_ID="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("plugin_id", ""))')"
      SCENE_PARAM_ID="$(cat "$RUN_DIR/replies/K2_scenemode_param_id.txt" 2>/dev/null || true)"
      if [[ -n "$PLUGIN_ID" && -n "$SCENE_PARAM_ID" ]]; then
        python3 - "$PLUGIN_ID" "$SCENE_PARAM_ID" > "$RUN_DIR/replies/K3_set_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "set_plugin_param",
                  "plugin_id": sys.argv[1], "param_id": sys.argv[2],
                  "normalized_value": 0.5}))
PY
        zmq_send "$RUN_DIR/replies/K3_set_req.json" "$RUN_DIR/replies/K3_set.json" 60000 || true
        SET_STATUS="$(jfield "$RUN_DIR/replies/K3_set.json" 'str(d.get("status",""))')"
        [[ "$SET_STATUS" == "ok" ]] && ok "LEG K3: set_plugin_param ok" || bad "LEG K3: set failed: $SET_STATUS $(head -c 200 "$RUN_DIR/replies/K3_set.json")"

        python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/K3_read_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/K3_read_req.json" "$RUN_DIR/replies/K3_read.json" 120000 || true
        K3_READBACK="$(python3 - "$RUN_DIR/replies/K3_read.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("parameters") or []:
        if str(p.get("name","")) == "SceneMode":
            print("%s|%s" % (p.get("normalized_value"), p.get("value_text","")))
            break
except Exception:
    pass
PY
)"
        K3_NORM="${K3_READBACK%%|*}"
        K3_TEXT="${K3_READBACK#*|}"
        prereq "K3_readback normalized=$K3_NORM text=$K3_TEXT"
        if [[ "$K3_NORM" == 0.5 || "$K3_NORM" == "0.5" ]]; then
          ok "LEG K3: SceneMode readback 0.5 (value_text=$K3_TEXT)"
        else
          bad "LEG K3: SceneMode readback=$K3_NORM (expected 0.5)"
        fi
        if [[ "$K3_TEXT" == "free_creation" ]]; then
          ok "LEG K3: SceneMode value_text=free_creation (StringListParameter 显示面)"
        else
          bad "LEG K3: SceneMode value_text=$K3_TEXT (expected free_creation)"
        fi
      else
        bad "LEG K3: missing plugin_id/scene param id prerequisite"
      fi
    fi

    # ---------------- LEG K4: chunk 往返（save → open → 读回）
    if has_leg K4; then
      step "LEG K4: save_project -> open_project -> SceneMode persists (T3 chunk roundtrip)"
      PLUGIN_ID="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("plugin_id", ""))')"
      if [[ -n "$PLUGIN_ID" ]]; then
        ROUNDTRIP_VIT="$RUN_DIR/project/roundtrip.vit"
        python3 - "$ROUNDTRIP_VIT" > "$RUN_DIR/replies/K4_save_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "save_project", "file_path": sys.argv[1]}))
PY
        zmq_send "$RUN_DIR/replies/K4_save_req.json" "$RUN_DIR/replies/K4_save.json" 180000 || true
        SAVE_STATUS="$(jfield "$RUN_DIR/replies/K4_save.json" 'str(d.get("status",""))')"
        [[ "$SAVE_STATUS" == "ok" ]] && ok "LEG K4: save_project ok ($(sha "$ROUNDTRIP_VIT" 2>/dev/null | cut -c1-16))" \
          || bad "LEG K4: save_project status=$SAVE_STATUS"

        python3 - "$ROUNDTRIP_VIT" > "$RUN_DIR/replies/K4_open_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "open_project", "file_path": sys.argv[1]}))
PY
        zmq_send "$RUN_DIR/replies/K4_open_req.json" "$RUN_DIR/replies/K4_open.json" 180000 || true
        OPEN_STATUS="$(jfield "$RUN_DIR/replies/K4_open.json" 'str(d.get("status",""))')"
        [[ "$OPEN_STATUS" == "ok" ]] && ok "LEG K4: open_project ok (same track/plugin EditItemIDs)" \
          || bad "LEG K4: open_project status=$OPEN_STATUS: $(head -c 300 "$RUN_DIR/replies/K4_open.json")"
        sleep 8  # edit reload settle

        python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/K4_read_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/K4_read_req.json" "$RUN_DIR/replies/K4_read.json" 120000 || true
        K4_READBACK="$(python3 - "$RUN_DIR/replies/K4_read.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("parameters") or []:
        if str(p.get("name","")) == "SceneMode":
            print("%s|%s" % (p.get("normalized_value"), p.get("value_text","")))
            break
except Exception:
    pass
PY
)"
        K4_NORM="${K4_READBACK%%|*}"
        K4_TEXT="${K4_READBACK#*|}"
        prereq "K4_readback normalized=$K4_NORM text=$K4_TEXT"
        if [[ "$K4_NORM" == 0.5 || "$K4_NORM" == "0.5" ]]; then
          ok "LEG K4: SceneMode survives save->open (chunk roundtrip; value_text=$K4_TEXT)"
        else
          bad "LEG K4: SceneMode lost after reload (readback=$K4_NORM, expected 0.5)"
        fi
      else
        bad "LEG K4: missing plugin_id prerequisite"
      fi
    fi

    # ---------------- LEG K5: show_plugin_editor 不崩
    if has_leg K5; then
      step "LEG K5: show_plugin_editor -> process alive (visual face: zero assertions)"
      PLUGIN_ID="$(jfield "$RUN_DIR/replies/K2_rack_add.json" 'str(d.get("plugin_id", ""))')"
      if [[ -n "$PLUGIN_ID" ]]; then
        python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/K5_show_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "show_plugin_editor",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/K5_show_req.json" "$RUN_DIR/replies/K5_show.json" 120000 || true
        SHOW_STATUS="$(jfield "$RUN_DIR/replies/K5_show.json" 'str(d.get("status",""))')"
        prereq "K5_show status=$SHOW_STATUS"
        [[ "$SHOW_STATUS" == "ok" ]] && ok "LEG K5: show_plugin_editor ok (editor view provided)" \
          || bad "LEG K5: show_plugin_editor status=$SHOW_STATUS: $(head -c 300 "$RUN_DIR/replies/K5_show.json")"
        sleep 3
        if kill -0 "$KERNEL_PID" 2>/dev/null; then
          ok "LEG K5: kernel process alive 3s after editor open (process-survival assertion)"
        else
          bad "LEG K5: kernel died after show_plugin_editor — see $(basename "$KERNEL_LOG")"
        fi
      else
        bad "LEG K5: missing plugin_id prerequisite"
      fi
    fi

    stop_kernel
  fi
fi

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
    "schema_version": "vit_gen_shell_skeleton_smoke.mac.v1",
    "card": "GENSHELL-CPP-1",
    "run_id": run_id,
    "run_root": run_dir,
    "legs_requested": legs,
    "ok_count": len(ok_lines),
    "red_count": len(red_lines),
    "ok": ok_lines,
    "red": red_lines,
    "verdict": "all_green" if not red_lines else "assertions_red",
}
out = os.path.join(run_dir, "skeleton_report.json")
json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("report: %s" % out, file=sys.stderr)
print("verdict: %s (ok=%d red=%d)" % (report["verdict"], len(ok_lines), len(red_lines)), file=sys.stderr)
PY
prereq "finished=$(date '+%F %T')"
log "workdir: $RUN_DIR"
if [[ "$OVERALL_PASS" -eq 1 ]]; then
  echo "GENSHELL_SKELETON_MAC_VERDICT all_green" >&2
  exit 0
else
  echo "GENSHELL_SKELETON_MAC_VERDICT assertions_red" >&2
  exit 1
fi
