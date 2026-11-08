package contextruntime

import (
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

// PLUGIN-CONTROL-EVAL-SEAM2-1 S2b (render-face strip), B4/C4 ablation-hook
// test pattern (305 seam tests same style): the kernel plugin_grabber_
// explain_controls reply carries binding_targets rows whose
// alias/normalized_role/template_role semantic overlay re-entered model
// context through summarizeToolResult's result_preview (307 seam leakage,
// seam_verify_corpus.json verdict). Default unset keeps the render
// byte-for-byte; VIT_PLUGIN_CONTROL_EVAL_SEAM=1 renders from a purified copy
// (binding_targets rows lose the three overlay keys, plugin-native keys
// stay, input untouched); only the exact value "1" arms the hook.

// seamS2bContaminatedResult reproduces the 307 T3_r1 payload shape (rows
// abbreviated to three; key faces identical) that entered model context via
// plugin_grabber_explain_controls result rendering.
func seamS2bContaminatedResult() map[string]any {
	return map[string]any{
		"status":         "ok",
		"plugin_id":      "1141",
		"plugin_item_id": "1141",
		"plugin_name":    "Abbey Road Plates Stereo",
		"track_id":       "1135",
		"binding_targets": []any{
			map[string]any{"alias": "Dry Level", "normalized_role": "common_mix", "param_id": "dry level", "param_name": "Dry Level", "plugin_id": "1141", "plugin_name": "Abbey Road Plates Stereo", "plugin_type": "vst", "target_kind": "plugin_param", "template_role": ""},
			map[string]any{"alias": "Wet Level", "normalized_role": "common_mix", "param_id": "wet level", "param_name": "Wet Level", "plugin_id": "1141", "plugin_name": "Abbey Road Plates Stereo", "plugin_type": "vst", "target_kind": "plugin_param", "template_role": ""},
			map[string]any{"alias": "Bypass", "normalized_role": "common_bypass", "param_id": "10000001", "param_name": "Bypass", "plugin_id": "1141", "plugin_name": "Abbey Road Plates Stereo", "plugin_type": "vst", "target_kind": "plugin_param", "template_role": ""},
		},
		"quick_controls": []any{
			map[string]any{"control_priority": 100, "host_controllable": true, "label": "Dry Level", "param_id": "dry level", "raw_param_id": "dry level", "value": 0, "widget": "slider"},
			map[string]any{"control_priority": 75, "host_controllable": true, "label": "Bypass", "param_id": "10000001", "raw_param_id": "10000001", "value": 0, "widget": "toggle"},
		},
		"capability_manifest":          []any{"open_ui", "get_param", "template_shell", "quick_controls"},
		"supports_param_grabber":       true,
		"current_param_signature_hash": "probe_365fe8436b01a5fc",
		"parameter_count":              18,
	}
}

func seamS2bSummarize(result map[string]any) map[string]any {
	return SummarizeToolResult(planner.ToolResult{
		ToolCallID: "tool_step_1",
		Tool:       "plugin_grabber_explain_controls",
		Status:     "ok",
		Result:     result,
	}, Options{})
}

// A-arm condition proof, S2b face: seam open → the rendered result_preview
// binding_targets rows carry no semantic overlay key, plugin-native keys and
// the native quick_controls rows stay, and the input tool result map is not
// mutated (purified-copy semantics).
func TestSeamS2bOpenRenderStripsBindingTargetsOverlay(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	result := seamS2bContaminatedResult()
	summary := seamS2bSummarize(result)

	preview, ok := summary["result_preview"].(map[string]any)
	if !ok {
		t.Fatalf("result_preview must render as a map, got %T", summary["result_preview"])
	}
	rows, ok := preview["binding_targets"].([]any)
	if !ok || len(rows) != 3 {
		t.Fatalf("binding_targets must render 3 rows, got %#v", preview["binding_targets"])
	}
	for i, rowAny := range rows {
		row, ok := rowAny.(map[string]any)
		if !ok {
			t.Fatalf("binding_targets[%d] must render as a map, got %T", i, rowAny)
		}
		for _, key := range []string{"alias", "normalized_role", "template_role"} {
			if _, exists := row[key]; exists {
				t.Fatalf("open seam binding_targets[%d] must not carry %q: %v", i, key, row)
			}
		}
		for _, key := range []string{"param_id", "param_name", "plugin_id", "plugin_name", "plugin_type", "target_kind"} {
			if _, exists := row[key]; !exists {
				t.Fatalf("open seam binding_targets[%d] must keep the plugin-native key %q", i, key)
			}
		}
	}
	rendered := jsonEncode(t, preview)
	for _, banned := range []string{`"alias"`, `"normalized_role"`, `"template_role"`} {
		if strings.Contains(rendered, banned) {
			t.Fatalf("open seam rendered preview must not carry %s anywhere: %s", banned, rendered)
		}
	}

	quick, ok := preview["quick_controls"].([]any)
	if !ok || len(quick) != 2 {
		t.Fatalf("quick_controls must render 2 native rows, got %#v", preview["quick_controls"])
	}
	first := quick[0].(map[string]any)
	if first["param_id"] != "dry level" || first["label"] != "Dry Level" || first["widget"] != "slider" {
		t.Fatalf("open seam must keep native quick_controls rows: %v", first)
	}

	// Purified-copy semantics: the input tool result stays untouched.
	inputRows := result["binding_targets"].([]any)
	if inputRow := inputRows[0].(map[string]any); inputRow["alias"] != "Dry Level" {
		t.Fatalf("input binding_targets row must stay untouched, got %v", inputRow)
	}
}

// Closed seam keeps the render byte-for-byte: unset still discloses the
// overlay keys (production behavior) and no non-"1" env value opens the
// strip.
func TestSeamS2bClosedKeepsRenderByteForByte(t *testing.T) {
	baseline := jsonEncode(t, seamS2bSummarize(seamS2bContaminatedResult()))
	for _, fragment := range []string{`"alias":"Dry Level"`, `"normalized_role":"common_mix"`, `"template_role":""`} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("closed seam render must keep the overlay keys (production behavior), missing %s in %s", fragment, baseline)
		}
	}
	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		if got := jsonEncode(t, seamS2bSummarize(seamS2bContaminatedResult())); got != baseline {
			t.Fatalf("env %q must keep the render byte-for-byte", value)
		}
	}
}

// The strip helper itself: nested binding_targets are purified too and the
// helper never mutates its input at any depth.
func TestSeamS2bStripHelperPurifiesNestedTargets(t *testing.T) {
	inner := map[string]any{
		"vsp": map[string]any{
			"binding_targets": []any{
				map[string]any{"alias": "Drive", "normalized_role": "tone_drive", "param_id": "3", "template_role": ""},
			},
		},
	}
	purified := stripBindingTargetsSemanticOverlay(inner, 0).(map[string]any)
	nested := purified["vsp"].(map[string]any)["binding_targets"].([]any)[0].(map[string]any)
	if _, exists := nested["alias"]; exists {
		t.Fatalf("nested binding_targets row must lose alias: %v", nested)
	}
	if nested["param_id"] != "3" {
		t.Fatalf("nested binding_targets row must keep param_id: %v", nested)
	}
	untouched := inner["vsp"].(map[string]any)["binding_targets"].([]any)[0].(map[string]any)
	if untouched["alias"] != "Drive" {
		t.Fatalf("nested input must stay untouched, got %v", untouched)
	}
}

func jsonEncode(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
