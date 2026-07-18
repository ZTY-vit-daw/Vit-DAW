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
	c.AddAlias("plugin_grabber_explain_controls", "plugin_grabber_explain_controls")
	c.AddAlias("plugin_learn_project_profile", "plugin_grabber_learn_project_profile")
	c.AddAlias("plugin.learn_project_profile", "plugin_grabber_learn_project_profile")
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
	c.AddAlias("plugin_grabber.apply", "plugin_grabber_apply_control")
	c.AddAlias("mix.observe", "mix_observe")
	c.AddAlias("mix.read", "mix_read")
	c.AddAlias("mix.derive", "mix_derive")
	c.AddAlias("control.add_macro", "control_add_macro")
	c.AddAlias("rack.add_macro", "control_add_macro")
	c.AddAlias("macro.create", "control_add_macro")
	c.AddAlias("control.rename_macro", "control_rename_macro")
	c.AddAlias("macro.rename", "control_rename_macro")
	c.AddAlias("artifact.list", "artifact_list")
	c.AddAlias("artifact.read", "artifact_read")
	c.AddAlias("artifact.extract", "artifact_extract")
	c.AddAlias("media.register_assets", "media_register_assets")
	c.AddAlias("media.index_authorized_folder", "media_index_authorized_folder")
	c.AddAlias("artifact.register_files", "media_register_assets")
	c.AddAlias("artifact.index_media", "media_index_authorized_folder")
	c.AddAlias("track.folder.create", "folder_track.create")
	c.AddAlias("track.create_folder", "folder_track.create")
	c.AddAlias("track.folder.set_routing_bus", "folder_track.set_routing_bus_enabled")
	c.AddAlias("track.folder.set_routing_bus_enabled", "folder_track.set_routing_bus_enabled")
	c.AddAlias("track_group.list", "track.group.list")
	c.AddAlias("track_group.create", "track.group.create")
	c.AddAlias("track_group.update", "track.group.update")
	c.AddAlias("track_group.set_members", "track.group.set_members")
	c.AddAlias("track_group.delete", "track.group.delete")
	c.AddAlias("track_group.apply_control", "track.group.apply_control")
	c.AddAlias("project.track_organization.apply", "project.apply_track_organization")
	c.AddAlias("browser.fetch", "browser_fetch")
	c.AddAlias("browser.search", "browser_search")
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
		parts = append(parts, modelSummaryLine(spec))
	}
	return strings.Join(parts, "\n")
}

func (c *Catalog) ModelSummaryForTools(toolNames []string) string {
	if c == nil {
		return ""
	}
	seen := map[string]bool{}
	var specs []CommandSpec
	for _, name := range toolNames {
		name = strings.TrimSpace(name)
		if name == "" || name == "daw.invoke" {
			continue
		}
		spec, ok := c.LookupTool(name)
		if !ok {
			if byCommand, commandOK := c.LookupCommand(name); commandOK {
				spec = byCommand
				ok = true
			}
		}
		if !ok {
			continue
		}
		key := spec.CommandName + "\x00" + spec.ToolName
		if seen[key] {
			continue
		}
		seen[key] = true
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].CommandName == specs[j].CommandName {
			return specs[i].ToolName < specs[j].ToolName
		}
		return specs[i].CommandName < specs[j].CommandName
	})
	parts := make([]string, 0, len(specs))
	for _, spec := range specs {
		parts = append(parts, modelSummaryLine(spec))
	}
	return strings.Join(parts, "\n")
}

func modelSummaryLine(spec CommandSpec) string {
	required := "-"
	if len(spec.RequiredTargetIDs) > 0 {
		required = strings.Join(spec.RequiredTargetIDs, ",")
	}
	return fmt.Sprintf("%s tool=%s risk=%s required=%s args=%s", spec.CommandName, spec.ToolName, modelRisk(spec), required, argHint(spec.CommandName))
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
	case "control_rename_macro":
		return "macro_id:string name:string"
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
	case "mix_request_observation", "mix_observe":
		return "mix_session_id:string target_ref:{kind,id,label} optional round:number goal_text:string duration_seconds:number"
	case "mix_read":
		return "observation_id:string OR mix_session_id:string keys:string[] optional range_sec:[start,end] detail:string max_items:number"
	case "mix_derive":
		return "observation_id:string OR mix_session_id:string type:string optional focus/a/b:object dimensions:string[] max_items:number"
	case "mix_propose_tick":
		return "operation:track_gain_adjust track_id:string optional delta_db:number observation_id:string evidence:object"
	case "mix_apply_tick":
		return "tick_id:string confirmation:boolean"
	case "mix_apply_static_balance_batch":
		return "actions:[{track_id:string target_db:number before_db:number delta_db:number}] observation_id:string candidate_plan_id:string"
	case "mix_apply_pan_layout_batch":
		return "actions:[{track_id:string target_pan:number before_pan:number delta_pan:number}] observation_id:string candidate_plan_id:string style_hash:string"
	case "mix_rollback_tick":
		return "optional tick_id:string"
	case "artifact_list":
		return "optional conversation_id:string limit:number"
	case "artifact_read", "artifact_extract":
		return "artifact_id:string OR file_path:string optional max_text_runes:number"
	case "media_register_assets":
		return "file_paths:string[] OR file_path:string optional media_kind:string title:string limit:number"
	case "media_index_authorized_folder":
		return "asset_location:string optional media_kinds:string[] recursive:boolean limit:number"
	case "project.get_audio_settings":
		return "no args"
	case "project.set_audio_settings":
		return "audio_settings:{sample_rate_hz?:number record_bit_depth?:number record_file_type?:string pcm_format?:string import_sample_rate_policy?:string import_bit_depth_policy?:string media_copy_policy?:string channel_import_policy?:string render_default_*?:... dither_policy?:string}"
	case "project.validate_audio_settings_change":
		return "audio_settings:{sample_rate_hz?:number record_bit_depth?:number ...}"
	case "project.import_preflight":
		return "folder_path:string OR file_paths:string[] optional recursive:boolean intended_mode:stems_folder start_time_seconds:number target_policy:create_tracks audio_settings_snapshot:object"
	case "project.import_folder_as_stems", "project.import_audio_files":
		return "folder_path:string OR file_paths:string[] optional recursive:boolean start_time_seconds:number target_policy:create_tracks confirmed:boolean skip_unreadable:boolean defer_audio_analysis:boolean start_audio_analysis:boolean command_timeout_ms:number"
	case "project.audio_analysis_start":
		return "optional analysis_job_id:string interval_ms:number max_submit_clips:number max_submit_feature_jobs:number retry_missing:boolean rebuild_from_project:boolean"
	case "project.audio_analysis_status", "project.audio_analysis_cancel":
		return "optional analysis_job_id:string ensure_ready:boolean timeout_ms:number poll_interval_ms:number"
	case "project.markers.list":
		return "no args"
	case "project.markers.upsert":
		return "name:string start_seconds:number optional end_seconds:number color:string"
	case "project.markers.apply_section_markers":
		return "sections:[{name|label:string start_seconds:number end_seconds:number confidence?:string}] optional replace_existing:boolean source:string"
	case "project.markers.rename":
		return "marker_id:string name:string"
	case "project.markers.delete":
		return "marker_id:string"
	case "folder_track.create":
		return "folder_name:string optional parent_track_id:string preceding_track_id:string routing_bus_enabled:boolean"
	case "track.move_to_folder":
		return "track_id:string folder_track_id:string optional preceding_track_id:string"
	case "folder_track.set_routing_bus_enabled":
		return "folder_track_id:string routing_bus_enabled:boolean optional force_clear_plugins:boolean"
	case "project.apply_track_organization":
		return "groups:[{folder_name/proposed_folder:string track_ids:string[] OR assignments:[{track_id:string}] optional routing_bus_enabled:boolean}]"
	case "track.group.list":
		return "no args"
	case "track.group.create":
		return "name:string track_ids:string[] optional color:string origin:string linked_controls:{volume?:boolean pan?:boolean mute?:boolean solo?:boolean}"
	case "track.group.update":
		return "group_id:string optional name:string color:string enabled:boolean suspended:boolean linked_controls:object track_ids:string[]"
	case "track.group.set_members":
		return "group_id:string track_ids:string[]"
	case "track.group.delete":
		return "group_id:string"
	case "track.group.apply_control":
		return "group_id:string OR track_ids:string[] create_group_if_missing:bool control:volume mode:absolute|relative db:number OR delta_db:number"
	case "media.inspect_files":
		return "file_paths:string[] OR folder_path:string optional recursive:boolean media_kinds:string[] audio_settings_snapshot:object"
	case "browser_fetch":
		return "url:string optional max_bytes:number timeout_ms:number"
	case "browser_search":
		return "query:string optional max_results:number"
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
	case "clip.fade.set":
		return "clip_id:string optional fade_in_seconds:number fade_out_seconds:number fade_in_curve:string fade_out_curve:string auto_crossfade:boolean"
	case "clip.fade.read":
		return "clip_id:string"
	case "clip.gain.set":
		return "clip_id:string gain_db:number"
	case "clip.gain.set_batch":
		return "pending_actions:[{args:{clip_id:string gain_db:number track_id?:string}}]"
	case "clip.gain.read":
		return "clip_id:string"
	case "clip.strip_silence.analyze":
		return "clip_id:string optional track_id:string ranges:[{clip_id:string track_id:string start_seconds:number end_seconds:number}] threshold_dbfs:number min_silence_ms:number clip_start_pad_ms:number clip_end_pad_ms:number scope:string"
	case "clip.strip_silence.suggest":
		return "optional clip_id:string track_id:string scope:selected_clip|selected_ranges|all_project ranges:[{clip_id:string track_id:string start_seconds:number end_seconds:number}] candidate_thresholds_dbfs:number[] min_silence_ms:number clip_start_pad_ms:number clip_end_pad_ms:number"
	case "clip.strip_silence.apply":
		return "clip_id:string optional track_id:string analysis_id:string strip_regions:[{clip_id:string track_id:string start_seconds:number end_seconds:number}] allow_remove_entire_clip:boolean"
	case "clip.strip_silence.apply_batch":
		return "pending_actions:[{args:{clip_id:string track_id:string analysis_id:string strip_regions:[{start_seconds:number end_seconds:number}]}}] allow_remove_entire_clip:boolean"
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
	case "plugin_grabber_learn_project_profile":
		return "track_id:string plugin_id:string optional intent:string"
	case "plugin_grabber_upsert_project_profile":
		return "track_id:string plugin_id:string quick_control_ids:string[] aliases?:object display_groups?:object normalized_roles?:object class?:string groups?:array virtual_controls?:array safety?:object plugin_skill?:object"
	case "plugin_grabber_remove_project_profile":
		return "profile_id:string OR track_id:string plugin_id:string"
	case "plugin_grabber_apply_control":
		return "track_id:string plugin_id:string control:string target:{freq_hz?:number gain_db?:number q?:number threshold_db?:number amount?:number|string component_id?:string}"
	case "capability_equalizer_inspect":
		return "optional target_ref:string track_id:string plugin_id:string provider_credential_id:string"
	case "capability_equalizer_plan":
		return "task:spectral_region_adjust|highpass|lowpass|output_control optional target_ref:string track_id:string plugin_id:string provider_credential_id:string band_ref:b1|b2|b3|b4 response_shape:bell|low_shelf|high_shelf frequency_hz:number gain_db:number q:number enabled:boolean cutoff_frequency_hz:number slope_db_per_octave:number bypass:boolean dry_mix_percent:number output_gain_db:number"
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
	if category == "artifact" {
		return "artifact"
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
		spec("project.get_audio_settings", "project.get_audio_settings", "project_audio", "Read the active project's audio specification metadata, including project sample rate, recording format defaults, import policies, and engine binding status.", RiskDirect, false, false, false, false),
		spec("project.set_audio_settings", "project.set_audio_settings", "project_audio", "Update and persist the active project's audio specification metadata without changing the audio device sample rate.", RiskUndoable, true, true, false, true),
		spec("project.validate_audio_settings_change", "project.validate_audio_settings_change", "project_audio", "Check whether an audio settings change needs user attention, especially when existing audio clips or device-rate mismatch warnings are present.", RiskDirect, false, false, false, false),
		spec("project.import_preflight", "project.import_preflight", "project_audio", "Inspect local audio files or a stems folder and return a kernel-built import plan without writing clips or tracks.", RiskDirect, false, false, false, false),
		spec("media.inspect_files", "media.inspect_files", "project_audio", "Read bounded local audio metadata for explicit files or a folder without importing media into the project.", RiskDirect, false, false, false, false),
		spec("project.markers.list", "project.markers.list", "project_marker", "Read project position/range markers from the timeline marker map.", RiskDirect, false, false, false, false),
		spec("project.markers.upsert", "project.markers.upsert", "project_marker", "Create or update one project marker. end_seconds omitted creates a position marker; end_seconds greater than start_seconds creates a range/section marker.", RiskUndoable, true, true, false, true),
		spec("project.markers.apply_section_markers", "project.markers.apply_section_markers", "project_marker", "Write a confirmed A5/EPM section map as range markers without moving clips or changing audio.", RiskConfirm, true, true, true, true),
		spec("project.markers.rename", "project.markers.rename", "project_marker", "Rename one project marker by marker_id.", RiskUndoable, true, true, false, true, "marker_id"),
		spec("project.markers.delete", "project.markers.delete", "project_marker", "Delete one project marker by marker_id.", RiskConfirm, true, true, true, true, "marker_id"),

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

		spec("artifact_list", "artifact.list", "artifact", "List Vit artifacts in the local workspace artifact store.", RiskDirect, false, false, false, false),
		spec("artifact_read", "artifact.read", "artifact", "Read one Vit artifact by id, including cached extracted text when available.", RiskDirect, false, false, false, false),
		spec("artifact_extract", "artifact.extract", "artifact", "Extract or refresh a safe text/metadata digest for one Vit artifact.", RiskDirect, false, false, false, false),
		spec("media_register_assets", "media.register_assets", "artifact", "Register user-authorized local media or document files as Vit artifacts for media pool preview.", RiskDirect, false, false, false, false),
		spec("media_index_authorized_folder", "media.index_authorized_folder", "artifact", "Index previewable files in a user-authorized local asset folder and register them as Vit artifacts.", RiskDirect, false, false, false, false),
		spec("browser_fetch", "browser.fetch", "browser", "Fetch a public webpage and store it as a web_page artifact.", RiskDirect, false, false, false, false),
		spec("browser_search", "browser.search", "browser", "Search the public web and store search results as a browser artifact.", RiskDirect, false, false, false, false),
		spec("web_search", "web.search", "web", "Search the public web with confirmation and privacy limits.", RiskConfirm, false, false, true, false),
		spec("web_fetch", "web.fetch", "web", "Fetch a public http/https URL with confirmation and network safety limits; returns bounded text_digest/body_excerpt by default, with raw body only when include_body/raw_body is explicitly set.", RiskConfirm, false, false, true, false),
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
		spec("version_project_new", "version.project_new", "project_history", "Start a fresh unsaved Project History draft for a new project.", RiskDirect, false, false, false, false),
		spec("version_project_opened", "version.project_opened", "project_history", "Activate the Project History working session for a project opened by the host.", RiskDirect, false, false, false, false),
		spec("version_project_save_prepare", "version.project_save_prepare", "project_history", "Freeze the active Agent working session before the host saves the project file.", RiskDirect, false, false, false, false),
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
		spec("mix_request_observation", "mix.request_observation", "mix", "Read project/shadow facts and write a time-rulered MixBoard observation packet for a mix session.", RiskDirect, false, false, false, false),
		spec("mix_observe", "mix.observe", "mix", "Observe the requested mix scope and return a compact acoustic digest plus a readable observation catalog; does not mutate the project.", RiskDirect, false, false, false, false),
		spec("mix_read", "mix.read", "mix", "Read selected catalog entries from a stored MixBoard observation, including bounded ranges for long acoustic rows.", RiskDirect, false, false, false, false),
		spec("mix_derive", "mix.derive", "mix", "Derive an on-demand relationship package from stored MixBoard observations, such as before/after or focus/project comparisons.", RiskDirect, false, false, false, false),
		spec("mix_propose_tick", "mix.propose_tick", "mix", "Agent-local proposal for one safe mix tick after observation; supports small track_gain_adjust and track_pan_adjust/track_pan_set without mutating the project.", RiskDirect, false, false, false, false, "track_id"),
		spec("mix_apply_tick", "mix.apply_tick", "mix", "Agent-local confirmed execution of one proposed mix tick through primitive set_volume or set_pan kernel commands.", RiskUndoable, true, true, false, true),
		spec("mix_apply_static_balance_batch", "mix.apply_static_balance_batch", "mix", "Apply one validated B2 static-balance plan as an atomic batch of absolute track fader targets.", RiskConfirm, true, true, true, true),
		spec("mix_apply_pan_layout_batch", "mix.apply_pan_layout_batch", "mix", "Apply one validated B3 pan-layout plan as an atomic batch of absolute track pan targets.", RiskConfirm, true, true, true, true),
		spec("mix_rollback_tick", "mix.rollback_tick", "mix", "Agent-local rollback for an applied mix tick by restoring the previous primitive track volume or pan.", RiskUndoable, true, true, false, true),

		spec("add_track", "track.add", "track", "Add a new track.", RiskUndoable, true, true, false, true),
		spec("add_audio_track", "track.add_audio", "track", "Add a new audio track.", RiskUndoable, true, true, false, true),
		spec("folder_track.create", "track.folder.create", "track", "Create a track folder container. By default this only organizes hybrid tracks; routing_bus_enabled turns it into a folder bus.", RiskUndoable, true, true, false, true),
		spec("track.move_to_folder", "track.move_to_folder", "track", "Move a user-visible track under a folder container without changing clips or media.", RiskUndoable, true, true, false, true, "track_id", "folder_track_id"),
		spec("folder_track.set_routing_bus_enabled", "track.folder.set_routing_bus_enabled", "track", "Enable or disable bus routing on a folder container. Disabling refuses to clear user plugins unless force_clear_plugins is explicitly confirmed.", RiskConfirm, true, true, true, true, "folder_track_id"),
		spec("project.apply_track_organization", "project.apply_track_organization", "project", "Apply a confirmed TOM organization proposal by creating ordinary folder containers and moving tracks into them; bus routing stays off unless explicitly requested.", RiskConfirm, true, true, true, true),
		spec("track.group.list", "track.group.list", "track", "Read persistent track control groups from the active project.", RiskDirect, false, false, false, false),
		spec("track.group.create", "track.group.create", "track", "Create a persistent track control group with explicit member track IDs.", RiskUndoable, true, true, false, true),
		spec("track.group.update", "track.group.update", "track", "Update a persistent track control group's name, color, enabled/suspended state, linked controls, or members.", RiskUndoable, true, true, false, true, "group_id"),
		spec("track.group.set_members", "track.group.set_members", "track", "Replace a track control group's member track IDs.", RiskUndoable, true, true, false, true, "group_id"),
		spec("track.group.delete", "track.group.delete", "track", "Delete a persistent track control group by group_id.", RiskConfirm, true, true, true, true, "group_id"),
		spec("track.group.apply_control", "track.group.apply_control", "track", "Apply a confirmed volume control operation to every member of a track group and return per-member verification; can create/reuse a group from explicit track_ids for B1 fader reset.", RiskConfirm, true, true, true, true),
		spec("append_ghost_track", "track.append_ghost", "track", "Append a ghost track placeholder.", RiskUndoable, true, true, false, true),
		spec("select_track", "track.select", "track", "Select a user-visible track in the UI without changing the project.", RiskDirect, false, false, false, false, "track_id"),
		spec("delete_track", "track.delete", "track", "Delete a track by stable track_id.", RiskConfirm, true, true, true, true, "track_id"),
		spec("rename_track", "track.rename", "track", "Rename a track by stable track_id.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_mute", "track.mute", "track", "Set track mute state.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_solo", "track.solo", "track", "Set track solo state.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_volume", "track.volume", "track", "Set track volume in dB.", RiskUndoable, true, true, false, true, "track_id"),
		spec("set_pan", "track.pan", "track", "Set track pan from -1.0 left to +1.0 right.", RiskUndoable, true, true, false, true, "track_id"),
		spec("freeze_track", "track.freeze", "track", "Freeze a track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("unfreeze_track", "track.unfreeze", "track", "Unfreeze a track.", RiskConfirm, true, true, true, true, "track_id"),

		spec("add_audio_clip", "clip.add_audio", "clip", "Add an audio clip to a known track and start time.", RiskConfirm, true, true, true, true, "track_id"),
		spec("import_audio", "clip.import_audio", "clip", "Import audio to a target track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("import_media_to_track", "clip.import_media_to_track", "clip", "Import media to a target track.", RiskConfirm, true, true, true, true, "track_id"),
		spec("project.import_folder_as_stems", "project.import_folder_as_stems", "project_audio", "Import a stems folder as one audio track and clip per readable file in a single kernel command.", RiskConfirm, true, true, true, true),
		spec("project.import_audio_files", "project.import_audio_files", "project_audio", "Import multiple audio files as one track and clip per file in a single kernel command.", RiskConfirm, true, true, true, true),
		spec("project.audio_analysis_start", "project.audio_analysis_start", "project_audio", "Start a deferred project audio-analysis queue at a throttled background submission rate.", RiskDirect, false, false, false, false),
		spec("project.audio_analysis_status", "project.audio_analysis_status", "project_audio", "Read lightweight status for a deferred project audio-analysis queue.", RiskDirect, false, false, false, false),
		spec("project.audio_analysis_cancel", "project.audio_analysis_cancel", "project_audio", "Cancel pending items in a deferred project audio-analysis queue; already submitted background bakes may continue.", RiskDirect, false, false, false, false),
		spec("move_clip", "clip.move", "clip", "Move a clip by stable track and clip IDs.", RiskConfirm, true, true, true, true, "source_track_id", "target_track_id", "clip_id"),
		spec("resize_clip", "clip.resize", "clip", "Resize a clip by stable clip ID.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("split_clip", "clip.split", "clip", "Split a clip at a timeline position.", RiskConfirm, true, true, true, true, "track_id", "clip_id"),
		spec("clone_clip", "clip.clone", "clip", "Clone an existing clip.", RiskConfirm, true, true, true, true, "source_clip_id", "target_track_id"),
		spec("remove_clips", "clip.remove", "clip", "Remove one or more clips.", RiskConfirm, true, true, true, true, "clip_ids"),
		spec("clip.fade.set", "clip.fade.set", "clip", "Set clip-bound fade-in/fade-out lengths and fade metadata on an audio clip.", RiskConfirm, true, true, true, true, "clip_id"),
		spec("clip.fade.read", "clip.fade.read", "clip", "Read clip-bound fade-in/fade-out state from an audio clip.", RiskDirect, false, false, false, false, "clip_id"),
		spec("clip.gain.set", "clip.gain.set", "clip", "Set static clip gain in dB on an audio clip before track processing.", RiskConfirm, true, true, true, true, "clip_id"),
		spec("clip.gain.set_batch", "clip.gain.set_batch", "clip", "Apply multiple confirmed static clip gain targets as one Agent tool call for B1 full-project source calibration.", RiskConfirm, true, true, true, true),
		spec("clip.gain.read", "clip.gain.read", "clip", "Read static clip gain, pan, and mute state from an audio clip.", RiskDirect, false, false, false, false, "clip_id"),
		spec("clip.strip_silence.analyze", "clip.strip_silence.analyze", "clip", "Analyze an audio clip or selected clip ranges for Strip Silence and return preview regions/actions without mutating the project.", RiskDirect, false, false, false, false, "clip_id"),
		spec("clip.strip_silence.suggest", "clip.strip_silence.suggest", "clip", "Agent-local Strip Silence recommendation: sweep real kernel analysis thresholds for selected clip/ranges or all project audio, then return conservative parameters and pending clip.strip_silence.apply actions without mutating the project.", RiskDirect, false, false, false, false),
		spec("clip.strip_silence.apply", "clip.strip_silence.apply", "clip", "Apply a confirmed Strip Silence preview by deleting the supplied silent regions from one audio clip.", RiskConfirm, true, true, true, true, "clip_id"),
		spec("clip.strip_silence.apply_batch", "clip.strip_silence.apply_batch", "clip", "Apply multiple confirmed Strip Silence previews as one Agent tool call and return only a compact summary.", RiskConfirm, true, true, true, true),
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
		spec("select_plugin", "plugin.select", "plugin", "Select a plugin/rack node in the UI without changing the project.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("open_plugin_ui", "plugin.open", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("show_plugin_editor", "plugin.show_editor", "plugin", "Open a plugin editor window.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("get_plugin_parameters", "plugin.get_parameters", "plugin", "Read normalized plugin parameter values.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("set_plugin_param", "plugin.set_parameter", "plugin", "Set one explicit plugin parameter by raw/normalized value, or by plugin-exposed display text via value_text such as 1000 ms when get_plugin_parameters display_probe is high confidence.", RiskUndoable, true, true, false, true, "track_id", "plugin_id", "param_id"),
		spec("set_plugin_param_aliases", "plugin.set_aliases", "plugin", "Store semantic aliases for plugin parameters.", RiskUndoable, true, true, false, true, "plugin_id"),
		spec("plugin_grabber_get_project_profiles", "plugin_grabber.get_project_profiles", "plugin_grabber", "Read project-scoped plugin grabber profiles.", RiskDirect, false, false, false, false),
		spec("plugin_grabber_explain_controls", "plugin_grabber.explain_controls", "plugin_grabber", "Build a compact AI-friendly context pack for one loaded plugin without filtering full parameters.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("plugin_grabber_learn_project_profile", "plugin_grabber.learn_project_profile", "plugin_grabber", "Learn and propose a project/global plugin grabber profile for one loaded plugin, then ask the user to confirm before saving.", RiskDirect, false, false, false, false, "track_id", "plugin_id"),
		spec("plugin_grabber_upsert_project_profile", "plugin_grabber.upsert_project_profile", "plugin_grabber", "Save project-scoped plugin grabber profile annotations for one plugin.", RiskConfirm, true, false, true, true, "track_id", "plugin_id"),
		spec("plugin_grabber_remove_project_profile", "plugin_grabber.remove_project_profile", "plugin_grabber", "Remove a project-scoped plugin grabber profile.", RiskConfirm, true, false, true, true),
		spec("plugin_grabber_apply_control", "plugin_grabber.apply_control", "plugin_grabber", "Apply a learned plugin grabber runtime control from an acoustic target using the current validated parameter profile; result.applied_parameters[].new_value_text is the actual applied display value.", RiskUndoable, true, true, false, true, "track_id", "plugin_id"),
		spec("capability_equalizer_inspect", "capability.equalizer.inspect", "capability", "Read the equalizer work-card contract, currently loaded Provider candidates, conformed generic actions, and capability gaps without learning a plugin or exposing raw parameter IDs.", RiskDirect, false, false, false, false),
		spec("capability_equalizer_plan", "capability.equalizer.plan", "capability", "Submit a vendor-neutral equalizer task action to the capability layer. The capability layer returns semantic gaps or Band resource choices to the Agent, and only a complete action is handed to SPAL for verified VPS Provider binding and a governed Proposal. Never substitute Plugin Learning for this tool.", RiskUndoable, true, true, false, true),
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
		spec("control_add_macro", "plugin_map_macro_to_params", "plugin_grabber", "Create a new macro control only when the user asks to create a new macro.", RiskConfirm, true, true, true, true, "track_id"),
		spec("control_rename_macro", "control.rename_macro", "control", "Rename an existing macro control by macro_id or visible macro name.", RiskUndoable, true, true, false, true),
		spec("control_add_binding", "control.add_binding", "control", "Bind an existing macro/control to a plugin parameter; use an existing macro_id from macro_refs or available_macro_controls.", RiskConfirm, true, true, true, true),
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
		case "folder_track.create":
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_folder_track", Kind: "track", ResultKeys: []string{"folder_track_id", "track_id", "id", "item_id"}},
			}
		case "track.move_to_folder":
			specs[i].ConsumedBindings = []BindingSpec{
				{Key: "target_track", Kind: "track", Arg: "track_id"},
				{Key: "target_folder_track", Kind: "track", Arg: "folder_track_id"},
			}
		case "folder_track.set_routing_bus_enabled":
			specs[i].ConsumedBindings = []BindingSpec{{Key: "target_folder_track", Kind: "track", Arg: "folder_track_id"}}
		case "project.apply_track_organization":
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_folder_track", Kind: "track", ResultKeys: []string{"created_folder_ids", "folder_track_id", "track_id", "id"}},
			}
		case "track.group.create":
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_track_group", Kind: "track_group", ResultKeys: []string{"group_id", "id"}},
				{Key: "active_work_target_track_group", Kind: "track_group", ResultKeys: []string{"group_id", "id"}},
			}
		case "track.group.update", "track.group.set_members", "track.group.delete", "track.group.apply_control":
			specs[i].ConsumedBindings = []BindingSpec{{Key: "target_track_group", Kind: "track_group", Arg: "group_id"}}
		case "create_midi_clip", "insert_midi_clip", "import_midi_to_track", "import_audio", "import_media_to_track", "add_audio_clip":
			specs[i].ConsumedBindings = []BindingSpec{{Key: "target_track", Kind: "track", Arg: "track_id"}}
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_clip", Kind: "clip", ResultKeys: []string{"clip_id", "id", "item_id"}},
				{Key: "active_work_target_clip", Kind: "clip", ResultKeys: []string{"clip_id", "id", "item_id"}},
			}
		case "project.import_folder_as_stems", "project.import_audio_files":
			specs[i].ProducedBindings = []BindingSpec{
				{Key: "last_created_track", Kind: "track", ResultKeys: []string{"last_created_track_id", "track_id", "id", "item_id"}},
				{Key: "last_created_clip", Kind: "clip", ResultKeys: []string{"last_created_clip_id", "clip_id", "id", "item_id"}},
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
