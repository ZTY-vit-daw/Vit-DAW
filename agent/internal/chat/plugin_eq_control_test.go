package chat

import (
	"math"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
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

func TestEQWritesForRoleUsesDecreasingSentinelTrimmedFrequencyCurve(t *testing.T) {
	section := map[string]any{"frequency_bindings": []map[string]any{{
		"param_id": "lp", "channel": "shared",
		"curve": []any{
			[]any{0.25, 10700.0}, []any{0.5, 5000.0},
			[]any{0.75, 3700.0}, []any{1.0, 3000.0},
		},
	}}}
	writes, err := eqWritesForRole(section, "frequency", "freq", 8000, 20, 20000, "log")
	if err != nil || len(writes) != 1 {
		t.Fatalf("decreasing frequency writes=%+v err=%v", writes, err)
	}
	write := writes[0]
	if write.Quantized || !write.PhysicalDecreasing || write.NormalizedValue <= 0.25 || write.NormalizedValue >= 0.5 {
		t.Fatalf("decreasing frequency write=%+v", write)
	}
	if write.CorrectionLow != 0.25 || write.CorrectionHigh != 0.5 {
		t.Fatalf("correction bounds=[%v,%v], want [0.25,0.5]", write.CorrectionLow, write.CorrectionHigh)
	}
	if _, err := eqWritesForRole(section, "frequency", "freq", 12000, 20, 20000, "log"); err == nil {
		t.Fatal("target beyond the measured active suffix must still reject")
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

func TestEQTypedSectionSelectsZeroInclusiveFrequencyCurve(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "supported_filter_kinds": []string{"low_cut"},
		"sections": []map[string]any{{
			"section": "low cut", "complete": true, "dedicated_kind": "low_cut",
			"reachable_kinds": []string{"low_cut"},
			"activation":      map[string]any{"strategy": "explicit_binding", "active": true},
			"frequency_bindings": []map[string]any{{"param_id": "hp-f", "channel": "shared", "current_text": "0",
				"domain": map[string]any{"min": 0.0, "max": 26000.0, "scale": "linear"},
				"curve":  [][2]float64{{0, 0}, {0.25, 120}, {0.5, 721}, {0.75, 4330}, {1, 26000}}}},
		}},
	}
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 80, Shape: "low_cut"})
	if err != nil {
		t.Fatal(err)
	}
	if info["section"] != "low cut" || len(writes) != 1 || writes[0].ParamID != "hp-f" || writes[0].RequestedPhysical == nil || *writes[0].RequestedPhysical != 80 {
		t.Fatalf("zero-inclusive frequency plan writes=%+v info=%+v", writes, info)
	}
}

func TestEQTypedSectionPlansFrequencyRangeMultiplier(t *testing.T) {
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "supported_filter_kinds": []string{"bell"},
		"sections": []map[string]any{{
			"section": "high mid 1", "complete": true, "dedicated_kind": "bell",
			"reachable_kinds": []string{"bell"},
			"activation":      map[string]any{"strategy": "explicit_binding", "active": true},
			"frequency_bindings": []map[string]any{{"param_id": "hmf-f", "channel": "shared", "current_physical": 1425.0,
				"domain": map[string]any{"min": 250.0, "max": 2500.0, "scale": "log"}}},
			"frequency_transform_bindings": []map[string]any{{"param_id": "hmf-x10", "channel": "shared", "current_physical": 1.0,
				"reachable_values": []map[string]any{{"normalized": 0.0, "physical": 1.0, "label": "Off"}, {"normalized": 1.0, "physical": 10.0, "label": "On"}}}},
			"q_bindings": []map[string]any{{"param_id": "hmf-q", "channel": "shared", "domain": map[string]any{"min": 0.4, "max": 4.0}}},
			"gain_bindings": []map[string]any{{"param_id": "hmf-g", "channel": "shared", "reachable_values": []map[string]any{
				{"normalized": 0.4, "physical": -4.0, "label": "-4.0"}, {"normalized": 0.45, "physical": -2.0, "label": "-2.0"}}}},
		}},
	}
	gain, q := -3.0, 0.5
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 3400, GainDB: &gain, Q: &q, Shape: "bell"})
	if err != nil {
		t.Fatal(err)
	}
	if info["section"] != "high mid 1" || len(writes) != 4 {
		t.Fatalf("multiplier plan writes=%+v info=%+v", writes, info)
	}
	wantRoles := []string{"freq", "frequency_transform", "q", "gain"}
	for index, role := range wantRoles {
		if writes[index].Role != role {
			t.Fatalf("multiplier write order=%+v, want %v", writes, wantRoles)
		}
	}
	if writes[0].RequestedPhysical == nil || *writes[0].RequestedPhysical != 3400 || writes[0].PhysicalMultiplier != 10 ||
		writes[1].ExpectedLabel != "On" || writes[1].NormalizedValue != 1 {
		t.Fatalf("multiplier targets=%+v", writes)
	}
	if actual := eqEffectivePhysicalReadback(writes[0], 340); actual != 3400 {
		t.Fatalf("base-frequency readback was not projected through multiplier: %g", actual)
	}
}

func TestPhysicalReadbackForRoleUsesLocalizedEQNumbers(t *testing.T) {
	value, ok := physicalReadbackForRole("q", map[string]any{"value_text": "0,5"})
	if !ok || value != 0.5 {
		t.Fatalf("localized Q readback=(%g,%v), want (0.5,true)", value, ok)
	}
}

func propertySentinelCutSummary(active bool, currentSlope string) map[string]any {
	return map[string]any{
		"eq_model": "fixed_slot_adjustable", "supported_filter_kinds": []string{"low_cut"},
		"sections": []map[string]any{{
			"section": "low cut 1", "complete": true, "dedicated_kind": "low_cut",
			"reachable_kinds": []string{"low_cut"}, "active": active,
			"activation_strategy": "property_sentinel",
			"activation":          map[string]any{"strategy": "property_sentinel", "binding_role": "slope", "active": active},
			"frequency_bindings": []map[string]any{{"param_id": "hpf", "channel": "shared", "current_physical": 20.0,
				"domain": map[string]any{"min": 20.0, "max": 22000.0, "scale": "log"}}},
			"slope_bindings": []map[string]any{{"param_id": "hps", "channel": "shared", "current_text": currentSlope,
				"reachable_values": []map[string]any{
					{"normalized": 0.0, "label": "Off"},
					{"normalized": 0.5, "physical": 6.0, "label": "6 dB"},
					{"normalized": 1.0, "physical": 12.0, "label": "12 dB"},
				}}},
		}},
	}
}

func TestEQTypedSectionActivatesSlopePropertySentinelConservatively(t *testing.T) {
	summary := propertySentinelCutSummary(false, "Off")
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 80, Shape: "low_cut"})
	if err != nil {
		t.Fatal(err)
	}
	if info["section"] != "low cut 1" || len(writes) != 2 || writes[0].Role != "freq" ||
		writes[1].ParamID != "hps" || writes[1].Role != "used" || writes[1].NormalizedValue != 0.5 ||
		writes[1].ExpectedLabel != "6 dB" || !writes[1].ActivationLast {
		t.Fatalf("property-sentinel writes=%+v info=%+v", writes, info)
	}
}

func TestEQTypedSectionExplicitSlopeServesAsPropertySentinelActivation(t *testing.T) {
	summary := propertySentinelCutSummary(false, "Off")
	slope := 12.0
	writes, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 80, Shape: "low_cut", SlopeDBPerOct: &slope,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 || writes[1].ParamID != "hps" || writes[1].Role != "slope" ||
		writes[1].NormalizedValue != 1 || !writes[1].ActivationLast {
		t.Fatalf("explicit slope must be the single activation write: %+v", writes)
	}
}

func TestEQTypedSectionPreservesActivePropertySentinelSlope(t *testing.T) {
	summary := propertySentinelCutSummary(true, "12 dB")
	writes, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 80, Shape: "low_cut"})
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].ParamID != "hpf" {
		t.Fatalf("unspecified active slope must be preserved: %+v", writes)
	}
}

func TestEQPropertySentinelDisableAndActivationOrdering(t *testing.T) {
	section := mapRowsValue(propertySentinelCutSummary(false, "Off")["sections"])[0]
	disable, err := planEQDeactivateWrites(section, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(disable) != 1 || disable[0].ParamID != "hps" || disable[0].Role != "disabled" ||
		disable[0].NormalizedValue != 0 || disable[0].ExpectedLabel != "Off" {
		t.Fatalf("property-sentinel disable=%+v", disable)
	}
	combined, err := combineAtomicEQWrites([]eqPlannedEdit{{Writes: []eqWriteStep{
		{ParamID: "hps", Role: "slope", ActivationLast: true},
		{ParamID: "hpf", Role: "freq"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(combined) != 2 || combined[0].ParamID != "hpf" || combined[1].ParamID != "hps" {
		t.Fatalf("activation must be ordered last: %+v", combined)
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
	if _, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 3400, GainDB: &gain, Q: &q, Shape: "low_cut", SlopeDBPerOct: &slope}); err == nil {
		t.Fatal("gain on a cut must reject the complete plan before writing")
	}
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 3400, Q: &q, Shape: "low_cut", SlopeDBPerOct: &slope})
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
	if partial, _ := info["partial"].(bool); partial {
		t.Fatalf("strict plan cannot report partial success: %+v", info)
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
	if _, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 120, GainDB: &gain, Q: &q, Shape: "low_shelf", SlopeDBPerOct: &slope}); err == nil || !strings.Contains(err.Error(), "explicit_field_unavailable") {
		t.Fatalf("explicit Q/slope must reject the whole plan, got %v", err)
	}
	if _, _, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{FreqHz: 120, GainDB: &gain, Shape: "notch"}); err == nil {
		t.Fatal("unreachable shape must fail before any write")
	}
}

func TestEQTypedSectionSelectsCandidateThatSatisfiesAllExplicitFields(t *testing.T) {
	section := func(name, freqID, qID string, current, qMin float64) map[string]any {
		return map[string]any{
			"section": name, "complete": true, "dedicated_kind": "bell",
			"reachable_kinds": []string{"bell"},
			"activation":      map[string]any{"strategy": "always_active", "active": true},
			"frequency_bindings": []map[string]any{{"param_id": freqID, "channel": "shared", "current_physical": current,
				"domain": map[string]any{"min": 370.0, "max": 26000.0, "scale": "log"}}},
			"q_bindings": []map[string]any{{"param_id": qID, "channel": "shared",
				"domain": map[string]any{"min": qMin, "max": 4.0, "scale": "log"}}},
			"gain_bindings": []map[string]any{{"param_id": name + " gain", "channel": "shared",
				"domain": map[string]any{"min": -20.0, "max": 20.0, "scale": "linear"}}},
		}
	}
	summary := map[string]any{
		"eq_model": "fixed_slot_adjustable", "supported_filter_kinds": []string{"bell"},
		"sections": []map[string]any{
			section("HF", "HF frequency", "HF Q", 3400, 0.9),
			section("HMF", "HMF frequency", "HMF Q", 5000, 0.4),
		},
	}
	gain, q := -3.0, 0.5
	writes, info, err := eqTypedSectionWritePlan(summary, eqTypedPointRequest{
		FreqHz: 3400, GainDB: &gain, Q: &q, Shape: "bell",
	})
	if err != nil {
		t.Fatal(err)
	}
	if info["section"] != "HMF" {
		t.Fatalf("selected section=%v, want HMF; info=%+v", info["section"], info)
	}
	for _, write := range writes {
		if write.ParamID == "HF Q" || write.ParamID == "HF frequency" {
			t.Fatalf("infeasible HF section leaked into writes: %+v", writes)
		}
	}
}

func TestParseEQFrequencyReadbackSupportsBareK(t *testing.T) {
	if got, ok := parseEQFrequencyReadback("12.00k"); !ok || got != 12000 {
		t.Fatalf("parseEQFrequencyReadback=(%g,%v), want (12000,true)", got, ok)
	}
}

func TestNormalizeRequestedEQShapeAliases(t *testing.T) {
	cases := map[string]string{"Peak": "bell", "Low-Pass": "high_cut", "HP": "low_cut", "Hi Shelf": "high_shelf"}
	for input, want := range cases {
		got, err := normalizeRequestedEQShape(input)
		if err != nil || got != want {
			t.Fatalf("normalize(%q)=%q,%v want %q", input, got, err, want)
		}
	}
	if _, err := normalizeRequestedEQShape("Band Stop"); err == nil {
		t.Fatal("Notch/Stop is outside the generic static EQ protocol")
	}
}

// EQNIL-1: a typed-nil *kernel.VSPCommandResult (non-nil interface wrapping a
// nil pointer) must degrade to the same "empty VSP response" contract as a
// nil interface, not panic on field access.
func TestEQVSPFailureTypedNilResultReturnsEmptyResponseWithoutPanic(t *testing.T) {
	var typedNil *kernel.VSPCommandResult

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("eqVSPFailure panicked on typed-nil *kernel.VSPCommandResult: %v", r)
		}
	}()

	if got := eqVSPFailure(typedNil); got != "empty VSP response" {
		t.Fatalf("eqVSPFailure(typed-nil) = %q, want %q", got, "empty VSP response")
	}
}

// EQNIL-1: pin the surrounding eqVSPFailure contracts so the nil guard is the
// only behavior change.
func TestEQVSPFailureNonNilResultBehaviorUnchanged(t *testing.T) {
	if got := eqVSPFailure(nil); got != "empty VSP response" {
		t.Fatalf("eqVSPFailure(nil) = %q, want %q", got, "empty VSP response")
	}
	if got := eqVSPFailure("not a vsp result"); got != "invalid VSP response" {
		t.Fatalf("eqVSPFailure(non-VSP type) = %q, want %q", got, "invalid VSP response")
	}
	healthy := &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}
	if got := eqVSPFailure(healthy); got != "" {
		t.Fatalf("eqVSPFailure(healthy result) = %q, want empty", got)
	}
	failed := &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "band rejected"}}
	if got := eqVSPFailure(failed); got != "band rejected" {
		t.Fatalf("eqVSPFailure(error result) = %q, want %q", got, "band rejected")
	}
}
