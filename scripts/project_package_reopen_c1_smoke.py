"""Two-phase real product-path smoke for portable project persistence.

Phase create imports the real multitrack fixture, waits for the existing L3
offline queue, and saves a portable project folder. Phase reopen runs in a new
Godot/Kernel/Agent lifecycle after global observation artifacts were removed;
it opens the saved project, proves package restoration, and runs C1 without a
measurement repair or project mutation.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import time
from pathlib import Path
from typing import Any

from c1_frequency_cleanup_agent_smoke import (
    C1_CAPABILITY_ID,
    C1_MESSAGE,
    SUCCESS_STAGES,
    execution_tools,
    invoke_tool,
    request_json,
    require,
    result_map,
    track_signature,
    wait_agent,
)


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().lower()


def verify_vit_container(project_path: Path) -> None:
    require(project_path.is_file(), f"project file missing: {project_path}")
    with project_path.open("rb") as handle:
        header = handle.read(4)
    require(header == b"VIT1", f".vit project is not an encrypted VIT1 container: {project_path} header={header!r}")


def verify_manifest(package_dir: Path) -> dict[str, Any]:
    manifest_path = package_dir / "manifest.json"
    require(manifest_path.is_file(), f"project package manifest missing: {manifest_path}")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    require(manifest.get("schema_version") == "vit_project_package.v1", f"unexpected manifest: {manifest}")
    files = manifest.get("files") if isinstance(manifest.get("files"), list) else []
    require(files, "project package manifest contains no checksummed files")
    for entry in files:
        require(isinstance(entry, dict), f"invalid file entry: {entry!r}")
        relative = str(entry.get("path") or "")
        candidate = package_dir / Path(relative)
        require(candidate.is_file(), f"checksummed package file missing: {relative}")
        require(candidate.stat().st_size == int(entry.get("size") or -1), f"size mismatch: {relative}")
        require(sha256_file(candidate) == str(entry.get("sha256") or "").lower(), f"checksum mismatch: {relative}")
    return manifest


def ready_project_l3(acoustic_path: Path, project_uuid: str) -> list[dict[str, Any]]:
    if not acoustic_path.is_file():
        return []
    payload = json.loads(acoustic_path.read_text(encoding="utf-8"))
    packages = payload.get("packages") if isinstance(payload, dict) else []
    out: list[dict[str, Any]] = []
    for row in packages if isinstance(packages, list) else []:
        if not isinstance(row, dict) or str(row.get("project_id") or "") != project_uuid:
            continue
        layers = row.get("package_layers") if isinstance(row.get("package_layers"), dict) else {}
        l3 = layers.get("l3_deep") if isinstance(layers.get("l3_deep"), dict) else {}
        features = l3.get("features") if isinstance(l3.get("features"), dict) else {}
        required = ("band_energy_summary", "stereo_relation_summary", "loudness_summary")
        if all(
            isinstance(features.get(name), dict)
            and str(features[name].get("status") or "").lower() in {"ready", "partial", "suspect"}
            for name in required
        ):
            out.append(row)
    return out


def ready_project_l3_log(log_path: Path, project_uuid: str) -> set[tuple[str, str, str]]:
    grouped: dict[tuple[str, str, str], set[str]] = {}
    if not log_path.is_file():
        return set()
    for line in log_path.read_text(encoding="utf-8", errors="strict").splitlines():
        if not line.strip():
            continue
        try:
            row = json.loads(line)
        except json.JSONDecodeError:
            # The analyzer appends and flushes each record. A polling read can
            # briefly observe the final record before its write is complete.
            continue
        if not isinstance(row, dict) or str(row.get("project_id") or row.get("project_uuid") or "") != project_uuid:
            continue
        status = str(row.get("status") or row.get("quality_status") or "").lower()
        if status not in {"ready", "partial", "suspect"}:
            continue
        track_id = str(row.get("track_id") or row.get("source_track_id") or "")
        clip_id = str(row.get("clip_id") or "")
        source_revision = str(row.get("source_revision") or row.get("source_fingerprint") or "")
        feature_type = str(row.get("feature_type") or "")
        if track_id and clip_id and source_revision:
            grouped.setdefault((track_id, clip_id, source_revision), set()).add(feature_type)
    required = {"band_energy_summary", "stereo_relation_summary", "loudness_summary"}
    return {identity for identity, features in grouped.items() if required.issubset(features)}


def wait_project_l3_log(log_path: Path, project_uuid: str, expected_count: int, timeout: float) -> dict[str, Any]:
    deadline = time.monotonic() + timeout
    ready: set[tuple[str, str, str]] = set()
    while time.monotonic() < deadline:
        ready = ready_project_l3_log(log_path, project_uuid)
        if len(ready) >= expected_count:
            return {"status": "ready", "ready_count": len(ready), "total_count": expected_count, "log_path": str(log_path)}
        time.sleep(1.0)
    return {"status": "timeout", "ready_count": len(ready), "total_count": expected_count, "log_path": str(log_path)}


def create_phase(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(args.repo_root).resolve()
    stems = Path(args.stems_folder).resolve()
    project_path = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    wav_files = sorted(stems.glob("*.wav"))
    require(len(wav_files) >= 2, f"expected multitrack WAV folder: {stems}")
    require(project_path.parent.name == project_path.stem, "portable project path must be <folder>/<folder>.vit")
    project_path.parent.mkdir(parents=True, exist_ok=True)

    summary: dict[str, Any] = {
        "schema_version": "vit_project_package_reopen_c1_smoke.v1",
        "phase": "create",
        "status": "running",
        "project_path": str(project_path),
        "wav_count": len(wav_files),
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    invoke_tool(args.agent_http, "project.new", {}, args.timeout_sec, confirmed=True)
    saved_as = invoke_tool(
        args.agent_http,
        "project.save_as",
        {"file_path": str(project_path), "project_folder_package": True},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "save_as_response.json", saved_as)
    imported = invoke_tool(
        args.agent_http,
        "project.import_folder_as_stems",
        {
            "folder_path": str(stems),
            "recursive": False,
            "target_policy": "create_tracks",
            "start_time_seconds": 0.0,
            "skip_unreadable": False,
            "command_timeout_ms": int(args.timeout_sec * 1000),
        },
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "import_response.json", imported)
    imported_result = result_map(imported)
    refs = imported_result.get("imported_track_refs") if isinstance(imported_result.get("imported_track_refs"), list) else []
    refs = [row for row in refs if isinstance(row, dict)]
    require(len(refs) == len(wav_files), f"import refs {len(refs)} != wav files {len(wav_files)}")

    state = result_map(invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec))
    project_uuid = str(state.get("project_uuid") or state.get("project_id") or "")
    require(project_uuid, f"imported project omitted UUID: {state}")

    l3_log_path = project_path.parent / ".vit_derived" / project_uuid / "l3_feature_log.jsonl"
    l3_started = time.monotonic()
    l3 = wait_project_l3_log(l3_log_path, project_uuid, len(wav_files), args.timeout_sec)
    l3["elapsed_ms"] = int((time.monotonic() - l3_started) * 1000)
    require(l3.get("status") == "ready", f"L3 preparation failed: {l3}")
    summary["l3_preparation"] = l3

    saved = invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "save_response.json", saved)
    verify_vit_container(project_path)
    state_response = invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec)
    state = result_map(state_response)
    saved_project_uuid = str(state.get("project_uuid") or state.get("project_id") or "")
    require(saved_project_uuid == project_uuid, f"saved project UUID changed: before={project_uuid} after={saved_project_uuid}")
    package_dir = project_path.parent / ".vit_project"
    manifest = verify_manifest(package_dir)
    require(str(manifest.get("project_uuid") or "") == project_uuid, "package/project UUID mismatch")
    require(int(manifest.get("acoustic_package_count") or 0) == len(wav_files), f"package L3 count mismatch: {manifest}")
    required_files = {
        "agent_runtime_state.json",
        "conversation_graph.json",
        "mixboard.json",
        "observation_index.json",
        "acoustic_packages.json",
        "decision_ledger.json",
        "media_manifest.json",
    }
    require(required_files.issubset({path.name for path in package_dir.iterdir()}), "portable package contract incomplete")
    summary.update(
        {
            "status": "passed",
            "project_uuid": project_uuid,
            "package_dir": str(package_dir),
            "manifest": manifest,
            "project_sha256": sha256_file(project_path),
        }
    )
    return summary


def reopen_phase(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(args.repo_root).resolve()
    project_path = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    verify_vit_container(project_path)
    package_dir = project_path.parent / ".vit_project"
    manifest = verify_manifest(package_dir)
    expected_count = int(manifest.get("acoustic_package_count") or 0)
    require(expected_count > 0, "portable package contains no acoustic packages")

    summary: dict[str, Any] = {
        "schema_version": "vit_project_package_reopen_c1_smoke.v1",
        "phase": "reopen",
        "status": "running",
        "project_path": str(project_path),
        "expected_l3_count": expected_count,
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    opened = invoke_tool(args.agent_http, "project.open", {"file_path": str(project_path)}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "open_response.json", opened)
    opened_result = result_map(opened)
    workspace = opened_result.get("project_workspace") if isinstance(opened_result.get("project_workspace"), dict) else {}
    restore = workspace.get("project_package_restore") if isinstance(workspace.get("project_package_restore"), dict) else {}
    require(str(restore.get("status") or "") == "ok", f"project package was not restored: {restore}")
    require(bool(restore.get("manifest_verified")), f"project package was not verified: {restore}")
    require(int(restore.get("acoustic_packages_restored") or 0) == expected_count, f"restored L3 mismatch: {restore}")
    require(str(workspace.get("project_package_migration") or "") != "scheduled", "reopen unexpectedly scheduled analysis migration")

    before_response = invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec)
    before = result_map(before_response)
    project_uuid = str(before.get("project_uuid") or before.get("project_id") or "")
    require(project_uuid == str(manifest.get("project_uuid") or ""), "reopened project UUID differs from manifest")
    before_signature = track_signature(before)
    require(len(before_signature) >= expected_count, f"reopened project track count too small: {len(before_signature)}")

    acoustic_path = repo_root / "VitApp" / "Workspace" / "Artifacts" / "acoustic_package_status.json"
    ready_l3 = ready_project_l3(acoustic_path, project_uuid)
    require(len(ready_l3) == expected_count, f"restored ready L3 {len(ready_l3)} != {expected_count}")
    acoustic_before = sha256_file(acoustic_path)

    conversation_id = "project_package_reopen_c1_" + time.strftime("%Y%m%d_%H%M%S")
    chat_started = time.monotonic()
    chat = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": C1_MESSAGE,
            "context": {
                "agent_mode": "chat",
                "capability_id": C1_CAPABILITY_ID,
                "interaction_mode": "propose",
                "c1_execute": True,
                "smoke_contract": "project_package_reopen_existing_evidence.v1",
            },
        },
        args.timeout_sec,
    )
    write_json(artifact_dir / "chat_c1.json", chat)
    after_response = invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec)
    after = result_map(after_response)
    after_signature = track_signature(after)

    workflow = chat.get("workflow_data") if isinstance(chat.get("workflow_data"), dict) else {}
    timing = workflow.get("timing") if isinstance(workflow.get("timing"), dict) else {}
    stage = str(workflow.get("canary_stage") or "")
    tools = execution_tools(chat)
    forbidden = [name for name in tools if name in {"mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation"}]
    require(str(workflow.get("capability_id") or "") == C1_CAPABILITY_ID, f"C1 route mismatch: {workflow}")
    require(stage != "readiness_blocked", f"C1 readiness blocked after restore: {chat.get('reply')}")
    require(stage in SUCCESS_STAGES or bool(chat.get("proposal")), f"unexpected C1 stage: {stage}")
    require(not forbidden, f"C1 requested observation after restore: {forbidden}")
    require(int(timing.get("diagnosis_fill_requested_track_count") or 0) == 0, f"C1 requested diagnosis fill: {timing}")
    require(int(timing.get("diagnosis_fill_render_count") or 0) == 0, f"C1 rendered diagnosis fill: {timing}")
    require(before_signature == after_signature, "C1 mutated the reopened project")
    require(acoustic_before == sha256_file(acoustic_path), "C1 rewrote acoustic packages after restore")

    summary.update(
        {
            "status": "passed",
            "project_uuid": project_uuid,
            "project_package_restore": restore,
            "ready_l3_count": len(ready_l3),
            "conversation_id": conversation_id,
            "chat_elapsed_ms": int((time.monotonic() - chat_started) * 1000),
            "canary_stage": stage,
            "reply": str(chat.get("reply") or ""),
            "tool_route": tools,
            "forbidden_observation_tools": forbidden,
            "timing": timing,
            "project_mutation_detected": False,
            "acoustic_package_mutation_detected": False,
        }
    )
    return summary


def recover_phase(args: argparse.Namespace) -> dict[str, Any]:
    repo_root = Path(args.repo_root).resolve()
    source_path = Path(args.source_project_path).resolve()
    project_path = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(source_path.is_file(), f"history source snapshot missing: {source_path}")
    require(project_path.parent.name == project_path.stem, "recovery target must be <folder>/<folder>.vit")
    project_path.parent.mkdir(parents=True, exist_ok=True)

    summary: dict[str, Any] = {
        "schema_version": "vit_history_project_recovery_smoke.v1",
        "phase": "recover",
        "status": "running",
        "source_project_path": str(source_path),
        "project_path": str(project_path),
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    opened = invoke_tool(args.agent_http, "project.open", {"file_path": str(source_path)}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "source_open_response.json", opened)
    source_state = result_map(invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec))
    source_uuid = str(source_state.get("project_uuid") or source_state.get("project_id") or "")
    require(source_uuid, "recovery source project omitted UUID")

    saved_as = invoke_tool(
        args.agent_http,
        "project.save_as",
        {"file_path": str(project_path), "project_folder_package": True},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "recovery_save_as_response.json", saved_as)
    target_state = result_map(invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec))
    target_uuid = str(target_state.get("project_uuid") or target_state.get("project_id") or "")
    require(target_uuid and target_uuid != source_uuid, f"recovery did not create a new UUID: source={source_uuid} target={target_uuid}")
    tracks = track_signature(target_state)
    expected_count = sum(1 for row in tracks if row.get("clips"))
    require(expected_count > 0, "recovered project has no analyzable audio tracks")

    analysis = invoke_tool(
        args.agent_http,
        "project.audio_analysis_status",
        {"latest": True, "ensure_ready": True, "timeout_ms": int(args.timeout_sec * 1000), "poll_interval_ms": 250},
        args.timeout_sec,
    )
    write_json(artifact_dir / "analysis_status.json", analysis)

    deadline = time.monotonic() + args.timeout_sec
    l3_log_path = project_path.parent / ".vit_derived" / target_uuid / "l3_feature_log.jsonl"
    ready: set[tuple[str, str, str]] = set()
    while time.monotonic() < deadline:
        ready = ready_project_l3_log(l3_log_path, target_uuid)
        if len(ready) >= expected_count:
            break
        time.sleep(1.0)
    require(len(ready) == expected_count, f"recovered L3 {len(ready)} != expected {expected_count}")

    saved = invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "recovery_save_response.json", saved)
    package_dir = project_path.parent / ".vit_project"
    manifest = verify_manifest(package_dir)
    require(str(manifest.get("project_uuid") or "") == target_uuid, "recovery package UUID mismatch")
    require(int(manifest.get("acoustic_package_count") or 0) == expected_count, f"recovery package acoustic count mismatch: {manifest}")
    conversation_text = (package_dir / "conversation_graph.json").read_text(encoding="utf-8", errors="replace")
    if args.required_history_text:
        require(args.required_history_text in conversation_text, f"required history marker missing: {args.required_history_text}")

    summary.update(
        {
            "status": "passed",
            "source_project_uuid": source_uuid,
            "project_uuid": target_uuid,
            "track_count": len(tracks),
            "audio_track_count": expected_count,
            "ready_l3_count": len(ready),
            "package_dir": str(package_dir),
            "manifest": manifest,
            "project_sha256": sha256_file(project_path),
            "required_history_text": args.required_history_text,
            "required_history_text_found": not args.required_history_text or args.required_history_text in conversation_text,
        }
    )
    return summary


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--phase", choices=("create", "reopen", "recover"), required=True)
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--stems-folder", default="")
    parser.add_argument("--source-project-path", default="")
    parser.add_argument("--required-history-text", default="")
    parser.add_argument("--timeout-sec", type=float, default=600.0)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    try:
        if args.phase == "create":
            summary = create_phase(args)
        elif args.phase == "reopen":
            summary = reopen_phase(args)
        else:
            summary = recover_phase(args)
    except Exception as exc:  # noqa: BLE001 - smoke must persist full failure evidence.
        summary = {
            "schema_version": "vit_project_package_reopen_c1_smoke.v1",
            "phase": args.phase,
            "status": "failed",
            "error": f"{type(exc).__name__}: {exc}",
            "project_path": str(Path(args.project_path).resolve()),
        }
        write_json(artifact_dir / "summary.json", summary)
        raise
    write_json(artifact_dir / "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
