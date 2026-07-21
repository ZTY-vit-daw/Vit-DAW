"""Product-path smoke for B1.2 recovery after A4 creates multi-clip tracks."""

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
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        result = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(result, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return result


def invoke(base: str, tool: str, args: dict[str, Any], timeout: float, confirmed: bool = False) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "tool": tool,
        "args": args,
        "source": "b1_2_a4_multiclip_agent_smoke",
    }
    if confirmed:
        payload["confirmed"] = True
    return request_json("POST", base.rstrip("/") + "/agent/invoke", payload, timeout)


def state_snapshot(base: str, timeout: float) -> tuple[dict[str, Any], dict[str, float]]:
    response = invoke(base, "project.state", {}, timeout)
    shadow = response.get("result") if isinstance(response.get("result"), dict) else {}
    gains: dict[str, float] = {}
    for track in shadow.get("tracks", []):
        if not isinstance(track, dict):
            continue
        for clip in track.get("clips", []):
            if not isinstance(clip, dict):
                continue
            clip_id = str(clip.get("clip_id") or clip.get("id") or "").strip()
            if clip_id:
                gains[clip_id] = float(clip.get("clip_gain_db") or clip.get("gain_db") or 0.0)
    return shadow, gains


def wait_multiclip_project(base: str, timeout: float) -> tuple[dict[str, Any], dict[str, float]]:
    deadline = time.time() + timeout
    latest: tuple[dict[str, Any], dict[str, float]] = ({}, {})
    while time.time() < deadline:
        latest = state_snapshot(base, min(timeout, 30.0))
        tracks = [row for row in latest[0].get("tracks", []) if isinstance(row, dict)]
        if len(tracks) >= 2 and any(len(row.get("clips", [])) > 1 for row in tracks):
            return latest
        time.sleep(0.5)
    tracks = [row for row in latest[0].get("tracks", []) if isinstance(row, dict)]
    raise RuntimeError(f"fixture did not become a multi-clip project: tracks={len(tracks)}")


def chat(base: str, conversation_id: str, message: str, timeout: float) -> dict[str, Any]:
    return request_json(
        "POST",
        base.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation_id, "message": message, "context": {"agent_mode": "chat"}},
        timeout,
    )


def wait_dad(base: str, timeout: float) -> dict[str, Any]:
    deadline = time.time() + timeout
    latest: dict[str, Any] = {}
    while time.time() < deadline:
        response = invoke(base, "project.audio_analysis_status", {"latest": True}, min(timeout, 120.0))
        result = response.get("result") if isinstance(response.get("result"), dict) else {}
        latest = result.get("analysis_job") if isinstance(result.get("analysis_job"), dict) else {}
        ready = int(latest.get("dad_fact_ready_count") or 0)
        total = int(latest.get("dad_fact_total_count") or 0)
        if total > 0 and ready >= total and str(latest.get("dad_fact_status", "")).lower() == "ready":
            return latest
        time.sleep(1.0)
    raise RuntimeError("DAD did not become ready: " + json.dumps(latest, ensure_ascii=False)[:2000])


def pending_command(response: dict[str, Any]) -> dict[str, Any]:
    for request in response.get("interaction_requests", []):
        if not isinstance(request, dict):
            continue
        payload = request.get("payload") if isinstance(request.get("payload"), dict) else {}
        for row in payload.get("commands", []):
            if not isinstance(row, dict):
                continue
            command = row.get("command") if isinstance(row.get("command"), dict) else {}
            if command.get("cmd") == "clip.gain.set_batch":
                return command
    return {}


def pending_targets(actions: list[Any]) -> dict[str, float]:
    targets: dict[str, float] = {}
    for row in actions:
        if not isinstance(row, dict):
            continue
        args = row.get("args") if isinstance(row.get("args"), dict) else row
        clip_id = str(args.get("clip_id") or row.get("clip_id") or "").strip()
        gain_db = args.get("gain_db", row.get("gain_db"))
        if clip_id and isinstance(gain_db, (int, float)):
            targets[clip_id] = float(gain_db)
    return targets


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--project-path", required=True)
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--decision", choices=("deny", "approve"), default="deny")
    parser.add_argument("--full-b1", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    project = Path(args.project_path).resolve()
    if not project.is_file():
        raise RuntimeError(f"project not found: {project}")
    stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b1_2_a4_multiclip_{stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = request_json("GET", args.agent_http.rstrip("/") + "/health", None, 10.0)
    opened = invoke(args.agent_http, "open_project", {"file_path": str(project)}, args.timeout_sec, confirmed=True)
    if str(opened.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError("open_project failed: " + json.dumps(opened, ensure_ascii=False)[:2000])
    shadow, before_gains = wait_multiclip_project(args.agent_http, min(args.timeout_sec, 60.0))
    tracks = [row for row in shadow.get("tracks", []) if isinstance(row, dict)]
    multi_clip_tracks = sum(1 for row in tracks if len(row.get("clips", [])) > 1)
    if len(tracks) < 2 or multi_clip_tracks == 0:
        raise RuntimeError(f"fixture is not a multi-clip project: tracks={len(tracks)} multi={multi_clip_tracks}")

    conversation_id = f"b1_2_a4_multiclip_smoke_{stamp}"
    request_message = "\u6267\u884cB1" if args.full_b1 else "\u8fdb\u884cB1.2"
    response = chat(args.agent_http, conversation_id, request_message, args.timeout_sec)
    if response.get("needs_confirmation") is not True:
        raise RuntimeError(
            "B1 workflow did not wait for DAD and queue B1.2 in one request: "
            + json.dumps(response, ensure_ascii=False)[:2000]
        )
    dad = wait_dad(args.agent_http, args.timeout_sec)

    command = pending_command(response)
    actions = command.get("pending_actions") if isinstance(command.get("pending_actions"), list) else []
    summary = command.get("calibration_summary") if isinstance(command.get("calibration_summary"), dict) else {}
    track_action_count = int(summary.get("track_action_count") or 0)
    clip_action_count = int(summary.get("clip_action_count") or 0)
    distinct_tracks = len({str(row.get("track_id", "")) for row in actions if isinstance(row, dict)})
    distinct_clips = len({str(row.get("clip_id", "")) for row in actions if isinstance(row, dict)})
    if response.get("needs_confirmation") is not True or not response.get("plan_id"):
        raise RuntimeError("B1.2 did not create a pending plan: " + json.dumps(response, ensure_ascii=False)[:2000])
    if not actions or clip_action_count != len(actions) or distinct_clips != len(actions):
        raise RuntimeError("B1.2 pending clip expansion is inconsistent")
    if track_action_count != distinct_tracks or clip_action_count <= track_action_count:
        raise RuntimeError("B1.2 did not expand track calibration across multi-clip tracks")

    targets = pending_targets(actions)
    if len(targets) != clip_action_count:
        raise RuntimeError("B1.2 pending actions are missing target clip gains")

    decision_result = request_json(
        "POST",
        args.agent_http.rstrip("/") + "/agent/confirm",
        {"plan_id": response["plan_id"], "decision": args.decision},
        args.timeout_sec,
    )
    time.sleep(1.0)
    _, after_gains = state_snapshot(args.agent_http, 30.0)
    mismatched_targets: list[str] = []
    changed_non_targets: list[str] = []
    if args.decision == "deny":
        if before_gains != after_gains:
            raise RuntimeError("denied B1.2 pending plan changed clip gains")
    else:
        mismatched_targets = [
            clip_id
            for clip_id, target_db in targets.items()
            if clip_id not in after_gains or abs(after_gains[clip_id] - target_db) > 0.01
        ]
        changed_non_targets = [
            clip_id
            for clip_id, before_db in before_gains.items()
            if clip_id not in targets and abs(after_gains.get(clip_id, before_db) - before_db) > 0.01
        ]
        if mismatched_targets or changed_non_targets:
            raise RuntimeError(
                f"approved B1.2 verification failed: mismatched_targets={len(mismatched_targets)} "
                f"changed_non_targets={len(changed_non_targets)}"
            )

    output = {
        "schema_version": "b1_2_a4_multiclip_agent_smoke.v2",
        "status": "ok",
        "project_path": str(project),
        "health": health,
        "track_count": len(tracks),
        "clip_count": len(before_gains),
        "multi_clip_track_count": multi_clip_tracks,
        "dad_ready_count": dad.get("dad_fact_ready_count"),
        "dad_total_count": dad.get("dad_fact_total_count"),
        "track_action_count": track_action_count,
        "clip_action_count": clip_action_count,
        "distinct_track_count": distinct_tracks,
        "distinct_clip_count": distinct_clips,
        "plan_id": response["plan_id"],
        "request_mode": "full_b1" if args.full_b1 else "b1_2",
        "decision": args.decision,
        "decision_status": decision_result.get("status") or decision_result.get("goal_status"),
        "verified_target_count": len(targets) if args.decision == "approve" else 0,
        "mismatched_target_count": len(mismatched_targets),
        "changed_non_target_count": len(changed_non_targets),
        "mutation_after_deny": before_gains != after_gains if args.decision == "deny" else None,
        "mutation_after_approve": before_gains != after_gains if args.decision == "approve" else None,
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(output, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(output, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
