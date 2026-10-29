package experiment

import (
	"strings"
	"testing"
)

// M3-C4-AUTO-INTENT-1: the section_automation domain row's point-array
// validation authority and the readback settle skeleton.

func validEnvelopeTypedAction() map[string]any {
	return map[string]any{
		"action_domain": SectionAutomationActionDomain,
		"action_kind":   AutomationEnvelopeWriteKind,
		"plugin_id":     "plug_1",
		"param_id":      "volume",
		"points": []any{
			map[string]any{"t": 0.0, "v": 0.3, "c": 0.0},
			map[string]any{"t": 30.0, "v": 0.6, "c": 0.0},
		},
		"region": map[string]any{"start": 0.0, "end": 30.0},
	}
}

func envelopeAdmission(typedAction map[string]any) Admission {
	return Admission{
		SchemaVersion:        SchemaVersion,
		TargetRef:            map[string]any{"kind": "track", "id": "track_1"},
		EvidenceRefs:         []string{"automation_intent:test"},
		Hypothesis:           "test envelope",
		TypedAction:          typedAction,
		DiagnosticDoseBounds: map[string]any{"max_action_attempts": 1, "point_count": float64(2)},
		RetainedDoseBounds:   map[string]any{"max_action_attempts": 1, "point_count": float64(2)},
		ExperimentBudget:     1,
		ExpectedEffect:       "ride up",
		VerificationPlan:     map[string]any{"kind": "readback"},
		CheckpointRef:        "1",
		RollbackPlan:         map[string]any{"kind": "clear"},
		AuthorityMode:        AuthorityOrdinary,
	}
}

func TestSectionAutomationDomainRowAdmitsValidEnvelope(t *testing.T) {
	spec, ok := D1S1SpecForAction(SectionAutomationActionDomain, AutomationEnvelopeWriteKind)
	if !ok {
		t.Fatalf("section_automation row missing from the domain table")
	}
	if spec.CapabilityID != "fine_mix.section_automation.v1" {
		t.Errorf("capability id = %q", spec.CapabilityID)
	}
	if err := envelopeAdmission(validEnvelopeTypedAction()).ValidateD1S1(); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}
	if !envelopeAdmission(validEnvelopeTypedAction()).IsD1S1() {
		t.Errorf("section_automation admission must register as D1-S1")
	}
	if domains := D1S1AdmittedDomains(); !contains(domains, SectionAutomationActionDomain) {
		t.Errorf("admitted domain list missing section_automation: %v", domains)
	}
}

func TestSectionAutomationDomainRowRejectsStructuralViolations(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(map[string]any) map[string]any
		wantMessage string
	}{
		{"single point", func(a map[string]any) map[string]any {
			a["points"] = []any{map[string]any{"t": 0.0, "v": 0.3}}
			return a
		}, "2-16"},
		{"descending t", func(a map[string]any) map[string]any {
			a["points"] = []any{
				map[string]any{"t": 30.0, "v": 0.3},
				map[string]any{"t": 0.0, "v": 0.6},
			}
			return a
		}, "ascending"},
		{"curve out of range", func(a map[string]any) map[string]any {
			a["points"] = []any{
				map[string]any{"t": 0.0, "v": 0.3, "c": 1.5},
				map[string]any{"t": 30.0, "v": 0.6},
			}
			return a
		}, "[-1,1]"},
		{"negative t", func(a map[string]any) map[string]any {
			a["points"] = []any{
				map[string]any{"t": -1.0, "v": 0.3},
				map[string]any{"t": 30.0, "v": 0.6},
			}
			return a
		}, ">= 0"},
		{"point outside region", func(a map[string]any) map[string]any {
			a["points"] = []any{
				map[string]any{"t": 0.0, "v": 0.3},
				map[string]any{"t": 45.0, "v": 0.6},
			}
			return a
		}, "outside the region"},
		{"inverted region", func(a map[string]any) map[string]any {
			a["region"] = map[string]any{"start": 30.0, "end": 0.0}
			return a
		}, "0 <= start < end"},
		{"missing plugin id", func(a map[string]any) map[string]any {
			delete(a, "plugin_id")
			return a
		}, "plugin_id"},
		{"missing param id", func(a map[string]any) map[string]any {
			delete(a, "param_id")
			return a
		}, "param_id"},
	}
	for _, tc := range cases {
		typedAction := tc.mutate(validEnvelopeTypedAction())
		err := envelopeAdmission(typedAction).ValidateD1S1()
		if err == nil {
			t.Errorf("%s: envelope admitted, want rejection", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantMessage) {
			t.Errorf("%s: error %q missing %q", tc.name, err.Error(), tc.wantMessage)
		}
	}
}

func TestSectionAutomationMultiRoundFailsClosed(t *testing.T) {
	// The D2-2 cumulative-dose machine is dB-dimensioned; the envelope row
	// must fail closed on multi-round admission (single-round choreography).
	admission := envelopeAdmission(validEnvelopeTypedAction())
	admission.ExperimentBudget = 2
	if err := admission.ValidateD2MultiRound(); err == nil {
		t.Errorf("multi-round envelope admission must fail closed")
	}
}

func TestAutomationEnvelopeCurveMatches(t *testing.T) {
	written := []AutomationEnvelopePoint{
		{T: 0.0, V: 0.3, C: 0.0},
		{T: 30.0, V: 0.6, C: -0.25},
	}
	readback := []map[string]any{
		{"t": 0.0, "v": 0.3, "c": 0.0},
		{"t": 30.0, "v": 0.6, "c": -0.25},
	}
	if err := AutomationEnvelopeCurveMatches(written, readback); err != nil {
		t.Fatalf("identical curve rejected: %v", err)
	}
	// Tolerance-level float noise stays a match.
	noisy := []map[string]any{
		{"t": 0.0 + 5e-7, "v": 0.3 - 5e-7, "c": 0.0},
		{"t": 30.0, "v": 0.6, "c": -0.25 + 5e-7},
	}
	if err := AutomationEnvelopeCurveMatches(written, noisy); err != nil {
		t.Fatalf("tolerance-level noise rejected: %v", err)
	}
	// Count mismatch fails.
	if err := AutomationEnvelopeCurveMatches(written, readback[:1]); err == nil {
		t.Errorf("count mismatch accepted")
	}
	// Value drift beyond tolerance fails.
	drift := []map[string]any{
		{"t": 0.0, "v": 0.4, "c": 0.0},
		{"t": 30.0, "v": 0.6, "c": -0.25},
	}
	if err := AutomationEnvelopeCurveMatches(written, drift); err == nil {
		t.Errorf("value drift accepted")
	}
}
