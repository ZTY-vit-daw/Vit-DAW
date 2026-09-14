package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestrationcontroller"
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
// （CONT-STALL-2 补注：本钉期望不变。CONT-STALL-2 修的是注入侧——context 的
// capacity state 必须来自与本 continuation 同 task 的已验证路由；fail-closed 守卫
// 与 restoreCapabilityRoutes 准入集逐字未动。）
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

// ---------------------------------------------------------------------------
// CONT-STALL-2（2026-09-13 22:59 真栈，会话 webui_mtzxugg8，goal_16efa2e9 /
// cont_8061dbfd，约 3.5 分钟 stall 刷屏后用户手动停止）。
//
// 本族另一支路：CONT-STALL-1 修掉的「裸身份」不适用——这条 continuation 的
// context 带着真 capacity state，却照样永久死驻留。盘上工件逐字段
// （.vit_history session_20260913T145738 的 agent_runtime_state.json）：
//
//	durable_continuations[cont_8061dbfd].task_id          = task_4f49f514（第二 goal 自己的）
//	...continuation.context.capability_route_decision      = 第一 goal 的已验证路由
//	    （task_70d5383 / goal_c7ecb4fb，controller=minimal_audio_closure——**在
//	     restoreCapabilityRoutes 白名单内**，route=open_semantic，带 capacity_assessment）
//	...continuation.context.free_state_capacity_assessment = 在场
//	...capacity_assessment（durable 顶层）                  = 在场（由上面 context 解出）
//	capability_routes                                       = 只有第一 goal 的 task
//
// 即：continuation 带着真 assessment，但索引里不存在**与其自身 TaskID 匹配**的
// 已验证路由——reconcileDurableCapabilityRoutes 按 routes[item.TaskID] 查，恒查无，
// 每次 reload fail-closed 成 waiting_interaction，claim 永不取。controller 在不在
// 白名单无关紧要（本例的 controller 就在白名单内）——缺口在注入侧：context 的
// capacity state 可以来自**别的 task** 的路由。
//
// 两条注入路径（均修，见 capability_routing.go 的 CONT-STALL-2 注释）：
//   A. refreshCapabilityRouteForRevision 的会话级 fallback：requestContext 无
//      task_id 时取「该会话最新路由」＝别的 task 的记录，原样注入新请求；
//   B. contextWithCapabilityRoute 只 merge 不删键：注入无 assessment 的新身份
//      （explicit_control 族，按构造永不入索引）时，上一路由留在 context 里的
//      free_state_capacity_assessment / capability_entry_plan 键残留，被
//      durableContinuationFromResult 解成新 continuation 的 capacity state。

// contStallFirstGoalValidatedRoute 复刻 2026-09-13 22:59 工件里第一 goal 的
// 已验证路由（intent「请检查当前工程有什么问题」，observation/open_semantic 族）。
func contStallFirstGoalValidatedRoute() CapabilityRouteRecord {
	now := time.Now().UTC()
	return CapabilityRouteRecord{
		SchemaVersion: capabilityRouteSchema, TaskID: "task-cont-stall-first", GoalID: "goal-cont-stall-first",
		RunID: "run-cont-stall-first", ConversationID: contStallConversation,
		OriginalIntent: "请检查当前工程有什么问题",
		Controller:     string(orchestrationcontroller.MinimalAudioClosure),
		SemanticEntry: map[string]any{
			"route": semanticEntryRouteOpenSemantic, "target_scope": semanticEntryScopeProjectContext,
			"control_mode": semanticEntryControlSemanticLoop, "user_authorization": semanticEntryAuthorizationAction,
			"confidence": 0.9, "reason": "observation first",
		},
		ProjectRevision: "rev-1",
		Assessment: &FreeStateCapacityAssessment{
			SchemaVersion: capacityAssessmentSchema, Authority: "product_runtime",
			ProjectRevision: "rev-1", SelectedCapability: capabilityFreeState,
			ObservedFacts: CapacityObservedFacts{
				ProjectUUID: "vitproj-cont-stall", ProjectRevision: "rev-1",
				RequestScope: semanticEntryScopeProjectContext,
			},
		},
		CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute),
	}
}

// 钉③-A（主钉）：同会话第二个 goal 的请求不得继承第一个 goal 的已验证路由，
// 其检查点必须在调度器自己的 reload 之后仍可被 claim。真栈时序：第一 goal 的
// free-state loop 活跃 ⇒ 第二请求 classificationRequired=false ⇒
// refreshCapabilityRouteForRevision 成为路由 context 的唯一来源。
func TestContStallSecondGoalDoesNotInheritFirstGoalRoute(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	s.schedulerWake = nil
	s.activeWorkspaceUUID = "vitproj-cont-stall"
	s.conversationGoals[contStallConversation] = "goal-cont-stall-second"
	s.mu.Lock()
	s.capabilityRoutes = map[string]CapabilityRouteRecord{"task-cont-stall-first": contStallFirstGoalValidatedRoute()}
	s.mu.Unlock()

	// 第二请求不带 task_id（语义入口未跑——22:59 真实形态）。
	refreshed, err := s.refreshCapabilityRouteForRevision(context.Background(), contStallConversation,
		map[string]any{"conversation_id": contStallConversation})
	if err != nil {
		t.Fatalf("refreshing the route for the second goal failed: %v", err)
	}
	if refreshed[capabilityRouteContextKey] != nil || refreshed[capacityAssessmentContextKey] != nil {
		t.Fatalf("the second goal inherited the first goal's validated route/assessment: durableContinuationFromResult lifts a foreign assessment into its capacity state while the index holds no route for its own task id — the CONT-STALL-2 dead park")
	}

	// 端到端：第二 goal 的切片信封带上 refresh 产物 → arm → 走调度器同一段 reload。
	intent := "对低音轨做一次混音改进实验，改完让我 A/B 试听对比一下"
	res := agentloop.Result{
		GoalID: "goal-cont-stall-second", RunID: "run-cont-stall-second", TaskID: "task-cont-stall-second",
		SliceID: "slice-cont-stall-second-1", TurnID: "turn-cont-stall-second-1",
		OriginalIntent: intent, Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-cont-stall-second", RunID: "run-cont-stall-second", TaskID: "task-cont-stall-second",
			SliceID: "slice-cont-stall-second-1", TurnID: "turn-cont-stall-second-1",
			OriginalIntent: intent, UserText: intent, Summary: intent, Context: refreshed,
		},
	}
	if err := s.recordGoalResult(contStallConversation, res); err != nil {
		t.Fatalf("arming the second goal's checkpoint failed: %v", err)
	}
	snapshot := s.projectAgentRuntimeStateLocked()
	s.restoreProjectAgentRuntimeStateLocked(snapshot)
	for id, item := range contStallDurableStatuses(s) {
		if item.Status != ContinuationPending {
			t.Fatalf("reload fail-closed the second goal's checkpoint %s to %q (reason=%v): the CONT-STALL-2 dead park",
				id, item.Status, item.PendingInteraction)
		}
	}
	claimed, ok := s.claimNextContinuation(time.Now().UTC())
	if !ok || claimed.GoalID != "goal-cont-stall-second" {
		t.Fatalf("the second goal's checkpoint was not claimable after reload: ok=%v goal=%s", ok, claimed.GoalID)
	}
}

// 钉③-B（身份换绑残留钉）：注入不带 assessment 的身份（explicit_control 族，
// planObservationFirstCapabilityRoute 按构造不 store、索引永不收录）时，context
// 里上一路由的 capacity state 必须被同步清除——否则 durableContinuationFromResult
// 把旧 assessment 解成新 task 的 capacity state，制造「有 assessment 而索引无该
// task 路由」的 fail-closed 死驻留。
func TestContStallStaleCapacityStateClearedWithAssessmentlessIdentity(t *testing.T) {
	first := contStallFirstGoalValidatedRoute()
	base := contextWithCapabilityRoute(map[string]any{}, first)
	if base[capacityAssessmentContextKey] == nil {
		t.Fatal("setup: the first goal's route must seed a valid capacity assessment")
	}
	base[capabilityEntryPlanContextKey] = CapabilityEntryPlan{
		SchemaVersion: capabilityEntryPlanSchema, Capability: capabilityProjectMix,
		Stage: "A2", CapabilityID: "project_prep.technical_integrity.v0",
		Source: "explicit_user_request", Reason: "setup residue",
	}

	identity := CapabilityRouteRecord{
		SchemaVersion: capabilityRouteSchema, TaskID: "task-cont-stall-explicit", GoalID: "goal-cont-stall-explicit",
		RunID: "run-cont-stall-explicit", ConversationID: contStallConversation,
		OriginalIntent: "把低音轨/bass 提 1dB",
		Controller:     string(orchestrationcontroller.DirectTypedAction),
		SemanticEntry: map[string]any{
			"route": semanticEntryRouteExplicitControl, "target_scope": semanticEntryScopeProjectContext,
			"control_mode": semanticEntryControlTyped, "user_authorization": semanticEntryAuthorizationAction,
			"confidence": 0.9,
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	out := contextWithCapabilityRoute(base, identity)
	if out[capacityAssessmentContextKey] != nil || out[capabilityEntryPlanContextKey] != nil {
		t.Fatal("the assessment-less identity inherited the previous route's capacity state: the index can never hold a validated route for this task, so the armed checkpoint would fail-close on every reload — the CONT-STALL-2 dead park")
	}
	if out[capabilityRouteContextKey] == nil {
		t.Fatal("the new routing identity must be present after the swap")
	}

	// 反向锁定：带 assessment 的身份仍必须播下 capacity state（同 task 的合法
	// 续跑不得被本修法饿死）。
	again := contextWithCapabilityRoute(out, first)
	if again[capacityAssessmentContextKey] == nil {
		t.Fatal("an identity with an assessment must still seed the capacity state")
	}
}

// 钉③-C（外来身份注入钉，2026-09-14 09:41 真栈同型复现形态）：continuation 的
// context 带着别的 task 的已验证路由（bindActiveOrchestrationController 的 owner
// 恢复与 refresh 的会话级 fallback 都会把会话最新路由种进新请求 context，新 goal
// 的 durable 于是驮着外来 assessment），而索引中没有本 task 的路由——reload 不得
// 把它 fail-closed 成永久 waiting_interaction：外来身份的 capacity state 不算本
// 记录的，检查点必须保持 pending 可 claim。
func TestContStallForeignRouteIdentityDoesNotStrandContinuation(t *testing.T) {
	s := testContinuationServer()
	defer s.Close()
	s.schedulerWake = nil
	s.activeWorkspaceUUID = "vitproj-cont-stall"
	s.conversationGoals[contStallConversation] = "goal-cont-stall-second"
	// 索引里只有第一 goal 的已验证路由（真实形态：capability_routes 只含
	// task-first，第二 goal 的 task-second 无路由可查）。
	s.mu.Lock()
	s.capabilityRoutes = map[string]CapabilityRouteRecord{"task-cont-stall-first": contStallFirstGoalValidatedRoute()}
	s.mu.Unlock()

	// 第二 goal 的切片信封：durable 顶层的 task 是自己的，context 却驮着第一
	// goal 的路由身份 + assessment（2026-09-14 09:41 run 20260914_094126 的
	// cont_52e645 逐字段形态）。
	intent := "对低音轨做一次混音改进实验，改完让我 A/B 试听对比一下"
	res := agentloop.Result{
		GoalID: "goal-cont-stall-second", RunID: "run-cont-stall-second", TaskID: "task-cont-stall-second",
		SliceID: "slice-cont-stall-second-1", TurnID: "turn-cont-stall-second-1",
		OriginalIntent: intent, Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-cont-stall-second", RunID: "run-cont-stall-second", TaskID: "task-cont-stall-second",
			SliceID: "slice-cont-stall-second-1", TurnID: "turn-cont-stall-second-1",
			OriginalIntent: intent, UserText: intent, Summary: intent,
			Context: contextWithCapabilityRoute(map[string]any{
				"conversation_id": contStallConversation,
				"goal_id":         "goal-cont-stall-second",
				"run_id":          "run-cont-stall-second",
				"task_id":         "task-cont-stall-second",
			}, contStallFirstGoalValidatedRoute()),
		},
	}
	if err := s.recordGoalResult(contStallConversation, res); err != nil {
		t.Fatalf("arming the second goal's checkpoint failed: %v", err)
	}
	armed := contStallDurableStatuses(s)
	for _, item := range armed {
		if item.TaskID != "task-cont-stall-second" {
			continue
		}
		if item.CapacityAssessment == nil {
			t.Fatalf("setup %s: the injected route must have lifted a foreign assessment into the durable record (the real-stack shape)", item.ContinuationID)
		}
	}

	snapshot := s.projectAgentRuntimeStateLocked()
	s.restoreProjectAgentRuntimeStateLocked(snapshot)
	for _, item := range contStallDurableStatuses(s) {
		if item.TaskID != "task-cont-stall-second" {
			continue
		}
		if item.Status != ContinuationPending {
			t.Fatalf("reload fail-closed a checkpoint whose capacity state is foreign (carried by another task's route) to %q (reason=%v): the CONT-STALL-2 dead park",
				item.Status, item.PendingInteraction)
		}
	}
	claimed, ok := s.claimNextContinuation(time.Now().UTC())
	if !ok || claimed.GoalID != "goal-cont-stall-second" {
		t.Fatalf("the second goal's checkpoint was not claimable after reload: ok=%v goal=%s", ok, claimed.GoalID)
	}
}
