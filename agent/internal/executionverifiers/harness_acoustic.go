package executionverifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/protocolvalue"
)

// HarnessInvoker is deliberately narrower than *harness.Harness so the
// verifier can be tested without Chat or a live project foundation.
type HarnessInvoker interface {
	Invoke(context.Context, harness.InvokeRequest) (harness.InvokeResponse, error)
}

// HarnessAcoustic performs the independent, read-only post-execution
// observation required by B2. PreviousObservationID is mandatory: without a
// before identity, a successful mix.observe call cannot prove freshness.
type HarnessAcoustic struct {
	Invoker               HarnessInvoker
	PreviousObservationID string
	MixSessionID          string
	GoalText              string
}

func (v HarnessAcoustic) VerifyStaticBalance(ctx context.Context, actionSet orchestration.ActionSet) (AcousticResult, error) {
	result, err := v.verifyFreshMOM(ctx, actionSet, "static_level_relationship", verifyStaticBalanceMOM)
	if err != nil || result.RelationshipStatus != "fail" || strings.TrimSpace(result.ObservationID) == "" {
		return result, err
	}

	// The effective-level projection can briefly lag behind the structural
	// project snapshot while a preceding B1 analysis refresh is still
	// converging. A single contradictory hierarchy sample must therefore not
	// turn 29/29 applied receipts plus an exact fader readback into a terminal
	// B2 failure. Require the contradiction to survive one independently fresh
	// MOM observation before treating it as acoustic evidence of failure.
	retry := v
	retry.PreviousObservationID = result.ObservationID
	confirmed, retryErr := retry.verifyFreshMOM(ctx, actionSet, "static_level_relationship", verifyStaticBalanceMOM)
	confirmed.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, confirmed.EvidenceRefs...)
	if retryErr != nil {
		return confirmed, retryErr
	}
	if confirmed.RelationshipStatus != "fail" {
		confirmed.Summary = firstVerifierText(
			confirmed.Summary+"; initial contradictory hierarchy sample was not reproduced",
			"initial contradictory hierarchy sample was not reproduced",
		)
	}
	return confirmed, nil
}

func (v HarnessAcoustic) VerifyPanLayout(ctx context.Context, actionSet orchestration.ActionSet) (AcousticResult, error) {
	return v.verifyFreshMOM(ctx, actionSet, "multitrack_relation", verifyPanLayoutMOM)
}

type freshMOMPolicy func(map[string]any, orchestration.ActionSet) (string, string)

func (v HarnessAcoustic) verifyFreshMOM(ctx context.Context, actionSet orchestration.ActionSet, relationKey string, policy freshMOMPolicy) (AcousticResult, error) {
	result := AcousticResult{Status: "inconclusive", MOMStatus: "not_run"}
	if v.Invoker == nil {
		return result, fmt.Errorf("harness invoker is required")
	}
	previousID := strings.TrimSpace(v.PreviousObservationID)
	if previousID == "" {
		result.Summary = "fresh acoustic verification requires a previous observation id"
		return result, nil
	}

	response, err := v.Invoker.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"scope":                "full_project",
			"project_context":      true,
			"observation_only":     true,
			"disclosure":           "digest_catalog",
			"mom_intent":           mom.IntentProjectMultitrackObservation,
			"previous_observation": previousID,
			"mix_session_id":       strings.TrimSpace(v.MixSessionID),
			"goal_text":            strings.TrimSpace(v.GoalText),
		},
		Context: map[string]any{
			"capability_runtime_v1": true,
			"observation_only":      true,
		},
		Source:     "capability_runtime_v1_verifier",
		Confirmed:  true,
		ToolCallID: "verify:" + actionSet.ID + ":mix.observe",
	})
	if err != nil {
		result.Summary = "mix.observe could not produce post-execution evidence: " + err.Error()
		return result, nil
	}
	if !strings.EqualFold(strings.TrimSpace(response.Status), "ok") {
		result.Summary = "mix.observe returned " + firstVerifierText(response.Status, response.Error, "non-ok status")
		return result, nil
	}

	observationID := verifierString(response.Result, "observation_id")
	result.ObservationID = observationID
	result.ObservationRevision = observationRevision(response.Result)
	if observationID == "" {
		result.Status = "fail"
		result.Summary = "mix.observe response omitted observation_id"
		return result, nil
	}
	result.EvidenceRefs = append(result.EvidenceRefs, "mix.observe:"+observationID)
	if observationID == previousID {
		result.Status = "fail"
		result.Summary = "mix.observe reused the pre-execution observation id"
		return result, nil
	}
	if result.ObservationRevision == "" {
		result.Summary = "post-execution observation has a new id but no revision or creation timestamp"
		return result, nil
	}
	result.EvidenceRefs = append(result.EvidenceRefs, "mix.observe.revision:"+result.ObservationRevision)

	projection := verifierMap(response.Result["mom_projection"])
	if len(projection) == 0 {
		projection = verifierMap(verifierMap(response.Result["observation"])["mom_projection"])
	}
	if verifierString(projection, "intent") != mom.IntentProjectMultitrackObservation {
		result.Summary = "post-execution MOM projection is missing the project multitrack intent"
		return result, nil
	}
	relation := verifierMap(projection[relationKey])
	result.MOMStatus = strings.ToLower(verifierString(relation, "status"))
	result.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, verifierStrings(relation["evidence_refs"])...)
	if result.MOMStatus == "" {
		result.MOMStatus = "missing"
		result.Summary = "fresh observation omitted MOM " + relationKey + " status"
		return result, nil
	}
	if policy == nil {
		result.Summary = "fresh MOM verification policy is missing"
		return result, nil
	}
	result.Status, result.Summary = policy(relation, actionSet)
	if result.Status == "pass" {
		result.RelationshipStatus, result.RelationshipSummary = verifyStaticBalanceHierarchy(relation, actionSet)
		switch result.RelationshipStatus {
		case "fail":
			result.Status = "fail"
		case "inconclusive":
			result.Status = "inconclusive"
		}
		if result.RelationshipSummary != "" {
			result.Summary = firstVerifierText(result.Summary+"; "+result.RelationshipSummary, result.RelationshipSummary)
		}
	}
	if result.Summary != "" {
		result.Summary = fmt.Sprintf("fresh observation %s at %s: %s; musical acceptance remains unknown", observationID, result.ObservationRevision, result.Summary)
	}
	return result, nil
}

type hierarchySample struct {
	before float64
	after  float64
	delta  float64
}

// verifyStaticBalanceHierarchy checks the acoustic realization of the exact
// approved functional correction. It does not impose a universal instrument
// loudness order: the expected direction comes from the frozen B2 candidate.
func verifyStaticBalanceHierarchy(relation map[string]any, actionSet orchestration.ActionSet) (string, string) {
	compared := verifierTrackIndex(verifierRows(relation["tracks"]))
	byFunction := map[string][]hierarchySample{}
	metadataCount := 0
	for _, action := range actionSet.Actions {
		function := verifierString(action.Args, "hierarchy_function")
		if function == "" {
			continue
		}
		metadataCount++
		before, beforeOK := verifierFloat(action.Args["before_effective_level_db"])
		delta, deltaOK := verifierFloat(action.Args["delta_db"])
		row := compared[strings.TrimSpace(action.TargetRef)]
		after, afterOK := verifierFirstFloat(row, "effective_static_rms_dbfs")
		if !beforeOK || !deltaOK || !afterOK {
			return "inconclusive", fmt.Sprintf("B2 hierarchy evidence is incomplete for %s/%s", action.TargetRef, function)
		}
		beforeMetric := verifierString(action.Args, "before_effective_level_metric")
		afterMetric := verifierString(row, "metric")
		beforeTap := verifierString(action.Args, "before_effective_level_tap_point")
		afterTap := verifierString(row, "tap_point")
		if beforeMetric == "" || afterMetric == "" || !strings.EqualFold(beforeMetric, afterMetric) || beforeTap == "" || afterTap == "" || !strings.EqualFold(beforeTap, afterTap) {
			return "inconclusive", fmt.Sprintf("B2 hierarchy metric/tap changed for %s/%s (before %s@%s, after %s@%s)", action.TargetRef, function, beforeMetric, beforeTap, afterMetric, afterTap)
		}
		rowStatus := verifierStatus(row)
		if rowStatus != mom.StatusReady && rowStatus != mom.StatusApprox {
			return "inconclusive", fmt.Sprintf("B2 hierarchy projection for %s/%s is %s", action.TargetRef, function, firstVerifierText(rowStatus, "missing"))
		}
		observedDelta := after - before
		if math.Abs(delta) >= 0.125 {
			if observedDelta*delta <= 0 || math.Abs(observedDelta-delta) > 0.75 {
				return "fail", fmt.Sprintf("B2 hierarchy correction for %s/%s moved %.3f dB, expected %.3f dB", action.TargetRef, function, observedDelta, delta)
			}
		}
		byFunction[function] = append(byFunction[function], hierarchySample{before: before, after: after, delta: delta})
	}
	if metadataCount == 0 {
		return "not_assessed", ""
	}
	functions := make([]string, 0, len(byFunction))
	for function := range byFunction {
		functions = append(functions, function)
	}
	sort.Strings(functions)
	pairCount := 0
	for i := 0; i < len(functions); i++ {
		for j := i + 1; j < len(functions); j++ {
			left, right := byFunction[functions[i]], byFunction[functions[j]]
			expectedShift := hierarchyMedianDelta(left) - hierarchyMedianDelta(right)
			if math.Abs(expectedShift) < 0.125 {
				continue
			}
			pairCount++
			observedShift := (hierarchyMedianAfter(left) - hierarchyMedianAfter(right)) - (hierarchyMedianBefore(left) - hierarchyMedianBefore(right))
			if observedShift*expectedShift <= 0 || math.Abs(observedShift-expectedShift) > 1.0 {
				return "fail", fmt.Sprintf("B2 %s/%s relationship moved %.3f dB, expected %.3f dB", functions[i], functions[j], observedShift, expectedShift)
			}
		}
	}
	if len(functions) < 2 || pairCount == 0 {
		return "inconclusive", "B2 post-observation did not expose two differently corrected functions for hierarchy verification"
	}
	return "pass", fmt.Sprintf("B2 approved functional hierarchy was acoustically realized across %d functions and %d relative comparisons", len(functions), pairCount)
}

func hierarchyMedianBefore(samples []hierarchySample) float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.before)
	}
	return verifierMedian(values)
}

func hierarchyMedianAfter(samples []hierarchySample) float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.after)
	}
	return verifierMedian(values)
}

func hierarchyMedianDelta(samples []hierarchySample) float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, sample.delta)
	}
	return verifierMedian(values)
}

func verifierMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2
	}
	return values[mid]
}

func verifierFirstFloat(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := verifierFloat(row[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func verifierFloat(value any) (float64, bool) {
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
	default:
		return 0, false
	}
}

// verifyStaticBalanceMOM consumes the same typed MOM projection used for B2
// planning. Missing, stale, suspect, or partial evidence below the B2 95%
// usable-coverage contract requests review; it never falls through to generic
// multitrack fields or raw acoustic packages.
func verifyStaticBalanceMOM(relation map[string]any, actionSet orchestration.ActionSet) (string, string) {
	status := verifierStatus(relation)
	if status != mom.StatusReady && status != mom.StatusApprox && status != mom.StatusPartial {
		return "inconclusive", "B2 MOM static-level relationship is " + firstVerifierText(status, "missing")
	}
	compared := verifierRows(relation["tracks"])
	coverage := verifierMap(relation["coverage"])
	trackCount := verifierInt(coverage["track_count"])
	if trackCount == 0 {
		trackCount = len(compared)
	}
	if trackCount < 2 || len(compared) < 2 {
		return "inconclusive", "B2 MOM static-level projection omitted a comparable full-project track inventory"
	}
	usableCount := 0
	for _, row := range compared {
		rowStatus := verifierStatus(row)
		if rowStatus != mom.StatusReady && rowStatus != mom.StatusApprox {
			continue
		}
		if _, ok := verifierFirstFloat(row, "effective_static_rms_dbfs"); ok {
			usableCount++
		}
	}
	if float64(usableCount)/float64(trackCount) < 0.95 {
		return "inconclusive", fmt.Sprintf("B2 MOM usable static-level coverage is %d/%d", usableCount, trackCount)
	}
	indexed := verifierTrackIndex(compared)
	for _, action := range actionSet.Actions {
		row := indexed[strings.TrimSpace(action.TargetRef)]
		if row == nil {
			return "inconclusive", "B2 fresh MOM static-level projection omitted action target " + strings.TrimSpace(action.TargetRef)
		}
		if rowStatus := verifierStatus(row); rowStatus != mom.StatusReady && rowStatus != mom.StatusApprox {
			return "inconclusive", "B2 fresh MOM static-level target " + strings.TrimSpace(action.TargetRef) + " is " + firstVerifierText(rowStatus, "missing")
		}
	}
	return "pass", fmt.Sprintf("B2 typed static-level relationship verified for %d/%d usable tracks", usableCount, trackCount)
}

// verifyPanLayoutMOM deliberately keeps B3 stricter than B2: a pan change
// requires a fresh stereo relationship projection for post-execution review.
func verifyPanLayoutMOM(relation map[string]any, actionSet orchestration.ActionSet) (string, string) {
	status := verifierStatus(relation)
	if verifierStatusUnsafe(status) {
		return "fail", "B3 MOM multitrack relation is " + status
	}
	stereo := verifierMap(relation["stereo_distribution"])
	if verifierStatus(stereo) != mom.StatusReady {
		return "inconclusive", "B3 stereo_distribution is " + firstVerifierText(verifierStatus(stereo), "missing")
	}
	compared := verifierRows(relation["compared_tracks"])
	indexed := verifierTrackIndex(compared)
	for _, action := range actionSet.Actions {
		if indexed[strings.TrimSpace(action.TargetRef)] == nil {
			return "inconclusive", "B3 fresh MOM inventory omitted action target " + strings.TrimSpace(action.TargetRef)
		}
	}
	return "pass", fmt.Sprintf("B3 stereo relationship verified for %d compared tracks", len(compared))
}

func observationRevision(result map[string]any) string {
	projection := verifierMap(result["mom_projection"])
	project := verifierMap(projection["project_structure"])
	trust := verifierMap(projection["trust_quality"])
	observation := verifierMap(result["observation"])
	catalog := verifierMap(result["catalog"])
	return firstVerifierText(
		verifierString(project, "render_revision", "source_revision", "clip_revision"),
		verifierString(trust, "render_revision", "source_revision", "clip_revision", "updated_at"),
		verifierString(observation, "created_at"),
		verifierString(catalog, "generated_at"),
	)
}

func verifierMap(value any) map[string]any {
	return protocolvalue.Object(value)
}

func verifierString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fmt.Sprint(row[key])); value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func verifierStrings(value any) []string {
	var out []string
	switch values := value.(type) {
	case []string:
		out = append(out, values...)
	case []any:
		for _, value := range values {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	}
	return out
}

func verifierStatus(row map[string]any) string {
	return strings.ToLower(verifierString(row, "status", "freshness"))
}

func verifierStatusUnsafe(status string) bool {
	for _, token := range []string{mom.StatusStale, "suspect", "invalid", "fail", "error"} {
		if strings.Contains(status, token) {
			return true
		}
	}
	return false
}

func verifierRows(value any) []map[string]any {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil
	}
	return rows
}

func verifierTrackIndex(rows []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		if id := verifierString(row, "track_id", "id"); id != "" {
			out[id] = row
		}
	}
	return out
}

func verifierInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(math.Round(typed))
	case float32:
		return int(math.Round(float64(typed)))
	default:
		return 0
	}
}

func appendUniqueVerifierRefs(base []string, values ...string) []string {
	seen := make(map[string]bool, len(base)+len(values))
	for _, value := range base {
		seen[value] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			base = append(base, value)
			seen[value] = true
		}
	}
	return base
}

func firstVerifierText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
