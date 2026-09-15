package chat

import "testing"

// UI-PLUGIN-COUNT-1: the kernel compact-snapshot track whitelist carries
// plugin rows only inside rack.nodes (VspKernelReference.cpp normaliseRackState
// has no plugins/rack_nodes fields), so the chat UI projection must extract
// plugin rows the way harness visiblePluginRefs does: explicit plugins and
// rack_nodes arrays first, then the rack graph's nodes array.

func uiRackNodeRow(id, name string) map[string]any {
	return map[string]any{
		"plugin_id": id, "plugin_item_id": id, "node_id": id, "item_id": id, "id": id,
		"plugin_name": name, "name": name,
	}
}

func uiSnapshotProjectState(tracks ...map[string]any) map[string]any {
	rows := make([]any, 0, len(tracks))
	for _, track := range tracks {
		rows = append(rows, track)
	}
	return map[string]any{
		"status": "ok", "project_uuid": "project-ui", "project_revision": "rev-ui",
		"tracks": rows,
	}
}

func uiRackPluginIDs(rack map[string]any) []string {
	plugins, _ := rack["plugins"].([]map[string]any)
	ids := make([]string, 0, len(plugins))
	for _, row := range plugins {
		ids = append(ids, firstStringFromMap(row, "plugin_id", "plugin_item_id", "node_id", "item_id", "id"))
	}
	return ids
}

func TestUiPluginRackExtractsPluginRowsFromRackNodes(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass", "track_type": "audio", "is_audio_track": true,
		"rack": map[string]any{
			"rack_item_id": "rack-bass", "node_count": 2,
			"nodes": []any{
				uiRackNodeRow("node-1040", "bx_hybrid V2"),
				uiRackNodeRow("node-1041", "TDR SlickEQ M"),
			},
		},
	})
	state["selected_track_id"] = "track-bass"
	state["selected_plugin_id"] = "node-1041"

	rack := uiPluginRack(state)
	ids := uiRackPluginIDs(rack)
	if len(ids) != 2 || ids[0] != "node-1040" || ids[1] != "node-1041" {
		t.Fatalf("plugin_rack.plugins ids = %v, want node-1040 and node-1041 extracted from rack.nodes", ids)
	}
	selected := false
	for _, row := range rack["plugins"].([]map[string]any) {
		if row["selected"] == true && firstStringFromMap(row, "plugin_id") == "node-1041" {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("rack.nodes-extracted plugin node-1041 was not marked selected: %#v", rack["plugins"])
	}
	if firstStringFromMap(rack["rack"].(map[string]any), "rack_item_id") != "rack-bass" {
		t.Fatalf("plugin_rack.rack graph was not preserved: %#v", rack["rack"])
	}
}

func TestUiPluginRackFallsBackToExplicitPluginRows(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass",
		"plugins": []any{
			map[string]any{"plugin_id": "p-1", "plugin_name": "Legacy EQ"},
			map[string]any{"plugin_id": "p-2", "plugin_name": "Legacy Comp"},
		},
	})
	state["selected_track_id"] = "track-bass"

	rack := uiPluginRack(state)
	if ids := uiRackPluginIDs(rack); len(ids) != 2 {
		t.Fatalf("plugin_rack.plugins ids = %v, want the 2 explicit plugin rows without a rack graph", ids)
	}
}

func TestUiPluginRackEmptyRackNodesKeepExplicitRows(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass",
		"plugins": []any{map[string]any{"plugin_id": "p-1", "plugin_name": "Legacy EQ"}},
		"rack":    map[string]any{"rack_item_id": "rack-bass", "nodes": []any{}, "node_count": 0},
	})
	state["selected_track_id"] = "track-bass"

	rack := uiPluginRack(state)
	if ids := uiRackPluginIDs(rack); len(ids) != 1 || ids[0] != "p-1" {
		t.Fatalf("plugin_rack.plugins ids = %v, want the explicit row kept when rack.nodes is empty", ids)
	}
}

func TestUiPluginRackWithoutPluginsOrRackStaysEmpty(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass",
	})
	state["selected_track_id"] = "track-bass"

	rack := uiPluginRack(state)
	if ids := uiRackPluginIDs(rack); len(ids) != 0 {
		t.Fatalf("plugin_rack.plugins ids = %v, want no rows for a track without plugins or rack", ids)
	}
}

func TestUiPluginRackDeduplicatesRackNodeAgainstExplicitRows(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass",
		"plugins": []any{map[string]any{"plugin_id": "p-1", "plugin_name": "Explicit"}},
		"rack": map[string]any{
			"nodes": []any{
				uiRackNodeRow("p-1", "Same node mirrored in rack"),
				uiRackNodeRow("p-9", "Rack-only node"),
			},
		},
	})
	state["selected_track_id"] = "track-bass"

	rack := uiPluginRack(state)
	if ids := uiRackPluginIDs(rack); len(ids) != 2 {
		t.Fatalf("plugin_rack.plugins ids = %v, want p-1 counted once plus rack-only p-9", ids)
	}
}

func TestCapacityFactsCountPluginsFromRackNodes(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass", "track_type": "audio", "is_audio_track": true,
		"rack": map[string]any{
			"nodes": []any{uiRackNodeRow("node-1", "One"), uiRackNodeRow("node-2", "Two")},
		},
	})
	facts := capacityFactsFromProjectState(state, semanticEntryScopeProjectContext)
	if facts.PluginCount != 2 {
		t.Fatalf("PluginCount = %d, want 2 counted from rack.nodes", facts.PluginCount)
	}
}

func TestUiStateProjectionDerivesTrackPluginCountFromRackNodes(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass", "track_type": "audio",
		"rack": map[string]any{
			"rack_item_id": "rack-bass", "node_count": 2,
			"nodes": []any{
				uiRackNodeRow("node-1040", "bx_hybrid V2"),
				uiRackNodeRow("node-1041", "TDR SlickEQ M"),
			},
		},
	})

	projection := uiStateProjection(state)
	rows := mapRowsFromAny(projection["tracks"])
	if len(rows) != 1 {
		t.Fatalf("projected tracks = %d rows, want 1", len(rows))
	}
	if count := intNumber(rows[0]["plugin_count"]); count != 2 {
		t.Fatalf("projected plugin_count = %d, want 2 derived from rack.nodes", count)
	}
	// The shadow's own row stays authoritative: the count is written to the
	// projected clone only.
	original := mapRowsFromAny(state["tracks"])[0]
	if count := intNumber(original["plugin_count"]); count != 0 {
		t.Fatalf("shadow track row plugin_count = %d, want the original row left untouched", count)
	}
}

func TestUiStateProjectionKeepsExplicitPluginCountFloor(t *testing.T) {
	state := uiSnapshotProjectState(map[string]any{
		"track_id": "track-bass", "track_name": "Bass",
		// legacy state: explicit plugins array plus an explicit count that
		// already exceeds the extractable rows
		"plugins":      []any{map[string]any{"plugin_id": "p-1"}},
		"plugin_count": 3,
	})

	projection := uiStateProjection(state)
	rows := mapRowsFromAny(projection["tracks"])
	if count := intNumber(rows[0]["plugin_count"]); count != 3 {
		t.Fatalf("projected plugin_count = %d, want the explicit count 3 kept as floor", count)
	}
}

func TestUiStateProjectionWithoutTracksReturnsStateUnchanged(t *testing.T) {
	state := map[string]any{"status": "ok", "project_uuid": "project-ui"}
	if projection := uiStateProjection(state); projection["tracks"] != nil {
		t.Fatalf("state without tracks gained a tracks value: %#v", projection["tracks"])
	}
}
