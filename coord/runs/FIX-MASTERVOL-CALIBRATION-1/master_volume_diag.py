#!/usr/bin/env python3
"""FIX-MASTERVOL-CALIBRATION-1 (134) kernel-stack leg: master-volume default calibration.

Talks to a live headless VitApp kernel over its ZMQ REQ command face
(tcp://127.0.0.1:5555) and measures offline-render amplitudes per channel.

Context: KERNEL-RENDERGAIN-1 (132) proved the stock kernel rendered +6.000 dB hot
because the project master-volume state carried fader position 1.0 (the engine maps
pos=1.0 -> +6.0 dB). This card changed the three project templates to the -3.0 dB
fader position (0.6376281516) and deliberately does NOT migrate existing projects.

Legs (deterministic; zero LLM):
  L0  startup readback: startup project file MASTERVOLUME attr = 0.6376281516;
      list_tracks master volume_db is null (master volume stays user-invisible).
  L1  startup default project: clear_project -> add_track -> import 0.5 sine ->
      render 16bit -> ratio ~ 0.7079 (-3.0 dB, was 1.9953 / +6 dB before the fix).
  L2  new_project path (blank template from VitPaths.h): add/import/render ->
      ratio ~ 0.7079.
  L3  bit depth 16/24/32 on the new_project edit -> ratio invariant ~ 0.7079
      (writer is not the gain source).
  L4  hot source (0.9 peak) on the new_project edit: 16/24/32-bit renders no longer
      clamp (peak ~ 0.6372); the pre-fix +6 dB counterfactual (0.9 * 1.9953 = 1.7957)
      is recorded as "would have clipped at 16/24-bit".
  L5  legacy project fixture with MASTERVOLUME volume="1.0" -> still renders
      ~ 1.9952 (+6 dB): existing projects are NOT silently migrated (policy).
  L6  44.1k source on the new_project edit -> ratio ~ 0.7079 (resample path
      does not change the master gain).

Outputs: <artifacts>/diag_summary.json plus every rendered wav and the fixture XMLs
used. Exit 0 only if all legs pass their expected values.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import struct
import sys
import time
import wave
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

# Fader-position law of the pinned engine (tracktion_AudioUtilities.cpp:136-156):
#   volumeFaderPositionToDB(pos) = 20*ln(pos) + 6 ; decibelsToVolumeFaderPosition(db) = exp((db-6)/20)
POS_100 = 1.0
POS_MINUS3DB = 0.6376281516217733
GAIN_POS_100 = 10.0 ** (6.0 / 20.0)         # +6.000 dB (legacy template value)
GAIN_POS_MINUS3DB = 10.0 ** (-3.0 / 20.0)   # -3.000 dB (fixed template value)
FIXED_TEMPLATE_VOLUME_ATTR = "0.6376281516"

SINE_FREQ = 440.0
DURATION_S = 3.0
TOL_GAIN = 0.01          # 1% on the ratio (16-bit quantization + writer rounding)
TOL_HOT_PEAK = 0.02      # 2% on the hot-source linear peak
CLIP_PEAK = 0.999        # 16/24-bit PCM full-scale clamp indicator

LEGACY_FIXTURE_TEMPLATE = """<?xml version="1.0" encoding="UTF-8"?>
<EDIT appVersion="Unknown" projectID="0/{project_id}" creationTime="0" modifiedBy="master_volume_diag">
  <TRANSPORT endToEnd="1" position="0.0"/>
  <TEMPOSEQUENCE>
    <TEMPO startBeat="0.0" bpm="120.0" curve="1.0"/>
    <TIMESIG numerator="4" denominator="4" startBeat="0.0"/>
  </TEMPOSEQUENCE>
  <VIDEO/>
  <CLICKTRACK level="0.6000000238418579"/>
  <MASTERVOLUME>
    <PLUGIN type="volume" id="1001" enabled="1" volume="1.0">
      <MODIFIERASSIGNMENTS/>
    </PLUGIN>
  </MASTERVOLUME>
  <RACKS/>
  <MASTERPLUGINS/>
  <INPUTDEVICES>
    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins"/>
  </INPUTDEVICES>
  <TRACKCOMPS/>
  <ARRANGERTRACK name="Arranger" id="1002"/>
  <CHORDTRACK name="Chord" id="1003"/>
  <MARKERTRACK name="Marker" id="1004"/>
  <TEMPOTRACK name="Global" id="1005">
    <MODIFIERS/>
  </TEMPOTRACK>
  <MASTERTRACK name="Master" id="1006">
    <MODIFIERS/>
  </MASTERTRACK>
  <SCENES/>
</EDIT>
"""


# ------------------------------------------------------------------ zmq plumbing

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


def is_ok(reply: Dict[str, Any]) -> bool:
    return str(reply.get("status", "")).strip().lower() in {"ok", "success", "completed"}


def require_ok(label: str, reply: Dict[str, Any]) -> Dict[str, Any]:
    if not is_ok(reply):
        raise RuntimeError(f"{label} failed: {json.dumps(reply, ensure_ascii=False)[:1200]}")
    return reply


def command_retry(sock: zmq.Socket, payload: Dict[str, Any], attempts: int = 40,
                  delay_s: float = 0.5) -> Dict[str, Any]:
    """Retry only the kernel's transient 'render already in progress' answer."""
    last: Dict[str, Any] = {}
    for _ in range(attempts):
        last = send_command(sock, payload)
        if not (not is_ok(last) and "already in progress" in json.dumps(last)):
            return last
        time.sleep(delay_s)
    return last


# ------------------------------------------------------------------ audio helpers

def write_sine_wav(path: Path, freq: float, amp: float, rate: int, channels: int) -> None:
    n = int(rate * DURATION_S)
    buf = bytearray()
    for i in range(n):
        v = int(max(-1.0, min(1.0, amp * math.sin(2.0 * math.pi * freq * i / rate))) * 32767.0)
        if channels == 1:
            buf += struct.pack("<h", v)
        else:
            buf += struct.pack("<hh", v, v)
    with wave.open(str(path), "wb") as wf:
        wf.setnchannels(channels)
        wf.setsampwidth(2)
        wf.setframerate(rate)
        wf.writeframes(bytes(buf))


def read_wav_channels(path: Path) -> Tuple[List[List[float]], int, int, int, str]:
    """Return (per-channel samples, rate, sampwidth, nchannels, format).

    start_render bit_depth=32 writes an IEEE-float WAV (RIFF format tag 3) in this
    build; python's `wave` module cannot read it, so float files fall back to
    soundfile. PCM files use the same 0x7fff / 0x7fffff / 0x7fffffff normalization
    as the JUCE writer.
    """
    try:
        with wave.open(str(path), "rb") as wf:
            channels = wf.getnchannels()
            sampwidth = wf.getsampwidth()
            rate = wf.getframerate()
            frames = wf.readframes(wf.getnframes())
        fmt = "pcm"
    except wave.Error as exc:
        if "unknown format" not in str(exc):
            raise
        import soundfile  # local import: only the float32 leg needs it

        data, rate = soundfile.read(str(path), dtype="float64", always_2d=True)
        channels = int(data.shape[1])
        samples_f = [list(data[:, c]) for c in range(channels)]
        return samples_f, int(rate), 4, channels, "float32"

    samples: List[List[float]] = [[] for _ in range(channels)]
    if sampwidth == 2:
        vals = struct.unpack("<%dh" % (len(frames) // 2), frames)
        maxv = 32767.0
    elif sampwidth == 3:
        vals = []
        for i in range(0, len(frames), 3):
            b = frames[i:i + 3]
            v = b[0] | (b[1] << 8) | (b[2] << 16)
            if v >= 0x800000:
                v -= 0x1000000
            vals.append(v)
        maxv = 8388607.0
    elif sampwidth == 4:
        vals = struct.unpack("<%di" % (len(frames) // 4), frames)
        maxv = 2147483647.0
    else:
        raise RuntimeError(f"unsupported sample width {sampwidth}: {path}")
    for i, v in enumerate(vals):
        samples[i % channels].append(v / maxv)
    return samples, rate, sampwidth, channels, fmt


def goertzel_amp(samples: List[float], rate: int, freq: float) -> float:
    start = int(0.5 * rate)
    count = int(2.0 * rate)
    if len(samples) < start + count:
        raise RuntimeError(f"render too short for analysis: {len(samples)} samples")
    k = 2.0 * math.pi * freq / rate
    coeff = 2.0 * math.cos(k)
    s1 = 0.0
    s2 = 0.0
    for i in range(start, start + count):
        s0 = samples[i] + coeff * s1 - s2
        s2 = s1
        s1 = s0
    power = s1 * s1 + s2 * s2 - coeff * s1 * s2
    return math.sqrt(abs(power)) * 2.0 / count


def analyze_wav(path: Path) -> Dict[str, Any]:
    chans, rate, sampwidth, nch, fmt = read_wav_channels(path)
    peaks = [round(max(abs(v) for v in ch), 8) for ch in chans]
    rms = [round(math.sqrt(sum(v * v for v in ch) / len(ch)), 8) for ch in chans]
    amps = []
    for ch in chans:
        try:
            amps.append(round(goertzel_amp(ch, rate, SINE_FREQ), 8))
        except RuntimeError:
            amps.append(None)
    return {
        "file": path.name,
        "rate": rate,
        "sampwidth": sampwidth,
        "format": fmt,
        "channels": nch,
        "peak": peaks,
        "rms": rms,
        "amp440": amps,
        "bytes": path.stat().st_size,
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest()[:16],
    }


def wait_for_render(path: Path, timeout_s: float = 120.0) -> None:
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if path.exists() and path.stat().st_size > 1024:
            size = path.stat().st_size
            time.sleep(0.4)
            if path.exists() and path.stat().st_size == size:
                return
        time.sleep(0.3)
    raise RuntimeError(f"render output did not appear/settle within {timeout_s}s: {path}")


def file_master_volume(path: Path) -> Optional[str]:
    """Read the MASTERVOLUME volume attribute straight from a project XML file."""
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return None
    start = text.find("<MASTERVOLUME>")
    end = text.find("</MASTERVOLUME>")
    if start < 0 or end < 0:
        return None
    block = text[start:end]
    marker = 'volume="'
    idx = block.find(marker)
    if idx < 0:
        return "<absent_attr>"
    idx += len(marker)
    return block[idx:block.find('"', idx)]


def snapshot_master_volume(state: Dict[str, Any]) -> Optional[str]:
    xml = str(state.get("snapshot_xml", ""))
    if "MASTERVOLUME" not in xml:
        return None
    start = xml.find("<MASTERVOLUME>")
    end = xml.find("</MASTERVOLUME>")
    if start < 0 or end < 0:
        return None
    block = xml[start:end]
    marker = 'volume="'
    idx = block.find(marker)
    if idx < 0:
        return "<absent_attr>"
    idx += len(marker)
    return block[idx:block.find('"', idx)]


# ------------------------------------------------------------------ kernel helpers

class Kernel:
    def __init__(self, sock: zmq.Socket, artifacts: Path):
        self.sock = sock
        self.artifacts = artifacts

    def cmd(self, payload: Dict[str, Any]) -> Dict[str, Any]:
        return send_command(self.sock, payload)

    def ok(self, payload: Dict[str, Any], label: str = "", retry_render: bool = False) -> Dict[str, Any]:
        reply = command_retry(self.sock, payload) if retry_render else self.cmd(payload)
        return require_ok(label or str(payload.get("cmd")), reply)

    def project_state(self) -> Dict[str, Any]:
        return self.ok({"cmd": "get_project_state"}, "get_project_state")

    def clear_and_single_track(self) -> str:
        """clear_project (keeps tracks) then collapse to one silent track and add a fresh one."""
        self.ok({"cmd": "clear_project"}, "clear_project", retry_render=True)
        while True:
            listing = self.ok({"cmd": "list_tracks"}, "list_tracks")
            audio = [str(t.get("track_id")) for t in listing.get("tracks", []) if t.get("is_audio_track")]
            if len(audio) <= 1:
                break
            self.ok({"cmd": "delete_track", "track_id": audio[0]}, "delete_track")
        reply = self.ok({"cmd": "add_track"}, "add_track")
        return str(reply["track_id"])

    def import_audio(self, track_id: str, wav: Path) -> Dict[str, Any]:
        return self.ok({"cmd": "import_audio", "file_path": str(wav), "track_id": track_id},
                       "import_audio", retry_render=True)

    def new_project(self) -> Dict[str, Any]:
        return self.ok({"cmd": "new_project"}, "new_project", retry_render=True)

    def load_project(self, fixture: Path) -> Dict[str, Any]:
        live = self.artifacts / "fixtures_live" / f"{fixture.stem}_live.vit"
        live.parent.mkdir(parents=True, exist_ok=True)
        live.write_bytes(fixture.read_bytes())
        return self.ok({"cmd": "load_project", "file_path": str(live)}, "load_project", retry_render=True)

    def render(self, name: str, bit_depth: int = 16,
               use_master_plugins: bool = True) -> Tuple[Path, Dict[str, Any]]:
        out = self.artifacts / "renders" / f"{name}.wav"
        out.parent.mkdir(parents=True, exist_ok=True)
        if out.exists():
            out.unlink()
        payload = {
            "cmd": "start_render",
            "file_path": str(out),
            "range": [0.0, DURATION_S],
            "bit_depth": bit_depth,
            "use_master_plugins": use_master_plugins,
        }
        reply = self.ok(payload, f"start_render:{name}", retry_render=True)
        wait_for_render(out)
        stats = analyze_wav(out)
        stats["requested"] = {"bit_depth": bit_depth, "use_master_plugins": use_master_plugins}
        stats["job_id"] = reply.get("job_id")
        return out, stats


# ------------------------------------------------------------------ experiments

def ratio(stats: Dict[str, Any], source_amp: float, channel: int = 0) -> float:
    amp = stats["amp440"][channel]
    if amp is None:
        raise RuntimeError(f"no 440 amplitude for {stats['file']}")
    return amp / source_amp


def delta_db(measured: float) -> float:
    return 20.0 * math.log10(measured) if measured > 0 else -999.0


def close(a: float, b: float, tol: float = TOL_GAIN) -> bool:
    return abs(a - b) <= tol * max(abs(b), 1e-9)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--output", required=True, help="diag_summary.json path")
    parser.add_argument("--startup-project", default="",
                        help="project XML the kernel loaded at startup (L0 readback evidence)")
    parser.add_argument("--timeout-ms", type=int, default=120000)
    args = parser.parse_args()

    out_path = Path(args.output).resolve()
    artifacts = out_path.parent
    artifacts.mkdir(parents=True, exist_ok=True)
    for sub in ("sources", "fixtures", "fixtures_live", "renders"):
        (artifacts / sub).mkdir(exist_ok=True)

    summary: Dict[str, Any] = {
        "feature": "master_volume_diag",
        "card": "FIX-MASTERVOL-CALIBRATION-1",
        "started_utc": datetime.now(timezone.utc).isoformat(),
        "url": args.url,
        "expected": {
            "fixed_template_volume_attr": FIXED_TEMPLATE_VOLUME_ATTR,
            "gain_pos_1_0": GAIN_POS_100,
            "gain_pos_minus3db": GAIN_POS_MINUS3DB,
            "gain_pos_minus3db_db": 20.0 * math.log10(GAIN_POS_MINUS3DB),
        },
        "legs": [],
        "pass": False,
    }
    legs = summary["legs"]
    ctx = zmq.Context.instance()
    sock = make_socket(ctx, args.url, args.timeout_ms)
    kernel = Kernel(sock, artifacts)

    def record(leg: str, detail: Dict[str, Any], ok: bool) -> None:
        entry = dict(detail)
        entry.update({"leg": leg, "pass": bool(ok)})
        legs.append(entry)
        flag = "PASS" if ok else "FAIL"
        print(f"[leg] {leg}: {flag} " + json.dumps(entry, ensure_ascii=False)[:600])

    # ---- sources ---------------------------------------------------------
    src_48k_05 = artifacts / "sources" / "sine440_48k_stereo_0.5.wav"
    write_sine_wav(src_48k_05, SINE_FREQ, 0.5, 48000, 2)
    src_441 = artifacts / "sources" / "sine440_44100_stereo_0.5.wav"
    write_sine_wav(src_441, SINE_FREQ, 0.5, 44100, 2)
    src_mono = artifacts / "sources" / "sine440_48k_mono_0.5.wav"
    write_sine_wav(src_mono, SINE_FREQ, 0.5, 48000, 1)
    src_hot = artifacts / "sources" / "sine440_48k_stereo_0.9.wav"
    write_sine_wav(src_hot, SINE_FREQ, 0.9, 48000, 2)

    source_stats = {}
    for p in (src_48k_05, src_441, src_mono, src_hot):
        source_stats[p.name] = analyze_wav(p)
    summary["sources"] = source_stats

    # legacy project fixture: fader position 1.0 = +6.0 dB, must stay un-migrated
    legacy_fixture = artifacts / "fixtures" / "fixture_legacy_master_1.0.vit"
    legacy_fixture.write_text(
        LEGACY_FIXTURE_TEMPLATE.format(project_id="fglgcy"), encoding="utf-8", newline="\n")
    summary["fixtures"] = {"legacy_master_1.0": {
        "path": str(legacy_fixture),
        "sha256": hashlib.sha256(legacy_fixture.read_bytes()).hexdigest(),
        "master_volume_attr": file_master_volume(legacy_fixture),
    }}

    try:
        require_ok("ping", kernel.cmd({"cmd": "ping"}))

        # =============================================================== L0
        state = kernel.project_state()
        startup_file_vol = file_master_volume(Path(args.startup_project)) if args.startup_project else None
        listing = kernel.ok({"cmd": "list_tracks"}, "list_tracks")
        master_rows = [t for t in listing.get("tracks", []) if str(t.get("track_type")) == "master"]
        l0 = {
            "startup_project_file": args.startup_project,
            "startup_project_master_volume_attr": startup_file_vol,
            "kernel_snapshot_master_volume_attr": snapshot_master_volume(state),
            "master_track_volume_db": [(r.get("track_id"), r.get("volume_db")) for r in master_rows],
            "project_path": state.get("project_path"),
            "note": "the kernel command face exposes no master-volume readback; the startup project file "
                    "is the state source of truth and list_tracks reports no master volume db.",
        }
        summary["L0_startup_readback"] = l0
        record("L0_startup_readback", l0,
               ok=(startup_file_vol == FIXED_TEMPLATE_VOLUME_ATTR
                   and all(db is None for _, db in l0["master_track_volume_db"])))

        # =============================================================== L1
        kernel.clear_and_single_track()
        track_id = kernel.clear_and_single_track()
        kernel.import_audio(track_id, src_48k_05)
        _, st = kernel.render("L1_startup_project_16bit", 16, True)
        measured = ratio(st, 0.5)
        summary["L1_startup_default_project"] = {"stats": st, "ratio": measured, "db": delta_db(measured)}
        record("L1_startup_default_project",
               {"ratio": measured, "db": round(delta_db(measured), 6),
                "expected_ratio": round(GAIN_POS_MINUS3DB, 6), "expected_db": -3.0},
               close(measured, GAIN_POS_MINUS3DB))

        # =============================================================== L2
        kernel.new_project()
        state = kernel.project_state()
        new_project_volume_attr = snapshot_master_volume(state)
        track_id = kernel.clear_and_single_track()
        kernel.import_audio(track_id, src_48k_05)
        _, st = kernel.render("L2_new_project_16bit", 16, True)
        measured = ratio(st, 0.5)
        summary["L2_new_project_blank_template"] = {
            "stats": st, "ratio": measured, "db": delta_db(measured),
            "snapshot_master_volume_attr": new_project_volume_attr,
            "note": "blank template is the hard-coded XML in VitPaths.h; its MASTERVOLUME volume "
                    "attribute is now 0.6376281516 (-3.0 dB).",
        }
        record("L2_new_project_blank_template",
               {"ratio": measured, "db": round(delta_db(measured), 6),
                "snapshot_master_volume_attr": new_project_volume_attr,
                "expected_ratio": round(GAIN_POS_MINUS3DB, 6)},
               close(measured, GAIN_POS_MINUS3DB))

        # =============================================================== L3 bit depth
        bit_results = {}
        for bd in (16, 24, 32):
            _, st = kernel.render(f"L3_bitdepth{bd}", bd, True)
            r = ratio(st, 0.5)
            bit_results[bd] = {"stats": st, "ratio": r, "db": delta_db(r)}
        summary["L3_bitdepth"] = bit_results
        record("L3_bitdepth_invariance",
               {str(k): {"ratio": round(v["ratio"], 6), "sampwidth": v["stats"]["sampwidth"],
                         "format": v["stats"]["format"]} for k, v in bit_results.items()},
               all(close(v["ratio"], GAIN_POS_MINUS3DB) for v in bit_results.values()))

        # =============================================================== L4 hot-source clipping
        track_id = kernel.clear_and_single_track()
        kernel.import_audio(track_id, src_hot)
        _, st16 = kernel.render("L4_hot_16bit", 16, True)
        _, st24 = kernel.render("L4_hot_24bit", 24, True)
        _, st32 = kernel.render("L4_hot_32bit_float", 32, True)
        expected_hot = 0.9 * GAIN_POS_MINUS3DB           # 0.637151... after the fix
        legacy_would_be = 0.9 * GAIN_POS_100             # 1.795736... before the fix (+6 dB)
        peaks = {"16": st16["peak"][0], "24": st24["peak"][0], "32_float": st32["peak"][0]}
        summary["L4_hot_source_clipping"] = {
            "source_amp": 0.9,
            "expected_linear_peak": expected_hot,
            "legacy_plus6db_peak_counterfactual": legacy_would_be,
            "peak_16": st16["peak"], "peak_24": st24["peak"], "peak_32_float": st32["peak"],
            "amp16": st16["amp440"], "amp32_float": st32["amp440"],
            "formats": {"16": st16["format"], "24": st24["format"], "32": st32["format"]},
        }
        ok4 = (all(abs(p - expected_hot) <= TOL_HOT_PEAK * expected_hot for p in peaks.values())
               and all(p < CLIP_PEAK for p in peaks.values())
               and legacy_would_be > 1.0)
        record("L4_hot_source_clipping",
               {"peak16": peaks["16"], "peak24": peaks["24"], "peak32_float": peaks["32_float"],
                "expected_linear_peak": round(expected_hot, 6),
                "legacy_plus6db_peak_counterfactual": round(legacy_would_be, 6),
                "formats": summary["L4_hot_source_clipping"]["formats"],
                "note": "16/24/32-bit renders all stay below unity after the -3 dB fix; at the old +6 dB "
                        "the linear peak was 1.7957 and 16/24-bit clamped (132 E5)."},
               ok4)

        # =============================================================== L5 legacy project un-migrated
        kernel.load_project(legacy_fixture)
        state = kernel.project_state()
        legacy_readback = snapshot_master_volume(state)
        track_id = kernel.clear_and_single_track()
        kernel.import_audio(track_id, src_48k_05)
        _, st = kernel.render("L5_legacy_master_1.0", 16, True)
        measured = ratio(st, 0.5)
        summary["L5_legacy_project_not_migrated"] = {
            "stats": st, "ratio": measured, "db": delta_db(measured),
            "fixture_master_volume_attr": file_master_volume(legacy_fixture),
            "snapshot_master_volume_attr": legacy_readback,
            "note": "existing project with fader position 1.0 still renders +6.0 dB: the fix only changes "
                    "templates/new projects; stored projects are not silently migrated (policy).",
        }
        record("L5_legacy_project_not_migrated",
               {"ratio": measured, "db": round(delta_db(measured), 6),
                "fixture_master_volume_attr": "1.0", "expected_ratio": round(GAIN_POS_100, 6)},
               close(measured, GAIN_POS_100))

        # =============================================================== L6 44.1k source
        kernel.new_project()
        track_id = kernel.clear_and_single_track()
        kernel.import_audio(track_id, src_441)
        _, st = kernel.render("L6_new_project_44k1", 16, True)
        measured = ratio(st, 0.5)
        summary["L6_new_project_44k1_source"] = {"stats": st, "ratio": measured, "db": delta_db(measured)}
        record("L6_new_project_44k1_source",
               {"ratio": measured, "db": round(delta_db(measured), 6),
                "expected_ratio": round(GAIN_POS_MINUS3DB, 6)},
               close(measured, GAIN_POS_MINUS3DB))

        summary["pass"] = all(leg["pass"] for leg in legs)
    except Exception as exc:  # noqa: BLE001 - diagnostic harness must record failures
        summary["error"] = f"{type(exc).__name__}: {exc}"
        summary["pass"] = False
        print(f"FATAL: {summary['error']}", file=sys.stderr)
    finally:
        summary["finished_utc"] = datetime.now(timezone.utc).isoformat()
        out_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        sock.close(0)
        ctx.term()

    print("master volume diag: " + ("PASS" if summary.get("pass") else "FAIL"))
    return 0 if summary.get("pass") else 1


if __name__ == "__main__":
    sys.exit(main())
