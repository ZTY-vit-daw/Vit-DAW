#!/usr/bin/env python3
"""Real ordinary-Agent semantic EQ smoke.

The smoke intentionally exercises chat, not direct parameter control:

  ordinary Agent -> typed semantic_action -> read-only EQ preflight ->
  frozen Proposal -> exact conversational approval -> governed
  plugin_grabber.apply_eq_edits -> fresh readback -> verification.

The direct invoke endpoint is used only to undo a completed smoke mutation so
the next independent natural-language case starts from the same plug-in state.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.request
from typing import Any


CAPABILITY_ID = "agent.effect.eq_control.v0"
AGENT_HTTP_DEFAULT = "http://127.0.0.1:7878"

CASES = [
    {
        "id": "discussion_only",
        "message": "为什么听起来浑？",
        "kind": "discussion",
    },
    {
        "id": "reduce_mud",
        "message": "减少一些浑浊",
        "kind": "action",
        "min_atoms": 1,
    },
    {
        "id": "raise_highs",
        "message": "提高一些高频",
        "kind": "action",
        "min_atoms": 1,
        "allowed_shapes": {"high_shelf", "bell"},
    },
    {
        "id": "bright_not_harsh",
        "message": "更亮但不要更刺耳",
        "kind": "action",
        "min_atoms": 2,
        "require_negative_constraint": True,
    },
]


def request_json(method: str, url: str, payload: dict | None, timeout: float) -> dict:
    data = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json; charset=utf-8"},
        method=method,
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = resp.read().decode("utf-8", errors="replace")
    value = json.loads(body)
    if not isinstance(value, dict):
        raise RuntimeError(f"{url} returned non-object JSON")
    return value


def wait_agent(base: str, timeout: float) -> None:
    deadline = time.time() + timeout
    last_error: Exception | None = None
    while time.time() < deadline:
        try:
            health = request_json("GET", base.rstrip("/") + "/health", None, 3)
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
        except Exception as exc:  # pragma: no cover - live diagnostic path
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"agent not ready at {base}: {last_error}")


def chat(base: str, conversation_id: str, message: str, context: dict, timeout: float) -> dict:
    return request_json(
        "POST",
        base.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation_id, "message": message, "context": context},
        timeout,
    )


def invoke(base: str, tool: str, args: dict, timeout: float) -> dict:
    return request_json(
        "POST",
        base.rstrip("/") + "/agent/invoke",
        {"tool": tool, "args": args, "confirmed": True, "source": "semantic_eq_smoke_cleanup"},
        timeout,
    )


def workflow_data(response: dict) -> dict:
    value = response.get("workflow_data")
    return value if isinstance(value, dict) else {}


def rows(value: Any) -> list[dict]:
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, dict)]


def recursively_contains(value: Any, needles: set[str]) -> bool:
    if isinstance(value, dict):
        return any(recursively_contains(k, needles) or recursively_contains(v, needles) for k, v in value.items())
    if isinstance(value, list):
        return any(recursively_contains(item, needles) for item in value)
    text = str(value).lower()
    return any(needle in text for needle in needles)


def has_mutation_evidence(response: dict) -> bool:
    return recursively_contains(
        response.get("executed_kernel_reply", []),
        {"apply_eq_edits", "set_eq_point", "set_plugin_param", "apply_control", "plugin.set_params_batch"},
    )


def executed_tool_names(response: dict) -> set[str]:
    names: set[str] = set()
    for row in rows(response.get("executed_kernel_reply")):
        for key in ("tool", "command_name"):
            value = str(row.get(key, "")).strip().lower()
            if value:
                names.add(value)
    return names


def assert_no_forbidden_semantic_eq_tools(response: dict) -> None:
    forbidden_fragments = {
        "learn_project_profile",
        "apply_control",
        "spal.",
        "spal_",
        "plugin.load",
        "plugin_load",
    }
    bad = sorted(
        name
        for name in executed_tool_names(response)
        if any(fragment in name for fragment in forbidden_fragments)
    )
    if bad:
        raise AssertionError(f"ordinary semantic EQ used forbidden tool path: {bad}")


def exact_confirmation_context(proposal_response: dict, base_context: dict) -> dict:
    data = workflow_data(proposal_response)
    required = [
        "session_id",
        "capability_id",
        "proposal_id",
        "proposal_revision",
        "action_set_hash",
        "project_cut_hash",
    ]
    missing = [key for key in required if data.get(key) in (None, "")]
    if missing:
        raise AssertionError(f"proposal response omitted exact binding fields: {missing}")
    context = dict(base_context)
    context.update(
        {
            "capability_session_id": data["session_id"],
            "capability_id": data["capability_id"],
            "proposal_id": data["proposal_id"],
            "proposal_revision": data["proposal_revision"],
            "action_set_hash": data["action_set_hash"],
            "project_cut_hash": data["project_cut_hash"],
        }
    )
    return context


def validate_discussion(response: dict) -> None:
    data = workflow_data(response)
    if response.get("needs_confirmation") is True or response.get("proposal_presentation") is not None:
        raise AssertionError("discussion-only request created a Proposal/confirmation")
    if response.get("plan_id") not in (None, ""):
        raise AssertionError(f"discussion-only request returned plan_id={response.get('plan_id')!r}")
    if data.get("capability_id") == CAPABILITY_ID or data.get("session_id"):
        raise AssertionError(f"discussion-only request created semantic EQ session state: {data}")
    if has_mutation_evidence(response):
        raise AssertionError("discussion-only request mutated EQ")


def validate_typed_plan(case: dict, response: dict) -> tuple[dict, list[dict]]:
    data = workflow_data(response)
    if response.get("needs_confirmation") is not True:
        raise AssertionError(
            f"actionable request did not create confirmation: status={response.get('goal_status')!r} "
            f"reply={str(response.get('reply', ''))[:400]!r} error={response.get('error')!r}"
        )
    if data.get("capability_id") != CAPABILITY_ID or data.get("stage") != "proposal":
        raise AssertionError(f"wrong proposal owner/stage: {data}")
    if int(data.get("writes_performed", -1)) != 0 or has_mutation_evidence(response):
        raise AssertionError("proposal turn wrote parameters before confirmation")

    action = data.get("semantic_action")
    if not isinstance(action, dict):
        raise AssertionError("proposal omitted typed semantic_action")
    if action.get("schema_version") != "semantic_effect_action.v1" or action.get("payload_schema") != "semantic_effect.eq_plan.v1":
        raise AssertionError(f"wrong semantic schemas: {action}")
    eq_plan = action.get("eq_plan")
    atoms = rows(eq_plan.get("atoms") if isinstance(eq_plan, dict) else None)
    if len(atoms) < int(case.get("min_atoms", 1)) or len(atoms) > 3:
        raise AssertionError(f"wrong atom count for {case['id']}: {len(atoms)}")
    for atom in atoms:
        if atom.get("action") != "upsert":
            raise AssertionError(f"smoke atom is not concrete upsert: {atom}")
        if atom.get("shape") not in {"bell", "low_shelf", "high_shelf", "low_cut", "high_cut"}:
            raise AssertionError(f"unsupported/missing atom shape: {atom}")
        if not isinstance(atom.get("frequency_hz"), (int, float)):
            raise AssertionError(f"atom omitted concrete frequency: {atom}")
        if atom.get("shape") in {"bell", "low_shelf", "high_shelf"} and not isinstance(atom.get("gain_db"), (int, float)):
            raise AssertionError(f"gain shape omitted concrete gain: {atom}")
        if not str(atom.get("purpose", "")).strip():
            raise AssertionError(f"atom omitted acoustic purpose: {atom}")
        origins = atom.get("field_origins")
        if not isinstance(origins, dict) or "frequency_hz" not in origins:
            raise AssertionError(f"atom omitted field origin: {atom}")
    if case.get("allowed_shapes"):
        if atoms[0].get("shape") not in case["allowed_shapes"]:
            raise AssertionError(f"LLM did not choose shelf/bell for high-frequency request: {atoms[0]}")
    if case.get("require_negative_constraint"):
        constraints = action.get("negative_constraints")
        if not isinstance(constraints, list) or not any("刺耳" in str(item) or "harsh" in str(item).lower() for item in constraints):
            raise AssertionError(f"negative harshness constraint was not preserved: {constraints}")
    previews = rows(data.get("preview_results"))
    if len(previews) != len(atoms) or any(str(row.get("status")) not in {"exact", "quantized"} for row in previews):
        raise AssertionError(f"proposal preview did not report exact/quantized per atom: {previews}")
    assert_no_forbidden_semantic_eq_tools(response)
    return action, atoms


def validate_execution(response: dict, atom_count: int) -> str:
    data = workflow_data(response)
    details = data.get("result")
    if not isinstance(details, dict):
        raise AssertionError(f"execution response omitted receipt details: {data}")
    if details.get("structural_readback") != "pass":
        raise AssertionError(f"structural readback did not pass: {details}")
    inner = details.get("result")
    if not isinstance(inner, dict) or inner.get("status") not in {"exact", "quantized"}:
        raise AssertionError(f"executor omitted exact/quantized status: {inner}")
    atom_results = rows(details.get("atom_results"))
    if len(atom_results) != atom_count:
        raise AssertionError(f"atom result coverage mismatch: {atom_results}")
    for atom in atom_results:
        if atom.get("status") not in {"exact", "quantized"}:
            raise AssertionError(f"invalid atom execution status: {atom}")
        if not rows(atom.get("actual_readback")):
            raise AssertionError(f"atom omitted actual_readback: {atom}")
    operation_ref = str(inner.get("operation_ref", "")).strip()
    if not operation_ref:
        raise AssertionError(f"execution omitted operation_ref: {inner}")
    rollback = inner.get("rollback")
    if not isinstance(rollback, dict) or rollback.get("available") is not True:
        raise AssertionError(f"execution omitted rollback limitation: {rollback}")
    verification = data.get("verification")
    if not isinstance(verification, dict) or verification.get("structural") != "pass":
        raise AssertionError(f"verification omitted structural pass: {verification}")
    if verification.get("acoustic") not in {"pass", "unavailable", "inconclusive"}:
        raise AssertionError(f"invalid acoustic verification report: {verification}")
    if verification.get("user_acceptance") != "unknown":
        raise AssertionError(f"user acceptance was overclaimed: {verification}")
    assert_no_forbidden_semantic_eq_tools(response)
    return operation_ref


def cleanup_operation(base: str, track_id: str, plugin_id: str, operation_ref: str, timeout: float) -> None:
    response = invoke(
        base,
        "plugin_grabber.apply_eq_edits",
        {
            "track_id": track_id,
            "plugin_id": plugin_id,
            "atomic": True,
            "edits": [{"action": "undo", "operation_ref": operation_ref}],
        },
        timeout,
    )
    if str(response.get("status", "")).lower() != "ok":
        raise AssertionError(f"smoke cleanup undo failed: {response}")


def dump_response(directory: str, case_id: str, stage: str, response: dict) -> None:
    if not directory:
        return
    os.makedirs(directory, exist_ok=True)
    path = os.path.join(directory, f"{case_id}_{stage}.json")
    with open(path, "w", encoding="utf-8") as handle:
        json.dump(response, handle, ensure_ascii=False, indent=2)


def run(args: argparse.Namespace) -> int:
    base = args.agent_http.rstrip("/")
    timeout = float(args.timeout_sec)
    base_context: dict[str, Any] = {
        "agent_mode": "chat",
        "selected_track_id": args.track_id,
        "selected_plugin_track_id": args.track_id,
        "selected_plugin_id": args.plugin_id,
        "selected_plugin_name": args.plugin_name or "TDR Nova",
    }
    print("== ordinary-Agent semantic EQ smoke ==")
    print(f"agent={base} track={args.track_id} plugin={args.plugin_id}")
    wait_agent(base, min(timeout, 30))
    failures: list[str] = []
    summary: list[dict] = []

    for index, case in enumerate(CASES):
        conversation_id = f"semantic_eq_smoke_{int(time.time())}_{index}"
        print(f"\n[{index + 1}/{len(CASES)}] {case['message']}")
        try:
            first = chat(base, conversation_id, case["message"], base_context, timeout)
            dump_response(args.output_dir, case["id"], "proposal", first)
            if case["kind"] == "discussion":
                validate_discussion(first)
                print("  ok: discussion/observation only; no proposal and no mutation")
                summary.append({"id": case["id"], "status": "pass", "stage": "discussion"})
                continue

            _, atoms = validate_typed_plan(case, first)
            previews = rows(workflow_data(first).get("preview_results"))
            print("  ok: frozen proposal; " + ", ".join(f"{row.get('atom_id')}={row.get('status')}" for row in previews))
            confirmation_context = exact_confirmation_context(first, base_context)
            confirmed = chat(base, conversation_id, "确认执行这个方案", confirmation_context, timeout)
            dump_response(args.output_dir, case["id"], "execution", confirmed)
            operation_ref = validate_execution(confirmed, len(atoms))
            verification = workflow_data(confirmed).get("verification") or {}
            print(
                f"  ok: governed execution operation_ref={operation_ref} "
                f"structural={verification.get('structural')} acoustic={verification.get('acoustic')}"
            )
            cleanup_operation(base, args.track_id, args.plugin_id, operation_ref, timeout)
            print("  ok: restored smoke preimage")
            summary.append(
                {
                    "id": case["id"],
                    "status": "pass",
                    "atoms": len(atoms),
                    "preview": [row.get("status") for row in previews],
                    "acoustic": verification.get("acoustic"),
                }
            )
        except Exception as exc:
            message = f"{case['id']}: {exc}"
            failures.append(message)
            summary.append({"id": case["id"], "status": "fail", "error": str(exc)})
            print(f"  FAIL: {exc}")

    if args.output_dir:
        os.makedirs(args.output_dir, exist_ok=True)
        with open(os.path.join(args.output_dir, "summary.json"), "w", encoding="utf-8") as handle:
            json.dump({"cases": summary, "failures": failures}, handle, ensure_ascii=False, indent=2)
    print()
    if failures:
        print(f"{len(failures)}/{len(CASES)} case(s) FAILED")
        for failure in failures:
            print("- " + failure)
        return 1
    print(f"All {len(CASES)} semantic EQ cases passed.")
    return 0


def main() -> None:
    parser = argparse.ArgumentParser(description="ordinary-Agent semantic EQ smoke")
    parser.add_argument("--agent-http", default=AGENT_HTTP_DEFAULT)
    parser.add_argument("--track-id", required=True)
    parser.add_argument("--plugin-id", required=True)
    parser.add_argument("--plugin-name", default="TDR Nova")
    parser.add_argument("--timeout-sec", type=float, default=180)
    parser.add_argument("--output-dir", default="")
    sys.exit(run(parser.parse_args()))


if __name__ == "__main__":
    main()
