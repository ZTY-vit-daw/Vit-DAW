package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

func semanticTreatmentTestInstances() []semanticTreatmentInstance {
	return []semanticTreatmentInstance{
		{Key: "loaded_instance_1", TrackID: "track-1", PluginID: "eq-1", PluginName: "Pro-Q 3", ProcessorType: "eq", QualificationStatus: "generic_static_eq_qualified", NextPlanner: "semantic_eq", Topology: map[string]any{"schema_version": "generic_eq_topology.prompt.v1"}},
		{Key: "loaded_instance_2", TrackID: "track-1", PluginID: "eq-2", PluginName: "TDR Nova", ProcessorType: "eq", QualificationStatus: "generic_static_eq_qualified", NextPlanner: "semantic_eq", Topology: map[string]any{"schema_version": "generic_eq_topology.prompt.v1"}},
		{Key: "loaded_instance_3", TrackID: "track-1", PluginID: "comp-1", PluginName: "Pro-C 2", ProcessorType: "unknown", QualificationStatus: "identity_only", NextPlanner: "capability_boundary"},
	}
}

func semanticTreatmentChoiceJSON() string {
	return `{
		"schema_version":"semantic_treatment_strategy.v1",
		"decision_mode":"choice_required",
		"user_goal":"让人声更靠前",
		"summary":"优先用均衡建立存在感，同时保留动态和密度方向。",
		"choices":[
			{"choice_key":"presence_eq","role":"recommended","title":"存在感 EQ","processor_type":"eq","target_mode":"existing_plugin","instance_key":"loaded_instance_1","reason":"已有合格 EQ 可做保守音色前移。","expected_effect":"人声更清晰靠前。","tradeoff":"过量会变硬。","confidence":"high","next_planner":"semantic_eq","material_difference":"直接改变频谱存在感。"},
			{"choice_key":"density_comp","role":"alternative","title":"动态稳定","processor_type":"compressor","target_mode":"load_required","reason":"稳定峰谷可以让人声持续靠前。","expected_effect":"密度更稳定。","tradeoff":"可能减少动态。","confidence":"medium","next_planner":"plugin_recommendation","material_difference":"改变动态而不是静态频谱。"},
			{"choice_key":"warmth_sat","role":"alternative","title":"轻度饱和","processor_type":"distortion","target_mode":"load_required","reason":"增加谐波密度可以提升可感知存在。","expected_effect":"更有实体感。","tradeoff":"可能增加粗糙感。","confidence":"medium","next_planner":"plugin_recommendation","material_difference":"增加谐波而不是压缩或均衡。"}
		],
		"global_constraints":["不要更刺耳"]
	}`
}

func TestSemanticTreatmentPlanRequiresOnePrimaryTwoMaterialAlternatives(t *testing.T) {
	plan, err := decodeAndValidateSemanticTreatmentPlan(semanticTreatmentChoiceJSON(), "让人声更靠前", semanticTreatmentTestInstances(), false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.DecisionMode != "choice_required" || len(plan.Choices) != 3 || plan.Choices[0].NextPlanner != "semantic_eq" || plan.Choices[1].NextPlanner != "plugin_recommendation" {
		t.Fatalf("plan=%#v", plan)
	}
	duplicate := strings.Replace(semanticTreatmentChoiceJSON(), `"processor_type":"distortion"`, `"processor_type":"compressor"`, 1)
	if _, err := decodeAndValidateSemanticTreatmentPlan(duplicate, "让人声更靠前", semanticTreatmentTestInstances(), false); err == nil || !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("duplicate strategy was accepted: %v", err)
	}
}

func TestSemanticTreatmentCannotEscalateIdentityOnlyInstanceToExecutablePlanner(t *testing.T) {
	text := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"更稳","choices":[{"choice_key":"comp","role":"recommended","title":"压缩","processor_type":"compressor","target_mode":"existing_plugin","instance_key":"loaded_instance_3","reason":"稳定动态","expected_effect":"更稳","confidence":"medium","next_planner":"semantic_eq"}]}`
	if _, err := decodeAndValidateSemanticTreatmentPlan(text, "更稳", semanticTreatmentTestInstances(), false); err == nil || !strings.Contains(err.Error(), "exceeds qualified") {
		t.Fatalf("identity-only instance gained mutation authority: %v", err)
	}
}

func TestSemanticTreatmentNativeHandoffIsDirectOnlyAndRequiresExistingAgentResult(t *testing.T) {
	text := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"让人声更靠前","choices":[{"choice_key":"native_gain","role":"recommended","title":"原生电平关系","processor_type":"native","target_mode":"native","reason":"已有观察支持调整原生电平关系","expected_effect":"人声相对靠前","confidence":"high","next_planner":"existing_agent_result"}]}`
	if _, err := decodeAndValidateSemanticTreatmentPlan(text, "让人声更靠前", nil, false, true); err != nil {
		t.Fatalf("qualified native handoff rejected: %v", err)
	}
	if _, err := decodeAndValidateSemanticTreatmentPlan(text, "让人声更靠前", nil, false, false); err == nil {
		t.Fatal("native strategy was accepted without an existing Agent result")
	}
}

func TestSemanticTreatmentLoadedEQArbitrationNeverInventsThirdInstance(t *testing.T) {
	instances := semanticTreatmentTestInstances()[:2]
	text := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"choice_required","user_goal":"提高一些高频","choices":[
		{"choice_key":"a","role":"recommended","title":"Pro-Q 3","processor_type":"eq","target_mode":"existing_plugin","instance_key":"loaded_instance_1","reason":"精细处理","expected_effect":"更亮","confidence":"high","next_planner":"semantic_eq"},
		{"choice_key":"b","role":"alternative","title":"Nova","processor_type":"eq","target_mode":"existing_plugin","instance_key":"loaded_instance_2","reason":"另一工作流","expected_effect":"更亮","confidence":"medium","next_planner":"semantic_eq"}]}`
	plan, err := decodeAndValidateSemanticTreatmentPlan(text, "提高一些高频", instances, true)
	if err != nil || len(plan.Choices) != 2 {
		t.Fatalf("real two-instance arbitration failed: plan=%#v err=%v", plan, err)
	}
	invented := strings.Replace(text, "loaded_instance_2", "loaded_instance_3", 1)
	if _, err := decodeAndValidateSemanticTreatmentPlan(invented, "提高一些高频", instances, true); err == nil {
		t.Fatal("invented loaded instance was accepted")
	}
}

func TestSemanticTreatmentRoutingSeparatesDiscussionEQAndOpenMethodGoal(t *testing.T) {
	ctx := map[string]any{"selected_track_id": "track-1"}
	if ordinaryAgentTreatmentStrategyIntent("为什么人声听起来靠后", ctx) {
		t.Fatal("discussion-only request entered treatment strategy")
	}
	if !ordinaryAgentSemanticEQMutationRequest("提高一些高频", ctx) || ordinaryAgentTreatmentStrategyIntent("提高一些高频", ctx) {
		t.Fatal("explicit EQ request did not keep the EQ fast path")
	}
	if !ordinaryAgentTreatmentStrategyIntent("让人声更靠前一些", ctx) {
		t.Fatal("open audible change goal did not enter method arbitration")
	}
}

func TestSemanticTreatmentStateTokenExpiresWhenPluginGraphChanges(t *testing.T) {
	state := map[string]any{"project_uuid": "project-1", "graph_revision": 7, "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "plugins": []any{map[string]any{"plugin_id": "eq-1", "plugin_name": "EQ"}},
	}}}
	before := semanticTreatmentStateToken(state, "track-1")
	changed := cloneContext(state)
	changed["graph_revision"] = 8
	after := semanticTreatmentStateToken(changed, "track-1")
	if before == "" || before == after {
		t.Fatalf("state token did not expire: before=%s after=%s", before, after)
	}
}

func TestSemanticTreatmentSelectionResponseHasNoMutationAuthority(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	plan, err := decodeAndValidateSemanticTreatmentPlan(semanticTreatmentChoiceJSON(), "让人声更靠前", semanticTreatmentTestInstances(), false)
	if err != nil {
		t.Fatal(err)
	}
	resp := server.semanticTreatmentSelectionResponse("chat-1", agentModeDefault, "track-1", "Vocal", "state-1",
		map[string]any{"selected_track_id": "track-1"}, agentloop.Result{GoalID: "goal-1", RunID: "run-1"}, plan, semanticTreatmentTestInstances())
	if resp.NeedsConfirmation || resp.Workflow != semanticTreatmentWorkflow || resp.GoalStatus != "waiting_clarification" || boolValue(resp.WorkflowData["mutation_performed"]) {
		t.Fatalf("response=%#v", resp)
	}
	if len(resp.InteractionRequests) != 1 || len(resp.InteractionRequests[0].Actions) != 4 || !resp.InteractionRequests[0].Actions[0].Recommended {
		t.Fatalf("interaction=%#v", resp.InteractionRequests)
	}
}

func TestSemanticTreatmentInteractionBoundsObservationPayload(t *testing.T) {
	large := strings.Repeat("spectral-evidence ", 10000)
	payload := semanticTreatmentObservationPayload(&agentloop.RecentObservation{
		Tool: "mix.observe", Status: "ok", Summary: map[string]any{"observation_id": "obs-large", "oversized": large},
	})
	raw := fmt.Sprint(payload)
	if len(raw) > 40000 || !strings.Contains(raw, "obs-large") || !strings.Contains(raw, "exceeded semantic planner budget") {
		t.Fatalf("observation payload was not bounded correctly: bytes=%d payload=%s", len(raw), raw)
	}
}

type nonEQQualificationKernel struct{}

func (nonEQQualificationKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstStringFromMap(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command")
	}
	return map[string]any{"status": "ok", "track_id": firstStringFromMap(command, "track_id"), "plugin_id": firstStringFromMap(command, "plugin_id"),
		"parameters": []any{map[string]any{"id": "mix", "name": "Mix", "normalized_value": 0.5, "host_controllable": true}}}, "", nil
}

func (nonEQQualificationKernel) SendVSPCommand(context.Context, string, map[string]any) (*kernel.VSPCommandResult, error) {
	return nil, errors.New("unexpected mutation")
}

func TestSemanticEQPostLoadQualificationFailureStopsBeforeParameterPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = nonEQQualificationKernel{}
	plan := PendingPlan{Context: map[string]any{"conversation_id": "chat-1", "goal_id": "goal-1", "run_id": "run-1"}, WorkflowData: map[string]any{
		"semantic_eq_post_load_handoff": true, "semantic_eq_post_load_goal": "减少浑浊", "track_id": "track-1", "plugin_name": "Not an EQ",
	}}
	resp, handled := server.semanticEQPostLoadHandoff(context.Background(), plan, []map[string]any{{"command_name": "rack_add_node", "result": map[string]any{
		"track_id": "track-1", "plugin_id": "plugin-1", "plugin_name": "Not an EQ",
	}}})
	if !handled || resp.NeedsConfirmation || resp.StopReason != "semantic_eq_post_load_not_qualified" || !strings.Contains(resp.Reply, "没有生成或写入参数方案") {
		t.Fatalf("response=%#v handled=%t", resp, handled)
	}
}
