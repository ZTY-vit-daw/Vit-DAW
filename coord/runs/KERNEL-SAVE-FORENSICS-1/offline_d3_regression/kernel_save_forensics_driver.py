# -*- coding: utf-8 -*-
"""KERNEL-SAVE-FORENSICS-1 offline D1/D2/D3 forensic driver.

Kernel-only ZMQ driver (no agent / no Godot / no webui). Deterministic,
zero LLM. Scenarios:

  S1  D1 repro (XML-boot):   add_track -> insert_midi_clip -> add_midi_notes
                             -> get_project_state (pre-save disclosure)
                             -> save_project {file_path=run out1.vit,
                                encryption=app_bound_aes}   [agent-equivalent
                                enrichment for an unsaved XML-boot project]
                             -> get_project_state (post-save disclosure)
                             -> get_midi_clip_notes
  S2  reopen control:        open_project out1.vit -> get_project_state
                             -> get_midi_clip_notes (does reopen restore?)
  S3  D2 repro (.vit-boot):  add_track -> insert_midi_clip -> add_midi_notes
                             -> save_project {} (current path already .vit)
                             -> get_project_state xN (clip_id disclosure)
                             -> open_project -> get_project_state
  D3  project_path disclosure is recorded at every get_project_state step
      (legacy face; the agent compact-face check runs in the real-stack leg).

Exit code: 0 = driver completed and wrote report.json; individual scenario
assertions are recorded as pass/fail in report.json (exit 1 only on driver
infrastructure failure so the report always lands).
"""
import json
import os
import sys
import time

import zmq

REQ_URL = os.environ.get("VIT_KERNEL_REQ", "tcp://127.0.0.1:5555")

FIXTURE_NOTES = [
    {"start_beat": 0.0, "length_beats": 1.0, "pitch": 60, "velocity": 90},
    {"start_beat": 1.0, "length_beats": 0.5, "pitch": 62, "velocity": 80},
    {"start_beat": 1.5, "length_beats": 0.5, "pitch": 64, "velocity": 80},
    {"start_beat": 2.0, "length_beats": 2.0, "pitch": 67, "velocity": 85},
]


def zmq_command(payload, timeout_ms=30000):
    ctx = zmq.Context.instance()
    s = ctx.socket(zmq.REQ)
    s.setsockopt(zmq.LINGER, 0)
    s.setsockopt(zmq.RCVTIMEO, timeout_ms)
    s.setsockopt(zmq.SNDTIMEO, timeout_ms)
    try:
        s.connect(REQ_URL)
        s.send_json(payload)
        return json.loads(s.recv_string())
    finally:
        s.close(0)


def state_summary(reply):
    """Compact view of the disclosure face the assertions need."""
    out = {
        "status": reply.get("status"),
        "project_path": reply.get("project_path"),
        "effective_project_path": reply.get("effective_project_path"),
        "project_uuid": reply.get("project_uuid"),
        "track_rows": [],
    }
    for track in reply.get("tracks") or []:
        row = {
            "track_id": track.get("track_id"),
            "track_name": track.get("track_name"),
            "is_audio_track": track.get("is_audio_track"),
            "clips": [
                {
                    "id": clip.get("id"),
                    "clip_id": clip.get("clip_id"),
                    "clip_type": clip.get("clip_type"),
                }
                for clip in (track.get("clips") or [])
            ],
        }
        out["track_rows"].append(row)
    out["audio_track_count"] = sum(
        1 for r in out["track_rows"] if r["is_audio_track"]
    )
    out["clip_rows_total"] = sum(len(r["clips"]) for r in out["track_rows"])
    return out


def find_track(state, track_id):
    for row in state["track_rows"]:
        if row["track_id"] == track_id:
            return row
    return None


def main():
    report = {
        "schema": "kernel_save_forensics_offline.v1",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "req_url": REQ_URL,
        "scenarios": {},
        "asserts": [],
    }

    def record(scenario, key, value):
        report["scenarios"].setdefault(scenario, {})[key] = value

    def check(name, ok, detail=""):
        report["asserts"].append({"name": name, "pass": bool(ok), "detail": detail})
        print(("PASS " if ok else "FAIL ") + name + (" :: " + detail if detail else ""))

    # -- boot probe ----------------------------------------------------------
    ping = zmq_command({"cmd": "ping"}, 10000)
    record("boot", "ping", ping)
    if ping.get("status") != "ok":
        print("kernel ping failed: " + json.dumps(ping)[:300])
        json.dump(report, open("report.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        return 1

    # -- S1: D1 repro on XML-boot -------------------------------------------
    s1 = {}
    add = zmq_command({"cmd": "add_track"})
    s1["add_track"] = add
    track_id = str(add.get("track_id", ""))
    check("S1-add-track-ok", add.get("status") == "ok" and track_id != "",
          json.dumps(add)[:200])

    ins = zmq_command({"cmd": "insert_midi_clip", "track_id": track_id,
                       "time_unit": "beats", "start": 0.0, "length": 4.0})
    s1["insert_midi_clip"] = ins
    clip_id = str(ins.get("clip_id", ins.get("new_clip_id", "")))
    check("S1-insert-midi-clip-ok", ins.get("status") == "ok" and clip_id != "",
          json.dumps(ins)[:200])

    notes = zmq_command({"cmd": "add_midi_notes", "track_id": track_id,
                         "clip_id": clip_id, "time_unit": "beats",
                         "notes": FIXTURE_NOTES})
    s1["add_midi_notes"] = notes
    check("S1-add-midi-notes-ok", notes.get("status") == "ok",
          json.dumps(notes)[:200])

    pre = zmq_command({"cmd": "get_project_state"})
    pre_state = state_summary(pre)
    s1["pre_save_state"] = pre_state
    record("S1", "pre_save_state", pre_state)
    pre_track = find_track(pre_state, track_id)
    check("S1-pre-save-track-disclosed", pre_track is not None,
          "track_id=" + track_id)
    check("S1-pre-save-clip-id-disclosed",
          pre_track is not None and any(c["id"] == clip_id for c in pre_track["clips"]),
          "clip_id=" + clip_id + " clips=" + json.dumps(pre_track["clips"] if pre_track else []))
    # D3 kernel face: XML-boot pre-save discloses the boot-file fallback while
    # the bound path stays honestly empty.
    boot_xml = os.path.abspath("default_project.xml").replace("\\", "/")
    check("S1-D3-pre-save-effective-path-disclosed",
          str(pre_state.get("effective_project_path") or "").replace("\\", "/") == boot_xml
          and str(pre_state.get("project_path") or "") == "",
          "effective=" + str(pre_state.get("effective_project_path")) + " project_path=" + str(pre_state.get("project_path")))

    # agent-equivalent enriched save for an unsaved XML-boot project:
    # file_path + app_bound encryption (mirrors harness ensureAppBoundVitSaveCommand)
    out_vit = os.path.abspath("out1.vit").replace("\\", "/")
    save = zmq_command({"cmd": "save_project", "file_path": out_vit,
                        "encryption": {"mode": "app_bound_aes"}})
    s1["save_project"] = save
    check("S1-save-ok", save.get("status") == "ok",
          json.dumps(save)[:300])

    post = zmq_command({"cmd": "get_project_state"})
    post_state = state_summary(post)
    s1["post_save_state"] = post_state
    record("S1", "post_save_state", post_state)
    post_track = find_track(post_state, track_id)
    check("S1-D1-post-save-track-disclosed", post_track is not None,
          "track_id=" + track_id + " (D1: dropped?)" if post_track is None else "track present")
    check("S1-D1-post-save-clip-id-disclosed",
          post_track is not None and any(c["id"] == clip_id for c in post_track["clips"]),
          "clip_id=" + clip_id)
    check("S1-D3-post-save-paths-bound",
          str(post_state.get("project_path") or "").replace("\\", "/") == out_vit
          and str(post_state.get("effective_project_path") or "").replace("\\", "/") == out_vit,
          "project_path=" + str(post_state.get("project_path")) + " effective=" + str(post_state.get("effective_project_path")))

    g1 = zmq_command({"cmd": "get_midi_clip_notes", "clip_id": clip_id})
    s1["post_save_get_midi_clip_notes"] = {"status": g1.get("status"),
                                           "note_count": len(g1.get("notes") or [])}
    check("S1-D1-post-save-notes-readable", g1.get("status") == "ok",
          json.dumps(g1)[:200])
    record("S1", "replies", {k: v for k, v in s1.items() if k.endswith(("track", "clip", "notes", "project"))})

    # -- S2: reopen control ---------------------------------------------------
    s2 = {}
    op = zmq_command({"cmd": "open_project", "file_path": out_vit})
    s2["open_project"] = {"status": op.get("status"),
                          "project_path": op.get("project_path")}
    check("S2-open-ok", op.get("status") == "ok", json.dumps(op)[:300])

    rstate = zmq_command({"cmd": "get_project_state"})
    r_state = state_summary(rstate)
    s2["reopen_state"] = r_state
    record("S2", "reopen_state", r_state)
    r_track = find_track(r_state, track_id)
    check("S2-reopen-track-disclosed", r_track is not None,
          "track_id=" + track_id)
    check("S2-reopen-clip-id-disclosed",
          r_track is not None and any(c["id"] == clip_id for c in r_track["clips"]),
          "clip_id=" + clip_id)

    g2 = zmq_command({"cmd": "get_midi_clip_notes", "clip_id": clip_id})
    s2["reopen_get_midi_clip_notes"] = {"status": g2.get("status"),
                                        "note_count": len(g2.get("notes") or [])}
    check("S2-reopen-notes-readable", g2.get("status") == "ok",
          json.dumps(g2)[:200])
    record("S2", "replies", s2)

    # -- S3: D2 repro on .vit-boot session ------------------------------------
    s3 = {}
    add2 = zmq_command({"cmd": "add_track"})
    s3["add_track"] = add2
    track2 = str(add2.get("track_id", ""))
    check("S3-add-track-ok", add2.get("status") == "ok" and track2 != "",
          json.dumps(add2)[:200])

    ins2 = zmq_command({"cmd": "insert_midi_clip", "track_id": track2,
                        "time_unit": "beats", "start": 0.0, "length": 4.0})
    s3["insert_midi_clip"] = ins2
    clip2 = str(ins2.get("clip_id", ins2.get("new_clip_id", "")))
    check("S3-insert-midi-clip-ok", ins2.get("status") == "ok" and clip2 != "",
          json.dumps(ins2)[:200])

    notes2 = zmq_command({"cmd": "add_midi_notes", "track_id": track2,
                          "clip_id": clip2, "time_unit": "beats",
                          "notes": FIXTURE_NOTES})
    s3["add_midi_notes"] = notes2
    check("S3-add-midi-notes-ok", notes2.get("status") == "ok",
          json.dumps(notes2)[:200])

    pre2 = zmq_command({"cmd": "get_project_state"})
    pre2_state = state_summary(pre2)
    s3["pre_save_state"] = pre2_state
    record("S3", "pre_save_state", pre2_state)
    pre2_track = find_track(pre2_state, track2)
    check("S3-pre-save-clip-id-disclosed",
          pre2_track is not None and any(c["id"] == clip2 for c in pre2_track["clips"]),
          "clip2=" + clip2)

    save2 = zmq_command({"cmd": "save_project", "file_path": out_vit,
                         "encryption": {"mode": "app_bound_aes"}})
    s3["save_project"] = save2
    check("S3-save-ok", save2.get("status") == "ok", json.dumps(save2)[:300])

    post2 = zmq_command({"cmd": "get_project_state"})
    post2_state = state_summary(post2)
    s3["post_save_state"] = post2_state
    record("S3", "post_save_state", post2_state)
    post2_track = find_track(post2_state, track2)
    check("S3-D2-post-save-track-disclosed", post2_track is not None,
          "track2=" + track2)
    check("S3-D2-post-save-clip-id-disclosed",
          post2_track is not None and any(c["id"] == clip2 for c in post2_track["clips"]),
          "clip2=" + clip2 + " clips=" + json.dumps(post2_track["clips"] if post2_track else []))

    g3 = zmq_command({"cmd": "get_midi_clip_notes", "clip_id": clip2})
    s3["post_save_get_midi_clip_notes"] = {"status": g3.get("status"),
                                           "note_count": len(g3.get("notes") or [])}
    check("S3-D2-post-save-notes-readable", g3.get("status") == "ok",
          json.dumps(g3)[:200])

    op2 = zmq_command({"cmd": "open_project", "file_path": out_vit})
    s3["open_project"] = {"status": op2.get("status")}
    check("S3-reopen-ok", op2.get("status") == "ok", json.dumps(op2)[:300])

    post3 = zmq_command({"cmd": "get_project_state"})
    post3_state = state_summary(post3)
    s3["reopen_state"] = post3_state
    record("S3", "reopen_state", post3_state)
    post3_track = find_track(post3_state, track2)
    check("S3-reopen-track-disclosed", post3_track is not None, "track2=" + track2)
    check("S3-reopen-clip-id-disclosed",
          post3_track is not None and any(c["id"] == clip2 for c in post3_track["clips"]),
          "clip2=" + clip2)

    g4 = zmq_command({"cmd": "get_midi_clip_notes", "clip_id": clip2})
    s3["reopen_get_midi_clip_notes"] = {"status": g4.get("status"),
                                        "note_count": len(g4.get("notes") or [])}
    check("S3-reopen-notes-readable", g4.get("status") == "ok",
          json.dumps(g4)[:200])
    record("S3", "replies", s3)

    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    report["exit_code"] = 0
    json.dump(report, open("report.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    failed = [a for a in report["asserts"] if not a["pass"]]
    print("ASSERTS: %d pass, %d fail" % (len(report["asserts"]) - len(failed), len(failed)))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:  # infrastructure failure: still drop a report
        try:
            json.dump({"schema": "kernel_save_forensics_offline.v1",
                       "driver_error": repr(exc), "exit_code": 1},
                      open("report.json", "w", encoding="utf-8"), indent=1)
        except Exception:
            pass
        raise
