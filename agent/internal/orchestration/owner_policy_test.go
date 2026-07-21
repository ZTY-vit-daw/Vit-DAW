package orchestration

import "testing"

func TestOwnerPolicyPinsExistingSessionAndDrainsLegacy(t *testing.T) {
	policy := ParseOwnerPolicy("b2,b3")
	if decision := policy.Select("static_mix.static_balance.v0", false, true, nil); decision.Owner != EngineLegacy || decision.Source != OwnerLegacyDrain {
		t.Fatalf("legacy pending was not drained by legacy owner: %#v", decision)
	}
	existing, _ := NewSession("s1", "p1", "goal", EngineV1, CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if decision := policy.Select(existing.Invocation.CapabilityID, false, false, &existing); decision.Owner != EngineV1 || decision.Source != OwnerExistingSession {
		t.Fatalf("existing owner was not pinned: %#v", decision)
	}
	if decision := policy.Select(existing.Invocation.CapabilityID, true, true, &existing); !decision.Conflict || decision.Source != OwnerConflict {
		t.Fatalf("dual authority conflict was not quarantined: %#v", decision)
	}
}

func TestOwnerPolicyRolloutAndExplicitCanaryOnlyAffectNewSessions(t *testing.T) {
	policy := ParseOwnerPolicy("b3")
	if decision := policy.Select("static_mix.static_balance.v0", false, false, nil); decision.Owner != EngineLegacy {
		t.Fatalf("B2 unexpectedly rolled out: %#v", decision)
	}
	if decision := policy.Select("static_mix.pan_layout.v0", false, false, nil); decision.Owner != EngineV1 || decision.Source != OwnerRolloutPolicy {
		t.Fatalf("B3 rollout missing: %#v", decision)
	}
	if decision := policy.Select("static_mix.static_balance.v0", true, false, nil); decision.Owner != EngineV1 || decision.Source != OwnerExplicitCanary {
		t.Fatalf("explicit canary missing: %#v", decision)
	}
}
