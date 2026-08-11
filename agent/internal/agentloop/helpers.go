package agentloop

import (
	"strings"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func (r *Runner) buildContextSnapshot(state *runState) contextruntime.Snapshot {
	if state == nil {
		return contextruntime.Build(contextruntime.Input{}, contextruntime.Options{Now: r.now})
	}
	return contextruntime.Build(contextruntime.Input{
		GoalID:                state.goal.GoalID,
		RunID:                 state.goal.RunID,
		UserText:              state.input.UserText,
		GoalSummary:           firstNonEmpty(state.input.Summary, state.goal.Summary, state.input.UserText),
		Conversation:          append([]llm.Message(nil), state.input.Conversation...),
		GoalTrace:             append([]planner.TraceEvent(nil), state.trace...),
		Context:               cloneMap(state.input.Context),
		State:                 cloneMap(state.input.State),
		ProjectHistorySummary: cloneMap(projectHistoryFromState(state)),
		PlanItems:             append([]planner.PlanItem(nil), state.planItems...),
		PendingToolCall:       cloneToolCallPtrValue(state.pendingToolCall),
		PendingToolQueue:      append([]planner.ToolCall(nil), state.pendingToolQueue...),
		ExecutionMemory:       executionMemoryMap(state.executionMemory),
		RecentObservation:     recentObservationMap(state.recentObservation),
		PreviousSnapshot:      cloneMap(state.contextSnapshot),
	}, contextruntime.Options{Now: r.now})
}

// buildModelContextSnapshot returns the model-visible projection. The runner
// keeps the complete snapshot in state.contextSnapshot for deterministic
// binding and recovery, but processor-family reasoning must not see loaded
// plug-in identity or topology before it has chosen a family.
func (r *Runner) buildModelContextSnapshot(state *runState, full contextruntime.Snapshot) string {
	if state == nil {
		return "{}"
	}
	modelSnapshot := full
	if messageLoopNeedsNeutralFamilyProjection(state) {
		neutralContext := messageLoopPreFamilyContextProjection(state.input.Context)
		neutralState := messageLoopPreFamilyStateProjection(state.input.State)
		neutralTrace := messageLoopPreFamilyTraceProjection(state.trace)
		neutralObservation := map[string]any{}
		if value, ok := messageLoopPreFamilyValue(recentObservationMap(state.recentObservation)).(map[string]any); ok {
			neutralObservation = value
		}
		modelSnapshot = contextruntime.Build(contextruntime.Input{
			GoalID:            state.goal.GoalID,
			RunID:             state.goal.RunID,
			UserText:          state.input.UserText,
			GoalSummary:       firstNonEmpty(state.input.Summary, state.goal.Summary, state.input.UserText),
			GoalTrace:         neutralTrace,
			Context:           neutralContext,
			State:             neutralState,
			RecentObservation: neutralObservation,
		}, contextruntime.Options{Now: r.now, SkipPluginSemanticLoad: true})
	}
	projected := contextruntime.ProjectModelSnapshot(contextruntime.ModelProjectionInput{
		Snapshot:          modelSnapshot,
		GoalTrace:         state.trace,
		RecentObservation: recentObservationMap(state.recentObservation),
	}, contextruntime.Options{Now: r.now, SkipPluginSemanticLoad: true})
	return contextruntime.ModelJSON(projected)
}

func messageLoopNeedsNeutralFamilyProjection(state *runState) bool {
	// Processor materialization is normally local and never re-enters the
	// model. If it does, fail safe: every active free-state turn gets the same
	// neutral prompt assembly and model-visible projection.
	return state != nil && messageLoopFreeStateActive(state)
}

func messageLoopPreFamilyContextProjection(source map[string]any) map[string]any {
	out := map[string]any{}
	// IDs are retained only to preserve target binding. No selected plug-in
	// identity, product, vendor, path, topology, or coverage hint is copied.
	for _, key := range []string{
		"selected_track_id", "selected_scene_track_id", "selected_clip_id",
		"selected_clip_track_id", "piano_roll_focus_clip_id", "piano_roll_focus_track_id",
		"selected_clip_ids", "selected_clip_ranges", "playhead_seconds",
		"current_playhead_seconds", "transport_position_seconds",
		"semantic_entry_verified", "semantic_entry_decision",
	} {
		if value, ok := source[key]; ok && value != nil {
			out[key] = messageLoopPreFamilyValue(value)
		}
	}
	if out["selected_track_id"] == nil {
		if trackID := firstMapText(source, "selected_plugin_track_id"); trackID != "" {
			out["selected_track_id"] = trackID
		}
	}
	return out
}

func messageLoopPreFamilyStateProjection(source map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"initialized", "track_count", "user_track_count", "engine_track_count", "internal_track_count", "graph_revision", "project_revision"} {
		if value, ok := source[key]; ok && value != nil {
			out[key] = messageLoopPreFamilyValue(value)
		}
	}
	return out
}

func messageLoopPreFamilyTraceProjection(trace []planner.TraceEvent) []planner.TraceEvent {
	out := make([]planner.TraceEvent, 0, len(trace))
	for _, event := range trace {
		keep := false
		if event.ToolCall != nil && messageLoopIsCCBObservationTool(*event.ToolCall) {
			keep = true
		}
		if event.ToolResult != nil && strings.HasPrefix(strings.ToLower(strings.TrimSpace(event.ToolResult.Tool)), "ccb.observation_") {
			keep = true
		}
		if !keep {
			continue
		}
		copyEvent := event
		if event.ToolCall != nil {
			call := *event.ToolCall
			call.Args = messageLoopPreFamilyMap(call.Args)
			copyEvent.ToolCall = &call
		}
		if event.ToolResult != nil {
			result := *event.ToolResult
			result.Result = messageLoopPreFamilyToolResultProjection(result)
			copyEvent.ToolResult = &result
		}
		out = append(out, copyEvent)
	}
	return recentTrace(out, 12)
}

// messageLoopPreFamilyToolResultProjection keeps CCB's neutral decision
// surface, rather than copying its transport-sized result into the next model
// turn. The model still receives the view IDs and their neutral metadata, but
// never needs opaque kernel detail to decide what to observe next.
func messageLoopPreFamilyToolResultProjection(result planner.ToolResult) map[string]any {
	switch {
	case messageLoopIsCCBObservationCatalogName(result.Tool):
		return messageLoopPreFamilyCatalogProjection(result.Result)
	case messageLoopIsCCBObservationRequestName(result.Tool):
		bundle := messageLoopMapValue(result.Result["bundle"])
		if len(bundle) == 0 {
			return messageLoopPreFamilyMap(compactSelectedKeys(result.Result, []string{"status", "reason", "error"}))
		}
		// The complete bundle is already carried once by RecentObservation. Keep
		// only receipt-level facts in the trace to avoid multiplying large view
		// payloads on every neutral reasoning turn.
		return messageLoopPreFamilyMap(compactSelectedKeys(bundle, []string{
			"schema_version", "bundle_id", "request_id", "status", "observation_id",
			"target_ref", "project_binding", "freshness", "requested_views",
			"evidence_refs", "limitations", "omission_reasons", "omissions",
			"disclosure_bytes", "max_disclosure_bytes",
		}))
	}
	return messageLoopPreFamilyMap(result.Result)
}

func messageLoopIsCCBObservationCatalogName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ccb.observation_catalog", "ccb_observation_catalog":
		return true
	default:
		return false
	}
}

func messageLoopPreFamilyCatalogProjection(result map[string]any) map[string]any {
	catalog := messageLoopMapValue(result["catalog"])
	if len(catalog) == 0 {
		return messageLoopPreFamilyMap(compactSelectedKeys(result, []string{"status", "reason", "error"}))
	}
	views := make([]map[string]any, 0)
	for _, view := range messageLoopMapRows(catalog["views"]) {
		views = append(views, compactSelectedKeys(view, []string{
			"view_id", "questions", "supported_target_kinds", "temporal_resolution",
			"availability", "quality_ceiling", "limitations", "required_dependencies",
		}))
	}
	out := compactSelectedKeys(catalog, []string{"schema_version", "boundary", "target_ref", "exclusions"})
	if len(views) > 0 {
		out["views"] = views
	}
	return messageLoopPreFamilyMap(out)
}

func messageLoopPreFamilyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return messageLoopPreFamilyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, messageLoopPreFamilyValue(item))
		}
		return out
	default:
		return value
	}
}

func messageLoopPreFamilyMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]any, len(source))
	for key, value := range source {
		if messageLoopPreFamilyForbiddenKey(key) {
			continue
		}
		out[key] = messageLoopPreFamilyValue(value)
	}
	return out
}

func messageLoopPreFamilyForbiddenKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, marker := range []string{
		"plugin", "vendor", "manufacturer", "product", "generic_eq_topology",
		"identity_card", "processor_identity", "topology", "parameter_id", "param_id",
		"required_coverage", "default_coverage", "coverage_axes", "expected_view",
		"recommended_family", "recommended_processor", "processor_type",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

func recentTrace(trace []planner.TraceEvent, max int) []planner.TraceEvent {
	if max <= 0 || len(trace) <= max {
		return append([]planner.TraceEvent(nil), trace...)
	}
	return append([]planner.TraceEvent(nil), trace[len(trace)-max:]...)
}

func cloneToolCallPtr(call planner.ToolCall) *planner.ToolCall {
	out := call
	out.Args = cloneMap(call.Args)
	out.Command = cloneMap(call.Command)
	return &out
}

func cloneToolCallPtrValue(call *planner.ToolCall) *planner.ToolCall {
	if call == nil {
		return nil
	}
	return cloneToolCallPtr(*call)
}
