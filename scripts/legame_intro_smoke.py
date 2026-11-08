#!/usr/bin/env python3
"""Legame L1 real-stack smoke driver (LEGAME-DIR-1 / LEGAME-AUTOINTRO-1 /
LEGAME-FRESH-1 stack legs, 2026-11-08).

Drives the L1 acceptance faces over the real kernel + agent berth
(dev_agent_smoke.ps1 -StartKernel -RestartAgent -LegameSmoke -LegameMode
<dir314|intro>, no UI). Two modes, one stack session each:

  dir314 (LEGAME-DIR-1 acceptance, design 2.1):
     NO VIT_EDU_LEVELS_DIR anywhere in the agent environment; the levels
     card was installed into the REAL ~/.vit/levels by
     legame_placement.py install-levels before the stack came up. The
     kernel opens an isolated fixture COPY (rehearsal-copy rule - never the
     %APPDATA% user target). Face: chat 开始乐局 -> edu_jam_entered with the
     first-run-twinkle opening narrative (proves the card resolved from the
     product install tier, not the repo tier and not an env override) ->
     结束乐局 -> edu_jam_exited. Zero LLM by design (deterministic jam
     command face, 281-proven).

  intro (LEGAME-AUTOINTRO-1 + LEGAME-FRESH-1 acceptance, design 2.2/2.3):
     The isolated copy carries a fresh binding (place-copy / relay wrote
     .vit_agent/legame_binding.json with opened_at=null). Face:
     conv-A white-talk ("在吗" - NO command word) -> edu_jam_entered
     (auto-intro fired inside handleChat) -> binding on disk has
     opened_at + opened_conversation_id == conv-A -> conv-B white-talk ->
     edu_jam_recovery_broadcast ("接着上局：...") -> the %APPDATA% user
     target project sha256 still equals the pristine fixture hash
     (rehearsal-copy rule: target untouched). Between two intro runs the
     orchestrator relays the copy (real relay command) so run 2 proves
     "relay -> fresh agent -> auto-intro fires again" (fresh-project
     semantics).

LLM budget (RUNBOOK section 8): zero by design for both modes (jam faces
are deterministic; activation is polled to completion BEFORE any chat so a
deferred intro never spills a white-talk turn into the semantic entry).
Honest ceiling if an intro/broadcast fails to intercept: <= 2 deepseek-
flash calls per conversation (the un-intercepted white talk), recorded as
failure symptom evidence, never as success.

Exit codes (fixed before any run; single run, no rerun):
  0 pass            every phase asserted green
  2 environment     stack/transport/fixture failure, activation window
                    exhaustion (gate rejections are by-design)
  4 face_defect     the deterministic legame faces answered wrong (missed
                    auto-intro, wrong broadcast, card not from the product
                    tier, target hash drift) - forensics hand-over, no rerun
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(2)

CHAT_TIMEOUT_SEC = 60.0
ACTIVATION_WAIT_SEC = 30.0
ENTER_RETRY_MAX = 6
ENTER_RETRY_SLEEP_SEC = 1.0
GATE_STOP_REASON = "edu_jam_workspace_activation_incomplete"
FIXTURE_SHA256 = "e0e9533824207bb15d949a53d2e99df9507c687e1746f029205e5492d02441e3"

EXIT_PASS = 0
EXIT_ENV = 2
EXIT_FACE_DEFECT = 4
EXIT_CLASS = {EXIT_PASS: "pass", EXIT_ENV: "environment", EXIT_FACE_DEFECT: "face_defect"}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


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
        except Exception as exc:  # noqa: BLE001
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"kernel did not answer ping on {url}: {last_error}")


def get_ui_state(base_url: str) -> Dict[str, Any]:
    with urllib.request.urlopen(base_url.rstrip("/") + "/agent/ui/state", timeout=10.0) as response:
        return json.loads(response.read().decode("utf-8"))


def norm_project_path(path: str) -> str:
    return os.path.normcase(os.path.normpath(str(path or "")))


def wait_workspace_activation(base_url: str, project_path: str, timeout_s: float) -> Dict[str, Any]:
    deadline = time.time() + timeout_s
    state: Dict[str, Any] = {}
    want = norm_project_path(project_path)
    last_path = ""
    while time.time() < deadline:
        try:
            state = get_ui_state(base_url)
        except Exception:  # noqa: BLE001 - transient poll errors are retried.
            state = {}
        project = state.get("project") if isinstance(state.get("project"), dict) else {}
        last_path = str(project.get("project_path") or "")
        if norm_project_path(last_path) == want:
            return {"activated": True, "project_path": last_path}
        time.sleep(0.5)
    return {"activated": False, "project_path": last_path}


def telemetry_rows_for(telemetry_path: Optional[Path], needle: str) -> Optional[int]:
    if not telemetry_path or not telemetry_path.exists():
        return None
    calls = 0
    try:
        for line in telemetry_path.read_text(encoding="utf-8", errors="replace").splitlines():
            line = line.strip()
            if not line or needle not in line:
                continue
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            if isinstance(row, dict) and needle in json.dumps(row, ensure_ascii=False):
                calls += 1
    except OSError:
        return None
    return calls


def main() -> int:
    parser = argparse.ArgumentParser(description="Legame L1 stack-leg driver (dir314 | intro)")
    parser.add_argument("--mode", required=True, choices=["dir314", "intro"])
    parser.add_argument("--agent-http", required=True)
    parser.add_argument("--kernel-zmq", required=True)
    parser.add_argument("--project-path", required=True, help="isolated fixture copy path (never the %APPDATA% target)")
    parser.add_argument("--output", required=True, help="summary json path")
    parser.add_argument("--telemetry-path", default="", help="agent LLM telemetry jsonl")
    parser.add_argument("--user-target", default="", help="read-only reference: %APPDATA% user target .vit (hash-asserted untouched)")
    parser.add_argument("--chat-timeout", type=float, default=CHAT_TIMEOUT_SEC)
    args = parser.parse_args()

    stamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    summary: Dict[str, Any] = {
        "card": "LEGAME-L1-STACK-1",
        "mode": args.mode,
        "stamp": stamp,
        "project_path": str(Path(args.project_path).resolve()),
        "phases": {},
    }

    def save_summary() -> None:
        Path(args.output).write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    def fail(exit_code: int, phase: str, reason: str, extra: Optional[Dict[str, Any]] = None) -> int:
        summary["exit_code"] = exit_code
        summary["exit_class"] = EXIT_CLASS.get(exit_code, "unknown")
        summary["fail_phase"] = phase
        summary["fail_reason"] = reason
        if extra:
            summary["phases"].setdefault(phase, {}).update(extra)
        save_summary()
        print(f"[legame-smoke] FAIL exit={exit_code} phase={phase}: {reason}", flush=True)
        return exit_code

    # Guard of guards: the dir314 leg proves the PRODUCT tier; an env override
    # left in this process would silently void the proof.
    if args.mode == "dir314" and os.environ.get("VIT_EDU_LEVELS_DIR", "").strip():
        return fail(EXIT_ENV, "env_guard", "VIT_EDU_LEVELS_DIR is set in the driver environment; the dir314 leg requires it unset")

    chat_log: List[Dict[str, Any]] = []

    def chat(conversation: str, message: str, timeout: Optional[float] = None) -> Dict[str, Any]:
        try:
            response = post_json(args.agent_http, "/agent/chat", {
                "conversation_id": conversation,
                "message": message,
                "context": {"agent_mode": "chat"},
            }, timeout or args.chat_timeout)
        except Exception as exc:  # noqa: BLE001 - env vs face classified by caller.
            raise RuntimeError(f"chat request exception ({message!r}): {exc}") from exc
        chat_log.append({"conversation": conversation, "message": message, "response": response})
        Path(args.output).with_name("chat_responses.json").write_text(
            json.dumps(chat_log, ensure_ascii=False, indent=2), encoding="utf-8")
        return response

    def data_of(response: Dict[str, Any]) -> Dict[str, Any]:
        data = response.get("workflow_data")
        return data if isinstance(data, dict) else {}

    telemetry_path = Path(args.telemetry_path) if args.telemetry_path else None

    try:
        wait_kernel_zmq(args.kernel_zmq, 30.0)
        wait_agent_http(args.agent_http, 60.0)

        # Open the isolated fixture copy in the kernel, then wait for the agent
        # workspace to bind it (activation to completion BEFORE any chat - a
        # deferred auto-intro would otherwise spill a white-talk turn into the
        # semantic entry and bill an LLM call).
        place_path = str(Path(args.project_path).resolve())
        reply = zmq_command(args.kernel_zmq, {"cmd": "open_project", "file_path": place_path, "command_timeout_ms": 60000})
        if str(reply.get("status", "")).lower() not in {"ok", "success"}:
            return fail(EXIT_ENV, "open_project", f"open_project failed: {json.dumps(reply, ensure_ascii=False)[:400]}")
        activation = wait_workspace_activation(args.agent_http, place_path, ACTIVATION_WAIT_SEC)
        summary["phases"]["activation"] = activation
        save_summary()
        print(f"[legame-smoke] activation: {json.dumps(activation, ensure_ascii=False)}", flush=True)
        if not activation.get("activated"):
            return fail(EXIT_ENV, "activation",
                        f"workspace never activated onto {place_path} within {ACTIVATION_WAIT_SEC}s (white-talk NOT sent; zero-LLM budget intact)")
    except Exception as exc:  # noqa: BLE001
        return fail(EXIT_ENV, "stack", f"stack/transport exception: {exc}")

    if args.mode == "dir314":
        conv = f"legame_dir314_{stamp}"
        try:
            resp: Dict[str, Any] = {}
            enter_attempts = 0
            for attempt in range(1, ENTER_RETRY_MAX + 1):
                enter_attempts = attempt
                resp = chat(conv, "开始乐局")
                if resp.get("stop_reason") != GATE_STOP_REASON:
                    break
                time.sleep(ENTER_RETRY_SLEEP_SEC)
            summary["phases"]["enter_attempts"] = enter_attempts
            d = data_of(resp)
            if resp.get("stop_reason") == GATE_STOP_REASON:
                return fail(EXIT_ENV, "enter", "enter still gated after retries (by-design gate, stack did not settle)")
            if resp.get("workflow") != "edu_jam" or resp.get("stop_reason") != "edu_jam_entered":
                return fail(EXIT_FACE_DEFECT, "enter",
                            f"enter not intercepted as jam face without any env: {json.dumps(resp, ensure_ascii=False)[:800]}")
            if d.get("level_id") != "first-run-twinkle" or d.get("current_node") != "n0_photo_score_open":
                return fail(EXIT_FACE_DEFECT, "enter", f"enter workflow_data wrong: {json.dumps(d, ensure_ascii=False)[:400]}")
            if "乐局开始" not in str(resp.get("reply") or ""):
                return fail(EXIT_FACE_DEFECT, "enter", f"enter reply missing opening narrative: {resp.get('reply')}")

            resp = chat(conv, "结束乐局")
            d = data_of(resp)
            if resp.get("stop_reason") != "edu_jam_exited" or d.get("status") != "exited":
                return fail(EXIT_FACE_DEFECT, "exit", f"exit wrong: stop={resp.get('stop_reason')} data={json.dumps(d, ensure_ascii=False)[:400]}")
        except RuntimeError as exc:
            return fail(EXIT_ENV, "chat", str(exc))

        llm_rows = telemetry_rows_for(telemetry_path, conv)
        summary["phases"]["jam"] = {"conversation_id": conv, "llm_rows": llm_rows}
        if llm_rows not in (None, 0):
            return fail(EXIT_FACE_DEFECT, "zero_llm", f"jam conversation billed {llm_rows} LLM rows (must be 0)")

    elif args.mode == "intro":
        conv_a = f"legame_intro_a_{stamp}"
        conv_b = f"legame_intro_b_{stamp}"
        binding_path = Path(place_path).parent / ".vit_agent" / "legame_binding.json"
        try:
            # conv-A: white talk, NO command word -> auto-intro.
            resp = chat(conv_a, "在吗")
            d = data_of(resp)
            if resp.get("workflow") != "edu_jam" or resp.get("stop_reason") != "edu_jam_entered":
                return fail(EXIT_FACE_DEFECT, "auto_intro",
                            f"white-talk did not auto-intro (no command word sent): stop={resp.get('stop_reason')} workflow={resp.get('workflow')} reply={str(resp.get('reply'))[:300]}")
            if d.get("level_id") != "first-run-twinkle" or d.get("current_node") != "n0_photo_score_open":
                return fail(EXIT_FACE_DEFECT, "auto_intro", f"intro workflow_data wrong: {json.dumps(d, ensure_ascii=False)[:400]}")
            if "乐局开始" not in str(resp.get("reply") or ""):
                return fail(EXIT_FACE_DEFECT, "auto_intro", f"intro reply missing opening narrative: {resp.get('reply')}")
            summary["phases"]["auto_intro"] = {"conversation_id": conv_a, "stop_reason": resp.get("stop_reason")}

            # Binding markers written back on disk by the agent.
            try:
                binding = json.loads(binding_path.read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError) as exc:
                return fail(EXIT_FACE_DEFECT, "binding_writeback", f"binding unreadable after intro: {binding_path} ({exc})")
            if not binding.get("opened_at") or binding.get("opened_conversation_id") != conv_a:
                return fail(EXIT_FACE_DEFECT, "binding_writeback",
                            f"binding opened markers wrong: {json.dumps(binding, ensure_ascii=False)}")
            summary["phases"]["binding_writeback"] = {"opened_at": binding.get("opened_at"),
                                                      "opened_conversation_id": binding.get("opened_conversation_id")}

            # conv-B: white talk in a NEW conversation -> recovery broadcast.
            resp = chat(conv_b, "嗨")
            if resp.get("stop_reason") != "edu_jam_recovery_broadcast":
                return fail(EXIT_FACE_DEFECT, "recovery_broadcast",
                            f"second conversation did not get recovery broadcast: stop={resp.get('stop_reason')} reply={str(resp.get('reply'))[:300]}")
            if "接着上局" not in str(resp.get("reply") or ""):
                return fail(EXIT_FACE_DEFECT, "recovery_broadcast", f"broadcast reply missing recovery lead: {resp.get('reply')}")
            bd = data_of(resp)
            if bd.get("owner_conversation_id") != conv_a:
                return fail(EXIT_FACE_DEFECT, "recovery_broadcast", f"broadcast owner wrong: {json.dumps(bd, ensure_ascii=False)[:400]}")
            summary["phases"]["recovery_broadcast"] = {"conversation_id": conv_b,
                                                       "reply_head": str(resp.get("reply"))[:120]}
        except RuntimeError as exc:
            return fail(EXIT_ENV, "chat", str(exc))

        # Rehearsal-copy rule: the %APPDATA% user target must be untouched.
        if args.user_target:
            target = Path(args.user_target)
            if not target.is_file():
                return fail(EXIT_FACE_DEFECT, "target_untouched", f"user target missing: {target}")
            target_sha = sha256_file(target)
            summary["phases"]["target_untouched"] = {"target": str(target), "sha256": target_sha}
            if target_sha != FIXTURE_SHA256:
                return fail(EXIT_FACE_DEFECT, "target_untouched",
                            f"user target hash drifted from the pristine fixture: {target_sha} != {FIXTURE_SHA256}")

        llm_a = telemetry_rows_for(telemetry_path, conv_a)
        llm_b = telemetry_rows_for(telemetry_path, conv_b)
        summary["phases"]["zero_llm"] = {"conv_a_rows": llm_a, "conv_b_rows": llm_b}
        if llm_a not in (None, 0) or llm_b not in (None, 0):
            return fail(EXIT_FACE_DEFECT, "zero_llm",
                        f"legame conversations billed LLM rows (a={llm_a}, b={llm_b}; must be 0 - a row here means an un-intercepted white-talk turn)")

    summary["exit_code"] = EXIT_PASS
    summary["exit_class"] = EXIT_CLASS[EXIT_PASS]
    save_summary()
    print(f"[legame-smoke] PASS ({args.mode}): {json.dumps(summary['phases'], ensure_ascii=False)[:400]}", flush=True)
    return EXIT_PASS


if __name__ == "__main__":
    sys.exit(main())
