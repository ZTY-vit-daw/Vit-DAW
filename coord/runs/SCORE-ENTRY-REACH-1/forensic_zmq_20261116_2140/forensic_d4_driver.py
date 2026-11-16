# SCORE-ENTRY-REACH-1 (382) forensic leg: D4 pathology triage (kernel-only ZMQ).
#
# Questions (RUNBOOK): is length_seconds=8.0 on an 80bpm template clip
#   A) template XML unit mismatch (clip XML attr holds beats, loader reads seconds),
#   B) kernel write-path conversion failure (insert_midi_clip echoes beats as seconds),
#   C) read/write inconsistency (insert echo != get disclosure)?
# Deterministic REQ command face only; zero LLM. Writes forensic_report.json.

import json
import os
import sys
import time

import zmq

REQ_URL = os.environ.get("VIT_KERNEL_REQ", "tcp://127.0.0.1:5555")
RUN_DIR = os.path.dirname(os.path.abspath(__file__))
REPORT_PATH = os.path.join(RUN_DIR, "forensic_report.json")
ROUNDTRIP_VIT = os.path.join(RUN_DIR, "forensic_roundtrip.vit")

report = {
    "schema": "score_entry_reach_1.forensic_d4.v1",
    "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
    "req_url": REQ_URL,
    "steps": [],
    "assertions": {},
    "verdict": None,
    "verdict_reason": "",
}


def zmq_command(payload, timeout_ms=60000):
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


def step(name, payload, timeout_ms=60000):
    reply = zmq_command(payload, timeout_ms)
    report["steps"].append({"step": name, "cmd": payload.get("cmd"), "reply": reply})
    print("[step]", name, "->", json.dumps(reply, ensure_ascii=False)[:400])
    return reply


def check(key, ok, detail):
    report["assertions"][key] = {"ok": bool(ok), "detail": detail}
    print(("[PASS] " if ok else "[FAIL] ") + key + ": " + detail)


def clip_rows(state_reply):
    rows = []
    for t in state_reply.get("tracks", []) or []:
        track_id = str(t.get("track_id") or t.get("id") or "")
        name = str(t.get("name") or "")
        clip_ids = [str(c.get("clip_id") or c.get("id") or "") for c in (t.get("clips") or [])]
        rows.append({"track_id": track_id, "name": name, "clip_ids": clip_ids})
    return rows


# ---- F0 boot sanity ----
ping = step("ping", {"cmd": "ping"}, 15000)
if ping.get("status") != "ok" and ping.get("cmd") != "ping":
    # ping reply shape on this kernel: {'cmd':'ping','status':'ok',...} or plain ok
    if str(ping).find("ok") < 0:
        report["verdict"] = "env_interrupt"
        report["verdict_reason"] = "ping not ok: " + json.dumps(ping)[:200]
        json.dump(report, open(REPORT_PATH, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
        sys.exit(2)

state = step("get_project_state", {"cmd": "get_project_state"})
# Attempt1 probe_defect fix: the ZMQ get_project_state track objects carry no
# name/clips (that merged shape is agent-side). Track names come from
# list_tracks; the template melody clip id comes from the boot XML itself
# (tracktion EditItemID survives the XML id attribute).
lt = step("list_tracks", {"cmd": "list_tracks"})
tracks_lt = lt.get("tracks", []) or []
melody_track_id = ""
for t in tracks_lt:
    if "旋律" in str(t.get("name", "")) and t.get("is_audio_track"):
        melody_track_id = str(t.get("track_id", ""))
        break
check("F0-template-melody-track-found", melody_track_id != "",
      "list_tracks names=" + json.dumps([str(t.get("name")) for t in tracks_lt], ensure_ascii=False)[:220])

# template clip id: first MIDICLIP inside the melody track block of the boot XML
# (regex over ElementTree: the template carries undeclared namespace prefixes,
# strict XML parsing fails -- attempt2 probe fix)
import html as _html
import re as _re
with open(os.path.join(RUN_DIR, "kernel_runtime", "Workspace", "default_project.xml"), "r", encoding="utf-8") as f:
    _boot = f.read()
melody_clip_xml_id = ""
# attempt4 probe fix: the kernel rewrote the boot XML (names are xml-escaped
# entities now); unescape before matching. Track/clip ids stay numeric.
_boot_u = _html.unescape(_boot)
_m = _re.search(r'<TRACK name="[^"]*旋律[^"]*"[^>]*>(.*?)(?=<TRACK |</EDIT>|$)', _boot_u, _re.S)
if _m:
    _c = _re.search(r'<MIDICLIP [^>]*\bid="([^"]+)"', _m.group(1))
    if _c:
        melody_clip_xml_id = _c.group(1)
check("F0-template-clip-xml-id", melody_clip_xml_id != "", "melody track first MIDICLIP xml id=" + melody_clip_xml_id)
if not melody_track_id or not melody_clip_xml_id:
    report["verdict"] = "probe_defect"
    report["verdict_reason"] = "melody track/clip discovery failed"
    json.dump(report, open(REPORT_PATH, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    sys.exit(3)

melody_clip_id = melody_clip_xml_id

# ---- F1 tempo disclosure (music_context truth; state reply carries it) ----
mc = state.get("music_context", {}) or {}
tempo = mc.get("tempo_bpm")
report["tempo_bpm"] = tempo
check("F1-tempo-echo", tempo is not None and abs(float(tempo) - 80.0) < 1e-6,
      "get_project_state music_context tempo_bpm=" + str(tempo))

# ---- F2 template clip disclosure (377 D4 repro anchor) ----
tpl = step("get_midi_clip_data-template", {"cmd": "get_midi_clip_data", "clip_id": melody_clip_id})
tpl_len = tpl.get("length_seconds")
tpl_start = tpl.get("start_seconds")
report["template_clip"] = {"clip_id": melody_clip_id, "start_seconds": tpl_start, "length_seconds": tpl_len}
check("F2-template-length-disclosed", isinstance(tpl_len, (int, float)),
      "template melody clip length_seconds=" + str(tpl_len) + " start_seconds=" + str(tpl_start))

# ---- F3 write path: insert_midi_clip beats 8 (expect echo 6.0s @80bpm) ----
new_track = step("add_track", {"cmd": "add_track"})
new_track_id = str(new_track.get("track_id") or new_track.get("id") or "")
ins8 = step("insert_midi_clip-beats8",
            {"cmd": "insert_midi_clip", "track_id": new_track_id, "time_unit": "beats", "start": 0, "length": 8})
ins8_len = ins8.get("length_seconds")
ins8_clip = str(ins8.get("clip_id") or ins8.get("new_clip_id") or "")
report["insert_beats8"] = {"clip_id": ins8_clip, "echo_length_seconds": ins8_len}
check("F3-insert-beats8-echo", ins8.get("status") == "ok" and isinstance(ins8_len, (int, float)),
      "insert(beats,8) echo length_seconds=" + str(ins8_len))

# ---- F3b identity probe: insert beats 6 (80bpm truth 4.5s; 6.0 == 60bpm identity) ----
ins6 = step("insert_midi_clip-beats6",
            {"cmd": "insert_midi_clip", "track_id": new_track_id, "time_unit": "beats", "start": 0, "length": 6})
ins6_len = ins6.get("length_seconds")
ins6_clip = str(ins6.get("clip_id") or ins6.get("new_clip_id") or "")
report["insert_beats6"] = {"clip_id": ins6_clip, "echo_length_seconds": ins6_len}
check("F3b-insert-beats6-echo", ins6.get("status") == "ok" and isinstance(ins6_len, (int, float)),
      "insert(beats,6) echo length_seconds=" + str(ins6_len))

# ---- F4 readback of the two fresh clips ----
get8 = step("get_midi_clip_data-ins8", {"cmd": "get_midi_clip_data", "clip_id": ins8_clip})
get6 = step("get_midi_clip_data-ins6", {"cmd": "get_midi_clip_data", "clip_id": ins6_clip})
get8_len = get8.get("length_seconds")
get6_len = get6.get("length_seconds")
report["readback"] = {"ins8": get8_len, "ins6": get6_len}
consistent = (abs(float(get8_len) - float(ins8_len)) < 1e-9) and (abs(float(get6_len) - float(ins6_len)) < 1e-9)
check("F4-readback-consistent", consistent,
      "echo8=" + str(ins8_len) + " get8=" + str(get8_len) + " echo6=" + str(ins6_len) + " get6=" + str(get6_len))

# ---- F5 save->open roundtrip ----
if os.path.exists(ROUNDTRIP_VIT):
    os.unlink(ROUNDTRIP_VIT)
save = step("save_project", {"cmd": "save_project", "file_path": ROUNDTRIP_VIT})
check("F5-save-ok", save.get("status") == "ok", json.dumps(save, ensure_ascii=False)[:160])
opened = step("open_project", {"cmd": "open_project", "file_path": ROUNDTRIP_VIT})
check("F5-open-ok", opened.get("status") == "ok", json.dumps(opened, ensure_ascii=False)[:160])
state2 = step("get_project_state-postreopen", {"cmd": "get_project_state"})
tpl2_len = None
ins8b_len = None
# attempt3 probe fix: tracktion preserves itemIDs across save->open; query the
# known clip ids directly (state reply carries no clip rows on the ZMQ face).
if str(state2.get("status")) == "ok":
    tpl2 = step("get_midi_clip_data-template-postreopen",
                {"cmd": "get_midi_clip_data", "clip_id": melody_clip_id})
    if str(tpl2.get("status")) == "ok":
        tpl2_len = tpl2.get("length_seconds")
    g8 = step("get_midi_clip_data-ins8-postreopen", {"cmd": "get_midi_clip_data", "clip_id": ins8_clip})
    if str(g8.get("status")) == "ok":
        ins8b_len = g8.get("length_seconds")
report["roundtrip"] = {"template_length_seconds": tpl2_len, "insert8_length_seconds": ins8b_len}
check("F5-roundtrip-stable",
      tpl2_len is not None and abs(float(tpl2_len) - float(tpl_len)) < 1e-9,
      "template " + str(tpl_len) + " -> " + str(tpl2_len))

# ---- verdict triage ----
def num(x):
    return float(x)


if any(not v["ok"] for v in report["assertions"].values()):
    report["verdict"] = "inconclusive"
    report["verdict_reason"] = "one or more assertions failed; see assertions"
else:
    insert_correct = abs(num(ins8_len) - 6.0) < 1e-6 and abs(num(ins6_len) - 4.5) < 1e-6
    insert_identity60 = abs(num(ins8_len) - 8.0) < 1e-6 and abs(num(ins6_len) - 6.0) < 1e-6
    if insert_correct and abs(num(tpl_len) - 8.0) < 1e-6:
        report["verdict"] = "template_xml_unit_mismatch"
        report["verdict_reason"] = (
            "insert path converts beats->seconds correctly (echo 8 beats=6.0s, 6 beats=4.5s @80bpm) "
            "while the template melody clip discloses length_seconds=8.0: the hand-written 373 template "
            "XML MIDICLIP length attribute holds beats but the tracktion loader reads it as seconds. "
            "Kernel command face is correct; fix surface = webui defense + template asset (out of card domain).")
    elif insert_identity60:
        report["verdict"] = "kernel_write_path_bug"
        report["verdict_reason"] = (
            "insert_midi_clip echoes beat-count numerals as seconds (8.0/6.0 identity => effective 60bpm "
            "or no conversion in the write path): kernel-side fix is in scope per card goal 0.")
    elif not consistent:
        report["verdict"] = "readback_inconsistent"
        report["verdict_reason"] = "insert echo disagrees with get_midi_clip_data disclosure"
    else:
        report["verdict"] = "inconclusive"
        report["verdict_reason"] = (
            "unexpected numeric shape: tpl_len=%s ins8=%s ins6=%s" % (tpl_len, ins8_len, ins6_len))

report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
json.dump(report, open(REPORT_PATH, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
print("[verdict]", report["verdict"], "-", report["verdict_reason"])
sys.exit(0)
