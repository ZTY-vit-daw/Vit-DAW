"""Smoke-test B1.2 source-level clip gain calibration through agent chat."""

from __future__ import annotations

import argparse
import json
import math
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request
import wave
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
        raise RuntimeError(f"{label} failed: {json.dumps(resp, ensure_ascii=False)[:1500]}")
    return resp


def invoke(base_url: str, tool: str, args: dict[str, Any] | None = None, *, confirmed: bool = True, timeout: float = 60.0) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args or {},
        "source": "b1_2_source_calibration_agent_smoke",
    }
    if confirmed:
        payload["confirmed"] = True
    return post_json(base_url, "/agent/invoke", payload, timeout)


def result_map(resp: dict[str, Any]) -> dict[str, Any]:
    result = resp.get("result")
    return result if isinstance(result, dict) else {}


def read_id(resp: dict[str, Any], *keys: str) -> str:
    result = result_map(resp)
    for key in keys:
        value = str(result.get(key, "")).strip()
        if value:
            return value
    raise RuntimeError(f"response has none of {keys}: {json.dumps(resp, ensure_ascii=False)[:1000]}")


def tracks_by_id(state: dict[str, Any]) -> dict[str, dict[str, Any]]:
    rows = result_map(state).get("tracks", [])
    out: dict[str, dict[str, Any]] = {}
    if not isinstance(rows, list):
        return out
    for row in rows:
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id", row.get("id", ""))).strip()
        if track_id:
            out[track_id] = row
    return out


def clip_gain_db(track: dict[str, Any], clip_id: str) -> float:
    for clip in track.get("clips", []):
        if not isinstance(clip, dict):
            continue
        current_id = str(clip.get("clip_id", clip.get("id", ""))).strip()
        if current_id != clip_id:
            continue
        for key in ("clip_gain_db", "gain_db", "db"):
            if key not in clip:
                continue
            try:
                value = float(clip[key])
            except (TypeError, ValueError):
                continue
            if math.isfinite(value):
                return value
    raise RuntimeError(f"clip {clip_id} gain not found in track row: {track!r}")


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


def write_sine_wav(path: Path, frequency: float, amplitude: float, duration_sec: float = 0.75, sample_rate: int = 48000) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    frames = int(duration_sec * sample_rate)
    with wave.open(str(path), "wb") as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(sample_rate)
        payload = bytearray()
        for i in range(frames):
            sample = amplitude * math.sin(2.0 * math.pi * frequency * (i / sample_rate))
            payload += struct.pack("<h", max(-32767, min(32767, int(sample * 32767))))
        wav.writeframes(bytes(payload))


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
    parser.add_argument("--agent-http", default="http://127.0.0.1:17879")
    parser.add_argument("--agent-http-addr", default="127.0.0.1:17879")
    parser.add_argument("--timeout-sec", type=float, default=120.0)
    parser.add_argument("--reuse", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    kernel = Path(args.kernel_exe) if args.kernel_exe else repo / "VitApp" / "build_release" / "VitApp_artefacts" / "Release" / "VitApp.exe"
    agent = Path(args.agent_exe) if args.agent_exe else repo / "agent" / "bin" / "VitAgent.exe"
    run_id = int(time.time())
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b1_2_source_calibration_{run_id}"
    logs = repo / "VitApp" / "Workspace" / "Logs"
    logs.mkdir(parents=True, exist_ok=True)
    artifact_dir.mkdir(parents=True, exist_ok=True)
    agent_log = logs / "b1_2_source_calibration_agent_smoke_agent.log"
    feature_snapshot_path = artifact_dir / "mixboard_feature_snapshot.json"
    acoustic_status_path = artifact_dir / "acoustic_package_status.json"

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

        material = [
            ("quiet", 220.0, 0.03, -30.0, -12.0),
            ("reference_a", 330.0, 0.10, -20.0, -8.0),
            ("reference_b", 440.0, 0.10, -20.0, -8.0),
        ]
        wav_paths: dict[str, Path] = {}
        for name, hz, amp, _rms, _peak in material:
            path = artifact_dir / f"{name}.wav"
            write_sine_wav(path, hz, amp)
            wav_paths[name] = path

        require_ok(invoke(args.agent_http, "project.new", {}, confirmed=True, timeout=args.timeout_sec), "project.new")

        tracks: dict[str, dict[str, Any]] = {}
        for name, _hz, _amp, rms_dbfs, peak_dbfs in material:
            track_id = read_id(require_ok(invoke(args.agent_http, "track.add_audio", {}, confirmed=True), f"track.add_audio {name}"), "track_id", "id")
            require_ok(invoke(args.agent_http, "track.volume", {"track_id": track_id, "db": 0.0}, confirmed=True), f"track.volume {name}")
            imported = require_ok(
                invoke(
                    args.agent_http,
                    "clip.import_media_to_track",
                    {"track_id": track_id, "file_path": str(wav_paths[name]), "start_time": 0.0},
                    confirmed=True,
                    timeout=args.timeout_sec,
                ),
                f"clip.import_media_to_track {name}",
            )
            clip_id = read_id(imported, "clip_id", "id", "item_id")
            tracks[name] = {
                "track_id": track_id,
                "clip_id": clip_id,
                "clip_name": wav_paths[name].name,
                "path": wav_paths[name],
                "rms_dbfs": rms_dbfs,
                "peak_dbfs": peak_dbfs,
            }

        session_id = f"b1_2_source_calibration_{int(time.time())}"
        snapshot_rows = []
        for name, data in tracks.items():
            snapshot_rows.append(
                {
                    "status": "ready",
                    "feature_type": "waveform_envelope",
                    "project_id": "current",
                    "session_id": session_id,
                    "track_id": data["track_id"],
                    "clip_id": data["clip_id"],
                    "clip_name": data["clip_name"],
                    "file_path": str(data["path"]),
                    "source_path": str(data["path"]),
                    "source_revision": f"rev_{name}",
                    "duration_seconds": 0.75,
                    "peak_dbfs": data["peak_dbfs"],
                    "rms_dbfs": data["rms_dbfs"],
                    "headroom_db": -data["peak_dbfs"],
                    "crest_db": data["peak_dbfs"] - data["rms_dbfs"],
                }
            )
        feature_snapshot = {
            "schema_version": "mixboard_feature_snapshot.v1",
            "updated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "track_waveform_envelopes": snapshot_rows,
            "waveform_envelope": snapshot_rows[0],
        }
        feature_snapshot_path.write_text(json.dumps(feature_snapshot, ensure_ascii=False, indent=2), encoding="utf-8")

        conversation_id = f"b1_2_source_calibration_smoke_{int(time.time())}"
        pending = post_json(
            args.agent_http,
            "/agent/chat",
            {
                "conversation_id": conversation_id,
                "message": "继续做 B1.2 电平证据检查和源素材校准",
                "context": {
                    "agent_mode": "chat",
                    "feature_snapshot_path": str(feature_snapshot_path),
                    "acoustic_package_status_path": str(acoustic_status_path),
                    "mix_session_id": session_id,
                },
            },
            args.timeout_sec,
        )
        (artifact_dir / "pending_response.json").write_text(json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8")
        stop_reason = str(pending.get("stop_reason", ""))
        needs_confirmation = bool(pending.get("needs_confirmation"))
        plan_id = str(pending.get("plan_id", "")).strip()
        if stop_reason != "needs_confirmation" and not needs_confirmation:
            raise RuntimeError(f"B1.2 did not create pending confirmation: {json.dumps(pending, ensure_ascii=False)[:1800]}")
        if not plan_id:
            raise RuntimeError(f"B1.2 pending response has no plan_id: {json.dumps(pending, ensure_ascii=False)[:1800]}")
        pending_json = json.dumps(pending, ensure_ascii=False)
        if "clip.gain.set" not in pending_json or "b1_2_source_calibration" not in pending_json:
            raise RuntimeError(f"B1.2 pending route did not include source clip.gain.set: {pending_json[:1800]}")
        if "project.audio_analysis_status" not in pending_json:
            raise RuntimeError(f"B1.2 pending route did not read project.audio_analysis_status: {pending_json[:1800]}")
        for forbidden in ("mix.apply_tick", "mix.propose_tick", "track.group.apply_control"):
            if forbidden in pending_json:
                raise RuntimeError(f"B1.2 incorrectly used {forbidden}: {pending_json[:1800]}")

        confirmed = post_json(args.agent_http, "/agent/confirm", {"plan_id": plan_id, "decision": "approve"}, args.timeout_sec)
        if str(confirmed.get("status", "")).lower() != "ok" or str(confirmed.get("goal_status", "")).lower() != "completed":
            raise RuntimeError(f"B1.2 confirmation failed: {json.dumps(confirmed, ensure_ascii=False)[:1800]}")
        confirmed_json = json.dumps(confirmed, ensure_ascii=False)
        for required in ("project.state", "mix.observe", "clip gain"):
            if required not in confirmed_json:
                raise RuntimeError(f"B1.2 confirmation missing {required}: {confirmed_json[:1800]}")

        state = require_ok(invoke(args.agent_http, "project.state", {}, confirmed=True), "project.state after B1.2")
        rows = tracks_by_id(state)
        quiet_track = rows.get(str(tracks["quiet"]["track_id"]))
        if quiet_track is None:
            raise RuntimeError("quiet track missing after B1.2")
        gain = clip_gain_db(quiet_track, str(tracks["quiet"]["clip_id"]))
        if abs(gain - 10.0) > 0.05:
            raise RuntimeError(f"quiet clip gain was not calibrated to +10 dB, got {gain:.3f}")

        print(
            json.dumps(
                {
                    "status": "ok",
                    "conversation_id": conversation_id,
                    "plan_id": plan_id,
                    "quiet_track_id": tracks["quiet"]["track_id"],
                    "quiet_clip_id": tracks["quiet"]["clip_id"],
                    "checked": [
                        "agent_chat_b1_2_pending_confirmation",
                        "clip_gain_set_source_calibration_route",
                        "no_track_group_or_mix_tick_for_b1_2",
                        "agent_confirm_completed",
                        "project_state_clip_gain_verified",
                        "pending_route_reads_audio_analysis_status",
                        "confirm_reply_mentions_project_state_and_mix_observe",
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
        print(f"b1_2_source_calibration_agent_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
