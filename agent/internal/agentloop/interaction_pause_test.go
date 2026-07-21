package agentloop

import (
	"context"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type interactionPausePlanner struct{}

func (interactionPausePlanner) Next(context.Context, planner.Input) (planner.Output, error) {
	return planner.Output{ToolCalls: []planner.ToolCall{
		{ID: "learn", Tool: "plugin_grabber.learn_project_profile"},
		{ID: "must_not_run", Tool: "plugin_grabber.explain_controls"},
	}}, nil
}

type interactionPauseExecutor struct {
	calls []string
}

func (e *interactionPauseExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall.ID)
	return executorpkg.Result{
		ToolCallID: in.ToolCall.ID,
		Tool:       in.ToolCall.Tool,
		Status:     "ok",
		Result: map[string]any{
			"reply": "请选择是否跳过插件界面图样。",
			"interaction_requests": []map[string]any{{
				"id":     "interaction_ui_reference",
				"kind":   "form",
				"type":   "plugin_learning_ui_reference_request",
				"status": "waiting_for_user",
				"body":   "请选择是否跳过插件界面图样。",
			}},
		},
	}, nil
}

func TestRunnerStopsQueuedToolsForFormalInteraction(t *testing.T) {
	executor := &interactionPauseExecutor{}
	runner := Runner{Planner: interactionPausePlanner{}, Executor: executor}
	result := runner.Start(context.Background(), Input{
		GoalID:       "goal_interaction",
		RunID:        "run_interaction",
		UserText:     "学习当前插件",
		AllowedTools: []string{"plugin_grabber.learn_project_profile", "plugin_grabber.explain_controls"},
	})
	if result.Status != agentruntime.StatusWaitingClarification || result.StopReason != StopReasonNeedsClarification {
		t.Fatalf("result = %+v", result)
	}
	if !result.NeedsClarification || result.ClarificationQuestion != "请选择是否跳过插件界面图样。" {
		t.Fatalf("clarification = %+v", result)
	}
	if len(executor.calls) != 1 || executor.calls[0] != "learn" {
		t.Fatalf("queued tools continued after interaction: %+v", executor.calls)
	}
	if len(result.Executed) != 1 || result.Executed[0]["tool_call_id"] != "learn" {
		t.Fatalf("executed = %#v", result.Executed)
	}
}
