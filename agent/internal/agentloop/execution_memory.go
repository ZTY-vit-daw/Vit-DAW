package agentloop

import (
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/tools"
)

type ExecutionBinding struct {
	Key              string  `json:"key,omitempty"`
	Kind             string  `json:"kind,omitempty"`
	ID               string  `json:"id,omitempty"`
	Name             string  `json:"name,omitempty"`
	TrackID          string  `json:"track_id,omitempty"`
	SourceToolCallID string  `json:"source_tool_call_id,omitempty"`
	Tool             string  `json:"tool,omitempty"`
	CommandName      string  `json:"command_name,omitempty"`
	PlanItemID       string  `json:"plan_item_id,omitempty"`
	Provenance       string  `json:"provenance,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
}

type ExecutionMemory struct {
	LastCreatedTrackID       string             `json:"last_created_track_id,omitempty"`
	LastCreatedTrackName     string             `json:"last_created_track_name,omitempty"`
	LastCreatedClipID        string             `json:"last_created_clip_id,omitempty"`
	LastCreatedClipName      string             `json:"last_created_clip_name,omitempty"`
	LastLoadedPluginID       string             `json:"last_loaded_plugin_id,omitempty"`
	LastLoadedPluginName     string             `json:"last_loaded_plugin_name,omitempty"`
	ActiveWorkTargetTrackID  string             `json:"active_work_target_track_id,omitempty"`
	ActiveWorkTargetClipID   string             `json:"active_work_target_clip_id,omitempty"`
	ActiveWorkTargetPluginID string             `json:"active_work_target_plugin_id,omitempty"`
	Bindings                 []ExecutionBinding `json:"bindings,omitempty"`
}

type RecentObservation struct {
	ToolCallID       string                      `json:"tool_call_id,omitempty"`
	Tool             string                      `json:"tool,omitempty"`
	CommandName      string                      `json:"command_name,omitempty"`
	Status           string                      `json:"status,omitempty"`
	Error            string                      `json:"error,omitempty"`
	Summary          map[string]any              `json:"summary,omitempty"`
	MutationBarrier  bool                        `json:"mutation_barrier,omitempty"`
	ProducedBindings []ExecutionBinding          `json:"produced_bindings,omitempty"`
	Verification     *planner.VerificationResult `json:"verification,omitempty"`
}

func toolNeedsMutationBarrier(call planner.ToolCall, result executorpkg.Result) bool {
	if spec, ok := catalogSpecForToolCall(call, result); ok {
		return spec.MutatesProject || spec.RefreshAfter
	}
	switch normalizedActionName(call, result) {
	case "add_track", "add_audio_track", "track.add", "track.add_audio",
		"create_midi_clip", "insert_midi_clip", "import_midi_to_track",
		"import_audio", "import_media_to_track", "add_audio_clip",
		"rack_add_node", "rack.add_node", "plugin.load_to_rack", "instantiate_plugin":
		return true
	default:
		return false
	}
}

func catalogSpecForToolCall(call planner.ToolCall, result executorpkg.Result) (tools.CommandSpec, bool) {
	for _, name := range []string{call.Tool, result.Tool} {
		if spec, ok := defaultCatalog.LookupTool(strings.TrimSpace(name)); ok {
			return spec, true
		}
	}
	for _, name := range []string{
		result.CommandName,
		commandNameFromMap(call.Command),
		commandNameFromMap(call.Args),
		normalizedActionName(call, result),
	} {
		if spec, ok := defaultCatalog.LookupCommand(strings.TrimSpace(name)); ok {
			return spec, true
		}
	}
	return tools.CommandSpec{}, false
}

func updateExecutionMemoryForTool(state *runState, call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult) []ExecutionBinding {
	if state == nil {
		return nil
	}
	action := normalizedActionName(call, result)
	switch action {
	case "add_track", "add_audio_track", "track.add", "track.add_audio":
		return bindCreatedTrack(state, call, result, ver)
	case "create_midi_clip", "insert_midi_clip", "import_midi_to_track", "midi.create_clip", "midi.insert_clip", "midi.import_file",
		"import_audio", "import_media_to_track", "add_audio_clip", "clip.import_audio", "clip.import_media_to_track", "clip.add_audio":
		return bindCreatedClip(state, call, result, ver)
	case "rack_add_node", "rack.add_node", "plugin.load_to_rack", "instantiate_plugin", "plugin.instantiate":
		return bindLoadedPlugin(state, call, result, ver)
	default:
		return nil
	}
}

func bindCreatedTrack(state *runState, call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult) []ExecutionBinding {
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "id", "item_id"),
		firstMapText(ver.Evidence, "observed_track_id", "expected_track_id"),
		firstMapText(call.Args, "track_id", "selected_track_id", "target_track_id"),
	)
	trackName := firstNonEmpty(
		firstMapText(result.Result, "track_name", "name"),
		firstMapText(ver.Evidence, "observed_track_name", "expected_track_name"),
		firstMapText(call.Args, "name", "track_name"),
	)
	if trackID == "" && trackName == "" {
		return nil
	}
	state.executionMemory.LastCreatedTrackID = trackID
	state.executionMemory.LastCreatedTrackName = trackName
	state.executionMemory.ActiveWorkTargetTrackID = trackID
	bindings := []ExecutionBinding{
		executionBinding("last_created_track", "track", trackID, trackName, "", call, result, "tool_result"),
		executionBinding("active_work_target_track", "track", trackID, trackName, "", call, result, "created_object"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func bindCreatedClip(state *runState, call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult) []ExecutionBinding {
	clipID := firstNonEmpty(
		firstMapText(result.Result, "clip_id", "id", "item_id"),
		firstMapText(ver.Evidence, "observed_clip_id", "expected_clip_id"),
		firstMapText(call.Args, "clip_id"),
	)
	clipName := firstNonEmpty(
		firstMapText(result.Result, "clip_name", "name"),
		firstMapText(ver.Evidence, "observed_clip_name", "expected_clip_name"),
		firstMapText(call.Args, "clip_name", "name"),
	)
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "target_track_id", "selected_track_id"),
		firstMapText(ver.Evidence, "observed_track_id", "expected_track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	if clipID == "" && clipName == "" {
		return nil
	}
	state.executionMemory.LastCreatedClipID = clipID
	state.executionMemory.LastCreatedClipName = clipName
	state.executionMemory.ActiveWorkTargetClipID = clipID
	if trackID != "" {
		state.executionMemory.ActiveWorkTargetTrackID = trackID
	}
	bindings := []ExecutionBinding{
		executionBinding("last_created_clip", "clip", clipID, clipName, trackID, call, result, "tool_result"),
		executionBinding("active_work_target_clip", "clip", clipID, clipName, trackID, call, result, "created_object"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func bindLoadedPlugin(state *runState, call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult) []ExecutionBinding {
	pluginID := firstNonEmpty(
		firstMapText(result.Result, "plugin_id", "plugin_item_id", "node_id", "item_id", "id"),
		firstMapText(ver.Evidence, "observed_node_id", "expected_plugin_id"),
		firstMapText(call.Args, "plugin_id", "plugin_item_id", "node_id", "item_id", "id"),
	)
	pluginName := firstNonEmpty(
		firstMapText(result.Result, "plugin_name", "name", "descriptive_name"),
		firstMapText(ver.Evidence, "observed_plugin_name", "expected_plugin_name"),
		firstMapText(call.Args, "plugin_name", "name", "plugin_query", "query"),
	)
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id", "target_track_id", "selected_track_id"),
		firstMapText(ver.Evidence, "expected_track_id", "observed_track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	if pluginID == "" && pluginName == "" {
		return nil
	}
	state.executionMemory.LastLoadedPluginID = pluginID
	state.executionMemory.LastLoadedPluginName = pluginName
	state.executionMemory.ActiveWorkTargetPluginID = pluginID
	if trackID != "" {
		state.executionMemory.ActiveWorkTargetTrackID = trackID
	}
	bindings := []ExecutionBinding{
		executionBinding("last_loaded_plugin", "plugin", pluginID, pluginName, trackID, call, result, "tool_result"),
		executionBinding("active_work_target_plugin", "plugin", pluginID, pluginName, trackID, call, result, "loaded_object"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func executionBinding(key, kind, id, name, trackID string, call planner.ToolCall, result executorpkg.Result, provenance string) ExecutionBinding {
	return ExecutionBinding{
		Key:              strings.TrimSpace(key),
		Kind:             strings.TrimSpace(kind),
		ID:               strings.TrimSpace(id),
		Name:             strings.TrimSpace(name),
		TrackID:          strings.TrimSpace(trackID),
		SourceToolCallID: firstNonEmpty(result.ToolCallID, call.ID),
		Tool:             firstNonEmpty(result.Tool, call.Tool),
		CommandName:      strings.TrimSpace(result.CommandName),
		PlanItemID:       strings.TrimSpace(call.PlanItemID),
		Provenance:       strings.TrimSpace(provenance),
		Confidence:       1,
	}
}

func addExecutionBindings(memory *ExecutionMemory, bindings ...ExecutionBinding) {
	if memory == nil {
		return
	}
	for _, binding := range bindings {
		if binding.Key == "" || (binding.ID == "" && binding.Name == "") {
			continue
		}
		replaced := false
		for i := range memory.Bindings {
			if memory.Bindings[i].Key == binding.Key {
				memory.Bindings[i] = binding
				replaced = true
				break
			}
		}
		if !replaced {
			memory.Bindings = append(memory.Bindings, binding)
		}
	}
	if len(memory.Bindings) > 12 {
		memory.Bindings = append([]ExecutionBinding(nil), memory.Bindings[len(memory.Bindings)-12:]...)
	}
}

func recentObservationForTool(call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult, mutationBarrier bool, bindings []ExecutionBinding) *RecentObservation {
	return &RecentObservation{
		ToolCallID:       firstNonEmpty(result.ToolCallID, call.ID),
		Tool:             firstNonEmpty(result.Tool, call.Tool),
		CommandName:      strings.TrimSpace(result.CommandName),
		Status:           strings.TrimSpace(result.Status),
		Error:            strings.TrimSpace(firstNonEmpty(result.Error, "")),
		Summary:          mixObservationPromptSummary(result.Result),
		MutationBarrier:  mutationBarrier,
		ProducedBindings: cloneExecutionBindings(bindings),
		Verification:     cloneVerificationResult(&ver),
	}
}

func cloneExecutionMemory(in ExecutionMemory) ExecutionMemory {
	out := in
	out.Bindings = cloneExecutionBindings(in.Bindings)
	return out
}

func cloneExecutionBindings(in []ExecutionBinding) []ExecutionBinding {
	if len(in) == 0 {
		return nil
	}
	return append([]ExecutionBinding(nil), in...)
}

func cloneRecentObservation(in *RecentObservation) *RecentObservation {
	if in == nil {
		return nil
	}
	out := *in
	out.Summary = cloneMap(in.Summary)
	out.ProducedBindings = cloneExecutionBindings(in.ProducedBindings)
	out.Verification = cloneVerificationResult(in.Verification)
	return &out
}

func cloneVerificationResult(in *planner.VerificationResult) *planner.VerificationResult {
	if in == nil {
		return nil
	}
	out := *in
	out.Evidence = cloneMap(in.Evidence)
	return &out
}

func executionMemoryMap(memory ExecutionMemory) map[string]any {
	out := map[string]any{
		"last_created_track_id":        memory.LastCreatedTrackID,
		"last_created_track_name":      memory.LastCreatedTrackName,
		"last_created_clip_id":         memory.LastCreatedClipID,
		"last_created_clip_name":       memory.LastCreatedClipName,
		"last_loaded_plugin_id":        memory.LastLoadedPluginID,
		"last_loaded_plugin_name":      memory.LastLoadedPluginName,
		"active_work_target_track_id":  memory.ActiveWorkTargetTrackID,
		"active_work_target_clip_id":   memory.ActiveWorkTargetClipID,
		"active_work_target_plugin_id": memory.ActiveWorkTargetPluginID,
	}
	if len(memory.Bindings) > 0 {
		out["bindings"] = cloneExecutionBindings(memory.Bindings)
	}
	removeEmptyExecutionMap(out)
	return out
}

func recentObservationMap(observation *RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	out := map[string]any{
		"tool_call_id":      observation.ToolCallID,
		"tool":              observation.Tool,
		"command_name":      observation.CommandName,
		"status":            observation.Status,
		"error":             observation.Error,
		"summary":           cloneMap(observation.Summary),
		"mutation_barrier":  observation.MutationBarrier,
		"produced_bindings": cloneExecutionBindings(observation.ProducedBindings),
	}
	if observation.Verification != nil {
		out["verification"] = cloneVerificationResult(observation.Verification)
	}
	removeEmptyExecutionMap(out)
	return out
}

func removeEmptyExecutionMap(row map[string]any) {
	for key, value := range row {
		switch v := value.(type) {
		case nil:
			delete(row, key)
		case string:
			if strings.TrimSpace(v) == "" {
				delete(row, key)
			}
		case bool:
			if !v {
				delete(row, key)
			}
		case []ExecutionBinding:
			if len(v) == 0 {
				delete(row, key)
			}
		}
	}
}
