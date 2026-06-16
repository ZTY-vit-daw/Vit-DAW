package agentloop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/promptruntime"
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
	Logger   *logx.Logger
}

func (l *MessageLoop) logTiming(stage string, started time.Time, format string, args ...any) {
	if l == nil || l.Logger == nil || started.IsZero() {
		return
	}
	detail := ""
	if strings.TrimSpace(format) != "" {
		detail = " " + fmt.Sprintf(format, args...)
	}
	l.Logger.Info("[timing] %s ms=%d%s", stage, time.Since(started).Milliseconds(), detail)
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
	if issue := messageLoopToolGuardIssue(&state, call, messageLoopHasUsableMixObservation(&state)); issue != "" {
		messageLoopAppendGuardGate(&state, call, issue)
		return l.loop(ctx, r, &state)
	}
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, &state, call, true, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
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
		assembly := l.assembly(state, snapshot.JSON())
		llmStarted := time.Now()
		raw, err := llm.CompleteText(ctx, l.Client, l.Config, llm.Request{
			Messages: assembly.Messages,
			Metadata: llm.RequestMetadata{
				Source:            "message_loop",
				ConversationID:    messageLoopConversationID(state),
				GoalID:            state.goal.GoalID,
				PromptFingerprint: assembly.Fingerprint,
				PromptStats:       assembly.Stats.Map(),
			},
		})
		l.logTiming("message_loop.llm", llmStarted, "goal=%s conversation=%s turn=%d err=%t", state.goal.GoalID, messageLoopConversationID(state), state.turnsUsed+1, err != nil)
		state.turnsUsed++
		if err != nil {
			return r.fail(state, err)
		}
		out, err := parseMessageLoopOutput(raw)
		if err != nil {
			appendMessageLoopDiagnostic(messageLoopDiagnostic{
				Stage:             "parse_failed",
				Error:             err.Error(),
				GoalID:            state.goal.GoalID,
				RunID:             state.goal.RunID,
				ConversationID:    messageLoopConversationID(state),
				PromptFingerprint: assembly.Fingerprint,
				Raw:               raw,
			})
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_error", Message: err.Error(), Reply: raw})
			repairedRaw, repairErr := l.repairOutput(ctx, state, raw, err, assembly.Fingerprint)
			state.turnsUsed++
			if repairErr != nil {
				appendMessageLoopDiagnostic(messageLoopDiagnostic{
					Stage:             "repair_failed",
					Error:             repairErr.Error(),
					GoalID:            state.goal.GoalID,
					RunID:             state.goal.RunID,
					ConversationID:    messageLoopConversationID(state),
					PromptFingerprint: assembly.Fingerprint,
					Raw:               repairedRaw,
				})
				state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_error", Message: repairErr.Error(), Reply: repairedRaw})
				return r.fail(state, fmt.Errorf("Agent 返回的计划格式不完整，自动修复也失败了"))
			}
			repairedOut, parseRepairErr := parseMessageLoopOutput(repairedRaw)
			if parseRepairErr != nil {
				appendMessageLoopDiagnostic(messageLoopDiagnostic{
					Stage:             "repair_parse_failed",
					Error:             parseRepairErr.Error(),
					GoalID:            state.goal.GoalID,
					RunID:             state.goal.RunID,
					ConversationID:    messageLoopConversationID(state),
					PromptFingerprint: assembly.Fingerprint,
					Raw:               repairedRaw,
				})
				state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_error", Message: parseRepairErr.Error(), Reply: repairedRaw})
				return r.fail(state, fmt.Errorf("Agent 返回的计划格式不完整，自动修复也没有得到可执行计划"))
			}
			raw = repairedRaw
			out = repairedOut
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_repair", Message: "message loop JSON repaired"})
		}
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "assistant", Content: strings.TrimSpace(raw)})
		if strings.TrimSpace(out.Reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "assistant", Reply: strings.TrimSpace(out.Reply)})
		}
		if strings.TrimSpace(out.FailureReason) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_failure", Message: out.FailureReason})
			return r.fail(state, errors.New(out.FailureReason))
		}
		if out.NeedsClarification {
			question := firstNonEmpty(out.ClarificationQuestion, out.Reply, "请告诉我这次要编辑的具体目标。")
			state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: question})
			res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", question, "", "", nil)
			res.NeedsClarification = true
			res.ClarificationQuestion = question
			return res
		}
		if out.Final || len(out.ToolCalls) == 0 {
			if issue := messageLoopFinalIssue(state); issue != "" {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue, PlanItems: append([]planner.PlanItem(nil), state.planItems...)})
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
				continue
			}
			reply := strings.TrimSpace(out.Reply)
			if reply == "" {
				reply = "已完成。"
			}
			return r.complete(state, reply)
		}
		hadMixObservationBeforeTurn := messageLoopHasUsableMixObservation(state)
		for i := range out.ToolCalls {
			call := normalizeMessageLoopToolCall(out.ToolCalls[i], state.completedSteps+1)
			call = resolveMessageLoopBindings(state, call)
			call = coercePluginGrabberLearningToolCall(state.input.UserText, call)
			call = coercePluginGrabberRuntimeToolCall(state.input.UserText, call)
			call = coerceWaveformBakeToMixObservation(state, call)
			if stopped, result := r.checkpoint("before_message_loop_tool", state); stopped {
				return result
			}
			if limit, result := r.checkToolBudget(state); limit {
				return result
			}
			if !allowedTool(call.Tool, state.input.AllowedTools) {
				result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
				state.trace = append(state.trace, planner.TraceEvent{Kind: "tool_call", ToolCall: &call}, planner.TraceEvent{Kind: "tool_result", ToolResult: &result})
				state.consecutiveErrors++
				appendMessageLoopToolResult(state)
				if state.consecutiveErrors >= state.budget.MaxConsecutiveErrors {
					return r.fail(state, errors.New(result.Error))
				}
				continue
			}
			if issue := messageLoopToolGuardIssue(state, call, hadMixObservationBeforeTurn); issue != "" {
				messageLoopAppendGuardGate(state, call, issue)
				continue
			}
			toolStarted := time.Now()
			stopped, result := r.executeTool(ctx, state, call, false, nil)
			l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
			if stopped {
				return result
			}
			appendMessageLoopToolResult(state)
		}
		if reply, ok := messageLoopFastCompleteReply(state, out); ok {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic tool result completed the turn"})
			return r.complete(state, reply)
		}
	}
}

func messageLoopAppendGuardGate(state *runState, call planner.ToolCall, issue string) {
	if state == nil {
		return
	}
	state.trace = append(state.trace,
		planner.TraceEvent{Kind: "tool_call_blocked", ToolCall: &call, Message: issue},
		planner.TraceEvent{Kind: "final_gate", Message: issue},
	)
	gate := map[string]any{
		"blocked_tool": strings.TrimSpace(call.Tool),
		"reason":       strings.TrimSpace(issue),
	}
	data, _ := json.Marshal(gate)
	state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<tool_gate>" + string(data) + "</tool_gate>"})
}

func coerceWaveformBakeToMixObservation(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || !messageLoopAudioObservationRequest(state.input.UserText) || !messageLoopIsWaveformBakeTool(call) {
		return call
	}
	next := call
	next.Tool = "mix.request_observation"
	if next.ID == "" || strings.Contains(strings.ToLower(next.ID), "bake") {
		next.ID = "observe_mix"
	}
	args := cloneMap(next.Args)
	if args == nil {
		args = map[string]any{}
	}
	delete(args, "cmd")
	delete(args, "command")
	if strings.TrimSpace(fmt.Sprint(args["mix_session_id"])) == "" {
		args["mix_session_id"] = "mix_" + strings.TrimSpace(state.goal.GoalID)
	}
	next.Args = args
	next.Command = nil
	next.Reason = firstNonEmpty(strings.TrimSpace(next.Reason), "route audio/acoustic observation through mix.request_observation")
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "clip.warm_waveform_bake was routed through mix.request_observation for audio observation",
		ToolCall: &next,
	})
	return next
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
	return l.assembly(state, snapshotJSON).Messages
}

func (l *MessageLoop) assembly(state *runState, snapshotJSON string) promptruntime.Assembly {
	system := messageLoopSystemPrompt(state)
	user := fmt.Sprintf("Current Goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nCurrent context snapshot JSON:\n%s", strings.TrimSpace(state.input.UserText), state.goal.GoalID, state.goal.RunID, state.budget.MaxToolCalls-state.toolCallsUsed, snapshotJSON)
	return promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "message_loop_system", "", system, true),
		},
		History: state.input.Conversation,
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "message_loop_runtime", "", user, false),
		},
	})
}

func messageLoopConversationID(state *runState) string {
	if state == nil || state.input.Context == nil {
		return ""
	}
	value, ok := state.input.Context["conversation_id"]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func messageLoopSystemPrompt(state *runState) string {
	catalog := ""
	allowed := ""
	modeRules := ""
	if state != nil {
		catalog = state.input.CatalogSummary
		allowed = strings.Join(state.input.AllowedTools, ", ")
		if messageLoopPlanMode(state.input.Context) {
			modeRules = `
Plan mode:
- This run is read-only. Do not execute DAW/project mutations, transport/live controls, plugin UI opening, shell commands, or workspace writes.
- If the user asks you to directly execute a change, return final:true with a concise refusal that says Plan mode is read-only and the change was not executed. Do not say the DAW lacks that capability or that the tool does not exist.
- You may provide an execution plan, prerequisites, risks, and the exact tools/commands that Default or Goal mode would use.`
		}
	}
	return fmt.Sprintf(`You are Ask Vit's DAW ReAct runtime inside Vit-DAW.
Return ONLY strict JSON in one of these shapes:
{"final":true,"reply":"short final user-facing reply","tool_calls":[]}
{"final":false,"needs_clarification":true,"clarification_question":"ask exactly what target/choice is missing","reply":"same question","tool_calls":[]}
{"final":false,"reply":"short progress note","tool_calls":[{"tool":"track.add","args":{},"reason":"why"}]}

Rules:
- Use only tools from Allowed tools. For low-level DAW commands, use tool:"daw.invoke" only when it is explicitly allowed, with args containing cmd.
- Tool results appear in <tool_result> JSON messages. Treat those results as the source of truth for executed actions, refreshed DAW state, bindings, and verification.
- For plugin_grabber_apply_control results, prefer applied_parameters[].new_value_text, applied_value, and confirmed display_domain data. Do not infer control limits from a parameter's current value_text or from advisory safety notes.
- For exact one-parameter plugin writes with explicit param_id and high-confidence get_plugin_parameters display_probe evidence, plugin.set_parameter may use value_text such as "1000 ms" or "28 percent". Do not use this for acoustic goals, multi-parameter moves, or automatic mixing; those require plugin_grabber.apply_control and a learned profile.
- Mixing is a native Ask Vit conversation task, not a separate Auto Mix/Co-Mix mode. Do not create or ask the user to fill a planning card for mixing.
- For natural/broad mixing goals such as making a vocal more forward, increasing loudness, reducing mud/harshness, tightening dynamics, adding space, or "mix this audio", you MUST call mix.request_observation first and wait for its result before choosing plugins, learning plugin profiles, loading effects, changing volume, or writing parameters.
- Do not call clip.warm_waveform_bake / warm_waveform_bake directly for broad mixing observation. mix.request_observation owns waveform and envelope feature preparation.
- After mix.request_observation for a broad mixing request, stop and summarize the observed project/audio facts plus one suggested next small move. Do not load plugins, learn profiles, change volume, apply controls, or write parameters in the same user request. Wait for the user to explicitly confirm a concrete follow-up action first.
- Keep each mixing action to one safe small step or one clearly coupled small move. Use track.volume for simple gain staging, or plugin_grabber.apply_control for learned plugin changes. If the needed plugin profile/skill is missing or stale, stop and use plugin_grabber.learn_project_profile or ask for learning confirmation; do not guess raw plugin parameters.
- Use plugin.set_parameter only when the user explicitly names an exact raw parameter/value or prior tool evidence gives a high-confidence exact param_id and display domain. Never use it as a fallback for subjective acoustic mixing goals.
- If the user says to undo or roll back the last mix move, use the available project undo/rollback path directly instead of returning to a mixing workflow.
- Do not invent track_id, clip_id, plugin_id, or tool names.
- Prefer read-only observation before risky writes, but do not over-observe when the current context already contains enough state.
- For dependent DAW edits, you may emit multiple tool calls in one response. Use symbolic refs such as {"track_ref":"last_created_track"} or {"track_id":"$last_created_track"} for later calls that target an object created by an earlier call.
- The local runtime resolves symbolic refs from prior tool results and execution bindings. Do not call the model again only to bind an ID that the tool result already produced.
- Target priority is: explicit user-named/indexed target, object created in this turn, explicit current/selected UI target, single unambiguous default, otherwise ask for clarification.
- Audio effects such as EQ, compressor, delay, reverb, analyzer, meter, or distortion belong in rack zone Z3. Instruments, synths, and samplers belong in Z2. If plugin metadata is unavailable, omit zone_id and let the tool resolve it.
- For ordinary plugin/effect loading, use plugin.load_to_rack or rack.add_node. Do not use plugin.instantiate unless the user explicitly asks for a track-level plugin outside the rack.
- For MIDI note writing, prefer midi.apply_note_patch with time_unit:"beats" and operations[]. Use insert_note/delete_note/move_note/resize_note/transpose_note/set_velocity/quantize_region/replace_region. Do not use legacy MIDI note tools for new note-writing plans unless midi.apply_note_patch is unavailable.
- If the current goal asks to create a MIDI clip and write notes, do not finish after only creating the clip. Continue until the MIDI notes are written and verified in refreshed state.
- If a MIDI note write is unverified, observe the current clip notes before retrying so you do not duplicate notes that were already written.
- When the user gives a local asset file or folder and asks what media is available, use media.register_assets or media.index_authorized_folder so the response can include clickable media pool artifact cards. Do not replace artifact cards with long raw path lists.
- Mark final:true only when requested outcomes are evidenced by tool results or context.
- User-facing reply text must be Chinese when the user writes Chinese. Do not expose JSON, tool names, tool IDs, or internal IDs unless the user explicitly asks for technical details.
%s

Available tool catalog:
%s

Allowed tools:
%s`, modeRules, catalog, allowed)
}

func messageLoopPlanMode(ctx map[string]any) bool {
	if ctx == nil {
		return false
	}
	for _, key := range []string{"agent_mode", "mode"} {
		mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(ctx[key])))
		switch mode {
		case "plan", "planner", "planning", "read_only_plan", "readonly_plan", "read-only-plan":
			return true
		}
	}
	return false
}

func messageLoopInitialTranscript(in Input) []llm.Message {
	out := append([]llm.Message(nil), in.Conversation...)
	out = append(out, llm.Message{Role: "user", Content: strings.TrimSpace(in.UserText)})
	return out
}

func (l *MessageLoop) repairOutput(ctx context.Context, state *runState, raw string, parseErr error, fingerprint string) (string, error) {
	if l == nil || l.Client == nil {
		return "", fmt.Errorf("agent message loop LLM client is nil")
	}
	rawJSON, _ := json.Marshal(strings.TrimSpace(raw))
	messages := []llm.Message{
		{
			Role: "system",
			Content: `You repair Ask Vit MessageLoop outputs.
Return ONLY one strict JSON object in one of these shapes:
{"final":true,"reply":"short final user-facing reply","tool_calls":[]}
{"final":false,"needs_clarification":true,"clarification_question":"ask exactly what target/choice is missing","reply":"same question","tool_calls":[]}
{"final":false,"reply":"short progress note","tool_calls":[{"tool":"track.add","args":{},"reason":"why"}]}
Do not add markdown fences, comments, prose, or multiple JSON objects. Preserve the original intent and tool arguments whenever possible.`,
		},
		{
			Role:    "user",
			Content: fmt.Sprintf("The previous output failed to parse: %s\nRaw output as a JSON string:\n%s", parseErr, string(rawJSON)),
		},
	}
	return llm.CompleteText(ctx, l.Client, l.Config, llm.Request{
		Messages: messages,
		Metadata: llm.RequestMetadata{
			Source:            "message_loop_repair",
			ConversationID:    messageLoopConversationID(state),
			GoalID:            state.goal.GoalID,
			PromptFingerprint: fingerprint,
		},
	})
}

func parseMessageLoopOutput(raw string) (messageLoopOutput, error) {
	for _, candidate := range messageLoopJSONCandidates(raw) {
		out, err := decodeMessageLoopCandidate(candidate)
		if err == nil {
			return normalizeMessageLoopOutput(out), nil
		}
	}
	return messageLoopOutput{}, fmt.Errorf("Agent 返回的计划格式不是有效 JSON")
}

func decodeMessageLoopCandidate(text string) (messageLoopOutput, error) {
	var out messageLoopOutput
	decoder := json.NewDecoder(bytes.NewReader([]byte(strings.TrimSpace(text))))
	if err := decoder.Decode(&out); err != nil {
		return messageLoopOutput{}, err
	}
	if !messageLoopOutputHasShape(out) {
		return messageLoopOutput{}, fmt.Errorf("message loop JSON does not match an executable shape")
	}
	return out, nil
}

func messageLoopOutputHasShape(out messageLoopOutput) bool {
	return out.Final || out.NeedsClarification || strings.TrimSpace(out.FailureReason) != "" || len(out.ToolCalls) > 0
}

func messageLoopJSONCandidates(raw string) []string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	seen := map[string]bool{}
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		candidates = append(candidates, value)
	}
	add(text)
	for _, fenced := range fencedJSONBlocks(text) {
		add(fenced)
	}
	for _, object := range balancedJSONObjectCandidates(text, 12) {
		add(object)
	}
	return candidates
}

func fencedJSONBlocks(text string) []string {
	var blocks []string
	remaining := text
	for {
		start := strings.Index(remaining, "```")
		if start < 0 {
			break
		}
		afterFence := remaining[start+3:]
		lineEnd := strings.IndexAny(afterFence, "\r\n")
		if lineEnd < 0 {
			break
		}
		label := strings.ToLower(strings.TrimSpace(afterFence[:lineEnd]))
		bodyStart := lineEnd + 1
		if strings.HasPrefix(afterFence[lineEnd:], "\r\n") {
			bodyStart = lineEnd + 2
		}
		bodyAndRest := afterFence[bodyStart:]
		end := strings.Index(bodyAndRest, "```")
		if end < 0 {
			break
		}
		body := strings.TrimSpace(bodyAndRest[:end])
		if label == "" || strings.Contains(label, "json") {
			blocks = append(blocks, body)
		}
		remaining = bodyAndRest[end+3:]
	}
	return blocks
}

func balancedJSONObjectCandidates(text string, limit int) []string {
	var out []string
	for start := 0; start < len(text); start++ {
		if text[start] != '{' {
			continue
		}
		depth := 0
		inString := false
		escaped := false
		for i := start; i < len(text); i++ {
			ch := text[i]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				switch ch {
				case '\\':
					escaped = true
				case '"':
					inString = false
				}
				continue
			}
			switch ch {
			case '"':
				inString = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					out = append(out, text[start:i+1])
					if limit > 0 && len(out) >= limit {
						return out
					}
					start = i
					break
				}
			}
		}
	}
	return out
}

type messageLoopDiagnostic struct {
	At                string `json:"at"`
	Stage             string `json:"stage"`
	Error             string `json:"error,omitempty"`
	GoalID            string `json:"goal_id,omitempty"`
	RunID             string `json:"run_id,omitempty"`
	ConversationID    string `json:"conversation_id,omitempty"`
	PromptFingerprint string `json:"prompt_fingerprint,omitempty"`
	Raw               string `json:"raw,omitempty"`
}

func appendMessageLoopDiagnostic(row messageLoopDiagnostic) {
	path := messageLoopDiagnosticPath()
	if path == "" {
		return
	}
	row.At = time.Now().Format(time.RFC3339Nano)
	row.Raw = truncateRunes(strings.TrimSpace(row.Raw), 5000)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(row)
}

func messageLoopDiagnosticPath() string {
	if override := strings.TrimSpace(os.Getenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH")); override != "" {
		if strings.EqualFold(override, "off") || strings.EqualFold(override, "disabled") {
			return ""
		}
		return override
	}
	if telemetry := strings.TrimSpace(llm.DefaultTelemetryPath()); telemetry != "" {
		return filepath.Join(filepath.Dir(telemetry), "agent_message_loop_debug.jsonl")
	}
	return filepath.Join(os.TempDir(), "vit_agent_message_loop_debug.jsonl")
}

func truncateRunes(text string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max]) + "...<truncated>"
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
	clipID := resolveClipRef(state, firstMapText(call.Args, "clip_ref", "target_clip_ref", "clip_id", "selected_clip_id", "primary_selected_clip_id"))
	if clipID == "" && messageLoopToolWantsClip(call) {
		clipID = firstNonEmpty(state.executionMemory.ActiveWorkTargetClipID, state.executionMemory.LastCreatedClipID)
	}
	if clipID != "" && messageLoopToolWantsClip(call) {
		setIfEmpty(call.Args, "clip_id", clipID)
		setIfEmpty(call.Command, "clip_id", clipID)
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

func resolveClipRef(state *runState, ref string) string {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "$"))
	switch strings.ToLower(ref) {
	case "last_created_clip", "new_clip", "created_clip", "active_work_target_clip":
		return firstNonEmpty(state.executionMemory.ActiveWorkTargetClipID, state.executionMemory.LastCreatedClipID)
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

func messageLoopToolWantsClip(call planner.ToolCall) bool {
	name := normalizedActionName(call, executorpkg.Result{})
	switch name {
	case "midi.apply_note_patch", "apply_midi_note_patch", "midi.add_notes", "midi.add_notes_bulk", "add_midi_notes", "add_midi_notes_bulk",
		"midi.legacy_add_notes", "midi.legacy_add_notes_bulk", "midi.read_notes", "midi.read_clip_notes", "midi.read_clip_data", "get_midi_clip_notes", "get_midi_clip_data",
		"mutate_midi_notes", "delete_midi_notes":
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

func messageLoopToolGuardIssue(state *runState, call planner.ToolCall, hadMixObservationBeforeTurn bool) string {
	if state == nil || !messageLoopNaturalMixRequest(state.input.UserText) || messageLoopExplicitPluginOrRawRequest(state.input.UserText) {
		return ""
	}
	if messageLoopIsWaveformBakeTool(call) {
		return "broad mixing observation must use mix.request_observation; waveform/envelope preparation is handled inside that observation tool"
	}
	if messageLoopIsMixObservationTool(call) || messageLoopMixObserveFirstAllowedTool(call) {
		return ""
	}
	if !messageLoopMixObserveFirstBlockedTool(call) {
		return ""
	}
	if hadMixObservationBeforeTurn {
		if messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
			return ""
		}
		return "mix.request_observation is complete; broad mixing requests must stop here, summarize the observation, propose one concrete next move, and wait for explicit user confirmation before loading plugins, learning profiles, changing volume, applying controls, or writing parameters"
	}
	return "ordinary acoustic mixing requests must run mix.request_observation and wait for its result before loading plugins, learning plugin profiles, changing volume, applying controls, or writing parameters"
}

func messageLoopHasUsableMixObservation(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		if messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			result := messageLoopMapValue(record["result"])
			if len(result) == 0 {
				result = record
			}
			return messageLoopMixObservationResultUsable(result)
		}
	}
	for _, event := range state.trace {
		if event.ToolResult != nil && !toolStatusFailed(event.ToolResult.Status) && messageLoopIsMixObservationName(event.ToolResult.Tool) {
			return messageLoopMixObservationResultUsable(event.ToolResult.Result)
		}
	}
	if observationIsMixObservation(state.recentObservation) {
		return true
	}
	return false
}

func observationIsMixObservation(observation *RecentObservation) bool {
	if observation == nil || toolStatusFailed(observation.Status) {
		return false
	}
	return messageLoopIsMixObservationName(firstNonEmpty(observation.Tool, observation.CommandName))
}

func messageLoopMixObservationResultUsable(result map[string]any) bool {
	if len(result) == 0 {
		return true
	}
	for _, row := range []map[string]any{
		result,
		messageLoopMapValue(result["mixboard"]),
		messageLoopMapValue(result["observation"]),
	} {
		if len(row) == 0 {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(messageLoopText(row["status"])))
		switch status {
		case "unavailable", "blocked", "missing", "invalid", "failed", "error":
			return false
		}
		if blockers := messageLoopStringList(row["open_blockers"]); len(blockers) > 0 {
			return false
		}
		if blockers := messageLoopStringList(row["blockers"]); len(blockers) > 0 {
			return false
		}
		packageStatus := messageLoopMapValue(row["package_status"])
		if mixStatus := strings.ToLower(strings.TrimSpace(messageLoopText(packageStatus["mix"]))); mixStatus != "" && mixStatus != "ready" && mixStatus != "baseline_ready" && mixStatus != "limited" {
			return false
		}
	}
	if featureRequest := messageLoopMapValue(result["feature_request"]); len(featureRequest) > 0 {
		switch strings.ToLower(strings.TrimSpace(messageLoopText(featureRequest["status"]))) {
		case "blocked", "unavailable", "failed", "error":
			return false
		}
	}
	return true
}

func messageLoopMapValue(v any) map[string]any {
	if row, ok := v.(map[string]any); ok {
		return row
	}
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
}

func messageLoopAnyMap(v any) map[string]any {
	return messageLoopMapValue(v)
}

func messageLoopAnySlice(v any) []any {
	switch xs := v.(type) {
	case []any:
		return xs
	default:
		if v == nil {
			return nil
		}
		data, err := json.Marshal(v)
		if err != nil || len(data) == 0 || bytes.Equal(data, []byte("null")) {
			return nil
		}
		var out []any
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}

func messageLoopStringList(v any) []string {
	switch xs := v.(type) {
	case []string:
		out := make([]string, 0, len(xs))
		for _, item := range xs {
			if strings.TrimSpace(item) != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(xs))
		for _, item := range xs {
			text := strings.TrimSpace(messageLoopText(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func messageLoopNaturalMixRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"\u6df7\u97f3", "\u7f29\u6df7", "\u58f0\u97f3\u5904\u7406", "\u8c03\u4e00\u4e0b", "\u5904\u7406\u4e00\u4e0b",
		"\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal",
		"\u9760\u524d", "\u5f80\u524d", "\u63d0\u5347\u54cd\u5ea6", "\u54cd\u5ea6", "\u66f4\u4eae", "\u660e\u4eae", "\u6d51\u6d4a", "\u523a\u8033",
		"\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u7a7a\u95f4\u611f", "\u52a0\u4e00\u70b9\u7a7a\u95f4", "\u52a8\u6001", "\u538b\u7f29",
		"mix", "mixing", "loudness", "louder", "forward", "mud", "muddy", "harsh", "bright", "space", "reverb", "dynamic",
	)
}

func messageLoopAudioObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if messageLoopNaturalMixRequest(text) {
		return true
	}
	return messageLoopTextHasAny(text,
		"\u97f3\u9891\u89c2\u5bdf", "\u6df7\u97f3\u89c2\u5bdf", "\u58f0\u5b66\u89c2\u5bdf", "\u58f0\u5b66\u5206\u6790", "\u58f0\u5b66\u6570\u636e",
		"\u89c2\u5bdf\u5206\u6790", "\u5177\u4f53\u5206\u6790", "\u7ee7\u7eed\u89c2\u5bdf", "\u5206\u6790\u4e00\u4e0b",
		"\u9891\u8c31", "\u9891\u8c31\u5206\u5e03", "\u9891\u6bb5\u80fd\u91cf", "\u4f4e\u9891", "\u4e2d\u9891", "\u9ad8\u9891",
		"\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u52a8\u6001\u8303\u56f4", "\u6ce2\u5f62", "\u5305\u7edc",
		"audio observation", "mix observation", "acoustic observation", "acoustic analysis", "acoustic data",
		"observation analysis", "analyze audio", "specific analysis", "spectrum", "spectral", "frequency distribution",
		"band energy", "loudness", "peak", "rms", "lufs", "dynamic range", "waveform", "envelope",
	)
}

func messageLoopExplicitPluginOrRawRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasExplicitVerb := messageLoopTextHasAny(text,
		"\u52a0\u8f7d", "\u6302\u8f7d", "\u6253\u5f00", "\u5b66\u4e60", "\u6293\u624b", "\u63d2\u5165", "\u65b0\u589e",
		"\u8bbe\u7f6e\u53c2\u6570", "\u5199\u53c2\u6570", "\u6539\u53c2\u6570", "\u8c03\u53c2\u6570",
		"load", "insert", "open", "learn", "grabber", "set parameter", "write parameter",
	)
	hasPluginObject := messageLoopTextHasAny(text,
		"\u63d2\u4ef6", "\u6548\u679c\u5668", "\u5747\u8861\u5668", "\u538b\u7f29\u5668", "\u6df7\u54cd", "\u5ef6\u8fdf",
		"plugin", "vst", "eq", "compressor", "reverb", "delay", "tdr", "nova", "zl",
	)
	hasRawParam := messageLoopTextHasAny(text, "param_id", "parameter id", "\u53c2\u6570 id", "\u5f52\u4e00\u5316", "normalized")
	return hasRawParam || (hasExplicitVerb && hasPluginObject)
}

func messageLoopExplicitMixExecutionConfirmation(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasApproval := messageLoopTextHasAny(text,
		"\u786e\u8ba4", "\u6267\u884c", "\u5f00\u59cb", "\u6309\u4f60\u8bf4\u7684", "\u6309\u8fd9\u4e2a", "\u5c31\u8fd9\u6837", "\u53ef\u4ee5",
		"confirm", "execute", "apply", "do it", "go ahead", "as you said",
	)
	hasConcreteMove := messageLoopTextHasAny(text,
		"db", "hz", "khz", "%", "percent", "\u5206\u8d1d", "\u8d6b\u5179",
		"\u63d0\u5347", "\u964d\u4f4e", "\u589e\u52a0", "\u51cf\u5c11", "\u538b", "\u524a", "\u5207", "\u52a0", "\u51cf", "\u4e0d\u8981\u52a8\u97f3\u91cf",
		"boost", "cut", "raise", "lower", "reduce", "compress", "volume", "gain", "reverb", "eq",
	)
	return hasApproval && hasConcreteMove
}

func messageLoopIsMixObservationTool(call planner.ToolCall) bool {
	return messageLoopIsMixObservationName(normalizedActionName(call, executorpkg.Result{}))
}

func messageLoopIsMixObservationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mix.request_observation", "mix_request_observation":
		return true
	default:
		return false
	}
}

func messageLoopIsWaveformBakeTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "clip.warm_waveform_bake", "warm_waveform_bake":
		return true
	default:
		return false
	}
}

func messageLoopMixObserveFirstAllowedTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	if strings.HasPrefix(name, "get_") || strings.HasPrefix(name, "list_") || strings.Contains(name, ".list") || strings.Contains(name, ".read") || strings.Contains(name, "project.state") {
		return true
	}
	switch name {
	case "project.state", "get_project_state", "track.list":
		return true
	default:
		return false
	}
}

func messageLoopMixObserveFirstBlockedTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "plugin.load_to_rack", "rack.add_node", "rack_add_node", "instantiate_plugin", "plugin.instantiate",
		"plugin.search", "plugin.semantic_search", "plugin.list", "plugin.list_available", "plugin.get_parameters", "get_plugin_parameters",
		"plugin_grabber.get_project_profiles", "plugin_grabber_get_project_profiles", "plugin_grabber.explain_controls", "plugin_grabber_explain_controls",
		"plugin_grabber.learn_project_profile", "plugin_grabber_learn_project_profile",
		"plugin_grabber.apply_control", "plugin_grabber_apply_control", "plugin_grabber.apply",
		"plugin.set_parameter", "plugin_set_parameter", "set_plugin_param",
		"track.volume", "set_volume",
		"rack.add_macro", "control_add_macro", "control.add_macro", "control.add_binding", "control_add_binding":
		return true
	default:
		return false
	}
}

func appendMessageLoopToolResult(state *runState) {
	if state == nil {
		return
	}
	payload := map[string]any{}
	if len(state.executed) > 0 {
		payload["execution"] = compactMessageLoopExecution(state.executed[len(state.executed)-1])
	}
	if state.recentObservation != nil {
		payload["recent_observation"] = contextruntime.CompactValue(recentObservationMap(state.recentObservation), messageLoopCompactOptions())
	}
	if memory := executionMemoryMap(state.executionMemory); len(memory) > 0 {
		payload["execution_memory"] = contextruntime.CompactValue(memory, messageLoopCompactOptions())
	}
	if len(payload) == 0 && len(state.trace) > 0 {
		payload["trace_tail"] = contextruntime.CompactValue(state.trace[len(state.trace)-1], messageLoopCompactOptions())
	}
	data, _ := json.Marshal(payload)
	state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<tool_result>" + string(data) + "</tool_result>"})
}

func compactMessageLoopExecution(record map[string]any) map[string]any {
	opts := messageLoopCompactOptions()
	out := map[string]any{}
	for _, key := range []string{
		"status", "tool_call_id", "agent_action_id", "tool", "command_name",
		"preview", "undo_label", "error", "observed_state",
	} {
		if value, ok := record[key]; ok && !messageLoopEmptyValue(value) {
			out[key] = contextruntime.CompactValue(value, opts)
		}
	}
	if result, ok := record["result"].(map[string]any); ok && len(result) > 0 {
		if summary := mixObservationPromptSummary(result); len(summary) > 0 {
			out["mix_observation_summary"] = summary
		}
		out["result_summary"] = contextruntime.SummarizeToolResult(planner.ToolResult{
			ToolCallID: messageLoopText(record["tool_call_id"]),
			Tool:       messageLoopText(record["tool"]),
			Status:     messageLoopText(record["status"]),
			Error:      messageLoopText(record["error"]),
			Result:     result,
		}, opts)
	}
	if verification, ok := record["verification"]; ok && !messageLoopEmptyValue(verification) {
		out["verification"] = contextruntime.CompactValue(verification, opts)
	}
	if projectHistory, ok := record["project_history"]; ok && !messageLoopEmptyValue(projectHistory) {
		out["project_history"] = contextruntime.CompactValue(projectHistory, opts)
	}
	return out
}

func mixObservationPromptSummary(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(result["tool"]), messageLoopText(result["command_name"]))) {
		// Tool execution records keep tool/command_name outside result, so also
		// accept the characteristic MixBoard result shape.
		if result["acoustic_digest"] == nil && result["observation"] == nil && result["context_pack"] == nil {
			return nil
		}
	}
	out := map[string]any{}
	for _, key := range []string{"status", "mix_session_id", "observation_id", "observation_path", "context_pack_path"} {
		if value, ok := result[key]; ok && !messageLoopEmptyValue(value) {
			out[key] = value
		}
	}
	if digest := messageLoopMapValue(result["acoustic_digest"]); len(digest) > 0 {
		out["acoustic_digest"] = compactMixObservationDigest(digest)
	}
	observation := messageLoopMapValue(result["observation"])
	if len(observation) == 0 {
		if pack := messageLoopMapValue(result["context_pack"]); len(pack) > 0 {
			observation = messageLoopMapValue(pack["latest_observation"])
		}
	}
	if len(observation) > 0 {
		if target := messageLoopMapValue(observation["target_ref"]); len(target) > 0 {
			out["target_ref"] = compactSelectedKeys(target, []string{"kind", "id", "label", "confidence"})
		}
		if ruler := messageLoopMapValue(observation["time_ruler"]); len(ruler) > 0 {
			out["time_ruler"] = compactSelectedKeys(ruler, []string{"duration_seconds", "segment_seconds", "frame_seconds"})
		}
		if global := messageLoopMapValue(observation["global_summary"]); len(global) > 0 {
			out["global_summary"] = compactSelectedKeys(global, []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "dominant_problem_tags"})
		}
		if mixPkg := messageLoopMapValue(observation["mix_package"]); len(mixPkg) > 0 {
			out["mix_package"] = compactMixPackageForPrompt(mixPkg)
		}
		if envPkg := messageLoopMapValue(observation["environment_package"]); len(envPkg) > 0 {
			if caps := messageLoopMapValue(envPkg["source_capabilities"]); len(caps) > 0 {
				out["environment_source_capabilities"] = caps
			}
		}
		if deepPkg := messageLoopMapValue(observation["deep_package"]); len(deepPkg) > 0 {
			out["deep_package"] = compactSelectedKeys(deepPkg, []string{"status", "source_capabilities"})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactMixObservationDigest(digest map[string]any) map[string]any {
	out := compactSelectedKeys(digest, []string{
		"status", "target", "clip_id", "clip_name", "file_path", "duration_seconds",
		"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db",
		"band_energy_status", "stereo_relation_status", "balance_db", "correlation_estimate",
		"feature_request_status", "feature_request_reason", "missing_metrics", "notes",
	})
	if waveform := messageLoopMapValue(digest["waveform"]); len(waveform) > 0 {
		out["waveform"] = compactSelectedKeys(waveform, []string{"status", "peak_abs", "rms", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"})
	}
	if rows := messageLoopMapRows(digest["time_energy"]); len(rows) > 0 {
		out["time_energy"] = capMessageLoopRows(rows, 8)
	}
	if band := messageLoopMapValue(digest["band_energy"]); len(band) > 0 {
		out["band_energy"] = band
	}
	return out
}

func compactMixPackageForPrompt(mixPkg map[string]any) map[string]any {
	out := compactSelectedKeys(mixPkg, []string{"status", "role", "round", "missing_metrics", "source_capabilities"})
	metrics := messageLoopMapValue(mixPkg["current_metrics"])
	if len(metrics) == 0 {
		return out
	}
	current := map[string]any{}
	if waveform := messageLoopMapValue(metrics["waveform"]); len(waveform) > 0 {
		current["waveform"] = compactSelectedKeys(waveform, []string{"status", "peak_abs", "rms", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "sample_count", "file_path", "updated_at"})
	}
	if rows := messageLoopMapRows(metrics["time_energy"]); len(rows) > 0 {
		current["time_energy"] = capMessageLoopRows(rows, 8)
	}
	if band := messageLoopMapValue(metrics["band_energy"]); len(band) > 0 {
		current["band_energy"] = compactSelectedKeys(band, []string{"status", "source", "bands", "updated_at"})
	}
	if stereo := messageLoopMapValue(metrics["stereo_relation"]); len(stereo) > 0 {
		current["stereo_relation"] = compactSelectedKeys(stereo, []string{"status", "balance_db", "balance_state", "correlation_estimate", "correlation_state", "left_level_db", "right_level_db", "updated_at"})
	}
	if len(current) > 0 {
		out["current_metrics"] = current
	}
	return out
}

func compactSelectedKeys(row map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && !messageLoopEmptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func messageLoopMapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		items := messageLoopAnySlice(value)
		if len(items) == 0 {
			return nil
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if row := messageLoopAnyMap(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	}
}

func capMessageLoopRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return rows[:max]
}

func messageLoopCompactOptions() contextruntime.Options {
	return contextruntime.Options{
		MaxTextRunes:           900,
		MaxListItems:           20,
		MaxPreviewBytes:        12 * 1024,
		SkipPluginSemanticLoad: true,
	}
}

func messageLoopText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func messageLoopEmptyValue(value any) bool {
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func messageLoopFastCompleteReply(state *runState, out messageLoopOutput) (string, bool) {
	if state == nil || len(out.ToolCalls) != 1 || len(state.executed) == 0 {
		return "", false
	}
	if len(openPlanItems(state.planItems, nil)) > 0 {
		return "", false
	}
	if state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return "", false
	}
	if issue := messageLoopFinalIssue(state); issue != "" {
		return "", false
	}
	call := out.ToolCalls[0]
	record := state.executed[len(state.executed)-1]
	if !messageLoopFastCompleteRecordMatches(call, record) || !messageLoopExecutionSucceeded(record) {
		return "", false
	}
	ver, ok := messageLoopExecutionVerification(record)
	if !ok {
		return "", false
	}
	switch ver.Status {
	case verificationVerified, verificationNotRequired:
	default:
		return "", false
	}
	if messageLoopUserRequestedMediaArtifacts(state.input.UserText) && messageLoopIsMediaArtifactToolName(messageLoopText(record["tool"])) {
		if reply := messageLoopMediaArtifactsFastCompleteReply(record); strings.TrimSpace(reply) != "" {
			return reply, true
		}
	}
	required := messageLoopRequiredOutcomes(state, out, ver)
	if len(required) == 0 {
		return "", false
	}
	satisfied := messageLoopSatisfiedOutcomes(state)
	if !messageLoopOutcomesSatisfied(required, satisfied) {
		return "", false
	}
	reply := messageLoopOutcomeFastCompleteReply(state, record, ver, required)
	if strings.TrimSpace(reply) == "" {
		return "", false
	}
	return reply, true
}

func messageLoopFastCompleteRecordMatches(call planner.ToolCall, record map[string]any) bool {
	if len(record) == 0 {
		return false
	}
	callID := strings.TrimSpace(call.ID)
	recordID := messageLoopText(record["tool_call_id"])
	if callID != "" && recordID != "" && callID != recordID {
		return false
	}
	return messageLoopExecutionActionName(call, record) != ""
}

func messageLoopExecutionSucceeded(record map[string]any) bool {
	if len(record) == 0 {
		return false
	}
	if messageLoopText(record["error"]) != "" {
		return false
	}
	status := strings.ToLower(messageLoopText(record["status"]))
	if status == "" || toolStatusFailed(status) {
		return false
	}
	switch status {
	case "ok", "success", "succeeded", "completed":
		return true
	default:
		return false
	}
}

func messageLoopExecutionVerification(record map[string]any) (planner.VerificationResult, bool) {
	if len(record) == 0 {
		return planner.VerificationResult{}, false
	}
	switch ver := record["verification"].(type) {
	case planner.VerificationResult:
		ver.Status = strings.TrimSpace(ver.Status)
		return ver, ver.Status != ""
	case *planner.VerificationResult:
		if ver == nil {
			return planner.VerificationResult{}, false
		}
		out := *ver
		out.Status = strings.TrimSpace(out.Status)
		return out, out.Status != ""
	case map[string]any:
		out := planner.VerificationResult{
			ToolCallID:    firstMapText(ver, "tool_call_id"),
			PlanItemID:    firstMapText(ver, "plan_item_id"),
			Tool:          firstMapText(ver, "tool"),
			CommandName:   firstMapText(ver, "command_name"),
			Status:        firstMapText(ver, "status"),
			Postcondition: firstMapText(ver, "postcondition"),
			Message:       firstMapText(ver, "message"),
		}
		if evidence, ok := ver["evidence"].(map[string]any); ok {
			out.Evidence = evidence
		}
		return out, out.Status != ""
	default:
		return planner.VerificationResult{}, false
	}
}

func messageLoopExecutionActionName(call planner.ToolCall, record map[string]any) string {
	result := executorpkg.Result{
		Tool:        messageLoopText(record["tool"]),
		CommandName: messageLoopText(record["command_name"]),
	}
	return normalizedActionName(call, result)
}

func messageLoopRequiredOutcomes(state *runState, out messageLoopOutput, ver planner.VerificationResult) map[string]bool {
	required := map[string]bool{}
	if state == nil {
		return required
	}
	if messageLoopUserRequestedMIDINotes(state.input.UserText) {
		required[postconditionMIDINotesPresent] = true
		return required
	}
	postcondition := strings.TrimSpace(ver.Postcondition)
	if !messageLoopFastCompletablePostcondition(postcondition) {
		return required
	}
	if messageLoopOutcomeBlockedByUserText(state.input.UserText, postcondition) {
		return required
	}
	required[postcondition] = true
	return required
}

func messageLoopFastCompletablePostcondition(postcondition string) bool {
	switch strings.TrimSpace(postcondition) {
	case postconditionTrackPresent, postconditionClipPresent, postconditionMIDINotesPresent, postconditionRackNodePresent:
		return true
	default:
		return false
	}
}

func messageLoopSatisfiedOutcomes(state *runState) map[string]planner.VerificationResult {
	out := map[string]planner.VerificationResult{}
	if state == nil {
		return out
	}
	add := func(ver planner.VerificationResult) {
		postcondition := strings.TrimSpace(ver.Postcondition)
		if !messageLoopFastCompletablePostcondition(postcondition) || strings.TrimSpace(ver.Status) != verificationVerified {
			return
		}
		out[postcondition] = ver
	}
	for _, record := range state.executed {
		if ver, ok := messageLoopExecutionVerification(record); ok {
			add(ver)
		}
	}
	for _, event := range state.trace {
		if event.Verification != nil {
			add(*event.Verification)
		}
	}
	if messageLoopHasVerifiedMIDINoteWrite(state) || messageLoopStateShowsExpectedMIDINotes(state) || messageLoopReadbackShowsExpectedMIDINotes(state) {
		if _, ok := out[postconditionMIDINotesPresent]; !ok {
			out[postconditionMIDINotesPresent] = planner.VerificationResult{
				Status:        verificationVerified,
				Postcondition: postconditionMIDINotesPresent,
				Message:       "MIDI notes are present in refreshed or readback state",
			}
		}
	}
	return out
}

func messageLoopOutcomesSatisfied(required map[string]bool, satisfied map[string]planner.VerificationResult) bool {
	if len(required) == 0 {
		return false
	}
	for outcome := range required {
		if _, ok := satisfied[outcome]; !ok {
			return false
		}
	}
	return true
}

func messageLoopOutcomeBlockedByUserText(userText, postcondition string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	for _, token := range []string{"\u7136\u540e", "\u63a5\u7740", "\u968f\u540e", "\u987a\u4fbf", "\u540c\u65f6", "then", "after that", "next"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	switch strings.TrimSpace(postcondition) {
	case postconditionTrackPresent:
		return messageLoopTextHasAny(text,
			"\u63d2\u4ef6", "\u52a0\u8f7d", "\u6302\u8f7d", "\u6548\u679c\u5668", "\u5747\u8861", "\u538b\u7f29", "\u6df7\u54cd", "\u5ef6\u8fdf",
			"plugin", "vst", "effect", "eq", "compressor", "reverb", "delay",
			"clip", "\u7247\u6bb5", "\u97f3\u7b26", "\u65cb\u5f8b", "\u548c\u5f26", "note", "notes", "melody", "chord", "drum",
		)
	case postconditionClipPresent:
		return messageLoopTextHasAny(text,
			"\u97f3\u7b26", "\u65cb\u5f8b", "\u548c\u5f26", "\u9f13", "\u5199", "\u5199\u5165", "\u8f93\u5165", "\u63d2\u5165", "\u6dfb\u52a0", "\u751f\u6210",
			"note", "notes", "melody", "chord", "drum", "write", "insert", "add", "pattern",
		)
	case postconditionRackNodePresent:
		return messageLoopTextHasAny(text,
			"\u5b66\u4e60", "\u6293\u624b", "\u8c03\u6574", "\u63a7\u5236", "\u53c2\u6570", "\u6df7\u97f3",
			"learn", "grabber", "adjust", "control", "parameter", "mix",
		)
	default:
		return false
	}
}

func messageLoopTextHasAny(text string, tokens ...string) bool {
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" && strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func messageLoopOutcomeFastCompleteReply(state *runState, record map[string]any, ver planner.VerificationResult, required map[string]bool) string {
	switch {
	case required[postconditionMIDINotesPresent]:
		return messageLoopMIDINotesFastCompleteReply(state)
	case required[postconditionTrackPresent]:
		return messageLoopTrackPresentFastCompleteReply(state, record, ver)
	case required[postconditionClipPresent]:
		return messageLoopClipPresentFastCompleteReply(state, record, ver)
	case required[postconditionRackNodePresent]:
		return messageLoopRackNodeFastCompleteReply(record, ver)
	default:
		return ""
	}
}

func messageLoopTrackPresentFastCompleteReply(state *runState, record map[string]any, ver planner.VerificationResult) string {
	result, _ := record["result"].(map[string]any)
	trackName := firstNonEmpty(
		firstMapText(result, "track_name", "name"),
		firstMapText(ver.Evidence, "observed_track_name", "expected_track_name"),
		state.executionMemory.LastCreatedTrackName,
	)
	if trackName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u521b\u5efa\u65b0\u8f68\u9053\u300c%s\u300d\u3002", trackName)
	}
	return "\u5df2\u6210\u529f\u521b\u5efa\u65b0\u8f68\u9053\u3002"
}

func messageLoopClipPresentFastCompleteReply(state *runState, record map[string]any, ver planner.VerificationResult) string {
	result, _ := record["result"].(map[string]any)
	clipName := firstNonEmpty(
		firstMapText(result, "clip_name", "name"),
		firstMapText(ver.Evidence, "observed_clip_name", "expected_clip_name"),
		state.executionMemory.LastCreatedClipName,
	)
	if clipName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u521b\u5efa MIDI \u7247\u6bb5\u300c%s\u300d\u3002", clipName)
	}
	return "\u5df2\u6210\u529f\u521b\u5efa MIDI \u7247\u6bb5\u3002"
}

func messageLoopMIDINotesFastCompleteReply(state *runState) string {
	_, _, _, count := messageLoopExpectedMIDINoteWriteEvidence(state)
	createdClip := messageLoopHasClipCreationAttempt(state)
	if count > 0 {
		if createdClip {
			if !messageLoopReadbackShowsExpectedMIDINotes(state) {
				return "\u5df2\u521b\u5efa MIDI \u7247\u6bb5\u5e76\u5199\u5165\u97f3\u7b26\u3002"
			}
			return fmt.Sprintf("\u5df2\u521b\u5efa MIDI \u7247\u6bb5\u5e76\u5199\u5165 %d \u4e2a\u97f3\u7b26\u3002", count)
		}
		return fmt.Sprintf("\u5df2\u5199\u5165 %d \u4e2a MIDI \u97f3\u7b26\u3002", count)
	}
	if createdClip {
		return "\u5df2\u521b\u5efa MIDI \u7247\u6bb5\u5e76\u5199\u5165\u97f3\u7b26\u3002"
	}
	return "\u5df2\u5199\u5165 MIDI \u97f3\u7b26\u3002"
}

func messageLoopRackNodeFastCompleteReply(record map[string]any, ver planner.VerificationResult) string {
	result, _ := record["result"].(map[string]any)
	pluginName := firstNonEmpty(
		firstMapText(result, "plugin_name", "name", "descriptive_name"),
		firstMapText(ver.Evidence, "observed_plugin_name", "expected_plugin_name"),
	)
	if pluginName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u52a0\u8f7d\u63d2\u4ef6\u300c%s\u300d\u3002", pluginName)
	}
	return "\u5df2\u6210\u529f\u52a0\u8f7d\u63d2\u4ef6\u3002"
}

func messageLoopHasClipCreationAttempt(state *runState) bool {
	if state == nil {
		return false
	}
	for _, event := range state.trace {
		if event.Verification != nil && event.Verification.Postcondition == postconditionClipPresent && event.Verification.Status == verificationVerified {
			return true
		}
		if event.ToolResult != nil && messageLoopIsClipCreateName(event.ToolResult.Tool) && !toolStatusFailed(event.ToolResult.Status) {
			return true
		}
		if event.ToolCall != nil && messageLoopIsClipCreateName(normalizedActionName(*event.ToolCall, executorpkg.Result{})) {
			return true
		}
	}
	return false
}

func messageLoopIsClipCreateName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "create_midi_clip", "insert_midi_clip", "midi.create_clip", "midi.insert_clip":
		return true
	default:
		return false
	}
}

func messageLoopFastCompleteAllowsTrackAdd(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	for _, token := range []string{
		"然后", "接着", "随后", "顺便", "同时",
		"插件", "加载", "挂载", "效果器", "均衡", "压缩", "混响", "延迟",
		"plugin", "vst", "effect", "eq", "compressor", "reverb", "delay",
		"clip", "片段", "音符", "旋律", "和弦", "note", "notes", "melody", "chord", "drum",
	} {
		if strings.Contains(text, token) {
			return false
		}
	}
	return true
}

func messageLoopTrackAddFastCompleteReply(state *runState, record map[string]any, ver planner.VerificationResult) string {
	result, _ := record["result"].(map[string]any)
	trackName := firstNonEmpty(
		firstMapText(result, "track_name", "name"),
		firstMapText(ver.Evidence, "observed_track_name", "expected_track_name"),
		state.executionMemory.LastCreatedTrackName,
	)
	if trackName != "" {
		return fmt.Sprintf("已成功创建新轨道「%s」。", trackName)
	}
	return "已成功创建新轨道。"
}

func messageLoopFinalIssue(state *runState) string {
	if issue := messageLoopMediaFinalIssue(state); issue != "" {
		return issue
	}
	if state == nil || !messageLoopUserRequestedMIDINotes(state.input.UserText) {
		return ""
	}
	if messageLoopHasVerifiedMIDINoteWrite(state) || messageLoopStateShowsExpectedMIDINotes(state) || messageLoopReadbackShowsExpectedMIDINotes(state) {
		return ""
	}
	if messageLoopHasMIDINoteWriteAttempt(state) {
		return "cannot finish yet; the user asked to write MIDI notes, but the MIDI note write has not been verified in refreshed project state"
	}
	return "cannot finish yet; the user asked to write MIDI notes, but no MIDI note write tool has succeeded"
}

func messageLoopMediaFinalIssue(state *runState) string {
	if state == nil || !messageLoopUserRequestedMediaArtifacts(state.input.UserText) {
		return ""
	}
	if messageLoopHasMediaArtifactToolAttempt(state) {
		return ""
	}
	return "cannot finish yet; the user asked to inspect a local media asset location, but no media artifact tool has run"
}

func messageLoopUserRequestedMediaArtifacts(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasLocalPath := strings.Contains(text, ":\\") || strings.Contains(text, ":/") || strings.Contains(text, "\\\\") ||
		strings.Contains(text, "\u6587\u4ef6\u5939") || strings.Contains(text, "\u8def\u5f84") ||
		strings.Contains(text, "folder") || strings.Contains(text, "directory") || strings.Contains(text, "path")
	if !hasLocalPath {
		return false
	}
	hasMedia := messageLoopTextHasAny(text,
		"\u7d20\u6750", "\u5a92\u4f53", "\u5a92\u4f53\u6c60", "\u97f3\u9891", "\u89c6\u9891", "\u97f3\u89c6\u9891", "\u56fe\u7247",
		"media", "asset", "audio", "video", "image", ".wav", ".mp3", ".flac", ".mp4", ".mov", ".mkv", ".mid", ".midi",
	)
	hasInspection := messageLoopTextHasAny(text,
		"\u67e5\u770b", "\u770b\u770b", "\u68c0\u7d22", "\u641c\u7d22", "\u627e\u5230", "\u627e\u627e", "\u5217\u51fa", "\u6709\u54ea\u4e9b", "\u4e0b\u6709", "\u4e0b\u627e",
		"find", "search", "list", "show", "what", "which", "available",
	)
	return hasMedia && hasInspection
}

func messageLoopHasMediaArtifactToolAttempt(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		if messageLoopIsMediaArtifactToolName(messageLoopText(record["tool"])) && messageLoopExecutionSucceeded(record) {
			return true
		}
	}
	for _, event := range state.trace {
		if event.ToolResult != nil && messageLoopIsMediaArtifactToolName(event.ToolResult.Tool) && !toolStatusFailed(event.ToolResult.Status) {
			return true
		}
	}
	return false
}

func messageLoopIsMediaArtifactToolName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "media.index_authorized_folder", "media_index_authorized_folder", "media.register_assets", "media_register_assets", "artifact.extract", "artifact_extract":
		return true
	default:
		return false
	}
}

func messageLoopMediaArtifactsFastCompleteReply(record map[string]any) string {
	result, _ := record["result"].(map[string]any)
	count := firstPositiveMapInt(result, "count", "artifact_count")
	if count <= 0 {
		count = messageLoopListCount(result["artifacts"])
	}
	if count > 0 {
		return fmt.Sprintf("\u5df2\u5c06 %d \u4e2a\u7d20\u6750\u6ce8\u518c\u5230\u5a92\u4f53\u6c60\uff0c\u53ef\u4ee5\u70b9\u51fb\u4e0b\u65b9\u5361\u7247\u9884\u89c8\u6216\u64ad\u653e\u3002", count)
	}
	return "\u8fd9\u4e2a\u7d20\u6750\u4f4d\u7f6e\u91cc\u6ca1\u6709\u627e\u5230\u53ef\u9884\u89c8\u7684\u5a92\u4f53\u6587\u4ef6\u3002"
}

func messageLoopListCount(value any) int {
	switch rows := value.(type) {
	case []any:
		return len(rows)
	case []map[string]any:
		return len(rows)
	case []string:
		return len(rows)
	default:
		return 0
	}
}

func messageLoopUserRequestedMIDINotes(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasMidi := strings.Contains(text, "midi") || strings.Contains(text, "音符") || strings.Contains(text, "和弦") || strings.Contains(text, "旋律") || strings.Contains(text, "鼓")
	if !hasMidi {
		return false
	}
	for _, token := range []string{
		"note", "notes", "melody", "chord", "drum", "pattern", "write", "insert", "add",
		"写", "写入", "输入", "插入", "添加", "加入", "生成", "做一个", "做成", "和弦进行",
	} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func messageLoopHasVerifiedMIDINoteWrite(state *runState) bool {
	for _, event := range state.trace {
		if event.Verification == nil {
			continue
		}
		ver := event.Verification
		if ver.Postcondition == postconditionMIDINotesPresent && ver.Status == verificationVerified {
			return true
		}
	}
	return false
}

func messageLoopStateShowsExpectedMIDINotes(state *runState) bool {
	trackID, clipID, expectedIDs, expectedCount := messageLoopExpectedMIDINoteWriteEvidence(state)
	if expectedCount == 0 && len(expectedIDs) == 0 {
		return false
	}
	notes := stateClipNotes(state.input.State, trackID, clipID)
	if len(expectedIDs) > 0 {
		return observedNotesContainIDs(notes, expectedIDs)
	}
	return expectedCount > 0 && len(notes) >= expectedCount
}

func messageLoopReadbackShowsExpectedMIDINotes(state *runState) bool {
	trackID, clipID, expectedIDs, expectedCount := messageLoopExpectedMIDINoteWriteEvidence(state)
	if expectedCount == 0 && len(expectedIDs) == 0 {
		return false
	}
	for _, event := range state.trace {
		if event.ToolResult == nil || toolStatusFailed(event.ToolResult.Status) || !messageLoopIsMIDINoteReadName(event.ToolResult.Tool) {
			continue
		}
		result := event.ToolResult.Result
		if clipID != "" {
			observedClipID := firstMapText(result, "clip_id", "id", "item_id")
			if observedClipID != "" && observedClipID != clipID {
				continue
			}
		}
		if trackID != "" {
			observedTrackID := firstMapText(result, "track_id", "target_track_id", "selected_track_id")
			if observedTrackID != "" && observedTrackID != trackID {
				continue
			}
		}
		notes := messageLoopNotesFromReadResult(result)
		if len(notes) == 0 {
			continue
		}
		if len(expectedIDs) > 0 {
			if observedNotesContainIDs(notes, expectedIDs) {
				return true
			}
			if !messageLoopNotesExposeIDs(notes) && len(notes) >= len(expectedIDs) {
				return true
			}
			continue
		}
		if expectedCount > 0 && len(notes) >= expectedCount {
			return true
		}
	}
	return false
}

func messageLoopNotesFromReadResult(result map[string]any) []map[string]any {
	if len(result) == 0 {
		return nil
	}
	if notes := mapRows(result["notes"]); len(notes) > 0 {
		return notes
	}
	if notes := mapRows(result["midi_notes"]); len(notes) > 0 {
		return notes
	}
	for _, key := range []string{"clip", "midi_clip", "data"} {
		if row, ok := result[key].(map[string]any); ok {
			if notes := mapRows(row["notes"]); len(notes) > 0 {
				return notes
			}
			if notes := mapRows(row["midi_notes"]); len(notes) > 0 {
				return notes
			}
		}
	}
	return nil
}

func messageLoopNotesExposeIDs(notes []map[string]any) bool {
	for _, note := range notes {
		if firstMapText(note, "id", "note_id", "vit_note_id", "uid") != "" {
			return true
		}
	}
	return false
}

func messageLoopExpectedMIDINoteWriteEvidence(state *runState) (string, string, []string, int) {
	var trackID, clipID string
	var ids []string
	count := 0
	for _, event := range state.trace {
		if event.ToolResult != nil && messageLoopIsMIDINoteWriteName(event.ToolResult.Tool) {
			result := event.ToolResult.Result
			trackID = firstNonEmpty(firstMapText(result, "track_id", "target_track_id"), trackID)
			clipID = firstNonEmpty(firstMapText(result, "clip_id", "id", "item_id"), clipID)
			if next := noteIDsFromAny(result["inserted_note_ids"]); len(next) > 0 {
				ids = next
			}
			if next := firstPositiveMapInt(result, "inserted_count", "added_count", "written_count", "note_count"); next > 0 {
				count = next
			}
		}
		if event.Verification != nil && event.Verification.Postcondition == postconditionMIDINotesPresent {
			evidence := event.Verification.Evidence
			trackID = firstNonEmpty(firstMapText(evidence, "expected_track_id"), trackID)
			clipID = firstNonEmpty(firstMapText(evidence, "expected_clip_id"), clipID)
			if next := noteIDsFromAny(evidence["expected_note_ids"]); len(next) > 0 {
				ids = next
			}
			if next := firstPositiveMapInt(evidence, "expected_inserted_count"); next > 0 {
				count = next
			}
		}
	}
	if count == 0 && len(ids) > 0 {
		count = len(ids)
	}
	return trackID, clipID, ids, count
}

func messageLoopHasMIDINoteWriteAttempt(state *runState) bool {
	for _, event := range state.trace {
		if event.ToolResult != nil && messageLoopIsMIDINoteWriteName(firstNonEmpty(event.ToolResult.Tool, "")) && !toolStatusFailed(event.ToolResult.Status) {
			return true
		}
		if event.ToolCall != nil && messageLoopIsMIDINoteWriteName(normalizedActionName(*event.ToolCall, executorpkg.Result{})) {
			return true
		}
		if event.Verification != nil && event.Verification.Postcondition == postconditionMIDINotesPresent {
			return true
		}
	}
	return false
}

func messageLoopIsMIDINoteWriteName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "midi.apply_note_patch", "apply_midi_note_patch", "midi.add_notes", "midi.add_notes_bulk", "add_midi_notes", "add_midi_notes_bulk",
		"midi.legacy_add_notes", "midi.legacy_add_notes_bulk":
		return true
	default:
		return false
	}
}

func messageLoopIsMIDINoteReadName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "midi.read_notes", "midi.read_clip_notes", "midi.read_clip_data", "get_midi_clip_notes", "get_midi_clip_data":
		return true
	default:
		return false
	}
}
