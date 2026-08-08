package plugingrabber

import "testing"

func transientTestParam(id, name, current string, labels ...string) ParameterInfo {
	param := ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: .5, ValueText: current}
	if len(labels) > 0 {
		param.DisplayProbe = &ParameterDisplayProbe{}
		for index, label := range labels {
			normalized := float64(index) / float64(len(labels)-1)
			param.DisplayProbe.Samples = append(param.DisplayProbe.Samples, ParameterDisplayProbeSample{NormalizedValue: normalized, Text: label})
			param.DisplayProbe.DiscreteLabels = append(param.DisplayProbe.DiscreteLabels, ParameterDisplayProbeLabel{Index: index, Value: normalized, Label: label})
		}
		param.DisplayDomainCandidate = DisplayDomainCandidateForParameter(param)
	}
	return param
}

func TestDetectTransientShaperModelSignedAttackSustainAndLimiterIsolation(t *testing.T) {
	digest := ParameterDigest{PluginName: "identity ignored", Parameters: []ParameterInfo{
		transientTestParam("attack", "Attack", "0 dB", "-15 dB", "-7.5 dB", "0 dB", "7.5 dB", "15 dB"),
		transientTestParam("sustain", "Sustain", "0 dB", "-24 dB", "-12 dB", "0 dB", "12 dB", "24 dB"),
		transientTestParam("output", "Output", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
		transientTestParam("mix", "Mix", "100 %", "0 %", "25 %", "50 %", "75 %", "100 %"),
		transientTestParam("limit", "Limit", "Off", "Off", "On"),
	}}
	model, boundary := DetectTransientShaperModelWithBoundary(digest)
	if model == nil || boundary != "" || model.Classification != "paired_envelope_transient_shaper" {
		t.Fatalf("model=%+v boundary=%q", model, boundary)
	}
	if len(model.AuxiliaryStages) != 1 || model.AuxiliaryStages[0].Kind != "limiter" {
		t.Fatalf("auxiliary stages=%+v", model.AuxiliaryStages)
	}
	if transientShaperHasRole(model.Stage.Mode, "limiter") || transientShaperHasRole(model.Stage.Output, "limiter") {
		t.Fatalf("limiter leaked into controllable stage: %+v", model.Stage)
	}
}

// Disclosed SPL Transient Designer Plus surface: the two signed dB axes are
// the identity evidence; output, mix, SC filter, and the optional Limit stage
// are separate controls/auxiliary evidence.
func TestDetectTransientShaperModelSPLTransientDesignerPlus(t *testing.T) {
	digest := ParameterDigest{Parameters: []ParameterInfo{
		transientTestParam("attack", "Attack", "0.0 dB", "-15.0 dB", "-7.5 dB", "0.0 dB", "7.5 dB", "15.0 dB"),
		transientTestParam("sustain", "Sustain", "0.0 dB", "-24.0 dB", "-12.0 dB", "0.0 dB", "12.0 dB", "24.0 dB"),
		transientTestParam("output", "Output", "0.0 dB", "-12.0 dB", "-6.0 dB", "0.0 dB", "6.0 dB", "12.0 dB"),
		transientTestParam("mix", "Mix", "100 %", "0 %", "25 %", "50 %", "75 %", "100 %"),
		transientTestParam("sc-on", "SC Filter On", "Off", "Off", "On"),
		transientTestParam("sc-freq", "SC Freq", "2.5 kHz", "80 Hz", "500 Hz", "2500 Hz", "8000 Hz", "20000 Hz"),
		transientTestParam("limit", "Limit", "Off", "Off", "On"),
	}}
	model, boundary := DetectTransientShaperModelWithBoundary(digest)
	if model == nil || boundary != "" || model.Classification != "paired_envelope_transient_shaper" {
		t.Fatalf("SPL model=%+v boundary=%q", model, boundary)
	}
	if !transientShaperHasRole(model.Stage.EnvelopeAction, "attack_amount") || !transientShaperHasRole(model.Stage.EnvelopeAction, "sustain_amount") {
		t.Fatalf("SPL signed axes missing: %+v", model.Stage.EnvelopeAction)
	}
	if !transientShaperHasRole(model.Stage.Detector, "focus_frequency") {
		t.Fatalf("SPL SC frequency missing: %+v", model.Stage.Detector)
	}
	if !transientShaperHasRole(model.Stage.Detector, "focus_enable") {
		t.Fatalf("SPL SC enable missing: %+v", model.Stage.Detector)
	}
	if len(model.AuxiliaryStages) != 1 || model.AuxiliaryStages[0].Kind != "limiter" {
		t.Fatalf("SPL auxiliary stages=%+v", model.AuxiliaryStages)
	}
}

func TestDetectTransientShaperModelRejectsAttackTiming(t *testing.T) {
	model, boundary := DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		transientTestParam("threshold", "Threshold", "-12 dB", "-36 dB", "-24 dB", "-12 dB", "0 dB"),
		transientTestParam("ratio", "Ratio", "4:1", "1:1", "2:1", "4:1", "8:1"),
		transientTestParam("attack", "Attack", "10 ms", "0.1 ms", "1 ms", "10 ms", "100 ms"),
		transientTestParam("release", "Release", "100 ms", "10 ms", "50 ms", "100 ms", "500 ms"),
	}})
	if model != nil || boundary != "unsupported_broadband_compressor" {
		t.Fatalf("model=%+v boundary=%q", model, boundary)
	}
}

func TestDetectTransientShaperModelModeRemap(t *testing.T) {
	params := []ParameterInfo{
		transientTestParam("mode", "Mode", "Full Range", "Full Range", "Dual Band", "Shelf EQ"),
		transientTestParam("high", "Attack/Gain H", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
		transientTestParam("low", "Sustain/Gain L", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
	}
	model, boundary := DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || boundary != "" || model.SemanticMode != "full_range" ||
		!transientShaperHasRole(model.Stage.EnvelopeAction, "attack_amount") || !transientShaperHasRole(model.Stage.EnvelopeAction, "sustain_amount") {
		t.Fatalf("model=%+v boundary=%q", model, boundary)
	}
	params[0].ValueText = "Dual Band"
	if model, boundary = DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: params}); model != nil || boundary != "unsupported_dual_band_mode" {
		t.Fatalf("dual-band model=%+v boundary=%q", model, boundary)
	}
	params[0].ValueText = "Shelf EQ"
	if model, boundary = DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: params}); model != nil || boundary != "unsupported_eq_mode" {
		t.Fatalf("shelf model=%+v boundary=%q", model, boundary)
	}
}

// Disclosed elysia nvelope surface: Attack/Gain H and Sustain/Gain L are
// transient roles only in Full Range; the same labels are frequency/shelf
// roles in the other reachable modes and therefore fail closed in v1.
func TestDetectTransientShaperModelElysiaNvelope(t *testing.T) {
	params := []ParameterInfo{
		transientTestParam("mode", "Mode", "Full Range", "Full Range", "Dual Band", "Shelf EQ"),
		transientTestParam("high", "Attack/Gain H", "0.0 dB", "-12.0 dB", "-6.0 dB", "0.0 dB", "6.0 dB", "12.0 dB"),
		transientTestParam("low", "Sustain/Gain L", "0.0 dB", "-12.0 dB", "-6.0 dB", "0.0 dB", "6.0 dB", "12.0 dB"),
	}
	model, boundary := DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || boundary != "" || model.SemanticMode != "full_range" || model.Classification != "paired_envelope_transient_shaper" {
		t.Fatalf("nvelope model=%+v boundary=%q", model, boundary)
	}
	params[0].ValueText = "Dual Band"
	if model, boundary = DetectTransientShaperModelWithBoundary(ParameterDigest{Parameters: params}); model != nil || boundary != "unsupported_dual_band_mode" {
		t.Fatalf("nvelope dual-band model=%+v boundary=%q", model, boundary)
	}
}

func TestDetectTransientShaperModelWavesStructuralVariants(t *testing.T) {
	smack := ParameterDigest{Parameters: []ParameterInfo{
		transientTestParam("attack", "Attack", "0", "-100", "-50", "0", "50", "100"),
		transientTestParam("attack-sense", "AttackSensitivity", "60", "0", "25", "50", "75", "100"),
		transientTestParam("attack-duration", "AttackDuration", "8 ms", ".5 ms", "2 ms", "8 ms", "50 ms", "500 ms"),
		transientTestParam("attack-shape", "AttackShape", "Nail", "Needle", "Nail", "Blunt"),
		transientTestParam("sustain", "Sustain", "0", "-100", "-50", "0", "50", "100"),
		transientTestParam("sustain-sense", "SustainSensitivity", "80", "0", "25", "50", "75", "100"),
		transientTestParam("sustain-duration", "SustainDuration", "200 ms", "30 ms", "100 ms", "200 ms", "500 ms", "1000 ms"),
		transientTestParam("sustain-shape", "SustainShape", "Linear", "Linear", "NonLinear", "Blunt"),
		transientTestParam("guard", "Guard", "Off", "Off", "Clip", "Limit"),
	}}
	model, boundary := DetectTransientShaperModelWithBoundary(smack)
	if model == nil || boundary != "" || model.Classification != "paired_envelope_transient_shaper" {
		t.Fatalf("Smack structural model=%+v boundary=%q", model, boundary)
	}
	if len(model.AuxiliaryStages) != 1 || model.AuxiliaryStages[0].Kind != "limiter_or_clipper" {
		t.Fatalf("Smack auxiliary=%+v", model.AuxiliaryStages)
	}
	for role, bindings := range map[string][]CompressorBinding{
		"attack_sensitivity":  model.Stage.Detector,
		"sustain_sensitivity": model.Stage.Detector,
		"attack_duration":     model.Stage.Timing,
		"sustain_duration":    model.Stage.Timing,
		"attack_shape":        model.Stage.Shape,
		"sustain_shape":       model.Stage.Shape,
	} {
		if !transientShaperHasRole(bindings, role) {
			t.Fatalf("Smack role %s missing: %+v", role, model.Stage)
		}
	}

	transx := ParameterDigest{Parameters: []ParameterInfo{
		transientTestParam("range", "Range", "6 dB", "-24 dB", "-12 dB", "0 dB", "6 dB", "18 dB"),
		transientTestParam("sense", "Sense", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
		transientTestParam("duration", "Duration", "5 ms", ".01 ms", "1 ms", "5 ms", "50 ms", "500 ms"),
		transientTestParam("release", "Release", "10 ms", ".5 ms", "2 ms", "10 ms", "100 ms", "500 ms"),
	}}
	model, boundary = DetectTransientShaperModelWithBoundary(transx)
	if model == nil || boundary != "" || model.Classification != "single_envelope_range_transient_shaper" {
		t.Fatalf("TransX structural model=%+v boundary=%q", model, boundary)
	}
}

func TestTransientShaperGenerationIgnoresIdentityAndAmounts(t *testing.T) {
	one := ParameterDigest{PluginName: "one", Parameters: []ParameterInfo{
		transientTestParam("attack", "Attack", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
		transientTestParam("sustain", "Sustain", "0 dB", "-10 dB", "-5 dB", "0 dB", "5 dB", "10 dB"),
	}}
	two := one
	two.PluginName = "two"
	two.Parameters = append([]ParameterInfo(nil), one.Parameters...)
	two.Parameters[0].ValueText, two.Parameters[0].NormalizedValue = "5 dB", .75
	a, b := DetectTransientShaperModel(one), DetectTransientShaperModel(two)
	if a == nil || b == nil || a.Generation != b.Generation {
		t.Fatalf("generations differ: %+v %+v", a, b)
	}
}
