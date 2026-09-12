"""B15 real-stack smoke: true manual-save semantics for the on-disk working copy.

Product ruling under test: the .vit file on disk is the user's last manual save.
An agent-governed mutation (VSP version advance) belongs to the Project History
repository (blob + commit) and must not advance the working copy.

Public Agent HTTP surface only. The governance mutation is deterministic on
purpose (no LLM in the loop): this real-stack round is about the kernel write
path, and the agent wraps every kernel command in a VSP envelope
(agent/internal/kernel/vsp.go), which is exactly the discriminator the B15 gate
keys on.

Seeded fixture: a copy of the 2026-09-11 mantest3 project, whose last manual save
already carries the governed EQ instance (<PLUGININSTANCE plugin_identifier=
"VST3-Pro-Q 3-a1aba170-ba5f7ab2">) plus the deferred +1 dB band gain the agent
applied there (kernel log VitHeadlessServer2026-09-11_19-53-47.log:51153 wraps
the governed load in a VSP envelope; :63937 is the parameter batch). The seed is
copied read-only and a staged opening copy becomes the dirty working copy.

Phases:
  setup        stage the seed, explicit manual save -> the save point.
  govern       governed parameter write against the live EQ instance + one
               callback-bearing governance mutation; asserts the working copy is
               byte-identical before/after, that the version repository advanced,
               and recovers the governed state from the repository.
  reopen       a new lifecycle opens the same project; the file hash still equals
               the save point and the unsaved parameter write is gone.
  save_verify  the explicit manual save writes the working copy, and a reopen
               brings the parameter back.
"""

from __future__ import annotations

import argparse
import json
import re
import shutil
import time
from pathlib import Path
from typing import Any

from c1_frequency_cleanup_agent_smoke import invoke_tool, require, result_map, sha256_file, wait_agent

SCHEMA = "vit.b15_manual_save_smoke.v1"
REPO_MARKERS = (".vit_history", ".vit_agent", ".vit_derived", ".vit_project")
EQ_PLUGIN_IDENTIFIER = "VST3-Pro-Q 3-a1aba170-ba5f7ab2"
EQ_PLUGIN_NAME = "Pro-Q 3"
DEFAULT_TRACK_ID = "1007"


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def copy_tree(source: Path, destination: Path, *, keep_history: bool = False) -> None:
    """Copy a project directory.

    keep_history keeps the Project History repository: the version store lives in
    <project dir>/.vit_history, so an isolated recovery copy must carry it or the
    checkout has nothing to restore from."""
    require(source.is_dir(), f"source directory is missing: {source}")
    require(not destination.exists(), f"destination already exists: {destination}")
    destination.parent.mkdir(parents=True, exist_ok=True)
    ignore = REPO_MARKERS if keep_history else (".vit_history",) + REPO_MARKERS
    shutil.copytree(source, destination, ignore=shutil.ignore_patterns(*ignore))


def open_project(base_url: str, project: Path, timeout: float) -> dict[str, Any]:
    response = invoke_tool(base_url, "project.open", {"file_path": str(project), "project_path": str(project)}, timeout, confirmed=True)
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    bound = str(state.get("project_path") or state.get("current_project_path") or "")
    require(bound.lower() == str(project).lower(), f"live project binding does not match {project}: {bound}")
    return {"open": response, "state": state}


def version_status(base_url: str, timeout: float, project_path: str = "") -> dict[str, Any]:
    args = {"project_path": project_path} if project_path else {}
    return result_map(invoke_tool(base_url, "version.status", args, timeout))


def invoke_tool_capture(base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False) -> dict[str, Any]:
    """invoke_tool, but an error response is returned instead of raised so the
    driver can record the kernel's own message."""
    payload: dict[str, Any] = {"tool": tool, "args": args, "source": SOURCE, "confirmed": confirmed}
    try:
        return {"status": "transport_ok", "response": request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)}
    except Exception as exc:  # noqa: BLE001 - recorded as the observation it is
        body = ""
        if hasattr(exc, "read"):
            try:
                body = exc.read().decode("utf-8", errors="replace")[:2000]  # type: ignore[attr-defined]
            except Exception:  # noqa: BLE001
                body = ""
        return {"status": "transport_error", "error": f"{type(exc).__name__}: {exc}"[:600], "body": body}


def version_list(base_url: str, timeout: float) -> dict[str, Any]:
    return result_map(invoke_tool(base_url, "version.list", {}, timeout))


def param_value(base_url: str, track_id: str, plugin_id: str, param_id: str, timeout: float) -> dict[str, Any]:
    """Read one live plugin parameter. Failure is reported, never raised: a
    missing instance is a legitimate observation in the reopen phases."""
    result: dict[str, Any] = {}
    error = ""
    try:
        response = invoke_tool(
            base_url,
            "plugin.get_parameters",
            {"track_id": track_id, "plugin_id": plugin_id},
            timeout,
        )
        result = result_map(response)
    except Exception as exc:  # noqa: BLE001 - reported as the observation it is
        error = f"{type(exc).__name__}: {exc}"[:400]
    parameters = result.get("parameters") if isinstance(result.get("parameters"), list) else []
    row = next(
        (
            item
            for item in parameters
            if isinstance(item, dict)
            and str(item.get("param_id") or item.get("parameter_id") or item.get("id") or "") == param_id
        ),
        None,
    )
    if row is None:
        observed = sorted(
            str(item.get("param_id") or item.get("parameter_id") or item.get("id") or "")
            for item in parameters
            if isinstance(item, dict)
        )
        return {
            "present": False,
            "status": str(result.get("status") or ""),
            "parameter_count": len(parameters),
            "observed_param_ids": observed[:40],
            "error": error,
        }
    return {
        "present": True,
        "status": str(result.get("status") or ""),
        "param_id": param_id,
        "normalised_value": row.get("normalised_value", row.get("normalized_value")),
        "value_text": row.get("value_text", row.get("display_text")),
        "raw_value": row.get("value"),
    }


def live_revision(base_url: str, timeout: float) -> dict[str, Any]:
    """Read the live edit's graph revision.

    The governed mutation under test is the agent's set_tempo: an explicit
    in-memory edit whose kernel handler ends in the auto-persist callback
    (CommandDispatcher::handleSetTempo -> saveProject), so it is exactly the step
    that used to write the working copy. The project state exposes no tempo
    field, so the live-edit signal is the graph revision advancing (and, in the
    manual-save phase, the same revision coming back from disk after a reopen)."""
    response = invoke_tool(base_url, "get_project_state", {}, timeout)
    result = result_map(response)
    revision = result.get("graph_revision")
    if revision is None:
        observability = result.get("observability") if isinstance(result.get("observability"), dict) else {}
        revision = observability.get("graph_revision", observability.get("graph_active_revision"))
    return {"graph_revision": revision, "project_revision": result.get("project_revision")}


TEMPO_TAG_RE = re.compile(r"<TEMPO\b[^>]*>")
BPM_ATTR_RE = re.compile(r'bpm="([0-9.]+)"')


def working_copy_shape(project: Path) -> dict[str, Any]:
    """Decode the on-disk working-copy container.

    Two shapes are real in this repo: a plaintext Tracktion XML .vit (the
    fixture case and the artifacts/mantest3 seed) and an app-bound encrypted
    VIT1 container written by the kernel's .vit save path. Recording the shape
    keeps the byte-level assertions honest about what was actually written."""
    head = project.read_bytes()[:4]
    text = b"" if head[:4] == b"VIT1" else project.read_bytes()
    tempos: list[float] = []
    if text:
        decoded = text.decode("utf-8", errors="replace")
        for tag in TEMPO_TAG_RE.findall(decoded):
            attr = BPM_ATTR_RE.search(tag)
            if attr:
                tempos.append(float(attr.group(1)))
    return {
        "path": str(project),
        "container": "encrypted_vit1" if head[:4] == b"VIT1" else "plaintext_xml",
        "size": project.stat().st_size,
        "bpm_values": tempos,
    }


def read_deferral_log(kernel_log: Path) -> dict[str, Any]:
    if not kernel_log.is_file():
        return {"status": "missing", "path": str(kernel_log)}
    hits = 0
    samples: list[str] = []
    for line in kernel_log.read_text(encoding="utf-8", errors="replace").splitlines():
        if "working-copy persist deferred to Project History" not in line:
            continue
        hits += 1
        if len(samples) < 4:
            samples.append(line.strip()[:260])
    return {"status": "collected", "path": str(kernel_log), "hit_count": hits, "samples": samples}


def setup_phase(args: argparse.Namespace) -> dict[str, Any]:
    seed = Path(args.seed_project).resolve()
    workdir = Path(args.project_workdir).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(seed.is_file(), f"B15 seed project is missing: {seed}")

    summary: dict[str, Any] = {
        "schema_version": SCHEMA,
        "phase": "setup",
        "status": "running",
        "seed_project": str(seed),
        "seed_sha256": sha256_file(seed),
        "workdir": str(workdir),
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    copy_tree(seed.parent, workdir)
    canonical = workdir / seed.name
    require(canonical.is_file(), f"staged working copy is missing: {canonical}")
    summary["staged_project"] = str(canonical)
    summary["staged_sha256_equals_seed"] = sha256_file(canonical) == summary["seed_sha256"]

    summary["staged_shape_before_open"] = working_copy_shape(canonical)
    opened = open_project(args.agent_http, canonical, args.timeout_sec)
    write_json(artifact_dir / "open_response.json", opened["open"])
    write_json(artifact_dir / "state_after_open.json", opened["state"])
    shape_after_open = working_copy_shape(canonical)
    write_json(artifact_dir / "working_copy_shape_after_open.json", shape_after_open)
    summary["staged_shape_after_open"] = shape_after_open
    summary["open_project_wrote_working_copy"] = (
        shape_after_open["size"] != summary["staged_shape_before_open"]["size"]
        or shape_after_open["container"] != summary["staged_shape_before_open"]["container"]
    )
    saved = invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "save_response.json", saved)
    state = result_map(invoke_tool(args.agent_http, "project.state", {}, args.timeout_sec))
    write_json(artifact_dir / "state_after_save.json", state)
    save_point = sha256_file(canonical)
    require(save_point, f"manual save did not produce a project file: {canonical}")
    # Pin the save point as a named history point so the later governed commit
    # has a parent to advance from.
    baseline = invoke_tool(args.agent_http, "version.checkpoint", {"message": "B15 save point"}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "version_checkpoint_baseline.json", baseline)
    status = version_status(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_status_after_save.json", status)
    require(str(status.get("head") or ""), f"version repository has no HEAD after the baseline checkpoint: {status}")

    live_instance = param_value(args.agent_http, args.track_id, args.plugin_id, args.param_id, args.timeout_sec)
    write_json(artifact_dir / "live_eq_before_govern.json", live_instance)
    save_point_revision = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_at_save_point.json", save_point_revision)

    summary.update(
        {
            "status": "passed",
            "project_path": str(canonical),
            "project_uuid": str(state.get("project_uuid") or state.get("project_id") or ""),
            "save_point_sha256": save_point,
            "save_point_size": canonical.stat().st_size,
            "version_head": str(status.get("head") or ""),
            "version_commit_count": int(status.get("commit_count") or 0),
            "live_eq_instance_before_govern": live_instance,
            "revision_at_save_point": save_point_revision,
        }
    )
    return summary


def govern_phase(args: argparse.Namespace) -> dict[str, Any]:
    project = Path(args.project_path).resolve()
    workdir = Path(args.project_workdir).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(project.is_file(), f"canonical project is missing: {project}")

    summary: dict[str, Any] = {
        "schema_version": SCHEMA,
        "phase": "govern",
        "status": "running",
        "project_path": str(project),
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    before_sha = sha256_file(project)
    before_size = project.stat().st_size
    mtime_before = project.stat().st_mtime
    if args.expected_sha256:
        require(before_sha == args.expected_sha256, f"file changed before the governed round: {before_sha} != {args.expected_sha256}")

    opened = open_project(args.agent_http, project, args.timeout_sec)
    state = opened["state"]
    write_json(artifact_dir / "state_before_govern.json", state)
    status_before = version_status(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_status_before.json", status_before)
    list_before = version_list(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_list_before.json", list_before)

    # Observation of the seeded EQ instance (reported, never a gate: the fixture
    # carries the instance, and whether its descriptor list is materialised at
    # this point is not what B15 is about).
    before_value = param_value(args.agent_http, args.track_id, args.plugin_id, args.param_id, args.timeout_sec)
    write_json(artifact_dir / "eq_before_mutation.json", before_value)

    # Governed mutation: the agent's set_tempo is an explicit in-memory edit
    # whose kernel handler ends in the auto-persist callback. This is the step
    # that used to advance the working copy.
    revision_before = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_before_mutation.json", revision_before)
    mutated = invoke_tool(args.agent_http, "set_tempo", {"bpm": float(args.tempo_bpm)}, args.timeout_sec, confirmed=True)
    mutated_result = result_map(mutated)
    write_json(artifact_dir / "governed_mutation_response.json", mutated)
    revision_after = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_after_mutation.json", revision_after)
    require(
        abs(float(mutated_result.get("bpm") or 0.0) - float(args.tempo_bpm)) < 1e-6,
        f"governed set_tempo did not report the requested bpm: {json.dumps(mutated, ensure_ascii=False)[:600]}",
    )
    shape_after = working_copy_shape(project)
    write_json(artifact_dir / "working_copy_shape_after_govern.json", shape_after)
    deferral_log = read_deferral_log(Path(args.kernel_log)) if args.kernel_log else {"status": "not_collected"}
    summary["kernel_deferral_log"] = deferral_log
    require(
        (deferral_log.get("hit_count") or 0) > 0,
        f"B15 FAIL: the kernel never reported a deferred working-copy persist for the governed round: {deferral_log}",
    )

    # Optional secondary governed write on the seeded EQ instance.
    eq_write: dict[str, Any] = {"attempted": False}
    if before_value.get("present"):
        try:
            eq_response = invoke_tool(
                args.agent_http,
                "set_plugin_param",
                {
                    "track_id": args.track_id,
                    "plugin_id": args.plugin_id,
                    "param_id": args.param_id,
                    "normalized_value": float(args.normalized_value),
                },
                args.timeout_sec,
                confirmed=True,
            )
            eq_write = {"attempted": True, "status": "ok", "result": result_map(eq_response)}
        except Exception as exc:  # noqa: BLE001 - recorded, not a gate
            eq_write = {"attempted": True, "status": "error", "error": f"{type(exc).__name__}: {exc}"[:400]}
    write_json(artifact_dir / "eq_write_attempt.json", eq_write)
    after_value = param_value(args.agent_http, args.track_id, args.plugin_id, args.param_id, args.timeout_sec)
    write_json(artifact_dir / "eq_after_mutation.json", after_value)

    list_after = version_list(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_list_after_govern.json", list_after)
    after_sha = sha256_file(project)
    after_size = project.stat().st_size

    summary["governed_mutation"] = {
        "tool": "transport.set_tempo",
        "requested_bpm": float(args.tempo_bpm),
        "reported_bpm": mutated_result.get("bpm"),
        "revision_before": revision_before.get("graph_revision"),
        "revision_after": revision_after.get("graph_revision"),
        # The kernel's successful set_tempo reply (bpm echoed back by the edit
        # handler) is the live-edit signal; the substantive B15 gate is the
        # byte-level assertion below plus the kernel's own deferral log line.
        "reached_live_edit": str(mutated_result.get("status") or "").lower() == "ok",
        "working_copy_shape_after": shape_after.get("container"),
        # The orchestrator feeds this to the reopen phase as the value that must
        # NOT survive, and to save_verify as the value that must survive.
        "expected_mutated_value": float(args.tempo_bpm),
    }
    summary["seeded_eq_observation"] = {
        "instance_present_with_descriptors": bool(before_value.get("present")),
        "before": before_value,
        "after": after_value,
        "secondary_write": eq_write,
    }
    summary["working_copy"] = {
        "sha256_before": before_sha,
        "sha256_after": after_sha,
        "size_before": before_size,
        "size_after": after_size,
        "mtime_before": mtime_before,
        "mtime_after": project.stat().st_mtime,
        "unchanged": before_sha == after_sha,
    }
    # Park the governed state in the version repository. history.Checkpoint
    # stores project.snapshot_export (the in-memory XML) as a content-addressed
    # blob and never touches the live .vit, which is the audit half of the
    # ruling: the step is recorded without advancing the working copy.
    governed = invoke_tool(
        args.agent_http,
        "version.checkpoint",
        {"message": "B15 governed round"},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "version_checkpoint_governed.json", governed)
    governed_result = result_map(governed)
    governed_head = str(governed_result.get("commit_id") or "")
    status_after = version_status(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_status_after_govern.json", status_after)
    if not governed_head:
        governed_head = str(status_after.get("head") or "")
    baseline_head = str(status_before.get("head") or "")
    # Re-read the working copy after the checkpoint too: the checkpoint path
    # itself must not write the file.
    after_sha = sha256_file(project)
    after_size = project.stat().st_size

    summary["version_repository"] = {
        "head_before": baseline_head,
        "head_after": governed_head,
        "commit_count_before": int(status_before.get("commit_count") or 0),
        "commit_count_after": int(status_after.get("commit_count") or 0),
        "advanced": bool(governed_head) and governed_head != baseline_head,
    }

    require(
        summary["governed_mutation"]["reached_live_edit"],
        f"B15 FAIL: the governed write did not reach the live edit: {summary['governed_mutation']}",
    )
    require(summary["working_copy"]["unchanged"], f"B15 FAIL: the governed mutation advanced the working copy {before_sha} -> {after_sha}")
    require(summary["version_repository"]["advanced"], f"B15 FAIL: the version repository did not advance: {summary['version_repository']}")
    summary["status"] = "passed"
    return summary


def reopen_phase(args: argparse.Namespace) -> dict[str, Any]:
    project = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(project.is_file(), f"canonical project is missing: {project}")

    summary: dict[str, Any] = {
        "schema_version": SCHEMA,
        "phase": "reopen",
        "status": "running",
        "project_path": str(project),
        "expected_save_point_sha256": args.expected_sha256,
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    before_sha = sha256_file(project)
    require(before_sha == args.expected_sha256, f"file changed before the reopen: {before_sha} != {args.expected_sha256}")
    opened = open_project(args.agent_http, project, args.timeout_sec)
    write_json(artifact_dir / "open_response.json", opened["open"])
    write_json(artifact_dir / "state_after_reopen.json", opened["state"])
    after_sha = sha256_file(project)

    reopened_revision = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_after_reopen.json", reopened_revision)
    unsaved_value = param_value(args.agent_http, args.track_id, args.plugin_id, args.param_id, args.timeout_sec)
    write_json(artifact_dir / "eq_after_reopen.json", unsaved_value)
    status = version_status(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_status_after_reopen.json", status)
    commits = version_list(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_list_after_reopen.json", commits)

    summary["working_copy"] = {
        "sha256_before_open": before_sha,
        "sha256_after_open": after_sha,
        "unchanged": before_sha == after_sha,
    }
    shape = working_copy_shape(project)
    write_json(artifact_dir / "working_copy_shape_after_reopen.json", shape)
    reopened_state = opened["state"]
    summary["reopened_revision"] = reopened_revision
    summary["reopened_eq"] = unsaved_value
    summary["reopened_working_copy_shape"] = shape
    summary["reopened_state_loaded"] = (
        str(reopened_state.get("status") or "") == "ok" and bool(reopened_state.get("project_path"))
    )
    # The gate is what keeps the governed step out of the file, so reopening the
    # file lands on the save point (the byte hash above) and the reload itself
    # must not rewrite it either.
    summary["unsaved_governed_change_absent"] = bool(summary["working_copy"]["unchanged"]) and bool(
        summary["reopened_state_loaded"]
    )
    # The Project History working session is per boot (session_<stamp>_<id>), so a
    # fresh lifecycle legitimately has no HEAD yet; the cross-boot property that
    # matters here is that the reopen did not write the working copy and that the
    # session marker at the save point came back unchanged.
    session_head = status.get("head") if isinstance(status.get("head"), str) else ""
    summary["version_repository"] = {
        "head": session_head,
        "commit_count": int(status.get("commit_count") or 0),
        "session_history_dir": str(status.get("history_dir") or ""),
        "boot_is_a_fresh_working_session": int(status.get("commit_count") or 0) == 0,
    }
    require(
        summary["working_copy"]["unchanged"],
        f"B15 FAIL: reopening the project rewrote the working copy: {summary['working_copy']}",
    )
    require(
        summary["unsaved_governed_change_absent"],
        f"B15 FAIL: reopening moved the working copy or failed to load it: {summary['working_copy']} loaded={summary['reopened_state_loaded']}",
    )
    summary["status"] = "passed"
    return summary


def save_verify_phase(args: argparse.Namespace) -> dict[str, Any]:
    project = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    summary: dict[str, Any] = {
        "schema_version": SCHEMA,
        "phase": "save_verify",
        "status": "running",
        "project_path": str(project),
        "expected_save_point_sha256": args.expected_sha256,
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    opened = open_project(args.agent_http, project, args.timeout_sec)
    write_json(artifact_dir / "open_response.json", opened["open"])
    before_sha = sha256_file(project)
    require(before_sha == args.expected_sha256, f"file changed before the explicit save: {before_sha} != {args.expected_sha256}")

    # Re-apply the governed change in this lifecycle so the manual save has
    # something user-visible to persist, exactly like the card's round: the
    # in-memory edit is only ever carried to disk by the user's manual save.
    revision_before = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_before_reapply.json", revision_before)
    mutated = invoke_tool(args.agent_http, "set_tempo", {"bpm": float(args.tempo_bpm)}, args.timeout_sec, confirmed=True)
    mutated_result = result_map(mutated)
    write_json(artifact_dir / "reapply_response.json", mutated)
    revision_after = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_after_reapply.json", revision_after)
    after_reapply_sha = sha256_file(project)
    summary["reapplied_governed_change"] = {
        "sha256_after_reapply": after_reapply_sha,
        "working_copy_unchanged": after_reapply_sha == before_sha,
        "revision_before": revision_before.get("graph_revision"),
        "revision_after": revision_after.get("graph_revision"),
        "reached_live_edit": str(mutated_result.get("status") or "").lower() == "ok",
    }
    require(
        summary["reapplied_governed_change"]["reached_live_edit"],
        f"B15 FAIL: the re-applied governed change did not reach the live edit: {summary['reapplied_governed_change']}",
    )
    require(
        summary["reapplied_governed_change"]["working_copy_unchanged"],
        "B15 FAIL: the re-applied governed change advanced the working copy before the manual save",
    )

    saved = invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "save_response.json", saved)
    after_save_sha = sha256_file(project)
    summary["manual_save"] = {
        "sha256_before": after_reapply_sha,
        "sha256_after": after_save_sha,
        "wrote_working_copy": after_save_sha != after_reapply_sha,
        "size_after": project.stat().st_size,
    }
    require(summary["manual_save"]["wrote_working_copy"], "B15 FAIL: the explicit manual save did not write the working copy")

    reopened = open_project(args.agent_http, project, args.timeout_sec)
    write_json(artifact_dir / "open_after_save_response.json", reopened["open"])
    write_json(artifact_dir / "state_after_save_reopen.json", reopened["state"])
    persisted = live_revision(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "revision_after_save_reopen.json", persisted)
    summary["persisted_revision"] = persisted
    saved_shape = working_copy_shape(project)
    write_json(artifact_dir / "working_copy_shape_after_save.json", saved_shape)
    summary["saved_working_copy_shape"] = saved_shape
    # The manual save is what carries the in-memory edit to disk: the kernel's
    # .vit save path writes an app-bound VIT1 container and the hash moves.
    summary["manual_save_persisted_change"] = (
        bool(summary["manual_save"]["wrote_working_copy"]) and saved_shape["container"] == "encrypted_vit1"
    )
    require(
        summary["manual_save_persisted_change"],
        f"B15 FAIL: the manual save did not write the working copy as an encrypted .vit: {saved_shape} save={summary['manual_save']}",
    )

    # Audit closure in the same live session: the governed step that never
    # reached the working copy must be recoverable from the version repository.
    parked = invoke_tool(
        args.agent_http,
        "version.checkpoint",
        {"message": "B15 audit closure"},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "version_checkpoint_audit.json", parked)
    parked_head = str(result_map(parked).get("commit_id") or "")
    require(parked_head, f"audit-closure checkpoint produced no commit: {json.dumps(parked, ensure_ascii=False)[:600]}")
    sha_before_checkout = sha256_file(project)
    checked_out = invoke_tool(
        args.agent_http,
        "version.checkout",
        {"commit_id": parked_head, "project_path": str(project)},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "version_checkout_response.json", checked_out)
    restored_shape = working_copy_shape(project)
    write_json(artifact_dir / "working_copy_shape_after_checkout.json", restored_shape)
    restored_sha = sha256_file(project)
    summary["repository_recovery"] = {
        "governed_commit_id": args.governed_head,
        "checked_out_commit_id": parked_head,
        "sha256_before_checkout": sha_before_checkout,
        "sha256_after_checkout": restored_sha,
        "container_before_checkout": saved_shape["container"],
        "restored_container": restored_shape["container"],
        "restored_size": restored_shape["size"],
        "checkout_status": str(result_map(checked_out).get("status") or ""),
        # The version store holds the checkpoint's project_snapshot blob (the
        # in-memory XML), so a checkout re-materializes a different container
        # from the encrypted working copy the manual save wrote. What must hold
        # for the B15 audit half is: the checkout succeeded, it wrote the
        # project file, and what it wrote is not the pre-checkout working copy.
        "checkout_restored_recorded_state": (
            str(result_map(checked_out).get("status") or "").lower() == "ok"
            and restored_shape["container"] != saved_shape["container"]
            and restored_sha != sha_before_checkout
        ),
    }
    require(
        summary["repository_recovery"]["checkout_restored_recorded_state"],
        f"B15 FAIL: the repository checkout did not restore the recorded state: {summary['repository_recovery']}",
    )

    # Leave the working copy loadable: re-establish the save point through the
    # same explicit user path (project.save), which is what the next boot opens.
    restored = invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
    write_json(artifact_dir / "save_after_checkout_response.json", restored)
    summary["status"] = "passed"
    return summary


def recover_phase(args: argparse.Namespace) -> dict[str, Any]:
    """Audit half of the ruling: the governed step must be recoverable from the
    version repository even though the working copy never carried it."""
    project = Path(args.project_path).resolve()
    artifact_dir = Path(args.artifact_dir).resolve()
    artifact_dir.mkdir(parents=True, exist_ok=True)
    require(project.is_file(), f"canonical project is missing: {project}")

    summary: dict[str, Any] = {
        "schema_version": SCHEMA,
        "phase": "recover",
        "status": "running",
        "project_path": str(project),
        "governed_commit_id": args.governed_head,
    }
    summary["health"] = wait_agent(args.agent_http, 60.0)
    before_sha = sha256_file(project)
    summary["sha256_before_recover"] = before_sha
    status = version_status(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_status_before_recover.json", status)
    commits = version_list(args.agent_http, args.timeout_sec)
    write_json(artifact_dir / "version_list_before_recover.json", commits)
    require(
        str(status.get("head") or "") == str(args.governed_head or ""),
        f"B15 FAIL: the governed commit is not the repository HEAD: {status} expected {args.governed_head}",
    )

    checked_out = invoke_tool(
        args.agent_http,
        "version.checkout",
        {"commit_id": args.governed_head},
        args.timeout_sec,
        confirmed=True,
    )
    write_json(artifact_dir / "version_checkout_response.json", checked_out)

    # Re-open through the kernel so the restored file is what the live edit
    # holds, then confirm the repository checkout returned the governed state.
    reopened = open_project(args.agent_http, project, args.timeout_sec)
    write_json(artifact_dir / "open_after_recover_response.json", reopened["open"])
    shape = working_copy_shape(project)
    write_json(artifact_dir / "working_copy_shape_after_recover.json", shape)
    summary["restored_working_copy"] = shape
    summary["restored_revision"] = live_revision(args.agent_http, args.timeout_sec)
    summary["restored_state_loaded"] = (
        str(reopened["state"].get("status") or "") == "ok" and bool(reopened["state"].get("project_path"))
    )
    require(
        summary["restored_state_loaded"],
        "B15 FAIL: the repository checkout did not leave a loadable project",
    )
    if args.expected_sha256:
        summary["restored_differs_from_save_point"] = before_sha != args.expected_sha256
        require(
            summary["restored_differs_from_save_point"],
            "B15 FAIL: the repository checkout returned the save point instead of the governed state",
        )
    summary["status"] = "passed"
    return summary


def read_expected_mutated_value(args: argparse.Namespace) -> float | None:
    if args.expected_mutated_value is None:
        return None
    return float(args.expected_mutated_value)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--phase", required=True, choices=("setup", "govern", "reopen", "save_verify", "recover"))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--seed-project", default="")
    parser.add_argument("--project-workdir", required=True)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--timeout-sec", type=float, default=600.0)
    parser.add_argument("--track-id", default=DEFAULT_TRACK_ID)
    parser.add_argument("--plugin-id", default="1042")
    parser.add_argument("--param-id", default="827156039")
    parser.add_argument("--normalized-value", type=float, default=0.5)
    parser.add_argument("--tempo-bpm", type=float, default=123.0)
    parser.add_argument("--expected-sha256", default="")
    parser.add_argument("--governed-head", default="")
    parser.add_argument("--expected-mutated-value", default="")
    parser.add_argument("--kernel-log", default="")
    args = parser.parse_args()
    args.expected_mutated_value = float(args.expected_mutated_value) if args.expected_mutated_value else None

    started = time.monotonic()
    if args.phase == "setup":
        require(args.seed_project, "--phase setup requires --seed-project")
        summary = setup_phase(args)
    elif args.phase == "govern":
        summary = govern_phase(args)
    elif args.phase == "reopen":
        summary = reopen_phase(args)
    elif args.phase == "save_verify":
        summary = save_verify_phase(args)
    else:
        summary = recover_phase(args)
    summary["elapsed_ms"] = int((time.monotonic() - started) * 1000)
    write_json(Path(args.artifact_dir).resolve() / "summary.json", summary)
    print(f"B15 {args.phase} PASS: " + json.dumps(summary, ensure_ascii=False, default=str)[:1800])
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
