#!/usr/bin/env python3
"""WAVESHELL-OFFRENDER-FIX-1 (card 309) stack-leg driver, 2026-11-09.

One stack window, five experiment legs (card rulings 1-5):
  --leg-mode realtime (E1/M1): fixture + direct-mount + 16-note clip, then
      return_to_zero -> play -> sample the kernel log for VIT309_RTLEVEL tap
      lines (VitApp env-gated instrumentation) -> stop -> return_to_zero.
      Verdict realtime_verdict = sound|silent|marginal, ALL of which are valid
      observations (exit 0). Zero RTLEVEL lines at all = exit 4 (tap/harness
      defect), never silently retried.
  --leg-mode render (E2a/E2b/E3/E4/E5): the 301 M2 shape verbatim (precheck ->
      new_project -> add_track -> mount -> 16-note clip -> start_render ->
      physical wav metering) with three additions:
        --mount-mode direct (301 M2 verbatim instantiate_plugin) or rack
            (rack_add_node, the seg3a_r2/M0a2 mount shape);
        start_render reply capture (shell_class_plugin_hint / realtime_fallback
            = fix-B disclosure) and render_done capture
            (degraded_to_realtime_render / render_silence_observed = form-3
            disclosure);
        VIT301_GUARD + VIT309_* trace capture from the kernel logs
            (mode telemetry: external_initialise / prepareToPlay /
            setupProcessing / firstProcessAudio / BLACKLIST / DEGRADED /
            RTLEVEL).
  Silent vs non-silent render output is an observation, not a failure exit
  (same contract as 301); the fix-B / form-3 / C-prototype state is read from
  the reply, the render_done publication and the kernel log traces.

Environment contract (set by dev_agent_smoke.ps1 switches on the KERNEL
process): VIT309_RTLEVEL (E1 tap), VIT309_DISABLE_BLACKLIST (E2a/E2b bypass
fix B), VIT309_C_CONSISTENCY (E2b fix-C prototype). The driver records the
expected flags for the receipt but does not assert process env itself.

Exit codes (fixed before execution, single run, no rerun):
  0 chain_complete   leg observation recorded (any verdict; silent/non-silent
                     render and sound/silent realtime are both valid outcomes)
  2 environment      stack/transport/fixture failure, including the requested
                     plugin missing from the local inventory
  4 command_defect   deterministic chain defect: mount/visibility/clip/render
                     status != ok, render product missing, or realtime leg with
                     zero RTLEVEL tap lines (forensics hand-over, no rerun)
"""

from __future__ import annotations

import argparse
import json
import math
import re
import struct
import sys
import time
import wave
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

CLIP_LENGTH_BEATS = 16.0
RENDER_WAIT_SEC = 240.0
NON_SILENCE_THRESHOLD = 500  # max |sample| out of int16 full scale
NON_SILENCE_RMS_THRESHOLD = 20.0  # same metering contract as 301

# M1 verdict thresholds (recon section 3.2): sound > -60 dB, silent <= -90 dB.
RT_SOUND_DB = -60.0
RT_SILENT_DB = -90.0
RT_SAMPLE_SEC = 5.0  # >= 4s of play covering the note span

# 16-beat C-major arpeggio fixture (byte-identical to seg3a_r2 / 301).
ARPEGGIO: List[int] = [72, 76, 79, 84, 79, 76] + [72, 76, 79, 84, 79, 76] + [72, 74, 76, 79]

EXIT_CHAIN_COMPLETE = 0
EXIT_ENV = 2
EXIT_CMD_DEFECT = 4

TRACE_MARKERS = ("VIT301_GUARD", "VIT309_")
RTLEVEL_LINE_RE = re.compile(r"VIT309_RTLEVEL .*?db_l=(-?[\d.]+) db_r=(-?[\d.]+)")

PLUGIN_SELECTORS: Dict[str, Dict[str, str]] = {
    "grand_rhapsody": {"name_contains": "Grand Rhapsody",
                       "label": "Grand Rhapsody (WaveShell shared container)"},
    "surge_xt": {"name_contains": "Surge XT",
                 "label": "Surge XT (plugin-specificity control)"},
}


def zmq_command(url: str, payload: Dict[str, Any], timeout_ms: int = 60000) -> Dict[str, Any]:
    context = zmq.Context.instance()
    sock = context.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    try:
        sock.connect(url)
        sock.send_json(payload)
        return json.loads(sock.recv_string())
    finally:
        sock.close(0)


def wait_kernel_zmq(url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            reply = zmq_command(url, {"cmd": "ping"}, timeout_ms=5000)
            if str(reply.get("status", "")).lower() in {"ok", "success"}:
                return
            last_error = RuntimeError(f"ping replied {reply.get('status')!r}")
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def rows_of(value: Any) -> List[Dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    return []


def wav_max_abs(path: Path) -> Dict[str, Any]:
    with wave.open(str(path), "rb") as handle:
        channels = handle.getnchannels()
        width = handle.getsampwidth()
        frames = handle.getnframes()
        raw = handle.readframes(frames)
    if width != 2:
        return {"error": f"unexpected sample width {width}", "channels": channels, "frames": frames}
    count = len(raw) // 2
    samples = struct.unpack("<%dh" % count, raw[: count * 2])
    peak = max(abs(sample) for sample in samples) if samples else 0
    dbfs = round(20 * math.log10(peak / 32768.0), 2) if peak > 0 else -120.0
    square_sum = sum(float(sample) * float(sample) for sample in samples)
    rms = math.sqrt(square_sum / count) if count else 0.0
    rms_dbfs = round(20 * math.log10(rms / 32768.0), 2) if rms > 0 else -120.0
    return {
        "channels": channels,
        "frames": frames,
        "peak_abs": peak,
        "peak_dbfs": dbfs,
        "rms_abs": round(rms, 2),
        "rms_dbfs": rms_dbfs,
    }


def wait_render_done(sub_url: str, wav_path: Path, job_id: str, timeout_s: float) -> Dict[str, Any]:
    """Wait for the terminal render publication of this job. Returns the
    sub_signal plus the last job-matched render message (form-3 disclosure
    fields live there)."""
    outcome: Dict[str, Any] = {"sub_signal": None, "fallback_file": False, "last_message": None}
    context = zmq.Context.instance()
    sock = context.socket(zmq.SUB)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt_string(zmq.SUBSCRIBE, "")
    sock.setsockopt(zmq.RCVTIMEO, 1000)
    try:
        sock.connect(sub_url)
        deadline = time.time() + timeout_s
        while time.time() < deadline:
            try:
                payload = sock.recv_string()
            except zmq.error.Again:
                if wav_path.exists() and wav_path.stat().st_size > 44:
                    if time.time() > deadline - timeout_s / 3:
                        outcome["fallback_file"] = True
                        return outcome
                continue
            try:
                message = json.loads(payload)
            except json.JSONDecodeError:
                continue
            if not isinstance(message, dict):
                continue
            if message.get("topic") == "render" and str(message.get("job_id", "")) == job_id:
                sub = str(message.get("subtopic", ""))
                outcome["last_message"] = message
                if sub == "render_done" and str(message.get("status")) == "ok":
                    outcome["sub_signal"] = "render_done"
                    return outcome
                if sub in {"render_failed"} or (sub == "render_done" and str(message.get("status")) != "ok"):
                    outcome["sub_signal"] = f"{sub}:{message.get('status')}:{message.get('message', '')}"
                    return outcome
    finally:
        sock.close(0)
    if wav_path.exists() and wav_path.stat().st_size > 44:
        outcome["fallback_file"] = True
    return outcome


def newest_kernel_log(kernel_log_dir: Path) -> Optional[Path]:
    try:
        candidates = sorted(kernel_log_dir.glob("VitHeadlessServer*.log"),
                            key=lambda p: p.stat().st_mtime, reverse=True)
    except OSError:
        return None
    return candidates[0] if candidates else None


def read_log_from(path: Path, offset: int) -> Tuple[str, int]:
    try:
        with open(path, "rb") as handle:
            handle.seek(offset)
            data = handle.read()
        return data.decode("utf-8", errors="replace"), offset + len(data)
    except OSError:
        return "", offset


def collect_vit_trace(kernel_log_dir: Path, run_started_epoch: float) -> Dict[str, Any]:
    """Scan the kernel's VitHeadlessServer logs written since this run started
    for VIT301_GUARD / VIT309_* trace lines; copy them verbatim."""
    result: Dict[str, Any] = {"log_dir": str(kernel_log_dir), "logs_scanned": [],
                              "trace_lines": [], "error": None}
    try:
        candidates = sorted(kernel_log_dir.glob("VitHeadlessServer*.log"),
                            key=lambda p: p.stat().st_mtime, reverse=True)
    except OSError as exc:
        result["error"] = f"log dir scan failed: {exc}"
        return result
    for log_path in candidates[:4]:
        try:
            if log_path.stat().st_mtime < run_started_epoch - 120:
                continue  # stale log from an earlier kernel process
            text = log_path.read_text(encoding="utf-8", errors="replace")
            result["logs_scanned"].append({"file": log_path.name,
                                           "mtime": datetime.fromtimestamp(log_path.stat().st_mtime).isoformat()})
            for line in text.splitlines():
                if any(marker in line for marker in TRACE_MARKERS):
                    result["trace_lines"].append({"log": log_path.name, "line": line.strip()})
        except OSError as exc:
            result["logs_scanned"].append({"file": log_path.name, "error": str(exc)})
    return result


def telemetry_row_count(telemetry_path: Optional[Path], run_started_epoch: float) -> Dict[str, Any]:
    """Zero-LLM evidence for this leg: telemetry jsonl rows whose timestamps
    fall in this run window (recorded, not asserted)."""
    if telemetry_path is None or not telemetry_path.exists():
        return {"path": str(telemetry_path) if telemetry_path else None, "rows_total": None,
                "note": "telemetry file absent (no agent start or no rows yet)"}
    rows_total = 0
    recent: List[str] = []
    try:
        for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
            if not line.strip():
                continue
            rows_total += 1
            try:
                row = json.loads(line)
                ts = str(row.get("ts") or row.get("timestamp") or "")
                if ts and ts >= datetime.fromtimestamp(run_started_epoch).isoformat(timespec="seconds"):
                    recent.append(line[:220])
            except json.JSONDecodeError:
                continue
    except OSError as exc:
        return {"path": str(telemetry_path), "error": str(exc)}
    return {"path": str(telemetry_path), "rows_total": rows_total,
            "rows_in_run_window": len(recent), "recent_excerpts": recent[:8]}


def check_toplevel_visibility(state_reply: Dict[str, Any], track_id: str,
                              plugin_id: str, plugin_name: str, rack_mode: bool) -> Dict[str, Any]:
    """Direct-mount visibility: the instantiated plugin must appear as a
    TOP-LEVEL track.plugins row. In rack mode the assertion is the plugin
    appearing inside track.rack.nodes[] (the seg3a_r2 shape) or as a plugin row
    whose name matches (rack shape is the bisection delta being reproduced)."""
    evidence: Dict[str, Any] = {"track_id": track_id, "want_plugin_id": plugin_id,
                                "rack_mode": rack_mode,
                                "top_level_plugin_names": [], "rack_node_names": [],
                                "matched_by": None, "ok": False}
    for track in rows_of(state_reply.get("tracks")):
        if str(track.get("track_id") or track.get("id") or "") != track_id:
            continue
        plugin_rows = rows_of(track.get("plugins"))
        evidence["top_level_plugin_names"] = [str(row.get("name", "")) for row in plugin_rows]
        for row in plugin_rows:
            row_id = str(row.get("plugin_id") or row.get("id") or row.get("item_id") or "")
            if row_id and row_id == plugin_id:
                evidence["matched_by"] = "id"
                break
        if evidence["matched_by"] is None:
            for row in plugin_rows:
                if plugin_name and str(row.get("name", "")) == plugin_name:
                    evidence["matched_by"] = "name"
                    break
        if evidence["matched_by"] is None and rack_mode:
            for node in rows_of(track.get("rack", {}).get("nodes") if isinstance(track.get("rack"), dict) else []):
                evidence["rack_node_names"].append(str(node.get("name", node.get("plugin_name", ""))))
                node_id = str(node.get("plugin_id") or node.get("id") or node.get("node_id") or "")
                if (node_id and node_id == plugin_id) or (plugin_name and str(node.get("name", node.get("plugin_name", ""))) == plugin_name):
                    evidence["matched_by"] = "rack_node"
                    break
        break
    evidence["ok"] = evidence["matched_by"] is not None
    return evidence


def main() -> int:
    parser = argparse.ArgumentParser(description="WAVESHELL-OFFRENDER-FIX-1 (309) stack-leg driver (M1 realtime / render legs with fix-B + form-3 disclosure capture)")
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--kernel-sub", default="tcp://127.0.0.1:5556")
    parser.add_argument("--leg-mode", required=True, choices=["render", "realtime"])
    parser.add_argument("--mount-mode", default="direct", choices=["direct", "rack"],
                        help="render leg only: direct track mount (301 M2) or rack node mount (seg3a_r2/M0a2 shape)")
    parser.add_argument("--plugin-select", required=True, choices=sorted(PLUGIN_SELECTORS))
    parser.add_argument("--project-path", required=True, help="fixture project save path (run artifacts dir)")
    parser.add_argument("--render-path", required=False, help="render output wav path (render leg)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--kernel-log-dir", required=True,
                        help="kernel Workspace/Logs dir (VitHeadlessServer_*.log trace source)")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl (zero-LLM evidence)")
    parser.add_argument("--expect-env", default="",
                        help="comma-separated VIT309_* env flags expected on the kernel (receipt record)")
    args = parser.parse_args()

    run_started_epoch = time.time()
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "WAVESHELL-OFFRENDER-FIX-1",
        "leg": f"{args.leg_mode}/{args.mount_mode}: {PLUGIN_SELECTORS[args.plugin_select]['label']} (zero-LLM)",
        "stamp": stamp,
        "expect_env": [flag for flag in args.expect_env.split(",") if flag],
        "phases": {},
    }

    def save_summary() -> None:
        Path(args.output).write_text(
            json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def fail(exit_code: int, phase: str, reason: str, extra: Optional[Dict[str, Any]] = None) -> int:
        summary["exit_code"] = exit_code
        summary["exit_class"] = {
            EXIT_CHAIN_COMPLETE: "chain_complete",
            EXIT_ENV: "environment",
            EXIT_CMD_DEFECT: "command_defect",
        }.get(exit_code, "unknown")
        summary["fail_phase"] = phase
        summary["fail_reason"] = reason
        if extra:
            summary["phases"].setdefault(phase, {}).update(extra)
        save_summary()
        print(f"[waveshell-stackleg] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # ---- Phase A: precheck + fixture ----
    try:
        wait_kernel_zmq(args.kernel_zmq, 30.0)
        listing = zmq_command(args.kernel_zmq, {"cmd": "plugin_list_available", "limit": 2000, "command_timeout_ms": 60000})
        if str(listing.get("status", "")) != "ok":
            return fail(EXIT_ENV, "precheck", f"plugin_list_available failed: {json.dumps(listing, ensure_ascii=False)[:800]}")
        plugin_rows = rows_of(listing.get("plugins"))
        instrument_rows = [row for row in plugin_rows if bool(row.get("is_instrument"))]
        needle = PLUGIN_SELECTORS[args.plugin_select]["name_contains"].lower()
        selected = [row for row in instrument_rows if needle in str(row.get("name", "")).lower()]
        summary["phases"]["precheck"] = {
            "plugin_count": len(plugin_rows),
            "instrument_count": len(instrument_rows),
            "selector": args.plugin_select,
            "matched": [{"name": str(r.get("name", "")), "identifier": str(r.get("identifier", ""))}
                        for r in selected[:5]],
        }
        save_summary()
        if not selected:
            return fail(EXIT_ENV, "precheck",
                        f"requested plugin {args.plugin_select!r} ({needle!r}) not in local instrument inventory "
                        f"(instruments={len(instrument_rows)}) — environment precondition hand-over")

        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(Path(args.project_path).resolve()), "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"save_as_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "add_track", "command_timeout_ms": 60000})
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"add_track failed: {reply}")
        track_id = str(reply.get("track_id") or "")
        if not track_id:
            return fail(EXIT_ENV, "fixture", f"add_track returned no track_id: {reply}")
    except Exception as exc:  # noqa: BLE001 - env-class catch for the setup phase.
        return fail(EXIT_ENV, "fixture", f"setup exception: {exc}")

    summary["phases"]["fixture"] = {"track_id": track_id, "project_path": str(Path(args.project_path).resolve())}
    save_summary()
    print(f"[waveshell-stackleg] precheck ok: instruments={len(instrument_rows)}, selected={selected[0].get('name')!r}; "
          f"fixture track={track_id}", flush=True)

    # ---- Phase B: mount (direct 301-M2 shape or rack seg3a_r2 shape) ----
    chosen = selected[0]
    plugin_identifier = str(chosen.get("identifier") or "")
    rack_mode = (args.mount_mode == "rack") and (args.leg_mode == "render")
    try:
        if rack_mode:
            reply = zmq_command(args.kernel_zmq, {
                "cmd": "rack_add_node", "track_id": track_id,
                "plugin_identifier": plugin_identifier,
                "x": 300, "y": 200, "auto_connect": True,
                "command_timeout_ms": 120000,
            })
        else:
            reply = zmq_command(args.kernel_zmq, {
                "cmd": "instantiate_plugin", "track_id": track_id,
                "plugin_identifier": plugin_identifier, "command_timeout_ms": 120000,
            })
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "instantiate", f"mount command exception: {exc}")
    mount_phase: Dict[str, Any] = {
        "cmd": "rack_add_node" if rack_mode else "instantiate_plugin",
        "status": reply.get("status"),
        "plugin_name": reply.get("plugin_name"),
        "plugin_id": reply.get("plugin_id") or reply.get("node_id"),
        "plugin_identifier": reply.get("plugin_identifier"),
    }
    summary["phases"]["instantiate"] = mount_phase
    save_summary()
    if str(reply.get("status", "")) != "ok":
        return fail(EXIT_CMD_DEFECT, "instantiate",
                    f"mount status={reply.get('status')!r}", mount_phase)
    plugin_id = str(reply.get("plugin_id") or reply.get("node_id") or "")
    plugin_name = str(reply.get("plugin_name") or "")
    if not plugin_id and not plugin_name:
        return fail(EXIT_CMD_DEFECT, "instantiate", "mount ok but neither plugin_id nor plugin_name echoed", mount_phase)

    # allow the async external-plugin load a moment, then visibility check
    time.sleep(2.0)
    try:
        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        visibility = check_toplevel_visibility(state, track_id, plugin_id, plugin_name, rack_mode)
        summary["phases"]["instantiate"]["visibility"] = visibility
        save_summary()
        if not visibility["ok"]:
            return fail(EXIT_CMD_DEFECT, "instantiate",
                        f"mounted plugin not visible ({'rack nodes' if rack_mode else 'track.plugins top-level rows'}): {visibility}",
                        visibility)
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "instantiate", f"visibility check exception: {exc}")
    print(f"[waveshell-stackleg] mount ok ({args.mount_mode}): {plugin_name!r} plugin_id={plugin_id} "
          f"matched_by={visibility['matched_by']}", flush=True)

    # ---- Phase C: same clip shape as seg3a_r2 ----
    try:
        reply = zmq_command(args.kernel_zmq, {
            "cmd": "insert_midi_clip", "track_id": track_id,
            "time_unit": "beats", "start": 0.0, "length": CLIP_LENGTH_BEATS,
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "clip", f"insert_midi_clip failed: {reply}")
        clip_id = str(reply.get("clip_id") or reply.get("new_clip_id") or "")
        if not clip_id:
            return fail(EXIT_ENV, "clip", f"insert_midi_clip returned no clip_id: {reply}")
        notes = [{"pitch": pitch, "start": float(index), "length": 1.0, "velocity": 90}
                 for index, pitch in enumerate(ARPEGGIO)]
        reply = zmq_command(args.kernel_zmq, {
            "cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
            "time_unit": "beats", "notes": notes, "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "clip", f"add_midi_notes failed: {reply}")
        notes_reply = zmq_command(args.kernel_zmq, {"cmd": "get_midi_clip_notes", "clip_id": clip_id, "command_timeout_ms": 60000})
        readback = rows_of(notes_reply.get("notes"))
        summary["phases"]["clip"] = {"clip_id": clip_id, "note_count": len(ARPEGGIO),
                                     "readback_count": len(readback)}
        save_summary()
        if len(readback) != len(ARPEGGIO):
            return fail(EXIT_ENV, "clip", f"readback note count {len(readback)} != {len(ARPEGGIO)}")
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "chain", f"clip exception: {exc}")
    print(f"[waveshell-stackleg] clip ok: {clip_id} with {len(ARPEGGIO)} notes", flush=True)

    # ---- Phase D-E: leg-specific execution ----
    if args.leg_mode == "realtime":
        return run_realtime_leg(args, track_id, plugin_name, run_started_epoch, summary, save_summary, fail)
    return run_render_leg(args, track_id, plugin_name, run_started_epoch, summary, save_summary, fail)


def run_realtime_leg(args: argparse.Namespace, track_id: str, plugin_name: str,
                     run_started_epoch: float, summary: Dict[str, Any],
                     save_summary, fail) -> int:
    """E1/M1: realtime playback observation via the VIT309_RTLEVEL tap."""
    kernel_log_dir = Path(args.kernel_log_dir)

    def send(cmd: str, timeout_ms: int = 60000) -> Dict[str, Any]:
        return zmq_command(args.kernel_zmq, {"cmd": cmd, "command_timeout_ms": timeout_ms})

    try:
        rtz = send("return_to_zero")
        if str(rtz.get("status", "")) != "ok":
            return fail(EXIT_ENV, "realtime", f"return_to_zero failed: {rtz}")

        log_path = newest_kernel_log(kernel_log_dir)
        if log_path is None:
            return fail(EXIT_ENV, "realtime", f"no VitHeadlessServer log under {kernel_log_dir}")
        with open(log_path, "rb") as handle:
            handle.seek(0, 2)
            offset_before_play = handle.tell()

        play = send("play", 30000)
        if str(play.get("status", "")) != "ok":
            return fail(EXIT_ENV, "realtime", f"play failed: {play}")

        deadline = time.time() + RT_SAMPLE_SEC
        rtlevel_lines: List[str] = []
        while time.time() < deadline:
            time.sleep(0.5)
            text, offset_before_play = read_log_from(log_path, offset_before_play)
            for line in text.splitlines():
                if "VIT309_RTLEVEL" in line:
                    rtlevel_lines.append(line.strip())

        stop = send("stop")
        rtz2 = send("return_to_zero")

        # drain any final log lines written until stop
        time.sleep(1.0)
        text, _ = read_log_from(log_path, offset_before_play)
        for line in text.splitlines():
            if "VIT309_RTLEVEL" in line:
                rtlevel_lines.append(line.strip())
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "realtime", f"realtime leg exception: {exc}")

    samples: List[float] = []
    for line in rtlevel_lines:
        match = RTLEVEL_LINE_RE.search(line)
        if match:
            samples.extend([float(match.group(1)), float(match.group(2))])

    max_db = max(samples) if samples else None
    if max_db is None:
        verdict = "no_tap_lines"
    elif max_db > RT_SOUND_DB:
        verdict = "sound"
    elif max_db <= RT_SILENT_DB:
        verdict = "silent"
    else:
        verdict = "marginal"

    summary["phases"]["realtime"] = {
        "track_id": track_id,
        "plugin_name": plugin_name,
        "sample_seconds": RT_SAMPLE_SEC,
        "rtlevel_line_count": len(rtlevel_lines),
        "level_sample_count": len(samples),
        "max_db": max_db,
        "verdict": verdict,
        "stop_status": stop.get("status"),
        "rtz_after_status": rtz2.get("status"),
        "rtlevel_lines": rtlevel_lines[:120],
    }
    save_summary()

    if verdict == "no_tap_lines":
        return fail(EXIT_CMD_DEFECT, "realtime",
                    "play window produced zero VIT309_RTLEVEL lines (tap not running: kernel env VIT309_RTLEVEL "
                    "or instrumentation/build defect) — forensics hand-over",
                    summary["phases"]["realtime"])

    print(f"[waveshell-stackleg] REALTIME VERDICT: {verdict} (max_db={max_db}, "
          f"lines={len(rtlevel_lines)}, samples={len(samples)})", flush=True)

    # ---- trace capture + zero-LLM evidence ----
    trace = collect_vit_trace(Path(args.kernel_log_dir), run_started_epoch)
    summary["phases"]["vit_trace"] = trace
    trace_artifact = Path(args.output).with_name(Path(args.output).stem + "_vit_trace.txt")
    trace_artifact.write_text(
        "\n".join(f"[{row['log']}] {row['line']}" for row in trace.get("trace_lines", []))
        or "(no VIT301_GUARD/VIT309 lines in kernel logs scanned for this run window)",
        encoding="utf-8")
    summary["phases"]["vit_trace"]["artifact"] = str(trace_artifact)
    summary["phases"]["billing"] = telemetry_row_count(
        Path(args.telemetry_path) if args.telemetry_path else None, run_started_epoch)
    summary["exit_code"] = EXIT_CHAIN_COMPLETE
    summary["exit_class"] = "chain_complete"
    summary["realtime_verdict"] = verdict
    save_summary()
    return EXIT_CHAIN_COMPLETE


def run_render_leg(args: argparse.Namespace, track_id: str, plugin_name: str,
                   run_started_epoch: float, summary: Dict[str, Any],
                   save_summary, fail) -> int:
    """E2a/E2b/E3/E4/E5: offline render with fix-B/form-3 disclosure capture."""
    try:
        render_path = Path(args.render_path).resolve()
        if render_path.exists():
            render_path.unlink()
        reply = zmq_command(args.kernel_zmq, {
            "cmd": "start_render", "file_path": str(render_path), "bit_depth": 16,
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "render", f"start_render failed: {json.dumps(reply, ensure_ascii=False)[:600]}")
        job_id = str(reply.get("job_id") or "")
        start_reply = {
            "job_id": job_id,
            "shell_class_plugin_hint": reply.get("shell_class_plugin_hint"),
            "realtime_fallback": reply.get("realtime_fallback"),
        }
        outcome = wait_render_done(args.kernel_sub, render_path, job_id, RENDER_WAIT_SEC)
        if outcome.get("sub_signal") and str(outcome["sub_signal"]).startswith("render_failed"):
            return fail(EXIT_CMD_DEFECT, "render", f"render failed publication: {outcome.get('sub_signal')}", outcome)
        if not render_path.exists() or render_path.stat().st_size <= 44:
            return fail(EXIT_CMD_DEFECT, "render",
                        f"render product missing after wait (sub={outcome.get('sub_signal')}, fallback={outcome.get('fallback_file')})",
                        outcome)
        peak = wav_max_abs(render_path)
        render_silent = (peak.get("peak_abs", 0) <= NON_SILENCE_THRESHOLD
                         or peak.get("rms_abs", 0.0) <= NON_SILENCE_RMS_THRESHOLD)
        done_message = outcome.get("last_message") or {}
        render_phase = {
            "job_id": job_id,
            "start_reply": start_reply,
            "wav": str(render_path),
            "bytes": render_path.stat().st_size,
            "completion": {k: outcome.get(k) for k in ("sub_signal", "fallback_file")},
            "done_disclosure": {
                "degraded_to_realtime_render": done_message.get("degraded_to_realtime_render"),
                "render_silence_observed": done_message.get("render_silence_observed"),
                "status": done_message.get("status"),
            },
            "render_silent": render_silent,
            **peak,
        }
        summary["phases"]["render"] = render_phase
        save_summary()
        if "error" in peak:
            return fail(EXIT_ENV, "render", f"wav decode error: {peak.get('error')}", peak)
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "chain", f"render exception: {exc}")

    # ---- trace capture (VIT301 guard + VIT309 mode/blacklist/degraded) ----
    trace = collect_vit_trace(Path(args.kernel_log_dir), run_started_epoch)
    summary["phases"]["vit_trace"] = trace
    trace_artifact = Path(args.output).with_name(Path(args.output).stem + "_vit_trace.txt")
    trace_artifact.write_text(
        "\n".join(f"[{row['log']}] {row['line']}" for row in trace.get("trace_lines", []))
        or "(no VIT301_GUARD/VIT309 lines in kernel logs scanned for this run window)",
        encoding="utf-8")
    summary["phases"]["vit_trace"]["artifact"] = str(trace_artifact)
    save_summary()

    # ---- zero-LLM evidence ----
    summary["phases"]["billing"] = telemetry_row_count(
        Path(args.telemetry_path) if args.telemetry_path else None, run_started_epoch)
    save_summary()

    observation = ("SILENT" if render_silent else "NON-SILENT")
    print(f"[waveshell-stackleg] CHAIN COMPLETE: {plugin_name} {args.mount_mode}-mount render => {observation} "
          f"peak_abs={peak.get('peak_abs')} ({peak.get('peak_dbfs')} dBFS); "
          f"fixB hint={start_reply.get('shell_class_plugin_hint')} fallback={start_reply.get('realtime_fallback')}; "
          f"form3 degraded={render_phase['done_disclosure']['degraded_to_realtime_render']} "
          f"silence_observed={render_phase['done_disclosure']['render_silence_observed']}; "
          f"trace_lines={len(trace.get('trace_lines', []))}", flush=True)
    summary["exit_code"] = EXIT_CHAIN_COMPLETE
    summary["exit_class"] = "chain_complete"
    summary["observation"] = observation
    save_summary()
    return EXIT_CHAIN_COMPLETE


if __name__ == "__main__":
    sys.exit(main())
