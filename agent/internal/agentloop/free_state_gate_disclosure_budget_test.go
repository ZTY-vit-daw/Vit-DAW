package agentloop

import (
	"testing"
)

// B13-A (2026-09-12): gateG3 receipt fact consistency.
//
// Forensics M-A (queue/reports/2026-09-12-B13-frontier-exhaustion.md §3, artifact
// artifacts/b10_static_eq_reuse/20260911_230347_freshstack/round2_d1_smoke_report.json):
// a receipt with status=partial and
// rejection_reasons=["mix.frequency_relationship: omitted by disclosure budget"]
// passed G1-G4 while G5 could never pass, because the only candidate-producing
// view was dropped by the CCB disclosure budget. G3 and G5 therefore reported
// opposite facts about the same view of the same receipt.
//
// The fix stays mechanical and content-blind: a qualified scan view counts as
// delivered only when no rejection reason carries the disclosure-budget marker
// ("<view_id>: omitted by disclosure budget", emitted at
// capabilitycontext/free_state_observation.go:383) for that view id. No view
// content, track identity, processor, or dosage is inspected.

const b13aDisclosureBudgetReason = "mix.frequency_relationship: omitted by disclosure budget"

// b13aScanReceiptState builds the shared green gate control minus every
// qualified mix scan receipt (the TIMING-2 fixture), then appends exactly one
// project-scan receipt whose delivery facts the caller controls.
func b13aScanReceiptState(status string, requestedViews []any, rejectionReasons []any) *runState {
	state := timing2WithoutMixScanReceipts(nil)
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	row := map[string]any{
		"status": status, "observation_id": "obs-mix-budget", "requested_views": requestedViews,
		"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
	}
	if rejectionReasons != nil {
		row["rejection_reasons"] = rejectionReasons
	}
	ledger["receipts"] = append(messageLoopMapRows(ledger["receipts"]), row)
	return state
}

// b13aMergedLedgerState runs the real recording path (mergeFreeStateObservationLedger)
// over an observation summary and installs the resulting ledger on the green
// control, so G3 is judged on the row the runtime actually writes.
func b13aMergedLedgerState(t *testing.T, summary map[string]any) *runState {
	t.Helper()
	ledger := map[string]any{"schema_version": freeStateObservationLedgerSchema}
	mergeFreeStateObservationLedger(ledger, &RecentObservation{ToolCallID: "call-b13a", Summary: summary}, 1)
	state := timing2WithoutMixScanReceipts(nil)
	messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])["receipts"] = messageLoopMapRows(ledger["receipts"])
	return state
}

// ① RED: partial + the only qualified scan view trimmed by the disclosure
// budget must fail G3. Before B13-A this passed on the nominal requested_views
// hit while G5 could never pass.
func TestB13AG3FailsWhenOnlyScanViewTrimmedByDisclosureBudget(t *testing.T) {
	state := b13aScanReceiptState("partial",
		[]any{"project.structure", "mix.frequency_relationship"},
		[]any{b13aDisclosureBudgetReason})
	if gateG3(state) {
		t.Fatalf("G3 passed on a receipt whose only qualified mix scan view was trimmed by the disclosure budget: %v",
			messageLoopMapRows(messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])["receipts"]))
	}
	// The same receipt keeps its status: the gate, not the receipt, changed.
	if !freeStateReceiptUsable(messageLoopMapRows(messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])["receipts"])[0]) {
		t.Fatal("control: status=partial must stay usable for the other gates")
	}
}

// ① RED via the production recording path: the omitted view must survive from
// the observation summary into the ledger receipt the gate reads.
func TestB13AG3FailsOnMergedReceiptWithDisclosureBudgetOmission(t *testing.T) {
	state := b13aMergedLedgerState(t, map[string]any{
		"status":           "partial",
		"observation_id":   "obs-b13a-trimmed",
		"requested_views":  []any{"project.structure", "mix.frequency_relationship"},
		"project_revision": "rev-7",
		"freshness":        map[string]any{"status": "partial", "project_revision": "rev-7"},
		"omission_reasons": []any{b13aDisclosureBudgetReason},
		"audit_receipt": map[string]any{
			"receipt_id": "ccbr_b13a_trimmed", "schema_version": "ccb_observation_receipt.v1",
			"status": "partial", "rejection_reasons": []any{b13aDisclosureBudgetReason},
		},
	})
	if gateG3(state) {
		t.Fatalf("G3 passed on the merged ledger receipt for a disclosure-budget-trimmed scan view: %v",
			messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])["receipts"])
	}
}

// ② GREEN / zero regression: a delivered qualified scan view keeps passing G3.
func TestB13AG3PassesWhenQualifiedScanViewActuallyDelivered(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		requested []any
		reasons   []any
	}{
		{"ready receipt without rejection reasons", "ready",
			[]any{"mix.frequency_relationship"}, nil},
		{"partial receipt whose other view was trimmed", "partial",
			[]any{"project.structure", "mix.frequency_relationship"},
			[]any{"project.structure: omitted by disclosure budget"}},
		{"partial receipt with one of two scan views trimmed", "partial",
			[]any{"mix.multitrack_relationship", "mix.frequency_relationship"},
			[]any{"mix.frequency_relationship: omitted by disclosure budget"}},
		{"partial receipt trimmed by a non-budget reason", "partial",
			[]any{"mix.multitrack_relationship"},
			[]any{"mix.multitrack_relationship: source evidence is stale"}},
	}
	for _, tc := range cases {
		if !gateG3(b13aScanReceiptState(tc.status, tc.requested, tc.reasons)) {
			t.Fatalf("%s: G3 must still pass, the qualified scan view was delivered", tc.name)
		}
	}

	// The standing fixtures keep their verdicts: the shared green control and
	// the TIMING-2 ledger advance both carry an untrimmed scan receipt.
	if !gateG3(gateTestState(nil)) {
		t.Fatal("the shared green gate control must keep passing G3")
	}
	advanced := timing2WithoutMixScanReceipts(nil)
	timing2AddMixScanReceipt(advanced)
	if !gateG3(advanced) {
		t.Fatal("the TIMING-2 mix scan receipt must keep flipping G3")
	}

	// ③ No-regression on the negative side: a receipt that is not usable at all
	// still fails, with or without disclosure-budget reasons.
	for _, status := range []string{"rejected", "insufficient", "missing", ""} {
		if gateG3(b13aScanReceiptState(status, []any{"mix.frequency_relationship"}, nil)) {
			t.Fatalf("status %q must not satisfy G3", status)
		}
	}
	// A trimmed scan receipt on a merged ledger where a later untrimmed receipt
	// delivered the same view still passes (delivery is per view, not per row).
	state := b13aScanReceiptState("partial", []any{"mix.frequency_relationship"}, []any{b13aDisclosureBudgetReason})
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	ledger["receipts"] = append(messageLoopMapRows(ledger["receipts"]), map[string]any{
		"status": "ready", "observation_id": "obs-mix-late", "requested_views": []any{"mix.frequency_relationship"},
		"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
	})
	if !gateG3(state) {
		t.Fatal("a later delivered scan receipt must satisfy G3 despite an earlier trimmed one")
	}
}
