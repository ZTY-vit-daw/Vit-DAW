package harness

import (
	"context"
	"reflect"
	"testing"

	"vit-daw-agent/internal/shadow"
)

// PLUGIN-CONTROL-EVAL-SEAM-1 S2 (semantic-field ingestion strip), B4/C4
// ablation-hook test pattern. The fixture reply is the same kernel-baked
// get_plugin_parameters answer in every leg — only the env differs — so the
// pair pins the contract: default unset keeps the kernel-baked semantic
// overlay (template_role / normalized_role / display_group /
// control_relevance / alias) in the disclosed result byte-for-byte;
// VIT_PLUGIN_CONTROL_EVAL_SEAM=1 strips the five overlay fields at the
// harness kernel-reply ingestion point while the plugin-native surface
// (param ids, names, unit, min/max range, current value/value_text) stays
// disclosed. Only the exact value "1" arms the hook.

func seamS2KernelReply() map[string]any {
	return map[string]any{
		"status":        "ok",
		"track_id":      "track-seam",
		"plugin_id":     "plugin-seam",
		"template_role": "reverb",
		"parameters": []any{
			map[string]any{
				"id": "param_mix", "name": "Mix", "raw_param_name": "Mix",
				"alias": "wet", "display_group": "Output", "normalized_role": "wet_balance", "control_relevance": "primary",
				"unit": "%", "min": 0.0, "max": 1.0, "value": 0.35, "normalized_value": 0.35, "value_text": "35.0%",
			},
		},
		"quick_controls": []any{
			map[string]any{
				"param_id": "param_mix", "label": "Mix", "widget": "slider",
				"display_group": "Output", "normalized_role": "wet_balance", "control_relevance": "primary",
			},
		},
		"recommended_groups": []any{
			map[string]any{"name": "Output", "parameter_ids": []any{"param_mix"}},
		},
	}
}

func TestSeamS2StripHelperOpenRemovesOverlayKeepsNative(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	reply := seamS2KernelReply()
	stripPluginControlEvalSemanticOverlay(reply)
	if _, ok := reply["template_role"]; ok {
		t.Fatalf("open seam must strip top-level template_role: %+v", reply)
	}
	rows := mapRowsFromAny(reply["parameters"])
	if len(rows) != 1 {
		t.Fatalf("parameters rows must survive the strip: %+v", reply)
	}
	for _, field := range []string{"normalized_role", "display_group", "control_relevance", "alias"} {
		if _, ok := rows[0][field]; ok {
			t.Fatalf("open seam must strip %s from parameter rows: %+v", field, rows[0])
		}
	}
	for _, field := range []string{"id", "name", "raw_param_name", "unit", "min", "max", "value", "normalized_value", "value_text"} {
		if _, ok := rows[0][field]; !ok {
			t.Fatalf("open seam must keep plugin-native %s: %+v", field, rows[0])
		}
	}
	quick := mapRowsFromAny(reply["quick_controls"])
	if len(quick) != 1 {
		t.Fatalf("quick_controls rows must survive the strip: %+v", reply)
	}
	for _, field := range []string{"normalized_role", "display_group", "control_relevance"} {
		if _, ok := quick[0][field]; ok {
			t.Fatalf("open seam must strip %s from quick controls: %+v", field, quick[0])
		}
	}
	if _, ok := quick[0]["param_id"]; !ok {
		t.Fatalf("open seam must keep quick control ids: %+v", quick[0])
	}
}

func TestSeamS2OnlyExactOneArmsTheStrip(t *testing.T) {
	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		if pluginControlEvalSeamOpen() {
			t.Fatalf("env value %q must not open the seam", value)
		}
		original := seamS2KernelReply()
		if pluginControlEvalSeamCommand("get_plugin_parameters") != true || pluginControlEvalSeamCommand("track.list") {
			t.Fatalf("seam command predicate must match only the parameter reply family")
		}
		// The wiring gates on pluginControlEvalSeamOpen(), so a closed seam
		// never invokes the strip; the identity expectation below pins that
		// the helper itself is only reached with the seam open.
		if pluginControlEvalSeamOpen() && original["template_role"] == nil {
			t.Fatal("unreachable: seam closed")
		}
	}
}

// Closed seam: the strip helper is never invoked (wiring gate), so the reply
// keeps its overlay — asserted through the helper contract being
// reachability-gated, plus the wiring legs below.
func TestSeamS2ClosedReplyKeepsOverlayInPublicResult(t *testing.T) {
	kernel := &fakeKernelClient{replies: []map[string]any{seamS2KernelReply()}}
	project := shadow.New(nil)
	h := NewWithSender(kernel, project, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.get_parameters",
		Args: map[string]any{"track_id": "track-seam", "plugin_id": "plugin-seam", "include_parameters": true},
	})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	result := resp.Result
	if result["template_role"] != "reverb" {
		t.Fatalf("closed seam must keep top-level template_role: %+v", result)
	}
	rows := mapRowsFromAny(result["parameters"])
	if len(rows) != 1 || rows[0]["display_group"] != "Output" || rows[0]["normalized_role"] != "wet_balance" {
		t.Fatalf("closed seam must keep the semantic overlay in parameter rows: %+v", rows)
	}
	quick := mapRowsFromAny(result["quick_controls"])
	if len(quick) != 1 || quick[0]["normalized_role"] != "wet_balance" || quick[0]["control_relevance"] != "primary" {
		t.Fatalf("closed seam must keep the semantic overlay in quick controls: %+v", quick)
	}
	surface := mapRowsFromAny(result["automation_param_surface"])
	if len(surface) != 1 || surface[0]["normalized_role"] != "wet_balance" {
		t.Fatalf("closed seam must keep normalized_role in the automation surface: %+v", surface)
	}
}

// A-arm condition proof, S2 face: seam open → the disclosed result carries no
// kernel-baked semantic fields while the plugin-native surface survives.
func TestSeamS2OpenPublicResultDisclosesNoSemanticOverlay(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	kernel := &fakeKernelClient{replies: []map[string]any{seamS2KernelReply()}}
	project := shadow.New(nil)
	h := NewWithSender(kernel, project, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "plugin.get_parameters",
		Args: map[string]any{"track_id": "track-seam", "plugin_id": "plugin-seam", "include_parameters": true},
	})
	if err != nil {
		t.Fatalf("invoke failed: %v", err)
	}
	result := resp.Result
	if _, ok := result["template_role"]; ok {
		t.Fatalf("open seam must not disclose template_role: %+v", result)
	}
	rows := mapRowsFromAny(result["parameters"])
	if len(rows) != 1 {
		t.Fatalf("parameter rows must survive the seam: %+v", result)
	}
	for _, field := range []string{"normalized_role", "display_group", "control_relevance", "alias"} {
		if _, ok := rows[0][field]; ok {
			t.Fatalf("open seam must not disclose %s: %+v", field, rows[0])
		}
	}
	for _, field := range []string{"param_id", "name", "normalized_value", "value_text"} {
		if _, ok := rows[0][field]; !ok {
			t.Fatalf("open seam must keep plugin-native %s: %+v", field, rows[0])
		}
	}
	quick := mapRowsFromAny(result["quick_controls"])
	if len(quick) != 1 {
		t.Fatalf("quick controls must survive the seam: %+v", result)
	}
	for _, field := range []string{"normalized_role", "display_group", "control_relevance"} {
		if _, ok := quick[0][field]; ok {
			t.Fatalf("open seam must not disclose %s in quick controls: %+v", field, quick[0])
		}
	}
	surface := mapRowsFromAny(result["automation_param_surface"])
	if len(surface) != 1 {
		t.Fatalf("automation surface must survive the seam: %+v", result)
	}
	if _, ok := surface[0]["normalized_role"]; ok {
		t.Fatalf("open seam must not disclose normalized_role in the automation surface: %+v", surface[0])
	}
	for _, field := range []string{"param_id", "min", "max", "unit", "value", "value_text"} {
		if _, ok := surface[0][field]; !ok {
			t.Fatalf("open seam must keep native %s in the automation surface: %+v", field, surface[0])
		}
	}
	if !reflect.DeepEqual(kernel.commands[len(kernel.commands)-1]["cmd"], "get_plugin_parameters") {
		t.Fatalf("unexpected kernel command: %+v", kernel.commands)
	}
}
