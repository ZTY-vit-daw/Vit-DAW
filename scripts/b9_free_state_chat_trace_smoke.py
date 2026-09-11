"""E2E-1 layer B9 driver: free-state chat trace unified surface (B9 card, 2026-09-11).

Drives the real WebUI (agent-served at --webui-url) through the MANUAL test-3
form that produced the B9 four symptoms (2026-09-11 19:54-19:55, conversation
webui_mtwwegtp): a free-state default chat round ("检查一下当前工程有什么问题吗")
that spawns a scheduler-fragmented chain with the experiment trajectory emitted
in a batch late in the round.

Segments (all gated green for exit 0):
  W1_submit             composer accepts the message, conversation id captured
  W2_execution_period   during chain execution the dynamic area shows content
                        AND accumulates increments (fix for symptom 1: polling
                        stays awake via the live trajectory round)
  W3_unified_terminal   exactly ONE new trace block for the round, carrying the
                        experiment steps, terminal merged into it, no empty-shell
                        copy ("本回合尚未产生轨迹节点") anywhere (symptoms 2+3)
  W4_refresh_survival   after reload the terminal message AND the trace block
                        with steps are both present (symptom 4)

Forensics recorded alongside (not gated): /agent/runtime/status goal status +
continuation statuses sampled every second during execution — settles whether
the free-state chain surfaces in the goal/continuations projection (the
GUI-F5 sleep hypothesis input).

Environment failures (no composer, submission rejected, no chain at all within
budget) exit 2. Read-only against production code; screenshots + JSON report
into --out-dir.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.request
from datetime import datetime
from pathlib import Path
from typing import Any

from playwright.sync_api import sync_playwright

TERMINAL_MIN_CHARS = 30
EMPTY_SHELL_TEXT = "本回合尚未产生轨迹节点"

LAUNCH_CHANNELS: list[dict[str, Any]] = [
    {"label": "bundled-chromium"},
    {"label": "msedge", "channel": "msedge"},
    {"label": "chrome", "channel": "chrome"},
]


def launch_chromium(pw):
    last_error: Exception | None = None
    for candidate in LAUNCH_CHANNELS:
        try:
            kwargs: dict[str, Any] = {"headless": True}
            if candidate.get("channel"):
                kwargs["channel"] = candidate["channel"]
            return pw.chromium.launch(**kwargs), candidate["label"]
        except Exception as exc:  # noqa: BLE001
            last_error = exc
    raise RuntimeError("no chromium channel launchable: " + str(last_error))


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


def http_get_json(url: str, timeout: float = 6.0) -> dict[str, Any] | None:
    try:
        with urllib.request.urlopen(url, timeout=timeout) as response:
            return json.loads(response.read().decode("utf-8", errors="replace"))
    except Exception:  # noqa: BLE001 - sampling must survive transient errors
        return None


class Bed:
    def __init__(self, page, out_dir: Path) -> None:
        self.page = page
        self.out_dir = out_dir
        self.timeline: list[dict[str, Any]] = []
        self.results: dict[str, dict[str, Any]] = {}
        self.conversation_id = ""
        self.event_requests: list[str] = []
        self.runtime_samples: list[dict[str, Any]] = []

    def mark(self, label: str, **fields: Any) -> None:
        row = {"wallclock": now_stamp(), "label": label, **fields}
        self.timeline.append(row)
        print(f"[{row['wallclock']}] {label} " + json.dumps({k: v for k, v in fields.items()}, ensure_ascii=False), flush=True)

    def shot(self, step: str) -> None:
        path = self.out_dir / f"{step}_{datetime.now().strftime('%H%M%S_%f')[:-3]}.png"
        try:
            self.page.screenshot(path=str(path), full_page=False)
            self.mark("shot", step=step, path=str(path))
        except Exception as exc:  # noqa: BLE001
            self.mark("shot_failed", step=step, error=str(exc))

    def record(self, segment: str, ok: bool, **evidence: Any) -> None:
        self.results[segment] = {"segment": segment, "ok": ok, "wallclock": now_stamp(), **evidence}
        self.mark("segment_" + segment, ok=ok, **{k: v for k, v in evidence.items() if k != "samples"})

    def text(self, selector: str) -> str:
        try:
            locator = self.page.locator(selector)
            if locator.count() == 0:
                return ""
            return " | ".join(locator.all_inner_texts()[:12])
        except Exception:  # noqa: BLE001
            return ""

    def count(self, selector: str) -> int:
        try:
            return self.page.locator(selector).count()
        except Exception:  # noqa: BLE001
            return 0


def trace_block_ids(page) -> list[str]:
    ids: list[str] = []
    try:
        for i in range(page.locator("section.trace-block").count()):
            value = page.locator("section.trace-block").nth(i).get_attribute("data-turn-id")
            ids.append(value or f"<anon{i}>")
    except Exception:  # noqa: BLE001
        pass
    return ids


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--webui-url", default="http://127.0.0.1:7878/app/")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--message", default="检查一下当前工程有什么问题吗")
    parser.add_argument("--authority", default="full_project_access", choices=["full_project_access", "manual_confirmation"],
                        help="pre-set authority mode (the manual-test form ran full access; mix_tick applies directly)")
    parser.add_argument("--accept-budget", type=float, default=20.0, help="composer acceptance budget (s)")
    parser.add_argument("--chat-budget", type=float, default=120.0, help="first-response budget (s)")
    parser.add_argument("--terminal-budget", type=float, default=240.0, help="chain terminal budget (s)")
    parser.add_argument("--reload-budget", type=float, default=30.0, help="post-reload hydration budget (s)")
    args = parser.parse_args()

    out_dir = Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    started_at = time.monotonic()

    def finish(bed: Bed | None, browser, code: int, blocked_reason: str = "") -> int:
        report: dict[str, Any] = {
            "driver": "b9_free_state_chat_trace_smoke",
            "exit_code": code,
            "blocked_reason": blocked_reason,
            "conversation_id": bed.conversation_id if bed else "",
            "message": args.message,
            "results": bed.results if bed else {},
            "runtime_samples": bed.runtime_samples if bed else [],
            "event_requests": bed.event_requests if bed else [],
            "timeline": bed.timeline if bed else [],
            "wallclock_finished": now_stamp(),
            "elapsed_s": round(time.monotonic() - started_at, 1),
        }
        (out_dir / "b9_report.json").write_text(json.dumps(report, ensure_ascii=False, indent=1), encoding="utf-8")
        if browser:
            try:
                browser.close()
            except Exception:  # noqa: BLE001
                pass
        print(f"B9_EXIT {code}" + (f" ({blocked_reason})" if blocked_reason else ""), flush=True)
        return code

    with sync_playwright() as pw:
        try:
            browser, _label = launch_chromium(pw)
        except Exception as exc:  # noqa: BLE001
            print("browser launch failed: " + str(exc), flush=True)
            return finish(None, None, 2, f"browser_launch:{exc}")

        page = browser.new_page()
        bed = Bed(page, out_dir)

        def on_request(request) -> None:
            url = request.url
            if not bed.conversation_id and request.method == "POST" and url.endswith("/agent/chat"):
                try:
                    body = json.loads(request.post_data or "{}")
                    bed.conversation_id = str(body.get("conversation_id") or "")
                    bed.mark("conversation_id_captured", conversation_id=bed.conversation_id)
                except Exception:  # noqa: BLE001
                    pass
            if "/agent/events" in url:
                match = re.search(r"conversation_id=([^&]+)", url)
                if match and match.group(1) not in bed.event_requests:
                    bed.event_requests.append(match.group(1))
                    bed.mark("events_fetch", conversation_id=match.group(1))

        page.on("request", on_request)
        page.on("pageerror", lambda exc: bed.mark("page_error", error=str(exc)))

        # --- W0: page load -----------------------------------------------------
        try:
            page.goto(args.webui_url, wait_until="domcontentloaded", timeout=30000)
        except Exception as exc:  # noqa: BLE001
            return finish(bed, browser, 2, f"page_load:{exc}")
        deadline = time.monotonic() + args.accept_budget
        while time.monotonic() < deadline and bed.count("form.composer textarea") == 0:
            time.sleep(0.5)
        if bed.count("form.composer textarea") == 0:
            return finish(bed, browser, 2, "composer_not_present")
        baseline_block_ids = trace_block_ids(page)
        baseline_terminal_texts = [t for t in [m.strip() for m in bed.text("article.message-row.assistant .message-body").split(" | ")] if len(t) >= TERMINAL_MIN_CHARS]
        bed.shot("01_loaded")
        bed.mark("baseline", trace_blocks=baseline_block_ids, terminal_like=len(baseline_terminal_texts))

        # authority pre-set (manual-test form ran full access; mix_tick applies directly)
        switch_ok = False
        for _ in range(6):
            payload = json.dumps({"authority_mode": args.authority}).encode("utf-8")
            req = urllib.request.Request(f"{args.agent_http.rstrip('/')}/agent/authority", data=payload,
                                         headers={"Content-Type": "application/json"}, method="POST")
            try:
                with urllib.request.urlopen(req, timeout=6.0) as response:
                    body = json.loads(response.read().decode("utf-8", errors="replace"))
                switch_ok = str(body.get("authority_mode") or "") == args.authority
                if switch_ok:
                    break
            except Exception as exc:  # noqa: BLE001
                bed.mark("authority_switch_retry", error=str(exc))
            time.sleep(5.0)
        bed.mark("authority_precheck", mode=args.authority, ok=switch_ok)
        if not switch_ok:
            return finish(bed, browser, 2, "authority_switch_busy")

        # --- W1: submit the free-state chat round -------------------------------
        send_button = page.locator("form.composer button.send-button[type=submit]")
        try:
            send_button.first.wait_for(state="visible", timeout=15000)
        except Exception as exc:  # noqa: BLE001
            return finish(bed, browser, 2, f"send_button_missing:{exc}")
        if bed.count("button.send-button.stop-turn-button") > 0:
            return finish(bed, browser, 2, "composer_stuck_stop_turn")
        page.locator("form.composer textarea").first.fill(args.message)
        t_submit = time.monotonic()
        send_button.first.click()
        bed.mark("submitted", message=args.message)
        bed.shot("02_submitted")

        first_response_deadline = t_submit + args.chat_budget
        while time.monotonic() < first_response_deadline and not bed.conversation_id:
            time.sleep(0.5)
        if not bed.conversation_id:
            return finish(bed, browser, 2, "conversation_id_not_captured")
        bed.record("W1_submit", True, conversation_id=bed.conversation_id, submit_to_id_ms=round((time.monotonic() - t_submit) * 1000, 1))

        # --- W2: execution period sampling (symptom 1) ---------------------------
        event_since = 0
        terminal_seen_at: float | None = None
        last_snapshot: tuple[Any, ...] | None = None
        increments = 0
        live_content_samples = 0
        samples = 0
        loop_started = time.monotonic()
        terminal_deadline = loop_started + args.terminal_budget

        def poll_events() -> dict[str, Any] | None:
            nonlocal event_since
            url = f"{args.agent_http.rstrip('/')}/agent/events?conversation_id={bed.conversation_id}&since={event_since}&limit=120"
            payload = http_get_json(url)
            if payload is None:
                return None
            events = payload.get("events") or []
            next_seq = payload.get("next_seq")
            if isinstance(next_seq, (int, float)):
                event_since = max(event_since, int(next_seq))
            else:
                for event in events:
                    event_since = max(event_since, int(event.get("seq") or 0))
            return {"events": events, "next_seq": event_since}

        while time.monotonic() < terminal_deadline:
            poll = poll_events()
            runtime = http_get_json(f"{args.agent_http.rstrip('/')}/agent/runtime/status")
            goal = (runtime or {}).get("goal") or {}
            continuations = (runtime or {}).get("continuations") or []
            bed.runtime_samples.append({
                "wallclock": now_stamp(),
                "goal_status": str(goal.get("status") or ""),
                "continuation_statuses": [str(row.get("status") or "") for row in continuations],
            })
            live_blocks = bed.count("section.trace-block.is-live")
            think_text = bed.text("section.trace-block.is-live .trace-think-line")
            live_metas = bed.text("section.trace-block.is-live .th-meta")
            step_rows = bed.count("section.trace-block .trace-step")
            if live_blocks > 0 and (think_text.strip() != "" or step_rows > 0):
                live_content_samples += 1
            snapshot = (live_blocks, think_text, live_metas, step_rows, len(trace_block_ids(page)))
            if last_snapshot is not None and snapshot != last_snapshot:
                increments += 1
                bed.mark("execution_increment", sample=samples, snapshot=list(snapshot))
            last_snapshot = snapshot
            samples += 1
            if samples % 15 == 0:
                bed.shot("03_executing")
            if poll:
                for event in poll["events"]:
                    payload = event.get("payload") or {}
                    if str(event.get("type") or "") in ("turn.completed", "turn.failed", "turn.stopped") and payload.get("scheduler_chain"):
                        terminal_seen_at = time.monotonic()
                        bed.mark("terminal_event", seq=event.get("seq"), type=event.get("type"))
                        break
            if terminal_seen_at is not None:
                break
            time.sleep(1.0)

        if terminal_seen_at is None:
            return finish(bed, browser, 2, "no_chain_terminal_within_budget")
        elapsed = terminal_seen_at - t_submit
        bed.record(
            "W2_execution_period",
            increments >= 1 and live_content_samples >= 1,
            increments=increments,
            live_content_samples=live_content_samples,
            samples=samples,
            chain_elapsed_s=round(elapsed, 1),
            runtime_goal_statuses=sorted({row["goal_status"] for row in bed.runtime_samples}),
            runtime_any_live_continuation=any(
                any(status in ("pending", "claimed", "running", "waiting_interaction") for status in row["continuation_statuses"])
                for row in bed.runtime_samples
            ),
        )
        bed.shot("04_terminal")

        # --- W3: unified terminal block (symptoms 2+3) ---------------------------
        time.sleep(2.0)
        new_block_ids = [block for block in trace_block_ids(page) if block not in baseline_block_ids]
        page_text = bed.text("body") + " | " + bed.text(".message-stream")
        assistant_texts = [t for t in [m.strip() for m in bed.text("article.message-row.assistant .message-body").split(" | ")] if len(t) >= TERMINAL_MIN_CHARS]
        new_terminals = [t for t in assistant_texts if t not in baseline_terminal_texts]
        steps_in_new_block = bed.count(f'section.trace-block[data-turn-id="{new_block_ids[0]}"] .trace-step') if new_block_ids else 0
        bed.record(
            "W3_unified_terminal",
            len(new_block_ids) == 1
            and steps_in_new_block >= 1
            and EMPTY_SHELL_TEXT not in page_text
            and len(new_terminals) >= 1,
            new_trace_blocks=new_block_ids,
            steps_in_new_block=steps_in_new_block,
            empty_shell_text_present=EMPTY_SHELL_TEXT in page_text,
            new_terminal_messages=len(new_terminals),
        )
        bed.shot("05_unified_block")

        # --- W4: refresh survival (symptom 4) -------------------------------------
        terminal_before = new_terminals[:1]
        blocks_before = set(trace_block_ids(page))
        steps_before = steps_in_new_block
        bed.event_requests.clear()
        page.reload(wait_until="domcontentloaded")
        hydrate_deadline = time.monotonic() + args.reload_budget
        while time.monotonic() < hydrate_deadline and bed.count("form.composer textarea") == 0:
            time.sleep(0.5)
        # hydration settle: allow the event replay to rebuild the trajectory
        time.sleep(4.0)
        assistant_after = [t for t in [m.strip() for m in bed.text("article.message-row.assistant .message-body").split(" | ")] if len(t) >= TERMINAL_MIN_CHARS]
        terminal_survives = all(marker in " || ".join(assistant_after) for marker in terminal_before)
        blocks_after = set(trace_block_ids(page))
        restored_round_blocks = [block for block in blocks_after if block in set(new_block_ids)]
        steps_after = bed.count(f'section.trace-block[data-turn-id="{new_block_ids[0]}"] .trace-step') if new_block_ids else 0
        bed.record(
            "W4_refresh_survival",
            terminal_survives and len(restored_round_blocks) == 1 and steps_after >= 1,
            terminal_survives=terminal_survives,
            restored_round_blocks=restored_round_blocks,
            steps_before=steps_before,
            steps_after=steps_after,
            event_conversations_post_reload=bed.event_requests,
            blocks_before=sorted(blocks_before),
            blocks_after=sorted(blocks_after),
        )
        bed.shot("06_after_reload")

        all_ok = all(row["ok"] for row in bed.results.values())
        return finish(bed, browser, 0 if all_ok else 1)


if __name__ == "__main__":
    sys.exit(main())
