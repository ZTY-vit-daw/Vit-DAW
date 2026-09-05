package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/shadow"
)

// AGENT-1 RED 断言（任务卡 2026-09-05-AGENT-1）：
//   ① 开关旧路径（默认 legacy_native）与现状逐字节等价：native mix-tick 确认面
//     原样落盘，且 track_gain/pan 不受任何开关取值影响；
//   ② 新路径首例：broadband_compression 放行进语义策略层，无合格实例时产出
//     load_required 选择并把接力交给现有插件推荐入口（推荐→加载确认→
//     qualification→执行→回读的完整链由真栈 E2E 冒烟验收）；
//   ③ selection 记录持久化（workspace runtime state 序列化/反序列化）且模型
//     投影只含候选 key 与能力摘要——路径/指纹/PCA 内部字段不出服务端；
//   ④ 旧路径与无 marker 的普通语义流永不产生 selection 记录。

func broadbandProposalForTest() agentprotocol.ImprovementProposal {
	return agentprotocol.ImprovementProposal{
		SchemaVersion:     agentprotocol.ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "vocals"},
		EvidenceRefs:      []string{"obs-vocal"},
		ImprovementIntent: "稳定人声动态",
		Hypothesis:        "bounded threshold move evens the level",
		ExpectedEffect:    "more stable vocal level",
		ActionDomain:      agentprotocol.ImprovementActionDomainBroadbandCompression,
		ActionKind:        "broadband_threshold_adjust",
		ParameterBounds:   map[string]any{"threshold_db": -1.0},
		Confidence:        0.5,
	}
}

func processorSelectionTestLoop(conversationID string) freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema,
		LoopID:        "loop-psel", ConversationID: conversationID,
		GoalID: "goal-psel", RunID: "run-psel",
		Status: "awaiting_experiment", OriginalIntent: "稳定人声动态",
		LatestObservation: d1FreshObservationForTest("7"),
		CreatedAt:         now, UpdatedAt: now,
	}
}

// RED①：默认（开关关闭）时 broadband_compression 仍走原 native mix-tick 拦截，
// 响应形状与现状逐字段等价；同时 RED④：无 marker、无 selection 记录。
func TestProcessorSelectionLegacyRouteStaysByteEquivalentNativeMixTick(t *testing.T) {
	t.Setenv(domainRoutingEnvName, "")
	server := New(nil, nil, nil)
	proposal := broadbandProposalForTest()
	interaction := PendingInteraction{ConversationID: "chat-psel-legacy", GoalID: "goal-psel", RunID: "run-psel"}
	requestContext := map[string]any{}

	response, routed := server.routeAcceptedImprovementProposalNativeDomain(context.Background(), interaction, proposal, requestContext)
	if !routed {
		t.Fatal("legacy route declined the broadband_compression proposal")
	}
	if response.Workflow != "mix_tick" || !response.NeedsConfirmation ||
		response.StopReason != "improvement_proposal_native_tool_confirmation_required" ||
		response.GoalStatus != "waiting_confirmation" {
		t.Fatalf("legacy mix-tick confirmation surface drifted: workflow=%q needs=%v stop=%q status=%q",
			response.Workflow, response.NeedsConfirmation, response.StopReason, response.GoalStatus)
	}
	if firstStringFromMap(response.WorkflowData, "proposal_execution_authority") != "mix_tick_tools" ||
		response.WorkflowData["action_domain_router"] != true || boolValue(response.WorkflowData["mutation_performed"]) {
		t.Fatalf("legacy workflow data drifted: %+v", response.WorkflowData)
	}
	proposalRow := firstMapFromAny(response.WorkflowData["improvement_proposal"])
	if firstStringFromMap(proposalRow, "action_domain") != agentprotocol.ImprovementActionDomainBroadbandCompression {
		t.Fatalf("legacy path lost its proposal echo: %+v", proposalRow)
	}
	tick, ok := server.pendingMixTickForConversation("chat-psel-legacy")
	if !ok || tick.Operation != d1BroadbandCompressionKind || tick.TrackID != "vocals" {
		t.Fatalf("legacy pending tick drifted: %+v ok=%v", tick, ok)
	}
	if _, hasMarker := requestContext[processorSelectionRouteContextKey]; hasMarker {
		t.Fatal("legacy route froze a processor_selection marker")
	}
	if len(server.processorSelections) != 0 {
		t.Fatalf("legacy route wrote selection records: %+v", server.processorSelections)
	}
}

// RED① 补充：开关只作用于显式列出的域，track_gain/pan 在任何取值下都不离开
// native 路由。
func TestProcessorSelectionSwitchNeverTouchesNativeControlDomains(t *testing.T) {
	t.Setenv(domainRoutingEnvName, "broadband_compression=processor_selection,static_eq=processor_selection")
	server := New(nil, nil, nil)
	for _, domain := range []string{agentprotocol.ImprovementActionDomainTrackGain, agentprotocol.ImprovementActionDomainPan} {
		proposal := broadbandProposalForTest()
		proposal.ActionDomain = domain
		proposal.ParameterBounds = map[string]any{"delta_db": -0.5}
		if domain == agentprotocol.ImprovementActionDomainPan {
			proposal.ParameterBounds = map[string]any{"delta_pan": 0.1}
		}
		requestContext := map[string]any{}
		response, routed := server.routeAcceptedImprovementProposalNativeDomain(context.Background(),
			PendingInteraction{ConversationID: "chat-psel-native", GoalID: "goal-psel", RunID: "run-psel"}, proposal, requestContext)
		if !routed || response.Workflow != "mix_tick" {
			t.Fatalf("native control domain %q left the native router: routed=%v workflow=%q", domain, routed, response.Workflow)
		}
		if _, hasMarker := requestContext[processorSelectionRouteContextKey]; hasMarker {
			t.Fatalf("native control domain %q received a selection marker", domain)
		}
	}
}

// RED② 第一段：开关启用后 native 拦截放行、冻结 marker、不落 mix tick。
func TestProcessorSelectionRouteDeclinesNativeInterceptionWithMarker(t *testing.T) {
	t.Setenv(domainRoutingEnvName, "broadband_compression=processor_selection")
	server := New(nil, nil, nil)
	proposal := broadbandProposalForTest()
	interaction := PendingInteraction{ConversationID: "chat-psel-new", GoalID: "goal-psel", RunID: "run-psel"}
	requestContext := map[string]any{}

	response, routed := server.routeAcceptedImprovementProposalNativeDomain(context.Background(), interaction, proposal, requestContext)
	if routed {
		t.Fatalf("processor_selection route still intercepted natively: %+v", response)
	}
	marker, ok := processorSelectionRouteFromContext(requestContext)
	if !ok {
		t.Fatal("fall-through froze no routing marker")
	}
	if marker.ActionDomain != agentprotocol.ImprovementActionDomainBroadbandCompression ||
		marker.ConversationID != "chat-psel-new" || marker.GoalID != "goal-psel" {
		t.Fatalf("marker identity drifted: %+v", marker)
	}
	if target := marker.TargetRef; firstStringFromMap(target, "id") != "vocals" || len(marker.EvidenceRefs) != 1 || marker.EvidenceRefs[0] != "obs-vocal" {
		t.Fatalf("marker target/evidence drifted: %+v %+v", target, marker.EvidenceRefs)
	}
	if family := firstStringFromMap(marker.Intent, "family"); family != processorintent.FamilyBroadbandCompressor {
		t.Fatalf("marker intent family = %q, want %q", family, processorintent.FamilyBroadbandCompressor)
	}
	if _, hasTick := server.pendingMixTickForConversation("chat-psel-new"); hasTick {
		t.Fatal("processor_selection route stored a pending mix tick")
	}
	// marker 本身不含内部字段：可以安全随 request_context 进入交互载荷。
	markerJSON, _ := json.Marshal(requestContext[processorSelectionRouteContextKey])
	for _, forbidden := range []string{"plugin_path", "binary_fingerprint", "attestation_id", "subject_key", "pca_admission_receipt"} {
		if strings.Contains(string(markerJSON), forbidden) {
			t.Fatalf("routing marker leaked %q: %s", forbidden, markerJSON)
		}
	}
}

// RED② 第二段（无实例首例）：放行的提案进入语义策略层，策略给出
// load_required 物化选择后既有插件推荐入口接管，selection 记录在参数规划
// 开始前落盘。本机目录存在候选时再推进一步：用户选定候选创建加载计划，
// 记录推进到 load_pending 并绑定候选 key（完整链的加载确认→qualification→
// 执行→回读由真栈 E2E 冒烟验收）。
func TestProcessorSelectionNewPathRoutesThroughStrategyAndRecordsSelection(t *testing.T) {
	strategyJSON := `{"schema_version":"semantic_treatment_strategy.v1","decision_mode":"direct","user_goal":"稳定人声动态","summary":"broadband compression","choices":[` +
		`{"choice_key":"comp","role":"recommended","title":"宽带压缩","processor_type":"compressor","target_mode":"load_required","reason":"no qualified instance is loaded","expected_effect":"stable level","tradeoff":"dynamic loss if overdone","confidence":"high","next_planner":"plugin_recommendation"}],` +
		`"global_constraints":[],"evidence_refs":[],"limitations":[]}`
	var modelRequestBodies []map[string]any
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		modelRequestBodies = append(modelRequestBodies, payload)
		responseText := strategyJSON
		// 插件推荐阶段：回显请求里的第一个本机候选，保证响应与目录内容一致。
		for _, message := range mapRowsValue(payload["messages"]) {
			if firstStringFromMap(message, "role") != "user" {
				continue
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(firstStringFromMap(message, "content")), &decoded); err != nil {
				continue
			}
			candidates := mapRowsValue(decoded["loadable_local_candidates"])
			if len(candidates) == 0 {
				continue
			}
			responseText = fmt.Sprintf(`{"schema_version":"plugin_recommendation.v1","processor_type":"compressor","user_goal":"稳定人声动态","summary":"本机可用压缩器","choices":[{"candidate_key":%q,"identifier":%q,"role":"recommended","reason":"本机目录中可加载的压缩候选","tradeoff":"按目录事实选择","confidence":"medium","knowledge_basis":["catalog_metadata"]}],"limitations":[]}`,
				firstStringFromMap(candidates[0], "candidate_key"), firstStringFromMap(candidates[0], "identifier"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": responseText}}}})
	}))
	defer model.Close()
	t.Setenv("VIT_AGENT_LLM_BASE_URL", model.URL)
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test")
	t.Setenv(domainRoutingEnvName, "broadband_compression=processor_selection")

	server := New(nil, shadow.New(nil), nil)
	server.llm = &llm.Client{HTTPClient: model.Client()}
	loop := processorSelectionTestLoop("chat-psel-route")
	server.storeFreeStateLoop(loop)

	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat-psel-route", GoalID: "goal-psel", RunID: "run-psel", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(broadbandProposalForTest()), "request_context": map[string]any{
			"selected_track_id": "vocals", "free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")

	if response.WorkflowData["action_domain_router"] != true {
		t.Fatalf("accepted broadband proposal did not reach the governed router: workflow=%q data=%+v", response.Workflow, response.WorkflowData)
	}
	if response.Workflow != pluginRecommendationWorkflow {
		t.Fatalf("load_required handoff did not reach the existing plugin recommendation entry: workflow=%q", response.Workflow)
	}
	_, record, ok := server.latestProcessorSelection("chat-psel-route")
	if !ok {
		t.Fatal("processor_selection route wrote no selection record")
	}
	if record.SchemaVersion != processorSelectionSchema ||
		record.SelectedFamily != processorintent.FamilyBroadbandCompressor ||
		record.TargetMode != "load_required" ||
		record.AuthorizationState != processorSelectionAuthSelected ||
		record.ActionDomain != agentprotocol.ImprovementActionDomainBroadbandCompression {
		t.Fatalf("selection record drifted: %+v", record)
	}
	if target := record.TargetRef; firstStringFromMap(target, "id") != "vocals" || len(record.EvidenceRefs) != 1 || record.EvidenceRefs[0] != "obs-vocal" {
		t.Fatalf("selection record target/evidence drifted: %+v %+v", record.TargetRef, record.EvidenceRefs)
	}
	sawRequiredProcessorType := false
	for _, body := range modelRequestBodies {
		for _, message := range mapRowsValue(body["messages"]) {
			if firstStringFromMap(message, "role") != "user" {
				continue
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(firstStringFromMap(message, "content")), &decoded); err != nil {
				continue
			}
			if _, hasField := decoded["required_processor_type"]; hasField && firstStringFromMap(decoded, "required_processor_type") == "compressor" {
				sawRequiredProcessorType = true
			}
		}
	}
	if !sawRequiredProcessorType {
		t.Fatal("strategy LLM request never carried the required compressor family")
	}

	// 深链段（本机目录能供出候选时）：推荐选择把记录推进到 load_pending。
	if interaction, firstCandidateKey := pluginRecommendationInteractionForTest(server, "chat-psel-route"); interaction.ID != "" && firstCandidateKey != "" {
		t.Logf("deep segment exercised with candidate %s", firstCandidateKey)
		server.continuePluginRecommendationInteraction(context.Background(), interaction, "select_"+firstCandidateKey)
		_, updated, updatedOK := server.latestProcessorSelection("chat-psel-route")
		if !updatedOK || updated.AuthorizationState != processorSelectionAuthLoadPending || updated.CandidateKey != firstCandidateKey {
			t.Fatalf("candidate selection did not advance the record: %+v", updated)
		}
		if updated.SelectedFamily != processorintent.FamilyBroadbandCompressor || updated.TargetMode != "load_required" {
			t.Fatalf("candidate selection mutated the selection identity: %+v", updated)
		}
	}
}

// pluginRecommendationInteractionForTest returns the newest stored plugin
// recommendation interaction of a conversation plus its first offered
// candidate key.
func pluginRecommendationInteractionForTest(s *Server, conversationID string) (PendingInteraction, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest PendingInteraction
	for _, interaction := range s.interactions {
		if interaction.ConversationID != conversationID || interaction.Type != "plugin_recommendation_selection" {
			continue
		}
		if newest.ID == "" || interaction.CreatedAt.After(newest.CreatedAt) {
			newest = interaction
		}
	}
	if newest.ID == "" {
		return PendingInteraction{}, ""
	}
	rows := mapRowsValue(newest.Payload["recommendations"])
	if len(rows) == 0 {
		return newest, ""
	}
	return newest, firstStringFromMap(rows[0], "candidate_key")
}

// RED③：记录经 workspace runtime state 序列化往返后仍在（服务端持有 PCA
// receipt），而模型投影是白名单——不含路径/指纹/PCA 内部字段。
func TestProcessorSelectionRecordPersistsWithoutInternalFieldLeak(t *testing.T) {
	server := New(nil, nil, nil)
	receipt := map[string]any{
		"processor_family": processorintent.FamilyBroadbandCompressor, "name": "Comp", "manufacturer": "Fab",
		"format": "vst3", "identifier": "comp.id", "plugin_path": `C:\VST\Comp.vst3`,
		"subject_key": "subj-1", "binary_fingerprint": "fp-1", "attestation_id": "att-1",
	}
	server.storeProcessorSelection(ProcessorSelectionRecord{
		SchemaVersion: processorSelectionSchema, SelectionID: "psel_persist_1",
		ConversationID: "chat-psel-persist", ActionDomain: agentprotocol.ImprovementActionDomainBroadbandCompression,
		TargetRef:               map[string]any{"kind": "track", "id": "vocals"},
		SemanticProcessorIntent: map[string]any{"family": processorintent.FamilyBroadbandCompressor, "intent": "稳定人声动态"},
		SelectedFamily:          processorintent.FamilyBroadbandCompressor,
		TargetMode:              "load_required", CandidateKey: "cand-1",
		EvidenceRefs: []string{"obs-vocal"}, ProjectRevision: "7",
		PCAAdmissionReceipt: receipt, AuthorizationState: processorSelectionAuthQualified,
	})

	state := server.projectAgentRuntimeStateLocked()
	if len(state.ProcessorSelections) != 1 {
		t.Fatalf("runtime state snapshot lost the selection record: %+v", state.ProcessorSelections)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"processor_selection.v1", "pca_admission_receipt", "binary_fingerprint", "authorization_state"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("persisted selection record lost %q: %s", want, raw)
		}
	}
	var reloaded projectAgentRuntimeState
	if err := json.Unmarshal(raw, &reloaded); err != nil {
		t.Fatal(err)
	}
	if restored := nonNilMap(reloaded.ProcessorSelections); len(restored) != 1 ||
		firstStringFromMap(restored["psel_persist_1"].PCAAdmissionReceipt, "attestation_id") != "att-1" {
		t.Fatalf("selection record did not survive the persistence round trip: %+v", restored)
	}

	record := state.ProcessorSelections["psel_persist_1"]
	view := processorSelectionModelView(record)
	viewJSON, _ := json.Marshal(view)
	for _, forbidden := range []string{"plugin_path", "binary_fingerprint", "attestation_id", "subject_key", "pca_admission_receipt", "identifier", "target_ref", "evidence_refs"} {
		if strings.Contains(string(viewJSON), forbidden) {
			t.Fatalf("model view leaked %q: %s", forbidden, viewJSON)
		}
	}
	for _, required := range []string{"candidate_key", "selected_family", "target_mode", "authorization_state"} {
		if !strings.Contains(string(viewJSON), required) {
			t.Fatalf("model view lost %q: %s", required, viewJSON)
		}
	}
}

// RED④：无 marker（旧路径与既有普通语义流）时记录写入点是 no-op。
func TestProcessorSelectionWithoutMarkerWritesNothing(t *testing.T) {
	server := New(nil, nil, nil)
	server.recordProcessorSelectionFromStrategy(context.Background(), map[string]any{"selected_track_id": "vocals"},
		semanticTreatmentChoice{ProcessorType: "compressor", TargetMode: "load_required"})
	server.recordProcessorSelectionFromSelectedChoice(context.Background(), PendingInteraction{ConversationID: "chat-plain"}, map[string]any{"target_mode": "existing_plugin"})
	server.markProcessorSelectionCandidateChosen(map[string]any{"selected_track_id": "vocals"}, "cand-1")
	server.markProcessorSelectionQualified(map[string]any{"selected_track_id": "vocals"}, map[string]any{"attestation_id": "att-1"})
	server.markProcessorSelectionCancelled(map[string]any{"selected_track_id": "vocals"})
	if len(server.processorSelections) != 0 {
		t.Fatalf("marker-less flows wrote selection records: %+v", server.processorSelections)
	}
}

// 开关四要件之可观测：每次路由决策把 domain/route/来源写入运行日志。
func TestProcessorSelectionDomainRouteDecisionsAreLogged(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent.log")
	server := &Server{logger: logx.New(false, logPath, 64)}
	t.Setenv(domainRoutingEnvName, "broadband_compression=processor_selection")

	if got := server.domainRouteFor(agentprotocol.ImprovementActionDomainBroadbandCompression); got != DomainRouteProcessorSelection {
		t.Fatalf("broadband_compression route = %q", got)
	}
	if got := server.domainRouteFor(agentprotocol.ImprovementActionDomainStaticEQ); got != DomainRouteLegacyNative {
		t.Fatalf("static_eq default route = %q", got)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logged)
	for _, want := range []string{
		"[domain-route] domain=broadband_compression route=processor_selection",
		"[domain-route] domain=static_eq route=legacy_native",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("route decision log missing %q in: %s", want, logText)
		}
	}
}
