package tools

import "testing"

func TestDefaultCatalogExposesCoreCommands(t *testing.T) {
	catalog := DefaultCatalog()

	state, ok := catalog.LookupCommand("get_project_state")
	if !ok {
		t.Fatal("get_project_state missing")
	}
	if state.ToolName != "project.state" || state.RequiresConfirmation {
		t.Fatalf("project state metadata = %+v", state)
	}

	del, ok := catalog.LookupCommand("delete_track")
	if !ok {
		t.Fatal("delete_track missing")
	}
	if !del.RequiresConfirmation || del.RiskLevel != RiskConfirm {
		t.Fatalf("delete_track should require confirmation: %+v", del)
	}

	add, ok := catalog.LookupTool("track.add")
	if !ok {
		t.Fatal("track.add missing")
	}
	if add.CommandName != "add_track" || add.RequiresConfirmation || add.RiskLevel != RiskUndoable {
		t.Fatalf("track.add metadata = %+v", add)
	}
}

func TestPluginGrabberAliasIsCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	spec, ok := catalog.LookupTool("plugin_create_control_graph_node")
	if !ok {
		t.Fatal("plugin_create_control_graph_node missing")
	}
	if spec.CommandName != "control_add_node" {
		t.Fatalf("command = %q, want control_add_node", spec.CommandName)
	}
	params, ok := catalog.LookupTool("plugin_get_parameters")
	if !ok {
		t.Fatal("plugin_get_parameters missing")
	}
	if params.CommandName != "get_plugin_parameters" {
		t.Fatalf("command = %q, want get_plugin_parameters", params.CommandName)
	}
}

func TestCommandNameFallbacks(t *testing.T) {
	if got := CommandName(map[string]any{"action": "play"}); got != "play" {
		t.Fatalf("got %q", got)
	}
	if got := CommandName(map[string]any{"command": "stop"}); got != "stop" {
		t.Fatalf("got %q", got)
	}
}
