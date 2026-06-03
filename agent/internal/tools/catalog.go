package tools

import (
	"fmt"
	"sort"
	"strings"
)

type RiskLevel string

const (
	RiskDirect   RiskLevel = "direct"
	RiskUndoable RiskLevel = "undoable"
	RiskConfirm  RiskLevel = "confirm"
)

type BindingSpec struct {
	Key        string   `json:"key,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	Arg        string   `json:"arg,omitempty"`
	ResultKeys []string `json:"result_keys,omitempty"`
}

type CommandSpec struct {
	CommandName          string         `json:"command_name"`
	ToolName             string         `json:"tool_name"`
	Namespace            string         `json:"namespace"`
	Domain               string         `json:"domain"`
	Category             string         `json:"category"`
	Description          string         `json:"description"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	RiskLevel            RiskLevel      `json:"risk_level"`
	MutatesProject       bool           `json:"mutates_project"`
	SupportsUndo         bool           `json:"supports_undo"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	RequiredTargetIDs    []string       `json:"required_target_ids,omitempty"`
	ProducedBindings     []BindingSpec  `json:"produced_bindings,omitempty"`
	ConsumedBindings     []BindingSpec  `json:"consumed_bindings,omitempty"`
	RefreshAfter         bool           `json:"refresh_after"`
	Implemented          bool           `json:"implemented"`
}

type Tool struct {
	Name                 string         `json:"name"`
	Namespace            string         `json:"namespace"`
	Domain               string         `json:"domain"`
	Category             string         `json:"category"`
	Description          string         `json:"description"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	RiskLevel            RiskLevel      `json:"risk_level"`
	MutatesProject       bool           `json:"mutates_project"`
	SupportsUndo         bool           `json:"supports_undo"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	RequiredTargetIDs    []string       `json:"required_target_ids,omitempty"`
	ProducedBindings     []BindingSpec  `json:"produced_bindings,omitempty"`
	ConsumedBindings     []BindingSpec  `json:"consumed_bindings,omitempty"`
	KernelCommand        string         `json:"kernel_command"`
	RefreshAfter         bool           `json:"refresh_after"`
	Implemented          bool           `json:"implemented"`
}

type Catalog struct {
	commands map[string]CommandSpec
	tools    map[string]CommandSpec
}

func DefaultCatalog() *Catalog {
	c := NewCatalog()
	for _, spec := range defaultSpecs() {
		c.Add(spec)
	}
	c.AddAlias("plugin_get_parameters", "get_plugin_parameters")
	c.AddAlias("plugin_set_parameter", "set_plugin_param")
	c.AddAlias("plugin_set_aliases", "set_plugin_param_aliases")
	c.AddAlias("plugin_explain_controls", "plugin_grabber_explain_controls")
	c.AddAlias("plugin.explain_controls", "plugin_grabber_explain_controls")
	c.AddAlias("plugin_learn_common_roles", "set_plugin_param_aliases")
	c.AddAlias("plugin.list", "plugin_list_available")
	c.AddAlias("plugin.find", "plugin_search")
	c.AddAlias("plugin.load_to_rack", "rack_add_node")
	c.AddAlias("plugin.semantic.build_index", "plugin_semantic_build_index")
	c.AddAlias("plugin.semantic.search", "plugin_semantic_search")
	c.AddAlias("plugin.semantic.get", "plugin_semantic_get")
	c.AddAlias("plugin_grabber.list_project_profiles", "plugin_grabber_get_project_profiles")
	c.AddAlias("plugin_grabber.save_project_profile", "plugin_grabber_upsert_project_profile")
	c.AddAlias("plugin_grabber.reset_project_profile", "plugin_grabber_remove_project_profile")
	c.AddAlias("midi.read_clip_notes", "get_midi_clip_notes")
	c.AddAlias("midi.import_file", "import_midi_to_track")
	c.AddAlias("midi.write_clip_notes", "apply_midi_note_patch")
	c.AddAlias("midi.apply_note_patch", "apply_midi_note_patch")
	c.AddAlias("midi.insert_notes", "apply_midi_note_patch")
	c.AddAlias("midi.delete_notes", "apply_midi_note_patch")
	c.AddAlias("midi.move_notes", "apply_midi_note_patch")
	c.AddAlias("midi.resize_notes", "apply_midi_note_patch")
	c.AddAlias("midi.quantize", "apply_midi_note_patch")
	c.AddAlias("midi.transpose", "apply_midi_note_patch")
	c.AddAlias("midi.set_velocity", "apply_midi_note_patch")
	c.AddAlias("midi.replace_region", "apply_midi_note_patch")
	c.AddAlias("midi.add_notes", "add_midi_notes")
	c.AddAlias("midi.add_notes_bulk", "add_midi_notes_bulk")
	c.AddAlias("midi.mutate_notes", "mutate_midi_notes")
	c.AddAlias("midi.delete_notes_legacy", "delete_midi_notes")
	return c
}

func NewCatalog() *Catalog {
	return &Catalog{
		commands: map[string]CommandSpec{},
		tools:    map[string]CommandSpec{},
	}
}

func (c *Catalog) Add(spec CommandSpec) {
	if c == nil {
		return
	}
	spec.CommandName = strings.TrimSpace(spec.CommandName)
	spec.ToolName = strings.TrimSpace(spec.ToolName)
	spec.Namespace = strings.TrimSpace(spec.Namespace)
	if spec.CommandName == "" {
		return
	}
	if spec.ToolName == "" {
		spec.ToolName = "daw." + spec.CommandName
	}
	if spec.Namespace == "" {
		spec.Namespace = namespaceOf(spec.ToolName)
	}
	if spec.Domain == "" {
		spec.Domain = domainOf(spec.ToolName, spec.Category)
	}
	if spec.InputSchema == nil {
		spec.InputSchema = commandInputSchema(spec.CommandName)
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = basicOutputSchema()
	}
	spec.Implemented = true
	c.commands[spec.CommandName] = spec
	c.tools[spec.ToolName] = spec
}

func (c *Catalog) AddAlias(toolName, commandName string) {
	if c == nil {
		return
	}
	spec, ok := c.LookupCommand(commandName)
	if !ok {
		return
	}
	spec.ToolName = strings.TrimSpace(toolName)
	spec.Namespace = namespaceOf(spec.ToolName)
	c.tools[spec.ToolName] = spec
}

func (c *Catalog) LookupCommand(name string) (CommandSpec, bool) {
	if c == nil {
		return CommandSpec{}, false
	}
	spec, ok := c.commands[strings.TrimSpace(name)]
	return spec, ok
}

func (c *Catalog) LookupTool(name string) (CommandSpec, bool) {
	if c == nil {
		return CommandSpec{}, false
	}
	spec, ok := c.tools[strings.TrimSpace(name)]
	return spec, ok
}

func (c *Catalog) Commands() []CommandSpec {
	if c == nil {
		return nil
	}
	out := make([]CommandSpec, 0, len(c.commands))
	for _, spec := range c.commands {
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CommandName < out[j].CommandName
	})
	return out
}

func (c *Catalog) Tools() []Tool {
	if c == nil {
		return nil
	}
	out := make([]Tool, 0, len(c.tools))
	for _, spec := range c.tools {
		out = append(out, Tool{
			Name:                 spec.ToolName,
			Namespace:            spec.Namespace,
			Domain:               spec.Domain,
			Category:             spec.Category,
			Description:          spec.Description,
			InputSchema:          spec.InputSchema,
			OutputSchema:         spec.OutputSchema,
			RiskLevel:            spec.RiskLevel,
			MutatesProject:       spec.MutatesProject,
			SupportsUndo:         spec.SupportsUndo,
			RequiresConfirmation: spec.RequiresConfirmation,
			RequiredTargetIDs:    append([]string(nil), spec.RequiredTargetIDs...),
			ProducedBindings:     append([]BindingSpec(nil), spec.ProducedBindings...),
			ConsumedBindings:     append([]BindingSpec(nil), spec.ConsumedBindings...),
			KernelCommand:        spec.CommandName,
			RefreshAfter:         spec.RefreshAfter,
			Implemented:          spec.Implemented,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (c *Catalog) DirectCommandNames() []string {
	if c == nil {
		return nil
	}
	var names []string
	for _, spec := range c.commands {
		if !spec.RequiresConfirmation {
			names = append(names, spec.CommandName)
		}
	}
	sort.Strings(names)
	return names
}

func (c *Catalog) ModelSummary() string {
	if c == nil {
		return ""
	}
	var parts []string
	for _, spec := range c.Commands() {
		required := "-"
		if len(spec.RequiredTargetIDs) > 0 {
			required = strings.Join(spec.RequiredTargetIDs, ",")
		}
		parts = append(parts, fmt.Sprintf("%s tool=%s risk=%s required=%s args=%s", spec.CommandName, spec.ToolName, modelRisk(spec), required, argHint(spec.CommandName)))
	}
	return strings.Join(parts, "\n")
}

func modelRisk(spec CommandSpec) string {
	if spec.RequiresConfirmation || spec.RiskLevel == RiskConfirm {
		return "confirm"
	}
	if spec.RiskLevel == RiskUndoable {
		return "undoable"
	}
	return "direct"
}

func argHint(commandName string) string {
	switch commandName {
	case "rename_track":
		return "name:string"
	case "set_mute":
		return "mute:boolean"
	case "set_solo":
		return "solo:boolean"
	case "arm_track":
		return "is_armed:boolean"
	case "add_track", "add_audio_track":
		return "optional name:string"
	case "delete_track":
		return "track_id:string"
	case "set_tempo":
		return "bpm:number"
	case "seek":
		return "time:number_seconds"
	case "set_click":
		return "enabled:boolean"
	case "route_wave_input_to_track":
		return "track_id:string device_id:string"
	case "import_audio", "import_media_to_track":
		return "track_id:string file_path:string optional start_time:number asset_query:string"
	case "move_clip":
		return "clip_id:string source_track_id:string target_track_id:string new_start:number_seconds"
	case "resize_clip":
		return "clip_id:string optional track_id:string new_length:number_seconds"
	case "split_clip":
		return "clip_id:string optional track_id:string split_time:number_seconds"
	case "clone_clip":
		return "source_clip_id:string target_track_id:string time_unit:string new_start:number"
	case "remove_clips":
		return "clip_ids:string[]"
	case "select_clip":
		return "clip_id:string optional track_id:string clip_name:string clip_index:number"
	case "get_plugin_parameters", "open_plugin_ui", "show_plugin_editor":
		return "track_id:string plugin_id:string"
	case "plugin_list_available":
		return "optional format:string limit:number"
	case "plugin_search":
		return "query:string optional format:string limit:number"
	case "plugin_semantic_build_index":
		return "optional limit:number force:boolean"
	case "plugin_semantic_search":
		return "query:string optional type:string manufacturer:string format:string limit:number"
	case "plugin_semantic_get":
		return "id:string OR query:string"
	case "scan_plugins":
		return "optional paths:string[] path:string use_default_paths:boolean"
	case "rack_add_node":
		return "track_id:string plugin_path:string optional x:number y:number zone_id:string"
	case "set_plugin_param":
		return "track_id:string plugin_id:string param_id:string value:number"
	case "plugin_grabber_get_project_profiles":
		return "optional profile_id:string plugin_id:string"
	case "plugin_grabber_explain_controls":
		return "track_id:string plugin_id:string optional intent:string"
	case "plugin_grabber_upsert_project_profile":
		return "track_id:string plugin_id:string quick_control_ids:string[] aliases?:object display_groups?:object normalized_roles?:object"
	case "plugin_grabber_remove_project_profile":
		return "profile_id:string OR track_id:string plugin_id:string"
	case "import_midi_to_track":
		return "track_id:string file_path:string optional start_time_beats:number mode:merge_tracks"
	case "get_midi_clip_notes", "get_midi_clip_data":
		return "clip_id:string optional track_id:string"
	case "add_midi_notes", "add_midi_notes_bulk":
		return "clip_id:string optional track_id:string time_unit:beats notes:{pitch:number start:number length|duration:number velocity?:number id?:string}[]"
	case "mutate_midi_notes":
		return "clip_id:string optional track_id:string time_unit:beats notes:{id|note_id:string optional pitch/start/length|duration/velocity}[]"
	case "delete_midi_notes":
		return "clip_id:string optional track_id:string note_ids:string[]"
	case "apply_midi_note_patch":
		return "clip_id:string optional track_id:string time_unit:beats operations:object[]"
	default:
		return "-"
	}
}

func CommandName(cmd map[string]any) string {
	for _, key := range []string{"cmd", "action", "command"} {
		if v, ok := cmd[key]; ok {
			name := strings.TrimSpace(fmt.Sprint(v))
			if name != "" {
				return name
			}
		}
	}
	return ""
}

func BuildCommand(commandName string, args map[string]any) map[string]any {
	out := cloneMap(args)
	out["cmd"] = strings.TrimSpace(commandName)
	return out
}

func CloneCommand(in map[string]any) map[string]any {
	return cloneMap(in)
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func namespaceOf(toolName string) string {
	if idx := strings.Index(toolName, "."); idx > 0 {
		return toolName[:idx]
	}
	return "daw"
}

func domainOf(toolName, category string) string {
	ns := namespaceOf(toolName)
	switch ns {
	case "workspace":
		return "workspace"
	case "logs":
		return "workspace"
	case "web":
		return "web"
	case "shell":
		return "shell"
	case "version":
		return "version"
	case "goal", "runtime":
		return "runtime"
	case "agent":
		if strings.Contains(toolName, "rollback") {
			return "runtime"
		}
	}
	if category == "project_history" {
		return "version"
	}
	return "daw"
}

func commandInputSchema(commandName string) map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Kernel IPC command payload. Extra fields are passed through to VitApp.",
		"required":    []string{"cmd"},
		"properties": map[string]any{
			"cmd": map[string]any{"type": "string", "const": commandName},
		},
	}
}

func basicOutputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Kernel JSON reply wrapped with agent action metadata by VitAgent.",
	}
}

func spec(command, tool, category, description string, risk RiskLevel, mutates, undo, confirm, refresh bool, ids ...string) CommandSpec {
	return CommandSpec{
		CommandName:          command,
		ToolName:             tool,
		Category:             category,
		Description:          description,
		RiskLevel:            risk,
		MutatesProject:       mutates,
		SupportsUndo:         undo,
		RequiresConfirmation: confirm,
		RequiredTargetIDs:    ids,
		RefreshAfter:         refresh,
	}
}

func defaultSpecs() []CommandSpec {
	specs := []CommandSpec{
		spec("ping", "project.ping", "project", "Check whether the kernel command path is alive.", RiskDirect, false, false, false, false),
		spec("get_project_state", "project.state", "project", "Refresh agent shadow from the kernel and return the user-visible project state; internal engine tracks stay hidden.", RiskDirect, false, false, false, false),
		spec("list_tracks", "track.list", "track", "Read the user-visible editable track list.", RiskDirect, false, false, false, false),
		spec("project_health_check", "project.health", "project", "Run kernel project health diagnostics.", RiskDirect, false, false, false, false),
		spec("get_recent_projects", "project.recent", "project", "Read recent project entries.", RiskDirect, false, false, false, false),
		spec("undo", "project.undo", "project", "Undo the latest kernel edit transaction.", RiskUndoable, true, true, false, true),
		spec("redo", "project.redo", "project", "Redo the latest kernel edit transaction.", RiskUndoable, true, true, false, true),
		spec("save_project", "project.save", "project", "Save the active project file.", RiskConfirm, true, false, true, false),
		spec("save_as_project", "project.save_as", "project", "Save the project to a new path.", RiskConfirm, true, false, true, false),
		spec("open_project", "project.open", "project", "Open an existing project.", RiskConfirm, true, false, true, true),
		spec("load_project", "project.load", "project", "Load an existing project.", RiskConfirm, true, false, true, true),
		spec("reload_project", "project.reload", "project", "Reload the current project.", RiskConfirm, true, false, true, true),
		spec("new_project", "project.new", "project", "Create a new project.", RiskConfirm, true, false, true, true),
		spec("clear_project", "project.clear", "project", "Clear the current project.", RiskConfirm, true, true, true, true),
		spec("project_snapshot_export", "project.snapshot_export", "project", "Export the active in-memory project snapshot without saving the live .vit file.", RiskDirect, false, false, false, false),
		spec("project_undo_state", "project.undo_state", "project", "Read whether the active project can undo or redo.", RiskDirect, false, false, false, false),

		spec("goal_status", "goal.status", "runtime", "Read the current agent goal runtime status.", RiskDirect, false, false, false, false),
		spec("goal_cancel", "goal.cancel", "runtime", "Request cancellation of the active or named agent goal.", RiskDirect, false, false, false, false),
		spec("goal_tick", "goal.tick", "runtime", "Advance a goal runtime checkpoint; intended for internal orchestration.", RiskDirect, false, false, false, false),

		spec("workspace_glob", "workspace.glob", "workspace", "List files under an allowed workspace root using a bounded pattern search.", RiskDirect, false, false, false, false),
		spec("workspace_grep", "workspace.grep", "workspace", "Search text files under an allowed workspace root with bounded results.", RiskDirect, false, false, false, false),
		spec("workspace_read_file", "workspace.read_file", "workspace", "Read a text file from an allowed workspace root.", RiskDirect, false, false, false, false),
		spec("workspace_file_info", "workspace.file_info", "workspace", "Inspect metadata for a file under an allowed workspace root.", RiskDirect, false, false, false, false),
		spec("logs_read", "logs.read", "workspace", "Read recent Vit Agent/VitApp log lines from allowed log roots.", RiskDirect, false, false, false, false),
		spec("logs_search", "logs.search", "workspace", "Search Vit Agent/VitApp logs from allowed log roots.", RiskDirect, false, false, false, false),
		spec("workspace_edit_preview", "workspace.edit_preview", "workspace", "Preview a text replacement for an allowed workspace file.", RiskDirect, false, false, false, false),
		spec("workspace_apply_edit", "workspace.apply_edit", "workspace", "Apply a confirmed text replacement to an allowed workspace file.", RiskConfirm, false, false, true, false),
		spec("workspace_write_file", "workspace.write_file", "workspace", "Write a confirmed text file under an allowed workspace root.", RiskConfirm, false, false, true, false),
		spec("workspace_blob_info", "workspace.blob_info", "workspace", "Inspect hash and metadata for a binary/blob file under an allowed root.", RiskDirect, false, false, false, false),
		spec("workspace_blob_copy", "workspace.blob_copy", "workspace", "Copy a confirmed binary/blob file between allowed workspace roots.", RiskConfirm, false, false, true, false),

		spec("web_search", "web.search", "web", "Search the public web with confirmation and privacy limits.", RiskConfirm, false, false, true, false),
		spec("web_fetch", "web.fetch", "web", "Fetch a public http/https URL with confirmation and network safety limits.", RiskConfirm, false, false, true, false),
		spec("shell_run", "shell.run", "shell", "Run an allowlisted shell command with confirmation.", RiskConfirm, false, false, true, false),
		spec("agent_rollback_action", "agent.rollback_action", "runtime", "Rollback a journaled agent action through its owning undo domain.", RiskConfirm, false, false, true, true),

		spec("version_status", "version.status", "project_history", "Read Project History status for the active project.", RiskDirect, false, false, false, false),
		spec("version_checkpoint", "version.checkpoint", "project_history", "Create a local Project History checkpoint using content-addressed storage.", RiskConfirm, false, false, true, false),
		spec("version_list", "version.list", "project_history", "List Project History checkpoints and refs.", RiskDirect, false, false, false, false),
		spec("version_show", "version.show", "project_history", "Show one Project History checkpoint manifest.", RiskDirect, false, false, false, false),
		spec("version_diff", "version.diff", "project_history", "Compare Project History checkpoints or current working files.", RiskDirect, false, false, false, false),
		spec("version_restore_preview", "version.restore_preview", "project_history", "Preview restoring a Project History checkpoint.", RiskDirect, false, false, false, false),
		spec("version_restore", "version.restore", "project_history", "Restore a confirmed Project History checkpoint into the active project folder.", RiskConfirm, false, false, true, true),
		spec("version_branch_create", "version.branch_create", "project_history", "Create and checkout a local Project History branch ref.", RiskConfirm, false, false, true, true),
		spec("version_node_checkout", "version.node_checkout", "project_history", "Checkout a confirmed Project History conversation node into the active project folder.", RiskConfirm, false, false, true, true),
		spec("version_node_delete", "version.node_delete", "project_history", "Delete a confirmed Project History conversation node subtree.", RiskConfirm, false, false, true, true),
		spec("version_worktree_create", "version.worktree_create", "project_history", "Materialize a local Project History worktree.", RiskConfirm, false, false, true, false),
		spec("version_worktree_checkout", "version.worktree_checkout", "project_history", "Open a confirmed Project History worktree in this Vit window.", RiskConfirm, false, false, true, true),
		spec("version_worktree_list", "version.worktree_list", "project_history", "List local Project History worktrees.", RiskDirect, false, false, false, false),
		spec("version_project_saved", "version.project_saved", "project_history", "Adopt draft Project History after an unsaved project is saved to a real path.", RiskDirect, false, false, false, false),
		spec("version_checkout", "version.checkout", "project_history", "Checkout a confirmed Project History branch or checkpoint into the active project folder.", RiskConfirm, false, false, true, true),

		spec("play", "transport.play", "transport", "Start playback.", RiskDirect, false, false, false, false),
		spec("stop", "transport.stop", "transport", "Stop playback.", RiskDirect, false, false, false, false),
		spec("return_to_zero", "transport.return_to_zero", "transport", "Stop and return the playhead to the project start.", RiskDirect, false, false, false, false),
		spec("transport_option_stop_return_to_start", "transport.option_stop_return_to_start", "transport", "Toggle the stop-return-to-start transport option.", RiskDirect, false, false, false, false),
		spec("seek", "transport.seek", "transport", "Move the playhead.", RiskDirect, false, false, false, false),
		spec("toggle_click", "transport.toggle_click", "transport", "Toggle the metronome click.", RiskDirect, false, false, false, false),
		spec("set_click", "transport.set_click", "transport", "Set metronome click state.", RiskDirect, false, false, false, false),
		spec("set_tempo", "transport.set_tempo", "transport", "Set project tempo.", RiskUndoable, true, true, false, true),
		spec("start_recording", "transport.record.start", "transport", "Start recording.", RiskDirect, false, false, false, false),
		spec("stop_recording", "transport.record.stop", "transport", "Stop recording.", RiskDirect, false, false, false, true),

		spec("get_audio_device_types", "audio.get_device_types", "audio", "List available audio device backends.", RiskDirect, false, false, false, false),
		spec("get_audio_devices", "audio.get_devices", "audio", "List audio devices for a backend.", RiskDirect, false, false, false, false),
		spec("set_audio_device", "audio.set_device", "audio", "Switch audio device.", RiskConfirm, true, false, true, false),
		spec("get_wave_input_devices", "audio.get_wave_input_devices", "audio", "List wave input devices.", RiskDirect, false, false, false, false),
		spec("route_wave_input_to_track", "audio.route_wave_input_to_track", "audio", "Route a wave input to a track.", RiskUndoable, true, true, false, true, "track_id", "device_id"),
		spec("arm_track", "track.arm", "track", "Arm or disarm a track for recording.", RiskUndoable, true, true, false, true, "track_id"),

		spec("add_track", "track.add", "track", "Add a new track.", RiskUndoable, true, true, false, true),
		spec("add_audio_track", "track.add_audio", "track", "Add a new audio track.", RiskUndoable, true, true, false, true),
		spec("append_ghost_track", "track.append_ghost", "track", "Append a ghost track placeholder.", RiskUndoable, true, true, false, true),
		spec("delete_track", "track.delete", "track", "Delete a track by stable track_id.", RiskConfirm, true, true, true, true, "track_id"),
		spec("rename_track", "track.rename", "track", "Rename a track by stable track_id.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_mute", "track.mute", "track", "Set track mute state.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_solo", "track.solo", "track", "Set track solo state.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_volume", "track.volume", "track", "Set track volume in dB.", RiskUndoable, true, true, false, true, "track_id"),
		spec("freeze_track", "track.freeze", "track", "Freeze a track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("unfreeze_track", "track.unfreeze", "track", "Unfreeze a track.", RiskConfirm, true, true, true, true, "track_id"),

		spec("add_audio_clip", "clip.add_audio", "clip", "Add an audio clip to a known track and start time.", RiskConfirm, true, true, true, true, "track_id"),
		spec("import_audio", "clip.import_audio", "clip", "Import audio to a target track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("import_media_to_track", "clip.import_media_to_track", "clip", "Import media to a target track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("move_clip", "clip.move", "clip", "Move a clip by stable track and clip IDs.", RiskConfirm, true, true, true, true, "source_track_id", "target_track_id", "clip_id"),
		spec("resize_clip", "clip.resize", "clip", "Resize a clip by stable clip ID.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("split_clip", "clip.split", "clip", "Split a clip at a timeline position.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("clone_clip", "clip.clone", "clip", "Clone an existing clip.", RiskConfirm, true, true, true, true, "source_clip_id", "target_track_id"),
		spec("remove_clips", "clip.remove", "clip", "Remove one or more clips.", RiskConfirm, true, true, true, true, "clip_ids"),
		spec("select_clip", "clip.select", "clip", "Select a user-visible clip in the UI without changing the project.", RiskDirect, false, false, false, false, "track_id", "clip_id"),
		spec("warm_waveform_bake", "clip.warm_waveform_bake", "clip", "Request waveform/tile preparation.", RiskDirect, false, false, false, false),

		spec("get_midi_clip_notes", "midi.read_notes", "midi", "Read MIDI notes from a clip.", RiskDirect, false, false, false, false, "clip_id"),
		spec("get_midi_clip_data", "midi.read_clip_data", "midi", "Read MIDI clip data.", RiskDirect, false, false, false, false, "clip_id"),
		spec("import_midi_to_track", "midi.import_file", "midi", "Import a Standard MIDI File into a target track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("insert_midi_clip", "midi.insert_clip", "midi", "Insert a MIDI clip.", RiskConfirm, true, true, true, true, "track_id"),
		spec("create_midi_clip", "midi.create_clip", "midi", "Create a MIDI clip.", RiskConfirm, true, true, true, true, "track_id"),
		spec("add_midi_notes", "midi.legacy_add_notes", "midi", "Add MIDI notes to a clip using the legacy compatibility path.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("add_midi_notes_bulk", "midi.legacy_add_notes_bulk", "midi", "Add many MIDI notes to a clip using the legacy compatibility path.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("mutate_midi_notes", "midi.legacy_mutate_notes", "midi", "Modify existing MIDI notes using the legacy compatibility path.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("delete_midi_notes", "midi.legacy_delete_notes", "midi", "Delete MIDI notes using the legacy compatibility path.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("apply_midi_note_patch", "midi.apply_note_patch", "midi", "Apply a beat-based MIDI note patch to a clip.", RiskConfirm, true, true, true, true, "clip_id"),

		spec("plugin_list_available", "plugin.list_available", "plugin", "List indexed plugins with loadable paths.", RiskDirect, false, false, false, false),
		spec("plugin_search", "plugin.search", "plugin", "Search indexed plugins and return loadable candidates.", RiskDirect, false, false, false, false),
		spec("plugin_semantic_build_index", "plugin.semantic_build_index", "plugin", "Build the local plugin semantic library from the indexed plugin list.", RiskConfirm, false, false, true, false),
		spec("plugin_semantic_search", "plugin.semantic_search", "plugin", "Search the local plugin semantic library by role, type, maker, or name.", RiskDirect, false, false, false, false),
		spec("plugin_semantic_get", "plugin.semantic_get", "plugin", "Read one local plugin semantic library entry.", RiskDirect, false, false, false, false),
		spec("scan_plugins", "plugin.scan", "plugin", "Scan plugin folders and refresh the indexed plugin list.", RiskConfirm, true, false, true, false),
		spec("instantiate_plugin", "plugin.instantiate", "plugin", "Instantiate a plugin on a track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("open_plugin_ui", "plugin.open", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("show_plugin_editor", "plugin.show_editor", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("get_plugin_parameters", "plugin.get_parameters", "plugin", "Read normalized plugin parameter values.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("set_plugin_param", "plugin.set_parameter", "plugin", "Set one plugin parameter.", RiskUndoable, true, true, false, true, "track_id", "plugin_id", "param_id"),
		spec("set_plugin_param_aliases", "plugin.set_aliases", "plugin", "Store semantic aliases for plugin parameters.", RiskUndoable, true, true, false, true, "plugin_id"),
		spec("plugin_grabber_get_project_profiles", "plugin_grabber.get_project_profiles", "plugin_grabber", "Read project-scoped plugin grabber profiles.", RiskDirect, false, false, false, false),
		spec("plugin_grabber_explain_controls", "plugin_grabber.explain_controls", "plugin_grabber", "Build a compact AI-friendly context pack for one loaded plugin without filtering full parameters.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("plugin_grabber_upsert_project_profile", "plugin_grabber.upsert_project_profile", "plugin_grabber", "Save project-scoped plugin grabber profile annotations for one plugin.", RiskConfirm, true, false, true, true, "track_id", "plugin_id"),
		spec("plugin_grabber_remove_project_profile", "plugin_grabber.remove_project_profile", "plugin_grabber", "Remove a project-scoped plugin grabber profile.", RiskConfirm, true, false, true, true),
		spec("delete_plugin", "plugin.delete", "plugin", "Delete a plugin instance.", RiskConfirm, true, true, true, true, "track_id", "plugin_id"),
		spec("move_plugin", "plugin.move", "plugin", "Move a plugin instance.", RiskConfirm, true, true, true, true, "track_id", "plugin_id"),

		spec("rack_add_node", "rack.add_node", "rack", "Load a plugin as a graph rack node.", RiskConfirm, true, true, true, true, "track_id"),
		spec("rack_connect_pins", "rack.connect_pins", "rack", "Connect rack pins.", RiskConfirm, true, true, true, true),
		spec("rack_remove_connection", "rack.remove_connection", "rack", "Remove a rack connection.", RiskConfirm, true, true, true, true),
		spec("rack_set_node_clip_scope", "rack.set_node_clip_scope", "rack", "Set rack node clip scope.", RiskConfirm, true, true, true, true),
		spec("rack_add_edge", "rack.add_edge", "rack", "Connect rack edge alias.", RiskConfirm, true, true, true, true),
		spec("rack_remove_edge", "rack.remove_edge", "rack", "Remove rack edge alias.", RiskConfirm, true, true, true, true),
		spec("control_add_node", "plugin_create_control_graph_node", "plugin_grabber", "Create a control graph node.", RiskConfirm, true, true, true, true),
		spec("control_update_node", "control.update_node", "control", "Update a control graph node.", RiskConfirm, true, true, true, true),
		spec("control_remove_node", "control.remove_node", "control", "Remove a control graph node.", RiskConfirm, true, true, true, true),
		spec("control_add_macro", "plugin_map_macro_to_params", "plugin_grabber", "Create a macro control.", RiskConfirm, true, true, true, true),
		spec("control_add_binding", "control.add_binding", "control", "Bind a macro/control to a plugin parameter.", RiskConfirm, true, true, true, true),
		spec("control_update_binding", "control.update_binding", "control", "Update a control binding.", RiskConfirm, true, true, true, true),
		spec("control_remove_binding", "control.remove_binding", "control", "Remove a control binding.", RiskConfirm, true, true, true, true),
		spec("control_set_node_value", "control.set_node_value", "control", "Set a control node value.", RiskUndoable, true, true, false, true),
		spec("control_set_macro_values", "control.set_macro_values", "control", "Set macro values.", RiskUndoable, true, true, false, true),
		spec("connector_upsert_profile", "plugin_make_grabber_profile", "plugin_grabber", "Create or update a connector/grabber profile.", RiskConfirm, true, true, true, true),
		spec("connector_remove_profile", "plugin.remove_grabber_profile", "plugin_grabber", "Remove a connector/grabber profile.", RiskConfirm, true, true, true, true),

		spec("aigc_register_job", "assets.register_job", "assets", "Register an AIGC job with the project.", RiskUndoable, true, true, false, true),
		spec("bridge_ingest_generated_asset", "assets.ingest_generated_asset", "assets", "Ingest a generated asset into the project.", RiskConfirm, true, true, true, true),
		spec("switch_asset_take", "assets.switch_take", "assets", "Switch the active generated asset take.", RiskConfirm, true, true, true, true),
		spec("set_async_ghost_state", "assets.set_async_ghost_state", "assets", "Set async ghost asset state.", RiskUndoable, true, true, false, true),
		spec("start_render", "render.start", "render", "Start an offline render job.", RiskConfirm, true, false, true, false),
		spec("cancel_render", "render.cancel", "render", "Cancel the current render job.", RiskDirect, false, false, false, false),
	}
	return withDefaultBindingSpecs(specs)
}

func withDefaultBindingSpecs(specs []CommandSpec) []CommandSpec {
	for i := range specs {
		switch specs[i].CommandName {
		case "add_track", "add_audio_track":
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_track", Kind: "track", ResultKeys: []string{"track_id", "id", "item_id"}},
				{Key: "active_work_target_track", Kind: "track", ResultKeys: []string{"track_id", "id", "item_id"}},
			}
		case "create_midi_clip", "insert_midi_clip", "import_midi_to_track", "import_audio", "import_media_to_track", "add_audio_clip":
			specs[i].ConsumedBindings = []BindingSpec{{Key: "target_track", Kind: "track", Arg: "track_id"}}
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_clip", Kind: "clip", ResultKeys: []string{"clip_id", "id", "item_id"}},
				{Key: "active_work_target_clip", Kind: "clip", ResultKeys: []string{"clip_id", "id", "item_id"}},
			}
		case "rack_add_node", "instantiate_plugin":
			specs[i].ConsumedBindings = []BindingSpec{{Key: "target_track", Kind: "track", Arg: "track_id"}}
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_loaded_plugin", Kind: "plugin", ResultKeys: []string{"plugin_id", "plugin_item_id", "node_id", "item_id", "id"}},
				{Key: "active_work_target_plugin", Kind: "plugin", ResultKeys: []string{"plugin_id", "plugin_item_id", "node_id", "item_id", "id"}},
			}
		}
	}
	return specs
}
