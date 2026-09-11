"""B6: mix-tick terminal delivery quality forensics driver (real stack).

Replicates the 2026-09-11 hand-test-3 attempt-#2 mix-tick shape on a live
stack with full_project_access active: the free-state improvement chain
proposes a bounded native adjustment, the pending mix-tick surface must not
promise a confirmation wait it will never hold, and the chain's terminal
reply must be actionable without logs (five elements: target track in
user-recognizable form, parameter, signed change plus readback, the finding
it targets, where to look in the DAW) with no internal codes and no hollow
pending-human-judgment claim.

Segments:
  M1 stack gate
  M2 full_project_access active before the conversation
  M3 kernel opens the project, chat parks the improvement chain; the live
     chain's kernel snapshots move the shadow onto the opened project
  M4 mix_tick.pending display is authority-aware (no confirmation promise)
  M5 terminal delivery: five elements + term/judgment discipline; judgment
     path honest (answerable park with a servicable entry, or settled with
     no pending-judgment claim)
  M6 terminal node landed in the conversation graph, no silent record failures

Exit codes: 0 = all segments green; 1 = assertion failure; 2 = environment
failure before the flow could run.
"""

from __future__ import annotations

import argparse
import json
import re
import threading
import time
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any

LIVE_CONTINUATION_STATUSES = {"pending", "claimed", "running", "waiting_interaction"}
TERMINAL_EVENT_TYPES = {"turn.completed", "turn.stopped", "turn.failed"}

# Five-element vocabulary: every admitted native domain's user-facing parameter
# wording from the chat composer (terminal_report_wording.go).
PARAM_WORDS = (
    "声像", "音量", "EQ 频段增益", "压缩器阈值", "齿音处理器阈值",
    "瞬态处理器起振", "限幅器输出上限", "门限处理器范围", "多段压限器频段阈值", "参数",
)
INTERNAL_TERM_PATTERN = re.compile(
    r"\b(?:FAM\d+(?:-S\d+(\.\d+)?)?|FS\d+|D\d+(?:-\d+(\.\d+)?)?|DAD|DOM|MOM|TIM|TOM|FXM|COM|EPM|RLM|CCB|PCA|VSP)\b"
)
SIGNED_DELTA_PATTERN = re.compile(r"[+-]\d+(?:\.\d+)?(?:\s?dB)?")
TRACK_LABEL_PATTERN = re.compile(r"Track\s+\S+")


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


def request_json(method: str, url: str, payload: dict[str, Any] | None, timeout: float) -> dict[str, Any]:
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        body = json.loads(resp.read().decode("utf-8"))
    if not isinstance(body, dict):
        raise RuntimeError(f"non-object response from {url}")
    return body


def save_json(path: Path, payload: Any) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def kernel_open_project(req_url: str, file_path: str, timeout_ms: int = 20000) -> dict[str, Any]:
    import zmq  # pyzmq is part of the repo script environment (bridge_core.py)

    ctx = zmq.Context.instance()
    sock = ctx.socket(zmq.REQ)
    sock.connect(req_url)
    sock.setsockopt(zmq.RCVTIMEO, timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, timeout_ms)
    sock.setsockopt(zmq.LINGER, 0)
    try:
        sock.send_string(json.dumps({"cmd": "open_project", "file_path": file_path}))
        reply = json.loads(sock.recv_string())
    finally:
        sock.close()
    return reply if isinstance(reply, dict) else {"reply": reply}


class EventRecorder:
    """Polls /agent/events with a since cursor, mirroring the WebUI consumer."""

    def __init__(self, base_url: str, conversation_id: str, interval: float, log_path: Path):
        self.url = base_url.rstrip("/") + "/agent/events"
        self.conversation_id = conversation_id
        self.interval = interval
        self.log_path = log_path
        self.since = 0
        self.events: list[dict[str, Any]] = []
        self._stop = threading.Event()
        self._lock = threading.Lock()

    def _poll_once(self) -> None:
        query = f"?conversation_id={self.conversation_id}&since={self.since}&limit=120"
        response = request_json("GET", self.url + query, None, 10.0)
        with self._lock:
            self.events.extend(response.get("events") or [])
            for event in response.get("events") or []:
                self.since = max(self.since, int(event.get("seq") or 0))
            next_seq = response.get("next_seq")
            if isinstance(next_seq, (int, float)):
                self.since = max(self.since, int(next_seq))

    def run(self) -> None:
        while not self._stop.is_set():
            try:
                self._poll_once()
            except Exception:
                pass
            self._stop.wait(self.interval)

    def start(self) -> None:
        self._thread = threading.Thread(target=self.run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        self._thread.join(timeout=15.0)

    def snapshot(self) -> list[dict[str, Any]]:
        with self._lock:
            return list(self.events)

    def dump(self) -> None:
        with self._lock:
            payload = {"conversation_id": self.conversation_id, "final_since": self.since, "raw_events": self.events}
        self.log_path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def current_shadow(base: str) -> dict[str, Any]:
    runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    return (runtime.get("shadow") or {}), runtime


def grep_files(root: Path, needle: str, patterns: tuple[str, ...] = ("*.json",)) -> list[Path]:
    hits: list[Path] = []
    for pattern in patterns:
        for path in root.rglob(pattern):
            try:
                if needle in path.read_text(encoding="utf-8", errors="replace"):
                    hits.append(path)
            except OSError:
                continue
    return hits


def agent_log_contains(log_path: Path, needle: str, deadline: float) -> bool:
    while time.monotonic() < deadline:
        try:
            if needle in log_path.read_text(encoding="utf-8", errors="replace"):
                return True
        except OSError:
            pass
        time.sleep(0.5)
    return False


def five_element_gaps(body: str) -> list[str]:
    gaps: list[str] = []
    if not TRACK_LABEL_PATTERN.search(body):
        gaps.append("target-track-label")
    if not any(word in body for word in PARAM_WORDS):
        gaps.append("parameter-word")
    if not SIGNED_DELTA_PATTERN.search(body):
        gaps.append("signed-change-amount")
    if "针对的发现" not in body:
        gaps.append("targeted-finding")
    if "去哪看" not in body:
        gaps.append("where-to-look")
    return gaps


def terminal_applied_form(event: dict[str, Any]) -> bool:
    """The applied slice ended the chain (forensic seq22 shape): its reply IS
    the terminal and must carry the five-element applied report."""
    payload = event.get("payload") or {}
    if not isinstance(payload, dict):
        return False
    if str(payload.get("stop_reason") or "") == "d1_post_action_evaluation_required":
        return True
    return "已应用并回读验证" in str(event.get("body") or "")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-req", default="tcp://127.0.0.1:5555")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--run-root", required=True, help="smoke run root (workspace artifacts live under it)")
    parser.add_argument("--agent-log", required=True)
    parser.add_argument("--project", required=True, help=".vit file the kernel opens before the conversation")
    parser.add_argument("--message", default="请从混音角度检查当前工程有什么可以改善的地方，并做一个小步尝试")
    parser.add_argument("--chat-budget", type=float, default=180.0)
    parser.add_argument("--switch-budget", type=float, default=300.0)
    parser.add_argument("--pending-budget", type=float, default=420.0)
    parser.add_argument("--terminal-budget", type=float, default=420.0)
    parser.add_argument("--poll-interval", type=float, default=0.25)
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    run_root = Path(args.run_root)
    agent_log = Path(args.agent_log)
    project_file = Path(args.project).resolve()
    base = args.agent_http.rstrip("/")
    started_at = now_stamp()
    timeline: list[dict[str, Any]] = []
    segments: dict[str, dict[str, Any]] = {}

    def mark(label: str, **fields: Any) -> None:
        row = {"wallclock": now_stamp(), "label": label, **fields}
        timeline.append(row)
        print(f"[{row['wallclock']}] {label} " + json.dumps({k: v for k, v in fields.items()}, ensure_ascii=False), flush=True)

    def record(segment: str, ok: bool, expected: str, **evidence: Any) -> bool:
        row = {"segment": segment, "ok": ok, "expected": expected, "wallclock": now_stamp(), **evidence}
        segments[segment] = row
        safe = {k: v for k, v in evidence.items() if k not in {"runtime_after", "chat_response"}}
        mark("segment_" + segment, ok=ok, expected=expected, **safe)
        return ok

    def finish(exit_code: int, recorder: EventRecorder | None = None) -> int:
        if recorder is not None:
            recorder.stop()
            recorder.dump()
        save_json(out_dir / "timeline.json", {"rows": timeline})
        report = {
            "started_at": started_at,
            "finished_at": now_stamp(),
            "segments": segments,
            "all_green": all(row["ok"] for row in segments.values()) and len(segments) >= 6,
        }
        save_json(out_dir / "b6_mixtick_report.json", report)
        print(json.dumps(report["segments"], ensure_ascii=False, indent=1), flush=True)
        return exit_code

    # --- M1: stack gate --------------------------------------------------------
    try:
        state = request_json("GET", base + "/agent/state", None, 30.0)
    except Exception as exc:  # noqa: BLE001
        mark("env_failure", error=str(exc))
        return finish(2)
    save_json(out_dir / "m1_agent_state.json", state)
    state_shadow = state.get("shadow") or {}
    record("M1_stack", bool(state_shadow.get("initialized")), "green",
           shadow_initialized=state_shadow.get("initialized"),
           boot_project_uuid=state_shadow.get("project_uuid"))
    if not segments["M1_stack"]["ok"]:
        return finish(2)

    # --- M2: full_project_access active before the conversation ---------------
    try:
        authority = request_json("POST", base + "/agent/authority", {"authority_mode": "full_project_access"}, 60.0)
    except Exception as exc:  # noqa: BLE001
        record("M2_full_access", False, "green", error=str(exc))
        return finish(1)
    try:
        snapshot = request_json("GET", base + "/agent/authority", None, 15.0)
    except Exception as exc:  # noqa: BLE001
        record("M2_full_access", False, "green", error=str(exc))
        return finish(1)
    record("M2_full_access", str(snapshot.get("authority_mode") or "") == "full_project_access", "green",
           post_status=authority.get("status"), snapshot=snapshot.get("authority_mode"))
    if not segments["M2_full_access"]["ok"]:
        return finish(1)

    # --- M3: kernel opens the project; a throwaway conversation absorbs the
    # workspace switch, then the real conversation runs on the settled
    # workspace. (Attempt #4 lesson: the run-dir project copy is a real
    # identity change from the boot draft — same uuid, different path — so the
    # B5 switch settle correctly cancels whichever chain is live when the
    # shadow first follows the kernel. The forensic shape has the project
    # bound before the measured conversation starts.)
    open_reply = kernel_open_project(args.kernel_req, str(project_file))
    mark("kernel_open_project", reply=json.dumps(open_reply, ensure_ascii=False)[:300])
    if str(open_reply.get("status") or "ok").lower() not in {"ok", ""}:
        record("M3_switch_settled", False, "green", error=f"open_project reply: {open_reply}")
        return finish(1)

    def shadow_snapshot() -> dict[str, Any]:
        try:
            shadow_block, _runtime = current_shadow(base)
            return shadow_block if isinstance(shadow_block, dict) else {}
        except Exception:  # noqa: BLE001
            return {}

    throwaway_id = f"b6w_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    throwaway_recorder = EventRecorder(base, throwaway_id, args.poll_interval, out_dir / "m3_throwaway_events.json")
    throwaway_recorder.start()
    try:
        throwaway = request_json(
            "POST",
            base + "/agent/chat",
            {"conversation_id": throwaway_id, "message": args.message, "context": {"agent_mode": "chat"}},
            args.chat_budget,
        )
        save_json(out_dir / "m3_throwaway_chat_response.json", throwaway)
        mark("throwaway_chat", goal_status=throwaway.get("goal_status"), stop_reason=throwaway.get("stop_reason"))
    except Exception as exc:  # noqa: BLE001
        record("M3_switch_settled", False, "green", error=f"throwaway chat failed: {exc}")
        return finish(1)
    # The shadow only follows the kernel while some chain polls it. Wait for
    # the switch to settle: shadow on the project path, and the throwaway
    # conversation reaching a terminal (its chain may be switch-cancelled —
    # that is the B5-correct behavior it exists to absorb).
    deadline = time.monotonic() + args.switch_budget
    switched = False
    shadow: dict[str, Any] = {}
    while time.monotonic() < deadline:
        shadow = shadow_snapshot()
        if str(shadow.get("project_path") or "") == str(project_file):
            switched = True
            break
        time.sleep(1.0)
    settled_terminal = False
    deadline = time.monotonic() + 30.0
    while time.monotonic() < deadline and not settled_terminal:
        for event in throwaway_recorder.snapshot():
            if event.get("type") in TERMINAL_EVENT_TYPES and event.get("item_id") == "chain_result":
                settled_terminal = True
                break
        if not settled_terminal and switched:
            break
        time.sleep(0.5)
    throwaway_recorder.stop()
    throwaway_recorder.dump()
    save_json(out_dir / "m1_runtime_shadow.json", shadow)
    record("M3_switch_settled", switched, "green", project_path=str(project_file),
           shadow_path=str(shadow.get("project_path") or ""), track_count=shadow.get("track_count"),
           throwaway_terminal=settled_terminal)
    if not segments["M3_switch_settled"]["ok"]:
        return finish(1)

    # The workspace switch restores the target workspace's persisted runtime
    # state, which resets the server authority mode to that state's value (a
    # fresh project copy carries none → manual). Re-assert full access on the
    # settled workspace before the measured conversation.
    try:
        authority = request_json("POST", base + "/agent/authority", {"authority_mode": "full_project_access"}, 60.0)
        snapshot = request_json("GET", base + "/agent/authority", None, 15.0)
    except Exception as exc:  # noqa: BLE001
        record("M3_full_access_resettled", False, "green", error=str(exc))
        return finish(1)
    record("M3_full_access_resettled", str(snapshot.get("authority_mode") or "") == "full_project_access", "green",
           post_status=authority.get("status"), snapshot=snapshot.get("authority_mode"))
    if not segments["M3_full_access_resettled"]["ok"]:
        return finish(1)

    conversation_id = f"b6_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    recorder = EventRecorder(base, conversation_id, args.poll_interval, out_dir / "m3_event_recording.json")
    recorder.start()
    mark("recorder_started", conversation=conversation_id)

    t0 = time.monotonic()
    try:
        chat = request_json(
            "POST",
            base + "/agent/chat",
            {"conversation_id": conversation_id, "message": args.message, "context": {"agent_mode": "chat"}},
            args.chat_budget,
        )
    except Exception as exc:  # noqa: BLE001
        record("M3_chat", False, "green", error=f"chat failed: {exc}")
        return finish(1, recorder)
    chat_ms = round((time.monotonic() - t0) * 1000, 1)
    save_json(out_dir / "m3_chat_response.json", chat)
    runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    save_json(out_dir / "m3_runtime_status.json", runtime)
    live_rows = [
        row for row in (runtime.get("continuations") or [])
        if str(row.get("conversation_id") or "") == conversation_id
        and str(row.get("status") or "").strip().lower() in LIVE_CONTINUATION_STATUSES
    ]
    record("M3_chat", bool(live_rows), "green", chat_ms=chat_ms, goal_status=chat.get("goal_status"),
           stop_reason=chat.get("stop_reason"), reply_len=len(str(chat.get("reply") or "")),
           live_continuations=len(live_rows))
    if not segments["M3_chat"]["ok"]:
        return finish(1, recorder)

    # --- M4: mix_tick.pending display is authority-aware -----------------------
    pending_event = None
    deadline = time.monotonic() + args.pending_budget
    while time.monotonic() < deadline and pending_event is None:
        for event in recorder.snapshot():
            if event.get("type") == "mix_tick.pending":
                pending_event = event
                break
        if pending_event is None:
            time.sleep(0.5)
    if pending_event is None:
        # Record and continue: the terminal segments below still capture what
        # the chain actually delivered (branch-variance forensics), the run
        # fails honestly at the summary.
        record("M4_pending_display", False, "green", error="mix_tick.pending never fired (branch not taken)",
               note="LLM branch variance: retry allowed per card run-count rules")
        pending_event = {"type": "mix_tick.pending", "body": "", "title": "", "payload": {}}
    pending_body = str(pending_event.get("body") or "")
    pending_title = str(pending_event.get("title") or "")
    pending_display = (pending_event.get("payload") or {}).get("display") or {}
    promise_leak = ("正在等待你确认" in pending_body) or ("确认前不会修改工程" in pending_body) or ("待确认" in pending_title)
    display_leak = isinstance(pending_display, dict) and ("等待你确认" in str(pending_display.get("body") or ""))
    direct_wording = ("完全访问" in pending_body) and ("直接应用" in pending_body)
    record("M4_pending_display", (not promise_leak) and (not display_leak) and direct_wording, "green",
           title=pending_title, body=pending_body,
           display_body=str(pending_display.get("body") or "") if isinstance(pending_display, dict) else "")

    # --- M5: terminal delivery quality -----------------------------------------
    deadline = time.monotonic() + args.terminal_budget
    terminal_event = None
    while time.monotonic() < deadline and terminal_event is None:
        for event in recorder.snapshot():
            if event.get("type") in TERMINAL_EVENT_TYPES and event.get("item_id") == "chain_result":
                terminal_event = event
                break
        if terminal_event is None:
            time.sleep(0.5)
    if terminal_event is None:
        record("M5_terminal", False, "green", error="chain terminal never arrived")
        return finish(1, recorder)
    body = str(terminal_event.get("body") or "")
    payload = terminal_event.get("payload") or {}
    goal_status = str(terminal_event.get("status") or payload.get("goal_status") or "")
    save_json(out_dir / "m5_terminal_event.json", terminal_event)

    internal_leak = INTERNAL_TERM_PATTERN.search(body)
    judgment_leak = ("人工判定" in body and "仍待" in body.replace(" ", ""))
    applied_form = terminal_applied_form(terminal_event)
    gaps = five_element_gaps(body) if applied_form else []
    m5_ok = bool(body.strip()) and internal_leak is None and not judgment_leak and not gaps
    record("M5_terminal_quality", m5_ok, "green", event_type=terminal_event.get("type"),
           goal_status=goal_status, stop_reason=payload.get("stop_reason"), applied_form=applied_form,
           five_element_gaps=gaps, internal_term=(internal_leak.group(0) if internal_leak else ""),
           body=body[:400])

    # Judgment path honesty (B6 defect 2, chosen route B): a completed settle
    # carries no pending-judgment claim; an answerable park must actually hold
    # a servicable entry (pending_interaction with an interaction id).
    if goal_status in {"waiting_confirmation", "waiting_clarification"}:
        runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
        save_json(out_dir / "m5_runtime_status.json", runtime)
        rows = [row for row in (runtime.get("continuations") or [])
                if str(row.get("conversation_id") or "") == conversation_id
                and (row.get("pending_interaction") or {})]
        entry_ids = [str((row.get("pending_interaction") or {}).get("interaction_id") or "") for row in rows]
        entry_ids = [eid for eid in entry_ids if eid]
        record("M5_judgment_path", bool(entry_ids), "green", shape="answerable_park",
               interaction_ids=entry_ids[:4])
    else:
        record("M5_judgment_path", "人工判定仍待" not in body.replace(" ", ""), "green",
               shape="settled_no_pending_judgment_claim", goal_status=goal_status)

    # --- M6: terminal node landed, no silent record failures -------------------
    needle = body.strip()[:24]
    time.sleep(2.0)
    landed = grep_files(run_root, needle)
    record("M6_terminal_node_landed", bool(landed), "green", needle=needle,
           files=[str(p.relative_to(run_root)) for p in landed[:6]])
    silent_failures = agent_log_contains(agent_log, "conversation node NOT recorded", time.monotonic() + 2.0)
    record("M6_no_silent_record_failures", not silent_failures, "green")

    all_green = all(row["ok"] for row in segments.values())
    return finish(0 if all_green else 1, recorder)


if __name__ == "__main__":
    raise SystemExit(main())
