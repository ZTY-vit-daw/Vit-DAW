extends Node
var output: FileAccess
var sequence: int = 0
var ready_seen: bool = false
var elapsed: float = 0.0
var transition_load_code: int = -1
var transition_loaded: bool = false
var transition_stopping: bool = false
func transition_stop(nonce_matches: bool) -> void:
	transition_stopping = true
	var unload_code: int = int(GDExtensionManager.unload_extension("res://test.gdextension"))
	var still_loaded: bool = GDExtensionManager.is_extension_loaded("res://test.gdextension")
	var removed: bool = false
	var shutdown_ok: bool = false
	var native_text: String = FileAccess.get_file_as_string("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/transition_observer.jsonl")
	for line in native_text.split("\n",false):
		var item: Variant = JSON.parse_string(line)
		if typeof(item) != TYPE_DICTIONARY:
			continue
		if str(item.get("nonce","")) != "406statictestnonce000000000000001" or int(item.get("pid",0)) != OS.get_process_id() or int(item.get("hwnd",0)) != DisplayServer.window_get_native_handle(DisplayServer.WINDOW_HANDLE):
			continue
		if str(item.get("kind","")) == "removed_verified":
			removed = bool(item.get("ok",false)) and not bool(item.get("installed",true))
		if str(item.get("kind","")) == "shutdown":
			shutdown_ok = bool(item.get("ok",false)) and int(item.get("failures",-1)) == 0 and int(item.get("dropped",-1)) == 0
	var ack = {"kind":"observer_unloaded","nonce":"406statictestnonce000000000000001","pid":OS.get_process_id(),"hwnd":DisplayServer.window_get_native_handle(DisplayServer.WINDOW_HANDLE),"nonce_matches":nonce_matches,"unload_result":unload_code,"still_loaded":still_loaded,"removed_verified":removed,"shutdown":shutdown_ok}
	var ack_file: FileAccess = FileAccess.open("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/unload_ack.json",FileAccess.WRITE)
	if ack_file != null:
		ack_file.store_line(JSON.stringify(ack))
		ack_file.flush()
		ack_file.close()
	emit_observation("observer_unloaded")
	OS.set_environment("VIT_TEST_TRANSITION_NONCE","")
	OS.set_environment("VIT_TEST_TRANSITION_OUTPUT","")
	var success: bool = nonce_matches and unload_code == 0 and not still_loaded and removed and shutdown_ok and ack_file != null
	get_tree().quit(0 if success else 2)
func emit_observation(kind: String) -> void:
	if output == null:
		return
	sequence += 1
	var record = {"kind":kind,"seq":sequence,"nonce":"406statictestnonce000000000000001","pid":OS.get_process_id(),"unix_ms":int(Time.get_unix_time_from_system()*1000.0),"ticks_msec":Time.get_ticks_msec(),"ready":int(ready_seen),"window_mode":get_window().mode,"display_mode":DisplayServer.window_get_mode(),"focused":int(get_window().has_focus()),"hwnd":DisplayServer.window_get_native_handle(DisplayServer.WINDOW_HANDLE),"width":get_window().size.x,"height":get_window().size.y,"version":Engine.get_version_info(),"observer_load_result":transition_load_code,"observer_loaded":transition_loaded}
	output.store_line(JSON.stringify(record))
	output.flush()
func _ready() -> void:
	output = FileAccess.open("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/observation.jsonl",FileAccess.WRITE)
	OS.set_environment("VIT_TEST_TRANSITION_NONCE","406statictestnonce000000000000001")
	OS.set_environment("VIT_TEST_TRANSITION_OUTPUT","C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/transition_observer.jsonl")
	transition_load_code = int(GDExtensionManager.load_extension("res://test.gdextension"))
	transition_loaded = GDExtensionManager.is_extension_loaded("res://test.gdextension")
	emit_observation("observer_load_result")
	get_window().focus_entered.connect(func(): emit_observation("focus_entered"))
	get_window().focus_exited.connect(func(): emit_observation("focus_exited"))
	get_window().size_changed.connect(func(): emit_observation("size_changed"))
	emit_observation("scene_ready")
func _process(delta: float) -> void:
	if not transition_stopping and FileAccess.file_exists("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/stop.request"):
		var request: String = FileAccess.get_file_as_string("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/WINDOW-TRANSITION-OBSERVER-1/assembly_static/generated_fixture/stop.request").strip_edges()
		transition_stop(request == "406statictestnonce000000000000001")
		return
	if not ready_seen:
		ready_seen = true
		emit_observation("first_process_ready")
	elapsed += delta
	if elapsed >= 0.02:
		elapsed = 0.0
		emit_observation("sample")