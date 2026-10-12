import json, zmq
ctx = zmq.Context.instance()
def call(payload, tag):
    s = ctx.socket(zmq.REQ); s.setsockopt(zmq.LINGER,0); s.setsockopt(zmq.RCVTIMEO,30000); s.setsockopt(zmq.SNDTIMEO,30000)
    s.connect("tcp://127.0.0.1:5555")
    s.send_string(json.dumps(payload)); raw = s.recv_string(); s.close()
    d = json.loads(raw)
    open(f"/private/tmp/vit-keyfield1-prep/isolated/reply_{tag}.json","w").write(raw)
    mc = d.get("music_context") or {}
    ks = mc.get("key_signature")
    print(f"[{tag}] status={d.get('status')} key={json.dumps(ks,ensure_ascii=False) if ks else None}")
    return d

P2="/private/tmp/vit-keyfield1-prep/isolated/proj/probe2.vit"
call({"cmd":"set_project_key","root":"G","scale":"phrygian"},"10_set_G")
call({"cmd":"save_as_project","file_path":P2},"11_save_as_p2")   # encrypted, contains G/phrygian
call({"cmd":"clear_project"},"12_clear")
call({"cmd":"open_project","file_path":P2},"13_open_p2")          # retriever now set
call({"cmd":"get_project_state"},"14_get_G")
call({"cmd":"set_project_key","root":"B","scale":"dorian"},"15_set_B")  # auto-persist: save() branch
call({"cmd":"get_project_state"},"16_get_B_mem")
call({"cmd":"clear_project"},"17_clear")
call({"cmd":"open_project","file_path":P2},"18_reopen_p2")
call({"cmd":"get_project_state"},"19_get_after_reopen")
