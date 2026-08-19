package agentloop

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestSemanticGuidanceAuditNeutralProjectionHidesPluginIdentityBeforeFamilySelection(t *testing.T) {
	state := &runState{
		goal: agentruntime.Goal{GoalID: "goal-1", RunID: "run-1"},
		input: Input{
			UserText: "make the selected track steadier without flattening it",
			Summary:  "make the selected track steadier without flattening it",
			Context: map[string]any{
				"selected_track_id":        "track-vocal",
				"selected_plugin_track_id": "track-vocal",
				"selected_plugin_id":       "plugin-comp-1",
				"selected_plugin_name":     "FabFilter Pro-C 2",
				"generic_eq_topology":      map[string]any{"schema_version": "generic_eq_topology.prompt.v1"},
				"processor_identity_card":  map[string]any{"archetype": "broadband_compressor"},
				"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
					"decision_phase": "processor_selection", "original_intent": "make it steadier",
					"target_ref": map[string]any{"kind": "plugin", "id": "plugin-comp-1", "plugin_name": "FabFilter Pro-C 2"},
					"actions": []any{map[string]any{"processor_type": "compressor", "summary": "FabFilter Pro-C 2 applied", "receipt": map[string]any{
						"plugin_id": "plugin-comp-1", "topology_generation": "topo-1",
					}}},
				},
			},
			State: map[string]any{
				"initialized": true, "graph_revision": 9,
				"tracks": []any{map[string]any{"track_id": "track-vocal", "track_name": "Vocal", "plugins": []any{
					map[string]any{"plugin_id": "plugin-comp-1", "plugin_name": "FabFilter Pro-C 2", "vendor": "FabFilter"},
				}}},
			},
			Conversation: []llm.Message{{Role: "assistant", Content: "I will use FabFilter Pro-C 2."}},
		},
		trace: []planner.TraceEvent{
			{Kind: "assistant", Reply: "I will use FabFilter Pro-C 2."},
			{Kind: "tool_result", ToolResult: &planner.ToolResult{Tool: "ccb.observation_request", Status: "ok", Result: map[string]any{
				"observation_id": "obs-1", "summary": "time-varying level evidence", "coverage": map[string]any{"views": []string{"track.time_dynamics"}},
			}}},
		},
		recentObservation: &RecentObservation{Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
			"observation_id": "obs-1", "summary": "time-varying level evidence",
		}},
		budget: Budget{MaxToolCalls: 4},
	}
	runner := &Runner{}
	full := runner.buildContextSnapshot(state)
	modelJSON := runner.buildModelContextSnapshot(state, full)

	if !strings.Contains(full.JSON(), "FabFilter Pro-C 2") {
		t.Fatal("fixture did not prove the internal full snapshot carries plugin identity")
	}
	for _, forbidden := range []string{
		"FabFilter Pro-C 2", "FabFilter", "plugin-comp-1", "generic_eq_topology",
		"processor_identity_card", "semantic_library", "recommended_family", "default_coverage", "expected_view",
		"I will use FabFilter",
	} {
		if strings.Contains(modelJSON, forbidden) {
			t.Fatalf("pre-family model projection leaked %q: %s", forbidden, modelJSON)
		}
	}
	if !strings.Contains(modelJSON, "track-vocal") || !strings.Contains(modelJSON, "obs-1") {
		t.Fatalf("neutral target/evidence was removed from model projection: %s", modelJSON)
	}

	state.contextSnapshot = full.Map()
	state.input.CatalogSummary = "plugin_grabber.inspect_compressor\nccb.observation_request"
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	joined := ""
	for _, message := range (&MessageLoop{}).assembly(state, modelJSON).Messages {
		joined += message.Content
	}
	if strings.Contains(joined, "FabFilter Pro-C 2") || strings.Contains(joined, "plugin-comp-1") || strings.Contains(joined, "topology_generation") || strings.Contains(joined, "plugin_grabber.inspect_compressor") {
		t.Fatalf("neutral family prompt leaked prior history or typed plugin catalog: %s", joined)
	}
	if !strings.Contains(joined, "choose a treatment family only from the observations you explicitly requested") {
		t.Fatal("neutral family prompt omitted model-owned decision boundary")
	}

	// Ensure the projection remains valid JSON, so it is an actual model input
	// rather than an audit-only string transformation.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(modelJSON), &decoded); err != nil || len(decoded) == 0 {
		t.Fatalf("neutral projection is not valid JSON: err=%v body=%s", err, modelJSON)
	}
}

func TestSemanticGuidanceAuditModelPromptDoesNotInjectFamilyOrObservation(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"semantic_entry_verified": true,
		"semantic_entry_decision": map[string]any{
			"schema_version": "semantic_entry_decision.v1", "route": "open_semantic",
			"control_mode": "semantic_loop", "user_authorization": "action_requested",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version":  "free_state_reasoning_loop.v1",
			"status":          "reasoning",
			"decision_phase":  "processor_selection",
			"original_intent": "make the selected track clearer and more stable",
		},
	}}}
	prompt := messageLoopSystemPrompt(state)

	for _, disclosure := range []string{
		"source-only macro dynamics may support choosing compression",
		"broad tonal evidence may support choosing EQ",
		"prefer mix.observe first",
		`"requested_view_ids":["track.time_dynamics"]`,
		"concrete EQ proposal for confirmation",
		"For an actionable generic static-EQ listening goal",
	} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(disclosure)) {
			t.Fatalf("prompt retained family/observation steering %q", disclosure)
		}
	}
	if !strings.Contains(prompt, "choose a treatment family only from the observations you explicitly requested") {
		t.Fatal("neutral model-owned family selection rule is missing")
	}
}

func TestSemanticGuidanceAuditNeutralTraceCompactsCCBTransportDetail(t *testing.T) {
	largeTransportDetail := strings.Repeat("transport-only-detail ", 4096)
	trace := []planner.TraceEvent{{
		Kind: "tool_result",
		ToolResult: &planner.ToolResult{
			Tool:   "ccb.observation_catalog",
			Status: "ok",
			Result: map[string]any{"catalog": map[string]any{
				"schema_version": "ccb_observation_catalog.v1",
				"views": []any{map[string]any{
					"view_id":   "track.time_dynamics",
					"questions": []any{"How does energy evolve over time?"},
				}},
				"transport_detail": largeTransportDetail,
			}},
		},
	}}

	projected := messageLoopPreFamilyTraceProjection(trace)
	encoded, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("marshal projected trace: %v", err)
	}
	body := string(encoded)
	if strings.Contains(body, "transport-only-detail") {
		t.Fatalf("neutral projection retained opaque CCB transport detail: %d bytes", len(encoded))
	}
	for _, required := range []string{"ccb_observation_catalog.v1", "track.time_dynamics", "How does energy evolve over time?"} {
		if !strings.Contains(body, required) {
			t.Fatalf("neutral projection removed catalog decision detail %q: %s", required, body)
		}
	}
}

func TestSemanticGuidanceAuditNeutralSnapshotStaysBoundedAfterCCBBundle(t *testing.T) {
	bundle := map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "observation_id": "obs-1",
		"requested_views": []any{"project.structure", "mix.multitrack_relationship"},
		"views": map[string]any{
			"project.structure":           map[string]any{"facts": strings.Repeat("structure-fact ", 1600), "status": "ready"},
			"mix.multitrack_relationship": map[string]any{"facts": strings.Repeat("relationship-fact ", 1600), "status": "ready"},
		},
		"evidence_refs": []any{"evidence-1"}, "limitations": []any{"bounded"},
	}
	result := map[string]any{"bundle": bundle, "status": "ok"}
	state := &runState{
		goal: agentruntime.Goal{GoalID: "goal-1", RunID: "run-1"},
		input: Input{UserText: "inspect the whole project", Summary: "inspect the whole project", Context: map[string]any{
			"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "decision_phase": "processor_selection"},
		}},
		trace:             []planner.TraceEvent{{Kind: "tool_result", ToolResult: &planner.ToolResult{Tool: "ccb.observation_request", Status: "ok", Result: result}}},
		recentObservation: recentObservationForTool(planner.ToolCall{Tool: "ccb.observation_request"}, executorpkg.Result{Tool: "ccb.observation_request", Status: "ok", Result: result}, planner.VerificationResult{}, false, nil),
		budget:            Budget{MaxToolCalls: 4},
	}
	runner := &Runner{}
	snapshot := runner.buildModelContextSnapshot(state, runner.buildContextSnapshot(state))
	if len(snapshot) > 40000 {
		t.Fatalf("neutral snapshot remains gateway-sized: %d chars", len(snapshot))
	}
}

func TestSemanticGuidanceAuditEveryActiveFreeStateBranchUsesNeutralPrompt(t *testing.T) {
	for _, phase := range []string{"", "processor_selection", "post_action_evaluation", "processor_materialization"} {
		state := &runState{input: Input{
			UserText:       "make the selected track clearer and steadier",
			CatalogSummary: "ccb.observation_catalog tool=ccb.observation_catalog: macro dynamics support compression; use track.time_dynamics first\nccb.observation_request tool=ccb.observation_request: use track.timbre_frequency for EQ",
			AllowedTools:   []string{"plugin_grabber.inspect_compressor", "plugin.load_to_rack", "ccb.observation_catalog", "ccb.observation_request"},
			Context: map[string]any{
				"selected_plugin_id": "plugin-secret", "selected_plugin_name": "Secret Product",
				"free_state_reasoning_loop": map[string]any{
					"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "decision_phase": phase,
					"original_intent": "make the selected track clearer and steadier",
				}},
			Conversation: []llm.Message{{Role: "assistant", Content: "Use Secret Product and track.time_dynamics."}},
		}}
		runner := &Runner{}
		full := runner.buildContextSnapshot(state)
		modelSnapshot := runner.buildModelContextSnapshot(state, full)
		assembly := (&MessageLoop{}).assembly(state, modelSnapshot)
		prompt := ""
		for _, message := range assembly.Messages {
			prompt += message.Content
		}
		for _, forbidden := range []string{
			"source-only macro dynamics may support choosing compression",
			"broad tonal evidence may support choosing eq",
			"prefer mix.observe first",
			`"requested_view_ids":["track.time_dynamics"]`,
			"concrete eq proposal for confirmation",
			"plugin_grabber.inspect_compressor",
			"plugin.load_to_rack",
			"track.time_dynamics",
			"track.timbre_frequency",
			"plugin-secret",
			"secret product",
		} {
			if strings.Contains(strings.ToLower(prompt), strings.ToLower(forbidden)) {
				t.Fatalf("phase %q retained free-state prompt guidance %q: %s", phase, forbidden, prompt)
			}
		}
		for _, required := range []string{
			"there is no default, expected, or phrase-to-view mapping",
			"choose a treatment family only from the observations you explicitly requested",
			"the local governed router, pca eligibility gate",
			"available observation catalog",
			"ccb.observation_request",
		} {
			if !strings.Contains(strings.ToLower(prompt), strings.ToLower(required)) {
				t.Fatalf("phase %q omitted neutral protocol rule %q: %s", phase, required, prompt)
			}
		}
	}
}

func TestSemanticGuidanceAuditInitialActionRequiresSuccessfulCCBObservation(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":  "free_state_reasoning_loop.v1",
			"status":          "reasoning",
			"decision_phase":  "processor_selection",
			"original_intent": "make the selected track more stable",
		},
	}}}
	out := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion:   FreeStateDecisionSchema,
		Status:          FreeStateNeedsAction,
		EvidenceStatus:  "sufficient",
		Summary:         "compression is the next treatment",
		RemainingIntent: "make the selected track more stable",
		ProcessorType:   "compressor",
	}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "model-requested CCB") {
		t.Fatalf("initial needs_action without observation was accepted: %q", issue)
	}
	state.recentObservation = semanticGuidanceUsableCCBObservation("ready")
	if issue := messageLoopFreeStateOutputIssue(state, out); issue != "" {
		t.Fatalf("initial needs_action with successful CCB evidence was rejected: %s", issue)
	}
}

func TestSemanticGuidanceAuditBaselineModelRequestedViewsArePreserved(t *testing.T) {
	requested := []string{"track.time_dynamics", "mix.multitrack_relationship"}
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":  "free_state_reasoning_loop.v1",
			"status":          "reasoning",
			"decision_phase":  "processor_selection",
			"original_intent": "make the selected track more stable in the mix",
		},
	}}}
	out := messageLoopOutput{FreeStateDecision: &FreeStateDecision{
		SchemaVersion:    FreeStateDecisionSchema,
		Status:           FreeStateNeedsObservation,
		EvidenceStatus:   "insufficient",
		Summary:          "inspect dynamics and mix relationship",
		RequestedViewIDs: requested,
	}}

	coerced := coerceMessageLoopFreeStateObservationOutput(state, out)
	if len(coerced.ToolCalls) != 1 || coerced.ToolCalls[0].Tool != "ccb.observation_request" {
		t.Fatalf("structured model observation was not materialized as one CCB request: %#v", coerced.ToolCalls)
	}
	actual, ok := coerced.ToolCalls[0].Args["view_ids"].([]string)
	if !ok || !reflect.DeepEqual(actual, requested) {
		t.Fatalf("CCB view_ids = %#v, want exact model request %#v", coerced.ToolCalls[0].Args["view_ids"], requested)
	}
	if coerced.ToolCalls[0].Reason != out.FreeStateDecision.Summary {
		t.Fatalf("CCB reason = %q, want model summary %q", coerced.ToolCalls[0].Reason, out.FreeStateDecision.Summary)
	}
	if issue := messageLoopFreeStateOutputIssue(state, coerced); issue != "" {
		t.Fatalf("materialized model observation request was rejected: %s", issue)
	}
}

func TestSemanticGuidanceAuditRejectsCCBCallOutsideModelRequestedViewSet(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":  "free_state_reasoning_loop.v1",
			"status":          "reasoning",
			"decision_phase":  "processor_selection",
			"original_intent": "inspect the selected track",
		},
	}}}
	out := messageLoopOutput{
		FreeStateDecision: &FreeStateDecision{
			SchemaVersion:    FreeStateDecisionSchema,
			Status:           FreeStateNeedsObservation,
			EvidenceStatus:   "insufficient",
			Summary:          "inspect dynamics",
			RequestedViewIDs: []string{"track.time_dynamics"},
		},
		ToolCalls: []planner.ToolCall{{
			Tool: "ccb.observation_request",
			Args: map[string]any{"view_ids": []string{"track.timbre_frequency"}},
		}},
	}

	coerced := coerceMessageLoopFreeStateObservationOutput(state, out)
	if !reflect.DeepEqual(coerced.ToolCalls, out.ToolCalls) {
		t.Fatalf("current explicit CCB call was unexpectedly rewritten: got %#v want %#v", coerced.ToolCalls, out.ToolCalls)
	}
	if issue := messageLoopFreeStateOutputIssue(state, coerced); !strings.Contains(issue, "exactly match") {
		t.Fatalf("mismatched structured/tool view request was accepted: %q", issue)
	}
}

func semanticGuidanceUsableCCBObservation(status string) *RecentObservation {
	return &RecentObservation{Tool: "ccb.observation_request", Status: "ok", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": status,
		"read_only": true, "mutation_authority": false, "observation_id": "obs-guidance",
		"views": map[string]any{"track.time_dynamics": map[string]any{"status": status}},
	}}
}
