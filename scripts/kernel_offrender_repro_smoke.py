#!/usr/bin/env python3
"""KERNEL-OFFRENDER-REPRO-1 (card 301) offline-render silence repro driver,
2026-11-07. Zero-LLM real-stack leg (M2 rack bisection / M3 plugin
specificity per the 297 forensics report section 3 design).

The 277 seg3a_r2 observation was: rack-mounted Grand Rhapsody VSTi +
16-note MIDI clip -> offline render completes (render_done, non-fallback)
-> 8.0 s product is digital silence (all 1,411,200 data bytes zero). The
297 report's only non-excludable candidate is the fail-silent guard pair
in te ExternalPlugin::applyToBuffer (hasLoadedInstance / isInstancePrepared
early returns). This driver reproduces the same edit shape WITHOUT the
rack container and WITHOUT any chat turn:

  A  precheck + fixture (kernel ZMQ, deterministic): ping kernel;
     plugin_list_available -> is_instrument rows -> select the requested
     plugin by name substring (--plugin-select grand_rhapsody|surge_xt);
     new_project -> save_as_project -> add_track (exactly one track).
  B  direct track mount: instantiate_plugin {track_id, plugin_identifier}
     (plugin goes straight into track->pluginList, NOT inside a rack
     container) -> assert status ok + plugin_id/plugin_name echo; assert
     visibility in get_project_state track.plugins TOP-LEVEL rows (direct
     mount shape; matched_by=id with name fallback).
  C  same clip shape as seg3a_r2: insert_midi_clip 16 beats +
     add_midi_notes 16-note C-major arpeggio (same fixture notes) +
     readback count.
  D  start_render (16-bit WAV in the run artifacts dir) -> wait for the
     render/render_done publication (SUB, file-poll fallback) -> physical
     metering (peak/rms).
  E  guard-trace capture: the kernel under test is the instrumented build
     (VIT301_GUARD one-shot traces at the two applyToBuffer guards); scan
     the kernel's newest VitHeadlessServer_*.log under --kernel-log-dir
     for VIT301_GUARD lines and copy them verbatim into the summary and a
     guard_trace artifact file.
  F  zero-LLM evidence: telemetry jsonl row count for this run window
     (recorded; not asserted here so the summary stays an observation).

Silence is NOT a failure exit for this leg: silent vs non-silent is the
discriminating observation (non-silent -> the defect is rack-path
specific / plugin-specific per leg; silent -> the defect is in the
common offline path). Both outcomes exit 0 with the observation recorded.

Exit codes (card section 8, fixed before execution; single run, no rerun):
  0 chain_complete    direct mount -> clip -> render product metered,
                      guard trace captured; silent/non-silent recorded
  2 environment       stack/transport/fixture failure, including the
                      requested plugin missing from the local inventory
  4 command_defect    deterministic chain defect: instantiate_plugin
                      status != ok, top-level visibility mismatch, render
                      failed publication, or no render product (forensics
                      hand-over, no rerun)
"""

from __future__ import annotations

import argparse
import json
import math
import struct
import sys
import time
import wave
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

CLIP_LENGTH_BEATS = 16.0
RENDER_WAIT_SEC = 180.0
NON_SILENCE_THRESHOLD = 500  # max |sample| out of int16 full scale
NON_SILENCE_RMS_THRESHOLD = 20.0  # same metering contract as arr_srcload

# 16-beat C-major arpeggio fixture (byte-identical to seg3a_r2).
ARPEGGIO: List[int] = [72, 76, 79, 84, 79, 76] + [72, 76, 79, 84, 79, 76] + [72, 74, 76, 79]

EXIT_CHAIN_COMPLETE = 0
EXIT_ENV = 2
EXIT_CMD_DEFECT = 4

GUARD_MARKER = "VIT301_GUARD"

PLUGIN_SELECTORS: Dict[str, Dict[str, str]] = {
    "grand_rhapsody": {"name_contains": "Grand Rhapsody",
                       "label": "Grand Rhapsody (seg3a_r2 same plugin, direct mount)"},
    "surge_xt": {"name_contains": "Surge XT",
                 "label": "Surge XT (plugin-specificity leg)"},
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
    """Primary: SUB render/render_done for this job. Fallback: file appears and is readable."""
    outcome: Dict[str, Any] = {"sub_signal": None, "fallback_file": False}
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


def collect_guard_trace(kernel_log_dir: Path, run_started_epoch: float) -> Dict[str, Any]:
    """Scan the kernel's VitHeadlessServer logs written since this run
    started for VIT301_GUARD one-shot trace lines; copy them verbatim."""
    result: Dict[str, Any] = {"log_dir": str(kernel_log_dir), "logs_scanned": [],
                              "guard_lines": [], "error": None}
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
                if GUARD_MARKER in line:
                    result["guard_lines"].append({"log": log_path.name, "line": line.strip()})
        except OSError as exc:
            result["logs_scanned"].append({"file": log_path.name, "error": str(exc)})
    return result


def telemetry_row_count(telemetry_path: Optional[Path], run_started_epoch: float) -> Dict[str, Any]:
    """Zero-LLM evidence for this leg: count telemetry jsonl rows whose
    timestamps fall in this run window (recorded, not asserted)."""
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
                              plugin_id: str, plugin_name: str) -> Dict[str, Any]:
    """Direct-mount visibility: the instantiated plugin must appear as a
    TOP-LEVEL track.plugins row (this is the bisection delta vs seg3a_r2,
    where the instrument lived INSIDE track.rack.nodes[])."""
    evidence: Dict[str, Any] = {"track_id": track_id, "want_plugin_id": plugin_id,
                                "top_level_plugin_names": [], "matched_by": None, "ok": False}
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
        break
    evidence["ok"] = evidence["matched_by"] is not None
    return evidence


def main() -> int:
    parser = argparse.ArgumentParser(description="KERNEL-OFFRENDER-REPRO-1 (301) zero-LLM offline-render silence repro leg")
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--kernel-sub", default="tcp://127.0.0.1:5556")
    parser.add_argument("--plugin-select", required=True, choices=sorted(PLUGIN_SELECTORS))
    parser.add_argument("--project-path", required=True, help="fixture project save path (run artifacts dir)")
    parser.add_argument("--render-path", required=True, help="render output wav path (run artifacts dir)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--kernel-log-dir", required=True,
                        help="kernel Workspace/Logs dir (VitHeadlessServer_*.log guard-trace source)")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl (zero-LLM evidence)")
    args = parser.parse_args()

    run_started_epoch = time.time()
    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "KERNEL-OFFRENDER-REPRO-1",
        "leg": PLUGIN_SELECTORS[args.plugin_select]["label"] + " (zero-LLM, direct track mount, no rack, no chat)",
        "stamp": stamp,
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
        print(f"[offrender-repro] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
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
    print(f"[offrender-repro] precheck ok: instruments={len(instrument_rows)}, selected={selected[0].get('name')!r}; "
          f"fixture track={track_id}", flush=True)

    # ---- Phase B: direct track mount via instantiate_plugin ----
    chosen = selected[0]
    plugin_identifier = str(chosen.get("identifier") or "")
    try:
        reply = zmq_command(args.kernel_zmq, {
            "cmd": "instantiate_plugin", "track_id": track_id,
            "plugin_identifier": plugin_identifier, "command_timeout_ms": 120000,
        })
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "instantiate", f"instantiate_plugin exception: {exc}")
    mount_phase: Dict[str, Any] = {
        "cmd": "instantiate_plugin",
        "status": reply.get("status"),
        "plugin_name": reply.get("plugin_name"),
        "plugin_id": reply.get("plugin_id"),
        "plugin_identifier": reply.get("plugin_identifier"),
        "template_role": reply.get("template_role"),
        "graph_last_diff_kind": reply.get("graph_last_diff_kind"),
    }
    summary["phases"]["instantiate"] = mount_phase
    save_summary()
    if str(reply.get("status", "")) != "ok":
        return fail(EXIT_CMD_DEFECT, "instantiate",
                    f"instantiate_plugin status={reply.get('status')!r}", mount_phase)
    plugin_id = str(reply.get("plugin_id") or "")
    plugin_name = str(reply.get("plugin_name") or "")
    if not plugin_id:
        return fail(EXIT_CMD_DEFECT, "instantiate", "instantiate_plugin ok but no plugin_id echoed", mount_phase)

    # allow the async external-plugin load a moment, then visibility check
    time.sleep(2.0)
    try:
        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        visibility = check_toplevel_visibility(state, track_id, plugin_id, plugin_name)
        summary["phases"]["instantiate"]["top_level_visibility"] = visibility
        save_summary()
        if not visibility["ok"]:
            return fail(EXIT_CMD_DEFECT, "instantiate",
                        f"direct-mounted plugin not visible in track.plugins top-level rows: {visibility}",
                        visibility)
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "instantiate", f"visibility check exception: {exc}")
    print(f"[offrender-repro] direct mount ok: {plugin_name!r} plugin_id={plugin_id} "
          f"top-level matched_by={visibility['matched_by']}", flush=True)

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
    print(f"[offrender-repro] clip ok: {clip_id} with {len(ARPEGGIO)} notes", flush=True)

    # ---- Phase D: render + metering ----
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
        summary["phases"]["render"] = {
            "job_id": job_id,
            "wav": str(render_path),
            "bytes": render_path.stat().st_size,
            "completion": outcome,
            "render_silent": render_silent,
            **peak,
        }
        save_summary()
        if "error" in peak:
            return fail(EXIT_ENV, "render", f"wav decode error: {peak.get('error')}", peak)
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "chain", f"render exception: {exc}")

    # ---- Phase E: guard-trace capture (instrumented kernel evidence) ----
    guard = collect_guard_trace(Path(args.kernel_log_dir), run_started_epoch)
    summary["phases"]["guard_trace"] = guard
    guard_artifact = Path(args.output).with_name(
        Path(args.output).stem + "_guard_trace.txt")
    guard_artifact.write_text(
        "\n".join(f"[{row['log']}] {row['line']}" for row in guard.get("guard_lines", []))
        or "(no VIT301_GUARD lines in kernel logs scanned for this run window)",
        encoding="utf-8")
    summary["phases"]["guard_trace"]["artifact"] = str(guard_artifact)
    save_summary()

    # ---- Phase F: zero-LLM evidence ----
    summary["phases"]["billing"] = telemetry_row_count(
        Path(args.telemetry_path) if args.telemetry_path else None, run_started_epoch)
    save_summary()

    observation = ("SILENT (digital silence reproduced on the direct-mount path — defect NOT rack-specific)"
                   if render_silent else
                   "NON-SILENT (direct-mount render sounds — the seg3a_r2 silence is specific to the rack/chat path)")
    print(f"[offrender-repro] CHAIN COMPLETE: {plugin_name} direct-mount -> clip {len(ARPEGGIO)} notes -> render "
          f"peak_abs={peak.get('peak_abs')} ({peak.get('peak_dbfs')} dBFS) rms_abs={peak.get('rms_abs')} "
          f"({peak.get('rms_dbfs')} dBFS) => {observation}; guard_lines={len(guard.get('guard_lines', []))}",
          flush=True)
    summary["exit_code"] = EXIT_CHAIN_COMPLETE
    summary["exit_class"] = "chain_complete"
    summary["observation"] = observation
    save_summary()
    return EXIT_CHAIN_COMPLETE


if __name__ == "__main__":
    sys.exit(main())
