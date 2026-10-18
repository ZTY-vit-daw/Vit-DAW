#!/usr/bin/env python3
"""Send one legacy command to the live kernel ZMQ REQ face (read or lifecycle).

Used by FE-START-EMPTY-DRAFT-1 real-stack probe to emulate the frontend start
page path faithfully: the Godot start page first asks the kernel for its current
project state, calls kernel new_project, and THEN notifies the agent through
/agent/invoke version.project_new with the uuid from the kernel reply. Keeping
the pre-notification reads on the KERNEL face (not the agent HTTP face) is
essential: an /agent/state read before the notification would materialize history
inside the implicit default draft and destroy the pristine reuse precondition.

Print a single-line JSON object: the kernel reply. Exit 0 on a JSON reply,
2 on transport error, 3 when reply status is not ok.
"""
import argparse
import json
import sys

import zmq


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--url", default="tcp://127.0.0.1:5555")
    ap.add_argument("--cmd", default="new_project")
    ap.add_argument("--args-json", default="")
    ap.add_argument("--timeout-ms", type=int, default=20000)
    args = ap.parse_args()
    command = {"cmd": args.cmd, "command_timeout_ms": args.timeout_ms}
    if args.args_json.strip():
        extra = json.loads(args.args_json)
        if isinstance(extra, dict):
            command.update(extra)
    ctx = zmq.Context()
    sock = ctx.socket(zmq.REQ)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt(zmq.RCVTIMEO, args.timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, args.timeout_ms)
    if hasattr(zmq, "REQ_RELAXED"):
        sock.setsockopt(zmq.REQ_RELAXED, 1)
    if hasattr(zmq, "REQ_CORRELATE"):
        sock.setsockopt(zmq.REQ_CORRELATE, 1)
    sock.connect(args.url)
    try:
        sock.send_string(json.dumps(command))
        raw = sock.recv()
    except zmq.ZMQError as exc:
        print(json.dumps({"status": "error", "error": "zmq: %s" % exc}))
        return 2
    finally:
        sock.close(0)
        ctx.term()
    try:
        reply = json.loads(raw.decode("utf-8", "replace"))
    except ValueError:
        print(json.dumps({"status": "error", "error": "invalid JSON reply"}))
        return 2
    print(json.dumps(reply, ensure_ascii=False))
    return 0 if str(reply.get("status", "")).lower() in ("ok", "success") else 3


if __name__ == "__main__":
    sys.exit(main())
