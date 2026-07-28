package chat

import (
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/eqcontrolgraph"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func eqControlGraphRuntime(section map[string]any) (*plugingrabber.EQControlGraphRuntime, bool) {
	runtime, ok := section["control_graph_runtime"].(*plugingrabber.EQControlGraphRuntime)
	return runtime, ok && runtime != nil
}

func eqControlGraphInputs(request eqTypedPointRequest) map[string]interface{} {
	inputs := map[string]interface{}{}
	if request.FreqHz > 0 {
		inputs["frequency_hz"] = request.FreqHz
	}
	if request.GainDB != nil {
		inputs["gain_db"] = *request.GainDB
	}
	if request.Q != nil {
		inputs["q"] = *request.Q
	}
	if request.SlopeDBPerOct != nil {
		inputs["slope_db_per_oct"] = *request.SlopeDBPerOct
	}
	return inputs
}

func eqControlGraphEditInputs(edit eqEditRequest) map[string]interface{} {
	inputs := map[string]interface{}{}
	if edit.FrequencyHz != nil {
		inputs["frequency_hz"] = *edit.FrequencyHz
	}
	if edit.GainDB != nil {
		inputs["gain_db"] = *edit.GainDB
	}
	if edit.Q != nil {
		inputs["q"] = *edit.Q
	}
	if edit.SlopeDBPerOct != nil {
		inputs["slope_db_per_oct"] = *edit.SlopeDBPerOct
	}
	return inputs
}

func eqControlGraphUpsertWritePlan(summary map[string]any, request eqTypedPointRequest) ([]eqWriteStep, map[string]any, error) {
	if (request.Shape == "bell" || request.Shape == "low_shelf" || request.Shape == "high_shelf") && request.GainDB == nil {
		return nil, nil, fmt.Errorf("required_field_missing: gain_db is required for %s", request.Shape)
	}
	if (request.Shape == "low_cut" || request.Shape == "high_cut") && request.GainDB != nil {
		return nil, nil, fmt.Errorf("invalid_field_for_shape: gain_db is not applicable to %s", request.Shape)
	}
	inputs := eqControlGraphInputs(request)
	candidates := []map[string]any{}
	failures := []string{}
	for _, section := range mapRowsValue(summary["sections"]) {
		complete, _ := section["complete"].(bool)
		if !complete || !eqSectionActionSupported(section, request.Shape, "upsert") ||
			!eqSectionFrequencyTargetInRange(section, request.FreqHz) {
			continue
		}
		runtime, ok := eqControlGraphRuntime(section)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: missing validated runtime", firstNonEmptyText(section, "section")))
			continue
		}
		if _, err := runtime.Compile(request.Shape, "upsert", inputs); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", firstNonEmptyText(section, "section"), err))
			continue
		}
		candidates = append(candidates, section)
	}
	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("explicit_field_unavailable: no %s section at %g Hz accepts the complete request (%s)",
			request.Shape, request.FreqHz, strings.Join(failures, "; "))
	}
	selected := selectEQSectionByFrequency(candidates, request.FreqHz, firstNonEmptyText(summary, "eq_model"))
	if selected == nil {
		return nil, nil, fmt.Errorf("no complete %s section has a recoverable frequency selection", request.Shape)
	}
	runtime, _ := eqControlGraphRuntime(selected)
	result, err := runtime.Compile(request.Shape, "upsert", inputs)
	if err != nil {
		return nil, nil, err
	}
	writes, err := eqControlGraphTargetsToWrites(result)
	if err != nil {
		return nil, nil, err
	}
	frequencyQuantized := false
	if fixed, ok := eqBandFloat(selected, "fixed_freq_hz"); ok {
		frequencyQuantized = math.Abs(fixed-request.FreqHz) > 1e-9
	}
	applied := append([]string{"shape"}, result.AppliedInputs...)
	return writes, map[string]any{
		"section": firstNonEmptyText(selected, "section"), "shape": request.Shape,
		"addressing": selected["addressing"], "channel_bindings": selected["channel_bindings"],
		"activation": selected["activation"], "frequency_quantized": frequencyQuantized,
		"partial": false, "applied_fields": uniqueStrings(applied), "unsupported_fields": []string{},
		"limitations": []string{}, "mapping_source": "vps_control_graph",
	}, nil
}

func eqControlGraphCompileWrites(section map[string]any, shape, action string, inputs map[string]interface{}) ([]eqWriteStep, error) {
	runtime, ok := eqControlGraphRuntime(section)
	if !ok {
		return nil, fmt.Errorf("section %s has no validated control graph runtime", firstNonEmptyText(section, "section"))
	}
	result, err := runtime.Compile(shape, action, inputs)
	if err != nil {
		return nil, err
	}
	return eqControlGraphTargetsToWrites(result)
}

func eqControlGraphTargetsToWrites(result eqcontrolgraph.CompileResult) ([]eqWriteStep, error) {
	writes := make([]eqWriteStep, 0, len(result.Targets))
	for _, target := range result.Targets {
		role := controlGraphExecutorRole(target.Role)
		if target.Phase == "activation" {
			switch result.Action {
			case "disable":
				role = "disabled"
			case "remove":
				role = "removed"
			default:
				role = "used"
			}
		}
		write := eqWriteStep{ParamID: target.Binding.ParameterID, Role: role, Channel: target.Binding.Channel,
			ActivationLast: target.Phase == "activation"}
		if target.Number != nil {
			domain := target.Binding.Domain
			if domain == nil {
				return nil, fmt.Errorf("numeric binding %s has no domain", target.BindingKey)
			}
			binding := map[string]any{
				"param_id": target.Binding.ParameterID, "channel": target.Binding.Channel,
				"domain": map[string]any{"unit": domain.Unit, "min": domain.Minimum, "max": domain.Maximum,
					"scale": controlGraphExecutorScale(domain.ValueLaw)},
				"curve": append([][2]float64(nil), domain.Curve...),
			}
			value := *target.Number
			write.NormalizedValue, write.Quantized = eqBindingTarget(value, binding, domain.Minimum, domain.Maximum,
				controlGraphExecutorScale(domain.ValueLaw))
			write.CorrectionLow, write.CorrectionHigh = eqBindingCorrectionBounds(value, binding)
			write.RequestedPhysical = &value
			curve := domain.Curve
			write.PhysicalDecreasing = len(curve) >= 2 && curve[len(curve)-1][1] < curve[0][1]
		} else if target.EnumValue != nil {
			write.NormalizedValue = target.EnumValue.Normalized
			write.Quantized = true
			write.RequestedLabel = target.EnumKey
			write.ExpectedLabel = target.EnumValue.Label
		} else {
			return nil, fmt.Errorf("binding %s produced no target", target.BindingKey)
		}
		writes = append(writes, write)
	}
	return writes, nil
}

func controlGraphExecutorRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "frequency", "frequency_hz":
		return "freq"
	case "gain_db":
		return "gain"
	case "slope_db_per_oct":
		return "slope"
	default:
		return strings.TrimSpace(role)
	}
}

func controlGraphExecutorScale(valueLaw string) string {
	if strings.EqualFold(strings.TrimSpace(valueLaw), "logarithmic") {
		return "log"
	}
	return "linear"
}
