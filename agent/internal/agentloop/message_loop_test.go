package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
)

type fakeMessageCompleter struct {
	responses []string
	calls     [][]llm.Message
}

func (f *fakeMessageCompleter) Complete(_ context.Context, _ config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), messages...))
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"done"}`, nil
	}
	out := f.responses[0]
	f.responses = f.responses[1:]
	return out, nil
}

type fakeMessageExecutor struct {
	calls []planner.ToolCall
}

func (f *fakeMessageExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	f.calls = append(f.calls, in.ToolCall)
	switch strings.TrimSpace(in.ToolCall.Tool) {
	case "track.add":
		return executorpkg.Result{
			ToolCallID:    in.ToolCall.ID,
			Tool:          in.ToolCall.Tool,
			CommandName:   "track.add",
			Status:        "ok",
			Result:        map[string]any{"track_id": "1007", "track_name": "Track 1"},
			ObservedState: map[string]any{"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1"}}},
		}, nil
	case "plugin.load_to_rack":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "plugin.load_to_rack",
			Status:      "ok",
			Result: map[string]any{
				"track_id":    in.ToolCall.Args["track_id"],
				"plugin_id":   "plugin_1",
				"plugin_name": "TDR Nova",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"rack": map[string]any{"nodes": []map[string]any{{
					"plugin_id":                       "plugin_1",
					"plugin_name":                     "TDR Nova",
					"track_id":                        "1007",
					"audio_reachable_from_rack_input": true,
					"vit_effective_in_output_path":    true,
				}}},
			}}},
		}, nil
	default:
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "error", Error: "unexpected tool"}, nil
	}
}

func TestMessageLoopExecutesDependentToolCallsWithoutIntermediateReplan(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"working","tool_calls":[{"id":"create_track","tool":"track.add","args":{}},{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_ref":"last_created_track","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"Track created and EQ loaded."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "新建一条轨道然后加载 TDR Nova 均衡器",
		AllowedTools: []string{"track.add", "plugin.load_to_rack"},
	})

	if res.Status != "completed" || res.Reply != "Track created and EQ loaded." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(exec.calls[1].Args["track_id"].(string)); got != "1007" {
		t.Fatalf("plugin target track_id = %q, want 1007; call=%+v", got, exec.calls[1])
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2 after both tools finish", len(client.calls))
	}
	secondPrompt := client.calls[1]
	toolResultCount := 0
	for _, msg := range secondPrompt {
		if strings.HasPrefix(strings.TrimSpace(msg.Content), "<tool_result>") {
			toolResultCount++
		}
	}
	if toolResultCount != 2 {
		t.Fatalf("second LLM prompt should include two tool results, got %d messages=%+v", toolResultCount, secondPrompt)
	}
}
