"""Product-path smoke for B2 static balance pending-plan generation."""

from __future__ import annotations

import argparse
import json
import time
import urllib.request
from pathlib import Path
from typing import Any


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as response:
        parsed = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return parsed


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retry diagnostics belong in smoke output.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready at {base_url}: {last_error}")


def execution_tools(response: dict[str, Any]) -> list[str]:
    out: list[str] = []
    for row in response.get("executed_kernel_reply", []):
        if not isinstance(row, dict):
            continue
        name = str(row.get("tool", row.get("command_name", ""))).strip()
        if name:
            out.append(name)
    return out


def invoke_tool(base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False) -> dict[str, Any]:
    payload: dict[str, Any] = {"tool": tool, "args": args, "source": "b2_static_balance_agent_smoke.fixture"}
    if confirmed:
        payload["confirmed"] = True
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:1800]}")
    return response


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result")
    return result if isinstance(result, dict) else {}


def track_volumes(base_url: str, timeout: float) -> dict[str, float]:
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    out: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = row.get("volume_db", row.get("fader_db"))
        if track_id and isinstance(value, (int, float)):
            out[track_id] = float(value)
    return out


def prepare_stems_fixture(base_url: str, folder: Path, project_path: Path, timeout: float, dad_timeout: float) -> dict[str, Any]:
    invoke_tool(base_url, "project.new", {}, timeout, confirmed=True)
    invoke_tool(base_url, "project.save_as", {"file_path": str(project_path)}, timeout, confirmed=True)
    imported = invoke_tool(
        base_url,
        "project.import_folder_as_stems",
        {
            "folder_path": str(folder),
            "recursive": False,
            "target_policy": "create_tracks",
            "start_time_seconds": 0.0,
            "skip_unreadable": False,
            "command_timeout_ms": int(timeout * 1000),
        },
        timeout,
        confirmed=True,
    )
    imported_result = result_map(imported)
    job = imported_result.get("analysis_job") if isinstance(imported_result.get("analysis_job"), dict) else {}
    job_id = str(imported_result.get("analysis_job_id", job.get("analysis_job_id", job.get("job_id", "")))).strip()
    if not job_id:
        raise RuntimeError("stems import did not return analysis_job_id")
    invoke_tool(
        base_url,
        "project.audio_analysis_start",
        {"analysis_job_id": job_id, "interval_ms": 10},
        timeout,
    )
    deadline = time.time() + dad_timeout
    latest: dict[str, Any] = {}
    while time.time() < deadline:
        latest = invoke_tool(
            base_url,
            "project.audio_analysis_status",
            {"analysis_job_id": job_id, "latest": True},
            timeout,
        )
        status_result = result_map(latest)
        analysis_job = status_result.get("analysis_job") if isinstance(status_result.get("analysis_job"), dict) else {}
        ready_count = int(analysis_job.get("dad_fact_ready_count") or 0)
        total_count = int(analysis_job.get("dad_fact_total_count") or 0)
        dad_status = str(analysis_job.get("dad_fact_status", "")).lower()
        waveform_rows = analysis_job.get("track_waveform_envelopes")
        waveform_count = len(waveform_rows) if isinstance(waveform_rows, list) else 0
        if total_count > 0 and ready_count >= total_count and dad_status == "ready" and waveform_count >= total_count:
            return {
                "analysis_job_id": job_id,
                "dad_fact_status": dad_status,
                "dad_fact_ready_count": ready_count,
                "dad_fact_total_count": total_count,
                "track_waveform_envelope_count": waveform_count,
                "tracks_created": (imported_result.get("summary") or {}).get("tracks_created"),
            }
        time.sleep(1.0)
    raise RuntimeError(
        "DAD fixture did not become ready: "
        + json.dumps(result_map(latest).get("analysis_job", result_map(latest)), ensure_ascii=False)[:3000]
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    parser.add_argument("--prepare-stems-folder", default="")
    parser.add_argument("--decision", choices=("cancel", "approve"), default="cancel")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b2_static_balance_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)
    fixture: dict[str, Any] | None = None
    if args.prepare_stems_folder:
        folder = Path(args.prepare_stems_folder).resolve()
        if not folder.is_dir():
            raise RuntimeError(f"fixture stems folder not found: {folder}")
        fixture = prepare_stems_fixture(
            args.agent_http,
            folder,
            artifact_dir / "fixture_project.vit",
            args.timeout_sec,
            args.dad_timeout_sec,
        )
    conversation_id = f"b2_static_balance_smoke_{run_stamp}"
    pending = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": "帮我做一下B2功能",
            "context": {"agent_mode": "chat"},
        },
        args.timeout_sec,
    )
    (artifact_dir / "pending_response.json").write_text(
        json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    tools = execution_tools(pending)
    required_tools = {"mix.observe", "project.state", "project.audio_analysis_status"}
    missing_tools = sorted(required_tools.difference(tools))
    forbidden_tools = sorted({"project.audio_analysis_start", "project.audio_analysis_cancel"}.intersection(tools))
    actions = ((pending.get("workflow_data") or {}).get("actions") or [])
    interaction_kinds = [
        str(row.get("kind", ""))
        for row in pending.get("interaction_requests", [])
        if isinstance(row, dict)
    ]
    static_balance_interaction = next(
        (
            row
            for row in pending.get("interaction_requests", [])
            if isinstance(row, dict) and str(row.get("kind", "")) == "static_balance_confirmation"
        ),
        None,
    )
    pending_ok = (
        pending.get("needs_confirmation") is True
        and str(pending.get("workflow", "")) == "static_balance"
        and str(pending.get("stop_reason", "")) == "needs_confirmation"
        and "static_balance_confirmation" in interaction_kinds
        and isinstance(actions, list)
        and len(actions) > 0
        and not missing_tools
        and not forbidden_tools
    )
    if not pending_ok:
        raise RuntimeError(
            "B2 did not produce a complete pending static-balance plan: "
            + json.dumps(
                {
                    "needs_confirmation": pending.get("needs_confirmation"),
                    "workflow": pending.get("workflow"),
                    "stop_reason": pending.get("stop_reason"),
                    "action_count": len(actions) if isinstance(actions, list) else -1,
                    "interaction_kinds": interaction_kinds,
                    "tools": tools,
                    "missing_tools": missing_tools,
                    "forbidden_tools": forbidden_tools,
                    "reply": pending.get("reply"),
                },
                ensure_ascii=False,
            )
        )

    before_volumes = track_volumes(args.agent_http, args.timeout_sec)
    decision_response: dict[str, Any]
    decision_duration_seconds = 0.0
    if args.decision == "cancel":
        decision_response = request_json(
            "POST",
            args.agent_http.rstrip("/") + "/agent/interaction/respond",
            {
                "interaction_id": str(static_balance_interaction.get("id", "")),
                "action_id": "cancel_static_balance",
                "decision": "cancel_static_balance",
                "payload": {},
            },
            args.timeout_sec,
        )
        if str(decision_response.get("goal_status", "")) != "cancelled":
            raise RuntimeError("B2 pending plan was not cleanly cancelled: " + json.dumps(decision_response, ensure_ascii=False)[:1800])
    else:
        started = time.monotonic()
        decision_response = request_json(
            "POST",
            args.agent_http.rstrip("/") + "/agent/interaction/respond",
            {
                "interaction_id": str(static_balance_interaction.get("id", "")),
                "action_id": "approve",
                "decision": "approve",
                "payload": {},
            },
            args.timeout_sec,
        )
        decision_duration_seconds = time.monotonic() - started
        decision_tools = execution_tools(decision_response)
        expected_tools = ["mix.apply_static_balance_batch", "project.state", "mix.observe"]
        if decision_tools != expected_tools or str(decision_response.get("stop_reason", "")) != "static_balance_applied_verified":
            raise RuntimeError(
                "B2 approve did not complete through one batch and verification: "
                + json.dumps({"tools": decision_tools, "stop_reason": decision_response.get("stop_reason"), "reply": decision_response.get("reply")}, ensure_ascii=False)[:3000]
            )
        targets = {
            str(row.get("track_id", "")): float(row.get("target_db"))
            for row in actions
            if isinstance(row, dict) and str(row.get("track_id", "")).strip() and isinstance(row.get("target_db"), (int, float))
        }
        after_volumes = track_volumes(args.agent_http, args.timeout_sec)
        mismatched = [track_id for track_id, target in targets.items() if track_id not in after_volumes or abs(after_volumes[track_id] - target) > 0.05]
        changed_non_targets = [track_id for track_id, before in before_volumes.items() if track_id not in targets and abs(after_volumes.get(track_id, before) - before) > 0.05]
        if len(targets) != len(actions) or mismatched or changed_non_targets:
            raise RuntimeError(
                f"B2 fader verification failed: targets={len(targets)}/{len(actions)} mismatched={len(mismatched)} changed_non_targets={len(changed_non_targets)}"
            )

    history_messages = (decision_response.get("project_history") or {}).get("conversation_messages") or []
    if (
        len(history_messages) < 2
        or not isinstance(history_messages[-2], dict)
        or not isinstance(history_messages[-1], dict)
        or str(history_messages[-2].get("role", "")) != "user"
        or not str(history_messages[-2].get("content", "")).strip()
        or str(history_messages[-1].get("role", "")) != "assistant"
        or str(history_messages[-1].get("content", "")) != str(decision_response.get("reply", ""))
    ):
        raise RuntimeError(
            "B2 interaction decision was not appended to project conversation history: "
            + json.dumps(history_messages[-4:], ensure_ascii=False)[:2400]
        )

    (artifact_dir / f"{args.decision}_response.json").write_text(
        json.dumps(decision_response, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    summary = {
        "schema_version": "b2_static_balance_agent_smoke.v2",
        "status": "ok",
        "agent_http": args.agent_http,
        "health": health,
        "fixture": fixture,
        "conversation_id": conversation_id,
        "preflight_tools": tools,
        "action_count": len(actions),
        "workflow": pending.get("workflow"),
        "stop_reason": pending.get("stop_reason"),
        "decision": args.decision,
        "decision_path": "agent_interaction_respond",
        "history_tail_roles": [str(row.get("role", "")) for row in history_messages[-2:] if isinstance(row, dict)],
        "decision_stop_reason": decision_response.get("stop_reason"),
        "decision_duration_seconds": round(decision_duration_seconds, 3),
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
