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
	"vit-daw-agent/internal/spal"
)

// SPALVSPClient is intentionally typed. The port uses one read-only legacy
// command to capture/verify the plug-in state, and the typed VSP batch command
// for every actual semantic parameter write and compensation.
type SPALVSPClient interface {
	VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error)
	SendVSPLegacyCommandWithIDs(context.Context, map[string]any, string, string) (*kernel.VSPCommandResult, error)
	SendVSPCommandWithIDs(context.Context, string, map[string]any, string, string) (*kernel.VSPCommandResult, error)
}

// SPALVSPPort applies a frozen SPAL Manifest. The VSP kernel's parameter batch
// is ordered rather than atomic, so this port restores the complete preimage
// after any partial write or readback mismatch.
type SPALVSPPort struct {
	Client SPALVSPClient

	mu           sync.Mutex
	baseRevision int64
	projectEpoch string
}

// SPALVSPControlledRoundtripPort is the non-persistent Forge acceptance
// transport. It uses the exact same frozen Manifest and guarded parameter
// writes as production, but restores the complete frozen preimage before the
// action returns. It is deliberately a separate port so a production SPAL
// execution can never become auto-restoring by accident.
//
// The successful receipt still describes the observed apply postimage and
// adds the independently fresh-read back restoration evidence. Callers must
// use a verifier that requires the controlled-roundtrip fields.
type SPALVSPControlledRoundtripPort struct {
	Client SPALVSPClient
	port   *SPALVSPPort
}

func (p *SPALVSPControlledRoundtripPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.Client == nil {
		return fmt.Errorf("SPAL controlled-roundtrip VSP client is required")
	}
	p.port = &SPALVSPPort{Client: p.Client}
	return p.port.Preflight(ctx, actionSet, cut)
}

func (p *SPALVSPControlledRoundtripPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	if p == nil || p.port == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("SPAL controlled-roundtrip preflight has not completed")
	}
	return p.port.ApplyControlledRoundtrip(ctx, action, idempotencyKey)
}

// SPALVSPInspector is the read-only runtime service used before a Proposal is
// frozen. It captures the physical preimage without leaking it into the
// capability-layer instruction.
type SPALVSPInspector struct {
	Client SPALVSPClient
}

func (i SPALVSPInspector) CaptureSPALPreimage(ctx context.Context, binding spal.RuntimeBinding, writes []spal.PhysicalParameter) ([]spal.PhysicalParameter, error) {
	if i.Client == nil {
		return nil, fmt.Errorf("SPAL VSP client is required")
	}
	manifest := spal.ExecutionManifest{Binding: binding, Writes: writes}
	port := SPALVSPPort{Client: i.Client}
	return port.readParameters(ctx, manifest, "spal:manifest-preimage:"+binding.ID)
}

func (p *SPALVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.Client == nil {
		return fmt.Errorf("SPAL VSP client is required")
	}
	if !cut.IsExecutable() || actionSet.ProjectCutHash != cut.Hash {
		return fmt.Errorf("strong matching ProjectCut is required")
	}
	baseRevision, err := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if err != nil || baseRevision <= 0 {
		return fmt.Errorf("valid base project revision is required")
	}
	if len(actionSet.Actions) == 0 {
		return fmt.Errorf("SPAL action set is empty")
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return fmt.Errorf("read current VSP snapshot: %w", err)
	}
	if current.ProjectEpoch != cut.ProjectEpoch || current.Revision != baseRevision {
		return fmt.Errorf("stale_project_cut: current epoch/revision does not match proposal")
	}
	for _, action := range actionSet.Actions {
		manifest, err := spal.ManifestFromAction(action)
		if err != nil {
			return err
		}
		if manifest.Instruction.TargetRef != action.TargetRef || strings.TrimSpace(action.BeforeFingerprint) == "" {
			return fmt.Errorf("SPAL action %s is unguarded or targets a different binding", action.ID)
		}
		actual, err := p.readParameters(ctx, manifest, "spal:preflight:"+action.ID)
		if err != nil {
			return fmt.Errorf("read SPAL preimage for %s: %w", action.ID, err)
		}
		if err := spalSameParameters(manifest.Preimage, actual); err != nil {
			return fmt.Errorf("stale SPAL preimage for %s: %w", action.ID, err)
		}
	}
	p.mu.Lock()
	p.baseRevision = baseRevision
	p.projectEpoch = cut.ProjectEpoch
	p.mu.Unlock()
	return nil
}

func (p *SPALVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	return p.apply(ctx, action, idempotencyKey, false)
}

// ApplyControlledRoundtrip makes the requested mutation observable through a
// normal fresh post-write readback, then immediately restores the frozen full
// preimage with another fresh readback. It is only used by the Forge acceptance
// port above, never by the production mutation port.
func (p *SPALVSPPort) ApplyControlledRoundtrip(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	return p.apply(ctx, action, idempotencyKey, true)
}

func (p *SPALVSPPort) apply(ctx context.Context, action orchestration.Action, idempotencyKey string, controlledRoundtrip bool) (orchestration.ActionReceipt, error) {
	if p == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("SPAL VSP port is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.Client == nil || p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("SPAL VSP preflight has not completed")
	}
	manifest, err := spal.ManifestFromAction(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	result, err := p.sendBatch(ctx, manifest, manifest.Writes, p.baseRevision, requestID, "tx_"+requestID)
	if err != nil || spalCommandFailed(result) {
		rollback := p.compensate(ctx, manifest, requestID, firstSPALError(err, result))
		receipt := p.failedReceipt(action.ID, manifest, rollback, firstSPALError(err, result))
		return receipt, fmt.Errorf("SPAL parameter batch failed: %s", firstSPALError(err, result))
	}
	postimage, readErr := p.readParameters(ctx, manifest, requestID+":readback")
	if readErr == nil {
		readErr = spalSameParameters(manifest.Writes, postimage)
	}
	if readErr != nil {
		rollback := p.compensate(ctx, manifest, requestID, readErr.Error())
		receipt := p.failedReceipt(action.ID, manifest, rollback, "post-write readback: "+readErr.Error())
		return receipt, fmt.Errorf("SPAL structural readback failed: %w", readErr)
	}
	current, snapshotErr := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if snapshotErr != nil || current == nil || !current.OK() {
		details := spalReceiptDetails(manifest, manifest.Preimage, postimage, nil, result)
		details["structural_readback"] = "pass"
		return orchestration.ActionReceipt{
			ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true,
			Error: firstSPALError(snapshotErr, nil), Details: details,
		}, fmt.Errorf("SPAL mutation applied but project revision readback failed: %w", snapshotErr)
	}
	if current.ProjectEpoch != p.projectEpoch {
		details := spalReceiptDetails(manifest, manifest.Preimage, postimage, nil, result)
		details["structural_readback"] = "pass"
		return orchestration.ActionReceipt{
			ActionID: action.ID, Status: "applied_unreconciled", EffectivelyOnce: true,
			Error: "project epoch changed during SPAL execution", Details: details,
		}, fmt.Errorf("project epoch changed during SPAL execution")
	}
	p.baseRevision = current.Revision
	details := spalReceiptDetails(manifest, manifest.Preimage, postimage, nil, result)
	details["structural_readback"] = "pass"
	if controlledRoundtrip {
		restored := p.compensate(ctx, manifest, requestID+":controlled-roundtrip", "Forge controlled acceptance cleanup")
		details["controlled_roundtrip"] = true
		details["controlled_roundtrip_apply_postimage"] = postimage
		details["controlled_roundtrip_restore_status"] = restored.Status
		details["controlled_roundtrip_restore_postimage"] = restored.Postimage
		if restored.Status != "restored" {
			details["structural_readback"] = "failed"
			details["rollback_status"] = restored.Status
			details["rollback_error"] = restored.Error
			return orchestration.ActionReceipt{
				ActionID: action.ID, Status: "partial_failure", Error: "controlled roundtrip cleanup: " + restored.Error,
				EvidenceRefs: uniqueSPALRefs(manifest.EvidenceRefs), Details: details,
			}, fmt.Errorf("SPAL controlled-roundtrip restore failed: %s", restored.Error)
		}
		details["controlled_roundtrip_restored_preimage"] = true
	}
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true,
		EvidenceRefs: uniqueSPALRefs(manifest.EvidenceRefs),
		Details:      details,
	}, nil
}

func (p *SPALVSPPort) Reconcile(ctx context.Context, action orchestration.Action, _ string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	if p == nil || p.Client == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("SPAL VSP client is required")
	}
	manifest, err := spal.ManifestFromAction(action)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown", Error: err.Error()}, err
	}
	state, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("read SPAL reconciliation snapshot: %w", err)
	}
	if state.ProjectEpoch != cut.ProjectEpoch {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("project epoch changed before SPAL reconciliation")
	}
	actual, err := p.readParameters(ctx, manifest, "spal:reconcile:"+action.ID)
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown", Error: err.Error()}, err
	}
	if err := spalSameParameters(manifest.Writes, actual); err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(state.Revision, 10), Details: spalReceiptDetails(manifest, manifest.Preimage, actual, nil, nil)}, nil
	}
	details := spalReceiptDetails(manifest, manifest.Preimage, actual, nil, nil)
	details["structural_readback"] = "pass"
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(state.Revision, 10), EffectivelyOnce: true,
		EvidenceRefs: uniqueSPALRefs(manifest.EvidenceRefs), Details: details,
	}, nil
}

type spalRollbackResult struct {
	Status    string
	Postimage []spal.PhysicalParameter
	Error     string
	Result    *kernel.VSPCommandResult
}

func (p *SPALVSPPort) compensate(ctx context.Context, manifest spal.ExecutionManifest, requestID, cause string) spalRollbackResult {
	state, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() || state.ProjectEpoch != p.projectEpoch {
		return spalRollbackResult{Status: "unknown", Error: "cannot establish rollback revision: " + firstSPALError(err, nil)}
	}
	actual, err := p.readParameters(ctx, manifest, requestID+":rollback-safety")
	if err != nil {
		return spalRollbackResult{Status: "unknown", Error: "cannot read rollback safety state: " + err.Error()}
	}
	if err := spalRollbackSafe(actual, manifest.Writes, manifest.Preimage); err != nil {
		return spalRollbackResult{Status: "unsafe", Postimage: actual, Error: err.Error()}
	}
	result, err := p.sendBatch(ctx, manifest, manifest.Preimage, state.Revision, requestID+":rollback", "tx_"+requestID+":rollback")
	if err != nil || spalCommandFailed(result) {
		return spalRollbackResult{Status: "failed", Error: firstSPALError(err, result), Result: result}
	}
	postimage, err := p.readParameters(ctx, manifest, requestID+":rollback-readback")
	if err != nil {
		return spalRollbackResult{Status: "unknown", Error: err.Error(), Result: result}
	}
	if err := spalSameParameters(manifest.Preimage, postimage); err != nil {
		return spalRollbackResult{Status: "failed", Postimage: postimage, Error: err.Error(), Result: result}
	}
	return spalRollbackResult{Status: "restored", Postimage: postimage, Result: result}
}

func (p *SPALVSPPort) failedReceipt(actionID string, manifest spal.ExecutionManifest, rollback spalRollbackResult, failure string) orchestration.ActionReceipt {
	status := "partial_failure"
	if rollback.Status == "restored" {
		status = "rolled_back"
	}
	details := spalReceiptDetails(manifest, manifest.Preimage, rollback.Postimage, &rollback, rollback.Result)
	if rollback.Status == "restored" {
		details["structural_readback"] = "rolled_back"
	} else {
		details["structural_readback"] = "failed"
	}
	return orchestration.ActionReceipt{
		ActionID: actionID, Status: status, Error: failure,
		EvidenceRefs: uniqueSPALRefs(manifest.EvidenceRefs),
		Details:      details,
	}
}

func (p *SPALVSPPort) sendBatch(ctx context.Context, manifest spal.ExecutionManifest, parameters []spal.PhysicalParameter, baseRevision int64, requestID, transactionID string) (*kernel.VSPCommandResult, error) {
	rows := make([]any, 0, len(parameters))
	for _, parameter := range parameters {
		if parameter.RequiresKernelDisplayResolution() {
			return nil, fmt.Errorf("semantic display/enum write for %s must be delegated to governed B4 plugin control", parameter.ParameterID)
		}
		row := map[string]any{"parameter_id": parameter.ParameterID, "value": parameter.Value}
		if parameter.Unit != "" {
			row["unit"] = parameter.Unit
		}
		rows = append(rows, row)
	}
	return p.Client.SendVSPCommandWithIDs(ctx, "plugin.set_params_batch", map[string]any{
		"track_id":      manifest.Binding.Instance.TrackID,
		"plugin_id":     manifest.Binding.Instance.PluginID,
		"base_revision": baseRevision,
		"readback":      true,
		"parameters":    rows,
	}, requestID, transactionID)
}

func (p *SPALVSPPort) readParameters(ctx context.Context, manifest spal.ExecutionManifest, requestID string) ([]spal.PhysicalParameter, error) {
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd": "get_plugin_parameters", "track_id": manifest.Binding.Instance.TrackID, "plugin_id": manifest.Binding.Instance.PluginID,
	}, requestID, "")
	if err != nil {
		return nil, err
	}
	legacy := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(legacy["status"])), "error") {
		return nil, fmt.Errorf("get_plugin_parameters failed: %s", strings.TrimSpace(fmt.Sprint(legacy["message"])))
	}
	return spalReadbackParameters(legacy, manifest.Writes)
}

func spalReadbackParameters(reply map[string]any, requested []spal.PhysicalParameter) ([]spal.PhysicalParameter, error) {
	rows := spalRows(reply["parameters"])
	if len(rows) == 0 {
		rows = spalRows(reply["changed_params"])
	}
	indexed := map[string]map[string]any{}
	for _, row := range rows {
		id := strings.TrimSpace(fmt.Sprint(spalFirst(row, "parameter_id", "param_id", "id")))
		if id != "" && id != "<nil>" {
			indexed[id] = row
		}
	}
	out := make([]spal.PhysicalParameter, 0, len(requested))
	for _, expected := range requested {
		row := indexed[expected.ParameterID]
		if row == nil {
			return nil, fmt.Errorf("readback omitted parameter %s", expected.ParameterID)
		}
		value, ok := spalNumber(spalFirst(row, "value", "new_value", "actual_value", "normalized_value", "new_normalised_value", "new_normalized_value", "actual_normalized_value"))
		if !ok {
			return nil, fmt.Errorf("readback parameter %s has no numeric value", expected.ParameterID)
		}
		out = append(out, spal.PhysicalParameter{ParameterID: expected.ParameterID, Value: value, Unit: expected.Unit})
	}
	return out, nil
}

func spalRows(value any) []map[string]any {
	if rows, ok := value.([]map[string]any); ok {
		return rows
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var rows []map[string]any
	if json.Unmarshal(data, &rows) == nil {
		return rows
	}
	return nil
}

func spalFirst(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func spalNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return spalNumber(float64(typed))
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func spalSameParameters(expected, actual []spal.PhysicalParameter) error {
	indexed := map[string]spal.PhysicalParameter{}
	for _, value := range actual {
		indexed[value.ParameterID] = value
	}
	for _, want := range expected {
		got, ok := indexed[want.ParameterID]
		if !ok {
			return fmt.Errorf("parameter %s missing", want.ParameterID)
		}
		if math.Abs(got.Value-want.Value) > 0.0001 {
			return fmt.Errorf("parameter %s = %.6f, want %.6f", want.ParameterID, got.Value, want.Value)
		}
	}
	return nil
}

// A compensation may overwrite a value only when the current state is either
// the frozen preimage or the exact write this execution attempted. Any third
// value indicates a concurrent/manual mutation and must fail closed rather
// than clobber it during rollback.
func spalRollbackSafe(actual, writes, preimage []spal.PhysicalParameter) error {
	writeIndex := map[string]spal.PhysicalParameter{}
	beforeIndex := map[string]spal.PhysicalParameter{}
	actualIndex := map[string]spal.PhysicalParameter{}
	for _, value := range writes {
		writeIndex[value.ParameterID] = value
	}
	for _, value := range preimage {
		beforeIndex[value.ParameterID] = value
	}
	for _, value := range actual {
		actualIndex[value.ParameterID] = value
	}
	for parameterID, before := range beforeIndex {
		current, ok := actualIndex[parameterID]
		if !ok {
			return fmt.Errorf("rollback safety readback omitted parameter %s", parameterID)
		}
		write := writeIndex[parameterID]
		if math.Abs(current.Value-before.Value) <= 0.0001 || math.Abs(current.Value-write.Value) <= 0.0001 {
			continue
		}
		return fmt.Errorf("rollback unsafe: parameter %s is %.6f, neither frozen preimage %.6f nor requested write %.6f", parameterID, current.Value, before.Value, write.Value)
	}
	return nil
}

func spalCommandFailed(result *kernel.VSPCommandResult) bool {
	if result == nil {
		return true
	}
	if strings.Contains(strings.ToLower(fmt.Sprint(result.Response["type"])), "error") {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(result.Payload["status"])))
	return status != "" && status != "ok"
}

func firstSPALError(err error, result *kernel.VSPCommandResult) string {
	if err != nil {
		return err.Error()
	}
	if result != nil {
		if legacy := result.LegacyLikeReply(); strings.TrimSpace(fmt.Sprint(legacy["message"])) != "" {
			return strings.TrimSpace(fmt.Sprint(legacy["message"]))
		}
		if errorRow, ok := result.Response["error"].(map[string]any); ok {
			if message := strings.TrimSpace(fmt.Sprint(errorRow["message"])); message != "" {
				return message
			}
		}
	}
	return "unknown SPAL VSP failure"
}

func spalReceiptDetails(manifest spal.ExecutionManifest, preimage, postimage []spal.PhysicalParameter, rollback *spalRollbackResult, result *kernel.VSPCommandResult) map[string]any {
	details := map[string]any{
		"spal_schema":         manifest.Instruction.SchemaID,
		"spal_operation":      manifest.Operation,
		"spal_manifest_id":    manifest.ID,
		"provider_id":         manifest.Binding.Provider.ID,
		"adapter_version":     manifest.Binding.Provider.AdapterVersion,
		"provider_signature":  manifest.Binding.Provider.ProfileSignature,
		"binding_id":          manifest.Binding.ID,
		"plugin_id":           manifest.Binding.Instance.PluginID,
		"track_id":            manifest.Binding.Instance.TrackID,
		"preimage":            preimage,
		"postimage":           postimage,
		"structural_readback": "inconclusive",
		"rollback_contract":   "restore_complete_physical_preimage",
	}
	if manifest.RollbackOf != "" {
		details["spal_rollback_of"] = manifest.RollbackOf
	}
	if rollback != nil {
		details["rollback_status"] = rollback.Status
		details["rollback_error"] = rollback.Error
	}
	if result != nil {
		details["transaction_id"] = result.TransactionID
		details["changed_params"] = result.Payload["changed_params"]
	}
	return details
}

func uniqueSPALRefs(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
