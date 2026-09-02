package plugingrabber

import (
	"fmt"
	"testing"
)

func limiterParam(id, name, valueText string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: 0.5, ValueText: valueText}
}

func TestDetectLimiterModelL2ThresholdCeilingTime(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{PluginName: "identity is ignored", Parameters: []ParameterInfo{
		limiterParam("1", "Thresh Slider", "-6 dB"), limiterParam("2", "Ceiling Slider", "-0.1 dB"),
		limiterParam("3", "Release Slider", "1.00 ms"), limiterParam("4", "ARC", "On"),
		limiterParam("5", "Quantize", "24 Bits"),
	}})
	if model == nil || code != "" || model.Classification != "threshold_ceiling_time" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if len(model.Stages) != 1 || !limiterBindingsHaveRole(model.Stages[0].Safety, "ceiling") ||
		!limiterBindingsHaveRole(model.Stages[0].Timing, "release") || !limiterBindingsHaveRole(model.Stages[0].Mode, "auto_release") {
		t.Fatalf("stage=%+v", model.Stages)
	}
}

func TestDetectLimiterModelDriveCeilingTime(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("1", "Gain", "6 dB"), limiterParam("2", "Ceiling", "-1 dB"),
		limiterParam("3", "Release", "100 ms"), limiterParam("4", "Limiter Mode", "Modern"),
		limiterParam("5", "Limiter Mix", "100 %"),
	}})
	if model == nil || code != "" || model.Classification != "drive_ceiling_time" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !limiterBindingsHaveRole(model.Stages[0].OperatingPoint, "input_drive") ||
		!limiterBindingsHaveRole(model.Stages[0].Mode, "limiter_mode") {
		t.Fatalf("stage=%+v", model.Stages[0])
	}
}

func TestDetectLimiterModelPromotesMeasuredPeakOutputAsCeiling(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("gain", "Gain", "0.00 dB"),
		limiterParam("look", "Lookahead", "0.180 ms"),
		limiterParam("release", "Release", "400.0 ms"),
		limiterParam("out", "Output Level", "0.00 dBTP"),
	}})
	if model == nil || code != "" || model.Classification != "drive_ceiling_time" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	stage := model.Stages[0]
	if !limiterBindingsHaveRole(stage.OperatingPoint, "input_drive") || !limiterBindingsHaveRole(stage.Safety, "ceiling") ||
		!limiterBindingsHaveRole(stage.Timing, "lookahead") || !limiterBindingsHaveRole(stage.Timing, "release") {
		t.Fatalf("stage=%+v", stage)
	}
}

func TestDetectLimiterModelIndexedStagesAndSharedControls(t *testing.T) {
	params := []ParameterInfo{
		limiterParam("tp", "True Peak", "On"),
	}
	for index := 1; index <= 2; index++ {
		params = append(params,
			limiterParam(fmt.Sprintf("b%d", index), fmt.Sprintf("Limiter Bypass %d", index), "Off"),
			limiterParam(fmt.Sprintf("i%d", index), fmt.Sprintf("Input Level %d", index), "0 dB"),
			limiterParam(fmt.Sprintf("o%d", index), fmt.Sprintf("Output Level %d", index), "0 dB"),
			limiterParam(fmt.Sprintf("ot%d", index), fmt.Sprintf("Output Trim %d", index), "0 dB"),
			limiterParam(fmt.Sprintf("r%d", index), fmt.Sprintf("Release Time %d", index), "20 ms"),
		)
	}
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: params})
	if model == nil || code != "" || len(model.Stages) != 2 || model.Classification != "indexed_multi_stage_limiter" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if len(model.SharedControls) != 1 || model.SharedControls[0].Role != "true_peak" {
		t.Fatalf("shared=%+v", model.SharedControls)
	}
	for _, stage := range model.Stages {
		if !limiterBindingsHaveRole(stage.Safety, "ceiling") || !limiterBindingsHaveRole(stage.OperatingPoint, "input_drive") {
			t.Fatalf("stage=%+v", stage)
		}
		if len(stage.Safety) != 1 || !limiterBindingsHaveRole(stage.Output, "output_gain") {
			t.Fatalf("ambiguous output promotion=%+v", stage)
		}
	}
}

func TestDetectLimiterModelRejectsClipper(t *testing.T) {
	params := []ParameterInfo{}
	for index := 1; index <= 2; index++ {
		params = append(params,
			limiterParam(fmt.Sprintf("i%d", index), fmt.Sprintf("Input Level %d", index), "0 dB"),
			limiterParam(fmt.Sprintf("t%d", index), fmt.Sprintf("Type %d", index), "Soft"),
			limiterParam(fmt.Sprintf("k%d", index), fmt.Sprintf("Knee %d", index), "0"),
			limiterParam(fmt.Sprintf("c%d", index), fmt.Sprintf("Ceiling %d", index), "0 dB"),
		)
	}
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: params})
	if model != nil || code != "unsupported_clipper" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectLimiterModelRejectsMultibandDynamics(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("x", "Crossover 1", "120 Hz"),
		limiterParam("t1", "Band 1 Threshold", "-12 dB"),
		limiterParam("r1", "Band 1 Release", "100 ms"),
		limiterParam("c1", "Band 1 Ceiling", "-1 dB"),
	}})
	if model != nil || code != "unsupported_multiband_dynamics" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectLimiterModelRejectsCompressorAndKeepsExplicitHybrid(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("t", "Threshold", "-18 dB"), limiterParam("r", "Ratio", "4:1"),
		limiterParam("c", "Ceiling", "-1 dB"), limiterParam("rel", "Release", "100 ms"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("compressor model=%+v code=%q", model, code)
	}

	hybrid, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("ct", "Compressor Threshold", "-18 dB"), limiterParam("cr", "Compressor Ratio", "4:1"),
		limiterParam("lt", "Limiter Threshold", "-1 dB"), limiterParam("lc", "Limiter Ceiling", "-0.1 dB"),
		limiterParam("lr", "Limiter Release", "100 ms"), limiterParam("le", "Limiter Enable", "On"),
	}})
	if hybrid == nil || code != "" || len(hybrid.AuxiliaryStages) == 0 {
		t.Fatalf("hybrid model=%+v code=%q", hybrid, code)
	}
}

func TestLimiterGenerationIgnoresIdentityAndCurrentValue(t *testing.T) {
	one := ParameterDigest{PluginName: "A", Parameters: []ParameterInfo{
		limiterParam("t", "Threshold", "-18 dB"), limiterParam("c", "Ceiling", "-1 dB"), limiterParam("r", "Release", "100 ms"),
	}}
	two := ParameterDigest{PluginName: "B", Parameters: []ParameterInfo{
		limiterParam("t", "Threshold", "-6 dB"), limiterParam("c", "Ceiling", "-0.1 dB"), limiterParam("r", "Release", "10 ms"),
	}}
	one.Parameters[0].NormalizedValue = 0.1
	two.Parameters[0].NormalizedValue = 0.9
	a, b := DetectLimiterModel(one), DetectLimiterModel(two)
	if a == nil || b == nil || a.Generation != b.Generation {
		t.Fatalf("generation mismatch a=%+v b=%+v", a, b)
	}
}

// TestDetectLimiterModelFabFilterProL2 classifies the exact product disclosed
// by the 2026-09-02 pluginprobe (all 32 probed surface names, verbatim) so the
// FAM4-S1 carrier's live kernel face stays covered by an in-repo test.
func TestDetectLimiterModelFabFilterProL2(t *testing.T) {
	model, code := DetectLimiterModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		limiterParam("0", "Gain", "0.00 dB"),
		limiterParam("1", "Style", "Modern"),
		limiterParam("2", "Lookahead", "0.180 ms"),
		limiterParam("3", "Attack", "275.0 ms"),
		limiterParam("4", "Release", "400.0 ms"),
		limiterParam("5", "Channel Link Transients", "75%"),
		limiterParam("6", "Channel Link Release", "100%"),
		limiterParam("7", "Channel Link Center", "Excluded"),
		limiterParam("8", "Channel Link LFE", "Excluded"),
		limiterParam("9", "Oversampling", "Off"),
		limiterParam("10", "True Peak Limiting", "On"),
		limiterParam("11", "Dithering", "Off"),
		limiterParam("12", "Noise Shaping", "Optimized"),
		limiterParam("13", "Filter DC Offset", "Off"),
		limiterParam("14", "Side Chain Triggering", "Off"),
		limiterParam("15", "Unity Gain", "Off"),
		limiterParam("16", "Audition Limiting", "Off"),
		limiterParam("17", "Bypass", "Not Bypassed"),
		limiterParam("18", "Output Level", "0.00 dBTP"),
		limiterParam("19", "Lock Output", "Locked"),
		limiterParam("20", "Receive Midi", "Enabled"),
		limiterParam("21", "Show Advanced", "Hide"),
		limiterParam("22", "True Peak Metering", "Show True Peaks"),
		limiterParam("23", "Display Mode", "Slow Down"),
		limiterParam("24", "Meter Scale", "-48 dB"),
		limiterParam("25", "Loudness Time Scale", "Short Term"),
		limiterParam("26", "Loudness Meter Scale", "Target +9"),
		limiterParam("27", "Loudness Meter Origin", "Absolute"),
		limiterParam("28", "Loudness Meter Target", "-14"),
		limiterParam("29", "Loudness Integrated Peak Tim", "Max Momentary"),
		limiterParam("30", "Loudness Recording", "Recording"),
		limiterParam("31", "Loudness Auto-Reset", "Disabled"),
	}})
	if model == nil || code != "" || model.Classification != "drive_ceiling_time" {
		t.Fatalf("Pro-L 2 model=%+v boundary=%q", model, code)
	}
	stage := model.Stages[0]
	if !limiterBindingsHaveRole(stage.OperatingPoint, "input_drive") || !limiterBindingsHaveRole(stage.Safety, "ceiling") ||
		!limiterBindingsHaveRole(stage.Timing, "lookahead") || !limiterBindingsHaveRole(stage.Timing, "release") {
		t.Fatalf("Pro-L 2 stage=%+v", stage)
	}
}
