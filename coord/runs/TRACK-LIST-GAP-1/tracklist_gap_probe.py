#!/usr/bin/env python3
"""TRACK-LIST-GAP-1 (139) same-frame probe: track.list tool vs kernel vs VSP hub vs shadow.

Surfaces captured at one moment (read-only; no mutations between the calls):
  S1 kernel direct ZMQ        : list_tracks, get_project_state   (authoritative kernel edit)
  S2 VSP hub canonical command: project.tracks.list              (legacy list_tracks through hub->kernel)
  S3 VSP hub state snapshot   : project.timeline                 (kernel edit state as tracks)
  S4 agent shadow             : GET /agent/state                 (shadow Summary)
  S5 agent tool               : POST /agent/invoke track.list    (what the LLM/user tool face returns)
  S6 agent tool               : POST /agent/invoke project.state (refreshes shadow from VSP snapshot)

Stages:
  S0 fresh stack baseline (blank project, no user tracks)
  S1 EXTERNAL new_project + add_track x6 + import_audio (kernel direct == GUI/external mutation path)
     -> compare all surfaces; expect kernel/hub/snapshot=6 while agent tool returns shadow (stale 0)
  S1b after agent project.state refresh -> expect shadow/tool catches up (self-heal, as observed in 118)
  S2 EXTERNAL load_project fixture with 2 tracks (open-existing path)
     -> expect the same tool-vs-kernel gap => candidate 2 (new+import path) ruled out
  S3 agent-path mutation (authority full + invoke add_track) when possible
     -> expect agent path keeps shadow in sync (no gap)

Candidate faces from 118 ATTRIBUTION 搂5:
  1) VSP channel hits a different kernel/edit instance than the GUI current edit
  2) new+import path track enumeration difference

LLM: zero calls (deterministic HTTP/ZMQ only). Failures are recorded, not faked.
"""

from __future__ import annotations

import argparse
import json
import math
import struct
import sys
import time
import urllib.error
import urllib.request
import wave
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

SINE_FREQ = 440.0
DURATION_S = 1.0


# ---------------------------------------------------------------- http helpers

def http_json(method: str, url: str, body: Optional[Dict[str, Any]] = None,
              timeout: float = 30.0) -> Dict[str, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            status = resp.status
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        status = exc.code
    try:
        parsed = json.loads(raw)
    except json.JSONDecodeError:
        parsed = {"_raw": raw[:4000]}
    if isinstance(parsed, dict):
        parsed["_http_status"] = status
    return parsed if isinstance(parsed, dict) else {"_value": parsed, "_http_status": status}


def http_get(url: str, timeout: float = 15.0) -> Dict[str, Any]:
    return http_json("GET", url, None, timeout)


# ---------------------------------------------------------------- zmq helpers

class KernelLink:
    def __init__(self, url: str, timeout_ms: int = 120000):
        self.ctx = zmq.Context.instance()
        self.sock = self.ctx.socket(zmq.REQ)
        self.sock.setsockopt(zmq.LINGER, 0)
        self.sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
        self.sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
        if hasattr(zmq, "REQ_RELAXED"):
            self.sock.setsockopt(zmq.REQ_RELAXED, 1)
        if hasattr(zmq, "REQ_CORRELATE"):
            self.sock.setsockopt(zmq.REQ_CORRELATE, 1)
        self.sock.connect(url)

    def command(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        self.sock.send_string(json.dumps(payload, ensure_ascii=False))
        raw = self.sock.recv()
        if isinstance(raw, bytes):
            raw = raw.decode("utf-8", errors="replace")
        try:
            reply = json.loads(raw)
        except json.JSONDecodeError:
            reply = {"_raw": raw[:4000]}
        if isinstance(reply, dict):
            reply["_transport"] = "kernel-zmq-direct"
        return reply

    def close(self) -> None:
        try:
            self.sock.close(0)
        except Exception:  # noqa: BLE001
            pass


# ---------------------------------------------------------------- VSP hub client

class HubVsp:
    def __init__(self, url: str, status_url: str):
        self.url = url
        self.status_url = status_url
        self.session_id = "session_pending"
        self.hello_reply: Dict[str, Any] = {}

    @staticmethod
    def _envelope(channel: str, message_type: str, schema: str, payload: Dict[str, Any],
                  session_id: str, role: str = "agent") -> Dict[str, Any]:
        return {
            "vsp_version": "1.0",
            "schema": schema,
            "message_id": f"msg_probe_{int(time.time() * 1000)}_{time.time_ns() % 100000}",
            "session_id": session_id,
            "client_id": "probe.tracklist",
            "role": role,
            "channel": channel,
            "type": message_type,
            "created_at": datetime.now(timezone.utc).isoformat(),
            "trace_id": f"trace_probe_{time.time_ns() % 1000000}",
            "payload": payload,
        }

    def hello(self) -> Dict[str, Any]:
        env = self._envelope("session", "session.hello", "vsp.session.hello.v1", {
            "client_name": "tracklist.probe",
            "client_version": "probe-1",
            "protocol_min": "1.0",
            "protocol_max": "1.0",
            "wants": ["command.request", "state.snapshot"],
            "transport_bindings": ["http"],
        }, "session_pending")
        reply = http_json("POST", self.url, env, timeout=30)
        sid = str(reply.get("session_id") or "").strip()
        hub = reply.get("hub") if isinstance(reply.get("hub"), dict) else {}
        if not sid and isinstance(hub, dict):
            sid = str(hub.get("session_id") or "").strip()
        if sid:
            self.session_id = sid
        self.hello_reply = {
            "session_id": self.session_id,
            "type": reply.get("type"),
            "http_status": reply.get("_http_status"),
            "hub": hub,
        }
        return self.hello_reply

    def command(self, command: str, args: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        env = self._envelope("command", "command.request", "vsp.command.request.v1", {
            "command": command,
            "args": args or {},
        }, self.session_id)
        return http_json("POST", self.url, env, timeout=120)

    def state_snapshot(self, scope: str = "project.timeline") -> Dict[str, Any]:
        env = self._envelope("state", "state.snapshot_request", "vsp.state.snapshot_request.v1",
                             {"scope": scope}, self.session_id)
        return http_json("POST", self.url, env, timeout=120)


# ---------------------------------------------------------------- summaries

def tracks_from_rows(rows: Any, id_keys: List[str]) -> List[str]:
    out: List[str] = []
    if not isinstance(rows, list):
        return out
    for row in rows:
        if not isinstance(row, dict):
            continue
        for key in id_keys:
            value = row.get(key)
            if value is not None and str(value).strip() != "":
                out.append(str(value))
                break
    return out


def summarize_kernel_list(reply: Dict[str, Any]) -> Dict[str, Any]:
    tracks = reply.get("tracks")
    ids = tracks_from_rows(tracks, ["track_id", "id"])
    return {
        "status": reply.get("status"),
        "message": reply.get("message"),
        "track_count": len(ids) if isinstance(tracks, list) else None,
        "track_ids": ids,
        "has_no_active_edit_error": "No active edit loaded" in json.dumps(reply)[:2000],
    }


def summarize_kernel_state(reply: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "status": reply.get("status"),
        "project_uuid": reply.get("project_uuid") or reply.get("project_id"),
        "project_path": reply.get("project_path"),
        "track_count_field": reply.get("track_count"),
        "has_tracks_array": isinstance(reply.get("tracks"), list),
        "revision": reply.get("project_revision") or reply.get("graph_revision"),
        "keys": sorted(reply.keys())[:30],
    }


def summarize_hub_command(reply: Dict[str, Any]) -> Dict[str, Any]:
    payload = reply.get("payload") if isinstance(reply.get("payload"), dict) else {}
    legacy = payload.get("legacy_reply") if isinstance(payload.get("legacy_reply"), dict) else {}
    ack = reply.get("ack") if isinstance(reply.get("ack"), dict) else {}
    rows = legacy.get("tracks")
    ids = tracks_from_rows(rows, ["track_id", "id"])
    return {
        "http_status": reply.get("_http_status"),
        "type": reply.get("type"),
        "ok": reply.get("type") != "command.error" and ack.get("stage") == "completed",
        "command": payload.get("command"),
        "legacy_command": payload.get("legacy_command"),
        "ack_message": ack.get("message"),
        "ack_stage": ack.get("stage"),
        "legacy_status": payload.get("legacy_status"),
        "legacy_message": legacy.get("message"),
        "legacy_track_count": len(ids) if isinstance(rows, list) else None,
        "legacy_track_ids": ids,
        "transaction_id": reply.get("transaction_id"),
        "error": reply.get("error"),
    }


def summarize_hub_state(reply: Dict[str, Any]) -> Dict[str, Any]:
    payload = reply.get("payload") if isinstance(reply.get("payload"), dict) else {}
    project = payload.get("project") if isinstance(payload.get("project"), dict) else {}
    rows = payload.get("tracks")
    ids = tracks_from_rows(rows, ["track_id", "id", "track_id"])
    return {
        "http_status": reply.get("_http_status"),
        "type": reply.get("type"),
        "status": payload.get("status"),
        "revision": reply.get("revision") if "revision" in reply else payload.get("revision"),
        "project_uuid": project.get("project_uuid"),
        "project_epoch": reply.get("project_epoch") or payload.get("project_epoch"),
        "snapshot_hash": payload.get("snapshot_hash"),
        "project_track_count": project.get("track_count"),
        "tracks_len": len(ids),
        "track_ids": ids,
        "resync": payload.get("resync"),
    }


def shadow_summary(state_reply: Dict[str, Any]) -> Dict[str, Any]:
    shadow = state_reply.get("shadow") if isinstance(state_reply.get("shadow"), dict) else {}
    tracks = shadow.get("tracks")
    ids = tracks_from_rows(tracks, ["track_id", "id", "uid"])
    return {
        "initialized": shadow.get("initialized"),
        "track_count": shadow.get("track_count"),
        "user_track_count": shadow.get("user_track_count"),
        "engine_track_count": shadow.get("engine_track_count"),
        "internal_track_count": shadow.get("internal_track_count"),
        "project_uuid": shadow.get("project_uuid"),
        "project_path": shadow.get("project_path"),
        "last_delta_seq": shadow.get("last_delta_seq"),
        "tracks_len": len(ids) if isinstance(tracks, list) else None,
        "track_ids": ids,
        "keys": sorted(shadow.keys())[:40],
    }


def tool_summary(invoke_reply: Dict[str, Any]) -> Dict[str, Any]:
    result = invoke_reply.get("result") if isinstance(invoke_reply.get("result"), dict) else {}
    tracks = result.get("tracks")
    ids = tracks_from_rows(tracks, ["track_id", "id"])
    vsp = invoke_reply.get("vsp") or result.get("vsp") or {}
    command_ack = result.get("command_ack") if isinstance(result.get("command_ack"), dict) else vsp.get("command_ack")
    ack = command_ack.get("ack") if isinstance(command_ack, dict) and isinstance(command_ack.get("ack"), dict) else {}
    observe = vsp.get("project_observe") if isinstance(vsp, dict) and isinstance(vsp.get("project_observe"), dict) else {}
    return {
        "http_status": invoke_reply.get("_http_status"),
        "status": invoke_reply.get("status") or result.get("status"),
        "error": invoke_reply.get("error"),
        "tool": invoke_reply.get("tool"),
        "command_name": invoke_reply.get("command_name"),
        "result_track_count": result.get("track_count"),
        "result_user_track_count": result.get("user_track_count"),
        "result_tracks_len": len(ids) if isinstance(tracks, list) else (0 if tracks is None else None),
        "result_track_ids": ids,
        "result_keys": sorted(result.keys())[:40],
        "vsp_ack_message": ack.get("message"),
        "vsp_command": (command_ack or {}).get("command") if isinstance(command_ack, dict) else None,
        "vsp_legacy_command": (command_ack or {}).get("legacy_command") if isinstance(command_ack, dict) else None,
        "vsp_transaction_id": (command_ack or {}).get("transaction_id") if isinstance(command_ack, dict) else None,
        "vsp_observe_project_uuid": observe.get("project", {}).get("project_uuid") if isinstance(observe.get("project"), dict) else observe.get("project_uuid"),
        "vsp_observe_revision": observe.get("revision"),
        "vsp_observe_tracks_len": len(observe.get("tracks")) if isinstance(observe.get("tracks"), list) else None,
    }


# ---------------------------------------------------------------- fixtures

FIXTURE_TEMPLATE = """<?xml version="1.0" encoding="UTF-8"?>
<EDIT appVersion="Unknown" projectID="0/{project_id}" creationTime="0" modifiedBy="tracklist_probe">
  <TRANSPORT endToEnd="1" position="0.0"/>
  <TEMPOSEQUENCE>
    <TEMPO startBeat="0.0" bpm="120.0" curve="1.0"/>
    <TIMESIG numerator="4" denominator="4" startBeat="0.0"/>
  </TEMPOSEQUENCE>
  <VIDEO/>
  <CLICKTRACK level="0.6000000238418579"/>
  <MASTERVOLUME>
    <PLUGIN type="volume" id="1001" enabled="1" volume="0.7408182206817179">
      <MODIFIERASSIGNMENTS/>
    </PLUGIN>
  </MASTERVOLUME>
  <RACKS/>
  <MASTERPLUGINS/>
  <INPUTDEVICES>
    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins"/>
  </INPUTDEVICES>
  <TRACKCOMPS/>
  <ARRANGERTRACK name="Arranger" id="1002"/>
  <CHORDTRACK name="Chord" id="1003"/>
  <MARKERTRACK name="Marker" id="1004"/>
  <TEMPOTRACK name="Global" id="1005">
    <MODIFIERS/>
  </TEMPOTRACK>
  <MASTERTRACK name="Master" id="1006">
    <MODIFIERS/>
  </MASTERTRACK>
{tracks}
  <SCENES/>
</EDIT>
"""

TRACK_TEMPLATE = """  <TRACK id="{track_id}" name="{name}">
    <MODIFIERS/>
    <OUTPUTDEVICES>
      <DEVICE name="(default audio output)"/>
    </OUTPUTDEVICES>
    <CLIPSLOTS/>
    <PLUGIN type="volume" id="{volume_id}" enabled="1" remapOnTempoChange="1">
      <MODIFIERASSIGNMENTS/>
    </PLUGIN>
    <PLUGIN type="level" id="{level_id}" enabled="1"/>
  </TRACK>"""


def write_two_track_fixture(path: Path) -> Path:
    tracks = "\n".join([
        TRACK_TEMPLATE.format(track_id=1007, name="probe_a", volume_id=1008, level_id=1009),
        TRACK_TEMPLATE.format(track_id=1010, name="probe_b", volume_id=1011, level_id=1012),
    ])
    path.write_text(FIXTURE_TEMPLATE.format(project_id="tracklistprobe1", tracks=tracks),
                    encoding="utf-8", newline="\n")
    return path


def write_sine_wav(path: Path, freq: float, amp: float, rate: int) -> Path:
    n = int(rate * DURATION_S)
    buf = bytearray()
    for i in range(n):
        v = int(max(-1.0, min(1.0, amp * math.sin(2.0 * math.pi * freq * i / rate))) * 32767.0)
        buf += struct.pack("<h", v)
    with wave.open(str(path), "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(rate)
        wf.writeframes(bytes(buf))
    return path


# ---------------------------------------------------------------- probe

class Probe:
    def __init__(self, args: argparse.Namespace):
        self.args = args
        self.artifacts = Path(args.output).resolve().parent
        self.artifacts.mkdir(parents=True, exist_ok=True)
        (self.artifacts / "raw").mkdir(exist_ok=True)
        self.kernel = KernelLink(args.kernel_url)
        self.hub = HubVsp(args.hub_url, args.hub_status_url)
        self.agent = args.agent_url.rstrip("/")
        self.summary: Dict[str, Any] = {
            "feature": "tracklist_gap_probe",
            "card": "TRACK-LIST-GAP-1",
            "started_utc": datetime.now(timezone.utc).isoformat(),
            "kernel_url": args.kernel_url,
            "hub_url": args.hub_url,
            "agent_url": self.agent,
            "kernel_pid": args.kernel_pid,
            "startup_project": args.startup_project,
            "stages": {},
            "comparisons": [],
            "findings": [],
            "pass": False,
        }
        self.tool_names: Dict[str, str] = {}
        self.sine = write_sine_wav(self.artifacts / "probe_sine.wav", SINE_FREQ, 0.1, 48000)
        self.fixture = write_two_track_fixture(self.artifacts / "fixture_open_existing_2tracks.vit")

    # -- surface capture ------------------------------------------------
    def discover_tools(self) -> None:
        data = http_get(f"{self.agent}/agent/tools")
        rows: List[Dict[str, Any]] = []
        for key in ("command_catalog", "tools"):
            value = data.get(key)
            if isinstance(value, list):
                rows.extend([row for row in value if isinstance(row, dict)])
        for row in rows:
            command = str(row.get("command_name") or row.get("CommandName") or "").strip()
            tool = str(row.get("tool_name") or row.get("ToolName") or row.get("name") or "").strip()
            for target in ("list_tracks", "get_project_state"):
                if target not in self.tool_names and (command == target or tool == target or
                                                      tool in ("track.list", "project.state") and
                                                      ((target == "list_tracks" and tool == "track.list") or
                                                       (target == "get_project_state" and tool == "project.state"))):
                    self.tool_names[target] = tool or target
        # deterministic fallbacks (the UI-facing tool names observed in the 118 round4 events)
        self.tool_names.setdefault("list_tracks", "track.list")
        self.tool_names.setdefault("get_project_state", "project.state")
        self.summary["tool_names"] = dict(self.tool_names)

    def invoke_tool(self, tool_name: str, args: Optional[Dict[str, Any]] = None,
                    confirmed: bool = False, fallbacks: Optional[List[str]] = None) -> Dict[str, Any]:
        payload: Dict[str, Any] = {"tool": tool_name, "source": "http"}
        if args:
            payload["args"] = args
        if confirmed:
            payload["confirmed"] = True
        reply = http_json("POST", f"{self.agent}/agent/invoke", payload, timeout=120)
        text = json.dumps(reply, ensure_ascii=False)[:2000].lower()
        if reply.get("status") != "ok" and ("unknown tool" in text or "unsupported tool" in text or
                                           "unknown command" in text):
            for alt in (fallbacks or []):
                if not alt or alt == tool_name:
                    continue
                payload["tool"] = alt
                reply2 = http_json("POST", f"{self.agent}/agent/invoke", payload, timeout=120)
                text2 = json.dumps(reply2, ensure_ascii=False)[:2000].lower()
                if "unknown tool" not in text2 and "unsupported tool" not in text2 and "unknown command" not in text2:
                    reply2["_tool_fallback_used"] = alt
                    return reply2
        return reply

    def capture(self, label: str) -> Dict[str, Any]:
        t_start = time.time()
        stage: Dict[str, Any] = {"label": label, "t_start_utc": datetime.now(timezone.utc).isoformat()}

        # 1 kernel direct (legacy authority, bypassing agent/hub)
        k_list = self.kernel.command({"cmd": "list_tracks"})
        stage["kernel_list_tracks"] = summarize_kernel_list(k_list)
        (self.artifacts / "raw" / f"{label}_kernel_list_tracks.json").write_text(
            json.dumps(k_list, ensure_ascii=False, indent=2), encoding="utf-8")
        k_state = self.kernel.command({"cmd": "get_project_state"})
        stage["kernel_get_project_state"] = summarize_kernel_state(k_state)
        (self.artifacts / "raw" / f"{label}_kernel_get_project_state.json").write_text(
            json.dumps(k_state, ensure_ascii=False, indent=2), encoding="utf-8")

        # 2 VSP hub canonical command project.tracks.list
        hub_list = self.hub.command("project.tracks.list")
        stage["hub_canonical_project_tracks_list"] = summarize_hub_command(hub_list)
        (self.artifacts / "raw" / f"{label}_hub_project_tracks_list.json").write_text(
            json.dumps(hub_list, ensure_ascii=False, indent=2), encoding="utf-8")

        # 3 VSP hub state snapshot
        hub_state = self.hub.state_snapshot("project.timeline")
        stage["hub_state_snapshot"] = summarize_hub_state(hub_state)
        (self.artifacts / "raw" / f"{label}_hub_state_snapshot.json").write_text(
            json.dumps(hub_state, ensure_ascii=False, indent=2), encoding="utf-8")

        # 4 agent shadow
        agent_state = http_get(f"{self.agent}/agent/state")
        stage["agent_shadow"] = shadow_summary(agent_state)
        (self.artifacts / "raw" / f"{label}_agent_state.json").write_text(
            json.dumps(agent_state, ensure_ascii=False, indent=2), encoding="utf-8")

        # 5 agent tool track.list
        track_tool = self.tool_names.get("list_tracks", "track.list")
        invoke_track = self.invoke_tool(track_tool, fallbacks=["track.list", "list_tracks", "track_list"])
        stage["agent_tool_track_list"] = tool_summary(invoke_track)
        stage["agent_tool_track_list"]["tool_used"] = track_tool
        (self.artifacts / "raw" / f"{label}_agent_invoke_track_list.json").write_text(
            json.dumps(invoke_track, ensure_ascii=False, indent=2), encoding="utf-8")

        t_end = time.time()
        stage["t_end_utc"] = datetime.now(timezone.utc).isoformat()
        stage["duration_ms"] = int((t_end - t_start) * 1000)

        comparison = self.compare(label, stage)
        stage["comparison"] = comparison
        self.summary["stages"][label] = stage
        self.summary["comparisons"].append(comparison)
        print(f"[stage] {label}: kernel={comparison['kernel_track_count']} "
              f"hub_cmd={comparison['hub_canonical_track_count']} "
              f"hub_state={comparison['hub_state_track_count']} "
              f"shadow={comparison['shadow_track_count']} "
              f"tool={comparison['tool_track_count']} "
              f"| tool==shadow:{comparison['tool_matches_shadow']} "
              f"tool==kernel:{comparison['tool_matches_kernel']}")
        return stage

    @staticmethod
    def compare(label: str, stage: Dict[str, Any]) -> Dict[str, Any]:
        kernel_ids = stage["kernel_list_tracks"]["track_ids"]
        hub_cmd = stage["hub_canonical_project_tracks_list"]
        hub_state = stage["hub_state_snapshot"]
        shadow = stage["agent_shadow"]
        tool = stage["agent_tool_track_list"]
        tool_ids = tool["result_track_ids"] or []
        shadow_ids = shadow["track_ids"] or []
        shadow_count = shadow["track_count"]
        tool_count = tool["result_track_count"]
        if tool_count is None:
            tool_count = len(tool_ids)
        return {
            "label": label,
            "kernel_track_count": len(kernel_ids),
            "kernel_track_ids": kernel_ids,
            "kernel_project_uuid": stage["kernel_get_project_state"]["project_uuid"],
            "hub_canonical_track_count": hub_cmd["legacy_track_count"],
            "hub_canonical_track_ids": hub_cmd["legacy_track_ids"],
            "hub_canonical_vsp_ack_message": hub_cmd["ack_message"],
            "hub_state_track_count": hub_state["tracks_len"],
            "hub_state_track_ids": hub_state["track_ids"],
            "hub_state_project_uuid": hub_state["project_uuid"],
            "hub_state_revision": hub_state["revision"],
            "shadow_track_count": shadow_count,
            "shadow_tracks_len": shadow["tracks_len"],
            "shadow_track_ids": shadow_ids,
            "shadow_project_uuid": shadow["project_uuid"],
            "tool_track_count": tool_count,
            "tool_track_ids": tool_ids,
            "tool_vsp_ack_message": tool["vsp_ack_message"],
            "tool_vsp_legacy_command": tool["vsp_legacy_command"],
            "tool_status": tool["status"],
            "tool_matches_shadow": bool(tool_ids == shadow_ids and (tool_count == shadow_count)),
            "tool_matches_kernel": bool(tool_ids == kernel_ids),
            "hub_matches_kernel": bool((hub_cmd["legacy_track_ids"] or []) == kernel_ids),
            "hub_state_matches_kernel": bool((hub_state["track_ids"] or []) == kernel_ids),
            "same_project_uuid_across_surfaces": self_same_uuid(stage),
        }

    # -- mutations (external path == GUI/kernel-direct path) ------------
    def external_new_project_and_import(self, track_count: int = 6) -> Dict[str, Any]:
        log: Dict[str, Any] = {"op": "external_new_project_and_import", "steps": []}
        reply = self.kernel.command({"cmd": "new_project"})
        log["steps"].append({"cmd": "new_project", "status": reply.get("status"), "message": reply.get("message")})
        track_ids: List[str] = []
        for _ in range(track_count):
            reply = self.kernel.command({"cmd": "add_track"})
            tid = str(reply.get("track_id") or "").strip()
            track_ids.append(tid)
            log["steps"].append({"cmd": "add_track", "track_id": tid, "status": reply.get("status")})
        import_reply = self.kernel.command({
            "cmd": "import_audio", "file_path": str(self.sine),
            "track_id": track_ids[0] if track_ids else "",
        })
        log["steps"].append({"cmd": "import_audio", "status": import_reply.get("status"),
                             "clip_id": import_reply.get("clip_id")})
        log["track_ids"] = track_ids
        return log

    def external_open_existing(self) -> Dict[str, Any]:
        live = self.artifacts / "fixture_open_existing_live.vit"
        live.write_bytes(self.fixture.read_bytes())
        reply = self.kernel.command({"cmd": "load_project", "file_path": str(live)})
        return {"op": "external_open_existing", "fixture": str(live),
                "status": reply.get("status"), "message": reply.get("message")}

    def agent_path_mutation(self) -> Dict[str, Any]:
        out: Dict[str, Any] = {"op": "agent_path_mutation"}
        auth = http_json("POST", f"{self.agent}/agent/authority",
                         {"authority_mode": "full_project_access"})
        out["authority"] = {"status": auth.get("status"), "authority_mode": auth.get("authority_mode"),
                            "error": auth.get("error")}
        tool = "add_track"
        out["tool"] = tool
        reply = self.invoke_tool(tool, confirmed=True)
        out["invoke"] = {"status": reply.get("status"), "error": reply.get("error"),
                         "requires_confirmation": reply.get("requires_confirmation"),
                         "command_name": reply.get("command_name"),
                         "result": reply.get("result") if not isinstance(reply.get("result"), dict)
                         else {k: reply["result"].get(k) for k in list(reply["result"].keys())[:10]}}
        return out

    def refresh_shadow_via_project_state(self, label: str) -> Dict[str, Any]:
        tool = self.tool_names.get("get_project_state", "project.state")
        reply = self.invoke_tool(tool, fallbacks=["project.state", "get_project_state", "project_state"])
        out = tool_summary(reply)
        out["tool_used"] = tool
        (self.artifacts / "raw" / f"{label}_agent_invoke_project_state.json").write_text(
            json.dumps(reply, ensure_ascii=False, indent=2), encoding="utf-8")
        self.summary["stages"].setdefault(label, {})["agent_tool_project_state"] = out
        print(f"[stage] {label}: project.state tool -> status={out['status']} "
              f"tracks={out['result_track_count']} observe_uuid={out['vsp_observe_project_uuid']}")
        return out

    # -- run ------------------------------------------------------------
    def run(self) -> int:
        try:
            ping = self.kernel.command({"cmd": "ping"})
            self.summary["kernel_ping"] = {"status": ping.get("status")}
            health = http_get(f"{self.agent}/health", timeout=10)
            self.summary["agent_health"] = health
            hub_status = http_get(self.args.hub_status_url.replace("/vsp/status", "/vsp/status"), timeout=10)
            self.summary["hub_status"] = {
                "status": hub_status.get("status"),
                "pid": hub_status.get("pid"),
                "started_at": hub_status.get("started_at"),
                "kernel_req": hub_status.get("kernel_req"),
                "kernel_sub": hub_status.get("kernel_sub"),
                "kernel": hub_status.get("kernel"),
                "session_count": hub_status.get("session_count"),
                "requests": hub_status.get("requests"),
                "errors": hub_status.get("errors"),
            }
            self.summary["hub_hello"] = self.hub.hello()
            self.discover_tools()

            # S0 baseline
            self.capture("S0_fresh_baseline")

            # S1 external new_project + import (the GUI/external mutation path)
            self.summary["mutation_S1"] = self.external_new_project_and_import(6)
            time.sleep(0.5)
            self.capture("S1_external_new_import")

            # S1b shadow refresh through the agent project.state tool (the self-heal path)
            self.refresh_shadow_via_project_state("S1b")
            time.sleep(0.3)
            self.capture("S1b_after_project_state_refresh")

            # S2 external open-existing (same external path class, different project origin)
            self.summary["mutation_S2"] = self.external_open_existing()
            time.sleep(0.5)
            self.capture("S2_external_open_existing")

            # S3 agent-path mutation (isolated; only when authority + tool available)
            try:
                self.summary["mutation_S3"] = self.agent_path_mutation()
                time.sleep(0.5)
                self.capture("S3_agent_path_mutation")
            except Exception as exc:  # noqa: BLE001
                self.summary["mutation_S3"] = {"op": "agent_path_mutation", "error": f"{type(exc).__name__}: {exc}"}
                print(f"[stage] S3 skipped/failed: {exc}")

            self.analyze()
        except Exception as exc:  # noqa: BLE001
            self.summary["error"] = f"{type(exc).__name__}: {exc}"
            print(f"FATAL: {self.summary['error']}", file=sys.stderr)
        finally:
            self.summary["finished_utc"] = datetime.now(timezone.utc).isoformat()
            Path(self.args.output).write_text(
                json.dumps(self.summary, ensure_ascii=False, indent=2), encoding="utf-8")
            self.kernel.close()
        print("tracklist gap probe: " + ("PASS" if self.summary.get("pass") else "FAIL"))
        return 0 if self.summary.get("pass") else 1

    def analyze(self) -> None:
        stages = self.summary["stages"]
        comps = {row["label"]: row for row in self.summary["comparisons"]}
        findings: List[str] = []

        s1 = comps.get("S1_external_new_import")
        s2 = comps.get("S2_external_open_existing")
        s1b = comps.get("S1b_after_project_state_refresh")

        # Candidate 1: VSP channel instance mismatch -> all three kernel-facing surfaces
        # must agree on track ids/project uuid in every stage.
        vsp_same = True
        uuid_same = True
        for label, comp in comps.items():
            if not comp.get("hub_matches_kernel") or not comp.get("hub_state_matches_kernel"):
                vsp_same = False
            if not comp.get("same_project_uuid_across_surfaces"):
                uuid_same = False
        findings.append(
            "candidate 1 (VSP channel hits a different kernel/edit instance): "
            + ("RULED OUT" if (vsp_same and uuid_same) else "NOT ruled out")
            + f" 鈥?kernel/hub-canonical/hub-state track ids agree in all stages={vsp_same}, "
              f"project_uuid agrees across surfaces={uuid_same}")
        self.summary["candidate1_vsp_instance"] = {
            "verdict": "ruled_out" if (vsp_same and uuid_same) else "not_ruled_out",
            "kernel_hub_track_ids_agree": vsp_same,
            "project_uuid_agree": uuid_same,
        }

        # Candidate 2: new+import path specifically -> S2 external open-existing must show
        # the same tool-vs-kernel gap as S1.
        gap_s1 = bool(s1 and s1["tool_matches_kernel"] is False)
        gap_s2 = bool(s2 and s2["tool_matches_kernel"] is False)
        same_gap = gap_s1 and gap_s2
        findings.append(
            "candidate 2 (new+import path track enumeration gap): "
            + ("RULED OUT as path-specific" if same_gap else "needs review")
            + f" 鈥?tool!=kernel in external new+import={gap_s1}, in external open-existing={gap_s2}")
        self.summary["candidate2_new_import_path"] = {
            "verdict": "ruled_out_as_path_specific" if same_gap else "needs_review",
            "tool_gap_external_new_import": gap_s1,
            "tool_gap_external_open_existing": gap_s2,
        }

        # Root cause face: the track.list tool result is the shadow Summary, not the kernel reply.
        tool_is_shadow = all(
            comp["tool_matches_shadow"] for comp in comps.values()
        )
        gap_stages = [label for label, comp in comps.items() if not comp["tool_matches_kernel"]]
        findings.append(
            "root-cause face (agent tool result = shadow summary, not the kernel reply): "
            + ("CONFIRMED" if tool_is_shadow else "not confirmed")
            + f" 鈥?tool==shadow in all stages={tool_is_shadow}; tool!=kernel in stages={gap_stages}")
        self.summary["root_cause_tool_is_shadow"] = {
            "tool_matches_shadow_all_stages": tool_is_shadow,
            "tool_mismatch_kernel_stages": gap_stages,
        }

        # Self-heal: after project.state refresh, shadow (and tool) catches up with the kernel.
        heal = bool(s1b and s1b["tool_matches_kernel"])
        findings.append(
            "self-heal face (get_project_state refreshes shadow from the VSP state snapshot): "
            + ("CONFIRMED" if heal else "not confirmed")
            + f" 鈥?after project.state tool: tool==kernel={heal}")
        self.summary["self_heal_after_project_state"] = {"tool_matches_kernel": heal}

        # kernel reply itself was correct in the same response: agent tool response carries a
        # VSP command ack (kernel was queried) while its tracks came from shadow.
        ack_present = [label for label, comp in comps.items()
                       if comp.get("tool_vsp_ack_message")]
        findings.append(
            f"same-frame chain: agent track.list responses carry kernel VSP acks in stages={ack_present} "
            "while their track rows equal the shadow summary")
        self.summary["same_frame_chain"] = {"stages_with_vsp_ack": ack_present}

        self.summary["findings"] = findings
        self.summary["pass"] = bool(
            self.summary["candidate1_vsp_instance"]["verdict"] == "ruled_out"
            and self.summary["candidate2_new_import_path"]["verdict"] == "ruled_out_as_path_specific"
            and tool_is_shadow
            and heal
            and gap_stages
        )
        print("\n".join(findings))


def self_same_uuid(stage: Dict[str, Any]) -> bool:
    kernel_uuid = (stage["kernel_get_project_state"].get("project_uuid") or "").strip()
    hub_uuid = (stage["hub_state_snapshot"].get("project_uuid") or "").strip()
    shadow_uuid = (stage["agent_shadow"].get("project_uuid") or "").strip()
    known = [u for u in (kernel_uuid, hub_uuid, shadow_uuid) if u]
    if len(known) < 2:
        return False
    return len(set(known)) == 1


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--kernel-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--hub-url", default="http://127.0.0.1:8787/vsp")
    parser.add_argument("--hub-status-url", default="http://127.0.0.1:8787/vsp/status")
    parser.add_argument("--agent-url", default="http://127.0.0.1:7878")
    parser.add_argument("--output", required=True)
    parser.add_argument("--kernel-pid", type=int, default=0)
    parser.add_argument("--startup-project", default="")
    args = parser.parse_args()
    probe = Probe(args)
    return probe.run()


if __name__ == "__main__":
    sys.exit(main())
