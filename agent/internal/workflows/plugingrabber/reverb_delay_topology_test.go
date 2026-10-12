package plugingrabber

import "testing"

// M2-TIMEFX-1 topology recognizer tests. Fixtures are identity-free parameter
// surfaces modeled on the family's canonical controls: a reverb needs
// decay-time evidence, a delay needs echo-time evidence; everything else
// stays an auxiliary boundary.

func timefxParam(id, name, valueText string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: 0.5, ValueText: valueText}
}

func TestDetectReverbModelDecayPredelayWet(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{PluginName: "identity is ignored", Parameters: []ParameterInfo{
		timefxParam("1", "Decay Time", "1.20 s"), timefxParam("2", "Pre-Delay", "20 ms"),
		timefxParam("3", "Wet", "35 %"), timefxParam("4", "Stereo Width", "100 %"),
		timefxParam("5", "Output Level", "0.00 dB"),
	}})
	if model == nil || code != "" || model.Classification != "predelay_decay_reverb" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !reverbBindingRolesContains(model.Stage.Tail, "decay") || !reverbBindingRolesContains(model.Stage.Timing, "pre_delay") ||
		!reverbBindingRolesContains(model.Stage.Output, "wet") || !reverbBindingRolesContains(model.Stage.Output, "output_gain") {
		t.Fatalf("stage=%+v", model.Stage)
	}
	if model.Generation == "" || len(model.AuxiliaryStages) != 0 {
		t.Fatalf("generation=%q aux=%+v", model.Generation, model.AuxiliaryStages)
	}
}

func TestDetectReverbModelSizeOnlyTail(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("a", "Room Size", "0.50"), timefxParam("b", "Dry/Wet Mix", "30 %"),
	}})
	if model == nil || code != "" || model.Classification != "decay_time_reverb" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !reverbBindingRolesContains(model.Stage.Tail, "size") {
		t.Fatalf("stage=%+v", model.Stage)
	}
}

func TestDetectReverbModelRejectsDelaySurface(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Time", "375 ms"), timefxParam("2", "Feedback", "25 %"), timefxParam("3", "Mix", "20 %"),
	}})
	if model != nil || code != "unsupported_delay" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectReverbModelRejectsWetOnlySurface(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Wet", "35 %"), timefxParam("2", "Width", "100 %"),
	}})
	if model != nil || code != "unresolved_reverb_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectReverbModelRejectsCompressorSurface(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Threshold", "-10 dB"), timefxParam("2", "Ratio", "4 : 1"), timefxParam("3", "Release", "100 ms"),
	}})
	if model != nil || code != "unsupported_broadband_compressor" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectReverbModelNotReverb(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Tone", "0.5"),
	}})
	if model != nil || code != "not_reverb" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectDelayModelTimeFeedbackWet(t *testing.T) {
	model, code := DetectDelayModelWithBoundary(ParameterDigest{PluginName: "identity is ignored", Parameters: []ParameterInfo{
		timefxParam("1", "Time", "375 ms"), timefxParam("2", "Feedback", "25 %"),
		timefxParam("3", "Mix", "20 %"), timefxParam("4", "Ping Pong", "Off"),
		timefxParam("5", "Output", "0.00 dB"),
	}})
	if model == nil || code != "" || model.Classification != "feedback_delay" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !delayBindingRolesContains(model.Stage.Timing, "time") || !delayBindingRolesContains(model.Stage.Feedback, "feedback") ||
		!delayBindingRolesContains(model.Stage.Output, "mix") || !delayBindingRolesContains(model.Stage.Character, "ping_pong") {
		t.Fatalf("stage=%+v", model.Stage)
	}
	if model.Generation == "" || len(model.AuxiliaryStages) != 0 {
		t.Fatalf("generation=%q aux=%+v", model.Generation, model.AuxiliaryStages)
	}
}

func TestDetectDelayModelSyncSubdivision(t *testing.T) {
	model, code := DetectDelayModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Sync", "1/8"), timefxParam("2", "Feedback", "35 %"),
	}})
	if model == nil || code != "" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !delayBindingRolesContains(model.Stage.Timing, "sync") {
		t.Fatalf("stage=%+v", model.Stage)
	}
}

func TestDetectDelayModelRejectsReverbSurface(t *testing.T) {
	model, code := DetectDelayModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Decay", "2.00 s"), timefxParam("2", "Predelay", "10 ms"), timefxParam("3", "Mix", "30 %"),
	}})
	if model != nil || code != "unsupported_reverb" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectDelayModelNotDelay(t *testing.T) {
	model, code := DetectDelayModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Tone", "0.5"),
	}})
	if model != nil || code != "not_delay" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestBuildReverbSummaryShape(t *testing.T) {
	summary, code := BuildReverbSummaryWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Decay Time", "1.20 s"), timefxParam("2", "Pre-Delay", "20 ms"), timefxParam("3", "Wet", "35 %"),
	}})
	if summary == nil || code != "" {
		t.Fatalf("summary=%+v code=%q", summary, code)
	}
	if summary["mapping_source"] != "generic_structural" || summary["schema_version"] != reverbTopologySchema {
		t.Fatalf("summary header=%+v", summary)
	}
	stage := stageRowsOf(summary["reverb_stage"])
	if len(stageRows(stage, "tail")) == 0 || len(stageRows(stage, "timing")) == 0 {
		t.Fatalf("stage=%+v", stage)
	}
}

func TestBuildDelaySummaryShape(t *testing.T) {
	summary, code := BuildDelaySummaryWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Time", "375 ms"), timefxParam("2", "Feedback", "25 %"),
	}})
	if summary == nil || code != "" {
		t.Fatalf("summary=%+v code=%q", summary, code)
	}
	if summary["mapping_source"] != "generic_structural" || summary["schema_version"] != delayTopologySchema {
		t.Fatalf("summary header=%+v", summary)
	}
	stage := stageRowsOf(summary["delay_stage"])
	if len(stageRows(stage, "timing")) == 0 || len(stageRows(stage, "feedback")) == 0 {
		t.Fatalf("stage=%+v", stage)
	}
}

func reverbBindingRolesContains(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func delayBindingRolesContains(bindings []CompressorBinding, role string) bool {
	for _, binding := range bindings {
		if binding.Role == role {
			return true
		}
	}
	return false
}

func stageRowsOf(value any) map[string]any {
	if stage, ok := value.(map[string]any); ok {
		return stage
	}
	return map[string]any{}
}

func stageRows(stage map[string]any, section string) []map[string]any {
	rows, _ := stage[section].([]map[string]any)
	return rows
}
