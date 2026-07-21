package panlayout

import (
	"fmt"
	"math"
	"strings"
)

func CompileDecision(model Model, result Result, decision Decision) (CandidatePlan, error) {
	if result.ModelID == "" || result.ModelID != model.ModelID {
		return CandidatePlan{}, fmt.Errorf("pan layout result/model mismatch")
	}
	if !result.Readiness.CanProceed || !model.Readiness.CanProceed {
		return CandidatePlan{}, fmt.Errorf("pan layout readiness does not permit an executable pending plan")
	}
	if d := strings.ToLower(strings.TrimSpace(decision.Disposition)); d != "" && d != "select" {
		return CandidatePlan{}, fmt.Errorf("pan layout decision disposition %q is not executable", d)
	}
	id := strings.TrimSpace(decision.CandidatePlanID)
	if id == "" {
		return CandidatePlan{}, fmt.Errorf("pan layout decision is missing candidate_plan_id")
	}
	for _, candidate := range result.Candidates {
		if candidate.CandidatePlanID == id {
			if err := ValidateCandidate(model, candidate); err != nil {
				return CandidatePlan{}, err
			}
			return candidate, nil
		}
	}
	return CandidatePlan{}, fmt.Errorf("unknown pan layout candidate_plan_id %q", id)
}

func ValidateCandidate(model Model, candidate CandidatePlan) error {
	if candidate.CandidatePlanID == "" || len(candidate.Actions) == 0 {
		return fmt.Errorf("pan layout candidate id/actions are required")
	}
	tracks := map[string]Track{}
	for _, t := range model.Tracks {
		tracks[t.TrackID] = t
	}
	seen := map[string]bool{}
	for _, a := range candidate.Actions {
		if a.Operation != "track_pan_set" {
			return fmt.Errorf("pan layout action %s has forbidden operation %q", a.TrackID, a.Operation)
		}
		if a.TrackID == "" || seen[a.TrackID] {
			return fmt.Errorf("pan layout action has empty or duplicate track id %q", a.TrackID)
		}
		seen[a.TrackID] = true
		t, ok := tracks[a.TrackID]
		if !ok || !t.Eligible || t.CurrentPan == nil {
			return fmt.Errorf("pan layout action targets ineligible or unknown track %q", a.TrackID)
		}
		if math.Abs(a.BeforePan-*t.CurrentPan) > .001 {
			return fmt.Errorf("pan layout action %s has stale pan baseline", a.TrackID)
		}
		if a.TargetPan < -1 || a.TargetPan > 1 || math.Abs(a.TargetPan) > candidate.MaxAbsPan+.001 {
			return fmt.Errorf("pan layout action %s has unsafe target %.3f", a.TrackID, a.TargetPan)
		}
		if math.Abs(a.DeltaPan-(a.TargetPan-a.BeforePan)) > .011 || math.Abs(a.DeltaPan) < .049 {
			return fmt.Errorf("pan layout action %s has invalid delta %.3f", a.TrackID, a.DeltaPan)
		}
		if contains(t.RiskFlags, "low_stereo_correlation") {
			return fmt.Errorf("pan layout action %s targets low-correlation stereo source", a.TrackID)
		}
	}
	return nil
}
