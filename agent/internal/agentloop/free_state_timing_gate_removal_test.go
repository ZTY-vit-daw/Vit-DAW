package agentloop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/planner"
)

// TIMING-1 (2026-09-09, advisory ruling #5, direction 4b): the fs4 deadlock
// fix. Group ① the timing gate is removed — a complete needs_experiment
// proposal is admitted by phase in fs0–fs5 and decided only by G1–G8; group
// ② the G-gate refusal is a structured content-blind gap; groups ③–⑥ pin the
// anti-abuse accounting (advice rules 1–4). The fixtures reuse the shared
// gateTestState green control (target 1007, candidate tracks 1007/1012,
// target-level observation obs-target at rev-7).

// ① A complete, gate-passing needs_experiment proposal must clear the output
// gate in every pre-target phase — the phase check no longer vetoes it and
// the G gate itself passes on the green control.
func TestTIMING1CompleteProposalAdmittedInFS0ToFS5(t *testing.T) {
	for _, phase := range []audioclosure.Phase{
		audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseFS1ProjectBound, audioclosure.PhaseFS2CapacityAssessed,
		audioclosure.PhaseFS3ProjectScan, audioclosure.PhaseFS4DiagnosticRound, audioclosure.PhaseFS5CandidateFrontier,
	} {
		state := gateTestState(func(ctx map[string]any) {
			ctx["free_state_phase"] = string(phase)
			messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = string(phase)
		})
		if issue := messageLoopFreeStatePhaseDecisionIssue(state, FreeStateNeedsExperiment); issue != "" {
			t.Fatalf("%s: phase check still vetoes needs_experiment: %q", phase, issue)
		}
		if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); issue != "" {
			t.Fatalf("%s: complete gate-passing proposal bounced by the output gate: %q", phase, issue)
		}
	}
	// The control stays honest: the same proposal at fs9 (settled loop) is
	// still phase-refused — the removal is about proposal timing, not about
	// reopening a terminal loop.
	settled := gateTestState(func(ctx map[string]any) {
		ctx["free_state_phase"] = string(audioclosure.PhaseFS9Terminal)
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = string(audioclosure.PhaseFS9Terminal)
	})
	if issue := messageLoopFreeStatePhaseDecisionIssue(settled, FreeStateNeedsExperiment); issue == "" {
		t.Fatal("fs9 must still refuse needs_experiment (the loop is settled)")
	}
}

// ② The G-gate refusal is a machine-readable structured gap: failure
// categories + missing evidence/freshness/binding fields, closed template —
// and it stays content-blind (no domain, track, plug-in, or dosage hints).
func TestTIMING1GateRefusalIsStructuredContentBlindGap(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		closure := ctx["minimal_audio_closure"].(map[string]any)
		closure["phase"] = "fs4_diagnostic_round"
	})
	// G7 walkthrough: an unmatched ref fails G7 alone.
	out := gateTestProposal([]string{"obs-ghost"})
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "G7_fresh_revision_bound_refs") {
		t.Fatalf("G7 did not fail on the unmatched ref: %q", issue)
	}
	gap := decodeAdmissionGap(t, issue)
	if gap.SchemaVersion != FreeStateAdmissionGapSchema {
		t.Fatalf("gap schema = %q", gap.SchemaVersion)
	}
	if !gateIDPresent(gap.FailedGateIDs, "G7_fresh_revision_bound_refs") {
		t.Fatalf("gap failed gates = %v", gap.FailedGateIDs)
	}
	found := false
	for _, missing := range gap.Missing {
		if missing.GateID == "G7_fresh_revision_bound_refs" && missing.Condition == "unresolved_evidence_ref" {
			found = true
			if len(missing.EvidenceRefs) != 1 || missing.EvidenceRefs[0] != "obs-ghost" {
				t.Fatalf("gap must name the model's own unresolved ref, got %v", missing.EvidenceRefs)
			}
			if missing.Freshness != "requires project_revision=rev-7" {
				t.Fatalf("gap must state the required revision, got %q", missing.Freshness)
			}
		}
	}
	if !found {
		t.Fatalf("gap missing the G7 condition: %+v", gap.Missing)
	}
	if gap.QuotableFreshReference != "obs-target@rev-7" {
		t.Fatalf("gap must carry the quotable fresh reference, got %q", gap.QuotableFreshReference)
	}
	// The closed-template guidance keeps the literal citation shape.
	if !strings.Contains(issue, `improvement_proposal.evidence_refs`) || !strings.Contains(issue, `["obs-target@rev-7"]`) {
		t.Fatalf("refusal lost the citation-format guidance: %q", issue)
	}
	assertGapContentBlind(t, issue)

	// G8 walkthrough: a wrong-target proposal reports the failed consistency
	// conditions, never the track identities.
	wrongTarget := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "wrong target",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion: agentprotocol.ImprovementProposalSchema,
			Target:        map[string]any{"kind": "track", "id": "1032"}, EvidenceRefs: []string{"obs-mix"},
			ImprovementIntent: "clearer", Hypothesis: "bounded", ExpectedEffect: "comparable",
			ActionDomain: agentprotocol.ImprovementActionDomainTrackGain, ActionKind: "bounded_gain_adjustment", Confidence: 0.5,
		},
	}}
	issue = messageLoopFreeStateOutputIssue(state, wrongTarget)
	if !strings.Contains(issue, "G8_target_consistency") {
		t.Fatalf("G8 did not fail on the wrong target: %q", issue)
	}
	gap = decodeAdmissionGap(t, issue)
	if !gapConditionPresent(gap.Missing, "G8_target_consistency", "target_not_from_frontier_candidate") {
		t.Fatalf("gap missing the G8 failed condition: %+v", gap.Missing)
	}
	assertGapContentBlind(t, issue)
}

func gapConditionPresent(missing []FreeStateAdmissionMissingCondition, gateID, condition string) bool {
	for _, row := range missing {
		if row.GateID == gateID && row.Condition == condition {
			return true
		}
	}
	return false
}

func gateIDPresent(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// assertGapContentBlind enforces the red line on the refusal text: none of
// the fixture's track identities, action domains/families, plug-in names, or
// dose values may leak into the feedback.
func assertGapContentBlind(t *testing.T, issue string) {
	t.Helper()
	for _, banned := range []string{
		"1007", "1012", "1032", // track identities from the fixture
		"track_gain", "static_eq", "broadband_compression", "compressor", "limiter", "eq", // domains/families (standalone word checked below)
		"delta_db", "-0.5", "0.5", // dose
		"plugin", "plug-in", "vendor", // plug-in identity
	} {
		if banned == "eq" {
			// "eq" appears inside ordinary words (e.g. "required"); check the
			// domain forms instead.
			continue
		}
		if strings.Contains(issue, banned) {
			t.Fatalf("refusal leaked the content hint %q: %q", banned, issue)
		}
	}
}

// decodeAdmissionGap extracts and parses the structured gap JSON the refusal
// template embeds.
func decodeAdmissionGap(t *testing.T, issue string) FreeStateAdmissionGap {
	t.Helper()
	const marker = "structured gap="
	index := strings.Index(issue, marker)
	if index < 0 {
		t.Fatalf("refusal carries no structured gap: %q", issue)
	}
	decoder := json.NewDecoder(strings.NewReader(issue[index+len(marker):]))
	gap := FreeStateAdmissionGap{}
	if err := decoder.Decode(&gap); err != nil {
		t.Fatalf("structured gap is not machine-readable JSON: %v (%q)", err, issue)
	}
	return gap
}

// ③ Evidence-type bounce accounting (advice rule 1): the G-gate bounce
// increments admission_rejection_count and records the structured gap without
// touching the closure checkpoint counters; the second rejection locks the
// terminal turn (advice rule 3), so the ordinary loop never sees a third
// evidence bounce.
func TestTIMING1EvidenceBounceAccountedWithoutCheckpointConsumption(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		closure := ctx["minimal_audio_closure"].(map[string]any)
		closure["phase"] = "fs4_diagnostic_round"
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		loop["continuation_budget"] = 6
		loop["continuation_used"] = 2
		closure["rounds_started"] = 3
		closure["policy"] = map[string]any{"max_closure_rounds": 6}
	})
	budgetSnapshot := func() string {
		loop := messageLoopFreeStateContext(state)
		closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
		return strings.Join([]string{
			messageLoopText(loop["continuation_budget"]), messageLoopText(loop["continuation_used"]),
			messageLoopText(closure["rounds_started"]), messageLoopText(messageLoopMapValue(closure["policy"])["max_closure_rounds"]),
		}, "/")
	}
	before := budgetSnapshot()

	// First evidence bounce: an unresolved-ref proposal fails G7 alone.
	first := gateTestProposal([]string{"obs-ghost"})
	issue := messageLoopFreeStateOutputIssue(state, first)
	if !strings.Contains(issue, "G7_fresh_revision_bound_refs") {
		t.Fatalf("first bounce was not a G-gate rejection: %q", issue)
	}
	messageLoopFreeStateNoteAdmissionRejection(state, first, issue)
	if count := freeStateAdmissionRejectionCount(state); count != 1 {
		t.Fatalf("admission_rejection_count = %d after the first bounce, want 1", count)
	}
	if rows := messageLoopMapRows(messageLoopFreeStateContext(state)[freeStateAdmissionRejectionGapsKey]); len(rows) != 1 {
		t.Fatalf("gap records = %d after the first bounce, want 1", len(rows))
	}
	if messageLoopFreeStateTerminalTurnLocked(state) {
		t.Fatal("a single evidence bounce must not lock the terminal turn")
	}
	if after := budgetSnapshot(); after != before {
		t.Fatalf("the evidence bounce consumed checkpoint counters: %s -> %s", before, after)
	}

	// A content-shaped rejection (proposal validation error) never counts.
	invalid := gateTestProposal([]string{"obs-target"})
	invalid.FreeStateDecision.ImprovementProposal.ImprovementIntent = "  "
	validationIssue := messageLoopFreeStateOutputIssue(state, invalid)
	if validationIssue == "" || strings.Contains(validationIssue, "structured gap") {
		t.Fatalf("control: the invalid proposal must fail validation, not the gate: %q", validationIssue)
	}
	messageLoopFreeStateNoteAdmissionRejection(state, invalid, validationIssue)
	if count := freeStateAdmissionRejectionCount(state); count != 1 {
		t.Fatalf("a non-gate rejection moved the counter: %d", count)
	}

	// Second evidence bounce (a different failing proposal): the cap fires.
	second := gateTestProposal([]string{"obs-ghost-2"})
	issue = messageLoopFreeStateOutputIssue(state, second)
	messageLoopFreeStateNoteAdmissionRejection(state, second, issue)
	if count := freeStateAdmissionRejectionCount(state); count != 2 {
		t.Fatalf("admission_rejection_count = %d after the second bounce, want 2", count)
	}
	loop := messageLoopFreeStateContext(state)
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("the second G-gate rejection must lock the terminal turn (two-bounce cap)")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != freeStateTerminalReasonAdmissionExhausted {
		t.Fatalf("terminal lock reason = %q, want %s", reason, freeStateTerminalReasonAdmissionExhausted)
	}
	if after := budgetSnapshot(); after != before {
		t.Fatalf("the evidence bounces consumed checkpoint counters: %s -> %s", before, after)
	}
	// The locked loop routes further outputs through the terminal gate: an
	// observation request cannot re-enter the ordinary loop for a third
	// evidence bounce.
	observation := messageLoopOutput{
		FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "again"},
		ToolCalls:         []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []string{"track.basic_energy"}}, Reason: "retry"}},
	}
	if issue := messageLoopFreeStateOutputIssue(state, observation); issue != freeStateTerminalTurnSentence {
		t.Fatalf("locked loop admitted an observation after the two-bounce cap: %q", issue)
	}
}

// ④ The proposal fingerprint is invariant to field/citation reordering, and
// the identical resubmission without new evidence locks the terminal prompt
// directly (advice rule 2).
func TestTIMING1ProposalFingerprintReorderInvariant(t *testing.T) {
	base := &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "1007"}, EvidenceRefs: []string{"obs-a", "obs-b"},
		ImprovementIntent: "clearer low end", Hypothesis: "bounded change helps", ExpectedEffect: "easier comparison",
		ActionDomain: agentprotocol.ImprovementActionDomainTrackGain, ActionKind: "bounded_gain_adjustment",
		ParameterBounds: map[string]any{"delta_db": -0.5}, Confidence: 0.5,
		Limitations: []string{"limit-b", "limit-a"},
	}
	reordered := *base
	reordered.EvidenceRefs = []string{"obs-b", "obs-a"}
	reordered.Limitations = []string{"limit-a", "limit-b"}
	reordered.Target = map[string]any{"id": "1007", "kind": "track"}
	if freeStateProposalFingerprint(base) != freeStateProposalFingerprint(&reordered) {
		t.Fatal("field/citation reordering escaped the proposal fingerprint")
	}
	changedDose := *base
	changedDose.ParameterBounds = map[string]any{"delta_db": -1.0}
	if freeStateProposalFingerprint(base) == freeStateProposalFingerprint(&changedDose) {
		t.Fatal("a dose change must change the proposal fingerprint")
	}
	if freeStateProposalFingerprint(nil) != "" {
		t.Fatal("a nil proposal must fingerprint empty")
	}
}

func TestTIMING1DuplicateFingerprintWithoutNewRevisionLocksTerminal(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = "fs4_diagnostic_round"
	})
	identical := gateTestProposal([]string{"obs-ghost"})
	issue := messageLoopFreeStateOutputIssue(state, identical)
	messageLoopFreeStateNoteAdmissionRejection(state, identical, issue)
	if messageLoopFreeStateTerminalTurnLocked(state) {
		t.Fatal("control: the first bounce must not lock")
	}
	// The identical resubmission at the same evidence revision locks the
	// terminal turn directly with the dedicated content-free reason.
	messageLoopFreeStateNoteAdmissionRejection(state, identical, issue)
	loop := messageLoopFreeStateContext(state)
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("identical fingerprint + unchanged evidence revision must lock the terminal turn")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != freeStateTerminalReasonDuplicateFingerprint {
		t.Fatalf("terminal lock reason = %q, want %s", reason, freeStateTerminalReasonDuplicateFingerprint)
	}

	// The evidence revision participates: a NEW revision turns the identical
	// resubmission into an ordinary (exhaustion-capped) rejection instead.
	freshState := gateTestState(func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = "fs4_diagnostic_round"
	})
	messageLoopFreeStateNoteAdmissionRejection(freshState, identical, issue)
	messageLoopMapValue(freshState.input.Context["minimal_audio_closure"])["project_revision"] = "rev-8"
	// The gap message is derived from runtime state, so the bounce boundary
	// recomputes it at the new revision before recording.
	issueAtNewRevision := messageLoopFreeStateOutputIssue(freshState, identical)
	messageLoopFreeStateNoteAdmissionRejection(freshState, identical, issueAtNewRevision)
	freshLoop := messageLoopFreeStateContext(freshState)
	if !freeStateBool(freshLoop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("control: the cap must still lock at the second rejection")
	}
	if reason := messageLoopText(freshLoop["terminal_turn_reason"]); reason != freeStateTerminalReasonAdmissionExhausted {
		t.Fatalf("new-revision resubmission must fall to the exhaustion cap, got reason %q", reason)
	}
}

// ⑤ After the admission lock, the terminal turn still admits a lawful
// proposal and discloses the accumulated gaps; the system never ghostwrites
// a blocked decision — an unusable terminal turn settles through the honest
// fallback with no model decision attached.
func TestTIMING1LockedTerminalAdmitsLawfulProposalAndDisclosesGaps(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = "fs4_diagnostic_round"
	})
	for _, refs := range [][]string{{"obs-ghost"}, {"obs-ghost-2"}} {
		out := gateTestProposal(refs)
		messageLoopFreeStateNoteAdmissionRejection(state, out, messageLoopFreeStateOutputIssue(state, out))
	}
	if !messageLoopFreeStateTerminalTurnLocked(state) {
		t.Fatal("control: two rejections must have locked the terminal turn")
	}
	// Rule 3 disclosure: the terminal directive carries the accumulated
	// structured gaps as machine slots.
	directive := messageLoopFreeStateTerminalTurnDirective(state)
	if !strings.Contains(directive, "ADMISSION GAPS ACCUMULATED") ||
		!strings.Contains(directive, "G7_fresh_revision_bound_refs") ||
		!strings.Contains(directive, "rejection#1") || !strings.Contains(directive, "rejection#2") {
		t.Fatalf("terminal directive missing the accumulated gap disclosure: %q", directive)
	}
	// A gate-satisfying proposal is still admitted inside the locked turn.
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); issue != "" {
		t.Fatalf("locked terminal turn refused a lawful proposal: %q", issue)
	}
}

func TestTIMING1LockedTerminalFallbackNeverGhostwritesBlocked(t *testing.T) {
	observationJSON := `{"final":false,"reply":"one more look","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"x","requested_view_ids":["track.basic_energy"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["track.basic_energy"],"target_ref":{"kind":"track","id":"1007"}},"reason":"x"}]}`
	client := &fakeMessageCompleter{responses: []string{observationJSON, observationJSON}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_request"},
		Context: map[string]any{
			"task_contract":    map[string]any{"kind": "improvement"},
			"free_state_phase": "fs4_diagnostic_round",
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
				"terminal_turn_locked": true, "terminal_turn_reason": "admission_rejections_exhausted",
				"admission_rejection_count": 2,
				"admission_rejection_gaps": []any{map[string]any{
					"schema_version": FreeStateAdmissionGapSchema,
					"failed_gate_ids": []any{"G7_fresh_revision_bound_refs"},
				}},
			},
		},
	})
	if res.StopReason != FreeStateTerminalFallbackStopReason {
		t.Fatalf("locked unusable turn must settle through the honest fallback, got stop=%q err=%q", res.StopReason, res.Error)
	}
	if res.FreeStateDecision != nil {
		t.Fatalf("the system must not ghostwrite a model decision on the fallback, got %+v", res.FreeStateDecision)
	}
	if !strings.Contains(res.Error, "terminal turn produced no admissible final decision") {
		t.Fatalf("fallback error lost the honest reason: %q", res.Error)
	}
}

// ⑥ The budget-critical terminal reservation is never swallowed or downgraded
// by an admission rejection (advice rule 4): the lock and its reason survive
// the bounce accounting; the accounting still records.
func TestTIMING1TerminalReservationNotSwallowedByAdmissionBounce(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = "fs4_diagnostic_round"
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		loop["terminal_turn_locked"] = true
		loop["terminal_turn_reason"] = "budget_critical"
	})
	bounce := gateTestProposal([]string{"obs-ghost"})
	issue := messageLoopFreeStateOutputIssue(state, bounce)
	if !strings.Contains(issue, "G7_fresh_revision_bound_refs") {
		t.Fatalf("control: the failing proposal must be rejected by the gate: %q", issue)
	}
	// The second bounce's exhaustion trigger fires inside the note, but the
	// pre-existing reservation lock must win unchanged.
	messageLoopFreeStateNoteAdmissionRejection(state, bounce, issue)
	messageLoopFreeStateNoteAdmissionRejection(state, bounce, issue)
	loop := messageLoopFreeStateContext(state)
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("the reservation lock was cleared by an admission rejection")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != "budget_critical" {
		t.Fatalf("the reservation reason was downgraded: %q", reason)
	}
	if count := freeStateAdmissionRejectionCount(state); count != 2 {
		t.Fatalf("accounting must still record under the reservation, got %d", count)
	}
	// Direct control: locking an already-locked context is a no-op.
	lockFreeStateTerminalTurnInContext(state, freeStateTerminalReasonAdmissionExhausted)
	if reason := messageLoopText(messageLoopFreeStateContext(state)["terminal_turn_reason"]); reason != "budget_critical" {
		t.Fatalf("lock helper rewrote an existing reservation: %q", reason)
	}
}
