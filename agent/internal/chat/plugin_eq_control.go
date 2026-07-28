package chat

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/tools"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberSetEQPointCommand = "plugin_grabber_set_eq_point"
	pluginGrabberSetEQPointTool    = "plugin_grabber.set_eq_point"
)

type eqKernelTransport interface {
	SendCommand(context.Context, map[string]any) (map[string]any, string, error)
	SendVSPCommand(context.Context, string, map[string]any) (*kernel.VSPCommandResult, error)
}

func (s *Server) eqKernelClient() eqKernelTransport {
	if s.eqKernelOverride != nil {
		return s.eqKernelOverride
	}
	if s.kernel == nil {
		return nil
	}
	return s.kernel
}

func pluginGrabberSetEQPointInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName == pluginGrabberSetEQPointTool || toolName == pluginGrabberSetEQPointCommand {
		cmd := map[string]any{"cmd": pluginGrabberSetEQPointCommand}
		for k, v := range req.Args {
			cmd[k] = v
		}
		return cmd, true
	}
	args := workflowCommandArgs(req.Command)
	name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(args["command"]))
	}
	if name == pluginGrabberSetEQPointCommand {
		return args, true
	}
	return nil, false
}

func (s *Server) invokePluginGrabberSetEQPointWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	requestContext := req.Context
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	resp := s.runPluginGrabberSetEQPointWorkflow(ctx, "invoke_"+randomID(), workflowCmd, requestContext)
	out := harness.InvokeResponse{
		Status:               "ok",
		Tool:                 pluginGrabberSetEQPointTool,
		CommandName:          pluginGrabberSetEQPointCommand,
		RiskLevel:            tools.RiskUndoable,
		RequiresConfirmation: false,
		UndoLabel:            "Set EQ point",
	}
	if len(resp.ExecutedKernelReply) > 0 {
		if result, ok := resp.ExecutedKernelReply[0]["result"].(map[string]any); ok {
			out.Result = result
		}
	}
	if resp.Error != "" {
		out.Status = "error"
		out.Error = resp.Error
		return out, fmt.Errorf("%s", resp.Error)
	}
	return out, nil
}

func (s *Server) runPluginGrabberSetEQPointWorkflow(ctx context.Context,
	conversationID string,
	workflowCmd map[string]any,
	requestContext map[string]any,
) ChatResponse {
	args := workflowCommandArgs(workflowCmd)
	edit := map[string]any{
		"action": "upsert",
		"shape":  firstNonEmpty(firstNonEmptyText(args, "shape", "filter_type"), "bell"),
	}
	if value, ok := args["freq_hz"]; ok {
		edit["frequency_hz"] = value
	}
	for _, key := range []string{"gain_db", "q", "slope_db_per_oct"} {
		if value, ok := args[key]; ok {
			edit[key] = value
		}
	}
	compat := make(map[string]any, len(args)+2)
	for key, value := range args {
		compat[key] = value
	}
	compat["cmd"] = pluginGrabberApplyEQEditsCommand
	compat["atomic"] = true
	compat["edits"] = []map[string]any{edit}
	result, err := s.applyPluginGrabberEQEdits(ctx, compat, requestContext)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error(),
			ExecutedKernelReply: []map[string]any{{"status": "rejected", "command_name": pluginGrabberSetEQPointCommand,
				"result": map[string]any{"status": "rejected", "rejection_code": eqControlFailureCode(err), "message": err.Error()}}}}
	}
	result["compatibility_adapter"] = pluginGrabberSetEQPointTool
	return ChatResponse{ConversationID: conversationID, ExecutedKernelReply: []map[string]any{
		{"status": "ok", "command_name": pluginGrabberSetEQPointCommand, "result": result},
	}}
}

// eqWriteStep is one normalised parameter write inside a set_eq_point operation.
type eqWriteStep struct {
	ParamID            string
	NormalizedValue    float64
	Role               string // "freq" | "gain" | "q" | "used" | "shape"
	Channel            string
	RequestedPhysical  *float64
	Quantized          bool
	CorrectionLow      float64
	CorrectionHigh     float64
	PhysicalDecreasing bool
	RequestedLabel     string
	ExpectedLabel      string
	ActivationLast     bool
	PhysicalMultiplier float64
	DiscretePhysical   bool
}

type eqTypedPointRequest struct {
	FreqHz        float64
	GainDB        *float64
	Q             *float64
	Shape         string
	SlopeDBPerOct *float64
}

type eqPreimageValue struct {
	ParamID    string
	Normalized float64
	Role       string
}

func eqWritePreimage(digest plugingrabber.ParameterDigest, writes []eqWriteStep) ([]eqPreimageValue, error) {
	byID := map[string]plugingrabber.ParameterInfo{}
	for _, param := range digest.Parameters {
		byID[strings.TrimSpace(param.ID)] = param
	}
	seen := map[string]bool{}
	out := make([]eqPreimageValue, 0, len(writes))
	for _, write := range writes {
		if seen[write.ParamID] {
			continue
		}
		param, ok := byID[write.ParamID]
		if !ok {
			return nil, fmt.Errorf("transaction preimage omitted parameter %s", write.ParamID)
		}
		value, ok := numericAny(param.NormalizedValue)
		if !ok {
			return nil, fmt.Errorf("parameter %s has no numeric normalized preimage", write.ParamID)
		}
		seen[write.ParamID] = true
		out = append(out, eqPreimageValue{ParamID: write.ParamID, Normalized: value, Role: write.Role})
	}
	return out, nil
}

func eqParameterSnapshot(digest plugingrabber.ParameterDigest) []eqPreimageValue {
	out := make([]eqPreimageValue, 0, len(digest.Parameters))
	seen := map[string]bool{}
	for _, param := range digest.Parameters {
		paramID := strings.TrimSpace(param.ID)
		value, ok := numericAny(param.NormalizedValue)
		if paramID == "" || !ok || seen[paramID] {
			continue
		}
		seen[paramID] = true
		out = append(out, eqPreimageValue{ParamID: paramID, Normalized: value, Role: "snapshot"})
	}
	return out
}

func numericAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	}
	return 0, false
}

func (s *Server) executeEQTransaction(ctx context.Context, trackID, pluginID string,
	writes []eqWriteStep, preimage, snapshot []eqPreimageValue) ([]map[string]any, []map[string]any, error) {
	client := s.eqKernelClient()
	if client == nil {
		return nil, nil, fmt.Errorf("kernel client is nil")
	}
	parameters := make([]map[string]any, 0, len(writes))
	for _, write := range writes {
		parameters = append(parameters, map[string]any{"parameter_id": write.ParamID, "normalized_value": write.NormalizedValue})
	}
	result, err := client.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "parameters": parameters, "readback": true,
	})
	if err != nil || eqVSPFailure(result) != "" {
		failure := firstNonEmpty(errorText(err), eqVSPFailure(result), "EQ parameter batch failed")
		return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage, failure)
	}

	actualRows, err := s.readEQParameters(ctx, trackID, pluginID)
	if err != nil {
		return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
			fmt.Sprintf("fresh readback failed: %v", err))
	}
	index := eqParameterRowIndex(actualRows)
	if unexpected := unexpectedEQParameterChanges(snapshot, writes, index); len(unexpected) > 0 {
		failure := s.eqTransactionFailure(ctx, trackID, pluginID, append(preimage, unexpected...),
			fmt.Sprintf("unplanned parameter changes: %s", eqPreimageParamIDs(unexpected)))
		return nil, nil, rejectEQControl("unplanned_parameter_change", "%v", failure)
	}
	if corrected, correctionErr := s.correctEQPhysicalReadback(ctx, trackID, pluginID, writes, index); correctionErr != nil {
		return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage, correctionErr.Error())
	} else if corrected {
		actualRows, err = s.readEQParameters(ctx, trackID, pluginID)
		if err != nil {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("post-correction readback failed: %v", err))
		}
		index = eqParameterRowIndex(actualRows)
		if unexpected := unexpectedEQParameterChanges(snapshot, writes, index); len(unexpected) > 0 {
			failure := s.eqTransactionFailure(ctx, trackID, pluginID, append(preimage, unexpected...),
				fmt.Sprintf("unplanned parameter changes: %s", eqPreimageParamIDs(unexpected)))
			return nil, nil, rejectEQControl("unplanned_parameter_change", "%v", failure)
		}
	}
	executed := make([]map[string]any, 0, len(writes))
	actual := make([]map[string]any, 0, len(writes))
	for _, write := range writes {
		row, ok := index[write.ParamID]
		if !ok {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("fresh readback omitted touched parameter %s", write.ParamID))
		}
		actualNormalized, ok := firstNumericAny(row, "normalized_value", "normalised_value", "current_normalized_value")
		if !ok {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("fresh readback has no normalized value for %s", write.ParamID))
		}
		if math.Abs(actualNormalized-write.NormalizedValue) > 1e-4 {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("parameter %s normalized readback mismatch requested %.9f actual %.9f",
					write.ParamID, write.NormalizedValue, actualNormalized))
		}
		if write.ExpectedLabel != "" && !strings.EqualFold(strings.TrimSpace(firstNonEmptyText(row, "value_text")), strings.TrimSpace(write.ExpectedLabel)) {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("parameter %s enum readback mismatch requested %s actual %s",
					write.ParamID, write.ExpectedLabel, firstNonEmptyText(row, "value_text")))
		}
		if write.Role == "used" && eqActivationReadbackInactive(firstNonEmptyText(row, "value_text")) {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("parameter %s remained inactive after activation (%s)",
					write.ParamID, firstNonEmptyText(row, "value_text")))
		}
		if (write.Role == "disabled" || write.Role == "removed") && !eqActivationReadbackInactive(firstNonEmptyText(row, "value_text")) {
			return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
				fmt.Sprintf("parameter %s remained active after %s (%s)",
					write.ParamID, write.Role, firstNonEmptyText(row, "value_text")))
		}
		entry := map[string]any{"param_id": write.ParamID, "role": write.Role, "channel": write.Channel,
			"requested_normalized": write.NormalizedValue, "actual_normalized": actualNormalized, "quantized": write.Quantized}
		if write.RequestedPhysical != nil {
			entry["requested_physical"] = *write.RequestedPhysical
		}
		if write.RequestedLabel != "" {
			entry["requested_label"] = write.RequestedLabel
			entry["actual_label"] = firstNonEmptyText(row, "value_text")
		}
		displayPhysical, physicalOK := physicalReadbackForRole(write.Role, row)
		physical := eqEffectivePhysicalReadback(write, displayPhysical)
		if write.RequestedPhysical != nil && isEQPhysicalRole(write.Role) && !write.Quantized {
			if !physicalOK || math.Abs(physical-*write.RequestedPhysical) > eqPhysicalTolerance(write.Role, *write.RequestedPhysical) {
				return nil, nil, s.eqTransactionFailure(ctx, trackID, pluginID, preimage,
					fmt.Sprintf("parameter %s physical readback mismatch requested %g actual %s",
						write.ParamID, *write.RequestedPhysical, firstNonEmptyText(row, "value_text")))
			}
		}
		if physicalOK {
			entry["actual_physical"] = physical
			if eqWritePhysicalMultiplier(write) != 1 {
				entry["display_physical"] = displayPhysical
				entry["physical_multiplier"] = eqWritePhysicalMultiplier(write)
			}
		}
		executed = append(executed, entry)
		actual = append(actual, map[string]any{"param_id": write.ParamID, "role": write.Role,
			"channel": write.Channel, "normalized": actualNormalized, "value_text": firstNonEmptyText(row, "value_text"),
			"physical": func() any {
				if physicalOK {
					return physical
				}
				return nil
			}()})
	}
	return executed, actual, nil
}

func unexpectedEQParameterChanges(snapshot []eqPreimageValue, writes []eqWriteStep,
	index map[string]map[string]any) []eqPreimageValue {
	planned := map[string]bool{}
	for _, write := range writes {
		planned[write.ParamID] = true
	}
	out := []eqPreimageValue{}
	for _, before := range snapshot {
		if planned[before.ParamID] {
			continue
		}
		row, ok := index[before.ParamID]
		if !ok {
			continue
		}
		after, ok := firstNumericAny(row, "normalized_value", "normalised_value", "current_normalized_value")
		if ok && math.Abs(after-before.Normalized) > 1e-4 {
			before.Role = "side_effect"
			out = append(out, before)
		}
	}
	return out
}

func eqPreimageParamIDs(values []eqPreimageValue) string {
	ids := make([]string, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ParamID)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func (s *Server) eqTransactionFailure(ctx context.Context, trackID, pluginID string,
	preimage []eqPreimageValue, failure string) error {
	if rollbackErr := s.rollbackEQTransaction(ctx, trackID, pluginID, preimage); rollbackErr != nil {
		return fmt.Errorf("%s; rollback failed: %w", failure, rollbackErr)
	}
	return fmt.Errorf("%s; full preimage restored", failure)
}

func eqActivationReadbackInactive(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, token := range []string{"unused", "disabled", "bypass", "bypassed", "off", "out", "false"} {
		if lower == token || strings.HasPrefix(lower, token+" ") {
			return true
		}
	}
	return false
}

func (s *Server) correctEQPhysicalReadback(ctx context.Context, trackID, pluginID string,
	writes []eqWriteStep, index map[string]map[string]any) (bool, error) {
	client := s.eqKernelClient()
	if client == nil {
		return false, fmt.Errorf("kernel client is nil")
	}
	corrected := false
	// Five-point curves provide the initial estimate.  Twelve bisection steps
	// then give enough resolution for coarse/nonlinear host mappings (notably
	// frequency controls) while remaining strictly bounded and transactional.
	const maxPhysicalCorrectionIterations = 12
	for iteration := 0; iteration < maxPhysicalCorrectionIterations; iteration++ {
		corrections := []map[string]any{}
		anyMismatch := false
		for i := range writes {
			write := &writes[i]
			if write.Quantized || write.DiscretePhysical || write.RequestedPhysical == nil || !isEQPhysicalRole(write.Role) {
				continue
			}
			row, ok := index[write.ParamID]
			if !ok {
				return corrected, fmt.Errorf("physical correction readback omitted %s", write.ParamID)
			}
			displayPhysical, ok := physicalReadbackForRole(write.Role, row)
			if !ok {
				return corrected, fmt.Errorf("cannot parse %s physical readback %q", write.Role, firstNonEmptyText(row, "value_text"))
			}
			actual := eqEffectivePhysicalReadback(*write, displayPhysical)
			target := *write.RequestedPhysical
			if math.Abs(actual-target) <= eqPhysicalTolerance(write.Role, target) {
				continue
			}
			anyMismatch = true
			if actual < target && !write.PhysicalDecreasing {
				write.CorrectionLow = math.Max(write.CorrectionLow, write.NormalizedValue)
			} else if !write.PhysicalDecreasing {
				write.CorrectionHigh = math.Min(write.CorrectionHigh, write.NormalizedValue)
			} else if actual < target {
				write.CorrectionHigh = math.Min(write.CorrectionHigh, write.NormalizedValue)
			} else {
				write.CorrectionLow = math.Max(write.CorrectionLow, write.NormalizedValue)
			}
			next := (write.CorrectionLow + write.CorrectionHigh) / 2
			if math.Abs(next-write.NormalizedValue) < 1e-7 {
				return corrected, fmt.Errorf("%s correction stalled at %g (target %g actual %g)", write.ParamID, next, target, actual)
			}
			write.NormalizedValue = next
			corrections = append(corrections, map[string]any{"parameter_id": write.ParamID, "normalized_value": next})
		}
		if !anyMismatch {
			return corrected, nil
		}
		result, err := client.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
			"track_id": trackID, "plugin_id": pluginID, "parameters": corrections, "readback": true})
		if err != nil || eqVSPFailure(result) != "" {
			return corrected, fmt.Errorf("physical correction failed: %s", firstNonEmpty(errorText(err), eqVSPFailure(result)))
		}
		corrected = true
		rows, err := s.readEQParameters(ctx, trackID, pluginID)
		if err != nil {
			return corrected, err
		}
		index = eqParameterRowIndex(rows)
	}
	for _, write := range writes {
		if write.Quantized || write.DiscretePhysical || write.RequestedPhysical == nil || !isEQPhysicalRole(write.Role) {
			continue
		}
		displayPhysical, ok := physicalReadbackForRole(write.Role, index[write.ParamID])
		actual := eqEffectivePhysicalReadback(write, displayPhysical)
		if !ok || math.Abs(actual-*write.RequestedPhysical) > eqPhysicalTolerance(write.Role, *write.RequestedPhysical) {
			return corrected, fmt.Errorf("parameter %s did not reach requested %s after %d bounded corrections: requested=%g actual=%g tolerance=%g",
				write.ParamID, write.Role, maxPhysicalCorrectionIterations, *write.RequestedPhysical, actual,
				eqPhysicalTolerance(write.Role, *write.RequestedPhysical))
		}
	}
	return corrected, nil
}

func eqWritePhysicalMultiplier(write eqWriteStep) float64 {
	if write.Role == "freq" && write.PhysicalMultiplier > 0 {
		return write.PhysicalMultiplier
	}
	return 1
}

func eqEffectivePhysicalReadback(write eqWriteStep, displayPhysical float64) float64 {
	return displayPhysical * eqWritePhysicalMultiplier(write)
}

func isEQPhysicalRole(role string) bool {
	return role == "freq" || role == "gain" || role == "q" || role == "slope"
}

func eqPhysicalTolerance(role string, target float64) float64 {
	switch role {
	case "freq":
		return math.Max(1, math.Abs(target)*0.0025)
	case "gain":
		return 0.15
	case "q":
		return math.Max(0.05, math.Abs(target)*0.02)
	case "slope":
		return 0.25
	}
	return 0
}

func (s *Server) readEQParameters(ctx context.Context, trackID, pluginID string) ([]map[string]any, error) {
	client := s.eqKernelClient()
	if client == nil {
		return nil, fmt.Errorf("kernel client is nil")
	}
	reply, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters",
		"track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return nil, err
	}
	if !kernelReplyOK(reply) {
		return nil, fmt.Errorf("%s", firstNonEmptyText(reply, "message", "error"))
	}
	digest := buildPluginParameterDigest(reply)
	rows := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		rows = append(rows, map[string]any{"param_id": param.ID, "normalized_value": param.NormalizedValue,
			"value_text": param.ValueText})
	}
	return rows, nil
}

func (s *Server) rollbackEQTransaction(ctx context.Context, trackID, pluginID string, preimage []eqPreimageValue) error {
	client := s.eqKernelClient()
	if client == nil {
		return fmt.Errorf("kernel client is nil")
	}
	ordered := append([]eqPreimageValue(nil), preimage...)
	sort.SliceStable(ordered, func(i, j int) bool {
		rank := func(role string) int {
			if role == "side_effect" {
				return 2
			}
			if role == "used" || role == "disabled" || role == "removed" {
				return 1
			}
			return 0
		}
		return rank(ordered[i].Role) < rank(ordered[j].Role)
	})
	parameters := make([]map[string]any, 0, len(ordered))
	for _, value := range ordered {
		parameters = append(parameters, map[string]any{"parameter_id": value.ParamID, "normalized_value": value.Normalized})
	}
	result, err := client.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "parameters": parameters, "readback": true,
	})
	if err != nil {
		return err
	}
	if failure := eqVSPFailure(result); failure != "" {
		return fmt.Errorf("%s", failure)
	}
	rows, err := s.readEQParameters(ctx, trackID, pluginID)
	if err != nil {
		return err
	}
	index := eqParameterRowIndex(rows)
	for _, expected := range preimage {
		row, ok := index[expected.ParamID]
		if !ok {
			return fmt.Errorf("rollback readback omitted %s", expected.ParamID)
		}
		actual, ok := firstNumericAny(row, "normalized_value")
		if !ok || math.Abs(actual-expected.Normalized) > 1e-4 {
			return fmt.Errorf("rollback mismatch for %s", expected.ParamID)
		}
	}
	return nil
}

func eqVSPFailure(result any) string {
	if result == nil {
		return "empty VSP response"
	}
	// VSPCommandResult is intentionally inspected through its public maps so
	// this transaction remains local to the direct Plugin Grabber path.
	typed, ok := result.(*kernel.VSPCommandResult)
	if !ok {
		return "invalid VSP response"
	}
	for _, candidate := range []map[string]any{typed.Payload, typed.Response, typed.LegacyReply} {
		status := strings.ToLower(firstNonEmptyText(candidate, "status", "stage"))
		if status == "error" || status == "failed" || status == "partial_failure" || status == "rejected" {
			return firstNonEmptyText(candidate, "message", "error", "status")
		}
		if payload := mapValue(candidate["payload"]); len(payload) > 0 {
			status = strings.ToLower(firstNonEmptyText(payload, "status"))
			if status == "error" || status == "failed" || status == "partial_failure" {
				return firstNonEmptyText(payload, "message", "error", "status")
			}
		}
	}
	return ""
}

func eqParameterRowIndex(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		if id := firstNonEmptyText(row, "param_id", "parameter_id", "id"); id != "" {
			out[id] = row
		}
	}
	return out
}

func firstNumericAny(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := numericAny(row[key]); ok {
			return value, true
		}
	}
	return 0, false
}

func physicalReadbackForRole(role string, row map[string]any) (float64, bool) {
	text := firstNonEmptyText(row, "value_text", "current_value_text")
	switch role {
	case "freq":
		return parseEQFrequencyReadback(text)
	case "gain", "q", "slope":
		return plugingrabber.ParseEQLocalizedNumber(text)
	}
	return 0, false
}

func parseEQFrequencyReadback(text string) (float64, bool) {
	return plugingrabber.ParseEQFrequencyText(text)
}

func eqTypedSectionWritePlan(summary map[string]any, request eqTypedPointRequest) ([]eqWriteStep, map[string]any, error) {
	if strings.EqualFold(firstNonEmptyText(summary, "mapping_source"), "vps_control_graph") {
		return eqControlGraphUpsertWritePlan(summary, request)
	}
	sections := mapRowsValue(summary["sections"])
	if len(sections) == 0 {
		return nil, nil, fmt.Errorf("EQ summary has no typed sections for requested shape %s", request.Shape)
	}
	if (request.Shape == "low_cut" || request.Shape == "high_cut") && request.GainDB != nil {
		return nil, nil, fmt.Errorf("invalid_field_for_shape: gain_db is not applicable to %s", request.Shape)
	}
	structuralCandidates := []map[string]any{}
	for _, section := range sections {
		complete, _ := section["complete"].(bool)
		if !complete || !eqSectionActionSupported(section, request.Shape, "upsert") {
			continue
		}
		if !eqSectionFrequencyTargetInRange(section, request.FreqHz) {
			continue
		}
		structuralCandidates = append(structuralCandidates, section)
	}
	if len(structuralCandidates) == 0 {
		return nil, nil, fmt.Errorf("requested shape %s at %g Hz is not provably reachable in any section; available shapes: %v",
			request.Shape, request.FreqHz, summary["supported_filter_kinds"])
	}
	candidates := make([]map[string]any, 0, len(structuralCandidates))
	candidateFailures := make([]string, 0, len(structuralCandidates))
	for _, section := range structuralCandidates {
		if err := eqTypedSectionExplicitFieldsAvailable(section, request); err != nil {
			sectionName := firstNonEmptyText(section, "section", "band")
			if sectionName == "" {
				sectionName = "<unnamed>"
			}
			candidateFailures = append(candidateFailures, fmt.Sprintf("%s: %v", sectionName, err))
			continue
		}
		candidates = append(candidates, section)
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("explicit_field_unavailable: no %s section at %g Hz can satisfy all explicitly requested fields (%s)",
			request.Shape, request.FreqHz, strings.Join(candidateFailures, "; "))
	}
	selected := selectEQSectionByFrequency(candidates, request.FreqHz, firstNonEmptyText(summary, "eq_model"))
	if selected == nil {
		return nil, nil, fmt.Errorf("no complete %s section has a recoverable frequency binding", request.Shape)
	}

	writes := []eqWriteStep{}
	applied := []string{"shape"}
	shapeWrites, err := eqFilterKindWrites(selected, request.Shape)
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, shapeWrites...)

	frequencyQuantized := false
	if len(mapRowsValue(selected["frequency_bindings"])) > 0 {
		freqWrites, quantized, freqErr := eqFrequencyWritesForSection(selected, request.FreqHz)
		if freqErr != nil {
			return nil, nil, freqErr
		}
		frequencyQuantized = quantized
		writes = append(writes, freqWrites...)
		applied = append(applied, "frequency")
	} else if fixed, ok := eqBandFloat(selected, "fixed_freq_hz"); ok {
		frequencyQuantized = math.Abs(fixed-request.FreqHz) > 1e-9
		applied = append(applied, "frequency_selection")
	} else {
		return nil, nil, fmt.Errorf("selected section %v has no writable or fixed frequency", selected["section"])
	}

	if request.Q != nil {
		qWrites, qErr := eqWritesForRole(selected, "q", "q", *request.Q, 0.1, 6, "log")
		if qErr != nil {
			return nil, nil, fmt.Errorf("explicit_field_unavailable: requested Q cannot be applied: %w", qErr)
		}
		writes = append(writes, qWrites...)
		applied = append(applied, "q")
	}
	if request.SlopeDBPerOct != nil {
		slopeWrites, slopeErr := eqWritesForRole(selected, "slope", "slope", *request.SlopeDBPerOct, 6, 96, "linear")
		if slopeErr != nil {
			return nil, nil, fmt.Errorf("explicit_field_unavailable: requested slope cannot be applied: %w", slopeErr)
		}
		writes = append(writes, slopeWrites...)
		applied = append(applied, "slope_db_per_oct")
	}
	if request.GainDB != nil {
		gainWrites, gainErr := eqWritesForRole(selected, "gain", "gain", *request.GainDB, -24, 24, "linear")
		if gainErr != nil {
			return nil, nil, fmt.Errorf("requested gain cannot be applied: %w", gainErr)
		}
		writes = append(writes, gainWrites...)
		applied = append(applied, "gain_db")
	}

	activation := mapValue(selected["activation"])
	if active, _ := activation["active"].(bool); !active {
		switch firstNonEmptyText(activation, "strategy") {
		case "explicit_binding":
			writes, err = appendEQActivationLast(writes, selected)
			if err != nil {
				return nil, nil, err
			}
			applied = append(applied, "activation")
		case "implicit_in_domain":
			// Writing the frequency moves the control out of its sentinel "Out"
			// value and is itself the proven activation operation.
		case "property_sentinel":
			writes, err = appendEQPropertySentinelActivation(writes, selected)
			if err != nil {
				return nil, nil, err
			}
			applied = append(applied, "activation")
		default:
			return nil, nil, fmt.Errorf("inactive section %v has no safe activation strategy", selected["section"])
		}
	}
	if len(writes) == 0 {
		return nil, nil, fmt.Errorf("requested operation produced no writable parameters")
	}
	info := map[string]any{
		"section":             selected["section"],
		"shape":               request.Shape,
		"dedicated_kind":      selected["dedicated_kind"],
		"channel_bindings":    selected["channel_bindings"],
		"activation":          activation,
		"frequency_quantized": frequencyQuantized,
		"partial":             false,
		"applied_fields":      uniqueStrings(applied),
		"unsupported_fields":  []string{},
		"limitations":         []string{},
	}
	if fixed, ok := eqBandFloat(selected, "fixed_freq_hz"); ok {
		info["actual_selected_freq_hz"] = fixed
	} else if current, ok := eqSectionCurrentFrequency(selected); ok {
		info["previous_freq_hz"] = current
	}
	return writes, info, nil
}

func eqSectionActionSupported(section map[string]any, shape, action string) bool {
	for _, capability := range mapRowsValue(section["shape_capabilities"]) {
		if !strings.EqualFold(firstNonEmptyText(capability, "shape"), shape) {
			continue
		}
		actions := mapValue(capability["actions"])
		supported, _ := actions[action].(bool)
		return supported
	}
	// Legacy summaries predate the multi-axis capability projection.
	return action == "upsert" && eqSectionSupportsKind(section, shape)
}

func eqSectionFrequencyTargetInRange(section map[string]any, target float64) bool {
	bindings := mapRowsValue(section["frequency_bindings"])
	if len(bindings) == 0 {
		// Fixed-frequency sections are selected and reported as quantized later.
		_, fixed := eqBandFloat(section, "fixed_freq_hz")
		return fixed
	}
	_, _, err := eqFrequencyWritesForSection(section, target)
	return err == nil
}

// eqTypedSectionExplicitFieldsAvailable performs a dry, side-effect-free plan
// for every field the caller explicitly requested. Section ranking must only
// see candidates that can execute the whole atomic operation; otherwise a
// frequency-nearest section can mask another section whose Q/Gain/Slope domain
// actually satisfies the request.
func eqTypedSectionExplicitFieldsAvailable(section map[string]any, request eqTypedPointRequest) error {
	if _, err := eqFilterKindWrites(section, request.Shape); err != nil {
		return fmt.Errorf("shape cannot be applied: %w", err)
	}
	if len(mapRowsValue(section["frequency_bindings"])) > 0 {
		if _, _, err := eqFrequencyWritesForSection(section, request.FreqHz); err != nil {
			return fmt.Errorf("frequency cannot be applied: %w", err)
		}
	} else if _, ok := eqBandFloat(section, "fixed_freq_hz"); !ok {
		return fmt.Errorf("frequency cannot be applied: section has no writable or fixed frequency")
	}
	if request.Q != nil {
		if _, err := eqWritesForRole(section, "q", "q", *request.Q, 0.1, 6, "log"); err != nil {
			return fmt.Errorf("Q cannot be applied: %w", err)
		}
	}
	if request.SlopeDBPerOct != nil {
		if _, err := eqWritesForRole(section, "slope", "slope", *request.SlopeDBPerOct, 6, 96, "linear"); err != nil {
			return fmt.Errorf("slope cannot be applied: %w", err)
		}
	}
	if request.GainDB != nil {
		if _, err := eqWritesForRole(section, "gain", "gain", *request.GainDB, -24, 24, "linear"); err != nil {
			return fmt.Errorf("gain cannot be applied: %w", err)
		}
	}
	activation := mapValue(section["activation"])
	if active, _ := activation["active"].(bool); !active {
		switch firstNonEmptyText(activation, "strategy") {
		case "explicit_binding":
			if _, err := appendEQActivationLast(nil, section); err != nil {
				return fmt.Errorf("activation cannot be applied: %w", err)
			}
		case "implicit_in_domain":
			// The frequency write already proved the transition out of the
			// inactive sentinel domain.
		case "property_sentinel":
			if _, err := appendEQPropertySentinelActivation(nil, section); err != nil {
				return fmt.Errorf("activation cannot be applied: %w", err)
			}
		default:
			return fmt.Errorf("activation cannot be applied: no safe activation strategy")
		}
	}
	return nil
}

func eqSectionSupportsKind(section map[string]any, wanted string) bool {
	for _, value := range eqStringSlice(section["reachable_kinds"]) {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}

func eqStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out
	}
	return nil
}

func selectEQSectionByFrequency(sections []map[string]any, target float64, model string) map[string]any {
	var best map[string]any
	bestDistance := math.MaxFloat64
	if model == "free_floating" {
		for _, section := range sections {
			activation := mapValue(section["activation"])
			active, _ := activation["active"].(bool)
			current, ok := eqSectionCurrentFrequency(section)
			if active && ok && current > 0 && math.Abs(math.Log2(target/current)) < 0.5 {
				if distance := math.Abs(target - current); distance < bestDistance {
					best, bestDistance = section, distance
				}
			}
		}
		if best != nil {
			return best
		}
	}
	bestCenterDistance := math.MaxFloat64
	for _, section := range sections {
		score, ok := eqFrequencySelectionScore(section, target)
		if !ok {
			continue
		}
		if score.Distance < bestDistance-1e-9 ||
			(math.Abs(score.Distance-bestDistance) <= 1e-9 && score.CenterDistance < bestCenterDistance) {
			best, bestDistance, bestCenterDistance = section, score.Distance, score.CenterDistance
		}
	}
	return best
}

type eqFrequencyScore struct {
	Distance       float64
	CenterDistance float64
	ReachableHz    float64
}

func eqFrequencySelectionScore(row map[string]any, target float64) (eqFrequencyScore, bool) {
	if current, ok := eqSectionCurrentFrequency(row); ok {
		return eqFrequencyScore{Distance: math.Abs(target - current), ReachableHz: current}, true
	}
	values := []float64{}
	for _, binding := range mapRowsValue(row["frequency_bindings"]) {
		for _, reachable := range mapRowsValue(binding["reachable_values"]) {
			if physical, ok := eqBandFloatAny(reachable, "physical"); ok && physical > 0 {
				values = append(values, physical)
			}
		}
		for _, point := range eqBandCurve(binding, "curve") {
			if point[1] > 0 {
				values = append(values, point[1])
			}
		}
	}
	if len(values) == 0 {
		minValue, maxValue := math.MaxFloat64, -math.MaxFloat64
		for _, binding := range mapRowsValue(row["frequency_bindings"]) {
			domain := mapValue(binding["domain"])
			minimum, minOK := eqBandFloatAny(domain, "min")
			maximum, maxOK := eqBandFloatAny(domain, "max")
			if !minOK || !maxOK || minimum <= 0 || maximum <= 0 {
				continue
			}
			if minimum > maximum {
				minimum, maximum = maximum, minimum
			}
			minValue = math.Min(minValue, minimum)
			maxValue = math.Max(maxValue, maximum)
		}
		if minValue == math.MaxFloat64 || maxValue == -math.MaxFloat64 {
			return eqFrequencyScore{}, false
		}
		nearest := math.Max(minValue, math.Min(maxValue, target))
		center := math.Sqrt(minValue * maxValue)
		centerDistance := math.Abs(target - center)
		if target > 0 && center > 0 {
			centerDistance = math.Abs(math.Log(target / center))
		}
		return eqFrequencyScore{
			Distance:       math.Abs(target - nearest),
			CenterDistance: centerDistance,
			ReachableHz:    nearest,
		}, true
	}
	minValue, maxValue := values[0], values[0]
	bestValue, bestDistance := values[0], math.Abs(target-values[0])
	for _, value := range values[1:] {
		minValue = math.Min(minValue, value)
		maxValue = math.Max(maxValue, value)
		if distance := math.Abs(target - value); distance < bestDistance {
			bestValue, bestDistance = value, distance
		}
	}
	center := (minValue + maxValue) / 2
	if minValue > 0 && maxValue > 0 {
		center = math.Sqrt(minValue * maxValue)
	}
	centerDistance := math.Abs(target - center)
	if target > 0 && center > 0 {
		centerDistance = math.Abs(math.Log(target / center))
	}
	return eqFrequencyScore{Distance: bestDistance, CenterDistance: centerDistance, ReachableHz: bestValue}, true
}

func eqSectionCurrentFrequency(section map[string]any) (float64, bool) {
	if fixed, ok := eqBandFloat(section, "fixed_freq_hz"); ok {
		return fixed, true
	}
	for _, binding := range mapRowsValue(section["frequency_bindings"]) {
		if current, ok := eqBandFloatAny(binding, "current_physical"); ok {
			factor, factorOK := eqCurrentFrequencyTransformFactor(section)
			if !factorOK {
				factor = 1
			}
			return current * factor, true
		}
	}
	return 0, false
}

func eqCurrentFrequencyTransformFactor(section map[string]any) (float64, bool) {
	bindings := mapRowsValue(section["frequency_transform_bindings"])
	if len(bindings) == 0 {
		return 1, true
	}
	factor := 0.0
	for _, binding := range bindings {
		current, ok := eqBandFloatAny(binding, "current_physical")
		if !ok || current <= 0 {
			return 0, false
		}
		if factor == 0 {
			factor = current
		} else if math.Abs(factor-current) > 1e-9 {
			return 0, false
		}
	}
	return factor, factor > 0
}

func eqFilterKindWrites(section map[string]any, wanted string) ([]eqWriteStep, error) {
	bindings := mapRowsValue(section["filter_kind_bindings"])
	if len(bindings) == 0 {
		if strings.EqualFold(firstNonEmptyText(section, "dedicated_kind"), wanted) {
			return nil, nil
		}
		return nil, fmt.Errorf("section %v advertises %s but has no filter-kind binding", section["section"], wanted)
	}
	out := make([]eqWriteStep, 0, len(bindings))
	for _, binding := range bindings {
		found := false
		for _, row := range mapRowsValue(binding["reachable_values"]) {
			label := firstNonEmptyText(row, "label")
			if !strings.EqualFold(firstNonEmptyText(row, "kind"), wanted) &&
				!eqFilterKindLabelMatches(label, wanted, firstNonEmptyText(section, "section")) {
				continue
			}
			normalized, ok := eqBandFloatAny(row, "normalized")
			if !ok {
				continue
			}
			out = append(out, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"), Role: "shape",
				Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized,
				Quantized: true, RequestedLabel: wanted, ExpectedLabel: label})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("section %v filter-kind binding %s has no provable %s value",
				section["section"], firstNonEmptyText(binding, "param_id"), wanted)
		}
	}
	return out, nil
}

func eqFilterKindLabelMatches(label, wanted, sectionKey string) bool {
	canonical, err := normalizeRequestedEQShape(label)
	if err == nil && canonical == wanted {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(label), "shelf") {
		key := strings.ToLower(sectionKey)
		return wanted == "low_shelf" && containsEQToken(key, "low", "lo", "lf") ||
			wanted == "high_shelf" && containsEQToken(key, "high", "hi", "hf")
	}
	return false
}

func containsEQToken(text string, wanted ...string) bool {
	fields := strings.FieldsFunc(text, func(r rune) bool { return r < 'a' || r > 'z' })
	for _, field := range fields {
		for _, token := range wanted {
			if field == token {
				return true
			}
		}
	}
	return false
}

func normalizeRequestedEQShape(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return "", nil
	}
	compact := strings.NewReplacer(" ", "", "-", "", "_", "", "/", "").Replace(lower)
	switch compact {
	case "bell", "peak", "peaking", "pqbell", "parametric":
		return "bell", nil
	case "lowshelf", "loshelf":
		return "low_shelf", nil
	case "highshelf", "hishelf":
		return "high_shelf", nil
	case "lowcut", "locut", "highpass", "hipass", "hp":
		return "low_cut", nil
	case "highcut", "hicut", "lowpass", "lopass", "lp":
		return "high_cut", nil
	}
	return "", fmt.Errorf("unsupported EQ shape %q; use bell, low_shelf, high_shelf, low_cut, or high_cut", value)
}

func eqBandWritePlan(summary map[string]any, freqHz, gainDB float64, q *float64) ([]eqWriteStep, map[string]any, error) {
	if supported, ok := summary["set_eq_point_supported"].(bool); ok && !supported {
		return nil, nil, fmt.Errorf("set_eq_point is not supported: %s", firstNonEmptyText(summary, "reason"))
	}
	if completeness := mapValue(summary["completeness"]); len(completeness) > 0 {
		if complete, ok := completeness["complete"].(bool); ok && !complete {
			return nil, nil, fmt.Errorf("EQ structure is incomplete: %s", firstNonEmptyText(summary, "reason"))
		}
	}
	model := firstNonEmptyText(summary, "eq_model")
	switch model {
	case "fixed_slot_adjustable":
		return eqWritePlanFixedSlotAdjustable(mapRowsValue(summary["bands"]), freqHz, gainDB, q)
	case "fixed_freq":
		return eqWritePlanFixedFreq(mapRowsValue(summary["bands"]), freqHz, gainDB, q)
	case "free_floating":
		return eqWritePlanFreeFloating(mapRowsValue(summary["active_bands"]), mapRowsValue(summary["available_slots"]), freqHz, gainDB, q)
	default:
		return nil, nil, fmt.Errorf("unknown eq_model %q", model)
	}
}

func eqWritePlanFixedSlotAdjustable(bands []map[string]any, freqHz, gainDB float64, q *float64) ([]eqWriteStep, map[string]any, error) {
	if len(bands) == 0 {
		return nil, nil, fmt.Errorf("fixed_slot_adjustable EQ has no bands")
	}
	var best map[string]any
	bestDist := math.MaxFloat64
	bestCenterDistance := math.MaxFloat64
	bestReachable := 0.0
	for _, b := range bands {
		score, ok := eqFrequencySelectionScore(b, freqHz)
		if !ok {
			continue
		}
		if score.Distance < bestDist-1e-9 ||
			(math.Abs(score.Distance-bestDist) <= 1e-9 && score.CenterDistance < bestCenterDistance) {
			best, bestDist, bestCenterDistance, bestReachable = b, score.Distance, score.CenterDistance, score.ReachableHz
		}
	}
	if best == nil {
		return nil, nil, fmt.Errorf("fixed_slot_adjustable EQ has no recoverable current or reachable band frequency")
	}
	writes := []eqWriteStep{}
	shapeWrites, err := eqOptionalBellWrites(best)
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, shapeWrites...)
	freqWrites, err := eqWritesForRole(best, "frequency", "freq", freqHz, 20, 20000, "log")
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, freqWrites...)
	gainWrites, err := eqWritesForRole(best, "gain", "gain", gainDB, -24, 24, "linear")
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, gainWrites...)
	writes, err = appendEQQWrite(writes, best, q)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": best["band"], "current_freq_hz": best["current_freq_hz"],
		"nearest_reachable_freq_hz": bestReachable, "freq_distance_hz": bestDist}
	writes, err = appendEQActivationLast(writes, best)
	if err != nil {
		return nil, nil, err
	}
	return writes, info, nil
}

func eqOptionalBellWrites(band map[string]any) ([]eqWriteStep, error) {
	bindings := mapRowsValue(band["filter_kind_bindings"])
	if len(bindings) == 0 {
		bindings = mapRowsValue(band["shape_bindings"])
	}
	if len(bindings) == 0 {
		return nil, nil
	}
	out := make([]eqWriteStep, 0, len(bindings))
	for _, binding := range bindings {
		found := false
		for _, row := range mapRowsValue(binding["reachable_values"]) {
			label := firstNonEmptyText(row, "label")
			if !strings.EqualFold(firstNonEmptyText(row, "kind"), "bell") &&
				!eqFilterKindLabelMatches(label, "bell", firstNonEmptyText(band, "band")) {
				continue
			}
			normalized, ok := eqBandFloatAny(row, "normalized")
			if !ok {
				continue
			}
			out = append(out, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"), Role: "shape",
				Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized,
				Quantized: true, RequestedLabel: "bell", ExpectedLabel: label})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("band %v exposes a filter-kind binding but Bell is not provably reachable", band["band"])
		}
	}
	return out, nil
}

func eqWritePlanFixedFreq(bands []map[string]any, freqHz, gainDB float64, q *float64) ([]eqWriteStep, map[string]any, error) {
	if len(bands) == 0 {
		return nil, nil, fmt.Errorf("fixed_freq EQ has no bands")
	}
	var best map[string]any
	bestDist := math.MaxFloat64
	for _, b := range bands {
		fixed, ok := eqBandFloat(b, "fixed_freq_hz")
		if !ok {
			continue
		}
		if d := math.Abs(freqHz - fixed); d < bestDist {
			bestDist = d
			best = b
		}
	}
	// No band publishes a centre frequency (e.g. Marvel GEQ's opaque
	// "1EQ0".."1EQ15"). Falling back to the first band would write a plausible
	// gain change at an unknown frequency and report success, so refuse instead.
	if best == nil {
		return nil, nil, fmt.Errorf("this EQ has %d fixed-frequency bands but publishes no centre "+
			"frequency for any of them, so the band nearest %.0f Hz cannot be identified", len(bands), freqHz)
	}
	writes, err := eqWritesForRole(best, "gain", "gain", gainDB, -24, 24, "linear")
	if err != nil {
		return nil, nil, err
	}
	writes, err = appendEQQWrite(writes, best, q)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": best["band"], "fixed_freq_hz": best["fixed_freq_hz"], "freq_distance_hz": bestDist}
	return writes, info, nil
}

func eqWritePlanFreeFloating(active, available []map[string]any, freqHz, gainDB float64, q *float64) ([]eqWriteStep, map[string]any, error) {
	// Prefer an active band within half an octave: just adjust its gain.
	for _, b := range active {
		cur, ok := eqBandFloat(b, "current_freq_hz")
		if !ok {
			continue
		}
		if cur > 0 && math.Abs(math.Log2(freqHz/cur)) < 0.5 {
			writes, err := eqWritesForRole(b, "gain", "gain", gainDB, -24, 24, "linear")
			if err != nil {
				continue
			}
			writes, err = appendEQQWrite(writes, b, q)
			if err != nil {
				return nil, nil, err
			}
			info := map[string]any{"band": b["band"], "operation": "adjust_existing", "current_freq_hz": cur}
			return writes, info, nil
		}
	}
	// No nearby active band — create from the first available slot.
	if len(available) == 0 {
		return nil, nil, fmt.Errorf("free_floating EQ has no available slots to create a new band")
	}
	slot := available[0]
	writes := []eqWriteStep{}
	shapeWrites, _ := eqEnumWritesForRole(slot, "shape", "shape", []string{"bell", "peak"})
	writes = append(writes, shapeWrites...)
	freqWrites, err := eqWritesForRole(slot, "frequency", "freq", freqHz, 20, 20000, "log")
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, freqWrites...)
	gainWrites, err := eqWritesForRole(slot, "gain", "gain", gainDB, -24, 24, "linear")
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, gainWrites...)
	writes, err = appendEQQWrite(writes, slot, q)
	if err != nil {
		return nil, nil, err
	}
	writes, err = appendEQActivationLast(writes, slot)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": slot["band"], "operation": "create_band"}
	return writes, info, nil
}

// appendEQQWrite adds the Q write when the selected band has one.
//
// Missing Q is valid only when Q was not requested.  An explicit Q target with
// no binding rejects the whole plan before any parameter is touched.
func appendEQQWrite(writes []eqWriteStep, band map[string]any, q *float64) ([]eqWriteStep, error) {
	if q == nil {
		return writes, nil
	}
	qWrites, err := eqWritesForRole(band, "q", "q", *q, 0.1, 6, "log")
	if err != nil {
		return nil, fmt.Errorf("requested Q cannot be applied: %w", err)
	}
	return append(writes, qWrites...), nil
}

func eqWritesForRole(band map[string]any, summaryRole, writeRole string, target,
	fallbackMin, fallbackMax float64, fallbackScale string) ([]eqWriteStep, error) {
	bindings := mapRowsValue(band[summaryRole+"_bindings"])
	if len(bindings) == 0 {
		legacy := summaryRole
		if summaryRole == "frequency" {
			legacy = "freq"
		}
		paramID := firstNonEmptyText(band, legacy+"_param_id")
		if paramID == "" {
			return nil, fmt.Errorf("band %v is missing %s binding", band["band"], summaryRole)
		}
		bindings = []map[string]any{{
			"param_id": paramID,
			"domain":   band[legacy+"_domain"],
			"curve":    band[legacy+"_curve"],
			"channel":  "shared",
		}}
	}
	out := make([]eqWriteStep, 0, len(bindings))
	for _, binding := range bindings {
		paramID := firstNonEmptyText(binding, "param_id")
		if paramID == "" {
			return nil, fmt.Errorf("band %v has an empty %s binding", band["band"], summaryRole)
		}
		minimum, maximum, ok := eqBindingTargetRange(binding, fallbackMin, fallbackMax)
		if ok && !eqPhysicalTargetInRange(target, minimum, maximum) {
			return nil, fmt.Errorf("requested %s %g is outside parameter %s physical domain [%g, %g]",
				summaryRole, target, paramID, minimum, maximum)
		}
		normalized, quantized := eqBindingTarget(target, binding, fallbackMin, fallbackMax, fallbackScale)
		low, high := eqBindingCorrectionBounds(target, binding)
		curve := eqBandCurve(binding, "curve")
		decreasing := len(curve) >= 2 && curve[len(curve)-1][1] < curve[0][1]
		discretePhysical := strings.EqualFold(firstNonEmptyText(mapValue(binding["domain"]), "scale"), "enum") &&
			len(mapRowsValue(binding["reachable_values"])) > 0
		requested := target
		out = append(out, eqWriteStep{ParamID: paramID, Role: writeRole,
			Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized,
			RequestedPhysical: &requested, Quantized: quantized,
			CorrectionLow: low, CorrectionHigh: high, PhysicalDecreasing: decreasing,
			DiscretePhysical: discretePhysical})
	}
	return out, nil
}

func eqFrequencyWritesForSection(section map[string]any, effectiveTarget float64) ([]eqWriteStep, bool, error) {
	transforms := mapRowsValue(section["frequency_transform_bindings"])
	if len(transforms) == 0 {
		writes, err := eqWritesForRole(section, "frequency", "freq", effectiveTarget, 20, 20000, "log")
		return writes, eqWritesAreQuantized(writes), err
	}
	factors := eqCommonFrequencyTransformFactors(transforms)
	if len(factors) == 0 {
		return nil, false, fmt.Errorf("section %v has no common reachable frequency transform", section["section"])
	}
	if current, ok := eqCurrentFrequencyTransformFactor(section); ok {
		for index, factor := range factors {
			if math.Abs(factor-current) <= 1e-9 {
				factors[0], factors[index] = factors[index], factors[0]
				break
			}
		}
	}
	var failures []string
	for _, factor := range factors {
		baseTarget := effectiveTarget / factor
		frequencyWrites, err := eqWritesForRole(section, "frequency", "freq", baseTarget, 20, 20000, "log")
		if err != nil {
			failures = append(failures, fmt.Sprintf("x%g: %v", factor, err))
			continue
		}
		for index := range frequencyWrites {
			requested := effectiveTarget
			frequencyWrites[index].RequestedPhysical = &requested
			frequencyWrites[index].PhysicalMultiplier = factor
		}
		transformWrites, err := eqFrequencyTransformWrites(transforms, factor)
		if err != nil {
			failures = append(failures, fmt.Sprintf("x%g: %v", factor, err))
			continue
		}
		writes := append(frequencyWrites, transformWrites...)
		return writes, eqWritesAreQuantized(frequencyWrites), nil
	}
	return nil, false, fmt.Errorf("requested frequency %g is unavailable through section transforms (%s)",
		effectiveTarget, strings.Join(failures, "; "))
}

func eqCommonFrequencyTransformFactors(bindings []map[string]any) []float64 {
	common := []float64{}
	for index, binding := range bindings {
		values := []float64{}
		for _, row := range mapRowsValue(binding["reachable_values"]) {
			if factor, ok := eqBandFloatAny(row, "physical"); ok && factor > 0 {
				values = append(values, factor)
			}
		}
		if index == 0 {
			common = values
			continue
		}
		filtered := common[:0]
		for _, candidate := range common {
			for _, value := range values {
				if math.Abs(candidate-value) <= 1e-9 {
					filtered = append(filtered, candidate)
					break
				}
			}
		}
		common = filtered
	}
	sort.Float64s(common)
	return common
}

func eqFrequencyTransformWrites(bindings []map[string]any, factor float64) ([]eqWriteStep, error) {
	writes := []eqWriteStep{}
	for _, binding := range bindings {
		current, currentOK := eqBandFloatAny(binding, "current_physical")
		if currentOK && math.Abs(current-factor) <= 1e-9 {
			continue
		}
		found := false
		for _, row := range mapRowsValue(binding["reachable_values"]) {
			physical, physicalOK := eqBandFloatAny(row, "physical")
			normalized, normalizedOK := eqBandFloatAny(row, "normalized")
			if !physicalOK || !normalizedOK || math.Abs(physical-factor) > 1e-9 {
				continue
			}
			label := firstNonEmptyText(row, "label")
			writes = append(writes, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"),
				Role: "frequency_transform", Channel: firstNonEmptyText(binding, "channel"),
				NormalizedValue: normalized, Quantized: true, RequestedLabel: fmt.Sprintf("x%g", factor), ExpectedLabel: label})
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("frequency transform binding %s has no x%g value",
				firstNonEmptyText(binding, "param_id"), factor)
		}
	}
	return writes, nil
}

func eqBindingTargetRange(binding map[string]any, fallbackMin, fallbackMax float64) (float64, float64, bool) {
	values := make([]float64, 0)
	for _, row := range mapRowsValue(binding["reachable_values"]) {
		if physical, ok := eqBandFloatAny(row, "physical"); ok && !math.IsNaN(physical) && !math.IsInf(physical, 0) {
			values = append(values, physical)
		}
	}
	if len(values) == 0 {
		for _, point := range eqBandCurve(binding, "curve") {
			if !math.IsNaN(point[1]) && !math.IsInf(point[1], 0) {
				values = append(values, point[1])
			}
		}
	}
	if len(values) > 0 {
		minimum, maximum := values[0], values[0]
		for _, value := range values[1:] {
			minimum, maximum = math.Min(minimum, value), math.Max(maximum, value)
		}
		return minimum, maximum, true
	}
	domain := mapValue(binding["domain"])
	minimum, minOK := eqBandFloatAny(domain, "min")
	maximum, maxOK := eqBandFloatAny(domain, "max")
	if minOK && maxOK && !math.IsNaN(minimum) && !math.IsNaN(maximum) &&
		!math.IsInf(minimum, 0) && !math.IsInf(maximum, 0) {
		if minimum > maximum {
			minimum, maximum = maximum, minimum
		}
		return minimum, maximum, true
	}
	if !math.IsNaN(fallbackMin) && !math.IsNaN(fallbackMax) &&
		!math.IsInf(fallbackMin, 0) && !math.IsInf(fallbackMax, 0) {
		if fallbackMin > fallbackMax {
			fallbackMin, fallbackMax = fallbackMax, fallbackMin
		}
		return fallbackMin, fallbackMax, true
	}
	return 0, 0, false
}

func eqPhysicalTargetInRange(target, minimum, maximum float64) bool {
	margin := 1e-9 * math.Max(1, math.Max(math.Abs(minimum), math.Abs(maximum)))
	return target >= minimum-margin && target <= maximum+margin
}

func eqBindingTarget(target float64, binding map[string]any,
	fallbackMin, fallbackMax float64, fallbackScale string) (float64, bool) {
	reachable := mapRowsValue(binding["reachable_values"])
	bestDistance := math.MaxFloat64
	best := 0.0
	found := false
	for _, row := range reachable {
		physical, ok := eqBandFloatAny(row, "physical")
		if !ok {
			continue
		}
		normalized, normalizedOK := eqBandFloatAny(row, "normalized")
		if !normalizedOK {
			continue
		}
		if distance := math.Abs(target - physical); distance < bestDistance {
			bestDistance, best, found = distance, normalized, true
		}
	}
	if found {
		return best, bestDistance > 1e-9
	}
	curve := eqBandCurve(binding, "curve")
	domain := mapValue(binding["domain"])
	return eqNormalizeTarget(target, curve, domain, fallbackMin, fallbackMax, fallbackScale), false
}

func eqBindingCorrectionBounds(target float64, binding map[string]any) (float64, float64) {
	curve := eqBandCurve(binding, "curve")
	if len(curve) < 2 {
		return 0, 1
	}
	increasing := curve[len(curve)-1][1] > curve[0][1]
	for i := 1; i < len(curve); i++ {
		if increasing && target <= curve[i][1] || !increasing && target >= curve[i][1] {
			return curve[i-1][0], curve[i][0]
		}
	}
	return curve[len(curve)-2][0], curve[len(curve)-1][0]
}

func eqEnumWritesForRole(band map[string]any, summaryRole, writeRole string, preferred []string) ([]eqWriteStep, error) {
	bindings := mapRowsValue(band[summaryRole+"_bindings"])
	out := []eqWriteStep{}
	for _, binding := range bindings {
		value, ok := eqReachableLabelTarget(binding, preferred)
		if !ok {
			continue
		}
		out = append(out, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"), Role: writeRole,
			Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: value, Quantized: true})
	}
	return out, nil
}

func appendEQActivationLast(writes []eqWriteStep, band map[string]any) ([]eqWriteStep, error) {
	if active, ok := band["active"].(bool); ok && active {
		return writes, nil
	}
	if firstNonEmptyText(band, "activation_strategy") == "implicit_in_domain" {
		return writes, nil
	}
	bindings := mapRowsValue(band["activation_bindings"])
	if len(bindings) == 0 {
		return nil, fmt.Errorf("band %v is inactive and has no activation binding", band["band"])
	}
	for _, binding := range bindings {
		value, ok := eqReachableLabelTarget(binding, []string{"used", "enabled", "active", "on", "in", "not bypassed"})
		if !ok {
			return nil, fmt.Errorf("band %v activation binding %s has no provable active value", band["band"], firstNonEmptyText(binding, "param_id"))
		}
		writes = append(writes, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"), Role: "used",
			Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: value, Quantized: true})
	}
	return writes, nil
}

func appendEQPropertySentinelActivation(writes []eqWriteStep, section map[string]any) ([]eqWriteStep, error) {
	bindings := mapRowsValue(section["slope_bindings"])
	if len(bindings) == 0 {
		return nil, fmt.Errorf("section %v has property-sentinel activation but no slope binding", section["section"])
	}
	for _, binding := range bindings {
		paramID := firstNonEmptyText(binding, "param_id")
		planned := false
		for index := range writes {
			if writes[index].ParamID != paramID {
				continue
			}
			writes[index].ActivationLast = true
			planned = true
			break
		}
		if planned {
			continue
		}
		if !eqActivationReadbackInactive(firstNonEmptyText(binding, "current_text")) {
			continue
		}
		normalized, label, ok := eqLowestActiveSlopeTarget(binding)
		if !ok {
			return nil, fmt.Errorf("section %v slope binding %s has no provable active slope value",
				section["section"], paramID)
		}
		writes = append(writes, eqWriteStep{ParamID: paramID, Role: "used",
			Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized, Quantized: true,
			RequestedLabel: "active", ExpectedLabel: label, ActivationLast: true})
	}
	return writes, nil
}

func eqLowestActiveSlopeTarget(binding map[string]any) (float64, string, bool) {
	bestPhysical := math.MaxFloat64
	bestNormalized := 0.0
	bestLabel := ""
	for _, row := range mapRowsValue(binding["reachable_values"]) {
		label := firstNonEmptyText(row, "label")
		if eqActivationReadbackInactive(label) {
			continue
		}
		physical, physicalOK := eqBandFloatAny(row, "physical")
		normalized, normalizedOK := eqBandFloatAny(row, "normalized")
		if !physicalOK || !normalizedOK || physical <= 0 || physical >= bestPhysical {
			continue
		}
		bestPhysical, bestNormalized, bestLabel = physical, normalized, label
	}
	return bestNormalized, bestLabel, bestLabel != ""
}

func eqReachableLabelTarget(binding map[string]any, preferred []string) (float64, bool) {
	rows := mapRowsValue(binding["reachable_values"])
	// Exact matches must win globally. A substring-first search would match
	// "used" against "Unused" and leave a free-floating band inactive.
	for _, want := range preferred {
		for _, row := range rows {
			label := strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "label")))
			if label == want {
				value, ok := eqBandFloatAny(row, "normalized")
				if ok {
					return value, true
				}
			}
		}
	}
	for _, want := range preferred {
		for _, row := range rows {
			label := strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "label")))
			fields := strings.FieldsFunc(label, func(r rune) bool { return r < 'a' || r > 'z' })
			for _, field := range fields {
				if field == want {
					value, ok := eqBandFloatAny(row, "normalized")
					if ok {
						return value, true
					}
				}
			}
		}
	}
	return 0, false
}

func eqBandFloatAny(row map[string]any, key string) (float64, bool) {
	switch value := row[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	}
	return 0, false
}

// eqNormalizeTarget converts a target display value into the normalised value to
// write, preferring the parameter's measured response curve over the
// min/max/scale summary.
//
// The summary's "scale" is only ever "log" or "linear", and it is inferred partly
// from the declared unit — neither of which describes what the plugin actually
// does. FreeEQ8 maps frequency quadratically and ZamEQ2 maps it linearly while
// declaring Hz; applying a logarithmic formula to those put a 3400 Hz request at
// 11064 Hz and 5119 Hz respectively, with a clean readback and a success status.
func eqNormalizeTarget(target float64, curve [][2]float64, domain map[string]any,
	fallbackMin, fallbackMax float64, fallbackScale string) float64 {
	if value, ok := eqNormalizedFromCurve(target, curve); ok {
		return value
	}
	return normalizeEQValue(target,
		eqDomainMin(domain, fallbackMin),
		eqDomainMax(domain, fallbackMax),
		eqDomainScale(domain, fallbackScale))
}

// eqNormalizedFromCurve inverts a measured response curve.
//
// Two curve families cover every plugin measured so far, and they are told apart
// by which one actually fits the samples rather than by any declared metadata:
//
//	logarithmic  v = lo · (hi/lo)^n     TDR Nova, Pro-Q 3
//	power law    v = lo + (hi-lo)·n^k   FreeEQ8 (k=2), ZamEQ2 (k=1, i.e. linear),
//	                                    and every dB gain fader measured
//
// k is recovered from the interior samples and the winner is chosen by residual
// against all five points, so a plugin whose curve is neither will produce a
// visibly poor fit rather than a confidently wrong write.
func eqNormalizedFromCurve(target float64, curve [][2]float64) (float64, bool) {
	if len(curve) < 3 {
		return 0, false
	}
	first, last := curve[0], curve[len(curve)-1]
	lo, hi := first[1], last[1]
	if hi < lo {
		if target >= lo {
			return clampUnit(first[0]), true
		}
		if target <= hi {
			return clampUnit(last[0]), true
		}
		for i := 1; i < len(curve); i++ {
			upper, lower := curve[i-1], curve[i]
			if target <= upper[1] && target >= lower[1] {
				fraction := (upper[1] - target) / (upper[1] - lower[1])
				return clampUnit(upper[0] + fraction*(lower[0]-upper[0])), true
			}
		}
		return 0, false
	}
	span := hi - lo
	if span <= 0 {
		return 0, false
	}
	if target <= lo {
		return clampUnit(first[0]), true
	}
	if target >= hi {
		return clampUnit(last[0]), true
	}

	powerExponent, powerSamples := 0.0, 0
	for _, point := range curve[1 : len(curve)-1] {
		n, value := point[0], point[1]
		fraction := (value - lo) / span
		if n <= 0 || n >= 1 || fraction <= 0 || fraction >= 1 {
			continue
		}
		powerExponent += math.Log(fraction) / math.Log(n)
		powerSamples++
	}
	powerResidual := math.Inf(1)
	if powerSamples > 0 {
		powerExponent /= float64(powerSamples)
		if powerExponent > 0 {
			powerResidual = eqCurveResidual(curve, span, func(n float64) float64 {
				return lo + span*math.Pow(n, powerExponent)
			})
		}
	}

	logResidual := math.Inf(1)
	if lo > 0 {
		logResidual = eqCurveResidual(curve, span, func(n float64) float64 {
			return lo * math.Pow(hi/lo, n)
		})
	}

	switch {
	case math.IsInf(logResidual, 1) && math.IsInf(powerResidual, 1):
		return 0, false
	case logResidual <= powerResidual:
		return clampUnit(math.Log(target/lo) / math.Log(hi/lo)), true
	default:
		return clampUnit(math.Pow((target-lo)/span, 1/powerExponent)), true
	}
}

// eqCurveResidual scores a candidate curve against the measured points. The error
// is scaled by the full span rather than by each sample, so a gain curve passing
// through zero does not blow the score up.
func eqCurveResidual(curve [][2]float64, span float64, predict func(float64) float64) float64 {
	total := 0.0
	for _, point := range curve {
		total += math.Abs(predict(point[0])-point[1]) / span
	}
	return total
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func eqBandCurve(band map[string]any, key string) [][2]float64 {
	switch typed := band[key].(type) {
	case [][2]float64:
		return typed
	case []any:
		out := make([][2]float64, 0, len(typed))
		for _, item := range typed {
			pair, ok := item.([]any)
			if !ok || len(pair) < 2 {
				return nil
			}
			n, nOK := pair[0].(float64)
			v, vOK := pair[1].(float64)
			if !nOK || !vOK {
				return nil
			}
			out = append(out, [2]float64{n, v})
		}
		return out
	}
	return nil
}

func normalizeEQValue(target, minVal, maxVal float64, scale string) float64 {
	var v float64
	if scale == "log" && minVal > 0 && maxVal > minVal && target > 0 {
		v = (math.Log10(target) - math.Log10(minVal)) / (math.Log10(maxVal) - math.Log10(minVal))
	} else {
		denom := maxVal - minVal
		if denom == 0 {
			return 0.5
		}
		v = (target - minVal) / denom
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func eqBandFloat(band map[string]any, key string) (float64, bool) {
	v := band[key]
	switch x := v.(type) {
	case float64:
		return x, x > 0
	case float32:
		return float64(x), float64(x) > 0
	case int:
		return float64(x), x > 0
	}
	return 0, false
}

func eqDomainMin(d map[string]any, fallback float64) float64 {
	if v, ok := d["min"].(float64); ok {
		return v
	}
	return fallback
}

func eqDomainMax(d map[string]any, fallback float64) float64 {
	if v, ok := d["max"].(float64); ok {
		return v
	}
	return fallback
}

func eqDomainScale(d map[string]any, fallback string) string {
	if s, ok := d["scale"].(string); ok && s != "" {
		return s
	}
	return fallback
}

func parseFloat64Arg(args map[string]any, key string) (float64, error) {
	v := args[key]
	if v == nil {
		return 0, fmt.Errorf("required argument %q is missing", key)
	}
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as number for %q", x, key)
		}
		return f, nil
	}
	return 0, fmt.Errorf("unsupported type %T for argument %q", v, key)
}

func parseOptionalFloat64Arg(args map[string]any, key string) (float64, bool, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	if text, ok := v.(string); ok && strings.TrimSpace(text) == "" {
		return 0, false, nil
	}
	f, err := parseFloat64Arg(args, key)
	return f, true, err
}
