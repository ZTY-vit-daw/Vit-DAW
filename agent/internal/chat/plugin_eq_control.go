package chat

import (
	"context"
	"fmt"
	"math"
	"regexp"
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
	fail := func(err error) ChatResponse {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	target, err := s.resolvePluginLearningTarget(ctx, workflowCmd, requestContext, "")
	if err != nil {
		return fail(err)
	}
	if s.kernel == nil {
		return fail(fmt.Errorf("kernel client is nil"))
	}

	args := workflowCommandArgs(workflowCmd)
	freqHz, err := parseFloat64Arg(args, "freq_hz")
	if err != nil {
		return fail(fmt.Errorf("freq_hz: %w", err))
	}
	gainDB, gainProvided, err := parseOptionalFloat64Arg(args, "gain_db")
	if err != nil {
		return fail(fmt.Errorf("gain_db: %w", err))
	}
	if freqHz <= 0 {
		return fail(fmt.Errorf("freq_hz must be positive, got %g", freqHz))
	}
	qValue, qProvided, err := parseOptionalFloat64Arg(args, "q")
	if err != nil {
		return fail(fmt.Errorf("q: %w", err))
	}
	if qProvided && qValue <= 0 {
		return fail(fmt.Errorf("q must be positive, got %g", qValue))
	}
	shape, err := normalizeRequestedEQShape(firstNonEmptyText(args, "shape", "filter_type"))
	if err != nil {
		return fail(err)
	}
	slopeValue, slopeProvided, err := parseOptionalFloat64Arg(args, "slope_db_per_oct")
	if err != nil {
		return fail(fmt.Errorf("slope_db_per_oct: %w", err))
	}
	if slopeProvided && slopeValue <= 0 {
		return fail(fmt.Errorf("slope_db_per_oct must be positive, got %g", slopeValue))
	}
	if shape == "" && !gainProvided {
		return fail(fmt.Errorf("gain_db is required when shape is not specified"))
	}
	if (shape == "bell" || shape == "low_shelf" || shape == "high_shelf") && !gainProvided {
		return fail(fmt.Errorf("gain_db is required for shape %s", shape))
	}
	var qTarget *float64
	if qProvided {
		qTarget = &qValue
	}

	// Read live parameters — also seeds the snapshot cache so harness.Invoke's
	// validateSetPluginParam guard passes without a separate pre-read.
	paramsReply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  target.TrackID,
		"plugin_id": target.PluginID,
	})
	if err != nil {
		return fail(fmt.Errorf("get_plugin_parameters: %w", err))
	}
	if !kernelReplyOK(paramsReply) {
		msg := firstNonEmptyText(paramsReply, "message", "error")
		if msg == "" {
			msg = "get_plugin_parameters failed"
		}
		return fail(fmt.Errorf("%s", msg))
	}
	s.observePluginParametersReply(paramsReply)

	digest := buildPluginParameterDigest(paramsReply)
	summary := plugingrabber.BuildEQBandSummary(digest)
	if summary == nil {
		return fail(fmt.Errorf("plugin %s/%s is not recognised as an EQ (no 'Band N Frequency' parameters found)", target.TrackID, target.PluginID))
	}

	var writes []eqWriteStep
	var selectionInfo map[string]any
	if shape == "" {
		writes, selectionInfo, err = eqBandWritePlan(summary, freqHz, gainDB, qTarget)
	} else {
		request := eqTypedPointRequest{FreqHz: freqHz, Shape: shape, Q: qTarget}
		if gainProvided {
			request.GainDB = &gainDB
		}
		if slopeProvided {
			request.SlopeDBPerOct = &slopeValue
		}
		writes, selectionInfo, err = eqTypedSectionWritePlan(summary, request)
	}
	if err != nil {
		return fail(err)
	}

	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return fail(err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, target.TrackID, target.PluginID, writes, preimage)
	if err != nil {
		return fail(err)
	}

	result := map[string]any{
		"status":          "ok",
		"track_id":        target.TrackID,
		"plugin_id":       target.PluginID,
		"requested":       map[string]any{"freq_hz": freqHz},
		"eq_model":        summary["eq_model"],
		"mapping_source":  summary["mapping_source"],
		"selected_band":   selectionInfo,
		"writes":          executed,
		"actual_readback": actual,
	}
	if gainProvided {
		result["requested"].(map[string]any)["gain_db"] = gainDB
	}
	if qProvided {
		result["requested"].(map[string]any)["q"] = qValue
	}
	if shape != "" {
		result["requested"].(map[string]any)["shape"] = shape
		result["selected_section"] = selectionInfo
	}
	if slopeProvided {
		result["requested"].(map[string]any)["slope_db_per_oct"] = slopeValue
	}
	for _, key := range []string{"partial", "applied_fields", "unsupported_fields", "limitations"} {
		if value, ok := selectionInfo[key]; ok {
			result[key] = value
		}
	}
	return ChatResponse{
		ConversationID: conversationID,
		ExecutedKernelReply: []map[string]any{
			{"status": "ok", "command_name": pluginGrabberSetEQPointCommand, "result": result},
		},
	}
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
	writes []eqWriteStep, preimage []eqPreimageValue) ([]map[string]any, []map[string]any, error) {
	if s.kernel == nil {
		return nil, nil, fmt.Errorf("kernel client is nil")
	}
	parameters := make([]map[string]any, 0, len(writes))
	for _, write := range writes {
		parameters = append(parameters, map[string]any{"parameter_id": write.ParamID, "normalized_value": write.NormalizedValue})
	}
	result, err := s.kernel.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "parameters": parameters, "readback": true,
	})
	if err != nil || eqVSPFailure(result) != "" {
		failure := firstNonEmpty(errorText(err), eqVSPFailure(result), "EQ parameter batch failed")
		rollbackErr := s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
		if rollbackErr != nil {
			return nil, nil, fmt.Errorf("%s; rollback failed: %w", failure, rollbackErr)
		}
		return nil, nil, fmt.Errorf("%s; full preimage restored", failure)
	}

	actualRows, err := s.readEQParameters(ctx, trackID, pluginID)
	if err != nil {
		rollbackErr := s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
		if rollbackErr != nil {
			return nil, nil, fmt.Errorf("fresh readback failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return nil, nil, fmt.Errorf("fresh readback failed: %w; full preimage restored", err)
	}
	index := eqParameterRowIndex(actualRows)
	if corrected, correctionErr := s.correctEQPhysicalReadback(ctx, trackID, pluginID, writes, index); correctionErr != nil {
		rollbackErr := s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
		if rollbackErr != nil {
			return nil, nil, fmt.Errorf("%v; rollback failed: %v", correctionErr, rollbackErr)
		}
		return nil, nil, fmt.Errorf("%v; full preimage restored", correctionErr)
	} else if corrected {
		actualRows, err = s.readEQParameters(ctx, trackID, pluginID)
		if err != nil {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, err
		}
		index = eqParameterRowIndex(actualRows)
	}
	executed := make([]map[string]any, 0, len(writes))
	actual := make([]map[string]any, 0, len(writes))
	for _, write := range writes {
		row, ok := index[write.ParamID]
		if !ok {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, fmt.Errorf("fresh readback omitted touched parameter %s; full preimage restored", write.ParamID)
		}
		actualNormalized, ok := firstNumericAny(row, "normalized_value", "normalised_value", "current_normalized_value")
		if !ok {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, fmt.Errorf("fresh readback has no normalized value for %s; full preimage restored", write.ParamID)
		}
		if math.Abs(actualNormalized-write.NormalizedValue) > 1e-4 {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, fmt.Errorf("parameter %s normalized readback mismatch requested %.9f actual %.9f; full preimage restored", write.ParamID, write.NormalizedValue, actualNormalized)
		}
		if write.ExpectedLabel != "" && !strings.EqualFold(strings.TrimSpace(firstNonEmptyText(row, "value_text")), strings.TrimSpace(write.ExpectedLabel)) {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, fmt.Errorf("parameter %s enum readback mismatch requested %s actual %s; full preimage restored",
				write.ParamID, write.ExpectedLabel, firstNonEmptyText(row, "value_text"))
		}
		if write.Role == "used" && eqActivationReadbackInactive(firstNonEmptyText(row, "value_text")) {
			_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
			return nil, nil, fmt.Errorf("parameter %s remained inactive after activation (%s); full preimage restored",
				write.ParamID, firstNonEmptyText(row, "value_text"))
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
		physical, physicalOK := physicalReadbackForRole(write.Role, row)
		if write.RequestedPhysical != nil && isEQPhysicalRole(write.Role) && !write.Quantized {
			if !physicalOK || math.Abs(physical-*write.RequestedPhysical) > eqPhysicalTolerance(write.Role, *write.RequestedPhysical) {
				_ = s.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
				return nil, nil, fmt.Errorf("parameter %s physical readback mismatch requested %g actual %s; full preimage restored",
					write.ParamID, *write.RequestedPhysical, firstNonEmptyText(row, "value_text"))
			}
		}
		if physicalOK {
			entry["actual_physical"] = physical
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
			if write.Quantized || write.RequestedPhysical == nil || !isEQPhysicalRole(write.Role) {
				continue
			}
			row, ok := index[write.ParamID]
			if !ok {
				return corrected, fmt.Errorf("physical correction readback omitted %s", write.ParamID)
			}
			actual, ok := physicalReadbackForRole(write.Role, row)
			if !ok {
				return corrected, fmt.Errorf("cannot parse %s physical readback %q", write.Role, firstNonEmptyText(row, "value_text"))
			}
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
		result, err := s.kernel.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
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
		if write.Quantized || write.RequestedPhysical == nil || !isEQPhysicalRole(write.Role) {
			continue
		}
		actual, ok := physicalReadbackForRole(write.Role, index[write.ParamID])
		if !ok || math.Abs(actual-*write.RequestedPhysical) > eqPhysicalTolerance(write.Role, *write.RequestedPhysical) {
			return corrected, fmt.Errorf("parameter %s did not reach requested %s after %d bounded corrections: requested=%g actual=%g tolerance=%g",
				write.ParamID, write.Role, maxPhysicalCorrectionIterations, *write.RequestedPhysical, actual,
				eqPhysicalTolerance(write.Role, *write.RequestedPhysical))
		}
	}
	return corrected, nil
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
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters",
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
	sort.SliceStable(preimage, func(i, j int) bool {
		return preimage[i].Role == "used" && preimage[j].Role != "used"
	})
	parameters := make([]map[string]any, 0, len(preimage))
	for _, value := range preimage {
		parameters = append(parameters, map[string]any{"parameter_id": value.ParamID, "normalized_value": value.Normalized})
	}
	result, err := s.kernel.SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
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
		match := regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`).FindString(text)
		if match == "" {
			return 0, false
		}
		value, err := strconv.ParseFloat(match, 64)
		return value, err == nil
	}
	return 0, false
}

func parseEQFrequencyReadback(text string) (float64, bool) {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	match := regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`).FindString(lower)
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0, false
	}
	if strings.Contains(lower, "khz") {
		value *= 1000
	}
	return value, value > 0
}

func eqTypedSectionWritePlan(summary map[string]any, request eqTypedPointRequest) ([]eqWriteStep, map[string]any, error) {
	if supported, ok := summary["set_eq_point_supported"].(bool); ok && !supported {
		return nil, nil, fmt.Errorf("set_eq_point is not supported: %s", firstNonEmptyText(summary, "reason"))
	}
	sections := mapRowsValue(summary["sections"])
	if len(sections) == 0 {
		return nil, nil, fmt.Errorf("EQ summary has no typed sections for requested shape %s", request.Shape)
	}
	candidates := []map[string]any{}
	for _, section := range sections {
		complete, _ := section["complete"].(bool)
		if !complete || !eqSectionSupportsKind(section, request.Shape) {
			continue
		}
		candidates = append(candidates, section)
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("requested shape %s is not provably reachable; available shapes: %v",
			request.Shape, summary["supported_filter_kinds"])
	}
	selected := selectEQSectionByFrequency(candidates, request.FreqHz, firstNonEmptyText(summary, "eq_model"))
	if selected == nil {
		return nil, nil, fmt.Errorf("no complete %s section has a recoverable frequency binding", request.Shape)
	}

	writes := []eqWriteStep{}
	applied := []string{"shape"}
	unsupported := []string{}
	limitations := []string{}
	shapeWrites, err := eqFilterKindWrites(selected, request.Shape)
	if err != nil {
		return nil, nil, err
	}
	writes = append(writes, shapeWrites...)

	frequencyQuantized := false
	if len(mapRowsValue(selected["frequency_bindings"])) > 0 {
		freqWrites, freqErr := eqWritesForRole(selected, "frequency", "freq", request.FreqHz, 20, 20000, "log")
		if freqErr != nil {
			return nil, nil, freqErr
		}
		for _, write := range freqWrites {
			frequencyQuantized = frequencyQuantized || write.Quantized
		}
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
			unsupported = append(unsupported, "q")
			limitations = append(limitations, "requested Q is not exposed by the selected section")
		} else {
			writes = append(writes, qWrites...)
			applied = append(applied, "q")
		}
	}
	if request.SlopeDBPerOct != nil {
		slopeWrites, slopeErr := eqWritesForRole(selected, "slope", "slope", *request.SlopeDBPerOct, 6, 96, "linear")
		if slopeErr != nil {
			unsupported = append(unsupported, "slope_db_per_oct")
			limitations = append(limitations, "requested slope is not exposed by the selected section")
		} else {
			writes = append(writes, slopeWrites...)
			applied = append(applied, "slope_db_per_oct")
		}
	}
	if request.GainDB != nil {
		if request.Shape == "low_cut" || request.Shape == "high_cut" || request.Shape == "notch" {
			unsupported = append(unsupported, "gain_db")
			limitations = append(limitations, "gain is not applicable to the requested filter topology")
		} else {
			gainWrites, gainErr := eqWritesForRole(selected, "gain", "gain", *request.GainDB, -24, 24, "linear")
			if gainErr != nil {
				return nil, nil, fmt.Errorf("requested gain cannot be applied: %w", gainErr)
			}
			writes = append(writes, gainWrites...)
			applied = append(applied, "gain_db")
		}
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
		"partial":             len(unsupported) > 0,
		"applied_fields":      uniqueStrings(applied),
		"unsupported_fields":  uniqueStrings(unsupported),
		"limitations":         uniqueStrings(limitations),
	}
	if fixed, ok := eqBandFloat(selected, "fixed_freq_hz"); ok {
		info["actual_selected_freq_hz"] = fixed
	} else if current, ok := eqSectionCurrentFrequency(selected); ok {
		info["previous_freq_hz"] = current
	}
	return writes, info, nil
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
			return current, true
		}
	}
	return 0, false
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
	case "notch", "bandstop", "bandreject":
		return "notch", nil
	}
	return "", fmt.Errorf("unsupported EQ shape %q; use bell, low_shelf, high_shelf, low_cut, high_cut, or notch", value)
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
		normalized, quantized := eqBindingTarget(target, binding, fallbackMin, fallbackMax, fallbackScale)
		low, high := eqBindingCorrectionBounds(target, binding)
		curve := eqBandCurve(binding, "curve")
		decreasing := len(curve) >= 2 && curve[len(curve)-1][1] < curve[0][1]
		requested := target
		out = append(out, eqWriteStep{ParamID: paramID, Role: writeRole,
			Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized,
			RequestedPhysical: &requested, Quantized: quantized,
			CorrectionLow: low, CorrectionHigh: high, PhysicalDecreasing: decreasing})
	}
	return out, nil
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
