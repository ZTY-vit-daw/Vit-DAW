package executor

import (
	"testing"

	"vit-daw-agent/internal/planner"
)

func TestCoercePluginInstantiateWithPathToRackLoad(t *testing.T) {
	call := coercePluginInstantiateCall(planner.ToolCall{
		Tool: "plugin.instantiate",
		Args: map[string]any{
			"track_id":    "1007",
			"plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`,
		},
	})
	if call.Tool != "plugin.load_to_rack" {
		t.Fatalf("tool = %q", call.Tool)
	}
	if call.Args["track_id"] != "1007" || call.Args["plugin_path"] == "" {
		t.Fatalf("args = %+v", call.Args)
	}
	if call.Command != nil {
		t.Fatalf("command should be cleared: %+v", call.Command)
	}
}

func TestCoercePluginInstantiateWithoutPathToSemanticSearch(t *testing.T) {
	call := coercePluginInstantiateCall(planner.ToolCall{
		Tool: "daw.invoke",
		Command: map[string]any{
			"cmd":      "instantiate_plugin",
			"track_id": "1007",
			"name":     "TDR Nova",
		},
	})
	if call.Tool != "plugin.semantic_search" {
		t.Fatalf("tool = %q", call.Tool)
	}
	if call.Args["query"] != "TDR Nova" {
		t.Fatalf("args = %+v", call.Args)
	}
	if call.Command != nil {
		t.Fatalf("command should be cleared: %+v", call.Command)
	}
}
