package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
)

// SCAFFOLD-1: the phase-deferred final-candidate marker and its neutral
// resubmission hint. The marker records one procedural fact only — a
// needs_experiment decision was rejected solely because the closure phase did
// not admit it — and the hint is a closed fixed sentence citing that fact.
// The whitelist below pins the discipline: no domain, track, plug-in, dosage,
// or view content may enter the hint, and the hint may only surface when the
// current host phase admits final candidates (the same audioclosure decision
// policy the phase gate enforces).

const phaseDeferredHintSentence = "An earlier final candidate was deferred only because the closure phase did not admit it at that time; the current phase now admits final candidates. Independently re-evaluate whether to submit a final candidate based on the evidence you hold, or end the turn honestly."

func phaseDeferredState(phase string, marked bool) *runState {
	loop := map[string]any{"schema_version": "free_state_reasoning_loop.v1"}
	if marked {
		loop[freeStatePhaseDeferredFinalCandidateKey] = true
	}
	context := map[string]any{
		"task_contract":             map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": loop,
	}
	if phase != "" {
		context["free_state_phase"] = phase
	}
	return &runState{input: Input{Context: context}}
}

func TestPhaseDeferredResubmissionHintIsClosedFixedSentence(t *testing.T) {
	state := phaseDeferredState("fs6_target_confirmed", true)
	directive := messageLoopFreeStatePhaseDeferredResubmissionDirective(state)
	if directive != phaseDeferredHintSentence+"\n\n" {
		t.Fatalf("hint must be the pinned closed sentence, got %q", directive)
	}
	for _, banned := range []string{
		// Domain and family vocabulary never enters the hint.
		"track_gain", "static_eq", "broadband_compression", "compressor", "limiter",
		"de_esser", "multiband_dynamics", "eq", "gain", "db", "hz",
		// Track, plug-in, view, and dosage content never enters the hint.
		"track", "plugin", "plug-in", "view", "view_id", "dose", "dosage",
		// The hint never echoes the rejected proposal or its target.
		"proposal target", "evidence_ref",
	} {
		if strings.Contains(strings.ToLower(directive), strings.ToLower(banned)) {
			t.Fatalf("resubmission hint carries domain content %q in %q", banned, directive)
		}
	}
}

func TestPhaseDeferredResubmissionDirectiveTriggerMatrix(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		marked bool
		want  bool
	}{
		{"marker absent fs6", "fs6_target_confirmed", false, false},
		{"marker present phase still refuses", "fs4_diagnostic_round", true, false},
		{"marker present fs5 still refuses", "fs5_candidate_frontier", true, false},
		{"marker present fs6 admits", "fs6_target_confirmed", true, true},
		{"marker present fs7 admits", "fs7_improvement_proposal", true, true},
		{"marker present fs8 admits", "fs8_experiment_verification", true, true},
		{"marker present fs9 terminal only", "fs9_terminal", true, false},
		{"marker present no phase disclosed", "", true, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			directive := messageLoopFreeStatePhaseDeferredResubmissionDirective(phaseDeferredState(testCase.phase, testCase.marked))
			if got := directive != ""; got != testCase.want {
				t.Fatalf("directive presence = %v, want %v (directive %q)", got, testCase.want, directive)
			}
		})
	}
}

func phaseDeferredNeedsExperimentDecision() *FreeStateDecision {
	return &FreeStateDecision{
		SchemaVersion:  FreeStateDecisionSchema,
		Status:         FreeStateNeedsExperiment,
		EvidenceStatus: "plausible",
		Summary:        "model summary",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion:     agentprotocol.ImprovementProposalSchema,
			Target:            map[string]any{"kind": "track", "id": "visible-id"},
			EvidenceRefs:      []string{"obs-1"},
			ImprovementIntent: "model intent",
			Hypothesis:        "model hypothesis",
			ExpectedEffect:    "model effect",
			ActionDomain:      "track_gain",
			ActionKind:        "track_gain_adjust",
			Confidence:        0.5,
		},
	}
}

func TestPhaseDeferredFinalCandidateMarkerSetOnlyForPhaseRejectedNeedsExperiment(t *testing.T) {
	t.Run("phase rejected needs_experiment sets marker", func(t *testing.T) {
		state := phaseDeferredState("fs4_diagnostic_round", false)
		out := messageLoopOutput{FreeStateDecision: phaseDeferredNeedsExperimentDecision()}
		issue := messageLoopFreeStateOutputIssue(state, out)
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, issue)
		loop := messageLoopFreeStateContext(state)
		if !freeStateBool(loop[freeStatePhaseDeferredFinalCandidateKey]) {
			t.Fatalf("phase rejection of needs_experiment must set the marker, issue %q", issue)
		}
		if count := messageLoopFreeStatePositiveInt(loop[freeStatePhaseDeferredFinalCandidateCount]); count != 1 {
			t.Fatalf("first rejection must count 1, got %d", count)
		}
		if phase := firstMapText(loop, freeStatePhaseDeferredFinalCandidatePhase); phase != string(audioclosure.PhaseFS4DiagnosticRound) {
			t.Fatalf("recorded phase = %q, want fs4", phase)
		}
		// A second bounced proposal increments the counter without changing
		// the recorded rejection semantics.
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, issue)
		if count := messageLoopFreeStatePositiveInt(loop[freeStatePhaseDeferredFinalCandidateCount]); count != 2 {
			t.Fatalf("second rejection must count 2, got %d", count)
		}
	})
	t.Run("content-shaped rejection at the same phase does not set the marker", func(t *testing.T) {
		state := phaseDeferredState("fs4_diagnostic_round", false)
		decision := phaseDeferredNeedsExperimentDecision()
		decision.ImprovementProposal = nil
		out := messageLoopOutput{FreeStateDecision: decision}
		issue := messageLoopFreeStateOutputIssue(state, out)
		if issue == "" {
			t.Fatal("expected the proposal-shape gate to reject the decision")
		}
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, issue)
		loop := messageLoopFreeStateContext(state)
		if freeStateBool(loop[freeStatePhaseDeferredFinalCandidateKey]) {
			t.Fatal("a content-shaped rejection must not set the marker")
		}
	})
	t.Run("non needs_experiment status never sets the marker", func(t *testing.T) {
		// The phase decision policy admits terminal statuses wherever needs_observation
		// is admitted, so a capability_blocked decision is never bounced by the phase
		// gate; the status predicate below is the defense in depth.
		state := phaseDeferredState("fs4_diagnostic_round", false)
		out := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
			SchemaVersion:  FreeStateDecisionSchema,
			Status:         FreeStateCapabilityBlocked,
			EvidenceStatus: "insufficient",
			Summary:        "boundary",
		}}
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, "any rejection issue")
		loop := messageLoopFreeStateContext(state)
		if freeStateBool(loop[freeStatePhaseDeferredFinalCandidateKey]) {
			t.Fatal("only a needs_experiment rejection may set the marker")
		}
	})
	t.Run("needs_experiment admitted by the phase does not set the marker", func(t *testing.T) {
		state := phaseDeferredState("fs6_target_confirmed", false)
		decision := phaseDeferredNeedsExperimentDecision()
		decision.ImprovementProposal = nil
		out := messageLoopOutput{FreeStateDecision: decision}
		issue := messageLoopFreeStateOutputIssue(state, out)
		if issue == "" {
			t.Fatal("expected the proposal-shape gate to reject the decision")
		}
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, issue)
		loop := messageLoopFreeStateContext(state)
		if freeStateBool(loop[freeStatePhaseDeferredFinalCandidateKey]) {
			t.Fatal("a rejection the phase gate did not produce must not set the marker")
		}
	})
	t.Run("no loop context is a no-op", func(t *testing.T) {
		state := &runState{input: Input{Context: map[string]any{"free_state_phase": "fs4_diagnostic_round"}}}
		out := messageLoopOutput{FreeStateDecision: phaseDeferredNeedsExperimentDecision()}
		issue := messageLoopFreeStatePhaseDecisionIssue(state, FreeStateNeedsExperiment)
		messageLoopFreeStateNotePhaseDeferredFinalCandidate(state, out, issue)
	})
}

func TestPhaseDeferredHintRidesSystemPromptWithoutDisturbingExistingDirectives(t *testing.T) {
	newState := func(marked bool) *runState {
		state := phaseDeferredState("fs6_target_confirmed", marked)
		loop := messageLoopFreeStateContext(state)
		loop["observation_saturation_notice"] = map[string]any{
			"schema_version":      "free_state_observation_saturation_notice.v1",
			"coverage_status":     "per_track_primary_complete",
			"open_dimensions":     []any{"level_headroom"},
			"frontier_candidates": 0,
			"continuation_used":   7,
			"continuation_budget": 11,
		}
		loop["continuation_budget"] = 2
		loop["continuation_used"] = 2
		return state
	}
	plain := messageLoopNeutralFamilySystemPrompt(newState(false))
	marked := messageLoopNeutralFamilySystemPrompt(newState(true))
	if strings.Contains(plain, phaseDeferredHintSentence) {
		t.Fatal("prompt without the marker must not carry the resubmission hint")
	}
	if !strings.Contains(marked, phaseDeferredHintSentence) {
		t.Fatal("prompt with the marker at an admitting phase must carry the resubmission hint")
	}
	if !strings.Contains(marked, "OBSERVATION SATURATION") {
		t.Fatal("the saturation notice directive must still render beside the hint")
	}
	if !strings.Contains(marked, "CONTINUATION BUDGET CRITICAL") {
		t.Fatal("the continuation-budget directive must still render beside the hint")
	}
	// The hint is a pure insertion: removing the exact hint text from the
	// marked prompt restores the unmarked prompt byte for byte.
	if restored := strings.Replace(marked, freeStatePhaseDeferredResubmissionHint+"\n\n", "", 1); restored != plain {
		t.Fatalf("the hint must be appended verbatim without reordering or altering existing directives")
	}
}

func TestPhaseDeferredHintDoesNotChangeBudgetDirectiveText(t *testing.T) {
	newState := func(marked bool) *runState {
		state := phaseDeferredState("fs6_target_confirmed", marked)
		loop := messageLoopFreeStateContext(state)
		loop["continuation_budget"] = 3
		loop["continuation_used"] = 3
		return state
	}
	if unmarked, markedText := messageLoopFreeStateContinuationBudgetDirective(newState(false)), messageLoopFreeStateContinuationBudgetDirective(newState(true)); unmarked != markedText {
		t.Fatalf("budget directive must not depend on the marker:\nunmarked %q\nmarked   %q", unmarked, markedText)
	}
	if saturationUnmarked, saturationMarked := messageLoopFreeStateObservationSaturationDirective(phaseDeferredState("fs6_target_confirmed", false)), messageLoopFreeStateObservationSaturationDirective(phaseDeferredState("fs6_target_confirmed", true)); saturationUnmarked != saturationMarked {
		t.Fatal("saturation directive must not depend on the marker")
	}
}
