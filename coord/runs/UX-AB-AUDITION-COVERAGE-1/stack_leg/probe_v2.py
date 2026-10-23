#!/usr/bin/env python3
"""Card 177 v2 judgment probes against the live stack (capture-defect fix).

The driver's v1 probes read the topo identity off the re-seat event row (the
kernel-serialized session without the agent-side turn/round fields) and hit
the required-fields refusal. This script re-runs the same two probes with the
identity from the MOUNT event (turn_id/round_id/project_revision present) and
writes the v2 artifacts next to the v1 ones (v1 kept as the defect record).
"""
from __future__ import annotations

import json
import sys
import urllib.error
import urllib.request
from pathlib import Path

BASE = "http://127.0.0.1:7878"


def post_judgment(payload: dict) -> dict:
    request = urllib.request.Request(
        BASE + "/agent/audition/judgment",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return {"http_status": response.status,
                    "body": json.loads(response.read().decode("utf-8", errors="replace"))}
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode("utf-8", errors="replace")
        try:
            body = json.loads(raw)
        except Exception:
            body = {"raw": raw}
        return {"http_status": exc.code, "body": body}


def main() -> int:
    round_dir = Path(sys.argv[1])
    conversation_id = sys.argv[2]
    events = json.loads((round_dir / "events.json").read_text(encoding="utf-8"))
    mount = None
    for event in events.get("events") or []:
        if not isinstance(event, dict) or event.get("type") != "audition.ready":
            continue
        session = (event.get("payload") or {}).get("session") or {}
        sid = str(session.get("session_id") or "")
        if sid.startswith("audition:dynamic:topo_") and str(session.get("turn_id") or "").startswith("dynamic_turn:topo_"):
            mount = session
            break
    if mount is None:
        print("no topo mount row with turn_id in events.json")
        return 2
    identity = {
        "conversation_id": conversation_id,
        "turn_id": str(mount.get("turn_id") or ""),
        "round_id": str(mount.get("round_id") or ""),
        "audition_session_id": str(mount.get("session_id") or ""),
        "project_revision": str(mount.get("project_revision") or ""),
    }
    print("mount identity:", json.dumps(identity, ensure_ascii=False))

    wrong_turn = dict(identity)
    wrong_turn["turn_id"] = identity["turn_id"] + "__wrong"
    wrong_turn["heard_difference"] = "yes"
    wrong_turn["preference"] = "b"
    probe1 = post_judgment(wrong_turn)
    (round_dir / "judgment_409_identity_v2.json").write_text(json.dumps(
        {"request": wrong_turn, "http_status": probe1["http_status"], "body": probe1["body"]},
        ensure_ascii=False, indent=1), encoding="utf-8")
    print("wrong-turn:", probe1["http_status"])

    correct = dict(identity)
    correct["heard_difference"] = "yes"
    correct["preference"] = "b"
    correct["free_text"] = "判定探针v2：topo 通道选 B，保留这次装载"
    probe2 = post_judgment(correct)
    (round_dir / "judgment_topo_ok_v2.json").write_text(json.dumps(
        {"request": correct, "http_status": probe2["http_status"], "body": probe2["body"]},
        ensure_ascii=False, indent=1), encoding="utf-8")
    print("correct:", probe2["http_status"])
    return 0


if __name__ == "__main__":
    sys.exit(main())
