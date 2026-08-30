package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
)

type productionFreeStateCompleter struct {
	responses []string
}

func (f *productionFreeStateCompleter) Complete(_ context.Context, _ config.EngineConfig, _ []llm.Message) (string, error) {
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"proposal","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"bounded gain proposal","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"1007"},"evidence_refs":["obs-production-3"],"improvement_intent":"balance bass","hypothesis":"a small gain change may improve balance","expected_effect":"clearer balance","action_domain":"track_gain","action_kind":"track_gain_adjust","parameter_bounds":{"delta_db":-0.5},"verification_plan":{"experiment_budget":1,"max_action_attempts":1}}},"tool_calls":[]}`, nil
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

type productionFreeStateCCBExecutor struct {
	calls []planner.ToolCall
	seq   int
}

func (e *productionFreeStateCCBExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall)
	e.seq++
	if in.ToolCall.Tool == "ccb.observation_catalog" {
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{
			"status": "ok", "catalog": map[string]any{"schema_version": "ccb_observation_catalog.v1", "views": []any{
				map[string]any{"view_id": "project.structure"}, map[string]any{"view_id": "mix.multitrack_relationship"}, map[string]any{"view_id": "track.band_dynamics"},
			}},
		}}, nil
	}
	views := in.ToolCall.Args["view_ids"]
	observationID := fmt.Sprintf("obs-production-%d", e.seq)
	receiptID := fmt.Sprintf("receipt-production-%d", e.seq)
	target := map[string]any{"kind": "project", "id": "current", "label": "Current project"}
	if targetRef, ok := in.ToolCall.Args["target_ref"].(map[string]any); ok && len(targetRef) > 0 {
		target = targetRef
	}
	return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{
		"status": "ok", "bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready", "read_only": true, "mutation_authority": false,
			"observation_id": observationID, "requested_views": views, "target_ref": target,
			"views": map[string]any{"project.structure": map[string]any{"status": "ready"}, "mix.multitrack_relationship": map[string]any{"status": "ready"}, "track.band_dynamics": map[string]any{"status": "ready"}},
			"audit_receipt": map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": receiptID, "status": "ready"},
		},
	}}, nil
}

func TestProductionFreeStateRunnerObservationsSurviveDurableSlices(t *testing.T) {
	model := &productionFreeStateCompleter{responses: []string{
		`{"final":false,"reply":"catalog","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"discover catalog","requested_view_ids":[]},"tool_calls":[{"tool":"ccb.observation_catalog","args":{}}]}`,
		`{"final":false,"reply":"project","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"project evidence","requested_view_ids":["project.structure","mix.multitrack_relationship"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["project.structure","mix.multitrack_relationship"]}}]}`,
		`{"final":false,"reply":"target","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"plausible","summary":"target evidence","requested_view_ids":["track.band_dynamics"],"priority_reason":"target_selection"},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["track.band_dynamics"],"target_ref":{"kind":"track","id":"1007","label":"bass"}}}]}`,
		`{"final":true,"reply":"proposal","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"bounded gain proposal","improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"1007"},"evidence_refs":["obs-production-3"],"improvement_intent":"balance bass","hypothesis":"a small gain change may improve balance","expected_effect":"clearer balance","action_domain":"track_gain","action_kind":"track_gain_adjust","parameter_bounds":{"delta_db":-0.5},"verification_plan":{"experiment_budget":1,"max_action_attempts":1}}},"tool_calls":[]}`,
	}}
	executor := &productionFreeStateCCBExecutor{}
	runner := agentloop.MessageLoop{Runtime: agentruntime.New(), Client: model, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "fake", APIKey: "fake"}, Executor: executor, Budget: agentloop.Budget{MaxTurns: 1, MaxToolCalls: 1, MaxConsecutiveErrors: 1}}
	s := testContinuationServer()
	defer s.Close()
	s.freeStateLoops = map[string]freeStateReasoningLoop{}
	conversationID := "production-cross-slice"
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "production-loop", ConversationID: conversationID, GoalID: "goal-production", RunID: "run-production", Status: "reasoning", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", CurrentPhase: "fs2_capacity_assessed", MaxCycles: 6, ContinuationBudget: 12, CreatedAt: now, UpdatedAt: now})
	input := agentloop.Input{GoalID: "goal-production", RunID: "run-production", UserText: "inspect the project", Summary: "inspect the project", AllowedTools: []string{"ccb.observation_catalog", "ccb.observation_request"}, Context: map[string]any{"free_state_reasoning_loop": freeStateLoopMap(mustFreeStateLoop(s, conversationID))}}
	result := runner.Start(context.Background(), input)
	var childResult agentloop.Result
	childAlreadyRecorded := false
	for slice := 1; ; slice++ {
		if !childAlreadyRecorded {
			if _, ok := s.recordFreeStateDecision(conversationID, result); !ok {
				t.Fatalf("slice %d did not record runner result", slice)
			}
			if err := s.recordGoalResult(conversationID, result); err != nil {
				t.Fatalf("slice %d checkpoint failed: %v", slice, err)
			}
		}
		childAlreadyRecorded = false
		loop, _ := s.freeStateLoop(conversationID)
		expectedEvidence := slice - 1 // catalog discovery is persisted separately, not acoustic evidence
		if slice <= 3 {
			if len(loop.ObservationIDs) != expectedEvidence || len(loop.ObservationReceipts) != expectedEvidence || len(freeStateMapRows(loop.ObservationLedger["receipts"])) != expectedEvidence || (expectedEvidence > 0 && loop.LatestObservation == nil) {
				t.Fatalf("slice %d observation projection inconsistent: result_status=%s stop=%s executed=%v recent=%v ids=%v receipts=%d ledger=%d latest=%v", slice, result.Status, result.StopReason, result.Executed, result.RecentObservation, loop.ObservationIDs, len(loop.ObservationReceipts), len(freeStateMapRows(loop.ObservationLedger["receipts"])), loop.LatestObservation)
			}
		}
		if result.FreeStateDecision != nil && result.FreeStateDecision.Status == agentloop.FreeStateNeedsExperiment {
			if loop.LatestDecision == nil || loop.LatestDecision.ImprovementProposal == nil {
				t.Fatalf("proposal was not durable: phase=%q decision=%#v", loop.CurrentPhase, loop.LatestDecision)
			}
			break
		}
		var pending DurableContinuation
		for _, item := range s.durableContinuations {
			if item.Status == ContinuationPending || item.Status == ContinuationRunning || item.Status == ContinuationClaimed {
				pending = item
			}
		}
		if pending.ContinuationID == "" {
			if loop, ok := s.freeStateLoop(conversationID); ok && loop.CurrentPhase == "fs7_improvement_proposal" && loop.LatestDecision != nil && loop.LatestDecision.ImprovementProposal != nil {
				break
			}
			statuses := []string{}
			for _, item := range s.durableContinuations { statuses = append(statuses, string(item.Status)+":"+item.ContinuationID) }
			t.Fatalf("slice %d did not create child checkpoint: statuses=%v result_cont=%v", slice, statuses, result.Continuation != nil)
		}
		s.continuationExecutor = func(ctx context.Context, item DurableContinuation) error {
			child := runner.Continue(ctx, item.Continuation)
			childResult = child
			if _, ok := s.recordFreeStateDecision(conversationID, child); !ok {
				return fmt.Errorf("child result was not recorded")
			}
			childAlreadyRecorded = true
			return s.recordGoalResult(conversationID, child)
		}
		claimed, ok := s.claimNextContinuation(time.Now().UTC().Add(time.Minute))
		if !ok {
			t.Fatalf("slice %d child checkpoint was not claimable", slice)
		}
		if err := s.continuationExecutor(context.Background(), claimed); err != nil {
			t.Fatalf("slice %d durable execution failed: %v", slice, err)
		}
		result = childResult
		if slice >= 4 {
			loop, _ := s.freeStateLoop(conversationID)
			if len(loop.ObservationIDs) >= 2 && len(loop.ObservationReceipts) >= 2 && loop.LatestObservation != nil {
				break
			}
			t.Fatalf("production runner lost target evidence: phase=%s status=%s decision=%#v ids=%v calls=%d", loop.CurrentPhase, loop.Status, loop.LatestDecision, loop.ObservationIDs, len(executor.calls))
		}
	}
	projectCalls := 0
	for _, call := range executor.calls {
		if call.Tool == "ccb.observation_request" && len(call.Args["view_ids"].([]any)) == 2 {
			projectCalls++
		}
	}
	if projectCalls != 1 { t.Fatalf("duplicate project view set entered CCB executor: %d", projectCalls) }
}

func mustFreeStateLoop(s *Server, conversationID string) freeStateReasoningLoop {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		panic("missing free-state loop")
	}
	return loop
}

type continuationTestPlanner struct {
	outputs []planner.Output
}

func (p *continuationTestPlanner) Next(_ context.Context, _ planner.Input) (planner.Output, error) {
	if len(p.outputs) == 0 {
		return planner.Output{Done: true, Reply: "done"}, nil
	}
	out := p.outputs[0]
	p.outputs = p.outputs[1:]
	return out, nil
}

type continuationTestExecutor struct{}

func (continuationTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok", Result: map[string]any{"observed": true}}, nil
}

func testContinuationServer() *Server {
	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	s := &Server{
		goalContinuations:    map[string]agentloop.Continuation{},
		durableContinuations: map[string]DurableContinuation{},
		conversationGoals:    map[string]string{},
		conversationMemory:   map[string]agentloop.ExecutionMemory{},
		schedulerOwner:       "test-worker",
		continuationLease:    time.Minute,
		schedulerWake:        make(chan struct{}, 1),
		schedulerCtx:         schedulerCtx,
		schedulerCancel:      schedulerCancel,
	}
	return s
}

func waitingContinuationResult(slice, turn string, status agentruntime.GoalStatus, reason string) agentloop.Result {
	return agentloop.Result{
		GoalID: "goal-c", RunID: "run-c", TaskID: "task-c", SliceID: slice, TurnID: turn,
		OriginalIntent: "inspect the project", Status: status, StopReason: reason,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-c", RunID: "run-c", TaskID: "task-c", SliceID: slice, TurnID: turn,
			OriginalIntent: "inspect the project", UserText: "inspect the project", Summary: "inspect the project",
			Context: map[string]any{"free_state_reasoning_loop": map[string]any{"status": "reasoning"}},
		},
	}
}

func TestRecordGoalResultEnqueuesDurableContinuationIdempotently(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-1", "turn-1", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	s.recordGoalResult("conversation-c", res)
	if len(s.durableContinuations) != 1 {
		t.Fatalf("expected one durable continuation, got %d", len(s.durableContinuations))
	}
	var first DurableContinuation
	for _, item := range s.durableContinuations {
		first = item
	}
	if first.Status != ContinuationPending || first.OriginalIntent != "inspect the project" {
		t.Fatalf("unexpected durable state: %+v", first)
	}
	s.recordGoalResult("conversation-c", res)
	if len(s.durableContinuations) != 1 {
		t.Fatalf("duplicate checkpoint created another queue item: %+v", s.durableContinuations)
	}
}

func TestWaitingContinueWithoutContinuationFailsClosed(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	resp := s.chatResponseFromAgentLoopResult("conversation-missing-checkpoint", agentModeDefault, agentloop.Result{
		GoalID: "goal-missing-checkpoint", RunID: "run-missing-checkpoint", TaskID: "task-missing-checkpoint",
		SliceID: "slice-missing-checkpoint", TurnID: "turn-missing-checkpoint", Status: agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached, LimitType: "max_turns",
	})
	if resp.GoalStatus != string(agentruntime.StatusFailed) || resp.StopReason != "durable_continuation_missing" {
		t.Fatalf("waiting_continue without checkpoint was not failed closed: %+v", resp)
	}
}

func TestFreeStateObservationsSurviveFourDurableSlices(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	s.freeStateLoops = map[string]freeStateReasoningLoop{}
	conversationID := "cross-slice-free-state"
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-cross-slice", ConversationID: conversationID,
		GoalID: "goal-cross-slice", RunID: "run-cross-slice", Status: "observing", OriginalIntent: "inspect the project",
		ActiveIntent: "inspect the project", CurrentPhase: "fs2_capacity_assessed", MaxCycles: 6,
		ContinuationBudget: 4, CreatedAt: now, UpdatedAt: now,
	})
	phases := []string{"fs3_project_scan", "fs4_diagnostic_round", "fs5_candidate_frontier", "fs7_improvement_proposal"}
	views := []string{"project.structure", "mix.multitrack_relationship", "track.basic_energy", "track.band_dynamics"}
	for index, phase := range phases {
		before, ok := s.freeStateLoop(conversationID)
		if !ok {
			t.Fatalf("slice %d lost server loop", index+1)
		}
		observationID := fmt.Sprintf("obs-cross-slice-%d", index+1)
		receiptID := fmt.Sprintf("receipt-cross-slice-%d", index+1)
		updated := cloneFreeStateLoop(before)
		updated.CurrentPhase = phase
		updated.ObservationIDs = appendUniqueFreeStateStrings(updated.ObservationIDs, []string{observationID})
		updated.ObservationReceipts = append(updated.ObservationReceipts, map[string]any{"receipt_id": receiptID, "observation_id": observationID})
		updated.ObservationLedger = mergeFreeStateLedgers(updated.ObservationLedger, map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"receipts": []map[string]any{{"receipt_id": receiptID, "observation_id": observationID, "requested_views": []string{views[index]}}},
			"available_views": map[string]any{views[index]: map[string]any{"view_id": views[index], "observation_id": observationID, "status": "ready"}},
		},)
		updated.LatestObservation = &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
			"observation_id": observationID, "requested_views": []any{views[index]}, "audit_receipt": map[string]any{"receipt_id": receiptID},
		}}
		updated.UpdatedAt = now.Add(time.Duration(index+1) * time.Second)
		status := agentloop.FreeStateNeedsObservation
		if index == len(phases)-1 {
			status = agentloop.FreeStateNeedsExperiment
		}
		res := agentloop.Result{
			GoalID: "goal-cross-slice", RunID: "run-cross-slice", TaskID: "task-cross-slice",
			SliceID: fmt.Sprintf("slice-%d", index+1), TurnID: fmt.Sprintf("turn-%d", index+1),
			Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
			ContextSnapshot: map[string]any{"free_state_reasoning_loop": freeStateLoopMap(updated)},
			Continuation: &agentloop.Continuation{GoalID: "goal-cross-slice", RunID: "run-cross-slice", TaskID: "task-cross-slice",
				SliceID: fmt.Sprintf("slice-%d", index+1), TurnID: fmt.Sprintf("turn-%d", index+1),
				Context: map[string]any{"free_state_reasoning_loop": freeStateLoopMap(before)}},
			FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: status,
				EvidenceStatus: "plausible", Summary: "cross-slice evidence", RequestedViewIDs: []string{views[index]}},
		}
		if _, ok := s.recordFreeStateDecision(conversationID, res); !ok {
			t.Fatalf("slice %d did not record free-state result", index+1)
		}
		if err := s.recordGoalResult(conversationID, res); err != nil {
			t.Fatalf("slice %d checkpoint failed: %v", index+1, err)
		}
		var checkpoint DurableContinuation
		for _, item := range s.durableContinuations {
			if item.ConversationID == conversationID && item.Status == ContinuationPending {
				checkpoint = item
			}
		}
		if checkpoint.ContinuationID == "" {
			t.Fatalf("slice %d did not create a pending child checkpoint", index+1)
		}
		child, ok := freeStateLoopFromAny(checkpoint.Continuation.Context["free_state_reasoning_loop"])
		if !ok || len(child.ObservationIDs) != index+1 || len(child.ObservationReceipts) != index+1 || child.CurrentPhase != phase {
			t.Fatalf("slice %d child checkpoint lost durable evidence: phase=%q ids=%v receipts=%d", index+1, child.CurrentPhase, child.ObservationIDs, len(child.ObservationReceipts))
		}
		encoded, err := json.Marshal(checkpoint)
		if err != nil {
			t.Fatal(err)
		}
		var reloaded DurableContinuation
		if err := json.Unmarshal(encoded, &reloaded); err != nil {
			t.Fatal(err)
		}
		reloadedLoop, ok := freeStateLoopFromAny(reloaded.Continuation.Context["free_state_reasoning_loop"])
		if !ok || len(reloadedLoop.ObservationIDs) != index+1 {
			t.Fatalf("slice %d restart round-trip lost evidence: %#v", index+1, reloadedLoop)
		}
		merged := mergeFreeStateLoops(reloadedLoop, before, true)
		if merged.CurrentPhase != phase || len(merged.ObservationIDs) != index+1 || len(merged.ObservationReceipts) != index+1 {
			t.Fatalf("slice %d stale transport overlay regressed durable state: phase=%q ids=%v receipts=%d", index+1, merged.CurrentPhase, merged.ObservationIDs, len(merged.ObservationReceipts))
		}
		s.continuationExecutor = func(_ context.Context, item DurableContinuation) error {
			if item.Continuation.Context["free_state_reasoning_loop"] == nil {
				t.Fatalf("slice %d execute received no free-state loop", index+1)
			}
			return nil
		}
		if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
			t.Fatalf("slice %d durable claim/execute failed: %v", index+1, err)
		}
	}
}

func TestDurableContinuationCheckpointIsDeeplyIsolated(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-isolated", "turn-isolated", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	res.Continuation.PendingToolCall = &planner.ToolCall{ID: "pending-tool", Tool: "project.state", Args: map[string]any{"scope": "project"}}
	res.Continuation.ExecutionMemory.PendingTrackOrganization = map[string]any{"coverage": "partial"}
	s.recordGoalResult("conversation-c", res)
	res.Continuation.PendingToolCall.Args["scope"] = "mutated"
	res.Continuation.ExecutionMemory.PendingTrackOrganization["coverage"] = "mutated"
	for _, item := range s.durableContinuations {
		if item.Continuation.PendingToolCall.Args["scope"] != "project" || item.Continuation.ExecutionMemory.PendingTrackOrganization["coverage"] != "partial" {
			t.Fatalf("durable checkpoint was aliased to the live result: %+v", item.Continuation)
		}
	}
}

func TestContinuationSchedulerKeepsTaskRunAndCompletesClaimedSlice(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-1", "turn-1", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	s.recordGoalResult("conversation-c", res)
	var calls int
	s.continuationExecutor = func(_ context.Context, item DurableContinuation) error {
		calls++
		if item.TaskID != "task-c" || item.GoalID != "goal-c" || item.RunID != "run-c" {
			t.Fatalf("identity changed during dispatch: %+v", item)
		}
		if item.OriginalIntent != "inspect the project" {
			t.Fatalf("original intent changed: %q", item.OriginalIntent)
		}
		return nil
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one dispatch, got %d", calls)
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationCompleted || item.Attempt != 1 {
			t.Fatalf("claimed item was not completed: %+v", item)
		}
	}
	// A second drive cannot dispatch the completed record.
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("completed continuation dispatched twice: %d", calls)
	}
	// Re-delivery after completion must not reopen the checkpoint.
	s.recordGoalResult("conversation-c", res)
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("completed checkpoint reopened after duplicate enqueue: %d", calls)
	}
	if _, ok := s.goalContinuations[res.GoalID]; ok {
		t.Fatal("completed duplicate checkpoint remained manually resumable")
	}
}

func TestLimitBoundaryAutomaticallyCreatesNewSliceInSameTaskRun(t *testing.T) {
	runtime := agentruntime.New()
	firstRunner := agentloop.Runner{
		Runtime:  runtime,
		Planner:  &continuationTestPlanner{outputs: []planner.Output{{ToolCalls: []planner.ToolCall{{ID: "observe-once", Tool: "project.state"}}}}},
		Executor: continuationTestExecutor{},
		Budget:   agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	paused := firstRunner.Start(context.Background(), agentloop.Input{
		UserText: "inspect the current project", Summary: "inspect the current project",
		AllowedTools: []string{"project.state"},
	})
	if paused.Status != agentruntime.StatusWaitingContinue || paused.StopReason != agentloop.StopReasonLimitReached || paused.Continuation == nil {
		t.Fatalf("first slice did not stop at its local limit: %+v", paused)
	}

	s := testContinuationServer()
	defer s.Close()
	s.recordGoalResult("conversation-runtime", paused)
	var resumed agentloop.Result
	s.continuationExecutor = func(ctx context.Context, item DurableContinuation) error {
		nextRunner := agentloop.Runner{
			Runtime: runtime, Planner: &continuationTestPlanner{outputs: []planner.Output{{Done: true, Reply: "completed automatically"}}},
			Executor: continuationTestExecutor{}, Budget: agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		}
		resumed = nextRunner.Continue(ctx, item.Continuation)
		s.recordGoalResult(item.ConversationID, resumed)
		return nil
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resumed.Status != agentruntime.StatusCompleted || resumed.SliceID == paused.SliceID {
		t.Fatalf("automatic continuation did not complete in a new slice: paused=%+v resumed=%+v", paused, resumed)
	}
	if resumed.TaskID != paused.TaskID || resumed.GoalID != paused.GoalID || resumed.RunID != paused.RunID {
		t.Fatalf("task identity changed across automatic slices: paused=%+v resumed=%+v", paused, resumed)
	}
	if resumed.OriginalIntent != "inspect the current project" {
		t.Fatalf("original intent changed across automatic continuation: %q", resumed.OriginalIntent)
	}
	goal := runtime.Status(paused.GoalID)
	if goal.Task == nil || len(goal.Task.Run.Slices) != 2 || goal.Task.Run.Slices[0].SliceID != paused.SliceID || goal.Task.Run.Slices[1].SliceID != resumed.SliceID {
		t.Fatalf("runtime did not retain the two-slice history: %+v", goal)
	}
}

func TestContinuationLeaseExpiryAllowsExactlyOneReclaim(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	s.durableContinuations["cont-lease"] = DurableContinuation{
		ContinuationID: "cont-lease", TaskID: "task", GoalID: "goal", RunID: "run",
		Status: ContinuationPending, CreatedAt: now, UpdatedAt: now,
	}
	claimed, ok := s.claimNextContinuation(now)
	if !ok || claimed.Attempt != 1 {
		t.Fatalf("initial claim failed: %+v %v", claimed, ok)
	}
	if _, ok := s.claimNextContinuation(now.Add(10 * time.Second)); ok {
		t.Fatal("live lease was claimed twice")
	}
	item := s.durableContinuations["cont-lease"]
	item.LeaseExpiresAt = now.Add(-time.Second)
	s.durableContinuations["cont-lease"] = item
	reclaimed, ok := s.claimNextContinuation(now.Add(11 * time.Second))
	if !ok || reclaimed.Attempt != 2 {
		t.Fatalf("expired lease was not reclaimed: %+v %v", reclaimed, ok)
	}
	if _, ok := s.claimNextContinuation(now.Add(11 * time.Second)); ok {
		t.Fatal("reclaimed live lease was dispatched twice")
	}
}

func TestStartupScanWaitsForLiveLeaseThenReclaims(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	now := time.Now().UTC()
	s.durableContinuations["cont-startup-lease"] = DurableContinuation{
		ContinuationID: "cont-startup-lease", TaskID: "task", GoalID: "goal", RunID: "run",
		Status: ContinuationRunning, Attempt: 1, LeaseOwner: "crashed-worker",
		LeaseExpiresAt: now.Add(100 * time.Millisecond), CreatedAt: now, UpdatedAt: now,
	}
	dispatched := make(chan DurableContinuation, 1)
	s.continuationExecutor = func(_ context.Context, item DurableContinuation) error {
		dispatched <- item
		return nil
	}
	s.wakeContinuationScheduler()
	select {
	case item := <-dispatched:
		if item.Attempt != 2 || item.LeaseOwner != "test-worker" {
			t.Fatalf("expired startup lease was not reclaimed by the new worker: %+v", item)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("startup scanner did not reclaim the expired lease")
	}
}

func TestConcurrentClaimsHaveSingleWinner(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	s.durableContinuations["cont-concurrent"] = DurableContinuation{
		ContinuationID: "cont-concurrent", TaskID: "task", GoalID: "goal", RunID: "run",
		Status: ContinuationPending, CreatedAt: now, UpdatedAt: now,
	}
	const contenders = 12
	var wg sync.WaitGroup
	winners := make(chan DurableContinuation, contenders)
	for i := 0; i < contenders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if item, ok := s.claimNextContinuation(now); ok {
				winners <- item
			}
		}()
	}
	wg.Wait()
	close(winners)
	if len(winners) != 1 {
		t.Fatalf("concurrent claim winners = %d, want exactly 1", len(winners))
	}
}

func TestWaitingInteractionIsNotAutoPending(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-1", "turn-1", agentruntime.StatusWaitingClarification, agentloop.StopReasonNeedsClarification)
	s.recordGoalResult("conversation-c", res)
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationWaitingInteraction {
			t.Fatalf("interaction boundary was auto-queued: %+v", item)
		}
	}
	if _, ok := s.claimNextContinuation(time.Now().UTC().Add(time.Hour)); ok {
		t.Fatal("waiting interaction was claimed by scheduler")
	}
	if _, ok := s.automaticContinuationForConversation("conversation-c"); ok {
		t.Fatal("interaction boundary was exposed as an automatic continuation")
	}
	if item, ok := s.interactionContinuationForConversation("conversation-c"); !ok || item.Status != ContinuationWaitingInteraction {
		t.Fatalf("manual continue guard cannot identify the pending interaction: %+v ok=%v", item, ok)
	}
}

func TestWaitingInteractionPersistsRuntimeRequestPayload(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-interaction", "turn-interaction", agentruntime.StatusWaitingConfirmation, agentloop.StopReasonNeedsConfirmation)
	res.Executed = []map[string]any{{"result": map[string]any{"interaction_requests": []any{map[string]any{
		"request_id": "interaction-7", "kind": "confirmation", "prompt": "Confirm this bounded action",
	}}}}}
	if err := s.recordGoalResult("conversation-c", res); err != nil {
		t.Fatal(err)
	}
	for _, item := range s.durableContinuations {
		requests, ok := item.PendingInteraction["requests"].([]any)
		if item.Status != ContinuationWaitingInteraction || !ok || len(requests) != 1 {
			t.Fatalf("runtime interaction payload was not durably retained: %+v", item)
		}
		request, _ := requests[0].(map[string]any)
		if request["request_id"] != "interaction-7" || item.PendingInteraction["continuation_id"] != item.ContinuationID {
			t.Fatalf("persisted interaction identity is incomplete: %+v", item.PendingInteraction)
		}
	}
}

func TestRestartRestoresPendingInteractionWithoutSchedulingIt(t *testing.T) {
	source := testContinuationServer()
	res := waitingContinuationResult("slice-interaction-restart", "turn-interaction-restart", agentruntime.StatusWaitingConfirmation, agentloop.StopReasonNeedsConfirmation)
	res.Executed = []map[string]any{{"result": map[string]any{"interaction_requests": []any{map[string]any{
		"request_id": "interaction-restart", "kind": "human_judgment", "prompt": "Choose the preferred candidate",
	}}}}}
	if err := source.recordGoalResult("conversation-c", res); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(source.projectAgentRuntimeStateLocked())
	if err != nil {
		t.Fatal(err)
	}
	var snapshot projectAgentRuntimeState
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	target := testContinuationServer()
	target.restoreProjectAgentRuntimeStateLocked(snapshot)
	item, ok := target.interactionContinuationForConversation("conversation-c")
	if !ok || item.Status != ContinuationWaitingInteraction {
		t.Fatalf("pending interaction did not survive restart: %+v ok=%v", item, ok)
	}
	requests, _ := item.PendingInteraction["requests"].([]any)
	if len(requests) != 1 || firstStringFromMap(firstMapFromAny(requests[0]), "request_id") != "interaction-restart" {
		t.Fatalf("restored interaction request payload is incomplete: %+v", item.PendingInteraction)
	}
	if _, claimable := target.claimNextContinuation(time.Now().UTC().Add(time.Hour)); claimable {
		t.Fatal("restored human interaction was scheduled automatically")
	}
}

func TestChildCheckpointAtomicallyCompletesItsParent(t *testing.T) {
	s := testContinuationServer()
	first := waitingContinuationResult("slice-parent", "turn-parent", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	if err := s.recordGoalResult("conversation-c", first); err != nil {
		t.Fatal(err)
	}
	if first.Continuation.ContinuationID == "" {
		t.Fatal("runtime checkpoint ID was not propagated to downstream interaction continuations")
	}
	var parent DurableContinuation
	for _, item := range s.durableContinuations {
		parent = item
	}
	child := waitingContinuationResult("slice-child", "turn-child", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	child.ResumedFromID = parent.ContinuationID
	if err := s.recordGoalResult("conversation-c", child); err != nil {
		t.Fatal(err)
	}
	if s.durableContinuations[parent.ContinuationID].Status != ContinuationCompleted {
		t.Fatalf("parent checkpoint remained claimable after child was recorded: %+v", s.durableContinuations[parent.ContinuationID])
	}
	childID := continuationIDForResult(child)
	if got := s.durableContinuations[childID]; got.Status != ContinuationPending || got.Continuation.ContinuationID != childID {
		t.Fatalf("child checkpoint is not the sole runnable continuation: %+v", got)
	}
}

func TestClaimMustPersistBeforeExecutorRuns(t *testing.T) {
	s := testContinuationServer()
	if err := s.recordGoalResult("conversation-c", waitingContinuationResult("slice-persist", "turn-persist", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)); err != nil {
		t.Fatal(err)
	}
	persistErr := errors.New("runtime store unavailable")
	s.continuationPersist = func() error { return persistErr }
	calls := 0
	s.continuationExecutor = func(context.Context, DurableContinuation) error {
		calls++
		return nil
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); !errors.Is(err, persistErr) {
		t.Fatalf("scheduler persistence error = %v, want %v", err, persistErr)
	}
	if calls != 0 {
		t.Fatalf("executor ran before its claim was durable: calls=%d", calls)
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationPending || item.LeaseOwner != "" || item.LastError != persistErr.Error() {
			t.Fatalf("failed durable claim was not safely released: %+v", item)
		}
	}
}

func TestCheckpointPersistenceFailurePreventsAutomaticContinuation(t *testing.T) {
	s := testContinuationServer()
	persistErr := errors.New("checkpoint write failed")
	s.continuationPersist = func() error { return persistErr }
	res := waitingContinuationResult("slice-checkpoint-fail", "turn-checkpoint-fail", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	if err := s.recordGoalResult("conversation-c", res); !errors.Is(err, persistErr) {
		t.Fatalf("record persistence error = %v, want %v", err, persistErr)
	}
	if _, ok := s.goalContinuations[res.GoalID]; ok {
		t.Fatal("undurable checkpoint remained available through the compatibility resume index")
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationFailed || item.LastError != persistErr.Error() {
			t.Fatalf("undurable checkpoint remained runnable: %+v", item)
		}
	}
}

func TestHumanJudgmentAndPermissionBoundariesNeverAutoRun(t *testing.T) {
	tests := []agentloop.Result{
		waitingContinuationResult("slice-judgment", "turn-judgment", agentruntime.StatusWaitingContinue, "user_judgment_pending"),
		waitingContinuationResult("slice-permission", "turn-permission", agentruntime.StatusWaitingContinue, "insufficient_permission"),
	}
	for _, res := range tests {
		s := testContinuationServer()
		s.recordGoalResult("conversation-c", res)
		for _, item := range s.durableContinuations {
			if item.Status != ContinuationWaitingInteraction {
				t.Fatalf("interaction boundary %q was auto-queued: %+v", res.StopReason, item)
			}
		}
		if _, ok := s.claimNextContinuation(time.Now().UTC().Add(time.Hour)); ok {
			t.Fatalf("interaction boundary %q was claimed", res.StopReason)
		}
	}
}

func TestRestoreMigratesLegacyContinuationToDurableRecord(t *testing.T) {
	s := testContinuationServer()
	legacyResult := waitingContinuationResult("slice-1", "turn-1", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	s.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		GoalContinuations: map[string]agentloop.Continuation{
			"goal-c": *legacyResult.Continuation,
		},
		ConversationGoals: map[string]string{"conversation-c": "goal-c"},
		GoalRuntime:       agentruntime.Snapshot{Goals: []agentruntime.Goal{{GoalID: "goal-c", Status: agentruntime.StatusWaitingContinue}}},
	})
	if len(s.durableContinuations) != 1 {
		t.Fatalf("legacy continuation was not migrated: %+v", s.durableContinuations)
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationWaitingInteraction {
			t.Fatalf("legacy checkpoint without a stop reason must fail closed: %+v", item)
		}
	}
}

func TestRestartSnapshotAutomaticallyDispatchesPendingContinuationOnce(t *testing.T) {
	source := testContinuationServer()
	defer source.Close()
	res := waitingContinuationResult("slice-before-restart", "turn-before-restart", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	res.Continuation.Context["observation_ledger"] = map[string]any{"active_observation_id": "obs-7"}
	res.Continuation.ContextSnapshot = map[string]any{"project_revision": "rev-7"}
	res.Continuation.ProjectHistory = map[string]any{"head": "rev-7", "active_branch": "task-branch"}
	res.Continuation.PlanItems = []planner.PlanItem{{ID: "inspect", Description: "inspect project", Status: "running"}}
	source.recordGoalResult("conversation-c", res)

	snapshot := source.projectAgentRuntimeStateLocked()
	projectPath := filepath.Join(t.TempDir(), "continuation-restart.vit")
	if err := os.WriteFile(projectPath, []byte("test project identity only"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_continuation_restart"
	snapshot.ProjectPath = projectPath
	snapshot.ProjectUUID = projectUUID
	for id, item := range snapshot.DurableContinuations {
		item.ProjectPath = projectPath
		item.ProjectUUID = projectUUID
		snapshot.DurableContinuations[id] = item
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := history.WriteAgentRuntimeState(projectPath, projectUUID, data); err != nil {
		t.Fatal(err)
	}
	data, err = history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	var restoredState projectAgentRuntimeState
	if err := json.Unmarshal(data, &restoredState); err != nil {
		t.Fatal(err)
	}

	target := testContinuationServer()
	defer target.Close()
	target.activeWorkspacePath = projectPath
	target.activeWorkspaceUUID = projectUUID
	dispatched := make(chan DurableContinuation, 2)
	target.continuationExecutor = func(_ context.Context, item DurableContinuation) error {
		dispatched <- item
		return nil
	}
	target.restoreProjectAgentRuntimeStateLocked(restoredState)
	target.wakeContinuationScheduler()

	var item DurableContinuation
	select {
	case item = <-dispatched:
	case <-time.After(2 * time.Second):
		t.Fatal("startup scan did not dispatch the restored continuation")
	}
	if item.TaskID != "task-c" || item.GoalID != "goal-c" || item.RunID != "run-c" || item.ConversationID != "conversation-c" {
		t.Fatalf("restored identity changed: %+v", item)
	}
	if item.ProjectPath != projectPath || item.ProjectUUID != projectUUID {
		t.Fatalf("restored project identity changed: %+v", item)
	}
	if item.CurrentSliceID != "slice-before-restart" || item.OriginalIntent != "inspect the project" {
		t.Fatalf("restored checkpoint changed: %+v", item)
	}
	ledger, _ := item.Continuation.Context["observation_ledger"].(map[string]any)
	if ledger["active_observation_id"] != "obs-7" || len(item.Continuation.PlanItems) != 1 {
		t.Fatalf("observation ledger or current plan was lost: %+v", item.Continuation)
	}
	if item.ProjectRevision != "rev-7" || item.ProjectHistory["head"] != "rev-7" {
		t.Fatalf("project revision was lost: revision=%q history=%+v", item.ProjectRevision, item.ProjectHistory)
	}
	// Let the worker commit completion, then verify no periodic scan repeats it.
	time.Sleep(continuationSchedulerTick * 2)
	select {
	case duplicate := <-dispatched:
		t.Fatalf("restored continuation dispatched more than once: %+v", duplicate)
	default:
	}
}

func TestProductStartupRestoresAndExecutesSameTaskClosureAndFreeStateLoop(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "runtime-restart.vit")
	if err := os.WriteFile(projectPath, []byte("project identity fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_product_runtime_restart"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})

	source := New(nil, project, nil)
	source.activateCurrentProjectWorkspace(context.Background())
	firstRunner := agentloop.Runner{
		Runtime:  source.harness.Runtime(),
		Planner:  &continuationTestPlanner{outputs: []planner.Output{{ToolCalls: []planner.ToolCall{{ID: "observe-before-restart", Tool: "project.state"}}}}},
		Executor: continuationTestExecutor{},
		Budget:   agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	paused := firstRunner.Start(context.Background(), agentloop.Input{
		GoalID: "goal-product-restart", RunID: "run-product-restart",
		UserText: "inspect the project without a preset target", Summary: "inspect the project without a preset target",
		AllowedTools:   []string{"project.state"},
		Context:        map[string]any{"observation_ledger": map[string]any{"active_observation_id": "obs-product-7"}},
		ProjectHistory: map[string]any{"head": "revision-product-7", "active_branch": "task-branch"},
		PlanItems:      []planner.PlanItem{{ID: "inspect", Description: "inspect project", Status: "running"}},
	})
	if paused.Status != agentruntime.StatusWaitingContinue || paused.Continuation == nil {
		t.Fatalf("source runtime did not produce a resumable slice: %+v", paused)
	}
	closure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-product-restart", ConversationID: "conversation-product-restart",
		GoalID: paused.GoalID, RunID: paused.RunID, ProjectUUID: projectUUID, ProjectRevision: "revision-product-7",
		OriginalIntent: paused.OriginalIntent, Mode: audioclosure.ModeDiagnostic,
		Scope: audioclosure.Scope{Kind: "project"}, Now: time.Now().UTC(),
	})
	if err != nil || source.audioClosures.Create(closure) != nil {
		t.Fatalf("create source closure: state=%+v err=%v", closure, err)
	}
	source.mu.Lock()
	source.freeStateLoops["conversation-product-restart"] = freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-product-restart",
		ConversationID: "conversation-product-restart", GoalID: paused.GoalID, RunID: paused.RunID,
		Status: "reasoning", OriginalIntent: paused.OriginalIntent, ActiveIntent: paused.OriginalIntent,
		ObservationLedger: map[string]any{"active_observation_id": "obs-product-7"},
		CreatedAt:         time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	// Keep the source HTTP owner active until shutdown so its worker cannot
	// consume the checkpoint that this test needs the restarted process to own.
	source.activeRuntimeInvocations = 1
	source.mu.Unlock()
	if err := source.recordGoalResult("conversation-product-restart", paused); err != nil {
		t.Fatal(err)
	}
	_ = source.Close()

	targetProject := shadow.New(nil)
	target := New(nil, targetProject, nil)
	defer target.Close()
	dispatched := make(chan agentloop.Result, 1)
	target.continuationExecutor = func(ctx context.Context, item DurableContinuation) error {
		nextRunner := agentloop.Runner{
			Runtime: target.harness.Runtime(),
			Planner: &continuationTestPlanner{outputs: []planner.Output{{
				Done: true, Reply: "completed after process restart",
				PlanItems:  []planner.PlanItem{{ID: "inspect", Description: "inspect project", Status: "completed", Evidence: "project.state observation restored"}},
				Completion: &planner.Completion{SatisfiedPlanIDs: []string{"inspect"}, Evidence: []string{"project.state observation restored"}},
			}}},
			Executor: continuationTestExecutor{},
			Budget:   agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		}
		resumed := nextRunner.Continue(ctx, item.Continuation)
		if err := target.recordGoalResult(item.ConversationID, resumed); err != nil {
			return err
		}
		dispatched <- resumed
		return nil
	}
	target.Start()
	// Product startup begins before the bridge has necessarily delivered the
	// first project state. The worker must keep discovering until identity is
	// available, then restore without an HTTP request.
	time.AfterFunc(100*time.Millisecond, func() {
		targetProject.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	})

	var resumed agentloop.Result
	select {
	case resumed = <-dispatched:
	case <-time.After(3 * time.Second):
		t.Fatal("product startup worker did not recover and execute the durable checkpoint")
	}
	if resumed.Status != agentruntime.StatusCompleted || resumed.TaskID != paused.TaskID || resumed.GoalID != paused.GoalID || resumed.RunID != paused.RunID || resumed.SliceID == paused.SliceID {
		t.Fatalf("restart execution changed task identity or reused the invocation slice: paused=%+v resumed=%+v", paused, resumed)
	}
	goal := target.harness.RuntimeStatus(paused.GoalID)
	if goal.Task == nil || goal.Task.OriginalIntent != paused.OriginalIntent || len(goal.Task.Run.Slices) != 2 {
		t.Fatalf("restored task runtime cannot continue its original run: %+v", goal)
	}
	loop, ok := target.freeStateLoop("conversation-product-restart")
	if !ok || loop.LoopID != "loop-product-restart" || firstStringFromMap(loop.ObservationLedger, "active_observation_id") != "obs-product-7" {
		t.Fatalf("free-state loop or observation ledger was not restored: %+v", loop)
	}
	restoredClosure, ok := target.audioClosures.Load("closure-product-restart")
	if !ok || restoredClosure.GoalID != paused.GoalID || restoredClosure.RunID != paused.RunID || restoredClosure.OriginalIntent != paused.OriginalIntent {
		t.Fatalf("audio closure identity was not restored with the task: %+v", restoredClosure)
	}
	projectRevisionRestored := false
	for _, item := range target.durableContinuations {
		if item.GoalID == paused.GoalID && item.ProjectUUID == projectUUID && item.ProjectRevision == "revision-product-7" && item.ProjectHistory["head"] == "revision-product-7" {
			projectRevisionRestored = true
		}
	}
	if !projectRevisionRestored {
		t.Fatalf("project revision/history was not retained in durable continuation projection: %+v", target.durableContinuations)
	}
}

func TestAutomaticSchedulerRunsAfterRequestOwnerReturns(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	done := make(chan DurableContinuation, 1)
	s.continuationExecutor = func(_ context.Context, item DurableContinuation) error {
		done <- item
		return nil
	}
	// recordGoalResult represents the last synchronous operation owned by the
	// HTTP invocation. The scheduler owns everything after it returns.
	s.recordGoalResult("conversation-c", waitingContinuationResult("slice-http", "turn-http", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached))
	select {
	case item := <-done:
		if item.CurrentSliceID != "slice-http" {
			t.Fatalf("unexpected dispatched checkpoint: %+v", item)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("continuation did not run without a second request")
	}
}

func TestActiveHTTPInvocationDefersContinuationClaim(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	s.durableContinuations["cont-http-gate"] = DurableContinuation{
		ContinuationID: "cont-http-gate", TaskID: "task", GoalID: "goal", RunID: "run",
		Status: ContinuationPending, CreatedAt: now, UpdatedAt: now,
	}
	s.activeRuntimeInvocations = 1
	if _, ok := s.claimNextContinuation(now); ok {
		t.Fatal("scheduler claimed work before the HTTP invocation ended")
	}
	s.activeRuntimeInvocations = 0
	if _, ok := s.claimNextContinuation(now); !ok {
		t.Fatal("scheduler did not claim work after the HTTP invocation ended")
	}
}

func TestTerminalResultClosesOutstandingContinuations(t *testing.T) {
	s := testContinuationServer()
	res := waitingContinuationResult("slice-terminal", "turn-terminal", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	s.recordGoalResult("conversation-c", res)
	terminal := res
	terminal.Status = agentruntime.StatusCancelled
	terminal.StopReason = agentloop.StopReasonCancelled
	terminal.Continuation = nil
	s.recordGoalResult("conversation-c", terminal)
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationCancelled {
			t.Fatalf("cancelled task retained runnable continuation: %+v", item)
		}
	}
	if _, ok := s.claimNextContinuation(time.Now().UTC().Add(time.Hour)); ok {
		t.Fatal("cancelled continuation remained claimable")
	}
}

func TestSchedulerFailureIsDurableAndNotManuallyResumable(t *testing.T) {
	s := testContinuationServer()
	s.recordGoalResult("conversation-c", waitingContinuationResult("slice-fail", "turn-fail", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached))
	s.recordGoalResult("conversation-c", waitingContinuationResult("slice-fail-next", "turn-fail-next", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached))
	s.continuationExecutor = func(context.Context, DurableContinuation) error { return errors.New("executor unavailable") }
	if err := s.runContinuationSchedulerOnce(context.Background()); err == nil {
		t.Fatal("scheduler failure was not returned")
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationFailed || item.LastError != "executor unavailable" || item.LeaseOwner != "" || !item.LeaseExpiresAt.IsZero() {
			t.Fatalf("scheduler failure did not close every checkpoint for the failed goal: %+v", item)
		}
	}
	if _, ok := s.goalContinuations["goal-c"]; ok {
		t.Fatal("failed scheduler checkpoint remained manually resumable")
	}
}

func TestSchedulerShutdownCancellationLeavesCheckpointRecoverable(t *testing.T) {
	s := testContinuationServer()
	if err := s.recordGoalResult("conversation-c", waitingContinuationResult("slice-shutdown", "turn-shutdown", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	s.continuationExecutor = func(ctx context.Context, _ DurableContinuation) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	s.wakeContinuationScheduler()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("continuation executor did not start")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("scheduler did not drain its cancellation checkpoint: %v", err)
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationPending || item.LeaseOwner != "" || !item.LeaseExpiresAt.IsZero() {
			t.Fatalf("shutdown cancellation made the task terminal: %+v", item)
		}
	}
	if _, ok := s.goalContinuations["goal-c"]; !ok {
		t.Fatal("shutdown cancellation removed the resumable checkpoint")
	}
}

func TestDurableInvocationMarkerCannotBeForgedByRequestContext(t *testing.T) {
	clean := contextWithoutUntrustedSemanticEntry(map[string]any{
		"durable_continuation":    true,
		"durable_continuation_id": "cont-forged",
		"selected_track_id":       "track-1",
	})
	if clean["durable_continuation"] != nil || clean["durable_continuation_id"] != nil {
		t.Fatalf("untrusted continuation marker survived sanitization: %+v", clean)
	}
	if clean["selected_track_id"] != "track-1" {
		t.Fatalf("unrelated request context was removed: %+v", clean)
	}
	ctx := context.WithValue(context.Background(), durableContinuationContextKey{}, "cont-internal")
	if durableContinuationInvocationID(ctx) != "cont-internal" || durableContinuationInvocationID(context.Background()) != "" {
		t.Fatal("internal continuation marker is not isolated from ordinary contexts")
	}
}

func TestDurableResumeUsesExactClaimedCheckpoint(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	first := durableContinuationFromResult("conversation-c", waitingContinuationResult("slice-first", "turn-first", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached), now)
	second := durableContinuationFromResult("conversation-c", waitingContinuationResult("slice-second", "turn-second", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached), now.Add(time.Second))
	first.Status = ContinuationRunning
	second.Status = ContinuationPending
	s.durableContinuations[first.ContinuationID] = first
	s.durableContinuations[second.ContinuationID] = second
	cont, ok := s.resumeContinuationForChat("conversation-c", map[string]any{"durable_continuation_id": first.ContinuationID})
	if !ok || cont.SliceID != "slice-first" {
		t.Fatalf("scheduler resumed the wrong checkpoint: ok=%v continuation=%+v", ok, cont)
	}
}

func TestManualContinueCannotDuplicateAutomaticCheckpoint(t *testing.T) {
	s := testContinuationServer()
	s.recordGoalResult("conversation-c", waitingContinuationResult("slice-auto", "turn-auto", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached))
	item, ok := s.automaticContinuationForConversation("conversation-c")
	if !ok || item.Status != ContinuationPending || item.CurrentSliceID != "slice-auto" {
		t.Fatalf("automatic checkpoint was not discoverable for continue deduplication: %+v ok=%v", item, ok)
	}
	if _, ok := s.automaticContinuationForConversation("other-conversation"); ok {
		t.Fatal("automatic checkpoint leaked across conversations")
	}
}

func TestContinuationRuntimeProjectionExposesDurableIdentity(t *testing.T) {
	s := testContinuationServer()
	s.recordGoalResult("conversation-c", waitingContinuationResult("slice-projection", "turn-projection", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached))
	rows := s.continuationRuntimeProjection()
	if len(rows) != 1 || rows[0]["task_id"] != "task-c" || rows[0]["run_id"] != "run-c" || rows[0]["slice_id"] != "slice-projection" || rows[0]["status"] != ContinuationPending || rows[0]["original_intent"] != "inspect the project" {
		t.Fatalf("durable continuation projection is incomplete: %+v", rows)
	}
}

func TestRecordGoalResultPersistsCanonicalTaskSemantics(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	goal := s.harness.EnsureGoal("goal-semantic", "run-semantic", "inspect and improve the project")
	contract := taskstate.Contract{
		ConversationID: "conversation-semantic", Kind: taskstate.ContractImprovement, Scope: taskstate.Scope{Kind: "project"},
		Temporary: true, TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria: []string{"governed outcome"}, EvidenceRequirements: []string{"observation reference"},
		ProjectUUID: "project-semantic", ProjectRevision: "revision-semantic",
	}
	goal, err := s.harness.EnsureTaskContract(goal.GoalID, contract)
	if err != nil {
		t.Fatal(err)
	}
	res := waitingContinuationResult("slice-semantic", "turn-semantic", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	res.GoalID, res.RunID, res.TaskID, res.OriginalIntent = goal.GoalID, goal.RunID, goal.Task.TaskID, goal.Task.OriginalIntent
	res.Continuation.GoalID, res.Continuation.RunID, res.Continuation.TaskID = goal.GoalID, goal.RunID, goal.Task.TaskID
	res.Continuation.OriginalIntent = goal.Task.OriginalIntent
	if err := s.recordGoalResult("conversation-semantic", res); err != nil {
		t.Fatal(err)
	}
	for _, item := range s.durableContinuations {
		if item.TaskContract == nil || item.TaskSemanticState == nil {
			t.Fatalf("canonical task semantics were not persisted: %+v", item)
		}
		if item.TaskContract.ContractID != goal.Task.Contract.ContractID || item.TaskSemanticState.State != taskstate.StateObservationInProgress {
			t.Fatalf("canonical task semantics changed in continuation: %+v", item)
		}
		rows := s.continuationRuntimeProjection()
		if len(rows) != 1 || rows[0]["task_state"] != taskstate.StateObservationInProgress || rows[0]["task_state_revision"] != uint64(1) {
			t.Fatalf("runtime projection omitted canonical task state: %+v", rows)
		}
	}
}

func TestRestoreRejectsContinuationWithDifferentCanonicalContract(t *testing.T) {
	now := time.Now().UTC()
	contract := taskstate.NormalizeContract(taskstate.Contract{
		ContractID: "contract-authority", TaskID: "task-authority", GoalID: "goal-authority", RunID: "run-authority", ConversationID: "conversation-authority",
		OriginalIntent: "inspect the project", Kind: taskstate.ContractImprovement, Scope: taskstate.Scope{Kind: "project"},
		AuthorizationBoundary: "governed_experiment", CompletionCriteria: []string{"governed outcome"}, EvidenceRequirements: []string{"observation reference"},
	}, now)
	semantic, err := taskstate.New(contract, now)
	if err != nil {
		t.Fatal(err)
	}
	state := projectAgentRuntimeState{ConversationGoals: map[string]string{"conversation-authority": "goal-authority"}, GoalRuntime: agentruntime.Snapshot{Goals: []agentruntime.Goal{{
		GoalID: "goal-authority", RunID: "run-authority", Status: agentruntime.StatusWaitingContinue,
		Task: &agentruntime.Task{TaskID: "task-authority", GoalID: "goal-authority", OriginalIntent: "inspect the project", Contract: &contract, SemanticState: &semantic},
	}}}}
	different := contract
	different.ContractID = "contract-different"
	differentSemantic, err := taskstate.New(different, now)
	if err != nil {
		t.Fatal(err)
	}
	_, restored := normalizeRestoredDurableContinuation("continuation-authority", DurableContinuation{
		ContinuationID: "continuation-authority", TaskID: "task-authority", GoalID: "goal-authority", RunID: "run-authority",
		ConversationID: "conversation-authority", CurrentSliceID: "slice-authority", OriginalIntent: "inspect the project",
		TaskContract: &different, TaskSemanticState: &differentSemantic,
		Continuation: agentloop.Continuation{GoalID: "goal-authority", RunID: "run-authority", TaskID: "task-authority", SliceID: "slice-authority", OriginalIntent: "inspect the project"},
		Status:       ContinuationPending,
	}, state, now)
	if restored.Status != ContinuationWaitingInteraction || firstStringFromMap(restored.PendingInteraction, "status") != "recovery_validation_required" {
		t.Fatalf("canonical contract mismatch remained runnable: %+v", restored)
	}
}

func TestRestoreUsesTaskSnapshotAsIdentityAndIntentAuthority(t *testing.T) {
	now := time.Now().UTC()
	state := projectAgentRuntimeState{
		ConversationGoals: map[string]string{"conversation-authority": "goal-authority"},
		GoalRuntime: agentruntime.Snapshot{Goals: []agentruntime.Goal{{
			GoalID: "goal-authority", RunID: "run-authority",
			Task: &agentruntime.Task{TaskID: "task-authority", GoalID: "goal-authority", OriginalIntent: "authoritative original intent"},
		}}},
	}
	id, item := normalizeRestoredDurableContinuation("cont-authority", DurableContinuation{
		GoalID: "goal-authority", RunID: "run-corrupt", TaskID: "task-corrupt", OriginalIntent: "continuation text",
		Continuation: agentloop.Continuation{GoalID: "goal-authority", RunID: "run-corrupt", TaskID: "task-corrupt", OriginalIntent: "continuation text"},
		Status:       "unknown_status",
	}, state, now)
	if id != "cont-authority" || item.RunID != "run-authority" || item.TaskID != "task-authority" || item.OriginalIntent != "authoritative original intent" {
		t.Fatalf("task snapshot did not repair durable identity: %+v", item)
	}
	if item.Continuation.OriginalIntent != item.OriginalIntent || item.Continuation.RunID != item.RunID || item.Continuation.TaskID != item.TaskID {
		t.Fatalf("nested continuation was not normalized to task authority: %+v", item.Continuation)
	}
	if item.Status != ContinuationWaitingInteraction || item.ConversationID != "conversation-authority" {
		t.Fatalf("invalid restored state did not fail closed: %+v", item)
	}
}

func TestRestoreFailsClosedOnProjectOrConversationIdentityMismatch(t *testing.T) {
	now := time.Now().UTC()
	state := projectAgentRuntimeState{
		ProjectUUID:       "project-authority",
		ConversationGoals: map[string]string{"conversation-authority": "goal-authority"},
	}
	_, projectMismatch := normalizeRestoredDurableContinuation("cont-project-mismatch", DurableContinuation{
		ContinuationID: "cont-project-mismatch", GoalID: "goal-authority", RunID: "run-authority", TaskID: "task-authority",
		ConversationID: "conversation-authority", ProjectUUID: "different-project", OriginalIntent: "inspect project",
		Continuation: agentloop.Continuation{GoalID: "goal-authority", RunID: "run-authority", TaskID: "task-authority", OriginalIntent: "inspect project"},
		Status:       ContinuationPending,
	}, state, now)
	if projectMismatch.Status != ContinuationWaitingInteraction || firstStringFromMap(projectMismatch.PendingInteraction, "reason") == "" {
		t.Fatalf("project identity mismatch remained automatically runnable: %+v", projectMismatch)
	}
	_, conversationMismatch := normalizeRestoredDurableContinuation("cont-conversation-mismatch", DurableContinuation{
		ContinuationID: "cont-conversation-mismatch", GoalID: "different-goal", RunID: "run-authority", TaskID: "task-authority",
		ConversationID: "conversation-authority", ProjectUUID: "project-authority", OriginalIntent: "inspect project",
		Continuation: agentloop.Continuation{GoalID: "different-goal", RunID: "run-authority", TaskID: "task-authority", OriginalIntent: "inspect project"},
		Status:       ContinuationRunning, LeaseOwner: "crashed-worker", LeaseExpiresAt: now.Add(time.Hour),
	}, state, now)
	if conversationMismatch.Status != ContinuationWaitingInteraction || conversationMismatch.LeaseOwner != "" || !conversationMismatch.LeaseExpiresAt.IsZero() {
		t.Fatalf("conversation/goal mismatch retained a live scheduler lease: %+v", conversationMismatch)
	}
}

func TestRecoveryValidationContinuationIsNotExecutableThroughChatLookup(t *testing.T) {
	s := testContinuationServer()
	cont := waitingContinuationResult("slice-recovery", "turn-recovery", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached).Continuation
	s.goalContinuations["goal-c"] = *cont
	item := durableContinuationFromResult("conversation-c", agentloop.Result{GoalID: "goal-c", RunID: "run-c", TaskID: "task-c", SliceID: "slice-recovery", TurnID: "turn-recovery", OriginalIntent: "inspect the project", Status: agentruntime.StatusWaitingContinue, Continuation: cont}, time.Now().UTC())
	item.Status = ContinuationWaitingInteraction
	item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "identity mismatch"}
	s.durableContinuations[item.ContinuationID] = item
	delete(s.goalContinuations, "goal-c")
	if _, ok := s.goalContinuationForConversation("conversation-c"); ok {
		t.Fatal("recovery-validation continuation was returned as executable chat continuation")
	}
}

func TestRestoreKeepsOnlyNewestCheckpointRunnablePerTaskRun(t *testing.T) {
	now := time.Now().UTC()
	items := map[string]DurableContinuation{
		"parent": {
			ContinuationID: "parent", TaskID: "task-linear", GoalID: "goal-linear", RunID: "run-linear",
			Status: ContinuationRunning, LeaseOwner: "crashed-worker", LeaseExpiresAt: now.Add(time.Hour),
			CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute),
		},
		"child": {
			ContinuationID: "child", TaskID: "task-linear", GoalID: "goal-linear", RunID: "run-linear",
			Status: ContinuationPending, CreatedAt: now, UpdatedAt: now,
		},
	}
	reconciled := reconcileRestoredContinuations(items)
	if reconciled["parent"].Status != ContinuationCompleted || reconciled["parent"].LeaseOwner != "" || !reconciled["parent"].LeaseExpiresAt.IsZero() {
		t.Fatalf("superseded parent remained runnable after restore: %+v", reconciled["parent"])
	}
	if reconciled["child"].Status != ContinuationPending {
		t.Fatalf("newest child checkpoint was not retained: %+v", reconciled["child"])
	}
}

func TestRestoreNeverReopensContinuationForTerminalGoal(t *testing.T) {
	now := time.Now().UTC()
	state := projectAgentRuntimeState{GoalRuntime: agentruntime.Snapshot{Goals: []agentruntime.Goal{{
		GoalID: "goal-terminal-authority", RunID: "run-terminal-authority", Status: agentruntime.StatusCancelled,
		Task: &agentruntime.Task{TaskID: "task-terminal-authority", GoalID: "goal-terminal-authority", OriginalIntent: "inspect project"},
	}}}}
	_, item := normalizeRestoredDurableContinuation("cont-terminal-authority", DurableContinuation{
		ContinuationID: "cont-terminal-authority", GoalID: "goal-terminal-authority", RunID: "run-terminal-authority", TaskID: "task-terminal-authority",
		ConversationID: "conversation-terminal-authority", CurrentSliceID: "slice-terminal-authority", OriginalIntent: "inspect project",
		Continuation: agentloop.Continuation{GoalID: "goal-terminal-authority", RunID: "run-terminal-authority", TaskID: "task-terminal-authority", SliceID: "slice-terminal-authority", OriginalIntent: "inspect project"},
		Status:       ContinuationRunning, LeaseOwner: "crashed-worker", LeaseExpiresAt: now.Add(time.Hour),
	}, state, now)
	if item.Status != ContinuationCancelled || item.LeaseOwner != "" || !item.LeaseExpiresAt.IsZero() {
		t.Fatalf("terminal goal authority was reopened by its stale continuation: %+v", item)
	}
}

// A request in flight owns the authoritative in-memory runtime state. The
// scheduler's durable-snapshot reload must wait for it: reload replaces the
// in-memory maps wholesale, so a capability route or checkpoint whose persist
// has not landed yet would be wiped and later fail closed as unrecoverable
// (17:20 D1 smoke: "durable capacity state has no validated task route").
func TestSchedulerWaitsForActiveInvocationBeforeReloadingRuntimeState(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	dir := t.TempDir()
	const projectUUID = "vitproj_test_reload_guard"
	s.mu.Lock()
	s.activeWorkspacePath = dir
	s.activeWorkspaceUUID = projectUUID
	s.mu.Unlock()

	// Mirror the production sequence: the HTTP handler holds the invocation
	// guard for the whole request, including recordGoalResult — which lazily
	// starts the scheduler goroutine via wakeContinuationScheduler. Without
	// the guard held first, that goroutine's first tick can reload the stale
	// snapshot before the request finishes (the 17:20 smoke failure mode).
	release := s.beginContinuationSensitiveInvocation()
	res := waitingContinuationResult("slice-guard", "turn-guard", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	res.Continuation.Context[capacityAssessmentContextKey] = FreeStateCapacityAssessment{
		SchemaVersion: capacityAssessmentSchema, Authority: "product_runtime",
		ProjectRevision: "2", SelectedCapability: capabilityFreeState,
		ObservedFacts: CapacityObservedFacts{ProjectUUID: projectUUID, ProjectRevision: "2", RequestScope: semanticEntryScopeProjectContext},
	}
	if err := s.recordGoalResult("conversation-guard", res); err != nil {
		t.Fatal(err)
	}
	// Simulate a stale durable snapshot: the checkpoint persist was contended
	// away, so the disk state predates the continuation and its route.
	stale, err := json.Marshal(projectAgentRuntimeState{
		SchemaVersion: "vit_project_agent_runtime.v2", ProjectPath: dir, ProjectUUID: projectUUID,
		DurableContinuations: map[string]DurableContinuation{}, CapabilityRoutes: map[string]CapabilityRouteRecord{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := history.WriteAgentRuntimeState(dir, projectUUID, stale); writeErr != nil {
		// Windows can transiently refuse the replace-rename on a file the
		// server just wrote; retry briefly instead of failing the fixture.
		deadline := time.Now().Add(2 * time.Second)
		for writeErr != nil && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			writeErr = history.WriteAgentRuntimeState(dir, projectUUID, stale)
		}
		if writeErr != nil {
			t.Fatal(writeErr)
		}
	}

	var executed int
	s.continuationExecutor = func(_ context.Context, _ DurableContinuation) error {
		executed++
		return nil
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if executed != 0 {
		t.Fatal("scheduler dispatched a continuation during an active invocation")
	}
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationPending {
			t.Fatalf("in-memory checkpoint was reloaded away during an active invocation: %+v", item)
		}
	}
	// Control: once the request finishes, the next drive does reload the
	// durable snapshot. The stale-disk fixture therefore wipes the
	// continuation, proving the guard above was what protected it.
	release()
	if err := s.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if executed != 0 {
		t.Fatal("stale durable snapshot unexpectedly claimed the continuation")
	}
	if len(s.durableContinuations) != 0 {
		t.Fatalf("expected the stale snapshot reload to replace in-memory continuations: %+v", s.durableContinuations)
	}
}

// Lock contention must surface as an error: pretending the persist succeeded
// silently drops authoritative routing state from the durable snapshot.
func TestPersistCheckedReportsLockContention(t *testing.T) {
	s := testContinuationServer()
	dir := t.TempDir()
	const projectUUID = "vitproj_test_persist_contention"
	s.mu.Lock()
	s.activeWorkspacePath = dir
	s.activeWorkspaceUUID = projectUUID
	s.mu.Unlock()
	lease, err := history.AcquireAgentRuntimeStateLock(dir, projectUUID, "external-holder", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lease.Release() }()
	if err := s.persistCurrentProjectWorkspaceChecked(); err == nil {
		t.Fatal("persist reported success while the runtime state lock was held by another owner")
	} else if !errors.Is(err, history.ErrAgentRuntimeStateLocked) {
		t.Fatalf("expected lock contention error, got %v", err)
	}
}

// A scheduler-driven slice that stops at an interaction boundary must park
// its checkpoint as waiting_interaction with the pending requests, and the
// parked checkpoint must retire when the interaction completes (2026-08-25
// D1 smoke: the confirmation completed invisibly).
func TestParkClaimedContinuationAtInteractionRetiresOnAnswer(t *testing.T) {
	s := testContinuationServer()
	item := DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: "cont_park_1",
		TaskID: "task-park", GoalID: "goal-park", RunID: "run-park", ConversationID: "conversation-park",
		CurrentSliceID: "slice-park", OriginalIntent: "inspect the project", Status: ContinuationClaimed,
	}
	s.durableContinuations[item.ContinuationID] = item
	pending := map[string]any{
		"status": "waiting_confirmation", "stop_reason": "improvement_proposal_confirmation_required",
		"interaction_id": "interaction-park-1",
		"requests": []any{map[string]any{
			"id": "interaction-park-1", "kind": "improvement_proposal_confirmation", "status": "waiting_for_user",
			"actions": []any{map[string]any{"id": "approve", "style": "primary", "recommended": true}},
		}},
	}
	s.parkClaimedContinuationAtInteraction(item, pending)
	parked, ok := s.durableContinuations[item.ContinuationID]
	if !ok || parked.Status != ContinuationWaitingInteraction || parked.LeaseOwner != "" || !parked.LeaseExpiresAt.IsZero() {
		t.Fatalf("claimed checkpoint was not parked at the interaction boundary: %+v", parked)
	}
	if firstStringFromMap(parked.PendingInteraction, "interaction_id") != "interaction-park-1" {
		t.Fatalf("parked pending lost the interaction id: %+v", parked.PendingInteraction)
	}
	s.completePendingInteractionContinuation(PendingInteraction{ID: "interaction-park-1", ConversationID: "conversation-park", GoalID: "goal-park", RunID: "run-park"})
	if retired, ok := s.durableContinuations[item.ContinuationID]; !ok || retired.Status != ContinuationCompleted || retired.PendingInteraction != nil {
		t.Fatalf("answered interaction did not retire the parked checkpoint: %+v", retired)
	}
}

// An answered mix-tick confirmation on the chat-message surface must close the
// durable waiting_confirmation park bound to the confirmed candidate. The
// interaction-respond surface routes that completion through
// completePendingInteractionContinuation before executing (S3c); the chat
// surface reaches execution without the bridge, so the answered park survived
// restarts as a non-terminal waiting_interaction continuation (2026-08-30 S3h8
// smoke: the round-2 proposal park resurrected by the restart-idempotency
// check while round 1 — confirmed through the respond surface — stayed clean).
func answeredMixTickParkServer(t *testing.T) (*Server, DurableContinuation) {
	t.Helper()
	s := testContinuationServer()
	s.mu.Lock()
	if s.pendingMixTicks == nil {
		s.pendingMixTicks = map[string]agentloop.PendingMixTickCandidate{}
	}
	s.pendingMixTicks["conversation-s3h8"] = agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "1032", ObservationID: "obs_s3h8_round2@4",
		Status: "pending_confirmation",
	}
	park := DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: "cont_s3h8_park",
		TaskID: "task-s3h8", GoalID: "goal-s3h8", RunID: "run-s3h8", ConversationID: "conversation-s3h8",
		CurrentSliceID: "slice-s3h8", OriginalIntent: "improve the boxy vocal", Status: ContinuationWaitingInteraction,
		PendingInteraction: map[string]any{
			"status": "waiting_confirmation", "stop_reason": "improvement_proposal_native_tool_confirmation_required",
			"interaction_id": "interaction-s3h8-1",
			"requests": []any{map[string]any{
				"id": "interaction-s3h8-1", "kind": "mix_tick_confirmation", "status": "waiting_for_user",
				"payload": map[string]any{"observation_id": "obs_s3h8_round2@4", "track_id": "1032", "operation": "track_gain_adjust"},
			}},
		},
	}
	s.durableContinuations[park.ContinuationID] = park
	s.mu.Unlock()
	return s, park
}

func TestHandlePendingMixTickChatAcceptanceClosesBoundPark(t *testing.T) {
	s, park := answeredMixTickParkServer(t)
	resp, handled := s.handlePendingMixTickChat(context.Background(), "conversation-s3h8", ChatRequest{
		ConversationID: "conversation-s3h8", Message: "可以执行",
	}, agentModeDefault)
	if !handled {
		t.Fatal("explicit confirmation must be handled by the pending mix tick surface")
	}
	if resp.StopReason != "expired_pending_mix_tick_candidate" {
		t.Fatalf("unexpected execution outcome for the zero-dose candidate: stop=%q err=%q", resp.StopReason, resp.Error)
	}
	retired, ok := s.durableContinuations[park.ContinuationID]
	if !ok || retired.Status != ContinuationCompleted || retired.PendingInteraction != nil {
		t.Fatalf("answered chat-surface confirmation left its bound park non-terminal: %+v", retired)
	}
}

// A rejection is an answered wait too: the same park must close, or a chat
// cancellation leaks the identical restart orphan the acceptance path did.
func TestHandlePendingMixTickChatRejectionClosesBoundPark(t *testing.T) {
	s, park := answeredMixTickParkServer(t)
	resp, handled := s.handlePendingMixTickChat(context.Background(), "conversation-s3h8", ChatRequest{
		ConversationID: "conversation-s3h8", Message: "取消",
	}, agentModeDefault)
	if !handled || resp.StopReason != "mix_tick_rejected" {
		t.Fatalf("unexpected rejection outcome: handled=%v stop=%q", handled, resp.StopReason)
	}
	retired, ok := s.durableContinuations[park.ContinuationID]
	if !ok || retired.Status != ContinuationCompleted || retired.PendingInteraction != nil {
		t.Fatalf("rejected chat-surface confirmation left its bound park non-terminal: %+v", retired)
	}
}

// A park bound to a different candidate is not this confirmation's to close:
// only the wait that was actually answered may be finalized.
func TestHandlePendingMixTickChatConfirmationSparesForeignPark(t *testing.T) {
	s, park := answeredMixTickParkServer(t)
	foreign := park
	foreign.ContinuationID = "cont_s3h8_foreign"
	foreign.GoalID = "goal-foreign"
	foreign.PendingInteraction = map[string]any{
		"status": "waiting_confirmation", "interaction_id": "interaction-s3h8-foreign",
		"requests": []any{map[string]any{
			"id": "interaction-s3h8-foreign", "kind": "mix_tick_confirmation",
			"payload": map[string]any{"observation_id": "obs_other_round@2", "track_id": "1032"},
		}},
	}
	s.mu.Lock()
	s.durableContinuations[foreign.ContinuationID] = foreign
	s.mu.Unlock()
	_, _ = s.handlePendingMixTickChat(context.Background(), "conversation-s3h8", ChatRequest{
		ConversationID: "conversation-s3h8", Message: "取消",
	}, agentModeDefault)
	if item, ok := s.durableContinuations[foreign.ContinuationID]; !ok || item.Status != ContinuationWaitingInteraction {
		t.Fatalf("a foreign candidate's park was closed by this confirmation: %+v", item)
	}
}

func TestPendingInteractionFromResultCarriesInteractionID(t *testing.T) {
	res := agentloop.Result{
		GoalID: "goal-c", RunID: "run-c", TaskID: "task-c", SliceID: "slice-park", TurnID: "turn-park",
		Status: agentruntime.StatusWaitingConfirmation, StopReason: "improvement_proposal_confirmation_required",
		Executed: []map[string]any{{
			"result": map[string]any{
				"interaction_requests": []any{map[string]any{
					"id": "interaction-res-1", "kind": "improvement_proposal_confirmation", "status": "waiting_for_user",
				}},
			},
		}},
	}
	pending := pendingInteractionFromResult(res)
	if pending == nil || firstStringFromMap(pending, "interaction_id") != "interaction-res-1" {
		t.Fatalf("pending interaction lost the id completePendingInteractionContinuation matches on: %+v", pending)
	}
}

func TestInteractionBoundaryChatResponse(t *testing.T) {
	if !interactionBoundaryChatResponse(ChatResponse{NeedsConfirmation: true}) ||
		!interactionBoundaryChatResponse(ChatResponse{GoalStatus: string(agentruntime.StatusWaitingConfirmation)}) ||
		!interactionBoundaryChatResponse(ChatResponse{GoalStatus: string(agentruntime.StatusWaitingClarification)}) {
		t.Fatal("an interaction boundary response was not recognized")
	}
	if interactionBoundaryChatResponse(ChatResponse{GoalStatus: string(agentruntime.StatusWaitingContinue)}) {
		t.Fatal("waiting_continue was misclassified as an interaction boundary")
	}
}
