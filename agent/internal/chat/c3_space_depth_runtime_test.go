package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// ————— 注册链完整性契约（模式=fastpath.DefaultEntryNames 完整性契约，
// MIX-CAP-RECON-1 §3 建议）：C3 触达链各注册点使用同一 capability ID，
// 任一点漂移（遗漏/改名/错版本）即显式失败。 —————

func TestC3CapabilityRegistrationChainContract(t *testing.T) {
	// 点 1：显式 stage 映射（capability_routing.go capabilityIDForStage）与本
	// runtime 的 ID 一致——C1/C2 的 stage 映射是 v0 而 runtime 用 v1 的历史
	// 漂移不允许在 C3 复现（同 ID 单一真相源）。
	if got := capabilityIDForStage("C3"); got != spaceDepthCapabilityID {
		t.Fatalf("capabilityIDForStage(C3)=%q want %q（stage 映射与 runtime ID 漂移）", got, spaceDepthCapabilityID)
	}
	// 点 2：话术显式标记可解析（两种标记形态）。
	for _, marker := range []string{"c3 观察空间与深度", "fine_mix.space_depth.v0 全工程观察", "C3"} {
		if got := inferProjectAwareCapability(marker); got != spaceDepthCapabilityID {
			t.Fatalf("inferProjectAwareCapability(%q)=%q want %q", marker, got, spaceDepthCapabilityID)
		}
	}
	// 点 3：会话 ID 前缀。
	if got := capabilitySessionBase("conv/测试", spaceDepthCapabilityID); !strings.HasPrefix(got, "cap_v1_c3_") {
		t.Fatalf("capabilitySessionBase(...)=%q want cap_v1_c3_ 前缀", got)
	}
	// 点 4：显式 capability_id 经 resolveCapabilityOwner 解析为 EngineV1 owner。
	server := New(nil, shadow.New(nil), nil)
	resolution := server.resolveCapabilityOwner("conv-c3", ChatRequest{
		Message: "观察一下",
		Context: map[string]any{"capability_id": spaceDepthCapabilityID},
	})
	if resolution.CapabilityID != spaceDepthCapabilityID || resolution.Ambiguous {
		t.Fatalf("resolveCapabilityOwner explicit=%#v", resolution)
	}
	if resolution.Decision.Owner != orchestration.EngineV1 {
		t.Fatalf("resolveCapabilityOwner owner=%q want %q", resolution.Decision.Owner, orchestration.EngineV1)
	}
}

// 普通空间话术不被 C3 劫持（C3 是工程级显式入口，同 C2 反门语义）。
func TestC3OrdinarySpaceTalkIsNotHijacked(t *testing.T) {
	for _, message := range []string{
		"让人声更空一点", "这个混响太大了", "空间感不够", "帮我加深深度",
		"the space feels small", "增加一点 room 感",
	} {
		if got := inferProjectAwareCapability(message); got == spaceDepthCapabilityID {
			t.Fatalf("inferProjectAwareCapability(%q) 劫持为 C3（应为空或其他能力）", message)
		}
	}
	// C1/C2 标记不误入 C3。
	if got := inferProjectAwareCapability("c2 动态控制"); got != dynamicControlCapabilityID {
		t.Fatalf("c2 marker resolved=%q", got)
	}
	if got := inferProjectAwareCapability("c1 频段清理"); got != frequencyCleanupCapabilityID {
		t.Fatalf("c1 marker resolved=%q", got)
	}
}

// canary seam 分发：显式 C3 上下文经 handleCapabilityRuntimeCanary 进入
// handleSpaceDepthRuntime（nil kernel 下为 observation_blocked 响应，
// 但 capability_id/stage/read_only 字段证明分发落点正确）。
func TestC3CanarySeamDispatchesToSpaceDepthRuntime(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conv-c3", ChatRequest{
		Message: "c3 观察空间与深度",
		Context: map[string]any{"agent_mode": "chat"},
	}, agentruntime.Goal{})
	if !handled {
		t.Fatal("canary seam 未接管 C3 消息（分发缺失）")
	}
	if response.WorkflowData["capability_id"] != spaceDepthCapabilityID {
		t.Fatalf("dispatched capability_id=%#v want %q", response.WorkflowData["capability_id"], spaceDepthCapabilityID)
	}
	if response.WorkflowData["canary_stage"] != "observation_blocked" {
		t.Fatalf("canary_stage=%#v want observation_blocked（nil kernel 环境）", response.WorkflowData["canary_stage"])
	}
	if response.WorkflowData["read_only"] != true || response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("read-only 标记缺失: %#v", response.WorkflowData)
	}
}

// ————— 只读性契约：C3 的 harness 调用白名单不含任何写命令。 —————

func TestC3ReadOnlyToolWhitelistHasNoWriteCommands(t *testing.T) {
	// 写命令黑名单：catalog 内核写命令族 + 固定编排批命令族 + mix 写回族。
	writeCommands := []string{
		"rack_add_node", "set_plugin_param", "set_plugin_parameters",
		"plugin_grabber_apply_compressor_controls", "plugin_grabber_apply_de_esser_controls",
		"plugin_grabber_apply_limiter_controls", "plugin_grabber_apply_gate_expander_controls",
		"plugin_grabber_apply_transient_shaper_controls", "plugin_grabber_apply_multiband_controls",
		"plugin_grabber_apply_eq_edits", "mix_propose_tick", "mix_apply_tick",
		"mix_apply_static_balance_batch", "mix_apply_pan_layout_batch", "mix_rollback_tick",
		"track.apply_control", "set_transport", "clip.gain.set_batch", "clip.fade.set",
		"remove_track", "add_track", "set_fader", "set_project_key", "set_project_style_note",
	}
	for _, command := range writeCommands {
		if c3ReadOnlyTools[command] {
			t.Fatalf("C3 白名单含写命令 %q（只读能力 Session 禁止写控制面）", command)
		}
	}
	// 白名单成员本身不得带 mutation 词根（防未来误加 apply/set/add/remove 族）。
	for tool := range c3ReadOnlyTools {
		lower := strings.ToLower(tool)
		for _, root := range []string{"apply", "set_", "add_", "remove", "rollback", "_tick", "mutate"} {
			if strings.Contains(lower, root) {
				t.Fatalf("C3 白名单工具 %q 含写词根 %q", tool, root)
			}
		}
	}
	if len(c3ReadOnlyTools) != 2 {
		t.Fatalf("C3 白名单=%v（改动需同步只读性契约测试与卡面声明）", c3ReadOnlyTools)
	}
}

// c3InvokeReadOnly 对白名单外工具 fail-closed。
func TestC3InvokeReadOnlyRejectsNonWhitelistedTool(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	if _, err := server.c3InvokeReadOnly(context.Background(), "s", "rack_add_node", map[string]any{}); err == nil {
		t.Fatal("白名单外工具未被拒绝")
	} else if !strings.Contains(err.Error(), "只读白名单") {
		t.Fatalf("拒绝消息不含白名单语义: %v", err)
	}
}

// ————— 观察载体三面纯函数面 —————

// 载体 1+3：mix.observe 参数（MOM 意图 + FXM 手动差分透传语义）。
func TestC3ObserveArgsCarriesMOMIntentAndManualFXM(t *testing.T) {
	args := c3ObserveArgs("sess-1", "c3 观察空间与深度", nil)
	if args["mom_intent"] != "project_multitrack_relation_observation" || args["observation_only"] != true {
		t.Fatalf("observe args 缺 MOM 只读意图: %#v", args)
	}
	if _, exists := args["fxm_measurement"]; exists {
		t.Fatal("无手动差分时不应携带 fxm_measurement 键")
	}
	manual := map[string]any{"baseline": map[string]any{"rms_dbfs": -20.0}, "processed": map[string]any{"rms_dbfs": -17.5}}
	args = c3ObserveArgs("sess-1", "goal", manual)
	if args["fxm_measurement"] == nil {
		t.Fatal("手动差分未透传给 mixboard FXM 组装")
	}
}

// 载体 2：rack 面空间/深度插件词法识别。
func TestC3SpacePluginsFromState(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "vocal", "rack": map[string]any{"nodes": []any{
				map[string]any{"node_id": "n1", "plugin_path": "C:/Plugins/TSAR-1Reverb.vst3", "enabled": true},
				map[string]any{"node_id": "n2", "plugin_path": "C:/Plugins/CompressorX.vst3"},
			}}},
			map[string]any{"track_id": "lead", "rack": map[string]any{"nodes": []any{
				map[string]any{"node_id": "n3", "plugin_path": "C:/Plugins/DubDelay.dll"},
			}}},
			map[string]any{"track_id": "bass"}, // 无 rack：跳过
			map[string]any{"rack": map[string]any{"nodes": []any{map[string]any{"node_id": "n4", "plugin_path": "xReverb"}}}}, // 无 track_id：跳过
		},
	}
	plugins := c3SpacePluginsFromState(state)
	if len(plugins) != 2 {
		t.Fatalf("识别数=%d want 2: %#v", len(plugins), plugins)
	}
	// 排序契约：按 (track_id, plugin_id) 稳定排序，lead < vocal。
	if plugins[0].TrackID != "lead" || plugins[0].PluginID != "n3" || plugins[0].MatchMarker != "delay" {
		t.Fatalf("lead 插件识别错误: %#v", plugins[0])
	}
	if plugins[1].TrackID != "vocal" || plugins[1].PluginID != "n1" || plugins[1].MatchMarker != "reverb" || !plugins[1].Enabled {
		t.Fatalf("vocal 插件识别错误: %#v", plugins[1])
	}
}

// 空间参数摘要只保留命中词法标记的参数。
func TestC3SpaceParamDigest(t *testing.T) {
	rows := []map[string]any{
		{"id": "wet_level", "value": 0.35},
		{"id": "room_size", "value": 0.6},
		{"id": "threshold_db", "value": -20.0},
		{"id": "attack_ms", "value": 5.0},
	}
	digest := c3SpaceParamDigest(rows)
	if len(digest) != 2 {
		t.Fatalf("摘要数=%d want 2: %#v", len(digest), digest)
	}
	ids := []string{firstStringFromMap(digest[0], "param_id"), firstStringFromMap(digest[1], "param_id")}
	if ids[0] != "room_size" || ids[1] != "wet_level" {
		t.Fatalf("摘要排序/内容错误: %#v", ids)
	}
	if digest[1]["value"] != 0.35 {
		t.Fatalf("参数值丢失: %#v", digest[1])
	}
}

// 三面状态归一：全缺场景如实 missing，不虚构。
func TestC3AssembleSpaceDepthReportMissingSurfaces(t *testing.T) {
	report := c3AssembleSpaceDepthReport("", map[string]any{}, 0, nil, map[string]any{}, nil)
	if report.MOMStaticStatus != "missing" || report.FXMStatus != "missing" {
		t.Fatalf("缺失面未如实标注: %#v", report)
	}
	if report.PluginCandidates != 0 || report.PluginsRead != 0 {
		t.Fatalf("插件面计数错误: %#v", report)
	}
	joined := strings.Join(report.Limitations, "\n")
	if !strings.Contains(joined, "不生产自动差分") {
		t.Fatalf("FXM 边界 limitation 缺失: %#v", report.Limitations)
	}
	if !strings.Contains(joined, "static_level_relationship 缺席") {
		t.Fatalf("MOM 缺席 limitation 缺失: %#v", report.Limitations)
	}
}

func TestC3AssembleSpaceDepthReportPresentSurfaces(t *testing.T) {
	mom := map[string]any{
		"status":   "ready",
		"coverage": map[string]any{"track_count": 12, "usable_track_count": 10, "ready_track_count": 9},
	}
	fxm := map[string]any{"effect_delta": map[string]any{"rms_delta_db": -2.5}}
	snapshots := []c3PluginParamSnapshot{{TrackID: "vocal", PluginID: "n1", Status: "read", ParamCount: 14, SpaceParams: []map[string]any{{"param_id": "wet_level", "value": 0.35}}}}
	report := c3AssembleSpaceDepthReport("obs-1", mom, 1, snapshots, fxm, nil)
	if report.MOMStaticStatus != "ready" || !strings.Contains(report.MOMCoverage, "tracks=12") {
		t.Fatalf("MOM 摘要错误: %#v", report)
	}
	if report.FXMStatus != "present" || report.FXMDelta["rms_delta_db"] != -2.5 {
		t.Fatalf("FXM 摘要错误: %#v", report)
	}
	if report.PluginsRead != 1 || report.PluginSnapshots[0].ParamCount != 14 {
		t.Fatalf("插件快照摘要错误: %#v", report)
	}
}

// 冒测 needle 契约：渲染回复必须含 schema 标识与只读声明（fs_largeproject_smoke
// -C3Observation 的 needle 锚在这两行上，文案变化须同步脚本）。
func TestC3RenderSpaceDepthReplyNeedles(t *testing.T) {
	report := c3AssembleSpaceDepthReport("", map[string]any{}, 0, nil, map[string]any{}, nil)
	reply := c3RenderSpaceDepthReply(report)
	for _, needle := range []string{"C3 空间与深度观察完成", "c3.space_depth.observation.v0", "只读", "未注册任何写命令，未修改工程"} {
		if !strings.Contains(reply, needle) {
			t.Fatalf("回复缺 needle %q: %s", needle, reply)
		}
	}
	if strings.Contains(reply, "Proposal 已创建") || strings.Contains(reply, "已应用") {
		t.Fatalf("只读回复出现写语义: %s", reply)
	}
}
