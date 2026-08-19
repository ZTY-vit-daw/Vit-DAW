package orchestration

// DefaultRegistry contains only declarations. The adapters remain owned by
// their domain packages and are intentionally not imported here.
func DefaultRegistry() *Registry {
	r, _ := NewRegistry(
		CapabilityDefinition{
			ID:              "static_mix.static_balance.v0",
			Version:         "v0",
			Family:          "parametric_transform",
			Description:     "Derive bounded static fader-balance candidates from project and observation projections.",
			Effects:         []string{"project_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "static_mix.static_balance.verification.v0",
		},
		CapabilityDefinition{
			ID:              "static_mix.pan_layout.v0",
			Version:         "v0",
			Family:          "parametric_transform",
			Description:     "Derive bounded pan-layout candidates from project and observation projections.",
			Effects:         []string{"project_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "static_mix.pan_layout.verification.v0",
		},
		CapabilityDefinition{
			ID:              "static_mix.low_end_relation.v0",
			Version:         "v0",
			Family:          "project_specialist_generic_eq_orchestration",
			Description:     "Diagnose full-project low-end relationships and orchestrate separately confirmed generic-EQ loading and atomic parameter treatment through the ordinary Agent EQ runtime.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "static_mix.low_end_relation.eq_batch.verification.v1",
		},
		CapabilityDefinition{
			ID:              "fine_mix.frequency_cleanup.v1",
			Version:         "v1",
			Family:          "project_specialist_generic_eq_orchestration",
			Description:     "Diagnose whole-project frequency relationships, classify every track, and apply only confirmed static-EQ treatments through the shared ordinary-Agent EQ runtime.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "frequency_cleanup.verification.v1",
		},
		CapabilityDefinition{
			ID:              "fine_mix.dynamic_control.v1",
			Version:         "v1",
			Family:          "project_specialist_dynamic_control_orchestration",
			Description:     "Independently orchestrate dynamic-control treatment through existing PCA-governed semantic processor workflows and typed executors.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "fine_mix.dynamic_control.verification.v1",
		},
		CapabilityDefinition{
			ID:              "agent.effect.eq_control.v0",
			Version:         "v0",
			Family:          "ordinary_agent_semantic_effect",
			Description:     "Apply an ordinary-Agent concrete generic static-EQ plan through frozen topology, preimage, confirmation, readback, rollback, and verification.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "agent.effect.eq_control.verification.v0",
		},
	)
	return r
}
