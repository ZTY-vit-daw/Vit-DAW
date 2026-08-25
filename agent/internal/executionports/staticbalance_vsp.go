// Package executionports adapts typed orchestration actions to concrete project
// foundation protocols. It contains no authorization logic.
package executionports

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type VSPClient interface {
	VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error)
	SendVSPLegacyCommandWithIDs(context.Context, map[string]any, string, string) (*kernel.VSPCommandResult, error)
}

type StaticBalanceVSPPort struct {
	Client VSPClient

	mu           sync.Mutex
	baseRevision int64
	projectEpoch string
	beforeGainDB map[string]float64
}

func (p *StaticBalanceVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
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
		if action.Command != "track_gain_adjust" || strings.TrimSpace(action.TargetRef) == "" || strings.TrimSpace(action.BeforeFingerprint) == "" {
			return fmt.Errorf("unsupported or unguarded B2 action %s", action.ID)
		}
		if _, ok := numeric(action.Args["target_db"]); !ok {
			return fmt.Errorf("action %s requires numeric target_db", action.ID)
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
	p.beforeGainDB = map[string]float64{}
	for _, action := range actionSet.Actions {
		if value, ok := trackVolume(current.LegacyState["tracks"], action.TargetRef); ok {
			p.beforeGainDB[action.TargetRef] = value
		}
	}
	p.mu.Unlock()
	return nil
}

func (p *StaticBalanceVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("VSP preflight has not completed")
	}
	targetDB, ok := numeric(action.Args["target_db"])
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("numeric target_db is required")
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	txID := "tx_" + requestID
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":           "set_volume",
		"track_id":      action.TargetRef,
		"db":            targetDB,
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
	actualDB, ok := trackVolume(current.LegacyState["tracks"], action.TargetRef)
	if !ok || math.Abs(actualDB-targetDB) > 0.001 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, fmt.Errorf("track gain readback did not match target")
	}
	beforeRevision := p.baseRevision
	beforeDB, beforeOK := p.beforeGainDB[action.TargetRef]
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
			"requested_target_db": targetDB, "actual_readback_db": actualDB, "readback_verified": true,
			"before_readback_db": beforeDB, "before_readback_available": beforeOK,
		},
	}, nil
}

func (p *StaticBalanceVSPPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
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
	targetDB, ok := numeric(action.Args["target_db"])
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("numeric target_db is required")
	}
	actualDB, ok := trackVolume(current.LegacyState["tracks"], action.TargetRef)
	baseRevision, parseErr := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if parseErr != nil || baseRevision <= 0 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("invalid reconcile base revision")
	}
	if !ok || math.Abs(actualDB-targetDB) > 0.001 || current.Revision <= baseRevision {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true,
		EvidenceRefs: []string{"vsp.reconcile:" + idempotencyKey, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details: map[string]any{
			"before_revision": strconv.FormatInt(baseRevision, 10), "after_revision": strconv.FormatInt(current.Revision, 10),
			"transaction_id": "tx_" + idempotencyKey, "idempotency_key": idempotencyKey,
			"requested_target_db": targetDB, "actual_readback_db": actualDB, "readback_verified": true, "reconciled": true,
		},
	}, nil
}

func numeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case jsonNumber:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

type jsonNumber interface {
	Float64() (float64, error)
}

func trackVolume(value any, trackID string) (float64, bool) {
	tracks := trackRows(value)
	for _, track := range tracks {
		id := strings.TrimSpace(fmt.Sprint(track["track_id"]))
		if id == "" || id == "<nil>" {
			id = strings.TrimSpace(fmt.Sprint(track["id"]))
		}
		if id != trackID {
			continue
		}
		for _, key := range []string{"volume_db", "fader_db", "gain_db", "db"} {
			if parsed, ok := numeric(track[key]); ok {
				return parsed, true
			}
		}
	}
	return 0, false
}

func trackRows(value any) []map[string]any {
	var tracks []map[string]any
	data, _ := json.Marshal(value)
	_ = json.Unmarshal(data, &tracks)
	return tracks
}
