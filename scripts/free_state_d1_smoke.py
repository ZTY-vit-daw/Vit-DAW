#!/usr/bin/env python3
"""Public-only real-stack smoke for the Vit-DAW D1-S1 track-gain experiment."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import shutil
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any


PUBLIC_SCHEMA = "semantic_processor_agent_project_smoke_public_manifest.v1"
DEFAULT_CASE_ID = "spv1_p01"
OPEN_PROMPT = "检查一下当前工程有什么问题？"
# Prompt flavors are case-agnostic on purpose (the fixture blindness contract
# requires one neutral prompt per flavor across all cases; no sealed truth is
# encoded here). The frequency flavor expresses a listening goal that steers
# the free-state proposal toward the admitted static_eq domain; the compression
# flavor steers toward the admitted broadband_compression domain the same way.
# The leveling flavor (D2-1.5-S2f-2) states the opposite compression premise --
# dynamically loose material that needs bounded leveling -- so the physically
# reachable direction (lowering a fresh compressor threshold) is the one the
# proposal follows.
PROMPT_FLAVORS = {
    "neutral": OPEN_PROMPT,
    "frequency": "人声在 200-400Hz 听起来浑浊（boxy），但各轨电平平衡已经合适，不要用整体增益来解决。请先观察工程，再针对这个频段给一个有界的小步改进建议。",
    "compression": "整轨动态听起来被压得过平（over-compressed），但各轨电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个动态问题给一个有界的小步改进建议。",
    "leveling": "有几轨的动态起伏偏大、峰值偶尔跳出来，但各轨电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个动态问题给一个有界的小步改进建议。",
    # FAM1-S1 GLM 裁定②：Pro-DS threshold 新实例默认居中（normalized 0.4），
    # 双向物理可达，前提只描述问题（咝声/齿音突出）不指定参数方向；
    # 四要素结构与 frequency/compression/leveling 同构（前提/禁手段/先观察/有界小步），
    # case-agnostic，不编码 sealed 真值。
    "sibilance": "人声的高频咝声（齿音）有些刺耳、比较突出，但各轨电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个齿音问题给一个有界的小步改进建议。",
    # FAM2-S2 同款裁定②形态：SPL TD+ attack 新实例默认严格居中（normalized 0.5 /
    # display "0.00"，FAM2-S1 probe 定锚）→ 双向物理可达，前提只描述问题（起音偏钝、
    # 瞬态对比不足）不指定参数方向；四要素同构，case-agnostic，零 sealed 真值
    # （目标轨由模型观察自主选择，盲法保持）。
    "transient": "鼓和打击乐的起音听起来偏钝、瞬态对比不足，但各轨电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个起音问题给一个有界的小步改进建议。",
    # FAM3-S2：前提与 p06 烘焙 fault 对齐（单一轨道 L/R 增益倾斜，生成机第 8 配方
    # channel_balance_shift，方向由 sealed 持有）。措辞不指定轨名与方向——模型经
    # track.stereo_space 观察自选目标与方向；delta_pan 双向物理可达；四要素同构，
    # case-agnostic，零 sealed 真值。
    "pan": "有一条轨的声像明显偏向一侧、立体声左右平衡听起来不自然，但各轨电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个声像问题给一个有界的小步改进建议。",
    # FAM4-S2：前提与 p02 烘焙 fault 对齐（稀疏峰值超限，i05，方向由 sealed
    # 持有）。措辞不指定轨名与方向——模型经 track.peak_structure 观察自选目标；
    # Pro-L 2 Output Level（ceiling）新实例默认顶格（normalized 1.0/display
    # "0.00 dBTP"，FAM4-S1 probe 定锚），物理可达方向向下，但前提只描述问题
    # （偶发尖峰超顶）不指定参数方向；四要素同构，case-agnostic，零 sealed 真值。
    "limiter": "有几轨偶尔冒出零星的尖峰、峰值听起来超出了混音的顶部边界，但各轨整体电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个峰值问题给一个有界的小步改进建议。",
    # FAM5-S2：前提与 p02 烘焙 fault 对齐（低段噪声床，i06，方向由 sealed
    # 持有）。措辞不指定轨名与方向——模型经 track.activity_structure 观察自选
    # 目标；Pro-G Range（attenuation floor）新实例默认顶格（normalized 1.0/
    # display "50.00 dB"，FAM5-S1 probe 定锚），前提只描述问题（安静段有可闻
    # 噪声床）不指定参数方向；四要素同构，case-agnostic，零 sealed 真值。
    "gate": "有几轨在乐句之间的安静段落能听到明显的低电平噪声床，但各轨整体电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个安静段噪声问题给一个有界的小步改进建议。",
    # FAM6-S2：前提与 p02 烘焙 fault 对齐（频段限定的时变电平起伏，i07，
    # 方向与具体频段数值由 sealed 持有）。措辞不指定轨名、频段数值与
    # 方向——频段语义由模型经 track.band_dynamics 观察得出；Lindell MBC
    # band Threshold 新实例默认严格居中（normalized 0.5，FAM6-S1 probe
    # 定锚）双向物理可达，前提只描述问题（低频段电平随时间起伏明显）
    # 不指定参数方向；四要素同构，case-agnostic，零 sealed 真值。
    "multiband": "有一条轨的低频段电平随时间起伏明显、这个频段内的动态听起来忽紧忽松，但各轨整体电平平衡已经合适，不要用整体增益或 EQ 来解决。请先观察工程，再针对这个频段动态问题给一个有界的小步改进建议。",
}
# The D2-1 domain table mirrored for runner-side gating. Admission itself is
# always decided by the agent's experiment domain table, never here.
ADMITTED_DOMAIN_KINDS = {
    "track_gain": "track_gain_adjust",
    "static_eq": "static_eq_band_adjust",
    "broadband_compression": "broadband_threshold_adjust",
    "de_esser": "de_esser_threshold_adjust",
    "transient_shaper": "transient_attack_adjust",
    "pan": "track_pan_adjust",
    "limiter": "limiter_ceiling_adjust",
    "gate_expander": "gate_range_adjust",
    "multiband_dynamics": "multiband_band_threshold_adjust",
}
NOT_EXERCISED_EXIT = 3
ACTIVE_CONTINUATION_STATUSES = {"pending", "claimed", "running"}
# Generic mix suggestions are chain artifacts of the reobserve loop, not part
# of the D1 experiment flow: approving them spends one durable continuation
# each and drifts the project revision the experiment receipt is bound to.
GENERIC_MIX_CONFIRMATION_KINDS = {"mix_tick_confirmation", "mix_treatment_confirmation"}
EXPERIMENT_FLOW_MARKERS = ("experiment", "judgment", "audition", "settlement")
# The experiment action confirmation surfaces on the mix-tick surface and is
# confirmed twice (processor selection, then execution): every passing trace
# approves the same interaction id at the pre-drain and post-drain positions.
MAX_EXPERIMENT_TICK_CONFIRMATIONS = 2
# After the receipt lands the goal parks in waiting_continue; the designed
# resume path is a user turn. A bounded neutral "continue" nudge stands in for
# that user without fabricating any judgment input.
MAX_CONTINUE_NUDGES = 6
CONTINUE_NUDGE_MESSAGE = "继续"
FORBIDDEN_SELECTION_KEYS = {
    "selected_track_id",
    "selected_track_name",
    "selected_clip_id",
    "selected_clip_ids",
    "selected_clip_track_id",
    "selected_clip_ranges",
    "selected_clip_range",
}


class SchedulerDrainTimeout(RuntimeError):
    def __init__(self, message: str, timeline: list[dict[str, Any]], runtime_status: dict[str, Any], continuations: list[dict[str, Any]]):
        super().__init__(message)
        self.timeline = timeline
        self.runtime_status = runtime_status
        self.continuations = continuations


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            value = json.loads(response.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as http_error:
        body = ""
        try:
            body = http_error.read().decode("utf-8", errors="replace")[:500]
        except Exception:  # noqa: BLE001 - body is diagnostic only.
            pass
        raise RuntimeError(f"{method} {url} -> HTTP {http_error.code}: {body}") from http_error
    if not isinstance(value, dict):
        raise RuntimeError(f"non-object response from {url}")
    return value


def invoke(base_url: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    envelope = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "source": "free_state_d1_smoke.setup", "confirmed": confirmed},
        timeout,
    )
    if str(envelope.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(envelope, ensure_ascii=False)[:2000]}")
    result = envelope.get("result", envelope)
    if not isinstance(result, dict):
        raise RuntimeError(f"{tool} returned no object result")
    return result


def load_public_case(manifest_path: Path, case_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    resolved = manifest_path.resolve()
    if "sealed" in {part.lower() for part in resolved.parts}:
        raise ValueError("public manifest path must not contain a sealed segment")
    manifest = json.loads(resolved.read_text(encoding="utf-8-sig"))
    if not isinstance(manifest, dict) or manifest.get("schema_version") != PUBLIC_SCHEMA:
        raise ValueError("public manifest schema mismatch")
    blindness = manifest.get("blindness_contract")
    required = (
        "sealed_truth_not_in_agent_context",
        "case_ids_are_semantically_opaque",
        "runner_reads_public_manifest_only",
        "same_neutral_prompt_for_all_projects",
        "preselected_track_forbidden",
        "preselected_plugin_forbidden",
    )
    if not isinstance(blindness, dict) or any(blindness.get(key) is not True for key in required):
        raise ValueError("public manifest blindness contract is incomplete")
    cases = [row for row in manifest.get("cases", []) if isinstance(row, dict) and row.get("public_case_id") == case_id]
    if len(cases) != 1:
        raise ValueError(f"public manifest must contain exactly one requested case {case_id}")
    case = cases[0]
    project_path = Path(str(case.get("project_path", ""))).resolve()
    if "sealed" in {part.lower() for part in project_path.parts}:
        raise ValueError("public project path must not contain a sealed segment")
    if not project_path.is_file():
        raise ValueError(f"public project is missing: {project_path}")
    return manifest, case


def materialize_public_case(case: dict[str, Any], workdir: Path) -> dict[str, Any]:
    source_project = Path(str(case["project_path"])).resolve()
    destination = workdir.resolve()
    if destination.exists():
        raise ValueError(f"D1 project workdir already exists: {destination}")
    shutil.copytree(source_project.parent, destination, ignore=shutil.ignore_patterns(".vit_history"))
    copied_project = destination / source_project.name
    if not copied_project.is_file():
        raise RuntimeError(f"copied public project is missing: {copied_project}")
    copied_case = dict(case)
    copied_case["project_path"] = str(copied_project)
    return copied_case


# Runner-side material qualification for the compression-positive fixture
# (D2-1.5-S2f-2). The gates mirror the smoke contract's context-validity style:
# a per-track RMS floor plus crest-factor floors proving the public material is
# dynamically loose enough that bounded compression is acoustically meaningful
# (spv1 stems measure crest 14.5-21.0 dB; the floors keep real headroom instead
# of pinning the measured values). Metrics are machine-computed from the public
# stems only, land in the smoke report (never in agent context), and name no
# target track, so the blindness contract stays mechanically auditable.
MATERIAL_QUALIFICATION_SCHEMA = "vit.free_state_d1_material_qualification.v1"
MATERIAL_MIN_RMS_DBFS = -45.0
MATERIAL_MIN_CREST_DB_PER_TRACK = 10.0
MATERIAL_MIN_BEST_CREST_DB = 14.0


def dbfs(value: float) -> float:
    return 20.0 * math.log10(max(value, 1e-12))


def qualify_material(case: dict[str, Any], stereo_balance: bool = False, limiter: bool = False, gate: bool = False, multiband: bool = False) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, _rate = sf.read(path, dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        peak = float(np.max(np.abs(mono)))
        rms = float(np.sqrt(np.mean(mono * mono)))
        crest_db = dbfs(peak / max(rms, 1e-12)) if peak > 0 and rms > 0 else 0.0
        track_rows.append({
            "track": str(stem["track"]),
            "file": str(path),
            "rms_dbfs": round(dbfs(rms), 3),
            "peak_dbfs": round(dbfs(peak), 3),
            "crest_db": round(crest_db, 3),
        })
    if not track_rows:
        raise RuntimeError("material qualification found no public stems")
    weak_rms = [row["track"] for row in track_rows if row["rms_dbfs"] <= MATERIAL_MIN_RMS_DBFS]
    weak_crest = [row["track"] for row in track_rows if row["crest_db"] < MATERIAL_MIN_CREST_DB_PER_TRACK]
    best_crest_db = max(row["crest_db"] for row in track_rows)
    if weak_rms or weak_crest or best_crest_db < MATERIAL_MIN_BEST_CREST_DB:
        raise RuntimeError(
            "public material failed the compression-fixture qualification gates: "
            + json.dumps({"weak_rms_tracks": weak_rms, "weak_crest_tracks": weak_crest, "best_crest_db": best_crest_db}, ensure_ascii=False)
        )
    sibilance = qualify_sibilance_material(case)
    transient = qualify_transient_material(case)
    result = {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "gates": {
            "min_rms_dbfs_per_track": MATERIAL_MIN_RMS_DBFS,
            "min_crest_db_per_track": MATERIAL_MIN_CREST_DB_PER_TRACK,
            "min_best_crest_db": MATERIAL_MIN_BEST_CREST_DB,
        },
        "best_crest_db": round(best_crest_db, 3),
        "tracks": track_rows,
        "sibilance_band": sibilance,
        "transient_window": transient,
    }
    # FAM3-S2: the stereo-balance gate is flavor-scoped, unlike the crest /
    # sibilance / transient gates above, because those describe properties
    # every spv1 material set carries while a clearly off-center track exists
    # only in the p06 material (the shared p01 stems measure |balance| <= 1.5
    # dB, so an unconditional best-track imbalance floor would fail every
    # earlier case).
    if stereo_balance:
        result["stereo_balance_window"] = qualify_stereo_balance_material(case)
    # FAM4-S2: the sparse-peak gate is flavor-scoped for the same reason as
    # the stereo-balance gate above -- the property is characteristic of the
    # baked p02 material rather than every spv1 set.
    if limiter:
        result["sparse_peak_window"] = qualify_limiter_material(case)
    # FAM5-S2: the quiet/active contrast gate is flavor-scoped for the same
    # reason as the sparse-peak gate above.
    if gate:
        result["quiet_active_window"] = qualify_gate_material(case)
    # FAM6-S2: the band-limited time-contrast gate is flavor-scoped for the
    # same reason as the quiet/active gate above.
    if multiband:
        result["band_contrast_window"] = qualify_multiband_material(case)
    return result


# Sibilance-domain material qualification (D2-FAM1-S2). The public fixture
# stems carry time-localized high-band energy events, so a windowed
# high-band/full-band energy-ratio series (4.8-10.5 kHz against the full
# spectrum) must disperse enough per track for a bounded de-esser move to be
# acoustically meaningful. Same gate style as the crest family above: a
# per-track dispersion floor plus a best-track floor with real headroom below
# the measured values (spv1 stems measure per-track 7.6-19.6 dB with the best
# track at 19.6 dB; floors stay clear of pinning the measurements). Metrics
# are machine-computed from the public stems only, land in the smoke report
# (never in agent context), and name no target track.
SIBILANCE_MIN_CONTRAST_DB_PER_TRACK = 6.0
SIBILANCE_MIN_BEST_CONTRAST_DB = 14.0
SIBILANCE_BAND_LOW_HZ = 4800.0
SIBILANCE_BAND_HIGH_HZ = 10500.0
SIBILANCE_FFT_SIZE = 2048


def qualify_sibilance_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        window = np.hanning(SIBILANCE_FFT_SIZE)
        hop = SIBILANCE_FFT_SIZE // 2
        spectrum = np.fft.rfft
        freqs = np.fft.rfftfreq(SIBILANCE_FFT_SIZE, 1.0 / rate)
        band = (freqs >= SIBILANCE_BAND_LOW_HZ) & (freqs <= SIBILANCE_BAND_HIGH_HZ)
        count = max(0, (len(mono) - SIBILANCE_FFT_SIZE) // hop + 1)
        if count <= 0:
            raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
        ratios = np.empty(count)
        for index in range(count):
            segment = mono[index * hop:index * hop + SIBILANCE_FFT_SIZE] * window
            power = np.abs(spectrum(segment)) ** 2
            total = float(power.sum())
            ratios[index] = 10.0 * math.log10(max(float(power[band].sum()), 1e-20) / max(total, 1e-20))
        p50 = float(np.percentile(ratios, 50))
        p95 = float(np.percentile(ratios, 95))
        track_rows.append({
            "track": str(stem["track"]),
            "contrast_p95_p50_db": round(p95 - p50, 3),
            "p50_db": round(p50, 3),
            "p95_db": round(p95, 3),
        })
    weak_contrast = [row["track"] for row in track_rows if row["contrast_p95_p50_db"] < SIBILANCE_MIN_CONTRAST_DB_PER_TRACK]
    best_contrast_db = max(row["contrast_p95_p50_db"] for row in track_rows)
    if weak_contrast or best_contrast_db < SIBILANCE_MIN_BEST_CONTRAST_DB:
        raise RuntimeError(
            "public material failed the sibilance-fixture qualification gates: "
            + json.dumps({"weak_contrast_tracks": weak_contrast, "best_contrast_p95_p50_db": best_contrast_db}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "windowed high-band/full-band energy ratio dispersion (p95-p50, dB)",
        "gates": {
            "band_low_hz": SIBILANCE_BAND_LOW_HZ,
            "band_high_hz": SIBILANCE_BAND_HIGH_HZ,
            "fft_size": SIBILANCE_FFT_SIZE,
            "min_contrast_db_per_track": SIBILANCE_MIN_CONTRAST_DB_PER_TRACK,
            "min_best_contrast_db": SIBILANCE_MIN_BEST_CONTRAST_DB,
        },
        "best_contrast_p95_p50_db": round(best_contrast_db, 3),
        "tracks": track_rows,
    }


# Transient-domain material qualification (D2-FAM2-S2). The public fixture
# stems carry short energy events over a slower background, so a short-window
# vs long-window RMS level contrast (~10 ms window peak against ~200 ms window
# median, a pure windowed-arithmetic quantity from the same measurement family
# as the sealed i04 anchor) must stay measurable per track for a bounded
# transient-shaper move to be acoustically meaningful. Same gate style as the
# sibilance family: a per-track floor plus a best-track floor with real
# headroom below the measured values (spv1 stems measure per-track 7.4-19.5 dB
# with the best track at 19.5 dB; floors stay clear of pinning the
# measurements). Metrics are machine-computed from the public stems only, land
# in the smoke report (never in agent context), and name no target track.
TRANSIENT_MIN_CONTRAST_DB_PER_TRACK = 6.0
TRANSIENT_MIN_BEST_CONTRAST_DB = 14.0
TRANSIENT_SHORT_WINDOW_SECONDS = 0.010
TRANSIENT_LONG_WINDOW_SECONDS = 0.200


def qualify_transient_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        short_n = max(1, int(round(TRANSIENT_SHORT_WINDOW_SECONDS * rate)))
        long_n = max(1, int(round(TRANSIENT_LONG_WINDOW_SECONDS * rate)))

        def window_level_db(size: int) -> np.ndarray:
            count = max(0, (len(mono) - size) // size + 1)
            if count <= 0:
                raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
            levels = np.empty(count)
            for index in range(count):
                segment = mono[index * size:index * size + size]
                levels[index] = 20.0 * math.log10(max(float(np.sqrt(np.mean(segment * segment))), 1e-12))
            return levels

        short_levels = window_level_db(short_n)
        long_levels = window_level_db(long_n)
        contrast_db = float(np.max(short_levels) - np.median(long_levels))
        track_rows.append({
            "track": str(stem["track"]),
            "contrast_short_peak_long_median_db": round(contrast_db, 3),
            "short_peak_db": round(float(np.max(short_levels)), 3),
            "long_median_db": round(float(np.median(long_levels)), 3),
        })
    weak_contrast = [row["track"] for row in track_rows if row["contrast_short_peak_long_median_db"] < TRANSIENT_MIN_CONTRAST_DB_PER_TRACK]
    best_contrast_db = max(row["contrast_short_peak_long_median_db"] for row in track_rows)
    if weak_contrast or best_contrast_db < TRANSIENT_MIN_BEST_CONTRAST_DB:
        raise RuntimeError(
            "public material failed the transient-fixture qualification gates: "
            + json.dumps({"weak_contrast_tracks": weak_contrast, "best_contrast_short_peak_long_median_db": best_contrast_db}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "short-window peak vs long-window median RMS level (dB)",
        "gates": {
            "short_window_seconds": TRANSIENT_SHORT_WINDOW_SECONDS,
            "long_window_seconds": TRANSIENT_LONG_WINDOW_SECONDS,
            "min_contrast_db_per_track": TRANSIENT_MIN_CONTRAST_DB_PER_TRACK,
            "min_best_contrast_db": TRANSIENT_MIN_BEST_CONTRAST_DB,
        },
        "best_contrast_short_peak_long_median_db": round(best_contrast_db, 3),
        "tracks": track_rows,
    }


# Stereo-balance-domain material qualification (D2-FAM3-S2). The public p06
# fixture stems carry one clearly off-center track (the baked L/R gain tilt),
# so a windowed L/R RMS difference series (200 ms non-overlapping windows,
# 20*log10(RMS_R/RMS_L) — the same caliber as the kernel balance_db) must show
# exactly one strongly off-center track for a bounded pan move to be
# acoustically meaningful. The gate pair is a best-track floor plus a
# localization cap on the second-strongest track instead of the sibilance /
# transient per-track floor form: the measured material legitimately contains
# near-mono stems (bass median windowed |balance| ~0.02 dB), so a per-track
# imbalance floor would be vacuous, while "best >= floor AND second <= cap"
# expresses the actual pan-relevant fact — some track is clearly off-side and
# the imbalance is localized to it. Floors leave real headroom under the
# measured values (best 5.99 dB, second 0.74 dB); metrics are machine-computed
# from the public stems only, land in the smoke report (never in agent
# context), and name no target track.
STEREO_BALANCE_MIN_BEST_ABS_MEDIAN_DB = 4.0
STEREO_BALANCE_MAX_SECOND_ABS_MEDIAN_DB = 2.0
STEREO_BALANCE_WINDOW_SECONDS = 0.200


# Limiter-domain material qualification (D2-FAM4-S2). The public fixture
# stems carry sparse short-window peak outliers over a steadier peak
# population, so the top short-window peak must stand clearly above the
# typical (95th-percentile) short-window peak (a pure windowed-arithmetic
# quantity from the same measurement family as the sealed i05 anchor's
# sample-peak/crest contrast) for a bounded ceiling move to be acoustically
# meaningful. Flavor-scoped like the stereo-balance gate (D2-FAM3-S2
# precedent): a per-track floor plus a best-track floor with real headroom
# below the measured values (p02 stems measure per-track 2.2-9.0 dB with the
# best track at 9.0 dB; floors stay clear of pinning the measurements).
# Metrics are machine-computed from the public stems only, land in the smoke
# report (never in agent context), and name no target track.
LIMITER_MIN_PROMINENCE_DB_PER_TRACK = 1.5
LIMITER_MIN_BEST_PROMINENCE_DB = 6.0
LIMITER_SHORT_WINDOW_SECONDS = 0.010


def qualify_limiter_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        short_n = max(1, int(round(LIMITER_SHORT_WINDOW_SECONDS * rate)))
        count = max(0, (len(mono) - short_n) // short_n + 1)
        if count <= 0:
            raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
        window_peaks = np.empty(count)
        for index in range(count):
            segment = mono[index * short_n:index * short_n + short_n]
            window_peaks[index] = float(np.max(np.abs(segment)))
        prominence_db = 20.0 * math.log10(float(np.max(window_peaks)) / max(float(np.percentile(window_peaks, 95)), 1e-12))
        track_rows.append({
            "track": str(stem["track"]),
            "sparse_peak_prominence_db": round(prominence_db, 3),
            "sample_peak_dbfs": round(20.0 * math.log10(max(float(np.max(np.abs(mono))), 1e-12)), 3),
        })
    weak = [row["track"] for row in track_rows if row["sparse_peak_prominence_db"] < LIMITER_MIN_PROMINENCE_DB_PER_TRACK]
    best_prominence_db = max(row["sparse_peak_prominence_db"] for row in track_rows)
    if weak or best_prominence_db < LIMITER_MIN_BEST_PROMINENCE_DB:
        raise RuntimeError(
            "public material failed the limiter-fixture qualification gates: "
            + json.dumps({"weak_prominence_tracks": weak, "best_sparse_peak_prominence_db": best_prominence_db}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "top short-window peak vs 95th-percentile short-window peak (dB)",
        "gates": {
            "short_window_seconds": LIMITER_SHORT_WINDOW_SECONDS,
            "min_prominence_db_per_track": LIMITER_MIN_PROMINENCE_DB_PER_TRACK,
            "min_best_prominence_db": LIMITER_MIN_BEST_PROMINENCE_DB,
        },
        "best_sparse_peak_prominence_db": round(best_prominence_db, 3),
        "tracks": track_rows,
    }


# Gate/expander-domain material qualification (D2-FAM5-S2). The public
# fixture stems carry clearly separated quiet and active intervals, so a
# long-window RMS percentile contrast (200 ms windows, 15th vs 85th
# percentile -- a pure windowed-arithmetic quantity from the same
# measurement family as the sealed i06 anchor's quiet/active interval
# medians) must stay measurable per track for a bounded attenuation-floor
# move to be acoustically meaningful. Flavor-scoped like the sparse-peak
# gate (D2-FAM4-S2 precedent): a per-track floor plus a best-track floor
# with real headroom below the measured values (p02 stems measure per-track
# 45.6-66.1 dB with the best track at 66.1 dB; floors stay clear of pinning
# the measurements). Metrics are machine-computed from the public stems
# only, land in the smoke report (never in agent context), and name no
# target track.
GATE_MIN_CONTRAST_DB_PER_TRACK = 30.0
GATE_MIN_BEST_CONTRAST_DB = 50.0
GATE_WINDOW_SECONDS = 0.200


def qualify_gate_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        window_n = max(1, int(round(GATE_WINDOW_SECONDS * rate)))
        count = max(0, (len(mono) - window_n) // window_n + 1)
        if count <= 0:
            raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
        levels = np.empty(count)
        for index in range(count):
            segment = mono[index * window_n:index * window_n + window_n]
            levels[index] = 20.0 * math.log10(max(float(np.sqrt(np.mean(segment * segment))), 1e-12))
        contrast_db = float(np.percentile(levels, 85) - np.percentile(levels, 15))
        track_rows.append({
            "track": str(stem["track"]),
            "quiet_active_contrast_db": round(contrast_db, 3),
            "quiet_p15_db": round(float(np.percentile(levels, 15)), 3),
            "active_p85_db": round(float(np.percentile(levels, 85)), 3),
        })
    weak = [row["track"] for row in track_rows if row["quiet_active_contrast_db"] < GATE_MIN_CONTRAST_DB_PER_TRACK]
    best_contrast_db = max(row["quiet_active_contrast_db"] for row in track_rows)
    if weak or best_contrast_db < GATE_MIN_BEST_CONTRAST_DB:
        raise RuntimeError(
            "public material failed the gate-fixture qualification gates: "
            + json.dumps({"weak_contrast_tracks": weak, "best_quiet_active_contrast_db": best_contrast_db}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "long-window RMS 15th vs 85th percentile contrast (dB)",
        "gates": {
            "window_seconds": GATE_WINDOW_SECONDS,
            "min_contrast_db_per_track": GATE_MIN_CONTRAST_DB_PER_TRACK,
            "min_best_contrast_db": GATE_MIN_BEST_CONTRAST_DB,
        },
        "best_quiet_active_contrast_db": round(best_contrast_db, 3),
        "tracks": track_rows,
    }


# Multiband-domain material qualification (D2-FAM6-S2). The public fixture
# stems carry band-limited time-varying level motion in the low band, so the
# windowed band-level dynamic range (200 ms non-overlapping windows, per-band
# in-window FFT energy, 90th minus 10th percentile -- a pure windowed
# arithmetic quantity from the same measurement family as the sealed i07
# anchor's band p90-p10 ranges) must stand above the same quantity measured
# in a fixed high reference band for a bounded band-threshold move to be
# acoustically meaningful. Two robustness constraints keep the quantity
# meaningful: only the track's active windows count (the quietest 35% of
# full-band windows are dropped, mirroring the quiet/active split caliber),
# and band levels are floored 60 dB under the active full-band 10th
# percentile so near-empty bands cannot manufacture floor-to-content
# pseudo-range. Blind-form discipline differs from the FAM1-S2 sibilance
# precedent on purpose: no baked fault band edges are encoded -- the target
# band is found by scanning a generic one-third-octave low-band bank (ISO
# centers 40-315 Hz) and keeping the band with the highest range, so which
# band wins is itself measured from the public stems (the measured best
# track is not the sealed target track, same blind-form note as the
# D2-FAM5-S2 gate qualifier). Flavor-scoped like the quiet/active gate
# (D2-FAM5-S2 precedent): a per-track floor plus a best-track floor with
# real headroom below the measured values (p02 stems measure per-track
# deltas 10.0-28.3 dB with the best track at 28.3 dB; floors stay clear of
# pinning the measurements). Metrics are machine-computed from the public
# stems only, land in the smoke report (never in agent context), and name no
# target track.
MULTIBAND_WINDOW_SECONDS = 0.200
MULTIBAND_BAND_CENTERS_HZ = (40.0, 50.0, 63.0, 80.0, 100.0, 125.0, 160.0, 200.0, 250.0, 315.0)
MULTIBAND_OCTAVE_FRACTION = 3.0
MULTIBAND_REFERENCE_LOW_HZ = 1000.0
MULTIBAND_REFERENCE_HIGH_HZ = 8000.0
MULTIBAND_ACTIVE_WINDOW_DROP_PERCENTILE = 35.0
MULTIBAND_BAND_FLOOR_OFFSET_DB = 60.0
MULTIBAND_MIN_DELTA_DB_PER_TRACK = 6.0
MULTIBAND_MIN_BEST_DELTA_DB = 12.0


def _band_edges(center_hz: float) -> tuple[float, float]:
    ratio = 2.0 ** (1.0 / (2.0 * MULTIBAND_OCTAVE_FRACTION))
    return center_hz / ratio, center_hz * ratio


def qualify_multiband_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        mono = np.asarray(audio, dtype=np.float64).mean(axis=1)
        window_n = max(1, int(round(MULTIBAND_WINDOW_SECONDS * rate)))
        count = max(0, (len(mono) - window_n) // window_n + 1)
        if count <= 0:
            raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
        window = np.hanning(window_n)
        freqs = np.fft.rfftfreq(window_n, 1.0 / rate)
        bank_masks = [(center, (freqs >= low) & (freqs <= high)) for center, (low, high) in
                      ((center, _band_edges(center)) for center in MULTIBAND_BAND_CENTERS_HZ)]
        reference_mask = (freqs >= MULTIBAND_REFERENCE_LOW_HZ) & (freqs <= MULTIBAND_REFERENCE_HIGH_HZ)
        spectra = np.empty((count, len(freqs)))
        full_band_db = np.empty(count)
        for index in range(count):
            segment = mono[index * window_n:index * window_n + window_n]
            spectra[index] = np.abs(np.fft.rfft(segment * window)) ** 2
            full_band_db[index] = 20.0 * math.log10(max(float(np.sqrt(np.mean(segment * segment))), 1e-12))
        active = full_band_db >= np.percentile(full_band_db, MULTIBAND_ACTIVE_WINDOW_DROP_PERCENTILE)
        band_floor_db = float(np.percentile(full_band_db[active], 10)) - MULTIBAND_BAND_FLOOR_OFFSET_DB

        def band_range_db(mask: np.ndarray) -> float:
            levels = 10.0 * np.log10(np.maximum(spectra[:, mask].sum(axis=1), 1e-20))
            levels = np.maximum(levels, band_floor_db)[active]
            return float(np.percentile(levels, 90) - np.percentile(levels, 10))

        bank_rows = sorted(
            ({"center_hz": center, "band_range_db": round(band_range_db(mask), 3)} for center, mask in bank_masks),
            key=lambda row: row["band_range_db"],
            reverse=True,
        )
        reference_range_db = band_range_db(reference_mask)
        track_rows.append({
            "track": str(stem["track"]),
            "active_window_count": int(np.count_nonzero(active)),
            "best_band_center_hz": bank_rows[0]["center_hz"],
            "best_band_range_db": bank_rows[0]["band_range_db"],
            "second_band_range_db": bank_rows[1]["band_range_db"],
            "reference_band_range_db": round(reference_range_db, 3),
            "contrast_delta_db": round(bank_rows[0]["band_range_db"] - reference_range_db, 3),
            "band_bank": bank_rows,
        })
    weak = [row["track"] for row in track_rows if row["contrast_delta_db"] < MULTIBAND_MIN_DELTA_DB_PER_TRACK]
    best_delta_db = max(row["contrast_delta_db"] for row in track_rows)
    if weak or best_delta_db < MULTIBAND_MIN_BEST_DELTA_DB:
        raise RuntimeError(
            "public material failed the multiband-fixture qualification gates: "
            + json.dumps({"weak_delta_tracks": weak, "best_contrast_delta_db": best_delta_db}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "active-window best low-bank band-level p90-p10 range minus fixed high reference band same quantity (dB)",
        "gates": {
            "window_seconds": MULTIBAND_WINDOW_SECONDS,
            "band_centers_hz": list(MULTIBAND_BAND_CENTERS_HZ),
            "octave_fraction": MULTIBAND_OCTAVE_FRACTION,
            "reference_band_hz": [MULTIBAND_REFERENCE_LOW_HZ, MULTIBAND_REFERENCE_HIGH_HZ],
            "active_window_drop_percentile": MULTIBAND_ACTIVE_WINDOW_DROP_PERCENTILE,
            "band_floor_offset_db": MULTIBAND_BAND_FLOOR_OFFSET_DB,
            "min_delta_db_per_track": MULTIBAND_MIN_DELTA_DB_PER_TRACK,
            "min_best_delta_db": MULTIBAND_MIN_BEST_DELTA_DB,
        },
        "best_contrast_delta_db": round(best_delta_db, 3),
        "tracks": track_rows,
    }


def qualify_stereo_balance_material(case: dict[str, Any]) -> dict[str, Any]:
    import numpy as np
    import soundfile as sf

    track_rows: list[dict[str, Any]] = []
    for stem in case.get("stem_files", []):
        path = Path(str(stem["file"]))
        if not path.is_file():
            raise RuntimeError(f"material qualification stem is missing: {path}")
        audio, rate = sf.read(str(path), dtype="float64", always_2d=True)
        audio = np.asarray(audio, dtype=np.float64)
        size = max(1, int(round(STEREO_BALANCE_WINDOW_SECONDS * rate)))
        count = (len(audio) - size) // size + 1
        if count <= 0:
            raise RuntimeError(f"material qualification stem is shorter than one analysis window: {path}")
        blocks = audio[: count * size].reshape(count, size, 2)
        left = np.sqrt(np.mean(blocks[:, :, 0] * blocks[:, :, 0], axis=1))
        right = np.sqrt(np.mean(blocks[:, :, 1] * blocks[:, :, 1], axis=1))
        balance = 20.0 * np.log10(np.maximum(right, 1e-12) / np.maximum(left, 1e-12))
        track_rows.append({
            "track": str(stem["track"]),
            "balance_median_db": round(float(np.median(balance)), 3),
            "abs_balance_median_db": round(abs(float(np.median(balance))), 3),
        })
    magnitudes = sorted((row["abs_balance_median_db"] for row in track_rows), reverse=True)
    best = magnitudes[0]
    second = magnitudes[1] if len(magnitudes) > 1 else 0.0
    if best < STEREO_BALANCE_MIN_BEST_ABS_MEDIAN_DB or second > STEREO_BALANCE_MAX_SECOND_ABS_MEDIAN_DB:
        raise RuntimeError(
            "public material failed the stereo-balance-fixture qualification gates: "
            + json.dumps({"best_abs_balance_median_db": best, "second_abs_balance_median_db": second}, ensure_ascii=False)
        )
    return {
        "schema_version": MATERIAL_QUALIFICATION_SCHEMA,
        "status": "passed",
        "metric": "windowed L/R RMS balance median (dB, RMS_R - RMS_L caliber)",
        "gates": {
            "window_seconds": STEREO_BALANCE_WINDOW_SECONDS,
            "min_best_abs_balance_median_db": STEREO_BALANCE_MIN_BEST_ABS_MEDIAN_DB,
            "max_second_abs_balance_median_db": STEREO_BALANCE_MAX_SECOND_ABS_MEDIAN_DB,
        },
        "best_abs_balance_median_db": round(best, 3),
        "second_abs_balance_median_db": round(second, 3),
        "tracks": track_rows,
    }


def first_text(*values: Any) -> str:
    for value in values:
        if value is None:
            continue
        text = str(value).strip()
        if text and text != "<nil>":
            return text
    return ""


def rows(value: Any) -> list[dict[str, Any]]:
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def walk(value: Any):
    yield value
    if isinstance(value, dict):
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def dicts(value: Any) -> list[dict[str, Any]]:
    return [item for item in walk(value) if isinstance(item, dict)]


def values_for_key(value: Any, key: str) -> list[Any]:
    return [item[key] for item in dicts(value) if key in item]


def project_path_from_state(state: dict[str, Any]) -> str:
    for item in dicts(state):
        path = first_text(item.get("project_path"), item.get("current_project_path"))
        if path:
            return str(Path(path).resolve())
    return ""


def project_revision(state: dict[str, Any]) -> str:
    for key in ("project_revision", "revision"):
        for value in values_for_key(state, key):
            text = first_text(value)
            if text.isdigit():
                return text
    return ""


def continuation_rows(base_url: str, conversation_id: str, timeout: float) -> list[dict[str, Any]]:
    status = request_json("GET", base_url.rstrip("/") + "/agent/runtime/status", None, timeout)
    return [item for item in rows(status.get("continuations")) if first_text(item.get("conversation_id")) == conversation_id]


def wait_scheduler_drain(base_url: str, conversation_id: str, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    latest: list[dict[str, Any]] = []
    latest_status: dict[str, Any] = {}
    timeline: list[dict[str, Any]] = []
    seen_continuation = False
    while time.monotonic() < deadline:
        latest_status = request_json("GET", base_url.rstrip("/") + "/agent/runtime/status", None, min(timeout, 30))
        latest = [item for item in rows(latest_status.get("continuations")) if first_text(item.get("conversation_id")) == conversation_id]
        timeline.append({
            "observed_at": dt.datetime.now(dt.timezone.utc).isoformat(),
            "continuations": [
                {key: item.get(key) for key in ("continuation_id", "status", "attempt", "updated_at", "last_error", "free_state_status", "free_state_current_phase", "free_state_continuation_budget", "free_state_continuation_used", "free_state_decision_status", "free_state_stop_reason") if key in item}
                for item in latest
            ],
        })
        if latest:
            seen_continuation = True
        active = [item for item in latest if first_text(item.get("status")).lower() in ACTIVE_CONTINUATION_STATUSES]
        if not active and seen_continuation:
            causes = [first_text(item.get("free_state_stop_reason"), item.get("last_error"), item.get("status")) for item in latest]
            return {"continuations": latest, "timeline": timeline, "runtime_status": latest_status, "terminal_causes": [cause for cause in causes if cause]}
        time.sleep(2)
    reason = "durable_continuation_missing_after_waiting_continue" if not seen_continuation else "D1 durable continuation did not drain within the smoke timeout"
    raise SchedulerDrainTimeout(reason, timeline, latest_status, latest)


def continuation_interaction_requests(continuations: list[dict[str, Any]]) -> list[dict[str, Any]]:
    """Project a durable waiting interaction back into the chat response shape.

    Scheduler-drained responses historically retained only the free-state loop,
    which made the smoke skip the second mix-tick confirmation and incorrectly
    assert mutation cardinality before the approved action could run.
    """
    requests: list[dict[str, Any]] = []
    for item in continuations:
        pending = item.get("pending_interaction")
        if not isinstance(pending, dict):
            continue
        raw_requests = pending.get("requests")
        candidates = raw_requests if isinstance(raw_requests, list) else [pending]
        for candidate in candidates:
            if not isinstance(candidate, dict):
                continue
            request = dict(candidate)
            if not first_text(request.get("id"), request.get("interaction_id")):
                interaction_id = first_text(pending.get("interaction_id"))
                if interaction_id:
                    request["id"] = interaction_id
            if not first_text(request.get("kind"), request.get("type")):
                request["kind"] = first_text(pending.get("kind"))
            if not first_text(request.get("status")):
                request["status"] = "waiting_for_user"
            request.setdefault("workflow", first_text(pending.get("workflow"), request.get("kind")))
            request.setdefault("goal_id", first_text(pending.get("goal_id")))
            request.setdefault("run_id", first_text(pending.get("run_id")))
            request.setdefault("conversation_id", first_text(pending.get("conversation_id")))
            requests.append(request)
    return requests


def test_continuation_interaction_requests() -> None:
    rows = continuation_interaction_requests([{
        "pending_interaction": {
            "interaction_id": "interaction-mix",
            "kind": "mix_tick_confirmation",
            "requests": [{"id": "interaction-mix", "kind": "mix_tick_confirmation", "status": "waiting_for_user"}],
        }
    }])
    assert len(rows) == 1 and rows[0]["id"] == "interaction-mix" and rows[0]["kind"] == "mix_tick_confirmation"


def newest_agent_runtime_state_for_conversation(project_path: str, conversation_id: str, started_at: float) -> dict[str, Any]:
    project = Path(project_path)
    roots = {project.parent, project}
    if project.parent.parent != project.parent:
        roots.add(project.parent.parent)
    candidates: list[Path] = []
    for root in roots:
        history_root = root / ".vit_history"
        if history_root.is_dir():
            candidates.extend(history_root.rglob("agent_runtime_state.json"))
    found: dict[str, Any] = {}
    found_updated = ""
    for path in sorted(set(candidates), key=lambda item: item.stat().st_mtime, reverse=True):
        try:
            if path.stat().st_mtime < started_at - 60:
                continue
            state = json.loads(path.read_text(encoding="utf-8", errors="replace"))
        except (OSError, ValueError):
            continue
        loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
        loop = loops.get(conversation_id)
        if not isinstance(loop, dict):
            continue
        updated = first_text(loop.get("updated_at"))
        if not found or (updated and updated >= found_updated):
            found, found_updated = state, updated
    return found


def persisted_free_state_loop(project_path: str, conversation_id: str, started_at: float) -> dict[str, Any]:
    state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, started_at)
    loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
    loop = loops.get(conversation_id)
    return loop if isinstance(loop, dict) else {}


def poll_persisted_loop_for_audition(project_path: str, conversation_id: str, run_started: float, timeout: float) -> dict[str, Any]:
    """Wait for the D1 loop to reach human_audition_ready while leaving generic
    mix suggestions unanswered. The scheduler advances the experiment on its own
    (an unanswered suggestion parks its continuation in waiting_interaction,
    which is not an active status), so polling the persisted projection is the
    faithful stand-in for a user who simply ignores the suggestion."""
    deadline = time.monotonic() + timeout
    loop: dict[str, Any] = {}
    while time.monotonic() < deadline:
        loop = persisted_free_state_loop(project_path, conversation_id, run_started)
        receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
        if receipt.get("human_audition_ready") is True:
            return loop
        time.sleep(2)
    return loop


def persisted_task_semantic_state(state: dict[str, Any], conversation_id: str) -> dict[str, Any]:
    goal_runtime = state.get("goal_runtime") if isinstance(state.get("goal_runtime"), dict) else {}
    goals = rows(goal_runtime.get("goals"))
    # The conversation -> goal binding is authoritative in conversation_goals;
    # the goal row's own conversation_id field is not always populated.
    conversation_goals = state.get("conversation_goals") if isinstance(state.get("conversation_goals"), dict) else {}
    bound_goal_id = first_text(conversation_goals.get(conversation_id))
    for goal in goals:
        if bound_goal_id and first_text(goal.get("goal_id")) == bound_goal_id:
            task = goal.get("task") if isinstance(goal.get("task"), dict) else {}
            semantic = task.get("semantic_state") if isinstance(task.get("semantic_state"), dict) else {}
            if semantic:
                return semantic
    for goal in goals:
        if first_text(goal.get("conversation_id")) != conversation_id:
            continue
        task = goal.get("task") if isinstance(goal.get("task"), dict) else {}
        semantic = task.get("semantic_state") if isinstance(task.get("semantic_state"), dict) else {}
        if semantic:
            return semantic
    return {}


def prepare_project(base_url: str, case: dict[str, Any], timeout: float) -> dict[str, Any]:
    project_path = str(Path(str(case["project_path"])).resolve())
    invoke(base_url, "project.open", {"file_path": project_path, "project_path": project_path}, timeout, confirmed=True)
    state = invoke(base_url, "project.state", {}, timeout)
    if project_path_from_state(state).lower() != project_path.lower():
        raise RuntimeError("live project binding does not match the public p01 project")

    deadline = time.monotonic() + timeout
    latest: dict[str, Any] = {}
    started = False
    while time.monotonic() < deadline:
        latest = invoke(base_url, "project.audio_analysis_status", {"latest": True}, timeout)
        candidates = [item for item in dicts(latest) if "dad_fact_status" in item]
        analysis = candidates[0] if candidates else latest
        status = first_text(analysis.get("dad_fact_status")).lower()
        ready = int(analysis.get("dad_fact_ready_count", 0) or 0)
        total = int(analysis.get("dad_fact_total_count", 0) or 0)
        waveforms = rows(analysis.get("track_waveform_envelopes"))
        if status == "ready" and total > 0 and ready == total and len(waveforms) >= total:
            return {"project_path": project_path, "dad_fact_status": status, "dad_fact_ready_count": ready, "dad_fact_total_count": total}
        queue_status = first_text(analysis.get("analysis_queue_status")).lower()
        if not started and (status not in {"ready", "building", "queued", "pending", "running"} or queue_status == "missing"):
            invoke(base_url, "project.audio_analysis_start", {"retry_missing": True, "rebuild_from_project": True, "interval_ms": 50}, timeout)
            started = True
        time.sleep(1)
    raise RuntimeError("public p01 DAD evidence did not become ready: " + json.dumps(latest, ensure_ascii=False)[:2000])


def payload_mentions_experiment_flow(payload: dict[str, Any]) -> bool:
    text = json.dumps(payload, ensure_ascii=False, default=str).lower()
    return any(marker in text for marker in EXPERIMENT_FLOW_MARKERS)


def recommended_interaction(response: dict[str, Any], admitted_domain_selected: bool, experiment_proposal_approved: bool = False, experiment_applied: bool = False, approved_domain_tick_ids: dict[str, int] | None = None) -> dict[str, Any] | None:
    interactions = rows(response.get("interaction_requests"))
    confirmed_ticks = approved_domain_tick_ids if approved_domain_tick_ids is not None else {}
    for interaction in interactions:
        payload = interaction.get("payload") if isinstance(interaction.get("payload"), dict) else {}
        kind = first_text(interaction.get("kind"), interaction.get("type")).lower()
        domains = {first_text(value).lower() for value in values_for_key(payload, "action_domain")}
        if kind in GENERIC_MIX_CONFIRMATION_KINDS:
            # The experiment action confirmations arrive on the mix-tick
            # surface too (the domain entry routes them there), so the gate is
            # the payload's nested action_domain. The same interaction id is
            # legitimately confirmed twice (selection, then execution), so a
            # re-surfaced id stays approvable after the receipt lands; only
            # brand-new domain ticks and settlement-marked exceptions are
            # filtered once the experiment has applied (reobserve chains).
            if not (domains & set(ADMITTED_DOMAIN_KINDS)):
                continue
            interaction_id = first_text(interaction.get("id"), interaction.get("interaction_id"))
            if confirmed_ticks.get(interaction_id, 0) >= MAX_EXPERIMENT_TICK_CONFIRMATIONS:
                continue
            if experiment_applied and interaction_id not in confirmed_ticks and not payload_mentions_experiment_flow(payload):
                continue
        if kind == "improvement_proposal_confirmation" and experiment_proposal_approved:
            continue
        if kind == "confirmation" and first_text(interaction.get("workflow")) == "plugin_grabber_load_and_get_params" and rows(payload.get("commands")):
            # FAM1-S2 (2026-08-31 run 20260831_100305 forensics): the de_esser
            # diagnosis may route through the plugin recommendation surface,
            # whose follow-up grabber load confirmation arrives as a plan
            # confirmation without an actions array, so the generic action
            # picker below cannot answer it and the run stalls before the D1
            # mutation. The user stand-in approves the model-requested load:
            # the plugin choice itself originated from the model's own
            # PCA-filtered recommendation, so no fixture truth is injected
            # here and every D1 admission/whitelist gate stays in charge.
            interaction_id = first_text(interaction.get("id"), interaction.get("interaction_id"))
            durable_payload = dict(payload)
            durable_payload.setdefault("workflow", first_text(interaction.get("workflow"), interaction.get("kind")))
            durable_payload.setdefault("kind", first_text(interaction.get("kind")))
            durable_payload.setdefault("type", first_text(interaction.get("type"), interaction.get("kind")))
            durable_payload.setdefault("conversation_id", first_text(interaction.get("conversation_id")))
            durable_payload.setdefault("goal_id", first_text(interaction.get("goal_id")))
            durable_payload.setdefault("run_id", first_text(interaction.get("run_id")))
            return {"interaction_id": interaction_id, "decision": "approve", "action_id": "approve", "payload": durable_payload}
        if not admitted_domain_selected and not (domains & set(ADMITTED_DOMAIN_KINDS)):
            continue
        eligible = []
        for action in rows(interaction.get("actions")):
            action_id = first_text(action.get("id"), action.get("action_id")).lower()
            if action_id and action_id not in {"cancel", "reject", "decline", "abort"}:
                eligible.append(action)
        selected = next((item for item in eligible if item.get("recommended") is True), None)
        if selected is None:
            selected = next((item for item in eligible if first_text(item.get("style")).lower() in {"primary", "confirm"}), None)
        if selected is None:
            continue
        action_id = first_text(selected.get("id"), selected.get("action_id"))
        interaction_id = first_text(interaction.get("id"), interaction.get("interaction_id"))
        durable_payload = dict(payload)
        durable_payload.setdefault("workflow", first_text(interaction.get("workflow"), interaction.get("kind"), interaction.get("type")))
        durable_payload.setdefault("kind", first_text(interaction.get("kind"), interaction.get("type")))
        durable_payload.setdefault("type", first_text(interaction.get("type"), interaction.get("kind")))
        durable_payload.setdefault("conversation_id", first_text(interaction.get("conversation_id")))
        durable_payload.setdefault("goal_id", first_text(interaction.get("goal_id")))
        durable_payload.setdefault("run_id", first_text(interaction.get("run_id")))
        return {"interaction_id": interaction_id, "decision": action_id, "action_id": action_id, "payload": durable_payload}
    return None


def find_d1_loop(responses: list[dict[str, Any]], authoritative: dict[str, Any] | None = None) -> dict[str, Any] | None:
    # One response can embed several snapshots of the same loop (the persisted
    # authoritative projection plus stale copies inside interaction payloads).
    # Select by updated_at so the freshest snapshot wins regardless of walk
    # order; equal stamps keep the last-walked copy. An explicitly supplied
    # authoritative (persisted) projection outranks every envelope-embedded
    # copy at equal or missing stamps — an envelope copy replaces it only with
    # a strictly later updated_at (2026-08-29 175049 smoke: a stale round-1=0
    # envelope copy could beat the authoritative projection on a tie).
    found = None
    found_updated = ""
    for response in responses:
        for item in dicts(response):
            if item.get("schema_version") == "free_state_reasoning_loop.v1" and isinstance(item.get("experiment"), dict):
                admission = item["experiment"].get("admission")
                if isinstance(admission, dict) and first_text(admission.get("typed_action", {}).get("action_domain") if isinstance(admission.get("typed_action"), dict) else "").lower() in ADMITTED_DOMAIN_KINDS:
                    updated = first_text(item.get("updated_at"))
                    if found is None or updated >= found_updated:
                        found = item
                        found_updated = updated
    if authoritative and (found is None or first_text(found.get("updated_at")) <= first_text(authoritative.get("updated_at"))):
        return authoritative
    return found


def test_find_d1_loop_authoritative_tie_break() -> None:
    admitted = {
        "schema_version": "free_state_reasoning_loop.v1",
        "updated_at": "2026-08-29T09:55:02.5614462Z",
        "experiment": {"admission": {"typed_action": {"action_domain": "static_eq"}}, "rounds": [{"interventions": [{}]}]},
    }
    stale_envelope = {
        "schema_version": "free_state_reasoning_loop.v1",
        "updated_at": "2026-08-29T09:55:02.5614462Z",
        "experiment": {"admission": {"typed_action": {"action_domain": "static_eq"}}, "rounds": [{"interventions": []}]},
    }
    later_envelope = {**stale_envelope, "updated_at": "2026-08-29T09:56:02.0000000Z"}
    responses = [{"workflow_data": {"free_state_reasoning_loop": stale_envelope}}]
    picked = find_d1_loop(responses, authoritative=admitted)
    assert picked is admitted, "equal-stamp envelope copy must not beat the authoritative projection"
    picked = find_d1_loop([{"workflow_data": {"free_state_reasoning_loop": {**stale_envelope, "updated_at": ""}}}], authoritative=admitted)
    assert picked is admitted, "missing-stamp envelope copy must not beat the authoritative projection"
    picked = find_d1_loop([{"workflow_data": {"free_state_reasoning_loop": later_envelope}}], authoritative=admitted)
    assert picked is later_envelope, "strictly later envelope copy must still win"
    assert find_d1_loop(responses) is stale_envelope, "no-authoritative walk keeps the previous semantics"


def find_admission_boundary(responses: list[dict[str, Any]]) -> dict[str, Any] | None:
    """Return the first durable FS6/FS7 admission projection.

    Admission-only smoke must stop before interaction confirmation. A receipt
    is authoritative when present; the phase/status fallback keeps the smoke
    explainable for terminal closures produced by older response envelopes.
    """
    found: dict[str, Any] | None = None
    for item in responses:
        for row in dicts(item):
            if row.get("schema_version") != "free_state_reasoning_loop.v1":
                continue
            receipt = row.get("admission_receipt") if isinstance(row.get("admission_receipt"), dict) else {}
            phase = first_text(row.get("current_phase")).lower()
            status = first_text(row.get("status")).lower()
            if receipt or phase == "fs7_improvement_proposal" or (phase == "fs9_terminal" and status in {"blocked", "capability_blocked"}):
                found = {
                    "phase": phase,
                    "status": status,
                    "admission_receipt": receipt,
                    "latest_decision": row.get("latest_decision") if isinstance(row.get("latest_decision"), dict) else {},
                    # FAM1-S2 ruling 4: the fs8 parked-state branch re-derives
                    # its invariants from the raw row, and the pending
                    # interaction surface lives on the enclosing response
                    # (scheduler-drained responses carry it only there).
                    "loop_row": row,
                    "interaction_requests": rows(item.get("interaction_requests")),
                }
    return found


def validate_admission_only_boundary(boundary: dict[str, Any]) -> None:
    phase = first_text(boundary.get("phase")).lower()
    status = first_text(boundary.get("status")).lower()
    require(phase in {"fs7_improvement_proposal", "fs8_experiment_verification", "fs9_terminal"}, f"admission-only ended at unexpected phase {phase}")
    require(status != "no_candidate_found", "admission-only fabricated no_candidate_found")
    receipt = boundary.get("admission_receipt") if isinstance(boundary.get("admission_receipt"), dict) else {}
    if phase == "fs8_experiment_verification":
        # FAM1-S2 GLM ruling 4 (2026-08-31, run 20260831_103303): when one
        # scheduler continuation absorbs diagnosis through admission, the
        # boundary surfaces as the parked post-admission state instead of an
        # fs7 turn — the phase labels the ladder position while the stop
        # reason is the still-pending proposal confirmation. Admit that
        # parked state only under its full contract (tightened invariants,
        # not a bare whitelist widening): awaiting the experiment, decision
        # still needs_experiment, zero executed interventions, no d1_receipt,
        # a valid admission receipt, and the unanswered
        # improvement_proposal_confirmation face proving the loop stopped at
        # the confirmation gate rather than past it.
        require(status == "awaiting_experiment", f"fs8 admission-only boundary status is {status!r}, not awaiting_experiment")
        decision = boundary.get("latest_decision") if isinstance(boundary.get("latest_decision"), dict) else {}
        require(first_text(decision.get("status")).lower() == "needs_experiment", "fs8 admission-only boundary decision is not needs_experiment")
        loop_row = boundary.get("loop_row") if isinstance(boundary.get("loop_row"), dict) else {}
        experiment = loop_row.get("experiment") if isinstance(loop_row.get("experiment"), dict) else {}
        for round_row in rows(experiment.get("rounds")):
            require(not rows(round_row.get("interventions")), "fs8 admission-only boundary already carries executed interventions")
        require(not loop_row.get("d1_receipt"), "fs8 admission-only boundary already carries a d1_receipt")
        require(receipt.get("proposal_present") is True and receipt.get("proposal_valid") is True, "fs8 admission receipt is not valid")
        require(not receipt.get("failed_gate_ids"), "fs8 admission receipt contains failed gates")
        require(any(first_text(item.get("kind"), item.get("type")).lower() == "improvement_proposal_confirmation" for item in rows(boundary.get("interaction_requests"))),
                "fs8 admission-only boundary has no pending improvement_proposal_confirmation interaction")
    elif phase == "fs7_improvement_proposal":
        proposal = boundary.get("latest_decision", {}).get("improvement_proposal")
        require(isinstance(proposal, dict), "FS7 admission-only boundary omitted improvement_proposal")
        require(receipt.get("proposal_present") is True and receipt.get("proposal_valid") is True, "FS7 admission receipt is not valid")
        require(not receipt.get("failed_gate_ids"), "FS7 admission receipt contains failed gates")
    else:
        require(status in {"blocked", "capability_blocked"}, f"FS9 admission-only ended with status {status}")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def validate_d1(base_url: str, conversation_id: str, responses: list[dict[str, Any]], timeout: float, case_id: str) -> dict[str, Any]:
    loop = find_d1_loop(responses)
    require(loop is not None, "D1 loop projection is missing")
    assert loop is not None
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    admission = experiment.get("admission") if isinstance(experiment.get("admission"), dict) else {}
    typed = admission.get("typed_action") if isinstance(admission.get("typed_action"), dict) else {}
    domain = first_text(typed.get("action_domain")).lower()
    require(domain in ADMITTED_DOMAIN_KINDS, f"D1 action_domain {domain!r} is not admitted by the D2-1 domain table")
    require(first_text(typed.get("action_kind")).lower() == ADMITTED_DOMAIN_KINDS[domain], f"D1 action_kind mismatch for admitted domain {domain}")
    time_dynamics_disclosure: dict[str, Any] | None = None
    frequency_time_events_disclosure: dict[str, Any] | None = None
    transient_structure_disclosure: dict[str, Any] | None = None
    stereo_space_disclosure: dict[str, Any] | None = None
    peak_structure_disclosure: dict[str, Any] | None = None
    activity_structure_disclosure: dict[str, Any] | None = None
    band_dynamics_disclosure: dict[str, Any] | None = None
    if domain == "static_eq":
        gain = typed.get("gain_db")
        require(isinstance(gain, (int, float)) and not isinstance(gain, bool) and gain != 0 and abs(gain) <= 2, "static_eq typed gain_db must be non-zero within +/-2")
        frequency = typed.get("frequency_hz")
        require(isinstance(frequency, (int, float)) and not isinstance(frequency, bool) and 20 <= frequency <= 20000, "static_eq typed frequency_hz must be within 20-20000")
    if domain == "broadband_compression":
        threshold = typed.get("threshold_db")
        require(isinstance(threshold, (int, float)) and not isinstance(threshold, bool) and threshold != 0 and abs(threshold) <= 2, "broadband_compression typed threshold_db must be non-zero within +/-2")
    if domain == "de_esser":
        threshold = typed.get("threshold_db")
        require(isinstance(threshold, (int, float)) and not isinstance(threshold, bool) and threshold != 0 and abs(threshold) <= 2, "de_esser typed threshold_db must be non-zero within +/-2")
    if domain == "transient_shaper":
        attack = typed.get("attack_db")
        require(isinstance(attack, (int, float)) and not isinstance(attack, bool) and attack != 0 and abs(attack) <= 2, "transient_shaper typed attack_db must be non-zero within +/-2")
    if domain == "gate_expander":
        rng = typed.get("range_db")
        require(isinstance(rng, (int, float)) and not isinstance(rng, bool) and rng != 0 and abs(rng) <= 2, "gate_expander typed range_db must be non-zero within +/-2")
    if domain == "multiband_dynamics":
        band_threshold = typed.get("band_threshold_db")
        require(isinstance(band_threshold, (int, float)) and not isinstance(band_threshold, bool) and band_threshold != 0 and abs(band_threshold) <= 2, "multiband_dynamics typed band_threshold_db must be non-zero within +/-2")
    if domain == "limiter":
        ceiling = typed.get("ceiling_db")
        require(isinstance(ceiling, (int, float)) and not isinstance(ceiling, bool) and ceiling != 0 and abs(ceiling) <= 2, "limiter typed ceiling_db must be non-zero within +/-2")
    if domain == "pan":
        delta = typed.get("delta_pan")
        require(isinstance(delta, (int, float)) and not isinstance(delta, bool) and delta != 0 and abs(delta) <= 0.15, "pan typed delta_pan must be non-zero within +/-0.15")
    require(int(admission.get("experiment_budget", 0) or 0) == 1, "D1 experiment_budget must equal one")
    for key in ("diagnostic_dose_bounds", "retained_dose_bounds"):
        bounds = admission.get(key) if isinstance(admission.get(key), dict) else {}
        require(int(bounds.get("max_action_attempts", 0) or 0) == 1, f"D1 {key}.max_action_attempts must equal one")

    experiment_rounds = rows(experiment.get("rounds"))
    require(len(experiment_rounds) == 1, "D1 must contain exactly one experiment round")
    round_row = experiment_rounds[0]
    interventions = rows(round_row.get("interventions"))
    require(len(interventions) == 1, "D1 must contain exactly one forward mutation")
    intervention = interventions[0]
    receipt = intervention.get("receipt") if isinstance(intervention.get("receipt"), dict) else {}
    before_revision = first_text(receipt.get("before_revision"))
    after_revision = first_text(receipt.get("after_revision"), receipt.get("applied_revision"))
    require(before_revision and after_revision and before_revision != after_revision, "D1 receipt requires distinct before/after revisions")
    # Native-domain receipt readback keys: track_gain books dB, pan books the
    # normalized pan readback (requested_target_pan/actual_readback_pan pair),
    # every other domain uses the generic value readback.
    readback_key = {"track_gain": "actual_readback_db", "pan": "actual_readback_pan"}.get(domain, "actual_readback_value")
    for key in ("transaction_id", "idempotency_key", readback_key):
        require(receipt.get(key) not in (None, ""), f"D1 execution receipt missing {key}")
    if domain == "static_eq":
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"static_eq execution receipt missing {key}")
    if domain == "broadband_compression":
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"broadband_compression execution receipt missing {key}")
        # D2-1.5-S2f-2 evidence-chain assertions. The model freely chooses its
        # own observation views (no server view injection), so the round check
        # only requires every requested view to have been disclosed; the
        # disclosability of the COM time-dynamics view itself is probed
        # directly below against the live stack.
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "broadband_compression observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.time_dynamics"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # The contract's formal-run gate for track.time_dynamics is "ready or
        # partial and fresh": source-only COM discloses as partial by design
        # (micro-transient limits), so partial counts as disclosable; the
        # s2f-2a rejection window answers rejected/missing instead.
        require(disclosure_status in {"ready", "partial"},
                "track.time_dynamics was not disclosable on the admitted target (s2f-2a rejection window): " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.time_dynamics" in executed_views,
                "track.time_dynamics disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.time_dynamics disclosure was stale")
        time_dynamics_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "de_esser":
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"de_esser execution receipt missing {key}")
        # FAM1-S2 evidence-chain assertions, mirroring the compression branch:
        # the model freely chooses its own observation views (no server view
        # injection), so the round check only requires every requested view to
        # have been disclosed; the disclosability of the DOM frequency-time
        # events view itself is probed directly below against the live stack.
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "de_esser observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.frequency_time_events"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the COM probe: "ready or partial and
        # fresh" — the DOM source-only projection discloses as partial by
        # design (bounded time-frequency evidence with explicit omissions), so
        # partial counts as disclosable; rejected/missing answers the
        # post-action disclosure risk called out in the FAM1 survey.
        require(disclosure_status in {"ready", "partial"},
                "track.frequency_time_events was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.frequency_time_events" in executed_views,
                "track.frequency_time_events disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.frequency_time_events disclosure was stale")
        frequency_time_events_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "transient_shaper":
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"transient_shaper execution receipt missing {key}")
        # FAM2-S2 evidence-chain assertions, mirroring the de_esser branch
        # three-piece: the model freely chooses its own observation views (no
        # server view injection), so the round check only requires every
        # requested view to have been disclosed; the disclosability of the DOM
        # transient-structure view itself is probed directly below against the
        # live stack.
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "transient_shaper observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.transient_structure"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the COM/DOM probes: "ready or partial
        # and fresh" — the DOM source-only projection discloses as partial by
        # design (bounded onset/body evidence with explicit omissions), so
        # partial counts as disclosable; rejected/missing answers the
        # post-action disclosure risk called out in the FAM2 survey.
        require(disclosure_status in {"ready", "partial"},
                "track.transient_structure was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.transient_structure" in executed_views,
                "track.transient_structure disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.transient_structure disclosure was stale")
        transient_structure_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "limiter":
        # FAM4-S2 evidence-chain assertions, mirroring the transient branch:
        # plugin identity pair, round view coverage, and the disclosability of
        # the DOM peak-structure view itself probed against the live stack.
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"limiter execution receipt missing {key}")
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "limiter observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.peak_structure"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the DOM/COM probes: "ready or partial
        # and fresh" -- the DOM source-only projection may disclose as partial
        # by design (bounded peak evidence with explicit omissions), so partial
        # counts as disclosable; rejected/missing answers the post-action
        # disclosure risk.
        require(disclosure_status in {"ready", "partial"},
                "track.peak_structure was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.peak_structure" in executed_views,
                "track.peak_structure disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.peak_structure disclosure was stale")
        peak_structure_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "gate_expander":
        # FAM5-S2 evidence-chain assertions, mirroring the limiter branch:
        # plugin identity pair, round view coverage, and the disclosability of
        # the DOM activity-structure view itself probed against the live stack.
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"gate_expander execution receipt missing {key}")
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "gate_expander observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.activity_structure"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the DOM probes: "ready or partial and
        # fresh" -- the DOM source-only projection may disclose as partial by
        # design (bounded activity evidence with explicit omissions), so
        # partial counts as disclosable; rejected/missing answers the
        # post-action disclosure risk.
        require(disclosure_status in {"ready", "partial"},
                "track.activity_structure was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.activity_structure" in executed_views,
                "track.activity_structure disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.activity_structure disclosure was stale")
        activity_structure_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "multiband_dynamics":
        # FAM6-S2 evidence-chain assertions, mirroring the gate branch:
        # plugin identity pair, round view coverage, and the disclosability of
        # the DAD band-dynamics view itself probed against the live stack.
        for key in ("plugin_id", "param_id"):
            require(receipt.get(key) not in (None, ""), f"multiband_dynamics execution receipt missing {key}")
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "multiband_dynamics observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.band_dynamics"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the DOM probes: "ready or partial and
        # fresh" -- the DAD band-dynamics projection may disclose as partial by
        # design (bounded per-band evidence with explicit omissions), so
        # partial counts as disclosable; rejected/missing answers the
        # post-action disclosure risk.
        require(disclosure_status in {"ready", "partial"},
                "track.band_dynamics was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.band_dynamics" in executed_views,
                "track.band_dynamics disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.band_dynamics disclosure was stale")
        band_dynamics_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    if domain == "pan":
        # FAM3-S2 native-domain evidence-chain assertions, mirroring the p01/p03
        # branch form minus the plugin identity pair (pan is not PluginBound):
        # the requested/absolute-target versus actual-readback pan pair, the
        # round view coverage, and the disclosability of the CCB stereo-space
        # view itself probed against the live stack.
        for key in ("requested_target_pan", "actual_readback_pan"):
            require(receipt.get(key) not in (None, ""), f"pan execution receipt missing {key}")
        for observation_row in rows(round_row.get("observations")):
            requested = {first_text(value) for value in (observation_row.get("requested_view_ids") or [])}
            executed = {first_text(value) for value in (observation_row.get("executed_view_ids") or [])}
            require(requested <= executed, "pan observation lost requested views: " + json.dumps({"requested": sorted(requested), "executed": sorted(executed)}, ensure_ascii=False))
        target_ref = admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {}
        disclosure = invoke(base_url, "ccb.observation_request", {
            "view_ids": ["track.stereo_space"],
            "target_ref": {"kind": first_text(target_ref.get("kind")) or "track", "id": first_text(target_ref.get("id"))},
            "freshness_class": "fresh",
        }, timeout)
        bundle = disclosure.get("bundle") if isinstance(disclosure.get("bundle"), dict) else {}
        audit = bundle.get("audit_receipt") if isinstance(bundle.get("audit_receipt"), dict) else {}
        executed_views = {first_text(value) for value in (audit.get("actual_executed_view_ids") or bundle.get("actual_executed_view_ids") or disclosure.get("actual_executed_view_ids") or [])}
        if not executed_views:
            executed_views = {first_text(key) for key in (bundle.get("views") or {})}
        disclosure_status = first_text(disclosure.get("status")).lower() or first_text(bundle.get("status")).lower()
        # Same formal-run gate wording as the COM/DOM probes: "ready or partial
        # and fresh" — the slow-stereo summary projection may disclose as
        # partial by design (bounded stereo evidence with explicit omissions),
        # so partial counts as disclosable; rejected/missing answers the
        # post-action disclosure risk called out in the FAM3 survey.
        require(disclosure_status in {"ready", "partial"},
                "track.stereo_space was not disclosable on the admitted target: " + json.dumps({"status": disclosure.get("status"), "bundle_status": bundle.get("status")}, ensure_ascii=False))
        require("track.stereo_space" in executed_views,
                "track.stereo_space disclosure probe did not execute the view: " + json.dumps(sorted(executed_views), ensure_ascii=False))
        freshness = bundle.get("freshness") if isinstance(bundle.get("freshness"), dict) else {}
        require(first_text(freshness.get("status")).lower() != "stale", "track.stereo_space disclosure was stale")
        stereo_space_disclosure = {"observation_id": first_text(bundle.get("observation_id")), "status": disclosure_status, "executed_view_ids": sorted(executed_views)}
    require(receipt.get("readback_verified") is True, "D1 actual readback was not verified")

    post_observations = [item for item in rows(round_row.get("observations")) if item.get("post_action") is True]
    require(len(post_observations) == 1, "D1 requires one post-action CCB observation")
    post = post_observations[0]
    require(post.get("fresh") is True and first_text(post.get("project_revision")) == after_revision, "post-action CCB is not fresh and revision-bound")
    require(round_row.get("materiality") is not None, "acoustic materiality record is missing")
    require(round_row.get("target_response") is not None, "target response record is missing")

    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    required_flags = ("parameter_applied", "readback_verified", "evaluation_ready", "human_audition_ready", "human_confirmed", "ambiguous", "rolled_back", "settled")
    require(all(flag in d1_receipt and isinstance(d1_receipt[flag], bool) for flag in required_flags), "D1 receipt does not express all required states")
    require(d1_receipt.get("parameter_applied") is True and d1_receipt.get("readback_verified") is True and d1_receipt.get("evaluation_ready") is True, "D1 technical/evaluation gates are incomplete")
    require(d1_receipt.get("human_audition_ready") is True, "D1 did not reach human_audition_ready")
    require(d1_receipt.get("human_confirmed") is False and d1_receipt.get("settled") is False, "smoke must not fabricate human confirmation or settlement")
    require(int(d1_receipt.get("forward_mutation_count", 0) or 0) == 1, "D1 receipt forward mutation cardinality mismatch")
    require("validation_error" not in d1_receipt, "D1 receipt schema validation failed: " + first_text(d1_receipt.get("validation_error")))

    session = loop.get("audition_session_snapshot") if isinstance(loop.get("audition_session_snapshot"), dict) else {}
    session_id = first_text(loop.get("audition_session_id"), session.get("session_id"))
    candidates = rows(session.get("candidates"))
    require(session_id and first_text(session.get("status")).lower() in {"ready", "playing", "stopped"}, "D1 audition session is not ready")
    require(len(candidates) == 2, "D1 audition requires exactly two candidates")
    by_id = {first_text(item.get("id")): item for item in candidates}
    require(set(by_id) == {"candidate-a", "candidate-b"}, "D1 audition candidate identity mismatch")
    for candidate in by_id.values():
        require(first_text(candidate.get("source_kind")) == "audio_file", "D1 audition candidates must use real audio_file sources")
        require(Path(first_text(candidate.get("source_ref"))).is_file(), "D1 audition WAV is missing")
        require(first_text(candidate.get("render_revision")) and first_text(candidate.get("preview_revision")), "D1 audition candidate provenance is incomplete")
    for key in ("source_ref", "project_revision", "render_revision", "preview_revision"):
        require(first_text(by_id["candidate-a"].get(key)) != first_text(by_id["candidate-b"].get(key)), f"D1 A/B candidates share {key}")

    actions_before = request_json("GET", base_url.rstrip("/") + "/agent/actions?limit=200", None, timeout)
    d1_actions_before = [item for item in rows(actions_before.get("actions")) if first_text(item.get("source")) == "free_state_d1_s1"]
    require(len(d1_actions_before) == 1, "D1 journal must contain exactly one forward mutation")
    state_before = invoke(base_url, "project.state", {}, timeout)
    revision_before_select = project_revision(state_before)
    for candidate_id in ("candidate-a", "candidate-b"):
        selected = request_json("POST", base_url.rstrip("/") + "/agent/audition/select", {"conversation_id": conversation_id, "session_id": session_id, "candidate_id": candidate_id}, timeout)
        require(first_text(selected.get("status")).lower() == "ok", f"audition.select failed for {candidate_id}")
    request_json("POST", base_url.rstrip("/") + "/agent/audition/stop", {"conversation_id": conversation_id, "session_id": session_id}, timeout)
    state_after = invoke(base_url, "project.state", {}, timeout)
    actions_after = request_json("GET", base_url.rstrip("/") + "/agent/actions?limit=200", None, timeout)
    d1_actions_after = [item for item in rows(actions_after.get("actions")) if first_text(item.get("source")) == "free_state_d1_s1"]
    require(project_revision(state_after) == revision_before_select, "audition.select changed the project revision")
    require(len(d1_actions_after) == len(d1_actions_before) == 1, "audition.select changed journal mutation cardinality")

    result = {
        "status": "pass",
        "public_case_id": case_id,
        "conversation_id": conversation_id,
        "action_domain": domain,
        "turn_id": first_text(experiment.get("turn_id")),
        "round_id": first_text(round_row.get("round_id")),
        "before_revision": before_revision,
        "after_revision": after_revision,
        "transaction_id": receipt["transaction_id"],
        "idempotency_key": receipt["idempotency_key"],
        "readback_key": readback_key,
        "readback_value": receipt[readback_key],
        "post_action_observation_id": post.get("observation_id"),
        "time_dynamics_disclosure": time_dynamics_disclosure,
        "forward_mutation_count": 1,
        "audition_session_id": session_id,
        "human_audition_ready": True,
        "human_confirmed": False,
    }
    if frequency_time_events_disclosure is not None:
        result["frequency_time_events_disclosure"] = frequency_time_events_disclosure
    if transient_structure_disclosure is not None:
        result["transient_structure_disclosure"] = transient_structure_disclosure
    if stereo_space_disclosure is not None:
        result["stereo_space_disclosure"] = stereo_space_disclosure
    if peak_structure_disclosure is not None:
        result["peak_structure_disclosure"] = peak_structure_disclosure
    if activity_structure_disclosure is not None:
        result["activity_structure_disclosure"] = activity_structure_disclosure
    if band_dynamics_disclosure is not None:
        result["band_dynamics_disclosure"] = band_dynamics_disclosure
    return result


# D2-REG3 A face: the honest-refusal expectation mode pins the REG2 signature
# (stats §48, run 20260902_184456) for a fail-closed refused execution. The
# four requirements are the task-card contract: (1) a valid typed proposal in
# an admitted domain, (2) the execution refused by the fail-closed
# unreachable-range gate, (3) zero forward mutation and zero unplanned side
# effects, (4) the honest terminal form with the pre-REG2
# StopProjectRevisionStale misattribution (stats §47 runs 20260902_182340/
# 182716) explicitly rejected. The stale scan matches the full misattribution
# phrases rather than the bare word "stale" on purpose: benign schema text
# (MOM llm_context contract strings, task_trajectory
# stale_for_current_revision keys) legitimately contains "stale".
HONEST_REFUSAL_FAIL_CLOSED_MARKER = "outside the reachable normalized range"
HONEST_REFUSAL_TERMINAL_STATE = "capability_blocked"
HONEST_REFUSAL_TERMINAL_REASON = "closure observation round boundary reached"
HONEST_REFUSAL_STALE_SIGNATURES = ("project_revision_stale", "stopprojectrevisionstale", "different project revision")
# Typed dose keys mirror the per-domain value checks in validate_d1 (value
# non-zero within the bound); either direction is accepted -- the refusal
# expectation pins the honest handling, not the model's direction choice.
HONEST_REFUSAL_DOSE_KEYS = {
    "track_gain": ("gain_db", 2.0),
    "static_eq": ("gain_db", 2.0),
    "broadband_compression": ("threshold_db", 2.0),
    "de_esser": ("threshold_db", 2.0),
    "transient_shaper": ("attack_db", 2.0),
    "pan": ("delta_pan", 0.15),
    "limiter": ("ceiling_db", 2.0),
    "gate_expander": ("range_db", 2.0),
    "multiband_dynamics": ("band_threshold_db", 2.0),
}


def honest_refusal_stale_hits(*containers: Any) -> list[str]:
    """Collect stale-misattribution phrase occurrences with their JSON paths."""
    hits: list[str] = []

    def walk(node: Any, path: str) -> None:
        if isinstance(node, dict):
            for key, value in node.items():
                if isinstance(value, str):
                    for signature in HONEST_REFUSAL_STALE_SIGNATURES:
                        if signature in value.lower():
                            hits.append(f"{path}.{key}={value!r}")
                            break
                walk(value, f"{path}.{key}")
        elif isinstance(node, list):
            for index, item in enumerate(node):
                if isinstance(item, str):
                    for signature in HONEST_REFUSAL_STALE_SIGNATURES:
                        if signature in item.lower():
                            hits.append(f"{path}[{index}]={item!r}")
                            break
                else:
                    walk(item, f"{path}[{index}]")

    for container in containers:
        walk(container, "$")
    return hits


def validate_honest_refusal(
    loop: dict[str, Any] | None,
    responses: list[dict[str, Any]],
    continuations: list[dict[str, Any]],
    runtime_status: dict[str, Any],
    terminal_causes: list[Any],
    case_id: str,
) -> dict[str, Any]:
    require(loop is not None, "D1 loop projection is missing")
    assert loop is not None
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    admission = experiment.get("admission") if isinstance(experiment.get("admission"), dict) else {}
    typed = admission.get("typed_action") if isinstance(admission.get("typed_action"), dict) else {}
    domain = first_text(typed.get("action_domain")).lower()
    # Requirement 1: the model formed a valid typed proposal in an admitted
    # domain (either dose direction).
    require(domain in ADMITTED_DOMAIN_KINDS, f"(1) honest-refusal action_domain {domain!r} is not admitted by the D2-1 domain table")
    require(first_text(typed.get("action_kind")).lower() == ADMITTED_DOMAIN_KINDS[domain], f"(1) honest-refusal action_kind mismatch for admitted domain {domain}")
    dose_key, dose_bound = HONEST_REFUSAL_DOSE_KEYS[domain]
    dose = typed.get(dose_key)
    require(isinstance(dose, (int, float)) and not isinstance(dose, bool) and dose != 0 and abs(dose) <= dose_bound,
            f"(1) honest-refusal typed {dose_key} must be non-zero within +/-{dose_bound:g}")
    experiment_rounds = rows(experiment.get("rounds"))
    require(len(experiment_rounds) == 1, "(1) honest-refusal requires exactly one experiment round")
    round_row = experiment_rounds[0]
    interventions = rows(round_row.get("interventions"))
    require(len(interventions) == 1, "(1) honest-refusal requires exactly one attempted intervention (zero means the confirmation never reached execution)")
    intervention = interventions[0]
    require(intervention.get("user_confirmed") is True, "(1) honest-refusal intervention was never user-confirmed")

    # Requirement 2: the confirmed execution was refused by the fail-closed
    # unreachable-range gate (staticeq_delta.go planDeltaChannels).
    require(first_text(intervention.get("technical_application")).lower() == "failed", "(2) honest-refusal technical_application is not failed")
    receipt = intervention.get("receipt") if isinstance(intervention.get("receipt"), dict) else {}
    require(first_text(receipt.get("status")).lower() == "failed", "(2) honest-refusal execution receipt status is not failed")
    refusal_error = first_text(receipt.get("error"))
    require(HONEST_REFUSAL_FAIL_CLOSED_MARKER in refusal_error,
            "(2) honest-refusal error is not the fail-closed unreachable-range gate: " + refusal_error)

    # Requirement 3: zero forward mutation, zero unplanned side effects. The
    # refused execution applies nothing, renders no after state, books no
    # rollback compensation, honors the single-attempt dose budget, and the
    # receipt self-reports its own validation gap instead of hiding it.
    require(int(admission.get("experiment_budget", 0) or 0) == 1, "(3) honest-refusal experiment_budget must equal one")
    for key in ("diagnostic_dose_bounds", "retained_dose_bounds"):
        bounds = admission.get(key) if isinstance(admission.get(key), dict) else {}
        require(int(bounds.get("max_action_attempts", 0) or 0) == 1, f"(3) honest-refusal {key}.max_action_attempts must equal one")
    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    require(bool(d1_receipt), "(3) honest refusal did not produce a D1 receipt")
    require(d1_receipt.get("parameter_applied") is False, "(3) refused execution must not apply parameters")
    require(d1_receipt.get("readback_verified") is False, "(3) refused execution must not claim a verified readback")
    require(d1_receipt.get("after_render") is None, "(3) refused execution must not produce an after render")
    require(d1_receipt.get("rolled_back") is False and int(d1_receipt.get("rollback_compensation_count", 0) or 0) == 0 and d1_receipt.get("rollback_receipt") is None,
            "(3) refused execution must not book rollback compensation")
    layers = d1_receipt.get("layers") if isinstance(d1_receipt.get("layers"), dict) else {}
    technical_readback = layers.get("technical_readback") if isinstance(layers.get("technical_readback"), dict) else {}
    require(first_text(technical_readback.get("status")).lower() == "failed", "(3) refused receipt technical_readback layer is not failed")
    net_outcome = layers.get("net_outcome") if isinstance(layers.get("net_outcome"), dict) else {}
    require(first_text(net_outcome.get("status")).lower() == "stable", "(3) refused receipt net_outcome layer is not stable")
    require(bool(first_text(d1_receipt.get("validation_error"))), "(3) refused receipt must self-report its validation gap")

    # Requirement 4: the honest terminal form (stats §48) with the stale
    # misattribution explicitly rejected. The scan runs first so a pre-REG2
    # form dies on the stale ban itself, not on a downstream form mismatch.
    stale_hits = honest_refusal_stale_hits(loop, responses, continuations, runtime_status, terminal_causes)
    require(not stale_hits, "(4) stale misattribution signatures present (pre-REG2 form): " + "; ".join(stale_hits[:5]))
    require(first_text(loop.get("status")).lower() == HONEST_REFUSAL_TERMINAL_STATE, "(4) honest-refusal loop status is not capability_blocked")
    decision = loop.get("latest_decision") if isinstance(loop.get("latest_decision"), dict) else {}
    require(first_text(decision.get("stop_reason")).lower() == HONEST_REFUSAL_TERMINAL_STATE, "(4) honest-refusal stop_reason is not capability_blocked")
    semantic: dict[str, Any] = {}
    task = runtime_status.get("task") if isinstance(runtime_status.get("task"), dict) else {}
    if isinstance(runtime_status.get("task_semantic_state"), dict):
        semantic = runtime_status["task_semantic_state"]
    elif isinstance(task.get("semantic_state"), dict):
        semantic = task["semantic_state"]
    else:
        for row in continuations:
            if isinstance(row.get("task_semantic_state"), dict):
                semantic = row["task_semantic_state"]
    require(bool(semantic), "(4) honest-refusal terminal semantic state is not inspectable")
    require(first_text(semantic.get("state")).lower() == HONEST_REFUSAL_TERMINAL_STATE, "(4) honest-refusal terminal semantic state is not capability_blocked")
    require(semantic.get("terminal") is True, "(4) honest-refusal terminal semantic state is not terminal")
    require(first_text(semantic.get("transition_reason")).lower() == HONEST_REFUSAL_TERMINAL_REASON,
            "(4) honest-refusal transition_reason is not the bounded-window terminal form: " + first_text(semantic.get("transition_reason")))

    return {
        "status": "honest_refusal",
        "public_case_id": case_id,
        "action_domain": domain,
        "action_kind": first_text(typed.get("action_kind")),
        "typed_action": {dose_key: dose},
        "target_ref": admission.get("target_ref") if isinstance(admission.get("target_ref"), dict) else {},
        "round_id": first_text(round_row.get("round_id")),
        "action_id": first_text(intervention.get("action_id")),
        "refusal_error": refusal_error,
        "receipt_validation_error": first_text(d1_receipt.get("validation_error")),
        "stale_signature_scan": "clean",
        "terminal_state": first_text(semantic.get("state")),
        "terminal_reason": first_text(semantic.get("transition_reason")),
    }


def _honest_refusal_fixture() -> dict[str, Any]:
    """Minimal honest-refusal report fixture mirroring run 20260902_184456."""
    loop: dict[str, Any] = {
        "schema_version": "free_state_reasoning_loop.v1",
        "status": "capability_blocked",
        "experiment": {
            "admission": {
                "typed_action": {"action_domain": "broadband_compression", "action_kind": "broadband_threshold_adjust", "threshold_db": 1},
                "target_ref": {"kind": "track", "id": "1012"},
                "experiment_budget": 1,
                "diagnostic_dose_bounds": {"max_action_attempts": 1},
                "retained_dose_bounds": {"max_action_attempts": 1},
            },
            "rounds": [{
                "round_id": "round-1-a2e58426115985ab",
                "interventions": [{
                    "action_id": "d1_turn_comp",
                    "user_confirmed": True,
                    "technical_application": "failed",
                    "receipt": {"status": "failed", "error": "delta target 12.8 dB (current 11.8 + 1) is outside the reachable normalized range"},
                }],
            }],
        },
        "latest_decision": {"stop_reason": "capability_blocked", "summary": "closure observation round boundary reached"},
        "d1_receipt": {
            "parameter_applied": False,
            "readback_verified": False,
            "after_render": None,
            "rolled_back": False,
            "rollback_compensation_count": 0,
            "rollback_receipt": None,
            "layers": {"technical_readback": {"status": "failed"}, "net_outcome": {"status": "stable"}},
            "validation_error": "project_revision is required (receipts are revision-bound)",
        },
    }
    runtime_status = {"task": {"task_semantic_state": {"state": "capability_blocked", "terminal": True, "transition_reason": "closure observation round boundary reached"}}}
    return {
        "loop": loop,
        "responses": [{"reply": "改善性提案已进入原生控制工具链。精确混音单步仍需确认，确认前不会修改工程。"}],
        "continuations": [{"free_state_stop_reason": "capability_blocked", "task_semantic_state": {"state": "capability_blocked", "terminal": True, "transition_reason": "closure observation round boundary reached"}}],
        "runtime_status": runtime_status,
        "terminal_causes": ["capability_blocked", "capability_blocked"],
        "case_id": "spv1_p03",
    }


def test_validate_honest_refusal_forms() -> None:
    # Honest form (REG2 §48): all four requirements hold.
    validate_honest_refusal(**_honest_refusal_fixture())

    def expect_rejection(mutate, fragment: str) -> None:
        fixture = _honest_refusal_fixture()
        mutate(fixture)
        try:
            validate_honest_refusal(**fixture)
        except RuntimeError as exc:
            assert fragment in str(exc), f"expected {fragment!r} in honest-refusal rejection, got: {exc}"
            return
        raise AssertionError("honest-refusal validation accepted a form it must reject")

    # Pre-REG2 stale form (§47 20260902_182340): the buried settlement reason
    # alone must trip the requirement-4 stale ban even when every other
    # surface still reads capability_blocked.
    def bury_stale(fixture: dict[str, Any]) -> None:
        fixture["responses"].append({"workflow_data": {"minimal_audio_closure": {"settlement": {"reason": "project_revision_stale"}}}})
    expect_rejection(bury_stale, "(4) stale misattribution signatures")

    # Applied form (positive path, e.g. 20260902_184702): requirement 2 fires.
    def apply_action(fixture: dict[str, Any]) -> None:
        intervention = fixture["loop"]["experiment"]["rounds"][0]["interventions"][0]
        intervention["technical_application"] = "applied"
        intervention["receipt"] = {"status": "applied", "before_revision": "7", "after_revision": "8"}
        receipt = fixture["loop"]["d1_receipt"]
        receipt["parameter_applied"] = True
        receipt["readback_verified"] = True
        receipt["after_render"] = {"sha256": "x"}
        receipt["validation_error"] = ""
    expect_rejection(apply_action, "(2)")

    # Drain-race form (§47 20260902_182544): the confirmation never reached
    # execution, so requirement 1's intervention cardinality fires.
    def drop_intervention(fixture: dict[str, Any]) -> None:
        fixture["loop"]["experiment"]["rounds"][0]["interventions"] = []
    expect_rejection(drop_intervention, "(1)")


SETTLEMENT_PROBE_TAG = "smoke_settlement_probe"
SETTLEMENT_PROBE_FREE_TEXT = "machine-originated settlement probe; not a human judgment"
SETTLEMENT_PROBE_ANSWERS = {
    "retain": {"heard_difference": "yes", "preference": "b"},
    "rollback": {"heard_difference": "yes", "preference": "a"},
    "ambiguous": {"heard_difference": "unsure", "preference": "unsure"},
}
SETTLEMENT_EXPECTATIONS = {
    "retain": {"outcome": "improved", "human_confirmed": True, "ambiguous": False, "rolled_back": False, "disposition": "retain"},
    "rollback": {"outcome": "rolled_back", "human_confirmed": True, "ambiguous": False, "rolled_back": True, "disposition": "rollback"},
    "ambiguous": {"outcome": "needs_user_judgment", "human_confirmed": False, "ambiguous": True, "rolled_back": False, "disposition": "request_audition"},
}
TERMINAL_CONTINUATION_STATUSES = {"completed", "cancelled", "failed"}


def probe_tags(evidence: dict[str, Any]) -> set[str]:
    return {first_text(tag) for tag in (evidence.get("reason_tags") or [])}


def settled_projection(state: dict[str, Any], conversation_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    loops = state.get("free_state_reasoning_loops") if isinstance(state.get("free_state_reasoning_loops"), dict) else {}
    loop = loops.get(conversation_id) if isinstance(loops.get(conversation_id), dict) else {}
    return loop, persisted_task_semantic_state(state, conversation_id)


def assert_settled_projection(loop: dict[str, Any], semantic: dict[str, Any], disposition: str, evidence_id: str, expected_outcome: str) -> None:
    expected = SETTLEMENT_EXPECTATIONS[disposition]
    require(bool(loop), "settled free-state loop was not persisted")
    require(first_text(loop.get("status")).lower() == "completed", "settled loop status mismatch: " + first_text(loop.get("status")))
    # NOTE: loop.last_error may legitimately carry the boundary-park message
    # ("experiment round is waiting for the human judgment boundary") -- that
    # is the designed stop point, not a settlement defect.
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    require(first_text(experiment.get("status")).lower() == "settled", "experiment status mismatch: " + first_text(experiment.get("status")))
    require(first_text(experiment.get("outcome")).lower() == expected_outcome, f"experiment outcome mismatch for {disposition}: " + first_text(experiment.get("outcome")))
    rounds = rows(experiment.get("rounds"))
    require(len(rounds) == 1, "settlement must not open a second experiment round")
    judgments = rows(rounds[0].get("user_judgment_evidence"))
    require(judgments and first_text(judgments[-1].get("id")) == evidence_id, "persisted judgment evidence identity mismatch")
    require(SETTLEMENT_PROBE_TAG in probe_tags(judgments[-1]), "persisted judgment lost the machine-origin probe marker")
    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    require(d1_receipt.get("settled") is True, "d1 receipt did not record settlement")
    for flag in ("human_confirmed", "ambiguous", "rolled_back"):
        require(d1_receipt.get(flag) is expected[flag], f"d1 receipt {flag} mismatch for {disposition}: {d1_receipt.get(flag)!r}")
    require(first_text(d1_receipt.get("disposition")).lower() == expected["disposition"], "d1 receipt disposition mismatch: " + first_text(d1_receipt.get("disposition")))
    require(int(d1_receipt.get("forward_mutation_count", 0) or 0) == 1, "settlement must not add forward mutations")
    require(int(d1_receipt.get("rollback_compensation_count", -1) or 0) == (1 if disposition == "rollback" else 0), "rollback compensation cardinality mismatch for " + disposition)
    layers = d1_receipt.get("layers") if isinstance(d1_receipt.get("layers"), dict) else {}
    human_ab = layers.get("human_ab") if isinstance(layers.get("human_ab"), dict) else {}
    require(first_text(human_ab.get("status")).lower() == "decided", "human A/B layer was not decided")
    require(first_text(semantic.get("state")).lower() == "settled", "canonical task semantic state is not settled: " + first_text(semantic.get("state")))
    require(semantic.get("terminal") is True, "canonical task semantic state is not terminal")


def run_settlement_probe(base_url: str, conversation_id: str, project_path: str, validation: dict[str, Any], disposition: str, timeout: float, run_started: float) -> dict[str, Any]:
    answers = SETTLEMENT_PROBE_ANSWERS[disposition]
    payload = {
        "conversation_id": conversation_id,
        "turn_id": first_text(validation.get("turn_id")),
        "round_id": first_text(validation.get("round_id")),
        "audition_session_id": first_text(validation.get("audition_session_id")),
        "project_revision": first_text(validation.get("after_revision")),
        "heard_difference": answers["heard_difference"],
        "preference": answers["preference"],
        "reason_tags": [SETTLEMENT_PROBE_TAG],
        "free_text": SETTLEMENT_PROBE_FREE_TEXT,
    }
    for key in ("turn_id", "round_id", "audition_session_id", "project_revision"):
        require(payload[key], f"settlement probe is missing {key} from the D1 validation receipt")
    # The agent establishes the durable judgment boundary asynchronously after
    # audition.ready. The WebUI waits for trajectory.user_judgment.requested;
    # the probe accepts the equivalent durable state: the round binding, or
    # (task semantic human_judgment_required + round decision user_judgment_pending)
    # -- the handler re-establishes a dropped round binding through the guarded
    # judgment-request API.
    boundary_deadline = time.monotonic() + 60
    boundary_bound = False
    while not boundary_bound and time.monotonic() < boundary_deadline:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, run_started)
        loop_now, semantic_now = settled_projection(state, conversation_id)
        rounds_now = rows((loop_now.get("experiment") or {}).get("rounds")) if isinstance(loop_now.get("experiment"), dict) else []
        if rounds_now:
            round_now = rounds_now[0]
            bound_to_session = round_now.get("user_judgment_requested") is True and first_text(round_now.get("audition_session_id")) == payload["audition_session_id"]
            parked_at_boundary = first_text(semantic_now.get("state")).lower() == "human_judgment_required" and first_text(round_now.get("decision")).lower() == "user_judgment_pending"
            boundary_bound = bound_to_session or parked_at_boundary
        if not boundary_bound:
            time.sleep(1)
    require(boundary_bound, "durable judgment boundary was not established before the probe judgment")
    judged = request_json("POST", base_url.rstrip("/") + "/agent/audition/judgment", payload, timeout)
    require(first_text(judged.get("status")).lower() == "ok", "settlement probe judgment was rejected: " + first_text(judged.get("error")))
    evidence = judged.get("evidence") if isinstance(judged.get("evidence"), dict) else {}
    evidence_id = first_text(evidence.get("id"))
    require(evidence_id, "settlement probe judgment returned no evidence identity")
    require(SETTLEMENT_PROBE_TAG in probe_tags(evidence), "settlement probe marker was not retained on the judgment response")

    # The judgment settles and persists synchronously before the HTTP response
    # returns; the retry window only tolerates a slow disk flush.
    deadline = time.monotonic() + 15
    loop: dict[str, Any] = {}
    semantic: dict[str, Any] = {}
    while True:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, run_started)
        loop, semantic = settled_projection(state, conversation_id)
        if (first_text(loop.get("status")).lower() == "completed" and semantic) or time.monotonic() >= deadline:
            break
        time.sleep(1)
    assert_settled_projection(loop, semantic, disposition, evidence_id, SETTLEMENT_EXPECTATIONS[disposition]["outcome"])

    state_now = invoke(base_url, "project.state", {}, timeout)
    revision_now = project_revision(state_now)
    if disposition == "rollback":
        require(revision_now != payload["project_revision"], "rollback did not move the project revision off the treatment")
    else:
        require(revision_now == payload["project_revision"], disposition + " settlement changed the project revision")
    return {
        "disposition": disposition,
        "probe_origin": True,
        "evidence_id": evidence_id,
        "task_semantic_state": first_text(semantic.get("state")),
        "experiment_status": first_text((loop.get("experiment") or {}).get("status")) if isinstance(loop.get("experiment"), dict) else "",
        "experiment_outcome": SETTLEMENT_EXPECTATIONS[disposition]["outcome"],
        "project_revision": revision_now,
    }


def verify_settled_after_restart(base_url: str, report_path: Path, timeout: float) -> dict[str, Any]:
    prior = json.loads(report_path.read_text(encoding="utf-8"))
    probe = prior.get("settlement_probe") if isinstance(prior.get("settlement_probe"), dict) else {}
    require(probe.get("probe_origin") is True, "report has no settlement probe section to verify")
    disposition = first_text(probe.get("disposition"))
    expected_outcome = SETTLEMENT_EXPECTATIONS.get(disposition, {}).get("outcome", "")
    evidence_id = first_text(probe.get("evidence_id"))
    conversation_id = first_text(prior.get("conversation_id"))
    setup = prior.get("project_setup") if isinstance(prior.get("project_setup"), dict) else {}
    project_path = first_text(setup.get("project_path"))
    started = float(prior.get("started_at_epoch") or 0)
    require(disposition and expected_outcome and evidence_id and conversation_id and project_path and started > 0, "prior report is missing settlement identity")

    # Reactivating the workspace drives the restart reconciliation path
    # (state restore + semantic projection reconcile) before asserting.
    invoke(base_url, "project.open", {"file_path": project_path, "project_path": project_path}, timeout, confirmed=True)
    deadline = time.monotonic() + 15
    loop: dict[str, Any] = {}
    semantic: dict[str, Any] = {}
    while True:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, started)
        loop, semantic = settled_projection(state, conversation_id)
        if (loop and semantic) or time.monotonic() >= deadline:
            break
        time.sleep(1)
    assert_settled_projection(loop, semantic, disposition, evidence_id, expected_outcome)

    continuations = continuation_rows(base_url, conversation_id, min(timeout, 30))
    resurrected = [item for item in continuations if first_text(item.get("status")).lower() not in TERMINAL_CONTINUATION_STATUSES]
    require(not resurrected, "restart resurrected non-terminal continuations: " + json.dumps([{ "status": item.get("status"), "error": item.get("last_error")} for item in resurrected], ensure_ascii=False))
    return {
        "verified_at": dt.datetime.now(dt.timezone.utc).isoformat(),
        "disposition": disposition,
        "task_semantic_state": first_text(semantic.get("state")),
        "experiment_outcome": expected_outcome,
        "continuations_total": len(continuations),
    }


MULTI_ROUND_MIN_ROUNDS = 2
# D2-1 domain-table absolute dose ceiling mirrored for runner-side probing;
# admission itself stays the agent's responsibility (same stance as the
# ADMITTED_DOMAIN_KINDS gate above).
MULTI_ROUND_DOSE_ABS_LIMIT_DB = 2.0


class MultiRoundNotExercised(RuntimeError):
    """The probe could not observe an autonomous multi-round experiment."""


def numeric_receipt_field(source: dict[str, Any], *keys: str) -> float | None:
    for key in keys:
        value = source.get(key)
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            return float(value)
    return None


MULTI_ROUND_PROBE_TAG = "smoke_multi_round_probe"
MULTI_ROUND_PROBE_FREE_TEXT = "machine-originated multi-round probe judgment; not a human judgment"
# The judgment park leaves the goal waiting for a user chat turn: the POST's
# scheduler wake only retries queued continuations, and none is queued for the
# fresh calibration round (20260829_095609: round 2 opened and then the
# projection froze for the whole wait window). The drive therefore stands in
# for the user's "continue" and answers any experiment-flow confirmation the
# recalibration turn surfaces, exactly like the main driver's parked-state
# resume.
MULTI_ROUND_DRIVE_MAX_NUDGES = 3
MULTI_ROUND_DRIVE_NUDGE_INTERVAL_SEC = 120.0


def multi_round_pending_round(loop: dict[str, Any]) -> dict[str, Any]:
    """First experiment round parked on a user judgment with no judgment
    evidence recorded yet. Accepts both durable boundary shapes: the round
    that actively requested the judgment, and the round whose request binding
    a scheduler transport replay dropped while the decision stayed pending."""
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    for round_row in rows(experiment.get("rounds")):
        requested = round_row.get("user_judgment_requested") is True
        decision_pending = first_text(round_row.get("decision")).lower() == "user_judgment_pending"
        if (requested or decision_pending) and not rows(round_row.get("user_judgment_evidence")):
            return round_row
    return {}


def run_multi_round_judgment_drive(base_url: str, conversation_id: str, project_path: str, timeout: float, case_id: str, run_started: float) -> dict[str, Any]:
    """D2-2-S3b deterministic drive: one machine-origin judgment at the round-1 boundary.

    Round 1 parks at the human A/B judgment boundary with no further scheduler
    work queued, so a multi-round admission can never continue on its own.
    This drive submits exactly one machine-originated "no audible difference"
    judgment through the same guarded /agent/audition/judgment channel as the
    settlement probe; reason_tags + free_text permanently mark it as
    machine-originated and it never claims a human decision. The runtime's
    frozen ruling-1 path ("no audible difference; recalibrate") then opens
    round 2; a continue nudge (the driver's parked-state resume) then lets the
    scheduler execute the second single-change intervention. The drive waits
    for round 2 to reach its own stable stop state (applied
    intervention + fresh post-action observation + round decision/boundary)
    before validate_d2_multi_round asserts the frozen acceptance scope.

    Variance before the judgment (the run never forms the boundary, or the
    experiment settles without one) raises MultiRoundNotExercised (exit 3,
    rerun mechanism unchanged). After the judgment POST is accepted, round
    continuation is runtime contract, so a stall there is an honest failure.
    """
    boundary_deadline = time.monotonic() + timeout
    payload: dict[str, Any] = {}
    while time.monotonic() < boundary_deadline and not payload:
        state = newest_agent_runtime_state_for_conversation(project_path, conversation_id, run_started)
        loop_now, semantic_now = settled_projection(state, conversation_id)
        experiment_now = loop_now.get("experiment") if isinstance(loop_now.get("experiment"), dict) else {}
        payload = multi_round_boundary_payload(experiment_now, loop_now, semantic_now)
        if payload:
            # Do not spend the machine judgment on a single-round admission:
            # below the multi-round minimum the frozen runtime contract settles
            # any judgment without opening a next round, so a continuation is
            # not exercisable this run (admission variance -> NOT_EXERCISED).
            admission_now = experiment_now.get("admission") if isinstance(experiment_now.get("admission"), dict) else {}
            budget_now = int(admission_now.get("experiment_budget", 0) or 0)
            if budget_now < MULTI_ROUND_MIN_ROUNDS:
                raise MultiRoundNotExercised(f"{case_id} multi-round probe: admission granted experiment_budget {budget_now}; no multi-round continuation is exercisable")
            break
        if first_text(experiment_now.get("status")).lower() in {"settled", "stopped"}:
            # No further round can open after a settlement, so the boundary
            # this drive waits for can never form: report the variance.
            raise MultiRoundNotExercised(f"{case_id} multi-round probe: experiment settled/stopped before a judgment boundary formed (status={first_text(experiment_now.get('status'))})")
        time.sleep(2)
    if not payload:
        raise MultiRoundNotExercised(f"{case_id} multi-round probe: round 1 never reached the human judgment boundary within the smoke timeout")

    for key in ("turn_id", "round_id", "audition_session_id", "project_revision"):
        require(payload[key], f"multi-round probe drive is missing {key} from the persisted boundary projection")
    judged = request_json("POST", base_url.rstrip("/") + "/agent/audition/judgment", payload, timeout)
    require(first_text(judged.get("status")).lower() == "ok", "multi-round probe judgment was rejected: " + first_text(judged.get("error")))
    evidence = judged.get("evidence") if isinstance(judged.get("evidence"), dict) else {}
    evidence_id = first_text(evidence.get("id"))
    require(evidence_id, "multi-round probe judgment returned no evidence identity")
    require(MULTI_ROUND_PROBE_TAG in probe_tags(evidence), "multi-round probe marker was not retained on the judgment response")

    round_two_deadline = time.monotonic() + timeout
    round_two: dict[str, Any] = {}
    last_observation: dict[str, Any] = {}
    nudges = 0
    next_action_at = time.monotonic()
    while time.monotonic() < round_two_deadline:
        loop_now = persisted_free_state_loop(project_path, conversation_id, run_started)
        experiment_now = loop_now.get("experiment") if isinstance(loop_now.get("experiment"), dict) else {}
        rounds_now = rows(experiment_now.get("rounds"))
        loop_status_now = first_text(loop_now.get("status")).lower()
        last_observation = {
            "rounds_observed": len(rounds_now),
            "round_decisions": [first_text(row.get("decision")) for row in rounds_now],
            "experiment_status": first_text(experiment_now.get("status")),
            "loop_status": loop_status_now,
            "loop_last_error": first_text(loop_now.get("last_error")),
        }
        if len(rounds_now) >= 2 and loop_status_now in {"capability_blocked", "completed", "cancelled", "failed", "stopped", "no_candidate_found"}:
            # A scheduler-side closure settlement (audio_closure_controller
            # admitAudioClosureRound: "closure observation round boundary
            # reached") can terminalize the loop between the recalibration
            # judgment and round 2's intervention. Waiting longer cannot
            # revive it; fail immediately with the boundary identity.
            raise RuntimeError(
                "multi-round probe drive: the loop was terminalized before round 2 executed its intervention (status="
                + loop_status_now
                + "; last_error="
                + first_text(loop_now.get("last_error"))
                + "); observed="
                + json.dumps(last_observation, ensure_ascii=False)
            )
        if len(rounds_now) >= 2:
            round_two_row = rounds_now[1]
            interventions = rows(round_two_row.get("interventions"))
            receipt = interventions[0].get("receipt") if interventions and isinstance(interventions[0].get("receipt"), dict) else {}
            post_observations = [item for item in rows(round_two_row.get("observations")) if item.get("post_action") is True]
            round_two = {
                "round_two_open": True,
                "round_two_round_id": first_text(round_two_row.get("round_id")),
                "round_two_interventions": len(interventions),
                "round_two_post_action_observations": len(post_observations),
                "round_two_decision": first_text(round_two_row.get("decision")),
                "round_two_transaction_id": first_text(receipt.get("transaction_id")),
                "stable": bool(
                    first_text(receipt.get("transaction_id"))
                    and post_observations
                    and (round_two_row.get("user_judgment_requested") is True or first_text(round_two_row.get("decision")) or rows(round_two_row.get("user_judgment_evidence")))
                ),
            }
            # A round-2 decision without any intervention is a terminal shape;
            # hand it to validate_d2_multi_round for the precise frozen-scope
            # failure instead of spinning to the deadline.
            if round_two["stable"] or (not interventions and first_text(round_two_row.get("decision"))):
                break
        if nudges < MULTI_ROUND_DRIVE_MAX_NUDGES and time.monotonic() >= next_action_at:
            nudges += 1
            next_action_at = time.monotonic() + MULTI_ROUND_DRIVE_NUDGE_INTERVAL_SEC
            nudge = request_json("POST", base_url.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": CONTINUE_NUDGE_MESSAGE, "context": {"agent_mode": "chat"}}, timeout)
            interaction = recommended_interaction(nudge, True, True, True, {})
            if interaction is not None:
                request_json("POST", base_url.rstrip("/") + "/agent/interaction/respond", interaction, timeout)
        time.sleep(2)
    if not round_two:
        raise RuntimeError("multi-round probe drive: round 2 did not reach a stable stop state after the recalibration judgment; observed=" + json.dumps(last_observation, ensure_ascii=False))
    if not round_two["stable"]:
        # Let validate_d2_multi_round produce the exact assertion failure.
        report_note = dict(round_two)
        report_note["stable"] = False
        return {"probe_origin": True, "judgment": {"heard_difference": "no", "preference": "unsure"}, "evidence_id": evidence_id, "judged_round_id": payload["round_id"], "round_two_observed": report_note, "round_two_nudges": nudges}
    return {
        "probe_origin": True,
        "judgment": {"heard_difference": "no", "preference": "unsure"},
        "evidence_id": evidence_id,
        "judged_round_id": payload["round_id"],
        "round_two_observed": round_two,
        "round_two_nudges": nudges,
    }


def multi_round_boundary_payload(experiment: dict[str, Any], loop: dict[str, Any], semantic: dict[str, Any]) -> dict[str, Any]:
    """Judgment payload for the parked boundary round, or {} while the boundary
    is not durably established. Accepts the same equivalent durable states as
    the settlement probe: the session-bound round, or the parked
    human_judgment_required canonical state with the round decision pending."""
    round_row = multi_round_pending_round({"experiment": experiment})
    if not round_row:
        return {}
    session = loop.get("audition_session_snapshot") if isinstance(loop.get("audition_session_snapshot"), dict) else {}
    # The judgment handler matches the request session against the loop-level
    # durable audition identity, so that is the identity to send.
    session_id = first_text(loop.get("audition_session_id"), session.get("session_id"))
    bound_to_session = bool(session_id) and first_text(round_row.get("audition_session_id")) == session_id
    parked_at_boundary = first_text(semantic.get("state")).lower() == "human_judgment_required" and first_text(round_row.get("decision")).lower() == "user_judgment_pending"
    if not (bound_to_session or parked_at_boundary):
        return {}
    interventions = rows(round_row.get("interventions"))
    receipt = interventions[-1].get("receipt") if interventions and isinstance(interventions[-1].get("receipt"), dict) else {}
    return {
        "conversation_id": first_text(loop.get("conversation_id")),
        "turn_id": first_text(experiment.get("turn_id")),
        "round_id": first_text(round_row.get("round_id")),
        "audition_session_id": session_id,
        "project_revision": first_text(session.get("project_revision"), receipt.get("after_revision"), receipt.get("applied_revision")),
        "heard_difference": "no",
        "preference": "unsure",
        "reason_tags": [MULTI_ROUND_PROBE_TAG],
        "free_text": MULTI_ROUND_PROBE_FREE_TEXT,
    }


def validate_d2_multi_round(base_url: str, conversation_id: str, project_path: str, responses: list[dict[str, Any]], timeout: float, case_id: str, run_started: float) -> dict[str, Any]:
    """D2-2-S3 multi-round probe: independent assertions, default-D1 path intact.

    Runs against the freshest free_state_reasoning_loop.v1 projection (the
    persisted authoritative copy competes with response snapshots by
    updated_at) and asserts the frozen acceptance scope from
    docs/FREE_STATE_PHASE_D_D2_2_SURVEY_2026-08-26.md §5/§8 ruling 4, without
    modifying validate_d1 / assert_settled_projection:

      - round count agrees with admission.experiment_budget; more rounds than
        budget is always fatal,
      - the run terminates either with the budget exhausted (rounds used ==
        budget, no parked judgment) or parked at exactly one
        user_judgment_pending boundary as the final round (no round may follow
        a pending verdict),
      - every round carries exactly one forward intervention with distinct
        chained before/after revisions and unique transaction identities, and
        the persisted forward_mutation_count equals the round count,
      - cross-round cumulative dose never exceeds the D2-1 absolute bounds
        (track_gain |ΣΔ| <= 2 dB; static_eq per plugin/param/band |ΣΔ| <= 2 dB),
      - restart idempotency: the projection persisted on disk repeats neither
        rounds nor interventions relative to the validated snapshot, no
        non-terminal continuation resurrects, and each round's post-action CCB
        observation (the second round's revision pin included) is fresh and
        bound to that round's after_revision,
      - budget-exhausted runs record budget_exhausted=true; deeper stop_reason
        pinning stays deferred behind the TODO below.

    Dose deltas prefer an explicit applied/gain delta on the execution receipt
    and otherwise fall back to the numeric readback, treated as the round's
    applied delta for that band.

    TODO(D2-2-S3b): resolved -- run_multi_round_judgment_drive now supplies the
    deterministic round-1 judgment ("no audible difference" recalibration), so
    an admitted budget>=2 run is expected to continue into round 2. This
    function still raises MultiRoundNotExercised when fewer than two rounds
    are observed (pre-boundary variance; the caller reports NOT_EXERCISED,
    exit 3, and records it). No assertion here is ever relaxed to manufacture
    a pass (sealed-test discipline).
    """
    persisted = persisted_free_state_loop(project_path, conversation_id, run_started)
    loop = find_d1_loop(responses, authoritative=persisted or None)
    require(loop is not None, f"{case_id} multi-round probe found no admitted free-state loop projection")
    assert loop is not None
    experiment = loop.get("experiment") if isinstance(loop.get("experiment"), dict) else {}
    admission = experiment.get("admission") if isinstance(experiment.get("admission"), dict) else {}
    typed = admission.get("typed_action") if isinstance(admission.get("typed_action"), dict) else {}
    domain = first_text(typed.get("action_domain")).lower()
    require(domain in ADMITTED_DOMAIN_KINDS, f"multi-round probe: action_domain {domain!r} is not admitted by the D2-1 domain table")
    budget = int(admission.get("experiment_budget", 0) or 0)
    require(budget >= MULTI_ROUND_MIN_ROUNDS, f"multi-round probe: experiment_budget {budget} admits no multi-round continuation")
    # Frozen S1 contract: each dose scope keeps exactly one action attempt per
    # round (ValidateD2MultiRound rejects anything but 1); cross-round
    # continuation is expressed solely by experiment_budget above. Demanding
    # attempts >= 2 here misread the per-round bound as a cross-round one and
    # fail-closed the 20260828_202855 budget-2 run at its admission boundary.
    for key in ("diagnostic_dose_bounds", "retained_dose_bounds"):
        bounds = admission.get(key) if isinstance(admission.get(key), dict) else {}
        attempts = int(bounds.get("max_action_attempts", 0) or 0)
        require(attempts == 1, f"multi-round probe: {key}.max_action_attempts ({attempts}) violates the frozen per-round single-change contract (must equal 1)")

    rounds_all = rows(experiment.get("rounds"))
    if len(rounds_all) < MULTI_ROUND_MIN_ROUNDS:
        raise MultiRoundNotExercised(f"{case_id} multi-round probe observed {len(rounds_all)} experiment round(s); the run never autonomously entered multi-round continuation")

    used = len(rounds_all)
    require(used <= budget, f"multi-round probe: {used} rounds exceed experiment_budget {budget}")
    pending_indices = [index for index, round_row in enumerate(rounds_all) if first_text(round_row.get("decision")).lower() == "user_judgment_pending"]
    require(len(pending_indices) <= 1, f"multi-round probe: {len(pending_indices)} rounds park on a pending user judgment")
    for index in pending_indices:
        require(index == used - 1, f"multi-round probe: round {index} judged pending but round {index + 1} still executed")
    budget_exhausted = not pending_indices
    if budget_exhausted:
        require(used == budget, f"multi-round probe: experiment stopped after {used} of {budget} budgeted rounds without a pending boundary")

    round_ids = [first_text(row.get("round_id")) for row in rounds_all]
    require(all(round_ids) and len(set(round_ids)) == used, "multi-round probe: round identities are missing or duplicated")

    previous_after_revision = ""
    transaction_ids: set[str] = set()
    intervention_ids: set[str] = set()
    per_band_totals: dict[str, float] = {}
    domain_total_delta = 0.0
    for index, round_row in enumerate(rounds_all):
        interventions = rows(round_row.get("interventions"))
        require(len(interventions) == 1, f"multi-round probe: round {index} carries {len(interventions)} forward interventions (exactly one required)")
        intervention = interventions[0]
        receipt = intervention.get("receipt") if isinstance(intervention.get("receipt"), dict) else {}
        before_revision = first_text(receipt.get("before_revision"))
        after_revision = first_text(receipt.get("after_revision"), receipt.get("applied_revision"))
        require(before_revision and after_revision and before_revision != after_revision, f"multi-round probe: round {index} receipt lacks distinct before/after revisions")
        if index > 0:
            require(before_revision == previous_after_revision, f"multi-round probe: round {index} did not apply onto round {index - 1}'s after_revision")
        previous_after_revision = after_revision
        transaction_id = first_text(receipt.get("transaction_id"))
        require(transaction_id and transaction_id not in transaction_ids, f"multi-round probe: round {index} transaction identity is missing or duplicated")
        transaction_ids.add(transaction_id)
        # Frozen durable-identity contract (d1MultiRoundRoundScopeSuffix):
        # round 1 keeps the experiment-level action identity byte-for-byte and
        # every later round appends _round_<n>, so each round owns a distinct
        # journal action and idempotent transaction chain.
        intervention_id = first_text(intervention.get("action_id"))
        require(intervention_id and intervention_id not in intervention_ids, f"multi-round probe: round {index} intervention action identity is missing or duplicated")
        round_number = int(round_row.get("number") or 0) or (index + 1)
        if round_number > 1:
            require(f"_round_{round_number}" in intervention_id, f"multi-round probe: round {index} action identity {intervention_id!r} lacks the durable _round_{round_number} scope suffix")
        intervention_ids.add(intervention_id)

        post_observations = [item for item in rows(round_row.get("observations")) if item.get("post_action") is True]
        require(post_observations, f"multi-round probe: round {index} has no post-action CCB observation")
        post = post_observations[-1]
        require(post.get("fresh") is True and first_text(post.get("project_revision")) == after_revision, f"multi-round probe: round {index} post-action observation is not fresh and revision-bound")

        delta_keys = ("applied_delta_db", "gain_delta_db", "actual_readback_db") if domain == "track_gain" else ("applied_delta_db", "gain_delta_db", "actual_readback_value")
        delta = numeric_receipt_field(receipt, *delta_keys)
        if delta is None:
            delta = numeric_receipt_field(intervention, *delta_keys)
        require(delta is not None, f"multi-round probe: round {index} exposes no numeric dose delta for {domain}")
        domain_total_delta += float(delta)
        if domain == "static_eq":
            frequency = numeric_receipt_field(receipt, "frequency_hz")
            if frequency is None:
                frequency = numeric_receipt_field(typed, "frequency_hz")
            band_key = first_text(receipt.get("plugin_id")) + ":" + first_text(receipt.get("param_id")) + ":" + ("" if frequency is None else format(frequency, "g"))
            per_band_totals[band_key] = per_band_totals.get(band_key, 0.0) + float(delta)

    if domain == "track_gain":
        require(abs(domain_total_delta) <= MULTI_ROUND_DOSE_ABS_LIMIT_DB, f"multi-round probe: cross-round track_gain cumulative |{domain_total_delta:g}|dB exceeds the 2dB absolute bound")
        cumulative_dose = {"domain_total_delta_db": domain_total_delta}
    else:
        worst_band = max((abs(value) for value in per_band_totals.values()), default=0.0)
        require(worst_band <= MULTI_ROUND_DOSE_ABS_LIMIT_DB, f"multi-round probe: cross-round static_eq cumulative band delta {worst_band:g}dB exceeds the 2dB absolute bound")
        cumulative_dose = {"per_band_total_delta_db": per_band_totals}
    d1_receipt = loop.get("d1_receipt") if isinstance(loop.get("d1_receipt"), dict) else {}
    require(int(d1_receipt.get("forward_mutation_count", 0) or 0) == used, f"multi-round probe: persisted forward mutation count disagrees with one-forward-change-per-round across {used} rounds")

    # Restart idempotency: the projection persisted on disk must repeat neither
    # rounds nor interventions relative to the validated snapshot.
    require(bool(persisted), "multi-round probe: no persisted projection survived for the restart-idempotency check")
    persisted_experiment = persisted.get("experiment") if isinstance(persisted.get("experiment"), dict) else {}
    persisted_rounds = rows(persisted_experiment.get("rounds"))
    require(len(persisted_rounds) == used, f"multi-round probe: persisted projection holds {len(persisted_rounds)} rounds versus {used} observed (restart duplication/loss)")
    persisted_transactions: set[str] = set()
    for persisted_round in persisted_rounds:
        for persisted_intervention in rows(persisted_round.get("interventions")):
            persisted_receipt = persisted_intervention.get("receipt") if isinstance(persisted_intervention.get("receipt"), dict) else {}
            identity = first_text(persisted_receipt.get("transaction_id"))
            if identity:
                persisted_transactions.add(identity)
    require(persisted_transactions and persisted_transactions == transaction_ids, "multi-round probe: persisted interventions differ from the observed transaction set (restart duplication/loss)")

    continuations = continuation_rows(base_url, conversation_id, min(timeout, 30))
    resurrected = [item for item in continuations if first_text(item.get("status")).lower() not in TERMINAL_CONTINUATION_STATUSES]
    require(not resurrected, "multi-round probe: restart resurrected non-terminal continuations: " + json.dumps([{"status": item.get("status"), "error": item.get("last_error")} for item in resurrected], ensure_ascii=False))

    return {
        "status": "pass",
        "public_case_id": case_id,
        "conversation_id": conversation_id,
        "action_domain": domain,
        "experiment_budget": budget,
        "rounds_used": used,
        "round_ids": round_ids,
        "budget_exhausted": budget_exhausted,
        "pending_boundary_round": pending_indices[0] if pending_indices else None,
        "interventions_total": len(transaction_ids),
        "cumulative_dose": cumulative_dose,
        "cross_round_dose_abs_limit_db": MULTI_ROUND_DOSE_ABS_LIMIT_DB,
        "restart_idempotent": True,
    }


def write_report(path: Path, report: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--public-manifest", required=True)
    parser.add_argument("--public-case-id", default=DEFAULT_CASE_ID)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=600)
    parser.add_argument("--project-workdir", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--admission-only", action="store_true", help="stop at FS7/admission boundary before confirmation or mutation")
    parser.add_argument("--settlement-probe", choices=sorted(SETTLEMENT_PROBE_ANSWERS), default="", help="after the boundary validation, submit a machine-origin judgment and machine-check the settlement receipt; the default mode without this flag keeps asserting settled is False")
    parser.add_argument("--prompt-flavor", choices=sorted(PROMPT_FLAVORS), default="neutral", help="case-agnostic open-prompt flavor; frequency steers the proposal toward the admitted static_eq domain")
    parser.add_argument("--expect-domain", choices=sorted(ADMITTED_DOMAIN_KINDS) + ["any"], default="any", help="require the run to autonomously select this admitted domain (regression pin) or any admitted domain")
    parser.add_argument("--multi-round-probe", action="store_true", help="D2-2-S3 multi-round probe: drive one machine-origin judgment at the round-1 boundary (D2-2-S3b), then assert multi-round continuation against the D2-1 dose bounds; a run that never forms the boundary may legally report NOT_EXERCISED (exit 3)")
    parser.add_argument("--expect-honest-refusal", action="store_true", help="D2-REG3 A face: expect the confirmed execution to be refused by the fail-closed unreachable-range gate and validate the REG2 honest-refusal signature (four requirements incl. the StopProjectRevisionStale ban) instead of the applied-path D1 contract")
    parser.add_argument("--verify-settled", default="", help="verify a previously settled probe report after an agent restart (path to d1_smoke_report.json)")
    args = parser.parse_args()
    if args.multi_round_probe and args.settlement_probe:
        parser.error("--multi-round-probe cannot be combined with --settlement-probe")
    if args.multi_round_probe and args.admission_only:
        parser.error("--multi-round-probe cannot be combined with --admission-only")
    if args.expect_honest_refusal and (args.multi_round_probe or args.settlement_probe or args.admission_only):
        parser.error("--expect-honest-refusal owns the run tail and cannot be combined with --multi-round-probe, --settlement-probe, or --admission-only")
    output = Path(args.output).resolve()
    if args.verify_settled:
        verification = verify_settled_after_restart(args.agent_http, Path(args.verify_settled).resolve(), args.timeout_sec)
        prior_path = Path(args.verify_settled).resolve()
        prior = json.loads(prior_path.read_text(encoding="utf-8"))
        prior["restart_verification"] = verification
        write_report(prior_path, prior)
        print(f"D1-S1 SETTLEMENT RESTART VERIFY PASS: disposition={verification['disposition']} report={prior_path}")
        return 0
    report: dict[str, Any] = {"schema_version": "vit.free_state_d1_smoke.v1", "started_at": dt.datetime.now(dt.timezone.utc).isoformat(), "public_case_id": args.public_case_id, "prompt_flavor": args.prompt_flavor, "expect_domain": args.expect_domain, "expect_honest_refusal": args.expect_honest_refusal}
    responses: list[dict[str, Any]] = []
    try:
        _, public_case = load_public_case(Path(args.public_manifest), args.public_case_id)
        case = materialize_public_case(public_case, Path(args.project_workdir))
        report["public_source_project"] = str(Path(str(public_case["project_path"])).resolve())
        report["material_qualification"] = qualify_material(case, stereo_balance=args.prompt_flavor == "pan", limiter=args.prompt_flavor == "limiter", gate=args.prompt_flavor == "gate", multiband=args.prompt_flavor == "multiband")
        report["project_setup"] = prepare_project(args.agent_http, case, args.timeout_sec)
        report["started_at_epoch"] = time.time()
        ui_context_response = request_json("GET", args.agent_http.rstrip("/") + "/agent/ui/context", None, min(args.timeout_sec, 30))
        ui_context = ui_context_response.get("context") if isinstance(ui_context_response.get("context"), dict) else {}
        leaked_selection = FORBIDDEN_SELECTION_KEYS.intersection(ui_context)
        require(not leaked_selection, "D1 open-intent preflight inherited selected DAW targets: " + str(sorted(leaked_selection)))
        require("dev_smoke_" not in json.dumps(ui_context, ensure_ascii=False).lower(), "D1 open-intent preflight inherited dev smoke context")
        report["ui_context_preflight"] = {"status": "clean", "keys": sorted(ui_context)}
        conversation_id = "d1_s1_" + dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%S%f")
        # Persist the identity before the model request so a transport or
        # scheduler timeout still leaves a traceable open-intent attempt.
        report["conversation_id"] = conversation_id
        write_report(output, report)
        run_started = float(report["started_at_epoch"])
        response = request_json("POST", args.agent_http.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": PROMPT_FLAVORS[args.prompt_flavor], "context": {"agent_mode": "chat"}}, args.timeout_sec)
        responses.append(response)
        report["responses"] = responses
        write_report(output, report)
        selected_domains: set[str] = set()
        experiment_proposal_approved = False
        experiment_applied = False
        confirmed_domain_ticks: dict[str, int] = {}
        continue_nudges = 0
        # Per-class budgets bound the driver (1 proposal approval, 2 same-id
        # experiment confirmations, 3 continue nudges); the loop ceiling only
        # needs headroom for the interleaved drains and re-surfaces.
        for _ in range(16):
            require("dev_smoke_" not in json.dumps(response, ensure_ascii=False).lower(), "D1 response inherited dev smoke target context")
            domains = {first_text(value).lower() for value in values_for_key(response, "action_domain")}
            kinds = {first_text(value).lower() for value in values_for_key(response, "action_kind")}
            for domain in domains & set(ADMITTED_DOMAIN_KINDS):
                selected_domains.add(domain)
                require(ADMITTED_DOMAIN_KINDS[domain] in kinds, f"model selected {domain} without {ADMITTED_DOMAIN_KINDS[domain]}")
            if args.admission_only:
                boundary = find_admission_boundary(responses)
                if boundary is not None:
                    validate_admission_only_boundary(boundary)
                    report.update({
                        "status": "admission_only",
                        "admission": {key: value for key, value in boundary.items() if key != "loop_row"},
                        "mutation_performed": False,
                        "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                        "responses": responses,
                    })
                    write_report(output, report)
                    print(f"D1-S1 ADMISSION_ONLY: report={output}")
                    return 0
            if (first_text(response.get("workflow")).lower() == "free_state_d1_s1" and first_text(response.get("goal_status")).lower() != "waiting_continue") or find_d1_loop(responses) is not None and any(bool(item.get("human_audition_ready")) for item in dicts(response)):
                break
            applied_data = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
            if first_text(response.get("workflow")).lower() == "free_state_d1_s1" and (applied_data.get("mutation_performed") is True or first_text(applied_data.get("status")).lower() == "applied"):
                experiment_applied = True
            if not experiment_applied:
                # The execution receipt can land inside a continuation without
                # any response carrying it; the persisted loop is authoritative.
                persisted = persisted_free_state_loop(report["project_setup"]["project_path"], conversation_id, run_started)
                persisted_receipt = persisted.get("d1_receipt") if isinstance(persisted.get("d1_receipt"), dict) else {}
                if persisted_receipt.get("parameter_applied") is True:
                    experiment_applied = True
            interaction = recommended_interaction(response, bool(selected_domains), experiment_proposal_approved, experiment_applied, confirmed_domain_ticks)
            if interaction is not None:
                approved_payload = interaction.get("payload") if isinstance(interaction.get("payload"), dict) else {}
                if first_text(approved_payload.get("kind")).lower() == "improvement_proposal_confirmation":
                    experiment_proposal_approved = True
                confirmed_id = first_text(interaction.get("interaction_id"))
                confirmed_kind = first_text(approved_payload.get("kind")).lower()
                if confirmed_id and confirmed_kind in GENERIC_MIX_CONFIRMATION_KINDS:
                    confirmed_domain_ticks[confirmed_id] = confirmed_domain_ticks.get(confirmed_id, 0) + 1
                response = request_json("POST", args.agent_http.rstrip("/") + "/agent/interaction/respond", interaction, args.timeout_sec)
                responses.append(response)
                report["responses"] = responses
                write_report(output, report)
                continue
            if first_text(response.get("goal_status")).lower() == "waiting_continue":
                drain = wait_scheduler_drain(args.agent_http, conversation_id, args.timeout_sec)
                continuation_state = drain["continuations"]
                report["continuation_timeline"] = drain["timeline"]
                report["last_runtime_status"] = drain["runtime_status"]
                report["continuations"] = continuation_state
                report["terminal_causes"] = drain["terminal_causes"]
                write_report(output, report)
                failed_continuations = [item for item in continuation_state if first_text(item.get("status")).lower() == "failed" or first_text(item.get("last_error"))]
                if failed_continuations:
                    raise RuntimeError("D1 durable continuation failed: " + first_text(failed_continuations[0].get("last_error")))
                loop = persisted_free_state_loop(report["project_setup"]["project_path"], conversation_id, run_started)
                require(bool(loop), "D1 scheduler drained without a persisted free-state loop")
                response = {
                    "goal_status": "scheduler_drained",
                    "workflow_data": {"free_state_reasoning_loop": loop},
                    "continuation_statuses": [first_text(item.get("status")) for item in continuation_state],
                }
                interaction_requests = continuation_interaction_requests(continuation_state)
                if interaction_requests:
                    response["interaction_requests"] = interaction_requests
                responses.append(response)
                continue
            if rows(response.get("interaction_requests")) and experiment_proposal_approved and continue_nudges < MAX_CONTINUE_NUDGES:
                # Only filtered-out generic suggestions are pending; the goal is
                # parked mid-experiment, so stand in for the user's next turn
                # with a neutral continue nudge and re-enter the loop with the
                # resulting response (the designed resume for waiting_continue).
                continue_nudges += 1
                response = request_json("POST", args.agent_http.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": CONTINUE_NUDGE_MESSAGE, "context": {"agent_mode": "chat"}}, args.timeout_sec)
                responses.append(response)
                report["responses"] = responses
                write_report(output, report)
                continue
            if rows(response.get("interaction_requests")):
                # Nudges exhausted: leave the suggestions unanswered and wait
                # for the experiment continuation the scheduler runs on its
                # own; break on timeout so validation reports the honest
                # terminal state.
                loop = poll_persisted_loop_for_audition(report["project_setup"]["project_path"], conversation_id, run_started, args.timeout_sec)
                if loop:
                    response = {"goal_status": "mix_suggestions_awaited", "workflow_data": {"free_state_reasoning_loop": loop}}
                    responses.append(response)
                    report["responses"] = responses
                    write_report(output, report)
                    continue
            break

        report["responses"] = responses
        if args.admission_only:
            boundary = find_admission_boundary(responses)
            if boundary is not None:
                validate_admission_only_boundary(boundary)
                report.update({
                    "status": "admission_only",
                    "admission": {key: value for key, value in boundary.items() if key != "loop_row"},
                    "mutation_performed": False,
                    "finished_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                })
                write_report(output, report)
                print(f"D1-S1 ADMISSION_ONLY: report={output}")
                return 0
        report["selected_domains"] = sorted(selected_domains)
        if args.expect_domain == "any":
            domain_exercised = bool(selected_domains)
            not_exercised_reason = f"{args.public_case_id} open run did not autonomously select an admitted D1-S1 domain ({', '.join(sorted(ADMITTED_DOMAIN_KINDS))})"
        else:
            domain_exercised = args.expect_domain in selected_domains
            not_exercised_reason = f"{args.public_case_id} open run did not autonomously select {args.expect_domain}"
        if not domain_exercised:
            report.update({"status": "not_exercised", "reason": not_exercised_reason})
            write_report(output, report)
            print(f"D1-S1 NOT_EXERCISED: report={output}")
            return NOT_EXERCISED_EXIT
        if args.expect_honest_refusal:
            # D2-REG3 A face: the domain-selection pin above still applies;
            # the tail swaps the applied-path D1 contract for the four
            # honest-refusal requirements (proposal formed, fail-closed
            # refusal, zero side effects, honest non-stale terminal).
            report["honest_refusal"] = validate_honest_refusal(
                find_d1_loop(responses),
                responses,
                report.get("continuations") or [],
                report.get("last_runtime_status") or {},
                report.get("terminal_causes") or [],
                args.public_case_id,
            )
            report["status"] = "honest_refusal_pass"
            report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
            write_report(output, report)
            print(f"D1-S1 HONEST_REFUSAL PASS: report={output}")
            return 0
        if args.multi_round_probe:
            # D2-2-S3: the multi-round probe owns the run tail; the single-round
            # D1 assertions below stay untouched and are intentionally not run.
            # D2-2-S3b: drive the round-1 boundary with one machine-origin
            # "no audible difference" judgment so the recalibration round can
            # open; validate_d2_multi_round then asserts the frozen scope.
            try:
                report["multi_round_drive"] = run_multi_round_judgment_drive(args.agent_http, conversation_id, report["project_setup"]["project_path"], args.timeout_sec, args.public_case_id, run_started)
                report["multi_round_probe"] = validate_d2_multi_round(args.agent_http, conversation_id, report["project_setup"]["project_path"], responses, args.timeout_sec, args.public_case_id, run_started)
            except MultiRoundNotExercised as exc:
                report.update({"status": "not_exercised", "reason": str(exc), "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
                write_report(output, report)
                print(f"D1-S1 NOT_EXERCISED(MULTI_ROUND): report={output}")
                return NOT_EXERCISED_EXIT
            report.update({"status": "multi_round_probe_pass", "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
            write_report(output, report)
            print(f"D1-S1 MULTI_ROUND_PROBE PASS: report={output}")
            return 0
        report["validation"] = validate_d1(args.agent_http, conversation_id, responses, args.timeout_sec, args.public_case_id)
        if args.settlement_probe:
            # The probe judgment is machine-originated and permanently marked
            # as such; it exercises the settlement machinery on this temporary
            # engineering copy only and never claims a human decision.
            report["settlement_probe"] = run_settlement_probe(args.agent_http, conversation_id, report["project_setup"]["project_path"], report["validation"], args.settlement_probe, args.timeout_sec, run_started)
            report["status"] = "settlement_probe_pass"
        else:
            report["status"] = "pass"
        report["finished_at"] = dt.datetime.now(dt.timezone.utc).isoformat()
        write_report(output, report)
        if args.settlement_probe:
            print(f"D1-S1 SETTLEMENT({args.settlement_probe}) PASS: report={output}")
        else:
            print(f"D1-S1 PASS: report={output}")
        return 0
    except KeyboardInterrupt:
        report.update({"status": "interrupted", "reason": "operator_interrupted", "responses": responses, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
        if report.get("project_setup") and report.get("conversation_id"):
            try:
                report["last_runtime_status"] = request_json("GET", args.agent_http.rstrip("/") + "/agent/runtime/status", None, 10)
                report["continuations"] = continuation_rows(args.agent_http, str(report["conversation_id"]), 10)
                report["persisted_loop"] = persisted_free_state_loop(report["project_setup"]["project_path"], str(report["conversation_id"]), float(report.get("started_at_epoch", time.time())))
            except Exception as snapshot_error:
                report["snapshot_error"] = str(snapshot_error)
        write_report(output, report)
        print(f"D1-S1 INTERRUPTED: report={output}", file=sys.stderr)
        return 130
    except Exception as exc:  # noqa: BLE001 - smoke reports fail closed.
        report.update({"status": "fail", "error": str(exc), "responses": responses, "finished_at": dt.datetime.now(dt.timezone.utc).isoformat()})
        if isinstance(exc, SchedulerDrainTimeout):
            report["continuation_timeline"] = exc.timeline
            report["last_runtime_status"] = exc.runtime_status
            report["continuations"] = exc.continuations
            report["terminal_causes"] = [first_text(item.get("free_state_stop_reason"), item.get("last_error"), item.get("status")) for item in exc.continuations]
        if report.get("project_setup"):
            persisted = persisted_free_state_loop(report["project_setup"]["project_path"], str(report.get("conversation_id", "")), float(report.get("started_at_epoch", time.time())))
            if persisted:
                report["persisted_loop"] = persisted
        write_report(output, report)
        print(f"D1-S1 FAIL: {exc}; report={output}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
