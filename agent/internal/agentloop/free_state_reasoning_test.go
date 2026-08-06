package agentloop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type freeStateTestExecutor struct {
	calls []planner.ToolCall
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
	}}}
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

func TestFreeStateRejectsAutomaticRepeatOfAppliedProcessorFamily(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "re_evaluating",
			"original_intent": "make the vocal steadier and clearer",
			"actions":         []map[string]any{{"cycle": 1, "processor_type": "compressor", "status": "applied"}},
		},
	}}}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "try compression again", RemainingIntent: "make the vocal steadier", ProcessorType: "compressor",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "same processor_type") {
		t.Fatalf("automatic repeated compressor action was accepted: %q", issue)
	}
	out.FreeStateDecision.ProcessorType = "eq"
	out.FreeStateDecision.Summary = "address the independent tonal remainder"
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("independent EQ remainder was rejected: %q", issue)
	}
}

func TestFreeStateRejectsProcessorWithoutGovernedOpenSemanticPath(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"original_intent": "avoid over-compressing the vocal",
		},
	}}}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient",
		Summary: "add a limiter", RemainingIntent: "avoid over-compression", ProcessorType: "limiter",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "only eq and compressor") {
		t.Fatalf("unsupported limiter action was accepted: %q", issue)
	}
	for _, processorType := range []string{"eq", "compressor"} {
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
		"return satisfied with no processor action when the condition is false",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("free-state conditional authorization rule missing %q", required)
		}
	}
}
