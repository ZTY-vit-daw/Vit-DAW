"""Smoke-test B1.3: fader unity reset confirmation chains into B1.2 clip gain confirmation."""

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


def parse_json_response(raw: bytes, label: str) -> dict[str, Any]:
    text = raw.decode("utf-8", errors="replace")
    parsed = json.loads(text)
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{label} returned non-object JSON: {text[:500]!r}")
    return parsed


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
        raise RuntimeError(f"{label} failed: {json.dumps(resp, ensure_ascii=False)[:1800]}")
    return resp


def invoke(base_url: str, tool: str, args: dict[str, Any] | None = None, *, confirmed: bool = True, timeout: float = 60.0) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args or {},
        "source": "b1_3_full_gain_staging_agent_smoke",
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


def finite_db(row: dict[str, Any], *keys: str) -> float:
    for key in keys:
        if key not in row:
            continue
        try:
            value = float(row[key])
        except (TypeError, ValueError):
            continue
        if math.isfinite(value):
            return value
    raise RuntimeError(f"row has no finite db value for {keys}: {row!r}")


def clip_gain_db(track: dict[str, Any], clip_id: str) -> float:
    for clip in track.get("clips", []):
        if not isinstance(clip, dict):
            continue
        current_id = str(clip.get("clip_id", clip.get("id", ""))).strip()
        if current_id == clip_id:
            return finite_db(clip, "clip_gain_db", "gain_db", "db")
    raise RuntimeError(f"clip {clip_id} gain not found in track row: {track!r}")


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


def extract_next_plan_id(resp: dict[str, Any], label: str) -> str:
    for key in ("next_plan_id", "plan_id"):
        value = str(resp.get(key, "")).strip()
        if value:
            return value
    raise RuntimeError(f"{label} has no next plan id: {json.dumps(resp, ensure_ascii=False)[:1800]}")


def assert_json_contains(resp: dict[str, Any], needle: str, label: str) -> None:
    text = json.dumps(resp, ensure_ascii=False)
    if needle not in text:
        raise RuntimeError(f"{label} missing {needle}: {text[:1800]}")


def assert_json_not_contains(resp: dict[str, Any], needle: str, label: str) -> None:
    text = json.dumps(resp, ensure_ascii=False)
    if needle in text:
        raise RuntimeError(f"{label} unexpectedly contains {needle}: {text[:1800]}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--kernel-exe", default="")
    parser.add_argument("--agent-exe", default="")
    parser.add_argument("--agent-http", default="http://127.0.0.1:17880")
    parser.add_argument("--agent-http-addr", default="127.0.0.1:17880")
    parser.add_argument("--timeout-sec", type=float, default=150.0)
    parser.add_argument("--reuse", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    kernel = Path(args.kernel_exe) if args.kernel_exe else repo / "VitApp" / "build_release" / "VitApp_artefacts" / "Release" / "VitApp.exe"
    agent = Path(args.agent_exe) if args.agent_exe else repo / "agent" / "bin" / "VitAgent.exe"
    run_id = int(time.time())
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b1_3_full_gain_staging_{run_id}"
    logs = repo / "VitApp" / "Workspace" / "Logs"
    logs.mkdir(parents=True, exist_ok=True)
    artifact_dir.mkdir(parents=True, exist_ok=True)
    agent_log = logs / "b1_3_full_gain_staging_agent_smoke_agent.log"
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
            ("quiet_a", 220.0, 0.03, -30.0, -12.0, -12.0),
            ("quiet_b", 247.0, 0.035, -29.0, -11.0, -9.0),
            ("reference_a", 330.0, 0.10, -20.0, -8.0, 0.0),
            ("reference_b", 440.0, 0.10, -20.0, -8.0, 0.0),
            ("reference_c", 550.0, 0.10, -20.0, -8.0, 0.0),
        ]
        wav_paths: dict[str, Path] = {}
        for name, hz, amp, _rms, _peak, _fader in material:
            path = artifact_dir / f"{name}.wav"
            write_sine_wav(path, hz, amp)
            wav_paths[name] = path

        require_ok(invoke(args.agent_http, "project.new", {}, confirmed=True, timeout=args.timeout_sec), "project.new")

        tracks: dict[str, dict[str, Any]] = {}
        for name, _hz, _amp, rms_dbfs, peak_dbfs, fader_db in material:
            track_id = read_id(require_ok(invoke(args.agent_http, "track.add_audio", {}, confirmed=True), f"track.add_audio {name}"), "track_id", "id")
            require_ok(invoke(args.agent_http, "track.volume", {"track_id": track_id, "db": fader_db}, confirmed=True), f"track.volume {name}")
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
            tracks[name] = {
                "track_id": track_id,
                "clip_id": read_id(imported, "clip_id", "id", "item_id"),
                "clip_name": wav_paths[name].name,
                "path": wav_paths[name],
                "rms_dbfs": rms_dbfs,
                "peak_dbfs": peak_dbfs,
            }

        session_id = f"b1_3_full_gain_staging_{run_id}"
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
        feature_snapshot_path.write_text(
            json.dumps(
                {
                    "schema_version": "mixboard_feature_snapshot.v1",
                    "updated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                    "track_waveform_envelopes": snapshot_rows,
                    "waveform_envelope": snapshot_rows[0],
                },
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )

        conversation_id = f"b1_3_full_gain_staging_smoke_{run_id}"
        pending_reset = post_json(
            args.agent_http,
            "/agent/chat",
            {
                "conversation_id": conversation_id,
                "message": "execute B1 gain staging repair for the whole project",
                "context": {
                    "agent_mode": "chat",
                    "feature_snapshot_path": str(feature_snapshot_path),
                    "acoustic_package_status_path": str(acoustic_status_path),
                    "mix_session_id": session_id,
                },
            },
            args.timeout_sec,
        )
        (artifact_dir / "pending_reset_response.json").write_text(json.dumps(pending_reset, ensure_ascii=False, indent=2), encoding="utf-8")
        if str(pending_reset.get("stop_reason", "")) != "needs_confirmation" and not bool(pending_reset.get("needs_confirmation")):
            raise RuntimeError(f"B1.1 did not create pending confirmation: {json.dumps(pending_reset, ensure_ascii=False)[:1800]}")
        reset_plan_id = extract_next_plan_id(pending_reset, "B1.1 pending response")
        assert_json_contains(pending_reset, "track.group.apply_control", "B1.1 pending response")
        assert_json_not_contains(pending_reset, "mix.apply_tick", "B1.1 pending response")

        pending_clip = post_json(args.agent_http, "/agent/confirm", {"plan_id": reset_plan_id, "decision": "approve"}, args.timeout_sec)
        (artifact_dir / "pending_clip_response.json").write_text(json.dumps(pending_clip, ensure_ascii=False, indent=2), encoding="utf-8")
        if str(pending_clip.get("status", "")).lower() != "ok" or not bool(pending_clip.get("needs_confirmation")):
            raise RuntimeError(f"B1.1 confirmation did not queue B1.2 pending action: {json.dumps(pending_clip, ensure_ascii=False)[:1800]}")
        clip_plan_id = extract_next_plan_id(pending_clip, "B1.2 queued response")
        assert_json_contains(pending_clip, "clip.gain.set_batch", "B1.2 queued response")
        assert_json_contains(pending_clip, "b1_2_source_calibration", "B1.2 queued response")
        assert_json_contains(pending_clip, "project.state", "B1.2 queued response")
        assert_json_contains(pending_clip, "mix.observe", "B1.2 queued response")
        assert_json_contains(pending_clip, "project.audio_analysis_status", "B1.2 queued response")
        assert_json_not_contains(pending_clip, "mix.apply_tick", "B1.2 queued response")

        completed = post_json(args.agent_http, "/agent/confirm", {"plan_id": clip_plan_id, "decision": "approve"}, args.timeout_sec)
        (artifact_dir / "completed_response.json").write_text(json.dumps(completed, ensure_ascii=False, indent=2), encoding="utf-8")
        if str(completed.get("status", "")).lower() != "ok" or str(completed.get("goal_status", "")).lower() != "completed":
            raise RuntimeError(f"B1.2 confirmation failed: {json.dumps(completed, ensure_ascii=False)[:1800]}")
        assert_json_contains(completed, "project.state", "completed response")
        assert_json_contains(completed, "mix.observe", "completed response")

        state = require_ok(invoke(args.agent_http, "project.state", {}, confirmed=True), "project.state after B1.3")
        rows = tracks_by_id(state)
        expected_quiet_gains = {"quiet_a": 10.0, "quiet_b": 9.0}
        for name, expected_gain in expected_quiet_gains.items():
            quiet_track = rows.get(str(tracks[name]["track_id"]))
            if quiet_track is None:
                raise RuntimeError(f"{name} track missing after B1.3")
            fader = finite_db(quiet_track, "volume_db", "fader_db", "gain_db", "db")
            if abs(fader) > 0.05:
                raise RuntimeError(f"{name} track fader was not reset to 0 dB, got {fader:.3f}")
            gain = clip_gain_db(quiet_track, str(tracks[name]["clip_id"]))
            if abs(gain - expected_gain) > 0.05:
                raise RuntimeError(f"{name} clip gain was not calibrated to {expected_gain:+.1f} dB, got {gain:.3f}")

        print(
            json.dumps(
                {
                    "status": "ok",
                    "conversation_id": conversation_id,
                    "reset_plan_id": reset_plan_id,
                    "clip_plan_id": clip_plan_id,
                    "quiet_track_ids": [tracks["quiet_a"]["track_id"], tracks["quiet_b"]["track_id"]],
                    "quiet_clip_ids": [tracks["quiet_a"]["clip_id"], tracks["quiet_b"]["clip_id"]],
                    "checked": [
                        "B1.1_pending_track_group_apply_control",
                        "B1.1_confirm_reobserves_project_state_mix_observe_and_audio_analysis_status",
                        "B1.1_confirm_queues_B1.2_clip_gain_set_batch",
                        "B1.2_confirm_completed",
                        "project_state_fader_reset_verified",
                        "project_state_clip_gain_verified",
                        "no_mix_apply_tick_route",
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
        print(f"b1_3_full_gain_staging_agent_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
