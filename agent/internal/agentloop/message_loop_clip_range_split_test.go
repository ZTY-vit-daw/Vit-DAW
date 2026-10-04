package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

// clipRangeSplitTestExecutor wraps the shared fake executor with clip.split
// behaviour: the unconfirmed call proposes (needs_confirmation), the confirmed
// call executes. Every other tool delegates to the shared fake.
type clipRangeSplitTestExecutor struct {
	fake    *fakeMessageExecutor
	calls   []planner.ToolCall
	confirm []bool
}

func (e *clipRangeSplitTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	if strings.TrimSpace(in.ToolCall.Tool) == "clip.split" {
		e.calls = append(e.calls, in.ToolCall)
		e.confirm = append(e.confirm, in.Confirmed)
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "split_clip",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              "Split clip " + firstMapText(in.ToolCall.Args, "clip_id") + " at split_time.",
			}, nil
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "split_clip",
			Status:      "ok",
			Result:      map[string]any{"status": "ok", "clip_id": firstMapText(in.ToolCall.Args, "clip_id"), "split_time": in.ToolCall.Args["split_time"]},
		}, nil
	}
	return e.fake.RunToolCall(context.Background(), in)
}

func clipRangeSplitLoopConfig() config.EngineConfig {
	return config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"}
}

func boxedRangeSplitLoopInput() Input {
	return Input{
		Context: map[string]any{
			"selected_clip_id":       "clip_a",
			"selected_clip_track_id": "track_a",
			"selected_clip_ranges": []any{
				map[string]any{
					"range_id":                 "range_1",
					"clip_id":                  "clip_a",
					"track_id":                 "track_a",
					"start_seconds":            2.0,
					"end_seconds":              3.5,
					"duration_seconds":         1.5,
					"clip_start_seconds":       1.0,
					"clip_end_seconds":         5.0,
					"clip_local_start_seconds": 1.0,
					"clip_local_end_seconds":   2.5,
				},
			},
		},
		AllowedTools: []string{"clip.split"},
	}
}

func TestMessageLoopDeterministicClipRangeSplitCallsGate(t *testing.T) {
	state := &runState{input: boxedRangeSplitLoopInput()}
	state.input.UserText = "把这段拆出来"
	calls, ok := messageLoopDeterministicClipRangeSplitCalls(state)
	if !ok || len(calls) != 2 {
		t.Fatalf("expected the two-cut plan, ok=%t calls=%+v", ok, calls)
	}
	wantTimes := []string{"3.5", "2"}
	for i, call := range calls {
		if call.Tool != "clip.split" || firstMapText(call.Command, "cmd") != "split_clip" {
			t.Fatalf("call %d = %+v", i, call)
		}
		if got := firstMapText(call.Args, "split_time"); got != wantTimes[i] {
			t.Fatalf("call %d split_time = %v, want %v (end cut first)", i, got, wantTimes[i])
		}
		if firstMapText(call.Args, "clip_id") != "clip_a" || firstMapText(call.Args, "track_id") != "track_a" {
			t.Fatalf("call %d args = %+v", i, call.Args)
		}
	}

	// Boundary: no ranges, spoken seconds, playhead, and an in-flight pending
	// tool call must all keep the fast intent off (model path unchanged).
	for name, mutate := range map[string]func(*runState){
		"no ranges":            func(s *runState) { s.input.Context = map[string]any{} },
		"spoken seconds":       func(s *runState) { s.input.UserText = "在2.5秒切开这段" },
		"playhead":             func(s *runState) { s.input.UserText = "在播放头这里切开这段" },
		"no split verb":        func(s *runState) { s.input.UserText = "把这段放大" },
		"pending tool call":    func(s *runState) { s.pendingToolCall = &planner.ToolCall{ID: "p", Tool: "clip.split"} },
		"pending tool queue":   func(s *runState) { s.pendingToolQueue = []planner.ToolCall{{ID: "q", Tool: "clip.split"}} },
		"different clip text":  func(s *runState) { s.input.UserText = "切开这个片段" },
		"empty user text":      func(s *runState) { s.input.UserText = "" },
		"track add precedence": func(s *runState) { s.input.UserText = "新建轨道把框选范围拆开" },
	} {
		candidate := &runState{input: boxedRangeSplitLoopInput()}
		candidate.input.UserText = "把这段拆出来"
		mutate(candidate)
		if _, ok := messageLoopDeterministicClipRangeSplitCalls(candidate); ok {
			t.Fatalf("%s: fast intent must not engage", name)
		}
	}
}

// TestMessageLoopClipRangeSplitFastIntentProposesEndCutFirst pins the first
// turn: the fast intent proposes the end cut through the regular confirmation
// pause, and the start cut rides the continuation queue so the confirmed
// resume can run it — with zero model involvement.
func TestMessageLoopClipRangeSplitFastIntentProposesEndCutFirst(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model must not run for the boxed-range split plan"}`,
	}}
	exec := &clipRangeSplitTestExecutor{fake: &fakeMessageExecutor{}}
	loop := &MessageLoop{
		Client:   client,
		Config:   clipRangeSplitLoopConfig(),
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 8, MaxConsecutiveErrors: 2},
	}

	input := boxedRangeSplitLoopInput()
	input.UserText = "把这段拆出来"
	res := loop.Start(context.Background(), input)

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("fast intent must short-circuit the model, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.split" || firstMapText(exec.calls[0].Args, "split_time") != "3.5" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil {
		t.Fatalf("pending continuation with the end cut expected, got %+v", res.Continuation)
	}
	if firstMapText(res.Continuation.PendingToolCall.Args, "split_time") != "3.5" {
		t.Fatalf("pending call = %+v", res.Continuation.PendingToolCall)
	}
	if len(res.Continuation.PendingToolQueue) != 1 || firstMapText(res.Continuation.PendingToolQueue[0].Args, "split_time") != "2" {
		t.Fatalf("start cut must ride the continuation queue, got %+v", res.Continuation.PendingToolQueue)
	}
}

// TestMessageLoopClipRangeSplitResumesRunRemainingCutThenComplete walks the
// full confirmation chain: first resume executes the confirmed end cut and
// proposes the queued start cut; second resume executes the start cut and
// completes deterministically — the model never re-enters between cuts.
func TestMessageLoopClipRangeSplitResumesRunRemainingCutThenComplete(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model must not run"}`,
		`{"final":true,"reply":"model must not run"}`,
	}}
	exec := &clipRangeSplitTestExecutor{fake: &fakeMessageExecutor{}}
	loop := &MessageLoop{
		Client:   client,
		Config:   clipRangeSplitLoopConfig(),
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 8, MaxConsecutiveErrors: 2},
	}

	input := boxedRangeSplitLoopInput()
	input.UserText = "把这段拆出来"
	first := loop.Start(context.Background(), input)
	if first.Continuation == nil {
		t.Fatalf("first turn must pause with a continuation: %+v", first)
	}

	second := loop.ResumeAfterConfirmation(context.Background(), *first.Continuation)
	if second.Status != "waiting_confirmation" || second.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("second turn = status=%q stop=%q reply=%q", second.Status, second.StopReason, second.Reply)
	}
	if len(client.calls) != 0 {
		t.Fatalf("resume must not call the LLM, got %d calls", len(client.calls))
	}
	// calls[0] = end-cut proposal, calls[1] = confirmed end-cut execution,
	// calls[2] = start-cut proposal from the continuation queue.
	if len(exec.calls) != 3 || firstMapText(exec.calls[1].Args, "split_time") != "3.5" || !exec.confirm[1] ||
		firstMapText(exec.calls[2].Args, "split_time") != "2" || exec.confirm[2] {
		t.Fatalf("executor calls after first resume = %+v confirmed=%v", exec.calls, exec.confirm)
	}
	if second.Continuation == nil || second.Continuation.PendingToolCall == nil ||
		firstMapText(second.Continuation.PendingToolCall.Args, "split_time") != "2" {
		t.Fatalf("second pause must carry the start cut, got %+v", second.Continuation)
	}
	if len(second.Continuation.PendingToolQueue) != 0 {
		t.Fatalf("plan exhausted, queue must be empty: %+v", second.Continuation.PendingToolQueue)
	}

	third := loop.ResumeAfterConfirmation(context.Background(), *second.Continuation)
	if third.Status != "completed" || third.StopReason != StopReasonDone {
		t.Fatalf("third turn = status=%q stop=%q reply=%q error=%q", third.Status, third.StopReason, third.Reply, third.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("completion must not call the LLM, got %d calls", len(client.calls))
	}
	// calls[3] = confirmed start-cut execution in the third turn.
	if len(exec.calls) != 4 || firstMapText(exec.calls[3].Args, "split_time") != "2" || !exec.confirm[3] {
		t.Fatalf("executor calls = %+v confirmed=%v", exec.calls, exec.confirm)
	}
	for _, want := range []string{"框选范围拆分完成", "3.5", "2"} {
		if !strings.Contains(third.Reply, want) {
			t.Fatalf("completion reply missing %q: %s", want, third.Reply)
		}
	}
}

// TestMessageLoopClipRangeSplitWithoutRangesKeepsModelPath pins the zero-change
// boundary: the same phrase without selected_clip_ranges keeps the existing
// model-driven turn.
func TestMessageLoopClipRangeSplitWithoutRangesKeepsModelPath(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"plain reply without ranges"}`,
	}}
	exec := &clipRangeSplitTestExecutor{fake: &fakeMessageExecutor{}}
	loop := &MessageLoop{
		Client:   client,
		Config:   clipRangeSplitLoopConfig(),
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "把这段拆出来",
		Context:      map[string]any{"selected_clip_id": "clip_a"},
		AllowedTools: []string{"clip.split"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 1 {
		t.Fatalf("without ranges the model must drive the turn, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 0 {
		t.Fatalf("no split calls may execute without ranges: %+v", exec.calls)
	}
}
