package daw

import (
	"fmt"
	"strings"
)

type PluginRef struct {
	ID      string
	Name    string
	TrackID string
}

func ResolveTrackIDForPluginLoad(state map[string]any, requestContext map[string]any) (string, error) {
	tracks := trackRows(state)
	if selectedID := firstNonEmptyText(requestContext, "selected_track_id", "selected_plugin_track_id", "track_id"); selectedID != "" {
		for _, track := range tracks {
			if firstNonEmptyText(track, "track_id", "id") == selectedID {
				return selectedID, nil
			}
		}
	}
	if selectedName := firstNonEmptyText(requestContext, "selected_track_name", "track_name", "name"); selectedName != "" {
		for _, track := range tracks {
			if strings.EqualFold(firstNonEmptyText(track, "track_name", "name"), selectedName) {
				return firstNonEmptyText(track, "track_id", "id"), nil
			}
		}
	}
	switch len(tracks) {
	case 0:
		return "", fmt.Errorf("there are no user-visible tracks")
	case 1:
		return firstNonEmptyText(tracks[0], "track_id", "id"), nil
	default:
		return "", fmt.Errorf("multiple tracks exist; select a track before loading a plugin")
	}
}

func VisiblePluginRefs(state map[string]any, trackScope string) []PluginRef {
	out := []PluginRef{}
	seen := map[string]struct{}{}
	scope := strings.TrimSpace(trackScope)
	for _, track := range trackRows(state) {
		trackID := firstNonEmptyText(track, "track_id", "id")
		if scope != "" && trackID != scope {
			continue
		}

		pluginRows := [][]map[string]any{
			mapRowsValue(track["plugins"]),
			mapRowsValue(track["rack_nodes"]),
		}
		if rack, ok := track["rack"].(map[string]any); ok {
			pluginRows = append(pluginRows, mapRowsValue(rack["nodes"]))
		}

		for _, rows := range pluginRows {
			for _, plugin := range rows {
				id := firstNonEmptyText(plugin, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
				if id == "" {
					continue
				}
				key := trackID + "::" + id
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				name := firstNonEmptyText(plugin, "plugin_name", "name", "display_name", "path")
				if name == "" {
					name = id
				}
				out = append(out, PluginRef{
					ID:      id,
					Name:    name,
					TrackID: trackID,
				})
			}
		}
	}
	return out
}

func trackRows(state map[string]any) []map[string]any {
	if state == nil {
		return nil
	}
	return mapRowsValue(state["tracks"])
}

func mapRowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{rows}
	default:
		return nil
	}
}

func firstNonEmptyText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}
