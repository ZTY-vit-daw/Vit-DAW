package shadow

import "testing"

func TestShadowInitializeAndDelta(t *testing.T) {
	p := New(nil)
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1007",
		"action":     "property_changed:volume",
		"value":      0.5,
	})
	p.Initialize(map[string]any{
		"status":       "ok",
		"project_path": "D:/demo.tracktionedit",
		"tracks": []any{
			map[string]any{"id": "1007", "name": "Audio 1"},
		},
	})

	snap := p.Snapshot()
	nodes := snap["nodes_by_uid"].(map[string]any)
	node := nodes["1007"].(map[string]any)
	props := node["delta_properties"].(map[string]any)
	if got := props["volume"]; got != 0.5 {
		t.Fatalf("volume delta = %#v, want 0.5", got)
	}
}

func TestShadowTracksOrphanDeltas(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{"tracks": []any{}})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "orphan",
		"action":     "property_changed:name",
		"value":      "X",
	})
	summary := p.Summary()
	if got := summary["orphan_uid_keys"]; got != 1 {
		t.Fatalf("orphan_uid_keys = %#v, want 1", got)
	}
}
