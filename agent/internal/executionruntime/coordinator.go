// Package executionruntime owns the only v1 path that may submit a typed
// ActionSet to a project mutation port. It does not know DAW command names.
package executionruntime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
)

type MutationPort interface {
	Preflight(context.Context, orchestration.ActionSet, orchestration.ProjectCut) error
	Apply(context.Context, orchestration.Action, string) (orchestration.ActionReceipt, error)
}

type ReconcilePort interface {
	Reconcile(context.Context, orchestration.Action, string, orchestration.ProjectCut) (orchestration.ActionReceipt, error)
}

type Verifier interface {
	Verify(context.Context, orchestration.ActionSet, []orchestration.ActionReceipt) (orchestration.VerificationResult, error)
}

type PersistencePort interface {
	PrepareExecution(context.Context, orchestration.PlanningSession, orchestration.ActionSet, orchestration.ProjectCut) ([]string, error)
}

type Coordinator struct {
	Store orchestration.Store
	Now   func() time.Time
}

func New(store orchestration.Store) *Coordinator {
	return &Coordinator{Store: store, Now: func() time.Time { return time.Now().UTC() }}
}

func (c *Coordinator) Execute(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, currentCut orchestration.ProjectCut, port MutationPort, verifier Verifier) (orchestration.PlanningSession, error) {
	return c.ExecuteWithPersistence(ctx, sessionID, actionSet, currentCut, port, verifier, nil)
}

func (c *Coordinator) ExecuteWithPersistence(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, currentCut orchestration.ProjectCut, port MutationPort, verifier Verifier, persistence PersistencePort) (orchestration.PlanningSession, error) {
	if c == nil || c.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("execution coordinator store is nil")
	}
	if !orchestration.IsDurableStore(c.Store) {
		return orchestration.PlanningSession{}, fmt.Errorf("execution coordinator requires a durable orchestration store")
	}
	if port == nil || verifier == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("mutation port and verifier are required")
	}
	if persistence == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("execution persistence port is required")
	}
	session, ok := c.Store.Load(sessionID)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s not found", sessionID)
	}
	if session.EngineOwner != orchestration.EngineV1 {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s is owned by %s", sessionID, session.EngineOwner)
	}
	if session.Status != orchestration.StatusAuthorized || session.Authorization == nil || session.Authorization.Consumed {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s has no consumable authorization", sessionID)
	}
	if session.ActiveProposal == nil || actionSet.Hash == "" || actionSet.Hash != actionSet.ComputeHash() {
		return orchestration.PlanningSession{}, fmt.Errorf("action set hash is missing or invalid")
	}
	if session.FrozenPlan == nil || session.FrozenPlan.ActionSet.Hash != actionSet.Hash || session.FrozenPlan.ProjectCut.Hash != currentCut.Hash {
		return orchestration.PlanningSession{}, fmt.Errorf("execution input does not match the persisted frozen plan")
	}
	if session.ActiveProposal.ActionSetHash != actionSet.Hash || session.ActiveProposal.ProjectCutHash != currentCut.Hash {
		return orchestration.PlanningSession{}, fmt.Errorf("action set or project cut does not match authorized proposal")
	}
	if !currentCut.IsExecutable() {
		return orchestration.PlanningSession{}, fmt.Errorf("project cut is not executable: consistency=%s", currentCut.Consistency)
	}
	if err := port.Preflight(ctx, actionSet, currentCut); err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("execution preflight: %w", err)
	}
	persistenceRefs, err := persistence.PrepareExecution(ctx, session, actionSet, currentCut)
	if err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("prepare execution persistence: %w", err)
	}
	if len(persistenceRefs) == 0 {
		return orchestration.PlanningSession{}, fmt.Errorf("prepare execution persistence returned no durable reference")
	}

	expected := session.Revision
	now := c.now()
	session.Status = orchestration.StatusExecuting
	session.Revision++
	session.UpdatedAt = now
	session.Authorization.Consumed = true
	session.Execution = &orchestration.ExecutionRecord{
		ID:              "execution_" + session.ID + "_" + actionSet.Hash[:16],
		SessionID:       session.ID,
		ActionSetHash:   actionSet.Hash,
		ProjectCut:      currentCut,
		IdempotencyKey:  "exec:" + session.ID + ":" + actionSet.Hash,
		Status:          "prepared",
		PersistenceRefs: append([]string(nil), persistenceRefs...),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := c.Store.Save(session, expected); err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("persist prepared execution: %w", err)
	}
	expected = session.Revision

	for _, action := range actionSet.Actions {
		if err := ctx.Err(); err != nil {
			return c.cancel(session, expected, err)
		}
		receipt, err := port.Apply(ctx, action, session.Execution.IdempotencyKey+":"+action.ID)
		if err != nil {
			receipt.ActionID = action.ID
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if receipt.Status == "" {
					receipt.Status = "unknown_after_cancel"
				}
				session.Execution.Receipts = append(session.Execution.Receipts, receipt)
				return c.cancel(session, expected, firstContextError(ctx, err))
			}
			if receipt.Status == "" {
				receipt.Status = "failed"
			}
			session.Execution.Receipts = append(session.Execution.Receipts, receipt)
			session.Execution.Status = "partial_failure"
			session.Execution.UpdatedAt = c.now()
			session.Status = orchestration.StatusFailed
			session.Revision++
			if saveErr := c.Store.Save(session, expected); saveErr != nil {
				return orchestration.PlanningSession{}, fmt.Errorf("persist failed execution: %w (apply: %v)", saveErr, err)
			}
			return session, fmt.Errorf("action %s failed: %w", action.ID, err)
		}
		if receipt.ActionID == "" {
			receipt.ActionID = action.ID
		}
		if receipt.Status == "" {
			receipt.Status = "applied"
		}
		session.Execution.Receipts = append(session.Execution.Receipts, receipt)
		cancelled := ctx.Err()
		if cancelled != nil {
			session.Execution.Status = "cancelled"
			session.Status = orchestration.StatusCancelled
		} else {
			session.Execution.Status = "applied"
		}
		session.Execution.UpdatedAt = c.now()
		session.Revision++
		if err := c.Store.Save(session, expected); err != nil {
			return orchestration.PlanningSession{}, fmt.Errorf("persist action receipt: %w", err)
		}
		expected = session.Revision
		if cancelled != nil {
			return session, cancelled
		}
	}

	if err := ctx.Err(); err != nil {
		return c.cancel(session, expected, err)
	}
	session.Status = orchestration.StatusVerifying
	session.Revision++
	if err := c.Store.Save(session, expected); err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("persist verifying execution: %w", err)
	}
	expected = session.Revision
	verification, err := verifier.Verify(ctx, actionSet, session.Execution.Receipts)
	session.Execution.VerificationResult = normalizeVerificationResult(verification)
	session.Execution.Verification = session.Execution.VerificationResult.Status
	session.Execution.UpdatedAt = c.now()
	switch {
	case verificationExplicitFailure(*session.Execution.VerificationResult):
		session.Execution.Status = "verification_failed"
		session.Status = orchestration.StatusFailed
	case verificationPassed(*session.Execution.VerificationResult) && err == nil:
		session.Execution.Status = "verified"
		session.Status = orchestration.StatusCompleted
	default:
		// Mutation and durable Receipts already succeeded. Missing or unavailable
		// verification evidence is a terminal review outcome, not an execution
		// failure and must never invite an automatic mutation retry.
		session.Execution.Status = "verification_inconclusive"
		session.Status = orchestration.StatusNeedsReview
	}
	session.Revision++
	if saveErr := c.Store.Save(session, expected); saveErr != nil {
		if err != nil {
			return orchestration.PlanningSession{}, fmt.Errorf("persist verification: %w (verify: %v)", saveErr, err)
		}
		return orchestration.PlanningSession{}, fmt.Errorf("persist verification: %w", saveErr)
	}
	if session.Status == orchestration.StatusFailed {
		if err != nil {
			return session, fmt.Errorf("verification failed: %w", err)
		}
		return session, fmt.Errorf("verification status %s", session.Execution.Verification)
	}
	return session, nil
}

// Reconcile resumes an execution after a crash without blindly retrying a
// mutation. Missing receipts are rebuilt from the real project state.
func (c *Coordinator) Reconcile(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, port ReconcilePort, verifier Verifier) (orchestration.PlanningSession, error) {
	if c == nil || c.Store == nil || port == nil || verifier == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("store, reconcile port and verifier are required")
	}
	if !orchestration.IsDurableStore(c.Store) {
		return orchestration.PlanningSession{}, fmt.Errorf("execution recovery requires a durable orchestration store")
	}
	session, ok := c.Store.Load(sessionID)
	if !ok || session.Execution == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("prepared execution for session %s not found", sessionID)
	}
	if session.EngineOwner != orchestration.EngineV1 || (session.Status != orchestration.StatusExecuting && session.Status != orchestration.StatusVerifying) {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s is not recoverable from status %s", sessionID, session.Status)
	}
	if session.Execution.ActionSetHash != actionSet.Hash || actionSet.Hash != actionSet.ComputeHash() {
		return orchestration.PlanningSession{}, fmt.Errorf("recovery action set does not match execution record")
	}
	receipts := make(map[string]orchestration.ActionReceipt, len(session.Execution.Receipts))
	for _, receipt := range session.Execution.Receipts {
		receipts[receipt.ActionID] = receipt
	}
	expected := session.Revision
	for _, action := range actionSet.Actions {
		if receipt, exists := receipts[action.ID]; exists && strings.EqualFold(receipt.Status, "applied") {
			continue
		}
		receipt, err := port.Reconcile(ctx, action, session.Execution.IdempotencyKey+":"+action.ID, session.Execution.ProjectCut)
		if err != nil {
			return session, fmt.Errorf("reconcile action %s: %w", action.ID, err)
		}
		if receipt.ActionID == "" {
			receipt.ActionID = action.ID
		}
		receipts[action.ID] = receipt
	}
	session.Execution.Receipts = make([]orchestration.ActionReceipt, 0, len(actionSet.Actions))
	allApplied := true
	for _, action := range actionSet.Actions {
		receipt := receipts[action.ID]
		if !strings.EqualFold(receipt.Status, "applied") {
			allApplied = false
		}
		session.Execution.Receipts = append(session.Execution.Receipts, receipt)
	}
	if !allApplied {
		session.Execution.Status = "reconcile_incomplete"
		session.Execution.UpdatedAt = c.now()
		session.Status = orchestration.StatusFailed
		session.Revision++
		if err := c.Store.Save(session, expected); err != nil {
			return orchestration.PlanningSession{}, fmt.Errorf("persist incomplete reconciliation: %w", err)
		}
		return session, fmt.Errorf("execution remains incomplete after state reconciliation")
	}
	session.Status = orchestration.StatusVerifying
	session.Execution.Status = "reconciled"
	session.Execution.UpdatedAt = c.now()
	session.Revision++
	if err := c.Store.Save(session, expected); err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("persist reconciled receipts: %w", err)
	}
	expected = session.Revision
	verification, err := verifier.Verify(ctx, actionSet, session.Execution.Receipts)
	session.Execution.VerificationResult = normalizeVerificationResult(verification)
	session.Execution.Verification = session.Execution.VerificationResult.Status
	session.Execution.UpdatedAt = c.now()
	switch {
	case verificationExplicitFailure(*session.Execution.VerificationResult):
		session.Execution.Status = "verification_failed"
		session.Status = orchestration.StatusFailed
	case verificationPassed(*session.Execution.VerificationResult) && err == nil:
		session.Execution.Status = "verified"
		session.Status = orchestration.StatusCompleted
	default:
		session.Execution.Status = "verification_inconclusive"
		session.Status = orchestration.StatusNeedsReview
	}
	session.Revision++
	if saveErr := c.Store.Save(session, expected); saveErr != nil {
		if err != nil {
			return orchestration.PlanningSession{}, fmt.Errorf("persist reconciled verification: %w (verify: %v)", saveErr, err)
		}
		return orchestration.PlanningSession{}, fmt.Errorf("persist reconciled verification: %w", saveErr)
	}
	if session.Status == orchestration.StatusFailed {
		if err != nil {
			return session, fmt.Errorf("verify reconciled execution: %w", err)
		}
		return session, fmt.Errorf("reconciled verification status %s", session.Execution.Verification)
	}
	return session, nil
}

func (c *Coordinator) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c *Coordinator) cancel(session orchestration.PlanningSession, expected uint64, cause error) (orchestration.PlanningSession, error) {
	if session.Execution == nil {
		return session, cause
	}
	session.Status = orchestration.StatusCancelled
	session.Execution.Status = "cancelled"
	session.Execution.UpdatedAt = c.now()
	session.Revision++
	if err := c.Store.Save(session, expected); err != nil {
		return orchestration.PlanningSession{}, fmt.Errorf("persist cancelled execution: %w (cancel: %v)", err, cause)
	}
	return session, cause
}

func firstContextError(ctx context.Context, fallback error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fallback
}

func normalizeVerificationResult(result orchestration.VerificationResult) *orchestration.VerificationResult {
	copy := result
	copy.EvidenceRefs = append([]string(nil), result.EvidenceRefs...)
	copy.Status = firstNonEmpty(copy.Status, "inconclusive")
	copy.Structural = firstNonEmpty(copy.Structural, "inconclusive")
	copy.Acoustic = firstNonEmpty(copy.Acoustic, "not_run")
	copy.UserAcceptance = firstNonEmpty(copy.UserAcceptance, "unknown")
	return &copy
}

func verificationPassed(result orchestration.VerificationResult) bool {
	return strings.EqualFold(strings.TrimSpace(result.Status), "pass") &&
		!strings.EqualFold(strings.TrimSpace(result.Structural), "fail") &&
		!strings.EqualFold(strings.TrimSpace(result.Acoustic), "fail")
}

func verificationExplicitFailure(result orchestration.VerificationResult) bool {
	return strings.EqualFold(strings.TrimSpace(result.Status), "fail") ||
		strings.EqualFold(strings.TrimSpace(result.Structural), "fail") ||
		strings.EqualFold(strings.TrimSpace(result.Acoustic), "fail")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
