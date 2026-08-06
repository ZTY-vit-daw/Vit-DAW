package capabilitycontext

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

const (
	FreeStateObservationCatalogSchema = "ccb_observation_catalog.v1"
	FreeStateObservationRequestSchema = "ccb_observation_request.v1"
	FreeStateObservationBundleSchema  = "ccb_observation_bundle.v1"
)

type FreeStateObservationView struct {
	ViewID               string   `json:"view_id"`
	Questions            []string `json:"questions"`
	SupportedTargetKinds []string `json:"supported_target_kinds"`
	TemporalResolution   string   `json:"temporal_resolution"`
	Availability         string   `json:"availability"`
	CostLatencyClass     string   `json:"cost_latency_class"`
	QualityCeiling       string   `json:"quality_ceiling"`
	Limitations          []string `json:"limitations,omitempty"`
	RequiredDependencies []string `json:"required_dependencies,omitempty"`
}

type FreeStateObservationCatalog struct {
	SchemaVersion string                     `json:"schema_version"`
	Boundary      string                     `json:"boundary"`
	TargetRef     mixboard.TargetRef         `json:"target_ref,omitempty"`
	Views         []FreeStateObservationView `json:"views"`
	Exclusions    []string                   `json:"exclusions"`
}

type FreeStateObservationRequest struct {
	SchemaVersion      string             `json:"schema_version"`
	RequestID          string             `json:"request_id,omitempty"`
	ObservationID      string             `json:"observation_id,omitempty"`
	MixSessionID       string             `json:"mix_session_id,omitempty"`
	ViewIDs            []string           `json:"view_ids"`
	TargetRef          mixboard.TargetRef `json:"target_ref,omitempty"`
	FreshnessClass     string             `json:"freshness_class,omitempty"`
	MaxDisclosureBytes int                `json:"max_disclosure_bytes,omitempty"`
	MaxItems           int                `json:"max_items,omitempty"`
}

type FreeStateObservationBundle struct {
	SchemaVersion      string                                  `json:"schema_version"`
	BundleID           string                                  `json:"bundle_id"`
	RequestID          string                                  `json:"request_id,omitempty"`
	Status             string                                  `json:"status"`
	ReadOnly           bool                                    `json:"read_only"`
	MutationAuthority  bool                                    `json:"mutation_authority"`
	ObservationID      string                                  `json:"observation_id"`
	MixSessionID       string                                  `json:"mix_session_id,omitempty"`
	TargetRef          map[string]any                          `json:"target_ref,omitempty"`
	ProjectBinding     map[string]any                          `json:"project_binding,omitempty"`
	Freshness          map[string]any                          `json:"freshness"`
	RequestedViews     []string                                `json:"requested_views"`
	Views              map[string]any                          `json:"views"`
	EvidenceRefs       []string                                `json:"evidence_refs,omitempty"`
	Limitations        []string                                `json:"limitations,omitempty"`
	OmissionReasons    []string                                `json:"omission_reasons,omitempty"`
	Omissions          map[string]orchestration.OmissionStatus `json:"omissions,omitempty"`
	DisclosureBytes    int                                     `json:"disclosure_bytes"`
	MaxDisclosureBytes int                                     `json:"max_disclosure_bytes"`
}

type freeStateViewDefinition struct {
	view FreeStateObservationView
	keys []string
}

func FreeStateObservationCatalogFor(target mixboard.TargetRef) FreeStateObservationCatalog {
	defs := freeStateViewDefinitions(target.ID)
	views := make([]FreeStateObservationView, 0, len(defs))
	for _, def := range defs {
		views = append(views, def.view)
	}
	return FreeStateObservationCatalog{
		SchemaVersion: FreeStateObservationCatalogSchema,
		Boundary:      "semantic_views_only",
		TargetRef:     target,
		Views:         views,
		Exclusions: []string{
			"raw PCM or audio buffers",
			"waveform and spectrogram arrays",
			"full DAD packages or evidence blobs",
			"processor selection, parameter decisions, and mutation authority",
		},
	}
}

func NormalizeFreeStateObservationRequest(req FreeStateObservationRequest) FreeStateObservationRequest {
	req.SchemaVersion = FreeStateObservationRequestSchema
	if strings.TrimSpace(req.FreshnessClass) == "" {
		req.FreshnessClass = "current_observation"
	}
	if req.MaxDisclosureBytes <= 0 {
		req.MaxDisclosureBytes = 16 * 1024
	}
	if req.MaxDisclosureBytes < 256 {
		req.MaxDisclosureBytes = 256
	}
	if req.MaxDisclosureBytes > 64*1024 {
		req.MaxDisclosureBytes = 64 * 1024
	}
	if req.MaxItems <= 0 {
		req.MaxItems = 12
	}
	if req.MaxItems > 24 {
		req.MaxItems = 24
	}
	req.ViewIDs = uniqueNonEmpty(req.ViewIDs)
	if len(req.ViewIDs) == 0 {
		req.ViewIDs = []string{"project.structure"}
	}
	return req
}

func FreeStateObservationReadKeys(req FreeStateObservationRequest, targetID string) []string {
	req = NormalizeFreeStateObservationRequest(req)
	defs := freeStateViewDefinitions(targetID)
	byID := map[string]freeStateViewDefinition{}
	for _, def := range defs {
		byID[def.view.ViewID] = def
	}
	keys := []string{"observation.binding"}
	for _, viewID := range req.ViewIDs {
		keys = append(keys, byID[viewID].keys...)
	}
	return uniqueNonEmpty(keys)
}

func AssembleFreeStateObservation(req FreeStateObservationRequest, readResult map[string]any) FreeStateObservationBundle {
	req = NormalizeFreeStateObservationRequest(req)
	items := anyMap(readResult["items"])
	binding := anyMap(items["observation.binding"])
	observationID := firstNonEmptyString(stringValue(binding["observation_id"]), stringValue(readResult["observation_id"]), req.ObservationID)
	mixSessionID := firstNonEmptyString(stringValue(binding["mix_session_id"]), stringValue(readResult["mix_session_id"]), req.MixSessionID)
	defs := freeStateViewDefinitions(targetIDFromBinding(binding, req.TargetRef.ID))
	byID := map[string]freeStateViewDefinition{}
	for _, def := range defs {
		byID[def.view.ViewID] = def
	}
	bundle := FreeStateObservationBundle{
		SchemaVersion:     FreeStateObservationBundleSchema,
		BundleID:          "ccbobs_" + compactID(observationID, req.RequestID),
		RequestID:         req.RequestID,
		Status:            "ready",
		ReadOnly:          true,
		MutationAuthority: false,
		ObservationID:     observationID,
		MixSessionID:      mixSessionID,
		TargetRef:         anyMap(binding["target_ref"]),
		ProjectBinding:    anyMap(binding["project_binding"]),
		Freshness: map[string]any{
			"class":       req.FreshnessClass,
			"status":      firstNonEmptyString(stringValue(binding["status"]), "unknown"),
			"observed_at": stringValue(binding["created_at"]),
		},
		RequestedViews:     append([]string(nil), req.ViewIDs...),
		Views:              map[string]any{},
		EvidenceRefs:       stringSlice(binding["evidence_refs"]),
		Omissions:          map[string]orchestration.OmissionStatus{},
		MaxDisclosureBytes: req.MaxDisclosureBytes,
	}
	for _, viewID := range req.ViewIDs {
		def, known := byID[viewID]
		if !known {
			bundle.Omissions[viewID] = orchestration.OmissionForbidden
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": semantic view is not in the CCB catalog")
			continue
		}
		if len(def.keys) == 0 {
			bundle.Omissions[viewID] = orchestration.OmissionUnavailable
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": "+firstNonEmptyString(firstStringSlice(def.view.Limitations), "view is unavailable"))
			continue
		}
		view, status := assembleFreeStateView(viewID, def, items)
		if status == "missing" || status == "deferred" || status == "unavailable" {
			bundle.Omissions[viewID] = orchestration.OmissionUnavailable
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": source evidence is "+status)
			continue
		}
		if status == "stale" {
			bundle.Omissions[viewID] = orchestration.OmissionStale
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": source evidence is stale")
			continue
		}
		candidate := cloneAnyMap(bundle.Views)
		candidate[viewID] = sanitizeFreeStateValue(view, req.MaxItems, 0)
		if jsonSize(candidate) > req.MaxDisclosureBytes {
			bundle.Omissions[viewID] = orchestration.OmissionBudget
			bundle.OmissionReasons = append(bundle.OmissionReasons, viewID+": omitted by disclosure budget")
			continue
		}
		bundle.Views = candidate
		if status == "partial" || status == "suspect" || status == "approximate" {
			bundle.Limitations = append(bundle.Limitations, viewID+": evidence status is "+status)
		}
	}
	bundle.DisclosureBytes = jsonSize(bundle.Views)
	if len(bundle.Views) == 0 {
		bundle.Status = "insufficient"
	} else if len(bundle.Omissions) > 0 || len(bundle.Limitations) > 0 {
		bundle.Status = "partial"
	}
	bundle.EvidenceRefs = uniqueNonEmpty(bundle.EvidenceRefs)
	bundle.Limitations = uniqueNonEmpty(bundle.Limitations)
	bundle.OmissionReasons = uniqueNonEmpty(bundle.OmissionReasons)
	if len(bundle.Omissions) == 0 {
		bundle.Omissions = nil
	}
	return bundle
}

func freeStateViewDefinitions(targetID string) []freeStateViewDefinition {
	if strings.TrimSpace(targetID) == "" {
		targetID = "target"
	}
	track := "track." + targetID
	return []freeStateViewDefinition{
		{view: semanticView("project.structure", []string{"What tracks and sources are present?", "What is the current project/selection structure?"}, []string{"project", "track", "selection"}, "state snapshot", "ready_on_observation", "cheap", "compact project and TOM-adjacent identity summary", []string{"Does not disclose a full TOM tree."}, []string{"project state", "MixBoard observation"}), keys: []string{"project.static.summary", "project.tracks.summary", "observation.tim_projection", "observation.mom_projection"}},
		{view: semanticView("track.basic_energy", []string{"How loud and peaky is the target?", "Is headroom or crest factor unusual?"}, []string{"track", "clip", "selection"}, "whole window", "ready_on_observation", "cheap", "bounded level summary", nil, []string{"waveform envelope summary"}), keys: []string{track + ".static.identity", track + ".fast.levels"}},
		{view: semanticView("track.time_dynamics", []string{"How does energy evolve over time?", "What transient and macro-dynamic structure is observable?"}, []string{"track", "clip", "selection"}, "macro and short-window summary", "conditional", "medium", "COM plus bounded time-energy summaries", []string{"Fine envelopes and event lists stay in evidence storage."}, []string{"time-energy summary", "COM projection"}), keys: []string{track + ".slow.time_energy.summary", "observation.com_projection"}},
		{view: semanticView("track.timbre_frequency", []string{"Where is energy concentrated by band?", "Which broad tonal regions need inspection?"}, []string{"track", "clip", "selection"}, "whole-window band summary", "conditional", "medium", "broad-band energy only", []string{"Not a raw spectrum or a static-EQ decision."}, []string{"band-energy summary"}), keys: []string{track + ".slow.band_energy.summary"}},
		{view: semanticView("track.stereo_space", []string{"How wide or correlated is the target?", "Is left-right balance unusual?"}, []string{"track", "clip", "selection"}, "whole-window stereo summary", "conditional", "medium", "balance/correlation summary", nil, []string{"stereo-relation summary"}), keys: []string{track + ".slow.stereo.summary"}},
		{view: semanticView("mix.multitrack_relationship", []string{"How do track levels and risks relate?", "Which track deserves attention first?"}, []string{"project", "track_group"}, "project snapshot", "conditional", "medium", "bounded MOM and project relationship summaries", []string{"This is observation, not a B2/B3 solver result."}, []string{"MOM multitrack projection", "project acoustic summaries"}), keys: []string{"project.relationship_inputs", "project.rankings.level", "project.rankings.peak", "project.risks.headroom", "project.attention.first", "observation.mom_projection"}},
		{view: semanticView("mix.frequency_relationship", []string{"How do track band occupancies relate?", "Where are broad frequency conflicts plausible?"}, []string{"project", "track_group"}, "project snapshot", "conditional", "medium", "compact MOM frequency relationship projection", []string{"Does not claim psychoacoustic masking certainty."}, []string{"MOM frequency relationship", "project band-energy coverage"}), keys: []string{"project.frequency_relationship_inputs", "observation.mom_projection"}},
		{view: semanticView("mix.masking_relationship", []string{"Which sources measurably mask each other?"}, []string{"project", "track_group"}, "not available", "deferred", "expensive", "unavailable in v1", []string{"A dedicated masking observation unit is not implemented."}, []string{"future masking projection"})},
		{view: semanticView("processor.identity_and_controls", []string{"Which processor is bound to this observation?", "Which semantic controls are available?"}, []string{"processor"}, "processor state snapshot", "conditional", "cheap", "processor scope only", []string{"COM does not disclose a live control surface; use the processor inspector separately."}, []string{"COM processor scope", "read-only processor inspector"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("processor.behavior", []string{"What gain action and transient/recovery behavior is observed?"}, []string{"processor", "track"}, "paired input/output summary", "conditional", "medium", "compact COM behavior projection", []string{"Requires paired evidence for processor-caused behavior."}, []string{"COM paired_io projection"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("processor.change_delta", []string{"How did processor behavior change between observations?"}, []string{"processor", "track"}, "change delta", "conditional", "medium", "compact COM change projection", []string{"Requires compatible before/after COM evidence."}, []string{"COM change_delta projection"}), keys: []string{"observation.com_projection"}},
		{view: semanticView("comparison.before_after", []string{"What changed between the current and previous observation?"}, []string{"project", "track", "processor"}, "observation pair", "conditional", "cheap", "bounded delta summary", []string{"Requires a prior observation in the same mix session."}, []string{"MixBoard observation history"}), keys: []string{"observation.before_after.latest"}},
	}
}

func semanticView(id string, questions, targets []string, temporal, availability, cost, ceiling string, limitations, dependencies []string) FreeStateObservationView {
	return FreeStateObservationView{ViewID: id, Questions: questions, SupportedTargetKinds: targets, TemporalResolution: temporal, Availability: availability, CostLatencyClass: cost, QualityCeiling: ceiling, Limitations: limitations, RequiredDependencies: dependencies}
}

func assembleFreeStateView(viewID string, def freeStateViewDefinition, items map[string]any) (map[string]any, string) {
	facts := map[string]any{}
	useful := 0
	degraded := false
	stale := false
	for _, key := range def.keys {
		value, ok := items[key]
		if !ok || value == nil {
			degraded = true
			continue
		}
		itemStatus := semanticItemStatus(viewID, key, value)
		switch itemStatus {
		case "missing", "deferred", "unavailable", "blocked", "requested", "pending":
			degraded = true
		case "stale":
			stale = true
		case "partial", "suspect", "approximate":
			useful++
			degraded = true
		default:
			useful++
		}
		facts[key] = compactProjectionForView(viewID, key, value)
	}
	status := "ready"
	if len(facts) == 0 {
		status = "missing"
	} else if useful == 0 && stale {
		status = "stale"
	} else if useful == 0 {
		status = "missing"
	} else if stale || degraded {
		status = "partial"
	}
	return map[string]any{
		"view_id":     viewID,
		"status":      status,
		"facts":       facts,
		"limitations": def.view.Limitations,
	}, status
}

func semanticItemStatus(viewID, key string, value any) string {
	row := anyMap(value)
	status := strings.ToLower(strings.TrimSpace(stringValue(row["status"])))
	if key == "observation.mom_projection" {
		switch viewID {
		case "project.structure":
			status = stringValue(anyMap(row["project_structure"])["status"])
		case "mix.multitrack_relationship":
			status = stringValue(anyMap(row["multitrack_relation"])["status"])
		case "mix.frequency_relationship":
			status = stringValue(anyMap(row["frequency_relationship"])["status"])
		}
	}
	if key == "observation.com_projection" {
		switch viewID {
		case "track.time_dynamics":
			status = stringValue(anyMap(row["source_dynamics"])["status"])
		case "processor.identity_and_controls":
			if len(anyMap(row["processor_scope"])) == 0 {
				return "missing"
			}
			return "partial"
		case "processor.behavior":
			if !hasAnyField(row, "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation") {
				return "missing"
			}
		case "processor.change_delta":
			if len(anyMap(row["behavior_change"])) == 0 {
				return "missing"
			}
		}
	}
	if strings.TrimSpace(status) == "" {
		return "ready"
	}
	return strings.ToLower(strings.TrimSpace(status))
}

func hasAnyField(row map[string]any, fields ...string) bool {
	for _, field := range fields {
		if value, ok := row[field]; ok && value != nil && len(anyMap(value)) > 0 {
			return true
		}
	}
	return false
}

func compactProjectionForView(viewID, key string, value any) any {
	row := anyMap(value)
	if len(row) == 0 {
		return value
	}
	if key == "observation.com_projection" {
		switch viewID {
		case "processor.identity_and_controls":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "identifiability", "trust_quality", "evidence_refs", "limitations")
		case "processor.behavior":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "gain_action", "transient_response", "recovery_motion", "level_effect", "stereo_behavior", "trigger_relation", "identifiability", "trust_quality", "llm_context", "evidence_refs", "limitations")
		case "processor.change_delta":
			return selectFields(row, "schema_version", "com_version", "projection_id", "mode", "status", "processor_scope", "behavior_change", "trust_quality", "llm_context", "evidence_refs", "limitations")
		case "track.time_dynamics":
			return compactCOMSourceDynamicsForSelection(row)
		}
	}
	if key == "observation.tim_projection" {
		return selectFields(row, "schema_version", "tim_version", "status", "technical_summary", "coverage", "risk_summary", "evidence_refs", "limitations")
	}
	if key == "observation.mom_projection" {
		switch viewID {
		case "project.structure":
			return selectFields(row, "mom_version", "intent", "project_structure", "trust_quality", "llm_context")
		case "mix.multitrack_relationship":
			return map[string]any{
				"mom_version":         row["mom_version"],
				"intent":              row["intent"],
				"multitrack_relation": compactMOMMultitrackRelation(anyMap(row["multitrack_relation"])),
			}
		case "mix.frequency_relationship":
			return selectFields(row, "mom_version", "intent", "frequency_relationship")
		}
	}
	return value
}

func compactCOMSourceDynamicsForSelection(row map[string]any) map[string]any {
	source := anyMap(row["source_dynamics"])
	trust := anyMap(row["trust_quality"])
	compactSource := selectFields(source,
		"schema_version", "status", "declared_scale", "analyzed_duration_seconds",
		"levels", "activity", "macro_dynamics", "events", "evidence_refs",
	)
	if scales, ok := source["time_scale_coverage"].([]any); ok {
		bounded := make([]any, 0, 2)
		for _, value := range scales {
			scale := anyMap(value)
			switch strings.ToLower(stringValue(scale["scale"])) {
			case "macro_program", "micro_transient":
				bounded = append(bounded, scale)
			}
		}
		if len(bounded) > 0 {
			compactSource["time_scale_coverage"] = bounded
		}
	}
	canDescribe, _ := trust["can_support_source_description"].(bool)
	selectionStatus := "unavailable"
	if canDescribe {
		selectionStatus = "supported_with_limitations"
	}
	return map[string]any{
		"schema_version":  row["schema_version"],
		"com_version":     row["com_version"],
		"projection_id":   row["projection_id"],
		"mode":            row["mode"],
		"status":          row["status"],
		"source_dynamics": compactSource,
		"decision_support": map[string]any{
			"current_decision":                   "processor_family_selection",
			"status":                             selectionStatus,
			"can_describe_source_dynamics":       canDescribe,
			"supports":                           []string{"source_macro_dynamics", "treatment_family_selection"},
			"does_not_support":                   []string{"compressor_behavior_claims", "compressor_parameter_values", "post_action_evaluation"},
			"non_blocking_constraints":           []string{"micro_transient_detail_unresolved", "preserve_transients_conservatively"},
			"downstream_evidence_owner":          "governed_processor_workflow",
			"paired_processor_io_required_after": "processor_selection",
		},
		"evidence_refs": row["evidence_refs"],
	}
}

func compactMOMMultitrackRelation(relation map[string]any) map[string]any {
	if len(relation) == 0 {
		return nil
	}
	level := anyMap(relation["level_distribution"])
	out := selectFields(relation, "status", "freshness", "track_count", "limitations", "evidence_refs")
	out["coverage"] = map[string]any{
		"compared_track_count": anyListLength(relation["compared_tracks"]),
		"missing_track_count":  anyListLength(relation["missing_tracks"]),
	}
	if len(level) > 0 {
		out["level_summary"] = selectFields(level, "status", "track_count", "tracks_with_level", "spread_db", "highest", "lowest")
	}
	out["risk_summary"] = map[string]any{
		"band_occupancy_count": anyListLength(relation["band_occupancy"]),
		"band_conflict_count":  anyListLength(relation["band_conflict_candidates"]),
		"phase_risk_count":     anyListLength(relation["phase_risk_tracks"]),
	}
	return out
}

func anyListLength(value any) int {
	if rows, ok := value.([]any); ok {
		return len(rows)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	var rows []any
	if json.Unmarshal(data, &rows) != nil {
		return 0
	}
	return len(rows)
}

func selectFields(row map[string]any, fields ...string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		if value, ok := row[field]; ok && value != nil {
			out[field] = value
		}
	}
	return out
}

func sanitizeFreeStateValue(value any, maxItems, depth int) any {
	if depth > 10 {
		return map[string]any{"status": "omitted", "reason": "depth_limit"}
	}
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			if freeStateForbiddenField(key) {
				continue
			}
			out[key] = sanitizeFreeStateValue(child, maxItems, depth+1)
		}
		return out
	case []any:
		if maxItems > 0 && len(typed) > maxItems {
			typed = typed[:maxItems]
		}
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			out = append(out, sanitizeFreeStateValue(child, maxItems, depth+1))
		}
		return out
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprint(value)
		}
		var normalized any
		if json.Unmarshal(data, &normalized) == nil {
			if _, changed := normalized.(map[string]any); changed {
				return sanitizeFreeStateValue(normalized, maxItems, depth+1)
			}
			if _, changed := normalized.([]any); changed {
				return sanitizeFreeStateValue(normalized, maxItems, depth+1)
			}
		}
		return value
	}
}

func freeStateForbiddenField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(key, "raw_") || strings.HasSuffix(key, "_samples") {
		return true
	}
	switch key {
	case "pcm", "pcm_data", "samples", "audio_buffer", "waveform_array", "spectrogram_tiles", "time_segments", "event_list", "observation_path", "board_path", "context_pack_path", "file_path", "source_path":
		return true
	default:
		return false
	}
}

func targetIDFromBinding(binding map[string]any, fallback string) string {
	return firstNonEmptyString(stringValue(anyMap(binding["target_ref"])["id"]), fallback)
}

func anyMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, _ := json.Marshal(value)
	var row map[string]any
	_ = json.Unmarshal(data, &row)
	return row
}

func cloneAnyMap(row map[string]any) map[string]any {
	out := make(map[string]any, len(row)+1)
	for key, value := range row {
		out[key] = value
	}
	return out
}

func stringSlice(value any) []string {
	values := []string{}
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			values = append(values, stringValue(item))
		}
	}
	return uniqueNonEmpty(values)
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func compactID(values ...string) string {
	value := firstNonEmptyString(values...)
	if value == "" {
		return "unbound"
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, value)
	if len(value) > 80 {
		value = value[:80]
	}
	return value
}

func jsonSize(value any) int {
	data, _ := json.Marshal(value)
	return len(data)
}

func SortedFreeStateObservationViewIDs() []string {
	ids := []string{}
	for _, def := range freeStateViewDefinitions("target") {
		ids = append(ids, def.view.ViewID)
	}
	sort.Strings(ids)
	return ids
}
