package mom

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func Build(input Input) Projection {
	intent := ResolveIntent(input.Args, input.Intent)
	intentPolicy := buildIntentPolicy(intent)
	project := buildProjectStructure(input)
	projectMixProfile := buildProjectMixProfile(input)
	multitrackRelation := buildMultitrackRelation(input, projectMixProfile)
	staticLevelRelationship := buildStaticLevelRelationship(input)
	frequencyRelationship := FrequencyRelationship{}
	var frequencyRelationshipProjection *FrequencyRelationship
	if intent == IntentProjectFrequencyObservation {
		frequencyRelationship = buildFrequencyRelationship(input)
		frequencyRelationshipProjection = &frequencyRelationship
	}
	layers := Layers{
		BasicEnergy:            buildBasicEnergy(input),
		TimbreFrequency:        buildTimbreFrequency(input, intent),
		SpaceStereo:            buildSpaceStereo(input, intent),
		TimeDynamicsStructure:  buildTimeDynamicsStructure(input),
		MultitrackRelationship: multitrackRelationLayer(multitrackRelation),
		ABResultComparison:     buildABResultComparison(input),
	}
	trust := buildTrustQuality(input, intent, project, projectMixProfile, multitrackRelation, frequencyRelationship, layers)
	proj := Projection{
		MOMVersion:              Version,
		Intent:                  intent,
		IntentPolicy:            intentPolicy,
		ObservationID:           input.ObservationID,
		MixSessionID:            input.MixSessionID,
		ProjectStructure:        project,
		ProjectMixProfile:       projectMixProfile,
		MultitrackRelation:      multitrackRelation,
		StaticLevelRelationship: staticLevelRelationship,
		FrequencyRelationship:   frequencyRelationshipProjection,
		Layers:                  layers,
		TrustQuality:            trust,
	}
	proj.LLMContext = BuildLLMContext(proj)
	return proj
}

func buildProjectStructure(input Input) ProjectStructure {
	identity := sourceIdentity(input)
	projectStatus := StatusFromSource(firstNonEmpty(text(input.ProjectPackage["status"]), text(input.EnvironmentPackage["status"]), "partial"))
	out := ProjectStructure{
		Status:         projectStatus,
		Freshness:      FreshnessForStatus(projectStatus),
		Source:         "project.shadow+acoustic_package_status",
		ProjectID:      firstNonEmpty(text(identity["project_id"]), text(input.ProjectPackage["project_id"])),
		SessionID:      firstNonEmpty(text(identity["session_id"]), input.MixSessionID),
		TrackID:        firstNonEmpty(text(identity["track_id"]), text(input.TargetRef["id"])),
		ClipID:         text(identity["clip_id"]),
		GUIID:          firstNonEmpty(text(input.TargetRef["gui_id"]), text(input.TargetRef["id"])),
		SourcePath:     firstNonEmpty(text(identity["source_path"]), text(identity["file_path"])),
		SourceRevision: text(identity["source_revision"]),
		ClipRevision:   text(identity["clip_revision"]),
		RenderRevision: text(identity["render_revision"]),
		DurationSec:    firstPositiveNumber(identity, "duration_seconds", "duration", "total_duration"),
		SampleRate:     firstPositiveNumber(identity, "sample_rate", "sample_rate_hz"),
		ChannelCount:   int(firstPositiveNumber(identity, "channel_count", "channels")),
		TargetRef:      compactMap(input.TargetRef, "kind", "id", "label", "source", "confidence"),
		ListenScope:    compactMap(input.ListenScope, "time", "source"),
		TimeRuler:      compactMap(input.TimeRuler, "duration_seconds", "segment_seconds", "frame_seconds", "tempo_bpm", "bar_map_available"),
		EvidenceRefs:   evidenceRefs(observationRef(input), "mix.read:project.static.summary", "mix.read:project.limitations", "acoustic_package_status"),
	}
	if out.DurationSec <= 0 {
		out.DurationSec = firstPositiveNumber(input.TimeRuler, "duration_seconds")
	}
	if out.ProjectID == "" {
		out.ProjectID = firstNonEmpty(text(input.EnvironmentPackage["project_id"]), "current")
	}
	if out.TrackID == "" && strings.EqualFold(text(input.TargetRef["kind"]), "track") {
		out.TrackID = text(input.TargetRef["id"])
	}
	if limits := stringsFromAny(input.ProjectPackage["limitations"]); len(limits) > 0 {
		out.Limitations = limits
	}
	return out
}

func buildBasicEnergy(input Input) Layer {
	current := mapValue(input.MixPackage["current_metrics"])
	waveform := mapValue(current["waveform"])
	status := StatusFromSource(text(waveform["status"]))
	if status == StatusMissing && (input.GlobalSummary["peak_dbfs"] != nil || input.GlobalSummary["rms_dbfs"] != nil) {
		waveform = compactMap(input.GlobalSummary, "peak_dbfs", "rms_dbfs", "crest_db")
		status = StatusPartial
	}
	facts := compactMap(waveform, "status", "source", "rms", "peak_abs", "rms_dbfs", "peak_dbfs", "headroom_db", "crest_db", "reason", "updated_at")
	facts["silence_risk"] = silenceRisk(waveform)
	facts["clip_risk"] = clipRisk(waveform)
	return Layer{
		Status:       status,
		Freshness:    FreshnessForStatus(status),
		Source:       firstNonEmpty(text(waveform["source"]), "l1_static.peak_rms_summary"),
		Summary:      energySummary(waveform, status),
		Facts:        facts,
		EvidenceRefs: evidenceRefs(observationRef(input), trackReadKey(input, "fast.levels"), acousticFeatureRef("l1_static", "peak_rms_summary")),
	}
}

func buildTimbreFrequency(input Input, intent string) Layer {
	current := mapValue(input.MixPackage["current_metrics"])
	realtime := mapValue(input.MixPackage["realtime_metrics"])
	l3 := compactBandEnergy(mapValue(current["band_energy"]))
	l2 := compactBandEnergy(mapValue(realtime["band_energy"]))
	renderProbe := l2RenderProbe(input)
	l2ProbeBands := compactBandEnergy(renderProbe)
	primary := l3
	source := "l3_deep.band_energy_summary"
	refs := evidenceRefs(observationRef(input), trackReadKey(input, "slow.band_energy.summary"), acousticFeatureRef("l3_deep", "band_energy_summary"))
	if intent == IntentRealtimeBandStereoObservation {
		if renderProbeUsable(renderProbe) {
			primary = l2ProbeBands
			source = "l2_realtime.render_probe"
			refs = evidenceRefs(observationRef(input), trackReadKey(input, "l2.render_probe.band_energy"), acousticFeatureRef("l2_realtime", "render_probe"), l2RenderProbeEvidenceRef(renderProbe))
		} else {
			primary = l2
			source = "l2_realtime.realtime_spectrum"
			refs = evidenceRefs(observationRef(input), trackReadKey(input, "realtime.band_energy.summary"), acousticFeatureRef("l2_realtime", "realtime_spectrum"))
		}
	}
	status := StatusFromSource(text(primary["status"]))
	facts := map[string]any{
		"primary_layer":       layerNameForIntent(intent),
		"band_energy_summary": primary,
		"spectral_coverage":   compactSpectral(input),
	}
	limitations := []string{}
	if intent == IntentGeneralBandStereoObservation && StatusFromSource(text(l2["status"])) == StatusReady {
		facts["l2_realtime_status"] = statusOnlyRealtime(l2)
		limitations = append(limitations, "l2_realtime_is_secondary_for_general_observation")
	}
	if intent == IntentGeneralBandStereoObservation && renderProbeHasStatus(renderProbe) {
		facts["l2_render_probe_status"] = statusOnlyRealtime(renderProbe)
		limitations = append(limitations, "l2_render_probe_is_secondary_for_general_observation")
	}
	if intent != IntentRealtimeBandStereoObservation && status == StatusMissing && StatusFromSource(text(l2["status"])) == StatusReady {
		status = StatusPartial
		limitations = append(limitations, "l3_band_summary_missing_l2_available_only_as_secondary")
	}
	if status == StatusReady || status == StatusPartial {
		facts["band_energy_compact"] = compactBands(mapValue(primary["bands"]))
	}
	return Layer{
		Status:       status,
		Freshness:    FreshnessForStatus(status),
		Source:       source,
		Summary:      bandSummary(primary, status),
		Facts:        facts,
		EvidenceRefs: refs,
		Limitations:  limitations,
	}
}

func buildSpaceStereo(input Input, intent string) Layer {
	current := mapValue(input.MixPackage["current_metrics"])
	realtime := mapValue(input.MixPackage["realtime_metrics"])
	l3 := compactStereo(mapValue(current["stereo_relation"]))
	l2 := compactStereo(mapValue(realtime["stereo_relation"]))
	renderProbe := l2RenderProbe(input)
	l2ProbeStereo := compactStereo(renderProbe)
	primary := l3
	source := "l3_deep.stereo_relation_summary"
	refs := evidenceRefs(observationRef(input), trackReadKey(input, "slow.stereo.summary"), acousticFeatureRef("l3_deep", "stereo_relation_summary"))
	if intent == IntentRealtimeBandStereoObservation {
		if renderProbeUsable(renderProbe) {
			primary = l2ProbeStereo
			source = "l2_realtime.render_probe"
			refs = evidenceRefs(observationRef(input), trackReadKey(input, "l2.render_probe.stereo"), acousticFeatureRef("l2_realtime", "render_probe"), l2RenderProbeEvidenceRef(renderProbe))
		} else {
			primary = l2
			source = "l2_realtime.realtime_stereo_correlation"
			refs = evidenceRefs(observationRef(input), trackReadKey(input, "realtime.stereo.summary"), acousticFeatureRef("l2_realtime", "realtime_stereo_correlation"))
		}
	}
	status := StatusFromSource(text(primary["status"]))
	facts := map[string]any{
		"primary_layer":             layerNameForIntent(intent),
		"stereo_relation_summary":   primary,
		"width_center_tendency":     widthCenterTendency(primary),
		"phase_risk":                phaseRisk(primary),
		"l3_l2_are_separate_layers": true,
	}
	limitations := []string{}
	if intent == IntentGeneralBandStereoObservation && StatusFromSource(text(l2["status"])) == StatusReady {
		facts["l2_realtime_status"] = statusOnlyRealtime(l2)
		limitations = append(limitations, "l2_realtime_is_secondary_for_general_observation")
	}
	if intent == IntentGeneralBandStereoObservation && renderProbeHasStatus(renderProbe) {
		facts["l2_render_probe_status"] = statusOnlyRealtime(renderProbe)
		limitations = append(limitations, "l2_render_probe_is_secondary_for_general_observation")
	}
	if intent != IntentRealtimeBandStereoObservation && status == StatusMissing && StatusFromSource(text(l2["status"])) == StatusReady {
		status = StatusPartial
		limitations = append(limitations, "l3_stereo_summary_missing_l2_available_only_as_secondary")
	}
	return Layer{
		Status:       status,
		Freshness:    FreshnessForStatus(status),
		Source:       source,
		Summary:      stereoSummary(primary, status),
		Facts:        facts,
		EvidenceRefs: refs,
		Limitations:  limitations,
	}
}

func buildTimeDynamicsStructure(input Input) Layer {
	current := mapValue(input.MixPackage["current_metrics"])
	rows := rowsFromAny(current["time_energy"])
	loudness := compactLoudness(mapValue(current["loudness"]))
	loudnessStatus := StatusFromSource(text(loudness["status"]))
	status := StatusDeferred
	facts := map[string]any{
		"lufs_analysis":      map[string]any{"status": StatusDeferred, "reason": "phase_5_deferred"},
		"masking_analysis":   map[string]any{"status": StatusDeferred, "reason": "phase_5_deferred"},
		"structure_analysis": map[string]any{"status": StatusDeferred, "reason": "phase_5_deferred"},
		"reference_match":    map[string]any{"status": StatusDeferred, "reason": "phase_5_deferred"},
	}
	if len(rows) > 0 {
		status = StatusPartial
		facts["time_energy"] = map[string]any{"status": StatusReady, "row_count": len(rows)}
	}
	if loudnessStatus == StatusReady || loudnessStatus == StatusPartial || loudnessStatus == StatusApprox {
		status = StatusPartial
		facts["loudness_summary"] = loudness
		reason := "commercial_lufs_analysis_deferred"
		if loudnessIsApproximate(loudness) || loudnessStatus == StatusApprox {
			reason = "commercial_lufs_analysis_deferred_l3_loudness_approximate"
		}
		facts["lufs_analysis"] = map[string]any{"status": StatusDeferred, "reason": reason, "loudness_summary_status": loudnessStatus}
	} else if loudnessStatus == StatusSuspect {
		status = StatusSuspect
		facts["loudness_summary"] = loudness
		facts["lufs_analysis"] = map[string]any{"status": StatusDeferred, "reason": "commercial_lufs_analysis_deferred_l3_loudness_suspect", "loudness_summary_status": loudnessStatus}
	}
	source := "mom.v1.schema"
	summary := "Time/dynamics/structure schema is present; LUFS, masking, structure, and reference matching are deferred in v1."
	if len(mapValue(facts["loudness_summary"])) > 0 {
		source = "l3_deep.loudness_summary"
		summary = "Initial L3 loudness summary is available; full LUFS, masking, structure, and reference matching remain deferred."
	}
	return Layer{
		Status:       status,
		Freshness:    FreshnessForStatus(status),
		Source:       source,
		Summary:      summary,
		Facts:        facts,
		EvidenceRefs: evidenceRefs(observationRef(input), trackReadKey(input, "slow.time_energy.summary"), trackReadKey(input, "slow.loudness.summary"), acousticFeatureRef("l3_deep", "loudness_summary"), text(loudness["evidence_ref"])),
		Limitations:  []string{"lufs_analysis_deferred", "masking_analysis_deferred", "structure_analysis_deferred", "reference_match_deferred"},
	}
}

func buildMultitrackRelationship(input Input) Layer {
	return multitrackRelationLayer(buildMultitrackRelation(input, buildProjectMixProfile(input)))
}

func buildABResultComparison(input Input) Layer {
	current := mapValue(input.MixPackage["current_metrics"])
	abResult := mapValue(current["ab_result"])
	if len(abResult) > 0 {
		status := StatusFromSource(text(abResult["status"]))
		facts := map[string]any{
			"ab_result": compactABResult(abResult),
		}
		refs := evidenceRefs(
			observationRef(input),
			text(abResult["before_evidence_ref"]),
			text(abResult["after_evidence_ref"]),
			"mix.read:observation.ab_result.latest",
		)
		return Layer{
			Status:       status,
			Freshness:    FreshnessForStatus(status),
			Source:       "mixboard.ab_result",
			Summary:      abResultLayerSummary(abResult, status),
			Facts:        facts,
			EvidenceRefs: refs,
			Limitations:  abResultLimitations(abResult, status),
		}
	}
	delta := mapValue(current["before_after_delta"])
	status := StatusFromSource(text(delta["status"]))
	if status == StatusMissing || status == StatusDeferred {
		status = StatusDeferred
	}
	facts := map[string]any{
		"before_after_delta": compactMap(delta, "status", "reason", "summary", "summary_tags", "before_observation_id", "after_observation_id", "updated_at"),
	}
	return Layer{
		Status:       status,
		Freshness:    FreshnessForStatus(status),
		Source:       "mixboard.before_after_delta",
		Summary:      "AB comparison interface is present; it becomes ready when a previous observation in the same session exists.",
		Facts:        facts,
		EvidenceRefs: evidenceRefs(observationRef(input), "mix.derive:before_after", "mix.read:observation.before_after.latest"),
	}
}

func compactABResult(ab map[string]any) map[string]any {
	if len(ab) == 0 {
		return nil
	}
	out := compactMap(ab,
		"schema_version", "status", "reason", "summary", "summary_tags", "action_summary",
		"tap_point", "render_mode", "before_observation_id", "after_observation_id",
		"before_render_revision", "after_render_revision", "before_evidence_ref",
		"after_evidence_ref", "quality_gates", "updated_at")
	if delta := compactABDelta(mapValue(ab["delta"])); len(delta) > 0 {
		out["delta"] = delta
	}
	return out
}

func compactABDelta(delta map[string]any) map[string]any {
	if len(delta) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, key := range []string{"levels", "bands", "stereo", "risk"} {
		if value, ok := delta[key]; ok && !empty(value) {
			out[key] = value
		}
	}
	return out
}

func abResultLayerSummary(ab map[string]any, status string) string {
	if summary := text(ab["summary"]); summary != "" {
		return summary
	}
	switch StatusFromSource(status) {
	case StatusReady:
		return "AB result is ready from same-tap L2 Render Probe observations."
	case StatusStale:
		return "AB result is stale; after observation did not provide a new render revision."
	case StatusSuspect:
		return "AB result is suspect; quality gates or tap point checks did not pass."
	case StatusMissing:
		return "AB result is missing until before/after L2 Render Probe observations are available."
	default:
		return "AB result layer is present as a compact comparison surface."
	}
}

func abResultLimitations(ab map[string]any, status string) []string {
	limitations := []string{}
	switch StatusFromSource(status) {
	case StatusReady:
	default:
		if reason := text(ab["reason"]); reason != "" {
			limitations = append(limitations, reason)
		}
		limitations = append(limitations, "do_not_infer_ab_result_without_ready_same_tap_render_probe")
	}
	return limitations
}

func buildTrustQuality(input Input, intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers) TrustQuality {
	identity := sourceIdentity(input)
	renderProbe := l2RenderProbe(input)
	overall := rollupStatus(intent, layers, project)
	if intent == IntentProjectMultitrackObservation {
		overall = rollupMultitrackStatus(project, profile, relation)
	} else if intent == IntentProjectFrequencyObservation {
		overall = rollupStatusFromList(project.Status, frequency.Status)
	} else if intent == IntentABResultObservation {
		overall = StatusFromSource(layers.ABResultComparison.Status)
	}
	refs := append([]string{observationRef(input)}, allEvidenceRefs(Projection{
		ProjectStructure:      project,
		ProjectMixProfile:     profile,
		MultitrackRelation:    relation,
		FrequencyRelationship: &frequency,
		Layers:                layers,
	})...)
	l2TapPoint := l2TapPoint(input)
	limitations := []string{}
	l2Limitations := []string{}
	if intent != IntentProjectFrequencyObservation && (l2TapPoint == "" || strings.EqualFold(l2TapPoint, "unknown_live_meter")) {
		l2TapPoint = firstNonEmpty(l2TapPoint, "unknown_live_meter")
		l2Limitations = append(l2Limitations, "l2_tap_point_not_fully_closed_loop")
		limitations = append(limitations, l2Limitations...)
	} else if intent == IntentProjectFrequencyObservation {
		l2TapPoint = ""
		limitations = append(limitations, frequency.Limitations...)
	}
	required := IntentRequiredLayers(intent)
	optional := IntentOptionalLayers(intent)
	coverage := trustCoverage(input, intent, profile, relation, frequency)
	deferred := deferredLayers(intent, profile, relation, layers)
	gates := trustQualityGates(intent, project, profile, relation, frequency, layers, l2TapPoint)
	approximateFields := approximateFields(input, layers)
	suspectFields, staleFields, missingFields := trustStatusFields(intent, project, profile, relation, frequency, layers)
	if len(approximateFields) > 0 {
		gates = append(gates, fmt.Sprintf("approximate_fields:%d", len(approximateFields)))
	}
	if len(suspectFields) > 0 {
		gates = append(gates, fmt.Sprintf("suspect_fields:%d", len(suspectFields)))
	}
	if len(staleFields) > 0 {
		gates = append(gates, fmt.Sprintf("stale_fields:%d", len(staleFields)))
	}
	if len(missingFields) > 0 {
		gates = append(gates, fmt.Sprintf("missing_fields:%d", len(missingFields)))
	}
	sourceRevision := firstNonEmpty(project.SourceRevision, text(identity["source_revision"]))
	clipRevision := firstNonEmpty(project.ClipRevision, text(identity["clip_revision"]))
	renderRevision := firstNonEmpty(project.RenderRevision, text(identity["render_revision"]), text(renderProbe["render_revision"]))
	sourceRevisionStatus := revisionStatus(sourceRevision)
	clipRevisionStatus := revisionStatus(clipRevision)
	renderRevisionStatus := revisionStatus(renderRevision)
	canObserve := canSupportObservation(intent, project, profile, relation, frequency, layers)
	canSuggest := canObserve && len(suspectFields) == 0 && len(staleFields) == 0
	canAction := canSupportActionPreflight(project, layers, refs)
	if intent == IntentProjectFrequencyObservation {
		canAction = false
	}
	canAB := canSupportABResult(layers)
	blockedReasons := blockedReasonsForTrust(intent, canObserve, canSuggest, canAction, canAB, suspectFields, staleFields, missingFields)
	return TrustQuality{
		SchemaVersion:             "mom_trust_quality.v1",
		OverallStatus:             overall,
		RequiredLayers:            required,
		OptionalLayers:            optional,
		DeferredLayers:            deferred,
		Coverage:                  coverage,
		QualityGates:              evidenceRefs(gates...),
		BlockedReasons:            blockedReasons,
		ApproximateFields:         approximateFields,
		SuspectFields:             suspectFields,
		StaleFields:               staleFields,
		MissingFields:             missingFields,
		SourceRevisionStatus:      sourceRevisionStatus,
		ClipRevisionStatus:        clipRevisionStatus,
		RenderRevisionStatus:      renderRevisionStatus,
		CanSupportObservation:     canObserve,
		CanSupportSuggestion:      canSuggest,
		CanSupportActionPreflight: canAction,
		CanSupportABResult:        canAB,
		Freshness:                 FreshnessForStatus(overall),
		Source:                    "mom_projection_from_dad_v1_2_acoustic_package_and_mixboard",
		L2TapPoint:                l2TapPoint,
		L2Limitations:             l2Limitations,
		Limitations:               evidenceRefs(append(append([]string{}, limitations...), blockedReasons...)...),
		EvidenceRefs:              evidenceRefs(refs...),
		UpdatedAt:                 firstNonEmpty(text(input.AcousticPackageStatus["updated_at"]), input.CreatedAt),
	}
}

func revisionStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return StatusMissing
	}
	return StatusReady
}

func trustStatusFields(intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers) ([]string, []string, []string) {
	rows := trustStatusRowsForIntent(intent, project, profile, relation, frequency, layers)
	suspect := []string{}
	stale := []string{}
	missing := []string{}
	for _, row := range rows {
		if row.status == "not_applicable_single_track" {
			continue
		}
		switch StatusFromSource(row.status) {
		case StatusSuspect:
			suspect = append(suspect, row.name)
		case StatusStale:
			stale = append(stale, row.name)
		case StatusMissing:
			missing = append(missing, row.name)
		}
	}
	return suspect, stale, missing
}

func trustStatusRowsForIntent(intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers) []struct {
	name   string
	status string
} {
	base := []struct {
		name   string
		status string
	}{
		{"project_structure", project.Status},
	}
	switch intent {
	case IntentProjectMultitrackObservation:
		return append(base,
			struct {
				name   string
				status string
			}{"project_mix_profile", profile.Status},
			struct {
				name   string
				status string
			}{"multitrack_relation", relation.Status},
		)
	case IntentProjectFrequencyObservation:
		return append(base, struct {
			name   string
			status string
		}{"frequency_relationship", frequency.Status})
	case IntentABResultObservation:
		return append(base, struct {
			name   string
			status string
		}{"ab_result_comparison", layers.ABResultComparison.Status})
	default:
		return append(base,
			struct {
				name   string
				status string
			}{"timbre_frequency", layers.TimbreFrequency.Status},
			struct {
				name   string
				status string
			}{"space_stereo", layers.SpaceStereo.Status},
		)
	}
}

func allTrustStatusRows(project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, layers Layers) []struct {
	name   string
	status string
} {
	return []struct {
		name   string
		status string
	}{
		{"project_structure", project.Status},
		{"project_mix_profile", profile.Status},
		{"multitrack_relation", relation.Status},
		{"basic_energy", layers.BasicEnergy.Status},
		{"timbre_frequency", layers.TimbreFrequency.Status},
		{"space_stereo", layers.SpaceStereo.Status},
		{"time_dynamics_structure", layers.TimeDynamicsStructure.Status},
		{"ab_result_comparison", layers.ABResultComparison.Status},
	}
}

func approximateFields(input Input, layers Layers) []string {
	out := []string{}
	collectApproximateFields("mix.current.loudness", mapValue(mapValue(input.MixPackage["current_metrics"])["loudness"]), &out)
	collectApproximateFields("time_dynamics_structure", layers.TimeDynamicsStructure.Facts, &out)
	return evidenceRefs(out...)
}

func collectApproximateFields(prefix string, value any, out *[]string) {
	row := mapValue(value)
	if len(row) == 0 {
		return
	}
	if boolValue(row["approximate"]) {
		*out = append(*out, prefix)
	}
	if row["approximate_lufs"] != nil {
		*out = append(*out, prefix+".approximate_lufs")
	}
	if algorithm := strings.ToLower(text(row["algorithm"])); strings.Contains(algorithm, "approximate") || strings.Contains(algorithm, "rms_lufs") {
		*out = append(*out, prefix+".algorithm")
	}
	for key, child := range row {
		if key == "quality_evidence" {
			continue
		}
		if childMap := mapValue(child); len(childMap) > 0 {
			collectApproximateFields(prefix+"."+key, childMap, out)
		}
	}
}

func canSupportObservation(intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers) bool {
	for _, status := range requiredLayerStatuses(intent, project, profile, relation, frequency, layers) {
		switch StatusFromSource(status) {
		case StatusReady, StatusPartial, StatusApprox:
			continue
		default:
			return false
		}
	}
	return true
}

func canSupportActionPreflight(project ProjectStructure, layers Layers, refs []string) bool {
	if len(evidenceRefs(refs...)) == 0 {
		return false
	}
	for _, status := range []string{project.Status, layers.TimbreFrequency.Status, layers.SpaceStereo.Status} {
		if StatusFromSource(status) != StatusReady {
			return false
		}
	}
	return true
}

func canSupportABResult(layers Layers) bool {
	return StatusFromSource(layers.ABResultComparison.Status) == StatusReady
}

func requiredLayerStatuses(intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers) []string {
	switch intent {
	case IntentProjectMultitrackObservation:
		if relation.Status == "not_applicable_single_track" {
			return []string{project.Status, profile.Status}
		}
		return []string{project.Status, profile.Status, relation.Status}
	case IntentProjectFrequencyObservation:
		return []string{project.Status, frequency.Status}
	case IntentABResultObservation:
		return []string{project.Status, layers.ABResultComparison.Status}
	default:
		return []string{project.Status, layers.TimbreFrequency.Status, layers.SpaceStereo.Status}
	}
}

func blockedReasonsForTrust(intent string, canObserve, canSuggest, canAction, canAB bool, suspectFields, staleFields, missingFields []string) []string {
	out := []string{}
	if !canObserve {
		out = append(out, "required_observation_evidence_not_ready")
	}
	if !canSuggest {
		if len(suspectFields) > 0 {
			out = append(out, "suspect_data_cannot_support_suggestion")
		}
		if len(staleFields) > 0 {
			out = append(out, "stale_data_cannot_support_suggestion")
		}
	}
	if intent == IntentActionPreflightObservation && !canAction {
		out = append(out, "action_preflight_requires_ready_band_stereo_evidence")
	}
	if intent == IntentABResultObservation && !canAB {
		out = append(out, "ab_result_requires_ready_same_tap_render_probe")
	}
	if len(missingFields) > 0 {
		out = append(out, "missing_fields_limit_current_conclusion")
	}
	return evidenceRefs(out...)
}

func deferredLayers(intent string, profile ProjectMixProfile, relation MultitrackRelation, layers Layers) []string {
	out := []string{}
	for _, row := range []struct {
		name  string
		layer Layer
	}{
		{"basic_energy", layers.BasicEnergy},
		{"time_dynamics_structure", layers.TimeDynamicsStructure},
		{"multitrack_relationship", layers.MultitrackRelationship},
		{"ab_result_comparison", layers.ABResultComparison},
	} {
		if row.name == "ab_result_comparison" && intent != IntentABResultObservation && StatusFromSource(row.layer.Status) == StatusMissing {
			continue
		}
		if row.name == "multitrack_relationship" && intent != IntentProjectMultitrackObservation && StatusFromSource(row.layer.Status) == StatusMissing {
			continue
		}
		status := StatusFromSource(row.layer.Status)
		if status == StatusDeferred || status == StatusMissing {
			out = append(out, row.name)
		}
	}
	if intent == IntentProjectMultitrackObservation {
		if profileStatus := StatusFromSource(profile.Status); profileStatus == StatusDeferred || profileStatus == StatusMissing {
			out = append(out, "project_mix_profile")
		}
		if relationStatus := StatusFromSource(relation.Status); relationStatus == StatusDeferred || relationStatus == StatusMissing {
			out = append(out, "multitrack_relation")
		}
	}
	return out
}

func trustCoverage(input Input, intent string, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship) map[string]any {
	if intent == IntentProjectFrequencyObservation {
		return frequency.Coverage
	}
	tracks := projectTrackRows(input)
	return map[string]any{
		"track_count":                 profile.TrackCount,
		"compared_track_count":        len(profile.ComparedTracks),
		"missing_track_count":         len(relation.MissingTracks),
		"tracks_with_band_energy":     countTracksWithStatus(tracks, "band_energy"),
		"tracks_with_stereo_relation": countTracksWithStatus(tracks, "stereo_relation"),
	}
}

func countTracksWithStatus(tracks []map[string]any, key string) int {
	count := 0
	for _, track := range tracks {
		status := StatusFromSource(text(mapValue(track[key])["status"]))
		if status == StatusReady || status == StatusPartial {
			count++
		}
	}
	return count
}

func trustQualityGates(intent string, project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation, frequency FrequencyRelationship, layers Layers, l2TapPoint string) []string {
	gates := []string{}
	if status := StatusFromSource(project.Status); status != StatusReady {
		gates = append(gates, "project_structure:"+status)
	}
	if intent == IntentProjectMultitrackObservation {
		if status := StatusFromSource(profile.Status); status != StatusReady {
			gates = append(gates, "project_mix_profile:"+status)
		}
		relationStatus := StatusFromSource(relation.Status)
		if relation.Status != "not_applicable_single_track" && relationStatus != StatusReady && relationStatus != StatusMissing {
			gates = append(gates, "multitrack_relation:"+relationStatus)
		} else if relation.Status == "not_applicable_single_track" {
			gates = append(gates, "multitrack_relation:not_applicable_single_track")
		}
		if len(relation.MissingTracks) > 0 {
			gates = append(gates, fmt.Sprintf("missing_tracks:%d", len(relation.MissingTracks)))
		}
		if len(relation.BandConflictCandidates) > 0 {
			gates = append(gates, fmt.Sprintf("band_conflict_candidates:%d", len(relation.BandConflictCandidates)))
		}
		if len(relation.PhaseRiskTracks) > 0 {
			gates = append(gates, fmt.Sprintf("phase_risk_tracks:%d", len(relation.PhaseRiskTracks)))
		}
	}
	if intent == IntentProjectFrequencyObservation {
		gates = append(gates, "frequency_relationship:"+StatusFromSource(frequency.Status))
		gates = append(gates, "frequency_relationship.tap_point:"+firstNonEmpty(frequency.TapPoint, "unknown"))
		if boolValue(frequency.Coverage["decision_tracks_truncated"]) {
			gates = append(gates, "frequency_relationship.decision_tracks_truncated:true")
		}
		if !boolValue(frequency.Coverage["supports_post_fx_compare"]) {
			gates = append(gates, "frequency_relationship.post_fx_compare:false")
		}
	}
	if intent != IntentProjectFrequencyObservation {
		for _, row := range []struct {
			name  string
			layer Layer
		}{
			{"timbre_frequency", layers.TimbreFrequency},
			{"space_stereo", layers.SpaceStereo},
		} {
			if status := StatusFromSource(row.layer.Status); status == StatusStale || status == StatusSuspect || status == StatusMissing {
				gates = append(gates, row.name+":"+status)
			}
		}
	}
	if intent == IntentABResultObservation || StatusFromSource(layers.ABResultComparison.Status) != StatusMissing {
		if status := StatusFromSource(layers.ABResultComparison.Status); status == StatusStale || status == StatusSuspect || status == StatusMissing {
			gates = append(gates, "ab_result_comparison:"+status)
		}
	}
	if intent != IntentProjectFrequencyObservation {
		if l2TapPoint == "" || strings.EqualFold(l2TapPoint, "unknown_live_meter") {
			gates = append(gates, "l2_tap_point:unknown")
		} else {
			gates = append(gates, "l2_tap_point:"+l2TapPoint)
		}
	}
	if intent == IntentABResultObservation || StatusFromSource(layers.ABResultComparison.Status) != StatusMissing {
		ab := mapValue(layers.ABResultComparison.Facts["ab_result"])
		if gatesMap := mapValue(ab["quality_gates"]); len(gatesMap) > 0 {
			for key, value := range gatesMap {
				if b, ok := value.(bool); ok && !b {
					gates = append(gates, "ab_result."+key+":false")
				}
			}
		}
	}
	return gates
}

func rollupMultitrackStatus(project ProjectStructure, profile ProjectMixProfile, relation MultitrackRelation) string {
	projectStatus := StatusFromSource(project.Status)
	profileStatus := StatusFromSource(profile.Status)
	relationStatus := StatusFromSource(relation.Status)
	if relation.Status == "not_applicable_single_track" {
		if projectStatus == StatusReady && profileStatus == StatusReady {
			return StatusPartial
		}
		if projectStatus == StatusSuspect || profileStatus == StatusSuspect {
			return StatusSuspect
		}
		if projectStatus == StatusStale || profileStatus == StatusStale {
			return StatusStale
		}
		return StatusPartial
	}
	return rollupStatusFromList(projectStatus, profileStatus, relationStatus)
}

func rollupStatusFromList(statuses ...string) string {
	ready := 0
	partial := 0
	deferred := 0
	stale := 0
	suspect := 0
	missing := 0
	for _, status := range statuses {
		switch StatusFromSource(status) {
		case StatusReady:
			ready++
		case StatusPartial, StatusApprox:
			partial++
		case StatusDeferred:
			deferred++
		case StatusStale:
			stale++
		case StatusSuspect:
			suspect++
		default:
			missing++
		}
	}
	if suspect > 0 && ready == 0 && partial == 0 {
		return StatusSuspect
	}
	if stale > 0 && ready == 0 && partial == 0 {
		return StatusStale
	}
	if ready == len(statuses) {
		return StatusReady
	}
	if ready > 0 || partial > 0 {
		return StatusPartial
	}
	if deferred > 0 {
		return StatusDeferred
	}
	_ = missing
	return StatusMissing
}

func sourceIdentity(input Input) map[string]any {
	if identity := mapValue(input.AcousticPackageStatus["source_identity"]); len(identity) > 0 {
		return identity
	}
	if len(input.AcousticPackageStatus) > 0 {
		return compactMap(input.AcousticPackageStatus, "project_id", "session_id", "track_id", "clip_id", "source_path", "file_path", "source_hash", "source_fingerprint", "source_revision", "clip_revision", "render_revision", "analyzer_revision", "duration_seconds", "clip_start_seconds", "sample_rate", "channel_count")
	}
	return map[string]any{
		"session_id": input.MixSessionID,
		"track_id":   text(input.TargetRef["id"]),
	}
}

func compactBandEnergy(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": StatusMissing}
	}
	out := compactMap(row, "schema_version", "status", "reason", "source", "source_kind", "layer", "capture_mode", "tap_point", "render_mode", "capture_time", "time_basis", "quality_status", "quality_reason", "source_revision", "clip_revision", "render_revision", "updated_at", "track_id", "clip_id", "request_id", "tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "total_duration", "duration_seconds", "sample_rate", "channel_count", "analyzed_range", "evidence_ref", "derivation_status")
	if quality := compactQualityEvidence(row["quality_evidence"]); len(quality) > 0 {
		out["quality_evidence"] = quality
	}
	if bands := compactBands(mapValue(row["bands"])); len(bands) > 0 {
		out["bands"] = bands
	}
	return out
}

func compactBands(bands map[string]any) map[string]any {
	out := map[string]any{}
	for _, id := range []string{"sub", "bass", "low_mid", "mid", "presence", "air"} {
		band := mapValue(bands[id])
		if len(band) == 0 {
			continue
		}
		out[id] = compactMap(band, "status", "unit_energy", "energy_db", "min_hz", "max_hz", "coverage_ratio")
	}
	return out
}

func compactStereo(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": StatusMissing}
	}
	out := compactMap(row, "schema_version", "status", "reason", "source", "source_kind", "layer", "capture_mode", "tap_point", "render_mode", "capture_time", "time_basis", "quality_status", "quality_reason", "source_revision", "clip_revision", "render_revision", "updated_at", "track_id", "clip_id", "request_id", "coverage_seconds", "coverage_ratio", "total_duration", "duration_seconds", "sample_rate", "channel_count", "analyzed_range", "evidence_ref", "derivation_status", "left_level_db", "right_level_db", "balance_db", "balance_unit", "balance_state", "phase_deviation", "phase_negative_ratio", "correlation_estimate", "correlation_state", "bin_count", "sample_count", "phase_sample_count")
	if quality := compactQualityEvidence(row["quality_evidence"]); len(quality) > 0 {
		out["quality_evidence"] = quality
	}
	return out
}

func compactLoudness(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": StatusMissing}
	}
	out := compactMap(row,
		"schema_version", "status", "reason", "source", "source_kind", "layer", "quality_status",
		"quality_reason", "quality_reasons", "source_revision", "clip_revision",
		"render_revision", "analyzer_revision", "analyzer_version", "updated_at", "track_id", "clip_id",
		"request_id", "coverage_ratio", "duration_seconds", "sample_rate", "channel_count", "channels",
		"expected_sample_count", "analyzed_sample_count", "nonzero_count", "sum_abs", "max_abs",
		"nan_count", "inf_count", "evidence_ref", "peak", "peak_abs", "peak_dbfs", "rms",
		"rms_dbfs", "integrated_lufs", "approximate_lufs", "approximate", "algorithm", "crest_factor", "crest_db")
	if quality := compactQualityEvidence(row["quality_evidence"]); len(quality) > 0 {
		out["quality_evidence"] = quality
	}
	if loudnessIsApproximate(out) {
		if out["approximate_lufs"] == nil && out["integrated_lufs"] != nil {
			out["approximate_lufs"] = out["integrated_lufs"]
		}
		delete(out, "integrated_lufs")
		out["approximate"] = true
		if text(out["algorithm"]) == "" {
			out["algorithm"] = "approximate_rms_lufs_v1"
		}
	}
	return out
}

func compactQualityEvidence(value any) map[string]any {
	row := mapValue(value)
	if len(row) == 0 {
		return nil
	}
	return compactMap(row,
		"status", "reason", "coverage", "nonzero", "sum_abs", "max_abs", "nan_inf_count",
		"nan_count", "inf_count", "latency_compensated", "tail_captured", "deterministic")
}

func loudnessIsApproximate(row map[string]any) bool {
	if boolValue(row["approximate"]) {
		return true
	}
	algorithm := strings.ToLower(text(row["algorithm"]))
	return strings.Contains(algorithm, "approximate") || strings.Contains(algorithm, "rms_lufs")
}

func statusOnlyRealtime(row map[string]any) map[string]any {
	if len(row) == 0 {
		return map[string]any{"status": StatusMissing}
	}
	out := compactMap(row, "status", "reason", "source", "source_kind", "layer", "capture_mode", "tap_point", "render_mode", "time_basis", "quality_status", "quality_reason", "source_revision", "clip_revision", "render_revision", "updated_at", "track_id", "clip_id", "request_id", "coverage_seconds", "coverage_ratio", "duration_seconds", "sample_rate", "channel_count", "analyzed_range", "evidence_ref")
	out["note"] = "L2 realtime available but not requested"
	return out
}

func l2RenderProbe(input Input) map[string]any {
	realtime := mapValue(input.MixPackage["realtime_metrics"])
	row := mapValue(realtime["render_probe"])
	if len(row) > 0 {
		return row
	}
	featureSnap := mapValue(input.GlobalSummary["feature_snapshot"])
	if row = mapValue(featureSnap["l2_render_probe"]); len(row) > 0 {
		return row
	}
	return nil
}

func renderProbeUsable(row map[string]any) bool {
	switch StatusFromSource(text(row["status"])) {
	case StatusReady, StatusPartial, StatusSuspect, StatusStale:
		return len(row) > 0
	default:
		return false
	}
}

func renderProbeHasStatus(row map[string]any) bool {
	return len(row) > 0 && text(row["status"]) != ""
}

func l2RenderProbeEvidenceRef(row map[string]any) string {
	if ref := text(row["evidence_ref"]); ref != "" {
		return ref
	}
	if renderRevision := text(row["render_revision"]); renderRevision != "" {
		return "dad.l2_render_probe:" + renderRevision
	}
	return ""
}

func compactSpectral(input Input) map[string]any {
	deepSnap := mapValue(input.DeepPackage["feature_snapshot"])
	row := mapValue(deepSnap["spectrogram_tiles"])
	if len(row) == 0 {
		row = mapValue(input.GlobalSummary["spectrogram_tiles"])
	}
	if len(row) == 0 {
		return map[string]any{"status": StatusMissing}
	}
	return compactMap(row, "status", "reason", "source", "source_revision", "clip_revision", "updated_at", "track_id", "clip_id", "request_id", "tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "total_duration")
}

func l2TapPoint(input Input) string {
	current := mapValue(input.MixPackage["realtime_metrics"])
	for _, key := range []string{"render_probe", "band_energy", "stereo_relation"} {
		row := mapValue(current[key])
		if tap := text(row["tap_point"]); tap != "" {
			return tap
		}
	}
	return ""
}

func silenceRisk(row map[string]any) string {
	if rmsDB := number(row["rms_dbfs"]); rmsDB < -60 && rmsDB != 0 {
		return "possible_silence"
	}
	if rms := number(row["rms"]); rms > 0 && rms < 0.001 {
		return "possible_silence"
	}
	if StatusFromSource(text(row["status"])) == StatusMissing {
		return "unknown"
	}
	return "low"
}

func clipRisk(row map[string]any) string {
	if peakDB := number(row["peak_dbfs"]); peakDB >= -1 && peakDB != 0 {
		return "high"
	}
	if peak := number(row["peak_abs"]); peak >= 0.95 {
		return "high"
	}
	if StatusFromSource(text(row["status"])) == StatusMissing {
		return "unknown"
	}
	return "low"
}

func phaseRisk(row map[string]any) string {
	correlation := number(row["correlation_estimate"])
	negative := number(row["phase_negative_ratio"])
	if correlation < 0.2 && correlation != 0 {
		return "high"
	}
	if negative > 0.2 {
		return "medium"
	}
	state := strings.ToLower(text(row["correlation_state"]))
	if strings.Contains(state, "risk") || strings.Contains(state, "watch") {
		return "medium"
	}
	if StatusFromSource(text(row["status"])) == StatusMissing {
		return "unknown"
	}
	return "low"
}

func widthCenterTendency(row map[string]any) string {
	balance := math.Abs(number(row["balance_db"]))
	correlation := number(row["correlation_estimate"])
	if balance >= 3 {
		return "off_center"
	}
	if correlation > 0.85 {
		return "center_strong"
	}
	if correlation > 0 && correlation < 0.35 {
		return "wide_or_phase_sensitive"
	}
	if StatusFromSource(text(row["status"])) == StatusMissing {
		return "unknown"
	}
	return "centered_or_moderate_width"
}

func layerNameForIntent(intent string) string {
	if intent == IntentRealtimeBandStereoObservation {
		return "l2_realtime"
	}
	return "l3_deep"
}

func energySummary(row map[string]any, status string) string {
	if StatusFromSource(status) == StatusMissing {
		return "Basic peak/RMS evidence is missing."
	}
	return fmt.Sprintf("Peak %v dBFS, RMS %v dBFS, headroom %v dB.", row["peak_dbfs"], row["rms_dbfs"], row["headroom_db"])
}

func bandSummary(row map[string]any, status string) string {
	if StatusFromSource(status) == StatusMissing {
		return "Band-energy summary is missing or not trustworthy."
	}
	bands := compactBands(mapValue(row["bands"]))
	return fmt.Sprintf("Band-energy summary available for %d compact bands.", len(bands))
}

func stereoSummary(row map[string]any, status string) string {
	if StatusFromSource(status) == StatusMissing {
		return "Stereo balance/correlation summary is missing or not trustworthy."
	}
	return fmt.Sprintf("Stereo balance=%v dB, correlation=%v, state=%v.", row["balance_db"], row["correlation_estimate"], row["correlation_state"])
}

func ToMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func mapValue(value any) map[string]any {
	return ToMap(value)
}

func rowsFromAny(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row := mapValue(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func compactMap(row map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && !empty(value) {
			out[key] = value
		}
	}
	return out
}

func stringsFromAny(value any) []string {
	out := []string{}
	switch rows := value.(type) {
	case []string:
		for _, row := range rows {
			if row = strings.TrimSpace(row); row != "" {
				out = append(out, row)
			}
		}
	case []any:
		for _, row := range rows {
			if text := text(row); text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func text(value any) string {
	if value == nil {
		return ""
	}
	out := strings.TrimSpace(fmt.Sprint(value))
	if out == "" || out == "<nil>" {
		return ""
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositiveNumber(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if n := number(value); n > 0 {
				return n
			}
		}
	}
	return 0
}

func number(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		text := strings.ToLower(strings.TrimSpace(v))
		return text == "true" || text == "1" || text == "yes" || text == "on"
	default:
		return false
	}
}

func empty(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) || strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func safeKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "target"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
