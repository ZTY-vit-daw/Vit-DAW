#!/usr/bin/env python3
"""ARR-SRCLOAD-1 (card 277) segment-2 real-stack smoke driver, 2026-11-07.

Chat-driven instrument-load leg ("load an instrument source onto a MIDI
track and make it sound") — the first real-stack verification of the
arrangement load chain (card goal 1, A1-corrected conclusion landing):

  A  fixture + environment precheck (kernel ZMQ, deterministic, zero LLM):
     plugin_list_available -> is_instrument count > 0 (card stop condition:
     empty local VSTi inventory is an environment hand-over, never a code
     obstacle); new_project -> save_as_project -> add_track (exactly one
     user-visible track so the chat load target is unambiguous).
  B  one real chat turn (billed, default engine):
     POST /agent/chat with the arrangement phrasing
     「给这条 MIDI 轨加一个钢琴音源」(agent_mode=chat). The load is
     RiskConfirm-gated, so the expected shape is a pending plan
     (NeedsConfirmation + plan_id); the driver then approves it once via
     POST /agent/confirm. Route evidence (agent-loop capability round vs
     plugin_grabber workflow) is recorded honestly from the response, not
     asserted — both are legitimate chat-driven load channels.
  C  deterministic kernel-side chain:
     assert the executed rack_add_node row: status ok, zone_id == "Z2"
     (synth zone), plugin_instance_ready recorded; insert_midi_clip +
     add_midi_notes (16-beat C-major arpeggio) with readback; start_render
     (16-bit WAV in the run artifacts dir); wait for the kernel
     render/render_done publication (SUB 5556, file-poll fallback); assert
     the render product is non-silent (max |sample| above threshold).

Exit codes (card section 8, fixed before execution; single run, no rerun):
  0 PASS              load chain complete: instrument loaded Z2 via chat,
                      clip written, offline render non-silent
  2 environment       stack/transport/fixture failure, including the
                      is_instrument == 0 inventory precheck (environment
                      hand-over per card stop condition) — raw evidence saved
  3 llm_behavior_miss model proposed/executed no load (no plan_id, no
                      executed rack_add_node row; clarification ask or
                      unrelated action) — honest model-behavior record
  4 command_defect    load executed but the chain contradicts the design
                      (zone != Z2, load status error after confirm, render
                      silent/failing with the instrument present) —
                      forensics hand-over, no rerun

LLM budget: one chat turn plus at most a few in-turn roundtrips on the
user's default engine; telemetry rows for the conversation are summarized
into the run summary for billing evidence.
"""

from __future__ import annotations

import argparse
import json
import struct
import sys
import time
import urllib.error
import urllib.request
import wave
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

CHAT_TIMEOUT_SEC = 300.0
CLIP_LENGTH_BEATS = 16.0
RENDER_WAIT_SEC = 180.0
NON_SILENCE_THRESHOLD = 500  # max |sample| out of int16 full scale

# 16-beat C-major arpeggio fixture (C5 E5 G5 up/down, one note per beat).
ARPEGGIO: List[int] = [72, 76, 79, 84, 79, 76] + [72, 76, 79, 84, 79, 76] + [72, 74, 76, 79]

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_LLM_MISS = 3
EXIT_CMD_DEFECT = 4

LOAD_MESSAGE = "给这条 MIDI 轨加一个钢琴音源"


def post_json(base_url: str, path: str, payload: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {body[:800]}") from exc


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


def wait_agent_http(base_url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(base_url.rstrip("/") + "/health", timeout=2.0) as response:
                health = json.loads(response.read().decode("utf-8"))
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"agent HTTP did not become ready at {base_url}: {last_error}")


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


def tool_name_of(row: Dict[str, Any]) -> str:
    return str(row.get("tool") or row.get("command_name") or row.get("cmd") or "")


def find_executed_load(rows: List[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    for row in rows:
        if tool_name_of(row) in {"rack.add_node", "rack_add_node", "plugin.load_to_rack"}:
            return row
    return None


def collect_executed(response: Dict[str, Any]) -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    for key in ("executed_kernel_reply", "executed", "replies"):
        rows.extend(rows_of(response.get(key)))
    return rows


def wav_max_abs(path: Path) -> Dict[str, Any]:
    import math
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
    return {
        "channels": channels,
        "frames": frames,
        "peak_abs": peak,
        "peak_dbfs": dbfs,
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
                    # keep waiting briefly for the authoritative signal, but
                    # allow the fallback after the file is complete-looking
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


def parse_tool_calls(log_text: str) -> List[str]:
    import re
    tools: List[str] = []
    for match in re.finditer(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)", log_text):
        name = match.group(1)
        if name not in tools:
            tools.append(name)
    return tools


def telemetry_for_conversation(telemetry_path: Optional[Path], conversation: str) -> Dict[str, Any]:
    result: Dict[str, Any] = {"available": False}
    if not telemetry_path or not telemetry_path.exists():
        return result
    calls = 0
    input_tokens = 0
    output_tokens = 0
    cached_tokens = 0
    try:
        for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line or conversation not in line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            if not isinstance(row, dict):
                continue
            calls += 1
            usage = row.get("usage") if isinstance(row.get("usage"), dict) else {}
            input_tokens += int(usage.get("prompt_tokens") or row.get("prompt_tokens") or 0)
            output_tokens += int(usage.get("completion_tokens") or row.get("completion_tokens") or 0)
            cached = usage.get("prompt_cache_hit_tokens") or usage.get("prompt_tokens_details")
            if isinstance(cached, dict):
                cached = cached.get("cached_tokens")
            if isinstance(cached, (int, float)):
                cached_tokens += int(cached)
        result.update({
            "available": True,
            "calls": calls,
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "cached_input_tokens": cached_tokens,
        })
    except OSError as exc:
        result["error"] = str(exc)
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description="ARR-SRCLOAD-1 segment-2 chat-driven instrument load smoke")
    parser.add_argument("--agent-http", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--kernel-sub", default="tcp://127.0.0.1:5556")
    parser.add_argument("--project-path", required=True, help="fixture project save path (run artifacts dir)")
    parser.add_argument("--render-path", required=True, help="render output wav path (run artifacts dir)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--agent-log", default="", help="agent log path for harness.invoke route cross-check")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl (billing evidence)")
    parser.add_argument("--chat-timeout", type=float, default=CHAT_TIMEOUT_SEC)
    args = parser.parse_args()

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "ARR-SRCLOAD-1",
        "leg": "smokerun1 chat-driven instrument load -> clip -> non-silent render",
        "stamp": stamp,
        "phases": {},
    }

    def save_summary() -> None:
        Path(args.output).write_text(
            json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def fail(exit_code: int, phase: str, reason: str, extra: Optional[Dict[str, Any]] = None) -> int:
        summary["exit_code"] = exit_code
        summary["exit_class"] = {
            EXIT_PASS: "pass",
            EXIT_ENV: "environment",
            EXIT_LLM_MISS: "llm_behavior_miss",
            EXIT_CMD_DEFECT: "command_defect",
        }.get(exit_code, "unknown")
        summary["fail_phase"] = phase
        summary["fail_reason"] = reason
        if extra:
            summary["phases"].setdefault(phase, {}).update(extra)
        save_summary()
        print(f"[arr-srcload] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # ---- Phase A: environment precheck + fixture (kernel ZMQ, deterministic, zero LLM) ----
    track_id = ""
    try:
        wait_kernel_zmq(args.kernel_zmq, 30.0)
        wait_agent_http(args.agent_http, 60.0)

        listing = zmq_command(args.kernel_zmq, {"cmd": "plugin_list_available", "limit": 2000, "command_timeout_ms": 60000})
        if str(listing.get("status", "")) != "ok":
            return fail(EXIT_ENV, "precheck", f"plugin_list_available failed: {json.dumps(listing, ensure_ascii=False)[:800]}")
        plugin_rows = rows_of(listing.get("plugins"))
        instrument_rows = [row for row in plugin_rows if bool(row.get("is_instrument"))]
        summary["phases"]["precheck"] = {
            "plugin_count": len(plugin_rows),
            "instrument_count": len(instrument_rows),
            "instrument_names": [str(row.get("name", "")) for row in instrument_rows[:10]],
        }
        save_summary()
        if not instrument_rows:
            # Card stop condition: empty local VSTi inventory is an
            # environment hand-over, never a code obstacle.
            return fail(EXIT_ENV, "precheck",
                        "is_instrument count == 0 (local VSTi inventory empty) — environment precondition hand-over per card stop condition",
                        {"instrument_count": 0})

        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(Path(args.project_path).resolve()), "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"save_as_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "add_track", "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"add_track failed: {reply}")
        track_id = str(reply.get("track_id") or "")
        if not track_id:
            return fail(EXIT_ENV, "fixture", f"add_track returned no track_id: {reply}")
    except Exception as exc:  # noqa: BLE001 - env-class catch for the setup phase.
        return fail(EXIT_ENV, "fixture", f"setup exception: {exc}")

    summary["phases"]["fixture"] = {"track_id": track_id, "project_path": str(Path(args.project_path).resolve())}
    save_summary()
    print(f"[arr-srcload] precheck ok: instruments={len(instrument_rows)}; fixture track={track_id}", flush=True)

    # ---- Phase B: one real chat turn (billed) + confirmation ----
    agent_log_path = Path(args.agent_log) if args.agent_log else None
    log_size_before = agent_log_path.stat().st_size if (agent_log_path and agent_log_path.exists()) else 0

    conversation = f"arr_srcload_smoke_{stamp}"
    try:
        chat_response = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": conversation,
            "message": LOAD_MESSAGE,
            "context": {"agent_mode": "chat"},
        }, args.chat_timeout)
    except Exception as exc:  # noqa: BLE001 - env vs timeout classified below.
        return fail(EXIT_ENV, "chat", f"chat request exception: {exc}")

    Path(args.output).with_name("chat_response.json").write_text(
        json.dumps(chat_response, ensure_ascii=False, indent=2), encoding="utf-8")

    load_row = find_executed_load(collect_executed(chat_response))
    plan_id = str(chat_response.get("plan_id") or "")
    chat_phase: Dict[str, Any] = {
        "conversation_id": conversation,
        "stop_reason": chat_response.get("stop_reason"),
        "goal_status": chat_response.get("goal_status"),
        "needs_confirmation": bool(chat_response.get("needs_confirmation")),
        "plan_id": plan_id,
        "workflow": chat_response.get("workflow"),
        "workflow_plugin_kind": (chat_response.get("workflow_data") or {}).get("plugin_kind") if isinstance(chat_response.get("workflow_data"), dict) else None,
        "reply_excerpt": str(chat_response.get("reply") or "")[:400],
    }
    summary["phases"]["chat"] = chat_phase
    save_summary()

    confirm_response: Optional[Dict[str, Any]] = None
    if load_row is None and plan_id:
        try:
            confirm_response = post_json(args.agent_http, "/agent/confirm", {
                "conversation_id": conversation,
                "plan_id": plan_id,
                "decision": "approve",
            }, args.chat_timeout)
        except Exception as exc:  # noqa: BLE001
            return fail(EXIT_ENV, "confirm", f"confirm request exception: {exc}")
        Path(args.output).with_name("confirm_response.json").write_text(
            json.dumps(confirm_response, ensure_ascii=False, indent=2), encoding="utf-8")
        confirm_phase = {
            "status": confirm_response.get("status"),
            "goal_status": confirm_response.get("goal_status"),
            "message_excerpt": str(confirm_response.get("message") or "")[:300],
        }
        summary["phases"]["confirm"] = confirm_phase
        save_summary()
        load_row = find_executed_load(collect_executed(confirm_response))

    if agent_log_path and agent_log_path.exists():
        with agent_log_path.open("r", encoding="utf-8", errors="replace") as handle:
            handle.seek(log_size_before)
            summary["phases"]["chat"]["log_delta_tools"] = parse_tool_calls(handle.read())
        save_summary()

    if load_row is None:
        reason = (
            f"no rack_add_node executed row after chat+confirm; plan_id={plan_id or 'none'}; "
            f"needs_confirmation={chat_phase.get('needs_confirmation')}; "
            f"stop_reason={chat_phase.get('stop_reason')}; workflow={chat_phase.get('workflow')}"
        )
        return fail(EXIT_LLM_MISS, "chat", reason, chat_phase)

    # ---- Phase B2: load-chain assertions on the executed row ----
    result = load_row.get("result") if isinstance(load_row.get("result"), dict) else {}
    if not result:
        # some faces nest the kernel reply directly on the row
        result = {k: load_row.get(k) for k in ("status", "zone_id", "plugin_name", "plugin_id", "plugin_instance_ready", "plugin_load_state") if k in load_row}
    load_assert: Dict[str, Any] = {
        "tool": tool_name_of(load_row),
        "status": result.get("status"),
        "zone_id": result.get("zone_id"),
        "plugin_name": result.get("plugin_name"),
        "plugin_id": result.get("plugin_id"),
        "plugin_instance_ready": result.get("plugin_instance_ready"),
        "plugin_load_state": result.get("plugin_load_state"),
    }
    summary["phases"]["load"] = load_assert
    save_summary()

    if str(result.get("status") or "") != "ok":
        return fail(EXIT_CMD_DEFECT, "load", f"rack_add_node executed but status={result.get('status')!r}", load_assert)
    if str(result.get("zone_id") or "").upper() != "Z2":
        return fail(EXIT_CMD_DEFECT, "load",
                    f"instrument load zone_id={result.get('zone_id')!r}, want Z2 (synth zone)", load_assert)

    plugin_name = str(result.get("plugin_name") or "")
    print(f"[arr-srcload] load ok: {plugin_name} zone={result.get('zone_id')} "
          f"instance_ready={result.get('plugin_instance_ready')}", flush=True)

    # ---- Phase C: deterministic kernel chain — clip, render, non-silence ----
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
        if len(readback) != len(ARPEGGIO):
            return fail(EXIT_ENV, "clip", f"readback note count {len(readback)} != {len(ARPEGGIO)}")

        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        plugin_names_on_track: List[str] = []
        for track in rows_of(state.get("tracks")):
            if str(track.get("track_id") or track.get("id") or "") != track_id:
                continue
            for plugin_row in rows_of(track.get("plugins")):
                plugin_names_on_track.append(str(plugin_row.get("name") or ""))
        summary["phases"]["clip"] = {
            "clip_id": clip_id,
            "note_count": len(ARPEGGIO),
            "track_plugin_names": plugin_names_on_track,
        }
        save_summary()
        if plugin_name and plugin_name not in plugin_names_on_track:
            return fail(EXIT_CMD_DEFECT, "clip",
                        f"loaded plugin {plugin_name!r} not visible on track {track_id}: {plugin_names_on_track}",
                        {"track_plugin_names": plugin_names_on_track})

        render_path = Path(args.render_path).resolve()
        if render_path.exists():
            render_path.unlink()
        reply = zmq_command(args.kernel_zmq, {
            "cmd": "start_render", "file_path": str(render_path), "bit_depth": 16,
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            reason = str(reply.get("reason") or "")
            if reason == "no_renderable_audio_content":
                return fail(EXIT_CMD_DEFECT, "render",
                            "start_render rejected no_renderable_audio_content although instrument+clip present (route/instantiation defect?)",
                            {"reply": reply})
            return fail(EXIT_ENV, "render", f"start_render failed: {json.dumps(reply, ensure_ascii=False)[:600]}")
        job_id = str(reply.get("job_id") or "")
        outcome = wait_render_done(args.kernel_sub, render_path, job_id, RENDER_WAIT_SEC)
        if outcome.get("sub_signal") and str(outcome["sub_signal"]).startswith("render_failed"):
            return fail(EXIT_CMD_DEFECT, "render", f"render failed publication: {outcome.get('sub_signal')}", outcome)
        if not render_path.exists() or render_path.stat().st_size <= 44:
            return fail(EXIT_ENV, "render",
                        f"render product missing after wait (sub={outcome.get('sub_signal')}, fallback={outcome.get('fallback_file')})",
                        outcome)

        peak = wav_max_abs(render_path)
        summary["phases"]["render"] = {
            "job_id": job_id,
            "wav": str(render_path),
            "bytes": render_path.stat().st_size,
            "completion": outcome,
            **peak,
        }
        telemetry = telemetry_for_conversation(
            Path(args.telemetry_path) if args.telemetry_path else None, conversation)
        summary["phases"]["billing"] = telemetry
        save_summary()

        if "error" in peak:
            return fail(EXIT_ENV, "render", f"wav decode error: {peak.get('error')}", peak)
        if peak.get("peak_abs", 0) <= NON_SILENCE_THRESHOLD:
            return fail(EXIT_CMD_DEFECT, "render",
                        f"render product is silent: peak_abs={peak.get('peak_abs')} <= {NON_SILENCE_THRESHOLD} "
                        "(instrument loaded + clip present — silent render is a chain defect; forensics hand-over)",
                        peak)
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "chain", f"chain exception: {exc}")

    print(f"[arr-srcload] PASS: chat-loaded {plugin_name} (Z2) -> clip {len(ARPEGGIO)} notes -> "
          f"render peak_abs={peak.get('peak_abs')} ({peak.get('peak_dbfs')} dBFS)", flush=True)
    summary["exit_code"] = EXIT_PASS
    summary["exit_class"] = "pass"
    save_summary()
    return EXIT_PASS


if __name__ == "__main__":
    sys.exit(main())
