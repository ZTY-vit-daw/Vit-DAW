# KERNEL-RENDER-FREEZE-1 step-B probe: MIDI setup -> render.start -> frozen confirm.
# Kernel must already be running (started detached). Writes events incrementally.
# NO child-process ownership: never spawns or kills the kernel.
import json
import os
import sys
import time
from datetime import datetime

import zmq

RUN_DIR = sys.argv[1]
REQ_EP = "tcp://127.0.0.1:5555"

ev_path = os.path.join(RUN_DIR, "probe_events.jsonl")


def log(kind, **fields):
    rec = {"t": time.time(), "wall": datetime.now().isoformat(), "kind": kind}
    rec.update(fields)
    with open(ev_path, "a", encoding="utf-8") as fh:
        fh.write(json.dumps(rec, ensure_ascii=False) + "\n")
    print(f"[{rec['wall'][11:23]}] {kind}: {str(fields)[:200]}", flush=True)


def main():
    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(REQ_EP)

    def send(cmd, timeout_ms):
        req.setsockopt(zmq.RCVTIMEO, timeout_ms)
        try:
            req.send_string(json.dumps(cmd))
            return req.recv_string()
        except zmq.Again:
            return None

    # sanity: kernel alive?
    rep = send({"cmd": "get_project_state"}, 4000)
    log("initial_ping", reply=(rep[:150] if rep else None))
    if rep is None:
        log("abort", reason="kernel not responding at start")
        return 3

    def must(cmd, name, timeout_ms=15000):
        rep = send(cmd, timeout_ms)
        log("cmd_reply", name=name, reply=(rep[:400] if rep else None))
        return rep

    rep = must({"cmd": "add_track"}, "add_track")
    track_id = json.loads(rep)["track_id"]
    rep = must({"cmd": "insert_midi_clip", "track_id": track_id,
                "start": 0, "length": 8, "time_unit": "beats"}, "insert_midi_clip")
    clip_id = json.loads(rep)["clip_id"]
    must({"cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
          "time_unit": "beats",
          "notes": [
              {"pitch": 60, "start": 0, "length": 1, "velocity": 90},
              {"pitch": 64, "start": 1, "length": 1, "velocity": 80},
              {"pitch": 67, "start": 2, "length": 1, "velocity": 70},
          ]}, "add_midi_notes")
    must({"cmd": "get_midi_clip_notes", "clip_id": clip_id}, "readback")

    wav = os.path.join(RUN_DIR, "render_probe.wav")
    t0 = time.time()
    rep = send({"cmd": "start_render", "file_path": wav, "bit_depth": 24}, 30000)
    log("render_start_reply", reply=(rep[:300] if rep else None),
        rtt_s=round(time.time() - t0, 2))

    time.sleep(8.0)
    t1 = time.time()
    rep = send({"cmd": "get_project_state"}, 6000)
    log("post_render_ping", reply=None if rep is None else "alive",
        rtt_s=round(time.time() - t1, 2), frozen=(rep is None))

    time.sleep(2.0)
    log("wav_exists", exists=os.path.exists(wav),
        size=(os.path.getsize(wav) if os.path.exists(wav) else 0))
    print("PROBE_DONE", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
