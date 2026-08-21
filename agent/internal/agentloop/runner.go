package agentloop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/contextruntime"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
	"vit-daw-agent/internal/tools"
)

const (
	StopReasonDone                 = "done"
	StopReasonNeedsConfirmation    = "needs_confirmation"
	StopReasonNeedsClarification   = "needs_clarification"
	StopReasonLimitReached         = "limit_reached"
	StopReasonTransientLLMError    = "transient_llm_error"
	StopReasonModelProtocolFailure = "model_protocol_failure"
	StopReasonCancelled            = "cancelled"
	StopReasonInterjection         = "interjection"
	StopReasonFailed               = "failed"

	LimitTypeTurns     = "max_turns"
	LimitTypeToolCalls = "max_tool_calls"
	LimitTypeTimeout   = "timeout"
)

type Budget struct {
	MaxTurns             int           `json:"max_turns,omitempty"`
	MaxToolCalls         int           `json:"max_tool_calls,omitempty"`
	Timeout              time.Duration `json:"timeout,omitempty"`
	MaxConsecutiveErrors int           `json:"max_consecutive_errors,omitempty"`
}

type Input struct {
	GoalID            string               `json:"goal_id,omitempty"`
	RunID             string               `json:"run_id,omitempty"`
	TaskID            string               `json:"task_id,omitempty"`
	SliceID           string               `json:"slice_id,omitempty"`
	TurnID            string               `json:"turn_id,omitempty"`
	ResumedFromID     string               `json:"resumed_from_continuation_id,omitempty"`
	OriginalIntent    string               `json:"original_intent,omitempty"`
	UserText          string               `json:"user_text"`
	Summary           string               `json:"summary,omitempty"`
	Context           map[string]any       `json:"context,omitempty"`
	ContextSnapshot   map[string]any       `json:"context_snapshot,omitempty"`
	ProjectHistory    map[string]any       `json:"project_history,omitempty"`
	Conversation      []llm.Message        `json:"conversation,omitempty"`
	State             map[string]any       `json:"state,omitempty"`
	CatalogSummary    string               `json:"catalog_summary,omitempty"`
	AllowedTools      []string             `json:"allowed_tools,omitempty"`
	Trace             []planner.TraceEvent `json:"trace,omitempty"`
	PlanItems         []planner.PlanItem   `json:"plan_items,omitempty"`
	Budget            Budget               `json:"budget,omitempty"`
	ExecutionMemory   ExecutionMemory      `json:"execution_memory,omitempty"`
	RecentObservation *RecentObservation   `json:"recent_observation,omitempty"`
	FreeStateDecision *FreeStateDecision   `json:"free_state_decision,omitempty"`
}

type Continuation struct {
	ContinuationID    string               `json:"continuation_id,omitempty"`
	GoalID            string               `json:"goal_id"`
	RunID             string               `json:"run_id,omitempty"`
	TaskID            string               `json:"task_id,omitempty"`
	SliceID           string               `json:"slice_id,omitempty"`
	TurnID            string               `json:"turn_id,omitempty"`
	OriginalIntent    string               `json:"original_intent,omitempty"`
	UserText          string               `json:"user_text"`
	Summary           string               `json:"summary,omitempty"`
	Context           map[string]any       `json:"context,omitempty"`
	ContextSnapshot   map[string]any       `json:"context_snapshot,omitempty"`
	ProjectHistory    map[string]any       `json:"project_history,omitempty"`
	Conversation      []llm.Message        `json:"conversation,omitempty"`
	State             map[string]any       `json:"state,omitempty"`
	CatalogSummary    string               `json:"catalog_summary,omitempty"`
	AllowedTools      []string             `json:"allowed_tools,omitempty"`
	Trace             []planner.TraceEvent `json:"trace,omitempty"`
	PlanItems         []planner.PlanItem   `json:"plan_items,omitempty"`
	Budget            Budget               `json:"budget,omitempty"`
	PendingToolCall   *planner.ToolCall    `json:"pending_tool_call,omitempty"`
	PendingToolQueue  []planner.ToolCall   `json:"pending_tool_queue,omitempty"`
	CompletedSteps    int                  `json:"completed_steps,omitempty"`
	TurnsUsed         int                  `json:"turns_used,omitempty"`
	ToolCallsUsed     int                  `json:"tool_calls_used,omitempty"`
	ExecutionMemory   ExecutionMemory      `json:"execution_memory,omitempty"`
	RecentObservation *RecentObservation   `json:"recent_observation,omitempty"`
	FreeStateDecision *FreeStateDecision   `json:"free_state_decision,omitempty"`
}

type Result struct {
	Goal                  agentruntime.Goal           `json:"goal"`
	GoalID                string                      `json:"goal_id,omitempty"`
	RunID                 string                      `json:"run_id,omitempty"`
	TaskID                string                      `json:"task_id,omitempty"`
	SliceID               string                      `json:"slice_id,omitempty"`
	TurnID                string                      `json:"turn_id,omitempty"`
	ResumedFromID         string                      `json:"resumed_from_continuation_id,omitempty"`
	OriginalIntent        string                      `json:"original_intent,omitempty"`
	Status                agentruntime.GoalStatus     `json:"status"`
	Reply                 string                      `json:"reply,omitempty"`
	GoalSummary           string                      `json:"goal_summary,omitempty"`
	CurrentStep           string                      `json:"current_step,omitempty"`
	CompletedSteps        int                         `json:"completed_steps,omitempty"`
	StopReason            string                      `json:"stop_reason,omitempty"`
	LimitType             string                      `json:"limit_type,omitempty"`
	Preview               string                      `json:"preview,omitempty"`
	UndoLabel             string                      `json:"undo_label,omitempty"`
	Error                 string                      `json:"error,omitempty"`
	NeedsClarification    bool                        `json:"needs_clarification,omitempty"`
	ClarificationQuestion string                      `json:"clarification_question,omitempty"`
	FailureReason         string                      `json:"failure_reason,omitempty"`
	Trace                 []planner.TraceEvent        `json:"trace,omitempty"`
	PlanItems             []planner.PlanItem          `json:"plan_items,omitempty"`
	Verification          *planner.VerificationResult `json:"verification,omitempty"`
	Executed              []map[string]any            `json:"executed,omitempty"`
	ContextSnapshot       map[string]any              `json:"context_snapshot,omitempty"`
	ProjectHistory        map[string]any              `json:"project_history,omitempty"`
	ExecutionMemory       ExecutionMemory             `json:"execution_memory,omitempty"`
	RecentObservation     *RecentObservation          `json:"recent_observation,omitempty"`
	SemanticAction        *semanticeffect.Action      `json:"semantic_action,omitempty"`
	FreeStateDecision     *FreeStateDecision          `json:"free_state_decision,omitempty"`
	Continuation          *Continuation               `json:"continuation,omitempty"`
	ModelProtocolRepairs  int                         `json:"model_protocol_repairs,omitempty"`
	ModelProtocolFailure  bool                        `json:"model_protocol_failure,omitempty"`
}

type Planner interface {
	Next(context.Context, planner.Input) (planner.Output, error)
}

type ToolExecutor interface {
	RunToolCall(context.Context, executorpkg.Input) (executorpkg.Result, error)
}

type Runner struct {
	Runtime  *agentruntime.Runtime
	Planner  Planner
	Executor ToolExecutor
	Budget   Budget
	Now      func() time.Time
}

func (r *Runner) Start(ctx context.Context, in Input) Result {
	goal := r.ensureGoal(in.GoalID, in.RunID, firstNonEmpty(in.Summary, in.UserText))
	baseBudget := normalizeBudget(firstNonZeroBudget(in.Budget, r.Budget))
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
	return r.loop(ctx, &state)
}

func (r *Runner) Continue(ctx context.Context, cont Continuation) Result {
	if r.Runtime == nil {
		return Result{Status: agentruntime.StatusFailed, StopReason: StopReasonFailed, Error: "goal runtime is nil"}
	}
	goal := r.Runtime.Continue(cont.GoalID, cont.Summary)
	if strings.TrimSpace(cont.Summary) != "" {
		goal.Summary = strings.TrimSpace(cont.Summary)
	}
	if r.Runtime != nil {
		goal = r.Runtime.ClearInterjections(goal.GoalID)
	}
	baseBudget := normalizeBudget(firstNonZeroBudget(cont.Budget, r.Budget))
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
	in := Input{
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
		Budget:            baseBudget,
		ExecutionMemory:   cloneExecutionMemory(cont.ExecutionMemory),
		RecentObservation: cloneRecentObservation(cont.RecentObservation),
		FreeStateDecision: cloneFreeStateDecision(cont.FreeStateDecision),
	}
	state := runState{
		input:               in,
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
	return r.loop(ctx, &state)
}

func (r *Runner) ResumeAfterConfirmation(ctx context.Context, cont Continuation) Result {
	goal := r.ensureGoal(cont.GoalID, cont.RunID, cont.Summary)
	baseBudget := normalizeBudget(firstNonZeroBudget(cont.Budget, r.Budget))
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
			Budget:            baseBudget,
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
	call := *cont.PendingToolCall
	call = coerceMixObservationCall(&state, call)
	call = coerceMixTickPrimitiveCall(&state, call)
	if issue := messageLoopToolGuardIssue(&state, call, messageLoopHasUsableMixObservation(&state)); issue != "" {
		result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: issue}
		state.trace = append(state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: &call},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: issue},
		)
		return r.loop(ctx, &state)
	}
	if stopped, result := r.executeTool(ctx, &state, call, true, nil); stopped {
		return result
	}
	return r.loop(ctx, &state)
}

type runState struct {
	input                Input
	goal                 agentruntime.Goal
	trace                []planner.TraceEvent
	executed             []map[string]any
	planItems            []planner.PlanItem
	pendingToolCall      *planner.ToolCall
	pendingToolQueue     []planner.ToolCall
	contextSnapshot      map[string]any
	projectHistory       map[string]any
	executionMemory      ExecutionMemory
	recentObservation    *RecentObservation
	semanticAction       *semanticeffect.Action
	freeStateDecision    *FreeStateDecision
	replanAfterTool      bool
	completedSteps       int
	turnsUsed            int
	toolCallsUsed        int
	sliceTurnsStart      int
	sliceToolCallsStart  int
	consecutiveErrors    int
	modelProtocolRepairs int
	modelProtocolFailure bool
	budget               Budget
	continuationBudget   Budget
	startedAt            time.Time
}

func (r *Runner) loop(ctx context.Context, state *runState) Result {
	if r.Planner == nil {
		return r.fail(state, fmt.Errorf("goal planner is nil"))
	}
	messageLoopApplyReadOnlyMutationBarrier(state)
	for {
		if stopped, result := r.checkpoint("before_plan", state); stopped {
			return result
		}
		if limit, result := r.checkTurnBudget(state); limit {
			return result
		}
		if r.Runtime != nil {
			state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusProcessing, nil)
		}
		snapshot := r.buildContextSnapshot(state)
		state.contextSnapshot = snapshot.Map()
		_ = contextruntime.AppendDefault(snapshot)
		out, err := r.Planner.Next(ctx, planner.Input{
			GoalID:                state.goal.GoalID,
			RunID:                 state.goal.RunID,
			UserText:              state.input.UserText,
			Context:               cloneMap(state.input.Context),
			ContextSnapshot:       cloneMap(state.contextSnapshot),
			State:                 cloneMap(state.input.State),
			CatalogSummary:        state.input.CatalogSummary,
			AllowedTools:          append([]string(nil), state.input.AllowedTools...),
			Trace:                 recentTrace(state.trace, 12),
			PlanItems:             append([]planner.PlanItem(nil), state.planItems...),
			MaxToolCallsRemaining: state.budget.MaxToolCalls - state.toolCallsUsed,
		})
		state.turnsUsed++
		if r.Runtime != nil {
			state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusRunning, nil)
		}
		if err != nil {
			if out.Raw != "" {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "planner_error", Message: err.Error(), Reply: out.Raw})
			}
			return r.fail(state, err)
		}
		if strings.TrimSpace(out.Reply) != "" {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "assistant", Reply: strings.TrimSpace(out.Reply)})
		}
		if len(out.PlanItems) > 0 {
			state.planItems = mergePlanItems(state.planItems, out.PlanItems)
			state.trace = append(state.trace, planUpdateEvent(state.planItems, "planner updated task ledger"))
		}
		if out.Verification != nil {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "verification", Verification: out.Verification})
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
		if out.Done || len(out.ToolCalls) == 0 {
			if issue := finalGateIssue(state, out); issue != "" {
				state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: issue, PlanItems: append([]planner.PlanItem(nil), state.planItems...)})
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
			call := out.ToolCalls[i]
			if strings.TrimSpace(call.PlanItemID) != "" {
				state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "running", "")
				state.trace = append(state.trace, planUpdateEvent(state.planItems, "tool started for plan item"))
			}
			if stopped, result := r.checkpoint("before_tool", state); stopped {
				return result
			}
			if limit, result := r.checkToolBudget(state); limit {
				return result
			}
			call = coerceWaveformBakeToMixObservation(state, call)
			call = coerceMixObservationCall(state, call)
			call = coerceMixTickPrimitiveCall(state, call)
			if !allowedTool(call.Tool, state.input.AllowedTools) {
				result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: "未知或不允许的工具：" + strings.TrimSpace(call.Tool)}
				state.trace = append(state.trace,
					planner.TraceEvent{Kind: "tool_call", ToolCall: &call},
					planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
				)
				state.consecutiveErrors++
				if state.consecutiveErrors >= state.budget.MaxConsecutiveErrors {
					return r.fail(state, errors.New(result.Error))
				}
				continue
			}
			if issue := messageLoopToolGuardIssue(state, call, hadMixObservationBeforeTurn); issue != "" {
				result := planner.ToolResult{ToolCallID: stableToolCallID(call, state.completedSteps+1), Tool: call.Tool, Status: "error", Error: issue}
				state.trace = append(state.trace,
					planner.TraceEvent{Kind: "tool_call", ToolCall: &call},
					planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
					planner.TraceEvent{Kind: "final_gate", Message: issue},
				)
				continue
			}
			remaining := append([]planner.ToolCall(nil), out.ToolCalls[i+1:]...)
			if r.Runtime != nil {
				state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusExecuting, nil)
			}
			if stopped, result := r.executeTool(ctx, state, call, false, remaining); stopped {
				return result
			}
			if state.replanAfterTool {
				state.replanAfterTool = false
				break
			}
		}
	}
}

func (r *Runner) executeTool(ctx context.Context, state *runState, call planner.ToolCall, confirmed bool, queued []planner.ToolCall) (bool, Result) {
	if r.Executor == nil {
		return true, r.fail(state, fmt.Errorf("goal executor is nil"))
	}
	if strings.TrimSpace(call.ID) == "" {
		call.ID = stableToolCallID(call, state.completedSteps+1)
	}
	state.trace = append(state.trace, planner.TraceEvent{Kind: "tool_call", ToolCall: cloneToolCallPtr(call)})
	execResult, err := r.Executor.RunToolCall(ctx, executorpkg.Input{
		GoalID:    state.goal.GoalID,
		RunID:     state.goal.RunID,
		ToolCall:  call,
		Context:   cloneMap(state.input.Context),
		Confirmed: confirmed,
		Source:    "agentloop",
	})
	state.toolCallsUsed++
	if strings.TrimSpace(execResult.ToolCallID) == "" {
		execResult.ToolCallID = call.ID
	}
	toolResult := planner.ToolResult{
		ToolCallID: execResult.ToolCallID,
		Tool:       firstNonEmpty(execResult.Tool, call.Tool),
		Status:     execResult.Status,
		Error:      execResult.Error,
		Result:     execResult.Result,
	}
	if err != nil && toolResult.Error == "" {
		toolResult.Error = err.Error()
	}
	state.trace = append(state.trace, planner.TraceEvent{Kind: "tool_result", ToolResult: &toolResult})
	execRecord := map[string]any{
		"status":          execResult.Status,
		"tool_call_id":    execResult.ToolCallID,
		"agent_action_id": execResult.AgentActionID,
		"tool":            execResult.Tool,
		"command_name":    execResult.CommandName,
		"preview":         execResult.Preview,
		"undo_label":      execResult.UndoLabel,
		"result":          execResult.Result,
		"error":           toolResult.Error,
	}
	if len(execResult.ProjectHistory) > 0 {
		state.projectHistory = cloneMap(execResult.ProjectHistory)
		execRecord["project_history"] = execResult.ProjectHistory
	}
	if len(execResult.ObservedState) > 0 {
		state.input.State = cloneMap(execResult.ObservedState)
		execRecord["observed_state"] = true
	}
	if interactionPause, waitingForUser := pendingInteractionPauseForResult(execResult.Result); waitingForUser {
		// A workflow card is a real control-flow boundary, not merely display text.
		state.pendingToolQueue = nil
		ver := verifyToolExecution(call, execResult)
		state.trace = append(state.trace, planner.TraceEvent{Kind: "verification", Verification: &ver})
		execRecord["verification"] = ver
		state.recentObservation = recentObservationForTool(call, execResult, ver, false, nil)
		recordFreeStateCCBObservation(state, state.recentObservation)
		if strings.TrimSpace(call.PlanItemID) != "" {
			state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "pending", "waiting for formal user interaction")
			state.trace = append(state.trace, planUpdateEvent(state.planItems, "plan item is waiting for formal user interaction"))
		}
		state.executed = append(state.executed, execRecord)
		result := r.pause(state, interactionPause.Status, interactionPause.StopReason, "", interactionPause.Reply, execResult.Preview, execResult.UndoLabel, nil)
		if interactionPause.Status == agentruntime.StatusWaitingClarification {
			result.NeedsClarification = true
			result.ClarificationQuestion = interactionPause.Reply
		}
		return true, result
	}
	if execResult.RequiresConfirmation {
		state.pendingToolQueue = nil
		ver := verificationForPendingConfirmation(call, execResult)
		state.trace = append(state.trace, planner.TraceEvent{Kind: "verification", Verification: &ver})
		execRecord["verification"] = ver
		state.recentObservation = recentObservationForTool(call, execResult, ver, false, nil)
		recordFreeStateCCBObservation(state, state.recentObservation)
		if strings.TrimSpace(call.PlanItemID) != "" {
			state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "waiting_confirmation", planEvidenceText(ver))
			state.trace = append(state.trace, planUpdateEvent(state.planItems, "plan item is waiting for confirmation"))
		}
		if messageLoopShouldAutoApplyConfirmedMixTick(state, call, execResult) {
			state.executed = append(state.executed, execRecord)
			applyCall := messageLoopPendingConfirmationCall(call, execResult)
			return r.executeTool(ctx, state, applyCall, true, nil)
		}
		state.executed = append(state.executed, execRecord)
		if r.Runtime != nil {
			state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusWaitingConfirmation, nil)
		} else {
			state.goal.Status = agentruntime.StatusWaitingConfirmation
		}
		pending := messageLoopPendingConfirmationCall(call, execResult)
		state.pendingToolCall = &pending
		return true, r.pause(state, agentruntime.StatusWaitingConfirmation, StopReasonNeedsConfirmation, "", "这个操作需要你确认后才会执行。", execResult.Preview, execResult.UndoLabel, &pending)
	}
	ver := verifyToolExecution(call, execResult)
	state.trace = append(state.trace, planner.TraceEvent{Kind: "verification", Verification: &ver})
	execRecord["verification"] = ver
	producedBindings := updateExecutionMemoryForTool(state, call, execResult, ver)
	mutationBarrier := toolNeedsMutationBarrier(call, execResult)
	state.recentObservation = recentObservationForTool(call, execResult, ver, mutationBarrier, producedBindings)
	recordFreeStateCCBObservation(state, state.recentObservation)
	if mutationBarrier && len(queued) > 0 {
		state.pendingToolQueue = nil
	}
	if mutationBarrier {
		message := "project-mutating tool completed; the runtime will re-observe state and re-bind targets"
		if len(queued) > 0 {
			message = "project-mutating tool completed; queued tool calls were discarded so the runtime can re-observe state and re-bind targets"
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:    "mutation_barrier",
			Message: message,
		})
	}
	state.executed = append(state.executed, execRecord)
	if strings.TrimSpace(call.PlanItemID) != "" {
		switch ver.Status {
		case verificationVerified, verificationNotRequired:
			state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "completed", planEvidenceText(ver))
		case verificationFailed:
			state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "failed", planEvidenceText(ver))
		case verificationUnverified:
			state.planItems = markPlanItemStatus(state.planItems, call.PlanItemID, "pending", planEvidenceText(ver))
		}
		state.trace = append(state.trace, planUpdateEvent(state.planItems, "verification updated plan item"))
	}
	if toolResult.Error != "" || strings.EqualFold(execResult.Status, "error") || strings.EqualFold(execResult.Status, "kernel_error") {
		state.consecutiveErrors++
		if state.consecutiveErrors >= state.budget.MaxConsecutiveErrors {
			return true, r.fail(state, errors.New(firstNonEmpty(toolResult.Error, "工具执行失败")))
		}
	} else {
		state.consecutiveErrors = 0
		state.completedSteps++
	}
	if r.Runtime != nil {
		state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusRunning, nil)
	}
	if stopped, result := r.checkpoint("after_tool", state); stopped {
		return true, result
	}
	if mutationBarrier {
		state.replanAfterTool = true
	}
	return false, Result{}
}

func (r *Runner) checkpoint(label string, state *runState) (bool, Result) {
	if state == nil {
		return true, Result{Status: agentruntime.StatusFailed, StopReason: StopReasonFailed, Error: "runner state is nil"}
	}
	if state.budget.Timeout > 0 && !state.startedAt.IsZero() && r.now().Sub(state.startedAt) >= state.budget.Timeout {
		return true, r.pause(state, agentruntime.StatusWaitingContinue, StopReasonLimitReached, LimitTypeTimeout, limitReply(LimitTypeTimeout), "", "", nil)
	}
	if r.Runtime != nil {
		state.goal = r.Runtime.Tick(state.goal.GoalID, label)
	}
	if state.goal.StopRequested || state.goal.Status == agentruntime.StatusStopped {
		if r.Runtime != nil {
			state.goal = r.Runtime.MarkStopped(state.goal.GoalID, state.goal.LastCheckpoint)
		} else {
			state.goal.Status = agentruntime.StatusStopped
		}
		return true, r.result(state, agentruntime.StatusStopped, StopReasonCancelled, "", "已停止。", "", "", nil)
	}
	if state.goal.CancelRequested || state.goal.Status == agentruntime.StatusCancelled || state.goal.Status == agentruntime.StatusCancelling {
		if r.Runtime != nil {
			state.goal = r.Runtime.SetStatus(state.goal.GoalID, agentruntime.StatusCancelled, nil)
		} else {
			state.goal.Status = agentruntime.StatusCancelled
		}
		return true, r.result(state, agentruntime.StatusCancelled, StopReasonCancelled, "", "已取消。", "", "", nil)
	}
	if len(state.goal.PendingInterjection) > 0 {
		msg := "收到新的用户输入，当前任务已暂停，等待你确认下一步。"
		return true, r.pause(state, agentruntime.StatusWaitingContinue, StopReasonInterjection, "", msg, "", "", nil)
	}
	return false, Result{}
}

func (r *Runner) checkTurnBudget(state *runState) (bool, Result) {
	if state.budget.MaxTurns > 0 && state.turnsUsed >= state.budget.MaxTurns {
		return true, r.pause(state, agentruntime.StatusWaitingContinue, StopReasonLimitReached, LimitTypeTurns, limitReply(LimitTypeTurns), "", "", nil)
	}
	return false, Result{}
}

func (r *Runner) checkToolBudget(state *runState) (bool, Result) {
	if state.budget.MaxToolCalls > 0 && state.toolCallsUsed >= state.budget.MaxToolCalls {
		return true, r.pause(state, agentruntime.StatusWaitingContinue, StopReasonLimitReached, LimitTypeToolCalls, limitReply(LimitTypeToolCalls), "", "", nil)
	}
	return false, Result{}
}

func (r *Runner) complete(state *runState, reply string) Result {
	if r.Runtime != nil {
		state.goal = r.Runtime.Complete(state.goal.GoalID, nil)
	} else {
		state.goal.Status = agentruntime.StatusCompleted
	}
	return r.result(state, agentruntime.StatusCompleted, StopReasonDone, "", reply, "", "", nil)
}

func (r *Runner) fail(state *runState, err error) Result {
	if err == nil {
		err = fmt.Errorf("goal failed")
	}
	if r.Runtime != nil && state != nil && state.goal.GoalID != "" {
		state.goal = r.Runtime.Complete(state.goal.GoalID, err)
	}
	stopReason := StopReasonFailed
	if state != nil && state.modelProtocolFailure {
		stopReason = StopReasonModelProtocolFailure
	}
	res := r.result(state, agentruntime.StatusFailed, stopReason, "", "执行失败："+friendlyAgentLoopError(err), "", "", nil)
	res.Error = err.Error()
	res.FailureReason = err.Error()
	return res
}

func (r *Runner) pause(state *runState, status agentruntime.GoalStatus, stopReason, limitType, reply, preview, undoLabel string, pending *planner.ToolCall) Result {
	if r.Runtime != nil {
		state.goal = r.Runtime.SetStatus(state.goal.GoalID, status, nil)
	} else {
		state.goal.Status = status
	}
	return r.result(state, status, stopReason, limitType, reply, preview, undoLabel, pending)
}

func (r *Runner) result(state *runState, status agentruntime.GoalStatus, stopReason, limitType, reply, preview, undoLabel string, pending *planner.ToolCall) Result {
	if state == nil {
		return Result{Status: status, StopReason: stopReason, LimitType: limitType, Reply: reply, Preview: preview, UndoLabel: undoLabel}
	}
	if pending != nil {
		state.pendingToolCall = pending
	} else {
		state.pendingToolCall = nil
	}
	if r.Runtime != nil && state.input.TurnID != "" {
		state.goal = r.Runtime.EndTurn(state.goal.GoalID, state.input.TurnID, string(status),
			maxInt(0, state.turnsUsed-state.sliceTurnsStart), maxInt(0, state.toolCallsUsed-state.sliceToolCallsStart))
	}
	snapshot := r.buildContextSnapshot(state)
	state.contextSnapshot = snapshot.Map()
	cont := &Continuation{
		GoalID:            state.goal.GoalID,
		RunID:             state.goal.RunID,
		TaskID:            state.input.TaskID,
		SliceID:           state.input.SliceID,
		TurnID:            state.input.TurnID,
		OriginalIntent:    firstNonEmpty(state.input.OriginalIntent, state.goal.TaskIntent()),
		UserText:          state.input.UserText,
		Summary:           firstNonEmpty(state.input.Summary, state.goal.Summary, state.input.UserText),
		Context:           cloneMap(state.input.Context),
		ContextSnapshot:   cloneMap(state.contextSnapshot),
		ProjectHistory:    cloneMap(projectHistoryFromState(state)),
		State:             cloneMap(state.input.State),
		Conversation:      append([]llm.Message(nil), state.input.Conversation...),
		CatalogSummary:    state.input.CatalogSummary,
		AllowedTools:      append([]string(nil), state.input.AllowedTools...),
		Trace:             append([]planner.TraceEvent(nil), state.trace...),
		PlanItems:         append([]planner.PlanItem(nil), state.planItems...),
		Budget:            firstNonZeroBudget(state.continuationBudget, state.budget),
		PendingToolCall:   pending,
		PendingToolQueue:  append([]planner.ToolCall(nil), state.pendingToolQueue...),
		CompletedSteps:    state.completedSteps,
		TurnsUsed:         state.turnsUsed,
		ToolCallsUsed:     state.toolCallsUsed,
		ExecutionMemory:   cloneExecutionMemory(state.executionMemory),
		RecentObservation: cloneRecentObservation(state.recentObservation),
		FreeStateDecision: cloneFreeStateDecision(state.freeStateDecision),
	}
	if status == agentruntime.StatusCompleted || status == agentruntime.StatusCancelled || status == agentruntime.StatusStopped || status == agentruntime.StatusFailed {
		cont = nil
	}
	currentStep := ""
	if pending != nil {
		currentStep = firstNonEmpty(pending.Reason, pending.Tool)
	}
	if currentStep == "" {
		currentStep = currentStepFromPlanItems(state.planItems)
	}
	return Result{
		Goal:                 state.goal,
		GoalID:               state.goal.GoalID,
		RunID:                state.goal.RunID,
		TaskID:               state.input.TaskID,
		SliceID:              state.input.SliceID,
		TurnID:               state.input.TurnID,
		ResumedFromID:        state.input.ResumedFromID,
		OriginalIntent:       firstNonEmpty(state.input.OriginalIntent, state.goal.TaskIntent()),
		Status:               status,
		Reply:                strings.TrimSpace(reply),
		GoalSummary:          firstNonEmpty(state.goal.Summary, state.input.Summary, state.input.UserText),
		CurrentStep:          currentStep,
		CompletedSteps:       state.completedSteps,
		StopReason:           stopReason,
		LimitType:            limitType,
		Preview:              preview,
		UndoLabel:            undoLabel,
		Verification:         lastVerification(state.trace),
		Trace:                append([]planner.TraceEvent(nil), state.trace...),
		PlanItems:            append([]planner.PlanItem(nil), state.planItems...),
		Executed:             append([]map[string]any(nil), state.executed...),
		ContextSnapshot:      cloneMap(state.contextSnapshot),
		ProjectHistory:       cloneMap(projectHistoryFromState(state)),
		ExecutionMemory:      cloneExecutionMemory(state.executionMemory),
		RecentObservation:    cloneRecentObservation(state.recentObservation),
		SemanticAction:       cloneSemanticEffectAction(state.semanticAction),
		FreeStateDecision:    cloneFreeStateDecision(state.freeStateDecision),
		Continuation:         cont,
		ModelProtocolRepairs: state.modelProtocolRepairs,
		ModelProtocolFailure: state.modelProtocolFailure,
	}
}

func cloneSemanticEffectAction(in *semanticeffect.Action) *semanticeffect.Action {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var out semanticeffect.Action
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return &out
}

func projectHistoryFromState(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	if state.goal.ProjectHistory != nil {
		out := map[string]any{
			"baseline_commit":  state.goal.ProjectHistory.BaselineCommitID,
			"baseline_created": state.goal.ProjectHistory.BaselineCreated,
			"active_branch":    state.goal.ProjectHistory.ActiveBranch,
			"detached":         state.goal.ProjectHistory.Detached,
			"head":             state.goal.ProjectHistory.Head,
		}
		if len(state.goal.ProjectHistory.Warnings) > 0 {
			out["warnings"] = append([]string(nil), state.goal.ProjectHistory.Warnings...)
		}
		for key, value := range out {
			if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
				delete(out, key)
			}
		}
		return out
	}
	return cloneMap(state.projectHistory)
}

func currentStepFromPlanItems(items []planner.PlanItem) string {
	for _, status := range []string{"running", "waiting_confirmation", "pending"} {
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item.Status), status) {
				return firstNonEmpty(item.Description, item.ID)
			}
		}
	}
	return ""
}

func lastVerification(trace []planner.TraceEvent) *planner.VerificationResult {
	for i := len(trace) - 1; i >= 0; i-- {
		if trace[i].Verification != nil {
			ver := *trace[i].Verification
			if len(ver.Evidence) > 0 {
				ver.Evidence = cloneMap(ver.Evidence)
			}
			return &ver
		}
	}
	return nil
}

func (r *Runner) ensureGoal(goalID, runID, summary string) agentruntime.Goal {
	if r.Runtime == nil {
		return agentruntime.Goal{GoalID: strings.TrimSpace(goalID), RunID: strings.TrimSpace(runID), Status: agentruntime.StatusRunning, Summary: strings.TrimSpace(summary)}
	}
	return r.Runtime.Ensure(goalID, runID, summary)
}

func (r *Runner) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func normalizeBudget(b Budget) Budget {
	if b.MaxTurns <= 0 {
		b.MaxTurns = 8
	}
	if b.MaxToolCalls <= 0 {
		b.MaxToolCalls = 12
	}
	if b.MaxConsecutiveErrors <= 0 {
		b.MaxConsecutiveErrors = 2
	}
	return b
}

var defaultCatalog = tools.DefaultCatalog()

func firstNonZeroBudget(values ...Budget) Budget {
	for _, b := range values {
		if b.MaxTurns > 0 || b.MaxToolCalls > 0 || b.Timeout > 0 || b.MaxConsecutiveErrors > 0 {
			return b
		}
	}
	return Budget{}
}

// extendContinuationBudget grants a fresh per-invocation slice while keeping
// cumulative usage counters in the continuation for reporting and guards.
func extendContinuationBudget(base Budget, turnsUsed, toolCallsUsed int) Budget {
	base = normalizeBudget(base)
	if turnsUsed > 0 && base.MaxTurns > 0 {
		base.MaxTurns += turnsUsed
	}
	if toolCallsUsed > 0 && base.MaxToolCalls > 0 {
		base.MaxToolCalls += toolCallsUsed
	}
	return base
}

func stableToolCallID(call planner.ToolCall, step int) string {
	if strings.TrimSpace(call.ID) != "" {
		return strings.TrimSpace(call.ID)
	}
	if step <= 0 {
		step = 1
	}
	return fmt.Sprintf("tool_step_%d", step)
}

func allowedTool(tool string, allowed []string) bool {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return false
	}
	if len(allowed) == 0 || tool == "daw.invoke" {
		return true
	}
	catalog := tools.DefaultCatalog()
	requestedCommand := ""
	if spec, ok := catalog.LookupTool(tool); ok {
		requestedCommand = strings.TrimSpace(spec.CommandName)
	} else if spec, ok := catalog.LookupCommand(tool); ok {
		requestedCommand = strings.TrimSpace(spec.CommandName)
	}
	for _, name := range allowed {
		allowedName := strings.TrimSpace(name)
		if tool == allowedName {
			return true
		}
		if requestedCommand == "" {
			continue
		}
		if spec, ok := catalog.LookupTool(allowedName); ok && requestedCommand == strings.TrimSpace(spec.CommandName) {
			return true
		}
		if spec, ok := catalog.LookupCommand(allowedName); ok && requestedCommand == strings.TrimSpace(spec.CommandName) {
			return true
		}
	}
	return false
}

func limitReply(limitType string) string {
	switch limitType {
	case LimitTypeTurns:
		return "本轮思考步数已到上限；任务会从已保存的检查点自动继续。"
	case LimitTypeToolCalls:
		return "本轮工具调用次数已到上限；任务会从已保存的检查点自动继续。"
	case LimitTypeTimeout:
		return "本轮运行时间已到上限；任务会从已保存的检查点自动继续。"
	default:
		return "本轮运行预算已到上限；任务会从已保存的检查点自动继续。"
	}
}

func friendlyAgentLoopError(err error) string {
	if err == nil {
		return "目标执行失败。"
	}
	text := strings.TrimSpace(err.Error())
	if text == "" {
		return "目标执行失败。"
	}
	switch {
	case strings.Contains(text, "TLS handshake timeout"):
		return "连接 AI 服务超时，请稍后再试。"
	case strings.Contains(text, "context canceled"):
		return "请求已取消或连接中断，请重新发送一次。"
	case strings.Contains(text, "invalid JSON") || strings.Contains(text, "有效 JSON") || strings.Contains(text, "计划格式"):
		return "Agent 返回的计划格式不完整，这次没有执行工程修改。"
	default:
		return text
	}
}
