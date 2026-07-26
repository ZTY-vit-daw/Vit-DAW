package plugingrabber

import (
	"fmt"
	"testing"
)

// Every fixture below is built from a live probe dump of the real plugin, and
// every one deliberately leaves Unit empty. Role detection must not consult it:
// FreeEQ8 reports no unit label on any of its 129 parameters and prints bare
// numbers, and Pro-Q 3 reports no unit label on any of its 348.

func eqRange(lo, hi float64, scale string) *PluginDisplayDomain {
	return &PluginDisplayDomain{Unit: "", Scale: scale, Min: &lo, Max: &hi, Confidence: 0.84}
}

func eqParam(id, name string, valueText string, domain *PluginDisplayDomain) ParameterInfo {
	return ParameterInfo{ID: id, Name: name, ValueText: valueText, DisplayDomainCandidate: domain}
}

// FreeEQ8: "Band N Freq" naming, zero unit labels, bare numeric value text.
// Previously classified fixed_freq, which made a 3400 Hz request move only the
// gain of the band resting at 4000 Hz while reporting success.
func eqFreeEQ8Digest() ParameterDigest {
	params := []ParameterInfo{
		eqParam("out", "Output Gain", "0.00", eqRange(-24, 24, "linear")),
	}
	freqs := []string{"80.000", "250.000", "500.000", "1000.000", "2000.000", "4000.000", "8000.000", "16000.000"}
	for band := 1; band <= 8; band++ {
		prefix := fmt.Sprintf("Band %d ", band)
		params = append(params,
			eqParam(fmt.Sprintf("%d_on", band), prefix+"On", "On", nil),
			eqParam(fmt.Sprintf("%d_type", band), prefix+"Type", "Bell", nil),
			eqParam(fmt.Sprintf("%d_slope", band), prefix+"Slope", "12 dB", eqRange(12, 48, "linear")),
			eqParam(fmt.Sprintf("%d_freq", band), prefix+"Freq", freqs[band-1], eqRange(20, 20000, "log")),
			eqParam(fmt.Sprintf("%d_q", band), prefix+"Q", "1.000", eqRange(0.1, 24, "log")),
			eqParam(fmt.Sprintf("%d_gain", band), prefix+"Gain", "0.00", eqRange(-24, 24, "linear")),
			eqParam(fmt.Sprintf("%d_drive", band), prefix+"Drive", "0.00", eqRange(0, 100, "linear")),
			eqParam(fmt.Sprintf("%d_thr", band), prefix+"Threshold", "0.00", eqRange(-60, 0, "linear")),
			eqParam(fmt.Sprintf("%d_ratio", band), prefix+"Ratio", "1.00", eqRange(1, 20, "linear")),
			eqParam(fmt.Sprintf("%d_atk", band), prefix+"Attack", "10.0", eqRange(0.1, 100, "log")),
			eqParam(fmt.Sprintf("%d_rel", band), prefix+"Release", "100.0", eqRange(1, 1000, "log")),
		)
	}
	return ParameterDigest{TemplateRole: "eq", Parameters: params}
}

func TestBuildEQBandSummaryFreeEQ8IsAdjustableWithoutAnyUnitLabel(t *testing.T) {
	summary := BuildEQBandSummary(eqFreeEQ8Digest())
	if summary == nil {
		t.Fatal("expected a summary for FreeEQ8, got nil")
	}
	if got := summary["eq_model"]; got != "fixed_slot_adjustable" {
		t.Fatalf("eq_model = %v, want fixed_slot_adjustable (no parameter carries a unit label, "+
			"so the range shape has to carry the decision)", got)
	}
	bands, _ := summary["bands"].([]map[string]any)
	if len(bands) != 8 {
		t.Fatalf("band count = %d, want 8", len(bands))
	}
	sixth := bands[5]
	if sixth["band"] != "B6" {
		t.Fatalf("bands[5].band = %v, want B6", sixth["band"])
	}
	// Freq must beat Release (1..1000) and Attack (0.1..100); Q must beat
	// Attack, which shares its 0.1 lower bound.
	if sixth["freq_param_id"] != "6_freq" {
		t.Errorf("B6 freq_param_id = %v, want 6_freq", sixth["freq_param_id"])
	}
	if sixth["gain_param_id"] != "6_gain" {
		t.Errorf("B6 gain_param_id = %v, want 6_gain", sixth["gain_param_id"])
	}
	if sixth["q_param_id"] != "6_q" {
		t.Errorf("B6 q_param_id = %v, want 6_q (Attack shares the 0.1 lower bound)", sixth["q_param_id"])
	}
	if _, leaked := sixth["fixed_freq_hz"]; leaked {
		t.Error("an adjustable band must not report fixed_freq_hz")
	}
	domain, _ := sixth["freq_domain"].(map[string]any)
	if domain["max"] != 20000.0 {
		t.Errorf("B6 freq_domain max = %v, want 20000 (the measured range, not a fallback)", domain["max"])
	}
}

// ZamEQ2: no "Band" token anywhere, and its bands are indexed by a mix of
// numbers and letters. All four frequencies are adjustable; only the two mids
// have a width control.
func eqZamEQ2Digest() ParameterDigest {
	return ParameterDigest{
		TemplateRole: "eq",
		Parameters: []ParameterInfo{
			eqParam("3", "Boost/Cut 1", "0.000000", eqRange(-20, 20, "linear")),
			eqParam("4", "Bandwidth 1", "1.000000", eqRange(0.7, 2.5, "linear")),
			eqParam("5", "Frequency 1", "1000.000000", eqRange(200, 2500, "log")),
			eqParam("6", "Boost/Cut 2", "0.000000", eqRange(-20, 20, "linear")),
			eqParam("7", "Bandwidth 2", "1.000000", eqRange(0.7, 2.5, "linear")),
			eqParam("8", "Frequency 2", "3000.000000", eqRange(600, 7000, "log")),
			eqParam("9", "Boost/Cut L", "0.000000", eqRange(-20, 20, "linear")),
			eqParam("10", "Frequency L", "250.000000", eqRange(40, 600, "log")),
			eqParam("11", "Boost/Cut H", "0.000000", eqRange(-20, 20, "linear")),
			eqParam("12", "Frequency H", "8000.000000", eqRange(1500, 22000, "log")),
			eqParam("13", "Output gain", "0.000000", eqRange(-10, 10, "linear")),
			eqParam("14", "Input gain", "0.000000", eqRange(-10, 10, "linear")),
		},
	}
}

func TestBuildEQBandSummaryZamEQ2GroupsMixedNumberAndLetterIndices(t *testing.T) {
	summary := BuildEQBandSummary(eqZamEQ2Digest())
	if summary == nil {
		t.Fatal("expected a summary for ZamEQ2, got nil (no parameter contains the token \"Band\")")
	}
	if got := summary["eq_model"]; got != "fixed_slot_adjustable" {
		t.Fatalf("eq_model = %v, want fixed_slot_adjustable — every ZamEQ2 frequency is adjustable "+
			"(shelves span 40..600 and 1500..22000)", got)
	}
	bands, _ := summary["bands"].([]map[string]any)
	if len(bands) != 4 {
		t.Fatalf("band count = %d, want 4; Output/Input gain must not become bands: %+v", len(bands), bands)
	}
	byBand := map[string]map[string]any{}
	for _, band := range bands {
		byBand[fmt.Sprint(band["band"])] = band
	}
	for _, name := range []string{"B1", "B2", "BL", "BH"} {
		band, ok := byBand[name]
		if !ok {
			t.Fatalf("band %s missing; got %v", name, byBand)
		}
		if band["freq_param_id"] == nil || band["gain_param_id"] == nil {
			t.Errorf("band %s must have both freq and gain: %+v", name, band)
		}
	}
	// "Boost/Cut" is named nothing like gain, and is identified by its range.
	if byBand["BL"]["gain_param_id"] != "9" {
		t.Errorf("BL gain_param_id = %v, want 9 (Boost/Cut L)", byBand["BL"]["gain_param_id"])
	}
	if byBand["B1"]["q_param_id"] != "4" {
		t.Errorf("B1 q_param_id = %v, want 4 (Bandwidth 1)", byBand["B1"]["q_param_id"])
	}
	if byBand["BL"]["q_param_id"] != nil {
		t.Errorf("BL is a shelf with no width control, got q_param_id %v", byBand["BL"]["q_param_id"])
	}
}

// ZamGEQ31: the band index *is* the centre frequency ("32Hz".."20801Hz"), and
// the parameter behind each label is a dB fader, not a frequency control.
func eqZamGEQ31Digest() ParameterDigest {
	hz := []int{32, 40, 50, 63, 79, 100, 126, 158, 200, 251, 316, 398, 501, 631, 794, 999,
		1257, 1584, 1997, 2514, 3165, 3986, 5017, 6318, 7963, 10032, 12662, 16081, 20801}
	params := []ParameterInfo{
		eqParam("3", "Master Gain", "0.000000", eqRange(-30, 30, "linear")),
	}
	for i, f := range hz {
		params = append(params,
			eqParam(fmt.Sprintf("%d", i+4), fmt.Sprintf("%dHz", f), "0.000000", eqRange(-12, 12, "linear")))
	}
	return ParameterDigest{TemplateRole: "eq", Parameters: params}
}

func TestBuildEQBandSummaryZamGEQ31TakesFrequenciesFromBandLabels(t *testing.T) {
	summary := BuildEQBandSummary(eqZamGEQ31Digest())
	if summary == nil {
		t.Fatal("expected a summary for ZamGEQ31, got nil")
	}
	if got := summary["eq_model"]; got != "fixed_freq" {
		t.Fatalf("eq_model = %v, want fixed_freq", got)
	}
	bands, _ := summary["bands"].([]map[string]any)
	if len(bands) != 29 {
		t.Fatalf("band count = %d, want 29; Master Gain must not become a band", len(bands))
	}
	first, last := bands[0], bands[len(bands)-1]
	if first["fixed_freq_hz"] != 32.0 || last["fixed_freq_hz"] != 20801.0 {
		t.Fatalf("frequency labels = %v..%v, want 32..20801", first["fixed_freq_hz"], last["fixed_freq_hz"])
	}
	// "32Hz" contains "Hz" but is a dB fader; it must be the gain, not a frequency.
	if first["gain_param_id"] != "4" {
		t.Errorf("first gain_param_id = %v, want 4", first["gain_param_id"])
	}
	if first["freq_param_id"] != nil {
		t.Errorf("a fixed-frequency band must expose no writable frequency, got %v", first["freq_param_id"])
	}
}

// An opaque graphic EQ (Marvel GEQ's shape, without the name that triggers its
// hardcoded frequency table). The structure is recoverable; the frequencies are
// not present in any machine-readable field, and must not be invented.
func TestBuildEQBandSummaryOpaqueGraphicEQReportsUnknownFrequencies(t *testing.T) {
	params := []ParameterInfo{}
	for i := 0; i < 16; i++ {
		params = append(params,
			eqParam(fmt.Sprintf("%d", i+1), fmt.Sprintf("1EQ%d", i), "0.0", eqRange(-12, 12, "linear")))
	}
	// Same dB range as the bands, but outside the index series.
	params = append(params, eqParam("17", "1OGain", "0.0", eqRange(-12, 12, "linear")))

	summary := BuildEQBandSummary(ParameterDigest{
		TemplateRole:   "eq",
		PluginIdentity: map[string]any{"plugin_name": "Generic Graphic EQ"},
		Parameters:     params,
	})
	if summary == nil {
		t.Fatal("expected the 16-slot structure to be recovered, got nil")
	}
	bands, _ := summary["bands"].([]map[string]any)
	if len(bands) != 16 {
		t.Fatalf("band count = %d, want 16; 1OGain shares the band dB range and is excluded only "+
			"by not belonging to the index series", len(bands))
	}
	for _, band := range bands {
		if _, ok := band["fixed_freq_hz"]; ok {
			t.Fatalf("band %v invented a centre frequency: %v", band["band"], band["fixed_freq_hz"])
		}
	}
	if summary["frequency_labels"] == nil {
		t.Error("summary must state that the frequency labels are unknown")
	}
}

func TestEQDecompositionPrefersTheWidestIndexSeries(t *testing.T) {
	// "1EQ0" can be read as "<n>EQ0" or "1EQ<n>"; only the second is a band series.
	family := detectEQBandFamily([]ParameterInfo{
		eqParam("1", "1EQ0", "0.0", eqRange(-12, 12, "linear")),
		eqParam("2", "1EQ1", "0.0", eqRange(-12, 12, "linear")),
		eqParam("3", "1EQ2", "0.0", eqRange(-12, 12, "linear")),
		eqParam("4", "2EQ0", "0.0", eqRange(-12, 12, "linear")),
		eqParam("5", "2EQ1", "0.0", eqRange(-12, 12, "linear")),
		eqParam("6", "2EQ2", "0.0", eqRange(-12, 12, "linear")),
	})
	if family == nil {
		t.Fatal("expected a family")
	}
	if len(family.Indices) != 3 {
		t.Fatalf("indices = %v, want 3 bands (0,1,2) rather than 2 channels", family.Indices)
	}
}

func TestEQRoleClassificationFromRangeShape(t *testing.T) {
	cases := []struct {
		name           string
		lo, hi         float64
		freq, gain, qq bool
	}{
		{"TDR Nova frequency", 10, 40000, true, false, false},
		{"ZamEQ2 shelf frequency", 40, 600, true, false, false},
		{"TDR Nova gain", -18, 18, false, true, false},
		{"ZamGEQ31 fader", -12, 12, false, true, false},
		{"Pro-Q 3 Q", 0.025, 40, false, false, true},
		{"ZamEQ2 bandwidth", 0.7, 2.5, false, false, true},
		{"FreeEQ8 slope", 12, 48, false, false, false},
		{"TDR Nova threshold", -50, 0, false, false, false},
		{"FreeEQ8 drive", 0, 100, false, false, false},
	}
	for _, tc := range cases {
		shape := eqShapeOfParam(eqParam("x", "x", "", eqRange(tc.lo, tc.hi, "linear")))
		if got := shape.looksLikeFreq(); got != tc.freq {
			t.Errorf("%s: looksLikeFreq = %v, want %v", tc.name, got, tc.freq)
		}
		if got := shape.looksLikeGain(); got != tc.gain {
			t.Errorf("%s: looksLikeGain = %v, want %v", tc.name, got, tc.gain)
		}
		if got := shape.looksLikeQ(); got != tc.qq {
			t.Errorf("%s: looksLikeQ = %v, want %v", tc.name, got, tc.qq)
		}
	}
}
