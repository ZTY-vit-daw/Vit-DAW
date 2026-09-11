#!/usr/bin/env python3
"""F4 minimal forensics driver: bare kernel + ZMQ probe to pin the
agent-instantiate vs manual-rack_add_node divergence.

Boots ONLY the VitApp kernel (no agent, no Godot) against an isolated stack
copy, then against the same track issues:
  phase A  legacy instantiate_plugin (the agent/VSP static_eq path form)
  phase B  legacy rack_add_node (the Godot plugin-library drop path form)
and captures for each phase:
  - the raw delta_update stream published on the kernel PUB socket (5556)
  - the legacy get_project_state track rows (plugins array + rack object)
  - the VSP state.snapshot (project.timeline) the Godot repository consumes

Findings are written to <artifacts>/findings.json plus a printed summary.
Exit 0 = experiment completed; assertions below are about EVIDENCE CAPTURE,
not about any fix being applied.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import shutil
import signal
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import Any

import zmq

REPO_ROOT = Path(__file__).resolve().parents[1]
DEFAULT_KERNEL_EXE = REPO_ROOT / "VitApp" / "build" / "VitApp_artefacts" / "Release" / "VitApp.exe"
DEFAULT_VITAPP_ROOT = REPO_ROOT / "VitApp"
DEFAULT_FIXTURE = REPO_ROOT / "artifacts" / "mantest3" / "20260911" / "project" / "spv1_p01.vit"
BX_HYBRID_PATH = r"C:\Program Files\Common Files\VST3\Plugin Alliance\bx_hybrid V2.vst3"
TARGET_TRACK_ID = "1017"  # guitar: untouched by tonight's live-stack loads
SETTLE_SECONDS = 4.0


def now_utc() -> str:
    return dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def port_open(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.settimeout(0.5)
        return s.connect_ex(("127.0.0.1", port)) == 0


def wait_port(port: int, timeout_s: float) -> bool:
    deadline = time.monotonic() + timeout_s
    while time.monotonic() < deadline:
        if port_open(port):
            return True
        time.sleep(0.5)
    return False


class PubCapture:
    """SUB socket on the kernel PUB surface; keeps every message with a wall clock tag."""

    def __init__(self, endpoint: str) -> None:
        self.ctx = zmq.Context.instance()
        self.sock = self.ctx.socket(zmq.SUB)
        self.sock.setsockopt(zmq.SUBSCRIBE, b"")
        self.sock.setsockopt(zmq.RCVHWM, 20000)
        self.sock.setsockopt(zmq.RCVTIMEO, 100)
        self.sock.connect(endpoint)
        self.messages: list[dict[str, Any]] = []
        self._lock = threading.Lock()
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, daemon=True)

    def start(self) -> None:
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._thread.join(timeout=2.0)
        try:
            self.sock.close(0)
        except zmq.ZMQError:
            pass

    def _run(self) -> None:
        while not self._stop.is_set():
            try:
                raw = self.sock.recv_string()
            except zmq.Again:
                continue
            except zmq.ZMQError:
                break
            entry: dict[str, Any] = {"wall": time.time()}
            try:
                entry["payload"] = json.loads(raw)
            except json.JSONDecodeError:
                entry["raw"] = raw
            with self._lock:
                self.messages.append(entry)

    def drain(self, seconds: float) -> None:
        time.sleep(seconds)

    def snapshot_since(self, mark_index: int) -> list[dict[str, Any]]:
        with self._lock:
            return list(self.messages[mark_index:])

    def mark(self) -> int:
        with self._lock:
            return len(self.messages)


def vsp_envelope(session_id: str, channel: str, mtype: str, schema: str, payload: dict[str, Any],
                 timeout_ms: int = 30000) -> dict[str, Any]:
    return {"vsp_version": "1.0", "schema": schema, "message_id": f"msg_f4{time.time_ns()}",
            "session_id": session_id, "client_id": "agent.main", "role": "agent",
            "channel": channel, "type": mtype, "created_at": now_utc(),
            "trace_id": f"trace_f4{time.time_ns()}", "payload": payload,
            "command_timeout_ms": timeout_ms}


class KernelSession:
    def __init__(self) -> None:
        self.ctx = zmq.Context.instance()
        self.sock = self.ctx.socket(zmq.REQ)
        self.sock.setsockopt(zmq.RCVTIMEO, 130000)
        self.sock.setsockopt(zmq.LINGER, 0)
        self.sock.connect("tcp://127.0.0.1:5555")
        self.session_id = "session_pending"

    def _request(self, envelope: dict[str, Any]) -> dict[str, Any]:
        self.sock.send_json(envelope)
        return json.loads(self.sock.recv())

    def hello(self) -> None:
        reply = self._request(vsp_envelope("session_pending", "session", "session.hello", "vsp.session.hello.v1", {
            "client_name": "F4 forensics", "client_version": "f4-probe", "protocol_min": "1.0", "protocol_max": "1.0",
            "wants": ["command.request"], "transport_bindings": ["legacy.zmq.reqrep"]}, timeout_ms=15000))
        self.session_id = str(reply.get("session_id", ""))

    def legacy(self, cmd: str, args: dict[str, Any], timeout_ms: int = 30000) -> dict[str, Any]:
        reply = self._request(vsp_envelope(self.session_id, "command", "command.request", "vsp.command.request.v1", {
            "command": "legacy.command", "legacy": {"cmd": cmd, "args": args}}, timeout_ms=timeout_ms))
        return (reply.get("payload") or {}).get("legacy_reply") or reply

    def vsp_state_snapshot(self, scope: str = "project.timeline", timeout_ms: int = 15000) -> dict[str, Any]:
        return self._request(vsp_envelope(self.session_id, "state", "state.snapshot_request",
                                          "vsp.state.snapshot_request.v1", {"scope": scope}, timeout_ms=timeout_ms))

    def close(self) -> None:
        try:
            self.sock.close(0)
        except zmq.ZMQError:
            pass


def track_row(state: dict[str, Any], track_id: str) -> dict[str, Any] | None:
    for row in state.get("tracks") or []:
        if str(row.get("track_id") or row.get("id")) == track_id:
            return row
    return None


def summarize_track_row(row: dict[str, Any] | None) -> dict[str, Any]:
    if row is None:
        return {"present": False}
    plugins = []
    for p in row.get("plugins") or []:
        plugins.append({"id": str(p.get("plugin_item_id") or p.get("plugin_id") or p.get("id")),
                        "name": str(p.get("plugin_name") or p.get("name")),
                        "type": str(p.get("type") or p.get("plugin_type"))})
    rack = row.get("rack") or {}
    return {
        "present": True,
        "track_id": str(row.get("track_id")),
        "name": str(row.get("name")),
        "plugins": plugins,
        "rack_present": bool(rack),
        "rack_item_id": str(rack.get("rack_item_id", "")),
        "rack_nodes": [{"id": str(n.get("node_id")), "name": str(n.get("name")),
                        "zone_id": str(n.get("zone_id")), "x": n.get("x"), "y": n.get("y")}
                       for n in rack.get("nodes") or []],
        "rack_edges": [str(e.get("edge_id")) for e in rack.get("edges") or []],
    }


def summarize_vsp_track(payload: dict[str, Any], track_id: str) -> dict[str, Any]:
    for row in payload.get("tracks") or []:
        if str(row.get("track_id") or row.get("id")) == track_id:
            rack = row.get("rack") or {}
            return {
                "present": True,
                "has_plugins_field": "plugins" in row,
                "rack_present": bool(rack),
                "rack_nodes": [str(n.get("node_id")) for n in rack.get("nodes") or []],
            }
    return {"present": False}


def delta_family(events: list[dict[str, Any]]) -> list[dict[str, Any]]:
    out = []
    for entry in events:
        p = entry.get("payload") or {}
        if p.get("type") != "delta_update":
            continue
        out.append({"seq_id": p.get("seq_id"), "action": p.get("action"),
                    "target_uid": p.get("target_uid"), "value": p.get("value")})
    return out


def prepare_stack(art_dir: Path, kernel_exe: Path, vitapp_root: Path, fixture: Path) -> tuple[Path, Path]:
    stack_root = art_dir / "stack" / "VitApp"
    stack_root.mkdir(parents=True, exist_ok=True)
    shutil.copy2(kernel_exe, stack_root / kernel_exe.name)
    # Anchor VitPaths::climbToVitAppRoot at the isolated copy, never the repo tree.
    shutil.copy2(vitapp_root / "CMakeLists.txt", stack_root / "CMakeLists.txt")
    (stack_root / "Source").mkdir(exist_ok=True)
    settings_src = vitapp_root / "Workspace" / "Settings" / "Settings.xml"
    settings_dst = stack_root / "Workspace" / "Settings"
    settings_dst.mkdir(parents=True, exist_ok=True)
    if settings_src.is_file():
        shutil.copy2(settings_src, settings_dst / "Settings.xml")
    project_dir = art_dir / "project"
    project_dir.mkdir(parents=True, exist_ok=True)
    project_path = project_dir / fixture.name
    shutil.copy2(fixture, project_path)
    return stack_root, project_path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kernel-exe", type=Path, default=DEFAULT_KERNEL_EXE)
    parser.add_argument("--vitapp-root", type=Path, default=DEFAULT_VITAPP_ROOT)
    parser.add_argument("--fixture", type=Path, default=DEFAULT_FIXTURE)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--skip-teardown", action="store_true")
    args = parser.parse_args()

    if not args.kernel_exe.is_file():
        raise SystemExit(f"kernel exe missing: {args.kernel_exe}")
    if not args.fixture.is_file():
        raise SystemExit(f"fixture missing: {args.fixture}")
    if port_open(5555):
        raise SystemExit("port 5555 already listening; refusing to touch a live stack (AGENTS §9)")

    args.artifacts.mkdir(parents=True, exist_ok=True)
    stack_root, project_path = prepare_stack(args.artifacts, args.kernel_exe, args.vitapp_root, args.fixture)

    kernel_log = open(args.artifacts / "kernel_stdout.log", "wb")
    kernel = subprocess.Popen([str(stack_root / args.kernel_exe.name)], cwd=str(stack_root),
                              stdout=kernel_log, stderr=subprocess.STDOUT,
                              creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
    findings: dict[str, Any] = {"started_at": now_utc(), "kernel_pid": kernel.pid,
                                "kernel_exe": str(args.kernel_exe),
                                "project_fixture": str(args.fixture)}

    pub = PubCapture("tcp://127.0.0.1:5556")
    session = KernelSession()
    try:
        if not wait_port(5555, 60.0):
            raise RuntimeError("kernel command port 5555 never came up")
        pub.start()
        session.hello()
        findings["session_id"] = session.session_id

        open_reply = session.legacy("open_project", {"file_path": str(project_path)}, timeout_ms=60000)
        findings["open_project"] = {"status": str(open_reply.get("status")), "message": str(open_reply.get("message", ""))[:300]}
        if str(open_reply.get("status")).lower() != "ok":
            raise RuntimeError(f"open_project failed: {json.dumps(open_reply)[:500]}")
        pub.drain(SETTLE_SECONDS)

        baseline_state = session.legacy("get_project_state", {}, timeout_ms=60000)
        baseline_vsp = session.vsp_state_snapshot()
        findings["baseline"] = {
            "legacy_track": summarize_track_row(track_row(baseline_state, TARGET_TRACK_ID)),
            "vsp_track": summarize_vsp_track(baseline_vsp.get("payload") or {}, TARGET_TRACK_ID),
        }

        # ---- phase A: agent/VSP path (bare instantiate) ----
        mark_a = pub.mark()
        t0_a = time.time()
        inst_reply = session.legacy("instantiate_plugin", {
            "track_id": TARGET_TRACK_ID, "plugin_path": BX_HYBRID_PATH}, timeout_ms=120000)
        findings["phase_a_instantiate_reply"] = {
            "status": str(inst_reply.get("status")),
            "plugin_id": str(inst_reply.get("plugin_id", "")),
            "plugin_identifier": str(inst_reply.get("plugin_identifier", "")),
            "graph_last_diff_summary": str(inst_reply.get("graph_last_diff_summary", "")),
        }
        pub.drain(SETTLE_SECONDS)
        events_a = pub.snapshot_since(mark_a)
        state_a = session.legacy("get_project_state", {}, timeout_ms=60000)
        vsp_a = session.vsp_state_snapshot()
        findings["phase_a"] = {
            "command_wall_s": round(time.time() - t0_a, 3),
            "delta_family": delta_family(events_a),
            "legacy_track": summarize_track_row(track_row(state_a, TARGET_TRACK_ID)),
            "vsp_track": summarize_vsp_track(vsp_a.get("payload") or {}, TARGET_TRACK_ID),
        }

        # ---- phase B: manual/Godot path (rack_add_node) ----
        plugin_identifier = findings["phase_a_instantiate_reply"]["plugin_identifier"]
        if not plugin_identifier:
            raise RuntimeError("phase A returned no plugin_identifier; cannot run phase B")
        mark_b = pub.mark()
        t0_b = time.time()
        rack_reply = session.legacy("rack_add_node", {
            "track_id": TARGET_TRACK_ID, "plugin_identifier": plugin_identifier,
            "x": 40.0, "y": 500.0, "zone_id": "z3", "auto_connect": True}, timeout_ms=120000)
        findings["phase_b_rack_add_reply"] = {
            "status": str(rack_reply.get("status")),
            "plugin_id": str(rack_reply.get("plugin_id", "")),
            "rack_item_id": str(rack_reply.get("rack_item_id", "")),
            "graph_last_diff_summary": str(rack_reply.get("graph_last_diff_summary", "")),
        }
        pub.drain(SETTLE_SECONDS)
        events_b = pub.snapshot_since(mark_b)
        state_b = session.legacy("get_project_state", {}, timeout_ms=60000)
        vsp_b = session.vsp_state_snapshot()
        findings["phase_b"] = {
            "command_wall_s": round(time.time() - t0_b, 3),
            "delta_family": delta_family(events_b),
            "legacy_track": summarize_track_row(track_row(state_b, TARGET_TRACK_ID)),
            "vsp_track": summarize_vsp_track(vsp_b.get("payload") or {}, TARGET_TRACK_ID),
        }

        # raw captures for the report trail
        (args.artifacts / "baseline_state.json").write_text(
            json.dumps(baseline_state, indent=2, ensure_ascii=False), encoding="utf-8")
        (args.artifacts / "state_after_instantiate.json").write_text(
            json.dumps(state_a, indent=2, ensure_ascii=False), encoding="utf-8")
        (args.artifacts / "state_after_rack_add.json").write_text(
            json.dumps(state_b, indent=2, ensure_ascii=False), encoding="utf-8")
        (args.artifacts / "vsp_snapshot_after_rack_add.json").write_text(
            json.dumps(vsp_b, indent=2, ensure_ascii=False), encoding="utf-8")
        with pub._lock:
            (args.artifacts / "pub_stream.json").write_text(
                json.dumps(pub.messages, indent=2, ensure_ascii=False), encoding="utf-8")

        a_legacy = findings["phase_a"]["legacy_track"]
        b_legacy = findings["phase_b"]["legacy_track"]
        a_vsp = findings["phase_a"]["vsp_track"]
        b_vsp = findings["phase_b"]["vsp_track"]
        summary = {
            "A_agent_path": {
                "delta_count": len(findings["phase_a"]["delta_family"]),
                "delta_actions": sorted({d["action"] for d in findings["phase_a"]["delta_family"]}),
                "legacy_plugins": [p["name"] for p in a_legacy.get("plugins", [])],
                "legacy_rack_nodes": [n["name"] for n in a_legacy.get("rack_nodes", [])],
                "vsp_rack_nodes": a_vsp.get("rack_nodes", []),
                "vsp_has_plugins_field": a_vsp.get("has_plugins_field"),
            },
            "B_manual_path": {
                "delta_count": len(findings["phase_b"]["delta_family"]),
                "delta_actions": sorted({d["action"] for d in findings["phase_b"]["delta_family"]}),
                "legacy_plugins": [p["name"] for p in b_legacy.get("plugins", [])],
                "legacy_rack_nodes": [n["name"] for n in b_legacy.get("rack_nodes", [])],
                "vsp_rack_nodes": b_vsp.get("rack_nodes", []),
                "vsp_has_plugins_field": b_vsp.get("has_plugins_field"),
            },
        }
        findings["summary"] = summary
        findings["completed_at"] = now_utc()
        print(json.dumps(summary, indent=2, ensure_ascii=False))
    finally:
        session.close()
        pub.stop()
        if not args.skip_teardown:
            kernel.send_signal(signal.SIGTERM)
            try:
                kernel.wait(timeout=10)
            except subprocess.TimeoutExpired:
                kernel.kill()
                kernel.wait(timeout=10)
        kernel_log.close()
        (args.artifacts / "findings.json").write_text(
            json.dumps(findings, indent=2, ensure_ascii=False), encoding="utf-8")
        print(f"findings -> {args.artifacts / 'findings.json'}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
