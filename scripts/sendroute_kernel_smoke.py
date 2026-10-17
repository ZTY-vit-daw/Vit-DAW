#!/usr/bin/env python3
"""Smoke-test the send/return routing command face on the live kernel ZMQ REQ
face (KERNEL-SENDROUTE-IMPL-1, design MIX-SENDRETURN-DESIGN-1 §5 scenario 1).

Same pattern as track_folder_smoke.py / settempo_persist_smoke.py: kernel
stack only (no agent HTTP face). Legs:

1.  setup — clear_project, two tracks, two generated sine clips (440/880 Hz,
    3 s, 0.5 amplitude) imported; a control offline render fixes the dry
    calibration R440/R880.
2.  connect — track.send.connect auto_create ("Smoke FX Bus") from track 1,
    explicit fx_track_id connect from track 2, amounts pinned at
    -6.0206 dB (0.5x, the probe's wet marker); track.send.list read-back must
    match (2 rows, bus pair 0/1, fx_tracks source_count=2).
3.  rejections (anti-loop gate 1) — FX track as source, self-loop, send
    source as FX target, unknown fx id, set_amount on a nonexistent edge.
4.  idempotency — the identical auto_create resend must not create a second
    track (created_fx_track=false, same fx id, aux plugin count unchanged).
5.  wet render — offline render with both sends at 0.5x: the wet marker
    injects into BOTH target tracks, so each master component must be
    (1 + 2*0.5) = 2.0x its control amplitude (5% tolerance).
6.  undo atomicity — a fresh auto_create (name colliding with a plain track
    → sequence suffix asserted) undone by a single undo must remove track +
    pair + edge (list row count back to 2).
7.  disconnect — both edges removed; the last disconnect must release the FX
    track (fx_released=true, list sends empty) and the comparison render must
    return to the control amplitudes (wet path gone).

Exit code 0 only if every leg passes; summary JSON artifact at --output.
"""

from __future__ import annotations

import argparse
import json
import math
import struct
import sys
import time
import wave
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Tuple

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)

SAMPLE_RATE = 44100
DURATION_S = 3.0
# 0.1 amplitude keeps the summed render (dry + wet) inside the 16-bit writer's
# linear range: this stack's offline render path applies a constant ~2x
# per-channel gain to rendered tracks (pre-existing stock-kernel behaviour,
# verified 2026-10-17: single track / vol 0dB / 0.5 sine renders at 0.9976,
# left-only source doubles L and leaves R at zero). The wet assertions are
# amplitude RATIOS against the control render, so the constant gain cancels —
# only clipping would break linearity, and 0.1 leaves ample headroom.
SINE_AMP = 0.1
WET_AMOUNT_DB = -6.0206  # linear 0.5x, the probe's wet marker
TOLERANCE = 0.05


# ---------------------------------------------------------------- zmq plumbing

def make_socket(ctx: zmq.Context, url: str, timeout_ms: int) -> zmq.Socket:
    sock = ctx.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    if hasattr(zmq, "REQ_RELAXED"):
        sock.setsockopt(zmq.REQ_RELAXED, 1)
    if hasattr(zmq, "REQ_CORRELATE"):
        sock.setsockopt(zmq.REQ_CORRELATE, 1)
    sock.connect(url)
    return sock


def send_command(sock: zmq.Socket, payload: Dict[str, Any]) -> Dict[str, Any]:
    cmd = payload.get("cmd")
    try:
        sock.send_string(json.dumps(payload, ensure_ascii=False))
        raw = sock.recv()
    except zmq.Again as exc:
        raise RuntimeError(f"ZMQ timeout while waiting for {cmd}") from exc
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8", errors="replace")
    try:
        reply = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"invalid JSON reply for {cmd}: {raw[:1000]!r}") from exc
    if not isinstance(reply, dict):
        raise RuntimeError(f"non-object reply for {cmd}: {reply!r}")
    return reply


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def require_ok(label: str, reply: Dict[str, Any]) -> Dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{label} failed: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def require_error(label: str, reply: Dict[str, Any]) -> Dict[str, Any]:
    status = str(reply.get("status", "")).strip().lower()
    if status == "ok":
        raise RuntimeError(f"{label} unexpectedly succeeded: {json.dumps(reply, ensure_ascii=False)}")
    return reply


# ---------------------------------------------------------------- audio helpers

def write_sine_wav(path: Path, freq: float) -> None:
    n = int(SAMPLE_RATE * DURATION_S)
    buf = bytearray()
    for i in range(n):
        v = int(max(-1.0, min(1.0, SINE_AMP * math.sin(2.0 * math.pi * freq * i / SAMPLE_RATE))) * 32767.0)
        buf += struct.pack("<hh", v, v)
    with wave.open(str(path), "wb") as wf:
        wf.setnchannels(2)
        wf.setsampwidth(2)
        wf.setframerate(SAMPLE_RATE)
        wf.writeframes(bytes(buf))


def read_wav_mono(path: Path) -> Tuple[List[float], int]:
    with wave.open(str(path), "rb") as wf:
        channels = wf.getnchannels()
        sampwidth = wf.getsampwidth()
        rate = wf.getframerate()
        frames = wf.readframes(wf.getnframes())
    require(sampwidth == 2, f"expected 16-bit render, got sampwidth={sampwidth}")
    total = len(frames) // (2 * channels)
    mono: List[float] = []
    for i in range(total):
        acc = 0.0
        for c in range(channels):
            off = (i * channels + c) * 2
            (v,) = struct.unpack("<h", frames[off:off + 2])
            acc += v / 32768.0
        mono.append(acc / channels)
    return mono, rate


def goertzel(data: List[float], rate: int, freq: float) -> float:
    start = int(0.5 * rate)
    count = int(2.0 * rate)
    require(len(data) >= start + count, f"render too short for analysis: {len(data)} samples")
    k = 2.0 * math.pi * freq / rate
    coeff = 2.0 * math.cos(k)
    s1 = 0.0
    s2 = 0.0
    for i in range(start, start + count):
        s0 = data[i] + coeff * s1 - s2
        s2 = s1
        s1 = s0
    power = s1 * s1 + s2 * s2 - coeff * s1 * s2
    return math.sqrt(abs(power)) * 2.0 / count


def wait_for_render_file(path: Path, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if path.exists() and path.stat().st_size > 1024:
            time.sleep(0.5)  # settle the writer
            return
        time.sleep(0.5)
    raise RuntimeError(f"render output did not appear within {timeout_s}s: {path}")


# ---------------------------------------------------------------- legs

def leg_setup(sock: zmq.Socket, out_dir: Path) -> Dict[str, Any]:
    require_ok("clear_project", send_command(sock, {"cmd": "clear_project"}))

    # clear_project clears clips but keeps tracks — previous runs (or the
    # workspace template) may leave audio tracks behind, including stale FX
    # tracks that would collide with the idempotency-by-name anchor. Delete
    # every audio track first (the kernel refuses to remove the last one, so
    # stop at that point), then build the fixture fresh.
    while True:
        reply = require_ok("list_tracks", send_command(sock, {"cmd": "list_tracks"}))
        audio = [str(t.get("track_id")) for t in reply.get("tracks", []) if t.get("is_audio_track")]
        if len(audio) <= 1:
            break
        require_ok("delete_track", send_command(sock, {"cmd": "delete_track", "track_id": audio[0]}))

    track_ids: List[str] = []
    for _ in range(2):
        reply = require_ok("add_track", send_command(sock, {"cmd": "add_track"}))
        track_ids.append(str(reply["track_id"]))

    sin440 = out_dir / "sin440.wav"
    sin880 = out_dir / "sin880.wav"
    write_sine_wav(sin440, 440.0)
    write_sine_wav(sin880, 880.0)

    require_ok("import_audio t1", send_command(
        sock, {"cmd": "import_audio", "file_path": str(sin440), "track_id": track_ids[0]}))
    require_ok("import_audio t2", send_command(
        sock, {"cmd": "import_audio", "file_path": str(sin880), "track_id": track_ids[1]}))

    control_wav = out_dir / "render_control.wav"
    require_ok("start_render control", send_command(
        sock, {"cmd": "start_render", "file_path": str(control_wav),
               "range": [0, DURATION_S], "bit_depth": 16}))
    wait_for_render_file(control_wav, 120.0)
    data, rate = read_wav_mono(control_wav)
    r440 = goertzel(data, rate, 440.0)
    r880 = goertzel(data, rate, 880.0)
    require(r440 > 0.05 and r880 > 0.05, f"control render amplitudes too small: {r440} / {r880}")
    return {"track_ids": track_ids, "r440": r440, "r880": r880}


def track_plugin_count(sock: zmq.Socket, track_id: str) -> int:
    reply = require_ok("list_tracks", send_command(sock, {"cmd": "list_tracks"}))
    for track in reply.get("tracks", []):
        if str(track.get("track_id")) == track_id:
            return int(track.get("plugin_count", -1))
    raise RuntimeError(f"track {track_id} not found in list_tracks")


def leg_connect(sock: zmq.Socket, state: Dict[str, Any]) -> Dict[str, Any]:
    t1, t2 = state["track_ids"]
    fx_name = "Smoke FX Bus " + time.strftime("%H%M%S")

    reply = require_ok("connect auto_create", send_command(sock, {
        "cmd": "track.send.connect",
        "source_track_id": t1,
        "auto_create_fx_track": {"name": fx_name},
        "amount_db": WET_AMOUNT_DB,
    }))
    require(bool(reply.get("created_fx_track")) is True, "auto_create must create the FX track")
    require(str(reply.get("fx_name")) == fx_name, f"unexpected fx name: {reply.get('fx_name')}")

    # The allocator picks the smallest free even/odd pair; with a wiped slate
    # that is (0,1), but the assertion holds for any legal pair.
    send_bus = int(reply.get("send_bus", -1))
    return_bus = int(reply.get("return_bus", -1))
    require(send_bus >= 0 and send_bus % 2 == 0 and return_bus == send_bus + 1,
            f"illegal bus pair: {send_bus}/{return_bus}")
    state["send_bus"] = send_bus
    state["return_bus"] = return_bus
    state["fx_name"] = fx_name
    fx_id = str(reply["fx_track_id"])

    reply = require_ok("connect second source", send_command(sock, {
        "cmd": "track.send.connect",
        "source_track_id": t2,
        "fx_track_id": fx_id,
        "amount_db": WET_AMOUNT_DB,
    }))
    require(bool(reply.get("created_fx_track")) is False, "explicit connect must not create tracks")

    listing = require_ok("track.send.list", send_command(sock, {"cmd": "track.send.list"}))
    sends = listing.get("sends", [])
    require(len(sends) == 2, f"list must read back 2 send rows, got {len(sends)}")
    for row in sends:
        require(str(row.get("fx_track_id")) == fx_id, "read-back row must point at the FX track")
        require(int(row.get("send_bus", -1)) == send_bus and int(row.get("return_bus", -1)) == return_bus,
                f"read-back bus pair mismatch: {row}")
        require(abs(float(row.get("amount_db", 0.0)) - WET_AMOUNT_DB) < 0.05,
                f"read-back amount mismatch: {row}")
    fx_rows = listing.get("fx_tracks", [])
    require(len(fx_rows) == 1 and int(fx_rows[0].get("source_count", -1)) == 2,
            f"fx_tracks read-back mismatch: {fx_rows}")
    return {"fx_id": fx_id}


def leg_rejections(sock: zmq.Socket, state: Dict[str, Any]) -> None:
    t1, t2 = state["track_ids"]
    fx_id = state["fx_id"]

    require_error("fx-as-source", send_command(sock, {
        "cmd": "track.send.connect", "source_track_id": fx_id, "fx_track_id": fx_id}))
    require_error("self-loop", send_command(sock, {
        "cmd": "track.send.connect", "source_track_id": t1, "fx_track_id": t1}))
    require_error("send-source-as-fx-target", send_command(sock, {
        "cmd": "track.send.connect", "source_track_id": t1, "fx_track_id": t2}))
    require_error("unknown fx id", send_command(sock, {
        "cmd": "track.send.connect", "source_track_id": t1, "fx_track_id": "999999"}))
    require_error("set_amount no edge", send_command(sock, {
        "cmd": "track.send.set_amount", "source_track_id": t1,
        "fx_track_id": "999998", "amount_db": -3.0}))


def leg_idempotency(sock: zmq.Socket, state: Dict[str, Any]) -> None:
    t1 = state["track_ids"][0]
    before = track_plugin_count(sock, t1)

    reply = require_ok("auto_create resend", send_command(sock, {
        "cmd": "track.send.connect",
        "source_track_id": t1,
        "auto_create_fx_track": {"name": state["fx_name"]},
        "amount_db": WET_AMOUNT_DB,
    }))
    require(bool(reply.get("created_fx_track")) is False, "resend must not rebuild the FX track")
    require(str(reply.get("fx_track_id")) == state["fx_id"], "resend must reuse the same FX track")
    require(track_plugin_count(sock, t1) == before,
            f"idempotent resend must not add plugins ({before} -> {track_plugin_count(sock, t1)})")


def leg_wet_render(sock: zmq.Socket, state: Dict[str, Any], out_dir: Path) -> Dict[str, Any]:
    wet_wav = out_dir / "render_wet.wav"
    require_ok("start_render wet", send_command(sock, {
        "cmd": "start_render", "file_path": str(wet_wav),
        "range": [0, DURATION_S], "bit_depth": 16}))
    wait_for_render_file(wet_wav, 120.0)
    data, rate = read_wav_mono(wet_wav)
    amp440 = goertzel(data, rate, 440.0)
    amp880 = goertzel(data, rate, 880.0)

    ratio440 = amp440 / state["r440"]
    ratio880 = amp880 / state["r880"]
    expected = 1.0 + 2.0 * (10.0 ** (WET_AMOUNT_DB / 20.0))  # 1 + 2*0.5 = 2.0
    require(abs(ratio440 - expected) <= TOLERANCE * expected,
            f"wet render 440 ratio {ratio440:.4f} != {expected}")
    require(abs(ratio880 - expected) <= TOLERANCE * expected,
            f"wet render 880 ratio {ratio880:.4f} != {expected}")
    return {"amp440": amp440, "amp880": amp880, "ratio440": ratio440, "ratio880": ratio880}


def leg_undo_atomicity(sock: zmq.Socket, state: Dict[str, Any]) -> None:
    t1 = state["track_ids"][0]

    # Name collision with a plain track appends a sequence suffix; the whole
    # creation must then vanish under a single undo.
    plain = require_ok("add_track", send_command(sock, {"cmd": "add_track"}))
    plain_name = str(plain.get("track_name", ""))

    reply = require_ok("auto_create collision", send_command(sock, {
        "cmd": "track.send.connect",
        "source_track_id": t1,
        "auto_create_fx_track": {"name": plain_name},
        "amount_db": WET_AMOUNT_DB,
    }))
    require(str(reply.get("fx_name")) == plain_name + " 2",
            f"collision must append a sequence suffix, got: {reply.get('fx_name')}")

    listing = require_ok("list after collision connect", send_command(sock, {"cmd": "track.send.list"}))
    require(len(listing.get("sends", [])) == 3, "collision connect must add a third send row")

    require_ok("undo", send_command(sock, {"cmd": "undo"}))
    listing = require_ok("list after undo", send_command(sock, {"cmd": "track.send.list"}))
    require(len(listing.get("sends", [])) == 2, f"single undo must revert the whole creation: {listing}")


def leg_disconnect(sock: zmq.Socket, state: Dict[str, Any], out_dir: Path) -> Dict[str, Any]:
    t1, t2 = state["track_ids"]
    fx_id = state["fx_id"]

    reply = require_ok("disconnect t1", send_command(sock, {
        "cmd": "track.send.disconnect", "source_track_id": t1, "fx_track_id": fx_id}))
    require(bool(reply.get("fx_released")) is False, "FX must survive while a source remains")
    listing = require_ok("list after disconnect t1", send_command(sock, {"cmd": "track.send.list"}))
    require(len(listing.get("sends", [])) == 1, "one send row must remain")

    reply = require_ok("disconnect t2", send_command(sock, {
        "cmd": "track.send.disconnect", "source_track_id": t2, "fx_track_id": fx_id}))
    require(bool(reply.get("fx_released")) is True, "zero sources must release the FX track")
    listing = require_ok("list after disconnect t2", send_command(sock, {"cmd": "track.send.list"}))
    require(len(listing.get("sends", [])) == 0, "send rows must be empty after full disconnect")
    require(len(listing.get("fx_tracks", [])) == 0, "fx_tracks must be empty after release")

    # Comparison render: back at the control amplitudes — the wet path is gone.
    dry_wav = out_dir / "render_after_disconnect.wav"
    require_ok("start_render after disconnect", send_command(sock, {
        "cmd": "start_render", "file_path": str(dry_wav),
        "range": [0, DURATION_S], "bit_depth": 16}))
    wait_for_render_file(dry_wav, 120.0)
    data, rate = read_wav_mono(dry_wav)
    amp440 = goertzel(data, rate, 440.0)
    amp880 = goertzel(data, rate, 880.0)
    require(abs(amp440 / state["r440"] - 1.0) <= TOLERANCE,
            f"post-disconnect 440 ratio {amp440 / state['r440']:.4f} != 1.0")
    require(abs(amp880 / state["r880"] - 1.0) <= TOLERANCE,
            f"post-disconnect 880 ratio {amp880 / state['r880']:.4f} != 1.0")
    return {"amp440": amp440, "amp880": amp880}


# ---------------------------------------------------------------- main

def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--output", required=True)
    parser.add_argument("--timeout-ms", type=int, default=60000)
    args = parser.parse_args()

    out_dir = Path(args.output).parent
    out_dir.mkdir(parents=True, exist_ok=True)

    ctx = zmq.Context.instance()
    sock = make_socket(ctx, args.url, args.timeout_ms)

    summary: Dict[str, Any] = {
        "feature": "sendroute_kernel_smoke",
        "started_utc": datetime.now(timezone.utc).isoformat(),
        "url": args.url,
        "legs": [],
    }

    def record(leg: str, detail: Dict[str, Any]) -> None:
        detail = dict(detail)
        detail.update({"leg": leg, "pass": True})
        summary["legs"].append(detail)
        print(f"[leg] {leg}: PASS " + json.dumps(detail, ensure_ascii=False)[:400])

    try:
        require_ok("ping", send_command(sock, {"cmd": "ping"}))

        state = leg_setup(sock, out_dir)
        record("setup", {"r440": state["r440"], "r880": state["r880"]})

        state.update(leg_connect(sock, state))
        record("connect", {"fx_id": state["fx_id"], "fx_name": state["fx_name"]})

        leg_rejections(sock, state)
        record("rejections", {})

        leg_idempotency(sock, state)
        record("idempotency", {})

        wet = leg_wet_render(sock, state, out_dir)
        record("wet_render", wet)

        leg_undo_atomicity(sock, state)
        record("undo_atomicity", {})

        dry = leg_disconnect(sock, state, out_dir)
        record("disconnect", dry)
    except RuntimeError as exc:
        summary["error"] = str(exc)
        summary["pass"] = False
        print(f"FAIL: {exc}", file=sys.stderr)
        Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        return 1

    summary["pass"] = True
    summary["finished_utc"] = datetime.now(timezone.utc).isoformat()
    Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print("sendroute kernel smoke: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
