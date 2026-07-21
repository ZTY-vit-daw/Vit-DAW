"""Smoke-test B1 fader unity reset through the agent confirmation path."""

from __future__ import annotations

import argparse
import json
import math
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


def post_json(base_url: str, path: str, payload: dict[str, Any], timeout: float) -> dict[str, Any]:
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return parse_json_response(resp.read(), path)


def get_json(base_url: str, path: str, timeout: float) -> dict[str, Any]:
    with urllib.request.urlopen(base_url.rstrip("/") + path, timeout=timeout) as resp:
        return parse_json_response(resp.read(), path)


def parse_json_response(raw: bytes, label: str) -> dict[str, Any]:
    text = raw.decode("utf-8", errors="replace")
    parsed = json.loads(text)
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{label} returned non-object JSON: {text[:500]!r}")
    return parsed


def wait_http(base_url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = get_json(base_url, "/health", 2.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"agent HTTP did not become ready at {base_url}: {last_error}")


def require_ok(resp: dict[str, Any], label: str) -> dict[str, Any]:
    status = str(resp.get("status", "")).lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label} failed: {json.dumps(resp, ensure_ascii=False)[:1200]}")
    return resp


def invoke(base_url: str, tool: str, args: dict[str, Any] | None = None, *, confirmed: bool = True, timeout: float = 60.0) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args or {},
        "source": "b1_group_reset_agent_smoke",
    }
    if confirmed:
        payload["confirmed"] = True
    return post_json(base_url, "/agent/invoke", payload, timeout)


def result_map(resp: dict[str, Any]) -> dict[str, Any]:
    result = resp.get("result")
    return result if isinstance(result, dict) else {}


def read_track_id(resp: dict[str, Any]) -> str:
    result = result_map(resp)
    for key in ("track_id", "id", "item_id"):
        value = str(result.get(key, "")).strip()
        if value:
            return value
    raise RuntimeError(f"response has no track_id: {json.dumps(resp, ensure_ascii=False)[:800]}")


def volume_db(row: dict[str, Any]) -> float:
    for key in ("volume_db", "fader_db", "gain_db", "db"):
        if key not in row:
            continue
        try:
            value = float(row[key])
        except (TypeError, ValueError):
            continue
        if math.isfinite(value):
            return value
    raise RuntimeError(f"track row has no finite volume db: {row!r}")


def tracks_by_id(state: dict[str, Any]) -> dict[str, dict[str, Any]]:
    result = result_map(state)
    rows = result.get("tracks", [])
    if not isinstance(rows, list):
        return {}
    out: dict[str, dict[str, Any]] = {}
    for row in rows:
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id", row.get("id", ""))).strip()
        if track_id:
            out[track_id] = row
    return out


def response_tool_names(resp: dict[str, Any]) -> list[str]:
    names: list[str] = []
    for item in resp.get("tool_route", []):
        text = str(item).strip()
        if text:
            names.append(text)
    for record in resp.get("executed", []):
        if not isinstance(record, dict):
            continue
        for key in ("tool", "command_name"):
            text = str(record.get(key, "")).strip()
            if text:
                names.append(text)
    return names


def start_process(path: Path, args: list[str], cwd: Path) -> subprocess.Popen[Any]:
    creationflags = 0
    if sys.platform == "win32":
        creationflags = getattr(subprocess, "CREATE_NO_WINDOW", 0)
    return subprocess.Popen([str(path), *args], cwd=str(cwd), creationflags=creationflags)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--kernel-exe", default="")
    parser.add_argument("--agent-exe", default="")
    parser.add_argument("--agent-http", default="http://127.0.0.1:17878")
    parser.add_argument("--agent-http-addr", default="127.0.0.1:17878")
    parser.add_argument("--timeout-sec", type=float, default=90.0)
    parser.add_argument("--reuse", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    kernel = Path(args.kernel_exe) if args.kernel_exe else repo / "VitApp" / "build_release" / "VitApp_artefacts" / "Release" / "VitApp.exe"
    agent = Path(args.agent_exe) if args.agent_exe else repo / "agent" / "bin" / "VitAgent.exe"
    logs = repo / "VitApp" / "Workspace" / "Logs"
    logs.mkdir(parents=True, exist_ok=True)
    agent_log = logs / "b1_group_reset_agent_smoke_agent.log"

    if not kernel.exists():
        raise RuntimeError(f"kernel exe not found: {kernel}")
    if not agent.exists():
        raise RuntimeError(f"agent exe not found: {agent}")

    processes: list[subprocess.Popen[Any]] = []
    try:
        if not args.reuse:
            processes.append(start_process(kernel, [], kernel.parent))
            time.sleep(2.0)
            processes.append(
                start_process(
                    agent,
                    [
                        "-http",
                        args.agent_http_addr,
                        "-udp-to-godot",
                        "14444",
                        "-udp-from-godot",
                        "14445",
                        "-vsp-hub-url",
                        "",
                        "-last-log-path",
                        str(agent_log),
                        "-keep-last-log-lines",
                        "1000",
                    ],
                    agent.parent.parent,
                )
            )
        wait_http(args.agent_http, args.timeout_sec)

        require_ok(invoke(args.agent_http, "project.new", {}, confirmed=True, timeout=args.timeout_sec), "project.new")
        track_ids: list[str] = []
        for _ in range(3):
            track_ids.append(read_track_id(require_ok(invoke(args.agent_http, "track.add_audio", {}, confirmed=True), "track.add_audio")))

        for track_id, db in zip(track_ids, (-60.0, -18.0, 0.0)):
            require_ok(invoke(args.agent_http, "track.volume", {"track_id": track_id, "db": db}, confirmed=True), f"track.volume {track_id}")

        conversation_id = f"b1_group_reset_smoke_{int(time.time())}"
        pending = post_json(
            args.agent_http,
            "/agent/chat",
            {
                "conversation_id": conversation_id,
                "message": "B1 第一步，把所有轨道推子固定在 0 dB",
                "context": {"agent_mode": "chat"},
            },
            args.timeout_sec,
        )
        stop_reason = str(pending.get("stop_reason", ""))
        needs_confirmation = bool(pending.get("needs_confirmation"))
        plan_id = str(pending.get("plan_id", "")).strip()
        if stop_reason != "needs_confirmation" and not needs_confirmation:
            raise RuntimeError(f"B1 reset did not create pending confirmation: {json.dumps(pending, ensure_ascii=False)[:1500]}")
        if not plan_id:
            raise RuntimeError(f"B1 reset pending response has no plan_id: {json.dumps(pending, ensure_ascii=False)[:1500]}")
        tool_route = response_tool_names(pending)
        pending_json = json.dumps(pending, ensure_ascii=False)
        if "track.group.apply_control" not in tool_route and "track.group.apply_control" not in pending_json:
            raise RuntimeError(f"B1 reset route did not include track.group.apply_control: {tool_route!r}")
        if any("mix.apply_tick" == item for item in tool_route) or "mix.apply_tick" in pending_json:
            raise RuntimeError(f"B1 reset incorrectly used mix.apply_tick route: {tool_route!r}")

        confirmed = post_json(args.agent_http, "/agent/confirm", {"plan_id": plan_id, "decision": "approve"}, args.timeout_sec)
        if str(confirmed.get("status", "")).lower() != "ok" or str(confirmed.get("goal_status", "")).lower() != "completed":
            raise RuntimeError(f"B1 reset confirmation failed: {json.dumps(confirmed, ensure_ascii=False)[:1500]}")

        state = require_ok(invoke(args.agent_http, "project.state", {}, confirmed=True), "project.state after reset")
        rows = tracks_by_id(state)
        for track_id in track_ids:
            row = rows.get(track_id)
            if row is None:
                raise RuntimeError(f"track missing after reset: {track_id}")
            db = volume_db(row)
            if abs(db) > 0.05:
                raise RuntimeError(f"track {track_id} was not reset to 0 dB, got {db:.3f}")

        print(
            json.dumps(
                {
                    "status": "ok",
                    "conversation_id": conversation_id,
                    "plan_id": plan_id,
                    "track_ids": track_ids,
                    "tool_route": tool_route,
                    "checked": [
                        "agent_chat_b1_pending_confirmation",
                        "track_group_apply_control_absolute_0db_route",
                        "no_mix_apply_tick_for_b1_reset",
                        "agent_confirm_completed",
                        "project_state_all_member_faders_0db",
                    ],
                },
                ensure_ascii=False,
            )
        )
        return 0
    finally:
        if not args.reuse:
            for proc in reversed(processes):
                if proc.poll() is None:
                    proc.terminate()
            deadline = time.time() + 5.0
            for proc in reversed(processes):
                while proc.poll() is None and time.time() < deadline:
                    time.sleep(0.1)
                if proc.poll() is None:
                    proc.kill()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (urllib.error.URLError, RuntimeError, AssertionError, json.JSONDecodeError) as exc:
        print(f"b1_group_reset_agent_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
