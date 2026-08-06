package semanticeffect

import "testing"

func validCompressorPlan() CompressorPlan {
	value := -18.0
	return CompressorPlan{SchemaVersion: CompressorPlanSchema, PlanningOnly: true, MutationAuthorized: false,
		Target: Target{TrackID: "track-1", PluginID: "plugin-1"}, UserGoal: "压得更稳但保留瞬态",
		IdentityCardID: "apsic1_test", TopologyGeneration: "c2t1_test", SelectedAxes: []string{"activation_intensity", "transient_timing"},
		Evidence: CompressorPlanEvidence{COMMode: "paired_io", COMStatus: "partial", Summary: "bounded evidence"},
		Controls: []CompressorControlProposal{
			{ProposalID: "activation", Axis: "activation_intensity", PathKey: "main", Role: "threshold", Target: CompressorControlTarget{ValueDB: &value}, Purpose: "increase activation", Confidence: "medium"},
		},
		EvaluationContract: CompressorEvaluationContract{SchemaVersion: "semantic_effect.compressor_evaluation_contract.v1", FrozenUserGoal: "压得更稳但保留瞬态",
			Axes: []CompressorAxisEvaluation{
				{Axis: "activation_intensity", DesiredDirection: "more bounded gain action", COMDimensions: []string{"gain_action"}, AcceptanceCondition: "gain action increases without excessive duty cycle"},
				{Axis: "transient_timing", DesiredDirection: "transients remain preserved", COMDimensions: []string{"transient_response"}, AcceptanceCondition: "transient response does not become more rounded"},
			}, LevelMatchRequired: true, SuccessPolicy: "bounded_com_change_delta_plus_user_acceptance", NoAutoIteration: true},
		Revision: CompressorRevision{SchemaVersion: CompressorRevisionSchema, Attempt: 0, MaxAttempts: 1}}
}

func TestCompressorPlanRequiresPlanningOnlyAndFrozenEvaluation(t *testing.T) {
	plan := validCompressorPlan()
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid plan: %v", err)
	}
	plan.MutationAuthorized = true
	if err := plan.Validate(); err == nil {
		t.Fatal("mutation authority escaped COM-6 plan")
	}
}

func TestCompressorPlanForbidsOutputAndMixAsCompressionIntensity(t *testing.T) {
	for _, role := range []string{"output_gain", "mix"} {
		plan := validCompressorPlan()
		plan.Controls[0].Role = role
		if err := plan.Validate(); err == nil {
			t.Fatalf("role %s accepted as activation intensity", role)
		}
	}
}

func TestCompressorPlanRequiresOneBoundRevision(t *testing.T) {
	plan := validCompressorPlan()
	plan.Revision = CompressorRevision{SchemaVersion: CompressorRevisionSchema, Attempt: 1, MaxAttempts: 1, RejectionID: "cmr1_test"}
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid constrained revision: %v", err)
	}
	plan.Revision.Attempt = 2
	if err := plan.Validate(); err == nil {
		t.Fatal("second revision was accepted")
	}
}
