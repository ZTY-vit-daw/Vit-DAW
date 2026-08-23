"""Ephemeral GET-only fixture for G local smoke and WebUI projection checks."""

import argparse
import json
from http.server import BaseHTTPRequestHandler, HTTPServer


TRAJECTORY = {
    "schema_version": "vit.task_runtime_trajectory.v1",
    "task": {
        "task_id": "task-g-readonly",
        "goal_id": "goal-g-readonly",
        "run_id": "run-g-readonly",
        "conversation_id": "conversation-g-readonly",
        "original_intent": "检查一下当前工程有什么问题？",
        "status": "active",
    },
    "run": {
        "run_id": "run-g-readonly",
        "current_slice_id": "slice-g-2",
        "current_turn_id": "turn-g-2",
        "slices": [
            {"slice_id": "slice-g-1", "sequence": 1, "status": "waiting_continuation", "max_turns": 1},
            {"slice_id": "slice-g-2", "sequence": 2, "status": "running", "max_turns": 1},
        ],
    },
    "continuation": {"continuation_id": "cont-g-readonly", "status": "running"},
    "semantic": {"state": "observation_in_progress", "summary": "正在观察工程结构"},
}


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):  # noqa: N802
        path = self.path.split("?", 1)[0]
        if path == "/health":
            body = {"status": "ok", "service": "G readonly fixture"}
        elif path == "/agent/runtime/status":
            body = {"status": "ok", "task_trajectory": TRAJECTORY}
        elif path == "/agent/state":
            body = {"status": "ok", "project": {"track_count": 0, "duration_seconds": 0}}
        elif path == "/agent/events":
            body = {"status": "ok", "events": [{"type": "trajectory.observation.recorded", "payload": {"summary": "正在观察工程结构"}}]}
        elif path == "/agent/ui/state":
            body = {"status": "ok", "agent": {"status": "ready"}}
        else:
            self.send_response(404)
            self.end_headers()
            return
        raw = json.dumps(body, ensure_ascii=False).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def log_message(self, *_args):
        return


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=7879)
    args = parser.parse_args()
    HTTPServer((args.host, args.port), Handler).serve_forever()


if __name__ == "__main__":
    main()
