import json, zmq
ctx = zmq.Context.instance()
def call(payload, tag):
    s = ctx.socket(zmq.REQ); s.setsockopt(zmq.LINGER,0); s.setsockopt(zmq.RCVTIMEO,30000); s.setsockopt(zmq.SNDTIMEO,30000)
    s.connect("tcp://127.0.0.1:5555")
    s.send_string(json.dumps(payload)); raw = s.recv_string(); s.close()
    d = json.loads(raw)
    mc = d.get("music_context") or {}
    print(f"[{tag}] status={d.get('status')} tempo={mc.get('tempo_bpm')} key={json.dumps(mc.get('key_signature'),ensure_ascii=False) if mc.get('key_signature') else None}")
    return d

P3="/private/tmp/vit-keyfield1-prep/isolated/proj/probe3.vit"
call({"cmd":"set_tempo","bpm":100},"20_set_tempo_100")
call({"cmd":"save_as_project","file_path":P3},"21_save_as_p3")   # encrypted, tempo=100
call({"cmd":"clear_project"},"22_clear")
call({"cmd":"open_project","file_path":P3},"23_open_p3")          # retriever set
call({"cmd":"get_project_state"},"24_get_100")
call({"cmd":"set_tempo","bpm":111},"25_set_tempo_111")            # auto-persist: save() branch
call({"cmd":"clear_project"},"26_clear")
call({"cmd":"open_project","file_path":P3},"27_reopen_p3")
call({"cmd":"get_project_state"},"28_get_after_reopen")
