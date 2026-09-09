package agentloop

import (
	"encoding/json"
	"strings"
	"testing"
)

// REPLAY-1 (2026-09-09, advisory ruling #5 six scenarios): the 4b offline
// replay bed on the MessageCompleter/ToolExecutor seam HARNESS-VER-1 §1
// probed. Matrix rows M18–M23 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md
// §5; the ids are scoped to this replay-2 family, the historical §2 M18/M19
// L3/L4 rows keep their meaning unchanged). Fixtures live in
// testdata/free_state_replay/timing1_*.json — hand-synthesized isomorphic
// model-turn sequences derived from the TIMING-1 pinned behaviors (each
// records its provenance sha256); the green control context mirrors
// gateTestState (target track 1007, candidate tracks 1007/1012, target-level
// observation obs-target at rev-7).
//
// Channel extensions this file pins (card REPLAY-1 three gaps):
//
//   - gap ① budget unpinned: replayFixture.Budget injects ≥10-turn budgets
//     (timing1_m22 runs 11 model calls in one Start leg);
//   - gap ② long convergence: M22 is a ≥10-round hand-synthesized sequence
//     (10 distinct bounded observation turns before the proposal phase);
//   - gap ③ continuation seam: every multi-rejection scenario spans legs via
//     the transient-pause continuation (Result.Continuation.Context carries
//     the durable free_state_reasoning_loop state exactly as the chat side
//     consumes it); replayMergeLoopState models the chat-side merge (host
//     owns closure/ledger, the anti-abuse and terminal keys stay monotonic).
//
// Any red here on TIMING-1 code is acceptance-failure evidence for TIMING-1,
// not a fix target for this bed: production code is untouched by design.

// replayDurableLoopKeys are the free_state_reasoning_loop keys the chat-side
// continuation merge keeps monotonic across slices (TIMING-1 anti-abuse
// accounting + BOUNDARY-1 terminal-turn reservation).
var replayDurableLoopKeys = []string{
	freeStateAdmissionRejectionCountKey,
	freeStateAdmissionRejectionGapsKey,
	freeStateLastRejectedFingerprintKey,
	freeStateLastRejectedEvidenceRevisionKey,
	freeStateTerminalTurnLockedKey,
	"terminal_turn_reason",
	freeStateTerminalRetryCountKey,
}

// replayCloneContext deep-copies a fixture context so derived legs (phase
// variants, host revision advances) never mutate the fixture base.
func replayCloneContext(context map[string]any) map[string]any {
	data, err := json.Marshal(context)
	if err != nil {
		panic(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

// replayMergeLoopState models the chat-side continuation merge at the
// agentloop seam: the host slice provides the fresh closure/ledger context,
// the durable anti-abuse and terminal-turn keys ride the continuation's
// loop state and override the host values.
func replayMergeLoopState(hostContext map[string]any, contLoop map[string]any) map[string]any {
	merged := replayCloneContext(hostContext)
	if len(contLoop) == 0 {
		return merged
	}
	loop := messageLoopMapValue(merged["free_state_reasoning_loop"])
	for _, key := range replayDurableLoopKeys {
		if value, ok := contLoop[key]; ok {
			loop[key] = value
		}
	}
	if len(loop) > 0 {
		merged["free_state_reasoning_loop"] = loop
	}
	return merged
}

// replayLoopStateOf extracts the durable loop state from a paused result's
// continuation — the same slice the chat side resumes from.
func replayLoopStateOf(t *testing.T, result Result) map[string]any {
	t.Helper()
	if result.Continuation == nil {
		t.Fatalf("expected a paused continuation to read the loop state from (stop=%q err=%q)", result.StopReason, result.Error)
	}
	loop := messageLoopMapValue(result.Continuation.Context["free_state_reasoning_loop"])
	if len(loop) == 0 {
		t.Fatal("continuation context carries no free_state_reasoning_loop state")
	}
	return loop
}

// replayCallPromptText joins every message of one recorded model call so
// directive/prompt assertions do not depend on message layout.
func replayCallPromptText(client *fakeMessageCompleter, index int) string {
	if index < 0 || index >= len(client.calls) {
		return ""
	}
	parts := make([]string, 0, len(client.calls[index]))
	for _, message := range client.calls[index] {
		parts = append(parts, message.Content)
	}
	return strings.Join(parts, "\n")
}

// replayTraceGapMessage returns the first final_gate trace message carrying
// the structured admission gap (the G-gate refusal the model received).
func replayTraceGapMessage(result Result) string {
	for _, event := range result.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "structured gap=") {
			return event.Message
		}
	}
	return ""
}

func replayTraceHas(t *testing.T, result Result, kind, needle string) {
	t.Helper()
	if !replayTraceContains(result, kind, needle) {
		t.Fatalf("trace lacks %s event containing %q (stop=%q err=%q)", kind, needle, result.StopReason, result.Error)
	}
}

func replayAssertNoFinalGateBounce(t *testing.T, name string, result Result) {
	t.Helper()
	for _, event := range result.Trace {
		if event.Kind == "final_gate" {
			t.Fatalf("%s: decision was bounced by the output gate: %q (stop=%q err=%q)", name, event.Message, result.StopReason, result.Error)
		}
	}
}

// M18: a complete G-gate-passing needs_experiment proposal submitted early at
// fs4 is admitted — the phase check does not veto it and the gate itself
// passes on the green control. Direct counter-pin of the HARNESS-VER-1 R1/R2
// deadlock shape (fs4 × proposal). The fs9 control keeps the loop honest: a
// settled terminal phase still refuses to reopen via needs_experiment.
func TestFreeStateReplayM18Fs4EarlyCompleteProposalAdmitted(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m18_fs4_early_complete_proposal.json")
	executor := &replayExecutor{}
	result, client := fx.startReplay(executor, fx.Responses[:1], nil, fx.Context)
	if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateNeedsExperiment ||
		result.FreeStateDecision.ImprovementProposal == nil {
		t.Fatalf("m18: the gate-passing fs4 proposal was not admitted as the model artifact: %+v (stop=%q err=%q)",
			result.FreeStateDecision, result.StopReason, result.Error)
	}
	if result.StopReason != StopReasonDone {
		t.Fatalf("m18: admitted proposal must complete the loop, got stop=%q err=%q", result.StopReason, result.Error)
	}
	// The admitted artifact is byte-identical to the model's own proposal.
	scripted, err := parseMessageLoopOutput(fx.Responses[0])
	if err != nil || scripted.FreeStateDecision == nil || scripted.FreeStateDecision.ImprovementProposal == nil {
		t.Fatalf("m18: fixture response is not a parseable proposal turn: %v", err)
	}
	if freeStateProposalFingerprint(result.FreeStateDecision.ImprovementProposal) !=
		freeStateProposalFingerprint(scripted.FreeStateDecision.ImprovementProposal) {
		t.Fatal("m18: the admitted proposal differs from the model's submitted proposal")
	}
	if len(client.calls) != 1 || len(executor.calls) != 0 {
		t.Fatalf("m18: early admission must cost exactly one model turn and zero observations (calls=%d obs=%d)", len(client.calls), len(executor.calls))
	}
	replayAssertNoFinalGateBounce(t, "m18", result)

	// fs9 control: the same proposal is phase-refused in a settled loop and
	// the model settles through its own blocked decision.
	ctx9 := replayCloneContext(fx.Context)
	ctx9["free_state_phase"] = "fs9_terminal"
	messageLoopMapValue(ctx9["minimal_audio_closure"])["phase"] = "fs9_terminal"
	control, controlClient := fx.startReplay(executor, fx.Responses[1:], nil, ctx9)
	replayTraceHas(t, control, "final_gate", "fs9_terminal which does not admit decision status needs_experiment")
	if control.FreeStateDecision == nil || control.FreeStateDecision.Status != FreeStateBlocked {
		t.Fatalf("m18 fs9 control: terminal = %+v (stop=%q err=%q)", control.FreeStateDecision, control.StopReason, control.Error)
	}
	if len(controlClient.calls) != 2 {
		t.Fatalf("m18 fs9 control: model calls = %d, want 2 (bounce + settle)", len(controlClient.calls))
	}
}

// M19: a G7-failing proposal at fs4 bounces with the structured content-blind
// gap — failure category, the model's own unresolved ref, the required
// revision, and the quotable fresh reference; no domain/track/plugin/dosage
// hint. The bounce is accounted (admission_rejection_count=1) and the loop
// still settles through the model's own blocked decision on resume.
func TestFreeStateReplayM19Fs4InsufficientProposalGetsContentBlindGap(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m19_fs4_gap_content_blind.json")
	executor := &replayExecutor{}
	first, client := fx.startReplay(executor, fx.Responses[:2], []int{1}, fx.Context)
	if first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m19: first leg stop = %q, want %s (err=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	issue := replayTraceGapMessage(first)
	if issue == "" {
		t.Fatalf("m19: the G-gate bounce left no structured gap in the trace (err=%q)", first.Error)
	}
	if !strings.Contains(issue, "G7_fresh_revision_bound_refs") {
		t.Fatalf("m19: refusal is not the G7 gap: %q", issue)
	}
	gap := decodeAdmissionGap(t, issue)
	if !gateIDPresent(gap.FailedGateIDs, "G7_fresh_revision_bound_refs") {
		t.Fatalf("m19: gap failed gates = %v", gap.FailedGateIDs)
	}
	found := false
	for _, missing := range gap.Missing {
		if missing.GateID == "G7_fresh_revision_bound_refs" && missing.Condition == "unresolved_evidence_ref" {
			found = true
			if len(missing.EvidenceRefs) != 1 || missing.EvidenceRefs[0] != "obs-ghost" {
				t.Fatalf("m19: gap must name the model's own unresolved ref, got %v", missing.EvidenceRefs)
			}
			if missing.Freshness != "requires project_revision=rev-7" {
				t.Fatalf("m19: gap freshness = %q", missing.Freshness)
			}
		}
	}
	if !found {
		t.Fatalf("m19: gap missing the G7 condition: %+v", gap.Missing)
	}
	if gap.QuotableFreshReference != "obs-target@rev-7" {
		t.Fatalf("m19: quotable fresh reference = %q", gap.QuotableFreshReference)
	}
	assertGapContentBlind(t, issue)
	// The bounced model actually received the gap as its feedback message.
	if prompt := replayCallPromptText(client, 1); !strings.Contains(prompt, "structured gap=") {
		t.Fatal("m19: the structured gap never reached the model as final_gate feedback")
	}
	loop := replayLoopStateOf(t, first)
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 1 {
		t.Fatalf("m19: admission_rejection_count = %d, want 1", count)
	}
	if rows := messageLoopMapRows(loop[freeStateAdmissionRejectionGapsKey]); len(rows) != 1 {
		t.Fatalf("m19: gap records = %d, want 1", len(rows))
	}
	if freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("m19: a single evidence bounce must not lock the terminal turn")
	}
	// Resume through the continuation seam: the loop settles on the model's
	// own blocked decision, never a system ghostwrite.
	resume := replayMergeLoopState(fx.Context, loop)
	second, _ := fx.startReplay(executor, fx.Responses[1:], nil, resume)
	if second.FreeStateDecision == nil || second.FreeStateDecision.Status != FreeStateBlocked || second.StopReason != StopReasonDone {
		t.Fatalf("m19: resume terminal = %+v stop=%q (err=%q)", second.FreeStateDecision, second.StopReason, second.Error)
	}
}

// M20: an identical-fingerprint resubmission without a new evidence revision
// never re-enters the ordinary loop — the second bounce locks the terminal
// turn directly (dedicated content-free reason), the very next model call is
// the terminal prompt with the accumulated gaps, and no observation is
// executed. The model still settles with its own blocked decision.
func TestFreeStateReplayM20DuplicateFingerprintLocksTerminalPrompt(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m20_duplicate_fingerprint_rate_limit.json")
	executor := &replayExecutor{}
	first, client := fx.startReplay(executor, fx.Responses[:3], []int{2}, fx.Context)
	if first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m20: first leg stop = %q, want %s (err=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	loop := replayLoopStateOf(t, first)
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 2 {
		t.Fatalf("m20: admission_rejection_count = %d, want 2", count)
	}
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("m20: the identical resubmission must lock the terminal turn")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != freeStateTerminalReasonDuplicateFingerprint {
		t.Fatalf("m20: terminal lock reason = %q, want %s", reason, freeStateTerminalReasonDuplicateFingerprint)
	}
	if fp := messageLoopText(loop[freeStateLastRejectedFingerprintKey]); !strings.HasPrefix(fp, "proposal:") {
		t.Fatalf("m20: rejected fingerprint not latched: %q", fp)
	}
	if rows := messageLoopMapRows(loop[freeStateAdmissionRejectionGapsKey]); len(rows) != 2 {
		t.Fatalf("m20: gap records = %d, want 2", len(rows))
	}
	// The first bounce stayed an ordinary gap round; the call after the
	// duplicate is already the terminal prompt with the accumulated gaps.
	if prompt := replayCallPromptText(client, 1); strings.Contains(prompt, "TERMINAL TURN") {
		t.Fatal("m20: the first bounce must keep its ordinary gap feedback")
	}
	terminalPrompt := replayCallPromptText(client, 2)
	for _, needle := range []string{"TERMINAL TURN", "ADMISSION GAPS ACCUMULATED", "rejection#1", "rejection#2"} {
		if !strings.Contains(terminalPrompt, needle) {
			t.Fatalf("m20: terminal prompt missing %q", needle)
		}
	}
	if len(executor.calls) != 0 {
		t.Fatalf("m20: the rate-limited path re-entered the ordinary observation loop (%d calls)", len(executor.calls))
	}
	// The locked turn still lets the model decide: its own blocked decision
	// completes the loop (no system fallback, no ghostwrite).
	resume := replayMergeLoopState(fx.Context, loop)
	second, _ := fx.startReplay(executor, fx.Responses[2:], nil, resume)
	if second.FreeStateDecision == nil || second.FreeStateDecision.Status != FreeStateBlocked || second.StopReason != StopReasonDone {
		t.Fatalf("m20: resume terminal = %+v stop=%q (err=%q)", second.FreeStateDecision, second.StopReason, second.Error)
	}
}

// M21: after the host advances the evidence revision, the repaired proposal
// citing the fresh target-level observation is admitted on its first try —
// the fingerprint rate limit does not hurt lawful repairs, and the recorded
// rejection stays at one.
func TestFreeStateReplayM21NewEvidenceRevisionRepairAdmitted(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m21_new_revision_repair.json")
	executor := &replayExecutor{}
	first, _ := fx.startReplay(executor, fx.Responses[:2], []int{1}, fx.Context)
	if first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m21: first leg stop = %q, want %s (err=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	loop := replayLoopStateOf(t, first)
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 1 {
		t.Fatalf("m21: admission_rejection_count = %d, want 1", count)
	}
	if revision := messageLoopText(loop[freeStateLastRejectedEvidenceRevisionKey]); revision != "rev-7" {
		t.Fatalf("m21: rejected evidence revision = %q, want rev-7", revision)
	}
	if freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("m21: the first bounce must not lock the terminal turn")
	}
	// Host advance between slices: project revision bumps to rev-8 with a
	// fresh target-level observation bound to it (the fixture's documented
	// in-test transform).
	advanced := replayCloneContext(fx.Context)
	messageLoopMapValue(advanced["task_contract"])["project_revision"] = "rev-8"
	closure := messageLoopMapValue(advanced["minimal_audio_closure"])
	closure["project_revision"] = "rev-8"
	loopCtx := messageLoopMapValue(advanced["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loopCtx["observation_ledger"])
	freshReceipt := map[string]any{
		"status": "ready", "observation_id": "obs-target-2", "requested_views": []any{"track.timbre_frequency"},
		"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target-2"},
		"project_revision": "rev-8", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-8"},
	}
	ledger["receipts"] = append(messageLoopMapRows(ledger["receipts"]), freshReceipt)
	messageLoopMapValue(ledger["available_views"])["track:1007::track.timbre_frequency"] = map[string]any{
		"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-target-2",
		"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target-2"},
		"freshness": map[string]any{"status": "fresh", "project_revision": "rev-8"},
	}
	resume := replayMergeLoopState(advanced, loop)
	second, client := fx.startReplay(executor, fx.Responses[1:], nil, resume)
	if second.FreeStateDecision == nil || second.FreeStateDecision.Status != FreeStateNeedsExperiment ||
		second.FreeStateDecision.ImprovementProposal == nil {
		t.Fatalf("m21: the repaired proposal was not admitted: %+v (stop=%q err=%q)", second.FreeStateDecision, second.StopReason, second.Error)
	}
	if refs := second.FreeStateDecision.ImprovementProposal.EvidenceRefs; len(refs) != 1 || refs[0] != "obs-target-2" {
		t.Fatalf("m21: admitted proposal cites %v, want the fresh rev-8 observation", refs)
	}
	if second.StopReason != StopReasonDone {
		t.Fatalf("m21: admitted repair must complete the loop, got stop=%q err=%q", second.StopReason, second.Error)
	}
	replayAssertNoFinalGateBounce(t, "m21", second)
	if prompt := replayCallPromptText(client, 0); strings.Contains(prompt, "TERMINAL TURN") {
		t.Fatal("m21: the repair round must never see a terminal prompt (no lock fired)")
	}
}

// M22: long convergence (>=10 observation rounds in one Start leg on the
// unpinned budget), then two distinct G7-failing proposals bounce on the
// host-advanced green context; the second rejection locks the terminal turn
// with the accumulated gap disclosure, and the lawful proposal is still
// admitted inside the locked terminal turn — the model settles with its own
// artifact, the system never ghostwrites blocked.
func TestFreeStateReplayM22LongConvergenceTwoRejectionsModelSettles(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m22_long_two_rejections_terminal.json")
	executor := &replayExecutor{results: append([]map[string]any(nil), fx.ToolResults...)}
	// Leg 1 — pre-frontier diagnostic prefix: 10 distinct bounded
	// observations, then a transient pause at the continuation seam.
	prefixCtx := replayCloneContext(fx.Context)
	delete(messageLoopMapValue(prefixCtx["minimal_audio_closure"]), "hypothesis_frontier")
	first, client := fx.startReplay(executor, fx.Responses[:11], []int{10}, prefixCtx)
	if first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m22: leg 1 stop = %q, want %s (err=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	if len(client.calls) != 11 {
		t.Fatalf("m22: leg 1 model calls = %d, want 11 (10 turns + pause probe)", len(client.calls))
	}
	if first.Continuation == nil || first.Continuation.TurnsUsed != 10 {
		t.Fatalf("m22: leg 1 must consume 10 model turns (continuation=%+v)", first.Continuation)
	}
	if len(executor.calls) != 10 {
		t.Fatalf("m22: leg 1 observation calls = %d, want 10 (one per distinct view set)", len(executor.calls))
	}
	// Leg 2 — host advanced the closure (frontier + green ledger); two
	// distinct failing proposals bounce, the second locks the terminal turn.
	advanced := replayMergeLoopState(fx.Context, replayLoopStateOf(t, first))
	second, client2 := fx.startReplay(executor, fx.Responses[10:], []int{2}, advanced)
	if second.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m22: leg 2 stop = %q, want %s (err=%q)", second.StopReason, StopReasonTransientLLMError, second.Error)
	}
	loop := replayLoopStateOf(t, second)
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 2 {
		t.Fatalf("m22: admission_rejection_count = %d, want 2", count)
	}
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("m22: the second G-gate rejection must lock the terminal turn")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != freeStateTerminalReasonAdmissionExhausted {
		t.Fatalf("m22: terminal lock reason = %q, want %s", reason, freeStateTerminalReasonAdmissionExhausted)
	}
	terminalPrompt := replayCallPromptText(client2, 2)
	for _, needle := range []string{"TERMINAL TURN", "ADMISSION GAPS ACCUMULATED", "rejection#1", "rejection#2"} {
		if !strings.Contains(terminalPrompt, needle) {
			t.Fatalf("m22: terminal prompt missing %q", needle)
		}
	}
	// Leg 3 — the locked terminal turn still admits the lawful proposal; the
	// model settles with its own artifact.
	resume := replayMergeLoopState(fx.Context, loop)
	third, client3 := fx.startReplay(executor, fx.Responses[12:], nil, resume)
	if third.FreeStateDecision == nil || third.FreeStateDecision.Status != FreeStateNeedsExperiment ||
		third.FreeStateDecision.ImprovementProposal == nil || third.StopReason != StopReasonDone {
		t.Fatalf("m22: the locked terminal turn refused the lawful proposal: %+v stop=%q (err=%q)",
			third.FreeStateDecision, third.StopReason, third.Error)
	}
	if refs := third.FreeStateDecision.ImprovementProposal.EvidenceRefs; len(refs) != 1 || refs[0] != "obs-target" {
		t.Fatalf("m22: admitted terminal proposal cites %v", refs)
	}
	if prompt := replayCallPromptText(client3, 0); !strings.Contains(prompt, "TERMINAL TURN") {
		t.Fatal("m22: the final admission round must run inside the locked terminal turn")
	}
	if len(executor.calls) != 10 {
		t.Fatalf("m22: no further observation may run after the frontier (%d calls)", len(executor.calls))
	}
}

// M23: the chat-side budget-critical terminal reservation survives an
// evidence bounce unconsumed — the lock and its reason are never downgraded,
// the window counters do not move, no observation spends the reserved
// checkpoint, and after the one strengthened retry the loop settles through
// the honest fallback (terminal turn first, never a silent system exit).
func TestFreeStateReplayM23TerminalReservationNotConsumedByBounce(t *testing.T) {
	fx := loadReplayFixture(t, "timing1_m23_reservation_not_consumed.json")
	executor := &replayExecutor{}
	first, client := fx.startReplay(executor, fx.Responses[:2], []int{1}, fx.Context)
	if first.StopReason != StopReasonTransientLLMError {
		t.Fatalf("m23: first leg stop = %q, want %s (err=%q)", first.StopReason, StopReasonTransientLLMError, first.Error)
	}
	loop := replayLoopStateOf(t, first)
	if !freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatal("m23: the reservation lock was cleared by the bounce")
	}
	if reason := messageLoopText(loop["terminal_turn_reason"]); reason != "budget_critical" {
		t.Fatalf("m23: reservation reason downgraded to %q", reason)
	}
	if retry := messageLoopFreeStatePositiveInt(loop[freeStateTerminalRetryCountKey]); retry != 1 {
		t.Fatalf("m23: strengthened retry count = %d, want 1", retry)
	}
	if count := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); count != 0 {
		t.Fatalf("m23: a locked-turn bounce must not run the ordinary anti-abuse note (count=%d)", count)
	}
	// The evidence bounce consumed no checkpoint: the window counters in the
	// durable context are unchanged and the live directive prices the same
	// remaining window on both sides of the bounce.
	if used := messageLoopFreeStatePositiveInt(loop["continuation_used"]); used != 5 {
		t.Fatalf("m23: continuation_used moved during the bounce: %d", used)
	}
	closure := messageLoopMapValue(first.Continuation.Context["minimal_audio_closure"])
	if started := messageLoopFreeStatePositiveInt(closure["rounds_started"]); started != 5 {
		t.Fatalf("m23: rounds_started moved during the bounce: %d", started)
	}
	window := "closure rounds remaining: 1/6; continuations remaining: 1/6."
	if prompt := replayCallPromptText(client, 0); !strings.Contains(prompt, window) {
		t.Fatal("m23: pre-bounce terminal directive lost the window facts")
	}
	if len(executor.calls) != 0 {
		t.Fatalf("m23: the reserved checkpoint was consumed by an observation (%d calls)", len(executor.calls))
	}
	// After the one strengthened retry the loop settles through the honest
	// fallback: no ghostwritten model decision, terminal turn first.
	resume := replayMergeLoopState(fx.Context, loop)
	second, client2 := fx.startReplay(executor, fx.Responses[1:], nil, resume)
	if second.StopReason != FreeStateTerminalFallbackStopReason {
		t.Fatalf("m23: fallback stop = %q, want %s (err=%q)", second.StopReason, FreeStateTerminalFallbackStopReason, second.Error)
	}
	if second.FreeStateDecision != nil {
		t.Fatalf("m23: the system ghostwrote a model decision on the fallback: %+v", second.FreeStateDecision)
	}
	if !strings.Contains(second.Error, "terminal turn produced no admissible final decision") {
		t.Fatalf("m23: fallback error lost the honest reason: %q", second.Error)
	}
	gateIndex, fallbackIndex := -1, -1
	for i, event := range second.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "structured gap=") && gateIndex < 0 {
			gateIndex = i
		}
		if event.Kind == "final_gate" && strings.Contains(event.Message, "terminal-turn fallback") && fallbackIndex < 0 {
			fallbackIndex = i
		}
	}
	if gateIndex < 0 || fallbackIndex < 0 || gateIndex > fallbackIndex {
		t.Fatalf("m23: the fallback must follow the gate refusal (gate=%d fallback=%d)", gateIndex, fallbackIndex)
	}
	if prompt := replayCallPromptText(client2, 0); !strings.Contains(prompt, window) {
		t.Fatalf("m23: the bounce shrank the reserved window in the live directive: %q", prompt)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("m23: the fallback path executed an observation (%d calls)", len(executor.calls))
	}
}
