#!/usr/bin/env python3
"""Smoke-test the VSP Phase 2 kernel reference over the existing ZMQ endpoint."""

from __future__ import annotations

import argparse
import json
import sys
import uuid
from datetime import datetime, timezone
from typing import Any

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


CLIENT_ID = "smoke.vsp.phase2"


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid.uuid4().hex}"


def envelope(
    *,
    channel: str,
    message_type: str,
    schema: str,
    payload: dict[str, Any],
    session_id: str = "session_pending",
    request_id: str | None = None,
    transaction_id: str | None = None,
    timeout_ms: int = 60000,
) -> dict[str, Any]:
    msg: dict[str, Any] = {
        "vsp_version": "1.0",
        "schema": schema,
        "message_id": new_id("msg"),
        "session_id": session_id,
        "client_id": CLIENT_ID,
        "role": "test",
        "channel": channel,
        "type": message_type,
        "created_at": utc_now(),
        "trace_id": new_id("trace"),
        "payload": payload,
        "command_timeout_ms": timeout_ms,
    }
    if request_id:
        msg["request_id"] = request_id
    if transaction_id:
        msg["transaction_id"] = transaction_id
    return msg


def send(socket: Any, payload: dict[str, Any]) -> dict[str, Any]:
    socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = socket.recv_string()
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"reply was not JSON: {raw[:500]}") from exc
    if not isinstance(parsed, dict):
        raise AssertionError(f"reply was not a JSON object: {type(parsed)!r}")
    return parsed


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def require_old_ok(reply: dict[str, Any], label: str) -> None:
    require(reply.get("status") == "ok", f"{label}: expected legacy status ok, got {reply!r}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=60000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    socket = ctx.socket(zmq.REQ)
    socket.setsockopt(zmq.RCVTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.SNDTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect(args.url)

    summary: dict[str, Any] = {
        "schema_version": "vsp_phase2_smoke.v1",
        "url": args.url,
        "checks": [],
    }

    try:
        direct_ping = send(socket, {"cmd": "ping"})
        require_old_ok(direct_ping, "direct ping")
        summary["checks"].append("legacy_direct_ping")

        direct_tracks = send(socket, {"cmd": "list_tracks"})
        require_old_ok(direct_tracks, "direct list_tracks")
        require(isinstance(direct_tracks.get("tracks"), list), "direct list_tracks: tracks must be an array")
        summary["checks"].append("legacy_direct_list_tracks")

        hello = send(
            socket,
            envelope(
                channel="session",
                message_type="session.hello",
                schema="vsp.session.hello.v1",
                payload={
                    "client_name": "VSP Phase 2 smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["legacy.command", "project.read"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: expected session.hello_ack, got {hello!r}")
        require("legacy.command" in (hello.get("capabilities") or []), "hello: legacy.command capability missing")
        require((hello.get("feature_flags") or {}).get("legacy.ipc_adapter") is True, "hello: legacy.ipc_adapter flag missing")
        require((hello.get("feature_flags") or {}).get("command.idempotency") is True, "hello: command.idempotency flag missing")
        require((hello.get("feature_flags") or {}).get("command.base_revision_cas") is True, "hello: command.base_revision_cas flag missing")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, "hello: session_id missing")
        summary["session_id"] = session_id
        summary["checks"].append("vsp_session_hello")

        state_snapshot = send(
            socket,
            envelope(
                channel="state",
                message_type="state.snapshot_request",
                schema="vsp.state.snapshot_request.v1",
                session_id=session_id,
                request_id=new_id("req_state"),
                payload={"scope": "project.timeline"},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(state_snapshot.get("type") == "state.snapshot", f"state snapshot: bad type {state_snapshot!r}")
        base_revision = int(state_snapshot.get("revision") or 0)
        project_epoch = str(state_snapshot.get("project_epoch") or "").strip()
        require(base_revision > 0 and project_epoch, f"state snapshot missing revision/epoch: {state_snapshot!r}")
        summary["base_revision"] = base_revision
        summary["project_epoch"] = project_epoch
        summary["checks"].append("vsp_state_snapshot_barrier")

        stale_stop = send(
            socket,
            envelope(
                channel="command",
                message_type="command.request",
                schema="vsp.command.request.v1",
                session_id=session_id,
                request_id=new_id("req_stale_cut"),
                transaction_id=new_id("tx_stale_cut"),
                payload={"command": "transport.stop", "args": {"base_revision": base_revision + 1}},
                timeout_ms=args.timeout_ms,
            ),
        )
        stale_error = stale_stop.get("error") or {}
        require(stale_stop.get("type") == "command.error", f"stale cut: expected command.error, got {stale_stop!r}")
        require(stale_error.get("code") == "stale_project_cut", f"stale cut: wrong error {stale_stop!r}")
        summary["checks"].append("vsp_stale_project_cut_rejected")

        # A repeated write request with the same session_id/request_id must
        # return the original receipt without invoking the legacy handler a
        # second time. transport.stop is intentionally used as a harmless
        # write-like command for this protocol check.
        idempotency_request_id = new_id("req_idempotent")
        first_stop = send(
            socket,
            envelope(
                channel="command",
                message_type="command.request",
                schema="vsp.command.request.v1",
                session_id=session_id,
                request_id=idempotency_request_id,
                transaction_id="tx_" + idempotency_request_id,
                payload={"command": "transport.stop", "args": {"base_revision": base_revision}},
                timeout_ms=args.timeout_ms,
            ),
        )
        second_stop = send(
            socket,
            envelope(
                channel="command",
                message_type="command.request",
                schema="vsp.command.request.v1",
                session_id=session_id,
                request_id=idempotency_request_id,
                transaction_id="tx_" + idempotency_request_id,
                payload={"command": "transport.stop", "args": {"base_revision": base_revision}},
                timeout_ms=args.timeout_ms,
            ),
        )
        require(first_stop.get("type") == "command.response", f"idempotent first stop: bad type {first_stop!r}")
        require(second_stop.get("type") == "command.response", f"idempotent second stop: bad type {second_stop!r}")
        require(second_stop.get("payload") == first_stop.get("payload"), "idempotent retry returned a different payload")
        summary["checks"].append("vsp_command_idempotency_retry")

        legacy_ping = send(
            socket,
            envelope(
                channel="command",
                message_type="command.request",
                schema="vsp.command.request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={
                    "command": "legacy.command",
                    "legacy": {
                        "cmd": "ping",
                        "args": {},
                    },
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(legacy_ping.get("type") == "command.response", f"legacy.command ping: bad type {legacy_ping!r}")
        lp = legacy_ping.get("payload") or {}
        require(lp.get("legacy_command") == "ping", f"legacy.command ping: bad legacy command {lp!r}")
        require((lp.get("legacy_reply") or {}).get("status") == "ok", f"legacy.command ping: legacy reply not ok {lp!r}")
        summary["checks"].append("vsp_legacy_command_ping")

        snapshot = send(
            socket,
            envelope(
                channel="command",
                message_type="command.request",
                schema="vsp.command.request.v1",
                session_id=session_id,
                request_id=new_id("req"),
                payload={
                    "command": "project.snapshot.get",
                    "args": {},
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(snapshot.get("type") == "command.response", f"project.snapshot.get: bad type {snapshot!r}")
        sp = snapshot.get("payload") or {}
        legacy_reply = sp.get("legacy_reply") or {}
        require(sp.get("legacy_command") == "get_project_state", f"project.snapshot.get: bad legacy command {sp!r}")
        require(legacy_reply.get("status") == "ok", f"project.snapshot.get: legacy reply not ok {legacy_reply!r}")
        require(isinstance(legacy_reply.get("tracks"), list), "project.snapshot.get: tracks must be an array")
        summary["checks"].append("vsp_canonical_project_snapshot_get")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        socket.close(0)


if __name__ == "__main__":
    main()
