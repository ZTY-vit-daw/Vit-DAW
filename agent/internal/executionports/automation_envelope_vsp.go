// Package executionports adapts typed orchestration actions to concrete project
// foundation protocols. It contains no authorization logic.
package executionports

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/orchestration"
)

// AutomationEnvelopeVSPPort executes the section_automation domain row
// (M3-C4-AUTO-INTENT-1): one parameter automation envelope write routed to
// the kernel's legacy automation.set_points command, verified by an
// automation.get_curve per-point readback. It mirrors the StaticBalance port
// shape (Preflight/Apply/Reconcile under one idempotency key) so the
// orchestration runtime drives it exactly like the scalar D1-S1 rows.
//
// Governance notes (the STOP-NOTE red line): this port is the materializer
// face of the domain-table row — it never runs inside the public invoke
// surface (freeStateInvokeMutationReason stays untouched and still rejects
// automation.* there for active loops). The parameter raw value range is read
// from the kernel per parameter and every written point is rejected
// fail-closed when out of range.
type AutomationEnvelopeVSPPort struct {
	Client VSPClient

	mu           sync.Mutex
	baseRevision int64
	projectEpoch string
	valueMin     map[string]float64
	valueMax     map[string]float64
	beforeCount  map[string]int
}

func automationParamKey(pluginID, paramID string) string {
	return strings.TrimSpace(pluginID) + "\x1f" + strings.TrimSpace(paramID)
}

// readParameterRange reads one parameter's raw value range and current curve
// point count through the governed legacy get_plugin_parameters command
// (201/217 kernel face: parameter rows carry top-level min/max and
// envelope_point_count).
func (p *AutomationEnvelopeVSPPort) readParameterRange(ctx context.Context, trackID, pluginID, paramID string) (min, max float64, pointCount int, err error) {
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  trackID,
		"plugin_id": pluginID,
		"param_id":  paramID,
	}, "read:auto:"+automationParamKey(pluginID, paramID), "tx_read_auto:"+automationParamKey(pluginID, paramID))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("automation parameter range read failed: %w", err)
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		return 0, 0, 0, fmt.Errorf("automation parameter range read failed: %s", strings.TrimSpace(fmt.Sprint(reply["message"])))
	}
	rows, _ := reply["parameters"].([]any)
	for _, row := range rows {
		entry, _ := row.(map[string]any)
		if entry == nil || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["param_id"])), strings.TrimSpace(paramID)) {
			continue
		}
		minValue, minOK := numeric(entry["min"])
		maxValue, maxOK := numeric(entry["max"])
		if !minOK || !maxOK {
			return 0, 0, 0, fmt.Errorf("automation parameter %s carries no readable raw value range", paramID)
		}
		count := 0
		if raw, ok := numeric(entry["envelope_point_count"]); ok {
			count = int(raw)
		}
		return minValue, maxValue, count, nil
	}
	return 0, 0, 0, fmt.Errorf("automation parameter %q was not present on plugin %q", paramID, pluginID)
}

// readCurve reads the current envelope curve through the legacy
// automation.get_curve command (RiskDirect face).
func (p *AutomationEnvelopeVSPPort) readCurve(ctx context.Context, pluginID, paramID string) ([]map[string]any, error) {
	return p.readCurveUnlocked(ctx, pluginID, paramID)
}

// automationActionPayload parses the action args into the port's write
// payload identity + points + region.
func automationActionPayload(action orchestration.Action) (pluginID, paramID string, points []experiment.AutomationEnvelopePoint, region experiment.AutomationEnvelopeRegion, err error) {
	pluginID = actionArgText(action, "plugin_id")
	paramID = actionArgText(action, "param_id")
	if pluginID == "" || paramID == "" {
		return "", "", nil, region, fmt.Errorf("automation envelope action %s requires plugin_id and param_id", action.ID)
	}
	args := map[string]any{"points": action.Args["points"], "region": action.Args["region"]}
	points, err = experiment.ParseAutomationEnvelopePoints(args)
	if err != nil {
		return "", "", nil, region, err
	}
	region, err = experiment.ParseAutomationEnvelopeRegion(args)
	if err != nil {
		return "", "", nil, region, err
	}
	if err = experiment.AutomationEnvelopePointsWithinRegion(points, region); err != nil {
		return "", "", nil, region, err
	}
	return pluginID, paramID, points, region, nil
}

func (p *AutomationEnvelopeVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
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
		if action.Command != experiment.AutomationEnvelopeWriteKind || strings.TrimSpace(action.TargetRef) == "" {
			return fmt.Errorf("unsupported or unguarded automation action %s", action.ID)
		}
		if _, _, _, _, payloadErr := automationActionPayload(action); payloadErr != nil {
			return payloadErr
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
	baseRevision, projectEpoch := baseRevision, cut.ProjectEpoch
	valueMin, valueMax, beforeCount := map[string]float64{}, map[string]float64{}, map[string]int{}
	for _, action := range actionSet.Actions {
		pluginID, paramID, points, _, payloadErr := automationActionPayload(action)
		if payloadErr != nil {
			p.mu.Unlock()
			return payloadErr
		}
		minValue, maxValue, pointCount, rangeErr := p.readParameterRange(ctx, action.TargetRef, pluginID, paramID)
		if rangeErr != nil {
			p.mu.Unlock()
			return rangeErr
		}
		for index, point := range points {
			if point.V < minValue || point.V > maxValue {
				p.mu.Unlock()
				return fmt.Errorf("automation envelope point #%d v=%g lies outside the parameter raw value range %s",
					index, point.V, experiment.AutomationEnvelopeValueRangeHint(minValue, maxValue))
			}
		}
		key := automationParamKey(pluginID, paramID)
		valueMin[key], valueMax[key], beforeCount[key] = minValue, maxValue, pointCount
	}
	p.baseRevision, p.projectEpoch = baseRevision, projectEpoch
	p.valueMin, p.valueMax, p.beforeCount = valueMin, valueMax, beforeCount
	p.mu.Unlock()
	return nil
}

func (p *AutomationEnvelopeVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("VSP preflight has not completed")
	}
	pluginID, paramID, points, region, err := automationActionPayload(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, err
	}
	key := automationParamKey(pluginID, paramID)
	minValue, hasMin := p.valueMin[key]
	maxValue, hasMax := p.valueMax[key]
	if hasMin && hasMax {
		for index, point := range points {
			if point.V < minValue || point.V > maxValue {
				return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"},
					fmt.Errorf("automation envelope point #%d v=%g lies outside the parameter raw value range %s",
						index, point.V, experiment.AutomationEnvelopeValueRangeHint(minValue, maxValue))
			}
		}
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	txID := "tx_" + requestID
	command := map[string]any{
		"cmd":       "automation.set_points",
		"plugin_id": pluginID,
		"param_id":  paramID,
		"points":    experiment.AutomationEnvelopePointsMaps(points),
	}
	if region.End > region.Start {
		command["region"] = map[string]any{"start": region.Start, "end": region.End}
	}
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, command, requestID, txID)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	legacy := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(legacy["status"])), "error") {
		message := strings.TrimSpace(fmt.Sprint(legacy["message"]))
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: message}, fmt.Errorf("VSP automation write failed: %s", message)
	}
	written := 0
	if raw, ok := numeric(legacy["written"]); ok {
		written = int(raw)
	}
	if written != len(points) {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed",
				Error: fmt.Sprintf("automation write receipt written=%d does not match the %d submitted points", written, len(points))},
			fmt.Errorf("automation write receipt written=%d does not match the %d submitted points", written, len(points))
	}
	readback, readErr := p.readCurveUnlocked(ctx, pluginID, paramID)
	if readErr != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true}, readErr
	}
	if matchErr := experiment.AutomationEnvelopeCurveMatches(points, readback); matchErr != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true},
			fmt.Errorf("automation envelope readback mismatch: %w", matchErr)
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
	beforeRevision := p.baseRevision
	beforeCount := p.beforeCount[key]
	transactionID := strings.TrimSpace(result.TransactionID)
	if transactionID == "" {
		transactionID = txID
	}
	mode := "full"
	if region.End > region.Start {
		mode = "region"
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
			"plugin_id": pluginID, "param_id": paramID, "mode": mode,
			"written": written, "point_count": len(points), "before_point_count": beforeCount,
			"region_start_s": region.Start, "region_end_s": region.End,
			"readback_verified": true, "readback_tolerance": experiment.D1S1AutomationReadbackTol,
		},
	}, nil
}

func (p *AutomationEnvelopeVSPPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	if p == nil || p.Client == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("VSP client is required")
	}
	pluginID, paramID, points, _, err := automationActionPayload(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, err
	}
	readback, readErr := p.readCurveUnlocked(ctx, pluginID, paramID)
	if readErr != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, readErr
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("read reconcile snapshot: %w", err)
	}
	baseRevision, parseErr := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if parseErr != nil || baseRevision <= 0 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("invalid reconcile base revision")
	}
	if matchErr := experiment.AutomationEnvelopeCurveMatches(points, readback); matchErr != nil || current.Revision <= baseRevision {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true,
		EvidenceRefs: []string{"vsp.reconcile:" + idempotencyKey, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details: map[string]any{
			"plugin_id": pluginID, "param_id": paramID, "point_count": len(points),
			"readback_verified": true, "reconciled": true,
		},
	}, nil
}

func (p *AutomationEnvelopeVSPPort) readCurveUnlocked(ctx context.Context, pluginID, paramID string) ([]map[string]any, error) {
	// The readCurve body does not touch the mutex-guarded fields; it is split
	// from readCurve only so Apply can reuse it while holding the lock.
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":       "automation.get_curve",
		"plugin_id": pluginID,
		"param_id":  paramID,
	}, "read:curve:"+automationParamKey(pluginID, paramID), "tx_read_curve:"+automationParamKey(pluginID, paramID))
	if err != nil {
		return nil, fmt.Errorf("automation curve readback failed: %w", err)
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		return nil, fmt.Errorf("automation curve readback failed: %s", strings.TrimSpace(fmt.Sprint(reply["message"])))
	}
	points, _ := reply["points"].([]any)
	out := make([]map[string]any, 0, len(points))
	for _, row := range points {
		if entry, ok := row.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out, nil
}
