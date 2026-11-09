#!/usr/bin/env python3
"""FIX-GOV-INTERACTION-TTL-1 — run9 r2 governance-interaction replay (real-stack leg).

Stack contract (AGENTS.md §5/§9, r2_replay_plan.md §8 frozen before the run):
  1 run; retry budget 2 (agent-log anchors re-checked between attempts);
  LLM ceiling 16 calls x 40k in / 8k out per call (parking a card cost 5
  calls / 37s in run9 r2; ceiling covers one follow-up park round for the
  normal-approve positive); failure classes recorded separately:
  no_card_parked / approve_lost / receipt_wrong / env_interrupt;
  stop-loss = approve_lost twice on the same code version.

Phases:
  0 (deterministic, zero LLM): authority + Pro-Q 3 registry resolve; build the
    round project copy B (new_project -> save_as -> add_track ->
    clip.import_audio -> rack_add_node(Pro-Q 3) -> save, card-311 placement
    precedent, fresh unique paths only) and the bare warm project A; settle the
    agent workspace onto A (bounded poll + nudge); then open B through the
    kernel with NO agent-activation wait — the run9 r2 precondition (agent
    activeWorkspace stays A while the kernel holds B).
  1 (LLM leg): one T7 chat turn (verbatim) on a B-bound conversation until it
    parks an improvement_proposal confirmation card.
  2 (assert legs, supervisor criteria 2026-11-09):
    leg1 late approve — FAIL only on the pre-fix canned shape (罐头
    「已处理或已过期」) or a fabricated goal_status=completed; PASS when the
    approve routes (no catch-all marker at all — the restore-preserve fix
    keeping the card answerable) OR answers with an explicit
    interaction_not_found / interaction_already_retired receipt whose
    goal_status != completed.
    leg2 repeated approve — explicit already_retired receipt, goal_status
    empty, reply says 未执行.
    leg3 unknown id — explicit interaction_not_found receipt, goal_status
    empty, reply says 未执行.
    positive (normal approve must not regress): satisfied by leg1 routing; if
    leg1 answered with an explicit receipt instead, a second park round
    (conversation 2) supplies the positive within the LLM ceiling.

Agent-log WARN anchors ([interaction.respond] ... lines) and the
[workspace] activated timeline are captured from <RepoRoot>/VitApp/Workspace/
Logs/agent_last.log by the operator after the run (see run receipt), not by
this driver.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any

_HERE = Path(__file__).resolve().parent
_REPO = _HERE.parents[3]
for _p in (str(_REPO / "scripts"),
           str(_REPO / "coord" / "runs" / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen")):
    if _p not in sys.path:
        sys.path.insert(0, _p)

import plugin_control_eval_driver as frozen  # noqa: E402  (byte-frozen 318 trio)
from compressor_compat_matrix_smoke import (  # noqa: E402
    ensure_full_project_access, first_text, kernel_command,
)

SOURCE = "gov_ttl_replay"
CANNED_MARKER = "已处理或已过期"
EXPLICIT_STOPS = ("interaction_not_found", "interaction_already_retired")
T7_MESSAGE = (
    "请用这条轨上的 Pro-Q 3 做你认为有必要的均衡处理。调整完必须说明："
    "动了哪个/哪些频段，每段的频率、Q 值、增益量、滤波器类型"
    "（钟形/搁架/高通/低通等），以及为什么。"
)
PROQ3_NAME = "Pro-Q 3"
PROQ3_IDENTIFIER = "VST3-Pro-Q 3-a1aba170-ba5f7ab2"
TRACK_NAME = "素材轨"
AUDIO_SOURCE = (
    r"D:\Vit_DAW\artifacts\B6\20260911_181409\project\.vit_agent"
    r"\vitproj_b3f2838183d64add9ca7824b8b3ac557\artifacts\d1_s1"
    r"\turnfree_state_c5eee91577cfc5b5\before_revision_1.wav"
)


class ReplayFailure(RuntimeError):
    def __init__(self, failure_class: str, detail: str):
        super().__init__(f"{failure_class}: {detail}")
        self.failure_class = failure_class


def kernel_ok(reply: dict[str, Any]) -> bool:
    return str(reply.get("status", "")).lower() in {"ok", "success"}


def resolve_proq3(timeout: float) -> dict[str, Any]:
    reply = kernel_command({"cmd": "plugin_search", "query": PROQ3_NAME,
                            "command_timeout_ms": 60000}, timeout)
    matches = (reply.get("matches") or reply.get("plugins")
               or reply.get("result") or [])
    if isinstance(matches, dict):
        matches = matches.get("matches", [])
    rows = [row for row in matches if isinstance(row, dict)
            and PROQ3_IDENTIFIER in json.dumps(row, ensure_ascii=False)]
    if not rows:
        raise ReplayFailure("env_interrupt", f"plugin_search did not resolve {PROQ3_IDENTIFIER}: {str(reply)[:400]}")
    row = rows[0]
    path = first_text(row, "file_or_identifier", "plugin_path", "path")
    if not path:
        raise ReplayFailure("env_interrupt", f"Pro-Q 3 row without path: {str(row)[:300]}")
    return {"plugin_identifier": PROQ3_IDENTIFIER, "plugin_path": path}


def build_round_project(project_path: str, plugin: dict[str, Any], agent_http: str,
                        timeout: float, audio_source: str) -> dict[str, Any]:
    """Card-311 placement precedent, fresh path only (no overwrite)."""
    if Path(project_path).exists():
        raise ReplayFailure("env_interrupt", f"project copy already exists (rerun hygiene): {project_path}")
    steps: list[tuple[str, dict[str, Any]]] = [
        ("new_project", kernel_command({"cmd": "new_project", "command_timeout_ms": 60000}, timeout)),
        ("save_as", kernel_command({"cmd": "save_as_project", "file_path": project_path,
                                    "command_timeout_ms": 60000}, timeout)),
    ]
    add = kernel_command({"cmd": "add_track", "name": TRACK_NAME,
                          "command_timeout_ms": 60000}, timeout)
    if not kernel_ok(add):
        add = kernel_command({"cmd": "add_track", "command_timeout_ms": 60000}, timeout)
    steps.append(("add_track", add))
    track_id = first_text(add, "track_id", "id")
    if not kernel_ok(add) or not track_id:
        raise ReplayFailure("env_interrupt", f"add_track failed: {str(add)[:300]}")
    imported = frozen.invoke(agent_http, "clip.import_audio",
                             {"track_id": track_id, "file_path": audio_source}, timeout, True)
    if str(imported.get("status", "")).lower() not in {"ok", "success", "completed"}:
        raise ReplayFailure("env_interrupt", f"clip.import_audio failed: {str(imported)[:400]}")
    node = kernel_command({"cmd": "rack_add_node", "track_id": track_id,
                           "plugin_path": plugin["plugin_path"],
                           "plugin_identifier": plugin["plugin_identifier"],
                           "auto_connect": True, "x": 0.32, "y": 0.32}, timeout)
    if not kernel_ok(node):
        raise ReplayFailure("env_interrupt", f"rack_add_node rejected Pro-Q 3: {str(node)[:400]}")
    save = kernel_command({"cmd": "save_project", "command_timeout_ms": 60000}, timeout)
    if not kernel_ok(save):
        raise ReplayFailure("env_interrupt", f"save_project failed: {str(save)[:300]}")
    return {"project_path": project_path, "track_id": track_id,
            "steps": {name: str(reply.get("status", "")) for name, reply in steps}}


def open_project_kernel(project_path: str, timeout: float) -> None:
    reply = kernel_command({"cmd": "open_project", "file_path": project_path,
                            "command_timeout_ms": 60000}, timeout)
    if not kernel_ok(reply):
        raise ReplayFailure("env_interrupt", f"open_project failed for {project_path}: {str(reply)[:300]}")


def agent_workspace_path(agent_http: str, timeout: float) -> str:
    try:
        state = frozen.request_json("GET", agent_http.rstrip("/") + "/agent/ui/state", None, 10.0)
    except Exception:  # noqa: BLE001 — evidence poll, not a gate
        return ""
    return str(state.get("project_path", "") or "")


def settle_workspace_on(agent_http: str, project_path: str, timeout_s: float) -> dict[str, Any]:
    """Bounded settle with the card-311 nudge precedent (GET /agent/state).
    Evidence, not a gate: the r2 window only needs the agent to be SOMEWHERE
    else when B's chat starts."""
    record: dict[str, Any] = {"target": project_path, "settled": False, "polls": 0}
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        record["polls"] += 1
        if agent_workspace_path(agent_http, 10.0) == project_path:
            record["settled"] = True
            return record
        try:
            frozen.request_json("GET", agent_http.rstrip("/") + "/agent/state", None, 10.0)
        except Exception:  # noqa: BLE001 — nudge best effort
            pass
        time.sleep(4.0)
    return record


def park_improvement_card(agent_http: str, conversation: str, chat_timeout: float,
                          followups: int = 2) -> tuple[str, dict[str, Any]]:
    """One chat turn (T7 verbatim, bare product chat context) until it parks an
    improvement_proposal card; bounded frozen follow-ups for clarifications."""
    resp = frozen.chat(agent_http, conversation, T7_MESSAGE, {"agent_mode": "chat"}, chat_timeout)
    record: dict[str, Any] = {"turns": 1, "final_stop": resp.get("stop_reason"),
                              "goal_status": resp.get("goal_status"),
                              "first_reply": str(resp.get("reply", ""))[:400]}
    for _ in range(followups):
        card = first_improvement_interaction(resp)
        if card:
            record["interaction_id"] = card
            return card, record
        if str(resp.get("goal_status", "")).lower() in {"completed", "done", "succeeded", "failed", "cancelled"}:
            break
        record["turns"] += 1
        # Attempt-3 lesson: the bare frozen follow-up ("请按任务描述继续即可...")
        # is rejected by the semantic entry layer as task-less content
        # (semantic_entry_unresolved, goal-less turn). Re-send the T7 task text
        # verbatim so the entry classifier binds a concrete control request.
        record.setdefault("followup_replies", []).append(str(resp.get("reply", ""))[:400])
        resp = frozen.chat(agent_http, conversation, T7_MESSAGE,
                           {"agent_mode": "chat"}, chat_timeout)
        record["final_stop"] = resp.get("stop_reason")
        record["goal_status"] = resp.get("goal_status")
        record.setdefault("followup_replies", []).append(str(resp.get("reply", ""))[:400])
    record["interaction_id"] = ""
    return "", record


def first_improvement_interaction(resp: dict[str, Any]) -> str:
    for row in resp.get("interaction_requests") or []:
        if not isinstance(row, dict):
            continue
        kind = str(row.get("kind", ""))
        if "improvement_proposal" in kind and str(row.get("status", "")) in (
                "waiting_for_user", "waiting_user", "pending_user", ""):
            return str(row.get("id") or "")
    return ""


def classify_approve(resp: dict[str, Any]) -> dict[str, Any]:
    """Supervisor criteria: the canned catch-all or a fabricated
    goal_status=completed is the defect; routing or an explicit receipt passes."""
    reply = str(resp.get("reply", ""))
    stop = str(resp.get("stop_reason", ""))
    goal = str(resp.get("goal_status", ""))
    canned = CANNED_MARKER in reply
    explicit = stop in EXPLICIT_STOPS and "未执行" in reply
    # goal_status=completed is only "fabricated" on a catch-all shaped receipt
    # (the run9 r2 defect); a routed continuation that legitimately reaches
    # completed (e.g. an honest boundary response) is not a swallow.
    fabricated_completed = goal.lower() == "completed" and (canned or explicit)
    routed = not canned and not explicit
    verdict = "FAIL" if (canned or fabricated_completed) else "PASS"
    return {"reply_head": reply[:160], "stop_reason": stop, "goal_status": goal,
            "canned": canned, "explicit_receipt": explicit,
            "fabricated_completed": fabricated_completed, "routed": routed,
            "verdict": verdict}


def clear_leftover_waiting_goal(agent_http: str, timeout: float) -> dict[str, Any]:
    """Boot-leg / previous-attempt leftovers park waiting goals that block the
    authority switch (attempt-1/attempt-4 shape, both evidenced). Stop any
    waiting/active goal before phase 0 so the run starts from a clean berth."""
    try:
        status = frozen.request_json("GET", agent_http.rstrip("/") + "/agent/runtime/status", None, 10.0)
    except Exception as error:  # noqa: BLE001
        return {"checked": False, "error": str(error)[:200]}
    goal = status.get("goal") if isinstance(status.get("goal"), dict) else {}
    goal_id = str(goal.get("goal_id") or "")
    goal_status = str(goal.get("status") or "")
    record = {"checked": True, "goal_id": goal_id, "status": goal_status, "stopped": False}
    if not goal_id or goal_status.lower() in {"", "idle", "completed", "done", "succeeded", "failed", "cancelled"}:
        return record
    body = {"goal_id": goal_id, "conversation_id": str(goal.get("conversation_id") or ""),
            "reason": "govttl_env_clear_327"}
    try:
        stop = frozen.request_json("POST", agent_http.rstrip("/") + "/agent/turn/stop", body, 30.0)
        record["stopped"] = True
        record["stop_reply_status"] = str(stop.get("status", ""))[:80]
    except Exception as error:  # noqa: BLE001
        record["stop_error"] = str(error)[:200]
    return record


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--projects-dir", required=True,
                        help="fresh run workspace for the two project copies (A warm / B round)")
    parser.add_argument("--audio-source", default=AUDIO_SOURCE)
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--chat-timeout", type=float, default=240.0)
    parser.add_argument("--out-dir", required=True)
    args = parser.parse_args()

    out = Path(args.out_dir)
    out.mkdir(parents=True, exist_ok=True)
    stamp = time.strftime("%Y%m%d_%H%M%S")
    record: dict[str, Any] = {"schema_version": "gov_ttl_replay.v1", "source": SOURCE,
                              "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"), "legs": {}}
    try:
        # Phase 0 — deterministic device leg (zero LLM).
        record["env_clear"] = clear_leftover_waiting_goal(args.agent_http, args.timeout)
        authority = ensure_full_project_access(args.agent_http, args.timeout)
        record["authority"] = {"status": str(authority.get("status", ""))}
        plugin = resolve_proq3(args.timeout)
        record["plugin_registry"] = {"identifier": plugin["plugin_identifier"],
                                     "path": plugin["plugin_path"]}
        projects = Path(args.projects_dir)
        projects.mkdir(parents=True, exist_ok=True)
        project_b = str((projects / f"govttl_round_{stamp}.vit").resolve())
        project_a = str((projects / f"govttl_warm_{stamp}.vit").resolve())
        record["placement_b"] = build_round_project(project_b, plugin, args.agent_http,
                                                    args.timeout, args.audio_source)
        warm = kernel_command({"cmd": "new_project", "command_timeout_ms": 60000}, args.timeout)
        if not kernel_ok(warm):
            raise ReplayFailure("env_interrupt", f"new_project (A) failed: {str(warm)[:300]}")
        warm_save = kernel_command({"cmd": "save_as_project", "file_path": project_a,
                                    "command_timeout_ms": 60000}, args.timeout)
        if not kernel_ok(warm_save):
            raise ReplayFailure("env_interrupt", f"save_as (A) failed: {str(warm_save)[:300]}")
        record["placement_a"] = {"project_path": project_a, "status": "placed"}

        # r2 precondition: agent workspace settles on A (kernel current = A),
        # then B opens through the kernel with NO agent-activation wait.
        settle = settle_workspace_on(args.agent_http, project_a, 60.0)
        record["settle_on_a"] = settle
        open_project_kernel(project_b, args.timeout)
        record["workspace_at_chat_start"] = agent_workspace_path(args.agent_http, 10.0)

        # Phase 1 — park the card (LLM leg, budget per §8).
        conversation = f"gov_ttl_replay_{stamp}"
        card_id, park = park_improvement_card(args.agent_http, conversation, args.chat_timeout)
        record["park"] = {"conversation": conversation, **park}
        if not card_id:
            raise ReplayFailure("no_card_parked", f"chat parked no improvement_proposal card: {json.dumps(park, ensure_ascii=False)[:400]}")

        # Phase 2 — the three assertion legs.
        leg1_raw = frozen.interaction_respond(args.agent_http, card_id, "approve", "approve",
                                              args.chat_timeout)
        leg1 = classify_approve(leg1_raw)
        record["legs"]["late_approve"] = leg1
        if leg1["verdict"] == "FAIL":
            raise ReplayFailure("approve_lost", json.dumps(leg1, ensure_ascii=False))

        leg2_raw = frozen.interaction_respond(args.agent_http, card_id, "approve", "approve", args.timeout)
        leg2 = {"stop_reason": str(leg2_raw.get("stop_reason", "")),
                "goal_status": str(leg2_raw.get("goal_status", "")),
                "reply_head": str(leg2_raw.get("reply", ""))[:160]}
        leg2["verdict"] = "PASS" if (leg2["stop_reason"] == "interaction_already_retired"
                                     and leg2["goal_status"] == "" and "未执行" in leg2["reply_head"]) else "FAIL"
        record["legs"]["repeat_approve"] = leg2

        unknown_id = f"interaction_govttl_probe_{stamp}"
        leg3_raw = frozen.interaction_respond(args.agent_http, unknown_id, "approve", "approve", args.timeout)
        leg3 = {"stop_reason": str(leg3_raw.get("stop_reason", "")),
                "goal_status": str(leg3_raw.get("goal_status", "")),
                "reply_head": str(leg3_raw.get("reply", ""))[:160]}
        leg3["verdict"] = "PASS" if (leg3["stop_reason"] == "interaction_not_found"
                                     and leg3["goal_status"] == "" and "未执行" in leg3["reply_head"]) else "FAIL"
        record["legs"]["unknown_approve"] = leg3

        # Normal-approve positive: leg1 routing satisfies it; otherwise one
        # more park round within the LLM ceiling.
        positive: dict[str, Any] = {"satisfied_by": "leg1_routed" if leg1["routed"] else "", "verdict": "PASS"}
        if not leg1["routed"]:
            conversation2 = f"gov_ttl_replay2_{stamp}"
            card2, park2 = park_improvement_card(args.agent_http, conversation2, args.chat_timeout)
            record["park2"] = {"conversation": conversation2, **park2}
            if not card2:
                positive = {"satisfied_by": "second_round", "verdict": "FAIL",
                            "detail": "no card parked in the follow-up round"}
            else:
                approve2_raw = frozen.interaction_respond(args.agent_http, card2, "approve", "approve",
                                                           args.chat_timeout)
                approve2 = classify_approve(approve2_raw)
                positive = {"satisfied_by": "second_round", **approve2}
                if not approve2["routed"] or approve2["verdict"] == "FAIL":
                    positive["verdict"] = "FAIL"
        record["legs"]["normal_approve_positive"] = positive

        failures = [name for name, leg in record["legs"].items() if leg.get("verdict") == "FAIL"]
        if failures:
            raise ReplayFailure("receipt_wrong", "failed legs: " + ",".join(failures))
        record["verdict"] = "PASS"
        return 0
    except ReplayFailure as failure:
        record["verdict"] = "FAIL"
        record["failure_class"] = failure.failure_class
        record["failure_detail"] = str(failure)
        print(f"FAIL {failure.failure_class}: {failure}", file=sys.stderr)
        return 1
    finally:
        record["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
        (out / "replay_record.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"record: {out / 'replay_record.json'}")
        print(record.get("verdict", "INCOMPLETE"))


if __name__ == "__main__":
    raise SystemExit(main())
