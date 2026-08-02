package daw

import (
	"reflect"
	"testing"
)

func TestVisiblePluginRefsIncludesTrackAndRackProjections(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "1132",
				"plugins": []any{
					map[string]any{"plugin_id": "100", "plugin_name": "Track EQ"},
				},
				"rack_nodes": []any{
					map[string]any{"node_id": "200", "display_name": "Rack Node"},
				},
				"rack": map[string]any{
					"nodes": []any{
						map[string]any{"plugin_item_id": "2624", "node_id": "2624", "name": "Pro-Q 3"},
					},
				},
			},
		},
	}

	want := []PluginRef{
		{ID: "100", Name: "Track EQ", TrackID: "1132"},
		{ID: "200", Name: "Rack Node", TrackID: "1132"},
		{ID: "2624", Name: "Pro-Q 3", TrackID: "1132"},
	}
	if got := VisiblePluginRefs(state, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisiblePluginRefs() = %#v, want %#v", got, want)
	}
}

func TestVisiblePluginRefsDeduplicatesSameInstanceAcrossProjections(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "1132",
				"plugins": []any{
					map[string]any{"plugin_id": "2624", "plugin_name": "Pro-Q 3"},
				},
				"rack_nodes": []any{
					map[string]any{"plugin_item_id": "2624", "name": "Pro-Q 3"},
				},
				"rack": map[string]any{
					"nodes": []any{
						map[string]any{"node_id": "2624", "name": "Pro-Q 3"},
					},
				},
			},
		},
	}

	want := []PluginRef{{ID: "2624", Name: "Pro-Q 3", TrackID: "1132"}}
	if got := VisiblePluginRefs(state, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisiblePluginRefs() = %#v, want %#v", got, want)
	}
}

func TestVisiblePluginRefsAppliesExactTrackScope(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "1132",
				"rack": map[string]any{"nodes": []any{
					map[string]any{"plugin_item_id": "2624", "name": "Pro-Q 3"},
				}},
			},
			map[string]any{
				"track_id": "1133",
				"rack_nodes": []any{
					map[string]any{"node_id": "2625", "name": "Other EQ"},
				},
			},
		},
	}

	want := []PluginRef{{ID: "2624", Name: "Pro-Q 3", TrackID: "1132"}}
	if got := VisiblePluginRefs(state, "1132"); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisiblePluginRefs() = %#v, want %#v", got, want)
	}
}

func TestVisiblePluginRefsFallsBackToInstanceIDForName(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "1132",
				"rack_nodes": []any{
					map[string]any{"plugin_item_id": "2624"},
				},
			},
		},
	}

	want := []PluginRef{{ID: "2624", Name: "2624", TrackID: "1132"}}
	if got := VisiblePluginRefs(state, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("VisiblePluginRefs() = %#v, want %#v", got, want)
	}
}
