package executionverifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	return v.verifyFreshMOM(ctx, actionSet, verifyStaticBalanceMOM)
}

func (v HarnessAcoustic) VerifyPanLayout(ctx context.Context, actionSet orchestration.ActionSet) (AcousticResult, error) {
	return v.verifyFreshMOM(ctx, actionSet, verifyPanLayoutMOM)
}

type freshMOMPolicy func(map[string]any, orchestration.ActionSet) (string, string)

func (v HarnessAcoustic) verifyFreshMOM(ctx context.Context, actionSet orchestration.ActionSet, policy freshMOMPolicy) (AcousticResult, error) {
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
	relation := verifierMap(projection["multitrack_relation"])
	result.MOMStatus = strings.ToLower(verifierString(relation, "status"))
	result.EvidenceRefs = appendUniqueVerifierRefs(result.EvidenceRefs, verifierStrings(relation["evidence_refs"])...)
	if result.MOMStatus == "" {
		result.MOMStatus = "missing"
		result.Summary = "fresh observation omitted MOM multitrack relation status"
		return result, nil
	}
	if policy == nil {
		result.Summary = "fresh MOM verification policy is missing"
		return result, nil
	}
	result.Status, result.Summary = policy(relation, actionSet)
	if result.Summary != "" {
		result.Summary = fmt.Sprintf("fresh observation %s at %s: %s; musical acceptance remains unknown", observationID, result.ObservationRevision, result.Summary)
	}
	return result, nil
}

// verifyStaticBalanceMOM mirrors B2 admission semantics. Static balance needs
// a fresh full-project level relationship, not complete L3 spectral/stereo
// evidence. A generic MOM partial status is acceptable only when the level
// projection is ready and the compared-track inventory is effectively full.
func verifyStaticBalanceMOM(relation map[string]any, actionSet orchestration.ActionSet) (string, string) {
	status := verifierStatus(relation)
	if verifierStatusUnsafe(status) {
		return "fail", "B2 MOM multitrack relation is " + status
	}
	compared := verifierRows(relation["compared_tracks"])
	trackCount := verifierInt(relation["track_count"])
	if trackCount < 2 || len(compared) < 2 {
		return "inconclusive", "B2 MOM omitted a comparable full-project track inventory"
	}
	if float64(len(compared))/float64(trackCount) < 0.95 {
		return "inconclusive", fmt.Sprintf("B2 MOM compared-track coverage is %d/%d", len(compared), trackCount)
	}
	level := verifierMap(relation["level_distribution"])
	if verifierStatus(level) != mom.StatusReady {
		return "inconclusive", "B2 level_distribution is " + firstVerifierText(verifierStatus(level), "missing")
	}
	indexed := verifierTrackIndex(compared)
	for _, action := range actionSet.Actions {
		if indexed[strings.TrimSpace(action.TargetRef)] == nil {
			return "inconclusive", "B2 fresh MOM inventory omitted action target " + strings.TrimSpace(action.TargetRef)
		}
	}
	return "pass", fmt.Sprintf("B2 level relationship verified with ready level_distribution and %d/%d compared tracks; absent band/stereo evidence remains an optional limitation", len(compared), trackCount)
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
