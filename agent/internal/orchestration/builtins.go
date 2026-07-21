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
			Family:          "semantic_parameter_application",
			Description:     "Apply a bounded low-end semantic parameter instruction through SPAL and a verified Provider instance.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "spal.spectral.static_bell.verification.v0",
		},
		CapabilityDefinition{
			// Registration is a separate confirmed control-plane action. It
			// persists an already observed and conformed instance; it never
			// loads a plug-in or acts as fallback resolution for an EQ request.
			ID:              "spal.reference_eq_provider_registration.v0",
			Version:         "v0",
			Family:          "semantic_parameter_application",
			Description:     "Register one explicitly conformed Reference EQ Provider instance for the current project.",
			Effects:         []string{"provider_registry_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "spal.reference_eq_provider_registration.verification.v0",
		},
		CapabilityDefinition{
			// This is deliberately a product-path fixture rather than B4.  It
			// proves the governed semantic-parameter lifecycle without teaching
			// the chat layer any low-end diagnosis or vendor parameter names.
			ID:              "spal.reference_eq_test.v0",
			Version:         "v0",
			Family:          "semantic_parameter_application",
			Description:     "Run one explicitly specified Reference EQ semantic-control test through a verified SPAL Provider instance.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "spal.spectral.static_bell.verification.v0",
		},
		CapabilityDefinition{
			ID:              "plugin.effect_control.v0",
			Version:         "v0",
			Family:          "semantic_parameter_application",
			Description:     "Apply one verified plug-in semantic control through the governed grabber path with frozen preimage, rollback, and evidence verification.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "plugin.effect_control.verification.v0",
		},
		CapabilityDefinition{
			ID:              "spal.eq.v2",
			Version:         "v2",
			Family:          "semantic_parameter_application",
			Description:     "Apply an explicit generic EQ v2 instruction through a capability-matrix-qualified VPS Provider.",
			Effects:         []string{"project_mutation", "plugin_parameter_mutation"},
			RiskCeiling:     "bounded_reversible",
			VerificationRef: "spal.eq.v2.structural.verification.v2",
		},
	)
	return r
}
