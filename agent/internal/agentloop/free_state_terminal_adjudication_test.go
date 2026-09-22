package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

// FIX-F3-G4-SEMANTICS 方案乙: on a locked terminal turn a complete, valid
// improvement proposal whose only admission failures are evidence-completeness
// gates (G3-G6) must park on the proposal confirmation face instead of running
// the bounce → strengthened-retry → terminal-fallback chain that structurally
// cannot close those gates (tools are banned on a locked turn). The F3 failure
// chain: two honest complete proposals died at gap=[G4_dimension_closed] with
// the model blamed as "unparseable" (run 201633 vs the green 103431 round that
// chose capability_blocked instead — the reverse incentive).

// terminalAdjudicationContext mirrors gateTestState with the terminal-turn
// lock latched after two admission rejections and one specific gate condition
// broken.
func terminalAdjudicationContext(mutate func(ctx map[string]any)) map[string]any {
	ctx := map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-7",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version":      "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
			"observation_ledger": map[string]any{
				"receipts": []any{
					map[string]any{
						"status": "ready", "observation_id": "obs-mix", "requested_views": []any{"mix.frequency_relationship"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
					map[string]any{
						"status": "ready", "observation_id": "obs-target", "requested_views": []any{"track.timbre_frequency"},
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
				"available_views": map[string]any{
					"track:1007::track.timbre_frequency": map[string]any{
						"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-target",
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
			},
			"diagnostic_rounds": []any{map[string]any{
				"schema_version": "free_state_diagnostic_round.v1", "round_id": "r_f3adjud00001",
				"primary_dimension": "frequency_occupancy", "priority_reason": "default_order",
				"views_requested": []any{"track.timbre_frequency"},
				"evidence_status": "ready", "project_revision": "rev-7",
			}},
			"terminal_turn_locked":          true,
			"terminal_turn_reason":          "admission_rejections_exhausted",
			"admission_rejection_count":     2,
		},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7",
			"hypothesis_frontier": map[string]any{
				"candidate_id": "candidate-1",
				"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
			},
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return ctx
}

// terminalAdjudicationUndeliveredScan is the post-甲 reproducible F3 shape: a
// sole evidence-completeness rejection (G3 — the scan receipt is unusable,
// every other gate including the OR-relaxed G4 passes). 方案甲 made a G4-only
// failure structurally unreachable (G4 now fails only when G3 and G6 fail
// too), so the parking path is pinned on the G3 form.
func terminalAdjudicationUndeliveredScan(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	ledger := loop["observation_ledger"].(map[string]any)
	for _, row := range ledger["receipts"].([]any) {
		receipt := row.(map[string]any)
		if receipt["observation_id"] == "obs-mix" {
			receipt["status"] = "rejected"
		}
	}
}

const terminalAdjudicationProposalJSON = `{"final": true, "reply": "The frontier target's fresh evidence supports one bounded gain experiment.", "free_state": {"schema_version": "free_state_decision.v1", "status": "needs_experiment", "evidence_status": "plausible", "summary": "target evidence supports a bounded improvement hypothesis", "improvement_proposal": {"schema_version": "improvement_proposal.v1", "target": {"kind": "track", "id": "1007"}, "evidence_refs": ["obs-target"], "improvement_intent": "make the bass relationship feel clearer", "hypothesis": "a small bounded change may improve separation", "expected_effect": "the relationship should be easier to compare", "action_domain": "track_gain", "action_kind": "bounded_gain_adjustment", "parameter_bounds": {"delta_db": -0.5}, "confidence": 0.55}}}`

const terminalAdjudicationCapabilityBlockedJSON = `{"final": true, "reply": "The remaining evidence boundary cannot be closed on this locked turn.", "free_state": {"schema_version": "free_state_decision.v1", "status": "capability_blocked", "evidence_status": "insufficient", "summary": "the diagnostic dimension did not close within the bounded window", "limitations": ["no closed diagnostic dimension at the terminal boundary"]}}`

// Red test (pre-fix red): the locked-turn complete proposal rejected solely by
// G4 must complete with its decision so the chat side can park it on the
// proposal confirmation face — not die in the terminal fallback.
func TestTerminalCompleteProposalEvidenceCompletenessSoleRejectionParks(t *testing.T) {
	// Post-甲 fixture: G3 (scan receipt unusable) is the sole failing gate.
	client := &fakeMessageCompleter{responses: []string{terminalAdjudicationProposalJSON, terminalAdjudicationProposalJSON}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "please improve this project",
		AllowedTools: []string{"ccb.observation_request"},
		Context:      terminalAdjudicationContext(terminalAdjudicationUndeliveredScan),
	})
	if res.FreeStateDecision == nil || res.FreeStateDecision.ImprovementProposal == nil ||
		res.FreeStateDecision.Status != FreeStateNeedsExperiment {
		t.Fatalf("the parked terminal proposal must reach the confirmation face with its decision, got decision=%+v stop=%q err=%q",
			res.FreeStateDecision, res.StopReason, res.Error)
	}
	if res.StopReason != StopReasonDone {
		t.Fatalf("parked terminal proposal must complete, got stop=%q err=%q", res.StopReason, res.Error)
	}
	if len(client.calls) != 1 {
		t.Fatalf("a parked proposal consumes no strengthened retry (model calls = %d, want 1)", len(client.calls))
	}
}

// The green-round behavior stays untouched: an explicit capability_blocked on
// the locked turn is the model's own terminal and settles as a decision.
func TestTerminalCapabilityBlockedStillSettlesAsModelDecision(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{terminalAdjudicationCapabilityBlockedJSON}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_request"},
		Context:      terminalAdjudicationContext(terminalAdjudicationUndeliveredScan),
	})
	if res.FreeStateDecision == nil || res.FreeStateDecision.Status != FreeStateCapabilityBlocked {
		t.Fatalf("capability_blocked must settle as the model's own decision, got decision=%+v stop=%q err=%q",
			res.FreeStateDecision, res.StopReason, res.Error)
	}
	if res.StopReason == FreeStateTerminalFallbackStopReason || res.StopReason == FreeStateTerminalGateRejectedStopReason {
		t.Fatalf("a clean terminal decision must not hit the fallback, got stop=%q", res.StopReason)
	}
}

// Anti-abuse rules 2/3 untouched: parking neither counts as an admission
// bounce nor clears the lock, and unlocked turns keep the ordinary gap bounce.
func TestTerminalParkingKeepsAntiAbuseAccounting(t *testing.T) {
	state := &runState{input: Input{Context: terminalAdjudicationContext(terminalAdjudicationUndeliveredScan)}}
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); issue != "" {
		t.Fatalf("locked turn must park the complete G4-only proposal, got issue %q", issue)
	}
	loop := messageLoopFreeStateContext(state)
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 2 {
		t.Fatalf("parking must not run the ordinary anti-abuse note (count=%d, want 2)", count)
	}
	if !messageLoopFreeStateTerminalTurnLocked(state) {
		t.Fatal("parking must not clear the terminal-turn lock (parking is not re-entry)")
	}
	// The same G4 failure on an unlocked turn keeps today's bounce semantics.
	unlocked := &runState{input: Input{Context: terminalAdjudicationContext(func(ctx map[string]any) {
		terminalAdjudicationUndeliveredScan(ctx)
		delete(ctx["free_state_reasoning_loop"].(map[string]any), "terminal_turn_locked")
	})}}
	if issue := messageLoopFreeStateOutputIssue(unlocked, gateTestProposal(nil)); !strings.Contains(issue, "needs_experiment requires the full admission gate") {
		t.Fatalf("unlocked turn must keep the ordinary G-gate bounce, got %q", issue)
	}
}

// Red test (pre-change red, FIX-F3-G4-SEMANTICS 方案甲, user ruling 2026-09-22):
// G4 for improvement proposals mirrors the closure spine's own FS5 guard OR
// semantics (audioclosure/phase.go: "an established frontier and a closed
// dimension (or a unique scan-level candidate)"). A complete proposal on a
// state with NO closed diagnostic dimension — but a delivered scan, a frontier,
// and target-level evidence — must pass the admission gate. Pre-change this is
// exactly the G4-only rejection that killed run 201633.
func TestG4ImprovementProposalAlignsSpineOrSemantics(t *testing.T) {
	state := gateTestState(gateVariantNoRounds) // scan + frontier + target evidence intact, no closed dimension
	failed := evaluateFreeStateNeedsExperimentGate(state, gateTestProposal(nil).FreeStateDecision)
	for _, id := range failed {
		if id == freeStateGateG4 {
			t.Fatalf("G4 must not fail on the scan/target OR arms (failed=%v)", failed)
		}
	}
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); issue != "" {
		t.Fatalf("proposal with scan+target basis must be admitted without a closed dimension: %q", issue)
	}
	// The strict closed-dimension predicate itself still fails here: the spine's
	// DimensionClosed disjunct is false; the OR arms carry the admission, and a
	// diagnostic-only loop keeps the strict gate (it never admits proposals).
	if gateG4(state) {
		t.Fatal("the strict dimension-closed predicate must still fail without a closed dimension")
	}
	diagnosticOnly := gateTestState(func(ctx map[string]any) {
		gateVariantNoRounds(ctx)
		ctx["free_state_diagnostic_only"] = true
	})
	if gateG4ImprovementProposal(diagnosticOnly) {
		t.Fatal("diagnostic-only mode keeps the full closed-dimension gate")
	}
	if !gateG4ImprovementProposal(state) {
		t.Fatal("improvement mode must admit via the OR arms")
	}
}

// Red test (pre-fix red): a gate-rejected complete proposal on the locked turn
// settles through the honest fallback with the honest gate-rejected stop
// reason — the output was parseable, complete, and evidence-gate refused, so
// "unparseable" is a false verdict (the F3 forensic wording defect).
func TestTerminalGateRejectedFallbackStopReasonIsHonest(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(ctx map[string]any)
	}{
		{"g7_unresolved_evidence_ref", func(ctx map[string]any) {}}, // proposal cites obs-target; staled below
		{"g1_binding_mismatch", func(ctx map[string]any) {
			ctx["task_contract"] = map[string]any{"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-stale"}
		}},
	}
	for _, tc := range cases {
		ctx := terminalAdjudicationContext(terminalAdjudicationUndeliveredScan)
		tc.mutate(ctx)
		if tc.name == "g7_unresolved_evidence_ref" {
			// G7-only failure: all gates pass except the evidence ref freshness
			// binding (the receipts stay, the proposal cites a stale ref).
			ledger := ctx["free_state_reasoning_loop"].(map[string]any)["observation_ledger"].(map[string]any)
			for _, row := range ledger["receipts"].([]any) {
				receipt := row.(map[string]any)
				if receipt["observation_id"] == "obs-target" {
					receipt["freshness"] = map[string]any{"status": "stale", "project_revision": "rev-7"}
				}
			}
			views := ledger["available_views"].(map[string]any)
			views["track:1007::track.timbre_frequency"].(map[string]any)["freshness"] = map[string]any{"status": "stale", "project_revision": "rev-7"}
		}
		client := &fakeMessageCompleter{responses: []string{terminalAdjudicationProposalJSON, terminalAdjudicationProposalJSON}}
		loop := &MessageLoop{
			Client: client,
			Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
			Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		}
		res := loop.Start(context.Background(), Input{
			UserText:     "improve the mix",
			AllowedTools: []string{"ccb.observation_request"},
			Context:      ctx,
		})
		if res.StopReason != FreeStateTerminalGateRejectedStopReason {
			t.Fatalf("%s: gate-rejected complete proposal must settle with the honest gate-rejected stop reason, got %q (err=%q)",
				tc.name, res.StopReason, res.Error)
		}
		if res.FreeStateDecision != nil {
			t.Fatalf("%s: the fallback must not ghostwrite a model decision, got %+v", tc.name, res.FreeStateDecision)
		}
		if !strings.Contains(res.Error, "terminal turn produced no admissible final decision") {
			t.Fatalf("%s: fallback error lost the honest reason: %q", tc.name, res.Error)
		}
	}
	// Contrast: an inadmissible-shape output (needs_observation on the locked
	// turn) keeps the parse-side fallback stop reason.
	observation := `{"final":false,"reply":"one more look","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"x"},"tool_calls":[{"tool":"ccb.observation_catalog","args":{},"reason":"discover"}]}`
	client := &fakeMessageCompleter{responses: []string{observation, observation}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_catalog"},
		Context:      terminalAdjudicationContext(terminalAdjudicationUndeliveredScan),
	})
	if res.StopReason != FreeStateTerminalFallbackStopReason {
		t.Fatalf("inadmissible-shape output keeps the unparseable fallback label, got %q", res.StopReason)
	}
}
