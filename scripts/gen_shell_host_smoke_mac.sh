#!/usr/bin/env bash
# gen_shell_host_smoke_mac.sh — GENSHELL-HOST-1 真栈烟测（mac）
#
# 驱动真实内核二进制（VitApp，含本卡 GenerationShellService 改动——由
# --kernel-bin 指定，默认本仓 worktree 的 Debug 产物）走内核命令线
# （ZMQ REQ 5555，与 gen_shell_skeleton_smoke_mac.sh 同模式）：
#
#   LEG S   壳单测回归（genshell_selftest 对已部署 bundle exit 0——
#           壳体零改动证据；已部署 bundle 不 redeploy，sha256 留档）
#   LEG H1  装载    — scan_plugins → plugin_list_available → add_track →
#                     rack_add_node（UI 路径形态=直接内核命令）→
#                     get_plugin_parameters 断言 T1 五参数在场
#   LEG H2  主选核实 — gen_shell.pending(debug_shell_state) 应答含壳注册表
#                     项（identifier/plugin_id 匹配）+ 探针结果：
#                     probe_qi_edit_controller=kNoInterface（独立类壳 component
#                     直取 controller 不成立）+ probe_connect_attempt=kResultFalse
#                     （processor 端点被 JUCE 互联单 peer 占用）+
#                     t2_transport=downgrade_a_param_chunk —— 运行时核实证据
#   LEG H3  伪提交  — gen_shell.pending(debug_submit) → 应答 jobs 含 queued 作业
#                     +请求载荷（request 字段四要素）；get_project_state jobs 快照
#                     断言 job_state=queued/ghost_state=mute/source=gen_shell:<node>/
#                     request_hash 非空；重复 job_id 提交被拒（幂等 fail-closed）
#   LEG H4  伪推进+JobState 镜像 — aigc_register_job upsert running（agent runner
#                     同面）→ gen_shell.pending（服务刷新）→ get_plugin_parameters
#                     JobState value_text=running；upsert done → 镜像 done
#   LEG H5  取消    — gen_shell.pending(debug_cancel) → 应答 job state=
#                     cancel_requested+cancel_requested=true；jobs 快照一致
#   LEG H6  ghost 清除 — aigc_register_job upsert cancelled（携带 ghost_state=mute
#                     完整记录）→ gen_shell.pending → jobs 快照 ghost_state=bypass
#                     +JobState 镜像=cancelled（终态下行）
#
# 工程隔离：VIT_PROJECT_XML 指向 run 目录的项目副本（AGENTS §10）；
# 内核 cwd=本脚本所在仓库根（Settings.xml 变更留在该 checkout 内）。
#
# §8 纪律：确定性内核行为断言（无 LLM）；一次运行全部 leg，exit 0 = 全绿。
#
# Usage:
#   ./scripts/gen_shell_host_smoke_mac.sh                    # 全部 leg
#   ./scripts/gen_shell_host_smoke_mac.sh --leg H1,H2
#   ./scripts/gen_shell_host_smoke_mac.sh --kernel-bin PATH
#   ./scripts/gen_shell_host_smoke_mac.sh --vst3-bundle PATH
#                                                 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
#   ./scripts/gen_shell_host_smoke_mac.sh --selftest-bin PATH
#                                                 (default <repo>/GenShell/build/genshell_selftest)
#   ./scripts/gen_shell_host_smoke_mac.sh --artifacts-root DIR
#                                                 (default ~/Documents/vit-genshell-artifacts)

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
VST3_BUNDLE="$HOME/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
SELFTEST_BIN_DEFAULT=""
ARTIFACTS_ROOT_DEFAULT="$HOME/Documents/vit-genshell-artifacts"
LEGS="S,H1,H2,H3,H4,H5,H6"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
STARTUP_TIMEOUT_SECONDS=240
STOP_GRACE_SECONDS=10
SCAN_TIMEOUT_SECONDS=900

usage() {
  cat <<'EOF'
gen_shell_host_smoke_mac.sh — GENSHELL-HOST-1 mac real-stack smoke.

Options:
  --repo-root PATH      Repository root (default: parent of this script's dir)
  --kernel-bin PATH     Kernel binary with GenerationShellService
                        (default: repo VitApp/build/VitApp_artefacts/Debug/VitApp)
  --vst3-bundle PATH    Deployed GenShell.vst3 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
  --selftest-bin PATH   genshell_selftest binary (default repo GenShell/build/genshell_selftest)
  --artifacts-root DIR  Artifacts root (default ~/Documents/vit-genshell-artifacts)
  --leg LIST            Comma subset of S,H1,H2,H3,H4,H5,H6 (default all)
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

RUN_ID="genshell_host_mac_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/host/$RUN_ID"
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
  echo "repo_branch=$(git -C "$REPO_ROOT" branch --show-current 2>/dev/null || echo unknown)"
  echo "repo_dirty_files=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ')"
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

# ================================================================ LEG S（壳单测回归——壳体零改动证据）
if has_leg S; then
  step "LEG S: genshell_selftest against deployed bundle (shell regression)"
  if [[ -x "$SELFTEST_BIN" ]]; then
    "$SELFTEST_BIN" "$VST3_BUNDLE" > "$RUN_DIR/selftest/selftest.log" 2>&1
    SEXIT=$?
    cp "$RUN_DIR/selftest/selftest.log" "$RUN_DIR/selftest/selftest_copy.log"
    if [[ $SEXIT -eq 0 ]]; then
      ok "LEG S: selftest exit 0 ($(grep -c '^\[PASS\]' "$RUN_DIR/selftest/selftest.log") PASS / $(grep -c '^\[FAIL\]' "$RUN_DIR/selftest/selftest.log") FAIL) — deployed bundle unchanged"
    else
      bad "LEG S: selftest exit=$SEXIT — $(grep '^\[FAIL\]' "$RUN_DIR/selftest/selftest.log" | head -3)"
    fi
  else
    bad "LEG S: selftest binary not executable: $SELFTEST_BIN (pass --selftest-bin)"
  fi
fi

# ================================================================ 真栈 legs
if has_leg H1 || has_leg H2 || has_leg H3 || has_leg H4 || has_leg H5 || has_leg H6; then
  step "Kernel start"
  KERNEL_OK=1
  start_kernel || KERNEL_OK=0
  if [[ "$KERNEL_OK" -eq 1 ]]; then
    sleep 5  # edit settle

    # ---------------- LEG H1: scan → 发现 → 装载 → T1 参数面
    if has_leg H1; then
      step "LEG H1: scan -> plugin_list_available -> add_track -> rack_add_node -> T1 five params"
      USER_VST3_DIR="$HOME/Library/Audio/Plug-Ins/VST3"
      printf '{"cmd":"scan_plugins","paths":["%s"]}' "$USER_VST3_DIR" > "$RUN_DIR/replies/scan_req.json"
      zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/H1_scan_start.json" 60000 || true
      SCAN_ID="$(jfield "$RUN_DIR/replies/H1_scan_start.json" 'str(d.get("scan_id",""))')"
      STATE=""
      deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
      while (( SECONDS < deadline )); do
        printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/status_req.json"
        zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/H1_status.json" 30000 >/dev/null || true
        STATE="$(jfield "$RUN_DIR/replies/H1_status.json" 'str(d.get("status",""))')"
        [[ "$STATE" != "scanning" && "$STATE" != "cancelling" ]] && break
        sleep 3
      done
      prereq "H1_scan_terminal=$STATE"
      [[ "$STATE" == "completed" ]] && ok "LEG H1: scan completed" || bad "LEG H1: scan state=$STATE"

      printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/list_req.json"
      zmq_send "$RUN_DIR/replies/list_req.json" "$RUN_DIR/replies/H1_list.json" 60000 || true
      GENSHELL_FOUND="$(python3 - "$RUN_DIR/replies/H1_list.json" <<'PY'
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
        ok "LEG H1: GenShell discovered (identifier=$IDENTIFIER)"
        printf '%s' "$IDENTIFIER" > "$RUN_DIR/replies/H1_identifier.txt"
      else
        bad "LEG H1: GenShell not in plugin_list_available (match='$GENSHELL_FOUND')"
      fi

      printf '{"cmd":"add_track"}' > "$RUN_DIR/replies/H1_add_track_req.json"
      zmq_send "$RUN_DIR/replies/H1_add_track_req.json" "$RUN_DIR/replies/H1_add_track.json" 60000 || true
      TRACK_ID="$(jfield "$RUN_DIR/replies/H1_add_track.json" 'str(d.get("track_id",""))')"
      prereq "H1_track_id=$TRACK_ID"
      [[ -n "$TRACK_ID" ]] && ok "LEG H1: track added" || bad "LEG H1: add_track failed"

      IDENTIFIER="$(cat "$RUN_DIR/replies/H1_identifier.txt" 2>/dev/null || true)"
      PLUGIN_ID=""
      if [[ -n "$TRACK_ID" && -n "$IDENTIFIER" ]]; then
        python3 - "$TRACK_ID" "$IDENTIFIER" > "$RUN_DIR/replies/H1_rack_add_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node",
                  "track_id": sys.argv[1], "plugin_identifier": sys.argv[2],
                  "x": 0, "y": 0}))
PY
        zmq_send "$RUN_DIR/replies/H1_rack_add_req.json" "$RUN_DIR/replies/H1_rack_add.json" 180000 || true
        PLUGIN_ID="$(jfield "$RUN_DIR/replies/H1_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
        ADD_STATUS="$(jfield "$RUN_DIR/replies/H1_rack_add.json" 'str(d.get("status",""))')"
        ADD_ERR="$(jfield "$RUN_DIR/replies/H1_rack_add.json" 'str(d.get("message", d.get("error","")))')"
        prereq "H1_rack_add status=$ADD_STATUS plugin_id=$PLUGIN_ID error=$ADD_ERR"
        if [[ "$ADD_STATUS" == "ok" && -n "$PLUGIN_ID" ]]; then
          ok "LEG H1: rack_add_node landed (UI path form, direct kernel command; plugin_id=$PLUGIN_ID)"
        else
          bad "LEG H1: rack_add_node failed: status=$ADD_STATUS error=$ADD_ERR"
        fi
      else
        bad "LEG H1: missing track_id or identifier prerequisite"
      fi

      if [[ -n "$PLUGIN_ID" ]]; then
        python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/H1_params_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/H1_params_req.json" "$RUN_DIR/replies/H1_params.json" 120000 || true
        H1_PARAM_REPORT="$(python3 - "$RUN_DIR/replies/H1_params.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    params = d.get("parameters") or []
    names = set(str(p.get("name","")) for p in params)
    need = ["JobState", "JobProgress", "JobErrorClass", "SceneMode", "Bypass"]
    missing = [n for n in need if n not in names]
    print("ok|%s|%d|%s" % (d.get("status")=="ok", len(params), ",".join(missing)))
except Exception as e:
    print("err||%s" % e)
PY
)"
        if [[ "$H1_PARAM_REPORT" == ok\|True\|*\| ]]; then
          ok "LEG H1: T1 five params present via get_plugin_parameters (count=$(printf '%s' "$H1_PARAM_REPORT" | cut -d'|' -f3) incl. wrapper params)"
        else
          bad "LEG H1: params check failed: $H1_PARAM_REPORT"
        fi
      fi
    fi

    PLUGIN_ID="$(jfield "$RUN_DIR/replies/H1_rack_add.json" 'str(d.get("plugin_id", ""))')"
    IDENTIFIER="$(cat "$RUN_DIR/replies/H1_identifier.txt" 2>/dev/null || true)"

    # ---------------- LEG H2: 主选路径核实（运行时探针证据）
    if has_leg H2; then
      step "LEG H2: gen_shell.pending registry + primary-path probe evidence"
      if [[ -n "$PLUGIN_ID" ]]; then
        python3 - "$PLUGIN_ID" > "$RUN_DIR/replies/H2_pending_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "gen_shell.pending", "debug_shell_state": True}))
PY
        zmq_send "$RUN_DIR/replies/H2_pending_req.json" "$RUN_DIR/replies/H2_pending.json" 60000 || true
        H2_REPORT="$(python3 - "$RUN_DIR/replies/H2_pending.json" "$PLUGIN_ID" "$IDENTIFIER" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want_id, want_ident = sys.argv[2], sys.argv[3]
    if d.get("status") != "ok":
        print("err|status=%s" % d.get("status"))
    else:
        shells = d.get("shells") or []
        entry = next((s for s in shells if s.get("plugin_id") == want_id), None)
        if entry is None:
            print("err|shell not registered: %s (shells=%d)" % (want_id, len(shells)))
        else:
            print("%s|%s|%s|%s" % (entry.get("probe_qi_edit_controller",""),
                                   entry.get("probe_qi_connection_point",""),
                                   entry.get("probe_connect_attempt",""),
                                   entry.get("t2_transport","")))
        ident = str(entry.get("identifier","")) if entry is not None else ""
        print("ident_ok=%s" % (entry is not None and "GenShell" in ident and "GenShell" in want_ident))
        print("instance_ready=%s" % (entry is not None and entry.get("instance_ready")))
except Exception as e:
    print("err|%s" % e)
PY
)"
        H2_PROBES="$(printf '%s' "$H2_REPORT" | head -1)"
        H2_IDENT="$(printf '%s' "$H2_REPORT" | grep '^ident_ok=' | tail -1)"
        H2_READY="$(printf '%s' "$H2_REPORT" | grep '^instance_ready=' | tail -1)"
        prereq "H2_probes=$H2_PROBES $H2_IDENT $H2_READY"
        if [[ "$H2_IDENT" == "ident_ok=True" ]]; then
          ok "LEG H2: shell registered in service registry (identifier match)"
        else
          bad "LEG H2: shell not registered / identifier mismatch: $H2_PROBES $H2_IDENT"
        fi
        if [[ "$H2_READY" == "instance_ready=True" ]]; then
          ok "LEG H2: shell instance ready"
        else
          bad "LEG H2: shell instance not ready: $H2_READY"
        fi
        QI_CTRL="$(printf '%s' "$H2_PROBES" | cut -d'|' -f1)"
        QI_CP="$(printf '%s' "$H2_PROBES" | cut -d'|' -f2)"
        CONNECT="$(printf '%s' "$H2_PROBES" | cut -d'|' -f3)"
        TRANSPORT="$(printf '%s' "$H2_PROBES" | cut -d'|' -f4)"
        if [[ "$QI_CTRL" == "kNoInterface" ]]; then
          ok "LEG H2: probe_qi_edit_controller=kNoInterface (separate-class shell; primary path step 1 fails)"
        else
          bad "LEG H2: probe_qi_edit_controller=$QI_CTRL (expected kNoInterface)"
        fi
        if [[ "$QI_CP" == "kResultOk" ]]; then
          ok "LEG H2: probe_qi_connection_point=kResultOk (processor endpoint reachable)"
        else
          bad "LEG H2: probe_qi_connection_point=$QI_CP (expected kResultOk)"
        fi
        if [[ "$CONNECT" == "kResultFalse" ]]; then
          ok "LEG H2: probe_connect_attempt=kResultFalse (endpoint occupied by JUCE interconnect — single-peer fail-closed)"
        else
          bad "LEG H2: probe_connect_attempt=$CONNECT (expected kResultFalse)"
        fi
        if [[ "$TRANSPORT" == "downgrade_a_param_chunk" ]]; then
          ok "LEG H2: t2_transport=downgrade_a_param_chunk (fallback A engaged, queue semantics unchanged)"
        else
          bad "LEG H2: t2_transport=$TRANSPORT (expected downgrade_a_param_chunk)"
        fi
      else
        bad "LEG H2: missing plugin_id prerequisite"
      fi
    fi

    # ---------------- LEG H3: 伪提交 → 队列 + pending 应答 + jobs 快照
    if has_leg H3; then
      step "LEG H3: debug_submit -> queued job in gen_shell.pending + jobs snapshot"
      if [[ -n "$PLUGIN_ID" && -n "$TRACK_ID" ]]; then
        JOB_ID="smoke-host1-$(date '+%H%M%S')"
        printf '%s' "$JOB_ID" > "$RUN_DIR/replies/H3_job_id.txt"
        python3 - "$JOB_ID" "$PLUGIN_ID" "$TRACK_ID" > "$RUN_DIR/replies/H3_submit_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "gen_shell.pending",
                  "debug_submit": {
                      "schema_version": "vit.genshell.request.v1",
                      "job_id": sys.argv[1],
                      "shell_node_id": sys.argv[2],
                      "track_id": sys.argv[3],
                      "start_time": 0.0,
                      "length_beats": 8,
                      "scene": "in_project",
                      "provider_id": "mock-provider",
                      "template_id": "song_segment_v1",
                      "form": {"mood": "warm"},
                      "free_text": "smoke host leg H3 pseudo submit",
                      "gen_params": {"duration_seconds": 8, "count": 1,
                                     "style_tags": "smoke", "instrumental": False, "title": ""}
                  }}))
PY
        zmq_send "$RUN_DIR/replies/H3_submit_req.json" "$RUN_DIR/replies/H3_submit.json" 60000 || true
        H3_STATUS="$(jfield "$RUN_DIR/replies/H3_submit.json" 'str(d.get("status",""))')"
        [[ "$H3_STATUS" == "ok" ]] && ok "LEG H3: debug_submit accepted (job_id=$JOB_ID)" \
          || bad "LEG H3: debug_submit status=$H3_STATUS: $(head -c 300 "$RUN_DIR/replies/H3_submit.json")"

        # 应答断言：queued 作业 + 请求载荷四要素
        printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/H3_pending_req.json"
        zmq_send "$RUN_DIR/replies/H3_pending_req.json" "$RUN_DIR/replies/H3_pending.json" 60000 || true
        H3_PENDING="$(python3 - "$RUN_DIR/replies/H3_pending.json" "$JOB_ID" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want = sys.argv[2]
    job = next((j for j in d.get("jobs") or [] if j.get("job_id") == want), None)
    if job is None:
        print("missing|jobs=%s" % [j.get("job_id") for j in d.get("jobs") or []])
    else:
        req = job.get("request") or {}
        payload_ok = all(req.get(k) not in (None, "") for k in ("free_text","template_id","provider_id","scene"))
        print("%s|%s|%s" % (job.get("state",""), job.get("cancel_requested"), payload_ok))
    print("providers_ok=%s" % isinstance(d.get("providers"), list))
except Exception as e:
    print("err|%s" % e)
PY
)"
        H3_STATE="$(printf '%s' "$H3_PENDING" | head -1 | cut -d'|' -f1)"
        H3_CANCEL="$(printf '%s' "$H3_PENDING" | head -1 | cut -d'|' -f2)"
        H3_PAYLOAD="$(printf '%s' "$H3_PENDING" | head -1 | cut -d'|' -f3)"
        H3_PROVIDERS="$(printf '%s' "$H3_PENDING" | grep '^providers_ok=' | tail -1)"
        prereq "H3_pending state=$H3_STATE cancel=$H3_CANCEL payload_ok=$H3_PAYLOAD $H3_PROVIDERS"
        if [[ "$H3_STATE" == "queued" && "$H3_CANCEL" == "False" && "$H3_PAYLOAD" == "True" ]]; then
          ok "LEG H3: gen_shell.pending answers queued job + full request payload"
        else
          bad "LEG H3: pending answer wrong: $H3_PENDING"
        fi
        if [[ "$H3_PROVIDERS" == "providers_ok=True" ]]; then
          ok "LEG H3: providers array present (v1 empty — kernel has no genapi knowledge)"
        else
          bad "LEG H3: providers missing: $H3_PROVIDERS"
        fi

        # jobs 快照断言（ghost 置位/source/request_hash）
        printf '{"cmd":"get_project_state"}' > "$RUN_DIR/replies/H3_state_req.json"
        zmq_send "$RUN_DIR/replies/H3_state_req.json" "$RUN_DIR/replies/H3_state.json" 120000 || true
        H3_SNAP="$(python3 - "$RUN_DIR/replies/H3_state.json" "$JOB_ID" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want = sys.argv[2]
    job = next((j for j in d.get("jobs") or [] if j.get("job_id") == want), None)
    if job is None:
        print("missing|jobs=%d" % len(d.get("jobs") or []))
    else:
        print("%s|%s|%s|%s" % (job.get("job_state",""), job.get("ghost_state",""),
                               job.get("source",""), bool(job.get("request_hash"))))
except Exception as e:
    print("err|%s" % e)
PY
)"
        H3_JSTATE="$(printf '%s' "$H3_SNAP" | cut -d'|' -f1)"
        H3_GHOST="$(printf '%s' "$H3_SNAP" | cut -d'|' -f2)"
        H3_SOURCE="$(printf '%s' "$H3_SNAP" | cut -d'|' -f3)"
        H3_HASH="$(printf '%s' "$H3_SNAP" | cut -d'|' -f4)"
        prereq "H3_snapshot job_state=$H3_JSTATE ghost=$H3_GHOST source=$H3_SOURCE hash=$H3_HASH"
        if [[ "$H3_JSTATE" == "queued" && "$H3_GHOST" == "mute" && "$H3_SOURCE" == "gen_shell:$PLUGIN_ID" && "$H3_HASH" == "True" ]]; then
          ok "LEG H3: jobs snapshot: queued + ghost=mute + source=gen_shell:<node> + request_hash present"
        else
          bad "LEG H3: jobs snapshot wrong: $H3_SNAP (expected source=gen_shell:$PLUGIN_ID)"
        fi

        # 幂等负探针：重复 job_id 拒绝
        zmq_send "$RUN_DIR/replies/H3_submit_req.json" "$RUN_DIR/replies/H3_dup.json" 60000 || true
        H3_DUP_STATUS="$(jfield "$RUN_DIR/replies/H3_dup.json" 'str(d.get("status",""))')"
        H3_DUP_MSG="$(jfield "$RUN_DIR/replies/H3_dup.json" 'str(d.get("message",""))')"
        prereq "H3_dup status=$H3_DUP_STATUS msg=$H3_DUP_MSG"
        if [[ "$H3_DUP_STATUS" == "error" && "$H3_DUP_MSG" == *"already submitted"* ]]; then
          ok "LEG H3: duplicate job_id rejected (fail-closed idempotency)"
        else
          bad "LEG H3: duplicate submit not rejected: status=$H3_DUP_STATUS msg=$H3_DUP_MSG"
        fi
      else
        bad "LEG H3: missing plugin_id/track_id prerequisite"
      fi
    fi

    JOB_ID="$(cat "$RUN_DIR/replies/H3_job_id.txt" 2>/dev/null || true)"

    # ---------------- LEG H4: 伪推进 + JobState 镜像
    if has_leg H4; then
      step "LEG H4: aigc_register_job advance -> service refresh -> T1 JobState mirror"
      if [[ -n "$PLUGIN_ID" && -n "$JOB_ID" ]]; then
        for ADV_STATE in running done; do
          python3 - "$JOB_ID" "$PLUGIN_ID" "$ADV_STATE" > "$RUN_DIR/replies/H4_advance_req.json" <<'PY'
import json, sys
# 与 agent runner 生产流同面（件二 §4）：aigc_register_job 整记录 upsert，
# 携带全部字段（upsert 是替换语义——缺字段会丢记录）
print(json.dumps({"cmd": "aigc_register_job",
                  "job_id": sys.argv[1],
                  "node_id": sys.argv[2],
                  "job_state": sys.argv[3],
                  "ghost_state": "mute",
                  "status_message": "class=none smoke advance %s" % sys.argv[3],
                  "request_hash": "smoke",
                  "source": "gen_shell:%s" % sys.argv[2]}))
PY
          zmq_send "$RUN_DIR/replies/H4_advance_req.json" "$RUN_DIR/replies/H4_advance_$ADV_STATE.json" 60000 || true
          ADV_STATUS="$(jfield "$RUN_DIR/replies/H4_advance_$ADV_STATE.json" 'str(d.get("status",""))')"
          prereq "H4_advance_$ADV_STATE status=$ADV_STATUS"
          [[ "$ADV_STATUS" == "ok" ]] || bad "LEG H4: advance to $ADV_STATE failed: $ADV_STATUS"

          printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/H4_refresh_req.json"
          zmq_send "$RUN_DIR/replies/H4_refresh_req.json" "$RUN_DIR/replies/H4_refresh_$ADV_STATE.json" 60000 || true

          python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/H4_read_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
          zmq_send "$RUN_DIR/replies/H4_read_req.json" "$RUN_DIR/replies/H4_read_$ADV_STATE.json" 120000 || true
          H4_MIRROR="$(python3 - "$RUN_DIR/replies/H4_read_$ADV_STATE.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("parameters") or []:
        if str(p.get("name","")) == "JobState":
            print("%s|%s" % (p.get("value_text",""), p.get("normalized_value")))
            break
    else:
        print("missing|")
except Exception as e:
    print("err|%s" % e)
PY
)"
          H4_TEXT="$(printf '%s' "$H4_MIRROR" | cut -d'|' -f1)"
          H4_NORM="$(printf '%s' "$H4_MIRROR" | cut -d'|' -f2)"
          # running=2/7≈0.2857；done=5/7≈0.7143
          H4_EXPECT_NORM="$(python3 -c "print({'running':2,'done':5}['$ADV_STATE']/7)")"
          H4_NORM_OK="$(python3 -c "print(abs(float('$H4_NORM' or 0)-float('$H4_EXPECT_NORM'))<0.02)")"
          prereq "H4_mirror_$ADV_STATE text=$H4_TEXT normalized=$H4_NORM (expected $H4_EXPECT_NORM)"
          if [[ "$H4_TEXT" == "$ADV_STATE" && "$H4_NORM_OK" == "True" ]]; then
            ok "LEG H4: JobState mirror=$ADV_STATE (value_text + normalized $H4_NORM)"
          else
            bad "LEG H4: JobState mirror wrong after advance to $ADV_STATE: text=$H4_TEXT normalized=$H4_NORM"
          fi
        done
      else
        bad "LEG H4: missing plugin_id/job_id prerequisite"
      fi
    fi

    # ---------------- LEG H5: 取消路径（作业 B——A 在 H4 已终态，取消走独立提交）
    if has_leg H5; then
      step "LEG H5: submit job B -> debug_cancel -> cancel_requested visible"
      if [[ -n "$PLUGIN_ID" && -n "$TRACK_ID" ]]; then
        JOB_ID="smoke-host1-cancel-$(date '+%H%M%S')"
        printf '%s' "$JOB_ID" > "$RUN_DIR/replies/H5_job_id.txt"
        python3 - "$JOB_ID" "$PLUGIN_ID" "$TRACK_ID" > "$RUN_DIR/replies/H5_submit_req.json" <<'PY5S'
import json, sys
print(json.dumps({"cmd": "gen_shell.pending",
                  "debug_submit": {
                      "schema_version": "vit.genshell.request.v1",
                      "job_id": sys.argv[1],
                      "shell_node_id": sys.argv[2],
                      "track_id": sys.argv[3],
                      "start_time": 0.0, "length_beats": 4,
                      "scene": "in_project", "provider_id": "mock-provider",
                      "template_id": "song_segment_v1", "form": {},
                      "free_text": "smoke host leg H5 cancel path",
                      "gen_params": {"duration_seconds": 4, "count": 1,
                                     "style_tags": "smoke", "instrumental": True, "title": ""}
                  }}))
PY5S
        zmq_send "$RUN_DIR/replies/H5_submit_req.json" "$RUN_DIR/replies/H5_submit.json" 60000 || true
        H5_SUB="$(jfield "$RUN_DIR/replies/H5_submit.json" 'str(d.get("status",""))')"
        [[ "$H5_SUB" == "ok" ]] && ok "LEG H5: job B submitted ($JOB_ID)" \
          || bad "LEG H5: job B submit status=$H5_SUB"

        python3 - "$JOB_ID" > "$RUN_DIR/replies/H5_cancel_req.json" <<'PY5C'
import json, sys
print(json.dumps({"cmd": "gen_shell.pending",
                  "debug_cancel": {"job_id": sys.argv[1]}}))
PY5C
        zmq_send "$RUN_DIR/replies/H5_cancel_req.json" "$RUN_DIR/replies/H5_cancel.json" 60000 || true
        H5_STATUS="$(jfield "$RUN_DIR/replies/H5_cancel.json" 'str(d.get("status",""))')"
        [[ "$H5_STATUS" == "ok" ]] && ok "LEG H5: debug_cancel accepted" \
          || bad "LEG H5: debug_cancel status=$H5_STATUS: $(head -c 300 "$RUN_DIR/replies/H5_cancel.json")"

        printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/H5_pending_req.json"
        zmq_send "$RUN_DIR/replies/H5_pending_req.json" "$RUN_DIR/replies/H5_pending.json" 60000 || true
        H5_PENDING="$(python3 - "$RUN_DIR/replies/H5_pending.json" "$JOB_ID" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want = sys.argv[2]
    job = next((j for j in d.get("jobs") or [] if j.get("job_id") == want), None)
    print("%s|%s" % (job.get("state","missing") if job else "missing",
                     job.get("cancel_requested") if job else ""))
except Exception as e:
    print("err|%s" % e)
PY
)"
        H5_STATE="$(printf '%s' "$H5_PENDING" | cut -d'|' -f1)"
        H5_FLAG="$(printf '%s' "$H5_PENDING" | cut -d'|' -f2)"
        prereq "H5_pending state=$H5_STATE cancel_requested=$H5_FLAG"
        if [[ "$H5_STATE" == "cancel_requested" && "$H5_FLAG" == "True" ]]; then
          ok "LEG H5: pending answer shows cancel_requested + flag"
        else
          bad "LEG H5: pending answer wrong: $H5_PENDING"
        fi

        printf '{"cmd":"get_project_state"}' > "$RUN_DIR/replies/H5_state_req.json"
        zmq_send "$RUN_DIR/replies/H5_state_req.json" "$RUN_DIR/replies/H5_state.json" 120000 || true
        H5_SNAP="$(jfield "$RUN_DIR/replies/H5_state.json" 'str(next((j for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_ID'"),{}).get("job_state","missing"))')"
        prereq "H5_snapshot job_state=$H5_SNAP"
        if [[ "$H5_SNAP" == "cancel_requested" ]]; then
          ok "LEG H5: jobs snapshot job_state=cancel_requested (cancel path upsert visible)"
        else
          bad "LEG H5: jobs snapshot wrong: $H5_SNAP"
        fi
      else
        bad "LEG H5: missing job_id prerequisite"
      fi
    fi

    # ---------------- LEG H6: 终态 ghost 清除 + cancelled 镜像（作业 B）
    if has_leg H6; then
      step "LEG H6: terminal cancelled -> ghost cleared + JobState mirror=cancelled"
      JOB_ID="$(cat "$RUN_DIR/replies/H5_job_id.txt" 2>/dev/null || true)"
      if [[ -n "$PLUGIN_ID" && -n "$JOB_ID" ]]; then
        python3 - "$JOB_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/H6_terminal_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "aigc_register_job",
                  "job_id": sys.argv[1],
                  "node_id": sys.argv[2],
                  "job_state": "cancelled",
                  "ghost_state": "mute",
                  "status_message": "class=none smoke terminal cancelled",
                  "request_hash": "smoke",
                  "source": "gen_shell:%s" % sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/H6_terminal_req.json" "$RUN_DIR/replies/H6_terminal.json" 60000 || true
        H6_ADV="$(jfield "$RUN_DIR/replies/H6_terminal.json" 'str(d.get("status",""))')"
        [[ "$H6_ADV" == "ok" ]] && ok "LEG H6: terminal upsert cancelled accepted" || bad "LEG H6: terminal upsert failed: $H6_ADV"

        printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/H6_refresh_req.json"
        zmq_send "$RUN_DIR/replies/H6_refresh_req.json" "$RUN_DIR/replies/H6_refresh.json" 60000 || true

        printf '{"cmd":"get_project_state"}' > "$RUN_DIR/replies/H6_state_req.json"
        zmq_send "$RUN_DIR/replies/H6_state_req.json" "$RUN_DIR/replies/H6_state.json" 120000 || true
        H6_SNAP="$(python3 - "$RUN_DIR/replies/H6_state.json" "$JOB_ID" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want = sys.argv[2]
    job = next((j for j in d.get("jobs") or [] if j.get("job_id") == want), None)
    print("%s|%s" % (job.get("job_state","missing") if job else "missing",
                     job.get("ghost_state","") if job else ""))
except Exception as e:
    print("err|%s" % e)
PY
)"
        H6_STATE="$(printf '%s' "$H6_SNAP" | cut -d'|' -f1)"
        H6_GHOST="$(printf '%s' "$H6_SNAP" | cut -d'|' -f2)"
        prereq "H6_snapshot job_state=$H6_STATE ghost=$H6_GHOST"
        if [[ "$H6_STATE" == "cancelled" && "$H6_GHOST" == "bypass" ]]; then
          ok "LEG H6: ghost cleared on terminal cancelled (mute -> bypass)"
        else
          bad "LEG H6: ghost clear wrong: state=$H6_STATE ghost=$H6_GHOST"
        fi

        python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/H6_read_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters",
                  "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
        zmq_send "$RUN_DIR/replies/H6_read_req.json" "$RUN_DIR/replies/H6_read.json" 120000 || true
        H6_TEXT="$(python3 - "$RUN_DIR/replies/H6_read.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    for p in d.get("parameters") or []:
        if str(p.get("name","")) == "JobState":
            print(p.get("value_text",""))
            break
    else:
        print("missing")
except Exception:
    print("err")
PY
)"
        prereq "H6_mirror text=$H6_TEXT"
        if [[ "$H6_TEXT" == "cancelled" ]]; then
          ok "LEG H6: JobState mirror=cancelled (job_terminal downlink)"
        else
          bad "LEG H6: JobState mirror wrong: $H6_TEXT (expected cancelled)"
        fi
      else
        bad "LEG H6: missing plugin_id/job_id prerequisite"
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
    "schema_version": "vit_gen_shell_host_smoke.mac.v1",
    "card": "GENSHELL-HOST-1",
    "run_id": run_id,
    "run_root": run_dir,
    "legs_requested": legs,
    "ok_count": len(ok_lines),
    "red_count": len(red_lines),
    "ok": ok_lines,
    "red": red_lines,
    "verdict": "all_green" if not red_lines else "assertions_red",
}
out = os.path.join(run_dir, "host_report.json")
json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("report: %s" % out, file=sys.stderr)
print("verdict: %s (ok=%d red=%d)" % (report["verdict"], len(ok_lines), len(red_lines)), file=sys.stderr)
PY
prereq "finished=$(date '+%F %T')"
log "workdir: $RUN_DIR"
if [[ "$OVERALL_PASS" -eq 1 ]]; then
  echo "GENSHELL_HOST_MAC_VERDICT all_green" >&2
  exit 0
else
  echo "GENSHELL_HOST_MAC_VERDICT assertions_red" >&2
  exit 1
fi
