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

	folder, ok := catalog.LookupTool("track.folder.create")
	if !ok {
		t.Fatal("track.folder.create missing")
	}
	if folder.CommandName != "folder_track.create" || folder.RequiresConfirmation || folder.RiskLevel != RiskUndoable {
		t.Fatalf("track.folder.create metadata = %+v", folder)
	}

	applyOrg, ok := catalog.LookupTool("project.apply_track_organization")
	if !ok {
		t.Fatal("project.apply_track_organization missing")
	}
	if applyOrg.CommandName != "project.apply_track_organization" || !applyOrg.RequiresConfirmation || applyOrg.RiskLevel != RiskConfirm {
		t.Fatalf("project.apply_track_organization metadata = %+v", applyOrg)
	}
}

func TestProjectAudioToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		risk    RiskLevel
		mutates bool
		undo    bool
		confirm bool
		argHint string
	}{
		{"project.get_audio_settings", "project.get_audio_settings", RiskDirect, false, false, false, "no args"},
		{"project.set_audio_settings", "project.set_audio_settings", RiskUndoable, true, true, false, "audio_settings"},
		{"project.validate_audio_settings_change", "project.validate_audio_settings_change", RiskDirect, false, false, false, "record_bit_depth"},
		{"project.import_preflight", "project.import_preflight", RiskDirect, false, false, false, "stems_folder"},
		{"project.import_folder_as_stems", "project.import_folder_as_stems", RiskConfirm, true, true, true, "target_policy:create_tracks"},
		{"project.import_audio_files", "project.import_audio_files", RiskConfirm, true, true, true, "file_paths:string[]"},
		{"media.inspect_files", "media.inspect_files", RiskDirect, false, false, false, "file_paths"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.RiskLevel != tt.risk || spec.MutatesProject != tt.mutates || spec.SupportsUndo != tt.undo || spec.RequiresConfirmation != tt.confirm {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
		line := modelSummaryLine(spec)
		if !strings.Contains(line, tt.argHint) {
			t.Fatalf("%s arg hint missing %q in %q", tt.tool, tt.argHint, line)
		}
	}
}

func TestProjectMarkerToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		risk    RiskLevel
		mutates bool
		confirm bool
		argHint string
	}{
		{"project.markers.list", "project.markers.list", RiskDirect, false, false, "no args"},
		{"project.markers.upsert", "project.markers.upsert", RiskUndoable, true, false, "start_seconds"},
		{"project.markers.apply_section_markers", "project.markers.apply_section_markers", RiskConfirm, true, true, "sections"},
		{"project.markers.rename", "project.markers.rename", RiskUndoable, true, false, "marker_id"},
		{"project.markers.delete", "project.markers.delete", RiskConfirm, true, true, "marker_id"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.RiskLevel != tt.risk || spec.MutatesProject != tt.mutates || spec.RequiresConfirmation != tt.confirm {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
		line := modelSummaryLine(spec)
		if !strings.Contains(line, tt.argHint) {
			t.Fatalf("%s arg hint missing %q in %q", tt.tool, tt.argHint, line)
		}
	}
}

func TestTrackGroupToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		risk    RiskLevel
		mutates bool
		undo    bool
		confirm bool
		argHint string
	}{
		{"track.group.list", "track.group.list", RiskDirect, false, false, false, "no args"},
		{"track.group.create", "track.group.create", RiskUndoable, true, true, false, "track_ids:string[]"},
		{"track.group.update", "track.group.update", RiskUndoable, true, true, false, "group_id:string"},
		{"track.group.set_members", "track.group.set_members", RiskUndoable, true, true, false, "track_ids:string[]"},
		{"track.group.delete", "track.group.delete", RiskConfirm, true, true, true, "group_id:string"},
		{"track.group.apply_control", "track.group.apply_control", RiskConfirm, true, true, true, "mode:absolute|relative"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.RiskLevel != tt.risk || spec.MutatesProject != tt.mutates || spec.SupportsUndo != tt.undo || spec.RequiresConfirmation != tt.confirm || !spec.RefreshAfter && tt.mutates {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
		line := modelSummaryLine(spec)
		if !strings.Contains(line, tt.argHint) {
			t.Fatalf("%s arg hint missing %q in %q", tt.tool, tt.argHint, line)
		}
	}

	alias, ok := catalog.LookupTool("track_group.apply_control")
	if !ok || alias.CommandName != "track.group.apply_control" || !alias.RequiresConfirmation {
		t.Fatalf("track_group.apply_control alias metadata = %+v ok=%v", alias, ok)
	}
	if len(alias.RequiredTargetIDs) != 0 {
		t.Fatalf("track.group.apply_control should allow group_id OR track_ids, required IDs = %+v", alias.RequiredTargetIDs)
	}
}

func TestClipFadeGainToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		risk    RiskLevel
		mutates bool
		undo    bool
		confirm bool
		argHint string
	}{
		{"clip.fade.set", "clip.fade.set", RiskConfirm, true, true, true, "fade_in_seconds"},
		{"clip.fade.read", "clip.fade.read", RiskDirect, false, false, false, "clip_id:string"},
		{"clip.gain.set", "clip.gain.set", RiskConfirm, true, true, true, "gain_db"},
		{"clip.gain.set_batch", "clip.gain.set_batch", RiskConfirm, true, true, true, "pending_actions"},
		{"clip.gain.read", "clip.gain.read", RiskDirect, false, false, false, "clip_id:string"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.RiskLevel != tt.risk || spec.MutatesProject != tt.mutates || spec.SupportsUndo != tt.undo || spec.RequiresConfirmation != tt.confirm || !spec.RefreshAfter && tt.mutates {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
		line := modelSummaryLine(spec)
		if !strings.Contains(line, tt.argHint) {
			t.Fatalf("%s arg hint missing %q in %q", tt.tool, tt.argHint, line)
		}
	}
}

func TestClipStripSilenceToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	for _, tt := range []struct {
		tool    string
		command string
		risk    RiskLevel
		mutates bool
		undo    bool
		confirm bool
		argHint string
	}{
		{"clip.strip_silence.analyze", "clip.strip_silence.analyze", RiskDirect, false, false, false, "threshold_dbfs"},
		{"clip.strip_silence.suggest", "clip.strip_silence.suggest", RiskDirect, false, false, false, "candidate_thresholds_dbfs"},
		{"clip.strip_silence.apply", "clip.strip_silence.apply", RiskConfirm, true, true, true, "strip_regions"},
		{"clip.strip_silence.apply_batch", "clip.strip_silence.apply_batch", RiskConfirm, true, true, true, "pending_actions"},
	} {
		spec, ok := catalog.LookupTool(tt.tool)
		if !ok {
			t.Fatalf("%s missing", tt.tool)
		}
		if spec.CommandName != tt.command || spec.RiskLevel != tt.risk || spec.MutatesProject != tt.mutates || spec.SupportsUndo != tt.undo || spec.RequiresConfirmation != tt.confirm || !spec.RefreshAfter && tt.mutates {
			t.Fatalf("%s metadata = %+v", tt.tool, spec)
		}
		line := modelSummaryLine(spec)
		if !strings.Contains(line, tt.argHint) {
			t.Fatalf("%s arg hint missing %q in %q", tt.tool, tt.argHint, line)
		}
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
	applyEQEdits, ok := catalog.LookupTool("plugin_grabber.apply_eq_edits")
	if !ok {
		t.Fatal("plugin_grabber.apply_eq_edits missing")
	}
	if applyEQEdits.CommandName != "plugin_grabber_apply_eq_edits" || applyEQEdits.RequiresConfirmation ||
		applyEQEdits.RiskLevel != RiskUndoable || !applyEQEdits.SupportsUndo || !applyEQEdits.MutatesProject {
		t.Fatalf("apply EQ edits metadata = %+v", applyEQEdits)
	}
	if hint := argHint("plugin_grabber_apply_eq_edits"); !strings.Contains(hint, "edits:") ||
		!strings.Contains(hint, "operation_ref") {
		t.Fatalf("apply EQ edits argument hint = %q", hint)
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
		{"project.import_folder_as_stems", "last_created_track", "track"},
		{"project.import_folder_as_stems", "last_created_clip", "clip"},
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

func TestStaticBalanceCapabilityPackExposesMOMInsteadOfDirectDAD(t *testing.T) {
	packs := DefaultCatalog().CapabilityPacks([]string{"static_mix_static_balance"})
	if len(packs) != 1 {
		t.Fatalf("static balance capability packs = %+v", packs)
	}
	pack := packs[0]
	for _, tool := range pack.Tools {
		if tool == "project.audio_analysis_status" {
			t.Fatalf("B2 capability pack exposes direct DAD tool: %+v", pack.Tools)
		}
	}
	joined := strings.ToLower(pack.Description + " " + strings.Join(pack.StateSlices, " "))
	if !strings.Contains(joined, "static_level_relationship") || !strings.Contains(joined, "raw dad rows remain behind mom") {
		t.Fatalf("B2 capability pack does not describe the MOM observation boundary: %s", joined)
	}
}

func TestModelSummaryForMediaCapabilityPack(t *testing.T) {
	summary := DefaultCatalog().ModelSummaryForCapabilityPacks([]string{"media"}, []string{
		"artifact.list",
		"media.register_assets",
		"media.index_authorized_folder",
		"project.import_preflight",
		"media.inspect_files",
	})
	for _, want := range []string{
		"Capability pack: media",
		"media_register_assets tool=media.register_assets",
		"media_index_authorized_folder tool=media.index_authorized_folder",
		"project.import_preflight tool=project.import_preflight",
		"media.inspect_files tool=media.inspect_files",
		"clickable preview cards",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("media capability summary missing %q in:\n%s", want, summary)
		}
	}
}

func TestModelSummaryForProjectAudioCapabilityPack(t *testing.T) {
	summary := DefaultCatalog().ModelSummaryForCapabilityPacks([]string{"project_audio"}, []string{
		"project.state",
		"project.get_audio_settings",
		"project.set_audio_settings",
		"project.validate_audio_settings_change",
		"project.import_preflight",
		"media.inspect_files",
	})
	for _, want := range []string{
		"Capability pack: project_audio",
		"not audio-device configuration",
		"project.get_audio_settings tool=project.get_audio_settings",
		"project.set_audio_settings tool=project.set_audio_settings risk=undoable",
		"project.import_preflight tool=project.import_preflight",
		"do not simulate folder import by repeated clip.import_audio",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("project audio capability summary missing %q in:\n%s", want, summary)
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
