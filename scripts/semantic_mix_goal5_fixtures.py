#!/usr/bin/env python3
"""Build the sealed, controlled audio fixtures for semantic-mix goal 5.

The generated WAV files intentionally use opaque case IDs.  Acoustic issue
labels, injected filter parameters, paired controls, and evaluation prompts are
written only to ``sealed/sealed_truth.json``.  The product/Agent must receive
only a case's ``stems`` directory and exact track context during a blind run.

Copyrighted source audio is never copied into the repository.  By default the
fixtures are generated below ``%LOCALAPPDATA%/Vit/Goal5Fixtures`` and projects
continue to reference those local generated WAV files.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import numpy as np
import soundfile as sf
from scipy.signal import lfilter


SCHEMA_VERSION = "semantic_mix_goal5_fixture.v1"
RECIPE_VERSION = "goal5_controlled_problems_20260729.1"
TRACK_ORDER = ("bass", "drums", "guitar", "other", "piano", "vocals")
TRACK_LABELS = {
    "bass": "Bass",
    "drums": "Drums",
    "guitar": "Guitar",
    "other": "Other",
    "piano": "Piano",
    "vocals": "Vocals",
}
BANDS_HZ = (
    (20.0, 60.0),
    (60.0, 120.0),
    (120.0, 250.0),
    (250.0, 500.0),
    (500.0, 1000.0),
    (1000.0, 2000.0),
    (2000.0, 4000.0),
    (4000.0, 8000.0),
    (8000.0, 16000.0),
)


@dataclass(frozen=True)
class SongRecipe:
    key: str
    directory: str
    file_prefix: str
    display_name: str
    start_seconds: float
    duration_seconds: float
    control_case: str


@dataclass(frozen=True)
class EQEdit:
    track: str
    shape: str
    frequency_hz: float
    gain_db: float
    q: float


@dataclass(frozen=True)
class CaseRecipe:
    case_id: str
    song_key: str
    issue_kind: str
    issue_summary: str
    edits: tuple[EQEdit, ...]
    prompts: tuple[str, ...]
    expected_focus_tracks: tuple[str, ...]
    expected_context_tracks: tuple[str, ...]
    expected_reasoning: tuple[str, ...]


SONGS = (
    SongRecipe(
        key="iw",
        directory="Da-iCE - I wonder_361760586",
        file_prefix="Da-iCE - I wonder_361760586",
        display_name="Da-iCE - I wonder",
        start_seconds=46.0,
        duration_seconds=20.0,
        control_case="g5_01",
    ),
    SongRecipe(
        key="hana",
        directory="ヨルシカ - 花も騒めく_546069536",
        file_prefix="ヨルシカ - 花も騒めく_546069536",
        display_name="ヨルシカ - 花も騒めく",
        start_seconds=162.0,
        duration_seconds=20.0,
        control_case="g5_04",
    ),
)


CASES = (
    CaseRecipe(
        case_id="g5_01",
        song_key="iw",
        issue_kind="control",
        issue_summary="Unmodified level-matched control for the I wonder excerpt.",
        edits=(),
        prompts=("为什么这一段主唱听起来浑？", "减少一些主唱的浑浊"),
        expected_focus_tracks=("vocals",),
        expected_context_tracks=("vocals", "guitar", "piano", "other"),
        expected_reasoning=(
            "Must not assert an injected low-mid excess merely because the prompt says muddy.",
            "Diagnosis-only wording must not create a mutation proposal.",
        ),
    ),
    CaseRecipe(
        case_id="g5_02",
        song_key="iw",
        issue_kind="vocal_low_mid_buildup",
        issue_summary="Broad, obvious vocal low-mid buildup while other stems remain identical to control.",
        edits=(EQEdit("vocals", "bell", 280.0, 5.0, 0.75),),
        prompts=("为什么这一段主唱听起来浑？", "减少一些主唱的浑浊"),
        expected_focus_tracks=("vocals",),
        expected_context_tracks=("vocals", "guitar", "piano", "other"),
        expected_reasoning=(
            "Locate a broad vocal excess in the low-mid region, not a project-wide low-end problem.",
            "An action request should form a conservative broad bell cut around the evidenced region.",
        ),
    ),
    CaseRecipe(
        case_id="g5_03",
        song_key="iw",
        issue_kind="cross_track_vocal_masking",
        issue_summary="Guitar and piano are pushed into vocal intelligibility bands; vocals are unchanged.",
        edits=(
            EQEdit("guitar", "bell", 2100.0, 4.5, 0.90),
            EQEdit("piano", "bell", 1700.0, 3.5, 0.90),
        ),
        prompts=("为什么副歌里人声听起来靠后？", "别让吉他挡住人声，但不要让吉他变薄"),
        expected_focus_tracks=("vocals", "guitar"),
        expected_context_tracks=("vocals", "guitar", "piano"),
        expected_reasoning=(
            "Compare the unchanged vocal against midrange maskers instead of blaming vocal tone alone.",
            "Preserve guitar body and prefer a bounded presence-region move if EQ is chosen.",
        ),
    ),
    CaseRecipe(
        case_id="g5_04",
        song_key="hana",
        issue_kind="control",
        issue_summary="Unmodified level-matched control for the 花も騒めく excerpt.",
        edits=(),
        prompts=("让底鼓和贝斯更分开", "让主唱更亮但不要更刺耳"),
        expected_focus_tracks=("bass", "drums", "vocals"),
        expected_context_tracks=("bass", "drums", "vocals", "guitar", "other"),
        expected_reasoning=(
            "Must not invent the controlled low-frequency collision or vocal dull/harsh combination.",
            "A clean control may justify no-op, clarification, or a materially different plan.",
        ),
    ),
    CaseRecipe(
        case_id="g5_05",
        song_key="hana",
        issue_kind="bass_drum_low_frequency_collision",
        issue_summary="Bass and drum low fundamentals are both exaggerated to create low-frequency competition.",
        edits=(
            EQEdit("bass", "low_shelf", 120.0, 5.0, 0.70),
            EQEdit("drums", "bell", 85.0, 4.0, 0.90),
        ),
        prompts=("让底鼓和贝斯更分开", "为什么这一段低频边界不清楚？"),
        expected_focus_tracks=("bass", "drums"),
        expected_context_tracks=("bass", "drums"),
        expected_reasoning=(
            "Inspect both bass and drums and explain their overlapping low-frequency evidence.",
            "Do not turn the observation scope into a project-wide mutation target.",
        ),
    ),
    CaseRecipe(
        case_id="g5_06",
        song_key="hana",
        issue_kind="vocal_dull_with_harsh_peak",
        issue_summary="Vocal air is reduced while a narrower upper-mid/high peak is exaggerated.",
        edits=(
            EQEdit("vocals", "high_shelf", 8000.0, -3.5, 0.70),
            EQEdit("vocals", "bell", 5200.0, 5.5, 3.20),
        ),
        prompts=("让主唱更亮，但不要让它更刺耳", "为什么主唱既不够亮又有点刺耳？"),
        expected_focus_tracks=("vocals",),
        expected_context_tracks=("vocals", "drums", "guitar", "other"),
        expected_reasoning=(
            "Keep brightness and harshness as a shared constraint rather than treating them independently.",
            "If proposing EQ, use distinct broad-brightness and narrow-harshness atoms supported by evidence.",
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


def json_bytes(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def source_path(source_root: Path, song: SongRecipe, track: str) -> Path:
    return source_root / song.directory / f"{song.file_prefix}_{track}.wav"


def source_inventory(source_root: Path) -> list[dict[str, Any]]:
    inventory: list[dict[str, Any]] = []
    for song in SONGS:
        for track in TRACK_ORDER:
            path = source_path(source_root, song, track)
            if not path.is_file():
                raise FileNotFoundError(f"missing source stem: {path}")
            info = sf.info(path)
            inventory.append(
                {
                    "song_key": song.key,
                    "track": track,
                    "path": str(path),
                    "sha256": sha256_file(path),
                    "size_bytes": path.stat().st_size,
                    "samplerate": info.samplerate,
                    "channels": info.channels,
                    "frames": info.frames,
                    "subtype": info.subtype,
                }
            )
    return inventory


def recipe_identity(inventory: list[dict[str, Any]]) -> str:
    recipe = {
        "schema_version": SCHEMA_VERSION,
        "recipe_version": RECIPE_VERSION,
        "songs": [song.__dict__ for song in SONGS],
        "cases": [
            {
                **{key: value for key, value in case.__dict__.items() if key != "edits"},
                "edits": [edit.__dict__ for edit in case.edits],
            }
            for case in CASES
        ],
        "source_identity": [
            {key: item[key] for key in ("song_key", "track", "sha256", "size_bytes")}
            for item in inventory
        ],
    }
    return hashlib.sha256(json_bytes(recipe)).hexdigest()[:12]


def _biquad(edit: EQEdit, samplerate: int) -> tuple[np.ndarray, np.ndarray]:
    frequency = float(edit.frequency_hz)
    q = float(edit.q)
    gain_db = float(edit.gain_db)
    if not 10.0 <= frequency < samplerate * 0.49:
        raise ValueError(f"invalid frequency {frequency} for samplerate {samplerate}")
    if q <= 0.0:
        raise ValueError(f"invalid Q {q}")

    omega = 2.0 * math.pi * frequency / samplerate
    cos_w = math.cos(omega)
    sin_w = math.sin(omega)
    a = 10.0 ** (gain_db / 40.0)

    if edit.shape == "bell":
        alpha = sin_w / (2.0 * q)
        b0 = 1.0 + alpha * a
        b1 = -2.0 * cos_w
        b2 = 1.0 - alpha * a
        a0 = 1.0 + alpha / a
        a1 = -2.0 * cos_w
        a2 = 1.0 - alpha / a
    elif edit.shape in {"low_shelf", "high_shelf"}:
        # RBJ shelf. Q is represented as shelf slope S for deterministic
        # fixture construction; S=0.7 is a gentle, normal shelf transition.
        slope = q
        alpha = sin_w / 2.0 * math.sqrt((a + 1.0 / a) * (1.0 / slope - 1.0) + 2.0)
        two_sqrt_a_alpha = 2.0 * math.sqrt(a) * alpha
        if edit.shape == "low_shelf":
            b0 = a * ((a + 1.0) - (a - 1.0) * cos_w + two_sqrt_a_alpha)
            b1 = 2.0 * a * ((a - 1.0) - (a + 1.0) * cos_w)
            b2 = a * ((a + 1.0) - (a - 1.0) * cos_w - two_sqrt_a_alpha)
            a0 = (a + 1.0) + (a - 1.0) * cos_w + two_sqrt_a_alpha
            a1 = -2.0 * ((a - 1.0) + (a + 1.0) * cos_w)
            a2 = (a + 1.0) + (a - 1.0) * cos_w - two_sqrt_a_alpha
        else:
            b0 = a * ((a + 1.0) + (a - 1.0) * cos_w + two_sqrt_a_alpha)
            b1 = -2.0 * a * ((a - 1.0) + (a + 1.0) * cos_w)
            b2 = a * ((a + 1.0) + (a - 1.0) * cos_w - two_sqrt_a_alpha)
            a0 = (a + 1.0) - (a - 1.0) * cos_w + two_sqrt_a_alpha
            a1 = 2.0 * ((a - 1.0) - (a + 1.0) * cos_w)
            a2 = (a + 1.0) - (a - 1.0) * cos_w - two_sqrt_a_alpha
    else:
        raise ValueError(f"unsupported fixture EQ shape: {edit.shape}")

    return (
        np.asarray([b0 / a0, b1 / a0, b2 / a0], dtype=np.float64),
        np.asarray([1.0, a1 / a0, a2 / a0], dtype=np.float64),
    )


def apply_edit(audio: np.ndarray, samplerate: int, edit: EQEdit) -> np.ndarray:
    b, a = _biquad(edit, samplerate)
    return np.stack([lfilter(b, a, audio[:, channel]) for channel in range(audio.shape[1])], axis=1)


def db(value: float, floor: float = -180.0) -> float:
    if value <= 0.0:
        return floor
    return 20.0 * math.log10(value)


def metrics(audio: np.ndarray, samplerate: int) -> dict[str, Any]:
    mono = np.mean(audio, axis=1)
    peak = float(np.max(np.abs(audio)))
    rms = float(np.sqrt(np.mean(np.square(audio))))
    window = np.hanning(len(mono))
    spectrum = np.fft.rfft(mono * window)
    frequencies = np.fft.rfftfreq(len(mono), 1.0 / samplerate)
    power = np.square(np.abs(spectrum))
    total = float(np.sum(power))
    band_rows: list[dict[str, Any]] = []
    for low, high in BANDS_HZ:
        mask = (frequencies >= low) & (frequencies < high)
        band_power = float(np.sum(power[mask]))
        band_rows.append(
            {
                "low_hz": low,
                "high_hz": high,
                "relative_db": round(10.0 * math.log10(max(band_power, 1e-30) / max(total, 1e-30)), 4),
            }
        )
    return {
        "peak_dbfs": round(db(peak), 4),
        "rms_dbfs": round(db(rms), 4),
        "crest_db": round(db(peak / max(rms, 1e-30)), 4),
        "bands": band_rows,
    }


def fade_edges(audio: np.ndarray, samplerate: int, seconds: float = 0.05) -> np.ndarray:
    frames = min(int(round(seconds * samplerate)), len(audio) // 2)
    if frames <= 1:
        return audio
    out = audio.copy()
    fade = np.sin(np.linspace(0.0, math.pi / 2.0, frames, endpoint=True)) ** 2
    out[:frames] *= fade[:, None]
    out[-frames:] *= fade[::-1, None]
    return out


def load_song_excerpt(source_root: Path, song: SongRecipe) -> tuple[int, dict[str, np.ndarray], dict[str, Any]]:
    tracks: dict[str, np.ndarray] = {}
    samplerate: int | None = None
    expected_frames: int | None = None
    source_rows: list[dict[str, Any]] = []
    for track in TRACK_ORDER:
        path = source_path(source_root, song, track)
        info = sf.info(path)
        if info.channels != 2 or info.subtype != "PCM_16":
            raise ValueError(f"expected stereo PCM_16 source: {path} ({info})")
        if samplerate is None:
            samplerate = info.samplerate
            expected_frames = int(round(song.duration_seconds * samplerate))
        if info.samplerate != samplerate:
            raise ValueError(f"sample-rate mismatch in {song.directory}: {path}")
        start = int(round(song.start_seconds * samplerate))
        data, read_rate = sf.read(
            path,
            start=start,
            frames=expected_frames,
            dtype="float64",
            always_2d=True,
        )
        if read_rate != samplerate or len(data) != expected_frames:
            raise ValueError(f"could not read exact excerpt from {path}: {len(data)}/{expected_frames}")
        tracks[track] = fade_edges(data, samplerate)
        source_rows.append({"track": track, "path": str(path), "source_start_frame": start})

    assert samplerate is not None and expected_frames is not None
    summed = np.sum(np.stack([tracks[name] for name in TRACK_ORDER], axis=0), axis=0)
    clean_peak = float(np.max(np.abs(summed)))
    target_peak = 10.0 ** (-9.0 / 20.0)
    shared_gain = min(1.0, target_peak / max(clean_peak, 1e-30))
    tracks = {name: value * shared_gain for name, value in tracks.items()}
    return samplerate, tracks, {
        "source_rows": source_rows,
        "frames": expected_frames,
        "shared_gain_db": round(db(shared_gain), 6),
        "pre_gain_sum_peak_dbfs": round(db(clean_peak), 6),
        "target_clean_sum_peak_dbfs": -9.0,
    }


def clone_tracks(tracks: dict[str, np.ndarray]) -> dict[str, np.ndarray]:
    return {name: audio.copy() for name, audio in tracks.items()}


def band_delta(problem: dict[str, Any], control: dict[str, Any]) -> list[dict[str, Any]]:
    control_by_band = {
        (row["low_hz"], row["high_hz"]): row["relative_db"] for row in control["bands"]
    }
    return [
        {
            "low_hz": row["low_hz"],
            "high_hz": row["high_hz"],
            "relative_delta_db": round(
                row["relative_db"] - control_by_band[(row["low_hz"], row["high_hz"])], 4
            ),
        }
        for row in problem["bands"]
    ]


def verify_written_case(case_dir: Path, expected_frames: int, samplerate: int) -> dict[str, Any]:
    tracks: dict[str, np.ndarray] = {}
    rows: list[dict[str, Any]] = []
    for track in TRACK_ORDER:
        path = case_dir / "stems" / f"{TRACK_LABELS[track]}.wav"
        info = sf.info(path)
        if info.samplerate != samplerate or info.channels != 2 or info.frames != expected_frames:
            raise AssertionError(f"fixture format mismatch: {path} ({info})")
        if info.subtype != "PCM_16":
            raise AssertionError(f"fixture must be PCM_16: {path} ({info.subtype})")
        audio, _ = sf.read(path, dtype="float64", always_2d=True)
        tracks[track] = audio
        rows.append(
            {
                "track": track,
                "file": str(path),
                "sha256": sha256_file(path),
                "size_bytes": path.stat().st_size,
                "metrics": metrics(audio, samplerate),
            }
        )
    mix = np.sum(np.stack([tracks[name] for name in TRACK_ORDER], axis=0), axis=0)
    mix_peak = float(np.max(np.abs(mix)))
    if mix_peak >= 0.98:
        raise AssertionError(f"fixture sum has insufficient headroom: {case_dir} peak={db(mix_peak):.3f} dBFS")
    return {"tracks": rows, "sum_metrics": metrics(mix, samplerate)}


def build(args: argparse.Namespace) -> dict[str, Any]:
    source_root = Path(args.source_root).resolve()
    output_root = Path(args.output_root).resolve()
    inventory = source_inventory(source_root)
    identity = recipe_identity(inventory)
    set_id = f"semantic_mix_goal5_v1_{identity}"
    set_root = output_root / set_id
    cases_root = set_root / "cases"
    sealed_root = set_root / "sealed"
    set_root.mkdir(parents=True, exist_ok=True)

    song_audio: dict[str, dict[str, np.ndarray]] = {}
    song_rates: dict[str, int] = {}
    song_meta: dict[str, dict[str, Any]] = {}
    for song in SONGS:
        rate, audio, meta = load_song_excerpt(source_root, song)
        song_audio[song.key] = audio
        song_rates[song.key] = rate
        song_meta[song.key] = meta

    public_cases: list[dict[str, Any]] = []
    sealed_cases: list[dict[str, Any]] = []
    case_metrics: dict[str, dict[str, Any]] = {}
    song_by_key = {song.key: song for song in SONGS}

    for case in CASES:
        song = song_by_key[case.song_key]
        samplerate = song_rates[case.song_key]
        tracks = clone_tracks(song_audio[case.song_key])
        for edit in case.edits:
            tracks[edit.track] = apply_edit(tracks[edit.track], samplerate, edit)

        case_dir = cases_root / case.case_id
        stems_dir = case_dir / "stems"
        stems_dir.mkdir(parents=True, exist_ok=True)
        for track in TRACK_ORDER:
            path = stems_dir / f"{TRACK_LABELS[track]}.wav"
            temp = path.with_suffix(".tmp.wav")
            sf.write(temp, tracks[track], samplerate, subtype="PCM_16", format="WAV")
            os.replace(temp, path)

        verified = verify_written_case(case_dir, song_meta[case.song_key]["frames"], samplerate)
        case_metrics[case.case_id] = verified
        public_cases.append(
            {
                "case_id": case.case_id,
                "song_key": case.song_key,
                "source_display_name": song.display_name,
                "stems_directory": str(stems_dir),
                "project_path": str(case_dir / f"{case.case_id}.vit"),
                "track_order": list(TRACK_ORDER),
                "samplerate": samplerate,
                "channels": 2,
                "frames": song_meta[case.song_key]["frames"],
                "duration_seconds": song.duration_seconds,
                "sum_peak_dbfs": verified["sum_metrics"]["peak_dbfs"],
                "stem_files": [
                    {
                        "track": row["track"],
                        "file": row["file"],
                        "sha256": row["sha256"],
                        "size_bytes": row["size_bytes"],
                    }
                    for row in verified["tracks"]
                ],
            }
        )
        sealed_cases.append(
            {
                "case_id": case.case_id,
                "song_key": case.song_key,
                "control_case_id": song.control_case,
                "issue_kind": case.issue_kind,
                "issue_summary": case.issue_summary,
                "injected_eq_edits": [edit.__dict__ for edit in case.edits],
                "blind_prompts": list(case.prompts),
                "expected_focus_tracks": list(case.expected_focus_tracks),
                "expected_context_tracks": list(case.expected_context_tracks),
                "expected_reasoning": list(case.expected_reasoning),
            }
        )

    for sealed_case in sealed_cases:
        control_id = sealed_case["control_case_id"]
        case_id = sealed_case["case_id"]
        control_tracks = {row["track"]: row for row in case_metrics[control_id]["tracks"]}
        deltas: list[dict[str, Any]] = []
        for row in case_metrics[case_id]["tracks"]:
            control = control_tracks[row["track"]]
            deltas.append(
                {
                    "track": row["track"],
                    "peak_delta_db": round(
                        row["metrics"]["peak_dbfs"] - control["metrics"]["peak_dbfs"], 4
                    ),
                    "rms_delta_db": round(
                        row["metrics"]["rms_dbfs"] - control["metrics"]["rms_dbfs"], 4
                    ),
                    "band_deltas": band_delta(row["metrics"], control["metrics"]),
                }
            )
        sealed_case["measured_output_deltas_vs_control"] = deltas

    public_manifest = {
        "schema_version": SCHEMA_VERSION,
        "recipe_version": RECIPE_VERSION,
        "set_id": set_id,
        "created_at": now_iso(),
        "blindness_contract": {
            "agent_visible_root": str(cases_root),
            "sealed_truth_not_in_agent_context": True,
            "case_names_are_semantically_opaque": True,
        },
        "format": {"samplerate": 44100, "channels": 2, "subtype": "PCM_16"},
        "cases": public_cases,
    }
    sealed_truth = {
        "schema_version": SCHEMA_VERSION,
        "recipe_version": RECIPE_VERSION,
        "set_id": set_id,
        "warning": "Evaluator-only truth. Never send this file or its path to the product Agent.",
        "source_inventory": inventory,
        "song_excerpts": [
            {
                **song.__dict__,
                **song_meta[song.key],
            }
            for song in SONGS
        ],
        "cases": sealed_cases,
        "evaluation_invariants": [
            "Diagnosis-only prompts perform zero mutation and create no execution confirmation.",
            "A problem case and its clean control must not receive an evidence-independent identical diagnosis.",
            "Observation scope may contain related tracks, but every proposed mutation target is exact.",
            "The model may disagree with injected exact parameters but must locate the affected object and region.",
            "Uncertainty or a justified no-op is preferable to claiming unavailable acoustic evidence.",
        ],
    }
    write_json_atomic(set_root / "fixture_manifest.json", public_manifest)
    write_json_atomic(sealed_root / "sealed_truth.json", sealed_truth)
    write_json_atomic(
        output_root / "current.json",
        {
            "schema_version": SCHEMA_VERSION,
            "set_id": set_id,
            "fixture_manifest": str(set_root / "fixture_manifest.json"),
            "sealed_truth": str(sealed_root / "sealed_truth.json"),
        },
    )
    return {
        "status": "passed",
        "set_id": set_id,
        "set_root": str(set_root),
        "fixture_manifest": str(set_root / "fixture_manifest.json"),
        "sealed_truth": str(sealed_root / "sealed_truth.json"),
        "case_count": len(public_cases),
        "stem_count": len(public_cases) * len(TRACK_ORDER),
        "minimum_sum_headroom_db": round(-max(row["sum_peak_dbfs"] for row in public_cases), 4),
    }


def verify(args: argparse.Namespace) -> dict[str, Any]:
    output_root = Path(args.output_root).resolve()
    current_path = output_root / "current.json"
    if not current_path.is_file():
        raise FileNotFoundError(f"fixture pointer does not exist: {current_path}")
    current = json.loads(current_path.read_text(encoding="utf-8"))
    manifest_path = Path(current["fixture_manifest"])
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise AssertionError(f"unexpected fixture schema: {manifest.get('schema_version')}")
    verified_files = 0
    minimum_headroom = math.inf
    for case in manifest.get("cases", []):
        expected_frames = int(case["frames"])
        samplerate = int(case["samplerate"])
        result = verify_written_case(Path(case["stems_directory"]).parent, expected_frames, samplerate)
        expected_hashes = {row["track"]: row["sha256"] for row in case["stem_files"]}
        for row in result["tracks"]:
            if row["sha256"] != expected_hashes[row["track"]]:
                raise AssertionError(f"fixture hash mismatch: {row['file']}")
            verified_files += 1
        minimum_headroom = min(minimum_headroom, -float(result["sum_metrics"]["peak_dbfs"]))
    return {
        "status": "passed",
        "set_id": manifest["set_id"],
        "fixture_manifest": str(manifest_path),
        "case_count": len(manifest.get("cases", [])),
        "verified_stem_count": verified_files,
        "minimum_sum_headroom_db": round(minimum_headroom, 4),
    }


def default_output_root() -> str:
    local = os.environ.get("LOCALAPPDATA")
    if local:
        return str(Path(local) / "Vit" / "Goal5Fixtures")
    return str(Path.home() / "AppData" / "Local" / "Vit" / "Goal5Fixtures")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--source-root",
        default=r"C:\Users\timoz\Documents\RipX\Stems",
        help="Folder containing the two six-stem source directories.",
    )
    parser.add_argument("--output-root", default=default_output_root())
    parser.add_argument("--verify-only", action="store_true")
    args = parser.parse_args()
    try:
        result = verify(args) if args.verify_only else build(args)
    except Exception as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
