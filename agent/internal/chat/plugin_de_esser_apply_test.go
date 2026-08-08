package chat

import (
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestDeEsserApplyCommandRecognition(t *testing.T) {
	cmd, ok := pluginGrabberApplyDeEsserInvokeCommand(harness.InvokeRequest{Tool: pluginGrabberApplyDeEsserTool, Args: map[string]any{"track_id": "1007", "plugin_id": "1013"}})
	if !ok || firstNonEmptyText(cmd, "cmd") != pluginGrabberApplyDeEsserCommand || firstNonEmptyText(cmd, "track_id") != "1007" {
		t.Fatalf("cmd=%+v ok=%v", cmd, ok)
	}
}

func testDeEsserSummaryForApply(t *testing.T) (map[string]any, map[string]string) {
	t.Helper()
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "d1t1_apply"},
		"de_esser_stages": []map[string]any{{
			"stage_key": "de_esser_main",
			"operating_point": []map[string]any{
				{"param_id": "threshold", "role": "threshold", "channel": "shared", "domain": map[string]any{"unit": "dB", "min": -60.0, "max": 0.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, -60}, {.5, -30}, {1, 0}}},
				{"param_id": "range", "role": "reduction_range", "channel": "shared", "domain": map[string]any{"unit": "dB", "min": 0.0, "max": 24.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, 0}, {.5, 12}, {1, 24}}},
			},
			"frequency_selectivity": []map[string]any{{"param_id": "focus", "role": "focus_frequency", "channel": "shared", "domain": map[string]any{"unit": "Hz", "min": 2000.0, "max": 12000.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, 2000}, {.5, 7000}, {1, 12000}}}},
			"timing":                []map[string]any{{"param_id": "release", "role": "release", "channel": "shared", "domain": map[string]any{"unit": "ms", "min": 1.0, "max": 1001.0, "scale": "linear", "confidence": 0.95}, "curve": [][2]float64{{0, 1}, {.5, 501}, {1, 1001}}}},
			"mode":                  []map[string]any{{"param_id": "mode", "role": "mode", "channel": "shared", "reachable_values": []map[string]any{{"normalized": 0.0, "label": "Wide"}, {"normalized": 1.0, "label": "Split"}}}},
			"output":                []map[string]any{},
		}},
	}
	if err := attachDeEsserControlRefs(summary, "1007", "1013"); err != nil {
		t.Fatal(err)
	}
	refs := map[string]string{}
	stage := mapRowsValue(summary["de_esser_stages"])[0]
	for _, section := range []string{"operating_point", "frequency_selectivity", "timing", "mode"} {
		for _, row := range mapRowsValue(stage[section]) {
			refs[firstNonEmptyText(row, "role")] = firstNonEmptyText(row, "control_ref")
		}
	}
	return summary, refs
}

func TestPlanDeEsserControlsUseIndependentRefsAndUnits(t *testing.T) {
	summary, refs := testDeEsserSummaryForApply(t)
	value := -15.0
	write, _, err := planDeEsserControl(summary, deEsserControlRequest{ControlRef: refs["threshold"], Value: &value, Unit: "dB"}, "1007", "1013", "d1t1_apply")
	if err != nil || write.ParamID != "threshold" || write.NormalizedValue != .75 {
		t.Fatalf("write=%+v err=%v", write, err)
	}
	freq := 7000.0
	write, _, err = planDeEsserControl(summary, deEsserControlRequest{ControlRef: refs["focus_frequency"], Value: &freq, Unit: "Hz"}, "1007", "1013", "d1t1_apply")
	if err != nil || write.NormalizedValue != .5 {
		t.Fatalf("write=%+v err=%v", write, err)
	}
	write, _, err = planDeEsserControl(summary, deEsserControlRequest{ControlRef: refs["mode"], EnumLabel: "Split"}, "1007", "1013", "d1t1_apply")
	if err != nil || write.NormalizedValue != 1 {
		t.Fatalf("write=%+v err=%v", write, err)
	}
	if _, err := decodeLimiterControlRef(refs["threshold"]); err == nil {
		t.Fatal("De-esser ref must not decode as limiter ref")
	}
}

func TestPlanDeEsserControlRejectsStaleWrongUnitAndAuxiliaryOwnership(t *testing.T) {
	summary, refs := testDeEsserSummaryForApply(t)
	value := 10.0
	if _, _, err := planDeEsserControl(summary, deEsserControlRequest{ControlRef: refs["threshold"], Value: &value, Unit: "ms"}, "1007", "1013", "d1t1_apply"); deEsserControlFailureCode(err) != "unit_role_mismatch" {
		t.Fatalf("wrong unit err=%v", err)
	}
	if _, _, err := planDeEsserControl(summary, deEsserControlRequest{ControlRef: refs["threshold"], Value: &value, Unit: "dB"}, "1007", "1013", "stale"); deEsserControlFailureCode(err) != "stale_control_ref" {
		t.Fatalf("stale err=%v", err)
	}
	if deEsserSummaryOwnsParameter(summary, "clipper_aux") {
		t.Fatal("auxiliary stage must not be De-esser-owned")
	}
}

func TestParseDeEsserControlsRequiresExactlyOneTarget(t *testing.T) {
	_, err := parseDeEsserControlRequests(map[string]any{"controls": []map[string]any{{"control_ref": "x", "value_db": -1.0, "enum_label": "Split"}}})
	if deEsserControlFailureCode(err) != "ambiguous_value" {
		t.Fatalf("err=%v", err)
	}
}
