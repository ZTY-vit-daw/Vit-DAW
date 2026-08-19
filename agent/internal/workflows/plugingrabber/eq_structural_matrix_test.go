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

func TestEQCurveFromProbeKeepsDecreasingActiveSuffixAfterSentinel(t *testing.T) {
	param := ParameterInfo{ID: "lp", Name: "LP Frequency", ValueText: "Out KHz",
		DisplayProbe: &ParameterDisplayProbe{Samples: []ParameterDisplayProbeSample{
			{NormalizedValue: 0, Text: "Out KHz"},
			{NormalizedValue: 0.25, Text: "10.7 KHz"},
			{NormalizedValue: 0.5, Text: "5.0 KHz"},
			{NormalizedValue: 0.75, Text: "3.7 KHz"},
			{NormalizedValue: 1, Text: "3.0 KHz"},
		}}}
	curve := eqCurveFromProbe(param)
	want := [][2]float64{{0.25, 10700}, {0.5, 5000}, {0.75, 3700}, {1, 3000}}
	if !slices.Equal(curve, want) {
		t.Fatalf("sentinel-trimmed curve=%v, want %v", curve, want)
	}
}

func TestParseEQFrequencyTextSupportsMixedHzAndBareKDisplays(t *testing.T) {
	tests := map[string]float64{
		"20": 20, "115 Hz": 115, "663": 663, "3.82k": 3820, "22.00K": 22000, "12 kHz": 12000,
		"1k0": 1000, "2k7": 2700, "3k3 Hz": 3300, "0,5 kHz": 500,
	}
	for text, want := range tests {
		got, ok := ParseEQFrequencyText(text)
		if !ok || got != want {
			t.Fatalf("ParseEQFrequencyText(%q)=(%g,%v), want (%g,true)", text, got, ok, want)
		}
	}
}

func TestParseEQLocalizedNumberSupportsDecimalComma(t *testing.T) {
	for text, want := range map[string]float64{"0,5": 0.5, "-3,0 dB": -3, "+1.25": 1.25} {
		got, ok := ParseEQLocalizedNumber(text)
		if !ok || got != want {
			t.Fatalf("ParseEQLocalizedNumber(%q)=(%g,%v), want (%g,true)", text, got, ok, want)
		}
	}
}

func TestDetectEQModelUnifiesAbbreviatedSectionsAndLocalizedEnumValues(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("hm-f", "High Mid Frequency 1", "2k7", "560", "1k0", "2k7", "3k3", "3k9"),
		typedEnumParam("hm-g", "HM Boost 1", "0.0", "-8.0", "-3.0", "0.0", "3.0", "8.0"),
		typedEnumParam("hm-q", "HM Bandwidth 1", "1", "0,5", "0,7", "1", "1,5", "Shelf"),
		typedEnumParam("hm-on", "High Mid Band On 1", "On", "Off", "On"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	section := findEQSection(t, model, "high mid 1")
	if !section.Complete || len(section.Bindings[eqRoleFrequency]) != 1 ||
		len(section.Bindings[eqRoleGain]) != 1 || len(section.Bindings[eqRoleQ]) != 1 {
		t.Fatalf("abbreviated/localized section was not unified: %+v", section)
	}
	qValues := section.Bindings[eqRoleQ][0].Reachable
	if len(qValues) < 1 || qValues[0].Physical == nil || *qValues[0].Physical != 0.5 {
		t.Fatalf("localized Q values=%+v", qValues)
	}
	frequencyValues := section.Bindings[eqRoleFrequency][0].Reachable
	if len(frequencyValues) < 4 || frequencyValues[3].Physical == nil || *frequencyValues[3].Physical != 3300 {
		t.Fatalf("engineering frequency values=%+v", frequencyValues)
	}
}

func TestDetectEQModelExcludesCompressorHighpassFromStaticEQSurface(t *testing.T) {
	params := []ParameterInfo{
		typedContinuousParam("comp-hpf", "Comp Highpass", "80 Hz", 20, 2000),
		typedContinuousParam("eq-low-f", "EQ Low Frequency", "120 Hz", 20, 2000),
		typedContinuousParam("eq-low-g", "EQ Low Gain", "0 dB", -18, 18),
		typedEnumParam("eq-low-shape", "EQ Low Bell", "Shelf", "Bell", "Shelf"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil {
		t.Fatal("expected main EQ surface")
	}
	compFilter := findEQSection(t, model, "comp low cut")
	if capability := findEQShapeCapability(t, compFilter.ShapeCapabilities, EQFilterLowCut); capability.Upsert ||
		!slices.Contains(compFilter.ExclusionCodes, "sidechain_or_detector_section_excluded") {
		t.Fatalf("compressor detector filter remained a static EQ control: %+v", compFilter)
	}
	mainEQ := findEQSection(t, model, "low")
	if capability := findEQShapeCapability(t, mainEQ.ShapeCapabilities, EQFilterBell); !capability.Upsert ||
		slices.Contains(mainEQ.ExclusionCodes, "sidechain_or_detector_section_excluded") {
		t.Fatalf("main EQ was incorrectly excluded with compressor filter: %+v", mainEQ)
	}
}

func TestEQCurveFromProbeDoesNotInventCurveFromTooFewActiveSamples(t *testing.T) {
	param := ParameterInfo{DisplayProbe: &ParameterDisplayProbe{Samples: []ParameterDisplayProbeSample{
		{NormalizedValue: 0, Text: "Out Hz"},
		{NormalizedValue: 0.5, Text: "80 Hz"},
		{NormalizedValue: 1, Text: "160 Hz"},
	}}}
	if curve := eqCurveFromProbe(param); curve != nil {
		t.Fatalf("two active samples cannot prove a response curve: %v", curve)
	}
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

func TestDetectEQModelRecoversNumberedCompoundCutWithSlopePropertySentinel(t *testing.T) {
	for _, test := range []struct {
		current string
		active  bool
	}{{"Off", false}, {"12 dB", true}} {
		params := []ParameterInfo{
			typedEnumParam("hpf", "High-pass 1 Frequency", "20", "20", "115", "663", "3.82k", "22.00k"),
			typedEnumParam("hps", "High-pass 1 Slope", test.current, "Off", "6 dB", "12 dB"),
		}
		model := DetectEQModel(ParameterDigest{Parameters: params})
		section := findEQSection(t, model, "low cut 1")
		if section.DedicatedKind != EQFilterLowCut || !section.Complete ||
			section.Activation.Strategy != EQActivationPropertySentinel ||
			section.Activation.BindingRole != eqRoleSlope || section.Activation.Active != test.active {
			t.Fatalf("current=%q compound property-sentinel section=%+v", test.current, section)
		}
		capability := findEQShapeCapability(t, section.ShapeCapabilities, EQFilterLowCut)
		if !capability.Upsert || !capability.Modify || !capability.Disable {
			t.Fatalf("current=%q cut capability=%+v", test.current, capability)
		}
	}
}

func TestDetectEQModelProjectsQualityByLocalPhysicalEvidence(t *testing.T) {
	qMin, qMax := 0.3, 15.0
	params := []ParameterInfo{
		typedContinuousParam("mf-f", "1 Mf Frequency", "3.15k", 20, 26000),
		typedContinuousParam("mf-g", "1 Mf Gain", "0.0", -12, 12),
		{ID: "mf-quality", Name: "1 Mf Quality", ValueText: "0.5", NormalizedValue: 0.13,
			DisplayDomainCandidate: eqRange(qMin, qMax, "log")},
		typedEnumParam("mf-on", "1 Mf On/Off", "On", "Off", "On"),
		typedContinuousParam("hp-f", "1 HP Frequency", "80", 20, 26000),
		typedEnumParam("hp-quality", "1 HP Quality", "18 dB/o", "0.3", "6.0", "6 dB/o", "12 dB/o", "18 dB/o"),
		typedEnumParam("hp-on", "1 HP On/Off", "On", "Off", "On"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	bell := findEQSection(t, model, "1 mf")
	if len(bell.Bindings[eqRoleQ]) != 1 || bell.Bindings[eqRoleQ][0].ParamID != "mf-quality" {
		t.Fatalf("numeric Quality was not projected to Q: %+v", bell)
	}
	cut := findEQSection(t, model, "1 low cut")
	if len(cut.Bindings[eqRoleSlope]) != 1 || cut.Bindings[eqRoleSlope][0].ParamID != "hp-quality" {
		t.Fatalf("Cut Quality was not projected to Slope: %+v", cut)
	}
	for _, value := range cut.Bindings[eqRoleSlope][0].Reachable {
		if value.Physical != nil && !textLooksLikeSlopeUnit(value.Label) {
			t.Fatalf("non-slope Quality label gained slope physical value: %+v", value)
		}
	}
}

func TestDetectEQModelRecoversFrequencyMultiplierBinding(t *testing.T) {
	params := []ParameterInfo{
		typedContinuousParam("hmf-f", "High-Mid Frequency 1", "1425", 250, 2500),
		typedEnumParam("hmf-x10", "High-Mid Frequency X10 1", "Off", "Off", "On"),
		typedContinuousParam("hmf-q", "High-Mid Q 1", "1.0", 0.4, 4),
		typedContinuousParam("hmf-g", "High-Mid Gain 1", "0.0", -20, 20),
		typedEnumParam("hmf-on", "High-Mid In/Out 1", "In", "Out", "In"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	section := findEQSection(t, model, "high mid 1")
	bindings := section.Bindings[eqRoleFrequencyTransform]
	if len(bindings) != 1 || bindings[0].TransformFactor != 10 || bindings[0].CurrentPhysical == nil || *bindings[0].CurrentPhysical != 1 {
		t.Fatalf("frequency multiplier binding=%+v section=%+v", bindings, section)
	}
	if len(bindings[0].Reachable) != 2 || bindings[0].Reachable[0].Physical == nil || *bindings[0].Reachable[0].Physical != 1 ||
		bindings[0].Reachable[1].Physical == nil || *bindings[0].Reachable[1].Physical != 10 {
		t.Fatalf("frequency multiplier reachable values=%+v", bindings[0].Reachable)
	}
}

func TestDetectEQModelRecoversDirectionalShelfAlternativeFromBellSelector(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("lf-f", "LFF", "100 Hz", "33 Hz", "100 Hz", "330 Hz"),
		typedContinuousParam("lf-g", "LFG", "0.0 dB", -18, 18),
		typedEnumParam("lf-bell", "LF Bell", "Out", "Out", "In"),
		typedEnumParam("hmf-f", "HMFF", "3.3 kHz", "1.5 kHz", "3.3 kHz", "8.2 kHz"),
		typedContinuousParam("hmf-g", "HMFG", "0.0 dB", -18, 18),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	section := findEQSection(t, model, "low")
	if !slices.Contains(section.ReachableKinds, EQFilterBell) || !slices.Contains(section.ReachableKinds, EQFilterLowShelf) {
		t.Fatalf("named Bell topology=%+v", section)
	}
	bindings := section.Bindings[eqRoleFilterKind]
	if len(bindings) != 1 || len(bindings[0].Reachable) != 2 || bindings[0].Reachable[0].Kind != EQFilterLowShelf ||
		bindings[0].Reachable[1].Kind != EQFilterBell {
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

func TestDetectEQModelRecognizesFilterOnlySurfaceWithoutGainBands(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("hp", "High Pass Frequency", "80 Hz", "Off", "40 Hz", "80 Hz", "160 Hz"),
		typedEnumParam("lp", "Low Pass Frequency", "12 kHz", "4 kHz", "8 kHz", "12 kHz", "Off"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil || len(model.Sections) != 2 {
		t.Fatalf("filter-only model=%+v", model)
	}
	for _, test := range []struct {
		key   string
		shape EQFilterKind
	}{{"low cut", EQFilterLowCut}, {"high cut", EQFilterHighCut}} {
		section := findEQSection(t, model, test.key)
		capability := findEQShapeCapability(t, section.ShapeCapabilities, test.shape)
		if !section.Complete || !capability.Upsert || !capability.Modify || !capability.Disable {
			t.Fatalf("filter-only %s capability=%+v section=%+v", test.shape, capability, section)
		}
	}
}

func TestDetectEQModelRejectsCoupledGainLocallyButKeepsIndependentCut(t *testing.T) {
	params := []ParameterInfo{
		typedEnumParam("f1", "Band 1 Frequency", "100 Hz", "50 Hz", "100 Hz", "200 Hz"),
		typedContinuousParam("boost", "Band 1 Boost", "2 dB", 0, 12),
		typedContinuousParam("atten", "Band 1 Attenuate", "0 dB", 0, 12),
		typedEnumParam("hp", "High Pass Frequency", "80 Hz", "Off", "40 Hz", "80 Hz", "160 Hz"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	if model == nil {
		t.Fatal("local coupled section must not reject the whole plugin")
	}
	bell := findEQShapeCapability(t, findEQSection(t, model, "1").ShapeCapabilities, EQFilterBell)
	if bell.Upsert || !slices.Contains(bell.RejectionCodes["upsert"], "gain_law_not_arbitrary_bipolar") {
		t.Fatalf("coupled Bell capability=%+v", bell)
	}
	cut := findEQShapeCapability(t, findEQSection(t, model, "low cut").ShapeCapabilities, EQFilterLowCut)
	if !cut.Upsert {
		t.Fatalf("independent static cut was blocked by another section: %+v", cut)
	}
}

func TestDetectEQModelKeepsStaticCoreWhenBandHasDynamicAuxiliaryParameters(t *testing.T) {
	params := []ParameterInfo{
		typedContinuousParam("f1", "Band 1 Frequency", "1000 Hz", 20, 20000),
		typedContinuousParam("g1", "Band 1 Gain", "0 dB", -12, 12),
		typedContinuousParam("q1", "Band 1 Q", "1.0", 0.1, 10),
		typedContinuousParam("t1", "Band 1 Threshold", "-20 dB", -80, 0),
		typedContinuousParam("r1", "Band 1 Dynamic Range", "0 dB", -12, 12),
		typedEnumParam("sc1", "Band 1 External Sidechain", "Off", "Off", "On"),
		typedEnumParam("hp", "High Pass Frequency", "80 Hz", "Off", "40 Hz", "80 Hz", "160 Hz"),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	bellSection := findEQSection(t, model, "1")
	bell := findEQShapeCapability(t, bellSection.ShapeCapabilities, EQFilterBell)
	if !bell.Upsert || slices.Contains(bellSection.ExclusionCodes, "dynamic_section_excluded") ||
		slices.Contains(bellSection.ExclusionCodes, "sidechain_or_detector_section_excluded") {
		t.Fatalf("static core was polluted by auxiliary siblings: capability=%+v section=%+v", bell, bellSection)
	}
	cut := findEQShapeCapability(t, findEQSection(t, model, "low cut").ShapeCapabilities, EQFilterLowCut)
	if !cut.Upsert {
		t.Fatalf("independent cut must remain available: %+v", cut)
	}
}

func TestDetectEQModelStillExcludesOwnedDynamicAndSidechainComponents(t *testing.T) {
	params := []ParameterInfo{
		typedContinuousParam("main-f", "Band 1 Frequency", "1000 Hz", 20, 20000),
		typedContinuousParam("main-g", "Band 1 Gain", "0 dB", -12, 12),
		typedContinuousParam("de-f", "DeEsser Frequency", "6000 Hz", 1000, 16000),
		typedContinuousParam("de-g", "DeEsser Gain", "0 dB", -12, 12),
		typedContinuousParam("sc-f", "SC Bell Frequency", "120 Hz", 20, 20000),
		typedContinuousParam("sc-g", "SC Bell Gain", "0 dB", -12, 12),
	}
	model := DetectEQModel(ParameterDigest{Parameters: params})
	main := findEQShapeCapability(t, findEQSection(t, model, "1").ShapeCapabilities, EQFilterBell)
	if !main.Upsert {
		t.Fatalf("main static EQ section was excluded: %+v", main)
	}
	deEsser := findEQSection(t, model, "de esser")
	deEsserBell := findEQShapeCapability(t, deEsser.ShapeCapabilities, EQFilterBell)
	if deEsserBell.Upsert || !slices.Contains(deEsser.ExclusionCodes, "dynamic_section_excluded") {
		t.Fatalf("DeEsser component was not excluded: section=%+v capability=%+v", deEsser, deEsserBell)
	}
	sidechain := findEQSection(t, model, "sc bell")
	sidechainBell := findEQShapeCapability(t, sidechain.ShapeCapabilities, EQFilterBell)
	if sidechainBell.Upsert || !slices.Contains(sidechain.ExclusionCodes, "sidechain_or_detector_section_excluded") {
		t.Fatalf("sidechain component was not excluded: section=%+v capability=%+v", sidechain, sidechainBell)
	}
}

func TestEQTopologyGenerationIgnoresIdentityAndCurrentValues(t *testing.T) {
	makeDigest := func(identity string, freqText, gainText string, normalized float64) ParameterDigest {
		params := []ParameterInfo{
			typedContinuousParam("f1", "Band 1 Frequency", freqText, 20, 20000),
			typedContinuousParam("g1", "Band 1 Gain", gainText, -12, 12),
			typedContinuousParam("q1", "Band 1 Q", "1.0", 0.1, 10),
			typedContinuousParam("f2", "Band 2 Frequency", "4000 Hz", 20, 20000),
			typedContinuousParam("g2", "Band 2 Gain", "0 dB", -12, 12),
			typedContinuousParam("q2", "Band 2 Q", "1.0", 0.1, 10),
		}
		params[0].NormalizedValue = normalized
		return ParameterDigest{PluginIdentity: map[string]any{"plugin_name": identity}, Parameters: params}
	}
	one := DetectEQModel(makeDigest("Identity A", "1000 Hz", "0 dB", 0.4))
	two := DetectEQModel(makeDigest("Identity B", "1200 Hz", "-3 dB", 0.5))
	if one == nil || two == nil || one.ControlTopology.Generation == "" ||
		one.ControlTopology.Generation != two.ControlTopology.Generation {
		t.Fatalf("topology generations must be identity/value independent: one=%+v two=%+v", one, two)
	}
}

func findEQShapeCapability(t *testing.T, capabilities []EQShapeCapability, shape EQFilterKind) EQShapeCapability {
	t.Helper()
	for _, capability := range capabilities {
		if capability.Shape == shape {
			return capability
		}
	}
	t.Fatalf("shape capability %s not found in %+v", shape, capabilities)
	return EQShapeCapability{}
}
