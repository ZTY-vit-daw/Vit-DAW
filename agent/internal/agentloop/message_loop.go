package agentloop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	call = coerceMixObservationCall(&state, call)
	call = coerceObservationPackageReadCall(&state, call)
	call = coerceMixTickPrimitiveCall(&state, call)
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
	messageLoopApplyReadOnlyMutationBarrier(state)
	for {
		if stopped, result := l.preflightNaturalMixObservation(ctx, r, state); stopped {
			return result
		}
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
			if reply, needsClarification, ok := messageLoopObservationFallbackAfterLLMError(state, err); ok {
				state.trace = append(state.trace, planner.TraceEvent{
					Kind:    "final_gate",
					Message: "LLM reply failed after read-only mix observation; returned materialized observation fallback: " + err.Error(),
				})
				if needsClarification {
					state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: reply})
					res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", reply, "", "", nil)
					res.NeedsClarification = true
					res.ClarificationQuestion = reply
					return res
				}
				return r.complete(state, reply)
			}
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
			if candidate := messageLoopDeterministicVocalClarificationPendingTick(state, firstNonEmpty(out.Reply, out.ClarificationQuestion)); candidate != nil {
				messageLoopAttachDiagnosisToMixTick(state, candidate, state.input.UserText)
				state.executionMemory.PendingMixTickCandidate = candidate
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "resolved vocal clarification synthesized pending mix tick"})
				return r.complete(state, messageLoopDeterministicVocalClarificationPendingReply(state, candidate))
			}
			if reply, ok := messageLoopClarificationAsPendingMixSuggestion(state, out); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "mix execution question normalized to pending suggestion"})
				return r.complete(state, reply)
			}
			if reply, ok := messageLoopClarificationAsPendingPanTreatment(state, out); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "pan clarification normalized to pending treatment"})
				return r.complete(state, reply)
			}
			question := firstNonEmpty(out.ClarificationQuestion, out.Reply, "请告诉我这次要编辑的具体目标。")
			question = messageLoopClarificationQuestion(state, question)
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
			if candidate := messageLoopDeterministicVocalClarificationPendingTick(state, reply); candidate != nil {
				messageLoopAttachDiagnosisToMixTick(state, candidate, state.input.UserText)
				state.executionMemory.PendingMixTickCandidate = candidate
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "resolved vocal clarification synthesized pending mix tick"})
				return r.complete(state, messageLoopDeterministicVocalClarificationPendingReply(state, candidate))
			}
			if question, ok := messageLoopFinalFocusTrackClarification(state, reply); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: question})
				res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", question, "", "", nil)
				res.NeedsClarification = true
				res.ClarificationQuestion = question
				return res
			}
			return r.complete(state, messageLoopMixObservationFinalReply(state, reply))
		}
		hadMixObservationBeforeTurn := messageLoopHasUsableMixObservation(state)
		for i := range out.ToolCalls {
			call := normalizeMessageLoopToolCall(out.ToolCalls[i], state.completedSteps+1)
			call = resolveMessageLoopBindings(state, call)
			call = coercePluginGrabberLearningToolCall(state.input.UserText, call)
			call = coercePluginGrabberRuntimeToolCall(state.input.UserText, call)
			call = coerceWaveformBakeToMixObservation(state, call)
			call = coerceMixObservationCall(state, call)
			call = coerceObservationPackageReadCall(state, call)
			call = coerceMixTickPrimitiveCall(state, call)
			if reply, ok := messageLoopMixTickProposalAsPendingTreatment(state, call, out); ok {
				state.trace = append(state.trace, planner.TraceEvent{
					Kind:     "final_gate",
					Message:  "natural-language mix tick proposal normalized to pending treatment",
					ToolCall: cloneToolCallPtr(call),
				})
				return r.complete(state, reply)
			}
			if messageLoopIsMixObservationTool(call) && messageLoopHasMixObservationExecution(state) {
				issue := "mix.observe already ran for this turn; summarize the existing observation result instead of requesting it again"
				state.trace = append(state.trace,
					planner.TraceEvent{Kind: "tool_call_blocked", ToolCall: cloneToolCallPtr(call), Message: issue},
					planner.TraceEvent{Kind: "final_gate", Message: issue},
				)
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
				continue
			}
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
			if question, ok := messageLoopFinalFocusTrackClarification(state, reply); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: question})
				res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", question, "", "", nil)
				res.NeedsClarification = true
				res.ClarificationQuestion = question
				return res
			}
			return r.complete(state, messageLoopMixObservationFinalReply(state, reply))
		}
	}
}

func (l *MessageLoop) preflightNaturalMixObservation(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || !messageLoopNeedsDeterministicMixObservation(state) {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_mix_observe_preflight", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	call := messageLoopDeterministicMixObservationCall(state)
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic mix observation preflight"},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: result.Error},
		)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "natural mix request was routed through deterministic mix.observe preflight",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false preflight=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if !messageLoopHasUsableMixObservation(state) {
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>mix.observe ran but did not return a usable acoustic observation; summarize the blocker and do not execute a mix action.</final_gate>"})
	} else if reply, ok := messageLoopDeterministicGainPendingAfterObservation(state); ok {
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:    "final_gate",
			Message: "natural-language gain request normalized to pending treatment after observation",
		})
		return true, r.complete(state, reply)
	} else if stopped, result := l.preflightFocusRelationship(ctx, r, state); stopped {
		return true, result
	}
	return false, Result{}
}

func (l *MessageLoop) preflightFocusRelationship(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || !messageLoopNeedsFocusRelationshipDerive(state) {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_focus_relationship_preflight", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	call := messageLoopFocusRelationshipDeriveCall(state)
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "vocal/focus mix request was routed through deterministic mix.derive focus_vs_project preflight",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false preflight=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	return false, Result{}
}

func messageLoopNeedsFocusRelationshipDerive(state *runState) bool {
	if state == nil || !allowedTool("mix.derive", state.input.AllowedTools) || !messageLoopHasUsableMixObservation(state) {
		return false
	}
	if !messageLoopFocusRelationshipIntent(state.input.UserText) {
		return false
	}
	for _, record := range state.executed {
		if messageLoopExecutionActionName(planner.ToolCall{}, record) == "mix_derive" || strings.EqualFold(messageLoopText(record["tool"]), "mix.derive") {
			return false
		}
	}
	return true
}

func messageLoopFocusRelationshipIntent(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text, "\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal", "voice") &&
		messageLoopTextHasAny(text, "\u9760\u524d", "\u5f80\u524d", "\u7a81\u51fa", "\u6e05\u695a", "forward", "front", "presence", "present")
}

func messageLoopFocusRelationshipDeriveCall(state *runState) planner.ToolCall {
	args := map[string]any{
		"type":      "focus_vs_project",
		"focus":     map[string]any{"role": "vocal", "source": "user_intent"},
		"max_items": 8,
	}
	if state != nil && state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		if observationID := firstMapText(state.recentObservation.Summary, "observation_id"); observationID != "" {
			args["observation_id"] = observationID
		}
		if sessionID := firstMapText(state.recentObservation.Summary, "mix_session_id"); sessionID != "" {
			args["mix_session_id"] = sessionID
		}
		if target := messageLoopMapValue(state.recentObservation.Summary["target_ref"]); len(target) > 0 {
			if strings.EqualFold(firstMapText(target, "kind"), "track") {
				if trackID := firstMapText(target, "id", "track_id"); trackID != "" {
					if messageLoopTrackIsResolvedVocal(state, trackID) {
						args["focus"] = map[string]any{"track_id": trackID, "role": "vocal", "source": "resolved_observation_target"}
					}
				}
			}
		}
	}
	if state != nil {
		if _, ok := args["observation_id"]; !ok {
			if observationID := messageLoopLastMixObservationField(state, "observation_id"); observationID != "" {
				args["observation_id"] = observationID
			}
		}
		if _, ok := args["mix_session_id"]; !ok {
			if sessionID := messageLoopLastMixObservationField(state, "mix_session_id"); sessionID != "" {
				args["mix_session_id"] = sessionID
			}
		}
	}
	return planner.ToolCall{
		ID:     "derive_focus_vs_project",
		Tool:   "mix.derive",
		Args:   args,
		Reason: "derive vocal focus relationship after project observation",
	}
}

func messageLoopNeedsDeterministicMixObservation(state *runState) bool {
	if state == nil {
		return false
	}
	needsRealtimeRefresh := messageLoopRealtimeObservationRequest(state.input.UserText)
	if messageLoopObservationPackageReadRequest(state.input.UserText) && observationIsMixObservation(state.recentObservation) && !needsRealtimeRefresh {
		return false
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopAudioObservationRequest(state.input.UserText) {
		return false
	}
	if messageLoopExplicitPluginOrRawRequest(state.input.UserText) && !messageLoopLowMudPluginPrepRequest(state.input.UserText) && !messageLoopMutationBarrierActive(state) {
		return false
	}
	if messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
		return false
	}
	if !allowedTool(messageLoopPreferredMixObservationTool(state), state.input.AllowedTools) {
		return false
	}
	if messageLoopHasMixObservationExecution(state) {
		return false
	}
	if !needsRealtimeRefresh && (messageLoopHasAnyMixObservationAttempt(state) || messageLoopHasUsableMixObservation(state)) {
		return false
	}
	return true
}

func messageLoopDeterministicMixObservationCall(state *runState) planner.ToolCall {
	args := messageLoopMixObservationArgs(state.input.UserText, map[string]any{})
	scope := strings.ToLower(strings.TrimSpace(fmt.Sprint(args["scope"])))
	if scope != "full_project" && scope != "full_project_with_focus_track" {
		if trackID := firstNonEmpty(
			state.executionMemory.ActiveWorkTargetTrackID,
			firstStateTrackID(state.input.Context),
			firstStateTrackID(state.input.State),
		); trackID != "" {
			setIfEmpty(args, "track_id", trackID)
		}
		if clipID := firstNonEmpty(
			state.executionMemory.ActiveWorkTargetClipID,
			firstStateClipID(state.input.Context),
			firstStateClipID(state.input.State),
		); clipID != "" {
			setIfEmpty(args, "clip_id", clipID)
		}
	}
	return planner.ToolCall{
		ID:     "observe_mix",
		Tool:   messageLoopPreferredMixObservationTool(state),
		Args:   args,
		Reason: "deterministic observation before broad acoustic mix advice",
	}
}

func messageLoopDeterministicGainPendingAfterObservation(state *runState) (string, bool) {
	if state == nil || messageLoopExplicitPluginOrRawRequest(state.input.UserText) || messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
		return "", false
	}
	if messageLoopMutationBarrierActive(state) {
		return "", false
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) || !messageLoopHasUsableMixObservation(state) {
		return "", false
	}
	delta, ok := messageLoopImplicitGainDeltaFromText(state.input.UserText)
	if !ok || delta == 0 || mathAbs(delta) > 2 {
		return "", false
	}
	trackID := firstNonEmpty(
		messageLoopLastMixObservationTrackID(state),
		state.executionMemory.ActiveWorkTargetTrackID,
		firstStateTrackID(state.input.Context),
		firstStateTrackID(state.input.State),
	)
	trackID = messageLoopCanonicalMixTrackID(state, trackID)
	if trackID == "" {
		return "", false
	}
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	treatment := &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    strings.TrimSpace(state.input.UserText),
		TargetRef:                 "track:" + trackID,
		ActionKind:                "gain_balance",
		ProcessorType:             "utility",
		DeltaDB:                   delta,
		Target:                    map[string]any{"delta_db": delta},
		ReasoningSummary:          "explicit small gain move from user text after mix observation",
		Confidence:                "high",
		EvidenceRefs:              []string{observationID},
		ExpiresAfterContextChange: true,
		CreatedFromReply:          state.input.UserText,
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"target_track_id":   trackID,
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
			"source":            "deterministic_gain_pending_after_observation",
		},
	}
	if observationID == "" {
		treatment.EvidenceRefs = nil
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	state.executionMemory.PendingMixTreatment = treatment
	reply := messageLoopPendingMixTreatmentReply(treatment)
	if !messageLoopMixReplyAsksForExecution(reply) && !messageLoopClarificationAsksForExecution(reply) {
		reply += "\n\n如果你认可这个混音建议，需要我继续执行吗？"
	}
	return reply, true
}

func firstStateTrackID(values ...map[string]any) string {
	for _, value := range values {
		if trackID := firstMapText(value, "selected_track_id", "selected_clip_track_id", "track_id", "target_track_id"); trackID != "" {
			return trackID
		}
		for _, row := range messageLoopMapRows(value["tracks"]) {
			if trackID := firstMapText(row, "track_id", "id", "item_id"); trackID != "" {
				return trackID
			}
		}
	}
	return ""
}

func firstStateClipID(values ...map[string]any) string {
	for _, value := range values {
		if clipID := firstMapText(value, "selected_clip_id", "primary_selected_clip_id", "clip_id", "target_clip_id"); clipID != "" {
			return clipID
		}
		for _, row := range messageLoopMapRows(value["tracks"]) {
			for _, clip := range messageLoopMapRows(row["clips"]) {
				if clipID := firstMapText(clip, "clip_id", "id", "item_id"); clipID != "" {
					return clipID
				}
			}
		}
	}
	return ""
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
	next.Tool = messageLoopPreferredMixObservationTool(state)
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
	args = messageLoopMixObservationArgs(state.input.UserText, args)
	next.Args = args
	next.Command = nil
	next.Reason = firstNonEmpty(strings.TrimSpace(next.Reason), "route audio/acoustic observation through mix.observe")
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "clip.warm_waveform_bake was routed through mix.observe for audio observation",
		ToolCall: &next,
	})
	return next
}

func coerceMixObservationCall(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || !messageLoopIsMixObservationTool(call) {
		return call
	}
	next := call
	if strings.EqualFold(strings.TrimSpace(next.Tool), "mix.request_observation") && allowedTool("mix.observe", state.input.AllowedTools) {
		next.Tool = "mix.observe"
	}
	if next.ID == "" {
		next.ID = "observe_mix"
	}
	args := cloneMap(next.Args)
	if args == nil {
		args = map[string]any{}
	}
	if strings.TrimSpace(fmt.Sprint(args["mix_session_id"])) == "" {
		args["mix_session_id"] = "mix_" + strings.TrimSpace(state.goal.GoalID)
	}
	if strings.TrimSpace(fmt.Sprint(args["goal_text"])) == "" {
		args["goal_text"] = strings.TrimSpace(state.input.UserText)
	}
	next.Args = messageLoopMixObservationArgs(state.input.UserText, args)
	return next
}

func coerceObservationPackageReadCall(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || !messageLoopObservationPackageReadRequest(state.input.UserText) || !messageLoopLooksLikePluginParameterRead(call) {
		return call
	}
	if !allowedTool("mix.read", state.input.AllowedTools) || !messageLoopHasAnyMixObservationAttempt(state) {
		return call
	}
	next := call
	next.Tool = "mix.read"
	if next.ID == "" || strings.Contains(strings.ToLower(next.ID), "param") {
		next.ID = "read_mix_observation"
	}
	args := cloneMap(next.Args)
	if args == nil {
		args = map[string]any{}
	}
	for _, key := range []string{"plugin_id", "param_id", "parameter_id", "parameter_name", "plugin_name"} {
		delete(args, key)
	}
	if strings.TrimSpace(fmt.Sprint(args["mix_session_id"])) == "" && strings.TrimSpace(fmt.Sprint(args["observation_id"])) == "" {
		if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
			if observationID := firstMapText(state.recentObservation.Summary, "observation_id"); observationID != "" {
				args["observation_id"] = observationID
			}
			if sessionID := firstMapText(state.recentObservation.Summary, "mix_session_id"); sessionID != "" {
				args["mix_session_id"] = sessionID
			}
		}
	}
	args["keys"] = messageLoopObservationPackageReadKeys(state, args)
	if _, ok := args["max_items"]; !ok {
		args["max_items"] = 12
	}
	next.Args = args
	next.Command = nil
	next.Reason = firstNonEmpty(strings.TrimSpace(call.Reason), "read MixBoard acoustic observation package instead of plugin parameters")
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "plugin parameter read was routed to mix.read for an observation package question",
		ToolCall: &next,
	})
	return next
}

func messageLoopObservationPackageReadKeys(state *runState, args map[string]any) []string {
	keys := []string{"observation.digest", "project.limitations"}
	trackID := firstMapText(args, "track_id", "target_track_id", "selected_track_id")
	if trackID == "" && state != nil {
		trackID = firstNonEmpty(state.executionMemory.ActiveWorkTargetTrackID, state.executionMemory.LastCreatedTrackID)
	}
	if trackID != "" {
		keys = append(keys,
			"track."+trackID+".fast.levels",
			"track."+trackID+".slow.time_energy.summary",
		)
	}
	return keys
}

func messageLoopObservationPackageReadRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasObservation := messageLoopTextHasAny(text,
		"observation", "mixboard", "acoustic", "audio feature", "feature snapshot",
		"\u89c2\u5bdf", "\u58f0\u5b66", "\u97f3\u9891\u7279\u5f81", "\u6570\u636e\u5305", "\u5feb\u7167",
		"\u6ce2\u5f62", "\u5305\u7edc", "\u54cd\u5ea6", "\u5cf0\u503c", "rms", "headroom", "crest",
	)
	hasReadIntent := messageLoopTextHasAny(text,
		"read", "show", "inspect", "missing", "unavailable", "why", "package", "data",
		"\u770b", "\u67e5", "\u8bfb", "\u4e3a\u4ec0\u4e48", "\u4e3a\u5565", "\u6ca1\u6709", "\u7f3a\u5931", "\u4e0d\u89c1", "\u770b\u4e0d\u89c1",
	)
	return hasObservation && hasReadIntent
}

func messageLoopLooksLikePluginParameterRead(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "plugin.get_parameters", "get_plugin_parameters":
		return true
	default:
		return false
	}
}

func coerceMixTickPrimitiveCall(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || messageLoopExplicitPluginOrRawRequest(state.input.UserText) {
		return call
	}
	isPanPrimitive := messageLoopPrimitiveTrackPanCall(call)
	isVolumePrimitive := messageLoopPrimitiveTrackVolumeCall(call)
	if !isVolumePrimitive && !isPanPrimitive {
		return call
	}
	if !messageLoopHasUsableMixObservation(state) && !(isPanPrimitive && messageLoopImplicitPanFollowupRequest(state)) && !(isVolumePrimitive && messageLoopImplicitGainFollowupRequest(state)) {
		return call
	}
	if !allowedTool("mix.propose_tick", state.input.AllowedTools) {
		return call
	}
	trackID := firstMapText(call.Args, "track_id", "target_track_id", "selected_track_id")
	if trackID == "" {
		trackID = firstMapText(call.Command, "track_id", "target_track_id", "selected_track_id")
	}
	if trackID == "" {
		trackID = state.executionMemory.ActiveWorkTargetTrackID
	}
	if trackID == "" {
		return call
	}
	if isPanPrimitive {
		operation := ""
		args := map[string]any{
			"track_id": trackID,
			"evidence": map[string]any{
				"adapter":        "primitive_track_pan",
				"primitive_tool": strings.TrimSpace(call.Tool),
				"primitive_cmd":  firstMapText(call.Args, "cmd", "command"),
				"reason":         strings.TrimSpace(call.Reason),
			},
		}
		if target, ok := messageLoopPrimitiveTrackPanTarget(state, call); ok {
			operation = "track_pan_set"
			args["target_pan"] = clampPanTarget(target)
		} else if delta, ok := messageLoopPrimitiveTrackPanDelta(call); ok && delta != 0 && mathAbs(delta) <= 0.15 {
			operation = "track_pan_adjust"
			args["delta_pan"] = delta
		}
		if operation == "" {
			return call
		}
		args["operation"] = operation
		next := call
		next.Tool = "mix.propose_tick"
		next.Command = nil
		next.Args = args
		next.Reason = firstNonEmpty(strings.TrimSpace(call.Reason), "wrap acoustic mix pan change as a single mix tick")
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "acoustic mix pan primitive was wrapped as mix.propose_tick",
			ToolCall: &next,
		})
		return next
	}
	delta, ok := messageLoopPrimitiveTrackVolumeDelta(state, call, trackID)
	if !ok {
		delta, ok = messageLoopImplicitGainDeltaFromText(firstNonEmpty(state.input.UserText, call.Reason))
	}
	if !ok || delta == 0 {
		return call
	}
	next := call
	next.Tool = "mix.propose_tick"
	next.Command = nil
	next.Args = map[string]any{
		"operation": "track_gain_adjust",
		"track_id":  trackID,
		"delta_db":  delta,
		"evidence": map[string]any{
			"adapter":        "primitive_track_volume",
			"primitive_tool": strings.TrimSpace(call.Tool),
			"primitive_cmd":  firstMapText(call.Args, "cmd", "command"),
			"reason":         strings.TrimSpace(call.Reason),
		},
	}
	next.Reason = firstNonEmpty(strings.TrimSpace(call.Reason), "wrap confirmed acoustic mix volume change as a single mix tick")
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "confirmed acoustic mix volume primitive was wrapped as mix.propose_tick",
		ToolCall: &next,
	})
	return next
}

func messageLoopPendingConfirmationCall(call planner.ToolCall, result executorpkg.Result) planner.ToolCall {
	if !messageLoopIsMixProposeTickCall(call) {
		return call
	}
	tickID := firstMapText(result.Result, "tick_id")
	if tickID == "" {
		return call
	}
	pending := planner.ToolCall{
		ID:     firstNonEmpty(call.ID, "apply_mix_tick"),
		Tool:   "mix.apply_tick",
		Reason: "apply the confirmed mix tick",
		Args: map[string]any{
			"tick_id": tickID,
		},
	}
	if trackID := firstMapText(result.Result, "track_id"); trackID != "" {
		pending.Args["track_id"] = trackID
	}
	return pending
}

func messageLoopShouldAutoApplyConfirmedMixTick(state *runState, call planner.ToolCall, result executorpkg.Result) bool {
	return false
}

func messageLoopMixExecutionApproval(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return messageLoopExplicitMixExecutionConfirmation(text)
}

func messageLoopIsMixProposeTickCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	return name == "mix.propose_tick" || name == "mix_propose_tick"
}

func messageLoopMixTickProposalAsPendingTreatment(state *runState, call planner.ToolCall, out messageLoopOutput) (string, bool) {
	if state == nil || !messageLoopIsMixProposeTickCall(call) {
		return "", false
	}
	if messageLoopMutationBarrierActive(state) {
		return "", false
	}
	if (!messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopImplicitPanFollowupRequest(state) && !messageLoopExplicitMixExecutionConfirmation(state.input.UserText)) || messageLoopExplicitPluginOrRawRequest(state.input.UserText) {
		return "", false
	}
	treatment := messageLoopMixTreatmentPendingFromTickProposal(state, call, out)
	if treatment == nil {
		return "", false
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	state.executionMemory.PendingMixTreatment = treatment
	reply := strings.TrimSpace(out.Reply)
	if reply == "" || messageLoopReplyClaimsCompletedMixWrite(reply) {
		reply = messageLoopPendingMixTreatmentReply(treatment)
	}
	reply = messageLoopStripMixTreatmentPendingMarkup(reply)
	if reply == "" {
		reply = messageLoopPendingMixTreatmentReply(treatment)
	}
	if !messageLoopMixReplyAsksForExecution(reply) && !messageLoopClarificationAsksForExecution(reply) {
		reply += "\n\n如果你认可这个混音建议，需要我继续执行吗？"
	}
	return reply, true
}

func messageLoopReplyClaimsCompletedMixWrite(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"已完成", "已经完成", "已执行", "已经执行", "执行完成", "设置为", "已设置",
		"completed", "done", "applied", "has been set", "set to",
	)
}

func messageLoopMixTreatmentPendingFromTickProposal(state *runState, call planner.ToolCall, out messageLoopOutput) *MixTreatmentPending {
	if state == nil {
		return nil
	}
	args := cloneMap(call.Args)
	if len(args) == 0 {
		args = cloneMap(call.Command)
	}
	operation := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		firstMapText(args, "operation", "action", "type"),
		normalizedActionName(call, executorpkg.Result{}),
	)))
	if operation == "" {
		return nil
	}
	trackID := firstNonEmpty(
		firstMapText(args, "track_id", "target_track_id", "selected_track_id"),
		messageLoopPendingMixCandidateTrackID(state, firstNonEmpty(out.Reply, state.input.UserText, call.Reason), state.input.UserText),
	)
	trackID = messageLoopCanonicalMixTrackID(state, trackID)
	if trackID == "" {
		return nil
	}
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	treatment := &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    strings.TrimSpace(state.input.UserText),
		TargetRef:                 "track:" + trackID,
		ProcessorType:             "utility",
		ReasoningSummary:          firstNonEmpty(strings.TrimSpace(call.Reason), "model proposed a typed mix tick from natural language"),
		Confidence:                "high",
		EvidenceRefs:              []string{observationID},
		ExpiresAfterContextChange: true,
		CreatedFromReply:          firstNonEmpty(out.Reply, state.input.UserText),
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"target_track_id":   trackID,
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
			"source_tool":       strings.TrimSpace(call.Tool),
		},
	}
	if observationID == "" {
		treatment.EvidenceRefs = nil
	}
	switch operation {
	case "track_gain_adjust":
		delta, ok := firstNumericMapValue(args, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db")
		if !ok || delta == 0 || mathAbs(delta) > 2 {
			return nil
		}
		treatment.ActionKind = "gain_balance"
		treatment.DeltaDB = delta
		treatment.Target = map[string]any{"delta_db": delta}
	case "track_pan_adjust":
		delta, ok := firstNumericMapValue(args, "delta_pan", "pan_delta")
		if !ok || delta == 0 || mathAbs(delta) > 0.15 {
			return nil
		}
		treatment.ActionKind = "pan_balance"
		treatment.DeltaPan = delta
		treatment.Target = map[string]any{"delta_pan": delta}
	case "track_pan_set":
		target, ok := firstNumericMapValue(args, "target_pan", "pan", "pan_value")
		if !ok {
			return nil
		}
		target = clampPanTarget(target)
		if _, userTarget := messageLoopTreatmentPanFromReply(state.input.UserText); userTarget != nil {
			target = *userTarget
		}
		treatment.ActionKind = "pan_balance"
		treatment.TargetPan = &target
		treatment.Target = map[string]any{"target_pan": target}
	default:
		return nil
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	return treatment
}

func messageLoopPendingMixTreatmentReply(treatment *MixTreatmentPending) string {
	if treatment == nil {
		return "我已把这个混音动作整理成待确认建议。"
	}
	target := strings.TrimPrefix(strings.TrimSpace(treatment.TargetRef), "track:")
	if target == "" {
		target = "当前目标"
	}
	switch strings.TrimSpace(treatment.ActionKind) {
	case "gain_balance":
		return fmt.Sprintf("我建议先把 %s 的电平调整 %.2f dB。", target, treatment.DeltaDB)
	case "pan_balance":
		if treatment.TargetPan != nil {
			return fmt.Sprintf("我建议先把 %s 的声像设置到 %.2f。", target, *treatment.TargetPan)
		}
		return fmt.Sprintf("我建议先把 %s 的声像调整 %.2f。", target, treatment.DeltaPan)
	default:
		return "我已把这个混音动作整理成待确认建议。"
	}
}

func messageLoopConservativeLowMudTreatmentPendingReply(treatment *MixTreatmentPending) string {
	if treatment == nil {
		return "我已把低频处理整理成待确认的保守 EQ 预备方案；当前缺少 band_energy_summary，所以只作为可回退探测，不会直接加载或写入插件参数。"
	}
	target := strings.TrimPrefix(strings.TrimSpace(treatment.TargetRef), "track:")
	if target == "" || strings.EqualFold(target, "project") {
		target = "当前目标"
	} else {
		target = "Track " + target
	}
	return fmt.Sprintf("已基于这次观察把 %s 的低频糊问题整理成一个待确认的保守 EQ 预备方案。当前 band_energy_summary / 频谱细节仍缺失，所以我不会声称已经观测到低频堆积，也不会直接加载或改参数；候选只限于可回退的低切/低中频轻微削减探测。如果你确认，我会继续进入插件准备和参数确认。", target)
}

func messageLoopPrimitiveTrackVolumeCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "track.volume", "set_volume":
		return true
	default:
		return false
	}
}

func messageLoopPrimitiveTrackPanCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "track.pan", "track_pan", "set_pan":
		return true
	default:
		return false
	}
}

func messageLoopImplicitGainFollowupRequest(state *runState) bool {
	if state == nil {
		return false
	}
	if messageLoopNaturalMixRequest(state.input.UserText) {
		return true
	}
	if _, ok := messageLoopImplicitGainDeltaFromText(state.input.UserText); !ok {
		return false
	}
	return messageLoopHasUsableMixObservation(state) ||
		len(messageLoopMapRows(state.input.State["tracks"])) > 0 ||
		len(messageLoopMapRows(state.contextSnapshot["tracks"])) > 0 ||
		state.executionMemory.ActiveWorkTargetTrackID != ""
}

func messageLoopImplicitGainDeltaFromText(text string) (float64, bool) {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return 0, false
	}
	if delta, _, ok := messageLoopExtractSingleGainDelta(text); ok && delta != 0 && mathAbs(delta) <= 2 {
		return delta, true
	}
	hasGainContext := messageLoopTextHasAny(text,
		"\u8fd9\u6761\u8f68", "\u5f53\u524d\u8f68", "\u8f68\u9053", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca", "\u54cd",
		"track", "volume", "level", "gain", "loud", "quiet",
	)
	if !hasGainContext {
		return 0, false
	}
	switch {
	case messageLoopTextHasAny(text,
		"\u592a\u54cd", "\u592a\u5927", "\u592a\u51b2", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u5c0f\u58f0", "\u5c0f\u4e00\u70b9", "\u4f4e\u4e00\u70b9", "\u6536\u4e00\u70b9", "\u5f80\u4e0b",
		"too loud", "lower", "reduce", "decrease", "quieter", "turn down", "down",
	):
		if messageLoopTextHasAny(text, "\u4e00\u70b9", "\u7a0d\u5fae", "\u5c0f\u5e45", "\u8f7b\u5fae", "a little", "slightly", "small") {
			return -1.5, true
		}
		return -2, true
	case messageLoopTextHasAny(text,
		"\u592a\u5c0f", "\u592a\u8f7b", "\u542c\u4e0d\u89c1", "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u66f4\u54cd", "\u5927\u4e00\u70b9", "\u9ad8\u4e00\u70b9", "\u5f80\u4e0a",
		"too quiet", "raise", "boost", "increase", "louder", "turn up", "up",
	):
		if messageLoopTextHasAny(text, "\u4e00\u70b9", "\u7a0d\u5fae", "\u5c0f\u5e45", "\u8f7b\u5fae", "a little", "slightly", "small") {
			return 1.5, true
		}
		return 2, true
	default:
		return 0, false
	}
}

func messageLoopPrimitiveTrackPanTarget(state *runState, call planner.ToolCall) (float64, bool) {
	if _, target := messageLoopTreatmentPanFromReply(state.input.UserText); target != nil {
		return *target, true
	}
	if value, ok := firstNumericMapValue(call.Args, "target_pan", "pan", "pan_value"); ok {
		return value, true
	}
	if value, ok := firstNumericMapValue(call.Command, "target_pan", "pan", "pan_value"); ok {
		return value, true
	}
	return 0, false
}

func messageLoopPrimitiveTrackPanDelta(call planner.ToolCall) (float64, bool) {
	if value, ok := firstNumericMapValue(call.Args, "delta_pan", "pan_delta"); ok {
		return value, true
	}
	if value, ok := firstNumericMapValue(call.Command, "delta_pan", "pan_delta"); ok {
		return value, true
	}
	return 0, false
}

func messageLoopPrimitiveTrackVolumeDelta(state *runState, call planner.ToolCall, trackID string) (float64, bool) {
	if value, ok := firstNumericMapValue(call.Args, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value, true
	}
	if value, ok := firstNumericMapValue(call.Command, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value, true
	}
	if targetDB, ok := firstNumericMapValue(call.Args, "db", "volume_db", "gain_db", "fader_db"); ok {
		if currentDB, found := messageLoopCurrentTrackDB(state, trackID); found {
			return targetDB - currentDB, true
		}
	}
	if targetDB, ok := firstNumericMapValue(call.Command, "db", "volume_db", "gain_db", "fader_db"); ok {
		if currentDB, found := messageLoopCurrentTrackDB(state, trackID); found {
			return targetDB - currentDB, true
		}
	}
	return 0, false
}

func messageLoopCurrentTrackDB(state *runState, trackID string) (float64, bool) {
	if state == nil || strings.TrimSpace(trackID) == "" {
		return 0, false
	}
	for _, row := range messageLoopMapRows(state.input.State["tracks"]) {
		if firstMapText(row, "track_id", "id") != trackID {
			continue
		}
		return firstNumericMapValue(row, "volume_db", "gain_db", "fader_db", "db")
	}
	for _, row := range messageLoopMapRows(state.contextSnapshot["tracks"]) {
		if firstMapText(row, "track_id", "id") != trackID {
			continue
		}
		return firstNumericMapValue(row, "volume_db", "gain_db", "fader_db", "db")
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if value, found := messageLoopCurrentTrackDBFromResult(result, trackID); found {
			return value, true
		}
	}
	return 0, false
}

func messageLoopCurrentTrackDBFromResult(result map[string]any, trackID string) (float64, bool) {
	if len(result) == 0 || strings.TrimSpace(trackID) == "" {
		return 0, false
	}
	for _, source := range []any{
		result["tracks"],
		messageLoopMapValue(result["project"])["tracks"],
		messageLoopMapValue(result["mixboard"])["tracks"],
		messageLoopMapValue(result["observation"])["tracks"],
	} {
		for _, row := range messageLoopMapRows(source) {
			if firstMapText(row, "track_id", "id") != trackID {
				continue
			}
			return firstNumericMapValue(row, "volume_db", "gain_db", "fader_db", "db")
		}
	}
	return 0, false
}

func firstNumericMapValue(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if row == nil {
			return 0, false
		}
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case json.Number:
			if n, err := typed.Float64(); err == nil {
				return n, true
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return n, true
			}
		}
	}
	return 0, false
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

func messageLoopClarificationQuestion(state *runState, question string) string {
	question = strings.TrimSpace(question)
	if state == nil || question == "" {
		return question
	}
	if messageLoopFocusRelationshipIntent(state.input.UserText) && !messageLoopHasResolvedFocusTrack(state) {
		if messageLoopClarificationAsksForExecution(question) || !messageLoopClarificationAsksForFocusTrack(question) {
			return messageLoopFocusTrackClarificationQuestion()
		}
	}
	return question
}

func messageLoopFocusTrackClarificationQuestion() string {
	return "\u9700\u8981\u5148\u786e\u8ba4\u54ea\u6761\u662f\u4e3b\u5531\u8f68\u3002\u8bf7\u544a\u8bc9\u6211\u4e3b\u5531\u662f Track \u51e0\uff0c\u6216\u628a\u4e3b\u5531\u8f68\u91cd\u547d\u540d\u4e3a vocal / \u4e3b\u5531\u540e\u8ba9\u6211\u91cd\u65b0\u89c2\u5bdf\u3002"
}

func messageLoopClarificationAsPendingMixSuggestion(state *runState, out messageLoopOutput) (string, bool) {
	if state == nil || !out.NeedsClarification {
		return "", false
	}
	if messageLoopMutationBarrierActive(state) {
		return "", false
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) || messageLoopExplicitPluginOrRawRequest(state.input.UserText) || !messageLoopHasUsableMixObservation(state) {
		return "", false
	}
	reply := strings.TrimSpace(firstNonEmpty(out.Reply, out.ClarificationQuestion))
	if reply == "" {
		return "", false
	}
	if messageLoopFocusRelationshipIntent(state.input.UserText) && !messageLoopHasResolvedFocusTrack(state) {
		return "", false
	}
	hasSmallGainMove := false
	if delta, _, ok := messageLoopExtractSingleGainDelta(reply); ok && delta != 0 && mathAbs(delta) <= 2 {
		hasSmallGainMove = true
	}
	deltaPan, targetPan := messageLoopTreatmentPanFromReply(reply)
	hasSmallPanMove := deltaPan != 0 || targetPan != nil
	if !hasSmallGainMove && !hasSmallPanMove {
		return "", false
	}
	executionQuestion := messageLoopTextHasAny(strings.ToLower(reply), "should i", "shall i", "would you like me", "\u662f\u5426", "\u8981\u4e0d\u8981", "\u8981\u6211")
	if !executionQuestion && !messageLoopMixReplyAsksForExecution(reply) && !messageLoopClarificationAsksForExecution(reply) {
		return "", false
	}
	return messageLoopMixObservationFinalReply(state, reply), true
}

func messageLoopClarificationAsPendingPanTreatment(state *runState, out messageLoopOutput) (string, bool) {
	if state == nil || !out.NeedsClarification {
		return "", false
	}
	if messageLoopMutationBarrierActive(state) {
		return "", false
	}
	userText := strings.TrimSpace(state.input.UserText)
	isNaturalMix := messageLoopNaturalMixRequest(userText)
	isImplicitPanFollowup := messageLoopImplicitPanFollowupRequest(state)
	if (!isNaturalMix && !isImplicitPanFollowup) || messageLoopExplicitPluginOrRawRequest(userText) {
		return "", false
	}
	if !messageLoopHasUsableMixObservation(state) && !messageLoopHasPanPendingContext(state) {
		return "", false
	}
	deltaPan, targetPan := messageLoopTreatmentPanFromReply(userText)
	if deltaPan == 0 && targetPan == nil {
		deltaPan, targetPan = messageLoopTreatmentPanFromReply(firstNonEmpty(out.Reply, out.ClarificationQuestion))
		if deltaPan == 0 && targetPan == nil {
			return "", false
		}
	}
	targetID := messageLoopPendingMixCandidateTrackID(state, firstNonEmpty(out.Reply, userText), userText)
	if targetID == "" {
		return "", false
	}
	targetID = messageLoopCanonicalMixTrackID(state, targetID)
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	treatment := &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    strings.TrimSpace(state.input.UserText),
		TargetRef:                 "track:" + targetID,
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		DeltaPan:                  deltaPan,
		TargetPan:                 targetPan,
		ReasoningSummary:          "explicit pan move from user text",
		Confidence:                "high",
		EvidenceRefs:              []string{observationID},
		NeedsResolution:           nil,
		ExpiresAfterContextChange: true,
		CreatedFromReply:          firstNonEmpty(out.Reply, state.input.UserText),
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"target_track_id":   targetID,
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
		},
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	state.executionMemory.PendingMixTreatment = treatment
	reply := strings.TrimSpace(out.Reply)
	if reply == "" {
		reply = "我已把这个声像小动作作为待确认混音步骤。"
	}
	if !messageLoopMixReplyAsksForExecution(reply) && !messageLoopClarificationAsksForExecution(reply) {
		reply += "\n\n如果你认可这个小步建议，需要我继续执行吗？"
	}
	return reply, true
}

func messageLoopHasPanPendingContext(state *runState) bool {
	if state == nil {
		return false
	}
	return len(messageLoopPendingMixCandidateTrackRows(state)) > 0 || strings.TrimSpace(state.executionMemory.ActiveWorkTargetTrackID) != ""
}

func messageLoopCanonicalMixTrackID(state *runState, ref string) string {
	ref = strings.TrimSpace(ref)
	if state == nil || ref == "" {
		return ref
	}
	for _, row := range messageLoopPendingMixCandidateTrackRows(state) {
		trackID := firstMapText(row, "track_id", "id", "target_track_id")
		if trackID == "" {
			continue
		}
		if strings.EqualFold(trackID, ref) {
			return trackID
		}
		for _, alias := range messageLoopTrackMentionAliases(row) {
			if strings.EqualFold(strings.TrimSpace(alias), ref) {
				return trackID
			}
		}
	}
	return ref
}

func messageLoopFinalFocusTrackClarification(state *runState, reply string) (string, bool) {
	reply = strings.TrimSpace(reply)
	if state == nil || reply == "" || !messageLoopFocusRelationshipIntent(state.input.UserText) || messageLoopHasResolvedFocusTrack(state) {
		return "", false
	}
	if messageLoopClarificationAsksForFocusTrack(reply) && !messageLoopClarificationAsksForExecution(reply) {
		return messageLoopClarificationQuestion(state, reply), true
	}
	if _, _, ok := messageLoopExtractSingleGainDelta(reply); ok {
		return messageLoopFocusTrackClarificationQuestion(), true
	}
	if messageLoopMixReplyAsksForExecution(reply) {
		return messageLoopFocusTrackClarificationQuestion(), true
	}
	return "", false
}

func messageLoopHasResolvedFocusTrack(state *runState) bool {
	if state == nil {
		return false
	}
	if messageLoopUserExplicitlyIdentifiesVocalTrackResolved(state.input.UserText) {
		return true
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		if target := messageLoopMapValue(state.recentObservation.Summary["target_ref"]); messageLoopTargetRefIsTrack(target) {
			if messageLoopTrackIsResolvedVocal(state, firstMapText(target, "id", "track_id")) {
				return true
			}
		}
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if len(result) == 0 {
			continue
		}
		if target := messageLoopObservationTargetRefFromResult(result); messageLoopTargetRefIsTrack(target) {
			if messageLoopTrackIsResolvedVocal(state, firstMapText(target, "id", "track_id")) {
				return true
			}
		}
		observation := messageLoopObservationFromResult(result)
		if target := messageLoopMapValue(observation["target_ref"]); messageLoopTargetRefIsTrack(target) {
			if messageLoopTrackIsResolvedVocal(state, firstMapText(target, "id", "track_id")) {
				return true
			}
		}
	}
	return false
}

func messageLoopUserExplicitlyIdentifiesVocalTrack(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if !messageLoopTextHasAny(lower, "主唱", "人声", "vocal", "lead vocal", "voice") {
		return false
	}
	if idx, ok := messageLoopUserTrackIndexFromText(text); ok && idx > 0 {
		return true
	}
	return false
}

func messageLoopUserExplicitlyIdentifiesVocalTrackResolved(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if !messageLoopTextHasAny(lower, "\u4e3b\u5531", "\u4eba\u58f0", "\u4eba\u8072", "vocal", "lead vocal", "voice") {
		return false
	}
	if idx, ok := messageLoopUserTrackIndexFromText(text); ok && idx > 0 {
		return true
	}
	return false
}

func messageLoopTrackIsResolvedVocal(state *runState, trackID string) bool {
	trackID = strings.TrimSpace(trackID)
	if state == nil || trackID == "" {
		return false
	}
	if messageLoopUserExplicitlyIdentifiesVocalTrackResolved(state.input.UserText) {
		return true
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if len(result) == 0 {
			continue
		}
		if track := messageLoopFindProjectTrackInResult(result, trackID); messageLoopTrackHasVocalEvidence(track) {
			return true
		}
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		if track := messageLoopFindProjectTrackInSummary(state.recentObservation.Summary, trackID); messageLoopTrackHasVocalEvidence(track) {
			return true
		}
	}
	return false
}

func messageLoopTargetRefIsTrack(target map[string]any) bool {
	return len(target) > 0 && strings.EqualFold(firstMapText(target, "kind"), "track") && firstMapText(target, "id", "track_id") != ""
}

func messageLoopFindProjectTrackInResult(result map[string]any, trackID string) map[string]any {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return nil
	}
	observation := messageLoopObservationFromResult(result)
	packages := []map[string]any{
		messageLoopMapValue(observation["project_package"]),
		messageLoopMapValue(result["project_package"]),
	}
	for _, pkg := range packages {
		for _, track := range messageLoopMapRows(pkg["tracks"]) {
			if strings.EqualFold(firstMapText(track, "track_id", "id"), trackID) {
				return track
			}
		}
	}
	return nil
}

func messageLoopFindProjectTrackInSummary(summary map[string]any, trackID string) map[string]any {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return nil
	}
	project := messageLoopMapValue(summary["project_package"])
	for _, track := range messageLoopMapRows(project["tracks"]) {
		if strings.EqualFold(firstMapText(track, "track_id", "id"), trackID) {
			return track
		}
	}
	return nil
}

func messageLoopTrackHasVocalEvidence(track map[string]any) bool {
	if len(track) == 0 {
		return false
	}
	role := strings.ToLower(firstMapText(track, "role_guess", "role"))
	if role == "vocal" || role == "voice" || role == "lead_vocal" {
		return true
	}
	text := strings.ToLower(strings.Join([]string{
		firstMapText(track, "name", "track_name", "user_label", "label"),
		firstMapText(track, "clip_name", "primary_clip_name"),
	}, " "))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text, "vocal", "voice", "lead vocal", "lead_vox", "vox", "主唱", "人声", "人聲")
}

func messageLoopClarificationAsksForExecution(question string) bool {
	text := strings.ToLower(strings.TrimSpace(question))
	if text == "" {
		return false
	}
	return messageLoopMixReplyAsksForExecution(text) ||
		(messageLoopTextHasAny(text, "execute", "apply", "continue", "do it", "执行", "继续") && messageLoopTextHasAny(text, "db", "分贝"))
}

func messageLoopClarificationAsksForFocusTrack(question string) bool {
	text := strings.ToLower(strings.TrimSpace(question))
	if text == "" {
		return false
	}
	if messageLoopTextHasAny(text, "哪条", "哪一条", "哪个", "哪一个", "track 几", "track几", "which track", "what track", "which one") {
		return true
	}
	return messageLoopTextHasAny(text, "主唱", "人声", "vocal", "lead vocal") &&
		messageLoopTextHasAny(text, "轨", "track") &&
		messageLoopTextHasAny(text, "确认", "告诉", "说明", "identify", "tell me", "clarify")
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
		if messageLoopReadOnlyObservationRequest(state.input.UserText) {
			modeRules += `
Read-only acoustic observation:
- The current user turn explicitly asks for read-only/observe-only analysis. You may use mix.observe, mix.read, mix.derive, and read/list/project-state tools only.
- Do not call mix.propose_tick, mix.apply_tick, track.volume, track.pan, plugin preparation/load/learn/apply/write tools, or any DAW mutation tool.
- Do not append mix_treatment_pending and do not ask whether to continue executing. Summarize observed facts, missing or partial evidence, and state that no pending action was created.`
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
- For natural/broad mixing goals such as making a vocal more forward, increasing loudness, reducing mud/harshness, tightening dynamics, adding space, or "mix this audio", you MUST call mix.observe first and wait for its result before choosing plugins, learning plugin profiles, loading effects, changing volume, or writing parameters.
- Choose mix.observe scope from intent, not trigger phrases: selected_clip, selected_track, named_track, track_group, full_project, or full_project_with_focus_track. Use project_context for current-track mixing, full_project for overall mix questions, and full_project_with_focus_track for vocal/lead/focus relationships.
- mix.observe returns a digest and catalog. Use mix.read for the catalog entries you need and mix.derive for local relationship packages such as rank_tracks, focus_vs_project, a_vs_b, group_overlap, or before_after. Do not manually compare large raw packages in your hidden reasoning when a relationship package can be derived locally.
- For vocal/lead/focus relationship goals, only treat a track as the vocal when the track name/metadata explicitly identifies it as vocal/voice/lead/主唱/人声, or the user explicitly identifies the track by name/index. If the project only has generic names such as Track 1 / Track 2 and you are not sure which one is the vocal, ask which track is the lead vocal. Do not propose or store an executable move while asking that clarification.
- Do not call clip.warm_waveform_bake / warm_waveform_bake directly for broad mixing observation. mix.observe owns waveform and envelope feature preparation.
- After mix.observe/mix.read/mix.derive for a broad mixing request, stop and summarize the observed project/audio facts plus one suggested next small move, then ask whether the user wants you to continue executing that move. Do not load plugins, learn profiles, change volume, apply controls, or write parameters in the same user request. Wait for the user to explicitly confirm a concrete follow-up action first.
- For Chinese acoustic observation replies, use natural-language sections in this order: 结论、证据、限制、建议. Keep the evidence human-readable, such as "来自 L3 频段、声像和响度分析"; do not expose schema names, source/render revision strings, raw evidence_ref lists, raw JSON, waveform arrays, tile payloads, or internal IDs unless the user explicitly asks for technical details.
- Treat deep/slow packages as optional. If they are missing, pending, partial, or blocked, say what uncertainty remains and base suggestions only on available evidence.
- If acoustic_package_status.v0 shows l3_deep building or partial, reply in Chinese with the available L1 facts, the L3 feature status, tile/coverage progress when present, and say full-song band/stereo judgement is not reliable yet. Do not create pending actions or ask to continue executing for read-only observation.
- When the user confirms the proposed small mix move, use mix.propose_tick and then mix.apply_tick; v1 supports track_gain_adjust up to +/-2 dB through set_volume and track_pan_adjust/track_pan_set through set_pan. Do not call track.volume or track.pan directly for an acoustic mix tick.
- Keep each mixing action to one safe small step or one clearly coupled small move. v1 direct execution supports gain and pan ticks only.
- For a simple concrete gain/pan move that v1 can execute as one acoustic mix tick, ask for confirmation in normal user-facing text with the concrete small amount; do not append mix_treatment_pending for that tick. The local runtime will turn the confirmed move into mix.propose_tick/mix.apply_tick.
- For plugin/EQ/compressor/reverb/delay/saturation or non-tick gain/pan treatment after observation, you may propose a treatment direction but you must not execute it in the same turn. If you propose one, append one internal marker line exactly like: mix_treatment_pending: {"schema_version":"mix_treatment_pending.v0","status":"pending_confirmation","intent":"...","target_ref":"track:<id>|vocal_unknown|project","action_kind":"plugin_treatment|gain_balance|pan_balance","processor_type":"eq|compressor|reverb|delay|saturation|utility|unknown","reasoning_summary":"...","confidence":"low|medium|high","evidence_refs":["..."],"needs_resolution":["target_track","plugin_instance","plugin_profile","exact_control"],"expires_after_context_change":true}. For gain_balance only, include an explicit "delta_db" within +/-2 dB; for pan_balance include either "delta_pan" within +/-0.15 or "target_pan" within -1.0..+1.0. If you do not have an exact small value, leave exact_control in needs_resolution instead. Clarification and observation-only branches must not append this marker. This marker is for the local resolver and will be hidden from the user.
- Do not invent plugin instances, profiles, controls, or exact parameters in a treatment pending. The local resolver decides whether the confirmed treatment can execute, needs preparation, or needs clarification.
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
	if repaired := escapeBareNewlinesInJSONStringLiterals(strings.TrimSpace(raw)); repaired != strings.TrimSpace(raw) {
		if out, err := decodeMessageLoopCandidate(repaired); err == nil {
			return normalizeMessageLoopOutput(out), nil
		}
	}
	for _, candidate := range messageLoopJSONCandidates(raw) {
		out, err := decodeMessageLoopCandidate(candidate)
		if err == nil {
			return normalizeMessageLoopOutput(out), nil
		}
		if repaired := escapeBareNewlinesInJSONStringLiterals(candidate); repaired != candidate {
			out, repairedErr := decodeMessageLoopCandidate(repaired)
			if repairedErr == nil {
				return normalizeMessageLoopOutput(out), nil
			}
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
	if out.Final || out.NeedsClarification || strings.TrimSpace(out.FailureReason) != "" || len(out.ToolCalls) > 0 {
		return true
	}
	reply := strings.TrimSpace(out.Reply)
	return reply != "" && messageLoopReplyHasPendingMarker(reply)
}

func messageLoopReplyHasPendingMarker(reply string) bool {
	return messageLoopMixTreatmentPendingPattern.MatchString(reply)
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

func escapeBareNewlinesInJSONStringLiterals(text string) string {
	if text == "" {
		return text
	}
	var b strings.Builder
	b.Grow(len(text) + 8)
	inString := false
	escaped := false
	changed := false
	for _, ch := range text {
		if inString {
			if escaped {
				escaped = false
				b.WriteRune(ch)
				continue
			}
			switch ch {
			case '\\':
				escaped = true
				b.WriteRune(ch)
			case '"':
				inString = false
				b.WriteRune(ch)
			case '\n':
				changed = true
				b.WriteString(`\n`)
			case '\r':
				changed = true
				b.WriteString(`\r`)
			default:
				b.WriteRune(ch)
			}
			continue
		}
		if ch == '"' {
			inString = true
		}
		b.WriteRune(ch)
	}
	if !changed {
		return text
	}
	return b.String()
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
	tickID := resolveMixTickRef(state, firstMapText(call.Args, "tick_ref", "mix_tick_ref", "tick_id"))
	if tickID == "" && messageLoopToolWantsMixTick(call) {
		tickID = state.executionMemory.LastMixTickID
	}
	if tickID != "" && messageLoopToolWantsMixTick(call) {
		setIfEmpty(call.Args, "tick_id", tickID)
		setIfEmpty(call.Command, "tick_id", tickID)
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

func resolveMixTickRef(state *runState, ref string) string {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "$"))
	switch strings.ToLower(ref) {
	case "last_mix_tick", "last_proposed_mix_tick", "proposed_mix_tick":
		return state.executionMemory.LastMixTickID
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

func messageLoopToolWantsMixTick(call planner.ToolCall) bool {
	name := normalizedActionName(call, executorpkg.Result{})
	switch name {
	case "mix.apply_tick", "mix_apply_tick", "mix.rollback_tick", "mix_rollback_tick":
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
	if state == nil {
		return ""
	}
	if messageLoopMutationBarrierActive(state) {
		return messageLoopReadOnlyGuardIssue(call)
	}
	if messageLoopExplicitPluginOrRawRequest(state.input.UserText) && !messageLoopLowMudPluginPrepRequest(state.input.UserText) {
		return ""
	}
	isMixIntent := messageLoopNaturalMixRequest(state.input.UserText) || messageLoopImplicitPanFollowupRequest(state) || messageLoopImplicitGainFollowupRequest(state)
	if !isMixIntent {
		return ""
	}
	if messageLoopIsWaveformBakeTool(call) {
		return "broad mixing observation must use mix.observe; waveform/envelope preparation is handled inside that observation tool"
	}
	if messageLoopIsMixObservationTool(call) || messageLoopMixObserveFirstAllowedTool(call) {
		return ""
	}
	if !messageLoopMixObserveFirstBlockedTool(call) {
		return ""
	}
	if messageLoopPrimitiveTrackVolumeCall(call) || messageLoopPrimitiveTrackPanCall(call) {
		if hadMixObservationBeforeTurn || messageLoopHasUsableMixObservation(state) {
			return "mix.observe is complete; direct track volume or pan writes must be converted into a pending mix treatment/mix tick, then wait for explicit user confirmation before applying"
		}
		return "ordinary acoustic mixing requests must run mix.observe and wait for its result before changing volume or pan"
	}
	if hadMixObservationBeforeTurn {
		if messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
			return ""
		}
		return "mix.observe is complete; broad mixing requests must stop here, summarize the observation, propose one concrete next move, and wait for explicit user confirmation before loading plugins, learning profiles, changing volume, applying controls, or writing parameters"
	}
	return "ordinary acoustic mixing requests must run mix.observe and wait for its result before loading plugins, learning plugin profiles, changing volume, applying controls, or writing parameters"
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

func messageLoopHasAnyMixObservationAttempt(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		if messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			return true
		}
	}
	for _, event := range state.trace {
		if event.ToolResult != nil && messageLoopIsMixObservationName(event.ToolResult.Tool) {
			return true
		}
		if event.ToolCall != nil && messageLoopIsMixObservationName(event.ToolCall.Tool) {
			return true
		}
	}
	if observationIsMixObservation(state.recentObservation) {
		return true
	}
	return false
}

func messageLoopHasMixObservationExecution(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		if messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			return true
		}
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
	hasReadyAcousticEvidence := messageLoopMixObservationHasReadyAcousticEvidence(result)
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
		if blockers := messageLoopFatalObservationBlockers(messageLoopStringList(row["open_blockers"]), hasReadyAcousticEvidence); len(blockers) > 0 {
			return false
		}
		if blockers := messageLoopFatalObservationBlockers(messageLoopStringList(row["blockers"]), hasReadyAcousticEvidence); len(blockers) > 0 {
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

func messageLoopMixObservationHasReadyAcousticEvidence(result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	for _, key := range []string{
		"waveform_envelope",
		"track_waveform_envelopes",
		"spectrogram_tiles",
		"band_energy",
		"stereo_relation",
		"realtime_band_energy",
		"realtime_stereo_relation",
	} {
		if messageLoopCapabilityStatusReady(messageLoopObservationFallbackCapabilityStatus(nil, result, key)) {
			return true
		}
	}
	for _, row := range []map[string]any{
		result,
		messageLoopMapValue(result["digest"]),
		messageLoopMapValue(result["acoustic_digest"]),
		messageLoopObservationFromResult(result),
	} {
		if messageLoopObservationRowHasReadyAcousticEvidence(row) {
			return true
		}
	}
	return false
}

func messageLoopObservationRowHasReadyAcousticEvidence(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	if messageLoopAnyFeatureReady(row, "band_energy_summary", "stereo_relation_summary", "realtime_band_energy_summary", "realtime_stereo_relation_summary", "spectrogram_tiles", "waveform", "waveform_envelope", "track_waveform_envelopes") {
		return true
	}
	if messageLoopAnyFeatureReady(messageLoopMapValue(row["global_summary"]), "band_energy_summary", "stereo_relation_summary", "realtime_band_energy_summary", "realtime_stereo_relation_summary", "spectrogram_tiles", "waveform", "waveform_envelope", "track_waveform_envelopes") {
		return true
	}
	if messageLoopAnyFeatureReady(messageLoopMapValue(row["available_detail"]), "band_energy", "stereo_relation", "realtime_band_energy", "realtime_stereo_relation", "spectrogram_tiles", "waveform_envelope") {
		return true
	}
	mixPackage := messageLoopMapValue(row["mix_package"])
	if messageLoopAnyFeatureReady(mixPackage, "band_energy", "stereo_relation", "realtime_band_energy", "realtime_stereo_relation") {
		return true
	}
	if messageLoopAnyFeatureReady(messageLoopMapValue(mixPackage["current_metrics"]), "waveform", "band_energy", "stereo_relation") {
		return true
	}
	if messageLoopAnyFeatureReady(messageLoopMapValue(mixPackage["realtime_metrics"]), "band_energy", "stereo_relation") {
		return true
	}
	return messageLoopAcousticPackageHasReadyEvidence(messageLoopMapValue(row["acoustic_package_status"]))
}

func messageLoopAnyFeatureReady(row map[string]any, keys ...string) bool {
	for _, key := range keys {
		if messageLoopFeatureReady(row[key]) {
			return true
		}
	}
	return false
}

func messageLoopFeatureReady(value any) bool {
	row := messageLoopMapValue(value)
	if len(row) > 0 {
		if messageLoopCapabilityStatusReady(messageLoopText(row["status"])) {
			return true
		}
		ref := messageLoopMapValue(row["ref"])
		if messageLoopCapabilityStatusReady(messageLoopText(ref["status"])) {
			return true
		}
	}
	return messageLoopCapabilityStatusReady(messageLoopText(value))
}

func messageLoopAcousticPackageHasReadyEvidence(status map[string]any) bool {
	if len(status) == 0 {
		return false
	}
	if !messageLoopCapabilityStatusReady(messageLoopText(status["status"])) {
		return false
	}
	layers := messageLoopMapValue(status["package_layers"])
	for _, layerName := range []string{"l1_static", "l2_realtime", "l3_deep"} {
		layer := messageLoopMapValue(layers[layerName])
		if !messageLoopCapabilityStatusReady(messageLoopText(layer["status"])) {
			continue
		}
		features := messageLoopMapValue(layer["features"])
		if messageLoopAnyFeatureReady(features,
			"waveform_envelope",
			"peak_rms_summary",
			"time_energy",
			"live_meter",
			"realtime_spectrum",
			"realtime_stereo_correlation",
			"spectrogram_tiles",
			"band_energy_summary",
			"stereo_relation_summary",
		) {
			return true
		}
	}
	return false
}

func messageLoopFatalObservationBlockers(blockers []string, hasReadyAcousticEvidence bool) []string {
	out := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		switch strings.ToLower(strings.TrimSpace(blocker)) {
		case "", "audio_feature_request_pending":
			continue
		case "audio_feature_reader_not_connected":
			if hasReadyAcousticEvidence {
				continue
			}
			out = append(out, blocker)
		default:
			out = append(out, blocker)
		}
	}
	return out
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
	if messageLoopTextHasAny(text, "\u5de6", "\u53f3", "\u5c45\u4e2d", "\u56de\u4e2d", "\u4e2d\u95f4", "left", "right", "center", "centre") &&
		messageLoopTextHasAny(text, "\u58f0\u50cf", "\u58f0\u76f8", "\u8f68\u9053", "\u5409\u4ed6", "\u8d1d\u65af", "\u9f13", "\u4e3b\u5531", "\u4eba\u58f0", "pan", "panning", "track", "guitar", "bass", "drum", "vocal", "voice") {
		return true
	}
	return messageLoopTextHasAny(text,
		"\u6df7\u97f3", "\u6df7\u4e00\u4e0b", "\u5e2e\u6211\u6df7", "\u7f29\u6df7", "\u58f0\u97f3\u5904\u7406", "\u8c03\u4e00\u4e0b", "\u5904\u7406\u4e00\u4e0b",
		"\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal",
		"\u9760\u524d", "\u5f80\u524d", "\u63d0\u5347\u54cd\u5ea6", "\u54cd\u5ea6", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca", "\u66f4\u4eae", "\u660e\u4eae", "\u6d51\u6d4a", "\u523a\u8033",
		"\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u7a7a\u95f4\u611f", "\u52a0\u4e00\u70b9\u7a7a\u95f4", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u52a8\u6001", "\u538b\u7f29",
		"mix", "mixing", "loudness", "louder", "too loud", "too quiet", "volume", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "forward", "mud", "muddy", "harsh", "bright", "space", "reverb", "pan", "panning", "stereo field", "dynamic",
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

func messageLoopRealtimeObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasRealtimeLayer := messageLoopTextHasAny(text,
		"l2", "realtime", "real-time", "real time", "live", "playback", "after playback", "during playback", "post playback",
		"\u5b9e\u65f6", "\u64ad\u653e\u540e", "\u64ad\u653e\u65f6", "\u64ad\u653e\u4e2d", "\u542c\u4e86", "\u653e\u5b8c", "\u8fb9\u64ad\u8fb9", "\u5b9e\u65f6\u5c42",
	)
	hasAcousticTarget := messageLoopTextHasAny(text,
		"\u7535\u5e73", "\u9891\u8c31", "\u9891\u6bb5", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u7acb\u4f53\u58f0", "\u76f8\u4f4d", "\u76f8\u5173", "\u91c7\u96c6", "\u89c2\u5bdf", "\u68c0\u67e5", "\u5206\u6790",
		"meter", "level", "spectrum", "spectral", "band", "stereo", "phase", "correlation", "capture", "observe", "inspect", "analysis",
	)
	return hasRealtimeLayer && hasAcousticTarget
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
		"\u786e\u8ba4", "\u6267\u884c", "\u5f00\u59cb", "\u6309\u4f60\u8bf4\u7684", "\u6309\u8fd9\u4e2a", "\u5c31\u8fd9\u6837", "\u53ef\u4ee5\u6267\u884c", "\u53ef\u4ee5\u7ee7\u7eed", "\u7ee7\u7eed\u6267\u884c",
		"confirm", "execute", "apply", "do it", "go ahead", "as you said",
	)
	hasConcreteMove := messageLoopTextHasAny(text,
		"db", "hz", "khz", "%", "percent", "\u5206\u8d1d", "\u8d6b\u5179",
		"\u63d0\u5347", "\u964d\u4f4e", "\u589e\u52a0", "\u51cf\u5c11", "\u58f0\u50cf", "\u58f0\u76f8", "\u5de6", "\u53f3", "\u538b", "\u524a", "\u5207", "\u52a0", "\u51cf", "\u4e0d\u8981\u52a8\u97f3\u91cf",
		"boost", "cut", "raise", "lower", "reduce", "compress", "volume", "gain", "pan", "panning", "left", "right", "center", "reverb", "eq",
	)
	return hasApproval && hasConcreteMove
}

func messageLoopPreferredMixObservationTool(state *runState) string {
	if state != nil && allowedTool("mix.observe", state.input.AllowedTools) {
		return "mix.observe"
	}
	return "mix.request_observation"
}

func messageLoopMixObservationArgs(userText string, args map[string]any) map[string]any {
	out := cloneMap(args)
	if out == nil {
		out = map[string]any{}
	}
	if index, ok := messageLoopUserTrackIndexFromText(userText); ok {
		if firstMapText(out, "track_id", "selected_track_id", "target_track_id") == "" {
			out["track_id"] = fmt.Sprintf("Track %d", index)
		}
		if _, exists := out["user_track_index"]; !exists {
			out["user_track_index"] = index
		}
	}
	setIfEmpty(out, "goal_text", strings.TrimSpace(userText))
	scope := normalizeMessageLoopMixScope(fmt.Sprint(out["scope"]))
	inferredScope := normalizeMessageLoopMixScope(messageLoopMixScopeFromIntent(userText, out))
	if scope == "" {
		scope = inferredScope
	} else if expanded := messageLoopExpandedMixObservationScope(scope, inferredScope); expanded != "" {
		scope = expanded
	}
	out["scope"] = scope
	if _, ok := out["project_context"]; !ok {
		out["project_context"] = scope != "selected_clip"
	}
	if _, ok := out["observation_only"]; !ok {
		out["observation_only"] = true
	}
	if messageLoopReadOnlyObservationRequest(userText) {
		out["workflow_intent"] = "observation_only"
		out["mutation_barrier"] = true
		out["no_pending"] = true
		out["reason"] = "user_requested_read_only_observation"
	}
	if strings.TrimSpace(fmt.Sprint(out["disclosure"])) == "" || strings.TrimSpace(fmt.Sprint(out["disclosure"])) == "<nil>" {
		out["disclosure"] = "digest_catalog"
	}
	if messageLoopBandStereoObservationRequest(userText) {
		out["projection"] = "frequency_stereo"
		out["include_raw"] = false
		out["observation_ready_gate"] = true
		out["feature_keys"] = []any{"band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"}
		if _, ok := out["max_rows"]; !ok {
			out["max_rows"] = 8
		}
		out["target_scope"] = scope
	}
	if messageLoopProjectMultitrackObservationRequest(userText) {
		setIfEmpty(out, "mom_intent", "project_multitrack_relation_observation")
		setIfEmpty(out, "requested_layer", "project_multitrack_relation")
		out["target_scope"] = scope
	}
	if messageLoopRealtimeObservationRequest(userText) {
		out["projection"] = "frequency_stereo"
		out["include_raw"] = false
		out["capture_mode"] = "realtime_playback"
		out["requested_layer"] = "l2_realtime"
		out["prefer_realtime"] = true
		out["feature_keys"] = []any{"realtime_band_energy_summary", "realtime_stereo_relation_summary", "band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"}
		if _, ok := out["max_rows"]; !ok {
			out["max_rows"] = 8
		}
		out["target_scope"] = scope
	}
	if messageLoopLowMudPluginPrepRequest(userText) {
		setIfEmpty(out, "mom_intent", "action_preflight_observation")
		out["observation_ready_gate"] = true
	}
	if messageLoopTextHasAny(strings.ToLower(userText), "\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal") {
		focusHint := map[string]any{"role": "vocal", "source": "user_intent"}
		if index, ok := messageLoopUserTrackIndexFromText(userText); ok {
			focusHint["user_track_index"] = index
			focusHint["source"] = "user_clarification"
		}
		setIfEmptyMap(out, "focus_hint", focusHint)
	}
	if messageLoopTextHasAny(strings.ToLower(userText), "\u4f4e\u9891", "\u8d1d\u65af", "\u9f13", "\u5e95\u9f13", "\u6d51\u6d4a", "low end", "bass", "kick", "mud", "muddy") {
		setIfEmptyMap(out, "relationship_focus", map[string]any{
			"type":       "low_frequency_relationship_focus",
			"dimensions": []any{"sub", "bass", "low_mid", "time_overlap", "headroom"},
			"source":     "user_intent",
		})
	}
	return out
}

func normalizeMessageLoopMixScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "<nil>":
		return ""
	case "track":
		return "selected_track"
	case "clip":
		return "selected_clip"
	case "project":
		return "full_project"
	default:
		return strings.ToLower(strings.TrimSpace(scope))
	}
}

func messageLoopExpandedMixObservationScope(current, inferred string) string {
	switch inferred {
	case "full_project_with_focus_track":
		if current != "full_project_with_focus_track" {
			return inferred
		}
	case "full_project":
		switch current {
		case "selected_clip", "selected_track", "named_track", "track_group":
			return inferred
		}
	}
	return ""
}

func messageLoopBandStereoObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasBand := messageLoopTextHasAny(text,
		"\u9891\u6bb5", "\u9891\u8c31", "\u4f4e\u9891", "\u4e2d\u9891", "\u9ad8\u9891", "\u9891\u6bb5\u80fd\u91cf",
		"frequency", "spectrum", "spectral", "band energy", "low end", "midrange", "high end",
	)
	hasStereo := messageLoopTextHasAny(text,
		"\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u7acb\u4f53\u58f0", "\u76f8\u4f4d",
		"stereo", "stereo image", "stereo field", "panning", "phase", "correlation",
	)
	return hasBand && hasStereo
}

func messageLoopProjectMultitrackObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"整体混音", "全工程", "多轨", "各轨", "各个轨", "各条轨", "各轨道", "轨道关系", "轨道对比", "频段占用", "频段分布", "声像关系", "声像布局", "冲突",
		"overall mix", "full project", "multitrack", "multi-track", "track relationship", "track relationships", "track conflict", "track conflicts", "band distribution", "stereo layout",
	)
}

func messageLoopUserTrackIndexFromText(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\btrack\s*([1-9]\d*)\b`),
		regexp.MustCompile(`(?i)\btrk\s*([1-9]\d*)\b`),
		regexp.MustCompile(`\b([1-9]\d*)\s*(?:号|號)?\s*(?:轨|軌|轨道|軌道)\b`),
		regexp.MustCompile(`(?:第)\s*([1-9]\d*)\s*(?:轨|軌|轨道|軌道)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(text)
		if len(match) < 2 {
			continue
		}
		index, err := strconv.Atoi(strings.TrimSpace(match[1]))
		if err == nil && index > 0 {
			return index, true
		}
	}
	return 0, false
}

func messageLoopMixScopeFromIntent(userText string, args map[string]any) string {
	text := strings.ToLower(strings.TrimSpace(userText))
	if messageLoopTextHasAny(text, "\u6574\u4f53", "\u6574\u9996", "\u5168\u5de5\u7a0b", "\u8fd9\u9996\u6b4c", "\u5168\u5c40", "overall", "whole song", "full project", "entire mix") {
		if messageLoopTextHasAny(text, "\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal") {
			return "full_project_with_focus_track"
		}
		return "full_project"
	}
	if messageLoopProjectMultitrackObservationRequest(userText) {
		return "full_project"
	}
	if messageLoopTextHasAny(text, "\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal") {
		return "full_project_with_focus_track"
	}
	if messageLoopTextHasAny(text, "\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u6d51\u6d4a", "\u8d1d\u65af", "\u5e95\u9f13", "low end", "bass", "kick", "mud", "muddy") {
		return "full_project"
	}
	if firstMapText(args, "clip_id", "selected_clip_id") != "" || messageLoopTextHasAny(text, "clip", "\u7247\u6bb5") {
		return "selected_clip"
	}
	if firstMapText(args, "track_name", "target_track_name", "name") != "" {
		return "named_track"
	}
	if firstMapText(args, "track_id", "selected_track_id", "target_track_id") != "" || messageLoopTextHasAny(text, "\u5f53\u524d\u8f68\u9053", "\u9009\u4e2d\u8f68\u9053", "current track", "selected track") {
		return "selected_track"
	}
	return "selected_track"
}

func setIfEmptyMap(row map[string]any, key string, value map[string]any) {
	if row == nil || key == "" || len(value) == 0 {
		return
	}
	if current, ok := row[key]; ok && !messageLoopEmptyValue(current) {
		return
	}
	row[key] = value
}

func messageLoopIsMixObservationTool(call planner.ToolCall) bool {
	return messageLoopIsMixObservationName(normalizedActionName(call, executorpkg.Result{}))
}

func messageLoopIsMixObservationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation":
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
	case "project.state", "get_project_state", "track.list", "mix.read", "mix_read", "mix.derive", "mix_derive",
		"mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "mix.rollback_tick", "mix_rollback_tick":
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
		"daw.invoke", "daw_invoke",
		"track.volume", "set_volume", "track.pan", "track_pan", "set_pan",
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
		momProjection := messageLoopMOMProjectionForPrompt(observation)
		if len(momProjection) > 0 {
			out["mom_projection"] = momProjection
		}
		if llmContext := messageLoopMapValue(observation["llm_context"]); len(llmContext) > 0 {
			out["llm_context"] = llmContext
		}
		if target := messageLoopMapValue(observation["target_ref"]); len(target) > 0 {
			out["target_ref"] = compactSelectedKeys(target, []string{"kind", "id", "label", "confidence"})
		}
		if ruler := messageLoopMapValue(observation["time_ruler"]); len(ruler) > 0 {
			out["time_ruler"] = compactSelectedKeys(ruler, []string{"duration_seconds", "segment_seconds", "frame_seconds"})
		}
		if global := messageLoopMapValue(observation["global_summary"]); len(global) > 0 && len(momProjection) == 0 {
			out["global_summary"] = compactSelectedKeys(global, []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "dominant_problem_tags"})
		}
		if mixPkg := messageLoopMapValue(observation["mix_package"]); len(mixPkg) > 0 && len(momProjection) == 0 {
			out["mix_package"] = compactMixPackageForPrompt(mixPkg, "")
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

func messageLoopMOMProjectionForPrompt(observation map[string]any) map[string]any {
	proj := messageLoopMapValue(observation["mom_projection"])
	if len(proj) == 0 {
		if ctx := messageLoopMapValue(observation["context_pack"]); len(ctx) > 0 {
			latest := messageLoopMapValue(ctx["latest_observation"])
			proj = messageLoopMapValue(latest["mom_projection"])
		}
	}
	if len(proj) == 0 {
		return nil
	}
	out := compactSelectedKeys(proj, []string{"mom_version", "intent", "observation_id", "mix_session_id", "llm_context"})
	trust := messageLoopMapValue(proj["trust_quality"])
	if project := messageLoopMOMProjectStructureForPrompt(messageLoopMapValue(proj["project_structure"]), trust); len(project) > 0 {
		out["project_structure"] = project
	}
	if safeTrust := messageLoopMOMTrustQualityForPrompt(trust); len(safeTrust) > 0 {
		out["trust_quality"] = safeTrust
	}
	if profile := compactMOMProjectMixProfile(messageLoopMapValue(proj["project_mix_profile"])); len(profile) > 0 {
		out["project_mix_profile"] = profile
	}
	if relation := compactMOMMultitrackRelation(messageLoopMapValue(proj["multitrack_relation"])); len(relation) > 0 {
		out["multitrack_relation"] = relation
	}
	return out
}

func messageLoopMOMProjectStructureForPrompt(project, trust map[string]any) map[string]any {
	if len(project) == 0 {
		return nil
	}
	out := compactSelectedKeys(project, []string{
		"status", "freshness", "project_id", "session_id", "track_id", "clip_id", "gui_id",
		"duration_seconds", "sample_rate", "channel_count", "target_ref", "listen_scope",
		"evidence_refs", "limitations",
	})
	if status := firstMapText(trust, "source_revision_status"); status != "" {
		out["source_revision_status"] = status
	}
	if status := firstMapText(trust, "clip_revision_status"); status != "" {
		out["clip_revision_status"] = status
	}
	if status := firstMapText(trust, "render_revision_status"); status != "" {
		out["render_revision_status"] = status
	}
	return out
}

func messageLoopMOMTrustQualityForPrompt(trust map[string]any) map[string]any {
	if len(trust) == 0 {
		return nil
	}
	return compactSelectedKeys(trust, []string{
		"schema_version", "overall_status", "required_layers", "optional_layers", "deferred_layers",
		"coverage", "quality_gates", "blocked_reasons", "approximate_fields", "suspect_fields",
		"stale_fields", "missing_fields", "source_revision_status", "clip_revision_status",
		"render_revision_status", "can_support_observation", "can_support_suggestion",
		"can_support_action_preflight", "can_support_ab_result", "freshness", "l2_tap_point",
		"l2_limitations", "limitations", "evidence_refs",
	})
}

func compactMOMProjectMixProfile(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := compactSelectedKeys(row, []string{"status", "freshness", "track_count", "band_tendency", "level_overview", "stereo_overview", "limitations", "evidence_refs"})
	if rows := messageLoopMapRows(row["dominant_bands"]); len(rows) > 0 {
		out["dominant_bands"] = capMessageLoopRows(rows, 8)
	}
	return out
}

func compactMOMMultitrackRelation(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := compactSelectedKeys(row, []string{"status", "freshness", "track_count", "level_distribution", "stereo_distribution", "limitations", "evidence_refs"})
	if rows := compactMOMComparedTracks(messageLoopMapRows(row["compared_tracks"]), 8); len(rows) > 0 {
		out["compared_tracks"] = rows
	}
	if rows := compactMOMBandOccupancyRows(messageLoopMapRows(row["band_occupancy"]), 8); len(rows) > 0 {
		out["band_occupancy"] = rows
	}
	if rows := messageLoopMapRows(row["band_conflict_candidates"]); len(rows) > 0 {
		out["band_conflict_candidates"] = capMessageLoopRows(rows, 8)
	}
	if rows := messageLoopMapRows(row["phase_risk_tracks"]); len(rows) > 0 {
		out["phase_risk_tracks"] = capMessageLoopRows(rows, 8)
	}
	return out
}

func compactMOMComparedTracks(rows []map[string]any, max int) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	if max > 0 && len(rows) > max {
		rows = rows[:max]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compact := compactSelectedKeys(row, []string{"track_id", "track_name", "name", "user_label", "role_guess", "peak_dbfs", "rms_dbfs", "headroom_db", "level_db", "pan"})
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func compactMOMBandOccupancyRows(rows []map[string]any, max int) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	if max > 0 && len(rows) > max {
		rows = rows[:max]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		compact := compactSelectedKeys(row, []string{"band", "track_count", "status", "conflict_candidate"})
		if leaders := compactMOMComparedTracks(messageLoopMapRows(row["leaders"]), 4); len(leaders) > 0 {
			for i := range leaders {
				source := messageLoopMapRows(row["leaders"])
				if i < len(source) {
					for _, key := range []string{"energy_db", "unit_energy"} {
						if value, ok := source[i][key]; ok {
							leaders[i][key] = value
						}
					}
				}
			}
			compact["leaders"] = leaders
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
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
	if status := messageLoopMapValue(digest["acoustic_package_status"]); len(status) > 0 {
		out["acoustic_package_status"] = status
	}
	return out
}

func compactMixPackageForPrompt(mixPkg map[string]any, intent string) map[string]any {
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
	if intent == "realtime_band_stereo_observation" {
		realtimeMetrics := messageLoopMapValue(mixPkg["realtime_metrics"])
		realtime := map[string]any{}
		if band := messageLoopMapValue(realtimeMetrics["band_energy"]); len(band) > 0 {
			realtime["band_energy"] = compactSelectedKeys(band, []string{"status", "source", "bands", "updated_at", "capture_mode", "tap_point"})
		}
		if stereo := messageLoopMapValue(realtimeMetrics["stereo_relation"]); len(stereo) > 0 {
			realtime["stereo_relation"] = compactSelectedKeys(stereo, []string{"status", "source", "balance_db", "balance_state", "correlation_estimate", "correlation_state", "left_level_db", "right_level_db", "updated_at", "capture_mode", "tap_point"})
		}
		if len(realtime) > 0 {
			out["realtime_metrics"] = realtime
		}
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

func messageLoopObservationFallbackAfterLLMError(state *runState, err error) (string, bool, bool) {
	if !messageLoopObservationFallbackEligible(state, err) {
		return "", false, false
	}
	if messageLoopFocusRelationshipIntent(state.input.UserText) && !messageLoopHasResolvedFocusTrack(state) {
		return messageLoopFocusTrackClarificationQuestion(), true, true
	}
	reply := messageLoopMaterializedObservationFallbackReply(state)
	if strings.TrimSpace(reply) == "" {
		return "", false, false
	}
	if question, ok := messageLoopFinalFocusTrackClarification(state, reply); ok {
		return question, true, true
	}
	return messageLoopMixObservationFinalReply(state, reply), false, true
}

func messageLoopObservationFallbackEligible(state *runState, err error) bool {
	if state == nil || err == nil {
		return false
	}
	if !messageLoopTransientLLMError(err) {
		return false
	}
	if !messageLoopHasMixObservationExecution(state) || !messageLoopHasUsableMixObservation(state) {
		return false
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopAudioObservationRequest(state.input.UserText) {
		return false
	}
	if messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
		return false
	}
	for _, record := range state.executed {
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if messageLoopReadOnlyObservationFallbackTool(name) {
			continue
		}
		return false
	}
	return true
}

func messageLoopTransientLLMError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"context deadline exceeded", "timeout", "timed out", "awaiting headers",
		"502", "503", "504", "temporary", "stream returned no output", "connection reset",
	)
}

func messageLoopReadOnlyObservationFallbackTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return true
	}
	if messageLoopIsMixObservationName(name) {
		return true
	}
	if strings.Contains(name, ".read") || strings.Contains(name, "_read") || strings.Contains(name, ".list") || strings.Contains(name, "_list") {
		return true
	}
	switch name {
	case "mix.derive", "mix_derive", "project.state", "get_project_state", "track.list", "goal.status":
		return true
	default:
		return false
	}
}

func messageLoopMaterializedObservationFallbackReply(state *runState) string {
	result := messageLoopLastMixObservationResult(state)
	if len(result) == 0 {
		return ""
	}
	if reply := messageLoopMOMObservationFallbackReply(result); strings.TrimSpace(reply) != "" {
		return reply
	}
	lines := []string{
		"混音观察已经完成；最终自然语言生成超时，所以我先基于已物化的 observation read model 给出保守摘要。",
	}
	if trackCount := messageLoopPendingMixCandidateTrackCount(state); trackCount > 0 {
		lines = append(lines, fmt.Sprintf("当前工程可读到 %d 条音频轨。", trackCount))
	}
	if waveform := messageLoopObservationFallbackWaveform(result); len(waveform) > 0 {
		if summary := messageLoopObservationMetricSummary("整体电平", waveform); summary != "" {
			lines = append(lines, summary)
		}
	}
	if trackID, risk := messageLoopHeadroomRiskTrack(state); trackID != "" {
		label := messageLoopObservationFallbackTrackLabel(trackID, risk)
		summary := "首要注意对象是 " + label
		if metrics := messageLoopObservationMetricSuffix(risk); metrics != "" {
			summary += metrics
		}
		summary += "。"
		lines = append(lines, summary)
		if delta := messageLoopConservativeHeadroomDelta(risk); delta < 0 {
			lines = append(lines, fmt.Sprintf("建议第一步：先把 %s 降低 %.2f dB，释放一点峰值余量；这只是可回退的小步电平整理，不会直接写入，除非你确认。要我继续执行这一步吗？", label, -delta))
		}
	}
	if ready, deferred := messageLoopObservationFallbackCapabilities(state, result); len(ready) > 0 || len(deferred) > 0 {
		if len(ready) > 0 {
			lines = append(lines, "已就绪的声学投影："+strings.Join(ready, "、")+"。")
		}
		if len(deferred) > 0 {
			lines = append(lines, "仍处于 deferred / O1-O2 边界内的分析："+strings.Join(deferred, "、")+"。")
		}
	}
	lines = append(lines, "这条兜底路径只读取现有 observation surface，不触发新的 bake，也不加载插件或写参数。")
	return strings.Join(lines, "\n\n")
}

func messageLoopMOMObservationFallbackReply(result map[string]any) string {
	proj := messageLoopMOMProjectionFromResult(result)
	if len(proj) == 0 {
		return ""
	}
	profile := messageLoopMapValue(proj["project_mix_profile"])
	relation := messageLoopMapValue(proj["multitrack_relation"])
	if len(profile) == 0 && len(relation) == 0 {
		return ""
	}
	version := firstNonEmpty(firstMapText(proj, "mom_version"), "MOM")
	lines := []string{
		fmt.Sprintf("混音观察已经完成；最终自然语言生成暂时失败，所以我先基于 %s 只读投影给出保守摘要。", version),
	}
	trackCount := 0
	if value, ok := firstNumericMapValue(relation, "track_count"); ok {
		trackCount = int(value)
	} else if value, ok := firstNumericMapValue(profile, "track_count"); ok {
		trackCount = int(value)
	}
	relationStatus := messageLoopMOMStatusLabel(firstNonEmpty(firstMapText(relation, "status"), firstMapText(profile, "status")))
	if trackCount > 0 {
		if relationStatus != "" {
			lines = append(lines, fmt.Sprintf("当前可比较 %d 条轨，多轨关系状态：%s。", trackCount, relationStatus))
		} else {
			lines = append(lines, fmt.Sprintf("当前可比较 %d 条轨。", trackCount))
		}
	} else if status := firstMapText(relation, "status"); status == "not_applicable_single_track" {
		lines = append(lines, "当前工程暂不适用多轨关系对比；我只保留单轨观察结论。")
	}
	if summary := messageLoopMOMLevelFallbackSummary(firstNonEmptyMap(relation, profile, "level_distribution", "level_overview")); summary != "" {
		lines = append(lines, summary)
	}
	if summary := messageLoopMOMBandFallbackSummary(relation, profile); summary != "" {
		lines = append(lines, summary)
	}
	if summary := messageLoopMOMStereoFallbackSummary(firstNonEmptyMap(relation, profile, "stereo_distribution", "stereo_overview")); summary != "" {
		lines = append(lines, summary)
	}
	if limitations := messageLoopMOMFallbackLimitations(profile, relation); len(limitations) > 0 {
		lines = append(lines, "限制： "+strings.Join(limitations, "；")+"。")
	}
	lines = append(lines, "这条兜底路径只读取现有 MOM projection 和 evidence refs，不触发新的 bake，不加载插件，也不写入工程。")
	return strings.Join(lines, "\n\n")
}

func messageLoopMOMProjectionFromResult(result map[string]any) map[string]any {
	for _, row := range []map[string]any{
		messageLoopMapValue(result["mom_projection"]),
		messageLoopMapValue(messageLoopMapValue(result["observation"])["mom_projection"]),
		messageLoopMapValue(messageLoopMapValue(messageLoopMapValue(result["context_pack"])["latest_observation"])["mom_projection"]),
	} {
		if len(row) > 0 {
			return row
		}
	}
	observation := messageLoopObservationFromResult(result)
	for _, row := range []map[string]any{
		messageLoopMapValue(observation["mom_projection"]),
		messageLoopMapValue(messageLoopMapValue(observation["latest_observation"])["mom_projection"]),
	} {
		if len(row) > 0 {
			return row
		}
	}
	return nil
}

func messageLoopMOMLevelFallbackSummary(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	parts := []string{}
	if loud := messageLoopMapValue(row["loudest_by_rms"]); len(loud) > 0 {
		parts = append(parts, fmt.Sprintf("RMS 最突出的轨道是 %s%s", messageLoopMOMTrackLabel(loud), messageLoopMOMMetricSuffix(loud, "value", "dBFS")))
	}
	if peak := messageLoopMapValue(row["highest_peak"]); len(peak) > 0 {
		parts = append(parts, fmt.Sprintf("峰值最高的是 %s%s", messageLoopMOMTrackLabel(peak), messageLoopMOMMetricSuffix(peak, "value", "dBFS")))
	}
	if headroom := messageLoopMapValue(row["lowest_headroom"]); len(headroom) > 0 {
		parts = append(parts, fmt.Sprintf("余量最低的是 %s%s", messageLoopMOMTrackLabel(headroom), messageLoopMOMMetricSuffix(headroom, "value", "dB")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "电平分布：" + strings.Join(parts, "；") + "。"
}

func messageLoopMOMBandFallbackSummary(relation, profile map[string]any) string {
	rows := messageLoopMapRows(relation["band_occupancy"])
	if len(rows) == 0 {
		rows = messageLoopMapRows(profile["dominant_bands"])
	}
	if len(rows) == 0 {
		return ""
	}
	parts := []string{}
	for _, row := range rows {
		band := messageLoopMOMBandLabel(firstMapText(row, "band"))
		leader := messageLoopMapValue(row["leader"])
		if len(leader) == 0 {
			leaders := messageLoopMapRows(row["leaders"])
			if len(leaders) > 0 {
				leader = leaders[0]
			}
		}
		if band == "" || len(leader) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s：%s%s", band, messageLoopMOMTrackLabel(leader), messageLoopMOMMetricSuffix(leader, "energy_db", "dB")))
		if len(parts) >= 6 {
			break
		}
	}
	if len(parts) == 0 {
		return ""
	}
	conflicts := len(messageLoopMapRows(relation["band_conflict_candidates"]))
	if conflicts == 0 {
		return "频段占用：" + strings.Join(parts, "；") + "。当前没有明确频段冲突候选。"
	}
	return fmt.Sprintf("频段占用：%s。当前有 %d 个频段冲突候选，需要按 evidence refs 复核。", strings.Join(parts, "；"), conflicts)
}

func messageLoopMOMStereoFallbackSummary(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	center := len(messageLoopMapRows(row["center_heavy_tracks"]))
	offCenter := len(messageLoopMapRows(row["off_center_tracks"]))
	phaseRisk := len(messageLoopMapRows(row["phase_risk_tracks"]))
	parts := []string{}
	if center > 0 {
		parts = append(parts, fmt.Sprintf("%d 条轨偏居中", center))
	}
	if offCenter > 0 {
		parts = append(parts, fmt.Sprintf("%d 条轨明显偏左/右", offCenter))
	}
	if phaseRisk > 0 {
		parts = append(parts, fmt.Sprintf("%d 条轨有相位风险", phaseRisk))
	}
	if len(parts) == 0 {
		return ""
	}
	return "声像关系：" + strings.Join(parts, "；") + "。"
}

func messageLoopMOMFallbackLimitations(rows ...map[string]any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, row := range rows {
		for _, item := range messageLoopAnySlice(row["limitations"]) {
			label := messageLoopMOMLimitationLabel(messageLoopText(item))
			if label == "" || seen[label] {
				continue
			}
			seen[label] = true
			out = append(out, label)
			if len(out) >= 4 {
				return out
			}
		}
	}
	return out
}

func messageLoopMOMTrackLabel(row map[string]any) string {
	label := firstMapText(row, "user_label", "track_name", "name", "label")
	if label != "" {
		return label
	}
	if id := firstMapText(row, "track_id", "id"); id != "" {
		return "Track " + id
	}
	return "未知轨道"
}

func messageLoopMOMMetricSuffix(row map[string]any, key, unit string) string {
	value, ok := firstNumericMapValue(row, key)
	if !ok {
		return ""
	}
	return fmt.Sprintf(" %.1f %s", value, unit)
}

func messageLoopMOMBandLabel(band string) string {
	switch strings.ToLower(strings.TrimSpace(band)) {
	case "sub":
		return "超低频"
	case "bass":
		return "低频"
	case "low_mid", "low-mid", "low mid":
		return "低中频"
	case "mid":
		return "中频"
	case "presence":
		return "存在感频段"
	case "air":
		return "空气感频段"
	default:
		return strings.TrimSpace(band)
	}
}

func messageLoopMOMStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "ok", "fresh":
		return "就绪"
	case "partial":
		return "部分可用"
	case "missing":
		return "缺失"
	case "deferred":
		return "暂未展开"
	case "not_applicable_single_track":
		return "单轨工程不适用"
	default:
		return strings.TrimSpace(status)
	}
}

func messageLoopMOMLimitationLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "<nil>":
		return ""
	case "project_rankings_are_agent_side_lightweight_o1":
		return "多轨排序仍是轻量本地判断"
	case "kernel_project_contrast_analyzer_deferred_o3":
		return "内核级工程对比分析器尚未展开"
	case "lufs_analysis_deferred_phase_5":
		return "LUFS 分析暂未展开"
	case "masking_analysis_deferred_phase_5":
		return "遮蔽分析暂未展开"
	case "reference_match_deferred_phase_5":
		return "参考曲匹配暂未展开"
	case "post_fx_probe_unavailable_phase_4_1":
		return "后级/插件后探测暂不可用"
	default:
		return strings.ReplaceAll(strings.TrimSpace(value), "_", " ")
	}
}

func firstNonEmptyMap(primary, fallback map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if row := messageLoopMapValue(primary[key]); len(row) > 0 {
			return row
		}
		if row := messageLoopMapValue(fallback[key]); len(row) > 0 {
			return row
		}
	}
	return nil
}

func messageLoopObservationFallbackWaveform(result map[string]any) map[string]any {
	for _, row := range []map[string]any{
		messageLoopMapValue(messageLoopMapValue(result["acoustic_digest"])["waveform"]),
		messageLoopMapValue(messageLoopMapValue(result["digest"])["waveform"]),
		messageLoopMapValue(result["waveform"]),
	} {
		if len(row) > 0 {
			return row
		}
	}
	observation := messageLoopObservationFromResult(result)
	for _, row := range []map[string]any{
		messageLoopMapValue(messageLoopMapValue(messageLoopMapValue(observation["mix_package"])["current_metrics"])["waveform"]),
		messageLoopMapValue(messageLoopMapValue(messageLoopMapValue(observation["environment_package"])["current_metrics"])["waveform"]),
	} {
		if len(row) > 0 {
			return row
		}
	}
	return nil
}

func messageLoopObservationMetricSummary(label string, row map[string]any) string {
	if strings.TrimSpace(label) == "" || len(row) == 0 {
		return ""
	}
	if suffix := messageLoopObservationMetricSuffix(row); suffix != "" {
		return label + suffix + "。"
	}
	return ""
}

func messageLoopObservationMetricSuffix(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	parts := []string{}
	if peak, ok := firstNumericMapValue(row, "peak_dbfs", "peak"); ok {
		parts = append(parts, fmt.Sprintf("峰值 %.1f dBFS", peak))
	}
	if rms, ok := firstNumericMapValue(row, "rms_dbfs", "rms"); ok {
		parts = append(parts, fmt.Sprintf("RMS %.1f dBFS", rms))
	}
	if headroom, ok := firstNumericMapValue(row, "headroom_db", "headroom"); ok {
		parts = append(parts, fmt.Sprintf("余量 %.1f dB", headroom))
	}
	if len(parts) == 0 {
		return ""
	}
	return "（" + strings.Join(parts, "，") + "）"
}

func messageLoopObservationFallbackTrackLabel(trackID string, row map[string]any) string {
	trackID = strings.TrimSpace(trackID)
	label := firstNonEmpty(firstMapText(row, "track_name", "name", "user_label", "label"), firstMapText(messageLoopMapValue(row["track"]), "track_name", "name", "user_label", "label"))
	if strings.TrimSpace(label) != "" {
		return label
	}
	if trackID != "" {
		return "Track " + trackID
	}
	return "当前目标"
}

func messageLoopObservationFallbackCapabilities(state *runState, result map[string]any) ([]string, []string) {
	readySpecs := []struct {
		key   string
		label string
	}{
		{"waveform_envelope", "波形/峰值包络"},
		{"track_waveform_envelopes", "轨道波形包络"},
		{"spectrogram_tiles", "频谱覆盖"},
		{"band_energy", "频段能量摘要"},
		{"stereo_relation", "立体声关系摘要"},
		{"realtime_band_energy", "L2 实时频段能量"},
		{"realtime_stereo_relation", "L2 实时立体声关系"},
	}
	deferredSpecs := []struct {
		key   string
		label string
	}{
		{"lufs_analysis", "LUFS"},
		{"masking_analysis", "遮蔽分析"},
		{"reference_match", "参考匹配"},
	}
	ready := []string{}
	for _, spec := range readySpecs {
		if messageLoopCapabilityStatusReady(messageLoopObservationFallbackCapabilityStatus(state, result, spec.key)) {
			ready = append(ready, spec.label)
		}
	}
	deferred := []string{}
	for _, spec := range deferredSpecs {
		status := strings.ToLower(strings.TrimSpace(messageLoopObservationFallbackCapabilityStatus(state, result, spec.key)))
		if status == "deferred" || status == "phase_5_deferred" {
			deferred = append(deferred, spec.label)
		}
	}
	return ready, deferred
}

func messageLoopObservationFallbackCapabilityStatus(state *runState, result map[string]any, key string) string {
	keys := messageLoopCapabilityStatusKeys(key)
	for _, caps := range messageLoopObservationFallbackCapabilityMaps(state, result) {
		for _, candidate := range keys {
			if status := firstMapText(caps, candidate); status != "" {
				return status
			}
		}
	}
	return ""
}

func messageLoopCapabilityStatusKeys(key string) []string {
	switch strings.TrimSpace(key) {
	case "band_energy":
		return []string{"band_energy", "band_energy_summary"}
	case "stereo_relation":
		return []string{"stereo_relation", "stereo_relation_summary", "stereo_correlation"}
	case "realtime_band_energy":
		return []string{"realtime_band_energy", "realtime_band_energy_summary", "realtime_spectrum", "live_meter"}
	case "realtime_stereo_relation":
		return []string{"realtime_stereo_relation", "realtime_stereo_relation_summary", "realtime_stereo_correlation"}
	case "waveform_envelope":
		return []string{"waveform_envelope", "track_waveform_envelopes"}
	default:
		return []string{key}
	}
}

func messageLoopObservationFallbackCapabilityMaps(state *runState, result map[string]any) []map[string]any {
	var maps []map[string]any
	add := func(row map[string]any) {
		if len(row) > 0 {
			maps = append(maps, row)
		}
	}
	add(messageLoopMapValue(result["source_capabilities"]))
	add(messageLoopMapValue(messageLoopMapValue(result["digest"])["source_capabilities"]))
	add(messageLoopMapValue(messageLoopMapValue(result["acoustic_digest"])["source_capabilities"]))
	observation := messageLoopObservationFromResult(result)
	add(messageLoopMapValue(observation["source_capabilities"]))
	for _, key := range []string{"project_package", "mix_package", "deep_package", "environment_package"} {
		add(messageLoopMapValue(messageLoopMapValue(observation[key])["source_capabilities"]))
	}
	if state != nil && state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		summary := state.recentObservation.Summary
		add(messageLoopMapValue(summary["source_capabilities"]))
		add(messageLoopMapValue(messageLoopMapValue(summary["digest"])["source_capabilities"]))
		add(messageLoopMapValue(messageLoopMapValue(summary["acoustic_digest"])["source_capabilities"]))
	}
	return maps
}

func messageLoopCapabilityStatusReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "baseline_ready", "fresh", "available", "ok":
		return true
	default:
		return false
	}
}

func messageLoopMixObservationFinalReply(state *runState, reply string) string {
	reply = strings.TrimSpace(reply)
	if state == nil || reply == "" {
		return reply
	}
	if messageLoopMutationBarrierActive(state) {
		return messageLoopReadOnlyFinalReply(state, reply)
	}
	isLowMudPluginPrep := messageLoopLowMudPluginPrepRequest(state.input.UserText)
	isObservationFollowup := messageLoopMixObservationActionFollowupRequest(state)
	hasTreatmentMarkup := messageLoopMixTreatmentPendingMarkupPresent(reply)
	if !messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopImplicitPanFollowupRequest(state) && !isLowMudPluginPrep && !isObservationFollowup && !hasTreatmentMarkup {
		return reply
	}
	if messageLoopExplicitPluginOrRawRequest(state.input.UserText) && !isLowMudPluginPrep {
		return messageLoopStripMixTreatmentPendingMarkup(reply)
	}
	deepIncomplete := messageLoopLastMixObservationDeepPackageIncomplete(state) || messageLoopReplyMentionsIncompleteDeepPackage(reply)
	hasResolvedActionIntent := messageLoopResolvedMixActionIntent(state)
	if deepIncomplete && !isLowMudPluginPrep && !hasResolvedActionIntent {
		return messageLoopStripExecutionQuestion(messageLoopStripMixTreatmentPendingMarkup(reply))
	}
	preferTreatmentPending := hasTreatmentMarkup || isLowMudPluginPrep || messageLoopExplicitPanActionText(state.input.UserText)
	if preferTreatmentPending {
		if treatment := messageLoopMixTreatmentPendingFromReply(state, reply); treatment != nil {
			state.executionMemory.PendingMixTreatment = treatment
			reply = messageLoopStripMixTreatmentPendingMarkup(reply)
		} else if treatment := messageLoopConservativeLowMudTreatmentPendingFromReply(state, reply); treatment != nil {
			state.executionMemory.PendingMixTreatment = treatment
			reply = messageLoopConservativeLowMudTreatmentPendingReply(treatment)
		} else if hasTreatmentMarkup {
			reply = messageLoopStripMixTreatmentPendingMarkup(reply)
		}
	} else if candidate := messageLoopPendingMixTickCandidateFromReply(state, reply); candidate != nil {
		messageLoopAttachDiagnosisToMixTick(state, candidate, state.input.UserText)
		state.executionMemory.PendingMixTickCandidate = candidate
	} else if treatment := messageLoopMixTreatmentPendingFromReply(state, reply); treatment != nil {
		state.executionMemory.PendingMixTreatment = treatment
		reply = messageLoopStripMixTreatmentPendingMarkup(reply)
	} else if treatment := messageLoopConservativeLowMudTreatmentPendingFromReply(state, reply); treatment != nil {
		state.executionMemory.PendingMixTreatment = treatment
		reply = messageLoopConservativeLowMudTreatmentPendingReply(treatment)
	}
	if !messageLoopHasUsableMixObservation(state) {
		return reply
	}
	if deepIncomplete && !messageLoopHasPendingMixAction(state) {
		return messageLoopStripExecutionQuestion(reply)
	}
	if !messageLoopHasPendingMixAction(state) {
		if treatment := messageLoopConservativeHeadroomTreatmentPending(state, reply); treatment != nil {
			state.executionMemory.PendingMixTreatment = treatment
		}
	}
	if messageLoopMixReplyAsksForExecution(reply) {
		return reply
	}
	return reply + "\n\n如果你认可这个小步建议，需要我继续执行吗？"
}

func messageLoopMixObservationActionFollowupRequest(state *runState) bool {
	if state == nil || !messageLoopHasAnyMixObservationAttempt(state) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(state.input.UserText))
	if text == "" || messageLoopExplicitPluginOrRawRequest(text) {
		return false
	}
	return messageLoopTextHasAny(text,
		"\u600e\u4e48\u529e", "\u600e\u4e48\u5904\u7406", "\u600e\u4e48\u8c03", "\u8be5\u600e\u4e48", "\u4f60\u6253\u7b97", "\u6253\u7b97\u600e\u4e48",
		"\u4e0b\u4e00\u6b65", "\u63a5\u4e0b\u6765", "\u7136\u540e\u5462", "\u5efa\u8bae", "\u5904\u7406\u5efa\u8bae", "\u65b9\u6848",
		"\u53ef\u4ee5\u600e\u4e48", "\u8981\u600e\u4e48", "\u8be5\u4e0d\u8be5", "\u53ef\u4ee5\u5904\u7406", "\u53ef\u4ee5\u8fdb\u884c\u5904\u7406", "\u5f00\u59cb\u5904\u7406",
		"what should", "what next", "next step", "how should", "how would you", "suggest", "recommend", "proposal",
	)
}

func messageLoopHasPendingMixAction(state *runState) bool {
	if state == nil {
		return false
	}
	return state.executionMemory.PendingMixTickCandidate != nil || state.executionMemory.PendingMixTreatment != nil
}

func messageLoopResolvedMixActionIntent(state *runState) bool {
	if state == nil {
		return false
	}
	if messageLoopLowMudPluginPrepRequest(state.input.UserText) ||
		messageLoopFocusRelationshipIntent(state.input.UserText) ||
		messageLoopExplicitPanActionText(state.input.UserText) {
		return true
	}
	if delta, ok := messageLoopImplicitGainDeltaFromText(state.input.UserText); ok && delta != 0 && mathAbs(delta) <= 2 {
		return true
	}
	if messageLoopUserExplicitlyIdentifiesVocalTrackResolved(state.input.UserText) {
		return messageLoopConversationHasFocusRelationshipIntent(state)
	}
	return false
}

func messageLoopExplicitPanActionText(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasPanSubject := messageLoopTextHasAny(text, "声像", "声相", "声场", "pan", "panning", "stereo")
	hasPanDirection := messageLoopTextHasAny(text,
		"往左", "向左", "靠左", "偏左", "左一点", "左边",
		"往右", "向右", "靠右", "偏右", "右一点", "右边",
		"居中", "回中", "中间", "center", "centre", "left", "right",
	)
	return hasPanSubject && hasPanDirection
}

func messageLoopConversationHasFocusRelationshipIntent(state *runState) bool {
	if state == nil {
		return false
	}
	for i := len(state.input.Conversation) - 1; i >= 0; i-- {
		msg := state.input.Conversation[i]
		if !strings.EqualFold(strings.TrimSpace(msg.Role), "user") {
			continue
		}
		if strings.TrimSpace(msg.Content) == strings.TrimSpace(state.input.UserText) {
			continue
		}
		if messageLoopFocusRelationshipIntent(msg.Content) {
			return true
		}
	}
	return false
}

func messageLoopLastMixObservationDeepPackageIncomplete(state *runState) bool {
	result := messageLoopLastMixObservationResult(state)
	if len(result) == 0 {
		return false
	}
	status := messageLoopMapValue(result["acoustic_package_status"])
	if len(status) == 0 {
		if digest := messageLoopMapValue(result["acoustic_digest"]); len(digest) > 0 {
			status = messageLoopMapValue(digest["acoustic_package_status"])
		}
	}
	if len(status) == 0 {
		observation := messageLoopMapValue(result["observation"])
		status = messageLoopMapValue(observation["acoustic_package_status"])
	}
	if len(status) == 0 {
		return false
	}
	layers := messageLoopMapValue(status["package_layers"])
	l3 := messageLoopMapValue(layers["l3_deep"])
	if l3Status := strings.ToLower(strings.TrimSpace(messageLoopText(l3["status"]))); l3Status != "" && l3Status != "ready" {
		return true
	}
	features := messageLoopMapValue(l3["features"])
	for _, name := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"} {
		feature := messageLoopMapValue(features[name])
		switch strings.ToLower(strings.TrimSpace(messageLoopText(feature["status"]))) {
		case "ready":
			continue
		case "":
			return true
		default:
			return true
		}
	}
	return false
}

func messageLoopReplyMentionsIncompleteDeepPackage(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return false
	}
	if !messageLoopTextHasAny(text, "l3", "深度", "spectrogram", "band energy", "stereo relation") {
		return false
	}
	return messageLoopTextHasAny(text,
		"building", "partial", "incomplete", "not ready",
		"还在构建", "正在构建", "未完整", "不完整", "未完成", "不可靠", "仍是 partial", "还在 building",
	)
}

func messageLoopLastMixObservationResult(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		if !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if len(result) == 0 {
			result = record
		}
		return result
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		event := state.trace[i]
		if event.ToolResult == nil || toolStatusFailed(event.ToolResult.Status) || !messageLoopIsMixObservationName(event.ToolResult.Tool) {
			continue
		}
		return event.ToolResult.Result
	}
	return nil
}

func messageLoopStripExecutionQuestion(reply string) string {
	reply = strings.TrimSpace(reply)
	if reply == "" || !messageLoopMixReplyAsksForExecution(reply) {
		return reply
	}
	lines := strings.Split(reply, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || messageLoopMixReplyAsksForExecution(last) {
			lines = lines[:len(lines)-1]
			continue
		}
		break
	}
	stripped := strings.TrimSpace(strings.Join(lines, "\n"))
	if stripped == "" {
		return reply
	}
	return stripped
}

func messageLoopMixReplyAsksForExecution(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return false
	}
	if messageLoopTextHasAny(text,
		"需要我继续执行吗", "要我继续执行吗", "需要我执行吗", "要我执行吗", "要我现在执行吗", "现在执行吗", "执行这个", "继续执行这一步吗",
	) {
		return true
	}
	return messageLoopTextHasAny(text,
		"需要我继续执行吗", "要我继续执行吗", "需要我执行吗", "要我执行吗", "要我现在执行吗", "现在执行吗", "执行这个",
		"我下一步就", "如果你确认", "你确认后",
		"should i continue", "want me to continue", "shall i continue", "should i execute", "execute this", "if you confirm", "once you confirm",
	)
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
	if state != nil && (messageLoopNaturalMixRequest(state.input.UserText) || messageLoopAudioObservationRequest(state.input.UserText)) && !messageLoopObservationPackageReadRequest(state.input.UserText) && (!messageLoopExplicitPluginOrRawRequest(state.input.UserText) || messageLoopLowMudPluginPrepRequest(state.input.UserText) || messageLoopMutationBarrierActive(state)) && !messageLoopHasAnyMixObservationAttempt(state) {
		return "cannot finish yet; broad acoustic mixing requests must run mix.observe first so the reply is grounded in current observation data"
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
