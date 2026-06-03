package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type MessageCompleter interface {
	Complete(context.Context, config.EngineConfig, []llm.Message) (string, error)
}

type MessageLoop struct {
	Runtime  *agentruntime.Runtime
	Client   MessageCompleter
	Config   config.EngineConfig
	Executor ToolExecutor
	Budget   Budget
	Now      func() time.Time
}

type messageLoopOutput struct {
	Final                 bool               `json:"final"`
	Reply                 string             `json:"reply,omitempty"`
	NeedsClarification    bool               `json:"needs_clarification,omitempty"`
	ClarificationQuestion string             `json:"clarification_question,omitempty"`
	FailureReason         string             `json:"failure_reason,omitempty"`
	ToolCalls             []planner.ToolCall `json:"tool_calls,omitempty"`
}

func (l *MessageLoop) Start(ctx context.Context, in Input) Result {
	r := l.runner()
	goal := r.ensureGoal(in.GoalID, in.RunID, firstNonEmpty(in.Summary, in.UserText))
	state := runState{
		input:             in,
		goal:              goal,
		trace:             append([]planner.TraceEvent(nil), in.Trace...),
		planItems:         mergePlanItems(nil, in.PlanItems),
		contextSnapshot:   cloneMap(in.ContextSnapshot),
		projectHistory:    cloneMap(in.ProjectHistory),
		executionMemory:   cloneExecutionMemory(in.ExecutionMemory),
		recentObservation: cloneRecentObservation(in.RecentObservation),
		budget:            normalizeBudget(firstNonZeroBudget(in.Budget, l.Budget)),
		startedAt:         r.now(),
	}
	state.input.GoalID = goal.GoalID
	state.input.RunID = goal.RunID
	state.input.Conversation = messageLoopInitialTranscript(in)
	return l.loop(ctx, r, &state)
}

func (l *MessageLoop) Continue(ctx context.Context, cont Continuation) Result {
	r := l.runner()
	if r.Runtime == nil {
		return Result{Status: agentruntime.StatusFailed, StopReason: StopReasonFailed, Error: "goal runtime is nil"}
	}
	goal := r.Runtime.Continue(cont.GoalID, cont.Summary)
	if strings.TrimSpace(cont.Summary) != "" {
		goal.Summary = strings.TrimSpace(cont.Summary)
	}
	goal = r.Runtime.ClearInterjections(goal.GoalID)
	state := runState{
		input: Input{
			GoalID:            goal.GoalID,
			RunID:             goal.RunID,
			UserText:          cont.UserText,
			Summary:           cont.Summary,
			Context:           cloneMap(cont.Context),
			ContextSnapshot:   cloneMap(cont.ContextSnapshot),
			ProjectHistory:    cloneMap(cont.ProjectHistory),
			Conversation:      append([]llm.Message(nil), cont.Conversation...),
			State:             cloneMap(cont.State),
			CatalogSummary:    cont.CatalogSummary,
			AllowedTools:      append([]string(nil), cont.AllowedTools...),
			Trace:             append([]planner.TraceEvent(nil), cont.Trace...),
			PlanItems:         append([]planner.PlanItem(nil), cont.PlanItems...),
			Budget:            cont.Budget,
			ExecutionMemory:   cloneExecutionMemory(cont.ExecutionMemory),
			RecentObservation: cloneRecentObservation(cont.RecentObservation),
		},
		goal:              goal,
		trace:             append([]planner.TraceEvent(nil), cont.Trace...),
		planItems:         mergePlanItems(nil, cont.PlanItems),
		pendingToolQueue:  append([]planner.ToolCall(nil), cont.PendingToolQueue...),
		completedSteps:    cont.CompletedSteps,
		turnsUsed:         cont.TurnsUsed,
		toolCallsUsed:     cont.ToolCallsUsed,
		contextSnapshot:   cloneMap(cont.ContextSnapshot),
		projectHistory:    cloneMap(cont.ProjectHistory),
		executionMemory:   cloneExecutionMemory(cont.ExecutionMemory),
		recentObservation: cloneRecentObservation(cont.RecentObservation),
		budget:            normalizeBudget(firstNonZeroBudget(cont.Budget, l.Budget)),
		startedAt:         r.now(),
	}
	return l.loop(ctx, r, &state)
}

func (l *MessageLoop) ResumeAfterConfirmation(ctx context.Context, cont Continuation) Result {
	r := l.runner()
	goal := r.ensureGoal(cont.GoalID, cont.RunID, cont.Summary)
	state := runState{
		input: Input{
			GoalID:            goal.GoalID,
			RunID:             goal.RunID,
			UserText:          cont.UserText,
			Summary:           cont.Summary,
			Context:           cloneMap(cont.Context),
			ContextSnapshot:   cloneMap(cont.ContextSnapshot),
			ProjectHistory:    cloneMap(cont.ProjectHistory),
			Conversation:      append([]llm.Message(nil), cont.Conversation...),
			State:             cloneMap(cont.State),
			CatalogSummary:    cont.CatalogSummary,
			AllowedTools:      append([]string(nil), cont.AllowedTools...),
			Trace:             append([]planner.TraceEvent(nil), cont.Trace...),
			PlanItems:         append([]planner.PlanItem(nil), cont.PlanItems...),
			Budget:            cont.Budget,
			ExecutionMemory:   cloneExecutionMemory(cont.ExecutionMemory),
			RecentObservation: cloneRecentObservation(cont.RecentObservation),
		},
		goal:              goal,
		trace:             append([]planner.TraceEvent(nil), cont.Trace...),
		planItems:         mergePlanItems(nil, cont.PlanItems),
		pendingToolQueue:  append([]planner.ToolCall(nil), cont.PendingToolQueue...),
		contextSnapshot:   cloneMap(cont.ContextSnapshot),
		projectHistory:    cloneMap(cont.ProjectHistory),
		executionMemory:   cloneExecutionMemory(cont.ExecutionMemory),
		recentObservation: cloneRecentObservation(cont.RecentObservation),
		completedSteps:    cont.CompletedSteps,
		turnsUsed:         cont.TurnsUsed,
		toolCallsUsed:     cont.ToolCallsUsed,
		budget:            normalizeBudget(firstNonZeroBudget(cont.Budget, l.Budget)),
		startedAt:         r.now(),
	}
	if cont.PendingToolCall == nil {
		return r.fail(&state, fmt.Errorf("confirmation continuation is missing pending tool call"))
	}
	if stopped, result := r.checkpoint("before_confirmed_tool", &state); stopped {
		return result
	}
	if limit, result := r.checkToolBudget(&state); limit {
		return result
	}
	call := resolveMessageLoopBindings(&state, *cont.PendingToolCall)
	if stopped, result := r.executeTool(ctx, &state, call, true, nil); stopped {
		return result
	}
	appendMessageLoopToolResult(&state)
	return l.loop(ctx, r, &state)
}

func (l *MessageLoop) loop(ctx context.Context, r *Runner, state *runState) Result {
	if l == nil || l.Client == nil {
		return r.fail(state, fmt.Errorf("agent message loop LLM client is nil"))
	}
	if !l.Config.Complete() {
		return r.fail(state, fmt.Errorf("agent message loop LLM config incomplete"))
	}
	for {
		if stopped, result := r.checkpoint("before_message_loop_model", state); stopped {
			return result
		}
		if limit, result := r.checkTurnBudget(state); limit {
			return result
		}
		snapshot := r.buildContextSnapshot(state)
		state.contextSnapshot = snapshot.Map()
		raw, err := l.Client.Complete(ctx, l.Config, l.messages(state, snapshot.JSON()))
		state.turnsUsed++
		if err != nil {
			return r.fail(state, err)
		}
		out, err := parseMessageLoopOutput(raw)
		if err != nil {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_error", Message: err.Error(), Reply: raw})
			return r.fail(state, err)
		}
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "assistant", Content: strings.TrimSpace(raw)})
		if strings.TrimSpace(out.Reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "assistant", Reply: strings.TrimSpace(out.Reply)})
		}
		if strings.TrimSpace(out.FailureReason) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_failure", Message: out.FailureReason})
			return r.fail(state, fmt.Errorf(out.FailureReason))
		}
		if out.NeedsClarification {
			question := firstNonEmpty(out.ClarificationQuestion, out.Reply, "Please tell me which object or target to edit.")
			state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: question})
			res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", question, "", "", nil)
			res.NeedsClarification = true
			res.ClarificationQuestion = question
			return res
		}
		if out.Final || len(out.ToolCalls) == 0 {
			reply := strings.TrimSpace(out.Reply)
			if reply == "" {
				reply = "Done."
			}
			return r.complete(state, reply)
		}
		for i := range out.ToolCalls {
			call := normalizeMessageLoopToolCall(out.ToolCalls[i], state.completedSteps+1)
			call = resolveMessageLoopBindings(state, call)
			if stopped, result := r.checkpoint("before_message_loop_tool", state); stopped {
				return result
			}
			if limit, result := r.checkToolBudget(state); limit {
				return result
			}
			if !allowedTool(call.Tool, state.input.AllowedTools) {
				result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: "unknown or disallowed tool: " + strings.TrimSpace(call.Tool)}
				state.trace = append(state.trace, planner.TraceEvent{Kind: "tool_call", ToolCall: &call}, planner.TraceEvent{Kind: "tool_result", ToolResult: &result})
				state.consecutiveErrors++
				appendMessageLoopToolResult(state)
				if state.consecutiveErrors >= state.budget.MaxConsecutiveErrors {
					return r.fail(state, fmt.Errorf(result.Error))
				}
				continue
			}
			if stopped, result := r.executeTool(ctx, state, call, false, nil); stopped {
				return result
			}
			appendMessageLoopToolResult(state)
		}
	}
}

func (l *MessageLoop) runner() *Runner {
	if l == nil {
		return &Runner{}
	}
	return &Runner{
		Runtime:  l.Runtime,
		Executor: l.Executor,
		Budget:   l.Budget,
		Now:      l.Now,
	}
}

func (l *MessageLoop) messages(state *runState, snapshotJSON string) []llm.Message {
	system := messageLoopSystemPrompt(state)
	user := fmt.Sprintf("Current Goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nCurrent context snapshot JSON:\n%s", strings.TrimSpace(state.input.UserText), state.goal.GoalID, state.goal.RunID, state.budget.MaxToolCalls-state.toolCallsUsed, snapshotJSON)
	out := []llm.Message{{Role: "system", Content: system}}
	out = append(out, state.input.Conversation...)
	out = append(out, llm.Message{Role: "user", Content: user})
	return out
}

func messageLoopSystemPrompt(state *runState) string {
	catalog := ""
	allowed := ""
	if state != nil {
		catalog = state.input.CatalogSummary
		allowed = strings.Join(state.input.AllowedTools, ", ")
	}
	return fmt.Sprintf(`You are Ask Vit's DAW ReAct runtime inside Vit-DAW.
Return ONLY strict JSON in one of these shapes:
{"final":true,"reply":"short final user-facing reply","tool_calls":[]}
{"final":false,"needs_clarification":true,"clarification_question":"ask exactly what target/choice is missing","reply":"same question","tool_calls":[]}
{"final":false,"reply":"short progress note","tool_calls":[{"tool":"track.add","args":{},"reason":"why"}]}

Rules:
- Use only tools from Allowed tools. For low-level DAW commands, use tool:"daw.invoke" only when it is explicitly allowed, with args containing cmd.
- Tool results appear in <tool_result> JSON messages. Treat those results as the source of truth for executed actions, refreshed DAW state, bindings, and verification.
- Do not invent track_id, clip_id, plugin_id, or tool names.
- Prefer read-only observation before risky writes, but do not over-observe when the current context already contains enough state.
- For dependent DAW edits, you may emit multiple tool calls in one response. Use symbolic refs such as {"track_ref":"last_created_track"} or {"track_id":"$last_created_track"} for later calls that target an object created by an earlier call.
- The local runtime resolves symbolic refs from prior tool results and execution bindings. Do not call the model again only to bind an ID that the tool result already produced.
- Target priority is: explicit user-named/indexed target, object created in this turn, explicit current/selected UI target, single unambiguous default, otherwise ask for clarification.
- Audio effects such as EQ, compressor, delay, reverb, analyzer, meter, or distortion belong in rack zone Z3. Instruments, synths, and samplers belong in Z2. If plugin metadata is unavailable, omit zone_id and let the tool resolve it.
- For ordinary plugin/effect loading, use plugin.load_to_rack or rack.add_node. Do not use plugin.instantiate unless the user explicitly asks for a track-level plugin outside the rack.
- Mark final:true only when requested outcomes are evidenced by tool results or context.

Available tool catalog:
%s

Allowed tools:
%s`, catalog, allowed)
}

func messageLoopInitialTranscript(in Input) []llm.Message {
	out := append([]llm.Message(nil), in.Conversation...)
	out = append(out, llm.Message{Role: "user", Content: strings.TrimSpace(in.UserText)})
	return out
}

func parseMessageLoopOutput(raw string) (messageLoopOutput, error) {
	var out messageLoopOutput
	text := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(text), &out); err == nil {
		return normalizeMessageLoopOutput(out), nil
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(text[start:end+1]), &out); err == nil {
			return normalizeMessageLoopOutput(out), nil
		}
	}
	return messageLoopOutput{}, fmt.Errorf("message loop returned invalid JSON")
}

func normalizeMessageLoopOutput(out messageLoopOutput) messageLoopOutput {
	out.Reply = strings.TrimSpace(out.Reply)
	out.ClarificationQuestion = strings.TrimSpace(out.ClarificationQuestion)
	out.FailureReason = strings.TrimSpace(out.FailureReason)
	for i := range out.ToolCalls {
		out.ToolCalls[i] = normalizeMessageLoopToolCall(out.ToolCalls[i], i+1)
	}
	if out.NeedsClarification {
		out.ToolCalls = nil
	}
	if out.Final {
		out.ToolCalls = nil
	}
	return out
}

func normalizeMessageLoopToolCall(call planner.ToolCall, step int) planner.ToolCall {
	call.Tool = strings.TrimSpace(call.Tool)
	call.PlanItemID = strings.TrimSpace(call.PlanItemID)
	if call.Args == nil {
		call.Args = map[string]any{}
	}
	if strings.TrimSpace(call.ID) == "" {
		call.ID = stableToolCallID(call, step)
	}
	return call
}

func resolveMessageLoopBindings(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil {
		return call
	}
	call.Args = cloneMap(call.Args)
	call.Command = cloneMap(call.Command)
	if call.Args == nil {
		call.Args = map[string]any{}
	}
	trackID := resolveTrackRef(state, firstMapText(call.Args, "track_ref", "target_track_ref", "track_id", "target_track_id", "selected_track_id"))
	if trackID == "" && messageLoopToolWantsTrack(call) {
		trackID = state.executionMemory.ActiveWorkTargetTrackID
	}
	if trackID != "" && messageLoopToolWantsTrack(call) {
		setIfEmpty(call.Args, "track_id", trackID)
		setIfEmpty(call.Command, "track_id", trackID)
	}
	return call
}

func resolveTrackRef(state *runState, ref string) string {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "$"))
	switch strings.ToLower(ref) {
	case "last_created_track", "new_track", "created_track", "active_work_target_track":
		return firstNonEmpty(state.executionMemory.ActiveWorkTargetTrackID, state.executionMemory.LastCreatedTrackID)
	default:
		return ref
	}
}

func messageLoopToolWantsTrack(call planner.ToolCall) bool {
	name := normalizedActionName(call, executorpkg.Result{})
	switch name {
	case "plugin.load_to_rack", "rack.add_node", "rack_add_node", "instantiate_plugin", "plugin.instantiate",
		"midi.create_clip", "midi.insert_clip", "midi.import_file", "clip.import_media_to_track", "clip.import_audio", "clip.add_audio",
		"create_midi_clip", "insert_midi_clip", "import_midi_to_track", "import_audio", "import_media_to_track", "add_audio_clip":
		return true
	default:
		return false
	}
}

func setIfEmpty(row map[string]any, key, value string) {
	if row == nil || strings.TrimSpace(value) == "" {
		return
	}
	if current := strings.TrimSpace(fmt.Sprint(row[key])); current == "" || current == "<nil>" || strings.HasPrefix(current, "$") {
		row[key] = value
	}
}

func appendMessageLoopToolResult(state *runState) {
	if state == nil {
		return
	}
	payload := map[string]any{}
	if len(state.executed) > 0 {
		payload["execution"] = state.executed[len(state.executed)-1]
	}
	if state.recentObservation != nil {
		payload["recent_observation"] = recentObservationMap(state.recentObservation)
	}
	if memory := executionMemoryMap(state.executionMemory); len(memory) > 0 {
		payload["execution_memory"] = memory
	}
	if len(payload) == 0 && len(state.trace) > 0 {
		payload["trace_tail"] = state.trace[len(state.trace)-1]
	}
	data, _ := json.Marshal(payload)
	state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<tool_result>" + string(data) + "</tool_result>"})
}
