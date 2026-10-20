package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// FULLACCESS-SEMANTIC-CHAIN-1 acceptance pins: under an explicit
// full_project_access turn the semantic processing chain issues zero
// interaction cards and executes autonomously (or fails honestly without
// parking); under manual_confirmation every face keeps its existing parked
// card byte-identically.

func fullAccessTestContext(extra map[string]any) map[string]any {
	return mergeContext(map[string]any{
		"authority_mode": authorityModeFull, "authority_mode_explicit": true,
		"conversation_id": "conversation-fa", "goal_id": "goal-fa", "run_id": "run-fa",
	}, extra)
}

func manualTestContext(extra map[string]any) map[string]any {
	return mergeContext(map[string]any{
		"authority_mode": authorityModeManual, "authority_mode_explicit": true,
		"conversation_id": "conversation-m", "goal_id": "goal-m", "run_id": "run-m",
	}, extra)
}

func TestFullAccessChainGateRequiresExplicitPairAndChainKey(t *testing.T) {
	if fullAccessSemanticChainContext(fullAccessTestContext(nil)) {
		t.Fatal("chain gate fired without the plugin-recommendation chain key")
	}
	withChain := fullAccessTestContext(map[string]any{"semantic_plugin_recommendation_selection": true})
	if !fullAccessSemanticChainContext(withChain) {
		t.Fatal("chain gate refused an explicit full-access chain context")
	}
	if explicitFullAccessRequestContext(manualTestContext(nil)) {
		t.Fatal("manual_confirmation was treated as a full-access grant")
	}
	bareMode := map[string]any{"authority_mode": authorityModeFull}
	if explicitFullAccessRequestContext(bareMode) {
		t.Fatal("bare mode string without the explicit flag was treated as a grant")
	}
}

func TestFullAccessPluginRecommendationExecutesChainWithoutSelectionCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-fa", RunID: "run-fa", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-fa", agentModeDefault, fullAccessTestContext(map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}), res, plan, pluginRecommendationTestCandidates())
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation {
		t.Fatalf("full-access selection issued a card: %#v", resp.InteractionRequests)
	}
	if resp.GoalStatus == string(agentruntime.StatusWaitingClarification) || resp.StopReason == "plugin_selection_required" {
		t.Fatalf("full-access selection still parked for the user: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}
	if len(server.interactions) != 0 {
		t.Fatalf("full-access selection stored a pending interaction: %#v", server.interactions)
	}
	data := resp.WorkflowData
	if !boolValue(data["selection_performed"]) || !boolValue(data["full_access_auto_authorized"]) ||
		firstStringFromMap(data, "selection_owner") != "full_project_access" ||
		firstStringFromMap(data, "recommendation_schema_version") != pluginRecommendationSchema {
		t.Fatalf("full-access selection omitted autonomy markers: %#v", data)
	}
	if !strings.Contains(resp.Reply, "主推荐") {
		t.Fatalf("autonomous selection did not report the chosen candidate: %s", resp.Reply)
	}
	// The chain must have continued into the load leg: either it executed, or
	// it failed honestly. It must never park a load confirmation plan.
	if len(server.pending) != 0 {
		t.Fatalf("full-access chain parked a load confirmation plan: %#v", server.pending)
	}
	if resp.PlanID != "" {
		t.Fatalf("full-access chain returned a confirmable plan id: %s", resp.PlanID)
	}
}

func TestManualPluginRecommendationKeepsSelectionCardByteIdentical(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidatePluginRecommendationPlan(pluginRecommendationTestJSON(), "eq", "减少一些浑浊", pluginRecommendationTestCandidates())
	if err != nil {
		t.Fatal(err)
	}
	res := agentloop.Result{GoalID: "goal-m", RunID: "run-m", Status: "completed"}
	resp := server.pluginRecommendationSelectionResponse("conversation-m", agentModeDefault, manualTestContext(map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	}), res, plan, pluginRecommendationTestCandidates())
	if resp.NeedsConfirmation || resp.Workflow != pluginRecommendationWorkflow ||
		resp.GoalStatus != string(agentruntime.StatusWaitingClarification) ||
		resp.StopReason != "plugin_selection_required" || resp.MessageKind != "proposal" {
		t.Fatalf("manual selection face changed: status=%s stop=%s kind=%s workflow=%s",
			resp.GoalStatus, resp.StopReason, resp.MessageKind, resp.Workflow)
	}
	if len(resp.InteractionRequests) != 1 || len(resp.InteractionRequests[0].Actions) != 4 ||
		resp.InteractionRequests[0].Actions[0].ID != "select_plugin_candidate_2" ||
		resp.InteractionRequests[0].Actions[3].ID != "cancel" {
		t.Fatalf("manual selection actions changed: %#v", resp.InteractionRequests[0].Actions)
	}
	if !strings.Contains(resp.Reply, "选择一个候选后，我只会生成加载确认，不会立即加载或修改参数。") {
		t.Fatalf("manual selection reply changed: %s", resp.Reply)
	}
	if boolValue(resp.WorkflowData["selection_performed"]) || boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("manual selection claimed action: %#v", resp.WorkflowData)
	}
	if len(server.interactions) != 1 {
		t.Fatalf("manual selection did not park its interaction: %#v", server.interactions)
	}
}

func TestFullAccessPluginLoadExecutesInsteadOfParkingConfirmationPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := PendingInteraction{
		ConversationID: "conversation-fa", GoalID: "goal-fa", RunID: "run-fa",
		RequestContext: fullAccessTestContext(map[string]any{
			"selected_track_id": "1007", "semantic_plugin_recommendation_selection": true,
		}),
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema, "processor_type": "reverb",
			"listening_goal": "加一点短衰减混响",
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
			"recommendations": []map[string]any{{
				"candidate_key": "plugin_candidate_1", "name": "FabFilter Pro-R", "plugin_path": `C:\VST3\FabFilter Pro-R.vst3`,
				"identifier": "VST3-FabFilter-Pro-R", "role": "recommended",
			}},
		},
	}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_1")
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation {
		t.Fatalf("full-access load issued a confirmation card: needs=%t requests=%#v", resp.NeedsConfirmation, resp.InteractionRequests)
	}
	if len(server.pending) != 0 || resp.PlanID != "" {
		t.Fatalf("full-access load parked a confirmable plan: plan_id=%q pending=%#v", resp.PlanID, server.pending)
	}
	if !boolValue(resp.WorkflowData["full_access_auto_authorized"]) || !boolValue(resp.WorkflowData["selection_performed"]) {
		t.Fatalf("full-access load omitted autonomy markers: %#v", resp.WorkflowData)
	}
	// With no kernel behind the test server the executed load fails honestly;
	// the pinned property is that the outcome is an execution result, never a
	// waiting-confirmation shape.
	if resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("full-access load still waits for the user: %#v", resp)
	}
	if resp.Error == "" && resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("full-access load neither executed nor reported an error: %#v", resp)
	}
}

func TestManualPluginLoadStillParksConfirmationPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	interaction := PendingInteraction{
		ConversationID: "conversation-m", GoalID: "goal-m", RunID: "run-m",
		RequestContext: manualTestContext(map[string]any{"selected_track_id": "1007"}),
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema, "processor_type": "reverb",
			"listening_goal": "加一点短衰减混响",
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
			"recommendations": []map[string]any{{
				"candidate_key": "plugin_candidate_1", "name": "FabFilter Pro-R", "plugin_path": `C:\VST3\FabFilter Pro-R.vst3`,
				"identifier": "VST3-FabFilter-Pro-R", "role": "recommended",
			}},
		},
	}
	resp := server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_plugin_candidate_1")
	if resp.Error != "" || !resp.NeedsConfirmation || resp.PlanID == "" || resp.Workflow != pluginGrabberLoadCommand {
		t.Fatalf("manual load face changed: %#v", resp)
	}
	if boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("manual load claimed a mutation: %#v", resp.WorkflowData)
	}
	plan, ok := server.pending[resp.PlanID]
	if !ok || len(plan.Decisions) != 1 || firstStringFromMap(plan.Decisions[0].Command, "cmd") != "rack_add_node" {
		t.Fatalf("manual load did not park its confirmation plan: %#v", plan)
	}
	if !strings.Contains(resp.Reply, "将加载") || !strings.Contains(resp.Reply, "确认后") {
		t.Fatalf("manual load reply changed: %s", resp.Reply)
	}
}

func TestFullAccessTypedControllerExecutesWithoutConfirmationCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	spec, ok := semanticDynamicSpecForFamily(processorintent.FamilyReverb)
	if !ok {
		t.Fatal("reverb dynamic spec missing")
	}
	summary := map[string]any{
		"control_topology": map[string]any{"generation": "generation-fa"},
		"reverb_stage": map[string]any{
			"stage_key": "reverb_main",
			"tail":      []map[string]any{dynamicTestBinding("decay", "r-decay")},
		},
	}
	_, bindings, err := semanticDynamicBriefFor(spec, summary, []string{"decay"})
	if err != nil {
		t.Fatal(err)
	}
	path := "stage/reverb_main/tail/decay/0"
	binding, ok := bindings[path]
	if !ok {
		t.Fatalf("reverb decay binding missing: %#v", bindings)
	}
	intent := processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
		Family: spec.Family, Intent: "short decay reverb", RequiredCoverage: []string{"decay"},
		Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .9}
	ticket := semanticDynamicTicket{SchemaVersion: semanticDynamicSchema, TicketID: "dyn-fa", Family: spec.Family,
		ProcessorType: spec.Processor, TrackID: "1007", PluginID: "plugin-1", TopologyGeneration: "generation-fa",
		Intent: intent, Controls: []semanticDynamicTicketRow{{Axis: "decay", PathKey: path, Role: binding.Role,
			ControlRef: binding.ControlRef, Target: map[string]any{"time_seconds": 0.9}, Purpose: "short decay", Confidence: "high"}}}
	resp := server.semanticDynamicFullAccessExecution(context.Background(), "conversation-fa",
		fullAccessTestContext(map[string]any{"semantic_plugin_recommendation_selection": true}), ticket)
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation {
		t.Fatalf("full-access typed controller issued a confirmation card: %#v", resp.InteractionRequests)
	}
	if resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) || resp.StopReason == "semantic_dynamic_confirmation_required" {
		t.Fatalf("full-access typed controller still waits for the user: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}
	if len(server.interactions) != 0 {
		t.Fatalf("full-access typed controller stored a pending interaction: %#v", server.interactions)
	}
	if !boolValue(resp.WorkflowData["full_access_auto_authorized"]) ||
		firstStringFromMap(resp.WorkflowData, "confirmation_bypassed") != "full_project_access" {
		t.Fatalf("typed controller execution omitted autonomy markers: %#v", resp.WorkflowData)
	}
}

func TestFullAccessStopPendingSkipsChainLegHonestly(t *testing.T) {
	resp := fullAccessStopPendingSkippedResponse("conversation-fa", "goal-fa", "run-fa", semanticDynamicWorkflow, "typed_controller")
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation || boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("stop-pending skip mutated or asked: %#v", resp)
	}
	if resp.GoalStatus != string(agentruntime.StatusStopped) || resp.StopReason != stopPendingProposalSkippedReason ||
		!boolValue(resp.WorkflowData["intervention_skipped"]) {
		t.Fatalf("stop-pending skip lost the honest shape: %#v", resp)
	}
}
