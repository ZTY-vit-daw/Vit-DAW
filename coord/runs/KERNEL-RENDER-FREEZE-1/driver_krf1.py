# KERNEL-RENDER-FREEZE-1 minimal reproduction driver
# Stack: headless kernel only (no agent, no Godot UI). Direct ZMQ legacy commands.
# Reuses scripts/render_wedge_probe.py capture tooling (stacks/wct/minidump).
import json
import os
import subprocess
import sys
import threading
import time
from datetime import datetime

REPO = r"D:\Vit_DAW"
sys.path.insert(0, os.path.join(REPO, "scripts"))
import render_wedge_probe as rwp  # noqa: E402

import zmq  # noqa: E402

RUN_DIR = sys.argv[1]
REQ_EP = "tcp://127.0.0.1:5555"
PUB_EP = "tcp://127.0.0.1:5556"
EXE = os.path.join(REPO, "Export", "staging", "runtime", "VitApp.exe")
WD = os.path.dirname(EXE)

events = []
ev_lock = threading.Lock()


def log(kind, **fields):
    rec = {"t": time.time(), "wall": datetime.now().isoformat(), "kind": kind}
    rec.update(fields)
    with ev_lock:
        events.append(rec)
    print(f"[{rec['wall'][11:23]}] {kind}: {fields}", flush=True)


def flush_events():
    with open(os.path.join(RUN_DIR, "driver_events.jsonl"), "w", encoding="utf-8") as fh:
        for e in events:
            fh.write(json.dumps(e, ensure_ascii=False) + "\n")


def main():
    pub_msgs = []
    pub_stop = threading.Event()

    def pub_reader():
        ctx = zmq.Context()
        sub = ctx.socket(zmq.SUB)
        sub.setsockopt_string(zmq.SUBSCRIBE, "")
        sub.setsockopt(zmq.RCVHWM, 4096)
        sub.connect(PUB_EP)
        while not pub_stop.is_set():
            try:
                raw = sub.recv_string(flags=zmq.NOBLOCK)
                pub_msgs.append({"t": time.time(), "payload": raw})
            except zmq.Again:
                time.sleep(0.05)
        sub.close(0)
        ctx.term()

    def send(sock, cmd_obj, timeout_ms):
        sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
        try:
            sock.send_string(json.dumps(cmd_obj))
            reply = sock.recv_string()
            return reply
        except zmq.Again:
            return None

    ctx = zmq.Context()
    req = ctx.socket(zmq.REQ)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(REQ_EP)

    # 1. start kernel
    t0 = time.time()
    proc = subprocess.Popen(
        [EXE], cwd=WD, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        creationflags=subprocess.CREATE_NO_WINDOW)
    log("kernel_started", pid=proc.pid, exe=EXE)

    threading.Thread(target=pub_reader, daemon=True).start()

    # 2. wait for gateway
    ready = False
    for _ in range(60):
        rep = send(req, {"cmd": "get_project_state"}, 3000)
        if rep is not None:
            ready = True
            break
        time.sleep(0.5)
    log("gateway_ready", ok=ready, wait_s=round(time.time() - t0, 1))
    if not ready:
        log("abort", reason="gateway never ready")
        flush_events()
        return 3

    def must(cmd, name, timeout_ms=15000):
        rep = send(req, cmd, timeout_ms)
        log("cmd_reply", name=name, reply=rep)
        return rep

    # 3. build MIDI track + clip + notes (registered commands per MIDI-RECON-1)
    rep = must({"cmd": "add_track"}, "add_track")
    track_id = json.loads(rep)["track_id"] if rep else None
    rep = must({"cmd": "insert_midi_clip", "track_id": track_id,
                "start": 0, "length": 8, "time_unit": "beats"}, "insert_midi_clip")
    clip_id = json.loads(rep)["clip_id"] if rep else None
    must({"cmd": "add_midi_notes", "track_id": track_id, "clip_id": clip_id,
          "time_unit": "beats",
          "notes": [
              {"pitch": 60, "start": 0, "length": 1, "velocity": 90},
              {"pitch": 64, "start": 1, "length": 1, "velocity": 80},
              {"pitch": 67, "start": 2, "length": 1, "velocity": 70},
          ]}, "add_midi_notes")
    must({"cmd": "get_midi_clip_notes", "clip_id": clip_id}, "readback")

    # 4. render.start (plain legacy name funnels to same startOfflineRender)
    wav = os.path.join(RUN_DIR, "render_probe.wav")
    t_render = time.time()
    rep = send(req, {"cmd": "start_render", "file_path": wav, "bit_depth": 24}, 30000)
    log("render_start_reply", reply=rep, rtt_s=round(time.time() - t_render, 2))

    # 5. freeze probe: ping with short timeout after a short wait
    time.sleep(8.0)
    t_ping = time.time()
    rep = send(req, {"cmd": "get_project_state"}, 6000)
    log("post_render_ping", reply=rep, rtt_s=round(time.time() - t_ping, 2),
        frozen=(rep is None))

    # 6. capture forensics
    pid = proc.pid
    sym_cache = os.path.join(RUN_DIR, "syms")
    try:
        rwp.capture_stacks(pid, os.path.join(RUN_DIR, "stacks_frozen.json"),
                           sym_cache, note="KERNEL-RENDER-FREEZE-1 frozen capture")
        log("stacks_captured", ok=True)
    except Exception as e:
        log("stacks_captured", ok=False, error=repr(e))
    try:
        rwp.capture_wct(pid, os.path.join(RUN_DIR, "wct_frozen.json"),
                        note="KERNEL-RENDER-FREEZE-1 frozen wait chains")
        log("wct_captured", ok=True)
    except Exception as e:
        log("wct_captured", ok=False, error=repr(e))
    try:
        res = rwp.write_minidump(pid, os.path.join(RUN_DIR, "minidump_frozen.dmp"),
                                 note="KERNEL-RENDER-FREEZE-1 frozen dump")
        log("minidump", ok=res.get("ok"), size=res.get("size"))
    except Exception as e:
        log("minidump", ok=False, error=repr(e))

    # 7. observables: output file? pub telemetry?
    time.sleep(2.0)
    pub_stop.set()
    log("wav_exists", path=wav, exists=os.path.exists(wav),
        size=(os.path.getsize(wav) if os.path.exists(wav) else 0))
    render_pub = [m for m in pub_msgs if '"render"' in m["payload"]
                  or "render" in m["payload"].lower()]
    log("pub_render_msgs", count=len(render_pub),
        samples=[m["payload"][:300] for m in render_pub[:5]])
    with open(os.path.join(RUN_DIR, "pub_all.jsonl"), "w", encoding="utf-8") as fh:
        for m in pub_msgs:
            fh.write(json.dumps(m, ensure_ascii=False) + "\n")

    # 8. teardown: graceful attempt, then force kill (recorded per card)
    rep = send(req, {"cmd": "shutdown"}, 4000)
    log("graceful_shutdown_reply", reply=rep)
    time.sleep(2.0)
    alive = proc.poll() is None
    if alive:
        subprocess.run(["taskkill", "/F", "/PID", str(pid)], capture_output=True)
        log("force_killed", pid=pid, method="taskkill /F")
    else:
        log("graceful_exit", code=proc.returncode)
    time.sleep(1.5)
    flush_events()
    print("DRIVER_DONE", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
