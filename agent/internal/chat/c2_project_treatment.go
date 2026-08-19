package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorregistry"
)

const (
	// A C2 target decision needs the complete compact dynamic view bundle. This
	// matches the existing C2 target-confirmation and post-action CCB budget;
	// 2048 bytes retained only receipts and omissions, not acoustic evidence.
	c2ProjectTrackDisclosureBytes  = 8192
	c2ProjectDynamicCandidateLimit = 4
	c2ProjectTreatmentMaxTokens    = 8000
	c2ProjectTreatmentTimeout      = 3 * time.Minute
	c2ProjectTreatmentRetryDelay   = 250 * time.Millisecond
	c2ProjectTreatmentMaxAttempts  = 2
	c2ProjectTreatmentMaxRepairs   = 2
	c2ObservationReadyTimeout      = 3 * time.Minute
	c2ObservationPollInterval      = 500 * time.Millisecond
	c2DiscoveryMaxAttempts         = 3
	c2DiscoveryRetryDelay          = 750 * time.Millisecond
)

// c2ObservationPendingError means the project has not failed C2.  Its
// acoustic evidence is still being prepared and the same project session can
// safely resume from observation without any mutation having occurred.
type c2ObservationPendingError struct {
	Reason string
}

func (e *c2ObservationPendingError) Error() string {
	if e == nil || strings.TrimSpace(e.Reason) == "" {
		return "C2 observation is still preparing"
	}
	return "C2 observation is still preparing: " + strings.TrimSpace(e.Reason)
}

// c2DiscoverProjectTreatment is C2's fixed project observation stage.  It
// deliberately collects the same dynamic view set for every project track
// before the model is allowed to choose a treatment target.  Plugin inventory
// is not read here: PCA remains an execution-resource decision after this
// project treatment is frozen.
func (s *Server) c2DiscoverProjectTreatment(ctx context.Context, conversationID, goalText string, state *kernel.VSPStateResult, requestedTrack, requestedFamily string) (dynamiccontrol.ProjectTreatment, map[string]any, error) {
	if s == nil || s.harness == nil || s.llm == nil || state == nil {
		return dynamiccontrol.ProjectTreatment{}, nil, fmt.Errorf("C2 project observation is unavailable")
	}
	trackIDs := c2ProjectTrackIDs(state.LegacyState, requestedTrack)
	if len(trackIDs) == 0 {
		return dynamiccontrol.ProjectTreatment{}, nil, fmt.Errorf("project has no observable audio tracks")
	}
	if _, readyErr := s.c2EnsureProjectAudioObservationReady(ctx, conversationID, goalText); readyErr != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, readyErr
	}
	projectObserve, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "mix.observe", Args: map[string]any{
		"scope": "full_project", "project_context": true, "observation_only": true,
		"disclosure": "digest_catalog", "mom_intent": "action_preflight_observation",
		"mix_session_id": "c2_" + sanitizeCanaryID(conversationID), "goal_text": goalText,
	}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "c2_fixed_observation": true, "observation_only": true}, Source: "c2_project_observation", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":mix.observe"})
	if err != nil || !strings.EqualFold(projectObserve.Status, "ok") {
		return dynamiccontrol.ProjectTreatment{}, nil, fmt.Errorf("full-project mix.observe: %s", firstNonEmpty(errorText(err), projectObserve.Error, projectObserve.Status))
	}
	// The initial mix observation is full-project.  Detail CCB reads are then
	// limited to acoustically ranked candidates from that immutable observation;
	// reading every detailed view for every track is both redundant and too slow
	// for a project capability interaction.
	candidateTrackIDs := c2ProjectDynamicCandidateTrackIDs(projectObserve.Result, trackIDs)
	rows, collectErr := s.c2CollectProjectDynamicBundles(ctx, conversationID, goalText, state, candidateTrackIDs)
	if collectErr != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, collectErr
	}
	projectDigest := c2ProjectObservationDigest(projectObserve.Result)
	coverageVocabulary, vocabularyErr := s.c2ExecutableCoverageVocabulary(ctx, requestedFamily)
	if vocabularyErr != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, vocabularyErr
	}
	coverageOptions, optionsErr := s.c2ExecutableCoverageOptions(ctx, requestedFamily)
	if optionsErr != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, optionsErr
	}
	allowedFamilies := c2FamiliesWithExecutableCoverage(coverageVocabulary)
	if len(allowedFamilies) == 0 {
		return dynamiccontrol.ProjectTreatment{}, nil, fmt.Errorf("C2 has no locally PCA-covered dynamic control family")
	}
	inputRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		inputRows = append(inputRows, map[string]any{
			"track_id": firstStringFromMap(row, "track_id"), "track_name": firstStringFromMap(row, "track_name"),
			"allowed_families": allowedFamilies, "observation": c2ProjectTrackBundleDigest(firstMapFromAny(row["bundle"])),
		})
	}
	input := map[string]any{"schema_version": dynamiccontrol.ProjectPlanSchema, "user_request": goalText,
		"project_observation": projectDigest, "observation_id": firstStringFromMap(projectObserve.Result, "observation_id"), "tracks": inputRows, "coverage_vocabulary_by_family": coverageVocabulary, "coverage_options_by_family": coverageOptions}
	payload, _ := json.Marshal(input)
	cfg, _, cfgErr := config.Load()
	if cfgErr != nil || !cfg.Complete() {
		return dynamiccontrol.ProjectTreatment{}, nil, firstNonNilError(cfgErr, fmt.Errorf("AI configuration is incomplete"))
	}
	system := `You are C2, the fixed project-level dynamic-control stage in the A-F mixing capability workflow. Return ONLY compact fine_mix.dynamic_control.project_treatment.v1 JSON. This is neither free-state advice nor a per-track chat loop. All supplied rows were observed before this decision. Return one project treatment containing every justified dynamic target; each target must use an exact supplied track_id and one allowed family. For every target evidence_refs and semantic_processor_intent.evidence_refs, copy only that target row's exact CCB observation_id. Never use an audit receipt id, a nested evidence ref, or an invented identifier. required_coverage must be a non-empty minimal list copied only from coverage_vocabulary_by_family for the selected family and must be a subset of one entry in coverage_options_by_family for that family. The options are anonymous executable semantic shapes, not plugin choices. Do not combine axes from different entries. Do not mention plugins, PCA, vendors, or parameters. Those are resolved after this project treatment. status=no_action is allowed only when the supplied per-track dynamic evidence is complete enough to support that conclusion. Do not turn missing, stale, deferred, or partial observation into no_action; such a row requires a conservative targeted treatment only when other supplied evidence justifies it, otherwise return a boundary error by using no targets is forbidden. Keep summary, listening_goal, intent, and rationale below 160 characters each. Shape: {"schema_version":"fine_mix.dynamic_control.project_treatment.v1","status":"targeted|no_action","summary":"short project rationale","observation_id":"exact supplied project observation id","evidence_refs":["project observation id"],"targets":[{"schema_version":"fine_mix.dynamic_control.hypothesis.v1","status":"targeted","track_id":"exact supplied id","listening_goal":"audible dynamic goal","rationale":"evidence-grounded reason","evidence_refs":["exact target CCB observation_id"],"semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"exact allowed family","intent":"bounded acoustic intent","required_coverage":["exact registered family coverage axis"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.0,"evidence_refs":["exact target CCB observation_id"]}}]}`
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, Metadata: llm.RequestMetadata{Source: "c2_project_treatment_planner", ConversationID: conversationID}, MaxOutputTokens: c2ProjectTreatmentMaxTokens, PreferJSON: true, Timeout: c2ProjectTreatmentTimeout}
	response, err := s.c2CompleteProjectTreatmentRequest(ctx, cfg, request)
	if err != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, err
	}
	families := map[string]map[string]bool{}
	for _, id := range candidateTrackIDs {
		families[id] = map[string]bool{}
		for _, family := range allowedFamilies {
			families[id][family] = true
		}
	}
	decode := func(text string) (dynamiccontrol.ProjectTreatment, error) {
		var treatment dynamiccontrol.ProjectTreatment
		if err := decodeJSONObject(text, &treatment); err != nil {
			return treatment, err
		}
		if err := treatment.Validate(families); err != nil {
			return treatment, err
		}
		if treatment.ObservationID != firstStringFromMap(projectObserve.Result, "observation_id") {
			return treatment, fmt.Errorf("project treatment cited an unexpected observation")
		}
		for _, target := range treatment.Targets {
			if err := c2ValidateHypothesisEvidence(target, rows); err != nil {
				return treatment, err
			}
			if err := c2ValidateRequiredCoverage(coverageVocabulary, coverageOptions, target); err != nil {
				return treatment, err
			}
		}
		if treatment.Status == dynamiccontrol.HypothesisNoAction && !c2AllProjectDynamicBundlesReady(rows) {
			return treatment, fmt.Errorf("no_action is not permitted while project dynamic observation is incomplete")
		}
		return treatment, nil
	}
	treatment, decodeErr := decode(response.Text)
	for repairAttempt := 0; decodeErr != nil && repairAttempt < c2ProjectTreatmentMaxRepairs; repairAttempt++ {
		request.Messages = append(request.Messages,
			llm.Message{Role: "assistant", Content: response.Text},
			llm.Message{Role: "user", Content: "Your project treatment was rejected by the deterministic C2 boundary: " + decodeErr.Error() + ". Return ONLY one corrected project treatment JSON for the same observations."},
		)
		response, err = s.c2CompleteProjectTreatmentRequest(ctx, cfg, request)
		if err != nil {
			return dynamiccontrol.ProjectTreatment{}, nil, err
		}
		treatment, decodeErr = decode(response.Text)
	}
	if decodeErr != nil {
		return dynamiccontrol.ProjectTreatment{}, nil, fmt.Errorf("C2 project treatment remained invalid after %d repair attempts: %w", c2ProjectTreatmentMaxRepairs, decodeErr)
	}
	return treatment, map[string]any{"project_observation": projectDigest, "tracks": rows, "project_track_count": len(trackIDs), "candidate_track_count": len(rows)}, nil
}

func (s *Server) c2EnsureProjectAudioObservationReady(ctx context.Context, conversationID, goalText string) (map[string]any, error) {
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "project.audio_analysis_status",
		Args: map[string]any{
			"latest":           true,
			"ensure_ready":     true,
			"timeout_ms":       int(c2ObservationReadyTimeout / time.Millisecond),
			"poll_interval_ms": int(c2ObservationPollInterval / time.Millisecond),
		},
		Context: map[string]any{
			"capability_runtime_v1": true,
			"capability_id":         dynamicControlCapabilityID,
			"c2_fixed_observation":  true,
			"observation_only":      true,
		},
		Source:     "c2_project_observation_readiness",
		Confirmed:  true,
		ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":audio-analysis-ready",
	})
	if err != nil {
		return nil, &c2ObservationPendingError{Reason: err.Error()}
	}
	if !strings.EqualFold(response.Status, "ok") {
		return nil, &c2ObservationPendingError{Reason: firstNonEmpty(response.Error, response.Status, "audio analysis is not ready")}
	}
	return response.Result, nil
}

// c2CompleteProjectTreatmentRequest retries only a read-only planning request
// after a transport/service failure.  A returned treatment is never trusted by
// this helper: the caller still applies the normal C2 schema, evidence, and
// executable-coverage boundaries before it can reach any mutation path.
func (s *Server) c2CompleteProjectTreatmentRequest(ctx context.Context, cfg config.EngineConfig, request llm.Request) (llm.Response, error) {
	var response llm.Response
	var err error
	for attempt := 0; attempt < c2ProjectTreatmentMaxAttempts; attempt++ {
		response, err = s.llm.CompleteRequest(ctx, cfg, request)
		if err == nil || !c2TransientProjectTreatmentError(err) || attempt+1 == c2ProjectTreatmentMaxAttempts {
			return response, err
		}
		timer := time.NewTimer(c2ProjectTreatmentRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return llm.Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return response, err
}

func c2TransientProjectTreatmentError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, marker := range []string{
		"internal server error", "500", "502", "503", "504",
		"context deadline exceeded", "timeout", "timed out", "awaiting headers",
		"temporary", "stream returned no output", "connection reset",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func c2CoverageVocabulary(requestedFamily string) (map[string][]string, error) {
	registry, err := processorregistry.Default()
	if err != nil {
		return nil, err
	}
	vocabulary := map[string][]string{}
	for _, family := range c2SupportedFamilies(requestedFamily) {
		definition, ok := registry.Resolve(family)
		if !ok {
			return nil, fmt.Errorf("C2 family %s is not registered", family)
		}
		vocabulary[family] = append([]string(nil), definition.CoverageVocabulary...)
	}
	return vocabulary, nil
}

// c2ExecutableCoverageVocabulary is a capability projection, not plugin
// selection. It removes semantic axes for which no currently installed,
// PCA-promoted binary has passed the required action test. The model receives
// neither candidate identities nor binary details; exact binding remains after
// the project treatment is frozen.
func (s *Server) c2ExecutableCoverageVocabulary(ctx context.Context, requestedFamily string) (map[string][]string, error) {
	base, err := c2CoverageVocabulary(requestedFamily)
	if err != nil {
		return nil, err
	}
	result := map[string][]string{}
	for _, family := range c2SupportedFamilies(requestedFamily) {
		processorType := c2ProcessorTypeForFamily(family)
		if processorType == "" {
			continue
		}
		candidates, err := s.localPluginRecommendationCandidates(ctx, processorType)
		if err != nil {
			return nil, err
		}
		admitted, err := admittedPluginRecommendationCandidates(candidates, processorType)
		if err != nil {
			return nil, err
		}
		available, err := c2CoverageAxesForAdmittedCandidates(admitted, family)
		if err != nil {
			return nil, err
		}
		for _, axis := range base[family] {
			if available[axis] {
				result[family] = append(result[family], axis)
			}
		}
	}
	return result, nil
}

// c2ExecutableCoverageOptions projects anonymous PCA-certified semantic
// shapes. A treatment can choose a subset of one shape, but must never merge
// axes that happen to be certified by different binaries.
func (s *Server) c2ExecutableCoverageOptions(ctx context.Context, requestedFamily string) (map[string][][]string, error) {
	base, err := c2CoverageVocabulary(requestedFamily)
	if err != nil {
		return nil, err
	}
	result := map[string][][]string{}
	for _, family := range c2SupportedFamilies(requestedFamily) {
		processorType := c2ProcessorTypeForFamily(family)
		if processorType == "" {
			continue
		}
		candidates, err := s.localPluginRecommendationCandidates(ctx, processorType)
		if err != nil {
			return nil, err
		}
		admitted, err := admittedPluginRecommendationCandidates(candidates, processorType)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for _, candidate := range admitted {
			available, err := c2CoverageAxesForAdmittedCandidates([]pluginRecommendationCandidate{candidate}, family)
			if err != nil {
				return nil, err
			}
			option := make([]string, 0, len(base[family]))
			for _, axis := range base[family] {
				if available[axis] {
					option = append(option, axis)
				}
			}
			if len(option) == 0 {
				continue
			}
			key := strings.Join(option, "\x00")
			if !seen[key] {
				seen[key] = true
				result[family] = append(result[family], option)
			}
		}
		sort.Slice(result[family], func(i, j int) bool {
			return strings.Join(result[family][i], "\x00") < strings.Join(result[family][j], "\x00")
		})
	}
	return result, nil
}

func c2FamiliesWithExecutableCoverage(vocabulary map[string][]string) []string {
	families := make([]string, 0, len(vocabulary))
	for family, axes := range vocabulary {
		if len(axes) > 0 {
			families = append(families, family)
		}
	}
	sort.Strings(families)
	return families
}

func c2CoverageAxesForAdmittedCandidates(candidates []pluginRecommendationCandidate, family string) (map[string]bool, error) {
	matched := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.AttestationID != "" {
			matched[candidate.AttestationID] = true
		}
	}
	axes := map[string]bool{}
	if len(matched) == 0 {
		return axes, nil
	}
	if processorattestation.IsV2Family(family) {
		store, err := processorattestation.NewStoreV2("")
		if err != nil {
			return nil, err
		}
		library, _, err := store.Read()
		if err != nil {
			return nil, err
		}
		for _, attestation := range library.Attestations {
			if attestation.Status != processorattestation.StatusPromoted || !matched[attestation.AttestationID] {
				continue
			}
			for _, coverage := range attestation.Coverage {
				if coverage.Action == "adjust" && coverage.Axis != "" {
					axes[coverage.Axis] = true
				}
			}
		}
		return axes, nil
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		return nil, err
	}
	library, _, err := store.Read()
	if err != nil {
		return nil, err
	}
	for _, attestation := range library.Attestations {
		if attestation.Status != processorattestation.StatusPromoted || !matched[attestation.AttestationID] {
			continue
		}
		for _, coverage := range attestation.Coverage {
			if coverage.Action == "adjust" && coverage.Axis != "" {
				axes[coverage.Axis] = true
			}
		}
	}
	return axes, nil
}

func c2ValidateRequiredCoverage(vocabulary map[string][]string, options map[string][][]string, target dynamiccontrol.TargetHypothesis) error {
	allowed := map[string]bool{}
	for _, axis := range vocabulary[target.Intent.Family] {
		allowed[axis] = true
	}
	if len(target.Intent.RequiredCoverage) == 0 {
		return fmt.Errorf("C2 target %s has no required coverage", target.TrackID)
	}
	if len(target.Intent.RequiredCoverage) > 4 {
		return fmt.Errorf("C2 target %s exceeds the four-axis semantic control budget", target.TrackID)
	}
	for _, axis := range target.Intent.RequiredCoverage {
		if !allowed[axis] {
			return fmt.Errorf("coverage axis %q is not registered for family %s", axis, target.Intent.Family)
		}
	}
	for _, option := range options[target.Intent.Family] {
		covered := map[string]bool{}
		for _, axis := range option {
			covered[axis] = true
		}
		allCovered := true
		for _, axis := range target.Intent.RequiredCoverage {
			if !covered[axis] {
				allCovered = false
				break
			}
		}
		if allCovered {
			return nil
		}
	}
	return fmt.Errorf("C2 target %s combines coverage that no single PCA-certified dynamic capability shape supports", target.TrackID)
}

// c2ProjectDynamicCandidateTrackIDs is an acoustic preselection boundary, not
// a processor selection boundary.  It turns the full-project observation's
// already-materialized per-track dynamics into a bounded detail-read set.
func c2ProjectDynamicCandidateTrackIDs(observation map[string]any, trackIDs []string) []string {
	allowed := map[string]bool{}
	for _, trackID := range trackIDs {
		if strings.TrimSpace(trackID) != "" {
			allowed[trackID] = true
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	scores := map[string]float64{}
	var visit func(any)
	visit = func(value any) {
		switch row := value.(type) {
		case map[string]any:
			trackID := firstStringFromMap(row, "track_id", "id")
			if trackID == "" {
				trackID = firstStringFromMap(firstMapFromAny(row["source_identity"]), "track_id")
			}
			if allowed[trackID] {
				score := c2DynamicObservationScore(row)
				if score > scores[trackID] {
					scores[trackID] = score
				}
			}
			for _, nested := range row {
				visit(nested)
			}
		case []any:
			for _, nested := range row {
				visit(nested)
			}
		}
	}
	visit(observation)
	type candidate struct {
		trackID string
		score   float64
	}
	candidates := make([]candidate, 0, len(allowed))
	for trackID := range allowed {
		candidates = append(candidates, candidate{trackID: trackID, score: scores[trackID]})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].trackID < candidates[j].trackID
		}
		return candidates[i].score > candidates[j].score
	})
	limit := c2ProjectDynamicCandidateLimit
	if len(trackIDs) == 1 {
		limit = 1
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate.trackID)
	}
	return result
}

func c2DynamicObservationScore(row map[string]any) float64 {
	// Higher peaks and crest values are useful project-level signals for where
	// detailed dynamics inspection should begin.  They are not treatment values.
	score := 0.0
	if peak, ok := c2Float(row["peak_dbfs"]); ok {
		score += 120 + peak
	}
	if crest, ok := c2Float(row["crest_db"]); ok && crest > 0 {
		score += crest
	}
	if events := firstMapFromAny(row["frequency_time_events"]); strings.EqualFold(firstStringFromMap(events, "status"), "ready") {
		score += 8
	}
	if dynamics := firstMapFromAny(row["band_dynamics"]); strings.EqualFold(firstStringFromMap(dynamics, "status"), "ready") {
		score += 6
	}
	return score
}

func c2Float(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (s *Server) c2CollectProjectDynamicBundles(ctx context.Context, conversationID, goalText string, state *kernel.VSPStateResult, trackIDs []string) ([]map[string]any, error) {
	trackRows := c2ProjectTrackRows(state.LegacyState, trackIDs)
	byID := map[string]map[string]any{}
	for _, row := range trackRows {
		byID[firstStringFromMap(row, "track_id")] = row
	}
	type result struct {
		trackID string
		bundle  map[string]any
		skipped bool
		pending string
		err     error
	}
	jobs := make(chan string)
	results := make(chan result, len(trackIDs))
	workers := 4
	if len(trackIDs) < workers {
		workers = len(trackIDs)
	}
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for trackID := range jobs {
				// A Mixboard session has one current observation binding.  Project
				// discovery reads tracks concurrently, so a shared session would let
				// one track replace another before CCB reads its views.
				var response harness.InvokeResponse
				var bundle map[string]any
				var pendingReason string
				var err error
				for attempt := 0; attempt < c2DiscoveryMaxAttempts; attempt++ {
					response, err = s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "ccb.observation_request", Args: map[string]any{"request_id": "c2-discovery-" + sanitizeCanaryID(conversationID) + "-" + sanitizeCanaryID(trackID) + fmt.Sprintf("-%d", attempt+1), "mix_session_id": c2DiscoveryMixSessionID(conversationID, trackID), "view_ids": c2DiscoveryViews, "target_kind": "track", "target_id": trackID, "max_disclosure_bytes": c2ProjectTrackDisclosureBytes, "goal_text": goalText}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "c2_discovery": true, "observation_only": true}, Source: "c2_project_dynamic_observation", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":ccb:" + sanitizeCanaryID(trackID) + fmt.Sprintf(":%d", attempt+1)})
					if err != nil || (!strings.EqualFold(response.Status, "ok") && !strings.EqualFold(response.Status, "partial")) {
						results <- result{trackID: trackID, err: fmt.Errorf("CCB dynamic observation for %s: %s", trackID, firstNonEmpty(errorText(err), response.Error, response.Status))}
						break
					}
					bundle = c2ObservationBundle(response.Result["bundle"])
					if len(bundle) == 0 {
						bundle = c2ObservationBundle(response.Result)
					}
					pendingReason = c2ObservationPendingReason(response, bundle)
					if strings.EqualFold(firstStringFromMap(response.Result, "status"), "rejected") || strings.EqualFold(firstStringFromMap(bundle, "status"), "rejected") || len(bundle) == 0 {
						if pendingReason != "" && attempt+1 < c2DiscoveryMaxAttempts {
							time.Sleep(c2DiscoveryRetryDelay)
							continue
						}
						if pendingReason != "" {
							results <- result{trackID: trackID, pending: pendingReason}
						} else {
							results <- result{trackID: trackID, skipped: true}
						}
						break
					}
					results <- result{trackID: trackID, bundle: bundle}
					break
				}
			}
		}()
	}
	go func() {
		for _, id := range trackIDs {
			jobs <- id
		}
		close(jobs)
		group.Wait()
		close(results)
	}()
	rows := make([]map[string]any, 0, len(trackIDs))
	pending := []string{}
	for item := range results {
		if item.err != nil {
			return nil, item.err
		}
		if item.skipped {
			continue
		}
		if item.pending != "" {
			pending = append(pending, item.trackID+": "+item.pending)
			continue
		}
		if len(item.bundle) == 0 {
			return nil, fmt.Errorf("CCB dynamic observation for %s returned no bundle", item.trackID)
		}
		row := byID[item.trackID]
		rows = append(rows, map[string]any{"track_id": item.trackID, "track_name": firstStringFromMap(row, "track_name", "name"), "bundle": item.bundle})
	}
	if len(pending) > 0 {
		return nil, &c2ObservationPendingError{Reason: strings.Join(pending, "; ")}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("CCB returned no usable dynamic candidate bundles")
	}
	sort.Slice(rows, func(i, j int) bool {
		return firstStringFromMap(rows[i], "track_id") < firstStringFromMap(rows[j], "track_id")
	})
	return rows, nil
}

func c2ObservationPendingReason(response harness.InvokeResponse, bundle map[string]any) string {
	values := []string{}
	appendValues := func(value any) {
		for _, item := range contextStringSlice(value) {
			if strings.TrimSpace(item) != "" {
				values = append(values, item)
			}
		}
	}
	appendValues(bundle["omission_reasons"])
	appendValues(bundle["rejection_reasons"])
	appendValues(firstMapFromAny(bundle["audit_receipt"])["rejection_reasons"])
	if strings.Contains(strings.ToLower(strings.Join(values, " ")), "observation_ready_gate_timeout") {
		return "observation_ready_gate_timeout"
	}
	return ""
}

func c2DiscoveryMixSessionID(conversationID, trackID string) string {
	return "c2_" + sanitizeCanaryID(conversationID) + "_" + sanitizeCanaryID(trackID)
}

func c2ProjectTrackBundleDigest(bundle map[string]any) map[string]any {
	budget, omitted := c2ProjectTrackDisclosureBytes, false
	value := c2DigestValue(bundle, "", 0, &budget, &omitted)
	out, _ := value.(map[string]any)
	if out == nil {
		out = map[string]any{}
	}
	out["omitted_for_context_budget"] = omitted
	return out
}

func c2AllProjectDynamicBundlesReady(rows []map[string]any) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		bundle := firstMapFromAny(row["bundle"])
		if !strings.EqualFold(firstStringFromMap(bundle, "status"), "ready") && !strings.EqualFold(firstStringFromMap(bundle, "status"), "ok") {
			return false
		}
	}
	return true
}
