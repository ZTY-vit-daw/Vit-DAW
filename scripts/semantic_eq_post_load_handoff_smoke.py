#!/usr/bin/env python3
"""Live load -> qualification -> governed EQ proposal smoke for goal 4.

This smoke intentionally authorizes one plugin load on an existing disposable
track. By default it cancels the EQ proposal; with ``--execute-eq`` it verifies
and rolls back one confirmed EQ operation. It then removes only the exact test
plugin instance, without depending on global project-undo ordering.
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
    request = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json; charset=utf-8"}, method=method
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        value = json.loads(response.read().decode("utf-8", errors="replace"))
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def rows(value: Any) -> list[dict]:
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def workflow_data(response: dict) -> dict:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def write(output: pathlib.Path, name: str, value: dict) -> None:
    (output / name).write_text(json.dumps(value, ensure_ascii=False, indent=2), encoding="utf-8")


def interaction(response: dict, workflow: str) -> dict:
    for item in rows(response.get("interaction_requests")):
        if item.get("workflow") == workflow:
            return item
    raise AssertionError(f"response omitted {workflow} interaction")


def preferred_static_eq_action(interaction_request: dict) -> dict:
    """Prefer the known stable generic-topology fixture over LLM rank order."""
    actions = [
        item for item in rows(interaction_request.get("actions"))
        if str(item.get("id", "")).startswith("select_")
    ]
    if not actions:
        raise AssertionError("EQ recommendation omitted selection action")
    for action in actions:
        text = json.dumps(action, ensure_ascii=False).lower()
        if "pro-q 3" in text or "pro q 3" in text or "fabfilter pro-q" in text:
            return action
    return actions[0]


def mutation_names(response: dict) -> set[str]:
    names: set[str] = set()
    for item in rows(response.get("executed_kernel_reply")):
        for key in ("tool", "command_name"):
            value = str(item.get(key, "")).lower().strip()
            if value:
                names.add(value)
        result = item.get("result")
        if isinstance(result, dict):
            text = json.dumps(result, ensure_ascii=False).lower()
            for name in ("rack_add_node", "plugin_grabber_apply_eq_edits", "set_plugin_param", "plugin.set_params_batch"):
                if name in text:
                    names.add(name)
    return names


def plugin_ids(ui_state: dict, track_id: str) -> set[str]:
    for track in rows(ui_state.get("tracks")):
        if str(track.get("track_id", track.get("id", ""))) != str(track_id):
            continue
        plugin_rows = rows(track.get("plugins"))
        rack = track.get("rack") if isinstance(track.get("rack"), dict) else {}
        plugin_rows.extend(rows(rack.get("nodes")))
        return {
            str(plugin.get("plugin_id", plugin.get("item_id", plugin.get("id", ""))))
            for plugin in plugin_rows
            if str(plugin.get("plugin_id", plugin.get("item_id", plugin.get("id", ""))))
        }
    return set()


def stable_ui_state(base: str, timeout: float) -> dict:
    """Wait for two identical track/plugin snapshots after asynchronous resync."""
    deadline = time.monotonic() + min(timeout, 15.0)
    previous: tuple | None = None
    latest: dict = {}
    while time.monotonic() < deadline:
        latest = request_json("GET", base + "/agent/ui/state", None, 15.0)
        signature = tuple(
            (
                str(track.get("track_id", track.get("id", ""))),
                tuple(sorted(plugin_ids(latest, str(track.get("track_id", track.get("id", "")))))),
            )
            for track in rows(latest.get("tracks"))
        )
        if signature == previous:
            return latest
        previous = signature
        time.sleep(0.5)
    return latest


def wait_for_plugin_graph(base: str, track_id: str, expected: set[str], timeout: float) -> dict:
    deadline = time.monotonic() + min(timeout, 20.0)
    latest: dict = {}
    while time.monotonic() < deadline:
        latest = request_json("GET", base + "/agent/ui/state", None, 15.0)
        if plugin_ids(latest, track_id) == expected:
            return latest
        time.sleep(0.5)
    return latest


def delete_smoke_track(base: str, track_id: str, timeout: float) -> dict:
    return request_json(
        "POST", base + "/agent/invoke",
        {"tool": "track.delete", "args": {"track_id": track_id}, "confirmed": True, "source": "semantic_eq_post_load_handoff_smoke_track_cleanup"},
        timeout,
    )


def delete_loaded_plugin(base: str, track_id: str, plugin_id: str, timeout: float, source: str) -> dict:
    return request_json(
        "POST", base + "/agent/invoke",
        {
            "tool": "delete_plugin",
            "args": {"track_id": track_id, "plugin_id": plugin_id, "plugin_item_id": plugin_id},
            "confirmed": True,
            "source": source,
        },
        timeout,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--track-id", default="")
    parser.add_argument("--track-name", default="Goal4 Smoke Vocal")
    parser.add_argument("--message", default="减少一些浑浊")
    parser.add_argument(
        "--execute-eq",
        action="store_true",
        help="confirm the frozen EQ proposal, verify its readback, and undo it before removing the plugin",
    )
    parser.add_argument("--timeout-sec", type=float, default=240.0)
    parser.add_argument("--output-dir", required=True)
    args = parser.parse_args()

    base = args.agent_http.rstrip("/")
    output = pathlib.Path(args.output_dir)
    output.mkdir(parents=True, exist_ok=True)
    before_state = stable_ui_state(base, args.timeout_sec)
    track_id = str(args.track_id).strip()
    visible_tracks = rows(before_state.get("tracks"))
    visible_track_ids = {str(track.get("track_id", track.get("id", ""))) for track in visible_tracks}
    track_created = False
    if not track_id and visible_tracks:
        track_id = str(visible_tracks[0].get("track_id", visible_tracks[0].get("id", ""))).strip()
        context_name = str(visible_tracks[0].get("name", args.track_name)).strip()
    else:
        context_name = args.track_name
    if not track_id or track_id not in visible_track_ids:
        setup = request_json(
            "POST", base + "/agent/invoke",
            {"tool": "daw.invoke", "args": {"cmd": "add_audio_track", "name": args.track_name}, "confirmed": True, "source": "semantic_eq_post_load_handoff_smoke_setup"},
            args.timeout_sec,
        )
        write(output, "00_track_created.json", setup)
        result = setup.get("result") if isinstance(setup.get("result"), dict) else {}
        track_id = str(result.get("track_id", "")).strip()
        if str(setup.get("status", "")).lower() != "ok" or not track_id:
            raise AssertionError(f"could not create disposable smoke track: {setup}")
        track_created = True
        context_name = args.track_name
    before_plugins = plugin_ids(stable_ui_state(base, args.timeout_sec), track_id)
    conversation_id = f"goal4_post_load_{int(time.time())}"
    context = {"selected_track_id": track_id, "selected_track_name": context_name}

    loaded = False
    operation_ref = ""
    try:
        recommendation = request_json(
            "POST", base + "/agent/chat",
            {"conversation_id": conversation_id, "message": args.message, "context": context},
            args.timeout_sec,
        )
        write(output, "01_eq_recommendation.json", recommendation)
        rec_data = workflow_data(recommendation)
        if recommendation.get("workflow") != "plugin_recommendation_selection" or rec_data.get("post_load_planner") != "semantic_eq":
            raise AssertionError(f"EQ request did not create recommendation with post-load handoff: {recommendation}")
        rec_interaction = interaction(recommendation, "plugin_recommendation_selection")
        selected_action = preferred_static_eq_action(rec_interaction)

        load_proposal = request_json(
            "POST", base + "/agent/interaction/respond",
            {"interaction_id": rec_interaction["id"], "decision": selected_action["id"], "action_id": selected_action["id"], "payload": rec_interaction.get("payload", {})},
            args.timeout_sec,
        )
        write(output, "02_load_proposal.json", load_proposal)
        if load_proposal.get("workflow") != "plugin_grabber_load_and_get_params" or load_proposal.get("needs_confirmation") is not True:
            raise AssertionError(f"plugin selection did not create governed load proposal: {load_proposal}")

        handoff = request_json(
            "POST", base + "/agent/confirm",
            {"plan_id": load_proposal["plan_id"], "decision": "approve"},
            args.timeout_sec,
        )
        write(output, "03_loaded_qualified_eq_proposal.json", handoff)
        handoff_data = workflow_data(handoff)
        qualification = handoff_data.get("post_load_qualification") if isinstance(handoff_data.get("post_load_qualification"), dict) else {}
        names = mutation_names(handoff)
        loaded = "rack_add_node" in names
        if handoff.get("workflow") != "capability_runtime_v1" or handoff.get("needs_confirmation") is not True:
            raise AssertionError(f"load did not hand off to governed EQ proposal: {handoff}")
        if qualification.get("status") != "qualified" or qualification.get("processor_type") != "eq" or not qualification.get("plugin_id"):
            raise AssertionError(f"actual loaded instance was not qualified: {qualification}")
        if not loaded:
            raise AssertionError(f"load receipt omitted rack_add_node: {names}")
        if any(name in names for name in ("plugin_grabber_apply_eq_edits", "set_plugin_param", "plugin.set_params_batch")):
            raise AssertionError(f"load authorization leaked into EQ parameter writes: {names}")

        if args.execute_eq:
            required = ("session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash")
            missing = [key for key in required if handoff_data.get(key) in (None, "")]
            if missing:
                raise AssertionError(f"EQ proposal omitted exact confirmation bindings: {missing}")
            confirmation_context = dict(context)
            confirmation_context.update(
                {
                    "selected_plugin_track_id": track_id,
                    "selected_plugin_id": qualification["plugin_id"],
                    "selected_plugin_name": qualification.get("plugin_name", ""),
                    "capability_session_id": handoff_data["session_id"],
                    "capability_id": handoff_data["capability_id"],
                    "proposal_id": handoff_data["proposal_id"],
                    "proposal_revision": handoff_data["proposal_revision"],
                    "action_set_hash": handoff_data["action_set_hash"],
                    "project_cut_hash": handoff_data["project_cut_hash"],
                }
            )
            execution = request_json(
                "POST", base + "/agent/chat",
                {"conversation_id": conversation_id, "message": "确认执行这个方案", "context": confirmation_context},
                args.timeout_sec,
            )
            write(output, "04_eq_executed.json", execution)
            execution_data = workflow_data(execution)
            details = execution_data.get("result") if isinstance(execution_data.get("result"), dict) else {}
            inner = details.get("result") if isinstance(details.get("result"), dict) else {}
            atom_results = rows(details.get("atom_results"))
            expected_atoms = rows((handoff_data.get("semantic_action") or {}).get("eq_plan", {}).get("atoms"))
            if details.get("structural_readback") != "pass" or inner.get("status") not in {"exact", "quantized"}:
                raise AssertionError(f"EQ execution did not pass structural readback: {details}")
            if len(atom_results) != len(expected_atoms) or any(
                row.get("status") not in {"exact", "quantized"} or not rows(row.get("actual_readback"))
                for row in atom_results
            ):
                raise AssertionError(f"EQ execution omitted per-atom status/readback: {atom_results}")
            operation_ref = str(inner.get("operation_ref", "")).strip()
            if not operation_ref or not isinstance(inner.get("rollback"), dict) or inner["rollback"].get("available") is not True:
                raise AssertionError(f"EQ execution omitted rollback contract: {inner}")
            verification = execution_data.get("verification") if isinstance(execution_data.get("verification"), dict) else {}
            if verification.get("structural") != "pass" or verification.get("acoustic") not in {"pass", "unavailable", "inconclusive"} or verification.get("user_acceptance") != "unknown":
                raise AssertionError(f"EQ execution overclaimed or omitted verification: {verification}")
            restored = request_json(
                "POST", base + "/agent/invoke",
                {
                    "tool": "plugin_grabber.apply_eq_edits",
                    "args": {
                        "track_id": track_id,
                        "plugin_id": qualification["plugin_id"],
                        "atomic": True,
                        "edits": [{"action": "undo", "operation_ref": operation_ref}],
                    },
                    "confirmed": True,
                    "source": "semantic_eq_post_load_handoff_smoke_eq_cleanup",
                },
                args.timeout_sec,
            )
            write(output, "05_eq_undone.json", restored)
            if str(restored.get("status", "")).lower() != "ok":
                raise AssertionError(f"EQ operation cleanup failed: {restored}")
            operation_ref = ""
        else:
            proposal_interaction = interaction(handoff, "capability_runtime_v1")
            cancelled = request_json(
                "POST", base + "/agent/interaction/respond",
                {"interaction_id": proposal_interaction["id"], "decision": "cancel", "action_id": "cancel", "payload": proposal_interaction.get("payload", {})},
                args.timeout_sec,
            )
            write(output, "04_eq_proposal_cancelled.json", cancelled)
            if any(name in mutation_names(cancelled) for name in ("plugin_grabber_apply_eq_edits", "set_plugin_param", "plugin.set_params_batch")):
                raise AssertionError("cancelling EQ proposal wrote parameters")

        plugin_cleanup = delete_loaded_plugin(
            base,
            track_id,
            str(qualification["plugin_id"]),
            args.timeout_sec,
            "semantic_eq_post_load_handoff_smoke_plugin_cleanup",
        )
        write(output, "06_plugin_instance_removed.json" if args.execute_eq else "05_plugin_instance_removed.json", plugin_cleanup)
        if str(plugin_cleanup.get("status", "")).lower() != "ok":
            raise AssertionError(f"plugin cleanup failed: {plugin_cleanup}")
        loaded = False

        after_state = wait_for_plugin_graph(base, track_id, before_plugins, args.timeout_sec)
        after_plugins = plugin_ids(after_state, track_id)
        if after_plugins != before_plugins:
            raise AssertionError(f"plugin graph was not restored: before={before_plugins} after={after_plugins}")

        if track_created:
            track_undo = delete_smoke_track(base, track_id, args.timeout_sec)
            write(output, "07_smoke_track_undone.json" if args.execute_eq else "06_smoke_track_undone.json", track_undo)
            if str(track_undo.get("status", "")).lower() != "ok":
                raise AssertionError(f"smoke-track cleanup delete failed: {track_undo}")
            track_created = False

        summary = {
            "status": "pass",
            "conversation_id": conversation_id,
            "track_id": track_id,
            "loaded_plugin_id": qualification.get("plugin_id"),
            "topology_generation": qualification.get("topology_generation"),
            "eq_parameter_writes": len(expected_atoms) if args.execute_eq else 0,
            "eq_proposal_cancelled": not args.execute_eq,
            "eq_execution_verified": args.execute_eq,
            "eq_operation_undone": args.execute_eq,
            "plugin_load_reverted": True,
            "plugin_cleanup_method": "exact_delete",
            "plugin_graph_restored": True,
            "disposable_track_restored": True,
        }
        write(output, "summary.json", summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return 0
    finally:
        if operation_ref:
            try:
                emergency_eq = request_json(
                    "POST", base + "/agent/invoke",
                    {
                        "tool": "plugin_grabber.apply_eq_edits",
                        "args": {"track_id": track_id, "plugin_id": qualification.get("plugin_id", ""), "atomic": True, "edits": [{"action": "undo", "operation_ref": operation_ref}]},
                        "confirmed": True,
                        "source": "semantic_eq_post_load_handoff_smoke_emergency_eq_cleanup",
                    },
                    30.0,
                )
                write(output, "emergency_eq_cleanup.json", emergency_eq)
            except Exception as exc:  # pragma: no cover - live cleanup diagnostic
                (output / "emergency_eq_cleanup_error.txt").write_text(str(exc), encoding="utf-8")
        if loaded:
            try:
                emergency = delete_loaded_plugin(
                    base,
                    track_id,
                    str(qualification.get("plugin_id", "")),
                    30.0,
                    "semantic_eq_post_load_handoff_smoke_emergency_plugin_cleanup",
                )
                write(output, "emergency_cleanup.json", emergency)
            except Exception as exc:  # pragma: no cover - live cleanup diagnostic
                (output / "emergency_cleanup_error.txt").write_text(str(exc), encoding="utf-8")
        if track_created:
            try:
                emergency_track = delete_smoke_track(base, track_id, 30.0)
                write(output, "emergency_track_cleanup.json", emergency_track)
            except Exception as exc:  # pragma: no cover - live cleanup diagnostic
                (output / "emergency_track_cleanup_error.txt").write_text(str(exc), encoding="utf-8")


if __name__ == "__main__":
    raise SystemExit(main())
