"""Smoke-test B1.2 strict reference-level calibration through agent chat."""

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
        except Exception as exc:  # noqa: BLE001
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
        "source": "b1_2_strict_reference_calibration_agent_smoke",
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
    parser.add_argument("--timeout-sec", type=float, default=150.0)
    parser.add_argument("--reuse", action="store_true")
    parser.add_argument("--precheck-first", action="store_true", help="Run a normal B1 report in the same conversation before strict B1.2.")
    parser.add_argument("--chinese-strict", action="store_true", help="Use a Chinese strict B1.2 request message.")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    kernel = Path(args.kernel_exe) if args.kernel_exe else repo / "VitApp" / "build_release" / "VitApp_artefacts" / "Release" / "VitApp.exe"
    agent = Path(args.agent_exe) if args.agent_exe else repo / "agent" / "bin" / "VitAgent.exe"
    run_id = int(time.time())
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b1_2_strict_reference_{run_id}"
    logs = repo / "VitApp" / "Workspace" / "Logs"
    logs.mkdir(parents=True, exist_ok=True)
    artifact_dir.mkdir(parents=True, exist_ok=True)
    agent_log = logs / "b1_2_strict_reference_calibration_agent_smoke_agent.log"
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

        levels = [-21.0, -20.8, -20.6, -20.4, -20.2, -19.8, -19.6, -19.4, -19.2, -19.0]
        require_ok(invoke(args.agent_http, "project.new", {}, confirmed=True, timeout=args.timeout_sec), "project.new")

        tracks: list[dict[str, Any]] = []
        for index, active_rms in enumerate(levels):
            name = f"strict_{index:02d}"
            path = artifact_dir / f"{name}.wav"
            write_sine_wav(path, 220.0 + index * 30.0, 0.08)
            track_id = read_id(require_ok(invoke(args.agent_http, "track.add_audio", {}, confirmed=True), f"track.add_audio {name}"), "track_id", "id")
            require_ok(invoke(args.agent_http, "track.volume", {"track_id": track_id, "db": 0.0}, confirmed=True), f"track.volume {name}")
            imported = require_ok(
                invoke(
                    args.agent_http,
                    "clip.import_media_to_track",
                    {"track_id": track_id, "file_path": str(path), "start_time": 0.0},
                    confirmed=True,
                    timeout=args.timeout_sec,
                ),
                f"clip.import_media_to_track {name}",
            )
            clip_id = read_id(imported, "clip_id", "id", "item_id")
            tracks.append(
                {
                    "name": name,
                    "track_id": track_id,
                    "clip_id": clip_id,
                    "clip_name": path.name,
                    "path": path,
                    "active_rms_dbfs": active_rms,
                }
            )

        session_id = f"b1_2_strict_reference_{int(time.time())}"
        snapshot_rows = []
        for data in tracks:
            active_rms = float(data["active_rms_dbfs"])
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
                    "source_revision": f"rev_{data['name']}",
                    "duration_seconds": 0.75,
                    "active_rms_dbfs": active_rms,
                    "rms_dbfs": active_rms - 1.0,
                    "peak_dbfs": -8.0,
                    "headroom_db": 8.0,
                    "crest_db": 12.0,
                }
            )
        feature_snapshot = {
            "schema_version": "mixboard_feature_snapshot.v1",
            "updated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "track_waveform_envelopes": snapshot_rows,
            "waveform_envelope": snapshot_rows[0],
        }
        feature_snapshot_path.write_text(json.dumps(feature_snapshot, ensure_ascii=False, indent=2), encoding="utf-8")

        conversation_id = f"b1_2_strict_reference_smoke_{int(time.time())}"
        chat_context = {
            "agent_mode": "chat",
            "feature_snapshot_path": str(feature_snapshot_path),
            "acoustic_package_status_path": str(acoustic_status_path),
            "mix_session_id": session_id,
        }
        if args.precheck_first:
            precheck = post_json(
                args.agent_http,
                "/agent/chat",
                {
                    "conversation_id": conversation_id,
                    "message": "Run a B1 gain staging level check for the full project.",
                    "context": chat_context,
                },
                args.timeout_sec,
            )
            (artifact_dir / "precheck_response.json").write_text(json.dumps(precheck, ensure_ascii=False, indent=2), encoding="utf-8")
            if bool(precheck.get("needs_confirmation")):
                raise RuntimeError(f"B1 precheck unexpectedly created pending confirmation: {json.dumps(precheck, ensure_ascii=False)[:2200]}")

        strict_message = "B1.2 strict reference level calibration for the full project"
        if args.chinese_strict:
            strict_message = (
                "B1.2 \u4e25\u683c\u53c2\u8003\u7535\u5e73\u6821\u51c6\uff1a"
                "\u4f7f\u7528\u5168\u5de5\u7a0b\u540c\u4e00\u53c2\u8003\u6307\u6807\uff0c"
                "\u8bc6\u522b\u9700\u8981\u9759\u6001 clip gain \u6821\u51c6\u7684\u4e3b\u97f3\u9891\u7247\u6bb5\uff0c"
                "\u5e76\u51c6\u5907 clip.gain.set_batch \u5f85\u786e\u8ba4\u52a8\u4f5c"
            )
        pending = post_json(
            args.agent_http,
            "/agent/chat",
            {
                "conversation_id": conversation_id,
                "message": strict_message,
                "context": chat_context,
            },
            args.timeout_sec,
        )
        (artifact_dir / "pending_response.json").write_text(json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8")
        stop_reason = str(pending.get("stop_reason", ""))
        needs_confirmation = bool(pending.get("needs_confirmation"))
        plan_id = str(pending.get("plan_id", "")).strip()
        if stop_reason != "needs_confirmation" and not needs_confirmation:
            raise RuntimeError(f"B1.2 strict did not create pending confirmation: {json.dumps(pending, ensure_ascii=False)[:2200]}")
        if not plan_id:
            raise RuntimeError(f"B1.2 strict pending response has no plan_id: {json.dumps(pending, ensure_ascii=False)[:2200]}")
        pending_json = json.dumps(pending, ensure_ascii=False)
        if "clip.gain.set_batch" not in pending_json or "source_clip_gain_calibration_batch" not in pending_json:
            raise RuntimeError(f"B1.2 strict pending route did not include one clip.gain.set_batch: {pending_json[:2200]}")
        if '"clip.gain.set"' not in pending_json:
            raise RuntimeError(f"B1.2 strict batch did not include child clip.gain.set actions: {pending_json[:2200]}")
        if "rlm_projection_id" not in pending_json or "calibration_mode" not in pending_json:
            raise RuntimeError(f"B1.2 strict pending route did not expose RLM metadata: {pending_json[:2200]}")
        if "project.audio_analysis_status" not in pending_json:
            raise RuntimeError(f"B1.2 strict pending route did not read project.audio_analysis_status: {pending_json[:2200]}")
        for forbidden in ("mix.apply_tick", "mix.propose_tick", "track.group.apply_control"):
            if forbidden in pending_json:
                raise RuntimeError(f"B1.2 strict incorrectly used {forbidden}: {pending_json[:2200]}")

        confirmed = post_json(args.agent_http, "/agent/confirm", {"plan_id": plan_id, "decision": "approve"}, args.timeout_sec)
        if str(confirmed.get("status", "")).lower() != "ok" or str(confirmed.get("goal_status", "")).lower() != "completed":
            raise RuntimeError(f"B1.2 strict confirmation failed: {json.dumps(confirmed, ensure_ascii=False)[:2200]}")
        confirmed_json = json.dumps(confirmed, ensure_ascii=False)
        for required in ("project.state", "mix.observe", "clip gain"):
            if required not in confirmed_json:
                raise RuntimeError(f"B1.2 strict confirmation missing {required}: {confirmed_json[:2200]}")

        state = require_ok(invoke(args.agent_http, "project.state", {}, confirmed=True), "project.state after B1.2 strict")
        rows = tracks_by_id(state)
        first_track = rows.get(str(tracks[0]["track_id"]))
        last_track = rows.get(str(tracks[-1]["track_id"]))
        if first_track is None or last_track is None:
            raise RuntimeError("first or last strict smoke track missing after B1.2")
        first_gain = clip_gain_db(first_track, str(tracks[0]["clip_id"]))
        last_gain = clip_gain_db(last_track, str(tracks[-1]["clip_id"]))
        if abs(first_gain - 1.0) > 0.05:
            raise RuntimeError(f"first clip gain expected +1.0 dB, got {first_gain:.3f}")
        if abs(last_gain + 1.0) > 0.05:
            raise RuntimeError(f"last clip gain expected -1.0 dB, got {last_gain:.3f}")

        print(
            json.dumps(
                {
                    "status": "ok",
                    "conversation_id": conversation_id,
                    "plan_id": plan_id,
                    "track_count": len(tracks),
                    "checked": [
                        "optional_same_conversation_b1_precheck" if args.precheck_first else "direct_strict_request",
                        "agent_chat_b1_2_strict_pending_confirmation",
                        "single_clip_gain_set_batch_route",
                        "rlm_metadata_present",
                        "no_track_group_or_mix_tick_for_b1_2",
                        "agent_confirm_completed",
                        "project_state_first_last_clip_gain_verified",
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
        print(f"b1_2_strict_reference_calibration_agent_smoke failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
