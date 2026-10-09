package pullharness

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
)

type cancelLLMFunc func(context.Context, config.EngineConfig, []llm.Message) (string, error)

func (f cancelLLMFunc) Complete(ctx context.Context, cfg config.EngineConfig, msgs []llm.Message) (string, error) {
	return f(ctx, cfg, msgs)
}

type cancelRouterFunc func(context.Context, GoalInput) (FastPathOutcome, bool)

func (f cancelRouterFunc) Route(ctx context.Context, in GoalInput) (FastPathOutcome, bool) {
	return f(ctx, in)
}

type cancelTools struct {
	plan    func(string) []ToolCall
	execute func(context.Context, []ToolCall) []ToolResult
}

func (f cancelTools) Plan(reply string) []ToolCall { return f.plan(reply) }
func (f cancelTools) Execute(ctx context.Context, calls []ToolCall) []ToolResult {
	return f.execute(ctx, calls)
}

// Each cancellation happens synchronously inside the named stage, including
// successful returns from components that have already performed work.
func TestPullLoopCancellationBoundaries(t *testing.T) {
	for _, stage := range []string{"entry", "complete", "plan", "execute_empty", "execute_result", "route"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var routes, models, plans, executes int
			exit := &fakeExit{}
			loop := &PullLoop{
				Prefix: &fakePrefix{}, Exit: exit,
				Budget: ObservationBudget{ProbeCost: 0.25, MaxProbeCost: 1, MaxCycles: 1},
			}
			loop.Router = cancelRouterFunc(func(context.Context, GoalInput) (FastPathOutcome, bool) {
				routes++
				if stage == "route" {
					cancel()
					return FastPathOutcome{Reply: "routed fact", ProbeCost: 0.75}, true
				}
				return FastPathOutcome{}, false
			})
			loop.LLM = cancelLLMFunc(func(context.Context, config.EngineConfig, []llm.Message) (string, error) {
				models++
				if stage == "complete" {
					cancel()
				}
				return "model fact", nil
			})
			loop.Tools = cancelTools{
				plan: func(string) []ToolCall {
					plans++
					if stage == "plan" {
						cancel()
					}
					return []ToolCall{{ID: "action-1", Tool: "existing.tool"}}
				},
				execute: func(context.Context, []ToolCall) []ToolResult {
					executes++
					cancel()
					if stage == "execute_empty" {
						return nil
					}
					return []ToolResult{{ID: "action-1", Tool: "existing.tool", Status: "ok",
						ModelLine: "executed fact", ProbeCost: 0.75}}
				},
			}
			in := goalInput("run-cancel-" + stage)
			in.Conversation = []llm.Message{{Role: "assistant", Content: "prior fact"}}
			if stage == "entry" {
				cancel()
			}
			result := loop.Run(ctx, in)
			if result.Outcome != OutcomeInterrupted {
				t.Errorf("outcome=%q, want interrupted", result.Outcome)
			}
			if result.Cycles != 0 || len(exit.events) != 0 {
				t.Errorf("cancelled batch counted complete or fired T1/T2: cycles=%d events=%+v", result.Cycles, exit.events)
			}
			want := map[string][4]int{
				"entry": {0, 0, 0, 0}, "route": {1, 0, 0, 0}, "complete": {1, 1, 0, 0},
				"plan": {1, 1, 1, 0}, "execute_empty": {1, 1, 1, 1}, "execute_result": {1, 1, 1, 1},
			}[stage]
			if got := [4]int{routes, models, plans, executes}; got != want {
				t.Errorf("route/model/plan/execute=%v, want %v", got, want)
			}
			if result.ModelTurns != models {
				t.Errorf("model turns=%d, calls=%d", result.ModelTurns, models)
			}
			if result.Conversation[0] != in.Conversation[0] {
				t.Error("lost incoming conversation")
			}
			wantCost := 0.25
			wantConversation := append(append([]llm.Message(nil), in.Conversation...), llm.Message{Role: "user", Content: in.UserText})
			wantReply := ""
			if models > 0 {
				wantReply = "model fact"
				wantConversation = append(wantConversation, llm.Message{Role: "assistant", Content: wantReply})
			}
			if stage == "route" {
				wantCost = 1
				wantReply = "routed fact"
			}
			if stage == "execute_result" {
				wantCost = 1
				wantConversation = append(wantConversation, llm.Message{Role: "user", Content: "executed fact"})
				if !strings.Contains(strings.Join(result.Trace, "\n"), "action-1") {
					t.Error("returned tool ID has no Result carrier")
				}
			}
			if result.ProbeSpent != wantCost || result.Reply != wantReply {
				t.Errorf("lost reply/cost: reply=%q cost=%v, want %q/%v", result.Reply, result.ProbeSpent, wantReply, wantCost)
			}
			if len(result.Conversation) != len(wantConversation) {
				t.Errorf("conversation length=%d, want %d: %+v", len(result.Conversation), len(wantConversation), result.Conversation)
			} else {
				for i := range wantConversation {
					if result.Conversation[i] != wantConversation[i] {
						t.Errorf("conversation[%d]=%+v, want %+v", i, result.Conversation[i], wantConversation[i])
					}
				}
			}
		})
	}
}

func TestPullLoopCancellationKeepsEarlierCompletedBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	models, executes := 0, 0
	exit := &fakeExit{}
	loop := &PullLoop{Prefix: &fakePrefix{}, Exit: exit, Budget: ObservationBudget{MaxCycles: 2}}
	loop.LLM = cancelLLMFunc(func(context.Context, config.EngineConfig, []llm.Message) (string, error) {
		models++
		return "model fact", nil
	})
	loop.Tools = cancelTools{
		plan: func(string) []ToolCall { return []ToolCall{{ID: "action"}} },
		execute: func(context.Context, []ToolCall) []ToolResult {
			executes++
			if executes == 2 {
				cancel()
			}
			return []ToolResult{{ID: "action", ModelLine: "executed fact", ProbeCost: 0.5}}
		},
	}
	result := loop.Run(ctx, goalInput("run-prior-batch"))
	if result.Outcome != OutcomeInterrupted || result.Cycles != 1 || result.ModelTurns != 2 || models != 2 || executes != 2 {
		t.Errorf("completed/cancelled batch accounting: %+v, model/execute=%d/%d", result, models, executes)
	}
	if len(exit.events) != 1 || exit.events[0].TurnID != "run-prior-batch:cycle:1" {
		t.Errorf("prior T1 must survive, no new T1/T2: %+v", exit.events)
	}
	if result.ProbeSpent != 1 || len(result.Conversation) != 5 {
		t.Errorf("both returned batches must survive: %+v", result)
	}
}
