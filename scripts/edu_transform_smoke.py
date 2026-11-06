#!/usr/bin/env python3
"""EDU-SYMBOL-CORE-1 (card 276) segment-3 real-stack smoke driver, 2026-11-07.

Chat-driven verification leg for the analyze_midi_transformations command
face (segment-2 chain: catalog spec -> policy directCommands -> harness
invokeLocal -> edu command -> kernel read-only commands). One real LLM chat
turn must discover and call midi.analyze_transformations on a 42-note
Twinkle-Twinkle fixture imported through the kernel ZMQ face; the executed
reply carried back on /agent/chat (executed_kernel_reply rows) is asserted
against the segment-1 golden expectations.

Stack shape: same as the automation_llm_route family — kernel + agent berth
via dev_agent_smoke.ps1 -StartKernel -RestartAgent (no UI), isolated project
(VIT_PROJECT_XML run-dir copy set by the shared GUI-leg isolation block;
this driver's kernel-side new_project + save_as_project lands the fixture in
the run artifacts dir only, AGENTS.md section 10).

Phases:
  A  fixture setup (kernel ZMQ, deterministic, zero LLM):
     new_project -> save_as_project -> add_track -> insert_midi_clip
     (52 beats) -> add_midi_notes (42 notes) -> readback verification
     (get_project_state music_context bar-gate prerequisites +
     get_midi_clip_notes golden pitch/beat sequence + get_midi_clip_data
     start_seconds ~ 0). Any failure here is exit 2 (environment) — the
     LLM budget is never spent on a broken fixture.
  B  one chat round (real LLM, billed, user engine config):
     POST /agent/chat agent_mode=chat with an explicit instruction naming
     the tool, the clip_id and the key argument (instruction-shaped; the
     golden findings themselves are never leaked to the model).
  C  deterministic assertions on the response:
     executed row with tool=midi.analyze_transformations and result
     status ok; analyzed_note_count 42; phrase2->phrase3 diatonic sequence
     (proto beats 8-16, match 16-24, shift_semitones 2, diatonic_steps 1,
     label 模进（调内，上移 2 半音）, bars 3-4 -> 5-6); phrase3->phrase4
     repeat (proto 16-24, match 24-32, shift 0, label 原样重复, no
     sequence_quality, bars 5-6 -> 7-8).

Exit codes (card section 8, fixed before execution; single run, no rerun):
  0 PASS              chat-driven call happened and all golden assertions
                      held on the executed reply
  2 environment       stack/transport/fixture failure (raw evidence saved)
  3 llm_behavior_miss model never called the tool, turn failed/clarified,
                      or called it without the instructed key (the reply's
                      limitation text distinguishes the arg gap) — honest
                      model-behavior record, not a command-face verdict
  4 command_defect    tool executed with status ok (key present) but the
                      findings mismatch the golden expectations — segment-2
                      face defect, forensics hand-over (no rerun)

LLM budget: one chat turn (plus any bounded in-turn tool roundtrips) on the
user's default engine (deepseek-flash at run time); telemetry rows for the
conversation are summarized into the run summary for billing evidence.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

CHAT_TIMEOUT_SEC = 300.0
CLIP_LENGTH_BEATS = 52.0

# Segment-1 golden fixture: Twinkle Twinkle, C major, 4/4, six phrases
# a a' b b a a' (7 notes each, last note of every phrase is a half note).
PHRASES: List[List[int]] = [
    [60, 60, 67, 67, 69, 69, 67],
    [65, 65, 64, 64, 62, 62, 60],
    [67, 67, 65, 65, 64, 64, 62],
    [67, 67, 65, 65, 64, 64, 62],
    [60, 60, 67, 67, 69, 69, 67],
    [65, 65, 64, 64, 62, 62, 60],
]


def twinkle_rows() -> List[Dict[str, Any]]:
    rows: List[Dict[str, Any]] = []
    for pi, phrase in enumerate(PHRASES):
        for ni, pitch in enumerate(phrase):
            rows.append({
                "pitch": pitch,
                "start": float(pi * 8 + ni),
                "length": 1.0 if ni < len(phrase) - 1 else 2.0,
                "velocity": 80,
            })
    return rows


GOLDEN_PITCHES = [row["pitch"] for row in twinkle_rows()]
GOLDEN_STARTS = [row["start"] for row in twinkle_rows()]

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_LLM_MISS = 3
EXIT_CMD_DEFECT = 4

KEY_MISSING_LIMITATION = "未提供 key 参数"


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


def as_number(value: Any) -> Optional[float]:
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)):
        return float(value)
    return None


def approx(actual: Any, expected: float, tolerance: float = 1e-6) -> bool:
    number = as_number(actual)
    return number is not None and abs(number - expected) <= tolerance


def rows_of(value: Any) -> List[Dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    return []


def segment_of(finding: Dict[str, Any], role: str) -> Optional[Dict[str, Any]]:
    for seg in rows_of(finding.get("segments")):
        if seg.get("role") == role:
            return seg
    return None


def segment_start(finding: Dict[str, Any], role: str) -> Optional[float]:
    seg = segment_of(finding, role)
    if seg is None:
        return None
    return as_number(seg.get("start_beats"))


def tool_name_of(row: Dict[str, Any]) -> str:
    return str(row.get("tool") or row.get("command_name") or "")


def parse_tool_calls(log_text: str) -> List[str]:
    """Distinct tool names from harness.invoke timing lines (route evidence)."""
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
            blob = json.dumps(row, ensure_ascii=False)
            if f'"conversation_id": "{conversation}"' not in blob and f'"conversation_id":"{conversation}"' not in blob:
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
    parser = argparse.ArgumentParser(description="EDU-SYMBOL-CORE-1 segment-3 chat-driven smoke")
    parser.add_argument("--agent-http", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--project-path", required=True, help="fixture project save path (run artifacts dir)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--agent-log", default="", help="agent log path for harness.invoke route cross-check")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl (billing evidence)")
    parser.add_argument("--chat-timeout", type=float, default=CHAT_TIMEOUT_SEC)
    args = parser.parse_args()

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "EDU-SYMBOL-CORE-1",
        "leg": "smokerun1 chat-driven analyze_midi_transformations",
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
        print(f"[edu-transform] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # ---- Phase A: fixture setup (kernel ZMQ, deterministic, zero LLM) ----
    try:
        wait_kernel_zmq(args.kernel_zmq, 30.0)
        wait_agent_http(args.agent_http, 60.0)

        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        if str(reply.get("status", "")).lower() not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(Path(args.project_path).resolve()), "command_timeout_ms": 60000})
        if str(reply.get("status", "")).lower() not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"save_as_project failed: {reply}")

        reply = zmq_command(args.kernel_zmq, {"cmd": "add_track", "command_timeout_ms": 60000})
        if str(reply.get("status", "")) not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"add_track failed: {reply}")
        track_id = str(reply.get("track_id") or "")
        if not track_id:
            return fail(EXIT_ENV, "fixture", f"add_track returned no track_id: {reply}")

        reply = zmq_command(args.kernel_zmq, {
            "cmd": "insert_midi_clip", "track_id": track_id,
            "time_unit": "beats", "start": 0.0, "length": CLIP_LENGTH_BEATS,
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"insert_midi_clip failed: {reply}")
        clip_id = str(reply.get("clip_id") or reply.get("new_clip_id") or "")
        if not clip_id:
            return fail(EXIT_ENV, "fixture", f"insert_midi_clip returned no clip_id: {reply}")

        reply = zmq_command(args.kernel_zmq, {
            "cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
            "time_unit": "beats", "notes": twinkle_rows(),
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"add_midi_notes failed: {reply}")

        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        music = state.get("music_context") if isinstance(state.get("music_context"), dict) else {}
        if as_number(music.get("tempo_setting_count")) != 1.0:
            return fail(EXIT_ENV, "fixture",
                        f"bar-gate prerequisite missing: music_context={music}")
        if str(music.get("time_signature") or "") != "4/4":
            return fail(EXIT_ENV, "fixture",
                        f"bar-gate prerequisite missing (time sig): music_context={music}")

        clip_data = zmq_command(args.kernel_zmq, {"cmd": "get_midi_clip_data", "clip_id": clip_id, "command_timeout_ms": 60000})
        if str(clip_data.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"get_midi_clip_data failed: {clip_data}")
        start_seconds = as_number(clip_data.get("start_seconds"))
        if start_seconds is None or abs(start_seconds) > 1e-6:
            return fail(EXIT_ENV, "fixture", f"clip start_seconds expected ~0, got {start_seconds}")

        notes_reply = zmq_command(args.kernel_zmq, {"cmd": "get_midi_clip_notes", "clip_id": clip_id, "command_timeout_ms": 60000})
        notes = rows_of(notes_reply.get("notes"))
        if len(notes) != 42:
            return fail(EXIT_ENV, "fixture", f"readback note count {len(notes)} != 42")
        for i, note in enumerate(notes):
            if not approx(note.get("pitch"), GOLDEN_PITCHES[i]) or not approx(note.get("start"), GOLDEN_STARTS[i]):
                return fail(EXIT_ENV, "fixture",
                            f"readback note {i} mismatch: {note} (want pitch={GOLDEN_PITCHES[i]} start={GOLDEN_STARTS[i]})")
    except Exception as exc:  # noqa: BLE001 - env-class catch for the whole setup phase.
        return fail(EXIT_ENV, "fixture", f"setup exception: {exc}")

    summary["phases"]["fixture"] = {
        "track_id": track_id,
        "clip_id": clip_id,
        "project_path": str(Path(args.project_path).resolve()),
        "note_count": 42,
        "music_context": music,
        "verified": True,
    }
    save_summary()
    print(f"[edu-transform] fixture verified: track={track_id} clip={clip_id} notes=42", flush=True)

    # ---- Phase B: one real chat turn (billed) ----
    agent_log_path = Path(args.agent_log) if args.agent_log else None
    log_size_before = agent_log_path.stat().st_size if (agent_log_path and agent_log_path.exists()) else 0

    conversation = f"edu_transform_smoke_{stamp}"
    message = (
        f"当前工程里有一条 MIDI 片段（clip_id={clip_id}，共 42 个音符，是一段 C 大调旋律）。"
        "请调用工具 midi.analyze_transformations 分析这条片段里的变换关系："
        f"参数 clip_id 用 \"{clip_id}\"，key 传 {{\"tonic\": 0, \"mode\": \"major\"}}，"
        "range_beats / min_match_notes / transformations 都不用传（用默认值，分析全曲）。"
        "分析完成后把检出的每种关系（repeat / sequence 等）简要列给我。"
    )
    try:
        response = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": conversation,
            "message": message,
            "context": {"agent_mode": "chat"},
        }, args.chat_timeout)
    except Exception as exc:  # noqa: BLE001 - env vs timeout classified below.
        return fail(EXIT_ENV, "chat", f"chat request exception: {exc}")

    Path(args.output).with_name("chat_response.json").write_text(
        json.dumps(response, ensure_ascii=False, indent=2), encoding="utf-8")

    executed_rows = rows_of(response.get("executed_kernel_reply"))
    executed_tools = [tool_name_of(row) for row in executed_rows]
    analyze_rows = [row for row in executed_rows
                    if tool_name_of(row) in {"midi.analyze_transformations", "analyze_midi_transformations"}]

    log_delta_tools: List[str] = []
    if agent_log_path and agent_log_path.exists():
        with agent_log_path.open("r", encoding="utf-8", errors="replace") as handle:
            handle.seek(log_size_before)
            log_delta_tools = parse_tool_calls(handle.read())

    chat_phase = {
        "conversation_id": conversation,
        "stop_reason": response.get("stop_reason"),
        "goal_status": response.get("goal_status"),
        "reply_excerpt": str(response.get("reply") or "")[:400],
        "executed_tools": executed_tools,
        "log_delta_tools": log_delta_tools,
    }
    summary["phases"]["chat"] = chat_phase
    save_summary()

    if not analyze_rows:
        # Distinguish: model never called the tool (llm miss) vs turn-level
        # failure (still llm-behavior class for this leg; env only when the
        # stack itself errored, which post_json would have raised).
        reason = (
            f"no midi.analyze_transformations row in executed_kernel_reply; "
            f"executed={executed_tools or 'none'}; log_tools={log_delta_tools or 'none'}; "
            f"stop_reason={response.get('stop_reason')}"
        )
        return fail(EXIT_LLM_MISS, "chat", reason, chat_phase)

    analyze_row = analyze_rows[-1]
    result = analyze_row.get("result") if isinstance(analyze_row.get("result"), dict) else {}
    if str(result.get("status") or analyze_row.get("status") or "") != "ok":
        reason = f"analyze_midi_transformations row not ok: {json.dumps(analyze_row, ensure_ascii=False)[:1200]}"
        return fail(EXIT_LLM_MISS, "chat", reason, chat_phase)

    # ---- Phase C: golden assertions on the executed reply ----
    findings = rows_of(result.get("findings"))
    mismatches: List[str] = []

    if as_number(result.get("analyzed_note_count")) != 42.0:
        mismatches.append(f"analyzed_note_count={result.get('analyzed_note_count')} want 42")

    diatonic: Optional[Dict[str, Any]] = None
    repeat: Optional[Dict[str, Any]] = None
    for finding in findings:
        kind = str(finding.get("kind") or "")
        if kind == "sequence" and str(finding.get("sequence_quality") or "") == "diatonic" \
                and segment_start(finding, "prototype") == 8.0 and segment_start(finding, "match") == 16.0:
            diatonic = finding
        if kind == "repeat" \
                and segment_start(finding, "prototype") == 16.0 and segment_start(finding, "match") == 24.0:
            repeat = finding

    key_was_missing = any(KEY_MISSING_LIMITATION in str(item) for item in (result.get("limitations") or []))

    if diatonic is None:
        detail = f"diatonic sequence (proto 8 -> match 16) not found among {len(findings)} findings"
        if key_was_missing:
            mismatches.append(f"{detail}; model called without key (limitation disclosed)")
        else:
            mismatches.append(detail)
    else:
        if not approx(diatonic.get("shift_semitones"), 2.0):
            mismatches.append(f"diatonic shift_semitones={diatonic.get('shift_semitones')} want 2")
        if as_number(diatonic.get("diatonic_steps")) != 1.0:
            mismatches.append(f"diatonic diatonic_steps={diatonic.get('diatonic_steps')} want 1")
        if str(diatonic.get("label") or "") != "模进（调内，上移 2 半音）":
            mismatches.append(f"diatonic label={diatonic.get('label')!r} want '模进（调内，上移 2 半音）'")
        proto, match = segment_of(diatonic, "prototype"), segment_of(diatonic, "match")
        if proto is None or not approx(proto.get("bar_start"), 3) or not approx(proto.get("bar_end"), 4):
            mismatches.append(f"diatonic proto bars={None if proto is None else (proto.get('bar_start'), proto.get('bar_end'))} want 3-4")
        if match is None or not approx(match.get("bar_start"), 5) or not approx(match.get("bar_end"), 6):
            mismatches.append(f"diatonic match bars={None if match is None else (match.get('bar_start'), match.get('bar_end'))} want 5-6")

    if repeat is None:
        mismatches.append(f"repeat (proto 16 -> match 24) not found among {len(findings)} findings")
    else:
        if not approx(repeat.get("shift_semitones"), 0.0):
            mismatches.append(f"repeat shift_semitones={repeat.get('shift_semitones')} want 0")
        if "sequence_quality" in repeat:
            mismatches.append(f"repeat must not carry sequence_quality: {repeat.get('sequence_quality')}")
        if str(repeat.get("label") or "") != "原样重复":
            mismatches.append(f"repeat label={repeat.get('label')!r} want '原样重复'")
        proto, match = segment_of(repeat, "prototype"), segment_of(repeat, "match")
        if proto is None or not approx(proto.get("bar_start"), 5) or not approx(proto.get("bar_end"), 6):
            mismatches.append(f"repeat proto bars={None if proto is None else (proto.get('bar_start'), proto.get('bar_end'))} want 5-6")
        if match is None or not approx(match.get("bar_start"), 7) or not approx(match.get("bar_end"), 8):
            mismatches.append(f"repeat match bars={None if match is None else (match.get('bar_start'), match.get('bar_end'))} want 7-8")

    summary["phases"]["assertions"] = {
        "finding_count": len(findings),
        "diatonic_located": diatonic is not None,
        "repeat_located": repeat is not None,
        "key_missing_limitation": key_was_missing,
        "mismatches": mismatches,
    }

    telemetry = telemetry_for_conversation(
        Path(args.telemetry_path) if args.telemetry_path else None, conversation)
    summary["phases"]["billing"] = telemetry
    save_summary()

    if mismatches:
        exit_code = EXIT_LLM_MISS if (key_was_missing and diatonic is None) else EXIT_CMD_DEFECT
        return fail(exit_code, "assertions",
                    "; ".join(mismatches),
                    {"finding_count": len(findings), "key_missing_limitation": key_was_missing})

    print(f"[edu-transform] PASS: chat-driven analyze findings match golden "
          f"({len(findings)} findings; diatonic 8->16 shift2/steps1 bars 3-4->5-6; "
          f"repeat 16->24 shift0 bars 5-6->7-8)", flush=True)
    summary["exit_code"] = EXIT_PASS
    summary["exit_class"] = "pass"
    save_summary()
    return EXIT_PASS


if __name__ == "__main__":
    sys.exit(main())
