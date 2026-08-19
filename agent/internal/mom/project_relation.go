package mom

import (
	"fmt"
	"math"
	"sort"
)

var mixBands = []string{"sub", "bass", "low_mid", "mid", "presence", "air"}

func buildIntentPolicy(intent string) IntentPolicy {
	out := IntentPolicy{
		Name:           intent,
		RequiredLayers: IntentRequiredLayers(intent),
		OptionalLayers: IntentOptionalLayers(intent),
		OutputContract: []string{
			"compact_projection_only",
			"evidence_refs_required",
			"quality_summary_required",
			"do_not_include_raw_package",
			"no_waveform_time_segments_in_llm_context",
			"do_not_use_suspect_stale_missing_as_action_evidence",
		},
	}
	if intent == IntentProjectMaskingObservation {
		out.OutputContract = append(out.OutputContract,
			"directional_improvement_risk_candidates_only",
			"no_deterministic_masking_or_mix_defect_claims",
		)
	}
	return out
}

func buildProjectMixProfile(input Input) ProjectMixProfile {
	tracks := projectTrackRows(input)
	trackCount := len(tracks)
	refs := evidenceRefs(observationRef(input), "mix.read:project.tracks.summary", "mix.read:project.relationship_inputs", "mix.derive:rank_tracks")
	compared := compactComparedTracks(tracks)
	level := levelOverview(tracks)
	if projected := mapValue(input.ProjectPackage["level_distribution"]); len(projected) > 0 && StatusFromSource(text(projected["status"])) != StatusMissing {
		level = projected
	}
	projectedOccupancy := bandOccupancyFromProjectPackage(mapValue(input.ProjectPackage["project_band_occupancy"]))
	dominant := projectDominantBands(tracks)
	if len(projectedOccupancy) > 0 {
		dominant = dominantBandsFromOccupancy(projectedOccupancy)
	}
	stereo := stereoOverview(tracks)
	if projected := stereoDistributionFromProjectPackage(mapValue(input.ProjectPackage["project_stereo_spread"]), nil); len(projected) > 0 && StatusFromSource(text(projected["status"])) != StatusMissing {
		stereo = projected
	}
	limitations := stringsFromAny(input.ProjectPackage["limitations"])
	riskTags := profileRiskTags(level, dominant, stereo)
	status := profileStatus(trackCount, compared, limitations)
	return ProjectMixProfile{
		Status:         status,
		Freshness:      FreshnessForStatus(status),
		TrackCount:     trackCount,
		ComparedTracks: compared,
		LevelOverview:  level,
		DominantBands:  dominant,
		BandTendency:   bandTendency(dominant),
		StereoOverview: stereo,
		RiskTags:       riskTags,
		Limitations:    limitations,
		EvidenceRefs:   refs,
	}
}

func buildMultitrackRelation(input Input, profile ProjectMixProfile) MultitrackRelation {
	tracks := projectTrackRows(input)
	trackCount := len(tracks)
	refs := evidenceRefs(observationRef(input), "mix.read:project.tracks.summary", "mix.read:project.relationship_inputs", "mix.derive:rank_tracks")
	if trackCount <= 1 {
		return MultitrackRelation{
			Status:         "not_applicable_single_track",
			Freshness:      "fresh",
			TrackCount:     trackCount,
			ComparedTracks: profile.ComparedTracks,
			Limitations:    []string{"not_applicable_single_track", "do_not_infer_multitrack_conflict_from_single_track"},
			EvidenceRefs:   refs,
		}
	}
	if hasProjectRelationProjection(input.ProjectPackage) {
		return buildMultitrackRelationFromProjectPackage(input, profile, tracks, refs)
	}
	missing := missingRelationTracks(tracks)
	level := levelDistribution(tracks)
	occupancy := bandOccupancy(tracks)
	conflicts := bandConflictCandidates(occupancy)
	stereo := stereoDistribution(tracks)
	phase := phaseRiskTracks(tracks)
	status := StatusReady
	limitations := []string{}
	if len(missing) > 0 {
		status = StatusPartial
		limitations = append(limitations, "some_tracks_missing_band_or_stereo_evidence")
	}
	if len(occupancy) == 0 && len(level) == 0 {
		status = StatusMissing
		limitations = append(limitations, "no_comparable_multitrack_metrics")
	}
	if relationHasSuspect(tracks) {
		status = StatusSuspect
		limitations = append(limitations, "one_or_more_tracks_have_suspect_relation_evidence")
	}
	return MultitrackRelation{
		Status:                 status,
		Freshness:              FreshnessForStatus(status),
		TrackCount:             trackCount,
		ComparedTracks:         profile.ComparedTracks,
		MissingTracks:          missing,
		LevelDistribution:      level,
		BandOccupancy:          occupancy,
		BandConflictCandidates: conflicts,
		StereoDistribution:     stereo,
		PhaseRiskTracks:        phase,
		Limitations:            limitations,
		EvidenceRefs:           refs,
	}
}

func hasProjectRelationProjection(project map[string]any) bool {
	for _, key := range []string{"project_band_occupancy", "project_stereo_spread", "level_distribution", "conflict_candidates"} {
		row := mapValue(project[key])
		status := StatusFromSource(text(row["status"]))
		if len(row) > 0 && status != StatusMissing && status != StatusDeferred {
			return true
		}
	}
	return false
}

func buildMultitrackRelationFromProjectPackage(input Input, profile ProjectMixProfile, tracks []map[string]any, refs []string) MultitrackRelation {
	trackCount := len(tracks)
	missing := missingRelationTracks(tracks)
	level := mapValue(input.ProjectPackage["level_distribution"])
	occupancy := bandOccupancyFromProjectPackage(mapValue(input.ProjectPackage["project_band_occupancy"]))
	conflicts := conflictCandidatesFromProjectPackage(mapValue(input.ProjectPackage["conflict_candidates"]))
	stereo := stereoDistributionFromProjectPackage(mapValue(input.ProjectPackage["project_stereo_spread"]), conflicts)
	phase := phaseRiskTracksFromProjectPackage(stereo, conflicts)
	status := rollupProjectRelationProjectionStatus(level, occupancy, stereo, conflicts, missing)
	limitations := []string{"project_relation_from_dad_v1_2_projection", "llm_must_explain_local_projection_not_recalculate"}
	if len(missing) > 0 {
		limitations = append(limitations, "some_tracks_missing_band_or_stereo_evidence")
	}
	if status == StatusSuspect {
		limitations = append(limitations, "one_or_more_project_relation_inputs_are_suspect_or_stale")
	}
	return MultitrackRelation{
		Status:                 status,
		Freshness:              FreshnessForStatus(status),
		TrackCount:             trackCount,
		ComparedTracks:         profile.ComparedTracks,
		MissingTracks:          missing,
		LevelDistribution:      level,
		BandOccupancy:          occupancy,
		BandConflictCandidates: conflicts,
		StereoDistribution:     stereo,
		PhaseRiskTracks:        phase,
		Limitations:            limitations,
		EvidenceRefs: evidenceRefs(append(refs,
			"project_package.project_band_occupancy",
			"project_package.project_stereo_spread",
			"project_package.level_distribution",
			"project_package.conflict_candidates",
		)...),
	}
}

func rollupProjectRelationProjectionStatus(level map[string]any, occupancy []map[string]any, stereo map[string]any, conflicts []map[string]any, missing []map[string]any) string {
	statuses := []string{}
	if len(level) > 0 {
		statuses = append(statuses, StatusFromSource(text(level["status"])))
	}
	for _, row := range occupancy {
		statuses = append(statuses, StatusFromSource(text(row["status"])))
	}
	if len(stereo) > 0 {
		statuses = append(statuses, StatusFromSource(text(stereo["status"])))
	}
	for _, row := range conflicts {
		statuses = append(statuses, StatusFromSource(text(row["status"])))
	}
	if len(statuses) == 0 {
		return StatusMissing
	}
	overall := rollupStatusFromList(statuses...)
	if len(missing) > 0 && overall == StatusReady {
		return StatusPartial
	}
	return overall
}

func bandOccupancyFromProjectPackage(row map[string]any) []map[string]any {
	if len(row) == 0 || StatusFromSource(text(row["status"])) == StatusMissing {
		return nil
	}
	bands := mapValue(row["bands"])
	out := []map[string]any{}
	for _, bandID := range mixBands {
		band := mapValue(bands[bandID])
		if len(band) == 0 {
			continue
		}
		leaders := rowsFromAny(firstPresentAny(band, "dominant_tracks", "leaders", "tracks"))
		decisionTracks := rowsFromAny(band["decision_tracks"])
		if len(decisionTracks) == 0 {
			decisionTracks = leaders
		}
		out = append(out, map[string]any{
			"band":                bandID,
			"status":              StatusFromSource(text(band["status"])),
			"track_count":         firstNonZeroInt(band["track_count"], len(leaders)),
			"average_unit_energy": band["average_unit_energy"],
			"leaders":             capRelationRows(leaders, 3),
			"decision_tracks":     decisionTracks,
			"evidence_ref":        firstNonEmpty(text(band["evidence_ref"]), "project_package.project_band_occupancy."+bandID),
		})
	}
	return out
}

func dominantBandsFromOccupancy(occupancy []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range occupancy {
		leaders := rowsFromAny(row["leaders"])
		if len(leaders) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"band":         row["band"],
			"leader":       leaders[0],
			"track_count":  row["track_count"],
			"evidence_ref": row["evidence_ref"],
		})
	}
	return out
}

func conflictCandidatesFromProjectPackage(row map[string]any) []map[string]any {
	if len(row) == 0 {
		return nil
	}
	candidates := rowsFromAny(row["candidates"])
	for i := range candidates {
		if text(candidates[i]["status"]) == "" {
			candidates[i]["status"] = StatusReady
		}
		if text(candidates[i]["evidence_ref"]) == "" {
			candidates[i]["evidence_ref"] = "project_package.conflict_candidates"
		}
	}
	return capRelationRows(candidates, 12)
}

func stereoDistributionFromProjectPackage(row map[string]any, conflicts []map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := compactMap(row, "status", "reason", "track_count", "tracks_with_stereo_summary", "widest_balance_tracks", "evidence_ref")
	rows := rowsFromAny(row["widest_balance_tracks"])
	center := []map[string]any{}
	offCenter := []map[string]any{}
	phase := []map[string]any{}
	for _, track := range rows {
		balance := math.Abs(number(track["balance_db"]))
		correlation := number(track["correlation_estimate"])
		if balance <= 0.5 && (track["balance_db"] != nil || correlation >= 0.85) {
			center = append(center, track)
		}
		if balance >= 3 {
			offCenter = append(offCenter, track)
		}
		if correlation > 0 && correlation < 0.25 {
			phase = append(phase, track)
		}
	}
	for _, candidate := range conflicts {
		if text(candidate["type"]) == "phase_risk_candidate" {
			phase = append(phase, rowsFromAny(candidate["tracks"])...)
		}
	}
	if len(center) > 0 {
		out["center_heavy_tracks"] = capRelationRows(center, 6)
	}
	if len(offCenter) > 0 {
		out["off_center_tracks"] = capRelationRows(offCenter, 6)
	}
	if len(phase) > 0 {
		out["phase_risk_tracks"] = capRelationRows(phase, 6)
	}
	if text(out["evidence_ref"]) == "" {
		out["evidence_ref"] = "project_package.project_stereo_spread"
	}
	return out
}

func phaseRiskTracksFromProjectPackage(stereo map[string]any, conflicts []map[string]any) []map[string]any {
	out := rowsFromAny(stereo["phase_risk_tracks"])
	for _, candidate := range conflicts {
		if text(candidate["type"]) == "phase_risk_candidate" {
			out = append(out, rowsFromAny(candidate["tracks"])...)
		}
	}
	return capRelationRows(out, 6)
}

func multitrackRelationLayer(relation MultitrackRelation) Layer {
	status := relation.Status
	if status == "not_applicable_single_track" {
		status = StatusPartial
	}
	summary := fmt.Sprintf("MOM v1.4 multitrack relation status=%s track_count=%d.", relation.Status, relation.TrackCount)
	if relation.Status == "not_applicable_single_track" {
		summary = "MOM v1.4 multitrack relation is not applicable for a single-track project; the compact project mix profile is still available."
	}
	return Layer{
		Status:    status,
		Freshness: relation.Freshness,
		Source:    "mom.v1.4.project_multitrack_relation",
		Summary:   summary,
		Facts: map[string]any{
			"multitrack_relation": map[string]any{
				"status":                      relation.Status,
				"track_count":                 relation.TrackCount,
				"compared_tracks":             relation.ComparedTracks,
				"missing_tracks":              relation.MissingTracks,
				"level_distribution":          relation.LevelDistribution,
				"band_occupancy":              relation.BandOccupancy,
				"band_conflict_candidates":    relation.BandConflictCandidates,
				"stereo_distribution":         relation.StereoDistribution,
				"phase_risk_tracks":           relation.PhaseRiskTracks,
				"limitations":                 relation.Limitations,
				"local_math_completed":        true,
				"raw_package_required_by_llm": false,
			},
		},
		EvidenceRefs: relation.EvidenceRefs,
		Limitations:  relation.Limitations,
	}
}

func projectTrackRows(input Input) []map[string]any {
	return rowsFromAny(input.ProjectPackage["tracks"])
}

func compactComparedTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		row := compactMap(track, "track_id", "name", "track_name", "user_label", "role_guess", "active_state", "selected", "focused", "volume_db", "level_db", "rms_dbfs", "effective_static_rms_dbfs", "peak_dbfs", "effective_static_peak_dbfs", "headroom_db", "pan")
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func profileStatus(trackCount int, compared []map[string]any, limitations []string) string {
	if trackCount == 0 {
		return StatusMissing
	}
	if len(compared) == 0 {
		return StatusPartial
	}
	for _, limit := range limitations {
		if limit == "shadow_track_level_db_missing" || limit == "per_track_waveform_acoustic_missing" {
			return StatusPartial
		}
	}
	return StatusReady
}

func levelOverview(tracks []map[string]any) map[string]any {
	return map[string]any{
		"loudest_by_rms":     topMetricTrack(tracks, "rms_dbfs", true),
		"highest_peak":       topMetricTrack(tracks, "peak_dbfs", true),
		"lowest_headroom":    topMetricTrack(tracks, "headroom_db", false),
		"tracks_with_level":  countTracksWithAnyMetric(tracks, "level_db", "rms_dbfs"),
		"tracks_with_peak":   countTracksWithAnyMetric(tracks, "peak_dbfs"),
		"headroom_risk_rows": headroomRiskRows(tracks),
	}
}

func levelDistribution(tracks []map[string]any) map[string]any {
	return levelOverview(tracks)
}

func topMetricTrack(tracks []map[string]any, metric string, descending bool) map[string]any {
	var best map[string]any
	var bestValue float64
	for _, track := range tracks {
		value := number(track[metric])
		if track[metric] == nil {
			continue
		}
		if best == nil || (descending && value > bestValue) || (!descending && value < bestValue) {
			best = track
			bestValue = value
		}
	}
	if best == nil {
		return map[string]any{"status": StatusMissing, "metric": metric}
	}
	return map[string]any{
		"track_id":   best["track_id"],
		"name":       firstNonEmpty(text(best["name"]), text(best["track_name"]), text(best["user_label"])),
		"role_guess": best["role_guess"],
		"metric":     metric,
		"value":      round3mom(bestValue),
	}
}

func countTracksWithAnyMetric(tracks []map[string]any, metrics ...string) int {
	count := 0
	for _, track := range tracks {
		for _, metric := range metrics {
			if track[metric] != nil {
				count++
				break
			}
		}
	}
	return count
}

func headroomRiskRows(tracks []map[string]any) []map[string]any {
	rows := []map[string]any{}
	for _, track := range tracks {
		headroom := number(track["headroom_db"])
		if track["headroom_db"] == nil {
			continue
		}
		risk := "low"
		if headroom <= 1 {
			risk = "high"
		} else if headroom <= 3 {
			risk = "medium"
		}
		rows = append(rows, map[string]any{
			"track_id":    track["track_id"],
			"name":        firstNonEmpty(text(track["name"]), text(track["track_name"]), text(track["user_label"])),
			"headroom_db": round3mom(headroom),
			"risk":        risk,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return number(rows[i]["headroom_db"]) < number(rows[j]["headroom_db"]) })
	return capRelationRows(rows, 6)
}

func projectDominantBands(tracks []map[string]any) []map[string]any {
	occupancy := bandOccupancy(tracks)
	out := []map[string]any{}
	for _, row := range occupancy {
		leaders := rowsFromAny(row["leaders"])
		if len(leaders) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"band":        row["band"],
			"leader":      leaders[0],
			"track_count": row["track_count"],
		})
	}
	return out
}

func bandOccupancy(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, band := range mixBands {
		rows := []map[string]any{}
		for _, track := range tracks {
			bands := mapValue(mapValue(track["band_energy"])["bands"])
			bandRow := mapValue(bands[band])
			if len(bandRow) == 0 {
				continue
			}
			energy, ok := bandEnergyValue(bandRow)
			if !ok {
				continue
			}
			rows = append(rows, map[string]any{
				"track_id":    track["track_id"],
				"name":        firstNonEmpty(text(track["name"]), text(track["track_name"]), text(track["user_label"])),
				"role_guess":  track["role_guess"],
				"unit_energy": round3mom(energy),
				"energy_db":   bandRow["energy_db"],
			})
		}
		if len(rows) == 0 {
			continue
		}
		sort.Slice(rows, func(i, j int) bool { return number(rows[i]["unit_energy"]) > number(rows[j]["unit_energy"]) })
		out = append(out, map[string]any{
			"band":        band,
			"track_count": len(rows),
			"leaders":     capRelationRows(rows, 3),
		})
	}
	return out
}

func bandEnergyValue(row map[string]any) (float64, bool) {
	if row["unit_energy"] != nil {
		return number(row["unit_energy"]), true
	}
	if row["energy_db"] != nil {
		db := number(row["energy_db"])
		return math.Pow(10, db/20), true
	}
	return 0, false
}

func bandConflictCandidates(occupancy []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range occupancy {
		leaders := rowsFromAny(row["leaders"])
		if len(leaders) < 2 {
			continue
		}
		first := number(leaders[0]["unit_energy"])
		second := number(leaders[1]["unit_energy"])
		if first <= 0 || second <= 0 {
			continue
		}
		if second >= first*0.7 && first >= 0.18 {
			out = append(out, map[string]any{
				"band":       row["band"],
				"status":     "candidate",
				"reason":     "two_or_more_tracks_have_close_relative_energy_in_band",
				"tracks":     []map[string]any{leaders[0], leaders[1]},
				"confidence": "low_to_medium",
			})
		}
	}
	return out
}

func bandTendency(dominant []map[string]any) map[string]any {
	out := map[string]any{}
	for _, group := range []struct {
		name  string
		bands []string
	}{
		{"low_end", []string{"sub", "bass"}},
		{"low_mid", []string{"low_mid"}},
		{"presence", []string{"presence"}},
		{"air", []string{"air"}},
	} {
		count := 0
		for _, row := range dominant {
			for _, band := range group.bands {
				if text(row["band"]) == band {
					count++
				}
			}
		}
		out[group.name] = map[string]any{"dominant_band_count": count, "state": tendencyState(count)}
	}
	return out
}

func tendencyState(count int) string {
	if count <= 0 {
		return "unknown_or_unavailable"
	}
	if count == 1 {
		return "present"
	}
	return "prominent"
}

func stereoOverview(tracks []map[string]any) map[string]any {
	return map[string]any{
		"center_heavy_tracks": centerHeavyTracks(tracks),
		"off_center_tracks":   offCenterTracks(tracks),
		"phase_risk_tracks":   phaseRiskTracks(tracks),
		"tracks_with_stereo":  countTracksWithStereo(tracks),
	}
}

func stereoDistribution(tracks []map[string]any) map[string]any {
	return stereoOverview(tracks)
}

func centerHeavyTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		correlation := number(stereo["correlation_estimate"])
		balance := math.Abs(number(stereo["balance_db"]))
		if (correlation >= 0.85 && stereo["correlation_estimate"] != nil) || (balance <= 0.5 && stereo["balance_db"] != nil) {
			out = append(out, stereoTrackRow(track, "center_heavy"))
		}
	}
	return capRelationRows(out, 6)
}

func offCenterTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		balance := math.Abs(number(stereo["balance_db"]))
		if balance >= 3 {
			out = append(out, stereoTrackRow(track, "off_center"))
		}
	}
	return capRelationRows(out, 6)
}

func phaseRiskTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		stereo := mapValue(track["stereo_relation"])
		if phaseRisk(stereo) == "high" || phaseRisk(stereo) == "medium" {
			out = append(out, stereoTrackRow(track, "phase_risk"))
		}
	}
	return capRelationRows(out, 6)
}

func stereoTrackRow(track map[string]any, tag string) map[string]any {
	stereo := mapValue(track["stereo_relation"])
	return map[string]any{
		"track_id":             track["track_id"],
		"name":                 firstNonEmpty(text(track["name"]), text(track["track_name"]), text(track["user_label"])),
		"role_guess":           track["role_guess"],
		"tag":                  tag,
		"balance_db":           stereo["balance_db"],
		"balance_state":        stereo["balance_state"],
		"correlation_estimate": stereo["correlation_estimate"],
		"correlation_state":    stereo["correlation_state"],
		"phase_negative_ratio": stereo["phase_negative_ratio"],
	}
}

func countTracksWithStereo(tracks []map[string]any) int {
	count := 0
	for _, track := range tracks {
		if StatusFromSource(text(mapValue(track["stereo_relation"])["status"])) == StatusReady || StatusFromSource(text(mapValue(track["stereo_relation"])["status"])) == StatusPartial {
			count++
		}
	}
	return count
}

func missingRelationTracks(tracks []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, track := range tracks {
		missing := []string{}
		if StatusFromSource(text(mapValue(track["band_energy"])["status"])) != StatusReady {
			missing = append(missing, "band_energy")
		}
		if StatusFromSource(text(mapValue(track["stereo_relation"])["status"])) != StatusReady {
			missing = append(missing, "stereo_relation")
		}
		if len(missing) > 0 {
			out = append(out, map[string]any{
				"track_id": track["track_id"],
				"name":     firstNonEmpty(text(track["name"]), text(track["track_name"]), text(track["user_label"])),
				"missing":  missing,
			})
		}
	}
	return out
}

func relationHasSuspect(tracks []map[string]any) bool {
	for _, track := range tracks {
		for _, key := range []string{"band_energy", "stereo_relation"} {
			status := StatusFromSource(text(mapValue(track[key])["status"]))
			if status == StatusSuspect || status == StatusStale {
				return true
			}
		}
	}
	return false
}

func profileRiskTags(level map[string]any, dominant []map[string]any, stereo map[string]any) []string {
	tags := []string{}
	for _, row := range rowsFromAny(level["headroom_risk_rows"]) {
		if text(row["risk"]) == "high" {
			tags = append(tags, "headroom_risk_high")
			break
		}
	}
	if len(rowsFromAny(stereo["phase_risk_tracks"])) > 0 {
		tags = append(tags, "phase_risk_present")
	}
	return tags
}

func capRelationRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return rows[:max]
}

func round3mom(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func firstPresentAny(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && !empty(value) {
			return value
		}
	}
	return nil
}

func firstNonZeroInt(value any, fallback int) int {
	if n := int(number(value)); n > 0 {
		return n
	}
	return fallback
}
