package chat

import "testing"

func testCompressorSummaryForApply(t *testing.T) (map[string]any, string) {
	t.Helper()
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "c2t1_apply"},
		"compressor_stage": map[string]any{"stage_key": "compressor", "control_paths": []map[string]any{{
			"path_key": "main", "operating_point": []map[string]any{{"param_id": "1", "role": "threshold", "channel": "shared",
				"domain": map[string]any{"unit": "dB", "min": -60.0, "max": 0.0, "scale": "linear", "confidence": 0.9},
				"curve":  [][2]float64{{0, -60}, {0.5, -30}, {1, 0}}}},
		}}, "output": []map[string]any{}},
	}
	if err := attachCompressorControlRefs(summary, "1007", "1013"); err != nil {
		t.Fatal(err)
	}
	path := mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0]
	ref := firstNonEmptyText(mapRowsValue(path["operating_point"])[0], "control_ref")
	return summary, ref
}

func TestPlanCompressorControlPhysicalDB(t *testing.T) {
	summary, ref := testCompressorSummaryForApply(t)
	value := -18.0
	write, _, err := planCompressorControl(summary, compressorControlRequest{ControlRef: ref, Value: &value, Unit: "dB"},
		"1007", "1013", "c2t1_apply")
	if err != nil {
		t.Fatal(err)
	}
	if write.ParamID != "1" || write.Role != "threshold" || write.NormalizedValue < 0.699 || write.NormalizedValue > 0.701 {
		t.Fatalf("write = %+v", write)
	}
}

func TestPlanCompressorControlRejectsStaleRefAndWrongUnit(t *testing.T) {
	summary, ref := testCompressorSummaryForApply(t)
	value := 4.0
	if _, _, err := planCompressorControl(summary, compressorControlRequest{ControlRef: ref, Value: &value, Unit: "ratio"},
		"1007", "1013", "c2t1_apply"); compressorControlFailureCode(err) != "unit_role_mismatch" {
		t.Fatalf("wrong unit error = %v", err)
	}
	if _, _, err := planCompressorControl(summary, compressorControlRequest{ControlRef: ref, Value: &value, Unit: "dB"},
		"1007", "1013", "different"); compressorControlFailureCode(err) != "stale_control_ref" {
		t.Fatalf("stale ref error = %v", err)
	}
}

func TestCompressorUnitCompatibilityUsesObservedPercentDomain(t *testing.T) {
	if !compressorUnitCompatible("reduction_amount", "%", "%") {
		t.Fatal("observed percentage reduction amount should accept a percentage target")
	}
	if compressorUnitCompatible("reduction_amount", "%", "enum") {
		t.Fatal("enum reduction amount must not accept a percentage target")
	}
}

func TestParseCompressorControlRequestsRequiresExactlyOneTarget(t *testing.T) {
	_, err := parseCompressorControlRequests(map[string]any{"controls": []map[string]any{{
		"control_ref": "x", "value_db": -10.0, "ratio": 4.0,
	}}})
	if compressorControlFailureCode(err) != "ambiguous_value" {
		t.Fatalf("error = %v", err)
	}
}
