#!/usr/bin/env python3
"""FIX-GOV-INTERACTION-TTL-1 — run9 r2 governance-interaction replay (real-stack leg).

STATUS: [等待真栈窗口] — designed against the 320 frozen faces, NOT yet run.
Occupies the PC runtime stack per coord/resources/PC-RUNTIME-STACK.md before
boot; asserts the three receipt classes from r2_replay_plan.md §断言.

Pre-stack contract (§8, frozen before the run):
  1 run, legs serial; retry budget 2 (evidence re-checked between attempts);
  LLM call cap 12; failure classes no_card_parked / approve_lost /
  receipt_wrong / env_interrupt recorded separately; stop-loss = approve_lost
  twice on the same code version.

Reuses the frozen 320 driver faces (chat / interaction_respond / invoke /
request_json) exactly like coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/
proq3_free_driver.py — no second test mechanism (AGENTS.md §5).
"""

from __future__ import annotations

import argparse
import json
import sys
import time
from pathlib import Path
from typing import Any

_REPO = Path(__file__).resolve().parents[4]
_FROZEN = _REPO / "coord" / "runs" / "PLUGIN-CONTROL-EVAL-BAREFREE-1" / "runner_frozen"
sys.path.insert(0, str(_FROZEN))

import plugin_control_eval_driver as frozen  # noqa: E402

CATCH_ALL_MARKERS = ("已处理或已过期", "未找到待处理交互", "interaction_already_retired", "interaction_not_found")
T7_MESSAGE = ("请用这条轨上的 Pro-Q 3 做你认为有必要的均衡处理。调整完必须说明：动了哪个/哪些频段，"
              "每段的频率、Q 值、增益量、滤波器类型（钟形/搁架/高通/低通等），以及为什么。")


def open_project(agent_http: str, project_path: str, timeout: float) -> Any:
    return frozen.invoke(agent_http, "open_project", {"path": project_path}, timeout)


def first_improvement_interaction(resp: dict[str, Any]) -> str:
    for row in resp.get("interaction_requests") or []:
        if not isinstance(row, dict):
            continue
        kind = str(row.get("kind", ""))
        if "improvement_proposal" in kind and str(row.get("status", "")) in ("waiting_for_user", "waiting_user", "pending_user", ""):
            return str(row.get("id") or "")
    return ""


def llm_calls_used(record: dict[str, Any]) -> int:
    return len(record.get("llm_capture_rows", []))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--project-a", required=True, help="warm workspace project (opened LAST before the round, e.g. r1.vit)")
    parser.add_argument("--project-b", required=True, help="round project whose chat parks the card (e.g. r2.vit)")
    parser.add_argument("--timeout", type=float, default=120.0)
    parser.add_argument("--chat-timeout", type=float, default=180.0)
    parser.add_argument("--out-dir", required=True)
    args = parser.parse_args()

    out = Path(args.out_dir)
    out.mkdir(parents=True, exist_ok=True)
    record: dict[str, Any] = {"legs": {}, "started_at": time.strftime("%Y-%m-%dT%H:%M:%S%z")}

    # Reproduce the r2 precondition: activate workspace A, then run the round
    # on project B (chat-start activation no-ops on the stale identity, the
    # chat-tail sync crosses the identity boundary).
    open_project(args.agent_http, args.project_b, args.timeout)
    open_project(args.agent_http, args.project_a, args.timeout)

    conversation = f"gov_ttl_replay_{int(time.time())}"
    resp = frozen.chat(args.agent_http, conversation, T7_MESSAGE, {}, args.chat_timeout)
    record["turn1"] = {"conversation": conversation, "needs_confirmation": resp.get("needs_confirmation"),
                       "stop_reason": resp.get("stop_reason"), "goal_status": resp.get("goal_status")}
    interaction_id = first_improvement_interaction(resp)
    if not interaction_id:
        record["failure_class"] = "no_card_parked"
        (out / "replay_record.json").write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
        print("FAIL no_card_parked: chat did not park an improvement_proposal card", file=sys.stderr)
        return 2

    # Leg 1 — the r2 shape: approve right after the chat tail crossed the
    # workspace identity boundary. Must route, not hit any catch-all class.
    leg1 = frozen.interaction_respond(args.agent_http, interaction_id, "approve", "approve", args.chat_timeout)
    record["legs"]["late_approve"] = leg1
    leg1_reply = str(leg1.get("reply", ""))
    leg1_stop = str(leg1.get("stop_reason", ""))
    leg1_hit_catch_all = any(marker in leg1_reply or marker == leg1_stop for marker in CATCH_ALL_MARKERS)
    record["legs"]["late_approve_catch_all"] = leg1_hit_catch_all
    if leg1_hit_catch_all:
        record["failure_class"] = "approve_lost"

    # Leg 2 — repeated approve on the consumed card: explicit idempotent receipt.
    leg2 = frozen.interaction_respond(args.agent_http, interaction_id, "approve", "approve", args.timeout)
    record["legs"]["repeat_approve"] = leg2
    leg2_ok = (str(leg2.get("stop_reason", "")) == "interaction_already_retired"
               and str(leg2.get("goal_status", "")) == "" and "未执行" in str(leg2.get("reply", "")))

    # Leg 3 — unknown id: explicit not-found receipt, never a fabricated completion.
    unknown_id = f"interaction_govttl_probe_{int(time.time())}"
    leg3 = frozen.interaction_respond(args.agent_http, unknown_id, "approve", "approve", args.timeout)
    record["legs"]["unknown_approve"] = leg3
    leg3_ok = (str(leg3.get("stop_reason", "")) == "interaction_not_found"
               and str(leg3.get("goal_status", "")) == "" and "未执行" in str(leg3.get("reply", "")))

    if not leg2_ok:
        record["failure_class"] = record.get("failure_class") or "receipt_wrong"
    if not leg3_ok:
        record["failure_class"] = record.get("failure_class") or "receipt_wrong"

    record["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S%z")
    (out / "replay_record.json").write_text(json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
    ok = (not leg1_hit_catch_all) and leg2_ok and leg3_ok
    print("PASS" if ok else f"FAIL {record.get('failure_class', 'unknown')}")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
