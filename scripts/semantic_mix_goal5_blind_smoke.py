#!/usr/bin/env python3
"""Run the goal-5 small-project acoustic reasoning cases through ordinary Agent.

The runner deliberately reads only ``fixture_manifest.json`` and opaque case
directories.  It never opens ``sealed/sealed_truth.json`` and never includes
fixture recipes, injected parameters, or expected answers in Agent context.

Every tested turn is diagnostic/read-only.  The runner pre-warms deterministic
acoustic evidence, lets the ordinary Agent choose its own observe/read/derive
route, and verifies that no proposal, confirmation, or project mutation was
created.  Sealed evaluation is a separate post-run step.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.request
from pathlib import Path
from typing import Any

from semantic_mix_goal5_projects import build_case as rebuild_vit_case


SCHEMA_VERSION = "semantic_mix_goal5_blind_run.v1"

# Prompts intentionally describe only the user's audible concern.  They do not
# encode the hidden processing recipe or an expected answer.
CASES: dict[str, dict[str, str]] = {
    "g5_01": {"prompt": "为什么这一段主唱听起来浑？", "focus": "vocals"},
    "g5_02": {"prompt": "为什么这一段主唱听起来浑？", "focus": "vocals"},
    "g5_03": {"prompt": "为什么副歌里人声听起来靠后？", "focus": "vocals"},
    "g5_04": {"prompt": "为什么这一段低频边界不清楚？", "focus": "bass"},
    "g5_05": {"prompt": "为什么这一段低频边界不清楚？", "focus": "bass"},
    "g5_06": {"prompt": "为什么主唱既不够亮又有点刺耳？", "focus": "vocals"},
}

MUTATION_FRAGMENTS = (
    "apply_eq_edits",
    "set_eq_point",
    "set_plugin_param",
    "apply_control",
    "rack_add_node",
    "instantiate_plugin",
    "mix.apply_tick",
    "mix_apply_tick",
    "track.volume",
    "track.pan",
    "set_track_volume",
    "set_track_pan",
)


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def invoke(
    base_url: str,
    tool: str,
    args: dict[str, Any],
    timeout: float,
    *,
    confirmed: bool = False,
) -> dict[str, Any]:
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/invoke",
        {
            "tool": tool,
            "args": args,
            "confirmed": confirmed,
            "source": "semantic_mix_goal5_blind_smoke",
        },
        timeout,
    )
    if str(response.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise RuntimeError(f"{tool} failed: {json.dumps(response, ensure_ascii=False)[:2600]}")
    return response


def result_map(response: dict[str, Any]) -> dict[str, Any]:
    value = response.get("result")
    return value if isinstance(value, dict) else {}


def rows(value: Any) -> list[dict[str, Any]]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def write_json_atomic(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temp, path)


def default_manifest() -> Path:
    local = os.environ.get("LOCALAPPDATA")
    if not local:
        raise RuntimeError("LOCALAPPDATA is unavailable; pass --fixture-manifest")
    pointer = Path(local) / "Vit" / "Goal5Fixtures" / "current.json"
    current = json.loads(pointer.read_text(encoding="utf-8"))
    return Path(str(current["fixture_manifest"])).resolve()


def assert_blind_manifest(manifest_path: Path, manifest: dict[str, Any]) -> None:
    if "sealed" in {part.lower() for part in manifest_path.parts}:
        raise RuntimeError(f"refusing sealed input as fixture manifest: {manifest_path}")
    contract = manifest.get("blindness_contract")
    if not isinstance(contract, dict) or contract.get("sealed_truth_not_in_agent_context") is not True:
        raise RuntimeError("fixture manifest does not assert the sealed-context contract")
    if any(key in manifest for key in ("edits", "issue_kind", "expected_reasoning", "sealed_truth")):
        raise RuntimeError("fixture manifest unexpectedly contains sealed evaluation fields")


def wait_agent(base_url: str, timeout: float) -> None:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base_url.rstrip("/") + "/health", None, 3.0)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # noqa: BLE001 - retained for timeout diagnostics.
            last_error = exc
        time.sleep(0.25)
    raise RuntimeError(f"VitAgent did not become ready: {last_error}")


def project_tracks(state_response: dict[str, Any]) -> list[dict[str, Any]]:
    result = result_map(state_response)
    direct = rows(result.get("tracks"))
    if direct:
        return direct
    observed = result.get("project_observe")
    if isinstance(observed, dict):
        return rows(observed.get("tracks"))
    return []


def track_name(row: dict[str, Any]) -> str:
    return str(row.get("track_name") or row.get("name") or "").strip()


def track_id(row: dict[str, Any]) -> str:
    return str(row.get("track_id") or row.get("id") or "").strip()


def clip_rows(row: dict[str, Any]) -> list[dict[str, Any]]:
    return rows(row.get("clips"))


def stable_project_fingerprint(state_response: dict[str, Any]) -> dict[str, Any]:
    result = result_map(state_response)
    tracks: list[dict[str, Any]] = []
    for row in project_tracks(state_response):
        clips: list[dict[str, Any]] = []
        for clip in clip_rows(row):
            clips.append(
                {
                    "clip_id": str(clip.get("clip_id") or clip.get("id") or ""),
                    "source_path": str(
                        clip.get("current_source_path")
                        or clip.get("source_path")
                        or clip.get("file_path")
                        or ""
                    ),
                    "gain_db": clip.get("clip_gain_db", clip.get("gain_db")),
                    "pan": clip.get("clip_pan", clip.get("pan")),
                    "mute": clip.get("clip_mute", clip.get("mute")),
                    "start_seconds": clip.get("start_seconds"),
                    "length_seconds": clip.get("length_seconds"),
                }
            )
        rack = row.get("rack") if isinstance(row.get("rack"), dict) else {}
        nodes = []
        for node in rows(rack.get("nodes")):
            nodes.append(
                {
                    "plugin_id": str(node.get("plugin_id") or node.get("id") or ""),
                    "plugin_name": str(node.get("plugin_name") or node.get("name") or ""),
                    "bypass": node.get("bypass"),
                }
            )
        tracks.append(
            {
                "track_id": track_id(row),
                "track_name": track_name(row),
                "volume_db": row.get("volume_db", row.get("fader_db", row.get("gain_db"))),
                "pan": row.get("pan", row.get("pan_value")),
                "mute": row.get("mute"),
                "solo": row.get("solo"),
                "clips": clips,
                "rack_nodes": nodes,
            }
        )
    tracks.sort(key=lambda item: (item["track_name"].lower(), item["track_id"]))
    return {
        "project_id": str(result.get("project_id") or result.get("project_uuid") or ""),
        "project_path": str(result.get("project_path") or ""),
        "tracks": tracks,
    }


def acoustic_status(result: dict[str, Any]) -> dict[str, Any]:
    value = result.get("acoustic_package_status")
    return value if isinstance(value, dict) else {}


def package_ready(status: dict[str, Any]) -> bool:
    if str(status.get("status", "")).lower() != "ready":
        return False
    layers = status.get("package_layers")
    if not isinstance(layers, dict):
        return False
    return all(
        str((layers.get(name) or {}).get("status", "")).lower() == "ready"
        for name in ("l1_static", "l3_deep")
        if isinstance(layers.get(name), dict)
    ) and isinstance(layers.get("l1_static"), dict) and isinstance(layers.get("l3_deep"), dict)


def compact_package(status: dict[str, Any]) -> dict[str, Any]:
    layers = status.get("package_layers") if isinstance(status.get("package_layers"), dict) else {}
    return {
        "status": status.get("status"),
        "project_id": status.get("project_id"),
        "track_id": status.get("track_id"),
        "clip_id": status.get("clip_id"),
        "source_path": status.get("source_path"),
        "l1": (layers.get("l1_static") or {}).get("status") if isinstance(layers.get("l1_static"), dict) else None,
        "l3": (layers.get("l3_deep") or {}).get("status") if isinstance(layers.get("l3_deep"), dict) else None,
    }


def prewarm_track_evidence(
    base_url: str,
    focus_track: dict[str, Any],
    request_timeout: float,
    ready_timeout: float,
) -> dict[str, Any]:
    clips = clip_rows(focus_track)
    focus_id = track_id(focus_track)
    focus_clip_id = str(clips[0].get("clip_id") or clips[0].get("id") or "") if clips else ""
    deadline = time.time() + ready_timeout
    latest: dict[str, Any] = {}
    attempts = 0
    while time.time() < deadline:
        attempts += 1
        response = invoke(
            base_url,
            "mix.observe",
            {
                "scope": "full_project_with_focus_track",
                "track_id": focus_id,
                "clip_id": focus_clip_id,
                "focus_hint": {"track_id": focus_id, "source": "goal5_blind_prewarm"},
                "intent": "goal5_blind_readonly_prewarm",
                "projection": "frequency_stereo",
                "feature_keys": [
                    "band_energy_summary",
                    "stereo_relation_summary",
                    "spectrogram_tiles",
                    "acoustic_package_status",
                    "source_identity",
                ],
                "include_raw": False,
                "mutation_barrier": True,
                "no_pending": True,
                "workflow_intent": "observation_only",
            },
            request_timeout,
        )
        latest = result_map(response)
        status = acoustic_status(latest)
        if package_ready(status):
            return {
                "attempts": attempts,
                "observation_id": latest.get("observation_id"),
                "mix_session_id": latest.get("mix_session_id"),
                "package": compact_package(status),
            }
        time.sleep(1.0)
    raise RuntimeError(
        "acoustic package did not become ready: "
        + json.dumps(compact_package(acoustic_status(latest)), ensure_ascii=False)
    )


def executed_tool_route(response: dict[str, Any]) -> list[str]:
    route: list[str] = []
    for item in rows(response.get("executed_kernel_reply")):
        name = str(item.get("tool") or item.get("command_name") or "").strip()
        if name:
            route.append(name)
    return route


def mutation_tools(response: dict[str, Any]) -> list[str]:
    return sorted(
        {
            name
            for name in executed_tool_route(response)
            if any(fragment in name.lower() for fragment in MUTATION_FRAGMENTS)
        }
    )


def assert_diagnostic_contract(response: dict[str, Any], before: dict[str, Any], after: dict[str, Any]) -> None:
    if response.get("needs_confirmation") is True:
        raise AssertionError("diagnostic turn returned needs_confirmation=true")
    if str(response.get("stop_reason", "")).lower() == "needs_confirmation":
        raise AssertionError("diagnostic turn stopped for confirmation")
    if response.get("semantic_action") is not None or response.get("pending_action") is not None:
        raise AssertionError("diagnostic turn created semantic/pending action")
    workflow_data = response.get("workflow_data")
    if isinstance(workflow_data, dict) and workflow_data.get("mutation_performed") is True:
        raise AssertionError("diagnostic turn reported mutation_performed=true")
    bad = mutation_tools(response)
    if bad:
        raise AssertionError(f"diagnostic turn executed mutation tools: {bad}")
    if before != after:
        raise AssertionError("stable project fingerprint changed during diagnostic turn")
    reply = str(response.get("reply") or "")
    lower = reply.lower()
    if "mix_treatment_pending" in lower or any(
        marker in lower
        for marker in ("需要我继续执行吗", "要我继续执行吗", "需要我执行吗", "should i execute", "if you confirm")
    ):
        raise AssertionError("diagnostic reply leaked pending/confirmation language")


def run_case(
    base_url: str,
    case: dict[str, Any],
    case_spec: dict[str, str],
    output: Path,
    request_timeout: float,
    ready_timeout: float,
) -> dict[str, Any]:
    case_id = str(case["case_id"])
    project_path = Path(str(case["project_path"])).resolve()
    invoke(base_url, "project.open", {"file_path": str(project_path)}, request_timeout, confirmed=True)
    state = invoke(base_url, "project.state", {}, request_timeout)
    tracks = project_tracks(state)
    focus_name = case_spec["focus"].lower()
    focus = next((row for row in tracks if track_name(row).lower() == focus_name), None)
    if focus is None:
        raise RuntimeError(f"{case_id} has no focus track {focus_name!r}: {[track_name(row) for row in tracks]}")
    clips = clip_rows(focus)
    if len(tracks) != 6 or len(clips) != 1:
        raise RuntimeError(f"{case_id} is not a complete 6-track/one-focus-clip project")
    # Populate current-material packages for every track.  This is deliberately
    # role-agnostic and identical for all cases: it prevents reused numeric
    # track IDs from making a cross-track case depend on whichever old project
    # happened to be observed most recently.
    prewarm_rows: list[dict[str, Any]] = []
    for row in tracks:
        ready = prewarm_track_evidence(base_url, row, request_timeout, ready_timeout)
        prewarm_rows.append(
            {
                "track_id": track_id(row),
                "track_name": track_name(row),
                **ready,
            }
        )
    prewarm = {"track_count": len(prewarm_rows), "tracks": prewarm_rows}
    before_response = invoke(base_url, "project.state", {}, request_timeout)
    before = stable_project_fingerprint(before_response)
    focus_id = track_id(focus)
    focus_clip_id = str(clips[0].get("clip_id") or clips[0].get("id") or "")
    conversation_id = f"goal5_blind_{case_id}_{int(time.time() * 1000)}"
    response = request_json(
        "POST",
        base_url.rstrip("/") + "/agent/chat",
        {
            "conversation_id": conversation_id,
            "message": case_spec["prompt"],
            "context": {
                "selected_track_id": focus_id,
                "selected_track_name": track_name(focus),
                "selected_clip_id": focus_clip_id,
                "goal5_blind_case_id": case_id,
            },
        },
        request_timeout,
    )
    after_response = invoke(base_url, "project.state", {}, request_timeout)
    after = stable_project_fingerprint(after_response)
    assert_diagnostic_contract(response, before, after)
    write_json_atomic(output / "raw" / f"{case_id}.json", response)
    return {
        "case_id": case_id,
        "project_path": str(project_path),
        "conversation_id": conversation_id,
        "prompt": case_spec["prompt"],
        "focus_track": {"track_id": focus_id, "track_name": track_name(focus), "clip_id": focus_clip_id},
        "prewarm": prewarm,
        "stop_reason": response.get("stop_reason"),
        "needs_confirmation": bool(response.get("needs_confirmation")),
        "workflow": response.get("workflow"),
        "tool_route": executed_tool_route(response),
        "reply": response.get("reply"),
        "zero_mutation": True,
        "project_fingerprint_unchanged": True,
        "raw_response": str(output / "raw" / f"{case_id}.json"),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--fixture-manifest", default="")
    parser.add_argument("--case", action="append", dest="cases", default=[])
    parser.add_argument("--timeout-sec", type=float, default=300.0)
    parser.add_argument("--ready-timeout-sec", type=float, default=240.0)
    parser.add_argument("--output-dir", default="")
    parser.add_argument(
        "--no-rebuild",
        action="store_true",
        help="reuse existing .vit files instead of rebuilding each case immediately before its blind turn",
    )
    args = parser.parse_args()
    try:
        wait_agent(args.agent_http, min(args.timeout_sec, 30.0))
        manifest_path = Path(args.fixture_manifest).resolve() if args.fixture_manifest else default_manifest()
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
        assert_blind_manifest(manifest_path, manifest)
        manifest_cases = {
            str(item.get("case_id")): item
            for item in rows(manifest.get("cases"))
            if str(item.get("case_id", "")) in CASES
        }
        selected = args.cases or list(CASES)
        unknown = [case_id for case_id in selected if case_id not in manifest_cases or case_id not in CASES]
        if unknown:
            raise ValueError(f"unknown case IDs: {unknown}")
        run_id = time.strftime("%Y%m%dT%H%M%SZ", time.gmtime())
        output = (
            Path(args.output_dir).resolve()
            if args.output_dir
            else manifest_path.parent / "blind_runs" / run_id
        )
        results: list[dict[str, Any]] = []
        for case_id in selected:
            rebuild = None
            if not args.no_rebuild:
                rebuild = rebuild_vit_case(
                    args.agent_http,
                    manifest_cases[case_id],
                    args.timeout_sec,
                    args.ready_timeout_sec,
                )
            result = run_case(
                args.agent_http,
                manifest_cases[case_id],
                CASES[case_id],
                output,
                args.timeout_sec,
                args.ready_timeout_sec,
            )
            if rebuild is not None:
                result["immediate_project_rebuild"] = rebuild
            results.append(result)
            print(
                json.dumps(
                    {
                        "status": "case_passed",
                        "case_id": case_id,
                        "tool_route": result["tool_route"],
                        "reply": result["reply"],
                    },
                    ensure_ascii=False,
                ),
                flush=True,
            )
        report = {
            "schema_version": SCHEMA_VERSION,
            "created_at": now_iso(),
            "status": "passed",
            "blindness": {
                "fixture_manifest": str(manifest_path),
                "sealed_truth_opened": False,
                "sealed_truth_sent_to_agent": False,
                "case_names_semantically_opaque": True,
            },
            "fixture_set_id": manifest.get("set_id"),
            "case_count": len(results),
            "cases": results,
        }
        report_path = output / "blind_run_report.json"
        write_json_atomic(report_path, report)
        print(json.dumps({"status": "passed", "report": str(report_path), "case_count": len(results)}, ensure_ascii=False))
        return 0
    except Exception as exc:  # noqa: BLE001 - top-level smoke diagnostic.
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
