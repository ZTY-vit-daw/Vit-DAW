package chat

import (
	"strconv"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
)

// B13-C (2026-09-12): the empty-frontier settlement reply must separate two
// mechanically distinct situations.
//
// B13 forensics (queue/reports/2026-09-12-B13-frontier-exhaustion.md §3/§4.3):
// the freshstack empty frontier was a SUPPLY defect — the only candidate
// producing scan view was dropped by the CCB disclosure budget
// (rejection_reasons=["mix.frequency_relationship: omitted by disclosure
// budget"]) and was never re-requested. The 235800 shape was the genuinely
// empty one: the scan view was delivered with conflict_candidates=[]. The old
// ③ wording ("this project has no hypothesis left to verify") is dishonest for
// the first shape, because it packages a supply defect as a product conclusion.
//
// The branch signal is the same predicate B13-A pinned in agentloop
// (free_state_gate.go, freeStateScanViewOmittedByDisclosureBudget): a
// structural "<view_id>: <marker>" match over the observation ledger receipt's
// rejection_reasons. Nothing here reads view content, a track identity, a
// processor or a dose.

const b13cDisclosureBudgetReason = "mix.frequency_relationship: omitted by disclosure budget"

func b13cNoCandidateSettlement() *audioclosure.Settlement {
	return &audioclosure.Settlement{Reason: audioclosure.StopNoCandidateFound}
}

// b13cLedger runs the real recording path (mergeFreeStateObservationLedger) over
// CCB bundle summaries, so every assertion below judges the ledger row the
// runtime actually writes rather than a hand-built one.
func b13cLedger(summaries ...map[string]any) map[string]any {
	ledger := map[string]any{"schema_version": freeStateObservationLedgerSchema}
	for index, summary := range summaries {
		ledger = mergeFreeStateObservationLedger(ledger, &agentloop.RecentObservation{
			Tool: "ccb.observation_request", ToolCallID: "call-b13c-" + strconv.Itoa(index), Summary: summary,
		}, index+1)
	}
	return ledger
}

func b13cScanSummary(status string, requested []any, omissionReasons []any) map[string]any {
	summary := map[string]any{
		"status": status, "observation_id": "obs-b13c-" + status, "requested_views": requested,
		"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
	}
	if omissionReasons != nil {
		summary["omission_reasons"] = omissionReasons
	}
	return summary
}

// ① RED: the trimmed supply reaches the ledger and produces the supply-trimmed
// wording. Before B13-C the ledger row carried no delivery fact at all
// (freeStateObservationCompactReceipt dropped it, exactly as B13-A found on the
// agentloop side) and the reply was the generic one.
func TestB13CTrimmedScanViewReplyNamesDisclosureBudget(t *testing.T) {
	ledger := b13cLedger(b13cScanSummary("partial",
		[]any{"project.structure", "mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}))
	supply := freeStateCandidateScanSupply(ledger)
	if !supply.Trimmed || supply.Delivered {
		t.Fatalf("supply classification missed the disclosure-budget trim: supply=%+v ledger=%v", supply, ledger)
	}
	reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 6, supply)
	for _, want := range []string{"被披露预算裁剪", "未能建立候选", "建议重新观察"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("trimmed-supply reply must contain %q: %q", want, reply)
		}
	}
	if strings.Contains(reply, "暂无证据支持的待验证假设") {
		t.Fatalf("a trimmed supply must not be reported as a project without hypotheses: %q", reply)
	}
}

// ② RED: the delivered supply produces the genuinely-empty wording.
func TestB13CDeliveredScanViewReplyNamesNoEvidenceBackedHypothesis(t *testing.T) {
	ledger := b13cLedger(b13cScanSummary("ready",
		[]any{"mix.frequency_relationship"}, nil))
	supply := freeStateCandidateScanSupply(ledger)
	if !supply.Delivered || supply.Trimmed {
		t.Fatalf("supply classification missed the delivered scan view: supply=%+v ledger=%v", supply, ledger)
	}
	reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 6, supply)
	for _, want := range []string{"暂无证据支持的待验证假设", "建议换个方向或换轨"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("delivered-supply reply must contain %q: %q", want, reply)
		}
	}
	if strings.Contains(reply, "被披露预算裁剪") {
		t.Fatalf("a delivered supply must not be reported as a budget trim: %q", reply)
	}
}

// ③ The delivery fact must survive the production recording path: the compact
// bundle summary carries omission_reasons (agentloop/message_loop.go
// messageLoopCCBObservationSummary selects it, not audit_receipt), so the ledger
// receipt has to keep it. This is the B13-C half of B13-A's data-path finding.
func TestB13CLedgerReceiptCarriesDisclosureBudgetFact(t *testing.T) {
	ledger := b13cLedger(b13cScanSummary("partial",
		[]any{"project.structure", "mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}))
	rows := freeStateMapRows(ledger["receipts"])
	if len(rows) == 0 {
		t.Fatalf("the recording path wrote no receipt row: %v", ledger)
	}
	reasons := freeStateStringSlice(rows[0]["rejection_reasons"])
	if len(reasons) != 1 || reasons[0] != b13cDisclosureBudgetReason {
		t.Fatalf("ledger receipt dropped the disclosure-budget fact: %v", rows[0])
	}

	// An observation with no omission keeps the previous row shape: the fact
	// field is absent rather than empty.
	clean := b13cLedger(b13cScanSummary("ready", []any{"mix.frequency_relationship"}, nil))
	cleanRows := freeStateMapRows(clean["receipts"])
	if len(cleanRows) == 0 {
		t.Fatalf("the recording path wrote no receipt row for the clean bundle: %v", clean)
	}
	if _, present := cleanRows[0]["rejection_reasons"]; present {
		t.Fatalf("an untrimmed receipt must keep its previous shape: %v", cleanRows[0])
	}
}

// ④ Content-blind parity with B13-A: only the disclosure-budget marker counts
// as a trim. Any other omission is neither a trim nor a delivery.
func TestB13CScanSupplyIgnoresNonBudgetOmissions(t *testing.T) {
	for _, reason := range []string{
		"mix.frequency_relationship: source evidence is stale",
		"mix.frequency_relationship: source evidence is missing",
		"mix.frequency_relationship: source evidence is deferred",
		"mix.frequency_relationship: view is unavailable",
	} {
		supply := freeStateCandidateScanSupply(
			b13cLedger(b13cScanSummary("partial", []any{"mix.frequency_relationship"}, []any{reason})))
		if supply.Trimmed || supply.Delivered {
			t.Fatalf("%q is neither a disclosure-budget trim nor a delivery: %+v", reason, supply)
		}
		reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 6, supply)
		if strings.Contains(reply, "被披露预算裁剪") || strings.Contains(reply, "暂无证据支持的待验证假设") {
			t.Fatalf("%q must keep the pre-existing wording: %q", reason, reply)
		}
	}

	// A trim on a view that cannot produce candidates is not a supply fact.
	nonCandidate := freeStateCandidateScanSupply(
		b13cLedger(b13cScanSummary("partial", []any{"project.structure"},
			[]any{"project.structure: omitted by disclosure budget"})))
	if nonCandidate.Trimmed || nonCandidate.Delivered {
		t.Fatalf("a non-candidate view cannot carry the supply verdict: %+v", nonCandidate)
	}
}

// ④b Real-stack shape (run 20260912_142240, turn 2 ledger): a request whose
// disclosure budget dropped every view it asked for comes back with
// status=insufficient, not partial — capabilitycontext marks a bundle
// insufficient as soon as it discloses zero views. That fully-cut receipt is
// exactly the supply-trim fact and must not be lost for failing a ready|partial
// test.
func TestB13CScanSupplyReadsFullyTrimmedInsufficientReceipt(t *testing.T) {
	ledger := b13cLedger(
		b13cScanSummary("insufficient", []any{"mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}),
	)
	supply := freeStateCandidateScanSupply(ledger)
	if !supply.Trimmed {
		t.Fatalf("a fully trimmed scan request must count as a trim: supply=%+v ledger=%v", supply, ledger)
	}
	// The live shape also carries a second request that DID disclose the other
	// scan view; the trim still owns the frequency view's verdict, and the reply
	// reports the cut rather than the project conclusion.
	ledger = b13cLedger(
		b13cScanSummary("insufficient", []any{"mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}),
		b13cScanSummary("ready", []any{"mix.multitrack_relationship", "track.basic_energy"}, nil),
	)
	supply = freeStateCandidateScanSupply(ledger)
	if !supply.Trimmed || !supply.Delivered {
		t.Fatalf("the live mixed shape must keep both facts: supply=%+v", supply)
	}
	reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 6, supply)
	if !strings.Contains(reply, "被披露预算裁剪") {
		t.Fatalf("a cut scan view must outrank the delivered one in the reply: %q", reply)
	}
}

// ⑤ Receipt order decides: the last usable receipt that asked for the view owns
// its delivery fact.
func TestB13CScanSupplyFollowsLatestDeliveryFact(t *testing.T) {
	recovered := freeStateCandidateScanSupply(b13cLedger(
		b13cScanSummary("partial", []any{"mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}),
		b13cScanSummary("ready", []any{"mix.frequency_relationship"}, nil),
	))
	if !recovered.Delivered || recovered.Trimmed {
		t.Fatalf("a later delivered scan view must clear the earlier trim: %+v", recovered)
	}

	trimmedLate := freeStateCandidateScanSupply(b13cLedger(
		b13cScanSummary("ready", []any{"mix.frequency_relationship"}, nil),
		b13cScanSummary("partial", []any{"mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason}),
	))
	if !trimmedLate.Trimmed || trimmedLate.Delivered {
		t.Fatalf("a later trim must own the view's delivery fact: %+v", trimmedLate)
	}
}

// ⑥ Zero regression on the existing degradation replies: the AGENT-W1
// single-track wording, the multi-track wording when the ledger carries no
// supply fact, and every other settlement reason.
func TestB13CExistingSettlementRepliesUnchanged(t *testing.T) {
	singleTrack := audioClosureSettlementReply(b13cNoCandidateSettlement(), 1, freeStateCandidateSupply{})
	if !strings.Contains(singleTrack, "只有 1 轨") || !strings.Contains(singleTrack, "没有可执行的改善建议") {
		t.Fatalf("AGENT-W1 single-track wording changed: %q", singleTrack)
	}
	for _, count := range []int{6, -1, 0} {
		reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), count, freeStateCandidateSupply{})
		if !strings.Contains(reply, "在已声明的观察范围内没有发现可信改善候选") {
			t.Fatalf("track count %d must keep the pre-existing wording: %q", count, reply)
		}
	}
	// A trim still outranks the project-shape wording: with the candidate view
	// cut, the single-track claim is no longer supported by the observation set.
	if reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 1, freeStateCandidateSupply{Trimmed: true}); !strings.Contains(reply, "被披露预算裁剪") {
		t.Fatalf("the trimmed supply must outrank the single-track wording: %q", reply)
	}

	cases := []struct {
		reason audioclosure.StopReason
		want   string
	}{
		{audioclosure.StopSatisfied, "已经完成闭环并通过结论检查"},
		{audioclosure.StopDiagnosticComplete, "只读声学诊断已经完成"},
		{audioclosure.StopCapabilityBlocked, "明确的能力边界"},
		{audioclosure.StopEvidenceCeilingReached, "唯一观察上限"},
		{audioclosure.StopNoProgress, "连续两轮没有获得新的有效证据"},
	}
	for _, tc := range cases {
		reply := audioClosureSettlementReply(&audioclosure.Settlement{Reason: tc.reason}, 1,
			freeStateCandidateSupply{Delivered: true, Trimmed: true})
		if !strings.Contains(reply, tc.want) {
			t.Fatalf("settlement %s wording changed: %q", tc.reason, reply)
		}
	}
}

// ⑦ The server reads the verdict off the conversation's durable ledger.
func TestB13CSettlementSupplyReadsConversationLedger(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-b13c", ConversationID: "conversation-b13c",
		Status: "no_candidate_found", OriginalIntent: "improve the mix", MaxCycles: 6,
		ObservationLedger: b13cLedger(b13cScanSummary("partial",
			[]any{"mix.frequency_relationship"}, []any{b13cDisclosureBudgetReason})),
	})
	if supply := server.freeStateCandidateSupplyForSettlement("conversation-b13c"); !supply.Trimmed {
		t.Fatalf("settlement supply did not read the durable ledger: %+v", supply)
	}
	if supply := server.freeStateCandidateSupplyForSettlement("conversation-absent"); supply.Trimmed || supply.Delivered {
		t.Fatalf("an unknown conversation must yield no supply fact: %+v", supply)
	}
}

// ⑧ Wording discipline: the user-facing reply carries no internal view id, no
// gate id and no FS/FAM code.
func TestB13CReplyWordingCarriesNoInternalCode(t *testing.T) {
	banned := []string{
		"mix.", "track.", "project.", "candidate", "frontier", "disclosure budget",
		"G1_", "G2_", "G3_", "G4_", "G5_", "G6_", "G7_", "G8_",
		"FAM", "FS0", "FS1", "FS2", "FS3", "FS4", "FS5", "FS6", "FS7", "FS8", "FS9",
		"no_frontier_candidates", "omitted by disclosure budget",
	}
	for _, supply := range []freeStateCandidateSupply{{Trimmed: true}, {Delivered: true}} {
		reply := audioClosureSettlementReply(b13cNoCandidateSettlement(), 6, supply)
		for _, token := range banned {
			if strings.Contains(reply, token) {
				t.Fatalf("reply leaks the internal token %q: %q", token, reply)
			}
		}
	}
}
