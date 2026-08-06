#!/usr/bin/env python3
"""Build sealed open-semantic EQ/compressor experiment fixtures.

The source material is the clean, level-matched control audio from the existing
Goal 5 fixture set. Public case directories contain only opaque problem stems.
Fault recipes, expected processor classes, clean references, and evaluator
metrics remain below ``sealed`` and must never enter Agent context.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf
from scipy.signal import fftconvolve, find_peaks

from semantic_mix_goal5_fixtures import EQEdit, apply_edit


SCHEMA_VERSION = "semantic_processor_open_fixture.v1"
RECIPE_VERSION = "semantic_processor_open_20260805.3"
TRACK_ORDER = ("bass", "drums", "guitar", "other", "piano", "vocals")
TRACK_LABELS = {name: name.capitalize() for name in TRACK_ORDER}


@dataclass(frozen=True)
class DynamicEdit:
    track: str
    kind: str
    depth_db: float
    period_seconds: float
    seed: int


@dataclass(frozen=True)
class CaseRecipe:
    case_id: str
    suite: str
    source_case_id: str
    issue_kind: str
    issue_summary: str
    prompt: str
    expected_first_processors: tuple[str, ...]
    expected_eventual_processors: tuple[str, ...]
    focus_track: str
    dynamic_edits: tuple[DynamicEdit, ...] = ()
    eq_edits: tuple[EQEdit, ...] = ()


CASES = (
    CaseRecipe(
        "so_01", "compressor_only", "g5_01", "control",
        "Clean control for the vocal-stability request.",
        "\u4e3b\u5531\u542c\u8d77\u6765\u65f6\u5927\u65f6\u5c0f\u5417\uff1f\u5982\u679c\u786e\u5b9e\u6709\u95ee\u9898\uff0c\u518d\u5e2e\u6211\u8ba9\u5b83\u66f4\u7a33\u5b9a\uff0c\u4fdd\u7559\u81ea\u7136\u8d77\u4f0f\u548c\u54ac\u5b57\u77ac\u6001\u3002",
        ("none",), ("none",), "vocals",
    ),
    CaseRecipe(
        "so_02", "compressor_only", "g5_01", "vocal_macro_dynamics",
        "The vocal phrase envelope alternates between materially loud and quiet sections at matched RMS.",
        "\u8ba9\u4e3b\u5531\u66f4\u7a33\u5b9a\u5730\u9760\u524d\uff0c\u4f46\u4fdd\u7559\u81ea\u7136\u8d77\u4f0f\u548c\u54ac\u5b57\u77ac\u6001\u3002",
        ("compressor",), ("compressor",), "vocals",
        dynamic_edits=(DynamicEdit("vocals", "macro_steps", 6.5, 1.25, 1201),),
    ),
    CaseRecipe(
        "so_03", "compressor_only", "g5_04", "drum_transient_overshoot",
        "Selected drum attacks overshoot while drum RMS remains matched to the control.",
        "\u628a\u9f13\u7ec4\u5076\u5c14\u7a81\u51fa\u7684\u5cf0\u503c\u6536\u7a33\u4e00\u4e9b\uff0c\u4f46\u4e0d\u8981\u628a\u51fb\u6253\u611f\u538b\u6241\u3002",
        ("compressor",), ("compressor",), "drums",
        dynamic_edits=(DynamicEdit("drums", "transient_spikes", 7.5, 0.38, 2303),),
    ),
    CaseRecipe(
        "so_04", "compressor_only", "g5_04", "bass_note_inconsistency",
        "Bass note groups alternate in level while spectral balance and RMS remain close to control.",
        "\u8ba9\u8d1d\u65af\u6bcf\u4e2a\u97f3\u7684\u5b58\u5728\u611f\u66f4\u5747\u5300\uff0c\u4f46\u522b\u8ba9\u4f4e\u9891\u53d8\u8584\u3002",
        ("compressor",), ("compressor",), "bass",
        dynamic_edits=(DynamicEdit("bass", "note_steps", 5.5, 0.52, 3407),),
    ),
    CaseRecipe(
        "so_05", "eq_compressor_mixed", "g5_01", "eq_only_vocal_low_mid",
        "Vocal low-mid buildup with unchanged source dynamics.",
        "\u4e3b\u5531\u6709\u70b9\u53d1\u95f7\u3001\u4f4e\u4e2d\u9891\u5806\u5728\u4e00\u8d77\uff0c\u8ba9\u5b83\u66f4\u6e05\u695a\u5730\u7ad9\u5230\u524d\u9762\uff0c\u4f46\u522b\u628a\u58f0\u97f3\u505a\u8584\u3002",
        ("eq",), ("eq",), "vocals",
        eq_edits=(EQEdit("vocals", "bell", 280.0, 7.0, 0.75),),
    ),
    CaseRecipe(
        "so_06", "eq_compressor_mixed", "g5_01", "compressor_only_vocal_macro",
        "Vocal macro dynamics are unstable without an injected spectral fault.",
        "\u8ba9\u4e3b\u5531\u66f4\u6301\u7eed\u5730\u7ad9\u5230\u524d\u9762\uff0c\u4fdd\u7559\u97f3\u8272\u548c\u81ea\u7136\u54ac\u5b57\u3002",
        ("compressor",), ("compressor",), "vocals",
        dynamic_edits=(DynamicEdit("vocals", "macro_steps", 6.5, 1.25, 4603),),
    ),
    CaseRecipe(
        "so_07", "eq_compressor_mixed", "g5_01", "vocal_eq_and_dynamics",
        "The same vocal has broad low-mid buildup and unstable phrase dynamics.",
        "\u4e3b\u5531\u6709\u70b9\u7cca\uff0c\u800c\u4e14\u65f6\u5927\u65f6\u5c0f\uff0c\u5e2e\u6211\u6574\u7406\u5f97\u66f4\u6e05\u695a\u7a33\u5b9a\uff0c\u4f46\u522b\u505a\u8584\u6216\u538b\u6b7b\u3002",
        ("eq", "compressor"), ("eq", "compressor"), "vocals",
        dynamic_edits=(DynamicEdit("vocals", "macro_steps", 6.0, 1.25, 5701),),
        eq_edits=(EQEdit("vocals", "bell", 280.0, 7.0, 0.75),),
    ),
    CaseRecipe(
        "so_08", "eq_compressor_mixed", "g5_01", "masking_and_vocal_dynamics",
        "Guitar and piano mask vocal presence while the vocal phrase envelope is also unstable.",
        "\u4eba\u58f0\u603b\u662f\u88ab\u4f34\u594f\u76d6\u4f4f\uff0c\u81ea\u5df1\u7684\u5b58\u5728\u611f\u4e5f\u4e0d\u7a33\uff0c\u8ba9\u5b83\u66f4\u9760\u524d\uff0c\u4f46\u522b\u628a\u4f34\u594f\u505a\u8584\u3002",
        ("eq", "compressor"), ("eq", "compressor"), "vocals",
        dynamic_edits=(DynamicEdit("vocals", "macro_steps", 5.5, 1.1, 6803),),
        eq_edits=(
            EQEdit("guitar", "bell", 2100.0, 6.5, 0.9),
            EQEdit("piano", "bell", 1700.0, 5.5, 0.9),
        ),
    ),
)


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def rms(audio: np.ndarray) -> float:
    return float(np.sqrt(np.mean(np.square(audio))))


def db(value: float, floor: float = -180.0) -> float:
    return floor if value <= 0 else 20.0 * math.log10(value)


def smooth_curve(values: np.ndarray, samplerate: int, milliseconds: float) -> np.ndarray:
    half = max(1, int(round(samplerate * milliseconds / 1000.0)))
    window = np.hanning(half * 2 + 1)
    window /= np.sum(window)
    return fftconvolve(values, window, mode="same")


def stepped_gain(frames: int, samplerate: int, edit: DynamicEdit) -> np.ndarray:
    block = max(1, int(round(edit.period_seconds * samplerate)))
    count = int(math.ceil(frames / block))
    rng = np.random.default_rng(edit.seed)
    signs = np.where(np.arange(count) % 2 == 0, 1.0, -1.0)
    magnitudes = rng.uniform(0.65, 1.0, count) * edit.depth_db
    gain_db = np.repeat(signs * magnitudes, block)[:frames]
    return smooth_curve(gain_db, samplerate, 65.0 if edit.kind == "note_steps" else 120.0)


def transient_gain(audio: np.ndarray, samplerate: int, edit: DynamicEdit) -> np.ndarray:
    mono = np.max(np.abs(audio), axis=1)
    envelope = smooth_curve(mono, samplerate, 4.0)
    distance = max(1, int(round(edit.period_seconds * samplerate)))
    peaks, _ = find_peaks(envelope, distance=distance, prominence=max(1e-5, float(np.std(envelope)) * 0.7))
    if len(peaks) == 0:
        raise RuntimeError("transient fixture could not find any drum attacks")
    strongest = peaks[np.argsort(envelope[peaks])[-min(18, len(peaks)):]]
    gain_db = np.zeros(len(audio), dtype=np.float64)
    attack = max(1, int(round(0.006 * samplerate)))
    release = max(1, int(round(0.095 * samplerate)))
    for peak in strongest:
        start = max(0, peak - attack)
        end = min(len(audio), peak + release)
        left = np.linspace(0.0, edit.depth_db, peak - start + 1)
        right = edit.depth_db * np.exp(-np.linspace(0.0, 5.0, end - peak - 1))
        shape = np.concatenate((left, right))
        gain_db[start:end] = np.maximum(gain_db[start:end], shape[: end - start])
    return gain_db


def apply_dynamic(audio: np.ndarray, samplerate: int, edit: DynamicEdit) -> np.ndarray:
    before_rms = rms(audio)
    if edit.kind in {"macro_steps", "note_steps"}:
        gain_db = stepped_gain(len(audio), samplerate, edit)
    elif edit.kind == "transient_spikes":
        gain_db = transient_gain(audio, samplerate, edit)
    else:
        raise ValueError(f"unsupported dynamic edit: {edit.kind}")
    out = audio * np.power(10.0, gain_db / 20.0)[:, None]
    out *= before_rms / max(rms(out), 1e-30)
    return out


def dynamics_metrics(audio: np.ndarray, samplerate: int) -> dict[str, float]:
    mono = np.mean(audio, axis=1)
    window = max(1, int(round(0.1 * samplerate)))
    usable = len(mono) // window * window
    blocks = mono[:usable].reshape(-1, window)
    block_rms = np.sqrt(np.mean(np.square(blocks), axis=1))
    active = block_rms[block_rms > max(float(np.max(block_rms)) * 0.01, 1e-8)]
    active_db = 20.0 * np.log10(np.maximum(active, 1e-30))
    peak = float(np.max(np.abs(audio)))
    total_rms = rms(audio)
    return {
        "peak_dbfs": round(db(peak), 4),
        "rms_dbfs": round(db(total_rms), 4),
        "crest_db": round(db(peak / max(total_rms, 1e-30)), 4),
        "active_100ms_range_db": round(float(np.percentile(active_db, 95) - np.percentile(active_db, 10)), 4),
        "active_100ms_std_db": round(float(np.std(active_db)), 4),
    }


def default_goal5_manifest() -> Path:
    local = os.environ.get("LOCALAPPDATA")
    if not local:
        raise RuntimeError("LOCALAPPDATA is unavailable")
    pointer = Path(local) / "Vit" / "Goal5Fixtures" / "current.json"
    value = json.loads(pointer.read_text(encoding="utf-8"))
    return Path(value["fixture_manifest"]).resolve()


def load_control(manifest: dict[str, Any], case_id: str) -> tuple[int, dict[str, np.ndarray], dict[str, Any]]:
    case = next(row for row in manifest["cases"] if row["case_id"] == case_id)
    tracks: dict[str, np.ndarray] = {}
    rate = int(case["samplerate"])
    sources: list[dict[str, Any]] = []
    for name in TRACK_ORDER:
        path = Path(case["stems_directory"]) / f"{TRACK_LABELS[name]}.wav"
        audio, actual_rate = sf.read(path, dtype="float64", always_2d=True)
        if actual_rate != rate or audio.shape[1] != 2:
            raise ValueError(f"invalid control stem: {path}")
        tracks[name] = audio
        sources.append({"track": name, "path": str(path), "sha256": sha256_file(path)})
    return rate, tracks, {"goal5_case_id": case_id, "sources": sources}


def build_case(case: CaseRecipe, manifest: dict[str, Any], cases_root: Path, sealed_root: Path) -> tuple[dict[str, Any], dict[str, Any]]:
    samplerate, clean, source = load_control(manifest, case.source_case_id)
    problem = {name: audio.copy() for name, audio in clean.items()}
    for edit in case.dynamic_edits:
        problem[edit.track] = apply_dynamic(problem[edit.track], samplerate, edit)
    for edit in case.eq_edits:
        before_rms = rms(problem[edit.track])
        problem[edit.track] = apply_edit(problem[edit.track], samplerate, edit)
        problem[edit.track] *= before_rms / max(rms(problem[edit.track]), 1e-30)

    clean_mix = np.sum(np.stack([clean[name] for name in TRACK_ORDER]), axis=0)
    problem_mix = np.sum(np.stack([problem[name] for name in TRACK_ORDER]), axis=0)
    max_peak = max(float(np.max(np.abs(clean_mix))), float(np.max(np.abs(problem_mix))))
    safety = min(1.0, (10.0 ** (-3.0 / 20.0)) / max(max_peak, 1e-30))
    clean = {name: audio * safety for name, audio in clean.items()}
    problem = {name: audio * safety for name, audio in problem.items()}
    clean_mix = np.sum(np.stack([clean[name] for name in TRACK_ORDER]), axis=0)
    problem_mix = np.sum(np.stack([problem[name] for name in TRACK_ORDER]), axis=0)

    case_dir = cases_root / case.case_id
    stems_dir = case_dir / "stems"
    stems_dir.mkdir(parents=True, exist_ok=True)
    files = []
    metric_rows = []
    for name in TRACK_ORDER:
        path = stems_dir / f"{TRACK_LABELS[name]}.wav"
        sf.write(path, problem[name], samplerate, subtype="PCM_16")
        files.append({"track": name, "file": str(path), "sha256": sha256_file(path), "size_bytes": path.stat().st_size})
        metric_rows.append({
            "track": name,
            "control": dynamics_metrics(clean[name], samplerate),
            "problem": dynamics_metrics(problem[name], samplerate),
        })

    refs = sealed_root / "references" / case.case_id
    refs.mkdir(parents=True, exist_ok=True)
    problem_path = refs / "problem_mix.wav"
    matched_path = refs / "problem_mix_level_matched.wav"
    clean_path = refs / "clean_reference.wav"
    fixed_path = refs / "reference_fixed.wav"
    match_gain = rms(clean_mix) / max(rms(problem_mix), 1e-30)
    sf.write(problem_path, problem_mix, samplerate, subtype="PCM_16")
    sf.write(matched_path, problem_mix * match_gain, samplerate, subtype="PCM_16")
    sf.write(clean_path, clean_mix, samplerate, subtype="PCM_16")
    sf.write(fixed_path, clean_mix, samplerate, subtype="PCM_16")

    public = {
        "case_id": case.case_id,
        "suite": case.suite,
        "stems_directory": str(stems_dir),
        "project_path": str(case_dir / f"{case.case_id}.vit"),
        "track_order": list(TRACK_ORDER),
        "samplerate": samplerate,
        "channels": 2,
        "frames": len(problem_mix),
        "duration_seconds": round(len(problem_mix) / samplerate, 4),
        "stem_files": files,
    }
    sealed = {
        "case_id": case.case_id,
        "suite": case.suite,
        "source": source,
        "issue_kind": case.issue_kind,
        "issue_summary": case.issue_summary,
        "blind_prompt": case.prompt,
        "focus_track": case.focus_track,
        "expected_first_processors": list(case.expected_first_processors),
        "expected_eventual_processors": list(case.expected_eventual_processors),
        "dynamic_edits": [asdict(edit) for edit in case.dynamic_edits],
        "eq_edits": [asdict(edit) for edit in case.eq_edits],
        "safety_gain_db": round(db(safety), 6),
        "problem_mix_match_gain_db": round(db(match_gain), 6),
        "track_metrics": metric_rows,
        "mix_metrics": {
            "clean": dynamics_metrics(clean_mix, samplerate),
            "problem": dynamics_metrics(problem_mix, samplerate),
            "problem_level_matched": dynamics_metrics(problem_mix * match_gain, samplerate),
        },
        "references": {
            "problem_mix": str(problem_path),
            "problem_mix_level_matched": str(matched_path),
            "clean_reference": str(clean_path),
            "reference_fixed": str(fixed_path),
        },
    }
    return public, sealed


def build(args: argparse.Namespace) -> dict[str, Any]:
    goal5_path = Path(args.goal5_manifest).resolve() if args.goal5_manifest else default_goal5_manifest()
    goal5 = json.loads(goal5_path.read_text(encoding="utf-8"))
    identity_payload = RECIPE_VERSION + "\n" + "\n".join(
        row["sha256"] for case in goal5["cases"] if case["case_id"] in {"g5_01", "g5_04"} for row in case["stem_files"]
    )
    set_id = "semantic_processor_open_v1_" + hashlib.sha256(identity_payload.encode()).hexdigest()[:12]
    root = Path(args.output_root).resolve() / set_id
    cases_root = root / "cases"
    sealed_root = root / "sealed"
    public_cases = []
    sealed_cases = []
    for case in CASES:
        public, sealed = build_case(case, goal5, cases_root, sealed_root)
        public_cases.append(public)
        sealed_cases.append(sealed)

    manifest = {
        "schema_version": SCHEMA_VERSION,
        "recipe_version": RECIPE_VERSION,
        "set_id": set_id,
        "created_at": now_iso(),
        "blindness_contract": {
            "agent_visible_root": str(cases_root),
            "sealed_truth_not_in_agent_context": True,
            "case_names_are_semantically_opaque": True,
            "projects_contain_problem_stems_only": True,
            "no_problem_making_plugins_in_projects": True,
        },
        "format": {"samplerate": 44100, "channels": 2, "subtype": "PCM_16"},
        "suites": {"compressor_only": 4, "eq_compressor_mixed": 4},
        "cases": public_cases,
    }
    truth = {
        "schema_version": SCHEMA_VERSION,
        "recipe_version": RECIPE_VERSION,
        "set_id": set_id,
        "warning": "Evaluator-only truth. Never send this file or its path to the product Agent.",
        "goal5_source_manifest": str(goal5_path),
        "cases": sealed_cases,
    }
    manifest_path = root / "fixture_manifest.json"
    truth_path = sealed_root / "sealed_truth.json"
    write_json_atomic(manifest_path, manifest)
    write_json_atomic(truth_path, truth)
    pointer = Path(args.output_root).resolve() / "current.json"
    write_json_atomic(pointer, {
        "schema_version": SCHEMA_VERSION,
        "set_id": set_id,
        "fixture_manifest": str(manifest_path),
        "sealed_truth": str(truth_path),
    })
    return {"status": "passed", "set_id": set_id, "fixture_manifest": str(manifest_path), "sealed_truth": str(truth_path), "case_count": len(public_cases)}


def main() -> int:
    local = os.environ.get("LOCALAPPDATA", "")
    default_output = str(Path(local) / "Vit" / "SemanticProcessorFixtures") if local else ""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--goal5-manifest", default="")
    parser.add_argument("--output-root", default=default_output)
    args = parser.parse_args()
    if not args.output_root:
        parser.error("--output-root is required when LOCALAPPDATA is unavailable")
    try:
        result = build(args)
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
