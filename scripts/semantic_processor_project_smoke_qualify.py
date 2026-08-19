#!/usr/bin/env python3
"""Qualify the frozen seven-family smoke recipes entirely in memory.

This evaluator-side tool reads exact local source excerpts and the frozen
contract. It writes no audio, calls no Agent, and mutates no DAW project.
"""

from __future__ import annotations

import argparse
import json
import math
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf
from scipy.signal import fftconvolve, find_peaks, lfilter

from semantic_processor_project_smoke_analyze import band_audio, block_rms


def require(condition: bool, message: str) -> None:
    if not condition:
        raise AssertionError(message)


def db(value: float, floor: float = -180.0) -> float:
    return floor if value <= 0.0 else 20.0 * math.log10(value)


def rms(audio: np.ndarray) -> float:
    return float(np.sqrt(np.mean(np.square(audio, dtype=np.float64)) + 1e-30))


def peak_dbfs(audio: np.ndarray) -> float:
    return db(float(np.max(np.abs(audio))))


def rounded(value: dict[str, Any]) -> dict[str, Any]:
    return {key: round(child, 4) if isinstance(child, float) else child for key, child in value.items()}


def rms_match(problem: np.ndarray, clean: np.ndarray) -> np.ndarray:
    return problem * (rms(clean) / max(rms(problem), 1e-30))


def smooth(values: np.ndarray, half_frames: int) -> np.ndarray:
    half_frames = max(1, int(half_frames))
    window = np.hanning(half_frames * 2 + 1)
    window /= np.sum(window)
    return fftconvolve(values, window, mode="same")


def band_stereo(audio: np.ndarray, sample_rate: int, low_hz: float, high_hz: float) -> np.ndarray:
    return np.stack(
        [band_audio(audio[:, channel], sample_rate, low_hz, high_hz) for channel in range(audio.shape[1])],
        axis=1,
    )


def biquad_bell(audio: np.ndarray, sample_rate: int, frequency_hz: float, gain_db: float, q: float) -> np.ndarray:
    omega = 2.0 * math.pi * frequency_hz / sample_rate
    cos_w = math.cos(omega)
    sin_w = math.sin(omega)
    amplitude = 10.0 ** (gain_db / 40.0)
    alpha = sin_w / (2.0 * q)
    numerator = np.asarray([1.0 + alpha * amplitude, -2.0 * cos_w, 1.0 - alpha * amplitude])
    denominator = np.asarray([1.0 + alpha / amplitude, -2.0 * cos_w, 1.0 - alpha / amplitude])
    numerator /= denominator[0]
    denominator /= denominator[0]
    return np.stack(
        [lfilter(numerator, denominator, audio[:, channel]) for channel in range(audio.shape[1])],
        axis=1,
    )


def block_db(audio: np.ndarray, sample_rate: int, seconds: float) -> np.ndarray:
    mono = np.mean(audio, axis=1, dtype=np.float64)
    values = block_rms(mono, max(1, int(round(seconds * sample_rate))))
    return 20.0 * np.log10(np.maximum(values, 1e-30))


def active_range_db(audio: np.ndarray, sample_rate: int, seconds: float) -> float:
    values = block_db(audio, sample_rate, seconds)
    require(len(values) > 0, "no blocks for active-range measurement")
    floor = max(float(np.percentile(values, 90)) - 38.0, -72.0)
    active = values[values > floor]
    require(len(active) >= 2, "insufficient active blocks")
    return float(np.percentile(active, 95) - np.percentile(active, 10))


def band_range_db(audio: np.ndarray, sample_rate: int, low_hz: float, high_hz: float) -> float:
    values = block_db(band_stereo(audio, sample_rate, low_hz, high_hz), sample_rate, 0.25)
    require(len(values) >= 2, "insufficient band blocks")
    return float(np.percentile(values, 90) - np.percentile(values, 10))


def load_excerpt(source_root: Path, project: dict[str, Any], track: str) -> tuple[np.ndarray, int]:
    name = str(project["source_project"])
    path = source_root / name / f"{name}_{track}.wav"
    with sf.SoundFile(path) as handle:
        sample_rate = int(handle.samplerate)
        handle.seek(int(round(float(project["start_seconds"]) * sample_rate)))
        frames = int(round((float(project["end_seconds"]) - float(project["start_seconds"])) * sample_rate))
        audio = handle.read(frames, dtype="float64", always_2d=True)
    require(len(audio) == frames and audio.shape[1] == 2, f"invalid excerpt {path}")
    return audio, sample_rate


def apply_broad_bell(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    problem = rms_match(
        biquad_bell(clean, sample_rate, float(recipe["frequency_hz"]), float(recipe["gain_db"]), float(recipe["q"])),
        clean,
    )
    target = db(rms(band_stereo(problem, sample_rate, 500.0, 1200.0))) - db(rms(band_stereo(clean, sample_rate, 500.0, 1200.0)))
    low = db(rms(band_stereo(problem, sample_rate, 120.0, 350.0))) - db(rms(band_stereo(clean, sample_rate, 120.0, 350.0)))
    high = db(rms(band_stereo(problem, sample_rate, 2200.0, 4500.0))) - db(rms(band_stereo(clean, sample_rate, 2200.0, 4500.0)))
    return problem, {"target_vs_flank_band_gain_db": target - (low + high) * 0.5}


def apply_macro_steps(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    block = max(1, int(round(float(recipe["period_seconds"]) * sample_rate)))
    count = int(math.ceil(len(clean) / block))
    rng = np.random.default_rng(int(recipe["seed"]))
    signs = np.where(np.arange(count) % 2 == 0, 1.0, -1.0)
    magnitudes = rng.uniform(0.65, 1.0, count) * float(recipe["depth_db"])
    gain_db = np.repeat(signs * magnitudes, block)[: len(clean)]
    gain_db = smooth(gain_db, int(round(float(recipe["smoothing_ms"]) / 1000.0 * sample_rate)))
    problem = rms_match(clean * np.power(10.0, gain_db / 20.0)[:, None], clean)
    return problem, {"active_macro_range_500ms_db_delta": active_range_db(problem, sample_rate, 0.5) - active_range_db(clean, sample_rate, 0.5)}


def high_frequency_events(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, np.ndarray]:
    mono = np.mean(clean, axis=1, dtype=np.float64)
    high = band_audio(mono, sample_rate, float(recipe["band_low_hz"]), float(recipe["band_high_hz"]))
    frames = max(1, int(round(0.01 * sample_rate)))
    ratio = 20.0 * np.log10(np.maximum(block_rms(high, frames), 1e-30) / np.maximum(block_rms(mono, frames), 1e-30))
    peaks, _ = find_peaks(ratio, distance=8, prominence=2.0)
    limit = min(int(recipe["event_limit"]), len(peaks))
    require(limit > 0, "no high-frequency events found")
    selected = peaks[np.argsort(ratio[peaks])[-limit:]]
    mask = np.zeros(len(clean), dtype=np.float64)
    duration = max(3, int(round(float(recipe["event_width_ms"]) / 1000.0 * sample_rate)))
    for peak in selected:
        center = int(round((float(peak) + 0.5) * 0.01 * sample_rate))
        start = max(0, center - duration // 2)
        end = min(len(clean), center + duration // 2)
        mask[start:end] = np.maximum(mask[start:end], np.hanning(max(3, end - start)))
    return selected, mask


def event_high_ratio(audio: np.ndarray, sample_rate: int, events: np.ndarray, recipe: dict[str, Any]) -> float:
    mono = np.mean(audio, axis=1, dtype=np.float64)
    high = np.mean(band_stereo(audio, sample_rate, float(recipe["band_low_hz"]), float(recipe["band_high_hz"])), axis=1)
    values: list[float] = []
    for event in events:
        center = int(round((float(event) + 0.5) * 0.01 * sample_rate))
        start = max(0, center - int(round(0.05 * sample_rate)))
        end = min(len(audio), center + int(round(0.05 * sample_rate)))
        if end > start:
            values.append(db(rms(high[start:end])) - db(rms(mono[start:end])))
    require(values, "no aligned high-frequency event measurements")
    return float(np.median(values))


def apply_high_events(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    events, mask = high_frequency_events(clean, sample_rate, recipe)
    high = band_stereo(clean, sample_rate, float(recipe["band_low_hz"]), float(recipe["band_high_hz"]))
    scale = 10.0 ** (float(recipe["event_gain_db"]) / 20.0) - 1.0
    problem = rms_match(clean + scale * high * mask[:, None], clean)
    return problem, {
        "event_count": int(len(events)),
        "event_mask_coverage": float(np.mean(mask > 0.01)),
        "aligned_event_high_ratio_db_delta": event_high_ratio(problem, sample_rate, events, recipe) - event_high_ratio(clean, sample_rate, events, recipe),
    }


def clean_onsets(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> np.ndarray:
    mono = np.mean(clean, axis=1, dtype=np.float64)
    frame_seconds = 0.005
    envelope = block_rms(mono, max(1, int(round(frame_seconds * sample_rate))))
    envelope_db = 20.0 * np.log10(np.maximum(envelope, 1e-30))
    distance = max(1, int(round(float(recipe["minimum_event_spacing_ms"]) / 1000.0 / frame_seconds)))
    peaks, _ = find_peaks(envelope_db, distance=distance, prominence=max(3.0, float(np.std(envelope_db)) * 0.45))
    limit = min(int(recipe["event_limit"]), len(peaks))
    require(limit > 0, "no transient events found")
    return peaks[np.argsort(envelope_db[peaks])[-limit:]]


def aligned_attack_body(audio: np.ndarray, sample_rate: int, events: np.ndarray) -> tuple[float, float]:
    mono = np.mean(audio, axis=1, dtype=np.float64)
    contrasts: list[float] = []
    bodies: list[float] = []
    for event in events:
        center = int(round(float(event) * 0.005 * sample_rate))
        attack_end = min(len(mono), center + int(round(0.018 * sample_rate)))
        body_start = center + int(round(0.030 * sample_rate))
        body_end = min(len(mono), center + int(round(0.085 * sample_rate)))
        if attack_end <= center or body_end <= body_start:
            continue
        attack = db(rms(mono[center:attack_end]))
        body = db(rms(mono[body_start:body_end]))
        contrasts.append(attack - body)
        bodies.append(body)
    require(contrasts and bodies, "no aligned transient measurements")
    return float(np.median(contrasts)), float(np.median(bodies))


def apply_attack_attenuation(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    events = clean_onsets(clean, sample_rate, recipe)
    gain = np.ones(len(clean), dtype=np.float64)
    low_gain = 10.0 ** (float(recipe["attack_gain_db"]) / 20.0)
    duration = max(1, int(round(float(recipe["attack_ms"]) / 1000.0 * sample_rate)))
    for event in events:
        center = int(round(float(event) * 0.005 * sample_rate))
        start = max(0, center - int(round(0.003 * sample_rate)))
        end = min(len(clean), center + duration)
        shape = low_gain + (1.0 - low_gain) * np.linspace(0.0, 1.0, end - start)
        gain[start:end] = np.minimum(gain[start:end], shape)
    problem = rms_match(clean * gain[:, None], clean)
    before_contrast, before_body = aligned_attack_body(clean, sample_rate, events)
    after_contrast, after_body = aligned_attack_body(problem, sample_rate, events)
    return problem, {
        "event_count": int(len(events)),
        "aligned_attack_body_db_delta": after_contrast - before_contrast,
        "body_level_db_delta": after_body - before_body,
    }


def apply_peak_overshoot(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    envelope = smooth(np.max(np.abs(clean), axis=1), int(round(float(recipe["attack_ms"]) / 1000.0 * sample_rate)))
    peaks, _ = find_peaks(envelope, distance=max(1, int(round(0.25 * sample_rate))), prominence=max(1e-5, float(np.std(envelope)) * 0.7))
    limit = min(int(recipe["event_limit"]), len(peaks))
    require(limit > 0, "no peak events found")
    selected = peaks[np.argsort(envelope[peaks])[-limit:]]
    gain_db = np.zeros(len(clean), dtype=np.float64)
    attack = max(1, int(round(float(recipe["attack_ms"]) / 1000.0 * sample_rate)))
    release = max(1, int(round(float(recipe["release_ms"]) / 1000.0 * sample_rate)))
    for peak in selected:
        start = max(0, int(peak) - attack)
        end = min(len(clean), int(peak) + release)
        left = np.linspace(0.0, float(recipe["peak_gain_db"]), int(peak) - start + 1)
        right = float(recipe["peak_gain_db"]) * np.exp(-np.linspace(0.0, 5.0, end - int(peak) - 1))
        shape = np.concatenate((left, right))
        gain_db[start:end] = np.maximum(gain_db[start:end], shape[: end - start])
    problem = rms_match(clean * np.power(10.0, gain_db / 20.0)[:, None], clean)
    before_crest = peak_dbfs(clean) - db(rms(clean))
    after_crest = peak_dbfs(problem) - db(rms(problem))
    return problem, {
        "event_count": int(len(selected)),
        "affected_fraction": float(np.mean(gain_db > 0.01)),
        "sample_peak_dbfs_delta": peak_dbfs(problem) - peak_dbfs(clean),
        "crest_db_delta": after_crest - before_crest,
        "active_macro_range_500ms_db_delta": active_range_db(problem, sample_rate, 0.5) - active_range_db(clean, sample_rate, 0.5),
    }


def pink_noise(frames: int, channels: int, seed: int) -> np.ndarray:
    white = np.random.default_rng(seed).normal(size=(frames, channels))
    return np.stack(
        [lfilter([0.02], [1.0, -0.98], white[:, channel]) for channel in range(channels)],
        axis=1,
    )


def apply_noise_bed(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    block_frames = max(1, int(round(0.1 * sample_rate)))
    clean_blocks = block_rms(np.mean(clean, axis=1), block_frames)
    clean_db = 20.0 * np.log10(np.maximum(clean_blocks, 1e-30))
    threshold = float(np.percentile(clean_db, 75) - float(recipe["active_guard_db"]))
    quiet = clean_db <= threshold
    require(np.any(quiet) and np.any(~quiet), "Gate fixture needs quiet and active blocks")
    noise = pink_noise(len(clean), clean.shape[1], int(recipe["seed"]))
    noise *= 10.0 ** (float(recipe["noise_rms_dbfs"]) / 20.0) / max(rms(noise), 1e-30)
    mask = np.repeat(quiet.astype(np.float64), block_frames)[: len(clean)]
    mask = smooth(mask, int(round(float(recipe["fade_ms"]) / 1000.0 * sample_rate)))
    problem = rms_match(clean + noise * mask[:, None], clean)
    problem_db = 20.0 * np.log10(np.maximum(block_rms(np.mean(problem, axis=1), block_frames), 1e-30))
    return problem, {
        "quiet_block_fraction": float(np.mean(quiet)),
        "quiet_interval_median_dbfs_delta": float(np.median(problem_db[quiet] - clean_db[quiet])),
        "active_interval_median_dbfs_delta": float(np.median(problem_db[~quiet] - clean_db[~quiet])),
    }


def apply_band_steps(clean: np.ndarray, sample_rate: int, recipe: dict[str, Any], gate: dict[str, Any]) -> tuple[np.ndarray, dict[str, Any]]:
    low_hz = float(recipe["band_low_hz"])
    high_hz = float(recipe["band_high_hz"])
    component = band_stereo(clean, sample_rate, low_hz, high_hz)
    remainder = clean - component
    block = max(1, int(round(float(recipe["period_seconds"]) * sample_rate)))
    count = int(math.ceil(len(clean) / block))
    rng = np.random.default_rng(int(recipe["seed"]))
    signs = np.where(np.arange(count) % 2 == 0, 1.0, -1.0)
    magnitudes = rng.uniform(0.7, 1.0, count) * float(recipe["depth_db"])
    gain_db = np.repeat(signs * magnitudes, block)[: len(clean)]
    gain_db = smooth(gain_db, int(round(0.08 * sample_rate)))
    problem = rms_match(remainder + component * np.power(10.0, gain_db / 20.0)[:, None], clean)
    reference_low = float(gate["reference_band_low_hz"])
    reference_high = float(gate["reference_band_high_hz"])
    return problem, {
        "target_band_p90_p10_range_db_delta": band_range_db(problem, sample_rate, low_hz, high_hz) - band_range_db(clean, sample_rate, low_hz, high_hz),
        "reference_band_p90_p10_range_db_delta": band_range_db(problem, sample_rate, reference_low, reference_high) - band_range_db(clean, sample_rate, reference_low, reference_high),
    }


APPLIERS = {
    "broad_bell_gain": lambda clean, rate, recipe, gate: apply_broad_bell(clean, rate, recipe),
    "macro_level_steps": lambda clean, rate, recipe, gate: apply_macro_steps(clean, rate, recipe),
    "event_localized_high_band_gain": lambda clean, rate, recipe, gate: apply_high_events(clean, rate, recipe),
    "attack_attenuation": lambda clean, rate, recipe, gate: apply_attack_attenuation(clean, rate, recipe),
    "sparse_peak_overshoot": lambda clean, rate, recipe, gate: apply_peak_overshoot(clean, rate, recipe),
    "low_interval_noise_bed": lambda clean, rate, recipe, gate: apply_noise_bed(clean, rate, recipe),
    "band_limited_level_steps": apply_band_steps,
}


def validate_gate(kind: str, metrics: dict[str, Any], gate: dict[str, Any]) -> None:
    if kind == "broad_bell_gain":
        require(metrics["target_vs_flank_band_gain_db"] >= float(gate["minimum_delta_db"]), "EQ target-band delta is too small")
        require(abs(metrics["active_macro_range_500ms_db_delta"]) <= float(gate["maximum_macro_range_delta_db"]), "EQ changed macro dynamics too much")
    elif kind == "macro_level_steps":
        require(metrics["active_macro_range_500ms_db_delta"] >= float(gate["minimum_delta_db"]), "compressor fixture macro delta is too small")
    elif kind == "event_localized_high_band_gain":
        require(metrics["aligned_event_high_ratio_db_delta"] >= float(gate["minimum_delta_db"]), "De-esser event contrast is too small")
        require(metrics["event_mask_coverage"] <= float(gate["maximum_event_mask_coverage"]), "De-esser event mask is not localized")
    elif kind == "attack_attenuation":
        require(metrics["aligned_attack_body_db_delta"] <= float(gate["maximum_delta_db"]), "Transient attack/body delta is too small")
        require(abs(metrics["body_level_db_delta"]) <= float(gate["maximum_absolute_body_level_delta_db"]), "Transient fixture changed body level too much")
    elif kind == "sparse_peak_overshoot":
        require(metrics["sample_peak_dbfs_delta"] >= float(gate["minimum_delta_db"]), "Limiter peak delta is too small")
        require(metrics["crest_db_delta"] >= float(gate["minimum_crest_delta_db"]), "Limiter crest delta is too small")
        require(abs(metrics["active_macro_range_500ms_db_delta"]) <= float(gate["maximum_macro_range_delta_db"]), "Limiter fixture changed macro range too much")
        require(metrics["affected_fraction"] <= float(gate["maximum_affected_fraction"]), "Limiter fixture is not sparse")
    elif kind == "low_interval_noise_bed":
        require(metrics["quiet_interval_median_dbfs_delta"] >= float(gate["minimum_delta_db"]), "Gate quiet-floor delta is too small")
        require(abs(metrics["active_interval_median_dbfs_delta"]) <= float(gate["maximum_absolute_active_interval_delta_db"]), "Gate fixture changed active blocks")
    elif kind == "band_limited_level_steps":
        require(metrics["target_band_p90_p10_range_db_delta"] >= float(gate["minimum_delta_db"]), "Multiband target-band delta is too small")
        require(abs(metrics["reference_band_p90_p10_range_db_delta"]) <= float(gate["maximum_absolute_reference_band_delta_db"]), "Multiband fixture changed reference band")
    else:
        raise AssertionError(f"unsupported recipe kind {kind}")


def qualify_project(source_root: Path, project: dict[str, Any], peak_ceiling_dbfs: float) -> dict[str, Any]:
    clean_tracks: dict[str, np.ndarray] = {}
    problem_tracks: dict[str, np.ndarray] = {}
    sample_rate = 0
    for track in ("bass", "drums", "guitar", "other", "piano", "vocals"):
        audio, rate = load_excerpt(source_root, project, track)
        if sample_rate == 0:
            sample_rate = rate
        require(rate == sample_rate, "project sample-rate mismatch")
        clean_tracks[track] = audio
        problem_tracks[track] = audio

    issue_results: list[dict[str, Any]] = []
    mutated_targets: set[str] = set()
    for issue in project["sealed_issue_assignments"]:
        target = str(issue["target_track"])
        require(target not in mutated_targets, f"target {target} is assigned more than once")
        mutated_targets.add(target)
        recipe = issue["fault_recipe"]
        gate = issue["qualification_gate"]
        kind = str(recipe["kind"])
        applier = APPLIERS.get(kind)
        require(applier is not None, f"no qualifier for {kind}")
        problem, metrics = applier(clean_tracks[target], sample_rate, recipe, gate)
        metrics["rms_match_delta_db"] = db(rms(problem) / rms(clean_tracks[target]))
        metrics["active_macro_range_500ms_db_delta"] = metrics.get(
            "active_macro_range_500ms_db_delta",
            active_range_db(problem, sample_rate, 0.5) - active_range_db(clean_tracks[target], sample_rate, 0.5),
        )
        tolerance = float(recipe.get("rms_match_tolerance_db", 0.1))
        require(abs(metrics["rms_match_delta_db"]) <= tolerance, f"{issue['issue_id']} failed RMS matching")
        validate_gate(kind, metrics, gate)
        problem_tracks[target] = problem
        issue_results.append({
            "issue_id": issue["issue_id"],
            "expected_family": issue["expected_family"],
            "target_track": target,
            "recipe_kind": kind,
            "status": "qualified",
            "measurements": rounded(metrics),
        })

    for track, clean in clean_tracks.items():
        if track not in mutated_targets:
            require(np.array_equal(problem_tracks[track], clean), f"non-target track {track} changed")

    clean_mix = np.sum(np.stack(list(clean_tracks.values())), axis=0)
    problem_mix = np.sum(np.stack(list(problem_tracks.values())), axis=0)
    raw_peak = max(float(np.max(np.abs(clean_mix))), float(np.max(np.abs(problem_mix))))
    ceiling = 10.0 ** (peak_ceiling_dbfs / 20.0)
    safety = min(1.0, ceiling / max(raw_peak, 1e-30))
    clean_scaled = clean_mix * safety
    problem_scaled = problem_mix * safety
    require(peak_dbfs(clean_scaled) <= peak_ceiling_dbfs + 1e-6, "scaled clean mix exceeds ceiling")
    require(peak_dbfs(problem_scaled) <= peak_ceiling_dbfs + 1e-6, "scaled problem mix exceeds ceiling")
    return {
        "public_case_id": project["public_case_id"],
        "status": "qualified",
        "issue_count": len(issue_results),
        "sample_rate_hz": sample_rate,
        "shared_safety_gain_db": round(db(safety), 4),
        "clean_mix_peak_dbfs_after_safety": round(peak_dbfs(clean_scaled), 4),
        "problem_mix_peak_dbfs_after_safety": round(peak_dbfs(problem_scaled), 4),
        "issues": issue_results,
    }


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(path)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contract", default=str(Path(__file__).with_name("semantic_processor_project_smoke_contract.json")))
    parser.add_argument("--output", default="")
    args = parser.parse_args()
    try:
        contract_path = Path(args.contract).resolve()
        contract = json.loads(contract_path.read_text(encoding="utf-8"))
        plan = contract["material_plan"]
        source_root = Path(plan["source_root"]).resolve()
        peak_ceiling = float(plan["interference_gates"]["clean_and_problem_mix_peak_ceiling_dbfs"])
        projects = [qualify_project(source_root, project, peak_ceiling) for project in plan["projects"]]
        report = {
            "schema_version": "semantic_processor_project_smoke_recipe_qualification.v1",
            "contract_id": contract["contract_id"],
            "status": "qualified",
            "audio_written": False,
            "project_count": len(projects),
            "issue_count": sum(project["issue_count"] for project in projects),
            "projects": projects,
        }
        require(report["project_count"] == 2 and report["issue_count"] == 7, "qualification count mismatch")
        if args.output:
            write_json(Path(args.output).resolve(), report)
        print(json.dumps(report, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
