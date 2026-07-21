package executionports

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

const pluginGrabberReadbackTolerance = 1e-4

type PluginGrabberClient interface {
	SendVSPCommandWithIDs(context.Context, string, map[string]any, string, string) (*kernel.VSPCommandResult, error)
}

type pluginResolvedParameter struct {
	Slot               string  `json:"slot,omitempty"`
	ParameterID        string  `json:"parameter_id"`
	ParameterName      string  `json:"parameter_name,omitempty"`
	OldValue           float64 `json:"old_value,omitempty"`
	OldNormalizedValue float64 `json:"old_normalized_value"`
	OldValueText       string  `json:"old_value_text,omitempty"`
}
type PluginGrabberPort struct {
	Client    PluginGrabberClient
	Tolerance float64
}

func (p PluginGrabberPort) Preflight(_ context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p.Client == nil {
		return fmt.Errorf("plugin grabber client is required")
	}
	if actionSet.CapabilityID != "plugin.effect_control.v0" {
		return fmt.Errorf("unexpected capability %q", actionSet.CapabilityID)
	}
	if actionSet.Hash == "" || actionSet.Hash != actionSet.ComputeHash() || actionSet.ProjectCutHash != cut.Hash || !cut.IsExecutable() {
		return fmt.Errorf("frozen action set/project cut is invalid")
	}
	if len(actionSet.Actions) != 1 {
		return fmt.Errorf("plugin effect control requires exactly one action")
	}
	action := actionSet.Actions[0]
	if action.Command != "plugin_grabber.apply_control.governed" || !action.Compensatable {
		return fmt.Errorf("unsupported or non-compensatable plugin effect action")
	}
	_, _, _, preimage, expected, err := pluginGrabberAction(action)
	if err != nil {
		return err
	}
	if len(preimage) == 0 || len(expected) == 0 {
		return fmt.Errorf("frozen parameter preimage and identities are required")
	}
	return nil
}

func (p PluginGrabberPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	trackID, pluginID, applyArgs, preimage, expectedIDs, err := pluginGrabberAction(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	requestID := idempotencyKey + ":apply"
	transactionID := idempotencyKey + ":tx"
	result, applyErr := p.Client.SendVSPCommandWithIDs(ctx, "plugin.apply_control", applyArgs, requestID, transactionID)
	applied := pluginRows(pluginReply(result)["applied_parameters"])
	failure := firstPluginFailure(applyErr, result)
	actualIDs := pluginParameterIDs(applied)
	if failure == "" && !samePluginIDs(expectedIDs, actualIDs) {
		failure = fmt.Sprintf("parameter identity mismatch: expected %v, applied %v", expectedIDs, actualIDs)
	}

	var readback []map[string]any
	if failure == "" {
		readResult, readErr := p.Client.SendVSPCommandWithIDs(ctx, "plugin.parameters.get", map[string]any{
			"track_id": trackID, "plugin_id": pluginID, "include_parameters": true,
		}, idempotencyKey+":readback", transactionID)
		if readErr != nil {
			failure = "fresh parameter readback failed: " + readErr.Error()
		} else if commandFailure(readResult) != "" {
			failure = "fresh parameter readback failed: " + commandFailure(readResult)
		} else {
			readback = pluginRows(pluginReply(readResult)["parameters"])
			if err := comparePluginReadback(applied, readback, p.tolerance()); err != nil {
				failure = err.Error()
			}
		}
	}

	details := map[string]any{
		"track_id": trackID, "plugin_id": pluginID,
		"expected_parameter_ids": expectedIDs,
		"applied_parameter_ids":  actualIDs,
		"preimage":               preimage,
		"applied_parameters":     applied,
		"fresh_readback":         readback,
		"structural_readback":    "pass",
		"apply_transaction_id":   transactionID,
	}
	if failure == "" {
		return orchestration.ActionReceipt{
			ActionID: action.ID, Status: "applied", EffectivelyOnce: true,
			AppliedRevision: pluginRevision(result),
			EvidenceRefs:    []string{"plugin.apply_control:" + transactionID, "plugin.parameters.get:" + transactionID},
			Details:         details,
		}, nil
	}

	rollback := p.rollback(ctx, trackID, pluginID, preimage, idempotencyKey, transactionID)
	details["structural_readback"] = "fail"
	details["failure"] = failure
	details["rollback"] = rollback
	receipt := orchestration.ActionReceipt{
		ActionID: action.ID, Status: "failed", Error: failure, EffectivelyOnce: false,
		EvidenceRefs: []string{"plugin.apply_control:" + transactionID, "plugin.rollback:" + transactionID},
		Details:      details,
	}
	if !rollback.OK {
		receipt.Status = "failed_closed"
		receipt.Error = failure + "; compensation failed: " + rollback.Error
		return receipt, fmt.Errorf("%s", receipt.Error)
	}
	return receipt, fmt.Errorf("%s; full preimage restored", failure)
}

func (p PluginGrabberPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	// A semantic apply must never be blindly retried after a crash. Read the
	// frozen identities and accept only an already-matching postimage; otherwise
	// restore the full preimage and fail closed.
	trackID, pluginID, _, preimage, expectedIDs, err := pluginGrabberAction(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	readResult, readErr := p.Client.SendVSPCommandWithIDs(ctx, "plugin.parameters.get", map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "include_parameters": true,
	}, idempotencyKey+":reconcile-read", idempotencyKey+":reconcile")
	readback := pluginRows(pluginReply(readResult)["parameters"])
	readbackFailure := firstPluginFailure(readErr, readResult)
	// The frozen action contains a complete preimage but not a trustworthy
	// postimage. Parameter identity presence alone cannot prove that the semantic
	// control completed, so an ambiguous outcome is always compensated.
	rollback := p.rollback(ctx, trackID, pluginID, preimage, idempotencyKey, idempotencyKey+":reconcile")
	details := map[string]any{
		"expected_parameter_ids": expectedIDs,
		"fresh_readback":         readback,
		"fresh_readback_error":   readbackFailure,
		"rollback":               rollback,
	}
	receipt := orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Details: details}
	if !rollback.OK {
		receipt.Status = "failed_closed"
		receipt.Error = rollback.Error
		return receipt, fmt.Errorf("reconcile compensation failed: %s", rollback.Error)
	}
	receipt.Error = "execution outcome was ambiguous; full preimage restored"
	return receipt, fmt.Errorf("%s", receipt.Error)
}

type PluginRollbackResult struct {
	OK       bool             `json:"ok"`
	Error    string           `json:"error,omitempty"`
	Readback []map[string]any `json:"readback,omitempty"`
}

func (p PluginGrabberPort) rollback(ctx context.Context, trackID, pluginID string, preimage []pluginResolvedParameter, idempotencyKey, transactionID string) PluginRollbackResult {
	parameters := make([]map[string]any, 0, len(preimage))
	for _, item := range preimage {
		parameters = append(parameters, map[string]any{
			"parameter_id": item.ParameterID, "normalized_value": item.OldNormalizedValue,
		})
	}
	result, err := p.Client.SendVSPCommandWithIDs(ctx, "plugin.set_params_batch", map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "parameters": parameters, "readback": true,
	}, idempotencyKey+":rollback", transactionID+":rollback")
	if err != nil {
		return PluginRollbackResult{Error: err.Error()}
	}
	if failure := commandFailure(result); failure != "" {
		return PluginRollbackResult{Error: failure}
	}
	reply := pluginReply(result)
	readback := pluginRows(pluginMap(reply["readback"])["parameters"])
	if len(readback) == 0 {
		readback = pluginRows(reply["parameters"])
	}
	if err := comparePluginPreimage(preimage, readback, p.tolerance()); err != nil {
		return PluginRollbackResult{Error: err.Error(), Readback: readback}
	}
	return PluginRollbackResult{OK: true, Readback: readback}
}

func (p PluginGrabberPort) tolerance() float64 {
	if p.Tolerance > 0 {
		return p.Tolerance
	}
	return pluginGrabberReadbackTolerance
}

func pluginGrabberAction(action orchestration.Action) (string, string, map[string]any, []pluginResolvedParameter, []string, error) {
	trackID := pluginString(action.Args["track_id"])
	pluginID := pluginString(action.Args["plugin_id"])
	if trackID == "" || pluginID == "" {
		return "", "", nil, nil, nil, fmt.Errorf("frozen action is missing track_id or plugin_id")
	}
	applyArgs := clonePluginArgs(pluginMap(action.Args["apply_args"]))
	if len(applyArgs) == 0 {
		applyArgs = map[string]any{}
	}
	applyArgs["track_id"] = trackID
	applyArgs["plugin_id"] = pluginID
	if control := pluginString(action.Args["control"]); control != "" {
		applyArgs["control"] = control
	}
	delete(applyArgs, "resolve_only")
	delete(applyArgs, "preview")

	var preimage []pluginResolvedParameter
	raw, err := json.Marshal(action.Args["resolved_parameters"])
	if err == nil {
		err = json.Unmarshal(raw, &preimage)
	}
	if err != nil {
		return "", "", nil, nil, nil, fmt.Errorf("decode frozen preimage: %w", err)
	}
	expected := pluginStrings(action.Args["expected_parameter_ids"])
	return trackID, pluginID, applyArgs, preimage, expected, nil
}

func comparePluginReadback(applied, readback []map[string]any, tolerance float64) error {
	index := pluginRowIndex(readback)
	if len(applied) == 0 {
		return fmt.Errorf("apply response omitted applied_parameters")
	}
	for _, item := range applied {
		id := pluginParameterID(item)
		actual, ok := index[id]
		if !ok {
			return fmt.Errorf("fresh readback omitted parameter_id %s", id)
		}
		expectedValue, expectedOK := pluginNumberFirst(item, "new_normalised_value", "new_normalized_value", "actual_normalised_value", "actual_normalized_value")
		actualValue, actualOK := pluginNumberFirst(actual, "normalized_value", "normalised_value", "current_normalised_value", "current_normalized_value")
		if expectedOK && actualOK && math.Abs(expectedValue-actualValue) > tolerance {
			return fmt.Errorf("parameter %s readback mismatch: expected %.9f actual %.9f tolerance %.9f", id, expectedValue, actualValue, tolerance)
		}
		if !expectedOK || !actualOK {
			expectedText := pluginStringFirst(item, "new_value_text", "display_text")
			actualText := pluginStringFirst(actual, "value_text", "current_value_text", "text")
			if expectedText == "" || actualText == "" || !strings.EqualFold(strings.TrimSpace(expectedText), strings.TrimSpace(actualText)) {
				return fmt.Errorf("parameter %s has no comparable fresh readback", id)
			}
		}
	}
	return nil
}

func comparePluginPreimage(preimage []pluginResolvedParameter, readback []map[string]any, tolerance float64) error {
	index := pluginRowIndex(readback)
	if len(index) == 0 {
		return fmt.Errorf("rollback readback is missing")
	}
	for _, item := range preimage {
		row, ok := index[item.ParameterID]
		if !ok {
			return fmt.Errorf("rollback readback omitted parameter_id %s", item.ParameterID)
		}
		actual, ok := pluginNumberFirst(row, "normalized_value", "normalised_value", "current_normalised_value", "current_normalized_value")
		if !ok || math.Abs(item.OldNormalizedValue-actual) > tolerance {
			return fmt.Errorf("rollback mismatch for %s: expected %.9f", item.ParameterID, item.OldNormalizedValue)
		}
	}
	return nil
}

func pluginReply(result *kernel.VSPCommandResult) map[string]any {
	if result == nil {
		return nil
	}
	for _, candidate := range []map[string]any{result.LegacyReply, result.Payload, pluginMap(result.Payload["legacy_reply"]), pluginMap(result.Payload["result"]), result.Response} {
		if len(candidate) > 0 {
			if nested := pluginMap(candidate["payload"]); len(nested) > 0 && (nested["applied_parameters"] != nil || nested["parameters"] != nil || nested["readback"] != nil) {
				return nested
			}
			if candidate["applied_parameters"] != nil || candidate["resolved_parameters"] != nil || candidate["parameters"] != nil || candidate["readback"] != nil {
				return candidate
			}
		}
	}
	return result.LegacyReply
}

func firstPluginFailure(err error, result *kernel.VSPCommandResult) string {
	if err != nil {
		return err.Error()
	}
	return commandFailure(result)
}

func commandFailure(result *kernel.VSPCommandResult) string {
	if result == nil {
		return "kernel returned no result"
	}
	for _, row := range []map[string]any{result.Response, result.Payload, result.LegacyReply} {
		status := strings.ToLower(pluginStringFirst(row, "status", "stage", "type"))
		if strings.Contains(status, "error") || status == "failed" || status == "partial_failure" || status == "rejected" {
			return pluginStringFirst(row, "message", "error", "status")
		}
		if errRow := pluginMap(row["error"]); len(errRow) > 0 {
			return pluginStringFirst(errRow, "message", "code")
		}
	}
	return ""
}

func pluginRevision(result *kernel.VSPCommandResult) string {
	if result == nil || result.Revision <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", result.Revision)
}

func pluginRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if item := pluginMap(row); len(item) > 0 {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func pluginMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if row, ok := value.(map[string]any); ok {
		return row
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return nil
	}
	return row
}

func pluginStrings(value any) []string {
	switch values := value.(type) {
	case []string:
		return canonicalPluginIDs(values)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			out = append(out, pluginString(value))
		}
		return canonicalPluginIDs(out)
	default:
		if text := pluginString(value); text != "" {
			return []string{text}
		}
		return nil
	}
}

func pluginParameterIDs(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, pluginParameterID(row))
	}
	return canonicalPluginIDs(out)
}

func pluginParameterID(row map[string]any) string {
	return pluginStringFirst(row, "parameter_id", "param_id", "id")
}

func canonicalPluginIDs(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func samePluginIDs(left, right []string) bool {
	left, right = canonicalPluginIDs(left), canonicalPluginIDs(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func pluginRowsContainIDs(rows []map[string]any, ids []string) bool {
	index := pluginRowIndex(rows)
	for _, id := range ids {
		if _, ok := index[id]; !ok {
			return false
		}
	}
	return len(ids) > 0
}

func pluginRowIndex(rows []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		if id := pluginParameterID(row); id != "" {
			out[id] = row
		}
	}
	return out
}

func pluginNumberFirst(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		switch value := row[key].(type) {
		case float64:
			if !math.IsNaN(value) && !math.IsInf(value, 0) {
				return value, true
			}
		case float32:
			return float64(value), true
		case int:
			return float64(value), true
		case int64:
			return float64(value), true
		case json.Number:
			if number, err := value.Float64(); err == nil {
				return number, true
			}
		}
	}
	return 0, false
}

func pluginStringFirst(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := pluginString(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func pluginString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func clonePluginArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
