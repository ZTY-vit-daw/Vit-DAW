package mixboard

import (
	"testing"
	"time"

	"vit-daw-agent/internal/orchestration"
)

func TestProjectDecisionBoardPropagatesRelevantCapabilityEffectsOnly(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	base := time.Unix(1_700_000_000, 0).UTC()

	b2 := terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut-b2", base)
	b3 := terminalMixDecisionSession("b3", "static_mix.pan_layout.v0", "cut-b3", base.Add(time.Minute))
	b4 := terminalMixDecisionSession("b4", "static_mix.low_end_relation.v0", "cut-b4", base.Add(2*time.Minute))
	for _, session := range []orchestration.PlanningSession{b2, b3, b4} {
		if _, err := store.RecordCapabilitySession(session); err != nil {
			t.Fatalf("record %s: %v", session.ID, err)
		}
	}

	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	if board.SchemaVersion != DecisionBoardSchemaVersion || board.DecisionCount != 3 {
		t.Fatalf("board = %#v", board)
	}
	byCapability := decisionViewsByCapability(board.Decisions)
	if got := byCapability["static_mix.static_balance.v0"].CurrentStatus; got != DecisionNeedsReview {
		t.Fatalf("B2 status = %s, decisions=%#v", got, board.Decisions)
	}
	if affected := byCapability["static_mix.static_balance.v0"].AffectedBy; len(affected) != 2 {
		t.Fatalf("B2 affected_by = %#v", affected)
	}
	if got := byCapability["static_mix.pan_layout.v0"].CurrentStatus; got != DecisionVerified {
		t.Fatalf("unrelated B3 status = %s", got)
	}
	if got := byCapability["static_mix.low_end_relation.v0"].CurrentStatus; got != DecisionVerified {
		t.Fatalf("B4 status = %s", got)
	}
}

func TestNewVerifiedDecisionSupersedesSameCapability(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	first := terminalMixDecisionSession("b2-first", "static_mix.static_balance.v0", "cut-1", base)
	second := terminalMixDecisionSession("b2-second", "static_mix.static_balance.v0", "cut-2", base.Add(time.Minute))
	if _, err := store.RecordCapabilitySession(first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordCapabilitySession(second); err != nil {
		t.Fatal(err)
	}
	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	if board.Decisions[0].CurrentStatus != DecisionSuperseded || board.Decisions[0].SupersededBy != "mixdec_b2-second" {
		t.Fatalf("first decision = %#v", board.Decisions[0])
	}
	if board.Decisions[1].CurrentStatus != DecisionVerified {
		t.Fatalf("second decision = %#v", board.Decisions[1])
	}
}

func TestMixReportDoesNotBlanketInvalidateOnProjectCutChange(t *testing.T) {
	store := NewStore(t.TempDir())
	session := terminalMixDecisionSession("b3", "static_mix.pan_layout.v0", "cut-recorded", time.Unix(1_700_000_000, 0).UTC())
	if _, err := store.RecordCapabilitySession(session); err != nil {
		t.Fatal(err)
	}
	current := session.FrozenPlan.ProjectCut
	current.BaseProjectRevision = "2"
	current.Hash = "cut-current"
	report, err := store.BuildMixReport(MixReportRequest{
		ProjectUUID: "project-1", CurrentProjectCut: &current,
		FinalMeasurements: map[string]any{"status": "ready", "true_peak_dbtp": -1.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != MixReportSchemaVersion || report.ExportReadiness != "needs_review" {
		t.Fatalf("report = %#v", report)
	}
	if len(report.DecisionTimeline) != 1 || report.DecisionTimeline[0].CurrentStatus != DecisionVerified {
		t.Fatalf("cut change blanket-invalidated decision: %#v", report.DecisionTimeline)
	}
	if len(report.Limitations) != 1 || report.Limitations[0] != "current_project_cut_has_unrecorded_changes" {
		t.Fatalf("limitations = %#v", report.Limitations)
	}
}

func TestMixReportDoesNotTreatContractOnlyCutHashDifferenceAsProjectChange(t *testing.T) {
	store := NewStore(t.TempDir())
	session := terminalMixDecisionSession("b3", "static_mix.pan_layout.v0", "capability-cut", time.Unix(1_700_000_000, 0).UTC())
	if _, err := store.RecordCapabilitySession(session); err != nil {
		t.Fatal(err)
	}
	current := session.FrozenPlan.ProjectCut
	current.Hash = "report-contract-cut"
	report, err := store.BuildMixReport(MixReportRequest{
		ProjectUUID: "project-1", CurrentProjectCut: &current,
		FinalMeasurements: map[string]any{"status": "ready"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExportReadiness != "ready" || len(report.UnresolvedItems) != 0 {
		t.Fatalf("contract-only cut difference was treated as project mutation: %#v", report)
	}
}

func TestMixReportMarksMissingFinalMeasurementsNotAssessed(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	report, err := store.BuildMixReport(MixReportRequest{ProjectUUID: "project-1"})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExportReadiness != "not_assessed" {
		t.Fatalf("export readiness = %q", report.ExportReadiness)
	}
}

func TestDecisionLedgerRejectsNonTerminalSession(t *testing.T) {
	store := NewStore(t.TempDir())
	session := terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut", time.Now().UTC())
	session.Status = orchestration.StatusVerifying
	if _, err := store.RecordCapabilitySession(session); err == nil {
		t.Fatal("non-terminal session was recorded")
	}
}

func TestDecisionRecordIsIdempotentAndImmutableBySessionID(t *testing.T) {
	root := t.TempDir()
	base := time.Unix(1_700_000_000, 0).UTC()
	store := NewStore(root)
	store.Now = func() time.Time { return base }
	session := terminalMixDecisionSession("immutable", "static_mix.static_balance.v0", "cut", base)
	first, err := store.RecordCapabilitySession(session)
	if err != nil {
		t.Fatal(err)
	}
	store.Now = func() time.Time { return base.Add(time.Hour) }
	second, err := store.RecordCapabilitySession(session)
	if err != nil || !second.Record.RecordedAt.Equal(first.Record.RecordedAt) {
		t.Fatalf("idempotent record=%#v err=%v", second.Record, err)
	}
	session.Goal = "different immutable content"
	if _, err := store.RecordCapabilitySession(session); err == nil {
		t.Fatal("immutable session record was overwritten")
	}
}

func TestDecisionReversionIsExplicitAndStopsRevertedWritePropagation(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut-b2", base)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b4", "static_mix.low_end_relation.v0", "cut-b4", base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	event, err := store.RecordDecisionReversion(DecisionReversionRequest{
		EventID: "undo-b4", ProjectUUID: "project-1", RecordID: "mixdec_b4",
		Reason:       "confirmed project rollback restored the frozen preimage",
		EvidenceRefs: []string{"project-history:undo-b4"}, OccurredAt: base.Add(2 * time.Minute),
	})
	if err != nil || event.Status != DecisionReverted {
		t.Fatalf("reversion event=%#v err=%v", event, err)
	}
	if _, err := store.RecordDecisionReversion(DecisionReversionRequest{
		EventID: "undo-b4", ProjectUUID: "project-1", RecordID: "mixdec_b4",
		Reason:       "confirmed project rollback restored the frozen preimage",
		EvidenceRefs: []string{"project-history:undo-b4"}, OccurredAt: base.Add(2 * time.Minute),
	}); err != nil {
		t.Fatalf("idempotent reversion failed: %v", err)
	}
	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	byCapability := decisionViewsByCapability(board.Decisions)
	if got := byCapability["static_mix.static_balance.v0"].CurrentStatus; got != DecisionVerified {
		t.Fatalf("reverted B4 still invalidated B2: %s", got)
	}
	if view := byCapability["static_mix.low_end_relation.v0"]; view.CurrentStatus != DecisionReverted || len(view.StateEventRefs) != 1 {
		t.Fatalf("reverted B4 projection = %#v", view)
	}
}

func TestRelatedDecisionRefsAreBoundedAndFlagUnsettledState(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut-b2", base)); err != nil {
		t.Fatal(err)
	}
	refs, err := store.RelatedDecisionRefs("project-1", "static_mix.pan_layout.v0")
	if err != nil || len(refs) != 1 || refs[0].Ref != "mixboard-decision:mixdec_b2" || refs[0].RequiresRevalidation {
		t.Fatalf("B3 related refs=%#v err=%v", refs, err)
	}
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b4", "static_mix.low_end_relation.v0", "cut-b4", base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	refs, err = store.RelatedDecisionRefs("project-1", "static_mix.pan_layout.v0")
	if err != nil || len(refs) != 1 || !refs[0].RequiresRevalidation || refs[0].CurrentStatus != DecisionNeedsReview {
		t.Fatalf("unsettled B2 ref was inherited without warning: refs=%#v err=%v", refs, err)
	}
}

func TestCancelledAndFailedTerminalSessionsHaveExplicitDecisionStatus(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	cancelled := terminalMixDecisionSession("cancelled", "static_mix.static_balance.v0", "cut-cancelled", base)
	cancelled.Status = orchestration.StatusCancelled
	cancelled.Execution = nil
	failed := terminalMixDecisionSession("failed", "static_mix.pan_layout.v0", "cut-failed", base.Add(time.Minute))
	failed.Status = orchestration.StatusFailed
	failed.Execution.VerificationResult.Status = "fail"
	for _, session := range []orchestration.PlanningSession{cancelled, failed} {
		if _, err := store.RecordCapabilitySession(session); err != nil {
			t.Fatal(err)
		}
	}
	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]DecisionView{}
	for _, view := range board.Decisions {
		byID[view.RecordID] = view
	}
	if byID["mixdec_cancelled"].CurrentStatus != DecisionCancelled || byID["mixdec_failed"].CurrentStatus != DecisionFailed {
		t.Fatalf("terminal decision statuses = %#v", byID)
	}
}

func TestLaterVerifiedDecisionDoesNotRewriteCancelledAuditTruth(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	cancelled := terminalMixDecisionSession("cancelled-first", "static_mix.static_balance.v0", "cut-cancelled", base)
	cancelled.Status = orchestration.StatusCancelled
	cancelled.Execution = nil
	if _, err := store.RecordCapabilitySession(cancelled); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("verified-later", "static_mix.static_balance.v0", "cut-verified", base.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	board, err := store.ReadProjectDecisionBoard("project-1")
	if err != nil {
		t.Fatal(err)
	}
	if board.Decisions[0].CurrentStatus != DecisionCancelled {
		t.Fatalf("cancelled audit truth was rewritten: %#v", board.Decisions[0])
	}
}

func terminalMixDecisionSession(id, capabilityID, cutHash string, completedAt time.Time) orchestration.PlanningSession {
	cut := orchestration.ProjectCut{
		ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "1",
		Consistency: "strong", Hash: cutHash,
	}
	actionSet := orchestration.ActionSet{
		ID: "actions-" + id, CapabilityID: capabilityID, ProjectCutHash: cutHash,
		Actions: []orchestration.Action{{ID: "action-" + id, Command: "test.apply", TargetRef: "track-1"}},
		Hash:    "action-hash-" + id,
	}
	verification := &orchestration.VerificationResult{
		Status: "pass", Structural: "pass", Acoustic: "pass", UserAcceptance: "unknown",
		EvidenceRefs: []string{"mix.observe:obs-after-" + id, "mix.observe.revision:2"},
	}
	return orchestration.PlanningSession{
		SchemaVersion: orchestration.SchemaVersion, ID: id, ProjectUUID: "project-1",
		EngineOwner: orchestration.EngineV1, Status: orchestration.StatusCompleted,
		Goal:       "test " + capabilityID,
		Invocation: orchestration.CapabilityInvocation{CapabilityID: capabilityID, CapabilityVer: "v0"},
		FrozenPlan: &orchestration.FrozenPlan{
			Proposal:  orchestration.Proposal{ID: "proposal-" + id, Revision: 1, CapabilityID: capabilityID, TargetScope: []string{"track-1"}},
			ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "context-" + id,
			PreviousObservationID: "obs-before-" + id,
		},
		Execution: &orchestration.ExecutionRecord{
			ID: "execution-" + id, SessionID: id, Status: "completed",
			ReceiptRef: "receipt-" + id, PersistenceRefs: []string{"project-history:commit-" + id},
			Receipts:     []orchestration.ActionReceipt{{ActionID: "action-" + id, Status: "ok", EffectivelyOnce: true}},
			Verification: "pass", VerificationResult: verification, UpdatedAt: completedAt,
		},
		CreatedAt: completedAt.Add(-time.Minute), UpdatedAt: completedAt,
	}
}

func decisionViewsByCapability(views []DecisionView) map[string]DecisionView {
	out := map[string]DecisionView{}
	for _, view := range views {
		out[view.CapabilityID] = view
	}
	return out
}
