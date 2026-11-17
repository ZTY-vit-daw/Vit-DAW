extends Node
var output: FileAccess
var sequence: int = 0
var ready_seen: bool = false
var elapsed: float = 0.0
func emit_observation(kind: String) -> void:
	if output == null:
		return
	sequence += 1
	var record = {"kind":kind,"seq":sequence,"nonce":"61a9d267c302457e82fd57dde33149d8","pid":OS.get_process_id(),"unix_ms":int(Time.get_unix_time_from_system()*1000.0),"ticks_msec":Time.get_ticks_msec(),"ready":int(ready_seen),"window_mode":get_window().mode,"display_mode":DisplayServer.window_get_mode(),"focused":int(get_window().has_focus()),"hwnd":DisplayServer.window_get_native_handle(DisplayServer.WINDOW_HANDLE),"width":get_window().size.x,"height":get_window().size.y,"version":Engine.get_version_info()}
	output.store_line(JSON.stringify(record))
	output.flush()
func _ready() -> void:
	output = FileAccess.open("C:/Users/timoz/.codex/worktrees/win32-restore-403/Vit_DAW/coord/runs/GODOT-MAIN-HWND-OBSERVATION-1/formal_02/synthetic.godot.project/observation.jsonl",FileAccess.WRITE)
	get_window().focus_entered.connect(func(): emit_observation("focus_entered"))
	get_window().focus_exited.connect(func(): emit_observation("focus_exited"))
	get_window().size_changed.connect(func(): emit_observation("size_changed"))
	emit_observation("scene_ready")
func _process(delta: float) -> void:
	if not ready_seen:
		ready_seen = true
		emit_observation("first_process_ready")
	elapsed += delta
	if elapsed >= 0.02:
		elapsed = 0.0
		emit_observation("sample")