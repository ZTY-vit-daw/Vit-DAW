extends SceneTree

var _ran := false

func _process(_delta: float) -> bool:
	if _ran:
		return false
	_ran = true
	_run_probe()
	return false


func _run_probe() -> void:
	var telemetry: Node = root.get_node_or_null("/root/VitTelemetryManager")
	if telemetry == null:
		printerr("l2 probe: /root/VitTelemetryManager autoload was not found")
		quit(2)
		return
	var source_path := OS.get_environment("VIT_DAW_L2_PROBE_SOURCE").strip_edges()
	if source_path.is_empty():
		source_path = "D:/Vit_DAW/test_100hz_10s.wav"
	var request := {
		"schema_version": "mixboard_feature_request.v1",
		"request_id": "l2_probe_req",
		"project_id": "current",
		"resolved_target": {
			"track_id": "track_1",
			"clip_id": "clip_1",
			"source_path": source_path,
			"file_path": source_path,
			"source_revision": "rev_prepared",
			"clip_revision": "clip_rev_prepared",
			"duration_seconds": 10.0,
		},
		"requested_features": [
			{"feature_type": "waveform_envelope", "request_id": "l2_probe_req", "track_id": "track_1", "clip_id": "clip_1"},
			{"feature_type": "spectral_field", "request_id": "l2_probe_req", "track_id": "track_1", "clip_id": "clip_1"},
		],
		"updated_at": Time.get_datetime_string_from_system(true),
	}
	telemetry.call("begin_mixboard_feature_request", request)
	telemetry.set("_mixboard_band_snapshot_last_ms", -1000000)
	telemetry.set("_last_transport_playing", true)

	var bin_count := 128
	var left := PackedFloat32Array()
	var right := PackedFloat32Array()
	var phase := PackedFloat32Array()
	var weight := PackedFloat32Array()
	left.resize(bin_count)
	right.resize(bin_count)
	phase.resize(bin_count)
	weight.resize(bin_count)
	for index in range(bin_count):
		left[index] = 0.01
		right[index] = 0.01
		phase[index] = 0.5
		weight[index] = 0.02
	var bass_start := int(telemetry.call("_mixboard_spectrum_bin_from_hz", 75.0, bin_count))
	var bass_end := int(telemetry.call("_mixboard_spectrum_bin_from_hz", 130.0, bin_count))
	for index in range(bass_start, bass_end + 1):
		left[index] = 0.82
		right[index] = 0.78
		phase[index] = 0.54
		weight[index] = 0.9

	telemetry.call("_update_mixboard_band_energy_from_levels", [
		{
			"track_id": "track_1",
			"spectrum_left": left,
			"spectrum_right": right,
			"spectrum_phase": phase,
			"spectrum_weight": weight,
			"left_level_db": -9.0,
			"right_level_db": -9.2,
		}
	])
	_write_ready_snapshot_artifact(telemetry)
	telemetry.set("_mixboard_band_snapshot_last_ms", -1000000)
	telemetry.set("_last_transport_playing", false)
	var zero_left := PackedFloat32Array()
	var zero_right := PackedFloat32Array()
	zero_left.resize(bin_count)
	zero_right.resize(bin_count)
	telemetry.call("_update_mixboard_band_energy_from_levels", [
		{
			"track_id": "track_1",
			"spectrum_left": zero_left,
			"spectrum_right": zero_right,
			"left_level_db": -100.0,
			"right_level_db": -100.0,
		}
	])
	var second_source_path := OS.get_environment("VIT_DAW_L2_PROBE_SECOND_SOURCE").strip_edges()
	if second_source_path.is_empty():
		second_source_path = source_path + ".second"
	var request_2 := request.duplicate(true)
	request_2["request_id"] = "l2_probe_req_second_material"
	request_2["resolved_target"] = {
		"track_id": "track_1",
		"clip_id": "clip_2",
		"source_path": second_source_path,
		"file_path": second_source_path,
		"source_revision": "rev_second_material",
		"clip_revision": "clip_rev_second_material",
		"duration_seconds": 10.0,
	}
	request_2["requested_features"] = [
		{"feature_type": "waveform_envelope", "request_id": "l2_probe_req_second_material", "track_id": "track_1", "clip_id": "clip_2"},
		{"feature_type": "spectral_field", "request_id": "l2_probe_req_second_material", "track_id": "track_1", "clip_id": "clip_2"},
	]
	telemetry.call("begin_mixboard_feature_request", request_2)
	telemetry.set("_mixboard_band_snapshot_last_ms", -1000000)
	telemetry.set("_last_transport_playing", false)
	telemetry.call("_update_mixboard_band_energy_from_levels", [
		{
			"track_id": "track_1",
			"spectrum_left": zero_left,
			"spectrum_right": zero_right,
			"left_level_db": -100.0,
			"right_level_db": -100.0,
		}
	])
	quit(0)


func _write_ready_snapshot_artifact(telemetry: Node) -> void:
	var output_path := OS.get_environment("VIT_DAW_L2_READY_SNAPSHOT_PATH").strip_edges()
	if output_path.is_empty():
		return
	var snapshot: Variant = telemetry.get("_mixboard_feature_snapshot")
	if typeof(snapshot) != TYPE_DICTIONARY:
		return
	DirAccess.make_dir_recursive_absolute(output_path.get_base_dir())
	var file := FileAccess.open(output_path, FileAccess.WRITE)
	if file == null:
		printerr("l2 probe: cannot write ready snapshot: %s" % output_path)
		quit(3)
		return
	file.store_string(JSON.stringify(snapshot, "\t"))
	file.close()
