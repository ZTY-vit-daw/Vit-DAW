package taskstate

import (
	"reflect"
	"testing"
)

// M3-FS-GATE-EVIDENCE-WIRE-1 B: the capability_blocked terminal's same-type
// re-entry is an idempotent no-op. The slicebudget_1 failure mode: an upstream
// settle retry re-declared the identical blocked outcome and the transition
// table's from.Terminal() guard turned that repeat into invalidTransition,
// surfacing the goal as failed while the snapshot already said exactly what
// the re-entry said. Only this one same-type pair is admitted; every other
// terminal transition keeps the refusal.

func blockedTestSnapshot(t *testing.T) Snapshot {
	t.Helper()
	contract := testContract(ContractImprovement)
	state, err := New(contract, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return mustApply(t, contract, state, TransitionRequest{
		Event: EventCapabilityBlocked, Reason: "admission gate refused the proposal",
		EvidenceRefs: []string{"obs-1"}, Summary: "capability boundary reached",
	})
}

// The idempotent no-op: repeating capability_blocked succeeds and changes
// nothing — no new revision, no history record, no state move.
func TestCapabilityBlockedSameTypeReentryIsIdempotentNoOp(t *testing.T) {
	contract := testContract(ContractImprovement)
	state := blockedTestSnapshot(t)
	if state.State != StateCapabilityBlocked || !state.Terminal {
		t.Fatalf("fixture must be a blocked terminal, got %+v", state)
	}
	reentry, err := Apply(contract, state, TransitionRequest{
		Event: EventCapabilityBlocked, Reason: "scheduler re-delivered the identical blocked settle",
		EvidenceRefs: []string{"obs-1"},
	}, testNow)
	if err != nil {
		t.Fatalf("same-type capability_blocked re-entry must be an idempotent success, got %v", err)
	}
	if reentry.Revision != state.Revision {
		t.Fatalf("no-op must not bump the revision: before=%d after=%d", state.Revision, reentry.Revision)
	}
	if len(reentry.History) != len(state.History) {
		t.Fatalf("no-op must not append history: before=%d after=%d", len(state.History), len(reentry.History))
	}
	if reentry.State != StateCapabilityBlocked || !reentry.Terminal {
		t.Fatalf("no-op must keep the terminal state, got %+v", reentry.State)
	}
	if !reflect.DeepEqual(reentry, state) {
		t.Fatalf("no-op must return the snapshot unchanged:\nbefore=%+v\nafter=%+v", state, reentry)
	}
}

// The no-op stays honest: a re-entry without a reason still fails the request
// contract (the reason gate runs before the idempotent arm).
func TestCapabilityBlockedReentryStillRequiresReason(t *testing.T) {
	contract := testContract(ContractImprovement)
	state := blockedTestSnapshot(t)
	if _, err := Apply(contract, state, TransitionRequest{Event: EventCapabilityBlocked}, testNow); err == nil {
		t.Fatal("a reasonless re-entry must still be refused")
	}
}

// Every different-type terminal transition keeps the invalidTransition
// refusal — the table's semantics are unchanged outside the one same-type
// pair (settled/cancelled/failed/closed re-declares included).
func TestCapabilityBlockedStillRejectsDifferentTerminalTransitions(t *testing.T) {
	contract := testContract(ContractImprovement)
	state := blockedTestSnapshot(t)
	for _, request := range []TransitionRequest{
		{Event: EventTaskCancelled, Reason: "user cancelled"},
		{Event: EventTaskFailed, Reason: "runtime failed", EvidenceRefs: []string{"obs-1"}},
		{Event: EventTaskSettled, Reason: "settled later", EvidenceRefs: []string{"obs-1"}, ExperimentID: "experiment-1"},
		{Event: EventOwnerTurnClosed, Reason: "turn closed"},
	} {
		if _, err := Apply(contract, state, request, testNow); err == nil {
			t.Fatalf("capability_blocked -> %s must keep the invalidTransition refusal", request.Event)
		}
	}
}

// Other terminals stay strict about their own same-type re-declares: only
// capability_blocked is admitted (the settle/cancel/fail repeats keep
// failing, so the idempotent arm is deliberately narrow).
func TestOtherTerminalsStillRejectSameTypeRedeclare(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, err := New(contract, testNow)
	if err != nil {
		t.Fatal(err)
	}
	cancelled := mustApply(t, contract, state, TransitionRequest{Event: EventTaskCancelled, Reason: "user cancelled"})
	if _, err := Apply(contract, cancelled, TransitionRequest{Event: EventTaskCancelled, Reason: "repeat cancel"}, testNow); err == nil {
		t.Fatal("cancelled -> cancelled must keep the refusal")
	}
	failed := mustApply(t, contract, state, TransitionRequest{Event: EventTaskFailed, Reason: "runtime failed", EvidenceRefs: []string{"obs-1"}})
	if _, err := Apply(contract, failed, TransitionRequest{Event: EventTaskFailed, Reason: "repeat fail", EvidenceRefs: []string{"obs-1"}}, testNow); err == nil {
		t.Fatal("failed -> failed must keep the refusal")
	}
}

// A non-blocked state entering capability_blocked keeps the normal transition
// (revision bump, history record) — the fix adds no new suppression on the
// first entry.
func TestCapabilityBlockedFirstEntryStillTransitions(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, err := New(contract, testNow)
	if err != nil {
		t.Fatal(err)
	}
	before := state.Revision
	blocked := mustApply(t, contract, state, TransitionRequest{Event: EventCapabilityBlocked, Reason: "gate refused"})
	if blocked.State != StateCapabilityBlocked || blocked.Revision != before+1 || len(blocked.History) != len(state.History)+1 {
		t.Fatalf("first entry must be a real transition, got %+v", blocked)
	}
}
