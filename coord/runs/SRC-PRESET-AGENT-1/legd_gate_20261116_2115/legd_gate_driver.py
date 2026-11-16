# 384 SRC-PRESET-AGENT-1 leg D gate driver (deterministic, zero LLM).
# Boots against an already-running XML-boot kernel (5555): add track,
# instantiate Surge XT, save in place (clear-XML branch), report.
import json
import os
import sys
import time

import zmq

REQ_URL = os.environ.get("VIT_KERNEL_REQ", "tcp://127.0.0.1:5555")

SURGE_IDENTIFIER = "VST3-Surge XT-1e1bcfb-190e4fbd"
SURGE_PATH = r"C:\Program Files\Common Files\VST3\Surge Synth Team\Surge XT.vst3\Contents\x86_64-win\Surge XT.vst3"


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
        "schema": "src_preset_legd_gate.v1",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "req_url": REQ_URL,
    }
    out = "legd_gate_report.json"

    def save():
        json.dump(report, open(out, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

    ping = zmq_command({"cmd": "ping"}, 10000)
    report["ping"] = ping
    if ping.get("status") != "ok":
        print("kernel ping failed:", json.dumps(ping)[:200])
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
    if inst.get("status") != "ok":
        inst = zmq_command({"cmd": "instantiate_plugin", "track_id": track_id,
                            "plugin_path": SURGE_PATH})
    report["instantiate_plugin"] = inst
    if inst.get("status") != "ok":
        print("instantiate_plugin failed:", json.dumps(inst)[:300])
        save()
        return 1
    # Let the instance settle so getStateInformation captures real state.
    time.sleep(3)

    params = zmq_command({"cmd": "get_plugin_parameters", "track_id": track_id,
                          "plugin_id": str(inst.get("plugin_id", ""))})
    report["get_plugin_parameters"] = {
        "status": params.get("status"),
        "param_count": len(params.get("binding_targets") or params.get("parameters") or []),
        "template_role": params.get("template_role"),
    }

    save_reply = zmq_command({"cmd": "save_project"})
    report["save_project"] = save_reply
    if save_reply.get("status") != "ok":
        print("save_project failed:", json.dumps(save_reply)[:300])
        save()
        return 1

    report["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%S")
    save()
    print("GATE_DRIVER_OK", json.dumps({"track_id": track_id,
                                        "plugin_id": inst.get("plugin_id"),
                                        "save": save_reply.get("status")}))
    return 0


if __name__ == "__main__":
    sys.exit(main())
