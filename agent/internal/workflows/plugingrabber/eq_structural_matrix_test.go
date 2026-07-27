package plugingrabber

import "testing"

func TestDetectEQModelRecoversMirroredFixedBandsWithoutGhostDecimalBand(t *testing.T) {
	params := []ParameterInfo{}
	for _, channel := range []string{"Left", "Right"} {
		for index, frequency := range []string{"1Khz", "1.6Khz", "2Khz"} {
			params = append(params, eqParam(channel+frequency, channel+" "+frequency, "0.0 dB", eqRange(-6, 6, "linear")))
			params[len(params)-1].ID = channel + string(rune('0'+index))
		}
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil || len(model.Bands) != 3 || len(model.Channels) != 2 {
		t.Fatalf("model=%+v", model)
	}
	if model.Bands[1].FixedFrequencyHz == nil || *model.Bands[1].FixedFrequencyHz != 1600 {
		t.Fatalf("decimal kHz band=%+v", model.Bands[1])
	}
	if len(model.Bands[1].Bindings[eqRoleGain]) != 2 {
		t.Fatalf("mirrored bindings=%+v", model.Bands[1].Bindings)
	}
}

func TestDetectEQModelUsesEnumPhysicalValuesAndRejectsCompressorClassification(t *testing.T) {
	freqMin, freqMax := 0.0, 1.0
	dbMin, dbMax := 0.0, 1.0
	makeEnum := func(id, name string, labels []ParameterDisplayProbeLabel) ParameterInfo {
		return ParameterInfo{ID: id, Name: name, HostControllable: true, IsDiscrete: true,
			NumSteps: len(labels), DisplayDomainCandidate: &PluginDisplayDomain{Scale: "enum", Min: &freqMin, Max: &freqMax},
			DisplayProbe: &ParameterDisplayProbe{DiscreteLabels: labels}}
	}
	params := []ParameterInfo{
		makeEnum("f1", "High Freq", []ParameterDisplayProbeLabel{{Value: 0.0, Label: "5.0kHz"}, {Value: 1.0, Label: "15.0kHz"}}),
		makeEnum("g1", "High Gain", []ParameterDisplayProbeLabel{{Value: 0.0, Label: "-12 dB"}, {Value: 1.0, Label: "+12 dB"}}),
		makeEnum("f2", "Low Freq", []ParameterDisplayProbeLabel{{Value: 0.0, Label: "50Hz"}, {Value: 1.0, Label: "400Hz"}}),
		makeEnum("g2", "Low Gain", []ParameterDisplayProbeLabel{{Value: 0.0, Label: "-12 dB"}, {Value: 1.0, Label: "+12 dB"}}),
	}
	_ = dbMin
	_ = dbMax
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil || model.Classification != "fixed_slot_adjustable" || len(model.Bands) != 2 {
		t.Fatalf("enum model=%+v", model)
	}
	if DetectEQModel(ParameterDigest{TemplateRole: "comp", Parameters: params}) != nil {
		t.Fatal("compressor-classified local filter bank must not become a top-level EQ")
	}
}
