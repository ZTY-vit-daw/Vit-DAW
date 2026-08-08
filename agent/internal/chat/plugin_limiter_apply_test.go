package chat

import "testing"

func testLimiterSummaryForApply(t *testing.T) (map[string]any, map[string]string) {
	t.Helper()
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "l1t1_apply"},
		"limiter_stages": []map[string]any{{
			"stage_key":       "limiter_main",
			"operating_point": []map[string]any{{"param_id": "threshold", "role": "threshold", "channel": "shared", "domain": map[string]any{"unit": "dB", "min": -60.0, "max": 0.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, -60}, {.5, -30}, {1, 0}}}},
			"safety":          []map[string]any{{"param_id": "ceiling", "role": "ceiling", "channel": "shared", "domain": map[string]any{"unit": "dB", "min": -12.0, "max": 0.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, -12}, {.5, -6}, {1, 0}}}},
			"timing":          []map[string]any{{"param_id": "release", "role": "release", "channel": "shared", "domain": map[string]any{"unit": "ms", "min": 1.0, "max": 1001.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, 1}, {.5, 501}, {1, 1001}}}},
			"detector":        []map[string]any{}, "mode": []map[string]any{}, "output": []map[string]any{},
		}},
		"shared_controls": []map[string]any{{"param_id": "true_peak", "role": "true_peak", "reachable_values": []map[string]any{{"normalized": 0.0, "label": "Off"}, {"normalized": 1.0, "label": "On"}}}},
	}
	if err := attachLimiterControlRefs(summary, "1007", "1013"); err != nil {
		t.Fatal(err)
	}
	refs := map[string]string{}
	stage := mapRowsValue(summary["limiter_stages"])[0]
	for _, section := range []string{"operating_point", "safety", "timing"} {
		for _, row := range mapRowsValue(stage[section]) {
			refs[firstNonEmptyText(row, "role")] = firstNonEmptyText(row, "control_ref")
		}
	}
	refs["true_peak"] = firstNonEmptyText(mapRowsValue(summary["shared_controls"])[0], "control_ref")
	return summary, refs
}

func TestPlanLimiterControlsUseIndependentRefsAndUnits(t *testing.T) {
	summary, refs := testLimiterSummaryForApply(t)
	ceiling := -1.0
	write, _, err := planLimiterControl(summary, limiterControlRequest{ControlRef: refs["ceiling"], Value: &ceiling, Unit: "dB"}, "1007", "1013", "l1t1_apply")
	if err != nil {
		t.Fatal(err)
	}
	if write.ParamID != "ceiling" || write.Role != "ceiling" || write.NormalizedValue < .916 || write.NormalizedValue > .917 {
		t.Fatalf("write=%+v", write)
	}
	release := 251.0
	write, _, err = planLimiterControl(summary, limiterControlRequest{ControlRef: refs["release"], Value: &release, Unit: "ms"}, "1007", "1013", "l1t1_apply")
	if err != nil || write.NormalizedValue != .25 {
		t.Fatalf("write=%+v err=%v", write, err)
	}
	write, _, err = planLimiterControl(summary, limiterControlRequest{ControlRef: refs["true_peak"], EnumLabel: "On"}, "1007", "1013", "l1t1_apply")
	if err != nil || write.NormalizedValue != 1 {
		t.Fatalf("write=%+v err=%v", write, err)
	}
	if _, err := decodeCompressorControlRef(refs["ceiling"]); err == nil {
		t.Fatal("limiter ref must not decode as compressor ref")
	}
}

func TestPlanLimiterControlRejectsStaleAndWrongUnit(t *testing.T) {
	summary, refs := testLimiterSummaryForApply(t)
	value := 20.0
	if _, _, err := planLimiterControl(summary, limiterControlRequest{ControlRef: refs["ceiling"], Value: &value, Unit: "ms"}, "1007", "1013", "l1t1_apply"); limiterControlFailureCode(err) != "unit_role_mismatch" {
		t.Fatalf("wrong unit err=%v", err)
	}
	if _, _, err := planLimiterControl(summary, limiterControlRequest{ControlRef: refs["ceiling"], Value: &value, Unit: "dB"}, "1007", "1013", "stale"); limiterControlFailureCode(err) != "stale_control_ref" {
		t.Fatalf("stale err=%v", err)
	}
}

func TestParseLimiterControlsRequiresExactlyOneTarget(t *testing.T) {
	_, err := parseLimiterControlRequests(map[string]any{"controls": []map[string]any{{"control_ref": "x", "value_db": -1.0, "enum_label": "On"}}})
	if limiterControlFailureCode(err) != "ambiguous_value" {
		t.Fatalf("err=%v", err)
	}
}

func TestLimiterSummaryOwnsOnlyTypedBindings(t *testing.T) {
	summary, _ := testLimiterSummaryForApply(t)
	summary["auxiliary_stages"] = []map[string]any{{"kind": "clipper", "param_ids": []string{"clip"}}}
	if !limiterSummaryOwnsParameter(summary, "ceiling") || !limiterSummaryOwnsParameter(summary, "true_peak") {
		t.Fatal("typed limiter binding not owned")
	}
	if limiterSummaryOwnsParameter(summary, "clip") {
		t.Fatal("adjacent clipper evidence must not become limiter-owned")
	}
}
