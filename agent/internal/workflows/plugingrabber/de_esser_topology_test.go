package plugingrabber

import "testing"

func deEsserParam(id, name, valueText string) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, RawName: name, HostControllable: true, NormalizedValue: 0.5, ValueText: valueText}
}

func TestDetectDeEsserModelTrainingSurfaces(t *testing.T) {
	tests := []struct {
		name           string
		classification string
		params         []ParameterInfo
	}{
		{name: "FabFilter disclosed training surface", classification: "frequency_selective_de_esser", params: []ParameterInfo{
			deEsserParam("0", "Mode", "Single Vocal"), deEsserParam("1", "Threshold", "-36.00 dB"),
			deEsserParam("2", "Range", "6.00 dB"), deEsserParam("4", "Stereo Link", "100%"),
			deEsserParam("6", "Lookahead", "12.00 ms"), deEsserParam("8", "Audition Triggering", "Off"),
			deEsserParam("9", "Side Chain Input Signal", "Normal input"), deEsserParam("10", "High-Pass Frequency", "7000.0 Hz"),
			deEsserParam("11", "Low-Pass Frequency", "14000 Hz"), deEsserParam("12", "Audition Side Chain", "Off"),
			deEsserParam("17", "Output Level", "0.00 dB"),
		}},
		{name: "Waves DeEsser disclosed training surface", classification: "frequency_selective_de_esser", params: []ParameterInfo{
			deEsserParam("0", "Threshold", "0.0 dB"), deEsserParam("1", "Freq", "5506 Hz"),
			deEsserParam("2", "SideChain", "HighPass"), deEsserParam("3", "Audio", "Split"),
			deEsserParam("4", "Monitor", "Output"),
		}},
		{name: "Waves RDeEsser disclosed training surface", classification: "frequency_selective_de_esser", params: []ParameterInfo{
			deEsserParam("0", "Freq", "5506 Hz"), deEsserParam("1", "FilterType", "HighPass"),
			deEsserParam("2", "FilterMode", "Split"), deEsserParam("3", "Range", "-16.0 dB"),
			deEsserParam("4", "Threshold", "-40.0 dB"), deEsserParam("5", "Monitor", "Audio"),
		}},
		{name: "Waves Sibilance disclosed training surface", classification: "sibilance_detector_de_esser", params: []ParameterInfo{
			deEsserParam("0", "Lookahead", "Off"), deEsserParam("1", "Detection", "50"),
			deEsserParam("2", "Threshold", "-12.5 dB"), deEsserParam("3", "Range", "-12.0 dB"),
			deEsserParam("4", "Mode", "50"), deEsserParam("5", "Monitor", "Off"),
		}},
		{name: "Waves Sibilance Live disclosed training surface", classification: "sibilance_detector_de_esser", params: []ParameterInfo{
			deEsserParam("0", "Detection", "50"), deEsserParam("1", "Threshold", "-12.5 dB"),
			deEsserParam("2", "Range", "-12.0 dB"), deEsserParam("3", "Mode", "50"),
			deEsserParam("4", "Monitor", "Off"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, code := DetectDeEsserModelWithBoundary(ParameterDigest{PluginName: "identity is ignored", Parameters: test.params})
			if model == nil || code != "" || model.Classification != test.classification || len(model.Stages) != 1 {
				t.Fatalf("model=%+v code=%q", model, code)
			}
			stage := model.Stages[0]
			if !deEsserHasRole(stage.OperatingPoint, "threshold") {
				t.Fatalf("threshold was not bound: %+v", stage)
			}
		})
	}
}

func TestDetectDeEsserModelRejectsSingleNamedParameter(t *testing.T) {
	model, code := DetectDeEsserModelWithBoundary(ParameterDigest{Parameters: []ParameterInfo{
		deEsserParam("1", "De-esser Threshold", "-20 dB"),
	}})
	if model != nil || code != "unresolved_de_esser_surface" {
		t.Fatalf("model=%+v code=%q", model, code)
	}
}

func TestDetectDeEsserModelNegativeBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		params []ParameterInfo
	}{
		{name: "broadband compressor with sidechain EQ", code: "unsupported_broadband_compressor", params: []ParameterInfo{
			deEsserParam("t", "Threshold", "-18 dB"), deEsserParam("ratio", "Ratio", "4:1"),
			deEsserParam("a", "Attack", "10 ms"), deEsserParam("r", "Release", "100 ms"),
			deEsserParam("f", "Side Chain Frequency", "6000 Hz"), deEsserParam("filter", "Sidechain FilterType", "HighPass"),
		}},
		{name: "dynamic EQ", code: "unsupported_spectral_dynamics", params: []ParameterInfo{
			deEsserParam("t", "Band Threshold", "-18 dB"), deEsserParam("range", "Band Range", "-6 dB"),
			deEsserParam("f", "Band Frequency", "6000 Hz"), deEsserParam("q", "Band Q", "2.0"),
			deEsserParam("a", "Band Attack", "10 ms"), deEsserParam("r", "Band Release", "100 ms"),
		}},
		{name: "multiband dynamics", code: "unsupported_multiband_dynamics", params: []ParameterInfo{
			deEsserParam("x", "Crossover 1", "200 Hz"), deEsserParam("t1", "Band Threshold 1", "-18 dB"),
			deEsserParam("r1", "Band Range 1", "-6 dB"), deEsserParam("t2", "Band Threshold 2", "-18 dB"),
			deEsserParam("r2", "Band Range 2", "-6 dB"),
		}},
		{name: "gate", code: "unsupported_gate_expander", params: []ParameterInfo{
			deEsserParam("t", "Gate Threshold", "-30 dB"), deEsserParam("range", "Gate Range", "-60 dB"),
			deEsserParam("h", "Hold", "50 ms"), deEsserParam("r", "Release", "200 ms"),
		}},
		{name: "limiter", code: "unsupported_limiter", params: []ParameterInfo{
			deEsserParam("t", "Threshold", "-6 dB"), deEsserParam("c", "Ceiling", "-1 dB"),
			deEsserParam("r", "Release", "100 ms"), deEsserParam("f", "Frequency", "6000 Hz"),
		}},
		{name: "clipper", code: "unsupported_clipper", params: []ParameterInfo{
			deEsserParam("t", "Clip Threshold", "-1 dB"), deEsserParam("s", "Clipper Shape", "Soft"),
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, code := DetectDeEsserModelWithBoundary(ParameterDigest{Parameters: test.params})
			if model != nil || code != test.code {
				t.Fatalf("model=%+v code=%q want=%q", model, code, test.code)
			}
		})
	}
}

func TestDeEsserGenerationIgnoresIdentityAndCurrentValue(t *testing.T) {
	one := ParameterDigest{PluginName: "A", Parameters: []ParameterInfo{
		deEsserParam("t", "Threshold", "-36 dB"), deEsserParam("range", "Range", "6 dB"),
		deEsserParam("f", "High-Pass Frequency", "7000 Hz"),
	}}
	two := ParameterDigest{PluginName: "B", Parameters: []ParameterInfo{
		deEsserParam("t", "Threshold", "-12 dB"), deEsserParam("range", "Range", "12 dB"),
		deEsserParam("f", "High-Pass Frequency", "9000 Hz"),
	}}
	one.Parameters[0].NormalizedValue = 0.1
	two.Parameters[0].NormalizedValue = 0.9
	a, b := DetectDeEsserModel(one), DetectDeEsserModel(two)
	if a == nil || b == nil || a.Generation != b.Generation {
		t.Fatalf("generation mismatch a=%+v b=%+v", a, b)
	}
}
