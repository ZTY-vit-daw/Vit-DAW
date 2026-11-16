# -*- coding: utf-8 -*-
"""KERNEL-SAVE-FORENSICS-1 agent-face D1/D2 driver.

Mirrors SCORE-WORKBENCH-1 attempt3's exact invoke sequence on an XML-boot
project through the real agent /agent/invoke face (kernel + agent only):

  add_track -> insert_midi_clip -> add_midi_notes
  -> get_project_state (pre-save disclosure)
  -> save_project {}            [EMPTY payload, exactly like the leg]
  -> get_project_state (post-save disclosure)   [D1: track dropped?]
  -> get_midi_clip_notes                        [the attempt3 failure point]
  -> (recovery probe) open_project <saved path> -> get_project_state
  -> get_midi_clip_notes                        [D2: restored after reopen?]

Deterministic, zero LLM (tool invokes only).
"""
import json
import os
import sys
import time
import urllib.request

AGENT_HTTP = os.environ.get("AGENT_HTTP", "http://127.0.0.1:7878")

FIXTURE_NOTES = [
    {"start_beat": 0.0, "length_beats": 1.0, "pitch": 60, "velocity": 90},
    {"start_beat": 1.0, "length_beats": 0.5, "pitch": 62, "velocity": 80},
    {"start_beat": 1.5, "length_beats": 0.5, "pitch": 64, "velocity": 80},
    {"start_beat": 2.0, "length_beats": 2.0, "pitch": 67, "velocity": 85},
]


def invoke(tool, args, confirmed=False):
    body = json.dumps({"tool": tool, "args": args, "confirmed": confirmed,
                       "source": "kernel_save_forensics_agent_face"}).encode()
    req = urllib.request.Request(AGENT_HTTP + "/agent/invoke", data=body,
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=120) as resp:
        return json.loads(resp.read().decode())


def state_face(reply_result):
    """Reply face of the agent get_project_state invoke."""
    out = {"keys": sorted(reply_result.keys())[:40]}
    tracks = reply_result.get("tracks") or []
    rows = []
    for t in tracks:
        rows.append({
            "track_id": t.get("track_id") or t.get("id"),
            "name": t.get("name") or t.get("track_name"),
            "clip_ids": [c.get("clip_id") or c.get("id") for c in (t.get("clips") or [])],
        })
    out["track_rows"] = rows
    out["audio_track_count"] = reply_result.get("audio_track_count") if "audio_track_count" in reply_result else len(rows)
    for k in ("project_path", "current_project_path", "project_uuid", "project_epoch"):
        if k in reply_result:
            out[k] = reply_result[k]
    proj = reply_result.get("project")
    if isinstance(proj, dict):
        out["project.face"] = {k: proj.get(k) for k in ("project_path", "project_uuid", "project_id", "clip_count", "track_count") if k in proj}
    return out


def main():
    report = {"schema": "kernel_save_forensics_agent_face.v1",
              "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
              "steps": [], "asserts": []}

    def step(name, resp):
        report["steps"].append({"name": name, "status": resp.get("status"),
                                "error": resp.get("error")})
        return resp.get("result") or {}

    def check(name, ok, detail=""):
        report["asserts"].append({"name": name, "pass": bool(ok), "detail": detail})
        print(("PASS " if ok else "FAIL ") + name + (" :: " + detail if detail else ""))

    r = invoke("add_track", {}, True)
    res = step("add_track", r)
    track_id = str(res.get("track_id", ""))
    check("A-add-track-ok", r.get("status") == "ok" and track_id != "", json.dumps(r)[:200])

    r = invoke("insert_midi_clip", {"track_id": track_id, "time_unit": "beats", "start": 0.0, "length": 4.0}, True)
    res = step("insert_midi_clip", r)
    clip_id = str(res.get("clip_id", res.get("new_clip_id", "")))
    check("A-insert-midi-clip-ok", r.get("status") == "ok" and clip_id != "", json.dumps(r)[:200])

    r = invoke("add_midi_notes", {"track_id": track_id, "clip_id": clip_id,
                                  "time_unit": "beats", "notes": FIXTURE_NOTES}, True)
    step("add_midi_notes", r)
    check("A-add-midi-notes-ok", r.get("status") == "ok", json.dumps(r)[:200])

    r = invoke("get_project_state", {})
    pre = state_face(step("get_project_state.pre", r))
    report["pre_save_face"] = pre
    pre_row = next((x for x in pre["track_rows"] if x["track_id"] == track_id), None)
    check("A-pre-save-track-disclosed", pre_row is not None, "track=" + track_id)
    check("A-pre-save-clip-disclosed",
          pre_row is not None and clip_id in (pre_row["clip_ids"]),
          "clip=" + clip_id + " rows=" + json.dumps(pre["track_rows"]))

    # ---- the exact attempt3 call: save_project with EMPTY args -------------
    r = invoke("save_project", {}, True)
    res = step("save_project", r)
    check("A-save-ok", r.get("status") == "ok", json.dumps(r)[:400])
    saved_path = str(res.get("project_path", res.get("current_project_path", "")))
    report["saved_path"] = saved_path

    r = invoke("get_project_state", {})
    post = state_face(step("get_project_state.post", r))
    report["post_save_face"] = post
    post_row = next((x for x in post["track_rows"] if x["track_id"] == track_id), None)
    check("A-D1-post-save-track-disclosed", post_row is not None,
          "track=" + track_id + (" (D1 REPRO: dropped)" if post_row is None else ""))
    check("A-D1-post-save-clip-disclosed",
          post_row is not None and clip_id in post_row["clip_ids"],
          "clip=" + clip_id + " rows=" + json.dumps(post["track_rows"]))

    r = invoke("get_midi_clip_notes", {"clip_id": clip_id})
    res = step("get_midi_clip_notes.post", r)
    check("A-D1-post-save-notes-readable", r.get("status") == "ok",
          json.dumps(r)[:300])

    # ---- recovery: reopen ---------------------------------------------------
    if saved_path:
        r = invoke("open_project", {"file_path": saved_path}, True)
        step("open_project", r)
        check("A-reopen-ok", r.get("status") == "ok", json.dumps(r)[:300])

        r = invoke("get_project_state", {})
        ro = state_face(step("get_project_state.reopen", r))
        report["reopen_face"] = ro
        ro_row = next((x for x in ro["track_rows"] if x["track_id"] == track_id), None)
        check("A-reopen-track-disclosed", ro_row is not None, "track=" + track_id)
        check("A-reopen-clip-disclosed",
              ro_row is not None and clip_id in ro_row["clip_ids"],
              "clip=" + clip_id)

        r = invoke("get_midi_clip_notes", {"clip_id": clip_id})
        res = step("get_midi_clip_notes.reopen", r)
        check("A-reopen-notes-readable", r.get("status") == "ok", json.dumps(r)[:300])
    else:
        check("A-reopen-ok", False, "saved_path empty; cannot probe reopen")

    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    json.dump(report, open("agent_face_report.json", "w", encoding="utf-8"),
              ensure_ascii=False, indent=1)
    failed = [a for a in report["asserts"] if not a["pass"]]
    print("ASSERTS: %d pass, %d fail" % (len(report["asserts"]) - len(failed), len(failed)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
