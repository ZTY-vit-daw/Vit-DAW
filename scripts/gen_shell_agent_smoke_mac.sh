#!/usr/bin/env bash
# gen_shell_agent_smoke_mac.sh — GENSHELL-AGENT-1 真栈烟测（mac）
#
# 驱动真实三件套中的两件（VitApp 内核 + Go agent；本卡端测边界=agent 数据面，
# 无 Godot 前端旅程），provider=本地回环模拟器（gen_api_adapter_smoke_mac.sh
# 先例——零真源/零订阅/零密钥出网）。内核侧零改动：复用含 HOST-1 的内核
# 二进制（--kernel-bin，记录 sha256/mtime，AGENTS §9）。
#
#   A0  agent 启动 + runner 就绪 — /health + gen_shell.status（HTTP /agent/invoke）
#                     应答 started=true enabled=true（chat Server Start 单点接线证据）
#   A1  装载    — set_tempo 96（music_context 确定性）→ scan_plugins →
#                     plugin_list_available → add_track → rack_add_node（UI 路径
#                     形态=直接内核命令）→ T1 五参数在场
#   A2  主链    — debug_submit(in_project, sim-fast) → pending 应答 queued →
#                     runner 领取（running upsert 整记录四字段）→ 模拟器请求体
#                     断言 == MUSIC CONTEXT / == CONSTRAINTS / tempo: 96.0 /
#                     用户原文置尾（genctx profile=gen_plugin 注入证据）→
#                     产物 WAV 落盘 → ingest 落轨（clip_id/asset_ref/take_stack
#                     回执）→ 终态 done + JobState 镜像=done
#   A3  多候选 — debug_submit(sim-multi, count=2) → done 双 take 同栈 →
#                     switch_asset_take 切换 active_take 断言
#   A4  取消负探针 — debug_submit(sim-slow) → running 后 debug_cancel →
#                     cancelled + 零落轨 + ghost 清除（bypass）
#   A5  失败负探针 — debug_submit(sim-broken 401) → failed class=auth_error +
#                     JobErrorClass 镜像=auth_error
#   A6  free_creation — scene=free_creation → 模拟器请求体零注入块断言 +
#                     不落轨 + 工件路径回执
#   A7  孤儿清扫 — sim-slow running 中 SIGTERM agent → 重启 agent →
#                     runner 启动清扫 → interrupted 断言
#   A8  对话流提交探针（卡面修订验收第 5 条）+ 管理面 — gen_shell.stop 暂停 →
#                     gen_shell.submit（HTTP）→ pending 应答 queued →
#                     gen_shell.start 恢复 → runner 领取 → 终态 done 全链
#   A9  密钥纪律 — 模拟器日志/内核日志/agent 日志/pending 应答/HTTP 回复
#                     零 key 材料（grep）
#
# 工程隔离：VIT_PROJECT_XML 指向 run 目录项目副本；内核 cwd=repo worktree 根
# （Settings.xml 变更留在该 checkout 内）；agent 状态/缓存全在 run 目录。
#
# §8 纪律：确定性断言（无 LLM）；一次运行全部 leg，exit 0 = 全绿。
#
# Usage:
#   ./scripts/gen_shell_agent_smoke_mac.sh --kernel-bin /path/to/VitApp
#   ./scripts/gen_shell_agent_smoke_mac.sh --agent-bin /path/to/vitagent   (skip build)
#   ./scripts/gen_shell_agent_smoke_mac.sh --leg A2,A8
#   Options: --repo-root --agent-http --vst3-bundle --artifacts-root --startup-timeout

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
AGENT_BIN_ARG=""
AGENT_HTTP="http://127.0.0.1:7878"
VST3_BUNDLE="$HOME/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
ARTIFACTS_ROOT_DEFAULT="$HOME/Documents/vit-genshell-artifacts"
LEGS="A0,A1,A2,A3,A4,A5,A6,A7,A8,A9"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
STARTUP_TIMEOUT_SECONDS=240
SCAN_TIMEOUT_SECONDS=900
JOB_TIMEOUT_SECONDS=90
STOP_GRACE_SECONDS=10
# 密钥探针材料：只存在于 provider 配置文件（配置面是合法持密面），不得出现在
# 任何日志/应答中
FAKE_KEY="wrong-key-smoke-9f3a2b"

usage() {
  cat <<'EOF'
gen_shell_agent_smoke_mac.sh — GENSHELL-AGENT-1 mac real-stack smoke.

Options:
  --repo-root PATH       Repository root (default: parent of this script's dir)
  --kernel-bin PATH      Kernel binary containing GenerationShellService (HOST-1)
  --agent-bin PATH       Prebuilt agent binary (skips the go build)
  --agent-http URL       Agent HTTP base (default http://127.0.0.1:7878)
  --vst3-bundle PATH     Deployed GenShell.vst3 (default ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3)
  --artifacts-root DIR   Artifacts root (default ~/Documents/vit-genshell-artifacts)
  --startup-timeout N    Startup timeout seconds (default 240)
  --leg LIST             Comma subset of A0..A9 (default all)
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --agent-bin) AGENT_BIN_ARG="$2"; shift 2 ;;
    --agent-http) AGENT_HTTP="$2"; shift 2 ;;
    --vst3-bundle) VST3_BUNDLE="$2"; shift 2 ;;
    --artifacts-root) ARTIFACTS_ROOT_DEFAULT="$2"; shift 2 ;;
    --startup-timeout) STARTUP_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --leg) LEGS="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == "Darwin" ]] || { echo "ERROR[env]: mac-only script, uname=$PLATFORM" >&2; exit 2; }
for tool in python3 lsof curl shasum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done
python3 -c "import zmq" 2>/dev/null || { echo "ERROR[env]: python3 pyzmq not available" >&2; exit 2; }

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/build/VitApp_artefacts/Debug/VitApp}"
ARTIFACTS_ROOT="$ARTIFACTS_ROOT_DEFAULT"

[[ -x "$KERNEL_BIN" ]] || { echo "ERROR[env]: kernel binary not executable: $KERNEL_BIN (pass --kernel-bin; this card adds zero kernel changes — reuse a HOST-1-era build and record the hash)" >&2; exit 2; }
[[ -d "$VST3_BUNDLE" ]] || { echo "ERROR[env]: GenShell.vst3 bundle not found: $VST3_BUNDLE (pass --vst3-bundle)" >&2; exit 2; }

RUN_ID="genshell_agent_mac_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/agent/$RUN_ID"
mkdir -p "$RUN_DIR"/{replies,logs,http,config,cache,project,selftest,bin,agent_state}
PREREQ_FILE="$RUN_DIR/prereq.txt"

REAL_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REAL_DEFAULT_PROJECT" ]] || { echo "ERROR[env]: default project not found: $REAL_DEFAULT_PROJECT" >&2; exit 2; }

KERNEL_PID=""
AGENT_PID=""
SIM_PID=""
OVERALL_PASS=1
has_leg() { [[ ",$LEGS," == *",$1,"* ]]; }

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" | tee -a "$PREREQ_FILE" >&2; }
bad()  { echo "RED: $*" | tee -a "$PREREQ_FILE" >&2; OVERALL_PASS=0; }
info() { echo "info: $*" | tee -a "$PREREQ_FILE" >&2; }
prereq() { echo "$1" | tee -a "$PREREQ_FILE" >&2; }

sha() { shasum -a 256 "$1" 2>/dev/null | cut -d' ' -f1; }

port_listener_pid() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true; }

emergency_teardown() {
  for pid in "$AGENT_PID" "$KERNEL_PID" "$SIM_PID"; do
    [[ -n "$pid" ]] && kill -9 "$pid" 2>/dev/null || true
  done
}
trap emergency_teardown EXIT INT TERM

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

invoke_tool() {
  # HTTP /agent/invoke — tool-level failures come back as HTTP 400 + JSON
  # envelope (replied failure); only transport errors return 9.
  local tag="$1" tmo="$2" tool="$3" args="$4" body code st
  local out="$RUN_DIR/http/${tag}.json"
  body="$(printf '{"tool":"%s","args":%s,"source":"gen_shell_agent_smoke"}' "$tool" "$args")"
  code="$(curl -sS --max-time "$tmo" -o "$out" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' --data "$body" \
    "$AGENT_HTTP/agent/invoke")" || { echo "$out"; return 9; }
  st="$(jfield "$out" 'str(d.get("status",""))')"
  [[ -n "$st" ]] || { echo "$out"; return 9; }
  echo "$st|$out"
}

wait_ports_free() {
  local deadline=$((SECONDS + 30)) busy
  while (( SECONDS < deadline )); do
    busy=""
    for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "${AGENT_HTTP##*:}"; do
      [[ -n "$(port_listener_pid "$port")" ]] && busy="$busy $port"
    done
    [[ -z "$busy" ]] && return 0
    sleep 1
  done
  for port in "$KERNEL_PORT_REQ" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT" "${AGENT_HTTP##*:}"; do
    [[ -n "$(port_listener_pid "$port")" ]] && prereq "port_still_busy=$port"
  done
  return 1
}

wait_job() {
  # wait_job <job_id> <python-predicate-on-job-dict> [timeout-seconds] [tag]
  local job_id="$1" pred="$2" timeout="${3:-$JOB_TIMEOUT_SECONDS}" tag="${4:-wait}"
  local deadline=$((SECONDS + timeout)) found=""
  while (( SECONDS < deadline )); do
    printf '{"cmd":"get_project_state"}' > "$RUN_DIR/replies/${tag}_state_req.json"
    zmq_send "$RUN_DIR/replies/${tag}_state_req.json" "$RUN_DIR/replies/${tag}_state.json" 120000 >/dev/null 2>&1 || true
    found="$(python3 - "$RUN_DIR/replies/${tag}_state.json" "$job_id" "$pred" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    want, pred = sys.argv[2], sys.argv[3]
    job = next((j for j in d.get("jobs") or [] if j.get("job_id") == want), None)
    if job is None:
        print("")
    else:
        print(eval(pred, {"j": job, "str": str}))
except Exception as e:
    print("")
PY
)"
    [[ -n "$found" ]] && { printf '%s' "$found"; return 0; }
    sleep 2
  done
  printf ''
  return 1
}

submit_job() {
  # submit_job <job_id> <provider> <scene> <free_text> <count> [extra-py] > req-file
  local job_id="$1" provider="$2" scene="$3" free_text="$4" count="$5" extra="${6:-}"
  python3 - "$job_id" "$PLUGIN_ID" "$TRACK_ID" "$provider" "$scene" "$free_text" "$count" "$extra" <<'PY'
import json, sys
job_id, plugin_id, track_id, provider, scene, free_text, count, extra = sys.argv[1:9]
req = {"cmd": "gen_shell.pending",
       "debug_submit": {
           "schema_version": "vit.genshell.request.v1",
           "job_id": job_id, "shell_node_id": plugin_id, "track_id": track_id,
           "start_time": 0.0, "length_beats": 8, "scene": scene,
           "provider_id": provider, "template_id": "song_segment_v1",
           "form": {}, "free_text": free_text,
           "gen_params": {"duration_seconds": 4, "count": int(count),
                          "style_tags": "smoke", "instrumental": False, "title": ""}}}
if extra:
    req["debug_submit"].update(json.loads(extra))
print(json.dumps(req))
PY
}

# ================================================================ preflight
step "Preflight"
if pgrep -f "$KERNEL_BIN" >/dev/null 2>&1; then
  echo "ERROR[env]: a kernel of this binary is already running — single-owner rule (AGENTS §9), aborting" >&2
  exit 2
fi
wait_ports_free || { echo "ERROR[env]: stack ports busy at start — single-owner rule, aborting" >&2; exit 2; }
prereq "ports_free_at_start=confirmed"

{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%F %T')"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_branch=$(git -C "$REPO_ROOT" branch --show-current 2>/dev/null || echo unknown)"
  echo "repo_dirty_files=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ')"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$(sha "$KERNEL_BIN")"
  echo "kernel_mtime=$(stat -f %Sm "$KERNEL_BIN")"
  echo "kernel_source_tree=$(git -C "$(dirname "$KERNEL_BIN")/../.." rev-parse HEAD 2>/dev/null || echo n/a)"
  echo "vst3_bundle=$VST3_BUNDLE"
  echo "vst3_bundle_binary_sha256=$(sha "$VST3_BUNDLE/Contents/MacOS/GenShell" 2>/dev/null || echo missing)"
  echo "legs=$LEGS"
} > "$RUN_DIR/run_meta.txt"
cat "$RUN_DIR/run_meta.txt" >&2

cp "$REAL_DEFAULT_PROJECT" "$RUN_DIR/project/default_project.xml"
prereq "project_copy_sha256=$(sha "$RUN_DIR/project/default_project.xml") (source project never written; VIT_PROJECT_XML redirect)"

# ---------------------------------------------------------------- binaries
if [[ -n "$AGENT_BIN_ARG" ]]; then
  AGENT_BIN="$AGENT_BIN_ARG"
  [[ -x "$AGENT_BIN" ]] || { echo "ERROR[env]: agent binary not executable: $AGENT_BIN" >&2; exit 2; }
else
  log "building agent (go build)..."
  (cd "$REPO_ROOT/agent" && go build -o "$RUN_DIR/bin/vitagent" ./cmd/vitagent) \
    || { echo "ERROR[env]: go build agent failed" >&2; exit 2; }
  AGENT_BIN="$RUN_DIR/bin/vitagent"
fi
prereq "agent_bin=$AGENT_BIN sha256=$(sha "$AGENT_BIN")"

# ---------------------------------------------------------------- P0 provider simulator
SIM_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
SIM_EVIDENCE="$RUN_DIR/sim_evidence.json"
SIM_LOG="$RUN_DIR/logs/provider_simulator.log"
python3 - "$SIM_PORT" "$SIM_EVIDENCE" > "$SIM_LOG" 2>&1 <<'PY' &
import json, math, struct, sys, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

port, evidence_path = int(sys.argv[1]), sys.argv[2]
evidence = {"sync_submit": [], "slow_submit": 0, "suno_submit": [], "suno_polls": 0,
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
            # request-body trace for the injection gates (grep SIM_LOG)
            sys.stderr.write("sync_request_body: " + str(body.get("prompt", ""))[:1600].replace("\n", "\\n") + "\n")
            self._wav(sine_wav(440.0, 0.5))
        elif self.path == "/generate_slow":
            evidence["slow_submit"] += 1
            sys.stderr.write("slow_request_body: " + str(body.get("prompt", ""))[:300].replace("\n", "\\n") + "\n")
            time.sleep(20)
            self._wav(sine_wav(330.0, 0.5))
        elif self.path == "/suno/submit/music":
            evidence["suno_submit"].append(body)
            self._json({"code": "success", "message": "", "data": "task-sim-multi"})
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
                self._json({"task_id": "task-sim-multi", "status": "IN_PROGRESS"})
            else:
                base = f"http://127.0.0.1:{port}"
                self._json({"task_id": "task-sim-multi", "status": "SUCCESS", "data": [
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
  kill -0 "$SIM_PID" 2>/dev/null || { echo "ERROR[env]: provider simulator exited during startup (see $SIM_LOG)" >&2; exit 2; }
  (( SECONDS < deadline )) || { echo "ERROR[env]: provider simulator not ready within 30s" >&2; exit 2; }
  sleep 1
done
prereq "simulator_up port=$SIM_PORT"

PROVIDER_CONFIG="$RUN_DIR/config/genapi_providers.json"
cat > "$PROVIDER_CONFIG" <<EOF
{
  "providers": [
    {"id": "sim-fast", "dialect": "sync_binary", "base_url": "http://127.0.0.1:$SIM_PORT",
     "generate_path": "/generate", "api_key": "", "model": "simulator/fast",
     "download_dir": "$RUN_DIR/cache/fast"},
    {"id": "sim-multi", "dialect": "suno_relay", "base_url": "http://127.0.0.1:$SIM_PORT",
     "api_key": "", "model": "chirp-v3-5", "poll_interval_seconds": 1, "timeout_seconds": 60,
     "download_dir": "$RUN_DIR/cache/multi"},
    {"id": "sim-slow", "dialect": "sync_binary", "base_url": "http://127.0.0.1:$SIM_PORT",
     "generate_path": "/generate_slow", "api_key": "", "model": "simulator/slow",
     "download_dir": "$RUN_DIR/cache/slow"},
    {"id": "sim-broken", "dialect": "sync_json_url", "base_url": "http://127.0.0.1:$SIM_PORT",
     "generate_path": "/v1/make", "api_key": "$FAKE_KEY",
     "download_dir": "$RUN_DIR/cache/broken"}
  ]
}
EOF
prereq "provider_config=$PROVIDER_CONFIG (run-scoped via VIT_GENAPI_CONFIG)"

# ---------------------------------------------------------------- start kernel + agent
KERNEL_LOG="$RUN_DIR/logs/kernel_main.log"
(
  cd "$REPO_ROOT"
  exec env VIT_PROJECT_XML="$RUN_DIR/project/default_project.xml" "$KERNEL_BIN"
) > "$KERNEL_LOG" 2>&1 &
KERNEL_PID=$!
AGENT_LOG="$RUN_DIR/logs/agent_stdout.log"

start_agent() {
  (
    cd "$RUN_DIR"
    VIT_ORCHESTRATION_STORE_PATH="$RUN_DIR/agent_state/orchestration_v1.json" \
    VIT_GENAPI_CONFIG="$PROVIDER_CONFIG" \
      exec "$AGENT_BIN" \
        -http "${AGENT_HTTP#http://}" \
        -last-log-path "$RUN_DIR/agent_state/agent_last.log" \
        -vsp-hub-url ""
  ) >> "$AGENT_LOG" 2>&1 &
  AGENT_PID=$!
}

wait_kernel_req() {
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || { bad "kernel exited during startup"; tail -5 "$KERNEL_LOG" >&2; return 1; }
    [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && return 0
    sleep 1
  done
  bad "kernel REQ port not up in ${STARTUP_TIMEOUT_SECONDS}s"
  return 1
}

wait_agent_health() {
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$AGENT_PID" 2>/dev/null || { bad "agent exited during startup"; tail -20 "$AGENT_LOG" >&2; return 1; }
    curl -sS --max-time 2 -o /dev/null "$AGENT_HTTP/health" 2>/dev/null && return 0
    sleep 1
  done
  bad "agent /health not ready within ${STARTUP_TIMEOUT_SECONDS}s"
  return 1
}

stop_agent_hard() {
  [[ -n "$AGENT_PID" ]] || return 0
  kill -TERM "$AGENT_PID" 2>/dev/null || true
  local start=$SECONDS
  while kill -0 "$AGENT_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$AGENT_PID" 2>/dev/null || true; break; }
    sleep 1
  done
  wait "$AGENT_PID" 2>/dev/null
  AGENT_PID=""
}

step "Kernel + agent start"
if wait_kernel_req; then
  sleep 5  # edit settle
  prereq "kernel_pid=$KERNEL_PID log=$KERNEL_LOG"
  start_agent
  wait_agent_health && prereq "agent_pid=$AGENT_PID http=$AGENT_HTTP" || true
fi

PLUGIN_ID=""
TRACK_ID=""

# ================================================================ A0 runner readiness
if has_leg A0 && [[ -n "$AGENT_PID" ]]; then
  step "A0: gen_shell.status — runner wired and started"
  reply="$(invoke_tool a0_status 30 "gen_shell.status" '{}')"
  st="${reply%%|*}"; out="${reply#*|}"
  STARTED="$(jfield "$out" 'str(d.get("result",{}).get("runner",{}).get("started")) if isinstance(d.get("result"),dict) else ""')"
  ENABLED="$(jfield "$out" 'str(d.get("result",{}).get("runner",{}).get("enabled")) if isinstance(d.get("result"),dict) else ""')"
  PROV_COUNT="$(jfield "$out" 'str(d.get("result",{}).get("runner",{})) and str(len(d.get("result",{}).get("providers") or [])) if isinstance(d.get("result"),dict) else ""')"
  prereq "A0 status=$st started=$STARTED enabled=$ENABLED providers=$PROV_COUNT"
  if [[ "$STARTED" == "True" && "$ENABLED" == "True" ]]; then
    ok "A0: runner started+enabled via chat Server Start single wiring point"
  else
    bad "A0: runner not started/enabled: st=$st started=$STARTED enabled=$ENABLED — $(head -c 300 "$out")"
  fi
fi

# ================================================================ A1 load shell
if has_leg A1; then
  step "A1: set_tempo -> scan -> discover -> add_track -> rack_add_node -> T1 params"
  printf '{"cmd":"set_tempo","bpm":96}' > "$RUN_DIR/replies/A1_tempo_req.json"
  zmq_send "$RUN_DIR/replies/A1_tempo_req.json" "$RUN_DIR/replies/A1_tempo.json" 60000 >/dev/null || true
  A1_TEMPO="$(jfield "$RUN_DIR/replies/A1_tempo.json" 'str(d.get("status",""))')"
  [[ "$A1_TEMPO" == "ok" ]] && ok "A1: set_tempo 96 accepted (music_context determinism)" || bad "A1: set_tempo status=$A1_TEMPO"

  USER_VST3_DIR="$HOME/Library/Audio/Plug-Ins/VST3"
  printf '{"cmd":"scan_plugins","paths":["%s"]}' "$USER_VST3_DIR" > "$RUN_DIR/replies/A1_scan_req.json"
  zmq_send "$RUN_DIR/replies/A1_scan_req.json" "$RUN_DIR/replies/A1_scan_start.json" 60000 >/dev/null || true
  SCAN_ID="$(jfield "$RUN_DIR/replies/A1_scan_start.json" 'str(d.get("scan_id",""))')"
  STATE=""
  deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/A1_status_req.json"
    zmq_send "$RUN_DIR/replies/A1_status_req.json" "$RUN_DIR/replies/A1_status.json" 30000 >/dev/null 2>&1 || true
    STATE="$(jfield "$RUN_DIR/replies/A1_status.json" 'str(d.get("status",""))')"
    [[ "$STATE" != "scanning" && "$STATE" != "cancelling" ]] && break
    sleep 3
  done
  prereq "A1_scan_terminal=$STATE"
  [[ "$STATE" == "completed" ]] && ok "A1: scan completed" || bad "A1: scan state=$STATE"

  printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/A1_list_req.json"
  zmq_send "$RUN_DIR/replies/A1_list_req.json" "$RUN_DIR/replies/A1_list.json" 60000 >/dev/null || true
  IDENTIFIER="$(jfield "$RUN_DIR/replies/A1_list.json" 'str(next((p.get("identifier","") for p in d.get("plugins") or [] if p.get("name")=="GenShell"), ""))')"
  [[ -n "$IDENTIFIER" && "$IDENTIFIER" == *"GenShell"* ]] && ok "A1: GenShell discovered (identifier=$IDENTIFIER)" || bad "A1: GenShell not discovered (identifier='$IDENTIFIER')"

  printf '{"cmd":"add_track"}' > "$RUN_DIR/replies/A1_add_track_req.json"
  zmq_send "$RUN_DIR/replies/A1_add_track_req.json" "$RUN_DIR/replies/A1_add_track.json" 60000 >/dev/null || true
  TRACK_ID="$(jfield "$RUN_DIR/replies/A1_add_track.json" 'str(d.get("track_id",""))')"
  [[ -n "$TRACK_ID" ]] && ok "A1: track added ($TRACK_ID)" || bad "A1: add_track failed"

  if [[ -n "$TRACK_ID" && -n "$IDENTIFIER" ]]; then
    python3 - "$TRACK_ID" "$IDENTIFIER" > "$RUN_DIR/replies/A1_rack_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node", "track_id": sys.argv[1], "plugin_identifier": sys.argv[2], "x": 0, "y": 0}))
PY
    zmq_send "$RUN_DIR/replies/A1_rack_req.json" "$RUN_DIR/replies/A1_rack_add.json" 180000 >/dev/null || true
    PLUGIN_ID="$(jfield "$RUN_DIR/replies/A1_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
    A1_ADD="$(jfield "$RUN_DIR/replies/A1_rack_add.json" 'str(d.get("status",""))')"
    [[ "$A1_ADD" == "ok" && -n "$PLUGIN_ID" ]] && ok "A1: rack_add_node landed (plugin_id=$PLUGIN_ID)" || bad "A1: rack_add_node status=$A1_ADD"
  fi

  if [[ -n "$PLUGIN_ID" ]]; then
    python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/A1_params_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters", "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
    zmq_send "$RUN_DIR/replies/A1_params_req.json" "$RUN_DIR/replies/A1_params.json" 120000 >/dev/null || true
    A1_MISSING="$(jfield "$RUN_DIR/replies/A1_params.json" '",".join(n for n in ["JobState","JobProgress","JobErrorClass","SceneMode","Bypass"] if n not in set(str(p.get("name","")) for p in d.get("parameters") or []))')"
    [[ -z "$A1_MISSING" ]] && ok "A1: T1 five params present" || bad "A1: T1 params missing: $A1_MISSING"
  fi
fi

# ================================================================ A2 happy chain
if has_leg A2 && [[ -n "$PLUGIN_ID" ]]; then
  step "A2: in_project submit -> queued -> claim -> injected prompt -> WAV -> ingest -> done + mirror"
  JOB_A2="smoke-agent1-happy-$(date '+%H%M%S')"
  submit_job "$JOB_A2" sim-fast in_project "SMOKE-A2-HAPPY a warm pad for the verse" 1 > "$RUN_DIR/replies/A2_submit_req.json"
  zmq_send "$RUN_DIR/replies/A2_submit_req.json" "$RUN_DIR/replies/A2_submit.json" 60000 >/dev/null || true
  A2_SUB="$(jfield "$RUN_DIR/replies/A2_submit.json" 'str(d.get("status",""))')"
  [[ "$A2_SUB" == "ok" ]] && ok "A2: debug_submit accepted ($JOB_A2)" || bad "A2: debug_submit status=$A2_SUB"

  printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/A2_pending_req.json"
  zmq_send "$RUN_DIR/replies/A2_pending_req.json" "$RUN_DIR/replies/A2_pending.json" 60000 >/dev/null || true
  A2_Q="$(jfield "$RUN_DIR/replies/A2_pending.json" 'str(next((j.get("state","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A2'"), "missing"))')"
  # runner 轮询 2s——允许已在 running（领取面在应答后即推进）
  if [[ "$A2_Q" == "queued" || "$A2_Q" == "running" || "$A2_Q" == "landing" ]]; then
    ok "A2: pending answer shows job on queue ($A2_Q)"
  else
    bad "A2: pending answer state=$A2_Q"
  fi

  A2_DONE="$(wait_job "$JOB_A2" 'str(j.get("job_state")) if j.get("job_state")=="done" else ""' "$JOB_TIMEOUT_SECONDS" A2)"
  if [[ -n "$A2_DONE" ]]; then
    ok "A2: job terminal done"
  else
    bad "A2: job did not reach done within ${JOB_TIMEOUT_SECONDS}s (last state: $(jfield "$RUN_DIR/replies/A2_state.json" 'str(next((j.get("job_state","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A2'"), "?"))'))"
  fi

  # 整记录字段保持（H4 同面语义）
  A2_REC="$(jfield "$RUN_DIR/replies/A2_state.json" 'j=next((j for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A2'"),{}); "%s|%s|%s" % (j.get("ghost_state",""), j.get("source",""), "y" if j.get("request_hash") else "n")')"
  prereq "A2_record ghost/source/hash=$A2_REC"

  A2_MSG="$(jfield "$RUN_DIR/replies/A2_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A2'"), ""))')"
  A2_CLIP="$(printf '%s' "$A2_MSG" | grep -o 'clip_id=[^ ]*' | head -1 | cut -d= -f2)"
  A2_STACK="$(printf '%s' "$A2_MSG" | grep -o 'stack [^ :]*' | head -1 | cut -d' ' -f2)"
  if [[ "$A2_MSG" == class=none* && -n "$A2_CLIP" && -n "$A2_STACK" ]]; then
    ok "A2: done receipt carries class=none + clip_id=$A2_CLIP + take_stack=$A2_STACK"
  else
    bad "A2: done receipt wrong: $A2_MSG"
  fi

  A2_WAV_COUNT="$(find "$RUN_DIR/cache/fast" -name '*.wav' 2>/dev/null | wc -l | tr -d ' ')"
  [[ "$A2_WAV_COUNT" -ge 1 ]] && ok "A2: artifact WAV on disk (cache/fast count=$A2_WAV_COUNT)" || bad "A2: no WAV artifact under cache/fast"

  sleep 4  # runner 2s 轮询驱动服务刷新 -> T1 镜像
  python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/A2_mirror_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters", "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
  zmq_send "$RUN_DIR/replies/A2_mirror_req.json" "$RUN_DIR/replies/A2_mirror.json" 120000 >/dev/null || true
  A2_MIRROR="$(jfield "$RUN_DIR/replies/A2_mirror.json" 'str(next((p.get("value_text","") for p in d.get("parameters") or [] if p.get("name")=="JobState"), "missing"))')"
  [[ "$A2_MIRROR" == "done" ]] && ok "A2: T1 JobState mirror=done" || bad "A2: JobState mirror=$A2_MIRROR (expected done)"
fi

# 注入块证据（模拟器收到的请求体）——A2/A6 共用判定放各 leg 内
if has_leg A2; then
  A2_PROMPT_LINE="$(grep 'SMOKE-A2-HAPPY' "$SIM_LOG" | head -1 || true)"
  if [[ -n "$A2_PROMPT_LINE" ]]; then
    if [[ "$A2_PROMPT_LINE" == *"== MUSIC CONTEXT"* && "$A2_PROMPT_LINE" == *"== CONSTRAINTS"* ]]; then
      ok "A2: simulator request body carries MUSIC CONTEXT + CONSTRAINTS blocks"
    else
      bad "A2: injection blocks missing in simulator request body: $(printf '%s' "$A2_PROMPT_LINE" | head -c 200)"
    fi
    if [[ "$A2_PROMPT_LINE" == *"tempo: 96.0"* ]]; then
      ok "A2: snapshot tempo 96.0 injected (set_tempo linkage)"
    else
      bad "A2: snapshot tempo missing in prompt body"
    fi
    if [[ "$A2_PROMPT_LINE" == *"...SMOKE-A2-HAPPY a warm pad for the verse"* || "$A2_PROMPT_LINE" == *"SMOKE-A2-HAPPY a warm pad for the verse"* ]]; then
      ok "A2: user free text verbatim in request body"
    else
      bad "A2: user free text not found in request body"
    fi
  else
    bad "A2: simulator never received the A2 request body"
  fi
fi

# ================================================================ A3 multi-candidate
if has_leg A3 && [[ -n "$PLUGIN_ID" ]]; then
  step "A3: count=2 -> two takes on one stack -> switch_asset_take"
  JOB_A3="smoke-agent1-multi-$(date '+%H%M%S')"
  submit_job "$JOB_A3" sim-multi in_project "SMOKE-A3-MULTI two candidates please" 2 > "$RUN_DIR/replies/A3_submit_req.json"
  zmq_send "$RUN_DIR/replies/A3_submit_req.json" "$RUN_DIR/replies/A3_submit.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A3_submit.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A3: submit accepted" || bad "A3: submit failed"

  A3_DONE="$(wait_job "$JOB_A3" 'str(j.get("job_state")) if j.get("job_state")=="done" else ""' "$JOB_TIMEOUT_SECONDS" A3)"
  [[ -n "$A3_DONE" ]] && ok "A3: job done" || bad "A3: job did not reach done"

  A3_MSG="$(jfield "$RUN_DIR/replies/A3_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A3'"), ""))')"
  A3_TAKES="$(printf '%s' "$A3_MSG" | grep -o 'take_id=[^ ;]*' | sed 's/take_id=//' | tr '\n' ' ')"
  A3_CLIP="$(printf '%s' "$A3_MSG" | grep -o 'clip_id=[^ ]*' | head -1 | cut -d= -f2)"
  TAKE_COUNT="$(printf '%s' "$A3_TAKES" | wc -w | tr -d ' ')"
  if [[ "$TAKE_COUNT" == "2" && -n "$A3_CLIP" ]]; then
    ok "A3: two takes landed on one stack (clip=$A3_CLIP takes=$A3_TAKES)"
  else
    bad "A3: take count wrong ($TAKE_COUNT): $A3_MSG"
  fi

  if [[ "$TAKE_COUNT" == "2" && -n "$A3_CLIP" ]]; then
    TAKE2="$(printf '%s' "$A3_TAKES" | awk '{print $2}')"
    python3 - "$A3_CLIP" "$TAKE2" > "$RUN_DIR/replies/A3_switch_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "switch_asset_take", "clip_id": sys.argv[1], "take_id": sys.argv[2]}))
PY
    zmq_send "$RUN_DIR/replies/A3_switch_req.json" "$RUN_DIR/replies/A3_switch.json" 60000 >/dev/null || true
    A3_SWITCH="$(jfield "$RUN_DIR/replies/A3_switch.json" 'str(d.get("active_take_id",""))')"
    if [[ "$(jfield "$RUN_DIR/replies/A3_switch.json" 'str(d.get("status",""))')" == "ok" && "$A3_SWITCH" == "$TAKE2" ]]; then
      ok "A3: switch_asset_take -> active_take=$A3_SWITCH"
    else
      bad "A3: switch failed: $(head -c 200 "$RUN_DIR/replies/A3_switch.json")"
    fi
  fi
fi

# ================================================================ A4 cancel negative probe
if has_leg A4 && [[ -n "$PLUGIN_ID" ]]; then
  step "A4: submit slow -> cancel while running -> cancelled, zero landing, ghost cleared"
  JOB_A4="smoke-agent1-cancel-$(date '+%H%M%S')"
  submit_job "$JOB_A4" sim-slow in_project "SMOKE-A4-CANCEL abort me" 1 > "$RUN_DIR/replies/A4_submit_req.json"
  zmq_send "$RUN_DIR/replies/A4_submit_req.json" "$RUN_DIR/replies/A4_submit.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A4_submit.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A4: slow job submitted" || bad "A4: submit failed"

  A4_RUN="$(wait_job "$JOB_A4" 'str(j.get("job_state")) if j.get("job_state") in ("running","landing") else ""' 30 A4)"
  [[ -n "$A4_RUN" ]] && ok "A4: runner claimed job ($A4_RUN)" || bad "A4: job never reached running"

  python3 - "$JOB_A4" > "$RUN_DIR/replies/A4_cancel_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "gen_shell.pending", "debug_cancel": {"job_id": sys.argv[1]}}))
PY
  zmq_send "$RUN_DIR/replies/A4_cancel_req.json" "$RUN_DIR/replies/A4_cancel.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A4_cancel.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A4: debug_cancel accepted" || bad "A4: debug_cancel failed"

  A4_TERM="$(wait_job "$JOB_A4" 'str(j.get("job_state")) if j.get("job_state")=="cancelled" else ""' "$JOB_TIMEOUT_SECONDS" A4)"
  [[ -n "$A4_TERM" ]] && ok "A4: terminal cancelled" || bad "A4: job did not reach cancelled"

  A4_MSG="$(jfield "$RUN_DIR/replies/A4_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A4'"), ""))')"
  if [[ "$A4_MSG" != *clip_id=* ]]; then
    ok "A4: cancelled job has zero landing evidence"
  else
    bad "A4: cancelled job landed a clip: $A4_MSG"
  fi

  # ghost 清除（服务终态下行；需要一次 pending 刷新——runner 轮询自带）
  sleep 4
  A4_GHOST="$(wait_job "$JOB_A4" 'str(j.get("ghost_state")) if j.get("ghost_state")=="bypass" else ""' 30 A4ghost)"
  [[ -n "$A4_GHOST" ]] && ok "A4: ghost cleared (bypass) after cancelled terminal" || bad "A4: ghost not cleared ($(jfield "$RUN_DIR/replies/A4ghost_state.json" 'str(next((j.get("ghost_state","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A4'"), "?"))'))"
fi

# ================================================================ A5 failure classification
if has_leg A5 && [[ -n "$PLUGIN_ID" ]]; then
  step "A5: 401 provider -> failed class=auth_error + JobErrorClass mirror"
  JOB_A5="smoke-agent1-auth-$(date '+%H%M%S')"
  submit_job "$JOB_A5" sim-broken in_project "SMOKE-A5-AUTH expect rejection" 1 > "$RUN_DIR/replies/A5_submit_req.json"
  zmq_send "$RUN_DIR/replies/A5_submit_req.json" "$RUN_DIR/replies/A5_submit.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A5_submit.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A5: submit accepted" || bad "A5: submit failed"

  A5_TERM="$(wait_job "$JOB_A5" 'str(j.get("job_state")) if j.get("job_state")=="failed" else ""' "$JOB_TIMEOUT_SECONDS" A5)"
  [[ -n "$A5_TERM" ]] && ok "A5: terminal failed" || bad "A5: job did not reach failed"

  A5_MSG="$(jfield "$RUN_DIR/replies/A5_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A5'"), ""))')"
  [[ "$A5_MSG" == *"class=auth_error"* ]] && ok "A5: status class=auth_error ($A5_MSG)" || bad "A5: classification wrong: $A5_MSG"

  sleep 4
  python3 - "$TRACK_ID" "$PLUGIN_ID" > "$RUN_DIR/replies/A5_mirror_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "get_plugin_parameters", "track_id": sys.argv[1], "plugin_id": sys.argv[2]}))
PY
  zmq_send "$RUN_DIR/replies/A5_mirror_req.json" "$RUN_DIR/replies/A5_mirror.json" 120000 >/dev/null || true
  A5_MIRROR="$(jfield "$RUN_DIR/replies/A5_mirror.json" 'str(next((p.get("value_text","") for p in d.get("parameters") or [] if p.get("name")=="JobErrorClass"), "missing"))')"
  [[ "$A5_MIRROR" == "auth_error" ]] && ok "A5: T1 JobErrorClass mirror=auth_error" || bad "A5: JobErrorClass mirror=$A5_MIRROR"
fi

# ================================================================ A6 free_creation
if has_leg A6 && [[ -n "$PLUGIN_ID" ]]; then
  step "A6: scene=free_creation -> zero injection blocks + no landing + artifact receipt"
  JOB_A6="smoke-agent1-free-$(date '+%H%M%S')"
  submit_job "$JOB_A6" sim-fast free_creation "SMOKE-A6-FREE totally free exploration" 1 > "$RUN_DIR/replies/A6_submit_req.json"
  zmq_send "$RUN_DIR/replies/A6_submit_req.json" "$RUN_DIR/replies/A6_submit.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A6_submit.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A6: submit accepted" || bad "A6: submit failed"

  A6_TERM="$(wait_job "$JOB_A6" 'str(j.get("job_state")) if j.get("job_state")=="done" else ""' "$JOB_TIMEOUT_SECONDS" A6)"
  [[ -n "$A6_TERM" ]] && ok "A6: terminal done" || bad "A6: job did not reach done"

  A6_MSG="$(jfield "$RUN_DIR/replies/A6_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A6'"), ""))')"
  if [[ "$A6_MSG" != *clip_id=* && "$A6_MSG" == *free_creation* && "$A6_MSG" == *.wav* ]]; then
    ok "A6: free_creation receipt = artifact paths, no landing"
  else
    bad "A6: receipt wrong: $A6_MSG"
  fi

  A6_LINE="$(grep 'SMOKE-A6-FREE' "$SIM_LOG" | head -1 || true)"
  if [[ -n "$A6_LINE" ]]; then
    if [[ "$A6_LINE" != *"== MUSIC CONTEXT"* && "$A6_LINE" != *"== CONSTRAINTS"* ]]; then
      ok "A6: simulator request body has ZERO injection blocks (exempt scene)"
    else
      bad "A6: exempt scene leaked auto blocks: $(printf '%s' "$A6_LINE" | head -c 200)"
    fi
  else
    bad "A6: simulator never received the A6 request body"
  fi
fi

# ================================================================ A7 orphan sweep (agent restart)
if has_leg A7 && [[ -n "$PLUGIN_ID" && -n "$AGENT_PID" ]]; then
  step "A7: running job -> agent SIGTERM -> restart -> interrupted sweep"
  JOB_A7="smoke-agent1-orphan-$(date '+%H%M%S')"
  submit_job "$JOB_A7" sim-slow in_project "SMOKE-A7-ORPHAN will be interrupted" 1 > "$RUN_DIR/replies/A7_submit_req.json"
  zmq_send "$RUN_DIR/replies/A7_submit_req.json" "$RUN_DIR/replies/A7_submit.json" 60000 >/dev/null || true
  [[ "$(jfield "$RUN_DIR/replies/A7_submit.json" 'str(d.get("status",""))')" == "ok" ]] && ok "A7: slow job submitted" || bad "A7: submit failed"

  A7_RUN="$(wait_job "$JOB_A7" 'str(j.get("job_state")) if j.get("job_state") in ("running","landing") else ""' 30 A7)"
  [[ -n "$A7_RUN" ]] && ok "A7: job running at agent shutdown" || bad "A7: job never reached running"

  log "A7: SIGTERM agent pid=$AGENT_PID..."
  stop_agent_hard
  prereq "A7_agent_stopped pid_reaped"
  sleep 2

  log "A7: restarting agent (simulating agent restart)..."
  start_agent
  wait_agent_health && prereq "A7_agent_restarted pid=$AGENT_PID"

  A7_TERM="$(wait_job "$JOB_A7" 'str(j.get("job_state")) if j.get("job_state")=="interrupted" else ""' "$JOB_TIMEOUT_SECONDS" A7)"
  if [[ -n "$A7_TERM" ]]; then
    A7_MSG="$(jfield "$RUN_DIR/replies/A7_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A7'"), ""))')"
    ok "A7: runner restart swept job -> interrupted ($A7_MSG)"
  else
    bad "A7: job not swept to interrupted (state: $(jfield "$RUN_DIR/replies/A7_state.json" 'str(next((j.get("job_state","") for j in d.get("jobs") or [] if j.get("job_id")=="'$JOB_A7'"), "?"))'))"
  fi
fi

# ================================================================ A8 conversation-flow submit + management plane
if has_leg A8 && [[ -n "$PLUGIN_ID" && -n "$AGENT_PID" && -n "$TRACK_ID" ]]; then
  step "A8: gen_shell.stop -> gen_shell.submit (HTTP) -> pending queued -> gen_shell.start -> done"
  reply="$(invoke_tool a8_stop 30 "gen_shell.stop" '{}')"
  [[ "${reply%%|*}" == "ok" ]] && ok "A8: gen_shell.stop accepted (claiming paused)" || bad "A8: gen_shell.stop failed: $(head -c 200 "${reply#*|}")"

  python3 - "$TRACK_ID" > "$RUN_DIR/http/a8_submit_args.json" <<'PY'
import json, sys
print(json.dumps({"free_text": "SMOKE-A8-CHAT generate a conversation-flow pad",
                  "track_id": sys.argv[1], "provider_id": "sim-fast",
                  "start_time": 0.0, "length_beats": 8, "count": 1}))
PY
  A8_ARGS="$(cat "$RUN_DIR/http/a8_submit_args.json")"
  reply="$(invoke_tool a8_submit 60 "gen_shell.submit" "$A8_ARGS")"
  A8_ST="${reply%%|*}"; A8_OUT="${reply#*|}"
  A8_JOB="$(jfield "$A8_OUT" 'str(d.get("result",{}).get("job_id","")) if isinstance(d.get("result"),dict) else ""')"
  A8_QUEUED="$(jfield "$A8_OUT" 'str(d.get("result",{}).get("queued","")) if isinstance(d.get("result"),dict) else ""')"
  prereq "A8_submit status=$A8_ST job=$A8_JOB queued=$A8_QUEUED"
  if [[ "$A8_ST" == "ok" && -n "$A8_JOB" ]]; then
    ok "A8: gen_shell.submit accepted job=$A8_JOB (conversation flow -> HOST queue)"
  else
    bad "A8: gen_shell.submit failed: $(head -c 300 "$A8_OUT")"
  fi

  if [[ -n "$A8_JOB" ]]; then
    # runner 已停——queued 状态必须稳定可见（等待即领取的竞态被消除）
    sleep 2
    printf '{"cmd":"gen_shell.pending"}' > "$RUN_DIR/replies/A8_pending_req.json"
    zmq_send "$RUN_DIR/replies/A8_pending_req.json" "$RUN_DIR/replies/A8_pending.json" 60000 >/dev/null || true
    A8_Q="$(jfield "$RUN_DIR/replies/A8_pending.json" 'str(next((j.get("state","") for j in d.get("jobs") or [] if j.get("job_id")=="'$A8_JOB'"), "missing"))')"
    [[ "$A8_Q" == "queued" ]] && ok "A8: gen_shell.pending answers state=queued (paused runner)" || bad "A8: pending state=$A8_Q (expected queued with runner paused)"

    reply="$(invoke_tool a8_start 30 "gen_shell.start" '{}')"
    [[ "${reply%%|*}" == "ok" ]] && ok "A8: gen_shell.start resumed claiming" || bad "A8: gen_shell.start failed"

    A8_TERM="$(wait_job "$A8_JOB" 'str(j.get("job_state")) if j.get("job_state")=="done" else ""' "$JOB_TIMEOUT_SECONDS" A8)"
    if [[ -n "$A8_TERM" ]]; then
      A8_MSG="$(jfield "$RUN_DIR/replies/A8_state.json" 'str(next((j.get("status_message","") for j in d.get("jobs") or [] if j.get("job_id")=="'$A8_JOB'"), ""))')"
      if [[ "$A8_MSG" == class=none* && "$A8_MSG" == *clip_id=* ]]; then
        ok "A8: chain closed — submit -> queued -> claim (simulator) -> done + landed"
      else
        bad "A8: terminal done but receipt wrong: $A8_MSG"
      fi
    else
      bad "A8: conversation job did not reach done"
    fi
  fi
fi

# ================================================================ A9 secrets discipline
if has_leg A9; then
  step "A9: zero key material in simulator/kernel/agent logs + pending answers + HTTP replies"
  LEAK_HITS=0
  for target in "$SIM_LOG" "$KERNEL_LOG" "$AGENT_LOG" "$RUN_DIR/agent_state/agent_last.log" "$RUN_DIR"/replies/A2_pending.json "$RUN_DIR"/replies/A8_pending.json "$RUN_DIR"/replies/A2_submit.json "$RUN_DIR"/http/a8_submit.json; do
    [[ -f "$target" ]] || continue
    hits="$(grep -c "$FAKE_KEY" "$target" 2>/dev/null || true)"
    if [[ "$hits" != "0" && -n "$hits" ]]; then
      LEAK_HITS=$((LEAK_HITS + hits))
      bad "A9: key material leaked in $target ($hits hits)"
    fi
  done
  if [[ "$LEAK_HITS" == "0" ]]; then
    ok "A9: zero key hits across simulator log / kernel log / agent logs / pending answers / HTTP replies"
  fi

  A9_PROV="$(jfield "$RUN_DIR/replies/A2_pending.json" 'str(isinstance(d.get("providers"), list))')"
  [[ "$A9_PROV" == "True" ]] && ok "A9: pending answer providers array present (v1 kernel-empty)" || bad "A9: pending providers missing"
fi

# ================================================================ teardown
step "Teardown"
stop_agent_hard && prereq "agent_stopped"
if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
  kill -TERM "$KERNEL_PID" 2>/dev/null || true
  start=$SECONDS
  while kill -0 "$KERNEL_PID" 2>/dev/null; do
    (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$KERNEL_PID" 2>/dev/null || true; break; }
    sleep 1
  done
  wait "$KERNEL_PID" 2>/dev/null
fi
KERNEL_PID=""
[[ -n "$SIM_PID" ]] && { kill -TERM "$SIM_PID" 2>/dev/null || true; sleep 1; kill -9 "$SIM_PID" 2>/dev/null || true; }
SIM_PID=""
sleep 2
if wait_ports_free; then
  prereq "ports_free_after_teardown=confirmed"
else
  bad "ports still busy after teardown"
fi

cp "$SIM_LOG" "$RUN_DIR/logs/provider_simulator_copy.log" 2>/dev/null || true
{
  echo "finished_at=$(date '+%F %T')"
  echo "overall_pass=$OVERALL_PASS"
} >> "$RUN_DIR/run_meta.txt"

if [[ "$OVERALL_PASS" -eq 1 ]]; then
  echo ""
  echo "ALL GREEN: gen_shell_agent_smoke_mac $RUN_ID (run dir: $RUN_DIR)"
  exit 0
fi
echo ""
echo "RED PRESENT: see $PREREQ_FILE"
exit 1
