#!/usr/bin/env python3
"""Chat-chain authority stamp smoke (M3-CHAT-AUTH-STAMP-1, card 223).

Same stack shape as automation_intent_chain_smoke.py (218): kernel + agent via
dev_agent_smoke.ps1 -StartKernel -RestartAgent, no UI, isolated project
(VIT_PROJECT_XML run-dir copy). Every assertion leg drives the real agent
/agent/chat and /agent/authority HTTP faces against the running stack. The
whole chain is LLM-free (deterministic vocabulary routing takes over before
any model turn; zero LLM participation, AGENTS §8).

The card's real-stack obligations, one leg each:

- full access selected through the authority control -> a choreography phrase
  (bare request: no authority_mode anywhere in body or context — the
  any-transport shape) executes autonomously: no confirmation card, autonomy
  markers disclosed, the governed write settles with kernel readback
  (phase "before", leg full_direct).
- after a REAL agent process restart the durable authority restore still
  direct-executes: the span-candidate adoption face ("副歌那 25 秒" -> clarify
  -> "可以") runs with no card and writes the envelope (phase "after", legs
  restart_restore + full_span_direct).
- switching back to manual_confirmation restores the confirmation card with
  zero writes (leg manual_card), and a forged authority_mode=full_project_access
  in the chat request body never authorizes: the turn is rejected (409), the
  server state stays manual, the curve stays empty (leg forged_body).

Exit code 0 only if every leg of the phase passes; summary JSON at --output.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)

CHAT_TIMEOUT_SEC = 60.0


def http_json(method: str, url: str, payload: Optional[Dict[str, Any]], timeout: float) -> Tuple[int, Dict[str, Any]]:
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8") if payload is not None else None
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method=method)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            body = response.read().decode("utf-8")
            code = response.status
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        code = exc.code
    try:
        parsed = json.loads(body)
    except json.JSONDecodeError:
        raise RuntimeError(f"{method} {url} returned non-JSON (HTTP {code}): {body[:800]}") from None
    if not isinstance(parsed, dict):
        raise RuntimeError(f"{method} {url} returned non-object JSON: {body[:400]}")
    return code, parsed


def chat(base_url: str, conversation: str, message: str, context: Dict[str, Any],
         authority_mode: str = "", timeout: float = CHAT_TIMEOUT_SEC) -> Tuple[int, Dict[str, Any]]:
    payload: Dict[str, Any] = {"conversation_id": conversation, "message": message, "context": context}
    if authority_mode:
        payload["authority_mode"] = authority_mode
    return http_json("POST", base_url.rstrip("/") + "/agent/chat", payload, timeout)


def authority_get(base_url: str) -> Dict[str, Any]:
    code, body = http_json("GET", base_url.rstrip("/") + "/agent/authority", None, 30.0)
    if code != 200 or body.get("status") != "ok":
        raise RuntimeError(f"GET /agent/authority failed: HTTP {code} {json.dumps(body, ensure_ascii=False)[:400]}")
    return body


def wait_shadow_tracks(base_url: str, want_tracks: int, timeout_s: float = 60.0) -> None:
    """Wait until the agent's shadow has bound the kernel's CURRENT project.

    The workspace/authority state is per project identity: switching the
    kernel project (new_project/save_as) re-binds the agent identity and the
    explicit authority latch correctly drops on that switch (AUTHORITY-LOST-1
    anti-leak; 219 face-5 window (iii)). Any authority write or assertion must
    therefore happen only after the shadow reports the new project's track
    count, or the switch lands on the previous identity's workspace and the
    restart leg honestly comes back manual (round-1 finding).
    """
    deadline = time.time() + timeout_s
    last: str = ""
    while time.time() < deadline:
        code, state = http_json("GET", base_url.rstrip("/") + "/agent/state", None, 10.0)
        shadow = state.get("shadow") or {}
        if code == 200 and shadow.get("initialized") is True:
            count = shadow.get("track_count")
            if isinstance(count, int) and count == want_tracks:
                return
            last = f"track_count={count!r}"
        else:
            last = f"shadow={json.dumps(shadow, ensure_ascii=False)[:120]}"
        time.sleep(0.5)
    raise RuntimeError(f"agent shadow never bound a project with {want_tracks} tracks: {last}")


def authority_post(base_url: str, mode: str) -> Dict[str, Any]:
    code, body = http_json("POST", base_url.rstrip("/") + "/agent/authority", {"authority_mode": mode}, 30.0)
    if code != 200 or body.get("status") != "ok" or body.get("authority_mode") != mode:
        raise RuntimeError(f"POST /agent/authority {mode} failed: HTTP {code} {json.dumps(body, ensure_ascii=False)[:500]}")
    return body


def zmq_command(url: str, payload: Dict[str, Any], timeout_ms: int = 60000) -> Dict[str, Any]:
    context = zmq.Context.instance()
    sock = context.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    try:
        sock.connect(url)
        sock.send_json(payload)
        return json.loads(sock.recv_string())
    finally:
        sock.close(0)


def wait_agent_http(base_url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            code, health = http_json("GET", base_url.rstrip("/") + "/health", None, 2.0)
            if code == 200 and str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
            last_error = RuntimeError(f"health replied HTTP {code}: {health}")
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"agent HTTP did not become ready at {base_url}: {last_error}")


def wait_kernel_zmq(url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            reply = zmq_command(url, {"cmd": "ping"}, timeout_ms=5000)
            if str(reply.get("status", "")).lower() in {"ok", "success"}:
                return
            last_error = RuntimeError(f"ping replied {reply.get('status')!r}")
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise RuntimeError(message)


def approx(actual: Any, expected: float, tolerance: float = 1e-6) -> bool:
    try:
        return abs(float(actual) - expected) <= tolerance
    except (TypeError, ValueError):
        return False


def find_track_volume_plugin(state: Dict[str, Any], track_id: str) -> Optional[str]:
    """196/217 precedent: the track's built-in Volume & Pan plugin row."""
    tracks = state.get("tracks") if isinstance(state.get("tracks"), list) else []
    for raw_track in tracks:
        if not isinstance(raw_track, dict):
            continue
        candidate = str(raw_track.get("track_id") or raw_track.get("id") or raw_track.get("item_id") or "").strip()
        if candidate != track_id:
            continue
        plugins = raw_track.get("plugins")
        if not isinstance(plugins, list):
            return None
        for raw_plugin in plugins:
            if not isinstance(raw_plugin, dict):
                continue
            kind = (str(raw_plugin.get("name") or "") + "/" + str(raw_plugin.get("type") or "")).lower()
            if "volume" not in kind:
                continue
            plugin_id = str(raw_plugin.get("id") or raw_plugin.get("plugin_item_id") or raw_plugin.get("plugin_id") or "").strip()
            if plugin_id:
                return plugin_id
        return None
    return None


def kernel_curve(kernel_zmq: str, plugin_id: str, param_id: str) -> Dict[str, Any]:
    reply = zmq_command(kernel_zmq, {"cmd": "automation.get_curve", "plugin_id": plugin_id, "param_id": param_id, "command_timeout_ms": 60000})
    require(str(reply.get("status", "")).lower() in {"ok", "success"},
            f"kernel automation.get_curve failed: {json.dumps(reply, ensure_ascii=False)[:500]}")
    return reply


def kernel_point_count(kernel_zmq: str, plugin_id: str, param_id: str) -> int:
    curve = kernel_curve(kernel_zmq, plugin_id, param_id)
    points = curve.get("points")
    require(isinstance(points, list), f"kernel curve points not a list: {points!r}")
    return len(points)


def require_no_card(where: str, reply: Dict[str, Any]) -> None:
    interactions = reply.get("interaction_requests") or []
    require(not interactions, f"{where}: interaction card issued under full access: {json.dumps(interactions, ensure_ascii=False)[:600]}")
    require(reply.get("needs_confirmation") is not True, f"{where}: needs_confirmation=true under full access")
    workflow_data = reply.get("workflow_data") or {}
    require(workflow_data.get("full_access_auto_authorized") is True,
            f"{where}: full_access_auto_authorized missing: {json.dumps(workflow_data, ensure_ascii=False)[:400]}")
    require(workflow_data.get("confirmation_bypassed") == "full_project_access",
            f"{where}: confirmation_bypassed = {workflow_data.get('confirmation_bypassed')!r}")
    require(workflow_data.get("interaction_card_issued") is False,
            f"{where}: interaction_card_issued = {workflow_data.get('interaction_card_issued')!r}")


def require_governed_settle(where: str, reply: Dict[str, Any]) -> Dict[str, Any]:
    require(reply.get("stop_reason") == "automation_choreo_settled",
            f"{where}: stop_reason = {reply.get('stop_reason')!r}: {json.dumps(reply, ensure_ascii=False)[:900]}")
    workflow_data = reply.get("workflow_data") or {}
    require(workflow_data.get("mutation_performed") is True, f"{where}: mutation_performed missing")
    require(workflow_data.get("written") == 2, f"{where}: written = {workflow_data.get('written')!r}, want 2")
    require(workflow_data.get("mixboard_decision_status") == "recorded" and bool(workflow_data.get("mixboard_decision_record_id")),
            f"{where}: decision ledger = {workflow_data.get('mixboard_decision_status')}/{workflow_data.get('mixboard_decision_record_id')}")
    queue_c4 = workflow_data.get("workflow_queue_c4") or {}
    require(queue_c4.get("node_id") == "C4" and str(queue_c4.get("state", "")).lower() == "in_progress",
            f"{where}: C4 queue node = {json.dumps(queue_c4, ensure_ascii=False)[:300]}")
    return workflow_data


def require_curve_points(where: str, kernel_zmq: str, plugin_id: str, want: List[Tuple[float, float]]) -> None:
    points = (kernel_curve(kernel_zmq, plugin_id, "volume").get("points")) or []
    require(len(points) == len(want), f"{where}: kernel points = {len(points)}, want {len(want)}: {points!r}")
    for index, (t, v) in enumerate(want):
        require(approx(points[index].get("t"), t) and approx(points[index].get("v"), v),
                f"{where}: point#{index} = {points[index]!r}, want t={t} v={v}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--phase", required=True, choices=["before", "after"])
    parser.add_argument("--baseline-summary", default="",
                        help="phase-after only: the before-phase summary JSON carrying baseline_authority to restore")
    parser.add_argument("--timeout-sec", type=int, default=240)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    import tempfile
    project_path = Path(args.project_path).resolve() if args.project_path else Path(tempfile.gettempdir()) / f"chat_auth_stamp_{int(time.time())}.vit"

    legs: List[str] = []
    summary: Dict[str, Any] = {
        "scenario": "M3-CHAT-AUTH-STAMP-1 chat-chain authority stamp",
        "phase": args.phase,
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
        "project_path": str(project_path),
        "llm_participation": "none (deterministic vocabulary routing; authority faces are control-plane HTTP)",
        "started_at": datetime.now(timezone.utc).isoformat(),
        "legs": legs,
    }

    def record(name: str, detail: Dict[str, Any]) -> None:
        legs.append(name)
        summary[name] = detail
        print(f"[ok] {name}: {json.dumps(detail, ensure_ascii=False)[:400]}")

    try:
        wait_agent_http(args.agent_http, args.timeout_sec)
        wait_kernel_zmq(args.kernel_zmq, args.timeout_sec)

        if args.phase == "before":
            # ---- baseline + isolated project + three ride tracks ------------
            baseline = authority_get(args.agent_http)
            require(baseline.get("authority_mode") in {"manual_confirmation", "full_project_access"},
                    f"baseline authority_mode = {baseline.get('authority_mode')!r}")
            summary["baseline_authority"] = baseline.get("authority_mode")

            project_path.parent.mkdir(parents=True, exist_ok=True)
            reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
            require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"new_project failed: {reply}")
            reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": 60000})
            require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"save_as_project failed: {reply}")

            track_ids: List[str] = []
            for _ in range(3):
                reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
                track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
                require(track_id, f"add_audio_track returned no track_id: {reply}")
                track_ids.append(track_id)
            kernel_state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
            plugins = {}
            for label, track_id in zip(("a", "b", "c"), track_ids):
                plugin_id = find_track_volume_plugin(kernel_state, track_id)
                require(bool(plugin_id), f"no built-in Volume & Pan plugin on track {track_id}")
                require(kernel_point_count(args.kernel_zmq, plugin_id, "volume") == 0,
                        f"setup baseline: track {label} volume curve must start empty")
                plugins[label] = plugin_id
            record("setup", {"tracks": dict(zip(("a", "b", "c"), track_ids)), "volume_plugins": plugins})

            # ---- switch full access through the control plane ----------------
            # Only after the shadow has bound the run project identity (see
            # wait_shadow_tracks): a switch that races the identity re-bind
            # persists under the previous workspace and the restart leg comes
            # back manual (round-1 finding, faithful to the per-workspace
            # authority contract).
            wait_shadow_tracks(args.agent_http, 3)
            switch = authority_post(args.agent_http, "full_project_access")
            require(switch.get("persisted") is True,
                    f"full-access switch not durable (workspace_ready={switch.get('workspace_ready')!r}) — "
                    f"the restart leg cannot hold: {json.dumps(switch, ensure_ascii=False)[:400]}")
            require(authority_get(args.agent_http).get("authority_mode") == "full_project_access", "switch did not stick")
            record("switch_full", {"persisted": switch.get("persisted"), "workspace_ready": switch.get("workspace_ready")})

            # ---- full_direct: bare phrase, no authority anywhere --------------
            conv_a = "chat_auth_stamp_full_direct"
            code, proposed = chat(args.agent_http, conv_a, "0 到 30 秒，让这条轨的存在感慢慢强起来",
                                  {"agent_mode": "chat", "selected_track_id": track_ids[0]})
            require(code == 200, f"full_direct HTTP {code}: {json.dumps(proposed, ensure_ascii=False)[:600]}")
            require(proposed.get("workflow") == "section_automation_choreo",
                    f"full_direct workflow = {proposed.get('workflow')!r}")
            require_no_card("full_direct", proposed)
            require_governed_settle("full_direct", proposed)
            require("完全访问权限下编舞写入已自主执行" in str(proposed.get("reply") or ""),
                    f"full_direct autonomy notice missing from reply: {str(proposed.get('reply'))[:300]!r}")
            require_curve_points("full_direct", args.kernel_zmq, plugins["a"], [(0.0, 0.30), (30.0, 0.60)])
            record("full_direct", {
                "stop_reason": proposed.get("stop_reason"),
                "written": (proposed.get("workflow_data") or {}).get("written"),
                "reply_head": str(proposed.get("reply") or "")[:80],
            })
        else:
            # ---- phase after: the ps1 restarted the agent process ------------
            wait_shadow_tracks(args.agent_http, 3)
            restored = authority_get(args.agent_http)
            require(restored.get("authority_mode") == "full_project_access",
                    f"authority after agent restart = {restored.get('authority_mode')!r}, want full_project_access "
                    f"(durable restore leg)")
            record("restart_restore", {"authority_mode": restored.get("authority_mode")})

            kernel_state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
            tracks_now = [str(row.get("track_id") or row.get("id") or row.get("item_id") or "").strip()
                          for row in (kernel_state.get("tracks") or []) if isinstance(row, dict)]
            require(len(tracks_now) == 3, f"expected the 3 setup tracks after restart, got {len(tracks_now)}")
            plugins = {}
            for label, track_id in zip(("a", "b", "c"), tracks_now):
                plugin_id = find_track_volume_plugin(kernel_state, track_id)
                require(bool(plugin_id), f"track {label} lost its volume plugin after restart")
                plugins[label] = plugin_id
            require_curve_points("restart_track_a_holds", args.kernel_zmq, plugins["a"], [(0.0, 0.30), (30.0, 0.60)])

            # ---- full_span_direct: clarify span candidate -> 可以 (bare) ------
            conv_b = "chat_auth_stamp_full_span"
            code, clarified = chat(args.agent_http, conv_b, "副歌那 25 秒，让这条轨的存在感慢慢强起来",
                                   {"agent_mode": "chat", "selected_track_id": tracks_now[1]})
            require(code == 200, f"span clarify HTTP {code}: {json.dumps(clarified, ensure_ascii=False)[:600]}")
            require(clarified.get("stop_reason") == "automation_choreo_clarify_region_span_candidate",
                    f"span clarify stop_reason = {clarified.get('stop_reason')!r}")
            require(kernel_point_count(args.kernel_zmq, plugins["b"], "volume") == 0,
                    "span clarify must not write before adoption")
            code, adopted = chat(args.agent_http, conv_b, "可以", {"agent_mode": "chat", "selected_track_id": tracks_now[1]})
            require(code == 200, f"span adoption HTTP {code}: {json.dumps(adopted, ensure_ascii=False)[:600]}")
            require_no_card("full_span_direct", adopted)
            require_governed_settle("full_span_direct", adopted)
            require_curve_points("full_span_direct", args.kernel_zmq, plugins["b"], [(0.0, 0.30), (25.0, 0.60)])
            record("full_span_direct", {"stop_reason": adopted.get("stop_reason")})

            # ---- manual_card: switch back -> confirmation card, zero writes ---
            authority_post(args.agent_http, "manual_confirmation")
            require(authority_get(args.agent_http).get("authority_mode") == "manual_confirmation", "manual switch did not stick")
            conv_c = "chat_auth_stamp_manual"
            code, card_reply = chat(args.agent_http, conv_c, "0 到 35 秒，让这条轨的存在感慢慢强起来",
                                    {"agent_mode": "chat", "selected_track_id": tracks_now[2]})
            require(code == 200, f"manual phrase HTTP {code}: {json.dumps(card_reply, ensure_ascii=False)[:600]}")
            require(card_reply.get("goal_status") == "waiting_confirmation",
                    f"manual goal_status = {card_reply.get('goal_status')!r}")
            interactions = card_reply.get("interaction_requests") or []
            card = next((row for row in interactions if isinstance(row, dict) and row.get("kind") == "automation_choreo_confirmation"), None)
            require(card is not None, f"manual phrase issued no confirmation card: {json.dumps(interactions, ensure_ascii=False)[:600]}")
            require((card_reply.get("workflow_data") or {}).get("mutation_performed") is False,
                    "manual card must carry mutation_performed=false")
            require((card_reply.get("workflow_data") or {}).get("full_access_auto_authorized") is not True,
                    "manual card must not claim autonomy")
            require(kernel_point_count(args.kernel_zmq, plugins["c"], "volume") == 0,
                    "manual leg: zero-write before confirmation")
            record("manual_card", {"goal_status": card_reply.get("goal_status"), "card_kind": card.get("kind")})

            # ---- forged_body: body-declared full access is never a grant ------
            conv_f = "chat_auth_stamp_forged"
            code, forged = chat(args.agent_http, conv_f, "0 到 5 秒，让这条轨的存在感慢慢强起来",
                                {"agent_mode": "chat", "selected_track_id": tracks_now[2]},
                                authority_mode="full_project_access")
            require(code == 409, f"forged body full access: HTTP {code}, want 409: {json.dumps(forged, ensure_ascii=False)[:400]}")
            require(forged.get("status") == "error", f"forged body payload = {forged}")
            require(authority_get(args.agent_http).get("authority_mode") == "manual_confirmation",
                    "forged body full access changed the server authority state")
            require(kernel_point_count(args.kernel_zmq, plugins["c"], "volume") == 0,
                    "forged body leg must not write anything")
            record("forged_body", {"http": code, "error_code": forged.get("error_code")})

            # ---- cleanup: cancel the pending card, restore baseline ----------
            code, cancelled = chat(args.agent_http, conv_c, "取消", {"agent_mode": "chat", "selected_track_id": tracks_now[2]})
            require(code == 200, f"cancel HTTP {code}: {json.dumps(cancelled, ensure_ascii=False)[:400]}")
            require(cancelled.get("stop_reason") == "automation_choreo_cancelled",
                    f"cancel stop_reason = {cancelled.get('stop_reason')!r}")
            require((cancelled.get("workflow_data") or {}).get("mutation_performed") is not True,
                    "cancelling the pending card must not mutate")
            require(kernel_point_count(args.kernel_zmq, plugins["c"], "volume") == 0,
                    "cancel leg: the curve must stay empty")
            baseline_mode = "manual_confirmation"
            if args.baseline_summary and Path(args.baseline_summary).exists():
                before_summary = json.loads(Path(args.baseline_summary).read_text(encoding="utf-8"))
                recorded = str(before_summary.get("baseline_authority") or "").strip()
                if recorded in {"manual_confirmation", "full_project_access"}:
                    baseline_mode = recorded
            authority_post(args.agent_http, baseline_mode)
            record("cleanup", {"restored_authority": authority_get(args.agent_http).get("authority_mode")})

        summary["status"] = "PASS"
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
    except Exception as exc:  # noqa: BLE001 - smoke failure reporting.
        summary["status"] = "FAIL"
        summary["error"] = str(exc)
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print(f"[FAIL] {exc}", file=sys.stderr)
        Path(args.output).parent.mkdir(parents=True, exist_ok=True)
        Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        sys.exit(1)

    Path(args.output).parent.mkdir(parents=True, exist_ok=True)
    Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[PASS] chat auth stamp smoke ({args.phase} phase): all legs green")


if __name__ == "__main__":
    main()
