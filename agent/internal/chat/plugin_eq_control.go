package chat

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"vit-daw-agent/internal/harness"
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
	gainDB, err := parseFloat64Arg(args, "gain_db")
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

	writes, selectionInfo, err := eqBandWritePlan(summary, freqHz, gainDB, qTarget)
	if err != nil {
		return fail(err)
	}

	goalID := firstNonEmptyText(requestContext, "goal_id")
	runID := firstNonEmptyText(requestContext, "run_id")
	baseCallID := firstNonEmptyText(requestContext, "tool_call_id", "set_eq_point")

	var lastResp harness.InvokeResponse
	executed := make([]map[string]any, 0, len(writes))
	for i, w := range writes {
		r, invokeErr := s.harness.Invoke(ctx, harness.InvokeRequest{
			Command: map[string]any{
				"cmd":       "set_plugin_param",
				"track_id":  target.TrackID,
				"plugin_id": target.PluginID,
				"param_id":  w.ParamID,
				"value":     w.NormalizedValue,
			},
			Context:    requestContext,
			Source:     pluginGrabberSetEQPointCommand,
			Confirmed:  true,
			GoalID:     goalID,
			RunID:      runID,
			ToolCallID: fmt.Sprintf("%s_%d", baseCallID, i),
		})
		if invokeErr != nil || r.Status == "error" {
			msg := firstNonEmpty(r.Error, errorText(invokeErr), "set_plugin_param failed")
			return fail(fmt.Errorf("write param %s (%s): %s", w.ParamID, w.Role, msg))
		}
		lastResp = r
		executed = append(executed, map[string]any{"param_id": w.ParamID, "role": w.Role, "value": w.NormalizedValue})
	}

	result := map[string]any{
		"status":        "ok",
		"track_id":      target.TrackID,
		"plugin_id":     target.PluginID,
		"freq_hz":       freqHz,
		"gain_db":       gainDB,
		"eq_model":      summary["eq_model"],
		"selected_band": selectionInfo,
		"writes":        executed,
	}
	if qProvided {
		result["q"] = qValue
	}
	if lastResp.AgentActionID != "" {
		result["agent_action_id"] = lastResp.AgentActionID
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
	ParamID         string
	NormalizedValue float64
	Role            string // "freq" | "gain" | "q" | "used" | "shape"
}

func eqBandWritePlan(summary map[string]any, freqHz, gainDB float64, q *float64) ([]eqWriteStep, map[string]any, error) {
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
	best := bands[0]
	bestDist := math.MaxFloat64
	for _, b := range bands {
		cur, ok := eqBandFloat(b, "current_freq_hz")
		if !ok {
			continue
		}
		if d := math.Abs(freqHz - cur); d < bestDist {
			bestDist = d
			best = b
		}
	}
	freqParamID := firstNonEmptyText(best, "freq_param_id")
	gainParamID := firstNonEmptyText(best, "gain_param_id")
	if freqParamID == "" || gainParamID == "" {
		return nil, nil, fmt.Errorf("band %v is missing freq_param_id or gain_param_id", best["band"])
	}
	writes := []eqWriteStep{
		{ParamID: freqParamID, Role: "freq", NormalizedValue: eqNormalizeTarget(freqHz,
			eqBandCurve(best, "freq_curve"), mapValue(best["freq_domain"]), 20, 20000, "log")},
		{ParamID: gainParamID, Role: "gain", NormalizedValue: eqNormalizeTarget(gainDB,
			eqBandCurve(best, "gain_curve"), mapValue(best["gain_domain"]), -24, 24, "linear")},
	}
	writes, qSkipped, err := appendEQQWrite(writes, best, q)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": best["band"], "current_freq_hz": best["current_freq_hz"], "freq_distance_hz": bestDist}
	noteSkippedQ(info, qSkipped)
	return writes, info, nil
}

// noteSkippedQ records that a requested Q could not be applied to the band that
// was selected, so the caller can say so instead of implying it was set.
func noteSkippedQ(info map[string]any, skipped bool) {
	if skipped {
		info["q_skipped"] = true
		info["q_skipped_reason"] = "the selected band exposes no width control (shelf, " +
			"pass filter, or fixed-frequency fader); frequency and gain were still applied"
	}
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
	gainParamID := firstNonEmptyText(best, "gain_param_id")
	if gainParamID == "" {
		return nil, nil, fmt.Errorf("band %v is missing gain_param_id", best["band"])
	}
	writes := []eqWriteStep{
		{ParamID: gainParamID, Role: "gain", NormalizedValue: eqNormalizeTarget(gainDB,
			eqBandCurve(best, "gain_curve"), mapValue(best["gain_domain"]), -24, 24, "linear")},
	}
	writes, qSkipped, err := appendEQQWrite(writes, best, q)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": best["band"], "fixed_freq_hz": best["fixed_freq_hz"], "freq_distance_hz": bestDist}
	noteSkippedQ(info, qSkipped)
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
			gainParamID := firstNonEmptyText(b, "gain_param_id")
			if gainParamID == "" {
				continue
			}
			writes := []eqWriteStep{
				{ParamID: gainParamID, Role: "gain", NormalizedValue: eqNormalizeTarget(gainDB,
					eqBandCurve(b, "gain_curve"), mapValue(b["gain_domain"]), -24, 24, "linear")},
			}
			writes, qSkipped, err := appendEQQWrite(writes, b, q)
			if err != nil {
				return nil, nil, err
			}
			info := map[string]any{"band": b["band"], "operation": "adjust_existing", "current_freq_hz": cur}
			noteSkippedQ(info, qSkipped)
			return writes, info, nil
		}
	}
	// No nearby active band — create from the first available slot.
	if len(available) == 0 {
		return nil, nil, fmt.Errorf("free_floating EQ has no available slots to create a new band")
	}
	slot := available[0]
	usedParamID := firstNonEmptyText(slot, "used_param_id")
	freqParamID := firstNonEmptyText(slot, "freq_param_id")
	gainParamID := firstNonEmptyText(slot, "gain_param_id")
	if usedParamID == "" || freqParamID == "" || gainParamID == "" {
		return nil, nil, fmt.Errorf("available slot %v is missing used_param_id, freq_param_id, or gain_param_id", slot["band"])
	}
	writes := []eqWriteStep{
		{ParamID: usedParamID, NormalizedValue: 1.0, Role: "used"},
		{ParamID: freqParamID, Role: "freq", NormalizedValue: eqNormalizeTarget(freqHz,
			eqBandCurve(slot, "freq_curve"), mapValue(slot["freq_domain"]), 20, 20000, "log")},
		{ParamID: gainParamID, Role: "gain", NormalizedValue: eqNormalizeTarget(gainDB,
			eqBandCurve(slot, "gain_curve"), mapValue(slot["gain_domain"]), -24, 24, "linear")},
	}
	writes, qSkipped, err := appendEQQWrite(writes, slot, q)
	if err != nil {
		return nil, nil, err
	}
	info := map[string]any{"band": slot["band"], "operation": "create_band"}
	noteSkippedQ(info, qSkipped)
	return writes, info, nil
}

// appendEQQWrite adds the Q write when the selected band has one.
//
// Plenty of legitimate bands do not: shelves and high/low-pass sections
// generally expose no width control, and a graphic-EQ fader has nothing but
// gain. Aborting the whole operation in that case threw away a frequency and
// gain move the user did ask for. The skip is reported back instead of being
// swallowed, so a requested Q that could not be applied stays visible.
func appendEQQWrite(writes []eqWriteStep, band map[string]any, q *float64) ([]eqWriteStep, bool, error) {
	if q == nil {
		return writes, false, nil
	}
	qParamID := firstNonEmptyText(band, "q_param_id")
	if qParamID == "" {
		return writes, true, nil
	}
	writes = append(writes, eqWriteStep{
		ParamID: qParamID,
		Role:    "q",
		NormalizedValue: eqNormalizeTarget(*q,
			eqBandCurve(band, "q_curve"), mapValue(band["q_domain"]), 0.1, 6, "log"),
	})
	return writes, false, nil
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
