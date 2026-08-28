package executionports

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

// staticEQActionCommand is the historical D2-1 domain command accepted by a
// zero-value StaticEQVSPPort; constructor injection can narrow it further
// without changing CAS/idempotency semantics.
const staticEQActionCommand = "static_eq_band_adjust"

// StaticEQVSPPort executes one bounded static EQ band adjustment as a single
// governed forward mutation. One orchestration Action may need two kernel
// commands (instantiate the EQ instance, then set one band parameter) plus a
// readback command; they share one idempotency key and produce one receipt,
// preserving the D-family "exactly one forward mutation" invariant. The port
// contains no authorization logic.
//
// The port is parameter driven: CommandName is injected by construction
// instead of hardcoding the domain action, and an action whose Args carry
// WriteModeNormalizedBatchV1 switches the write pipeline to the real-plugin
// path (path instantiate + kernel-normalized dual-channel batch +
// value_text physical readback). Legacy actions keep the historical
// raw-value pipeline byte-for-byte.
type StaticEQVSPPort struct {
	Client VSPClient
	// CommandName gates which action command this port executes. Empty keeps
	// the D2-1 static_eq_band_adjust default.
	CommandName string

	mu            sync.Mutex
	baseRevision  int64
	projectEpoch  string
	beforeValues  map[string]float64 // "plugin:param" -> value captured at preflight
	pluginIDByRef map[string]string  // action ID -> resolved plugin instance ID
}

func (p *StaticEQVSPPort) commandName() string {
	if strings.TrimSpace(p.CommandName) != "" {
		return strings.TrimSpace(p.CommandName)
	}
	return staticEQActionCommand
}

// instantiatePayload builds the instantiate_plugin command. The identifier is
// only attached when the action pins one; the real-plugin path carries
// plugin_path and leaves the identifier empty, because the kernel resolves a
// non-empty identifier exclusively against its known-plugin list (which an
// uns-scanned host does not populate) and never falls back to the path.
func (p *StaticEQVSPPort) instantiatePayload(action orchestration.Action) map[string]any {
	payload := map[string]any{
		"cmd":           "instantiate_plugin",
		"track_id":      action.TargetRef,
		"base_revision": p.baseRevision,
	}
	if identifier := actionArgText(action, "plugin_identifier"); identifier != "" {
		payload["plugin_identifier"] = identifier
	}
	if pluginPath := actionArgText(action, "plugin_path"); pluginPath != "" {
		payload["plugin_path"] = pluginPath
	}
	return payload
}

func (p *StaticEQVSPPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
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
		if action.Command != p.commandName() || strings.TrimSpace(action.TargetRef) == "" || strings.TrimSpace(action.BeforeFingerprint) == "" {
			return fmt.Errorf("unsupported or unguarded B2 action %s", action.ID)
		}
		if _, ok := numeric(action.Args["target_value"]); !ok {
			return fmt.Errorf("action %s requires numeric target_value", action.ID)
		}
		if strings.TrimSpace(fmt.Sprint(action.Args["param_id"])) == "" {
			return fmt.Errorf("action %s requires param_id", action.ID)
		}
		if actionArgText(action, "write_mode") == WriteModeNormalizedBatchV1 {
			if actionArgText(action, "plugin_path") == "" || actionArgText(action, "param_id_ch2") == "" {
				return fmt.Errorf("normalized batch action %s requires plugin_path and param_id_ch2", action.ID)
			}
		} else if actionArgText(action, "plugin_id") == "" && actionArgText(action, "plugin_identifier") == "" {
			return fmt.Errorf("action %s requires plugin_id or plugin_identifier", action.ID)
		}
	}
	current, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || current == nil || !current.OK() {
		return fmt.Errorf("read current VSP snapshot: %w", err)
	}
	if current.ProjectEpoch != cut.ProjectEpoch || current.Revision != baseRevision {
		return fmt.Errorf("stale_project_cut: current epoch/revision does not match proposal")
	}
	before := map[string]float64{}
	pluginIDs := map[string]string{}
	for _, action := range actionSet.Actions {
		pluginID := actionArgText(action, "plugin_id")
		if pluginID == "" {
			// The EQ instance will be created during Apply; its before value is
			// the plugin's initialized default, recorded as absent here.
			continue
		}
		value, err := p.readPluginParam(ctx, action.TargetRef, pluginID, actionArgText(action, "param_id"))
		if err != nil {
			return err
		}
		before[eqBeforeKey(pluginID, fmt.Sprint(action.Args["param_id"]))] = value
		pluginIDs[action.ID] = pluginID
	}
	p.mu.Lock()
	p.baseRevision = baseRevision
	p.projectEpoch = cut.ProjectEpoch
	p.beforeValues = before
	p.pluginIDByRef = pluginIDs
	p.mu.Unlock()
	return nil
}

func (p *StaticEQVSPPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.baseRevision <= 0 || p.projectEpoch == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("VSP preflight has not completed")
	}
	target, ok := numeric(action.Args["target_value"])
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("numeric target_value is required")
	}
	paramID := actionArgText(action, "param_id")
	if paramID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("param_id is required")
	}
	requestID := strings.TrimSpace(idempotencyKey)
	if requestID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("stable idempotency key is required")
	}
	txID := "tx_" + requestID
	pluginID := actionArgText(action, "plugin_id")
	instantiated := false
	var err error
	if pluginID == "" {
		result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, p.instantiatePayload(action), requestID+":instantiate", txID+":instantiate")
		if err != nil {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
		}
		reply := result.LegacyLikeReply()
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
			message := strings.TrimSpace(fmt.Sprint(reply["message"]))
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: message}, fmt.Errorf("VSP plugin instantiation failed: %s", message)
		}
		pluginID = strings.TrimSpace(firstReplyText(reply, "plugin_id", "pluginId", "instance_id"))
		if pluginID == "" {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("plugin instantiation returned no plugin_id")
		}
		// The instantiation is itself a project mutation: the kernel's
		// write-like base_revision CAS expects the post-instantiate revision
		// afterwards, so the parameter batch must rebase onto a fresh
		// snapshot instead of reusing the preflight base (stale_project_cut).
		rebased, rebaseErr := p.Client.VSPStateSnapshot(ctx, "project.timeline")
		if rebaseErr != nil || rebased == nil || !rebased.OK() {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("plugin instantiated but the rebasing snapshot failed: %v", rebaseErr)
		}
		if rebased.ProjectEpoch != p.projectEpoch {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("project epoch changed during plugin instantiation")
		}
		if rebased.Revision <= p.baseRevision {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("plugin instantiation did not advance the project revision")
		}
		p.baseRevision = rebased.Revision
		instantiated = true
	}
	writeMode := actionArgText(action, "write_mode")
	paramIDCh2 := ""
	var requestedChannels []eqGainChannel
	var deltaPlan *deltaChannelPlan
	if writeMode == WriteModeNormalizedBatchV1 {
		paramIDCh2 = actionArgText(action, "param_id_ch2")
		surface, surfaceErr := p.eqParameterSurface(ctx, action.TargetRef, pluginID)
		if surfaceErr != nil {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: surfaceErr.Error()}, fmt.Errorf("VSP plugin parameter mutation failed: %s", surfaceErr)
		}
		if strings.EqualFold(actionArgText(action, "target_semantics"), deltaSemantics) {
			channels, plan, deltaErr := p.planDeltaChannels(ctx, action.TargetRef, pluginID, requestID, txID, surface, target, paramID, paramIDCh2)
			if deltaErr != nil {
				return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: deltaErr.Error()}, fmt.Errorf("VSP plugin parameter mutation failed: %s", deltaErr)
			}
			requestedChannels = channels
			deltaPlan = plan
		} else {
			channels, _, planErr := eqPlanGainChannels(surface, target, paramID, paramIDCh2)
			if planErr != nil {
				return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: planErr.Error()}, fmt.Errorf("VSP plugin parameter mutation failed: %s", planErr)
			}
			requestedChannels = channels
		}
	}
	var result *kernel.VSPCommandResult
	if writeMode == WriteModeNormalizedBatchV1 {
		parameters := make([]map[string]any, 0, len(requestedChannels))
		for _, channel := range requestedChannels {
			parameters = append(parameters, map[string]any{"parameter_id": channel.ParamID, "normalized_value": channel.RequestedNormalized})
		}
		result, err = p.Client.SendVSPCommandWithIDs(ctx, "plugin.set_params_batch", map[string]any{
			"track_id":      action.TargetRef,
			"plugin_id":     pluginID,
			"parameters":    parameters,
			"readback":      true,
			"base_revision": p.baseRevision,
		}, requestID, txID)
	} else {
		result, err = p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
			"cmd":           "set_plugin_param",
			"track_id":      action.TargetRef,
			"plugin_id":     pluginID,
			"param_id":      paramID,
			"value":         target,
			"base_revision": p.baseRevision,
		}, requestID, txID)
	}
	if err != nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error()}, err
	}
	if writeMode == WriteModeNormalizedBatchV1 {
		if failure := eqTypedCommandFailure(result); failure != "" {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: failure}, fmt.Errorf("VSP plugin parameter mutation failed: %s", failure)
		}
	} else {
		reply := result.LegacyLikeReply()
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
			message := strings.TrimSpace(fmt.Sprint(reply["message"]))
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: message}, fmt.Errorf("VSP plugin parameter mutation failed: %s", message)
		}
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
	actual := 0.0
	readbackVerified := false
	var actualPhysical any
	normalizedRecords := make([]map[string]any, 0, len(requestedChannels))
	if writeMode == WriteModeNormalizedBatchV1 {
		freshSurface, surfaceErr := p.eqParameterSurface(ctx, action.TargetRef, pluginID)
		if surfaceErr != nil {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, fmt.Errorf("plugin parameter readback did not match target")
		}
		readbackVerified = true
		for index, channel := range requestedChannels {
			info, ok := freshSurface[channel.ParamID]
			var actualValue float64
			numericOK := false
			if ok {
				actualValue, numericOK = numeric(info.NormalizedValue)
			}
			channel.ActualNormalized = actualValue
			requestedChannels[index] = channel
			normalizedRecords = append(normalizedRecords, map[string]any{
				"parameter_id": channel.ParamID, "requested_normalized": channel.RequestedNormalized, "actual_normalized": actualValue,
			})
			if !numericOK || math.Abs(actualValue-channel.RequestedNormalized) > EqNormalizedTolerance {
				readbackVerified = false
			}
			if index == 0 {
				if physical, parsed := plugingrabber.ParseEQLocalizedNumber(info.ValueText); parsed {
					actualPhysical = physical
				}
			}
		}
		if !readbackVerified {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, fmt.Errorf("plugin parameter readback did not match target")
		}
		if deltaPlan != nil {
			achieved, parsed := actualPhysical.(float64)
			if !parsed || math.Abs(achieved-deltaPlan.TargetPhysical) > ThresholdDeltaToleranceDB {
				return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true},
					fmt.Errorf("delta physical target %.4g dB not achieved (readback %v)", deltaPlan.TargetPhysical, actualPhysical)
			}
		}
	} else {
		var legacyErr error
		actual, legacyErr = p.readPluginParamUnlocked(ctx, action.TargetRef, pluginID, paramID)
		if legacyErr != nil || math.Abs(actual-target) > 0.001 {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied_unreconciled", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true}, fmt.Errorf("plugin parameter readback did not match target")
		}
		readbackVerified = true
		actualPhysical = actual
	}
	beforeRevision := p.baseRevision
	beforeKey := eqBeforeKey(pluginID, paramID)
	beforeValue, beforeAvailable := p.beforeValues[beforeKey]
	transactionID := strings.TrimSpace(result.TransactionID)
	if transactionID == "" {
		transactionID = txID
	}
	p.baseRevision = current.Revision
	p.pluginIDByRef[action.ID] = pluginID
	details := map[string]any{
		"before_revision": strconv.FormatInt(beforeRevision, 10), "after_revision": strconv.FormatInt(current.Revision, 10),
		"transaction_id": transactionID, "idempotency_key": requestID,
		"plugin_id": pluginID, "param_id": paramID,
		"requested_target_value": target, "actual_readback_value": actualPhysical, "readback_verified": readbackVerified,
		"plugin_instantiated_by_action": instantiated,
	}
	if writeMode == WriteModeNormalizedBatchV1 {
		details["write_mode"] = WriteModeNormalizedBatchV1
		details["plugin_path"] = actionArgText(action, "plugin_path")
		details["param_id_ch2"] = paramIDCh2
		details["normalized_channels"] = normalizedRecords
	}
	if deltaPlan != nil {
		details["target_semantics"] = deltaSemantics
		details["delta_calibration"] = deltaPlan.audit()
	}
	if beforeAvailable {
		details["before_readback_value"] = beforeValue
	}
	return orchestration.ActionReceipt{
		ActionID:        action.ID,
		Status:          "applied",
		AppliedRevision: strconv.FormatInt(current.Revision, 10),
		EffectivelyOnce: true,
		EvidenceRefs:    []string{"vsp.command:" + requestID, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details:         details,
	}, nil
}

func (p *StaticEQVSPPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
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
	baseRevision, parseErr := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if parseErr != nil || baseRevision <= 0 {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("invalid reconcile base revision")
	}
	target, ok := numeric(action.Args["target_value"])
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("numeric target_value is required")
	}
	pluginID := actionArgText(action, "plugin_id")
	if pluginID == "" {
		// Without a durable plugin_id an applied state cannot be verified from
		// the persisted action alone; treat it as not reconcilable.
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	paramID := actionArgText(action, "param_id")
	actual, readErr := p.readPluginParam(ctx, action.TargetRef, pluginID, paramID)
	if readErr != nil || math.Abs(actual-target) > 0.001 || current.Revision <= baseRevision {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "not_applied", AppliedRevision: strconv.FormatInt(current.Revision, 10)}, nil
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: strconv.FormatInt(current.Revision, 10), EffectivelyOnce: true,
		EvidenceRefs: []string{"vsp.reconcile:" + idempotencyKey, "vsp.state.revision:" + strconv.FormatInt(current.Revision, 10)},
		Details: map[string]any{
			"before_revision": strconv.FormatInt(baseRevision, 10), "after_revision": strconv.FormatInt(current.Revision, 10),
			"transaction_id": "tx_" + idempotencyKey, "idempotency_key": idempotencyKey,
			"plugin_id": pluginID, "param_id": paramID,
			"requested_target_value": target, "actual_readback_value": actual, "readback_verified": true, "reconciled": true,
		},
	}, nil
}

// readPluginParam reads one plugin parameter value through the governed
// get_plugin_parameters command.
func (p *StaticEQVSPPort) readPluginParam(ctx context.Context, trackID, pluginID, paramID string) (float64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readPluginParamUnlocked(ctx, trackID, pluginID, paramID)
}

func (p *StaticEQVSPPort) readPluginParamUnlocked(ctx context.Context, trackID, pluginID, paramID string) (float64, error) {
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  trackID,
		"plugin_id": pluginID,
		"param_id":  paramID,
	}, "read:"+trackID+":"+pluginID+":"+paramID, "tx_read:"+pluginID)
	if err != nil {
		return 0, fmt.Errorf("plugin parameter readback failed: %w", err)
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		return 0, fmt.Errorf("plugin parameter readback failed: %s", strings.TrimSpace(fmt.Sprint(reply["message"])))
	}
	parameters, _ := reply["parameters"].([]any)
	for _, row := range parameters {
		entry, _ := row.(map[string]any)
		if entry == nil || !strings.EqualFold(strings.TrimSpace(fmt.Sprint(entry["param_id"])), strings.TrimSpace(paramID)) {
			continue
		}
		for _, key := range []string{"value", "current_value", "normalized_value"} {
			if value, ok := numeric(entry[key]); ok {
				return value, nil
			}
		}
		if text := strings.TrimSpace(fmt.Sprint(entry["value_text"])); text != "" {
			if parsed, parseErr := strconv.ParseFloat(text, 64); parseErr == nil {
				return parsed, nil
			}
		}
	}
	return 0, fmt.Errorf("plugin parameter %q was not present in the readback", paramID)
}

// actionArgText reads one action arg, treating an absent value ("<nil>" from
// fmt.Sprint(nil)) as missing so a deleted key cannot masquerade as an
// identity string.
func actionArgText(action orchestration.Action, key string) string {
	text := strings.TrimSpace(fmt.Sprint(action.Args[key]))
	if text == "<nil>" {
		return ""
	}
	return text
}

func eqBeforeKey(pluginID, paramID string) string {
	return pluginID + "\x1f" + paramID
}

func firstReplyText(reply map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := strings.TrimSpace(fmt.Sprint(reply[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}
