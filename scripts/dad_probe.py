import argparse
import json
import math
import mmap
import struct
import sys
import time
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional


SPECTRAL_FLOAT_STRIDE = 4
L3_ACOUSTIC_FEATURES = {
    "band_energy_summary",
    "stereo_relation_summary",
    "loudness_summary",
}


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def float_stats(values: Iterable[float]) -> Dict[str, Any]:
    sample_count = 0
    nonzero_count = 0
    nan_inf_count = 0
    sum_abs = 0.0
    max_abs = 0.0
    for value in values:
        sample_count += 1
        if not math.isfinite(value):
            nan_inf_count += 1
            continue
        abs_value = abs(value)
        if abs_value > 0:
            nonzero_count += 1
        sum_abs += abs_value
        if abs_value > max_abs:
            max_abs = abs_value
    return {
        "sample_count": sample_count,
        "nonzero_count": nonzero_count,
        "sum_abs": round(sum_abs, 6),
        "max_abs": round(max_abs, 9),
        "nan_inf_count": nan_inf_count,
    }


def posix_shm_name(memory_name: str) -> str:
    # Kernel SharedMemorySegmentPosix publishes unprefixed names (the exact
    # string the Windows side opens via tagname); the POSIX object resolves as
    # "/<published>". multiprocessing.shared_memory.SharedMemory prepends the
    # slash itself regardless of the input form, so the bare published name
    # must be passed through — a pre-slashed name would resolve to "//name",
    # a different object that never matches the kernel segment (empirical:
    # C shm_open("/X") vs SharedMemory(name="X") attach OK, name="/X") -> ENOENT).
    return memory_name.strip().lstrip("/")


def read_posix_shared_float32(memory_name: str, byte_count: int) -> List[float]:
    # POSIX counterpart of the tagname read below, aligned with the kernel's
    # SharedMemorySegmentPosix publisher and the harness shm_darwin reader:
    # attach an existing segment read-only-by-use, fail closed when the fstat
    # extent is smaller than the requested range (darwin rounds shm storage up
    # to 16 KiB and fstat reports the rounded extent, so only a logical
    # shortfall fails here), then close without unlinking — the kernel owns
    # the segment lifetime. multiprocessing.shared_memory is used instead of a
    # direct libc call because shm_open is variadic and ctypes passes the mode
    # argument under the wrong ABI on darwin arm64, which creates segments
    # with garbage permission bits.
    from multiprocessing import shared_memory

    segment = shared_memory.SharedMemory(name=posix_shm_name(memory_name))
    try:
        if segment.size < byte_count:
            raise ValueError(
                f"shared memory {memory_name} too small: have {segment.size} bytes, need {byte_count}"
            )
        data = bytes(segment.buf[:byte_count])
    finally:
        segment.close()
    return [item[0] for item in struct.iter_unpack("<f", data)]


def read_shared_float32(memory_name: str, float_count: int) -> List[float]:
    if not memory_name or float_count <= 0:
        raise ValueError("shared memory name and positive float_count are required")
    byte_count = float_count * 4
    if sys.platform == "win32":
        with mmap.mmap(-1, byte_count, tagname=memory_name, access=mmap.ACCESS_READ) as mm:
            data = mm.read(byte_count)
        return [item[0] for item in struct.iter_unpack("<f", data)]
    return read_posix_shared_float32(memory_name, byte_count)


def expected_float_count(event: Dict[str, Any]) -> int:
    value = int(float(event.get("float_count") or 0))
    if value > 0:
        return value
    shm_bytes = int(float(event.get("shm_bytes") or event.get("byte_count") or 0))
    if shm_bytes > 0 and shm_bytes % 4 == 0:
        return shm_bytes // 4
    feature_type = str(event.get("feature_type") or "")
    if feature_type == "waveform_envelope":
        frames = int(float(event.get("resolution_frame_count") or event.get("frame_count") or 0))
        stride = int(float(event.get("feature_stride") or 0))
        return frames * stride if frames > 0 and stride > 0 else 0
    frame_width = int(float(event.get("resolution_frame_width") or event.get("frame_count") or 0))
    bins = int(float(event.get("resolution_frequency_bins") or event.get("frequency_bins") or 0))
    return frame_width * bins * SPECTRAL_FLOAT_STRIDE if frame_width > 0 and bins > 0 else 0


def probe_event_shared_memory(event: Dict[str, Any]) -> Dict[str, Any]:
    memory_name = str(event.get("shared_memory") or "").strip()
    float_count = expected_float_count(event)
    out: Dict[str, Any] = {
        "shared_memory": memory_name,
        "float_count": float_count,
        "event_quality_status": event.get("quality_status"),
        "event_quality_reason": event.get("quality_reason"),
        "event_ready": event.get("ready"),
    }
    if not memory_name or float_count <= 0:
        out["status"] = "failed"
        out["reason"] = "missing_shared_memory_or_float_count"
        return out
    try:
        values = read_shared_float32(memory_name, float_count)
    except Exception as exc:
        out["status"] = "failed"
        out["reason"] = f"shared_memory_read_failed: {exc}"
        return out
    stats = float_stats(values)
    out.update(stats)
    if stats["nan_inf_count"] > 0:
        out["status"] = "failed"
        out["reason"] = "nan_or_inf_in_shared_memory"
    elif stats["nonzero_count"] <= 0:
        out["status"] = "suspect"
        out["reason"] = "shared_memory_all_zero"
    else:
        out["status"] = "ready"
        out["reason"] = "shared_memory_nonzero"
    return out


def event_matches_feature(event: Dict[str, Any], feature_type: str, track_id: str) -> bool:
    command = str(event.get("command") or event.get("cmd") or "")
    observed = str(event.get("feature_type") or "")
    observed_track = str(event.get("track_id") or event.get("source_track_id") or "")
    if track_id and observed_track and observed_track != track_id:
        return False
    if feature_type == "l2_render_probe":
        return observed == "l2_render_probe" and command in (
            "l2_render_probe_status",
            "l2_render_probe_ready",
        )
    if feature_type == "waveform_envelope":
        return command == "audio_feature_data_ready" and observed == "waveform_envelope"
    if feature_type == "spectral_field":
        return command == "tile_ready" and (observed == "spectral_field" or observed == "")
    if feature_type == "l3_acoustic_summary":
        return command == "audio_feature_data_ready" and observed in L3_ACOUSTIC_FEATURES
    if feature_type in L3_ACOUSTIC_FEATURES:
        return command == "audio_feature_data_ready" and observed == feature_type
    return False


def send_command(req_socket: Any, payload: Dict[str, Any]) -> Dict[str, Any]:
    req_socket.send_string(json.dumps(payload, ensure_ascii=False))
    raw = req_socket.recv_string()
    try:
        return json.loads(raw)
    except json.JSONDecodeError:
        return {"status": "error", "raw_reply": raw}


def build_warm_command(feature_type: str, material_path: Path, track_id: str, clip_id: str = "") -> Dict[str, Any]:
    if feature_type == "l2_render_probe":
        cmd: Dict[str, Any] = {
            "cmd": "l2_render_probe",
            "track_id": track_id,
            "tap_point": "track_post_fader",
            "render_mode": "offline_probe",
            "tail_seconds": 0.25,
        }
        if clip_id:
            cmd["clip_id"] = clip_id
        return cmd
    request_feature = "l3_acoustic_summary" if feature_type in L3_ACOUSTIC_FEATURES else feature_type
    cmd = {
        "cmd": "warm_waveform_bake",
        "track_id": track_id,
        "file_path": str(material_path),
        "allow_file_source": True,
        "feature_type": request_feature,
        "priority": "background_warm" if request_feature in ("spectral_field", "l3_acoustic_summary") else "on_demand",
        "source_id": str(material_path),
    }
    if clip_id:
        cmd["clip_id"] = clip_id
    return cmd


def extract_tracks(state: Dict[str, Any]) -> List[Dict[str, Any]]:
    rows = state.get("tracks")
    if isinstance(rows, list):
        return [row for row in rows if isinstance(row, dict)]
    rows = state.get("project_tracks")
    if isinstance(rows, list):
        return [row for row in rows if isinstance(row, dict)]
    return []


def first_audio_track(state: Dict[str, Any], requested_track_id: str = "") -> Dict[str, Any]:
    fallback: Dict[str, Any] = {}
    for row in extract_tracks(state):
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        is_audio = bool(row.get("is_audio") or row.get("is_audio_track"))
        if requested_track_id and track_id == requested_track_id:
            return row
        if not fallback and is_audio:
            fallback = row
    return fallback


def first_clip_id(track: Dict[str, Any]) -> str:
    clips = track.get("clips")
    if not isinstance(clips, list):
        return ""
    for row in clips:
        if not isinstance(row, dict):
            continue
        clip_id = str(row.get("clip_id") or row.get("id") or "").strip()
        if clip_id:
            return clip_id
    return ""


def resolve_l2_probe_target(req_socket: Any, material_path: Path, requested_track_id: str) -> Dict[str, str]:
    state = send_command(req_socket, {"cmd": "get_project_state"})
    track = first_audio_track(state, requested_track_id)
    track_id = str(track.get("track_id") or track.get("id") or requested_track_id).strip()
    if not track_id:
        reply = send_command(req_socket, {"cmd": "add_audio_track", "name": "DAD Probe"})
        if str(reply.get("status") or "").lower() != "ok":
            raise RuntimeError("add_audio_track for l2_render_probe failed: " + json.dumps(reply, ensure_ascii=False))
        track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
        if not track_id:
            raise RuntimeError("add_audio_track for l2_render_probe did not return track_id: " + json.dumps(reply, ensure_ascii=False))
        track = {"track_id": track_id, "clips": []}
    elif requested_track_id and str(track.get("track_id") or track.get("id") or "").strip() != requested_track_id:
        reply = send_command(req_socket, {"cmd": "add_audio_track", "name": "DAD Probe"})
        if str(reply.get("status") or "").lower() != "ok":
            raise RuntimeError("add_audio_track for requested l2_render_probe target failed: " + json.dumps(reply, ensure_ascii=False))
        track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
        if not track_id:
            raise RuntimeError("add_audio_track for requested l2_render_probe target did not return track_id: " + json.dumps(reply, ensure_ascii=False))
        track = {"track_id": track_id, "clips": []}
    clip_id = first_clip_id(track)
    if not clip_id:
        reply = send_command(req_socket, {
            "cmd": "import_audio",
            "track_id": track_id,
            "file_path": str(material_path),
            "offset_time": 0,
        })
        if str(reply.get("status") or "").lower() != "ok":
            raise RuntimeError("import_audio for l2_render_probe failed: " + json.dumps(reply, ensure_ascii=False))
        clip_id = str(reply.get("clip_id") or "").strip()
    return {"track_id": track_id, "clip_id": clip_id}


def summarize_feature(feature_type: str, events: List[Dict[str, Any]]) -> Dict[str, Any]:
    if feature_type == "l2_render_probe":
        return summarize_l2_render_probe(events)
    if feature_type == "l3_acoustic_summary" or feature_type in L3_ACOUSTIC_FEATURES:
        return summarize_l3_acoustic_summary(feature_type, events)
    rows = []
    ready_count = 0
    suspect_count = 0
    failed_count = 0
    false_ready_count = 0
    for event in events:
        probe = probe_event_shared_memory(event)
        event_ready = event.get("ready")
        if event_ready is True and probe.get("nonzero_count", 0) <= 0:
            false_ready_count += 1
        status = str(probe.get("status") or "")
        if status == "ready":
            ready_count += 1
        elif status == "suspect":
            suspect_count += 1
        else:
            failed_count += 1
        rows.append({
            "event": compact_event(event),
            "shared_memory_probe": probe,
        })
    if not events:
        status = "failed"
        reason = "no_feature_event_received"
    elif false_ready_count > 0:
        status = "failed"
        reason = "event_claimed_ready_but_shared_memory_all_zero"
    elif ready_count > 0:
        status = "ready"
        reason = "at_least_one_nonzero_shared_memory_payload"
    elif suspect_count > 0:
        status = "suspect"
        reason = "events_received_but_payload_suspect"
    else:
        status = "failed"
        reason = "events_received_but_payload_failed"
    return {
        "feature_type": feature_type,
        "status": status,
        "reason": reason,
        "provenance": summarize_provenance(events),
        "event_count": len(events),
        "ready_payload_count": ready_count,
        "suspect_payload_count": suspect_count,
        "failed_payload_count": failed_count,
        "false_ready_count": false_ready_count,
        "events": rows,
    }


def numeric_event_value(event: Dict[str, Any], key: str) -> float:
    value = event.get(key)
    if value in ("", None):
        evidence = event.get("quality_evidence")
        if isinstance(evidence, dict):
            value = evidence.get(key)
    try:
        return float(value or 0)
    except (TypeError, ValueError):
        return 0.0


def event_has_value(event: Dict[str, Any], key: str) -> bool:
    if event.get(key) not in ("", None):
        return True
    identity = event.get("source_identity")
    if isinstance(identity, dict) and identity.get(key) not in ("", None):
        return True
    evidence = event.get("quality_evidence")
    return isinstance(evidence, dict) and evidence.get(key) not in ("", None)


def l3_quality_failure_reasons(event: Dict[str, Any]) -> List[str]:
    reasons: List[str] = []
    evidence = event.get("quality_evidence") if isinstance(event.get("quality_evidence"), dict) else {}
    required = [
        "source_identity",
        "source_revision",
        "clip_revision",
        "analyzer_version",
        "sample_rate",
        "duration_seconds",
        "expected_sample_count",
        "analyzed_sample_count",
        "coverage_ratio",
        "nonzero_count",
        "sum_abs",
        "max_abs",
        "nan_count",
        "inf_count",
        "quality_status",
        "quality_evidence",
        "evidence_ref",
    ]
    missing = [key for key in required if not event_has_value(event, key)]
    if missing:
        reasons.append("missing_required_fields:" + ",".join(missing))
    if not evidence:
        reasons.append("quality_evidence_missing")
        return reasons
    nonzero = numeric_event_value(event, "nonzero_count")
    sum_abs = numeric_event_value(event, "sum_abs")
    max_abs = numeric_event_value(event, "max_abs")
    nan_count = numeric_event_value(event, "nan_count")
    inf_count = numeric_event_value(event, "inf_count")
    coverage = numeric_event_value(event, "coverage_ratio")
    if nonzero <= 0 or sum_abs <= 0 or max_abs <= 0:
        reasons.append("quality_evidence_all_zero")
    if nan_count > 0 or inf_count > 0:
        reasons.append("quality_evidence_nan_or_inf")
    if coverage < 0.95:
        reasons.append("quality_evidence_coverage_below_threshold")
    return reasons


def summarize_l3_acoustic_summary(feature_type: str, events: List[Dict[str, Any]]) -> Dict[str, Any]:
    expected = sorted(L3_ACOUSTIC_FEATURES) if feature_type == "l3_acoustic_summary" else [feature_type]
    rows = []
    ready_count = 0
    suspect_count = 0
    failed_count = 0
    false_ready_count = 0
    observed_features = set()
    per_feature: Dict[str, Dict[str, Any]] = {}
    for event in events:
        observed = str(event.get("feature_type") or "")
        observed_features.add(observed)
        status = str(event.get("status") or event.get("quality_status") or "").lower()
        failures = l3_quality_failure_reasons(event)
        if status == "ready" and failures:
            false_ready_count += 1
        if status == "ready" and not failures:
            ready_count += 1
        elif status == "suspect":
            suspect_count += 1
        else:
            failed_count += 1
        per_feature[observed] = {
            "status": status or "missing",
            "quality_failures": failures,
            "evidence_ref": event.get("evidence_ref"),
        }
        rows.append({
            "event": compact_event(event),
            "quality_failures": failures,
        })
    missing_features = [name for name in expected if name not in observed_features]
    if not events:
        status = "failed"
        reason = "no_l3_acoustic_event_received"
    elif missing_features:
        status = "failed"
        reason = "missing_l3_features:" + ",".join(missing_features)
    elif false_ready_count > 0:
        status = "failed"
        reason = "l3_event_claimed_ready_but_quality_evidence_failed"
    elif ready_count >= len(expected):
        status = "ready"
        reason = "l3_quality_evidence_ready"
    elif suspect_count > 0:
        status = "suspect"
        reason = "l3_quality_evidence_suspect"
    else:
        status = "failed"
        reason = "l3_events_failed"
    return {
        "feature_type": feature_type,
        "status": status,
        "reason": reason,
        "provenance": summarize_provenance(events),
        "event_count": len(events),
        "expected_features": expected,
        "observed_features": sorted(observed_features),
        "per_feature": per_feature,
        "ready_payload_count": ready_count,
        "suspect_payload_count": suspect_count,
        "failed_payload_count": failed_count,
        "false_ready_count": false_ready_count,
        "events": rows,
    }


def summarize_l2_render_probe(events: List[Dict[str, Any]]) -> Dict[str, Any]:
    rows = []
    ready_count = 0
    suspect_count = 0
    failed_count = 0
    raw_leaks: List[str] = []
    required_missing: List[str] = []
    final_events = [
        event for event in events
        if str(event.get("command") or "") == "l2_render_probe_ready"
        or str(event.get("status") or "").lower() in ("ready", "suspect")
    ]
    final_event = final_events[-1] if final_events else (events[-1] if events else {})
    forbidden = [
        "shared_memory",
        "time_segments",
        "spectral_tiles",
        "raw_ranges",
        "raw_waveform",
        "raw_samples",
        "render_file_path",
        "render_path",
        "file_path",
    ]
    for event in events:
        for key in forbidden:
            if key in event:
                raw_leaks.append(key)
        status = str(event.get("status") or "").lower()
        if status == "ready":
            ready_count += 1
        elif status == "suspect":
            suspect_count += 1
        elif status not in ("building", ""):
            failed_count += 1
        rows.append({"event": compact_event(event)})
    for key in ("tap_point", "render_mode", "source_revision", "clip_revision", "render_revision", "quality_evidence", "evidence_ref"):
        if final_event and final_event.get(key) in ("", None):
            required_missing.append(key)
    evidence = final_event.get("quality_evidence") if isinstance(final_event.get("quality_evidence"), dict) else {}
    if events and not final_event:
        status = "failed"
        reason = "no_final_l2_render_probe_event"
    elif raw_leaks:
        status = "failed"
        reason = "raw_payload_leaked:" + ",".join(sorted(set(raw_leaks)))
    elif required_missing:
        status = "failed"
        reason = "missing_required_fields:" + ",".join(required_missing)
    elif evidence.get("nonzero") is not True:
        status = "failed"
        reason = "quality_evidence_nonzero_false"
    elif float(evidence.get("sum_abs") or 0) <= 0 or float(evidence.get("max_abs") or 0) <= 0:
        status = "failed"
        reason = "quality_evidence_energy_missing"
    elif int(float(evidence.get("nan_inf_count") or 0)) != 0:
        status = "failed"
        reason = "quality_evidence_nan_inf"
    elif ready_count > 0:
        status = "ready"
        reason = "l2_render_probe_ready_nonzero"
    elif suspect_count > 0:
        status = "suspect"
        reason = "l2_render_probe_suspect"
    else:
        status = "failed"
        reason = "no_l2_render_probe_ready"
    return {
        "feature_type": "l2_render_probe",
        "status": status,
        "reason": reason,
        "provenance": summarize_provenance(events),
        "event_count": len(events),
        "ready_payload_count": ready_count,
        "suspect_payload_count": suspect_count,
        "failed_payload_count": failed_count,
        "raw_leak_keys": sorted(set(raw_leaks)),
        "events": rows,
    }


def summarize_provenance(events: List[Dict[str, Any]]) -> Dict[str, Any]:
    required = [
        "feature_type",
        "source_identity",
        "source_revision",
        "clip_revision",
        "render_revision",
        "analyzer_version",
        "capture_mode",
        "tap_point",
        "render_mode",
        "sample_rate",
        "duration_seconds",
        "quality_status",
    ]
    present = {key: 0 for key in required}
    values: Dict[str, List[Any]] = {key: [] for key in ["capture_mode", "tap_point", "layer", "time_basis"]}
    for event in events:
        for key in required:
            if event_has_value(event, key):
                present[key] += 1
        for key in values:
            value = event.get(key)
            if value not in ("", None) and value not in values[key]:
                values[key].append(value)
    missing = [key for key, count in present.items() if events and count < len(events)]
    return {
        "schema_version": "dad_probe_provenance.v1",
        "event_count": len(events),
        "present_counts": present,
        "missing_or_partial_fields": missing,
        "values": {key: value for key, value in values.items() if value},
        "quality_evidence_fields": [
            key for key in [
                "nonzero_count",
                "sum_abs",
                "max_abs",
                "nan_inf_count",
                "nan_count",
                "inf_count",
                "expected_sample_count",
                "analyzed_sample_count",
                "coverage_ratio",
                "reader_nonzero_count",
                "fft_input_nonzero_count",
                "shm_postwrite_nonzero_count",
            ]
            if any(event.get(key) not in ("", None) for event in events)
        ],
    }


def compact_event(event: Dict[str, Any]) -> Dict[str, Any]:
    keys = [
        "schema_version",
        "command",
        "feature_family",
        "feature_type",
        "layer",
        "capture_mode",
        "tap_point",
        "render_mode",
        "capture_time",
        "time_basis",
        "feature_version",
        "analysis_version",
        "project_id",
        "track_id",
        "clip_id",
        "target",
        "source_identity",
        "source_id",
        "source_path",
        "source_revision",
        "source_fingerprint",
        "clip_revision",
        "render_revision",
        "evidence_ref",
        "plugin_chain_revision",
        "fader_revision",
        "analyzer_revision",
        "analyzer_version",
        "session_id",
        "tile_index",
        "tile_count",
        "coverage_seconds",
        "coverage_ratio",
        "duration_seconds",
        "sample_rate",
        "channel_count",
        "channels",
        "expected_sample_count",
        "analyzed_sample_count",
        "analyzed_range",
        "frame_count",
        "audio_sample_count",
        "active_frame_count",
        "silent_frame_count",
        "resolution_frame_count",
        "resolution_frame_width",
        "resolution_frequency_bins",
        "feature_stride",
        "shared_memory",
        "float_count",
        "shm_bytes",
        "nonzero_count",
        "sum_abs",
        "max_abs",
        "nan_inf_count",
        "nan_count",
        "inf_count",
        "reader_nonzero_count",
        "reader_sum_abs",
        "reader_max_abs",
        "reader_nan_inf_count",
        "fft_input_nonzero_count",
        "fft_output_nonzero_count",
        "shm_postwrite_nonzero_count",
        "quality_status",
        "quality_reason",
        "quality_reasons",
        "quality_evidence",
        "peak",
        "peak_abs",
        "peak_dbfs",
        "rms",
        "rms_dbfs",
        "integrated_lufs",
        "approximate_lufs",
        "approximate",
        "algorithm",
        "crest_factor",
        "headroom_db",
        "crest_db",
        "bands",
        "noise_floor_evidence",
        "frequency_time_events",
        "transient_events",
        "band_dynamics",
        "left_level_db",
        "right_level_db",
        "balance_db",
        "balance_state",
        "correlation_estimate",
        "correlation_state",
        "ready",
    ]
    return {key: event[key] for key in keys if key in event and event[key] not in ("", None)}


def run_probe(args: argparse.Namespace) -> int:
    import zmq

    material_path = Path(args.material_path).resolve()
    if not material_path.exists():
        raise SystemExit(f"material does not exist: {material_path}")

    context = zmq.Context()
    req = context.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.SNDTIMEO, args.req_timeout_ms)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(args.req_url)

    sub = context.socket(zmq.SUB)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.setsockopt(zmq.RCVTIMEO, 120)
    sub.setsockopt(zmq.LINGER, 0)
    sub.connect(args.sub_url)
    time.sleep(args.subscriber_warmup_ms / 1000.0)

    features = [item.strip() for item in args.features.split(",") if item.strip()]
    commands = []
    replies = []
    collected: Dict[str, List[Dict[str, Any]]] = {feature: [] for feature in features}
    resolved_track_id = args.track_id
    resolved_clip_id = ""
    try:
        if "l2_render_probe" in features or "l3_acoustic_summary" in features or any(feature in L3_ACOUSTIC_FEATURES for feature in features):
            target = resolve_l2_probe_target(req, material_path, args.track_id)
            resolved_track_id = target["track_id"]
            resolved_clip_id = target["clip_id"]
        for feature in features:
            command = build_warm_command(feature, material_path, resolved_track_id, resolved_clip_id)
            commands.append(command)
            replies.append(send_command(req, command))

        deadline = time.time() + args.timeout_sec
        while time.time() < deadline:
            try:
                raw = sub.recv_string()
            except zmq.Again:
                continue
            try:
                event = json.loads(raw)
            except json.JSONDecodeError:
                continue
            if not isinstance(event, dict):
                continue
            for feature in features:
                if event_matches_feature(event, feature, resolved_track_id):
                    collected[feature].append(event)
            if all(feature_minimum_observed(feature, collected[feature]) for feature in features) and not args.wait_all_tiles:
                break
            if args.wait_all_tiles and all(feature_complete(feature, collected[feature]) for feature in features):
                break
    finally:
        req.close(0)
        sub.close(0)
        context.term()

    feature_reports = [summarize_feature(feature, collected[feature]) for feature in features]
    status = "ready"
    if any(row["status"] == "failed" for row in feature_reports):
        status = "failed"
    elif any(row["status"] == "suspect" for row in feature_reports):
        status = "suspect"

    report = {
        "schema_version": "dad_probe.v1",
        "created_at": now_iso(),
        "status": status,
        "material_path": str(material_path),
        "track_id": resolved_track_id,
        "clip_id": resolved_clip_id,
        "req_url": args.req_url,
        "sub_url": args.sub_url,
        "commands": commands,
        "replies": replies,
        "features": feature_reports,
    }
    if args.output:
        output = Path(args.output)
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, ensure_ascii=False))
    if status == "failed":
        return 1
    if status == "suspect" and args.fail_on_suspect:
        return 2
    return 0


def feature_complete(feature_type: str, events: List[Dict[str, Any]]) -> bool:
    if not events:
        return False
    if feature_type == "l2_render_probe":
        return any(str(event.get("status") or "").lower() in ("ready", "suspect") for event in events)
    if feature_type == "l3_acoustic_summary":
        observed = {str(event.get("feature_type") or "") for event in events}
        return L3_ACOUSTIC_FEATURES.issubset(observed)
    if feature_type in L3_ACOUSTIC_FEATURES:
        return any(str(event.get("feature_type") or "") == feature_type for event in events)
    if feature_type == "waveform_envelope":
        return True
    expected = max(int(float(event.get("tile_count") or 0)) for event in events)
    if expected <= 0:
        return True
    indexes = {int(float(event.get("tile_index") or -1)) for event in events}
    return len([index for index in indexes if index >= 0]) >= expected


def feature_minimum_observed(feature_type: str, events: List[Dict[str, Any]]) -> bool:
    if not events:
        return False
    if feature_type == "l3_acoustic_summary" or feature_type in L3_ACOUSTIC_FEATURES or feature_type == "l2_render_probe":
        return feature_complete(feature_type, events)
    return True


def self_test() -> int:
    memory_name = "Vit_DADProbeSelfTest"
    values = [0.0, 0.25, -0.5, float("nan"), 1.0]
    payload = b"".join(struct.pack("<f", value) for value in values)
    if sys.platform == "win32":
        with mmap.mmap(-1, len(payload), tagname=memory_name, access=mmap.ACCESS_WRITE) as mm:
            mm.write(payload)
        observed = read_shared_float32(memory_name, len(values))
    else:
        # Create the POSIX segment the same way the kernel publisher does
        # (shm_open O_CREAT + one ftruncate); a stale leftover from an unclean
        # run is cleared by a best-effort unlink first. The segment is created
        # under the same slash-prefixed name the reader resolves.
        from multiprocessing import shared_memory

        try:
            shared_memory.SharedMemory(name=posix_shm_name(memory_name)).unlink()
        except FileNotFoundError:
            pass
        segment = shared_memory.SharedMemory(create=True, size=len(payload), name=posix_shm_name(memory_name))
        try:
            segment.buf[: len(payload)] = payload
        finally:
            segment.close()
        try:
            observed = read_shared_float32(memory_name, len(values))
        finally:
            shared_memory.SharedMemory(name=posix_shm_name(memory_name)).unlink()
    stats = float_stats(observed)
    ok = stats["sample_count"] == 5 and stats["nonzero_count"] == 3 and stats["nan_inf_count"] == 1
    report = {"schema_version": "dad_probe_self_test.v1", "status": "ready" if ok else "failed", "stats": stats}
    print(json.dumps(report, indent=2))
    return 0 if ok else 1


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Direct DAD probe for Vit-DAW kernel audio feature events and shared memory.")
    parser.add_argument("--req-url", default="tcp://127.0.0.1:5555")
    parser.add_argument("--sub-url", default="tcp://127.0.0.1:5556")
    parser.add_argument("--material-path", default=str(Path("Paper Crown.mp3")))
    parser.add_argument("--track-id", default="dad_probe")
    parser.add_argument("--features", default="waveform_envelope,spectral_field,l3_acoustic_summary,l2_render_probe")
    parser.add_argument("--timeout-sec", type=float, default=30.0)
    parser.add_argument("--req-timeout-ms", type=int, default=120000)
    parser.add_argument("--subscriber-warmup-ms", type=int, default=250)
    parser.add_argument("--output", default="")
    parser.add_argument("--wait-all-tiles", action="store_true")
    parser.add_argument("--fail-on-suspect", action="store_true")
    parser.add_argument("--self-test", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    if args.self_test:
        return self_test()
    return run_probe(args)


if __name__ == "__main__":
    raise SystemExit(main())
