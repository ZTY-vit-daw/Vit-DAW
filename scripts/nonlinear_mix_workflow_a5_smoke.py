#!/usr/bin/env python3
"""Product-path smoke for non-linear B1/B2/B3/B4/C1 workflows on A5 copies.

The caller owns the Godot/Hub/Kernel/Agent lifecycle. Every scenario receives
its own full copy of the source project folder; the source project is hashed
before and after the suite and is never opened for mutation.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


CAPABILITIES: dict[str, dict[str, Any]] = {
    "B1": {
        "message": "执行 B1 全工程增益分级和素材电平校准",
        "context": {"agent_mode": "chat"},
    },
    "B2": {
        "message": "执行 B2 全工程静态平衡",
        "context": {"agent_mode": "chat", "capability_id": "static_mix.static_balance.v0", "interaction_mode": "propose"},
    },
    "B3": {
        "message": "执行 B3 全工程静态声像布局",
        "context": {"agent_mode": "chat", "capability_id": "static_mix.pan_layout.v0", "interaction_mode": "propose"},
    },
    "B4": {
        "message": "执行 B4 全工程低频关系处理",
        "context": {"agent_mode": "chat", "capability_id": "static_mix.low_end_relation.v0", "interaction_mode": "propose", "b4_execute": True},
    },
    "C1": {
        "message": "执行 C1 全工程频率清理",
        "context": {"agent_mode": "chat", "capability_id": "fine_mix.frequency_cleanup.v1", "interaction_mode": "propose", "c1_execute": True},
    },
}


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method)
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            response = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(response.get("status") or "").lower() in {"ok", "ready"}:
                return response
        except Exception as exc:  # noqa: BLE001 - retained in smoke diagnostics.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"agent did not become ready: {last_error}")


def invoke(base_url: str, tool: str, args: dict[str, Any], timeout: float) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "confirmed": True, "source": "nonlinear_mix_workflow_a5_smoke"},
        timeout,
    )
    require(str(response.get("status") or "").lower() in {"ok", "success", "completed"}, f"{tool} failed: {json.dumps(response, ensure_ascii=False)[:3000]}")
    return response


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else {}


def project_signature(base_url: str, timeout: float) -> list[dict[str, Any]]:
    state = result_map(invoke(base_url, "project.state", {}, timeout))
    rows: list[dict[str, Any]] = []
    for track in state.get("tracks", []):
        if not isinstance(track, dict):
            continue
        track_id = str(track.get("track_id") or track.get("id") or "").strip()
        if not track_id:
            continue
        clips = []
        for clip in track.get("clips") or []:
            if isinstance(clip, dict):
                clips.append({"clip_id": str(clip.get("clip_id") or clip.get("id") or ""), "gain_db": clip.get("clip_gain_db", clip.get("gain_db"))})
        rows.append({
            "track_id": track_id,
            "volume_db": track.get("volume_db", track.get("fader_db")),
            "pan": track.get("pan", track.get("pan_value")),
            "plugins": [str(row.get("plugin_id") or row.get("id") or "") for row in (track.get("plugins") or []) if isinstance(row, dict)],
            "clips": clips,
        })
    return sorted(rows, key=lambda row: row["track_id"])


def assert_project_history_open_clean(response: dict[str, Any], expected_project_path: Path, phase: str) -> None:
    history = response.get("project_history") if isinstance(response.get("project_history"), dict) else {}
    warnings = [str(value).strip() for value in history.get("warnings") or [] if str(value).strip()]
    require(not warnings, f"{phase} returned Project History warnings: {warnings}")
    actual_path = str(history.get("project_path") or "").strip()
    require(actual_path, f"{phase} omitted Project History project_path")
    require(Path(actual_path).resolve() == expected_project_path.resolve(), f"{phase} Project History path mismatch: {actual_path} != {expected_project_path}")


def workflow_stage(response: dict[str, Any]) -> str:
    data = response.get("workflow_data")
    return str(data.get("canary_stage") or data.get("stage") or "") if isinstance(data, dict) else ""


def successful_execution_tools(response: dict[str, Any]) -> list[str]:
    tools: list[str] = []
    for record in response.get("executed_kernel_reply") or []:
        if not isinstance(record, dict):
            continue
        status = str(record.get("status") or "").lower()
        error = str(record.get("error") or "").strip()
        require(status not in {"error", "failed"} and not error, f"tool execution failed: {json.dumps(record, ensure_ascii=False)[:3000]}")
        tool = str(record.get("tool") or record.get("command_name") or "").strip()
        if tool:
            tools.append(tool)
    return tools


def wait_for_audio_analysis(base_url: str, artifact_dir: Path, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    attempt = 0
    latest: dict[str, Any] = {}
    while time.monotonic() < deadline:
        latest = invoke(base_url, "project.audio_analysis_status", {"latest": True}, min(timeout, 120.0))
        write_json(artifact_dir / f"audio_analysis_wait_{attempt:02d}.json", latest)
        result = result_map(latest)
        job = result.get("analysis_job") if isinstance(result.get("analysis_job"), dict) else {}
        ready = job.get("dad_fact_ready_count")
        total = job.get("dad_fact_total_count", job.get("total_clips"))
        pending = job.get("dad_fact_pending_count", job.get("pending_clips"))
        completed_features = job.get("completed_feature_jobs")
        total_features = job.get("total_feature_jobs")
        dad_ready = isinstance(ready, (int, float)) and isinstance(total, (int, float)) and total > 0 and ready >= total and str(job.get("dad_fact_status") or "").lower() == "ready"
        features_ready = not isinstance(total_features, (int, float)) or total_features <= 0 or (isinstance(completed_features, (int, float)) and completed_features >= total_features)
        if dad_ready and features_ready and (not isinstance(pending, (int, float)) or pending <= 0):
            return latest
        attempt += 1
        time.sleep(0.5)
    raise RuntimeError(f"audio analysis did not become fully ready: {json.dumps(result_map(latest).get('analysis_job') or {}, ensure_ascii=False)[:3000]}")


def assert_capability_terminal(capability: str, response: dict[str, Any], tools: list[str]) -> dict[str, Any]:
    goal_status = str(response.get("goal_status") or "").lower()
    stage = workflow_stage(response).lower()
    workflow_data = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    require(goal_status == "completed", f"{capability} did not complete: status={goal_status} stage={stage} reply={response.get('reply')}")
    require(response.get("needs_confirmation") is not True, f"{capability} remained pending confirmation")
    if capability == "B1":
        require(any(tool.startswith("clip.gain.set") for tool in tools), f"B1 completed without applying clip-gain calibration: tools={tools} reply={response.get('reply')}")
        reply = str(response.get("reply") or "").lower()
        for forbidden in (
            "b1_2_reference_level_model_partial",
            "b1_2_source_level_admission_partial",
            "mix_observation_missing",
            "rlm_missing_selected_metric_for_some_tracks",
        ):
            require(forbidden not in reply, f"B1 returned a partial/no-action terminal result ({forbidden}): {response.get('reply')}")
    elif capability in {"B2", "B3"}:
        require(stage == "executed_verified", f"{capability} terminal stage is not executed_verified: stage={stage} data={json.dumps(workflow_data, ensure_ascii=False)[:3000]}")
    elif capability == "B4":
        require(stage in {"executed_verified", "no_treatment_required"}, f"B4 did not reach a justified terminal treatment result: stage={stage}")
    elif capability == "C1":
        require(stage in {"executed_verified", "no_static_eq_required"}, f"C1 did not reach a justified terminal treatment result: stage={stage}")
    return {"goal_status": goal_status, "stage": stage, "tools": tools, "mutation_performed": workflow_data.get("mutation_performed")}


def choose_interaction(response: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any]] | None:
    requests = [row for row in response.get("interaction_requests", []) if isinstance(row, dict)]
    for interaction in requests:
        actions = [row for row in interaction.get("actions", []) if isinstance(row, dict)]
        if not actions:
            continue
        chosen = next((row for row in actions if row.get("recommended") is True and "cancel" not in str(row.get("id") or "").lower()), None)
        if chosen is None:
            chosen = next((row for row in actions if str(row.get("id") or "").lower() in {"approve", "confirm", "apply"}), None)
        if chosen is None:
            chosen = next((row for row in actions if "cancel" not in str(row.get("id") or "").lower()), None)
        if chosen is not None:
            return interaction, chosen
    return None


def drive_capability(base_url: str, capability: str, conversation_id: str, artifact_dir: Path, timeout: float) -> dict[str, Any]:
    spec = CAPABILITIES[capability]
    response = request_json("POST", base_url.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": spec["message"], "context": dict(spec["context"])}, timeout)
    steps: list[dict[str, Any]] = []
    tools: list[str] = []
    for index in range(16):
        write_json(artifact_dir / f"{capability.lower()}_{index:02d}.json", response)
        tools.extend(successful_execution_tools(response))
        steps.append({
            "index": index,
            "status": response.get("status"),
            "goal_status": response.get("goal_status"),
            "workflow": response.get("workflow"),
            "stage": workflow_stage(response),
            "needs_confirmation": response.get("needs_confirmation"),
            "error": response.get("error"),
        })
        error = str(response.get("error") or "").strip()
        require(not error, f"{capability} failed: {error}; reply={response.get('reply')}")

        if workflow_stage(response).lower() == "band_analysis_triggered":
            wait_for_audio_analysis(base_url, artifact_dir / f"{capability.lower()}_analysis", timeout)
            response = request_json("POST", base_url.rstrip("/") + "/agent/chat", {"conversation_id": conversation_id, "message": spec["message"], "context": dict(spec["context"])}, timeout)
            continue

        selected = choose_interaction(response)
        if selected is not None:
            interaction, action = selected
            action_id = str(action.get("id") or "").strip()
            payload = action.get("value") if isinstance(action.get("value"), dict) else {}
            response = request_json(
                "POST",
                base_url.rstrip("/") + "/agent/interaction/respond",
                {"interaction_id": str(interaction.get("id") or ""), "action_id": action_id, "decision": action_id, "payload": payload},
                timeout,
            )
            continue

        if response.get("needs_confirmation") is True:
            plan_id = str(response.get("next_plan_id") or response.get("plan_id") or "").strip()
            require(plan_id, f"{capability} needs confirmation but returned no plan_id")
            response = request_json("POST", base_url.rstrip("/") + "/agent/confirm", {"plan_id": plan_id, "decision": "approve"}, timeout)
            continue

        goal_status = str(response.get("goal_status") or "").lower()
        require(goal_status not in {"failed", "cancelled"}, f"{capability} terminal status={goal_status}: {response.get('reply')}")
        require(goal_status not in {"waiting_confirmation", "waiting_clarification", "waiting_continue"}, f"{capability} stopped without an actionable continuation: {json.dumps(response, ensure_ascii=False)[:4000]}")
        terminal_assertion = assert_capability_terminal(capability, response, tools)
        return {"capability": capability, "conversation_id": conversation_id, "terminal": response, "steps": steps, "terminal_assertion": terminal_assertion}
    raise RuntimeError(f"{capability} exceeded continuation limit")


def assert_b1_peak_safe(base_url: str, timeout: float, artifact_dir: Path) -> dict[str, Any]:
    observed = invoke(base_url, "mix.observe", {
        "scope": "full_project", "project_context": True, "observation_only": True,
        "disclosure": "digest_catalog", "mom_intent": "project_multitrack_relation_observation",
        "mix_session_id": "nonlinear_a5_b1_peak_verify", "goal_text": "Verify B1 effective static peaks",
    }, timeout)
    write_json(artifact_dir / "b1_peak_verification_observation.json", observed)
    peaks: list[tuple[str, float]] = []

    def visit(value: Any) -> None:
        if isinstance(value, dict):
            if isinstance(value.get("effective_static_peak_dbfs"), (int, float)):
                peaks.append((str(value.get("track_id") or value.get("name") or "unknown"), float(value["effective_static_peak_dbfs"])))
            for child in value.values():
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)

    visit(result_map(observed))
    by_track: dict[str, float] = {}
    for track_id, peak in peaks:
        by_track[track_id] = max(peak, by_track.get(track_id, -999.0))
    unsafe = {track_id: peak for track_id, peak in by_track.items() if peak > -0.999}
    require(by_track, "B1 peak verification returned no effective_static_peak_dbfs evidence")
    require(not unsafe, f"B1 left unsafe effective static peaks: {unsafe}")
    return {"evaluated_track_count": len(by_track), "max_peak_dbfs": max(by_track.values()), "unsafe": unsafe}


def copy_scenario(source_dir: Path, target_dir: Path) -> Path:
    if target_dir.exists():
        shutil.rmtree(target_dir)
    shutil.copytree(source_dir, target_dir)
    candidates = sorted(target_dir.glob("*.vit"))
    require(len(candidates) == 1, f"scenario copy has {len(candidates)} .vit files: {target_dir}")
    return candidates[0]


def run_scenario(base_url: str, source_dir: Path, scenario_root: Path, name: str, sequence: list[str], timeout: float) -> dict[str, Any]:
    scenario_dir = scenario_root / name
    project_dir = scenario_dir / "project"
    project_path = copy_scenario(source_dir, project_dir)
    opened = invoke(base_url, "project.open", {"file_path": str(project_path)}, timeout)
    write_json(scenario_dir / "open_response.json", opened)
    assert_project_history_open_clean(opened, project_path, name + " open")
    initial = project_signature(base_url, timeout)
    require(len(initial) >= 50, f"{name} opened only {len(initial)} tracks")
    turns = []
    folder_exports: list[dict[str, Any]] = []
    for index, capability in enumerate(sequence):
        before_capability = project_signature(base_url, timeout)
        conversation_id = f"nonlinear_a5_{name}_{capability.lower()}_{int(time.time() * 1000)}"
        turn = drive_capability(base_url, capability, conversation_id, scenario_dir / "turns", timeout)
        after_capability = project_signature(base_url, timeout)
        turn["project_state_changed"] = after_capability != before_capability
        turns.append(turn)
        if capability == "B1":
            turn["peak_safety"] = assert_b1_peak_safe(base_url, timeout, scenario_dir / "turns")
        if name == "folder_save_b1_b2":
            export_dir = scenario_dir / "folder_exports" / capability.lower()
            exported = invoke(base_url, "project.save_as_folder", {
                "directory_path": str(export_dir),
                "project_file_name": f"{capability.lower()}_snapshot.vit",
                "media_policy": "reference_only",
            }, timeout)
            write_json(scenario_dir / f"folder_export_{capability.lower()}.json", exported)
            export_result = result_map(exported)
            require(str(export_result.get("status") or "").lower() == "ok", f"{capability} folder export failed: {export_result}")
            require(export_result.get("active_project_unchanged") is True, f"{capability} folder export switched the active source project")
            exported_path = Path(str(export_result.get("project_path") or ""))
            require(exported_path.is_file(), f"{capability} folder export omitted the project file: {exported_path}")
            folder_exports.append({"capability": capability, "project_path": str(exported_path), "response": export_result})
    if name != "folder_save_b1_b2":
        saved = invoke(base_url, "project.save", {}, timeout)
        write_json(scenario_dir / "save_response.json", saved)
    final_signature = project_signature(base_url, timeout)
    reopen_verification: dict[str, Any] = {}
    if name in {"cumulative", "folder_save_b1_b2"}:
        reopen_path = project_path
        if name == "folder_save_b1_b2":
            require(len(folder_exports) == 2, f"folder-save scenario produced {len(folder_exports)} snapshots")
            reopen_path = Path(folder_exports[-1]["project_path"])
        invoke(base_url, "project.new", {}, timeout)
        reopened = invoke(base_url, "project.open", {"file_path": str(reopen_path), "project_path": str(reopen_path)}, timeout)
        write_json(scenario_dir / "reopen_response.json", reopened)
        assert_project_history_open_clean(reopened, reopen_path, name + " reopen")
        reopened_signature = project_signature(base_url, timeout)
        require(reopened_signature == final_signature, f"{name} project state changed across save/reopen")
        # The default state endpoint is intentionally compact for frequent GUI
        # polling and omits conversation_messages. Reopen verification needs
        # the explicit full durable-history projection.
        agent_state = request_json("GET", base_url.rstrip("/") + "/agent/state?detail=full", None, timeout)
        write_json(scenario_dir / "reopened_agent_state.json", agent_state)
        project_history = agent_state.get("project_history") if isinstance(agent_state.get("project_history"), dict) else {}
        history_messages = project_history.get("conversation_messages") if isinstance(project_history.get("conversation_messages"), list) else []
        history_contents = [str(row.get("content") or "") for row in history_messages if isinstance(row, dict)]
        missing_history: list[str] = []
        for turn in turns:
            expected = [CAPABILITIES[turn["capability"]]["message"], str(turn["terminal"].get("reply") or "")]
            for content in expected:
                if content and content not in history_contents:
                    missing_history.append(turn["capability"] + ":" + content[:120])
        require(not missing_history, f"{name} Agent conversation history was not restored: {missing_history}")
        reopen_verification = {"state_equal": True, "history_message_count": len(history_messages), "verified_turn_count": len(turns)}
    return {
        "name": name,
        "sequence": sequence,
        "project_path": str(project_path),
        "initial_track_count": len(initial),
        "final_track_count": len(final_signature),
        "turns": turns,
        "folder_exports": folder_exports,
        "reopen_verification": reopen_verification,
    }


def completed_checks(scenarios: list[tuple[str, list[str]]]) -> list[str]:
    selected = {name for name, _ in scenarios}
    checks: list[str] = []
    if {"direct_b1", "direct_b2", "direct_b3", "direct_b4", "direct_c1"}.issubset(selected):
        checks.append("direct_entry_B1_B2_B3_B4_C1")
    if "shuffle_b3_b2" in selected:
        checks.append("shuffled_B3_then_B2")
    if "shuffle_c1_b4" in selected:
        checks.append("shuffled_C1_then_B4")
    if "cumulative" in selected:
        checks.extend([
            "same_project_cumulative_non_linear_sequence",
            "same_project_save_reopen_state_and_agent_history",
        ])
    if "folder_save_b1_b2" in selected:
        checks.extend([
            "repeated_folder_export_preserves_active_source_session",
            "B1_B2_conversation_history_survives_second_folder_export",
            "self_contained_v2_folder_snapshot_opens_without_parent_path",
        ])
    checks.append("copied_project_history_path_rebound_without_warning")
    if any("B1" in sequence for _, sequence in scenarios):
        checks.append("B1_effective_static_peak_at_or_below_minus_1_dbfs")
    checks.append("source_A5_hash_unchanged")
    return checks


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--source-project", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--timeout-sec", type=float, default=900.0)
    parser.add_argument("--scenarios", default="", help="Optional comma-separated scenario names for focused diagnostics")
    args = parser.parse_args()

    source_project = Path(args.source_project).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(source_project.is_file(), f"source project not found: {source_project}")
    source_dir = source_project.parent
    source_hash_before = sha256_file(source_project)
    health = wait_agent(args.agent_http, min(args.timeout_sec, 120.0))

    scenarios: list[tuple[str, list[str]]] = [
        ("direct_b1", ["B1"]),
        ("direct_b2", ["B2"]),
        ("direct_b3", ["B3"]),
        ("direct_b4", ["B4"]),
        ("direct_c1", ["C1"]),
        ("shuffle_b3_b2", ["B3", "B2"]),
        ("shuffle_c1_b4", ["C1", "B4"]),
        ("cumulative", ["B1", "B3", "C1", "B2", "B4"]),
    ]
    if args.scenarios.strip():
        scenarios.append(("folder_save_b1_b2", ["B1", "B2"]))
        requested = {name.strip() for name in args.scenarios.split(",") if name.strip()}
        known = {name for name, _ in scenarios}
        require(requested.issubset(known), f"unknown scenarios: {sorted(requested - known)}")
        scenarios = [(name, sequence) for name, sequence in scenarios if name in requested]
        require(bool(scenarios), "no scenarios selected")
    summary: dict[str, Any] = {
        "schema_version": "nonlinear_mix_workflow_a5_smoke.v1",
        "status": "running",
        "source_project": str(source_project),
        "source_sha256_before": source_hash_before,
        "health": health,
        "scenarios": [],
    }
    write_json(artifact_dir / "summary.json", summary)
    try:
        for name, sequence in scenarios:
            result = run_scenario(args.agent_http, source_dir, artifact_dir / "scenarios", name, sequence, args.timeout_sec)
            summary["scenarios"].append(result)
            write_json(artifact_dir / "summary.json", summary)
    finally:
        source_hash_after = sha256_file(source_project)
        summary["source_sha256_after"] = source_hash_after
        write_json(artifact_dir / "summary.json", summary)
    require(source_hash_after == source_hash_before, "source A5 project hash changed during isolated smoke")
    summary.update({
        "status": "passed",
        "source_sha256_after": source_hash_after,
        "scenario_count": len(scenarios),
        "capability_turn_count": sum(len(sequence) for _, sequence in scenarios),
        "checked": completed_checks(scenarios),
    })
    write_json(artifact_dir / "summary.json", summary)
    print(json.dumps({"status": "passed", "artifact_dir": str(artifact_dir), "scenario_count": len(scenarios)}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, OSError, urllib.error.URLError, json.JSONDecodeError) as exc:
        print(f"nonlinear_mix_workflow_a5_smoke failed: {exc}")
        raise SystemExit(1)
