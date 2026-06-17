package tools

import (
	"strings"
	"testing"
)

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

	rename, ok := catalog.LookupTool("track.rename")
	if !ok {
		t.Fatal("track.rename missing")
	}
	if rename.CommandName != "rename_track" || rename.RequiresConfirmation {
		t.Fatalf("track.rename metadata = %+v", rename)
	}

	solo, ok := catalog.LookupTool("track.solo")
	if !ok {
		t.Fatal("track.solo missing")
	}
	if solo.CommandName != "set_solo" || solo.RequiresConfirmation {
		t.Fatalf("track.solo metadata = %+v", solo)
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
	search, ok := catalog.LookupTool("plugin.search")
	if !ok {
		t.Fatal("plugin.search missing")
	}
	if search.CommandName != "plugin_search" || search.RequiresConfirmation || search.RiskLevel != RiskDirect {
		t.Fatalf("plugin search metadata = %+v", search)
	}
	semanticSearch, ok := catalog.LookupTool("plugin.semantic_search")
	if !ok {
		t.Fatal("plugin.semantic_search missing")
	}
	if semanticSearch.CommandName != "plugin_semantic_search" || semanticSearch.RequiresConfirmation || semanticSearch.RiskLevel != RiskDirect {
		t.Fatalf("plugin semantic search metadata = %+v", semanticSearch)
	}
	semanticBuild, ok := catalog.LookupTool("plugin.semantic_build_index")
	if !ok {
		t.Fatal("plugin.semantic_build_index missing")
	}
	if semanticBuild.CommandName != "plugin_semantic_build_index" || !semanticBuild.RequiresConfirmation || semanticBuild.RiskLevel != RiskConfirm {
		t.Fatalf("plugin semantic build metadata = %+v", semanticBuild)
	}
	load, ok := catalog.LookupTool("plugin.load_to_rack")
	if !ok {
		t.Fatal("plugin.load_to_rack missing")
	}
	if load.CommandName != "rack_add_node" || !load.RequiresConfirmation || load.RiskLevel != RiskConfirm {
		t.Fatalf("plugin load metadata = %+v", load)
	}
	macro, ok := catalog.LookupTool("rack.add_macro")
	if !ok {
		t.Fatal("rack.add_macro alias missing")
	}
	if macro.CommandName != "control_add_macro" || !macro.RequiresConfirmation || macro.RiskLevel != RiskConfirm {
		t.Fatalf("macro alias metadata = %+v", macro)
	}
	controlMacro, ok := catalog.LookupTool("control.add_macro")
	if !ok {
		t.Fatal("control.add_macro alias missing")
	}
	if controlMacro.CommandName != "control_add_macro" {
		t.Fatalf("control macro command = %q, want control_add_macro", controlMacro.CommandName)
	}
	renameMacro, ok := catalog.LookupTool("control.rename_macro")
	if !ok {
		t.Fatal("control.rename_macro alias missing")
	}
	if renameMacro.CommandName != "control_rename_macro" || renameMacro.RequiresConfirmation || renameMacro.RiskLevel != RiskUndoable {
		t.Fatalf("macro rename metadata = %+v", renameMacro)
	}
	profiles, ok := catalog.LookupTool("plugin_grabber.get_project_profiles")
	if !ok {
		t.Fatal("plugin_grabber.get_project_profiles missing")
	}
	if profiles.CommandName != "plugin_grabber_get_project_profiles" || profiles.RequiresConfirmation || profiles.RiskLevel != RiskDirect {
		t.Fatalf("profile read metadata = %+v", profiles)
	}
	explain, ok := catalog.LookupTool("plugin_grabber.explain_controls")
	if !ok {
		t.Fatal("plugin_grabber.explain_controls missing")
	}
	if explain.CommandName != "plugin_grabber_explain_controls" || explain.RequiresConfirmation || explain.RiskLevel != RiskDirect {
		t.Fatalf("explain controls metadata = %+v", explain)
	}
	upsert, ok := catalog.LookupTool("plugin_grabber.upsert_project_profile")
	if !ok {
		t.Fatal("plugin_grabber.upsert_project_profile missing")
	}
	if upsert.CommandName != "plugin_grabber_upsert_project_profile" || !upsert.RequiresConfirmation || upsert.RiskLevel != RiskConfirm {
		t.Fatalf("profile upsert metadata = %+v", upsert)
	}
	applyControl, ok := catalog.LookupTool("plugin_grabber.apply_control")
	if !ok {
		t.Fatal("plugin_grabber.apply_control missing")
	}
	if applyControl.CommandName != "plugin_grabber_apply_control" || applyControl.RequiresConfirmation || applyControl.RiskLevel != RiskUndoable || !applyControl.SupportsUndo {
		t.Fatalf("apply control metadata = %+v", applyControl)
	}
}

func TestMidiPatchToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()

	read, ok := catalog.LookupTool("midi.read_clip_notes")
	if !ok {
		t.Fatal("midi.read_clip_notes missing")
	}
	if read.CommandName != "get_midi_clip_notes" || read.RequiresConfirmation || read.RiskLevel != RiskDirect {
		t.Fatalf("read metadata = %+v", read)
	}

	patch, ok := catalog.LookupTool("midi.apply_note_patch")
	if !ok {
		t.Fatal("midi.apply_note_patch missing")
	}
	if patch.CommandName != "apply_midi_note_patch" || !patch.RequiresConfirmation || patch.RiskLevel != RiskConfirm || !patch.SupportsUndo || !patch.RefreshAfter {
		t.Fatalf("patch metadata = %+v", patch)
	}

	for _, alias := range []string{"midi.write_clip_notes", "midi.insert_notes", "midi.delete_notes", "midi.move_notes", "midi.resize_notes", "midi.quantize", "midi.transpose", "midi.set_velocity", "midi.replace_region"} {
		spec, ok := catalog.LookupTool(alias)
		if !ok {
			t.Fatalf("%s missing", alias)
		}
		if spec.CommandName != "apply_midi_note_patch" {
			t.Fatalf("%s command = %q", alias, spec.CommandName)
		}
	}

	for alias, command := range map[string]string{
		"midi.legacy_add_notes":      "add_midi_notes",
		"midi.legacy_add_notes_bulk": "add_midi_notes_bulk",
		"midi.legacy_mutate_notes":   "mutate_midi_notes",
		"midi.legacy_delete_notes":   "delete_midi_notes",
		"midi.delete_notes_legacy":   "delete_midi_notes",
	} {
		spec, ok := catalog.LookupTool(alias)
		if !ok {
			t.Fatalf("%s missing", alias)
		}
		if spec.CommandName != command {
			t.Fatalf("%s command = %q", alias, spec.CommandName)
		}
	}
}

func TestCoreMutationToolsExposeBindingMetadata(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool string
		key  string
		kind string
	}{
		{"track.add", "last_created_track", "track"},
		{"midi.create_clip", "last_created_clip", "clip"},
		{"midi.import_file", "last_created_clip", "clip"},
		{"clip.import_media_to_track", "last_created_clip", "clip"},
		{"plugin.load_to_rack", "last_loaded_plugin", "plugin"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if !hasBinding(spec.ProducedBindings, tt.key, tt.kind) {
			t.Fatalf("%s missing produced binding %s/%s: %+v", tt.tool, tt.key, tt.kind, spec.ProducedBindings)
		}
	}
}

func TestMixTickToolsAreAgentLayerCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	propose, ok := catalog.LookupTool("mix.propose_tick")
	if !ok {
		t.Fatal("mix.propose_tick missing")
	}
	if propose.CommandName != "mix_propose_tick" || propose.MutatesProject || propose.RequiresConfirmation || propose.RiskLevel != RiskDirect {
		t.Fatalf("propose metadata = %+v", propose)
	}

	apply, ok := catalog.LookupTool("mix.apply_tick")
	if !ok {
		t.Fatal("mix.apply_tick missing")
	}
	if apply.CommandName != "mix_apply_tick" || !apply.MutatesProject || !apply.SupportsUndo || apply.RiskLevel != RiskUndoable {
		t.Fatalf("apply metadata = %+v", apply)
	}

	rollback, ok := catalog.LookupTool("mix.rollback_tick")
	if !ok {
		t.Fatal("mix.rollback_tick missing")
	}
	if rollback.CommandName != "mix_rollback_tick" || !rollback.MutatesProject || !rollback.SupportsUndo || rollback.RiskLevel != RiskUndoable {
		t.Fatalf("rollback metadata = %+v", rollback)
	}
}

func TestAgentFoundationToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		domain  string
		confirm bool
	}{
		{"goal.status", "goal_status", "runtime", false},
		{"workspace.grep", "workspace_grep", "workspace", false},
		{"workspace.apply_edit", "workspace_apply_edit", "workspace", true},
		{"web.search", "web_search", "web", true},
		{"web.fetch", "web_fetch", "web", true},
		{"shell.run", "shell_run", "shell", true},
		{"version.checkpoint", "version_checkpoint", "version", true},
		{"version.node_checkout", "version_node_checkout", "version", true},
		{"version.worktree_checkout", "version_worktree_checkout", "version", true},
		{"agent.rollback_action", "agent_rollback_action", "runtime", true},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.Domain != tt.domain || spec.RequiresConfirmation != tt.confirm {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
	}
}

func hasBinding(bindings []BindingSpec, key, kind string) bool {
	for _, binding := range bindings {
		if binding.Key == key && binding.Kind == kind {
			return true
		}
	}
	return false
}

func TestModelSummaryIncludesToolAndArgumentHints(t *testing.T) {
	summary := DefaultCatalog().ModelSummary()
	for _, want := range []string{
		"set_mute tool=track.mute risk=undoable required=track_id args=mute:boolean",
		"rename_track tool=track.rename risk=undoable required=track_id args=name:string",
		"control_rename_macro tool=control.rename_macro risk=undoable required=- args=macro_id:string name:string",
		"delete_track tool=track.delete risk=confirm required=track_id args=track_id:string",
		"add_midi_notes_bulk tool=midi.legacy_add_notes_bulk risk=confirm required=track_id,clip_id args=clip_id:string optional track_id:string time_unit:beats notes:{pitch:number start:number length|duration:number velocity?:number id?:string}[]",
		"delete_midi_notes tool=midi.legacy_delete_notes risk=confirm required=track_id,clip_id args=clip_id:string optional track_id:string note_ids:string[]",
		"apply_midi_note_patch tool=midi.apply_note_patch risk=confirm required=clip_id args=clip_id:string optional track_id:string time_unit:beats operations:object[]",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("ModelSummary missing %q in:\n%s", want, summary)
		}
	}
}

func TestModelSummaryForToolsFiltersCatalog(t *testing.T) {
	summary := DefaultCatalog().ModelSummaryForTools([]string{"track.add", "midi.apply_note_patch"})
	for _, want := range []string{
		"add_track tool=track.add risk=undoable required=- args=optional name:string",
		"apply_midi_note_patch tool=midi.apply_note_patch risk=confirm required=clip_id args=clip_id:string optional track_id:string time_unit:beats operations:object[]",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("ModelSummaryForTools missing %q in:\n%s", want, summary)
		}
	}
	for _, unwanted := range []string{"plugin_search", "version_checkpoint", "delete_track"} {
		if strings.Contains(summary, unwanted) {
			t.Fatalf("ModelSummaryForTools leaked %q in:\n%s", unwanted, summary)
		}
	}
}

func TestModelSummaryForCapabilityPacksIncludesPackGuidance(t *testing.T) {
	summary := DefaultCatalog().ModelSummaryForCapabilityPacks([]string{"track"}, []string{"track.list", "track.add", "goal.status"})
	for _, want := range []string{
		"Capability pack: track",
		"State slices:",
		"Preconditions:",
		"Verification:",
		"add_track tool=track.add risk=undoable required=- args=optional name:string",
		"Shared support tools:",
		"goal_status tool=goal.status",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("Capability pack summary missing %q in:\n%s", want, summary)
		}
	}
	for _, unwanted := range []string{"plugin_search", "apply_midi_note_patch"} {
		if strings.Contains(summary, unwanted) {
			t.Fatalf("Capability pack summary leaked %q in:\n%s", unwanted, summary)
		}
	}
}

func TestModelSummaryForMediaCapabilityPack(t *testing.T) {
	summary := DefaultCatalog().ModelSummaryForCapabilityPacks([]string{"media"}, []string{
		"artifact.list",
		"media.register_assets",
		"media.index_authorized_folder",
	})
	for _, want := range []string{
		"Capability pack: media",
		"media_register_assets tool=media.register_assets",
		"media_index_authorized_folder tool=media.index_authorized_folder",
		"clickable preview cards",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("media capability summary missing %q in:\n%s", want, summary)
		}
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
