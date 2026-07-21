package tim

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ImportInput struct {
	ObservationID      string
	MixSessionID       string
	CreatedAt          string
	Summary            map[string]any
	Rows               []map[string]any
	AudioSettings      map[string]any
	SourceCapabilities map[string]any
}

func BuildFromImportRows(input ImportInput) Projection {
	tracks := make([]map[string]any, 0, len(input.Rows))
	for i, row := range input.Rows {
		if len(row) == 0 {
			continue
		}
		trackID := firstNonEmpty(fieldString(row, "track_id", "id"), fmt.Sprintf("imported_track_%d", i+1))
		trackName := firstNonEmpty(fieldString(row, "track_name", "name"), fieldString(row, "clip_name", "file_name"), trackID)
		clipID := firstNonEmpty(fieldString(row, "clip_id", "primary_clip_id"), fmt.Sprintf("%s_clip", trackID))
		clipName := firstNonEmpty(fieldString(row, "clip_name", "file_name", "name"), clipID)
		sourcePath := firstNonEmpty(fieldString(row,
			"source_file_path", "current_source_path", "source_path", "file_path",
			"imported_file_path", "copied_file_path", "path",
		))
		length := firstPositiveNumber(row, "length_seconds", "duration_seconds", "duration", "edit_length_seconds")
		primary := map[string]any{
			"clip_id":        clipID,
			"clip_name":      clipName,
			"clip_type":      "audio",
			"length_seconds": length,
		}
		if sourcePath != "" {
			primary["current_source_path"] = sourcePath
			primary["file_path"] = sourcePath
		}
		for _, key := range []string{"start_time_seconds", "start_seconds", "sample_rate_hz", "sample_rate", "bit_depth", "bits_per_sample", "channel_count", "channels"} {
			if value, ok := numberFromAny(row[key]); ok {
				primary[key] = value
			}
		}
		if valid, ok := firstBool(row, "playback_source_valid", "source_valid"); ok {
			primary["playback_source_valid"] = valid
		}
		tracks = append(tracks, map[string]any{
			"track_id":     trackID,
			"track_name":   trackName,
			"track_type":   "audio",
			"clip_count":   1,
			"primary_clip": primary,
			"acoustic": map[string]any{
				"status":          StatusMissing,
				"track_id":        trackID,
				"clip_id":         clipID,
				"primary_clip_id": clipID,
				"reason":          "dad_acoustic_analysis_pending_after_import",
			},
		})
	}
	projectPackage := map[string]any{
		"schema_version": "tim_import_project_packet.v0",
		"status":         "partial",
		"role":           "post_import_technical_integrity_seed",
		"track_count":    len(tracks),
		"tracks":         tracks,
	}
	if len(input.Summary) > 0 {
		projectPackage["import_summary"] = input.Summary
	}
	proj := Build(Input{
		ObservationID:      strings.TrimSpace(input.ObservationID),
		MixSessionID:       strings.TrimSpace(input.MixSessionID),
		CreatedAt:          strings.TrimSpace(input.CreatedAt),
		ProjectPackage:     projectPackage,
		SourceCapabilities: input.SourceCapabilities,
	})
	if reported := firstPositiveInt(input.Summary, "tracks_created", "created_track_count", "created_tracks", "track_count", "tracks_to_create"); reported > len(tracks) {
		proj.Limitations = evidenceRefs(append(proj.Limitations, "tim_import_projection_built_from_returned_track_rows_subset")...)
		proj.LLMContext = BuildLLMContext(proj)
	}
	return proj
}

func ContextProjectionMap(proj Projection) map[string]any {
	data, err := json.Marshal(ContextProjection(proj))
	if err != nil {
		return ContextProjection(proj)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return ContextProjection(proj)
	}
	return out
}
