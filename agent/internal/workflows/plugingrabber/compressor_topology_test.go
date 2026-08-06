package plugingrabber

import (
	"fmt"
	"testing"
)

func compressorParam(id, name, valueText string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, HostControllable: true, NormalizedValue: 0.5, ValueText: valueText}
}

func TestDetectCompressorModelThresholdDriven(t *testing.T) {
	digest := ParameterDigest{PluginName: "identity must not matter", Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Attack", "10 ms"), compressorParam("4", "Release", "120 ms"),
		compressorParam("5", "Makeup Gain", "3 dB"), compressorParam("6", "Mix", "75 %"),
	}}
	model := DetectCompressorModel(digest)
	if model == nil || model.Classification != "threshold_driven" {
		t.Fatalf("unexpected model: %+v", model)
	}
	if len(model.Stage.ControlPaths) != 1 || model.Stage.ControlPaths[0].Key != "main" {
		t.Fatalf("unexpected paths: %+v", model.Stage.ControlPaths)
	}
}

func TestDetectCompressorModelInputDrivenAndAmountDriven(t *testing.T) {
	input := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Input", "24"), compressorParam("2", "Ratio", "8:1"),
		compressorParam("3", "Attack", "3"), compressorParam("4", "Release", "7"),
	}})
	if input == nil || input.Classification != "input_driven_ratio" {
		t.Fatalf("input-driven model = %+v", input)
	}
	amount := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Peak Reduction", "40"), compressorParam("2", "Output Gain", "0 dB"),
	}})
	if amount == nil || amount.Classification != "amount_driven" {
		t.Fatalf("amount-driven model = %+v", amount)
	}
}

func TestDetectCompressorModelMinimalPeakReductionLeveler(t *testing.T) {
	model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Mode", "Compress"),
		compressorParam("2", "Recovery", "75 %"),
		compressorParam("3", "Gain", "24 %"),
		compressorParam("4", "Peak Reduction", "0 %"),
		compressorParam("5", "Drive", "25 %"),
		compressorParam("6", "Mix", "Wet"),
		compressorParam("7", "Meter Mode", "Reduction"),
	}})
	if model == nil || code != "" || model.Classification != "amount_driven" {
		t.Fatalf("minimal leveler model=%+v code=%q", model, code)
	}
	if !hasCompressorRole(model.Stage.ControlPaths[0].OperatingPoint, "reduction_amount") ||
		!hasCompressorRole(model.Stage.ControlPaths[0].Timing, "recovery") {
		t.Fatalf("minimal leveler path=%+v", model.Stage.ControlPaths[0])
	}
}

func TestDetectCompressorModelRejectsPureIndexedClipper(t *testing.T) {
	params := []ParameterInfo{}
	for channel := 1; channel <= 2; channel++ {
		params = append(params,
			compressorParam(fmt.Sprintf("input-%d", channel), fmt.Sprintf("Input Level %d", channel), "0 dB"),
			compressorParam(fmt.Sprintf("type-%d", channel), fmt.Sprintf("Type %d", channel), "FET"),
			compressorParam(fmt.Sprintf("knee-%d", channel), fmt.Sprintf("Knee %d", channel), "0"),
			compressorParam(fmt.Sprintf("ceiling-%d", channel), fmt.Sprintf("Ceiling %d", channel), "0 dB"),
			compressorParam(fmt.Sprintf("mix-%d", channel), fmt.Sprintf("Mix %d", channel), "100 %"),
			compressorParam(fmt.Sprintf("output-%d", channel), fmt.Sprintf("Output Level %d", channel), "0 dB"),
		)
	}
	model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unsupported_clipper" {
		t.Fatalf("pure indexed clipper model=%+v code=%q", model, code)
	}

	hybrid, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("threshold", "Threshold", "-12 dB"),
		compressorParam("ratio", "Ratio", "4:1"),
		compressorParam("input", "Input Level", "0 dB"),
		compressorParam("knee", "Knee", "3"),
		compressorParam("ceiling", "Ceiling", "-1 dB"),
	}})
	if hybrid == nil || code != "" || hybrid.Classification != "threshold_driven" {
		t.Fatalf("compressor with auxiliary ceiling model=%+v code=%q", hybrid, code)
	}
}

func TestDetectCompressorModelMultiPathLevelControl(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Low Level", "25"), compressorParam("2", "High Level", "40"),
		compressorParam("3", "Output", "0 dB"),
	}})
	if model == nil || model.Classification != "multi_path_level_control" || len(model.Stage.ControlPaths) != 2 {
		t.Fatalf("multi-path model = %+v", model)
	}
}

func TestDetectCompressorModelRetainsAuxiliaryClipper(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-12 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Clipper Threshold", "-1 dB"), compressorParam("4", "Clipper Enable", "On"),
	}})
	if model == nil || len(model.AuxiliaryStages) != 1 || model.AuxiliaryStages[0].Kind != "clipper" {
		t.Fatalf("auxiliary stages = %+v", model)
	}
}

func TestDetectCompressorModelRejectsLimiterAndMultiband(t *testing.T) {
	limiter := ParameterDigest{PluginName: "not consulted", Parameters: []ParameterInfo{
		compressorParam("1", "Limiter Threshold", "-6 dB"), compressorParam("2", "Limiter Release", "100 ms"),
		compressorParam("3", "Ceiling", "-0.1 dB"),
	}}
	if model := DetectCompressorModel(limiter); model != nil {
		t.Fatalf("pure limiter must be excluded: %+v", model)
	}
	identityFreeLimiter := ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-6 dB"), compressorParam("2", "Out Ceiling", "-0.1 dB"),
		compressorParam("3", "Release", "100 ms"), compressorParam("4", "IDR Type", "Type1"),
	}}
	if model := DetectCompressorModel(identityFreeLimiter); model != nil {
		t.Fatalf("ceiling topology without transfer evidence must be excluded: %+v", model)
	}
	hybrid := ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-6 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Limiter", "Off"), compressorParam("4", "Release", "100 ms"),
	}}
	if model := DetectCompressorModel(hybrid); model == nil || len(model.AuxiliaryStages) != 1 {
		t.Fatalf("compressor with auxiliary limiter must remain supported: %+v", model)
	}
	multiband := ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Band 1 Threshold", "-18 dB"), compressorParam("2", "Band 1 Ratio", "2:1"),
		compressorParam("3", "Band 2 Threshold", "-12 dB"), compressorParam("4", "Band 2 Ratio", "3:1"),
	}}
	if model := DetectCompressorModel(multiband); model != nil {
		t.Fatalf("multiband compressor must be excluded: %+v", model)
	}
}

func TestDetectCompressorModelReportsAdjacentDynamicsBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		params []ParameterInfo
		code   string
	}{
		{name: "true peak limiter", code: "unsupported_limiter", params: []ParameterInfo{
			compressorParam("1", "Input Trim", "0 dB"), compressorParam("2", "Release", "100 ms"),
			compressorParam("3", "Ceiling", "-0.1 dB"), compressorParam("4", "Limiter Mode", "True Peak"),
		}},
		{name: "gate", code: "unsupported_gate_expander", params: []ParameterInfo{
			compressorParam("1", "Threshold", "-24 dB"), compressorParam("2", "Attack", "1 ms"),
			compressorParam("3", "Release", "100 ms"), compressorParam("4", "Hold", "20 ms"),
			compressorParam("5", "Display Gate", "On"), compressorParam("6", "Display Input", "Active"),
			compressorParam("7", "Display Output", "Active"), compressorParam("8", "Sidechain", "Active"),
		}},
		{name: "spectral dynamics", code: "unsupported_spectral_dynamics", params: []ParameterInfo{
			compressorParam("1", "Threshold", "-12 dB"), compressorParam("2", "Compressor Ratio", "4:1"),
			compressorParam("3", "Threshold 1", "-18 dB"), compressorParam("4", "Frequency 1", "100 Hz"),
			compressorParam("5", "Q 1", "1"), compressorParam("6", "Threshold 2", "-12 dB"),
			compressorParam("7", "Frequency 2", "2 kHz"), compressorParam("8", "Q 2", "1"),
		}},
		{name: "named multiband", code: "unsupported_multiband_compressor", params: []ParameterInfo{
			compressorParam("1", "Low Threshold", "-18 dB"), compressorParam("2", "Low Ratio", "2:1"),
			compressorParam("3", "Low Attack", "10 ms"), compressorParam("4", "High Threshold", "-12 dB"),
			compressorParam("5", "High Ratio", "3:1"), compressorParam("6", "High Release", "80 ms"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: test.params})
			if model != nil || code != test.code {
				t.Fatalf("model=%+v boundary=%q want %q", model, code, test.code)
			}
		})
	}
}

func TestDetectCompressorModelKeepsSidechainEQOutOfMultibandEvidence(t *testing.T) {
	params := []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Att Time", "10 ms"), compressorParam("4", "Rel Time", "100 ms"),
	}
	for band := 1; band <= 8; band++ {
		params = append(params,
			compressorParam(fmt.Sprintf("f%d", band), fmt.Sprintf("SC EQ Band%d Freq.", band), "1 kHz"),
			compressorParam(fmt.Sprintf("t%d", band), fmt.Sprintf("SC EQ Band%d Type", band), "Bell"))
	}
	model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || model.Classification != "threshold_driven" {
		t.Fatalf("sidechain EQ contaminated compressor boundary: model=%+v code=%q", model, code)
	}
}

func TestDetectCompressorModelInputDrivenFixedTransfer(t *testing.T) {
	model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Input Gain", "4 dB"), compressorParam("2", "Response", "Normal"),
		compressorParam("3", "Output Gain", "0 dB"), compressorParam("4", "Dry/Wet Mix", "100 %"),
		compressorParam("5", "Stereo Link", "On"),
	}})
	if model == nil || code != "" || model.Classification != "input_driven_fixed_transfer" {
		t.Fatalf("fixed-transfer model=%+v code=%q", model, code)
	}
	if !hasCompressorRole(model.Stage.ControlPaths[0].Timing, "response") {
		t.Fatalf("response control missing: %+v", model.Stage.ControlPaths[0])
	}

	// Input plus a generic timing control is not enough to turn a limiter or
	// preamp-like surface into a compressor.
	if weak := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Input Trim", "0 dB"), compressorParam("2", "Release", "100 ms"),
	}}); weak != nil {
		t.Fatalf("weak input/timing surface must not be accepted: %+v", weak)
	}
}

func TestDetectCompressorModelRetainsHybridBroadbandStage(t *testing.T) {
	model, code := DetectCompressorModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Attack", "10 ms"), compressorParam("4", "Limiter Threshold", "-1 dB"),
		compressorParam("5", "K Comp Threshold", "-3 dB"), compressorParam("6", "K Comp In", "On"),
	}})
	if model == nil || code != "" || model.Classification != "threshold_driven" {
		t.Fatalf("hybrid broadband stage model=%+v code=%q", model, code)
	}
	if len(model.AuxiliaryStages) != 2 {
		t.Fatalf("hybrid auxiliaries=%+v", model.AuxiliaryStages)
	}
}

func TestDetectCompressorModelSeparatesProvableSequentialStages(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Optical Threshold 1", "-18 dB"), compressorParam("2", "Optical Gain 1", "0 dB"),
		compressorParam("3", "Discrete Threshold 1", "-12 dB"), compressorParam("4", "Discrete Ratio 1", "4:1"),
		compressorParam("5", "Discrete Attack 1", "10 ms"), compressorParam("6", "Discrete Recover 1", "100 ms"),
	}})
	if model == nil || model.Classification != "multi_stage_serial_control" || len(model.Stage.ControlPaths) != 2 {
		t.Fatalf("sequential stage model=%+v", model)
	}
	stages := map[string]bool{}
	for _, path := range model.Stage.ControlPaths {
		stages[path.CompressionStage] = true
	}
	if !stages["optical"] || !stages["discrete"] {
		t.Fatalf("sequential stages=%v paths=%+v", stages, model.Stage.ControlPaths)
	}
}

func TestCompressorTopologyGenerationIgnoresIdentityAndCurrentValue(t *testing.T) {
	one := ParameterDigest{PluginName: "A", Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
	}}
	two := ParameterDigest{PluginName: "B", Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-6 dB"), compressorParam("2", "Ratio", "10:1"),
	}}
	two.Parameters[0].NormalizedValue = 0.9
	a, b := DetectCompressorModel(one), DetectCompressorModel(two)
	if a == nil || b == nil || a.Generation != b.Generation {
		t.Fatalf("generation must be identity/value independent: %+v %+v", a, b)
	}
}

func TestDetectCompressorModelDoesNotUsePluginName(t *testing.T) {
	if model := DetectCompressorModel(ParameterDigest{PluginName: "Famous Compressor", Parameters: []ParameterInfo{
		compressorParam("1", "Color", "Warm"), compressorParam("2", "Noise", "Off"),
	}}); model != nil {
		t.Fatalf("identity invented a compressor topology: %+v", model)
	}
}

func TestDetectCompressorModelRealSurfaceShapesRemainIdentityFree(t *testing.T) {
	t.Run("linked dual path compression control", func(t *testing.T) {
		model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
			compressorParam("1", "Input L/M", "0 dB"), compressorParam("2", "Threshold L/M", "1.00"),
			compressorParam("3", "Compression L/M", "4.00"), compressorParam("4", "SC-HP L", "80 Hz"),
			compressorParam("5", "Input R/S", "0 dB"), compressorParam("6", "Threshold R/S", "1.00"),
			compressorParam("7", "Compression R/S", "4.00"), compressorParam("8", "SC-HP R", "80 Hz"),
			compressorParam("9", "Comp Mode", "STEREO"),
		}})
		if model == nil || model.Classification != "multi_path_channel_control" || len(model.Stage.ControlPaths) != 2 {
			t.Fatalf("linked dual-path model = %+v", model)
		}
		if model.Stage.ControlPaths[0].Key != "left_mid" || model.Stage.ControlPaths[1].Key != "right_side" {
			t.Fatalf("unexpected path keys: %+v", model.Stage.ControlPaths)
		}
		if !hasCompressorRole(model.Stage.ControlPaths[0].Transfer, "ratio") ||
			!hasCompressorRole(model.Stage.ControlPaths[0].Detector, "sidechain_filter") ||
			!hasCompressorRole(model.Stage.ControlPaths[0].Detector, "detector_mode") {
			t.Fatalf("bare Compression must expose a transfer binding: %+v", model.Stage.ControlPaths[0])
		}
	})

	t.Run("explicit output beats mix display group", func(t *testing.T) {
		output := compressorParam("3", "Output Gain", "0 dB")
		output.DisplayGroup = "Mix"
		model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
			compressorParam("1", "Peak Reduction", "40"), output, compressorParam("2", "Mix", "100 %"),
		}})
		if model == nil || !hasCompressorRole(model.Stage.Output, "output_gain") || !hasCompressorRole(model.Stage.Output, "mix") {
			t.Fatalf("output surface = %+v", model)
		}
	})

	t.Run("input control survives unrelated mix display group", func(t *testing.T) {
		input := compressorParam("1", "Input Control", "12")
		input.DisplayGroup = "Mix"
		model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
			input, compressorParam("2", "Recovery", "3"), compressorParam("3", "Compressor Type", "A"),
			compressorParam("4", "Input Control R", "12"), compressorParam("5", "Recovery R", "3"),
		}})
		if model == nil || len(model.Stage.ControlPaths) != 2 || model.Stage.ControlPaths[1].Key != "right" ||
			!hasCompressorRole(model.Stage.ControlPaths[0].OperatingPoint, "input_drive") {
			t.Fatalf("input-driven surface = %+v", model)
		}
	})

	t.Run("pdr is timing", func(t *testing.T) {
		model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
			compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
			compressorParam("3", "PDR Tc", "120 ms"),
		}})
		if model == nil || !hasCompressorRole(model.Stage.ControlPaths[0].Timing, "pdr_time") {
			t.Fatalf("PDR timing surface = %+v", model)
		}
	})
}

func TestDetectCompressorModelInfersBidirectionalRatioFromObservedDomain(t *testing.T) {
	minimum, maximum := 0.25, 4.0
	ratio := compressorParam("2", "Ratio", "1.00")
	ratio.DisplayDomainCandidate = &PluginDisplayDomain{Min: &minimum, Max: &maximum, Unit: "ratio", Confidence: 0.99}
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Input Gain", "0 dB"), ratio,
	}})
	if model == nil || model.Classification != "bidirectional_curve" || model.Stage.ControlPaths[0].Direction != "bidirectional" {
		t.Fatalf("bidirectional ratio surface = %+v", model)
	}
}

func TestCompressorCurveExcludesNonFiniteSentinelEndpoint(t *testing.T) {
	threshold := compressorParam("1", "Threshold", "-40 dB")
	threshold.DisplayProbe = &ParameterDisplayProbe{Samples: []ParameterDisplayProbeSample{
		{NormalizedValue: 0, Text: "-inf dB"},
		{NormalizedValue: 0.25, Text: "-60 dB"},
		{NormalizedValue: 0.5, Text: "-40 dB"},
		{NormalizedValue: 0.75, Text: "-20 dB"},
		{NormalizedValue: 1, Text: "0 dB"},
	}}
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		threshold, compressorParam("2", "Ratio", "4:1"),
	}})
	if model == nil {
		t.Fatal("sentinel threshold surface was rejected")
	}
	binding := model.Stage.ControlPaths[0].OperatingPoint[0]
	if len(binding.Curve) != 4 || binding.Curve[0] != [2]float64{0.25, -60} {
		t.Fatalf("finite curve=%v", binding.Curve)
	}
}

func TestCompressorCurveRetainsNumericInteriorBetweenSymbolicEndpoints(t *testing.T) {
	drive := compressorParam("1", "Drive", "25.0 %")
	drive.NormalizedValue = 0.25
	drive.DisplayProbe = &ParameterDisplayProbe{Samples: []ParameterDisplayProbeSample{
		{NormalizedValue: 0, Text: "Clean"},
		{NormalizedValue: 0.25, Text: "25.0 %"},
		{NormalizedValue: 0.5, Text: "50.0 %"},
		{NormalizedValue: 0.75, Text: "75.0 %"},
		{NormalizedValue: 1, Text: "Dirty"},
	}}
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		drive, compressorParam("2", "Peak Reduction", "0.0 %"), compressorParam("3", "Recovery", "75.0 %"),
	}})
	if model == nil || model.Classification != "amount_driven" {
		t.Fatalf("mixed-endpoint leveler model=%+v", model)
	}
	binding := model.Stage.ControlPaths[0].OperatingPoint[0]
	if binding.Role != "input_drive" {
		t.Fatalf("first operating-point binding=%+v", binding)
	}
	want := [][2]float64{{0.25, 25}, {0.5, 50}, {0.75, 75}}
	if len(binding.Curve) != len(want) {
		t.Fatalf("mixed-endpoint curve=%v want=%v", binding.Curve, want)
	}
	for index := range want {
		if binding.Curve[index] != want[index] {
			t.Fatalf("mixed-endpoint curve=%v want=%v", binding.Curve, want)
		}
	}
}

func TestDetectCompressorModelUsesObservedBareCompressionSemantics(t *testing.T) {
	amount := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Compression", "6.0 dB"), compressorParam("2", "Gain", "0.0 dB"),
		compressorParam("3", "Gate", "-Inf dB"),
	}})
	if amount == nil || amount.Classification != "amount_driven" ||
		!hasCompressorRole(amount.Stage.ControlPaths[0].OperatingPoint, "reduction_amount") {
		t.Fatalf("dB compression amount = %+v", amount)
	}
	ratio := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Input", "0 dB"), compressorParam("2", "Compression", "4.00"),
	}})
	if ratio == nil || ratio.Classification != "input_driven_ratio" ||
		!hasCompressorRole(ratio.Stage.ControlPaths[0].Transfer, "ratio") {
		t.Fatalf("unitless compression curve = %+v", ratio)
	}
}

func TestDetectCompressorModelRecognizesTimeConstantAndMergesDetectorBranches(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Left Input", "12"), compressorParam("2", "Left Threshold", "2.2"),
		compressorParam("3", "Left Time Constant", "1"), compressorParam("4", "Left Output", "0.3 dB"),
		compressorParam("5", "Right Input", "12"), compressorParam("6", "Right Threshold", "2.2"),
		compressorParam("7", "Right Time Constant", "1"), compressorParam("8", "Right Output", "0.3 dB"),
	}})
	if model == nil || model.Classification != "multi_path_channel_control" || len(model.Stage.ControlPaths) != 2 ||
		!hasCompressorRole(model.Stage.ControlPaths[0].Timing, "time_constant") {
		t.Fatalf("time-constant dual path = %+v", model)
	}

	merged := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Side Chain Low Frequency", "100 Hz"),
		compressorParam("4", "Side Chain Mid Q", "1.0"),
	}})
	if merged == nil || len(merged.Stage.ControlPaths) != 1 || len(merged.Stage.ControlPaths[0].Detector) != 2 {
		t.Fatalf("single core with detector branches = %+v", merged)
	}
}

func TestDetectCompressorModelIgnoresGroupingAndGenericRoleContamination(t *testing.T) {
	gain := compressorParam("3", "Gain0", "0.0")
	gain.DisplayGroup = "Dynamics"
	gain.NormalizedRole = "comp_makeup_gain"
	detector := compressorParam("4", "Side Chain Mid Gain", "0.0 dB")
	detector.DisplayGroup = "Mix"
	detector.NormalizedRole = "common_gain"
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"), gain, detector,
	}})
	if model == nil || len(model.Stage.ControlPaths) != 1 || hasCompressorRole(model.Stage.ControlPaths[0].GainAction, "makeup_gain") {
		t.Fatalf("grouping contamination entered topology: %+v", model)
	}
	if !hasCompressorRole(model.Stage.ControlPaths[0].Detector, "detector_mode") {
		t.Fatalf("side-chain parameter must remain detector evidence: %+v", model.Stage.ControlPaths[0])
	}
}

func TestDetectCompressorModelExposesModernCompressorModesWithoutPan(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Threshold", "-18 dB"), compressorParam("2", "Ratio", "4:1"),
		compressorParam("3", "Style", "Clean"), compressorParam("4", "Range", "60 dB"),
		compressorParam("5", "Lookahead", "1 ms"), compressorParam("6", "Auto Release", "Off"),
		compressorParam("7", "Input Pan", "Center"), compressorParam("8", "Output Pan", "Center"),
		compressorParam("9", "Output Level", "0 dB"), compressorParam("10", "L/R Link", "100%"),
	}})
	if model == nil || len(model.Stage.ControlPaths) != 1 {
		t.Fatalf("modern compressor topology = %+v", model)
	}
	path := model.Stage.ControlPaths[0]
	if !hasCompressorRole(path.Transfer, "transfer_mode") || !hasCompressorRole(path.GainAction, "reduction_range") ||
		!hasCompressorRole(path.Timing, "lookahead") || !hasCompressorRole(path.Timing, "auto_release") ||
		!hasCompressorRole(path.Detector, "channel_link") {
		t.Fatalf("modern compressor roles = %+v", path)
	}
	if len(path.OperatingPoint) != 1 || len(model.Stage.Output) != 1 || model.Stage.Output[0].Name != "Output Level" {
		t.Fatalf("pan controls entered compressor topology: path=%+v output=%+v", path, model.Stage.Output)
	}
}

func TestDetectCompressorModelCopiesSharedModifiersAcrossLevelPaths(t *testing.T) {
	model := DetectCompressorModel(ParameterDigest{Parameters: []ParameterInfo{
		compressorParam("1", "Low Level", "20"), compressorParam("2", "High Level", "30"),
		compressorParam("3", "ARC Type", "Auto"),
	}})
	if model == nil || len(model.Stage.ControlPaths) != 2 {
		t.Fatalf("shared modifier created a third path: %+v", model)
	}
	for _, path := range model.Stage.ControlPaths {
		if !hasCompressorRole(path.Transfer, "transfer_mode") {
			t.Fatalf("shared transfer mode was not copied to %s: %+v", path.Key, path)
		}
	}
}
