# 384 leg D gate driver v3 (attempt3; attempt1/2 = probe_defect save-form
# findings, product face green throughout): full sequence against a kernel
# booted with VIT_DUMP_CLEAR_XML=1 — add track, instantiate Surge XT,
# save_as_project (.vit normalized + clear-XML sidecar), then extract.
import json
import os
import sys
import time

import zmq

REQ_URL = "tcp://127.0.0.1:5555"
SURGE_IDENTIFIER = "VST3-Surge XT-1e1bcfb-190e4fbd"
BASE = os.path.abspath(".").replace("\\", "/")
OUT_BASE = BASE + "/legd_gate_project"


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


def main():
    report = {
        "schema": "src_preset_legd_gate_v3.v1",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "out_base": OUT_BASE,
    }
    out = "legd_gate_report_v3.json"

    def save():
        json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    ping = zmq_command({"cmd": "ping"}, 10000)
    report["ping"] = ping
    if ping.get("status") != "ok":
        save()
        return 1

    add = zmq_command({"cmd": "add_track"})
    report["add_track"] = add
    track_id = str(add.get("track_id", ""))
    if add.get("status") != "ok" or not track_id:
        print("add_track failed:", json.dumps(add)[:300])
        save()
        return 1

    inst = zmq_command({"cmd": "instantiate_plugin", "track_id": track_id,
                        "plugin_identifier": SURGE_IDENTIFIER})
    report["instantiate_plugin"] = inst
    if inst.get("status") != "ok":
        print("instantiate failed:", json.dumps(inst)[:300])
        save()
        return 1
    time.sleep(3)

    params = zmq_command({"cmd": "get_plugin_parameters", "track_id": track_id,
                          "plugin_id": str(inst.get("plugin_id", ""))})
    report["params"] = {
        "status": params.get("status"),
        "param_count": len(params.get("binding_targets") or params.get("parameters") or []),
        "template_role": params.get("template_role"),
    }

    save_reply = zmq_command({"cmd": "save_as_project", "file_path": OUT_BASE + ".xml"})
    report["save_as_project"] = save_reply
    sidecar = OUT_BASE + ".xml"
    vit = OUT_BASE + ".vit"
    report["sidecar_exists"] = os.path.exists(sidecar)
    report["vit_exists"] = os.path.exists(vit)
    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    save()
    ok = save_reply.get("status") == "ok" and report["sidecar_exists"]
    print("GATE_V3_" + ("OK" if ok else "FAIL"),
          json.dumps({"save": save_reply.get("status"),
                      "path": save_reply.get("project_path"),
                      "sidecar": report["sidecar_exists"]})[:220])
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
