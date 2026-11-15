#!/usr/bin/env python3
"""Smoke-test the kernel time-signature command face (KERNEL-TIMESIG-1).

Legs (against the live kernel ZMQ REQ face, same transport pattern as
automation_curve_smoke.py — VSP hello then legacy command envelopes):

T1. new_project {"time_signature": "2/4"} (the W01 contract wire form, the
    lesson meter from the §13/§14 rulings) -> ok reply echoes 2/4 and the
    get_project_state music_context discloses numerator=2/denominator=4.
T2. set_time_signature {numerator:3, denominator:4} -> ok; music_context
    reads back 3/4.
T3. Fail-closed counter-examples on the live face: string numerator, zero
    numerator, denominator above 64 — every rejection must leave the meter
    untouched (music_context still 3/4 after each).
T4. Persistence round-trip through real kernel processes: save_project to a
    run-dir .vit file -> open_project the same file -> music_context still
    3/4.
T5. Zero-change default: new_project without a time_signature field -> 4/4
    (the blank-template behaviour every existing caller relies on).

Exit 0 iff every assertion holds. Deterministic leg: no LLM, no chat.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


def make_socket(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    sock = ctx.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    if hasattr(zmq, "REQ_RELAXED"):
        sock.setsockopt(zmq.REQ_RELAXED, 1)
    if hasattr(zmq, "REQ_CORRELATE"):
        sock.setsockopt(zmq.REQ_CORRELATE, 1)
    sock.connect(url)
    return sock


def send_command(sock: zmq.Socket, payload: dict[str, Any]) -> dict[str, Any]:
    cmd = payload.get("cmd")
    sock.send_string(json.dumps(payload, ensure_ascii=False))
    raw = sock.recv()
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8", errors="replace")
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON reply for {cmd}: {raw[:1000]!r}") from exc
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object reply for {cmd}: {reply!r}")
    return reply


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def vsp_envelope(
    session_id: str,
    payload: dict[str, Any],
    timeout_ms: int,
    *,
    channel: str = "command",
    message_type: str = "command.request",
    schema: str = "vsp.command.request.v1",
) -> dict[str, Any]:
    return {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": new_id("msg"),
        "session_id": session_id,
        "client_id": "smoke.timesig",
        "role": "test",
        "channel": channel,
        "type": message_type,
        "created_at": utc_now(),
        "trace_id": new_id("trace"),
        "request_id": new_id("req"),
        "transaction_id": new_id("tx"),
        "command_timeout_ms": timeout_ms,
        "payload": payload,
    }


def vsp_hello(sock: zmq.Socket, timeout_ms: int) -> str:
    reply = send_command(
        sock,
        {
            "vsp_version": "1.0",
            "schema": "vsp.session.hello.v1",
            "message_id": new_id("msg"),
            "session_id": "session_pending",
            "client_id": "smoke.timesig",
            "role": "test",
            "channel": "session",
            "type": "session.hello",
            "created_at": utc_now(),
            "trace_id": new_id("trace"),
            "payload": {
                "client_name": "Time signature smoke",
                "client_version": "0.1.0",
                "protocol_min": "1.0",
                "protocol_max": "1.0",
                "wants": ["command.request", "project.read"],
                "transport_bindings": ["legacy.zmq_reqrep"],
            },
            "command_timeout_ms": timeout_ms,
        },
    )
    if reply.get("type") != "session.hello_ack":
        raise RuntimeError(f"VSP hello failed: {reply}")
    session_id = str(reply.get("session_id") or "").strip()
    if not session_id:
        raise RuntimeError(f"VSP hello did not return session_id: {reply}")
    return session_id


def wait_for_kernel(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    deadline = time.time() + (timeout_ms / 1000.0)
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            sock = make_socket(ctx, url, timeout_ms=10_000)
            vsp_hello(sock, timeout_ms=10_000)
            sock.close(linger=0)
            return make_socket(ctx, url, timeout_ms)
        except Exception as exc:  # noqa: BLE001 - retry until deadline
            last_error = exc
            time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer on {url} before timeout: {last_error}")


def send_vsp_command(sock: zmq.Socket, session_id: str, command: str, args: dict[str, Any], timeout_ms: int) -> dict[str, Any]:
    # Project-lifecycle and meter commands are not in the kernel's canonical
    # VSP command map (that face covers automation./clip./marker. families);
    # they ride the legacy.command envelope instead — same path the agent
    # takes for set_tempo et al. Inner legacy replies come back as
    # command.response (ok) or command.error (kernel-level rejection), both
    # carrying payload.legacy_reply; the caller asserts on that inner reply.
    reply = send_command(
        sock,
        vsp_envelope(
            session_id,
            {"command": "legacy.command", "legacy": {"cmd": command, "args": args}},
            timeout_ms,
        ),
    )
    if reply.get("type") not in {"command.response", "command.error"}:
        raise RuntimeError(f"VSP {command} returned unexpected reply: {reply}")
    payload = reply.get("payload")
    if not isinstance(payload, dict):
        raise RuntimeError(f"VSP {command} payload missing: {reply}")
    if payload.get("legacy_command") != command:
        raise RuntimeError(f"VSP {command} mapped to unexpected legacy command: {payload}")
    legacy_reply = payload.get("legacy_reply")
    if not isinstance(legacy_reply, dict):
        raise RuntimeError(f"VSP {command} legacy_reply missing: {payload}")
    return legacy_reply


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_ok(label: str, reply: dict[str, Any]) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status in {"ok", "success", "completed"}, f"{label} failed: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def require_error(label: str, reply: dict[str, Any], message_pattern: str) -> dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    require(status == "error", f"{label} must be rejected fail-closed, got: {json.dumps(reply, ensure_ascii=False)}")
    message = str(reply.get("message") or "")
    require(message_pattern in message, f"{label} rejection reason mismatch: expected [{message_pattern}] got [{message}]")
    return reply


def music_context(sock: zmq.Socket, session_id: str, timeout_ms: int) -> dict[str, Any]:
    reply = require_ok("get_project_state", send_vsp_command(sock, session_id, "get_project_state", {}, timeout_ms))
    ctx = reply.get("music_context")
    require(isinstance(ctx, dict), f"music_context missing from get_project_state reply: {json.dumps(reply)[:2000]}")
    return ctx


def assert_meter(label: str, ctx: dict[str, Any], numerator: int, denominator: int) -> dict[str, Any]:
    row = {
        "time_signature": str(ctx.get("time_signature", "")),
        "numerator": ctx.get("time_signature_numerator"),
        "denominator": ctx.get("time_signature_denominator"),
    }
    require(
        row["time_signature"] == f"{numerator}/{denominator}",
        f"{label}: time_signature mismatch: expected {numerator}/{denominator} got {row['time_signature']!r}",
    )
    require(row["numerator"] == numerator, f"{label}: numerator mismatch: expected {numerator} got {row['numerator']!r}")
    require(row["denominator"] == denominator, f"{label}: denominator mismatch: expected {denominator} got {row['denominator']!r}")
    return row


def main() -> None:
    parser = argparse.ArgumentParser(description="KERNEL-TIMESIG-1 kernel time-signature smoke")
    parser.add_argument("--url", default="tcp://127.0.0.1:5555", help="kernel ZMQ REQ endpoint")
    parser.add_argument("--output", required=True, help="summary JSON artifact path")
    parser.add_argument("--project-file", default="", help="run-dir .vit path for the save/reopen round trip")
    parser.add_argument("--timeout-ms", type=int, default=30_000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    sock = wait_for_kernel(ctx, args.url, args.timeout_ms)
    session_id = vsp_hello(sock, args.timeout_ms)

    summary: dict[str, Any] = {
        "card": "KERNEL-TIMESIG-1",
        "leg": "timesig_smoke",
        "url": args.url,
        "started_at": utc_now(),
        "legs": {},
    }

    # T1 — creation-time application of the W01 wire form (lesson meter 2/4).
    reply = require_ok("new_project 2/4", send_vsp_command(sock, session_id, "new_project", {"time_signature": "2/4"}, args.timeout_ms))
    require(str(reply.get("time_signature", "")) == "2/4", f"T1 echo mismatch: {json.dumps(reply)[:1000]}")
    require(int(reply.get("time_signature_numerator", -1)) == 2, f"T1 echo numerator mismatch: {json.dumps(reply)[:1000]}")
    require(int(reply.get("time_signature_denominator", -1)) == 4, f"T1 echo denominator mismatch: {json.dumps(reply)[:1000]}")
    t1_row = assert_meter("T1 music_context", music_context(sock, session_id, args.timeout_ms), 2, 4)
    summary["legs"]["T1_create_2_4"] = {"echo": "2/4", "music_context": t1_row}
    print("[T1] new_project 2/4 ok — echo + music_context 2/4")

    # T2 — command-face set 3/4.
    reply = require_ok(
        "set_time_signature 3/4",
        send_vsp_command(sock, session_id, "set_time_signature", {"numerator": 3, "denominator": 4}, args.timeout_ms),
    )
    require(str(reply.get("time_signature", "")) == "3/4", f"T2 echo mismatch: {json.dumps(reply)[:1000]}")
    t2_row = assert_meter("T2 music_context", music_context(sock, session_id, args.timeout_ms), 3, 4)
    summary["legs"]["T2_set_3_4"] = {"echo": "3/4", "music_context": t2_row}
    print("[T2] set_time_signature 3/4 ok — music_context 3/4")

    # T3 — fail-closed counter-examples; meter must stay 3/4 throughout.
    # Each case pins the verbatim kernel rejection reason (field-type
    # rejections name the command + field; range rejections come from the
    # shared validator with the 1..64 domain).
    counter_examples = {
        "string_numerator": ({"numerator": "3", "denominator": 4}, "requires an integer numerator"),
        "zero_numerator": ({"numerator": 0, "denominator": 4}, "must be integers in 1..64"),
        "denominator_above_64": ({"numerator": 3, "denominator": 128}, "must be integers in 1..64"),
        "missing_denominator": ({"numerator": 3}, "requires an integer denominator"),
    }
    t3_rows: dict[str, Any] = {}
    for name, (payload, reason_pattern) in counter_examples.items():
        reply = send_vsp_command(sock, session_id, "set_time_signature", payload, args.timeout_ms)
        require_error(f"T3 {name}", reply, reason_pattern)
        row = assert_meter(f"T3 {name} untouched", music_context(sock, session_id, args.timeout_ms), 3, 4)
        t3_rows[name] = {"rejected": True, "message": str(reply.get("message") or ""), "music_context": row}
        print(f"[T3] {name} rejected fail-closed — meter still 3/4")
    summary["legs"]["T3_fail_closed"] = t3_rows

    # T4 — persistence round trip through real kernel save/open.
    require(args.project_file, "T4 requires --project-file (run-dir .vit path)")
    project_path = str(Path(args.project_file).resolve())
    require_ok("save_project", send_vsp_command(sock, session_id, "save_project", {"file_path": project_path}, args.timeout_ms))
    require(Path(project_path).is_file(), f"T4 saved project file missing: {project_path}")
    require_ok("open_project", send_vsp_command(sock, session_id, "open_project", {"file_path": project_path}, args.timeout_ms))
    t4_row = assert_meter("T4 after reopen", music_context(sock, session_id, args.timeout_ms), 3, 4)
    summary["legs"]["T4_save_reopen"] = {"project_file": project_path, "music_context": t4_row}
    print("[T4] save -> reopen ok — music_context 3/4 (round trip consistent)")

    # T5 — zero-change default: new_project without the field stays 4/4.
    require_ok("new_project default", send_vsp_command(sock, session_id, "new_project", {}, args.timeout_ms))
    t5_row = assert_meter("T5 music_context", music_context(sock, session_id, args.timeout_ms), 4, 4)
    summary["legs"]["T5_default_4_4"] = {"music_context": t5_row}
    print("[T5] new_project without field ok — music_context 4/4 (zero change)")

    summary["finished_at"] = utc_now()
    summary["result"] = "PASS"

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"VERDICT PASS (summary: {output_path})")


if __name__ == "__main__":
    main()
