#!/usr/bin/env python3
"""Declarative interaction-card button-click chain smoke (FE-INTERACTION-CARD-KIT-1).

Stack shape: kernel + agent berth via dev_agent_smoke.ps1 -StartKernel
-RestartAgent (no UI berth), isolated project (VIT_PROJECT_XML), webui served
by the vite dev server (worktree agent/webui) proxying to the REAL agent HTTP
face, driven in a REAL browser (Edge channel via python-playwright). Zero LLM
participation: the choreography routing is vocabulary-driven and takes over
before any model turn (AGENTS §8 declaration: deterministic chain).

Legs:

B1. browser button-click round (primary assertion): the phrase "0 到 24 秒，
    让这条轨的存在感慢慢强起来" sent from the real webui page must render the
    confirmation card through the declarative contract (interaction_card.v1:
    six fields shape/start_value/end_value/curve/region_start/region_end with
    declared types, approve/cancel actions) — the card carries BUTTONS, not a
    degraded plain-text reply. Editing start/end/curve on the card and
    clicking 确认写入 must execute the governed write with the EDITED values
    (kernel get_curve per-point verification: 0.20 -> 0.80 over 0-24s).
R2. phrasing-confirmation control round (coexistence proof): a second track,
    same chain driven through the agent chat face with the classic wording
    reply 「确认写入」 — the legacy confirmation path must keep working
    unchanged next to the button path; card payload must also carry the
    declarative card contract.
R3. cancel leg: a third proposal answered through the interaction responder
    with action_id=cancel must retire the plan with zero writes.

Exit code 0 only if every leg passes; summary JSON artifact at --output.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
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

try:
    from playwright.sync_api import sync_playwright
except ImportError:
    print("pip install playwright", file=sys.stderr)
    sys.exit(1)

CHAT_TIMEOUT_SEC = 60.0

DECLARED_FIELD_TYPES = {
    "shape": "select",
    "start_value": "number",
    "end_value": "number",
    "curve": "range",
    "region_start": "number",
    "region_end": "number",
}


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


def respond(base_url: str, interaction_id: str, action_id: str, fields: Dict[str, str]) -> Dict[str, Any]:
    return post_json(base_url, "/agent/interaction/respond", {
        "interaction_id": interaction_id,
        "action_id": action_id,
        "decision": action_id,
        "payload": {"fields": fields},
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


def kernel_points(kernel_zmq: str, plugin_id: str, param_id: str) -> List[Dict[str, Any]]:
    points = kernel_curve(kernel_zmq, plugin_id, param_id).get("points")
    require(isinstance(points, list), f"kernel curve points not a list: {points!r}")
    return [point for point in points if isinstance(point, dict)]


def wait_vite_server(log_path: Path, timeout_s: float) -> str:
    """Parse the Local: URL out of the vite dev log (strictPort=false may bump).

    The banner line carries ANSI color codes between "Local:" and the URL, so
    strip them before matching.
    """
    deadline = time.time() + timeout_s
    ansi_re = re.compile(r"\x1b\[[0-9;]*m")
    pattern = re.compile(r"Local:\s*(https?://\S+?)(?:/app/)?\s*$", re.IGNORECASE | re.MULTILINE)
    while time.time() < deadline:
        try:
            text = ansi_re.sub("", log_path.read_text(encoding="utf-8", errors="replace"))
            match = pattern.search(text)
            if match:
                return match.group(1).rstrip("/")
        except FileNotFoundError:
            pass
        time.sleep(0.5)
    raise RuntimeError(f"vite dev server did not report a Local URL within {timeout_s}s (log: {log_path})")


def wait_http_ok(url: str, timeout_s: float) -> None:
    deadline = time.time() + timeout_s
    last_error: Optional[Exception] = None
    while time.time() < deadline:
        try:
            with urllib.request.urlopen(url, timeout=2.0) as response:
                if response.status == 200:
                    return
        except Exception as exc:  # noqa: BLE001 - smoke retry diagnostic.
            last_error = exc
        time.sleep(0.5)
    raise RuntimeError(f"{url} did not answer 200: {last_error}")


def set_native_input_value(page: Any, selector: str, value: str) -> None:
    """Set a controlled React input's value (range sliders reject .fill())."""
    page.evaluate(
        """([selector, value]) => {
            const el = document.querySelector(selector);
            if (!el) { throw new Error('missing ' + selector); }
            const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
            const setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
            setter.call(el, value);
            el.dispatchEvent(new Event('input', { bubbles: true }));
        }""",
        [selector, value],
    )


def run_browser_round(webui_origin: str, agent_http: str, phrase: str, edits: Dict[str, str], shots_dir: Path) -> Dict[str, Any]:
    """B1: the real-page journey — phrase -> declarative card -> edit -> click."""
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(channel="msedge", headless=True)
        try:
            page = browser.new_page()
            page.goto(webui_origin + "/app/", wait_until="domcontentloaded")
            composer = page.locator('textarea[placeholder="和 Vit 说..."]')
            composer.wait_for(state="visible", timeout=30000)
            composer.fill(phrase)
            page.keyboard.press("Enter")
            card = page.locator('[data-card-schema="interaction_card.v1"]')
            card.wait_for(state="visible", timeout=60000)

            dom = page.evaluate(
                """() => {
                    const card = document.querySelector('[data-card-schema="interaction_card.v1"]');
                    const schema = card.getAttribute('data-card-schema');
                    const fields = [...card.querySelectorAll('[data-field-id]')].map((f) => ({ id: f.dataset.fieldId, type: f.dataset.fieldType }));
                    const buttons = [...card.querySelectorAll('[data-action-id]')].map((b) => ({ id: b.dataset.actionId, label: b.textContent.trim() }));
                    return { schema, fields, buttons };
                }"""
            )
            require(dom["schema"] == "interaction_card.v1", f"B1 card schema = {dom['schema']!r}")
            field_map = {field["id"]: field["type"] for field in dom["fields"]}
            require(field_map == DECLARED_FIELD_TYPES, f"B1 declared field types = {field_map!r}, want {DECLARED_FIELD_TYPES!r}")
            button_map = {button["id"]: button["label"] for button in dom["buttons"]}
            require(button_map.get("approve") == "确认写入", f"B1 approve button = {button_map!r}")
            require(button_map.get("cancel") == "取消", f"B1 cancel button = {button_map!r}")
            require(len(dom["buttons"]) == 2, f"B1 button count = {len(dom['buttons'])}")

            shots_dir.mkdir(parents=True, exist_ok=True)
            card.screenshot(path=str(shots_dir / "b1_card_before_click.png"))

            for field_id, value in edits.items():
                selector = f'[data-field-id="{field_id}"] input'
                if field_id == "curve":
                    set_native_input_value(page, selector, value)
                else:
                    page.locator(selector).fill(value)
            readback = page.evaluate(
                """(ids) => ids.map((id) => {
                    const input = document.querySelector(`[data-field-id="${id}"] input`);
                    return input ? input.value : null;
                })""",
                sorted(edits.keys()),
            )
            edited_values = dict(zip(sorted(edits.keys()), readback))
            for field_id, value in edits.items():
                require(approx(float(edited_values[field_id]), float(value), 1e-9),
                        f"B1 edited {field_id} = {edited_values[field_id]!r}, want {value}")

            page.locator('[data-action-id="approve"]').click()
            page.wait_for_selector("text=已写入", timeout=90000)
            page.screenshot(path=str(shots_dir / "b1_settled.png"), full_page=False)
            remaining = page.locator('[data-action-id="approve"]').count()
            require(remaining == 0, f"B1 approve button still clickable after settle ({remaining})")
            return {
                "schema": dom["schema"],
                "fields": field_map,
                "buttons": button_map,
                "edited_values": edited_values,
            }
        finally:
            browser.close()


def assert_declarative_contract(card: Dict[str, Any], tag: str) -> Dict[str, Any]:
    """The interaction card must carry the interaction_card.v1 contract."""
    payload = card.get("payload") if isinstance(card.get("payload"), dict) else {}
    declared = payload.get("card") if isinstance(payload.get("card"), dict) else {}
    require(declared.get("schema") == "interaction_card.v1", f"{tag} payload.card.schema = {declared.get('schema')!r}")
    fields = [row for row in (declared.get("fields") or []) if isinstance(row, dict)]
    field_types = {str(row.get("id")): str(row.get("type")) for row in fields}
    require(field_types == DECLARED_FIELD_TYPES, f"{tag} payload.card fields = {field_types!r}")
    actions = [row for row in (declared.get("actions") or []) if isinstance(row, dict)]
    action_ids = sorted(str(row.get("action_id") or "") for row in actions)
    require(action_ids == ["approve", "cancel"], f"{tag} payload.card actions = {action_ids!r}")
    return declared


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--agent-http", default="http://127.0.0.1:7878")
    parser.add_argument("--kernel-zmq", default="tcp://127.0.0.1:5555")
    parser.add_argument("--webui-dir", default="", help="worktree agent/webui dir; the script spawns `npm run dev` with VITE_AGENT_PROXY pointed at the agent")
    parser.add_argument("--timeout-sec", type=int, default=300)
    parser.add_argument("--project-path", default="")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    import tempfile
    project_path = Path(args.project_path).resolve() if args.project_path else Path(tempfile.gettempdir()) / f"automation_card_button_{int(time.time())}.vit"

    legs: List[str] = []
    summary: Dict[str, Any] = {
        "scenario": "FE-INTERACTION-CARD-KIT-1 declarative card button-click chain",
        "agent_http": args.agent_http,
        "kernel_zmq": args.kernel_zmq,
        "project_path": str(project_path),
        "llm_participation": "none (deterministic vocabulary routing takes over before any model turn)",
        "started_at": datetime.now(timezone.utc).isoformat(),
        "legs": legs,
    }

    def record(name: str, detail: Dict[str, Any]) -> None:
        legs.append(name)
        summary[name] = detail
        print(f"[ok] {name}: {json.dumps(detail, ensure_ascii=False)[:400]}")

    vite_process: Optional[subprocess.Popen] = None
    vite_log_path: Path = Path(tempfile.gettempdir()) / f"vite_card_button_{int(time.time())}.log"
    try:
        wait_agent_http(args.agent_http, args.timeout_sec)
        wait_kernel_zmq(args.kernel_zmq, args.timeout_sec)

        # ---- setup: isolated project with ONE track (single-track resolver
        # fallback keeps the browser round deterministic without UI selection)
        project_path.parent.mkdir(parents=True, exist_ok=True)
        reply = zmq_command(args.kernel_zmq, {"cmd": "new_project", "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"new_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "save_as_project", "file_path": str(project_path), "command_timeout_ms": 60000})
        require(str(reply.get("status", "")).lower() in {"ok", "success"}, f"save_as_project failed: {reply}")
        reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
        track1 = str(reply.get("track_id") or reply.get("id") or "").strip()
        require(track1, f"add_audio_track returned no track_id: {reply}")
        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        plugin1 = find_track_volume_plugin(state, track1)
        require(bool(plugin1), f"no built-in Volume & Pan plugin on track {track1}")
        require(len(kernel_points(args.kernel_zmq, plugin1, "volume")) == 0, "setup baseline: volume curve must start empty")
        record("setup", {"track1": track1, "track1_volume_plugin": plugin1})

        # ---- LEG B1: browser button-click round (primary) ---------------
        require(bool(args.webui_dir), "--webui-dir is required for the browser leg")
        vite_env = {**os.environ, "VITE_AGENT_PROXY": args.agent_http}
        vite_log = open(vite_log_path, "w", encoding="utf-8", errors="replace")
        vite_process = subprocess.Popen(
            ["npm.cmd", "run", "dev"], cwd=args.webui_dir, env=vite_env,
            stdout=vite_log, stderr=subprocess.STDOUT,
        )
        webui_origin = wait_vite_server(vite_log_path, 90.0)
        wait_http_ok(webui_origin + "/app/", 60.0)
        shots_dir = project_path.parent / "card_button_shots"
        b1_edits = {"start_value": "0.20", "end_value": "0.80", "curve": "-0.25"}
        b1 = run_browser_round(webui_origin, args.agent_http, "0 到 24 秒，让这条轨的存在感慢慢强起来", b1_edits, shots_dir)
        points1 = kernel_points(args.kernel_zmq, plugin1, "volume")
        require(len(points1) == 2, f"B1 kernel points = {len(points1)}, want 2")
        require(approx(points1[0].get("t"), 0.0) and approx(points1[0].get("v"), 0.20),
                f"B1 point#0 = {points1[0]!r}, want t=0 v=0.20 (EDITED start)")
        require(approx(points1[1].get("t"), 24.0) and approx(points1[1].get("v"), 0.80),
                f"B1 point#1 = {points1[1]!r}, want t=24 v=0.80 (EDITED end)")
        curve_c = points1[0].get("c")
        if curve_c is not None:
            require(approx(curve_c, -0.25), f"B1 point#0 c = {curve_c!r}, want -0.25 (EDITED curve)")
        record("leg_b1_browser_button_click", {
            "dom": b1,
            "kernel_points": points1,
            "shots": str(shots_dir),
        })

        # ---- LEG R2: phrasing-confirmation control round (coexistence) --
        reply = zmq_command(args.kernel_zmq, {"cmd": "add_audio_track"})
        track2 = str(reply.get("track_id") or reply.get("id") or "").strip()
        require(track2, f"add_audio_track (2) returned no track_id: {reply}")
        state = zmq_command(args.kernel_zmq, {"cmd": "get_project_state", "command_timeout_ms": 60000})
        plugin2 = find_track_volume_plugin(state, track2)
        require(bool(plugin2), f"no built-in Volume & Pan plugin on track {track2}")

        conv_control = "automation_card_button_control"
        control_context = {"agent_mode": "chat", "selected_track_id": track2}
        proposed = chat(args.agent_http, conv_control, "45 到 60 秒，让这条轨的存在感慢慢强起来", control_context)
        require(proposed.get("workflow") == "section_automation_choreo",
                f"R2 proposal workflow = {proposed.get('workflow')!r}: {json.dumps(proposed, ensure_ascii=False)[:600]}")
        require(str(proposed.get("goal_status", "")).lower() in {"waiting_confirmation", "waiting_confirm"},
                f"R2 proposal goal_status = {proposed.get('goal_status')!r}")
        interactions = proposed.get("interaction_requests") or []
        card = next((row for row in interactions if isinstance(row, dict) and row.get("kind") == "automation_choreo_confirmation"), None)
        require(card is not None, "R2: no automation_choreo_confirmation card in interaction_requests")
        declared = assert_declarative_contract(card, "R2")
        require((proposed.get("workflow_data") or {}).get("mutation_performed") is False, "R2: card must carry mutation_performed=false")
        require(len(kernel_points(args.kernel_zmq, plugin2, "volume")) == 0, "R2: zero-write before confirmation")

        confirmed = chat(args.agent_http, conv_control, "确认写入", control_context)
        require(confirmed.get("stop_reason") == "automation_choreo_settled",
                f"R2 confirm stop_reason = {confirmed.get('stop_reason')!r}: {json.dumps(confirmed, ensure_ascii=False)[:1200]}")
        settle = confirmed.get("workflow_data") or {}
        require(settle.get("mutation_performed") is True and settle.get("written") == 2,
                f"R2 settle written = {settle.get('written')!r}")
        points2 = kernel_points(args.kernel_zmq, plugin2, "volume")
        require(len(points2) == 2, f"R2 kernel points = {len(points2)}, want 2")
        require(approx(points2[0].get("t"), 45.0) and approx(points2[0].get("v"), 0.30),
                f"R2 point#0 = {points2[0]!r}, want t=45 v=0.30 (default fade-in)")
        require(approx(points2[1].get("t"), 60.0) and approx(points2[1].get("v"), 0.60),
                f"R2 point#1 = {points2[1]!r}, want t=60 v=0.60")
        record("leg_r2_phrasing_control_round", {
            "card_schema": declared.get("schema"),
            "written": settle.get("written"),
            "kernel_points": points2,
        })

        # ---- LEG R3: cancel via responder (negative) --------------------
        conv_cancel = "automation_card_button_cancel"
        cancel_context = {"agent_mode": "chat", "selected_track_id": track2}
        proposed3 = chat(args.agent_http, conv_cancel, "10 到 20 秒，让这条轨的存在感慢慢强起来", cancel_context)
        interactions3 = proposed3.get("interaction_requests") or []
        card3 = next((row for row in interactions3 if isinstance(row, dict) and row.get("kind") == "automation_choreo_confirmation"), None)
        require(card3 is not None, "R3: no automation_choreo_confirmation card in interaction_requests")
        require(proposed3.get("workflow") == "section_automation_choreo", f"R3 proposal workflow = {proposed3.get('workflow')!r}")
        interaction_id = str(card3.get("id") or "")
        require(bool(interaction_id), "R3: card has no interaction id")
        cancelled = respond(args.agent_http, interaction_id, "cancel", {})
        require("取消" in str(cancelled.get("reply") or cancelled.get("message") or ""),
                f"R3 cancel reply = {cancelled.get('reply')!r}")
        require(str(cancelled.get("goal_status", "")).lower() in {"cancelled", "canceled"},
                f"R3 cancel goal_status = {cancelled.get('goal_status')!r}")
        points_after = kernel_points(args.kernel_zmq, plugin2, "volume")
        require(len(points_after) == 2, f"R3 curve points = {len(points_after)}, want unchanged 2 (cancel = zero write)")
        record("leg_r3_cancel_zero_write", {
            "interaction_id": interaction_id,
            "goal_status": cancelled.get("goal_status"),
            "curve_points_after_cancel": len(points_after),
        })

        summary["status"] = "pass"
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
    except Exception as exc:  # noqa: BLE001 - smoke failure surfaces in summary.
        summary["status"] = "fail"
        summary["error"] = str(exc)
        summary["finished_at"] = datetime.now(timezone.utc).isoformat()
        print(f"[FAIL] {exc}", file=sys.stderr)
        raise
    finally:
        if vite_process is not None:
            # Popen.terminate kills only the npm.cmd shim on Windows; the node
            # child keeps port 5178. Take the whole process tree down instead.
            subprocess.run(
                ["taskkill", "/PID", str(vite_process.pid), "/T", "/F"],
                stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=False,
            )
            try:
                vite_process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                vite_process.kill()
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"[summary] {output_path} status={summary.get('status')}")

    sys.exit(0 if summary.get("status") == "pass" else 1)


if __name__ == "__main__":
    main()
