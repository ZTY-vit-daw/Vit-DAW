package agentloop

import (
	"strings"
	"testing"
)

// B13-D (2026-09-12): available_views delivery-fact consistency.
//
// B13-A §6 secondary finding: mergeFreeStateObservationLedger's available_views
// write segment took each requested view's status as
// firstNonEmpty(Summary["views"][view].status, bundle.status). A view the CCB
// disclosure budget trimmed has no entry in Summary["views"] at all (the budget
// drops it at capabilitycontext/free_state_observation.go:381-384 before it
// reaches the payload), so the fallback restated the bundle's own partial
// status for a view that was never disclosed — and gateG6's target binding
// (free_state_gate.go:235) then accepted a target-level observation that never
// arrived. Same "nominal hit vs actual delivery" family B13-A fixed on G3.
//
// B13-D fixes the write site, not the gate: a row whose view the disclosure
// budget trimmed is not recorded in the catalog. The verdict reuses B13-A's
// predicate verbatim (freeStateScanViewOmittedByDisclosureBudget) and stays
// mechanical and content-blind — structural "<view_id>: omitted by disclosure
// budget" matching only, no view content, track identity, processor or dose.
// Every pin below runs the real recording path, so the row under test is the
// row the runtime writes.

const (
	b13dTrackViewID    = "track.timbre_frequency"
	b13dBudgetReason   = b13dTrackViewID + ": omitted by disclosure budget"
	b13dTrackLedgerKey = "track:1007::" + b13dTrackViewID
)

// b13dTrackObservationSummary builds a track-scoped CCB observation summary in
// the B13-A shape: the caller controls the bundle status, the omission reasons
// and whether the requested view actually reached the payload.
func b13dTrackObservationSummary(status, observationID string, requested []any, reasons []any, views map[string]any, auditReasons []any) map[string]any {
	summary := map[string]any{
		"status":           status,
		"observation_id":   observationID,
		"requested_views":  requested,
		"project_revision": "rev-7",
		"freshness":        map[string]any{"status": "fresh", "project_revision": "rev-7"},
		"target_ref":       map[string]any{"kind": "track", "id": "1007"},
		"evidence_refs":    []any{observationID},
		"audit_receipt": map[string]any{
			"receipt_id": "ccbr_" + observationID, "schema_version": "ccb_observation_receipt.v1", "status": status,
		},
	}
	if len(views) > 0 {
		summary["views"] = views
	}
	if reasons != nil {
		summary["omission_reasons"] = reasons
	}
	if auditReasons != nil {
		messageLoopMapValue(summary["audit_receipt"])["rejection_reasons"] = auditReasons
	}
	return summary
}

// b13dWithoutTargetEvidence is the green control minus its target-level
// evidence (the TIMING-2 shape: the catalog row removed and the obs-target
// receipt made unusable). G6 fails here, so whatever the caller adds back is
// what G6's verdict rests on.
func b13dWithoutTargetEvidence(mutate func(ctx map[string]any)) *runState {
	return timing2WithoutMixScanReceipts(func(ctx map[string]any) {
		control := messageLoopMapValue(messageLoopMapValue(ctx["free_state_reasoning_loop"])["observation_ledger"])
		delete(messageLoopMapValue(control["available_views"]), b13dTrackLedgerKey)
		for _, row := range messageLoopMapRows(control["receipts"]) {
			if messageLoopText(row["observation_id"]) == "obs-target" {
				row["status"] = "rejected"
			}
		}
		if mutate != nil {
			mutate(ctx)
		}
	})
}

// b13dMergedObservationState records an observation through the real write
// segment (mergeFreeStateObservationLedger) and installs the resulting catalog
// and receipts on that control. G6's verdict is therefore decided by the merged
// rows alone, while G1/G2/G4/G5 and the frontier keep their green-control shape.
func b13dMergedObservationState(t *testing.T, summary map[string]any) (*runState, map[string]any) {
	t.Helper()
	ledger := map[string]any{"schema_version": freeStateObservationLedgerSchema}
	mergeFreeStateObservationLedger(ledger, &RecentObservation{ToolCallID: "call-b13d", Summary: summary}, 1)
	state := b13dWithoutTargetEvidence(nil)
	control := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	if merged := messageLoopMapRows(ledger["receipts"]); len(merged) > 0 {
		control["receipts"] = append(messageLoopMapRows(control["receipts"]), merged...)
	}
	for key, row := range messageLoopMapValue(ledger["available_views"]) {
		messageLoopMapValue(control["available_views"])[key] = row
	}
	return state, ledger
}

// ① RED: a view the disclosure budget trimmed must not become a usable
// target-level observation. Before B13-D the trimmed row landed with the
// bundle's partial status and satisfied G6's target binding.
func TestB13DTrimmedViewDoesNotBindG6Target(t *testing.T) {
	state, ledger := b13dMergedObservationState(t, b13dTrackObservationSummary(
		"partial", "obs-b13d-trimmed", []any{b13dTrackViewID}, []any{b13dBudgetReason}, nil, nil))

	if row := messageLoopMapValue(messageLoopMapValue(ledger["available_views"])[b13dTrackLedgerKey]); len(row) > 0 {
		t.Fatalf("a disclosure-budget-trimmed view was recorded as an available view: %#v", row)
	}
	if len(messageLoopMapValue(ledger["available_views"])) != 0 {
		t.Fatalf("the trimmed bundle wrote a catalog row: %#v", ledger["available_views"])
	}
	if gateG6(state) {
		t.Fatalf("G6 bound the candidate target to a view the disclosure budget trimmed: %#v",
			messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])["available_views"])
	}
	// The refusal surface the model sees names the missing target evidence.
	if !strings.Contains(messageLoopFreeStateGatePathDirective(state), "G6_target_evidence") {
		t.Fatalf("the G6 row must render while target-level delivery is missing: %q",
			messageLoopFreeStateGatePathDirective(state))
	}
	// The delivery fact still rides the receipt (B13-A/B13-C surfaces read it);
	// B13-D changed the catalog, not the audit row.
	receipt := messageLoopMapRows(ledger["receipts"])
	if len(receipt) != 1 {
		t.Fatalf("receipt rows = %d, want 1", len(receipt))
	}
	if got := freeStateReceiptFreshRevisionBound(receipt[0], "rev-7"); !got {
		t.Fatalf("the receipt row lost its revision binding: %#v", receipt[0])
	}
	if !freeStateScanViewOmittedByDisclosureBudget(receipt[0], b13dTrackViewID) {
		t.Fatalf("the receipt row must keep the trim fact: %#v", receipt[0])
	}

	// Control: the same recording path, same state shape, same requested view —
	// only the trim (marker + missing payload entry) removed — does bind the
	// target. So the failure above is the trim, not an unsatisfiable fixture.
	delivered, _ := b13dMergedObservationState(t, b13dTrackObservationSummary(
		"ready", "obs-b13d-delivered", []any{b13dTrackViewID}, nil,
		map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}}, nil))
	if !gateG6(delivered) {
		t.Fatal("control: a delivered target-level view must still bind the candidate target")
	}
}

// ② Zero regression: every untrimmed shape keeps landing exactly as before,
// including a bundle that trims some other view.
func TestB13DTrimmedViewDoesNotDisturbUntrimmedRows(t *testing.T) {
	cases := []struct {
		name      string
		summary   map[string]any
		wantRow   bool
		wantState string
	}{
		{
			name: "ready bundle with the view delivered",
			summary: b13dTrackObservationSummary("ready", "obs-b13d-ready", []any{b13dTrackViewID}, nil,
				map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantRow: true, wantState: "ready",
		},
		{
			name: "partial bundle with the view delivered as partial",
			summary: b13dTrackObservationSummary("partial", "obs-b13d-partial", []any{b13dTrackViewID}, nil,
				map[string]any{b13dTrackViewID: map[string]any{"status": "partial"}}, nil),
			wantRow: true, wantState: "partial",
		},
		{
			name: "partial bundle trimming another view",
			summary: b13dTrackObservationSummary("partial", "obs-b13d-other-trimmed",
				[]any{"project.structure", b13dTrackViewID},
				[]any{"project.structure: omitted by disclosure budget"},
				map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantRow: true, wantState: "ready",
		},
		{
			name: "partial bundle trimming another view through the audit receipt only",
			summary: b13dTrackObservationSummary("partial", "obs-b13d-audit-trimmed",
				[]any{"project.structure", b13dTrackViewID}, nil,
				map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}},
				[]any{"project.structure: omitted by disclosure budget"}),
			wantRow: true, wantState: "ready",
		},
		{
			name: "partial bundle omitting another view for a non-budget reason",
			summary: b13dTrackObservationSummary("partial", "obs-b13d-other-stale",
				[]any{"project.structure", b13dTrackViewID},
				[]any{"project.structure: source evidence is stale"},
				map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantRow: true, wantState: "ready",
		},
	}
	for _, tc := range cases {
		state, ledger := b13dMergedObservationState(t, tc.summary)
		row := messageLoopMapValue(messageLoopMapValue(ledger["available_views"])[b13dTrackLedgerKey])
		if tc.wantRow && len(row) == 0 {
			t.Fatalf("%s: the delivered view row is missing: %#v", tc.name, ledger["available_views"])
		}
		if got := firstMapText(row, "status"); !strings.EqualFold(got, tc.wantState) {
			t.Fatalf("%s: row status = %q, want %q (row %#v)", tc.name, got, tc.wantState, row)
		}
		for _, key := range []string{"view_id", "observation_id", "tool_call_id", "round", "target_ref"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("%s: delivered row lost key %q: %#v", tc.name, key, row)
			}
		}
		if !gateG6(state) {
			t.Fatalf("%s: a delivered target-level view must keep satisfying G6", tc.name)
		}
		// The trimmed sibling never gains a row of its own.
		if _, ok := messageLoopMapValue(ledger["available_views"])["project.structure"]; ok {
			t.Fatalf("%s: the trimmed sibling view gained a catalog row: %#v", tc.name, ledger["available_views"])
		}
	}

	// The standing G6 fixtures keep their verdicts.
	if !gateG6(gateTestState(nil)) {
		t.Fatal("the shared green gate control must keep passing G6")
	}
	if gateG6(b13dWithoutTargetEvidence(nil)) {
		t.Fatal("control: the target-evidence-stripped state must keep failing G6")
	}
}

// ③ The fully-cut shape (B13-C precedent): when the budget drops every
// requested view the bundle itself reports insufficient, and the trim verdict
// is still read on any status — requiring ready|partial would lose exactly this
// shape. No catalog row is written, and the receipt keeps the delivery fact.
func TestB13DInsufficientBundleTrimmedVerdictAndNoRows(t *testing.T) {
	summary := b13dTrackObservationSummary("insufficient", "obs-b13d-insufficient",
		[]any{"project.structure", b13dTrackViewID},
		[]any{"project.structure: omitted by disclosure budget", b13dBudgetReason}, nil, nil)
	if !freeStateViewOmittedByDisclosureBudget(summary, b13dTrackViewID) {
		t.Fatal("the trim verdict must not require a ready|partial bundle status")
	}
	// Control on the same status: a non-budget omission is not a trim.
	other := b13dTrackObservationSummary("insufficient", "obs-b13d-insufficient-stale",
		[]any{b13dTrackViewID}, []any{b13dTrackViewID + ": source evidence is stale"}, nil, nil)
	if freeStateViewOmittedByDisclosureBudget(other, b13dTrackViewID) {
		t.Fatal("a non-budget omission must not be classified as a disclosure-budget trim")
	}

	state, ledger := b13dMergedObservationState(t, summary)
	if len(messageLoopMapValue(ledger["available_views"])) != 0 {
		t.Fatalf("an all-trimmed insufficient bundle wrote catalog rows: %#v", ledger["available_views"])
	}
	if gateG6(state) {
		t.Fatal("G6 must not bind a target no view was ever delivered for")
	}
	if rows := messageLoopMapRows(ledger["receipts"]); len(rows) != 1 || !freeStateScanViewOmittedByDisclosureBudget(rows[0], b13dTrackViewID) {
		t.Fatalf("the receipt must keep the trim fact for the G3/reply surfaces: %#v", rows)
	}
}

// ④ Boundary the fix deliberately does not cross (B13-A's ruling kept verbatim:
// a non-budget marker never disqualifies). A requested view omitted for source
// evidence reasons is not a disclosure-budget trim, so the row keeps landing
// exactly as it did before B13-D. Pinned so a future widening is a deliberate,
// visible change rather than a silent drift.
func TestB13DNonBudgetOmissionIsNotATrim(t *testing.T) {
	state, ledger := b13dMergedObservationState(t, b13dTrackObservationSummary(
		"partial", "obs-b13d-non-budget", []any{b13dTrackViewID},
		[]any{b13dTrackViewID + ": source evidence is stale"}, nil, nil))
	row := messageLoopMapValue(messageLoopMapValue(ledger["available_views"])[b13dTrackLedgerKey])
	if len(row) == 0 {
		t.Fatalf("a non-budget omission is out of B13-D's scope and must keep its prior row shape: %#v", ledger["available_views"])
	}
	if got := firstMapText(row, "status"); !strings.EqualFold(got, "partial") {
		t.Fatalf("non-budget omission row status = %q, want the unchanged partial fallback", got)
	}
	if !gateG6(state) {
		t.Fatal("control: the non-budget shape keeps its pre-B13-D G6 verdict")
	}
}

// ⑤ A trim is not a delivery and cannot retract one: the delivered round keeps
// its own row (identity, status, round) and the counter does not advance for
// the trimmed round.
func TestB13DTrimCannotRetractOrForgeTargetDelivery(t *testing.T) {
	ledger := map[string]any{"schema_version": freeStateObservationLedgerSchema}
	mergeFreeStateObservationLedger(ledger, &RecentObservation{ToolCallID: "call-b13d-r1", Summary: b13dTrackObservationSummary(
		"ready", "obs-b13d-r1", []any{b13dTrackViewID}, nil,
		map[string]any{b13dTrackViewID: map[string]any{"status": "ready"}}, nil)}, 1)
	mergeFreeStateObservationLedger(ledger, &RecentObservation{ToolCallID: "call-b13d-r2", Summary: b13dTrackObservationSummary(
		"partial", "obs-b13d-r2", []any{b13dTrackViewID}, []any{b13dBudgetReason}, nil, nil)}, 2)

	row := messageLoopMapValue(messageLoopMapValue(ledger["available_views"])[b13dTrackLedgerKey])
	if got := firstMapText(row, "observation_id"); got != "obs-b13d-r1" {
		t.Fatalf("the trimmed round replaced a real delivery: observation_id = %q (%#v)", got, row)
	}
	if got := firstMapText(row, "status"); !strings.EqualFold(got, "ready") {
		t.Fatalf("the trimmed round rewrote the delivered row's status: %q", got)
	}
	if got := freeStateLedgerCount(row["round"], 0); got != 1 {
		t.Fatalf("delivered row round = %d, want 1", got)
	}
	if got := freeStateLedgerCount(ledger["view_observation_count"], 0); got != 1 {
		t.Fatalf("view_observation_count = %d, want 1 (the trimmed round is not a delivery)", got)
	}
}

// ⑥ The predicate itself stays mechanical and content-blind, and reads the same
// delivery-fact source the ledger receipt binds (B13-A): the bundle's
// omission_reasons, falling back to the audit receipt's rejection_reasons.
func TestB13DTrimPredicateIsContentBlindAndSourceConsistent(t *testing.T) {
	cases := []struct {
		name    string
		summary map[string]any
		viewID  string
		want    bool
	}{
		{"marker naming this view", b13dTrackObservationSummary("partial", "o1", []any{b13dTrackViewID}, []any{b13dBudgetReason}, nil, nil), b13dTrackViewID, true},
		{"marker naming another view", b13dTrackObservationSummary("partial", "o2", []any{b13dTrackViewID}, []any{"project.structure: omitted by disclosure budget"}, nil, nil), b13dTrackViewID, false},
		{"marker only in the audit receipt", b13dTrackObservationSummary("partial", "o3", []any{b13dTrackViewID}, nil, nil, []any{b13dBudgetReason}), b13dTrackViewID, true},
		{"same view omitted for a non-budget reason", b13dTrackObservationSummary("partial", "o4", []any{b13dTrackViewID}, []any{b13dTrackViewID + ": source evidence is missing"}, nil, nil), b13dTrackViewID, false},
		{"owner case-insensitive", b13dTrackObservationSummary("partial", "o5", []any{b13dTrackViewID}, []any{"TRACK.TIMBRE_FREQUENCY: omitted by disclosure budget"}, nil, nil), b13dTrackViewID, true},
		{"no colon in the reason", b13dTrackObservationSummary("partial", "o6", []any{b13dTrackViewID}, []any{"omitted by disclosure budget"}, nil, nil), b13dTrackViewID, false},
		{"empty view id", b13dTrackObservationSummary("partial", "o7", []any{b13dTrackViewID}, []any{b13dBudgetReason}, nil, nil), "", false},
		{"no reasons at all", b13dTrackObservationSummary("ready", "o8", []any{b13dTrackViewID}, nil, nil, nil), b13dTrackViewID, false},
	}
	for _, tc := range cases {
		if got := freeStateViewOmittedByDisclosureBudget(tc.summary, tc.viewID); got != tc.want {
			t.Fatalf("%s: predicate = %v, want %v", tc.name, got, tc.want)
		}
	}
}
