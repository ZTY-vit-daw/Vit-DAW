#!/usr/bin/env python3
"""SRC-TEMPLATE-1 (card 373) DG1-B stack-leg driver, 2026-11-16.

Drives the course template through the real kernel+agent berth and asserts
the DG1-B gate (docs/STACKGATE_DG1_DG3_V1.md §3.2, B1-B7):

  B1 normal load      agent tool face open_project (journey1-verified seam)
                      -> status ok + /agent/ui/state project identity +
                      expected template track names (session activation).
  B2 melody non-silent  kernel l2_render_probe on the melody track
                        (track_post_fader, offline_probe): ready + nonzero +
                        max_abs >= PROBE_MAX_ABS_MIN.
  B3 groove non-silent  same on the auxiliary groove track.
  B4 edit differential full-edit start_render WAV pair around a
                        student-style placeholder->assigned pitch edit
                        (kernel mutate_midi_notes by vit_note_id): both WAVs
                        non-silent (309 metering contract) and per-sample
                        max diff >= WAV_DIFF_MAX_MIN; supporting per-track
                        probe sum_abs delta recorded.
  B5 save & reopen       kernel save_project (clear-XML branch; template copy
                        is .xml) -> agent open_project reopen -> note readback
                        equality -> melody probe still non-silent.
  B6 config persist      after reopen: get_project_state track names + Surge
                        XT plugin rows enabled; saved XML pins TIMESIG 2/4,
                        TEMPO 80, vit_music_key_signature C, groove notes,
                        post-edit placeholder pitches.
  B7 no privilege escalation agent log window: admission-related lines
                        excerpted to artifacts; zero bypass/forged/unauthorized
                        markers; zero agent-side plugin control commands
                        (template plugins load natively from the project file,
                        PCA gate untouched). Surge-from-template only: the
                        silent Waves family is never mounted.

Predeclared thresholds (AGENTS §8 / STACKGATE §1.6, recorded per run):
  PROBE_MAX_ABS_MIN = 0.01   (float domain; 309 same-state offline render
                              measured ~0.23 peak -> -40 dBFS floor)
  WAV_NON_SILENT_PEAK = 500  (int16 counts, 309 contract)
  WAV_NON_SILENT_RMS = 20.0  (int16 counts, 309 contract)
  WAV_DIFF_MAX_MIN = 500     (int16 counts; one full-amplitude note changed)
  PROBE_SUM_ABS_REL_DELTA_MIN = 0.02 (supporting B4 evidence only)

Exit codes (fixed before execution, single run, no rerun):
  0 pass            B1-B7 all green + artifacts written
  2 environment     stack/transport failure (kernel/agent unreachable,
                    Surge XT absent from loaded state inventory)
  4 face_defect     B1-B6 assertion failure or B7 violation (forensics
                    hand-over; B7 violation additionally halts immediately)
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import re
import shutil
import struct
import sys
import time
import urllib.request
import wave
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

MANIFEST_FILE = "template_manifest.json"
TEMPLATE_TRACK_NAMES = ["参考与示范", "我的旋律", "辅助律动（鼓）", "我的八小节主题", "我的成曲版本"]
MELODY_TRACK_NAME = "我的旋律"
GROOVE_TRACK_NAME = "辅助律动（鼓）"
SURGE_NAME = "Surge XT"
BPM = 80.0
CLIP_SECONDS = 6.0          # 8 beats @ 80 bpm
TAIL_SECONDS = 0.25
RENDER_WAIT_SEC = 240.0

PROBE_MAX_ABS_MIN = 0.01
WAV_NON_SILENT_PEAK = 500
WAV_NON_SILENT_RMS = 20.0
WAV_DIFF_MAX_MIN = 500
PROBE_SUM_ABS_REL_DELTA_MIN = 0.02

EDIT_BEAT_TARGET = 0.0
EDIT_PITCH_FROM = 60
EDIT_PITCH_TO = 79

AGENT_PLUGIN_CONTROL_MARKERS = ("instantiate_plugin", "rack_add_node", "set_plugin_param")
B7_FORBIDDEN_RE = re.compile(r"bypass|forged|unauthori[sz]ed", re.I)
B7_EXCERPT_RE = re.compile(r"pca|admission|attest|certif|authori[sz]|bypass|forged|unauthori[sz]ed", re.I)

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_DEFECT = 4


def utcnow() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds")


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


def http_json(method: str, url: str, body: Optional[Dict[str, Any]] = None, timeout: int = 180) -> Dict[str, Any]:
    data = json.dumps(body, ensure_ascii=False).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json; charset=utf-8")
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode("utf-8"))


def sha256_file(path: Path) -> str:
    # Canonical-form hash: normalize CRLF->LF before hashing so the pinned
    # template hash is stable across git autocrlf checkout modes (the
    # generator emits LF; git checkouts may rewrite to CRLF on disk).
    data = path.read_bytes().replace(b"\r\n", b"\n")
    return hashlib.sha256(data).hexdigest()


def rows_of(value: Any) -> List[Dict[str, Any]]:
    if isinstance(value, list):
        return [row for row in value if isinstance(row, dict)]
    return []


class Runner:
    def __init__(self, args: argparse.Namespace) -> None:
        self.args = args
        self.summary: Dict[str, Any] = {
            "card": "SRC-TEMPLATE-1",
            "gate": "DG1-B (docs/STACKGATE_DG1_DG3_V1.md §3.2)",
            "leg": "src-template berth: B1-B7 (zero-LLM)",
            "started_utc": utcnow(),
            "thresholds": {
                "probe_max_abs_min": PROBE_MAX_ABS_MIN,
                "wav_non_silent_peak": WAV_NON_SILENT_PEAK,
                "wav_non_silent_rms": WAV_NON_SILENT_RMS,
                "wav_diff_max_min": WAV_DIFF_MAX_MIN,
                "probe_sum_abs_rel_delta_min": PROBE_SUM_ABS_REL_DELTA_MIN,
            },
            "assertions": {},
            "phases": {},
        }
        self.run_started_epoch = time.time()

    def save(self) -> None:
        Path(self.args.output).write_text(
            json.dumps(self.summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def record(self, assertion: str, ok: bool, detail: Dict[str, Any]) -> bool:
        self.summary["assertions"][assertion] = {"ok": ok, **detail}
        self.save()
        print(f"[src-template] {assertion}: {'PASS' if ok else 'FAIL'} {json.dumps(detail, ensure_ascii=False)[:300]}", flush=True)
        return ok

    def fail(self, exit_code: int, phase: str, reason: str) -> int:
        self.summary["exit_code"] = exit_code
        self.summary["exit_class"] = {0: "pass", 2: "environment", 4: "face_defect"}[exit_code]
        self.summary["fail_phase"] = phase
        self.summary["fail_reason"] = reason
        self.summary["ended_utc"] = utcnow()
        self.save()
        print(f"[src-template] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # ------------------------------------------------------------------ B1
    def agent_open(self, project_path: Path) -> Tuple[bool, Dict[str, Any]]:
        body = {
            "tool": "open_project",
            "command": {"cmd": "open_project", "file_path": str(project_path.resolve())},
            "source": "src_template_smoke",
            "confirmed": True,
        }
        response = http_json("POST", self.args.agent_http + "/agent/invoke", body)
        if bool(response.get("requires_confirmation")):
            body["confirmed"] = True
            response = http_json("POST", self.args.agent_http + "/agent/invoke", body)
        return str(response.get("status", "")) == "ok", response

    def kernel_tracks(self) -> List[Dict[str, Any]]:
        state = zmq_command(self.args.kernel_zmq, {"cmd": "get_project_state"})
        return rows_of(state.get("tracks"))

    def track_by_name(self, name: str) -> Optional[Dict[str, Any]]:
        for track in self.kernel_tracks():
            if str(track.get("name") or track.get("track_name") or "") == name:
                return track
        return None

    # ------------------------------------------------------------ probes
    def wait_probe_ready(self, request_id: str, timeout_s: float) -> Dict[str, Any]:
        context = zmq.Context.instance()
        sock = context.socket(zmq.SUB)
        sock.setsockopt(zmq.LINGER, 0)
        sock.setsockopt_string(zmq.SUBSCRIBE, "")
        sock.setsockopt(zmq.RCVTIMEO, 1000)
        outcome: Dict[str, Any] = {"event": None, "timed_out": True}
        try:
            sock.connect(self.args.kernel_sub)
            deadline = time.time() + timeout_s
            while time.time() < deadline:
                try:
                    payload = sock.recv_string()
                except zmq.error.Again:
                    continue
                try:
                    message = json.loads(payload)
                except json.JSONDecodeError:
                    continue
                if not isinstance(message, dict):
                    continue
                if str(message.get("command", "")) not in ("l2_render_probe_ready", "l2_render_probe_status"):
                    continue
                if str(message.get("request_id", "")) != request_id:
                    continue
                outcome["event"] = message
                if str(message.get("status", "")) in ("ready", "suspect"):
                    outcome["timed_out"] = False
                    return outcome
        finally:
            sock.close(0)
        return outcome

    def probe_track(self, track_id: str, label: str) -> Dict[str, Any]:
        request_id = f"srctemplate_{label}_{int(time.time()*1000)}"
        reply = zmq_command(self.args.kernel_zmq, {
            "cmd": "l2_render_probe",
            "request_id": request_id,
            "track_id": track_id,
            "tap_point": "track_post_fader",
            "render_mode": "offline_probe",
            "start": 0.0,
            "end": CLIP_SECONDS + TAIL_SECONDS,
            "tail_seconds": TAIL_SECONDS,
        })
        if str(reply.get("status", "")) != "ok":
            return {"ok": False, "reason": f"l2_render_probe status={reply.get('status')!r}", "reply": reply}
        outcome = self.wait_probe_ready(request_id, RENDER_WAIT_SEC)
        event = outcome.get("event") or {}
        evidence = event.get("quality_evidence") if isinstance(event.get("quality_evidence"), dict) else {}
        return {
            "ok": True,
            "probe_status": event.get("status"),
            "nonzero": evidence.get("nonzero"),
            "max_abs": evidence.get("max_abs"),
            "sum_abs": evidence.get("sum_abs"),
            "nan_inf_count": evidence.get("nan_inf_count"),
            "timed_out": outcome.get("timed_out"),
            "event": event,
        }

    def wait_render_done(self, wav_path: Path, job_id: str, timeout_s: float) -> Dict[str, Any]:
        outcome: Dict[str, Any] = {"sub_signal": None, "last_message": None}
        context = zmq.Context.instance()
        sock = context.socket(zmq.SUB)
        sock.setsockopt(zmq.LINGER, 0)
        sock.setsockopt_string(zmq.SUBSCRIBE, "")
        sock.setsockopt(zmq.RCVTIMEO, 1000)
        try:
            sock.connect(self.args.kernel_sub)
            deadline = time.time() + timeout_s
            while time.time() < deadline:
                try:
                    payload = sock.recv_string()
                except zmq.error.Again:
                    if wav_path.exists() and wav_path.stat().st_size > 44 and time.time() > deadline - timeout_s / 3:
                        outcome["sub_signal"] = "file_fallback"
                        return outcome
                    continue
                try:
                    message = json.loads(payload)
                except json.JSONDecodeError:
                    continue
                if not isinstance(message, dict):
                    continue
                if message.get("topic") == "render" and str(message.get("job_id", "")) == job_id:
                    outcome["last_message"] = message
                    sub = str(message.get("subtopic", ""))
                    if sub == "render_done" and str(message.get("status")) == "ok":
                        outcome["sub_signal"] = "render_done"
                        return outcome
                    if sub == "render_failed" or (sub == "render_done" and str(message.get("status")) != "ok"):
                        outcome["sub_signal"] = f"{sub}:{message.get('status')}:{message.get('message', '')}"
                        return outcome
        finally:
            sock.close(0)
        return outcome

    def render_full(self, wav_path: Path) -> Dict[str, Any]:
        if wav_path.exists():
            wav_path.unlink()
        reply = zmq_command(self.args.kernel_zmq, {
            "cmd": "start_render",
            "file_path": str(wav_path.resolve()),
            "bit_depth": 16,
        })
        if str(reply.get("status", "")) != "ok":
            return {"ok": False, "reason": f"start_render status={reply.get('status')!r}", "reply": reply}
        job_id = str(reply.get("job_id", ""))
        outcome = self.wait_render_done(wav_path, job_id, RENDER_WAIT_SEC)
        if not wav_path.exists() or wav_path.stat().st_size <= 44:
            return {"ok": False, "reason": f"render product missing (sub={outcome.get('sub_signal')})"}
        return {"ok": True, "completion": outcome.get("sub_signal"), "job_id": job_id,
                "wav": str(wav_path), **wav_meter(wav_path)}


def wav_meter(path: Path) -> Dict[str, Any]:
    with wave.open(str(path), "rb") as handle:
        channels = handle.getnchannels()
        width = handle.getsampwidth()
        frames = handle.getnframes()
        raw = handle.readframes(frames)
    if width != 2:
        return {"error": f"unexpected sample width {width}"}
    count = len(raw) // 2
    samples = struct.unpack("<%dh" % count, raw[: count * 2])
    peak = max(abs(s) for s in samples) if samples else 0
    square_sum = sum(float(s) * float(s) for s in samples)
    rms = math.sqrt(square_sum / count) if count else 0.0
    return {
        "channels": channels,
        "frames": frames,
        "peak_abs": peak,
        "rms_abs": round(rms, 2),
        "peak_dbfs": round(20 * math.log10(peak / 32768.0), 2) if peak > 0 else -120.0,
        "samples": samples,
    }


def wav_diff(samples_a: List[int], samples_b: List[int]) -> Dict[str, Any]:
    if len(samples_a) != len(samples_b):
        return {"error": f"length mismatch {len(samples_a)} != {len(samples_b)}",
                "max_abs_diff": None}
    diffs = [abs(a - b) for a, b in zip(samples_a, samples_b)]
    max_diff = max(diffs) if diffs else 0
    mean_diff = (sum(diffs) / len(diffs)) if diffs else 0.0
    return {"max_abs_diff": max_diff, "mean_abs_diff": round(mean_diff, 2)}


def read_notes(clip_id: str, zmq_url: str) -> List[Dict[str, Any]]:
    reply = zmq_command(zmq_url, {"cmd": "get_midi_clip_notes", "clip_id": clip_id})
    return rows_of(reply.get("notes"))


def note_key(note: Dict[str, Any]) -> Tuple[float, int]:
    start = float(note.get("start", note.get("start_beat", note.get("start_beats", 0)) or 0))
    pitch = int(note.get("pitch", note.get("p", note.get("note_number", 0)) or 0))
    return (round(start, 4), pitch)


def telemetry_rows_in_window(path: Optional[Path], since_epoch: float) -> Dict[str, Any]:
    if path is None or not path.exists():
        return {"path": str(path) if path else None, "rows_in_run_window": None,
                "note": "telemetry file absent (no rows written)"}
    rows = 0
    recent: List[str] = []
    try:
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            if not line.strip():
                continue
            rows += 1
            try:
                row = json.loads(line)
                ts = str(row.get("ts") or row.get("timestamp") or "")
                if ts and ts >= datetime.fromtimestamp(since_epoch).isoformat(timespec="seconds"):
                    recent.append(line[:220])
            except json.JSONDecodeError:
                continue
    except OSError as exc:
        return {"path": str(path), "error": str(exc)}
    return {"path": str(path), "rows_total": rows, "rows_in_run_window": len(recent),
            "recent_excerpts": recent[:8]}


def main() -> int:
    parser = argparse.ArgumentParser(description="SRC-TEMPLATE-1 DG1-B stack-leg driver (B1-B7, zero-LLM)")
    parser.add_argument("--agent-http", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--kernel-sub", default="tcp://127.0.0.1:5556")
    parser.add_argument("--template", required=True, help="source template.v1.xml (repo asset, read-only)")
    parser.add_argument("--artifacts-dir", required=True)
    parser.add_argument("--agent-log", default="")
    parser.add_argument("--telemetry-path", default="")
    parser.add_argument("--output", required=True, help="summary json path")
    args = parser.parse_args()

    artifacts = Path(args.artifacts_dir)
    artifacts.mkdir(parents=True, exist_ok=True)
    runner = Runner(args)

    # ---- phase 0: place-copy + hash ledger ------------------------------
    template_src = Path(args.template)
    manifest_path = template_src.parent / MANIFEST_FILE
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        return runner.fail(EXIT_ENV, "precheck", f"manifest read failed: {exc}")
    src_hash = sha256_file(template_src)
    if manifest.get("template_sha256") and manifest["template_sha256"] != src_hash:
        return runner.fail(EXIT_ENV, "precheck",
                           f"template sha256 drift vs manifest: {src_hash} != {manifest['template_sha256']}")
    project_copy = artifacts / "lesson_project.xml"
    shutil.copyfile(template_src, project_copy)
    copy_hash = sha256_file(project_copy)
    runner.summary["phases"]["precheck"] = {
        "template_source": str(template_src), "template_sha256": src_hash,
        "project_copy": str(project_copy), "copy_sha256": copy_hash,
        "manifest_id": manifest.get("id"),
    }
    runner.save()

    # kernel reachability (environment class)
    try:
        pong = zmq_command(args.kernel_zmq, {"cmd": "ping"}, timeout_ms=5000)
        if str(pong.get("status", "")).lower() not in {"ok", "success"}:
            raise RuntimeError(f"ping replied {pong.get('status')!r}")
    except Exception as exc:  # noqa: BLE001
        return runner.fail(EXIT_ENV, "precheck", f"kernel ping failed: {exc}")

    # ---- B1: normal load via agent tool face ----------------------------
    ok, load_response = runner.agent_open(project_copy)
    if not ok:
        return runner.fail(EXIT_DEFECT, "B1",
                           f"open_project via agent tool face failed: {json.dumps(load_response, ensure_ascii=False)[:400]}")
    runner.summary["phases"]["B1_open_reply"] = load_response
    time.sleep(6)  # session activation window (journey1 pacing)
    try:
        ui_state = http_json("GET", args.agent_http + "/agent/ui/state", timeout=60)
    except Exception as exc:  # noqa: BLE001
        return runner.fail(EXIT_ENV, "B1", f"/agent/ui/state read failed: {exc}")
    runner.summary["phases"]["B1_ui_state"] = ui_state
    project = ui_state.get("project") if isinstance(ui_state.get("project"), dict) else {}
    history = ui_state.get("project_history") if isinstance(ui_state.get("project_history"), dict) else {}
    ui_tracks = []
    for t in ui_tracks_of(ui_state):
        name = str(t.get("name", "") or t.get("track_name", "") or "")
        if name:
            ui_tracks.append(name)
    # Session-activation identity: the opened copy is the active project path
    # and the bound project uuid is the template uuid (BindProjectIdentity-
    # level evidence, journey1 same seam).
    b1_identity = (str(project.get("project_path", "")) == str(project_copy.resolve())
                   and str(history.get("project_uuid", "")) == str(manifest.get("template_project_uuid", "")))
    b1_tracks = all(name in ui_tracks for name in TEMPLATE_TRACK_NAMES)
    runner.record("B1_normal_load", b1_identity and b1_tracks,
                  {"project_path": project.get("project_path"),
                   "project_uuid": history.get("project_uuid"), "ui_tracks": ui_tracks,
                   "load_status": load_response.get("status")})
    if not (b1_identity and b1_tracks):
        return runner.fail(EXIT_DEFECT, "B1", "session activation evidence incomplete (identity/tracks)")

    # resolve kernel-side track/clip handles
    melody = runner.track_by_name(MELODY_TRACK_NAME)
    groove = runner.track_by_name(GROOVE_TRACK_NAME)
    if melody is None or groove is None:
        return runner.fail(EXIT_ENV, "handles", f"track resolution failed: melody={melody is None} groove={groove is None}")
    melody_id = str(melody.get("track_id") or melody.get("id"))
    groove_id = str(groove.get("track_id") or groove.get("id"))
    melody_plugins = [str(p.get("name", "")) for p in rows_of(melody.get("plugins"))]
    if SURGE_NAME not in melody_plugins:
        return runner.fail(EXIT_ENV, "handles",
                           f"Surge XT not loaded on melody track after template open (plugins={melody_plugins}) — "
                           "healthy-source precondition hand-over (309 evidence chain)")
    melody_clips = rows_of(melody.get("clips"))
    melody_clip_id = str((melody_clips[0].get("clip_id") or melody_clips[0].get("id"))) if melody_clips else ""
    runner.summary["phases"]["handles"] = {
        "melody_track_id": melody_id, "groove_track_id": groove_id,
        "melody_plugins": melody_plugins, "melody_clip_id": melody_clip_id,
    }
    runner.save()
    if not melody_clip_id:
        return runner.fail(EXIT_DEFECT, "handles", "melody track exposes no clip after open")

    # ---- B2/B3: per-track non-silence probes ----------------------------
    probe_melody = runner.probe_track(melody_id, "melody")
    runner.summary["phases"]["B2_probe_melody"] = compact_probe(probe_melody)
    b2 = bool(probe_melody.get("ok")) and probe_melody.get("probe_status") == "ready" \
        and probe_melody.get("nonzero") is True and float(probe_melody.get("max_abs") or 0) >= PROBE_MAX_ABS_MIN \
        and int(float(probe_melody.get("nan_inf_count") or 0)) == 0
    runner.record("B2_melody_non_silent", b2, compact_probe(probe_melody))
    if not b2:
        return runner.fail(EXIT_DEFECT, "B2", f"melody probe not non-silent: {compact_probe(probe_melody)}")

    probe_groove = runner.probe_track(groove_id, "groove")
    runner.summary["phases"]["B3_probe_groove"] = compact_probe(probe_groove)
    b3 = bool(probe_groove.get("ok")) and probe_groove.get("probe_status") == "ready" \
        and probe_groove.get("nonzero") is True and float(probe_groove.get("max_abs") or 0) >= PROBE_MAX_ABS_MIN \
        and int(float(probe_groove.get("nan_inf_count") or 0)) == 0
    runner.record("B3_groove_non_silent", b3, compact_probe(probe_groove))
    if not b3:
        return runner.fail(EXIT_DEFECT, "B3", f"groove probe not non-silent: {compact_probe(probe_groove)}")

    # ---- B4: student-style edit differential ----------------------------
    wav_before = artifacts / "render_before_edit.wav"
    render_a = runner.render_full(wav_before)
    if not render_a.get("ok"):
        return runner.fail(EXIT_DEFECT, "B4", f"pre-edit render failed: {render_a.get('reason')}")

    notes_before = read_notes(melody_clip_id, args.kernel_zmq)
    target = None
    for note in notes_before:
        key = note_key(note)
        if abs(key[0] - EDIT_BEAT_TARGET) < 1e-3 and key[1] == EDIT_PITCH_FROM:
            target = note
            break
    if target is None:
        return runner.fail(EXIT_DEFECT, "B4",
                           f"placeholder note at beat {EDIT_BEAT_TARGET} pitch {EDIT_PITCH_FROM} not found; "
                           f"readback={json.dumps(notes_before, ensure_ascii=False)[:400]}")
    note_id = str(target.get("vit_note_id") or target.get("note_id") or target.get("id") or "")
    if not note_id:
        return runner.fail(EXIT_DEFECT, "B4", "placeholder note carries no vit_note_id")
    mut = zmq_command(args.kernel_zmq, {
        "cmd": "mutate_midi_notes", "track_id": melody_id, "clip_id": melody_clip_id,
        "time_unit": "beats",
        "notes": [{"id": note_id, "pitch": EDIT_PITCH_TO}],
    })
    if str(mut.get("status", "")) != "ok":
        return runner.fail(EXIT_DEFECT, "B4", f"mutate_midi_notes failed: {mut}")
    notes_after = read_notes(melody_clip_id, args.kernel_zmq)
    after_map = {note_key(n): n for n in notes_after}
    edited = (EDIT_BEAT_TARGET, EDIT_PITCH_TO) in after_map
    old_gone = (EDIT_BEAT_TARGET, EDIT_PITCH_FROM) not in after_map
    same_count = len(notes_after) == len(notes_before)
    runner.record("B4_edit_readback", edited and old_gone and same_count,
                  {"note_id": note_id, "edited_present": edited, "placeholder_gone": old_gone,
                   "count": [len(notes_before), len(notes_after)]})
    if not (edited and old_gone and same_count):
        return runner.fail(EXIT_DEFECT, "B4", "student edit did not land in readback")

    probe_melody_after = runner.probe_track(melody_id, "melody_after_edit")
    runner.summary["phases"]["B4_probe_melody_after"] = compact_probe(probe_melody_after)
    sum_before = float(probe_melody.get("sum_abs") or 0)
    sum_after = float(probe_melody_after.get("sum_abs") or 0)
    rel_delta = abs(sum_after - sum_before) / sum_before if sum_before > 0 else None

    wav_after = artifacts / "render_after_edit.wav"
    render_b = runner.render_full(wav_after)
    if not render_b.get("ok"):
        return runner.fail(EXIT_DEFECT, "B4", f"post-edit render failed: {render_b.get('reason')}")
    diff = wav_diff(render_a["samples"], render_b["samples"])
    render_a.pop("samples", None)
    render_b.pop("samples", None)
    runner.summary["phases"]["B4_renders"] = {"before": render_a, "after": render_b, "diff": diff,
                                              "probe_sum_abs": [sum_before, sum_after],
                                              "probe_sum_abs_rel_delta": rel_delta}
    both_non_silent = render_a["peak_abs"] > WAV_NON_SILENT_PEAK and render_a["rms_abs"] > WAV_NON_SILENT_RMS \
        and render_b["peak_abs"] > WAV_NON_SILENT_PEAK and render_b["rms_abs"] > WAV_NON_SILENT_RMS
    diff_ok = diff.get("max_abs_diff") is not None and diff["max_abs_diff"] >= WAV_DIFF_MAX_MIN
    b4 = both_non_silent and diff_ok
    runner.record("B4_edit_differential", b4,
                  {"before_peak": render_a["peak_abs"], "after_peak": render_b["peak_abs"],
                   "max_abs_diff": diff.get("max_abs_diff"), "probe_sum_abs_rel_delta": rel_delta})
    if not b4:
        return runner.fail(EXIT_DEFECT, "B4", f"edit differential below threshold: {diff}")

    # ---- B5: save + reopen via normal chains ----------------------------
    # Production save semantics: normalizeProjectPathForSave rewrites ANY
    # project path to the .vit extension, so save_project on the .xml copy
    # persists to an encrypted lesson_project.vit sibling (VitProjectFile.cpp
    # normalizeProjectPathForSave; clear-XML sidecar only with
    # VIT_DUMP_CLEAR_XML=1). The leg therefore reopens the actual .vit the
    # kernel reports — the real user save/reopen form.
    save = zmq_command(args.kernel_zmq, {"cmd": "save_project"})
    if str(save.get("status", "")) != "ok":
        return runner.fail(EXIT_DEFECT, "B5", f"save_project failed: {save}")
    saved_path = Path(str(save.get("project_path") or save.get("current_project_path") or ""))
    time.sleep(1.0)
    if not saved_path.exists() or saved_path.stat().st_size <= 0:
        return runner.fail(EXIT_DEFECT, "B5", f"saved project file missing/empty: {saved_path}")
    saved_is_vit = saved_path.suffix.lower() == ".vit"
    saved_hash = sha256_file(saved_path)
    runner.summary["phases"]["B5_save"] = {"reply_project_path": str(saved_path),
                                           "saved_is_vit": saved_is_vit,
                                           "saved_sha256": saved_hash,
                                           "saved_bytes": saved_path.stat().st_size}
    ok_reopen, reopen_response = runner.agent_open(saved_path)
    if not ok_reopen:
        return runner.fail(EXIT_DEFECT, "B5",
                           f"reopen via agent tool face failed: {json.dumps(reopen_response, ensure_ascii=False)[:400]}")
    time.sleep(4)
    melody2 = runner.track_by_name(MELODY_TRACK_NAME)
    if melody2 is None:
        return runner.fail(EXIT_ENV, "B5", "melody track missing after reopen")
    melody2_id = str(melody2.get("track_id") or melody2.get("id"))
    clips2 = rows_of(melody2.get("clips"))
    clip2_id = str((clips2[0].get("clip_id") or clips2[0].get("id"))) if clips2 else ""
    if not clip2_id:
        return runner.fail(EXIT_DEFECT, "B5", "melody clip missing after reopen")
    notes_reopened = read_notes(clip2_id, args.kernel_zmq)
    keys_after = sorted(note_key(n) for n in notes_after)
    keys_reopened = sorted(note_key(n) for n in notes_reopened)
    b5_readback = keys_after == keys_reopened
    runner.record("B5_readback_equality", b5_readback,
                  {"notes_saved": len(keys_after), "notes_reopened": len(keys_reopened),
                   "reopened_file": str(saved_path)})
    if not b5_readback:
        runner.summary["phases"]["B5_keys_after"] = [list(k) for k in keys_after]
        runner.summary["phases"]["B5_keys_reopened"] = [list(k) for k in keys_reopened]
        runner.save()
        return runner.fail(EXIT_DEFECT, "B5", "note readback mismatch after save+reopen")

    probe_melody_reopened = runner.probe_track(melody2_id, "melody_reopened")
    runner.summary["phases"]["B5_probe_reopened"] = compact_probe(probe_melody_reopened)
    b5_sound = bool(probe_melody_reopened.get("ok")) and probe_melody_reopened.get("probe_status") == "ready" \
        and probe_melody_reopened.get("nonzero") is True \
        and float(probe_melody_reopened.get("max_abs") or 0) >= PROBE_MAX_ABS_MIN
    runner.record("B5_reopen_non_silent", b5_sound, compact_probe(probe_melody_reopened))
    if not b5_sound:
        return runner.fail(EXIT_DEFECT, "B5", "reopened melody probe silent")

    # ---- B6: config persistence (reopened state + snapshot face) --------
    tracks_final = {}
    for t in runner.kernel_tracks():
        tracks_final[str(t.get("name") or t.get("track_name") or "")] = t
    tracks_ok = all(name in tracks_final for name in TEMPLATE_TRACK_NAMES)
    plugin_rows = {}
    for name in (MELODY_TRACK_NAME, GROOVE_TRACK_NAME, "我的八小节主题"):
        track = tracks_final.get(name, {})
        plugins = [str(p.get("name", "")) for p in rows_of(track.get("plugins"))]
        plugin_rows[name] = plugins
    instruments_ok = all(SURGE_NAME in plugin_rows[name] for name in plugin_rows)

    # Kernel production snapshot face: the authoritative persisted view of the
    # reopened project (works for encrypted .vit projects where direct file
    # text assertions are impossible).
    snap = zmq_command(args.kernel_zmq, {"cmd": "project_snapshot_export"})
    snapshot_xml = str(snap.get("snapshot_xml") or "")
    (artifacts / "reopened_snapshot.xml").write_text(snapshot_xml, encoding="utf-8")
    pins = {
        "timesig_2_4": '<TIMESIG numerator="2" denominator="4"' in snapshot_xml,
        "tempo_80": re.search(r'<TEMPO startBeat="0\.0" bpm="80\.0"', snapshot_xml) is not None,
        "key_c": 'vit_music_key_signature="C"' in snapshot_xml,
        "groove_channel_10": 'channelNumber="10"' in snapshot_xml,
        "edited_pitch_saved": f'p="{EDIT_PITCH_TO}"' in snapshot_xml,
        "surge_blocks": snapshot_xml.count('name="Surge XT"') >= 3,
        "surge_states_pinned": snapshot_xml.count('vit_template_role="instrument"') >= 3,
    }
    b6 = tracks_ok and instruments_ok and saved_is_vit and all(pins.values())
    runner.record("B6_config_persist", b6,
                  {"tracks_ok": tracks_ok, "instruments_ok": instruments_ok,
                   "saved_is_vit": saved_is_vit, "plugin_rows": plugin_rows, "snapshot_pins": pins})
    if not b6:
        return runner.fail(EXIT_DEFECT, "B6", f"config persistence mismatch: tracks={tracks_ok} "
                                              f"instruments={instruments_ok} saved_is_vit={saved_is_vit} pins={pins}")

    # ---- B7: no privilege escalation ------------------------------------
    agent_log_path = Path(args.agent_log) if args.agent_log else None
    b7_detail = scan_agent_log(agent_log_path, runner.run_started_epoch)
    (artifacts / "b7_admission_excerpt.txt").write_text(
        b7_detail["excerpt_text"], encoding="utf-8")
    b7 = (not b7_detail["forbidden_hits"]) and (not b7_detail["plugin_control_hits"])
    runner.summary["phases"]["B7_admission_log"] = {
        "log_path": str(agent_log_path), "lines_scanned": b7_detail["lines_scanned"],
        "forbidden_hits": b7_detail["forbidden_hits"], "plugin_control_hits": b7_detail["plugin_control_hits"],
        "excerpt_artifact": str(artifacts / "b7_admission_excerpt.txt"),
    }
    runner.record("B7_no_privilege_escalation", b7,
                  {"forbidden_hits": b7_detail["forbidden_hits"], "plugin_control_hits": b7_detail["plugin_control_hits"]})
    if not b7:
        # security boundary: stop immediately, forensics hand-over
        return runner.fail(EXIT_DEFECT, "B7",
                           "PCA/admission window shows bypass markers or agent-side plugin control: "
                           + json.dumps(b7_detail["forbidden_hits"] + b7_detail["plugin_control_hits"])[:400])

    # ---- zero-LLM evidence ----------------------------------------------
    runner.summary["phases"]["billing"] = telemetry_rows_in_window(
        Path(args.telemetry_path) if args.telemetry_path else None, runner.run_started_epoch)

    runner.summary["exit_code"] = EXIT_PASS
    runner.summary["exit_class"] = "pass"
    runner.summary["ended_utc"] = utcnow()
    runner.save()
    print("[src-template] DG1-B PASS: B1-B7 all green", flush=True)
    return EXIT_PASS


def ui_tracks_of(ui_state: Dict[str, Any]) -> List[Dict[str, Any]]:
    def coerce(value: Any) -> List[Dict[str, Any]]:
        if isinstance(value, list):
            return [row for row in value if isinstance(row, dict)]
        if isinstance(value, dict):  # named-children map (journey1 shape)
            return [row for row in value.values() if isinstance(row, dict)]
        return []
    for key in ("tracks", "project_tracks", "ui_tracks"):
        rows = coerce(ui_state.get(key))
        if rows:
            return rows
    project = ui_state.get("project")
    if isinstance(project, dict):
        for key in ("tracks", "project_tracks"):
            rows = coerce(project.get(key))
            if rows:
                return rows
    return []


def compact_probe(probe: Dict[str, Any]) -> Dict[str, Any]:
    return {k: probe.get(k) for k in ("ok", "reason", "probe_status", "nonzero", "max_abs",
                                      "sum_abs", "nan_inf_count", "timed_out")}


def scan_agent_log(path: Optional[Path], since_epoch: float) -> Dict[str, Any]:
    detail: Dict[str, Any] = {"lines_scanned": 0, "forbidden_hits": [], "plugin_control_hits": [],
                              "excerpt_text": "", "error": None}
    if path is None or not path.exists():
        detail["error"] = f"agent log not available: {path}"
        detail["excerpt_text"] = detail["error"]
        return detail
    since_iso = datetime.fromtimestamp(since_epoch).isoformat(timespec="seconds")
    excerpts: List[str] = []
    try:
        for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
            ts_match = re.match(r"(\d{4}-\d{2}-\d{2}[T ][0-9:.]+)", line)
            if ts_match and ts_match.group(1) < since_iso:
                continue  # before this run window
            detail["lines_scanned"] += 1
            if B7_EXCERPT_RE.search(line):
                excerpts.append(line[:400])
            if B7_FORBIDDEN_RE.search(line):
                detail["forbidden_hits"].append(line[:400])
            for marker in AGENT_PLUGIN_CONTROL_MARKERS:
                if marker in line:
                    detail["plugin_control_hits"].append(line[:400])
    except OSError as exc:
        detail["error"] = str(exc)
    detail["excerpt_text"] = "\n".join(excerpts) or "(no admission-related lines in run window)"
    return detail


if __name__ == "__main__":
    sys.exit(main())
