# KERNEL-RENDER-FREEZE-FIX-1 verification probe: drives the rebuilt staging
# kernel over raw ZMQ to verify all three fix legs on the real stack BEFORE
# the semantics are frozen into the dev_agent_smoke.ps1 scenario:
#   leg B  - MIDI-only edit render.start must return a synchronous
#            reason=no_renderable_audio_content error (no job, no freeze)
#   leg A  - empty-range render (audio clip present but outside range) must
#            deliver an asynchronous render_failed telemetry event and leave
#            the command surface alive (the KRF1 freeze path, reversed)
#   cancel - cancel_render during a live render must reply ok and terminate
#            the job via telemetry
# Kernel must already be running. NO child-process ownership (KRF1 lesson).
import json
import os
import struct
import sys
import time
import wave
from datetime import datetime

import zmq

RUN_DIR = os.path.abspath(sys.argv[1])
REQ_EP = "tcp://127.0.0.1:5555"
SUB_EP = "tcp://127.0.0.1:5556"

ev_path = os.path.join(RUN_DIR, "probe_events.jsonl")
telemetry_path = os.path.join(RUN_DIR, "telemetry_render.jsonl")


def log(kind, **fields):
    rec = {"t": time.time(), "wall": datetime.now().isoformat(), "kind": kind}
    rec.update(fields)
    with open(ev_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(rec, ensure_ascii=False) + "\n")
    print(f"[{rec['wall'][11:23]}] {kind}: {str(fields)[:260]}", flush=True)


def write_sine_wav(path, seconds=2.2, freq=440.0, amp=0.5, rate=44100):
    n = int(rate * seconds)
    with wave.open(path, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(rate)
        frames = bytearray()
        for i in range(n):
            v = int(amp * 32767.0 * __import__("math").sin(2.0 * 3.141592653589793 * freq * i / rate))
            frames += struct.pack("<h", v)
        w.writeframes(bytes(frames))
    return path


def main():
    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(REQ_EP)

    sub = ctx.socket(zmq.SUB)
    sub.setsockopt(zmq.LINGER, 0)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.connect(SUB_EP)
    time.sleep(0.3)

    def send(cmd, timeout_ms):
        req.setsockopt(zmq.RCVTIMEO, timeout_ms)
        try:
            req.send_string(json.dumps(cmd))
            return req.recv_string()
        except zmq.Again:
            return None

    def drain_telemetry():
        events = []
        while True:
            try:
                sub.setsockopt(zmq.RCVTIMEO, 50)
                msg = sub.recv_string()
            except zmq.Again:
                return events
            events.append(msg)

    def wait_terminal(job_id, timeout_s):
        deadline = time.time() + timeout_s
        seen = []
        while time.time() < deadline:
            for msg in drain_telemetry():
                with open(telemetry_path, "a", encoding="utf-8") as fh:
                    fh.write(msg + "\n")
                try:
                    obj = json.loads(msg)
                except ValueError:
                    continue
                seen.append(obj)
                if obj.get("topic") == "render" and obj.get("job_id") == job_id \
                        and obj.get("subtopic") in ("render_done", "render_failed"):
                    return obj, seen
            time.sleep(0.1)
        return None, seen

    rep = send({"cmd": "get_project_state"}, 5000)
    log("initial_ping", alive=(rep is not None))
    if rep is None:
        return 3

    # ---- clean slate -------------------------------------------------
    rep = send({"cmd": "clear_project"}, 15000)
    log("clear_project", reply=(rep[:120] if rep else None))

    # ---- leg B: MIDI-only edit ---------------------------------------
    rep = send({"cmd": "add_track"}, 15000)
    track_id = json.loads(rep)["track_id"]
    rep = send({"cmd": "insert_midi_clip", "track_id": track_id,
                "start": 0, "length": 8, "time_unit": "beats"}, 15000)
    clip_id = json.loads(rep)["clip_id"]
    send({"cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
          "time_unit": "beats",
          "notes": [{"pitch": 60, "start": 0, "length": 1, "velocity": 90}]}, 15000)
    drain_telemetry()

    midi_wav = os.path.join(RUN_DIR, "midi_only_render.wav")
    rep = send({"cmd": "start_render", "file_path": midi_wav, "bit_depth": 24}, 20000)
    log("legB_midi_only_start_render", reply=rep)
    parsed = json.loads(rep) if rep else {}
    log("legB_verdict",
        sync_error=(parsed.get("status") == "error"),
        reason=parsed.get("reason"),
        message=parsed.get("message"))
    log("legB_no_job_started", no_job=("job_id" not in parsed))

    rep = send({"cmd": "get_project_state"}, 6000)
    log("legB_post_error_ping", alive=(rep is not None))
    rep = send({"cmd": "ping"}, 6000)
    log("legB_post_error_ping_cmd", reply=(rep[:80] if rep else None))

    rep = send({"cmd": "cancel_render"}, 6000)
    log("legB_cancel_idle", reply=(rep[:120] if rep else None))

    # ---- leg A: audio present, render range excludes it ---------------
    sine = write_sine_wav(os.path.join(RUN_DIR, "sine_440_%d.wav" % int(time.time())))
    rep = send({"cmd": "add_track"}, 15000)
    audio_track_id = json.loads(rep)["track_id"]
    rep = send({"cmd": "import_audio", "track_id": audio_track_id,
                "file_path": sine, "offset_time": 0}, 60000)
    log("import_audio", reply=(rep[:200] if rep else None))
    if rep is None or json.loads(rep).get("status") != "ok":
        log("abort", reason="import_audio failed")
        return 4

    empty_wav = os.path.join(RUN_DIR, "empty_range_render.wav")
    drain_telemetry()
    rep = send({"cmd": "start_render", "file_path": empty_wav,
                "range": [10.0, 12.0], "bit_depth": 24}, 20000)
    log("legA_empty_range_start_render", reply=(rep[:220] if rep else None))
    parsed = json.loads(rep) if rep else {}
    job_id = parsed.get("job_id")
    if parsed.get("status") != "ok" or not job_id:
        log("abort", reason="empty-range render did not start", parsed=parsed)
        return 5
    terminal, seen = wait_terminal(job_id, 20.0)
    log("legA_terminal_telemetry", terminal=terminal)
    log("legA_verdict",
        telemetry_arrived=(terminal is not None),
        subtopic=(terminal or {}).get("subtopic"),
        status=(terminal or {}).get("status"),
        message=(terminal or {}).get("message"))

    rep = send({"cmd": "get_project_state"}, 6000)
    log("legA_post_failure_ping", alive=(rep is not None), rtt_probe=True)
    rep = send({"cmd": "ping"}, 6000)
    log("legA_post_failure_ping_cmd", reply=(rep[:80] if rep else None))

    # ---- deterministic async failure: unwritable destination drive -------
    # startOfflineRender ignores the createDirectory() result (pre-existing
    # behavior), so a destination on a non-existent drive passes the sync
    # checks, starts the job, then fails at NodeRenderContext writer open
    # ("Couldn't write to target file") -- a completion callback with a
    # FAILED result and a live nodePlayer: the exact ABBA deadlock shape.
    dead_drive = None
    for letter in ("B:", "A:", "Y:", "Z:", "X:"):
        if not os.path.exists(letter + "\\"):
            dead_drive = letter
            break
    log("dead_drive_selected", drive=dead_drive)
    if dead_drive is None:
        log("abort", reason="no absent drive letter found for unwritable-dest probe")
        return 6
    drain_telemetry()
    bad_wav = dead_drive + "\\vit_fix1_probe\\out.wav"
    rep = send({"cmd": "start_render", "file_path": bad_wav,
                "range": [0.0, 2.0], "bit_depth": 24}, 20000)
    log("unwritable_start_render", reply=(rep[:200] if rep else None))
    parsed = json.loads(rep) if rep else {}
    bad_job = parsed.get("job_id")
    if parsed.get("status") != "ok" or not bad_job:
        log("abort", reason="unwritable-dest render did not start", parsed=parsed)
        return 7
    terminal, _ = wait_terminal(bad_job, 20.0)
    log("unwritable_terminal_telemetry", terminal=terminal)
    rep = send({"cmd": "get_project_state"}, 6000)
    log("unwritable_post_failure_ping", alive=(rep is not None))

    # ---- cancel path: live render ------------------------------------
    live_wav = os.path.join(RUN_DIR, "live_cancel_render.wav")
    drain_telemetry()
    rep = send({"cmd": "start_render", "file_path": live_wav,
                "range": [0.0, 30.0], "bit_depth": 24}, 20000)
    log("cancel_live_start_render", reply=(rep[:200] if rep else None))
    parsed = json.loads(rep) if rep else {}
    live_job = parsed.get("job_id")
    rep = send({"cmd": "cancel_render"}, 6000)
    log("cancel_render_reply", reply=(rep[:160] if rep else None))
    if live_job:
        terminal, _ = wait_terminal(live_job, 30.0)
        log("cancel_terminal_telemetry", terminal=terminal)

    rep = send({"cmd": "get_project_state"}, 6000)
    log("final_ping", alive=(rep is not None))

    # no watchdog false positives during the whole run
    watchdog_events = 0
    for msg in drain_telemetry():
        with open(telemetry_path, "a", encoding="utf-8") as fh:
            fh.write(msg + "\n")
        try:
            obj = json.loads(msg)
            if obj.get("topic") == "render" and obj.get("source") == "render_watchdog":
                watchdog_events += 1
        except ValueError:
            pass
    log("watchdog_quiet", watchdog_events=watchdog_events)
    return 0


if __name__ == "__main__":
    sys.exit(main())
