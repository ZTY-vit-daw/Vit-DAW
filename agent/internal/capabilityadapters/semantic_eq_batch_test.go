package capabilityadapters

import (
	"testing"
	"time"

	"vit-daw-agent/internal/frequencycleanup"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/semanticeffect"
)

func TestFreezeSemanticEQBatchCreatesOneProjectAtomicAction(t *testing.T) {
	frequency, gain := 90.0, -1.0
	action := semanticeffect.Action{SchemaVersion: semanticeffect.ActionSchema, ActionType: semanticeffect.ActionEQEdit, PayloadSchema: semanticeffect.EQPlanSchema, Target: semanticeffect.Target{TrackID: "kick", PluginID: "eq-1"}, UserGoal: "make room for bass", Evidence: semanticeffect.EvidenceDecision{Choice: "derive", Basis: "observation", Reason: "B4 relationship", ObservationID: "obs-1"}, EQPlan: &semanticeffect.EQPlan{SchemaVersion: semanticeffect.EQPlanSchema, Atomic: true, Atoms: []semanticeffect.EQAtom{{AtomID: "a1", Action: "upsert", Shape: "bell", FrequencyHz: &frequency, GainDB: &gain, Purpose: "reduce overlap", Confidence: "medium", FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected"}}}}}
	treatment := lowendrelation.TreatmentPlan{SchemaVersion: lowendrelation.TreatmentPlanSchema, PlanID: "treatment-1", DiagnosisID: "diag-1", ObservationID: "obs-1", ObservationScope: "full_project", Summary: "separate low end", Targets: []lowendrelation.TreatmentTarget{{TargetID: "target-1", Order: 1, TrackID: "kick", RelationshipRefs: []string{"conflict-1"}, ListeningGoal: "make room"}}}
	leaf := SemanticEQPlan{Action: action, TopologyGeneration: "topology-1", Edits: []map[string]any{{"shape": "bell"}}, PlannedEdits: []map[string]any{{"atom_id": "a1"}}, PlannedWrites: []map[string]any{{"param_id": "gain", "normalized_value": 0.4}}, ParameterPreimage: []map[string]any{{"param_id": "gain", "normalized": 0.5}}}
	cut := orchestration.ProjectCut{ProjectUUID: "project", ProjectEpoch: "epoch", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	proposal, actionSet, err := FreezeSemanticEQBatch(SemanticEQBatchPlan{Treatment: treatment, Diagnosis: lowendrelation.Model{ModelID: "diag-1"}, Batch: semanticeffect.Batch{SchemaVersion: semanticeffect.BatchSchema, ProjectGoal: treatment.Summary, Atomic: true, Actions: []semanticeffect.Action{action}}, Leaves: []SemanticEQPlan{leaf}}, cut, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.CapabilityID != lowendrelation.CapabilityID || len(actionSet.Actions) != 1 || actionSet.Actions[0].Command != "b4.semantic_eq_batch.governed" || !actionSet.Actions[0].Compensatable {
		t.Fatalf("unexpected frozen batch: proposal=%#v actionSet=%#v", proposal, actionSet)
	}
	leaves, ok := actionSet.Actions[0].Args["leaves"].([]map[string]any)
	if !ok || len(leaves) != 1 || leaves[0]["track_id"] != "kick" || leaves[0]["plugin_id"] != "eq-1" {
		t.Fatalf("exact leaf identity was not frozen: %#v", actionSet.Actions[0].Args["leaves"])
	}
}

func TestFreezeProjectSemanticEQBatchUsesC1IdentityWithoutChangingLeafExecutor(t *testing.T) {
	frequency, gain := 420.0, -1.5
	action := semanticeffect.Action{SchemaVersion: semanticeffect.ActionSchema, ActionType: semanticeffect.ActionEQEdit, PayloadSchema: semanticeffect.EQPlanSchema, Target: semanticeffect.Target{TrackID: "guitar", PluginID: "eq-c1"}, UserGoal: "reduce stable low-mid buildup", Evidence: semanticeffect.EvidenceDecision{Choice: "derive", Basis: "observation", Reason: "C1 static evidence", ObservationID: "obs-c1"}, EQPlan: &semanticeffect.EQPlan{SchemaVersion: semanticeffect.EQPlanSchema, Atomic: true, Atoms: []semanticeffect.EQAtom{{AtomID: "a1", Action: "upsert", Shape: "bell", FrequencyHz: &frequency, GainDB: &gain, Purpose: "reduce stable buildup", Confidence: "medium", FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected"}}}}}
	leaf := SemanticEQPlan{Action: action, TopologyGeneration: "topology-c1", Edits: []map[string]any{{"shape": "bell"}}, PlannedEdits: []map[string]any{{"atom_id": "a1"}}, PlannedWrites: []map[string]any{{"param_id": "gain", "normalized_value": .4}}, ParameterPreimage: []map[string]any{{"param_id": "gain", "normalized": .5}}}
	cut := orchestration.ProjectCut{ProjectUUID: "project", ProjectEpoch: "epoch", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	baseline := frequencycleanup.BuildTargetPostFXBaseline("c1", "preflight", "project", []string{"guitar"}, []map[string]any{{"status": "ready", "track_id": "guitar", "tap_point": "track_post_fader", "render_revision": "render-1", "track_state_fingerprint": "state-1", "evidence_ref": "probe:guitar:1", "bands": map[string]any{"low_mid": map[string]any{"unit_energy": .3}}}}, time.Unix(1, 0))
	proposal, actionSet, err := FreezeProjectSemanticEQBatch(ProjectSemanticEQBatchSpec{CapabilityID: "fine_mix.frequency_cleanup.v1", CapabilityVersion: "v1", PlanID: "c1-plan", Command: "c1.semantic_eq_batch.governed", VerificationRef: "frequency_cleanup.verification.v1", Treatment: map[string]any{"plan_id": "c1-plan"}, Diagnosis: map[string]any{"model_id": "c1-model"}, VerificationContext: baseline, Batch: semanticeffect.Batch{SchemaVersion: semanticeffect.BatchSchema, ProjectGoal: "cleanup", Atomic: true, Actions: []semanticeffect.Action{action}}, Leaves: []SemanticEQPlan{leaf}, ExpectedTargetCount: 1}, cut, 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.CapabilityID != "fine_mix.frequency_cleanup.v1" || actionSet.CapabilityID != proposal.CapabilityID || actionSet.Actions[0].Command != "c1.semantic_eq_batch.governed" {
		t.Fatalf("C1 identities were not frozen: proposal=%+v actionSet=%+v", proposal, actionSet)
	}
	if actionSet.Actions[0].Args["leaves"] == nil || actionSet.Actions[0].Command == "plugin_grabber.apply_eq_edits.governed" {
		t.Fatalf("project batch did not preserve shared leaf handoff: %+v", actionSet.Actions[0])
	}
	verification, ok := actionSet.Actions[0].Args["verification_context"].(frequencycleanup.TargetPostFXBaseline)
	if !ok || verification.BaselineID != baseline.BaselineID || len(verification.RequestedTrackIDs) != 1 || verification.RequestedTrackIDs[0] != "guitar" {
		t.Fatalf("frozen C1 action lost target baseline: %#v", actionSet.Actions[0].Args["verification_context"])
	}
}
