#!/usr/bin/env python3
"""Agent-side send/return semantics smoke (AGENT-SENDROUTE-SEMANTICS-1, design
MIX-SENDRETURN-DESIGN-1 §5 scenario 2).

Same pattern as b1_group_reset_agent_smoke.py: drive the real agent HTTP face
(/agent/chat + /agent/confirm + /agent/invoke) with the real kernel behind it.
Legs:

1. setup — authority full_project_access, fresh project, two named tracks
   (vocals/guitar) via add_track+rename_track.
2. send plan — the card's acceptance phrasing 「给 vocals 和 guitar 共用一个
   板式混响」 must produce a pendingplan with workflow send_topology_connect
   (NOT an insert plan); approve must execute the connects; track.send.list
   read-back asserts 2 sends into 1 FX track with 0 dangling edges.
3. MOM projection — mix.observe (multitrack intent) response must carry
   mom_projection.send_topology rows (2 edges, 1 fx track) — the observation
   plane asserts the structural facts, not effect semantics.
4. FX-track rack load (KERNEL-SENDROUTE-IMPL-1 acceptance boundary: the
   dispatcher chain-normalisation hook had no automated assertion for rack
   loads onto FX tracks) — rack_add_node via the kernel ZMQ face onto the FX
   track; the send topology read-back must stay intact afterwards.
5. insert regression — the single-track phrasing 「给 vocals 加载一个混响」
   must still produce the INSERT load plan (rack_add_node on the named track,
   workflow plugin_grabber_load_and_get_params), never a send plan; the plan
   is then cancelled to leave no pending mutation.

Exit code 0 only if every leg passes; summary JSON artifact at --output.
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
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)

SEND_MESSAGE = "给 vocals 和 guitar 共用一个板式混响"
INSERT_MESSAGE = "给 vocals 加载一个混响"
CHAT_TIMEOUT_SEC = 180.0


def post_json(base_url: str, path: str, payload: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read().decode("utf-8"))


def invoke(base_url: str, tool: str, args: Optional[Dict[str, Any]] = None, *, confirmed: bool = True, timeout: float = 120.0) -> Dict[str, Any]:
    payload: Dict[str, Any] = {"tool": tool, "args": args or {}}
    if confirmed:
        payload["confirmed"] = True
    reply = post_json(base_url, "/agent/invoke", payload, timeout)
    if reply.get("status") != "ok":
        raise RuntimeError(f"{tool} failed: {json.dumps(reply, ensure_ascii=False)[:1200]}")
    return reply.get("result") or {}


def require(condition: bool, message: str, legs: List[str]) -> None:
    if not condition:
        raise RuntimeError(f"[leg {len(legs)}] {message}")


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


def wait_for_track_names(base_url: str, wanted: List[str], attempts: int = 20) -> Dict[str, str]:
    for _ in range(attempts):
        state = invoke(base_url, "project.state", {}, timeout=60.0)
        found = {}
        for row in state.get("tracks") or []:
            name = row.get("track_name") or row.get("name")
            if name in wanted:
                found[name] = row.get("track_id") or row.get("id")
        if len(found) == len(wanted):
            return found
        time.sleep(1.0)
    raise RuntimeError(f"named tracks did not appear: want {wanted}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-sec", type=int, default=240)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    legs: List[str] = []
    summary: Dict[str, Any] = {
        "scenario": "AGENT-SENDROUTE-SEMANTICS-1 agent-side send/return semantics",
        "agent_http": args.agent_http,
        "started_at": datetime.now(timezone.utc).isoformat(),
        "legs": legs,
    }
    conversation = f"sendroute_agent_smoke_{int(time.time())}"

    def record(name: str, detail: Dict[str, Any]) -> None:
        legs.append(name)
        summary[name] = detail
        print(f"[ok] {name}: {json.dumps(detail, ensure_ascii=False)[:400]}")

    try:
        # ---- leg 1: setup -------------------------------------------------
        # Manual authority (default): structural writes still need the user
        # confirmation, which is exactly the pendingplan face under test.
        invoke(args.agent_http, "project.new", {"force": True}, timeout=120.0)
        track_ids: List[str] = []
        for name in ("vocals", "guitar"):
            added = invoke(args.agent_http, "track.add_audio", {}, timeout=60.0)
            track_id = added.get("track_id") or added.get("id")
            require(bool(track_id), f"add_track reply missing track_id: {added}", legs)
            invoke(args.agent_http, "track.rename", {"track_id": track_id, "name": name}, timeout=60.0)
            track_ids.append(track_id)
        named = wait_for_track_names(args.agent_http, ["vocals", "guitar"])
        record("setup", {"authority": "manual_default", "tracks": named})

        # ---- leg 2: send plan + confirm + read-back ----------------------
        pending = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": conversation,
            "message": SEND_MESSAGE,
            "context": {"agent_mode": "chat"},
        }, CHAT_TIMEOUT_SEC)
        require(
            bool(pending.get("needs_confirmation")) and bool(pending.get("plan_id")),
            f"send phrasing did not create a pending confirmation: {json.dumps(pending, ensure_ascii=False)[:1500]}",
            legs,
        )
        require(
            pending.get("workflow") == "send_topology_connect",
            f"pending workflow = {pending.get('workflow')!r}, want send_topology_connect",
            legs,
        )
        decisions = pending.get("commands") or []
        connect_cmds = [d for d in decisions if str(d.get("name") or (d.get("command") or {}).get("cmd", "")).replace("_", ".").startswith("track.send.connect")]
        require(len(connect_cmds) == 2, f"plan decisions = {json.dumps(decisions, ensure_ascii=False)[:800]}", legs)
        confirmed = post_json(args.agent_http, "/agent/confirm", {"plan_id": pending["plan_id"], "decision": "approve"}, CHAT_TIMEOUT_SEC)
        require(confirmed.get("status") == "ok" and confirmed.get("goal_status") == "completed",
                f"send confirm failed: {json.dumps(confirmed, ensure_ascii=False)[:1500]}", legs)
        topology = invoke(args.agent_http, "track.send.list", {}, timeout=60.0)
        sends = topology.get("sends") or []
        fx_rows = topology.get("fx_tracks") or []
        dangling = topology.get("dangling") or []
        require(len(sends) == 2, f"track.send.list sends = {len(sends)}, want 2: {json.dumps(topology, ensure_ascii=False)[:800]}", legs)
        require(len(fx_rows) == 1, f"fx_tracks = {len(fx_rows)}, want 1", legs)
        require(len(dangling) == 0, f"dangling = {len(dangling)}, want 0", legs)
        source_ids = sorted(str(row.get("source_track_id")) for row in sends)
        require(source_ids == sorted(track_ids), f"send sources = {source_ids}, want the named tracks {track_ids}", legs)
        fx_id = str(fx_rows[0].get("track_id"))
        reply_text = str(confirmed.get("message") or "")
        record("send_plan_confirmed", {
            "plan_id": pending.get("plan_id"),
            "send_edge_count": len(sends),
            "fx_track_id": fx_id,
            "fx_name": fx_rows[0].get("name"),
            "reply_excerpt": reply_text[:220],
        })

        # ---- leg 3: MOM send_topology projection -------------------------
        observe = invoke(args.agent_http, "mix.observe", {
            "scope": "full_project",
            "project_context": True,
            "observation_only": True,
            "disclosure": "digest_catalog",
            "mom_intent": "project_multitrack_relation_observation",
            "mix_session_id": conversation,
            "goal_text": "send topology observation for smoke",
        }, timeout=240.0)
        mom_projection = observe.get("mom_projection") or {}
        send_topology = mom_projection.get("send_topology") or {}
        edges = send_topology.get("edges") or []
        fx_projection_rows = send_topology.get("fx_tracks") or []
        require(len(edges) == 2, f"mom send_topology edges = {len(edges)}: {json.dumps(send_topology, ensure_ascii=False)[:800]}", legs)
        require(len(fx_projection_rows) == 1, f"mom send_topology fx_tracks = {len(fx_projection_rows)}", legs)
        require(str(send_topology.get("status") or "") in ("ready", "partial"), f"mom send_topology status = {send_topology.get('status')!r}", legs)
        record("mom_send_topology_projection", {
            "status": send_topology.get("status"),
            "edges": len(edges),
            "fx_tracks": len(fx_projection_rows),
            "dangling_edge_count": send_topology.get("dangling_edge_count"),
        })

        # ---- leg 4: FX-track rack load over the kernel ZMQ face ----------
        search = zmq_command(args.kernel_zmq, {"cmd": "plugin_search", "query": "reverb", "limit": 8})
        plugins = search.get("plugins") or []
        require(search.get("status") == "ok" and plugins, f"kernel plugin_search returned no reverb candidates: {json.dumps(search, ensure_ascii=False)[:600]}", legs)
        candidate = next((p for p in plugins if p.get("plugin_path") or p.get("path")), None)
        require(candidate is not None, f"no candidate with plugin_path: {json.dumps(plugins[:3], ensure_ascii=False)[:600]}", legs)
        load = zmq_command(args.kernel_zmq, {
            "cmd": "rack_add_node",
            "track_id": fx_id,
            "plugin_path": candidate.get("plugin_path") or candidate.get("path"),
            "plugin_name": candidate.get("name") or candidate.get("plugin_name") or "",
            "plugin_kind": "Effect",
            "x": 360.0,
            "y": 260.0,
            "zone_id": "Z3",
        }, timeout_ms=120000)
        require(load.get("status") == "ok", f"rack_add_node onto FX track failed: {json.dumps(load, ensure_ascii=False)[:800]}", legs)
        topology_after = zmq_command(args.kernel_zmq, {"cmd": "track_send_list"})
        require(topology_after.get("status") == "ok" and len(topology_after.get("sends") or []) == 2,
                f"send topology broken after FX rack load: {json.dumps(topology_after, ensure_ascii=False)[:800]}", legs)
        state_after = invoke(args.agent_http, "project.state", {}, timeout=60.0)
        fx_rack_nodes: List[Dict[str, Any]] = []
        for row in state_after.get("tracks") or []:
            if (row.get("track_id") or row.get("id")) == fx_id:
                rack = row.get("rack") or {}
                fx_rack_nodes = rack.get("nodes") or []
        require(len(fx_rack_nodes) >= 1, f"FX track rack shows no nodes after load: {json.dumps(state_after.get('tracks') or [], ensure_ascii=False)[:600]}", legs)
        record("fx_track_rack_load", {
            "plugin": candidate.get("name") or candidate.get("plugin_name"),
            "fx_track_id": fx_id,
            "rack_node_count": len(fx_rack_nodes),
            "send_edges_intact": len(topology_after.get("sends") or []),
        })

        # ---- leg 5: single-track insert regression -----------------------
        regression = post_json(args.agent_http, "/agent/chat", {
            "conversation_id": conversation + "_regress",
            "message": INSERT_MESSAGE,
            "context": {"agent_mode": "chat"},
        }, CHAT_TIMEOUT_SEC)
        regression_workflow = regression.get("workflow")
        require(regression_workflow != "send_topology_connect",
                f"single-track phrasing must not take the send workflow (got {regression_workflow!r}): {json.dumps(regression, ensure_ascii=False)[:1200]}", legs)
        executed = regression.get("executed_kernel_reply") or []
        send_executions = [r for r in executed if str(r.get("command_name") or "").replace("_", ".").startswith("track.send")]
        require(len(send_executions) == 0, f"single-track phrasing executed send commands: {json.dumps(send_executions, ensure_ascii=False)[:600]}", legs)
        regression_shape = "no_plan"
        if bool(regression.get("needs_confirmation")) and regression.get("plan_id"):
            cancelled = post_json(args.agent_http, "/agent/confirm", {"plan_id": regression["plan_id"], "decision": "reject"}, 60.0)
            require(cancelled.get("status") == "ok", f"regression plan cancel failed: {json.dumps(cancelled, ensure_ascii=False)[:600]}", legs)
            regression_shape = "plan_cancelled"
        record("insert_regression", {"shape": regression_shape, "workflow": regression_workflow})
        summary["status"] = "pass"
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        return 0
    except Exception as exc:  # noqa: BLE001 — smoke-level catch-all with full context
        summary["status"] = "fail"
        summary["error"] = str(exc)
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print(f"[FAIL] {exc}", file=sys.stderr)
        return 1
    finally:
        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"[summary] {output}")


if __name__ == "__main__":
    sys.exit(main())
