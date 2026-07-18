package shadow

import (
	"math"
	"sync"
	"testing"
)

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

func TestSummaryOverlaysTrackRenameDelta(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":         "1012",
				"track_name":       "Track 2",
				"track_type":       "hybrid",
				"user_track_index": 2,
				"is_audio_track":   true,
			},
		},
	})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1012",
		"action":     "property_changed:name",
		"value":      "vocal",
	})

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	if len(tracks) != 1 {
		t.Fatalf("visible tracks = %d, want 1", len(tracks))
	}
	if tracks[0]["track_name"] != "vocal" || tracks[0]["name"] != "vocal" {
		t.Fatalf("renamed track not overlaid: %+v", tracks[0])
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
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"mute":           true,
				"solo":           false,
				"is_armed":       true,
				"gain_db":        -3.5,
				"level_db":       -18.25,
				"left_level_db":  -20.0,
				"right_level_db": -18.25,
			},
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
	if tracks[0]["gain_db"] != -3.5 || tracks[0]["level_db"] != -18.25 || tracks[0]["right_level_db"] != -18.25 {
		t.Fatalf("visible track mixer fields = %+v", tracks[0])
	}
	observability := summary["observability"].(map[string]any)
	profile := observability["profile"].(map[string]any)
	if got := profile["track_count"]; got != 1 {
		t.Fatalf("observability profile track_count = %#v, want 1", got)
	}
}

func TestSummaryTrackGroupAnnotationsDoNotMutateSnapshot(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Kick",
				"track_type":     "hybrid",
				"is_audio_track": true,
			},
		},
		"track_groups": []any{
			map[string]any{
				"group_id":         "drums",
				"name":             "Drums",
				"member_track_ids": []any{"1007"},
			},
		},
	})

	summary := p.Summary()
	track := summary["tracks"].([]map[string]any)[0]
	if got := track["track_group_ids"].([]string); len(got) != 1 || got[0] != "drums" {
		t.Fatalf("summary track groups = %#v, want drums", got)
	}

	snapshot := p.Snapshot()
	engine := snapshot["engine_snapshot"].(map[string]any)
	original := engine["tracks"].([]any)[0].(map[string]any)
	if _, exists := original["track_group_ids"]; exists {
		t.Fatalf("Summary mutated the shadow engine track: %+v", original)
	}
}

func TestSummarySupportsConcurrentReaders(t *testing.T) {
	p := New(nil)
	tracks := make([]any, 0, 61)
	memberIDs := make([]any, 0, 61)
	for i := 0; i < 61; i++ {
		id := string(rune('A' + i))
		tracks = append(tracks, map[string]any{
			"track_id":       id,
			"track_name":     "Track " + id,
			"track_type":     "hybrid",
			"is_audio_track": true,
			"pan":            float64(i) / 61,
		})
		memberIDs = append(memberIDs, id)
	}
	p.Initialize(map[string]any{
		"tracks": tracks,
		"track_groups": []any{
			map[string]any{"group_id": "all", "name": "All", "member_track_ids": memberIDs},
		},
	})

	const readers = 16
	const iterations = 50
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < iterations; j++ {
				if got := p.Summary()["track_count"]; got != 61 {
					t.Errorf("track_count = %#v, want 61", got)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestSummaryOverlaysTrackVolumePluginDelta(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"plugins": []any{
					map[string]any{"plugin_item_id": "1008", "name": "Volume & Pan", "type": "volume"},
				},
			},
		},
	})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1008",
		"action":     "property_changed:volume",
		"value":      math.Exp((-4.9 - 6) / 20),
	})

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	got, ok := tracks[0]["volume_db"].(float64)
	if !ok || math.Abs(got-(-4.9)) > 0.001 {
		t.Fatalf("volume_db = %#v, want -4.9 dB", tracks[0]["volume_db"])
	}
	if tracks[0]["gain_db"] != tracks[0]["volume_db"] || tracks[0]["fader_db"] != tracks[0]["volume_db"] {
		t.Fatalf("volume aliases not overlaid: %+v", tracks[0])
	}
}

func TestSummaryOverlaysTrackPanDelta(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
			},
		},
	})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1007",
		"action":     "property_changed:pan",
		"value":      -0.5,
	})

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	if tracks[0]["pan"] != -0.5 || tracks[0]["pan_value"] != -0.5 {
		t.Fatalf("pan aliases not overlaid: %+v", tracks[0])
	}
}

func TestSummaryOverlaysTrackPanPluginDelta(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status": "ok",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"plugins": []any{
					map[string]any{"plugin_item_id": "1008", "name": "Volume & Pan", "type": "volume"},
				},
			},
		},
	})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1008",
		"action":     "property_changed:pan",
		"value":      -0.7,
	})

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	if tracks[0]["pan"] != -0.7 || tracks[0]["pan_value"] != -0.7 {
		t.Fatalf("plugin pan aliases not overlaid: %+v", tracks[0])
	}
}

func TestInitializeRetainsTrackVolumePluginDeltaForSameSnapshot(t *testing.T) {
	p := New(nil)
	snapshot := map[string]any{
		"status":       "ok",
		"project_path": "D:/song/demo.vit",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"plugins": []any{
					map[string]any{"plugin_item_id": "1008", "name": "Volume & Pan", "type": "volume"},
				},
			},
		},
	}
	p.Initialize(snapshot)
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1008",
		"action":     "property_changed:volume",
		"value":      math.Exp((-8.25 - 6) / 20),
	})
	p.Initialize(snapshot)

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	got, ok := tracks[0]["volume_db"].(float64)
	if !ok || math.Abs(got-(-8.25)) > 0.001 {
		t.Fatalf("retained volume_db = %#v, want -8.25 dB", tracks[0]["volume_db"])
	}
}

func TestInitializeDropsTrackVolumePluginDeltaForDifferentProject(t *testing.T) {
	p := New(nil)
	p.Initialize(map[string]any{
		"status":       "ok",
		"project_path": "D:/song/old.vit",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"plugins": []any{
					map[string]any{"plugin_item_id": "1008", "name": "Volume & Pan", "type": "volume"},
				},
			},
		},
	})
	p.ApplyDelta(map[string]any{
		"type":       "delta_update",
		"seq_id":     float64(1),
		"target_uid": "1008",
		"action":     "property_changed:volume",
		"value":      math.Exp((-8.25 - 6) / 20),
	})
	p.Initialize(map[string]any{
		"status":       "ok",
		"project_path": "D:/song/new.vit",
		"tracks": []any{
			map[string]any{
				"track_id":       "1007",
				"track_name":     "Track 1",
				"track_type":     "hybrid",
				"is_audio_track": true,
				"plugins": []any{
					map[string]any{"plugin_item_id": "1008", "name": "Volume & Pan", "type": "volume"},
				},
			},
		},
	})

	summary := p.Summary()
	tracks := summary["tracks"].([]map[string]any)
	if tracks[0]["volume_db"] != nil {
		t.Fatalf("volume delta leaked across projects: %+v", tracks[0])
	}
}
