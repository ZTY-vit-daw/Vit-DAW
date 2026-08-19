package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/semanticeffect"
	"vit-daw-agent/internal/shadow"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func semanticTreatmentTestInstances() []semanticTreatmentInstance {
	return []semanticTreatmentInstance{
		{Key: "loaded_instance_1", TrackID: "track-1", PluginID: "eq-1", PluginName: "Pro-Q 3", ProcessorType: "eq", QualificationStatus: "generic_static_eq_qualified", NextPlanner: "semantic_eq", Topology: map[string]any{"schema_version": "generic_eq_topology.prompt.v1"}},
		{Key: "loaded_instance_2", TrackID: "track-1", PluginID: "eq-2", PluginName: "TDR Nova", ProcessorType: "eq", QualificationStatus: "generic_static_eq_qualified", NextPlanner: "semantic_eq", Topology: map[string]any{"schema_version": "generic_eq_topology.prompt.v1"}},
		{Key: "loaded_instance_3", TrackID: "track-1", PluginID: "comp-1", PluginName: "Pro-C 2", ProcessorType: "unknown", QualificationStatus: "identity_only", NextPlanner: "capability_boundary"},
		{Key: "loaded_instance_4", TrackID: "track-1", PluginID: "comp-2", PluginName: "Pro-C 2", ProcessorType: "compressor", QualificationStatus: "broadband_compressor_qualified", NextPlanner: "semantic_compressor"},
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

func TestSemanticTreatmentModelInstancesDiscloseOnlySelectedFamilyAndNoTopology(t *testing.T) {
	instances := semanticTreatmentTestInstances()
	instances[3].IdentityCard = &semanticeffect.AudioProcessorIdentityCard{}
	model := semanticTreatmentModelInstances(instances, "compressor")
	if len(model) != 1 || model[0].Key != "loaded_instance_4" {
		t.Fatalf("model candidates=%#v, want only the qualified compressor", model)
	}
	if model[0].PluginID != "" || model[0].Topology != nil || model[0].IdentityCard != nil {
		t.Fatalf("family candidate leaked execution identity/topology: %#v", model[0])
	}
	if len(model[0].QualifiedSurfaces) > 0 && (model[0].QualifiedSurfaces[0].SurfaceKey != "" || model[0].QualifiedSurfaces[0].Topology != nil ||
		model[0].QualifiedSurfaces[0].IdentityCard != nil || len(model[0].QualifiedSurfaces[0].OwnedParameterIDs) != 0) {
		t.Fatalf("family candidate leaked nested execution identity/topology: %#v", model[0])
	}
	if model[0].PluginName != "Pro-C 2" || model[0].ProcessorType != "compressor" {
		t.Fatalf("progressive family candidate lost its post-family display identity: %#v", model[0])
	}
	if got := semanticTreatmentModelInstances(instances, "eq"); len(got) != 2 || got[0].ProcessorType != "eq" || got[1].ProcessorType != "eq" {
		t.Fatalf("EQ family filter=%#v", got)
	}
	if got := semanticTreatmentModelInstances(instances, ""); len(got) != 0 {
		t.Fatalf("pre-family strategy received loaded instances: %#v", got)
	}
}

func TestSemanticTreatmentMixedInstanceKeepsIndependentSurfacesAndFailsOwnershipConflict(t *testing.T) {
	instance := semanticTreatmentInstance{
		Key: "mixed-1", ProcessorType: "mixed", QualificationStatus: "multiple_surfaces", NextPlanner: "semantic_family_selection",
		QualifiedSurfaces: []semanticProcessorSurface{
			{Family: processorintent.FamilyStaticEQ, QualificationStatus: "ownership_conflict", NextPlanner: "semantic_eq", OwnedParameterIDs: []string{"shared"}},
			{Family: processorintent.FamilyBroadbandCompressor, QualificationStatus: "ownership_conflict", NextPlanner: "semantic_compressor", OwnedParameterIDs: []string{"shared"}},
		},
	}
	if len(instance.QualifiedSurfaces) != 2 || instance.ProcessorType == "compressor" {
		t.Fatalf("mixed instance was collapsed: %+v", instance)
	}
	if semanticTreatmentInstanceExecutable(instance, "eq", "semantic_eq") {
		t.Fatal("EQ surface with cross-family ownership conflict remained executable")
	}
	if semanticTreatmentInstanceExecutable(instance, "compressor", "semantic_compressor") {
		t.Fatal("compressor surface with cross-family ownership conflict remained executable")
	}
}

func TestSemanticTreatmentBuildSurfacesKeepsRealEQAndCompressorSurfaces(t *testing.T) {
	eq := newFakeEQKernel()
	eqReply, _, err := eq.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	compressor := newFakeCompressorKernel()
	compressorReply, _, err := compressor.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	rows := append(mapRowsValue(eqReply["parameters"]), mapRowsValue(compressorReply["parameters"])...)
	digest := plugingrabber.BuildParameterDigest(map[string]any{
		"status": "ok", "track_id": "track-mixed", "plugin_id": "plugin-mixed", "plugin_name": "Mixed Surface",
		"parameters": rows,
	})
	surfaces, _ := semanticTreatmentBuildSurfaces("track-mixed", "plugin-mixed", digest)
	families := map[string]bool{}
	for _, surface := range surfaces {
		families[surface.Family] = true
	}
	if !families[processorintent.FamilyStaticEQ] || !families[processorintent.FamilyBroadbandCompressor] {
		t.Fatalf("mixed digest surfaces=%+v", surfaces)
	}
	if len(surfaces) != 2 {
		t.Fatalf("mixed digest exposed unexpected surfaces=%+v", surfaces)
	}
	model := semanticTreatmentModelInstances([]semanticTreatmentInstance{{
		Key: "mixed-instance", ProcessorType: "mixed", QualificationStatus: "multiple_surfaces", QualifiedSurfaces: surfaces,
	}}, "compressor")
	if len(model) != 1 || len(model[0].QualifiedSurfaces) != 1 || model[0].QualifiedSurfaces[0].Family != processorintent.FamilyBroadbandCompressor ||
		model[0].QualifiedSurfaces[0].SurfaceKey != "" || model[0].QualifiedSurfaces[0].Topology != nil ||
		model[0].QualifiedSurfaces[0].IdentityCard != nil || len(model[0].QualifiedSurfaces[0].OwnedParameterIDs) != 0 {
		t.Fatalf("real surface projection leaked execution state: %+v", model)
	}
}

func TestSemanticTreatmentBuildSurfacesFailsClosedOnRecognizerOwnershipConflict(t *testing.T) {
	eq := newFakeEQKernel()
	eqReply, _, err := eq.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	compressor := newFakeCompressorKernel()
	compressorReply, _, err := compressor.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	rows := append([]map[string]any{}, mapRowsValue(eqReply["parameters"])...)
	compressorRows := mapRowsValue(compressorReply["parameters"])
	for index, row := range compressorRows {
		copyRow := cloneStringAnyMap(row)
		copyRow["id"] = []string{"b1f", "b1g", "b1q", "b2f"}[index]
		rows = append(rows, copyRow)
	}
	digest := plugingrabber.BuildParameterDigest(map[string]any{
		"status": "ok", "track_id": "track-conflict", "plugin_id": "plugin-conflict", "parameters": rows,
	})
	surfaces, _ := semanticTreatmentBuildSurfaces("track-conflict", "plugin-conflict", digest)
	if len(surfaces) != 2 {
		t.Fatalf("surfaces=%+v", surfaces)
	}
	for _, surface := range surfaces {
		if surface.QualificationStatus != "ownership_conflict" ||
			semanticTreatmentInstanceExecutable(semanticTreatmentInstance{QualifiedSurfaces: []semanticProcessorSurface{surface}}, legacyProcessorTypeForFamily(surface.Family), surface.NextPlanner) {
			t.Fatalf("conflicting surface remained executable: %+v", surface)
		}
	}
}

func TestSemanticTreatmentPlannerRequestUsesProgressiveFamilyCandidateProjection(t *testing.T) {
	var requestBody map[string]any
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&requestBody)
		w.Header().Set("Content-Type", "application/json")
		responseText := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"make it stable","summary":"use compressor","choices":[{"choice_key":"comp","role":"recommended","title":"compression","processor_type":"compressor","target_mode":"existing_plugin","instance_key":"loaded_instance_4","reason":"control peaks","expected_effect":"stable","confidence":"high","next_planner":"semantic_compressor"}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": responseText}}}})
	}))
	defer httpServer.Close()
	server := &Server{llm: &llm.Client{HTTPClient: httpServer.Client()}}
	_, err := server.planSemanticTreatment(context.Background(), "chat-1", "make it stable", "track-1", "Vocal", nil, semanticTreatmentTestInstances(), false, false,
		config.EngineConfig{BaseURL: httpServer.URL, APIKey: "test", DefaultModel: "test"}, "compressor")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(requestBody)
	body := string(raw)
	for _, want := range []string{"loaded_instance_4", "Pro-C 2", "required_processor_type"} {
		if !strings.Contains(body, want) {
			t.Fatalf("post-family compressor candidate disclosure omitted %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"loaded_instance_1", "Pro-Q 3", "TDR Nova", "generic_eq_topology", "processor_identity_card", `"plugin_id"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("family candidate request leaked %q: %s", forbidden, body)
		}
	}
	for _, required := range []string{"gate_expander", "de_esser", "transient_shaper", "multiband_dynamics", "semantic_gate_expander", "semantic_de_esser", "semantic_transient_shaper", "semantic_multiband"} {
		if !strings.Contains(body, required) {
			t.Fatalf("treatment strategy protocol omitted current family %q: %s", required, body)
		}
	}
}

func TestSemanticTreatmentPlanAcceptsAllCurrentDynamicFamilyAdapters(t *testing.T) {
	text := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"reduce room noise","summary":"tighten the gate","choices":[{"choice_key":"gate","role":"recommended","title":"gate","processor_type":"gate_expander","target_mode":"existing_plugin","instance_key":"gate-instance","reason":"reduce inactive noise","expected_effect":"tighter tails","confidence":"high","next_planner":"semantic_gate_expander"}]}`
	instance := semanticTreatmentInstance{Key: "gate-instance", ProcessorType: "gate_expander", QualificationStatus: "gate_expander_topology_qualified", NextPlanner: "semantic_gate_expander", QualifiedSurfaces: []semanticProcessorSurface{{Family: processorintent.FamilyGateExpander, NextPlanner: "semantic_gate_expander"}}}
	plan, err := decodeAndValidateSemanticTreatmentPlan(text, "reduce room noise", []semanticTreatmentInstance{instance}, false)
	if err != nil || len(plan.Choices) != 1 || plan.Choices[0].ProcessorType != "gate_expander" {
		t.Fatalf("gate adapter choice was not accepted: plan=%+v err=%v", plan, err)
	}
}

func TestSemanticTreatmentPostLoadAdapterMatrixCoversAllCurrentDynamicFamilies(t *testing.T) {
	cases := map[string]string{
		"limiter":            processorintent.FamilyLimiter,
		"gate_expander":      processorintent.FamilyGateExpander,
		"de_esser":           processorintent.FamilyDeEsser,
		"transient_shaper":   processorintent.FamilyTransientShaper,
		"multiband_dynamics": processorintent.FamilyMultibandDynamics,
	}
	for processorType, wantFamily := range cases {
		family, planner, ok := semanticPostLoadAdapterForProcessorType(processorType)
		if !ok || family != wantFamily || planner == "" {
			t.Fatalf("processor %s adapter=(%q,%q,%v)", processorType, family, planner, ok)
		}
		spec, specOK := semanticDynamicSpecForFamily(family)
		if !specOK || spec.Planner != planner {
			t.Fatalf("processor %s planner drift: adapter=%q spec=%+v", processorType, planner, spec)
		}
	}
	for _, boundary := range []string{"spectral_dynamics", "clipper", "reverb", "delay"} {
		if _, _, ok := semanticPostLoadAdapterForProcessorType(boundary); ok {
			t.Fatalf("boundary/future processor %s unexpectedly entered dynamic adapter", boundary)
		}
	}
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

func TestSemanticTreatmentQualifiedCompressorUsesOnlyCompressorPlanner(t *testing.T) {
	valid := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"more stable","choices":[{"choice_key":"comp","role":"recommended","title":"compression","processor_type":"compressor","target_mode":"existing_plugin","instance_key":"loaded_instance_4","reason":"control peaks","expected_effect":"more stable","confidence":"high","next_planner":"semantic_compressor"}]}`
	plan, err := decodeAndValidateSemanticTreatmentPlan(valid, "more stable", semanticTreatmentTestInstances(), false)
	if err != nil || len(plan.Choices) != 1 || !semanticTreatmentInstanceExecutable(semanticTreatmentTestInstances()[3], "compressor", "semantic_compressor") {
		t.Fatalf("qualified compressor handoff failed: plan=%#v err=%v", plan, err)
	}
	wrongPlanner := strings.Replace(valid, `"semantic_compressor"`, `"semantic_eq"`, 1)
	if _, err := decodeAndValidateSemanticTreatmentPlan(wrongPlanner, "more stable", semanticTreatmentTestInstances(), false); err == nil || !strings.Contains(err.Error(), "exceeds qualified") {
		t.Fatalf("compressor escaped into the EQ planner: %v", err)
	}
	wrongType := strings.Replace(valid, `"processor_type":"compressor"`, `"processor_type":"eq"`, 1)
	if _, err := decodeAndValidateSemanticTreatmentPlan(wrongType, "more stable", semanticTreatmentTestInstances(), false); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("compressor instance accepted an EQ type: %v", err)
	}
}

func TestSemanticTreatmentRequiredProcessorPreventsFamilyRedecision(t *testing.T) {
	compressorPlan, err := decodeAndValidateSemanticTreatmentPlan(
		`{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"more present","choices":[{"choice_key":"comp","role":"recommended","title":"compression","processor_type":"compressor","target_mode":"existing_plugin","instance_key":"loaded_instance_4","reason":"control peaks","expected_effect":"more stable","confidence":"high","next_planner":"semantic_compressor"}]}`,
		"more present", semanticTreatmentTestInstances(), false)
	if err != nil {
		t.Fatal(err)
	}
	if issue := semanticTreatmentRequiredProcessorIssue(compressorPlan, "eq"); issue == nil || !strings.Contains(issue.Error(), "does not match") {
		t.Fatalf("compressor strategy replaced the required EQ family: %v", issue)
	}
	eqPlan, err := decodeAndValidateSemanticTreatmentPlan(
		`{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"more present","choices":[{"choice_key":"eq","role":"recommended","title":"EQ","processor_type":"eq","target_mode":"existing_plugin","instance_key":"loaded_instance_1","reason":"improve presence","expected_effect":"more present","confidence":"high","next_planner":"semantic_eq"}]}`,
		"more present", semanticTreatmentTestInstances(), false)
	if err != nil || semanticTreatmentRequiredProcessorIssue(eqPlan, "eq") != nil {
		t.Fatalf("matching required EQ strategy was rejected: plan=%#v err=%v", eqPlan, err)
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
	openPrompts := []string{
		"主唱听起来时大时小吗？如果确实有问题，再帮我让它更稳定，保留自然起伏和咬字瞬态。",
		"让主唱更稳定地靠前，但保留自然起伏和咬字瞬态。",
		"把鼓组偶尔突出的峰值收稳一些，但不要把击打感压扁。",
		"让贝斯每个音的存在感更均匀，但别让低频变薄。",
		"主唱有点发闷、低中频堆在一起，让它更清楚地站到前面，但别把声音做薄。",
		"让主唱更持续地站到前面，保留音色和自然咬字。",
		"主唱有点糊，而且时大时小，帮我整理得更清楚稳定，但别做薄或压死。",
		"人声总是被伴奏盖住，自己的存在感也不稳，让它更靠前，但别把伴奏做薄。",
	}
	for _, prompt := range openPrompts {
		if !ordinaryAgentTreatmentStrategyIntent(prompt, ctx) {
			t.Fatalf("open semantic experiment prompt missed method arbitration: %q", prompt)
		}
	}
}

func TestFreeStateNeedsActionWithoutSemanticIntentFailsClosed(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-missing-intent", ConversationID: "chat-missing-intent",
		Status: "awaiting_action", OriginalIntent: "protect peaks", ActiveIntent: "protect peaks", MaxCycles: 6,
	}
	response, routed := server.routeOrdinaryAgentTreatmentStrategy(context.Background(), "chat-missing-intent", agentModeDefault, "protect peaks",
		map[string]any{"selected_track_id": "track-1", "free_state_route_authorized": true, "free_state_processor_type": "limiter",
			"free_state_reasoning_loop": freeStateLoopMap(loop)},
		agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{Status: agentloop.FreeStateNeedsAction, ProcessorType: "limiter"}}, config.EngineConfig{})
	if !routed || response.Workflow != semanticTreatmentWorkflow || !strings.Contains(response.Reply, "semantic_processor_intent") {
		t.Fatalf("missing semantic intent was not rejected: routed=%t response=%+v", routed, response)
	}
}

func TestSelectedNonCompressorFallsThroughToOpenTreatmentStrategy(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = nonEQQualificationKernel{}
	response, routed := server.routeOrdinaryAgentSemanticCompressorPlanning(context.Background(), "chat-1", "compress it more",
		map[string]any{"selected_track_id": "track-1", "selected_plugin_track_id": "track-1", "selected_plugin_id": "eq-1"}, config.EngineConfig{})
	if routed || response.Workflow != "" {
		t.Fatalf("non-compressor selected instance was intercepted: routed=%t response=%#v", routed, response)
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

func TestSemanticTreatmentInstancesQualifyLiveBroadbandCompressor(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "track_type": "audio", "is_audio_track": true,
		"plugins": []any{map[string]any{"plugin_id": "comp-1", "plugin_name": "Pro-C 2"}},
	}}})
	server := New(nil, project, nil)
	fake := newFakeCompressorKernel()
	server.eqKernelOverride = fake
	instances, token := server.semanticTreatmentInstances(context.Background(), "track-1")
	if len(instances) != 1 || token == "" {
		t.Fatalf("instances=%#v token=%q", instances, token)
	}
	instance := instances[0]
	if instance.ProcessorType != "compressor" || instance.QualificationStatus != "broadband_compressor_qualified" ||
		instance.NextPlanner != "semantic_compressor" || instance.IdentityCard == nil || instance.IdentityCard.Archetype.EffectFamily != "broadband_compressor" {
		t.Fatalf("compressor was not qualified from live topology: %#v", instance)
	}
	if fake.readCalls != 1 {
		t.Fatalf("instance qualification used %d parameter reads, want one", fake.readCalls)
	}
}

type liveCompressorStateKernel struct {
	*fakeCompressorKernel
	stateCalls  int
	sidechainEQ bool
}

func (fake *liveCompressorStateKernel) SendCommand(ctx context.Context, command map[string]any) (map[string]any, string, error) {
	if firstStringFromMap(command, "cmd") == "get_project_state" {
		fake.stateCalls++
		return map[string]any{"status": "ok", "project_uuid": "project-1", "tracks": []any{map[string]any{
			"track_id": "track-1", "track_name": "Vocal", "plugins": []any{map[string]any{
				"plugin_item_id": "comp-live", "name": "Pro-C 2",
			}},
		}}}, "", nil
	}
	reply, raw, err := fake.fakeCompressorKernel.SendCommand(ctx, command)
	if err == nil && fake.sidechainEQ {
		reply["parameters"] = append(mapRowsValue(reply["parameters"]),
			fake.compParameter("sc-enabled", "Side Chain Mid Enabled", "enabled", []string{"Disabled", "Enabled"}),
			fake.compParameter("sc-freq", "Side Chain Mid Frequency", "frequency", []string{"20 Hz", "100 Hz", "1000 Hz", "5000 Hz", "20000 Hz"}),
			fake.compParameter("sc-gain", "Side Chain Mid Gain", "gain", []string{"-30 dB", "-15 dB", "0 dB", "15 dB", "30 dB"}),
			fake.compParameter("sc-q", "Side Chain Mid Q", "q", []string{"0.1", "0.5", "1.0", "5.0", "10.0"}),
			fake.compParameter("sc-shape", "Side Chain Mid Shape", "shape", []string{"Bell", "Low Shelf", "High Shelf", "Notch", "Band Pass"}),
		)
	}
	return reply, raw, err
}

func TestSemanticTreatmentInstancesPreferLivePluginGraphOverStaleShadow(t *testing.T) {
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_uuid": "project-1", "tracks": []any{map[string]any{
		"track_id": "track-1", "track_name": "Vocal", "plugins": []any{},
	}}})
	server := New(nil, project, nil)
	fake := &liveCompressorStateKernel{fakeCompressorKernel: newFakeCompressorKernel(), sidechainEQ: true}
	fake.params["sc-enabled"] = 1
	fake.params["sc-freq"] = 0.5
	fake.params["sc-gain"] = 0.5
	fake.params["sc-q"] = 0.5
	fake.params["sc-shape"] = 0
	server.eqKernelOverride = fake
	parameterReply, _, err := fake.SendCommand(context.Background(), map[string]any{"cmd": "get_plugin_parameters"})
	if err != nil {
		t.Fatal(err)
	}
	digest := plugingrabber.BuildParameterDigest(parameterReply)
	if len(plugingrabber.BuildEQBandSummary(digest)) == 0 {
		t.Fatal("test fixture did not expose the embedded sidechain EQ conflict")
	}
	if card, boundary := plugingrabber.BuildAudioProcessorIdentityCard(digest); card == nil || boundary != "" {
		t.Fatalf("test fixture did not expose the broadband compressor topology: card=%#v boundary=%q", card, boundary)
	}
	fake.readCalls = 0
	instances, token := server.semanticTreatmentInstances(context.Background(), "track-1")
	if len(instances) != 1 || token == "" {
		t.Fatalf("live plugin graph was not used: instances=%#v token=%q", instances, token)
	}
	instance := instances[0]
	if instance.PluginID != "comp-live" || instance.ProcessorType != "compressor" ||
		instance.QualificationStatus != "broadband_compressor_qualified" || instance.NextPlanner != "semantic_compressor" {
		t.Fatalf("live compressor was not qualified: %#v", instance)
	}
	if fake.stateCalls != 1 || fake.readCalls != 1 {
		t.Fatalf("live qualification reads state=%d parameters=%d, want one each", fake.stateCalls, fake.readCalls)
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

func TestSemanticCompressorPostLoadQualificationFailureStopsBeforeParameterPlan(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = nonEQQualificationKernel{}
	plan := PendingPlan{Context: map[string]any{"conversation_id": "chat-1", "goal_id": "goal-1", "run_id": "run-1"}, WorkflowData: map[string]any{
		"semantic_compressor_post_load_handoff": true, "semantic_compressor_post_load_goal": "make it more stable", "track_id": "track-1", "plugin_name": "Not a Compressor",
	}}
	resp, handled := server.semanticCompressorPostLoadHandoff(context.Background(), plan, []map[string]any{{"command_name": "rack_add_node", "result": map[string]any{
		"track_id": "track-1", "plugin_id": "plugin-1", "plugin_name": "Not a Compressor",
	}}})
	if !handled || resp.NeedsConfirmation || resp.StopReason != "semantic_compressor_post_load_not_qualified" || !strings.Contains(resp.Reply, "没有生成或写入参数方案") {
		t.Fatalf("response=%#v handled=%t", resp, handled)
	}
}

func TestSemanticGenericPostLoadRejectsUnprovenLiveTopology(t *testing.T) {
	plan := PendingPlan{Context: map[string]any{
		"free_state_semantic_processor_intent": map[string]any{
			"schema_version": processorintent.SchemaVersion, "status": processorintent.StatusResolved,
			"family": processorintent.FamilyLimiter, "intent": "protect peaks", "required_coverage": []string{"output_ceiling"},
			"scope": processorintent.ScopeCurrentTrack, "control_mode": processorintent.ControlModeSemantic, "confidence": 0.9,
		},
		"semantic_plugin_recommendation_candidate": map[string]any{
			"name": "Limiter", "manufacturer": "Vendor", "format": "VST3", "identifier": "limiter-id", "plugin_path": `C:\\Limiter.vst3`,
		},
	}, WorkflowData: map[string]any{"semantic_post_load_family": processorintent.FamilyLimiter}}
	_, err := semanticPostLoadPCAQualification(plan, plugingrabber.ParameterDigest{
		PluginName: "Limiter", PluginIdentifier: "limiter-id", PluginPath: `C:\\Limiter.vst3`, PluginFormat: "VST3", PluginManufacturer: "Vendor",
	}, processorintent.FamilyLimiter, "track-1", "limiter-1", "Limiter")
	if err == nil || !strings.Contains(err.Error(), "topology did not expose") {
		t.Fatalf("unproven live topology was accepted: %v", err)
	}
}
