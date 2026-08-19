#!/usr/bin/env python3
"""Blind runner and read-only preflight for Semantic Processor Project Smoke v1.

The runner consumes the public manifest plus non-sensitive runtime receipts. It
never opens sealed truth.  Formal execution is fail-closed: any preflight
blocker produces a closed run report and no Agent request is sent.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import time
import urllib.error
import urllib.request
import uuid
import wave
from pathlib import Path
from typing import Any


STATES = (
    "fixture_ready", "project_importing", "dad_waiting", "agent_started",
    "model_observing", "model_deciding", "candidate_query",
    "model_identifier_selection", "load_confirmation", "preload_pca_recheck",
    "postload_qualification", "control_planning", "parameter_confirmation",
    "typed_execution", "readback_and_snapshot_verification",
    "model_post_action_observation", "model_outcome", "project_terminal",
)
CONTRACT_ID = "semantic_processor_agent_project_smoke_v1_20260809"
RUN_REPORT_SCHEMA = "semantic_processor_agent_project_smoke_run_report.v1"
CHECKPOINT_SCHEMA = "semantic_processor_agent_project_smoke_checkpoint.v1"
PREFLIGHT_SCHEMA = "semantic_processor_agent_project_smoke_preflight.v1"
FORBIDDEN_CONTEXT = {
    "expected_family", "expected_families", "expected_target_track", "fault_label",
    "fault_recipe", "required_coverage", "view_id", "view_ids", "plugin_name",
    "vendor_name", "candidate_identifier", "parameter_id", "source_song_name",
}
DOM_PREFLIGHT_VIEWS = (
    "track.activity_structure",
    "track.frequency_time_events",
    "track.transient_structure",
    "track.band_dynamics",
)
DOM_PREFLIGHT_DIMENSIONS = {
    "track.activity_structure": "activity_structure",
    "track.frequency_time_events": "frequency_time_events",
    "track.transient_structure": "transient_structure",
    "track.band_dynamics": "band_dynamics",
}


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def elapsed_since(started: str, ended: str) -> float:
    begin = dt.datetime.fromisoformat(started.replace("Z", "+00:00"))
    finish = dt.datetime.fromisoformat(ended.replace("Z", "+00:00"))
    return max(0.0, (finish - begin).total_seconds())


def atomic_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError("Agent returned a non-object response")
    return value


def invoke_tool(base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False) -> dict[str, Any]:
    """Invoke product setup/read tools without adding anything to Agent context."""
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "semantic_processor_project_smoke_runner.setup",
    }
    if confirmed:
        payload["confirmed"] = True
    envelope = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    status = str(envelope.get("status", "")).strip().lower()
    if status not in {"ok", "success", "completed"}:
        raise RuntimeError(f"product tool {tool} failed: {json.dumps(envelope, ensure_ascii=False)[:2400]}")
    result = envelope.get("result", envelope)
    if not isinstance(result, dict):
        raise RuntimeError(f"product tool {tool} returned a non-object result")
    return result


def first_text(*values: Any) -> str:
    for value in values:
        text = str(value).strip() if value is not None else ""
        if text and text != "<nil>":
            return text
    return ""


def state_project_path(state: dict[str, Any]) -> str:
    project = state.get("project") if isinstance(state.get("project"), dict) else {}
    observed = state.get("project_observe") if isinstance(state.get("project_observe"), dict) else {}
    observed_project = observed.get("project") if isinstance(observed.get("project"), dict) else {}
    return first_text(
        state.get("project_path"),
        project.get("project_path"),
        observed_project.get("project_path"),
    )


def state_tracks(state: dict[str, Any]) -> list[dict[str, Any]]:
    rows = state.get("tracks")
    if isinstance(rows, list):
        return [row for row in rows if isinstance(row, dict)]
    observed = state.get("project_observe") if isinstance(state.get("project_observe"), dict) else {}
    rows = observed.get("tracks")
    return [row for row in rows if isinstance(row, dict)] if isinstance(rows, list) else []


def track_name(row: dict[str, Any]) -> str:
    return first_text(row.get("track_name"), row.get("name"), row.get("label"))


def track_clip_count(row: dict[str, Any]) -> int:
    for key in ("clips", "clip_summaries"):
        values = row.get(key)
        if isinstance(values, list):
            return len(values)
    value = row.get("clip_count")
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


def analysis_status_payload(value: dict[str, Any]) -> dict[str, Any]:
    job = value.get("analysis_job") if isinstance(value.get("analysis_job"), dict) else {}
    return job if job else value


def prepare_isolated_project(
    agent_http: str,
    case: dict[str, Any],
    limits: dict[str, Any],
    artifact_dir: Path,
) -> dict[str, Any]:
    """Open and verify one isolated fixture before sending the neutral prompt."""
    project_path = str(Path(str(case.get("project_path", ""))).resolve())
    if not project_path or not Path(project_path).is_file():
        raise RuntimeError(f"isolated project missing: {project_path}")
    setup_timeout = float(limits.get("model_turn_timeout_seconds", 420))
    open_result = invoke_tool(
        agent_http,
        "project.open",
        {"file_path": project_path, "project_path": project_path},
        setup_timeout,
        confirmed=True,
    )
    state = invoke_tool(agent_http, "project.state", {}, setup_timeout)
    bound_path = state_project_path(state)
    expected_tracks = [str(value) for value in case.get("track_order", [])]
    tracks = state_tracks(state)
    actual_tracks = [track_name(row) for row in tracks]
    clip_count = sum(track_clip_count(row) for row in tracks)
    if bound_path != project_path:
        raise RuntimeError(f"live project binding mismatch: expected={project_path!r} actual={bound_path!r}")
    if actual_tracks != expected_tracks or len(tracks) != len(expected_tracks) or clip_count != len(expected_tracks):
        raise RuntimeError(
            "live project topology mismatch: "
            + json.dumps({"expected_tracks": expected_tracks, "actual_tracks": actual_tracks, "clip_count": clip_count}, ensure_ascii=False)
        )

    deadline = time.monotonic() + float(limits.get("project_import_and_dad_timeout_seconds", 600))
    # Opening an isolated project does not guarantee that the deferred DAD
    # queue exists in a fresh runtime.  Start that read-only analysis queue
    # when the first status receipt proves it is absent, then poll the same
    # authoritative status endpoint until all six facts are terminal.
    latest_status = invoke_tool(agent_http, "project.audio_analysis_status", {"latest": True}, setup_timeout)
    initial_analysis = analysis_status_payload(latest_status)
    if str(initial_analysis.get("dad_fact_status", "")).strip().lower() not in {"ready", "building", "queued", "pending", "running"} or str(initial_analysis.get("analysis_queue_status", "")).strip().lower() == "missing":
        # A freshly opened isolated .vit may have no in-memory queue even when
        # the project was built with offline DAD evidence.  Ask Kernel to
        # rebuild the deferred queue from the current source clips; a bare
        # start request only looks for an already queued job and fails closed.
        invoke_tool(
            agent_http,
            "project.audio_analysis_start",
            {"retry_missing": True, "rebuild_from_project": True, "interval_ms": 50},
            setup_timeout,
        )
    while time.monotonic() < deadline:
        latest_status = invoke_tool(agent_http, "project.audio_analysis_status", {"latest": True}, setup_timeout)
        analysis = analysis_status_payload(latest_status)
        ready_count = int(analysis.get("dad_fact_ready_count", 0) or 0)
        total_count = int(analysis.get("dad_fact_total_count", 0) or 0)
        dad_status = str(analysis.get("dad_fact_status", "")).strip().lower()
        waveform_rows = analysis.get("track_waveform_envelopes")
        waveform_count = len(waveform_rows) if isinstance(waveform_rows, list) else 0
        if dad_status == "ready" and ready_count == len(expected_tracks) and total_count == len(expected_tracks) and waveform_count >= len(expected_tracks):
            setup = {
                "status": "passed",
                "project_path": project_path,
                "bound_project_path": bound_path,
                "track_names": actual_tracks,
                "track_count": len(tracks),
                "clip_count": clip_count,
                "dad_fact_status": dad_status,
                "dad_fact_ready_count": ready_count,
                "dad_fact_total_count": total_count,
                "track_waveform_envelope_count": waveform_count,
                "open_status": "ok",
            }
            atomic_json(artifact_dir / "project_setup" / f"{case['public_case_id']}.json", setup)
            return setup
        time.sleep(1.0)
    raise RuntimeError(
        "live project DAD did not become ready: "
        + json.dumps(analysis_status_payload(latest_status), ensure_ascii=False)[:3000]
    )


def load_json(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8-sig"))
    if not isinstance(value, dict):
        raise ValueError(f"expected JSON object: {path}")
    return value


def load_public_manifest(path: Path) -> dict[str, Any]:
    manifest = load_json(path)
    if manifest.get("schema_version") != "semantic_processor_agent_project_smoke_public_manifest.v1":
        raise ValueError("public manifest schema mismatch")
    forbidden = {
        "source_material", "source_project", "source_song_name", "issue_assignments", "issue_id",
        "expected_family", "expected_target_track", "target_track", "fault_recipe",
        "detectability_assertion", "clean_reference", "fixed_reference", "sealed_metrics",
    }
    encoded = json.dumps(manifest, ensure_ascii=False).lower()
    leaked = sorted(key for key in forbidden if key.lower() in encoded)
    if leaked:
        raise ValueError(f"public manifest contains forbidden fields: {leaked}")
    return manifest


def runtime_contract(contract_path: Path) -> dict[str, Any]:
    """Extract only runner configuration; sealed assignments are never read by name."""
    contract = load_json(contract_path)
    agent_input = contract.get("agent_input") if isinstance(contract.get("agent_input"), dict) else {}
    limits = contract.get("limits") if isinstance(contract.get("limits"), dict) else {}
    readiness = contract.get("observation_readiness") if isinstance(contract.get("observation_readiness"), dict) else {}
    preflight = contract.get("runtime_preflight") if isinstance(contract.get("runtime_preflight"), dict) else {}
    state_machine = contract.get("state_machine") if isinstance(contract.get("state_machine"), dict) else {}
    return {
        "contract_id": str(contract.get("contract_id", CONTRACT_ID)),
        "initial_prompt": str(agent_input.get("initial_prompt", "")),
        "continuation_prompt": str(agent_input.get("continuation_prompt", "")),
        "confirmation_prompt": str(agent_input.get("confirmation_prompt", "")),
        "context_allowlist": [str(item) for item in agent_input.get("context_allowlist", [])],
        "diagnostic_only": bool(agent_input.get("diagnostic_only", False)),
        "limits": limits,
        "readiness": readiness,
        "preflight": preflight,
        "state_machine": state_machine,
    }


def diagnostic_runtime_contract(contract_path: Path) -> dict[str, Any]:
    """Build the bounded read-only diagnostic override used by CLI replay."""
    runtime = runtime_contract(contract_path)
    runtime["diagnostic_only"] = True
    runtime["initial_prompt"] = "Inspect the whole current DAW project and identify any evidence-grounded issue worth reporting. Request only the observations you need. Do not change anything. End with a free_state_diagnostic.v1 conclusion: confirmed, ruled_out, or unresolved, and cite only returned observation IDs or evidence refs."
    runtime["continuation_prompt"] = "Continue the read-only project diagnosis only if a missing observation could change your conclusion. Otherwise stop with free_state_diagnostic.v1 and cite the returned evidence. Do not select or apply a processor."
    runtime["limits"] = dict(runtime.get("limits", {}))
    # Leave transport headroom above the Agent's 210-second bounded
    # diagnostic loop. Otherwise the runner cancels the request while the LLM
    # client is completing its bounded provider retries and no response or
    # checkpoint evidence can be returned.
    runtime["limits"].update({"maximum_model_turns_per_project": 6, "project_timeout_seconds": 900, "global_timeout_seconds": 1800, "model_turn_timeout_seconds": 240})
    return runtime


def l3_proposal_runtime_contract(contract_path: Path) -> dict[str, Any]:
    """Build the bounded p01 L3 proposal-only runtime contract.

    This path validates natural-language improvement admission and the
    confirmation boundary. It deliberately does not claim processor-family
    execution coverage.
    """
    runtime = runtime_contract(contract_path)
    runtime["l3_proposal_only"] = True
    runtime["initial_prompt"] = (
        "请检查这个完整工程的整体混音状态。自主申请你需要的观察；如果证据与用户的听感目标存在合理关系，"
        "请形成一条有边界、可逆的改善性提案。不要声称当前混音存在确定性错误，不要强行选择唯一效果器，"
        "不要直接修改工程；以 needs_experiment 和 improvement_proposal.v1 返回，等待确认。"
        "第一轮必须严格只返回协议 JSON，并先调用 ccb.observation_catalog；在收到 CCB 观察前不要给出结论，"
        "不要只输出自然语言。"
    )
    runtime["continuation_prompt"] = (
        "继续基于最初目标工作；如果已形成合理的改善性提案，保持目标、证据、假设和预期效果不变，"
        "不要直接执行或把主观改善说成声学真值。"
    )
    runtime["confirmation_prompt"] = "确认当前改善性提案；确认只代表允许进入受控 action domain，不代表声学结果已经被证明。"
    runtime["limits"] = dict(runtime.get("limits", {}))
    runtime["limits"].update({"maximum_model_turns_per_project": 6, "project_timeout_seconds": 900, "global_timeout_seconds": 1800, "model_turn_timeout_seconds": 240})
    return runtime


def l3_full_workflow_runtime_contract(contract_path: Path) -> dict[str, Any]:
    """Run the L3 admission path through the complete governed workflow.

    The public p01 fixture does not currently meet the full multi-family DOM
    preflight, so this mode keeps the neutral L3 entry contract while allowing
    the live Agent to continue past proposal confirmation. It is intentionally
    not an alternate executor: every interaction is answered through the
    product's normal confirmation endpoint.
    """
    runtime = l3_proposal_runtime_contract(contract_path)
    runtime["l3_proposal_only"] = False
    runtime["l3_full_workflow"] = True
    runtime["initial_prompt"] = (
        "请检查这个完整工程的整体混音状态，并从当前已返回的证据中寻找至少一处合理的局部改善机会。自主申请你需要的观察；"
        "只要现有证据与用户的听感目标存在合理关系，即使不能证明主观混音真值，也请形成一条明确标注为改善性假设、"
        "有边界且可逆的提案；不要求覆盖全工程或把局部证据外推成确定性结论。不要声称当前混音存在确定性错误，不要强行选择唯一效果器。"
        "在每个产品确认点等待确认；确认后继续使用受治理的 action domain、资格检查和参数确认。"
        "不要直接修改工程。第一轮必须严格只返回协议 JSON，并先调用 ccb.observation_catalog；"
        "在收到 CCB 观察前不要给出结论，不要只输出自然语言。"
    )
    runtime["continuation_prompt"] = (
        "继续基于最初目标工作。保留改善性假设和观察限制；只在产品确认后进入受治理的下一步。"
        "如果已经完成一次受治理的可逆动作并拿到新的 post-action CCB 观察，先判断原始局部目标是否已经得到足够处理；"
        "如果没有新的、必须处理的证据，不要因为还存在其他可能改善就再次发起动作，直接返回 satisfied/完成结论。"
        "证据或资格不足时停止并给出具体边界。"
    )
    runtime["limits"] = dict(runtime.get("limits", {}))
    # A post-action interaction synchronously covers governed execution,
    # authoritative refresh, a new CCB observation, and the next model turn.
    # Keep the smoke client alive long enough to receive that complete product
    # response; this does not change the model context or action budgets.
    runtime["limits"].update({"maximum_model_turns_per_project": 18, "project_timeout_seconds": 3000, "global_timeout_seconds": 7200, "model_turn_timeout_seconds": 480})
    return runtime


def public_context(runtime: dict[str, Any], manifest: dict[str, Any], case: dict[str, Any]) -> dict[str, Any]:
    context = {
        "agent_mode": "ordinary_agent",
        "interaction_path": "blind_project_smoke",
        "product_lifecycle": "pre_release_experiment",
        "blind_experiment": True,
        "fixture_set_id": manifest["fixture_set_id"],
        "public_case_id": case["public_case_id"],
        "project_id": case["public_case_id"],
    }
    if runtime.get("diagnostic_only"):
        context["free_state_diagnostic_only"] = True
    allowed = set(runtime["context_allowlist"])
    if runtime.get("diagnostic_only"):
        allowed.add("free_state_diagnostic_only")
    context = {key: value for key, value in context.items() if key in allowed}
    if set(context) - allowed or FORBIDDEN_CONTEXT.intersection(context):
        raise AssertionError("Agent context violates the frozen allowlist")
    return context


def verify_stems(manifest: dict[str, Any]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    expected = manifest.get("format") if isinstance(manifest.get("format"), dict) else {}
    manifest_rate = int(expected.get("sample_rate_hz", 0) or 0)
    manifest_channels = int(expected.get("channels", 0) or 0)
    manifest_duration = float(expected.get("duration_seconds", 0.0) or 0.0)
    for case in manifest.get("cases", []):
        if not isinstance(case, dict):
            continue
        files = case.get("stem_files") if isinstance(case.get("stem_files"), list) else []
        expected_rate = int(case.get("sample_rate_hz", manifest_rate) or 0)
        expected_channels = int(case.get("channels", manifest_channels) or 0)
        expected_duration = float(case.get("duration_seconds", manifest_duration) or 0.0)
        expected_frames = int(case.get("frames", round(expected_rate * expected_duration)) or 0)
        expected_track_count = len(case.get("track_order", [])) if isinstance(case.get("track_order"), list) else len(files)
        case_ok = len(files) == expected_track_count and expected_rate > 0 and expected_channels > 0 and expected_frames > 0 and expected_duration > 0
        rows: list[dict[str, Any]] = []
        for row in files:
            path = Path(str(row.get("file"))).resolve()
            exists = path.is_file()
            digest = hashlib.sha256(path.read_bytes()).hexdigest() if exists else ""
            format_ok = False
            actual_format: dict[str, Any] = {}
            if exists:
                try:
                    with wave.open(str(path), "rb") as audio:
                        actual_format = {"sample_rate_hz": audio.getframerate(), "channels": audio.getnchannels(), "frames": audio.getnframes(), "sample_width_bits": audio.getsampwidth() * 8}
                    format_ok = actual_format == {"sample_rate_hz": expected_rate, "channels": expected_channels, "frames": expected_frames, "sample_width_bits": 16}
                except (OSError, EOFError, wave.Error):
                    format_ok = False
            hash_ok = exists and digest == str(row.get("sha256")) and path.stat().st_size == int(row.get("size_bytes", -1))
            ok = hash_ok and format_ok
            rows.append({"track": row.get("track"), "exists": exists, "hash_matches": hash_ok, "format_matches": format_ok, "format": actual_format})
            case_ok = case_ok and ok
        findings.append({"public_case_id": case.get("public_case_id"), "status": "passed" if case_ok else "failed", "stems": rows})
    return findings


def inspect_dom_readiness(repo_root: Path) -> dict[str, Any]:
    path = repo_root / "scripts" / "semantic_processor_project_smoke_dom_readiness.json"
    if not path.is_file():
        return {"status": "blocked", "reason": "DOM readiness artifact missing", "evidence": []}
    artifact = load_json(path)
    cases = artifact.get("cases") if isinstance(artifact.get("cases"), list) else []
    statuses: dict[str, set[str]] = {"frequency_time_events": set(), "transient_structure": set(), "band_dynamics": set(), "activity_noise_floor": set()}
    for row in cases:
        projection = row.get("projection") if isinstance(row, dict) else {}
        for dimension in ("frequency_time_events", "transient_structure", "band_dynamics"):
            value = projection.get(dimension) if isinstance(projection, dict) else {}
            statuses[dimension].add(str(value.get("status", "missing")))
        activity = projection.get("activity_structure") if isinstance(projection, dict) else {}
        statuses["activity_noise_floor"].add(str(activity.get("noise_floor_status", "missing")))
    evidence = []
    if "ready" not in statuses["frequency_time_events"] or "partial" in statuses["frequency_time_events"]:
        evidence.append({"check": "track.frequency_time_events", "status": "blocked", "observed": sorted(statuses["frequency_time_events"]), "reason": "time-localized frequency events are not ready"})
    if "ready" not in statuses["transient_structure"] or "partial" in statuses["transient_structure"]:
        evidence.append({"check": "track.transient_structure", "status": "blocked", "observed": sorted(statuses["transient_structure"]), "reason": "typed onset/attack/body/sustain evidence is not ready"})
    if "ready" not in statuses["band_dynamics"] or "partial" in statuses["band_dynamics"]:
        evidence.append({"check": "track.band_dynamics", "status": "blocked", "observed": sorted(statuses["band_dynamics"]), "reason": "per-band time distribution and cross-band motion are not ready"})
    if "ready" not in statuses["activity_noise_floor"] or "missing" in statuses["activity_noise_floor"]:
        evidence.append({"check": "track.activity_structure.noise_floor", "status": "blocked", "observed": sorted(statuses["activity_noise_floor"]), "reason": "usable noise-floor fact is missing"})
    return {"status": "blocked" if evidence else "passed", "artifact": str(path), "evidence": evidence}


def inspect_godot_lifecycle(path: Path) -> dict[str, Any]:
    if not path.is_file():
        return {"status": "blocked", "artifact": str(path), "evidence": [{"reason": "lifecycle receipt missing"}]}
    receipt = load_json(path)
    evidence: list[dict[str, Any]] = []
    if receipt.get("schema_version") != "semantic_processor_agent_project_smoke_godot_lifecycle_receipt.v1":
        evidence.append({"reason": "lifecycle receipt schema mismatch", "observed": receipt.get("schema_version")})
    if receipt.get("status") != "passed" or receipt.get("product_lifecycle") != "godot_project":
        evidence.append({"reason": "lifecycle receipt is not a passed Godot project lifecycle", "status": receipt.get("status"), "product_lifecycle": receipt.get("product_lifecycle")})
    children = receipt.get("godot_autostart") if isinstance(receipt.get("godot_autostart"), list) else []
    roles = {str(row.get("role", "")) for row in children if isinstance(row, dict)}
    for role in ("kernel", "hub", "agent"):
        if role not in roles:
            evidence.append({"reason": "Godot autostart child missing", "role": role})
    ports = receipt.get("ports") if isinstance(receipt.get("ports"), list) else []
    port_values = {int(row.get("local_port", row.get("port", 0)) or 0) for row in ports if isinstance(row, dict)}
    for port in (5555, 5556, 7878, 8787):
        if port not in port_values:
            evidence.append({"reason": "required lifecycle port missing", "port": port})
    return {"status": "blocked" if evidence else "passed", "artifact": str(path), "evidence": evidence}


def run_product_dom_observation_preflight(agent_http: str, project_report: Path | None, timeout: float = 180.0, limits: dict[str, Any] | None = None) -> dict[str, Any]:
    if not agent_http or project_report is None or not project_report.is_file():
        return {"status": "skipped", "reason": "project_report_or_agent_http_missing", "observations": [], "blockers": []}
    report = load_json(project_report)
    setup_limits = dict(limits or {})
    setup_limits.setdefault("model_turn_timeout_seconds", 180)
    setup_limits.setdefault("project_import_and_dad_timeout_seconds", 420)
    rows: list[dict[str, Any]] = []
    blockers: list[dict[str, Any]] = []
    for project in report.get("projects", []):
        if not isinstance(project, dict):
            continue
        project_path = first_text(project.get("project_path"))
        tracks = project.get("tracks", [])
        track_order = [first_text(row.get("track_name"), row.get("name")) for row in tracks if isinstance(row, dict)]
        case = {
            "public_case_id": first_text(project.get("case_id"), project.get("public_case_id"), "case"),
            "project_path": project_path,
            "track_order": track_order,
        }
        try:
            setup = prepare_isolated_project(agent_http, case, setup_limits, project_report.parent / "dom_preflight_setup")
        except Exception as exc:  # noqa: BLE001
            blockers.append({"case_id": case["public_case_id"], "check": "project.open_and_dad_ready", "status": "blocked", "reason": str(exc)[:400]})
            continue
        if setup.get("status") != "passed":
            blockers.append({"case_id": case["public_case_id"], "check": "project.open_and_dad_ready", "status": "blocked", "reason": "isolated project setup did not pass"})
            continue
        for track in tracks:
            if not isinstance(track, dict):
                continue
            track_id = first_text(track.get("track_id"), track.get("id"))
            if not track_id:
                continue
            label = first_text(track.get("track_name"), track.get("name"), track_id)
            try:
                result = invoke_tool(agent_http, "ccb.observation_request", {
                    "view_ids": list(DOM_PREFLIGHT_VIEWS),
                    "target_ref": {"kind": "track", "id": track_id, "label": label},
                }, timeout)
            except Exception as exc:  # noqa: BLE001
                blockers.append({"track_id": track_id, "check": "ccb.observation_request", "status": "blocked", "reason": str(exc)[:400]})
                continue
            bundle = result.get("bundle") if isinstance(result, dict) else {}
            if not isinstance(bundle, dict):
                bundle = {}
            views = bundle.get("views") if isinstance(bundle.get("views"), dict) else {}
            view_statuses: dict[str, Any] = {}
            for view_id, dimension_key in DOM_PREFLIGHT_DIMENSIONS.items():
                view = views.get(view_id) if isinstance(views, dict) else {}
                if not isinstance(view, dict):
                    view = {}
                facts = view.get("facts") if isinstance(view.get("facts"), dict) else {}
                fact = facts.get("observation.dom_projection") if isinstance(facts.get("observation.dom_projection"), dict) else {}
                dimension = fact.get(dimension_key) if isinstance(fact.get(dimension_key), dict) else {}
                status = first_text(dimension.get("status"), fact.get("status"), view.get("status"))
                conditions = fact.get("conditions") if isinstance(fact.get("conditions"), dict) else {}
                measurement_key = first_text(conditions.get("measurement_key"))
                evidence_refs = fact.get("evidence_refs")
                has_refs = isinstance(evidence_refs, list) and len(evidence_refs) > 0
                ready = status == "ready" and bool(measurement_key) and has_refs
                view_statuses[view_id] = {
                    "status": status,
                    "measurement_key": measurement_key,
                    "evidence_refs_present": has_refs,
                    "ready": ready,
                }
                if not ready:
                    blockers.append({
                        "track_id": track_id,
                        "view_id": view_id,
                        "status": "blocked",
                        "observed_status": status,
                        "measurement_key_present": bool(measurement_key),
                        "evidence_refs_present": has_refs,
                    })
            rows.append({"track_id": track_id, "label": label, "bundle_status": first_text(bundle.get("status")), "views": view_statuses})
    return {"status": "passed" if not blockers else "blocked", "observations": rows, "blockers": blockers}


def preflight(repo_root: Path, contract_path: Path, manifest_path: Path, *, project_report: Path | None = None, runtime_receipts: Path | None = None, godot_lifecycle_receipt: Path | None = None, runtime: dict[str, Any] | None = None, manifest: dict[str, Any] | None = None, agent_http: str = "") -> dict[str, Any]:
    runtime = runtime or runtime_contract(contract_path)
    manifest = manifest or load_public_manifest(manifest_path)
    blockers: list[dict[str, Any]] = []
    prompt_text = "\n".join(runtime[key] for key in ("initial_prompt", "continuation_prompt", "confirmation_prompt"))
    prompt_forbidden = set(FORBIDDEN_CONTEXT) | {
        "static_eq", "broadband_compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics", "spectral_dynamics", "clipper",
    }
    leaked_prompt_tokens = sorted(token for token in prompt_forbidden if token.lower() in prompt_text.lower())
    if leaked_prompt_tokens:
        blockers.append({"check": "all Agent prompts remain neutral and answer-free", "status": "blocked", "leaked_tokens": leaked_prompt_tokens})
    stems = verify_stems(manifest)
    if any(row.get("status") != "passed" for row in stems):
        blockers.append({"check": "fixture audio passes format, duration, headroom, RMS-match, issue detectability, and non-target identity gates", "status": "blocked", "evidence": stems})
    receipt = manifest_path.parent / "fixture_preflight_receipt.json"
    if not receipt.is_file():
        blockers.append({"check": "fixture audio passes format, duration, headroom, RMS-match, issue detectability, and non-target identity gates", "status": "blocked", "reason": "fixture preflight receipt missing"})
    else:
        fixture_receipt = load_json(receipt)
        if fixture_receipt.get("status") != "passed" or fixture_receipt.get("sealed_truth_not_included") is not True:
            blockers.append({"check": "fixture audio passes format, duration, headroom, RMS-match, issue detectability, and non-target identity gates", "status": "blocked", "reason": "fixture receipt failed or is not sealed-safe"})
    if project_report is None:
        project_report = manifest_path.parent / "project_build_report.json"
    if not project_report.is_file():
        blockers.append({"check": "each rebuilt project contains exactly six expected tracks and six clips", "status": "blocked", "reason": "isolated product import report missing; .vit projects are not built"})
        blockers.append({"check": "all six DAD analyses are terminal and usable", "status": "blocked", "reason": "isolated product import/DAD report missing"})
    else:
        report = load_json(project_report)
        manifest_cases = [case for case in manifest.get("cases", []) if isinstance(case, dict)]
        expected_case_ids = [str(case.get("public_case_id", "")) for case in manifest_cases]
        expected_by_case = {str(case.get("public_case_id", "")): case for case in manifest_cases}
        expected_project_count = len(manifest_cases)
        report_projects = [project for project in report.get("projects", []) if isinstance(project, dict)]
        # A bounded --case replay may use a full shared build report. Restrict
        # structural preflight to the selected public case instead of treating
        # untouched sibling cases as mismatches.
        if len(expected_case_ids) < len([project for project in report.get("projects", []) if isinstance(project, dict)]):
            report_projects = [project for project in report_projects if str(project.get("case_id", project.get("public_case_id", ""))) in expected_by_case]
        reported_case_ids = [str(project.get("case_id", project.get("public_case_id", ""))) for project in report_projects]
        reported_project_count = len(report_projects)
        structure_check = (
            report.get("status") == "passed"
            and reported_project_count == expected_project_count
            and reported_case_ids == expected_case_ids
        )
        if not structure_check:
            blockers.append({
                "check": "each rebuilt project contains the manifest-declared tracks and clips",
                "status": "blocked",
                "reason": "project build report does not match the public manifest project set",
                "expected_project_count": expected_project_count,
                "reported_project_count": reported_project_count,
                "expected_case_ids": expected_case_ids,
                "reported_case_ids": reported_case_ids,
                "report": str(project_report),
            })
        for project in report_projects:
            case_id = str(project.get("case_id", project.get("public_case_id", "")))
            expected_case = expected_by_case.get(case_id, {})
            expected_tracks = [str(track) for track in expected_case.get("track_order", [])] if isinstance(expected_case.get("track_order"), list) else []
            tracks = project.get("tracks") if isinstance(project.get("tracks"), list) else []
            reported_tracks = [str(row.get("track_name", "")) if isinstance(row, dict) else "" for row in tracks]
            valid_rows = all(isinstance(row, dict) and str(row.get("track_id", "")).strip() and str(row.get("clip_id", "")).strip() for row in tracks)
            path_matches = not expected_case.get("project_path") or str(project.get("project_path", "")) == str(expected_case.get("project_path"))
            if case_id not in expected_by_case or len(tracks) != len(expected_tracks) or reported_tracks != expected_tracks or not valid_rows or not path_matches:
                blockers.append({
                    "check": "each rebuilt project contains the manifest-declared tracks and clips",
                    "status": "blocked",
                    "reason": "project report does not prove the manifest track order/identity",
                    "case_id": case_id,
                    "expected_tracks": expected_tracks,
                    "reported_tracks": reported_tracks,
                    "project_path_matches": path_matches,
                })
            analysis = project.get("analysis") if isinstance(project.get("analysis"), dict) else {}
            expected_track_count = len(expected_tracks)
            dad_ready = int(analysis.get("dad_fact_ready_count", 0) or 0)
            dad_total = int(analysis.get("dad_fact_total_count", 0) or 0)
            if str(analysis.get("dad_fact_status", "")).lower() != "ready" or dad_ready != expected_track_count or dad_total != expected_track_count:
                blockers.append({
                    "check": "all manifest-declared track DAD analyses are terminal and usable",
                    "status": "blocked",
                    "reason": "DAD report is not terminal and complete for the manifest track set",
                    "case_id": case_id,
                    "expected_track_count": expected_track_count,
                    "dad_fact_ready_count": dad_ready,
                    "dad_fact_total_count": dad_total,
                })
            fine = analysis.get("product_fine_evidence") if isinstance(analysis.get("product_fine_evidence"), dict) else {}
            fine_tracks = fine.get("tracks") if isinstance(fine.get("tracks"), list) else []
            if fine.get("status") != "passed" or len(fine_tracks) != expected_track_count or any(
                not isinstance(row, dict) or row.get("status") != "passed" for row in fine_tracks
            ):
                blockers.append({
                    "check": "product-path DAD exposes all bounded DOM producer facts before Agent start",
                    "status": "blocked",
                    "reason": "project build report lacks complete per-track product-path fine evidence",
                    "case_id": case_id,
                    "required_fields": fine.get("required_fields", ["noise_floor_evidence", "frequency_time_events", "transient_events", "band_dynamics"]),
                    "fine_evidence": fine,
                })
    l3_proposal_only = bool(runtime.get("l3_proposal_only") or runtime.get("l3_full_workflow"))
    dom = inspect_dom_readiness(repo_root)
    lifecycle = {"status": "not_supplied", "artifact": str(godot_lifecycle_receipt or ""), "evidence": []}
    if godot_lifecycle_receipt is not None:
        lifecycle = inspect_godot_lifecycle(godot_lifecycle_receipt)
        if lifecycle.get("status") != "passed":
            blockers.append({"check": "Godot-owned product lifecycle is current and complete", "status": "blocked", "evidence": lifecycle.get("evidence", [])})
    if runtime_receipts is None:
        runtime_receipts = manifest_path.parent / "runtime_preflight_receipts.json"
    receipt_data = load_json(runtime_receipts) if runtime_receipts.is_file() else {}
    runtime_checks = (
        ("ccb_catalog_neutral", "CCB catalog contains every contract view without family routing labels"),
        ("view_set_exact_match", "model-requested and actual executed view sets are audited for exact equality"),
        ("pca_family_matrix", "at least one PCA-eligible candidate exists for every executable family under test"),
        ("typed_controller_roundtrip", "a no-op parameter round trip and rollback probe passes for every typed controller without modifying the smoke fixture"),
        ("l2_before_after", "L2 before/after probe produces a ready same-tap same-mode changed-revision AB result"),
    )
    for key, check in runtime_checks:
        if receipt_data.get(key) is not True:
            blockers.append({"check": check, "status": "blocked", "reason": "runtime receipt not supplied; formal preflight fails closed"})
    if l3_proposal_only:
        dom_observation = {"status": "skipped", "reason": "l3_proposal_only_does_not_require_full_dom_family_preflight", "observations": [], "blockers": []}
    else:
        dom_observation = run_product_dom_observation_preflight(agent_http, project_report, limits=runtime.get("limits"))
        if dom_observation.get("status") != "passed":
            blockers.extend(dom.get("evidence", []))
            if dom_observation.get("status") == "blocked":
                blockers.append({"check": "product-path CCB DOM views are ready", "status": "blocked", "evidence": dom_observation.get("blockers", [])})
    return {
        "schema_version": PREFLIGHT_SCHEMA,
        "contract_id": runtime["contract_id"],
        "fixture_set_id": manifest.get("fixture_set_id"),
        "status": "preflight_blocked" if blockers else "passed",
        "official_run_budget_consumed": False,
        "formal_run_startable": not blockers,
        "checked_at": now_iso(),
        "blockers": blockers,
        "evidence": {"stem_checks": stems, "dom_readiness": dom, "product_dom_observation": dom_observation, "project_report": str(project_report), "runtime_receipts": str(runtime_receipts), "godot_lifecycle": lifecycle},
    }


class CheckpointStore:
    def __init__(self, path: Path, run_id: str, case_id: str, conversation_id: str, original_intent_hash: str) -> None:
        self.path = path
        self.value: dict[str, Any] = {
            "schema_version": CHECKPOINT_SCHEMA,
            "run_id": run_id,
            "public_case_id": case_id,
            "conversation_id": conversation_id,
            "original_intent_hash": original_intent_hash,
            "state": "fixture_ready",
            "state_index": 0,
            "last_response_artifact": "",
            "pending_interaction_id": "",
            "applied_action_receipt_ids": [],
            "project_revision": "initial",
            "updated_at": now_iso(),
            "resume_count": 0,
            "history": [],
        }

    def transition(self, state: str, **evidence: Any) -> None:
        if state not in STATES:
            raise ValueError(f"unknown state {state}")
        self.value["state"] = state
        self.value["state_index"] = STATES.index(state)
        self.value["updated_at"] = now_iso()
        self.value["history"].append({"state": state, "at": self.value["updated_at"], "evidence": evidence})
        atomic_json(self.path, self.value)


def make_blocked_report(runtime: dict[str, Any], manifest: dict[str, Any], run_id: str, started: str, ended: str, preflight_result: dict[str, Any], checkpoint_dir: Path) -> dict[str, Any]:
    projects = []
    for case in manifest.get("cases", []):
        case_id = str(case["public_case_id"])
        conversation_id = f"smoke_{run_id}_{case_id}"
        intent_hash = sha256_bytes((runtime["initial_prompt"] + "\n" + case_id).encode("utf-8"))
        checkpoint = CheckpointStore(checkpoint_dir / f"{case_id}.json", run_id, case_id, conversation_id, intent_hash)
        checkpoint.transition("fixture_ready", preflight="blocked")
        checkpoint.transition("project_terminal", reason="preflight_blocked")
        projects.append({
            "public_case_id": case_id,
            "conversation_id": conversation_id,
            "original_intent_hash": intent_hash,
            "project_lifecycle": "not_started_preflight_blocked",
            "model_turns": 0,
            "model_outcomes": ["inconclusive"],
            "agent_conformance": "unobservable",
            "execution_evidence": {"events": [], "missing": ["all runtime Agent evidence; preflight blocked before Agent start"]},
            "timeouts": [],
            "checkpoint_history": str(checkpoint.path),
            "terminal_state": "project_terminal",
        })
    return {
        "schema_version": RUN_REPORT_SCHEMA,
        "contract_id": runtime["contract_id"],
        "fixture_set_id": manifest.get("fixture_set_id"),
        "run_id": run_id,
        "status": "preflight_blocked",
        "started_at": started,
        "ended_at": ended,
        "elapsed_seconds": elapsed_since(started, ended),
        "sealed_truth_opened_by_runner": False,
        "projects": projects,
        "family_coverage": {"status": "not_exercised_by_model", "families": []},
        "partial_report": {"is_partial": True, "closed_at": ended, "last_completed_state": "fixture_ready", "failure_or_timeout": "preflight_blocked", "evidence_complete_through_state": "fixture_ready", "missing_evidence": [row["check"] for row in preflight_result["blockers"]], "resume_eligible": False},
        "resume_continuation": {"resumed": False, "resume_count": 0, "checkpoint_id": "", "same_conversation": True, "original_intent_hash_matches": True, "mutation_replay_guard": "no mutation attempted", "continuation_results": []},
        "preflight": preflight_result,
    }


def response_status(response: dict[str, Any]) -> str:
    workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    closure = workflow.get("minimal_audio_closure") if isinstance(workflow.get("minimal_audio_closure"), dict) else {}
    if not closure:
        request_context = workflow.get("request_context") if isinstance(workflow.get("request_context"), dict) else {}
        closure = request_context.get("minimal_audio_closure") if isinstance(request_context.get("minimal_audio_closure"), dict) else {}
    settlement = closure.get("settlement") if isinstance(closure.get("settlement"), dict) else {}
    closure_reason = str(settlement.get("reason", "")).strip().lower()
    if closure_reason in {"satisfied", "diagnostic_complete"}:
        return "satisfied"
    if closure_reason in {
        "insufficient_evidence", "evidence_ceiling_reached", "no_progress", "capability_unavailable",
        "pca_unavailable", "round_limit", "action_failed", "verification_failed_rolled_back",
        "project_revision_stale", "cancelled",
    }:
        return "model_blocked"
    if closure_reason in {"model_protocol_failure", "transport_failure"}:
        return "error"
    if closure_reason == "user_choice_required":
        return "waiting_clarification"
    loop = workflow.get("free_state_reasoning_loop") if isinstance(workflow.get("free_state_reasoning_loop"), dict) else {}
    decision = loop.get("latest_decision") if isinstance(loop.get("latest_decision"), dict) else {}
    free_state = response.get("free_state") if isinstance(response.get("free_state"), dict) else {}
    terminal_free_state = str((free_state or decision).get("status", "")).strip().lower()
    if terminal_free_state == "blocked":
        return "blocked"
    if terminal_free_state == "satisfied":
        # A structural snapshot (most notably project.state) is not the
        # model-requested acoustic evidence required by the smoke contract.
        # Do not let a free-state serializer turn that early exit into a
        # successful model outcome.
        counts = evidence_counts([response])
        if counts["model_observation_receipts"] <= 0:
            return "model_blocked"
        return "satisfied"
    stop_reason = str(response.get("stop_reason", "")).strip().lower()
    if stop_reason in {"limit_reached", "max_turns", "budget_exhausted"}:
        return "inconclusive"
    if stop_reason in {"failed", "error", "model_protocol_failure", "transport_failure"}:
        return "error"
    if stop_reason in {"no_progress", "round_limit", "evidence_ceiling_reached", "insufficient_evidence", "project_revision_stale", "capability_unavailable", "pca_unavailable"}:
        return "model_blocked"
    for key in ("goal_status", "execution_status", "status"):
        value = str(response.get(key, "")).strip().lower()
        if value:
            if value in {"completed", "done"}:
                counts = evidence_counts([response])
                has_mutation = counts["transaction_receipts"] > 0 or counts["typed_controller_receipts"] > 0
                if not has_mutation:
                    if counts["model_observation_receipts"] <= 0:
                        # project.state/get_project_state and legacy mix
                        # readers can prove topology, but they cannot prove
                        # an adequate model-owned acoustic observation for a
                        # blind full-project smoke run.
                        return "model_blocked"
                    if observation_result_is_limited(response):
                        return "model_blocked"
                    return "model_no_op"
            return value
    return ""


def orchestration_metrics(responses: list[dict[str, Any]]) -> dict[str, Any]:
    controllers: list[str] = []
    closure_ids: list[str] = []
    max_rounds_started = 0
    max_unique_observations = 0
    settlements: list[str] = []
    for response in responses:
        workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
        sources = [workflow]
        request_context = workflow.get("request_context") if isinstance(workflow.get("request_context"), dict) else {}
        if request_context:
            sources.append(request_context)
        for source in sources:
            decision = source.get("orchestration_controller_decision") if isinstance(source.get("orchestration_controller_decision"), dict) else {}
            controller = str(decision.get("controller", "")).strip()
            if controller and controller not in controllers:
                controllers.append(controller)
            closure = source.get("minimal_audio_closure") if isinstance(source.get("minimal_audio_closure"), dict) else {}
            closure_id = str(closure.get("closure_id", "")).strip()
            if closure_id and closure_id not in closure_ids:
                closure_ids.append(closure_id)
            try:
                max_rounds_started = max(max_rounds_started, int(closure.get("rounds_started", 0) or 0))
            except (TypeError, ValueError):
                pass
            observations = closure.get("observations")
            if isinstance(observations, dict):
                max_unique_observations = max(max_unique_observations, len(observations))
            settlement = closure.get("settlement") if isinstance(closure.get("settlement"), dict) else {}
            reason = str(settlement.get("reason", "")).strip()
            if reason and reason not in settlements:
                settlements.append(reason)
    return {
        "controllers": controllers,
        "closure_ids": closure_ids,
        "max_closure_rounds_started": max_rounds_started,
        "max_unique_observation_sets": max_unique_observations,
        "settlements": settlements,
        "bounded_v1": max_rounds_started <= 6 and max_unique_observations <= 3,
    }


def minimal_closure_metrics(responses: list[dict[str, Any]]) -> dict[str, Any]:
    """Report whether a treatment closure moved beyond broad observation.

    This is intentionally weaker than full seven-family execution evidence,
    but stronger than a bounded stop: once a relation observation exposes a
    candidate, the run must select a candidate target and reach either a
    model-owned needs_action decision or an explicit action-preflight boundary.
    """
    candidate_ids: set[str] = set()
    selected_ids: set[str] = set()
    needs_action = False
    needs_experiment = False
    explicit_boundary = False
    handoff = False
    terminal_reasons: set[str] = set()
    for response in responses:
        workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
        sources = [workflow]
        request_context = workflow.get("request_context") if isinstance(workflow.get("request_context"), dict) else {}
        if request_context:
            sources.append(request_context)
        for source in sources:
            closure = source.get("minimal_audio_closure") if isinstance(source.get("minimal_audio_closure"), dict) else {}
            frontier = closure.get("hypothesis_frontier") if isinstance(closure.get("hypothesis_frontier"), dict) else {}
            candidates = frontier.get("candidates") if isinstance(frontier.get("candidates"), list) else []
            for candidate in candidates:
                if isinstance(candidate, dict):
                    candidate_id = str(candidate.get("id") or "").strip()
                    if candidate_id:
                        candidate_ids.add(candidate_id)
            selected = str(frontier.get("candidate_id") or "").strip()
            if selected and selected in candidate_ids:
                selected_ids.add(selected)
            if isinstance(closure.get("active_capability_session"), dict):
                handoff = True
            settlement = closure.get("settlement") if isinstance(closure.get("settlement"), dict) else {}
            reason = str(settlement.get("reason") or "").strip().lower()
            if reason:
                terminal_reasons.add(reason)
        for decision in model_decision_roots(response):
            status = str(decision.get("status") or "").strip().lower()
            if status == "needs_action":
                needs_action = True
            if status == "needs_experiment":
                needs_experiment = True
            if status == "blocked" and any(
                str(decision.get(key) or "").strip()
                for key in ("stop_reason", "summary")
            ):
                # stop_reason is optional in free_state_decision.v1. A
                # concrete model summary is sufficient to make this an
                # auditable action-preflight boundary.
                explicit_boundary = True
    counts = evidence_counts(responses)
    handoff = handoff or counts["pca_preload_receipts"] > 0 or counts["pca_postload_receipts"] > 0 or counts["typed_controller_receipts"] > 0
    candidate_discovered = bool(candidate_ids)
    candidate_selected = bool(selected_ids)
    direction_reached = not candidate_discovered or (candidate_selected and (needs_action or needs_experiment or explicit_boundary))
    failure_reason = ""
    if candidate_discovered and not candidate_selected:
        failure_reason = "candidate_discovered_but_not_selected"
    elif candidate_discovered and not (needs_action or needs_experiment or explicit_boundary):
        failure_reason = "target_observed_but_no_action_or_preflight_boundary"
    if terminal_reasons.intersection({"no_progress", "round_limit", "evidence_ceiling_reached"}) and not (needs_action or needs_experiment or explicit_boundary):
        direction_reached = False
        failure_reason = "terminated_without_execution_direction"
    return {
        "candidate_discovered": candidate_discovered,
        "candidate_selected": candidate_selected,
        "target_observation": candidate_selected,
        "needs_action": needs_action,
        "needs_experiment": needs_experiment,
        "action_preflight_boundary": explicit_boundary,
        "capability_handoff": handoff,
        "terminal_reasons": sorted(terminal_reasons),
        "execution_direction_reached": direction_reached,
        "failure_reason": failure_reason,
    }


def improvement_proposal_metrics(responses: list[dict[str, Any]]) -> dict[str, Any]:
    """Validate the L3 semantic handoff without opening sealed truth."""
    valid_proposals = 0
    candidate_count = 0
    confirmation_count = 0
    proposals: list[dict[str, Any]] = []
    allowed_domains = {
        "track_gain", "clip_gain", "pan", "eq", "compressor", "limiter",
        "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics", "plugin",
    }
    for response in responses:
        for decision in model_decision_roots(response):
            if str(decision.get("status") or "").strip().lower() != "needs_experiment":
                continue
            proposal = decision.get("improvement_proposal") if isinstance(decision.get("improvement_proposal"), dict) else {}
            required = ("target", "evidence_refs", "improvement_intent", "hypothesis", "expected_effect", "action_domain", "action_kind")
            valid = (
                str(proposal.get("schema_version") or "").strip() == "improvement_proposal.v1"
                and isinstance(proposal.get("target"), dict) and bool(proposal.get("target"))
                and isinstance(proposal.get("evidence_refs"), list) and bool(proposal.get("evidence_refs"))
                and all(str(proposal.get(key) or "").strip() for key in required[2:])
                and str(proposal.get("action_domain") or "").strip().lower() in allowed_domains
            )
            if valid:
                valid_proposals += 1
                proposals.append(proposal)
        workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
        typed_state = workflow.get("typed_state") if isinstance(workflow.get("typed_state"), dict) else {}
        if str(typed_state.get("candidate_type") or "").strip().lower() == "improvement_proposal":
            candidate_count += 1
        if bool(response.get("needs_confirmation")):
            confirmation_count += 1
        interactions = response.get("interaction_requests")
        if isinstance(interactions, list) and any(
            isinstance(row, dict) and str(row.get("workflow") or "").strip().lower() == "improvement_proposal"
            for row in interactions
        ):
            confirmation_count += 1
    counts = evidence_counts(responses)
    mutation_count = counts["transaction_receipts"] + counts["typed_controller_receipts"]
    return {
        "proposal_valid": valid_proposals > 0,
        "valid_proposal_count": valid_proposals,
        "pending_candidate": candidate_count > 0,
        "pending_candidate_count": candidate_count,
        "confirmation_boundary": confirmation_count > 0,
        "mutation_count": mutation_count,
        "mutation_performed": mutation_count > 0,
        "admitted": valid_proposals > 0 and candidate_count > 0 and confirmation_count > 0 and mutation_count == 0,
        "proposals": proposals,
    }


def recommended_interaction_submission(response: dict[str, Any]) -> tuple[dict[str, Any] | None, str]:
    """Extract one explicit product interaction approval without inventing input.

    The live chat contract exposes confirmation as ``interaction_requests``;
    the runner must answer that contract through ``/agent/interaction/respond``
    rather than sending a synthetic continuation message.  A non-cancel action
    is eligible only when the product marks it recommended (or primary).
    """
    interactions = response.get("interaction_requests")
    if not isinstance(interactions, list):
        interactions = []
    waiting = bool(response.get("needs_confirmation")) or str(response.get("goal_status", "")).strip().lower() in {
        "waiting_confirmation", "needs_confirmation", "waiting_for_user",
    }
    for request in interactions:
        if not isinstance(request, dict):
            continue
        interaction_id = first_text(request.get("id"), request.get("interaction_id"))
        if not interaction_id:
            continue
        actions = request.get("actions")
        if not isinstance(actions, list):
            actions = []
        eligible: list[dict[str, Any]] = []
        for action in actions:
            if not isinstance(action, dict):
                continue
            action_id = first_text(action.get("id"), action.get("action_id")).lower()
            if not action_id or action_id in {"cancel", "reject", "decline", "abort"}:
                continue
            eligible.append(action)
        if not eligible:
            continue
        selected = next((item for item in eligible if bool(item.get("recommended"))), None)
        if selected is None:
            selected = next((item for item in eligible if str(item.get("style", "")).strip().lower() in {"primary", "confirm"}), None)
        if selected is None:
            continue
        action_id = first_text(selected.get("id"), selected.get("action_id"))
        payload = request.get("payload") if isinstance(request.get("payload"), dict) else {}
        return {
            "interaction_id": interaction_id,
            "decision": action_id,
            "action_id": action_id,
            "payload": payload,
        }, ""
    if waiting or interactions:
        return None, "confirmation interaction has no explicit recommended non-cancel action"
    return None, ""


def observation_result_is_limited(response: dict[str, Any]) -> bool:
    """Return true only for structured CCB results that report partial/blocked evidence."""
    replies = response.get("executed_kernel_reply")
    if not isinstance(replies, list):
        return False
    for row in replies:
        if not isinstance(row, dict):
            continue
        tool = str(row.get("tool") or row.get("command_name") or "").strip().lower()
        if tool not in {"ccb.observation_request", "ccb_observation_request", "ccb.observation_catalog", "ccb_observation_catalog"}:
            continue
        result = row.get("result")
        if not isinstance(result, dict):
            continue
        status = str(result.get("status", "")).strip().lower()
        if status in {"partial", "blocked", "failed", "deferred", "missing", "unavailable"}:
            return True
        receipt = result.get("audit_receipt")
        if isinstance(receipt, dict) and str(receipt.get("status", "")).strip().lower() in {"partial", "blocked", "failed"}:
            return True
    return False


def agent_model_service_failure(response: dict[str, Any]) -> str:
    """Return a transport/provider failure without treating it as model evidence."""
    stop_reason = str(response.get("stop_reason", "")).strip().lower()
    if stop_reason in {"transient_llm_error", "llm_service_error", "provider_error", "transport_failure"}:
        return stop_reason
    error = first_text(response.get("error"))
    if not error:
        return ""
    lowered = error.lower()
    markers = (
        "llm http error",
        "http error 5",
        "bad gateway",
        "gateway timeout",
        "connection refused",
        "connection reset",
        "dial tcp",
        "read tcp",
        "wsarecv",
        "connection attempt failed",
        "connected host has failed to respond",
        "i/o timeout",
        "context deadline exceeded",
        "socket",
        # VitAgent surfaces provider-side concurrency/resource rejection as a
        # localized error. It is infrastructure evidence, not a model answer.
        "请求体并发内存预算不足",
        "request body concurrency memory budget",
    )
    return error if any(marker in lowered for marker in markers) else ""


def _executed_kernel_replies(response: dict[str, Any]) -> list[dict[str, Any]]:
    """Return kernel receipts from both the response and persisted free-state actions.

    A free-state continuation stores the authoritative action receipt under
    ``workflow_data.free_state_reasoning_loop.actions[].receipt``.  The HTTP
    response may not mirror that list at its top level, so auditing only
    ``response.executed_kernel_reply`` incorrectly reports a successful action
    as evidence-free.
    """
    replies: list[dict[str, Any]] = []
    seen: set[int] = set()

    def add(value: Any) -> None:
        if not isinstance(value, list):
            return
        for item in value:
            if not isinstance(item, dict) or id(item) in seen:
                continue
            seen.add(id(item))
            replies.append(item)

    add(response.get("executed_kernel_reply"))
    workflow = response.get("workflow_data")
    if not isinstance(workflow, dict):
        return replies
    loop = workflow.get("free_state_reasoning_loop")
    if not isinstance(loop, dict):
        return replies
    actions = loop.get("actions")
    if not isinstance(actions, list):
        return replies
    for action in actions:
        if not isinstance(action, dict):
            continue
        receipt = action.get("receipt")
        if isinstance(receipt, dict):
            add(receipt.get("executed_kernel_reply"))
    return replies


def evidence_counts(response_rows: list[dict[str, Any]]) -> dict[str, int]:
    keys = (
        "model_observation_receipts", "semantic_intent_artifacts", "candidate_sets",
        "model_selected_identifiers", "pca_preload_receipts", "pca_postload_receipts",
        "typed_controller_receipts", "confirmation_receipts", "transaction_receipts",
        "parameter_readbacks", "snapshot_verifications", "rollback_receipts",
        "post_action_observation_receipts", "ab_results",
    )
    counts = {key: 0 for key in keys}
    for row in response_rows:
        sources = [row]
        if isinstance(row.get("execution_evidence"), dict):
            sources.append(row["execution_evidence"])
        if isinstance(row.get("free_state"), dict):
            sources.append(row["free_state"])
        workflow = row.get("workflow_data") if isinstance(row.get("workflow_data"), dict) else {}
        loop = workflow.get("free_state_reasoning_loop") if isinstance(workflow.get("free_state_reasoning_loop"), dict) else {}
        decision = loop.get("latest_decision") if isinstance(loop.get("latest_decision"), dict) else {}
        if decision:
            sources.append(decision)
        for key in keys:
            response_count = 0
            for source in sources:
                value = source.get(key)
                if isinstance(value, list):
                    response_count = max(response_count, len(value))
                elif isinstance(value, dict):
                    response_count = max(response_count, 1)
                elif value is not None and key in source:
                    response_count = max(response_count, 1)
                nested_counts = source.get("counts")
                if isinstance(nested_counts, dict) and key in nested_counts:
                    try:
                        response_count = max(response_count, int(nested_counts[key]))
                    except (TypeError, ValueError):
                        pass
            counts[key] += response_count
        for receipt in _executed_kernel_replies(row):
            if str(receipt.get("status", "")).strip().lower() not in {"ok", "success", "completed"}:
                continue
            tool = str(receipt.get("tool") or receipt.get("command_name") or "").strip().lower()
            if tool in {"ccb.observation_request", "ccb_observation_request"}:
                counts["model_observation_receipts"] += 1
            elif tool in {"mix.observe", "mix_observe"}:
                counts["post_action_observation_receipts"] += 1
            elif "candidate" in tool:
                counts["candidate_sets"] += 1
            elif "pca" in tool:
                counts["pca_preload_receipts"] += 1
            elif "confirm" in tool:
                counts["confirmation_receipts"] += 1
            elif "readback" in tool:
                counts["parameter_readbacks"] += 1
            elif "snapshot" in tool or "verify" in tool:
                counts["snapshot_verifications"] += 1
            elif "rollback" in tool or "undo" in tool:
                counts["rollback_receipts"] += 1
            elif "apply" in tool or "propose_tick" in tool or "transaction" in tool or "control" in tool:
                counts["transaction_receipts"] += 1
    return counts


FAMILY_ALIASES = {
    "eq": "static_eq", "static_eq": "static_eq",
    "compressor": "broadband_compressor", "broadband_compressor": "broadband_compressor",
    "limiter": "limiter",
    "gate": "gate_expander", "expander": "gate_expander", "gate_expander": "gate_expander",
    "deesser": "de_esser", "de-esser": "de_esser", "de_esser": "de_esser",
    "transient": "transient_shaper", "transient_shaper": "transient_shaper",
    "multiband": "multiband_dynamics", "multiband_dynamics": "multiband_dynamics",
}


def model_decision_roots(response: dict[str, Any]) -> list[dict[str, Any]]:
    roots = [response]
    for key in ("free_state", "decision", "latest_decision"):
        value = response.get(key)
        if isinstance(value, dict):
            roots.append(value)
            if isinstance(value.get("semantic_processor_intent"), dict):
                roots.append(value["semantic_processor_intent"])
    workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    for container in (workflow, workflow.get("request_context")):
        if not isinstance(container, dict):
            continue
        loop = container.get("free_state_reasoning_loop")
        if isinstance(loop, dict) and isinstance(loop.get("latest_decision"), dict):
            roots.append(loop["latest_decision"])
            if isinstance(loop["latest_decision"].get("semantic_processor_intent"), dict):
                roots.append(loop["latest_decision"]["semantic_processor_intent"])
    return roots


def model_decision_family(response: dict[str, Any]) -> str:
    roots = model_decision_roots(response)
    for root in roots:
        for key in ("family", "processor_family", "selected_family", "processor_type"):
            value = root.get(key)
            normalized = FAMILY_ALIASES.get(str(value or "").strip().lower(), "")
            if normalized:
                return normalized
    return ""


def model_declares_processor(response: dict[str, Any]) -> bool:
    for root in model_decision_roots(response):
        for key in ("family", "processor_family", "selected_family", "processor_type"):
            if str(root.get(key) or "").strip():
                return True
        if isinstance(root.get("semantic_processor_intent"), dict):
            return True
    return False


def diagnostic_from_response(response: dict[str, Any]) -> dict[str, Any] | None:
    decisions: list[dict[str, Any]] = []
    if isinstance(response.get("free_state"), dict):
        decisions.append(response["free_state"])
    workflow = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    for container in (workflow, workflow.get("request_context")):
        if not isinstance(container, dict):
            continue
        loop = container.get("free_state_reasoning_loop")
        if isinstance(loop, dict) and isinstance(loop.get("latest_decision"), dict):
            decisions.append(loop["latest_decision"])
    value = response.get("diagnostic")
    if isinstance(value, dict):
        return value
    for decision in decisions:
        if isinstance(decision.get("diagnostic"), dict):
            return decision["diagnostic"]
    return None


def diagnostic_response_issue(response: dict[str, Any], known_evidence_refs: set[str] | None = None) -> str:
    diagnostic = diagnostic_from_response(response)
    if diagnostic is None:
        return "diagnostic-only response omitted free_state_diagnostic.v1"
    if str(diagnostic.get("schema_version", "")).strip() != "free_state_diagnostic.v1":
        return "diagnostic-only response has an invalid diagnostic schema"
    status = str(diagnostic.get("status", "")).strip().lower()
    if status not in {"confirmed", "ruled_out", "unresolved"}:
        return "diagnostic-only response has an invalid diagnostic status"
    findings = diagnostic.get("findings", [])
    if not isinstance(findings, list):
        return "diagnostic-only findings must be an array"
    if status != "unresolved" and not findings:
        return "diagnostic-only confirmed/ruled_out conclusions require findings"
    for finding in findings:
        if not isinstance(finding, dict) or not str(finding.get("statement", "")).strip():
            return "diagnostic-only findings require model-authored statements"
        refs = finding.get("evidence_refs")
        if not isinstance(refs, list) or not refs or any(not str(ref).strip() for ref in refs):
            return "diagnostic-only findings require evidence_refs"
        if known_evidence_refs is not None and any(str(ref).strip() not in known_evidence_refs for ref in refs):
            return "diagnostic-only findings reference evidence not returned by a successful CCB observation"
    if model_declares_processor(response):
        return "diagnostic-only response must not select a processor family"
    return ""


def diagnostic_observation_evidence(response_rows: list[dict[str, Any]]) -> tuple[set[str], int]:
    """Return only model-citable IDs from successful CCB observation bundles."""
    refs: set[str] = set()
    receipt_count = 0
    for response in response_rows:
        replies = response.get("executed_kernel_reply")
        if not isinstance(replies, list):
            continue
        for receipt in replies:
            if not isinstance(receipt, dict) or str(receipt.get("status", "")).strip().lower() not in {"ok", "success", "completed"}:
                continue
            tool = str(receipt.get("tool") or receipt.get("command_name") or "").strip().lower()
            if tool not in {"ccb.observation_request", "ccb_observation_request"}:
                continue
            result = receipt.get("result") if isinstance(receipt.get("result"), dict) else {}
            bundle = result.get("bundle") if isinstance(result.get("bundle"), dict) else result
            status = str(bundle.get("status") or result.get("status") or "").strip().lower()
            if status not in {"ready", "partial"}:
                continue
            receipt_count += 1
            observation_id = str(bundle.get("observation_id") or result.get("observation_id") or "").strip()
            if observation_id:
                refs.add(observation_id)
            for source in (bundle, result):
                values = source.get("evidence_refs")
                if isinstance(values, list):
                    refs.update(str(value).strip() for value in values if str(value).strip())
    return refs, receipt_count


def diagnostic_forbidden_activity(response_rows: list[dict[str, Any]]) -> str:
    counts = evidence_counts(response_rows)
    forbidden = (
        "semantic_intent_artifacts", "candidate_sets", "model_selected_identifiers",
        "pca_preload_receipts", "pca_postload_receipts", "typed_controller_receipts",
        "confirmation_receipts", "transaction_receipts", "parameter_readbacks",
        "snapshot_verifications", "rollback_receipts", "post_action_observation_receipts", "ab_results",
    )
    present = [key for key in forbidden if counts[key] > 0]
    if present:
        return "diagnostic-only run crossed the read-only boundary: " + ", ".join(present)
    for response in response_rows:
        if model_declares_processor(response):
            return "diagnostic-only run selected a processor family"
        if response.get("requires_confirmation") or response.get("pending_interaction_id") or response.get("pending_action"):
            return "diagnostic-only run created a pending action or confirmation"
        if isinstance(response.get("commands"), list) and response["commands"]:
            return "diagnostic-only run emitted executable commands"
        replies = response.get("executed_kernel_reply")
        if not isinstance(replies, list):
            continue
        for receipt in replies:
            if not isinstance(receipt, dict):
                continue
            tool = str(receipt.get("tool") or receipt.get("command_name") or "").strip().lower()
            if tool and tool not in {
                "ccb.observation_catalog", "ccb_observation_catalog",
                "ccb.observation_request", "ccb_observation_request",
            }:
                return f"diagnostic-only run executed forbidden tool {tool}"
    return ""


def diagnostic_completion_issue(response: dict[str, Any], response_rows: list[dict[str, Any]]) -> str:
    refs, receipt_count = diagnostic_observation_evidence(response_rows)
    issue = diagnostic_response_issue(response, refs)
    if issue:
        return issue
    if receipt_count <= 0:
        return "diagnostic-only conclusion requires a successful CCB observation receipt"
    return diagnostic_forbidden_activity(response_rows)


def agent_model_service_resume_allowed(error: str, resume_count: int, maximum_resumes: int) -> bool:
    """Permit only the contract-bounded same-conversation service recovery path."""
    return bool(str(error).strip()) and resume_count < maximum_resumes


def receipt_ids(response: dict[str, Any]) -> list[str]:
    ids: list[str] = []
    for source in (response, response.get("execution_evidence")):
        if not isinstance(source, dict):
            continue
        values = source.get("transaction_receipts")
        if isinstance(values, dict):
            values = [values]
        if isinstance(values, list):
            for value in values:
                if isinstance(value, dict):
                    token = str(value.get("receipt_id") or value.get("transaction_id") or "").strip()
                    if token:
                        ids.append(token)
    return ids


def preliminary_agent_conformance(
    response_rows: list[dict[str, Any]],
    outcome: str,
    action_count: int,
    timeout_rows: list[dict[str, Any]],
    l3_proposal_only: bool = False,
) -> str:
    """Record only evidence-supported preliminary status; evaluator remains authoritative."""
    if timeout_rows:
        return "unobservable"
    if l3_proposal_only:
        return "pass" if outcome == "l3_proposal_admitted" and improvement_proposal_metrics(response_rows)["admitted"] else "fail"
    counts = evidence_counts(response_rows)
    closure = minimal_closure_metrics(response_rows)
    if outcome == "diagnostic_conclusion":
        return "pass" if counts["model_observation_receipts"] > 0 else "unobservable"
    if outcome in {"model_no_op", "model_blocked"}:
        if closure["candidate_discovered"] and not closure["execution_direction_reached"]:
            return "fail"
        return "pass" if counts["model_observation_receipts"] > 0 else "unobservable"
    if outcome == "satisfied":
        required = ("model_observation_receipts", "transaction_receipts", "post_action_observation_receipts")
        return "pass" if action_count > 0 and all(counts[key] > 0 for key in required) else "unobservable"
    return "unobservable"


def execute_agent_run(args: argparse.Namespace, runtime: dict[str, Any], manifest: dict[str, Any], run_id: str, artifact_dir: Path) -> dict[str, Any]:
    started = now_iso()
    started_epoch = time.monotonic()
    limits = runtime["limits"]
    global_deadline = started_epoch + float(limits.get("global_timeout_seconds", 7200))
    projects: list[dict[str, Any]] = []
    infrastructure_failures: list[dict[str, Any]] = []
    continuation_results: list[dict[str, Any]] = []
    total_resume_count = 0
    for case in manifest.get("cases", []):
        if time.monotonic() >= global_deadline:
            break
        case_id = str(case["public_case_id"])
        conversation_id = f"smoke_{run_id}_{case_id}"
        intent_hash = sha256_bytes((runtime["initial_prompt"] + "\n" + case_id).encode("utf-8"))
        checkpoint = CheckpointStore(artifact_dir / "checkpoints" / f"{case_id}.json", run_id, case_id, conversation_id, intent_hash)
        resumed = False
        resume_count = 0
        maximum_resumes = int(limits.get("maximum_transport_resumes_per_project", 2))
        if args.resume_checkpoint:
            resume_path = Path(args.resume_checkpoint).resolve()
            saved = load_json(resume_path)
            if str(saved.get("public_case_id")) != case_id:
                continue
            if str(saved.get("original_intent_hash")) != intent_hash:
                raise RuntimeError("resume original_intent_hash mismatch")
            resume_count = int(saved.get("resume_count", 0)) + 1
            if resume_count > maximum_resumes:
                raise RuntimeError(f"resume limit exceeded for {case_id}: {resume_count}>{maximum_resumes}")
            checkpoint.value.update(saved)
            checkpoint.value["resume_count"] = resume_count
            conversation_id = str(saved.get("conversation_id"))
            resumed = True
            atomic_json(checkpoint.path, checkpoint.value)
        context = public_context(runtime, manifest, case)
        response_rows: list[dict[str, Any]] = []
        raw_dir = artifact_dir / "responses" / case_id
        status = "inconclusive"
        timeout_rows: list[dict[str, Any]] = []
        project_deadline = time.monotonic() + float(limits.get("project_timeout_seconds", 3000))
        action_count = 0
        family_action_counts: dict[str, int] = {}
        model_observation_count = 0
        diagnostic_conclusion: dict[str, Any] | None = None
        project_infrastructure_failures: list[dict[str, Any]] = []
        checkpoint.transition("project_importing", project_path=case.get("project_path", ""))
        try:
            setup = prepare_isolated_project(args.agent_http, case, limits, artifact_dir)
        except Exception as exc:  # noqa: BLE001 - fail closed before any Agent turn.
            failure = {
                "scope": "project_setup",
                "public_case_id": case_id,
                "error": str(exc),
            }
            infrastructure_failures.append(failure)
            atomic_json(artifact_dir / "project_setup" / f"{case_id}.json", {"status": "blocked", **failure})
            checkpoint.transition("project_terminal", outcome="inconclusive", reason="project_setup_failed", error=str(exc))
            projects.append({
                "public_case_id": case_id,
                "conversation_id": conversation_id,
                "original_intent_hash": intent_hash,
                "project_lifecycle": "project_setup_failed",
                "model_turns": 0,
                "model_outcomes": ["inconclusive"],
                "agent_conformance": "unobservable",
                "execution_evidence": {"response_artifacts": [], "counts": evidence_counts([])},
                "timeouts": [],
                "infrastructure_failure": failure,
                "checkpoint_history": str(checkpoint.path),
                "terminal_state": "project_terminal",
                "diagnostic_conclusion": None,
            })
            break
        checkpoint.transition("dad_waiting", **{key: value for key, value in setup.items() if key.startswith("dad_") or key == "track_waveform_envelope_count"})
        checkpoint.transition("agent_started", conversation_id=conversation_id)
        message = runtime["continuation_prompt"] if resumed else runtime["initial_prompt"]
        pending_interaction: dict[str, Any] | None = None
        for turn in range(int(limits.get("maximum_model_turns_per_project", 18))):
            if time.monotonic() >= global_deadline:
                timeout_rows.append({"scope": "global", "turn": turn})
                status = "inconclusive"
                break
            if time.monotonic() >= project_deadline:
                timeout_rows.append({"scope": "project", "turn": turn})
                status = "inconclusive"
                break
            if pending_interaction is not None:
                checkpoint.transition("parameter_confirmation", turn=turn, interaction_id=pending_interaction["interaction_id"])
            else:
                checkpoint.transition("model_observing" if turn == 0 else "model_deciding", turn=turn)
            turn_started = time.monotonic()
            try:
                if pending_interaction is not None:
                    response = request_json(
                        "POST", args.agent_http.rstrip("/") + "/agent/interaction/respond",
                        pending_interaction,
                        float(limits.get("model_turn_timeout_seconds", 420)),
                    )
                    request_kind = "interaction"
                    pending_interaction = None
                else:
                    response = request_json(
                        "POST", args.agent_http.rstrip("/") + "/agent/chat",
                        {"conversation_id": conversation_id, "message": message, "context": context},
                        float(limits.get("model_turn_timeout_seconds", 420)),
                    )
                    request_kind = "chat"
            except (urllib.error.URLError, TimeoutError, OSError) as exc:
                timeout_rows.append({"scope": "transport", "turn": turn, "error": str(exc)})
                failure = {"scope": "transport", "public_case_id": case_id, "turn": turn, "error": str(exc)}
                project_infrastructure_failures.append(failure)
                infrastructure_failures.append(failure)
                status = "inconclusive"
                checkpoint.transition("model_outcome", outcome=status, reason="transport_failure", error=str(exc))
                break
            response_rows.append(response)
            raw_dir.mkdir(parents=True, exist_ok=True)
            response_artifact = raw_dir / f"{turn:02d}_{request_kind}.json"
            atomic_json(response_artifact, response)
            checkpoint.value["last_response_artifact"] = str(response_artifact)
            checkpoint.value["updated_at"] = now_iso()
            atomic_json(checkpoint.path, checkpoint.value)
            counts = evidence_counts([response])
            model_observation_count += counts["model_observation_receipts"]
            actions_this_turn = max(counts["transaction_receipts"], counts["typed_controller_receipts"])
            selected_family = model_decision_family(response)
            action_count += actions_this_turn
            if actions_this_turn and model_observation_count <= 0:
                status = "model_blocked"
                checkpoint.transition("model_outcome", outcome=status, reason="first_action_before_model_requested_observation")
                break
            if selected_family and actions_this_turn:
                family_action_counts[selected_family] = family_action_counts.get(selected_family, 0) + actions_this_turn
            if action_count > int(limits.get("maximum_total_actions_per_project", 6)):
                status = "model_blocked"
                checkpoint.transition("model_outcome", outcome=status, reason="maximum_total_actions_per_project")
                break
            if selected_family and family_action_counts[selected_family] > int(limits.get("maximum_actions_per_family_per_project", 2)):
                status = "model_blocked"
                checkpoint.transition("model_outcome", outcome=status, reason="maximum_actions_per_family_per_project", family=selected_family)
                break
            checkpoint.value["last_response_artifact"] = str(response_artifact)
            for receipt_id in receipt_ids(response):
                if receipt_id not in checkpoint.value["applied_action_receipt_ids"]:
                    checkpoint.value["applied_action_receipt_ids"].append(receipt_id)
            response_revision = response.get("project_revision")
            if response_revision not in (None, ""):
                checkpoint.value["project_revision"] = str(response_revision)
            checkpoint.value["updated_at"] = now_iso()
            atomic_json(checkpoint.path, checkpoint.value)
            receipt_states = (
                ("candidate_query", "candidate_sets"),
                ("model_identifier_selection", "model_selected_identifiers"),
                ("load_confirmation", "confirmation_receipts"),
                ("preload_pca_recheck", "pca_preload_receipts"),
                ("postload_qualification", "pca_postload_receipts"),
                ("postload_qualification", "typed_controller_receipts"),
                ("control_planning", "semantic_intent_artifacts"),
                ("parameter_confirmation", "confirmation_receipts"),
                ("typed_execution", "transaction_receipts"),
                ("readback_and_snapshot_verification", "parameter_readbacks"),
                ("readback_and_snapshot_verification", "snapshot_verifications"),
                ("readback_and_snapshot_verification", "rollback_receipts"),
                ("model_post_action_observation", "post_action_observation_receipts"),
            )
            for state_name, field_name in receipt_states:
                if field_name in response:
                    checkpoint.transition(state_name, receipt_field=field_name)
            action_fields_present = any(key in response for key in ("transaction_receipts", "typed_controller_receipts", "pending_action"))
            action_limit = float(limits.get("governed_action_timeout_seconds", 600))
            if action_fields_present and time.monotonic() - turn_started > action_limit:
                timeout_rows.append({"scope": "governed_action", "turn": turn})
                status = "model_blocked"
                break
            if time.monotonic() - turn_started > float(limits.get("model_turn_timeout_seconds", 420)):
                timeout_rows.append({"scope": "model_turn", "turn": turn})
                status = "inconclusive"
                break
            if service_error := agent_model_service_failure(response):
                if agent_model_service_resume_allowed(service_error, resume_count, maximum_resumes):
                    resume_count += 1
                    total_resume_count += 1
                    checkpoint.value["resume_count"] = resume_count
                    checkpoint.transition(
                        "model_deciding",
                        turn=turn,
                        transport_resume=resume_count,
                        error=service_error,
                        same_conversation=True,
                    )
                    continuation_results.append({
                        "public_case_id": case_id,
                        "turn": turn,
                        "resume_count": resume_count,
                        "error": service_error,
                        "status": "continued",
                        "same_conversation": True,
                        "original_intent_hash_matches": True,
                    })
                    message = runtime["continuation_prompt"]
                    continue
                failure = {
                    "scope": "agent_model_service",
                    "public_case_id": case_id,
                    "turn": turn,
                    "error": service_error,
                }
                project_infrastructure_failures.append(failure)
                infrastructure_failures.append(failure)
                timeout_rows.append({"scope": "agent_model_service", "turn": turn, "error": service_error})
                continuation_results.append({
                    "public_case_id": case_id,
                    "turn": turn,
                    "resume_count": resume_count,
                    "error": service_error,
                    "status": "resume_limit_exhausted",
                    "same_conversation": True,
                    "original_intent_hash_matches": True,
                })
                status = "inconclusive"
                checkpoint.transition("model_outcome", outcome=status, reason="agent_model_service_failure", error=service_error)
                break
            if runtime.get("diagnostic_only"):
                diagnostic_issue = diagnostic_completion_issue(response, response_rows)
                if diagnostic_issue == "":
                    checkpoint.transition("model_outcome", outcome="diagnostic_conclusion")
                    status = "diagnostic_conclusion"
                    checkpoint.value["diagnostic_conclusion"] = diagnostic_from_response(response)
                    diagnostic_conclusion = diagnostic_from_response(response)
                    checkpoint.value["updated_at"] = now_iso()
                    atomic_json(checkpoint.path, checkpoint.value)
                    break
                if response_status(response) in {"completed", "satisfied", "blocked", "model_blocked", "error"}:
                    status = "diagnostic_invalid"
                    checkpoint.transition("model_outcome", outcome=status, reason=diagnostic_issue)
                    break
            if runtime.get("l3_proposal_only"):
                l3_metrics = improvement_proposal_metrics(response_rows)
                if l3_metrics["admitted"]:
                    checkpoint.transition("model_outcome", outcome="l3_proposal_admitted")
                    status = "l3_proposal_admitted"
                    break
            interaction_submission, interaction_issue = recommended_interaction_submission(response)
            if interaction_issue:
                status = "model_blocked"
                checkpoint.transition("model_outcome", outcome=status, reason=interaction_issue)
                break
            if interaction_submission is not None:
                checkpoint.transition("parameter_confirmation", interaction_id=interaction_submission["interaction_id"], action_id=interaction_submission["action_id"])
                pending_interaction = interaction_submission
                continue
            current = response_status(response)
            if current in {"completed", "satisfied", "done"}:
                if runtime.get("l3_full_workflow") and action_count <= 0:
                    status = "model_no_op"
                    checkpoint.transition("model_outcome", outcome=status, reason="satisfied_without_governed_action")
                    break
                if action_count and counts["post_action_observation_receipts"] <= 0:
                    status = "model_blocked"
                    checkpoint.transition("model_outcome", outcome=status, reason="satisfied_without_fresh_post_action_observation")
                    break
                status = "satisfied"
                checkpoint.transition("model_outcome", outcome=status)
                break
            if current in {"waiting_clarification", "model_no_op", "no_op"}:
                if action_count and counts["post_action_observation_receipts"] <= 0:
                    status = "model_blocked"
                    checkpoint.transition("model_outcome", outcome=status, reason="terminal_decision_without_fresh_post_action_observation")
                    break
                status = "model_no_op"
                checkpoint.transition("model_outcome", outcome=status)
                break
            if current in {"blocked", "model_blocked", "error", "failed"}:
                status = "error" if current in {"error", "failed"} else "model_blocked"
                checkpoint.transition("model_outcome", outcome=status, reason="agent_terminal_failure" if status == "error" else "")
                break
            if counts["post_action_observation_receipts"] > 0:
                checkpoint.transition("model_post_action_observation", turn=turn)
            elif turn == 0:
                checkpoint.transition("candidate_query", turn=turn)
            else:
                checkpoint.transition("model_deciding", turn=turn)
            message = runtime["continuation_prompt"]
        else:
            status = "inconclusive"
        checkpoint.transition("project_terminal", outcome=status)
        closure = minimal_closure_metrics(response_rows)
        conformance = preliminary_agent_conformance(response_rows, status, action_count, timeout_rows, bool(runtime.get("l3_proposal_only")))
        projects.append({
            "public_case_id": case_id,
            "conversation_id": conversation_id,
            "original_intent_hash": intent_hash,
            "project_lifecycle": "completed",
            "model_turns": len(response_rows),
            "model_outcomes": [status],
            "agent_conformance": conformance,
            "minimal_closure": closure,
            "agent_conformance_failures": [closure["failure_reason"]] if conformance == "fail" and closure["failure_reason"] else [],
            "execution_evidence": {"response_artifacts": [str(path) for path in sorted(raw_dir.glob("*.json"))], "counts": evidence_counts(response_rows)},
            "orchestration": orchestration_metrics(response_rows),
            "timeouts": timeout_rows,
            "infrastructure_failure": project_infrastructure_failures[0] if project_infrastructure_failures else None,
            "checkpoint_history": str(checkpoint.path),
            "terminal_state": "project_terminal",
            "diagnostic_conclusion": diagnostic_conclusion,
        })
    ended = now_iso()
    terminal_successes = {"diagnostic_conclusion"} if runtime.get("diagnostic_only") else ({"l3_proposal_admitted"} if runtime.get("l3_proposal_only") else {"satisfied"})
    run_status = "infrastructure_failure" if infrastructure_failures else (
        "completed" if projects and all(row["model_outcomes"][-1] in terminal_successes for row in projects) else "partial_report"
    )
    diagnostic_inconclusive = bool(runtime.get("diagnostic_only")) and any(
        row["model_outcomes"][-1] != "diagnostic_conclusion" for row in projects
    )
    return {
        "schema_version": RUN_REPORT_SCHEMA,
        "contract_id": runtime["contract_id"],
        "fixture_set_id": manifest.get("fixture_set_id"),
        "run_id": run_id,
        "status": run_status,
        "started_at": started,
        "ended_at": ended,
        "elapsed_seconds": elapsed_since(started, ended),
        "sealed_truth_opened_by_runner": False,
        "projects": projects,
        "family_coverage": {"status": "model_owned", "families": []},
        "partial_report": {"is_partial": run_status != "completed", "closed_at": ended, "last_completed_state": "project_terminal", "failure_or_timeout": "agent_model_service_failure" if any(row.get("scope") == "agent_model_service" for row in infrastructure_failures) else ("project_setup_failure" if infrastructure_failures else ("diagnostic_inconclusive" if diagnostic_inconclusive else ("transport_or_timeout" if any(bool(row.get("timeouts")) for row in projects) else ""))), "evidence_complete_through_state": "project_terminal", "missing_evidence": (["Agent model service response"] if infrastructure_failures else (["free_state_diagnostic.v1 terminal conclusion"] if diagnostic_inconclusive else [])), "resume_eligible": False if infrastructure_failures or diagnostic_inconclusive else any(bool(row.get("timeouts")) for row in projects)},
        "resume_continuation": {"resumed": bool(args.resume_checkpoint) or total_resume_count > 0, "resume_count": total_resume_count + (1 if args.resume_checkpoint else 0), "checkpoint_id": str(args.resume_checkpoint or ""), "same_conversation": True, "original_intent_hash_matches": True, "mutation_replay_guard": "receipts recorded; no replay performed", "continuation_results": continuation_results},
        "infrastructure_failures": infrastructure_failures,
        "replay_of_run_id": str(args.replay_of_run_id or ""),
        "preflight": {"status": "passed", "official_run_budget_consumed": not bool(runtime.get("diagnostic_only"))},
    }


def run(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(args.repo_root).resolve()
    contract_path = Path(args.contract).resolve()
    manifest_path = Path(args.fixture_manifest).resolve()
    runtime = getattr(args, "_diagnostic_runtime", None) or getattr(args, "_l3_full_runtime", None) or getattr(args, "_l3_runtime", None) or runtime_contract(contract_path)
    full_manifest = load_public_manifest(manifest_path)
    manifest = full_manifest
    requested_case = str(getattr(args, "case", "") or "").strip()
    if requested_case:
        cases = [case for case in manifest.get("cases", []) if isinstance(case, dict) and str(case.get("public_case_id", "")) == requested_case]
        if not cases:
            raise ValueError(f"unknown public case: {requested_case}")
        manifest = dict(manifest)
        manifest["cases"] = cases
    run_id = args.run_id or f"smoke_{int(time.time())}_{uuid.uuid4().hex[:8]}"
    artifact_dir = Path(args.artifact_dir or manifest_path.parent / "runs" / run_id).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    project_report = Path(args.project_report).resolve() if args.project_report else None
    runtime_receipts = Path(args.runtime_receipts).resolve() if args.runtime_receipts else None
    lifecycle_receipt = Path(args.godot_lifecycle_receipt).resolve() if args.godot_lifecycle_receipt else None
    preflight_result = preflight(
        repo_root,
        contract_path,
        manifest_path,
        project_report=project_report,
        runtime_receipts=runtime_receipts,
        godot_lifecycle_receipt=lifecycle_receipt,
        runtime=runtime,
        manifest=manifest,
        agent_http=args.agent_http,
    )
    atomic_json(artifact_dir / "preflight.json", preflight_result)
    started = now_iso()
    if preflight_result["status"] != "passed":
        report = make_blocked_report(runtime, manifest, run_id, started, now_iso(), preflight_result, artifact_dir / "checkpoints")
        atomic_json(artifact_dir / "run_report.json", report)
        return {"status": "preflight_blocked", "artifact_dir": str(artifact_dir), "preflight": str(artifact_dir / "preflight.json"), "run_report": str(artifact_dir / "run_report.json"), "blocker_count": len(preflight_result["blockers"])}
    if not args.execute:
        report = {
            "schema_version": RUN_REPORT_SCHEMA,
            "contract_id": runtime["contract_id"],
            "fixture_set_id": manifest.get("fixture_set_id"),
            "run_id": run_id,
            "status": "preflight_passed_not_executed",
            "started_at": started,
            "ended_at": now_iso(),
            "elapsed_seconds": elapsed_since(started, now_iso()),
            "sealed_truth_opened_by_runner": False,
            "official_run_budget_consumed": False,
            "projects": [],
            "family_coverage": {"status": "not_exercised_by_model", "families": []},
            "partial_report": {"is_partial": True, "closed_at": now_iso(), "last_completed_state": "fixture_ready", "failure_or_timeout": "formal_execution_not_requested", "evidence_complete_through_state": "fixture_ready", "missing_evidence": [], "resume_eligible": False},
            "preflight": preflight_result,
        }
        atomic_json(artifact_dir / "run_report.json", report)
        return {"status": report["status"], "artifact_dir": str(artifact_dir), "preflight": str(artifact_dir / "preflight.json"), "run_report": str(artifact_dir / "run_report.json"), "blocker_count": 0}
    report = execute_agent_run(args, runtime, manifest, run_id, artifact_dir)
    atomic_json(artifact_dir / "run_report.json", report)
    return {"status": report["status"], "artifact_dir": str(artifact_dir), "run_report": str(artifact_dir / "run_report.json")}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parent.parent))
    parser.add_argument("--contract", default=str(Path(__file__).with_name("semantic_processor_project_smoke_contract.json")))
    parser.add_argument("--fixture-manifest", required=True)
    parser.add_argument("--project-report", default="")
    parser.add_argument("--runtime-receipts", default="")
    parser.add_argument("--godot-lifecycle-receipt", default="")
    parser.add_argument("--artifact-dir", default="")
    parser.add_argument("--run-id", default="")
    parser.add_argument("--execute", action="store_true")
    parser.add_argument("--diagnostic-only", action="store_true", help="require a read-only free_state_diagnostic.v1 conclusion")
    parser.add_argument("--l3-proposal-only", action="store_true", help="run the bounded L3 improvement-proposal admission smoke without full family/DOM preflight")
    parser.add_argument("--l3-full-workflow", action="store_true", help="run a fresh p01 L3 session through product confirmation and governed execution")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--resume-checkpoint", default="")
    parser.add_argument("--replay-of-run-id", default="")
    parser.add_argument("--case", default="", help="run one public case in a bounded diagnostic replay")
    args = parser.parse_args()
    try:
        if args.diagnostic_only:
            # This is an opt-in short diagnostic contract. It never authorizes
            # mutation; the Agent prompt is intentionally neutral and asks only
            # for an evidence-backed terminal conclusion.
            runtime = diagnostic_runtime_contract(Path(args.contract).resolve())
            # run() reads the contract itself; write the opt-in override through
            # a private attribute consumed by run() below.
            args._diagnostic_runtime = runtime
        elif args.l3_proposal_only:
            args._l3_runtime = l3_proposal_runtime_contract(Path(args.contract).resolve())
        elif args.l3_full_workflow:
            args._l3_full_runtime = l3_full_workflow_runtime_contract(Path(args.contract).resolve())
        print(json.dumps(run(args), ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
