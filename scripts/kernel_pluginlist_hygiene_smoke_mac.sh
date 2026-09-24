#!/usr/bin/env bash
# kernel_pluginlist_hygiene_smoke_mac.sh — FIX-KERNEL-PLUGINLIST-HYGIENE-1 mac
# real-stack fine legs (PORT-PCBATCH-MAC-LEGS-1 leg 2).
#
# Drives the REAL kernel binary (the one the frontend uses at
# VitApp/build/VitApp_artefacts/Debug/VitApp) with the REAL workspace
# settings (VitApp/Workspace/Settings/Settings.xml — the warm plugin
# database) over the same kernel command wire the Godot settings page uses
# (ZMQ REQ 5555, single-frame JSON, cf. vit_ipc_client.gd ->
# kernel_command_client.gd -> settings_client.gd plugin_scan_cancel). The
# project XML is redirected with VIT_PROJECT_XML to a byte copy so the real
# default_project.xml is never written (AGENTS §10); the settings dir is
# intentionally NOT isolated: the four legs under test are exactly the
# live-settings behaviours.
#
# Four legs (HYGIENE card acceptance ①-④):
#   A  startup lazy cleanup   — inject 2 stale VST3 type entries + 1 stale
#                               blacklist entry into knownPluginList64, start
#                               kernel, assert the one-line
#                               "PluginListHygiene: removed ..." summary with
#                               exact counts, stop, RESTORE the baseline
#                               Settings.xml (sha256 verified).
#   B  cancel -> rescan       — full scan over the system VST3 dir, cancel
#                               mid-scan, assert: state=cancelled, dead-man's
#                               pedal cleared, no new blacklist entries,
#                               partial results retained; then rescan to
#                               completion and assert the full plugin count
#                               came back (the 09-23 incident's silent-skip
#                               "completed plugins=1" must not happen).
#   C  kill -9 dichotomy      — kill -9 the kernel mid-scan, assert the pedal
#                               file survives, restart + start a scan, assert
#                               the pedal is consumed and the file being
#                               probed at kill time lands in the blacklist
#                               (crash defence not weakened). RESTORE the
#                               pre-leg settings afterwards.
#   D  full scan + load spot  — full scan to completion + rack_add_node with
#                               a plugin_identifier resolved through the
#                               knownPluginList (kernel :1153 resolution path).
#
# §8 discipline: deterministic kernel-behaviour assertions (no LLM); one run
# per invocation; failures are classified env vs functional in the summary.
# Exit 0 requires every requested leg green.
#
# Usage:
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh               # all four legs
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --leg A
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --leg A,B
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --kernel-bin PATH
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --vst3-dir PATH
#   ./scripts/kernel_pluginlist_hygiene_smoke_mac.sh --artifacts-root DIR
#                                                     (default
#                                                      ~/Documents/vit-pcbatch-mac-legs-artifacts)
#
# All evidence lands under <artifacts-root>/leg2_hygiene/<run-id>/.

set -uo pipefail

REPO_ROOT_DEFAULT=""
KERNEL_BIN_DEFAULT=""
VST3_DIR="/Library/Audio/Plug-Ins/VST3"
ARTIFACTS_ROOT_DEFAULT="$HOME/Documents/vit-pcbatch-mac-legs-artifacts"
LEGS="A,B,C,D"
KERNEL_PORT_REQ=5555
ZMQ_PUB_PORT=5556
ZMQ_LOG_PORT=5557
STARTUP_TIMEOUT_SECONDS=240
STOP_GRACE_SECONDS=10
SCAN_TIMEOUT_SECONDS=1500
CANCEL_MIN_COMPLETED=1
LOAD_SPOT_IDENTIFIER="VST3-C1 comp Mono-10456661-65e94c5e"
ZONE_ID="Z3"
PROJECT_SOURCE=""
STEMS_DIR=""
SKIP_FULL_RESCAN=0

usage() {
  cat <<'EOF'
kernel_pluginlist_hygiene_smoke_mac.sh — HYGIENE mac real-stack fine legs.

Options:
  --repo-root PATH       Repository root (default: parent of this script's dir)
  --kernel-bin PATH      Kernel binary (default: repo VitApp/build/VitApp_artefacts/Debug/VitApp)
  --vst3-dir PATH        VST3 directory to scan (default /Library/Audio/Plug-Ins/VST3)
  --artifacts-root DIR   Artifacts root (default ~/Documents/vit-pcbatch-mac-legs-artifacts)
  --leg LIST             Comma subset of A,B,C,D (default all)
  --project-source PATH  Project XML the kernel loads (VIT_PROJECT_XML copy;
                         default: the repo real default_project.xml). Leg D's
                         load spot needs a track with a rack zone — pass the
                         journey demo project (912.vit) for that.
  --stems-dir DIR        Real <stem>.wav sources for the 912.vit fixture paths
  --scan-timeout SECONDS Full-scan completion timeout (default 1500)
  --skip-full-rescan     Leg D: reuse the warm list, only run the load spot
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
    --project-source) PROJECT_SOURCE="$2"; shift 2 ;;
    --stems-dir) STEMS_DIR="$2"; shift 2 ;;
    --scan-timeout) SCAN_TIMEOUT_SECONDS="$2"; shift 2 ;;
    --skip-full-rescan) SKIP_FULL_RESCAN=1; shift ;;
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
ARTIFACTS_ROOT="$ARTIFACTS_ROOT_DEFAULT"

[[ -x "$KERNEL_BIN" ]] || { echo "ERROR[env]: kernel binary not executable: $KERNEL_BIN" >&2; exit 2; }
[[ -d "$VST3_DIR" ]] || { echo "ERROR[env]: VST3 dir not found: $VST3_DIR" >&2; exit 2; }

# Env sentinel: the VST3 tree must be REAL. On mac every .vst3 is a bundle
# DIRECTORY whose actual binary is Contents/MacOS/<name>; a stub/sandboxed
# filesystem can present names that stat but cannot be loaded. Abort as env
# failure instead of poisoning the legs. (Observed 2026-09-24.)
VST3_SENTINEL_BUNDLE="$(find "$VST3_DIR" -maxdepth 1 -name "*.vst3" -print 2>/dev/null | head -1)"
VST3_SENTINEL_BIN="$(find "$VST3_SENTINEL_BUNDLE/Contents/MacOS" -type f -print 2>/dev/null | head -1)"
VST3_SENTINEL_SIZE="$(stat -f %z "$VST3_SENTINEL_BIN" 2>/dev/null || echo 0)"
if [[ -z "$VST3_SENTINEL_BIN" || "${VST3_SENTINEL_SIZE:-0}" -lt 10000 ]]; then
  echo "ERROR[env]: VST3 tree not real (bundle=$VST3_SENTINEL_BUNDLE bin=$VST3_SENTINEL_BIN size=$VST3_SENTINEL_SIZE) — refusing to run against a stub/sandboxed filesystem" >&2
  exit 2
fi

RUN_ID="hygiene_mac_$(date '+%Y%m%d-%H%M%S')"
RUN_DIR="$ARTIFACTS_ROOT/leg2_hygiene/$RUN_ID"
mkdir -p "$RUN_DIR"/{replies,logs,settings_states}
PREREQ_FILE="$RUN_DIR/prereq.txt"

REAL_SETTINGS="$REPO_ROOT/VitApp/Workspace/Settings/Settings.xml"
REAL_PEDAL="$REPO_ROOT/VitApp/Workspace/Settings/plugin_scan_dead_mans_pedal.txt"
REAL_DEFAULT_PROJECT="$REPO_ROOT/VitApp/Workspace/default_project.xml"
[[ -f "$REAL_SETTINGS" ]] || { echo "ERROR[env]: real Settings.xml not found: $REAL_SETTINGS" >&2; exit 2; }
[[ -f "$REAL_DEFAULT_PROJECT" ]] || { echo "ERROR[env]: real default_project.xml not found" >&2; exit 2; }

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

# Kernel-log/pedal-derived strings can carry non-UTF-8 bytes (JUCE pedal
# content starts with 0xff; Waves child output is mixed). Keep printable
# ASCII so artifact readers never see decode failures.
ascii_only() { LC_ALL=C tr -d '\200-\377'; }

port_listener_pid() { lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true; }

zmq_send() {
  # zmq_send <json-file> <out-file> [timeout-ms] — one REQ socket per command
  # (same pattern as agent/internal/kernel/client.go SendRaw).
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

settings_snapshot() {
  # settings_snapshot <tag> — copy + sha of the live Settings.xml
  local tag="$1"
  cp "$REAL_SETTINGS" "$RUN_DIR/settings_states/settings_${tag}.xml"
  printf '%s\n' "$(sha "$REAL_SETTINGS")" > "$RUN_DIR/settings_states/settings_${tag}.sha256"
}

settings_blacklist_entries() {
  # prints the blacklist paths (one per line) from the live Settings.xml
  python3 - "$REAL_SETTINGS" <<'PY'
import sys, xml.etree.ElementTree as ET
root = ET.parse(sys.argv[1]).getroot()
for v in root.iter("VALUE"):
    if v.get("name") == "knownPluginList64":
        for b in v.iter("BLACKLISTED"):
            print(b.get("id", ""))
PY
}

settings_type_count() {
  python3 - "$REAL_SETTINGS" <<'PY'
import sys, xml.etree.ElementTree as ET
root = ET.parse(sys.argv[1]).getroot()
n = 0
for v in root.iter("VALUE"):
    if v.get("name") == "knownPluginList64":
        n += sum(1 for _ in v.iter("PLUGIN"))
print(n)
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
  # start_kernel <tag>: real-root kernel (cwd=repo root; executable-dir climb
  # lands on VitApp/ -> real Workspace), project XML redirected to the copy.
  # NOTE: never touch the dead-man's pedal here — leg C depends on a crash
  # residue surviving into the next kernel start (deleting it would silently
  # disarm the crash-defence under test).
  local tag="$1"
  wait_ports_free || { bad "ports busy before kernel start (leg $tag)"; return 1; }
  KERNEL_LOG="$RUN_DIR/logs/kernel_${tag}.log"
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
    [[ -n "$(port_listener_pid "$KERNEL_PORT_REQ")" ]] && break
    sleep 1
  done
  if [[ -z "$(port_listener_pid "$KERNEL_PORT_REQ")" ]]; then
    # Give-up path MUST reap the child: an abandoned kernel keeps the JUCE
    # single-instance lock and the real-workspace settings open (2026-09-25
    # incident: a slow CoreAudio recovery outlived the port-wait, the orphan
    # persisted its wiped list and poisoned later legs).
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    wait "$KERNEL_PID" 2>/dev/null
    bad "kernel REQ port not up in ${STARTUP_TIMEOUT_SECONDS}s (leg $tag; child reaped)"
    KERNEL_PID=""
    return 1
  fi
  [[ "$(port_listener_pid "$KERNEL_PORT_REQ")" == "$KERNEL_PID" ]] || { bad "kernel REQ port owner mismatch (leg $tag)"; return 1; }
  prereq "kernel_${tag}_pid=$KERNEL_PID log=$KERNEL_LOG"
  return 0
}

stop_kernel() {
  # stop_kernel <tag> [signal] — default graceful SIGTERM
  local tag="$1" signal="${2:-TERM}" start elapsed code
  start=$SECONDS
  if [[ "$signal" == "KILL" ]]; then
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    code="SIGKILL"
  else
    kill -TERM "$KERNEL_PID" 2>/dev/null || true
    while kill -0 "$KERNEL_PID" 2>/dev/null; do
      (( SECONDS - start < STOP_GRACE_SECONDS )) || { kill -9 "$KERNEL_PID" 2>/dev/null || true; break; }
      sleep 1
    done
    code="SIGTERM"
  fi
  wait "$KERNEL_PID" 2>/dev/null
  elapsed=$((SECONDS - start))
  prereq "kernel_${tag}_stop=${code} elapsed=${elapsed}s"
  KERNEL_PID=""
  wait_ports_free || prereq "ports_busy_after_${tag}"
  return 0
}

emergency_teardown() {
  # crash-safety: never leave a real-workspace kernel running after a script bug
  if [[ -n "$KERNEL_PID" ]] && kill -0 "$KERNEL_PID" 2>/dev/null; then
    kill -9 "$KERNEL_PID" 2>/dev/null || true
    echo "EMERGENCY: kernel $KERNEL_PID SIGKILLed by EXIT trap" | tee -a "$PREREQ_FILE" >&2
  fi
  KERNEL_PID=""
  # Ownership was proven by the preflight no-running-kernel guard: sweep any
  # stragglers of THIS binary (incl. orphaned plugin-scan children).
  if pgrep -f "$KERNEL_BIN" >/dev/null 2>&1; then
    pkill -9 -f "$KERNEL_BIN" 2>/dev/null || true
    echo "EMERGENCY: residual kernel processes swept (pgrep -f $KERNEL_BIN)" | tee -a "$PREREQ_FILE" >&2
  fi
}
trap emergency_teardown EXIT INT TERM

scan_cmd() { printf '{"cmd":"scan_plugins","paths":["%s"]}' "$VST3_DIR" > "$RUN_DIR/replies/scan_req.json"; }

wait_scan_terminal() {
  # wait_scan_terminal <scan_id> <out-prefix> [timeout] — poll plugin_scan_status
  # until not scanning/cancelling; prints final state; writes last reply.
  # (locals split across statements: a single `local a=1 b=$a` expands all
  # words before any assignment lands — set -u kills that form on bash 3.2)
  local scan_id="$1" prefix="$2"
  local timeout="${3:-$SCAN_TIMEOUT_SECONDS}"
  local state=""
  local deadline=$((SECONDS + timeout))
  while (( SECONDS < deadline )); do
    printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$scan_id" > "$RUN_DIR/replies/status_req.json"
    zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/${prefix}_status.json" 30000 >/dev/null || return 1
    state="$(jfield "$RUN_DIR/replies/${prefix}_status.json" 'str(d.get("status",""))')"
    case "$state" in scanning|cancelling) sleep 3 ;; *) break ;; esac
  done
  echo "$state"
}

has_leg() { [[ ",$LEGS," == *",$1,"* ]]; }

# ================================================================ preflight
step "Preflight"
# Single-owner guard (AGENTS §9): refuse to run beside any live kernel of this
# binary — a second instance would die on the JUCE single-instance check and
# a pre-existing one would fight us for the real workspace.
if pgrep -f "$KERNEL_BIN" >/dev/null 2>&1; then
  echo "ERROR[env]: a kernel of this binary is already running (pgrep -f $KERNEL_BIN) — single-owner rule, aborting" >&2
  pgrep -fl "$KERNEL_BIN" >&2
  exit 2
fi
prereq "no_running_kernel_at_start=confirmed"
# Crash-residue guard: a NON-EMPTY dead-man's pedal would blacklist its
# content at the first scan and poison the cancel/rescan assertions; leg C
# creates its own residue deliberately. An empty leftover (a fully completed
# scan clears entries but leaves the file) is harmless.
if [[ -e "$REAL_PEDAL" ]] && [[ -n "$(tr -d '[:space:]' < "$REAL_PEDAL")" ]]; then
  echo "ERROR[env]: dead-man's pedal non-empty at start ($REAL_PEDAL) — clean it or run leg C semantics deliberately" >&2
  exit 2
fi
rm -f "$REAL_PEDAL"
prereq "no_pedal_at_start=confirmed"
{
  echo "run_id=$RUN_ID"
  echo "started_at=$(date '+%F %T')"
  echo "repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "kernel_bin=$KERNEL_BIN"
  echo "kernel_sha256=$(sha "$KERNEL_BIN")"
  echo "kernel_mtime=$(stat -f %Sm "$KERNEL_BIN")"
  echo "vst3_dir=$VST3_DIR"
  echo "legs=$LEGS"
  echo "real_settings=$REAL_SETTINGS"
  echo "real_settings_sha256=$(sha "$REAL_SETTINGS")"
} > "$RUN_DIR/run_meta.txt"
cat "$RUN_DIR/run_meta.txt" >&2

mkdir -p "$RUN_DIR/project"
# The startup XML must stay the repo default (a .vit package like 912.vit is
# NOT parseable as startup XML — the journey loads it through the
# open_project command path instead; surgical-run 20260925-013355 evidence).
cp "$REAL_DEFAULT_PROJECT" "$RUN_DIR/project/default_project.xml"
prereq "project_copy_sha256=$(sha "$RUN_DIR/project/default_project.xml") (source project never written; VIT_PROJECT_XML redirect)"
if [[ -n "$PROJECT_SOURCE" ]]; then
  [[ -f "$PROJECT_SOURCE" ]] || { echo "ERROR[env]: project source not found: $PROJECT_SOURCE" >&2; exit 2; }
  cp "$PROJECT_SOURCE" "$RUN_DIR/project/load_spot_project.vit"
  prereq "project_source=$PROJECT_SOURCE (opened via open_project for the load spot)"
fi
if [[ -n "$STEMS_DIR" ]]; then
  # 912.vit fixture provisioning (journey-script semantics): the project's
  # clips store PC-absolute literal paths; the kernel resolves each stored
  # string as ONE filename under the project dir.
  python3 - "$RUN_DIR/project" "$STEMS_DIR" <<'PY'
import os, shutil, sys
project_dir, stems_dir = sys.argv[1], sys.argv[2]
prefix = (r'D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures'
          r'\semantic_processor_project_smoke_v1_80085263a651cf20\cases\spv1_p01\stems')
for name in ("bass", "drums", "guitar", "other", "piano", "vocals"):
    src = os.path.join(stems_dir, name + ".wav")
    dst = os.path.join(project_dir, prefix + "\\" + name + ".wav")
    if os.path.isfile(src):
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copyfile(src, dst)
        print("copied", name)
    else:
        print("MISSING", name, src)
PY
  prereq "stems_provisioned=1"
fi
settings_snapshot "preflight"

# ================================================================ LEG A
if has_leg A; then
  step "LEG A: startup lazy cleanup (construct stale -> start -> summary line -> restore)"
  settings_snapshot "A_before"
  BASE_TYPES=$(settings_type_count)
  BASE_BLACKLIST=$(settings_blacklist_entries | grep -c . || true)
  info "baseline types=$BASE_TYPES blacklist_entries=$BASE_BLACKLIST"

  python3 - "$REAL_SETTINGS" <<'PY'
import sys
path = sys.argv[1]
xml = open(path, encoding="utf-8").read()
stale_types = (
    '      <PLUGIN name="HygieneStaleA" format="VST3" category="Fx" manufacturer="HygieneProbe"\n'
    '              version="1.0" file="/nonexistent/hygiene_stale_a.vst3" uniqueId="aa000001"\n'
    '              isInstrument="0" fileTime="0" infoUpdateTime="0" numInputs="2" numOutputs="2"\n'
    '              isShell="0" hasARAExtension="0" uid="aa000001"/>\n'
    '      <PLUGIN name="HygieneStaleB" format="VST3" category="Fx" manufacturer="HygieneProbe"\n'
    '              version="1.0" file="/nonexistent/hygiene_stale_b.vst3" uniqueId="aa000002"\n'
    '              isInstrument="0" fileTime="0" infoUpdateTime="0" numInputs="2" numOutputs="2"\n'
    '              isShell="0" hasARAExtension="0" uid="aa000002"/>\n'
    '      <BLACKLISTED id="/nonexistent/hygiene_stale_blacklisted.vst3"/>\n'
)
marker = "</KNOWNPLUGINS>"
assert marker in xml, "KNOWNPLUGINS closing tag not found"
xml = xml.replace(marker, stale_types + marker, 1)
open(path, "w", encoding="utf-8").write(xml)
print("injected 2 stale PLUGIN + 1 stale BLACKLISTED")
PY
  [[ $? -eq 0 ]] || { bad "LEG A: stale-entry injection failed"; }
  CONSTRUCT_TYPES=$(settings_type_count)
  CONSTRUCT_BLACKLIST=$(settings_blacklist_entries | grep -c . || true)
  settings_snapshot "A_constructed"
  prereq "A_constructed types=$CONSTRUCT_TYPES (baseline $BASE_TYPES + 2), blacklist=$CONSTRUCT_BLACKLIST (baseline + 1)"

  start_kernel "A" || true
  if [[ -n "$KERNEL_PID" ]]; then
    sleep 8
    HYGIENE_LINE="$(grep -m1 "PluginListHygiene: removed" "$KERNEL_LOG" | ascii_only || true)"
    if [[ -n "$HYGIENE_LINE" ]]; then
      ok "LEG A: cleanup summary line present"
      echo "$HYGIENE_LINE" > "$RUN_DIR/legA_summary_line.txt"
      EXPECTED_TYPES=$((BASE_TYPES + 2))
      REMOVED_TYPES="$(printf '%s' "$HYGIENE_LINE" | sed -n 's/.*removed \([0-9]*\) stale type entries.*/\1/p')"
      REMOVED_BLACKLIST="$(printf '%s' "$HYGIENE_LINE" | sed -n 's/.*and \([0-9]*\) stale blacklist entries.*/\1/p')"
      if [[ "${REMOVED_TYPES:-0}" -eq 2 && "${REMOVED_BLACKLIST:-0}" -eq 1 ]]; then
        ok "LEG A: counts exact (2 stale types of $EXPECTED_TYPES, 1 stale blacklist of 1)"
      else
        # Mac defect canary: valid mac entries are VST3 bundle DIRECTORIES and
        # juce::File::existsAsFile() is false for directories, so the cleanup
        # mass-removes the whole warm list (PluginListHygiene.cpp:15-18).
        VALID_JUDGED_STALE=$(( ${REMOVED_TYPES:-0} - 2 ))
        bad "LEG A DEFECT (mac): removed ${REMOVED_TYPES} type entries — ${VALID_JUDGED_STALE} VALID mac bundle entries judged stale by existsAsFile(); anchor VitApp/Source/Service/PluginListHygiene.cpp:17; constructed samples were 2 types + 1 blacklist (got blacklist=${REMOVED_BLACKLIST})"
        python3 - "$RUN_DIR/settings_states/settings_A_constructed.xml" > "$RUN_DIR/legA_defect_quantify.txt" <<'PY'
import os, sys, xml.etree.ElementTree as ET
tree = ET.parse(sys.argv[1])
files = [p.get("file", "") for v in tree.getroot().iter("VALUE")
         if v.get("name") == "knownPluginList64" for p in v.iter("PLUGIN")]
isfile = sum(1 for f in files if os.path.isfile(f))
isdir = sum(1 for f in files if os.path.isdir(f))
missing = sum(1 for f in files if not os.path.exists(f))
print("entries=%d isfile(survive)=%d isdir(bundle, wrongly stale)=%d missing(truly stale)=%d"
      % (len(files), isfile, isdir, missing))
PY
        cat "$RUN_DIR/legA_defect_quantify.txt" | tee -a "$PREREQ_FILE" >&2
      fi
      [[ "$HYGIENE_LINE" == *"/nonexistent/hygiene_stale_a.vst3"* ]] \
        && ok "LEG A: constructed stale path named in summary (removed)" || bad "LEG A: constructed stale path missing from summary"
      LINES=$(grep -c "PluginListHygiene:" "$KERNEL_LOG" || true)
      [[ "$LINES" -le 1 ]] && ok "LEG A: exactly one hygiene line (no per-entry flood; got $LINES)" || bad "LEG A: hygiene line flood ($LINES)"
    else
      bad "LEG A: no PluginListHygiene summary line in kernel log"
    fi
    stop_kernel "A"
  fi

  # restore regardless of verdict (§10)
  cp "$RUN_DIR/settings_states/settings_A_before.xml" "$REAL_SETTINGS"
  RESTORED=$(sha "$REAL_SETTINGS")
  BASE_SHA=$(cat "$RUN_DIR/settings_states/settings_A_before.sha256")
  [[ "$RESTORED" == "$BASE_SHA" ]] && ok "LEG A: Settings.xml restored byte-identical ($RESTORED)" \
    || bad "LEG A: Settings.xml restore mismatch ($RESTORED != $BASE_SHA)"
fi

# ================================================================ LEG B
if has_leg B; then
  step "LEG B: cancel mid-scan -> no new blacklist + partial results kept -> rescan completes"
  settings_snapshot "B_before"
  B_TYPES_BEFORE=$(settings_type_count)
  B_BLACKLIST_BEFORE="$(settings_blacklist_entries | sort | sha1sum | cut -d' ' -f1 || true)"
  B_BLACKLIST_COUNT=$(settings_blacklist_entries | grep -c . || true)
  info "LEG B start state: types=$B_TYPES_BEFORE blacklist=$B_BLACKLIST_COUNT"

  LEGB_OK=1
  start_kernel "B1" || LEGB_OK=0
  if [[ "$LEGB_OK" -eq 1 ]]; then
    # In-session baseline: the startup hygiene cleanup may legitimately (or,
    # under the mac bundle-dir defect, wrongly) change the list after load —
    # cancel/rescan semantics are asserted session-relative.
    printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/list_req.json"
    zmq_send "$RUN_DIR/replies/list_req.json" "$RUN_DIR/replies/B_list_session_start.json" 60000 || true
    SESSION_START_COUNT="$(python3 - "$RUN_DIR/replies/B_list_session_start.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    print(len(d.get("plugins") or []))
except Exception:
    print(-1)
PY
)"
    prereq "B_session_start_list_count=$SESSION_START_COUNT (settings-file count was $B_TYPES_BEFORE; a large gap is the leg-A mac-defect signature, not a cancel-scan failure)"
    scan_cmd
    zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/B_scan_start.json" 60000 || LEGB_OK=0
    SCAN_ID="$(jfield "$RUN_DIR/replies/B_scan_start.json" 'str(d.get("scan_id",""))')"
    SCAN_STATE="$(jfield "$RUN_DIR/replies/B_scan_start.json" 'str(d.get("status",""))')"
    info "scan start: state=$SCAN_STATE scan_id=$SCAN_ID"
    [[ "$SCAN_STATE" == "scanning" && -n "$SCAN_ID" ]] || { bad "LEG B: scan did not start scanning (state=$SCAN_STATE)"; LEGB_OK=0; }

    if [[ "$LEGB_OK" -eq 1 ]]; then
      # wait until at least CANCEL_MIN_COMPLETED files completed, then cancel
      deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
      while (( SECONDS < deadline )); do
        printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/status_req.json"
        zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/B_status_precancel.json" 30000 || true
        st="$(jfield "$RUN_DIR/replies/B_status_precancel.json" 'str(d.get("status",""))')"
        cf="$(jfield "$RUN_DIR/replies/B_status_precancel.json" 'd.get("completed_files",0)')"
        cur="$(jfield "$RUN_DIR/replies/B_status_precancel.json" 'str(d.get("current_file",""))')"
        [[ "$st" != "scanning" ]] && break
        if (( ${cf:-0} >= CANCEL_MIN_COMPLETED )) && [[ -n "$cur" ]]; then break; fi
        sleep 3
      done
      CUR_FILE="$(jfield "$RUN_DIR/replies/B_status_precancel.json" 'str(d.get("current_file",""))')"
      COMPLETED_AT_CANCEL="$(jfield "$RUN_DIR/replies/B_status_precancel.json" 'd.get("completed_files",0)')"
      prereq "B_cancel_point completed_files=$COMPLETED_AT_CANCEL current_file=$CUR_FILE"

      printf '{"cmd":"plugin_scan_cancel","scan_id":"%s"}' "$SCAN_ID" > "$RUN_DIR/replies/cancel_req.json"
      zmq_send "$RUN_DIR/replies/cancel_req.json" "$RUN_DIR/replies/B_cancel.json" 30000 || LEGB_OK=0
      CANCEL_REPLY_STATE="$(jfield "$RUN_DIR/replies/B_cancel.json" 'str(d.get("status",""))')"
      [[ "$CANCEL_REPLY_STATE" == "cancelling" || "$CANCEL_REPLY_STATE" == "cancelled" ]] \
        && ok "LEG B: cancel accepted (state=$CANCEL_REPLY_STATE)" \
        || { bad "LEG B: cancel reply state=$CANCEL_REPLY_STATE"; LEGB_OK=0; }

      FINAL="$(wait_scan_terminal "$SCAN_ID" "B_cancelled" 300)" || true
      [[ "$FINAL" == "cancelled" ]] && ok "LEG B: scan reached terminal state=cancelled" \
        || { bad "LEG B: scan terminal state=$FINAL (expected cancelled)"; LEGB_OK=0; }

      CANCEL_LOG="$(grep -m1 "dead-man's pedal cleared" "$KERNEL_LOG" | ascii_only || true)"
      [[ -n "$CANCEL_LOG" && "$CANCEL_LOG" == *"cancelled"* ]] && ok "LEG B: cancel epilogue log present" || bad "LEG B: cancel epilogue log line missing"
      echo "$CANCEL_LOG" > "$RUN_DIR/legB_cancel_log_line.txt"

      [[ ! -e "$REAL_PEDAL" ]] && ok "LEG B: dead-man's pedal cleared after cancel" \
        || { bad "LEG B: pedal file still present after cancel"; LEGB_OK=0; }

      # partial results retained: live list never shrinks below pre-scan count
      printf '{"cmd":"plugin_list_available"}' > "$RUN_DIR/replies/list_req.json"
      zmq_send "$RUN_DIR/replies/list_req.json" "$RUN_DIR/replies/B_list_after_cancel.json" 60000 || true
      AFTER_CANCEL_COUNT="$(python3 - "$RUN_DIR/replies/B_list_after_cancel.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    print(len(d.get("plugins") or []))
except Exception:
    print(-1)
PY
)"
      prereq "B_types_after_cancel=$AFTER_CANCEL_COUNT (session start: $SESSION_START_COUNT, settings file: $B_TYPES_BEFORE)"
      (( ${AFTER_CANCEL_COUNT:-0} >= ${SESSION_START_COUNT:-0} )) \
        && ok "LEG B: partial results retained (list $AFTER_CANCEL_COUNT >= session-start $SESSION_START_COUNT)" \
        || { bad "LEG B: list shrank after cancel ($AFTER_CANCEL_COUNT < session-start $SESSION_START_COUNT)"; LEGB_OK=0; }
    fi

    if [[ "$LEGB_OK" -eq 1 ]]; then
      step "LEG B (rescan): full rescan must complete with the full count (no silent skip)"
      scan_cmd
      zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/B_rescan_start.json" 60000 || LEGB_OK=0
      RESCAN_ID="$(jfield "$RUN_DIR/replies/B_rescan_start.json" 'str(d.get("scan_id",""))')"
      FINAL2="$(wait_scan_terminal "$RESCAN_ID" "B_rescan")" || LEGB_OK=0
      [[ "$FINAL2" == "completed" ]] && ok "LEG B: rescan completed" \
        || { bad "LEG B: rescan terminal state=$FINAL2"; LEGB_OK=0; }
      RESCAN_PLUGINS="$(jfield "$RUN_DIR/replies/B_rescan_status.json" 'd.get("plugin_count",0)')"
      RESCAN_COMPLETED_FILES="$(jfield "$RUN_DIR/replies/B_rescan_status.json" 'd.get("completed_files",0)')"
      RESCAN_TOTAL="$(jfield "$RUN_DIR/replies/B_rescan_status.json" 'd.get("total_files",0)')"
      prereq "B_rescan plugin_count=$RESCAN_PLUGINS completed_files=$RESCAN_COMPLETED_FILES total_files=$RESCAN_TOTAL"
      (( ${RESCAN_PLUGINS:-0} >= 700 )) \
        && ok "LEG B: rescan recovered the full library count ($RESCAN_PLUGINS >= 700; the 09-23 incident form was a silent collapse)" \
        || { bad "LEG B: rescan count too low ($RESCAN_PLUGINS < 700)"; LEGB_OK=0; }
      (( ${RESCAN_COMPLETED_FILES:-0} >= ${RESCAN_TOTAL:-1} )) \
        && ok "LEG B: rescan probed every file (${RESCAN_COMPLETED_FILES}/${RESCAN_TOTAL}, no skip)" \
        || bad "LEG B: rescan skipped files (${RESCAN_COMPLETED_FILES}/${RESCAN_TOTAL})"
      grep -c "probing" "$KERNEL_LOG" > "$RUN_DIR/legB_probing_line_count.txt" || true
    fi
    stop_kernel "B1"
  fi

  # post-shutdown settings: no new blacklist entries
  settings_snapshot "B_after"
  B_BLACKLIST_AFTER_COUNT=$(settings_blacklist_entries | grep -c . || true)
  B_BLACKLIST_AFTER_DIFF="$(settings_blacklist_entries | grep -v '^$' | sort > "$RUN_DIR/legB_blacklist_after.txt"; diff <(echo "") "$RUN_DIR/legB_blacklist_after.txt" | grep '^>' | grep -v '^> $' || true)"
  if [[ -z "$B_BLACKLIST_AFTER_DIFF" ]]; then
    ok "LEG B: settings blacklist unchanged after cancel+rescan+shutdown (count=$B_BLACKLIST_AFTER_COUNT)"
  else
    bad "LEG B: new blacklist entries after cancel+rescan: $B_BLACKLIST_AFTER_DIFF"
  fi
  prereq "B_types_after_full_rescan=$(settings_type_count)"
fi

# ================================================================ LEG C
if has_leg C; then
  step "LEG C: kill -9 mid-scan -> pedal survives -> restart scan consumes it -> blacklisted (defence intact)"
  settings_snapshot "C_before"
  C_BLACKLIST_BEFORE="$(settings_blacklist_entries | sort | sha1sum | cut -d' ' -f1)"
  rm -f "$REAL_PEDAL"
  LEGC_OK=1
  start_kernel "C1" || LEGC_OK=0
  if [[ "$LEGC_OK" -eq 1 ]]; then
    scan_cmd
    zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/C_scan_start.json" 60000 || LEGC_OK=0
    C_SCAN_ID="$(jfield "$RUN_DIR/replies/C_scan_start.json" 'str(d.get("scan_id",""))')"
    [[ "$(jfield "$RUN_DIR/replies/C_scan_start.json" 'str(d.get("status",""))')" == "scanning" ]] \
      || { bad "LEG C: scan did not start"; LEGC_OK=0; }

    if [[ "$LEGC_OK" -eq 1 ]]; then
      # wait until the first (fast) file completed so the kill lands inside
      # the SLOW main-shell probe (killing during a <1s probe usually misses)
      deadline=$((SECONDS + SCAN_TIMEOUT_SECONDS))
      CUR=""
      CFC=0
      while (( SECONDS < deadline )); do
        printf '{"cmd":"plugin_scan_status","scan_id":"%s"}' "$C_SCAN_ID" > "$RUN_DIR/replies/status_req.json"
        zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/C_status_prekill.json" 30000 >/dev/null || break
        st="$(jfield "$RUN_DIR/replies/C_status_prekill.json" 'str(d.get("status",""))')"
        cur="$(jfield "$RUN_DIR/replies/C_status_prekill.json" 'str(d.get("current_file",""))')"
        cfc="$(jfield "$RUN_DIR/replies/C_status_prekill.json" 'd.get("completed_files",0)')"
        [[ "$st" != "scanning" ]] && break
        if [[ -n "$cur" ]]; then CUR="$cur"; CFC="${cfc:-0}"; fi
        if (( ${CFC:-0} >= 1 )); then break; fi
        sleep 2
      done
      prereq "C_kill_point completed_files=$CFC current_file=$CUR"
      [[ -n "$CUR" ]] || { bad "LEG C: no current_file observable before kill"; LEGC_OK=0; }
    fi

    if [[ "$LEGC_OK" -eq 1 ]]; then
      stop_kernel "C1" KILL
      sleep 2
      # Settle: the killed kernel's orphaned out-of-process scan child keeps
      # probing the Waves shell after the parent dies; a restart scan that
      # loads the SAME shell contends with it and the loser gets
      # crash-isolated (run 20260925-020259 evidence: next scan completed
      # plugins=1 with the main shell in failed_files). Wait it out.
      SETTLE_START=$SECONDS
      SETTLE_DEADLINE=$((SECONDS + 480))
      while (( SECONDS < SETTLE_DEADLINE )) && pgrep -f -- "--PluginScan:" >/dev/null 2>&1; do
        sleep 5
      done
      prereq "C_orphan_scan_child_settle=$((SECONDS - SETTLE_START))s"
      # The pedal is UTF-16LE with BOM (juce File::replaceWithText unicode
      # header) — decode before matching, a plain grep can never hit.
      PEDAL_MATCH="$(python3 -c "
import sys
try:
    text = open(sys.argv[1], encoding='utf-16').read()
except Exception:
    text = open(sys.argv[1], encoding='utf-8', errors='replace').read()
print('yes' if sys.argv[2] in text else 'no')" "$REAL_PEDAL" "$CUR" 2>/dev/null || echo no)"
      if [[ -e "$REAL_PEDAL" && "$PEDAL_MATCH" == "yes" ]]; then
        PEDAL_CONTENT="$(head -c 500 "$REAL_PEDAL" | ascii_only)"
        cp "$REAL_PEDAL" "$RUN_DIR/legC_pedal_residue.txt"
        ok "LEG C: dead-man's pedal survived kill -9 AND names the killed mid-probe file (UTF-16 content; raw copy kept)"
      elif [[ -e "$REAL_PEDAL" ]]; then
        cp "$REAL_PEDAL" "$RUN_DIR/legC_pedal_residue.txt"
        bad "LEG C: pedal survived kill -9 but does not name the killed file ($CUR); see legC_pedal_residue.txt"
      else
        bad "LEG C: pedal file missing after kill -9 (kill landed outside probing?)"
        # Non-fatal for the dichotomy verdict but the consume-assert below cannot run.
      fi

      step "LEG C (restart): next scan must blacklist the crashed file (pedal residue consumed)"
      start_kernel "C2" || LEGC_OK=0
      if [[ -n "$KERNEL_PID" ]]; then
        scan_cmd
        zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/C_rescan_start.json" 60000 || LEGC_OK=0
        # The scanner blacklists the pedal residue inside
        # setFilesOrIdentifiersToScan (juce_PluginDirectoryScanner.cpp:78) —
        # the pedal FILE itself is neither deleted nor required to vanish
        # (it is rewritten by later probing), so the live observable is the
        # blacklist surfacing in plugin_scan_status.failed_files.
        BLACKLIST_OBSERVED=""
        OBSERVE_DEADLINE=$((SECONDS + 120))
        while (( SECONDS < OBSERVE_DEADLINE )); do
          printf '{"cmd":"plugin_scan_status"}' > "$RUN_DIR/replies/status_req.json"
          zmq_send "$RUN_DIR/replies/status_req.json" "$RUN_DIR/replies/C_consume_status.json" 30000 >/dev/null || true
          FAILED="$(jfield "$RUN_DIR/replies/C_consume_status.json" '",".join(d.get("failed_files") or [])')"
          if [[ "$FAILED" == *"$CUR"* ]]; then BLACKLIST_OBSERVED="$FAILED"; break; fi
          sleep 2
        done
        if [[ -n "$BLACKLIST_OBSERVED" ]]; then
          ok "LEG C: crashed-at-kill file blacklisted at restart scan (live failed_files contains it; defence NOT weakened)"
          prereq "C_consume_failed_files=$BLACKLIST_OBSERVED"
        else
          bad "LEG C: crashed file never surfaced in failed_files after restart scan"
        fi
        # Graceful scan end (cancel) before stopping: the pedal-derived
        # blacklist entry was captured in blacklistAtScannerStart (snapshot
        # AFTER setFilesOrIdentifiersToScan consumed the pedal), so the
        # cancel epilogue KEEPS it — and the graceful scan finish flushes
        # the list to Settings.xml where a SIGTERM-only stop may not.
        C2_SCAN_ID="$(jfield "$RUN_DIR/replies/C_rescan_start.json" 'str(d.get("scan_id",""))')"
        if [[ -n "$C2_SCAN_ID" ]]; then
          printf '{"cmd":"plugin_scan_cancel","scan_id":"%s"}' "$C2_SCAN_ID" > "$RUN_DIR/replies/cancel_req.json"
          zmq_send "$RUN_DIR/replies/cancel_req.json" "$RUN_DIR/replies/C_cancel_consume.json" 30000 >/dev/null || true
          C2C_FINAL="$(wait_scan_terminal "$C2_SCAN_ID" "C_consume_cancel" 300)" || true
          prereq "C_consume_scan_end=$C2C_FINAL (cancel keeps the crash blacklist by snapshot design)"
        fi
        stop_kernel "C2"
        # blacklist assertion on the persisted settings
        C_BLACKLIST_NEW="$(settings_blacklist_entries | grep . | sort)"
        echo "$C_BLACKLIST_NEW" > "$RUN_DIR/legC_blacklist_after.txt"
        settings_snapshot "C_after"
        if [[ -n "$C_BLACKLIST_NEW" ]]; then
          if grep -qxF "$CUR" "$RUN_DIR/legC_blacklist_after.txt" || \
             grep -q "^$CUR$" "$RUN_DIR/legC_blacklist_after.txt" || \
             [[ "$C_BLACKLIST_NEW" == *"$CUR"* ]]; then
            ok "LEG C: crashed-at-probe file blacklisted after restart (defence NOT weakened): $CUR"
          else
            bad "LEG C: blacklist changed but killed file not in it: $C_BLACKLIST_NEW"
          fi
        else
          # persistence may lag; the pedal consumption + unit-test dichotomy is the primary anchor
          bad "LEG C: no blacklist persisted after restart (pedal-consumption blacklisting not observable in settings)"
        fi
      fi
    fi
  fi

  # restore pre-leg-C settings (remove the defence-blacklisted shell so the
  # machine's warm list is not degraded — documented restore, §10)
  if [[ -f "$RUN_DIR/settings_states/settings_C_before.xml" ]]; then
    cp "$RUN_DIR/settings_states/settings_C_before.xml" "$REAL_SETTINGS"
    R=$(sha "$REAL_SETTINGS"); B=$(cat "$RUN_DIR/settings_states/settings_C_before.sha256")
    [[ "$R" == "$B" ]] && ok "LEG C: Settings.xml restored to pre-leg state ($R)" || bad "LEG C: restore mismatch"
  fi
  rm -f "$REAL_PEDAL"
  prereq "C_pedal_cleanup=removed"
fi

# ================================================================ LEG D
if has_leg D; then
  step "LEG D: full scan + load spot through knownPluginList resolution"
  settings_snapshot "D_before"
  LEGD_OK=1
  start_kernel "D" || LEGD_OK=0
  if [[ "$LEGD_OK" -eq 1 ]]; then
    if [[ "$SKIP_FULL_RESCAN" -eq 1 ]]; then
      info "LEG D: --skip-full-rescan: reusing warm list for the load spot"
      D_SCAN_NOTE="skipped (warm list reused)"
    else
      scan_cmd
      zmq_send "$RUN_DIR/replies/scan_req.json" "$RUN_DIR/replies/D_scan_start.json" 60000 || LEGD_OK=0
      D_SCAN_ID="$(jfield "$RUN_DIR/replies/D_scan_start.json" 'str(d.get("scan_id",""))')"
      D_FINAL="$(wait_scan_terminal "$D_SCAN_ID" "D_scan")" || LEGD_OK=0
      D_PLUGINS="$(jfield "$RUN_DIR/replies/D_scan_status.json" 'd.get("plugin_count",0)')"
      D_COMPLETED="$(jfield "$RUN_DIR/replies/D_scan_status.json" 'd.get("completed_files",0)')"
      D_TOTAL="$(jfield "$RUN_DIR/replies/D_scan_status.json" 'd.get("total_files",0)')"
      D_SCAN_NOTE="state=$D_FINAL plugin_count=$D_PLUGINS completed_files=$D_COMPLETED total_files=$D_TOTAL"
      prereq "D_full_scan $D_SCAN_NOTE"
      [[ "$D_FINAL" == "completed" ]] && ok "LEG D: full scan completed" || { bad "LEG D: full scan state=$D_FINAL"; LEGD_OK=0; }
      (( ${D_PLUGINS:-0} > 100 )) && ok "LEG D: full scan enumerated ${D_PLUGINS} plugins (>100)" || { bad "LEG D: plugin count too low: $D_PLUGINS"; LEGD_OK=0; }
    fi

    if [[ "$LEGD_OK" -eq 1 ]]; then
      step "LEG D (load spot): rack_add_node with plugin_identifier"
      if [[ -f "$RUN_DIR/project/load_spot_project.vit" ]]; then
        python3 - "$RUN_DIR/project/load_spot_project.vit" > "$RUN_DIR/replies/D_open_project_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "open_project", "file_path": sys.argv[1]}))
PY
        zmq_send "$RUN_DIR/replies/D_open_project_req.json" "$RUN_DIR/replies/D_open_project.json" 180000 >/dev/null || true
        OPEN_STATUS="$(jfield "$RUN_DIR/replies/D_open_project.json" 'str(d.get("status",""))')"
        prereq "D_open_project status=$OPEN_STATUS"
        [[ "$OPEN_STATUS" == "ok" ]] || { bad "LEG D: open_project failed ($OPEN_STATUS): $(head -c 300 "$RUN_DIR/replies/D_open_project.json" | ascii_only)"; LEGD_OK=0; }
        sleep 6
      fi
      printf '{"cmd":"list_tracks"}' > "$RUN_DIR/replies/list_tracks_req.json"
      zmq_send "$RUN_DIR/replies/list_tracks_req.json" "$RUN_DIR/replies/D_list_tracks.json" 30000 || true
      TRACK_ID="$(python3 - "$RUN_DIR/replies/D_list_tracks.json" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1], encoding="utf-8"))
    rows = d.get("tracks") or d.get("result") or []
    if isinstance(rows, dict):
        rows = list(rows.values())
    rows = [t for t in rows if isinstance(t, dict)]
    def rank(t):
        name = str(t.get("name", "")).lower()
        kind = str(t.get("track_type", t.get("type", "track"))).lower()
        # prefer the journey bass track, then plain audio tracks (the repo
        # default project only carries Arranger/Chord/Marker/Tempo/Master —
        # none of them hosts a rack zone for the load spot)
        if name == "bass":
            return 0
        if kind not in ("master", "marker", "tempo", "arranger", "chord", "aux", "bus"):
            return 1
        return 2
    for t in sorted(rows, key=rank):
        tid = str(t.get("track_id") or t.get("id") or "")
        if tid and rank(t) < 2:
            print(tid); break
except Exception:
    pass
PY
)"
      prereq "D_load_spot track_id=$TRACK_ID identifier=$LOAD_SPOT_IDENTIFIER"
      if [[ -n "$TRACK_ID" ]]; then
        python3 - "$TRACK_ID" "$LOAD_SPOT_IDENTIFIER" "$ZONE_ID" > "$RUN_DIR/replies/D_rack_add_req.json" <<'PY'
import json, sys
print(json.dumps({"cmd": "rack_add_node",
                  "track_id": sys.argv[1], "plugin_identifier": sys.argv[2],
                  "x": 0, "y": 0, "zone_id": sys.argv[3]}))
PY
        zmq_send "$RUN_DIR/replies/D_rack_add_req.json" "$RUN_DIR/replies/D_rack_add.json" 180000 || LEGD_OK=0
        ADD_STATUS="$(jfield "$RUN_DIR/replies/D_rack_add.json" 'str(d.get("status",""))')"
        ADD_PLUGIN_ID="$(jfield "$RUN_DIR/replies/D_rack_add.json" 'str(d.get("plugin_id", d.get("result",{}).get("plugin_id","") if isinstance(d.get("result"),dict) else ""))')"
        ADD_ERR="$(jfield "$RUN_DIR/replies/D_rack_add.json" 'str(d.get("message", d.get("error","")))')"
        prereq "D_rack_add status=$ADD_STATUS plugin_id=$ADD_PLUGIN_ID error=$ADD_ERR"
        [[ "$ADD_STATUS" == "ok" && -n "$ADD_PLUGIN_ID" ]] \
          && ok "LEG D: load spot landed (status=ok plugin_id=$ADD_PLUGIN_ID; knownPluginList resolution path)" \
          || { bad "LEG D: rack_add_node failed: status=$ADD_STATUS error=$ADD_ERR"; LEGD_OK=0; }
      else
        bad "LEG D: no track resolvable from list_tracks for the load spot"
        LEGD_OK=0
      fi
    fi
    stop_kernel "D"
  fi
  settings_snapshot "D_after"
fi

# ================================================================ summary
step "Summary"
python3 - "$RUN_DIR" "$RUN_ID" "$LEGS" <<'PY'
import json, os, sys
run_dir, run_id, legs = sys.argv[1], sys.argv[2], sys.argv[3]
ok_lines = []
red_lines = []
if os.path.isfile(os.path.join(run_dir, "prereq.txt")):
    for line in open(os.path.join(run_dir, "prereq.txt"), encoding="utf-8", errors="replace"):
        line = line.rstrip("\n")
        if line.startswith("ok: "):
            ok_lines.append(line[4:])
        elif line.startswith("RED: "):
            red_lines.append(line[5:])
report = {
    "schema_version": "vit_kernel_pluginlist_hygiene_smoke.mac.v1",
    "card": "PORT-PCBATCH-MAC-LEGS-1 leg2 (FIX-KERNEL-PLUGINLIST-HYGIENE-1 ①-④)",
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
  echo "HYGIENE_MAC_VERDICT all_green" >&2
  exit 0
else
  echo "HYGIENE_MAC_VERDICT assertions_red" >&2
  exit 1
fi
