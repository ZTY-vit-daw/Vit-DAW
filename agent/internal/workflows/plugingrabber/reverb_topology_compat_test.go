package plugingrabber

import "testing"

// PLUGGRABBER-TIMEDYN-COMPAT-1 plate-family vocabulary tests. Fixtures stay
// identity-free and mirror the plate family's canonical controls: the decay
// control is a Damper (physical plate damping) rather than a decay/size/length
// wording, so the tail core must be provable through the plate tier with
// reduced confidence while character-only surfaces keep being rejected.

func TestDetectReverbModelPlateDamperTail(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{PluginName: "identity is ignored", Parameters: []ParameterInfo{
		timefxParam("0", "Plate Selector", "A"), timefxParam("1", "Damper", "0.45"),
		timefxParam("2", "BassCut", "0.20"), timefxParam("4", "Predelay", "10 ms"),
		timefxParam("wet level", "Wet Level", "-6.0 dB"), timefxParam("10", "Dry/Wet", "25 %"),
	}})
	if model == nil || code != "" || model.Classification != "predelay_decay_reverb" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !reverbBindingRolesContains(model.Stage.Tail, "decay") {
		t.Fatalf("plate damper missing from tail: %+v", model.Stage.Tail)
	}
	if !reverbBindingRolesContains(model.Stage.Timing, "pre_delay") ||
		!reverbBindingRolesContains(model.Stage.Output, "wet") ||
		!reverbBindingRolesContains(model.Stage.Character, "reverb_mode") {
		t.Fatalf("stage sections: timing=%+v output=%+v character=%+v",
			model.Stage.Timing, model.Stage.Output, model.Stage.Character)
	}
	if model.Stage.TailEvidence != reverbTailEvidencePlateFamily {
		t.Fatalf("tail evidence tier=%q want %q", model.Stage.TailEvidence, reverbTailEvidencePlateFamily)
	}
	if model.Confidence != 0.90 {
		t.Fatalf("plate-family confidence=%v want 0.90", model.Confidence)
	}
}

func TestDetectReverbModelPlateDamperCoexistsWithDamping(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damper", "0.45"), timefxParam("7", "HF Damping", "0.30"),
		timefxParam("wet level", "Wet Level", "-6.0 dB"),
	}})
	if model == nil || code != "" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if !reverbBindingRolesContains(model.Stage.Tail, "decay") ||
		!reverbBindingRolesContains(model.Stage.Character, "damping") {
		t.Fatalf("damper must stay tail while generic damping stays character: %+v", model.Stage)
	}
	if model.Stage.TailEvidence != reverbTailEvidencePlateFamily {
		t.Fatalf("tail evidence tier=%q", model.Stage.TailEvidence)
	}
}

func TestDetectReverbModelDamperPedalGuard(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damper Pedal", "On"), timefxParam("2", "Volume", "0.50"),
	}})
	if model != nil || code != "unresolved_reverb_surface" {
		t.Fatalf("damper-pedal wording must not prove a reverb tail: model=%+v code=%q", model, code)
	}
}

func TestDetectReverbModelDirectWordingOutranksPlateFamily(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damper", "0.45"), timefxParam("2", "Decay Time", "2.40 s"),
		timefxParam("3", "Wet", "35 %"),
	}})
	if model == nil || code != "" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
	if model.Stage.TailEvidence != reverbTailEvidenceDirect || model.Confidence != 0.96 {
		t.Fatalf("mixed surface must keep the direct tier: tier=%q confidence=%v",
			model.Stage.TailEvidence, model.Confidence)
	}
}

func TestDetectReverbModelCharacterOnlyStillRejected(t *testing.T) {
	model, code := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damping", "0.30"), timefxParam("2", "Width", "100 %"),
		timefxParam("3", "Modulation", "Off"), timefxParam("4", "Plate Selector", "B"),
	}})
	if model != nil || code != "unresolved_reverb_surface" {
		t.Fatalf("character-only surface must stay rejected: model=%+v code=%q", model, code)
	}
}

func TestReverbPlateFamilyConfidenceBelowDirect(t *testing.T) {
	plate, plateCode := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damper", "0.45"), timefxParam("wet level", "Wet Level", "-6.0 dB"),
	}})
	direct, directCode := DetectReverbModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Decay Time", "2.40 s"), timefxParam("2", "Wet", "35 %"),
	}})
	if plate == nil || plateCode != "" || direct == nil || directCode != "" {
		t.Fatalf("plate=%+v/%q direct=%+v/%q", plate, plateCode, direct, directCode)
	}
	if !(plate.Confidence < direct.Confidence) {
		t.Fatalf("plate confidence %v must sit below direct confidence %v", plate.Confidence, direct.Confidence)
	}
}

func TestBuildReverbSummaryCarriesTailEvidence(t *testing.T) {
	summary, code := BuildReverbSummaryWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Damper", "0.45"), timefxParam("4", "Predelay", "10 ms"),
		timefxParam("wet level", "Wet Level", "-6.0 dB"),
	}})
	if summary == nil || code != "" {
		t.Fatalf("summary=%+v code=%q", summary, code)
	}
	if summary["tail_evidence"] != reverbTailEvidencePlateFamily {
		t.Fatalf("summary tail_evidence=%v", summary["tail_evidence"])
	}
	direct, code := BuildReverbSummaryWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		timefxParam("1", "Decay Time", "2.40 s"), timefxParam("2", "Wet", "35 %"),
	}})
	if direct == nil || code != "" {
		t.Fatalf("direct summary=%+v code=%q", direct, code)
	}
	if direct["tail_evidence"] != reverbTailEvidenceDirect {
		t.Fatalf("direct summary tail_evidence=%v", direct["tail_evidence"])
	}
}
