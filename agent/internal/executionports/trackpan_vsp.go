package executionports

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"vit-daw-agent/internal/orchestration"
)

// TrackPanVSPPort executes the bounded track pan adjustment (FAM3-S1) at the
// same D1-grade strictness as StaticBalanceVSPPort: CAS base revision, stable
// idempotency/transaction ids, a revision-advance gate after the mutation, a
// value readback assert, and the full receipt details the settlement chain
// consumes. Pan is normalized [-1,+1]; the kernel silently jlimits
// out-of-range values, so the Preflight rejects any out-of-bounds
// target_pan fail-closed before a command can reach the kernel — the port
// never clamps at send time (a clamped write would explode the readback
// assert after the mutation already happened).
type TrackPanVSPPort struct {
	Client VSPClient

	mu           sync.Mutex
	baseRevision int64
	projectEpoch string
	beforePan    map[string]float64
}

func (p *TrackPanVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.Client == nil {
		return fmt.Errorf("VSP client is required")
	}
	if !cut.IsExecutable() || actionSet.ProjectCutHash != cut.Hash {
		return fmt.Errorf("strong matching ProjectCut is required")
	}
	baseRevision, err := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if err != nil || baseRevision <= 0 {
		return fmt.Errorf("valid base project revision is required")
	}
	if len(actionSet.Actions) == 0 {
		return fmt.Errorf("action set is empty")
	}
	for _, action := range actionSet.Actions {
		if action.Command != "track_pan_adjust" || strings.TrimSpace(action.TargetRef) == "" || strings.TrimSpace(action.BeforeFingerprint) == "" {
			return fmt.Errorf("unsupported or unguarded B2 action %s", action.ID)
		}
		target, ok := numeric(action.Args["target_pan"])
		if !ok {
			return fmt.Errorf("action %s requires numeric target_pan", action.ID)
		}
		// Clamp-out-of-bounds pre-rejection (fail-closed): the plan computes
		// current + delta, so a move past the [-1,1] boundary arrives here as
		// an out-of-bounds absolute target. Reject it before any kernel write;
		// exactly +/-1 stays reachable.
		if target < -1 || target > 1 {
			return fmt.Errorf("action %s target_pan %.3f is out of the [-1,1] pan range", action.ID, target)
		}
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return fmt.Errorf("read current VSP snapshot: %w", err)
	}
	if current.ProjectEpoch != cut.ProjectEpoch || current.Revision != baseRevision {
		return fmt.Errorf("stale_project_cut: current epoch/revision does not match proposal")
	}
	p.mu.Lock()
	p.baseRevision = baseRevision
	p.projectEpoch = cut.ProjectEpoch
	p.beforePan = map[string]float64{}
	for _, action := range actionSet.Actions {
		if value, ok := trackPan(current.LegacyState["tracks"], action.TargetRef); ok {
			p.beforePan[action.TargetRef] = value
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *TrackPanVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("VSP preflight has not completed")
	}
	targetPan, ok := numeric(action.Args["target_pan"])
	if !ok || targetPan < -1 || targetPan > 1 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("target_pan within [-1,1] is required")
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	txID := "tx_" + requestID
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":           "set_pan",
		"track_id":      action.TargetRef,
		"pan":           targetPan,
		"base_revision": p.baseRevision,
	}, requestID, txID)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	legacy := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(legacy["status"])), "error") {
		message := strings.TrimSpace(fmt.Sprint(legacy["message"]))
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: message}, fmt.Errorf("VSP mutation failed: %s", message)
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true}, fmt.Errorf("mutation applied but readback snapshot failed: %w", err)
	}
	if current.ProjectEpoch != p.projectEpoch {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true}, fmt.Errorf("project epoch changed during execution")
	}
	if current.Revision <= p.baseRevision {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true}, fmt.Errorf("VSP revision did not advance after mutation")
	}
	actualPan, ok := trackPan(current.LegacyState["tracks"], action.TargetRef)
	if !ok || math.Abs(actualPan-targetPan) > 0.001 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, fmt.Errorf("track pan readback did not match target")
	}
	beforeRevision := p.baseRevision
	beforePan, beforeOK := p.beforePan[action.TargetRef]
	transactionID := strings.TrimSpace(result.TransactionID)
	if transactionID == "" {
		transactionID = txID
	}
	p.baseRevision = current.Revision
	return orchestration.ActionReceipt{
		ActionID:        action.ID,
		Status:          "applied",
		AppliedRevision: strconv.FormatInt(current.Revision, 10),
		EffectivelyOnce: true,
		EvidenceRefs:    []string{"vsp.command:" + requestID, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details: map[string]any{
			"before_revision": strconv.FormatInt(beforeRevision, 10), "after_revision": strconv.FormatInt(current.Revision, 10),
			"transaction_id": transactionID, "idempotency_key": requestID,
			"requested_target_pan": targetPan, "actual_readback_pan": actualPan, "readback_verified": true,
			"before_readback_pan": beforePan, "before_readback_available": beforeOK,
		},
	}, nil
}

func (p *TrackPanVSPPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	if p == nil || p.Client == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("VSP client is required")
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("read reconcile snapshot: %w", err)
	}
	if current.ProjectEpoch != cut.ProjectEpoch {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("project epoch changed before reconcile")
	}
	targetPan, ok := numeric(action.Args["target_pan"])
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("numeric target_pan is required")
	}
	actualPan, ok := trackPan(current.LegacyState["tracks"], action.TargetRef)
	baseRevision, parseErr := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if parseErr != nil || baseRevision <= 0 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("invalid reconcile base revision")
	}
	if !ok || math.Abs(actualPan-targetPan) > 0.001 || current.Revision <= baseRevision {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	return orchestration.ActionReceipt{
		ActionID:        action.ID,
		Status:          "applied",
		AppliedRevision: strconv.FormatInt(current.Revision, 10),
		EffectivelyOnce: true,
		EvidenceRefs:    []string{"vsp.reconcile:" + idempotencyKey, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details: map[string]any{
			"before_revision": strconv.FormatInt(baseRevision, 10), "after_revision": strconv.FormatInt(current.Revision, 10),
			"transaction_id": "tx_" + idempotencyKey, "idempotency_key": idempotencyKey,
			"requested_target_pan": targetPan, "actual_readback_pan": actualPan, "readback_verified": true, "reconciled": true,
		},
	}, nil
}
