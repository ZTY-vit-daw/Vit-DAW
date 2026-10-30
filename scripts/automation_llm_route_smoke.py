#!/usr/bin/env python3
"""LLM-turn automation routing verification (M3-C4-AUTO-LLM-ROUTE-VERIFY-1).

Card 220. Verified face: the behavior of vocabulary-MISSING variant phrases
after they fall through the deterministic pre-model gates into a REAL LLM
chat turn — does the 217 system-prompt routing guidance line steer the model
to the automation tool chain (get_plugin_parameters -> set_points behind the
RiskConfirm confirmation chain -> get_curve readback), or does it honestly
report the boundary, without fabricating writes or detouring through static
tools.

Stack shape: same as automation_intent_chain_smoke.py (218) — kernel + agent
berth via dev_agent_smoke.ps1 -StartKernel -RestartAgent, no UI, isolated
project (VIT_PROJECT_XML). Setup uses the kernel ZMQ face (new project + two
bare audio tracks with the built-in Volume & Pan plugin, NO external plugin
loaded — the card's group-2 premise "space category without a plugin track").
Every round drives the real agent /agent/chat HTTP face with a REAL LLM turn
(the user's ~/.vit/config.json engine; zero deterministic takeover expected
because none of the three phrases arms the vocabulary gate — prechecked by
the lexicon-hit detector below, which classifies any deterministic takeover
round as lexicon_hit instead of an LLM sample).

Run matrix (card §8, fixed before execution):
  group 1 "这段让它越来越突出一点"  x5 rounds (level-family variant)
  group 2 "帮我把空间铺开"          x5 rounds (space family, no plugin)
  group 3 "30 秒之后动起来"         x5 rounds (boundary form)
  hard cap 15 LLM rounds; group success = >=4/5 correct-behavior rounds;
  stop-loss = 3 consecutive same-type failures inside a group (group aborts,
  evidence preserved, no further rounds spent).

Round verdict vocabulary (card §8 failure classes):
  tool_chain_ok      - model armed the automation chain and the confirmation
                       settle wrote verifiable points (success form)
  boundary_ok        - model honestly reported the missing scope/plugin, no
                       fabrication (success form for groups 1/2)
  lexicon_hit        - deterministic choreography took over before the LLM
                       (not an LLM sample; group 3 success form, groups 1/2
                       recorded as invalid_sample and re-sampled within the
                       15-round LLM cap since it costs no LLM turn)
  no_route           - plain-text advice/question, tools never touched
  wrong_tool         - detoured through a non-automation mutation tool
  confirm_broken     - confirmation chain failed (write without confirmation,
                       or a pending confirmation that cannot be settled)
  write_fail         - confirmation settled but the write/readback failed
  env_interrupt      - stack/LLM environment failure (kept only with raw
                       telemetry/log evidence on disk)

Choreo-pending non-interference leg (card subject 3), embedded at zero extra
LLM cost: before each group's round 5 a section-automation confirmation card
is armed (vocabulary phrase, its own conversation) and left pending; round 5's
LLM turn then runs in a separate conversation; afterwards the pending card
must still settle exactly like 218 LEG A (written==2, kernel fade-in points
0.30 -> 0.60). Three embeddings = one per group.

Exit codes (verification card, not a pass/fail production gate):
  0 - verification executed to completion (all planned rounds or stop-loss);
      model-behavior findings live in the summary artifact, defects fold to
      follow-up cards per the card's zero-production-change default
  2 - stop condition 2: chat face has no system-prompt automation guidance
      rendered (deployment defect, anchor for the decision side)
  3 - stop condition 1: LLM/stack environment unavailable (env interrupt)
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)

CHAT_TIMEOUT_SEC = 300.0          # one LLM turn with tool loops can be slow
FOLLOWUP_MAX = 3                  # extra chat calls inside one round
CONFIRM_PHRASE = "确认写入"
REGION_FOLLOWUP = "0 到 30 秒"

GROUP_PHRASES = {
    1: "这段让它越来越突出一点",
    2: "帮我把空间铺开",
    3: "30 秒之后动起来",
}

# 218-verified vocabulary phrase used only by the choreo-pending legs.
CHOREO_ARM_PHRASE = "0 到 30 秒，让这条轨的存在感慢慢强起来"


def post_json(base_url: str, path: str, payload: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        try:
            parsed = json.loads(body)
            if isinstance(parsed, dict):
                return parsed
        except json.JSONDecodeError:
            pass
        raise RuntimeError(f"{path} returned HTTP {exc.code}: {body[:800]}") from exc


def chat(base_url: str, conversation: str, message: str, context: Dict[str, Any]) -> Dict[str, Any]:
    return post_json(base_url, "/agent/chat", {
        "conversation_id": conversation,
        "message": message,
        "context": context,
    }, CHAT_TIMEOUT_SEC)


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
            with urllib.request.urlopen(base_url.rstrip("/") + "/health", timeout=2.0) as response:
                health = json.loads(response.read().decode("utf-8"))
            if str(health.get("status", "")).lower() in {"ok", "ready"}:
                return
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


def find_track_volume_plugin(state: Dict[str, Any], track_id: str) -> Optional[str]:
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
    points = kernel_curve(kernel_zmq, plugin_id, param_id).get("points")
    require(isinstance(points, list), "kernel curve points not a list")
    return len(points)


def kernel_clear_curve(kernel_zmq: str, plugin_id: str, param_id: str) -> None:
    reply = zmq_command(kernel_zmq, {"cmd": "automation.clear_region", "plugin_id": plugin_id, "param_id": param_id,
                                     "region": {"start": 0.0, "end": 100000.0}, "command_timeout_ms": 60000})
    require(str(reply.get("status", "")).lower() in {"ok", "success"},
            f"kernel automation.clear_region failed: {json.dumps(reply, ensure_ascii=False)[:500]}")


# ---- response classification ------------------------------------------------

def response_text(resp: Dict[str, Any]) -> str:
    return str(resp.get("reply") or resp.get("message") or "")


def response_blob(resp: Dict[str, Any]) -> str:
    """Serialized response for tool-name evidence greps (schema-tolerant)."""
    try:
        return json.dumps(resp, ensure_ascii=False)
    except (TypeError, ValueError):
        return response_text(resp)


def has_confirm_card(resp: Dict[str, Any]) -> bool:
    interactions = resp.get("interaction_requests")
    if isinstance(interactions, list):
        for row in interactions:
            if not isinstance(row, dict):
                continue
            kind = str(row.get("kind") or "")
            if "confirm" in kind.lower():
                return True
    for key in ("stop_reason", "goal_status"):
        value = str(resp.get(key) or "").lower()
        if "confirm" in value:
            return True
    blob = response_blob(resp)
    if '"needs_confirmation"' in blob or "automation_choreo_awaiting_confirmation" in blob:
        return True
    if "automation.set_points" in blob and "needs_confirmation" in blob:
        return True
    return False


def looks_clarification_pause(resp: Dict[str, Any]) -> bool:
    """Agent-loop clarification pause (interaction_requests without a
    confirmation kind) is a structured ask, not plain advice."""
    stop = str(resp.get("stop_reason") or "").lower()
    status = str(resp.get("goal_status") or "").lower()
    if "clarif" in stop or "clarif" in status:
        return True
    interactions = resp.get("interaction_requests")
    if isinstance(interactions, list) and interactions:
        has_confirm = any(isinstance(row, dict) and "confirm" in str(row.get("kind") or "").lower()
                          for row in interactions)
        if not has_confirm:
            return True
    return False


def looks_lexicon_hit(resp: Dict[str, Any]) -> bool:
    if resp.get("workflow") == "section_automation_choreo":
        return True
    stop = str(resp.get("stop_reason") or "")
    return stop.startswith("automation_choreo")


def looks_asking(text: str) -> bool:
    if "？" in text or "?" in text:
        return True
    return bool(re.search(r"(告诉|请问|哪一|哪个|是什么|需要.*吗|要不要)", text))


def looks_boundary(text: str) -> bool:
    return bool(re.search(r"(没有|未|无法|不支持|缺|不存在|需要先|没有挂|未挂).{0,24}(插件|效果器|空间)", text)
                or re.search(r"(插件|效果器).{0,16}(没有|未|不在|不存在)", text))


def mentions_automation_tools(blob: str) -> bool:
    return ("automation.set_points" in blob or "automation_set_points" in blob
            or "automation.get_curve" in blob or "automation.get_plugin_parameters" in blob
            or "get_plugin_parameters" in blob)


def mentions_detour_tools(blob: str) -> List[str]:
    detours: List[str] = []
    for name in ("set_plugin_param", "clip.fade", "set_volume", "set_track_volume"):
        if name in blob:
            detours.append(name)
    return detours


def looks_env_broken(resp: Dict[str, Any]) -> Optional[str]:
    err = str(resp.get("error") or "")
    text = response_text(resp)
    combined = (err + " " + text).lower()
    if "config is incomplete" in combined or "llm_config_incomplete" in combined:
        return "llm_config_incomplete"
    if "call failed" in combined and ("api" in combined or "connection" in combined or "timeout" in combined or "401" in combined or "402" in combined or "429" in combined):
        return "llm_call_failed"
    return None


# ---- round execution --------------------------------------------------------

def run_round(agent_http: str, kernel_zmq: str, group: int, index: int,
              phrase: str, chat_context: Dict[str, Any],
              ride_plugin_id: str, rounds_dir: Path) -> Dict[str, Any]:
    """One LLM round: phrase -> classify -> followups -> verdict -> cleanup.

    Returns the round record (phrase, exchanges, verdict, kernel evidence).
    Never raises for model-behavior outcomes; only stack errors propagate.
    """
    conversation = f"llm_route_g{group}_r{index}_{int(time.time())}"
    exchanges: List[Dict[str, Any]] = []
    record: Dict[str, Any] = {
        "group": group, "round": index, "phrase": phrase, "conversation": conversation,
        "exchanges": exchanges, "verdict": None, "verdict_detail": "",
        "kernel_points_before": kernel_point_count(kernel_zmq, ride_plugin_id, "volume"),
    }

    def send(message: str) -> Dict[str, Any]:
        resp = chat(agent_http, conversation, message, chat_context)
        exchanges.append({"sent": message, "response": resp})
        return resp

    resp = send(phrase)

    # environment breakage check first (stop condition 1 evidence)
    env_break = looks_env_broken(resp)
    if env_break:
        record["verdict"] = "env_interrupt"
        record["verdict_detail"] = env_break
        return record

    # deterministic takeover is not an LLM sample
    if looks_lexicon_hit(resp):
        record["verdict"] = "lexicon_hit"
        record["verdict_detail"] = f"stop_reason={resp.get('stop_reason')!r} workflow={resp.get('workflow')!r}"
        return record

    followups = 0
    while True:
        blob = response_blob(resp)
        text = response_text(resp)
        points_now = kernel_point_count(kernel_zmq, ride_plugin_id, "volume")

        if has_confirm_card(resp):
            # confirmation face present: zero-write before confirm is asserted
            pre_confirm_zero = points_now == 0
            record["pre_confirm_zero_write"] = pre_confirm_zero
            if not pre_confirm_zero:
                record["verdict"] = "confirm_broken"
                record["verdict_detail"] = "kernel curve mutated while a confirmation was still pending"
                break
            if followups >= FOLLOWUP_MAX:
                record["verdict"] = "confirm_broken"
                record["verdict_detail"] = "confirmation pending but followup budget exhausted"
                break
            resp = send(CONFIRM_PHRASE)
            followups += 1
            after = kernel_point_count(kernel_zmq, ride_plugin_id, "volume")
            settle_blob = response_blob(resp)
            if after > 0:
                record["kernel_points_after_confirm"] = kernel_curve(kernel_zmq, ride_plugin_id, "volume").get("points")
                record["settle_stop_reason"] = resp.get("stop_reason")
                record["settle_mentions_get_curve"] = "automation.get_curve" in settle_blob
                if mentions_detour_tools(settle_blob) and not mentions_automation_tools(settle_blob) and after == 0:
                    record["verdict"] = "wrong_tool"
                    record["verdict_detail"] = f"settle detoured: {mentions_detour_tools(settle_blob)}"
                else:
                    record["verdict"] = "tool_chain_ok"
                    record["verdict_detail"] = f"confirmed write reached kernel (points={after})"
                break
            # confirm phrase did not settle: one plain retry then classify
            if followups >= FOLLOWUP_MAX or "确认" not in response_text(resp):
                record["verdict"] = "confirm_broken"
                record["verdict_detail"] = f"confirm phrase did not settle (stop_reason={resp.get('stop_reason')!r}, reply={response_text(resp)[:200]!r})"
                break
            continue

        if points_now > 0:
            # kernel mutated without any confirmation face in view
            record["verdict"] = "confirm_broken"
            record["verdict_detail"] = f"kernel curve mutated ({points_now} points) with no confirmation face"
            break

        detours = mentions_detour_tools(blob)
        if detours:
            record["verdict"] = "wrong_tool"
            record["verdict_detail"] = f"detour tools in response: {detours}; automation tools={mentions_automation_tools(blob)}"
            break

        if looks_boundary(text):
            record["verdict"] = "boundary_ok"
            record["verdict_detail"] = f"honest boundary reply (no write): {text[:200]!r}"
            break

        if (looks_asking(text) or looks_clarification_pause(resp)) and followups < FOLLOWUP_MAX:
            resp = send(REGION_FOLLOWUP)
            followups += 1
            continue

        # plain text with no tool face, no boundary, no question
        record["verdict"] = "no_route"
        record["verdict_detail"] = f"plain-text response, no automation tool face: {text[:200]!r}"
        break

    (rounds_dir / f"g{group}_r{index}.json").write_text(
        json.dumps(record, ensure_ascii=False, indent=2), encoding="utf-8")
    # restore the empty baseline for the next round
    if kernel_point_count(kernel_zmq, ride_plugin_id, "volume") > 0:
        kernel_clear_curve(kernel_zmq, ride_plugin_id, "volume")
    return record


def settle_choreo_pending_leg(agent_http: str, kernel_zmq: str, group: int, conversation: str,
                              chat_context: Dict[str, Any], ride_plugin_id: str,
                              llm_round_record: Dict[str, Any], rounds_dir: Path) -> Dict[str, Any]:
    """Card subject 3 settle half: the choreo card the caller armed BEFORE the
    group's round-5 LLM turn (separate conversation) must still settle through
    the plain confirmation phrase and verify the 218 LEG A shape (written==2,
    fade-in 0.30 -> 0.60 over 0-30s)."""
    leg: Dict[str, Any] = {"group": group, "conversation": conversation,
                           "interleaved_llm_round": {
                               "conversation": llm_round_record.get("conversation"),
                               "verdict": llm_round_record.get("verdict"),
                           }}
    settle = chat(agent_http, conversation, CONFIRM_PHRASE, chat_context)
    leg["settle_response"] = settle
    points = kernel_curve(kernel_zmq, ride_plugin_id, "volume").get("points") or []
    leg["kernel_points"] = points
    ok = str(settle.get("stop_reason") or "") == "automation_choreo_settled" and len(points) == 2
    leg["verdict"] = "pending_intact" if ok else "pending_broken"
    kernel_clear_curve(kernel_zmq, ride_plugin_id, "volume")
    (rounds_dir / f"choreo_pending_g{group}.json").write_text(
        json.dumps(leg, ensure_ascii=False, indent=2), encoding="utf-8")
    return leg


def aggregate_usage(telemetry_path: Optional[Path]) -> Dict[str, Any]:
    if not telemetry_path or not telemetry_path.exists():
        return {"telemetry_path": str(telemetry_path), "available": False}
    calls = 0
    models: Dict[str, Dict[str, int]] = {}
    try:
        for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            calls += 1
            model = str(row.get("model") or "unknown")
            usage = row.get("usage") or {}
            bucket = models.setdefault(model, {"calls": 0, "input_tokens": 0, "output_tokens": 0})
            bucket["calls"] += 1
            bucket["input_tokens"] += int(usage.get("input_tokens") or 0)
            bucket["output_tokens"] += int(usage.get("output_tokens") or 0)
    except OSError as exc:
        return {"telemetry_path": str(telemetry_path), "available": False, "error": str(exc)}
    return {"telemetry_path": str(telemetry_path), "available": True,
            "llm_calls": calls, "models": models}


def check_prompt_guidance(prompt_render_path: Optional[Path]) -> bool:
    """Stop condition 2 anchor: the rendered chat system prompt must carry the
    217 automation routing guidance (both the legacy template and the embed
    source are byte-locked in production tests; here we check the live render
    actually reached the model)."""
    if not prompt_render_path or not prompt_render_path.exists():
        return False
    try:
        blob = prompt_render_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return False
    return "automation.set_points" in blob and "automation.get_curve" in blob


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--timeout-sec", type=int, default=600)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", required=True)
    parser.add_argument("--rounds-dir", required=True)
    parser.add_argument("--telemetry-path", default="")
    parser.add_argument("--prompt-render-path", default="")
    parser.add_argument("--rounds-per-group", type=int, default=5)
    args = parser.parse_args()

    import tempfile
    project_path = Path(args.project_path).resolve() if args.project_path else Path(tempfile.gettempdir()) / f"automation_llm_route_{int(time.time())}.vit"
    rounds_dir = Path(args.rounds_dir).resolve()
    rounds_dir.mkdir(parents=True, exist_ok=True)
    telemetry_path = Path(args.telemetry_path).resolve() if args.telemetry_path else None
    prompt_render_path = Path(args.prompt_render_path).resolve() if args.prompt_render_path else None

    summary: Dict[str, Any] = {
        "scenario": "M3-C4-AUTO-LLM-ROUTE-VERIFY-1 LLM-turn automation routing",
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
        "project_path": str(project_path),
        "llm_participation": "real (user engine config; every classified round below is a real billed LLM turn unless marked lexicon_hit)",
        "started_at": datetime.now(timezone.utc).isoformat(),
        "groups": {},
        "rounds": [],
    }

    exit_code = 0
    try:
        wait_agent_http(args.agent_http, args.timeout_sec)
        wait_kernel_zmq(args.kernel_zmq, args.timeout_sec)

        # ---- setup: isolated project + two bare audio tracks --------------
        project_path.parent.mkdir(parents=True, exist_ok=True)
        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"save_as_project failed: {reply}")
        track_ids: List[str] = []
        for _ in range(2):
            reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
            track_id = str(reply.get("track_id") or reply.get("id") or "").strip()
            require(track_id, f"add_audio_track returned no track_id: {reply}")
            track_ids.append(track_id)
        ride_track_id = track_ids[1]
        kernel_state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        ride_plugin_id = find_track_volume_plugin(kernel_state, ride_track_id)
        require(bool(ride_plugin_id), "no built-in Volume & Pan plugin on the ride track")
        require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 0,
                "setup baseline: the volume curve must start empty")
        summary["setup"] = {"ride_track_id": ride_track_id, "ride_volume_plugin_id": ride_plugin_id,
                            "external_plugins_loaded": 0}
        print(f"[setup] ride track {ride_track_id} volume plugin {ride_plugin_id} (no external plugin, card group-2 premise)")

        chat_context = {"agent_mode": "chat", "selected_track_id": ride_track_id}
        stop_loss_hit: Optional[Dict[str, Any]] = None

        for group in (1, 2, 3):
            phrase = GROUP_PHRASES[group]
            group_rounds: List[Dict[str, Any]] = []
            consecutive_same_failure: Dict[str, int] = {}
            llm_samples = 0
            attempt = 0
            attempt_cap = 2 * args.rounds_per_group + 3
            group_aborted = False
            lexicon_overflow = False
            pending_leg = None
            while llm_samples < args.rounds_per_group:
                if attempt >= attempt_cap:
                    # groups 1/2 only: re-sampling cannot find LLM samples
                    # because every attempt was taken over deterministically
                    lexicon_overflow = True
                    break
                attempt += 1
                # the last LLM sample of each group runs inside a choreo-pending
                # window: arm BEFORE the round, settle AFTER it (card subject 3)
                pending_conv = None
                if llm_samples == args.rounds_per_group - 1:
                    pending_conv = f"choreo_pending_g{group}"
                    arm = chat(args.agent_http, pending_conv, CHOREO_ARM_PHRASE, chat_context)
                    require(looks_lexicon_hit(arm),
                            f"choreo arm phrase did not take over: {json.dumps(arm, ensure_ascii=False)[:400]}")
                    require(kernel_point_count(args.kernel_zmq, ride_plugin_id, "volume") == 0,
                            "choreo pending armed but curve not empty")
                record = run_round(args.agent_http, args.kernel_zmq, group, attempt,
                                   phrase, chat_context, ride_plugin_id, rounds_dir)
                if pending_conv is not None:
                    pending_leg = settle_choreo_pending_leg(
                        args.agent_http, args.kernel_zmq, group, pending_conv,
                        chat_context, ride_plugin_id, record, rounds_dir)
                summary["rounds"].append(record)
                if record["verdict"] == "lexicon_hit":
                    # deterministic takeover: not an LLM sample, no LLM cost;
                    # success form for group 3, invalid sample for groups 1/2
                    record["counts_as_llm_sample"] = False
                    if group == 3:
                        group_rounds.append(record)
                        llm_samples += 1
                    continue
                record["counts_as_llm_sample"] = True
                group_rounds.append(record)
                llm_samples += 1

                if record["verdict"] == "env_interrupt":
                    # environment break: abort everything, keep raw evidence
                    summary["env_interrupt"] = {
                        "group": group, "round": attempt, "detail": record["verdict_detail"],
                        "evidence": "raw exchanges on disk + agent telemetry",
                    }
                    exit_code = 3
                    break

                # stop-loss: 3 consecutive same-type failures inside the group
                failure_types = {"no_route", "wrong_tool", "confirm_broken", "write_fail"}
                if record["verdict"] in failure_types:
                    consecutive_same_failure[record["verdict"]] = consecutive_same_failure.get(record["verdict"], 0) + 1
                    for other in list(consecutive_same_failure):
                        if other != record["verdict"]:
                            consecutive_same_failure[other] = 0
                    if consecutive_same_failure[record["verdict"]] >= 3:
                        stop_loss_hit = {"group": group, "failure_type": record["verdict"],
                                         "consecutive": consecutive_same_failure[record["verdict"]]}
                        group_aborted = True
                        break
                else:
                    consecutive_same_failure = {}

            verdicts = [r["verdict"] for r in group_rounds]
            success_types = {"tool_chain_ok", "boundary_ok"} | ({"lexicon_hit"} if group == 3 else set())
            successes = sum(1 for v in verdicts if v in success_types)
            group_result = {
                "phrase": phrase,
                "llm_samples": llm_samples,
                "verdicts": verdicts,
                "successes": successes,
                "group_pass": (successes >= 4) and not group_aborted and not lexicon_overflow,
                "stop_loss": stop_loss_hit if (stop_loss_hit and stop_loss_hit.get("group") == group) else None,
                "lexicon_overflow": lexicon_overflow,
                "choreo_pending_leg": None,
            }
            if pending_leg is not None:
                group_result["choreo_pending_leg"] = {k: pending_leg[k] for k in ("verdict", "group")}
            summary["groups"][f"group_{group}"] = group_result
            print(f"[group {group}] {json.dumps(group_result, ensure_ascii=False)[:400]}")
            if exit_code == 3:
                for remaining in (group + 1, group + 2):
                    if remaining in (1, 2, 3):
                        summary["groups"][f"group_{remaining}"] = {
                            "phrase": GROUP_PHRASES[remaining], "skipped": "env_interrupt_in_earlier_group",
                        }
                break
            if group_aborted or lexicon_overflow:
                # stop-loss/overflow ends this group only; remaining groups are
                # not run (card: stop, fold the failure type, spend no rounds)
                for remaining in (group + 1, group + 2):
                    if remaining in (1, 2, 3):
                        summary["groups"][f"group_{remaining}"] = {
                            "phrase": GROUP_PHRASES[remaining], "skipped": "stop_loss_in_earlier_group",
                        }
                break

        # ---- guidance render check (stop condition 2) ----------------------
        if prompt_render_path is not None:
            summary["prompt_guidance_rendered"] = check_prompt_guidance(prompt_render_path)
            if not summary["prompt_guidance_rendered"]:
                exit_code = 2
        summary["usage"] = aggregate_usage(telemetry_path)
        summary["status"] = "ENV_INTERRUPT" if exit_code == 3 else ("GUIDANCE_MISSING" if exit_code == 2 else "COMPLETED")
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
    except Exception as exc:  # noqa: BLE001 - smoke failure reporting.
        summary["status"] = "ERROR"
        summary["error"] = str(exc)
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print(f"[FAIL] {exc}", file=sys.stderr)
        Path(args.output).parent.mkdir(parents=True, exist_ok=True)
        Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        sys.exit(1)

    Path(args.output).parent.mkdir(parents=True, exist_ok=True)
    Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"[DONE] status={summary['status']} exit={exit_code}")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
