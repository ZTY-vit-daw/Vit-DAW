package executionports

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

// WriteModeNormalizedBatchV1 selects the real-plugin EQ write pipeline on a
// static_eq action: parameter ids come from the machine-local experiment
// whitelist (one band, ch1+ch2 for dual-channel carriers; one shared
// parameter for single-channel ones like the FAM1-S1 de_esser), the port
// instantiates by plugin_path with an empty identifier, writes one
// kernel-normalized batch for the resolved channels under one idempotency
// key (single revision advance), and verifies through the normalized readback
// plus the value_text physical parse. Legacy actions without this arg keep
// the historical single-parameter raw-value pipeline.
const WriteModeNormalizedBatchV1 = "normalized_batch_v1"

// EqNormalizedTolerance mirrors the mature chat EQ transaction tolerance: the
// authoritative post-write check is numeric normalized equality at 1e-4. The
// value_text physical parse stays informational because real plugin displays
// quantize their readings. Exported so the receipt-driven verifier consumes
// the same constant instead of duplicating the number.
const EqNormalizedTolerance = 1e-4

type eqGainChannel struct {
	ParamID             string  `json:"parameter_id"`
	RequestedNormalized float64 `json:"requested"`
	ActualNormalized    float64 `json:"actual"`
}

// eqParameterSurface fetches the full plugin parameter surface through the
// governed legacy get_plugin_parameters command and parses it with the shared
// grabber digest builder, so display-domain discovery here is exactly what
// the deterministic chat EQ path sees.
func (p *StaticEQVSPPort) eqParameterSurface(ctx context.Context, trackID, pluginID string) (map[string]plugingrabber.ParameterInfo, error) {
	result, err := p.Client.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd":                "get_plugin_parameters",
		"track_id":           trackID,
		"plugin_id":          pluginID,
		"include_parameters": true,
	}, "read:surface:"+trackID+":"+pluginID, "tx_read:surface:"+pluginID)
	if err != nil {
		return nil, fmt.Errorf("plugin parameter surface read failed: %w", err)
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		return nil, fmt.Errorf("plugin parameter surface read failed: %s", strings.TrimSpace(fmt.Sprint(reply["message"])))
	}
	digest := plugingrabber.BuildParameterDigest(reply)
	surface := make(map[string]plugingrabber.ParameterInfo, len(digest.Parameters))
	for _, param := range digest.Parameters {
		surface[param.ID] = param
	}
	return surface, nil
}

// dumpParameterCurveContext appends the live parameter surface context for a
// planned write to $VIT_PARAM_CURVE_DUMP (JSONL). Diagnostic instrumentation
// only: absent the env var it is a no-op and never affects the write path.
func dumpParameterCurveContext(surface map[string]plugingrabber.ParameterInfo, targetDB float64, paramIDs ...string) {
	path := strings.TrimSpace(os.Getenv("VIT_PARAM_CURVE_DUMP"))
	if path == "" {
		return
	}
	params := make(map[string]any, len(paramIDs))
	for _, paramID := range paramIDs {
		info, ok := surface[paramID]
		if !ok {
			params[paramID] = "missing_from_surface"
			continue
		}
		params[paramID] = map[string]any{
			"name":                    info.Name,
			"unit":                    info.Unit,
			"normalized_value":        info.NormalizedValue,
			"value_text":              info.ValueText,
			"min":                     info.Min,
			"max":                     info.Max,
			"display_domain_candidate": info.DisplayDomainCandidate,
			"display_probe":           info.DisplayProbe,
		}
	}
	entry := map[string]any{"captured_at": time.Now().UTC().Format(time.RFC3339Nano), "target_db": targetDB, "params": params}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

// eqPlanGainChannels resolves the band's gain params on the live surface and
// maps the admitted dB target onto kernel-normalized values using the display
// domain each parameter reports about itself. Machine-specific ranges are
// never hardcoded: linear dB candidates use their probed min/max; anything
// else is inverted through the measured value_to_string curve.
func eqPlanGainChannels(surface map[string]plugingrabber.ParameterInfo, targetDB float64, paramIDs ...string) ([]eqGainChannel, string, error) {
	dumpParameterCurveContext(surface, targetDB, paramIDs...)
	channels := make([]eqGainChannel, 0, len(paramIDs))
	descriptions := make([]string, 0, len(paramIDs))
	for _, paramID := range paramIDs {
		paramID = strings.TrimSpace(paramID)
		info, ok := surface[paramID]
		if !ok || paramID == "" {
			return nil, "", fmt.Errorf("band gain parameter %q was not present in the plugin parameter surface", paramID)
		}
		description := fmt.Sprintf("parameter %q", paramID)
		if info.DisplayDomainCandidate != nil && strings.TrimSpace(info.DisplayDomainCandidate.Text) != "" {
			description = fmt.Sprintf("parameter %q display domain %q", paramID, info.DisplayDomainCandidate.Text)
		}
		normalized, err := eqGainToNormalized(info, targetDB)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", description, err)
		}
		channels = append(channels, eqGainChannel{ParamID: paramID, RequestedNormalized: normalized})
		descriptions = append(descriptions, description)
	}
	return channels, strings.Join(descriptions, "; "), nil
}

// eqGainToNormalized converts one admitted dB target into the kernel's
// normalized 0..1 domain for a single parameter.
func eqGainToNormalized(info plugingrabber.ParameterInfo, targetDB float64) (float64, error) {
	if candidate := info.DisplayDomainCandidate; candidate != nil && isEQDecibelUnit(candidate.Unit) &&
		strings.EqualFold(strings.TrimSpace(candidate.Scale), "linear") && candidate.Min != nil && candidate.Max != nil {
		min, max := *candidate.Min, *candidate.Max
		if max > min {
			return clampUnit((targetDB - min) / (max - min)), nil
		}
	}
	if curve, ok := eqDisplayCurve(info); ok {
		if normalized, ok := eqNormalizedFromCurve(targetDB, curve); ok {
			return clampUnit(normalized), nil
		}
		return 0, fmt.Errorf("target %.4g dB is outside the measured display curve of %d samples", targetDB, len(curve))
	}
	return 0, fmt.Errorf("no usable dB display domain was discovered at runtime")
}

// eqDisplayCurve extracts {normalized, physical} samples from the kernel's
// five-point value_to_string probe. Only monotone-capable curves with at
// least three parsed points are usable.
func eqDisplayCurve(info plugingrabber.ParameterInfo) ([][2]float64, bool) {
	if info.DisplayProbe == nil || len(info.DisplayProbe.Samples) < 3 {
		return nil, false
	}
	curve := make([][2]float64, 0, len(info.DisplayProbe.Samples))
	for _, sample := range info.DisplayProbe.Samples {
		physical, ok := plugingrabber.ParseEQLocalizedNumber(sample.Text)
		if !ok {
			continue
		}
		curve = append(curve, [2]float64{sample.NormalizedValue, physical})
	}
	return curve, len(curve) >= 3
}

func isEQDecibelUnit(unit string) bool {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "db", "dbfs":
		return true
	}
	return false
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
// visibly poor fit rather than a confidently wrong write. Ported verbatim from
// the chat EQ control machine (plugin_eq_control.go) so both execution paths
// share identical inversion semantics without a package cycle.
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

// eqTypedCommandFailure inspects a typed VSP result through its public maps so
// batch failures (including partial_failure envelopes) surface their kernel
// message instead of being swallowed as generic transport errors.
func eqTypedCommandFailure(result *kernel.VSPCommandResult) string {
	if result == nil {
		return "empty VSP response"
	}
	failed := ""
	for _, candidate := range []map[string]any{result.Payload, result.Response, result.LegacyReply} {
		if candidate == nil {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(candidate["status"])))
		if status == "error" || status == "failed" || status == "partial_failure" || status == "rejected" {
			if failed == "" {
				failed = status
			}
			if message := eqFailureMessage(candidate); message != "" {
				return message
			}
		}
		payload, _ := candidate["payload"].(map[string]any)
		if len(payload) > 0 {
			status := strings.ToLower(strings.TrimSpace(fmt.Sprint(payload["status"])))
			if status == "error" || status == "failed" || status == "partial_failure" {
				if failed == "" {
					failed = status
				}
				if message := eqFailureMessage(payload); message != "" {
					return message
				}
			}
		}
	}
	return failed
}

// eqFailureMessage digs the human-readable failure text out of a kernel
// envelope: the VSP error envelopes keep it under error.message/error.code
// and the batch aggregates under ack.message, not at the top level.
func eqFailureMessage(candidate map[string]any) string {
	message := strings.TrimSpace(fmt.Sprint(candidate["message"]))
	if message != "" && message != "<nil>" {
		return message
	}
	for _, nestedKey := range []string{"ack", "error"} {
		nested, _ := candidate[nestedKey].(map[string]any)
		if nested == nil {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(nested["message"])); text != "" && text != "<nil>" {
			return text
		}
		if code := strings.TrimSpace(fmt.Sprint(nested["code"])); code != "" && code != "<nil>" {
			return code
		}
	}
	return ""
}
