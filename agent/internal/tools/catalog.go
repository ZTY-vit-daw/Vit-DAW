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

type CommandSpec struct {
	CommandName          string         `json:"command_name"`
	ToolName             string         `json:"tool_name"`
	Namespace            string         `json:"namespace"`
	Category             string         `json:"category"`
	Description          string         `json:"description"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	RiskLevel            RiskLevel      `json:"risk_level"`
	MutatesProject       bool           `json:"mutates_project"`
	SupportsUndo         bool           `json:"supports_undo"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	RequiredTargetIDs    []string       `json:"required_target_ids,omitempty"`
	RefreshAfter         bool           `json:"refresh_after"`
	Implemented          bool           `json:"implemented"`
}

type Tool struct {
	Name                 string         `json:"name"`
	Namespace            string         `json:"namespace"`
	Category             string         `json:"category"`
	Description          string         `json:"description"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	OutputSchema         map[string]any `json:"output_schema,omitempty"`
	RiskLevel            RiskLevel      `json:"risk_level"`
	MutatesProject       bool           `json:"mutates_project"`
	SupportsUndo         bool           `json:"supports_undo"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	RequiredTargetIDs    []string       `json:"required_target_ids,omitempty"`
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
	c.AddAlias("plugin_explain_controls", "get_plugin_parameters")
	c.AddAlias("plugin_learn_common_roles", "set_plugin_param_aliases")
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
			Category:             spec.Category,
			Description:          spec.Description,
			InputSchema:          spec.InputSchema,
			OutputSchema:         spec.OutputSchema,
			RiskLevel:            spec.RiskLevel,
			MutatesProject:       spec.MutatesProject,
			SupportsUndo:         spec.SupportsUndo,
			RequiresConfirmation: spec.RequiresConfirmation,
			RequiredTargetIDs:    append([]string(nil), spec.RequiredTargetIDs...),
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
	case "set_plugin_param":
		return "track_id:string plugin_id:string param_id:string value:number"
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
	return []CommandSpec{
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
		spec("insert_midi_clip", "midi.insert_clip", "midi", "Insert a MIDI clip.", RiskConfirm, true, true, true, true, "track_id"),
		spec("create_midi_clip", "midi.create_clip", "midi", "Create a MIDI clip.", RiskConfirm, true, true, true, true, "track_id"),
		spec("add_midi_notes", "midi.add_notes", "midi", "Add MIDI notes to a clip.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("add_midi_notes_bulk", "midi.add_notes_bulk", "midi", "Add many MIDI notes to a clip.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("mutate_midi_notes", "midi.mutate_notes", "midi", "Modify existing MIDI notes.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("delete_midi_notes", "midi.delete_notes", "midi", "Delete MIDI notes.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),

		spec("scan_plugins", "plugin.scan", "plugin", "Scan plugins.", RiskConfirm, true, false, true, false),
		spec("instantiate_plugin", "plugin.instantiate", "plugin", "Instantiate a plugin on a track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("open_plugin_ui", "plugin.open", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("show_plugin_editor", "plugin.show_editor", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("get_plugin_parameters", "plugin.get_parameters", "plugin", "Read normalized plugin parameter values.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("set_plugin_param", "plugin.set_parameter", "plugin", "Set one plugin parameter.", RiskUndoable, true, true, false, true, "track_id", "plugin_id", "param_id"),
		spec("set_plugin_param_aliases", "plugin.set_aliases", "plugin", "Store semantic aliases for plugin parameters.", RiskUndoable, true, true, false, true, "plugin_id"),
		spec("delete_plugin", "plugin.delete", "plugin", "Delete a plugin instance.", RiskConfirm, true, true, true, true, "track_id", "plugin_id"),
		spec("move_plugin", "plugin.move", "plugin", "Move a plugin instance.", RiskConfirm, true, true, true, true, "track_id", "plugin_id"),

		spec("rack_add_node", "rack.add_node", "rack", "Add a rack node.", RiskConfirm, true, true, true, true),
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
}
