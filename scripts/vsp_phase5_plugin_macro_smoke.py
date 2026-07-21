#!/usr/bin/env python3
"""Smoke-test VSP Phase 5 plugin batch, macro control, and command.batch."""

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


CLIENT_ID = "smoke.vsp.phase5.plugin_macro"


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
        "role": "agent",
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


def command_request(
    socket: Any,
    *,
    session_id: str,
    command: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="command",
            message_type="command.request",
            schema="vsp.command.request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            transaction_id=new_id("tx"),
            payload={"command": command, "args": args or {}},
            timeout_ms=timeout_ms,
        ),
    )


def legacy_command_request(
    socket: Any,
    *,
    session_id: str,
    cmd: str,
    args: dict[str, Any] | None = None,
    timeout_ms: int,
) -> dict[str, Any]:
    return send(
        socket,
        envelope(
            channel="command",
            message_type="command.request",
            schema="vsp.command.request.v1",
            session_id=session_id,
            request_id=new_id("req"),
            transaction_id=new_id("tx"),
            payload={"command": "legacy.command", "legacy": {"cmd": cmd, "args": args or {}}},
            timeout_ms=timeout_ms,
        ),
    )


def legacy_reply(reply: dict[str, Any]) -> dict[str, Any]:
    payload = reply.get("payload") or {}
    legacy = payload.get("legacy_reply") or {}
    require(isinstance(legacy, dict), f"legacy_reply missing object: {reply!r}")
    return legacy


def find_track_and_volume_plugin(state: dict[str, Any], track_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    for track in state.get("tracks") or []:
        if str(track.get("track_id") or track.get("id") or "").strip() != track_id:
            continue
        for plugin in track.get("plugins") or []:
            if str(plugin.get("type") or "").strip().lower() == "volume":
                return track, plugin
        raise AssertionError(f"created track has no volume plugin: {track!r}")
    raise AssertionError(f"created track not found in project state: {track_id}")


def require_changed_params(reply: dict[str, Any], expected_ids: set[str], label: str) -> None:
    payload = reply.get("payload") or {}
    changed = payload.get("changed_params") or []
    require(payload.get("status") == "ok", f"{label}: payload status not ok {reply!r}")
    seen = {str(item.get("parameter_id") or item.get("param_id") or "").strip() for item in changed}
    require(expected_ids <= seen, f"{label}: changed params missing {expected_ids - seen}: {reply!r}")
    for item in changed:
        if str(item.get("parameter_id") or item.get("param_id") or "").strip() in expected_ids:
            require("actual_normalized_value" in item or "actual_normalised_value" in item, f"{label}: no normalized readback {item!r}")
            require(item.get("display_text"), f"{label}: no display_text {item!r}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-ms", type=int, default=120000)
    args = parser.parse_args()

    ctx = zmq.Context.instance()
    socket = ctx.socket(zmq.REQ)
    socket.setsockopt(zmq.RCVTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.SNDTIMEO, args.timeout_ms)
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect(args.url)

    session_id = "session_pending"
    created_track_id = ""
    macro_node_id = ""
    summary: dict[str, Any] = {
        "schema_version": "vsp_phase5_plugin_macro_smoke.v1",
        "url": args.url,
        "checks": [],
    }

    try:
        legacy_state = send(socket, {"cmd": "get_project_state", "scope": "project.plugins"})
        require(legacy_state.get("status") == "ok", f"legacy get_project_state failed: {legacy_state!r}")
        summary["checks"].append("legacy_project_plugins_state_before")

        hello = send(
            socket,
            envelope(
                channel="session",
                message_type="session.hello",
                schema="vsp.session.hello.v1",
                payload={
                    "client_name": "VSP Phase 5 plugin/macro smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["command.batch", "plugin.control", "macro.control"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
                timeout_ms=args.timeout_ms,
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: bad type {hello!r}")
        flags = hello.get("feature_flags") or {}
        caps = set(hello.get("capabilities") or [])
        require(flags.get("command.batch") is True, f"hello: command.batch flag missing {hello!r}")
        require("command.batch" in caps, f"hello: command.batch capability missing {hello!r}")
        require("plugin.control" in caps, f"hello: plugin.control capability missing {hello!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, f"hello: session_id missing {hello!r}")
        summary["session_id"] = session_id
        summary["checks"].append("session_hello_batch_plugin_flags")

        create_track = command_request(
            socket,
            session_id=session_id,
            command="track.create",
            args={"track_name": "VSP Plugin Macro Smoke"},
            timeout_ms=args.timeout_ms,
        )
        require(create_track.get("type") == "command.response", f"track.create: bad type {create_track!r}")
        create_legacy = legacy_reply(create_track)
        require(create_legacy.get("status") == "ok", f"track.create: legacy failed {create_track!r}")
        created_track_id = str(create_legacy.get("track_id") or "").strip()
        require(created_track_id, f"track.create: no track_id {create_track!r}")
        summary["created_track_id"] = created_track_id
        summary["checks"].append("track_create_for_plugin_macro")

        project_state = send(socket, {"cmd": "get_project_state", "scope": "project.plugins"})
        require(project_state.get("status") == "ok", f"project state after track.create failed: {project_state!r}")
        _, volume_plugin = find_track_and_volume_plugin(project_state, created_track_id)
        volume_plugin_id = str(volume_plugin.get("id") or volume_plugin.get("plugin_id") or volume_plugin.get("plugin_item_id") or "").strip()
        require(volume_plugin_id, f"volume plugin id missing {volume_plugin!r}")
        summary["volume_plugin_id"] = volume_plugin_id
        summary["checks"].append("stable_volume_plugin_instance_id")

        plugin_batch = command_request(
            socket,
            session_id=session_id,
            command="plugin.set_params_batch",
            args={
                "track_id": created_track_id,
                "plugin_id": volume_plugin_id,
                "parameters": [
                    {"parameter_id": "pan", "normalized_value": 0.25},
                    {"parameter_id": "volume", "normalized_value": 0.70},
                ],
            },
            timeout_ms=args.timeout_ms,
        )
        require(plugin_batch.get("type") == "command.response", f"plugin.set_params_batch: bad type {plugin_batch!r}")
        require((plugin_batch.get("ack") or {}).get("stage") == "completed", f"plugin.set_params_batch: ack failed {plugin_batch!r}")
        require_changed_params(plugin_batch, {"pan", "volume"}, "plugin.set_params_batch")
        summary["checks"].append("plugin_set_params_batch_changed_params_readback")

        command_batch = command_request(
            socket,
            session_id=session_id,
            command="command.batch",
            args={
                "commands": [
                    {"command": "kernel.ping", "args": {}},
                    {
                        "command": "plugin.set_params_batch",
                        "args": {
                            "track_id": created_track_id,
                            "plugin_id": volume_plugin_id,
                            "parameters": [
                                {"parameter_id": "pan", "normalized_value": 0.50},
                                {"parameter_id": "volume", "normalized_value": 0.66},
                            ],
                        },
                    },
                ]
            },
            timeout_ms=args.timeout_ms,
        )
        require(command_batch.get("type") == "command.response", f"command.batch: bad type {command_batch!r}")
        batch_payload = command_batch.get("payload") or {}
        require(batch_payload.get("status") == "ok", f"command.batch: status not ok {command_batch!r}")
        require(batch_payload.get("ordered") is True, f"command.batch: ordered marker missing {command_batch!r}")
        require(int(batch_payload.get("completed_count") or 0) == 2, f"command.batch: completed count mismatch {command_batch!r}")
        results = batch_payload.get("results") or []
        require([item.get("command") for item in results] == ["kernel.ping", "plugin.set_params_batch"], f"command.batch: order mismatch {command_batch!r}")
        require_changed_params(results[1].get("response") or {}, {"pan", "volume"}, "command.batch child plugin.set_params_batch")
        summary["checks"].append("command_batch_ordered_plugin_child_results")

        macro_create = command_request(
            socket,
            session_id=session_id,
            command="macro.create",
            args={"name": "VSP Smoke Macro", "x": 12, "y": 24, "macro_count": 2},
            timeout_ms=args.timeout_ms,
        )
        require(macro_create.get("type") == "command.response", f"macro.create: bad type {macro_create!r}")
        macro_node = (legacy_reply(macro_create).get("node") or {})
        macro_node_id = str(macro_node.get("node_id") or "").strip()
        require(macro_node_id.startswith("macro_"), f"macro.create: unstable node id {macro_create!r}")
        summary["macro_node_id"] = macro_node_id
        summary["checks"].append("macro_create_stable_id")

        macro_bind = command_request(
            socket,
            session_id=session_id,
            command="macro.bind",
            args={
                "source_node_id": macro_node_id,
                "source_output": "macro_1",
                "target_kind": "plugin_param",
                "target_plugin_id": volume_plugin_id,
                "target_param_id": "volume",
                "min": 0.33,
                "max": 0.44,
            },
            timeout_ms=args.timeout_ms,
        )
        require(macro_bind.get("type") == "command.response", f"macro.bind: bad type {macro_bind!r}")
        binding = (legacy_reply(macro_bind).get("binding") or {})
        require(binding.get("source_node_id") == macro_node_id, f"macro.bind: wrong source {macro_bind!r}")
        require(str(binding.get("target_plugin_id") or "") == volume_plugin_id, f"macro.bind: wrong plugin target {macro_bind!r}")
        require(binding.get("target_param_id") == "volume", f"macro.bind: wrong param target {macro_bind!r}")
        summary["macro_binding_id"] = binding.get("binding_id")
        summary["checks"].append("macro_bind_target_verified")

        macro_set = command_request(
            socket,
            session_id=session_id,
            command="macro.set_values",
            args={"node_id": macro_node_id, "values": {"macro_1": 0.75}},
            timeout_ms=args.timeout_ms,
        )
        require(macro_set.get("type") == "command.response", f"macro.set_values: bad type {macro_set!r}")
        set_legacy = legacy_reply(macro_set)
        applied = set_legacy.get("applied_bindings") or []
        require(applied, f"macro.set_values: no applied bindings {macro_set!r}")
        first_applied = applied[0]
        require(str(first_applied.get("target_plugin_id") or "") == volume_plugin_id, f"macro.set_values: wrong applied target {macro_set!r}")
        require(first_applied.get("target_param_id") == "volume", f"macro.set_values: wrong applied param {macro_set!r}")
        summary["checks"].append("macro_set_values_binding_propagated")

        print(json.dumps(summary, ensure_ascii=False, indent=2))
    finally:
        if macro_node_id:
            try:
                legacy_command_request(
                    socket,
                    session_id=session_id,
                    cmd="control_remove_node",
                    args={"node_id": macro_node_id},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for macro {macro_node_id}: {cleanup_error}", file=sys.stderr)
        if created_track_id:
            try:
                command_request(
                    socket,
                    session_id=session_id,
                    command="track.delete",
                    args={"track_id": created_track_id},
                    timeout_ms=args.timeout_ms,
                )
            except Exception as cleanup_error:  # pragma: no cover
                print(f"cleanup failed for track {created_track_id}: {cleanup_error}", file=sys.stderr)
        socket.close(0)


if __name__ == "__main__":
    main()
