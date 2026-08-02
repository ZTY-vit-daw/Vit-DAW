#!/usr/bin/env python3
"""Live smoke for ordinary-Agent plugin recommendation and selection.

The smoke never approves a plugin load. It verifies:

  semantic listening request without selected plugin -> LLM recommendation ->
  one-primary/up-to-two-alternative selection -> exact governed load proposal ->
  cancellation with no project mutation.
"""

from __future__ import annotations

import argparse
import json
import pathlib
import time
import urllib.request
from typing import Any


def request_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def rows(value: Any) -> list[dict]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def recursively_contains(value: Any, needles: set[str]) -> bool:
    if isinstance(value, dict):
        return any(recursively_contains(key, needles) or recursively_contains(item, needles) for key, item in value.items())
    if isinstance(value, list):
        return any(recursively_contains(item, needles) for item in value)
    text = str(value).lower()
    return any(needle in text for needle in needles)


def first_selection_interaction(response: dict) -> dict:
    for item in rows(response.get("interaction_requests")):
        if item.get("workflow") == "plugin_recommendation_selection":
            return item
    raise AssertionError("recommendation response omitted plugin selection interaction")


def assert_no_mutation(response: dict, stage: str) -> None:
    data = response.get("workflow_data") if isinstance(response.get("workflow_data"), dict) else {}
    if data.get("mutation_performed") is True:
        raise AssertionError(f"{stage} reported mutation_performed=true")
    if recursively_contains(
        response.get("executed_kernel_reply", []),
        {"rack_add_node", "instantiate_plugin", "set_plugin_param", "apply_eq_edits", "apply_control"},
    ):
        raise AssertionError(f"{stage} executed a project/plugin mutation")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--track-id", default="1007")
    parser.add_argument("--track-name", default="Current Track")
    parser.add_argument("--message", default="减少一些浑浊")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    base = args.agent_http.rstrip("/")
    output_dir = pathlib.Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    health = request_json("GET", base + "/health", None, 5.0)
    if str(health.get("status", "")).lower() not in {"ok", "ready"}:
        raise AssertionError(f"agent health is not ready: {health}")

    conversation_id = f"plugin_recommendation_smoke_{int(time.time())}"
    context = {
        "selected_track_id": str(args.track_id),
        "selected_track_name": args.track_name,
        # Deliberately omit selected_plugin_id: this is the no-plugin handoff.
    }
    recommendation = request_json(
        "POST",
        base + "/agent/chat",
        {"conversation_id": conversation_id, "message": args.message, "context": context},
        args.timeout_sec,
    )
    (output_dir / "01_recommendation.json").write_text(
        json.dumps(recommendation, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    data = recommendation.get("workflow_data") if isinstance(recommendation.get("workflow_data"), dict) else {}
    if recommendation.get("workflow") != "plugin_recommendation_selection":
        raise AssertionError(f"wrong recommendation workflow: {recommendation.get('workflow')!r}")
    if data.get("schema_version") != "plugin_recommendation_selection.v1" or data.get("status") != "awaiting_selection":
        raise AssertionError(f"wrong recommendation state: {data}")
    choices = rows(data.get("recommendations"))
    if not 1 <= len(choices) <= 3:
        raise AssertionError(f"expected 1-3 recommendations, got {len(choices)}")
    if choices[0].get("role") != "recommended" or any(row.get("role") != "alternative" for row in choices[1:]):
        raise AssertionError(f"recommendation roles are invalid: {choices}")
    if any(not row.get("plugin_path") or not row.get("candidate_key") or not row.get("reason") for row in choices):
        raise AssertionError(f"recommendations omitted exact hard facts or LLM reasons: {choices}")
    if data.get("selection_performed") is True or data.get("load_proposed") is True:
        raise AssertionError(f"recommendation turn selected/proposed a plugin prematurely: {data}")
    if recursively_contains(recommendation, {"learn_project_profile", "plugin_profile", "spal"}):
        raise AssertionError("recommendation re-entered forbidden profile/learn/SPAL path")
    assert_no_mutation(recommendation, "recommendation")

    interaction = first_selection_interaction(recommendation)
    actions = rows(interaction.get("actions"))
    select_actions = [action for action in actions if str(action.get("id", "")).startswith("select_")]
    if len(select_actions) != len(choices) or not select_actions[0].get("recommended"):
        raise AssertionError(f"selection actions do not match recommendations: {actions}")

    selected = request_json(
        "POST",
        base + "/agent/interaction/respond",
        {
            "interaction_id": interaction["id"],
            "action_id": select_actions[0]["id"],
            "decision": select_actions[0]["id"],
            "payload": interaction.get("payload", {}),
        },
        args.timeout_sec,
    )
    (output_dir / "02_load_proposal.json").write_text(
        json.dumps(selected, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    selected_data = selected.get("workflow_data") if isinstance(selected.get("workflow_data"), dict) else {}
    if selected.get("needs_confirmation") is not True or not selected.get("plan_id"):
        raise AssertionError(f"selection did not create governed load confirmation: {selected}")
    if selected.get("workflow") != "plugin_grabber_load_and_get_params":
        raise AssertionError(f"selection entered wrong load workflow: {selected.get('workflow')!r}")
    selected_candidate = selected_data.get("selected_candidate") if isinstance(selected_data.get("selected_candidate"), dict) else {}
    if selected_candidate.get("candidate_key") != choices[0].get("candidate_key"):
        raise AssertionError(f"load proposal changed selected candidate: {selected_candidate}")
    commands = rows(selected.get("commands"))
    if len(commands) != 1:
        raise AssertionError(f"load proposal should contain exactly one command: {commands}")
    command = commands[0].get("command") if isinstance(commands[0].get("command"), dict) else {}
    if command.get("cmd") != "rack_add_node" or str(command.get("track_id")) != str(args.track_id):
        raise AssertionError(f"load command target is invalid: {command}")
    if command.get("plugin_path") != choices[0].get("plugin_path"):
        raise AssertionError(f"load command changed exact plugin path: {command}")
    if choices[0].get("identifier") and command.get("plugin_identifier") != choices[0].get("identifier"):
        raise AssertionError(f"load command omitted exact plugin_identifier: {command}")
    assert_no_mutation(selected, "load proposal")

    cancelled = request_json(
        "POST",
        base + "/agent/confirm",
        {"plan_id": selected["plan_id"], "decision": "cancel"},
        args.timeout_sec,
    )
    (output_dir / "03_cancelled.json").write_text(
        json.dumps(cancelled, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    assert_no_mutation(cancelled, "cancellation")

    summary = {
        "status": "pass",
        "conversation_id": conversation_id,
        "processor_type": data.get("processor_type"),
        "recommendation_count": len(choices),
        "recommended": choices[0],
        "load_plan_id": selected.get("plan_id"),
        "load_cancelled": True,
        "mutation_performed": False,
    }
    (output_dir / "summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
