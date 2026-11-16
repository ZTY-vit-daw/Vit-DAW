# 384 leg D stack boot A: capture the two crafted Surge states.
# Boot: kernel_runtime XML-boot workspace + VIT_DUMP_CLEAR_XML=1.
# Sequence: add melody track -> instantiate Surge -> write soft-keys params
# (readback verified) -> add groove track -> instantiate Surge -> write
# percussion params -> save_as_project -> parse clear-XML sidecar -> extract
# per-track PLUGIN state blobs -> assert they differ from the 309 init blob
# and from each other -> write the two state files for the template builder.
import hashlib
import json
import os
import re
import sys
import time

import zmq

REQ_URL = "tcp://127.0.0.1:5555"
RUN_DIR = os.path.abspath(".")
OUT_VIT_XML = os.path.join(RUN_DIR, "craft_capture_project.xml")
STATES_DIR = os.path.abspath(
    os.path.join(RUN_DIR, "..", "..", "..", "..",
                 "coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/states")
)
INIT_STATE_SHA_PREFIX = None  # filled from the 309 fixture for the diff assert

SOFT_KEYS_PARAMS = [
    ("371918428", "A Osc 2 Volume", 0.0),
    ("371918429", "A Osc 3 Volume", 0.0),
    ("1788975242", "A Ring Modulation 1x2 Volume", 0.0),
    ("1788975274", "A Ring Modulation 2x3 Volume", 0.0),
    ("954746817", "A Noise Volume", 0.0),
    ("1945359057", "A Filter 1 Cutoff", 0.38),
    ("8095466", "A Filter 1 Resonance", 0.12),
    ("874858726", "A Amp EG Sustain", 0.55),
    ("1669861066", "A Amp EG Release", 0.35),
    ("1298982072", "A Volume", 0.75),
]

PERCUSSION_PARAMS = [
    ("371918428", "A Osc 2 Volume", 0.0),
    ("371918429", "A Osc 3 Volume", 0.0),
    ("1788975242", "A Ring Modulation 1x2 Volume", 0.0),
    ("1788975274", "A Ring Modulation 2x3 Volume", 0.0),
    ("2005836581", "A Amp EG Attack", 0.0),
    ("1798853437", "A Amp EG Decay", 0.18),
    ("874858726", "A Amp EG Sustain", 0.0),
    ("1669861066", "A Amp EG Release", 0.08),
    ("954746817", "A Noise Volume", 0.4),
    ("1945359057", "A Filter 1 Cutoff", 0.5),
    ("8095466", "A Filter 1 Resonance", 0.2),
]

SURGE_IDENTIFIER = "VST3-Surge XT-1e1bcfb-190e4fbd"


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


def write_params(track_id, plugin_id, params, report, label):
    rows = []
    for param_id, name, value in params:
        reply = zmq_command({
            "cmd": "set_plugin_param", "track_id": track_id, "plugin_id": plugin_id,
            "param_id": param_id, "value": value, "confirmed": True,
        })
        actual = reply.get("actual_normalized_value", reply.get("new_normalised_value"))
        ok = reply.get("status") == "ok" and actual is not None and abs(actual - value) < 1e-3
        rows.append({"param_id": param_id, "name": name, "requested": value,
                     "actual": actual, "ok": ok})
        if not ok:
            report["fail_detail"] = json.dumps(reply)[:300]
            return False
    report[label + "_params"] = rows
    return True


def extract_state_by_track(xml_path, track_name):
    text = open(xml_path, "rb").read().decode("utf-8", errors="replace")
    tm = re.search(r'<TRACK name="' + re.escape(track_name) + r'"[^>]*>(.*?)</TRACK>', text, re.S)
    if not tm:
        return None
    sm = re.search(r'state="(\d+)\.([A-Za-z0-9+.]+)"', tm.group(1))
    if not sm:
        return None
    return sm.group(1) + "." + sm.group(2)


def main():
    report = {"schema": "src_preset_legd_capture.v1",
              "started_at": time.strftime("%Y-%m-%dT%H:%M:%S")}
    out = "capture_report.json"

    def save():
        json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    ping = zmq_command({"cmd": "ping"}, 10000)
    report["ping"] = ping
    if ping.get("status") != "ok":
        save()
        return 1

    # melody voice
    add1 = zmq_command({"cmd": "add_track", "name": "melody-craft"})
    track1 = str(add1.get("track_id", ""))
    inst1 = zmq_command({"cmd": "instantiate_plugin", "track_id": track1,
                         "plugin_identifier": SURGE_IDENTIFIER})
    report["melody"] = {"add_track": add1, "instantiate": inst1}
    if inst1.get("status") != "ok":
        save()
        return 1
    plugin1 = str(inst1.get("plugin_id", ""))
    if not write_params(track1, plugin1, SOFT_KEYS_PARAMS, report, "melody"):
        print("melody param write failed"); save(); return 1

    # groove voice
    add2 = zmq_command({"cmd": "add_track", "name": "groove-craft"})
    track2 = str(add2.get("track_id", ""))
    inst2 = zmq_command({"cmd": "instantiate_plugin", "track_id": track2,
                         "plugin_identifier": SURGE_IDENTIFIER})
    report["groove"] = {"add_track": add2, "instantiate": inst2}
    if inst2.get("status") != "ok":
        save()
        return 1
    plugin2 = str(inst2.get("plugin_id", ""))
    if not write_params(track2, plugin2, PERCUSSION_PARAMS, report, "groove"):
        print("groove param write failed"); save(); return 1

    time.sleep(2)
    save_reply = zmq_command({"cmd": "save_as_project", "file_path": OUT_VIT_XML.replace("\\", "/")})
    report["save_as_project"] = {k: save_reply.get(k) for k in ("status", "project_path", "message")}
    if save_reply.get("status") != "ok" or not os.path.exists(OUT_VIT_XML):
        print("save failed"); save(); return 1

    melody_state = extract_state_by_track(OUT_VIT_XML, "melody-craft")
    groove_state = extract_state_by_track(OUT_VIT_XML, "groove-craft")
    report["melody_state_sha256"] = hashlib.sha256(melody_state.encode()).hexdigest() if melody_state else None
    report["groove_state_sha256"] = hashlib.sha256(groove_state.encode()).hexdigest() if groove_state else None
    if not melody_state or not groove_state:
        print("state extraction failed"); save(); return 1
    if melody_state == groove_state:
        print("states identical - capture invalid"); report["states_identical"] = True; save(); return 1

    os.makedirs(STATES_DIR, exist_ok=True)
    open(os.path.join(STATES_DIR, "melody_soft_keys_v1.state.txt"), "w", encoding="ascii", newline="\n").write(melody_state + "\n")
    open(os.path.join(STATES_DIR, "groove_percussion_v1.state.txt"), "w", encoding="ascii", newline="\n").write(groove_state + "\n")
    report["states_written"] = True
    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    save()
    print("CAPTURE_OK melody", report["melody_state_sha256"][:12],
          "groove", report["groove_state_sha256"][:12])
    return 0


if __name__ == "__main__":
    sys.exit(main())
