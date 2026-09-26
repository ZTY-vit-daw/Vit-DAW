#!/usr/bin/env bash
# kernel_pluginlist_hygiene_smoke_pc.sh — PORT-WIN-HYGIENE-SMOKE-1
# Windows (Git Bash) port of the mac kernel-pluginlist-hygiene real-stack legs
# (scripts/kernel_pluginlist_hygiene_smoke_mac.sh), reduced to the decisive
# Windows-replay set for the bundle-aware predicate fix (37ae4f4,
# existsAsFile -> exists):
#
#   W1  warm-start zero-misjudged-cleanup — start the REAL kernel on the REAL
#       workspace settings (VitApp/Workspace/Settings/Settings.xml warm table),
#       assert plugin_list_available count stays >= 95% of the settings-file
#       entry count and, if a "PluginListHygiene: removed" summary line
#       appears, every removed path is genuinely absent on disk.
#   W2  load spot without rescan — resolve a known single-file VST3 through
#       the warm knownPluginList (rack_add_node on a freshly added audio
#       track; no scan_plugins issued before it).
#   W3  restart persistence — stop the kernel, start it again, assert the
#       list count is unchanged (removed=0), no scan_plugins command was
#       executed, and no cleanup line appeared on second start.
#
# Differences from the mac script (documented, not silently dropped):
#   - legs B (cancel/rescan) and C (kill -9 pedal dichotomy) are mac-incident
#     legs and are NOT replayed on PC; their semantics live in
#     VitPluginListHygieneTests which runs on PC via ctest (both configs).
#   - process control uses Windows taskkill (Git Bash kill cannot gracefully
#     signal native processes); W-legs perform no scan, so the graceful
#     shutdown flush concern from the mac script does not apply.
#   - port liveness is checked by connecting (python socket), not lsof.
#
# §8 discipline: deterministic kernel assertions, no LLM; one run per
# invocation; exit 0 requires every requested leg green.
#
# Usage:
#   ./scripts/kernel_pluginlist_hygiene_smoke_pc.sh
#   ./scripts/kernel_pluginlist_hygiene_smoke_pc.sh --leg W1,W3
#   ./scripts/kernel_pluginlist_hygiene_smoke_pc.sh --kernel-bin PATH
#   ./scripts/kernel_pluginlist_hygiene_smoke_pc.sh --artifacts-root DIR
#                                                   (default <repo>/coord/runs/PORT-WIN-HYGIENE-SMOKE-1)

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
VST3_DIR="C:/Program Files/Common Files/VST3"
ARTIFACTS_ROOT_DEFAULT=""
LEGS="W1,W2,W3"
KERNEL_PORT_REQ=5555
STARTUP_TIMEOUT_SECONDS=180
LOAD_SPOT_NAME="Jeesonic EQ Pro"
SETTINGS_COUNT_FLOOR_RATIO=95   # percent of settings-file entries the warm list must retain

usage() {
  cat <<'EOF'
kernel_pluginlist_hygiene_smoke_pc.sh — Windows HYGIENE replay legs (W1-W3).
Options:
  --repo-root PATH       Repository root (default: parent of this script's dir)
  --kernel-bin PATH      Kernel binary (default: repo VitApp/cmake-build-pcverify1/VitApp_artefacts/Release/VitApp.exe)
  --vst3-dir PATH        VST3 directory (default C:\Program Files\Common Files\VST3)
  --artifacts-root DIR   Artifacts root (default <repo>/coord/runs/PORT-WIN-HYGIENE-SMOKE-1)
  --leg LIST             Comma subset of W1,W2,W3 (default all)
  --load-spot NAME       Plugin name for the W2 load spot (default "Jeesonic EQ Pro")
  -h, --help             Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root) REPO_ROOT_DEFAULT="$2"; shift 2 ;;
    --kernel-bin) KERNEL_BIN_DEFAULT="$2"; shift 2 ;;
    --vst3-dir) VST3_DIR="$2"; shift 2 ;;
    --artifacts-root) ARTIFACTS_ROOT_DEFAULT="$2"; shift 2 ;;
    --leg) LEGS="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"; shift 2 ;;
    --load-spot) LOAD_SPOT_NAME="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

PLATFORM="$(uname -s)"
[[ "$PLATFORM" == MINGW* || "$PLATFORM" == MSYS* ]] || { echo "ERROR[env]: pc-only script (Git Bash expected), uname=$PLATFORM" >&2; exit 2; }
for tool in python sha256sum taskkill; do
  command -v "$tool" >/dev/null 2>&1 || { echo "ERROR[env]: required tool not found: $tool" >&2; exit 2; }
done
python -c "import zmq" 2>/dev/null || { echo "ERROR[env]: python pyzmq not available" >&2; exit 2; }

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_PATH")" && pwd)"
REPO_ROOT="${REPO_ROOT_DEFAULT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
KERNEL_BIN="${KERNEL_BIN_DEFAULT:-$REPO_ROOT/VitApp/cmake-build-pcverify1/VitApp_artefacts/Release/VitApp.exe}"
KERNEL_BIN="$(cygpath -u "$KERNEL_BIN")"
ARTIFACTS_ROOT="${ARTIFACTS_ROOT_DEFAULT:-$REPO_ROOT/coord/runs/PORT-WIN-HYGIENE-SMOKE-1}"

[[ -f "$KERNEL_BIN" ]] || { echo "ERROR[env]: kernel binary not found: $KERNEL_BIN" >&2; exit 2; }
[[ -d "$VST3_DIR" ]] || { echo "ERROR[env]: VST3 dir not found: $VST3_DIR" >&2; exit 2; }

# Env sentinel: the VST3 tree must be REAL (Windows single-file .vst3 form).
VST3_SENTINEL="$(find "$VST3_DIR" -maxdepth 1 -name "*.vst3" -type f -size +10k 2>/dev/null | head -1)"
[[ -n "$VST3_SENTINEL" ]] || { echo "ERROR[env]: no real single-file .vst3 (>10k) under $VST3_DIR — refusing a stub tree" >&2; exit 2; }

RUN_ID="hygiene_pc_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/$RUN_ID"
mkdir -p "$RUN_DIR"/{replies,logs,settings_states}
PREREQ_FILE="$RUN_DIR/prereq.txt"

REAL_SETTINGS="$REPO_ROOT/VitApp/Workspace/Settings/Settings.xml"
REAL_PEDAL="$REPO_ROOT/VitApp/Workspace/Settings/plugin_scan_dead_mans_pedal.txt"
REAL_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REAL_SETTINGS" ]] || { echo "ERROR[env]: real Settings.xml not found" >&2; exit 2; }
[[ -f "$REAL_DEFAULT_PROJECT" ]] || { echo "ERROR[env]: default_project.xml not found" >&2; exit 2; }

KERNEL_PID=""
KERNEL_LOG=""
OVERALL_PASS=1

log()  { echo "[$(date '+%H:%M:%S')] $*" >&2; }
step() { echo "" >&2; echo "== $*" >&2; }
ok()   { echo "ok: $*" | tee -a "$PREREQ_FILE" >&2; }
bad()  { echo "RED: $*" | tee -a "$PREREQ_FILE" >&2; OVERALL_PASS=0; }
info() { echo "info: $*" | tee -a "$PREREQ_FILE" >&2; }
prereq() { echo "$1" | tee -a "$PREREQ_FILE" >&2; }

sha() { sha256sum "$1" 2>/dev/null | cut -d' ' -f1; }
ascii_only() { LC_ALL=C tr -d '\200-\377'; }

port_listener_alive() { python - "$1" <<'PY'
import socket, sys
s = socket.socket()
s.settimeout(0.5)
try:
    s.connect(("127.0.0.1", int(sys.argv[1]))); print("yes")
except Exception:
    print("no")
finally:
    s.close()
PY
}

vitapp_pids() { tasklist //FI "IMAGENAME eq VitApp.exe" //FO CSV //NH 2>/dev/null | grep -i vitapp | cut -d'"' -f2 || true; }

zmq_send() {
  # zmq_send <json-file> <out-file> [timeout-ms]
  local body="$1" out="$2" timeout_ms="${3:-30000}"
  python - "$body" "$out" "$KERNEL_PORT_REQ" "$timeout_ms" <<'PY'
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
  python - "$1" "$2" <<'PY'
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8", errors="replace") as f:
        d = json.load(f)
    print(eval(sys.argv[2], {"d": d}))
except Exception:
    print("")
PY
}

settings_snapshot() {
  local tag="$1"
  cp "$REAL_SETTINGS" "$RUN_DIR/settings_states/settings_${tag}.xml"
  printf '%s\n' "$(sha "$REAL_SETTINGS")" > "$RUN_DIR/settings_states/settings_${tag}.sha256"
}

settings_type_count() {
  python - "$REAL_SETTINGS" <<'PY'
import sys, xml.etree.ElementTree as ET
root = ET.parse(sys.argv[1]).getroot()
n = 0
for v in root.iter("VALUE"):
    if v.get("name") == "knownPluginList64":
        n += sum(1 for _ in v.iter("PLUGIN"))
print(n)
PY
}

list_count() {
  # list_count <reply-file>: plugins array length of a plugin_list_available reply
  python - "$1" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    print(len(d.get("plugins") or []))
except Exception:
    print(-1)
PY
}

wait_ports_free() {
  local deadline=$((SECONDS + 30))
  while (( SECONDS < deadline )); do
    [[ "$(port_listener_alive "$KERNEL_PORT_REQ")" == "no" ]] && return 0
    sleep 1
  done
  prereq "port_still_busy=$KERNEL_PORT_REQ"
  return 1
}

start_kernel() {
  # start_kernel <tag>: cwd=repo root; VIT_PROJECT_XML redirected to the copy
  # (AGENTS §10); executable-dir climb lands on VitApp/ -> real Workspace.
  # Returns 0 with KERNEL_LOG set to the kernel's REAL log file: the JUCE
  # kernel writes VitHeadlessServer<ts>.log under Workspace/Logs (stdout is
  # empty for a windowed-subsystem build); every log assertion below reads
  # that file, never the empty stdout redirect.
  local tag="$1"
  wait_ports_free || { bad "ports busy before kernel start (leg $tag)"; return 1; }
  KERNEL_LOG="$RUN_DIR/logs/kernel_${tag}.log"
  local newest_before; newest_before="$(ls -t "$REPO_ROOT/VitApp/Workspace/Logs/"VitHeadlessServer*.log 2>/dev/null | head -1 || true)"
  (
    cd "$REPO_ROOT"
    exec env VIT_PROJECT_XML="$RUN_DIR/project/default_project.xml" \
             VIT_DAW_DEV_ROOT="$REPO_ROOT" \
             "$KERNEL_BIN"
  ) > "$KERNEL_LOG" 2>&1 &
  KERNEL_PID=$!
  local deadline=$((SECONDS + STARTUP_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    kill -0 "$KERNEL_PID" 2>/dev/null || { bad "kernel exited during startup (leg $tag)"; tail -5 "$KERNEL_LOG" | ascii_only >&2; KERNEL_PID=""; return 1; }
    [[ "$(port_listener_alive "$KERNEL_PORT_REQ")" == "yes" ]] && break
    sleep 1
  done
  if [[ "$(port_listener_alive "$KERNEL_PORT_REQ")" != "yes" ]]; then
    local winpid; winpid="$(kernel_winpid "$KERNEL_PID")"
    [[ -n "$winpid" ]] && taskkill //F //PID "$winpid" >/dev/null 2>&1 || taskkill //F //IM VitApp.exe >/dev/null 2>&1 || true
    wait "$KERNEL_PID" 2>/dev/null
    bad "kernel REQ port not up in ${STARTUP_TIMEOUT_SECONDS}s (leg $tag; child reaped)"
    KERNEL_PID=""
    return 1
  fi
  KERNEL_LOGFILE=""
  local wait_log=$((SECONDS + 20))
  while (( SECONDS < wait_log )); do
    KERNEL_LOGFILE="$(ls -t "$REPO_ROOT/VitApp/Workspace/Logs/"VitHeadlessServer*.log 2>/dev/null | head -1 || true)"
    [[ -n "$KERNEL_LOGFILE" && "$KERNEL_LOGFILE" != "$newest_before" ]] && break
    sleep 1
  done
  if [[ -n "$KERNEL_LOGFILE" && "$KERNEL_LOGFILE" != "$newest_before" ]]; then
    cp "$KERNEL_LOGFILE" "$KERNEL_LOG" 2>/dev/null || true
    prereq "kernel_${tag}_pid=$KERNEL_PID winlog=$KERNEL_LOGFILE (copied to $KERNEL_LOG; will re-copy at leg end)"
  else
    prereq "kernel_${tag}_pid=$KERNEL_PID winlog=NOT_FOUND (log assertions will read stdout only — UNRELIABLE)"
    bad "kernel ${tag}: real VitHeadlessServer log not found after startup — log-based assertions unreliable"
  fi
  return 0
}

snapshot_kernel_log() {
  # snapshot_kernel_log <tag>: refresh the copy from the live kernel log so
  # assertions see everything written so far this session.
  [[ -n "$KERNEL_LOGFILE" && -f "$KERNEL_LOGFILE" ]] && cp "$KERNEL_LOGFILE" "$KERNEL_LOG" 2>/dev/null || true
}

kernel_winpid() {
  # kernel_winpid <msys-pid>: the native Windows PID of the kernel child.
  # Git Bash $! is an MSYS pid; taskkill needs the WinPID (ps -W column 4).
  [[ -n "$1" ]] || return 0
  ps -W 2>/dev/null | awk -v m="$1" '$1 == m {print $4; exit}'
}

stop_kernel() {
  # stop_kernel <tag>: native Windows process — taskkill by WinPID. Git Bash
  # kill/MSYS pid and taskkill/WinPID live in different pid spaces, so $!
  # alone cannot address the child. W legs issue no scan, so the mac
  # graceful-flush concern does not apply; the warm table is on disk.
  local tag="$1" start elapsed winpid
  start=$SECONDS
  winpid="$(kernel_winpid "$KERNEL_PID")"
  if [[ -n "$winpid" ]]; then
    taskkill //F //PID "$winpid" >/dev/null 2>&1 || true
  else
    # fall back to image name (single-owner guard ran at preflight)
    taskkill //F //IM VitApp.exe >/dev/null 2>&1 || true
  fi
  wait "$KERNEL_PID" 2>/dev/null
  KERNEL_PID=""
  elapsed=$((SECONDS - start))
  prereq "kernel_${tag}_stop=taskkill(winpid=${winpid:-im}) elapsed=${elapsed}s"
  wait_ports_free || prereq "ports_busy_after_${tag}"
  return 0
}

emergency_teardown() {
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    local winpid; winpid="$(kernel_winpid "$KERNEL_PID")"
    [[ -n "$winpid" ]] && taskkill //F //PID "$winpid" >/dev/null 2>&1 || taskkill //F //IM VitApp.exe >/dev/null 2>&1 || true
    echo "EMERGENCY: kernel $KERNEL_PID (winpid=${winpid:-im}) killed by EXIT trap" | tee -a "$PREREQ_FILE" >&2
  fi
  KERNEL_PID=""
}
trap emergency_teardown EXIT INT TERM

has_leg() { [[ ",$LEGS," == *",$1,"* ]]; }

# ================================================================ preflight
step "Preflight"
RUNNING="$(vitapp_pids | tr '\n' ' ')"
if [[ -n "${RUNNING// /}" ]]; then
  echo "ERROR[env]: a VitApp.exe is already running (pids: $RUNNING) — single-owner rule, aborting" >&2
  exit 2
fi
prereq "no_running_kernel_at_start=confirmed"
if [[ -e "$REAL_PEDAL" ]]; then
  PEDAL_NONEMPTY="$(python - "$REAL_PEDAL" <<'PY'
import sys
try:
    text = open(sys.argv[1], encoding="utf-16").read()
except Exception:
    text = open(sys.argv[1], encoding="utf-8", errors="replace").read()
print("yes" if text.strip() else "no")
PY
)"
  if [[ "$PEDAL_NONEMPTY" == "yes" ]]; then
    echo "ERROR[env]: dead-man's pedal non-empty at start — clean it deliberately first" >&2
    exit 2
  fi
  rm -f "$REAL_PEDAL"
fi
prereq "no_pedal_at_start=confirmed"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%F %T')"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$(sha "$KERNEL_BIN")"
  echo "kernel_mtime=$(date -r "$KERNEL_BIN" '+%F %T' 2>/dev/null || echo unknown)"
  echo "vst3_dir=$VST3_DIR"
  echo "legs=$LEGS"
  echo "real_settings=$REAL_SETTINGS"
  echo "real_settings_sha256=$(sha "$REAL_SETTINGS")"
} > "$RUN_DIR/run_meta.txt"
cat "$RUN_DIR/run_meta.txt" >&2

mkdir -p "$RUN_DIR/project"
cp "$REAL_DEFAULT_PROJECT" "$RUN_DIR/project/default_project.xml"
prereq "project_copy_sha256=$(sha "$RUN_DIR/project/default_project.xml") (source project never written; VIT_PROJECT_XML redirect)"
SETTINGS_FILE_COUNT=$(settings_type_count)
FLOOR_COUNT=$(( SETTINGS_FILE_COUNT * SETTINGS_COUNT_FLOOR_RATIO / 100 ))
info "settings knownPluginList64 entries=$SETTINGS_FILE_COUNT; warm-list retention floor=${SETTINGS_COUNT_FLOOR_RATIO}%=$FLOOR_COUNT"
settings_snapshot "preflight"

# ================================================================ LEG W1
W1_COUNT=""
if has_leg W1; then
  step "LEG W1: warm start — zero misjudged cleanup on the bundle-aware predicate"
  start_kernel "W1" || true
  if [[ -n "$KERNEL_PID" ]]; then
    sleep 8
    snapshot_kernel_log "W1"
    printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/W1_list_req.json"
    zmq_send "$RUN_DIR/replies/W1_list_req.json" "$RUN_DIR/replies/W1_list.json" 60000 || true
    W1_COUNT="$(list_count "$RUN_DIR/replies/W1_list.json")"
    prereq "W1_list_count=$W1_COUNT (settings file: $SETTINGS_FILE_COUNT, floor: $FLOOR_COUNT)"
    if (( ${W1_COUNT:-0} >= FLOOR_COUNT )); then
      ok "LEG W1: warm list retained ${W1_COUNT}/${SETTINGS_FILE_COUNT} entries (no mass cleanup; mac defect form would collapse toward 0)"
    else
      bad "LEG W1: warm list collapsed to ${W1_COUNT}/${SETTINGS_FILE_COUNT} (below floor $FLOOR_COUNT) — misjudged cleanup suspected"
    fi
    HYGIENE_LINE="$(grep -m1 "PluginListHygiene: removed" "$KERNEL_LOG" | ascii_only || true)"
    if [[ -n "$HYGIENE_LINE" ]]; then
      echo "$HYGIENE_LINE" > "$RUN_DIR/W1_summary_line.txt"
      info "LEG W1: cleanup line present (legitimate stale removal possible) — verifying removed paths against disk"
      REMOVED_TYPES="$(printf '%s' "$HYGIENE_LINE" | sed -n 's/.*removed \([0-9]*\) stale type entries.*/\1/p')"
      info "LEG W1: removed stale type entries=${REMOVED_TYPES:-0}"
      # Every path the summary names must be genuinely absent on disk. The
      # summary line names individual files (mac format); if the line is
      # count-only, fall back to settings diff: removed entries must not
      # exist on disk either way.
      python - "$REAL_SETTINGS" "$RUN_DIR/settings_states/settings_preflight.xml" > "$RUN_DIR/W1_removed_paths.txt" <<'PY'
import sys, xml.etree.ElementTree as ET
def files(p):
    r = ET.parse(p).getroot()
    return set(e.get("file","") for v in r.iter("VALUE")
               if v.get("name")=="knownPluginList64" for e in v.iter("PLUGIN"))
removed = files(sys.argv[2]) - files(sys.argv[1])
for f in sorted(removed): print(f)
PY
      BAD_REMOVALS="$(python - "$RUN_DIR/W1_removed_paths.txt" <<'PY'
import os, sys
bad = [l.strip() for l in open(sys.argv[1], encoding="utf-8", errors="replace")
       if l.strip() and (os.path.exists(l.strip()) or os.path.isdir(l.strip()))]
print("\n".join(bad))
PY
)"
      REMOVED_N="$(grep -c . "$RUN_DIR/W1_removed_paths.txt" || true)"
      if [[ -n "$BAD_REMOVALS" ]]; then
        bad "LEG W1: ${REMOVED_N} entries removed but some still exist on disk (wrongly stale): $BAD_REMOVALS"
      else
        ok "LEG W1: cleanup verified correct — ${REMOVED_N} removed entries all genuinely absent on disk"
      fi
    else
      ok "LEG W1: no PluginListHygiene cleanup line (zero removals — table fully warm-valid)"
    fi
    stop_kernel "W1"
  fi
  settings_snapshot "W1_after"
fi

# ================================================================ LEG W2
if has_leg W2; then
  step "LEG W2: load spot without rescan (knownPluginList resolution)"
  LEGW2_OK=1
  start_kernel "W2" || LEGW2_OK=0
  if [[ "$LEGW2_OK" -eq 1 ]]; then
    printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/W2_list_req.json"
    zmq_send "$RUN_DIR/replies/W2_list_req.json" "$RUN_DIR/replies/W2_list.json" 60000 || true
    SPOT="$(python - "$RUN_DIR/replies/W2_list.json" "$LOAD_SPOT_NAME" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
name = sys.argv[2]
for p in (d.get("plugins") or []):
    if p.get("name") == name:
        ident = p.get("identifier") or p.get("plugin_identifier") or ""
        if ident:
            print(ident); break
        uid = p.get("uid") or p.get("unique_id") or ""
        if uid:
            print("VST3-%s-%s" % (name, uid)); break
PY
)"
    if [[ -n "$SPOT" ]]; then
      prereq "W2_spot_identifier=$SPOT"
    else
      bad "LEG W2: no identifier resolvable for '$LOAD_SPOT_NAME' from plugin_list_available"
      python - "$RUN_DIR/replies/W2_list.json" <<'PY' >&2
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
rows = d.get("plugins") or []
print("sample keys:", list(rows[0].keys()) if rows else "none", file=sys.stderr)
PY
      LEGW2_OK=0
    fi
    if [[ "$LEGW2_OK" -eq 1 ]]; then
      python - > "$RUN_DIR/replies/W2_add_track_req.json" <<'PY'
import json
print(json.dumps({"cmd": "add_track", "name": "HygieneLoadSpot", "type": "audio"}))
PY
      zmq_send "$RUN_DIR/replies/W2_add_track_req.json" "$RUN_DIR/replies/W2_add_track.json" 30000 || true
      TRACK_ID="$(jfield "$RUN_DIR/replies/W2_add_track.json" 'str(d.get("track_id", d.get("id", d.get("result",{}).get("track_id","")) if isinstance(d.get("result"),dict) else ""))')"
      ADD_STATUS="$(jfield "$RUN_DIR/replies/W2_add_track.json" 'str(d.get("status",""))')"
      prereq "W2_add_track status=$ADD_STATUS track_id=$TRACK_ID"
      [[ "$ADD_STATUS" == "ok" && -n "$TRACK_ID" ]] || { bad "LEG W2: add_track failed (status=$ADD_STATUS)"; LEGW2_OK=0; }
    fi
    if [[ "$LEGW2_OK" -eq 1 ]]; then
      python - "$TRACK_ID" "$SPOT" > "$RUN_DIR/replies/W2_rack_add_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node",
                  "track_id": sys.argv[1], "plugin_identifier": sys.argv[2],
                  "x": 0, "y": 0}))
PY
      zmq_send "$RUN_DIR/replies/W2_rack_add_req.json" "$RUN_DIR/replies/W2_rack_add.json" 180000 || LEGW2_OK=0
      ADD_STATUS="$(jfield "$RUN_DIR/replies/W2_rack_add.json" 'str(d.get("status",""))')"
      ADD_PLUGIN_ID="$(jfield "$RUN_DIR/replies/W2_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
      ADD_ERR="$(jfield "$RUN_DIR/replies/W2_rack_add.json" 'str(d.get("message", d.get("error","")))')"
      prereq "W2_rack_add status=$ADD_STATUS plugin_id=$ADD_PLUGIN_ID error=$ADD_ERR"
      snapshot_kernel_log "W2"
      SCAN_CMDS="$(grep -c "scan_plugins" "$KERNEL_LOG" || true)"
      if [[ "$ADD_STATUS" == "ok" && -n "$ADD_PLUGIN_ID" && "${SCAN_CMDS:-0}" -eq 0 ]]; then
        ok "LEG W2: load spot landed without any rescan (status=ok plugin_id=$ADD_PLUGIN_ID; scan_plugins commands in log: $SCAN_CMDS)"
      else
        bad "LEG W2: rack_add_node status=$ADD_STATUS plugin_id=$ADD_PLUGIN_ID scan_cmds=$SCAN_CMDS error=$ADD_ERR"
        LEGW2_OK=0
      fi
    fi
    stop_kernel "W2"
  fi
fi

# ================================================================ LEG W3
if has_leg W3; then
  step "LEG W3: restart persistence — removed=0, no scan command, table identical"
  LEGW3_OK=1
  BASE_COUNT="${W1_COUNT:-}"
  if [[ -z "$BASE_COUNT" ]]; then
    start_kernel "W3a" || LEGW3_OK=0
    if [[ "$LEGW3_OK" -eq 1 ]]; then
      printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/W3a_list_req.json"
      zmq_send "$RUN_DIR/replies/W3a_list_req.json" "$RUN_DIR/replies/W3a_list.json" 60000 || true
      BASE_COUNT="$(list_count "$RUN_DIR/replies/W3a_list.json")"
      stop_kernel "W3a"
    fi
  fi
  if [[ "$LEGW3_OK" -eq 1 ]]; then
    prereq "W3_base_count=$BASE_COUNT"
    start_kernel "W3b" || LEGW3_OK=0
    if [[ "$LEGW3_OK" -eq 1 ]]; then
      sleep 8
      snapshot_kernel_log "W3"
      printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/W3_list_req.json"
      zmq_send "$RUN_DIR/replies/W3_list_req.json" "$RUN_DIR/replies/W3_list.json" 60000 || true
      W3_COUNT="$(list_count "$RUN_DIR/replies/W3_list.json")"
      prereq "W3_list_count=$W3_COUNT (base: $BASE_COUNT)"
      [[ "${W3_COUNT:-0}" == "${BASE_COUNT:-x}" ]] \
        && ok "LEG W3: restart kept the list identical ($W3_COUNT == $BASE_COUNT; removed=0)" \
        || { bad "LEG W3: list changed across restart ($W3_COUNT != $BASE_COUNT)"; LEGW3_OK=0; }
      REMOVED_LINES="$(grep -c "PluginListHygiene: removed" "$KERNEL_LOG" || true)"
      [[ "${REMOVED_LINES:-0}" -eq 0 ]] \
        && ok "LEG W3: zero cleanup lines on restart (removed=0)" \
        || { bad "LEG W3: cleanup line on warm restart ($REMOVED_LINES line(s))"; LEGW3_OK=0; }
      SCAN_CMDS="$(grep -c -E "Command executed: (scan_plugins|plugin_scan)" "$KERNEL_LOG" || true)"
      [[ "${SCAN_CMDS:-0}" -eq 0 ]] \
        && ok "LEG W3: zero scan commands executed on restart (table trusted from disk)" \
        || { bad "LEG W3: scan command executed on restart ($SCAN_CMDS) — table not trusted"; LEGW3_OK=0; }
      stop_kernel "W3b"
    fi
  fi
  settings_snapshot "W3_after"
fi

# ================================================================ summary
step "Summary"
python - "$RUN_DIR" "$RUN_ID" "$LEGS" <<'PY'
import json, os, sys
run_dir, run_id, legs = sys.argv[1], sys.argv[2], sys.argv[3]
ok_lines, red_lines = [], []
if os.path.isfile(os.path.join(run_dir, "prereq.txt")):
    for line in open(os.path.join(run_dir, "prereq.txt"), encoding="utf-8", errors="replace"):
        line = line.rstrip("\n")
        if line.startswith("ok: "): ok_lines.append(line[4:])
        elif line.startswith("RED: "): red_lines.append(line[5:])
report = {
    "schema_version": "vit_kernel_pluginlist_hygiene_smoke.pc.v1",
    "card": "PORT-WIN-HYGIENE-SMOKE-1",
    "run_id": run_id,
    "run_root": run_dir,
    "legs_requested": legs,
    "ok_count": len(ok_lines),
    "red_count": len(red_lines),
    "ok": ok_lines,
    "red": red_lines,
    "verdict": "all_green" if not red_lines else "assertions_red",
}
out = os.path.join(run_dir, "hygiene_report.json")
json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("report: %s" % out, file=sys.stderr)
print("verdict: %s (ok=%d red=%d)" % (report["verdict"], len(ok_lines), len(red_lines)), file=sys.stderr)
PY
prereq "finished=$(date '+%F %T')"
log "workdir: $RUN_DIR"
if [[ "$OVERALL_PASS" -eq 1 ]]; then
  echo "HYGIENE_PC_VERDICT all_green" >&2
  exit 0
else
  echo "HYGIENE_PC_VERDICT assertions_red" >&2
  exit 1
fi
