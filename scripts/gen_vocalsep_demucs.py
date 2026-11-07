#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""GEN-VOCALSEP-1: demucs 人声分离驱动（生成产物后处理腿）。

对生成 wav 做 two-stems 分离（vocals + no_vocals residual），产物写工件目录，
并输出简单客观指标（能量占比/重建误差/低频泄漏代理/静默帧泄漏代理）。

用法：
  python scripts/gen_vocalsep_demucs.py INPUT.wav --out OUTDIR [--model htdemucs]
      [--device cpu] [--shifts 1] [--no-src-sr] [--all-stems] [--metrics-out FILE]

依赖：demucs 4.1.0（torch/torchaudio/soundfile/numpy）。权重经 HuggingFace
官方作者仓 adefossez/HTDemucs 加载（可用 HF_ENDPOINT=https://hf-mirror.com 镜像端点），
缓存在标准 HF cache，不入源码树。

指标口径（诚实边界：都是简单代理，不是 SDR）：
  rms_*            各信号整体 RMS（mixture 为回采样到模型采样率的输入）。
  energy_frac_*    vocals / no_vocals 能量占 mixture 能量比例。
  recon_rmse_ratio ||mix-(voc+novoc)|| / ||mix||：两 stem 对 mixture 的重建残差。
  voc_lf_frac      vocals stem 中 <150Hz 能量占比（人声基频多在 150Hz 以上；
                   该值高=伴奏低频泄漏进人声的可能性大；附 no_vocals 同口径对照）。
  quiet20_leak_db  人声 stem 能量最低 20% 帧里，vocals/no_vocals 能量差的 dB——
                   这些帧人声大概率不唱，vocals stem 里剩的即残余伴奏电平估计。
"""
import argparse
import json
import sys
import time
from pathlib import Path

import numpy as np
import soundfile as sf
import torch as th
from julius import resample_frac


def db(x: float) -> float:
    return float("-inf") if x <= 0 else float(20.0 * np.log10(x))


def stft_mag(x: np.ndarray, sr: int, n_fft: int = 4096, hop: int = 1024) -> np.ndarray:
    """单声道幅度谱 [frames, bins]（x 为 1-D 波形）。"""
    window = np.hanning(n_fft).astype(np.float32)
    n = len(x)
    frames = 1 + max(0, (n - n_fft)) // hop
    out = np.empty((frames, n_fft // 2 + 1), dtype=np.float32)
    for i in range(frames):
        seg = x[i * hop: i * hop + n_fft] * window
        out[i] = np.abs(np.fft.rfft(seg))
    return out


def rms(x: np.ndarray) -> float:
    return float(np.sqrt(np.mean(np.square(x)) + 1e-20))


def frame_energy(mag: np.ndarray) -> np.ndarray:
    return np.sqrt(np.mean(np.square(mag), axis=1))


def metrics_for(track: str, mix: np.ndarray, voc: np.ndarray, nov: np.ndarray, sr: int) -> dict:
    """mix/voc/nov 均为同采样率双声道 [ch, n] 的 float32。"""
    mono = lambda w: w.mean(axis=0)
    m, v, n = mono(mix), mono(voc), mono(nov)
    r_mix, r_voc, r_nov = rms(m), rms(v), rms(n)
    e_mix = float(np.sum(m * m)) + 1e-20
    recon = float(np.sqrt(np.mean(np.square(m - (v + n)))))
    # 低频泄漏代理
    freqs = np.fft.rfftfreq(4096, 1.0 / sr)
    lf_bins = freqs < 150.0
    mag_v = stft_mag(v, sr)
    mag_n = stft_mag(n, sr)
    ev = np.square(mag_v).sum(axis=0)
    en = np.square(mag_n).sum(axis=0)
    voc_lf_frac = float(ev[lf_bins].sum() / (ev.sum() + 1e-20))
    nov_lf_frac = float(en[lf_bins].sum() / (en.sum() + 1e-20))
    # 静默帧泄漏代理：vocals 能量最低 20% 帧
    fe_v = frame_energy(mag_v)
    fe_n = frame_energy(mag_n)
    k = max(1, int(len(fe_v) * 0.2))
    quiet = np.argsort(fe_v)[:k]
    q_v = float(np.sqrt(np.mean(np.square(fe_v[quiet]))))
    q_n = float(np.sqrt(np.mean(np.square(fe_n[quiet]))))
    return {
        "track": track,
        "sr": sr,
        "rms_mix": r_mix, "rms_vocals": r_voc, "rms_no_vocals": r_nov,
        "energy_frac_vocals": float(r_voc * r_voc * len(v) / e_mix),
        "energy_frac_no_vocals": float(r_nov * r_nov * len(n) / e_mix),
        "recon_rmse_ratio": float(recon / (r_mix + 1e-20)),
        "voc_lf_frac_lt150hz": voc_lf_frac,
        "no_vocals_lf_frac_lt150hz": nov_lf_frac,
        "quiet20_vocals_rms": q_v,
        "quiet20_no_vocals_rms": q_n,
        "quiet20_leak_db": db(q_v / (q_n + 1e-20)),
    }


def main() -> int:
    ap = argparse.ArgumentParser(description="demucs vocals separation driver (GEN-VOCALSEP-1)")
    ap.add_argument("input", type=Path)
    ap.add_argument("--out", type=Path, required=True)
    ap.add_argument("--model", default="htdemucs")
    ap.add_argument("--device", default="cpu")
    ap.add_argument("--shifts", type=int, default=1)
    ap.add_argument("--no-src-sr", action="store_true",
                    help="不回采样到源采样率（保留模型采样率 44.1k）")
    ap.add_argument("--all-stems", action="store_true", help="额外落盘四源全 stem")
    ap.add_argument("--metrics-out", type=Path, default=None)
    args = ap.parse_args()

    from demucs.api import Separator

    src_info = sf.info(str(args.input))
    src_sr = src_info.samplerate
    t0 = time.time()
    sep = Separator(model=args.model, device=args.device, shifts=args.shifts, progress=False)
    load_s = time.time() - t0
    t1 = time.time()
    orig, stems = sep.separate_audio_file(args.input)
    infer_s = time.time() - t1
    model_sr = sep.samplerate  # type: ignore[attr-defined]

    def to_np(w: th.Tensor) -> np.ndarray:
        return w.detach().cpu().numpy().astype(np.float32)

    mix = to_np(orig)
    voc = to_np(stems["vocals"])
    others = [s for s in stems if s != "vocals"]
    nov = sum(to_np(stems[s]) for s in others)

    out_dir = args.out / args.input.stem
    out_dir.mkdir(parents=True, exist_ok=True)
    write_sr = src_sr if (src_sr != model_sr and not args.no_src_sr) else model_sr
    resample = (write_sr != model_sr)

    def write(name: str, w: np.ndarray) -> Path:
        p = out_dir / f"{name}.wav"
        if resample:
            w = resample_frac(th.from_numpy(w.copy()), model_sr, write_sr).numpy()
        sf.write(str(p), w.T if w.shape[0] <= 8 else w, write_sr)  # [ch,n] -> [n,ch]
        return p

    paths = {"vocals": str(write("vocals", voc)), "no_vocals": str(write("no_vocals", nov))}
    if args.all_stems:
        for s in others:
            paths[s] = str(write(s, to_np(stems[s])))

    m = metrics_for(args.input.stem, mix, voc, nov, model_sr)
    m.update({
        "input": str(args.input), "src_sr": src_sr, "model_sr": model_sr, "write_sr": write_sr,
        "duration_s": float(src_info.duration), "model_load_s": round(load_s, 2),
        "infer_wall_s": round(infer_s, 2),
        "realtime_factor": round(infer_s / max(0.001, float(mix.shape[1]) / model_sr), 3),
        "device": args.device, "shifts": args.shifts, "model": args.model,
        "outputs": paths,
    })
    target = args.metrics_out or (args.out / f"{args.input.stem}.metrics.json")
    Path(target).write_text(json.dumps(m, indent=1, ensure_ascii=False), encoding="utf-8")
    print(json.dumps({k: m[k] for k in
                      ("track", "infer_wall_s", "realtime_factor", "energy_frac_vocals",
                       "recon_rmse_ratio", "voc_lf_frac_lt150hz", "quiet20_leak_db")}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
