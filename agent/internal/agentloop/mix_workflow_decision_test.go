package agentloop

import "testing"

func TestMixDecisionGateObservationOnly(t *testing.T) {
	decision := MixWorkflowDecisionGate(
		"read-only observation: inspect the current mix spectrum and stereo status; do not modify.",
		map[string]any{"observation_id": "obs_1", "status": "partial"},
		"",
	)
	if decision.SchemaVersion != mixWorkflowDecisionSchemaVersion || decision.Intent != "observation_only" {
		t.Fatalf("decision = %+v", decision)
	}
	if decision.ObservationID != "obs_1" {
		t.Fatalf("observation id missing: %+v", decision)
	}
}

func TestMixDecisionGateExplicitActionCreatesPending(t *testing.T) {
	decision := MixWorkflowDecisionGate(
		"Move Track 1 pan left a little.",
		map[string]any{"observation_id": "obs_pan"},
		"pan_balance",
	)
	if decision.Intent != "propose_pending" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestObservationOnlyTreatmentMarkerIsNotPending(t *testing.T) {
	state := &runState{input: Input{UserText: "observe the current spectrum and stereo status"}}
	reply := `Observation only.
mix_treatment_pending: {"schema_version":"mix_treatment_pending.v0","status":"pending_confirmation","intent":"observe","target_ref":"project","action_kind":"observation_only","processor_type":"unknown"}`
	if pending := messageLoopMixTreatmentPendingFromReply(state, reply); pending != nil {
		t.Fatalf("observation_only marker became pending: %+v", pending)
	}
}

func TestTreatmentPendingBlockedWhenMOMActionPreflightUntrusted(t *testing.T) {
	state := &runState{
		input: Input{UserText: "lower the low end a little, but show evidence first"},
		recentObservation: &RecentObservation{
			Tool:   "mix.observe",
			Status: "ok",
			Summary: map[string]any{
				"observation_id": "obs_untrusted",
				"mom_projection": map[string]any{
					"mom_version": "v1.4",
					"intent":      "action_preflight_observation",
					"trust_quality": map[string]any{
						"schema_version":               "mom_trust_quality.v1",
						"overall_status":               "partial",
						"can_support_observation":      true,
						"can_support_suggestion":       false,
						"can_support_action_preflight": false,
						"suspect_fields":               []any{"timbre_frequency"},
						"blocked_reasons":              []any{"action_preflight_requires_ready_band_stereo_evidence"},
					},
				},
			},
		},
	}
	reply := `I would prepare a small EQ move after confirmation.
mix_treatment_pending: {"schema_version":"mix_treatment_pending.v0","status":"pending_confirmation","intent":"reduce low end","target_ref":"track:1007","action_kind":"plugin_treatment","processor_type":"eq","evidence_refs":["observation:obs_untrusted"],"needs_resolution":["plugin_instance"],"expires_after_context_change":true}`
	if pending := messageLoopMixTreatmentPendingFromReply(state, reply); pending != nil {
		t.Fatalf("untrusted MOM evidence became pending: %+v", pending)
	}
}
