package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

// CONT-STALL-1（2026-09-12 22:53 真栈，会话 webui_mtyi4sjw，goal_5b9cb1a9e48ace5b；
// 2026-09-13 09:02 同型复现，goal_a36712edacc9cb50）：切片以
// waiting_continue + stop=limit_reached 结束、检查点已按既定节奏 arm 成 pending，
// 随后**调度器自己的 reload 把它 fail-closed 成 waiting_interaction**，claim 再也
// 取不到它——203.5 s 零事件零日志的死驻留，而该回合的回复仍承诺「我还在继续处理
// 这个任务，完成后再向你汇报。」。
//
// 根因（本文件钉住）：reconcileDurableCapabilityRoutes 把 continuation 上下文里的
// capability_route_decision 身份当成了「durable capacity state」。但该身份可以没有
// capacity assessment（direct typed action 的语义入口就是这条路径：
// capability_routing.go 只在 record.Assessment != nil 时 storeCapabilityRoute，
// restoreCapabilityRoutes 也拒绝收没有有效 assessment 的记录），于是
// routes[TaskID] 永远查不到，记录每次 reload 都被判「没有已验证任务路由」——
// 一个按构造不可能被满足的条件，必然导致永久死驻留。
//
// 真栈持久化形态（run 20260913_090238 的 agent_runtime_state.json 逐字段）：
//
//	durable_continuations[cont_c9fe09a5...].status        = "pending"
//	...continuation.context.capability_route_decision     = {controller:"direct_typed_action",
//	                                                         semantic_entry.route:"explicit_control", ...}
//	...continuation.context.free_state_capacity_assessment = 缺省
//	...capacity_assessment / capability_entry_plan          = null
//	capability_routes                                       = 空表
//
// 钉的意义：armed 的 pending 检查点必须在调度器的 reload 之后仍然可被 claim。
const contStallConversation = "conversation-cont-stall"

// contStallArmedResult 构造真栈那一拍的切片信封：waiting_continue +
// limit_reached + 携带路由身份的 continuation（无 capacity assessment）。
func contStallArmedResult() agentloop.Result {
	return agentloop.Result{
		GoalID: "goal-cont-stall", RunID: "run-cont-stall", TaskID: "task-cont-stall",
		SliceID: "slice-cont-stall", TurnID: "turn-cont-stall",
		OriginalIntent: "把低音轨/bass 提 1dB",
		Status:         agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-cont-stall", RunID: "run-cont-stall", TaskID: "task-cont-stall",
			SliceID: "slice-cont-stall", TurnID: "turn-cont-stall",
			OriginalIntent: "把低音轨/bass 提 1dB", UserText: "把低音轨/bass 提 1dB", Summary: "把低音轨/bass 提 1dB",
			Context: map[string]any{
				"conversation_id": contStallConversation,
				"goal_id":         "goal-cont-stall",
				"run_id":          "run-cont-stall",
				"task_id":         "task-cont-stall",
				"original_intent": "把低音轨/bass 提 1dB",
				// 真栈逐字段形态：路由身份在案，assessment / entry_plan 皆无。
				capabilityRouteContextKey: map[string]any{
					"schema_version":   capabilityRouteSchema,
					"task_id":          "task-cont-stall",
					"goal_id":          "goal-cont-stall",
					"run_id":           "run-cont-stall",
					"conversation_id":  contStallConversation,
					"original_intent":  "把低音轨/bass 提 1dB",
					"controller":       "direct_typed_action",
					"project_revision": "",
					"semantic_entry": map[string]any{
						"schema_version": semanticEntryDecisionSchema, "route": "explicit_control",
						"controller": "direct_typed_action", "target_scope": semanticEntryScopeProjectContext,
						"control_mode": "typed_control", "user_authorization": "action_requested", "confidence": 0.95,
					},
				},
			},
		},
	}
}

func contStallDurableStatuses(s *Server) map[string]DurableContinuation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]DurableContinuation{}
	for id, item := range s.durableContinuations {
		out[id] = item
	}
	return out
}

// 钉①（RED→GREEN 主钉）：arm 出来的 pending 检查点在调度器 reload 之后必须仍然
// 可被 claim。reload 前的 pending 是既有行为（另一条钉已覆盖）；本钉钉的是
// **reload 之后**——真栈里就是这一步把链条钉死的。
func TestContStallArmedCheckpointSurvivesSchedulerReload(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	// 本钉同步驱动 claim，不需要后台 worker（与既有 STAB1 家族同一处理）。
	s.schedulerWake = nil
	s.activeWorkspaceUUID = "vitproj-cont-stall"
	s.conversationGoals[contStallConversation] = "goal-cont-stall"

	if err := s.recordGoalResult(contStallConversation, contStallArmedResult()); err != nil {
		t.Fatalf("arming the checkpoint failed: %v", err)
	}
	armed := contStallDurableStatuses(s)
	if len(armed) != 1 {
		t.Fatalf("expected exactly one durable checkpoint, got %d: %+v", len(armed), armed)
	}
	for id, item := range armed {
		if item.Status != ContinuationPending {
			t.Fatalf("checkpoint %s did not arm as pending: %+v", id, item)
		}
	}
	// setup 断言：上下文里确实只有路由身份、没有 capacity state——这正是真栈形态。
	for _, item := range armed {
		if item.CapacityAssessment != nil || item.CapabilityEntryPlan != nil {
			t.Fatalf("setup: the real-stack shape carries no capacity assessment/entry plan: %+v", item)
		}
		if item.Continuation.Context[capabilityRouteContextKey] == nil {
			t.Fatal("setup: the routing identity must be present in the continuation context")
		}
	}

	// 调度器 tick 的同一段 reload：读盘快照 + 归一化 + 路由对账。
	snapshot := s.projectAgentRuntimeStateLocked()
	s.restoreProjectAgentRuntimeStateLocked(snapshot)

	after := contStallDurableStatuses(s)
	if len(after) != 1 {
		t.Fatalf("reload changed the checkpoint set: %+v", after)
	}
	for id, item := range after {
		if item.Status != ContinuationPending {
			t.Fatalf("reload rewrote the armed checkpoint %s to %q (reason=%v): the scheduler can never claim it, which is the 203 s dead park",
				id, item.Status, item.PendingInteraction)
		}
		if reason := firstStringFromMap(item.PendingInteraction, "reason"); reason != "" {
			t.Fatalf("reload quarantined the armed checkpoint %s: %s", id, reason)
		}
	}
	claimed, ok := s.claimNextContinuation(time.Now().UTC())
	if !ok {
		t.Fatal("the armed checkpoint was not claimable after the scheduler's own reload")
	}
	if claimed.ContinuationID == "" || claimed.GoalID != "goal-cont-stall" {
		t.Fatalf("claim returned an unexpected checkpoint: %+v", claimed)
	}
}

// 钉②（边界反向锁定）：真正携带 capacity state 却拿不到已验证任务路由的检查点
// **仍然 fail-closed**——修法收窄的是谓词对「capacity state」的认定，不是这条
// 保护本身。语料取自真栈同族：assessment 在案而 capability_routes 为空。
func TestContStallCapacityStateStillFailsClosedWithoutValidatedRoute(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	s.schedulerWake = nil
	s.activeWorkspaceUUID = "vitproj-cont-stall"
	s.conversationGoals[contStallConversation] = "goal-cont-stall"

	res := contStallArmedResult()
	// A genuinely valid capacity assessment (the same shape production writes):
	// this is exactly what a "validated task route" exists to cover.
	res.Continuation.Context[capacityAssessmentContextKey] = FreeStateCapacityAssessment{
		SchemaVersion: capacityAssessmentSchema, Authority: "product_runtime",
		ProjectRevision: "rev-1", SelectedCapability: capabilityFreeState,
		ObservedFacts: CapacityObservedFacts{
			ProjectUUID: "vitproj-cont-stall", ProjectRevision: "rev-1",
			RequestScope: semanticEntryScopeProjectContext,
		},
	}
	s.recordGoalResult(contStallConversation, res)

	snapshot := s.projectAgentRuntimeStateLocked()
	s.restoreProjectAgentRuntimeStateLocked(snapshot)

	for id, item := range contStallDurableStatuses(s) {
		if item.Status != ContinuationWaitingInteraction {
			t.Fatalf("capacity state without a validated task route must stay fail-closed: %s=%+v", id, item)
		}
	}
	if _, ok := s.claimNextContinuation(time.Now().UTC()); ok {
		t.Fatal("a fail-closed capacity checkpoint was claimed")
	}
}
