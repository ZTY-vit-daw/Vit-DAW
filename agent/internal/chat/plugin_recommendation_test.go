package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/shadow"
)

func pluginRecommendationTestCandidates() []pluginRecommendationCandidate {
	return []pluginRecommendationCandidate{
		{Key: "plugin_candidate_1", ID: "nova", Identifier: "nova", Name: "TDR Nova", Manufacturer: "Tokyo Dawn Labs", Format: "VST3", Category: "Fx|EQ", PluginPath: `C:\VST3\TDR Nova.vst3`, PrimaryType: "eq"},
		{Key: "plugin_candidate_2", ID: "proq", Identifier: "proq", Name: "FabFilter Pro-Q 3", Manufacturer: "FabFilter", Format: "VST3", Category: "Fx|EQ", PluginPath: `C:\VST3\FabFilter Pro-Q 3.vst3`, PrimaryType: "eq"},
		{Key: "plugin_candidate_3", ID: "api", Identifier: "api", Name: "API-550A", Manufacturer: "Example", Format: "VST3", Category: "Fx|EQ", PluginPath: `C:\VST3\API-550A.vst3`, PrimaryType: "eq"},
	}
}

func pluginRecommendationTestJSON() string {
	return `{
		"schema_version":"plugin_recommendation.v1",
		"processor_type":"eq",
		"user_goal":"减少一些浑浊",
		"summary":"主推荐透明且便于精细定位的参数均衡器。",
		"choices":[
			{"candidate_key":"plugin_candidate_2","role":"recommended","reason":"适合精确定位并保守削减低中频。","tradeoff":"功能较多，操作界面相对复杂。","confidence":"high","knowledge_basis":["catalog_metadata","model_knowledge","user_request"]},
			{"candidate_key":"plugin_candidate_1","role":"alternative","reason":"同样适合透明修整，并提供动态均衡工作流。","tradeoff":"当前任务只需要静态 EQ。","confidence":"high","knowledge_basis":["catalog_metadata","model_knowledge"]},
			{"candidate_key":"plugin_candidate_3","role":"alternative","reason":"适合更有性格的宽频段塑形。","tradeoff":"不如参数均衡器适合精细定位。","confidence":"medium","knowledge_basis":["catalog_metadata","model_knowledge"]}
		],
		"limitations":["推荐基于本机目录事实、用户目标和模型已有知识，没有进行插件实测。"]
	}`
}

func TestOrdinaryAgentPluginRecommendationIntentCoversHandoffAndDirectQuestion(t *testing.T) {
	noPlugin := map[string]any{"selected_track_id": "1007"}
	if !ordinaryAgentPluginRecommendationIntent("减少一些浑浊", noPlugin) {
		t.Fatal("semantic EQ without a plugin should enter plugin recommendation")
	}
	if !ordinaryAgentPluginRecommendationIntent("你推荐用哪个 EQ 插件？", map[string]any{}) {
		t.Fatal("direct plugin recommendation question was not recognized")
	}
	if ordinaryAgentPluginRecommendationIntent("加载 TDR Nova", noPlugin) {
		t.Fatal("explicit named load request must remain in the existing load workflow")
	}
}

func TestPlanPluginRecommendationLetsLLMChooseNonFirstLocalCandidate(t *testing.T) {
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{pluginRecommendationTestJSON()})
	plan, err := server.planPluginRecommendation(context.Background(), "conversation-1", "减少一些浑浊", "eq", map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}, nil, pluginRecommendationTestCandidates(), cfg)
	if err != nil {
		t.Fatalf("planPluginRecommendation: %v", err)
	}
	if *calls != 1 || len(plan.Choices) != 3 || plan.Choices[0].CandidateKey != "plugin_candidate_2" {
		t.Fatalf("plan=%#v calls=%d", plan, *calls)
	}
	if len(*bodies) != 1 || !strings.Contains((*bodies)[0], "pretrained knowledge") || !strings.Contains((*bodies)[0], "FabFilter Pro-Q 3") {
		t.Fatalf("planner prompt omitted LLM-judgement contract or hard candidates: %s", (*bodies)[0])
	}
}

func TestPlanPluginRecommendationRejectsInventedCandidateAndRepairsOnce(t *testing.T) {
	invalid := strings.Replace(pluginRecommendationTestJSON(), "plugin_candidate_2", "invented_plugin", 1)
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{invalid, pluginRecommendationTestJSON()})
	plan, err := server.planPluginRecommendation(context.Background(), "conversation-1", "减少一些浑浊", "eq", nil, nil, pluginRecommendationTestCandidates(), cfg)
	if err != nil {
		t.Fatalf("planPluginRecommendation repair: %v", err)
	}
	if *calls != 2 || plan.Choices[0].CandidateKey != "plugin_candidate_2" {
		t.Fatalf("plan=%#v calls=%d", plan, *calls)
	}
	if len(*bodies) != 2 || !strings.Contains((*bodies)[1], "invented candidate_key") {
		t.Fatalf("repair prompt omitted deterministic rejection: %#v", *bodies)
	}
}

func TestPluginRecommendationCandidatesPreserveOnlyLoadableHardFacts(t *testing.T) {
	rows := []map[string]any{
		{"id": "b", "name": "Plugin B", "manufacturer": "Maker", "format": "VST3", "category": "Fx|EQ", "plugin_path": `C:\VST3\B.vst3`, "primary_type": "eq", "search_score": 9999},
		{"id": "missing", "name": "Not Loadable"},
		{"id": "b", "name": "Duplicate", "plugin_path": `C:\VST3\B-copy.vst3`},
	}
	candidates := pluginRecommendationCandidatesFromRows(rows)
	if len(candidates) != 1 {
		t.Fatalf("candidates=%#v", candidates)
	}
	if candidates[0].Key != "plugin_candidate_1" || candidates[0].PluginPath != `C:\VST3\B.vst3` || candidates[0].Manufacturer != "Maker" {
		t.Fatalf("candidate hard facts=%#v", candidates[0])
	}
}

func TestGenericStaticEQRecommendationCandidatesExcludeExplicitDynamicEQ(t *testing.T) {
	candidates := []pluginRecommendationCandidate{
		{Key: "dynamic-category", Name: "bx_dynEQ V2", Category: "Fx|EQ|Dynamics"},
		{Key: "dynamic-name", Name: "Example Dynamic EQ", Category: "Fx|EQ"},
		{Key: "static", Name: "Pro-Q 3", Category: "Fx|EQ"},
	}
	got := genericStaticEQRecommendationCandidates(candidates)
	if len(got) != 1 || got[0].Key != "static" {
		t.Fatalf("static EQ admission=%#v", got)
	}
}

func TestPluginRecommendationSelectionResponseOffersOnePrimaryTwoAlternativesWithoutMutation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-1", RunID: "run-1", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}, res, plan, pluginRecommendationTestCandidates())
	if resp.NeedsConfirmation || resp.Workflow != pluginRecommendationWorkflow || resp.GoalStatus != "waiting_clarification" || resp.MessageKind != "proposal" {
		t.Fatalf("response=%#v", resp)
	}
	if boolValue(resp.WorkflowData["mutation_performed"]) || boolValue(resp.WorkflowData["selection_performed"]) || len(mapRowsValue(resp.WorkflowData["recommendations"])) != 3 {
		t.Fatalf("workflow_data=%#v", resp.WorkflowData)
	}
	if len(resp.InteractionRequests) != 1 || len(resp.InteractionRequests[0].Actions) != 4 || !resp.InteractionRequests[0].Actions[0].Recommended {
		t.Fatalf("interaction=%#v", resp.InteractionRequests)
	}
	if resp.InteractionRequests[0].Actions[0].ID != "select_plugin_candidate_2" || resp.InteractionRequests[0].Actions[3].ID != "cancel" {
		t.Fatalf("actions=%#v", resp.InteractionRequests[0].Actions)
	}
}

func TestPluginRecommendationCancelDoesNotCreateLoadPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := PendingInteraction{ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1", Payload: map[string]any{
		"schema_version":  pluginRecommendationSchema,
		"recommendations": []map[string]any{{"candidate_key": "plugin_candidate_1", "plugin_path": `C:\VST3\TDR Nova.vst3`}},
	}}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "cancel")
	if resp.GoalStatus != "cancelled" || boolValue(resp.WorkflowData["mutation_performed"]) || len(server.pending) != 0 {
		t.Fatalf("response=%#v pending=%#v", resp, server.pending)
	}
}

func TestPluginRecommendationSelectionCreatesGovernedLoadConfirmationWithoutMutation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := PendingInteraction{
		ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		RequestContext: map[string]any{"selected_track_id": "1007"},
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema,
			"processor_type": "eq",
			"listening_goal": "减少一些浑浊",
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
			"recommendations": []map[string]any{{
				"candidate_key": "plugin_candidate_1", "name": "TDR Nova", "plugin_path": `C:\VST3\TDR Nova.vst3`, "identifier": "VST3-TDR-Nova", "role": "recommended",
			}},
		},
	}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_1")
	if resp.Error != "" || !resp.NeedsConfirmation || resp.PlanID == "" || resp.Workflow != pluginGrabberLoadCommand {
		t.Fatalf("response=%#v", resp)
	}
	if boolValue(resp.WorkflowData["mutation_performed"]) || !boolValue(resp.WorkflowData["selection_performed"]) {
		t.Fatalf("workflow_data=%#v", resp.WorkflowData)
	}
	plan, ok := server.pending[resp.PlanID]
	if !ok || len(plan.Decisions) != 1 || firstStringFromMap(plan.Decisions[0].Command, "cmd") != "rack_add_node" {
		t.Fatalf("pending load plan=%#v", plan)
	}
	if firstStringFromMap(plan.Decisions[0].Command, "plugin_path") != `C:\VST3\TDR Nova.vst3` || firstStringFromMap(plan.Decisions[0].Command, "track_id") != "1007" {
		t.Fatalf("load command did not preserve exact selected hard facts: %#v", plan.Decisions[0].Command)
	}
	if firstStringFromMap(plan.Decisions[0].Command, "plugin_identifier") != "VST3-TDR-Nova" {
		t.Fatalf("load command omitted exact shell-safe plugin_identifier: %#v", plan.Decisions[0].Command)
	}
}

func TestPluginRecommendationSelectionRetainsFreeStateIntentIntoLoadConfirmation(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		GoalID: "goal-1", RunID: "run-1", Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "make the vocal steadier and more forward", ActiveIntent: "stabilize the vocal",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	}
	server.storeFreeStateLoop(loop)
	interaction := PendingInteraction{
		ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		RequestContext: map[string]any{
			"selected_track_id": "1007", "free_state_reasoning_loop": freeStateLoopMap(loop),
		},
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema, "processor_type": "compressor",
			"listening_goal": "stabilize the vocal", "post_load_planner": "semantic_compressor",
			"post_load_goal": "stabilize the vocal",
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Vocal"},
			"recommendations": []map[string]any{{
				"candidate_key": "plugin_candidate_1", "name": "Pro-C 2", "plugin_path": `C:\VST3\Pro-C 2.vst3`,
				"identifier": "VST3-Pro-C-2", "role": "recommended",
			}},
		},
	}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_1")
	if resp.Error != "" || !resp.NeedsConfirmation || resp.PlanID == "" {
		t.Fatalf("load response=%#v", resp)
	}
	responseLoop := firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"])
	requestLoop := firstMapFromAny(firstMapFromAny(resp.WorkflowData["request_context"])["free_state_reasoning_loop"])
	if firstStringFromMap(responseLoop, "original_intent") != loop.OriginalIntent ||
		firstStringFromMap(requestLoop, "original_intent") != loop.OriginalIntent {
		t.Fatalf("selection response lost free-state intent: %#v", resp.WorkflowData)
	}
	pending, ok := server.pending[resp.PlanID]
	if !ok || firstStringFromMap(firstMapFromAny(pending.Context["free_state_reasoning_loop"]), "original_intent") != loop.OriginalIntent {
		t.Fatalf("load confirmation plan lost free-state intent: %#v", pending.Context)
	}
}

func TestPluginRecommendationUsesCCBAuthoritativeTrackOverUISelection(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "awaiting_action", OriginalIntent: "stabilize the vocal", ActiveIntent: "stabilize the vocal",
		TargetRef: map[string]any{"kind": "track", "id": "1032", "track_id": "1032", "track_name": "Vocals"},
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	candidates := pluginRecommendationTestCandidates()
	plan := pluginRecommendationPlan{SchemaVersion: "plugin_recommendation.v1", ProcessorType: "compressor",
		UserGoal: "stabilize the vocal", Summary: "use a compressor",
		Choices: []pluginRecommendationChoice{{CandidateKey: candidates[0].Key, Role: "recommended", Reason: "control peaks", Confidence: "high"}}}
	resp := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}, agentloop.Result{GoalID: "goal-1", RunID: "run-1"}, plan, candidates)
	target := firstMapFromAny(resp.WorkflowData["target_ref"])
	if firstStringFromMap(target, "id") != "1032" || firstStringFromMap(target, "label") != "Vocals" {
		t.Fatalf("plugin selection retained stale UI target: %#v", target)
	}
}

func TestPluginRecommendationEQHandoffMarkerSurvivesSelectionIntoLoadPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	selection := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
		"semantic_eq_post_load_handoff": true, "semantic_eq_post_load_goal": "减少一些浑浊",
	}, agentloop.Result{GoalID: "goal-1", RunID: "run-1", RecentObservation: &agentloop.RecentObservation{
		Tool: "mix.observe", Status: "ok", Summary: map[string]any{"observation_id": "obs-1"},
	}}, plan, pluginRecommendationTestCandidates())
	if firstStringFromMap(selection.WorkflowData, "post_load_planner") != "semantic_eq" {
		t.Fatalf("selection omitted EQ handoff: %#v", selection.WorkflowData)
	}
	interaction := server.interactions[selection.InteractionRequests[0].ID]
	load := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_2")
	if load.Error != "" || !load.NeedsConfirmation {
		t.Fatalf("load response=%#v", load)
	}
	pending := server.pending[load.PlanID]
	if !boolValue(pending.WorkflowData["semantic_eq_post_load_handoff"]) || firstStringFromMap(pending.WorkflowData, "semantic_eq_post_load_goal") != "减少一些浑浊" {
		t.Fatalf("load plan lost EQ handoff: %#v", pending.WorkflowData)
	}
	if firstStringFromMap(firstMapFromAny(pending.WorkflowData["semantic_eq_post_load_observation_context"]), "tool") != "mix.observe" {
		t.Fatalf("load plan lost bounded observation evidence: %#v", pending.WorkflowData)
	}
}

func TestPluginRecommendationCompressorHandoffMarkerSurvivesSelectionIntoLoadPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	candidates := pluginRecommendationTestCandidates()
	plan := pluginRecommendationPlan{SchemaVersion: "plugin_recommendation.v1", ProcessorType: "compressor",
		UserGoal: "make the vocal more stable", Summary: "use a broadband compressor",
		Choices: []pluginRecommendationChoice{{CandidateKey: candidates[0].Key, Role: "recommended", Reason: "control vocal peaks", Confidence: "high"}}}
	selection := server.pluginRecommendationSelectionResponse("conversation-1", agentModeDefault, map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Vocal",
		"semantic_compressor_post_load_handoff": true, "semantic_compressor_post_load_goal": "make the vocal more stable",
	}, agentloop.Result{GoalID: "goal-1", RunID: "run-1", RecentObservation: &agentloop.RecentObservation{
		Tool: "mix.observe", Status: "ok", Summary: map[string]any{"observation_id": "obs-comp-1"},
	}}, plan, candidates)
	if firstStringFromMap(selection.WorkflowData, "post_load_planner") != "semantic_compressor" {
		t.Fatalf("selection omitted compressor handoff: %#v", selection.WorkflowData)
	}
	interaction := server.interactions[selection.InteractionRequests[0].ID]
	load := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_"+candidates[0].Key)
	if load.Error != "" || !load.NeedsConfirmation {
		t.Fatalf("load response=%#v", load)
	}
	pending := server.pending[load.PlanID]
	if !boolValue(pending.WorkflowData["semantic_compressor_post_load_handoff"]) ||
		firstStringFromMap(pending.WorkflowData, "semantic_compressor_post_load_goal") != "make the vocal more stable" {
		t.Fatalf("load plan lost compressor handoff: %#v", pending.WorkflowData)
	}
	if firstStringFromMap(firstMapFromAny(pending.WorkflowData["semantic_compressor_post_load_observation_context"]), "tool") != "mix.observe" {
		t.Fatalf("load plan lost bounded observation evidence: %#v", pending.WorkflowData)
	}
}

func TestPluginRecommendationCatalogUsesCompleteDynamicsFamily(t *testing.T) {
	if got := pluginRecommendationCatalogSearchType("compressor"); got != "dynamics" {
		t.Fatalf("compressor search type = %q", got)
	}
	if got := pluginRecommendationCatalogSearchType("limiter"); got != "dynamics" {
		t.Fatalf("limiter search type = %q", got)
	}
	if got := pluginRecommendationCatalogSearchType("eq"); got != "eq" {
		t.Fatalf("EQ search type = %q", got)
	}
}

func TestPluginRecommendationDynamicLoadHandoffMatrixCoversAllFiveFamilies(t *testing.T) {
	// The recommendation interaction is the model-owned boundary before a
	// load. Each governed dynamic family must retain its exact post-load
	// adapter through the confirmation plan without performing mutation.
	cases := []string{"limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics"}
	for _, family := range cases {
		t.Run(family, func(t *testing.T) {
			server := New(nil, shadow.New(nil), nil)
			server.harness = harness.New(nil, shadow.New(nil), nil)
			candidate := pluginRecommendationCandidate{
				Key:          "candidate_" + family,
				ID:           "id_" + family,
				Identifier:   "identifier_" + family,
				Name:         "Test " + family,
				Manufacturer: "Test",
				Format:       "VST3",
				Category:     "Fx|Dynamics",
				PluginPath:   `C:\VST3\` + family + `.vst3`,
				PrimaryType:  family,
			}
			plan := pluginRecommendationPlan{
				SchemaVersion: pluginRecommendationSchema,
				ProcessorType: family,
				UserGoal:      "apply the selected dynamic treatment",
				Summary:       "use the selected family",
				Choices: []pluginRecommendationChoice{{
					CandidateKey: candidate.Key,
					Role:         "recommended",
					Reason:       "family-specific governed load",
					Confidence:   "high",
				}},
			}
			requestContext := map[string]any{"selected_track_id": "track-1", "selected_track_name": "Vocal"}
			selection := server.pluginRecommendationSelectionResponse("conversation-"+family, agentModeDefault, requestContext,
				agentloop.Result{GoalID: "goal-1", RunID: "run-1"}, plan, []pluginRecommendationCandidate{candidate})
			if len(selection.InteractionRequests) != 1 {
				t.Fatalf("selection interaction = %+v", selection.InteractionRequests)
			}
			payload := selection.InteractionRequests[0].Payload
			if firstStringFromMap(payload, "post_load_family") != family || firstStringFromMap(payload, "post_load_planner") == "" {
				t.Fatalf("post-load adapter handoff = %+v", payload)
			}
			interaction, ok := server.interactions[selection.InteractionRequests[0].ID]
			if !ok {
				t.Fatal("dynamic recommendation interaction was not stored")
			}
			loaded := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_"+candidate.Key)
			if loaded.Error != "" || !loaded.NeedsConfirmation || loaded.Workflow != pluginGrabberLoadCommand {
				t.Fatalf("load confirmation = %+v", loaded)
			}
			pending, ok := server.pending[loaded.PlanID]
			if !ok {
				t.Fatalf("load plan %q was not persisted", loaded.PlanID)
			}
			if !boolValue(pending.WorkflowData["semantic_post_load_handoff"]) ||
				firstStringFromMap(pending.WorkflowData, "semantic_post_load_family") != family ||
				firstStringFromMap(pending.WorkflowData, "semantic_post_load_planner") == "" ||
				boolValue(pending.WorkflowData["mutation_performed"]) {
				t.Fatalf("dynamic load handoff plan = %+v", pending.WorkflowData)
			}
		})
	}
}

func TestRecoverPluginRecommendationInteractionRequiresTypedSelectionPayload(t *testing.T) {
	payload := map[string]any{
		"schema_version": pluginRecommendationSchema, "status": "awaiting_selection", "processor_type": "eq",
		"recommendations": []map[string]any{{"candidate_key": "plugin_candidate_1", "identifier": "nova"}},
		"request_context": map[string]any{"conversation_id": "conversation-1", "goal_id": "goal-1", "run_id": "run-1"},
	}
	interaction, ok := recoverPluginRecommendationInteractionFromPayload("interaction-1", payload)
	if !ok || interaction.ConversationID != "conversation-1" || !boolValue(interaction.RequestContext["plugin_recommendation_recovered"]) {
		t.Fatalf("recovered interaction=%#v ok=%t", interaction, ok)
	}
	if _, ok := recoverPluginRecommendationInteractionFromPayload("interaction-2", map[string]any{"schema_version": pluginRecommendationSchema}); ok {
		t.Fatal("incomplete client payload must not recover an interaction")
	}
}

func TestRecoveredPluginRecommendationCandidateIsReboundToCurrentCatalogFacts(t *testing.T) {
	candidates := pluginRecommendationTestCandidates()
	selected := map[string]any{
		"candidate_key": "plugin_candidate_2", "identifier": "proq", "name": "client supplied name",
		"plugin_path": `C:\VST3\FabFilter Pro-Q 3.vst3`, "role": "recommended", "reason": "reason", "confidence": "high",
	}
	verified, err := verifyRecoveredPluginRecommendationCandidateAgainst("eq", selected, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if firstStringFromMap(verified, "name") != "FabFilter Pro-Q 3" || firstStringFromMap(verified, "identifier") != "proq" {
		t.Fatalf("candidate was not rebound to server catalog facts: %#v", verified)
	}
	tampered := cloneContext(selected)
	tampered["plugin_path"] = `C:\VST3\Tampered.vst3`
	if _, err := verifyRecoveredPluginRecommendationCandidateAgainst("eq", tampered, candidates); err == nil {
		t.Fatal("tampered client plugin path was accepted")
	}
	tamperedKey := cloneContext(selected)
	tamperedKey["candidate_key"] = "candidate_outside_current_set"
	if _, err := verifyRecoveredPluginRecommendationCandidateAgainst("eq", tamperedKey, candidates); err == nil || !strings.Contains(err.Error(), "no longer present") {
		t.Fatalf("candidate outside current set was accepted: %v", err)
	}
}
