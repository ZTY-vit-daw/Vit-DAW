"""Godot-product-path smoke for B3 static pan-layout planning and execution."""

from __future__ import annotations

import argparse
import json
import time
import urllib.request
from pathlib import Path
from typing import Any


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        result = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(result, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return result


def wait_agent(base_url: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return health
        except Exception as exc:  # noqa: BLE001 - retry details belong in smoke artifacts.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready at {base_url}: {last_error}")


def execution_tools(response: dict[str, Any]) -> list[str]:
    tools: list[str] = []
    for row in response.get("executed_kernel_reply", []):
        if isinstance(row, dict):
            name = str(row.get("tool", row.get("command_name", ""))).strip()
            if name:
                tools.append(name)
    return tools


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    result = response.get("result")
    return result if isinstance(result, dict) else {}


def invoke_tool(
    base_url: str, tool: str, args: dict[str, Any], timeout: float, *, confirmed: bool = False
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "b3_pan_layout_agent_smoke.fixture",
        "confirmed": confirmed,
    }
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:2200]}")
    return response


def track_pans(base_url: str, timeout: float) -> dict[str, float]:
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    pans: dict[str, float] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        value = row.get("pan", row.get("pan_value"))
        if track_id and isinstance(value, (int, float)):
            pans[track_id] = float(value)
    return pans


def prepare_fixture(base_url: str, project_path: Path, audio_path: Path, timeout: float) -> dict[str, Any]:
    invoke_tool(base_url, "project.new", {}, timeout, confirmed=True)
    invoke_tool(base_url, "project.save_as", {"file_path": str(project_path)}, timeout, confirmed=True)
    created: list[dict[str, str]] = []
    for name in (
        "Lead Vocal",
        "Guitar Double 1",
        "Guitar Double 2",
        "Backing Vocal 1",
        "Backing Vocal 2",
        "Percussion 1",
        "Percussion 2",
    ):
        response = invoke_tool(base_url, "track.add_audio", {"name": name}, timeout, confirmed=True)
        result = result_map(response)
        track_id = str(result.get("track_id") or result.get("id") or result.get("item_id") or "").strip()
        if not track_id:
            raise RuntimeError(f"track.add_audio did not return track_id for {name}: {json.dumps(response, ensure_ascii=False)[:1200]}")
        invoke_tool(
            base_url,
            "track.rename",
            {"track_id": track_id, "name": name, "track_name": name},
            timeout,
            confirmed=True,
        )
        invoke_tool(
            base_url,
            "clip.import_audio",
            {"track_id": track_id, "file_path": str(audio_path), "offset_time": 0.0},
            timeout,
            confirmed=True,
        )
        created.append({"track_id": track_id, "track_name": name})
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    return {"project_path": str(project_path), "created": created, "track_count": len(state.get("tracks", []))}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--skip-fixture", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b3_pan_layout_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)
    fixture = None
    if not args.skip_fixture:
        audio_path = repo / "test_target_3s.wav"
        if not audio_path.is_file():
            raise RuntimeError(f"fixture audio not found: {audio_path}")
        fixture = prepare_fixture(args.agent_http, artifact_dir / "fixture_project.vit", audio_path, args.timeout_sec)

    before = track_pans(args.agent_http, args.timeout_sec)
    conversation_id = f"b3_pan_layout_smoke_{run_stamp}"
    pending = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": "请按现代流行风格执行 B3 全工程静态声像布局",
            "context": {"agent_mode": "chat"},
        },
        args.timeout_sec,
    )
    (artifact_dir / "pending_response.json").write_text(
        json.dumps(pending, ensure_ascii=False, indent=2), encoding="utf-8"
    )

    preflight_tools = execution_tools(pending)
    required_preflight = {"mix.observe", "project.state"}
    missing_preflight = sorted(required_preflight.difference(preflight_tools))
    forbidden_preflight = sorted(
        {"mix.apply_pan_layout_batch", "mix.apply_tick", "track.pan", "clip.gain.set"}.intersection(preflight_tools)
    )
    workflow_data = pending.get("workflow_data") if isinstance(pending.get("workflow_data"), dict) else {}
    actions = workflow_data.get("actions") if isinstance(workflow_data.get("actions"), list) else []
    interaction = next(
        (
            row
            for row in pending.get("interaction_requests", [])
            if isinstance(row, dict) and str(row.get("kind", "")) == "pan_layout_confirmation"
        ),
        None,
    )
    after_pending = track_pans(args.agent_http, args.timeout_sec)
    changed_before_confirmation = [
        track_id for track_id, value in before.items() if abs(after_pending.get(track_id, value) - value) > 0.001
    ]
    if (
        pending.get("needs_confirmation") is not True
        or str(pending.get("workflow", "")) != "pan_layout"
        or str(pending.get("stop_reason", "")) != "needs_confirmation"
        or not isinstance(interaction, dict)
        or not actions
        or missing_preflight
        or forbidden_preflight
        or changed_before_confirmation
    ):
        raise RuntimeError(
            "B3 did not produce a read-only complete pending plan: "
            + json.dumps(
                {
                    "workflow": pending.get("workflow"),
                    "stop_reason": pending.get("stop_reason"),
                    "needs_confirmation": pending.get("needs_confirmation"),
                    "action_count": len(actions),
                    "preflight_tools": preflight_tools,
                    "missing_preflight": missing_preflight,
                    "forbidden_preflight": forbidden_preflight,
                    "changed_before_confirmation": changed_before_confirmation,
                    "reply": pending.get("reply"),
                },
                ensure_ascii=False,
            )
        )

    decision = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/interaction/respond",
        {
            "interaction_id": str(interaction.get("id", "")),
            "action_id": "approve",
            "decision": "approve",
            "payload": {},
        },
        args.timeout_sec,
    )
    (artifact_dir / "approve_response.json").write_text(
        json.dumps(decision, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    decision_tools = execution_tools(decision)
    expected_tools = ["mix.apply_pan_layout_batch", "project.state", "mix.observe"]
    if decision_tools != expected_tools or str(decision.get("stop_reason", "")) != "pan_layout_applied_verified":
        raise RuntimeError(
            "B3 approval did not use one batch plus two readbacks: "
            + json.dumps(
                {"tools": decision_tools, "stop_reason": decision.get("stop_reason"), "reply": decision.get("reply")},
                ensure_ascii=False,
            )[:3200]
        )

    targets = {
        str(row.get("track_id", "")): float(row.get("target_pan"))
        for row in actions
        if isinstance(row, dict)
        and str(row.get("track_id", "")).strip()
        and isinstance(row.get("target_pan"), (int, float))
    }
    after = track_pans(args.agent_http, args.timeout_sec)
    mismatched = [
        track_id for track_id, target in targets.items() if track_id not in after or abs(after[track_id] - target) > 0.01
    ]
    changed_non_targets = [
        track_id
        for track_id, value in before.items()
        if track_id not in targets and abs(after.get(track_id, value) - value) > 0.001
    ]
    if len(targets) != len(actions) or mismatched or changed_non_targets:
        raise RuntimeError(
            f"B3 pan verification failed: targets={len(targets)}/{len(actions)} "
            f"mismatched={mismatched} changed_non_targets={changed_non_targets}"
        )

    history_messages = (decision.get("project_history") or {}).get("conversation_messages") or []
    if (
        len(history_messages) < 2
        or not isinstance(history_messages[-2], dict)
        or not isinstance(history_messages[-1], dict)
        or str(history_messages[-2].get("role", "")) != "user"
        or str(history_messages[-1].get("role", "")) != "assistant"
        or str(history_messages[-1].get("content", "")) != str(decision.get("reply", ""))
    ):
        raise RuntimeError(
            "B3 confirmation was not appended to project conversation history: "
            + json.dumps(history_messages[-4:], ensure_ascii=False)[:2400]
        )

    project_path = Path(str((fixture or {}).get("project_path", "")))
    reopen_pans: dict[str, float] = {}
    reopen_history: list[Any] = []
    if project_path.is_file():
        invoke_tool(args.agent_http, "project.save", {}, args.timeout_sec, confirmed=True)
        invoke_tool(args.agent_http, "project.new", {}, args.timeout_sec, confirmed=True)
        invoke_tool(
            args.agent_http,
            "project.open",
            {"file_path": str(project_path), "project_path": str(project_path)},
            args.timeout_sec,
            confirmed=True,
        )
        reopen_pans = track_pans(args.agent_http, args.timeout_sec)
        reopen_mismatched = [
            track_id
            for track_id, target in targets.items()
            if track_id not in reopen_pans or abs(reopen_pans[track_id] - target) > 0.01
        ]
        agent_state = request_json("GET", args.agent_http.rstrip("/") + "/agent/state", None, args.timeout_sec)
        (artifact_dir / "reopened_agent_state.json").write_text(
            json.dumps(agent_state, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        project_history = agent_state.get("project_history") if isinstance(agent_state.get("project_history"), dict) else {}
        reopen_history = project_history.get("conversation_messages") if isinstance(project_history.get("conversation_messages"), list) else []
        if (
            reopen_mismatched
            or len(reopen_history) < 2
            or not isinstance(reopen_history[-1], dict)
            or str(reopen_history[-1].get("role", "")) != "assistant"
            or str(reopen_history[-1].get("content", "")) != str(decision.get("reply", ""))
        ):
            raise RuntimeError(
                "B3 save/reopen verification failed: "
                + json.dumps(
                    {
                        "pan_mismatches": reopen_mismatched,
                        "history_tail": reopen_history[-4:],
                        "expected_reply": decision.get("reply"),
                    },
                    ensure_ascii=False,
                )[:3200]
            )

    summary = {
        "schema_version": "b3_pan_layout_agent_smoke.v1",
        "status": "ok",
        "health": health,
        "fixture": fixture,
        "conversation_id": conversation_id,
        "preflight_tools": preflight_tools,
        "action_count": len(actions),
        "changed_before_confirmation": changed_before_confirmation,
        "decision_tools": decision_tools,
        "decision_stop_reason": decision.get("stop_reason"),
        "target_mismatches": mismatched,
        "changed_non_targets": changed_non_targets,
        "history_tail_roles": [str(row.get("role", "")) for row in history_messages[-2:] if isinstance(row, dict)],
        "save_reopen_verified": bool(reopen_pans),
        "reopen_target_count": len(targets),
        "reopen_history_tail_roles": [
            str(row.get("role", "")) for row in reopen_history[-2:] if isinstance(row, dict)
        ],
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
