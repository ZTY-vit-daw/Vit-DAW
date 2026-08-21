package agentloop

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

type advancingMessageCompleter struct {
	now       *time.Time
	advances  []time.Duration
	responses []string
	calls     [][]llm.Message
}

func (f *advancingMessageCompleter) Complete(_ context.Context, _ config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), messages...))
	if len(f.advances) > 0 {
		*f.now = f.now.Add(f.advances[0])
		f.advances = f.advances[1:]
	}
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"done"}`, nil
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func TestMessageLoopContinueGrantsFreshBudgetSlice(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"continued"}`,
	}}
	loop := &MessageLoop{
		Runtime: loopTestRuntime(),
		Client:  client,
		Config:  config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget:  Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	result := loop.Continue(context.Background(), Continuation{
		GoalID:    "goal_continuation_budget",
		RunID:     "run_continuation_budget",
		UserText:  "continue the work",
		Summary:   "continue the work",
		Budget:    Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		TurnsUsed: 2,
	})

	if result.Status != agentruntime.StatusCompleted || result.Reply != "continued" {
		t.Fatalf("continuation result = status=%q reply=%q stop=%q", result.Status, result.Reply, result.StopReason)
	}
	if len(client.calls) != 1 {
		t.Fatalf("continuation LLM calls = %d, want 1", len(client.calls))
	}
	if result.TaskID == "" || result.SliceID == "" || result.TurnID == "" || result.OriginalIntent != "continue the work" {
		t.Fatalf("default message loop did not bind task/run/slice/turn identity: %+v", result)
	}
	goal := loop.Runtime.Status(result.GoalID)
	if goal.Task == nil || len(goal.Task.Run.Slices) != 1 || len(goal.Task.Run.Turns) != 1 || goal.Task.Run.Slices[0].SliceID != result.SliceID {
		t.Fatalf("default message loop did not project its invocation into task runtime: %+v", goal)
	}
}

func TestMessageLoopWallClockYieldPreservesQueuedToolAndResumesOnce(t *testing.T) {
	now := time.Date(2026, 8, 12, 22, 0, 0, 0, time.UTC)
	client := &advancingMessageCompleter{
		now:      &now,
		advances: []time.Duration{3*time.Minute + time.Second, 0},
		responses: []string{
			`{"final":false,"reply":"read current state","tool_calls":[{"id":"state-once","tool":"project.state","args":{}}]}`,
			`{"final":true,"reply":"continued after checkpoint"}`,
		},
	}
	executor := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Runtime:  loopTestRuntime(),
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: executor,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, Timeout: 3 * time.Minute, MaxConsecutiveErrors: 2},
		Now:      func() time.Time { return now },
	}
	originalObservation := &RecentObservation{
		ToolCallID: "ccb-before",
		Tool:       "ccb.observation_request",
		Status:     "ok",
		Summary:    map[string]any{"observation_id": "obs-before", "schema_version": "ccb_observation_bundle.v1"},
	}

	paused := loop.Start(context.Background(), Input{
		GoalID:   "goal_wall_clock_slice",
		RunID:    "run_wall_clock_slice",
		UserText: "run the queued continuation test",
		Summary:  "run the queued continuation test",
		Context: map[string]any{
			"observation_ledger": map[string]any{"active_observation_id": "obs-before"},
		},
		AllowedTools:      []string{"project.state"},
		RecentObservation: originalObservation,
		ExecutionMemory: ExecutionMemory{
			ActiveWorkTargetTrackID: "1032",
		},
	})

	if paused.Status != agentruntime.StatusWaitingContinue || paused.StopReason != StopReasonLimitReached || paused.LimitType != LimitTypeTimeout {
		t.Fatalf("paused result = status=%q stop=%q limit=%q reply=%q", paused.Status, paused.StopReason, paused.LimitType, paused.Reply)
	}
	if paused.Continuation == nil {
		t.Fatal("wall-clock yield did not create a continuation")
	}
	if paused.TaskID == "" || paused.SliceID == "" || paused.TurnID == "" || paused.OriginalIntent != "run the queued continuation test" {
		t.Fatalf("initial message-loop slice identity is incomplete: %+v", paused)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("tool executed after the time slice elapsed: %+v", executor.calls)
	}
	if got := paused.Continuation.PendingToolQueue; len(got) != 1 || got[0].ID != "state-once" {
		t.Fatalf("pending tool queue = %+v, want state-once", got)
	}
	if paused.Continuation.ExecutionMemory.ActiveWorkTargetTrackID != "1032" {
		t.Fatalf("target binding was lost: %+v", paused.Continuation.ExecutionMemory)
	}
	if paused.Continuation.RecentObservation == nil || paused.Continuation.RecentObservation.ToolCallID != "ccb-before" {
		t.Fatalf("active observation was lost: %+v", paused.Continuation.RecentObservation)
	}
	ledger, _ := paused.Continuation.Context["observation_ledger"].(map[string]any)
	if ledger["active_observation_id"] != "obs-before" {
		t.Fatalf("observation ledger was lost: %+v", paused.Continuation.Context)
	}

	paused.Continuation.ContinuationID = "cont_wall_clock_checkpoint"
	paused.Continuation.Summary = "resume from the saved checkpoint"
	resumed := loop.Continue(context.Background(), *paused.Continuation)
	if resumed.Status != agentruntime.StatusCompleted || resumed.Reply != "continued after checkpoint" {
		t.Fatalf("resumed result = status=%q stop=%q reply=%q error=%q trace=%+v", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error, resumed.Trace)
	}
	if len(executor.calls) != 1 || executor.calls[0].ID != "state-once" {
		t.Fatalf("resumed tool calls = %+v, want exactly one state-once", executor.calls)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want plan plus resumed completion", len(client.calls))
	}
	if resumed.TaskID != paused.TaskID || resumed.GoalID != paused.GoalID || resumed.RunID != paused.RunID || resumed.SliceID == paused.SliceID || resumed.ResumedFromID != "cont_wall_clock_checkpoint" || resumed.OriginalIntent != paused.OriginalIntent {
		t.Fatalf("automatic message-loop continuation changed identity or reused its slice: paused=%+v resumed=%+v", paused, resumed)
	}
	goal := loop.Runtime.Status(paused.GoalID)
	if goal.Task == nil || len(goal.Task.Run.Slices) != 2 || len(goal.Task.Run.Turns) != 2 {
		t.Fatalf("message-loop continuation did not create a second durable slice/turn: %+v", goal)
	}
	if goal.Task.Run.Slices[1].TurnsUsed > paused.Continuation.Budget.MaxTurns || goal.Task.Run.Slices[1].ToolCallsUsed > paused.Continuation.Budget.MaxToolCalls {
		t.Fatalf("new slice stored cumulative rather than slice-local usage: %+v", goal.Task.Run.Slices[1])
	}
	for _, event := range resumed.Trace {
		if event.Kind == "tool_call" && event.ToolCall != nil && event.ToolCall.ID == "state-once" {
			return
		}
	}
	t.Fatalf("resumed trace has no execution for state-once: %+v", resumed.Trace)
}

func loopTestRuntime() *agentruntime.Runtime {
	return agentruntime.New()
}
