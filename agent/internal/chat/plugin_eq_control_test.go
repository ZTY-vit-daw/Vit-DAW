package chat

import (
	"math"
	"testing"
)

// Curves below are the literal five-point probe readings from the plugins, so
// this test fails if the inversion stops matching what a real plugin does.
func TestEQNormalizedFromMeasuredCurves(t *testing.T) {
	cases := []struct {
		name   string
		curve  [][2]float64
		target float64
		// wantNormalized is derived from the plugin's own response, not from the
		// inversion under test: it is the n that the curve's closed form maps to
		// target.
		wantNormalized float64
	}{
		{
			// v = 10 · 4000^n
			name:           "TDR Nova frequency (logarithmic)",
			curve:          [][2]float64{{0, 10}, {0.25, 80}, {0.5, 632}, {0.75, 5000}, {1, 40000}},
			target:         3400,
			wantNormalized: math.Log(340) / math.Log(4000),
		},
		{
			// v = 20 + 19980·n²  — declares no unit at all
			name:           "FreeEQ8 frequency (quadratic)",
			curve:          [][2]float64{{0, 20}, {0.25, 1268.750}, {0.5, 5015}, {0.75, 11258.751}, {1, 20000}},
			target:         3400,
			wantNormalized: math.Sqrt(3380.0 / 19980.0),
		},
		{
			// v = 600 + 6400·n — declares "Hz", which would imply logarithmic
			name:           "ZamEQ2 frequency 2 (linear)",
			curve:          [][2]float64{{0, 600}, {0.25, 2200}, {0.5, 3800}, {0.75, 5400}, {1, 7000}},
			target:         3400,
			wantNormalized: 2800.0 / 6400.0,
		},
		{
			// v = 10 · 3000^n
			name:           "Pro-Q 3 frequency (logarithmic)",
			curve:          [][2]float64{{0, 10}, {0.25, 74.008}, {0.5, 547.72}, {0.75, 4053.6}, {1, 30000}},
			target:         3400,
			wantNormalized: math.Log(340) / math.Log(3000),
		},
		{
			name:           "TDR Nova gain (linear through zero)",
			curve:          [][2]float64{{0, -18}, {0.25, -9}, {0.5, 0}, {0.75, 9}, {1, 18}},
			target:         -3,
			wantNormalized: 15.0 / 36.0,
		},
		{
			// v = 0.1 + 23.9·n²
			name:           "FreeEQ8 Q (quadratic)",
			curve:          [][2]float64{{0, 0.1}, {0.25, 1.594}, {0.5, 6.075}, {0.75, 13.544}, {1, 24}},
			target:         6.075,
			wantNormalized: 0.5,
		},
		{
			// v = 0.1 · 60^n
			name:           "TDR Nova Q (logarithmic)",
			curve:          [][2]float64{{0, 0.10}, {0.25, 0.28}, {0.5, 0.77}, {0.75, 2.16}, {1, 6.00}},
			target:         0.77,
			wantNormalized: 0.5,
		},
	}
	for _, tc := range cases {
		got, ok := eqNormalizedFromCurve(tc.target, tc.curve)
		if !ok {
			t.Errorf("%s: inversion refused a strictly increasing curve", tc.name)
			continue
		}
		if math.Abs(got-tc.wantNormalized) > 0.01 {
			t.Errorf("%s: normalized = %.5f, want %.5f (a %.1f%% position error)",
				tc.name, got, tc.wantNormalized, math.Abs(got-tc.wantNormalized)*100)
		}
	}
}

func TestEQNormalizedFromCurveClampsAndRefuses(t *testing.T) {
	curve := [][2]float64{{0, 20}, {0.5, 5015}, {1, 20000}}
	if got, ok := eqNormalizedFromCurve(5, curve); !ok || got != 0 {
		t.Errorf("below-range target = %v, %v; want 0, true", got, ok)
	}
	if got, ok := eqNormalizedFromCurve(99999, curve); !ok || got != 1 {
		t.Errorf("above-range target = %v, %v; want 1, true", got, ok)
	}
	// Too few points, and non-monotonic input, must fall back rather than guess.
	if _, ok := eqNormalizedFromCurve(100, [][2]float64{{0, 20}, {1, 20000}}); ok {
		t.Error("a two-point curve must be refused")
	}
}

// A plugin whose response matches neither family must still produce a usable
// answer rather than a wild one: the better-fitting of the two is used, and the
// result has to stay inside the bracketing samples.
func TestEQNormalizedFromCurveStaysBracketedOnAnOddCurve(t *testing.T) {
	curve := [][2]float64{{0, 100}, {0.25, 900}, {0.5, 1000}, {0.75, 1100}, {1, 5000}}
	got, ok := eqNormalizedFromCurve(1000, curve)
	if !ok {
		t.Fatal("expected an answer for a strictly increasing curve")
	}
	if got < 0.25 || got > 0.75 {
		t.Errorf("normalized = %.3f for the midpoint sample, want within [0.25, 0.75]", got)
	}
}

func TestEQNormalizedFromCurveSupportsDecreasingPhysicalCurve(t *testing.T) {
	curve := [][2]float64{{0, 3.5}, {0.25, 2.25}, {0.5, 1.5}, {0.75, 0.88}, {1, 0.1}}
	got, ok := eqNormalizedFromCurve(0.5, curve)
	if !ok || got < 0.85 || got > 0.95 {
		t.Fatalf("decreasing Q curve normalized=%v ok=%v, want about 0.9", got, ok)
	}
}

func TestEQWritePlanWritesAllChannelsAndRejectsMissingQ(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_freq", "set_eq_point_supported": true,
		"completeness": map[string]any{"complete": true},
		"bands": []map[string]any{{
			"band": "B3400", "fixed_freq_hz": 3400.0,
			"gain_bindings": []map[string]any{
				{"param_id": "left", "channel": "left", "domain": map[string]any{"min": -6.0, "max": 6.0}},
				{"param_id": "right", "channel": "right", "domain": map[string]any{"min": -6.0, "max": 6.0}},
			},
		}},
	}
	writes, _, err := eqBandWritePlan(summary, 3400, -3, nil)
	if err != nil || len(writes) != 2 || writes[0].Channel != "left" || writes[1].Channel != "right" {
		t.Fatalf("multi-channel writes=%+v err=%v", writes, err)
	}
	q := 0.5
	if _, _, err := eqBandWritePlan(summary, 3400, -3, &q); err == nil {
		t.Fatal("fixed band without Q must reject a Q request before writing")
	}
}

func TestEQFreeFloatingActivationIsLast(t *testing.T) {
	summary := map[string]any{
		"eq_model": "free_floating", "set_eq_point_supported": true,
		"completeness": map[string]any{"complete": true}, "active_bands": []map[string]any{},
		"available_slots": []map[string]any{{
			"band": "B1", "active": false,
			"frequency_bindings": []map[string]any{{"param_id": "freq", "channel": "shared"}},
			"gain_bindings":      []map[string]any{{"param_id": "gain", "channel": "shared"}},
			"activation_bindings": []map[string]any{{"param_id": "used", "channel": "shared",
				"reachable_values": []map[string]any{{"normalized": 0.0, "label": "Unused"}, {"normalized": 1.0, "label": "Used"}}}},
		}},
	}
	writes, _, err := eqBandWritePlan(summary, 3400, -3, nil)
	if err != nil || len(writes) != 3 || writes[len(writes)-1].Role != "used" {
		t.Fatalf("free-floating writes=%+v err=%v", writes, err)
	}
	if writes[len(writes)-1].NormalizedValue != 1 {
		t.Fatalf("activation selected normalized=%v, want Used=1 rather than Unused=0", writes[len(writes)-1].NormalizedValue)
	}
}

func TestEQFixedSlotSelectsInactiveEnumBandAndQuantizesFrequency(t *testing.T) {
	frequencyBinding := func(id string, values ...map[string]any) []map[string]any {
		return []map[string]any{{"param_id": id, "channel": "shared", "reachable_values": values}}
	}
	gainBinding := func(id string) []map[string]any {
		return []map[string]any{{"param_id": id, "channel": "shared", "domain": map[string]any{"min": -18.0, "max": 18.0}}}
	}
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "set_eq_point_supported": true,
		"completeness": map[string]any{"complete": true},
		"bands": []map[string]any{
			{"band": "lf", "active": false, "activation_strategy": "implicit_in_domain",
				"frequency_bindings": frequencyBinding("LFF",
					map[string]any{"normalized": 0.0, "label": "OFF"},
					map[string]any{"normalized": 0.5, "physical": 100.0, "label": "100 Hz"},
					map[string]any{"normalized": 1.0, "physical": 330.0, "label": "330 Hz"}),
				"gain_bindings": gainBinding("LFG")},
			{"band": "lmf", "active": false, "activation_strategy": "implicit_in_domain",
				"frequency_bindings": frequencyBinding("LMFF",
					map[string]any{"normalized": 0.0, "label": "OFF"},
					map[string]any{"normalized": 0.5, "physical": 820.0, "label": "820 Hz"},
					map[string]any{"normalized": 1.0, "physical": 1500.0, "label": "1.5 kHz"}),
				"gain_bindings": gainBinding("LMFG")},
			{"band": "hmf", "active": false, "activation_strategy": "implicit_in_domain",
				"frequency_bindings": frequencyBinding("HMFF",
					map[string]any{"normalized": 0.0, "label": "OFF"},
					map[string]any{"normalized": 0.25, "physical": 1500.0, "label": "1.5 kHz"},
					map[string]any{"normalized": 0.5, "physical": 3300.0, "label": "3.3 kHz"},
					map[string]any{"normalized": 0.75, "physical": 3900.0, "label": "3.9 kHz"},
					map[string]any{"normalized": 1.0, "physical": 8200.0, "label": "8.2 kHz"}),
				"gain_bindings": gainBinding("HMFG")},
			{"band": "hf", "active": false, "activation_strategy": "implicit_in_domain",
				"frequency_bindings": frequencyBinding("HFF",
					map[string]any{"normalized": 0.0, "label": "OFF"},
					map[string]any{"normalized": 0.5, "physical": 5000.0, "label": "5 kHz"},
					map[string]any{"normalized": 1.0, "physical": 15000.0, "label": "15 kHz"}),
				"gain_bindings": gainBinding("HFG")},
		},
	}
	writes, info, err := eqBandWritePlan(summary, 3400, -3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if info["band"] != "hmf" || info["nearest_reachable_freq_hz"] != 3300.0 {
		t.Fatalf("selection info=%+v, want hmf at 3300 Hz", info)
	}
	if len(writes) != 2 || writes[0].ParamID != "HMFF" || writes[0].Role != "freq" ||
		writes[0].NormalizedValue != 0.5 || !writes[0].Quantized || writes[1].ParamID != "HMFG" {
		t.Fatalf("writes=%+v", writes)
	}
	for _, write := range writes {
		if write.Role == "used" {
			t.Fatalf("implicit frequency activation must not invent an enable write: %+v", writes)
		}
	}
}

func TestEQFixedSlotNamedBellWritePrecedesFrequencyAndGain(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "set_eq_point_supported": true,
		"completeness": map[string]any{"complete": true},
		"bands": []map[string]any{{
			"band": "lf", "active": false, "activation_strategy": "implicit_in_domain",
			"filter_kind_bindings": []map[string]any{{"param_id": "LF Bell", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.0, "label": "Out"}, {"normalized": 1.0, "label": "In", "kind": "bell"}}}},
			"frequency_bindings": []map[string]any{{"param_id": "LFF", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.0, "label": "OFF"}, {"normalized": 1.0, "physical": 100.0, "label": "100 Hz"}}}},
			"gain_bindings": []map[string]any{{"param_id": "LFG", "channel": "shared", "domain": map[string]any{"min": -18.0, "max": 18.0}}},
		}},
	}
	writes, _, err := eqBandWritePlan(summary, 100, -3, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"shape", "freq", "gain"}
	if len(writes) != len(want) {
		t.Fatalf("writes=%+v", writes)
	}
	for i, role := range want {
		if writes[i].Role != role {
			t.Fatalf("write order=%+v, want %v", writes, want)
		}
	}
	if writes[0].ParamID != "LF Bell" || writes[0].NormalizedValue != 1 || writes[0].ExpectedLabel != "In" {
		t.Fatalf("Bell write=%+v", writes[0])
	}
}

func TestEQTypedSectionSelectsInactiveContinuousCutFromPhysicalDomain(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "set_eq_point_supported": true,
		"supported_filter_kinds": []string{"low_cut"},
		"sections": []map[string]any{{
			"section": "guard", "complete": true, "dedicated_kind": "low_cut",
			"reachable_kinds": []string{"low_cut"},
			"activation":      map[string]any{"strategy": "implicit_in_domain", "active": false},
			"frequency_bindings": []map[string]any{{
				"param_id": "guard frequency", "channel": "shared", "current_text": "Out Hz",
				"domain": map[string]any{"min": 37.0, "max": 350.0, "scale": "log", "unit": "Hz"},
			}},
		}},
	}
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 120, Shape: "low_cut"})
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].ParamID != "guard frequency" || writes[0].Role != "freq" {
		t.Fatalf("writes=%+v", writes)
	}
	if info["section"] != "guard" || info["frequency_quantized"] != false {
		t.Fatalf("selection info=%+v", info)
	}
}

func TestEQTypedSectionPlanOrdersTopologyFieldsAndActivation(t *testing.T) {
	summary := map[string]any{
		"eq_model": "free_floating", "set_eq_point_supported": true,
		"supported_filter_kinds": []string{"bell", "low_cut"},
		"sections": []map[string]any{{
			"section": "1", "complete": true, "dedicated_kind": "unknown",
			"reachable_kinds": []string{"bell", "low_cut"},
			"activation":      map[string]any{"strategy": "explicit_binding", "active": false},
			"filter_kind_bindings": []map[string]any{{"param_id": "shape", "channel": "shared",
				"reachable_values": []map[string]any{{"normalized": 0.0, "label": "Bell"}, {"normalized": 0.5, "label": "Low Cut"}}}},
			"frequency_bindings": []map[string]any{{"param_id": "freq", "channel": "shared", "current_physical": 1000.0,
				"domain": map[string]any{"min": 10.0, "max": 30000.0, "scale": "log"}}},
			"q_bindings": []map[string]any{{"param_id": "q", "channel": "shared", "domain": map[string]any{"min": 0.1, "max": 10.0}}},
			"slope_bindings": []map[string]any{{"param_id": "slope", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.0, "physical": 12.0, "label": "12 dB/oct"}, {"normalized": 1.0, "physical": 24.0, "label": "24 dB/oct"}}}},
			"gain_bindings": []map[string]any{{"param_id": "gain", "channel": "shared"}},
			"activation_bindings": []map[string]any{{"param_id": "used", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.0, "label": "Unused"}, {"normalized": 1.0, "label": "Used"}}}},
		}},
	}
	gain, q, slope := -3.0, 0.7, 24.0
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 3400, GainDB: &gain, Q: &q, Shape: "low_cut", SlopeDBPerOct: &slope})
	if err != nil {
		t.Fatal(err)
	}
	wantRoles := []string{"shape", "freq", "q", "slope", "used"}
	if len(writes) != len(wantRoles) {
		t.Fatalf("writes=%+v", writes)
	}
	for i, role := range wantRoles {
		if writes[i].Role != role {
			t.Fatalf("write order=%+v, want %v", writes, wantRoles)
		}
	}
	if writes[0].NormalizedValue != 0.5 || writes[0].RequestedLabel != "low_cut" || writes[3].NormalizedValue != 1 {
		t.Fatalf("enum targets=%+v", writes)
	}
	if partial, _ := info["partial"].(bool); !partial {
		t.Fatalf("gain on a cut must be reported partial: %+v", info)
	}
}

func TestEQTypedSectionPlanReportsUnavailableOptionalFieldsWithoutGuessing(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "set_eq_point_supported": true,
		"supported_filter_kinds": []string{"low_shelf"},
		"sections": []map[string]any{{
			"section": "low", "complete": true, "dedicated_kind": "unknown",
			"reachable_kinds": []string{"bell", "low_shelf"},
			"activation":      map[string]any{"strategy": "always_active", "active": true},
			"filter_kind_bindings": []map[string]any{{"param_id": "type", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.0, "label": "Bell"}, {"normalized": 1.0, "label": "Shelf"}}}},
			"frequency_bindings": []map[string]any{{"param_id": "freq", "channel": "shared", "current_physical": 100.0}},
			"gain_bindings":      []map[string]any{{"param_id": "gain", "channel": "shared"}},
		}},
	}
	gain, q, slope := 3.0, 0.7, 24.0
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 120, GainDB: &gain, Q: &q, Shape: "low_shelf", SlopeDBPerOct: &slope})
	if err != nil {
		t.Fatal(err)
	}
	for _, write := range writes {
		if write.Role == "q" || write.Role == "slope" {
			t.Fatalf("unavailable role was guessed: %+v", writes)
		}
	}
	unsupported := eqStringSlice(info["unsupported_fields"])
	if len(unsupported) != 2 || unsupported[0] != "q" || unsupported[1] != "slope_db_per_oct" {
		t.Fatalf("unsupported=%v info=%+v", unsupported, info)
	}
	if _, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 120, GainDB: &gain, Shape: "notch"}); err == nil {
		t.Fatal("unreachable shape must fail before any write")
	}
}

func TestNormalizeRequestedEQShapeAliases(t *testing.T) {
	cases := map[string]string{"Peak": "bell", "Low-Pass": "high_cut", "HP": "low_cut", "Hi Shelf": "high_shelf", "Band Stop": "notch"}
	for input, want := range cases {
		got, err := normalizeRequestedEQShape(input)
		if err != nil || got != want {
			t.Fatalf("normalize(%q)=%q,%v want %q", input, got, err, want)
		}
	}
}
