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
audio tracks with the built-in Volume & Pan plugin, NO external plugin
loaded — the card's group-2 premise "space category without a plugin track").
Since card 230 (option A) each track carries a deterministic generated audio
clip (see write_deterministic_stereo_wav) so the derived observation layer has
real source evidence: the 229 root cause was that bare tracks left the
acoustic package missing and the model correctly stopped at the FS evidence
admission gate (capability_blocked) on every round. Setup verifies the three
evidence links before any LLM round is spent — clip present in kernel state ->
track-level CCB observation returns facts (bundle.views, not omissions) ->
acoustic package status excerpted from .vit_derived into the rounds dir.
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
  capability_blocked - model honestly declared the FS evidence admission gate
                       cannot be satisfied on the current observation
                       evidence (no fabricated write, no detour); its own
                       failure class since card 230 (option C) — 229
                       mis-filed these declarations as no_route because the
                       boundary lexicon never matched the declaration wording
  in_progress_timeout - card 232: the bounded terminal-state wait window
                       expired with the round still in progress (no
                       confirmation card, no completion, no capability
                       declaration) — filed honestly instead of forcing a
                       no_route verdict onto a queued continuation
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
import math
import re
import struct
import sys
import time
import urllib.error
import urllib.request
import wave
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

# Deterministic clip fixture (card 230 option A): self-contained, generated
# into the run artifacts dir, never the repo sample projects (§10).
CLIP_WAV_NAME = "automation_llm_route_clip.wav"
CLIP_SAMPLE_RATE = 48000
CLIP_DURATION_SECONDS = 6.0
# Evidence-link polling window: the import bake (waveform envelope) and the
# observation background fill are asynchronous; 229-grade fixtures settle in
# seconds, the window only bounds the wait.
ACOUSTIC_READY_TIMEOUT_SEC = 240.0
ACOUSTIC_READY_POLL_INTERVAL_SEC = 5.0
# Async-continuation receipt grace: after a queued-continuation reply the
# free-state loop receipt may still be settling when the reply arrives.
CAPABILITY_RECEIPT_GRACE_SEC = 15.0
# Terminal-state wait window (card 232): progress/queued replies (limit_reached
# continuation notice / automatic_continuation_already_scheduled) no longer
# classify as no_route on sight. The async continuation settles the round
# toward its natural stopping point (governance confirmation card / completion
# report / capability declaration); the smoke polls the agent runtime status
# inside this bounded window and dispatches on whichever terminal state lands.
TERMINAL_WAIT_TIMEOUT_SEC = 900.0
TERMINAL_WAIT_POLL_INTERVAL_SEC = 10.0

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


# ---- deterministic clip fixture (card 230 option A) --------------------------

def write_deterministic_stereo_wav(path: Path) -> None:
    """48 kHz / 16-bit stereo, fixed 6.0 s, no randomness: a 220 Hz base tone
    plus a rising high band (1760 Hz left / 1320 Hz right) under a rising
    envelope, so band-dynamics, level-trend and stereo observations all have
    real content to read. Pure stdlib; the file lives in the run artifacts
    dir only."""
    n_frames = int(CLIP_SAMPLE_RATE * CLIP_DURATION_SECONDS)
    frames = bytearray()
    for i in range(n_frames):
        t = i / CLIP_SAMPLE_RATE
        envelope = 0.25 + 0.55 * (t / CLIP_DURATION_SECONDS)
        left = envelope * (0.5 * math.sin(2 * math.pi * 220.0 * t) + 0.3 * math.sin(2 * math.pi * 1760.0 * t))
        right = envelope * (0.45 * math.sin(2 * math.pi * 220.0 * t) + 0.35 * math.sin(2 * math.pi * 1320.0 * t))
        frames += struct.pack("<hh",
                              int(max(-1.0, min(1.0, left)) * 32000),
                              int(max(-1.0, min(1.0, right)) * 32000))
    with wave.open(str(path), "wb") as handle:
        handle.setnchannels(2)
        handle.setsampwidth(2)
        handle.setframerate(CLIP_SAMPLE_RATE)
        handle.writeframes(bytes(frames))


def agent_invoke(base_url: str, tool: str, args: Dict[str, Any], timeout: float) -> Dict[str, Any]:
    """Deterministic read-only agent tool call (no LLM turn): the same
    /agent/invoke face the b4/CCB smokes use for fixtures."""
    return post_json(base_url, "/agent/invoke", {
        "tool": tool, "args": args,
        "source": "automation_llm_route_smoke.setup", "confirmed": False,
    }, timeout)


def track_clip_count(state: Dict[str, Any], track_id: str) -> int:
    tracks = state.get("tracks") if isinstance(state.get("tracks"), list) else []
    for raw_track in tracks:
        if not isinstance(raw_track, dict):
            continue
        candidate = str(raw_track.get("track_id") or raw_track.get("id") or raw_track.get("item_id") or "").strip()
        if candidate != track_id:
            continue
        clips = raw_track.get("clips")
        return len(clips) if isinstance(clips, list) else 0
    return 0


def request_track_observation_bundle(agent_http: str, track_id: str, timeout: float) -> Dict[str, Any]:
    """Track-level CCB observation over the same face the model uses. Returns
    the raw bundle map (status/bundle shape follows ccb_observation_bundle.v1;
    /agent/invoke wraps the tool result under "result", as in the b4/CCB
    smokes)."""
    response = agent_invoke(agent_http, "ccb.observation_request", {
        "request_id": f"setup-evidence-{int(time.time())}",
        "view_ids": ["track.basic_energy"],
        "target_kind": "track",
        "target_id": track_id,
        "max_disclosure_bytes": 8192,
    }, timeout)
    result = response.get("result") if isinstance(response.get("result"), dict) else response
    bundle = result.get("bundle")
    require(isinstance(bundle, dict), f"ccb.observation_request returned no bundle: {json.dumps(response, ensure_ascii=False)[:500]}")
    return bundle


def wait_track_observation_facts(agent_http: str, track_id: str,
                                 timeout_sec: float = ACOUSTIC_READY_TIMEOUT_SEC) -> Dict[str, Any]:
    """Evidence link 2+3: poll the track-level observation until it returns
    facts (view present in bundle.views) instead of a source-evidence
    omission. The first request may land while the import bake is still
    running; each re-request re-reads the acoustic package read-first chain
    and picks up the settled feature snapshot."""
    deadline = time.time() + timeout_sec
    attempts = 0
    last_bundle: Dict[str, Any] = {}
    while time.time() < deadline:
        attempts += 1
        last_bundle = request_track_observation_bundle(agent_http, track_id, 60.0)
        views = last_bundle.get("views") if isinstance(last_bundle.get("views"), dict) else {}
        if "track.basic_energy" in views:
            return {"attempts": attempts, "bundle": last_bundle, "view": views["track.basic_energy"]}
        time.sleep(ACOUSTIC_READY_POLL_INTERVAL_SEC)
    omissions = last_bundle.get("omissions") if isinstance(last_bundle.get("omissions"), dict) else {}
    require(False, "setup evidence link failed: track.basic_energy never returned facts "
                   f"(attempts={attempts}, omissions={json.dumps(omissions, ensure_ascii=False)[:400]}) — "
                   "clip present but observation still missing means an observation-pipeline defect "
                   "(card 230 stop condition 2, escalate)")


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
    """Strict confirmation-face detection (first-run lesson: a loose
    substring grep over the whole response blob misclassified
    limit_reached turns as confirmations because tool schemas echo the
    needs_confirmation field name). Only explicit faces count."""
    interactions = resp.get("interaction_requests")
    if isinstance(interactions, list):
        for row in interactions:
            if not isinstance(row, dict):
                continue
            kind = str(row.get("kind") or "").lower()
            status = str(row.get("status") or "").lower()
            if "confirm" in kind and status in ("waiting_for_user", "waiting_user", "pending_user", ""):
                return True
    stop = str(resp.get("stop_reason") or "")
    if stop in ("needs_confirmation", "automation_choreo_awaiting_confirmation"):
        return True
    goal = str(resp.get("goal_status") or "")
    if goal in ("waiting_confirmation", "waiting_confirm"):
        return True
    return False


def is_limit_reached(resp: Dict[str, Any]) -> bool:
    return str(resp.get("stop_reason") or "") == "limit_reached"


def is_queued_continuation(resp: Dict[str, Any]) -> bool:
    """Card 232: the scheduler-driven async continuation receipt — the round's
    work is queued and settling toward its natural stopping point; this is a
    progress signal, not a routing verdict."""
    return str(resp.get("stop_reason") or "") == "automatic_continuation_already_scheduled"


def get_runtime_status(base_url: str) -> Dict[str, Any]:
    """Read-only GET of /agent/runtime/status (same whitelist-GET posture as
    g_runtime_readonly_smoke.ps1): goal projection (status/stop_reason/
    summary/error), continuations projection."""
    request = urllib.request.Request(base_url.rstrip("/") + "/agent/runtime/status", method="GET")
    with urllib.request.urlopen(request, timeout=10.0) as response:
        return json.loads(response.read().decode("utf-8"))


def parse_tool_calls(log_text: str) -> List[str]:
    """Extract distinct tool names from agent-log harness.invoke lines."""
    tools: List[str] = []
    for match in re.finditer(r"harness\.invoke total_ms=\d+ tool=([A-Za-z0-9_.\-]+)", log_text):
        name = match.group(1)
        if name not in tools:
            tools.append(name)
    return tools


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


def looks_capability_blocked(resp: Dict[str, Any], text: str) -> bool:
    """Card 230 option C classification ruling: an honest capability_blocked
    declaration (the model found the route but the FS evidence admission gate
    cannot be satisfied on the current observation evidence) is its own
    failure class. It is NOT no_route (the route was found) and NOT
    boundary_ok (that lexicon covers plugin/space scope misses only — the
    existing four failure classes and boundary semantics are untouched).
    Structured signal first: the chat face reports stop_reason
    capability_blocked directly (229 r1); declaration wording second — the
    wording patterns come from the 229 r1/r3 receipts on disk."""
    if str(resp.get("stop_reason") or "") == "capability_blocked":
        return True
    if re.search(r"能力边界|capability[_ \-]blocked", text, re.IGNORECASE):
        return True
    return bool(re.search(r"无法确认.{0,16}候选|观察(数据|证据).{0,12}(缺失|不可用)|需要先恢复.{0,16}(观察|电平|频段)", text))


def runtime_state_capability_blocked(project_dir: Path, conversation: str) -> Optional[str]:
    """Fallback structural signal for the async-continuation form (229 r2/r3):
    the chat reply only says the task is queued while the continuation later
    settles the final free-state decision into the project's agent runtime
    state. Reads the loop receipt for this conversation; returns the receipt
    summary when the final decision is capability_blocked."""
    for candidate in project_dir.glob(".vit_history/**/agent_runtime_state.json"):
        try:
            data = json.loads(candidate.read_text(encoding="utf-8", errors="replace"))
        except (OSError, json.JSONDecodeError):
            continue
        loops = data.get("free_state_reasoning_loops")
        if not isinstance(loops, dict):
            continue
        for key, entries in loops.items():
            if not isinstance(entries, list) or conversation not in str(key):
                continue
            for entry in entries:
                if not isinstance(entry, dict):
                    continue
                decision = entry.get("latest_decision")
                if isinstance(decision, dict) and str(decision.get("status", "")) == "capability_blocked":
                    return str(decision.get("summary") or "")[:300]
    return None


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
              ride_plugin_id: str, rounds_dir: Path,
              agent_log: Optional[Path] = None,
              project_dir: Optional[Path] = None) -> Dict[str, Any]:
    """One LLM round: phrase -> classify -> followups -> verdict -> cleanup.

    Returns the round record (phrase, exchanges, verdict, kernel evidence,
    tool-call evidence aligned from the agent log delta). Never raises for
    model-behavior outcomes; only stack errors propagate.
    """
    conversation = f"llm_route_g{group}_r{index}_{int(time.time())}"
    exchanges: List[Dict[str, Any]] = []
    record: Dict[str, Any] = {
        "group": group, "round": index, "phrase": phrase, "conversation": conversation,
        "exchanges": exchanges, "verdict": None, "verdict_detail": "",
        "kernel_points_before": kernel_point_count(kernel_zmq, ride_plugin_id, "volume"),
    }

    log_offset = 0
    if agent_log is not None and agent_log.exists():
        try:
            log_offset = agent_log.stat().st_size
        except OSError:
            log_offset = 0

    def collect_tools() -> List[str]:
        if agent_log is None or not agent_log.exists():
            return []
        try:
            with agent_log.open("rb") as handle:
                handle.seek(log_offset)
                delta = handle.read().decode("utf-8", errors="replace")
            return parse_tool_calls(delta)
        except OSError:
            return []

    def send(message: str) -> Dict[str, Any]:
        resp = chat(agent_http, conversation, message, chat_context)
        exchanges.append({"sent": message, "response": resp})
        return resp

    def settle_from_terminal_wait() -> bool:
        """Card 232 terminal-state wait fuse: the round hit a progress/queued
        reply (limit_reached continuation notice / queued continuation), so
        poll the agent runtime inside a bounded window and dispatch on the
        first natural stopping point. Returns True when a verdict was reached;
        False means the window expired (caller files in_progress_timeout)."""
        nonlocal followups
        record["terminal_wait"] = {"window_sec": TERMINAL_WAIT_TIMEOUT_SEC,
                                   "poll_interval_sec": TERMINAL_WAIT_POLL_INTERVAL_SEC,
                                   "polls": 0, "goal_id": resp.get("goal_id") or ""}
        target_goal_id = str(resp.get("goal_id") or "")
        deadline = time.time() + TERMINAL_WAIT_TIMEOUT_SEC
        while time.time() < deadline:
            record["terminal_wait"]["polls"] += 1
            try:
                status = get_runtime_status(agent_http)
            except (OSError, ValueError) as exc:
                record["terminal_wait"]["last_poll_error"] = str(exc)
                time.sleep(TERMINAL_WAIT_POLL_INTERVAL_SEC)
                continue
            goal = status.get("goal") if isinstance(status.get("goal"), dict) else {}
            goal_id = str(goal.get("goal_id") or "")
            if target_goal_id and goal_id and goal_id != target_goal_id:
                # the last active goal is not this round's goal (e.g. a pending
                # choreo card from another conversation): keep waiting
                record["terminal_wait"]["last_foreign_goal"] = goal_id
                time.sleep(TERMINAL_WAIT_POLL_INTERVAL_SEC)
                continue
            gstatus = str(goal.get("status") or "")
            gstop = str(goal.get("stop_reason") or "")
            # An unconfirmed kernel write during the wait is a governance
            # breach; a write after OUR confirmation drive is that
            # confirmation settling asynchronously (success path below).
            points_now = kernel_point_count(kernel_zmq, ride_plugin_id, "volume")
            if points_now > 0:
                if record.get("confirmations_driven", 0) > 0:
                    record["kernel_points_after_confirm"] = kernel_curve(kernel_zmq, ride_plugin_id, "volume").get("points")
                    record["verdict"] = "tool_chain_ok"
                    record["verdict_detail"] = (f"confirmed write reached kernel via async continuation settle "
                                                f"(points={points_now})")
                    return True
                record["verdict"] = "confirm_broken"
                record["verdict_detail"] = "kernel curve mutated during the terminal-state wait with no confirmation face driven"
                return True
            if gstop == "capability_blocked":
                record["verdict"] = "capability_blocked"
                record["verdict_detail"] = (f"goal stop_reason capability_blocked (async continuation): "
                                            f"{str(goal.get('summary') or '')[:200]!r}")
                return True
            if project_dir is not None:
                receipt = runtime_state_capability_blocked(project_dir, conversation)
                if receipt is not None:
                    record["verdict"] = "capability_blocked"
                    record["verdict_detail"] = f"free-state loop receipt capability_blocked (async continuation): {receipt!r}"
                    return True
            # Natural stopping point 1: the governance confirmation card is on
            # the table — drive it, then verify the write and read the curve
            # back point by point.
            if gstatus in ("waiting_confirmation", "waiting_confirm"):
                if followups >= FOLLOWUP_MAX:
                    record["verdict"] = "confirm_broken"
                    record["verdict_detail"] = "confirmation card reached after terminal wait but followup budget exhausted"
                    return True
                confirm = send(CONFIRM_PHRASE)
                followups += 1
                record["confirmations_driven"] = record.get("confirmations_driven", 0) + 1
                after = kernel_point_count(kernel_zmq, ride_plugin_id, "volume")
                if after > 0:
                    record["kernel_points_after_confirm"] = kernel_curve(kernel_zmq, ride_plugin_id, "volume").get("points")
                    record["settle_stop_reason"] = confirm.get("stop_reason")
                    record["settle_mentions_get_curve"] = "automation.get_curve" in response_blob(confirm)
                    record["verdict"] = "tool_chain_ok"
                    record["verdict_detail"] = f"confirmed write reached kernel after terminal wait (points={after})"
                    return True
                # the confirm drive itself may queue (async settle): loop back
                # and let the points/receipt checks above catch the landing
                continue
            # Natural stopping points 2/3: terminal goal status without our
            # confirmation — classify faithfully, no fabrication.
            if gstatus in ("completed", "failed", "cancelled", "stopped"):
                record["terminal_wait"]["final_goal"] = {
                    "status": gstatus, "stop_reason": gstop,
                    "summary": str(goal.get("summary") or "")[:300],
                    "error": str(goal.get("error") or "")[:300],
                }
                summary_text = str(goal.get("summary") or "")
                if gstatus == "failed":
                    err_break = looks_env_broken({"error": str(goal.get("error") or ""), "reply": summary_text})
                    if err_break:
                        record["verdict"] = "env_interrupt"
                        record["verdict_detail"] = f"goal failed during continuation: {err_break}"
                        return True
                if looks_boundary(summary_text):
                    record["verdict"] = "boundary_ok"
                    record["verdict_detail"] = f"boundary reply via continuation settle: {summary_text[:200]!r}"
                    return True
                if looks_capability_blocked({"stop_reason": gstop}, summary_text):
                    record["verdict"] = "capability_blocked"
                    record["verdict_detail"] = f"capability declaration via continuation settle: {summary_text[:200]!r}"
                    return True
                record["verdict"] = "no_route"
                record["verdict_detail"] = (f"continuation settled goal {gstatus} with no write, confirmation, or "
                                            f"capability face (stop_reason={gstop!r}, summary={summary_text[:200]!r})")
                return True
            time.sleep(TERMINAL_WAIT_POLL_INTERVAL_SEC)
        return False

    resp = send(phrase)

    # environment breakage check first (stop condition 1 evidence)
    env_break = looks_env_broken(resp)
    if env_break:
        record["verdict"] = "env_interrupt"
        record["verdict_detail"] = env_break
        record["tools_called"] = collect_tools()
        return record

    # deterministic takeover is not an LLM sample
    if looks_lexicon_hit(resp):
        record["verdict"] = "lexicon_hit"
        record["verdict_detail"] = f"stop_reason={resp.get('stop_reason')!r} workflow={resp.get('workflow')!r}"
        return record

    limit_followups = 0
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

        if (is_limit_reached(resp) or is_queued_continuation(resp)) and limit_followups < 1:
            # turn-budget stop with the goal scheduled for continuation: wake
            # it once with a plain continue (first-run evidence: the model was
            # cut mid-exploration, never reaching the automation tools). A
            # queued-continuation receipt here lands on the terminal-state
            # wait below (card 232) instead of a no_route filing.
            record["limit_reached_seen"] = True
            resp = send("继续")
            limit_followups += 1
            followups += 1
            continue

        detours = mentions_detour_tools(blob)
        if detours and not mentions_automation_tools(blob):
            record["verdict"] = "wrong_tool"
            record["verdict_detail"] = f"detour tools in response: {detours}"
            break

        if looks_boundary(text):
            record["verdict"] = "boundary_ok"
            record["verdict_detail"] = f"boundary reply (no write): {text[:200]!r}"
            # faithfulness against the kernel truth: the track-level Volume
            # & Pan plugin IS addressable — a 'no plugin' claim contradicts it
            record["evidence"] = {
                "boundary_vs_kernel": "unfaithful_plugin_claim" if re.search(r"(没有|未|无法).{0,16}插件", text) else "faithful",
                "kernel_track_volume_plugin_present": True,
            }
            break

        if looks_capability_blocked(resp, text):
            record["verdict"] = "capability_blocked"
            record["verdict_detail"] = (f"honest capability-blocked declaration "
                                        f"(stop_reason={resp.get('stop_reason')!r}): {text[:200]!r}")
            break

        if (looks_asking(text) or looks_clarification_pause(resp)) and followups < FOLLOWUP_MAX:
            resp = send(REGION_FOLLOWUP)
            followups += 1
            continue

        if is_limit_reached(resp) or is_queued_continuation(resp):
            # Card 232 terminal-state wait fuse: a progress/queued reply means
            # the async continuation is still settling this round toward its
            # natural stopping point. Wait bounded, then dispatch on the
            # terminal state instead of filing no_route on sight.
            if settle_from_terminal_wait():
                break
            record["verdict"] = "in_progress_timeout"
            record["verdict_detail"] = (f"terminal-state wait window ({TERMINAL_WAIT_TIMEOUT_SEC:.0f}s, "
                                        f"{record['terminal_wait']['polls']} polls) expired with the round still "
                                        f"in progress; last reply: {text[:200]!r}")
            break

        # plain text with no tool face, no boundary, no question
        record["verdict"] = "no_route"
        record["verdict_detail"] = f"plain-text response, no automation tool face: {text[:200]!r}"
        break

    record["tools_called"] = collect_tools()
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
    parser.add_argument("--agent-log", default="")
    parser.add_argument("--rounds-per-group", type=int, default=5)
    args = parser.parse_args()

    import tempfile
    project_path = Path(args.project_path).resolve() if args.project_path else Path(tempfile.gettempdir()) / f"automation_llm_route_{int(time.time())}.vit"
    rounds_dir = Path(args.rounds_dir).resolve()
    rounds_dir.mkdir(parents=True, exist_ok=True)
    telemetry_path = Path(args.telemetry_path).resolve() if args.telemetry_path else None
    prompt_render_path = Path(args.prompt_render_path).resolve() if args.prompt_render_path else None
    agent_log = Path(args.agent_log).resolve() if args.agent_log else None

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

        # ---- setup stage 2 (card 230 option A): deterministic clips --------
        # 229 root cause: bare tracks left the acoustic package missing, so
        # every derived observation lacked source evidence and the model
        # correctly stopped at the FS evidence admission gate. Each track now
        # carries the deterministic generated clip; the three evidence links
        # below are verified BEFORE any LLM round is spent.
        clip_wav = project_path.parent / CLIP_WAV_NAME
        write_deterministic_stereo_wav(clip_wav)
        for track_id in track_ids:
            reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_clip", "track_id": track_id,
                                                  "file_path": str(clip_wav), "start_time": 0.0,
                                                  "command_timeout_ms": 60000})
            require(str(reply.get("status", "")).lower() in {"ok", "success"},
                    f"add_audio_clip failed for track {track_id}: {json.dumps(reply, ensure_ascii=False)[:500]}")
        # evidence link 1: clips visible in the kernel project state
        kernel_state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        clip_counts = {tid: track_clip_count(kernel_state, tid) for tid in track_ids}
        require(all(count >= 1 for count in clip_counts.values()),
                f"deterministic clips not visible in kernel state: {clip_counts}")
        # evidence links 2+3: track-level observation returns facts and the
        # acoustic package status is no longer missing
        observation = wait_track_observation_facts(args.agent_http, ride_track_id)
        acoustic_excerpt: Dict[str, Any] = {"excerpted": False}
        derived_candidates = sorted(project_path.parent.glob("**/acoustic_package_status.json"))
        for derived_status_path in derived_candidates:
            try:
                derived_status = json.loads(derived_status_path.read_text(encoding="utf-8", errors="replace"))
                (rounds_dir / "setup_acoustic_package_status.json").write_text(
                    json.dumps(derived_status, ensure_ascii=False, indent=2), encoding="utf-8")
                packages = derived_status.get("packages") if isinstance(derived_status.get("packages"), list) else []
                acoustic_excerpt = {"excerpted": True, "path": str(derived_status_path),
                                    "package_statuses": [str(p.get("status")) for p in packages if isinstance(p, dict)]}
                break
            except (OSError, json.JSONDecodeError) as exc:
                acoustic_excerpt["error"] = str(exc)
        summary["setup"] = {"ride_track_id": ride_track_id, "ride_volume_plugin_id": ride_plugin_id,
                            "external_plugins_loaded": 0,
                            "clip_wav": str(clip_wav), "clip_duration_seconds": CLIP_DURATION_SECONDS,
                            "clip_counts": clip_counts,
                            "observation_ready_attempts": observation["attempts"],
                            "observation_view_excerpt": json.dumps(observation["view"], ensure_ascii=False)[:400],
                            "acoustic_package_status": acoustic_excerpt}
        print(f"[setup] ride track {ride_track_id} volume plugin {ride_plugin_id} (no external plugin, card group-2 premise)")
        print(f"[setup] deterministic clips on both tracks (evidence: {clip_counts}, observation ready after "
              f"{observation['attempts']} attempt(s), acoustic excerpt={acoustic_excerpt.get('package_statuses')})")

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
                                   phrase, chat_context, ride_plugin_id, rounds_dir,
                                   agent_log=agent_log, project_dir=project_path.parent)
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
                # (card 230: capability_blocked joins the stop-loss set — the
                # honest declaration is a failure form for the group gate and
                # repeated occurrences must not keep spending billed rounds;
                # card 232: in_progress_timeout likewise — three expired wait
                # windows means the observation protocol is broken, not the
                # model, and must stop burning billed rounds)
                failure_types = {"no_route", "capability_blocked", "wrong_tool", "confirm_broken", "write_fail", "in_progress_timeout"}
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
