package staticbalance

import (
	"fmt"
	"math"
	"strings"
)

func CompileDecision(model Model, result Result, decision Decision) (CandidatePlan, error) {
	if result.ModelID == "" || result.ModelID != model.ModelID {
		return CandidatePlan{}, fmt.Errorf("static balance result/model mismatch")
	}
	if !result.Readiness.CanProceed || !model.Readiness.CanProceed {
		return CandidatePlan{}, fmt.Errorf("static balance readiness does not permit an executable pending plan")
	}
	if disposition := strings.ToLower(strings.TrimSpace(decision.Disposition)); disposition != "" && disposition != "select" {
		return CandidatePlan{}, fmt.Errorf("static balance decision disposition %q is not executable", disposition)
	}
	id := strings.TrimSpace(decision.CandidatePlanID)
	if id == "" {
		return CandidatePlan{}, fmt.Errorf("static balance decision is missing candidate_plan_id")
	}
	for _, candidate := range result.Candidates {
		if candidate.CandidatePlanID != id {
			continue
		}
		if err := ValidateCandidate(model, candidate); err != nil {
			return CandidatePlan{}, err
		}
		return candidate, nil
	}
	return CandidatePlan{}, fmt.Errorf("unknown static balance candidate_plan_id %q", id)
}

func ValidateCandidate(model Model, candidate CandidatePlan) error {
	if strings.TrimSpace(candidate.CandidatePlanID) == "" {
		return fmt.Errorf("static balance candidate id is empty")
	}
	if len(candidate.Actions) == 0 {
		return fmt.Errorf("static balance candidate has no actions")
	}
	tracks := map[string]Track{}
	for _, track := range model.Tracks {
		tracks[track.TrackID] = track
	}
	seen := map[string]bool{}
	for _, action := range candidate.Actions {
		if action.Operation != "track_gain_adjust" {
			return fmt.Errorf("static balance action %s has forbidden operation %q", action.TrackID, action.Operation)
		}
		if action.TrackID == "" || seen[action.TrackID] {
			return fmt.Errorf("static balance action has empty or duplicate track id %q", action.TrackID)
		}
		seen[action.TrackID] = true
		track, ok := tracks[action.TrackID]
		if !ok || !track.Eligible || track.FaderDB == nil || track.LevelDB == nil {
			return fmt.Errorf("static balance action targets ineligible or unknown track %q", action.TrackID)
		}
		if math.Abs(action.DeltaDB) < 0.25 || math.Abs(action.DeltaDB) > 2.000001 || math.Abs(action.DeltaDB) > candidate.MaxAbsDeltaDB+0.000001 {
			return fmt.Errorf("static balance action %s has unsafe delta %.3f dB", action.TrackID, action.DeltaDB)
		}
		if math.Abs(action.BeforeDB-*track.FaderDB) > 0.001 {
			return fmt.Errorf("static balance action %s has stale fader baseline", action.TrackID)
		}
		if math.Abs(action.TargetDB-(action.BeforeDB+action.DeltaDB)) > 0.001 {
			return fmt.Errorf("static balance action %s target does not match delta", action.TrackID)
		}
		if action.DeltaDB > 0 && track.HeadroomDB != nil && action.DeltaDB > math.Max(0, *track.HeadroomDB-1.0)+0.001 {
			return fmt.Errorf("static balance action %s exceeds available headroom", action.TrackID)
		}
	}
	return nil
}
