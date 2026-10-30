#!/usr/bin/env python3
"""Real-stack observation-surface assertion for M3-TRACK-PLUGIN-DISCLOSE-1
(card 228): the kernel compact face now releases track-level plugins as a
minimal id/name/type disclosure, and the agent CCB structure view passes the
plugin chain summary through to the model observation face.

Legs (zero LLM, read-only; kernel ZMQ + agent HTTP on the live stack started
by dev_agent_smoke.ps1 -StartKernel -RestartAgent):

1. kernel VSP state snapshot -- session.hello -> state.snapshot_request:
   every visible track row carries an array-typed "plugins" key (schema
   stability, legacy rows default to an empty array); at least one
   clip-bearing track has an entry with type=="volume" and a non-empty id
   (the track-level Volume & Pan built-in now discloses its plugin identity
   instead of only volume_db/pan + plugin_param_digest); every entry exposes
   only {id,name,type} keys so no parameter payload travels.
2. agent CCB observation -- project.state primes the kernel->shadow refresh,
   then ccb.observation_request view project.structure: the disclosed track
   rows carry plugin_count>=1 and a plugin_chain_summary whose Volume & Pan
   entry has a non-empty plugin_id; the full-face per-track "plugins" key
   must NOT appear inside the view payload (bounded disclosure red line,
   CCB peer-view semantics unchanged).

Exit 0 only if every leg passes; summary JSON artifact at --output.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)

ALLOWED_PLUGIN_KEYS = {"id", "name", "type"}


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


def send(sock: zmq.Socket, message: dict[str, Any]) -> dict[str, Any]:
    sock.send_json(message)
    reply = sock.recv_json()
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object kernel reply: {reply!r}")
    return reply


def envelope(
    session_id: str,
    channel: str,
    message_type: str,
    schema: str,
    payload: dict[str, Any],
) -> dict[str, Any]:
    return {
        "vsp": "1",
        "channel": channel,
        "session_id": session_id,
        "type": message_type,
        "schema": schema,
        "payload": payload,
    }


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float) -> dict[str, Any]:
    payload = json.dumps(
        {"tool": tool, "args": args, "confirmed": False, "source": "track_plugin_disclose_smoke"}
    ).encode("utf-8")
    request = urllib.request.Request(
        base.rstrip("/") + "/agent/invoke",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"HTTP {error.code} from {tool}: {body[:2000]}") from error
    parsed = json.loads(body)
    if str(parsed.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(parsed)[:2000]}")
    result = parsed.get("result", parsed)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned a non-object result")
    return result


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def as_rows(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [row for row in value if isinstance(row, dict)]


def volume_pan_entries(plugins: Any) -> list[dict[str, Any]]:
    rows = as_rows(plugins)
    return [
        row
        for row in rows
        if str(row.get("type", "")).strip().lower() == "volume"
    ]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--timeout-ms", type=int, default=20000)
    parser.add_argument("--http-timeout-sec", type=float, default=120.0)
    args = parser.parse_args()

    summary: dict[str, Any] = {
        "card": "M3-TRACK-PLUGIN-DISCLOSE-1",
        "started_at": datetime.now(timezone.utc).isoformat(),
        "legs": {},
    }

    # ---- leg 1: kernel VSP compact state snapshot --------------------------
    ctx = zmq.Context.instance()
    sock = make_socket(ctx, args.kernel_zmq, args.timeout_ms)
    try:
        hello = send(
            sock,
            envelope(
                "",
                "session",
                "session.hello",
                "vsp.session.hello.v1",
                {
                    "client_name": "track plugin disclose smoke",
                    "client_version": "0.1.0",
                    "protocol_min": "1.0",
                    "protocol_max": "1.0",
                    "wants": ["project.read", "state.subscribe"],
                    "transport_bindings": ["legacy.zmq_reqrep"],
                },
            ),
        )
        require(hello.get("type") == "session.hello_ack", f"hello: unexpected reply {hello!r}")
        session_id = str(hello.get("session_id") or "").strip()
        require(session_id, "hello: session_id missing")

        snapshot = send(
            sock,
            envelope(
                session_id,
                "state",
                "state.snapshot_request",
                "vsp.state.snapshot_request.v1",
                {"scope": "project.timeline"},
            ),
        )
        require(snapshot.get("type") == "state.snapshot", f"snapshot: bad type {snapshot!r}")
        payload = snapshot.get("payload") or {}
        tracks = as_rows(payload.get("tracks"))
        require(len(tracks) > 0, "snapshot: no visible tracks in payload (default project should carry audio tracks)")

        rows_without_plugins_array = [
            str(row.get("track_id")) for row in tracks if not isinstance(row.get("plugins"), list)
        ]
        require(
            not rows_without_plugins_array,
            f"snapshot: track rows without array-typed plugins key: {rows_without_plugins_array}",
        )

        bad_entry_keys: list[str] = []
        volume_rows: list[dict[str, Any]] = []
        for row in tracks:
            for entry in as_rows(row.get("plugins")):
                extra = sorted(set(entry.keys()) - ALLOWED_PLUGIN_KEYS)
                if extra:
                    bad_entry_keys.append(f"{row.get('track_id')}:{','.join(extra)}")
                if not str(entry.get("id", "")).strip():
                    bad_entry_keys.append(f"{row.get('track_id')}:missing-id")
            entries = volume_pan_entries(row.get("plugins"))
            if entries and isinstance(row.get("clips"), list) and row.get("clips"):
                volume_rows.append(row)
        require(
            not bad_entry_keys,
            f"snapshot: plugin entries leaked non-minimal/missing keys: {bad_entry_keys}",
        )
        require(
            len(volume_rows) > 0,
            "snapshot: no clip-bearing track disclosed a type=volume plugin entry "
            "(track-level Volume & Pan identity must be visible)",
        )
        sample = volume_rows[0]
        sample_entry = volume_pan_entries(sample.get("plugins"))[0]
        require(
            str(sample_entry.get("id", "")).strip() and str(sample_entry.get("name", "")).strip(),
            f"snapshot: Volume & Pan entry lacks id/name: {sample_entry!r}",
        )
        summary["legs"]["kernel_vsp_snapshot"] = {
            "status": "pass",
            "visible_tracks": len(tracks),
            "tracks_with_volume_plugin": len(volume_rows),
            "sample_track_id": str(sample.get("track_id")),
            "sample_entry": sample_entry,
        }
    finally:
        sock.close(0)
        ctx.term()

    # ---- leg 2: agent CCB project.structure observation --------------------
    state_result = invoke(args.agent_http, "project.state", {}, args.http_timeout_sec)
    state_tracks = as_rows(state_result.get("tracks"))
    summary["legs"]["agent_project_state_tracks"] = len(state_tracks)

    observation = invoke(
        args.agent_http,
        "ccb.observation_request",
        {
            "request_id": "track-plugin-disclose-1",
            "mix_session_id": "track_plugin_disclose_" + str(int(time.time() * 1000)),
            "view_ids": ["project.structure"],
            "max_disclosure_bytes": 65536,
        },
        args.http_timeout_sec,
    )
    bundle = observation.get("bundle")
    require(isinstance(bundle, dict), f"ccb result lacks bundle: {observation!r}")
    require(
        bundle.get("schema_version") == "ccb_observation_bundle.v1",
        f"unexpected bundle schema: {bundle.get('schema_version')}",
    )
    require(
        bundle.get("read_only") is True and bundle.get("mutation_authority") is False,
        f"bundle authority boundary failed: read_only={bundle.get('read_only')} "
        f"mutation_authority={bundle.get('mutation_authority')}",
    )
    views = bundle.get("views") if isinstance(bundle.get("views"), dict) else {}
    require("project.structure" in views, f"project.structure view missing; omissions={bundle.get('omissions')}")
    view = views["project.structure"]
    facts = view.get("facts") if isinstance(view, dict) and isinstance(view.get("facts"), dict) else {}
    tracks_summary = facts.get("project.tracks.summary")
    require(isinstance(tracks_summary, dict), f"project.tracks.summary fact missing: {facts!r}")
    view_tracks = as_rows(tracks_summary.get("tracks"))
    require(len(view_tracks) > 0, "project.tracks.summary has no track rows")

    chain_rows = 0
    volume_visible = False
    sample_chain_entry: dict[str, Any] = {}
    for row in view_tracks:
        count = row.get("plugin_count")
        chain = as_rows(row.get("plugin_chain_summary"))
        if isinstance(count, (int, float)) and int(count) >= 1 and chain:
            chain_rows += 1
        for entry in chain:
            plugin_id = str(entry.get("plugin_id", "")).strip()
            entry_type = str(entry.get("type", "")).strip().lower()
            if plugin_id and entry_type == "volume":
                volume_visible = True
                sample_chain_entry = entry
    require(chain_rows > 0, "no view track row carried plugin_count>=1 with a plugin_chain_summary")
    require(
        volume_visible,
        "Volume & Pan entry with a visible plugin_id not found in the disclosed chain summaries",
    )

    encoded = json.dumps(view, ensure_ascii=False)
    for row_text in ('"plugins"', '"params"'):
        require(
            row_text not in encoded,
            f"full-face plugin payload leaked into the project.structure view: {row_text}",
        )
    summary["legs"]["agent_ccb_structure_view"] = {
        "status": "pass",
        "view_track_rows": len(view_tracks),
        "rows_with_chain": chain_rows,
        "sample_volume_entry": sample_chain_entry,
        "disclosure_bytes": bundle.get("disclosure_bytes"),
    }

    summary["status"] = "pass"
    summary["finished_at"] = datetime.now(timezone.utc).isoformat()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as error:  # noqa: BLE001 - smoke harness reports and fails closed
        print(f"track_plugin_disclose_smoke FAILED: {error}", file=sys.stderr)
        sys.exit(1)
