package executionruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

type fakePort struct {
	Applied    []string
	FailID     string
	Reconciled []string
	Cancel     context.CancelFunc
}

func (f *fakePort) Reconcile(_ context.Context, action orchestration.Action, _ string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	f.Reconciled = append(f.Reconciled, action.ID)
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true}, nil
}

func (f *fakePort) Preflight(context.Context, orchestration.ActionSet, orchestration.ProjectCut) error {
	return nil
}
func (f *fakePort) Apply(_ context.Context, action orchestration.Action, _ string) (orchestration.ActionReceipt, error) {
	if action.ID == f.FailID {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, errFake
	}
	f.Applied = append(f.Applied, action.ID)
	if f.Cancel != nil {
		f.Cancel()
		f.Cancel = nil
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true}, nil
}

type fakeVerifier struct{ Status string }

func (f fakeVerifier) Verify(context.Context, orchestration.ActionSet, []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	return orchestration.VerificationResult{Status: f.Status, Structural: "pass"}, nil
}

type errorVerifier struct{}

func (errorVerifier) Verify(context.Context, orchestration.ActionSet, []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	return orchestration.VerificationResult{Summary: "verification transport unavailable"}, errors.New("verification transport unavailable")
}

type countingVerifier struct{ Calls int }

func (f *countingVerifier) Verify(context.Context, orchestration.ActionSet, []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	f.Calls++
	return orchestration.VerificationResult{Status: "pass", Structural: "pass", Acoustic: "pass"}, nil
}

type fakePersistence struct{}

func (fakePersistence) PrepareExecution(context.Context, orchestration.PlanningSession, orchestration.ActionSet, orchestration.ProjectCut) ([]string, error) {
	return []string{"project-history:baseline-1"}, nil
}

var errFake = fakeError("fake mutation failure")

type fakeError string

func (e fakeError) Error() string { return string(e) }

func TestCoordinatorRejectsBoundedCutBeforeMutation(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "bounded")
	port := &fakePort{}
	if _, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{}); err == nil {
		t.Fatal("bounded Cut must not reach mutation port")
	}
	if len(port.Applied) != 0 {
		t.Fatalf("unexpected mutation calls: %#v", port.Applied)
	}
}

func TestCoordinatorRejectsEphemeralStoreBeforeMutation(t *testing.T) {
	_, session, actionSet, cut := authorizedFixture(t, "strong")
	memory := orchestration.NewMemoryStore()
	if err := memory.Create(session); err != nil {
		t.Fatal(err)
	}
	port := &fakePort{}
	if _, err := New(memory).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{}); err == nil {
		t.Fatal("ephemeral store must not own project mutation")
	}
	if len(port.Applied) != 0 {
		t.Fatalf("ephemeral execution mutated project: %#v", port.Applied)
	}
}

func TestCoordinatorRequiresProjectPersistenceBeforeMutation(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	port := &fakePort{}
	if _, err := New(store).Execute(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}); err == nil {
		t.Fatal("execution without persistence port must be rejected")
	}
	if len(port.Applied) != 0 {
		t.Fatalf("execution without history baseline mutated project: %#v", port.Applied)
	}
}

func TestCoordinatorRejectsHashOnlyProposalWithoutFrozenPlan(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	loaded, _ := store.Load(session.ID)
	loaded.FrozenPlan = nil
	loaded.Revision++
	if err := store.Save(loaded, session.Revision); err != nil {
		t.Fatal(err)
	}
	port := &fakePort{}
	if _, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{}); err == nil {
		t.Fatal("hash-only proposal must not reach execution")
	}
	if len(port.Applied) != 0 {
		t.Fatalf("unfrozen execution mutated project: %#v", port.Applied)
	}
}

func TestCoordinatorAppliesAndVerifiesStrongCut(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	port := &fakePort{}
	completed, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != orchestration.StatusCompleted || len(port.Applied) != 2 {
		t.Fatalf("unexpected completed session=%#v applied=%v", completed, port.Applied)
	}
	if completed.Execution == nil || len(completed.Execution.PersistenceRefs) != 1 || completed.Execution.PersistenceRefs[0] != "project-history:baseline-1" {
		t.Fatalf("execution did not retain project history reference: %#v", completed.Execution)
	}
	if completed.Execution.VerificationResult == nil || completed.Execution.VerificationResult.Status != "pass" || completed.Execution.VerificationResult.Structural != "pass" {
		t.Fatalf("execution did not retain full verification result: %#v", completed.Execution)
	}
}

func TestCoordinatorStopsOnPartialFailure(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	port := &fakePort{FailID: actionSet.Actions[1].ID}
	failed, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{})
	if err == nil || failed.Status != orchestration.StatusFailed {
		t.Fatalf("expected failed execution, session=%#v err=%v", failed, err)
	}
	if len(port.Applied) != 1 {
		t.Fatalf("remaining actions must stop after partial failure: %#v", port.Applied)
	}
}

func TestCoordinatorPersistsNormalizedVerificationNeedsReviewWithoutCallingExecutionFailed(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	review, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, &fakePort{}, errorVerifier{}, fakePersistence{})
	if err != nil || review.Status != orchestration.StatusNeedsReview || review.Execution == nil {
		t.Fatalf("expected persisted needs-review outcome: session=%#v err=%v", review, err)
	}
	result := review.Execution.VerificationResult
	if review.Execution.Status != "verification_inconclusive" || review.Execution.Verification != "inconclusive" || result == nil || result.Status != "inconclusive" || result.Structural != "inconclusive" || result.Acoustic != "not_run" || result.UserAcceptance != "unknown" {
		t.Fatalf("verification review outcome was not normalized: %#v", review.Execution)
	}
	reloaded, ok := store.Load(session.ID)
	if !ok || reloaded.Status != orchestration.StatusNeedsReview || reloaded.Execution == nil || reloaded.Execution.VerificationResult == nil || reloaded.Execution.VerificationResult.UserAcceptance != "unknown" {
		t.Fatalf("normalized verification failure was not durable: %#v", reloaded)
	}
}

func TestCoordinatorKeepsExplicitVerificationFailureFailed(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	failed, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, &fakePort{}, fakeVerifier{"fail"}, fakePersistence{})
	if err == nil || failed.Status != orchestration.StatusFailed || failed.Execution == nil || failed.Execution.Status != "verification_failed" {
		t.Fatalf("explicit verification failure must remain failed: session=%#v err=%v", failed, err)
	}
}

func TestCoordinatorPersistsInconclusiveVerificationAsNeedsReview(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	review, err := New(store).ExecuteWithPersistence(context.Background(), session.ID, actionSet, cut, &fakePort{}, fakeVerifier{"inconclusive"}, fakePersistence{})
	if err != nil || review.Status != orchestration.StatusNeedsReview || review.Execution == nil || review.Execution.Status != "verification_inconclusive" {
		t.Fatalf("inconclusive verification must be a non-failed terminal review outcome: session=%#v err=%v", review, err)
	}
}

func TestCoordinatorCancelRacePersistsInflightReceiptAndStopsDispatch(t *testing.T) {
	store, session, actionSet, cut := authorizedFixture(t, "strong")
	ctx, cancel := context.WithCancel(context.Background())
	port := &fakePort{Cancel: cancel}
	cancelled, err := New(store).ExecuteWithPersistence(ctx, session.ID, actionSet, cut, port, fakeVerifier{"pass"}, fakePersistence{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if cancelled.Status != orchestration.StatusCancelled || cancelled.Execution == nil || len(cancelled.Execution.Receipts) != 1 {
		t.Fatalf("in-flight receipt was not durably retained: %#v", cancelled)
	}
	if len(port.Applied) != 1 || port.Applied[0] != actionSet.Actions[0].ID {
		t.Fatalf("cancel dispatched a later action: %#v", port.Applied)
	}
	persisted, _ := store.Load(session.ID)
	if persisted.Status != orchestration.StatusCancelled || len(persisted.Execution.Receipts) != 1 || persisted.Execution.Receipts[0].Status != "applied" {
		t.Fatalf("persisted cancel state lost receipt: %#v", persisted)
	}
}

func TestCoordinatorReconcilesCrashAfterApplyWithoutRetry(t *testing.T) {
	store, session, actionSet, _ := authorizedFixture(t, "strong")
	loaded, _ := store.Load(session.ID)
	loaded.Status = orchestration.StatusExecuting
	loaded.Authorization.Consumed = true
	loaded.Execution = &orchestration.ExecutionRecord{
		ID: "execution-1", SessionID: loaded.ID, ActionSetHash: actionSet.Hash,
		IdempotencyKey: "exec:s1", Status: "prepared", ProjectCut: orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "e1", Consistency: "strong", Hash: "cut1"},
	}
	loaded.Revision++
	if err := store.Save(loaded, session.Revision); err != nil {
		t.Fatal(err)
	}
	port := &fakePort{}
	completed, err := New(store).Reconcile(context.Background(), session.ID, actionSet, port, fakeVerifier{"pass"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != orchestration.StatusCompleted || len(port.Reconciled) != 2 || len(port.Applied) != 0 {
		t.Fatalf("recovery retried mutation or failed: session=%#v reconciled=%v applied=%v", completed, port.Reconciled, port.Applied)
	}
}

func TestCoordinatorCrashRecoveryMatrixUsesStateReconciliationNotMutationRetry(t *testing.T) {
	tests := []struct {
		name                string
		status              orchestration.SessionStatus
		receiptCount        int
		wantReconciliations int
	}{
		{name: "prepare_after_before_apply", status: orchestration.StatusExecuting, receiptCount: 0, wantReconciliations: 2},
		{name: "apply_after_before_receipt", status: orchestration.StatusExecuting, receiptCount: 0, wantReconciliations: 2},
		{name: "partial_batch", status: orchestration.StatusExecuting, receiptCount: 1, wantReconciliations: 1},
		{name: "receipt_after_before_verify", status: orchestration.StatusExecuting, receiptCount: 2, wantReconciliations: 0},
		{name: "verify_after_before_finalize", status: orchestration.StatusVerifying, receiptCount: 2, wantReconciliations: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, session, actionSet, cut := authorizedFixture(t, "strong")
			loaded, _ := store.Load(session.ID)
			loaded.Status = test.status
			loaded.Authorization.Consumed = true
			loaded.Execution = &orchestration.ExecutionRecord{
				ID: "execution-matrix", SessionID: loaded.ID, ActionSetHash: actionSet.Hash,
				IdempotencyKey: "exec:matrix", Status: "prepared", ProjectCut: cut,
			}
			for index := 0; index < test.receiptCount; index++ {
				loaded.Execution.Receipts = append(loaded.Execution.Receipts, orchestration.ActionReceipt{
					ActionID: actionSet.Actions[index].ID, Status: "applied", EffectivelyOnce: true,
				})
			}
			loaded.Revision++
			if err := store.Save(loaded, session.Revision); err != nil {
				t.Fatal(err)
			}
			port := &fakePort{}
			verifier := &countingVerifier{}
			completed, err := New(store).Reconcile(context.Background(), session.ID, actionSet, port, verifier)
			if err != nil || completed.Status != orchestration.StatusCompleted {
				t.Fatalf("recovery failed: session=%#v err=%v", completed, err)
			}
			if len(port.Applied) != 0 {
				t.Fatalf("recovery retried mutation: %#v", port.Applied)
			}
			if len(port.Reconciled) != test.wantReconciliations {
				t.Fatalf("reconciliations=%d want=%d (%#v)", len(port.Reconciled), test.wantReconciliations, port.Reconciled)
			}
			if verifier.Calls != 1 {
				t.Fatalf("verification calls=%d want=1", verifier.Calls)
			}
		})
	}
}

func authorizedFixture(t *testing.T, consistency string) (orchestration.Store, orchestration.PlanningSession, orchestration.ActionSet, orchestration.ProjectCut) {
	t.Helper()
	store, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "orchestration.json"))
	if err != nil {
		t.Fatal(err)
	}
	session, err := orchestration.NewSession("s1", "p1", "execute B2", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: "static_mix.static_balance.v0"})
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "e1", Consistency: consistency}
	cut.Hash = cut.ComputeHash()
	actionSet := orchestration.ActionSet{ID: "as1", CapabilityID: "static_mix.static_balance.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{
		{ID: "a1", Command: "track_gain_adjust", TargetRef: "t1", Compensatable: true},
		{ID: "a2", Command: "track_gain_adjust", TargetRef: "t2", Compensatable: true},
	}}
	actionSet.Hash = actionSet.ComputeHash()
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{
		Proposal:  orchestration.Proposal{ID: "prop1", Revision: 1, CapabilityID: actionSet.CapabilityID, ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash},
		ActionSet: actionSet, ProjectCut: cut, PreviousObservationID: "obs-before",
	})
	if err != nil {
		t.Fatal(err)
	}
	decision := orchestration.ApprovalDecision{SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove, ProposalID: "prop1", ProposalRevision: 1, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn1"}
	session, err = session.Authorize(orchestration.Authorization{ProposalID: "prop1", ProposalRevision: 1, ActionSetHash: actionSet.Hash, ProjectCutHash: cut.Hash, SourceTurnID: "turn1", Sequence: 1, Decision: &decision})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(session); err != nil {
		t.Fatal(err)
	}
	return store, session, actionSet, cut
}
