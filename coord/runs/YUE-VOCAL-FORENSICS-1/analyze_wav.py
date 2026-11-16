"""YUE-VOCAL-FORENSICS-1 question 2/3: offline WAV forensics (zero-stack).

Analyzes the two backed-up YuE2 renders from run_20261116_000009 (main-repo
artifacts, opened READ-ONLY) and emits:
  - RIFF facts (exact duration from the data chunk size)
  - energy envelope with adaptive silence threshold -> audible segments
  - coarse pitch track (autocorrelation f0, 70-800 Hz) -> voiced segments
    with mean f0 / f0 spread / harmonic-peak strength
  - 4-band spectral energy share timeline (singing-voice formant band 2-5k
    is reported per segment as a heuristic singing-voice indicator)

Heuristic caveat (recorded in REPORT.md): this is a coarse acoustic
locator, not a vocal/instrument classifier. The user's listening report
(v1: full lyrics/vocals; v2: one line only) is the primary ground truth
this analysis cross-checks.

Output: segments JSON + human-readable table next to this script.
Run:  python -I analyze_wav.py <wav1> <wav2> <outdir>
"""

import json
import os
import sys
import wave

import numpy as np

HOP_S = 0.010
WIN_S = 0.046
FMIN, FMAX = 70.0, 800.0
HARM_PEAK_MIN = 0.45      # autocorrelation peak strength floor for voiced
MERGE_GAP_S = 0.35        # merge voiced segments separated by < this
MIN_SEG_S = 0.18          # drop segments shorter than this


def read_wav_mono(path):
    with wave.open(path, "rb") as w:
        sr = w.getframerate()
        ch = w.getnchannels()
        sw = w.getsampwidth()
        n = w.getnframes()
        raw = w.readframes(n)
    if sw == 2:
        data = np.frombuffer(raw, dtype="<i2").astype(np.float64) / 32768.0
    elif sw == 4:
        data = np.frombuffer(raw, dtype="<i4").astype(np.float64) / 2147483648.0
    else:
        raise SystemExit("unsupported sample width %d" % sw)
    if ch == 2:
        data = data.reshape(-1, 2).mean(axis=1)
    return {
        "sr": sr, "channels": ch, "sample_width": sw, "frames": n,
        "duration_s": n / float(sr), "mono": data,
    }


def energy_envelope(mono, sr):
    hop = int(HOP_S * sr)
    win = int(WIN_S * sr)
    n_frames = max(0, 1 + (len(mono) - win) // hop)
    rms = np.zeros(n_frames)
    for i in range(n_frames):
        x = mono[i * hop:i * hop + win]
        rms[i] = np.sqrt(np.mean(x * x))
    return rms, hop


def f0_track(mono, sr, rms_gate):
    hop = int(HOP_S * sr)
    win = int(WIN_S * sr)
    n_frames = len(rms_gate)
    f0 = np.zeros(n_frames)
    harm = np.zeros(n_frames)
    lo_lag, hi_lag = int(sr / FMAX), int(sr / FMIN)
    for i in range(n_frames):
        if not rms_gate[i]:
            continue
        x = mono[i * hop:i * hop + win]
        x = x - x.mean()
        energy = np.sum(x * x)
        if energy <= 0:
            continue
        ac = np.correlate(x, x, "full")[win - 1:]
        ac = ac / (ac[0] + 1e-12)
        seg = ac[lo_lag:hi_lag]
        k = int(np.argmax(seg)) + lo_lag
        if ac[k] >= HARM_PEAK_MIN:
            f0[i] = sr / float(k)
            harm[i] = float(ac[k])
    return f0, harm


def merge_segments(flags, hop_s):
    """flags: boolean per-frame -> [(start_s, end_s), ...] after merge+minlen."""
    segs = []
    start = None
    for i, f in enumerate(flags):
        if f and start is None:
            start = i
        elif not f and start is not None:
            segs.append([start * hop_s, i * hop_s])
            start = None
    if start is not None:
        segs.append([start * hop_s, len(flags) * hop_s])
    merged = []
    for s in segs:
        if merged and s[0] - merged[-1][1] < MERGE_GAP_S:
            merged[-1][1] = s[1]
        else:
            merged.append(s)
    return [(s, e) for s, e in merged if e - s >= MIN_SEG_S]


def band_share(mono, sr, segs):
    """Per segment: energy share in 4 bands from 4096-point FFT frames."""
    nfft = 4096
    hop = int(0.05 * sr)
    bands = [(0, 200), (200, 2000), (2000, 5000), (5000, sr // 2)]
    out = []
    for (s, e) in segs:
        i0, i1 = int(s * sr), int(e * sr)
        acc = np.zeros(len(bands))
        frames = 0
        for i in range(i0, max(i0, i1 - nfft), hop):
            x = mono[i:i + nfft]
            if len(x) < nfft:
                break
            x = x * np.hanning(nfft)
            spec = np.abs(np.fft.rfft(x)) ** 2
            freqs = np.fft.rfftfreq(nfft, 1.0 / sr)
            for bi, (f0, f1) in enumerate(bands):
                mask = (freqs >= f0) & (freqs < f1)
                acc[bi] += float(spec[mask].sum())
            frames += 1
        total = float(acc.sum()) or 1.0
        out.append({
            "start_s": round(s, 2), "end_s": round(e, 2),
            "band_share": {
                "lt200hz": round(acc[0] / total, 3),
                "200_2k": round(acc[1] / total, 3),
                "2k_5k_formant": round(acc[2] / total, 3),
                "gt5k": round(acc[3] / total, 3),
            },
            "frames": frames,
        })
    return out


def analyze(path):
    info = read_wav_mono(path)
    mono, sr = info["mono"], info["sr"]
    rms, hop = energy_envelope(mono, sr)
    hop_s = HOP_S
    peak = float(np.percentile(rms, 99)) if len(rms) else 0.0
    # Continuous music fills the window: a median-relative floor exceeds the
    # peak (observed 0.40 thr vs 0.39 peak on v1). Use a peak-relative gate.
    thr = max(peak * 0.01, 1e-4)
    gate = rms >= thr
    f0, harm = f0_track(mono, sr, gate)
    voiced = f0 > 0
    segs = merge_segments(voiced, hop_s)
    band_rows = band_share(mono, sr, segs)
    detail = []
    for (s, e), band in zip(segs, band_rows):
        i0, i1 = int(s / hop_s), int(e / hop_s)
        fv = f0[i0:i1]
        hv = harm[i0:i1]
        fv = fv[fv > 0]
        detail.append({
            "start_s": round(s, 2), "end_s": round(e, 2),
            "len_s": round(e - s, 2),
            "f0_mean_hz": round(float(np.mean(fv)), 1) if len(fv) else None,
            "f0_min_hz": round(float(np.min(fv)), 1) if len(fv) else None,
            "f0_max_hz": round(float(np.max(fv)), 1) if len(fv) else None,
            "f0_cv": round(float(np.std(fv) / np.mean(fv)), 3) if len(fv) > 1 else None,
            "harm_peak_mean": round(float(np.mean(hv[hv > 0])), 3) if np.any(hv > 0) else None,
            "band_share": band["band_share"],
        })
    audible_segs = merge_segments(gate, hop_s)
    return {
        "file": os.path.basename(path),
        "sr": sr, "channels": info["channels"],
        "duration_from_data_chunk_s": round(info["duration_s"], 3),
        "rms_threshold": round(thr, 6),
        "audible_time_s": round(sum(e - s for s, e in audible_segs), 2),
        "audible_segments": [(round(s, 2), round(e, 2)) for s, e in audible_segs],
        "voiced_segment_count": len(detail),
        "voiced_time_s": round(sum(d["len_s"] for d in detail), 2),
        "voiced_segments": detail,
    }


def main():
    wav1, wav2, outdir = sys.argv[1], sys.argv[2], sys.argv[3]
    os.makedirs(outdir, exist_ok=True)
    res1, res2 = analyze(wav1), analyze(wav2)
    doc = {"schema_version": "yue_vocal_forensics_wav.v1", "v1": res1, "v2": res2}
    out_json = os.path.join(outdir, "wav_forensics.json")
    with open(out_json, "w", encoding="utf-8") as fh:
        json.dump(doc, fh, indent=1, ensure_ascii=False)
    lines = []
    for tag, res in (("v1 (6d7289990beafcec)", res1), ("v2 (45b1a3cbc41bbef5)", res2)):
        lines.append("== %s ==" % tag)
        lines.append("duration=%ss voiced_time=%ss (in %d segments)" % (
            res["duration_from_data_chunk_s"], res["voiced_time_s"], res["voiced_segment_count"]))
        lines.append("audible segments: %s" % (res["audible_segments"],))
        for d in res["voiced_segments"]:
            lines.append(
                "  voiced %6.2f-%6.2fs (%4.2fs) f0~%shz [%.0f-%.0f] cv=%s harm=%s formant2k5k=%s" % (
                    d["start_s"], d["end_s"], d["len_s"], d["f0_mean_hz"],
                    d["f0_min_hz"], d["f0_max_hz"], d["f0_cv"], d["harm_peak_mean"],
                    d["band_share"]["2k_5k_formant"]))
    text = "\n".join(lines)
    with open(os.path.join(outdir, "wav_forensics_summary.txt"), "w", encoding="utf-8") as fh:
        fh.write(text + "\n")
    print(text)
    print("written: %s" % out_json)


if __name__ == "__main__":
    main()
