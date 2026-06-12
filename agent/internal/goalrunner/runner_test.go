package goalrunner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("VIT_CONTEXT_SNAPSHOT_PATH", "off")
	os.Exit(m.Run())
}

type fakePlanner struct {
	outputs []planner.Output
	inputs  []planner.Input
	err     error
}

func (f *fakePlanner) Next(_ context.Context, in planner.Input) (planner.Output, error) {
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return planner.Output{}, f.err
	}
	if len(f.outputs) == 0 {
		return planner.Output{Done: true, Reply: "done"}, nil
	}
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return out, nil
}

type fakeExecutor struct {
	results []executorpkg.Result
	inputs  []executorpkg.Input
}

func (f *fakeExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	f.inputs = append(f.inputs, in)
	if len(f.results) == 0 {
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{"ok": true}}, nil
	}
	out := f.results[0]
	f.results = f.results[1:]
	if out.ToolCallID == "" {
		out.ToolCallID = in.ToolCall.ID
	}
	if out.Tool == "" {
		out.Tool = in.ToolCall.Tool
	}
	return out, nil
}

func TestRunnerSingleStepCompletes(t *testing.T) {
	rt := agentruntime.New()
	fp := &fakePlanner{outputs: []planner.Output{{Done: true, Reply: "完成了"}}}
	runner := Runner{Runtime: rt, Planner: fp, Executor: &fakeExecutor{}}

	res := runner.Start(context.Background(), Input{UserText: "看一下工程"})
	if res.Status != agentruntime.StatusCompleted || res.StopReason != StopReasonDone || res.Reply != "完成了" {
		t.Fatalf("result = %+v", res)
	}
	if rt.Status(res.GoalID).Status != agentruntime.StatusCompleted {
		t.Fatalf("runtime status = %+v", rt.Status(res.GoalID))
	}
}

func TestRunnerMultiStepFeedsToolResultBackToPlanner(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{ToolCalls: []planner.ToolCall{{Tool: "workspace.grep", Args: map[string]any{"query": "rack"}, Reason: "search"}}},
		{Done: true, Reply: "找到结果"},
	}}
	fe := &fakeExecutor{}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{UserText: "搜索 rack", AllowedTools: []string{"workspace.grep"}})
	if res.Status != agentruntime.StatusCompleted || len(fe.inputs) != 1 {
		t.Fatalf("result=%+v executor calls=%d", res, len(fe.inputs))
	}
	if len(fp.inputs) != 2 || len(fp.inputs[1].Trace) == 0 {
		t.Fatalf("planner did not receive trace: %+v", fp.inputs)
	}
}

func TestRunnerIncludesConversationInPlannerSnapshot(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{{Done: true, Reply: "ok"}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: &fakeExecutor{}}

	res := runner.Start(context.Background(), Input{
		UserText: "把这两个插件都加载到当前轨道",
		Conversation: []llm.Message{
			{Role: "user", Content: "搜索一下当前插件库是否有均衡和混响？"},
			{Role: "assistant", Content: "TDR Nova 是均衡；ValhallaSupermassive 是混响。"},
		},
	})
	if res.Status != agentruntime.StatusCompleted {
		t.Fatalf("result = %+v", res)
	}
	if len(fp.inputs) != 1 {
		t.Fatalf("planner inputs = %+v", fp.inputs)
	}
	recent, ok := fp.inputs[0].ContextSnapshot["recent_turns"].([]any)
	if !ok || len(recent) != 2 {
		t.Fatalf("recent turns missing from snapshot: %+v", fp.inputs[0].ContextSnapshot["recent_turns"])
	}
	if !strings.Contains(traceAnyText(recent), "ValhallaSupermassive") {
		t.Fatalf("recent turns did not preserve prior plugin candidates: %+v", recent)
	}
}

func TestRunnerPausesAndResumesConfirmation(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{ToolCalls: []planner.ToolCall{{ID: "call_1", Tool: "midi.apply_note_patch", Reason: "write midi"}}},
		{Done: true, Reply: "MIDI 已写入"},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{
		{ToolCallID: "call_1", Tool: "midi.apply_note_patch", Status: "needs_confirmation", RequiresConfirmation: true, Preview: "preview midi"},
		{ToolCallID: "call_1", Tool: "midi.apply_note_patch", Status: "ok", Result: map[string]any{"written": true}},
	}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	paused := runner.Start(context.Background(), Input{UserText: "写 MIDI", AllowedTools: []string{"midi.apply_note_patch"}})
	if paused.Status != agentruntime.StatusWaitingConfirmation || paused.Continuation == nil || paused.Continuation.PendingToolCall == nil || paused.Preview != "preview midi" {
		t.Fatalf("paused = %+v", paused)
	}
	resumed := runner.ResumeAfterConfirmation(context.Background(), *paused.Continuation)
	if resumed.Status != agentruntime.StatusCompleted || resumed.Reply != "MIDI 已写入" {
		t.Fatalf("resumed = %+v", resumed)
	}
	if len(fe.inputs) != 2 || !fe.inputs[1].Confirmed {
		t.Fatalf("executor inputs = %+v", fe.inputs)
	}
}

func TestRunnerPausesForClarificationWithoutExecutingTools(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{{
		NeedsClarification:    true,
		ClarificationQuestion: "请选择要编辑的 clip。",
		PlanItems:             []planner.PlanItem{{ID: "choose_clip", Description: "Clarify target clip", Status: "pending"}},
	}}}
	fe := &fakeExecutor{}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{UserText: "把这个 MIDI 改得更律动"})
	if res.Status != agentruntime.StatusWaitingClarification || !res.NeedsClarification || res.ClarificationQuestion == "" || res.Continuation == nil {
		t.Fatalf("result = %+v", res)
	}
	if len(fe.inputs) != 0 {
		t.Fatalf("clarification should not execute tools, calls=%d", len(fe.inputs))
	}
	if res.ContextSnapshot["recent_goal_context"] == nil {
		t.Fatalf("context snapshot missing recent_goal_context: %+v", res.ContextSnapshot)
	}
}

func TestRunnerMutationBarrierReplansBeforeDependentWrite(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{
			PlanItems: []planner.PlanItem{
				{ID: "add_track", Description: "Create a track", Status: "pending"},
				{ID: "load_eq", Description: "Load EQ on the created track", Status: "pending"},
			},
			ToolCalls: []planner.ToolCall{
				{ID: "call_add", Tool: "track.add", Args: map[string]any{"name": "EQ Track"}, PlanItemID: "add_track"},
				{ID: "call_stale_eq", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "old_track", "plugin_path": `C:\TDR Nova.vst3`}, PlanItemID: "load_eq"},
			},
		},
		{
			ToolCalls: []planner.ToolCall{
				{ID: "call_load_eq", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track_new", "plugin_path": `C:\TDR Nova.vst3`}, PlanItemID: "load_eq"},
			},
		},
		{Done: true, Reply: "EQ loaded", Completion: &planner.Completion{SatisfiedPlanIDs: []string{"add_track", "load_eq"}}},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{
		{
			ToolCallID:  "call_add",
			Tool:        "track.add",
			CommandName: "add_track",
			Status:      "ok",
			Result:      map[string]any{"track_id": "track_new", "track_name": "EQ Track"},
			ObservedState: map[string]any{"tracks": []any{
				map[string]any{"track_id": "track_new", "track_name": "EQ Track"},
			}},
		},
		{
			ToolCallID:  "call_load_eq",
			Tool:        "plugin.load_to_rack",
			CommandName: "rack_add_node",
			Status:      "ok",
			Result:      map[string]any{"track_id": "track_new", "plugin_name": "TDR Nova"},
			ObservedState: map[string]any{"tracks": []any{
				map[string]any{"track_id": "track_new", "track_name": "EQ Track", "rack": map[string]any{"nodes": []any{
					map[string]any{"node_id": "node_eq", "name": "TDR Nova"},
				}}},
			}},
		},
	}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxTurns: 5, MaxToolCalls: 4}}

	res := runner.Start(context.Background(), Input{UserText: "create a track then load an EQ", AllowedTools: []string{"track.add", "plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusCompleted {
		t.Fatalf("result = %+v", res)
	}
	if len(fe.inputs) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(fe.inputs))
	}
	if fe.inputs[1].ToolCall.ID != "call_load_eq" || fe.inputs[1].ToolCall.Args["track_id"] != "track_new" {
		t.Fatalf("dependent write did not rebind to new track: %+v", fe.inputs)
	}
	if len(fp.inputs) < 2 {
		t.Fatalf("planner was not called after mutation barrier: %+v", fp.inputs)
	}
	recent, _ := fp.inputs[1].ContextSnapshot["recent_goal_context"].(map[string]any)
	memory, _ := recent["execution_memory"].(map[string]any)
	if memory["last_created_track_id"] != "track_new" || memory["active_work_target_track_id"] != "track_new" {
		t.Fatalf("planner did not receive execution memory: %+v", fp.inputs[1].ContextSnapshot)
	}
	if !hasTraceKind(res.Trace, "mutation_barrier") {
		t.Fatalf("trace missing mutation barrier: %+v", res.Trace)
	}
}

func TestRunnerDropsQueuedMutatingToolCallsAcrossConfirmation(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{{
		PlanItems: []planner.PlanItem{
			{ID: "load_eq", Description: "Load EQ", Status: "pending"},
		},
		ToolCalls: []planner.ToolCall{
			{ID: "call_eq", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track_1", "plugin_path": `C:\TDR Nova.vst3`}, PlanItemID: "load_eq"},
			{ID: "call_reverb", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track_1", "plugin_path": `C:\ValhallaSupermassive.vst3`}, PlanItemID: "load_reverb"},
			{ID: "call_analyzer", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track_1", "plugin_path": `C:\SPAN.vst3`}, PlanItemID: "load_analyzer"},
		},
	}}}
	fe := &fakeExecutor{results: []executorpkg.Result{
		{ToolCallID: "call_eq", Tool: "plugin.load_to_rack", CommandName: "rack_add_node", Status: "needs_confirmation", RequiresConfirmation: true, Preview: "Load EQ"},
		{
			ToolCallID:  "call_eq",
			Tool:        "plugin.load_to_rack",
			CommandName: "rack_add_node",
			Status:      "ok",
			Result:      map[string]any{"track_id": "track_1", "plugin_name": "TDR Nova"},
			ObservedState: map[string]any{"tracks": []any{map[string]any{"track_id": "track_1", "rack": map[string]any{"nodes": []any{
				map[string]any{"node_id": "node_eq", "name": "TDR Nova"},
			}}}}},
		},
		{ToolCallID: "call_reverb", Tool: "plugin.load_to_rack", CommandName: "rack_add_node", Status: "needs_confirmation", RequiresConfirmation: true, Preview: "Load Reverb"},
	}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	paused := runner.Start(context.Background(), Input{UserText: "加载均衡混响分析器", AllowedTools: []string{"plugin.load_to_rack"}})
	if paused.Status != agentruntime.StatusWaitingConfirmation || paused.Continuation == nil || len(paused.Continuation.PendingToolQueue) != 0 {
		t.Fatalf("first pause = %+v", paused)
	}
	next := runner.ResumeAfterConfirmation(context.Background(), *paused.Continuation)
	if next.Status != agentruntime.StatusCompleted || next.Continuation != nil {
		t.Fatalf("resumed result = %+v", next)
	}
	if len(fp.inputs) != 2 {
		t.Fatalf("runner should re-plan after the confirmed mutation, planner inputs=%d", len(fp.inputs))
	}
	if len(fe.inputs) != 2 || !fe.inputs[1].Confirmed {
		t.Fatalf("executor inputs = %+v", fe.inputs)
	}
	if !hasTraceKind(next.Trace, "mutation_barrier") {
		t.Fatalf("trace missing mutation barrier: %+v", next.Trace)
	}
}

func TestRunnerCancelStopsBeforeNextTool(t *testing.T) {
	rt := agentruntime.New()
	fp := &fakePlanner{outputs: []planner.Output{{ToolCalls: []planner.ToolCall{{Tool: "workspace.grep"}}}}}
	fe := &fakeExecutor{}
	goal := rt.Create("cancel me")
	rt.Cancel(goal.GoalID, "user")
	runner := Runner{Runtime: rt, Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{GoalID: goal.GoalID, RunID: goal.RunID, UserText: "run", AllowedTools: []string{"workspace.grep"}})
	if res.Status != agentruntime.StatusCancelled || len(fe.inputs) != 0 {
		t.Fatalf("result=%+v executor calls=%d", res, len(fe.inputs))
	}
}

func TestRunnerBudgetLimitWaitsForContinue(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{ToolCalls: []planner.ToolCall{{Tool: "workspace.grep"}}},
		{ToolCalls: []planner.ToolCall{{Tool: "workspace.read_file"}}},
	}}
	fe := &fakeExecutor{}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxTurns: 6, MaxToolCalls: 1}}

	res := runner.Start(context.Background(), Input{UserText: "multi", AllowedTools: []string{"workspace.grep", "workspace.read_file"}})
	if res.Status != agentruntime.StatusWaitingContinue || res.StopReason != StopReasonLimitReached || res.LimitType != LimitTypeToolCalls || res.Continuation == nil {
		t.Fatalf("result = %+v", res)
	}
	if res.Continuation.ContextSnapshot["schema_version"] != "vit_context_snapshot.v1" {
		t.Fatalf("continuation missing context snapshot: %+v", res.Continuation.ContextSnapshot)
	}
	if !strings.Contains(res.Reply, "继续") {
		t.Fatalf("reply should mention continue: %q", res.Reply)
	}
}

func TestRunnerContinueKeepsGoalAndCreatesNewRun(t *testing.T) {
	rt := agentruntime.New()
	fp := &fakePlanner{outputs: []planner.Output{{Done: true, Reply: "继续完成"}}}
	runner := Runner{Runtime: rt, Planner: fp, Executor: &fakeExecutor{}, Budget: Budget{MaxTurns: 1, MaxToolCalls: 1}}
	goal := rt.Create("long task")
	cont := Continuation{GoalID: goal.GoalID, RunID: goal.RunID, UserText: "long task", Summary: "long task", ContextSnapshot: map[string]any{"schema_version": "vit_context_snapshot.v1", "created_at": "old"}, Budget: Budget{MaxTurns: 1, MaxToolCalls: 1}}

	res := runner.Continue(context.Background(), cont)
	if res.GoalID != goal.GoalID || res.RunID == goal.RunID || res.Status != agentruntime.StatusCompleted {
		t.Fatalf("result = %+v original run=%s", res, goal.RunID)
	}
	if len(fp.inputs) == 0 || fp.inputs[0].ContextSnapshot["schema_version"] != "vit_context_snapshot.v1" {
		t.Fatalf("planner missing context snapshot: %+v", fp.inputs)
	}
	if summary, _ := fp.inputs[0].ContextSnapshot["goal_trace_summary"].(map[string]any); summary["inherited_snapshot"] == nil {
		t.Fatalf("planner snapshot should inherit continuation snapshot: %+v", fp.inputs[0].ContextSnapshot)
	}
}

func TestRunnerRejectsUnknownToolAsToolResult(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{ToolCalls: []planner.ToolCall{{Tool: "not.real"}}},
		{Done: true, Reply: "handled"},
	}}
	fe := &fakeExecutor{}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxConsecutiveErrors: 3}}

	res := runner.Start(context.Background(), Input{UserText: "bad", AllowedTools: []string{"workspace.grep"}})
	if res.Status != agentruntime.StatusCompleted || len(fe.inputs) != 0 {
		t.Fatalf("result=%+v executor calls=%d", res, len(fe.inputs))
	}
	if len(fp.inputs) < 2 || !strings.Contains(traceText(fp.inputs[1].Trace), "未知或不允许的工具") {
		t.Fatalf("planner trace missing unknown tool error: %+v", fp.inputs)
	}
}

func TestRunnerVerifiesRackLoadBeforeCompletion(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{
			PlanItems: []planner.PlanItem{{ID: "load_eq", Description: "Load an EQ on the current track", Status: "pending"}},
			ToolCalls: []planner.ToolCall{{
				ID:         "call_load_eq",
				Tool:       "plugin.load_to_rack",
				Args:       map[string]any{"track_id": "track_1", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
				PlanItemID: "load_eq",
				Reason:     "load EQ",
			}},
		},
		{Done: true, Reply: "均衡已加载。", Completion: &planner.Completion{SatisfiedPlanIDs: []string{"load_eq"}, Evidence: []string{"rack node observed"}}},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{
		{
			ToolCallID:  "call_load_eq",
			Tool:        "plugin.load_to_rack",
			CommandName: "rack_add_node",
			Status:      "ok",
			Result:      map[string]any{"track_id": "track_1", "plugin_name": "TDR Nova"},
			ObservedState: map[string]any{"tracks": []any{
				map[string]any{
					"track_id":   "track_1",
					"track_name": "Track 1",
					"rack": map[string]any{"nodes": []any{
						map[string]any{"node_id": "node_nova", "name": "TDR Nova"},
					}},
				},
			}},
		},
	}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{UserText: "加载均衡", AllowedTools: []string{"plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusCompleted || res.Reply != "均衡已加载。" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.PlanItems) != 1 || res.PlanItems[0].Status != "completed" {
		t.Fatalf("plan items = %+v", res.PlanItems)
	}
	if !hasVerificationStatus(res.Trace, "verified") {
		t.Fatalf("trace missing verified postcondition: %+v", res.Trace)
	}
	if len(fp.inputs) < 2 || len(fp.inputs[1].PlanItems) != 1 || fp.inputs[1].PlanItems[0].Status != "completed" {
		t.Fatalf("planner did not receive completed plan item: %+v", fp.inputs)
	}
}

func TestRunnerTreatsOrphanedZ3RackLoadAsUnverified(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{
			PlanItems: []planner.PlanItem{{ID: "load_eq", Description: "Load an EQ on the current track", Status: "pending"}},
			ToolCalls: []planner.ToolCall{{
				ID:         "call_load_eq",
				Tool:       "plugin.load_to_rack",
				Args:       map[string]any{"track_id": "track_1", "plugin_path": `C:\Program Files\Common Files\VST3\TDR Nova.vst3`},
				PlanItemID: "load_eq",
			}},
		},
		{Done: true, Reply: "均衡已加载。"},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{{
		ToolCallID:  "call_load_eq",
		Tool:        "plugin.load_to_rack",
		CommandName: "rack_add_node",
		Status:      "ok",
		Result:      map[string]any{"track_id": "track_1", "plugin_name": "TDR Nova"},
		ObservedState: map[string]any{"tracks": []any{
			map[string]any{
				"track_id": "track_1",
				"rack": map[string]any{"nodes": []any{
					map[string]any{
						"node_id":                         "node_nova",
						"name":                            "TDR Nova",
						"zone_id":                         "Z3",
						"audio_reachable_from_rack_input": false,
						"vit_effective_in_output_path":    false,
						"vit_orphan_bypass_candidate":     true,
					},
				}},
			},
		}},
	}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxTurns: 2, MaxToolCalls: 2}}

	res := runner.Start(context.Background(), Input{UserText: "加载均衡", AllowedTools: []string{"plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusWaitingContinue || res.StopReason != StopReasonLimitReached {
		t.Fatalf("orphaned rack load should not complete, got %+v", res)
	}
	if !hasVerificationStatus(res.Trace, "unverified") || !hasTraceKind(res.Trace, "final_gate") {
		t.Fatalf("trace missing unverified/final_gate: %+v", res.Trace)
	}
	if len(res.PlanItems) != 1 || res.PlanItems[0].Status != "pending" {
		t.Fatalf("orphaned rack load should keep plan pending: %+v", res.PlanItems)
	}
}

func TestRunnerVerifiesTrackAddBeforeCompletion(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{
			PlanItems: []planner.PlanItem{{ID: "add_track", Description: "Create a user track", Status: "pending"}},
			ToolCalls: []planner.ToolCall{{
				ID:         "call_add_track",
				Tool:       "track.add",
				Args:       map[string]any{"name": "Track 1"},
				PlanItemID: "add_track",
			}},
		},
		{Done: true, Reply: "轨道已创建。", Completion: &planner.Completion{SatisfiedPlanIDs: []string{"add_track"}}},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{{
		ToolCallID:  "call_add_track",
		Tool:        "track.add",
		CommandName: "add_track",
		Status:      "ok",
		Result:      map[string]any{"track_id": "track_1", "track_name": "Track 1"},
		ObservedState: map[string]any{"tracks": []any{
			map[string]any{"track_id": "track_1", "track_name": "Track 1"},
		}},
	}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{UserText: "新建一条轨道", AllowedTools: []string{"track.add"}})
	if res.Status != agentruntime.StatusCompleted {
		t.Fatalf("result = %+v", res)
	}
	if len(res.PlanItems) != 1 || res.PlanItems[0].Status != "completed" || !hasVerificationStatus(res.Trace, "verified") {
		t.Fatalf("track add was not verified: plan=%+v trace=%+v", res.PlanItems, res.Trace)
	}
}

func TestRunnerFinalGateBlocksDoneWithPendingPlanItem(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{Done: true, Reply: "已经加载。", PlanItems: []planner.PlanItem{{ID: "load_eq", Description: "Load EQ", Status: "pending"}}},
		{ToolCalls: []planner.ToolCall{{ID: "call_load_eq", Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track_1", "plugin_path": `C:\TDR Nova.vst3`}, PlanItemID: "load_eq"}}},
		{Done: true, Reply: "现在已加载。", Completion: &planner.Completion{SatisfiedPlanIDs: []string{"load_eq"}}},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{{
		ToolCallID:  "call_load_eq",
		Tool:        "plugin.load_to_rack",
		CommandName: "rack_add_node",
		Status:      "ok",
		Result:      map[string]any{"track_id": "track_1", "plugin_name": "TDR Nova"},
		ObservedState: map[string]any{"tracks": []any{map[string]any{"track_id": "track_1", "rack": map[string]any{"nodes": []any{
			map[string]any{"node_id": "node_nova", "name": "TDR Nova"},
		}}}}},
	}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxTurns: 4, MaxToolCalls: 3}}

	res := runner.Start(context.Background(), Input{UserText: "加载均衡", AllowedTools: []string{"plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusCompleted || res.Reply != "现在已加载。" {
		t.Fatalf("result = %+v", res)
	}
	if !hasTraceKind(res.Trace, "final_gate") {
		t.Fatalf("trace missing final gate block: %+v", res.Trace)
	}
	if len(fe.inputs) != 1 {
		t.Fatalf("executor should run after final gate blocks premature completion, calls=%d", len(fe.inputs))
	}
}

func TestRunnerUnverifiedRackLoadCannotCompletePlan(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{
		{
			PlanItems: []planner.PlanItem{{ID: "load_reverb", Description: "Load reverb", Status: "pending"}},
			ToolCalls: []planner.ToolCall{{
				ID:         "call_load_reverb",
				Tool:       "plugin.load_to_rack",
				Args:       map[string]any{"track_id": "track_1", "plugin_path": `C:\ValhallaSupermassive.vst3`},
				PlanItemID: "load_reverb",
			}},
		},
		{Done: true, Reply: "混响已加载。"},
	}}
	fe := &fakeExecutor{results: []executorpkg.Result{{
		ToolCallID:    "call_load_reverb",
		Tool:          "plugin.load_to_rack",
		CommandName:   "rack_add_node",
		Status:        "ok",
		Result:        map[string]any{"track_id": "track_1", "plugin_name": "ValhallaSupermassive"},
		ObservedState: map[string]any{"tracks": []any{map[string]any{"track_id": "track_1", "rack": map[string]any{"nodes": []any{}}}}},
	}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe, Budget: Budget{MaxTurns: 2, MaxToolCalls: 2}}

	res := runner.Start(context.Background(), Input{UserText: "加载混响", AllowedTools: []string{"plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusWaitingContinue || res.StopReason != StopReasonLimitReached || res.LimitType != LimitTypeTurns {
		t.Fatalf("unverified plan should wait for continuation, got %+v", res)
	}
	if !hasVerificationStatus(res.Trace, "unverified") || !hasTraceKind(res.Trace, "final_gate") {
		t.Fatalf("trace missing unverified/final_gate: %+v", res.Trace)
	}
	if len(res.PlanItems) != 1 || res.PlanItems[0].Status != "pending" {
		t.Fatalf("plan should remain pending after unverified load: %+v", res.PlanItems)
	}
}

func TestRunnerConfirmationKeepsPlanItemWaiting(t *testing.T) {
	fp := &fakePlanner{outputs: []planner.Output{{
		PlanItems: []planner.PlanItem{{ID: "load_reverb", Description: "Load reverb", Status: "pending"}},
		ToolCalls: []planner.ToolCall{{
			ID:         "call_load_reverb",
			Tool:       "plugin.load_to_rack",
			Args:       map[string]any{"track_id": "track_1", "plugin_path": `C:\ValhallaSupermassive.vst3`},
			PlanItemID: "load_reverb",
		}},
	}}}
	fe := &fakeExecutor{results: []executorpkg.Result{{
		ToolCallID:           "call_load_reverb",
		Tool:                 "plugin.load_to_rack",
		CommandName:          "rack_add_node",
		Status:               "needs_confirmation",
		RequiresConfirmation: true,
		Preview:              "Load a plugin as a graph rack node.",
	}}}
	runner := Runner{Runtime: agentruntime.New(), Planner: fp, Executor: fe}

	res := runner.Start(context.Background(), Input{UserText: "加载混响", AllowedTools: []string{"plugin.load_to_rack"}})
	if res.Status != agentruntime.StatusWaitingConfirmation || res.Continuation == nil {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Continuation.PlanItems) != 1 || res.Continuation.PlanItems[0].Status != "waiting_confirmation" {
		t.Fatalf("continuation plan items = %+v", res.Continuation.PlanItems)
	}
	if !hasVerificationStatus(res.Trace, "not_executed_pending_confirmation") {
		t.Fatalf("trace missing pending confirmation verification: %+v", res.Trace)
	}
}

func traceText(trace []planner.TraceEvent) string {
	var parts []string
	for _, ev := range trace {
		parts = append(parts, ev.Message, ev.Reply)
		if ev.ToolResult != nil {
			parts = append(parts, ev.ToolResult.Error, ev.ToolResult.Status, ev.ToolResult.Tool)
		}
	}
	return strings.Join(parts, "\n")
}

func hasTraceKind(trace []planner.TraceEvent, kind string) bool {
	for _, ev := range trace {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

func hasVerificationStatus(trace []planner.TraceEvent, status string) bool {
	for _, ev := range trace {
		if ev.Verification != nil && ev.Verification.Status == status {
			return true
		}
	}
	return false
}

func traceAnyText(value any) string {
	return fmt.Sprintf("%+v", value)
}
