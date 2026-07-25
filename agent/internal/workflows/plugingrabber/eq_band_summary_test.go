package plugingrabber

import (
	"strings"
	"testing"
)

// Parameter shapes below are copied from live reads of the two plugins, so the
// test fails if the detection heuristic stops matching what the hosts report.

func eqSummaryTDRNovaDigest() ParameterDigest {
	hzMin := 10.0
	hzMax := 40000.0
	dbMin := -18.0
	dbMax := 18.0
	hzDomain := &PluginDisplayDomain{Unit: "Hz", Scale: "log", Min: &hzMin, Max: &hzMax}
	dbDomain := &PluginDisplayDomain{Unit: "dB", Scale: "linear", Min: &dbMin, Max: &dbMax}
	return ParameterDigest{
		TemplateRole: "eq",
		Parameters: []ParameterInfo{
			{ID: "48", Name: "Band 1 selected", ValueText: "On"},
			{ID: "50", Name: "Band 1 Gain", ValueText: "0.0", DisplayDomainCandidate: dbDomain},
			{ID: "51", Name: "Band 1 Q", ValueText: "0.40"},
			{ID: "52", Name: "Band 1 Frequency", ValueText: "80", DisplayDomainCandidate: hzDomain},
			{ID: "53", Name: "Band 1 Type", ValueText: "Bell"},
			{ID: "1571", Name: "Band 2 Gain", ValueText: "0.0", DisplayDomainCandidate: dbDomain},
			{ID: "1573", Name: "Band 2 Frequency", ValueText: "400", DisplayDomainCandidate: hzDomain},
			{ID: "1604", Name: "Band 3 Gain", ValueText: "0.0", DisplayDomainCandidate: dbDomain},
			{ID: "1606", Name: "Band 3 Frequency", ValueText: "2200", DisplayDomainCandidate: hzDomain},
			{ID: "1637", Name: "Band 4 Gain", ValueText: "0.0", DisplayDomainCandidate: dbDomain},
			{ID: "1660", Name: "Band 4 Frequency", ValueText: "6000", DisplayDomainCandidate: hzDomain},
			{ID: "1691", Name: "HP Frequency", ValueText: "15"},
		},
	}
}

func eqSummaryProQ3Digest() ParameterDigest {
	params := []ParameterInfo{}
	for _, band := range []struct {
		used, enabled, freq, gain, q, shape string
		number                              string
	}{
		{"0", "1", "2", "3", "7", "8", "1"},
		{"15", "16", "17", "18", "22", "23", "2"},
		{"30", "31", "32", "33", "37", "38", "3"},
	} {
		params = append(params,
			ParameterInfo{ID: band.used, Name: "Band " + band.number + " Used", ValueText: "Unused"},
			ParameterInfo{ID: band.enabled, Name: "Band " + band.number + " Enabled", ValueText: "Enabled"},
			ParameterInfo{ID: band.freq, Name: "Band " + band.number + " Frequency", ValueText: "1000.0 Hz"},
			ParameterInfo{ID: band.gain, Name: "Band " + band.number + " Gain", ValueText: "0.00 dB"},
			ParameterInfo{ID: band.q, Name: "Band " + band.number + " Q", ValueText: "1.000"},
			ParameterInfo{ID: band.shape, Name: "Band " + band.number + " Shape", ValueText: "Bell"},
		)
	}
	return ParameterDigest{TemplateRole: "eq", Parameters: params}
}

func TestBuildEQBandSummaryDetectsFixedSlotAdjustable(t *testing.T) {
	summary := BuildEQBandSummary(eqSummaryTDRNovaDigest())
	if summary == nil {
		t.Fatal("expected a summary for TDR Nova, got nil")
	}
	if got := summary["eq_model"]; got != "fixed_slot_adjustable" {
		t.Fatalf("eq_model = %v, want fixed_slot_adjustable", got)
	}
	bands, _ := summary["bands"].([]map[string]any)
	if len(bands) != 4 {
		t.Fatalf("band count = %d, want 4 (HP has no Band N prefix and must be excluded)", len(bands))
	}
	// B3 at 2200Hz is the band a 3.4 kHz request should land on — closest.
	third := bands[2]
	if third["band"] != "B3" {
		t.Errorf("bands[2].band = %v, want B3 (bands must be sorted numerically)", third["band"])
	}
	if third["freq_param_id"] != "1606" || third["gain_param_id"] != "1604" {
		t.Errorf("B3 param ids = freq %v / gain %v, want 1606 / 1604", third["freq_param_id"], third["gain_param_id"])
	}
	if hz, ok := third["current_freq_hz"].(float64); !ok || hz != 2200 {
		t.Errorf("B3 current_freq_hz = %v, want 2200", third["current_freq_hz"])
	}
	// freq_domain must be present so the LLM can compute normalized value
	if third["freq_domain"] == nil {
		t.Error("B3 must have freq_domain for unit conversion")
	}
	if _, ok := summary["available_slots"]; ok {
		t.Error("fixed_slot_adjustable summary must not advertise available_slots")
	}
	// Description must mention BOTH freq and gain writes
	desc := summary["how_to_pick_a_band"].(string)
	if !strings.Contains(desc, "freq_param_id") || !strings.Contains(desc, "gain_param_id") {
		t.Errorf("how_to_pick_a_band must instruct writing both freq and gain, got: %s", desc)
	}
}

func TestBuildEQBandSummaryDetectsFixedFreq(t *testing.T) {
	// Simulate a 4-band graphic EQ where frequency is fixed (no continuous Hz domain).
	summary := BuildEQBandSummary(ParameterDigest{
		TemplateRole: "eq",
		Parameters: []ParameterInfo{
			{ID: "1", Name: "Band 1 Gain", ValueText: "0.0"},
			{ID: "2", Name: "Band 1 Frequency", ValueText: "80", IsDiscrete: true},
			{ID: "3", Name: "Band 2 Gain", ValueText: "0.0"},
			{ID: "4", Name: "Band 2 Frequency", ValueText: "1000", IsDiscrete: true},
		},
	})
	if summary == nil {
		t.Fatal("expected a summary for fixed-freq EQ, got nil")
	}
	if got := summary["eq_model"]; got != "fixed_freq" {
		t.Fatalf("eq_model = %v, want fixed_freq (freq has no continuous Hz domain)", got)
	}
	bands, _ := summary["bands"].([]map[string]any)
	for _, b := range bands {
		if b["freq_param_id"] != nil {
			t.Errorf("band %v must not expose freq_param_id for fixed_freq EQ", b["band"])
		}
		if b["gain_param_id"] == nil {
			t.Errorf("band %v must have gain_param_id", b["band"])
		}
	}
	desc := summary["how_to_pick_a_band"].(string)
	if !strings.Contains(desc, "ONLY gain_param_id") {
		t.Errorf("fixed_freq description must say 'ONLY gain_param_id', got: %s", desc)
	}
}

func TestBuildEQBandSummaryDetectsFreeFloating(t *testing.T) {
	summary := BuildEQBandSummary(eqSummaryProQ3Digest())
	if summary == nil {
		t.Fatal("expected a summary for Pro-Q 3, got nil")
	}
	if got := summary["eq_model"]; got != "free_floating" {
		t.Fatalf("eq_model = %v, want free_floating", got)
	}
	if got := summary["active_band_count"]; got != 0 {
		t.Errorf("active_band_count = %v, want 0 (every slot reports Unused)", got)
	}
	slots, _ := summary["available_slots"].([]map[string]any)
	if len(slots) != 3 {
		t.Fatalf("available_slots = %d, want 3", len(slots))
	}
	first := slots[0]
	for _, key := range []string{"used_param_id", "freq_param_id", "gain_param_id", "shape_param_id"} {
		if first[key] == nil {
			t.Errorf("available_slots[0] missing %s; the create sequence would be incomplete", key)
		}
	}
	if summary["create_sequence_notes"] == nil {
		t.Error("free_floating summary must explain the create sequence")
	}
}

func TestBuildEQBandSummarySkipsNonEQ(t *testing.T) {
	digest := ParameterDigest{
		TemplateRole: "comp",
		Parameters: []ParameterInfo{
			{ID: "1", Name: "Threshold", ValueText: "-18.0 dB"},
			{ID: "2", Name: "Ratio", ValueText: "4.0:1"},
		},
	}
	if summary := BuildEQBandSummary(digest); summary != nil {
		t.Fatalf("expected nil for a compressor, got %v", summary)
	}
}

func TestBuildEQBandSummarySkipsEQWithoutBandParameters(t *testing.T) {
	digest := ParameterDigest{
		TemplateRole: "eq",
		Parameters: []ParameterInfo{
			{ID: "1", Name: "Low Shelf Gain", ValueText: "0.0 dB"},
			{ID: "2", Name: "High Shelf Gain", ValueText: "0.0 dB"},
		},
	}
	if summary := BuildEQBandSummary(digest); summary != nil {
		t.Fatalf("expected nil when no Band N parameters exist, got %v", summary)
	}
}

func TestParseDisplayHz(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"80", 80, true},
		{"3400", 3400, true},
		{"1000.0 Hz", 1000, true},
		{"3.40 kHz", 3400, true},
		{"15 Hz", 15, true},
		{"", 0, false},
		{"Bell", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseDisplayHz(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseDisplayHz(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseDisplayDB(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"0.0", 0, true},
		{"-3.0", -3, true},
		{"0.00 dB", 0, true},
		{"+6.00 dB", 6, true},
		{"", 0, false},
		{"Auto", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseDisplayDB(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseDisplayDB(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
