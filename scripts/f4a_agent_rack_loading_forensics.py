"""F4A real-stack forensics: agent-governed EQ load must land in the rack.

Reuses a LIVE three-piece stack (kernel ZMQ 5555, agent HTTP 7878, Godot
frontend) started by run_free_state_d1_smoke.ps1 / dev_agent_smoke.ps1 (both
leave the stack running). On that stack it drives one governed free-state
experiment round (free_state_d1_smoke.py --prompt-flavor frequency
--expect-domain static_eq), which makes the agent's StaticEQVSPPort load the
whitelisted EQ through rack_add_node, then proves the rack-wrapped form from
three surfaces:

  1. the raw legacy get_project_state track row: rack.nodes carries the EQ
     instance (the receipt's plugin_id) while the flat plugins array carries
     only the rack wrapper — the manual-drag shape from F4 forensics;
  2. rack.edges non-empty on the same track (the 6-connection family);
  3. the VSP compact snapshot (the surface the Godot rack repository reads):
     rack_nodes includes the plugin id.

Artifacts land under <artifacts>/f4a_report.json plus raw captures; exit 0
means every assertion passed. Exit 3 (NOT_EXERCISED) covers model-selection
outcomes that never reached an applied static_eq receipt.

This probe never mutates the project itself: the only writes come from the
governed agent round inside its own copied project workdir.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import subprocess
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any

import zmq

NOT_EXERCISED_EXIT = 3

REPO_ROOT = Path(__file__).resolve().parents[1]


def vsp_envelope(session_id: str, channel: str, mtype: str, schema: str, payload: dict[str, Any], timeout_ms: int) -> dict[str, Any]:
    now = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    return {"vsp_version": "1.0", "schema": schema, "message_id": f"msg_f4a{time.time_ns()}",
            "session_id": session_id, "client_id": "agent.main", "role": "agent",
            "channel": channel, "type": mtype, "created_at": now,
            "trace_id": f"trace_f4a{time.time_ns()}", "payload": payload,
            "command_timeout_ms": timeout_ms}


class KernelSession:
    """One VSP session over the kernel's raw ZMQ REQ surface."""

    def __init__(self, timeout_ms: int = 15000) -> None:
        self.timeout_ms = timeout_ms
        ctx = zmq.Context.instance()
        self.sock = ctx.socket(zmq.REQ)
        self.sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
        self.sock.setsockopt(zmq.LINGER, 0)
        self.sock.connect("tcp://127.0.0.1:5555")
        self.sock.send_json(vsp_envelope("session_pending", "session", "session.hello", "vsp.session.hello.v1", {
            "client_name": "F4A forensics", "client_version": "f4a-probe", "protocol_min": "1.0", "protocol_max": "1.0",
            "wants": ["command.request"], "transport_bindings": ["legacy.zmq.reqrep"]}, timeout_ms))
        reply = json.loads(self.sock.recv())
        self.session_id = str(reply.get("session_id") or "")
        if not self.session_id:
            self.close()
            raise RuntimeError("kernel hello returned no session_id")

    def legacy(self, cmd: str, args: dict[str, Any], timeout_ms: int = 60000) -> dict[str, Any]:
        self.timeout_ms = timeout_ms
        self.sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
        self.sock.send_json(vsp_envelope(self.session_id, "command", "command.request", "vsp.command.request.v1", {
            "command": "legacy.command", "legacy": {"cmd": cmd, "args": args}}, timeout_ms))
        reply = json.loads(self.sock.recv())
        return (reply.get("payload") or {}).get("legacy_reply") or {}

    def state_snapshot(self, scope: str, timeout_ms: int = 60000) -> dict[str, Any]:
        self.sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
        self.sock.send_json(vsp_envelope(self.session_id, "state", "state.snapshot_request", "vsp.state.snapshot_request.v1", {
            "scope": scope}, timeout_ms))
        return json.loads(self.sock.recv())

    def close(self) -> None:
        self.sock.close()


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as response:
        parsed = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def first_text(*values: Any) -> str:
    for value in values:
        if isinstance(value, str) and value.strip():
            return value.strip()
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            return str(value)
    return ""


def rows(value: Any) -> list[dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    return []


def save_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def find_governed_receipt(report: dict[str, Any]) -> dict[str, Any]:
    """Return the applied static_eq intervention receipt from a d1 smoke report.

    The report shape carries the reasoning loop under
    responses[].workflow_data.free_state_reasoning_loop.experiment.rounds[]
    .interventions[].receipt; the treated track lives in
    validation.target_ref.id (the receipt itself no longer repeats track_id).
    """
    for response in rows(report.get("responses")):
        workflow_data = response.get("workflow_data")
        if not isinstance(workflow_data, dict):
            continue
        loop = workflow_data.get("free_state_reasoning_loop")
        if not isinstance(loop, dict):
            continue
        experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
        for round_row in rows(experiment.get("rounds")):
            for intervention in rows(round_row.get("interventions")):
                receipt = intervention.get("receipt")
                if isinstance(receipt, dict) and receipt.get("plugin_id"):
                    details = receipt.get("details") if isinstance(receipt.get("details"), dict) else {}
                    merged = {k: v for k, v in receipt.items()}
                    for key in ("plugin_id", "plugin_instantiated_by_action", "param_id", "actual_readback_value"):
                        if key not in merged or merged[key] is None:
                            merged[key] = details.get(key)
                    validation = report.get("validation") if isinstance(report.get("validation"), dict) else {}
                    target = validation.get("target_ref") if isinstance(validation.get("target_ref"), dict) else {}
                    merged["track_id"] = first_text(merged.get("track_id"), target.get("id"))
                    return merged
    return {}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--public-manifest", default=str(
        REPO_ROOT / "temp" / "semantic-processor-agent-project-smoke-v1" / "fixtures"
        / "semantic_processor_project_smoke_v1_80085263a651cf20" / "fixture_manifest.json"))
    parser.add_argument("--public-case-id", default="spv1_p01")
    parser.add_argument("--timeout-sec", type=int, default=600)
    parser.add_argument("--artifacts", type=Path, default=REPO_ROOT / "artifacts" / "F4A" / "live")
    parser.add_argument("--verify-existing", action="store_true",
                        help="skip the governed driver round: verify the rack shape against the d1_smoke_report.json already inside --artifacts (the stack must still have that project open)")
    args = parser.parse_args()

    args.artifacts.mkdir(parents=True, exist_ok=True)
    report: dict[str, Any] = {"probe": "f4a_agent_rack_loading", "started_at": dt.datetime.now(dt.timezone.utc).isoformat()}
    sys.stderr.write("F4A forensics: probing live stack ...\n")

    # Liveness preconditions: the launcher left the three-piece stack running.
    health = request_json("GET", args.agent_http.rstrip("/") + "/health", None, 10.0)
    require(str(health.get("status", "")).lower() in {"ok", "healthy"} or health.get("status") is not None,
            "agent /health did not answer on the live stack: " + json.dumps(health)[:300])
    session = KernelSession()
    try:
        # The governed round: copy a fresh public case project, drive the
        # free-state chat loop until the model proposes an EQ move; the
        # StaticEQVSPPort Apply loads the whitelisted EQ through rack_add_node
        # and writes one band parameter on it. --verify-existing skips this
        # and reuses the round's report (stack keeps that project open).
        driver_report = args.artifacts / "d1_smoke_report.json"
        if args.verify_existing:
            require(driver_report.exists(), f"--verify-existing needs {driver_report}")
            report["driver_command"] = "(reused existing governed round via --verify-existing)"
            report["driver_exit_code"] = 0
        else:
            workdir = args.artifacts / "project"
            driver = [
                sys.executable, str(REPO_ROOT / "scripts" / "free_state_d1_smoke.py"),
                "--public-manifest", args.public_manifest, "--public-case-id", args.public_case_id,
                "--agent-http", args.agent_http, "--timeout-sec", str(args.timeout_sec),
                "--project-workdir", str(workdir), "--output", str(driver_report),
                "--prompt-flavor", "frequency", "--expect-domain", "static_eq",
            ]
            report["driver_command"] = " ".join(driver)
            completed = subprocess.run(driver, capture_output=True, text=True, encoding="utf-8", errors="replace")
            report["driver_exit_code"] = completed.returncode
            (args.artifacts / "driver_stdout.log").write_text(completed.stdout, encoding="utf-8")
            (args.artifacts / "driver_stderr.log").write_text(completed.stderr, encoding="utf-8")
            if completed.returncode == NOT_EXERCISED_EXIT:
                report["status"] = "not_exercised"
                report["reason"] = "model run did not autonomously select static_eq; see d1_smoke_report.json"
                report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
                save_json(args.artifacts / "f4a_report.json", report)
                print("F4A NOT_EXERCISED: " + report["reason"])
                return NOT_EXERCISED_EXIT
            require(completed.returncode == 0,
                    f"governed round failed with exit {completed.returncode}; stderr tail: {completed.stderr[-2000:]}")

        round1 = json.loads(driver_report.read_text(encoding="utf-8"))
        receipt = find_governed_receipt(round1)
        report["static_eq_receipt"] = receipt
        require(receipt.get("plugin_id") and receipt.get("track_id"),
                "applied static_eq receipt carries no plugin_id/track: " + json.dumps(receipt, ensure_ascii=False))
        plugin_id = str(receipt["plugin_id"])
        track_id = str(receipt["track_id"])
        require(receipt.get("plugin_instantiated_by_action") is True,
                "expected the governed load path (fresh track, no reusable instance); receipt: "
                + json.dumps(receipt, ensure_ascii=False))

        # Surface 1: raw legacy track row — rack.nodes carries the EQ instance
        # and the flat plugins array carries only the rack wrapper.
        state = session.legacy("get_project_state", {})
        save_json(args.artifacts / "state_after_governed_load.json", state)
        track_row = None
        for track in rows(state.get("tracks")):
            if first_text(track.get("track_id"), track.get("id")) == track_id:
                track_row = track
                break
        require(track_row is not None, f"target track {track_id} missing from get_project_state")
        rack = track_row.get("rack") if isinstance(track_row.get("rack"), dict) else {}
        rack_nodes = rows(rack.get("nodes"))
        node_ids = {first_text(node.get("plugin_item_id"), node.get("item_id"), node.get("plugin_id"), node.get("id"), node.get("node_id")) for node in rack_nodes}
        rack_edges = [str(edge) for edge in (rack.get("edges") or track_row.get("rack_edges") or [])]
        flat_ids = {first_text(plugin.get("plugin_item_id"), plugin.get("item_id"), plugin.get("plugin_id"), plugin.get("id"))
                    for plugin in rows(track_row.get("plugins"))}
        report["rack_shape"] = {
            "track_id": track_id, "rack_item_id": first_text(rack.get("rack_item_id")),
            "rack_node_ids": sorted(node_ids), "rack_edge_count": len(rack_edges),
            "flat_plugin_ids": sorted(flat_ids), "eq_node": next((node for node in rack_nodes
                if first_text(node.get("plugin_item_id"), node.get("item_id"), node.get("plugin_id"), node.get("id"), node.get("node_id")) == plugin_id), None),
        }
        require(plugin_id in node_ids,
                f"rack.nodes does not carry the governed EQ instance {plugin_id}: {sorted(node_ids)}")
        require(plugin_id not in flat_ids,
                f"EQ {plugin_id} must not appear as a bare plugin row (rack-wrapped form expected): {sorted(flat_ids)}")
        require(rack_edges, "rack edges are empty — the manual-drag connection family is missing")

        # Surface 2: VSP compact snapshot (what the Godot rack repository
        # reads) must expose the plugin through the rack — the raw shape nests
        # it under track.rack.nodes (plus a flat rack_nodes summary on some
        # builds), so accept the plugin id in either face.
        snapshot = session.state_snapshot("project.timeline")
        save_json(args.artifacts / "vsp_snapshot_after_governed_load.json", snapshot)
        vsp_rack_nodes: set[str] = set()
        payload = snapshot.get("payload") if isinstance(snapshot.get("payload"), dict) else {}
        for tracks_source in (project.get("tracks") if isinstance((project := payload.get("project") or {}).get("tracks"), list) else [],
                              payload.get("snapshot", {}).get("tracks") if isinstance(payload.get("snapshot"), dict) else [],
                              payload.get("tracks") if isinstance(payload.get("tracks"), list) else []):
            for track in rows(tracks_source):
                if first_text(track.get("track_id"), track.get("id")) != track_id:
                    continue
                for node_id in (track.get("rack_nodes") or []):
                    vsp_rack_nodes.add(first_text(node_id))
                rack_obj = track.get("rack") if isinstance(track.get("rack"), dict) else {}
                for node in rows(rack_obj.get("nodes")):
                    vsp_rack_nodes.add(first_text(node.get("plugin_item_id"), node.get("item_id"), node.get("plugin_id"), node.get("id"), node.get("node_id")))
        report["vsp_rack_nodes"] = sorted(vsp_rack_nodes)
        require(plugin_id in vsp_rack_nodes,
                f"VSP snapshot rack_nodes does not carry {plugin_id} on track {track_id}: {sorted(vsp_rack_nodes)}")

        report["status"] = "pass"
        report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        save_json(args.artifacts / "f4a_report.json", report)
        print("F4A PASS: governed EQ load is rack-wrapped on track " + track_id
              + " plugin " + plugin_id + " (" + str(len(rack_edges)) + " rack edges, VSP rack_nodes "
              + ",".join(sorted(vsp_rack_nodes)) + ")")
        print("STACK LEFT RUNNING for Godot rack visual inspection; report=" + str(args.artifacts / "f4a_report.json"))
        return 0
    finally:
        session.close()


if __name__ == "__main__":
    try:
        sys.exit(main())
    except RuntimeError as error:
        print("F4A FAIL: " + str(error))
        sys.exit(1)
