package experiment

import (
	"math"
	"strings"
	"testing"
)

// M2-TIMEFX-1 domain-table tests: the reverb/delay rows must carry the
// PluginBound write binding, the certified semantic intent anchor, and their
// own native-unit dose bounds; out-of-bound and wrong-key admissions fail
// closed.

func TestD1S1ReverbDomainRowMapping(t *testing.T) {
	spec, ok := D1S1SpecForAction("reverb", "reverb_decay_adjust")
	if !ok {
		t.Fatal("reverb row missing from the domain table")
	}
	if !spec.WriteBinding.PluginBound || spec.WriteBinding.Channels != 1 {
		t.Fatalf("reverb write binding=%+v", spec.WriteBinding)
	}
	if spec.AdmissionValueKey != "decay_pct" {
		t.Fatalf("reverb admission key=%q", spec.AdmissionValueKey)
	}
	if spec.TargetSemantics != "delta_physical" {
		t.Fatalf("reverb target semantics=%q", spec.TargetSemantics)
	}
	if spec.SemanticIntentFamily != "reverb" || len(spec.SemanticIntentCoverage) == 0 {
		t.Fatalf("reverb semantic anchor=%q %+v", spec.SemanticIntentFamily, spec.SemanticIntentCoverage)
	}
	for _, axis := range []string{"decay_time"} {
		found := false
		for _, coverage := range spec.SemanticIntentCoverage {
			if coverage == axis {
				found = true
			}
		}
		if !found {
			t.Fatalf("reverb coverage missing axis %q: %+v", axis, spec.SemanticIntentCoverage)
		}
	}
	if spec.Journal.Tool != "set_plugin_param" || spec.Journal.CommandLabel != "set_plugin_param" {
		t.Fatalf("reverb journal shape=%+v", spec.Journal)
	}
}

func TestD1S1DelayDomainRowMapping(t *testing.T) {
	spec, ok := D1S1SpecForAction("delay", "delay_feedback_adjust")
	if !ok {
		t.Fatal("delay row missing from the domain table")
	}
	if !spec.WriteBinding.PluginBound || spec.WriteBinding.Channels != 1 {
		t.Fatalf("delay write binding=%+v", spec.WriteBinding)
	}
	if spec.AdmissionValueKey != "feedback_pct" {
		t.Fatalf("delay admission key=%q", spec.AdmissionValueKey)
	}
	if spec.TargetSemantics != "delta_physical" {
		t.Fatalf("delay target semantics=%q", spec.TargetSemantics)
	}
	if spec.SemanticIntentFamily != "delay" || len(spec.SemanticIntentCoverage) == 0 {
		t.Fatalf("delay semantic anchor=%q %+v", spec.SemanticIntentFamily, spec.SemanticIntentCoverage)
	}
	for _, axis := range []string{"feedback_amount"} {
		found := false
		for _, coverage := range spec.SemanticIntentCoverage {
			if coverage == axis {
				found = true
			}
		}
		if !found {
			t.Fatalf("delay coverage missing axis %q: %+v", axis, spec.SemanticIntentCoverage)
		}
	}
}

func TestD1S1TimeFXDoseBoundsFailClosed(t *testing.T) {
	reverb, ok := D1S1SpecForAction("reverb", "reverb_decay_adjust")
	if !ok {
		t.Fatal("reverb row missing")
	}
	for _, value := range []float64{0, D1S1MaxAbsDeltaReverbDecayPct + 0.01, -D1S1MaxAbsDeltaReverbDecayPct - 0.01, math.NaN()} {
		if err := reverb.ValidateDoseBounds("diagnostic", map[string]any{"decay_pct": value}); err == nil {
			t.Fatalf("reverb decay_s %v admitted", value)
		}
	}
	// Wrong-key bounds fail closed: a dB-flavored payload must not satisfy the
	// decay axis.
	if err := reverb.ValidateDoseBounds("diagnostic", map[string]any{"decay_s": -0.3}); err == nil {
		t.Fatal("reverb accepted a wrong-unit payload key")
	}
	delay, ok := D1S1SpecForAction("delay", "delay_feedback_adjust")
	if !ok {
		t.Fatal("delay row missing")
	}
	for _, value := range []float64{0, D1S1MaxAbsDeltaDelayFeedbackPct + 0.01, -D1S1MaxAbsDeltaDelayFeedbackPct - 0.01, math.Inf(1)} {
		if err := delay.ValidateDoseBounds("retained", map[string]any{"feedback_pct": value}); err == nil {
			t.Fatalf("delay feedback_pct %v admitted", value)
		}
	}
	if err := delay.ValidateDoseBounds("retained", map[string]any{"feedback_db": 5}); err == nil {
		t.Fatal("delay accepted a wrong-unit payload key")
	}
}

func TestD1S1TimeFXAdmissionValidateD1S1(t *testing.T) {
	reverbAdmission := Admission{
		SchemaVersion:        SchemaVersion,
		TargetRef:            map[string]any{"kind": "track", "id": "vocal"},
		EvidenceRefs:         []string{"obs-timefx-1"},
		Hypothesis:           "the reverb tail drags the vocal",
		ExpectedEffect:       "a shorter tail cleans up the vocal",
		TypedAction:          map[string]any{"action_domain": "reverb", "action_kind": "reverb_decay_adjust", "decay_pct": -3},
		DiagnosticDoseBounds: map[string]any{"decay_pct": -3, "max_action_attempts": 1},
		RetainedDoseBounds:   map[string]any{"decay_pct": -3, "max_action_attempts": 1},
		ExperimentBudget:     1,
		VerificationPlan:     map[string]any{"view_ids": []any{"mix.multitrack_relationship"}},
		CheckpointRef:        "pending:timefx-test",
		RollbackPlan:         map[string]any{"kind": "agent_rollback_action"},
		AuthorityMode:        AuthorityOrdinary,
	}
	if err := reverbAdmission.ValidateD1S1(); err != nil {
		t.Fatalf("valid reverb admission rejected: %v", err)
	}
	outOfBound := reverbAdmission
	outOfBound.TypedAction = map[string]any{"action_domain": "reverb", "action_kind": "reverb_decay_adjust", "decay_pct": -9}
	outOfBound.DiagnosticDoseBounds = map[string]any{"decay_pct": -9, "max_action_attempts": 1}
	outOfBound.RetainedDoseBounds = map[string]any{"decay_pct": -9, "max_action_attempts": 1}
	if err := outOfBound.ValidateD1S1(); err == nil || !strings.Contains(err.Error(), "decay_pct") {
		t.Fatalf("out-of-bound reverb admission accepted: %v", err)
	}
}

func TestD1S1SemanticIntentSpecForTimeFX(t *testing.T) {
	admission := Admission{TypedAction: map[string]any{"action_domain": "delay", "action_kind": "delay_feedback_adjust"}}
	spec, ok := D1S1SemanticIntentSpecFor(admission)
	if !ok || spec.SemanticIntentFamily != "delay" {
		t.Fatalf("delay semantic intent spec=%+v ok=%v", spec, ok)
	}
	// Native domains without anchors keep returning false.
	native := Admission{TypedAction: map[string]any{"action_domain": D1S1ActionDomain, "action_kind": D1S1ActionKind}}
	if _, ok := D1S1SemanticIntentSpecFor(native); ok {
		t.Fatal("track_gain unexpectedly carries a semantic intent anchor")
	}
}
