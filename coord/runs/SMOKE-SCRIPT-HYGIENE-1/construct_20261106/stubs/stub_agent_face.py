# stub_agent_face.py — SMOKE-SCRIPT-HYGIENE-1 (260) item (4) constructed stub.
#
# Stateful zero-stack stand-in for the agent HTTP face + a bare kernel port
# listener, so the D1 launcher's post-runner phase gates can be exercised
# without VitApp/Godot/agent processes and without any LLM:
#   - listens on --raw-port (default 5555) as a bare TCP accept loop (the
#     launcher only checks LISTENING state, never speaks ZMQ here);
#   - GET  /agent/state  -> echoes the last project.open'ed path as
#     {"project": {"project_path": ...}} (the launcher's recursive walker
#     picks it up);
#   - POST /agent/invoke -> project.open (records file_path),
#     loudness_watch_configure (ok + stub digest), mix_request_observation
#     (ok + EMPTY result: no loudness_watch disclosure -> the 1-mutation
#     branch's LW gate 1 fails honestly, as designed),
#     mix_queue_view (ok + empty view.nodes);
#   - POST /agent/chat   -> a well-shaped non-settling response so every
#     settle-gate property access is defined (Set-StrictMode) while the
#     gates fail honestly on values.
# --godot-root exists so the launcher's Godot-process check (command line
# contains the project root) finds THIS process instead of autostarting one.
import argparse
import json
import socket
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

STATE = {"opened": None}


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _send(self, obj):
        body = json.dumps(obj).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path.startswith("/agent/state"):
            payload = {"project": {"project_path": STATE["opened"] or ""}}
            self._send(payload)
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
            self._send({
                "stop_reason": "stub_no_settle",
                "needs_confirmation": False,
                "interaction_requests": [],
                "workflow_data": {
                    "observation": {"loudness_peak_status": "absent_stub"},
                    "d1_settle": {"checked": False, "config_basis": "", "verdict": "stub"},
                    "mutation_performed": "False",
                },
            })
            return
        tool = str(req.get("tool", ""))
        if tool == "project.open":
            args = req.get("args") or {}
            STATE["opened"] = args.get("file_path")
            self._send({"status": "ok", "result": {"status": "ok"}})
        elif tool == "loudness_watch_configure":
            self._send({"status": "ok", "result": {"config_digest": "stubdigest260"}})
        elif tool == "mix_request_observation":
            self._send({"status": "ok", "result": {}})
        elif tool == "mix_queue_view":
            self._send({"status": "ok", "result": {"view": {"nodes": []}}})
        else:
            self._send({"status": "ok", "result": {}})

    def log_message(self, *args):
        pass


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
    args = ap.parse_args()
    threading.Thread(target=raw_listener, args=(args.raw_port,), daemon=True).start()
    ThreadingHTTPServer(("127.0.0.1", args.http_port), Handler).serve_forever()


if __name__ == "__main__":
    main()
