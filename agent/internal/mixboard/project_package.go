package mixboard

import (
	"fmt"
	"sort"
	"strings"
)

func buildProjectPackage(state map[string]any, target TargetRef, scope ListenScope, snap featureSnapshot) map[string]any {
	tracks := buildTrackSummaries(state, target, scope, snap)
	status := "ready"
	if len(tracks) == 0 {
		status = "partial"
	}
	activeCount := 0
	acousticTrackCount := 0
	activeAcousticTrackCount := 0
	for _, track := range tracks {
		if cleanAnyString(track["active_state"]) == "active" {
			activeCount++
		}
		acoustic, _ := track["acoustic"].(map[string]any)
		if len(acoustic) > 0 {
			acousticTrackCount++
			if featureStatus(acoustic) == "ready" {
				activeAcousticTrackCount++
			}
		}
	}
	loudnessRanking := rankTrackSummaries(tracks, "rms_dbfs")
	peakRanking := rankTrackSummaries(tracks, "peak_dbfs")
	peakRiskRanking := peakRiskRanking(tracks)
	headroomRisk := headroomRiskRanking(tracks)
	bandOccupancy := projectBandOccupancy(tracks)
	stereoSpread := projectStereoSpread(tracks)
	levelDistribution := projectLevelDistribution(tracks)
	conflictCandidates := projectConflictCandidates(tracks)
	return map[string]any{
		"schema_version":                "mixboard_project_packet.v1",
		"status":                        status,
		"role":                          "project_context",
		"summary":                       projectPackageSummary(tracks, activeCount, acousticTrackCount, activeAcousticTrackCount),
		"duration_seconds":              projectDuration(state),
		"track_count":                   len(tracks),
		"active_track_count":            activeCount,
		"acoustic_track_count":          acousticTrackCount,
		"active_acoustic_track_count":   activeAcousticTrackCount,
		"focus_ids":                     scope.Source.FocusIDs,
		"context_ids":                   scope.Source.ContextIDs,
		"tracks":                        tracks,
		"loudness_ranking":              loudnessRanking,
		"level_ranking":                 rankTrackSummaries(tracks, "level_db"),
		"peak_ranking":                  peakRanking,
		"peak_risk_ranking":             peakRiskRanking,
		"headroom_risk":                 headroomRisk,
		"project_band_occupancy":        bandOccupancy,
		"project_stereo_spread":         stereoSpread,
		"level_distribution":            levelDistribution,
		"conflict_candidates":           conflictCandidates,
		"likely_first_attention_target": projectFirstAttentionTarget(tracks, loudnessRanking, peakRanking, headroomRisk),
		"relationship_inputs":           relationshipInputStatus(tracks),
		"analysis_boundary": map[string]any{
			"agent_side_lightweight": []string{"loudness_ranking", "level_ranking", "peak_ranking", "headroom_risk", "project_band_occupancy", "project_stereo_spread", "level_distribution", "conflict_candidates", "likely_first_attention_target"},
			"kernel_deferred":        []string{"project_contrast_analyzer", "lufs_analysis", "masking_analysis", "reference_match", "post_fx_shadow_render"},
		},
		"limitations": projectPackageLimitations(tracks),
	}
}

func buildTrackSummaries(state map[string]any, target TargetRef, scope ListenScope, snap featureSnapshot) []map[string]any {
	rows := anySlice(state["tracks"])
	out := make([]map[string]any, 0, len(rows))
	focus := stringSet(scope.Source.FocusIDs)
	if strings.TrimSpace(target.ID) != "" {
		focus[target.ID] = true
	}
	bandRows := bandEnergyRowsByTrack(snap)
	stereoRows := stereoRelationRowsByTrack(snap)
	loudnessRows := loudnessRowsByTrack(snap)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		if len(row) == 0 {
			continue
		}
		id := firstNonEmpty(cleanAnyString(row["track_id"]), cleanAnyString(row["id"]))
		name := firstNonEmpty(cleanAnyString(row["track_name"]), cleanAnyString(row["name"]), id)
		if id == "" && name == "" {
			continue
		}
		trackType := cleanAnyString(row["track_type"])
		clips := anySlice(firstPresent(row, "clips", "clip_summaries"))
		primaryClip := primaryTrackClip(clips)
		plugins := anySlice(firstPresent(row, "plugins", "rack_nodes", "plugin_chain"))
		summary := map[string]any{
			"track_id":             id,
			"name":                 name,
			"track_name":           name,
			"user_label":           name,
			"track_type":           trackType,
			"user_track_index":     firstNonNil(row["user_track_index"], row["index"]),
			"role_guess":           guessTrackRole(name, trackType),
			"active_state":         trackActiveState(row, clips),
			"clip_count":           len(clips),
			"plugin_count":         len(plugins),
			"plugin_chain_summary": pluginChainSummary(plugins),
			"selected":             boolFromAny(row["selected"]),
			"focused":              focus[id],
			"mute":                 boolFromAny(firstPresent(row, "mute", "muted", "is_muted")),
			"solo":                 boolFromAny(firstPresent(row, "solo", "is_solo")),
			"is_armed":             boolFromAny(firstPresent(row, "is_armed", "armed")),
			"stereo_position":      stereoPositionSummary(row),
		}
		if compactClip := compactPrimaryClip(primaryClip); len(compactClip) > 0 {
			summary["primary_clip"] = compactClip
		}
		copyFirstNumber(summary, row, "volume_db", "volume_db", "gain_db", "fader_db", "db")
		copyFirstNumber(summary, row, "pan", "pan", "pan_value", "balance")
		copyFirstNumber(summary, row, "level_db", "level_db", "rms_dbfs", "rms_db")
		copyFirstNumber(summary, row, "peak_dbfs", "peak_dbfs", "peak_db")
		copyFirstNumber(summary, row, "headroom_db", "headroom_db")
		copyFirstNumber(summary, row, "left_level_db", "left_level_db")
		copyFirstNumber(summary, row, "right_level_db", "right_level_db")
		acoustic := trackAcousticPackage(id, clips, acousticRowForTrack(id, clips, snap))
		if len(acoustic) > 0 {
			summary["acoustic"] = acoustic
			applyAcousticMetricsToTrack(summary, acoustic)
		}
		summary["band_energy"] = projectTrackBandEnergy(id, bandRows[id])
		summary["stereo_relation"] = projectTrackStereoRelation(id, stereoRows[id])
		summary["loudness"] = projectTrackLoudness(id, loudnessRows[id])
		applyLoudnessMetricsToTrack(summary, mapValue(summary["loudness"]))
		out = append(out, summary)
	}
	return out
}

func bandEnergyRowsByTrack(snap featureSnapshot) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range snap.BandEnergySummaries {
		addBestFeatureRowByTrack(out, row)
	}
	addBestFeatureRowByTrack(out, snap.BandEnergySummary)
	return out
}

func stereoRelationRowsByTrack(snap featureSnapshot) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range snap.StereoRelationSummaries {
		addBestFeatureRowByTrack(out, row)
	}
	addBestFeatureRowByTrack(out, snap.StereoRelationSummary)
	return out
}

func loudnessRowsByTrack(snap featureSnapshot) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range snap.LoudnessSummaries {
		addBestFeatureRowByTrack(out, row)
	}
	addBestFeatureRowByTrack(out, snap.LoudnessSummary)
	return out
}

func addBestFeatureRowByTrack(out map[string]map[string]any, row map[string]any) {
	trackID := cleanAnyString(row["track_id"])
	if trackID == "" {
		return
	}
	existing := out[trackID]
	if len(existing) == 0 || acousticRowPriority(row) > acousticRowPriority(existing) {
		out[trackID] = row
	}
}

func projectTrackBandEnergy(trackID string, row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing", "track_id": trackID, "reason": "band_energy_summary_not_available"}
	}
	out := buildBandEnergySummary(row)
	if cleanAnyString(out["track_id"]) == "" {
		out["track_id"] = trackID
	}
	return out
}

func projectTrackStereoRelation(trackID string, row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing", "track_id": trackID, "reason": "stereo_relation_summary_not_available"}
	}
	out := buildStereoRelationSummary(row)
	if cleanAnyString(out["track_id"]) == "" {
		out["track_id"] = trackID
	}
	return out
}

func projectTrackLoudness(trackID string, row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": "missing", "track_id": trackID, "reason": "loudness_summary_not_available"}
	}
	out := buildLoudnessSummary(row)
	if cleanAnyString(out["track_id"]) == "" {
		out["track_id"] = trackID
	}
	return out
}

func projectPackageSummary(tracks []map[string]any, activeCount, acousticTrackCount, activeAcousticTrackCount int) map[string]any {
	roleCounts := map[string]int{}
	for _, track := range tracks {
		role := firstNonEmpty(cleanAnyString(track["role_guess"]), "unknown")
		roleCounts[role]++
	}
	return map[string]any{
		"track_count":                 len(tracks),
		"active_track_count":          activeCount,
		"acoustic_track_count":        acousticTrackCount,
		"active_acoustic_track_count": activeAcousticTrackCount,
		"role_counts":                 roleCounts,
	}
}

func projectPackageLimitations(tracks []map[string]any) []string {
	hasLevel := false
	hasPeak := false
	hasReadyAcoustic := false
	hasRequestedAcoustic := false
	hasBlockedAcoustic := false
	hasBand := false
	hasStereo := false
	hasLoudness := false
	for _, track := range tracks {
		if _, ok := numberField(track, "level_db"); ok {
			hasLevel = true
		}
		if _, ok := numberField(track, "peak_dbfs"); ok {
			hasPeak = true
		}
		acoustic, _ := track["acoustic"].(map[string]any)
		switch featureStatus(acoustic) {
		case "ready":
			hasReadyAcoustic = true
		case "requested":
			hasRequestedAcoustic = true
		case "blocked":
			hasBlockedAcoustic = true
		}
		if projectFeatureUsable(mapValue(track["band_energy"])) {
			hasBand = true
		}
		if projectFeatureUsable(mapValue(track["stereo_relation"])) {
			hasStereo = true
		}
		if projectFeatureUsable(mapValue(track["loudness"])) {
			hasLoudness = true
		}
	}
	limits := []string{
		"project_rankings_are_agent_side_lightweight_o1",
		"kernel_project_contrast_analyzer_deferred_o3",
		"lufs_analysis_deferred_phase_5",
		"masking_analysis_deferred_phase_5",
		"reference_match_deferred_phase_5",
		"post_fx_probe_unavailable_phase_4_1",
	}
	if !hasReadyAcoustic {
		limits = append(limits, "per_track_waveform_acoustic_missing")
	}
	if hasRequestedAcoustic {
		limits = append(limits, "per_track_waveform_acoustic_pending")
	}
	if hasBlockedAcoustic {
		limits = append(limits, "per_track_waveform_acoustic_blocked")
	}
	if !hasLevel {
		limits = append(limits, "shadow_track_level_db_missing")
	}
	if !hasPeak {
		limits = append(limits, "shadow_track_peak_dbfs_missing")
	}
	if !hasBand {
		limits = append(limits, "kernel_l3_band_energy_summary_missing")
	}
	if !hasStereo {
		limits = append(limits, "kernel_l3_stereo_relation_summary_missing")
	}
	if !hasLoudness {
		limits = append(limits, "kernel_l3_loudness_summary_missing")
	}
	return limits
}

func relationshipInputStatus(tracks []map[string]any) map[string]any {
	withLevel := 0
	withPeak := 0
	withAcoustic := 0
	withTimeEnergy := 0
	withBand := 0
	withStereo := 0
	withLoudness := 0
	for _, track := range tracks {
		if _, ok := numberField(track, "level_db"); ok {
			withLevel++
		}
		if _, ok := numberField(track, "peak_dbfs"); ok {
			withPeak++
		}
		acoustic, _ := track["acoustic"].(map[string]any)
		if featureStatus(acoustic) == "ready" {
			withAcoustic++
			if len(mapRowsAny(acoustic["time_segments"])) > 0 {
				withTimeEnergy++
			}
		}
		if projectFeatureUsable(mapValue(track["band_energy"])) {
			withBand++
		}
		if projectFeatureUsable(mapValue(track["stereo_relation"])) {
			withStereo++
		}
		if projectFeatureUsable(mapValue(track["loudness"])) {
			withLoudness++
		}
	}
	status := "partial"
	if len(tracks) > 0 && (withAcoustic == len(tracks) || withLevel == len(tracks) || withPeak == len(tracks) || (withBand == len(tracks) && withStereo == len(tracks) && withLoudness == len(tracks))) {
		status = "ready"
	}
	return map[string]any{
		"status":                       status,
		"tracks_with_level_db":         withLevel,
		"tracks_with_peak_db":          withPeak,
		"tracks_with_acoustic":         withAcoustic,
		"tracks_with_time_energy":      withTimeEnergy,
		"tracks_with_band_summary":     withBand,
		"tracks_with_stereo_summary":   withStereo,
		"tracks_with_loudness_summary": withLoudness,
		"track_count":                  len(tracks),
		"lightweight_relationships":    []string{"loudness_comparison", "peak_headroom_risk", "time_energy_overlap", "band_occupancy", "stereo_spread", "conflict_candidates", "focus_vs_project_average"},
	}
}

func acousticRowsByTrack(snap featureSnapshot) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range snap.TrackWaveformEnvelopes {
		trackID := cleanAnyString(row["track_id"])
		if trackID == "" {
			continue
		}
		existing := out[trackID]
		if len(existing) == 0 || acousticRowPriority(row) > acousticRowPriority(existing) {
			out[trackID] = row
		}
	}
	if trackID := cleanAnyString(snap.WaveformEnvelope["track_id"]); trackID != "" {
		if len(out[trackID]) == 0 || acousticRowPriority(snap.WaveformEnvelope) > acousticRowPriority(out[trackID]) {
			out[trackID] = snap.WaveformEnvelope
		}
	}
	return out
}

func acousticRowForTrack(trackID string, clips []any, snap featureSnapshot) map[string]any {
	primaryClip := primaryTrackClip(clips)
	primaryClipID := cleanAnyString(firstPresent(primaryClip, "clip_id", "id", "item_id"))
	var best map[string]any
	bestScore := -1
	consider := func(row map[string]any) {
		if len(row) == 0 || cleanAnyString(row["track_id"]) != trackID {
			return
		}
		score := acousticRowPriority(row)
		rowClipID := cleanAnyString(row["clip_id"])
		if primaryClipID != "" {
			if rowClipID != primaryClipID {
				score -= 20
			} else {
				score += 20
			}
		}
		if score > bestScore {
			best = row
			bestScore = score
		}
	}
	for _, row := range snap.TrackWaveformEnvelopes {
		consider(row)
	}
	consider(snap.WaveformEnvelope)
	if bestScore < 0 {
		return nil
	}
	return best
}

func acousticRowPriority(row map[string]any) int {
	switch featureStatus(row) {
	case "ready":
		return 4
	case "partial":
		return 3
	case "requested":
		return 2
	case "blocked":
		return 1
	default:
		return 0
	}
}

func trackAcousticPackage(trackID string, clips []any, row map[string]any) map[string]any {
	primaryClip := primaryTrackClip(clips)
	if len(row) == 0 {
		out := map[string]any{
			"status":   "missing",
			"track_id": trackID,
		}
		if clipID := cleanAnyString(firstPresent(primaryClip, "clip_id", "id", "item_id")); clipID != "" {
			out["clip_id"] = clipID
			out["primary_clip_id"] = clipID
			out["reason"] = "waveform_envelope_not_available"
		} else {
			out["reason"] = "primary_audio_clip_missing"
		}
		return out
	}
	metrics := buildWaveformMetrics(row)
	out := map[string]any{
		"status":          featureStatus(row),
		"track_id":        firstNonEmpty(cleanAnyString(row["track_id"]), trackID),
		"clip_id":         firstNonEmpty(cleanAnyString(row["clip_id"]), cleanAnyString(firstPresent(primaryClip, "clip_id", "id", "item_id"))),
		"primary_clip_id": firstNonEmpty(cleanAnyString(row["clip_id"]), cleanAnyString(firstPresent(primaryClip, "clip_id", "id", "item_id"))),
		"source":          cleanAnyString(row["source"]),
		"rms":             metrics["rms"],
		"peak_abs":        metrics["peak_abs"],
		"rms_dbfs":        metrics["rms_dbfs"],
		"peak_dbfs":       metrics["peak_dbfs"],
		"headroom_db":     metrics["headroom_db"],
		"crest_db":        metrics["crest_db"],
	}
	if name := firstNonEmpty(cleanAnyString(row["clip_name"]), cleanAnyString(row["name"]), cleanAnyString(firstPresent(primaryClip, "clip_name", "name"))); name != "" {
		out["primary_clip_name"] = name
	}
	if sourcePath := firstNonEmpty(cleanAnyString(row["file_path"]), cleanAnyString(row["source_path"]), cleanAnyString(firstPresent(primaryClip, "file_path", "source_path", "current_source_path"))); sourcePath != "" {
		out["source_path"] = sourcePath
	}
	for _, key := range []string{"request_id", "reason", "file_path", "updated_at", "float_count", "tile_count_seen", "tile_count_expected", "total_duration"} {
		if value, ok := row[key]; ok && value != nil {
			out[key] = value
		}
	}
	if segments := waveformTimeSegments(row); len(segments) > 0 {
		out["time_segments"] = capRows(segments, 12)
		out["time_energy_status"] = "ready"
	} else {
		out["time_energy_status"] = "missing"
	}
	return out
}

func primaryTrackClip(clips []any) map[string]any {
	var best map[string]any
	bestLength := -1.0
	for _, raw := range clips {
		clip, _ := raw.(map[string]any)
		if len(clip) == 0 {
			continue
		}
		length := firstPositiveFloat(clip, "length_seconds", "duration_seconds", "duration")
		if best == nil || length > bestLength {
			best = clip
			bestLength = length
		}
	}
	return best
}

func compactPrimaryClip(clip map[string]any) map[string]any {
	if len(clip) == 0 {
		return nil
	}
	out := map[string]any{}
	if id := firstNonEmpty(cleanAnyString(firstPresent(clip, "clip_id", "id", "item_id"))); id != "" {
		out["clip_id"] = id
	}
	if name := firstNonEmpty(cleanAnyString(firstPresent(clip, "clip_name", "name"))); name != "" {
		out["clip_name"] = name
	}
	if clipType := cleanAnyString(clip["clip_type"]); clipType != "" {
		out["clip_type"] = clipType
	}
	for _, key := range []string{"file_path", "source_path", "current_source_path"} {
		if value := cleanAnyString(clip[key]); value != "" {
			out[key] = value
		}
	}
	for _, key := range []string{"start_seconds", "end_seconds", "length_seconds", "duration_seconds", "offset_in_source_seconds", "sample_rate_hz", "sample_rate", "bit_depth", "bits_per_sample", "channel_count", "channels"} {
		if value, ok := numberField(clip, key); ok {
			out[key] = round3(value)
		}
	}
	if value, ok := clip["playback_source_valid"].(bool); ok {
		out["playback_source_valid"] = value
	}
	return out
}

func applyAcousticMetricsToTrack(track, acoustic map[string]any) {
	if len(track) == 0 || len(acoustic) == 0 || featureStatus(acoustic) != "ready" {
		return
	}
	if _, ok := numberField(track, "rms_dbfs"); !ok {
		copyNumberValue(track, acoustic, "rms_dbfs", "rms_dbfs")
	}
	if _, ok := numberField(track, "peak_dbfs"); !ok {
		copyNumberValue(track, acoustic, "peak_dbfs", "peak_dbfs")
	}
	if _, ok := numberField(track, "headroom_db"); !ok {
		copyNumberValue(track, acoustic, "headroom_db", "headroom_db")
	}
	if _, ok := numberField(track, "crest_db"); !ok {
		copyNumberValue(track, acoustic, "crest_db", "crest_db")
	}
	if _, ok := numberField(track, "level_db"); !ok {
		copyNumberValue(track, acoustic, "level_db", "rms_dbfs")
	}
}

func applyLoudnessMetricsToTrack(track, loudness map[string]any) {
	if len(track) == 0 || len(loudness) == 0 {
		return
	}
	switch featureStatus(loudness) {
	case "ready", "partial", "suspect":
	default:
		return
	}
	if _, ok := numberField(track, "rms_dbfs"); !ok {
		copyNumberValue(track, loudness, "rms_dbfs", "rms_dbfs")
	}
	if _, ok := numberField(track, "peak_dbfs"); !ok {
		copyNumberValue(track, loudness, "peak_dbfs", "peak_dbfs")
	}
	if _, ok := numberField(track, "crest_db"); !ok {
		copyNumberValue(track, loudness, "crest_db", "crest_db")
	}
	if _, ok := numberField(track, "level_db"); !ok {
		if value, ok := numberField(loudness, "integrated_lufs"); ok {
			track["level_db"] = round3(value)
		} else {
			copyNumberValue(track, loudness, "level_db", "rms_dbfs")
		}
	}
}

func copyNumberValue(dst, src map[string]any, outKey, inKey string) {
	if value, ok := numberField(src, inKey); ok {
		dst[outKey] = round3(value)
	}
}

func projectBandOccupancy(tracks []map[string]any) map[string]any {
	bandIDs := []string{"sub", "bass", "low_mid", "mid", "presence", "air"}
	out := map[string]any{
		"status":      "missing",
		"track_count": len(tracks),
		"bands":       map[string]any{},
	}
	if len(tracks) == 0 {
		out["reason"] = "no_tracks"
		return out
	}
	bandsOut := map[string]any{}
	readyTrackIDs := map[string]bool{}
	for _, bandID := range bandIDs {
		trackRows := []map[string]any{}
		sum := 0.0
		weight := 0.0
		for _, track := range tracks {
			bandSummary := mapValue(track["band_energy"])
			if !projectFeatureUsable(bandSummary) {
				continue
			}
			bands := mapValue(bandSummary["bands"])
			band := mapValue(bands[bandID])
			if len(band) == 0 {
				continue
			}
			unit, ok := numberField(band, "unit_energy")
			if !ok {
				continue
			}
			if trackID := cleanAnyString(track["track_id"]); trackID != "" {
				readyTrackIDs[trackID] = true
			}
			sum += unit
			weight++
			trackRows = append(trackRows, map[string]any{
				"track_id":    track["track_id"],
				"name":        track["name"],
				"role_guess":  track["role_guess"],
				"unit_energy": round3(unit),
				"energy_db":   firstNonNil(band["energy_db"], approxDbfs(unit)),
			})
		}
		sort.Slice(trackRows, func(i, j int) bool {
			return numberFromMap(trackRows[i], "unit_energy") > numberFromMap(trackRows[j], "unit_energy")
		})
		row := map[string]any{"status": "missing", "track_count": len(trackRows)}
		if weight > 0 {
			row["status"] = "ready"
			row["average_unit_energy"] = round3(sum / weight)
			row["dominant_tracks"] = capMapRows(trackRows, 3)
		}
		bandsOut[bandID] = row
	}
	out["bands"] = bandsOut
	if len(readyTrackIDs) > 0 {
		out["status"] = "ready"
		out["tracks_with_band_summary"] = len(readyTrackIDs)
	}
	return out
}

func projectStereoSpread(tracks []map[string]any) map[string]any {
	if len(tracks) <= 1 {
		return map[string]any{"status": "not_applicable_single_track", "track_count": len(tracks)}
	}
	rows := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		if !projectFeatureUsable(stereo) {
			continue
		}
		row := map[string]any{
			"track_id":   track["track_id"],
			"name":       track["name"],
			"role_guess": track["role_guess"],
			"status":     featureStatus(stereo),
		}
		if value, ok := numberField(stereo, "balance_db"); ok {
			row["balance_db"] = round3(value)
		}
		if value, ok := numberField(stereo, "correlation_estimate"); ok {
			row["correlation_estimate"] = round3(value)
		}
		if state := cleanAnyString(stereo["balance_state"]); state != "" {
			row["balance_state"] = state
		}
		if state := cleanAnyString(stereo["correlation_state"]); state != "" {
			row["correlation_state"] = state
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return map[string]any{"status": "missing", "track_count": len(tracks), "reason": "stereo_relation_summary_not_available"}
	}
	sort.Slice(rows, func(i, j int) bool {
		return absFloat(numberFromMap(rows[i], "balance_db")) > absFloat(numberFromMap(rows[j], "balance_db"))
	})
	return map[string]any{
		"status":                     "ready",
		"track_count":                len(tracks),
		"tracks_with_stereo_summary": len(rows),
		"widest_balance_tracks":      capMapRows(rows, 5),
	}
}

func projectLevelDistribution(tracks []map[string]any) map[string]any {
	rows := []map[string]any{}
	for _, track := range tracks {
		value, ok := numberField(track, "level_db")
		metric := "level_db"
		if !ok {
			value, ok = numberField(track, "rms_dbfs")
			metric = "rms_dbfs"
		}
		if !ok {
			loudness := mapValue(track["loudness"])
			value, ok = numberField(loudness, "integrated_lufs")
			metric = "integrated_lufs"
		}
		if !ok {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":   track["track_id"],
			"name":       track["name"],
			"role_guess": track["role_guess"],
			"metric":     metric,
			"value":      round3(value),
		})
	}
	if len(rows) == 0 {
		return map[string]any{"status": "missing", "track_count": len(tracks), "reason": "level_or_loudness_summary_not_available"}
	}
	sort.Slice(rows, func(i, j int) bool {
		return numberFromMap(rows[i], "value") > numberFromMap(rows[j], "value")
	})
	highest := numberFromMap(rows[0], "value")
	lowest := numberFromMap(rows[len(rows)-1], "value")
	return map[string]any{
		"status":            "ready",
		"track_count":       len(tracks),
		"tracks_with_level": len(rows),
		"highest":           rows[0],
		"lowest":            rows[len(rows)-1],
		"spread_db":         round3(highest - lowest),
		"ranking":           capMapRows(rows, 8),
	}
}

func projectConflictCandidates(tracks []map[string]any) map[string]any {
	if len(tracks) <= 1 {
		return map[string]any{"status": "not_applicable_single_track", "track_count": len(tracks), "candidates": []map[string]any{}}
	}
	candidates := []map[string]any{}
	candidates = append(candidates, bandConflictCandidates(tracks, "bass", "low_end_overlap")...)
	candidates = append(candidates, bandConflictCandidates(tracks, "low_mid", "low_mid_masking_candidate")...)
	if center := centerCongestionCandidates(tracks); len(center) > 0 {
		candidates = append(candidates, center...)
	}
	if phase := phaseRiskCandidates(tracks); len(phase) > 0 {
		candidates = append(candidates, phase...)
	}
	status := "ready"
	if len(candidates) == 0 {
		status = "clear"
	}
	return map[string]any{
		"status":      status,
		"track_count": len(tracks),
		"candidates":  capMapRows(candidates, 12),
	}
}

func bandConflictCandidates(tracks []map[string]any, bandID, kind string) []map[string]any {
	rows := []map[string]any{}
	for _, track := range tracks {
		bandSummary := mapValue(track["band_energy"])
		if !projectFeatureUsable(bandSummary) {
			continue
		}
		band := mapValue(mapValue(bandSummary["bands"])[bandID])
		unit, ok := numberField(band, "unit_energy")
		if !ok || unit <= 0 {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":    track["track_id"],
			"name":        track["name"],
			"role_guess":  track["role_guess"],
			"unit_energy": unit,
			"energy_db":   firstNonNil(band["energy_db"], approxDbfs(unit)),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return numberFromMap(rows[i], "unit_energy") > numberFromMap(rows[j], "unit_energy")
	})
	if len(rows) < 2 {
		return nil
	}
	return []map[string]any{{
		"type":         kind,
		"status":       "candidate",
		"band":         bandID,
		"tracks":       capMapRows(rows, 3),
		"evidence_ref": "project_package.project_band_occupancy." + bandID,
	}}
}

func centerCongestionCandidates(tracks []map[string]any) []map[string]any {
	rows := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		if !projectFeatureUsable(stereo) {
			continue
		}
		balance, _ := numberField(stereo, "balance_db")
		correlation, hasCorrelation := numberField(stereo, "correlation_estimate")
		if absFloat(balance) > 1.5 || (hasCorrelation && correlation < 0.35) {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":             track["track_id"],
			"name":                 track["name"],
			"role_guess":           track["role_guess"],
			"balance_db":           round3(balance),
			"correlation_estimate": round3(correlation),
		})
	}
	if len(rows) < 2 {
		return nil
	}
	return []map[string]any{{
		"type":         "center_congestion_candidate",
		"status":       "candidate",
		"tracks":       capMapRows(rows, 5),
		"evidence_ref": "project_package.project_stereo_spread",
	}}
}

func phaseRiskCandidates(tracks []map[string]any) []map[string]any {
	rows := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		if !projectFeatureUsable(stereo) {
			continue
		}
		correlation, hasCorrelation := numberField(stereo, "correlation_estimate")
		negative, hasNegative := numberField(stereo, "phase_negative_ratio")
		if (!hasCorrelation || correlation >= 0.25) && (!hasNegative || negative <= 0.1) {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":             track["track_id"],
			"name":                 track["name"],
			"role_guess":           track["role_guess"],
			"correlation_estimate": round3(correlation),
			"phase_negative_ratio": round3(negative),
		})
	}
	if len(rows) == 0 {
		return nil
	}
	return []map[string]any{{
		"type":         "phase_risk_candidate",
		"status":       "candidate",
		"tracks":       capMapRows(rows, 5),
		"evidence_ref": "project_package.project_stereo_spread",
	}}
}

func projectFeatureUsable(row map[string]any) bool {
	switch featureStatus(row) {
	case "ready", "partial", "suspect":
		return true
	default:
		return false
	}
}

func capMapRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func projectFirstAttentionTarget(tracks, loudnessRanking, peakRanking, headroomRisk []map[string]any) map[string]any {
	if risk := firstProjectHighHeadroomRisk(headroomRisk); len(risk) > 0 {
		return map[string]any{"reason": "headroom_risk", "track": risk}
	}
	if len(peakRanking) > 0 {
		if peak := numberFromMap(peakRanking[0], "value"); peak >= -1.0 {
			return map[string]any{"reason": "peak_near_full_scale", "track": peakRanking[0]}
		}
	}
	for _, track := range tracks {
		if boolFromAny(track["focused"]) {
			return map[string]any{"reason": "user_focus", "track": compactKeys(track, []string{"track_id", "name", "role_guess", "rms_dbfs", "peak_dbfs", "headroom_db", "focused"})}
		}
	}
	if len(loudnessRanking) > 0 {
		return map[string]any{"reason": "loudest_known_track", "track": loudnessRanking[0]}
	}
	return map[string]any{"status": "missing"}
}

func firstProjectHighHeadroomRisk(rows []map[string]any) map[string]any {
	for _, row := range rows {
		switch strings.ToLower(cleanAnyString(row["risk"])) {
		case "critical", "high":
			return row
		}
	}
	return nil
}

func rankTrackSummaries(tracks []map[string]any, metric string) []map[string]any {
	rows := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		value, ok := numberField(track, metric)
		if !ok {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":   track["track_id"],
			"name":       track["name"],
			"role_guess": track["role_guess"],
			"metric":     metric,
			"value":      round3(value),
			"focused":    track["focused"],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return numberFromMap(rows[i], "value") > numberFromMap(rows[j], "value")
	})
	for i := range rows {
		rows[i]["rank"] = i + 1
	}
	return rows
}

func peakRiskRanking(tracks []map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		peak, ok := numberField(track, "peak_dbfs")
		if !ok {
			continue
		}
		headroom, hasHeadroom := numberField(track, "headroom_db")
		if !hasHeadroom {
			headroom = 0 - peak
		}
		rows = append(rows, map[string]any{
			"track_id":    track["track_id"],
			"name":        track["name"],
			"role_guess":  track["role_guess"],
			"peak_dbfs":   round3(peak),
			"headroom_db": round3(headroom),
			"risk":        headroomRiskLabel(headroom),
			"focused":     track["focused"],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return numberFromMap(rows[i], "peak_dbfs") > numberFromMap(rows[j], "peak_dbfs")
	})
	for i := range rows {
		rows[i]["rank"] = i + 1
	}
	return rows
}

func headroomRiskRanking(tracks []map[string]any) []map[string]any {
	rows := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		headroom, ok := numberField(track, "headroom_db")
		if !ok {
			continue
		}
		rows = append(rows, map[string]any{
			"track_id":    track["track_id"],
			"name":        track["name"],
			"role_guess":  track["role_guess"],
			"headroom_db": round3(headroom),
			"risk":        headroomRiskLabel(headroom),
			"focused":     track["focused"],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return numberFromMap(rows[i], "headroom_db") < numberFromMap(rows[j], "headroom_db")
	})
	for i := range rows {
		rows[i]["rank"] = i + 1
	}
	return rows
}

func copyFirstNumber(dst, src map[string]any, outKey string, inKeys ...string) {
	for _, key := range inKeys {
		if value, ok := numberField(src, key); ok {
			dst[outKey] = round3(value)
			return
		}
	}
}

func numberField(row map[string]any, key string) (float64, bool) {
	if row == nil {
		return 0, false
	}
	value, ok := row[key]
	if !ok || value == nil {
		return 0, false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return 0, false
	}
	switch value.(type) {
	case int, int64, float32, float64:
		return numberFromMap(row, key), true
	default:
		parsed := numberFromMap(row, key)
		if parsed != 0 || text == "0" || text == "0.0" || text == "0.00" {
			return parsed, true
		}
	}
	return 0, false
}

func boolFromAny(value any) bool {
	switch x := value.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "enabled":
			return true
		}
	case int:
		return x != 0
	case float64:
		return x != 0
	}
	return false
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func trackActiveState(row map[string]any, clips []any) string {
	if boolFromAny(firstPresent(row, "mute", "muted", "is_muted")) {
		return "muted"
	}
	if level, ok := numberField(row, "level_db"); ok && level > -90 {
		return "active"
	}
	if len(clips) == 0 {
		return "empty"
	}
	return "active"
}

func stereoPositionSummary(row map[string]any) map[string]any {
	out := map[string]any{}
	if pan, ok := numberField(row, "pan"); ok {
		out["pan"] = round3(pan)
	}
	left, hasLeft := numberField(row, "left_level_db")
	right, hasRight := numberField(row, "right_level_db")
	if hasLeft && hasRight {
		out["left_level_db"] = round3(left)
		out["right_level_db"] = round3(right)
		out["balance_db"] = round3(left - right)
	}
	if len(out) == 0 {
		out["status"] = "missing"
	}
	return out
}

func pluginChainSummary(plugins []any) []map[string]any {
	out := make([]map[string]any, 0, len(plugins))
	for i, raw := range plugins {
		row, _ := raw.(map[string]any)
		if len(row) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"slot":        i + 1,
			"plugin_id":   firstNonEmpty(cleanAnyString(row["plugin_id"]), cleanAnyString(row["plugin_item_id"]), cleanAnyString(row["id"])),
			"name":        firstNonEmpty(cleanAnyString(row["plugin_name"]), cleanAnyString(row["name"])),
			"type":        firstNonEmpty(cleanAnyString(row["type"]), cleanAnyString(row["plugin_type"])),
			"enabled":     !boolFromAny(row["bypassed"]),
			"role_guess":  guessPluginRole(row),
			"source_kind": "shadow",
		})
	}
	return out
}

func guessPluginRole(row map[string]any) string {
	text := strings.ToLower(strings.Join([]string{
		cleanAnyString(row["plugin_name"]),
		cleanAnyString(row["name"]),
		cleanAnyString(row["type"]),
	}, " "))
	switch {
	case strings.Contains(text, "eq") || strings.Contains(text, "equal"):
		return "eq"
	case strings.Contains(text, "comp") || strings.Contains(text, "limit"):
		return "dynamics"
	case strings.Contains(text, "reverb") || strings.Contains(text, "delay"):
		return "space"
	case strings.Contains(text, "pan") || strings.Contains(text, "volume") || strings.Contains(text, "gain"):
		return "utility"
	default:
		return "unknown"
	}
}

func guessTrackRole(name, trackType string) string {
	text := strings.ToLower(strings.TrimSpace(name + " " + trackType))
	switch {
	case strings.Contains(text, "master"):
		return "master"
	case containsAnyText(text, "vocal", "vox", "lead vox", "lead vocal", "voice", "主唱", "人声", "歌声", "声乐"):
		return "vocal"
	case containsAnyText(text, "kick", "bd", "bass drum", "底鼓", "大鼓"):
		return "kick"
	case containsAnyText(text, "snare", "军鼓", "小鼓"):
		return "snare"
	case containsAnyText(text, "drum", "perc", "percussion", "鼓", "打击"):
		return "drums"
	case containsAnyText(text, "bass", "low end", "sub", "贝斯", "低音", "低频"):
		return "bass"
	case containsAnyText(text, "guitar", "gtr", "吉他"):
		return "guitar"
	case containsAnyText(text, "synth", "pad", "keys", "piano", "keyboard", "合成器", "钢琴", "键盘", "铺底"):
		return "keys_or_synth"
	case strings.Contains(text, "bus") || strings.Contains(text, "aux") || strings.Contains(text, "总线") || strings.Contains(text, "母线"):
		return "bus"
	default:
		return "unknown"
	}
}

func containsAnyText(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func headroomRiskLabel(headroom float64) string {
	switch {
	case headroom <= 1:
		return "high"
	case headroom <= 3:
		return "medium"
	default:
		return "low"
	}
}
