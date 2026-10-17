package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
)

// AGENT-SENDROUTE-SEMANTICS-1（设计件 §2.2 send 分支）：「给 vocals 和 guitar
// 共用一个板式混响」类共享话术的 send 结构工作流。计划决定面=每源一条
// track.send.connect（auto_create 同名 FX 轨，内核按名幂等复用，无需跨命令
// ID 传播）；效果器装载保持现状装载面（FX 轨自己的机架，PCA 准入不变），
// 不在本工作流里旁路。误判走 PendingPlan 用户确认面拦截。

const sendTopologyConnectWorkflow = "send_topology_connect"

// sendTopologyConnectRequest 是话术侧解析出的 send 结构请求。
type sendTopologyConnectRequest struct {
	Sources     []sendSourceTrack
	FXTrackName string
	ShareSignal string
}

// coerceSendTopologyConnectRequest 判定一条话术是否为具体的 send 结构请求：
// 共享信号（词法命中，如实回传命中词）+ 话术内可按名解析 ≥2 条工程轨。
// 不满足时返回 false，话术走既有路径（宽泛请求仍被观察优先闸拦截）。
func (s *Server) coerceSendTopologyConnectRequest(ctx context.Context, userText string, requestContext map[string]any) (sendTopologyConnectRequest, bool) {
	share := SendShareSignalText(userText)
	if share == "" {
		return sendTopologyConnectRequest{}, false
	}
	if s == nil || s.harness == nil {
		return sendTopologyConnectRequest{}, false
	}
	state := s.harness.UserStateSummary(ctx)
	sources := resolveSendSourceTracks(userText, state)
	// 源轨数不足 2 的共享话术不是可执行的 send 结构（单轨走既有 insert 面）。
	if len(sources) < 2 {
		return sendTopologyConnectRequest{}, false
	}
	return sendTopologyConnectRequest{
		Sources:     sources,
		FXTrackName: sendFXTrackNameForPluginQuery(extractPluginLoadQuery(userText)),
		ShareSignal: share,
	}, true
}

// runSendTopologyConnectWorkflow 构建 send 结构 PendingPlan：回路+各源发送量
// 呈现，确认后逐源 connect。
func (s *Server) runSendTopologyConnectWorkflow(ctx context.Context, conversationID, userText string, requestContext map[string]any, req sendTopologyConnectRequest) ChatResponse {
	commands := make([]map[string]any, 0, len(req.Sources))
	for _, source := range req.Sources {
		commands = append(commands, map[string]any{
			"cmd":                  "track.send.connect",
			"source_track_id":      source.TrackID,
			"auto_create_fx_track": req.FXTrackName,
		})
	}
	decisions := policy.Analyze(commands)
	preview, err := s.confirmationPreview(ctx, decisions, requestContext)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error(), Commands: decisions}
	}
	topology := sendTopologyPlanSummary(req)
	plan := PendingPlan{
		ID:        "plan_" + randomID(),
		CreatedAt: time.Now(),
		Decisions: decisions,
		// 确认执行时的写闸豁免凭据：本计划自己的源轨集合（qualifiedSendTopologyConnect）。
		Context:   harness.AuthorizeSendTopologyConnect(requestContext, sendSourceTrackIDs(req.Sources)),
		Preview:   preview,
		Workflow:  sendTopologyConnectWorkflow,
		WorkflowData: map[string]any{
			"fx_track_name":    req.FXTrackName,
			"source_track_ids": sendSourceTrackIDs(req.Sources),
			"source_names":     sendSourceTrackNames(req.Sources),
			"share_signal":     req.ShareSignal,
			"user_message":     userText,
		},
	}
	s.mu.Lock()
	s.pending[plan.ID] = plan
	s.mu.Unlock()

	reply := fmt.Sprintf("计划用 send/return 结构共享空间效果：新建 FX 轨「%s」，%s 各自发送进入，湿声回流各自原轨（轨内干湿混合）。确认后我会建边并读回拓扑。", req.FXTrackName, strings.Join(sendSourceTrackNames(req.Sources), "、"))
	return ChatResponse{
		ConversationID:    conversationID,
		Reply:             reply + "\n\n" + topology + "\n\n" + confirmationReply(decisions),
		NeedsConfirmation: true,
		PlanID:            plan.ID,
		Preview:           preview,
		Workflow:          sendTopologyConnectWorkflow,
		WorkflowData:      copyStringAnyMap(plan.WorkflowData),
		Commands:          decisions,
	}
}

// finishSendTopologyConnectWorkflow 确认执行后的收尾：从 connect 回执内嵌的
// 拓扑快照渲染话术模板（回路+发送量列表+机架视窗手动接回提示）。
func (s *Server) finishSendTopologyConnectWorkflow(ctx context.Context, plan PendingPlan, replies []map[string]any, fallbackMessage string) (string, []map[string]any) {
	fxName := firstNonEmptyText(plan.WorkflowData, "fx_track_name")
	fxID, snapshot := sendTopologySnapshotFromReplies(replies)
	if len(snapshot) == 0 {
		return strings.TrimSpace(fallbackMessage + "\n发送边已执行，但内核回执里没有拓扑快照，无法呈现回路；可用 track.send.list 读回核对。"), replies
	}
	if fxName == "" {
		fxName = firstNonEmptyText(snapshot, "fx_name")
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("send/return 结构已建立：回路 = 源轨 --发送--> FX 轨「%s」--湿声回流--> 各源轨（轨内干湿混合，FX 轨主输出压低只走回流）。", fxName))
	if fxID != "" {
		b.WriteString(fmt.Sprintf("（fx_track_id=%s）", fxID))
	}
	b.WriteString("\n发送量列表：\n")
	for _, row := range mapRowsFromAny(snapshot["sends"]) {
		b.WriteString(fmt.Sprintf("- %s → %s：%s dB\n", firstNonEmptyText(row, "source_name", "source_track_id"), firstNonEmptyText(row, "fx_name", "fx_track_id"), trimFloatText(firstNonEmptyText(row, "amount_db"))))
	}
	if dangling := mapRowsFromAny(snapshot["dangling"]); len(dangling) > 0 {
		b.WriteString(fmt.Sprintf("另有 %d 条悬空发送边（目标 FX 轨缺失），已如实标注，可用 track.send.disconnect 清理。\n", len(dangling)))
	}
	b.WriteString("可在机架视窗的发送列表里手动接回/断开/调量（每源一条，量在源轨侧主控）；给 FX 轨装载空间效果器走常规装载（例如说「给 " + fxName + " 加载混响」）。")
	return b.String(), replies
}

// sendTopologySnapshotFromReplies 取最后一条 connect 回执内嵌的拓扑快照
// （appendTopologySnapshot 的 sends/dangling/fx_tracks）与 fx_track_id。
func sendTopologySnapshotFromReplies(replies []map[string]any) (string, map[string]any) {
	for i := len(replies) - 1; i >= 0; i-- {
		result := mapValue(replies[i]["result"])
		if len(result) == 0 {
			continue
		}
		if firstNonEmptyText(result, "fx_track_id") == "" && len(mapRowsFromAny(result["sends"])) == 0 {
			continue
		}
		return firstNonEmptyText(result, "fx_track_id"), result
	}
	return "", nil
}

// sendTopologyPlanSummary 渲染计划面拓扑 diff：新建 FX 轨名、各源发送量缺省、
// 回路示意（设计件 §2.2 pendingplan 呈现要求）。
func sendTopologyPlanSummary(req sendTopologyConnectRequest) string {
	var b strings.Builder
	b.WriteString("拓扑预览（send/return）：\n")
	b.WriteString(fmt.Sprintf("  新建 FX 轨：%s（可改名）\n", req.FXTrackName))
	for _, source := range req.Sources {
		b.WriteString(fmt.Sprintf("  %s --发送(-6.0 dB 缺省)--> %s --湿声回流--> %s\n", source.TrackName, req.FXTrackName, source.TrackName))
	}
	b.WriteString("  各源轨插 AuxSend(干信号分叉)与 AuxReturn(湿声汇入)；FX 轨音量压低，湿声只经回流。")
	return b.String()
}

func sendSourceTrackIDs(sources []sendSourceTrack) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		out = append(out, source.TrackID)
	}
	return out
}

func sendSourceTrackNames(sources []sendSourceTrack) []string {
	out := make([]string, 0, len(sources))
	for _, source := range sources {
		out = append(out, source.TrackName)
	}
	return out
}

func trimFloatText(value string) string {
	if value == "" {
		return "?"
	}
	return value
}

// pendingPlanIsConfirmedSendTopology 报告计划是否来自 send 结构工作流——
// 确认面宽泛混音拦截的豁免依据（计划创建时已要求 ≥2 具名源轨+共享信号）。
func pendingPlanIsConfirmedSendTopology(plan PendingPlan) bool {
	return strings.TrimSpace(plan.Workflow) == sendTopologyConnectWorkflow
}

// sendTopologyAgentSuggestionNote 渲染 agent 主动建议（可拒）话术——决策表
// agent_suggest 分支的呈现面：不改本单轨 insert 计划，只附建议。
func sendTopologyAgentSuggestionNote(state map[string]any, targetTrackID string) string {
	exclude := map[string]bool{targetTrackID: true}
	peers := countSpatialFamilyPeers(state, exclude)
	if peers < 2 {
		return ""
	}
	return fmt.Sprintf("\n\n另外：工程里还有 %d 条轨挂着空间类效果器（词法识别）。如果想让它们共享同一个空间，可以说「给这些轨共用一个混响」，我会改用 send/return 结构（共享 FX 轨+各自发送量）；不需要就忽略这条建议。", peers)
}

// chatCommandsContainSendRoute 报告命令集里是否已有 send 命令（模型信封
// 已给出 send 命令时不抢占既有路径）。
func chatCommandsContainSendRoute(commands []map[string]any) bool {
	for _, command := range commands {
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(command["cmd"]), cleanContextText(command["command"]), cleanContextText(command["action"]), cleanContextText(command["tool"]))))
		if strings.HasPrefix(name, "track.send.") || strings.HasPrefix(name, "track_send_") {
			return true
		}
	}
	return false
}

// sendSourceIDsFromPlanWorkflowData 读回计划 WorkflowData 里的源轨 ID 集
// （持久化往返后是 []any，这里两种形态都收）。
func sendSourceIDsFromPlanWorkflowData(data map[string]any) []string {
	raw := data["source_track_ids"]
	out := []string{}
	switch rows := raw.(type) {
	case []string:
		out = append(out, rows...)
	case []any:
		for _, row := range rows {
			if text := strings.TrimSpace(fmt.Sprint(row)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	}
	return out
}
