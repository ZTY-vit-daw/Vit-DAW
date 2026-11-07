package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/rlm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// harnessForD1D3Test 构造带一条 bound render→delivery 绑定的 harness（经内核
// telemetry 面 seed ready render 后走 BindRenderProfile 写端——测试编排用，
// 被测的 D pack 自身不触任何写端）。
func harnessForD1D3Test(t *testing.T) *harness.Harness {
	t.Helper()
	h := harness.NewWithSender(nil, nil, nil)
	h.IngestKernelTelemetry(map[string]any{
		"topic": "render", "subtopic": "render_done", "job_id": "render-wire", "status": "ok", "file_path": "/tmp/render.wav",
	})
	if _, err := h.BindRenderProfile("render-wire", "builtin:apple_music"); err != nil {
		t.Fatalf("test setup: bind render profile: %v", err)
	}
	return h
}

// ————— 注册链完整性契约（模式=C3 完整性契约，MIX-CAP-RECON-1 §3 建议）：
// D1-D3 触达链各注册点使用同一 capability ID，任一点漂移（遗漏/改名/错版本）
// 即显式失败。 —————

func TestD1D3CapabilityRegistrationChainContract(t *testing.T) {
	// 点 1：显式 stage 映射（capability_routing.go capabilityIDForStage）与本
	// runtime 的 ID 一致——D1/D2/D3 三个 stage code 同指一个观察 pack，不留
	// mixing_capability_stage.d* 第二身份。
	for _, stage := range []string{"D1", "D2", "D3", "d1", "d2", "d3"} {
		if got := capabilityIDForStage(stage); got != premasterObservationCapabilityID {
			t.Fatalf("capabilityIDForStage(%q)=%q want %q（stage 映射与 runtime ID 漂移）", stage, got, premasterObservationCapabilityID)
		}
	}
	// D4 版本输出不在观察 pack 范围，维持 fallback。
	if got := capabilityIDForStage("D4"); got != "mixing_capability_stage.d4" {
		t.Fatalf("capabilityIDForStage(D4)=%q want mixing_capability_stage.d4（D4 不属本 pack）", got)
	}
	// 点 2：话术显式标记可解析。
	for _, marker := range []string{"d1 母带前响度峰值观察", "d2 翻译检查", "d3 参考曲对比观察", "pre_master.observation.v0 母带前观察", "D1"} {
		if got := inferProjectAwareCapability(marker); got != premasterObservationCapabilityID {
			t.Fatalf("inferProjectAwareCapability(%q)=%q want %q", marker, got, premasterObservationCapabilityID)
		}
	}
	// 点 3：会话 ID 前缀。
	if got := capabilitySessionBase("conv/测试", premasterObservationCapabilityID); !strings.HasPrefix(got, "cap_v1_d1d3_") {
		t.Fatalf("capabilitySessionBase(...)=%q want cap_v1_d1d3_ 前缀", got)
	}
	// 点 4：显式 capability_id 经 resolveCapabilityOwner 解析为 EngineV1 owner。
	server := New(nil, shadow.New(nil), nil)
	resolution := server.resolveCapabilityOwner("conv-d1d3", ChatRequest{
		Message: "观察一下",
		Context: map[string]any{"capability_id": premasterObservationCapabilityID},
	})
	if resolution.CapabilityID != premasterObservationCapabilityID || resolution.Ambiguous {
		t.Fatalf("resolveCapabilityOwner explicit=%#v", resolution)
	}
	if resolution.Decision.Owner != orchestration.EngineV1 {
		t.Fatalf("resolveCapabilityOwner owner=%q want %q", resolution.Decision.Owner, orchestration.EngineV1)
	}
}

// 普通母带话术不被 D pack 劫持（D 是工程级显式入口，同 C2/C3 反门语义）。
func TestD1D3OrdinaryMasteringTalkIsNotHijacked(t *testing.T) {
	for _, message := range []string{
		"整体响度不够大", "真峰值是不是超了", "小音量下听着怎么样",
		"和参考曲比差在哪", "mono 兼容性如何", "耳机里听着发闷",
		"the mix is too quiet", "check translation on small speakers",
	} {
		if got := inferProjectAwareCapability(message); got == premasterObservationCapabilityID {
			t.Fatalf("inferProjectAwareCapability(%q) 劫持为 D pack（应为空或其他能力）", message)
		}
	}
	// B/C 标记不误入 D。
	if got := inferProjectAwareCapability("c3 观察空间与深度"); got != spaceDepthCapabilityID {
		t.Fatalf("c3 marker resolved=%q", got)
	}
	if got := inferProjectAwareCapability("c2 动态控制"); got != dynamicControlCapabilityID {
		t.Fatalf("c2 marker resolved=%q", got)
	}
	if got := inferProjectAwareCapability("b2 静态平衡"); got != staticBalanceCapabilityID {
		t.Fatalf("b2 marker resolved=%q", got)
	}
}

// canary seam 分发：显式 D 标记消息经 handleCapabilityRuntimeCanary 进入
// handlePreMasterObservationRuntime（nil kernel 下为 observation_blocked 响应，
// 但 capability_id/stage/read_only 字段证明分发落点正确）。
func TestD1D3CanarySeamDispatchesToPreMasterRuntime(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conv-d1d3", ChatRequest{
		Message: "d1 母带前响度峰值观察",
		Context: map[string]any{"agent_mode": "chat"},
	}, agentruntime.Goal{})
	if !handled {
		t.Fatal("canary seam 未接管 D 标记消息（分发缺失）")
	}
	if response.WorkflowData["capability_id"] != premasterObservationCapabilityID {
		t.Fatalf("dispatched capability_id=%#v want %q", response.WorkflowData["capability_id"], premasterObservationCapabilityID)
	}
	if response.WorkflowData["canary_stage"] != "observation_blocked" {
		t.Fatalf("canary_stage=%#v want observation_blocked（nil kernel 环境）", response.WorkflowData["canary_stage"])
	}
	if response.WorkflowData["read_only"] != true || response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("read-only 标记缺失: %#v", response.WorkflowData)
	}
}

// ————— 只读性契约：D1-D3 的 harness 调用白名单不含任何写命令。 —————

func TestD1D3ReadOnlyToolWhitelistHasNoWriteCommands(t *testing.T) {
	// 写命令黑名单：catalog 内核写命令族 + 固定编排批命令族 + mix 写回族 +
	// RenderBindings 写端。
	writeCommands := []string{
		"rack_add_node", "set_plugin_param", "set_plugin_parameters",
		"plugin_grabber_apply_compressor_controls", "plugin_grabber_apply_de_esser_controls",
		"plugin_grabber_apply_limiter_controls", "plugin_grabber_apply_gate_expander_controls",
		"plugin_grabber_apply_transient_shaper_controls", "plugin_grabber_apply_multiband_controls",
		"plugin_grabber_apply_eq_edits", "mix_propose_tick", "mix_apply_tick",
		"mix_apply_static_balance_batch", "mix_apply_pan_layout_batch", "mix_rollback_tick",
		"track.apply_control", "set_transport", "clip.gain.set_batch", "clip.fade.set",
		"remove_track", "add_track", "set_fader", "set_project_key", "set_project_style_note",
		"render_profile_bind",
	}
	for _, command := range writeCommands {
		if d1d3ReadOnlyTools[command] {
			t.Fatalf("D1-D3 白名单含写命令 %q（只读能力 Session 禁止写控制面）", command)
		}
	}
	// 白名单成员本身不得带 mutation 词根（防未来误加 apply/set/add/remove/bind 族）。
	for tool := range d1d3ReadOnlyTools {
		lower := strings.ToLower(tool)
		for _, root := range []string{"apply", "set_", "add_", "remove", "rollback", "_tick", "mutate", "bind"} {
			if strings.Contains(lower, root) {
				t.Fatalf("D1-D3 白名单工具 %q 含写词根 %q", tool, root)
			}
		}
	}
	if len(d1d3ReadOnlyTools) != 2 {
		t.Fatalf("D1-D3 白名单=%v（改动需同步只读性契约测试与卡面声明）", d1d3ReadOnlyTools)
	}
}

// d1d3InvokeReadOnly 对白名单外工具 fail-closed。
func TestD1D3InvokeReadOnlyRejectsNonWhitelistedTool(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	if _, err := server.d1d3InvokeReadOnly(context.Background(), "s", "render_profile_bind", map[string]any{}); err == nil {
		t.Fatal("白名单外工具未被拒绝")
	} else if !strings.Contains(err.Error(), "只读白名单") {
		t.Fatalf("拒绝消息不含白名单语义: %v", err)
	}
}

// ————— 观察载体纯函数面 —————

// 载体 1+2：mix.observe 参数（只读意图 + 不带 MOM 意图的 D 面语义）。
func TestD1D3ObserveArgs(t *testing.T) {
	args := d1d3ObserveArgs("sess-1", "d1 母带前响度峰值观察")
	if args["observation_only"] != true || args["scope"] != "full_project" || args["disclosure"] != "digest_catalog" {
		t.Fatalf("observe args 缺只读工程面: %#v", args)
	}
	if args["mix_session_id"] != "sess-1" || args["goal_text"] != "d1 母带前响度峰值观察" {
		t.Fatalf("observe args 会话/意图缺失: %#v", args)
	}
	if _, exists := args["mom_intent"]; exists {
		t.Fatal("D1-D3 观察不消费 MOM 意图（B2/C3 载体），不应携带 mom_intent")
	}
}

// 载体 2 目标面：工程态 track id 提取（排序去重确定性）。
func TestD1D3TrackIDsFromState(t *testing.T) {
	state := map[string]any{
		"tracks": []any{
			map[string]any{"track_id": "vocal"},
			map[string]any{"id": "bass"},
			map[string]any{"track_id": "vocal"}, // 去重
			map[string]any{"name": "无 id 轨"},    // 跳过
		},
	}
	ids := d1d3TrackIDsFromState(state)
	if len(ids) != 2 || ids[0] != "bass" || ids[1] != "vocal" {
		t.Fatalf("track ids=%v want [bass vocal]", ids)
	}
}

// 载体 2 风险分类：阈值与 project_package phaseRiskCandidates 同款。
func TestD1D3MonoRiskThresholds(t *testing.T) {
	low := 0.1
	negative := 0.5
	healthy := 0.9
	if !d1d3MonoRisk(&low, nil) {
		t.Fatal("correlation<0.25 必须记风险")
	}
	if !d1d3MonoRisk(nil, &negative) {
		t.Fatal("phase_negative_ratio>0.1 必须记风险")
	}
	if d1d3MonoRisk(&healthy, nil) {
		t.Fatal("健康轨不得记风险")
	}
	if d1d3MonoRisk(nil, nil) {
		t.Fatal("双指标缺失不记风险（与投影层一致）")
	}
}

// 三面状态归一：全缺场景如实 missing，不虚构。
func TestD1D3AssembleReportMissingSurfaces(t *testing.T) {
	report := d1d3AssemblePreMasterReport(map[string]any{}, nil, 0, nil, rlm.Projection{Status: rlm.StatusMissing}, "d3 参考")
	if report.LoudnessPeakStatus != "missing" || report.StereoStatus != "missing" {
		t.Fatalf("缺失面未如实标注: %#v", report)
	}
	if report.RLMStatus != rlm.StatusMissing {
		t.Fatalf("RLM 状态未透传: %#v", report)
	}
	joined := strings.Join(report.Limitations, "\n")
	for _, needle := range []string{"参考曲导入/对比命令缺口", "无 render→delivery 绑定", "无可比参考电平指标"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("limitation 缺 %q: %#v", needle, report.Limitations)
		}
	}
	if report.ReferenceTrackGap != d1d3ReferenceTrackGap {
		t.Fatalf("D3 缺口登记文本漂移: %q", report.ReferenceTrackGap)
	}
}

// d1d3RealObservationResponse 是 mix.observe 的 harness 响应真实形状夹具
// （D1D3-DIGEST-WIRING-1 反钉蓝本=265 run 实盘观察包
// obs_20261106T152825_863f2de0d657，coord/runs/D2-STEREO-THIN-1 链路）：
// digest 嵌套在 out["digest"] 子图，顶层只有 status/mix_session_id/
// observation_id/observation/catalog/mixboard 等载体键——面键（peak_dbfs/
// stereo_status/waveform/rankings）顶层从不存在（harness requestMixObservation
// out map 形状；272 取证：按顶层读=结构性恒 missing）。数值与行形状取实盘原值
//（ranking 行的 track_id/name/role_guess/rank/value 字段面如实），行数精简。
func d1d3RealObservationResponse() map[string]any {
	return map[string]any{
		"status":         "ready",
		"mix_session_id": "cap_v1_d1d3_test",
		"observation_id": "obs_20261106T152825_863f2de0d657",
		"observation":    map[string]any{"observation_id": "obs_20261106T152825_863f2de0d657"},
		"catalog":        map[string]any{"keys": []any{}},
		"mixboard":       map[string]any{"path": "/tmp/mixboard"},
		"digest": map[string]any{
			"schema_version": "mix_observation_digest.v1",
			"status":         "ready",
			"scope":          "full_project",
			"peak_dbfs":      -15.873,
			"rms_dbfs":       -30.255,
			"crest_db":       14.383,
			"waveform": map[string]any{
				"status": "ready", "peak_dbfs": -15.873, "rms_dbfs": -30.255, "headroom_db": 15.873, "crest_db": 14.383,
			},
			"stereo_status":       "ready",
			"correlation_estimate": 0.689,
			"correlation_state":   "coherent",
			"balance_db":          -1.475,
			"balance_state":       "centered",
			"project_loudness_ranking_excerpt": []any{
				map[string]any{"track_id": "1032", "name": "vocals", "role_guess": "vocal", "rank": 1, "metric": "rms_dbfs", "value": -19.843, "focused": false},
			},
			"project_headroom_risk_excerpt": []any{
				map[string]any{"track_id": "1032", "name": "vocals", "role_guess": "vocal", "rank": 1, "headroom_db": 5.22, "risk": "low", "focused": false},
			},
			"project_peak_risk_ranking_excerpt": []any{
				map[string]any{"track_id": "1032", "name": "vocals", "role_guess": "vocal", "rank": 1, "peak_dbfs": -5.22, "headroom_db": 5.22, "risk": "low", "focused": false},
			},
			"missing_metrics":  []any{"before_after_delta", "ab_result"},
			"available_detail": map[string]any{"stereo_relation": "ready", "catalog": "ready"},
		},
	}
}

func TestD1D3AssembleReportPresentSurfaces(t *testing.T) {
	reference := -18.0
	rows := []d1d3TrackStereoRow{
		{TrackID: "vocal", Status: "read", CorrelationEstimate: &reference, MonoRisk: false},
		{TrackID: "bass", Status: "read", MonoRisk: true},
		{TrackID: "pad", Status: "missing"},
	}
	rlmProj := rlm.Projection{Status: rlm.StatusReady, Mode: rlm.ModeStrict, SelectedMetric: "integrated_lufs", ReferenceLevel: &reference,
		Summary:        rlm.Summary{TrackCount: 3, EligibleTrackCount: 3, CandidateCount: 3},
		Limitations:    []string{"rlm_x"},
		RenderBindings: []rlm.RenderBindingDisclosure{{RenderID: "render-1", ProfileID: "streaming_-14", Status: rlm.RenderBindingStatusBound}}}
	report := d1d3AssemblePreMasterReport(d1d3RealObservationResponse(), nil, 3, rows, rlmProj, "d1 响度")
	if report.ObservationID != "obs_20261106T152825_863f2de0d657" {
		t.Fatalf("observation_id 未按顶层读取: %q", report.ObservationID)
	}
	if report.LoudnessPeakStatus != "present" || report.GlobalPeakDBFS == nil || *report.GlobalPeakDBFS != -15.873 {
		t.Fatalf("D1 全局面错误（digest 子图未接线）: %#v", report)
	}
	if report.GlobalRMSDBFS == nil || *report.GlobalRMSDBFS != -30.255 || report.GlobalCrestDB == nil || *report.GlobalCrestDB != 14.383 {
		t.Fatalf("D1 全局 rms/crest 错误: %#v", report)
	}
	if report.Waveform["status"] != "ready" || report.Waveform["headroom_db"] != 15.873 {
		t.Fatalf("D1 waveform 面错误: %#v", report.Waveform)
	}
	if len(report.LoudnessRanking) != 1 || len(report.HeadroomRisk) != 1 || len(report.PeakRisk) != 1 || len(report.MissingMetrics) != 2 {
		t.Fatalf("D1 ranking/missing 面丢失: %#v", report)
	}
	if report.StereoStatus != "present" || report.CorrelationEstimate == nil || *report.CorrelationEstimate != 0.689 {
		t.Fatalf("D2 全局 stereo 面错误: %#v", report)
	}
	if report.CorrelationState != "coherent" || report.BalanceDB == nil || *report.BalanceDB != -1.475 || report.BalanceState != "centered" {
		t.Fatalf("D2 stereo 细节面错误: %#v", report)
	}
	if report.StereoSourceStatus != "ready" {
		t.Fatalf("D2 可用性标注丢失: %#v", report)
	}
	if report.TracksConsidered != 3 || report.MonoRiskTracks != 1 {
		t.Fatalf("D2 逐轨计数错误: %#v", report)
	}
	if report.RLMStatus != rlm.StatusReady || report.RLMSelectedMetric != "integrated_lufs" || report.RLMReferenceLevel == nil {
		t.Fatalf("RLM 面错误: %#v", report)
	}
	if report.RenderBindingCount != 1 || len(report.RenderBindings) != 1 || report.RenderBindings[0]["render_id"] != "render-1" {
		t.Fatalf("D3 绑定披露错误: %#v", report)
	}
	if report.UserIntent != "d1 响度" {
		t.Fatalf("D3 user intent 未入报告: %q", report.UserIntent)
	}
}

// 266 形态反钉（D1D3-DIGEST-WIRING-1）：D1 settle 相位 digest.waveform
// status=missing 值全 null 而 rankings 在场（实盘蓝本=266v1 settle
// obs_20261106T171721_b88a5b18ef6f，coord/runs/D3-REF-ALIGN-THIN-1 v1）——
// D1 门 A 断言（run_free_state_d1_smoke.ps1 loudness_peak_status=present）的
// 驱动是 rankings 面在场，waveform 缺失如实披露不升级不虚构。
func TestD1D3AssembleReportSettleWaveformMissingKeepsRankingsPresent(t *testing.T) {
	observe := d1d3RealObservationResponse()
	digest := mapValue(observe["digest"])
	digest["waveform"] = map[string]any{"status": "missing", "peak_dbfs": nil, "rms_dbfs": nil, "headroom_db": nil, "crest_db": nil}
	delete(digest, "peak_dbfs")
	delete(digest, "rms_dbfs")
	delete(digest, "crest_db")
	report := d1d3AssemblePreMasterReport(observe, nil, 0, nil, rlm.Projection{}, "d1 settle")
	if report.LoudnessPeakStatus != "present" {
		t.Fatalf("settle 形态 rankings 在场时 D1 面必须 present（266 门 A 语义）: %#v", report)
	}
	if len(report.LoudnessRanking) != 1 {
		t.Fatalf("rankings 面丢失: %#v", report)
	}
	if report.Waveform["status"] != "missing" {
		t.Fatalf("waveform 缺失必须如实披露（不虚构实值）: %#v", report.Waveform)
	}
	if report.StereoStatus != "present" || report.CorrelationEstimate == nil {
		t.Fatalf("settle 形态 stereo 面（266 实盘 ready）错误: %#v", report)
	}
}

// 顶层形状防回退（D1D3-DIGEST-WIRING-1）：真实响应顶层从不存在面键——喂顶层
// 形状（旧单测的假形状）必须投影 missing，杜绝顶层/digest 双形状自洽假绿。
func TestD1D3AssembleReportTopLevelFaceKeysStayMissing(t *testing.T) {
	// 旧假形状：面键直接挂在响应顶层（历史上曾把该形状钉进单测造成假绿）。
	topLevelShape := map[string]any{
		"status":         "ready",
		"observation_id": "obs-top",
		"peak_dbfs":      -0.8,
		"rms_dbfs":       -14.2,
		"crest_db":       12.5,
		"waveform":       map[string]any{"status": "ready", "headroom_db": -0.8},
		"stereo_status":  "ready",
		"correlation_estimate": 0.82,
		"balance_db":           0.4,
		"available_detail":     map[string]any{"stereo_relation": "ready"},
		"project_loudness_ranking_excerpt": []any{map[string]any{"track_id": "vocal", "value": -9.0}},
		"project_headroom_risk_excerpt":    []any{map[string]any{"track_id": "kick"}},
		"missing_metrics":                  []any{"integrated_lufs"},
	}
	report := d1d3AssemblePreMasterReport(topLevelShape, nil, 0, nil, rlm.Projection{}, "d1 响度")
	if report.LoudnessPeakStatus != "missing" {
		t.Fatalf("顶层形状不得判 present（真实响应顶层无面键）: %#v", report)
	}
	if report.StereoStatus != "missing" {
		t.Fatalf("顶层形状 stereo 面不得判 present: %#v", report)
	}
	if report.GlobalPeakDBFS != nil || len(report.Waveform) != 0 || len(report.LoudnessRanking) != 0 {
		t.Fatalf("顶层形状不得漏进投影面值: %#v", report)
	}
	joined := strings.Join(report.Limitations, "\n")
	if !strings.Contains(joined, "全局 stereo_relation 缺席") {
		t.Fatalf("stereo 缺席 limitation 未如实记录: %#v", report.Limitations)
	}
}

// 拍平面兜底（D1D3-DIGEST-WIRING-1）：digest 子图缺席而 out["acoustic_digest"]
// 拍平面在场时按其真实键名消费——stereo 键名 stereo_relation_status（harness
// mixObservationAcousticDigest，非 stereo_status）。键名差异在此归一。
func TestD1D3AssembleReportAcousticDigestFlattenedFallback(t *testing.T) {
	observe := map[string]any{
		"status":         "ready",
		"observation_id": "obs-flat",
		"acoustic_digest": map[string]any{
			"status":               "ready",
			"stereo_relation_status": "ready",
			"correlation_estimate":   0.42,
			"balance_db":             0.887,
			"peak_dbfs":              -6.2,
			"waveform":               map[string]any{"status": "ready", "peak_dbfs": -6.2},
			"missing_metrics":        []any{"ab_result"},
		},
	}
	report := d1d3AssemblePreMasterReport(observe, nil, 0, nil, rlm.Projection{}, "d1 响度")
	if report.StereoStatus != "present" || report.CorrelationEstimate == nil || *report.CorrelationEstimate != 0.42 {
		t.Fatalf("拍平面 stereo_relation_status 键名差异未归一: %#v", report)
	}
	if report.BalanceDB == nil || *report.BalanceDB != 0.887 {
		t.Fatalf("拍平面 balance_db 未消费: %#v", report)
	}
	if report.GlobalPeakDBFS == nil || *report.GlobalPeakDBFS != -6.2 {
		t.Fatalf("拍平面全局 peak 未消费: %#v", report)
	}
	if len(report.MissingMetrics) != 1 {
		t.Fatalf("拍平面 missing_metrics 未消费: %#v", report)
	}
}

// 视图无污染契约：d1d3DigestView 的键名归一不得写回响应本体（digest 子图与
// 拍平面在响应内保持原样）。
func TestD1D3DigestViewDoesNotMutateResponse(t *testing.T) {
	observe := map[string]any{
		"digest":          map[string]any{"status": "ready"},
		"acoustic_digest": map[string]any{"stereo_relation_status": "ready", "peak_dbfs": -3.0},
	}
	view := d1d3DigestView(observe)
	if view["stereo_status"] != "ready" || view["peak_dbfs"] != -3.0 {
		t.Fatalf("视图归一错误: %#v", view)
	}
	digest := mapValue(observe["digest"])
	flattened := mapValue(observe["acoustic_digest"])
	if _, polluted := digest["stereo_status"]; polluted {
		t.Fatal("视图归一污染了响应 digest 子图")
	}
	if _, polluted := digest["peak_dbfs"]; polluted {
		t.Fatal("视图归一污染了响应 digest 子图")
	}
	if _, polluted := flattened["stereo_status"]; polluted {
		t.Fatal("视图归一污染了响应拍平面")
	}
}

// 超限覆盖说明：逐轨读取上限如实记 skipped（12 轨读 8，skipped=4）。
func TestD1D3AssembleReportTrackLimitDisclosure(t *testing.T) {
	report := d1d3AssemblePreMasterReport(map[string]any{}, nil, 12, nil, rlm.Projection{}, "")
	if report.TracksSkipped != 4 {
		t.Fatalf("skipped=%d want 4", report.TracksSkipped)
	}
	joined := strings.Join(report.Limitations, "\n")
	if !strings.Contains(joined, "超出单次逐轨 stereo 读取上限") {
		t.Fatalf("覆盖说明缺失: %#v", report.Limitations)
	}
}

// RLM 入参接线（D3 面）：user intent+工程态修订键+绑定快照透传。
func TestD1D3BuildRLMProjectionInputWiring(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.harness = harnessForD1D3Test(t)
	state := &kernel.VSPStateResult{SnapshotHash: "snap-1"}
	state.Revision = 7
	state.ProjectEpoch = "2"
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": "vocal"}}}
	projection := server.d1d3BuildRLMProjection(ChatRequest{Message: "d3 参考曲对比观察"}, state, map[string]any{"observation_id": "obs-2"})
	if projection.SchemaVersion != rlm.SchemaVersion {
		t.Fatalf("RLM schema=%q want %q（现役投影）", projection.SchemaVersion, rlm.SchemaVersion)
	}
	if len(projection.Rows) == 0 {
		t.Fatalf("工程态轨面未进入 RLM 行: %#v", projection)
	}
	if len(projection.RenderBindings) != 1 || projection.RenderBindings[0].RenderID != "render-wire" {
		t.Fatalf("绑定快照未透传 RLM 披露: %#v", projection.RenderBindings)
	}
}

// 冒测 needle 契约：渲染回复必须含 schema 标识、三面行与只读声明
// （fs_largeproject_smoke -D1D3Observation 的 needle 锚在这些行上，文案变化
// 须同步脚本）。
func TestD1D3RenderPreMasterReplyNeedles(t *testing.T) {
	report := d1d3AssemblePreMasterReport(map[string]any{}, nil, 0, nil, rlm.Projection{Status: rlm.StatusMissing}, "d1 观")
	reply := d1d3RenderPreMasterReply(report)
	for _, needle := range []string{
		"D1-D3 母带前观察完成", "d1d3.premaster.observation.v0", "只读",
		"D1 响度峰值面", "D2 翻译检查面", "D3 参考曲对比面",
		"参考曲导入/对比命令缺口", "未注册任何写命令，未修改工程",
	} {
		if !strings.Contains(reply, needle) {
			t.Fatalf("回复缺 needle %q: %s", needle, reply)
		}
	}
	if strings.Contains(reply, "Proposal 已创建") || strings.Contains(reply, "已应用") || strings.Contains(reply, "已绑定") {
		t.Fatalf("只读回复出现写语义: %s", reply)
	}
}
