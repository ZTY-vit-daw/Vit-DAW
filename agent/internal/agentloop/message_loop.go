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
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/epm"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/promptruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	"vit-daw-agent/internal/tim"
	"vit-daw-agent/internal/tom"
	"vit-daw-agent/internal/toolpolicy"
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

var (
	messageLoopQuotedLocalPathPattern = regexp.MustCompile(`(?i)["'“”‘’]([a-z]:[\\/][^"'“”‘’\r\n]+|\\\\[^"'“”‘’\r\n]+)["'“”‘’]`)
	messageLoopLocalPathPattern       = regexp.MustCompile(`(?i)([a-z]:[\\/][^\r\n"'<>|]+|\\\\[^\r\n"'<>|]+)`)
)

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
	Final                 bool                   `json:"final"`
	Reply                 string                 `json:"reply,omitempty"`
	NeedsClarification    bool                   `json:"needs_clarification,omitempty"`
	ClarificationQuestion string                 `json:"clarification_question,omitempty"`
	FailureReason         string                 `json:"failure_reason,omitempty"`
	ToolCalls             []planner.ToolCall     `json:"tool_calls,omitempty"`
	SemanticAction        *semanticeffect.Action `json:"semantic_action,omitempty"`
	FreeStateDecision     *FreeStateDecision     `json:"free_state,omitempty"`
}

func (l *MessageLoop) Start(ctx context.Context, in Input) Result {
	r := l.runner()
	goal := r.ensureGoal(in.GoalID, in.RunID, firstNonEmpty(in.Summary, in.UserText))
	baseBudget := normalizeBudget(firstNonZeroBudget(in.Budget, l.Budget))
	if r.Runtime != nil {
		if opened, slice, ok := r.Runtime.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
			goal = opened
			in.SliceID = slice.SliceID
			if goal.Task != nil {
				in.TaskID = goal.Task.TaskID
				in.OriginalIntent = goal.Task.OriginalIntent
			}
			if openedGoal, turn, turnOK := r.Runtime.BeginTurn(goal.GoalID, slice.SliceID, "user"); turnOK {
				goal = openedGoal
				in.TurnID = turn.TurnID
			}
		}
	}
	state := runState{
		input:              in,
		goal:               goal,
		trace:              append([]planner.TraceEvent(nil), in.Trace...),
		planItems:          mergePlanItems(nil, in.PlanItems),
		contextSnapshot:    cloneMap(in.ContextSnapshot),
		projectHistory:     cloneMap(in.ProjectHistory),
		executionMemory:    cloneExecutionMemory(in.ExecutionMemory),
		recentObservation:  cloneRecentObservation(in.RecentObservation),
		freeStateDecision:  cloneFreeStateDecision(in.FreeStateDecision),
		budget:             baseBudget,
		continuationBudget: baseBudget,
		startedAt:          r.now(),
	}
	state.input.GoalID = goal.GoalID
	state.input.RunID = goal.RunID
	if goal.Task != nil {
		state.input.TaskID = goal.Task.TaskID
		state.input.OriginalIntent = goal.Task.OriginalIntent
	}
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
	baseBudget := normalizeBudget(firstNonZeroBudget(cont.Budget, l.Budget))
	if opened, slice, ok := r.Runtime.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
		goal = opened
		cont.SliceID = slice.SliceID
		if goal.Task != nil {
			cont.TaskID = goal.Task.TaskID
			cont.OriginalIntent = goal.Task.OriginalIntent
		}
		if openedGoal, turn, turnOK := r.Runtime.BeginTurn(goal.GoalID, slice.SliceID, "automatic_continuation"); turnOK {
			goal = openedGoal
			cont.TurnID = turn.TurnID
		}
	}
	state := runState{
		input: Input{
			GoalID:            goal.GoalID,
			RunID:             goal.RunID,
			TaskID:            cont.TaskID,
			SliceID:           cont.SliceID,
			TurnID:            cont.TurnID,
			ResumedFromID:     cont.ContinuationID,
			OriginalIntent:    cont.OriginalIntent,
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
			FreeStateDecision: cloneFreeStateDecision(cont.FreeStateDecision),
		},
		goal:                goal,
		trace:               append([]planner.TraceEvent(nil), cont.Trace...),
		planItems:           mergePlanItems(nil, cont.PlanItems),
		pendingToolQueue:    append([]planner.ToolCall(nil), cont.PendingToolQueue...),
		completedSteps:      cont.CompletedSteps,
		turnsUsed:           cont.TurnsUsed,
		toolCallsUsed:       cont.ToolCallsUsed,
		sliceTurnsStart:     cont.TurnsUsed,
		sliceToolCallsStart: cont.ToolCallsUsed,
		contextSnapshot:     cloneMap(cont.ContextSnapshot),
		projectHistory:      cloneMap(cont.ProjectHistory),
		executionMemory:     cloneExecutionMemory(cont.ExecutionMemory),
		recentObservation:   cloneRecentObservation(cont.RecentObservation),
		freeStateDecision:   cloneFreeStateDecision(cont.FreeStateDecision),
		budget:              extendContinuationBudget(baseBudget, cont.TurnsUsed, cont.ToolCallsUsed),
		continuationBudget:  baseBudget,
		startedAt:           r.now(),
	}
	return l.loop(ctx, r, &state)
}

func (l *MessageLoop) ResumeAfterConfirmation(ctx context.Context, cont Continuation) Result {
	r := l.runner()
	baseBudget := normalizeBudget(firstNonZeroBudget(cont.Budget, l.Budget))
	goal := r.ensureGoal(cont.GoalID, cont.RunID, cont.Summary)
	if r.Runtime != nil {
		if opened, slice, ok := r.Runtime.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
			goal = opened
			cont.SliceID = slice.SliceID
			if goal.Task != nil {
				cont.TaskID = goal.Task.TaskID
				cont.OriginalIntent = goal.Task.OriginalIntent
			}
			if openedGoal, turn, turnOK := r.Runtime.BeginTurn(goal.GoalID, slice.SliceID, "user_interaction"); turnOK {
				goal = openedGoal
				cont.TurnID = turn.TurnID
			}
		}
	}
	state := runState{
		input: Input{
			GoalID:            goal.GoalID,
			RunID:             goal.RunID,
			TaskID:            cont.TaskID,
			SliceID:           cont.SliceID,
			TurnID:            cont.TurnID,
			ResumedFromID:     cont.ContinuationID,
			OriginalIntent:    cont.OriginalIntent,
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
			FreeStateDecision: cloneFreeStateDecision(cont.FreeStateDecision),
		},
		goal:                goal,
		trace:               append([]planner.TraceEvent(nil), cont.Trace...),
		planItems:           mergePlanItems(nil, cont.PlanItems),
		pendingToolQueue:    append([]planner.ToolCall(nil), cont.PendingToolQueue...),
		contextSnapshot:     cloneMap(cont.ContextSnapshot),
		projectHistory:      cloneMap(cont.ProjectHistory),
		executionMemory:     cloneExecutionMemory(cont.ExecutionMemory),
		recentObservation:   cloneRecentObservation(cont.RecentObservation),
		freeStateDecision:   cloneFreeStateDecision(cont.FreeStateDecision),
		completedSteps:      cont.CompletedSteps,
		turnsUsed:           cont.TurnsUsed,
		toolCallsUsed:       cont.ToolCallsUsed,
		sliceTurnsStart:     cont.TurnsUsed,
		sliceToolCallsStart: cont.ToolCallsUsed,
		budget:              extendContinuationBudget(baseBudget, cont.TurnsUsed, cont.ToolCallsUsed),
		continuationBudget:  baseBudget,
		startedAt:           r.now(),
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
	settingsPatch := messageLoopProjectAudioSettingsPatchForConfirmedImport(call)
	if len(settingsPatch) > 0 {
		if !allowedTool("project.set_audio_settings", state.input.AllowedTools) {
			return r.fail(&state, fmt.Errorf("project.set_audio_settings is required before this import but is not allowed"))
		}
		settingsCall := messageLoopSetAudioSettingsCallForImport(call, settingsPatch)
		if issue := messageLoopToolGuardIssue(&state, settingsCall, messageLoopHasUsableMixObservation(&state)); issue != "" {
			messageLoopAppendGuardGate(&state, settingsCall, issue)
			return l.loop(ctx, r, &state)
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, &state, settingsCall, true, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=true stems_import_sample_rate_patch=true stopped=%t status=%s", state.goal.GoalID, settingsCall.Tool, stopped, result.Status)
		if stopped {
			return result
		}
		appendMessageLoopToolResult(&state)
		if len(state.executed) == 0 || !messageLoopExecutionSucceeded(state.executed[len(state.executed)-1]) {
			return r.fail(&state, fmt.Errorf("project.set_audio_settings failed before stems import"))
		}
	}
	if handled, result := l.executeConfirmedStripSilenceBundle(ctx, r, &state, call); handled {
		return result
	}
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, &state, call, true, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return result
	}
	appendMessageLoopToolResult(&state)
	if len(settingsPatch) > 0 {
		messageLoopAnnotateLastExecutionResultWithSampleRatePatch(&state, settingsPatch)
	}
	if len(state.executed) > 0 {
		record := state.executed[len(state.executed)-1]
		if reply, stopped, result := l.messageLoopStemsImportCompleteReplyWithDADGate(ctx, r, &state, record); stopped {
			return result
		} else if strings.TrimSpace(reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic stems import report completed the turn"})
			return r.complete(&state, messageLoopMixObservationFinalReply(&state, reply))
		}
		if reply := messageLoopTrackOrganizationFastCompleteReply(record); strings.TrimSpace(reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic track organization report completed the turn"})
			return r.complete(&state, reply)
		}
		if len(state.pendingToolQueue) == 0 {
			if reply, handled, result := l.messageLoopB12SourceCalibrationCompleteReply(ctx, r, &state, call); handled {
				if result.Status != "" {
					return result
				}
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed B1.2 source calibration completed with verification"})
				return r.complete(&state, reply)
			}
			if reply, handled, result := l.messageLoopB1FaderResetCompleteOrchestrate(ctx, r, &state, call); handled {
				if result.Status != "" {
					return result
				}
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed B1 fader reset completed with B1.2 orchestration"})
				return r.complete(&state, reply)
			}
			if reply, ok := messageLoopClipFadeGainSetReply(&state, call); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed clip fade/gain set completed the turn"})
				return r.complete(&state, reply)
			}
			if reply, ok := messageLoopTrackGroupApplyControlReply(&state, call); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed track group apply_control completed the turn"})
				return r.complete(&state, reply)
			}
			if messageLoopIsStripSilenceApplyCall(call) {
				if !messageLoopExecutionSucceeded(record) {
					errText := firstNonEmpty(messageLoopText(record["error"]), "confirmed Strip Silence apply failed")
					return r.fail(&state, fmt.Errorf("%s", errText))
				}
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed Strip Silence apply completed the turn"})
				return r.complete(&state, messageLoopStripSilenceApplyCompleteReply(record, call))
			}
		}
	}
	return l.loop(ctx, r, &state)
}

func (l *MessageLoop) executeConfirmedStripSilenceBundle(ctx context.Context, r *Runner, state *runState, call planner.ToolCall) (bool, Result) {
	calls, ok := messageLoopStripSilenceBundleCalls(call)
	if !ok {
		return false, Result{}
	}
	if len(calls) == 0 {
		return true, r.fail(state, fmt.Errorf("confirmed Strip Silence bundle has no executable apply actions"))
	}
	batchCall := messageLoopStripSilenceBatchCallFromActions(call, calls)
	if !allowedTool(batchCall.Tool, state.input.AllowedTools) && !allowedTool(call.Tool, state.input.AllowedTools) {
		return true, r.fail(state, fmt.Errorf("unknown or disallowed tool: %s", strings.TrimSpace(batchCall.Tool)))
	}
	if stopped, result := r.checkpoint("before_confirmed_strip_silence_apply_batch", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if issue := messageLoopToolGuardIssue(state, batchCall, messageLoopHasUsableMixObservation(state)); issue != "" {
		messageLoopAppendGuardGate(state, batchCall, issue)
		return true, r.fail(state, fmt.Errorf("%s", issue))
	}
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, batchCall, true, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=true strip_silence_legacy_bundle=true stopped=%t status=%s", state.goal.GoalID, batchCall.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if len(state.executed) == 0 || !messageLoopExecutionSucceeded(state.executed[len(state.executed)-1]) {
		errText := ""
		if len(state.executed) > 0 {
			errText = messageLoopText(state.executed[len(state.executed)-1]["error"])
		}
		if strings.TrimSpace(errText) == "" {
			errText = "confirmed Strip Silence batch apply failed"
		}
		return true, r.fail(state, fmt.Errorf("%s", errText))
	}
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "confirmed Strip Silence bundle completed"})
	return true, r.complete(state, messageLoopStripSilenceApplyCompleteReply(state.executed[len(state.executed)-1], batchCall))
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
		if len(state.pendingToolQueue) > 0 {
			queued := append([]planner.ToolCall(nil), state.pendingToolQueue...)
			if stopped, result := l.executeMessageLoopToolCalls(ctx, r, state, queued, messageLoopOutput{}, messageLoopHasUsableMixObservation(state)); stopped {
				return result
			}
		}
		// Diagnostic-only turns are a CCB-only experiment boundary. Ordinary
		// deterministic preflights (especially project.state) would bypass that
		// boundary and manufacture a terminal result before the model observes.
		if !messageLoopFreeStateDiagnosticOnly(state) {
			if stopped, result := l.preflightStaticMixCapabilityContract(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightProjectBlackboardStatus(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightClipFadeGainSet(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightClipFadeGainRead(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightStripSilenceSuggest(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightStemsFolderImport(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightPendingSectionMarkersApply(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightNaturalMixObservation(ctx, r, state); stopped {
				return result
			}
			if stopped, result := l.preflightStaticMixGainStagingContextPack(ctx, r, state); stopped {
				return result
			}
		}
		if stopped, result := r.checkpoint("before_message_loop_model", state); stopped {
			return result
		}
		if limit, result := r.checkTurnBudget(state); limit {
			return result
		}
		snapshot := r.buildContextSnapshot(state)
		state.contextSnapshot = snapshot.Map()
		modelSnapshotJSON := r.buildModelContextSnapshot(state, snapshot)
		if overflow := contextruntime.ModelContextOverflow(modelSnapshotJSON); overflow != "" {
			// Fail closed: an over-budget model request is never sent. The
			// snapshot already carries the explicit context_overflow marker
			// with per-section bytes and a cold ref; facts are never silently
			// truncated into a request.
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "context_overflow: " + overflow})
			return r.fail(state, fmt.Errorf("context_overflow: %s", overflow))
		}
		assembly := l.assembly(state, modelSnapshotJSON)
		// BOUNDARY-1 §3.1 prompt-render evidence: persist the actually
		// assembled system prompt in full for every model turn (D1 and
		// ordinary runs alike), with the round identity and the stable
		// section tag — closing the CONVDIAG "final-window rendering is
		// unverifiable" gap.
		appendMessageLoopPromptRenderDiagnostic(state, assembly)
		assemblyChars := 0
		messageCharParts := make([]string, 0, len(assembly.Messages))
		for index, message := range assembly.Messages {
			messageChars := len([]rune(message.Content))
			assemblyChars += messageChars
			messageCharParts = append(messageCharParts, fmt.Sprintf("%d:%s:%d", index, message.Role, messageChars))
		}
		llmStarted := time.Now()
		promptStats := assembly.Stats.Map()
		for key, value := range messageLoopModelContextPromptStats(modelSnapshotJSON, assembly.Messages) {
			promptStats[key] = value
		}
		raw, err := llm.CompleteText(ctx, l.Client, l.Config, llm.Request{
			Messages: assembly.Messages,
			Metadata: llm.RequestMetadata{
				Source:            "message_loop",
				ConversationID:    messageLoopConversationID(state),
				GoalID:            state.goal.GoalID,
				PromptFingerprint: assembly.Fingerprint,
				PromptStats:       promptStats,
			},
		})
		l.logTiming("message_loop.llm", llmStarted, "goal=%s conversation=%s turn=%d messages=%d chars=%d message_chars=%s err=%t", state.goal.GoalID, messageLoopConversationID(state), state.turnsUsed+1, len(assembly.Messages), assemblyChars, strings.Join(messageCharParts, ","), err != nil)
		state.turnsUsed++
		if err != nil {
			if messageLoopFreeStateActive(state) && messageLoopTransientLLMError(err) {
				state.turnsUsed--
				reply := messageLoopFreeStateTransientPauseReply(state)
				state.trace = append(state.trace, planner.TraceEvent{
					Kind:    "final_gate",
					Message: "transient LLM failure paused the active free-state loop without marking it complete: " + err.Error(),
				})
				return r.pause(state, agentruntime.StatusWaitingContinue, StopReasonTransientLLMError, "", reply, "", "", nil)
			}
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
			if reply, ok := messageLoopExecutionFallbackAfterLLMError(state, err); ok {
				state.trace = append(state.trace, planner.TraceEvent{
					Kind:    "final_gate",
					Message: "LLM reply failed after verified execution; returned deterministic execution fallback: " + err.Error(),
				})
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
				state.modelProtocolFailure = true
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
				if messageLoopFreeStateTerminalTurnLocked(state) {
					return messageLoopTerminalFallbackResult(r, state, assembly.Fingerprint, raw, messageLoopJSONRepairRetryPrompt, "terminal turn output was unparseable after the strengthened JSON repair retry")
				}
				return r.fail(state, fmt.Errorf("Agent 返回的计划格式不完整，自动修复也失败了"))
			}
			repairedOut, parseRepairErr := parseMessageLoopOutput(repairedRaw)
			if parseRepairErr != nil {
				state.modelProtocolFailure = true
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
				if messageLoopFreeStateTerminalTurnLocked(state) {
					return messageLoopTerminalFallbackResult(r, state, assembly.Fingerprint, raw, messageLoopJSONRepairRetryPrompt, "terminal turn output was unparseable after the strengthened JSON repair retry")
				}
				return r.fail(state, fmt.Errorf("Agent 返回的计划格式不完整，自动修复也没有得到可执行计划"))
			}
			raw = repairedRaw
			out = repairedOut
			state.modelProtocolRepairs++
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_repair", Message: "message loop JSON repaired"})
		}
		out = coerceMessageLoopFreeStateObservationOutput(state, out)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "assistant", Content: strings.TrimSpace(raw)})
		if strings.TrimSpace(out.Reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "assistant", Reply: strings.TrimSpace(out.Reply)})
		}
		// BOUNDARY-1 §3.2 strict terminal parse: on a locked turn the raw
		// response must be exactly one JSON object. A prose-mixed or
		// multi-object output is classified unparseable — no lenient repair,
		// no silent default — and follows the same one-retry-then-honest-
		// fallback chain as the gate rejection below.
		if messageLoopFreeStateTerminalTurnLocked(state) && messageLoopFreeStateTerminalRawUnparseable(raw) {
			appendMessageLoopDiagnostic(messageLoopDiagnostic{
				Stage:             "free_state_terminal_parse_strict",
				Error:             "terminal turn raw output was not one clean JSON object (prose-mixed or multi-object output)",
				GoalID:            state.goal.GoalID,
				RunID:             state.goal.RunID,
				ConversationID:    messageLoopConversationID(state),
				PromptFingerprint: assembly.Fingerprint,
				Raw:               raw,
			})
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "terminal-turn strict parse: raw output was not one clean JSON object"})
			if messageLoopFreeStateTerminalRetryCount(state) < freeStateTerminalTurnMaxRetries {
				messageLoopFreeStateNoteTerminalRetry(state)
				retryPrompt := freeStateTerminalTurnSentence + " " + freeStateTerminalTurnRetryDirective
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + retryPrompt + "</final_gate>"})
				continue
			}
			return messageLoopTerminalFallbackResult(r, state, assembly.Fingerprint, raw,
				freeStateTerminalTurnSentence+" "+freeStateTerminalTurnRetryDirective,
				"terminal turn raw output was not one clean JSON object after one strengthened retry")
		}
		if strings.TrimSpace(out.FailureReason) != "" {
			if strings.EqualFold(strings.TrimSpace(out.FailureReason), StopReasonModelProtocolFailure) {
				state.modelProtocolFailure = true
			}
			state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_failure", Message: out.FailureReason})
			return r.fail(state, errors.New(out.FailureReason))
		}
		if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
			appendMessageLoopDiagnostic(messageLoopDiagnostic{
				Stage:             "free_state_final_gate",
				Error:             issue,
				GoalID:            state.goal.GoalID,
				RunID:             state.goal.RunID,
				ConversationID:    messageLoopConversationID(state),
				PromptFingerprint: assembly.Fingerprint,
				Raw:               raw,
			})
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue})
			if messageLoopFreeStateTerminalTurnLocked(state) {
				// BOUNDARY-1 §1.3 fallback chain: one strengthened retry, then
				// the honest settle with the three-piece evidence (original
				// response / retry prompt / fallback reason) in the artifact
				// log. The fallback never counts as a model decision.
				if messageLoopFreeStateTerminalRetryCount(state) < freeStateTerminalTurnMaxRetries {
					messageLoopFreeStateNoteTerminalRetry(state)
					retryPrompt := issue + " " + freeStateTerminalTurnRetryDirective
					state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + retryPrompt + "</final_gate>"})
					continue
				}
				return messageLoopTerminalFallbackResult(r, state, assembly.Fingerprint, raw,
					issue+" "+freeStateTerminalTurnRetryDirective,
					"no admissible final decision after one strengthened retry")
			}
			// TIMING-1 anti-abuse accounting: count the evidence-type G-gate
			// bounce, latch the rejected proposal fingerprint, and lock the
			// terminal turn on the second rejection or an identical-fingerprint
			// resubmission without new evidence (advisory ruling #5 rules 1-3).
			// The lock lands after the locked-turn check above, so this bounce
			// keeps its ordinary gap feedback and the next turn is the terminal
			// prompt; the budget-critical reservation is never downgraded.
			messageLoopFreeStateNoteAdmissionRejection(state, out, issue)
			state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
			continue
		}
		state.freeStateDecision = cloneFreeStateDecision(out.FreeStateDecision)
		if out.NeedsClarification {
			if handled, stopped, result := l.maybeStartStemsImportConfirmation(ctx, r, state, out, "clarification_after_preflight"); handled {
				if stopped {
					return result
				}
				continue
			}
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
			// FULLACCESS-AUTONOMY-1 guardrail: under an explicit full project access
			// grant a direction/selection/dose question is bounced once (the prompt
			// directive already states who owns the choice). Request-object
			// ambiguity and genuine boundaries fall through untouched, and the
			// bounce is bounded to one per turn so the turn can never be trapped.
			if issue := messageLoopFullAccessDirectionClarificationIssue(state, out); issue != "" {
				if messageLoopFullAccessDirectionClarificationBounce(state, issue, raw) {
					continue
				}
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
			if handled, stopped, result := l.maybeStartStemsImportConfirmation(ctx, r, state, out, "final_after_preflight"); handled {
				if stopped {
					return result
				}
				continue
			}
			if reply, status, ok := messageLoopFreeStateTerminalReply(out); ok {
				state.trace = append(state.trace, planner.TraceEvent{
					Kind:    "final_gate",
					Message: "accepted authoritative free-state terminal decision: " + status,
				})
				return r.complete(state, reply)
			}
			if messageLoopOrdinarySemanticEQRequest(state) && out.SemanticAction == nil && !messageLoopHasGenericEQTopologyEvidence(state) {
				issue := "this is an actionable ordinary-Agent generic EQ listening goal with an exact selected plugin target; do not create MixTreatmentPending or use stored-mapping/B4 preparation. Read the live EQ topology if needed, then emit semantic_effect_action.v1, or ask a genuine target clarification"
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue})
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
				continue
			}
			if out.SemanticAction != nil && messageLoopOrdinarySemanticEQRequest(state) && !messageLoopHasGenericEQTopologyEvidence(state) {
				issue := "before freezing semantic_action, call plugin_grabber.explain_controls for the exact selected track/plugin and use only its live generic EQ eq_band_summary/control_topology evidence to choose a provably reachable Shape and explicit fields"
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue})
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
				continue
			}
			if reply, ok := messageLoopCompressorFailureFinalReply(state); ok {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "typed compressor failure completed truthfully without fallback"})
				return r.complete(state, reply)
			}
			if issue := messageLoopFinalIssue(state); issue != "" {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue, PlanItems: append([]planner.PlanItem(nil), state.planItems...)})
				state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
				continue
			}
			reply := strings.TrimSpace(out.Reply)
			if reply == "" {
				reply = "已完成。"
			}
			if out.SemanticAction != nil {
				if !messageLoopSemanticEffectProposalAllowed(state) {
					issue := "the current turn is discussion/read-only; answer the question without semantic_action, Proposal, pending state, or mutation authority"
					state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue})
					state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
					continue
				}
				if err := out.SemanticAction.Validate(); err != nil {
					issue := "semantic_action is invalid: " + err.Error() + "; correct the typed action without changing the user's acoustic goal"
					state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue})
					state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + issue + "</final_gate>"})
					continue
				}
				state.semanticAction = cloneSemanticEffectAction(out.SemanticAction)
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
		if stopped, result := l.executeMessageLoopToolCalls(ctx, r, state, out.ToolCalls, out, hadMixObservationBeforeTurn); stopped {
			return result
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

func (l *MessageLoop) executeMessageLoopToolCalls(ctx context.Context, r *Runner, state *runState, calls []planner.ToolCall, out messageLoopOutput, hadMixObservationBeforeTurn bool) (bool, Result) {
	for i := range calls {
		call := normalizeMessageLoopToolCall(calls[i], state.completedSteps+1)
		call = resolveMessageLoopBindings(state, call)
		call = coerceWaveformBakeToMixObservation(state, call)
		call = coerceMixObservationCall(state, call)
		call = coerceObservationPackageReadCall(state, call)
		call = coerceMixTickPrimitiveCall(state, call)

		remaining := append([]planner.ToolCall(nil), calls[i+1:]...)
		state.pendingToolQueue = append([]planner.ToolCall{call}, remaining...)
		if reply, ok := messageLoopMixTickProposalAsPendingTreatment(state, call, out); ok {
			state.pendingToolQueue = nil
			state.trace = append(state.trace, planner.TraceEvent{
				Kind:     "final_gate",
				Message:  "natural-language mix tick proposal normalized to pending treatment",
				ToolCall: cloneToolCallPtr(call),
			})
			return true, r.complete(state, reply)
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
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		if !allowedTool(call.Tool, state.input.AllowedTools) {
			result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
			state.trace = append(state.trace, planner.TraceEvent{Kind: "tool_call", ToolCall: &call}, planner.TraceEvent{Kind: "tool_result", ToolResult: &result})
			state.consecutiveErrors++
			appendMessageLoopToolResult(state)
			if state.consecutiveErrors >= state.budget.MaxConsecutiveErrors {
				return true, r.fail(state, errors.New(result.Error))
			}
			continue
		}
		if issue := messageLoopToolGuardIssue(state, call, hadMixObservationBeforeTurn); issue != "" {
			messageLoopAppendGuardGate(state, call, issue)
			continue
		}
		state.pendingToolQueue = remaining
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, remaining)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
		appendMessageLoopToolResult(state)
		if reply, failed := messageLoopCompressorFailureFinalReply(state); failed {
			state.pendingToolQueue = nil
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "typed compressor apply failed; stopped before retry or fallback"})
			return true, r.complete(state, reply)
		}
	}
	state.pendingToolQueue = nil
	return false, Result{}
}

func messageLoopModelContextPromptStats(snapshotJSON string, messages []llm.Message) map[string]any {
	out := map[string]any{"model_snapshot_bytes": len([]byte(snapshotJSON))}
	snapshot := map[string]any{}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err == nil {
		sections := contextruntime.ModelSectionBytes(snapshot)
		out["model_section_bytes"] = sections
		out["active_observation_bytes"] = sections["active_observation"]
		out["observation_ledger_bytes"] = sections["observation_ledger"]
		if size := messageLoopMapValue(snapshot["context_size"]); len(size) > 0 {
			out["hot_bytes"] = size["hot_bytes"]
			out["warm_bytes"] = size["warm_bytes"]
		}
		out["canonical_ccb_projection_count"] = messageLoopCountSchema(snapshot, contextruntime.CCBModelProjectionSchema)
		if active := messageLoopMapValue(snapshot["active_observation"]); len(active) > 0 {
			out["active_observation_id"] = firstMapText(active, "observation_id")
			out["active_tool_call_id"] = firstMapText(active, "tool_call_id")
		}
	}
	staticBytes := 0
	runtimeBytes := 0
	historyBytes := 0
	for index, message := range messages {
		bytes := len([]byte(message.Content))
		switch {
		case message.Role == "system":
			staticBytes += bytes
		case message.Role == "user" && index == len(messages)-1:
			runtimeBytes += bytes
		default:
			historyBytes += bytes
		}
	}
	out["static_prompt_bytes"] = staticBytes
	out["runtime_prompt_bytes"] = runtimeBytes
	out["history_prompt_bytes"] = historyBytes
	out["full_request_content_bytes"] = staticBytes + runtimeBytes + historyBytes
	return out
}

func messageLoopCountSchema(value any, schema string) int {
	switch typed := value.(type) {
	case map[string]any:
		count := 0
		if firstMapText(typed, "schema_version") == schema {
			count++
		}
		for _, child := range typed {
			count += messageLoopCountSchema(child, schema)
		}
		return count
	case []any:
		count := 0
		for _, child := range typed {
			count += messageLoopCountSchema(child, schema)
		}
		return count
	default:
		return 0
	}
}

func (l *MessageLoop) preflightStaticMixCapabilityContract(_ context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || !messageLoopStaticMixCapabilityContractRequest(state.input.UserText) {
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "static mix capability contract completed the turn"})
	return true, r.complete(state, messageLoopStaticMixCapabilityContractReply(state))
}

func (l *MessageLoop) preflightProjectBlackboardStatus(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || !messageLoopProjectBlackboardStatusRequest(state.input.UserText) {
		return false, Result{}
	}
	if record := messageLoopProjectBlackboardExecutionResult(state); len(record) > 0 {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "project blackboard status report completed from existing project.state"})
		return true, r.complete(state, messageLoopProjectBlackboardReportFromRecord(state, record))
	}
	call := messageLoopProjectBlackboardStateCall()
	if stopped, result := r.checkpoint("before_message_loop_project_blackboard_status", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		record := map[string]any{
			"tool_call_id": call.ID,
			"tool":         call.Tool,
			"status":       "error",
			"error":        "unknown or disallowed tool: " + strings.TrimSpace(call.Tool),
		}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "project blackboard status report requires project.state"},
			planner.TraceEvent{Kind: "final_gate", Message: "project blackboard status report could not read project.state"},
		)
		return true, r.complete(state, messageLoopProjectBlackboardReportFromRecord(state, record))
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "project status request was routed through Project Blackboard Status Report v0",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false project_blackboard_status=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	record := messageLoopProjectBlackboardExecutionResult(state)
	if len(record) == 0 {
		record = messageLoopProjectBlackboardFallbackRecord(call, result.Error)
	}
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "project blackboard status report completed the turn"})
	return true, r.complete(state, messageLoopProjectBlackboardReportFromRecord(state, record))
}

func (l *MessageLoop) preflightClipFadeGainRead(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state != nil && messageLoopClipFadeGainReadRequest(state.input.UserText) && messageLoopCurrentClipID(state) == "" {
		reply := "\u8bf7\u5148\u5728 GUI \u91cc\u9009\u4e2d\u4e00\u4e2a\u97f3\u9891 clip\uff0c\u7136\u540e\u6211\u518d\u8bfb\u53d6\u5b83\u7684 fade \u548c clip gain \u72b6\u6001\u3002"
		state.trace = append(state.trace, planner.TraceEvent{Kind: "clarification", Message: reply})
		res := r.pause(state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", reply, "", "", nil)
		res.NeedsClarification = true
		res.ClarificationQuestion = reply
		return true, res
	}
	calls, ok := messageLoopDeterministicClipFadeGainReadCalls(state)
	if !ok {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_clip_fade_gain_read", state); stopped {
		return true, result
	}
	for _, call := range calls {
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		if !allowedTool(call.Tool, state.input.AllowedTools) {
			result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "unknown or disallowed tool: " + strings.TrimSpace(call.Tool)}
			state.trace = append(state.trace,
				planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic clip fade/gain read"},
				planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
				planner.TraceEvent{Kind: "final_gate", Message: result.Error},
			)
			state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
			return false, Result{}
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "clip fade/gain read was routed through typed clip tools",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false clip_fade_gain_read=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
		appendMessageLoopToolResult(state)
	}
	if reply := messageLoopClipFadeGainReadReply(state); strings.TrimSpace(reply) != "" {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic clip fade/gain read completed the turn"})
		return true, r.complete(state, reply)
	}
	return true, r.complete(state, "Clip fade/gain read completed.")
}

func (l *MessageLoop) preflightClipFadeGainSet(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	call, ok := messageLoopDeterministicClipFadeGainSetCall(state)
	if !ok {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_clip_fade_gain_set", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "unknown or disallowed tool: " + strings.TrimSpace(call.Tool)}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic clip fade/gain set"},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: result.Error},
		)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "clip fade/gain set was routed through typed clip tools",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false clip_fade_gain_set=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if reply, ok := messageLoopClipFadeGainSetReply(state, call); ok {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic clip fade/gain set completed the turn"})
		return true, r.complete(state, reply)
	}
	return true, r.complete(state, "Clip fade/gain set completed.")
}

func (l *MessageLoop) preflightStripSilenceSuggest(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	call, ok := messageLoopDeterministicStripSilenceSuggestCall(state)
	if !ok {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_strip_silence_suggest", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "unknown or disallowed tool: " + strings.TrimSpace(call.Tool)}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic strip silence suggest"},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: result.Error},
		)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "strip silence recommendation was routed through clip.strip_silence.suggest",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false strip_silence_suggest=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if reply := messageLoopStripSilenceSuggestReply(state); strings.TrimSpace(reply) != "" {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic strip silence suggest completed the turn"})
		return true, r.complete(state, reply)
	}
	return true, r.complete(state, "清理静音分析完成。")
}

func messageLoopDeterministicClipFadeGainReadCalls(state *runState) ([]planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return nil, false
	}
	if !messageLoopClipFadeGainReadRequest(state.input.UserText) {
		return nil, false
	}
	clipID := messageLoopCurrentClipID(state)
	if clipID == "" {
		return nil, false
	}
	args := map[string]any{"clip_id": clipID}
	return []planner.ToolCall{
		{
			ID:      "read_clip_fade",
			Tool:    "clip.fade.read",
			Args:    cloneMap(args),
			Command: map[string]any{"cmd": "clip.fade.read", "clip_id": clipID},
			Reason:  "Read selected clip fade state.",
		},
		{
			ID:      "read_clip_gain",
			Tool:    "clip.gain.read",
			Args:    cloneMap(args),
			Command: map[string]any{"cmd": "clip.gain.read", "clip_id": clipID},
			Reason:  "Read selected clip gain state.",
		},
	}, true
}

func messageLoopClipFadeGainReadRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if !messageLoopClipFadeGainRequest(text) {
		return false
	}
	hasReadIntent := messageLoopTextHasAny(text,
		"read", "show", "inspect", "status", "state", "get",
		"\u8bfb\u53d6", "\u67e5\u770b", "\u770b\u4e00\u4e0b", "\u72b6\u6001",
	)
	hasWriteIntent := messageLoopTextHasAny(text,
		"set", "adjust", "change", "drag", "write", "apply",
		"\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9", "\u5199\u5165", "\u5e94\u7528",
	)
	if !hasReadIntent && messageLoopTextHasAny(text, "db", "d b", "\u5206\u8d1d") {
		return false
	}
	return hasReadIntent || !hasWriteIntent
}

func messageLoopDeterministicClipFadeGainSetCall(state *runState) (planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return planner.ToolCall{}, false
	}
	if !messageLoopClipFadeGainSetRequest(state.input.UserText) {
		return planner.ToolCall{}, false
	}
	clipID := messageLoopCurrentClipID(state)
	if clipID == "" {
		return planner.ToolCall{}, false
	}
	if args, ok := messageLoopClipFadeSetArgs(state.input.UserText, clipID); ok {
		return planner.ToolCall{
			ID:      "set_clip_fade",
			Tool:    "clip.fade.set",
			Args:    cloneMap(args),
			Command: cloneMap(args),
			Reason:  messageLoopClipFadeSetReason(args),
		}, true
	}
	if gainDB, ok := messageLoopClipGainSetValueDB(state.input.UserText); ok {
		args := map[string]any{"clip_id": clipID, "gain_db": gainDB}
		return planner.ToolCall{
			ID:      "set_clip_gain",
			Tool:    "clip.gain.set",
			Args:    args,
			Command: map[string]any{"cmd": "clip.gain.set", "clip_id": clipID, "gain_db": gainDB},
			Reason:  fmt.Sprintf("Set selected clip gain to %+.2f dB.", gainDB),
		}, true
	}
	return planner.ToolCall{}, false
}

func messageLoopDeterministicStripSilenceSuggestCall(state *runState) (planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return planner.ToolCall{}, false
	}
	if !messageLoopStripSilenceSuggestRequest(state.input.UserText) {
		return planner.ToolCall{}, false
	}
	args := map[string]any{"scope": "selected_clip"}
	if messageLoopStripSilenceAllProjectRequest(state.input.UserText) {
		args["scope"] = "all_project"
	} else {
		clipID := messageLoopCurrentClipID(state)
		trackID := messageLoopCurrentClipTrackID(state)
		ranges := messageLoopSelectedClipRanges(state)
		if messageLoopStripSilenceSelectedTrackRequest(state.input.UserText) {
			if trackID == "" {
				return planner.ToolCall{}, false
			}
			args["scope"] = "selected_track"
			args["track_id"] = trackID
		} else if useRanges := len(ranges) > 0 && (messageLoopStripSilenceRangeRequest(state.input.UserText) || clipID == ""); useRanges {
			args["scope"] = "selected_ranges"
			args["selected_clip_ranges"] = ranges
		} else {
			if clipID != "" {
				args["clip_id"] = clipID
			}
			if trackID != "" {
				args["track_id"] = trackID
			}
		}
		if args["clip_id"] == nil && args["selected_clip_ranges"] == nil && args["track_id"] == nil {
			return planner.ToolCall{}, false
		}
	}
	command := cloneMap(args)
	command["cmd"] = "clip.strip_silence.suggest"
	return planner.ToolCall{
		ID:      "suggest_strip_silence",
		Tool:    "clip.strip_silence.suggest",
		Args:    args,
		Command: command,
		Reason:  "Recommend Strip Silence parameters and pending apply actions without mutating the project.",
	}, true
}

func messageLoopClipFadeSetArgs(userText, clipID string) (map[string]any, bool) {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !messageLoopTextHasAny(text, "fade", "\u6de1\u5165", "\u6de1\u51fa", "\u6de1\u5316") {
		return nil, false
	}
	args := map[string]any{"cmd": "clip.fade.set", "clip_id": clipID}
	if seconds, ok := messageLoopExtractFadeSeconds(text, true); ok {
		args["fade_in_seconds"] = seconds
	}
	if seconds, ok := messageLoopExtractFadeSeconds(text, false); ok {
		args["fade_out_seconds"] = seconds
	}
	if _, hasIn := args["fade_in_seconds"]; hasIn {
		return args, true
	}
	if _, hasOut := args["fade_out_seconds"]; hasOut {
		return args, true
	}
	return nil, false
}

func messageLoopExtractFadeSeconds(text string, fadeIn bool) (float64, bool) {
	labels := []string{`fade\s*in`, `fade-in`, `fadein`, "\u6de1\u5165"}
	if !fadeIn {
		labels = []string{`fade\s*out`, `fade-out`, `fadeout`, "\u6de1\u51fa"}
	}
	unitPattern := `ms|msec|milliseconds?|millisecond|` + "\u6beb\u79d2" + `|s|sec|seconds?|second|` + "\u79d2"
	pattern := regexp.MustCompile(`(?i)(?:` + strings.Join(labels, "|") + `)[^\d+\-]{0,32}([+\-]?\d+(?:\.\d+)?)\s*(` + unitPattern + `)?`)
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	if err != nil || value < 0 {
		return 0, false
	}
	unit := ""
	if len(match) >= 3 {
		unit = strings.ToLower(strings.TrimSpace(match[2]))
	}
	switch unit {
	case "ms", "msec", "millisecond", "milliseconds", "\u6beb\u79d2":
		value = value / 1000.0
	}
	if value > 600 {
		return 0, false
	}
	return value, true
}

func messageLoopClipFadeSetReason(args map[string]any) string {
	parts := []string{}
	if seconds, ok := firstNumericMapValue(args, "fade_in_seconds"); ok {
		parts = append(parts, fmt.Sprintf("fade in %.3fs", seconds))
	}
	if seconds, ok := firstNumericMapValue(args, "fade_out_seconds"); ok {
		parts = append(parts, fmt.Sprintf("fade out %.3fs", seconds))
	}
	if len(parts) == 0 {
		return "Set selected clip fade."
	}
	return "Set selected clip " + strings.Join(parts, ", ") + "."
}

func messageLoopClipFadeGainSetRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if !messageLoopClipFadeGainRequest(text) {
		return false
	}
	return messageLoopTextHasAny(text,
		"set", "adjust", "change", "drag", "write", "apply", "to ", "at ",
		"\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9", "\u5199\u5165", "\u5e94\u7528", "\u5230", "\u4e3a",
	)
}

func messageLoopClipGainSetValueDB(userText string) (float64, bool) {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !messageLoopTextHasAny(text, "clip gain", "\u589e\u76ca") {
		return 0, false
	}
	return messageLoopExtractAbsoluteDBAmount(text)
}

func messageLoopExtractAbsoluteDBAmount(text string) (float64, bool) {
	matches := messageLoopDBAmountPattern.FindAllStringSubmatch(text, -1)
	if len(matches) != 1 || len(matches[0]) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(matches[0][1]), 64)
	if err != nil {
		return 0, false
	}
	if mathAbs(value) > 60 {
		return 0, false
	}
	return value, true
}

func messageLoopClipFadeGainSetReply(state *runState, call planner.ToolCall) (string, bool) {
	if state == nil || len(state.executed) == 0 {
		return "", false
	}
	record := state.executed[len(state.executed)-1]
	if !messageLoopExecutionSucceeded(record) {
		return "", false
	}
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]), call.Tool)))
	if name != "clip.gain.set" && name != "clip.fade.set" {
		return "", false
	}
	clipID := firstNonEmpty(firstMapText(messageLoopMapValue(record["result"]), "clip_id", "id", "item_id"), firstMapText(call.Args, "clip_id"))
	if name == "clip.gain.set" {
		if gainDB, ok := firstNumericMapValue(messageLoopMapValue(record["result"]), "clip_gain_db", "gain_db", "db"); ok {
			return fmt.Sprintf("Clip %s gain 已设置为 %+.2f dB。", clipID, gainDB), true
		}
		if gainDB, ok := firstNumericMapValue(call.Args, "gain_db", "clip_gain_db", "db"); ok {
			return fmt.Sprintf("Clip %s gain 已设置为 %+.2f dB。", clipID, gainDB), true
		}
	}
	if name == "clip.fade.set" {
		result := messageLoopMapValue(record["result"])
		inSeconds, hasIn := firstNumericMapValue(result, "fade_in_seconds", "fade_in", "fadeInSeconds")
		outSeconds, hasOut := firstNumericMapValue(result, "fade_out_seconds", "fade_out", "fadeOutSeconds")
		if !hasIn {
			inSeconds, hasIn = firstNumericMapValue(call.Args, "fade_in_seconds", "fade_in", "fadeInSeconds")
		}
		if !hasOut {
			outSeconds, hasOut = firstNumericMapValue(call.Args, "fade_out_seconds", "fade_out", "fadeOutSeconds")
		}
		parts := []string{}
		if hasIn {
			parts = append(parts, fmt.Sprintf("fade in %.3fs", inSeconds))
		}
		if hasOut {
			parts = append(parts, fmt.Sprintf("fade out %.3fs", outSeconds))
		}
		if len(parts) > 0 {
			return fmt.Sprintf("Clip %s fade 已设置：%s。", clipID, strings.Join(parts, "，")), true
		}
	}
	return "Clip fade/gain set completed.", true
}

func messageLoopTrackGroupApplyControlReply(state *runState, call planner.ToolCall) (string, bool) {
	if state == nil || len(state.executed) == 0 || strings.TrimSpace(call.Tool) != "track.group.apply_control" {
		return "", false
	}
	mode := strings.ToLower(firstNonEmpty(firstMapText(call.Args, "mode", "operation"), "absolute"))
	control := strings.ToLower(firstNonEmpty(firstMapText(call.Args, "control", "param", "parameter"), "volume"))
	targetDB, targetOK := firstNumericMapValue(call.Args, "db", "target_db", "value_db", "volume_db")
	if control != "volume" && control != "track.volume" {
		return "", false
	}
	if mode != "absolute" && mode != "set" && mode != "volume_absolute" {
		return "", false
	}
	if !targetOK || mathAbs(targetDB) > 0.0001 {
		return "", false
	}
	record := state.executed[len(state.executed)-1]
	if !messageLoopExecutionSucceeded(record) {
		return "", false
	}
	result := messageLoopMapValue(record["result"])
	groupID := firstNonEmpty(firstMapText(result, "group_id", "id"), firstMapText(call.Args, "group_id", "id"))
	applied := firstPositiveMapInt(result, "applied_count")
	verified := firstPositiveMapInt(result, "verified_count")
	if applied == 0 {
		applied = len(messageLoopMapRows(result["members"]))
	}
	if verified == 0 {
		for _, member := range messageLoopMapRows(result["members"]) {
			if ok, exists := firstMapBool(member, "verified"); exists && ok {
				verified++
			}
		}
	}
	if groupID == "" {
		groupID = "B1 reference group"
	}
	return fmt.Sprintf("B1 参考电平校准第一步已执行：%s 的 %d 条成员轨道 fader 已绝对设置为 0 dB，其中 %d 条通过回读验证。", groupID, applied, verified), true
}

func messageLoopCurrentClipID(state *runState) string {
	if state == nil {
		return ""
	}
	for _, row := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["current_selection"]),
		messageLoopMapValue(state.input.Context["ui_context"]),
		state.input.ContextSnapshot,
		messageLoopMapValue(state.input.ContextSnapshot["current_selection"]),
		messageLoopMapValue(state.input.ContextSnapshot["ui_context"]),
		state.input.State,
		messageLoopMapValue(state.input.State["current_selection"]),
		messageLoopMapValue(state.input.State["ui_context"]),
	} {
		if clipID := firstMapText(row, "selected_clip_id", "primary_selected_clip_id", "piano_roll_focus_clip_id", "clip_id", "target_clip_id"); clipID != "" {
			return clipID
		}
		for _, value := range []any{row["selected_clip_ids"], row["clip_ids"]} {
			for _, item := range messageLoopAnySlice(value) {
				if clipID := strings.TrimSpace(fmt.Sprint(item)); clipID != "" && clipID != "<nil>" {
					return clipID
				}
			}
		}
	}
	return firstNonEmpty(
		state.executionMemory.ActiveWorkTargetClipID,
		state.executionMemory.LastCreatedClipID,
	)
}

func messageLoopCurrentClipTrackID(state *runState) string {
	if state == nil {
		return ""
	}
	for _, row := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["current_selection"]),
		messageLoopMapValue(state.input.Context["ui_context"]),
		state.input.ContextSnapshot,
		messageLoopMapValue(state.input.ContextSnapshot["current_selection"]),
		messageLoopMapValue(state.input.ContextSnapshot["ui_context"]),
		state.input.State,
		messageLoopMapValue(state.input.State["current_selection"]),
		messageLoopMapValue(state.input.State["ui_context"]),
	} {
		if trackID := firstMapText(row, "selected_clip_track_id", "selected_track_id", "track_id", "focused_track_id"); trackID != "" {
			return trackID
		}
	}
	return state.executionMemory.ActiveWorkTargetTrackID
}

func messageLoopSelectedClipRanges(state *runState) []map[string]any {
	if state == nil {
		return nil
	}
	for _, row := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["current_selection"]),
		messageLoopMapValue(state.input.Context["ui_context"]),
		state.input.ContextSnapshot,
		messageLoopMapValue(state.input.ContextSnapshot["current_selection"]),
		messageLoopMapValue(state.input.ContextSnapshot["ui_context"]),
		state.input.State,
		messageLoopMapValue(state.input.State["current_selection"]),
		messageLoopMapValue(state.input.State["ui_context"]),
	} {
		if ranges := messageLoopMapRows(row["selected_clip_ranges"]); len(ranges) > 0 {
			return messageLoopCompactClipRanges(ranges)
		}
		if single := messageLoopMapValue(row["selected_clip_range"]); len(single) > 0 {
			return messageLoopCompactClipRanges([]map[string]any{single})
		}
	}
	return nil
}

func messageLoopCompactClipRanges(ranges []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(ranges))
	for _, row := range ranges {
		compact := map[string]any{}
		for _, key := range []string{
			"range_id", "clip_id", "track_id",
			"start_seconds", "end_seconds", "duration_seconds",
			"clip_local_start_seconds", "clip_local_end_seconds",
			"local_start_seconds", "local_end_seconds",
		} {
			if value, ok := row[key]; ok && !messageLoopEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func messageLoopClipFadeGainReadReply(state *runState) string {
	if state == nil {
		return ""
	}
	var fade map[string]any
	var gain map[string]any
	var fadeErr string
	var gainErr string
	clipID := messageLoopCurrentClipID(state)
	for _, record := range state.executed {
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if name != "clip.fade.read" && name != "clip.gain.read" {
			continue
		}
		if !messageLoopExecutionSucceeded(record) {
			if errText := messageLoopText(record["error"]); errText != "" {
				if name == "clip.fade.read" {
					fadeErr = errText
				} else {
					gainErr = errText
				}
			}
			continue
		}
		result := messageLoopMapValue(record["result"])
		if id := firstMapText(result, "clip_id", "id", "item_id"); id != "" {
			clipID = id
		}
		if name == "clip.fade.read" {
			fade = result
		} else {
			gain = result
		}
	}
	if len(fade) == 0 && len(gain) == 0 && fadeErr == "" && gainErr == "" {
		return ""
	}
	lines := []string{}
	if clipID != "" {
		lines = append(lines, fmt.Sprintf("Clip %s \u5f53\u524d\u72b6\u6001\uff1a", clipID))
	} else {
		lines = append(lines, "\u5f53\u524d clip \u72b6\u6001\uff1a")
	}
	if len(fade) > 0 {
		inSeconds, _ := firstNumericMapValue(fade, "fade_in_seconds", "fade_in", "fadeInSeconds")
		outSeconds, _ := firstNumericMapValue(fade, "fade_out_seconds", "fade_out", "fadeOutSeconds")
		lines = append(lines, fmt.Sprintf("- Fade in\uff1a%.3fs", inSeconds))
		lines = append(lines, fmt.Sprintf("- Fade out\uff1a%.3fs", outSeconds))
	} else if fadeErr != "" {
		lines = append(lines, "- Fade\uff1a\u8bfb\u53d6\u5931\u8d25\uff0c"+fadeErr)
	}
	if len(gain) > 0 {
		if gainDB, ok := firstNumericMapValue(gain, "clip_gain_db", "gain_db", "db"); ok {
			lines = append(lines, fmt.Sprintf("- Clip gain\uff1a%+.2f dB", gainDB))
		} else {
			lines = append(lines, "- Clip gain\uff1a\u8bfb\u53d6\u6210\u529f\uff0c\u4f46\u7ed3\u679c\u91cc\u6ca1\u6709 dB \u6570\u503c")
		}
	} else if gainErr != "" {
		lines = append(lines, "- Clip gain\uff1a\u8bfb\u53d6\u5931\u8d25\uff0c"+gainErr)
	}
	return strings.Join(lines, "\n")
}

func messageLoopStripSilenceSuggestReply(state *runState) string {
	if state == nil {
		return ""
	}
	var result map[string]any
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if name != "clip.strip_silence.suggest" {
			continue
		}
		if !messageLoopExecutionSucceeded(record) {
			if errText := messageLoopText(record["error"]); errText != "" {
				return "清理静音建议失败：" + errText
			}
			return "清理静音建议失败。"
		}
		result = messageLoopMapValue(record["result"])
		break
	}
	if len(result) == 0 {
		return ""
	}
	params := messageLoopMapValue(result["recommended_params"])
	analysis := messageLoopMapValue(result["analysis"])
	threshold, _ := firstNumericMapValue(params, "threshold_dbfs")
	minSilence, _ := firstNumericMapValue(params, "min_silence_ms")
	startPad, _ := firstNumericMapValue(params, "clip_start_pad_ms")
	endPad, _ := firstNumericMapValue(params, "clip_end_pad_ms")
	stripCount := intNumberFromAny(firstNonEmptyAny(analysis["strip_region_count"], result["strip_region_count"]))
	actionCount := intNumberFromAny(result["pending_action_count"])
	confidence := firstNonEmpty(firstMapText(result, "confidence"), "unknown")
	scope := firstNonEmpty(firstMapText(result, "scope"), "selected_clip")

	lines := []string{
		"清理静音建议已生成。",
		fmt.Sprintf("- 范围：%s", messageLoopStripSilenceScopeLabel(scope)),
		fmt.Sprintf("- 推荐参数：阈值 %.1f dBFS，最短静音 %.0f ms，起始保留 %.0f ms，结束保留 %.0f ms", threshold, minSilence, startPad, endPad),
		fmt.Sprintf("- 预览结果：%d 个可清理区域，%d 个待确认动作", stripCount, actionCount),
		fmt.Sprintf("- 置信度：%s", confidence),
	}
	if risks := messageLoopStringRows(result["risks"], 2); len(risks) > 0 {
		lines = append(lines, "- 风险提示："+strings.Join(risks, "；"))
	}
	if actionCount > 0 {
		lines = append(lines, "如果你确认应用，我会使用这次分析返回的真实清理区域执行，不会重新猜区域。")
	} else {
		lines = append(lines, "当前没有可应用的清理动作。")
	}
	return strings.Join(lines, "\n")
}

func messageLoopStripSilenceScopeLabel(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "all_project":
		return "全工程音频片段"
	case "selected_ranges":
		return "当前选中片段范围"
	default:
		return "当前片段"
	}
}

func messageLoopStringRows(value any, limit int) []string {
	items := messageLoopAnySlice(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text == "" || text == "<nil>" {
			continue
		}
		out = append(out, text)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func firstNonEmptyAny(values ...any) any {
	for _, value := range values {
		if !messageLoopEmptyValue(value) {
			return value
		}
	}
	return nil
}

func intNumberFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, _ := typed.Int64()
		return int(n)
	default:
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return n
	}
}

type messageLoopImportPreflightCandidate struct {
	call       planner.ToolCall
	result     map[string]any
	toolCallID string
	traceIndex int
}

func (l *MessageLoop) preflightStemsFolderImport(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	call, ok := messageLoopDeterministicStemsPreflightCall(state)
	if !ok {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_stems_import_preflight", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic stems import preflight"},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: result.Error},
		)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "stems folder import request was routed through deterministic project.import_preflight",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false stems_import_preflight=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if handled, stopped, result := l.maybeStartStemsImportConfirmation(ctx, r, state, messageLoopOutput{}, "deterministic_preflight"); handled {
		return stopped, result
	}
	return false, Result{}
}

func (l *MessageLoop) preflightPendingSectionMarkersApply(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	call, ok := messageLoopDeterministicSectionMarkersApplyCall(state)
	if !ok {
		return false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_section_markers_apply", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		result := planner.ToolResult{ToolCallID: call.ID, Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call), Message: "deterministic A5 section marker apply"},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: result.Error},
		)
		state.input.Conversation = append(state.input.Conversation, llm.Message{Role: "user", Content: "<final_gate>" + result.Error + "</final_gate>"})
		return false, Result{}
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "confirmed A5/EPM section map was routed to project.markers.apply_section_markers",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, true, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=true section_markers_apply=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	if len(state.executed) > 0 {
		if reply := messageLoopProjectMarkersFastCompleteReply(state.executed[len(state.executed)-1]); strings.TrimSpace(reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic section marker write completed the turn"})
			return true, r.complete(state, reply)
		}
	}
	return true, r.complete(state, "已写入段落 marker。")
}

func messageLoopDeterministicSectionMarkersApplyCall(state *runState) (planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return planner.ToolCall{}, false
	}
	if messageLoopMutationBarrierActive(state) || messageLoopPlanMode(state.input.Context) {
		return planner.ToolCall{}, false
	}
	if len(state.executionMemory.PendingSectionMarkers) == 0 {
		return planner.ToolCall{}, false
	}
	if !messageLoopPendingSectionMarkersApplyRequest(firstNonEmpty(state.input.UserText, state.input.Summary)) {
		return planner.ToolCall{}, false
	}
	sections := pendingSectionMarkersToolSections(state.executionMemory.PendingSectionMarkers)
	if len(sections) == 0 {
		return planner.ToolCall{}, false
	}
	args := map[string]any{
		"sections":         sections,
		"replace_existing": true,
		"source":           firstNonEmpty(firstMapText(state.executionMemory.PendingSectionMarkers, "source"), "epm_a5"),
	}
	if replace, ok := firstMapBool(state.executionMemory.PendingSectionMarkers, "replace_existing"); ok {
		args["replace_existing"] = replace
	}
	command := cloneMap(args)
	command["cmd"] = "project.markers.apply_section_markers"
	return planner.ToolCall{
		ID:      "apply_pending_section_markers",
		Tool:    "project.markers.apply_section_markers",
		Args:    args,
		Command: command,
		Reason:  "User confirmed writing the pending A5/EPM section map as project markers.",
	}, true
}

func messageLoopPendingSectionMarkersApplyRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasSectionMarkerTarget := messageLoopTextHasAny(text,
		"marker", "markers", "section", "sections", "a5", "epm",
		"标记", "段落", "段落地图",
	)
	if !hasSectionMarkerTarget {
		return false
	}
	return messageLoopTextHasAny(text,
		"确认", "写入", "应用", "执行", "落地", "生成", "添加", "创建",
		"confirm", "write", "apply", "execute", "add", "create",
	)
}

func messageLoopDeterministicStemsPreflightCall(state *runState) (planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return planner.ToolCall{}, false
	}
	if messageLoopMutationBarrierActive(state) || messageLoopPlanMode(state.input.Context) {
		return planner.ToolCall{}, false
	}
	if !allowedTool("project.import_preflight", state.input.AllowedTools) || !allowedTool("project.import_folder_as_stems", state.input.AllowedTools) {
		return planner.ToolCall{}, false
	}
	userText := firstNonEmpty(state.input.UserText, state.input.Summary)
	if messageLoopUserRequestedMediaArtifacts(userText) && !messageLoopStemsFolderImportRequest(userText) {
		return planner.ToolCall{}, false
	}
	if !messageLoopStemsFolderImportRequest(state.input.UserText) && !messageLoopStemsFolderImportRequest(state.input.Summary) {
		return planner.ToolCall{}, false
	}
	if messageLoopHasImportPreflightAttempt(state) || messageLoopHasStemsImportAttemptAfter(state, -1) {
		return planner.ToolCall{}, false
	}
	folderPath, folderHint := messageLoopStemsImportFolderCandidate(state)
	if strings.TrimSpace(folderPath) == "" {
		return planner.ToolCall{}, false
	}
	if !messageLoopStemsImportFolderObject(folderPath, folderHint, state.input.UserText, state.input.Summary) {
		return planner.ToolCall{}, false
	}
	args := map[string]any{
		"folder_path":        folderPath,
		"recursive":          false,
		"media_kinds":        []string{"audio"},
		"intended_mode":      "stems_folder",
		"target_policy":      "create_tracks",
		"start_time_seconds": 0,
		"command_timeout_ms": 120000,
	}
	command := cloneMap(args)
	command["cmd"] = "project.import_preflight"
	return planner.ToolCall{
		ID:      "stems_import_preflight",
		Tool:    "project.import_preflight",
		Args:    args,
		Command: command,
		Reason:  "preflight a stems/multitrack folder before project-level batch import",
	}, true
}

func messageLoopStemsFolderImportRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasImport := messageLoopTextHasAny(text,
		"import", "importing", "add to project", "bring into project", "into the project", "create tracks",
		"\u5bfc\u5165", "\u532f\u5165", "\u8f7d\u5165", "\u52a0\u5165\u5de5\u7a0b", "\u52a0\u5230\u5de5\u7a0b", "\u5bfc\u5230\u5de5\u7a0b", "\u653e\u8fdb\u5de5\u7a0b",
	)
	if !hasImport {
		return false
	}
	hasFolderOrStems := messageLoopTextHasAny(text,
		"stem", "stems", "stem folder", "multi-track", "multitrack", "tracks out", "folder", "directory",
		"\u5206\u8f68", "\u591a\u8f68", "\u6587\u4ef6\u5939", "\u76ee\u5f55", "\u521b\u5efa\u8f68", "\u521b\u5efa\u8f68\u9053", "\u5efa\u8f68",
	)
	hasProjectTarget := messageLoopTextHasAny(text,
		"project", "create tracks", "add to project", "bring into project",
		"\u5de5\u7a0b", "\u521b\u5efa\u8f68", "\u521b\u5efa\u8f68\u9053", "\u5efa\u8f68",
	)
	return hasFolderOrStems || hasProjectTarget
}

func messageLoopStemsImportFolderPath(state *runState) string {
	path, _ := messageLoopStemsImportFolderCandidate(state)
	return path
}

func messageLoopStemsImportFolderCandidate(state *runState) (string, bool) {
	if state == nil {
		return "", false
	}
	for _, row := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["current_selection"]),
		state.input.ContextSnapshot,
		messageLoopMapValue(state.input.ContextSnapshot["current_selection"]),
		state.input.State,
	} {
		if path := firstMapText(row,
			"folder_path", "folder", "directory", "asset_folder", "source_folder_path",
			"selected_folder_path", "selected_directory_path", "selected_library_folder_path", "selected_asset_folder_path",
		); path != "" {
			return messageLoopCleanExtractedLocalPath(path), true
		}
		if path := firstMapText(row, "asset_location", "source_root", "asset_path", "selected_asset_path"); path != "" {
			return messageLoopCleanExtractedLocalPath(path), false
		}
	}
	return firstNonEmpty(
		messageLoopExtractLocalFolderPath(state.input.UserText),
		messageLoopExtractLocalFolderPath(state.input.Summary),
	), false
}

func messageLoopExtractLocalFolderPath(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if match := messageLoopQuotedLocalPathPattern.FindStringSubmatch(text); len(match) > 1 {
		return messageLoopCleanExtractedLocalPath(match[1])
	}
	if match := messageLoopLocalPathPattern.FindStringSubmatch(text); len(match) > 1 {
		return messageLoopCleanExtractedLocalPath(match[1])
	}
	return ""
}

func messageLoopCleanExtractedLocalPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, " \t\r\n\"'`“”‘’<>[]{}()（）【】，,。；;")
	if path == "" {
		return ""
	}
	for _, marker := range []string{
		" \u8fd9\u4e2a", " \u8fd9\u4e9b", " \u8be5", " \u8fd9\u4efd", " \u6587\u4ef6\u5939", " \u76ee\u5f55", " \u8def\u5f84", " \u5bfc\u5165", " \u52a0\u5165", " \u52a0\u5230", " \u653e\u8fdb",
		" into ", " to project", " as stems", " as stem", " import ", " please ",
	} {
		lower := strings.ToLower(path)
		if idx := strings.Index(lower, strings.ToLower(marker)); idx > 0 {
			path = strings.TrimSpace(path[:idx])
		}
	}
	return strings.Trim(path, " \t\r\n\"'`“”‘’<>[]{}()（）【】，,。；;")
}

func messageLoopStemsImportFolderObject(path string, folderHint bool, texts ...string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if info, err := os.Stat(path); err == nil {
		return info.IsDir()
	}
	if messageLoopPathLooksSingleMediaFile(path) {
		return false
	}
	if folderHint {
		return true
	}
	return messageLoopTextHasStemsFolderObjectHint(strings.Join(texts, "\n"))
}

func messageLoopTextHasStemsFolderObjectHint(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"stem", "stems", "stem folder", "multi-track", "multitrack", "tracks out", "folder", "directory",
		"\u5206\u8f68", "\u591a\u8f68", "\u6587\u4ef6\u5939", "\u76ee\u5f55",
	)
}

func messageLoopPathLooksSingleMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	switch ext {
	case ".wav", ".mp3", ".flac", ".aif", ".aiff", ".ogg", ".oga", ".m4a", ".wma",
		".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v",
		".mid", ".midi":
		return true
	default:
		return false
	}
}

func (l *MessageLoop) maybeStartStemsImportConfirmation(ctx context.Context, r *Runner, state *runState, out messageLoopOutput, gate string) (bool, bool, Result) {
	call, ok := messageLoopDeterministicStemsImportCall(state, out)
	if !ok {
		return false, false, Result{}
	}
	if stopped, result := r.checkpoint("before_message_loop_stems_import_confirmation", state); stopped {
		return true, true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, true, result
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "successful stems preflight was routed to pending import confirmation: " + strings.TrimSpace(gate),
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false stems_import_preflight=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, true, result
	}
	appendMessageLoopToolResult(state)
	return true, false, Result{}
}

func messageLoopDeterministicStemsImportCall(state *runState, out messageLoopOutput) (planner.ToolCall, bool) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return planner.ToolCall{}, false
	}
	if messageLoopMutationBarrierActive(state) || messageLoopPlanMode(state.input.Context) {
		return planner.ToolCall{}, false
	}
	if !allowedTool("project.import_folder_as_stems", state.input.AllowedTools) {
		return planner.ToolCall{}, false
	}
	preflight, ok := messageLoopLastSuccessfulImportPreflight(state)
	if !ok || messageLoopHasStemsImportAttemptAfter(state, preflight.traceIndex) {
		return planner.ToolCall{}, false
	}
	if !messageLoopPreflightCanBecomeStemsImport(state, preflight, out) {
		return planner.ToolCall{}, false
	}
	rows := messageLoopImportRows(preflight)
	folderPath := firstNonEmpty(
		firstMapText(preflight.call.Args, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
		firstMapText(preflight.call.Command, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
		messageLoopFirstImportText(rows, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
	)
	if strings.TrimSpace(folderPath) == "" {
		return planner.ToolCall{}, false
	}
	args := cloneMap(preflight.call.Args)
	if args == nil {
		args = map[string]any{}
	}
	delete(args, "cmd")
	delete(args, "command")
	delete(args, "tool")
	args["folder_path"] = folderPath
	args["target_policy"] = "create_tracks"
	args["start_time_seconds"] = messageLoopFirstImportValueOrDefault(rows, 0.0, "start_time_seconds", "start_time", "offset_time", "start", "start_seconds", "position_seconds", "time")
	args["command_timeout_ms"] = messageLoopFirstImportValueOrDefault(rows, 120000, "command_timeout_ms")
	if value, ok := messageLoopFirstImportValue(rows, "recursive"); ok {
		args["recursive"] = value
	}
	if value, ok := messageLoopFirstImportValue(rows, "skip_unreadable"); ok {
		args["skip_unreadable"] = value
	}
	if decision := messageLoopSampleRateDecisionFromPreflight(preflight); len(decision) > 0 {
		args["sample_rate_decision"] = decision
		if messageLoopUserExplicitlyRequestsProjectSampleRateSwitch(state) && allowedTool("project.set_audio_settings", state.input.AllowedTools) {
			if patch := messageLoopProjectAudioSettingsPatchFromDecision(decision); len(patch) > 0 {
				args["project_audio_settings_patch"] = patch
				args["apply_project_audio_settings_patch"] = true
			}
		}
	}
	command := cloneMap(args)
	command["cmd"] = "project.import_folder_as_stems"
	id := strings.TrimSpace(preflight.call.ID)
	if id == "" {
		id = strings.TrimSpace(preflight.toolCallID)
	}
	if id == "" {
		id = "stems_import_after_preflight"
	} else {
		id += "_import"
	}
	return planner.ToolCall{
		ID:      id,
		Tool:    "project.import_folder_as_stems",
		Args:    args,
		Command: command,
		Reason:  "create one audio track and clip per readable file from the preflighted stems folder",
	}, true
}

func messageLoopSampleRateDecisionFromPreflight(preflight messageLoopImportPreflightCandidate) map[string]any {
	result := preflight.result
	plan := messageLoopMapValue(result["import_plan"])
	summary := messageLoopMapValue(result["summary"])
	for _, row := range []map[string]any{result, plan, summary} {
		if decision := messageLoopMapValue(row["sample_rate_decision"]); len(decision) > 0 {
			return cloneMap(decision)
		}
	}
	return nil
}

func messageLoopSampleRateDecisionRecommendsSwitch(decision map[string]any) bool {
	if len(decision) == 0 {
		return false
	}
	recommended := strings.ToLower(firstMapText(decision, "recommended_action"))
	if recommended == "switch_project_to_source_rate_then_import" || recommended == "switch_project_sample_rate_then_import" {
		return true
	}
	canSwitch, ok := firstMapBool(decision, "can_switch_project_sample_rate")
	return ok && canSwitch
}

func messageLoopUserExplicitlyRequestsProjectSampleRateSwitch(state *runState) bool {
	if state == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(firstNonEmpty(state.input.UserText, state.input.Summary)))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"switch project sample rate", "change project sample rate", "match source sample rate", "match the source sample rate",
		"set project sample rate", "use source sample rate", "use the source sample rate",
		"切换工程采样率", "更改工程采样率", "修改工程采样率", "匹配素材采样率", "使用素材采样率", "用素材采样率",
		"跟随素材采样率", "工程采样率跟随", "切到素材采样率",
	)
}

func messageLoopProjectAudioSettingsPatchFromDecision(decision map[string]any) map[string]any {
	if len(decision) == 0 {
		return nil
	}
	return cloneMap(messageLoopMapValue(decision["project_audio_settings_patch"]))
}

func messageLoopProjectAudioSettingsPatchForConfirmedImport(call planner.ToolCall) map[string]any {
	if !messageLoopIsStemsImportName(call.Tool) && !messageLoopIsStemsImportName(firstMapText(call.Args, "cmd")) && !messageLoopIsStemsImportName(firstMapText(call.Command, "cmd")) {
		return nil
	}
	for _, row := range []map[string]any{call.Args, call.Command} {
		applyPatch, ok := firstMapBool(row, "apply_project_audio_settings_patch")
		if !ok || !applyPatch {
			continue
		}
		if patch := messageLoopMapValue(row["project_audio_settings_patch"]); len(patch) > 0 {
			return cloneMap(patch)
		}
	}
	return nil
}

func messageLoopSetAudioSettingsCallForImport(importCall planner.ToolCall, patch map[string]any) planner.ToolCall {
	args := map[string]any{
		"audio_settings":                  patch,
		"change_origin":                   "stems_import_sample_rate_preflight",
		"allow_import_sample_rate_switch": true,
	}
	command := cloneMap(args)
	command["cmd"] = "project.set_audio_settings"
	id := strings.TrimSpace(importCall.ID)
	if id == "" {
		id = "stems_import"
	}
	return planner.ToolCall{
		ID:         id + "_set_audio_settings",
		Tool:       "project.set_audio_settings",
		Args:       args,
		Command:    command,
		Reason:     "apply the preflight-approved project sample-rate metadata before importing stems",
		PlanItemID: importCall.PlanItemID,
	}
}

func messageLoopAnnotateLastExecutionResultWithSampleRatePatch(state *runState, patch map[string]any) {
	if state == nil || len(state.executed) == 0 || len(patch) == 0 {
		return
	}
	record := state.executed[len(state.executed)-1]
	result := messageLoopMapValue(record["result"])
	if len(result) == 0 {
		return
	}
	result["project_audio_settings_patch"] = cloneMap(patch)
	result["project_audio_settings_changed_before_import"] = true
	record["result"] = result
	state.executed[len(state.executed)-1] = record
}

func messageLoopLastSuccessfulImportPreflight(state *runState) (messageLoopImportPreflightCandidate, bool) {
	if state == nil {
		return messageLoopImportPreflightCandidate{}, false
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		event := state.trace[i]
		if event.ToolResult == nil || !messageLoopIsImportPreflightName(event.ToolResult.Tool) || toolStatusFailed(event.ToolResult.Status) {
			continue
		}
		result := messageLoopMapValue(event.ToolResult.Result)
		if !messageLoopImportPreflightResultOK(result) {
			continue
		}
		call := messageLoopTraceToolCallForResult(state.trace, i, event.ToolResult.ToolCallID, "project.import_preflight")
		return messageLoopImportPreflightCandidate{
			call:       call,
			result:     result,
			toolCallID: strings.TrimSpace(event.ToolResult.ToolCallID),
			traceIndex: i,
		}, true
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) || !messageLoopIsImportPreflightName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if !messageLoopImportPreflightResultOK(result) {
			continue
		}
		toolCallID := messageLoopText(record["tool_call_id"])
		return messageLoopImportPreflightCandidate{
			call:       messageLoopTraceToolCallForResult(state.trace, len(state.trace), toolCallID, "project.import_preflight"),
			result:     result,
			toolCallID: toolCallID,
			traceIndex: -1,
		}, true
	}
	return messageLoopImportPreflightCandidate{}, false
}

func messageLoopTraceToolCallForResult(trace []planner.TraceEvent, before int, toolCallID, fallbackTool string) planner.ToolCall {
	if before > len(trace) || before < 0 {
		before = len(trace)
	}
	toolCallID = strings.TrimSpace(toolCallID)
	for i := before - 1; i >= 0; i-- {
		if trace[i].ToolCall == nil {
			continue
		}
		call := *trace[i].ToolCall
		if toolCallID != "" && strings.TrimSpace(call.ID) == toolCallID {
			return call
		}
	}
	for i := before - 1; i >= 0; i-- {
		if trace[i].ToolCall == nil {
			continue
		}
		call := *trace[i].ToolCall
		if messageLoopIsImportPreflightName(firstNonEmpty(call.Tool, firstMapText(call.Args, "cmd"), firstMapText(call.Command, "cmd"), fallbackTool)) {
			return call
		}
	}
	return planner.ToolCall{Tool: fallbackTool}
}

func messageLoopPreflightCanBecomeStemsImport(state *runState, preflight messageLoopImportPreflightCandidate, out messageLoopOutput) bool {
	if messageLoopImportPreflightTracksToCreate(preflight.result) <= 0 {
		return false
	}
	rows := messageLoopImportRows(preflight)
	if firstNonEmpty(
		firstMapText(preflight.call.Args, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
		firstMapText(preflight.call.Command, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
		messageLoopFirstImportText(rows, "folder_path", "folder", "directory", "asset_location", "asset_folder"),
	) == "" {
		return false
	}
	if messageLoopStemsImportIntent(state.input.UserText) ||
		messageLoopStemsImportIntent(state.input.Summary) ||
		messageLoopStemsImportIntent(preflight.call.Reason) ||
		messageLoopStemsImportIntent(firstNonEmpty(out.Reply, out.ClarificationQuestion)) ||
		messageLoopUserConfirmsStemsImport(state.input.UserText) {
		return true
	}
	mode := strings.ToLower(messageLoopFirstImportText(rows, "intended_mode", "mode", "import_mode"))
	targetPolicy := strings.ToLower(messageLoopFirstImportText(rows, "target_policy"))
	return strings.Contains(mode, "stems") || strings.Contains(mode, "folder") || targetPolicy == "create_tracks"
}

func messageLoopHasStemsImportAttemptAfter(state *runState, traceIndex int) bool {
	if state == nil {
		return false
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		if traceIndex >= 0 && i <= traceIndex {
			break
		}
		event := state.trace[i]
		if event.ToolCall != nil && messageLoopIsStemsImportName(firstNonEmpty(event.ToolCall.Tool, firstMapText(event.ToolCall.Args, "cmd"), firstMapText(event.ToolCall.Command, "cmd"))) {
			return true
		}
		if event.ToolResult != nil && messageLoopIsStemsImportName(firstNonEmpty(event.ToolResult.Tool, firstMapText(event.ToolResult.Result, "command", "cmd"))) {
			return true
		}
	}
	for _, record := range state.executed {
		if messageLoopIsStemsImportName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			return true
		}
		if messageLoopIsStemsImportName(firstMapText(messageLoopMapValue(record["result"]), "command", "cmd")) {
			return true
		}
	}
	return false
}

func messageLoopHasImportPreflightAttempt(state *runState) bool {
	if state == nil {
		return false
	}
	for _, event := range state.trace {
		if event.ToolCall != nil && messageLoopIsImportPreflightName(firstNonEmpty(event.ToolCall.Tool, firstMapText(event.ToolCall.Args, "cmd"), firstMapText(event.ToolCall.Command, "cmd"))) {
			return true
		}
		if event.ToolResult != nil && messageLoopIsImportPreflightName(event.ToolResult.Tool) {
			return true
		}
	}
	for _, record := range state.executed {
		if messageLoopIsImportPreflightName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			return true
		}
	}
	return false
}

func messageLoopImportPreflightResultOK(result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	status := strings.ToLower(firstNonEmpty(firstMapText(result, "status"), "ok"))
	if toolStatusFailed(status) {
		return false
	}
	return messageLoopImportPreflightTracksToCreate(result) > 0
}

func messageLoopImportPreflightTracksToCreate(result map[string]any) int {
	if len(result) == 0 {
		return 0
	}
	summary := messageLoopMapValue(result["summary"])
	plan := messageLoopMapValue(result["import_plan"])
	for _, value := range []any{
		plan["tracks_to_create"],
		summary["tracks_to_create"],
		summary["readable_file_count"],
		result["tracks_to_create"],
		result["readable_file_count"],
	} {
		if n := int(messageLoopImportNumber(value)); n > 0 {
			return n
		}
	}
	return 0
}

func messageLoopImportRows(preflight messageLoopImportPreflightCandidate) []map[string]any {
	result := preflight.result
	plan := messageLoopMapValue(result["import_plan"])
	summary := messageLoopMapValue(result["summary"])
	return []map[string]any{
		preflight.call.Args,
		preflight.call.Command,
		plan,
		summary,
		result,
	}
}

func messageLoopFirstImportText(rows []map[string]any, keys ...string) string {
	value, ok := messageLoopFirstImportValue(rows, keys...)
	if !ok {
		return ""
	}
	return messageLoopText(value)
}

func messageLoopFirstImportValueOrDefault(rows []map[string]any, fallback any, keys ...string) any {
	if value, ok := messageLoopFirstImportValue(rows, keys...); ok {
		return value
	}
	return fallback
}

func messageLoopFirstImportValue(rows []map[string]any, keys ...string) (any, bool) {
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		for _, key := range keys {
			if value, ok := row[key]; ok && !messageLoopEmptyValue(value) {
				return value, true
			}
		}
	}
	return nil, false
}

func messageLoopImportNumber(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case json.Number:
		n, _ := typed.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return n
	default:
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return n
	}
}

func messageLoopIsImportPreflightName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "project.import_preflight":
		return true
	default:
		return false
	}
}

func messageLoopIsStemsImportName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "project.import_folder_as_stems":
		return true
	default:
		return false
	}
}

func messageLoopStemsImportIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"import", "importing", "stem", "stems", "stem folder", "multi-track", "multitrack", "create tracks", "add to project", "bring into project",
		"\u5bfc\u5165", "\u532f\u5165", "\u8f7d\u5165", "\u52a0\u5165\u5de5\u7a0b", "\u52a0\u5230\u5de5\u7a0b", "\u5bfc\u5230\u5de5\u7a0b",
		"\u5206\u8f68", "\u591a\u8f68", "\u8f68\u9053", "\u521b\u5efa\u8f68", "\u521b\u5efa\u8f68\u9053", "\u5efa\u8f68",
	)
}

func messageLoopUserConfirmsStemsImport(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"confirm", "confirmed", "yes", "ok", "okay", "go ahead", "continue", "do it", "execute", "import it", "import them",
		"\u786e\u8ba4", "\u53ef\u4ee5", "\u597d", "\u597d\u7684", "\u7ee7\u7eed", "\u6267\u884c", "\u5f00\u59cb", "\u5bfc\u5165\u5427", "\u5c31\u8fd9\u6837",
	)
}

func (l *MessageLoop) preflightNaturalMixObservation(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || messageLoopFreeStateActive(state) || !messageLoopNeedsDeterministicMixObservation(state) {
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
	if messageLoopClipFadeGainRequest(state.input.UserText) {
		return false
	}
	if messageLoopGainStagingCapabilityRequest(state.input.UserText) {
		return false
	}
	needsRealtimeRefresh := messageLoopRealtimeObservationRequest(state.input.UserText)
	if messageLoopObservationPackageReadRequest(state.input.UserText) && observationIsMixObservation(state.recentObservation) && !needsRealtimeRefresh {
		return false
	}
	if !messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopAudioObservationRequest(state.input.UserText) {
		return false
	}
	if messageLoopExplicitPluginRequest(state) && !messageLoopLowMudPluginPrepForState(state) && !messageLoopMutationBarrierActive(state) {
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
	trackID := firstNonEmpty(
		messageLoopExactSelectedTrackID(state),
		state.executionMemory.ActiveWorkTargetTrackID,
	)
	if scope == "full_project" && trackID != "" && (messageLoopOrdinarySemanticEQRequest(state) || messageLoopSemanticEQNeedsPluginSelection(state)) {
		scope = "full_project_with_focus_track"
		args["scope"] = scope
	}
	if scope != "full_project" {
		if trackID != "" {
			setIfEmpty(args, "track_id", trackID)
		}
		if scope == "full_project_with_focus_track" && trackID != "" {
			setIfEmptyMap(args, "focus_hint", map[string]any{
				"track_id": trackID,
				"source":   "exact_selected_track",
			})
		}
	}
	if scope != "full_project" && scope != "full_project_with_focus_track" {
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

func messageLoopExactSelectedTrackID(state *runState) string {
	if state == nil {
		return ""
	}
	for _, source := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["current_selection"]),
		messageLoopMapValue(state.input.Context["ui_context"]),
		state.input.ContextSnapshot,
		messageLoopMapValue(state.input.ContextSnapshot["current_selection"]),
		messageLoopMapValue(state.input.ContextSnapshot["ui_context"]),
		state.input.State,
		messageLoopMapValue(state.input.State["current_selection"]),
		messageLoopMapValue(state.input.State["ui_context"]),
	} {
		if trackID := firstMapText(source, "selected_track_id", "selected_clip_track_id", "focused_track_id"); trackID != "" {
			return trackID
		}
	}
	onlyTrackID := ""
	for _, source := range []map[string]any{state.input.Context, state.input.ContextSnapshot, state.input.State} {
		for _, track := range messageLoopMapRows(source["tracks"]) {
			trackID := firstMapText(track, "track_id", "id", "item_id")
			if trackID == "" {
				continue
			}
			if onlyTrackID != "" && onlyTrackID != trackID {
				return ""
			}
			onlyTrackID = trackID
		}
	}
	return onlyTrackID
}

func messageLoopDeterministicGainPendingAfterObservation(state *runState) (string, bool) {
	if state == nil || messageLoopExplicitPluginRequest(state) || messageLoopExplicitMixExecutionConfirmation(state.input.UserText) {
		return "", false
	}
	if messageLoopClipFadeGainRequest(state.input.UserText) {
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
	if gate, blocked := messageLoopActionWorkflowPreflightGate(state); blocked {
		return actionworkflow.RenderBlockedPreflightReply(gate), true
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
	return messageLoopPendingMixTreatmentReply(treatment), true
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
	if state == nil || !messageLoopIsWaveformBakeTool(call) {
		return call
	}
	// Redirect whenever the user text reads as an audio-observation request,
	// OR the call is structurally malformed (track_id without clip_id/file_path)
	// regardless of user text — a bare warm_waveform_bake call like that always
	// fails in the kernel (ImportService::handleWarmWaveformBake), so there is
	// no case where forwarding it unmodified is useful.
	if !messageLoopAudioObservationRequest(state.input.UserText) && !messageLoopWaveformBakeCallMissingClipSource(call) {
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
	if state == nil || messageLoopExplicitPluginRequest(state) {
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
	if pending, ok := messageLoopStripSilencePendingConfirmationCall(call, result); ok {
		return pending
	}
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

func messageLoopStripSilencePendingConfirmationCall(call planner.ToolCall, result executorpkg.Result) (planner.ToolCall, bool) {
	actions := messageLoopStripSilencePendingActionCalls(call, result.Result)
	if len(actions) == 0 {
		return planner.ToolCall{}, false
	}
	if len(actions) == 1 {
		if messageLoopStripSilenceShouldBatchSingleAction(result.Result) {
			batch := messageLoopStripSilenceBatchCallFromActions(call, actions)
			messageLoopAnnotateStripSilenceBatchSourceStats(batch.Args, result.Result, len(actions))
			batch.Command = cloneMap(batch.Args)
			return batch, true
		}
		return actions[0], true
	}
	batch := messageLoopStripSilenceBatchCallFromActions(call, actions)
	messageLoopAnnotateStripSilenceBatchSourceStats(batch.Args, result.Result, len(actions))
	batch.Command = cloneMap(batch.Args)
	return batch, true
}

func messageLoopStripSilenceShouldBatchSingleAction(result map[string]any) bool {
	if result == nil {
		return false
	}
	if firstPositiveMapInt(result, "target_count", "analyzed_clip_count") > 1 {
		return true
	}
	if firstPositiveMapInt(result, "no_cleanup_needed_clip_count", "analysis_failed_clip_count") > 0 {
		return true
	}
	analysis := messageLoopMapValue(result["analysis"])
	return firstPositiveMapInt(analysis, "analysis_count") > 1 ||
		firstPositiveMapInt(analysis, "no_cleanup_needed_clip_count", "analysis_failed_clip_count") > 0
}

func messageLoopStripSilenceBatchCallFromActions(parent planner.ToolCall, actions []planner.ToolCall) planner.ToolCall {
	args := map[string]any{
		"cmd": "clip.strip_silence.apply_batch",
	}
	args["pending_action_count"] = len(actions)
	args["pending_actions"] = messageLoopStripSilenceActionCallsAsRows(actions)
	messageLoopCopyStripSilenceBatchMetadata(args, parent.Args)
	command := cloneMap(args)
	return planner.ToolCall{
		ID:      firstNonEmpty(parent.ID, "apply_strip_silence_batch"),
		Tool:    "clip.strip_silence.apply_batch",
		Args:    args,
		Command: command,
		Reason:  fmt.Sprintf("apply %d confirmed Strip Silence action(s)", len(actions)),
	}
}

func messageLoopAnnotateStripSilenceBatchSourceStats(args map[string]any, result map[string]any, actionCount int) {
	if args == nil || result == nil {
		return
	}
	messageLoopCopyStripSilenceBatchMetadata(args, result)
	analysis := messageLoopMapValue(result["analysis"])
	messageLoopCopyStripSilenceBatchMetadata(args, analysis)
	if scanned := firstPositiveMapInt(result, "target_count"); scanned > 0 {
		args["scanned_clip_count"] = scanned
		args["target_count"] = scanned
	}
	analyzed := firstPositiveMapInt(result, "analyzed_clip_count")
	if analyzed <= 0 {
		analyzed = firstPositiveMapInt(analysis, "analysis_count")
	}
	if analyzed > 0 {
		args["analyzed_clip_count"] = analyzed
		if _, exists := args["no_cleanup_needed_clip_count"]; !exists {
			noCleanup := analyzed - actionCount
			if noCleanup < 0 {
				noCleanup = 0
			}
			args["no_cleanup_needed_clip_count"] = noCleanup
		}
	}
}

func messageLoopCopyStripSilenceBatchMetadata(dst map[string]any, src map[string]any) {
	if dst == nil || src == nil {
		return
	}
	for _, key := range []string{
		"scope",
		"target_count",
		"scanned_clip_count",
		"analyzed_clip_count",
		"analysis_failed_clip_count",
		"no_cleanup_needed_clip_count",
	} {
		if value, ok := src[key]; ok {
			dst[key] = value
		}
	}
}

func messageLoopStripSilencePendingActionCalls(parent planner.ToolCall, result map[string]any) []planner.ToolCall {
	actions := messageLoopStripSilencePendingActionRows(result)
	out := make([]planner.ToolCall, 0, len(actions))
	baseID := firstNonEmpty(parent.ID, "apply_strip_silence")
	for i, action := range actions {
		tool := firstNonEmpty(firstMapText(action, "tool_name", "tool"), firstMapText(messageLoopMapValue(action["args"]), "cmd", "command"))
		if !strings.EqualFold(strings.TrimSpace(tool), "clip.strip_silence.apply") {
			continue
		}
		args := cloneMap(messageLoopMapValue(action["args"]))
		if len(args) == 0 {
			args = cloneMap(action)
		}
		delete(args, "tool_name")
		delete(args, "tool")
		if len(messageLoopMapRows(args["strip_regions"])) == 0 {
			continue
		}
		args["cmd"] = "clip.strip_silence.apply"
		command := cloneMap(args)
		command["cmd"] = "clip.strip_silence.apply"
		out = append(out, planner.ToolCall{
			ID:      fmt.Sprintf("%s_apply_%02d", baseID, i+1),
			Tool:    "clip.strip_silence.apply",
			Args:    args,
			Command: command,
			Reason:  firstNonEmpty(firstMapText(action, "reason", "summary"), "apply confirmed Strip Silence preview"),
		})
	}
	return out
}

func messageLoopStripSilencePendingActionRows(result map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	seen := map[string]bool{}
	add := func(row map[string]any) {
		if len(row) == 0 {
			return
		}
		key := messageLoopStripSilencePendingActionKey(row)
		if key != "" {
			if seen[key] {
				return
			}
			seen[key] = true
		}
		rows = append(rows, row)
	}
	if row := messageLoopMapValue(result["pending_action"]); len(row) > 0 {
		add(row)
	}
	for _, row := range messageLoopMapRows(result["pending_actions"]) {
		add(row)
	}
	return rows
}

func messageLoopStripSilencePendingActionKey(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	args := messageLoopMapValue(row["args"])
	if len(args) == 0 {
		args = row
	}
	tool := strings.ToLower(strings.TrimSpace(firstNonEmpty(firstMapText(row, "tool_name", "tool"), firstMapText(args, "cmd", "command"))))
	payload := map[string]any{
		"tool":          tool,
		"clip_id":       firstMapText(args, "clip_id"),
		"track_id":      firstMapText(args, "track_id"),
		"analysis_id":   firstMapText(args, "analysis_id"),
		"strip_regions": messageLoopMapRows(args["strip_regions"]),
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) == 0 {
		return fmt.Sprintf("%s|%s|%s|%s|%d", tool, payload["clip_id"], payload["track_id"], payload["analysis_id"], len(messageLoopMapRows(args["strip_regions"])))
	}
	return string(data)
}

func messageLoopStripSilenceActionCallsAsRows(calls []planner.ToolCall) []map[string]any {
	rows := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		rows = append(rows, map[string]any{
			"tool_name": call.Tool,
			"tool":      call.Tool,
			"args":      cloneMap(call.Args),
			"reason":    strings.TrimSpace(call.Reason),
		})
	}
	return rows
}

func messageLoopStripSilenceBundleCalls(call planner.ToolCall) ([]planner.ToolCall, bool) {
	bundled, _ := firstMapBool(call.Args, "_strip_silence_pending_bundle")
	if !bundled {
		return nil, false
	}
	rows := messageLoopMapRows(call.Args["pending_actions"])
	if len(rows) == 0 {
		return nil, true
	}
	calls := messageLoopStripSilencePendingActionCalls(call, map[string]any{"pending_actions": rows})
	return calls, true
}

func messageLoopStripSilenceBundleCompleteReply(calls []planner.ToolCall) string {
	return fmt.Sprintf(
		"\u5df2\u6267\u884c\u7247\u6bb5\u6e05\u7406\uff1a\u5df2\u5bf9 %d \u4e2a clip \u5e94\u7528 Strip Silence\uff0c\u5171\u5220\u9664 %d \u4e2a\u9759\u97f3\u533a\u57df\u3002",
		len(calls),
		messageLoopStripSilenceApplyRegionCount(calls),
	)
}

func messageLoopStripSilenceApplyRegionCount(calls []planner.ToolCall) int {
	total := 0
	for _, call := range calls {
		total += len(messageLoopMapRows(call.Args["strip_regions"]))
	}
	return total
}

func messageLoopIsStripSilenceApplyCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	return name == "clip.strip_silence.apply" || name == "clip.strip_silence.apply_batch"
}

func messageLoopStripSilenceApplyCompleteReply(record map[string]any, call planner.ToolCall) string {
	result := messageLoopMapValue(record["result"])
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "clip.strip_silence.apply_batch" {
		clipCount := firstPositiveMapInt(result, "applied_clip_count", "affected_clip_count")
		if clipCount <= 0 {
			clipCount = firstPositiveMapInt(result, "pending_action_count")
		}
		regionCount := firstPositiveMapInt(result, "applied_region_count", "deleted_region_count", "strip_region_count")
		if regionCount <= 0 {
			for _, row := range messageLoopMapRows(call.Args["pending_actions"]) {
				args := messageLoopMapValue(row["args"])
				if len(args) == 0 {
					args = row
				}
				regionCount += len(messageLoopMapRows(args["strip_regions"]))
			}
		}
		scannedCount := firstPositiveMapInt(result, "scanned_clip_count", "target_count", "analyzed_clip_count")
		noCleanupCount := firstPositiveMapInt(result, "no_cleanup_needed_clip_count", "no_cleanup_clip_count")
		skippedCount := firstPositiveMapInt(result, "protected_skip_clip_count", "skipped_clip_count")
		analysisFailedCount := firstPositiveMapInt(result, "analysis_failed_clip_count")
		failedCount := firstPositiveMapInt(result, "true_failed_clip_count", "failed_clip_count", "error_count")
		if analysisFailedCount > 0 {
			failedCount += analysisFailedCount
		}
		if scannedCount > 0 {
			details := []string{}
			if noCleanupCount > 0 {
				details = append(details, fmt.Sprintf("%d 个无需清理", noCleanupCount))
			}
			if skippedCount > 0 {
				details = append(details, fmt.Sprintf("%d 个保护跳过", skippedCount))
			}
			if failedCount > 0 || skippedCount > 0 {
				details = append(details, fmt.Sprintf("%d 个真正失败", failedCount))
			}
			reply := fmt.Sprintf("已执行片段清理：扫描 %d 个 clip，成功清理 %d 个，删除 %d 个静音区域", scannedCount, clipCount, regionCount)
			if len(details) > 0 {
				reply += "；" + strings.Join(details, "，")
			}
			return reply + "。"
		}
		if failedCount > 0 || skippedCount > 0 {
			details := []string{}
			if skippedCount > 0 {
				details = append(details, fmt.Sprintf("%d 个保护跳过", skippedCount))
			}
			if failedCount > 0 {
				details = append(details, fmt.Sprintf("%d 个真正失败", failedCount))
			}
			return fmt.Sprintf("已执行片段清理：成功清理 %d 个 clip，删除 %d 个静音区域；%s。", clipCount, regionCount, strings.Join(details, "，"))
		}
		return fmt.Sprintf("已执行片段清理：已处理 %d 个 clip，共删除 %d 个静音区域。", clipCount, regionCount)
	}
	count := firstPositiveMapInt(result, "deleted_region_count", "applied_region_count", "strip_region_count")
	if count <= 0 {
		count = len(messageLoopMapRows(call.Args["strip_regions"]))
	}
	clipID := firstNonEmpty(firstMapText(result, "clip_id"), firstMapText(call.Args, "clip_id"))
	if clipID == "" {
		clipID = "clip"
	}
	return fmt.Sprintf("\u5df2\u6267\u884c\u7247\u6bb5\u6e05\u7406\uff1a%s \u5df2\u5220\u9664 %d \u4e2a\u9759\u97f3\u533a\u57df\u3002", clipID, count)
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
	if (!messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopImplicitPanFollowupRequest(state) && !messageLoopExplicitMixExecutionConfirmation(state.input.UserText)) || messageLoopExplicitPluginRequest(state) {
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
	return messageLoopStripExecutionQuestion(reply), true
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
	return actionworkflow.RenderReply(messageLoopActionWorkflowSpecFromTreatment(*treatment), actionworkflow.PreflightGate{CanCreateExecutablePending: true, CanSuggest: true})
}

func messageLoopConservativeLowMudTreatmentPendingReply(treatment *MixTreatmentPending) string {
	if treatment == nil {
		return "结论：低频处理只能先作为保守 EQ 预备方案。\n\n依据：当前观察不足以直接支持精确处理。\n\n待确认动作：无。\n\n限制：确认前不会加载插件或写入参数。"
	}
	return actionworkflow.RenderReply(messageLoopActionWorkflowSpecFromTreatment(*treatment), actionworkflow.PreflightGate{CanCreateExecutablePending: true, CanSuggest: true})
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
	if messageLoopClipFadeGainRequest(state.input.UserText) {
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
	if messageLoopClipFadeGainRequest(text) {
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
	if messageLoopNeedsNeutralFamilyProjection(state) {
		return l.assemblyNeutralFamilySelection(state, snapshotJSON)
	}
	system := messageLoopSystemPrompt(state)
	user := fmt.Sprintf("Current Goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nCurrent context snapshot JSON:\n%s", strings.TrimSpace(state.input.UserText), state.goal.GoalID, state.goal.RunID, state.budget.MaxToolCalls-state.toolCallsUsed, snapshotJSON)
	if freeState := messageLoopFreeStatePromptContext(state); len(freeState) > 0 {
		data, _ := json.Marshal(freeState)
		user += "\nFree-state reasoning ledger JSON:\n" + string(data)
	}
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

func (l *MessageLoop) assemblyNeutralFamilySelection(state *runState, snapshotJSON string) promptruntime.Assembly {
	// FULLACCESS-AUTONOMY-1: this is the production assembly path for every
	// active free-state turn (messageLoopNeedsNeutralFamilyProjection mirrors
	// messageLoopFreeStateActive), so the full-access autonomy directive must be
	// appended here as well as inside messageLoopSystemPrompt. It renders "" for
	// every manual/ordinary turn, keeping those prompts byte-identical.
	system := messageLoopNeutralFamilySystemPrompt(state) + messageLoopFullAccessAutonomyRules(state)
	user := fmt.Sprintf("Current acoustic goal: %s\nGoalID: %s\nRunID: %s\nRemaining tool calls this run: %d\nNeutral context snapshot JSON:\n%s",
		strings.TrimSpace(state.input.UserText), state.goal.GoalID, state.goal.RunID,
		state.budget.MaxToolCalls-state.toolCallsUsed, snapshotJSON)
	if freeState := messageLoopFreeStatePromptContext(state); len(freeState) > 0 {
		data, _ := json.Marshal(freeState)
		user += "\nFree-state reasoning ledger JSON:\n" + string(data)
	}
	return promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "message_loop_neutral_family_selection", "", system, true),
		},
		// Prior assistant turns can contain materialization identity. Preserve
		// only the most recent final-gate feedback: it is workflow control
		// feedback, not materialization context, and must be visible to the next
		// neutral decision request.
		History: messageLoopNeutralFamilyFeedbackHistory(state.input.Conversation),
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionRuntime, "message_loop_neutral_family_runtime", "", user, false),
		},
	})
}

func messageLoopNeutralFamilyFeedbackHistory(conversation []llm.Message) []llm.Message {
	for index := len(conversation) - 1; index >= 0; index-- {
		message := conversation[index]
		if !strings.EqualFold(strings.TrimSpace(message.Role), "user") {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" || !strings.HasPrefix(content, "<final_gate>") || !strings.HasSuffix(content, "</final_gate>") {
			continue
		}
		return []llm.Message{{Role: "user", Content: content}}
	}
	return nil
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
	if !messageLoopNaturalMixRequest(state.input.UserText) || messageLoopExplicitPluginRequest(state) || !messageLoopHasUsableMixObservation(state) {
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
	return messageLoopStripExecutionQuestion(reply), true
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
	// An active free-state turn is always model-owned observation/family
	// reasoning. Keep the general Agent and typed-control examples outside it.
	if messageLoopFreeStateActive(state) {
		// FULLACCESS-AUTONOMY-1: the full-access autonomy contract is the same on
		// either prompt path, so the free-state neutral-family prompt carries the
		// same directive. It renders "" for every manual/ordinary turn, which
		// keeps both prompts byte-identical to their previous wording.
		return messageLoopNeutralFamilySystemPrompt(state) + messageLoopFullAccessAutonomyRules(state)
	}
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
		if messageLoopSemanticEntryReadOnly(state) || (!messageLoopHasVerifiedSemanticEntry(state) && messageLoopReadOnlyObservationRequest(state.input.UserText)) {
			modeRules += `
Read-only acoustic observation:
- The current user turn explicitly asks for read-only/observe-only analysis. You may use ccb.observation_catalog, ccb.observation_request, mix.observe, mix.read, mix.derive, and read/list/project-state tools only.
- Do not call mix.propose_tick, mix.apply_tick, track.volume, track.pan, plugin preparation/load/learn/apply/write tools, or any DAW mutation tool.
- Do not append mix_treatment_pending and do not ask whether to continue executing. Summarize observed facts, missing or partial evidence, and state that no pending action was created.`
		}
	}
	// FULLACCESS-AUTONOMY-1: full project access owns execution (observe -> decide the
	// direction and dose inside the admitted domain table -> execute -> explain -> hand
	// over to the user's A/B audition). The directive renders "" for every
	// manual/ordinary turn, so those prompts stay byte-identical.
	modeRules += messageLoopFullAccessAutonomyRules(state)
	prompt := fmt.Sprintf(`You are Ask Vit's DAW ReAct runtime inside Vit-DAW.
Return ONLY strict JSON in one of these shapes:
{"final":true,"reply":"short final user-facing reply","tool_calls":[]}
{"final":false,"needs_clarification":true,"clarification_question":"ask exactly what target/choice is missing","reply":"same question","tool_calls":[]}
{"final":false,"reply":"short progress note","tool_calls":[{"tool":"track.add","args":{},"reason":"why"}]}
{"final":true,"reply":"concrete EQ proposal for confirmation","semantic_action":{"schema_version":"semantic_effect_action.v1","action_type":"eq_edit","payload_schema":"semantic_effect.eq_plan.v1","target":{"track_id":"real id","plugin_id":"real id"},"user_goal":"the user's acoustic goal","negative_constraints":[],"evidence_decision":{"choice":"reuse|read|derive|observe","basis":"observation|both","reason":"why this evidence is sufficient","observation_id":"exact observed id","evidence_refs":[]},"limitations":[],"eq_plan":{"schema_version":"semantic_effect.eq_plan.v1","atomic":true,"atoms":[{"atom_id":"stable semantic id","action":"upsert","shape":"bell|low_shelf|high_shelf|low_cut|high_cut","frequency_hz":3500,"gain_db":-1.0,"q":1.2,"purpose":"acoustic purpose","field_origins":{"frequency_hz":"user_fixed|llm_selected|context_inherited","gain_db":"user_fixed|llm_selected|context_inherited","q":"user_fixed|llm_selected|context_inherited"},"evidence_refs":[],"confidence":"low|medium|high"}]}},"tool_calls":[]}

Rules:
- Use only tools from Allowed tools. For low-level DAW commands, use tool:"daw.invoke" only when it is explicitly allowed, with args containing cmd.
- Tool results appear in <tool_result> JSON messages. Treat those results as the source of truth for executed actions, refreshed DAW state, bindings, and verification.
- Capability context packs appear in <capability_context_pack> JSON messages. Treat them as deterministic default starting context for a named capability, not as a restriction; call additional allowed tools when the pack says evidence is missing, partial, stale, or too narrow.
- Ordinary semantic effect discussion is always available and is not B4 or any A-F stage. If the user is asking why, comparing options, requesting analysis, or explicitly asking for read-only observation, reply normally and do not emit semantic_action.
- An actionable abstract static-EQ listening goal requires successful current observation evidence. Reuse a fresh matching observation when possible; call mix.read or mix.derive for an existing artifact/projection; call mix.observe when scope, freshness, or evidence is insufficient. user_report/not_needed alone cannot authorize an abstract EQ plan.
- Once enough evidence and an exact selected track/plugin target exist, emit one semantic_effect_action.v1 with semantic_effect.eq_plan.v1. The LLM owns the acoustic Frequency/Gain/Q/Shape judgement and may infer fields the user did not state. Preserve negative constraints and use 1-3 jointly authorized atoms. Do not use a phrase-to-parameter lookup table.
- Treat a simultaneous positive goal and negative listening constraint as one coupled authorization. When one EQ atom cannot independently express both (for example, adding brightness while controlling harshness), use 2-3 coordinated atoms with distinct acoustic purposes and preserve the shared negative constraint; do not collapse the constraint into prose while proposing only the positive move.
- Before freezing an actionable generic EQ semantic_action, obtain the selected instance's live generic EQ eq_band_summary/control_topology through plugin_grabber.explain_controls unless matching topology evidence is already present in this run. Use only that generic topology evidence to choose reachable shapes/fields. A shape is usable only when a concrete section reports shape_capabilities.actions.upsert=true with every explicitly requested field writable; supported_filter_kinds alone is informational and is not execution authority. A topology rejection must lead to another LLM acoustic choice, not a vendor rule or phrase table.
- A <generic_eq_topology> or context.generic_eq_topology block is deterministic structural evidence for the selected instance. It says only which shapes and fields are reachable; use it to make your own acoustic choice. Do not ask whether to use a dynamic band, compressor, profile, or plug-in-specific mode: this stage is generic static EQ only, so express the listening goal and negative constraint with static EQ atoms or disclose a limitation.
- semantic_action has no mutation authority. Never call plugin_grabber.apply_eq_edits, set_eq_point, set_plugin_param, or plugin loading for an abstract or explicit generic-EQ adjustment. The server will validate, freeze, confirm, and execute the action through the governed runtime.
- For an explicit generic static-EQ parameter request, read the selected instance with plugin_grabber.explain_controls and use plugin_grabber.apply_eq_edits(track_id, plugin_id, edits, atomic:true). Shapes: bell, low_shelf, high_shelf, low_cut, high_cut. Actions: upsert, modify, disable, remove, undo. Explicit Q and slope are hard requirements; report exact/quantized/rejected and actual readback.
- For an explicit broadband-compressor parameter request, call plugin_grabber.inspect_compressor(track_id, plugin_id) first. Then call plugin_grabber.apply_compressor_controls(track_id, plugin_id, controls, atomic:true) using only the generation-scoped control_ref values returned by that inspect result. Never use a bare parameter ID as control_ref. Use exactly one explicit target per control: value_db, ratio, value_ms, percent, display_value, or enum_label. Pure limiters and multiband compressors are outside this tool.
- One successful inspect_compressor result is sufficient: read its compressor_control_summary controls and reuse those exact control_ref values. Do not inspect repeatedly, call goal.tick, or wrap either compressor tool in daw.invoke.
- A compressor apply result is successful only when its status is exact or quantized and every requested control has typed actual_readback. Report those readback values. If inspect or apply rejects or fails, report that failure and do not call plugin.set_parameter/set_plugin_param, plugin_grabber.explain_controls, or any other write as a fallback.
- Generic parameter writes are an explicit-control fallback only: they are never permitted in an abstract/free-state semantic turn, and they are forbidden for every parameter proven to belong to a live typed effect topology. Use the effect's typed inspect/apply tool whenever that surface exists; only a concrete user-authorized control request with no typed surface may use plugin_grabber.explain_controls, plugin.set_parameter, and plugin.get_parameters. Never use stored mappings, retired control mappings, or value_text writes.
- Mixing is a native Ask Vit conversation task, not a separate Auto Mix/Co-Mix mode. Do not create or ask the user to fill a planning card for mixing.
- Select observation tools and scopes from the current question, available catalog, target, and evidence gaps. Do not request a default view or prefer a particular observation solely because of wording. When free_state_reasoning_loop is active, use its CCB observation protocol; CCB owns any internal mix observation. A concrete explicit control request may use the typed control path when its live target and authorization are present.
- Clip fade/gain is an edit-domain operation, not a mixing-domain operation. For selected/current clip fade/gain status, use clip.fade.read and clip.gain.read. For static clip gain edits, use clip.gain.set. Do not route clip fade/gain wording to mix.observe, mix.propose_tick, mix.apply_tick, track.volume, or track.pan.
- Strip Silence / clip cleanup is an edit-domain operation. For parameter recommendation, noise-floor estimation, selected-range cleanup advice, selected-track cleanup advice, or all-project cleanup advice, use clip.strip_silence.suggest first. Choose scope from intent: all_project for whole-project/all-track cleanup, selected_track for current/selected track cleanup, selected_ranges for selected range cleanup, and selected_clip for current/selected clip cleanup. It does not mutate the project and returns pending clip.strip_silence.apply actions built from real analyze strip_regions. After explicit confirmation, run a single clip.strip_silence.apply for one action or clip.strip_silence.apply_batch for multiple actions; do not invent strip_regions.
- Choose mix.observe scope from intent, not trigger phrases: selected_clip, selected_track, named_track, track_group, full_project, or full_project_with_focus_track. Use project_context for current-track mixing, full_project for overall mix questions, and full_project_with_focus_track for vocal/lead/focus relationships.
- mix.observe returns a digest and catalog. Use mix.read for the catalog entries you need and mix.derive for local relationship packages such as rank_tracks, focus_vs_project, a_vs_b, group_overlap, or before_after. Do not manually compare large raw packages in your hidden reasoning when a relationship package can be derived locally.
- For vocal/lead/focus relationship goals, only treat a track as the vocal when the track name/metadata explicitly identifies it as vocal/voice/lead/主唱/人声, or the user explicitly identifies the track by name/index. If the project only has generic names such as Track 1 / Track 2 and you are not sure which one is the vocal, ask which track is the lead vocal. Do not propose or store an executable move while asking that clarification.
- Do not call clip.warm_waveform_bake / warm_waveform_bake directly for broad mixing observation. mix.observe owns waveform and envelope feature preparation.
- After mix.observe/mix.read/mix.derive for a broad request, normally summarize the observed facts and propose one small move. If the same user turn already explicitly names a loaded/learned plug-in and a concrete action, the governed control path may continue using the usable observation; otherwise ask for confirmation before mutation.
- For Chinese acoustic observation replies, use natural-language sections in this order: 结论、证据、限制、建议. Keep the evidence human-readable, such as "来自 L3 频段、声像和响度分析"; do not expose schema names, source/render revision strings, raw evidence_ref lists, raw JSON, waveform arrays, tile payloads, or internal IDs unless the user explicitly asks for technical details.
- Treat deep/slow packages as optional. If they are missing, pending, partial, or blocked, say what uncertainty remains and base suggestions only on available evidence.
- If acoustic_package_status.v0 shows l3_deep building or partial, reply in Chinese with the available L1 facts, the L3 feature status, tile/coverage progress when present, and say full-song band/stereo judgement is not reliable yet. Do not create pending actions or ask to continue executing for read-only observation.
- For B1 gain-staging fader unity reset, use track.group.apply_control with mode:absolute and db:0 after confirmation; this is an engineering state reset, not a B2 small mix tick, and is not limited to +/-2 dB.
- B2 whole-project static balance and B3 whole-project pan layout are owned by Project-aware Capability Runtime v1 before AgentLoop. If either request reaches this loop, do not create a pending plan, do not issue gain/pan mutations, and do not emulate the capability with local mix ticks; return a concise routing failure so the request can be retried through the v1 PlanningSession path.
- For local gain/pan moves outside B2 whole-project static balance, keep each action to one safe small step or one clearly coupled small move. After confirmation use mix.propose_tick then mix.apply_tick; do not call track.volume or track.pan directly.
- For a simple concrete gain/pan move that v1 can execute as one acoustic mix tick, ask for confirmation in normal user-facing text with the concrete small amount; do not append mix_treatment_pending for that tick. The local runtime will turn the confirmed move into mix.propose_tick/mix.apply_tick.
- For plugin/EQ/compressor/reverb/delay/saturation or non-tick gain/pan treatment after observation, propose the treatment direction in normal user-facing Chinese only. Do not append internal markers, schemas, raw JSON, or hidden pending payloads. The local runtime constructs PendingCandidate/MixTreatmentPending deterministically from MOM projection, evidence refs, trust quality, and the user request; if the runtime cannot resolve target/action/processor/confidence safely, ask for clarification or keep the result as advice only.
- Do not invent plugin instances, stored mappings, controls, or exact parameters in a treatment pending. The local resolver decides whether the confirmed treatment can execute, needs preparation, or needs clarification.
- Raw plug-in parameter mutation is not model-visible and must not be proposed as a fallback.
- If the user says to undo or roll back the last mix move, use the available project undo/rollback path directly instead of returning to a mixing workflow.
- Do not invent track_id, clip_id, plugin_id, or tool names.
- Prefer read-only observation before risky writes, but do not over-observe when the current context already contains enough state.
- For dependent DAW edits, you may emit multiple tool calls in one response. Use symbolic refs such as {"track_ref":"last_created_track"} or {"track_id":"$last_created_track"} for later calls that target an object created by an earlier call.
- For folder-track edits, use folder refs such as {"folder_track_ref":"last_created_folder_track"} or {"folder_track_id":"$last_created_folder_track"}. After project.apply_track_organization succeeds, stop and summarize the created folders/moved tracks instead of decomposing the same organization into extra manual folder_track.create + track.move_to_folder calls.
- If recent_goal_context.execution_memory.pending_track_organization exists and the user confirms the TOM/A3 organization proposal, use that complete manifest with project.apply_track_organization. Do not claim only visible_tracks are available, and do not rebuild the proposal from the capped visible_tracks summary.
- If recent_goal_context.execution_memory.pending_section_markers exists and the user confirms the A5/EPM section map, use project.markers.apply_section_markers with that complete sections manifest. Marker writing must not move clips or tracks.
- The local runtime resolves symbolic refs from prior tool results and execution bindings. Do not call the model again only to bind an ID that the tool result already produced.
- Target priority is: explicit user-named/indexed target, object created in this turn, explicit current/selected UI target, single unambiguous default, otherwise ask for clarification.
- For a single explicit audio file or selected library item that should be placed into a known track, use clip.import_media_to_track. For stems, multitrack folders, folders of audio files, or explicit create-tracks-from-folder requests, use project.import_preflight first, then project.import_folder_as_stems for the write; never simulate a folder/stems import with repeated track.add_audio plus clip.import_media_to_track calls.
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
	if state != nil && messageLoopHasVerifiedSemanticEntry(state) {
		if start := strings.Index(prompt, `{"final":true,"reply":"concrete EQ proposal`); start >= 0 {
			if end := strings.Index(prompt[start:], "\n\nRules:"); end >= 0 {
				prompt = prompt[:start] + prompt[start+end:]
			}
		}
		if start := strings.Index(prompt, "- For an actionable generic static-EQ listening goal"); start >= 0 {
			if end := strings.Index(prompt[start:], "- For an explicit generic static-EQ parameter request"); end >= 0 {
				prompt = prompt[:start] + "- For a verified semantic-entry turn, keep acoustic family and observation selection model-owned; use the free-state decision or the typed control protocol selected by the entry route.\n" + prompt[start+end:]
			}
		}
	}
	return prompt
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
	repairSystem := `You repair Ask Vit MessageLoop outputs.
Return ONLY one strict JSON object in one of these shapes:
{"final":true,"reply":"short final user-facing reply","tool_calls":[]}
{"final":false,"needs_clarification":true,"clarification_question":"ask exactly what target/choice is missing","reply":"same question","tool_calls":[]}
{"final":false,"reply":"short progress note","tool_calls":[{"tool":"track.add","args":{},"reason":"why"}]}
Do not add markdown fences, comments, prose, or multiple JSON objects. Preserve the original intent and tool arguments whenever possible.`
	if messageLoopAudioClosureActive(state) {
		repairSystem = `You repair one MinimalAudioClosure MessageLoop output.
The original user intent and target are already durably owned by the closure. Repair JSON syntax only; never ask the user to restate the task and never invent a clarification.
Return ONLY one strict JSON object. Preserve a recoverable final, tool_calls, semantic_action, or free_state decision exactly.
If the semantic content cannot be recovered from the raw output, return exactly:
{"final":false,"failure_reason":"model_protocol_failure","reply":"","tool_calls":[]}
Do not add markdown fences, comments, prose, multiple objects, or a needs_clarification field.`
	}
	messages := []llm.Message{
		{
			Role:    "system",
			Content: repairSystem,
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

func messageLoopAudioClosureActive(state *runState) bool {
	if state == nil {
		return false
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	return messageLoopText(closure["schema_version"]) == "minimal_audio_closure.v1" && messageLoopText(closure["phase"]) != "settled"
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
	if out.Final || out.NeedsClarification || strings.TrimSpace(out.FailureReason) != "" || len(out.ToolCalls) > 0 || out.SemanticAction != nil || out.FreeStateDecision != nil {
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
	// RetryPrompt carries the strengthened retry directive actually fed back
	// for BOUNDARY-1 terminal-turn fallback evidence (retry prompt of the
	// three-piece record).
	RetryPrompt string `json:"retry_prompt,omitempty"`
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

// messageLoopPromptRenderDiagnostic is the BOUNDARY-1 §3.1 prompt-render
// evidence row: the full system prompt actually assembled for one model
// turn, with the round identity and the stable section tag. Never truncated —
// the whole point is that the final window's rendering becomes verifiable.
type messageLoopPromptRenderDiagnostic struct {
	At             string `json:"at"`
	Stage          string `json:"stage"`
	GoalID         string `json:"goal_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	SliceID        string `json:"slice_id,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
	Turn           int    `json:"turn"`
	Tag            string `json:"tag"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	SystemPrompt   string `json:"system_prompt"`
}

// appendMessageLoopPromptRenderDiagnostic writes the per-turn prompt-render
// evidence (BOUNDARY-1 §3.1). D1 runs and ordinary runs both flow through
// this one assembly point, so both are covered.
func appendMessageLoopPromptRenderDiagnostic(state *runState, assembly promptruntime.Assembly) {
	path := messageLoopPromptRenderPath()
	if path == "" || state == nil {
		return
	}
	tag := "message_loop_system"
	if messageLoopNeedsNeutralFamilyProjection(state) {
		tag = "message_loop_neutral_family_selection"
	}
	system := ""
	for _, message := range assembly.Messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), "system") {
			system = message.Content
			break
		}
	}
	row := messageLoopPromptRenderDiagnostic{
		At:             time.Now().Format(time.RFC3339Nano),
		Stage:          "prompt_assembly",
		GoalID:         state.goal.GoalID,
		RunID:          state.goal.RunID,
		ConversationID: messageLoopConversationID(state),
		SliceID:        state.input.SliceID,
		TurnID:         state.input.TurnID,
		Turn:           state.turnsUsed + 1,
		Tag:            tag,
		Fingerprint:    assembly.Fingerprint,
		SystemPrompt:   system,
	}
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

func messageLoopPromptRenderPath() string {
	if override := strings.TrimSpace(os.Getenv("VIT_AGENT_PROMPT_RENDER_PATH")); override != "" {
		if strings.EqualFold(override, "off") || strings.EqualFold(override, "disabled") {
			return ""
		}
		return override
	}
	if telemetry := strings.TrimSpace(llm.DefaultTelemetryPath()); telemetry != "" {
		return filepath.Join(filepath.Dir(telemetry), "agent_prompt_render.jsonl")
	}
	return filepath.Join(os.TempDir(), "vit_agent_prompt_render.jsonl")
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

func messageLoopSemanticEffectProposalAllowed(state *runState) bool {
	if state == nil || messageLoopPlanMode(state.input.Context) {
		return false
	}
	if messageLoopBool(state.input.Context["semantic_entry_unavailable"]) {
		return false
	}
	if _, verified := messageLoopSemanticEntryRoute(state); verified {
		// Classified ordinary-Agent turns never materialize a family-specific
		// semantic_action directly. Open semantics hand off through free-state;
		// explicit controls use typed tools and their confirmation policy.
		return false
	}
	if messageLoopReadOnlyObservationRequest(state.input.UserText) {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(state.input.UserText))
	if text == "" {
		return false
	}
	discussion := messageLoopTextHasAny(text, "为什么", "為什麼", "什么原因", "什麼原因", "怎么判断", "怎麼判斷", "分析一下", "解释一下", "解釋一下", "why", "what causes", "how do you know")
	action := messageLoopSemanticEffectActionRequested(state.input.UserText)
	return !discussion || action
}

func messageLoopSemanticEffectActionRequested(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return messageLoopTextHasAny(text, "减少", "減少", "降低", "收一点", "收一些", "削", "切", "提高", "提升", "增加", "更亮", "更暗", "处理", "處理", "调整", "調整", "修改", "执行", "執行", "apply", "reduce", "cut", "boost", "raise", "lower", "make it", "adjust", "change", "execute")
}

func messageLoopSemanticEQTopic(state *runState) bool {
	if state == nil {
		return false
	}
	pluginID := firstNonEmpty(firstMapText(state.input.Context, "selected_plugin_id", "plugin_id"), firstMapText(state.input.State, "selected_plugin_id", "plugin_id"))
	if pluginID == "" {
		return false
	}
	return messageLoopSemanticEQIntent(state.input.UserText)
}

func messageLoopSemanticEQIntent(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return messageLoopTextHasAny(text,
		"浑浊", "浑", "闷", "刺耳", "尖锐", "更亮", "更暗", "高频", "低频", "中频", "低中频", "空气感", "清晰", "均衡", "频段", "赫兹",
		"mud", "muddy", "boxy", "harsh", "bright", "dark", "treble", "bass", "mid", "air", "clarity", "presence", "eq", "equalizer", "hz", "shelf", "bell", "cut",
	)
}

func messageLoopOrdinarySemanticEQRequest(state *runState) bool {
	if _, verified := messageLoopSemanticEntryRoute(state); verified {
		return false
	}
	return messageLoopSemanticEQTopic(state) && messageLoopSemanticEffectProposalAllowed(state)
}

func messageLoopSemanticEQNeedsPluginSelection(state *runState) bool {
	if _, verified := messageLoopSemanticEntryRoute(state); verified {
		return false
	}
	if state == nil || !messageLoopSemanticEQIntent(state.input.UserText) || !messageLoopSemanticEffectProposalAllowed(state) || !messageLoopSemanticEffectActionRequested(state.input.UserText) {
		return false
	}
	pluginID := firstNonEmpty(firstMapText(state.input.Context, "selected_plugin_id", "plugin_id"), firstMapText(state.input.State, "selected_plugin_id", "plugin_id"))
	return pluginID == "" && firstNonEmpty(messageLoopExactSelectedTrackID(state), state.executionMemory.ActiveWorkTargetTrackID) != ""
}

func messageLoopHasVerifiedSemanticEntry(state *runState) bool {
	_, ok := messageLoopSemanticEntryDecision(state)
	return ok
}

func messageLoopHasGenericEQTopologyEvidence(state *runState) bool {
	if state == nil {
		return false
	}
	for _, source := range []map[string]any{state.input.Context, state.input.State} {
		if len(messageLoopMapValue(source["eq_band_summary"])) > 0 || len(messageLoopMapValue(source["control_topology"])) > 0 || len(messageLoopMapValue(source["generic_eq_topology"])) > 0 {
			return true
		}
	}
	for _, record := range state.executed {
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if (name == "plugin_grabber.explain_controls" || name == "plugin_grabber_explain_controls") && messageLoopExecutionSucceeded(record) {
			return true
		}
	}
	sawTopologyRead := false
	for _, event := range state.trace {
		if event.ToolCall != nil {
			name := strings.ToLower(strings.TrimSpace(event.ToolCall.Tool))
			if name == "plugin_grabber.explain_controls" || name == "plugin_grabber_explain_controls" {
				sawTopologyRead = true
			}
		}
		if sawTopologyRead && event.ToolResult != nil {
			status := strings.ToLower(strings.TrimSpace(event.ToolResult.Status))
			if status == "ok" || status == "success" || status == "completed" {
				return true
			}
		}
	}
	return false
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
	mirrorCommand := len(call.Command) > 0
	call.Args = cloneMap(call.Args)
	if call.Args == nil {
		call.Args = map[string]any{}
	}
	if mirrorCommand {
		call.Command = cloneMap(call.Command)
	} else if tool := strings.TrimSpace(call.Tool); tool != "" && tool != "daw.invoke" {
		call.Command = cloneMap(call.Args)
		if call.Command == nil {
			call.Command = map[string]any{}
		}
		call.Command["tool"] = tool
		mirrorCommand = true
	} else {
		call.Command = nil
	}
	call = resolveMessageLoopCCBTarget(state, call)
	trackID := resolveTrackRef(state, firstMapText(call.Args, "track_ref", "target_track_ref", "track_id", "target_track_id", "selected_track_id"))
	if trackID == "" && messageLoopToolWantsTrack(call) {
		trackID = state.executionMemory.ActiveWorkTargetTrackID
	}
	if trackID != "" && messageLoopToolWantsTrack(call) {
		setIfEmpty(call.Args, "track_id", trackID)
		if mirrorCommand {
			setIfEmpty(call.Command, "track_id", trackID)
		}
	}
	folderTrackID := resolveFolderTrackRef(state, firstMapText(call.Args, "folder_track_ref", "target_folder_track_ref", "folder_track_id", "target_folder_track_id"))
	if folderTrackID == "" && messageLoopToolWantsFolderTrack(call) {
		folderTrackID = firstNonEmpty(state.executionMemory.ActiveWorkTargetFolderTrackID, state.executionMemory.LastCreatedFolderTrackID)
	}
	if folderTrackID != "" && messageLoopToolWantsFolderTrack(call) {
		setIfEmpty(call.Args, "folder_track_id", folderTrackID)
		if mirrorCommand {
			setIfEmpty(call.Command, "folder_track_id", folderTrackID)
		}
	}
	parentFolderTrackID := resolveFolderTrackRef(state, firstMapText(call.Args, "parent_folder_track_ref", "parent_folder_track_id"))
	if parentFolderTrackID != "" {
		setIfEmpty(call.Args, "parent_folder_track_id", parentFolderTrackID)
		if mirrorCommand {
			setIfEmpty(call.Command, "parent_folder_track_id", parentFolderTrackID)
		}
	}
	clipID := resolveClipRef(state, firstMapText(call.Args, "clip_ref", "target_clip_ref", "clip_id", "selected_clip_id", "primary_selected_clip_id"))
	if clipID == "" && messageLoopToolWantsClip(call) {
		clipID = firstNonEmpty(state.executionMemory.ActiveWorkTargetClipID, state.executionMemory.LastCreatedClipID)
	}
	if clipID != "" && messageLoopToolWantsClip(call) {
		setIfEmpty(call.Args, "clip_id", clipID)
		if mirrorCommand {
			setIfEmpty(call.Command, "clip_id", clipID)
		}
	}
	tickID := resolveMixTickRef(state, firstMapText(call.Args, "tick_ref", "mix_tick_ref", "tick_id"))
	if tickID == "" && messageLoopToolWantsMixTick(call) {
		tickID = state.executionMemory.LastMixTickID
	}
	if tickID != "" && messageLoopToolWantsMixTick(call) {
		setIfEmpty(call.Args, "tick_id", tickID)
		if mirrorCommand {
			setIfEmpty(call.Command, "tick_id", tickID)
		}
	}
	call = resolvePendingTrackOrganizationCall(state, call)
	call = resolvePendingSectionMarkersCall(state, call)
	return call
}

func resolveMessageLoopCCBTarget(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || !messageLoopIsCCBObservationTool(call) ||
		firstMapText(call.Args, "target_id", "track_id", "clip_id") != "" || len(messageLoopMapValue(call.Args["target_ref"])) > 0 {
		return call
	}
	views := messageLoopStringList(call.Args["view_ids"])
	hasTrackView := false
	hasProjectView := false
	for _, view := range views {
		view = strings.ToLower(strings.TrimSpace(view))
		hasTrackView = hasTrackView || strings.HasPrefix(view, "track.") || strings.HasPrefix(view, "processor.") || view == "comparison.before_after"
		hasProjectView = hasProjectView || strings.HasPrefix(view, "mix.") || strings.HasPrefix(view, "project.")
	}
	loop := messageLoopFreeStateContext(state)
	target := messageLoopMapValue(loop["target_ref"])
	trackID := firstNonEmpty(firstMapText(state.input.Context, "selected_track_id", "selected_plugin_track_id"), firstMapText(target, "track_id"))
	trackName := firstNonEmpty(firstMapText(state.input.Context, "selected_track_name"), firstMapText(target, "track_name"))
	if hasTrackView || !hasProjectView {
		if trackID == "" {
			return call
		}
		setIfEmpty(call.Args, "target_kind", "track")
		setIfEmpty(call.Args, "target_id", trackID)
		setIfEmpty(call.Args, "track_id", trackID)
		setIfEmpty(call.Args, "target_label", trackName)
	} else {
		setIfEmpty(call.Args, "target_kind", "project")
		setIfEmpty(call.Args, "target_id", "current")
		setIfEmpty(call.Args, "target_label", "Current project")
	}
	if call.Command != nil {
		for _, key := range []string{"target_kind", "target_id", "track_id", "target_label"} {
			if value := firstMapText(call.Args, key); value != "" {
				setIfEmpty(call.Command, key, value)
			}
		}
	}
	return call
}

func resolvePendingTrackOrganizationCall(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || len(state.executionMemory.PendingTrackOrganization) == 0 || !messageLoopIsApplyTrackOrganizationCall(call) {
		return call
	}
	pending := state.executionMemory.PendingTrackOrganization
	pendingGroups := pendingTrackOrganizationToolGroups(pending)
	if len(pendingGroups) == 0 {
		return call
	}
	desiredCount := firstPositiveMapInt(pending, "assignment_coverage_count", "track_count")
	currentCount := messageLoopApplyTrackOrganizationTrackCount(call.Args["groups"])
	usePending := currentCount == 0
	if desiredCount > 0 && currentCount > 0 && currentCount < desiredCount {
		usePending = true
	}
	if requested, ok := firstMapBool(call.Args, "use_pending_track_organization", "use_tom_manifest", "use_pending_tom_manifest"); ok && requested {
		usePending = true
	}
	if !usePending {
		return call
	}
	call.Args["groups"] = pendingGroups
	setIfEmpty(call.Args, "source", "pending_track_organization")
	if call.Command != nil {
		call.Command["groups"] = pendingGroups
		setIfEmpty(call.Command, "source", "pending_track_organization")
	}
	return call
}

func messageLoopIsApplyTrackOrganizationCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		call.Tool,
		firstMapText(call.Args, "cmd", "command", "action"),
		firstMapText(call.Command, "cmd", "command", "action"),
	)))
	switch name {
	case "project.apply_track_organization", "project.track_organization.apply", "apply_track_organization":
		return true
	default:
		return false
	}
}

func pendingTrackOrganizationToolGroups(pending map[string]any) []map[string]any {
	rows := messageLoopMapRows(pending["groups"])
	if len(rows) == 0 {
		return nil
	}
	groups := []map[string]any{}
	for _, row := range rows {
		trackIDs := messageLoopSplitTrackIDsCSV(firstMapText(row, "track_ids_csv"))
		if len(trackIDs) == 0 {
			continue
		}
		folderName := firstMapText(row, "folder_name", "proposed_folder", "label", "group_id")
		if folderName == "" {
			continue
		}
		group := map[string]any{
			"folder_name":         folderName,
			"track_ids":           trackIDs,
			"routing_bus_enabled": false,
		}
		if enabled, ok := firstMapBool(row, "routing_bus_enabled"); ok {
			group["routing_bus_enabled"] = enabled
		}
		groups = append(groups, group)
	}
	return groups
}

func messageLoopApplyTrackOrganizationTrackCount(value any) int {
	total := 0
	for _, group := range messageLoopMapRows(value) {
		total += len(messageLoopStringSliceFromAny(group["track_ids"]))
		if assignments := messageLoopMapRows(group["assignments"]); len(assignments) > 0 {
			for _, assignment := range assignments {
				if firstMapText(assignment, "track_id") != "" {
					total++
				}
			}
		}
	}
	return total
}

func messageLoopSplitTrackIDsCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func resolvePendingSectionMarkersCall(state *runState, call planner.ToolCall) planner.ToolCall {
	if state == nil || len(state.executionMemory.PendingSectionMarkers) == 0 || !messageLoopIsApplySectionMarkersCall(call) {
		return call
	}
	pending := state.executionMemory.PendingSectionMarkers
	pendingSections := pendingSectionMarkersToolSections(pending)
	if len(pendingSections) == 0 {
		return call
	}
	desiredCount := firstPositiveMapInt(pending, "section_count")
	currentCount := len(messageLoopMapRows(call.Args["sections"]))
	if currentCount == 0 {
		currentCount = len(messageLoopMapRows(call.Args["markers"]))
	}
	usePending := currentCount == 0
	if desiredCount > 0 && currentCount > 0 && currentCount < desiredCount {
		usePending = true
	}
	if requested, ok := firstMapBool(call.Args, "use_pending_section_markers", "use_epm_section_map", "use_pending_a5_sections"); ok && requested {
		usePending = true
	}
	if !usePending {
		return call
	}
	call.Args["sections"] = pendingSections
	setIfMissing(call.Args, "replace_existing", pending["replace_existing"])
	setIfEmpty(call.Args, "source", firstNonEmpty(firstMapText(pending, "source"), "epm_a5"))
	if call.Command != nil {
		call.Command["sections"] = pendingSections
		setIfMissing(call.Command, "replace_existing", pending["replace_existing"])
		setIfEmpty(call.Command, "source", firstNonEmpty(firstMapText(pending, "source"), "epm_a5"))
	}
	return call
}

func messageLoopIsApplySectionMarkersCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		call.Tool,
		firstMapText(call.Args, "cmd", "command", "action"),
		firstMapText(call.Command, "cmd", "command", "action"),
	)))
	switch name {
	case "project.markers.apply_section_markers", "project_apply_section_markers":
		return true
	default:
		return false
	}
}

func pendingSectionMarkersToolSections(pending map[string]any) []map[string]any {
	rows := messageLoopMapRows(pending["sections"])
	if len(rows) == 0 {
		return nil
	}
	sections := []map[string]any{}
	for i, row := range rows {
		start := firstPresentNumber(row, "start_seconds", "start")
		end := firstPresentNumber(row, "end_seconds", "end")
		if end <= start {
			continue
		}
		name := firstNonEmpty(firstMapText(row, "name", "label"), fmt.Sprintf("Section %d", i+1))
		section := map[string]any{
			"name":          name,
			"label":         name,
			"start_seconds": start,
			"end_seconds":   end,
		}
		if sectionID := firstMapText(row, "section_id", "id"); sectionID != "" {
			section["section_id"] = sectionID
		}
		if confidence := firstMapText(row, "confidence"); confidence != "" {
			section["confidence"] = confidence
		}
		sections = append(sections, section)
	}
	return sections
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

func resolveFolderTrackRef(state *runState, ref string) string {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "$"))
	switch strings.ToLower(ref) {
	case "last_created_folder_track", "new_folder_track", "created_folder_track", "active_work_target_folder_track", "target_folder_track":
		return firstNonEmpty(
			state.executionMemory.ActiveWorkTargetFolderTrackID,
			state.executionMemory.LastCreatedFolderTrackID,
			executionBindingID(state.executionMemory, "active_work_target_folder_track", "last_created_folder_track", "target_folder_track"),
		)
	case "last_created_track", "new_track", "created_track":
		return firstNonEmpty(
			state.executionMemory.ActiveWorkTargetFolderTrackID,
			state.executionMemory.LastCreatedFolderTrackID,
			executionBindingID(state.executionMemory, "active_work_target_folder_track", "last_created_folder_track", "target_folder_track"),
		)
	default:
		return ref
	}
}

func executionBindingID(memory ExecutionMemory, keys ...string) string {
	if len(keys) == 0 || len(memory.Bindings) == 0 {
		return ""
	}
	wanted := map[string]bool{}
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			wanted[key] = true
		}
	}
	for i := len(memory.Bindings) - 1; i >= 0; i-- {
		binding := memory.Bindings[i]
		if !wanted[strings.ToLower(strings.TrimSpace(binding.Key))] {
			continue
		}
		if id := strings.TrimSpace(binding.ID); id != "" {
			return id
		}
	}
	return ""
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
		"create_midi_clip", "insert_midi_clip", "import_midi_to_track", "import_audio", "import_media_to_track", "add_audio_clip",
		"track.move_to_folder":
		return true
	default:
		return false
	}
}

func messageLoopToolWantsFolderTrack(call planner.ToolCall) bool {
	name := normalizedActionName(call, executorpkg.Result{})
	switch name {
	case "track.move_to_folder", "folder_track.set_routing_bus_enabled", "track.folder.set_routing_bus_enabled":
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

func setIfMissing(row map[string]any, key string, value any) {
	if row == nil || value == nil {
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
	if messageLoopFreeStateActive(state) && messageLoopFreeStateMutationTool(call) {
		return "active free-state reasoning may request only read-only CCB observations; typed apply, generic parameter writes, plugin loading, and other mutation tools are forbidden"
	}
	if messageLoopFreeStateActive(state) && messageLoopIsCCBObservationRequestName(normalizedActionName(call, executorpkg.Result{})) {
		if issue := messageLoopFreeStateRejectedViewSetIssue(state, messageLoopStringList(call.Args["view_ids"]), messageLoopCCBTargetFromCall(call)); issue != "" {
			return issue
		}
	}
	if messageLoopIsGenericPluginParameterWrite(call) && messageLoopLatestCompressorApplyFailed(state) {
		return "typed compressor apply failed; generic plugin parameter writes are forbidden as a fallback and no further parameter may be changed"
	}
	if messageLoopIsDADAnalysisControlTool(call) {
		return "DAD analysis is automatic; the agent may read project.audio_analysis_status but must not start or cancel DAD analysis"
	}
	if messageLoopSemanticEQTopic(state) && !messageLoopSemanticEffectProposalAllowed(state) && messageLoopForbiddenOrdinarySemanticEQTool(call) {
		return "discussion-only or read-only generic EQ turns cannot call effect mutation tools or create mutation authority"
	}
	if messageLoopOrdinarySemanticEQRequest(state) && messageLoopForbiddenOrdinarySemanticEQTool(call) {
		return "ordinary-Agent generic EQ listening goals must emit a typed semantic_effect_action after read-only topology/evidence selection; plugin loading, raw parameter writes, and direct EQ mutation tools are forbidden"
	}
	if messageLoopSemanticEQNeedsPluginSelection(state) && messageLoopForbiddenOrdinarySemanticEQTool(call) {
		return "ordinary-Agent generic EQ listening goals without a selected EQ must stop at plugin_selection_required; plugin loading, raw parameter writes, and direct EQ mutation tools are forbidden"
	}
	if isPluginGrabberApplyEQEditsTool(call) || isPluginGrabberSetEQPointTool(call) {
		return "ordinary Agent EQ mutations require a validated semantic_effect_action, frozen Proposal, and authorization; emit semantic_action instead of calling the EQ mutation tool directly"
	}
	if messageLoopIsWaveformBakeTool(call) && messageLoopWaveformBakeCallMissingClipSource(call) {
		return "clip.warm_waveform_bake requires clip_id or file_path; use list_tracks or get_project_state to resolve clip_id before calling this tool, or use mix.observe which handles waveform preparation internally"
	}
	if messageLoopMutationBarrierActive(state) {
		return messageLoopReadOnlyGuardIssue(call)
	}
	if messageLoopBool(state.input.Context["semantic_entry_unavailable"]) && toolNeedsMutationBarrier(call, executorpkg.Result{}) {
		return "semantic entry classification was unavailable; project mutation is denied until target scope and user authorization are classified"
	}
	pluginDecision := toolpolicy.Decide(toolpolicy.TurnContext{
		UserText:             state.input.UserText,
		KnownPluginNames:     messageLoopKnownPluginNames(state),
		HasUsableObservation: messageLoopHasUsableMixObservation(state),
		ObservedBeforeTurn:   hadMixObservationBeforeTurn,
	}, messageLoopPluginPolicyCall(call))
	if pluginDecision.Verdict == toolpolicy.Allow {
		return ""
	}
	if pluginDecision.Verdict == toolpolicy.Deny {
		return pluginDecision.Reason
	}
	if messageLoopExplicitPluginRequest(state) && !messageLoopLowMudPluginPrepForState(state) {
		return ""
	}
	if messageLoopGainStagingFaderResetRequest(state.input.UserText) && strings.TrimSpace(call.Tool) == "track.group.apply_control" {
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
		return "mix.observe is complete; broad mixing requests must stop here, summarize the observation, propose one concrete next move, and wait for explicit user confirmation before loading plugins, changing volume, applying controls, or writing parameters"
	}
	return "ordinary acoustic mixing requests must run mix.observe and wait for its result before loading plugins, changing volume, applying controls, or writing parameters"
}

func messageLoopFreeStateMutationTool(call planner.ToolCall) bool {
	if toolNeedsMutationBarrier(call, executorpkg.Result{}) {
		return true
	}
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "plugin.set_parameter", "plugin_set_parameter", "set_plugin_param",
		"plugin.load_to_rack", "rack.add_node", "rack_add_node", "rack.load_plugin", "rack_load_plugin", "instantiate_plugin", "plugin.instantiate",
		"plugin_grabber.apply_eq_edits", "plugin_grabber_apply_eq_edits",
		"plugin_grabber.set_eq_point", "plugin_grabber_set_eq_point", "plugin_grabber.apply_control", "plugin_grabber_apply_control",
		"plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls",
		"plugin_grabber.apply_limiter_controls", "plugin_grabber_apply_limiter_controls",
		"plugin_grabber.apply_gate_expander_controls", "plugin_grabber_apply_gate_expander_controls",
		"plugin_grabber.apply_de_esser_controls", "plugin_grabber_apply_de_esser_controls",
		"plugin_grabber.apply_transient_shaper_controls", "plugin_grabber_apply_transient_shaper_controls",
		"plugin_grabber.apply_multiband_controls", "plugin_grabber_apply_multiband_controls":
		return true
	default:
		return false
	}
}

func messageLoopIsGenericPluginParameterWrite(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "plugin.set_parameter", "plugin_set_parameter", "set_plugin_param":
		return true
	default:
		return false
	}
}

func messageLoopForbiddenOrdinarySemanticEQTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "plugin.load_to_rack", "rack.add_node", "rack_add_node", "instantiate_plugin", "plugin.instantiate",
		"plugin_grabber.apply_eq_edits", "plugin_grabber_apply_eq_edits",
		"plugin_grabber.set_eq_point", "plugin_grabber_set_eq_point",
		"plugin.set_parameter", "plugin_set_parameter", "set_plugin_param":
		return true
	default:
		return false
	}
}

func isPluginGrabberApplyEQEditsTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name == "plugin_grabber.apply_eq_edits" || name == "plugin_grabber_apply_eq_edits" {
		return true
	}
	name = strings.ToLower(firstNonEmpty(firstMapText(call.Command, "cmd"), firstMapText(call.Command, "command"), firstMapText(call.Command, "tool")))
	return name == "plugin_grabber.apply_eq_edits" || name == "plugin_grabber_apply_eq_edits"
}

func isPluginGrabberSetEQPointTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name == "plugin_grabber.set_eq_point" || name == "plugin_grabber_set_eq_point" {
		return true
	}
	name = strings.ToLower(firstNonEmpty(firstMapText(call.Command, "cmd"), firstMapText(call.Command, "command"), firstMapText(call.Command, "tool")))
	return name == "plugin_grabber.set_eq_point" || name == "plugin_grabber_set_eq_point"
}

func messageLoopHasUsableMixObservation(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if messageLoopIsMixObservationName(name) {
			result := messageLoopMapValue(record["result"])
			if len(result) == 0 {
				result = record
			}
			return messageLoopMixObservationResultUsable(result)
		}
		if messageLoopIsCCBObservationRequestName(name) && messageLoopCCBObservationResultUsable(messageLoopMapValue(record["result"])) {
			return true
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
	if messageLoopClipFadeGainRequest(text) {
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
		"low end", "low-end", "bass", "kick",
	)
}

func messageLoopClipFadeGainRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if messageLoopGainStagingExplicitRequest(text) {
		return false
	}
	hasClipTarget := messageLoopTextHasAny(text,
		"clip", "clips", "selected clip", "current clip", "this clip", "audio clip",
		"\u7247\u6bb5", "\u97f3\u9891\u7247\u6bb5", "\u5f53\u524d\u7247\u6bb5", "\u9009\u4e2d\u7247\u6bb5",
		"\u5f53\u524d\u9009\u4e2d clip", "\u5f53\u524d clip", "\u9009\u4e2d clip", "\u8fd9\u4e2a clip",
	)
	if !hasClipTarget {
		return false
	}
	hasFadeOrGain := messageLoopTextHasAny(text,
		"fade", "fade in", "fade out", "clip gain", "gain",
		"\u6de1\u5165", "\u6de1\u51fa", "\u6de1\u5316", "\u589e\u76ca",
	)
	if !hasFadeOrGain {
		return false
	}
	if messageLoopTextHasAny(text, "fade", "gain", "clip gain", "fade/gain") && messageLoopTextHasAny(text, "clip", "audio clip") {
		return true
	}
	return messageLoopTextHasAny(text,
		"read", "show", "inspect", "status", "state", "get", "set", "adjust", "change", "drag",
		"\u8bfb\u53d6", "\u67e5\u770b", "\u770b\u4e00\u4e0b", "\u72b6\u6001", "\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9",
	)
}

func messageLoopStripSilenceSuggestRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasStripIntent := messageLoopTextHasAny(text,
		"strip silence", "strip_silence", "silence cleanup", "remove silence", "trim silence",
		"清理静音", "片段清理", "清理空白", "去静音", "去掉静音", "去掉空白", "删除静音", "过滤静音", "噪声底",
	) || messageLoopA4ClipCleanupRequest(text)
	if !hasStripIntent {
		return false
	}
	if messageLoopA4ClipCleanupRequest(text) {
		return true
	}
	hasApplyOnlyIntent := messageLoopTextHasAny(text,
		"apply", "execute", "confirm", "do it", "go ahead",
		"应用", "执行", "确认", "按这个", "就这样", "继续",
	)
	hasAnalysisIntent := messageLoopTextHasAny(text,
		"suggest", "recommend", "analyze", "analyse", "estimate", "parameter", "threshold", "preview",
		"建议", "推荐", "分析", "估算", "参数", "阈值", "预览", "检查",
	)
	return !hasApplyOnlyIntent || hasAnalysisIntent
}

func messageLoopStripSilenceAllProjectRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if messageLoopTextHasAny(text,
		"\u5168\u5de5\u7a0b", "\u6574\u4e2a\u5de5\u7a0b", "\u5168\u90e8\u5de5\u7a0b",
		"\u5168\u9879\u76ee", "\u6574\u4e2a\u9879\u76ee", "\u6240\u6709\u7247\u6bb5",
		"\u6240\u6709\u8f68\u9053", "\u5168\u90e8\u8f68\u9053", "\u6240\u6709\u97f3\u8f68", "\u5168\u90e8\u97f3\u8f68",
	) {
		return true
	}
	return messageLoopTextHasAny(text,
		"all project", "whole project", "full project", "entire project", "global", "all tracks", "every track", "all clips",
		"全工程", "整个工程", "全部工程", "所有片段", "全项目", "所有轨道", "全部轨道",
	) || messageLoopA4ClipCleanupWholeProjectRequest(text)
}

func messageLoopStripSilenceSelectedTrackRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return messageLoopTextHasAny(text,
		"selected track", "current track", "this track", "current selected track", "selected audio track",
		"\u9009\u4e2d\u8f68\u9053", "\u5f53\u524d\u8f68\u9053", "\u8fd9\u6761\u8f68", "\u8fd9\u4e2a\u8f68\u9053",
		"\u9009\u4e2d\u97f3\u8f68", "\u5f53\u524d\u97f3\u8f68",
	)
}

func messageLoopStripSilenceRangeRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return messageLoopTextHasAny(text,
		"selected range", "selected ranges", "clip range", "clip ranges",
		"time range", "time ranges", "time selection", "range selection",
		"\u9009\u4e2d\u8303\u56f4", "\u5f53\u524d\u8303\u56f4", "\u7247\u6bb5\u8303\u56f4",
		"\u65f6\u95f4\u8303\u56f4", "\u9009\u533a", "\u8303\u56f4",
	)
}

func messageLoopA4ClipCleanupRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || !messageLoopTextHasAny(text, "a4", "a 4", "epm") {
		return false
	}
	return messageLoopTextHasAny(text,
		"clip cleanup", "clip trim", "clip trimming", "trim clips", "cleanup clips",
		"片段裁剪", "片段清理", "裁剪片段", "清理片段", "裁剪", "清理",
	)
}

func messageLoopA4ClipCleanupWholeProjectRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if !messageLoopA4ClipCleanupRequest(text) {
		return false
	}
	return !messageLoopTextHasAny(text,
		"selected", "current", "this clip", "this track", "selected clip", "selected track", "current clip", "current track", "selected range", "range",
		"选中", "当前片段", "当前轨道", "选中片段", "选中轨道", "这个片段", "这条轨", "范围", "选区",
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
	return toolpolicy.ExplicitPluginRequest(userText, nil)
}

func messageLoopKnownPluginNames(state *runState) []string {
	if state == nil {
		return nil
	}
	values := []any{
		state.input.Context,
		state.input.State,
		state.contextSnapshot,
		state.projectHistory,
		state.executed,
	}
	if state.recentObservation != nil {
		values = append(values, state.recentObservation.Summary)
	}
	if name := strings.TrimSpace(state.executionMemory.LastLoadedPluginName); name != "" {
		values = append(values, map[string]any{"plugin_name": name})
	}
	return toolpolicy.CollectPluginNames(values...)
}

func messageLoopExplicitPluginRequest(state *runState) bool {
	return state != nil && toolpolicy.ExplicitPluginRequest(state.input.UserText, messageLoopKnownPluginNames(state))
}

func messageLoopLowMudPluginPrepForState(state *runState) bool {
	return state != nil && toolpolicy.LowMudNeedsObservation(state.input.UserText, messageLoopKnownPluginNames(state))
}

func messageLoopPluginPolicyCall(call planner.ToolCall) toolpolicy.ToolCall {
	name := strings.TrimSpace(normalizedActionName(call, executorpkg.Result{}))
	if name == "" {
		name = strings.TrimSpace(call.Tool)
	}
	args := cloneToolArgs(call.Args)
	for key, value := range call.Command {
		if _, exists := args[key]; !exists {
			args[key] = value
		}
	}
	return toolpolicy.ToolCall{Name: name, Args: args}
}

func cloneToolArgs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
	hasExactTrackFocus := firstMapText(args, "track_id", "selected_track_id", "target_track_id") != "" || messageLoopTextHasAny(text, "\u5f53\u524d\u8f68\u9053", "\u9009\u4e2d\u8f68\u9053", "current track", "selected track")
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
		if hasExactTrackFocus {
			return "full_project_with_focus_track"
		}
		return "full_project"
	}
	if firstMapText(args, "clip_id", "selected_clip_id") != "" || messageLoopTextHasAny(text, "clip", "\u7247\u6bb5") {
		return "selected_clip"
	}
	if firstMapText(args, "track_name", "target_track_name", "name") != "" {
		return "named_track"
	}
	if hasExactTrackFocus {
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

// messageLoopWaveformBakeCallMissingClipSource reports whether a direct
// warm_waveform_bake tool call is missing the clip_id/file_path it needs.
// The kernel handler (ImportService::handleWarmWaveformBake) requires one of
// these once track_id is present; without this check the malformed call
// reaches the kernel and fails there instead of being caught here.
func messageLoopWaveformBakeCallMissingClipSource(call planner.ToolCall) bool {
	trackID := firstMapText(call.Args, "track_id")
	clipID := firstMapText(call.Args, "clip_id")
	filePath := firstMapText(call.Args, "file_path")
	return trackID != "" && clipID == "" && filePath == ""
}

func messageLoopIsDADAnalysisControlTool(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(call.Tool))
	}
	switch name {
	case "project.audio_analysis_start", "audio_analysis_start", "project.audio_analysis_cancel", "audio_analysis_cancel":
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
	case "project.state", "get_project_state", "track.list", "ccb.observation_catalog", "ccb_observation_catalog", "ccb.observation_request", "ccb_observation_request", "mix.read", "mix_read", "mix.derive", "mix_derive", "mix.report", "mix_report",
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
		"plugin_grabber.explain_controls", "plugin_grabber_explain_controls",
		"plugin_grabber.apply_eq_edits", "plugin_grabber_apply_eq_edits",
		"plugin_grabber.inspect_compressor", "plugin_grabber_inspect_compressor",
		"plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls",
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
		if summary := messageLoopCCBObservationSummary(record, result); len(summary) > 0 {
			out["ccb_observation_summary"] = summary
		} else if summary := mixObservationPromptSummary(result); len(summary) > 0 {
			out["mix_observation_summary"] = summary
		}
		if summary := messageLoopCompressorControlResultSummary(record, result); len(summary) > 0 {
			out["compressor_control_summary"] = summary
		} else if summary := messageLoopMediaArtifactResultSummary(record, result); len(summary) > 0 {
			out["media_artifact_summary"] = summary
		} else {
			out["result_summary"] = contextruntime.SummarizeToolResult(planner.ToolResult{
				ToolCallID: messageLoopText(record["tool_call_id"]),
				Tool:       messageLoopText(record["tool"]),
				Status:     messageLoopText(record["status"]),
				Error:      messageLoopText(record["error"]),
				Result:     result,
			}, opts)
		}
	}
	if verification, ok := record["verification"]; ok && !messageLoopEmptyValue(verification) {
		out["verification"] = contextruntime.CompactValue(verification, opts)
	}
	if projectHistory, ok := record["project_history"]; ok && !messageLoopEmptyValue(projectHistory) {
		if summary := messageLoopProjectHistoryPromptSummary(projectHistory); len(summary) > 0 {
			out["project_history_summary"] = summary
		}
	}
	return out
}

func messageLoopProjectHistoryPromptSummary(value any) map[string]any {
	row := messageLoopMapValue(value)
	if len(row) == 0 {
		return nil
	}
	return compactSelectedKeys(row, []string{
		"available", "active_branch", "detached", "head", "baseline_commit", "baseline_created",
		"commit_count", "worktree_count", "unsaved", "warnings",
	})
}

func messageLoopCCBObservationSummary(record, result map[string]any) map[string]any {
	name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
	if !messageLoopIsCCBObservationName(name) {
		return nil
	}
	if messageLoopIsCCBObservationRequestName(name) {
		bundle := messageLoopMapValue(result["bundle"])
		if len(bundle) == 0 {
			return compactSelectedKeys(result, []string{"status", "reason", "error"})
		}
		return compactSelectedKeys(bundle, []string{
			"schema_version", "bundle_id", "request_id", "status", "read_only", "mutation_authority",
			"observation_id", "mix_session_id", "target_ref", "project_binding", "freshness",
			"requested_views", "views", "evidence_refs", "limitations", "omission_reasons", "omissions",
			"disclosure_bytes", "max_disclosure_bytes",
		})
	}
	catalog := messageLoopMapValue(result["catalog"])
	return compactSelectedKeys(catalog, []string{"schema_version", "boundary", "target_ref", "views", "exclusions"})
}

func messageLoopCompressorControlResultSummary(record, result map[string]any) map[string]any {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
	switch name {
	case "plugin_grabber.inspect_compressor", "plugin_grabber_inspect_compressor":
		out := compactSelectedKeys(result, []string{"status", "schema_version", "track_id", "plugin_id", "classification", "confidence", "mapping_source"})
		if topology := messageLoopMapValue(result["control_topology"]); len(topology) > 0 {
			out["control_topology"] = compactSelectedKeys(topology, []string{"schema_version", "generation"})
		}
		stage := messageLoopMapValue(result["compressor_stage"])
		controls := make([]map[string]any, 0)
		for _, path := range messageLoopMapRows(stage["control_paths"]) {
			pathKey := firstMapText(path, "path_key")
			for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
				for _, binding := range messageLoopMapRows(path[section]) {
					controls = append(controls, messageLoopCompactCompressorBinding(binding, pathKey, section))
				}
			}
		}
		for _, binding := range messageLoopMapRows(stage["output"]) {
			controls = append(controls, messageLoopCompactCompressorBinding(binding, "stage_output", "output"))
		}
		out["controls"] = controls
		return out
	case "plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls":
		out := compactSelectedKeys(result, []string{"status", "atomic", "restored", "track_id", "plugin_id", "topology_generation", "restore_ref", "rejection_code", "message"})
		rows := make([]map[string]any, 0)
		for _, control := range messageLoopMapRows(result["controls"]) {
			row := compactSelectedKeys(control, []string{"status", "control_ref", "role", "path_key", "requested", "rejection_code"})
			row["actual_readback"] = messageLoopMapRows(control["actual_readback"])
			rows = append(rows, row)
		}
		out["controls"] = rows
		if rollback := messageLoopMapValue(result["rollback"]); len(rollback) > 0 {
			out["rollback"] = rollback
		}
		return out
	default:
		return nil
	}
}

func messageLoopCompactCompressorBinding(binding map[string]any, pathKey, section string) map[string]any {
	out := compactSelectedKeys(binding, []string{
		"control_ref", "role", "param_id", "name", "channel", "current_text", "current_physical", "reachable_values",
	})
	out["path_key"] = pathKey
	out["section"] = section
	if domain := messageLoopMapValue(binding["domain"]); len(domain) > 0 {
		out["domain"] = compactSelectedKeys(domain, []string{"unit", "min", "max", "scale", "status", "confidence"})
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
		if comProjection := messageLoopCOMProjectionForPrompt(observation); len(comProjection) > 0 {
			out["com_projection"] = comProjection
		}
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

func messageLoopCOMProjectionForPrompt(observation map[string]any) map[string]any {
	projection := messageLoopMapValue(observation["com_projection"])
	if len(projection) == 0 {
		if ctx := messageLoopMapValue(observation["context_pack"]); len(ctx) > 0 {
			latest := messageLoopMapValue(ctx["latest_observation"])
			projection = messageLoopMapValue(latest["com_projection"])
		}
	}
	if len(projection) == 0 {
		return nil
	}
	out := compactSelectedKeys(projection, []string{
		"schema_version", "com_version", "projection_id", "mode", "status", "observation_id",
		"mix_session_id", "target_ref", "processor_scope", "conditions", "evidence_inputs",
		"source_dynamics", "gain_action", "transient_response", "recovery_motion", "level_effect",
		"stereo_behavior", "trigger_relation", "behavior_change", "identifiability", "trust_quality",
		"evidence_refs", "limitations", "llm_context",
	})
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
		MaxListItems:           8,
		MaxPreviewBytes:        6 * 1024,
		SkipPluginSemanticLoad: true,
	}
}

func messageLoopMediaArtifactResultSummary(record map[string]any, result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
	rows := messageLoopMapRows(result["artifacts"])
	count := firstPositiveMapInt(result, "count", "artifact_count")
	if count <= 0 {
		count = len(rows)
	}
	if count <= 0 || (!messageLoopIsMediaArtifactToolName(name) && len(rows) == 0) {
		return nil
	}
	out := map[string]any{
		"status": messageLoopText(result["status"]),
		"count":  count,
	}
	for _, key := range []string{"skipped_count", "recursive", "limit"} {
		if value, ok := result[key]; ok && !messageLoopEmptyValue(value) {
			out[key] = value
		}
	}
	if locations := messageLoopStringSlice(result["locations"]); len(locations) > 0 {
		out["locations"] = firstMessageLoopStrings(locations, 4)
		if len(locations) > 4 {
			out["locations_omitted_count"] = len(locations) - 4
		}
	}
	if len(rows) > 0 {
		kinds := map[string]int{}
		sampleLimit := len(rows)
		if sampleLimit > 6 {
			sampleLimit = 6
		}
		samples := make([]map[string]any, 0, sampleLimit)
		for i, row := range rows {
			kind := firstMapText(row, "kind", "media_type", "type")
			if kind != "" {
				kinds[kind]++
			}
			if i >= 6 {
				continue
			}
			sample := map[string]any{}
			for _, key := range []string{"id", "kind", "title", "status", "summary", "mime", "size_bytes"} {
				if value, ok := row[key]; ok && !messageLoopEmptyValue(value) {
					sample[key] = value
				}
			}
			if len(sample) > 0 {
				samples = append(samples, sample)
			}
		}
		if len(kinds) > 0 {
			out["kind_counts"] = kinds
		}
		if len(samples) > 0 {
			out["sample_artifacts"] = samples
		}
		if len(rows) > len(samples) {
			out["sample_omitted_count"] = len(rows) - len(samples)
		}
	}
	removeMessageLoopEmpty(out)
	return out
}

func firstMessageLoopStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}

func removeMessageLoopEmpty(row map[string]any) {
	for key, value := range row {
		if messageLoopEmptyValue(value) {
			delete(row, key)
		}
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
	if reply := messageLoopCompressorApplyFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply, true
	}
	if reply := messageLoopStemsImportFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply, true
	}
	if reply := messageLoopTrackOrganizationFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply, true
	}
	if reply := messageLoopProjectMarkersFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply, true
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

func messageLoopCompressorApplyFastCompleteReply(record map[string]any) string {
	if !messageLoopExecutionSucceeded(record) || !messageLoopIsCompressorApplyName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
		return ""
	}
	result := messageLoopMapValue(record["result"])
	status := strings.ToLower(strings.TrimSpace(messageLoopText(result["status"])))
	if status != "exact" && status != "quantized" {
		return ""
	}
	controls := messageLoopMapRows(result["controls"])
	if len(controls) == 0 {
		return ""
	}
	parts := make([]string, 0, len(controls))
	for _, control := range controls {
		readback := messageLoopMapRows(control["actual_readback"])
		if len(readback) == 0 {
			return ""
		}
		row := readback[0]
		role := strings.TrimSpace(messageLoopText(control["role"]))
		if role == "" {
			role = strings.TrimSpace(messageLoopText(row["role"]))
		}
		if role == "" {
			role = "control"
		}
		value := strings.TrimSpace(messageLoopText(row["value_text"]))
		if value == "" {
			if physical, ok := row["physical"]; ok && physical != nil {
				value = fmt.Sprint(physical)
			} else if normalized, ok := row["normalized"]; ok {
				value = fmt.Sprint(normalized)
			}
		}
		if value == "" {
			return ""
		}
		parts = append(parts, fmt.Sprintf("%s 实际读回 %s", compressorControlDisplayName(role), value))
	}
	return fmt.Sprintf("已完成并通过 typed 读回验证（%s）：%s。", status, strings.Join(parts, "；"))
}

func compressorControlDisplayName(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "threshold", "input_threshold", "reduction_amount":
		return "Threshold"
	case "ratio":
		return "Ratio"
	case "attack":
		return "Attack"
	case "release":
		return "Release"
	case "knee":
		return "Knee"
	case "makeup", "makeup_gain":
		return "Makeup"
	case "mix":
		return "Mix"
	case "output", "output_gain", "ceiling":
		return "Output"
	default:
		return role
	}
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

func messageLoopExecutionFallbackAfterLLMError(state *runState, err error) (string, bool) {
	if state == nil || err == nil || !messageLoopTransientLLMError(err) {
		return "", false
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		ver, ok := messageLoopExecutionVerification(record)
		if ok {
			switch ver.Status {
			case verificationVerified, verificationNotRequired:
			default:
				continue
			}
		}
		reply := messageLoopVerifiedExecutionFallbackReply(state, record, ver)
		if strings.TrimSpace(reply) == "" {
			continue
		}
		return reply + "\n\n" + "\u6700\u7ec8\u81ea\u7136\u8bed\u8a00\u56de\u590d\u9047\u5230\u4e34\u65f6\u9519\u8bef\uff0c\u6211\u5148\u6839\u636e\u5df2\u9a8c\u8bc1\u7684\u6267\u884c\u7ed3\u679c\u7ed9\u51fa\u8fd9\u4e2a\u6458\u8981\u3002", true
	}
	return "", false
}

func messageLoopFreeStateTransientPauseReply(state *runState) string {
	resume := "开放语义处理尚未完成；原始意图和当前观察状态均已保留。本轮因 LLM 服务临时错误暂停，可以继续恢复尚未完成的判断。"
	if state == nil || len(state.executed) == 0 {
		return resume
	}
	return "本轮已完成的工具调用及其验证回执已保留。\n\n" + resume
}

func messageLoopVerifiedExecutionFallbackReply(state *runState, record map[string]any, ver planner.VerificationResult) string {
	if state == nil || len(record) == 0 {
		return ""
	}
	if reply := messageLoopStemsImportFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply
	}
	if reply := messageLoopTrackOrganizationFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply
	}
	if reply := messageLoopProjectMarkersFastCompleteReply(record); strings.TrimSpace(reply) != "" {
		return reply
	}
	if messageLoopMediaArtifactResultSummary(record, messageLoopMapValue(record["result"])) != nil {
		return messageLoopMediaArtifactsFastCompleteReply(record)
	}
	postcondition := strings.TrimSpace(ver.Postcondition)
	if postcondition == "" {
		postcondition = postconditionFor(planner.ToolCall{Tool: messageLoopText(record["tool"])}, executorpkg.Result{CommandName: messageLoopText(record["command_name"])})
	}
	switch postcondition {
	case postconditionTrackPresent:
		return messageLoopTrackPresentFastCompleteReply(state, record, ver)
	case postconditionClipPresent:
		return messageLoopClipExecutionFallbackReply(state, record, ver)
	case postconditionMIDINotesPresent:
		return messageLoopMIDINotesFastCompleteReply(state)
	case postconditionRackNodePresent:
		return messageLoopRackNodeFastCompleteReply(record, ver)
	case postconditionNoObservableDawPostcondition, "":
		return "\u5de5\u5177\u64cd\u4f5c\u5df2\u5b8c\u6210\u3002"
	default:
		return "\u5de5\u5177\u64cd\u4f5c\u5df2\u6267\u884c\u5e76\u8bb0\u5f55\u3002"
	}
}

func messageLoopProjectMarkersFastCompleteReply(record map[string]any) string {
	if len(record) == 0 || !messageLoopExecutionSucceeded(record) {
		return ""
	}
	result := messageLoopMapValue(record["result"])
	actionName := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		messageLoopText(record["tool"]),
		messageLoopText(record["command_name"]),
		firstMapText(result, "command", "cmd", "action"),
	)))
	switch actionName {
	case "project.markers.apply_section_markers", "project_apply_section_markers":
		written := firstPositiveMapInt(result, "written_count", "marker_count")
		if written <= 0 {
			written = messageLoopListCount(result["markers"])
		}
		if written <= 0 {
			return ""
		}
		return fmt.Sprintf("已写入 %d 个段落 marker。", written)
	case "project.markers.upsert", "project_marker_upsert":
		marker := messageLoopMapValue(result["marker"])
		name := firstMapText(marker, "name", "label")
		kind := firstMapText(marker, "kind")
		if strings.EqualFold(kind, "range") {
			if name != "" {
				return fmt.Sprintf("已写入段落 marker：%s。", name)
			}
			return "已写入段落 marker。"
		}
		if name != "" {
			return fmt.Sprintf("已写入定位 marker：%s。", name)
		}
		return "已写入定位 marker。"
	case "project.markers.rename", "project_marker_rename":
		return "marker 已重命名。"
	case "project.markers.delete", "project_marker_delete":
		return "marker 已删除。"
	default:
		return ""
	}
}

func messageLoopTrackOrganizationFastCompleteReply(record map[string]any) string {
	if len(record) == 0 || !messageLoopExecutionSucceeded(record) {
		return ""
	}
	result := messageLoopMapValue(record["result"])
	actionName := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		messageLoopText(record["tool"]),
		messageLoopText(record["command_name"]),
		firstMapText(result, "command", "cmd", "action"),
	)))
	if actionName != "project.apply_track_organization" {
		return ""
	}
	createdFolders := firstPositiveMapInt(result, "created_folder_count", "folder_count", "folders_created")
	movedTracks := firstPositiveMapInt(result, "moved_track_count", "tracks_moved")
	groups := messageLoopMapRows(result["groups"])
	if createdFolders <= 0 {
		createdFolders = messageLoopListCount(result["created_folder_ids"])
	}
	if movedTracks <= 0 {
		for _, group := range groups {
			movedTracks += firstPositiveMapInt(group, "moved_track_count", "track_count")
		}
	}
	groupParts := []string{}
	routingBusEnabled := 0
	for _, group := range groups {
		name := firstMapText(group, "folder_name", "name", "group_name", "label")
		count := firstPositiveMapInt(group, "moved_track_count", "track_count")
		if name != "" && count > 0 && len(groupParts) < 8 {
			groupParts = append(groupParts, fmt.Sprintf("%s x%d", name, count))
		}
		if enabled, ok := firstMapBool(group, "routing_bus_enabled"); ok && enabled {
			routingBusEnabled++
		}
	}
	lines := []string{"整理完成。", ""}
	if createdFolders > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("新建文件夹：%d", createdFolders))
	}
	if movedTracks > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("已移动轨道：%d", movedTracks))
	}
	if len(groupParts) > 0 {
		appendMessageLoopBullet(&lines, "分组："+strings.Join(groupParts, " / "))
	}
	if routingBusEnabled > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("路由 Bus：已启用 %d 个", routingBusEnabled))
	} else {
		appendMessageLoopBullet(&lines, "路由 Bus：未启用，当前是普通文件夹整理")
	}
	return strings.Join(lines, "\n")
}

func messageLoopStemsImportFastCompleteReply(record map[string]any) string {
	if len(record) == 0 || !messageLoopExecutionSucceeded(record) {
		return ""
	}
	result := messageLoopMapValue(record["result"])
	actionName := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		messageLoopText(record["tool"]),
		messageLoopText(record["command_name"]),
		firstMapText(result, "command", "cmd", "action"),
	)))
	if !messageLoopIsStemsImportName(actionName) && actionName != "import_folder_as_stems" {
		return ""
	}
	summary := messageLoopMapValue(result["summary"])
	rows := messageLoopStemsImportRows(result)
	messageLoopRefreshImportTIMFromDADSnapshot(result)
	settings := messageLoopMapValue(result["audio_settings_snapshot"])
	analysisJob := messageLoopMapValue(result["analysis_job"])

	tracksCreated := firstPositiveMapInt(summary, "tracks_created", "created_track_count", "created_tracks", "track_count", "tracks_to_create")
	if tracksCreated <= 0 {
		tracksCreated = firstPositiveMapInt(result, "tracks_created", "created_track_count", "created_track_count_reported")
	}
	if tracksCreated <= 0 {
		tracksCreated = messageLoopListCount(result["created_track_ids"])
	}
	if tracksCreated <= 0 {
		tracksCreated = messageLoopListCount(result["created_track_ids_preview"])
	}
	if tracksCreated <= 0 {
		tracksCreated = len(rows)
	}

	clipsCreated := firstPositiveMapInt(summary, "clips_created", "created_clip_count", "created_clips", "clip_count")
	if clipsCreated <= 0 {
		clipsCreated = firstPositiveMapInt(result, "clips_created", "created_clip_count", "created_clip_count_reported")
	}
	if clipsCreated <= 0 {
		clipsCreated = messageLoopListCount(result["created_clip_ids"])
	}
	if clipsCreated <= 0 {
		clipsCreated = messageLoopListCount(result["created_clip_ids_preview"])
	}
	if clipsCreated <= 0 && tracksCreated > 0 {
		clipsCreated = tracksCreated
	}
	timReady := messageLoopTIMImportResultReadyForA2(result, tracksCreated)
	if !timReady {
		return ""
	}

	lines := []string{
		fmt.Sprintf("A1 工程接收整理完成：已导入 %d 条轨道 / %d 个音频片段。", tracksCreated, clipsCreated),
	}

	discovered := firstPositiveMapInt(summary, "discovered_audio_file_count", "audio_file_count", "file_count", "discovered_file_count")
	readable := firstPositiveMapInt(summary, "readable_file_count", "readable_audio_file_count")
	unreadable := firstPositiveMapInt(summary, "unreadable_file_count", "unreadable_audio_file_count")
	if unreadable <= 0 {
		unreadable = messageLoopCountRowsOrList(result["unreadable_files"])
	}
	if unreadable <= 0 {
		unreadable = messageLoopCountRowsOrList(result["unreadable_file_examples"])
	}
	if discovered > 0 || readable > 0 || unreadable > 0 {
		lines = append(lines, fmt.Sprintf("素材清点：发现 %d 个音频文件，可读取 %d 个，不可读 %d 个。", discovered, readable, unreadable))
	}

	firstRow := messageLoopFirstStemsImportRow(rows)
	start, hasStart := firstNumericMapValue(summary, "start_time_seconds", "start_time", "start_seconds", "position_seconds")
	if !hasStart {
		start, hasStart = firstNumericMapValue(firstRow, "start_time_seconds", "start_time", "start_seconds", "position_seconds")
	}
	length, hasLength := firstNumericMapValue(summary, "edit_length_seconds", "duration_seconds", "max_duration_seconds", "longest_duration_seconds", "timeline_length_seconds")
	if !hasLength {
		length, hasLength = firstNumericMapValue(firstRow, "duration_seconds", "edit_length_seconds", "length_seconds")
	}
	if hasStart || hasLength {
		parts := []string{}
		if hasStart {
			parts = append(parts, fmt.Sprintf("start %.2fs", start))
		}
		if hasLength {
			parts = append(parts, fmt.Sprintf("length %.2fs", length))
		}
		lines = append(lines, "时间对齐："+strings.Join(parts, " / ")+"。")
	}

	sourceSpec := messageLoopAudioSpecText(
		firstPositiveMapInt(firstRow, "source_sample_rate_hz", "sample_rate_hz"),
		firstPositiveMapInt(firstRow, "source_bit_depth", "bit_depth"),
		firstMapText(firstRow, "source_pcm_format", "pcm_format"),
	)
	if channels := firstPositiveMapInt(firstRow, "channel_count", "channels"); channels > 0 {
		if sourceSpec != "" {
			sourceSpec += fmt.Sprintf(" / %dch", channels)
		} else {
			sourceSpec = fmt.Sprintf("%dch", channels)
		}
	}
	if sourceSpec != "" {
		lines = append(lines, "Source preview: "+sourceSpec+"。")
	}

	projectSpec := messageLoopAudioSpecText(
		firstPositiveMapInt(settings, "sample_rate_hz", "project_sample_rate_hz"),
		firstPositiveMapInt(settings, "record_bit_depth", "project_record_bit_depth"),
		firstMapText(settings, "pcm_format"),
	)
	sampleRateMismatches := firstPositiveMapInt(summary, "sample_rate_mismatch_count", "sample_rate_mismatches")
	if sampleRateMismatches <= 0 {
		sampleRateMismatches = messageLoopCountRowsOrList(result["sample_rate_mismatches"])
	}
	if sampleRateMismatches <= 0 {
		sampleRateMismatches = messageLoopCountRowsOrList(result["sample_rate_mismatch_examples"])
	}
	bitDepthMismatches := firstPositiveMapInt(summary, "bit_depth_or_format_mismatch_count", "bit_depth_or_format_mismatches")
	if bitDepthMismatches <= 0 {
		bitDepthMismatches = messageLoopCountRowsOrList(result["bit_depth_or_format_mismatches"])
	}
	if bitDepthMismatches <= 0 {
		bitDepthMismatches = messageLoopCountRowsOrList(result["bit_depth_or_format_mismatch_examples"])
	}
	if projectSpec != "" {
		lines = append(lines, fmt.Sprintf("Project spec: %s；sample-rate mismatch: %d，bit-depth/format mismatch: %d。", projectSpec, sampleRateMismatches, bitDepthMismatches))
	} else {
		lines = append(lines, fmt.Sprintf("规格预检：sample-rate mismatch: %d，bit-depth/format mismatch: %d。", sampleRateMismatches, bitDepthMismatches))
	}
	if changed, ok := firstMapBool(result, "project_audio_settings_changed_before_import"); ok && changed {
		patch := messageLoopMapValue(result["project_audio_settings_patch"])
		if sampleRateHz := firstPositiveMapInt(patch, "sample_rate_hz"); sampleRateHz > 0 {
			lines = append(lines, fmt.Sprintf("工程采样率已在导入前切换为 %d Hz。", sampleRateHz))
		}
	}
	projectSettingsChanged := false
	projectSettingsPatchSpec := ""
	if changed, ok := firstMapBool(result, "project_audio_settings_changed_before_import"); ok && changed {
		projectSettingsChanged = true
		patch := messageLoopMapValue(result["project_audio_settings_patch"])
		projectSettingsPatchSpec = messageLoopAudioSpecText(
			firstPositiveMapInt(patch, "sample_rate_hz", "project_sample_rate_hz"),
			firstPositiveMapInt(patch, "record_bit_depth", "render_default_bit_depth", "bit_depth"),
			firstMapText(patch, "pcm_format"),
		)
		if projectSettingsPatchSpec == "" {
			projectSettingsPatchSpec = messageLoopAudioSpecText(
				firstPositiveMapInt(settings, "sample_rate_hz", "project_sample_rate_hz"),
				firstPositiveMapInt(settings, "record_bit_depth", "project_record_bit_depth"),
				firstMapText(settings, "pcm_format"),
			)
		}
	}

	copyPolicy := firstNonEmpty(
		firstMapText(summary, "copy_policy", "media_copy_policy"),
		firstMapText(result, "copy_policy", "media_copy_policy"),
		firstMapText(settings, "media_copy_policy"),
		firstMapText(firstRow, "media_copy_policy"),
	)
	if copyPolicy != "" {
		lines = append(lines, "媒体策略："+copyPolicy+"。")
	}

	analysisDeferred, hasAnalysisDeferred := firstMapBool(result, "analysis_deferred")
	if !hasAnalysisDeferred {
		analysisDeferred, hasAnalysisDeferred = firstMapBool(summary, "analysis_deferred")
	}
	queueStatus := firstNonEmpty(
		firstMapText(result, "analysis_queue_status", "background_analysis_status", "baking_status"),
		firstMapText(summary, "analysis_queue_status", "background_analysis_status", "baking_status"),
		firstMapText(analysisJob, "analysis_queue_status", "status"),
	)
	jobID := firstNonEmpty(
		firstMapText(result, "analysis_job_id", "job_id", "audio_analysis_job_id"),
		firstMapText(summary, "analysis_job_id", "job_id", "audio_analysis_job_id"),
		firstMapText(analysisJob, "analysis_job_id", "job_id"),
	)
	queuedJobs := firstPositiveMapInt(result, "analysis_jobs_queued", "analysis_total_clips", "analysis_total_feature_jobs")
	if queuedJobs <= 0 {
		queuedJobs = firstPositiveMapInt(summary, "analysis_jobs_queued", "analysis_total_clips", "analysis_total_feature_jobs")
	}
	if hasAnalysisDeferred || queueStatus != "" || jobID != "" || queuedJobs > 0 {
		prefix := "Analysis"
		if hasAnalysisDeferred && analysisDeferred {
			prefix = "Analysis deferred"
		}
		parts := []string{}
		if queueStatus != "" {
			parts = append(parts, "status "+queueStatus)
		}
		if queuedJobs > 0 {
			parts = append(parts, fmt.Sprintf("queued %d", queuedJobs))
		}
		if jobID != "" {
			parts = append(parts, "job "+jobID)
		}
		if len(parts) > 0 {
			lines = append(lines, prefix+"："+strings.Join(parts, " / ")+"。")
		} else {
			lines = append(lines, prefix+"。")
		}
	}

	return messageLoopFormatStemsImportDADGatedReply(messageLoopStemsImportReportData{
		TracksCreated:            tracksCreated,
		ClipsCreated:             clipsCreated,
		Discovered:               discovered,
		Readable:                 readable,
		Unreadable:               unreadable,
		StartSeconds:             start,
		HasStart:                 hasStart,
		LengthSeconds:            length,
		HasLength:                hasLength,
		SourceSpec:               sourceSpec,
		ProjectSpec:              projectSpec,
		SampleRateMismatches:     sampleRateMismatches,
		BitDepthMismatches:       bitDepthMismatches,
		ProjectSettingsChanged:   projectSettingsChanged,
		ProjectSettingsPatchSpec: projectSettingsPatchSpec,
		CopyPolicy:               copyPolicy,
		AnalysisDeferred:         analysisDeferred,
		HasAnalysisDeferred:      hasAnalysisDeferred,
		AnalysisStatus:           queueStatus,
		AnalysisQueued:           queuedJobs,
		AnalysisJobID:            jobID,
		WarningCount:             messageLoopCountRowsOrList(result["warnings"]),
		TIMReady:                 timReady,
		TIMLines:                 messageLoopTIMImportReportBullets(result, tracksCreated, clipsCreated),
		TOMLines:                 messageLoopTOMOrganizationReportBullets(result),
		A4EPMLines:               messageLoopEPMClipEditReportBullets(result),
		A5EPMLines:               messageLoopEPMSectionMapReportBullets(result),
	})
}

type messageLoopStemsImportReportData struct {
	TracksCreated            int
	ClipsCreated             int
	Discovered               int
	Readable                 int
	Unreadable               int
	StartSeconds             float64
	HasStart                 bool
	LengthSeconds            float64
	HasLength                bool
	SourceSpec               string
	ProjectSpec              string
	SampleRateMismatches     int
	BitDepthMismatches       int
	ProjectSettingsChanged   bool
	ProjectSettingsPatchSpec string
	CopyPolicy               string
	AnalysisDeferred         bool
	HasAnalysisDeferred      bool
	AnalysisStatus           string
	AnalysisQueued           int
	AnalysisJobID            string
	WarningCount             int
	TIMReady                 bool
	TIMLines                 []string
	TOMLines                 []string
	A4EPMLines               []string
	A5EPMLines               []string
}

func messageLoopFormatStemsImportDADGatedReply(data messageLoopStemsImportReportData) string {
	if !data.TIMReady {
		return ""
	}
	return messageLoopProjectBlackboardReportFromImport(data)
}

func (l *MessageLoop) messageLoopStemsImportCompleteReplyWithDADGate(ctx context.Context, r *Runner, state *runState, record map[string]any) (string, bool, Result) {
	result, ok := messageLoopStemsImportResultFromRecord(record)
	if !ok {
		return "", false, Result{}
	}
	expectedTracks := messageLoopStemsImportExpectedTrackCount(result)
	messageLoopRefreshImportTIMFromDADSnapshot(result)
	if ready, readyCount, totalCount := messageLoopTIMImportResultAcousticReady(result, expectedTracks); ready {
		messageLoopSetImportDADGate(result, "ready", readyCount, totalCount)
		return messageLoopStemsImportFastCompleteReply(record), false, Result{}
	}
	if !allowedTool("project.audio_analysis_status", state.input.AllowedTools) {
		return "", true, r.fail(state, fmt.Errorf("project.audio_analysis_status is required to wait for automatic DAD facts before replying with A2 TIM"))
	}

	jobID := messageLoopAnalysisJobIDFromImportResult(result)
	interval := messageLoopDADGatePollInterval()
	idleTimeout := messageLoopDADGateIdleTimeout()
	maxWait := messageLoopDADGateMaxWait()
	waitStartedAt := time.Now()
	lastProgressAt := waitStartedAt
	_, readyCount, totalCount := messageLoopTIMImportResultAcousticReady(result, expectedTracks)
	lastProgressSignature := messageLoopDADGateProgressSignature(result, readyCount, totalCount)
	for attempt := 1; ; attempt++ {
		if messageLoopCanReadDADStatus(state, jobID) {
			call := messageLoopDADStatusToolCall(jobID, attempt)
			toolStarted := time.Now()
			toolCallsUsedBeforeStatusRead := state.toolCallsUsed
			stopped, toolResult := r.executeTool(ctx, state, call, false, nil)
			state.toolCallsUsed = toolCallsUsedBeforeStatusRead
			l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false dad_gate=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, toolResult.Status)
			if stopped {
				return "", true, toolResult
			}
			if len(state.executed) > 0 {
				statusRecord := state.executed[len(state.executed)-1]
				if messageLoopExecutionSucceeded(statusRecord) {
					messageLoopMergeDADStatusIntoImportResult(result, messageLoopMapValue(statusRecord["result"]))
				}
			}
		}
		messageLoopRefreshImportTIMFromDADSnapshot(result)
		ready, readyCount, totalCount := messageLoopTIMImportResultAcousticReady(result, expectedTracks)
		if ready {
			messageLoopSetImportDADGate(result, "ready", readyCount, totalCount)
			return messageLoopStemsImportFastCompleteReply(record), false, Result{}
		}
		now := time.Now()
		progressSignature := messageLoopDADGateProgressSignature(result, readyCount, totalCount)
		if progressSignature != lastProgressSignature {
			lastProgressSignature = progressSignature
			lastProgressAt = now
		}
		messageLoopSetImportDADGate(result, "polling", readyCount, totalCount)
		if idleTimeout > 0 && now.Sub(lastProgressAt) >= idleTimeout {
			messageLoopSetImportDADGate(result, "timeout", readyCount, totalCount)
			return "", true, r.fail(state, messageLoopDADGateTimeoutError("no_progress", jobID, result, readyCount, totalCount, idleTimeout, maxWait))
		}
		if maxWait > 0 && now.Sub(waitStartedAt) >= maxWait {
			messageLoopSetImportDADGate(result, "timeout", readyCount, totalCount)
			return "", true, r.fail(state, messageLoopDADGateTimeoutError("max_wait", jobID, result, readyCount, totalCount, idleTimeout, maxWait))
		}
		if interval > 0 {
			if err := messageLoopSleepContext(ctx, interval); err != nil {
				return "", true, r.fail(state, err)
			}
		}
	}
}

func messageLoopStemsImportResultFromRecord(record map[string]any) (map[string]any, bool) {
	if len(record) == 0 || !messageLoopExecutionSucceeded(record) {
		return nil, false
	}
	result := messageLoopMapValue(record["result"])
	actionName := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		messageLoopText(record["tool"]),
		messageLoopText(record["command_name"]),
		firstMapText(result, "command", "cmd", "action"),
	)))
	if !messageLoopIsStemsImportName(actionName) && actionName != "import_folder_as_stems" {
		return nil, false
	}
	return result, len(result) > 0
}

func messageLoopDADGatePollInterval() time.Duration {
	return messageLoopDurationEnvMS("VIT_AGENT_DAD_GATE_POLL_MS", 250*time.Millisecond)
}

func messageLoopDADGateIdleTimeout() time.Duration {
	return messageLoopDurationEnvMS("VIT_AGENT_DAD_GATE_IDLE_TIMEOUT_MS", 45*time.Second)
}

func messageLoopDADGateMaxWait() time.Duration {
	return messageLoopDurationEnvMS("VIT_AGENT_DAD_GATE_MAX_WAIT_MS", 10*time.Minute)
}

func messageLoopDurationEnvMS(name string, fallback time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
			return time.Duration(value) * time.Millisecond
		}
	}
	return fallback
}

func messageLoopDADGateProgressSignature(result map[string]any, readyCount, totalCount int) string {
	return fmt.Sprintf(
		"ready=%d/%d|status=%s|submitted_clips=%.0f|pending_clips=%.0f|submitted_feature_jobs=%.0f|pending_feature_jobs=%.0f|progress=%.4f|progress_percent=%.4f",
		readyCount,
		totalCount,
		messageLoopDADGateFieldText(result, "analysis_queue_status", "status"),
		messageLoopDADGateFieldNumber(result, "submitted_clips"),
		messageLoopDADGateFieldNumber(result, "pending_clips"),
		messageLoopDADGateFieldNumber(result, "submitted_feature_jobs"),
		messageLoopDADGateFieldNumber(result, "pending_feature_jobs"),
		messageLoopDADGateFieldNumber(result, "progress"),
		messageLoopDADGateFieldNumber(result, "progress_percent"),
	)
}

func messageLoopDADGateTimeoutError(reason, jobID string, result map[string]any, readyCount, totalCount int, idleTimeout, maxWait time.Duration) error {
	reasonText := "DAD 自动分析长时间没有进展"
	if reason == "max_wait" {
		reasonText = "DAD 自动分析超过总等待上限"
	}
	jobLabel := firstNonEmpty(strings.TrimSpace(jobID), messageLoopAnalysisJobIDFromImportResult(result), "latest")
	status := firstNonEmpty(messageLoopDADGateFieldText(result, "analysis_queue_status", "status"), "unknown")
	submittedFeatureJobs := int(messageLoopDADGateFieldNumber(result, "submitted_feature_jobs"))
	totalFeatureJobs := int(messageLoopDADGateFieldNumber(result, "total_feature_jobs"))
	pendingFeatureJobs := int(messageLoopDADGateFieldNumber(result, "pending_feature_jobs"))
	return fmt.Errorf(
		"A2 TIM 等待 DAD 自动分析超时：%s（job=%s，队列=%s，A2 ready=%d/%d，feature jobs=%d/%d，pending=%d，idle_timeout=%s，max_wait=%s）。agent 只读取 project.audio_analysis_status，没有启动或取消 DAD；请检查 VitApp 自动分析队列",
		reasonText,
		jobLabel,
		status,
		readyCount,
		totalCount,
		submittedFeatureJobs,
		totalFeatureJobs,
		pendingFeatureJobs,
		idleTimeout,
		maxWait,
	)
}

func messageLoopDADGateFieldText(result map[string]any, keys ...string) string {
	for _, source := range messageLoopDADGateFieldMaps(result) {
		if text := firstMapText(source, keys...); text != "" {
			return text
		}
	}
	return ""
}

func messageLoopDADGateFieldNumber(result map[string]any, keys ...string) float64 {
	for _, source := range messageLoopDADGateFieldMaps(result) {
		for _, key := range keys {
			if value, ok := source[key]; ok && !messageLoopEmptyValue(value) {
				return messageLoopImportNumber(value)
			}
		}
	}
	return 0
}

func messageLoopDADGateFieldMaps(result map[string]any) []map[string]any {
	if len(result) == 0 {
		return nil
	}
	out := []map[string]any{result}
	if summary := messageLoopMapValue(result["summary"]); len(summary) > 0 {
		out = append(out, summary)
		if job := messageLoopMapValue(summary["analysis_job"]); len(job) > 0 {
			out = append(out, job)
		}
	}
	if job := messageLoopMapValue(result["analysis_job"]); len(job) > 0 {
		out = append(out, job)
	}
	return out
}

func messageLoopSleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func messageLoopCanReadDADStatus(state *runState, _ string) bool {
	if state == nil || !allowedTool("project.audio_analysis_status", state.input.AllowedTools) {
		return false
	}
	return true
}

func messageLoopImportHasAnalysisJobHint(state *runState) bool {
	if state == nil {
		return false
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		result := messageLoopMapValue(state.executed[i]["result"])
		if messageLoopAnalysisJobIDFromImportResult(result) != "" {
			return true
		}
	}
	return false
}

func messageLoopDADStatusToolCall(jobID string, attempt int) planner.ToolCall {
	args := map[string]any{"latest": true}
	if strings.TrimSpace(jobID) != "" {
		args["analysis_job_id"] = strings.TrimSpace(jobID)
		args["job_id"] = strings.TrimSpace(jobID)
	}
	command := cloneMap(args)
	command["cmd"] = "project.audio_analysis_status"
	return planner.ToolCall{
		ID:      fmt.Sprintf("dad_readiness_status_%d", attempt),
		Tool:    "project.audio_analysis_status",
		Args:    args,
		Command: command,
		Reason:  "read DAD queue status without starting analysis",
	}
}

func messageLoopMergeDADStatusIntoImportResult(importResult, statusResult map[string]any) {
	if len(importResult) == 0 || len(statusResult) == 0 {
		return
	}
	status := messageLoopMapValue(statusResult["analysis_job"])
	if len(status) == 0 {
		status = statusResult
	}
	for _, key := range []string{
		"analysis_queue_status", "status", "submitted_clips", "pending_clips", "total_clips",
		"submitted_feature_jobs", "pending_feature_jobs", "total_feature_jobs", "progress", "progress_percent",
		"dad_fact_status", "dad_fact_ready_count", "dad_fact_total_count", "dad_fact_pending_count", "dad_fact_failed_count",
		"dad_fact_completion_scope", "track_waveform_envelopes",
		"feature_snapshot_path", "mixboard_feature_snapshot_path",
	} {
		if value, ok := status[key]; ok && !messageLoopEmptyValue(value) {
			switch key {
			case "status":
				importResult["analysis_queue_status"] = value
			default:
				importResult[key] = value
			}
		}
	}
	if proj := messageLoopMapValue(statusResult["tim_projection"]); len(proj) > 0 {
		importResult["tim_projection"] = proj
	}
	if proj := messageLoopMapValue(status["tim_projection"]); len(proj) > 0 {
		importResult["tim_projection"] = proj
	}
}

func messageLoopRefreshImportTIMFromDADSnapshot(result map[string]any) {
	if len(result) == 0 {
		return
	}
	expectedTracks := messageLoopStemsImportExpectedTrackCount(result)
	candidate := messageLoopTIMProjectionFromImportDADSnapshot(result)
	if len(candidate) == 0 {
		return
	}
	current := messageLoopMapValue(result["tim_projection"])
	if messageLoopTIMProjectionShouldReplace(current, candidate, expectedTracks) {
		result["tim_projection"] = candidate
	}
}

func messageLoopTIMProjectionShouldReplace(current, candidate map[string]any, expectedTracks int) bool {
	if len(candidate) == 0 {
		return false
	}
	if len(current) == 0 {
		return true
	}
	candidateReady, candidateReadyCount, candidateTotal := messageLoopTIMProjectionAcousticReady(candidate, expectedTracks)
	currentReady, currentReadyCount, currentTotal := messageLoopTIMProjectionAcousticReady(current, expectedTracks)
	if candidateReady && !currentReady {
		return true
	}
	if expectedTracks > 0 && candidateTotal >= expectedTracks && currentTotal < expectedTracks {
		return true
	}
	if candidateReadyCount > currentReadyCount {
		return true
	}
	return candidateReadyCount == currentReadyCount && candidateTotal > currentTotal
}

func messageLoopTIMProjectionFromImportDADSnapshot(result map[string]any) map[string]any {
	rows := messageLoopStemsImportRows(result)
	if len(rows) == 0 {
		return nil
	}
	projectState := messageLoopProjectStateFromImportRows(result, rows)
	if messageLoopListCount(projectState["tracks"]) == 0 {
		return nil
	}
	args := map[string]any{}
	if duration, ok := firstNumericMapValue(messageLoopMapValue(result["summary"]), "edit_length_seconds", "duration_seconds", "max_duration_seconds", "longest_duration_seconds", "timeline_length_seconds"); ok && duration > 0 {
		args["duration_seconds"] = duration
	}
	if snapshot := messageLoopImportDADFeatureSnapshotFromStatus(result, rows); len(snapshot) > 0 {
		args["feature_snapshot"] = snapshot
	} else {
		for _, source := range []map[string]any{result, messageLoopMapValue(result["summary"])} {
			if path := firstMapText(source, "feature_snapshot_path", "mixboard_feature_snapshot_path"); path != "" {
				args["feature_snapshot_path"] = path
				if snapshot := messageLoopFilteredImportDADFeatureSnapshot(path, rows); len(snapshot) > 0 {
					args["feature_snapshot"] = snapshot
				}
				break
			}
		}
	}
	obs := mixboard.BuildObservation(mixboard.Request{
		MixSessionID: "stems_import_dad_gate",
		TargetRef:    mixboard.TargetRef{Kind: "project", ID: "current", Label: "Imported project"},
		ListenScope: mixboard.ListenScope{
			Time:   mixboard.ListenTimeScope{Mode: "full_project"},
			Source: mixboard.ListenSourceScope{Mode: "full_project"},
		},
		ProjectState: projectState,
		Args:         args,
	}, time.Now().UTC().Format(time.RFC3339Nano))
	if obs.TIMProjection == nil {
		return nil
	}
	return tim.ContextProjectionMap(*obs.TIMProjection)
}

func messageLoopImportDADFeatureSnapshotFromStatus(result map[string]any, importRows []map[string]any) map[string]any {
	if len(result) == 0 || len(importRows) == 0 {
		return nil
	}
	targets := messageLoopBuildImportDADTargets(importRows)
	if len(targets.byTrackClip) == 0 && len(targets.bySourcePath) == 0 {
		return nil
	}
	rows := messageLoopImportDADStatusWaveformRows(result)
	if len(rows) == 0 {
		return nil
	}
	filteredRows := messageLoopFilterImportDADTrackWaveformRows(rows, targets)
	if len(filteredRows) == 0 {
		return nil
	}
	return messageLoopImportDADFeatureSnapshotFromRows(filteredRows, "audio_analysis_status")
}

func messageLoopImportDADStatusWaveformRows(result map[string]any) []map[string]any {
	if len(result) == 0 {
		return nil
	}
	sources := []any{
		result["track_waveform_envelopes"],
		messageLoopMapValue(result["analysis_job"])["track_waveform_envelopes"],
		messageLoopMapValue(result["summary"])["track_waveform_envelopes"],
	}
	rows := []map[string]any{}
	for _, source := range sources {
		for _, row := range messageLoopMapRows(source) {
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func messageLoopImportDADFeatureSnapshotFromRows(rows []map[string]any, source string) map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := map[string]any{
		"schema_version":           "mixboard_feature_snapshot.v1",
		"latest_request":           map[string]any{"request_id": source, "status": "ready", "scope": "stems_import"},
		"track_waveform_envelopes": rows,
		"waveform_envelope":        map[string]any{"status": "missing"},
		"spectrogram_tiles":        map[string]any{"status": "missing"},
		"band_energy_summary":      map[string]any{"status": "missing"},
		"stereo_relation_summary":  map[string]any{"status": "missing"},
		"loudness_summary":         map[string]any{"status": "missing"},
	}
	if waveform := messageLoopBestImportDADWaveformRow(rows); len(waveform) > 0 {
		out["waveform_envelope"] = waveform
	}
	return out
}

func messageLoopFilteredImportDADFeatureSnapshot(path string, importRows []map[string]any) map[string]any {
	path = strings.TrimSpace(path)
	if path == "" || len(importRows) == 0 {
		return nil
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil || len(data) == 0 {
		return nil
	}
	snapshot := map[string]any{}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil
	}
	targets := messageLoopBuildImportDADTargets(importRows)
	if len(targets.byTrackClip) == 0 && len(targets.bySourcePath) == 0 {
		return nil
	}
	filteredRows := messageLoopFilterImportDADTrackWaveformRows(messageLoopMapRows(snapshot["track_waveform_envelopes"]), targets)
	out := messageLoopImportDADFeatureSnapshotFromRows(filteredRows, "filtered_mixboard_feature_snapshot")
	if len(out) == 0 {
		return nil
	}
	out["schema_version"] = firstNonEmpty(firstMapText(snapshot, "schema_version"), "mixboard_feature_snapshot.v1")
	for _, key := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		if value, ok := snapshot[key]; ok && !messageLoopEmptyValue(value) {
			out[key] = value
		} else if _, ok := out[key]; !ok {
			out[key] = map[string]any{"status": "missing"}
		}
	}
	return out
}

type messageLoopImportDADTargetSet struct {
	byTrackClip  map[string]map[string]any
	bySourcePath map[string]map[string]any
	order        []string
}

func messageLoopBuildImportDADTargets(rows []map[string]any) messageLoopImportDADTargetSet {
	targets := messageLoopImportDADTargetSet{
		byTrackClip:  map[string]map[string]any{},
		bySourcePath: map[string]map[string]any{},
	}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		trackID := firstMapText(row, "track_id", "id")
		clipID := firstMapText(row, "clip_id", "primary_clip_id", "item_id")
		key := messageLoopTrackClipKey(trackID, clipID)
		if key != "" {
			targets.byTrackClip[key] = row
			targets.order = append(targets.order, key)
		}
		if sourcePath := messageLoopNormalizeImportSourcePath(firstNonEmpty(firstMapText(row,
			"source_file_path", "current_source_path", "source_path", "file_path",
			"imported_file_path", "copied_file_path", "path",
		))); sourcePath != "" {
			targets.bySourcePath[sourcePath] = row
		}
	}
	return targets
}

func messageLoopFilterImportDADTrackWaveformRows(rows []map[string]any, targets messageLoopImportDADTargetSet) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	best := map[string]map[string]any{}
	for _, row := range rows {
		target, key := messageLoopImportDADTargetForRow(row, targets)
		if len(target) == 0 || key == "" {
			continue
		}
		normalized := messageLoopNormalizeImportDADTrackWaveformRow(row)
		if len(normalized) == 0 {
			continue
		}
		existing := best[key]
		if len(existing) == 0 || messageLoopImportDADRowScore(normalized) > messageLoopImportDADRowScore(existing) {
			best[key] = normalized
		}
	}
	out := make([]map[string]any, 0, len(best))
	seen := map[string]bool{}
	for _, key := range targets.order {
		if seen[key] {
			continue
		}
		if row := best[key]; len(row) > 0 {
			out = append(out, row)
			seen[key] = true
		}
	}
	for key, row := range best {
		if !seen[key] && len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func messageLoopImportDADTargetForRow(row map[string]any, targets messageLoopImportDADTargetSet) (map[string]any, string) {
	if len(row) == 0 {
		return nil, ""
	}
	rowTrackID := firstMapText(row, "track_id", "id")
	rowClipID := firstMapText(row, "clip_id", "primary_clip_id", "item_id")
	if key := messageLoopTrackClipKey(rowTrackID, rowClipID); key != "" {
		if target := targets.byTrackClip[key]; len(target) > 0 && messageLoopImportDADRowSourceCompatible(row, target) {
			return target, key
		}
	}
	if sourcePath := messageLoopNormalizeImportSourcePath(firstNonEmpty(firstMapText(row,
		"source_file_path", "current_source_path", "source_path", "file_path",
		"imported_file_path", "copied_file_path", "path",
	))); sourcePath != "" {
		if target := targets.bySourcePath[sourcePath]; len(target) > 0 {
			return target, messageLoopTrackClipKey(firstMapText(target, "track_id", "id"), firstMapText(target, "clip_id", "primary_clip_id", "item_id"))
		}
	}
	return nil, ""
}

func messageLoopImportDADRowSourceCompatible(row, target map[string]any) bool {
	rowPath := messageLoopNormalizeImportSourcePath(firstNonEmpty(firstMapText(row,
		"source_file_path", "current_source_path", "source_path", "file_path",
		"imported_file_path", "copied_file_path", "path",
	)))
	targetPath := messageLoopNormalizeImportSourcePath(firstNonEmpty(firstMapText(target,
		"source_file_path", "current_source_path", "source_path", "file_path",
		"imported_file_path", "copied_file_path", "path",
	)))
	if rowPath != "" && targetPath != "" && rowPath != targetPath && filepath.Base(rowPath) != filepath.Base(targetPath) {
		return false
	}
	for _, key := range []string{"source_revision", "source_fingerprint", "source_hash", "clip_revision"} {
		rowValue := strings.TrimSpace(firstMapText(row, key))
		targetValue := strings.TrimSpace(firstMapText(target, key))
		if rowValue != "" && targetValue != "" && rowValue != targetValue {
			return false
		}
	}
	return true
}

func messageLoopNormalizeImportDADTrackWaveformRow(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	out := cloneMap(row)
	seen, expected := messageLoopImportDADTileCounts(out)
	if expected > 0 && seen >= 0 && seen < expected {
		out["status"] = tim.StatusPartial
		out["reason"] = "waveform_tiles_incomplete_for_import_target"
	}
	return out
}

func messageLoopImportDADTileCounts(row map[string]any) (int, int) {
	metadata := messageLoopMapValue(row["metadata"])
	seen := firstPositiveMapInt(row, "tile_count_seen", "tile_count_parsed", "completed_tiles", "tiles_seen")
	if seen <= 0 {
		seen = firstPositiveMapInt(metadata, "tile_count_seen", "tile_count_parsed", "completed_tiles", "tiles_seen")
	}
	if seen <= 0 {
		if tileIndex := firstPositiveMapInt(row, "tile_index"); tileIndex > 0 {
			seen = tileIndex + 1
		} else if tileIndex := firstPositiveMapInt(metadata, "tile_index"); tileIndex > 0 {
			seen = tileIndex + 1
		}
	}
	expected := firstPositiveMapInt(row, "tile_count_expected", "tile_count", "total_tiles", "expected_tiles")
	if expected <= 0 {
		expected = firstPositiveMapInt(metadata, "tile_count_expected", "tile_count", "total_tiles", "expected_tiles")
	}
	return seen, expected
}

func messageLoopImportDADRowScore(row map[string]any) int {
	score := 0
	switch strings.ToLower(strings.TrimSpace(firstMapText(row, "status"))) {
	case tim.StatusReady:
		score += 100000
	case tim.StatusPartial:
		score += 50000
	case "requested", "building":
		score += 10000
	}
	seen, expected := messageLoopImportDADTileCounts(row)
	if expected > 0 && seen >= expected {
		score += 1000
	}
	score += seen
	if updatedAt := strings.TrimSpace(firstMapText(row, "updated_at", "created_at")); updatedAt != "" {
		score += len(updatedAt)
	}
	return score
}

func messageLoopBestImportDADWaveformRow(rows []map[string]any) map[string]any {
	var best map[string]any
	bestScore := -1
	for _, row := range rows {
		if score := messageLoopImportDADRowScore(row); len(row) > 0 && score > bestScore {
			best = row
			bestScore = score
		}
	}
	return best
}

func messageLoopTrackClipKey(trackID, clipID string) string {
	trackID = strings.TrimSpace(trackID)
	clipID = strings.TrimSpace(clipID)
	if trackID == "" && clipID == "" {
		return ""
	}
	return trackID + "\x00" + clipID
}

func messageLoopNormalizeImportSourcePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	path = strings.ReplaceAll(path, "\\", "/")
	return strings.ToLower(path)
}

func messageLoopProjectStateFromImportRows(result map[string]any, rows []map[string]any) map[string]any {
	tracks := make([]map[string]any, 0, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		trackID := firstNonEmpty(firstMapText(row, "track_id", "id"), fmt.Sprintf("imported_track_%d", i+1))
		trackName := firstNonEmpty(firstMapText(row, "track_name", "name"), firstMapText(row, "clip_name", "file_name"), trackID)
		clipID := firstNonEmpty(firstMapText(row, "clip_id", "primary_clip_id"), fmt.Sprintf("%s_clip", trackID))
		clipName := firstNonEmpty(firstMapText(row, "clip_name", "file_name", "name"), clipID)
		sourcePath := firstNonEmpty(firstMapText(row,
			"source_file_path", "current_source_path", "source_path", "file_path",
			"imported_file_path", "copied_file_path", "path",
		))
		clip := map[string]any{
			"id":        clipID,
			"clip_id":   clipID,
			"name":      clipName,
			"clip_name": clipName,
			"clip_type": "audio",
			"type":      "audio",
		}
		if sourcePath != "" {
			clip["current_source_path"] = sourcePath
			clip["source_path"] = sourcePath
			clip["file_path"] = sourcePath
		}
		for _, key := range []string{"start_time_seconds", "start_seconds", "length_seconds", "duration_seconds", "sample_rate_hz", "sample_rate", "bit_depth", "bits_per_sample", "channel_count", "channels"} {
			if value, ok := firstNumericMapValue(row, key); ok {
				clip[key] = value
			}
		}
		if valid, ok := firstMapBool(row, "playback_source_valid", "source_valid"); ok {
			clip["playback_source_valid"] = valid
		}
		tracks = append(tracks, map[string]any{
			"track_id":   trackID,
			"id":         trackID,
			"track_name": trackName,
			"name":       trackName,
			"track_type": "audio",
			"clips":      []map[string]any{clip},
		})
	}
	projectState := map[string]any{
		"tracks":      tracks,
		"track_count": len(tracks),
	}
	if duration, ok := firstNumericMapValue(messageLoopMapValue(result["summary"]), "edit_length_seconds", "duration_seconds", "timeline_length_seconds"); ok && duration > 0 {
		projectState["duration_seconds"] = duration
	}
	return projectState
}

func messageLoopTIMImportResultReadyForA2(result map[string]any, expectedTracks int) bool {
	ready, _, _ := messageLoopTIMImportResultAcousticReady(result, expectedTracks)
	return ready
}

func messageLoopTIMImportResultAcousticReady(result map[string]any, expectedTracks int) (bool, int, int) {
	return messageLoopTIMProjectionAcousticReady(messageLoopTIMProjectionForImportResult(result), expectedTracks)
}

func messageLoopTIMProjectionAcousticReady(proj map[string]any, expectedTracks int) (bool, int, int) {
	if len(proj) == 0 {
		return false, 0, maxInt(expectedTracks, 0)
	}
	summary := messageLoopMapValue(proj["technical_summary"])
	coverage := messageLoopMapValue(proj["coverage"])
	acoustic := messageLoopMapValue(coverage["acoustic_package"])
	readyCount := firstPositiveMapInt(acoustic, "known_count")
	if readyCount <= 0 {
		readyCount = firstPositiveMapInt(summary, "acoustic_ready_track_count")
	}
	totalCount := firstPositiveMapInt(acoustic, "total_count")
	if totalCount <= 0 {
		totalCount = firstPositiveMapInt(summary, "track_count")
	}
	if totalCount <= 0 {
		totalCount = expectedTracks
	}
	status := strings.TrimSpace(firstMapText(acoustic, "status"))
	ready := status == tim.StatusReady && readyCount > 0
	if totalCount > 0 {
		ready = ready && readyCount >= totalCount
	}
	if expectedTracks > 0 {
		ready = ready && readyCount >= expectedTracks
	}
	return ready, readyCount, totalCount
}

func messageLoopSetImportDADGate(result map[string]any, status string, readyCount, totalCount int) {
	if len(result) == 0 {
		return
	}
	result["dad_readiness_gate"] = map[string]any{
		"status":      status,
		"ready_count": readyCount,
		"total_count": totalCount,
		"policy":      "read_only_wait_for_automatic_dad",
	}
}

func messageLoopAnalysisJobIDFromImportResult(result map[string]any) string {
	if len(result) == 0 {
		return ""
	}
	summary := messageLoopMapValue(result["summary"])
	analysisJob := messageLoopMapValue(result["analysis_job"])
	return firstNonEmpty(
		firstMapText(result, "analysis_job_id", "job_id", "audio_analysis_job_id"),
		firstMapText(summary, "analysis_job_id", "job_id", "audio_analysis_job_id"),
		firstMapText(analysisJob, "analysis_job_id", "job_id"),
	)
}

func messageLoopStemsImportExpectedTrackCount(result map[string]any) int {
	if len(result) == 0 {
		return 0
	}
	summary := messageLoopMapValue(result["summary"])
	rows := messageLoopStemsImportRows(result)
	count := firstPositiveMapInt(summary, "tracks_created", "created_track_count", "created_tracks", "track_count", "tracks_to_create")
	if count <= 0 {
		count = firstPositiveMapInt(result, "tracks_created", "created_track_count", "created_track_count_reported")
	}
	if count <= 0 {
		count = messageLoopListCount(result["created_track_ids"])
	}
	if count <= 0 {
		count = messageLoopListCount(result["created_track_ids_preview"])
	}
	if count <= 0 {
		count = len(rows)
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func messageLoopFormatStemsImportCompactReply(data messageLoopStemsImportReportData) string {
	lines := []string{"导入完成。", "", "A1 工程接收"}
	appendMessageLoopBullet(&lines, fmt.Sprintf("轨道/片段：%d / %d", data.TracksCreated, data.ClipsCreated))
	if data.Discovered > 0 || data.Readable > 0 || data.Unreadable > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("素材清点：发现 %d，可读 %d，不可读 %d", data.Discovered, data.Readable, data.Unreadable))
	}
	if data.HasStart || data.HasLength {
		parts := []string{}
		if data.HasStart {
			parts = append(parts, fmt.Sprintf("%.2fs 起", data.StartSeconds))
		}
		if data.HasLength {
			parts = append(parts, fmt.Sprintf("%.2fs 长", data.LengthSeconds))
		}
		appendMessageLoopBullet(&lines, "时间范围："+strings.Join(parts, "，"))
	}
	if data.SourceSpec != "" {
		appendMessageLoopBullet(&lines, "素材规格："+data.SourceSpec)
	}
	if data.ProjectSpec != "" {
		appendMessageLoopBullet(&lines, "工程规格："+data.ProjectSpec)
	}
	appendMessageLoopBullet(&lines, fmt.Sprintf("差异：采样率不匹配 %d；位深/格式不匹配 %d", data.SampleRateMismatches, data.BitDepthMismatches))
	if data.ProjectSettingsChanged {
		if data.ProjectSettingsPatchSpec != "" {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步到 "+data.ProjectSettingsPatchSpec)
		} else {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步")
		}
	}
	if data.CopyPolicy != "" {
		appendMessageLoopBullet(&lines, "媒体策略："+messageLoopMediaPolicyLabel(data.CopyPolicy))
	}

	lines = append(lines, "", "A2 TIM 技术完整性")
	if len(data.TIMLines) > 0 {
		lines = append(lines, data.TIMLines...)
	} else {
		appendMessageLoopBullet(&lines, "元数据预检已完成；静音、削波和噪声需要等待音频分析返回后复查。")
	}
	if len(data.TOMLines) > 0 {
		lines = append(lines, "", "A3 TOM 智能整理建议")
		lines = append(lines, data.TOMLines...)
	}
	if len(data.A4EPMLines) > 0 {
		lines = append(lines, "", "A4 EPM Clip 裁剪/Fade 建议")
		lines = append(lines, data.A4EPMLines...)
	}
	if len(data.A5EPMLines) > 0 {
		lines = append(lines, "", "A5 EPM 段落地图推荐")
		lines = append(lines, data.A5EPMLines...)
	}

	if data.WarningCount > 0 {
		lines = append(lines, "", "提醒")
		appendMessageLoopBullet(&lines, fmt.Sprintf("工具返回 %d 条警告，可在详情里查看。", data.WarningCount))
	}
	return strings.Join(lines, "\n")
}

func appendMessageLoopBullet(lines *[]string, text string) {
	text = strings.TrimSpace(text)
	if text != "" {
		*lines = append(*lines, "- "+text)
	}
}

func messageLoopMediaPolicyLabel(policy string) string {
	switch strings.TrimSpace(policy) {
	case "reference_original":
		return "引用原文件"
	case "copy_to_project":
		return "复制到工程"
	default:
		return strings.TrimSpace(policy)
	}
}

func messageLoopAnalysisStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "queued":
		return "队列中"
	case "running":
		return "分析中"
	case "completed", "done":
		return "已完成"
	default:
		return strings.TrimSpace(status)
	}
}

func messageLoopTIMImportReportBullets(result map[string]any, tracksCreated, clipsCreated int) []string {
	proj := messageLoopTIMProjectionForImportResult(result)
	if len(proj) == 0 {
		return nil
	}
	summary := messageLoopMapValue(proj["technical_summary"])
	coverage := messageLoopMapValue(proj["coverage"])
	risk := messageLoopMapValue(proj["risk_summary"])
	trackCount := firstPositiveMapInt(summary, "track_count")
	if trackCount <= 0 {
		trackCount = tracksCreated
	}
	clipCount := firstPositiveMapInt(summary, "clip_count")
	if clipCount <= 0 {
		clipCount = clipsCreated
	}
	status := firstNonEmpty(firstMapText(proj, "status"), "partial")
	overallRisk := firstNonEmpty(firstMapText(risk, "overall_risk"), "unknown")
	lines := []string{
		fmt.Sprintf("- 状态：%s；风险：%s", messageLoopTIMStatusLabel(status), messageLoopTIMRiskLabel(overallRisk)),
		fmt.Sprintf("- 检查范围：%d 轨 / %d 片段", trackCount, clipCount),
	}
	sourceCoverage := messageLoopMapValue(coverage["source_path"])
	playbackCoverage := messageLoopMapValue(coverage["playback_validity"])
	sourceKnown := firstPositiveMapInt(sourceCoverage, "known_count")
	sourceTotal := firstPositiveMapInt(sourceCoverage, "total_count")
	playbackInvalid := firstPositiveMapInt(playbackCoverage, "invalid_count")
	playbackUnknown := firstPositiveMapInt(playbackCoverage, "missing_count")
	if sourceTotal > 0 || playbackInvalid > 0 || playbackUnknown > 0 {
		lines = append(lines, fmt.Sprintf("- 素材：路径 %d/%d；播放源无效 %d，未知 %d", sourceKnown, sourceTotal, playbackInvalid, playbackUnknown))
	}
	pcmCount := firstPositiveMapInt(summary, "pcm_source_count")
	compressedCount := firstPositiveMapInt(summary, "compressed_source_count")
	unknownFormatCount := firstPositiveMapInt(summary, "unknown_format_count")
	if pcmCount > 0 || compressedCount > 0 || unknownFormatCount > 0 {
		lines = append(lines, fmt.Sprintf("- 格式：无损/PCM %d；压缩 %d；未知 %d", pcmCount, compressedCount, unknownFormatCount))
	}
	lines = append(lines, fmt.Sprintf(
		"- 规格分布：采样率 %s；位深 %s；声道 %s",
		messageLoopTIMCountsText(messageLoopMapValue(summary["sample_rate_counts"]), "sample_rate"),
		messageLoopTIMCountsText(messageLoopMapValue(summary["bit_depth_counts"]), "bit_depth"),
		messageLoopTIMCountsText(messageLoopMapValue(summary["channel_count_counts"]), "channel_count"),
	))
	if issueLine := messageLoopTIMPrimaryBlockingIssueBullet(risk); issueLine != "" {
		lines = append(lines, issueLine)
	}
	acousticCoverage := messageLoopMapValue(coverage["acoustic_package"])
	acousticStatus := firstMapText(acousticCoverage, "status")
	if acousticStatus == "" || acousticStatus == "missing" || acousticStatus == "partial" {
		lines = append(lines, "- 待补：波形、静音、削波、噪声分析仍在后台队列中")
	}
	if messageLoopTIMProjectionSubsetLimited(proj) {
		lines = append(lines, "- 限制：本次统计来自导入工具回传行；全工程 observation 后会刷新")
	}
	return lines
}

func messageLoopTIMImportReportLines(result map[string]any, tracksCreated, clipsCreated int) []string {
	proj := messageLoopTIMProjectionForImportResult(result)
	if len(proj) == 0 {
		return nil
	}
	summary := messageLoopMapValue(proj["technical_summary"])
	coverage := messageLoopMapValue(proj["coverage"])
	risk := messageLoopMapValue(proj["risk_summary"])
	trackCount := firstPositiveMapInt(summary, "track_count")
	if trackCount <= 0 {
		trackCount = tracksCreated
	}
	clipCount := firstPositiveMapInt(summary, "clip_count")
	if clipCount <= 0 {
		clipCount = clipsCreated
	}
	status := firstNonEmpty(firstMapText(proj, "status"), "partial")
	overallRisk := firstNonEmpty(firstMapText(risk, "overall_risk"), "unknown")
	lines := []string{
		fmt.Sprintf("A2 TIM 技术完整性检查：状态 %s / 风险 %s；已检查 %d 条轨道 / %d 个音频片段。", messageLoopTIMStatusLabel(status), messageLoopTIMRiskLabel(overallRisk), trackCount, clipCount),
	}
	sourceCoverage := messageLoopMapValue(coverage["source_path"])
	playbackCoverage := messageLoopMapValue(coverage["playback_validity"])
	sourceKnown := firstPositiveMapInt(sourceCoverage, "known_count")
	sourceTotal := firstPositiveMapInt(sourceCoverage, "total_count")
	playbackInvalid := firstPositiveMapInt(playbackCoverage, "invalid_count")
	playbackUnknown := firstPositiveMapInt(playbackCoverage, "missing_count")
	if sourceTotal > 0 || playbackInvalid > 0 || playbackUnknown > 0 {
		lines = append(lines, fmt.Sprintf("素材可读性：source path %d/%d；播放源无效 %d，未知 %d。", sourceKnown, sourceTotal, playbackInvalid, playbackUnknown))
	}
	pcmCount := firstPositiveMapInt(summary, "pcm_source_count")
	compressedCount := firstPositiveMapInt(summary, "compressed_source_count")
	unknownFormatCount := firstPositiveMapInt(summary, "unknown_format_count")
	specParts := []string{}
	if pcmCount > 0 || compressedCount > 0 || unknownFormatCount > 0 {
		specParts = append(specParts, fmt.Sprintf("PCM/lossless %d，压缩格式 %d，未知格式 %d", pcmCount, compressedCount, unknownFormatCount))
	}
	specParts = append(specParts,
		"采样率 "+messageLoopTIMCountsText(messageLoopMapValue(summary["sample_rate_counts"]), "sample_rate"),
		"bit depth "+messageLoopTIMCountsText(messageLoopMapValue(summary["bit_depth_counts"]), "bit_depth"),
		"声道 "+messageLoopTIMCountsText(messageLoopMapValue(summary["channel_count_counts"]), "channel_count"),
	)
	lines = append(lines, "格式/规格："+strings.Join(specParts, "；")+"。")
	if issueLine := messageLoopTIMPrimaryIssueLine(risk); issueLine != "" {
		lines = append(lines, issueLine)
	}
	acousticCoverage := messageLoopMapValue(coverage["acoustic_package"])
	acousticStatus := firstMapText(acousticCoverage, "status")
	if acousticStatus == "" || acousticStatus == "missing" || acousticStatus == "partial" {
		lines = append(lines, "待补观察：DAD 波形/静音/削波/noise 分析尚未全部返回；metadata precheck 已完成，audio analysis 返回后 TIM 会更新这些判断。")
	}
	if messageLoopTIMProjectionSubsetLimited(proj) {
		lines = append(lines, "限制：本次 TIM 基于导入工具回传的轨道行生成，若工具只返回 preview，完整性统计会在下一次全工程 observation 后刷新。")
	}
	return lines
}

func messageLoopTIMProjectionForImportResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if proj := messageLoopMapValue(result["tim_projection"]); len(proj) > 0 {
		return proj
	}
	rows := messageLoopStemsImportRows(result)
	if len(rows) == 0 {
		return nil
	}
	projection := tim.BuildFromImportRows(tim.ImportInput{
		Summary:       messageLoopMapValue(result["summary"]),
		Rows:          rows,
		AudioSettings: messageLoopMapValue(result["audio_settings_snapshot"]),
	})
	proj := tim.ContextProjectionMap(projection)
	result["tim_projection"] = proj
	return proj
}

func messageLoopTOMProjectionForImportResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if proj := messageLoopMapValue(result["tom_projection"]); len(proj) > 0 {
		return proj
	}
	rows := messageLoopStemsImportRows(result)
	if len(rows) == 0 {
		return nil
	}
	projection := tom.BuildFromImportRows(tom.ImportInput{
		Summary:         messageLoopMapValue(result["summary"]),
		Rows:            rows,
		TIMProjection:   messageLoopTIMProjectionForImportResult(result),
		DADWaveformRows: messageLoopImportDADStatusWaveformRows(result),
	})
	proj := tom.ContextProjectionMap(projection)
	result["tom_projection"] = proj
	return proj
}

func messageLoopEPMProjectionForImportResult(result map[string]any) map[string]any {
	if len(result) == 0 {
		return nil
	}
	if proj := messageLoopMapValue(result["epm_projection"]); len(proj) > 0 {
		return proj
	}
	rows := messageLoopStemsImportRows(result)
	if len(rows) == 0 {
		return nil
	}
	projection := epm.BuildFromImportRows(epm.ImportInput{
		Summary:         messageLoopMapValue(result["summary"]),
		Rows:            rows,
		TIMProjection:   messageLoopTIMProjectionForImportResult(result),
		TOMProjection:   messageLoopTOMProjectionForImportResult(result),
		DADWaveformRows: messageLoopImportDADStatusWaveformRows(result),
	})
	proj := epm.ContextProjectionMap(projection)
	result["epm_projection"] = proj
	return proj
}

func messageLoopEPMClipEditReportBullets(result map[string]any) []string {
	proj := messageLoopEPMProjectionForImportResult(result)
	if len(proj) == 0 {
		return nil
	}
	cleanup := messageLoopMapValue(proj["clip_cleanup"])
	if len(cleanup) == 0 {
		return nil
	}
	trackCount := firstPositiveMapInt(cleanup, "track_count")
	clipCount := firstPositiveMapInt(cleanup, "clip_count")
	lines := []string{
		fmt.Sprintf("- 检查范围：%d 轨 / %d 片段", trackCount, clipCount),
	}
	preserve, _ := firstMapBool(cleanup, "preserve_stem_alignment")
	fullLength, _ := firstMapBool(cleanup, "full_length_stem_detected")
	confidence := messageLoopEPMConfidenceLabel(firstMapText(cleanup, "stem_alignment_confidence"))
	if fullLength || preserve {
		lines = append(lines, fmt.Sprintf("- 对齐保护：检测到整首 Stem 对齐；置信度：%s；默认不自动裁剪", confidence))
	} else {
		lines = append(lines, fmt.Sprintf("- 对齐保护：未确认整首 Stem 对齐；置信度：%s", confidence))
	}
	lines = append(lines, fmt.Sprintf(
		"- 裁剪：%s；Fade：%s；Crossfade：%s",
		messageLoopEPMRecommendationLabel(firstMapText(cleanup, "trim_recommendation")),
		messageLoopEPMRecommendationLabel(firstMapText(cleanup, "fade_recommendation")),
		messageLoopEPMRecommendationLabel(firstMapText(cleanup, "crossfade_recommendation")),
	))
	candidates := messageLoopMapValue(cleanup["candidate_summary"])
	trimCount := firstPositiveMapInt(candidates, "trim_candidate_count")
	fadeCount := firstPositiveMapInt(candidates, "fade_candidate_count")
	shortCount := firstPositiveMapInt(candidates, "short_clip_review_count")
	silenceCount := firstPositiveMapInt(candidates, "silence_review_count")
	hotCount := firstPositiveMapInt(candidates, "hot_peak_review_count")
	if trimCount > 0 || fadeCount > 0 || shortCount > 0 || silenceCount > 0 || hotCount > 0 {
		lines = append(lines, fmt.Sprintf("- 复核候选：裁剪 %d；Fade %d；短片段 %d；静音 %d；过热 %d", trimCount, fadeCount, shortCount, silenceCount, hotCount))
	} else {
		lines = append(lines, "- 复核候选：暂无明显裁剪/Fade 风险")
	}
	lines = append(lines, "- 当前未修改任何片段；需要用户确认后才执行裁剪或 Fade")
	return lines
}

func messageLoopEPMSectionMapReportBullets(result map[string]any) []string {
	proj := messageLoopEPMProjectionForImportResult(result)
	if len(proj) == 0 {
		return nil
	}
	section := messageLoopMapValue(proj["section_map"])
	if len(section) > 0 {
		status := messageLoopEPMSectionStatusLabel(firstMapText(section, "status"))
		strategy := messageLoopEPMSectionStrategyLabel(firstMapText(section, "reference_strategy"))
		confidence := messageLoopEPMConfidenceLabel(firstMapText(section, "confidence"))
		marker := messageLoopEPMMarkerSupportLabel(firstMapText(section, "marker_write_support"))
		count := firstPositiveMapInt(section, "candidate_count")
		duration := messageLoopFloatFromAny(section["duration_seconds"])
		lines := []string{
			fmt.Sprintf("- 状态：%s；置信度：%s；参考策略：%s", status, confidence, strategy),
		}
		coverage := messageLoopFloatFromAny(section["coverage_seconds"])
		if coverage <= 0 {
			coverage = duration
		}
		if duration > 0 || count > 0 {
			lines = append(lines, fmt.Sprintf("- 推荐段落：%d 段；覆盖 %.2fs", count, coverage))
		}
		if text := messageLoopEPMSectionListText(messageLoopMapRows(section["sections"]), 6); text != "" {
			lines = append(lines, "- 段落草案："+text)
		}
		if basis := messageLoopEPMSectionEvidenceText(messageLoopMapValue(section["evidence_summary"])); basis != "" {
			lines = append(lines, "- 依据："+basis)
		}
		if strings.EqualFold(firstMapText(section, "marker_write_support"), "ready") {
			lines = append(lines, "- Marker 写入："+marker+"；确认后会写入为段落 marker，当前未修改工程")
		} else {
			lines = append(lines, "- Marker 写入："+marker+"；当前只生成建议，不修改工程")
		}
		return lines
	}
	return nil
}

func messageLoopTOMOrganizationReportBullets(result map[string]any) []string {
	proj := messageLoopTOMProjectionForImportResult(result)
	if len(proj) == 0 {
		return nil
	}
	summary := messageLoopMapValue(proj["organization_summary"])
	disclosure := messageLoopMapValue(proj["disclosure_plan"])
	manifest := messageLoopMapValue(proj["full_assignment_manifest"])
	groups := messageLoopMapRows(proj["group_proposals"])
	if len(groups) == 0 {
		return nil
	}
	trackCount := firstPositiveMapInt(summary, "track_count")
	groupText := messageLoopTOMGroupListText(groups, 8)
	lines := []string{}
	if coverage := firstPositiveMapInt(manifest, "assignment_coverage_count"); coverage > 0 && trackCount > 0 {
		lines = append(lines, fmt.Sprintf("- 整理清单：覆盖 %d/%d；状态：%s", coverage, trackCount, messageLoopTOMCoverageStatusText(firstMapText(manifest, "coverage_status"))))
	}
	if stage := messageLoopTOMDisclosureStageText(firstMapText(disclosure, "selected_stage")); stage != "" {
		lines = append(lines, "- 当前依据："+stage)
	}
	if groupText != "" {
		lines = append(lines, "- 建议分组："+groupText)
	}
	highText := messageLoopTOMGroupsByConfidenceText(groups, "high", 5)
	if highText != "" {
		lines = append(lines, "- 高置信："+highText)
	}
	reviewCount := firstPositiveMapInt(summary, "needs_review_track_count")
	reviewText := messageLoopTOMReviewGroupsText(groups, 5)
	if reviewCount > 0 && reviewText != "" {
		lines = append(lines, fmt.Sprintf("- 需要确认：%d 轨低置信或仅技术聚类；重点复核 %s", reviewCount, reviewText))
	} else if reviewCount > 0 {
		lines = append(lines, fmt.Sprintf("- 需要确认：%d 轨低置信或仅技术聚类", reviewCount))
	}
	namingMatched := firstPositiveMapInt(summary, "naming_matched_track_count")
	technicalFallback := firstPositiveMapInt(summary, "technical_fallback_track_count")
	if trackCount > 0 {
		lines = append(lines, fmt.Sprintf("- 依据：命名/ID 命中 %d/%d；技术补充分组 %d", namingMatched, trackCount, technicalFallback))
	}
	lines = append(lines, "- 下一步：确认后再整理为文件夹轨道；当前未修改工程")
	return lines
}

func messageLoopTOMCoverageStatusText(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "complete":
		return "完整"
	case "partial":
		return "部分"
	case "":
		return "未知"
	default:
		return status
	}
}

func messageLoopTOMDisclosureStageText(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "naming_id":
		return "命名/ID 阶段"
	case "technical_cluster":
		return "技术聚类阶段"
	case "dad_lightweight":
		return "命名/ID + 技术聚类 + DAD 轻量观察"
	default:
		return strings.TrimSpace(stage)
	}
}

func messageLoopTOMGroupListText(groups []map[string]any, limit int) string {
	parts := []string{}
	for _, group := range groups {
		label := messageLoopTOMGroupDisplayLabel(group)
		count := firstPositiveMapInt(group, "track_count")
		if label == "" || count <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", label, count))
		if limit > 0 && len(parts) >= limit {
			break
		}
	}
	if len(groups) > len(parts) && limit > 0 {
		parts = append(parts, fmt.Sprintf("另 %d 组", len(groups)-len(parts)))
	}
	return strings.Join(parts, " / ")
}

func messageLoopTOMGroupsByConfidenceText(groups []map[string]any, confidence string, limit int) string {
	parts := []string{}
	for _, group := range groups {
		if firstMapText(group, "confidence") != confidence {
			continue
		}
		label := messageLoopTOMGroupDisplayLabel(group)
		count := firstPositiveMapInt(group, "track_count")
		if label == "" || count <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", label, count))
		if limit > 0 && len(parts) >= limit {
			break
		}
	}
	return strings.Join(parts, " / ")
}

func messageLoopTOMReviewGroupsText(groups []map[string]any, limit int) string {
	parts := []string{}
	for _, group := range groups {
		confidence := firstMapText(group, "confidence")
		needsConfirmation, _ := firstMapBool(group, "needs_confirmation")
		if confidence == "high" && !needsConfirmation {
			continue
		}
		label := messageLoopTOMGroupDisplayLabel(group)
		count := firstPositiveMapInt(group, "track_count")
		if label == "" || count <= 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", label, count))
		if limit > 0 && len(parts) >= limit {
			break
		}
	}
	return strings.Join(parts, " / ")
}

func messageLoopTOMGroupDisplayLabel(group map[string]any) string {
	groupID := firstMapText(group, "group_id")
	switch groupID {
	case "vocals":
		return "主人声"
	case "backing_vocals":
		return "和声/背景人声"
	case "drums":
		return "鼓组"
	case "bass":
		return "贝斯/低频"
	case "strings":
		return "弦乐"
	case "synths":
		return "合成器"
	case "guitars":
		return "吉他"
	case "keys":
		return "键盘/钢琴"
	case "fx":
		return "FX/转场"
	case "returns":
		return "效果返回"
	case "buses_prints":
		return "总线/打印轨"
	case "silent_candidates":
		return "疑似静音/空轨"
	case "hot_clipping_review":
		return "过热/削波复核"
	case "mono_sources":
		return "单声道素材"
	case "short_clips":
		return "短片段/FX候选"
	case "long_stereo_stems":
		return "长立体声Stem"
	case "needs_review":
		return "待人工确认"
	default:
		return firstMapText(group, "label", "proposed_folder", "group_id")
	}
}

func messageLoopEPMConfidenceLabel(confidence string) string {
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low_medium":
		return "中低"
	case "low":
		return "低"
	default:
		return "未知"
	}
}

func messageLoopEPMRecommendationLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "no_cleanup_needed":
		return "无需清理"
	case "review_candidates":
		return "复核候选"
	case "limited_missing_facts":
		return "信息不足"
	case "not_recommended_for_stems":
		return "整首 Stem 默认不裁剪"
	case "candidate_review_required":
		return "需人工确认候选"
	case "not_enough_data":
		return "数据不足"
	case "not_recommended_by_default":
		return "默认不添加"
	case "review_boundary_risk_only":
		return "仅复核边界风险"
	case "not_recommended_for_single_full_length_stems":
		return "整首 Stem 默认不交叉淡化"
	default:
		if strings.TrimSpace(value) == "" {
			return "未知"
		}
		return strings.TrimSpace(value)
	}
}

func messageLoopEPMSectionStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "framework_ready":
		return "前置框架已准备"
	case "recommended":
		return "已生成推荐"
	case "suggested_low_confidence":
		return "低置信推荐"
	case "limited":
		return "信息不足"
	case "not_computed":
		return "未生成"
	default:
		if strings.TrimSpace(status) == "" {
			return "未知"
		}
		return strings.TrimSpace(status)
	}
}

func messageLoopEPMSectionStrategyLabel(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "single_track_proxy":
		return "单轨参考"
	case "few_track_proxy_mix":
		return "少轨代理混合"
	case "multitrack_group_activity":
		return "多轨组活动图"
	default:
		if strings.TrimSpace(strategy) == "" {
			return "未知"
		}
		return strings.TrimSpace(strategy)
	}
}

func messageLoopEPMSectionListText(sections []map[string]any, limit int) string {
	parts := []string{}
	for _, section := range sections {
		label := firstMapText(section, "label", "label_hint", "section_id")
		start := messageLoopFloatFromAny(section["start_seconds"])
		end := messageLoopFloatFromAny(section["end_seconds"])
		confidence := messageLoopEPMConfidenceLabel(firstMapText(section, "confidence"))
		if label == "" || end <= start {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %.2f-%.2fs（%s）", label, start, end, confidence))
		if limit > 0 && len(parts) >= limit {
			break
		}
	}
	if len(sections) > len(parts) && limit > 0 {
		parts = append(parts, fmt.Sprintf("另 %d 段", len(sections)-len(parts)))
	}
	return strings.Join(parts, " / ")
}

func messageLoopEPMSectionEvidenceText(evidence map[string]any) string {
	if len(evidence) == 0 {
		return ""
	}
	source := firstMapText(evidence, "boundary_source")
	timeSegmentTracks := firstPositiveMapInt(evidence, "time_segment_track_count")
	timeSegments := firstPositiveMapInt(evidence, "time_segment_count")
	boundaries := firstPositiveMapInt(evidence, "boundary_candidate_count")
	parts := []string{}
	switch source {
	case "dad_time_segment_activity":
		parts = append(parts, "DAD 时间能量/活动变化")
	case "duration_template":
		parts = append(parts, "工程时长保守模板")
	case "":
	default:
		parts = append(parts, source)
	}
	if timeSegmentTracks > 0 || timeSegments > 0 {
		parts = append(parts, fmt.Sprintf("time segments %d 轨/%d 段", timeSegmentTracks, timeSegments))
	}
	if boundaries > 0 {
		parts = append(parts, fmt.Sprintf("边界候选 %d", boundaries))
	}
	return strings.Join(parts, "；")
}

func messageLoopEPMMarkerSupportLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "not_connected":
		return "尚未接入"
	case "ready":
		return "可写入"
	default:
		if strings.TrimSpace(status) == "" {
			return "未知"
		}
		return strings.TrimSpace(status)
	}
}

func messageLoopFloatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case int32:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint64:
		return float64(typed)
	case json.Number:
		parsed, _ := strconv.ParseFloat(typed.String(), 64)
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func messageLoopTIMCountsText(counts map[string]any, kind string) string {
	if len(counts) == 0 {
		return "未知"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		label := messageLoopTIMCountLabel(key, kind)
		count := firstPositiveMapInt(counts, key)
		if count > 0 {
			parts = append(parts, fmt.Sprintf("%s x%d", label, count))
		} else {
			parts = append(parts, label)
		}
		if len(parts) >= 4 {
			break
		}
	}
	if len(parts) == 0 {
		return "未知"
	}
	if len(keys) > len(parts) {
		parts = append(parts, fmt.Sprintf("另 %d 类", len(keys)-len(parts)))
	}
	return strings.Join(parts, " / ")
}

func messageLoopTIMCountLabel(key, kind string) string {
	key = strings.TrimSpace(key)
	switch kind {
	case "sample_rate":
		return strings.TrimSuffix(key, "_hz") + " Hz"
	case "bit_depth":
		return strings.TrimSuffix(key, "_bit") + "-bit"
	case "channel_count":
		return strings.TrimSuffix(key, "_ch") + "ch"
	default:
		return key
	}
}

func messageLoopTIMPrimaryIssueLine(risk map[string]any) string {
	codes := messageLoopStringSliceFromAny(risk["primary_codes"])
	if len(codes) == 0 {
		return ""
	}
	labels := []string{}
	for _, code := range codes {
		if label := messageLoopTIMIssueLabel(code); label != "" {
			labels = append(labels, label)
		}
		if len(labels) >= 5 {
			break
		}
	}
	if len(labels) == 0 {
		return ""
	}
	return "主要风险：" + strings.Join(labels, " / ") + "。"
}

func messageLoopTIMPrimaryBlockingIssueBullet(risk map[string]any) string {
	codes := messageLoopStringSliceFromAny(risk["primary_codes"])
	if len(codes) == 0 {
		return ""
	}
	labels := []string{}
	for _, code := range codes {
		switch strings.TrimSpace(code) {
		case "acoustic_package_missing_or_partial", "compressed_source_format":
			continue
		}
		if label := messageLoopTIMIssueLabel(code); label != "" {
			labels = append(labels, label)
		}
		if len(labels) >= 4 {
			break
		}
	}
	if len(labels) == 0 {
		return ""
	}
	return "- 风险：" + strings.Join(labels, " / ")
}

func messageLoopTIMIssueLabel(code string) string {
	switch strings.TrimSpace(code) {
	case "source_path_missing":
		return "素材路径缺失"
	case "playback_source_invalid":
		return "播放源无效"
	case "empty_track":
		return "空轨"
	case "abnormally_short_clip":
		return "异常短片段"
	case "compressed_source_format":
		return "压缩格式素材"
	case "acoustic_package_missing_or_partial":
		return "DAD 波形分析待完成"
	case "possible_clipping_or_no_headroom":
		return "可能削波/余量不足"
	case "possible_silence":
		return "可能静音"
	default:
		return strings.TrimSpace(code)
	}
}

func messageLoopTIMStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "ready":
		return "通过"
	case "suspect":
		return "需复查"
	case "missing":
		return "缺失"
	default:
		return "部分完成"
	}
}

func messageLoopTIMRiskLabel(risk string) string {
	switch strings.TrimSpace(risk) {
	case "none":
		return "无"
	case "low":
		return "低"
	case "high":
		return "高"
	default:
		return "中"
	}
}

func messageLoopTIMProjectionSubsetLimited(proj map[string]any) bool {
	for _, value := range messageLoopAnySlice(proj["limitations"]) {
		if strings.TrimSpace(fmt.Sprint(value)) == "tim_import_projection_built_from_returned_track_rows_subset" {
			return true
		}
	}
	return false
}

func messageLoopStringSliceFromAny(value any) []string {
	items := messageLoopAnySlice(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

func messageLoopStemsImportRows(result map[string]any) []map[string]any {
	if len(result) == 0 {
		return nil
	}
	if rows := mapRows(result["imported_track_refs"]); len(rows) > 0 {
		return rows
	}
	if rows := mapRows(result["imported_tracks"]); len(rows) > 0 {
		return rows
	}
	return mapRows(result["imported_tracks_preview"])
}

func messageLoopFirstStemsImportRow(rows []map[string]any) map[string]any {
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func messageLoopCountRowsOrList(value any) int {
	if rows := mapRows(value); len(rows) > 0 {
		return len(rows)
	}
	return messageLoopListCount(value)
}

func messageLoopAudioSpecText(sampleRate int, bitDepth int, pcmFormat string) string {
	parts := []string{}
	if sampleRate > 0 {
		parts = append(parts, fmt.Sprintf("%d Hz", sampleRate))
	}
	if bitDepth > 0 {
		parts = append(parts, fmt.Sprintf("%d-bit", bitDepth))
	}
	if pcmFormat = strings.TrimSpace(pcmFormat); pcmFormat != "" {
		parts = append(parts, pcmFormat)
	}
	return strings.Join(parts, " / ")
}

func messageLoopClipExecutionFallbackReply(state *runState, record map[string]any, ver planner.VerificationResult) string {
	result, _ := record["result"].(map[string]any)
	action := strings.ToLower(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])))
	isImport := strings.Contains(action, "import") || strings.Contains(action, "media") || strings.Contains(action, "audio")
	if !isImport {
		return messageLoopClipPresentFastCompleteReply(state, record, ver)
	}
	clipName := firstNonEmpty(
		firstMapText(result, "clip_name", "name", "file_name", "source_name"),
		firstMapText(ver.Evidence, "observed_clip_name", "expected_clip_name"),
		state.executionMemory.LastCreatedClipName,
	)
	trackName := firstNonEmpty(firstMapText(result, "track_name"), firstMapText(ver.Evidence, "observed_track_name", "expected_track_name"))
	if clipName != "" && trackName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u5c06\u97f3\u9891\u7247\u6bb5\u300c%s\u300d\u5bfc\u5165\u5230\u8f68\u9053\u300c%s\u300d\u3002", clipName, trackName)
	}
	if clipName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u5bfc\u5165\u97f3\u9891\u7247\u6bb5\u300c%s\u300d\u3002", clipName)
	}
	if trackName != "" {
		return fmt.Sprintf("\u5df2\u6210\u529f\u628a\u97f3\u9891\u5bfc\u5165\u5230\u8f68\u9053\u300c%s\u300d\u3002", trackName)
	}
	return "\u5df2\u6210\u529f\u628a\u97f3\u9891\u5bfc\u5165\u5230\u5de5\u7a0b\u8f68\u9053\u3002"
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
		"500", "502", "503", "504", "internal server error", "temporary", "stream returned no output", "connection reset",
		"wsarecv", "failed to respond", "connection attempt failed", "network is unreachable",
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
	case "mix.derive", "mix_derive", "mix.report", "mix_report", "project.state", "get_project_state", "track.list", "goal.status":
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
	if messageLoopOrdinarySemanticEQRequest(state) {
		// The typed semantic action is materialized by Chat after AgentLoop.
		// Never synthesize the legacy profile-oriented MixTreatmentPending here.
		return messageLoopStripExecutionQuestion(messageLoopStripMixTreatmentPendingMarkup(reply))
	}
	if messageLoopSemanticEQNeedsPluginSelection(state) {
		// Plugin recommendation/loading is a separate governed stage. Do not let
		// the legacy profile-oriented treatment path create mutation authority.
		state.executionMemory.PendingMixTreatment = nil
		return messageLoopStripExecutionQuestion(messageLoopStripMixTreatmentPendingMarkup(reply))
	}
	isLowMudPluginPrep := messageLoopLowMudPluginPrepForState(state)
	isObservationFollowup := messageLoopMixObservationActionFollowupRequest(state)
	hasTreatmentMarkup := messageLoopMixTreatmentPendingMarkupPresent(reply)
	if !messageLoopNaturalMixRequest(state.input.UserText) && !messageLoopImplicitPanFollowupRequest(state) && !isLowMudPluginPrep && !isObservationFollowup && !hasTreatmentMarkup {
		return reply
	}
	if messageLoopExplicitPluginRequest(state) && !isLowMudPluginPrep {
		return messageLoopStripMixTreatmentPendingMarkup(reply)
	}
	deepIncomplete := messageLoopLastMixObservationDeepPackageIncomplete(state) || messageLoopReplyMentionsIncompleteDeepPackage(reply)
	hasResolvedActionIntent := messageLoopResolvedMixActionIntent(state)
	if deepIncomplete && !isLowMudPluginPrep && !hasResolvedActionIntent {
		return messageLoopStripExecutionQuestion(messageLoopStripMixTreatmentPendingMarkup(reply))
	}
	replyAlreadyRendered := false
	preferTreatmentPending := hasTreatmentMarkup || isLowMudPluginPrep || messageLoopExplicitPanActionText(state.input.UserText)
	if preferTreatmentPending {
		if treatment := messageLoopMixTreatmentPendingFromReply(state, reply); treatment != nil {
			state.executionMemory.PendingMixTreatment = treatment
			reply = messageLoopPendingMixTreatmentReply(treatment)
			replyAlreadyRendered = true
		} else if treatment := messageLoopConservativeLowMudTreatmentPendingFromReply(state, reply); treatment != nil {
			state.executionMemory.PendingMixTreatment = treatment
			reply = messageLoopConservativeLowMudTreatmentPendingReply(treatment)
			replyAlreadyRendered = true
		} else if hasTreatmentMarkup {
			reply = messageLoopStripMixTreatmentPendingMarkup(reply)
		}
	} else if candidate := messageLoopPendingMixTickCandidateFromReply(state, reply); candidate != nil {
		messageLoopAttachDiagnosisToMixTick(state, candidate, state.input.UserText)
		state.executionMemory.PendingMixTickCandidate = candidate
	} else if treatment := messageLoopMixTreatmentPendingFromReply(state, reply); treatment != nil {
		state.executionMemory.PendingMixTreatment = treatment
		reply = messageLoopPendingMixTreatmentReply(treatment)
		replyAlreadyRendered = true
	} else if treatment := messageLoopConservativeLowMudTreatmentPendingFromReply(state, reply); treatment != nil {
		state.executionMemory.PendingMixTreatment = treatment
		reply = messageLoopConservativeLowMudTreatmentPendingReply(treatment)
		replyAlreadyRendered = true
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
	if replyAlreadyRendered {
		return messageLoopStripExecutionQuestion(reply)
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
	if messageLoopClipFadeGainRequest(text) {
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
	if messageLoopLowMudPluginPrepForState(state) ||
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
		if !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if len(result) == 0 {
			result = record
		}
		if messageLoopExecutionSucceeded(record) || (messageLoopText(record["error"]) == "" && messageLoopMixObservationResultUsable(result)) {
			return result
		}
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
	if issue := messageLoopCompressorFinalIssue(state); issue != "" {
		return issue
	}
	if state != nil && (messageLoopNaturalMixRequest(state.input.UserText) || messageLoopAudioObservationRequest(state.input.UserText)) && !messageLoopObservationPackageReadRequest(state.input.UserText) && (!messageLoopExplicitPluginRequest(state) || messageLoopLowMudPluginPrepForState(state) || messageLoopMutationBarrierActive(state)) && !messageLoopHasAnyMixObservationAttempt(state) && !(messageLoopFreeStateActive(state) && messageLoopHasSuccessfulCCBObservationRequest(state)) {
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

func messageLoopCompressorFinalIssue(state *runState) string {
	if state == nil {
		return ""
	}
	sawInspect := false
	var latestApply map[string]any
	for _, record := range state.executed {
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if (name == "plugin_grabber.inspect_compressor" || name == "plugin_grabber_inspect_compressor") && messageLoopExecutionSucceeded(record) {
			sawInspect = true
		}
		if messageLoopIsCompressorApplyName(name) {
			latestApply = record
		}
	}
	if len(latestApply) > 0 {
		if messageLoopCompressorApplyRecordComplete(latestApply) {
			return ""
		}
		return "cannot claim compressor success: the latest typed apply_compressor_controls call failed or was rejected; report the failure and that no fallback write was allowed"
	}
	if sawInspect {
		return "cannot claim compressor success: inspect_compressor succeeded but no typed apply_compressor_controls result with complete actual_readback exists"
	}
	return ""
}

func messageLoopCompressorApplyRecordComplete(record map[string]any) bool {
	if !messageLoopExecutionSucceeded(record) {
		return false
	}
	result := messageLoopMapValue(record["result"])
	status := strings.ToLower(strings.TrimSpace(messageLoopText(result["status"])))
	if status != "exact" && status != "quantized" {
		return false
	}
	controls := messageLoopMapRows(result["controls"])
	if len(controls) == 0 {
		return false
	}
	for _, control := range controls {
		if len(messageLoopMapRows(control["actual_readback"])) == 0 {
			return false
		}
	}
	return true
}

func messageLoopCompressorFailureFinalReply(state *runState) (string, bool) {
	if state == nil {
		return "", false
	}
	var latest map[string]any
	for _, record := range state.executed {
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if messageLoopIsCompressorApplyName(name) {
			latest = record
		}
	}
	if len(latest) == 0 || messageLoopCompressorApplyRecordComplete(latest) {
		return "", false
	}
	errorText := strings.TrimSpace(messageLoopText(latest["error"]))
	if errorText == "" {
		result := messageLoopMapValue(latest["result"])
		errorText = firstNonEmpty(
			strings.TrimSpace(messageLoopText(result["message"])),
			strings.TrimSpace(messageLoopText(result["rejection_code"])),
			"typed apply_compressor_controls 未返回完整读回",
		)
	}
	return fmt.Sprintf("未完成：压缩器 typed 参数控制失败（%s）。已停止执行，没有使用通用参数写入回退；本次失败未被当作成功。", errorText), true
}

func messageLoopLatestCompressorApplyFailed(state *runState) bool {
	if state == nil {
		return false
	}
	found := false
	failed := false
	for _, record := range state.executed {
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if !messageLoopIsCompressorApplyName(name) {
			continue
		}
		found = true
		failed = !messageLoopCompressorApplyRecordComplete(record)
	}
	return found && failed
}

func messageLoopIsCompressorApplyName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plugin_grabber.apply_compressor_controls", "plugin_grabber_apply_compressor_controls":
		return true
	default:
		return false
	}
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
