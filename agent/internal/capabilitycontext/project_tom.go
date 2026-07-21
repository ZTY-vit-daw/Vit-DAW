package capabilitycontext

import (
	"strings"
	"time"

	"vit-daw-agent/internal/tom"
)

// BuildProjectTOMProjection deterministically reconstructs the role/organization
// projection declared by the B2/B3 Context Manifests from the authoritative
// current project snapshot. It is context assembly, not a mutation proposal:
// no folder organization is applied and no pending authority is created.
func BuildProjectTOMProjection(projectState map[string]any, observationID, mixSessionID string) map[string]any {
	rows := projectTOMRows(projectState)
	if len(rows) == 0 {
		return nil
	}
	projection := tom.BuildFromImportRows(tom.ImportInput{
		ObservationID: strings.TrimSpace(observationID),
		MixSessionID:  strings.TrimSpace(mixSessionID),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Summary: map[string]any{
			"tracks_created": len(rows),
			"track_count":    len(rows),
			"source":         "capability_runtime_v1_ccb_project_state",
		},
		Rows: rows,
	})
	return tom.ContextProjectionMap(projection)
}

func projectTOMRows(projectState map[string]any) []map[string]any {
	tracks := rowsValue(projectState["tracks"])
	if len(tracks) == 0 {
		tracks = rowsValue(mapValue(projectState["daw_state_summary"])["tracks"])
	}
	if len(tracks) == 0 {
		tracks = rowsValue(projectState["visible_tracks"])
	}
	out := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		if projectTOMTrackIsContainer(track) {
			continue
		}
		trackID := firstText(track, "track_id", "id")
		trackName := firstText(track, "track_name", "name", "user_label")
		if trackID == "" || trackName == "" {
			continue
		}
		row := map[string]any{
			"track_id":   trackID,
			"track_name": trackName,
		}
		clips := rowsValue(track["clips"])
		if len(clips) > 0 {
			clip := clips[0]
			copyProjectTOMText(row, "clip_id", clip, "clip_id", "id")
			copyProjectTOMText(row, "clip_name", clip, "clip_name", "name", "file_name")
			copyProjectTOMText(row, "source_file_path", clip, "current_source_path", "source_file_path", "source_path", "file_path")
			copyProjectTOMNumber(row, "duration_seconds", clip, "length_seconds", "duration_seconds")
			copyProjectTOMNumber(row, "start_seconds", clip, "start_seconds", "start_time_seconds")
		}
		copyProjectTOMNumber(row, "channel_count", track, "channel_count", "channels", "source_channel_count")
		out = append(out, row)
	}
	return out
}

func projectTOMTrackIsContainer(track map[string]any) bool {
	if boolValue(track["is_folder_track"]) || boolValue(track["is_folder_container"]) || boolValue(track["can_contain_child_tracks"]) {
		return true
	}
	trackType := strings.ToLower(firstText(track, "track_type", "type", "kind"))
	return strings.Contains(trackType, "folder")
}

func copyProjectTOMText(dst map[string]any, dstKey string, source map[string]any, keys ...string) {
	if value := firstText(source, keys...); value != "" {
		dst[dstKey] = value
	}
}

func copyProjectTOMNumber(dst map[string]any, dstKey string, source map[string]any, keys ...string) {
	if value, ok := firstNumber(source, keys...); ok && value != nil {
		dst[dstKey] = *value
	}
}
