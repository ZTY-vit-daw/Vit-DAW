"""B5: workspace activation-switch orphan forensics driver (real stack).

Replicates the 2026-09-11 hand-test-3 attempt-#1 timeline on a live stack:
agent boots on the default workspace, a chat turn parks an in-flight
continuation chain, then the kernel opens another project and the agent's
activation switch fires mid-chain.

Segments:
  W1 stack gate + default workspace capture
  W2 chat turn on the default workspace (in-flight chain established), then an
     immediate kernel open_project (the Godot "open project" user action)
  W3 switch observed (shadow follows the kernel project)
  W4 settle assertions: explicit terminal notice event, persisted cancelled
     record with the workspace-switch reason in the OLD workspace state,
     notice node landed in the old workspace graph, settle log line present
  W5 post-switch chain on the NEW workspace: terminal delivered + terminal
     node landed (attempt-#2 regression), no silent node-record failures

Exit codes: 0 = all segments green; 1 = assertion failure; 2 = environment
failure before the flow could run.
"""

from __future__ import annotations

import argparse
import json
import threading
import time
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any

LIVE_CONTINUATION_STATUSES = {"pending", "claimed", "running", "waiting_interaction"}
TERMINAL_EVENT_TYPES = {"turn.completed", "turn.stopped", "turn.failed"}


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

    def dump(self) -> None:
        with self._lock:
            payload = {"conversation_id": self.conversation_id, "final_since": self.since, "raw_events": self.events}
        self.log_path.write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def current_shadow(base: str) -> dict[str, Any]:
    """/agent/runtime/status carries the shadow project path (the /agent/state
    shadow block does not); forensic runtime_status_after.json shape."""
    runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    return (runtime.get("shadow") or {}), runtime


def find_actionable_card(recorder: EventRecorder) -> dict[str, Any] | None:
    for event in list(recorder.events):
        if event.get("type") not in {"interaction.requested", "card.offered", "interaction.pending"}:
            continue
        payload = event.get("payload") or {}
        requests = payload.get("requests") if isinstance(payload, dict) else None
        if isinstance(requests, list) and requests:
            first = requests[0]
            if isinstance(first, dict) and first.get("request_id"):
                return first
        rid = (event.get("payload") or {}).get("request_id") if isinstance(event.get("payload"), dict) else None
        if rid:
            return dict(event.get("payload"))
    return None


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
    """Poll the live agent log until the needle appears (log flush latency)."""
    while time.monotonic() < deadline:
        try:
            if needle in log_path.read_text(encoding="utf-8", errors="replace"):
                return True
        except OSError:
            pass
        time.sleep(0.5)
    return False


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-req", default="tcp://127.0.0.1:5555")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--run-root", required=True, help="smoke run root (old workspace artifacts live under it)")
    parser.add_argument("--agent-log", required=True)
    parser.add_argument("--next-project", required=True, help=".vit file the kernel opens mid-chain (W3 switch)")
    parser.add_argument("--message", default="请检查当前工程是否有什么问题吗")
    parser.add_argument("--chat-budget", type=float, default=150.0)
    parser.add_argument("--switch-budget", type=float, default=45.0)
    parser.add_argument("--settle-budget", type=float, default=90.0)
    parser.add_argument("--terminal-budget", type=float, default=420.0)
    parser.add_argument("--card-approve", action="store_true", help="approve parked interaction cards on the post-switch chain")
    parser.add_argument("--poll-interval", type=float, default=0.25)
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    run_root = Path(args.run_root)
    agent_log = Path(args.agent_log)
    next_project = Path(args.next_project).resolve()
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
        mark("segment_" + segment, ok=ok, expected=expected, **{k: v for k, v in fields_safe(evidence).items()})
        return ok

    def fields_safe(evidence: dict[str, Any]) -> dict[str, Any]:
        return {k: v for k, v in evidence.items() if k not in {"runtime_after", "chat_response"}}

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
        save_json(out_dir / "b5_switch_report.json", report)
        print(json.dumps(report["segments"], ensure_ascii=False, indent=1), flush=True)
        return exit_code

    # --- W1: stack gate + default workspace ----------------------------------
    try:
        state = request_json("GET", base + "/agent/state", None, 30.0)
    except Exception as exc:  # noqa: BLE001
        mark("env_failure", error=str(exc))
        return finish(2)
    save_json(out_dir / "w1_agent_state.json", state)
    # Boot shape honesty: the workspace-default project reports its uuid but no
    # file path (the agent binds a draft for it); the path arrives only when a
    # real file is opened. The uuid lives in /agent/state's shadow block (the
    # runtime/status shadow projection omits it). Gate on initialized + uuid.
    state_shadow = state.get("shadow") or {}
    old_project_path = str(state_shadow.get("project_path") or "")
    old_project_uuid = str(state_shadow.get("project_uuid") or "")
    record("W1", bool(state_shadow.get("initialized")) and bool(old_project_uuid), "green",
           shadow_initialized=state_shadow.get("initialized"), old_project_path=old_project_path,
           old_project_uuid=old_project_uuid, track_count=state_shadow.get("track_count"))
    if not segments["W1"]["ok"]:
        return finish(2)

    conversation_id = f"b5_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    recorder = EventRecorder(base, conversation_id, args.poll_interval, out_dir / "w2_event_recording.json")
    recorder.start()
    mark("recorder_started", conversation=conversation_id)

    # --- W2: chat on the default workspace, then the user opens a project -----
    t0 = time.monotonic()
    try:
        chat = request_json(
            "POST",
            base + "/agent/chat",
            {"conversation_id": conversation_id, "message": args.message, "context": {"agent_mode": "chat"}},
            args.chat_budget,
        )
    except Exception as exc:  # noqa: BLE001
        record("W2", False, "green", error=f"chat failed: {exc}")
        return finish(1, recorder)
    chat_ms = round((time.monotonic() - t0) * 1000, 1)
    save_json(out_dir / "w2_chat_response.json", chat)
    runtime = request_json("GET", base + "/agent/runtime/status", None, 30.0)
    save_json(out_dir / "w2_runtime_status.json", runtime)
    live_rows = [
        row for row in (runtime.get("continuations") or [])
        if str(row.get("conversation_id") or "") == conversation_id
        and str(row.get("status") or "").strip().lower() in LIVE_CONTINUATION_STATUSES
    ]
    record("W2", bool(live_rows), "green", chat_ms=chat_ms, goal_status=chat.get("goal_status"),
           stop_reason=chat.get("stop_reason"), reply_len=len(str(chat.get("reply") or "")),
           live_continuations=len(live_rows))
    if not segments["W2"]["ok"]:
        return finish(1, recorder)

    # The Godot-equivalent user action: open another project in the kernel,
    # fired immediately so the switch lands while the chain is still in flight.
    open_reply = kernel_open_project(args.kernel_req, str(next_project))
    mark("kernel_open_project", reply=json.dumps(open_reply, ensure_ascii=False)[:300])
    if not str(open_reply.get("status") or "ok").lower() in {"ok", ""}:
        record("W3", False, "green", error=f"open_project reply: {open_reply}")
        return finish(1, recorder)

    # --- W3: the activation switch follows the kernel ------------------------
    deadline = time.monotonic() + args.switch_budget
    switched = False
    shadow = {}
    while time.monotonic() < deadline:
        shadow, _runtime = current_shadow(base)
        if str(shadow.get("project_path") or "") == str(next_project):
            switched = True
            break
        time.sleep(0.5)
    save_json(out_dir / "w3_runtime_shadow.json", shadow)
    record("W3", switched, "green", new_project_path=str(next_project),
           shadow_path=str(shadow.get("project_path") or ""))

    # --- W4: the settle is explicit, nothing silently orphaned ----------------
    deadline = time.monotonic() + args.settle_budget
    notice_event = None
    while time.monotonic() < deadline and notice_event is None:
        for event in list(recorder.events):
            if event.get("item_id") != "chain_result":
                continue
            payload = event.get("payload") or {}
            if str((payload.get("stop_reason") if isinstance(payload, dict) else "") or "") == "workspace_switched":
                notice_event = event
                break
        if notice_event is None:
            time.sleep(0.5)
    record("W4a_notice_event", notice_event is not None and str((notice_event or {}).get("type") or "") == "turn.stopped",
           "green", event_type=(notice_event or {}).get("type"), body_head=str((notice_event or {}).get("body") or "")[:120])

    # Persisted old-workspace state carries the explicit cancelled record.
    deadline = time.monotonic() + 20.0
    cancelled_hits: list[Path] = []
    while time.monotonic() < deadline:
        cancelled_hits = [
            path for path in run_root.rglob("agent_runtime_state.json")
            if "workspace switched" in path.read_text(encoding="utf-8", errors="replace")
            and '"cancelled"' in path.read_text(encoding="utf-8", errors="replace")
        ]
        if cancelled_hits:
            break
        time.sleep(1.0)
    record("W4b_cancelled_record", bool(cancelled_hits), "green",
           state_files=[str(p.relative_to(run_root)) for p in cancelled_hits])

    # Notice node landed in the old workspace graph (refresh hydration proof).
    notice_hits = grep_files(run_root, "工程已切换")
    record("W4c_notice_node_landed", bool(notice_hits), "green",
           files=[str(p.relative_to(run_root)) for p in notice_hits[:6]])

    settle_logged = agent_log_contains(agent_log, "[workspace] switch settled live chains=", time.monotonic() + 10.0)
    record("W4d_settle_log", settle_logged, "green")

    # --- W5: a fresh chain on the NEW workspace delivers + lands its terminal --
    conversation2 = f"b5y_{datetime.now().strftime('%Y%m%d_%H%M%S')}"
    recorder2 = EventRecorder(base, conversation2, args.poll_interval, out_dir / "w5_event_recording.json")
    recorder2.start()
    mark("recorder2_started", conversation=conversation2)
    t0 = time.monotonic()
    try:
        chat2 = request_json(
            "POST",
            base + "/agent/chat",
            {"conversation_id": conversation2, "message": args.message, "context": {"agent_mode": "chat"}},
            args.chat_budget,
        )
    except Exception as exc:  # noqa: BLE001
        record("W5a_chat", False, "green", error=f"post-switch chat failed: {exc}")
        return finish(1, recorder2)
    save_json(out_dir / "w5_chat_response.json", chat2)
    record("W5a_chat", True, "green", goal_status=chat2.get("goal_status"), stop_reason=chat2.get("stop_reason"),
           chat_ms=round((time.monotonic() - t0) * 1000, 1))

    deadline = time.monotonic() + args.terminal_budget
    terminal_event = None
    approved = 0
    while time.monotonic() < deadline and terminal_event is None:
        for event in list(recorder2.events):
            if event.get("type") in TERMINAL_EVENT_TYPES and event.get("item_id") == "chain_result":
                terminal_event = event
                break
        if terminal_event is not None:
            break
        if args.card_approve and approved < 2:
            card = find_actionable_card(recorder2)
            if card:
                try:
                    request_json("POST", base + "/agent/interaction/respond", {
                        "interaction_id": str(card.get("request_id") or card.get("id")),
                        "action_id": "approve", "decision": "approve", "payload": {},
                    }, 120.0)
                    approved += 1
                    mark("card_approved", request_id=str(card.get("request_id") or card.get("id")), approvals=approved)
                    time.sleep(3.0)
                except Exception as exc:  # noqa: BLE001
                    mark("card_approve_failed", error=str(exc)[:200])
        time.sleep(0.5)
    terminal_ok = terminal_event is not None and bool(str((terminal_event or {}).get("body") or "").strip())
    record("W5b_terminal_event", terminal_ok, "green",
           event_type=(terminal_event or {}).get("type"),
           body_head=str((terminal_event or {}).get("body") or "")[:120],
           scheduler_chain=bool(((terminal_event or {}).get("payload") or {}).get("scheduler_chain")) if terminal_event else False,
           approvals=approved)

    if terminal_ok:
        needle = str(terminal_event.get("body") or "").strip()[:24]
        time.sleep(2.0)
        landed = grep_files(run_root, needle)
        record("W5c_terminal_node_landed", bool(landed), "green",
               needle=needle, files=[str(p.relative_to(run_root)) for p in landed[:6]])
    else:
        record("W5c_terminal_node_landed", False, "green", note="no terminal event to land")

    silent_failures = agent_log_contains(agent_log, "conversation node NOT recorded", time.monotonic() + 2.0)
    record("W6_no_silent_record_failures", not silent_failures, "green")

    all_green = all(row["ok"] for row in segments.values())
    return finish(0 if all_green else 1, recorder2)


if __name__ == "__main__":
    raise SystemExit(main())
