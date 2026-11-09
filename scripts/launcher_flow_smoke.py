# -*- coding: utf-8 -*-
"""LAUNCHER-SMOKE-1 (339) assertion driver.

Legs (per RUNBOOK.md -- section 8 plan frozen before any stack run):
  main            launcher plain-language open -> directive receipt -> zero-friction
                  governance premise -> UI consumption chain (kernel open_project +
                  version.project_opened lifecycle notify) -> bind observation ->
                  bridge broadcast turn -> one project-flow LLM turn -> telemetry
                  budget check. LLM budget <= 10 calls.
  fallback        result-card click path (zero LLM): ambiguous reference ->
                  project_result_cards (kind=launcher_project with embedded
                  directive) -> click card[0] -> same open chain -> bridge turn.
                  Telemetry must be exactly 0 rows.
  degraded_kernel degraded leg kernel face: classic-flow data source
                  (get_recent_projects) + open_project availability with no agent
                  in the stack. Zero LLM by construction (no agent).

Exit codes (RUNBOOK failure classification, recorded separately):
  0 pass | 2 env_interrupt | 3 llm_route (intent_unresolved / mis_route)
  4 deterministic-face defect / launcher_gate_closed_at_boot / budget_exceeded
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

sys.stdout.reconfigure(encoding="utf-8", errors="replace")
sys.stderr.reconfigure(encoding="utf-8", errors="replace")

import zmq  # noqa: E402


class LegFailure(RuntimeError):
    def __init__(self, code: int, failure_class: str, detail: str):
        super().__init__(f"{failure_class}: {detail}")
        self.code = code
        self.failure_class = failure_class


def _now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S")


def http_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None
    headers = {}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read().decode("utf-8", errors="replace")
            return json.loads(body)
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")[:2000]
        raise LegFailure(4, "http_face_error", f"{method} {url} -> HTTP {exc.code}: {body}") from exc
    except urllib.error.URLError as exc:
        raise LegFailure(2, "agent_unreachable", f"{method} {url} -> {exc.reason}") from exc


def kernel_command(socket_uri: str, command: dict[str, Any], timeout: float) -> dict[str, Any]:
    context = zmq.Context.instance()
    socket = context.socket(zmq.REQ)
    timeout_ms = max(1000, int(timeout * 1000))
    socket.setsockopt(zmq.RCVTIMEO, timeout_ms)
    socket.setsockopt(zmq.SNDTIMEO, timeout_ms)
    socket.setsockopt(zmq.LINGER, 0)
    socket.connect(socket_uri)
    try:
        socket.send_json(command)
        reply = socket.recv_json()
    except zmq.error.ZMQError as exc:
        raise LegFailure(2, "kernel_zmq_error", f"{command.get('cmd')} -> {exc}") from exc
    finally:
        socket.close()
    if not isinstance(reply, dict):
        raise LegFailure(2, "kernel_zmq_error", f"{command.get('cmd')} returned non-object JSON")
    return reply


def norm_path(value: str) -> str:
    return value.strip().replace("\\", "/").rstrip("/").lower()


def chat(agent_http: str, conversation: str, message: str, launcher: bool, timeout: float) -> dict[str, Any]:
    context: dict[str, Any] = {"agent_mode": "chat"}
    if launcher:
        context["launcher"] = True
    return http_json(
        "POST",
        agent_http.rstrip("/") + "/agent/chat",
        {"conversation_id": conversation, "message": message, "context": context},
        timeout,
    )


def telemetry_rows(path: str) -> int:
    file = Path(path)
    if not file.exists():
        return 0
    count = 0
    with file.open("r", encoding="utf-8", errors="replace") as handle:
        for line in handle:
            if line.strip():
                count += 1
    return count


def agent_state(agent_http: str) -> dict[str, Any]:
    state = http_json("GET", agent_http.rstrip("/") + "/agent/state", None, 15)
    if str(state.get("status", "")) != "ok":
        raise LegFailure(2, "agent_state_not_ok", json.dumps(state, ensure_ascii=False)[:400])
    return state


def assert_gate_open(state: dict[str, Any], facts: dict[str, Any]) -> None:
    """340 gate semantics: UI flag + user_entered_project session fact.

    uuid/path are no longer consulted by launcherFlowApplicable, so the boot
    project identity is recorded for forensics only (the 339 real-stack boot
    shapes -- default-project uuid and/or the VSP placeholder uuid with the
    draft-fallback path -- are the expected forms here). The behavioral proof
    that the gate is OPEN is the launcher_directive receipt asserted on turn 1
    (launcher_route_miss) and the zero-LLM fallback route.
    """
    current = state.get("current_project") or {}
    facts["boot_current_project"] = current
    facts["boot_gate_semantics"] = "340: ui_flag + user_entered_project (uuid/path not consulted)"


def probe_greeting(agent_http: str, facts: dict[str, Any], expect_recent: str | None) -> None:
    greeting = http_json("GET", agent_http.rstrip("/") + "/agent/launcher/greeting", None, 20)
    facts["greeting"] = greeting
    if str(greeting.get("status", "")) != "ok" or greeting.get("launcher") is not True:
        raise LegFailure(4, "greeting_broken", json.dumps(greeting, ensure_ascii=False)[:400])
    if "我在" not in str(greeting.get("reply", "")):
        raise LegFailure(4, "greeting_broken", "greeting reply missing opener: " + str(greeting.get("reply", ""))[:200])
    if expect_recent is not None:
        recent = greeting.get("recent_project") or {}
        if norm_path(str(recent.get("file_path", ""))) != norm_path(expect_recent):
            raise LegFailure(
                4,
                "greeting_recent_mismatch",
                f"greeting recent_project={recent!r} expected fixture {expect_recent}",
            )


def consume_directive_ui_chain(
    agent_http: str,
    kernel_zmq: str,
    directive: dict[str, Any],
    conversation_note: str,
    facts: dict[str, Any],
    timeout: float,
) -> None:
    """Replicates the 336 start_page consumption chain server-side:
    zero-friction premise check -> kernel open_project (encryption app_bound for
    .vit, same options the UI sends) -> version.project_opened lifecycle notify
    -> bind observation on /agent/state."""
    file_path = str(directive.get("file_path", "")).strip()
    if not file_path:
        raise LegFailure(4, "directive_missing_file_path", json.dumps(directive, ensure_ascii=False)[:400])
    if not Path(file_path).exists():
        raise LegFailure(2, "fixture_missing", f"directive target not on disk: {file_path}")

    # Pre-open identity record (340): 339 proved on the real stack that
    # current_project.path at boot is the draft fallback (non-empty) even while
    # the user has entered no project, so the 336 UI zero-friction premise
    # (path=="") is unreachable at boot -- the UI shows its governance confirm
    # card instead (by design; the two faces keep different semantics: UI asks
    # "confirm or not", the backend gate asks "launcher route or not"). The
    # driver executes the confirmed-open chain directly; the hard assertion is
    # the bind observation below. Only an already-bound identical fixture
    # (re-open of the same project) would blur the bridge semantics.
    state = agent_state(agent_http)
    current = state.get("current_project") or {}
    facts[f"{conversation_note}_pre_open_current_project"] = current
    if norm_path(str(current.get("path", ""))) == norm_path(file_path):
        raise LegFailure(
            4,
            "governance_premise_broken",
            "current_project.path already bound to the fixture before open (re-open would blur bridge semantics): "
            + json.dumps(current, ensure_ascii=False),
        )

    open_cmd: dict[str, Any] = {"cmd": "open_project", "file_path": file_path, "command_timeout_ms": 60000}
    if file_path.lower().endswith(".vit"):
        open_cmd["encryption"] = {"mode": "app_bound_aes"}
    opened = kernel_command(kernel_zmq, open_cmd, timeout)
    facts[f"{conversation_note}_kernel_open_reply"] = {k: opened.get(k) for k in ("status", "message", "project_path", "project_uuid", "project_lifecycle")}
    if str(opened.get("status", "")).lower() not in ("ok", "success"):
        raise LegFailure(4, "kernel_open_failed", json.dumps(opened, ensure_ascii=False)[:500])

    opened_uuid = str(opened.get("project_uuid", opened.get("project_id", "")))
    notify = http_json(
        "POST",
        agent_http.rstrip("/") + "/agent/invoke",
        {
            "tool": "version.project_opened",
            "args": {"project_path": file_path, "project_uuid": opened_uuid},
            "source": "start_page_open_project",
            "confirmed": True,
        },
        30,
    )
    facts[f"{conversation_note}_lifecycle_notify"] = {"status": notify.get("status"), "error": notify.get("error")}
    if str(notify.get("status", "")).lower() not in ("ok", "success"):
        # The UI itself fail-silents this notify (push_warning only); record as
        # warning, the bind observation below is the hard assertion.
        facts[f"{conversation_note}_lifecycle_notify_warning"] = json.dumps(notify, ensure_ascii=False)[:300]

    deadline = time.time() + 45
    bound = None
    while time.time() < deadline:
        state = agent_state(agent_http)
        current = state.get("current_project") or {}
        if norm_path(str(current.get("path", ""))) == norm_path(file_path):
            bound = current
            break
        time.sleep(1.0)
    facts[f"{conversation_note}_bound_current_project"] = bound
    if bound is None:
        raise LegFailure(
            4,
            "bind_not_observed",
            f"agent never observed current_project.path == {file_path} within 45s",
        )


def assert_bridge_turn(agent_http: str, conversation: str, fixture_path: str, facts: dict[str, Any], key: str) -> None:
    reply = chat(agent_http, conversation, "现在这个工程的轨道情况怎么样？", False, 120)
    facts[key] = reply
    if str(reply.get("stop_reason", "")) != "launcher_bridge_broadcast":
        raise LegFailure(
            4,
            "bridge_broadcast_missing",
            f"expected stop_reason=launcher_bridge_broadcast, got {reply.get('stop_reason')!r}; "
            + json.dumps(reply, ensure_ascii=False)[:400],
        )
    if str(reply.get("context_flow", "")) != "project":
        raise LegFailure(4, "bridge_flow_mark_wrong", f"context_flow={reply.get('context_flow')!r}")
    if "已进入" not in str(reply.get("reply", "")):
        raise LegFailure(4, "bridge_reply_missing", str(reply.get("reply", ""))[:200])
    events = reply.get("typed_events") or []
    if not events or str(events[0].get("type", "")) != "volume_switch":
        raise LegFailure(4, "volume_switch_missing", json.dumps(events, ensure_ascii=False)[:300])
    if norm_path(str(events[0].get("project_path", ""))) != norm_path(fixture_path):
        raise LegFailure(4, "volume_switch_path_wrong", json.dumps(events[0], ensure_ascii=False)[:300])


def write_summary(artifacts_dir: Path, summary: dict[str, Any]) -> None:
    artifacts_dir.mkdir(parents=True, exist_ok=True)
    (artifacts_dir / "driver_summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
    )


def leg_main(args: argparse.Namespace, facts: dict[str, Any]) -> dict[str, Any]:
    facts.update({"leg": "main", "started_at": _now(), "conversation": args.conversation})
    state = agent_state(args.agent_http)
    facts["tool_count"] = state.get("tool_count")
    probe_greeting(args.agent_http, facts, args.fixture_main)
    assert_gate_open(state, facts)

    # Turn 1: plain-language launcher open (deterministic route).
    reply = chat(args.agent_http, args.conversation, "打开小星星乐局", True, 90)
    facts["directive_turn"] = reply
    directive = reply.get("launcher_directive")
    if not isinstance(directive, dict):
        after = agent_state(args.agent_http)
        facts["directive_turn_state_after"] = after.get("current_project")
        raise LegFailure(
            4,
            "launcher_route_miss",
            "no launcher_directive in receipt (reply keys: "
            + ",".join(sorted(reply.keys()))
            + "); gate state after: "
            + json.dumps(after.get("current_project"), ensure_ascii=False),
        )
    if str(directive.get("action", "")) != "open_project":
        raise LegFailure(4, "directive_action_wrong", json.dumps(directive, ensure_ascii=False)[:300])
    if norm_path(str(directive.get("file_path", ""))) != norm_path(args.fixture_main):
        raise LegFailure(4, "directive_target_wrong", json.dumps(directive, ensure_ascii=False)[:300])
    if directive.get("requires_governance") is not True:
        raise LegFailure(4, "directive_governance_flag_wrong", json.dumps(directive, ensure_ascii=False)[:300])
    if str(reply.get("context_flow", "")) != "launcher":
        raise LegFailure(4, "directive_flow_mark_wrong", f"context_flow={reply.get('context_flow')!r}")
    if "《小星星乐局》" not in str(reply.get("reply", "")):
        raise LegFailure(4, "confirmation_sentence_missing", str(reply.get("reply", ""))[:200])
    if reply.get("project_result_cards"):
        raise LegFailure(4, "unexpected_result_cards", "unique-hit turn must not carry cards")

    consume_directive_ui_chain(args.agent_http, args.kernel_zmq, directive, "main", facts, args.timeout)

    assert_bridge_turn(args.agent_http, args.conversation, args.fixture_main, facts, "bridge_turn")

    # Turn 3: one real project-flow LLM round (the budget consumer).
    reply = chat(args.agent_http, args.conversation, "简单说说这个工程现在的状态就行", False, 240)
    facts["project_flow_turn"] = {
        k: reply.get(k) for k in ("reply", "context_flow", "stop_reason", "error", "needs_confirmation")
    }
    if str(reply.get("context_flow", "")) != "project":
        raise LegFailure(3, "project_flow_mark_missing", f"context_flow={reply.get('context_flow')!r} (LLM turn)")
    if not str(reply.get("reply", "")).strip():
        raise LegFailure(3, "project_flow_reply_empty", json.dumps(reply, ensure_ascii=False)[:300])

    rows = telemetry_rows(args.telemetry_path)
    facts["llm_telemetry_rows"] = rows
    facts["llm_budget"] = 10
    if rows > 10:
        raise LegFailure(4, "budget_exceeded", f"LLM telemetry rows {rows} > budget 10")
    return facts


def leg_fallback(args: argparse.Namespace, facts: dict[str, Any]) -> dict[str, Any]:
    facts.update({"leg": "fallback", "started_at": _now(), "conversation": args.conversation})
    state = agent_state(args.agent_http)
    facts["tool_count"] = state.get("tool_count")
    probe_greeting(args.agent_http, facts, args.fixture_a)
    assert_gate_open(state, facts)

    reply = chat(args.agent_http, args.conversation, "打开退路", True, 90)
    facts["cards_turn"] = reply
    cards = reply.get("project_result_cards") or []
    if len(cards) < 2:
        raise LegFailure(
            4,
            "result_cards_missing",
            f"expected >=2 launcher_project cards for ambiguous reference, got {len(cards)}; "
            + json.dumps(reply, ensure_ascii=False)[:400],
        )
    if reply.get("launcher_directive"):
        raise LegFailure(4, "unexpected_top_level_directive", "multi-hit turn must use cards, not directive")
    if str(reply.get("context_flow", "")) != "launcher":
        raise LegFailure(4, "cards_flow_mark_wrong", f"context_flow={reply.get('context_flow')!r}")
    hit_paths = []
    for card in cards:
        if str(card.get("kind", "")) != "launcher_project":
            raise LegFailure(4, "card_kind_wrong", json.dumps(card, ensure_ascii=False)[:300])
        embedded = card.get("launcher_directive")
        if not isinstance(embedded, dict) or str(embedded.get("action", "")) != "open_project":
            raise LegFailure(4, "card_directive_missing", json.dumps(card, ensure_ascii=False)[:300])
        hit_paths.append(str(card.get("file_path", "")))
    facts["card_paths"] = hit_paths
    if norm_path(hit_paths[0]) != norm_path(args.fixture_a):
        raise LegFailure(4, "card_order_wrong", f"card[0]={hit_paths[0]} expected {args.fixture_a}")

    # Click card[0]: execute its embedded directive through the same UI chain.
    consume_directive_ui_chain(args.agent_http, args.kernel_zmq, cards[0], "fallback", facts, args.timeout)
    assert_bridge_turn(args.agent_http, args.conversation, args.fixture_a, facts, "bridge_turn")

    rows = telemetry_rows(args.telemetry_path)
    facts["llm_telemetry_rows"] = rows
    facts["llm_budget"] = 0
    if rows != 0:
        raise LegFailure(4, "fallback_used_llm", f"telemetry rows {rows} != 0 (fallback must be zero-LLM)")
    return facts


def leg_degraded_kernel(args: argparse.Namespace, facts: dict[str, Any]) -> dict[str, Any]:
    facts.update({"leg": "degraded_kernel", "started_at": _now()})
    recents = kernel_command(args.kernel_zmq, {"cmd": "get_recent_projects"}, args.timeout)
    facts["recent_projects"] = recents.get("recent_projects")
    if str(recents.get("status", "")).lower() not in ("ok", "success"):
        raise LegFailure(4, "recents_unavailable", json.dumps(recents, ensure_ascii=False)[:400])
    if not recents.get("recent_projects"):
        raise LegFailure(4, "recents_empty", "classic-flow data source returned no projects")

    fixture = args.fixture_degraded
    if not Path(fixture).exists():
        raise LegFailure(2, "fixture_missing", fixture)
    open_cmd: dict[str, Any] = {"cmd": "open_project", "file_path": fixture, "command_timeout_ms": 60000}
    if fixture.lower().endswith(".vit"):
        open_cmd["encryption"] = {"mode": "app_bound_aes"}
    opened = kernel_command(args.kernel_zmq, open_cmd, args.timeout)
    facts["kernel_open_reply"] = {k: opened.get(k) for k in ("status", "message", "project_path", "project_uuid")}
    if str(opened.get("status", "")).lower() not in ("ok", "success"):
        raise LegFailure(4, "kernel_open_failed", json.dumps(opened, ensure_ascii=False)[:400])
    if norm_path(str(opened.get("project_path", ""))) != norm_path(fixture):
        raise LegFailure(4, "kernel_open_path_wrong", json.dumps(opened, ensure_ascii=False)[:300])
    return facts


def main() -> int:
    parser = argparse.ArgumentParser(description="LAUNCHER-SMOKE-1 driver")
    parser.add_argument("--leg", required=True, choices=["main", "fallback", "degraded_kernel"])
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--artifacts-dir", required=True)
    parser.add_argument("--telemetry-path", default="")
    parser.add_argument("--fixture-main", default="")
    parser.add_argument("--fixture-a", default="")
    parser.add_argument("--fixture-degraded", default="")
    parser.add_argument("--conversation", default="")
    parser.add_argument("--timeout", type=float, default=60.0)
    args = parser.parse_args()

    if not args.conversation:
        args.conversation = "launcher_" + args.leg + "_" + time.strftime("%Y%m%d_%H%M%S")

    artifacts_dir = Path(args.artifacts_dir)
    summary: dict[str, Any] = {"leg": args.leg, "exit": None}
    facts: dict[str, Any] = {}
    try:
        if args.leg == "main":
            leg_main(args, facts)
        elif args.leg == "fallback":
            leg_fallback(args, facts)
        else:
            leg_degraded_kernel(args, facts)
        facts["finished_at"] = _now()
        summary.update(facts)
        summary["exit"] = 0
        summary["result"] = "PASS"
        write_summary(artifacts_dir, summary)
        print(f"LAUNCHER LEG {args.leg} PASS")
        return 0
    except LegFailure as exc:
        # partial facts survive into the summary (evidence for the finding)
        summary.update(facts)
        summary["failure_class"] = exc.failure_class
        summary["detail"] = str(exc)[:2000]
        summary["exit"] = exc.code
        summary["result"] = "FAIL"
        summary["finished_at"] = _now()
        write_summary(artifacts_dir, summary)
        print(f"LAUNCHER LEG {args.leg} FAIL code={exc.code} class={exc.failure_class}")
        print(str(exc)[:1500])
        return exc.code


if __name__ == "__main__":
    sys.exit(main())
