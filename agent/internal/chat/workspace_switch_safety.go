package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/logx"
	agentruntime "vit-daw-agent/internal/runtime"
)

// B5：工程打开×工作区激活竞态的会话安全层。
//
// 现场缺陷（2026-09-11 手测点 3 尝试 #1）：agent 启动期绑定陈旧 draft 工作区，
// chat 回合健康 checkpoint（waiting_continue）后，Godot 打开工程触发
// activateCurrentProjectWorkspace 同 uuid 新路径再激活——内存会话态被新工作区
// 空态整体替换，在飞链从此无人驱动（claimNextContinuation 按激活 uuid 过滤），
// 会话图止于中间汇报节点，终局永不发生。
//
// 选定语义（三选一之"先收尾"）：切换前对旧工作区的在飞链显式收尾——
// 终局投递（turn.stopped 通知事件）+ 落图（旧工作区会话图通知节点）+
// 记录显式取消（原因=工作区切换），禁止静默整体替换。可应答驻留
// （waiting_interaction 公园）不取消不投递——F1 口径：可应答驻留不得被
// 生命周期事件孤儿化，它们留在旧工作区持久态里保持可应答。

const (
	// workspaceSwitchStopReason 标记因工作区切换被显式收尾的链。
	workspaceSwitchStopReason = "workspace_switched"

	// restoredContinuationStaleness 是恢复卫生的陈旧阈值：一个 runnable
	// continuation 的 UpdatedAt 落后现在超过该值时，写入它的进程早已死透
	// （租约 2 分钟、调度 tick 250ms，正常重启恢复在秒级完成）——恢复它
	// 只会把死会话投影成幻忙态（遗留 #8 家族）。零时间戳视为来源不明、
	// fail-open 保持可恢复（既有重启恢复测试依赖）。
	restoredContinuationStaleness = 30 * time.Minute
)

// settleLiveChainsForWorkspaceSwitch closes the old workspace's in-flight
// chains explicitly before the activation switch replaces the in-memory
// runtime state. Runnable checkpoints (pending/claimed/running) can never run
// against the next workspace — the scheduler claims per active uuid — so the
// honest close is an explicit cancel, a delivered turn.stopped notice, and a
// notice node in the OLD workspace's conversation graph. Answerable parks
// (waiting_interaction) keep their durable record untouched: they stay
// answerable in the old workspace (F1 alignment).
//
// Caller contract: invoked from activateCurrentProjectWorkspace while holding
// schedulerExecutionMu and workspaceMu (so no scheduler slice can be mid-flight
// here); never called under s.mu.
func (s *Server) settleLiveChainsForWorkspaceSwitch(ctx context.Context, nextPath, nextUUID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	live := make([]DurableContinuation, 0, len(s.durableContinuations))
	for _, item := range s.durableContinuations {
		if !continuationRunnableStatus(item.Status) {
			continue
		}
		live = append(live, cloneDurableContinuation(item))
	}
	s.mu.Unlock()
	if len(live) == 0 {
		return
	}
	settled := 0
	for _, item := range live {
		reason := fmt.Sprintf("workspace switched to %s (%s); in-flight chain explicitly closed before activation", nextPath, nextUUID)
		// 内联取消（镜像 setContinuationStatus 的终态变更与兄弟级联）而不
		// 走它本身：setContinuationStatus 末尾同步 persistContinuationState
		// → persistCurrentProjectWorkspaceChecked → workspaceMu，而本收尾正
		// 运行在 activateCurrentProjectWorkspace 已持有的 workspaceMu 之下
		// （2026-09-11 B5 单测死锁实证）。持久化由激活路径紧随其后的
		// persistActiveProjectWorkspaceLocked 统一完成。
		cancelledAt := time.Now().UTC()
		s.mu.Lock()
		if current, exists := s.durableContinuations[item.ContinuationID]; exists && !continuationTerminalStatus(current.Status) {
			current.Status = ContinuationCancelled
			current.LeaseOwner = ""
			current.LeaseExpiresAt = time.Time{}
			current.UpdatedAt = cancelledAt
			current.LastError = reason
			s.durableContinuations[item.ContinuationID] = cloneDurableContinuation(current)
			delete(s.goalContinuations, current.GoalID)
			for otherID, other := range s.durableContinuations {
				if otherID == item.ContinuationID || other.GoalID != current.GoalID || continuationTerminalStatus(other.Status) {
					continue
				}
				other.Status = ContinuationCancelled
				other.LeaseOwner = ""
				other.LeaseExpiresAt = time.Time{}
				other.UpdatedAt = cancelledAt
				other.LastError = reason
				s.durableContinuations[otherID] = cloneDurableContinuation(other)
			}
		}
		s.mu.Unlock()
		if s.harness != nil {
			// Fold the chain's driving goal to the honest stopped form. Only
			// the auto-continue statuses fold: waiting_confirmation/
			// waiting_clarification parks stay answerable in the old workspace.
			switch s.harness.RuntimeStatus(item.GoalID).Status {
			case agentruntime.StatusRunning, agentruntime.StatusWaitingContinue:
				s.harness.SetGoalStatus(item.GoalID, agentruntime.StatusStopped, nil)
			}
		}
		resp := ChatResponse{
			ConversationID: item.ConversationID, GoalID: item.GoalID, RunID: item.RunID,
			GoalStatus: string(agentruntime.StatusStopped),
			Reply:      workspaceSwitchNoticeText(item, nextPath),
			StopReason: workspaceSwitchStopReason,
		}
		recordPath := firstNonEmpty(item.ProjectPath, s.activeWorkspacePath)
		s.recordWorkspaceSwitchNoticeNode(ctx, recordPath, item, resp)
		s.emitSchedulerChainResultEvent(item.ConversationID, resp, nil)
		settled++
	}
	if s.logger != nil {
		s.logger.Info("[workspace] switch settled live chains=%d next=%s uuid=%s", settled, nextPath, nextUUID)
	}
}

func workspaceSwitchNoticeText(item DurableContinuation, nextPath string) string {
	intent := strings.TrimSpace(item.OriginalIntent)
	if intent == "" {
		intent = strings.TrimSpace(item.Continuation.UserText)
	}
	if intent != "" {
		return fmt.Sprintf("工程已切换（%s）：对话「%s」的在飞处理链已显式收尾，本回合停止。切回原工程可查看完整记录。", nextPath, intent)
	}
	return fmt.Sprintf("工程已切换（%s）：本对话的在飞处理链已显式收尾，本回合停止。切回原工程可查看完整记录。", nextPath)
}

// recordWorkspaceSwitchNoticeNode lands the settle notice in the OLD
// workspace's conversation graph. It appends directly through history instead
// of the harness record API: a vit-kind harness record would ride the vit
// checkpoint gate, and at switch time the kernel already holds the NEXT
// project's state — exporting it into the old workspace's history would
// misattribute the snapshot. The direct append binds the old workspace's
// current HEAD (the chain's own last checkpoint), runs zero kernel exports,
// and returns a real error the harness wrapper would have swallowed into a
// warnings row (B5 留痕要求：失败显式 WARN，不静默）。message_kind 落
// execution_receipt（回合边界回执的既有词汇，20260911 取证 orphaned graph
// 同款），刷新水合按 assistant 文本呈现本链最后一句话。
func (s *Server) recordWorkspaceSwitchNoticeNode(ctx context.Context, projectPath string, item DurableContinuation, resp ChatResponse) {
	if s == nil || strings.TrimSpace(resp.Reply) == "" {
		return
	}
	historyData := chatResponseHistoryData(resp, nil)
	node, err := history.AppendConversationNode(map[string]any{
		"project_path":    strings.TrimSpace(projectPath),
		"kind":            "vit",
		"text":            resp.Reply,
		"goal_id":         resp.GoalID,
		"run_id":          resp.RunID,
		"turn_id":         firstNonEmpty(item.CurrentTurnID, resp.RunID),
		"message_kind":    "execution_receipt",
		"message_data":    historyData,
		"source":          "workspace_switch_settle",
		"set_active_node": true,
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[workspace] switch settle notice NOT recorded project=%s conversation=%s goal=%s error=%v",
				projectPath, item.ConversationID, item.GoalID, err)
		}
		return
	}
	if s.logger != nil {
		s.logger.Info("[workspace] switch settle notice recorded project=%s conversation=%s node_id=%v",
			projectPath, item.ConversationID, firstStringFromMap(node, "active_node_id"))
	}
	_ = ctx
}

// foldStaleRestoredContinuations normalizes restored live-looking
// continuations whose checkpoint is older than the staleness horizon to the
// cancelled terminal (B5 fix 2, legacy #8 family): a stale runnable
// checkpoint projects as live work on every runtime-status surface (webui
// 开屏幻忙态) and can never be honestly resumed. The same fold covers the
// unanswerable shells a stale snapshot leaves behind (recovery-validation or
// legacy_waiting_continue markers — nothing can ever answer them). Genuine
// answerable parks stay untouched (F1); unknown timestamps (zero) stay
// runnable so crash-restart recovery keeps working.
func foldStaleRestoredContinuations(items map[string]DurableContinuation, now time.Time, logger *logx.Logger) map[string]DurableContinuation {
	if len(items) == 0 {
		return items
	}
	folded := 0
	for id, item := range items {
		stale := !item.UpdatedAt.IsZero() && now.Sub(item.UpdatedAt) > restoredContinuationStaleness
		if !stale {
			continue
		}
		live := continuationRunnableStatus(item.Status)
		unanswerableShell := item.Status == ContinuationWaitingInteraction &&
			(continuationRecoveryValidationRequired(item) || legacyWaitingContinuePendingInteraction(item.PendingInteraction))
		if !live && !unanswerableShell {
			continue
		}
		item.Status = ContinuationCancelled
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.LastError = fmt.Sprintf("restored checkpoint stale for %s (workspace restore hygiene); chain closed as terminal", now.Sub(item.UpdatedAt).Round(time.Second))
		item.UpdatedAt = now
		items[id] = cloneDurableContinuation(item)
		folded++
	}
	if folded > 0 && logger != nil {
		logger.Info("[workspace] stale restored chains folded terminal count=%d", folded)
	}
	return items
}

// foldStaleRestoredGoals settles non-terminal goals restored from a stale
// snapshot when nothing still owes them a next move: the fold keys on the
// already-folded continuation map, so a goal backed by a live continuation or
// an answerable waiting_interaction park keeps its status (F1). The harness
// runtime has just been restored; callers may hold s.mu, so this reads
// s.durableContinuations directly without re-locking.
func (s *Server) foldStaleRestoredGoals(goals []agentruntime.Goal, now time.Time) {
	if s == nil || s.harness == nil || len(goals) == 0 {
		return
	}
	for _, goal := range goals {
		switch goal.Status {
		case agentruntime.StatusRunning, agentruntime.StatusWaitingContinue:
		default:
			continue
		}
		if goal.UpdatedAt.IsZero() || now.Sub(goal.UpdatedAt) <= restoredContinuationStaleness {
			continue
		}
		if s.restoredGoalHasLiveContinuationOwner(goal) {
			continue
		}
		s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusStopped, nil)
		if s.logger != nil {
			s.logger.Info("[workspace] stale restored goal folded stopped goal=%s updated_at=%s", goal.GoalID, goal.UpdatedAt.Format(time.RFC3339))
		}
	}
}

func (s *Server) restoredGoalHasLiveContinuationOwner(goal agentruntime.Goal) bool {
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationPending && item.Status != ContinuationClaimed &&
			item.Status != ContinuationRunning && item.Status != ContinuationWaitingInteraction {
			continue
		}
		if (goal.GoalID != "" && item.GoalID == goal.GoalID) ||
			(goal.RunID != "" && item.RunID == goal.RunID) {
			return true
		}
	}
	return false
}

// foldStaleRestoredFreeStateLoops settles active loops restored from a stale
// snapshot: an active loop projects perpetual progress for a dead session
// (legacy #8 trajectory-card live state). Loops parked at the judgment
// boundary keep their blocked residency (answerable, F1).
func foldStaleRestoredFreeStateLoops(loops map[string]freeStateReasoningLoop, now time.Time, logger *logx.Logger) map[string]freeStateReasoningLoop {
	if len(loops) == 0 {
		return loops
	}
	for id, loop := range loops {
		if !freeStateLoopActive(loop) || freeStateJudgmentBoundary(loop) || loop.UpdatedAt.IsZero() {
			continue
		}
		if now.Sub(loop.UpdatedAt) <= restoredContinuationStaleness {
			continue
		}
		loop.Status = "stopped"
		loop.TerminalTurnReason = "workspace_restore_stale"
		loop.UpdatedAt = now
		loops[id] = loop
		if logger != nil {
			logger.Info("[workspace] stale restored free-state loop folded stopped loop=%s", loop.LoopID)
		}
	}
	return loops
}
