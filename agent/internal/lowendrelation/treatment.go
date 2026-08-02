package lowendrelation

import (
	"fmt"
	"sort"
	"strings"
)

// FinalizeTreatmentPlan binds an LLM-authored specialist decision back to the
// complete deterministic B4 model. It fills deterministic identities and
// rejects invented tracks, relationships, duplicate targets, and incomplete
// ordering before plug-in discovery can begin.
func FinalizeTreatmentPlan(plan TreatmentPlan, model Model) (TreatmentPlan, error) {
	if strings.TrimSpace(plan.SchemaVersion) != TreatmentPlanSchema {
		return TreatmentPlan{}, fmt.Errorf("schema_version must be %s", TreatmentPlanSchema)
	}
	if !model.Readiness.CanProceed {
		return TreatmentPlan{}, fmt.Errorf("B4 diagnosis is not ready for treatment")
	}
	if strings.TrimSpace(plan.Summary) == "" {
		return TreatmentPlan{}, fmt.Errorf("treatment summary is required")
	}

	trackByID := make(map[string]LowEndTrack, len(model.Tracks))
	for _, track := range model.Tracks {
		trackByID[track.TrackID] = track
	}
	validRelationship := map[string]bool{}
	for _, conflict := range model.Conflicts {
		validRelationship[conflict.ID] = conflict.ID != ""
	}
	for _, observation := range model.Observations {
		validRelationship[observation.ID] = observation.ID != ""
	}

	seenTrack := map[string]bool{}
	seenOrder := map[int]bool{}
	for index := range plan.Targets {
		target := &plan.Targets[index]
		target.TrackID = strings.TrimSpace(target.TrackID)
		track, ok := trackByID[target.TrackID]
		if !ok {
			return TreatmentPlan{}, fmt.Errorf("target %d uses unknown B4 track_id %q", index+1, target.TrackID)
		}
		if seenTrack[target.TrackID] {
			return TreatmentPlan{}, fmt.Errorf("target %d duplicates track_id %q", index+1, target.TrackID)
		}
		seenTrack[target.TrackID] = true
		if target.Order <= 0 || seenOrder[target.Order] {
			return TreatmentPlan{}, fmt.Errorf("target %d requires a unique positive order", index+1)
		}
		seenOrder[target.Order] = true
		if strings.TrimSpace(target.ListeningGoal) == "" {
			return TreatmentPlan{}, fmt.Errorf("target %d listening_goal is required", index+1)
		}
		target.RelationshipRefs = unique(target.RelationshipRefs)
		if len(target.RelationshipRefs) == 0 {
			return TreatmentPlan{}, fmt.Errorf("target %d relationship_refs are required", index+1)
		}
		for _, ref := range target.RelationshipRefs {
			if !validRelationship[ref] {
				return TreatmentPlan{}, fmt.Errorf("target %d invented relationship_ref %q", index+1, ref)
			}
		}
		target.TrackName = firstNonEmptyString(target.TrackName, track.TrackName, target.TrackID)
		target.Constraints = unique(target.Constraints)
		target.EvidenceRefs = unique(append(target.EvidenceRefs, model.EvidenceRefs...))
		target.TargetID = stableID("b4_target", map[string]any{
			"diagnosis_id":  model.ModelID,
			"order":         target.Order,
			"track_id":      target.TrackID,
			"relationships": target.RelationshipRefs,
		})
	}
	for order := 1; order <= len(plan.Targets); order++ {
		if !seenOrder[order] {
			return TreatmentPlan{}, fmt.Errorf("treatment target order must be contiguous from 1")
		}
	}
	sort.SliceStable(plan.Targets, func(i, j int) bool { return plan.Targets[i].Order < plan.Targets[j].Order })
	plan.DiagnosisID = model.ModelID
	plan.ObservationID = model.ObservationID
	plan.ObservationScope = "full_project"
	plan.GlobalConstraints = unique(plan.GlobalConstraints)
	plan.EvidenceRefs = unique(append(plan.EvidenceRefs, model.EvidenceRefs...))
	plan.Limitations = unique(plan.Limitations)
	plan.PlanID = stableID("b4_treatment", map[string]any{
		"diagnosis_id": model.ModelID,
		"targets":      plan.Targets,
		"constraints":  plan.GlobalConstraints,
	})
	return plan, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
