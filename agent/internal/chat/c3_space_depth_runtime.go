package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// MIXLAYER-M1-3：C3 空间与深度·只读能力 Session（fine_mix.space_depth.v0）。
// 观察载体三面（全部既有只读面）：
//  1. MOM static_level_relationship —— mix.observe（project_multitrack_relation_observation 意图，B2 同款读面）；
//  2. 通用插件参数快照 —— plugin_grabber_explain_controls（翻译为内核 get_plugin_parameters；
//     plugin_param_guard 依赖面的读端，读后快照进 PluginSnapshotCache，供任何后续写前验证）；
//  3. FXM 手动差分通道 —— 调用方经 req.Context["fxm_measurement"] 携带的双测量原样透传给
//     mixboard 的 FXM 投影组装（mixboard.go finalizeFXMProjection）。本能力不生产自动差分
//     （M2-TIMEFX 线），无手动输入时 FXM 面如实报告缺失。
//
// 边界：无写控制面。内核 plugin_grabber 无 reverb/delay 语义 inspector（MIX-CAP-RECON-1
// §2），空间/深度插件的识别是 plugin_path 词法分类提示，不是语义控制；语义 reverb/delay
// 控制面属后续 C3 控制侧卡。
const spaceDepthCapabilityID = "fine_mix.space_depth.v0"

const c3ObservationSchema = "c3.space_depth.observation.v0"

// c3ReadOnlyTools 是本 runtime 的 harness 调用白名单：所有调用必须经
// c3InvokeReadOnly 走这张表，表外工具 fail-closed。写命令（rack_add_node /
// set_plugin_param / plugin_grabber_apply_* / mix_apply_* 等）一律不在表内
// ——单测以写命令黑名单交叉断言（TestC3ReadOnlyToolWhitelistHasNoWriteCommands）。
var c3ReadOnlyTools = map[string]bool{
	"track.send.list":                 true, // 载体 4：send/return 拓扑结构读回（AGENT-SENDROUTE-SEMANTICS-1；空间观察从插件参数扩展到轨间结构）
	"mix.observe":                     true, // 载体 1+3：MOM static_level + FXM 手动差分通道
	"plugin_grabber_explain_controls": true, // 载体 2：通用插件参数快照读端
}

// c3SpacePluginParamReadLimit 限制单次观察的逐插件参数读取数：插件参数读取
// 是逐内核命令往返，上限保护大工程；超出部分计入报告覆盖说明，不静默丢弃。
const c3SpacePluginParamReadLimit = 8

// c3SpacePluginPathMarkers 空间/深度类插件的词法识别标记（plugin_path 子串，
// 大小写不敏感）。词法识别≠语义 inspector：报告中的 match_marker 字段如实
// 标注命中词，供人工核对分类边界。
var c3SpacePluginPathMarkers = []string{
	"reverb", "delay", "echo", "room", "hall", "chamber", "plate",
	"convolution", "ambience", "space", "spring",
}

// c3SpaceParamMarkers 空间/深度相关参数的词级标记：参数快照摘要优先展示
// 命中词的参数（wet/dry/mix/time/size/decay/feedback 等），其余参数只计
// 数，避免把与空间无关的参数淹没进观察报告。匹配是词级（按 _/-/数字分
// 词后整词相等），防止 threshold 含 hold 这类子串误命中。
var c3SpaceParamMarkers = []string{
	"wet", "dry", "mix", "time", "size", "decay", "feedback", "damping",
	"diffusion", "predelay", "rate", "depth", "spread", "width", "tone",
	"hold", "delay", "echo",
}

type c3SpacePluginObservation struct {
	TrackID     string
	PluginID    string
	PluginPath  string
	Format      string
	Enabled     bool
	MatchMarker string
}

type c3PluginParamSnapshot struct {
	TrackID       string
	PluginID      string
	Status        string // read | error
	ParamCount    int
	SpaceParams   []map[string]any
	SkippedReason string
	Error         string
}

func (s *Server) handleSpaceDepthRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.kernel == nil || s.harness == nil {
		response := capabilityCanaryBlockedResponse(conversationID, goal, "C3 空间与深度观察需要可用的 Project Kernel 与 Harness；当前未启动。")
		response.WorkflowData = map[string]any{
			"capability_id": spaceDepthCapabilityID, "canary_stage": "observation_blocked",
			"read_only": true, "mutation_performed": false,
		}
		return response
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		message := "无法取得当前工程 snapshot，C3 空间与深度观察未启动。"
		if err != nil {
			message += " " + err.Error()
		}
		response := capabilityCanaryBlockedResponse(conversationID, goal, message)
		response.WorkflowData = map[string]any{
			"capability_id": spaceDepthCapabilityID, "canary_stage": "observation_blocked",
			"read_only": true, "mutation_performed": false,
		}
		return response
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, spaceDepthCapabilityID)
	}

	// 载体 1+3：mix.observe 一次取 MOM static_level_relationship 与（若调用方
	// 携带手动差分）FXM 投影。
	observeArgs := c3ObserveArgs(sessionID, req.Message, req.Context["fxm_measurement"])
	observe, observeErr := s.c3InvokeReadOnly(ctx, sessionID, "mix.observe", observeArgs)
	momStatic := map[string]any{}
	fxmProjection := map[string]any{}
	observationID := ""
	if observeErr == nil {
		observationID = firstStringFromMap(observe, "observation_id")
		momProjection := mapValue(observe["mom_projection"])
		if len(momProjection) > 0 {
			momStatic = mapValue(momProjection["static_level_relationship"])
		}
		fxmProjection = mapValue(observe["fxm_projection"])
	}

	// 载体 2：工程态 rack 面的空间/深度插件词法识别 + 通用参数快照读取。
	spacePlugins := c3SpacePluginsFromState(state.LegacyState)
	snapshots := make([]c3PluginParamSnapshot, 0, len(spacePlugins))
	for index, plugin := range spacePlugins {
		if index >= c3SpacePluginParamReadLimit {
			break
		}
		snapshots = append(snapshots, s.c3ReadPluginParams(ctx, sessionID, plugin))
	}

	// 载体 4（AGENT-SENDROUTE-SEMANTICS-1）：send/return 拓扑结构读回——空间观察
	// 从插件参数（词法）扩展到轨间结构事实（track.send.list 读源），恰好绕开
	// MIX-CAP-RECON-1 §2 词法缺口；读失败如实缺席不虚构。
	sendTopology, sendTopologyErr := s.c3InvokeReadOnly(ctx, sessionID, "track.send.list", map[string]any{})
	report := c3AssembleSpaceDepthReport(observationID, momStatic, len(spacePlugins), snapshots, fxmProjection, observeErr)
	report.SendTopology = c3CompactSendTopology(sendTopology, sendTopologyErr)
	workflowData := map[string]any{
		"session_id":         sessionID,
		"capability_id":      spaceDepthCapabilityID,
		"canary_stage":       "observation",
		"read_only":          true,
		"mutation_performed": false,
		"schema":             c3ObservationSchema,
		"observation":        report,
	}
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          c3RenderSpaceDepthReply(report),
		Workflow:       "capability_runtime_v1",
		WorkflowData:   workflowData,
		GoalStatus:     string(agentruntime.StatusCompleted),
	}
}

// c3ObserveArgs 构造 mix.observe 参数（B2 只读上下文同款读面 + FXM 手动差分
// 透传）。fxm_measurement 为 nil 时不带键，mixboard 侧不会产出 FXM 投影。
func c3ObserveArgs(sessionID, message string, manualMeasurement any) map[string]any {
	args := map[string]any{
		"scope": "full_project", "project_context": true, "observation_only": true,
		"disclosure": "digest_catalog", "mom_intent": "project_multitrack_relation_observation",
		"mix_session_id": sessionID, "goal_text": message,
	}
	if manualMeasurement != nil {
		args["fxm_measurement"] = manualMeasurement
	}
	return args
}

// c3InvokeReadOnly 是 C3 的唯一 harness 出口：白名单外的工具名直接报错，
// 保证观察 Session 不出现任何写命令调用点。
func (s *Server) c3InvokeReadOnly(ctx context.Context, sessionID, tool string, args map[string]any) (map[string]any, error) {
	if !c3ReadOnlyTools[tool] {
		return nil, fmt.Errorf("C3 只读白名单拒绝工具 %q（写控制不在 %s）", tool, spaceDepthCapabilityID)
	}
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: tool, Args: args,
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true, "capability_id": spaceDepthCapabilityID},
		Source:  "capability_runtime_v1_c3", Confirmed: true, ToolCallID: "c3:" + sessionID + ":" + tool,
	})
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(resp.Status, "ok") {
		return nil, fmt.Errorf("%s: %s", tool, firstNonEmpty(resp.Error, resp.Status))
	}
	if resp.Result == nil {
		return map[string]any{}, nil
	}
	return resp.Result, nil
}

// c3SpacePluginsFromState 从工程态（kernel get_project_state shape）提取
// 每轨 rack 里的空间/深度类插件。形状与 tim.RackSummariesFromProjectState
// 同源（tracks[].rack.nodes[]），此处按 C3 需求裁剪并做词法分类。
func c3SpacePluginsFromState(state map[string]any) []c3SpacePluginObservation {
	out := []c3SpacePluginObservation{}
	for _, row := range mapRowsFromAny(state["tracks"]) {
		trackID := firstStringFromMap(row, "track_id", "id")
		if trackID == "" {
			continue
		}
		for _, rawNode := range mapRowsFromAny(mapValue(row["rack"])["nodes"]) {
			pluginID := firstStringFromMap(rawNode, "node_id", "plugin_item_id", "item_id", "id", "plugin_id")
			pluginPath := firstStringFromMap(rawNode, "plugin_path", "path")
			if pluginID == "" && pluginPath == "" {
				continue
			}
			marker := c3MatchSpacePluginMarker(pluginPath)
			if marker == "" {
				continue
			}
			enabled, _ := rawNode["enabled"].(bool)
			out = append(out, c3SpacePluginObservation{
				TrackID: trackID, PluginID: pluginID, PluginPath: pluginPath,
				Format: firstStringFromMap(rawNode, "plugin_format", "format"), Enabled: enabled, MatchMarker: marker,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TrackID != out[j].TrackID {
			return out[i].TrackID < out[j].TrackID
		}
		return out[i].PluginID < out[j].PluginID
	})
	return out
}

func c3MatchSpacePluginMarker(pluginPath string) string {
	lower := strings.ToLower(pluginPath)
	for _, marker := range c3SpacePluginPathMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}

func (s *Server) c3ReadPluginParams(ctx context.Context, sessionID string, plugin c3SpacePluginObservation) c3PluginParamSnapshot {
	snapshot := c3PluginParamSnapshot{TrackID: plugin.TrackID, PluginID: plugin.PluginID, Status: "error"}
	result, err := s.c3InvokeReadOnly(ctx, sessionID, "plugin_grabber_explain_controls", map[string]any{
		"track_id": plugin.TrackID, "plugin_id": plugin.PluginID,
		"intent": "space_depth_observation",
	})
	if err != nil {
		snapshot.Error = err.Error()
		return snapshot
	}
	rows := mapRowsFromAny(result["parameters"])
	snapshot.Status = "read"
	snapshot.ParamCount = len(rows)
	snapshot.SpaceParams = c3SpaceParamDigest(rows)
	return snapshot
}

// c3SpaceParamDigest 从参数行提取空间/深度相关参数摘要（id + 原值），其余
// 参数不计入摘要。参数行形状与 plugin_param_guard 的 get_plugin_parameters
// 解析同源（id/param_id/raw_param_id + value/default_value）。
func c3SpaceParamDigest(rows []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		id := firstStringFromMap(row, "id", "param_id", "raw_param_id")
		if id == "" || c3MatchSpaceParamMarker(id) == "" {
			continue
		}
		entry := map[string]any{"param_id": id}
		for _, key := range []string{"value", "current_value", "default_value", "display_value"} {
			if value, ok := row[key]; ok && value != nil {
				entry[key] = value
				break
			}
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return firstStringFromMap(out[i], "param_id") < firstStringFromMap(out[j], "param_id")
	})
	return out
}

func c3MatchSpaceParamMarker(paramID string) string {
	lower := strings.ToLower(paramID)
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.' || (r >= '0' && r <= '9')
	}) {
		for _, marker := range c3SpaceParamMarkers {
			if token == marker {
				return marker
			}
		}
	}
	return ""
}

type c3SpaceDepthReport struct {
	SchemaVersion    string                  `json:"schema_version"`
	SendTopology     map[string]any          `json:"send_topology,omitempty"`
	ObservationID    string                  `json:"observation_id,omitempty"`
	MOMStaticStatus  string                  `json:"mom_static_level_status"`
	MOMCoverage      string                  `json:"mom_static_coverage,omitempty"`
	PluginCandidates int                     `json:"plugin_candidates"`
	PluginsRead      int                     `json:"plugins_read"`
	PluginsSkipped   int                     `json:"plugins_skipped"`
	PluginSnapshots  []c3PluginParamSnapshot `json:"plugin_snapshots,omitempty"`
	FXMStatus        string                  `json:"fxm_manual_diff_status"` // present | missing | n/a
	FXMDelta         map[string]any          `json:"fxm_effect_delta,omitempty"`
	Limitations      []string                `json:"limitations,omitempty"`
	ObserveError     string                  `json:"observe_error,omitempty"`
}

// c3AssembleSpaceDepthReport 是纯组装函数（单测直接喂面数据断言）：
// 三面状态归一 + limitations 汇总。缺失面如实记录，不升级不虚构。
func c3AssembleSpaceDepthReport(observationID string, momStatic map[string]any, pluginCandidates int, snapshots []c3PluginParamSnapshot, fxmProjection map[string]any, observeErr error) c3SpaceDepthReport {
	report := c3SpaceDepthReport{
		SchemaVersion: c3ObservationSchema, ObservationID: observationID,
		PluginCandidates: pluginCandidates, PluginsRead: len(snapshots),
		PluginSnapshots: snapshots,
		FXMStatus:       "missing",
	}
	if observeErr != nil {
		report.ObserveError = observeErr.Error()
		report.Limitations = append(report.Limitations, "mix.observe 未成功返回，MOM/FXM 载体缺席（工程观察未完成，插件快照面独立成立）")
	}
	if len(momStatic) > 0 {
		report.MOMStaticStatus = firstNonEmpty(firstStringFromMap(momStatic, "status"), "unknown")
		if coverage := mapValue(momStatic["coverage"]); len(coverage) > 0 {
			report.MOMCoverage = fmt.Sprintf("tracks=%v usable=%v ready=%v",
				coverage["track_count"], coverage["usable_track_count"], coverage["ready_track_count"])
		}
	} else {
		report.MOMStaticStatus = "missing"
		report.Limitations = append(report.Limitations, "MOM static_level_relationship 缺席（该观察依赖工程声学证据，缺失原样保留不升级）")
	}
	if len(fxmProjection) > 0 {
		report.FXMStatus = "present"
		report.FXMDelta = mapValue(fxmProjection["effect_delta"])
		if len(report.FXMDelta) == 0 {
			report.Limitations = append(report.Limitations, "FXM 投影存在但 effect_delta 为空")
		}
	} else {
		report.Limitations = append(report.Limitations, "FXM 手动差分未提供；本能力不生产自动差分（M2-TIMEFX 线）")
	}
	if pluginCandidates > c3SpacePluginParamReadLimit {
		report.PluginsSkipped = pluginCandidates - c3SpacePluginParamReadLimit
		report.Limitations = append(report.Limitations, fmt.Sprintf("空间类插件 %d 个超出单次参数读取上限 %d，剩余 %d 个仅词法识别未读参数", pluginCandidates, c3SpacePluginParamReadLimit, pluginCandidates-c3SpacePluginParamReadLimit))
	}
	for _, snapshot := range snapshots {
		if snapshot.Status != "read" {
			report.Limitations = append(report.Limitations, fmt.Sprintf("插件 %s/%s 参数快照读取失败：%s", snapshot.TrackID, snapshot.PluginID, snapshot.Error))
		}
	}
	report.Limitations = append(report.Limitations, "空间/深度插件识别为 plugin_path 词法分类（内核无 reverb/delay 语义 inspector），非语义判定")
	return report
}

func c3Count(value any) int {
	if n, ok := value.(float64); ok {
		return int(n)
	}
	if n, ok := value.(int); ok {
		return n
	}
	return 0
}

// c3CompactSendTopology 裁剪 track.send.list 读回为报告面（计数+边行），
// 读失败返回带 send_topology_status=missing 的说明行，不升级不虚构。
func c3CompactSendTopology(topology map[string]any, err error) map[string]any {
	if err != nil {
		return map[string]any{"status": "missing", "reason": "track_send_list_read_failed: " + err.Error()}
	}
	if len(topology) == 0 {
		return nil
	}
	sends := mapRowsFromAny(topology["sends"])
	fxTracks := mapRowsFromAny(topology["fx_tracks"])
	dangling := mapRowsFromAny(topology["dangling"])
	status := firstStringFromMap(topology, "status")
	if status == "" {
		status = "ok"
	}
	return map[string]any{
		"status": status, "send_edge_count": len(sends),
		"fx_track_count": len(fxTracks), "dangling_edge_count": len(dangling),
		"sends": sends, "fx_tracks": fxTracks,
	}
}
func c3RenderSpaceDepthReply(report c3SpaceDepthReport) string {
	var b strings.Builder
	b.WriteString("C3 空间与深度观察完成（schema " + c3ObservationSchema + "，只读）。\n")
	b.WriteString(fmt.Sprintf("插件面：识别到 %d 个空间/深度类插件（plugin_path 词法识别，非语义 inspector），读取 %d 份通用参数快照（get_plugin_parameters 读端）。", report.PluginCandidates, report.PluginsRead))
	if report.PluginsSkipped > 0 {
		b.WriteString(fmt.Sprintf("另有 %d 个超出读取上限，仅识别未读参数。", report.PluginsSkipped))
	}
	b.WriteString("\n")
	if report.ObservationID != "" {
		b.WriteString("静态电平面：MOM static_level_relationship status=" + report.MOMStaticStatus)
		if report.MOMCoverage != "" {
			b.WriteString("（" + report.MOMCoverage + "）")
		}
		b.WriteString("。\n")
	}
	switch report.FXMStatus {
	case "present":
		b.WriteString("FXM 面：携带手动差分，effect_delta 已并入报告。\n")
	default:
		b.WriteString("FXM 面：未提供手动差分；本能力不生产自动差分（M2-TIMEFX 线）。\n")
	}
	if report.SendTopology != nil {
		if sendStatus := firstStringFromMap(report.SendTopology, "status"); sendStatus != "" && sendStatus != "missing" {
			b.WriteString(fmt.Sprintf("结构面：send/return 拓扑 %d 条边、%d 条 FX 轨、%d 条悬空边（track.send.list 结构读回，非插件语义）。\n", c3Count(report.SendTopology["send_edge_count"]), c3Count(report.SendTopology["fx_track_count"]), c3Count(report.SendTopology["dangling_edge_count"])))
		} else {
			b.WriteString("结构面：send 拓扑读回缺席（track.send.list 读取失败），如实缺席不虚构。\n")
		}
	}
	b.WriteString("本观察未创建 Proposal，未注册任何写命令，未修改工程。")
	return b.String()
}
