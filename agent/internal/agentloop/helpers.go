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
