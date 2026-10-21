package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
)

// TIMEFX-BUS-WRITEGATE-1 验收单测：总线拓扑写门（确认/执行期拓扑校验）。
// 纯函数分支（幻觉源轨/幻觉 send 目标/create 新建豁免/观察不可读 fail-closed）
// + 计划构建面拒载（零停泊零派发）+ 派发面前拒载（停泊后状态漂移/持久化
// 往返形态，connect 派发前拦停）。K2 拦截面（send_route_workflow.go）零
// 改由 send_route_guard_test.go 既有回归覆盖。

func writeGateObservedState() map[string]any {
	return map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "1032", "track_name": "vocals"},
			map[string]any{"track_id": "1027", "track_name": "piano"},
			map[string]any{"track_id": "2001", "track_name": "Reverb Bus"},
		},
	}
}

func writeGateBusPlan(sourceTrackID, fxAction, fxTrackID string) *semanticTopologyPlan {
	plan := busTopologyPlanForTest()
	plan.Sends = []semanticTopologySendRef{{
		SourceTrackID:   sourceTrackID,
		SourceTrackName: "vocals",
		DefaultAmountDB: semanticTopologyDefaultSendAmountDB,
	}}
	plan.FXTrack = semanticTopologyFXTrackRef{Action: fxAction, Name: "Reverb Bus", TrackID: fxTrackID}
	return plan
}

func TestSemanticTopologyWriteGatePassesCreateWithObservedSource(t *testing.T) {
	if err := semanticTopologyWriteGateIssue(writeGateBusPlan("1032", semanticTopologyFXActionCreate, ""), writeGateObservedState()); err != nil {
		t.Fatalf("create 形态（FX 轨=本提案新建清单）必须放行：%v", err)
	}
}

func TestSemanticTopologyWriteGatePassesReuseWithObservedTarget(t *testing.T) {
	if err := semanticTopologyWriteGateIssue(writeGateBusPlan("1032", semanticTopologyFXActionReuse, "2001"), writeGateObservedState()); err != nil {
		t.Fatalf("reuse 形态（源轨与 send 目标均在观察披露）必须放行：%v", err)
	}
}

func TestSemanticTopologyWriteGateRefusesHallucinatedSource(t *testing.T) {
	err := semanticTopologyWriteGateIssue(writeGateBusPlan("999999", semanticTopologyFXActionCreate, ""), writeGateObservedState())
	if err == nil || !strings.Contains(err.Error(), "999999") || !strings.Contains(err.Error(), "幻觉拓扑") {
		t.Fatalf("幻觉源轨必须拒载并点名 ID：got %v", err)
	}
}

func TestSemanticTopologyWriteGateRefusesHallucinatedSendTarget(t *testing.T) {
	err := semanticTopologyWriteGateIssue(writeGateBusPlan("1032", semanticTopologyFXActionReuse, "888888"), writeGateObservedState())
	if err == nil || !strings.Contains(err.Error(), "888888") || !strings.Contains(err.Error(), "send 目标") {
		t.Fatalf("幻觉 send 目标必须拒载并点名 ID：got %v", err)
	}
}

func TestSemanticTopologyWriteGateRefusesEmptySourceID(t *testing.T) {
	err := semanticTopologyWriteGateIssue(writeGateBusPlan("  ", semanticTopologyFXActionCreate, ""), writeGateObservedState())
	if err == nil || !strings.Contains(err.Error(), "空源轨") {
		t.Fatalf("空源轨 ID 必须拒载：got %v", err)
	}
}

func TestSemanticTopologyWriteGateFailClosedOnUnreadableObservation(t *testing.T) {
	for _, state := range []map[string]any{nil, {}, {"tracks": []any{}}} {
		err := semanticTopologyWriteGateIssue(writeGateBusPlan("1032", semanticTopologyFXActionCreate, ""), state)
		if err == nil || !strings.Contains(err.Error(), "观察披露不可读") {
			t.Fatalf("观察不可读必须诚实拒载（fail-closed）：state=%v got %v", state, err)
		}
	}
}

func TestSemanticTopologyWriteGateZeroTouchForNonBusPlans(t *testing.T) {
	if err := semanticTopologyWriteGateIssue(nil, nil); err != nil {
		t.Fatalf("nil 计划零接触：%v", err)
	}
	plan := busTopologyPlanForTest()
	plan.Mode = semanticTopologyModeInlineException
	if err := semanticTopologyWriteGateIssue(plan, nil); err != nil {
		t.Fatalf("inline 例外走既有单轨装载面，不在写门范围：%v", err)
	}
}

// 计划构建面：幻觉拓扑在停泊之前拒载——零停泊、零内核命令。
func TestRunSemanticTopologyBusLoadWorkflowRefusesHallucinatedPlan(t *testing.T) {
	sender := &topologyFakeSender{}
	server := topologyTestServer(sender)
	plan := writeGateBusPlan("999999", semanticTopologyFXActionCreate, "")
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3"}}
	resp := server.runSemanticTopologyBusLoadWorkflow(context.Background(), "conv-gate", "给人声加一点混响", map[string]any{
		semanticTopologyPlanContextKey:             semanticTopologyPlanPayload(plan),
		"semantic_plugin_recommendation_selection": true,
	}, map[string]any{
		"cmd": "plugin_grabber_load", "plugin_name": "Test Verb", "plugin_path": "C:/Plugins/TestVerb.vst3",
	}, plan)
	if resp.Error != "topology_write_gate_refused" {
		t.Fatalf("拒载错误码缺失：error=%q reply=%s", resp.Error, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "999999") || !strings.Contains(resp.Reply, "没有执行任何连接或装载") {
		t.Fatalf("拒载文案必须点名幻觉 ID 并声明零执行：%s", resp.Reply)
	}
	if resp.NeedsConfirmation || resp.PlanID != "" || len(server.pending) != 0 {
		t.Fatalf("幻觉计划不得停泊：needs=%v plan=%q pending=%d", resp.NeedsConfirmation, resp.PlanID, len(server.pending))
	}
	for _, cmd := range sender.commands {
		if name := firstStringFromMap(cmd, "cmd"); name != "get_project_state" {
			t.Fatalf("拒载路径只允许写门自身的只读观察（get_project_state），got %s", name)
		}
	}
}

// 派发面前（executeSemanticTopologyBusLegs）：确认执行期发现引用轨不在
// 观察披露（停泊后状态漂移/持久化往返形态）——connect 派发前拦停，失败
// 如实呈现且不重试。
func TestFinishPluginGrabberLoadWorkflowWriteGateRefusesBeforeDispatch(t *testing.T) {
	sender := &topologyFakeSender{}
	server := topologyTestServer(sender)
	plan := writeGateBusPlan("777777", semanticTopologyFXActionReuse, "888888")
	plan.Loads = []semanticTopologyLoadRef{{PluginName: "Test Verb", PluginPath: "C:/Plugins/TestVerb.vst3"}}
	stored := PendingPlan{
		ID:        "plan_gate_" + randomID(),
		CreatedAt: time.Now(),
		Decisions: []policy.Decision{},
		Context:   harness.AuthorizeSendTopologyConnect(map[string]any{"user_message": "给人声加一点混响"}, []string{"777777"}),
		Workflow:  pluginGrabberLoadCommand,
		WorkflowData: map[string]any{
			"cmd":                          "plugin_grabber_load_and_get_params",
			semanticTopologyPlanContextKey: semanticTopologyPlanPayload(plan),
			"plugin_name":                  "Test Verb",
			"plugin_path":                  "C:/Plugins/TestVerb.vst3",
			"plugin_identifier":            "test-verb@1.0",
			"source_track_id":              plan.Sends[0].SourceTrackID,
			"fx_track_action":              plan.FXTrack.Action,
			"fx_track_name":                plan.FXTrack.Name,
		},
	}
	message, replies := server.finishPluginGrabberLoadWorkflow(context.Background(), stored, nil, "已确认。")
	if !strings.Contains(message, "拓扑写门拒载") || !strings.Contains(message, "不再自动重试") {
		t.Fatalf("写门拒载必须如实呈现且不重试：%s", message)
	}
	if len(replies) != 0 {
		t.Fatalf("写门拒载零回执（未派发任何命令），got %d", len(replies))
	}
	for _, cmd := range sender.commands {
		if name := firstStringFromMap(cmd, "cmd"); name == "track.send.connect" || name == "rack_add_node" {
			t.Fatalf("写门拒载后不得派发 %s：%+v", name, cmd)
		}
	}
}
