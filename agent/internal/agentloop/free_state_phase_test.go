package agentloop

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/taskstate"
)

// Matrix M01/M02 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md): the FS0–FS9
// phase machine hosted by audioclosure — legal forward chain with
// phase_transition events, and the five illegal-transition examples rejected.

func phaseTestStart(t *testing.T) audioclosure.State {
	t.Helper()
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-phase", ConversationID: "conv-phase",
		ProjectUUID: "proj-1", ProjectRevision: "rev-7", OriginalIntent: "inspect the project",
		Mode: audioclosure.ModeDiagnostic, Policy: audioclosure.DefaultPolicy(), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("start closure: %v", err)
	}
	return state
}

func phaseTestTransition(t *testing.T, driver audioclosure.Driver, state audioclosure.State, to audioclosure.Phase, guard audioclosure.PhaseGuardInput) audioclosure.State {
	t.Helper()
	next, err := driver.TransitionPhase(state, state.Revision, to, guard, "", time.Now().UTC())
	if err != nil {
		t.Fatalf("transition to %s failed: %v", to, err)
	}
	return next
}

func allGuardsPass(overrides ...func(*audioclosure.PhaseGuardInput)) audioclosure.PhaseGuardInput {
	guard := audioclosure.PhaseGuardInput{
		ProjectBound: true, CapacityAssessed: true, ScanUsable: true, QueueNonEmpty: true,
		DimensionClosed: true, FrontierEstablished: true, TargetEvidence: true, GatePassed: true,
		AdmissionValid: true, ContinueOnceRemaining: true, TerminalStopReason: "diagnostic_complete",
	}
	for _, override := range overrides {
		override(&guard)
	}
	return guard
}

func TestM01PhaseForwardChainFS0ToFS9(t *testing.T) {
	driver := audioclosure.Driver{}
	state := phaseTestStart(t)
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{})
	steps := []struct {
		to    audioclosure.Phase
		guard audioclosure.PhaseGuardInput
	}{
		{audioclosure.PhaseFS1ProjectBound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.CapacityAssessed = false
			g.ScanUsable = false
			g.QueueNonEmpty = false
			g.DimensionClosed = false
			g.FrontierEstablished = false
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS2CapacityAssessed, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.ScanUsable = false
			g.QueueNonEmpty = false
			g.DimensionClosed = false
			g.FrontierEstablished = false
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS3ProjectScan, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.QueueNonEmpty = false
			g.DimensionClosed = false
			g.FrontierEstablished = false
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS4DiagnosticRound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.DimensionClosed = false
			g.FrontierEstablished = false
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		// FS4 self-loop: the next diagnostic round.
		{audioclosure.PhaseFS4DiagnosticRound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.FrontierEstablished = false
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS5CandidateFrontier, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.TargetEvidence = false
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS6TargetConfirmed, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.GatePassed = false
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS7ImprovementProposal, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
			g.AdmissionValid = false
			g.ContinueOnceRemaining = false
			g.TerminalStopReason = ""
		})},
		{audioclosure.PhaseFS8ExperimentVerification, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.ContinueOnceRemaining = false; g.TerminalStopReason = "" })},
		// FS8 self-loop: the single continue_once allowance.
		{audioclosure.PhaseFS8ExperimentVerification, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" })},
		{audioclosure.PhaseFS9Terminal, allGuardsPass()},
	}
	transitions := 0
	for _, step := range steps {
		before := state.Revision
		state = phaseTestTransition(t, driver, state, step.to, step.guard)
		if state.Revision != before+1 {
			t.Fatalf("transition to %s did not append exactly one event", step.to)
		}
		transitions++
	}
	if state.Phase != audioclosure.PhaseFS9Terminal {
		t.Fatalf("final phase = %s", state.Phase)
	}
	phaseEvents := 0
	for _, event := range state.Events {
		if event.Type == audioclosure.EventPhaseTransition {
			phaseEvents++
		}
	}
	if phaseEvents != transitions+1 { // +1 for the FS0 entry event
		t.Fatalf("expected %d phase_transition events, got %d", transitions+1, phaseEvents)
	}
	// Restart recovery: replaying the event stream rebuilds the same phase.
	replayed, err := audioclosure.Fold(state.Events)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if replayed.Phase != state.Phase || replayed.Revision != state.Revision {
		t.Fatalf("replayed phase/revision drifted: %s/%d vs %s/%d", replayed.Phase, replayed.Revision, state.Phase, state.Revision)
	}
}

func TestM02IllegalPhaseTransitionsRejected(t *testing.T) {
	driver := audioclosure.Driver{}
	// Example 1: FS0/FS1 -> FS7 skips capacity, scan, frontier, and candidate
	// selection (the shape the old weak gate allowed).
	for _, setup := range []struct {
		phase audioclosure.Phase
		guard audioclosure.PhaseGuardInput
	}{
		{audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{}},
		{audioclosure.PhaseFS1ProjectBound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" })},
	} {
		state := phaseTestStart(t)
		state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{})
		if setup.phase != audioclosure.PhaseFS0SemanticEntry {
			state = phaseTestTransition(t, driver, state, setup.phase, setup.guard)
		}
		if _, err := driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS7ImprovementProposal, allGuardsPass(), "", time.Now().UTC()); err == nil ||
			!strings.Contains(err.Error(), "illegal phase transition") {
			t.Fatalf("jump %s -> FS7 was not rejected: %v", setup.phase, err)
		}
	}
	// Example 2: FS3 -> FS6 without closing any diagnostic dimension.
	state := phaseTestStart(t)
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{})
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS1ProjectBound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" }))
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS2CapacityAssessed, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" }))
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS3ProjectScan, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" }))
	if _, err := driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS6TargetConfirmed, allGuardsPass(), "", time.Now().UTC()); err == nil {
		t.Fatal("FS3 -> FS6 without a closed dimension was accepted")
	}
	// Example 3: FS4 -> FS8 without passing the needs_experiment gate.
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS4DiagnosticRound, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
		g.FrontierEstablished = false
		g.TargetEvidence = false
		g.GatePassed = false
		g.AdmissionValid = false
		g.ContinueOnceRemaining = false
		g.TerminalStopReason = ""
	}))
	if _, err := driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS8ExperimentVerification, allGuardsPass(), "", time.Now().UTC()); err == nil {
		t.Fatal("FS4 -> FS8 without the gate was accepted")
	}
	// Example 4: FS9 admits no successor.
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS5CandidateFrontier, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
		g.TargetEvidence = false
		g.GatePassed = false
		g.AdmissionValid = false
		g.ContinueOnceRemaining = false
		g.TerminalStopReason = ""
	}))
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS6TargetConfirmed, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
		g.GatePassed = false
		g.AdmissionValid = false
		g.ContinueOnceRemaining = false
		g.TerminalStopReason = ""
	}))
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS7ImprovementProposal, allGuardsPass(func(g *audioclosure.PhaseGuardInput) {
		g.AdmissionValid = false
		g.ContinueOnceRemaining = false
		g.TerminalStopReason = ""
	}))
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS8ExperimentVerification, allGuardsPass(func(g *audioclosure.PhaseGuardInput) { g.TerminalStopReason = "" }))
	atFS8 := state
	state = phaseTestTransition(t, driver, state, audioclosure.PhaseFS9Terminal, allGuardsPass())
	if _, err := driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{}, "", time.Now().UTC()); err == nil {
		t.Fatal("FS9 accepted a successor transition")
	}
	// A guard failure must be rejected even on a legal edge: the FS8 self-loop
	// with a spent continue_once allowance.
	if _, err := driver.TransitionPhase(atFS8, atFS8.Revision, audioclosure.PhaseFS8ExperimentVerification, audioclosure.PhaseGuardInput{ContinueOnceRemaining: false, TerminalStopReason: "rolled_back"}, "", time.Now().UTC()); err == nil ||
		!strings.Contains(err.Error(), "guard failed") {
		t.Fatalf("spent continue_once allowance was not rejected: %v", err)
	}
	// Example 5 (decision side): a needs_experiment decision in an early phase
	// is downgraded to needs_observation by the phase-aware decision check.
	early := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect"},
		"free_state_phase":          string(audioclosure.PhaseFS1ProjectBound),
	}}}
	if issue := messageLoopFreeStatePhaseDecisionIssue(early, FreeStateNeedsExperiment); !strings.Contains(issue, "needs_observation") {
		t.Fatalf("early-phase needs_experiment was not downgraded: %q", issue)
	}
	if issue := messageLoopFreeStatePhaseDecisionIssue(early, FreeStateNeedsObservation); issue != "" {
		t.Fatalf("needs_observation rejected in FS1: %q", issue)
	}
	late := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "repair"},
		"free_state_phase":          string(audioclosure.PhaseFS7ImprovementProposal),
	}}}
	if issue := messageLoopFreeStatePhaseDecisionIssue(late, FreeStateNeedsExperiment); issue != "" {
		t.Fatalf("needs_experiment rejected in FS7: %q", issue)
	}
	verification := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating", "original_intent": "repair"},
		"free_state_phase":          string(audioclosure.PhaseFS8ExperimentVerification),
	}}}
	if issue := messageLoopFreeStatePhaseDecisionIssue(verification, FreeStateNeedsObservation); issue != "" {
		t.Fatalf("needs_observation rejected in FS8 verification: %q", issue)
	}
	// The FS8 evaluation report (materiality/target response/round decision)
	// travels on needs_experiment; the phase check must admit it there. The
	// pre-2026-08-25 policy rejected it, so the verification phase could
	// observe but never report its evaluation.
	if issue := messageLoopFreeStatePhaseDecisionIssue(verification, FreeStateNeedsExperiment); issue != "" {
		t.Fatalf("needs_experiment rejected in FS8 verification: %q", issue)
	}
	// Legacy phase values migrate deterministically.
	if migrated, ok := audioclosure.ParsePhase("observing"); !ok || migrated != audioclosure.PhaseFS1ProjectBound {
		t.Fatalf("legacy observing migration = %s/%v", migrated, ok)
	}
	if migrated, ok := audioclosure.ParsePhase("settled"); !ok || migrated != audioclosure.PhaseFS9Terminal {
		t.Fatalf("legacy settled migration = %s/%v", migrated, ok)
	}
	_ = taskstate.StateObservationInProgress
}

// Legacy closure phases migrate onto the FS scale before the phase-aware
// decision check: a restored pre-FS closure in "awaiting_capability" (FS6,
// the governed handoff point where the gate has actually passed) must not
// deadlock a gate-passing needs_experiment behind FS1/FS4 semantics.
func TestLegacyClosurePhaseMigratesForDecisionCheck(t *testing.T) {
	for _, combination := range []struct {
		legacyPhase string
		fsPhase     audioclosure.Phase
	}{
		{"awaiting_capability", audioclosure.PhaseFS6TargetConfirmed},
		{"fs7_improvement_proposal", audioclosure.PhaseFS7ImprovementProposal},
	} {
		state := gateTestState(func(ctx map[string]any) {
			closure := ctx["minimal_audio_closure"].(map[string]any)
			closure["phase"] = combination.legacyPhase
		})
		proposal := gateTestProposal(nil)
		if failed := evaluateFreeStateNeedsExperimentGate(state, proposal.FreeStateDecision); len(failed) != 0 {
			t.Fatalf("%s: gate fixture failed: %v", combination.legacyPhase, failed)
		}
		if issue := messageLoopFreeStateOutputIssue(state, proposal); issue != "" {
			t.Fatalf("%s: gate-passing needs_experiment was deadlocked by the phase check: %q", combination.legacyPhase, issue)
		}
	}
	// An early legacy phase ("reasoning" migrates to FS4) still routes a
	// needs_experiment decision back to needs_observation.
	early := gateTestState(func(ctx map[string]any) {
		closure := ctx["minimal_audio_closure"].(map[string]any)
		closure["phase"] = "reasoning"
	})
	if migrated, ok := audioclosure.ParsePhase("reasoning"); !ok || migrated != audioclosure.PhaseFS4DiagnosticRound {
		t.Fatalf("reasoning migration = %s/%v", migrated, ok)
	}
	if issue := messageLoopFreeStateOutputIssue(early, gateTestProposal(nil)); !strings.Contains(issue, "needs_observation") {
		t.Fatalf("early-phase needs_experiment not downgraded: %q", issue)
	}
}

// An open improvement contract settles only through a governed experiment
// outcome, no_candidate_found, or a capability boundary (its completion
// criteria). A confirmed diagnostic must convert into a bounded proposal
// instead of closing the improvement task as diagnostic_complete
// (2026-08-25 D1 smoke regression: sufficient masking evidence closed the
// loop at fs4_diagnostic_round and the experiment chain never started).
func TestImprovementContractRejectsDiagnosticCompleteTerminal(t *testing.T) {
	out := messageLoopOutput{Final: true, Reply: "confirmed masking diagnostic",
		FreeStateDecision: &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateDiagnosticComplete,
			EvidenceStatus: "sufficient", Summary: "sub/presence masking", ObservationID: "obs-mix",
			Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "confirmed",
				Findings: []FreeStateDiagnosticFinding{{Statement: "sub and presence masking", EvidenceRefs: []string{"obs-mix"}}}},
		}}
	issue := messageLoopFreeStateOutputIssue(gateTestState(nil), out)
	if !strings.Contains(issue, "diagnostic_complete cannot settle an open improvement contract") {
		t.Fatalf("improvement contract accepted a diagnostic_complete terminal: %q", issue)
	}
	diagnosticOnly := gateTestState(func(ctx map[string]any) { ctx["free_state_diagnostic_only"] = true })
	if issue := messageLoopFreeStateOutputIssue(diagnosticOnly, out); strings.Contains(issue, "diagnostic_complete cannot settle an open improvement contract") {
		t.Fatalf("diagnostic-only run was denied its legal diagnostic_complete terminal: %q", issue)
	}
}
// Final-gate defense-in-depth: while the experiment round is durably parked at
// the human-judgment boundary (round decision user_judgment_pending / judgment
// requested / judgment recorded), a model decision that would revive the loop
// (observation / action / new admission) is rejected; only the settle-family
// round decisions remain legal (2026-08-25 21:09 D1 smoke regression).
func TestJudgmentBoundaryRejectsRevivalDecisions(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		loop["experiment"] = map[string]any{
			"status": "waiting_for_user",
			"rounds": []any{map[string]any{
				"round_id": "round-1", "number": 1, "decision": string(experiment.DecisionUserJudgment),
			}},
		}
	})
	observation := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "observe the post-action state again", RequestedViewIDs: []string{"track.basic_energy"},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, observation); !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("needs_observation revived the judgment boundary loop: %q", issue)
	}
	action := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction,
		EvidenceStatus: "sufficient", Summary: "write another adjustment", RemainingIntent: "apply again", ProcessorType: "eq",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, action); !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("needs_action revived the judgment boundary loop: %q", issue)
	}
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("new admission revived the judgment boundary loop: %q", issue)
	}
	settle := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "retain the treatment",
		ExperimentRoundDecision: string(experiment.DecisionRetain),
	}}
	if issue := messageLoopFreeStateOutputIssue(state, settle); issue != "" {
		t.Fatalf("settle decision was rejected at the judgment boundary: %q", issue)
	}
}


