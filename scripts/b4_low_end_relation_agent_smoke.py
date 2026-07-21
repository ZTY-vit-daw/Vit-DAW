"""Product-path smoke for B4 low-end relation read-only observation capability."""

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
        "source": "b4_low_end_relation_agent_smoke.fixture",
        "confirmed": confirmed,
    }
    response = request_json("POST", base_url.rstrip("/") + "/agent/invoke", payload, timeout)
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"fixture tool {tool} failed: {json.dumps(response, ensure_ascii=False)[:2200]}")
    return response


def chat(base_url: str, conversation_id: str, message: str, timeout: float) -> dict[str, Any]:
    return request_json(
        "POST",
        base_url.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": message,
            "context": {"agent_mode": "chat"},
        },
        timeout,
    )


def workflow_data(response: dict[str, Any]) -> dict[str, Any]:
    data = response.get("workflow_data")
    return data if isinstance(data, dict) else {}


def canary_stage(response: dict[str, Any]) -> str:
    return str(workflow_data(response).get("canary_stage", ""))


def track_mix_state(base_url: str, timeout: float) -> dict[str, dict[str, float]]:
    # The capability_runtime_v1 handler family never populates
    # executed_kernel_reply (see comments below), so "B4 wrote nothing" cannot
    # be checked by inspecting tool-call lists for this workflow. The only
    # reliable read-only proof is a before/after snapshot of the actual mix
    # state B4 could plausibly touch: fader volume and pan per track.
    state = result_map(invoke_tool(base_url, "project.state", {}, timeout))
    out: dict[str, dict[str, float]] = {}
    for row in state.get("tracks", []):
        if not isinstance(row, dict):
            continue
        track_id = str(row.get("track_id") or row.get("id") or "").strip()
        if not track_id:
            continue
        volume = row.get("volume_db", row.get("fader_db"))
        pan = row.get("pan", row.get("pan_value"))
        entry: dict[str, float] = {}
        if isinstance(volume, (int, float)):
            entry["volume_db"] = float(volume)
        if isinstance(pan, (int, float)):
            entry["pan"] = float(pan)
        if entry:
            out[track_id] = entry
    return out


def mix_state_diff(before: dict[str, dict[str, float]], after: dict[str, dict[str, float]]) -> list[str]:
    changed: list[str] = []
    for track_id, before_values in before.items():
        after_values = after.get(track_id, {})
        for key, value in before_values.items():
            if abs(after_values.get(key, value) - value) > 0.001:
                changed.append(f"{track_id}.{key}")
    return changed


def prepare_fixture(
    base_url: str, project_path: Path, sub_bass_audio: Path, secondary_audio: Path, timeout: float
) -> dict[str, Any]:
    invoke_tool(base_url, "project.new", {}, timeout, confirmed=True)
    invoke_tool(base_url, "project.save_as", {"file_path": str(project_path)}, timeout, confirmed=True)
    created: list[dict[str, str]] = []
    # Track 1 carries the pure 100Hz tone so there is real sub/bass-band energy
    # to detect. Tracks 2 and 3 reuse the shared 3s fixture so B4 has at least
    # three tracks of project context to reason about.
    plan = (
        ("Sub Bass Tone", sub_bass_audio),
        ("Secondary A", secondary_audio),
        ("Secondary B", secondary_audio),
    )
    for name, audio_path in plan:
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


def poll_audio_analysis_ready(base_url: str, timeout: float, dad_timeout: float) -> dict[str, Any]:
    deadline = time.time() + dad_timeout
    latest: dict[str, Any] = {}
    while time.time() < deadline:
        latest = invoke_tool(base_url, "project.audio_analysis_status", {"latest": True}, timeout)
        status_result = result_map(latest)
        analysis_job = status_result.get("analysis_job") if isinstance(status_result.get("analysis_job"), dict) else {}
        dad_status = str(analysis_job.get("dad_fact_status", status_result.get("dad_fact_status", ""))).lower()
        if dad_status == "ready":
            return {
                "dad_fact_status": dad_status,
                "dad_fact_ready_count": analysis_job.get("dad_fact_ready_count", status_result.get("dad_fact_ready_count")),
                "dad_fact_total_count": analysis_job.get("dad_fact_total_count", status_result.get("dad_fact_total_count")),
            }
        time.sleep(1.0)
    raise RuntimeError(
        "B4 fixture audio analysis did not become ready: "
        + json.dumps(result_map(latest), ensure_ascii=False)[:3000]
    )


FORBIDDEN_WRITE_TOOLS = {
    "track.volume",
    "clip.gain.set",
    "mix.apply_tick",
    "mix.apply_static_balance_batch",
    "mix.apply_pan_layout_batch",
}


def forbidden_write_tools(tools: list[str]) -> list[str]:
    hits: list[str] = []
    for name in tools:
        if name in FORBIDDEN_WRITE_TOOLS:
            hits.append(name)
            continue
        base = name.rsplit(".", 1)[-1]
        if base == "set" or base.startswith("apply"):
            hits.append(name)
    return hits


def assert_read_only(turn_label: str, response: dict[str, Any]) -> list[str]:
    # tools is expected to be empty for every capability_runtime_v1 response
    # today (see track_mix_state/mix_state_diff for the actual read-only
    # proof); this check stays as defense-in-depth in case executed_kernel_reply
    # ever gets populated for this handler family.
    tools = execution_tools(response)
    hits = forbidden_write_tools(tools)
    if hits:
        raise RuntimeError(f"B4 {turn_label} used forbidden write tool(s): {hits} (tools={tools})")
    if response.get("needs_confirmation"):
        raise RuntimeError(
            f"B4 {turn_label} set needs_confirmation=true; B4 must never produce a pending plan: "
            + json.dumps(response.get("workflow_data"), ensure_ascii=False)[:1600]
        )
    if response.get("interaction_requests"):
        raise RuntimeError(
            f"B4 {turn_label} returned interaction_requests; B4 must never request interaction: "
            + json.dumps(response.get("interaction_requests"), ensure_ascii=False)[:1600]
        )
    return tools


B4_QUESTION = "分析一下当前工程的低频关系"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo-root", default=str(Path(__file__).resolve().parents[1]))
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--timeout-sec", type=float, default=180.0)
    parser.add_argument("--dad-timeout-sec", type=float, default=240.0)
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve()
    run_stamp = time.strftime("%Y%m%d_%H%M%S")
    artifact_dir = repo / "VitApp" / "Workspace" / "Artifacts" / "smoke" / f"b4_low_end_relation_{run_stamp}"
    artifact_dir.mkdir(parents=True, exist_ok=True)

    health = wait_agent(args.agent_http, args.timeout_sec)

    sub_bass_audio = repo / "test_100hz_10s.wav"
    if not sub_bass_audio.is_file():
        raise RuntimeError(f"fixture audio not found: {sub_bass_audio}")
    secondary_audio = repo / "test_target_3s.wav"
    if not secondary_audio.is_file():
        raise RuntimeError(f"fixture audio not found: {secondary_audio}")

    fixture = prepare_fixture(
        args.agent_http, artifact_dir / "fixture_project.vit", sub_bass_audio, secondary_audio, args.timeout_sec
    )

    conversation_id = f"b4_low_end_relation_smoke_{run_stamp}"
    mix_state_before_all_turns = track_mix_state(args.agent_http, args.timeout_sec)

    # Turn 1: cold-start. This is the regression guard: before the fix, B4
    # would report canary_stage=readiness_blocked forever with no path
    # forward, because nothing auto-triggers L3 band-energy analysis and the
    # LLM tool-call guard blocks the agent from starting it. The fix wires the
    # auto-trigger into mix.observe itself and synchronously waits on the
    # background collector for a short warmup window, so on fast fixtures the
    # analysis can already be complete by the time this call returns
    # (canary_stage="analysis" directly) rather than merely "triggered".
    # Both outcomes are acceptable; only readiness_blocked is a regression.
    cold_start = chat(args.agent_http, conversation_id, B4_QUESTION, args.timeout_sec)
    (artifact_dir / "turn1_cold_start_response.json").write_text(
        json.dumps(cold_start, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    cold_start_tools = assert_read_only("cold-start turn", cold_start)
    cold_start_stage = canary_stage(cold_start)
    if cold_start_stage == "readiness_blocked":
        raise RuntimeError(
            "REGRESSION: B4 returned canary_stage=readiness_blocked on cold start with no "
            "auto-trigger path; this is the dead end the fix was meant to eliminate. "
            + json.dumps(workflow_data(cold_start), ensure_ascii=False)[:2000]
        )
    if cold_start_stage not in {"band_analysis_triggered", "analysis"}:
        raise RuntimeError(
            f"B4 cold start reached an unexpected canary_stage (got {cold_start_stage!r}); "
            "expected band_analysis_triggered or analysis: "
            + json.dumps(workflow_data(cold_start), ensure_ascii=False)[:2000]
        )

    analysis_turn = cold_start
    analysis_stage = cold_start_stage
    analysis_tools = cold_start_tools
    dad_ready: dict[str, Any] = {}

    if cold_start_stage == "band_analysis_triggered":
        # The capability_runtime_v1 canary handler family (B2/B3/B4) never
        # populates executed_kernel_reply -- unlike the ordinary
        # AgentLoop/goalrunner path that b2/b3's fixture helpers exercise --
        # so "claims it triggered analysis" can only be confirmed by
        # observing the real side effect: query project.audio_analysis_status
        # right after and confirm a job now exists (before any trigger it
        # would report "could not find an analysis job").
        post_trigger_status = invoke_tool(args.agent_http, "project.audio_analysis_status", {"latest": True}, args.timeout_sec)
        post_trigger_result = result_map(post_trigger_status)
        post_trigger_job = post_trigger_result.get("analysis_job") if isinstance(post_trigger_result.get("analysis_job"), dict) else {}
        if not (post_trigger_result.get("analysis_job_id") or post_trigger_job.get("analysis_job_id") or post_trigger_result.get("dad_fact_status") or post_trigger_job.get("dad_fact_status")):
            raise RuntimeError(
                "B4 cold-start turn claimed band_analysis_triggered but "
                "project.audio_analysis_status shows no analysis job exists yet: "
                + json.dumps(post_trigger_result, ensure_ascii=False)[:2000]
            )

        # Poll to readiness (pattern mirrors b2_static_balance_agent_smoke.prepare_stems_fixture).
        dad_ready = poll_audio_analysis_ready(args.agent_http, args.timeout_sec, args.dad_timeout_sec)

        # Turn 2: re-ask now that band-energy evidence exists; readiness gate clears.
        analysis_turn = chat(args.agent_http, conversation_id, B4_QUESTION, args.timeout_sec)
        (artifact_dir / "turn2_analysis_response.json").write_text(
            json.dumps(analysis_turn, ensure_ascii=False, indent=2), encoding="utf-8"
        )
        analysis_tools = assert_read_only("post-readiness analysis turn", analysis_turn)
        analysis_stage = canary_stage(analysis_turn)
        if analysis_stage != "analysis":
            raise RuntimeError(
                f"B4 did not reach canary_stage=analysis after readiness cleared (got {analysis_stage!r}): "
                + json.dumps(workflow_data(analysis_turn), ensure_ascii=False)[:2000]
            )

    low_end_summary = workflow_data(analysis_turn).get("low_end_summary")
    if not isinstance(low_end_summary, dict):
        raise RuntimeError(
            "B4 analysis turn did not include workflow_data.low_end_summary: "
            + json.dumps(workflow_data(analysis_turn), ensure_ascii=False)[:2000]
        )
    sub_band_track_count = low_end_summary.get("sub_band_track_count", 0)
    bass_band_track_count = low_end_summary.get("bass_band_track_count", 0)
    conflict_count = low_end_summary.get("conflict_count", 0)
    observation_count = workflow_data(analysis_turn).get("observation_count", 0)
    if not (int(sub_band_track_count or 0) > 0 or int(bass_band_track_count or 0) > 0):
        raise RuntimeError(
            "B4 low_end_summary evidence is empty: neither sub_band_track_count nor "
            f"bass_band_track_count is > 0: {json.dumps(low_end_summary, ensure_ascii=False)[:2000]}"
        )

    # Turn 3: idempotent re-trigger safety. Readiness already satisfied, so B4
    # must go straight to analysis without re-invoking project.audio_analysis_start.
    idempotent_turn = chat(args.agent_http, conversation_id, B4_QUESTION, args.timeout_sec)
    (artifact_dir / "turn3_idempotent_response.json").write_text(
        json.dumps(idempotent_turn, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    idempotent_tools = assert_read_only("idempotent re-trigger turn", idempotent_turn)
    idempotent_stage = canary_stage(idempotent_turn)
    # canary_stage is the reliable idempotency signal here: if B4 had a bug that
    # re-triggered analysis every time readiness was already satisfied, this
    # turn would come back as "band_analysis_triggered" again instead of
    # settling on "analysis". (executed_kernel_reply cannot be used for this --
    # see the cold-start comment above; this handler family never populates it.)
    if idempotent_stage != "analysis":
        raise RuntimeError(
            f"B4 idempotent turn did not stay at canary_stage=analysis (got {idempotent_stage!r}); "
            "a value of band_analysis_triggered here would mean B4 redundantly "
            "re-triggered audio analysis after readiness was already satisfied: "
            + json.dumps(workflow_data(idempotent_turn), ensure_ascii=False)[:2000]
        )

    # Real read-only proof: compare fader volume/pan across all three B4 turns
    # against the pre-turn snapshot. B4 must never move a fader or pan knob.
    mix_state_after_all_turns = track_mix_state(args.agent_http, args.timeout_sec)
    mutated_tracks = mix_state_diff(mix_state_before_all_turns, mix_state_after_all_turns)
    if mutated_tracks:
        raise RuntimeError(
            "B4 mutated track volume/pan across its three read-only turns, "
            f"which must never happen: {mutated_tracks}"
        )

    summary = {
        "schema_version": "b4_low_end_relation_agent_smoke.v1",
        "status": "ok",
        "health": health,
        "fixture": fixture,
        "conversation_id": conversation_id,
        "dad_ready": dad_ready,
        "turns": {
            "cold_start": {"canary_stage": cold_start_stage, "tools": cold_start_tools},
            "analysis": {"canary_stage": analysis_stage, "tools": analysis_tools},
            "idempotent": {"canary_stage": idempotent_stage, "tools": idempotent_tools},
        },
        "low_end_summary": {
            "sub_band_track_count": sub_band_track_count,
            "bass_band_track_count": bass_band_track_count,
            "conflict_count": conflict_count,
            "observation_count": observation_count,
        },
        "read_only_verified": not mutated_tracks,
        "mix_state_track_count": len(mix_state_before_all_turns),
        "artifact_dir": str(artifact_dir),
    }
    (artifact_dir / "summary.json").write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
