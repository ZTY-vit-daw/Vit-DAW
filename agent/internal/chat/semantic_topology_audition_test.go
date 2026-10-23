package chat

// UX-AB-AUDITION-COVERAGE-1（2026-10）bus 拓扑直发两腿的等价伴生 audition
// 路径覆盖测试族。取证锚点（coord/runs/UX-AB-AUDITION-COVERAGE-1/
// forensics_1/FORENSICS.md）：场次⑤三轮（mvjr15zw/mvjr5hcm/mvjriarj）的
// track.send.connect(auto_create)+rack_add_node 两腿全部经
// executeSemanticTopologyBusLegs 直发且零 audition。本族钉住 P1 路径上
// 「效果突变 → audition 必在（或降级必有见证行；腿失败必无卡且错误
// 原样）」的防回归契约：
//  ① 计划构造：会话/目标/运行身份映射 + topo_* 命名域 + 无会话身份回 nil
//  ② bracket 机制复用：topology 计划挂卡走与 ticket 完全同一机械（时序/
//     事件/身份域）
//  ③ P1 路径覆盖：两腿派发发生在 bracket 内（before 渲染先于 connect），
//     腿失败=mutation 失败=无卡无降级行、失败契约逐字保持
//  ④ 无会话身份的计划按历史形态跑无 bracket 两腿
//  ⑤ 条款措辞：挂卡共享入口行，降级行如实命名两腿回执（不承诺参数回读）
//  ⑥ topology 挂卡经第三身份通道可判定落账
//
// 单测边界如实声明：harness PCA 装载门在单测环境拒绝 rack_add_node（无
// 真实插件二进制与晋升准入，见 harness.go enforceAgentProcessorLoadGate），
// 两腿全链成功挂卡的形态归真栈腿覆盖——与 semantic_topology_plan_test.go
// 的既有边界声明同源。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/trajectory"
)

// topologyOrderingSender 把 fake sender 适配成可排序探针：connect/load 两腿
// 的真实派发时刻进入共享 order 日志，与渲染相的时机做交叉断言。
type topologyOrderingSender struct {
	inner *topologyFakeSender
	order *[]string
}

func (t *topologyOrderingSender) SendCommand(ctx context.Context, cmd map[string]any) (map[string]any, string, error) {
	switch firstStringFromMap(cmd, "cmd") {
	case "track.send.connect":
		*t.order = append(*t.order, "leg:connect")
	case "rack_add_node":
		*t.order = append(*t.order, "leg:load")
	}
	return t.inner.SendCommand(ctx, cmd)
}

func topologyParkedPlanForTest(t *testing.T, server *Server, conversationID string) (PendingPlan, *semanticTopologyPlan) {
	t.Helper()
	plan := busTopologyPlanForTest()
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3", PluginIdentifier: "test-verb@1.0"}}
	requestContext := map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}
	if conversationID != "" {
		requestContext["conversation_id"] = conversationID
		requestContext["goal_id"] = "goal-topo"
		requestContext["run_id"] = "run-topo"
	}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), conversationID, "给人声加一点混响", requestContext, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3", "plugin_identifier": "test-verb@1.0",
	}, plan)
	if !resp.NeedsConfirmation || resp.PlanID == "" {
		t.Fatalf("前置失败：bus 计划必须停泊确认：%+v", resp)
	}
	stored, ok := server.pending[resp.PlanID]
	if !ok {
		t.Fatal("前置失败：停泊计划未入 pending")
	}
	return stored, plan
}

// 钉①：计划构造把停泊计划映射成 bracket 计划——会话身份取
// WorkflowData→Context 双回落，ticket 命名域 topo_<planID>，family/目标轨
// 取装载插件名与发送源轨；无会话身份（无法挂用户可归属的卡）回 nil。
func TestSemanticTopologyAuditionPlanShapeAndIdentity(t *testing.T) {
	server := topologyTestServer(&topologyFakeSender{})
	stored, _ := topologyParkedPlanForTest(t, server, "conv-topo")

	auditionPlan := server.semanticTopologyAuditionPlan(context.Background(), &stored, busTopologyPlanForTest())
	if auditionPlan == nil {
		t.Fatal("有会话身份的停泊计划必须产出 bracket 计划")
	}
	if auditionPlan.ConversationID != "conv-topo" || auditionPlan.GoalID != "goal-topo" || auditionPlan.RunID != "run-topo" {
		t.Fatalf("会话/目标/运行身份映射错误：%+v", auditionPlan)
	}
	if auditionPlan.TicketID != "topo_"+stored.ID {
		t.Fatalf("ticket 命名域必须是 topo_<planID>：%q", auditionPlan.TicketID)
	}
	if auditionPlan.Family != semanticTopologyAuditionFamily || auditionPlan.ProcessorType != "Test Verb" || auditionPlan.TrackID != "1032" {
		t.Fatalf("family/处理器/目标轨映射错误：%+v", auditionPlan)
	}
	if auditionPlan.BeforeRevision != "" || auditionPlan.AfterRevision == nil {
		// 裸 server 无 VSP state 通道：live revision 解析为空=before 相降级开放，
		// after 相闭包必须存在（真栈上由内核修订解析）。
		t.Fatalf("裸环境 live revision 应为空串且 after 闭包在场：%+v", auditionPlan)
	}

	// 命名域：与 ticket 会话（audition:dynamic:dyn_*）同族不同名，同会话不冲突。
	session := semanticDynamicAuditionSessionID(auditionPlan.ConversationID, auditionPlan.TicketID)
	ticketSession := semanticDynamicAuditionSessionID(auditionPlan.ConversationID, "dyn_7d48faeeb0128811")
	if !strings.HasPrefix(session, "audition:dynamic:topo_"+stored.ID) || session == ticketSession {
		t.Fatalf("topology 会话必须落在 audition:dynamic:topo_* 且与 ticket 会话互斥：%s vs %s", session, ticketSession)
	}
	if turn := semanticDynamicAuditionTurnID(auditionPlan.ConversationID, auditionPlan.TicketID); !strings.HasPrefix(turn, "dynamic_turn:topo_") {
		t.Fatalf("turn 命名域漂移：%s", turn)
	}

	// 无会话身份：双回落都缺席 → nil（两腿按历史形态无 bracket 跑）。
	anonymous := stored
	anonymous.Context = map[string]any{}
	anonymous.WorkflowData = map[string]any{}
	if server.semanticTopologyAuditionPlan(context.Background(), &anonymous, busTopologyPlanForTest()) != nil {
		t.Fatal("无会话身份的计划不得产出 bracket 计划")
	}
	if server.semanticTopologyAuditionPlan(context.Background(), nil, busTopologyPlanForTest()) != nil {
		t.Fatal("nil 计划必须回 nil")
	}
}

// 钉②：topology 计划挂卡走与 ticket 完全同一机械——before 渲染先于 mutation、
// 成功挂 audition:dynamic:topo_* 会话、audition.ready 与判定请求事件同轮、
// 记录带 topology family 与源轨。真栈上两腿全链成功也走这一机械（装载门
// 边界见文件头声明）。
func TestSemanticTopologyAuditionBracketMountsTopologyPlan(t *testing.T) {
	s := testContinuationServer()
	s.auditionKernel = &mixTickAuditionKernelStub{}
	var order []string
	s.semanticDynamicAuditionRenderOverride = func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		order = append(order, "render:"+phase+":"+revision)
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	}
	plan := &semanticDynamicAuditionPlan{
		ConversationID: "conv-topo", GoalID: "goal-topo", RunID: "run-topo",
		TicketID: "topo_plan_topo1", Family: semanticTopologyAuditionFamily, ProcessorType: "Test Verb",
		TrackID: "1032", BeforeRevision: "rev-8",
		AfterRevision: func() string { return "rev-9" },
	}

	bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error {
		order = append(order, "mutate")
		return nil
	})
	if err != nil || bracket.Record == nil {
		t.Fatalf("干净对必须挂卡：err=%v bracket=%+v", err, bracket)
	}
	want := []string{"render:before:rev-8", "mutate", "render:after:rev-9"}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Fatalf("时序必须是 before 渲染 -> 两腿 -> after 渲染:\n got %v\nwant %v", order, want)
	}
	record := bracket.Record
	if record.SessionID != "audition:dynamic:topo_plan_topo1" || record.TurnID != "dynamic_turn:topo_plan_topo1" {
		t.Fatalf("挂卡身份必须携带 topology 命名域：%+v", record)
	}
	if record.Family != semanticTopologyAuditionFamily || record.ProcessorType != "Test Verb" || record.TrackID != "1032" {
		t.Fatalf("记录必须保留 topology family/处理器/源轨：%+v", record)
	}
	events, _ := s.agentEventsSince("conv-topo", 0, 64)
	sawReady, sawJudgmentRequest := false, false
	for _, event := range events {
		if event.Type == "audition.ready" {
			sawReady = true
		}
		if event.Type == string(trajectory.EventUserJudgmentRequested) {
			sawJudgmentRequest = true
			details, _ := event.Payload["details"].(map[string]any)
			if firstStringFromMap(details, "semantic_dynamic") != "topo_plan_topo1" {
				t.Fatalf("判定节点必须标记 topo 通道身份：%+v", details)
			}
		}
	}
	if !sawReady || !sawJudgmentRequest {
		t.Fatalf("挂卡必须同轮发 audition.ready 与判定请求事件：%+v", events)
	}
}

// 钉③（P1 路径覆盖防回归）：两腿派发必须发生在 bracket 内——before 渲染
// 严格先于 connect 派发；装载腿在单测环境被 PCA 门拒绝（真实边界），该
// mutation 失败必须：无卡、无降级承诺行、失败契约逐字保持。
func TestBusTopologyLegsRunInsideAuditionBracket(t *testing.T) {
	var order []string
	sender := &topologyOrderingSender{inner: &topologyFakeSender{}, order: &order}
	server := topologyTestServer(sender)
	server.auditionKernel = &mixTickAuditionKernelStub{}
	server.semanticDynamicAuditionRenderOverride = func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		order = append(order, "render:"+phase)
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	}
	stored, _ := topologyParkedPlanForTest(t, server, "conv-topo")

	message, replies := server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")

	joined := strings.Join(order, "|")
	if !strings.Contains(joined, "render:before|leg:connect") {
		t.Fatalf("两腿必须在 bracket 内派发（before 渲染先于 connect）：\n%s", joined)
	}
	if strings.Contains(joined, "render:after") {
		t.Fatalf("装载腿被拒后不得有 after 渲染（mutation 未落地）：\n%s", joined)
	}
	var connectDispatched bool
	for _, cmd := range sender.inner.commands {
		if firstStringFromMap(cmd, "cmd") == "track.send.connect" && cmd["auto_create_fx_track"] == "Reverb Bus" {
			connectDispatched = true
		}
	}
	if !connectDispatched {
		t.Fatal("connect 腿必须真实派发（auto_create 形态=取证命中的旁路命令）")
	}
	if len(replies) < 1 {
		t.Fatal("connect 腿回执必须在 replies 里")
	}
	if !strings.Contains(message, "总线拓扑装载未完成") || !strings.Contains(message, "pca_load_gate") {
		t.Fatalf("装载腿门拒绝必须如实失败（既有契约）：%s", message)
	}
	if strings.Contains(message, "A/B 对照试听已就绪") || strings.Contains(message, "没能生成") {
		t.Fatalf("腿失败=无卡无降级承诺行：%s", message)
	}
	if _, ok := server.semanticDynamicAuditionFor("conv-topo"); ok {
		t.Fatal("腿失败后不得留可应答卡")
	}
	events, _ := server.agentEventsSince("conv-topo", 0, 64)
	for _, event := range events {
		if event.Type == "audition.ready" || event.Type == string(trajectory.EventUserJudgmentRequested) {
			t.Fatalf("腿失败后不得发 %s：%+v", event.Type, events)
		}
	}
}

// 钉④：无会话身份的停泊计划按历史形态跑无 bracket 两腿——渲染零调用、
// connect 照发、失败契约不变。
func TestBusTopologyLegsWithoutConversationStayUnbracketed(t *testing.T) {
	var order []string
	sender := &topologyOrderingSender{inner: &topologyFakeSender{}, order: &order}
	server := topologyTestServer(sender)
	server.auditionKernel = &mixTickAuditionKernelStub{}
	server.semanticDynamicAuditionRenderOverride = func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		order = append(order, "render:"+phase)
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	}
	stored, _ := topologyParkedPlanForTest(t, server, "")

	message, _ := server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")

	if strings.Contains(strings.Join(order, "|"), "render:") {
		t.Fatalf("无会话身份不得开 bracket（渲染零调用）：%v", order)
	}
	if !strings.Contains(strings.Join(order, "|"), "leg:connect") {
		t.Fatalf("无会话身份的两腿必须照发（历史形态）：%v", order)
	}
	if !strings.Contains(message, "总线拓扑装载未完成") {
		t.Fatalf("失败契约必须不变：%s", message)
	}
}

// 钉⑤：条款措辞——挂卡与 ticket 通道共享入口行；降级行如实命名两腿回执，
// 不承诺 bus 腿从未做过的参数回读；空 bracket 逐字保持原文。
func TestAppendSemanticTopologyAuditionClauseWording(t *testing.T) {
	mounted := appendSemanticTopologyAuditionClause("装载完成。", semanticDynamicAuditionBracket{Record: &semanticDynamicAuditionRecord{SessionID: "audition:dynamic:topo_plan_x"}})
	if !strings.HasPrefix(mounted, "装载完成。") || !strings.Contains(mounted, "A/B 对照试听已就绪") {
		t.Fatalf("挂卡条款必须共享 ticket 入口行：%q", mounted)
	}
	degraded := appendSemanticTopologyAuditionClause("装载完成。", semanticDynamicAuditionBracket{Degraded: "after_render_failed"})
	if !strings.Contains(degraded, "没能生成") || !strings.Contains(degraded, "after_render_failed") || !strings.Contains(degraded, "发送连接与装载已执行并带回执") {
		t.Fatalf("降级行必须如实命名两腿回执：%q", degraded)
	}
	if strings.Contains(degraded, "带回读验证") {
		t.Fatalf("降级行不得承诺参数回读（bus 腿从未做）：%q", degraded)
	}
	if plain := appendSemanticTopologyAuditionClause("装载完成。", semanticDynamicAuditionBracket{}); plain != "装载完成。" {
		t.Fatalf("空 bracket 必须逐字保持原文：%q", plain)
	}
}

// 钉⑥：topology 挂卡经 /agent/audition/judgment 第三身份通道可判定落账
// （ticket_id/family 回显 topo 通道身份；实验契约零接触同 174 钉⑤）。
func TestTopologyAuditionRecordJudgmentRoutesThirdChannel(t *testing.T) {
	server := New(nil, nil, nil)
	loop := auditionReadyLoop(t)
	server.storeFreeStateLoop(loop)
	record := semanticDynamicAuditionRecordForTest()
	record.TicketID = "topo_plan_topo1"
	record.Family = semanticTopologyAuditionFamily
	record.SessionID = "audition:dynamic:topo_plan_topo1"
	record.TurnID = "dynamic_turn:topo_plan_topo1"
	record.RoundID = "dynamic_round:topo_plan_topo1"
	record.Session["session_id"] = record.SessionID
	record.Session["turn_id"] = record.TurnID
	record.Session["round_id"] = record.RoundID
	server.storeSemanticDynamicAudition(record)

	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":%q,"heard_difference":"yes","preference":"b"}`,
		record.ConversationID, record.TurnID, record.RoundID, record.SessionID, record.ProjectRevision)
	recorder := httptest.NewRecorder()
	server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	outcome, _ := response["semantic_dynamic_audition"].(map[string]any)
	if outcome == nil {
		t.Fatalf("topology 判定必须落第三通道：%s", recorder.Body.String())
	}
	if outcome["ticket_id"] != "topo_plan_topo1" || outcome["family"] != semanticTopologyAuditionFamily || outcome["decision"] != mixTickAuditionDecisionRetain {
		t.Fatalf("落账必须回显 topo 通道身份与保留判定：%+v", outcome)
	}
	if _, ok := server.semanticDynamicAuditionFor(record.ConversationID); ok {
		t.Fatal("已判定卡必须退役")
	}
}
