extends SceneTree

## K3.1 ① 运行时取证（临时，不入验收面）：空白右键菜单 popup_on_parent
## 坐标系行为量化。headless 下构造与生产同构的偏移容器 > GraphEdit，
## 分别以「局部坐标 / viewport 坐标」作为菜单锚点触发空白右键链路，
## 读 PopupMenu 弹出后的全局位置，判定 popup_on_parent 实际吃哪个坐标系；
## 同时量化 GraphNode 引擎摆放公式与 _graph_point_to_canvas 的口径差
## （zoom≠1、scroll≠0），校准 :6478 先例换算的输出系。
## 用法：godot --headless --path . -s tools/forensic_popup_coords_k31.gd

const RACK_SCRIPT_PATH := "res://app/rack/Vit_Graph_Rack.gd"
const EVENT_HUB_PATH := "res://vit_dock/core/project_event_hub.gd"

var _rack: Variant = null
var _host: Control = null
var _frame := 0
var _stage_no := 0


func _initialize() -> void:
	root.position = Vector2i(120, 80)
	root.size = Vector2i(1400, 900)
	var flags := FakeDebugFlags.new()
	Engine.register_singleton("VitDebugFlags", flags)
	var stage := Node.new()
	stage.name = "ForensicStage"
	root.add_child(stage)
	var hub_script: Variant = load(EVENT_HUB_PATH)
	var hub: Node = hub_script.new()
	hub.name = "ProbeEventHub"
	stage.add_child(hub)
	var repo := FakeRepo.new()
	stage.add_child(repo)
	var main := FakeMain.new()
	stage.add_child(main)
	var script := GDScript.new()
	script.source_code = "extends \"%s\"\nvar fake_client = null\n\nfunc _vit_rack_client() -> Variant:\n\treturn fake_client\n" % RACK_SCRIPT_PATH
	if script.reload() != OK:
		print("[forensic] FATAL probe rack compile failed")
		quit(2)
		return
	_rack = script.new()
	_rack.name = "ProbeRack"
	_rack.fake_client = FakeRackClient.new()
	_host = Control.new()
	_host.position = Vector2(300, 100)
	_host.size = Vector2(1200, 800)
	stage.add_child(_host)
	_host.add_child(_rack)
	_rack.size = Vector2(800, 600)


func _process(_delta: float) -> bool:
	_frame += 1
	if _frame == 3:
		_stage_a_formula_check()
	elif _frame == 5:
		_stage_b_popup_check()
		quit(0)
		return true
	return false


func _stage_a_formula_check() -> void:
	var rack: Variant = _rack
	print("[forensic] host.position=%s rack.global_position=%s" % [_host.position, rack.get_global_position()])
	rack.zoom = 0.8
	rack.scroll_offset = Vector2(50, 30)
	var gn := GraphNode.new()
	gn.position_offset = Vector2(100, 100)
	rack.add_child(gn)


func _stage_a_readback() -> void:
	pass


func _stage_b_popup_check() -> void:
	var rack: Variant = _rack
	# A 读数（3 帧后引擎已摆放 GraphNode）。
	var kids := (rack as GraphEdit).get_children()
	for k in kids:
		if k is GraphNode:
			var g: GraphNode = k as GraphNode
			print("[forensic] A. zoom=0.8 scroll=(50,30) offset=(100,100) engine node.position=%s  _graph_point_to_canvas=%s" % [
				g.position, rack._graph_point_to_canvas(g.position_offset)])
			var local_click := Vector2(400.0, 300.0)
			var node_local: Vector2 = (local_click - g.position) / rack.zoom
			print("[forensic] A. _node_context_menu_viewport_position(node,(%s))=%s  mouse_local=%s" % [
				node_local, rack._node_context_menu_viewport_position(g, node_local), local_click])
			g.queue_free()
	rack.zoom = 1.0
	rack.scroll_offset = Vector2.ZERO
	# B：第三参=局部 vs viewport，菜单弹出位置。
	var menu_local := Vector2(400.0, 300.0)
	var menu_viewport: Vector2 = rack.get_global_transform() * menu_local
	print("[forensic] B. anchor local=%s viewport-transformed=%s" % [menu_local, menu_viewport])
	_report_menu_position("local-arg", menu_local)
	_report_menu_position("viewport-arg", menu_viewport)
	# C：_open_canvas_action_menu(local) 存的 _canvas_action_menu_pos。
	rack._canvas_action_menu = null
	rack._open_canvas_action_menu(menu_local)
	print("[forensic] C. _open_canvas_action_menu(local) stored _canvas_action_menu_pos=%s" % rack._canvas_action_menu_pos)


func _report_menu_position(tag: String, third_arg: Vector2) -> void:
	var rack: Variant = _rack
	rack._canvas_action_menu = null
	# K3.1 修后真实链路：第三参=local→viewport 换算（与 :1037 相同式）。
	rack._rack_begin_right_pan(third_arg, null, (rack as GraphEdit).get_global_transform() * third_arg)
	rack._rack_finish_right_pan()
	var menu: Node = rack._canvas_action_menu
	if menu == null or not is_instance_valid(menu):
		print("[forensic] B.%s: menu NOT created" % tag)
		return
	print("[forensic] B.%s: local_arg=%s menu.position(screen)=%s expected=root+%s root.position=%s stored_local=%s" % [
		tag, third_arg, menu.position, (rack as GraphEdit).get_global_transform() * third_arg, root.position, rack._canvas_action_menu_pos])


class FakeRepo extends Node:
	func _enter_tree() -> void:
		add_to_group("project_repository")

	func get_tracks() -> Array:
		return [{"id": "1007", "name": "vocals"}]

	func get_track(_track_id: String) -> Dictionary:
		return {"id": "1007", "name": "vocals"}

	func get_rack_nodes_by_track() -> Dictionary:
		return {}

	func get_rack_nodes(_track_id: String) -> Array:
		return []

	func get_send_topology() -> Dictionary:
		return {}

	func get_control_graph() -> Dictionary:
		return {}

	func get_rack_control_macros() -> Dictionary:
		return {"macros": []}


class FakeMain extends Node:
	func _enter_tree() -> void:
		add_to_group("main_ui_controller")

	func get_selected_kernel_track_id() -> String:
		return "1007"


class FakeRackClient extends RefCounted:
	func queue_command(_payload: Dictionary, request_id: String = "", _timeout_ms: int = 5000) -> String:
		return "probe_%s" % request_id


class FakeDebugFlags extends Node:
	var ui_log_enabled := false
