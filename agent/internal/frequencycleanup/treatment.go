package frequencycleanup

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/mom"
)

func FinalizeTreatmentPlan(plan TreatmentPlan, model Model) (TreatmentPlan, error) {
	if model.SchemaVersion != ModelSchemaVersion || model.ModelID == "" || !model.Readiness.Diagnosis.CanProceed {
		return TreatmentPlan{}, fmt.Errorf("ready C1 diagnosis model is required")
	}
	if plan.SchemaVersion != TreatmentPlanSchema {
		return TreatmentPlan{}, fmt.Errorf("schema_version must be %s", TreatmentPlanSchema)
	}
	if strings.TrimSpace(plan.Summary) == "" {
		return TreatmentPlan{}, fmt.Errorf("project treatment summary is required")
	}
	if len(plan.Items) != len(model.Tracks) {
		return TreatmentPlan{}, fmt.Errorf("one explicit treatment classification is required for each of %d project tracks", len(model.Tracks))
	}
	tracks := map[string]TrackProfile{}
	for _, track := range model.Tracks {
		tracks[track.TrackID] = track
	}
	allowedRefs := map[string]bool{model.ObservationID: true}
	for _, candidate := range model.Candidates {
		allowedRefs[candidate.ID] = true
	}
	allowedEvidence := map[string]bool{}
	for _, ref := range model.EvidenceRefs {
		allowedEvidence[ref] = true
	}
	seenTracks, seenOrders := map[string]bool{}, map[int]bool{}
	for index := range plan.Items {
		item := &plan.Items[index]
		track, ok := tracks[strings.TrimSpace(item.TrackID)]
		if !ok || seenTracks[item.TrackID] {
			return TreatmentPlan{}, fmt.Errorf("item %d has unknown or duplicate track_id %q", index+1, item.TrackID)
		}
		if item.Order < 1 || item.Order > len(plan.Items) || seenOrders[item.Order] {
			return TreatmentPlan{}, fmt.Errorf("item %d has invalid or duplicate order", index+1)
		}
		if !validClassification(item.Classification) {
			return TreatmentPlan{}, fmt.Errorf("item %d has unsupported classification %q", index+1, item.Classification)
		}
		if strings.TrimSpace(item.Rationale) == "" {
			return TreatmentPlan{}, fmt.Errorf("item %d requires rationale", index+1)
		}
		if item.Classification == TreatmentStaticEQ && (strings.TrimSpace(item.ListeningGoal) == "" || len(item.DiagnosisRefs) == 0) {
			return TreatmentPlan{}, fmt.Errorf("static_eq item %d requires listening_goal and exact diagnosis_refs", index+1)
		}
		if track.SilenceConfirmed && item.Classification != TreatmentNoChange {
			return TreatmentPlan{}, fmt.Errorf("deterministically silent track %q must be classified no_change", item.TrackID)
		}
		for _, ref := range item.DiagnosisRefs {
			if !allowedRefs[ref] {
				return TreatmentPlan{}, fmt.Errorf("item %d invented diagnosis_ref %q", index+1, ref)
			}
		}
		for _, ref := range item.EvidenceRefs {
			if !allowedEvidence[ref] {
				return TreatmentPlan{}, fmt.Errorf("item %d invented evidence_ref %q", index+1, ref)
			}
		}
		seenTracks[item.TrackID], seenOrders[item.Order] = true, true
		item.TrackName = track.TrackName
		item.ItemID = stableID("fci", map[string]any{"diagnosis": model.ModelID, "track": item.TrackID, "classification": item.Classification, "order": item.Order})
		item.DiagnosisRefs, item.Constraints, item.EvidenceRefs = unique(item.DiagnosisRefs), unique(item.Constraints), unique(item.EvidenceRefs)
	}
	for _, ref := range plan.EvidenceRefs {
		if !allowedEvidence[ref] {
			return TreatmentPlan{}, fmt.Errorf("plan invented evidence_ref %q", ref)
		}
	}
	sort.SliceStable(plan.Items, func(i, j int) bool { return plan.Items[i].Order < plan.Items[j].Order })
	plan.DiagnosisID, plan.ObservationID, plan.ObservationScope = model.ModelID, model.ObservationID, "full_project"
	plan.GlobalConstraints = unique(append(plan.GlobalConstraints,
		"static_eq_only_for_static_eq_items", "deferred_and_no_change_items_must_not_create_actions",
		"no_dynamic_eq_compression_space_or_automation_parameters", "project_wide_all_or_rollback"))
	plan.EvidenceRefs = unique(append(plan.EvidenceRefs, model.EvidenceRefs...))
	plan.Limitations = unique(append(plan.Limitations, model.Limitations...))
	plan.PlanID = stableID("fcp", map[string]any{"diagnosis": model.ModelID, "items": plan.Items, "summary": plan.Summary})
	return plan, nil
}

func StaticEQItems(plan TreatmentPlan) []TreatmentItem {
	out := []TreatmentItem{}
	for _, item := range plan.Items {
		if item.Classification == TreatmentStaticEQ {
			out = append(out, item)
		}
	}
	return out
}

func validClassification(value string) bool {
	switch value {
	case TreatmentStaticEQ, TreatmentDeferredDynamic, TreatmentDeferredSpace, TreatmentDeferredAutomation, TreatmentArrangementSource, TreatmentNoChange:
		return true
	default:
		return false
	}
}

// VerifyRelationships proves observation comparability and records observable
// relationship changes. It deliberately does not claim audible improvement or
// user acceptance.
func VerifyRelationships(before, after mom.FrequencyRelationship, beforeObservationID, afterObservationID string) Verification {
	comparable, reasons := mom.FrequencyRelationshipsComparable(before, after)
	postFX := boolValue(before.Coverage["supports_post_fx_compare"]) && boolValue(after.Coverage["supports_post_fx_compare"])
	if !postFX {
		reasons = append(reasons, "post_fx_compare_not_supported")
	}
	changed := []string{}
	if comparable && postFX {
		for _, dimension := range []struct {
			name          string
			before, after any
		}{
			{"frequency_region_energy_relationship", before.FrequencyRegions, after.FrequencyRegions},
			{"conflict_candidate_membership", conflictMembership(before.ConflictCandidates), conflictMembership(after.ConflictCandidates)},
			{"relative_tonal_shape", before.TonalTendencies, after.TonalTendencies},
			{"time_frequency_persistence_when_available", before.PersistenceSummary, after.PersistenceSummary},
		} {
			if stableID("v", dimension.before) != stableID("v", dimension.after) {
				changed = append(changed, dimension.name)
			}
		}
	}
	status := "inconclusive"
	if comparable && postFX {
		status = "observed"
	}
	refs := unique(append(append([]string(nil), before.EvidenceRefs...), after.EvidenceRefs...))
	return Verification{SchemaVersion: VerificationSchema, Status: status, Comparable: comparable, PostFXComparable: postFX,
		BeforeObservation: beforeObservationID, AfterObservation: afterObservationID, TapPoint: before.TapPoint,
		TrackIDs: stringsValue(before.Scope["track_ids"]), ChangedDimensions: changed, Reasons: unique(reasons), EvidenceRefs: refs, UserAcceptance: "unknown"}
}

func conflictMembership(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		ids := stringsValue(row["track_ids"])
		if len(ids) == 0 {
			for _, track := range rowsValue(row["tracks"]) {
				ids = append(ids, text(track["track_id"]))
			}
		}
		out = append(out, map[string]any{"region": firstNonEmpty(text(row["region"]), text(row["band"])), "track_ids": unique(ids)})
	}
	sort.SliceStable(out, func(i, j int) bool { return stableID("m", out[i]) < stableID("m", out[j]) })
	return out
}
