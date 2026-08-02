#!/usr/bin/env python3
"""Zero-mutation live smoke for goal 4 treatment and instance arbitration.

Exercises ordinary-Agent discussion, one-primary/two-alternative treatment
selection, the target-3 recommendation handoff, exact governed load proposal,
and cancellation. Existing-instance qualification and stale-state behavior are
covered by Go tests because this smoke must not alter the user's open project.
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
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method
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


def data(response: dict) -> dict:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def interaction(response: dict, workflow: str) -> dict:
    for item in rows(response.get("interaction_requests")):
        if item.get("workflow") == workflow:
            return item
    raise AssertionError(f"response omitted {workflow} interaction")


def write(output: pathlib.Path, name: str, value: dict) -> None:
    (output / name).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def mutation_tools(response: dict) -> list[str]:
    forbidden = ("rack_add_node", "instantiate_plugin", "set_plugin_param", "apply_eq_edits", "apply_control", "set_params_batch")
    found: list[str] = []
    for item in rows(response.get("executed_kernel_reply")):
        # Inspect only this turn's executed command identity. Observation
        # results may contain durable project-history text mentioning an old
        # rack_add_node; that is evidence, not a mutation in this response.
        text = " ".join(str(item.get(key, "")) for key in ("tool", "command_name")).lower()
        found.extend(name for name in forbidden if name in text)
    return sorted(set(found))


def assert_zero_mutation(response: dict, stage: str) -> None:
    if data(response).get("mutation_performed") is True:
        raise AssertionError(f"{stage} reported mutation_performed=true")
    bad = mutation_tools(response)
    if bad:
        raise AssertionError(f"{stage} executed mutation tools: {bad}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--track-id", default="1007")
    parser.add_argument("--track-name", default="Vocal")
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    base = args.agent_http.rstrip("/")
    output = pathlib.Path(args.output_dir)
    output.mkdir(parents=True, exist_ok=True)
    health = request_json("GET", base + "/health", None, 5.0)
    if str(health.get("status", "")).lower() not in {"ok", "ready"}:
        raise AssertionError(f"agent is not healthy: {health}")

    context = {"selected_track_id": str(args.track_id), "selected_track_name": args.track_name}

    discussion = request_json(
        "POST", base + "/agent/chat",
        {"conversation_id": f"goal4_discussion_{int(time.time())}", "message": "为什么人声听起来靠后？", "context": context},
        args.timeout_sec,
    )
    write(output, "01_discussion.json", discussion)
    if discussion.get("workflow") == "semantic_treatment_strategy" or discussion.get("needs_confirmation") is True:
        raise AssertionError("discussion-only turn created a treatment strategy or confirmation")
    assert_zero_mutation(discussion, "discussion")

    conversation_id = f"goal4_strategy_{int(time.time())}"
    strategy = request_json(
        "POST", base + "/agent/chat",
        {"conversation_id": conversation_id, "message": "让人声更靠前一些", "context": context},
        args.timeout_sec,
    )
    write(output, "02_strategy.json", strategy)
    strategy_data = data(strategy)
    if strategy.get("workflow") != "semantic_treatment_strategy":
        raise AssertionError(f"wrong strategy workflow: {strategy.get('workflow')!r}")
    if strategy_data.get("schema_version") != "semantic_treatment_strategy.v1" or strategy_data.get("status") != "awaiting_selection":
        raise AssertionError(f"wrong strategy contract: {strategy_data}")
    choices = rows(strategy_data.get("choices"))
    if strategy_data.get("decision_mode") != "choice_required" or len(choices) != 3:
        raise AssertionError(f"expected one primary and two alternatives: {choices}")
    if choices[0].get("role") != "recommended" or any(item.get("role") != "alternative" for item in choices[1:]):
        raise AssertionError(f"invalid strategy roles: {choices}")
    signatures = {(item.get("target_mode"), item.get("processor_type"), item.get("instance_key")) for item in choices}
    if len(signatures) != 3 or any(not item.get("reason") or not item.get("expected_effect") for item in choices):
        raise AssertionError(f"strategies are duplicated or incomplete: {choices}")
    assert_zero_mutation(strategy, "strategy")

    strategy_interaction = interaction(strategy, "semantic_treatment_strategy")
    action_by_choice = {
        str(action.get("id", "")).removeprefix("select_"): action
        for action in rows(strategy_interaction.get("actions"))
        if str(action.get("id", "")).startswith("select_")
    }
    selected_choice = next(
        (item for item in choices if item.get("target_mode") == "load_required" and item.get("processor_type") == "eq"),
        next((item for item in choices if item.get("target_mode") == "load_required"), None),
    )
    if not selected_choice or selected_choice.get("choice_key") not in action_by_choice:
        raise AssertionError(f"strategy did not expose a load-required choice: {choices}")
    selected_action = action_by_choice[selected_choice["choice_key"]]
    recommendation = request_json(
        "POST", base + "/agent/interaction/respond",
        {"interaction_id": strategy_interaction["id"], "decision": selected_action["id"], "action_id": selected_action["id"], "payload": strategy_interaction.get("payload", {})},
        args.timeout_sec,
    )
    write(output, "03_plugin_recommendation.json", recommendation)
    recommendation_data = data(recommendation)
    if recommendation.get("workflow") != "plugin_recommendation_selection" or recommendation_data.get("status") != "awaiting_selection":
        raise AssertionError(f"strategy did not enter target-3 recommendation: {recommendation}")
    candidates = rows(recommendation_data.get("recommendations"))
    if not 1 <= len(candidates) <= 3 or candidates[0].get("role") != "recommended":
        raise AssertionError(f"invalid plugin recommendation: {candidates}")
    assert_zero_mutation(recommendation, "plugin recommendation")

    recommendation_interaction = interaction(recommendation, "plugin_recommendation_selection")
    select_actions = [
        item for item in rows(recommendation_interaction.get("actions"))
        if str(item.get("id", "")).startswith("select_")
    ]
    if not select_actions:
        raise AssertionError("plugin recommendation omitted selection action")
    load = request_json(
        "POST", base + "/agent/interaction/respond",
        {"interaction_id": recommendation_interaction["id"], "decision": select_actions[0]["id"], "action_id": select_actions[0]["id"], "payload": recommendation_interaction.get("payload", {})},
        args.timeout_sec,
    )
    write(output, "04_load_confirmation.json", load)
    if load.get("workflow") != "plugin_grabber_load_and_get_params" or load.get("needs_confirmation") is not True or not load.get("plan_id"):
        raise AssertionError(f"plugin selection did not create governed load confirmation: {load}")
    commands = rows(load.get("commands"))
    command = commands[0].get("command") if commands and isinstance(commands[0].get("command"), dict) else {}
    if command.get("cmd") != "rack_add_node" or str(command.get("track_id")) != str(args.track_id):
        raise AssertionError(f"load confirmation changed exact target: {command}")
    assert_zero_mutation(load, "load confirmation")

    cancelled = request_json(
        "POST", base + "/agent/confirm", {"plan_id": load["plan_id"], "decision": "cancel"}, args.timeout_sec
    )
    write(output, "05_cancelled.json", cancelled)
    assert_zero_mutation(cancelled, "cancel")

    summary = {
        "status": "pass",
        "conversation_id": conversation_id,
        "strategy_count": len(choices),
        "recommended_strategy": choices[0],
        "selected_strategy": selected_choice,
        "plugin_candidate_count": len(candidates),
        "load_plan_cancelled": True,
        "mutation_performed": False,
    }
    write(output, "summary.json", summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
