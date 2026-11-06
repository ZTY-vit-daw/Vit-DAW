# stub_agent_face_268.py — D1GATE-SCRIPT-HYGIENE-1 (268) constructed stub.
#
# Stateful zero-stack stand-in for the agent HTTP face + a bare kernel port
# listener, so the D1 launcher's D1-SETTLE gate legs can be exercised without
# VitApp/Godot/agent processes and without any LLM. Scenario-selected via the
# environment (the launcher's argument surface has no scenario selector):
#
#   VIT268_STUB_SCENARIO = settle_pass | breach_response | breach_durable
#
#   settle_pass     /agent/chat settle conv -> stop_reason=d1_loudness_settled,
#                   d1_settle verdict "pass" (non-breach), needs_confirmation
#                   present as false, NO interaction_requests key (the real
#                   ChatResponse omitempty form), no durable park;
#   breach_response breach verdict; the response top face carries the card
#                   (needs_confirmation=true + interaction_requests
#                   kind=mix_tick_confirmation) — the direct canary-path form;
#   breach_durable  breach verdict; the response face is SILENT (no
#                   needs_confirmation key, no interaction_requests key — the
#                   262 L3 projection-split form), while GET
#                   /agent/runtime/status holds the answerable park in
#                   continuations[].pending_interaction.
#
#   The cancel leg ("取消") answers stop_reason=mix_tick_rejected in every
#   scenario once the settle turn happened.
#   POST /agent/invoke: mix.report -> result.mix_workflow_queue.nodes (M1-5
#   three-view, the 265-fixed D1/D2 gate D read path); mix_queue_view (the
#   254-era guessed name) returns the unknown-tool error envelope the real
#   agent produced (265 run3 lesson), and project.open records the path.
#   GET  /agent/state -> echoes the last project.open'ed path.
#
# --godot-root exists so the launcher's Godot-process check (command line
# contains the project root) finds THIS process instead of autostarting one.
import argparse
import json
import os
import socket
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

STATE = {"opened": None, "settle_conv": None, "cancelled": False}

SETTLE_OK_PAYLOAD = {
    "stop_reason": "d1_loudness_settled",
    "goal_status": "completed",
    # The real ChatResponse serializes needs_confirmation unconditionally
    # (no omitempty) and interaction_requests only when non-empty.
    "needs_confirmation": False,
    "workflow_data": {
        "observation": {"loudness_peak_status": "present"},
        "d1_settle": {"checked": True, "config_basis": "内置流媒体目标（ebu_r128，watch 未配置，一次性判断不写配置面）", "verdict": "pass"},
        "mutation_performed": "False",
    },
}


def settle_breach_payload(with_response_face):
    payload = json.loads(json.dumps(SETTLE_OK_PAYLOAD))
    payload["workflow_data"]["d1_settle"]["verdict"] = "breach"
    payload["workflow_data"]["pending_action"] = {"kind": "track_gain", "delta_db": -1.5}
    if with_response_face:
        payload["needs_confirmation"] = True
        payload["interaction_requests"] = [{
            "id": "interaction_stub_268",
            "kind": "mix_tick_confirmation",
            "type": "mix_tick_confirmation",
            "source": "vit_agent",
            "workflow": "mix_tick",
            "stage": "pending_confirmation",
        }]
    # with_response_face=False: the keys stay absent entirely (the L3 split
    # face; json serialization of the real transport can drop them).
    return payload


def runtime_status_payload():
    rows = []
    breach_scene = ARG.scene.startswith("breach")
    if breach_scene and STATE["settle_conv"] and not STATE["cancelled"]:
        rows.append({
            "conversation_id": STATE["settle_conv"],
            "goal_status": "completed",
            "pending_interaction": {
                "kind": "mix_tick_confirmation",
                "status": "pending",
                "conversation_id": STATE["settle_conv"],
            },
        })
    return {"goal": {"status": "completed"}, "continuations": rows}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _send(self, obj):
        body = json.dumps(obj, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.startswith("/agent/runtime/status"):
            self._send(runtime_status_payload())
        elif self.path.startswith("/agent/state"):
            self._send({"project": {"project_path": STATE["opened"] or ""}})
        else:
            self._send({"status": "ok"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0") or 0)
        raw = self.rfile.read(length) if length else b"{}"
        try:
            req = json.loads(raw or b"{}")
        except json.JSONDecodeError:
            req = {}
        if self.path.startswith("/agent/chat"):
            conversation = str(req.get("conversation_id", ""))
            message = str(req.get("message", ""))
            scenario = ARG.scene
            if message == "取消" and STATE["settle_conv"]:
                STATE["cancelled"] = True
                self._send({"stop_reason": "mix_tick_rejected", "goal_status": "completed", "workflow_data": {}})
                return
            if conversation.startswith("conv_d1_settle_"):
                STATE["settle_conv"] = conversation
                STATE["cancelled"] = False
                if scenario == "breach_response":
                    self._send(settle_breach_payload(True))
                elif scenario == "breach_durable":
                    self._send(settle_breach_payload(False))
                else:
                    self._send(SETTLE_OK_PAYLOAD)
                return
            self._send({"stop_reason": "stub_idle", "goal_status": "completed", "workflow_data": {}})
            return
        tool = str(req.get("tool", ""))
        if tool == "project.open":
            args = req.get("args") or {}
            STATE["opened"] = args.get("file_path")
            self._send({"status": "ok", "result": {"status": "ok"}})
        elif tool == "mix.report":
            branch = os.environ.get("VIT268_STUB_BRANCH", "applied").strip().lower()
            if branch == "zero_mutation":
                node_state = "not_started"  # no applied action -> the D1 node never lit
            elif ARG.scene.startswith("breach"):
                node_state = "needs_confirmation"  # a parked card holds the D1 node
            else:
                node_state = "in_progress"  # the check ran, nothing parked
            self._send({"status": "ok", "result": {"mix_workflow_queue": {"nodes": [
                {"node_id": "D1", "state": node_state},
                {"node_id": "D2", "state": "not_started"},
            ]}}})
        elif tool == "mix_queue_view":
            # The real agent has no standalone mix_queue_view invoke tool
            # (265 run3 lesson): unknown tool error envelope, no result key.
            self._send({"status": "error", "error": "unknown tool: mix_queue_view"})
        elif tool == "loudness_watch_configure":
            self._send({"status": "ok", "result": {"config_digest": "stubdigest268"}})
        elif tool == "mix_request_observation":
            self._send({"status": "ok", "result": {}})
        else:
            self._send({"status": "ok", "result": {}})

    def log_message(self, *args):
        pass


class Args:
    scene = "settle_pass"
    http_port = 7878
    raw_port = 5555
    godot_root = ""


ARG = Args()


def raw_listener(port):
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("127.0.0.1", port))
    srv.listen(16)
    while True:
        try:
            conn, _ = srv.accept()
            threading.Thread(target=lambda c: c.close(), args=(conn,), daemon=True).start()
        except OSError:
            return


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--http-port", type=int, default=7878)
    ap.add_argument("--raw-port", type=int, default=5555)
    ap.add_argument("--godot-root", default="")
    ap.add_argument("--scene", default="")
    known, _unknown = ap.parse_known_args()
    ARG.http_port = known.http_port
    ARG.raw_port = known.raw_port
    ARG.godot_root = known.godot_root
    ARG.scene = (known.scene or "").strip() or os.environ.get("VIT268_STUB_SCENARIO", "settle_pass").strip()
    threading.Thread(target=raw_listener, args=(ARG.raw_port,), daemon=True).start()
    ThreadingHTTPServer(("127.0.0.1", ARG.http_port), Handler).serve_forever()


if __name__ == "__main__":
    main()
