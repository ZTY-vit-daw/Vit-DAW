#!/usr/bin/env python3
"""Smoke-test V0.6 IPC commands against VitHeadlessServer (ZMQ REQ on 5555)."""

from __future__ import annotations

import argparse
import json
import sys

try:
    import zmq
except ImportError:
    print("pip install pyzmq", file=sys.stderr)
    sys.exit(1)


def req_cmd(url: str, obj: dict, timeout_ms: int = 60_000) -> str:
    ctx = zmq.Context.instance()
    s = ctx.socket(zmq.REQ)
    s.connect(url)
    s.setsockopt(zmq.RCVTIMEO, timeout_ms)
    s.setsockopt(zmq.SNDTIMEO, timeout_ms)
    payload = json.dumps(obj, ensure_ascii=False)
    s.send_string(payload)
    return s.recv_string()


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--url", default="tcp://127.0.0.1:5555")
    p.add_argument("--track-id", default="", help="for arm_track / freeze tests")
    args = p.parse_args()

    print(req_cmd(args.url, {"cmd": "ping"}))

    st = json.loads(req_cmd(args.url, {"cmd": "list_tracks"}))
    print("list_tracks status", st.get("status"))
    tracks = st.get("tracks") or []
    tid = args.track_id.strip()
    if not tid and tracks:
        tid = str((tracks[0] or {}).get("id") or "").strip()
    print("using track_id", tid or "(none)")

    if tid:
        print(
            req_cmd(
                args.url,
                {"cmd": "arm_track", "track_id": tid, "is_armed": True},
            )
        )
        print(
            req_cmd(
                args.url,
                {"cmd": "arm_track", "track_id": tid, "is_armed": False},
            )
        )


if __name__ == "__main__":
    main()
