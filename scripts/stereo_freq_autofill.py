import argparse
import csv
import json
import math
import wave
from pathlib import Path
from typing import Dict, List, Optional, Tuple

import numpy as np


K_ORDER = 13
K_SIZE = 1 << K_ORDER  # 8192
K_FFT_BINS = K_SIZE // 2  # 4096
K_UI_BINS = 336
K_FRAME_SEC = 0.01
K_MIN_FREQ = 20.0
K_MAX_FREQ = 20000.0
K_GATE_NOISE_PERCENTILE = 35.0
K_GATE_ABS_NOISE_SCALE = 2.0
K_GATE_DYN_DB_MIN = 55.0
K_GATE_DYN_DB_MAX = 72.0
K_GATE_DYN_DB_SNR_FACTOR = 0.8
K_GATE_FLOOR = 1.0e-8


def cents_error(observed_hz: float, expected_hz: float) -> float:
    if observed_hz <= 0.0 or expected_hz <= 0.0:
        return 0.0
    return 1200.0 * math.log2(observed_hz / expected_hz)


def build_fft_to_ui_lut(sample_rate: float) -> np.ndarray:
    lut = np.zeros((K_FFT_BINS,), dtype=np.int32)
    denom = math.log2(K_MAX_FREQ / K_MIN_FREQ)
    for i in range(K_FFT_BINS):
        f = float(i) * float(sample_rate) / float(K_SIZE)
        if f < K_MIN_FREQ:
            lut[i] = 0
            continue
        if f > K_MAX_FREQ:
            lut[i] = K_UI_BINS - 1
            continue
        t = math.log2(f / K_MIN_FREQ) / denom
        b = int(round(t * (K_UI_BINS - 1)))
        lut[i] = max(0, min(K_UI_BINS - 1, b))
    return lut


def build_ui_bin_ranges(lut: np.ndarray) -> List[Tuple[int, int]]:
    first_seen = np.full((K_UI_BINS,), K_FFT_BINS, dtype=np.int32)
    last_seen = np.full((K_UI_BINS,), -1, dtype=np.int32)
    for i in range(K_FFT_BINS):
        b = int(lut[i])
        if i < first_seen[b]:
            first_seen[b] = i
        if i > last_seen[b]:
            last_seen[b] = i
    out: List[Tuple[int, int]] = []
    for b in range(K_UI_BINS):
        s = int(first_seen[b])
        e = int(last_seen[b]) + 1
        if s >= e:
            e = s + 1
        s = max(0, min(K_FFT_BINS - 1, s))
        e = max(s + 1, min(K_FFT_BINS, e))
        out.append((s, e))
    return out


def ui_bin_to_hz(bin_idx: int) -> float:
    t = float(bin_idx) / float(K_UI_BINS - 1)
    return K_MIN_FREQ * math.pow(K_MAX_FREQ / K_MIN_FREQ, t)


def percentile_nonzero(values: np.ndarray, p: float) -> float:
    nz = values[values > 0.0]
    if nz.size == 0:
        return 0.0
    return float(np.percentile(nz, p))


def adaptive_threshold(peak: float, noise: float) -> float:
    if peak <= 0.0:
        return K_GATE_FLOOR
    eps = 1.0e-12
    snr_db = 20.0 * math.log10((peak + eps) / (noise + eps))
    dyn_db = max(K_GATE_DYN_DB_MIN, min(K_GATE_DYN_DB_MAX, K_GATE_DYN_DB_SNR_FACTOR * snr_db))
    thr_rel = peak * math.pow(10.0, -dyn_db / 20.0)
    thr_abs = noise * K_GATE_ABS_NOISE_SCALE
    return max(K_GATE_FLOOR, thr_rel, thr_abs)


def read_wav_stereo(path: Path) -> Tuple[int, np.ndarray]:
    with wave.open(str(path), "rb") as wf:
        ch = wf.getnchannels()
        sw = wf.getsampwidth()
        sr = wf.getframerate()
        n = wf.getnframes()
        data = wf.readframes(n)
    if ch != 2:
        raise ValueError(f"expected stereo wav, got channels={ch}")
    if sw != 2:
        raise ValueError(f"expected 16-bit wav, got sampwidth={sw}")
    pcm = np.frombuffer(data, dtype=np.int16).reshape((-1, 2))
    samples = pcm.astype(np.float32) / 32768.0
    return sr, samples


def frame_peak_hz(
    frame_samples: np.ndarray,
    window: np.ndarray,
    ui_ranges: List[Tuple[int, int]],
) -> float:
    x = frame_samples * window
    spec = np.fft.rfft(x, n=K_SIZE)
    mag = np.abs(spec[:K_FFT_BINS]).astype(np.float32)
    mapped = np.zeros((K_UI_BINS,), dtype=np.float32)
    for b, (s, e) in enumerate(ui_ranges):
        mapped[b] = float(np.max(mag[s:e]))
    col_max = float(np.max(mapped))
    if col_max <= 1e-12:
        return 0.0
    noise = percentile_nonzero(mapped, K_GATE_NOISE_PERCENTILE)
    threshold = adaptive_threshold(col_max, noise)
    low_mask = mapped < threshold
    if np.any(low_mask):
        ratio = np.divide(mapped[low_mask], max(threshold, 1.0e-12))
        mapped[low_mask] = mapped[low_mask] * ratio
    peak_bin = int(np.argmax(mapped))
    if mapped[peak_bin] <= 0.0:
        return 0.0
    return ui_bin_to_hz(peak_bin)


def estimate_segment_observed(
    samples: np.ndarray,
    sample_rate: int,
    seg_start_s: float,
    seg_end_s: float,
    ui_ranges: List[Tuple[int, int]],
    edge_trim_ratio: float,
) -> Tuple[float, float]:
    hop = max(1, int(round(sample_rate * K_FRAME_SEC)))
    win = np.hanning(K_SIZE).astype(np.float32)
    total = samples.shape[0]

    dur = max(0.0, seg_end_s - seg_start_s)
    trim = max(0.0, min(dur * edge_trim_ratio, dur * 0.45))
    start_s = seg_start_s + trim
    end_s = seg_end_s - trim
    if end_s <= start_s:
        start_s = seg_start_s
        end_s = seg_end_s

    start_i = max(0, int(round(start_s * sample_rate)))
    end_i = min(total, int(round(end_s * sample_rate)))
    if end_i <= start_i:
        return 0.0, 0.0

    left_peaks: List[float] = []
    right_peaks: List[float] = []
    i = start_i
    while i < end_i:
        if i + K_SIZE > total:
            break
        l = frame_peak_hz(samples[i:i + K_SIZE, 0], win, ui_ranges)
        r = frame_peak_hz(samples[i:i + K_SIZE, 1], win, ui_ranges)
        if l > 0.0:
            left_peaks.append(l)
        if r > 0.0:
            right_peaks.append(r)
        i += hop

    l_obs = float(np.median(np.array(left_peaks, dtype=np.float32))) if left_peaks else 0.0
    r_obs = float(np.median(np.array(right_peaks, dtype=np.float32))) if right_peaks else 0.0
    return l_obs, r_obs


def load_manifest(path: Path) -> Dict:
    return json.loads(path.read_text(encoding="utf-8"))


def update_csv(
    csv_path: Path,
    segments: List[Dict],
    observed: Dict[int, Tuple[float, float]],
) -> None:
    with csv_path.open("r", newline="", encoding="utf-8") as f:
        rows = list(csv.reader(f))
    if not rows:
        raise ValueError("csv is empty")
    header = rows[0]
    h = {name: idx for idx, name in enumerate(header)}
    required = [
        "segment_id",
        "expected_left_hz",
        "expected_right_hz",
        "observed_left_hz",
        "observed_right_hz",
        "left_error_hz",
        "right_error_hz",
        "left_error_cents",
        "right_error_cents",
    ]
    for col in required:
        if col not in h:
            raise ValueError(f"csv missing column: {col}")

    for i in range(1, len(rows)):
        row = rows[i]
        if not row:
            continue
        seg_id = int(float(row[h["segment_id"]]))
        if seg_id not in observed:
            continue
        l_obs, r_obs = observed[seg_id]
        row[h["observed_left_hz"]] = f"{l_obs:.6f}" if l_obs > 0 else ""
        row[h["observed_right_hz"]] = f"{r_obs:.6f}" if r_obs > 0 else ""

        exp_l = float(row[h["expected_left_hz"]] or 0.0)
        exp_r = float(row[h["expected_right_hz"]] or 0.0)
        if exp_l > 0.0 and l_obs > 0.0:
            row[h["left_error_hz"]] = f"{l_obs - exp_l:.6f}"
            row[h["left_error_cents"]] = f"{cents_error(l_obs, exp_l):.3f}"
        else:
            row[h["left_error_hz"]] = ""
            row[h["left_error_cents"]] = ""
        if exp_r > 0.0 and r_obs > 0.0:
            row[h["right_error_hz"]] = f"{r_obs - exp_r:.6f}"
            row[h["right_error_cents"]] = f"{cents_error(r_obs, exp_r):.3f}"
        else:
            row[h["right_error_hz"]] = ""
            row[h["right_error_cents"]] = ""

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        csv.writer(f).writerows(rows)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Autofill stereo_freq_calibrator observations.csv by STFT simulation."
    )
    parser.add_argument(
        "--manifest",
        required=True,
        help="path to *.manifest.json from stereo_freq_calibrator",
    )
    parser.add_argument(
        "--wav",
        default="",
        help="optional override wav path (default from manifest base name)",
    )
    parser.add_argument(
        "--csv",
        default="",
        help="optional override observations csv path (default from manifest base name)",
    )
    parser.add_argument(
        "--edge-trim-ratio",
        type=float,
        default=0.20,
        help="trim segment edges before estimation (0~0.45)",
    )
    args = parser.parse_args()

    manifest_path = Path(args.manifest)
    man = load_manifest(manifest_path)
    base = manifest_path.with_suffix("").with_suffix("")
    wav_path = Path(args.wav) if args.wav else base.with_suffix(".wav")
    csv_path = Path(args.csv) if args.csv else base.with_suffix(".observations.csv")

    sr, samples = read_wav_stereo(wav_path)
    lut = build_fft_to_ui_lut(float(sr))
    ranges = build_ui_bin_ranges(lut)

    observed: Dict[int, Tuple[float, float]] = {}
    for seg in man.get("segments", []):
        seg_id = int(seg["segment_id"])
        s0 = float(seg["start_seconds"])
        s1 = float(seg["end_seconds"])
        l_obs, r_obs = estimate_segment_observed(
            samples=samples,
            sample_rate=sr,
            seg_start_s=s0,
            seg_end_s=s1,
            ui_ranges=ranges,
            edge_trim_ratio=args.edge_trim_ratio,
        )
        observed[seg_id] = (l_obs, r_obs)

    update_csv(csv_path, man.get("segments", []), observed)
    print(f"[ok] autofilled observations: {csv_path}")
    print(f"[info] segments processed: {len(observed)}")


if __name__ == "__main__":
    main()
