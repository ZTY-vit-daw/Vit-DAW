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

type PanLayoutVSPPort struct {
	Client VSPClient

	mu           sync.Mutex
	baseRevision int64
	projectEpoch string
}

func (p *PanLayoutVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
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
		target, ok := numeric(action.Args["target_pan"])
		if action.Command != "track_pan_set" || strings.TrimSpace(action.TargetRef) == "" || strings.TrimSpace(action.BeforeFingerprint) == "" || !ok || target < -1 || target > 1 {
			return fmt.Errorf("unsupported or unguarded B3 action %s", action.ID)
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
	p.baseRevision, p.projectEpoch = baseRevision, cut.ProjectEpoch
	p.mu.Unlock()
	return nil
}

func (p *PanLayoutVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("VSP preflight has not completed")
	}
	target, ok := numeric(action.Args["target_pan"])
	if !ok || target < -1 || target > 1 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("target_pan in [-1,1] is required")
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd": "set_pan", "track_id": action.TargetRef, "pan": target, "base_revision": p.baseRevision,
	}, requestID, "tx_"+requestID)
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
	p.baseRevision = current.Revision
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, nil
}

func (p *PanLayoutVSPPort) Reconcile(ctx context.Context, action orchestration.Action, _ string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
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
	target, ok := numeric(action.Args["target_pan"])
	actual, actualOK := trackPan(current.LegacyState["tracks"], action.TargetRef)
	if !ok || !actualOK || math.Abs(actual-target) > 0.001 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, nil
}

func trackPan(value any, trackID string) (float64, bool) {
	rows := trackRows(value)
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprint(row["track_id"]))
		if id == "" || id == "<nil>" {
			id = strings.TrimSpace(fmt.Sprint(row["id"]))
		}
		if id != trackID {
			continue
		}
		for _, key := range []string{"pan", "pan_value", "balance"} {
			if value, ok := numeric(row[key]); ok {
				return value, true
			}
		}
	}
	return 0, false
}
