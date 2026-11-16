# -*- coding: utf-8 -*-
"""KERNEL-SAVE-FORENSICS-1 D1/D2 race probe.

Hypothesis: D1/D2 were a race between the score panel's 250ms /agent/state
polling (concurrent get_project_state through the agent) and save_project's
shadow re-initialization. This probe hammers get_project_state in a background
thread (every 150ms, faster than the panel) while the main thread runs the
attempt3 sequence (add_track -> insert_midi_clip -> add_midi_notes ->
save_project{} -> immediate readback) N times on fresh boot state each round
is impossible without kernel restarts, so rounds run sequentially on one boot
(new track+clip each round; the assertion is about THAT round's ids).

Deterministic, zero LLM.
"""
import json
import os
import sys
import threading
import time
import urllib.request

AGENT_HTTP = os.environ.get("AGENT_HTTP", "http://127.0.0.1:7878")
ROUNDS = int(os.environ.get("ROUNDS", "10"))

NOTES = [
    {"start_beat": 0.0, "length_beats": 1.0, "pitch": 60, "velocity": 90},
    {"start_beat": 1.0, "length_beats": 1.0, "pitch": 64, "velocity": 80},
]


def invoke(tool, args, confirmed=False, timeout=120):
    body = json.dumps({"tool": tool, "args": args, "confirmed": confirmed,
                       "source": "kernel_save_forensics_race_probe"}).encode()
    req = urllib.request.Request(AGENT_HTTP + "/agent/invoke", data=body,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode())


def ui_state(timeout=30):
    with urllib.request.urlopen(AGENT_HTTP + "/agent/ui/state?name=ui_context", timeout=timeout) as resp:
        return json.loads(resp.read().decode())


def track_ids(result):
    return {(t.get("track_id") or t.get("id")): [c.get("clip_id") or c.get("id") for c in (t.get("clips") or [])]
            for t in (result.get("tracks") or [])}


def main():
    stop = threading.Event()
    poll_stats = {"ui_state": 0, "ui_state_err": 0, "gps": 0, "gps_err": 0,
                  "saw_empty_tracks": 0, "saw_missing_clip": []}

    def poller():
        while not stop.is_set():
            try:
                ui_state()
                poll_stats["ui_state"] += 1
            except Exception:
                poll_stats["ui_state_err"] += 1
            try:
                r = invoke("get_project_state", {}, timeout=30)
                res = r.get("result") or {}
                rows = track_ids(res)
                if not rows:
                    poll_stats["saw_empty_tracks"] += 1
                for tid, clips in rows.items():
                    if any(c in (None, "") for c in clips):
                        poll_stats["saw_missing_clip"].append({"track": tid, "clips": clips})
                poll_stats["gps"] += 1
            except Exception:
                poll_stats["gps_err"] += 1
            time.sleep(0.15)

    t = threading.Thread(target=poller, daemon=True)
    t.start()

    report = {"schema": "kernel_save_forensics_race.v1", "rounds": []}
    failures = 0
    for i in range(ROUNDS):
        row = {"round": i}
        try:
            r = invoke("add_track", {}, True)
            tid = str((r.get("result") or {}).get("track_id", ""))
            r2 = invoke("insert_midi_clip", {"track_id": tid, "time_unit": "beats",
                                             "start": 0.0, "length": 4.0}, True)
            cid = str((r2.get("result") or {}).get("clip_id",
                        (r2.get("result") or {}).get("new_clip_id", "")))
            invoke("add_midi_notes", {"track_id": tid, "clip_id": cid,
                                      "time_unit": "beats", "notes": NOTES}, True)
            row["track"] = tid
            row["clip"] = cid

            pre = invoke("get_project_state", {}).get("result") or {}
            row["pre_ok"] = cid in track_ids(pre).get(tid, [])
            if not row["pre_ok"]:
                row["pre_face"] = track_ids(pre)

            sv = invoke("save_project", {}, True)
            row["save_ok"] = sv.get("status") == "ok"

            post = invoke("get_project_state", {}).get("result") or {}
            post_rows = track_ids(post)
            row["post_track_ok"] = tid in post_rows
            row["post_clip_ok"] = cid in post_rows.get(tid, [])
            if not (row["post_track_ok"] and row["post_clip_ok"]):
                row["post_face"] = post_rows

            g = invoke("get_midi_clip_notes", {"clip_id": cid})
            row["notes_ok"] = g.get("status") == "ok"
            if not row["notes_ok"]:
                row["notes_err"] = str(g.get("error"))[:300]
        except Exception as exc:
            row["exception"] = repr(exc)[:300]
        ok = all(row.get(k) for k in ("pre_ok", "save_ok", "post_track_ok",
                                       "post_clip_ok", "notes_ok")) and "exception" not in row
        row["ok"] = ok
        if not ok:
            failures += 1
        report["rounds"].append(row)
        print("round %d: %s (track=%s clip=%s)" % (i, "OK" if ok else "FAIL",
                                                   row.get("track"), row.get("clip")))

    stop.set()
    t.join(timeout=5)
    report["poll_stats"] = poll_stats
    report["failures"] = failures
    json.dump(report, open("race_probe_report.json", "w", encoding="utf-8"),
              ensure_ascii=False, indent=1)
    print("POLL: ui_state=%d(err %d) gps=%d(err %d) empty_tracks=%d null_clip_rows=%d"
          % (poll_stats["ui_state"], poll_stats["ui_state_err"], poll_stats["gps"],
             poll_stats["gps_err"], poll_stats["saw_empty_tracks"],
             len(poll_stats["saw_missing_clip"])))
    print("FAILURES: %d/%d" % (failures, ROUNDS))
    return 0


if __name__ == "__main__":
    sys.exit(main())
