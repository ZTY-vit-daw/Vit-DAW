package plugingrabber

import (
	"strings"
	"testing"
)

func gateExpanderParam(id, name, valueText string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: .5, ValueText: valueText}
}

func gateExpanderCurveParam(id, name string, labels ...string) ParameterInfo {
	param := gateExpanderParam(id, name, labels[len(labels)/2])
	for index, label := range labels {
		param.DisplayProbe = ensureGateExpanderProbe(param.DisplayProbe)
		param.DisplayProbe.Samples = append(param.DisplayProbe.Samples, ParameterDisplayProbeSample{
			NormalizedValue: float64(index) / float64(len(labels)-1), Text: label,
		})
	}
	param.DisplayDomainCandidate = DisplayDomainCandidateForParameter(param)
	return param
}

func gateExpanderEnumParam(id, name string, labels ...string) ParameterInfo {
	param := gateExpanderParam(id, name, labels[0])
	param.IsDiscrete, param.NumSteps = true, len(labels)
	param.DisplayProbe = ensureGateExpanderProbe(param.DisplayProbe)
	for index, label := range labels {
		normalized := float64(index) / float64(len(labels)-1)
		param.DisplayProbe.DiscreteLabels = append(param.DisplayProbe.DiscreteLabels, ParameterDisplayProbeLabel{Index: index, Value: normalized, Label: label})
		param.DisplayProbe.AllLabels = append(param.DisplayProbe.AllLabels, label)
	}
	param.DisplayDomainCandidate = DisplayDomainCandidateForParameter(param)
	return param
}

func ensureGateExpanderProbe(probe *ParameterDisplayProbe) *ParameterDisplayProbe {
	if probe == nil {
		return &ParameterDisplayProbe{Mode: "read_only_value_to_string"}
	}
	return probe
}

// This is the disclosed Unfiltered Audio G8 historical regression surface. It
// remains a parameter-role fixture for fail-closed coverage; the live C1/PSE
// surfaces below are the primary Gate/Expander training targets.
func TestDetectGateExpanderModelG8RegressionSurfaceAndFailClosedCapabilities(t *testing.T) {
	params := []ParameterInfo{
		gateExpanderCurveParam("analysis", "Analysis Gain", "-26.8 dB", "-10 dB", "0 dB", "5 dB", "9.3 dB"),
		gateExpanderCurveParam("threshold", "Threshold", "-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"),
		gateExpanderCurveParam("attack", "Attack", "1 ms", "10 ms", "50 ms", "200 ms", "500 ms"),
		gateExpanderCurveParam("hold", "Hold", "0 ms", "10 ms", "100 ms", "250 ms", "500 ms"),
		gateExpanderCurveParam("release", "Release", "1 ms", "10 ms", "100 ms", "500 ms", "2000 ms"),
		gateExpanderCurveParam("range", "Reduction", "-20.8 dB", "-15 dB", "-10 dB", "-5 dB", "0 dB"),
		gateExpanderCurveParam("hysteresis", "Hysteresis", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("lookahead", "Lookahead", "0 ms", "10 ms", "25 ms", "50 ms", "100 ms"),
		gateExpanderCurveParam("cycle", "Cycle Delay", "0 ms", "100 ms", "250 ms", "500 ms", "1000 ms"),
		gateExpanderCurveParam("output", "Output Gain", "-26.8 dB", "-10 dB", "0 dB", "5 dB", "9.3 dB"),
		gateExpanderCurveParam("mix", "Dry/Wet", "0 %", "25 %", "50 %", "75 %", "100 %"),
		gateExpanderParam("midi", "MIDI In Note", "C3"),
		gateExpanderParam("ch1sc", "Channel 1 Sidechain", "Input 1"),
		gateExpanderCurveParam("ch1lp", "Channel 1 LP", "20 Hz", "100 Hz", "1000 Hz", "5000 Hz", "8460 Hz"),
		gateExpanderCurveParam("ch1hp", "Channel 1 HP", "332 Hz", "1000 Hz", "5000 Hz", "10000 Hz", "20000 Hz"),
		gateExpanderParam("link12", "Link 1 + 2", "On"),
	}
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{PluginName: "identity ignored", Parameters: params})
	if model == nil || code != "" || model.Classification != "hard_gate" || model.Stage.Direction != "closed_attenuation" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !gateExpanderBindingsHaveRole(model.Stage.OperatingPoint, "threshold") ||
		!gateExpanderBindingsHaveRole(model.Stage.OperatingPoint, "hysteresis") ||
		!gateExpanderBindingsHaveRole(model.Stage.GainAction, "range") ||
		!gateExpanderBindingsHaveRole(model.Stage.Timing, "hold") || !gateExpanderBindingsHaveRole(model.Stage.Timing, "release") {
		t.Fatalf("stage=%+v", model.Stage)
	}
	if len(model.UnsupportedCapabilities) != 2 {
		t.Fatalf("unsupported=%+v", model.UnsupportedCapabilities)
	}
	for _, section := range [][]GateExpanderBinding{model.Stage.Detector, model.Stage.OperatingPoint, model.Stage.GainAction, model.Stage.Timing, model.Stage.Mode, model.Stage.Output} {
		for _, binding := range section {
			if binding.ParamID == "midi" || binding.ParamID == "ch1sc" || binding.ParamID == "ch1lp" || binding.ParamID == "ch1hp" || binding.ParamID == "link12" {
				t.Fatalf("unsupported capability leaked into typed binding: %+v", binding)
			}
		}
	}
}

func TestDetectGateExpanderModelIncludesUnindexedDetectorFilters(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderParam("t", "Gate Threshold", "-30 dB"), gateExpanderParam("range", "Gate Range", "-60 dB"),
		gateExpanderParam("a", "Attack", "2 ms"), gateExpanderParam("r", "Release", "200 ms"),
		gateExpanderCurveParam("hp", "SC HPF", "20 Hz", "50 Hz", "100 Hz", "500 Hz", "1000 Hz"),
		gateExpanderCurveParam("lp", "SC LPF", "1000 Hz", "3000 Hz", "5000 Hz", "10000 Hz", "20000 Hz"),
	}})
	if model == nil || code != "" || !gateExpanderBindingsHaveRole(model.Stage.Detector, "sidechain_highpass") ||
		!gateExpanderBindingsHaveRole(model.Stage.Detector, "sidechain_lowpass") {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderModelC1GateTrainingSurface(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{PluginName: "C1 gate Stereo", Parameters: []ParameterInfo{
		gateExpanderEnumParam("0", "Gate/Expander", "Gate", "Expander"),
		gateExpanderCurveParam("1", "Hold", "0 ms", "20 ms", "100 ms", "250 ms", "500 ms"),
		gateExpanderCurveParam("2", "Attack", "0.01 ms", "1 ms", "10 ms", "100 ms", "500 ms"),
		gateExpanderCurveParam("3", "Release", "1 ms", "10 ms", "100 ms", "500 ms", "2000 ms"),
		gateExpanderCurveParam("4", "Floor", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("5", "Gate Open", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("6", "Gate Close", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("7", "Output Gain", "-24 dB", "-12 dB", "0 dB", "6 dB", "12 dB"),
	}})
	if model == nil || code != "" || model.Classification != "hard_gate" {
		t.Fatalf("C1 gate training surface model=%+v code=%q", model, code)
	}
	if !gateExpanderBindingsHaveRole(model.Stage.OperatingPoint, "threshold") ||
		!gateExpanderBindingsHaveRole(model.Stage.OperatingPoint, "hysteresis") ||
		!gateExpanderBindingsHaveRole(model.Stage.GainAction, "range") {
		t.Fatalf("C1 gate stage=%+v", model.Stage)
	}
}

func TestDetectGateExpanderModelC1CompGateSplitsCompressorSurface(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{PluginName: "C1 comp-gate Stereo", Parameters: []ParameterInfo{
		gateExpanderCurveParam("1", "Gate Hold", "0 ms", "20 ms", "100 ms", "250 ms", "500 ms"),
		gateExpanderCurveParam("2", "LookAhead", "0 ms", "1 ms", "5 ms", "10 ms", "20 ms"),
		gateExpanderCurveParam("3", "Gate Attack", "0.01 ms", "1 ms", "10 ms", "100 ms", "500 ms"),
		gateExpanderCurveParam("4", "Gate Release", "1 ms", "10 ms", "100 ms", "500 ms", "2000 ms"),
		gateExpanderCurveParam("5", "Floor", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("6", "Gate open", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderCurveParam("7", "Gate close", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderEnumParam("8", "Expander/Gate", "Gate", "Expander"),
		gateExpanderCurveParam("10", "Comp Attack", "0.01 ms", "1 ms", "10 ms", "100 ms", "500 ms"),
		gateExpanderCurveParam("11", "Comp Release", "1 ms", "10 ms", "100 ms", "500 ms", "2000 ms"),
		gateExpanderCurveParam("13", "Ratio", "1:1", "2:1", "4:1", "8:1", "20:1"),
		gateExpanderCurveParam("16", "Threshold", "-60 dB", "-40 dB", "-20 dB", "-10 dB", "0 dB"),
	}})
	if model == nil || code != "" || model.Classification != "hard_gate" {
		t.Fatalf("C1 comp-gate training surface model=%+v code=%q", model, code)
	}
	for _, binding := range append(append(append([]GateExpanderBinding{}, model.Stage.OperatingPoint...), model.Stage.GainAction...), model.Stage.Timing...) {
		if strings.Contains(strings.ToLower(binding.Name), "comp") || strings.EqualFold(binding.Name, "threshold") {
			t.Fatalf("compressor binding leaked into gate stage: %+v", binding)
		}
	}
}

func TestDetectGateExpanderModelPSEReleaseOnlyTrainingSurface(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{PluginName: "PSE Stereo", Parameters: []ParameterInfo{
		gateExpanderParam("0", "SC Source", "Internal"),
		gateExpanderCurveParam("2", "HPF Freq", "20 Hz", "100 Hz", "1000 Hz", "5000 Hz", "20000 Hz"),
		gateExpanderCurveParam("4", "LPF Freq", "1000 Hz", "3000 Hz", "5000 Hz", "10000 Hz", "20000 Hz"),
		gateExpanderCurveParam("5", "Threshold", "-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"),
		gateExpanderCurveParam("6", "Range", "-80 dB", "-60 dB", "-40 dB", "-20 dB", "0 dB"),
		gateExpanderEnumParam("7", "Release", "Slow", "Medium", "Fast"),
		gateExpanderEnumParam("8", "Ducker On/Off", "Off", "On"),
		gateExpanderParam("9", "DuckerGain", "0 dB"),
	}})
	if model == nil || code != "" || model.Classification != "downward_expander" || model.Stage.Direction != "downward_expansion" {
		t.Fatalf("PSE training surface model=%+v code=%q", model, code)
	}
	if !gateExpanderBindingsHaveRole(model.Stage.Detector, "sidechain_highpass") ||
		!gateExpanderBindingsHaveRole(model.Stage.Detector, "sidechain_lowpass") ||
		!gateExpanderBindingsHaveRole(model.Stage.GainAction, "range") {
		t.Fatalf("PSE stage=%+v", model.Stage)
	}
}

func TestDetectGateExpanderReleaseOnlySurfaceNeedsDownwardRangeEvidence(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderCurveParam("hpf", "SC HPF", "20 Hz", "100 Hz", "1000 Hz", "5000 Hz", "20000 Hz"),
		gateExpanderParam("t", "Threshold", "-30 dB"), gateExpanderParam("range", "Range", "12 dB"),
		gateExpanderParam("r", "Release", "300 ms"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("unproven release-only surface model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderModelProGPairedDirectionalTrainingSurface(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{PluginName: "Pro-G", Parameters: []ParameterInfo{
		gateExpanderCurveParam("0", "Threshold", "-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"),
		gateExpanderCurveParam("1", "Threshold (Upward)", "-30 dB", "-22.5 dB", "-15 dB", "-7.5 dB", "0 dB"),
		gateExpanderCurveParam("2", "Ratio", "1:1", "1.38:1", "2.75:1", "7:1", "100:1"),
		gateExpanderCurveParam("3", "Ratio (Upward)", "1:1", "1.5:1", "2:1", "2.5:1", "3:1"),
		gateExpanderCurveParam("4", "Range", "0 dB", "5 dB", "16 dB", "49.75 dB", "100 dB"),
		gateExpanderEnumParam("5", "Style", "Classic", "Clean", "Vocal", "Guitar", "Upward"),
		gateExpanderCurveParam("6", "Attack", "0 ms", "3.906 ms", "62.50 ms", "316.4 ms", "1000 ms"),
		gateExpanderCurveParam("7", "Release", "0 ms", "53.58 ms", "428.7 ms", "1447 ms", "5000 ms"),
		gateExpanderCurveParam("8", "Hold", "0 ms", "12.5 ms", "40 ms", "124.4 ms", "250 ms"),
		gateExpanderCurveParam("12", "Left Side Chain Level", "-24 dB", "-12 dB", "0 dB", "6 dB", "12 dB"),
		gateExpanderCurveParam("13", "Left Side Chain Mix", "0", "0.25", "0.5", "0.75", "1"),
		gateExpanderCurveParam("14", "Right Side Chain Level", "-24 dB", "-12 dB", "0 dB", "6 dB", "12 dB"),
		gateExpanderCurveParam("15", "Right Side Chain Mix", "0", "0.25", "0.5", "0.75", "1"),
		gateExpanderCurveParam("17", "Low Pass Frequency", "5 Hz", "44 Hz", "387 Hz", "3408 Hz", "30000 Hz"),
		gateExpanderCurveParam("18", "High Pass Frequency", "5 Hz", "44 Hz", "387 Hz", "3408 Hz", "30000 Hz"),
		gateExpanderEnumParam("34", "Ex Style", "(Other)", "Ducking"),
	}})
	if model == nil || code != "" || model.Classification != "downward_expander" || model.Stage.Direction != "downward_expansion" {
		t.Fatalf("Pro-G training surface model=%+v code=%q", model, code)
	}
	if !gateExpanderBindingsHaveRole(model.Stage.OperatingPoint, "threshold") ||
		!gateExpanderBindingsHaveRole(model.Stage.GainAction, "range") ||
		!gateExpanderBindingsHaveRole(model.Stage.GainAction, "expansion_ratio") {
		t.Fatalf("Pro-G downward stage=%+v", model.Stage)
	}
	if len(model.AuxiliaryStages) != 1 || model.AuxiliaryStages[0].Kind != "upward_expander" || len(model.AuxiliaryStages[0].ParamIDs) != 2 {
		t.Fatalf("Pro-G upward auxiliary=%+v", model.AuxiliaryStages)
	}
	if len(model.UnsupportedCapabilities) != 1 || model.UnsupportedCapabilities[0].Kind != "multichannel_detector" ||
		len(model.UnsupportedCapabilities[0].ParamIDs) != 4 {
		t.Fatalf("Pro-G multichannel detector boundary=%+v", model.UnsupportedCapabilities)
	}
	for _, binding := range model.Stage.Output {
		if strings.Contains(strings.ToLower(binding.Name), "side chain") {
			t.Fatalf("Pro-G side-chain binding leaked into output: %+v", binding)
		}
	}
}

func TestDetectGateExpanderPairedDirectionalSurfaceRequiresBothUpwardRoles(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderCurveParam("threshold", "Threshold", "-60 dB", "-30 dB", "0 dB"),
		gateExpanderCurveParam("ratio", "Ratio", "1:1", "2:1", "4:1", "10:1"),
		gateExpanderCurveParam("up-threshold", "Threshold (Upward)", "-30 dB", "-15 dB", "0 dB"),
		gateExpanderCurveParam("range", "Range", "0 dB", "20 dB", "50 dB"),
		gateExpanderCurveParam("attack", "Attack", "0 ms", "10 ms", "100 ms"),
		gateExpanderCurveParam("release", "Release", "10 ms", "100 ms", "1000 ms"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("incomplete paired direction model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderModelRejectsC1BidirectionalCompressor(t *testing.T) {
	ratio := gateExpanderCurveParam("ratio", "Ratio", "0.5:1", "0.75:1", "1:1", "2:1", "4:1")
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderParam("t", "Threshold", "-18 dB"), ratio,
		gateExpanderParam("a", "Attack", "10 ms"), gateExpanderParam("r", "Release", "100 ms"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderDoesNotUseHoldAsGateEvidence(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderParam("t", "Threshold", "-18 dB"), gateExpanderParam("range", "Range", "-40 dB"),
		gateExpanderParam("a", "Attack", "10 ms"), gateExpanderParam("h", "Hold", "20 ms"),
		gateExpanderParam("r", "Release", "100 ms"), gateExpanderParam("ratio", "Ratio", "4:1"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderRequiresProvenExpansionDirectionDomain(t *testing.T) {
	base := []ParameterInfo{
		gateExpanderParam("t", "Threshold", "-30 dB"), gateExpanderParam("range", "Range", "-40 dB"),
		gateExpanderParam("a", "Attack", "5 ms"), gateExpanderParam("r", "Release", "300 ms"),
	}
	unproven := append(append([]ParameterInfo{}, base...), gateExpanderParam("ratio", "Expander Ratio", "2:1"))
	if model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: unproven}); model != nil || code != "unresolved_gate_expander_surface" {
		t.Fatalf("unproven model=%+v code=%q", model, code)
	}

	proven := append(append([]ParameterInfo{}, base...), gateExpanderCurveParam("ratio", "Expander Ratio", "1:1", "1.25:1", "1.5:1", "2:1", "4:1"))
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: proven})
	if model == nil || code != "" || model.Classification != "downward_expander" || model.Stage.Direction != "downward_expansion" {
		t.Fatalf("proven model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderSelectableDirectionUsesObservedLabels(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderParam("t", "Threshold", "-30 dB"), gateExpanderParam("range", "Range", "-40 dB"),
		gateExpanderParam("a", "Attack", "5 ms"), gateExpanderParam("r", "Release", "300 ms"),
		gateExpanderEnumParam("mode", "Dynamic Mode", "Gate", "Expander"),
	}})
	if model == nil || code != "" || model.Classification != "selectable_gate_expander" ||
		!gateExpanderBindingsHaveRole(model.Stage.DirectionControl, "direction_mode") {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectGateExpanderRejectsUpwardExpanderDirection(t *testing.T) {
	model, code := DetectGateExpanderModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		gateExpanderParam("t", "Threshold", "-30 dB"), gateExpanderParam("range", "Range", "-40 dB"),
		gateExpanderParam("a", "Attack", "5 ms"), gateExpanderParam("r", "Release", "300 ms"),
		gateExpanderEnumParam("mode", "Dynamic Mode", "Upward Expander", "Compressor"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("upward direction must remain outside Gate/Expander: model=%+v code=%q", model, code)
	}
}

func TestGateExpanderGenerationIgnoresIdentityAndCurrentValues(t *testing.T) {
	makeDigest := func(name string, thresholdValue any) ParameterDigest {
		params := []ParameterInfo{
			gateExpanderParam("t", "Gate Threshold", "-30 dB"), gateExpanderParam("range", "Range", "-40 dB"),
			gateExpanderParam("a", "Attack", "5 ms"), gateExpanderParam("r", "Release", "300 ms"),
		}
		params[0].NormalizedValue = thresholdValue
		return ParameterDigest{PluginName: name, Parameters: params}
	}
	a, b := DetectGateExpanderModel(makeDigest("A", .1)), DetectGateExpanderModel(makeDigest("B", .9))
	if a == nil || b == nil || a.Generation != b.Generation {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}
