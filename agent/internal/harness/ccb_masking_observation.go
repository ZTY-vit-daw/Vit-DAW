package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/masking"
	"vit-daw-agent/internal/mixboard"
)

func (h *Harness) prepareCCBMaskingObservation(ctx context.Context, cmd map[string]any, sessionID string) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("masking_analysis_kernel_unavailable")
	}
	state := h.UserStateSummary(ctx)
	if refresh := h.refreshShadowWithStatus(ctx, "ccb_masking_observation"); boolValueDefault(refresh["shadow_refreshed"], false) {
		state = h.UserStateSummary(ctx)
	}
	trackIDs, trackNames := maskingObservationTracks(state)
	if len(trackIDs) < 2 {
		return nil, fmt.Errorf("masking_analysis_requires_at_least_two_audio_tracks")
	}
	duration := maskingObservationDuration(state)
	if duration <= 0 {
		return nil, fmt.Errorf("masking_analysis_requires_a_known_project_range")
	}
	collected, err := h.CollectL2RenderProbeBatch(ctx, L2RenderProbeBatchRequest{
		SessionID:            strings.TrimSpace(sessionID),
		GoalText:             firstNonEmpty(firstString(cmd, "goal_text", "goal"), "CCB masking-risk observation"),
		TrackIDs:             trackIDs,
		TapPoint:             "track_post_fader",
		FeatureSnapshotPath:  mixboard.FeatureSnapshotPath(cmd),
		StartSeconds:         0,
		EndSeconds:           duration,
		TailSeconds:          0,
		RequireMaskingFrames: true,
	})
	if err != nil {
		return nil, fmt.Errorf("masking_analysis_not_ready_after_bounded_wait: %w", err)
	}
	tracks := make([]masking.TrackEvidence, 0, len(collected.Rows))
	for _, row := range collected.Rows {
		trackID := firstString(row, "track_id")
		evidence, ok := masking.TrackEvidenceFromProbeRow(row, trackNames[trackID])
		if ok {
			tracks = append(tracks, evidence)
		}
	}
	project := mapAnyFromAny(state["project"])
	measurement := masking.Build(masking.BuildInput{
		ProjectUUID:      firstNonEmpty(firstString(state, "project_uuid", "project_id"), firstString(project, "project_uuid", "uuid", "id")),
		ProjectRevision:  firstNonEmpty(firstString(state, "project_revision", "revision"), firstString(project, "revision")),
		ProjectStateHash: firstNonEmpty(firstString(state, "snapshot_hash", "project_state_hash"), firstString(project, "snapshot_hash")),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		Tracks:           tracks,
	})
	measurementMap := maskingMeasurementMap(measurement)
	if !strings.EqualFold(firstString(measurementMap, "status"), "ready") {
		return measurementMap, fmt.Errorf("masking_measurement_not_ready: %s", strings.Join(stringSliceFromAny(measurementMap["limitations"]), ","))
	}
	writeMixboardMaskingMeasurementSnapshot(cmd, measurementMap)
	return measurementMap, nil
}

func maskingObservationTracks(state map[string]any) ([]string, map[string]string) {
	ids := []string{}
	names := map[string]string{}
	for _, track := range mapRowsFromAny(state["tracks"]) {
		trackID := firstNonEmpty(firstString(track, "track_id"), firstString(track, "id"))
		if trackID == "" {
			continue
		}
		trackType := strings.ToLower(firstString(track, "track_type", "type"))
		if strings.Contains(trackType, "folder") || strings.Contains(trackType, "container") || boolValueDefault(track["is_folder_track"], false) {
			continue
		}
		clips := mapRowsFromAny(track["clips"])
		if len(clips) == 0 {
			clips = mapRowsFromAny(track["clip_summaries"])
		}
		if len(clips) == 0 && !boolValueDefault(track["is_audio_track"], false) && !boolValueDefault(track["is_audio"], false) {
			continue
		}
		ids = append(ids, trackID)
		names[trackID] = firstNonEmpty(firstString(track, "track_name", "name"), trackID)
	}
	return uniqueSortedProbeTrackIDs(ids), names
}

func maskingObservationDuration(state map[string]any) float64 {
	project := mapAnyFromAny(state["project"])
	if duration := firstPositiveNumber(state, "duration_seconds", "duration", "length_seconds"); duration > 0 {
		return duration
	}
	if duration := firstPositiveNumber(project, "duration_seconds", "duration", "length_seconds"); duration > 0 {
		return duration
	}
	end := 0.0
	for _, track := range mapRowsFromAny(state["tracks"]) {
		clips := append(mapRowsFromAny(track["clips"]), mapRowsFromAny(track["clip_summaries"])...)
		for _, clip := range clips {
			start := firstPositiveNumber(clip, "start_seconds", "start", "position_seconds")
			length := firstPositiveNumber(clip, "duration_seconds", "length_seconds", "duration")
			if candidate := start + length; candidate > end {
				end = candidate
			}
		}
	}
	return end
}

func maskingMeasurementMap(measurement masking.Measurement) map[string]any {
	data, _ := json.Marshal(measurement)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func writeMixboardMaskingMeasurementSnapshot(cmd, measurement map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["masking_measurement"] = measurement
	writeMixboardFeatureSnapshotFile(path, existing, nil)
}
