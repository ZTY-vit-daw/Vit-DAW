# 384 leg D stack boot B: verify the FINAL template (beats fix + timbre swap).
# Boot: kernel_runtime with VIT_PROJECT_XML = regenerated template.v1.xml.
# Assertions:
#   A1 five design tracks present
#   A2 clip length_seconds ~= 6.0/6.0/12.0 (beats 8/8/16 @80bpm)  [382 fix]
#   A3 per-clip notes via get_midi_clip_data (8/21/16; groove keys 36/38/42/46)
#   A4 three Surge plugins pinned with the two crafted state shas
#   B  offline_probe non-silence on melody/rhythm/theme tracks
#      (373 DG1-B B2/B3 criteria: ready + nonzero + max_abs >= 0.01 + nan0)
import hashlib
import json
import os
import re
import sys
import time

import zmq

REQ_URL = "tcp://127.0.0.1:5555"
SUB_URL = "tcp://127.0.0.1:5556"
RUN_DIR = os.path.abspath(".")
TEMPLATE = os.path.abspath(os.path.join(
    RUN_DIR, "..", "..", "..", "..",
    "coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml"))
STATES_DIR = os.path.abspath(os.path.join(
    RUN_DIR, "..", "..", "..", "..",
    "coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/states"))

PROBE_MAX_ABS_MIN = 0.01
TAIL_SECONDS = 2.0
RENDER_WAIT_SEC = 120.0

TRACK_NAMES = {"参考与示范", "我的旋律", "辅助律动（鼓）", "我的八小节主题", "我的成曲版本"}
CLIP_SPEC = {
    "rhythm-lab": {"seconds": 6.0, "beats": 8, "notes": 8, "track": "我的旋律"},
    "groove-2-4-basic": {"seconds": 6.0, "beats": 8, "notes": 21, "track": "辅助律动（鼓）"},
    "eight-bar-theme": {"seconds": 12.0, "beats": 16, "notes": 16, "track": "我的八小节主题"},
}
GROOVE_KEYS = {36, 38, 42, 46}


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


def wait_probe_ready(request_id, timeout_s):
    ctx = zmq.Context.instance()
    sock = ctx.socket(zmq.SUB)
    sock.setsockopt(zmq.LINGER, 0)
    sock.setsockopt_string(zmq.SUBSCRIBE, "")
    sock.setsockopt(zmq.RCVTIMEO, 1000)
    deadline = time.time() + timeout_s
    try:
        sock.connect(SUB_URL)
        while time.time() < deadline:
            try:
                payload = sock.recv_string()
            except zmq.error.Again:
                continue
            try:
                message = json.loads(payload)
            except json.JSONDecodeError:
                continue
            if not isinstance(message, dict):
                continue
            if str(message.get("request_id", "")) != request_id:
                continue
            command = str(message.get("command", ""))
            if command in ("l2_render_probe_ready", "l2_render_probe_status", "l2_render_probe_failed"):
                return {"event": message, "timed_out": False}
    finally:
        sock.close(0)
    return {"event": None, "timed_out": True}


def probe_track(track_id, label, clip_seconds):
    request_id = "legd_{}_{}".format(label, int(time.time() * 1000))
    reply = zmq_command({
        "cmd": "l2_render_probe", "request_id": request_id, "track_id": track_id,
        "tap_point": "track_post_fader", "render_mode": "offline_probe",
        "start": 0.0, "end": clip_seconds + TAIL_SECONDS, "tail_seconds": TAIL_SECONDS,
    })
    if reply.get("status") != "ok":
        return {"ok": False, "reason": "status=%r" % reply.get("status"), "reply": reply}
    outcome = wait_probe_ready(request_id, RENDER_WAIT_SEC)
    event = outcome.get("event") or {}
    evidence = event.get("quality_evidence") if isinstance(event.get("quality_evidence"), dict) else {}
    return {
        "ok": True, "probe_status": event.get("status"),
        "nonzero": evidence.get("nonzero"), "max_abs": evidence.get("max_abs"),
        "sum_abs": evidence.get("sum_abs"), "nan_inf_count": evidence.get("nan_inf_count"),
        "timed_out": outcome.get("timed_out"),
    }


def main():
    report = {"schema": "src_preset_legd_verify.v1",
              "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"), "asserts": []}
    out = "verify_report.json"

    def check(name, ok, detail=""):
        report["asserts"].append({"name": name, "pass": bool(ok), "detail": str(detail)[:400]})
        print(("PASS " if ok else "FAIL ") + name + ((" :: " + str(detail)[:200]) if detail else ""))

    def save():
        json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    melody_sha = hashlib.sha256(open(os.path.join(STATES_DIR, "melody_soft_keys_v1.state.txt"), "rb").read().strip()).hexdigest()
    groove_sha = hashlib.sha256(open(os.path.join(STATES_DIR, "groove_percussion_v1.state.txt"), "rb").read().strip()).hexdigest()

    ping = zmq_command({"cmd": "ping"}, 10000)
    if ping.get("status") != "ok":
        check("boot-ping", False, ping); save(); return 1
    check("boot-ping", True)

    state = zmq_command({"cmd": "get_project_state"})
    if state.get("status") != "ok":
        check("A1-tracks", False, state); save(); return 1
    tracks = state.get("tracks") or []
    names = {t.get("track_name") for t in tracks}
    # Superset, not equality: get_project_state also lists the global tracks
    # (Arranger/Chord/Marker/Master/Tempo). attempt1 demanded equality —
    # probe_defect, product face was always green.
    check("A1-tracks", TRACK_NAMES <= names, sorted(names))

    clips_by_name = {}
    track_by_name = {}
    for t in tracks:
        track_by_name[t.get("track_name")] = t
        for c in t.get("clips") or []:
            clips_by_name[c.get("name")] = (c, t)

    for alias, spec in CLIP_SPEC.items():
        got = clips_by_name.get(alias)
        if not got:
            check("A2-clip-" + alias, False, "clip missing"); continue
        clip, _ = got
        seconds = float(clip.get("length_seconds", -1))
        beats = seconds * 80.0 / 60.0
        check("A2-clip-" + alias, abs(seconds - spec["seconds"]) < 0.01,
              "length_seconds=%s beats=%s" % (clip.get("length_seconds"), round(beats, 3)))

    for alias, spec in CLIP_SPEC.items():
        got = clips_by_name.get(alias)
        if not got:
            check("A3-notes-" + alias, False, "clip missing"); continue
        clip, _ = got
        data = zmq_command({"cmd": "get_midi_clip_data", "clip_id": clip.get("id")})
        notes = data.get("notes") or []
        ok = data.get("status") == "ok" and len(notes) == spec["notes"]
        detail = "notes=%d want=%d length_seconds=%s" % (len(notes), spec["notes"], data.get("length_seconds"))
        if ok and alias == "groove-2-4-basic":
            pitches = {int(n.get("p", n.get("pitch", 0))) for n in notes}
            ok = pitches <= GROOVE_KEYS
            detail += " pitches=%s" % sorted(pitches)
        if ok and alias in ("rhythm-lab", "eight-bar-theme"):
            ok = all(int(n.get("p", n.get("pitch", 0))) == 60 for n in notes)
        check("A3-notes-" + alias, ok, detail)

    xml_text = open(TEMPLATE, "rb").read().decode("utf-8", errors="replace")
    shas = []
    for sm in re.finditer(r'state="(\d+)\.([A-Za-z0-9+.]+)"', xml_text):
        blob = sm.group(1) + "." + sm.group(2)
        shas.append(hashlib.sha256(blob.encode()).hexdigest())
    check("A4-plugin-states", sorted(shas) == sorted([melody_sha, melody_sha, groove_sha]),
          "melody=%s groove=%s got=%s" % (melody_sha[:12], groove_sha[:12], [s[:12] for s in shas]))

    for alias, spec in CLIP_SPEC.items():
        got = clips_by_name.get(alias)
        if not got:
            check("B-probe-" + alias, False, "clip missing"); continue
        _, track = got
        result = probe_track(str(track.get("track_id")), alias, spec["seconds"])
        if not result.get("ok"):
            check("B-probe-" + alias, False, result.get("reason")); continue
        ok = (not result.get("timed_out") and result.get("nonzero") is True
              and result.get("max_abs") is not None and result["max_abs"] >= PROBE_MAX_ABS_MIN
              and result.get("nan_inf_count") == 0)
        check("B-probe-" + alias, ok,
              "status=%s nonzero=%s max_abs=%s nan=%s" % (
                  result.get("probe_status"), result.get("nonzero"),
                  result.get("max_abs"), result.get("nan_inf_count")))
        report.setdefault("probes", {})[alias] = result

    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    save()
    failed = [a for a in report["asserts"] if not a["pass"]]
    print("VERIFY_" + ("PASS" if not failed else "FAIL") + " (%d/%d)" % (
        len(report["asserts"]) - len(failed), len(report["asserts"])))
    return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
