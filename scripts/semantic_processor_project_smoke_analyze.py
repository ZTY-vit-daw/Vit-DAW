#!/usr/bin/env python3
"""Rank RipX stem excerpts for the seven-family project Agent smoke.

This is an evaluator-side material analysis tool. It never calls the product
Agent and its issue-family scores must not be copied into Agent-visible fixture
metadata. The scores answer only whether a source excerpt has enough events and
context to support a controlled, level-matched fault injection.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import time
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf
from scipy.signal import butter, find_peaks, sosfilt


SCHEMA_VERSION = "semantic_processor_project_material_analysis.v1"
ANALYZER_VERSION = "20260809.1"
TRACKS = ("bass", "drums", "guitar", "other", "piano", "vocals")
BANDS = {
    "low": (40.0, 160.0),
    "low_mid": (160.0, 500.0),
    "mid": (500.0, 2000.0),
    "presence": (2000.0, 5000.0),
    "high": (5000.0, 10000.0),
}


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def db(value: float, floor: float = -180.0) -> float:
    return floor if value <= 0.0 else 20.0 * math.log10(value)


def finite(value: float, fallback: float = 0.0) -> float:
    return float(value) if math.isfinite(float(value)) else fallback


def percentile(values: np.ndarray, q: float, fallback: float = 0.0) -> float:
    clean = values[np.isfinite(values)]
    return fallback if clean.size == 0 else float(np.percentile(clean, q))


def distribution(values: np.ndarray) -> dict[str, float | int] | None:
    clean = values[np.isfinite(values)]
    if clean.size == 0:
        return None
    return {
        "count": int(clean.size),
        "min": round(float(np.min(clean)), 4),
        "p50": round(float(np.percentile(clean, 50)), 4),
        "p90": round(float(np.percentile(clean, 90)), 4),
        "max": round(float(np.max(clean)), 4),
    }


def block_rms(audio: np.ndarray, frames: int) -> np.ndarray:
    usable = len(audio) // frames * frames
    if usable <= 0:
        return np.empty(0, dtype=np.float64)
    blocks = audio[:usable].reshape(-1, frames)
    return np.sqrt(np.mean(np.square(blocks, dtype=np.float64), axis=1) + 1e-30)


def band_audio(audio: np.ndarray, samplerate: int, low: float, high: float) -> np.ndarray:
    nyquist = samplerate * 0.5
    lo = max(10.0, low) / nyquist
    hi = min(high, nyquist * 0.96) / nyquist
    if not 0.0 < lo < hi < 1.0:
        return np.zeros_like(audio)
    return sosfilt(butter(4, [lo, hi], btype="bandpass", output="sos"), audio)


def normalize_score(value: float, low: float, high: float) -> float:
    if high <= low:
        return 0.0
    return round(float(np.clip((value - low) / (high - low), 0.0, 1.0)), 4)


def window_metrics(audio: np.ndarray, samplerate: int) -> dict[str, Any]:
    mono = np.asarray(audio, dtype=np.float64)
    peak = float(np.max(np.abs(mono))) if len(mono) else 0.0
    total_rms = float(np.sqrt(np.mean(np.square(mono)) + 1e-30)) if len(mono) else 0.0

    rms_100 = block_rms(mono, max(1, int(round(0.100 * samplerate))))
    rms_500 = block_rms(mono, max(1, int(round(0.500 * samplerate))))
    rms_100_db = 20.0 * np.log10(np.maximum(rms_100, 1e-30))
    rms_500_db = 20.0 * np.log10(np.maximum(rms_500, 1e-30))
    active_floor = max(percentile(rms_100_db, 90, -180.0) - 38.0, -72.0)
    active_100 = rms_100_db > active_floor
    low_100 = rms_100_db <= max(active_floor, percentile(rms_100_db, 75, -180.0) - 18.0)

    low_runs: list[float] = []
    current = 0
    for is_low in low_100:
        if bool(is_low):
            current += 1
        elif current:
            low_runs.append(current * 0.1)
            current = 0
    if current:
        low_runs.append(current * 0.1)

    env_frames = max(1, int(round(0.010 * samplerate)))
    env = block_rms(mono, env_frames)
    env_db = 20.0 * np.log10(np.maximum(env, 1e-30))
    prominence = max(3.0, float(np.std(env_db)) * 0.45)
    peaks, properties = find_peaks(env_db, distance=max(1, int(round(0.08 / 0.01))), prominence=prominence)
    peak_prom = np.asarray(properties.get("prominences", []), dtype=np.float64)
    onset_count = int(len(peaks))
    onset_rate = onset_count / max(len(mono) / samplerate, 1e-9)

    attack_body: list[float] = []
    sustain_decay: list[float] = []
    for index in peaks:
        body_start = index + 2
        body_end = min(len(env_db), index + 10)
        sustain_start = index + 10
        sustain_end = min(len(env_db), index + 35)
        if body_end > body_start:
            attack_body.append(float(env_db[index] - np.mean(env_db[body_start:body_end])))
        if sustain_end > sustain_start:
            sustain_decay.append(float(np.mean(env_db[body_start:body_end]) - np.mean(env_db[sustain_start:sustain_end])))

    band_rows: dict[str, dict[str, float]] = {}
    band_block_db: dict[str, np.ndarray] = {}
    for name, (low, high) in BANDS.items():
        filtered = band_audio(mono, samplerate, low, high)
        blocks = block_rms(filtered, max(1, int(round(0.250 * samplerate))))
        blocks_db = 20.0 * np.log10(np.maximum(blocks, 1e-30))
        band_block_db[name] = blocks_db
        band_rows[name] = {
            "rms_dbfs": round(db(float(np.sqrt(np.mean(np.square(filtered)) + 1e-30))), 4),
            "p90_p10_range_db": round(percentile(blocks_db, 90) - percentile(blocks_db, 10), 4),
            "std_db": round(float(np.std(blocks_db)), 4) if len(blocks_db) else 0.0,
        }

    transient_events: list[dict[str, float]] = []
    for index in peaks[:64]:
        body_start = index + 2
        body_end = min(len(env_db), index + 10)
        sustain_start = index + 10
        sustain_end = min(len(env_db), index + 35)
        if body_end <= body_start:
            continue
        body_db = float(np.mean(env_db[body_start:body_end]))
        sustain_db = float(np.mean(env_db[sustain_start:sustain_end])) if sustain_end > sustain_start else body_db
        transient_events.append({
            "onset_seconds": round(float(index * 0.010), 4),
            "body_end_seconds": round(float(body_end * 0.010), 4),
            "sustain_end_seconds": round(float(sustain_end * 0.010), 4),
            "onset_dbfs": round(float(env_db[index]), 4),
            "body_dbfs": round(body_db, 4),
            "sustain_dbfs": round(sustain_db, 4),
            "attack_body_contrast_db": round(float(env_db[index] - body_db), 4),
            "sustain_decay_db": round(float(body_db - sustain_db), 4),
        })

    high = band_audio(mono, samplerate, 4500.0, 11000.0)
    high_env = block_rms(high, max(1, int(round(0.020 * samplerate))))
    full_env = block_rms(mono, max(1, int(round(0.020 * samplerate))))
    usable = min(len(high_env), len(full_env))
    high_ratio_db = 20.0 * np.log10(np.maximum(high_env[:usable], 1e-30) / np.maximum(full_env[:usable], 1e-30))
    active_ratio = high_ratio_db[full_env[:usable] > max(total_rms * 0.08, 1e-8)]
    high_event_threshold = percentile(active_ratio, 75, -60.0) + 4.0
    high_events, _ = find_peaks(high_ratio_db, height=high_event_threshold, distance=max(1, int(round(0.06 / 0.02))))

    frequency_events: list[dict[str, float | str]] = []
    for index in high_events[:64]:
        if index >= usable:
            continue
        frequency_events.append({
            "start_seconds": round(float(index * 0.020), 4),
            "end_seconds": round(float((index + 1) * 0.020), 4),
            "band_id": "presence_high",
            "min_hz": 4500.0,
            "max_hz": 11000.0,
            "level_dbfs": round(float(20.0 * np.log10(max(float(high_env[index]), 1e-30))), 4),
            "contrast_db": round(float(high_ratio_db[index]), 4),
        })

    band_dynamic_rows = []
    for name, values in band_block_db.items():
        crest_values = np.asarray(values - np.median(values), dtype=np.float64)
        band_dynamic_rows.append({
            "id": name,
            "status": "ready" if len(values) else "missing",
            "time_distribution": distribution(values),
            "crest_distribution": distribution(crest_values),
            "evidence_refs": [f"offline_material_analysis:band:{name}"],
        })

    band_ranges = np.asarray([row["p90_p10_range_db"] for row in band_rows.values()], dtype=np.float64)
    band_stds = np.asarray([row["std_db"] for row in band_rows.values()], dtype=np.float64)
    dominant_band_index = int(np.argmax(band_ranges)) if len(band_ranges) else 0
    dominant_band = list(BANDS)[dominant_band_index]
    other_ranges = np.delete(band_ranges, dominant_band_index) if len(band_ranges) > 1 else np.asarray([0.0])

    active_500_db = rms_500_db[rms_500_db > max(percentile(rms_500_db, 90, -180.0) - 38.0, -72.0)]
    segment_frames = max(1, int(round(5.0 * samplerate)))
    dom_segments = []
    for start_frame in range(0, len(mono), segment_frames):
        segment = mono[start_frame : start_frame + segment_frames]
        if len(segment) < segment_frames // 2:
            continue
        segment_peak = float(np.max(np.abs(segment)))
        segment_rms = float(np.sqrt(np.mean(np.square(segment)) + 1e-30))
        segment_rms_db = db(segment_rms)
        if segment_rms_db > active_floor + 12.0:
            energy_state = "high"
        elif segment_rms_db > active_floor:
            energy_state = "medium"
        elif segment_rms_db > active_floor - 12.0:
            energy_state = "low"
        else:
            energy_state = "silent"
        dom_segments.append({
            "start_seconds": round(start_frame / samplerate, 4),
            "end_seconds": round(min(len(mono), start_frame + segment_frames) / samplerate, 4),
            "rms_dbfs": round(segment_rms_db, 4),
            "peak_dbfs": round(db(segment_peak), 4),
            "crest_db": round(db(segment_peak / max(segment_rms, 1e-30)), 4),
            "energy_state": energy_state,
        })
    dom_bands = []
    for name, (low_hz, high_hz) in BANDS.items():
        dom_bands.append({
            "id": name,
            "status": "ready",
            "min_hz": low_hz,
            "max_hz": high_hz,
            "energy_db": band_rows[name]["rms_dbfs"],
        })
    return {
        "rms_dbfs": round(db(total_rms), 4),
        "peak_dbfs": round(db(peak), 4),
        "headroom_db": round(-db(peak), 4),
        "crest_db": round(db(peak / max(total_rms, 1e-30)), 4),
        "active_ratio_100ms": round(float(np.mean(active_100)), 4) if len(active_100) else 0.0,
        "low_or_silent_ratio_100ms": round(float(np.mean(low_100)), 4) if len(low_100) else 0.0,
        "longest_low_run_seconds": round(max(low_runs, default=0.0), 4),
        "macro_range_db": round(percentile(active_500_db, 95) - percentile(active_500_db, 10), 4),
        "macro_std_db": round(float(np.std(active_500_db)), 4) if len(active_500_db) else 0.0,
        "onset_count": onset_count,
        "onset_rate_hz": round(onset_rate, 4),
        "onset_prominence_p50_db": round(percentile(peak_prom, 50), 4),
        "attack_body_p50_db": round(percentile(np.asarray(attack_body), 50), 4),
        "sustain_decay_p50_db": round(percentile(np.asarray(sustain_decay), 50), 4),
        "high_frequency_event_count": int(len(high_events)),
        "high_frequency_event_rate_hz": round(len(high_events) / max(len(mono) / samplerate, 1e-9), 4),
        "high_ratio_p95_minus_p50_db": round(percentile(active_ratio, 95) - percentile(active_ratio, 50), 4),
        "bands": band_rows,
        "dominant_dynamic_band": dominant_band,
        "dominant_band_range_advantage_db": round(float(band_ranges[dominant_band_index] - np.median(other_ranges)), 4),
        "band_range_spread_db": round(float(np.max(band_ranges) - np.min(band_ranges)), 4),
        "band_std_mean_db": round(float(np.mean(band_stds)), 4),
        "dom_source_evidence": {
            "schema_version": "dad.dynamic_source_evidence.v1",
            "status": "ready",
            "freshness": "fresh",
            "duration_seconds": round(len(mono) / samplerate, 4),
            "rms_dbfs": round(db(total_rms), 4),
            "peak_dbfs": round(db(peak), 4),
            "headroom_db": round(-db(peak), 4),
            "crest_db": round(db(peak / max(total_rms, 1e-30)), 4),
            "time_segments": dom_segments,
            "bands": dom_bands,
            "noise_floor": {
                "status": "ready" if len(rms_100_db) else "missing",
                "estimate_dbfs": round(percentile(rms_100_db, 10, -180.0), 4),
                "p10_dbfs": round(percentile(rms_100_db, 10, -180.0), 4),
                "p50_dbfs": round(percentile(rms_100_db, 50, -180.0), 4),
                "method": "bounded_rms_percentile_100ms",
                "confidence": "medium" if len(rms_100_db) >= 10 else "low",
                "window_count": int(len(rms_100_db)),
                "evidence_refs": ["offline_material_analysis:noise_floor"],
            },
            "frequency_events": {
                "status": "ready" if len(high_ratio_db) else "missing",
                "events": frequency_events,
                "event_count_available": bool(len(high_ratio_db)),
                "coverage": round(float(min(1.0, len(high_ratio_db) / max(len(mono) / max(1, int(round(0.020 * samplerate))), 1))), 4),
                "evidence_refs": ["offline_material_analysis:frequency_events"],
            },
            "transient_events": {
                "status": "ready" if transient_events else "partial",
                "events": transient_events,
                "coverage": round(float(len(env_db) / max(len(mono) / max(1, int(round(0.010 * samplerate))), 1)), 4) if len(mono) else 0.0,
                "evidence_refs": ["offline_material_analysis:transient_events"],
            },
            "band_dynamics_evidence": band_dynamic_rows,
            "quality": {"status": "ready", "nonzero": peak > 0.0, "coverage": 1.0, "nan_inf_count": 0},
            "evidence_refs": ["offline_material_analysis:window"],
            "source_features": ["waveform_envelope", "time_energy_segments", "whole_window_band_energy"],
        },
    }


def track_scores(metrics: dict[str, Any], track: str) -> dict[str, float]:
    active = float(metrics["active_ratio_100ms"])
    scores = {
        "static_eq": normalize_score(active, 0.35, 0.9) * normalize_score(float(metrics["rms_dbfs"]), -50.0, -24.0),
        "broadband_compressor": normalize_score(float(metrics["macro_range_db"]), 5.0, 16.0) * normalize_score(active, 0.35, 0.9),
        "limiter": normalize_score(float(metrics["crest_db"]), 10.0, 22.0) * normalize_score(float(metrics["onset_prominence_p50_db"]), 3.0, 13.0),
        "gate_expander": normalize_score(float(metrics["low_or_silent_ratio_100ms"]), 0.12, 0.58) * normalize_score(float(metrics["longest_low_run_seconds"]), 0.25, 3.0),
        "de_esser": normalize_score(float(metrics["high_frequency_event_rate_hz"]), 0.25, 2.2) * normalize_score(float(metrics["high_ratio_p95_minus_p50_db"]), 3.0, 12.0),
        "transient_shaper": normalize_score(float(metrics["onset_rate_hz"]), 0.5, 4.0) * normalize_score(float(metrics["attack_body_p50_db"]), 2.0, 12.0),
        "multiband_dynamics": normalize_score(float(metrics["band_std_mean_db"]), 2.0, 10.0) * normalize_score(active, 0.45, 0.95),
    }
    if track != "vocals":
        scores["de_esser"] *= 0.15
    if track not in {"drums", "guitar", "piano", "other"}:
        scores["transient_shaper"] *= 0.45
    if track not in {"drums", "other"}:
        scores["limiter"] *= 0.55
    return {key: round(float(value), 4) for key, value in scores.items()}


def source_path(project: Path, track: str) -> Path:
    matches = sorted(project.glob(f"*_{track}.wav"))
    if len(matches) != 1:
        raise ValueError(f"expected one {track} stem below {project}, found {len(matches)}")
    return matches[0]


def analyze_project(project: Path, duration: float, hop: float, analysis_rate: int) -> dict[str, Any]:
    paths = {track: source_path(project, track) for track in TRACKS}
    infos = {track: sf.info(path) for track, path in paths.items()}
    rates = {info.samplerate for info in infos.values()}
    channels = {info.channels for info in infos.values()}
    frames = {info.frames for info in infos.values()}
    if len(rates) != 1 or len(channels) != 1 or len(frames) != 1:
        raise ValueError(f"stem format mismatch in {project}")
    source_rate = rates.pop()
    total_seconds = next(iter(frames)) / source_rate
    starts = np.arange(0.0, max(0.0, total_seconds - duration) + 1e-6, hop)
    windows: list[dict[str, Any]] = []
    for start in starts:
        per_track: dict[str, Any] = {}
        mono_tracks: list[np.ndarray] = []
        for track in TRACKS:
            with sf.SoundFile(paths[track]) as handle:
                handle.seek(int(round(start * source_rate)))
                audio = handle.read(int(round(duration * source_rate)), dtype="float32", always_2d=True)
            mono = np.mean(audio, axis=1, dtype=np.float64)
            if source_rate != analysis_rate:
                step = max(1, int(round(source_rate / analysis_rate)))
                mono = mono[::step]
                actual_rate = int(round(source_rate / step))
            else:
                actual_rate = source_rate
            mono_tracks.append(mono)
            metrics = window_metrics(mono, actual_rate)
            per_track[track] = {"metrics": metrics, "scores": track_scores(metrics, track)}
        usable = min(map(len, mono_tracks))
        mix = np.sum(np.stack([row[:usable] for row in mono_tracks]), axis=0)
        mix_peak = float(np.max(np.abs(mix))) if len(mix) else 1.0
        mix /= max(mix_peak, 1.0)
        mix_metrics = window_metrics(mix, actual_rate)
        mix_scores = track_scores(mix_metrics, "mix")
        best: dict[str, dict[str, Any]] = {}
        for family in mix_scores:
            candidates = [(track, float(row["scores"][family])) for track, row in per_track.items()]
            candidates.append(("mix", float(mix_scores[family])))
            target, score = max(candidates, key=lambda row: row[1])
            best[family] = {"target": target, "score": round(score, 4)}
        windows.append({
            "start_seconds": round(float(start), 3),
            "end_seconds": round(float(start + duration), 3),
            "tracks": per_track,
            "mix": {"metrics": mix_metrics, "scores": mix_scores},
            "best_by_family": best,
        })
    return {
        "project": project.name,
        "source_directory": str(project),
        "samplerate": source_rate,
        "channels": next(iter(channels)),
        "duration_seconds": round(total_seconds, 4),
        "source_files": [{"track": track, "path": str(paths[track]), "sha256": sha256_file(paths[track])} for track in TRACKS],
        "windows": windows,
    }


def top_candidates(projects: list[dict[str, Any]], limit: int) -> dict[str, list[dict[str, Any]]]:
    families = ("static_eq", "broadband_compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics")
    result: dict[str, list[dict[str, Any]]] = {}
    for family in families:
        rows = []
        for project in projects:
            for window in project["windows"]:
                best = window["best_by_family"][family]
                rows.append({
                    "project": project["project"],
                    "start_seconds": window["start_seconds"],
                    "end_seconds": window["end_seconds"],
                    "target": best["target"],
                    "score": best["score"],
                })
        rows.sort(key=lambda row: (-float(row["score"]), str(row["project"]), float(row["start_seconds"])))
        selected = []
        used_projects: set[str] = set()
        for row in rows:
            if row["project"] in used_projects and len(selected) < min(3, limit):
                continue
            selected.append(row)
            used_projects.add(str(row["project"]))
            if len(selected) >= limit:
                break
        result[family] = selected
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-root", default=r"C:\Users\timoz\Documents\RipX\Stems")
    parser.add_argument("--output", default=r"D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\candidate_analysis.json")
    parser.add_argument("--window-seconds", type=float, default=20.0)
    parser.add_argument("--hop-seconds", type=float, default=10.0)
    parser.add_argument("--analysis-rate", type=int, default=11025)
    parser.add_argument("--top", type=int, default=8)
    args = parser.parse_args()
    try:
        root = Path(args.source_root).resolve()
        projects = []
        for directory in sorted((path for path in root.iterdir() if path.is_dir()), key=lambda path: path.name):
            print(json.dumps({"status": "analyzing", "project": directory.name}, ensure_ascii=False), flush=True)
            projects.append(analyze_project(directory, args.window_seconds, args.hop_seconds, args.analysis_rate))
        report = {
            "schema_version": SCHEMA_VERSION,
            "analyzer_version": ANALYZER_VERSION,
            "created_at": now_iso(),
            "warning": "Evaluator-side material analysis. Never disclose family scores or target recommendations to the product Agent.",
            "configuration": {
                "source_root": str(root),
                "window_seconds": args.window_seconds,
                "hop_seconds": args.hop_seconds,
                "analysis_rate": args.analysis_rate,
            },
            "project_count": len(projects),
            "projects": projects,
            "top_candidates": top_candidates(projects, args.top),
        }
        output = Path(args.output).resolve()
        write_json(output, report)
        print(json.dumps({"status": "passed", "output": str(output), "project_count": len(projects)}, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
