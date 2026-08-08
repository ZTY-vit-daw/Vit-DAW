#!/usr/bin/env python3
"""Read-only FabFilter/Waves multiband census on disposable tracks.

The only project mutations are creation/deletion of disposable tracks and
loading the requested plugin. No plugin parameter write command is issued.
Sealed cases are intentionally recorded without opening the parameter surface.
"""
from __future__ import annotations

import argparse
import json
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

import zmq


def request_json(url: str, payload: dict[str, Any], timeout: float) -> dict[str, Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=body, headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as error:
        value = json.loads(error.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError("agent returned a non-object response")
    return value


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    return request_json(base.rstrip("/") + "/agent/invoke", {"tool": tool, "args": args, "confirmed": confirmed, "source": "multiband_matrix_smoke"}, timeout)


def result(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result", response)
    return value if isinstance(value, dict) else {}


def text(row: dict[str, Any], *keys: str) -> str:
    for key in keys:
        value = row.get(key)
        if value is not None and str(value).strip():
            return str(value).strip()
    return ""


def kernel(command: dict[str, Any], timeout: float) -> dict[str, Any]:
    context = zmq.Context.instance()
    socket = context.socket(zmq.REQ)
    socket.setsockopt(zmq.RCVTIMEO, max(1000, int(timeout * 1000)))
    socket.setsockopt(zmq.SNDTIMEO, max(1000, int(timeout * 1000)))
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect("tcp://127.0.0.1:5555")
    try:
        socket.send_json(command)
        value = socket.recv_json()
    finally:
        socket.close()
    if not isinstance(value, dict):
        raise RuntimeError("kernel returned a non-object response")
    return value


def require(response: dict[str, Any], label: str) -> dict[str, Any]:
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label}: {response.get('error', response)}")
    value = result(response)
    if str(value.get("status", "ok")).lower() in {"error", "failed", "rejected"}:
        raise RuntimeError(f"{label}: {value.get('message', value)}")
    return value


def resolve(case: dict[str, Any], timeout: float) -> tuple[str, dict[str, Any]]:
    name = text(case, "plugin_name")
    configured = text(case, "plugin_path").casefold()
    reply = kernel({"cmd": "plugin_search", "query": name, "limit": 32}, timeout)
    rows = next((value for key, value in reply.items() if key in {"plugins", "entries", "results"} and isinstance(value, list)), [])
    named = [row for row in rows if isinstance(row, dict) and text(row, "name", "plugin_name").casefold() == name.casefold()]
    exact = [row for row in named if text(row, "file_or_identifier", "plugin_path", "path").casefold() == configured]
    candidates = exact or named
    unique = {text(row, "identifier", "plugin_identifier"): row for row in candidates if text(row, "identifier", "plugin_identifier")}
    if len(unique) != 1:
        raise RuntimeError(f"plugin_search did not uniquely resolve {name!r}: {list(unique)}")
    identifier = next(iter(unique))
    return identifier, unique[identifier]


def capture(base: str, case: dict[str, Any], output: Path, timeout: float, sealed: bool) -> dict[str, Any]:
    case_id = text(case, "id")
    if sealed:
        return {"id": case_id, "status": "sealed_not_opened"}
    track_id = ""
    try:
        track = require(invoke(base, "track.add_audio", {"name": f"Multiband census {case_id}"}, timeout, True), f"{case_id}:track.add_audio")
        track_id = text(track, "track_id", "id")
        identifier, resolution = resolve(case, timeout)
        loaded = require(invoke(base, "plugin.load_to_rack", {"track_id": track_id, "plugin_path": text(case, "plugin_path"), "plugin_name": text(case, "plugin_name"), "plugin_identifier": identifier}, timeout, True), f"{case_id}:plugin.load_to_rack")
        plugin_id = text(loaded, "plugin_id", "node_id", "id")
        time.sleep(0.35)
        row: dict[str, Any] = {"case": case, "plugin_identifier": identifier, "plugin_resolution": resolution}
        parameters = require(invoke(base, "plugin.get_parameters", {"track_id": track_id, "plugin_id": plugin_id, "plugin_identifier": identifier, "include_parameters": True}, timeout), f"{case_id}:plugin.get_parameters")
        row["parameters"] = parameters
        inspection = invoke(base, "plugin_grabber.inspect_multiband", {"track_id": track_id, "plugin_id": plugin_id}, timeout)
        row["inspect_response"] = inspection
        row["inspect"] = result(inspection)
        row["status"] = "captured"
        (output / f"{case_id}.json").write_text(json.dumps(row, ensure_ascii=False, indent=2), encoding="utf-8")
        return {"id": case_id, "status": "captured", "classification": text(row["inspect"], "classification"), "boundary": text(row["inspect"], "code")}
    finally:
        if track_id:
            require(invoke(base, "track.delete", {"track_id": track_id}, timeout, True), f"{case_id}:track.delete")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--manifest", type=Path, default=Path("scripts/multiband_control_matrix.json"))
    parser.add_argument("--output", type=Path, default=Path("temp/multiband-census"))
    parser.add_argument("--timeout", type=float, default=30)
    args = parser.parse_args()
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    args.output.mkdir(parents=True, exist_ok=True)
    report: dict[str, Any] = {"schema_version": "plugin_grabber.multiband_dynamics_census.v1", "cases": []}
    for group in ("training", "regression", "negative_regression", "sealed_blind"):
        for case in manifest.get(group, []):
            try:
                report["cases"].append({"group": group, **capture(args.agent_http, case, args.output, args.timeout, group == "sealed_blind")})
            except Exception as error:  # continue so one unavailable Waves shell member does not hide the rest
                report["cases"].append({"group": group, "id": text(case, "id"), "status": "unresolved", "error": str(error)})
    (args.output / "summary.json").write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
