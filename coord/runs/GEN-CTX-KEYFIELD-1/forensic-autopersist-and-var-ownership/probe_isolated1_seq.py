import json, sys, zmq, time
ctx = zmq.Context.instance()
def call(payload, tag):
    s = ctx.socket(zmq.REQ); s.setsockopt(zmq.LINGER,0); s.setsockopt(zmq.RCVTIMEO,30000); s.setsockopt(zmq.SNDTIMEO,30000)
    s.connect("tcp://127.0.0.1:5555")
    s.send_string(json.dumps(payload))
    raw = s.recv_string(); s.close()
    d = json.loads(raw)
    open(f"/private/tmp/vit-keyfield1-prep/isolated/reply_{tag}.json","w").write(raw)
    mc = d.get("music_context") or {}
    print(f"[{tag}] status={d.get('status')} key={json.dumps(mc.get('key_signature'),ensure_ascii=False)} style={json.dumps(mc.get('style_note'),ensure_ascii=False)[:40]}")
    return d

P="/private/tmp/vit-keyfield1-prep/isolated/proj/probe.vit"
call({"cmd":"new_project"},"01_new")
call({"cmd":"save_as_project","file_path":P},"02_save_as")
call({"cmd":"set_project_key","root":"A","scale":"minor"},"03_set_key")
call({"cmd":"get_project_state"},"04_get_after_set")
call({"cmd":"clear_project"},"05_clear")
call({"cmd":"open_project","file_path":P},"06_reopen")
call({"cmd":"get_project_state"},"07_get_after_reopen")
