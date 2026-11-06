#!/usr/bin/env python3
"""EDU-LEVEL-RUNTIME-B (card 281) real-stack smoke driver, 2026-11-07.

Chat-driven verification leg for the edu jam deterministic mount face
(chat/edu_jam_chat.go + goalrunner_chat.go intercept): one minimal jam
session (enter -> n0 advance -> n1 judge -> status -> exit) driven over
POST /agent/chat against the real kernel + agent berth
(dev_agent_smoke.ps1 -StartKernel -RestartAgent, no UI).

The whole leg is DETERMINISTIC AND ZERO-LLM: the jam face intercepts before
semantic-entry classification, so no LLM call may be billed for the smoke
conversation (asserted against the telemetry jsonl when provided).

Stack shape: same as edu_transform_smoke.py — kernel + agent berth; the
kernel-side fixture (new_project + add_track x3 + theme clip) lands in the
run artifacts dir only (VIT_PROJECT_XML run-dir copy set by the shared
GUI-leg isolation block, AGENTS.md section 10).

Fixture notes: the 42-note Twinkle theme below is the byte-aligned twin of
coord/levels/first-run-twinkle level.yaml notes.theme (279 final fixture,
cross-checked by card 280 against the same 276 golden table). The n1 judge
pass in phase C is itself the fixture-vs-card equality assertion: the judge
compares the imported clip against the card-embedded notes table note by
note; any drift surfaces as an attempt_failed reply carrying the mismatch
row (exit 4, honest face-defect record).

Phases:
  A  fixture setup (kernel ZMQ, deterministic): new_project ->
     save_as_project -> add_track x3 -> insert_midi_clip on track 1 (52
     beats) -> add_midi_notes (42 theme notes) -> readback count check.
     Failure = exit 2 (environment).
  B  chat phases (HTTP, zero LLM expected):
     1 开始乐局      -> workflow edu_jam, level first-run-twinkle,
                        status active, current_node n0_photo_score_open
     2 写好了        -> n0 done (clip_exists + human_replied), n1 entered
     3 写好了        -> n1 judge note_sequence_match pass, n2 entered
                        (fixture == card theme table, see note above)
     4 乐局状态      -> rendered progress shows n0/n1 done and n2 current
     5 结束乐局      -> terminal exited, end_reason user_exit
  C  deterministic assertions on every response (see per-phase asserts)
     plus the zero-LLM telemetry assertion.

Exit codes (card section 8, fixed before execution; single run, no rerun):
  0 PASS              every phase asserted green on the chat face
  2 environment       stack/transport/fixture failure (raw evidence saved)
  4 face_defect       the deterministic jam face answered wrong (missed
                      intercept, wrong node transition, judge mismatch) —
                      forensics hand-over, no rerun

LLM budget: zero by design; the telemetry check asserts it stayed zero.
"""

from __future__ import annotations

import argparse
import json
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

CHAT_TIMEOUT_SEC = 60.0
CLIP_LENGTH_BEATS = 52.0

# 42-note Twinkle theme, C major, 4/4 (twin of level.yaml notes.theme /
# the 276 golden fixture). Last note of every 7-note phrase is a half note.
PHRASES: List[List[int]] = [
    [60, 60, 67, 67, 69, 69, 67],
    [65, 65, 64, 64, 62, 62, 60],
    [67, 67, 65, 65, 64, 64, 62],
    [67, 67, 65, 65, 64, 64, 62],
    [60, 60, 67, 67, 69, 69, 67],
    [65, 65, 64, 64, 62, 62, 60],
]


def theme_rows() -> List[Dict[str, Any]]:
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


EXIT_PASS = 0
EXIT_ENV = 2
EXIT_FACE_DEFECT = 4


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


def telemetry_rows_for(telemetry_path: Optional[Path], conversation: str) -> Optional[int]:
    """LLM call rows for the smoke conversation (None = telemetry unavailable)."""
    if not telemetry_path or not telemetry_path.exists():
        return None
    calls = 0
    try:
        for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line or conversation not in line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            if isinstance(row, dict) and conversation in json.dumps(row, ensure_ascii=False):
                calls += 1
    except OSError:
        return None
    return calls


def main() -> int:
    parser = argparse.ArgumentParser(description="EDU-LEVEL-RUNTIME-B jam mount smoke (deterministic, zero LLM)")
    parser.add_argument("--agent-http", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--project-path", required=True, help="fixture project save path (run artifacts dir)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl (zero-LLM assertion)")
    parser.add_argument("--chat-timeout", type=float, default=CHAT_TIMEOUT_SEC)
    args = parser.parse_args()

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "EDU-LEVEL-RUNTIME-B",
        "leg": "smokerun1 chat-driven jam mount (deterministic, zero LLM)",
        "stamp": stamp,
        "phases": {},
    }

    def save_summary() -> None:
        Path(args.output).write_text(
            json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def fail(exit_code: int, phase: str, reason: str, extra: Optional[Dict[str, Any]] = None) -> int:
        summary["exit_code"] = exit_code
        summary["exit_class"] = {EXIT_PASS: "pass", EXIT_ENV: "environment", EXIT_FACE_DEFECT: "face_defect"}.get(exit_code, "unknown")
        summary["fail_phase"] = phase
        summary["fail_reason"] = reason
        if extra:
            summary["phases"].setdefault(phase, {}).update(extra)
        save_summary()
        print(f"[edu-jam] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # ---- Phase A: fixture setup (kernel ZMQ, deterministic, zero LLM) ----
    track_id = clip_id = ""
    try:
        wait_kernel_zmq(args.kernel_zmq, 30.0)
        wait_agent_http(args.agent_http, 60.0)

        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        if str(reply.get("status", "")).lower() not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(Path(args.project_path).resolve()), "command_timeout_ms": 60000})
        if str(reply.get("status", "")).lower() not in {"ok", "success"}:
            return fail(EXIT_ENV, "fixture", f"save_as_project failed: {reply}")

        # Runtime slots declare tracks 1..3 (melody_imported / melody_corrupted /
        # accompaniment_blank); the minimal flow only needs track 1 populated.
        track_ids: List[str] = []
        for _ in range(3):
            reply = zmq_command(args.kernel_zmq, {"cmd": "add_track", "command_timeout_ms": 60000})
            if str(reply.get("status", "")) not in {"ok", "success"}:
                return fail(EXIT_ENV, "fixture", f"add_track failed: {reply}")
            row_id = str(reply.get("track_id") or "")
            if not row_id:
                return fail(EXIT_ENV, "fixture", f"add_track returned no track_id: {reply}")
            track_ids.append(row_id)
        track_id = track_ids[0]

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
            "time_unit": "beats", "notes": theme_rows(),
            "command_timeout_ms": 60000,
        })
        if str(reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"add_midi_notes failed: {reply}")

        notes_reply = zmq_command(args.kernel_zmq, {"cmd": "get_midi_clip_notes", "clip_id": clip_id, "command_timeout_ms": 60000})
        if str(notes_reply.get("status", "")) != "ok":
            return fail(EXIT_ENV, "fixture", f"get_midi_clip_notes failed: {notes_reply}")
        rows = notes_reply.get("notes")
        if not isinstance(rows, list) or len(rows) != 42:
            return fail(EXIT_ENV, "fixture", f"readback note count {len(rows) if isinstance(rows, list) else 'none'} != 42")
    except Exception as exc:  # noqa: BLE001 - env-class catch for the setup phase.
        return fail(EXIT_ENV, "fixture", f"setup exception: {exc}")

    summary["phases"]["fixture"] = {
        "track_ids": track_ids, "clip_id": clip_id,
        "project_path": str(Path(args.project_path).resolve()),
        "note_count": 42, "verified": True,
    }
    save_summary()
    print(f"[edu-jam] fixture verified: track={track_id} clip={clip_id} notes=42", flush=True)

    # ---- Phase B: chat-driven minimal jam session (zero LLM expected) ----
    conversation = f"edu_jam_smoke_{stamp}"
    chat_log: List[Dict[str, Any]] = []

    def chat(message: str) -> Dict[str, Any]:
        try:
            response = post_json(args.agent_http, "/agent/chat", {
                "conversation_id": conversation,
                "message": message,
                "context": {"agent_mode": "chat"},
            }, args.chat_timeout)
        except Exception as exc:  # noqa: BLE001 - env vs face classified by caller.
            raise RuntimeError(f"chat request exception ({message!r}): {exc}") from exc
        chat_log.append({"message": message, "response": response})
        Path(args.output).with_name("chat_responses.json").write_text(
            json.dumps(chat_log, ensure_ascii=False, indent=2), encoding="utf-8")
        return response

    def data_of(response: Dict[str, Any]) -> Dict[str, Any]:
        data = response.get("workflow_data")
        return data if isinstance(data, dict) else {}

    try:
        # B1 enter
        resp = chat("开始乐局")
        d = data_of(resp)
        if resp.get("workflow") != "edu_jam" or resp.get("stop_reason") != "edu_jam_entered":
            return fail(EXIT_FACE_DEFECT, "enter", f"enter not intercepted as jam face: {json.dumps(resp, ensure_ascii=False)[:800]}")
        if d.get("level_id") != "first-run-twinkle" or d.get("status") != "active" or d.get("current_node") != "n0_photo_score_open":
            return fail(EXIT_FACE_DEFECT, "enter", f"enter workflow_data wrong: {json.dumps(d, ensure_ascii=False)[:800]}")
        if "乐局开始" not in str(resp.get("reply") or ""):
            return fail(EXIT_FACE_DEFECT, "enter", f"enter reply missing title line: {resp.get('reply')}")

        # B2 n0 advance (clip_exists + human_replied)
        resp = chat("写好了")
        d = data_of(resp)
        if resp.get("stop_reason") != "edu_jam_node_done" or d.get("node_done") != "n0_photo_score_open":
            return fail(EXIT_FACE_DEFECT, "n0", f"n0 advance wrong: stop={resp.get('stop_reason')} data={json.dumps(d, ensure_ascii=False)[:800]}")
        if d.get("current_node") != "n1_read_and_fix":
            return fail(EXIT_FACE_DEFECT, "n0", f"n1 not entered after n0: {json.dumps(d, ensure_ascii=False)[:800]}")

        # B3 n1 judge pass (fixture == card notes.theme; mismatch = face defect)
        resp = chat("写好了")
        d = data_of(resp)
        if resp.get("stop_reason") != "edu_jam_node_done" or d.get("node_done") != "n1_read_and_fix":
            reply = str(resp.get("reply") or "")
            return fail(EXIT_FACE_DEFECT, "n1_judge",
                        f"n1 judge did not pass (fixture vs card drift or face defect): stop={resp.get('stop_reason')} reply={reply[:600]}")
        if d.get("current_node") != "n2_set_tempo":
            return fail(EXIT_FACE_DEFECT, "n1_judge", f"n2 not entered after n1: {json.dumps(d, ensure_ascii=False)[:800]}")

        # B4 status rendering
        resp = chat("乐局状态")
        reply = str(resp.get("reply") or "")
        if resp.get("stop_reason") != "edu_jam_status":
            return fail(EXIT_FACE_DEFECT, "status", f"status stop={resp.get('stop_reason')}")
        for needle in ["拍谱开局", "读谱纠错", "定速度", "进行中"]:
            if needle not in reply:
                return fail(EXIT_FACE_DEFECT, "status", f"status reply missing {needle!r}: {reply[:600]}")

        # B5 exit -> terminal
        resp = chat("结束乐局")
        d = data_of(resp)
        if resp.get("stop_reason") != "edu_jam_exited" or d.get("status") != "exited" or d.get("end_reason") != "user_exit":
            return fail(EXIT_FACE_DEFECT, "exit", f"exit wrong: stop={resp.get('stop_reason')} data={json.dumps(d, ensure_ascii=False)[:800]}")
    except RuntimeError as exc:
        return fail(EXIT_ENV, "chat", str(exc))

    summary["phases"]["chat"] = {
        "conversation_id": conversation,
        "turns": len(chat_log),
        "flow": ["开始乐局", "写好了(n0)", "写好了(n1 judge)", "乐局状态", "结束乐局"],
    }

    # ---- Phase C: zero-LLM assertion (deterministic intercept must not bill) ----
    llm_rows = telemetry_rows_for(Path(args.telemetry_path) if args.telemetry_path else None, conversation)
    summary["phases"]["zero_llm"] = {"telemetry_available": llm_rows is not None, "llm_rows": llm_rows}
    if llm_rows not in (None, 0):
        return fail(EXIT_FACE_DEFECT, "zero_llm",
                    f"jam intercept billed {llm_rows} LLM rows for a deterministic face (must be 0)")

    summary["exit_code"] = EXIT_PASS
    summary["exit_class"] = "pass"
    save_summary()
    print(f"[edu-jam] PASS: minimal jam flow green (enter -> n0 -> n1 judge -> status -> exit), zero LLM rows={llm_rows}", flush=True)
    return EXIT_PASS


if __name__ == "__main__":
    sys.exit(main())
