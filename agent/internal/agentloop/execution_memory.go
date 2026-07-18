package agentloop

import (
	"fmt"
	"strconv"
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/panlayout"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/staticbalance"
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
	LastCreatedTrackID            string                    `json:"last_created_track_id,omitempty"`
	LastCreatedTrackName          string                    `json:"last_created_track_name,omitempty"`
	LastCreatedFolderTrackID      string                    `json:"last_created_folder_track_id,omitempty"`
	LastCreatedFolderTrackName    string                    `json:"last_created_folder_track_name,omitempty"`
	LastCreatedClipID             string                    `json:"last_created_clip_id,omitempty"`
	LastCreatedClipName           string                    `json:"last_created_clip_name,omitempty"`
	LastLoadedPluginID            string                    `json:"last_loaded_plugin_id,omitempty"`
	LastLoadedPluginName          string                    `json:"last_loaded_plugin_name,omitempty"`
	LastMixTickID                 string                    `json:"last_mix_tick_id,omitempty"`
	ActiveWorkTargetTrackID       string                    `json:"active_work_target_track_id,omitempty"`
	ActiveWorkTargetFolderTrackID string                    `json:"active_work_target_folder_track_id,omitempty"`
	ActiveWorkTargetClipID        string                    `json:"active_work_target_clip_id,omitempty"`
	ActiveWorkTargetPluginID      string                    `json:"active_work_target_plugin_id,omitempty"`
	PendingMixTickCandidate       *PendingMixTickCandidate  `json:"pending_mix_tick_candidate,omitempty"`
	PendingStaticBalancePlan      *PendingStaticBalancePlan `json:"pending_static_balance_plan,omitempty"`
	PendingPanLayoutPlan          *PendingPanLayoutPlan     `json:"pending_pan_layout_plan,omitempty"`
	PendingMixTreatment           *MixTreatmentPending      `json:"pending_mix_treatment,omitempty"`
	PendingTrackOrganization      map[string]any            `json:"pending_track_organization,omitempty"`
	PendingSectionMarkers         map[string]any            `json:"pending_section_markers,omitempty"`
	MixDiagnosisContextID         string                    `json:"mix_diagnosis_context_id,omitempty"`
	MixDiagnosisContext           map[string]any            `json:"mix_diagnosis_context,omitempty"`
	Bindings                      []ExecutionBinding        `json:"bindings,omitempty"`
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

type PendingMixTickCandidate struct {
	Operation                 string         `json:"operation,omitempty"`
	TrackID                   string         `json:"track_id,omitempty"`
	DeltaDB                   float64        `json:"delta_db,omitempty"`
	DeltaPan                  float64        `json:"delta_pan,omitempty"`
	TargetPan                 *float64       `json:"target_pan,omitempty"`
	ObservationID             string         `json:"observation_id,omitempty"`
	Evidence                  map[string]any `json:"evidence,omitempty"`
	CreatedFromReply          string         `json:"created_from_reply,omitempty"`
	ExpiresAfterContextChange bool           `json:"expires_after_context_change,omitempty"`
	Status                    string         `json:"status,omitempty"`
	Fingerprint               map[string]any `json:"fingerprint,omitempty"`
}

type PendingStaticBalanceAction struct {
	Operation   string         `json:"operation"`
	TrackID     string         `json:"track_id"`
	TrackName   string         `json:"track_name,omitempty"`
	Role        string         `json:"role,omitempty"`
	Function    string         `json:"function,omitempty"`
	DeltaDB     float64        `json:"delta_db"`
	BeforeDB    float64        `json:"before_db"`
	TargetDB    float64        `json:"target_db"`
	Reason      string         `json:"reason,omitempty"`
	Evidence    map[string]any `json:"evidence,omitempty"`
	Fingerprint map[string]any `json:"fingerprint,omitempty"`
}

type PendingStaticBalancePlan struct {
	SchemaVersion             string                          `json:"schema_version"`
	PlanID                    string                          `json:"plan_id"`
	CapabilityID              string                          `json:"capability_id"`
	ContextPackID             string                          `json:"context_pack_id"`
	CandidatePlanID           string                          `json:"candidate_plan_id"`
	StyleID                   string                          `json:"style_id"`
	StyleName                 string                          `json:"style_name,omitempty"`
	ObservationID             string                          `json:"observation_id,omitempty"`
	Status                    string                          `json:"status"`
	Confidence                string                          `json:"confidence,omitempty"`
	Actions                   []PendingStaticBalanceAction    `json:"actions"`
	Assumptions               []string                        `json:"assumptions,omitempty"`
	Limitations               []string                        `json:"limitations,omitempty"`
	EvidenceRefs              []string                        `json:"evidence_refs,omitempty"`
	TrackCount                int                             `json:"track_count,omitempty"`
	DisclosedTrackCount       int                             `json:"disclosed_track_count,omitempty"`
	FunctionSummary           []staticbalance.FunctionSummary `json:"function_summary,omitempty"`
	DecisionReasoning         string                          `json:"decision_reasoning,omitempty"`
	ExpiresAfterContextChange bool                            `json:"expires_after_context_change,omitempty"`
}

type PendingPanLayoutAction struct {
	Operation   string         `json:"operation"`
	TrackID     string         `json:"track_id"`
	TrackName   string         `json:"track_name,omitempty"`
	Role        string         `json:"role,omitempty"`
	Function    string         `json:"function,omitempty"`
	BeforePan   float64        `json:"before_pan"`
	TargetPan   float64        `json:"target_pan"`
	DeltaPan    float64        `json:"delta_pan"`
	Reason      string         `json:"reason"`
	Evidence    map[string]any `json:"evidence,omitempty"`
	Fingerprint map[string]any `json:"fingerprint,omitempty"`
}

type PendingPanLayoutPlan struct {
	SchemaVersion             string                   `json:"schema_version"`
	PlanID                    string                   `json:"plan_id"`
	CapabilityID              string                   `json:"capability_id"`
	ContextPackID             string                   `json:"context_pack_id"`
	CandidatePlanID           string                   `json:"candidate_plan_id"`
	StyleID                   string                   `json:"style_id"`
	StyleName                 string                   `json:"style_name"`
	StyleHash                 string                   `json:"style_hash"`
	ObservationID             string                   `json:"observation_id,omitempty"`
	Status                    string                   `json:"status"`
	Confidence                string                   `json:"confidence"`
	Actions                   []PendingPanLayoutAction `json:"actions"`
	Assumptions               []string                 `json:"assumptions,omitempty"`
	Limitations               []string                 `json:"limitations,omitempty"`
	EvidenceRefs              []string                 `json:"evidence_refs,omitempty"`
	TrackCount                int                      `json:"track_count"`
	DisclosedTrackCount       int                      `json:"disclosed_track_count"`
	GroupSummary              []panlayout.GroupSummary `json:"group_summary,omitempty"`
	DecisionReasoning         string                   `json:"decision_reasoning,omitempty"`
	ExpiresAfterContextChange bool                     `json:"expires_after_context_change"`
}

type MixTreatmentPending struct {
	SchemaVersion             string         `json:"schema_version,omitempty"`
	Status                    string         `json:"status,omitempty"`
	ConversationID            string         `json:"conversation_id,omitempty"`
	ObservationID             string         `json:"observation_id,omitempty"`
	Intent                    string         `json:"intent,omitempty"`
	TargetRef                 string         `json:"target_ref,omitempty"`
	ActionKind                string         `json:"action_kind,omitempty"`
	ProcessorType             string         `json:"processor_type,omitempty"`
	DeltaDB                   float64        `json:"delta_db,omitempty"`
	DeltaPan                  float64        `json:"delta_pan,omitempty"`
	TargetPan                 *float64       `json:"target_pan,omitempty"`
	PluginID                  string         `json:"plugin_id,omitempty"`
	PluginName                string         `json:"plugin_name,omitempty"`
	Control                   string         `json:"control,omitempty"`
	Target                    map[string]any `json:"target,omitempty"`
	ReasoningSummary          string         `json:"reasoning_summary,omitempty"`
	Confidence                string         `json:"confidence,omitempty"`
	EvidenceRefs              []string       `json:"evidence_refs,omitempty"`
	DiagnosisContextID        string         `json:"diagnosis_context_id,omitempty"`
	DiagnosisContext          map[string]any `json:"diagnosis_context,omitempty"`
	NeedsResolution           []string       `json:"needs_resolution,omitempty"`
	ExpiresAfterContextChange bool           `json:"expires_after_context_change,omitempty"`
	CreatedFromReply          string         `json:"created_from_reply,omitempty"`
	Fingerprint               map[string]any `json:"fingerprint,omitempty"`
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
	case "folder_track.create", "track.folder.create", "track.create_folder":
		return bindCreatedFolderTrack(state, call, result, ver)
	case "project.apply_track_organization":
		return bindAppliedTrackOrganization(state, call, result)
	case "project.import_folder_as_stems", "project.import_audio_files", "import_folder_as_stems":
		bindPendingSectionMarkers(state, call, result)
		return bindPendingTrackOrganization(state, call, result)
	case "project.state", "get_project_state", "track.list", "list_tracks":
		return bindPendingTrackOrganizationFromProjectState(state, call, result)
	case "create_midi_clip", "insert_midi_clip", "import_midi_to_track", "midi.create_clip", "midi.insert_clip", "midi.import_file",
		"import_audio", "import_media_to_track", "add_audio_clip", "clip.import_audio", "clip.import_media_to_track", "clip.add_audio":
		return bindCreatedClip(state, call, result, ver)
	case "rack_add_node", "rack.add_node", "plugin.load_to_rack", "instantiate_plugin", "plugin.instantiate":
		return bindLoadedPlugin(state, call, result, ver)
	case "mix_propose_tick", "mix.propose_tick":
		return bindMixTick(state, call, result)
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

func bindCreatedFolderTrack(state *runState, call planner.ToolCall, result executorpkg.Result, ver planner.VerificationResult) []ExecutionBinding {
	folderTrackID := firstNonEmpty(
		firstMapText(result.Result, "folder_track_id", "track_id", "id", "item_id"),
		firstMapText(ver.Evidence, "observed_folder_track_id", "expected_folder_track_id", "observed_track_id", "expected_track_id"),
		firstMapText(call.Args, "folder_track_id", "track_id", "selected_track_id", "target_track_id"),
	)
	folderTrackName := firstNonEmpty(
		firstMapText(result.Result, "folder_name", "track_name", "name"),
		firstMapText(ver.Evidence, "observed_folder_track_name", "expected_folder_track_name", "observed_track_name", "expected_track_name"),
		firstMapText(call.Args, "folder_name", "track_name", "name"),
	)
	if folderTrackID == "" && folderTrackName == "" {
		return nil
	}
	state.executionMemory.LastCreatedFolderTrackID = folderTrackID
	state.executionMemory.LastCreatedFolderTrackName = folderTrackName
	state.executionMemory.ActiveWorkTargetFolderTrackID = folderTrackID
	bindings := []ExecutionBinding{
		executionBinding("last_created_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "tool_result"),
		executionBinding("active_work_target_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "created_object"),
		executionBinding("target_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "created_object"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func bindAppliedTrackOrganization(state *runState, call planner.ToolCall, result executorpkg.Result) []ExecutionBinding {
	folderTrackID := firstNonEmpty(
		firstMapText(result.Result, "folder_track_id", "track_id", "id", "item_id"),
		lastTextFromAny(result.Result["created_folder_ids"]),
	)
	folderTrackName := firstNonEmpty(
		firstMapText(result.Result, "folder_name", "track_name", "name"),
		lastTrackOrganizationGroupName(result.Result["groups"]),
	)
	if folderTrackID == "" && folderTrackName == "" {
		return nil
	}
	state.executionMemory.LastCreatedFolderTrackID = folderTrackID
	state.executionMemory.LastCreatedFolderTrackName = folderTrackName
	state.executionMemory.ActiveWorkTargetFolderTrackID = folderTrackID
	state.executionMemory.PendingTrackOrganization = nil
	bindings := []ExecutionBinding{
		executionBinding("last_created_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "tool_result"),
		executionBinding("active_work_target_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "created_object"),
		executionBinding("target_folder_track", "track", folderTrackID, folderTrackName, "", call, result, "created_object"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func bindPendingTrackOrganization(state *runState, call planner.ToolCall, result executorpkg.Result) []ExecutionBinding {
	if state == nil || len(result.Result) == 0 {
		return nil
	}
	proj := messageLoopTOMProjectionForImportResult(result.Result)
	if len(proj) == 0 {
		return nil
	}
	pending := pendingTrackOrganizationMemory(proj, call, result)
	if len(pending) == 0 {
		return nil
	}
	state.executionMemory.PendingTrackOrganization = pending
	return nil
}

func bindPendingTrackOrganizationFromProjectState(state *runState, call planner.ToolCall, result executorpkg.Result) []ExecutionBinding {
	if state == nil || len(result.Result) == 0 || !messageLoopTrackOrganizationIntent(state.input.UserText) {
		return nil
	}
	rows := messageLoopTOMRowsFromProjectState(result.Result)
	if len(rows) == 0 {
		return nil
	}
	proj := messageLoopTOMProjectionForImportResult(map[string]any{
		"summary":             map[string]any{"tracks_created": len(rows), "track_count": len(rows)},
		"imported_track_refs": rows,
	})
	if len(proj) == 0 {
		return nil
	}
	pending := pendingTrackOrganizationMemory(proj, call, result)
	if len(pending) == 0 {
		return nil
	}
	pending["source"] = "project_state_rebuilt_tom"
	pending["instruction"] = "This TOM/A3 organization proposal was rebuilt from the full current project state because no completed-goal pending manifest was available; use it with project.apply_track_organization if the user confirms."
	state.executionMemory.PendingTrackOrganization = pending
	return nil
}

func messageLoopTrackOrganizationIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasTrackContext := strings.Contains(text, "track") || strings.Contains(text, "tracks") || strings.Contains(text, "轨") || strings.Contains(text, "工程")
	hasOrganization := strings.Contains(text, "organize") || strings.Contains(text, "organisation") || strings.Contains(text, "organization") ||
		strings.Contains(text, "group") || strings.Contains(text, "folder") || strings.Contains(text, "tom") ||
		strings.Contains(text, "整理") || strings.Contains(text, "分组") || strings.Contains(text, "文件夹") || strings.Contains(text, "建议")
	return hasOrganization && (hasTrackContext || strings.Contains(text, "folder") || strings.Contains(text, "文件夹"))
}

func messageLoopTOMRowsFromProjectState(result map[string]any) []map[string]any {
	tracks := messageLoopMapRows(result["tracks"])
	if len(tracks) == 0 {
		if state := messageLoopMapValue(result["state"]); len(state) > 0 {
			tracks = messageLoopMapRows(state["tracks"])
		}
	}
	rows := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		if len(track) == 0 || messageLoopProjectStateTrackIsFolder(track) {
			continue
		}
		trackID := firstMapText(track, "track_id", "id")
		trackName := firstMapText(track, "name", "track_name")
		clips := messageLoopMapRows(track["clips"])
		if len(clips) == 0 {
			rows = append(rows, map[string]any{
				"track_id":   trackID,
				"track_name": trackName,
			})
			continue
		}
		clip := clips[0]
		rows = append(rows, map[string]any{
			"track_id":         trackID,
			"track_name":       trackName,
			"clip_id":          firstMapText(clip, "clip_id", "id"),
			"clip_name":        firstMapText(clip, "name", "clip_name", "file_name"),
			"source_file_path": firstMapText(clip, "current_source_path", "source_file_path", "source_path", "file_path"),
			"duration_seconds": firstPresentNumber(clip, "length_seconds", "duration_seconds", "end_seconds"),
			"start_seconds":    firstPresentNumber(clip, "start_seconds", "start_time_seconds"),
			"channel_count":    firstPositiveMapInt(track, "channel_count", "channels"),
		})
	}
	return rows
}

func messageLoopProjectStateTrackIsFolder(track map[string]any) bool {
	if isFolder, ok := firstMapBool(track, "is_folder_track", "is_folder_container", "can_contain_child_tracks"); ok && isFolder {
		return true
	}
	trackType := strings.ToLower(firstMapText(track, "track_type", "type"))
	return strings.Contains(trackType, "folder")
}

func firstPresentNumber(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := executionMemoryNumberFromAny(row[key]); ok {
			return value
		}
	}
	return 0
}

func executionMemoryNumberFromAny(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func pendingTrackOrganizationMemory(proj map[string]any, call planner.ToolCall, result executorpkg.Result) map[string]any {
	manifest := messageLoopMapValue(proj["full_assignment_manifest"])
	if len(manifest) == 0 {
		return nil
	}
	groups := messageLoopMapRows(manifest["groups"])
	if len(groups) == 0 {
		return nil
	}
	memoryGroups := []map[string]any{}
	for _, group := range groups {
		trackIDs := messageLoopStringSliceFromAny(group["track_ids"])
		if len(trackIDs) == 0 {
			continue
		}
		row := map[string]any{
			"group_id":            firstMapText(group, "group_id"),
			"label":               firstMapText(group, "label"),
			"folder_name":         firstMapText(group, "proposed_folder", "folder_name"),
			"track_count":         len(trackIDs),
			"track_ids_csv":       strings.Join(trackIDs, ", "),
			"confidence":          firstMapText(group, "confidence"),
			"needs_confirmation":  true,
			"routing_bus_enabled": false,
		}
		if count := firstPositiveMapInt(group, "track_count"); count > 0 {
			row["track_count"] = count
		}
		if needs, ok := firstMapBool(group, "needs_confirmation"); ok {
			row["needs_confirmation"] = needs
		}
		memoryGroups = append(memoryGroups, row)
	}
	if len(memoryGroups) == 0 {
		return nil
	}
	disclosure := messageLoopMapValue(proj["disclosure_plan"])
	summary := messageLoopMapValue(proj["organization_summary"])
	return map[string]any{
		"schema_version":              firstMapText(proj, "schema_version"),
		"tom_version":                 firstMapText(proj, "tom_version"),
		"status":                      firstMapText(proj, "status"),
		"source_tool_call_id":         firstNonEmpty(result.ToolCallID, call.ID),
		"source_tool":                 firstNonEmpty(result.Tool, call.Tool),
		"track_count":                 firstPositiveMapInt(manifest, "track_count", "assignment_coverage_count"),
		"assignment_coverage_count":   firstPositiveMapInt(manifest, "assignment_coverage_count"),
		"unique_track_count":          firstPositiveMapInt(manifest, "unique_track_count"),
		"coverage_status":             firstMapText(manifest, "coverage_status"),
		"selected_disclosure_stage":   firstMapText(disclosure, "selected_stage"),
		"naming_matched_track_count":  firstPositiveMapInt(summary, "naming_matched_track_count"),
		"technical_fallback_count":    firstPositiveMapInt(summary, "technical_fallback_track_count"),
		"groups":                      memoryGroups,
		"tool_to_use_after_confirmed": "project.apply_track_organization",
		"tool_argument_shape":         "groups:[{folder_name:string, track_ids:string[], routing_bus_enabled:false}]",
		"instruction":                 "If the user confirms this TOM/A3 organization proposal, use these complete group track IDs with project.apply_track_organization; do not fall back to visible_tracks.",
	}
}

func bindPendingSectionMarkers(state *runState, call planner.ToolCall, result executorpkg.Result) []ExecutionBinding {
	if state == nil || len(result.Result) == 0 {
		return nil
	}
	proj := messageLoopEPMProjectionForImportResult(result.Result)
	if len(proj) == 0 {
		return nil
	}
	pending := pendingSectionMarkersMemory(proj, call, result)
	if len(pending) == 0 {
		return nil
	}
	state.executionMemory.PendingSectionMarkers = pending
	return nil
}

func pendingSectionMarkersMemory(proj map[string]any, call planner.ToolCall, result executorpkg.Result) map[string]any {
	sectionMap := messageLoopMapValue(proj["section_map"])
	if len(sectionMap) == 0 {
		return nil
	}
	sourceSections := messageLoopMapRows(sectionMap["sections"])
	if len(sourceSections) == 0 {
		return nil
	}
	sections := []map[string]any{}
	for i, row := range sourceSections {
		start := firstPresentNumber(row, "start_seconds", "start")
		end := firstPresentNumber(row, "end_seconds", "end")
		if end <= start {
			continue
		}
		label := firstNonEmpty(firstMapText(row, "label", "name"), fmt.Sprintf("Section %d", i+1))
		sectionID := firstNonEmpty(firstMapText(row, "section_id", "id"), fmt.Sprintf("section_%02d", i+1))
		out := map[string]any{
			"section_id":       sectionID,
			"name":             label,
			"label":            label,
			"start_seconds":    start,
			"end_seconds":      end,
			"duration_seconds": end - start,
			"confidence":       firstMapText(row, "confidence"),
		}
		if basis := messageLoopStringSliceFromAny(row["basis"]); len(basis) > 0 {
			out["basis"] = basis
		}
		sections = append(sections, out)
	}
	if len(sections) == 0 {
		return nil
	}
	return map[string]any{
		"schema_version":              firstMapText(proj, "schema_version"),
		"epm_version":                 firstMapText(proj, "epm_version"),
		"status":                      firstMapText(sectionMap, "status"),
		"source_tool_call_id":         firstNonEmpty(result.ToolCallID, call.ID),
		"source_tool":                 firstNonEmpty(result.Tool, call.Tool),
		"section_count":               len(sections),
		"duration_seconds":            firstPresentNumber(sectionMap, "duration_seconds"),
		"coverage_seconds":            firstPresentNumber(sectionMap, "coverage_seconds"),
		"confidence":                  firstMapText(sectionMap, "confidence"),
		"reference_strategy":          firstMapText(sectionMap, "reference_strategy"),
		"sections":                    sections,
		"replace_existing":            true,
		"source":                      "epm_a5",
		"created_by":                  "agent",
		"tool_to_use_after_confirmed": "project.markers.apply_section_markers",
		"tool_argument_shape":         "sections:[{name:string,start_seconds:number,end_seconds:number,confidence?:string}], replace_existing:true, source:\"epm_a5\"",
		"instruction":                 "If the user confirms the A5/EPM section map, write these complete sections with project.markers.apply_section_markers. Do not move clips or tracks.",
	}
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

func bindMixTick(state *runState, call planner.ToolCall, result executorpkg.Result) []ExecutionBinding {
	tickID := firstMapText(result.Result, "tick_id")
	if tickID == "" || !strings.EqualFold(strings.TrimSpace(result.Status), "ok") {
		return nil
	}
	trackID := firstNonEmpty(
		firstMapText(result.Result, "track_id"),
		firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id"),
	)
	state.executionMemory.LastMixTickID = tickID
	if trackID != "" {
		state.executionMemory.ActiveWorkTargetTrackID = trackID
	}
	bindings := []ExecutionBinding{
		executionBinding("last_mix_tick", "mix_tick", tickID, "", trackID, call, result, "tool_result"),
	}
	addExecutionBindings(&state.executionMemory, bindings...)
	return bindings
}

func lastTextFromAny(value any) string {
	switch v := value.(type) {
	case []string:
		for i := len(v) - 1; i >= 0; i-- {
			if text := strings.TrimSpace(v[i]); text != "" {
				return text
			}
		}
	case []any:
		for i := len(v) - 1; i >= 0; i-- {
			if text := strings.TrimSpace(fmt.Sprint(v[i])); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func lastTrackOrganizationGroupName(value any) string {
	switch v := value.(type) {
	case []map[string]any:
		for i := len(v) - 1; i >= 0; i-- {
			if text := firstMapText(v[i], "folder_name", "name", "group_name", "label"); text != "" {
				return text
			}
		}
	case []any:
		for i := len(v) - 1; i >= 0; i-- {
			if row, ok := v[i].(map[string]any); ok {
				if text := firstMapText(row, "folder_name", "name", "group_name", "label"); text != "" {
					return text
				}
			}
		}
	}
	return ""
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
	out.PendingMixTickCandidate = clonePendingMixTickCandidate(in.PendingMixTickCandidate)
	out.PendingStaticBalancePlan = clonePendingStaticBalancePlan(in.PendingStaticBalancePlan)
	out.PendingPanLayoutPlan = clonePendingPanLayoutPlan(in.PendingPanLayoutPlan)
	out.PendingMixTreatment = cloneMixTreatmentPending(in.PendingMixTreatment)
	out.PendingTrackOrganization = cloneMap(in.PendingTrackOrganization)
	out.PendingSectionMarkers = cloneMap(in.PendingSectionMarkers)
	out.MixDiagnosisContext = cloneMap(in.MixDiagnosisContext)
	out.Bindings = cloneExecutionBindings(in.Bindings)
	return out
}

func clonePendingStaticBalancePlan(in *PendingStaticBalancePlan) *PendingStaticBalancePlan {
	if in == nil {
		return nil
	}
	out := *in
	out.Actions = make([]PendingStaticBalanceAction, len(in.Actions))
	for i, action := range in.Actions {
		out.Actions[i] = action
		out.Actions[i].Evidence = cloneMap(action.Evidence)
		out.Actions[i].Fingerprint = cloneMap(action.Fingerprint)
	}
	out.Assumptions = append([]string(nil), in.Assumptions...)
	out.Limitations = append([]string(nil), in.Limitations...)
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	out.FunctionSummary = append([]staticbalance.FunctionSummary(nil), in.FunctionSummary...)
	return &out
}

func clonePendingPanLayoutPlan(in *PendingPanLayoutPlan) *PendingPanLayoutPlan {
	if in == nil {
		return nil
	}
	out := *in
	out.Actions = make([]PendingPanLayoutAction, len(in.Actions))
	for i, action := range in.Actions {
		out.Actions[i] = action
		out.Actions[i].Evidence = cloneMap(action.Evidence)
		out.Actions[i].Fingerprint = cloneMap(action.Fingerprint)
	}
	out.Assumptions = append([]string(nil), in.Assumptions...)
	out.Limitations = append([]string(nil), in.Limitations...)
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	out.GroupSummary = append([]panlayout.GroupSummary(nil), in.GroupSummary...)
	return &out
}

func clonePendingMixTickCandidate(in *PendingMixTickCandidate) *PendingMixTickCandidate {
	if in == nil {
		return nil
	}
	out := *in
	if in.TargetPan != nil {
		targetPan := *in.TargetPan
		out.TargetPan = &targetPan
	}
	out.Evidence = cloneMap(in.Evidence)
	out.Fingerprint = cloneMap(in.Fingerprint)
	return &out
}

func cloneMixTreatmentPending(in *MixTreatmentPending) *MixTreatmentPending {
	if in == nil {
		return nil
	}
	out := *in
	if in.TargetPan != nil {
		targetPan := *in.TargetPan
		out.TargetPan = &targetPan
	}
	out.Target = cloneMap(in.Target)
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	out.DiagnosisContext = cloneMap(in.DiagnosisContext)
	out.NeedsResolution = append([]string(nil), in.NeedsResolution...)
	out.Fingerprint = cloneMap(in.Fingerprint)
	return &out
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
		"last_created_track_id":              memory.LastCreatedTrackID,
		"last_created_track_name":            memory.LastCreatedTrackName,
		"last_created_folder_track_id":       memory.LastCreatedFolderTrackID,
		"last_created_folder_track_name":     memory.LastCreatedFolderTrackName,
		"last_created_clip_id":               memory.LastCreatedClipID,
		"last_created_clip_name":             memory.LastCreatedClipName,
		"last_loaded_plugin_id":              memory.LastLoadedPluginID,
		"last_loaded_plugin_name":            memory.LastLoadedPluginName,
		"last_mix_tick_id":                   memory.LastMixTickID,
		"active_work_target_track_id":        memory.ActiveWorkTargetTrackID,
		"active_work_target_folder_track_id": memory.ActiveWorkTargetFolderTrackID,
		"active_work_target_clip_id":         memory.ActiveWorkTargetClipID,
		"active_work_target_plugin_id":       memory.ActiveWorkTargetPluginID,
	}
	if memory.PendingMixTickCandidate != nil {
		out["pending_mix_tick_candidate"] = clonePendingMixTickCandidate(memory.PendingMixTickCandidate)
	}
	if memory.PendingStaticBalancePlan != nil {
		out["pending_static_balance_plan"] = clonePendingStaticBalancePlan(memory.PendingStaticBalancePlan)
	}
	if memory.PendingPanLayoutPlan != nil {
		out["pending_pan_layout_plan"] = clonePendingPanLayoutPlan(memory.PendingPanLayoutPlan)
	}
	if memory.PendingMixTreatment != nil {
		out["pending_mix_treatment"] = cloneMixTreatmentPending(memory.PendingMixTreatment)
	}
	if len(memory.PendingTrackOrganization) > 0 {
		out["pending_track_organization"] = cloneMap(memory.PendingTrackOrganization)
	}
	if len(memory.PendingSectionMarkers) > 0 {
		out["pending_section_markers"] = cloneMap(memory.PendingSectionMarkers)
	}
	if strings.TrimSpace(memory.MixDiagnosisContextID) != "" {
		out["mix_diagnosis_context_id"] = strings.TrimSpace(memory.MixDiagnosisContextID)
	}
	if len(memory.MixDiagnosisContext) > 0 {
		out["mix_diagnosis_context"] = cloneMap(memory.MixDiagnosisContext)
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
