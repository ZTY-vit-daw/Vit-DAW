# 384 leg D gate driver v2: attempt1 save_project{} was rejected with
# require_path (XML-boot session has no bound path) — classified probe_defect
# (wrong invocation form). Same live session; save_as_project to a .xml path
# (not .vit, no encryption field) writes the clear-XML branch per
# VitHeadlessService::ipcSaveAsProjectAt.
import json
import os
import sys
import time

import zmq

REQ_URL = "tcp://127.0.0.1:5555"
OUT_XML = os.path.abspath("legd_gate_project.xml").replace("\\", "/")


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
        "schema": "src_preset_legd_gate_v2.v1",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "out_xml": OUT_XML,
    }
    out = "legd_gate_report_v2.json"

    def save():
        json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    ping = zmq_command({"cmd": "ping"}, 10000)
    report["ping"] = ping
    if ping.get("status") != "ok":
        save()
        return 1

    # Session from attempt1 still holds track 1007 + Surge XT (plugin 1010);
    # re-read its parameter surface to prove the instance is still live.
    params = zmq_command({"cmd": "get_plugin_parameters", "track_id": "1007", "plugin_id": "1010"})
    report["params_recheck"] = {
        "status": params.get("status"),
        "param_count": len(params.get("binding_targets") or params.get("parameters") or []),
        "template_role": params.get("template_role"),
    }
    if params.get("status") != "ok":
        print("plugin recheck failed:", json.dumps(params)[:200])
        save()
        return 1

    save_reply = zmq_command({"cmd": "save_as_project", "file_path": OUT_XML})
    report["save_as_project"] = save_reply
    report["file_exists_after"] = os.path.exists(OUT_XML)
    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    save()
    ok = save_reply.get("status") == "ok" and report["file_exists_after"]
    print("GATE_V2_" + ("OK" if ok else "FAIL"), json.dumps(save_reply)[:200])
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
