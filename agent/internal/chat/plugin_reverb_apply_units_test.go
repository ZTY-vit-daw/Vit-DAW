package chat

import "testing"

// FIX-TIMEFX-AXISFILTER-1 third leg RED: the domain-row decay dose writes the
// measured Pro-R decay surface in percent (M2-TIMEFX-1 decay_pct redesign),
// so percent must be a first-class decay/size unit. The pre-fix table only
// accepted seconds there and the typed execution failed with
// unit_role_mismatch after the ticket was confirmed (run 20261013_103532).
func TestReverbUnitCompatibleAcceptsPercentDecayAndSize(t *testing.T) {
	accepted := [][3]string{
		{"decay", "%", "%"},
		{"decay", "%", ""},
		{"size", "%", "%"},
		{"mix", "%", "%"},
		{"wet", "%", ""},
		{"decay", "s", "s"},
		{"pre_delay", "ms", "ms"},
	}
	for _, c := range accepted {
		if !reverbUnitCompatible(c[0], c[1], c[2]) {
			t.Fatalf("reverbUnitCompatible(%q, %q, %q) must accept the measured-surface unit", c[0], c[1], c[2])
		}
	}
	rejected := [][3]string{
		{"decay", "s", "%"},
		{"pre_delay", "%", "%"},
		{"mix", "s", "s"},
		{"decay", "%", "s"},
	}
	for _, c := range rejected {
		if reverbUnitCompatible(c[0], c[1], c[2]) {
			t.Fatalf("reverbUnitCompatible(%q, %q, %q) must stay rejected", c[0], c[1], c[2])
		}
	}
}
