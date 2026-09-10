"""E2E-1 layer B driver: browser rendering/latency bed (R1-R7, T1-T6).

Drives the real WebUI (served by the agent at --webui-url, /app/ path) in
headless chromium through the same user loop as layer A: submit a natural
improvement instruction, approve both confirmation cards, wait for the chain
terminal, reload for recovery. Every step is screenshotted with wallclock
timestamps into --out-dir (demo/forensics evidence). DOM assertions follow
the anchors documented in the E2E-1 receipt (stable classnames/roles; the
frontend has no data-testid layer).

Conversation scoping: the composer renders runtime-level pending interaction
cards that are NOT conversation-scoped (fresh sessions see other
conversations' pending cards via continuations; dbg4 evidence 2026-09-09).
Cards and messages already present before submit are baselined and never
count as this conversation's own. Read-only against production code.
"""

from __future__ import annotations

import argparse
import json
import re
import time
from datetime import datetime
from pathlib import Path
from typing import Any

from playwright.sync_api import sync_playwright

ACK_MARKERS = ("还在继续处理",)
INTRO_MARKER = "Ask Vit 就绪。"
TERMINAL_MIN_CHARS = 30

LAUNCH_CHANNELS: list[dict[str, Any]] = [
    {"label": "bundled-chromium"},
    {"label": "msedge", "channel": "msedge"},
    {"label": "chrome", "channel": "chrome"},
]


def launch_chromium(pw):
    """Launch headless chromium with a channel fallback chain. The playwright
    CDN download can stall on this network; system Edge/Chrome serve the same
    engine. Returns (browser, channel_label)."""
    last_error: Exception | None = None
    for candidate in LAUNCH_CHANNELS:
        try:
            kwargs = {"headless": True}
            if candidate.get("channel"):
                kwargs["channel"] = candidate["channel"]
            return pw.chromium.launch(**kwargs), candidate["label"]
        except Exception as exc:  # noqa: BLE001 - try the next channel
            last_error = exc
    raise RuntimeError("no chromium channel launchable: " + str(last_error))


def probe_browser() -> bool:
    try:
        with sync_playwright() as _pw:
            browser, _label = launch_chromium(_pw)
            page = browser.new_page()
            page.set_content("<h1>ok</h1>")
            if page.inner_text("h1") != "ok":
                return False
            browser.close()
        return True
    except Exception:  # noqa: BLE001
        return False


def now_stamp() -> str:
    return datetime.now().astimezone().isoformat(timespec="milliseconds")


def shot_name(step: str) -> str:
    return f"{step}_{datetime.now().strftime('%H%M%S_%f')[:-3]}.png"


def card_selector_all() -> str:
    return ".composer-interaction-shell .capability-proposal-card, .action-card.interactive, .composer-interaction-shell .action-card"


APPROVE_RE = re.compile("确认|执行|同意|approve", re.I)
REJECT_RE = re.compile("取消|拒绝|不执行|reject|cancel", re.I)


class Bed:
    def __init__(self, page, out_dir: Path, shots_dir: Path) -> None:
        self.page = page
        self.out_dir = out_dir
        self.shots = shots_dir
        self.console: list[dict[str, str]] = []
        self.page_errors: list[str] = []
        self.conversation_id = ""
        self.timeline: list[dict[str, Any]] = []
        self.latencies: dict[str, Any] = {}
        self.results: dict[str, dict[str, Any]] = {}
        self.zero_step_sightings = 0
        self.busy_samples: list[dict[str, Any]] = []
        self.terminal_text_seen: str = ""

    def mark(self, label: str, **fields: Any) -> None:
        row = {"wallclock": now_stamp(), "monotonic_ms": round(time.monotonic() * 1000, 1), "label": label, **fields}
        self.timeline.append(row)
        print(f"[{row['wallclock']}] {label} " + json.dumps({k: v for k, v in fields.items()}, ensure_ascii=False), flush=True)

    def shot(self, step: str, note: str = "") -> None:
        path = self.shots / shot_name(step)
        try:
            self.page.screenshot(path=str(path), full_page=False)
            self.mark("shot", step=step, path=str(path), note=note)
        except Exception as exc:  # noqa: BLE001
            self.mark("shot_failed", step=step, error=str(exc))

    def record(self, segment: str, ok: bool, expected: str, **evidence: Any) -> None:
        row = {"segment": segment, "ok": ok, "expected": expected, "wallclock": now_stamp(), **evidence}
        self.results[segment] = row
        self.mark("segment_" + segment, ok=ok, expected=expected, **{k: v for k, v in evidence.items() if k not in ("history",)})

    def visible_text(self, selector: str) -> str:
        try:
            locator = self.page.locator(selector)
            if locator.count() == 0:
                return ""
            return " | ".join(locator.all_inner_texts()[:12])
        except Exception:  # noqa: BLE001
            return ""

    def poll_dom(self, predicate, timeout_s: float, interval_s: float = 0.5):
        deadline = time.monotonic() + timeout_s
        last_error = ""
        while time.monotonic() < deadline:
            try:
                if predicate():
                    return True, last_error
            except Exception as exc:  # noqa: BLE001
                last_error = f"{type(exc).__name__}: {exc}"
            time.sleep(interval_s)
        return False, last_error

    def save_stream_state(self, name: str) -> None:
        payload = {
            "wallclock": now_stamp(),
            "assistant_rows": self.page.locator(".message-stream article.message-row.assistant").count(),
            "assistant_texts": self.page.locator(".message-stream article.message-row.assistant").all_inner_texts()[-6:],
            "trace_blocks": self.page.locator("section.trace-block").count(),
            "trace_meta": self.page.locator("section.trace-block .th-meta").all_inner_texts(),
            "activity_lane": self.page.locator("section.activity-lane .activity-lane-item").all_inner_texts(),
            "audition_cards": self.page.locator("section.card[data-audition-session]").count(),
            "interactive_cards": self.page.locator(card_selector_all()).count(),
        }
        (self.out_dir / f"stream_state_{name}.json").write_text(json.dumps(payload, ensure_ascii=False, indent=1), encoding="utf-8")


def authority_state(bed: Bed) -> dict[str, Any]:
    button = bed.page.locator(".authority-select-button")
    state: dict[str, Any] = {"count": button.count()}
    if button.count():
        state["label"] = button.first.inner_text().strip().replace("\n", " ")
        state["disabled"] = button.first.is_disabled()
        state["title"] = button.first.get_attribute("title") or ""
    return state


def try_authority_switch(bed: Bed, target_label: str, timeout_s: float, agent_base: str) -> dict[str, Any]:
    """Open the selector and pick the option whose label contains target_label."""
    outcome: dict[str, Any] = {"target": target_label}
    button = bed.page.locator(".authority-select-button")
    if button.count() == 0:
        outcome["error"] = "authority button absent"
        return outcome
    if button.first.is_disabled():
        outcome["disabled"] = True
        outcome["title"] = button.first.get_attribute("title") or ""
        outcome["error"] = "button disabled (locked)"
        return outcome
    t0 = time.monotonic()
    button.first.click()
    menu = bed.page.locator(".authority-menu")
    try:
        menu.wait_for(state="visible", timeout=5000)
    except Exception as exc:  # noqa: BLE001
        outcome["error"] = f"menu did not open: {exc}"
        return outcome
    option = bed.page.locator(f".authority-menu button[role=menuitemradio]:has-text('{target_label}')")
    if option.count() == 0:
        outcome["error"] = "option absent in menu"
        return outcome
    option.first.click()
    ok, _ = bed.poll_dom(
        lambda: target_label in (authority_state(bed).get("label") or ""),
        timeout_s,
        interval_s=0.2,
    )
    outcome["ui_updated"] = ok
    outcome["ms"] = round((time.monotonic() - t0) * 1000, 1)
    outcome["after"] = authority_state(bed)
    try:
        response = bed.page.request.get(agent_base.rstrip("/") + "/agent/authority", timeout=5000)
        payload = response.json() if response.ok else {}
        outcome["server_mode"] = payload.get("authority_mode")
        outcome["server_status"] = response.status
    except Exception as exc:  # noqa: BLE001
        outcome["server_error"] = str(exc)
    notice = bed.page.locator(".error, .notice, [role=alert]")
    outcome["visible_notice"] = notice.first.inner_text().strip()[:200] if notice.count() else ""
    return outcome


def looks_like_terminal(text: str) -> bool:
    core = text.replace("Vit", " ").strip()
    if len(core) < TERMINAL_MIN_CHARS:
        return False
    if any(marker in text for marker in ACK_MARKERS):
        return False
    if INTRO_MARKER in text:
        return False
    # Card-bearing message rows (proposal ack with an embedded confirmation
    # card, e.g. "混音单步待确认 ... 确认执行 取消") are not terminal reports.
    if "待确认" in text or "确认执行" in text or "确认提案" in text:
        return False
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--webui-url", default="http://127.0.0.1:7878/app/")
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--message", default="请检查当前工程是否有什么问题吗")
    parser.add_argument("--t1-budget", type=float, default=5.0)
    parser.add_argument("--t2-budget", type=float, default=90.0)
    parser.add_argument("--card2-budget", type=float, default=120.0)
    parser.add_argument("--terminal-budget", type=float, default=180.0)
    parser.add_argument("--t4-budget", type=float, default=10.0)
    parser.add_argument("--t5-budget", type=float, default=10.0)
    parser.add_argument("--t6-budget", type=float, default=5.0)
    parser.add_argument("--approve-settle-budget", type=float, default=150.0)
    parser.add_argument("--expect-red", action="store_true")
    parser.add_argument("--f2-fixed", action="store_true", help="evaluate shape against the post-F2 expectation: R4 terminal message is green in the scheduled variant too (scheduler delivery gate covers the waiting park); R5's dead-card half stays red (F3 scope); R3 zero-step block stays B3-owned (recorded, not gated)")
    args = parser.parse_args()

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    shots_dir = out_dir / "shots"
    shots_dir.mkdir(parents=True, exist_ok=True)
    started_at = now_stamp()

    with sync_playwright() as pw:
        browser, browser_channel = launch_chromium(pw)
        print(f"browser_channel={browser_channel}", flush=True)
        context = browser.new_context(viewport={"width": 1600, "height": 1000}, locale="zh-CN")
        page = context.new_page()
        bed = Bed(page, out_dir, shots_dir)

        def on_console(message) -> None:
            bed.console.append({"type": message.type, "text": message.text[:400]})

        def on_page_error(error) -> None:
            bed.page_errors.append(str(error)[:400])

        def on_request(request) -> None:
            if not bed.conversation_id and request.method == "POST" and request.url.endswith("/agent/chat"):
                try:
                    body = json.loads(request.post_data or "{}")
                    bed.conversation_id = str(body.get("conversation_id") or "")
                    bed.mark("conversation_id_captured", conversation_id=bed.conversation_id)
                except Exception:  # noqa: BLE001
                    pass

        page.on("console", on_console)
        page.on("pageerror", on_page_error)
        page.on("request", on_request)

        page.goto(args.webui_url, wait_until="domcontentloaded", timeout=30000)
        ok, err = bed.poll_dom(lambda: page.locator("form.composer textarea").count() > 0, 15.0)
        bed.mark("page_loaded", ok=ok, error=err, url=page.url)
        if not ok:
            bed.record("R0_page_load", False, "green", error=err)
            finish(bed, browser, out_dir, started_at, args, blocked=True)
            return 2
        bed.shot("01_loaded")

        # --- baseline scoping (see module docstring) -----------------------------
        def collect_card_texts() -> list[str]:
            cards = page.locator(card_selector_all())
            return [" ".join(cards.nth(i).inner_text().split()) for i in range(cards.count())]

        def collect_assistant_texts() -> list[str]:
            rows = page.locator(".message-stream article.message-row.assistant")
            return [" ".join(rows.nth(i).inner_text().split()) for i in range(rows.count())]

        baseline_cards = [t for t in collect_card_texts() if t]
        baseline_assistants = [t for t in collect_assistant_texts() if t]
        bed.mark("baseline", foreign_pending_cards=baseline_cards, assistant_rows=baseline_assistants)

        def new_card_locator(require_approve: bool = True):
            """A card of THIS conversation: text not in baseline and (unless
            told otherwise) carrying an enabled approve button - the resolved
            form of an approved card changes its text and drops its buttons,
            which must not be mistaken for the next pending card."""
            cards = page.locator(card_selector_all())
            for index in range(cards.count()):
                text = " ".join(cards.nth(index).inner_text().split())
                if not text or text in baseline_cards:
                    continue
                if require_approve:
                    approve = cards.nth(index).locator("button").filter(has_text=APPROVE_RE)
                    enabled = 0
                    for bindex in range(approve.count()):
                        if approve.nth(bindex).is_enabled():
                            enabled += 1
                    if enabled == 0:
                        continue
                return cards.nth(index), text
            return None, ""

        # --- R6a / T6: idle-state authority switch (B4 fork adjudication) -------
        # Expected-status is "record": this segment adjudicates B4's two-fork
        # question. Evidence so far (dbg4): clean stack => switch succeeds;
        # stack with a parked live turn => server 409 + UX-1 human notice,
        # i.e. H-A (B1 downstream), not an independent idle-state defect.
        before = authority_state(bed)
        bed.mark("authority_before", state=before)
        switch_a = try_authority_switch(bed, "完全访问", args.t6_budget, args.agent_http)
        bed.latencies["T6_idle_switch_ms"] = switch_a.get("ms")
        bed.shot("02_authority_switch_idle")
        server_consistent = switch_a.get("server_mode") == "full_project_access"
        bed.record(
            "R6a_idle_switch",
            bool(switch_a.get("ui_updated")) and server_consistent,
            "record",
            switch=switch_a,
            before=before,
            note="green => idle switch works (B4 is B1-downstream, H-A); red-with-409-notice => stuck turn blocks it (still H-A); red-without-notice => H-B independent defect",
        )
        if switch_a.get("ui_updated"):
            back = try_authority_switch(bed, "普通确认", args.t6_budget, args.agent_http)
            bed.mark("authority_switch_back", state=back)
            bed.shot("03_authority_switch_back")

        # --- submit instruction (T1/T2 anchors) -----------------------------------
        send_button = page.locator("form.composer button.send-button[type=submit]")
        try:
            send_button.first.wait_for(state="visible", timeout=20000)
        except Exception:  # noqa: BLE001
            if page.locator("button.send-button.stop-turn-button").count() > 0:
                bed.record(
                    "R0_submit_blocked",
                    False,
                    "green",
                    note="composer is stuck in Stop Turn form: a live/parked agent turn from a previous session blocks new submissions (agentTurnRunning latch)",
                )
                bed.shot("00_submit_blocked")
                finish(bed, browser, out_dir, started_at, args, blocked=True)
                return 2
            raise
        textarea = page.locator("form.composer textarea").first
        textarea.fill(args.message)
        t_submit = time.monotonic()
        send_button.first.click()
        bed.mark("submitted", message=args.message)
        bed.shot("04_submitted")

        ok_t1, _ = bed.poll_dom(
            lambda: page.locator("section.activity-lane .activity-lane-item").count() > 0
            or page.locator("p.typing-message").count() > 0
            or page.locator("section.plan-bar").count() > 0
            or page.locator("button.send-button.stop-turn-button").count() > 0,
            args.t1_budget,
            interval_s=0.2,
        )
        bed.latencies["T1_acceptance_ms"] = round((time.monotonic() - t_submit) * 1000, 1)
        plan_bar = page.locator("section.plan-bar").count() > 0
        stop_turn = page.locator("button.send-button.stop-turn-button").count() > 0
        bed.record(
            "R1_acceptance_state",
            ok_t1 and (plan_bar or stop_turn),
            "green",
            t1_ok=ok_t1,
            plan_bar=plan_bar,
            stop_turn=stop_turn,
            t1_ms=bed.latencies["T1_acceptance_ms"],
            activity=bed.visible_text("section.activity-lane .activity-lane-item"),
        )
        bed.shot("05_acceptance")

        # --- confirmation card 1 (T2 / R2) ----------------------------------------
        ok_card, _ = bed.poll_dom(lambda: new_card_locator()[0] is not None, args.t2_budget)
        bed.latencies["T2_card_visible_ms"] = round((time.monotonic() - t_submit) * 1000, 1)
        card1_el, card1_text = new_card_locator()
        card1_header = card1_text.split("|")[0].strip() if card1_text else ""
        approve1_btn = card1_el.locator("button").filter(has_text=APPROVE_RE) if card1_el else None
        reject1_btn = card1_el.locator("button").filter(has_text=REJECT_RE) if card1_el else None
        bed.record(
            "R2_card_render",
            ok_card and approve1_btn is not None and approve1_btn.count() > 0 and reject1_btn is not None and reject1_btn.count() > 0,
            "green",
            card_visible=ok_card,
            approve_buttons=approve1_btn.count() if approve1_btn else 0,
            reject_buttons=reject1_btn.count() if reject1_btn else 0,
            card_header=card1_header,
            card_text_head=card1_text[:160],
            t2_ms=bed.latencies["T2_card_visible_ms"],
        )
        bed.shot("06_card1")
        if not ok_card:
            finish(bed, browser, out_dir, started_at, args, blocked=True)
            return 2

        # --- approve card 1 (T3: visual feedback, record-only pre-B2) -------------
        time.sleep(2.0)  # human-scale pacing; kernel render drain insurance
        t_approve1 = time.monotonic()
        approve1_btn.first.click()
        ok_t3, _ = bed.poll_dom(
            lambda: page.locator(card_selector_all()).count() == 0
            or page.locator(".capability-proposal-card .btn.approve:disabled").count() > 0
            or page.locator("section.activity-lane .activity-lane-item").count() > 0,
            5.0,
            interval_s=0.2,
        )
        bed.latencies["T3_approve1_feedback_ms"] = round((time.monotonic() - t_approve1) * 1000, 1)
        bed.mark("approve1_clicked", t3_ok=ok_t3, t3_ms=bed.latencies["T3_approve1_feedback_ms"], note="record-only until B2 lands")
        bed.shot("07_approve1")

        # --- card 2 ---------------------------------------------------------------
        ok_card2, _ = bed.poll_dom(lambda: new_card_locator()[0] is not None and new_card_locator()[1] != card1_text, args.card2_budget)
        card2_el, card2_text = new_card_locator()
        bed.mark("card2_visible", ok=ok_card2, card_text_head=(card2_text or "")[:120])
        bed.shot("08_card2")
        if not ok_card2:
            finish(bed, browser, out_dir, started_at, args, blocked=True)
            return 2
        approve2_btn = card2_el.locator("button").filter(has_text=APPROVE_RE)
        assistants_before_approve2 = [t for t in collect_assistant_texts() if t]
        time.sleep(2.0)  # human-scale pacing; kernel render drain insurance
        t_approve2 = time.monotonic()
        approve2_btn.first.click()
        bed.mark("approve2_clicked")
        bed.shot("09_approve2")

        # --- R3: trajectory streams steps while chain executes ---------------------
        import urllib.request as _ur

        event_since = 0
        event_terminal_at = None
        event_terminal_info: dict[str, Any] | None = None

        def poll_terminal_event() -> None:
            nonlocal event_since, event_terminal_at, event_terminal_info
            if not bed.conversation_id or event_terminal_at is not None:
                return
            url = f"{args.agent_http.rstrip('/')}/agent/events?conversation_id={bed.conversation_id}&since={event_since}&limit=120"
            try:
                with _ur.urlopen(url, timeout=5.0) as response:
                    payload = json.loads(response.read().decode("utf-8", errors="replace"))
            except Exception:  # noqa: BLE001 - polling must survive transient errors
                return
            events = payload.get("events") or []
            next_seq = payload.get("next_seq")
            if isinstance(next_seq, (int, float)):
                event_since = max(event_since, int(next_seq))
            else:
                for event in events:
                    event_since = max(event_since, int(event.get("seq") or 0))
            for event in events:
                body = str(event.get("body") or "")
                status = str(event.get("status") or "")
                is_scheduler_chain = bool((event.get("payload") or {}).get("scheduler_chain"))
                is_ack = any(marker in body for marker in ACK_MARKERS)
                # Only true chain-terminal events anchor T4: goal reached a
                # terminal status (completed/failed/stopped) or the
                # scheduler_chain result. Slice acks carry waiting_continue /
                # waiting_confirmation statuses and must not match (approve1's
                # proposal ack is status=waiting_confirmation).
                if str(event.get("type")) in {"turn.completed", "turn.failed", "turn.stopped"} and not is_ack and (is_scheduler_chain or status in {"completed", "failed", "stopped"}):
                    event_terminal_at = time.monotonic()
                    event_terminal_info = {"seq": event.get("seq"), "type": event.get("type"), "scheduler_chain": is_scheduler_chain, "status": status, "body_head": body[:140]}
                    bed.mark("terminal_event_seen_in_stream", **event_terminal_info)

        def assistant_terminal_locator():
            rows = page.locator(".message-stream article.message-row.assistant")
            # 20260910_212325 diag 取证实锤：调度变体的终局 reply 与 approve2 前
            # 已在 DOM 的提案卡行同文（park 片 reply 就是提案摘要），baseline
            # 精确成员排除把 F7 气泡永久吃掉（R4/R5 双红的床侧伪影）。见过服务
            # 端终局事件后，改按事件 body 锚定匹配（layer A S6 同口径：body 前
            # 缀 ⊆ 行文本）——事件 body 是权威终局文本，baseline 排除不再适用；
            # 带按钮的活卡仍排除。
            anchor = ""
            if event_terminal_info:
                anchor = " ".join(str(event_terminal_info.get("body_head") or "").split())[:60]
            for index in range(rows.count() - 1, -1, -1):
                row = rows.nth(index)
                text = " ".join(row.inner_text().split())
                if anchor and anchor in text:
                    if row.locator("button").count() > 0:
                        continue
                    return row, text
                if not text or text in baseline_assistants or text in assistants_before_approve2:
                    continue
                if not looks_like_terminal(text):
                    continue
                if row.locator("button").count() > 0:
                    # rows carrying actionable controls are cards, not reports
                    continue
                return row, text
            return None, ""

        # F2 取证：终局定位器在 20260910_202854/204745 两轮出现"轮询判否、随后
        # 快照却可见"的矛盾。给每个排除子句记账，R3 循环内节流采样，R4/R5 把
        # 最后拒绝原因带进断言证据——一次跑定位真因。
        locator_rejects: dict[str, int] = {}
        locator_reject_samples: list[str] = []
        locator_last_reject = ""

        def assistant_terminal_locator_diag():
            nonlocal locator_last_reject
            rows = page.locator(".message-stream article.message-row.assistant")
            for index in range(rows.count() - 1, -1, -1):
                row = rows.nth(index)
                text = " ".join(row.inner_text().split())
                if not text:
                    locator_rejects["empty"] = locator_rejects.get("empty", 0) + 1
                    continue
                if text in baseline_assistants or text in assistants_before_approve2:
                    locator_rejects["baseline"] = locator_rejects.get("baseline", 0) + 1
                    if len(locator_reject_samples) < 12:
                        locator_reject_samples.append("baseline:" + text[:80])
                    continue
                if not looks_like_terminal(text):
                    locator_rejects["not_terminal_like"] = locator_rejects.get("not_terminal_like", 0) + 1
                    if len(locator_reject_samples) < 12:
                        locator_reject_samples.append("not_terminal_like:" + text[:80])
                    continue
                if row.locator("button").count() > 0:
                    # rows carrying actionable controls are cards, not reports
                    locator_rejects["buttons"] = locator_rejects.get("buttons", 0) + 1
                    if len(locator_reject_samples) < 12:
                        locator_reject_samples.append("buttons:" + text[:80])
                    continue
                locator_last_reject = ""
                return row, text
            locator_last_reject = json.dumps({"rows": rows.count(), "rejects": locator_rejects, "samples": locator_reject_samples[-6:]}, ensure_ascii=False)
            return None, ""

        ok_trace, _ = bed.poll_dom(lambda: page.locator("section.trace-block").count() > 0, 60.0)
        steps_seen = 0
        live_updates = 0
        # B3 取证位：live 块步数 meta 在执行期间的变更次数（步数流式增量的直接
        # 证据）。think-line 的 live_updates 依赖 item 活动归属 turn（服务端
        # attribution，AGENT-F4 域），链活期恒 0；步数增量由本位承载。
        live_step_updates = 0
        live_step_samples: list[str] = []
        last_activity_text = ""
        last_live_step_text = ""
        loop_started = time.monotonic()
        busy_deadline = loop_started + args.terminal_budget
        dom_terminal_during_loop = False
        loop_iteration = 0
        while time.monotonic() < busy_deadline:
            poll_terminal_event()
            metas = page.locator("section.trace-block .th-meta").all_inner_texts()
            for meta in metas:
                match = re.match(r"^(\d+)\s*步", meta.strip())
                if match:
                    steps_seen = max(steps_seen, int(match.group(1)))
                    if int(match.group(1)) == 0:
                        bed.zero_step_sightings += 1
            activity_text = bed.visible_text("section.trace-block .trace-think-line")
            if activity_text and activity_text != last_activity_text:
                live_updates += 1
                last_activity_text = activity_text
            live_step_metas = page.locator("section.trace-block.is-live .th-meta").all_inner_texts()
            live_step_texts = [m.strip() for m in live_step_metas if re.match(r"^[1-9]\d*\s*步", m.strip())]
            if live_step_texts:
                joined = "|".join(live_step_texts)
                live_step_samples.append(joined)
                if joined != last_live_step_text:
                    live_step_updates += 1
                    last_live_step_text = joined
            bed.busy_samples.append({"wallclock": now_stamp(), **authority_state(bed)})
            loop_iteration += 1
            if loop_iteration % 10 == 0:
                assistant_terminal_locator_diag()
            if assistant_terminal_locator()[0] is not None:
                dom_terminal_during_loop = True
                break
            time.sleep(1.0)
        loop_ended = time.monotonic()
        bed.record(
            "R3_trajectory_streams",
            ok_trace and (steps_seen > 0 or live_updates > 0) and bed.zero_step_sightings == 0,
            "green",
            trace_blocks=ok_trace,
            steps_seen=steps_seen,
            live_updates=live_updates,
            live_step_updates=live_step_updates,
            live_step_samples=live_step_samples,
            zero_step_sightings=bed.zero_step_sightings,
        )

        # --- R6b: busy-state authority switch (record both signals) ---------------
        switch_busy = try_authority_switch(bed, "完全访问", args.t6_budget, args.agent_http)
        bed.shot("10_authority_busy")
        bed.record(
            "R6b_busy_switch",
            bool(switch_busy.get("visible_notice")),
            "record",
            switch=switch_busy,
            note="human-readable rejection (409 + UX-1 notice) counts as correct busy behavior; disabled-button silence counts as the silent form",
        )
        if switch_busy.get("ui_updated"):
            try_authority_switch(bed, "普通确认", args.t6_budget, args.agent_http)

        # --- R4 / T4: terminal message in conversation flow ------------------------
        t_terminal_dom = None
        terminal_row, terminal_text = assistant_terminal_locator()
        if terminal_row is None:
            ok_terminal, _ = bed.poll_dom(lambda: assistant_terminal_locator()[0] is not None, args.t4_budget, interval_s=0.5)
            if ok_terminal:
                terminal_row, terminal_text = assistant_terminal_locator()
        if terminal_row is not None:
            t_terminal_dom = time.monotonic()
            bed.terminal_text_seen = terminal_text
        if t_terminal_dom is not None and event_terminal_at is not None and t_terminal_dom >= event_terminal_at:
            bed.latencies["T4_terminal_render_ms"] = round((t_terminal_dom - event_terminal_at) * 1000, 1)
        elif dom_terminal_during_loop:
            bed.latencies["T4_terminal_render_ms"] = "dom-visible-before-event-or-during-r3-loop"
        else:
            bed.latencies["T4_terminal_render_ms"] = None
        spinner_cleared = page.locator("section.trace-block .trace-think").count() == 0
        send_restored = page.locator("button.send-button.stop-turn-button").count() == 0
        # F2 形状档：R4 门收窄到 F2 自有维度（终局消息 + 清场）。send_restored
        # 不参与判定——GUI-F3 设计上让忙态 Stop 覆盖仍驻留可应答的链（判定
        # park 期 Stop 在场是正确形态，20260910_214010 实证），链完成（跨会话
        # 应答或 settle）后才恢复（20260910_210958 实证 True）；该维度归
        # B2/GUI-F3 家族。
        r4_ok = terminal_row is not None and spinner_cleared
        bed.record(
            "R4_terminal_message",
            r4_ok,
            "red",
            terminal_message=terminal_row is not None,
            terminal_text_head=(terminal_text or "")[:140],
            spinner_cleared=spinner_cleared,
            send_restored=send_restored,
            terminal_event_arrived=event_terminal_at is not None,
            terminal_event=event_terminal_info,
            t4_ms=bed.latencies.get("T4_terminal_render_ms"),
            locator_diag=locator_last_reject,
        )
        bed.save_stream_state("terminal")
        bed.shot("11_terminal_state")

        # --- R7: A/B audition card presentation ------------------------------------
        ok_audition, _ = bed.poll_dom(lambda: page.locator("section.card[data-audition-session]").count() > 0, 20.0)
        rtag = bed.visible_text("section.card[data-audition-session] .rtag")
        chip = bed.visible_text("section.card[data-audition-session] .chip")
        vbtns = page.locator("section.card[data-audition-session] .vbtn").count()
        bed.record(
            "R7_audition_card",
            ok_audition and bool(rtag),
            "record",
            card_visible=ok_audition,
            round_tag=rtag,
            chip=chip,
            verdict_buttons=vbtns,
            note="presentation correctness only; human-ear verdict out of scope",
        )
        bed.shot("12_audition_card")

        # --- R5 / T5: refresh recovery + dead-card guard ----------------------------
        our_card_texts = [t for t in (card1_text, card2_text) if t]
        t_reload = time.monotonic()
        page.reload(wait_until="domcontentloaded")
        ok_composer, _ = bed.poll_dom(lambda: page.locator("form.composer textarea").count() > 0, 15.0)
        ok_terminal_reload, _ = bed.poll_dom(lambda: assistant_terminal_locator()[0] is not None, args.t5_budget, interval_s=0.5)
        bed.latencies["T5_reload_restore_ms"] = round((time.monotonic() - t_reload) * 1000, 1)

        def our_dead_card_interactive() -> int:
            cards = page.locator(card_selector_all())
            count = 0
            for index in range(cards.count()):
                text = " ".join(cards.nth(index).inner_text().split())
                for ours in our_card_texts:
                    if ours and ours[:60] in text:
                        enabled = cards.nth(index).locator("button:not([disabled])").count()
                        if enabled > 0:
                            count += 1
            return count

        dead_cards = our_dead_card_interactive()
        bed.record(
            "R5_refresh_recovery",
            ok_terminal_reload and dead_cards == 0,
            "red",
            composer_ok=ok_composer,
            terminal_survives=ok_terminal_reload,
            our_consumed_cards_reinteractive=dead_cards,
            t5_ms=bed.latencies["T5_reload_restore_ms"],
            locator_diag=locator_last_reject,
        )
        bed.save_stream_state("after_reload")
        bed.shot("13_after_reload")

        matched = finish(bed, browser, out_dir, started_at, args)
        return 0 if matched else 1


def finish(bed: Bed, browser, out_dir: Path, started_at: str, args: Any, blocked: bool = False) -> bool:
    # Card table (from the B1 scheduled-variant + manual-test shape): R4/R5 red.
    # The inline variant (chain finished inside approve2's respond) legitimately
    # delivers the terminal through the fast respond body - the browser renders
    # and persists it (official run 20260909_220134: 891ms render, survived
    # reload). R5 still stays red there when consumed cards return interactive
    # (dead-card guard, F3 scope). R3's zero-step check is variant-independent.
    chain_variant = "scheduled"
    r4 = bed.results.get("R4_terminal_message") or {}
    terminal_event = r4.get("terminal_event") or {}
    if terminal_event and not terminal_event.get("scheduler_chain"):
        chain_variant = "inline"
    expected_green = {"R0_page_load", "R1_acceptance_state", "R2_card_render", "R3_trajectory_streams"}
    expected_red = {"R4_terminal_message", "R5_refresh_recovery"} if chain_variant == "scheduled" else {"R5_refresh_recovery"}
    # Post-F2+F3 combined shape: the delivery gate covers the scheduled
    # variant's waiting park (R4 green), and R5 goes green in BOTH halves —
    # terminal survives reload via the conversation-graph node (F2), consumed
    # cards lose interactivity via the interaction guard (F3 receipt §5
    # pre-declared this). R3's zero-step block is the B3-owned deviation and is
    # recorded, not gated.
    f2_green = {"R0_page_load", "R1_acceptance_state", "R2_card_render", "R4_terminal_message", "R5_refresh_recovery"}
    f2_either = {"R3_trajectory_streams"}
    f2_red = set()
    mismatches: list[str] = []
    for name, row in bed.results.items():
        if row.get("expected") == "record":
            continue
        if args.f2_fixed:
            if name in f2_green and not row["ok"]:
                mismatches.append(f"{name} expected green (f2_fixed/{chain_variant}), got red")
            elif name in f2_red and row["ok"]:
                mismatches.append(f"{name} expected red (f2_fixed/{chain_variant}), got green")
            continue
        if not args.expect_red:
            if not row["ok"]:
                mismatches.append(f"{name} expected green, got red")
        elif name in expected_green and not row["ok"]:
            mismatches.append(f"{name} expected green, got red")
        elif name in expected_red and row["ok"]:
            mismatches.append(f"{name} expected red ({chain_variant}), got green")
    report = {
        "layer": "B",
        "started_at": started_at,
        "finished_at": now_stamp(),
        "conversation_id": bed.conversation_id,
        "blocked": blocked,
        "results": bed.results,
        "latencies_ms": bed.latencies,
        "zero_step_sightings": bed.zero_step_sightings,
        "busy_samples_tail": bed.busy_samples[-8:],
        "console_tail": bed.console[-40:],
        "page_errors": bed.page_errors,
        "shots": sorted(str(p.name) for p in (out_dir / "shots").glob("*.png")) if (out_dir / "shots").exists() else [],
        "shape_summary": {
            "expect_red": args.expect_red,
            "f2_fixed": args.f2_fixed,
            "chain_variant": chain_variant,
            "mismatches": mismatches,
            "matches_expectation": (not mismatches) and not blocked,
            "note": ("post-F2+F3 combined shape: R4 and R5 (both halves) green, R3 recorded as B3-owned"
                      if args.f2_fixed else
                      "inline variant renders+persists the terminal via the fast respond body; scheduled variant matches the card table; R5 dead-card half stays red in both"),
        },
    }
    (out_dir / "e2e_layer_b_report.json").write_text(json.dumps(report, ensure_ascii=False, indent=1), encoding="utf-8")
    (out_dir / "e2e_layer_b_timeline.json").write_text(json.dumps({"rows": bed.timeline}, ensure_ascii=False, indent=1), encoding="utf-8")
    browser.close()
    print("LAYER_B_DONE " + json.dumps(report["shape_summary"], ensure_ascii=False), flush=True)
    return bool(report["shape_summary"]["matches_expectation"])


if __name__ == "__main__":
    raise SystemExit(main())
