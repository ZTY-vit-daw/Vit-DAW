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

func TestSummaryHidesInternalTracktionTracks(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok",
		"observability": map[string]any{
			"profile": map[string]any{"track_count": 6},
		},
		"tracks": []any{
			map[string]any{"track_id": "1002", "track_name": "Arranger", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1003", "track_name": "Chord", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1004", "track_name": "Marker", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1005", "track_name": "Tempo", "track_type": "track", "is_audio_track": false},
			map[string]any{"track_id": "1006", "track_name": "Master", "track_type": "master", "is_audio_track": false},
			map[string]any{"track_id": "1007", "track_name": "Track 1", "track_type": "hybrid", "is_audio_track": true, "mute": true, "solo": false, "is_armed": true},
		},
	})

	summary := p.Summary()
	if got := summary["engine_track_count"]; got != 6 {
		t.Fatalf("engine_track_count = %#v, want 6", got)
	}
	if got := summary["track_count"]; got != 1 {
		t.Fatalf("track_count = %#v, want 1", got)
	}
	tracks := summary["tracks"].([]map[string]any)
	if len(tracks) != 1 {
		t.Fatalf("visible tracks = %d, want 1", len(tracks))
	}
	if tracks[0]["track_id"] != "1007" || tracks[0]["user_track_index"] != 1 {
		t.Fatalf("visible track = %+v", tracks[0])
	}
	if tracks[0]["mute"] != true || tracks[0]["solo"] != false || tracks[0]["is_armed"] != true {
		t.Fatalf("visible track state flags = %+v", tracks[0])
	}
	observability := summary["observability"].(map[string]any)
	profile := observability["profile"].(map[string]any)
	if got := profile["track_count"]; got != 1 {
		t.Fatalf("observability profile track_count = %#v, want 1", got)
	}
}
