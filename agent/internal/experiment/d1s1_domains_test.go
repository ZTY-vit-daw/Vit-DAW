package experiment

import (
	"strings"
	"testing"
)

func testD1StaticEQAdmission(mode AuthorityMode) Admission {
	a := testAdmission(mode)
	a.TargetRef = map[string]any{"kind": "track", "id": "track-1", "source": "fresh_g1_g7_observation"}
	a.TypedAction = map[string]any{
		"action_domain": "static_eq",
		"action_kind":   "static_eq_band_adjust",
		"frequency_hz":  220.0,
		"gain_db":       -1.5,
		"q":             1.0,
		"band_index":    2,
	}
	a.DiagnosticDoseBounds = map[string]any{"gain_db": -1.5, "max_action_attempts": 1}
	a.RetainedDoseBounds = map[string]any{"gain_db": -1.5, "max_action_attempts": 1}
	a.ExperimentBudget = 1
	return a
}

func TestD1S1AdmitsBoundedStaticEQBandAdjustment(t *testing.T) {
	a := testD1StaticEQAdmission(AuthorityFull)
	if err := a.ValidateD1S1(); err != nil {
		t.Fatalf("valid static_eq admission rejected: %v", err)
	}
	if !a.IsD1S1() {
		t.Fatal("static_eq admission must belong to the D1-S1 governed family")
	}
}

func TestD1S1StaticEQBoundsRejectOutOfBandParameters(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Admission)
	}{
		{"gain at bound edge", func(a *Admission) {
			a.DiagnosticDoseBounds["gain_db"] = 2.5
			a.RetainedDoseBounds["gain_db"] = 2.5
			a.TypedAction["gain_db"] = 2.5
		}},
		{"zero gain", func(a *Admission) {
			a.DiagnosticDoseBounds["gain_db"] = 0
			a.RetainedDoseBounds["gain_db"] = 0
		}},
		{"frequency below audible", func(a *Admission) { a.TypedAction["frequency_hz"] = 15.0 }},
		{"frequency above audible", func(a *Admission) { a.TypedAction["frequency_hz"] = 25000.0 }},
		{"resonant q", func(a *Admission) { a.TypedAction["q"] = 24.0 }},
		{"flat q", func(a *Admission) { a.TypedAction["q"] = 0.05 }},
		{"negative band index", func(a *Admission) { a.TypedAction["band_index"] = -1 }},
		{"fractional band index", func(a *Admission) { a.TypedAction["band_index"] = 1.5 }},
		{"missing frequency", func(a *Admission) { delete(a.TypedAction, "frequency_hz") }},
		{"hybrid domain borrows no bounds", func(a *Admission) { a.TypedAction["action_kind"] = "track_gain_adjust" }},
		{"attempts above one", func(a *Admission) { a.RetainedDoseBounds["max_action_attempts"] = 2 }},
		{"diagnostic gain unbounded", func(a *Admission) { a.DiagnosticDoseBounds["gain_db"] = -6.0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := testD1StaticEQAdmission(AuthorityOrdinary)
			test.mutate(&a)
			if err := a.ValidateD1S1(); err == nil {
				t.Fatal("out-of-band static_eq admission accepted")
			}
		})
	}
}

func TestD1S1UnknownDomainErrorListsAdmittedDomains(t *testing.T) {
	a := testD1Admission(AuthorityFull)
	a.TypedAction["action_domain"] = "spectral_repair"
	a.TypedAction["action_kind"] = "spectral_repair_heal"
	err := a.ValidateD1S1()
	if err == nil {
		t.Fatal("unknown domain accepted")
	}
	for _, domain := range D1S1AdmittedDomains() {
		if !strings.Contains(err.Error(), domain) {
			t.Fatalf("unknown-domain error must list admitted domain %q: %v", domain, err)
		}
	}
}
