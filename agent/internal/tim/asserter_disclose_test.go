package tim

import (
	"testing"
)

// TIM-KERNEL-DISCLOSE-1 red-first tests: AS-SR P2 (block size disclosure
// consistency, GAPS Item 3) and AS-PLUGIN P2 (per-instance load state,
// GAPS Item 4). Red state: before the asserter emits these rows and the
// rack reader maps the fields, the helpers below fail.

func TestBuildAssertionBlockSizeStates(t *testing.T) {
	track := func(id string, extra map[string]any) map[string]any {
		row := map[string]any{
			"track_id": id, "track_name": id, "clip_count": 1,
			"primary_clip": map[string]any{"clip_id": "c1", "current_source_path": "/a/b.wav"},
			"acoustic":     map[string]any{"status": "ready", "sample_rate_hz": 48000},
		}
		for k, v := range extra {
			row["acoustic"].(map[string]any)[k] = v
		}
		return row
	}

	// Project-level missing (no audio_settings at all): whole check NE,
	// mirroring the existing sample-rate NE behaviour (保持现状不降级).
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks":      []any{track("t1", nil)},
	}})
	row := assertionResultBy(t, proj, "sample_rate", "block_size", "")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("block_size without audio_settings status = %s, want not_evaluable", row.Status)
	}
	if row.Code != "assert_block_size_not_evaluable" {
		t.Fatalf("code = %s, want assert_block_size_not_evaluable", row.Code)
	}

	// Project-level present, track-level missing (轨级缺): per-track NE rows.
	proj = Build(Input{
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks":      []any{track("t1", nil)},
		},
		AudioSettings: map[string]any{"sample_rate_hz": 48000, "block_size": 512},
	})
	row = assertionResultBy(t, proj, "sample_rate", "block_size", "t1")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("track without block size status = %s, want not_evaluable", row.Status)
	}
	if row.Code != "assert_block_size_not_evaluable" {
		t.Fatalf("code = %s, want assert_block_size_not_evaluable", row.Code)
	}

	// Project-level present but block_size key absent (工程级缺字段): NE.
	proj = Build(Input{
		ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks":      []any{track("t1", nil)},
		},
		AudioSettings: map[string]any{"sample_rate_hz": 48000},
	})
	row = assertionResultBy(t, proj, "sample_rate", "block_size", "")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("block_size key absent status = %s, want not_evaluable", row.Status)
	}

	// Track-level present and equal: pass; differing: fail.
	proj = Build(Input{
		ProjectPackage: map[string]any{
			"track_count": 2,
			"tracks": []any{
				track("t-same", map[string]any{"block_size": 512}),
				track("t-diff", map[string]any{"block_size": 256}),
			},
		},
		AudioSettings: map[string]any{"sample_rate_hz": 48000, "block_size": 512},
	})
	row = assertionResultBy(t, proj, "sample_rate", "block_size", "t-same")
	if row.Status != AssertionStatusPass {
		t.Fatalf("matching block size status = %s, want pass: %#v", row.Status, row)
	}
	if row.Value == nil || *row.Value != 512 || row.Threshold == nil || *row.Threshold != 512 {
		t.Fatalf("pass row values = %v/%v, want 512/512", row.Value, row.Threshold)
	}
	row = assertionResultBy(t, proj, "sample_rate", "block_size", "t-diff")
	if row.Status != AssertionStatusFail {
		t.Fatalf("mismatched block size status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_block_size_mismatch" {
		t.Fatalf("code = %s, want assert_block_size_mismatch", row.Code)
	}
	if row.Value == nil || *row.Value != 256 {
		t.Fatalf("fail row value = %v, want 256", row.Value)
	}
}

func TestBuildAssertionPluginLoadStates(t *testing.T) {
	rackNode := func(id, loadState string, ready bool, withFields bool) map[string]any {
		node := map[string]any{
			"node_id": id, "plugin_path": "/plugins/" + id + ".vst3", "plugin_format": "VST3",
			"enabled": true, "audio_reachable_from_rack_input": true,
		}
		if withFields {
			node["plugin_load_state"] = loadState
			node["plugin_instance_ready"] = ready
		}
		return node
	}
	buildProj := func(nodes ...map[string]any) Projection {
		return Build(Input{ProjectPackage: map[string]any{
			"track_count": 1,
			"tracks": []any{map[string]any{
				"track_id": "t1", "track_name": "T1", "clip_count": 0,
				"rack":     map[string]any{"nodes": nodes},
			}},
		}})
	}

	// failed -> fail row with the dedicated code.
	proj := buildProj(rackNode("n1", "failed", false, true))
	row := assertionResultBy(t, proj, "plugin_legality", "load_state", "t1")
	if row.Status != AssertionStatusFail {
		t.Fatalf("failed load state status = %s, want fail: %#v", row.Status, row)
	}
	if row.Code != "assert_plugin_load_failed" {
		t.Fatalf("code = %s, want assert_plugin_load_failed", row.Code)
	}

	// ready -> pass.
	proj = buildProj(rackNode("n1", "ready", true, true))
	row = assertionResultBy(t, proj, "plugin_legality", "load_state", "t1")
	if row.Status != AssertionStatusPass {
		t.Fatalf("ready load state status = %s, want pass: %#v", row.Status, row)
	}

	// async_pending -> not_evaluable (transient state, never fail, never pass).
	proj = buildProj(rackNode("n1", "async_pending", false, true))
	row = assertionResultBy(t, proj, "plugin_legality", "load_state", "t1")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("async_pending status = %s, want not_evaluable", row.Status)
	}
	if row.Code != "assert_plugin_load_not_evaluable" {
		t.Fatalf("code = %s, want assert_plugin_load_not_evaluable", row.Code)
	}

	// fields not disclosed (older kernel) -> NE, never fabricated.
	proj = buildProj(rackNode("n1", "", false, false))
	row = assertionResultBy(t, proj, "plugin_legality", "load_state", "t1")
	if row.Status != AssertionStatusNotEvaluable {
		t.Fatalf("undisclosed load state status = %s, want not_evaluable", row.Status)
	}
}

func TestRackSummariesReadLoadStateFields(t *testing.T) {
	state := map[string]any{
		"tracks": []any{map[string]any{
			"track_id": "t1",
			"rack": map[string]any{
				"nodes": []any{
					map[string]any{
						"node_id": "n1", "plugin_path": "/p.vst3", "plugin_format": "VST3",
						"plugin_load_state": "failed", "plugin_instance_ready": false,
					},
					map[string]any{
						"node_id": "n2", "plugin_path": "/q.vst3", "plugin_format": "VST3",
					},
				},
			},
		}},
	}
	racks := RackSummariesFromProjectState(state)
	if len(racks) != 1 || len(racks[0].Nodes) != 2 {
		t.Fatalf("rack summary shape wrong: %#v", racks)
	}
	if racks[0].Nodes[0].PluginLoadState != "failed" || racks[0].Nodes[0].PluginInstanceRdy {
		t.Fatalf("n1 load state = %q/%v, want failed/false", racks[0].Nodes[0].PluginLoadState, racks[0].Nodes[0].PluginInstanceRdy)
	}
	if racks[0].Nodes[1].PluginLoadState != "" {
		t.Fatalf("n2 load state = %q, want empty (undisclosed)", racks[0].Nodes[1].PluginLoadState)
	}
}

func TestDiscloseChecksReconcileLoaded(t *testing.T) {
	rows := []AssertionResult{
		{Asserter: "sample_rate", Check: "block_size", Status: AssertionStatusNotEvaluable},
		{Asserter: "plugin_legality", Check: "load_state", Status: AssertionStatusFail},
	}
	kept, limitations := ReconcileLoadedAssertions(rows)
	if len(kept) != 2 {
		t.Fatalf("kept = %d rows, want 2 (new checks registered): %#v", len(kept), kept)
	}
	if len(limitations) != 0 {
		t.Fatalf("unexpected limitations: %v", limitations)
	}
}
