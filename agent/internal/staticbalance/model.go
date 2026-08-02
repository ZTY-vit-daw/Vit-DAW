package staticbalance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
)

type roleEvidence struct {
	Role       string
	Source     string
	Confidence float64
}

func BuildModel(in Input) Model {
	generatedAt := in.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	style := in.Style
	if mixstyle.Validate(style) != nil {
		style = mixstyle.Default()
	}
	mom := findMOMProjection(in.MOMProjection, in.MixObservation, in.ContextSnapshot, in.RequestContext)
	tom := findTOMProjection(in.TOMProjection, in.ContextSnapshot, in.RequestContext, in.ProjectState, in.ExecutionMemory)
	projectRows := findProjectRows(in.ProjectState, in.ContextSnapshot, in.RequestContext)
	momRows := collectMOMRows(mom, in.MixObservation)
	staticLevelRows := collectMOMStaticLevelRows(mom)
	tomRoles := collectTOMRoles(tom)

	tracks := make([]Track, 0, len(projectRows))
	seen := map[string]bool{}
	for _, row := range projectRows {
		track := buildTrack(row, momRows, staticLevelRows, tomRoles)
		if track.TrackID == "" || seen[track.TrackID] {
			continue
		}
		seen[track.TrackID] = true
		tracks = append(tracks, track)
	}
	sort.SliceStable(tracks, func(i, j int) bool { return tracks[i].TrackID < tracks[j].TrackID })

	coverage := measureCoverage(tracks)
	evidenceRefs := []string{}
	if len(projectRows) > 0 {
		evidenceRefs = append(evidenceRefs, "project.state")
	}
	if len(mom) > 0 {
		evidenceRefs = append(evidenceRefs, "mix.observe", "MOM")
	}
	if relation := mapValue(mom["static_level_relationship"]); len(relation) > 0 {
		evidenceRefs = append(evidenceRefs, "MOM:static_level_relationship")
		evidenceRefs = append(evidenceRefs, stringSlice(relation["evidence_refs"])...)
	}
	if len(tom) > 0 {
		evidenceRefs = append(evidenceRefs, "TOM")
	}
	observationID := observationID(in.MixObservation, mom)
	if observationID != "" {
		evidenceRefs = append(evidenceRefs, observationID)
	}
	evidenceRefs = uniqueStrings(evidenceRefs)
	readiness := assessReadiness(tracks, coverage, mom, tom, in.ProjectState, evidenceRefs)
	limitations := append([]string(nil), readiness.BlockedBy...)
	limitations = append(limitations, readiness.Warnings...)

	modelID := stableID("sbm", CapabilityID, in.UserIntent, style.ID, observationID, fmt.Sprint(len(tracks)), strings.Join(evidenceRefs, "|"), generatedAt.Format(time.RFC3339))
	return Model{
		SchemaVersion: ModelSchemaVersion,
		ModelID:       modelID,
		CapabilityID:  CapabilityID,
		GeneratedAt:   generatedAt.Format(time.RFC3339),
		UserIntent:    compactText(in.UserIntent, 256),
		ObservationID: observationID,
		Style: Style{
			SchemaVersion: style.SchemaVersion,
			Kind:          style.Kind,
			ID:            style.ID,
			Name:          style.Name,
			Explicit:      in.StyleExplicit,
			Dimensions:    style.Dimensions,
		},
		Tracks:       tracks,
		Coverage:     coverage,
		Readiness:    readiness,
		EvidenceRefs: evidenceRefs,
		Limitations:  uniqueStrings(limitations),
		MOMSummary:   compactMOM(mom),
		TOMSummary:   compactTOM(tom),
	}
}

func measureCoverage(tracks []Track) Coverage {
	out := Coverage{AnalyzedTrackCount: len(tracks)}
	functions := map[string]bool{}
	for _, track := range tracks {
		roleCandidate := track.Eligible || track.Exclusion == "role_unresolved" || track.Exclusion == "role_confidence_low"
		if roleCandidate {
			out.RoleCandidateCount++
			if track.Role != "" && track.Role != "unknown" {
				out.RawRoleKnownCount++
			}
			if track.Function != "" {
				out.RoleKnownCount++
			}
			if track.FaderDB != nil {
				out.FaderKnownCount++
			}
			if track.LevelDB != nil {
				out.LevelKnownCount++
				out.ProjectedLevelKnownCount++
				if track.LevelStatus == "approximate" {
					out.ApproximateLevelCount++
				}
			} else if track.LevelStatus != "" && track.LevelStatus != "ready" && track.LevelStatus != "approximate" {
				out.BlockedLevelCount++
			}
		}
		if track.Eligible {
			out.EligibleTrackCount++
		}
		if track.Eligible && track.FaderDB != nil && track.LevelDB != nil && track.Function != "" {
			out.ComparableCount++
			functions[track.Function] = true
		}
	}
	out.FunctionCount = len(functions)
	if out.RoleCandidateCount > 0 {
		out.RoleCoverage = round3(float64(out.EligibleTrackCount) / float64(out.RoleCandidateCount))
		out.EffectiveLevelCoverage = round3(float64(out.LevelKnownCount) / float64(out.RoleCandidateCount))
		out.LevelCoverage = round3(float64(out.ComparableCount) / float64(out.RoleCandidateCount))
	}
	return out
}

func assessReadiness(tracks []Track, coverage Coverage, mom, tom, projectState map[string]any, refs []string) capabilityruntime.Readiness {
	conditions := []capabilityruntime.Condition{}
	conditions = append(conditions, readinessCondition(
		"project_structure", true, len(tracks) > 0, len(tracks), len(tracks),
		"project.state contains the full project track inventory", refs,
		"Refresh project.state before B2 static balance.",
	))

	momStatus, momSummary, momRelationCount := b2MOMRelationReadiness(mom)
	conditions = append(conditions, capabilityruntime.Condition{
		ID: "mom_multitrack_relationships", Required: true, Status: momStatus, Summary: momSummary,
		KnownCount: momRelationCount, TotalCount: len(tracks), EvidenceRefs: refs,
		Remediation: []string{"Refresh one full-project mix.observe/MOM relationship observation."},
	})
	if blocked, explicit := momActionBlocked(mom); explicit && blocked {
		conditions = append(conditions, capabilityruntime.Condition{
			ID: "mom_generic_action_preflight", Required: false, Status: capabilityruntime.ConditionWarning,
			Summary:      "MOM generic action preflight is false; B2 uses its own L1/role/fader readiness and does not require L3 spectral or stereo evidence",
			EvidenceRefs: refs,
		})
	}

	roleReady := coverage.RoleKnownCount >= 2 && coverage.EligibleTrackCount >= 2 && coverage.RoleCoverage >= 0.55
	conditions = append(conditions, readinessCondition(
		"track_role_relationships", true, roleReady, coverage.EligibleTrackCount, coverage.RoleCandidateCount,
		fmt.Sprintf("functional role coverage is %.0f%%", coverage.RoleCoverage*100), refs,
		"Resolve or confirm track roles in TOM/project metadata; B1 is not required for role resolution.",
	))

	faderCoverage := ratio(coverage.FaderKnownCount, coverage.RoleCandidateCount)
	conditions = append(conditions, readinessCondition(
		"current_fader_state", true, coverage.FaderKnownCount >= 2 && faderCoverage >= 0.95, coverage.FaderKnownCount, coverage.RoleCandidateCount,
		fmt.Sprintf("current track-fader coverage is %.0f%%", faderCoverage*100), refs,
		"Refresh project.state so B2 can fingerprint current track faders before proposing actions.",
	))

	levelReady := coverage.LevelKnownCount >= 2 && coverage.EffectiveLevelCoverage >= 0.95
	conditions = append(conditions, readinessCondition(
		"effective_l1_track_levels", true, levelReady, coverage.LevelKnownCount, coverage.RoleCandidateCount,
		fmt.Sprintf("MOM static-level coverage is %.0f%% (%d approximate, %d blocked); solver-comparable coverage is %.0f%%", coverage.EffectiveLevelCoverage*100, coverage.ApproximateLevelCount, coverage.BlockedLevelCount, coverage.LevelCoverage*100), refs,
		"Refresh one full-project mix.observe MOM static-level projection after the latest project edit.",
	))

	staticRelation := mapValue(mom["static_level_relationship"])
	staticStatus := strings.ToLower(firstText(staticRelation, "status"))
	staticReady := staticStatus == "ready" || staticStatus == "approximate"
	conditions = append(conditions, readinessCondition(
		"mom_static_level_relationship", true, staticReady, coverage.ProjectedLevelKnownCount, coverage.RoleCandidateCount,
		fmt.Sprintf("MOM static-level relationship status is %s", defaultText(staticStatus, "missing")), refs,
		"Refresh the MOM static-level relationship; stale, suspect, partial, or missing projections cannot support B2 planning.",
	))
	relationCut := firstText(staticRelation, "project_cut_ref")
	currentCut := currentProjectCutRef(projectState)
	cutReady := relationCut != "" && (currentCut == "" || relationCut == currentCut)
	cutSummary := "MOM static-level projection identifies its source project cut"
	if relationCut == "" {
		cutSummary = "MOM static-level projection omitted project_cut_ref"
	} else if currentCut != "" && relationCut != currentCut {
		cutSummary = "MOM static-level projection belongs to a different project cut"
	}
	conditions = append(conditions, readinessCondition(
		"mom_static_level_project_cut", true, cutReady, boolInt(cutReady), 1, cutSummary, refs,
		"Refresh mix.observe against the current project revision before B2 planning.",
	))

	conditions = append(conditions, readinessCondition(
		"functional_diversity", true, coverage.FunctionCount >= 2, coverage.FunctionCount, 2,
		"at least two musical functions are required for relationship balancing", refs,
		"Confirm track roles so at least two functional groups can be compared.",
	))

	clipped, peakKnown := 0, 0
	for _, track := range tracks {
		if track.PeakDBFS == nil {
			continue
		}
		peakKnown++
		if *track.PeakDBFS >= 0 {
			clipped++
		}
	}
	technicalStatus := capabilityruntime.ConditionReady
	technicalSummary := "no known source/track clipping contradicts the B2 technical baseline"
	if clipped > 0 {
		technicalStatus = capabilityruntime.ConditionBlocked
		technicalSummary = fmt.Sprintf("%d tracks have non-negative peak evidence", clipped)
	} else if peakKnown == 0 {
		technicalStatus = capabilityruntime.ConditionWarning
		technicalSummary = "peak evidence is unavailable; technical baseline cannot be fully verified"
	}
	conditions = append(conditions, capabilityruntime.Condition{
		ID: "healthy_technical_baseline", Required: true, Status: technicalStatus, Summary: technicalSummary,
		KnownCount: peakKnown, TotalCount: len(tracks), EvidenceRefs: refs,
		Remediation: []string{"Run source-level calibration such as B1 when clipping or unhealthy source levels are present, then refresh observation."},
	})

	if len(tom) == 0 {
		conditions = append(conditions, capabilityruntime.Condition{
			ID: "tom_projection", Required: false, Status: capabilityruntime.ConditionWarning,
			Summary:      "TOM is missing; only explicit metadata and conservative name inference are available",
			EvidenceRefs: refs, Remediation: []string{"Refresh TOM or confirm ambiguous track roles."},
		})
	}
	return capabilityruntime.Evaluate(CapabilityID, conditions)
}

func currentProjectCutRef(projectState map[string]any) string {
	project := mapValue(projectState["project"])
	for _, value := range []string{
		firstText(projectState, "snapshot_hash", "project_state_hash", "project_revision", "revision"),
		firstText(project, "snapshot_hash", "project_state_hash", "project_revision", "revision"),
	} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func readinessCondition(id string, required, ready bool, known, total int, summary string, refs []string, remediation string) capabilityruntime.Condition {
	status := capabilityruntime.ConditionReady
	if !ready {
		status = capabilityruntime.ConditionBlocked
	}
	return capabilityruntime.Condition{
		ID: id, Required: required, Status: status, Summary: summary,
		KnownCount: known, TotalCount: total, EvidenceRefs: refs, Remediation: []string{remediation},
	}
}

func buildTrack(row map[string]any, momRows, staticLevelRows map[string]map[string]any, tomRoles map[string]roleEvidence) Track {
	id := firstText(row, "track_id", "id")
	merged := cloneMap(row)
	momRow := momRows[id]
	if len(momRow) > 0 {
		for key, value := range momRow {
			if acousticField(key) || emptyValue(merged[key]) {
				merged[key] = value
			}
		}
	}
	track := Track{
		TrackID:   id,
		TrackName: firstText(merged, "track_name", "name", "user_label"),
		TrackType: firstText(merged, "track_type", "type", "kind"),
	}
	role := firstText(row, "role", "role_hypothesis", "role_guess")
	if role != "" && !strings.EqualFold(role, "unknown") {
		track.Role, track.RoleSource, track.RoleConfidence = normalizeRole(role), "explicit_role", 0.90
	} else if evidence, ok := tomRoles[id]; ok {
		track.Role, track.RoleSource, track.RoleConfidence = evidence.Role, evidence.Source, evidence.Confidence
	} else if role = firstText(momRows[id], "role_guess", "role_hypothesis", "role"); role != "" && !strings.EqualFold(role, "unknown") {
		track.Role, track.RoleSource, track.RoleConfidence = normalizeRole(role), "mom_role_guess", 0.65
	} else {
		track.Role, track.RoleConfidence = inferRole(track.TrackName)
		if track.Role != "unknown" {
			track.RoleSource = "track_name_inference"
		}
	}
	track.Function = roleFunction(track.Role)
	track.FaderDB, _ = firstNumber(row, "volume_db", "fader_db", "track_gain_db", "gain_db", "db")
	levelRow := staticLevelRows[id]
	track.LevelStatus = strings.ToLower(firstText(levelRow, "status"))
	track.LevelFreshness = firstText(levelRow, "freshness")
	track.LevelTapPoint = firstText(levelRow, "tap_point")
	track.IncludedClipIDs = stringSlice(levelRow["included_clip_ids"])
	track.AggregationMethod = firstText(levelRow, "aggregation_method")
	if usableStaticLevelStatus(track.LevelStatus) {
		track.LevelDB, _ = firstNumber(levelRow, "effective_static_rms_dbfs")
		track.LevelMetric = defaultText(firstText(levelRow, "metric"), "effective_static_rms_dbfs")
		track.LevelSource = "mom_static_level_relationship"
		track.SourceLevelDB, _ = firstNumber(levelRow, "source_rms_dbfs")
		track.PeakDBFS, _ = firstNumber(levelRow, "effective_static_peak_dbfs")
	}
	if track.HeadroomDB == nil && track.PeakDBFS != nil {
		value := math.Max(0, -*track.PeakDBFS)
		track.HeadroomDB = &value
	}
	track.Eligible, track.Exclusion = trackEligibility(merged, track)
	return track
}

func usableStaticLevelStatus(status string) bool {
	return status == "ready" || status == "approximate"
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func trackEligibility(row map[string]any, track Track) (bool, string) {
	typeText := strings.ToLower(track.TrackType + " " + track.Role)
	if isAudio, explicit := explicitBool(row, "is_audio_track", "is_audio"); explicit && !isAudio && len(rowsValue(row["clips"])) == 0 {
		return false, "non_audio_or_empty_track"
	}
	if boolValue(row["is_master_track"]) || strings.Contains(typeText, "master") {
		return false, "master_track"
	}
	if boolValue(row["is_folder_track"]) || boolValue(row["is_folder_container"]) || strings.Contains(typeText, "folder") {
		return false, "folder_or_group_container"
	}
	if strings.Contains(typeText, "bus_or_print") || strings.Contains(typeText, "fx_return") || strings.Contains(typeText, "return") {
		return false, "routing_or_return_track"
	}
	activeState := strings.ToLower(firstText(row, "active_state", "state"))
	if boolValue(row["mute"]) || boolValue(row["muted"]) || boolValue(row["hidden"]) || boolValue(row["is_hidden"]) || strings.Contains(activeState, "inactive") || strings.Contains(activeState, "offline") || strings.Contains(activeState, "disabled") {
		return false, "inactive_muted_or_hidden_track"
	}
	if track.Role == "" || track.Role == "unknown" || track.Function == "" {
		return false, "role_unresolved"
	}
	if track.RoleConfidence < 0.60 {
		return false, "role_confidence_low"
	}
	return true, ""
}

func findProjectRows(projectState, contextSnapshot, requestContext map[string]any) []map[string]any {
	for _, source := range []map[string]any{projectState, mapValue(projectState["daw_state_summary"]), contextSnapshot, mapValue(contextSnapshot["daw_state_summary"]), requestContext} {
		if rows := rowsValue(source["tracks"]); len(rows) > 0 {
			return rows
		}
	}
	for _, source := range []map[string]any{projectState, contextSnapshot, requestContext} {
		if rows := rowsValue(source["visible_tracks"]); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func findMOMProjection(sources ...map[string]any) map[string]any {
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		if firstText(source, "mom_version") != "" && (len(mapValue(source["multitrack_relation"])) > 0 || len(mapValue(source["trust_quality"])) > 0) {
			return source
		}
		for _, candidate := range []map[string]any{
			mapValue(source["mom_projection"]),
			mapValue(mapValue(source["observation"])["mom_projection"]),
			mapValue(mapValue(mapValue(source["context_pack"])["latest_observation"])["mom_projection"]),
		} {
			if len(candidate) > 0 {
				return candidate
			}
		}
	}
	return nil
}

func findTOMProjection(sources ...map[string]any) map[string]any {
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		if firstText(source, "tom_version", "schema_version") != "" && (len(mapValue(source["full_assignment_manifest"])) > 0 || len(rowsValue(source["group_proposals"])) > 0) {
			return source
		}
		for _, key := range []string{"tom_projection", "track_organization_projection", "a3_tom_projection"} {
			if candidate := mapValue(source[key]); len(candidate) > 0 {
				return candidate
			}
		}
		for _, key := range []string{"project_preparation", "latest_import", "project_blackboard", "recent_goal_context"} {
			if candidate := findTOMProjection(mapValue(source[key])); len(candidate) > 0 {
				return candidate
			}
		}
	}
	return nil
}

func collectTOMRoles(tom map[string]any) map[string]roleEvidence {
	out := map[string]roleEvidence{}
	add := func(trackID, role, source string, confidence float64) {
		trackID, role = strings.TrimSpace(trackID), normalizeRole(role)
		if trackID == "" || role == "" || role == "unknown" {
			return
		}
		if current, ok := out[trackID]; ok && current.Confidence >= confidence {
			return
		}
		out[trackID] = roleEvidence{Role: role, Source: source, Confidence: confidence}
	}
	for _, group := range rowsValue(tom["group_proposals"]) {
		role, base := firstText(group, "role_hypothesis", "role"), confidence(group)
		for _, assignment := range rowsValue(group["assignment_excerpt"]) {
			assignmentRole := firstText(assignment, "role_hypothesis", "role")
			if assignmentRole == "" {
				assignmentRole = role
			}
			add(firstText(assignment, "track_id", "id"), assignmentRole, "tom_assignment", confidenceOverride(base, assignment))
		}
	}
	manifest := mapValue(tom["full_assignment_manifest"])
	for _, group := range rowsValue(manifest["groups"]) {
		role, base := firstText(group, "role_hypothesis", "role"), confidence(group)
		for _, assignment := range rowsValue(group["assignments"]) {
			assignmentRole := firstText(assignment, "role_hypothesis", "role")
			if assignmentRole == "" {
				assignmentRole = role
			}
			add(firstText(assignment, "track_id", "id"), assignmentRole, "tom_assignment", confidenceOverride(base, assignment))
		}
		for _, trackID := range stringSlice(group["track_ids"]) {
			add(trackID, role, "tom_manifest_group", base)
		}
	}
	return out
}

func collectMOMRows(mom, observation map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	addRows := func(rows []map[string]any) {
		for _, row := range rows {
			id := firstText(row, "track_id", "id", "target_track_id")
			if id == "" {
				continue
			}
			if out[id] == nil {
				out[id] = map[string]any{}
			}
			for key, value := range row {
				if !emptyValue(value) {
					out[id][key] = value
				}
			}
		}
	}
	relation := mapValue(mom["multitrack_relation"])
	addRows(rowsValue(relation["compared_tracks"]))
	for _, occupancy := range rowsValue(relation["band_occupancy"]) {
		addRows(rowsValue(occupancy["leaders"]))
	}
	for _, source := range []map[string]any{
		observation,
		mapValue(observation["observation"]),
		mapValue(mapValue(observation["observation"])["project_package"]),
		mapValue(observation["project_package"]),
	} {
		addRows(rowsValue(source["tracks"]))
	}
	return out
}

func collectMOMStaticLevelRows(mom map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	relation := mapValue(mom["static_level_relationship"])
	for _, row := range rowsValue(relation["tracks"]) {
		trackID := firstText(row, "track_id", "id")
		if trackID != "" {
			out[trackID] = row
		}
	}
	return out
}

func compactMOM(mom map[string]any) map[string]any {
	if len(mom) == 0 {
		return nil
	}
	out := compactMap(mom, "mom_version", "observation_id", "mix_session_id", "intent")
	if trust := mapValue(mom["trust_quality"]); len(trust) > 0 {
		out["trust_quality"] = compactMap(trust, "overall_status", "can_support_suggestion", "can_support_action_preflight", "freshness", "blocked_reasons", "limitations", "evidence_refs")
	}
	if profile := mapValue(mom["project_mix_profile"]); len(profile) > 0 {
		out["project_mix_profile"] = compactMap(profile, "status", "track_count", "level_overview", "limitations", "evidence_refs")
	}
	if relation := mapValue(mom["multitrack_relation"]); len(relation) > 0 {
		out["multitrack_relation"] = compactMap(relation, "status", "track_count", "level_distribution", "limitations", "evidence_refs")
	}
	if relation := mapValue(mom["static_level_relationship"]); len(relation) > 0 {
		out["static_level_relationship"] = compactMap(relation, "schema_version", "status", "freshness", "project_cut_ref", "coverage", "limitations", "evidence_refs")
	}
	return out
}

func compactTOM(tom map[string]any) map[string]any {
	if len(tom) == 0 {
		return nil
	}
	out := compactMap(tom, "tom_version", "status", "organization_summary", "naming_signal_report", "limitations")
	groups := rowsValue(tom["group_proposals"])
	compact := make([]map[string]any, 0, minInt(len(groups), 16))
	for i, group := range groups {
		if i >= 16 {
			break
		}
		compact = append(compact, compactMap(group, "group_id", "label", "role_hypothesis", "confidence", "confidence_score", "track_count", "needs_confirmation"))
	}
	if len(compact) > 0 {
		out["group_proposals"] = compact
	}
	return out
}

func momActionBlocked(mom map[string]any) (blocked, explicit bool) {
	trust := mapValue(mom["trust_quality"])
	value, ok := trust["can_support_action_preflight"]
	if !ok {
		return false, false
	}
	return !boolValue(value), true
}

func b2MOMRelationReadiness(mom map[string]any) (string, string, int) {
	if len(mom) == 0 {
		return capabilityruntime.ConditionMissing, "MOM multitrack relation evidence is missing", 0
	}
	relation := mapValue(mom["multitrack_relation"])
	compared := rowsValue(relation["compared_tracks"])
	if len(relation) == 0 || len(compared) == 0 {
		return capabilityruntime.ConditionMissing, "MOM does not contain a full-project multitrack relation inventory", 0
	}
	status := strings.ToLower(firstText(relation, "status", "freshness"))
	trust := mapValue(mom["trust_quality"])
	freshness := strings.ToLower(firstText(trust, "freshness", "relation_freshness"))
	for _, value := range []string{status, freshness} {
		if strings.Contains(value, "stale") || strings.Contains(value, "suspect") || strings.Contains(value, "invalid") {
			return capabilityruntime.ConditionBlocked, "MOM multitrack relation evidence is stale or suspect", len(compared)
		}
	}
	return capabilityruntime.ConditionReady, "MOM multitrack relation inventory is available for B2 relationship reasoning", len(compared)
}

func observationID(observation, mom map[string]any) string {
	for _, source := range []map[string]any{observation, mapValue(observation["observation"]), mom} {
		if value := firstText(source, "observation_id", "id"); value != "" {
			return value
		}
	}
	return ""
}

func acousticField(key string) bool {
	switch key {
	case "lufs_i", "integrated_lufs", "loudness_lufs", "effective_static_rms_dbfs", "rms_dbfs", "rms_db", "level_db", "active_rms_dbfs", "active_level_db", "peak_dbfs", "peak_db", "headroom_db", "headroom":
		return true
	default:
		return false
	}
}

func roleFunction(role string) string {
	switch normalizeRole(role) {
	case "lead_vocal", "lead_instrument":
		return "foreground"
	case "drums", "kick", "snare", "percussion":
		return "rhythm_anchor"
	case "bass", "low_synth":
		return "low_end_anchor"
	case "strings", "synth", "pad", "guitar", "keys", "piano":
		return "harmonic_bed"
	case "backing_vocal", "support_instrument":
		return "support"
	case "effect", "transition":
		return "effects"
	default:
		return ""
	}
}

func normalizeRole(role string) string {
	role = strings.NewReplacer(" ", "_", "-", "_", "/", "_").Replace(strings.ToLower(strings.TrimSpace(role)))
	switch role {
	case "vocal", "vocals", "lead_voice", "voice", "main_vocal":
		return "lead_vocal"
	case "backing_vocals", "background_vocal", "bgv", "bvox":
		return "backing_vocal"
	case "drums_or_percussion", "drum", "drumkit", "drum_kit":
		return "drums"
	case "synth_or_pad", "keys_or_synth", "synths_or_keys":
		return "synth"
	case "keys_or_piano":
		return "keys"
	case "effect_or_transition", "fx", "sfx":
		return "effect"
	default:
		return role
	}
}

func inferRole(name string) (string, float64) {
	name = strings.ToLower(strings.TrimSpace(name))
	checks := []struct {
		role   string
		tokens []string
	}{
		{"backing_vocal", []string{"backing vocal", "background vocal", "bgv", "bvox", "和声", "和聲"}},
		{"lead_vocal", []string{"lead vocal", "main vocal", "vocal", "vox", "主唱", "人声", "人聲"}},
		{"kick", []string{"kick", "bd", "底鼓"}},
		{"snare", []string{"snare", "军鼓", "軍鼓"}},
		{"percussion", []string{"percussion", "perc", "shaker", "tamb", "打击", "打擊"}},
		{"drums", []string{"drum", "overhead", "room", "鼓"}},
		{"bass", []string{"bass", "贝斯", "貝斯"}},
		{"strings", []string{"string", "violin", "viola", "cello", "弦乐", "弦樂"}},
		{"pad", []string{"pad", "铺底", "鋪底"}},
		{"synth", []string{"synth", "合成器"}},
		{"guitar", []string{"guitar", "gtr", "吉他"}},
		{"piano", []string{"piano", "钢琴", "鋼琴"}},
		{"keys", []string{"keys", "keyboard", "键盘", "鍵盤"}},
		{"effect", []string{"fx", "sfx", "riser", "impact", "效果"}},
		{"lead_instrument", []string{"lead", "solo", "主奏"}},
	}
	for _, check := range checks {
		for _, token := range check.tokens {
			if strings.Contains(name, token) {
				return check.role, 0.72
			}
		}
	}
	return "unknown", 0
}

func confidence(row map[string]any) float64 {
	if value, ok := explicitConfidence(row); ok {
		return value
	}
	return 0.65
}

func confidenceOverride(base float64, row map[string]any) float64 {
	if value, ok := explicitConfidence(row); ok && value > base {
		return value
	}
	return base
}

func explicitConfidence(row map[string]any) (float64, bool) {
	if value, ok := firstNumber(row, "confidence_score"); ok {
		return clamp(*value, 0, 1), true
	}
	switch strings.ToLower(firstText(row, "confidence")) {
	case "high":
		return 0.9, true
	case "medium":
		return 0.7, true
	case "low":
		return 0.4, true
	default:
		return 0, false
	}
}

func stableID(prefix string, values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}

func mapValue(value any) map[string]any {
	row, _ := value.(map[string]any)
	return row
}

func rowsValue(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row := mapValue(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func firstText(row map[string]any, keys ...string) string {
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

func firstNumber(row map[string]any, keys ...string) (*float64, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		var number float64
		switch typed := value.(type) {
		case float64:
			number = typed
		case float32:
			number = float64(typed)
		case int:
			number = float64(typed)
		case int64:
			number = float64(typed)
		default:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			if err != nil {
				continue
			}
			number = parsed
		}
		if !math.IsNaN(number) && !math.IsInf(number, 0) {
			return &number, true
		}
	}
	return nil, false
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "ready":
			return true
		}
	}
	return false
}

func explicitBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
	}
	return false, false
}

func numberPtr(value float64) *float64 { return &value }

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func compactMap(row map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && !emptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func emptyValue(value any) bool {
	text := strings.TrimSpace(fmt.Sprint(value))
	return value == nil || text == "" || text == "<nil>"
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return uniqueStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return uniqueStrings(out)
	case string:
		return uniqueStrings(strings.Split(typed, ","))
	default:
		return nil
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func compactText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return string(runes)
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func round3(value float64) float64 { return math.Round(value*1000) / 1000 }

func ratio(known, total int) float64 {
	if total <= 0 {
		return 0
	}
	return round3(float64(known) / float64(total))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
