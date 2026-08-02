package mom

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type frequencyRegionDefinition struct {
	ID    string
	MinHz float64
	MaxHz float64
}

var frequencyRegionDefinitions = []frequencyRegionDefinition{
	{ID: "sub", MinHz: 20, MaxHz: 60},
	{ID: "bass", MinHz: 60, MaxHz: 250},
	{ID: "low_mid", MinHz: 250, MaxHz: 500},
	{ID: "mid", MinHz: 500, MaxHz: 2000},
	{ID: "presence", MinHz: 2000, MaxHz: 6000},
	{ID: "air", MinHz: 6000, MaxHz: 20000},
}

func buildFrequencyRelationship(input Input) FrequencyRelationship {
	tracks := frequencyProjectTrackRows(input)
	profiles := make([]map[string]any, 0, len(tracks))
	evidenceRefsOut := []string{observationRef(input), "mix.read:project.tracks.summary", "mix.read:project.frequency_relationship_inputs"}
	limitations := []string{"energy_overlap_is_candidate_only_not_psychoacoustic_masking_fact", "no_eq_parameters_plugin_choices_proposals_or_pending_actions"}
	missingTrackIDs := []string{}
	eligible := 0
	partial := 0
	stale := 0
	suspect := 0
	tapSet := map[string]bool{}
	for _, track := range tracks {
		profile := frequencyTrackProfile(track)
		profiles = append(profiles, profile)
		trackID := text(profile["track_id"])
		switch StatusFromSource(text(profile["status"])) {
		case StatusReady:
			eligible++
		case StatusPartial, StatusApprox:
			eligible++
			partial++
		case StatusStale:
			stale++
			missingTrackIDs = append(missingTrackIDs, trackID)
		case StatusSuspect:
			suspect++
			missingTrackIDs = append(missingTrackIDs, trackID)
		default:
			missingTrackIDs = append(missingTrackIDs, trackID)
		}
		if tap := text(profile["tap_point"]); tap != "" && tap != "unknown" {
			tapSet[tap] = true
		}
		if ref := text(profile["evidence_ref"]); ref != "" {
			evidenceRefsOut = append(evidenceRefsOut, ref)
		}
	}

	tapPoint := uniformFrequencyTapPoint(tapSet)
	status := frequencyRelationshipStatus(len(tracks), eligible, partial, stale, suspect, tapPoint)
	if len(tapSet) == 0 {
		limitations = append(limitations, "tap_point_unknown")
	} else if tapPoint == "mixed" {
		limitations = append(limitations, "mixed_tap_points_are_not_comparable")
	}
	if tapPoint == "source_file_pre_fx" {
		limitations = append(limitations, "source_file_pre_fx_does_not_represent_current_post_plugin_signal")
	}
	if eligible < len(tracks) {
		limitations = append(limitations, "one_or_more_tracks_missing_fresh_frequency_evidence")
	}

	regions := buildFrequencyRegions(profiles)
	conflicts := buildFrequencyConflictCandidates(regions)
	tendencies := buildFrequencyTonalTendencies(profiles)
	persistence := buildFrequencyPersistenceSummary(profiles)
	if StatusFromSource(text(persistence["status"])) != StatusReady {
		limitations = append(limitations, "time_frequency_persistence_unavailable")
	}
	projectCutRef, weakCutRef := frequencyProjectCutRef(input)
	if weakCutRef {
		limitations = append(limitations, "project_cut_ref_is_observation_authority_not_execution_authority")
	}
	coverageRatio := 0.0
	if len(tracks) > 0 {
		coverageRatio = float64(eligible) / float64(len(tracks))
	}
	coverage := map[string]any{
		"project_track_count":       len(tracks),
		"profile_count":             len(profiles),
		"eligible_track_count":      eligible,
		"missing_track_count":       len(missingTrackIDs),
		"missing_track_ids":         missingTrackIDs,
		"eligible_track_ratio":      round3mom(coverageRatio),
		"frequency_region_count":    len(regions),
		"conflict_candidate_count":  len(conflicts),
		"decision_tracks_truncated": false,
		"supports_static_diagnosis": eligible > 0 && tapPoint != "mixed",
		"supports_same_tap_compare": tapPoint != "" && tapPoint != "unknown" && tapPoint != "mixed",
		"supports_post_fx_compare":  tapPoint != "source_file_pre_fx" && tapPoint != "" && tapPoint != "unknown" && tapPoint != "mixed",
	}
	return FrequencyRelationship{
		SchemaVersion:      FrequencyRelationshipSchema,
		Status:             status,
		Freshness:          FreshnessForStatus(status),
		ProjectCutRef:      projectCutRef,
		Scope:              frequencyRelationshipScope(input, profiles),
		TapPoint:           firstNonEmpty(tapPoint, "unknown"),
		Coverage:           coverage,
		TrackProfiles:      profiles,
		FrequencyRegions:   regions,
		ConflictCandidates: conflicts,
		TonalTendencies:    tendencies,
		PersistenceSummary: persistence,
		VerificationDimensions: []string{
			"tap_point", "track_coverage", "frequency_region_energy_relationship", "conflict_candidate_membership", "relative_tonal_shape", "time_frequency_persistence_when_available",
		},
		EvidenceRefs: evidenceRefs(evidenceRefsOut...),
		Limitations:  evidenceRefs(limitations...),
	}
}

func frequencyProjectTrackRows(input Input) []map[string]any {
	tracks := projectTrackRows(input)
	out := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		if !trackRequiresFrequencyProfile(track) {
			continue
		}
		out = append(out, track)
	}
	return out
}

func trackRequiresFrequencyProfile(track map[string]any) bool {
	if len(track) == 0 {
		return false
	}
	for _, key := range []string{"is_folder_track", "is_folder_container", "is_submix_folder"} {
		if boolValue(track[key]) {
			return false
		}
	}
	trackType := strings.ToLower(strings.TrimSpace(text(track["track_type"])))
	if strings.Contains(trackType, "folder") || strings.Contains(trackType, "container") {
		return false
	}
	if boolValue(track["has_child_tracks"]) && int(number(track["clip_count"])) == 0 && !boolValue(track["is_audio_track"]) && !boolValue(track["is_audio"]) {
		return false
	}
	_, hasAudioTrackFlag := track["is_audio_track"]
	_, hasAudioFlag := track["is_audio"]
	if hasAudioTrackFlag && hasAudioFlag && !boolValue(track["is_audio_track"]) && !boolValue(track["is_audio"]) && int(number(track["clip_count"])) == 0 {
		return false
	}
	return true
}

func frequencyTrackProfile(track map[string]any) map[string]any {
	bandSummary := mapValue(track["frequency_evidence"])
	if len(bandSummary) == 0 {
		bandSummary = mapValue(track["band_energy"])
	}
	status := StatusFromSource(text(bandSummary["status"]))
	bandsIn := mapValue(bandSummary["bands"])
	bands := map[string]any{}
	for _, definition := range frequencyRegionDefinitions {
		band := mapValue(bandsIn[definition.ID])
		if len(band) == 0 {
			continue
		}
		row := compactMap(band, "status", "unit_energy", "energy_db", "min_hz", "max_hz", "coverage_ratio", "persistence_ratio", "active_frame_ratio")
		if row["min_hz"] == nil {
			row["min_hz"] = definition.MinHz
		}
		if row["max_hz"] == nil {
			row["max_hz"] = definition.MaxHz
		}
		bands[definition.ID] = row
	}
	if (status == StatusReady || status == StatusPartial) && len(bands) == 0 {
		status = StatusMissing
	}
	trackID := text(track["track_id"])
	if trackID == "" {
		trackID = text(track["id"])
	}
	name := firstNonEmpty(text(track["name"]), text(track["track_name"]), text(track["user_label"]), trackID)
	tapPoint := frequencyEvidenceTapPoint(bandSummary)
	return map[string]any{
		"track_id":     trackID,
		"name":         name,
		"active_state": firstNonEmpty(text(track["active_state"]), "unknown"),
		"role_hypothesis": map[string]any{
			"value":      text(track["role_guess"]),
			"source":     "tom_or_project_track_hypothesis",
			"confidence": "unspecified",
		},
		"status":            status,
		"freshness":         FreshnessForStatus(status),
		"tap_point":         firstNonEmpty(tapPoint, "unknown"),
		"coverage":          compactMap(bandSummary, "coverage_seconds", "coverage_ratio", "total_duration"),
		"bands":             bands,
		"evidence_ref":      frequencyTrackEvidenceRef(trackID, bandSummary),
		"silence_confirmed": boolValue(bandSummary["silence_confirmed"]),
		"silence_reason":    text(bandSummary["silence_reason"]),
	}
}

func frequencyEvidenceTapPoint(row map[string]any) string {
	if tap := strings.ToLower(text(row["tap_point"])); tap != "" {
		return tap
	}
	source := strings.ToLower(firstNonEmpty(text(row["source"]), text(row["source_kind"]), text(row["layer"])))
	if containsAny(source, "l2_render_probe", "render_probe", "offline_probe") {
		return "unknown_post_chain"
	}
	if containsAny(source, "kernel_l3_offline_analyzer", "spectral_tile_derived", "l3_deep", "spectral_field") {
		return "source_file_pre_fx"
	}
	return ""
}

func uniformFrequencyTapPoint(taps map[string]bool) string {
	if len(taps) == 0 {
		return "unknown"
	}
	if len(taps) > 1 {
		return "mixed"
	}
	for tap := range taps {
		return tap
	}
	return "unknown"
}

func frequencyRelationshipStatus(trackCount, eligible, partial, stale, suspect int, tapPoint string) string {
	if trackCount == 0 || eligible == 0 {
		if suspect > 0 {
			return StatusSuspect
		}
		if stale > 0 {
			return StatusStale
		}
		return StatusMissing
	}
	if tapPoint == "mixed" || suspect > 0 {
		return StatusSuspect
	}
	if stale > 0 || partial > 0 || eligible < trackCount || tapPoint == "unknown" {
		return StatusPartial
	}
	return StatusReady
}

func buildFrequencyRegions(profiles []map[string]any) []map[string]any {
	regions := make([]map[string]any, 0, len(frequencyRegionDefinitions))
	for _, definition := range frequencyRegionDefinitions {
		decisionTracks := []map[string]any{}
		sum := 0.0
		for _, profile := range profiles {
			if StatusFromSource(text(profile["status"])) != StatusReady && StatusFromSource(text(profile["status"])) != StatusPartial {
				continue
			}
			band := mapValue(mapValue(profile["bands"])[definition.ID])
			energy, ok := bandEnergyValue(band)
			if !ok {
				continue
			}
			decisionTracks = append(decisionTracks, map[string]any{
				"track_id":    profile["track_id"],
				"name":        profile["name"],
				"unit_energy": round3mom(energy),
				"energy_db":   firstPresentAny(band, "energy_db"),
			})
			sum += energy
		}
		sort.SliceStable(decisionTracks, func(i, j int) bool {
			return number(decisionTracks[i]["unit_energy"]) > number(decisionTracks[j]["unit_energy"])
		})
		status := StatusMissing
		if len(decisionTracks) > 0 {
			status = StatusReady
		}
		row := map[string]any{
			"region":                    definition.ID,
			"min_hz":                    definition.MinHz,
			"max_hz":                    definition.MaxHz,
			"status":                    status,
			"track_count":               len(decisionTracks),
			"decision_tracks":           decisionTracks,
			"decision_tracks_truncated": false,
		}
		if len(decisionTracks) > 0 {
			row["average_unit_energy"] = round3mom(sum / float64(len(decisionTracks)))
		}
		regions = append(regions, row)
	}
	return regions
}

func buildFrequencyConflictCandidates(regions []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, region := range regions {
		tracks := rowsFromAny(region["decision_tracks"])
		if len(tracks) < 2 {
			continue
		}
		leader := number(tracks[0]["unit_energy"])
		if leader <= 0 {
			continue
		}
		closeTracks := []map[string]any{}
		for _, track := range tracks {
			if number(track["unit_energy"]) >= leader*0.65 {
				closeTracks = append(closeTracks, track)
			}
		}
		if len(closeTracks) < 2 {
			continue
		}
		out = append(out, map[string]any{
			"type":                 "frequency_energy_overlap_candidate",
			"status":               "candidate",
			"region":               region["region"],
			"min_hz":               region["min_hz"],
			"max_hz":               region["max_hz"],
			"tracks":               closeTracks,
			"confidence":           "low_to_medium",
			"basis":                "relative_whole_scope_band_energy",
			"interpretation_limit": "not_a_psychoacoustic_masking_fact",
		})
	}
	return out
}

func buildFrequencyTonalTendencies(profiles []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, profile := range profiles {
		if boolValue(profile["silence_confirmed"]) {
			continue
		}
		bands := mapValue(profile["bands"])
		type bandValue struct {
			id    string
			value float64
		}
		values := []bandValue{}
		for _, definition := range frequencyRegionDefinitions {
			if value, ok := bandEnergyValue(mapValue(bands[definition.ID])); ok {
				values = append(values, bandValue{id: definition.ID, value: value})
			}
		}
		if len(values) == 0 {
			continue
		}
		sort.SliceStable(values, func(i, j int) bool { return values[i].value > values[j].value })
		strongest := values[0]
		weakest := values[len(values)-1]
		deltaDB := 0.0
		if strongest.value > 0 && weakest.value > 0 {
			deltaDB = 20 * math.Log10(strongest.value/weakest.value)
		}
		out = append(out, map[string]any{
			"track_id":             profile["track_id"],
			"status":               "candidate",
			"shape_basis":          "within_track_relative_band_energy",
			"strongest_region":     strongest.id,
			"weakest_region":       weakest.id,
			"relative_spread_db":   round3mom(deltaDB),
			"interpretation_limit": "relative_shape_only_not_an_eq_instruction",
		})
	}
	return out
}

func buildFrequencyPersistenceSummary(profiles []map[string]any) map[string]any {
	rows := []map[string]any{}
	for _, profile := range profiles {
		for region, raw := range mapValue(profile["bands"]) {
			band := mapValue(raw)
			value := firstPresentAny(band, "persistence_ratio", "active_frame_ratio")
			if value == nil {
				continue
			}
			rows = append(rows, map[string]any{"track_id": profile["track_id"], "region": region, "ratio": value})
		}
	}
	if len(rows) == 0 {
		return map[string]any{
			"status": StatusMissing,
			"reason": "no_time_frequency_overlap_evidence",
			"can_distinguish_persistent_from_transient_overlap": false,
		}
	}
	return map[string]any{
		"status": StatusReady,
		"can_distinguish_persistent_from_transient_overlap": true,
		"rows": rows,
	}
}

func frequencyRelationshipScope(input Input, profiles []map[string]any) map[string]any {
	trackIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if id := text(profile["track_id"]); id != "" {
			trackIDs = append(trackIDs, id)
		}
	}
	return map[string]any{
		"kind":             "project_tracks",
		"project_id":       firstNonEmpty(text(input.ProjectPackage["project_uuid"]), text(input.ProjectPackage["project_id"]), text(sourceIdentity(input)["project_id"]), "current"),
		"source_mode":      firstNonEmpty(text(input.ListenScope["source_mode"]), text(mapValue(input.ListenScope["source"])["mode"]), "full_project"),
		"time_mode":        firstNonEmpty(text(input.ListenScope["time_mode"]), text(mapValue(input.ListenScope["time"])["mode"]), "full_song"),
		"duration_seconds": firstPositiveNumber(input.TimeRuler, "duration_seconds"),
		"track_ids":        trackIDs,
	}
}

func frequencyProjectCutRef(input Input) (string, bool) {
	if ref := firstNonEmpty(text(input.Args["project_cut_ref"]), text(input.Args["project_cut_hash"])); ref != "" {
		if text(input.Args["project_cut_ref"]) == "" {
			ref = "project-cut:" + ref
		}
		return ref, false
	}
	projectID := firstNonEmpty(text(input.ProjectPackage["project_uuid"]), text(input.ProjectPackage["project_id"]), text(sourceIdentity(input)["project_id"]))
	epoch := text(input.ProjectPackage["project_epoch"])
	revision := firstNonEmpty(text(input.ProjectPackage["project_revision"]), text(input.ProjectPackage["revision"]))
	snapshotHash := text(input.ProjectPackage["project_state_hash"])
	if projectID != "" && epoch != "" && revision != "" {
		ref := fmt.Sprintf("project-state:%s:%s:%s", safeKey(projectID), safeKey(epoch), safeKey(revision))
		if snapshotHash != "" {
			ref += ":" + safeKey(snapshotHash)
		}
		return ref, false
	}
	if input.ObservationID != "" {
		return "observation:" + input.ObservationID, true
	}
	return "observation:unbound", true
}

func frequencyTrackEvidenceRef(trackID string, row map[string]any) string {
	if ref := text(row["evidence_ref"]); ref != "" {
		return ref
	}
	requestID := text(row["request_id"])
	if requestID != "" {
		return "dad.band_energy_summary:" + safeKey(trackID) + ":" + safeKey(requestID)
	}
	return "mix.read:track." + safeKey(trackID) + ".slow.band_energy.summary"
}

// FrequencyRelationshipsComparable checks only observation comparability. It
// does not authorize execution or claim that an audible improvement occurred.
func FrequencyRelationshipsComparable(before, after FrequencyRelationship) (bool, []string) {
	reasons := []string{}
	if before.SchemaVersion != FrequencyRelationshipSchema || after.SchemaVersion != FrequencyRelationshipSchema {
		reasons = append(reasons, "schema_mismatch")
	}
	if before.TapPoint == "" || before.TapPoint == "unknown" || before.TapPoint == "mixed" || before.TapPoint != after.TapPoint {
		reasons = append(reasons, "tap_point_mismatch_or_unknown")
	}
	if text(before.Scope["project_id"]) == "" || text(before.Scope["project_id"]) != text(after.Scope["project_id"]) {
		reasons = append(reasons, "project_scope_mismatch")
	}
	if !sameFrequencyTrackIDs(before.Scope["track_ids"], after.Scope["track_ids"]) {
		reasons = append(reasons, "track_scope_mismatch")
	}
	if StatusFromSource(before.Status) == StatusMissing || StatusFromSource(after.Status) == StatusMissing || StatusFromSource(before.Status) == StatusStale || StatusFromSource(after.Status) == StatusStale || StatusFromSource(before.Status) == StatusSuspect || StatusFromSource(after.Status) == StatusSuspect {
		reasons = append(reasons, "frequency_relationship_not_comparable_status")
	}
	return len(reasons) == 0, evidenceRefs(reasons...)
}

func sameFrequencyTrackIDs(a, b any) bool {
	left := stringsFromAny(a)
	right := stringsFromAny(b)
	if len(left) != len(right) {
		return false
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
