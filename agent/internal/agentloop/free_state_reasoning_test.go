package agentloop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
)

type freeStateTestExecutor struct {
	calls []planner.ToolCall
}

type freeStateCatalogTestExecutor struct {
	calls []planner.ToolCall
}

func (e *freeStateCatalogTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall)
	return executorpkg.Result{
		ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "ccb_observation_catalog", Status: "ok",
		Result: map[string]any{"status": "ready", "catalog": map[string]any{
			"schema_version": "ccb_observation_catalog.v1", "catalog_version": "v1", "status": "ready",
			"views": []any{map[string]any{"view_id": "project.structure", "availability": "ready"}},
		}},
	}, nil
}

type rejectedFreeStateTestExecutor struct {
	calls []planner.ToolCall
}

func (e *rejectedFreeStateTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall)
	views := messageLoopStringList(in.ToolCall.Args["view_ids"])
	return executorpkg.Result{
		ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "ccb_observation_request", Status: "rejected",
		Result: map[string]any{"status": "rejected", "bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "bundle_id": "bundle-rejected", "request_id": "request-rejected",
			"status": "rejected", "read_only": true, "mutation_authority": false, "observation_id": "obs-rejected",
			"requested_views": views, "views": map[string]any{}, "freshness": map[string]any{"status": "rejected"},
			"omission_reasons": []any{"requested view set is deferred"},
			"audit_receipt": map[string]any{
				"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr-rejected", "status": "rejected",
				"rejection_reasons": []any{"requested view set is deferred"},
			},
		}},
	}, nil
}

func (e *freeStateTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	e.calls = append(e.calls, in.ToolCall)
	return executorpkg.Result{
		ToolCallID:  in.ToolCall.ID,
		Tool:        in.ToolCall.Tool,
		CommandName: "ccb_observation_request",
		Status:      "ok",
		Result: map[string]any{
			"status": "ready",
			"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready",
				"read_only": true, "mutation_authority": false,
				"observation_id": "obs-after", "views": map[string]any{
					"comparison.before_after": map[string]any{"status": "ready"},
				},
			},
		},
	}, nil
}

func TestFreeStateShortChainObservationThenAction(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Inspect bounded dynamics.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need current dynamics evidence.","requested_view_ids":["track.time_dynamics"]},"tool_calls":[{"id":"ccb-selection","tool":"ccb.observation_request","args":{"view_ids":["track.time_dynamics"]}}]}`,
		`{"final":true,"reply":"Use compression next.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"Current bounded dynamics evidence supports stabilization.","remaining_intent":"stabilize the selected vocal","processor_type":"compressor"},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 3, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}
	result := loop.Start(context.Background(), Input{
		UserText: "stabilize the selected vocal",
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"decision_phase": "processor_selection", "original_intent": "stabilize the selected vocal",
		}},
		AllowedTools: []string{"ccb.observation_request"},
	})
	if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateNeedsAction || result.FreeStateDecision.ProcessorType != "compressor" {
		t.Fatalf("short chain did not reach needs_action: %+v", result)
	}
	if len(executor.calls) != 1 || executor.calls[0].Tool != "ccb.observation_request" {
		t.Fatalf("short chain observation calls = %+v", executor.calls)
	}
	if len(client.calls) != 2 {
		t.Fatalf("short chain model calls = %d, want 2", len(client.calls))
	}
}

func TestFreeStateCatalogDiscoveryAllowsEmptyRequestedViewIDs(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Discover the catalog.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need the available neutral view IDs.","requested_view_ids":[]},"tool_calls":[{"id":"ccb-catalog","tool":"ccb.observation_catalog","args":{},"reason":"Discover available views."}]}`,
		`{"final":false,"reply":"Inspect the project.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need project structure.","requested_view_ids":["project.structure"]},"tool_calls":[{"id":"ccb-project","tool":"ccb.observation_request","args":{"view_ids":["project.structure"]}}]}`,
		`{"final":true,"reply":"No further test action.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"The test observation path is complete.","limitations":["test boundary"]},"tool_calls":[]}`,
	}}
	executor := &freeStateCatalogTestExecutor{}
	loop := MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 1},
	}
	result := loop.Start(context.Background(), Input{
		UserText: "inspect the project", AllowedTools: []string{"ccb.observation_catalog", "ccb.observation_request"},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect the project",
		}},
	})
	if len(executor.calls) == 0 || executor.calls[0].Tool != "ccb.observation_catalog" {
		t.Fatalf("catalog-only turn was not executed: %+v", executor.calls)
	}
	if result.StopReason == StopReasonFailed || strings.Contains(result.Error, "requires requested_view_ids") {
		t.Fatalf("catalog-only turn was rejected by view-set validation: %+v", result)
	}
}

func TestFreeStateObservationRequestStillRequiresNonEmptyRequestedViewIDs(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect the project",
	}}}}
	out := messageLoopOutput{Final: false, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "inspect",
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []any{}}}}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "at least one identifier") {
		t.Fatalf("empty concrete observation request was accepted: %q", issue)
	}
}

func TestFreeStateLoopRequiresCCBObservationBeforePostActionSatisfaction(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"已经满足。","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"听起来已经好了"},"tool_calls":[]}`,
		`{"final":false,"reply":"读取动作后的变化。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"尚无动作后证据","requested_view_ids":["comparison.before_after"]},"tool_calls":[{"id":"ccb-after","tool":"ccb.observation_request","args":{"view_ids":["comparison.before_after"]}}]}`,
		`{"final":true,"reply":"原始目标已经满足。","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"动作后证据覆盖原始目标","observation_id":"obs-after"},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 5, MaxToolCalls: 2, MaxConsecutiveErrors: 2}}
	result := loop.Start(context.Background(), Input{
		UserText: "让人声更稳定、更靠前", Summary: "让人声更稳定、更靠前",
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating",
			"original_intent": "让人声更稳定、更靠前", "requires_post_action_observation": true,
		}},
		AllowedTools: []string{"ccb.observation_request"},
	})
	if result.Status != agentruntime.StatusCompleted || result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateSatisfied {
		t.Fatalf("result = %+v", result)
	}
	if len(executor.calls) != 1 || executor.calls[0].Tool != "ccb.observation_request" {
		t.Fatalf("CCB calls = %+v", executor.calls)
	}
	if len(client.calls) != 3 {
		t.Fatalf("model call count = %d, want gate retry + observation + completion", len(client.calls))
	}
}

func TestFreeStateLoopRequiresCCBObservationBeforeAnotherPostActionProcessor(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Use EQ next.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"Presence still needs work.","remaining_intent":"bring the vocal forward","processor_type":"eq"},"tool_calls":[]}`,
		`{"final":false,"reply":"Read the post-action state first.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Fresh post-action evidence is required.","requested_view_ids":["comparison.before_after"]},"tool_calls":[{"id":"ccb-after","tool":"ccb.observation_request","args":{"view_ids":["comparison.before_after"]}}]}`,
		`{"final":true,"reply":"Use EQ next.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"Fresh evidence supports a separate tonal action.","remaining_intent":"bring the vocal forward","processor_type":"eq"},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 5, MaxToolCalls: 2, MaxConsecutiveErrors: 2}}
	result := loop.Start(context.Background(), Input{
		UserText: "make the vocal steadier and more forward",
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating",
			"decision_phase": "post_action_evaluation", "original_intent": "make the vocal steadier and more forward",
			"requires_post_action_observation": true,
		}},
		AllowedTools: []string{"ccb.observation_request"},
	})
	if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateNeedsAction {
		t.Fatalf("result = %+v", result)
	}
	if len(executor.calls) != 1 || executor.calls[0].Tool != "ccb.observation_request" {
		t.Fatalf("CCB calls = %+v", executor.calls)
	}
	if len(client.calls) != 3 {
		t.Fatalf("model call count = %d, want gate retry + observation + action", len(client.calls))
	}
}

func TestFreeStateTransientLLMFailurePausesAndResumesPostActionEvaluation(t *testing.T) {
	client := &fakeMessageCompleter{
		responses: []string{
			`{"final":false,"reply":"读取动作后的变化。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"需要动作后证据","requested_view_ids":["comparison.before_after"]},"tool_calls":[{"id":"ccb-after","tool":"ccb.observation_request","args":{"view_ids":["comparison.before_after"]}}]}`,
			`{"final":true,"reply":"原始目标已经满足。","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"动作后证据覆盖原始目标","observation_id":"obs-after"},"tool_calls":[]}`,
		},
		errors: []error{nil, fmt.Errorf("LLM HTTP error 502: error code: 502")},
	}
	executor := &freeStateTestExecutor{}
	runtime := agentruntime.New()
	loop := MessageLoop{
		Runtime:  runtime,
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	originalIntent := "让人声更稳定、更靠前"
	result := loop.Start(context.Background(), Input{
		UserText: originalIntent,
		Summary:  originalIntent,
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version":                   "free_state_reasoning_loop.v1",
			"status":                           "re_evaluating",
			"decision_phase":                   "post_action_evaluation",
			"original_intent":                  originalIntent,
			"requires_post_action_observation": true,
		}},
		AllowedTools: []string{"ccb.observation_request"},
	})
	if result.Status != agentruntime.StatusWaitingContinue || result.StopReason != StopReasonTransientLLMError {
		t.Fatalf("paused result = status=%q stop=%q reply=%q error=%q", result.Status, result.StopReason, result.Reply, result.Error)
	}
	if result.Continuation == nil || result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateNeedsObservation {
		t.Fatalf("transient failure discarded active continuation: %+v", result)
	}
	if result.Continuation.TurnsUsed != 1 {
		t.Fatalf("transient service failure consumed a reasoning turn: turns=%d, want 1 completed model turn", result.Continuation.TurnsUsed)
	}
	loopContext := messageLoopMapValue(result.Continuation.Context["free_state_reasoning_loop"])
	if firstMapText(loopContext, "original_intent") != originalIntent {
		t.Fatalf("original intent was not retained: %+v", loopContext)
	}
	if result.Continuation.RecentObservation == nil || !messageLoopCCBObservationBundleUsable(result.Continuation.RecentObservation.Summary) {
		t.Fatalf("fresh CCB evidence was not retained in continuation: %+v", result.Continuation.RecentObservation)
	}
	continuationState := &runState{
		goal: agentruntime.Goal{GoalID: result.Continuation.GoalID, RunID: result.Continuation.RunID},
		input: Input{
			UserText: result.Continuation.UserText, Summary: result.Continuation.Summary,
			Context: result.Continuation.Context, ContextSnapshot: result.Continuation.ContextSnapshot,
		},
		trace:             append([]planner.TraceEvent(nil), result.Continuation.Trace...),
		contextSnapshot:   cloneMap(result.Continuation.ContextSnapshot),
		recentObservation: cloneRecentObservation(result.Continuation.RecentObservation),
	}
	continuationModel := (&Runner{}).buildModelContextSnapshot(continuationState, (&Runner{}).buildContextSnapshot(continuationState))
	if strings.Count(continuationModel, `"schema_version":"ccb_model_projection.v1"`) != 1 {
		t.Fatalf("continuation rehydrated duplicate CCB projection: %s", continuationModel)
	}
	if runtime.Status(result.GoalID).Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("runtime status = %q, want waiting_continue", runtime.Status(result.GoalID).Status)
	}

	resumed := loop.Continue(context.Background(), *result.Continuation)
	if resumed.Status != agentruntime.StatusCompleted || resumed.FreeStateDecision == nil || resumed.FreeStateDecision.Status != FreeStateSatisfied {
		t.Fatalf("resumed result = status=%q stop=%q decision=%+v reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.FreeStateDecision, resumed.Reply, resumed.Error)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("resume repeated an already completed observation: %+v", executor.calls)
	}
}

func TestFreeStateRejectedViewSetIsNotRetriedAcrossContinuation(t *testing.T) {
	viewJSON := `["mix.masking_relationship","track.basic_energy"]`
	request := func(callID string) string {
		return `{"final":false,"reply":"Inspect masking.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need masking evidence.","requested_view_ids":` + viewJSON + `},"tool_calls":[{"id":"` + callID + `","tool":"ccb.observation_request","args":{"view_ids":` + viewJSON + `}}]}`
	}
	client := &fakeMessageCompleter{
		responses: []string{
			request("ccb-first"), request("ccb-retry-after-continue"),
			`{"final":true,"reply":"The rejected view set cannot be retried.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"The rejected view set cannot be retried.","stop_reason":"ccb_view_set_rejected","limitations":["No equivalent view is available."]},"tool_calls":[]}`,
		},
		errors: []error{nil, fmt.Errorf("LLM HTTP error 502: error code: 502")},
	}
	executor := &rejectedFreeStateTestExecutor{}
	runtime := agentruntime.New()
	loop := MessageLoop{
		Runtime: runtime, Client: client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}
	paused := loop.Start(context.Background(), Input{
		UserText: "inspect masking", AllowedTools: []string{"ccb.observation_request"},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "decision_phase": "processor_selection",
			"original_intent": "inspect masking",
		}},
	})
	if paused.Status != agentruntime.StatusWaitingContinue || paused.Continuation == nil || len(executor.calls) != 1 {
		t.Fatalf("rejected sequence did not pause with one execution: status=%q continuation=%v calls=%d", paused.Status, paused.Continuation != nil, len(executor.calls))
	}
	loopContext := messageLoopMapValue(paused.Continuation.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loopContext["observation_ledger"])
	if len(messageLoopMapRows(ledger["rejected_view_sets"])) != 1 {
		t.Fatalf("continuation lost rejection ledger: %#v", loopContext)
	}
	resumed := loop.Continue(context.Background(), *paused.Continuation)
	if resumed.FreeStateDecision == nil || resumed.FreeStateDecision.Status != FreeStateBlocked || len(executor.calls) != 1 {
		t.Fatalf("continuation retried rejected CCB set: decision=%+v calls=%d result=%+v", resumed.FreeStateDecision, len(executor.calls), resumed)
	}
}

func TestFreeStateNeedsActionCarriesRemainderWithoutToolAuthority(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"先处理动态。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_action","evidence_status":"sufficient","summary":"宏观动态不稳定","remaining_intent":"先稳定人声宏观动态，同时保留靠前目标","processor_type":"compressor"},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1}}
	result := loop.Start(context.Background(), Input{
		UserText: "让人声更稳定、更靠前",
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "让人声更稳定、更靠前",
		}},
		RecentObservation: semanticGuidanceUsableCCBObservation("ready"),
	})
	if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateNeedsAction || result.FreeStateDecision.RemainingIntent == "" {
		t.Fatalf("free-state handoff = %+v", result.FreeStateDecision)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("needs_action gained direct tool authority: %+v", executor.calls)
	}
}

func TestFreeStateHandoffStatusesAcceptFinalFalseWithoutToolCalls(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "make the vocal steadier and more forward",
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	tests := []struct {
		name     string
		decision FreeStateDecision
	}{
		{name: FreeStateNeedsAction, decision: FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "compression is supported", RemainingIntent: "stabilize the vocal", ProcessorType: "compressor",
		}},
		{name: FreeStateSatisfied, decision: FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateSatisfied,
			EvidenceStatus: "sufficient", Summary: "the original intent is satisfied",
		}},
		{name: FreeStateBlocked, decision: FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked,
			EvidenceStatus: "insufficient", Summary: "no governed processor is available", StopReason: "capability boundary",
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := messageLoopOutput{Final: false, FreeStateDecision: &test.decision}
			if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
				t.Fatalf("final=false %s decision was rejected: %s", test.name, issue)
			}
		})
	}
}

func TestFreeStateNeedsActionRequiresStructuredProcessorType(t *testing.T) {
	decision := FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction,
		EvidenceStatus: "sufficient", Summary: "frequency treatment is next", RemainingIntent: "bring the vocal forward",
	}
	if err := decision.Validate(); err == nil || !strings.Contains(err.Error(), "processor_type") {
		t.Fatalf("needs_action without processor_type was accepted: %v", err)
	}
	decision.ProcessorType = "eq"
	if err := decision.Validate(); err != nil {
		t.Fatalf("structured EQ handoff was rejected: %v", err)
	}
}

func TestFreeStateNeedsExperimentCarriesImprovementProposalWithoutProcessorRequirement(t *testing.T) {
	decision := FreeStateDecision{
		SchemaVersion:  FreeStateDecisionSchema,
		Status:         FreeStateNeedsExperiment,
		EvidenceStatus: "plausible",
		Summary:        "Evidence supports a bounded improvement hypothesis.",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion:     agentprotocol.ImprovementProposalSchema,
			Target:            map[string]any{"kind": "clip", "id": "clip_1"},
			EvidenceRefs:      []string{"obs_1"},
			ImprovementIntent: "让片段电平更自然",
			Hypothesis:        "小幅 clip gain 调整可能改善段落衔接",
			ExpectedEffect:    "段落间响度过渡更平滑",
			ActionDomain:      agentprotocol.ImprovementActionDomainClipGain,
			ActionKind:        "bounded_gain_adjustment",
			Confidence:        0.55,
		},
	}
	if err := decision.Validate(); err != nil {
		t.Fatalf("needs_experiment should validate without processor_type: %v", err)
	}
	if decision.ProcessorType != "" {
		t.Fatalf("generic improvement proposal should not require processor_type: %+v", decision)
	}
}

func TestFreeStateDiagnosticOnlyRequiresEvidenceBackedTerminalConclusion(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"diagnostic_only": true, "original_intent": "inspect the whole project",
		},
	}}, recentObservation: &RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{
		"status": "ready", "observation_id": "obs-diagnostic", "evidence_refs": []any{"evidence://mix"},
	}}}
	confirmed := &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateSatisfied, EvidenceStatus: "sufficient", Summary: "one bounded diagnosis",
		Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "confirmed", Findings: []FreeStateDiagnosticFinding{{
			Statement: "the returned evidence supports a measurable project-level difference", Scope: map[string]any{"kind": "project", "ids": []any{"current"}}, EvidenceRefs: []string{"obs-diagnostic", "evidence://mix"}, Confidence: 0.8,
		}}},
	}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: confirmed}); issue != "" {
		t.Fatalf("valid diagnostic-only conclusion rejected: %s", issue)
	}
	confirmed.Diagnostic.Findings[0].EvidenceRefs = []string{"sealed-expected-issue"}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: confirmed}); !strings.Contains(issue, "evidence_refs") {
		t.Fatalf("unknown diagnostic evidence ref accepted: %q", issue)
	}
	noConclusion := &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateSatisfied, EvidenceStatus: "sufficient", Summary: "done"}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: noConclusion}); !strings.Contains(issue, "diagnostic") {
		t.Fatalf("missing diagnostic conclusion accepted: %q", issue)
	}
	needsAction := &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient", Summary: "apply", RemainingIntent: "apply", ProcessorType: "compressor"}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: needsAction}); !strings.Contains(issue, "diagnostic-only") {
		t.Fatalf("diagnostic-only action accepted: %q", issue)
	}
}

func TestFreeStateDiagnosticOnlyRejectsTransportAndAuditIDsAsFindingEvidence(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true,
			"observation_ids": []any{"obs-ledger"},
			"observation_ledger": map[string]any{
				"schema_version": freeStateObservationLedgerSchema,
				"receipts": []any{map[string]any{
					"status": "ready", "observation_id": "obs-ledger", "evidence_refs": []any{"evidence://mix"},
					"tool_call_id": "call-hidden", "request_id": "request-hidden", "receipt_id": "receipt-hidden",
				}},
			},
		},
	}}}
	decision := &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateSatisfied, EvidenceStatus: "sufficient", Summary: "bounded finding",
		Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "confirmed", Findings: []FreeStateDiagnosticFinding{{
			Statement: "the visible evidence supports a bounded finding", EvidenceRefs: []string{"obs-ledger", "evidence://mix"}, Confidence: 0.7,
		}}},
	}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: decision}); issue != "" {
		t.Fatalf("visible observation/evidence refs rejected: %q", issue)
	}
	for _, hidden := range []string{"call-hidden", "request-hidden", "receipt-hidden"} {
		decision.Diagnostic.Findings[0].EvidenceRefs = []string{hidden}
		if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, FreeStateDecision: decision}); !strings.Contains(issue, "evidence_refs") {
			t.Fatalf("transport/audit ref %q was accepted: %q", hidden, issue)
		}
	}
}

func TestFreeStateDiagnosticOnlyShortReplayStopsBeforeAction(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Inspect the measurable project evidence.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need one current project observation.","requested_view_ids":["project.structure"]},"tool_calls":[{"id":"diag-observe","tool":"ccb.observation_request","args":{"view_ids":["project.structure"]}}]}`,
		`{"final":true,"reply":"The evidence supports one bounded finding.","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"Diagnostic conclusion complete.","diagnostic":{"schema_version":"free_state_diagnostic.v1","status":"confirmed","findings":[{"statement":"The returned project evidence contains a measurable structural state.","scope":{"kind":"project","ids":["current"]},"evidence_refs":["obs-after"],"confidence":0.8}]}},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"}, Executor: executor, Budget: Budget{MaxTurns: 3, MaxToolCalls: 1, MaxConsecutiveErrors: 1}}
	result := loop.Start(context.Background(), Input{
		UserText: "inspect the whole project", AllowedTools: []string{"ccb.observation_request"},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true, "original_intent": "inspect the whole project",
		}},
	})
	if result.Status != agentruntime.StatusCompleted || result.FreeStateDecision == nil || result.FreeStateDecision.Diagnostic == nil {
		t.Fatalf("diagnostic-only replay did not complete: status=%q decision=%+v error=%q", result.Status, result.FreeStateDecision, result.Error)
	}
	if result.FreeStateDecision.Diagnostic.Status != "confirmed" || len(executor.calls) != 1 || result.FreeStateDecision.ProcessorType != "" {
		t.Fatalf("diagnostic-only replay crossed action boundary: decision=%+v calls=%+v", result.FreeStateDecision, executor.calls)
	}
}

func TestFreeStateDiagnosticOnlyClosesObservationWindowAfterUsableNonStructuralBundle(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true,
		},
	}}, recentObservation: &RecentObservation{
		Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready", "read_only": true, "mutation_authority": false,
			"observation_id": "obs-mix", "views": map[string]any{
				"mix.multitrack_relationship": map[string]any{"status": "ready"},
			},
		},
	}}
	decision := messageLoopOutput{Final: false, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "request another view", RequestedViewIDs: []string{"mix.frequency_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"mix.frequency_relationship"},
	}}}}
	issue := messageLoopFreeStateOutputIssue(state, decision)
	if !strings.Contains(issue, "observation window is closed") {
		t.Fatalf("diagnostic observation window stayed open: %q", issue)
	}
}

func TestFreeStateDiagnosticOnlyAllowsStructureDiscoveryBeforeDiagnosticBundle(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true,
		},
	}}, recentObservation: &RecentObservation{
		Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready", "read_only": true, "mutation_authority": false,
			"observation_id": "obs-structure", "views": map[string]any{
				"project.structure": map[string]any{"status": "ready"},
			},
		},
	}}
	decision := messageLoopOutput{Final: false, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "request project relationship evidence", RequestedViewIDs: []string{"mix.multitrack_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"mix.multitrack_relationship"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); issue != "" {
		t.Fatalf("structure discovery prematurely closed diagnostic observation window: %q", issue)
	}
}

func TestFreeStateDiagnosticOnlyRestoresClosedWindowFromObservationLedger(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true,
			"observation_ledger": map[string]any{
				"schema_version": freeStateObservationLedgerSchema,
				"available_views": map[string]any{
					"mix.frequency_relationship": map[string]any{"status": "partial", "observation_id": "obs-restored"},
				},
			},
		},
	}}}
	if !messageLoopFreeStateDiagnosticEvidenceWindowClosed(state) {
		t.Fatal("continuation ledger did not restore the closed diagnostic observation window")
	}
}

func TestFreeStateDiagnosticOnlyFinalGatePreventsSecondObservationAndAcceptsUnresolved(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Inspect one relationship view.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need one bounded relationship observation.","requested_view_ids":["comparison.before_after"]},"tool_calls":[{"id":"diag-first","tool":"ccb.observation_request","args":{"view_ids":["comparison.before_after"]}}]}`,
		`{"final":false,"reply":"Inspect another view.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Want more evidence.","requested_view_ids":["mix.frequency_relationship"]},"tool_calls":[{"id":"diag-second","tool":"ccb.observation_request","args":{"view_ids":["mix.frequency_relationship"]}}]}`,
		`{"final":true,"reply":"The available evidence cannot establish a project-wide issue.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"The bounded evidence is not sufficient for a project-wide diagnosis.","limitations":["Only one relationship view was observed."],"diagnostic":{"schema_version":"free_state_diagnostic.v1","status":"unresolved","limitations":["Only one relationship view was observed."]}},"tool_calls":[]}`,
	}}
	executor := &freeStateTestExecutor{}
	loop := MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Executor: executor, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 1},
	}
	result := loop.Start(context.Background(), Input{
		UserText: "inspect the whole project", AllowedTools: []string{"ccb.observation_request"},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "diagnostic_only": true,
		}},
	})
	if len(executor.calls) != 1 {
		t.Fatalf("diagnostic final gate executed %d observations, want exactly one: %+v", len(executor.calls), executor.calls)
	}
	if result.Status != agentruntime.StatusCompleted || result.FreeStateDecision == nil || result.FreeStateDecision.Diagnostic == nil || result.FreeStateDecision.Diagnostic.Status != "unresolved" {
		t.Fatalf("diagnostic unresolved conclusion was not accepted: %+v", result)
	}
	gateSeen := false
	for _, messages := range client.calls {
		for _, message := range messages {
			if strings.Contains(message.Content, "observation window is closed") {
				gateSeen = true
			}
		}
	}
	if len(client.calls) != 3 || !gateSeen {
		t.Fatalf("model did not receive the structured final gate: calls=%d last=%+v", len(client.calls), client.calls)
	}
}

func TestFreeStateAllowsOneFreshEvidenceRepeatButBoundsSameFamily(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating", "decision_phase": "post_action_evaluation",
			"original_intent": "make the vocal steadier and clearer",
			"actions":         []map[string]any{{"cycle": 1, "processor_type": "compressor", "status": "applied"}},
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "try compression again", RemainingIntent: "make the vocal steadier", ProcessorType: "compressor",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("one evidence-grounded compressor repeat was rejected: %q", issue)
	}
	out.FreeStateDecision.ProcessorType = "eq"
	out.FreeStateDecision.Summary = "address the independent tonal remainder"
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("independent EQ remainder was rejected: %q", issue)
	}
	state.input.Context["free_state_reasoning_loop"].(map[string]any)["actions"] = []map[string]any{
		{"cycle": 1, "processor_type": "compressor", "status": "applied"},
		{"cycle": 2, "processor_type": "compressor", "status": "applied"},
	}
	out.FreeStateDecision.ProcessorType = "compressor"
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "bounded 2-action limit") {
		t.Fatalf("third compressor action escaped the same-family bound: %q", issue)
	}
}

func TestFreeStatePostActionInconclusiveEvidenceBlocksFurtherWrites(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating", "decision_phase": "post_action_evaluation",
			"original_intent": "reduce vocal sibilance",
			"actions":         []map[string]any{{"cycle": 1, "processor_type": "de_esser", "status": "applied"}},
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "inconclusive",
		Summary: "the fresh result is not decisive", RemainingIntent: "reduce sibilance", ProcessorType: "de_esser",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "inconclusive") {
		t.Fatalf("inconclusive post-action evidence authorized another write: %q", issue)
	}
}

func TestFreeStatePostActionRequiresObservationExecutedInCurrentTurn(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating", "decision_phase": "post_action_evaluation",
			"original_intent": "reduce vocal sibilance", "requires_post_action_observation": true,
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "reduce the remaining sibilance", RemainingIntent: "reduce sibilance", ProcessorType: "de_esser",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "fresh model-requested CCB observation_request") {
		t.Fatalf("stale prior-turn observation authorized an action: %q", issue)
	}
	state.executed = []map[string]any{{
		"tool": "ccb.observation_request", "status": "ok",
		"result": map[string]any{"bundle": map[string]any{
			"status": "ready", "read_only": true, "mutation_authority": false,
			"views": map[string]any{"track.frequency_time_events": map[string]any{"status": "ready"}},
		}},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("current-turn successful CCB observation was rejected: %q", issue)
	}
}

func TestFreeStateTotalActionBoundBlocksSeventhAction(t *testing.T) {
	actions := []map[string]any{
		{"cycle": 1, "processor_type": "eq", "status": "applied"},
		{"cycle": 2, "processor_type": "compressor", "status": "applied"},
		{"cycle": 3, "processor_type": "limiter", "status": "applied"},
		{"cycle": 4, "processor_type": "gate_expander", "status": "applied"},
		{"cycle": 5, "processor_type": "de_esser", "status": "applied"},
		{"cycle": 6, "processor_type": "transient_shaper", "status": "applied"},
	}
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating", "decision_phase": "post_action_evaluation",
			"original_intent": "finish the bounded treatment", "actions": actions,
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "apply the final correction", RemainingIntent: "apply the final correction", ProcessorType: "multiband_dynamics",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "bounded 6-action limit") {
		t.Fatalf("seventh action escaped total bound: %q", issue)
	}
}

func TestFreeStateRejectsProcessorWithoutGovernedOpenSemanticPath(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "avoid over-compressing the vocal",
		},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "add a reverb", RemainingIntent: "avoid over-compression", ProcessorType: "reverb",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "only eq, compressor, limiter") {
		t.Fatalf("unsupported reverb action was accepted: %q", issue)
	}
	for _, processorType := range []string{"eq", "compressor", "limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics"} {
		out.FreeStateDecision.ProcessorType = processorType
		if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
			t.Fatalf("governed processor %s was rejected: %q", processorType, issue)
		}
	}
}

func TestFreeStateBlockedSummaryIsSufficientBoundaryReason(t *testing.T) {
	decision := FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked,
		EvidenceStatus: "insufficient", Summary: "micro-transient evidence is unavailable",
	}
	if err := decision.Validate(); err != nil {
		t.Fatalf("summary-only blocked boundary was rejected: %v", err)
	}
}

func TestFreeStateBlockedBypassesLegacyBroadMixFinalGate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Dynamic treatment is blocked by the available evidence.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"No governed evidence path can resolve the required detail."},"tool_calls":[]}`,
	}}
	loop := MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}
	result := loop.Start(context.Background(), Input{
		UserText: "make the vocal dynamics more controlled while preserving transients",
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "make the vocal dynamics more controlled while preserving transients",
		}},
	})
	if result.Status != agentruntime.StatusCompleted || result.StopReason != StopReasonDone {
		t.Fatalf("blocked terminal result = status=%q stop=%q reply=%q", result.Status, result.StopReason, result.Reply)
	}
	if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateBlocked {
		t.Fatalf("blocked decision was not retained: %+v", result.FreeStateDecision)
	}
	if len(client.calls) != 1 {
		t.Fatalf("legacy broad-mix final gate retried a terminal decision: calls=%d", len(client.calls))
	}
}

func TestFreeStateRejectsFalseObservationUnavailableBoundaryWhenCCBIsAvailable(t *testing.T) {
	state := &runState{input: Input{
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "decision_phase": "processor_selection",
			"original_intent": "control occasional drum peaks without flattening the attack",
		}},
		AllowedTools: []string{"ccb.observation_catalog", "ccb.observation_request"},
	}}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked, EvidenceStatus: "insufficient",
		Summary: "\u6240\u9700\u89c2\u5bdf\u63a5\u53e3\u5f53\u524d\u4e0d\u53ef\u7528\uff0c\u65e0\u6cd5\u53ef\u9760\u51b3\u5b9a\u538b\u7f29\u5904\u7406\u3002",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "ccb.observation_request") {
		t.Fatalf("false observation boundary was accepted: %q", issue)
	}

	out.FreeStateDecision.Summary = "No qualified broadband compressor is available on the target track."
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("real processor capability boundary was rejected: %q", issue)
	}
}

func TestFreeStateObservationProtocolCoercesLegacyOrMissingToolCallToCCB(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "stabilize the vocal",
		},
	}}}
	for _, calls := range [][]planner.ToolCall{
		nil,
		{{ID: "legacy-observe", Tool: "mix.observe", Args: map[string]any{"duration_seconds": 20}, Reason: "inspect dynamics"}},
	} {
		out := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "need dynamics", RequestedViewIDs: []string{"track.time_dynamics"},
		}, ToolCalls: calls}
		coerced := coerceMessageLoopFreeStateObservationOutput(state, out)
		if len(coerced.ToolCalls) != 1 || coerced.ToolCalls[0].Tool != "ccb.observation_request" ||
			len(nonEmptyFreeStateStrings(messageLoopStringSlice(coerced.ToolCalls[0].Args["view_ids"]))) != 1 {
			t.Fatalf("structured observation request was not coerced to CCB: %#v", coerced.ToolCalls)
		}
		if issue := messageLoopFreeStateOutputIssue(state, coerced); issue != "" {
			t.Fatalf("coerced CCB request was rejected: %s", issue)
		}
	}
}

func TestFreeStateObservationRequestViewSetMustExactlyMatchModelDecision(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect the track",
	}}}}
	base := []string{"track.time_dynamics", "mix.multitrack_relationship"}
	tests := []struct {
		name      string
		decision  []string
		callViews any
		wantIssue bool
	}{
		{name: "exact", decision: base, callViews: []any{"track.time_dynamics", "mix.multitrack_relationship"}},
		{name: "reordered set", decision: base, callViews: []string{"mix.multitrack_relationship", "track.time_dynamics"}},
		{name: "added", decision: base, callViews: []string{"track.time_dynamics", "mix.multitrack_relationship", "track.timbre_frequency"}, wantIssue: true},
		{name: "removed", decision: base, callViews: []string{"track.time_dynamics"}, wantIssue: true},
		{name: "replaced", decision: base, callViews: []string{"track.time_dynamics", "track.timbre_frequency"}, wantIssue: true},
		{name: "duplicate call", decision: base, callViews: []string{"track.time_dynamics", "track.time_dynamics"}, wantIssue: true},
		{name: "duplicate decision", decision: []string{"track.time_dynamics", "track.time_dynamics"}, callViews: []string{"track.time_dynamics"}, wantIssue: true},
		{name: "non-array call", decision: base, callViews: "track.time_dynamics,mix.multitrack_relationship", wantIssue: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
				SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation,
				EvidenceStatus: "insufficient", Summary: "model-selected evidence", RequestedViewIDs: test.decision,
			}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": test.callViews}}}}
			issue := messageLoopFreeStateOutputIssue(state, out)
			if test.wantIssue && issue == "" {
				t.Fatal("mismatched or ambiguous view set was accepted")
			}
			if !test.wantIssue && issue != "" {
				t.Fatalf("exact model-owned view set was rejected: %s", issue)
			}
		})
	}
}

func TestFreeStateInitialActionRequiresUsableCCBBundle(t *testing.T) {
	decision := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "evidence supports EQ", RemainingIntent: "reduce harshness", ProcessorType: "eq",
	}}
	tests := []struct {
		name        string
		observation *RecentObservation
		wantIssue   bool
	}{
		{name: "missing", wantIssue: true},
		{name: "ready", observation: semanticGuidanceUsableCCBObservation("ready")},
		{name: "partial", observation: semanticGuidanceUsableCCBObservation("partial")},
		{name: "failed execution", observation: func() *RecentObservation {
			row := semanticGuidanceUsableCCBObservation("ready")
			row.Status = "failed"
			return row
		}(), wantIssue: true},
		{name: "empty views", observation: func() *RecentObservation {
			row := semanticGuidanceUsableCCBObservation("ready")
			row.Summary["views"] = map[string]any{}
			return row
		}(), wantIssue: true},
		{name: "not read only", observation: func() *RecentObservation {
			row := semanticGuidanceUsableCCBObservation("ready")
			row.Summary["read_only"] = false
			return row
		}(), wantIssue: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "reduce harshness",
			}}}, recentObservation: test.observation}
			issue := messageLoopFreeStateOutputIssue(state, decision)
			if test.wantIssue && issue == "" {
				t.Fatal("unusable CCB bundle authorized the first action")
			}
			if !test.wantIssue && issue != "" {
				t.Fatalf("usable CCB bundle was rejected: %s", issue)
			}
		})
	}
}

func TestFreeStateRejectsRetryOfRejectedCCBViewSet(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect the project",
			"rejected_observation_requests": []any{map[string]any{"requested_views": []any{"mix.masking_relationship", "track.peak_structure"}}},
		},
	}}}
	decision := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "retry rejected evidence", RequestedViewIDs: []string{"track.peak_structure", "mix.masking_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"track.peak_structure", "mix.masking_relationship"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); !strings.Contains(issue, "already rejected") {
		t.Fatalf("rejected view retry was accepted: %q", issue)
	}
}

func TestFreeStateRejectedViewCanChooseDifferentViewOrBlock(t *testing.T) {
	state := &runState{input: Input{
		AllowedTools: []string{"ccb.observation_request"},
		Context: map[string]any{"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing",
			"decision_phase": "processor_selection", "original_intent": "inspect the project",
			"rejected_observation_requests": []any{map[string]any{"requested_views": []any{"mix.masking_relationship"}}},
		}},
	}}
	different := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "use an available relationship view", RequestedViewIDs: []string{"mix.frequency_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"mix.frequency_relationship"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, different); issue != "" {
		t.Fatalf("different catalog view was rejected: %q", issue)
	}
	blocked := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked, EvidenceStatus: "insufficient",
		Summary:    "The requested masking view is deferred in the current CCB catalog.",
		StopReason: "ccb_view_deferred", Limitations: []string{"No safe equivalent view answers the masking question."},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, blocked); issue != "" {
		t.Fatalf("explicit deferred-view block was rejected: %q", issue)
	}
}

func TestFreeStateExecutionLedgerBlocksSameRejectedSetWithinRun(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect masking",
		},
	}}}
	rejected := &RecentObservation{
		ToolCallID: "ccb-rejected", Tool: "ccb.observation_request", Status: "rejected",
		Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "observation_id": "obs-rejected",
			"requested_views":  []any{"processor.identity_and_controls", "mix.masking_relationship"},
			"omission_reasons": []any{"deferred"},
			"audit_receipt":    map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr-rejected"},
		},
	}
	recordFreeStateCCBObservation(state, rejected)

	// A later usable observation replaces recentObservation but must not erase
	// the rejection that the guard needs for the next model decision.
	state.recentObservation = &RecentObservation{Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-ready",
		"requested_views": []any{"track.time_dynamics"}, "views": map[string]any{"track.time_dynamics": map[string]any{"status": "ready"}},
	}}
	decision := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "retry", RequestedViewIDs: []string{"mix.masking_relationship", "processor.identity_and_controls"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"mix.masking_relationship", "processor.identity_and_controls"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); !strings.Contains(issue, "already rejected") {
		t.Fatalf("same-run rejected set reached execution guard: %q context=%#v", issue, state.input.Context)
	}
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loop["observation_ledger"])
	if len(messageLoopMapRows(ledger["rejected_view_sets"])) != 1 || len(messageLoopMapRows(loop["rejected_observation_requests"])) != 1 {
		t.Fatalf("same-run ledger was not updated deterministically: %#v", loop)
	}

	different := decision
	different.FreeStateDecision = cloneFreeStateDecision(decision.FreeStateDecision)
	different.FreeStateDecision.RequestedViewIDs = []string{"mix.frequency_relationship"}
	different.ToolCalls = []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []any{"mix.frequency_relationship"}}}}
	if issue := messageLoopFreeStateOutputIssue(state, different); issue != "" {
		t.Fatalf("different view set was blocked: %q", issue)
	}
}

func TestFreeStateLedgerRetainsExactSetRejectionScope(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect"},
	}}}
	rejected := &RecentObservation{ToolCallID: "ccb-scoped", Tool: "ccb.observation_request", Status: "rejected", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "requested_views": []any{"mix.masking_relationship", "mix.frequency_relationship"},
		"rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"}, "non_blocking_view_ids": []any{"mix.frequency_relationship"},
		"audit_receipt": map[string]any{"receipt_id": "receipt-scoped", "rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"}, "non_blocking_view_ids": []any{"mix.frequency_relationship"}},
	}}
	recordFreeStateCCBObservation(state, rejected)
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loop["observation_ledger"])
	rows := messageLoopMapRows(ledger["rejected_view_sets"])
	if len(rows) != 1 || firstMapText(rows[0], "rejection_scope") != "exact_view_set" {
		t.Fatalf("scoped rejection was not retained: %#v", ledger)
	}
	if len(messageLoopStringList(rows[0]["blocking_view_ids"])) != 1 || len(messageLoopStringList(rows[0]["non_blocking_view_ids"])) != 1 {
		t.Fatalf("scoped rejection view metadata was not retained: %#v", rows[0])
	}
}

func TestFreeStateExecutionLedgerIndexesFirstReadyObservation(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect dynamics",
		},
	}}}
	ready := &RecentObservation{
		ToolCallID: "ccb-ready", Tool: "ccb.observation_request", Status: "ok",
		Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "bundle_id": "bundle-ready", "status": "ready", "observation_id": "obs-ready",
			"requested_views": []any{"track.time_dynamics"}, "freshness": map[string]any{"status": "fresh"},
			"views":         map[string]any{"track.time_dynamics": map[string]any{"status": "ready"}},
			"audit_receipt": map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr-ready"},
		},
	}
	recordFreeStateCCBObservation(state, ready)
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loop["observation_ledger"])
	available := messageLoopMapValue(ledger["available_views"])
	if firstMapText(messageLoopMapValue(available["track.time_dynamics"]), "observation_id") != "obs-ready" || len(messageLoopMapRows(ledger["receipts"])) != 1 {
		t.Fatalf("first ready observation was not indexed: %#v", ledger)
	}
}

func TestFreeStateExecutionLedgerKeepsSafeViewConclusion(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect"},
	}}}
	ready := &RecentObservation{ToolCallID: "ccb-conclusion", Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-conclusion",
		"target_ref":      map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
		"requested_views": []any{"track.timbre_frequency"},
		"views": map[string]any{"track.timbre_frequency": map[string]any{
			"status": "ready", "facts": map[string]any{"track.1007.slow.band_energy.summary": map[string]any{
				"status": "ready", "bands": map[string]any{"bass": map[string]any{"energy_db": -14.0}},
				"raw_samples": "must-not-enter", "plugin_id": "must-not-enter",
			}},
		}},
	}}
	recordFreeStateCCBObservation(state, ready)
	ledger := messageLoopMapValue(messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])["observation_ledger"])
	entry := messageLoopMapValue(messageLoopMapValue(ledger["available_views"])["track:1007::track.timbre_frequency"])
	conclusion := messageLoopMapValue(entry["conclusion"])
	data := fmt.Sprint(conclusion)
	if len(conclusion) == 0 || !strings.Contains(data, "energy_db") {
		t.Fatalf("safe view conclusion missing: %#v", entry)
	}
	if strings.Contains(data, "must-not-enter") || strings.Contains(data, "plugin_id") || strings.Contains(data, "raw_samples") {
		t.Fatalf("unsafe view data entered ledger conclusion: %#v", conclusion)
	}
}

func TestFreeStateExecutionLedgerIndexesOnlyUsableViewsFromPartialBundle(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect dynamics",
	}}}}
	partial := &RecentObservation{ToolCallID: "ccb-partial", Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "partial", "observation_id": "obs-partial",
		"requested_views": []any{"track.time_dynamics", "mix.masking_relationship"},
		"views": map[string]any{
			"track.time_dynamics":      map[string]any{"status": "ready"},
			"mix.masking_relationship": map[string]any{"status": "deferred"},
		},
	}}
	recordFreeStateCCBObservation(state, partial)
	ledger := messageLoopMapValue(messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])["observation_ledger"])
	available := messageLoopMapValue(ledger["available_views"])
	if available["track.time_dynamics"] == nil || available["mix.masking_relationship"] != nil {
		t.Fatalf("partial bundle indexed unusable views: %#v", available)
	}
}

func TestFreeStateMessageLoopDoesNotInvokeExecutorForRepeatedRejectedSet(t *testing.T) {
	for _, tc := range []struct {
		name             string
		viewJSON         string
		attemptedRepeats int
	}{
		{name: "smoke_sequence_six_to_one", viewJSON: `["mix.masking_relationship","processor.identity_and_controls"]`, attemptedRepeats: 6},
		{name: "smoke_sequence_two_to_one", viewJSON: `["mix.masking_relationship","track.basic_energy"]`, attemptedRepeats: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := func(callID string) string {
				return `{"final":false,"reply":"Inspect masking.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"Need masking evidence.","requested_view_ids":` + tc.viewJSON + `},"tool_calls":[{"id":"` + callID + `","tool":"ccb.observation_request","args":{"view_ids":` + tc.viewJSON + `}}]}`
			}
			responses := []string{request("ccb-first")}
			for i := 1; i < tc.attemptedRepeats; i++ {
				responses = append(responses, request(fmt.Sprintf("ccb-retry-%d", i)))
			}
			responses = append(responses, `{"final":true,"reply":"The requested view set was rejected, so no retry was made.","free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":"The rejected view set cannot be retried safely.","stop_reason":"ccb_view_set_rejected","limitations":["No different catalog view answers the question."]},"tool_calls":[]}`)
			client := &fakeMessageCompleter{responses: responses}
			executor := &rejectedFreeStateTestExecutor{}
			loop := MessageLoop{
				Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", DefaultModel: "test", APIKey: "test"},
				Executor: executor, Budget: Budget{MaxTurns: tc.attemptedRepeats + 2, MaxToolCalls: tc.attemptedRepeats + 1, MaxConsecutiveErrors: 2},
			}
			result := loop.Start(context.Background(), Input{
				UserText: "inspect masking", AllowedTools: []string{"ccb.observation_request"},
				Context: map[string]any{"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "decision_phase": "processor_selection",
					"original_intent": "inspect masking",
				}},
			})
			if len(executor.calls) != 1 {
				t.Fatalf("%d attempted requests invoked executor %d times, want 1: %+v", tc.attemptedRepeats, len(executor.calls), executor.calls)
			}
			if result.FreeStateDecision == nil || result.FreeStateDecision.Status != FreeStateBlocked {
				t.Fatalf("rejected sequence did not terminate explicitly: %+v", result)
			}
			if len(client.calls) != tc.attemptedRepeats+1 {
				t.Fatalf("guard model calls=%d, want %d", len(client.calls), tc.attemptedRepeats+1)
			}
		})
	}
}

func TestFreeStateContinuationLedgerBlocksRejectedSetAfterRecentObservationChanges(t *testing.T) {
	fingerprint := messageLoopFreeStateViewFingerprint([]string{"mix.masking_relationship", "track.basic_energy"})
	state := &runState{input: Input{Context: map[string]any{"free_state_reasoning_loop": map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect masking",
		"observation_ledger": map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"rejected_view_sets": []any{map[string]any{
				"fingerprint": fingerprint, "requested_views": []any{"mix.masking_relationship", "track.basic_energy"},
				"status": "rejected", "retry_policy": "do_not_retry",
			}},
		},
	}}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	decision := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "retry", RequestedViewIDs: []string{"track.basic_energy", "mix.masking_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"track.basic_energy", "mix.masking_relationship"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); !strings.Contains(issue, "already rejected") {
		t.Fatalf("continuation ledger did not block rejected set: %q", issue)
	}
	call := planner.ToolCall{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []any{"mix.masking_relationship", "track.basic_energy"},
	}}
	if issue := messageLoopToolGuardIssue(state, call, false); !strings.Contains(issue, "already rejected") {
		t.Fatalf("checkpoint/pending tool entry was not blocked before executor: %q", issue)
	}
}

func TestOpenSemanticNeedsActionRequiresValidatedProcessorIntent(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"semantic_entry_verified":   true,
		"semantic_entry_decision":   map[string]any{"schema_version": "semantic_entry_decision.v1", "route": "open_semantic"},
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "reduce sibilance"},
	}}, recentObservation: semanticGuidanceUsableCCBObservation("ready")}
	decision := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "evidence supports a de-esser", RemainingIntent: "reduce sibilance", ProcessorType: "de_esser",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); !strings.Contains(issue, "semantic_processor_intent") {
		t.Fatalf("missing semantic intent was accepted: %q", issue)
	}
	decision.FreeStateDecision.SemanticProcessorIntent = &processorintent.Intent{
		SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
		Family: processorintent.FamilyDeEsser, Intent: "reduce sibilance",
		RequiredCoverage: []string{"sibilance_reduction"}, Scope: processorintent.ScopeCurrentTrack,
		ControlMode: processorintent.ControlModeSemantic, Confidence: 0.94, EvidenceRefs: []string{"obs-1"},
	}
	if issue := messageLoopFreeStateOutputIssue(state, decision); issue != "" {
		t.Fatalf("validated De-esser intent was rejected: %q", issue)
	}
	if err := validateFreeStateProcessorIntent(*decision.FreeStateDecision.SemanticProcessorIntent, "limiter"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("De-esser intent escaped processor-type family binding: %v", err)
	}
}

func TestOpenSemanticNeedsObservationAllowsBoundedDistinctTrackTargets(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"semantic_entry_verified": true,
		"semantic_entry_decision": map[string]any{"schema_version": "semantic_entry_decision.v1", "route": "open_semantic"},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect the project",
		},
	}}}
	views := []string{"track.frequency_time_events", "track.band_dynamics"}
	decision := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "compare the two candidate tracks", RequestedViewIDs: views,
	}, ToolCalls: []planner.ToolCall{
		{ID: "observe-vocals", Tool: "ccb.observation_request", Args: map[string]any{
			"view_ids": views, "target_ref": map[string]any{"kind": "track", "id": "1032", "label": "vocals"},
		}},
		{ID: "observe-other", Tool: "ccb.observation_request", Args: map[string]any{
			"view_ids": views, "target_ref": map[string]any{"kind": "track", "id": "1022", "label": "other"},
		}},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, decision); issue != "" {
		t.Fatalf("distinct auditable track observations were rejected: %q", issue)
	}
	decision.FreeStateDecision.RequestedViewIDs = []string{"track.frequency_time_events", "track.frequency_time_events", "track.band_dynamics"}
	if issue := messageLoopFreeStateOutputIssue(state, decision); issue != "" {
		t.Fatalf("per-target repeated decision view was not treated as a multi-request union: %q", issue)
	}

	duplicate := decision
	duplicate.FreeStateDecision = &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "compare the two candidate tracks", RequestedViewIDs: views,
	}
	duplicate.ToolCalls = append([]planner.ToolCall(nil), decision.ToolCalls...)
	duplicate.ToolCalls[1].Args = map[string]any{
		"view_ids": views, "target_ref": map[string]any{"kind": "track", "id": "1032", "label": "vocals duplicate"},
	}
	if issue := messageLoopFreeStateOutputIssue(state, duplicate); !strings.Contains(issue, "must not repeat") {
		t.Fatalf("duplicate per-target observation was accepted: %q", issue)
	}

	uncovered := decision
	uncovered.FreeStateDecision = &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "request an uncovered view", RequestedViewIDs: []string{"track.frequency_time_events", "track.band_dynamics", "track.peak_structure"},
	}
	if issue := messageLoopFreeStateOutputIssue(state, uncovered); !strings.Contains(issue, "exactly cover") {
		t.Fatalf("multi-target calls did not have to cover the decision view union: %q", issue)
	}

	tooMany := decision
	tooMany.FreeStateDecision = &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient",
		Summary: "too many targets", RequestedViewIDs: views,
	}
	tooMany.ToolCalls = append([]planner.ToolCall(nil), decision.ToolCalls...)
	for i, id := range []string{"1017", "1012"} {
		tooMany.ToolCalls = append(tooMany.ToolCalls, planner.ToolCall{ID: fmt.Sprintf("extra-%d", i), Tool: "ccb.observation_request", Args: map[string]any{
			"view_ids": views, "target_ref": map[string]any{"kind": "track", "id": id},
		}})
	}
	if issue := messageLoopFreeStateOutputIssue(state, tooMany); !strings.Contains(issue, "at most 3") {
		t.Fatalf("unbounded multi-target observation was accepted: %q", issue)
	}
}

func TestOpenSemanticCandidateFrontierRequiresTargetLevelSelection(t *testing.T) {
	state := &runState{
		input: Input{Context: map[string]any{
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "repair the mix",
			},
			"minimal_audio_closure": map[string]any{"hypothesis_frontier": map[string]any{
				"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
			}},
		}},
		recentObservation: &RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
			"status": "ready", "read_only": true, "mutation_authority": false, "views": map[string]any{"track.timbre_frequency": map[string]any{"status": "ready"}},
		}},
	}
	projectRetry := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "inspect the project again", RequestedViewIDs: []string{"mix.frequency_relationship"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []string{"mix.frequency_relationship"}}}}}
	if issue := messageLoopFreeStateOutputIssue(state, projectRetry); !strings.Contains(issue, "target-level track") {
		t.Fatalf("candidate frontier allowed broad retry: %q", issue)
	}
	selection := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "inspect bass", RequestedViewIDs: []string{"track.timbre_frequency"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []string{"track.timbre_frequency"}, "target_ref": map[string]any{"kind": "track", "id": "1007"},
	}}}}
	if issue := messageLoopFreeStateOutputIssue(state, selection); issue != "" {
		t.Fatalf("candidate target selection rejected: %q", issue)
	}
	action := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient", Summary: "treat", RemainingIntent: "reduce overlap", ProcessorType: "eq",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, action); !strings.Contains(issue, "explicit target-level observation") {
		t.Fatalf("candidate frontier allowed action before target evidence: %q", issue)
	}
	frontier := state.input.Context["minimal_audio_closure"].(map[string]any)["hypothesis_frontier"].(map[string]any)
	frontier["candidate_id"] = "candidate-1"
	if issue := messageLoopFreeStateOutputIssue(state, selection); !strings.Contains(issue, "already has target-level evidence") {
		t.Fatalf("selected candidate allowed another observation: %q", issue)
	}
	boundary := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked, EvidenceStatus: "insufficient",
		Summary: "The selected track evidence remains inconclusive for a safe action.", Limitations: []string{"target observation is insufficient for treatment"},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, boundary); issue != "" {
		t.Fatalf("selected candidate rejected explicit evidence boundary: %q", issue)
	}
}

func TestOpenSemanticCandidateTargetEvidenceAllowsSameTurnFinalDecision(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "repair the mix"},
		"minimal_audio_closure": map[string]any{"hypothesis_frontier": map[string]any{
			"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
		}},
	}}, recentObservation: &RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"status": "ready", "read_only": true, "mutation_authority": false,
		"observation_id": "obs-target", "target_ref": map[string]any{"kind": "track", "id": "1007"}, "views": map[string]any{"track.timbre_frequency": map[string]any{"status": "ready"}},
	}}}
	if issue := messageLoopFreeStateCandidateProgressionIssue(state, FreeStateNeedsAction, nil); issue != "" {
		t.Fatalf("same-turn target evidence did not permit action decision: %q", issue)
	}
	observation := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "inspect again", RequestedViewIDs: []string{"track.time_dynamics"},
	}, ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []string{"track.time_dynamics"}, "target_ref": map[string]any{"kind": "track", "id": "1007"}}}}}
	if issue := messageLoopFreeStateOutputIssue(state, observation); !strings.Contains(issue, "already has target-level evidence") {
		t.Fatalf("same-turn target evidence allowed another observation: %q", issue)
	}
	proposal := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "target evidence supports a bounded improvement hypothesis",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion: agentprotocol.ImprovementProposalSchema,
			Target:        map[string]any{"kind": "track", "id": "1007"}, EvidenceRefs: []string{"obs-target"},
			ImprovementIntent: "make the bass relationship feel clearer", Hypothesis: "a small bounded change may improve separation",
			ExpectedEffect: "the relationship should be easier to compare", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind: "bounded_gain_adjustment", ParameterBounds: map[string]any{"delta_db": -0.5}, Confidence: 0.55,
		},
	}}
	if issue := messageLoopFreeStateOutputIssue(state, proposal); issue != "" {
		t.Fatalf("candidate target evidence rejected a valid improvement proposal: %q", issue)
	}
}

func TestActiveFreeStateRejectsMutationTools(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "make it steadier"},
	}}}
	for _, name := range []string{
		"plugin_grabber.apply_compressor_controls", "plugin.set_parameter", "set_plugin_param",
		"plugin.load_to_rack", "rack.add_node", "rack.load_plugin",
	} {
		if issue := messageLoopToolGuardIssue(state, planner.ToolCall{Tool: name}, false); !strings.Contains(issue, "active free-state") {
			t.Fatalf("mutation %s was not blocked: %q", name, issue)
		}
	}
	if issue := messageLoopToolGuardIssue(state, planner.ToolCall{Tool: "daw.invoke", Command: map[string]any{"cmd": "plugin.set_parameter"}}, false); !strings.Contains(issue, "active free-state") {
		t.Fatalf("wrapped generic mutation was not blocked: %q", issue)
	}
	if issue := messageLoopToolGuardIssue(state, planner.ToolCall{Tool: "ccb.observation_request"}, false); issue != "" {
		t.Fatalf("CCB observation was blocked: %q", issue)
	}
}

func TestFreeStateCCBTargetBindingUsesSelectedTrackForMixedTrackAndProjectViews(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"selected_track_id": "track-vocal", "selected_track_name": "Lead Vocal",
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "make the vocal steadier",
		},
	}}}
	call := resolveMessageLoopBindings(state, planner.ToolCall{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []string{"track.time_dynamics", "mix.multitrack_relationship"},
	}})
	if firstMapText(call.Args, "target_kind") != "track" || firstMapText(call.Args, "target_id") != "track-vocal" ||
		firstMapText(call.Args, "track_id") != "track-vocal" {
		t.Fatalf("mixed CCB target = %#v", call.Args)
	}

	projectCall := resolveMessageLoopBindings(state, planner.ToolCall{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids": []string{"mix.multitrack_relationship"},
	}})
	if firstMapText(projectCall.Args, "target_kind") != "project" || firstMapText(projectCall.Args, "target_id") != "current" {
		t.Fatalf("project CCB target = %#v", projectCall.Args)
	}
}

func TestFreeStateLedgerIsDisclosedToModelWithoutFullActionReceipt(t *testing.T) {
	state := &runState{
		goal: agentruntime.Goal{GoalID: "goal-1", RunID: "run-1"},
		input: Input{UserText: "make the vocal steadier and more forward", Context: map[string]any{
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "loop_id": "loop-1", "status": "re_evaluating",
				"decision_phase": "post_action_evaluation", "original_intent": "make the vocal steadier and more forward",
				"active_intent": "bring the vocal forward", "requires_post_action_observation": true,
				"actions": []map[string]any{{
					"cycle": 1, "processor_type": "compressor", "status": "applied", "summary": "compressor applied",
					"receipt": map[string]any{
						"schema_version": "semantic_effect.compressor_execution_receipt.v1", "status": "executed",
						"ticket_id": "ticket-1", "large_internal_evidence": strings.Repeat("x", 20000),
						"parameter_audit": map[string]any{"status": "pass", "target_parameter_count": 4},
						"com_evaluation": map[string]any{"status": "ready", "mode": "change_delta", "behavior_change": map[string]any{
							"status": "ready", "dimensions": []map[string]any{{
								"dimension": "transient_response", "status": "ready", "classification": "unchanged_within_tolerance",
								"evidence_refs": []string{"large-hidden-ref"},
							}},
						}},
					},
				}},
			},
		}},
		budget: Budget{MaxToolCalls: 4},
	}
	assembly := (&MessageLoop{}).assembly(state, `{}`)
	joined := ""
	for _, message := range assembly.Messages {
		joined += message.Content
	}
	if !strings.Contains(joined, `"original_intent":"make the vocal steadier and more forward"`) ||
		!strings.Contains(joined, `"decision_phase":"post_action_evaluation"`) ||
		!strings.Contains(joined, `"processor_type":"compressor"`) {
		t.Fatalf("compact free-state ledger missing from prompt: %s", joined)
	}
	if strings.Contains(joined, "large_internal_evidence") || strings.Contains(joined, strings.Repeat("x", 100)) {
		t.Fatal("full action receipt leaked into the model prompt")
	}
	if !strings.Contains(joined, `"classification":"unchanged_within_tolerance"`) || strings.Contains(joined, "large-hidden-ref") {
		t.Fatalf("compact COM behavior change was not disclosed safely: %s", joined)
	}
}

func TestFreeStatePromptPreservesConditionalTreatmentAuthorization(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "check whether the vocal is unstable and only treat it if the problem is real",
		},
	}}}
	prompt := messageLoopSystemPrompt(state)
	for _, required := range []string{
		"Evidence status and problem status are different",
		"Preserve conditional authorization exactly",
		"return no_candidate_found with the bounded evidence and limitations when the condition is false",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("free-state conditional authorization rule missing %q", required)
		}
	}
}
