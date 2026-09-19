#!/usr/bin/env bash
# run_project_stems_import_smoke mac equivalent — PORT-SMOKE-MAC-1 (script 3/7).
#
# Mac port of scripts/run_project_stems_import_smoke.ps1. Same two-step
# semantics — prepare a VitApp kernel, then drive the cross-platform probe
# scripts/project_stems_import_probe.py over kernel ZMQ and require its report
# status == passed:
#
#   | ps1 (run_project_stems_import_smoke.ps1)     | mac (this script)           |
#   |----------------------------------------------|-----------------------------|
#   | param($RepoRoot="D:\Vit_DAW", $KernelExe,    | flags below (same names)    |
#   |   $TrainingFolder="E:\…yingge - sattelites   |                             |
#   |   tracks out", $SealedFolder="E:\…Weekend   |   --sealed-folder optional   |
#   |   Lover tracks out", $PythonExe, $ZmqReqPort, |   (missing folder = skip,   |
#   |   -ReuseKernel, -KeepProcess,                 |   same as ps1)              |
#   |   $TimeoutSeconds=180)                        |   default 180               |
#   | Resolve-KernelExe: PC Release/Export candi-  | cmake+make Debug build in   |
#   |   dates                                     |   the run workdir (A5) or   |
#   |                                              |   --kernel-bin reuse        |
#   | Get-NetTCPListener / Wait-TcpListener 5555   | lsof port_listener_pid      |
#   | Start-Process kernel (cwd = exe dir → repo   | kernel under a fake VitApp  |
#   |   Workspace; ps1 fingerprints it instead)    |   root in the workdir +     |
#   |                                              |   VIT_PROJECT_XML copy —    |
#   |                                              |   repo tree never written   |
#   |                                              |   (AGENTS §10, A5/C2/       |
#   |                                              |   JOURNEY-1-MAC pattern)    |
#   | TrainingFolder = PC-local real stems         | default synthesizes 4       |
#   |   (E:\BaiduNetdiskDownload\…)                |   deterministic stems in    |
#   |                                              |   the run workdir (mac      |
#   |                                              |   machine fact: that audio  |
#   |                                              |   is not on this machine);  |
#   |                                              |   --training-folder overrid |
#   | $PythonExe = "python"                        | python3 + pyzmq preflight   |
#   | Artifacts under RepoRoot\VitApp\Workspace\   | artifacts under a fresh run |
#   |   Artifacts\smoke\project_stems_import_*     |   dir (default              |
#   |                                              |   ~/Documents/vit-smoke-    |
#   |                                              |   mac1-artifacts/<run_id>)  |
#   | summary.json wrapper schema                  | same fields, suffix .mac.v1 |
#   |   project_stems_import_smoke_wrapper.v1      |   + run meta (HEAD+dirty)   |
#   | probe exit/status checks + $probeReport.     | identical (probe JSON       |
#   |   assertions into summary                    |   status + assertions)      |
#   | Stop-Process started kernel (unless         | stop_process SIGTERM→grace→ |
#   |   -KeepProcess)                              |   SIGKILL + wait (A1)       |
#   | ReuseKernel: attach to port owner            | --reuse-kernel: attach to   |
#   |                                              |   whoever owns 5555; other- |
#   |                                              |   wise a busy port is an    |
#   |                                              |   env failure (AGENTS §9)   |
#
# §8 discipline: deterministic kernel+probe chain — ≤3 valid runs; success =
# single run exit 0 with the probe report status "passed" (its internal
# assertions all green); same-breakpoint two-failure stop-loss.

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_ARG=""
TRAINING_FOLDER_ARG=""
SEALED_FOLDER_ARG=""
PYTHON_BIN="python3"
ZMQ_REQ_PORT=5555
REUSE_KERNEL=0
KEEP_PROCESS=0
TIMEOUT_SECONDS=180
WORKDIR_ARG=""
TRACKTION_DIR=""
ARTIFACT_ROOT_ARG=""

ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557

usage() {
  cat <<'EOF'
run_project_stems_import_smoke_mac.sh — mac port of
run_project_stems_import_smoke.ps1 (PORT-SMOKE-MAC-1).

Options:
  --repo-root PATH        Repository root (default: parent of this script's dir)
  --kernel-bin PATH       Reuse an existing VitApp kernel binary (records
                          sha256+mtime; skips the cmake kernel build)
  --training-folder PATH  Folder with readable audio for the import preflight
                          (default: 4 synthetic stems created in the run dir —
                          mac machine fact, the PC E:\ training audio is not
                          on this machine)
  --python PATH           Python interpreter (default python3; needs pyzmq)
  --sealed-folder PATH    Optional sealed-audio folder for the read-only
                          sealed preflight (missing folder = skipped, same
                          as the ps1; PC default E:\ folder is not on mac)
  --zmq-req-port PORT     Kernel REQ port (default 5555)
  --reuse-kernel          Attach to an existing 5555 listener instead of
                          failing (AGENTS §9 single-owner rule)
  --keep-process          Do not stop a kernel this run started
  --timeout-seconds N     Startup/REQ timeouts (default 180)
  --workdir PATH          Reuse PATH as the run artifact dir
  --tracktion-dir PATH    tracktion_engine source dir for the kernel build
                          (worktree checkouts have an empty submodule — point
                          at any checkout of the pinned commit)
  --artifact-root DIR     Artifact root (default ~/Documents/vit-smoke-mac1-artifacts)
  -h, --help              Show this help

Exit codes: 0 = probe passed; 1 = functional failure; 2 = environment failure.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_ARG="$2"; shift 2 ;;
    --training-folder) TRAINING_FOLDER_ARG="$2"; shift 2 ;;
    --sealed-folder) SEALED_FOLDER_ARG="$2"; shift 2 ;;
    --python) PYTHON_BIN="$2"; shift 2 ;;
    --zmq-req-port) ZMQ_REQ_PORT="$2"; shift 2 ;;
    --reuse-kernel) REUSE_KERNEL=1; shift ;;
    --keep-process) KEEP_PROCESS=1; shift ;;
    --timeout-seconds) TIMEOUT_SECONDS="$2"; shift 2 ;;
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
fail_env()      { echo "ERROR[env]: $*" >&2; exit 2; }
fail_functional(){ echo "ERROR[functional]: $*" >&2; exit 1; }

json_field() {
  # json_field <file> <python expr against d>
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

RUN_ID="project_stems_import_mac_$(date '+%Y%m%d-%H%M%S')"
ARTIFACT_ROOT="${ARTIFACT_ROOT_ARG:-$HOME/Documents/vit-smoke-mac1-artifacts}"
WORKDIR="${WORKDIR_ARG:-$ARTIFACT_ROOT/$RUN_ID}"
mkdir -p "$WORKDIR"/{logs,temp_project} || fail_env "cannot create artifact dir: $WORKDIR"

KERNEL_ROOT="$WORKDIR/kernel_root"
mkdir -p "$KERNEL_ROOT/Source"
: > "$KERNEL_ROOT/CMakeLists.txt"
KERNEL_WORKSPACE="$WORKDIR/kernel_workspace"
mkdir -p "$KERNEL_WORKSPACE"
TEMP_PROJECT_PATH="$WORKDIR/temp_project/project_stems_import.vit"
PROBE_OUTPUT="$WORKDIR/project_stems_import_probe.json"
PROBE_LOG="$WORKDIR/project_stems_import_probe.log"
SUMMARY_PATH="$WORKDIR/summary.json"
REPO_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"

KERNEL_PID=""
KERNEL_STOP_RECORD=""

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
  KERNEL_STOP_RECORD="${signal}:${code}:${elapsed}"
}

cleanup() {
  local rc=$?
  if [[ -n "$KERNEL_PID" && "$KEEP_PROCESS" -ne 1 ]]; then
    log "stopping kernel (pid $KERNEL_PID)..."
    stop_process "$KERNEL_PID"
    log "kernel stop: $KERNEL_STOP_RECORD"
  fi
  exit $rc
}
trap cleanup EXIT INT TERM

step "Preflight: repo + python + pyzmq"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%Y-%m-%dT%H:%M:%S%z')"
  echo "script=run_project_audio_settings_preflight_smoke_mac.sh (mac port of run_project_audio_settings_preflight_smoke.ps1)"
  echo "repo_root=$REPO_ROOT"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "repo_dirty=$(git -C "$REPO_ROOT" status --short 2>/dev/null | wc -l | tr -d ' ') entries"
  echo "zmq_req_port=$ZMQ_REQ_PORT"
  echo "training_folder=${TRAINING_FOLDER_ARG:-<synthesized in run dir>}"
  echo "sealed_folder=${SEALED_FOLDER_ARG:-<skipped>}"
} > "$WORKDIR/run_meta.txt"
cat "$WORKDIR/run_meta.txt" >&2

[[ -f "$REPO_ROOT/agent/go.mod" ]] || fail_env "agent go.mod not found under $REPO_ROOT/agent"
[[ -f "$REPO_ROOT/scripts/project_stems_import_probe.py" ]] || fail_env "probe script missing: $REPO_ROOT/scripts/project_stems_import_probe.py"
[[ -f "$REPO_DEFAULT_PROJECT" ]] || fail_env "repo default project missing: $REPO_DEFAULT_PROJECT"
"$PYTHON_BIN" -c "import zmq" 2>/dev/null || fail_env "$PYTHON_BIN cannot import zmq (pyzmq) — install with: pip3 install --user pyzmq"

# ---------------------------------------------------------------- training stems
if [[ -z "$TRAINING_FOLDER_ARG" ]]; then
  step "Provision synthetic training stems (mac machine fact: PC E:\\ training audio is not on this machine)"
  TRAINING_FOLDER="$WORKDIR/training_stems"
  mkdir -p "$TRAINING_FOLDER"
  python3 - "$TRAINING_FOLDER" > "$WORKDIR/training_stems_provisioning.txt" <<'PY'
import math, os, struct, sys, wave

out_dir = sys.argv[1]
RATE, TAU = 44100, math.tau

def lcg(seed):
    state = seed
    while True:
        state = (state * 1103515245 + 12345) & 0x7FFFFFFF
        yield state

def write_stem(name, fn, stereo=True):
    target = os.path.join(out_dir, name + ".wav")
    rng = lcg(0x9E3779B1 ^ (sum(ord(c) for c in name) << 8))
    with wave.open(target, "wb") as w:
        w.setnchannels(2)
        w.setsampwidth(2)
        w.setframerate(RATE)
        chunks = []
        for i in range(int(RATE * 6.0)):
            t = i / RATE
            s = max(-0.95, min(0.95, fn(t, rng)))
            left = int(s * 32767)
            right = int((0.85 * s) * 32767) if stereo else left
            chunks.append(struct.pack("<hh", left, right))
        w.writeframes(b"".join(chunks))
    print("wrote", name, os.path.getsize(target), "bytes")

write_stem("bass",   lambda t, r: 0.42 * math.sin(TAU * 55 * t) * (0.7 + 0.3 * math.sin(TAU * 0.5 * t)))
write_stem("drums",  lambda t, r: 0.28 * ((next(r) / 0x7FFFFFFF - 0.5) * 2.0) * math.exp(-18.0 * (t % 0.5)) + 0.5 * math.sin(TAU * 60 * (t % 0.5)) * math.exp(-9.0 * (t % 0.5)))
write_stem("guitar", lambda t, r: 0.34 * math.exp(-1.6 * (t % 1.25)) * math.sin(TAU * [196.0, 247.0, 294.0, 392.0][int(t / 1.25) % 4] * t))
write_stem("vocals", lambda t, r: 0.3 * (math.sin(TAU * 196.0 * t) + 0.5 * math.sin(TAU * 392.0 * t)) * (0.65 + 0.35 * math.sin(TAU * 0.8 * t)))
PY
  cat "$WORKDIR/training_stems_provisioning.txt" >&2
  for stem in bass drums guitar vocals; do
    [[ -f "$TRAINING_FOLDER/$stem.wav" ]] || fail_env "training stem synthesis failed for $stem.wav"
  done
  ok "4 synthetic stems at $TRAINING_FOLDER"
else
  TRAINING_FOLDER="$TRAINING_FOLDER_ARG"
  [[ -d "$TRAINING_FOLDER" ]] || fail_env "training folder does not exist: $TRAINING_FOLDER"
fi

# ------------------------------------------------------------- sealed folder
SEALED_FOLDER_RESOLVED=""
if [[ -n "$SEALED_FOLDER_ARG" ]]; then
  [[ -d "$SEALED_FOLDER_ARG" ]] || fail_env "sealed folder does not exist: $SEALED_FOLDER_ARG"
  SEALED_FOLDER_RESOLVED="$SEALED_FOLDER_ARG"
fi

# ---------------------------------------------------------------- kernel binary
KERNEL_BIN=""
if [[ -n "$KERNEL_BIN_ARG" ]]; then
  [[ -x "$KERNEL_BIN_ARG" ]] || fail_env "kernel binary is not executable: $KERNEL_BIN_ARG"
  KERNEL_BIN="$KERNEL_BIN_ARG"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  stat -f "kernel_bin_mtime=%Sm kernel_bin_size=%z" "$KERNEL_BIN" > "$WORKDIR/kernel_bin.stat"
  log "using provided kernel binary: $KERNEL_BIN (sha256/mtime recorded)"
else
  if [[ -z "$TRACKTION_DIR" ]]; then
    TRACKTION_DIR="$REPO_ROOT/tracktion_engine"
  fi
  [[ -f "$TRACKTION_DIR/CMakeLists.txt" ]] \
    || fail_env "tracktion_engine not usable at $TRACKTION_DIR (submodule not checked out? pass --tracktion-dir or --kernel-bin)"
  log "building VitApp kernel (cmake+make, sources at $REPO_ROOT/VitApp, tracktion at $TRACKTION_DIR)..."
  if ! cmake -S "$REPO_ROOT/VitApp" -B "$WORKDIR/build/vitapp" -G "Unix Makefiles" \
        -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR="$TRACKTION_DIR" \
        > "$WORKDIR/logs/kernel_configure.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_configure.log" >&2
    fail_env "kernel cmake configure failed (see $WORKDIR/logs/kernel_configure.log)"
  fi
  if ! make -C "$WORKDIR/build/vitapp" -j"$(sysctl -n hw.ncpu)" VitApp > "$WORKDIR/logs/kernel_build.log" 2>&1; then
    tail -20 "$WORKDIR/logs/kernel_build.log" >&2
    fail_env "kernel make failed (see $WORKDIR/logs/kernel_build.log)"
  fi
  KERNEL_BIN="$WORKDIR/build/vitapp/VitApp_artefacts/Debug/VitApp"
  [[ -x "$KERNEL_BIN" ]] || fail_env "kernel binary not found after build: $KERNEL_BIN"
  shasum -a 256 "$KERNEL_BIN" > "$WORKDIR/kernel_bin.sha256"
  log "kernel built: $KERNEL_BIN"
fi

write_summary() {
  local status="$1" error="${2:-}" ended_at
  ended_at="$(date '+%Y-%m-%dT%H:%M:%S%z')"
  python3 - "$SUMMARY_PATH" "$RUN_ID" "$WORKDIR" "$status" "$error" "$ended_at" \
    "$KERNEL_BIN" "$TRAINING_FOLDER" "$TEMP_PROJECT_PATH" "$PROBE_OUTPUT" \
    "$KERNEL_STOP_RECORD" "$KERNEL_PID" "$REPO_ROOT" "$SEALED_FOLDER_RESOLVED" <<'PY'
import json, os, sys
(out, run_id, workdir, status, error, ended_at, kernel_bin, training_folder,
 project_path, probe_output, kernel_stop, kernel_pid, repo_root, sealed_folder) = sys.argv[1:15]
steps = []
probe_meta = {}
if os.path.isfile(probe_output):
    try:
        probe = json.load(open(probe_output, encoding="utf-8"))
        steps.append({"name": "Run project stems import probe",
                      "status": "passed" if probe.get("status") == "passed" else "failed",
                      "log_path": os.path.join(workdir, "project_audio_preflight_probe.log"),
                      "output_path": probe_output})
        probe_meta = {"probe_assertions": probe.get("assertions", {}),
                      "probe_status": probe.get("status", ""),
                      "probe_error": probe.get("error", "")}
    except Exception as exc:
        probe_meta = {"probe_parse_error": str(exc)}
else:
    steps.append({"name": "Run project stems import probe",
                  "status": "not_reached"})
steps.insert(0, {"name": "Prepare VitApp kernel", "status": "passed" if kernel_pid else "failed",
                 "pid": int(kernel_pid) if kernel_pid else None})
summary = {
    "schema_version": "project_stems_import_smoke_wrapper.mac.v1",
    "card": "PORT-SMOKE-MAC-1",
    "ps1_source": "scripts/run_project_stems_import_smoke.ps1",
    "run_id": run_id,
    "status": status,
    "error": error,
    "started_at_meta": "run_meta.txt",
    "ended_at": ended_at,
    "repo_root": repo_root,
    "kernel_exe": kernel_bin,
    "kernel_sha256": open(os.path.join(workdir, "kernel_bin.sha256")).read().split()[0] if os.path.isfile(os.path.join(workdir, "kernel_bin.sha256")) else "",
    "training_folder": training_folder,
    "sealed_folder": sealed_folder,
    "artifact_dir": workdir,
    "temp_project_path": project_path,
    "probe_output": probe_output,
    "kernel_stop": {"record": kernel_stop},
    "steps": steps,
}
summary.update(probe_meta)
with open(out, "w", encoding="utf-8") as f:
    json.dump(summary, f, ensure_ascii=False, indent=2)
PY
}

# ---------------------------------------------------------------- kernel start
step "Prepare VitApp kernel"
LISTENER_PID="$(port_listener_pid "$ZMQ_REQ_PORT")"
if [[ -n "$LISTENER_PID" ]]; then
  [[ "$REUSE_KERNEL" -eq 1 ]] \
    || fail_env "kernel command port $ZMQ_REQ_PORT is already in use by pid $LISTENER_PID; AGENTS §9 single-owner rule — pass --reuse-kernel if that is intentional"
  ok "reusing kernel command port $ZMQ_REQ_PORT pid=$LISTENER_PID"
else
  cp "$REPO_DEFAULT_PROJECT" "$KERNEL_WORKSPACE/default_project.xml"
  log "starting kernel (cwd=$KERNEL_ROOT, VIT_PROJECT_XML isolated)..."
  (
    cd "$KERNEL_ROOT"
    exec env VIT_PROJECT_XML="$KERNEL_WORKSPACE/default_project.xml" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             "$KERNEL_BIN"
  ) > "$WORKDIR/logs/kernel_stdout.log" 2>&1 &
  KERNEL_PID=$!
  deadline=$((SECONDS + TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || {
      tail -30 "$WORKDIR/logs/kernel_stdout.log" >&2
      fail_env "kernel process exited during startup (see $WORKDIR/logs/kernel_stdout.log)"
    }
    [[ -n "$(port_listener_pid "$ZMQ_REQ_PORT")" ]] && break
    sleep 1
  done
  [[ -n "$(port_listener_pid "$ZMQ_REQ_PORT")" ]] \
    || fail_env "kernel command port $ZMQ_REQ_PORT did not become ready within ${TIMEOUT_SECONDS}s"
  for port in "$ZMQ_REQ_PORT" "$ZMQ_PUB_PORT" "$ZMQ_LOG_PORT"; do
    owner="$(port_listener_pid "$port")"
    [[ "$owner" == "$KERNEL_PID" ]] \
      || fail_env "port $port listener pid $owner != kernel pid $KERNEL_PID"
  done
  ok "started kernel pid=$KERNEL_PID (ports $ZMQ_REQ_PORT/$ZMQ_PUB_PORT/$ZMQ_LOG_PORT)"
fi

# ---------------------------------------------------------------- probe
step "Run project stems import probe"
REQ_TIMEOUT_MS=$(( TIMEOUT_SECONDS * 1000 ))
(( REQ_TIMEOUT_MS >= 30000 )) || REQ_TIMEOUT_MS=180000
set +e
SEALED_ARGS=()
[[ -n "$SEALED_FOLDER_RESOLVED" ]] && SEALED_ARGS=(--sealed-folder "$SEALED_FOLDER_RESOLVED")
"$PYTHON_BIN" "$REPO_ROOT/scripts/project_stems_import_probe.py" \
  --req-url "tcp://127.0.0.1:$ZMQ_REQ_PORT" \
  --training-folder "$TRAINING_FOLDER" \
  --project-path "$TEMP_PROJECT_PATH" \
  --output "$PROBE_OUTPUT" \
  --req-timeout-ms "$REQ_TIMEOUT_MS" ${SEALED_ARGS[@]+"${SEALED_ARGS[@]}"} 2>&1 | tee "$PROBE_LOG"
PROBE_EXIT="${PIPESTATUS[0]}"
set -e
[[ "$PROBE_EXIT" -eq 0 ]] \
  || { write_summary failed "project_stems_import_probe.py failed with exit code $PROBE_EXIT; log=$PROBE_LOG"; \
       fail_functional "project_stems_import_probe.py failed with exit code $PROBE_EXIT; log=$PROBE_LOG"; }
[[ -f "$PROBE_OUTPUT" ]] || { write_summary failed "probe produced no report at $PROBE_OUTPUT"; fail_functional "probe produced no report at $PROBE_OUTPUT"; }
PROBE_STATUS="$(json_field "$PROBE_OUTPUT" 'str(d.get("status",""))')"
if [[ "$PROBE_STATUS" != "passed" ]]; then
  PROBE_ERROR="$(json_field "$PROBE_OUTPUT" 'str(d.get("error",""))')"
  write_summary failed "project stems import probe failed: $PROBE_ERROR"
  fail_functional "project stems import probe failed: $PROBE_ERROR"
fi

write_summary passed ""
ok "project stems import smoke passed; summary=$SUMMARY_PATH"
log "workdir: $WORKDIR"
exit 0
