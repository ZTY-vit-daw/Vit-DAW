package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/rules/ruleset"
)

// TIMEFX-BUS-TOPOPLAN-1 验收单测：确定性默认模板（bus/inline 例外码/词表
// 命中/复用既有 FX 轨/提示词惯例段）+ 装载链消费形态。模板是纯函数
// （决策表风格直喂）；K2 send_route 确定性工作流零触碰、入口分类器与 C3
// 观察侧零改动由既有包测试回归覆盖。

func topologyTestState() map[string]any {
	return map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "1032", "track_name": "vocals"},
			map[string]any{"track_id": "1027", "track_name": "piano"},
		},
	}
}

func TestSemanticTopologyTemplateBusCreate(t *testing.T) {
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:        "给人声加一点混响，衰减短些",
		ProcessorType:   "reverb",
		SourceTrackID:   "1032",
		SourceTrackName: "vocals",
		State:           topologyTestState(),
	})
	if plan == nil {
		t.Fatal("reverb 请求应产出 bus 拓扑计划")
	}
	if plan.Mode != semanticTopologyModeBus {
		t.Fatalf("mode=%s want bus", plan.Mode)
	}
	if plan.FXTrack.Action != semanticTopologyFXActionCreate || plan.FXTrack.Name != "Reverb Bus" {
		t.Fatalf("fx_track=%+v want create/Reverb Bus", plan.FXTrack)
	}
	if len(plan.Sends) != 1 || plan.Sends[0].SourceTrackID != "1032" {
		t.Fatalf("sends=%+v", plan.Sends)
	}
	if plan.Sends[0].DefaultAmountDB != semanticTopologyDefaultSendAmountDB {
		t.Fatalf("default_amount_db=%v want %v", plan.Sends[0].DefaultAmountDB, semanticTopologyDefaultSendAmountDB)
	}
	if plan.SchemaVersion != semanticTopologyPlanSchema {
		t.Fatalf("schema=%s", plan.SchemaVersion)
	}
}

func TestSemanticTopologyTemplateDelayFamily(t *testing.T) {
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "给这条吉他轨加一个轻微的延迟",
		ProcessorType: "delay",
		SourceTrackID: "1017",
	})
	if plan == nil || plan.Mode != semanticTopologyModeBus {
		t.Fatalf("delay 请求应产出 bus 计划，got %+v", plan)
	}
	if plan.FXTrack.Name != "Delay Bus" {
		t.Fatalf("fx name=%s want Delay Bus", plan.FXTrack.Name)
	}
}

func TestSemanticTopologyTemplateFuzzyEnglishVocabularyHit(t *testing.T) {
	// 词表命中兜底：processorType 缺席但话术含英文显式词（c3 词法标记复用）。
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "add some room feel to this track",
		SourceTrackID: "1012",
	})
	if plan == nil || plan.Mode != semanticTopologyModeBus {
		t.Fatalf("词表命中（room）应产出 bus 计划，got %+v", plan)
	}
}

func TestSemanticTopologyTemplateNonTimeDomainNil(t *testing.T) {
	for _, processorType := range []string{"eq", "compressor", "limiter", "distortion"} {
		if plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
			UserText:      "给这条轨做点均衡处理",
			ProcessorType: processorType,
			SourceTrackID: "1032",
		}); plan != nil {
			t.Fatalf("processorType=%s 不应产出拓扑计划（非时域族）", processorType)
		}
	}
}

func TestSemanticTopologyTemplateNoSourceNil(t *testing.T) {
	if plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "加混响",
		ProcessorType: "reverb",
	}); plan != nil {
		t.Fatal("无源轨时不产出拓扑计划（无法成 Sends，回落既有行为）")
	}
}

func TestSemanticTopologyTemplateK2Defer(t *testing.T) {
	// K2 让位：共享话术+≥2 具名源轨=显式 send 结构请求，确定性 send 工作流
	// 所有（卡面红线：K2 面零触碰——模板只让位不接管）。
	if plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:         "给 vocals 和 guitar 共用一个板式混响",
		ProcessorType:    "reverb",
		SourceTrackID:    "1032",
		ShareSignal:      "共用",
		SourceTrackCount: 2,
		State:            topologyTestState(),
	}); plan != nil {
		t.Fatal("显式 send 结构请求应让位 K2（返回 nil）")
	}
}

func TestSemanticTopologyTemplateInlineExceptionCodes(t *testing.T) {
	cases := []struct {
		name string
		in   semanticTopologyTemplateInput
		code string
	}{
		{
			name: "user_explicit: 明说插入",
			in: semanticTopologyTemplateInput{
				UserText:      "直接插在这条轨上，不要建轨",
				ProcessorType: "reverb",
				SourceTrackID: "1032",
			},
			code: semanticTopologyExceptionUserExplicit,
		},
		{
			name: "single_track_ab: 试听探测上下文标记",
			in: semanticTopologyTemplateInput{
				UserText:      "给 vocals 加混响",
				ProcessorType: "reverb",
				SourceTrackID: "1032",
				AuditionProbe: true,
			},
			code: semanticTopologyExceptionSingleTrackAB,
		},
		{
			name: "experiment_probe: 实验探测上下文标记",
			in: semanticTopologyTemplateInput{
				UserText:        "给 vocals 加混响",
				ProcessorType:   "reverb",
				SourceTrackID:   "1032",
				ExperimentProbe: true,
			},
			code: semanticTopologyExceptionExperimentProbe,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := planSemanticTopologyTemplate(tc.in)
			if plan == nil {
				t.Fatal("inline 例外应产出计划（带例外码）")
			}
			if plan.Mode != semanticTopologyModeInlineException {
				t.Fatalf("mode=%s want inline_exception", plan.Mode)
			}
			if plan.Exception.Code != tc.code {
				t.Fatalf("exception code=%s want %s", plan.Exception.Code, tc.code)
			}
			if strings.TrimSpace(plan.Exception.Reason) == "" {
				t.Fatal("例外码必须带 reason")
			}
			if len(plan.Loads) != 1 || plan.Loads[0].TrackID != "1032" {
				t.Fatalf("inline loads=%+v（装载目标=源轨）", plan.Loads)
			}
		})
	}
}

func TestSemanticTopologyTemplateReuseByName(t *testing.T) {
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "给人声加一点混响",
		ProcessorType: "reverb",
		SourceTrackID: "1032",
		State:         topologyTestState(),
		SendTopology: map[string]any{
			"status": "ok",
			"sends":  []any{},
			"fx_tracks": []any{
				map[string]any{"track_id": "2001", "name": "Reverb Bus"},
			},
		},
	})
	if plan == nil || plan.Mode != semanticTopologyModeBus {
		t.Fatalf("got %+v", plan)
	}
	if plan.FXTrack.Action != semanticTopologyFXActionReuse || plan.FXTrack.TrackID != "2001" {
		t.Fatalf("fx_track=%+v want reuse/2001（同名既有 FX 轨）", plan.FXTrack)
	}
}

func TestSemanticTopologyTemplateReuseByFamilyPlugin(t *testing.T) {
	// 同名不匹配但 fx 轨机架挂同族插件（c3 词法）→ 复用。
	state := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id":   "2002",
				"track_name": "Space Aux",
				"rack": map[string]any{
					"nodes": []any{
						map[string]any{"node_id": "6001", "plugin_path": "C:/Plugins/ValhallaRoom.vst3"},
					},
				},
			},
		},
	}
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "给人声加一点混响",
		ProcessorType: "reverb",
		SourceTrackID: "1032",
		State:         state,
		SendTopology: map[string]any{
			"status": "ok",
			"fx_tracks": []any{
				map[string]any{"track_id": "2002", "name": "Space Aux"},
			},
		},
	})
	if plan == nil || plan.Mode != semanticTopologyModeBus {
		t.Fatalf("got %+v", plan)
	}
	if plan.FXTrack.Action != semanticTopologyFXActionReuse || plan.FXTrack.TrackID != "2002" {
		t.Fatalf("fx_track=%+v want reuse/2002（同族插件既有 FX 轨）", plan.FXTrack)
	}
}

func TestSemanticTopologyTemplateNoReadFallsBackToCreate(t *testing.T) {
	// track.send.list 读不到（nil）→ create-by-default-name；内核 connect 按
	// 名幂等复用，同名既有轨不会被重复建。
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "给人声加一点混响",
		ProcessorType: "reverb",
		SourceTrackID: "1032",
	})
	if plan == nil || plan.FXTrack.Action != semanticTopologyFXActionCreate || plan.FXTrack.Name != "Reverb Bus" {
		t.Fatalf("读面缺席应 create/Reverb Bus，got %+v", plan)
	}
}

func TestSemanticTopologyPlanPayloadRoundTrip(t *testing.T) {
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:        "给钢琴加一点混响",
		ProcessorType:   "reverb",
		SourceTrackID:   "1027",
		SourceTrackName: "piano",
	})
	if plan == nil {
		t.Fatal("plan nil")
	}
	payload := semanticTopologyPlanPayload(plan)
	if payload["mode"] != semanticTopologyModeBus {
		t.Fatalf("payload mode=%v", payload["mode"])
	}
	decoded := semanticTopologyPlanFromPayload(payload)
	if decoded == nil || decoded.Mode != semanticTopologyModeBus || decoded.FXTrack.Name != "Reverb Bus" {
		t.Fatalf("round trip 丢失字段：%+v", decoded)
	}
	if decoded := semanticTopologyPlanFromPayload(map[string]any{"schema_version": "wrong"}); decoded != nil {
		t.Fatal("未知 schema 必须 fail-nil（回落既有行为，不虚构拓扑）")
	}
	if decoded := semanticTopologyPlanFromPayload(nil); decoded != nil {
		t.Fatal("nil payload → nil")
	}
	// inline 例外缺码 → nil（例外码必填契约）。
	if decoded := semanticTopologyPlanFromPayload(map[string]any{
		"schema_version": semanticTopologyPlanSchema, "mode": semanticTopologyModeInlineException,
	}); decoded != nil {
		t.Fatal("inline 例外无例外码必须拒收")
	}
}

func TestSemanticTopologyPlanCardLinesContract(t *testing.T) {
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:        "给人声加一点混响",
		ProcessorType:   "reverb",
		SourceTrackID:   "1032",
		SourceTrackName: "vocals",
	})
	lines := semanticTopologyPlanCardLines(plan)
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"vocals", "1032", "Reverb Bus", "-6.0"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("卡面缺 %q：%s", want, joined)
		}
	}
	if strings.Contains(joined, "当前轨道") {
		t.Fatal("148 指名合同：卡面绝不回「当前轨道」")
	}
}

// ————— 装载链消费形态 —————

// topologyFakeSender 按命令名回放内核回复（track.send.list / track.send.
// connect / rack_add_node），并记录收到的命令供断言。
type topologyFakeSender struct {
	commands []map[string]any
}

func (f *topologyFakeSender) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.commands = append(f.commands, cmd)
	name := firstStringFromMap(cmd, "cmd")
	switch name {
	case "track.send.list":
		return map[string]any{"status": "ok", "sends": []any{}, "fx_tracks": []any{}}, "", nil
	case "track.send.connect":
		return map[string]any{
			"status": "ok", "source_track_id": cmd["source_track_id"],
			"fx_track_id": "2001", "fx_name": "Reverb Bus", "amount_db": -6.0,
			"sends":     []any{map[string]any{"source_track_id": cmd["source_track_id"], "fx_track_id": "2001", "fx_name": "Reverb Bus", "amount_db": -6.0}},
			"fx_tracks": []any{map[string]any{"track_id": "2001", "name": "Reverb Bus"}},
		}, "", nil
	case "rack_add_node":
		return map[string]any{"status": "ok", "track_id": cmd["track_id"], "plugin_id": "7001", "plugin_name": "Test Verb"}, "", nil
	}
	return map[string]any{"status": "error", "message": "unexpected command " + name}, "", nil
}

func topologyTestServer(sender harness.KernelSender) *Server {
	return &Server{
		harness:      harness.NewWithSender(sender, nil, nil),
		pending:      map[string]PendingPlan{},
		interactions: map[string]PendingInteraction{},
	}
}

func busTopologyPlanForTest() *semanticTopologyPlan {
	return planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:        "给人声加一点混响",
		ProcessorType:   "reverb",
		SourceTrackID:   "1032",
		SourceTrackName: "vocals",
	})
}

func TestRunSemanticTopologyBusLoadWorkflowPlanShape(t *testing.T) {
	sender := &topologyFakeSender{}
	server := topologyTestServer(sender)
	plan := busTopologyPlanForTest()
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3", PluginIdentifier: "test-verb@1.0"}}
	requestContext := map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), "conv-topo", "给人声加一点混响", requestContext, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3", "plugin_identifier": "test-verb@1.0",
	}, plan)
	if !resp.NeedsConfirmation {
		t.Fatal("bus 计划必须停泊确认（NeedsConfirmation=true）")
	}
	if resp.PlanID == "" {
		t.Fatal("计划必须落 pending（PlanID 非空）")
	}
	if !strings.Contains(resp.Reply, "Reverb Bus") || !strings.Contains(resp.Reply, "vocals") || !strings.Contains(resp.Reply, "1032") {
		t.Fatalf("确认卡文案缺指名合同要素：%s", resp.Reply)
	}
	if strings.Contains(resp.Reply, "当前轨道") {
		t.Fatal("148 合同：绝不回「当前轨道」")
	}
	if !strings.Contains(resp.Reply, "不会写入任何插件参数") {
		t.Fatalf("148 合同：缺「只装载不写参数」句：%s", resp.Reply)
	}
	stored := server.pending[resp.PlanID]
	if len(stored.Decisions) != 0 {
		t.Fatalf("bus 计划 Decisions 应为空（connect+load 在 finish 期派发），got %d", len(stored.Decisions))
	}
	if semanticTopologyPlanFromAny(stored.WorkflowData[semanticTopologyPlanContextKey]) == nil {
		t.Fatal("WorkflowData 必须携带拓扑计划（finish 消费依据）")
	}
	if firstNonEmptyText(stored.WorkflowData, "plugin_path") != "C:/Plugins/TestVerb.vst3" {
		t.Fatal("WorkflowData 必须携带插件身份")
	}
}

// 单测边界如实声明：harness PCA 装载门对 rack_add_node 始终做插件二进制的
// 物理指纹校验（生产路径=真实插件文件+PCA 准入；见 harness.go enforceAgent
// ProcessorLoadGate）。单测没有真实插件文件，装载腿在此处被门拒绝——因此
// 单测钉 connect 腿的真实派发形态与装载命令的构造形态（纯函数），connect+
// load 两腿的完整执行链归真栈烟测腿覆盖（AGENTS §5）。
func TestFinishPluginGrabberLoadWorkflowExecutesConnectLeg(t *testing.T) {
	sender := &topologyFakeSender{}
	server := topologyTestServer(sender)
	plan := busTopologyPlanForTest()
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3", PluginIdentifier: "test-verb@1.0"}}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), "conv-topo", "给人声加一点混响", map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3", "plugin_identifier": "test-verb@1.0",
	}, plan)
	stored := server.pending[resp.PlanID]

	message, replies := server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")
	var connectCmd map[string]any
	for _, cmd := range sender.commands {
		if firstStringFromMap(cmd, "cmd") == "track.send.connect" {
			connectCmd = cmd
		}
	}
	if connectCmd == nil {
		t.Fatal("缺少 track.send.connect 派发")
	}
	if connectCmd["source_track_id"] != "1032" || connectCmd["auto_create_fx_track"] != "Reverb Bus" {
		t.Fatalf("connect 命令形态错误（create 场景）： %+v", connectCmd)
	}
	if _, hasAmount := connectCmd["amount_db"]; hasAmount {
		t.Fatal("connect 不显式传 amount_db（内核首连缺省 -6.0=提案默认；既有边保留用户量）")
	}
	// 装载腿：真实栈上经 PCA 门派发到 FX 轨；单测断言其失败被如实呈现
	// （pca_load_gate 指纹面缺席），不静默不虚构成功。
	if !strings.Contains(message, "总线拓扑装载未完成") || !strings.Contains(message, "pca_load_gate") {
		t.Fatalf("装载腿门拒绝必须如实失败：%s", message)
	}
	if len(replies) < 1 {
		t.Fatal("connect 腿回执必须在 replies 里")
	}
	// 装载命令构造形态（纯函数直测）：目标=connect 回执解析的 FX 轨。
	loadCommand := semanticTopologyBusLoadCommand("2001", stored.WorkflowData)
	if loadCommand["track_id"] != "2001" || loadCommand["plugin_path"] != "C:/Plugins/TestVerb.vst3" || loadCommand["plugin_identifier"] != "test-verb@1.0" {
		t.Fatalf("装载命令形态错误：%+v", loadCommand)
	}
	if loadCommand["plugin_kind"] != "Effect" || loadCommand["zone_id"] != "Z3" {
		t.Fatalf("装载命令缺效果器机架参数：%+v", loadCommand)
	}
}

func TestFinishPluginGrabberLoadWorkflowReuseCaseUsesFxTrackID(t *testing.T) {
	sender := &topologyFakeSender{}
	server := topologyTestServer(sender)
	plan := planSemanticTopologyTemplate(semanticTopologyTemplateInput{
		UserText:      "给人声加一点混响",
		ProcessorType: "reverb",
		SourceTrackID: "1032",
		SendTopology: map[string]any{
			"status":    "ok",
			"fx_tracks": []any{map[string]any{"track_id": "2001", "name": "Reverb Bus"}},
		},
	})
	if plan == nil || plan.FXTrack.Action != semanticTopologyFXActionReuse {
		t.Fatalf("前置失败：应为 reuse，got %+v", plan)
	}
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3"}}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), "conv-topo", "给人声加一点混响", map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3",
	}, plan)
	stored := server.pending[resp.PlanID]
	server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")
	for _, cmd := range sender.commands {
		if firstStringFromMap(cmd, "cmd") == "track.send.connect" {
			if cmd["fx_track_id"] != "2001" {
				t.Fatalf("reuse 场景 connect 必须指名 fx_track_id=2001，got %v", cmd["fx_track_id"])
			}
			if _, has := cmd["auto_create_fx_track"]; has {
				t.Fatal("reuse 场景不应带 auto_create")
			}
		}
	}
}

func TestFinishPluginGrabberLoadWorkflowBusConnectRefusedHonestFailure(t *testing.T) {
	sender := &topologyFakeSender{}
	refused := &topologyRefusingSender{inner: sender}
	server := topologyTestServer(refused)
	plan := busTopologyPlanForTest()
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3"}}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), "conv-topo", "给人声加一点混响", map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3",
	}, plan)
	stored := server.pending[resp.PlanID]
	message, _ := server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")
	if !strings.Contains(message, "总线拓扑装载未完成") {
		t.Fatalf("connect 被拒必须如实失败：%s", message)
	}
	if !strings.Contains(message, "不再自动重试") {
		t.Fatalf("失败腿不得静默重试：%s", message)
	}
	for _, cmd := range sender.commands {
		if firstStringFromMap(cmd, "cmd") == "rack_add_node" {
			t.Fatal("connect 失败后不得继续装载腿")
		}
	}
}

type topologyRefusingSender struct {
	inner *topologyFakeSender
}

func (f *topologyRefusingSender) SendCommand(ctx context.Context, cmd map[string]any) (map[string]any, string, error) {
	if firstStringFromMap(cmd, "cmd") == "track.send.connect" {
		return map[string]any{"status": "error", "message": "fx role conflict"}, "", nil
	}
	return f.inner.SendCommand(ctx, cmd)
}

// ————— 提示词惯例段（软面） —————

func TestCarriersNeutralFamilyCarriesTopologyConventionSection(t *testing.T) {
	bundle := carriers.Assemble(context.Background(), carriers.Options{
		Family:             ruleset.FamilyNeutralFamily,
		ObservationCatalog: "- ccb.observation_catalog: discover",
		AllowedTools:       "ccb.observation_catalog, ccb.observation_request",
	})
	found := false
	for _, section := range bundle.Sections {
		if section.ID == "ctx.layer.rules.semantic_topology_convention" {
			found = true
			for _, want := range []string{"时域效果惯例", "Reverb Bus", "Delay Bus", "不得发明轨道或拓扑", "单轨快速 A/B"} {
				if !strings.Contains(section.Content, want) {
					t.Fatalf("惯例段缺关键句 %q", want)
				}
			}
			if section.CacheKey != "semantic-timefx-bus-convention:v1" {
				t.Fatalf("CacheKey=%s", section.CacheKey)
			}
		}
	}
	if !found {
		ids := make([]string, 0, len(bundle.Sections))
		for _, section := range bundle.Sections {
			ids = append(ids, section.ID)
		}
		t.Fatalf("中性族 bundle 缺惯例段；sections=%v", ids)
	}
}
