// Package executionverifiers contains independent postcondition checks for
// capability executions.
package executionverifiers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type StateReader interface {
	VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error)
}

type AcousticVerifier interface {
	VerifyStaticBalance(context.Context, orchestration.ActionSet) (AcousticResult, error)
}

// AcousticResult is evidence that a fresh read-only observation was produced
// after mutation. Pass means the observation contract was satisfied; it does
// not mean the user has accepted the musical result.
type AcousticResult struct {
	Status              string
	ObservationID       string
	ObservationRevision string
	MOMStatus           string
	EvidenceRefs        []string
	Summary             string
	RelationshipStatus  string
	RelationshipSummary string
}

type StaticBalance struct {
	State    StateReader
	Acoustic AcousticVerifier
}

func (v StaticBalance) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if v.State == nil {
		return result, fmt.Errorf("state reader is required")
	}
	if len(receipts) != len(actionSet.Actions) {
		result.Structural = "fail"
		return result, fmt.Errorf("receipt coverage mismatch")
	}
	for _, receipt := range receipts {
		if !strings.EqualFold(receipt.Status, "applied") {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s was not applied", receipt.ActionID)
		}
	}
	state, err := v.State.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return result, fmt.Errorf("read structural postcondition: %w", err)
	}
	tracks := indexTracks(state.LegacyState["tracks"])
	for _, action := range actionSet.Actions {
		row := tracks[action.TargetRef]
		if row == nil {
			result.Structural = "fail"
			return result, fmt.Errorf("target track %s missing after execution", action.TargetRef)
		}
		target, ok := number(action.Args["target_db"])
		if !ok {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s target_db missing", action.ID)
		}
		actual, ok := firstNumber(row, "volume_db", "fader_db", "gain_db", "db")
		if !ok || math.Abs(actual-target) > 0.001 {
			result.Structural = "fail"
			return result, fmt.Errorf("track %s fader postcondition failed: got %.3f want %.3f", action.TargetRef, actual, target)
		}
	}
	result.Structural = "pass"
	result.EvidenceRefs = []string{"vsp.state.snapshot:postcondition"}
	if v.Acoustic == nil {
		result.Acoustic = "unsupported"
		return result, nil
	}
	acoustic, err := v.Acoustic.VerifyStaticBalance(ctx, actionSet)
	result.Acoustic = strings.TrimSpace(acoustic.Status)
	result.EvidenceRefs = append(result.EvidenceRefs, acoustic.EvidenceRefs...)
	result.Summary = strings.TrimSpace(acoustic.Summary)
	result.SpecialistRelationship = strings.TrimSpace(acoustic.RelationshipStatus)
	result.SpecialistSummary = strings.TrimSpace(acoustic.RelationshipSummary)
	if err != nil {
		return result, err
	}
	if strings.EqualFold(result.Acoustic, "pass") {
		result.Status = "pass"
	} else if strings.EqualFold(result.Acoustic, "fail") {
		result.Status = "fail"
	} else {
		result.Status = "inconclusive"
	}
	return result, nil
}

func indexTracks(value any) map[string]map[string]any {
	var rows []map[string]any
	data, _ := json.Marshal(value)
	_ = json.Unmarshal(data, &rows)
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		id := firstString(row, "track_id", "id")
		if id != "" {
			out[id] = row
		}
	}
	return out
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		text := strings.TrimSpace(fmt.Sprint(row[key]))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func firstNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if parsed, ok := number(value); ok {
				return parsed, true
			}
		}
	}
	return 0, false
}

func number(value any) (float64, bool) {
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
