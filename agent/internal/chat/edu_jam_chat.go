package chat

// EDU-LEVEL-RUNTIME-B（乐局 3b）：乐局运行时的 chat 侧确定性挂载面。
//
// 形态 i（拦截面内闭环，257 勘察点 1.3 方案）：本文件与 handleMixWorkflowQueueChat
// 并排挂在语义入口分类之前——乐局的入局/状态查询/节点推进检查/终局确认/出局是
// 确定性 agent 面，不依赖分类器心情；推进检查（「写好了」）在拦截面内完成
// 读 clip→判定器→回话的闭环，零侵入主循环。
//
// 红线（零路由改动）：乐局状态=叠加引导框架层（裁定 §五-16），不进
// orchestrationcontroller 枚举、不进 capabilityRoutes/restore 白名单、不占
// controllerOwners——任意 controller 之下的普通轮次照常走既有路由，本面只在自己
// 的确定性命令词形上应答。非命令轮次仅做拉回计数（目标 2），不偷走会话轮。
//
// 持久化（目标 4）：会话态 storeEduJamSession + persistCurrentProjectWorkspace
// （freeStateLoop 同型先例）；restore 逐会话 fail-closed（未知 schema/乐局卡不可
// 加载/jam state restore 拒绝 → 丢弃计数 + Warn 留痕，never execute）。
//
// 终局（目标 3/5）：闭合判定（全节点 done + no_placeholder_blanks + 整曲可播）
// 通过后以语义模板 closure_inquiry 问询（人格渲染层 v0 占位=原始模板文本，渲染
// 边界见回执）；用户确认后置终态、投影一条 trajectory EventSettled（零新增协议）
// 并携带沉淀钩子位（memory_writes/objectives_check/trajectory_archive，rulings
// §五-18——v0 只接线不代写记忆）。
//
// 判定器输入全部来自内核真值（get_project_state / get_midi_clip_notes，经
// edu.ParseClipNotes 与 276 命令面同口径），确定性零 LLM。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/edu"
	"vit-daw-agent/internal/trajectory"
)

const (
	// eduJamSessionSchemaVersion 会话态持久化版本（restore fail-closed 依据）。
	eduJamSessionSchemaVersion = "vit.edu_jam_session.v0"
	eduJamWorkflowName         = "edu_jam"
	// 会话状态闭集（terminal = completed | exited）。
	eduJamStatusActive          = "active"
	eduJamStatusAwaitingClosure = "awaiting_closure"
	eduJamStatusCompleted       = "completed"
	eduJamStatusExited          = "exited"
	// eduJamDefaultLevelID 当前唯一已装载乐局卡（279 定稿实例）。
	eduJamDefaultLevelID = "first-run-twinkle"
	// eduJamPullbackContextKey 拉回注入的上下文键（非命令轮次不拦截，仅注入）。
	eduJamPullbackContextKey = "edu_jam_pullback"
)

// eduJamSession 是一个会话的乐局运行时状态（会话级持久纯数据，freeStateLoop
// 同型；Jam 字段=280 数据层权威，其余为挂载面自己的引导/终局/拉回状态）。
type eduJamSession struct {
	SchemaVersion     string              `json:"schema_version"`
	LevelID           string              `json:"level_id"`
	ConversationID    string              `json:"conversation_id"`
	ProjectUUID       string              `json:"project_uuid"`
	Jam               edu.JamState        `json:"jam"`
	Status            string              `json:"status"`
	TurnsSinceAdvance int                 `json:"turns_since_advance"`
	EventLog          []edu.ObservedEvent `json:"event_log"`
	// EventsConsumed 是已被推进检查消费的事件窗口锚点（EventLog 下标）。
	EventsConsumed int `json:"events_consumed"`
	EnteredAt      string `json:"entered_at,omitempty"`
	EndedAt        string `json:"ended_at,omitempty"`
	EndReason      string `json:"end_reason,omitempty"`
}

func eduJamSessionTerminal(session eduJamSession) bool {
	return session.Status == eduJamStatusCompleted || session.Status == eduJamStatusExited
}

func cloneEduJamSession(session eduJamSession) eduJamSession {
	out := session
	out.EventLog = append([]edu.ObservedEvent(nil), session.EventLog...)
	out.Jam.Nodes = append([]edu.JamNodeState(nil), session.Jam.Nodes...)
	out.Jam.Events = append([]edu.JamEvent(nil), session.Jam.Events...)
	return out
}

// ---- 命令词形（确定性闭集；非命令消息一律放行给既有路由）----

var (
	eduJamEnterPattern = regexp.MustCompile(`^(?:开始乐局|进入乐局|开局乐局|edu_jam\s+enter)[\s:：]*(.*)$`)
	eduJamExitPattern  = regexp.MustCompile(`^(?:结束乐局|退出乐局|乐局结束|edu_jam\s+exit)$`)
	// eduJamCheckWords 推进检查词形（学生说「写好了」）：整句匹配（剥尾部标点），
	// 避免吞掉含「写好了」的长句（那是普通会话轮，归语义入口）。
	eduJamCheckWords = map[string]bool{
		"写好了": true, "写完了": true, "写完啦": true, "我写好了": true, "我写完了": true,
		"检查一下": true, "检查下": true, "判定": true, "判定一下": true, "乐局判定": true,
		"edu_jam check": true,
	}
	eduJamStatusWords   = []string{"现在第几关", "乐局状态", "乐局进度", "乐局进行到哪了", "第几关了", "edu_jam status"}
	eduJamConfirmWords  = map[string]bool{"确认收尾": true, "确认结算": true, "确认完成": true, "确认导出": true, "edu_jam confirm": true}
	eduJamDirectExport  = map[string]bool{"直接导出": true, "导出吧": true, "edu_jam export": true}
	eduJamEventPattern = regexp.MustCompile(`^(?:乐局事件|edu_jam\s+event)\s+([a-z_]+)(?:[\s:：]+([A-Za-z0-9_\-]+))?$`)
	eduJamLevelIDRegex = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)
	eduJamLevelAliases = map[string]string{"小星星": eduJamDefaultLevelID, "twinkle": eduJamDefaultLevelID, "《小星星》": eduJamDefaultLevelID}
)

// parseEduJamCommand 识别乐局面命令词形；返回面名与参数。
func parseEduJamCommand(text string) (face string, levelHint, eventName, eventKey string, direct bool, ok bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", "", "", "", false, false
	}
	if m := eduJamEnterPattern.FindStringSubmatch(trimmed); m != nil {
		hint := strings.TrimSpace(m[1])
		// 保守边界：enter 只吃「干净」的命令。尾部 hint 是合法 id 语法或已知名
		// 别名时才作为乐局命令（错误 id → fail-closed 报错）；否则视为普通会话
		// 轮放行（「开始乐局吧」这类自然语不入局，不吞会话轮）。
		if hint == "" || eduJamLevelIDRegex.MatchString(hint) {
			return "enter", hint, "", "", false, true
		}
		if _, aliased := eduJamLevelAliases[strings.ToLower(hint)]; aliased {
			return "enter", hint, "", "", false, true
		}
	}
	if m := eduJamEventPattern.FindStringSubmatch(trimmed); m != nil {
		return "event", "", strings.ToLower(m[1]), m[2], false, true
	}
	lower := strings.ToLower(trimmed)
	if eduJamCheckWords[trimmed] || eduJamCheckWords[lower] {
		return "check", "", "", "", false, true
	}
	for _, w := range eduJamStatusWords {
		if lower == w {
			return "status", "", "", "", false, true
		}
	}
	if eduJamConfirmWords[trimmed] || eduJamConfirmWords[lower] {
		return "confirm", "", "", "", false, true
	}
	if eduJamDirectExport[trimmed] || eduJamDirectExport[lower] {
		return "confirm", "", "", "", true, true
	}
	if eduJamExitPattern.MatchString(trimmed) || eduJamExitPattern.MatchString(lower) {
		return "exit", "", "", "", false, true
	}
	return "", "", "", "", false, false
}

// handleEduJamChat 是乐局确定性 chat 面（goalrunner_chat.go 插点，与 mix
// workflow queue 并排，同 durableInvocationID=="" 门）。非命令轮次在有活跃
// 乐局时只做拉回计数与目标注入，返回 not-handled——乐局永远不偷普通会话轮。
func (s *Server) handleEduJamChat(ctx context.Context, conversationID, mode, text string, chatContext map[string]any) (ChatResponse, bool) {
	trimmed := strings.TrimSpace(text)
	if face, levelHint, eventName, eventKey, direct, ok := parseEduJamCommand(trimmed); ok {
		return s.serveEduJamCommand(ctx, conversationID, mode, face, levelHint, eventName, eventKey, direct)
	}
	session, card, ok := s.eduJamActiveSessionCard(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	session.TurnsSinceAdvance++
	if goal, due := eduJamPullbackGoal(session, card); due {
		if chatContext != nil {
			chatContext[eduJamPullbackContextKey] = map[string]any{
				"schema_version":        eduJamSessionSchemaVersion,
				"level_id":              card.ID,
				"node_id":               eduJamCurrentNodeID(session, card),
				"goal":                  goal,
				"pullback_after_turns":  card.Guidance.PullbackAfterTurns,
				"injected_at":           time.Now().UTC().Format(time.RFC3339),
				"render_note":           "人格渲染层占位：注入原始目标文本（EDU-LEVEL-RUNTIME-B v0）",
			}
		}
		session.TurnsSinceAdvance = 0
		s.storeEduJamSession(session)
		s.persistCurrentProjectWorkspace() // 拉回触发点落盘（计数为咨询态，仅阈值处持久化）
		return ChatResponse{}, false
	}
	s.storeEduJamSession(session)
	return ChatResponse{}, false
}

func (s *Server) serveEduJamCommand(ctx context.Context, conversationID, mode, face, levelHint, eventName, eventKey string, direct bool) (ChatResponse, bool) {
	switch face {
	case "enter":
		return s.eduJamEnter(ctx, conversationID, mode, levelHint)
	case "status":
		return s.eduJamStatusReply(conversationID, mode)
	case "check":
		return s.eduJamAdvance(ctx, conversationID, mode)
	case "event":
		return s.eduJamRecordEvent(conversationID, mode, eventName, eventKey)
	case "confirm":
		return s.eduJamConfirmClosure(ctx, conversationID, mode, direct)
	case "exit":
		return s.eduJamExit(ctx, conversationID, mode)
	}
	return ChatResponse{}, false
}

func (s *Server) eduJamBaseResponse(conversationID, mode string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID,
		AgentMode:      firstNonEmpty(strings.TrimSpace(mode), agentModeDefault),
		Workflow:       eduJamWorkflowName,
		GoalStatus:     "completed",
	}
}

// ---- enter（开局，幂等拒绝重复 enter）----

func (s *Server) eduJamEnter(ctx context.Context, conversationID, mode, levelHint string) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	if session, ok := s.eduJamSessionFor(conversationID); ok && !eduJamSessionTerminal(session) {
		base.Reply = "乐局已在进行中（幂等拒绝重复开局）。当前：\n" + s.eduJamRenderStatus(session)
		base.StopReason = "edu_jam_enter_idempotent_rejected"
		base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"rejected": "active_jam_exists"})
		return base, true
	}
	levelID := eduJamResolveLevelID(levelHint)
	card, err := loadEduJamLevelCard(levelID)
	if err != nil {
		// 产品式文案（LEGAME-DIR-1 / 设计稿 §2.1-5）：三级目录都解析不出可用卡时
		// 不向用户暴露 env 名/内部路径；机器可读细节走 base.Error 与日志（布位腿
		// 排查面），失败语义维持 fail-closed。
		if s.logger != nil {
			s.logger.Warn("[edu-jam] level card unavailable (fail-closed, levels_dir=%s): %v", eduJamLevelsDir(), err)
		}
		base.Reply = "乐局内容未安装或暂不可用——稍等片刻重试，或重新安装乐局内容后再说「开始乐局」。"
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_level_card_unavailable"
		base.Error = err.Error()
		return base, true
	}
	// 281 修复腿（主管裁定 B 形态）：enter 在落盘前确保工作区激活完成——
	// pre-activation 开局正是 smokerun1_20261107_0924 exit 4 竞态的入口
	// （enter 落盘 → 同窗口惰性激活空态整表覆盖 eduJamSessions → 推进轮
	// 查无会话放行）。无法确保激活 → fail-closed 拒绝开局并明确回话。
	projectUUID, gateReason := s.eduJamEnsureWorkspaceActivation(ctx)
	if gateReason != "" {
		base.Reply = "工程还没打开好，稍等再开始乐局（" + eduJamGateReasonText(gateReason) + "）。"
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_workspace_activation_incomplete"
		base.Error = "workspace activation incomplete: " + gateReason
		base.WorkflowData = map[string]any{
			"schema_version": eduJamSessionSchemaVersion,
			"gate":           "workspace_activation",
			"gate_reason":    gateReason,
		}
		return base, true
	}
	nodeIDs := make([]string, 0, len(card.Nodes))
	for _, n := range card.Nodes {
		nodeIDs = append(nodeIDs, n.ID)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	jam, err := edu.NewJamState(card.ID, conversationID, projectUUID, nodeIDs, now)
	if err != nil {
		base.Reply = "乐局状态创建失败（fail-closed）：" + err.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_state_rejected"
		base.Error = err.Error()
		return base, true
	}
	session := eduJamSession{
		SchemaVersion: eduJamSessionSchemaVersion,
		LevelID:       card.ID,
		ConversationID: conversationID,
		ProjectUUID:   projectUUID,
		Jam:           jam,
		Status:        eduJamStatusActive,
		EventLog:      []edu.ObservedEvent{},
		EnteredAt:     now,
	}
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	base.Reply = eduJamRenderEnter(card)
	base.StopReason = "edu_jam_entered"
	base.WorkflowData = s.eduJamWorkflowData(session, nil)
	return base, true
}

func eduJamRenderEnter(card *edu.LevelCard) string {
	story := strings.TrimSpace(card.Story)
	story = strings.ReplaceAll(story, "{model_name}", "Vit") // 人格渲染层占位（rulings §五-19 v1 终审前）
	var b strings.Builder
	b.WriteString(fmt.Sprintf("乐局开始：%s\n", card.Title))
	if story != "" {
		b.WriteString(story + "\n\n")
	}
	if len(card.Nodes) > 0 {
		n := card.Nodes[0]
		b.WriteString(fmt.Sprintf("第 1 关（%d 关）·%s\n目标：%s", len(card.Nodes), n.Title, n.Goal))
		if len(n.HintLadder) > 0 {
			b.WriteString("\n（卡在哪可以说「乐局状态」看进度；写好了就说「写好了」）")
		}
	}
	return b.String()
}

// ---- 状态查询 ----

func (s *Server) eduJamStatusReply(conversationID, mode string) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok {
		base.Reply = "当前没有进行中的乐局。可以说「开始乐局」开局。"
		base.StopReason = "edu_jam_no_session"
		base.WorkflowData = map[string]any{"schema_version": eduJamSessionSchemaVersion, "status": "none"}
		return base, true
	}
	base.Reply = s.eduJamRenderStatus(session)
	base.StopReason = "edu_jam_status"
	base.WorkflowData = s.eduJamWorkflowData(session, nil)
	return base, true
}

func (s *Server) eduJamRenderStatus(session eduJamSession) string {
	card, err := loadEduJamLevelCard(session.LevelID)
	if err != nil {
		return fmt.Sprintf("乐局 %s（状态 %s）：乐局卡当前不可加载，状态以事件表为准（节点 %d 个，事件 %d 条）。",
			session.LevelID, session.Status, len(session.Jam.Nodes), len(session.Jam.Events))
	}
	done := 0
	var b strings.Builder
	b.WriteString(fmt.Sprintf("乐局《%s》（%s）· 状态：%s\n", card.Title, card.ID, eduJamStatusLabel(session.Status)))
	for i, n := range card.Nodes {
		nodeState := eduJamNodeStateOf(session, n.ID)
		mark := "…"
		if nodeState == edu.JamNodeDone {
			mark = "✓"
			done++
		} else if nodeState == edu.JamNodeActive {
			mark = "▶"
		}
		b.WriteString(fmt.Sprintf("%s 第 %d 关 %s（%s）\n", mark, i+1, n.Title, n.ID))
		if nodeState == edu.JamNodeActive {
			b.WriteString("　目标：" + n.Goal + "\n")
		}
	}
	if session.Status == eduJamStatusActive {
		b.WriteString(fmt.Sprintf("（已完成 %d/%d；距拉回提醒还有 %d 轮）", done, len(card.Nodes),
			eduJamPullbackRemaining(session, card)))
	}
	return b.String()
}

func eduJamStatusLabel(status string) string {
	switch status {
	case eduJamStatusActive:
		return "进行中"
	case eduJamStatusAwaitingClosure:
		return "等待终局确认"
	case eduJamStatusCompleted:
		return "已完成（终态）"
	case eduJamStatusExited:
		return "已退出（终态）"
	}
	return status
}

func eduJamNodeStateOf(session eduJamSession, nodeID string) string {
	for _, n := range session.Jam.Nodes {
		if n.NodeID == nodeID {
			return n.State
		}
	}
	return edu.JamNodePending
}

func eduJamCurrentNodeID(session eduJamSession, card *edu.LevelCard) string {
	if idx, _, ok := session.Jam.CurrentJamNode(card); ok {
		return card.Nodes[idx].ID
	}
	return ""
}

// ---- 事件记录（driver/模型面显式事件；学生 NL 识别归语义层，v0 不做）----

func (s *Server) eduJamRecordEvent(conversationID, mode, eventName, eventKey string) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok || eduJamSessionTerminal(session) {
		base.Reply = "没有进行中的乐局，事件未记录。"
		base.StopReason = "edu_jam_no_session"
		return base, true
	}
	known := false
	for _, name := range edu.LevelEventNames {
		if name == eventName {
			known = true
			break
		}
	}
	if !known {
		base.Reply = fmt.Sprintf("未知乐局事件 %q（闭集：%v）——fail-closed 不记录。", eventName, edu.LevelEventNames)
		base.StopReason = "edu_jam_unknown_event"
		base.Error = "unknown event name"
		return base, true
	}
	session.EventLog = append(session.EventLog, edu.ObservedEvent{Name: eventName, Key: eventKey})
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	base.Reply = fmt.Sprintf("已记录乐局事件 %s（key=%q）。", eventName, eventKey)
	base.StopReason = "edu_jam_event_recorded"
	base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"event": eventName, "key": eventKey})
	return base, true
}

// ---- 推进检查（形态 i 闭环：读 clip → 判定 → 回话）----

func (s *Server) eduJamAdvance(ctx context.Context, conversationID, mode string) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok {
		return ChatResponse{}, false // 没有乐局时「写好了」是普通会话轮，放行
	}
	if eduJamSessionTerminal(session) {
		base.Reply = "乐局已" + eduJamStatusLabel(session.Status) + "，不再推进。可以说「开始乐局」开新局。"
		base.StopReason = "edu_jam_terminal"
		base.WorkflowData = s.eduJamWorkflowData(session, nil)
		return base, true
	}
	card, err := loadEduJamLevelCard(session.LevelID)
	if err != nil {
		base.Reply = "乐局卡当前不可加载（fail-closed，不盲判）：" + err.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_level_card_unavailable"
		base.Error = err.Error()
		return base, true
	}
	// 终局待确认态：推进面转为重跑闭合判定（结构可能已补齐）。
	if session.Status == eduJamStatusAwaitingClosure {
		return s.eduJamClosurePass(ctx, base, session, card)
	}
	// 观察组装（全部来自内核真值；读不到真值即不判定）。
	obs, _, obsErr := s.observeEduJam(ctx, card)
	if obsErr != nil {
		base.Reply = "判定需要内核真值，本次读取失败（fail-closed，不盲判）：" + obsErr.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_observation_unavailable"
		base.Error = obsErr.Error()
		return base, true
	}
	// 检查消息本身是一次 human_replied 观察。
	session.EventLog = append(session.EventLog, edu.ObservedEvent{Name: "human_replied"})
	window := append([]edu.ObservedEvent(nil), session.EventLog[minInt(session.EventsConsumed, len(session.EventLog)):]...)
	now := time.Now().UTC().Format(time.RFC3339)
	obs.At = now
	obs.Events = window

	nodeID := eduJamCurrentNodeID(session, card)
	eval, evalErr := edu.EvaluateNodeConditions(card, nodeID, obs)
	if evalErr != nil {
		base.Reply = "节点条件评估失败（卡引用断裂，fail-closed 上交）：" + evalErr.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_eval_error"
		base.Error = evalErr.Error()
		return base, true
	}
	jamNext, advErr := edu.AdvanceJam(session.Jam, card, obs)
	if advErr != nil && advErr != edu.ErrJamCompleted {
		base.Reply = "乐局推进失败（fail-closed）：" + advErr.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_advance_error"
		base.Error = advErr.Error()
		return base, true
	}
	session.Jam = jamNext
	if advErr == edu.ErrJamCompleted {
		// 防御分支：active 态但无当前节点且全节点 done（闭合未确认的重入）→ 走闭合判定。
		s.storeEduJamSession(session)
		return s.eduJamClosurePass(ctx, base, session, card)
	}

	if eval.Satisfied {
		session.TurnsSinceAdvance = 0
		session.EventsConsumed = len(session.EventLog)
		s.storeEduJamSession(session)
		if jamNext.AllNodesDone(card) {
			return s.eduJamClosurePass(ctx, base, session, card)
		}
		s.persistCurrentProjectWorkspace()
		nextIdx, _, hasNext := jamNext.CurrentJamNode(card)
		reply := fmt.Sprintf("第 %s 关通过 ✓\n", nodeID)
		if hasNext {
			n := card.Nodes[nextIdx]
			reply += fmt.Sprintf("进入第 %d 关 ·%s\n目标：%s", nextIdx+1, n.Title, n.Goal)
			if len(n.HintLadder) > 0 {
				reply += "\n（提示阶梯已就位：先自己试，卡住再说「乐局状态」）"
			}
		}
		base.Reply = reply
		base.StopReason = "edu_jam_node_done"
		base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"node_done": nodeID, "leaves": eduJamLeafSummaries(eval.Leaves)})
		return base, true
	}

	// 未通过：有判定尝试 → attempt+1（AdvanceJam 内已记事件），窗口消费；反馈带缺口。
	if eval.JudgedAttempts > 0 {
		session.EventsConsumed = len(session.EventLog)
	}
	session.TurnsSinceAdvance++
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	attempt := eduJamNodeAttempt(session, nodeID)
	reply := fmt.Sprintf("第 %s 关还没过（第 %d 次尝试）：\n%s", nodeID, attempt, eduJamRenderLeaves(eval.Leaves))
	if hint := eduJamHintRung(card, nodeID, attempt); hint != "" {
		reply += "\n提示：" + hint
	}
	base.Reply = reply
	base.StopReason = "edu_jam_attempt_failed"
	base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"node": nodeID, "attempt": attempt, "leaves": eduJamLeafSummaries(eval.Leaves)})
	return base, true
}

func eduJamNodeAttempt(session eduJamSession, nodeID string) int {
	for _, n := range session.Jam.Nodes {
		if n.NodeID == nodeID {
			return n.Attempt
		}
	}
	return 0
}

func eduJamRenderLeaves(leaves []edu.LeafEvaluation) string {
	rows := make([]string, 0, len(leaves))
	for _, l := range leaves {
		mark := "✗"
		if l.Satisfied {
			mark = "✓"
		}
		row := fmt.Sprintf("%s %s:%s", mark, l.Kind, l.Name)
		if !l.Satisfied && l.Reason != "" {
			row += " — " + l.Reason
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func eduJamLeafSummaries(leaves []edu.LeafEvaluation) []string {
	out := make([]string, 0, len(leaves))
	for _, l := range leaves {
		out = append(out, fmt.Sprintf("%s:%s=%v", l.Kind, l.Name, l.Satisfied))
	}
	return out
}

// eduJamHintRung 取提示阶梯第 attempt 级（0 基；越界收敛到末级；无阶梯=空）。
func eduJamHintRung(card *edu.LevelCard, nodeID string, attempt int) string {
	for _, n := range card.Nodes {
		if n.ID != nodeID {
			continue
		}
		if len(n.HintLadder) == 0 {
			return ""
		}
		idx := attempt - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(n.HintLadder) {
			idx = len(n.HintLadder) - 1
		}
		return n.HintLadder[idx]
	}
	return ""
}

// ---- 闭合判定与终局（目标 3/5）----

// eduJamClosurePass 重跑结构闭合判定：通过 → awaiting_closure + 问询；未通过 →
// 保持 active 并如实给出缺口（fail-honest，不静默放宽）。
func (s *Server) eduJamClosurePass(ctx context.Context, base ChatResponse, session eduJamSession, card *edu.LevelCard) (ChatResponse, bool) {
	obs, projectState, obsErr := s.observeEduJam(ctx, card)
	if obsErr != nil {
		base.Reply = "闭合判定需要内核真值，本次读取失败（fail-closed）：" + obsErr.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_observation_unavailable"
		base.Error = obsErr.Error()
		return base, true
	}
	pass, gaps := eduJamClosureCheck(session, card, obs, projectState)
	if !pass {
		session.Status = eduJamStatusActive
		s.storeEduJamSession(session)
		s.persistCurrentProjectWorkspace()
		base.Reply = "全节点已过，但结构闭合还没达成：\n- " + strings.Join(gaps, "\n- ") + "\n补齐后再说「写好了」重跑闭合判定。"
		base.StopReason = "edu_jam_closure_blocked"
		base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"closure_gaps": gaps})
		return base, true
	}
	session.Status = eduJamStatusAwaitingClosure
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	// 问询（语义模板 closure_inquiry；人格渲染层 v0 占位=模板名+证据直出）。
	base.Reply = fmt.Sprintf(
		"【closure_inquiry】%s 的全部关卡都过了，整曲可以从头播到尾，伴奏轨也已填上。\n"+
			"要收下这一局吗？确认后我们一起命名并导出（说「确认收尾」，或直接说「直接导出」）。",
		card.Title)
	base.StopReason = "edu_jam_closure_inquiry"
	base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{
		"semantic_template": card.Finale.Ask.SemanticTemplate,
		"closure":           "structure_complete",
	})
	return base, true
}

// eduJamClosureCheck 结构闭合三件（finale.closure_check.structure_complete）：
// all_nodes_done + no_placeholder_blanks（轨槽非空）+ whole_song_playable
// （from_beats=0，v0 判据=工程存在可播放内容即最大 clip 末端>0；work_surface_quiet
// 与 soft_signals 是模型/人格层判断，v0 不在确定性面伪造——边界已在卡面声明）。
func eduJamClosureCheck(session eduJamSession, card *edu.LevelCard, obs edu.Observation, projectState map[string]any) (bool, []string) {
	gaps := []string{}
	sc := card.Finale.ClosureCheck.StructureComplete
	if sc.AllNodesDone && !session.Jam.AllNodesDone(card) {
		gaps = append(gaps, "尚有节点未完成（all_nodes_done）")
	}
	for _, slot := range sc.NoPlaceholderBlanks {
		if v, seen := obs.TrackNonEmpty[slot]; !seen || !v {
			gaps = append(gaps, fmt.Sprintf("轨槽 %s 仍为空（no_placeholder_blanks）", slot))
		}
	}
	if from, ok := sc.WholeSongPlayable["from_beats"]; ok && from == 0 {
		if eduJamProjectEndSeconds(projectState) <= 0 {
			gaps = append(gaps, "工程尚无可从头播放的内容（whole_song_playable）")
		}
	}
	return len(gaps) == 0, gaps
}

// eduJamProjectEndSeconds 取工程内全部 clip 的最大末端秒数（get_project_state
// tracks[].clips[]；无 clip=0）。
func eduJamProjectEndSeconds(projectState map[string]any) float64 {
	end := 0.0
	for _, track := range mapRowsFromAny(projectState["tracks"]) {
		for _, clip := range mapRowsFromAny(track["clips"]) {
			if e := eduJamFloatOf(clip["end_seconds"]); e > end {
				end = e
			}
		}
	}
	return end
}

func eduJamFloatOf(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// eduJamConfirmClosure 终局确认（或用户直接导出=skip_ask 语义）→ 终态 + trajectory
// 终局面 + 沉淀钩子位。
func (s *Server) eduJamConfirmClosure(ctx context.Context, conversationID, mode string, direct bool) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok {
		return ChatResponse{}, false // 没有乐局时是普通会话轮，放行
	}
	if eduJamSessionTerminal(session) {
		base.Reply = "乐局已" + eduJamStatusLabel(session.Status) + "。"
		base.StopReason = "edu_jam_terminal"
		base.WorkflowData = s.eduJamWorkflowData(session, nil)
		return base, true
	}
	card, err := loadEduJamLevelCard(session.LevelID)
	if err != nil {
		base.Reply = "乐局卡当前不可加载（fail-closed）：" + err.Error()
		base.GoalStatus = "failed"
		base.StopReason = "edu_jam_level_card_unavailable"
		base.Error = err.Error()
		return base, true
	}
	if session.Status != eduJamStatusAwaitingClosure {
		// 直接导出（skip_ask）只对全节点已过的局生效；否则如实回绝。
		if !(direct && session.Jam.AllNodesDone(card)) {
			base.Reply = "还不能收尾：乐局仍在进行中（" + eduJamStatusLabel(session.Status) + "），先把关卡过完。"
			base.StopReason = "edu_jam_closure_preconditions"
			base.WorkflowData = s.eduJamWorkflowData(session, nil)
			return base, true
		}
	}
	endReason := "closure_confirmed"
	if direct {
		endReason = "user_direct_export"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	session.Status = eduJamStatusCompleted
	session.EndedAt = now
	session.EndReason = endReason
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	s.emitEduJamTerminalEvent(conversationID, session, card)
	base.Reply = fmt.Sprintf(
		"乐局完成 ✓《%s》\n终局仪式：①一起给这首曲子起个名字；②导出属于你们的版本（导出完成后说「乐局事件 export_completed」留档）；③沉淀本局学到的（画像与学习目标核对）。\n局后继续修改工程完全正常——完成的是这一局，不是冻结工程（completion ≠ freeze）。",
		card.Title)
	base.StopReason = "edu_jam_completed"
	base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{
		"end_reason":         endReason,
		"memory_writes":      card.Finale.Settle.MemoryWrites,
		"objectives_check":   card.Finale.Settle.ObjectivesCheck,
		"trajectory_archive": card.Finale.Settle.TrajectoryArchive,
	})
	return base, true
}

// eduJamExit 出局（任意非终态可退；终态幂等说明）。
func (s *Server) eduJamExit(ctx context.Context, conversationID, mode string) (ChatResponse, bool) {
	base := s.eduJamBaseResponse(conversationID, mode)
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok {
		return ChatResponse{}, false // 没有乐局时是普通会话轮，放行
	}
	if eduJamSessionTerminal(session) {
		base.Reply = "乐局已" + eduJamStatusLabel(session.Status) + "，无需再退出。"
		base.StopReason = "edu_jam_terminal"
		base.WorkflowData = s.eduJamWorkflowData(session, nil)
		return base, true
	}
	card, cardErr := loadEduJamLevelCard(session.LevelID)
	now := time.Now().UTC().Format(time.RFC3339)
	session.Status = eduJamStatusExited
	session.EndedAt = now
	session.EndReason = "user_exit"
	s.storeEduJamSession(session)
	s.persistCurrentProjectWorkspace()
	if cardErr == nil {
		s.emitEduJamTerminalEvent(conversationID, session, card)
	} else if s.logger != nil {
		s.logger.Warn("[edu-jam] exit terminal event skipped (level card unavailable): %v", cardErr)
	}
	base.Reply = "乐局已退出（状态保留在快照里，想再玩说「开始乐局」开新局）。"
	base.StopReason = "edu_jam_exited"
	base.WorkflowData = s.eduJamWorkflowData(session, map[string]any{"end_reason": "user_exit"})
	return base, true
}

// emitEduJamTerminalEvent 投影一条终局面（trajectory 事件零新增协议：EventSettled
// 既有类型；失败只 Warn 不阻断终态落盘）。
func (s *Server) emitEduJamTerminalEvent(conversationID string, session eduJamSession, card *edu.LevelCard) {
	status := trajectory.StatusCompleted
	if session.Status == eduJamStatusExited {
		status = trajectory.StatusStopped
	}
	done := 0
	for _, n := range session.Jam.Nodes {
		if n.State == edu.JamNodeDone {
			done++
		}
	}
	evt := trajectory.Event{
		Type:            trajectory.EventSettled,
		ConversationID:  conversationID,
		ItemID:          "edu_jam:" + session.LevelID,
		Title:           "乐局终局：" + card.Title,
		Body:            fmt.Sprintf("%s · %s（%s）", card.Title, eduJamStatusLabel(session.Status), session.EndReason),
		Payload: trajectory.Payload{
			TurnID:          "edu_jam:" + session.LevelID,
			Status:          status,
			Terminal:        true,
			TransitionReason: session.EndReason,
			Summary:         fmt.Sprintf("nodes_done=%d/%d events=%d", done, len(card.Nodes), len(session.EventLog)),
			Details: map[string]any{
				"level_id": session.LevelID, "end_reason": session.EndReason,
				"nodes_done": done, "nodes_total": len(card.Nodes),
				"attempts_total": eduJamTotalAttempts(session), "events_observed": len(session.EventLog),
			},
		},
	}
	if _, err := s.emitTrajectoryEvent(conversationID, evt); err != nil && s.logger != nil {
		s.logger.Warn("[edu-jam] terminal trajectory event emit failed: %v", err)
	}
}

func eduJamTotalAttempts(session eduJamSession) int {
	total := 0
	for _, n := range session.Jam.Nodes {
		total += n.Attempt
	}
	return total
}

// ---- 拉回（目标 2：多轮未推进 → 重新注入当前节点目标）----

func eduJamPullbackRemaining(session eduJamSession, card *edu.LevelCard) int {
	r := card.Guidance.PullbackAfterTurns - session.TurnsSinceAdvance
	if r < 0 {
		r = 0
	}
	return r
}

// eduJamPullbackGoal 报告拉回是否到期与要注入的目标文本。
func eduJamPullbackGoal(session eduJamSession, card *edu.LevelCard) (string, bool) {
	if session.Status != eduJamStatusActive || card.Guidance.PullbackAfterTurns < 1 {
		return "", false
	}
	if session.TurnsSinceAdvance < card.Guidance.PullbackAfterTurns {
		return "", false
	}
	idx, _, ok := session.Jam.CurrentJamNode(card)
	if !ok {
		return "", false
	}
	return card.Nodes[idx].Goal, true
}

// ---- 观察组装（内核真值 → edu.Observation）----

func (s *Server) observeEduJam(ctx context.Context, card *edu.LevelCard) (edu.Observation, map[string]any, error) {
	send := s.eduJamKernelSendFunc()
	if send == nil {
		return edu.Observation{}, nil, fmt.Errorf("kernel read face unavailable (no kernel client)")
	}
	stateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	state, err := send(stateCtx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		return edu.Observation{}, nil, fmt.Errorf("get_project_state: %w", err)
	}
	if !eduJamKernelReplyOK(state) {
		return edu.Observation{}, nil, fmt.Errorf("get_project_state not ok: %s", eduJamKernelReplyError(state))
	}
	obs := edu.Observation{
		At:             time.Now().UTC().Format(time.RFC3339),
		Slots:          map[string][]edu.Note{},
		TrackNonEmpty:  map[string]bool{},
	}
	// tempo_set 观察口径：music_context 披露 tempo 已被设置（tempo_setting_count>=1
	// 且 tempo 可用）。注意内核把初始默认 tempo 也计 1（276 烟测实证）——对 n2 这类
	// 事件+结构双叶节点，真正的门是 user_confirmed 事件；本谓词只是诚实观察值。
	mc := mapValue(state["music_context"])
	if eduJamFloatOf(mc["tempo_bpm"]) > 0 && intNumber(mc["tempo_setting_count"]) >= 1 {
		obs.TempoSet = true
	}
	// 真实内核 tracks 按轨树文档序返回，前部固定有 5 条服务轨
	// （Arranger/Chord/Marker/Tempo/Master——smokerun2 实证
	// edu_jam_fixture_project.vit:40-48；Go 侧同形状钉死=shadow_test.go:127-131，
	// 本 run 日志 [shadow] initialized tracks=3 即过滤后计数）。runtime_slots.
	// track_index 是 1 基**用户轨**号（279 卡 §2.3），必须先过滤出用户轨再寻址；
	// 原始数组直接下标会把 melody_imported(track_index=1) 落到 Arranger 轨
	// （无 clips → clip_exists=false）——smokerun2_20261107_1006 失败面。
	tracks := eduJamUserTrackRows(mapRowsFromAny(state["tracks"]))
	for _, slotName := range eduJamSortedSlotNames(card) {
		slot := card.RuntimeSlots[slotName]
		idx := slot.TrackIndex - 1 // 过滤后用户轨数组的 1 基下标
		if idx < 0 || idx >= len(tracks) {
			continue // 轨不存在 → 轨槽未观察（clip_exists=false 语义）
		}
		clips := mapRowsFromAny(tracks[idx]["clips"])
		obs.TrackNonEmpty[slotName] = len(clips) > 0
		clipID := ""
		for _, clip := range clips {
			if strings.Contains(strings.ToLower(firstStringFromMap(clip, "clip_type", "type")), "midi") {
				clipID = firstStringFromMap(clip, "id", "clip_id", "item_id")
				if clipID != "" {
					break
				}
			}
		}
		if clipID == "" {
			continue // 无 MIDI clip → 键缺席=clip_exists=false
		}
		notesCtx, notesCancel := context.WithTimeout(ctx, 10*time.Second)
		reply, err := send(notesCtx, map[string]any{"cmd": "get_midi_clip_notes", "clip_id": clipID})
		notesCancel()
		if err != nil {
			return edu.Observation{}, nil, fmt.Errorf("get_midi_clip_notes(%s/%s): %w", slotName, clipID, err)
		}
		if !eduJamKernelReplyOK(reply) {
			return edu.Observation{}, nil, fmt.Errorf("get_midi_clip_notes(%s/%s) not ok: %s", slotName, clipID, eduJamKernelReplyError(reply))
		}
		notes, err := edu.ParseClipNotes(reply)
		if err != nil {
			return edu.Observation{}, nil, fmt.Errorf("slot %s: %w", slotName, err)
		}
		obs.Slots[slotName] = notes
	}
	return obs, state, nil
}

// eduJamUserTrackRows 从 get_project_state 的原始 tracks 行（真实内核轨树文档
// 序，前部固定 5 条服务轨）过滤出用户轨。判据与 shadow.isUserTrack
// （internal/shadow/shadow.go，包内未导出故在此复制同语义）同源：服务轨按
// track_type/track_name 负面清单剔除；is_audio_track/is_audio 布尔或
// vit_type∈{audio,midi,bus,ghost} 正判。生产消费者同型过滤先例=
// capability_routing.go:530（capacityFactsFromProjectState 音频轨计数）。
// 槽寻址（1 基用户轨号）必须落在过滤后的数组上。
func eduJamUserTrackRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if eduJamIsUserTrackRow(row) {
			out = append(out, row)
		}
	}
	return out
}

func eduJamIsUserTrackRow(row map[string]any) bool {
	kind := strings.ToLower(strings.TrimSpace(firstStringFromMap(row, "track_type", "type")))
	if eduJamServiceTrackKind(kind) {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(firstStringFromMap(row, "track_name", "name")))
	if eduJamServiceTrackKind(name) {
		return false
	}
	if contextBool(row, "is_audio_track") || contextBool(row, "is_audio") {
		return true
	}
	vitType := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["vit_type"])))
	return vitType == "audio" || vitType == "midi" || vitType == "bus" || vitType == "ghost"
}

// eduJamServiceTrackKind 匹配 tracktion 服务轨种类名（shadow.isUserTrack 负面
// 清单同语义）。真实内核五条服务轨里 Master 带 track_type=master，其余四条
// track_type=track 但 track_name 命中清单——type/name 两维任一命中即服务轨
// （shadow_test.go:127-131 钉死的真实形状）。
func eduJamServiceTrackKind(kind string) bool {
	switch kind {
	case "master", "arranger", "chord", "marker", "tempo":
		return true
	}
	return false
}

func eduJamKernelReplyOK(reply map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
	return status == "ok" || status == "success"
}

func eduJamKernelReplyError(reply map[string]any) string {
	return firstNonEmpty(strings.TrimSpace(fmt.Sprint(reply["error"])), strings.TrimSpace(fmt.Sprint(reply["message"])), "unknown error")
}

func eduJamSortedSlotNames(card *edu.LevelCard) []string {
	names := make([]string, 0, len(card.RuntimeSlots))
	for name := range card.RuntimeSlots {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// eduJamKernelSendFunc 返回乐局读面的内核发送函数（测试注入缝=
// eduJamKernelSendOverride；生产=s.kernel.SendCommand 包装）。
func (s *Server) eduJamKernelSendFunc() edu.KernelSendFunc {
	if s != nil && s.eduJamKernelSendOverride != nil {
		return s.eduJamKernelSendOverride
	}
	if s == nil || s.kernel == nil {
		return nil
	}
	return func(ctx context.Context, cmd map[string]any) (map[string]any, error) {
		reply, _, err := s.kernel.SendCommand(ctx, cmd)
		return reply, err
	}
}

// ---- 会话存取（freeStateLoop 同型 storeXxx；s.mu 保护）----

func (s *Server) storeEduJamSession(session eduJamSession) {
	if s == nil || strings.TrimSpace(session.ConversationID) == "" {
		return
	}
	session = cloneEduJamSession(session)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eduJamSessions == nil {
		s.eduJamSessions = map[string]eduJamSession{}
	}
	s.eduJamSessions[session.ConversationID] = session
}

func (s *Server) eduJamSessionFor(conversationID string) (eduJamSession, bool) {
	if s == nil {
		return eduJamSession{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.eduJamSessions[conversationID]
	return cloneEduJamSession(session), ok
}

// eduJamActiveSessionCard 返回活跃（非终态）会话与其乐局卡；终态或卡不可加载
// 时 ok=false（拉回计数对终态/坏卡静默跳过属正常：终局后乐局不再引导）。
func (s *Server) eduJamActiveSessionCard(conversationID string) (eduJamSession, *edu.LevelCard, bool) {
	session, ok := s.eduJamSessionFor(conversationID)
	if !ok || eduJamSessionTerminal(session) || session.Status == eduJamStatusAwaitingClosure {
		return eduJamSession{}, nil, false
	}
	card, err := loadEduJamLevelCard(session.LevelID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[edu-jam] pullback accounting skipped (level card unavailable): %v", err)
		}
		return eduJamSession{}, nil, false
	}
	return session, card, true
}

// eduJamEnsureWorkspaceActivation 确保 enter 落盘前工作区激活已完成（281 修复
// 腿，主管裁定 B 形态）。两步：
//
//  1. 同步触发一次激活：activateCurrentProjectWorkspace 幂等（同身份同工作会话
//     走 fast-path 返回）；身份源（shadow summary）已追上内核切换时，此处即
//     完成激活并按盘上快照 restore——此后同身份激活不再触发整表覆盖。
//  2. 双源核对：激活工作区 uuid（activeWorkspaceUUID）必须与内核当前工程 uuid
//     （get_project_state 直读真值，与 observeEduJam 同读面、不经 shadow 惯性）
//     一致；任一缺失或不一致 = 激活未完成/身份滞后 → 返回 reason（enter 拒绝）。
//
// 消灭的竞态类（smokerun1_20261107_0924 exit 4 根因）：enter 在 pre-activation
// 窗口开局（工作区未激活或仍绑旧工程）→ 同/后续请求边界工作区惰性激活 →
// restoreProjectAgentRuntimeStateLocked 以新工程空态执行
// `s.eduJamSessions = s.restoreEduJamSessionsLocked(空)`（server.go 恢复面）整表
// 覆盖在途会话 → 推进轮查无会话按设计放行。门拒绝开局后，激活窗口内不再有
// 在途会话可丢；门通过时 enter 绑定的即激活工作区本身（enter 同步持久化），
// 后续同身份激活走 fast-path 或从含会话的盘上快照 restore，均不丢会话。
//
// 边界（如实声明）：门通过后内核才切换工程 = 用户真实换工程动作，属 B5 跨身份
// 切换语义（settleLiveChainsForWorkspaceSwitch + 新工程快照 restore），不在本
// 竞态类内；内核 uuid 只作核对真值，不作任何恢复/落盘依据（FIX-START-STATE-
// RECOVERY-1 用户裁定：kernel 上报 uuid 不是恢复基址——本门不扫描草稿、不绕过
// 激活身份源，仅拒绝两者不一致时的开局）。
func (s *Server) eduJamEnsureWorkspaceActivation(ctx context.Context) (string, string) {
	if s == nil {
		return "", "server_unavailable"
	}
	// 步 1：同步触发激活（harness/shadow 不可用或身份源未就绪时为 no-op，
	// 由步 2 双源核对兜底拒绝）。
	s.activateCurrentProjectWorkspace(ctx)
	// 步 2a：激活工作区标识。
	s.mu.Lock()
	activeUUID := strings.TrimSpace(s.activeWorkspaceUUID)
	s.mu.Unlock()
	if activeUUID == "" {
		return "", "workspace_not_activated"
	}
	// 步 2b：内核当前工程真值直读。
	send := s.eduJamKernelSendFunc()
	if send == nil {
		return "", "kernel_read_face_unavailable"
	}
	stateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reply, err := send(stateCtx, map[string]any{"cmd": "get_project_state"})
	if err != nil || !eduJamKernelReplyOK(reply) {
		return "", "kernel_identity_unavailable"
	}
	kernelUUID := strings.TrimSpace(firstStringFromMap(reply, "project_uuid", "project_id"))
	if kernelUUID == "" {
		return "", "kernel_identity_unavailable"
	}
	if !strings.EqualFold(kernelUUID, activeUUID) {
		// 内核已切换而工作区仍绑旧工程（shadow 惯性窗口）——正是丢会话竞态
		// 的入口形态，fail-closed 拒绝。
		return "", "workspace_identity_lagging_kernel"
	}
	return activeUUID, ""
}

// eduJamGateReasonText 把门拒绝原因翻成回话内的用户可读短语（回话主干
// 「工程还没打开好，稍等再开始乐局」由 enter 模板统一给出）。
func eduJamGateReasonText(reason string) string {
	switch reason {
	case "workspace_not_activated":
		return "还没有打开的工程"
	case "kernel_read_face_unavailable":
		return "暂时读不到内核工程"
	case "kernel_identity_unavailable":
		return "暂时读不到内核工程标识"
	case "workspace_identity_lagging_kernel":
		return "工程正在切换，工作区还没跟上"
	case "server_unavailable":
		return "服务不可用"
	}
	return "工作区还没就位"
}

// ---- 工程↔level 绑定元数据 + 自动开场/恢复播报（LEGAME-AUTOINTRO-1，315）----
//
// 声明性绑定（与 F4 会话态区分）：工程旁 `.vit_agent/legame_binding.json` sidecar，
// 写入者=布位脚本（scripts/legame_placement.py write-binding）与未来新建工程向导
//（bound_source 枚举预留）；agent 只读 binding 本体、只回写 opened_at/
// opened_conversation_id 两个开场标记。无 binding 文件=普通工程，行为与现状零差异
//（§11 兼容义务：缺文件非错误、零迁移）。

const (
	legameBindingSchemaVersion = "vit.legame_binding.v1"
	legameBindingDirname       = ".vit_agent"
	legameBindingFilename      = "legame_binding.json"
)

type legameBinding struct {
	SchemaVersion        string  `json:"schema_version"`
	LevelID              string  `json:"level_id"`
	BoundAt              string  `json:"bound_at"`
	BoundSource          string  `json:"bound_source"`
	OpenedAt             *string `json:"opened_at"`
	OpenedConversationID *string `json:"opened_conversation_id"`
}

// legameBindingPath 工程旁 sidecar 路径（projectPath=.vit 工程文件路径；binding
// 落工程所在目录的 .vit_agent/ 下，与 projectstore.AgentDirName 同域）。
func legameBindingPath(projectPath string) string {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Clean(projectPath)), legameBindingDirname, legameBindingFilename)
}

// loadLegameBinding 读取 binding（fail-closed：缺文件/读失败/schema 未知/level id
// 非法 → 视同无 binding，普通工程零差异；不写日志——每轮 chat 都会探测，普通工程
// 的「无 binding」是常态而非事件）。
func loadLegameBinding(projectPath string) (legameBinding, bool) {
	path := legameBindingPath(projectPath)
	if path == "" {
		return legameBinding{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return legameBinding{}, false
	}
	var binding legameBinding
	if err := json.Unmarshal(data, &binding); err != nil {
		return legameBinding{}, false
	}
	if binding.SchemaVersion != legameBindingSchemaVersion {
		return legameBinding{}, false
	}
	if !eduJamLevelIDRegex.MatchString(binding.LevelID) {
		return legameBinding{}, false
	}
	return binding, true
}

// writeLegameBindingOpened 回写开场标记（agent 对 binding 的唯一写面：只填
// opened_at/opened_conversation_id，不动绑定本体；temp+rename 原子落盘）。
func writeLegameBindingOpened(projectPath string, binding legameBinding, conversationID string) error {
	path := legameBindingPath(projectPath)
	if path == "" {
		return fmt.Errorf("legame binding: empty project path")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	binding.OpenedAt = &now
	binding.OpenedConversationID = &conversationID
	data, err := json.MarshalIndent(binding, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

// eduJamProjectActiveSession 返回该工程（按 ProjectUUID 匹配）最新的非终态乐局
// 会话与其卡；无活跃局时 ok=false（活跃=非终态：active/awaiting_closure 都算——
// 后者是「全关已过待收尾」，恢复播报的收尾话术分支正为此形态）。
func (s *Server) eduJamProjectActiveSession(projectUUID string) (eduJamSession, *edu.LevelCard, bool) {
	if s == nil || strings.TrimSpace(projectUUID) == "" {
		return eduJamSession{}, nil, false
	}
	s.mu.Lock()
	sessions := make([]eduJamSession, 0, len(s.eduJamSessions))
	for _, session := range s.eduJamSessions {
		sessions = append(sessions, session)
	}
	s.mu.Unlock()
	var best eduJamSession
	found := false
	for _, session := range sessions {
		if !strings.EqualFold(session.ProjectUUID, projectUUID) || eduJamSessionTerminal(session) {
			continue
		}
		// 最新非终态局（EnteredAt 字典序可比：RFC3339 UTC；并列时按会话 id 定序
		// 保证确定性）。
		if !found || session.EnteredAt > best.EnteredAt ||
			(session.EnteredAt == best.EnteredAt && session.ConversationID > best.ConversationID) {
			best = session
			found = true
		}
	}
	if !found {
		return eduJamSession{}, nil, false
	}
	card, err := loadEduJamLevelCard(best.LevelID)
	if err != nil {
		return eduJamSession{}, nil, false
	}
	return cloneEduJamSession(best), card, true
}

// eduJamRecoveryBroadcast 重进恢复播报（确定性模板，零 LLM；设计稿 §2.2-3）：
// 「接着上局：{节点标题}（{n}/{total}）——继续或问我都行」；全节点已过（待收尾）
// 形态给收尾引导。播报只读不动状态（不触碰挂起/切换语义，§7-5）。
func (s *Server) eduJamRecoveryBroadcast(conversationID, mode string, session eduJamSession, card *edu.LevelCard) ChatResponse {
	base := s.eduJamBaseResponse(conversationID, mode)
	nodeID := eduJamCurrentNodeID(session, card)
	data := map[string]any{
		"schema_version":        eduJamSessionSchemaVersion,
		"level_id":              session.LevelID,
		"status":                session.Status,
		"owner_conversation_id": session.ConversationID,
		"current_node":          nodeID,
	}
	if nodeID != "" {
		index, total := 0, len(card.Nodes)
		for i, n := range card.Nodes {
			if n.ID == nodeID {
				index = i + 1
				break
			}
		}
		title := ""
		for _, n := range card.Nodes {
			if n.ID == nodeID {
				title = n.Title
				break
			}
		}
		base.Reply = fmt.Sprintf("接着上局：%s（%d/%d）——继续或问我都行。", title, index, total)
		data["node_index"] = index
		data["node_total"] = total
	} else {
		base.Reply = fmt.Sprintf("接着上局：《%s》全部关卡都过了，就差收尾——说「乐局状态」看看，或「确认收尾」完成这一局。", card.Title)
	}
	base.StopReason = "edu_jam_recovery_broadcast"
	base.WorkflowData = data
	return base
}

func (s *Server) rememberEduJamIntroServedLocked(conversationID string) {
	if s.eduJamIntroServed == nil {
		s.eduJamIntroServed = map[string]bool{}
	}
	s.eduJamIntroServed[conversationID] = true
}

// eduJamAutoIntroCheckpoint 进入工程自动开场/重进恢复播报检查点（LEGAME-
// AUTOINTRO-1；handleChat 在 activation+restore 完成后、语义入口分类前调用）。
//
// 注入条件（设计稿 §2.2-2 五项，全查）：
//  1. binding 在案（`.vit_agent/legame_binding.json` 可读且 schema/id 合法）
//  2. opened_at 为空（开场=工程级一次性事件；已开场工程的第二会话不重开场）
//  3. restore 后该工程无活跃（非终态）EduJamSession（有活跃局→恢复播报分支）
//  4. enter 门通过（复用 eduJamEnter 内核：其内建的 eduJamEnsureWorkspaceActivation
//     双源门正是 F5 竞态防线——门未过=不拦本轮、不写任何状态，下轮重试）
//  5. chat 主链路（handleChat 调用点已排除 note；乐局命令词形与 "/" 斜杠消息
//     放行给既有命令面，不在此拦截）
//
// 幂等语义：开场工程级一次性（opened_at+opened_conversation_id 落盘回写）；恢复
// 播报会话级一次性（eduJamIntroServed 进程内标记——跨进程重启后重播一次正是
// 「重进工程」的设计形态）。终态局→静默（HUD/命令面照常，§2.2-3）。
//
// fail-soft 边界：自动开场是增强注入，任何 enter 失败（门拒绝/卡不可用/状态拒绝）
// 都不拦截用户本轮对话——不写 opened_at，下一轮重试；手动「开始乐局」命令面语义
// 单源不变。
func (s *Server) eduJamAutoIntroCheckpoint(ctx context.Context, conversationID, mode, text string) (ChatResponse, bool) {
	if s == nil {
		return ChatResponse{}, false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ChatResponse{}, false
	}
	if _, _, _, _, _, ok := parseEduJamCommand(trimmed); ok {
		return ChatResponse{}, false // 命令词归命令面（自动开场与命令词同一 enter 内核，语义单源）
	}
	if strings.HasPrefix(trimmed, "/") {
		return ChatResponse{}, false // 斜杠命令归既有命令路径
	}
	// eduJamIntroMu 串行化整个决策+动作：并发两个会话同时命中开场条件时，第二
	// 个进入时 opened_at 已落盘（或活跃局已建立），不会双开场。
	s.eduJamIntroMu.Lock()
	defer s.eduJamIntroMu.Unlock()
	s.mu.Lock()
	projectPath := strings.TrimSpace(s.activeWorkspacePath)
	projectUUID := strings.TrimSpace(s.activeWorkspaceUUID)
	served := s.eduJamIntroServed[conversationID]
	s.mu.Unlock()
	if projectPath == "" || served {
		return ChatResponse{}, false
	}
	binding, ok := loadLegameBinding(projectPath)
	if !ok {
		return ChatResponse{}, false // 普通工程：零行为差异
	}
	// 恢复播报（活跃局在场=已开场过或手动开过；先于开场判定）。
	if session, card, has := s.eduJamProjectActiveSession(projectUUID); has {
		resp := s.eduJamRecoveryBroadcast(conversationID, mode, session, card)
		s.mu.Lock()
		s.rememberEduJamIntroServedLocked(conversationID)
		s.mu.Unlock()
		return resp, true
	}
	if binding.OpenedAt != nil && strings.TrimSpace(*binding.OpenedAt) != "" {
		return ChatResponse{}, false // 已开场且无活跃局（终态）→静默
	}
	resp, handled := s.eduJamEnter(ctx, conversationID, mode, binding.LevelID)
	if !handled || resp.StopReason != "edu_jam_entered" {
		// fail-soft：不拦本轮、不写 opened_at；门拒绝/卡不可用下轮重试。
		if s.logger != nil {
			s.logger.Info("[edu-jam] auto-intro deferred (enter not completed, will retry on a later turn): stop=%s", resp.StopReason)
		}
		return ChatResponse{}, false
	}
	if err := writeLegameBindingOpened(projectPath, binding, conversationID); err != nil {
		// 开场已在本会话发生（session 已落盘）；回写失败只影响跨进程的
		// opened_at 标记——活跃局在场仍会挡住二次开场，如实告警不回滚。
		if s.logger != nil {
			s.logger.Warn("[edu-jam] auto-intro binding opened_at write-back failed (in-session intro stands): %v", err)
		}
	}
	s.mu.Lock()
	s.rememberEduJamIntroServedLocked(conversationID)
	s.mu.Unlock()
	return resp, true
}

func (s *Server) eduJamWorkflowData(session eduJamSession, extra map[string]any) map[string]any {
	data := map[string]any{
		"schema_version": eduJamSessionSchemaVersion,
		"level_id":       session.LevelID,
		"status":         session.Status,
	}
	if nodeID := func() string {
		if card, err := loadEduJamLevelCard(session.LevelID); err == nil {
			return eduJamCurrentNodeID(session, card)
		}
		return ""
	}(); nodeID != "" {
		data["current_node"] = nodeID
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// ---- 乐局卡装载（进程内缓存：restore 每 tick 重放不做磁盘重读）----

var eduJamCardCache sync.Map // levelID -> *edu.LevelCard

// eduJamLevelsDir 三级解析乐局卡根目录（LEGAME-DIR-1 / 313 设计稿 §2.1，产品式
// 手测 v2 硬前置——用户手测路径零 env 零终端，违例一根因收口）：
//
//  1. env VIT_EDU_LEVELS_DIR（保留：测试/开发态覆写；设置即用、不静默回落——
//     覆写指向坏路径属配置错误，fail-closed 报错优于悄悄换目录）
//  2. ~/.vit/levels 产品主落点（安装布局 <dir>/<level_id>/ 整卡目录，与仓库
//     coord/levels 同构；内容分发归布位/发布脚本[scripts/legame_placement.py
//     install-levels；build_release 可选段]，agent 不自装）
//  3. coord/levels 仓库相对路径回退（开发态现状：agent cwd=仓库根时可达；
//     前端拉起 agent 时 cwd≠仓库根，此级不可达正是三级制的存在理由）
//
// 惯例对齐 F6（config.Path / genapi.DefaultConfigPath 的「用户目录默认 + env
// 覆写」）：不进 config.json 键——零配置目标下多一个键=多一个出错面，未来确需
// 用户改目录时再加键不迟（键缺省=走三级解析）。
func eduJamLevelsDir() string {
	if dir := strings.TrimSpace(os.Getenv("VIT_EDU_LEVELS_DIR")); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		levels := filepath.Join(home, ".vit", "levels")
		if info, statErr := os.Stat(levels); statErr == nil && info.IsDir() {
			return levels
		}
	}
	return filepath.Join("coord", "levels")
}

func eduJamResolveLevelID(hint string) string {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return eduJamDefaultLevelID
	}
	if id, ok := eduJamLevelAliases[strings.ToLower(hint)]; ok {
		return id
	}
	return hint
}

func loadEduJamLevelCard(levelID string) (*edu.LevelCard, error) {
	if !eduJamLevelIDRegex.MatchString(levelID) {
		return nil, fmt.Errorf("edu/jam: illegal level id %q", levelID)
	}
	if cached, ok := eduJamCardCache.Load(levelID); ok {
		return cached.(*edu.LevelCard), nil
	}
	card, err := edu.LoadLevelCard(filepath.Join(eduJamLevelsDir(), levelID, "level.yaml"))
	if err != nil {
		return nil, err
	}
	eduJamCardCache.Store(levelID, card)
	return card, nil
}

// ---- restore（fail-closed：逐会话校验，坏会话丢弃计数 + Warn 留痕）----

func (s *Server) restoreEduJamSessionsLocked(restored map[string]eduJamSession) map[string]eduJamSession {
	clean := map[string]eduJamSession{}
	dropped := 0
	for convID, session := range restored {
		bad := false
		if session.SchemaVersion != eduJamSessionSchemaVersion || strings.TrimSpace(convID) == "" || session.ConversationID != convID {
			bad = true
		}
		if !bad && !eduJamSessionStatusValid(session.Status) {
			bad = true
		}
		if !bad {
			if _, err := loadEduJamLevelCard(session.LevelID); err != nil {
				bad = true
			}
		}
		if !bad {
			if blob, err := json.Marshal(session.Jam); err == nil {
				if _, _, err := edu.RestoreJamState(blob, mustEduJamCard(session.LevelID)); err != nil {
					bad = true
				}
			} else {
				bad = true
			}
		}
		if bad {
			dropped++
			continue
		}
		clean[convID] = cloneEduJamSession(session)
	}
	if dropped > 0 && s.logger != nil {
		s.logger.Warn("[edu-jam] restore dropped %d invalid jam session(s) (fail-closed, never executed)", dropped)
	}
	return clean
}

func mustEduJamCard(levelID string) *edu.LevelCard {
	card, err := loadEduJamLevelCard(levelID)
	if err != nil {
		return nil // restore 视坏卡会话为坏（上一行已校验，此处仅防御）
	}
	return card
}

func eduJamSessionStatusValid(status string) bool {
	switch status {
	case eduJamStatusActive, eduJamStatusAwaitingClosure, eduJamStatusCompleted, eduJamStatusExited:
		return true
	}
	return false
}
