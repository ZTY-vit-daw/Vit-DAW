package mom

import (
	"fmt"
	"strings"
)

func BuildLLMContext(proj Projection) LLMContext {
	facts := sanitizeContextFacts(llmFactsForIntent(proj))
	return LLMContext{
		SummaryMD:              summaryMD(proj),
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           sanitizeEvidenceRefsForLLM(allEvidenceRefs(proj)),
		QualitySummary:         llmQualitySummary(proj.TrustQuality),
		TaskPolicy:             taskPolicyContext(proj),
		LimitationNotes:        evidenceRefs(append(append(append(append([]string{}, proj.TrustQuality.Limitations...), proj.MultitrackRelation.Limitations...), frequencyRelationshipValue(proj).Limitations...), maskingRelationshipValue(proj).Limitations...)...),
		SafetyGates:            proj.TrustQuality.QualityGates,
		SuggestedNextStep:      suggestedNextStep(proj),
	}
}

func llmFactsForIntent(proj Projection) []map[string]any {
	facts := []map[string]any{
		compactFact("project_structure", proj.ProjectStructure.Status, "Project/target identity and revision context.", proj.ProjectStructure.EvidenceRefs),
		intentPolicyFact(proj.IntentPolicy),
	}
	switch proj.Intent {
	case IntentProjectMultitrackObservation:
		facts = append(facts,
			projectMixProfileFact(proj.ProjectMixProfile),
			multitrackRelationFact(proj.MultitrackRelation),
			compactFact("trust_quality.coverage", proj.TrustQuality.OverallStatus, "Coverage and quality gates summarize the compact multitrack projection.", proj.TrustQuality.EvidenceRefs),
		)
	case IntentProjectFrequencyObservation:
		facts = append(facts,
			frequencyRelationshipFact(frequencyRelationshipValue(proj)),
			compactFact("trust_quality.frequency_relationship", proj.TrustQuality.OverallStatus, "Frequency relationship readiness is bounded by tap point, whole-project coverage, freshness, and persistence limitations.", proj.TrustQuality.EvidenceRefs),
		)
	case IntentProjectMaskingObservation:
		facts = append(facts,
			maskingRelationshipFact(maskingRelationshipValue(proj)),
			compactFact("trust_quality.masking_relationship", proj.TrustQuality.OverallStatus, "Masking-risk readiness is bounded by synchronized same-window post-fader evidence and remains candidate-only.", proj.TrustQuality.EvidenceRefs),
		)
	case IntentRealtimeBandStereoObservation:
		facts = append(facts,
			layerFact("l2_realtime.render_or_live_spectrum", proj.Layers.TimbreFrequency),
			layerFact("l2_realtime.render_or_live_stereo", proj.Layers.SpaceStereo),
			compactFact("trust_quality.l2_tap_point", proj.TrustQuality.OverallStatus, "L2 observation must report render/live tap point and limitations.", proj.TrustQuality.EvidenceRefs),
		)
	case IntentActionPreflightObservation:
		facts = append(facts,
			layerFact("action_relevant_timbre_frequency", proj.Layers.TimbreFrequency),
			layerFact("action_relevant_space_stereo", proj.Layers.SpaceStereo),
			compactFact("action_preflight_limits", proj.TrustQuality.OverallStatus, "Proposed action must cite MOM evidence and wait for confirmation before mutation.", proj.TrustQuality.EvidenceRefs),
		)
	case IntentABResultObservation:
		facts = append(facts,
			abResultFact(proj.Layers.ABResultComparison),
			compactFact("trust_quality.ab_result", proj.TrustQuality.OverallStatus, "AB result is only trusted when before/after render probes share tap point and render mode, both are ready, and render revision changed.", proj.TrustQuality.EvidenceRefs),
		)
	default:
		facts = append(facts,
			layerFact("timbre_frequency.l3_full_song", proj.Layers.TimbreFrequency),
			layerFact("space_stereo.l3_full_song", proj.Layers.SpaceStereo),
			compactFact("l2_realtime_status", proj.TrustQuality.OverallStatus, "L2 realtime available only as status unless explicitly requested.", proj.TrustQuality.EvidenceRefs),
		)
	}
	if len(proj.TrustQuality.DeferredLayers) > 0 {
		facts = append(facts, map[string]any{
			"layer":           "deferred_layers",
			"status":          StatusDeferred,
			"freshness":       FreshnessForStatus(StatusDeferred),
			"summary":         "Non-requested or deferred analyzers do not affect the current intent status.",
			"deferred_layers": proj.TrustQuality.DeferredLayers,
			"evidence_refs":   proj.TrustQuality.EvidenceRefs,
		})
	}
	if status := StatusFromSource(proj.Layers.ABResultComparison.Status); status != StatusMissing && status != StatusDeferred {
		facts = append(facts, abResultFact(proj.Layers.ABResultComparison))
	}
	return facts
}

func intentPolicyFact(policy IntentPolicy) map[string]any {
	return map[string]any{
		"layer":           "intent_policy",
		"status":          StatusReady,
		"freshness":       FreshnessForStatus(StatusReady),
		"summary":         strings.TrimSpace(fmt.Sprintf("required=%s optional=%s contract=%s", strings.Join(policy.RequiredLayers, ","), strings.Join(policy.OptionalLayers, ","), strings.Join(policy.OutputContract, ","))),
		"required_layers": policy.RequiredLayers,
		"optional_layers": policy.OptionalLayers,
		"output_contract": policy.OutputContract,
	}
}

func projectMixProfileFact(profile ProjectMixProfile) map[string]any {
	fact := compactFact("project_mix_profile", profile.Status, "Compact project mix profile summarises per-track headroom, dominant bands, and stereo risk.", profile.EvidenceRefs)
	fact["track_count"] = profile.TrackCount
	fact["compared_track_count"] = len(profile.ComparedTracks)
	fact["risk_tags"] = profile.RiskTags
	fact["limitations"] = profile.Limitations
	return fact
}

func multitrackRelationFact(relation MultitrackRelation) map[string]any {
	fact := compactFact("multitrack_relation", relation.Status, "Compact multitrack relation summarizes local comparison results and evidence refs.", relation.EvidenceRefs)
	fact["track_count"] = relation.TrackCount
	fact["compared_track_count"] = len(relation.ComparedTracks)
	fact["missing_track_count"] = len(relation.MissingTracks)
	fact["conflict_candidate_count"] = len(relation.BandConflictCandidates)
	fact["phase_risk_count"] = len(relation.PhaseRiskTracks)
	fact["limitations"] = relation.Limitations
	return fact
}

func frequencyRelationshipFact(relation FrequencyRelationship) map[string]any {
	fact := compactFact("frequency_relationship", relation.Status, "Compact project frequency relationship projection; overlap rows are candidates, not masking facts.", relation.EvidenceRefs)
	fact["schema_version"] = relation.SchemaVersion
	fact["project_cut_ref"] = relation.ProjectCutRef
	fact["scope"] = relation.Scope
	fact["tap_point"] = relation.TapPoint
	fact["coverage"] = relation.Coverage
	fact["track_profiles"] = relation.TrackProfiles
	fact["frequency_regions"] = relation.FrequencyRegions
	fact["conflict_candidates"] = relation.ConflictCandidates
	fact["tonal_tendencies"] = relation.TonalTendencies
	fact["persistence_summary"] = relation.PersistenceSummary
	fact["verification_dimensions"] = relation.VerificationDimensions
	fact["limitations"] = relation.Limitations
	return fact
}

func maskingRelationshipFact(relation MaskingRelationship) map[string]any {
	fact := compactFact("masking_relationship", relation.Status, "Directional masking-risk candidates from synchronized current-mix measurements; never deterministic mix defects.", relation.EvidenceRefs)
	fact["schema_version"] = relation.SchemaVersion
	fact["measurement_id"] = relation.MeasurementID
	fact["model_version"] = relation.ModelVersion
	fact["candidate_only"] = relation.CandidateOnly
	fact["project_binding"] = relation.ProjectBinding
	fact["conditions"] = relation.Conditions
	fact["coverage"] = relation.Coverage
	fact["candidates"] = relation.Candidates
	fact["limitations"] = relation.Limitations
	return fact
}

func layerFact(name string, layer Layer) map[string]any {
	out := compactFact(name, layer.Status, layer.Summary, layer.EvidenceRefs)
	if primary := strings.TrimSpace(fmt.Sprint(layer.Facts["primary_layer"])); primary != "" && primary != "<nil>" {
		out["primary_layer"] = primary
	}
	return out
}

func abResultFact(layer Layer) map[string]any {
	out := compactFact("ab_result_comparison", layer.Status, layer.Summary, layer.EvidenceRefs)
	ab := mapValue(layer.Facts["ab_result"])
	for _, key := range []string{
		"tap_point", "render_mode", "quality_gates", "summary_tags", "reason",
	} {
		if value, ok := ab[key]; ok {
			out[key] = value
		}
	}
	if before := text(ab["before_render_revision"]); before != "" || text(ab["after_render_revision"]) != "" {
		out["render_revision_changed"] = text(ab["before_render_revision"]) != "" && text(ab["after_render_revision"]) != "" && before != text(ab["after_render_revision"])
	}
	if delta := compactABDelta(mapValue(ab["delta"])); len(delta) > 0 {
		out["delta"] = delta
	}
	return out
}

func compactFact(name, status, summary string, refs []string) map[string]any {
	return map[string]any{
		"layer":         name,
		"status":        StatusFromSource(status),
		"freshness":     FreshnessForStatus(status),
		"summary":       strings.TrimSpace(summary),
		"evidence_refs": sanitizeEvidenceRefsForLLM(refs),
	}
}

func summaryMD(proj Projection) string {
	lines := []string{
		fmt.Sprintf("MOM %s intent=%s overall=%s.", proj.MOMVersion, proj.Intent, proj.TrustQuality.OverallStatus),
		fmt.Sprintf("Required layers: %s.", strings.Join(proj.TrustQuality.RequiredLayers, ", ")),
	}
	switch proj.Intent {
	case IntentProjectMultitrackObservation:
		lines = append(lines, fmt.Sprintf("Compact project_mix_profile and multitrack_relation are primary; track_count=%d; limitations=%s.", proj.ProjectMixProfile.TrackCount, strings.Join(proj.MultitrackRelation.Limitations, ",")))
	case IntentProjectFrequencyObservation:
		frequency := frequencyRelationshipValue(proj)
		lines = append(lines, fmt.Sprintf("MOM frequency_relationship is primary; tap_point=%s; eligible_tracks=%v; overlap rows are candidates rather than masking facts; limitations=%s.", frequency.TapPoint, frequency.Coverage["eligible_track_count"], strings.Join(frequency.Limitations, ",")))
	case IntentProjectMaskingObservation:
		masking := maskingRelationshipValue(proj)
		lines = append(lines, fmt.Sprintf("MOM masking_relationship is primary; tap_point=%s; candidate_count=%d; rows are directional improvement-risk candidates, never deterministic mix defects; limitations=%s.", text(masking.Conditions["tap_point"]), len(masking.Candidates), strings.Join(masking.Limitations, ",")))
	case IntentRealtimeBandStereoObservation:
		lines = append(lines, fmt.Sprintf("L2 render/live post-chain observation is primary; tap_point=%s; limitations=%s.", proj.TrustQuality.L2TapPoint, strings.Join(proj.TrustQuality.L2Limitations, ",")))
	case IntentActionPreflightObservation:
		lines = append(lines, "Action preflight must cite MOM evidence first and wait for confirmation before any plugin/rack/parameter mutation.")
	case IntentABResultObservation:
		lines = append(lines, "AB result requires same tap point, same render mode, changed render revision, and ready before/after evidence.")
	default:
		lines = append(lines, "General observation prioritizes L3 full-song band/stereo evidence; L2 render/live values are not expanded unless explicitly requested.")
	}
	lines = append(lines, "Sample arrays, spectral tile payloads, and full acoustic packages are excluded from LLM context; use evidence_refs for traceability.")
	return strings.Join(lines, "\n")
}

func ContextProjection(proj Projection) map[string]any {
	out := map[string]any{
		"mom_version":    proj.MOMVersion,
		"intent":         proj.Intent,
		"intent_policy":  proj.IntentPolicy,
		"observation_id": proj.ObservationID,
		"mix_session_id": proj.MixSessionID,
		"project_structure": map[string]any{
			"status":                 proj.ProjectStructure.Status,
			"freshness":              proj.ProjectStructure.Freshness,
			"project_id":             proj.ProjectStructure.ProjectID,
			"session_id":             proj.ProjectStructure.SessionID,
			"track_id":               proj.ProjectStructure.TrackID,
			"clip_id":                proj.ProjectStructure.ClipID,
			"gui_id":                 proj.ProjectStructure.GUIID,
			"source_revision_status": proj.TrustQuality.SourceRevisionStatus,
			"clip_revision_status":   proj.TrustQuality.ClipRevisionStatus,
			"render_revision_status": proj.TrustQuality.RenderRevisionStatus,
			"duration_seconds":       proj.ProjectStructure.DurationSec,
			"sample_rate":            proj.ProjectStructure.SampleRate,
			"channel_count":          proj.ProjectStructure.ChannelCount,
			"target_ref":             proj.ProjectStructure.TargetRef,
			"listen_scope":           proj.ProjectStructure.ListenScope,
			"evidence_refs":          sanitizeEvidenceRefsForLLM(proj.ProjectStructure.EvidenceRefs),
			"limitations":            proj.ProjectStructure.Limitations,
		},
		"project_mix_profile": proj.ProjectMixProfile,
		"multitrack_relation": proj.MultitrackRelation,
		"trust_quality":       trustQualityContext(proj.TrustQuality),
		"llm_context":         proj.LLMContext,
	}
	if proj.StaticLevelRelationship != nil {
		out["static_level_relationship"] = proj.StaticLevelRelationship
	}
	if proj.FrequencyRelationship != nil {
		out["frequency_relationship"] = proj.FrequencyRelationship
	}
	if proj.MaskingRelationship != nil {
		out["masking_relationship"] = proj.MaskingRelationship
	}
	return out
}

func frequencyRelationshipValue(proj Projection) FrequencyRelationship {
	if proj.FrequencyRelationship == nil {
		return FrequencyRelationship{}
	}
	return *proj.FrequencyRelationship
}

func maskingRelationshipValue(proj Projection) MaskingRelationship {
	if proj.MaskingRelationship == nil {
		return MaskingRelationship{}
	}
	return *proj.MaskingRelationship
}

func trustQualityContext(trust TrustQuality) map[string]any {
	return map[string]any{
		"schema_version":               trust.SchemaVersion,
		"overall_status":               trust.OverallStatus,
		"required_layers":              trust.RequiredLayers,
		"optional_layers":              trust.OptionalLayers,
		"deferred_layers":              trust.DeferredLayers,
		"coverage":                     trust.Coverage,
		"quality_gates":                trust.QualityGates,
		"blocked_reasons":              trust.BlockedReasons,
		"approximate_fields":           trust.ApproximateFields,
		"suspect_fields":               trust.SuspectFields,
		"stale_fields":                 trust.StaleFields,
		"missing_fields":               trust.MissingFields,
		"source_revision_status":       trust.SourceRevisionStatus,
		"clip_revision_status":         trust.ClipRevisionStatus,
		"render_revision_status":       trust.RenderRevisionStatus,
		"can_support_observation":      trust.CanSupportObservation,
		"can_support_suggestion":       trust.CanSupportSuggestion,
		"can_support_action_preflight": trust.CanSupportActionPreflight,
		"can_support_ab_result":        trust.CanSupportABResult,
		"freshness":                    trust.Freshness,
		"l2_tap_point":                 trust.L2TapPoint,
		"l2_limitations":               trust.L2Limitations,
		"limitations":                  trust.Limitations,
		"evidence_refs":                sanitizeEvidenceRefsForLLM(trust.EvidenceRefs),
	}
}

func llmQualitySummary(trust TrustQuality) map[string]any {
	return map[string]any{
		"schema_version":               trust.SchemaVersion,
		"overall_status":               trust.OverallStatus,
		"blocked_reasons":              trust.BlockedReasons,
		"approximate_fields":           trust.ApproximateFields,
		"suspect_fields":               trust.SuspectFields,
		"stale_fields":                 trust.StaleFields,
		"missing_fields":               trust.MissingFields,
		"can_support_observation":      trust.CanSupportObservation,
		"can_support_suggestion":       trust.CanSupportSuggestion,
		"can_support_action_preflight": trust.CanSupportActionPreflight,
		"can_support_ab_result":        trust.CanSupportABResult,
		"source_revision_status":       trust.SourceRevisionStatus,
		"clip_revision_status":         trust.ClipRevisionStatus,
		"render_revision_status":       trust.RenderRevisionStatus,
		"l2_tap_point":                 trust.L2TapPoint,
	}
}

func taskPolicyContext(proj Projection) map[string]any {
	return map[string]any{
		"intent":          proj.Intent,
		"required_layers": proj.IntentPolicy.RequiredLayers,
		"optional_layers": proj.IntentPolicy.OptionalLayers,
		"output_contract": proj.IntentPolicy.OutputContract,
	}
}

func suggestedNextStep(proj Projection) string {
	if len(proj.TrustQuality.BlockedReasons) > 0 {
		return "Refresh or complete the missing/suspect evidence before using this observation for action."
	}
	switch proj.Intent {
	case IntentActionPreflightObservation:
		if proj.TrustQuality.CanSupportActionPreflight {
			return "Provide a conservative suggestion and wait for explicit user confirmation before any pending action."
		}
		return "Request a fresh observation before creating pending action."
	case IntentABResultObservation:
		if proj.TrustQuality.CanSupportABResult {
			return "Explain compact AB delta and keep evidence refs structured."
		}
		return "Run same-tap before/after render probes before trusting AB."
	case IntentProjectMultitrackObservation:
		return "Explain the local project relation projection without recalculating raw packages."
	case IntentProjectFrequencyObservation:
		return "Use the bounded frequency relationship facts for readiness/diagnosis only; do not infer masking, EQ parameters, or execution authority."
	case IntentProjectMaskingObservation:
		return "Use directional masking-risk candidates to form reasonable improvement suggestions; do not claim a deterministic defect or infer processor parameters without a separate execution decision."
	default:
		return "Summarize conclusion, evidence, limitations, and a non-mutating suggestion."
	}
}

func sanitizeContextFacts(facts []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		if sanitized := sanitizeContextMap(fact); len(sanitized) > 0 {
			out = append(out, sanitized)
		}
	}
	return out
}

func sanitizeContextMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if contextForbiddenKey(key) {
			continue
		}
		if sanitized, ok := sanitizeContextValue(value); ok {
			out[key] = sanitized
		}
	}
	return out
}

func sanitizeContextValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeContextMap(typed), true
	case []map[string]any:
		rows := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			if sanitized := sanitizeContextMap(row); len(sanitized) > 0 {
				rows = append(rows, sanitized)
			}
		}
		return rows, true
	case []any:
		rows := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized, ok := sanitizeContextValue(item); ok {
				rows = append(rows, sanitized)
			}
		}
		return rows, true
	case string:
		return sanitizeContextString(typed), true
	default:
		return value, true
	}
}

func contextForbiddenKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, forbidden := range []string{
		"raw_package", "raw_waveform", "raw_samples", "waveform_arrays", "time_segments",
		"spectrogram_tiles", "spectrogram_tile_rows", "tile_payload", "render_file_path",
		"render_path", "shared_memory", "full_artifact_history",
	} {
		if key == forbidden || strings.Contains(key, forbidden) {
			return true
		}
	}
	return false
}

func sanitizeContextString(value string) string {
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"raw_samples", "raw_waveform", "spectrogram_tiles", "tile_payload", "shared_memory", ".wav", ".aiff", ".mp3", ":\\", "d:/", "c:/"} {
		if strings.Contains(lower, forbidden) {
			return "[omitted_context_payload]"
		}
	}
	return value
}

func sanitizeEvidenceRefsForLLM(refs []string) []string {
	out := []string{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || sanitizeContextString(ref) != ref {
			continue
		}
		out = append(out, ref)
	}
	return evidenceRefs(out...)
}
