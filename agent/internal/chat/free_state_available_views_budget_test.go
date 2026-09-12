package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
)

// B13-D 扩展面 (2026-09-12, 决策侧批准扩域): the chat write segment.
//
// agentloop's available_views write segment was fixed first, but it is only the
// in-flight projection of free_state_reasoning_loop.observation_ledger. This
// package owns the authoritative persistence of that same ledger
// (mergeFreeStateObservationLedger, called at the HTTP result boundary, after
// the projection), and it carried the identical defect: a view the disclosure
// budget trimmed has no entry in Summary["views"], so
// firstNonEmpty(view.status, bundle.status) restated the bundle's own partial
// status for a view that was never disclosed. Two real-stack runs (E2E bed
// 20260912_150847 / 20260912_151521) show the consequence with the fix in the
// other segment: the catalog still carried a row whose observation_id is
// byte-identical to the receipt that reports the same view as
// "omitted by disclosure budget" — so the G6 target binding could still be
// satisfied by a view that never arrived.
//
// The guard below mirrors agentloop's (same predicate verdict, same
// status-independent reading, same "a trim is not a delivery and cannot retract
// one" rule). It stays mechanical and content-blind: structural
// "<view_id>: omitted by disclosure budget" matching only.

const (
	b13dChatTrackViewID    = "track.timbre_frequency"
	b13dChatBudgetReason   = b13dChatTrackViewID + ": omitted by disclosure budget"
	b13dChatTrackLedgerKey = "track:1007::" + b13dChatTrackViewID
	b13dChatGateG6         = "G6_target_evidence"
)

// b13dChatObservationSummary builds a track-scoped CCB observation summary in
// the B13-A shape: the caller controls the bundle status, the omission reasons,
// whether the requested view reached the payload, and whether the trim fact
// rides only the audit receipt.
func b13dChatObservationSummary(status, observationID string, requested []any, reasons []any, views map[string]any, auditReasons []any) map[string]any {
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
		firstMapFromAny(summary["audit_receipt"])["rejection_reasons"] = auditReasons
	}
	return summary
}

// b13dChatMergeLedger runs the real chat write segment over one or more
// observation summaries, in order, and returns the ledger it produced.
func b13dChatMergeLedger(summaries ...map[string]any) map[string]any {
	ledger := map[string]any{"schema_version": freeStateObservationLedgerSchema}
	for index, summary := range summaries {
		ledger = mergeFreeStateObservationLedger(ledger, &agentloop.RecentObservation{
			ToolCallID: "call-b13d-" + string(rune('1'+index)), Tool: "ccb.observation_request",
			CommandName: "ccb_observation_request", Status: "ok", Summary: summary,
		}, index+1)
	}
	return ledger
}

// b13dChatGateContext is the green G1-G8 control context carrying exactly the
// ledger the chat write segment produced, so the G6 verdict rests on that
// catalog alone (no pre-seeded target-level observation, no recentObservation).
func b13dChatGateContext(ledger map[string]any) map[string]any {
	return map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-7",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version":      "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "improve the mix", "observation_ledger": ledger,
		},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7",
			"hypothesis_frontier": map[string]any{
				"candidate_id": "candidate-1",
				"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
			},
		},
	}
}

// b13dChatG6Failed reports whether the exported gate audit refuses G6 for this
// ledger.
func b13dChatG6Failed(ledger map[string]any) bool {
	for _, gateID := range agentloop.AuditFreeStateNeedsExperimentGate(b13dChatGateContext(ledger), nil).FailedGateIDs {
		if gateID == b13dChatGateG6 {
			return true
		}
	}
	return false
}

// ① RED: a view the disclosure budget trimmed must not enter the catalog this
// package persists, and must not satisfy G6's target binding. Before the
// extension the trimmed row landed with the bundle's partial status.
func TestB13DChatTrimmedViewDoesNotBindG6Target(t *testing.T) {
	ledger := b13dChatMergeLedger(b13dChatObservationSummary(
		"partial", "obs-b13d-chat-trimmed", []any{b13dChatTrackViewID}, []any{b13dChatBudgetReason}, nil, nil))

	available := firstMapFromAny(ledger["available_views"])
	if row := firstMapFromAny(available[b13dChatTrackLedgerKey]); len(row) > 0 {
		t.Fatalf("a disclosure-budget-trimmed view was recorded as an available view: %#v", row)
	}
	if len(available) != 0 {
		t.Fatalf("the trimmed bundle wrote a catalog row: %#v", available)
	}
	if !b13dChatG6Failed(ledger) {
		t.Fatalf("G6 bound the candidate target to a view the disclosure budget trimmed: %#v", available)
	}
	// The delivery fact still rides the receipt that the same merge wrote, and
	// the two surfaces cannot disagree: marker on the receipt <=> no row.
	receipts := freeStateMapRows(ledger["receipts"])
	if len(receipts) != 1 {
		t.Fatalf("receipt rows = %d, want 1", len(receipts))
	}
	if !freeStateScanViewOmittedByDisclosureBudget(receipts[0], b13dChatTrackViewID) {
		t.Fatalf("the receipt row must keep the trim fact: %#v", receipts[0])
	}
	if got := firstStringFromMap(receipts[0], "observation_id"); got != "obs-b13d-chat-trimmed" {
		t.Fatalf("receipt observation_id = %q", got)
	}

	// Control: the same write path and the same context shape, marker removed
	// and the view actually present in the payload, does bind the target — so
	// the refusal above is the trim, not an unsatisfiable fixture.
	delivered := b13dChatMergeLedger(b13dChatObservationSummary(
		"ready", "obs-b13d-chat-delivered", []any{b13dChatTrackViewID}, nil,
		map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}}, nil))
	if b13dChatG6Failed(delivered) {
		t.Fatal("control: a delivered target-level view must still bind the candidate target")
	}
	if row := firstMapFromAny(firstMapFromAny(delivered["available_views"])[b13dChatTrackLedgerKey]); len(row) == 0 {
		t.Fatalf("control: the delivered row is missing: %#v", delivered["available_views"])
	}
}

// ② Zero regression: every untrimmed shape keeps landing exactly as before.
func TestB13DChatTrimmedViewDoesNotDisturbUntrimmedRows(t *testing.T) {
	cases := []struct {
		name      string
		summary   map[string]any
		wantState string
	}{
		{
			name: "ready bundle with the view delivered",
			summary: b13dChatObservationSummary("ready", "obs-b13d-chat-ready", []any{b13dChatTrackViewID}, nil,
				map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantState: "ready",
		},
		{
			name: "partial bundle with the view delivered as partial",
			summary: b13dChatObservationSummary("partial", "obs-b13d-chat-partial", []any{b13dChatTrackViewID}, nil,
				map[string]any{b13dChatTrackViewID: map[string]any{"status": "partial"}}, nil),
			wantState: "partial",
		},
		{
			name: "partial bundle trimming another view",
			summary: b13dChatObservationSummary("partial", "obs-b13d-chat-other-trimmed",
				[]any{"project.structure", b13dChatTrackViewID},
				[]any{"project.structure: omitted by disclosure budget"},
				map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantState: "ready",
		},
		{
			name: "partial bundle trimming another view through the audit receipt only",
			summary: b13dChatObservationSummary("partial", "obs-b13d-chat-audit-trimmed",
				[]any{"project.structure", b13dChatTrackViewID}, nil,
				map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}},
				[]any{"project.structure: omitted by disclosure budget"}),
			wantState: "ready",
		},
		{
			name: "partial bundle omitting another view for a non-budget reason",
			summary: b13dChatObservationSummary("partial", "obs-b13d-chat-other-stale",
				[]any{"project.structure", b13dChatTrackViewID},
				[]any{"project.structure: source evidence is stale"},
				map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}}, nil),
			wantState: "ready",
		},
	}
	for _, tc := range cases {
		ledger := b13dChatMergeLedger(tc.summary)
		available := firstMapFromAny(ledger["available_views"])
		row := firstMapFromAny(available[b13dChatTrackLedgerKey])
		if len(row) == 0 {
			t.Fatalf("%s: the delivered view row is missing: %#v", tc.name, available)
		}
		if got := firstStringFromMap(row, "status"); !strings.EqualFold(got, tc.wantState) {
			t.Fatalf("%s: row status = %q, want %q (row %#v)", tc.name, got, tc.wantState, row)
		}
		for _, key := range []string{"view_id", "observation_id", "tool_call_id", "round", "target_ref"} {
			if _, ok := row[key]; !ok {
				t.Fatalf("%s: delivered row lost key %q: %#v", tc.name, key, row)
			}
		}
		if b13dChatG6Failed(ledger) {
			t.Fatalf("%s: a delivered target-level view must keep satisfying G6", tc.name)
		}
		if _, ok := available["project.structure"]; ok {
			t.Fatalf("%s: the trimmed sibling view gained a catalog row: %#v", tc.name, available)
		}
	}
}

// ③ The fully-cut shape (B13-C precedent): the trim verdict is read on any
// status, and an all-trimmed insufficient bundle writes no catalog row at all.
func TestB13DChatInsufficientBundleTrimmedVerdictAndNoRows(t *testing.T) {
	summary := b13dChatObservationSummary("insufficient", "obs-b13d-chat-insufficient",
		[]any{"project.structure", b13dChatTrackViewID},
		[]any{"project.structure: omitted by disclosure budget", b13dChatBudgetReason}, nil, nil)
	if !freeStateViewOmittedByDisclosureBudget(summary, b13dChatTrackViewID) {
		t.Fatal("the trim verdict must not require a ready|partial bundle status")
	}
	other := b13dChatObservationSummary("insufficient", "obs-b13d-chat-insufficient-stale",
		[]any{b13dChatTrackViewID}, []any{b13dChatTrackViewID + ": source evidence is stale"}, nil, nil)
	if freeStateViewOmittedByDisclosureBudget(other, b13dChatTrackViewID) {
		t.Fatal("a non-budget omission must not be classified as a disclosure-budget trim")
	}

	ledger := b13dChatMergeLedger(summary)
	if len(firstMapFromAny(ledger["available_views"])) != 0 {
		t.Fatalf("an all-trimmed insufficient bundle wrote catalog rows: %#v", ledger["available_views"])
	}
	if !b13dChatG6Failed(ledger) {
		t.Fatal("G6 must not bind a target no view was ever delivered for")
	}
	if rows := freeStateMapRows(ledger["receipts"]); len(rows) != 1 || !freeStateScanViewOmittedByDisclosureBudget(rows[0], b13dChatTrackViewID) {
		t.Fatalf("the receipt must keep the trim fact for the G3/reply surfaces: %#v", rows)
	}
}

// ④ Boundary the extension deliberately does not cross (B13-A's ruling kept
// verbatim): a non-budget marker never disqualifies. Pinned so a future
// widening is a deliberate, visible change.
func TestB13DChatNonBudgetOmissionIsNotATrim(t *testing.T) {
	ledger := b13dChatMergeLedger(b13dChatObservationSummary(
		"partial", "obs-b13d-chat-non-budget", []any{b13dChatTrackViewID},
		[]any{b13dChatTrackViewID + ": source evidence is stale"}, nil, nil))
	row := firstMapFromAny(firstMapFromAny(ledger["available_views"])[b13dChatTrackLedgerKey])
	if len(row) == 0 {
		t.Fatalf("a non-budget omission is out of B13-D's scope and must keep its prior row shape: %#v", ledger["available_views"])
	}
	if got := firstStringFromMap(row, "status"); !strings.EqualFold(got, "partial") {
		t.Fatalf("non-budget omission row status = %q, want the unchanged partial fallback", got)
	}
}

// ⑤ A trim is not a delivery and cannot retract one: the delivered round keeps
// its own row identity and the counter does not advance for the trimmed round.
func TestB13DChatTrimCannotRetractOrForgeTargetDelivery(t *testing.T) {
	ledger := b13dChatMergeLedger(
		b13dChatObservationSummary("ready", "obs-b13d-chat-r1", []any{b13dChatTrackViewID}, nil,
			map[string]any{b13dChatTrackViewID: map[string]any{"status": "ready"}}, nil),
		b13dChatObservationSummary("partial", "obs-b13d-chat-r2", []any{b13dChatTrackViewID}, []any{b13dChatBudgetReason}, nil, nil),
	)
	row := firstMapFromAny(firstMapFromAny(ledger["available_views"])[b13dChatTrackLedgerKey])
	if got := firstStringFromMap(row, "observation_id"); got != "obs-b13d-chat-r1" {
		t.Fatalf("the trimmed round replaced a real delivery: observation_id = %q (%#v)", got, row)
	}
	if got := firstStringFromMap(row, "status"); !strings.EqualFold(got, "ready") {
		t.Fatalf("the trimmed round rewrote the delivered row's status: %q", got)
	}
	if got := freeStateLedgerCount(row["round"], 0); got != 1 {
		t.Fatalf("delivered row round = %d, want 1", got)
	}
	if got := freeStateLedgerCount(ledger["view_observation_count"], 0); got != 1 {
		t.Fatalf("view_observation_count = %d, want 1 (the trimmed round is not a delivery)", got)
	}
}

// ⑥ The predicate reads the same delivery-fact source the compact receipt binds
// (B13-A/B13-C), so a receipt that reports the trim fact always implies the
// catalog row is suppressed — the two surfaces cannot disagree about one trim.
func TestB13DChatTrimPredicateReadsTheReceiptSource(t *testing.T) {
	cases := []struct {
		name    string
		summary map[string]any
		viewID  string
		want    bool
	}{
		{"marker naming this view", b13dChatObservationSummary("partial", "c1", []any{b13dChatTrackViewID}, []any{b13dChatBudgetReason}, nil, nil), b13dChatTrackViewID, true},
		{"marker naming another view", b13dChatObservationSummary("partial", "c2", []any{b13dChatTrackViewID}, []any{"project.structure: omitted by disclosure budget"}, nil, nil), b13dChatTrackViewID, false},
		{"marker only in the audit receipt", b13dChatObservationSummary("partial", "c3", []any{b13dChatTrackViewID}, nil, nil, []any{b13dChatBudgetReason}), b13dChatTrackViewID, true},
		{"same view omitted for a non-budget reason", b13dChatObservationSummary("partial", "c4", []any{b13dChatTrackViewID}, []any{b13dChatTrackViewID + ": source evidence is missing"}, nil, nil), b13dChatTrackViewID, false},
		{"owner case-insensitive", b13dChatObservationSummary("partial", "c5", []any{b13dChatTrackViewID}, []any{"TRACK.TIMBRE_FREQUENCY: omitted by disclosure budget"}, nil, nil), b13dChatTrackViewID, true},
		{"no colon in the reason", b13dChatObservationSummary("partial", "c6", []any{b13dChatTrackViewID}, []any{"omitted by disclosure budget"}, nil, nil), b13dChatTrackViewID, false},
		{"empty view id", b13dChatObservationSummary("partial", "c7", []any{b13dChatTrackViewID}, []any{b13dChatBudgetReason}, nil, nil), "", false},
		{"no reasons at all", b13dChatObservationSummary("ready", "c8", []any{b13dChatTrackViewID}, nil, nil, nil), b13dChatTrackViewID, false},
	}
	for _, tc := range cases {
		if got := freeStateViewOmittedByDisclosureBudget(tc.summary, tc.viewID); got != tc.want {
			t.Fatalf("%s: predicate = %v, want %v", tc.name, got, tc.want)
		}
	}

	// The audit-receipt-only shape: the receipt the merge writes carries the
	// marker and the catalog carries no row for the same view.
	ledger := b13dChatMergeLedger(b13dChatObservationSummary(
		"partial", "obs-b13d-chat-audit-only", []any{b13dChatTrackViewID}, nil, nil, []any{b13dChatBudgetReason}))
	if _, ok := firstMapFromAny(ledger["available_views"])[b13dChatTrackLedgerKey]; ok {
		t.Fatalf("audit-receipt-only trim still produced a catalog row: %#v", ledger["available_views"])
	}
	rows := freeStateMapRows(ledger["receipts"])
	if len(rows) != 1 || !freeStateScanViewOmittedByDisclosureBudget(rows[0], b13dChatTrackViewID) {
		t.Fatalf("audit-receipt-only trim must keep the receipt fact: %#v", rows)
	}
}
