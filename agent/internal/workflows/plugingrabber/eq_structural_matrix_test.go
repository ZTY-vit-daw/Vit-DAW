package plugingrabber

import (
	"fmt"
	"slices"
	"testing"
)

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

func BenchmarkDetectEQModel512Parameters(b *testing.B) {
	params := make([]ParameterInfo, 0, 512)
	for band := 1; band <= 24; band++ {
		prefix := fmt.Sprintf("Band %d", band)
		params = append(params,
			typedContinuousParam(fmt.Sprintf("f%d", band), prefix+" Frequency", "1000 Hz", 10, 30000),
			typedContinuousParam(fmt.Sprintf("g%d", band), prefix+" Gain", "0 dB", -30, 30),
			typedContinuousParam(fmt.Sprintf("q%d", band), prefix+" Q", "1.0", 0.025, 40),
			typedEnumParam(fmt.Sprintf("t%d", band), prefix+" Type", "Bell", "Bell", "Low Shelf", "High Shelf", "Low Cut", "High Cut", "Notch"),
		)
	}
	for len(params) < 512 {
		i := len(params)
		params = append(params, ParameterInfo{ID: fmt.Sprintf("noise-%d", i), Name: fmt.Sprintf("Peripheral Control %d", i), ValueText: "Off"})
	}
	digest := ParameterDigest{Parameters: params}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if model := DetectEQModel(digest); model == nil || len(model.Sections) != 24 {
			b.Fatalf("unexpected model: %+v", model)
		}
	}
}

func typedEnumParam(id, name, current string, labels ...string) ParameterInfo {
	rows := make([]ParameterDisplayProbeLabel, 0, len(labels))
	for i, label := range labels {
		n := 0.0
		if len(labels) > 1 {
			n = float64(i) / float64(len(labels)-1)
		}
		rows = append(rows, ParameterDisplayProbeLabel{Index: i, Value: n, Label: label})
	}
	return ParameterInfo{ID: id, Name: name, ValueText: current, IsDiscrete: true, NumSteps: len(labels),
		DisplayProbe: &ParameterDisplayProbe{DiscreteLabels: rows}}
}

func typedContinuousParam(id, name, current string, min, max float64) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, ValueText: current, NormalizedValue: 0.5,
		DisplayDomainCandidate: eqRange(min, max, "linear")}
}

func addTwoBellBands(params []ParameterInfo) []ParameterInfo {
	for i := 1; i <= 2; i++ {
		prefix := "Band " + string(rune('0'+i))
		params = append(params,
			typedContinuousParam(prefix+"f", prefix+" Frequency", "1000 Hz", 20, 20000),
			typedContinuousParam(prefix+"g", prefix+" Gain", "0 dB", -12, 12),
			typedContinuousParam(prefix+"q", prefix+" Q", "1.0", 0.1, 10),
		)
	}
	return params
}

func findEQSection(t *testing.T, model *EQModel, key string) EQSection {
	t.Helper()
	if model == nil {
		t.Fatal("model is nil")
	}
	for _, section := range model.Sections {
		if section.Key == key {
			return section
		}
	}
	t.Fatalf("section %q not found in %+v", key, model.Sections)
	return EQSection{}
}

func findEQBand(t *testing.T, model *EQModel, key string) EQBand {
	t.Helper()
	if model == nil {
		t.Fatal("model is nil")
	}
	for _, band := range model.Bands {
		if band.Key == key {
			return band
		}
	}
	t.Fatalf("band %q not found in %+v", key, model.Bands)
	return EQBand{}
}

func TestDetectEQModelRecoversCompactRoleSuffixesFromDomainsAndSiblingStem(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("x7f", "X7F", "OFF", "OFF", "1.5 kHz", "3.3 kHz", "3.9 kHz", "8.2 kHz"),
		typedContinuousParam("x7g", "X7G", "0.0 dB", -18, 18),
		typedEnumParam("zetaf", "ZetaF", "OFF", "OFF", "220 Hz", "470 Hz", "1.2 kHz"),
		typedContinuousParam("zetag", "ZetaG", "0.0 dB", -18, 18),
		// A compact Q suffix with an In/Out enum has no physical Q evidence and
		// must remain unresolved.
		typedEnumParam("x7q", "X7Q", "Out", "Out", "In"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil || model.Classification != "fixed_slot_adjustable" || len(model.Bands) != 2 {
		t.Fatalf("compact model=%+v", model)
	}
	for _, key := range []string{"x7", "zeta"} {
		band := findEQBand(t, model, key)
		if !band.Complete || band.Active || band.ActivationStrategy != EQActivationImplicitInDomain {
			t.Fatalf("compact band %s=%+v", key, band)
		}
		if len(band.Bindings[eqRoleFrequency]) != 1 || len(band.Bindings[eqRoleGain]) != 1 {
			t.Fatalf("compact roles %s=%+v", key, band.Bindings)
		}
	}
	if len(findEQBand(t, model, "x7").Bindings[eqRoleQ]) != 0 {
		t.Fatal("In/Out toggle was misclassified as compact Q")
	}
}

func TestDetectEQModelRecoversCompactDedicatedCutsWithOffEnum(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("a1f", "A1F", "100 Hz", "100 Hz", "200 Hz"),
		typedContinuousParam("a1g", "A1G", "0.0 dB", -18, 18),
		typedEnumParam("b2f", "B2F", "1.0 kHz", "1.0 kHz", "2.0 kHz"),
		typedContinuousParam("b2g", "B2G", "0.0 dB", -18, 18),
		typedEnumParam("hp", "HPF", "OFF", "OFF", "27 Hz", "47 Hz", "82 Hz", "150 Hz"),
		typedEnumParam("lp", "LPF", "OFF", "3.9 kHz", "8.2 kHz", "18.0 kHz", "OFF"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	for _, test := range []struct {
		key  string
		kind EQFilterKind
	}{{"low cut", EQFilterLowCut}, {"high cut", EQFilterHighCut}} {
		section := findEQSection(t, model, test.key)
		if !section.Complete || section.DedicatedKind != test.kind ||
			section.Activation.Strategy != EQActivationImplicitInDomain || section.Activation.Active {
			t.Fatalf("compact cut %s=%+v", test.key, section)
		}
	}
}

func TestDetectEQModelRecoversNamedBellToggleWithoutInventingShelf(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("lf-f", "LFF", "100 Hz", "33 Hz", "100 Hz", "330 Hz"),
		typedContinuousParam("lf-g", "LFG", "0.0 dB", -18, 18),
		typedEnumParam("lf-bell", "LF Bell", "Out", "Out", "In"),
		typedEnumParam("hmf-f", "HMFF", "3.3 kHz", "1.5 kHz", "3.3 kHz", "8.2 kHz"),
		typedContinuousParam("hmf-g", "HMFG", "0.0 dB", -18, 18),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	section := findEQSection(t, model, "low")
	if !slices.Contains(section.ReachableKinds, EQFilterBell) || slices.Contains(section.ReachableKinds, EQFilterLowShelf) {
		t.Fatalf("named Bell topology=%+v", section)
	}
	bindings := section.Bindings[eqRoleFilterKind]
	if len(bindings) != 1 || len(bindings[0].Reachable) != 2 || bindings[0].Reachable[1].Kind != EQFilterBell {
		t.Fatalf("named Bell binding=%+v", bindings)
	}
}

func TestDetectEQModelRejectsUnpairedOrPhysicallyInvalidCompactSuffixes(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("fast", "DriveF", "Fast", "Fast", "Slow"),
		typedContinuousParam("level", "OutputG", "-6 dB", -18, 0),
		typedEnumParam("attack", "AttackQ", "Out", "Out", "In"),
		typedContinuousParam("orphan", "OrphanG", "0.0 dB", -18, 18),
	}
	if model := DetectEQModel(ParameterDigest{Parameters: params}); model != nil {
		t.Fatalf("invalid compact suffixes became EQ: %+v", model)
	}
}

func TestDetectEQModelDisambiguatesTypeIntoKindSlopeAndDesign(t *testing.T) {
	params := addTwoBellBands(nil)
	params = append(params,
		typedContinuousParam("hp-f", "HP Frequency", "96 Hz", 20, 20000),
		typedContinuousParam("hp-q", "HP Q", "0.7", 0.1, 10),
		typedEnumParam("hp-design", "HP Type", "Digital", "US Vint", "Digital"),
		typedContinuousParam("lp-f", "LP Frequency", "3501 Hz", 20, 20000),
		typedEnumParam("lp-slope", "LP Slope", "24 dB/oct", "6 dB/oct", "12 dB/oct", "24 dB/oct", "72 dB/oct"),
	)
	model := DetectEQModel(ParameterDigest{Parameters: params})
	lowCut := findEQSection(t, model, "low cut")
	if lowCut.DedicatedKind != EQFilterLowCut || len(lowCut.Bindings[eqRoleDesign]) != 1 || len(lowCut.Bindings[eqRoleFilterKind]) != 0 {
		t.Fatalf("H-EQ-like low cut=%+v", lowCut)
	}
	highCut := findEQSection(t, model, "high cut")
	if highCut.DedicatedKind != EQFilterHighCut || len(highCut.Bindings[eqRoleSlope]) != 1 {
		t.Fatalf("TDR-like high cut=%+v", highCut)
	}
}

func TestDetectEQModelRecoversImplicitOutActivationAndIgnoresAmbiguousType(t *testing.T) {
	params := addTwoBellBands(nil)
	params = append(params,
		typedContinuousParam("hp", "HP Frq", "Out Hz", 20, 20000),
		typedContinuousParam("lp", "LP Frq", "Out KHz", 20, 20000),
		typedEnumParam("lf-type", "LF Type", "Off", "Off", "On"),
	)
	model := DetectEQModel(ParameterDigest{Parameters: params})
	for _, key := range []string{"low cut", "high cut"} {
		section := findEQSection(t, model, key)
		if section.Activation.Strategy != EQActivationImplicitInDomain || section.Activation.Active {
			t.Fatalf("%s activation=%+v", key, section.Activation)
		}
	}
	for _, section := range model.Sections {
		if len(section.Bindings[eqRoleFilterKind]) > 0 && section.Key == "lf" {
			t.Fatalf("On/Off Type was guessed as filter kind: %+v", section)
		}
	}
}

func TestDetectEQModelRecoversContextualShelfAndGenericKindAliases(t *testing.T) {
	params := []ParameterInfo{
		typedContinuousParam("lf-f", "Low Frequency", "100 Hz", 20, 20000),
		typedContinuousParam("lf-g", "Low Gain", "0 dB", -12, 12),
		typedEnumParam("lf-t", "LF-Type", "Bell", "Bell", "Shelf"),
		typedContinuousParam("hf-f", "High Frequency", "8000 Hz", 20, 20000),
		typedContinuousParam("hf-g", "High Gain", "0 dB", -12, 12),
		typedEnumParam("hf-t", "HF-Type", "Shelf", "Bell", "Shelf"),
		typedContinuousParam("b1-f", "Band 1 Frequency", "1000 Hz", 20, 20000),
		typedContinuousParam("b1-g", "Band 1 Gain", "0 dB", -12, 12),
		typedEnumParam("b1-t", "Band 1 Type", "Bell", "Bell", "Low-Shelf", "Hi-Shelf", "Low-Pass", "Hi-Pass", "Notch"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	low := findEQSection(t, model, "low")
	if !slices.Contains(low.ReachableKinds, EQFilterLowShelf) || slices.Contains(low.ReachableKinds, EQFilterHighShelf) {
		t.Fatalf("low kinds=%v", low.ReachableKinds)
	}
	high := findEQSection(t, model, "high")
	if !slices.Contains(high.ReachableKinds, EQFilterHighShelf) || slices.Contains(high.ReachableKinds, EQFilterLowShelf) {
		t.Fatalf("high kinds=%v", high.ReachableKinds)
	}
	band := findEQSection(t, model, "1")
	for _, kind := range []EQFilterKind{EQFilterBell, EQFilterLowShelf, EQFilterHighShelf, EQFilterLowCut, EQFilterHighCut, EQFilterNotch} {
		if !slices.Contains(band.ReachableKinds, kind) {
			t.Fatalf("Q10-like band kinds=%v missing %s", band.ReachableKinds, kind)
		}
	}
}

func TestDetectEQModelKeepsFixedGraphicBandsAndAuxiliaryStereoCuts(t *testing.T) {
	params := []ParameterInfo{}
	for _, channel := range []string{"Left", "Right"} {
		for _, frequency := range []string{"100Hz", "1Khz", "10Khz"} {
			params = append(params, typedContinuousParam(channel+frequency, channel+" "+frequency, "0 dB", -12, 12))
		}
		short := string(channel[0])
		params = append(params,
			typedContinuousParam(short+"hp-f", short+" HiPass Frequency", "141 Hz", 20, 20000),
			typedEnumParam(short+"hp-on", short+" HiPass OnOff", "On", "Off", "On"),
			typedContinuousParam(short+"lp-f", short+" LoPass Frequency", "9.5 kHz", 20, 20000),
			typedEnumParam(short+"lp-on", short+" LoPass OnOff", "On", "Off", "On"),
		)
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil || model.Classification != "fixed_freq" || len(model.Bands) != 3 {
		t.Fatalf("graphic model=%+v", model)
	}
	for _, key := range []string{"low cut", "high cut"} {
		section := findEQSection(t, model, key)
		if section.Primary || len(section.Bindings[eqRoleFrequency]) != 2 || section.Activation.Strategy != EQActivationExplicit {
			t.Fatalf("auxiliary %s=%+v", key, section)
		}
	}
}

func TestDetectEQModelFailsClosedWhenAuxiliaryChannelBindingIsMissing(t *testing.T) {
	params := addTwoBellBands(nil)
	params = append(params,
		typedContinuousParam("lhp-f", "L HiPass Frequency", "120 Hz", 20, 20000),
		typedContinuousParam("rhp-f", "R HiPass Frequency", "120 Hz", 20, 20000),
		typedEnumParam("lhp-on", "L HiPass OnOff", "Off", "Off", "On"),
	)
	section := findEQSection(t, DetectEQModel(ParameterDigest{Parameters: params}), "low cut")
	if section.Complete || !slices.Contains(section.Issues, "channel bindings missing for role activation") {
		t.Fatalf("missing right activation must fail closed: %+v", section)
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
