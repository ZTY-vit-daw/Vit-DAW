package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
)

// B6（2026-09-11 手测点 3 尝试 #2 用户质问三连）：原生微调（mix-tick/D1 域）
// 终局用户交付的三组钉——①五要素+术语黑名单；②判定一致性（完成态终局
// 禁用「人工判定仍待完成」表述；可应答 waiting park 的试听等待不属于待
// 判定空头承诺，不得误伤）；③mix_tick.pending display 按权限模式分支。

func b6PanAppliedLoop() freeStateReasoningLoop {
	return freeStateReasoningLoop{
		SchemaVersion:   freeStateReasoningLoopSchema,
		LoopID:          "free_state_b6",
		ConversationID:  "conversation-b6",
		GoalID:          "goal-b6",
		RunID:           "run-b6",
		OriginalIntent:  "请检查当前工程是否有什么问题吗",
		Status:          "re_evaluating",
		DecisionPhase:   freeStatePhasePostActionEvaluation,
		RequiresPostActionObservation: false,
		Experiment: &experiment.Turn{
			ID: "turn_b6", Status: experiment.StatusRunning,
			OriginalIntent: "请检查当前工程是否有什么问题吗",
			Admission: experiment.Admission{
				TargetRef:   map[string]any{"kind": "track", "id": "1017", "label": "guitar"},
				EvidenceRefs: []string{"obs_b6_seed"},
				Hypothesis:  "吉他轨立体声相关性 0.23 偏高，尝试小幅居中微调",
				TypedAction: map[string]any{"action_domain": d1PanDomain, "action_kind": d1PanKind, "delta_pan": 0.05},
			},
		},
	}
}

func b6AssertNoInternalTerms(t *testing.T, text string) {
	t.Helper()
	for _, poison := range []string{"FAM", "FS0", "FS1", "D1-S1", "D2-1", "D2-2", "DAD", "DOM", "MOM", "TIM", "TOM", "FXM", "COM", "EPM", "RLM", "CCB", "PCA", "VSP"} {
		if strings.Contains(text, poison) {
			t.Fatalf("user-facing terminal text must not leak the internal term %q: %q", poison, text)
		}
	}
}

// 钉①-a：pan 域应用回复五要素齐备（目标轨可辨名/参数/变更量与方向含回读/
// 针对的发现/去哪看），且无内部术语、无待判定承诺。
func TestD1AppliedReportCarriesFiveElementsPan(t *testing.T) {
	loop := b6PanAppliedLoop()
	receipt := map[string]any{
		"status":              "applied",
		"before_readback_pan": 0.0,
		"actual_readback_pan": 0.050000000745058,
		"readback_verified":   true,
	}
	reply := d1AppliedReportReply(loop, receipt)
	if strings.TrimSpace(reply) == "" {
		t.Fatal("applied report must not be empty")
	}
	if !strings.Contains(reply, "Track 1017") || !strings.Contains(reply, "guitar") {
		t.Fatalf("applied report must name the target track in user-recognizable form: %q", reply)
	}
	if !strings.Contains(reply, "声像") {
		t.Fatalf("applied report must name the parameter (pan/声像): %q", reply)
	}
	if !strings.Contains(reply, "+0.05") {
		t.Fatalf("applied report must state the signed change amount and direction: %q", reply)
	}
	if !strings.Contains(reply, "回读") {
		t.Fatalf("applied report must state the readback before/after values: %q", reply)
	}
	if !strings.Contains(reply, "针对的发现") || !strings.Contains(reply, "相关性") {
		t.Fatalf("applied report must state the finding it targets: %q", reply)
	}
	if !strings.Contains(reply, "去哪看") || !strings.Contains(reply, "通道条") {
		t.Fatalf("applied report must tell the user where to look in the DAW: %q", reply)
	}
	b6AssertNoInternalTerms(t, reply)
	if strings.Contains(reply, "人工判定") {
		t.Fatalf("applied report must not promise a pending human judgment: %q", reply)
	}
}

// 钉①-b：track_gain 域与未知 action_kind 的回退都保五要素（参数名按域表
// 措辞、目标轨无 label 时退 Track <id>）。
func TestD1AppliedReportGainDomainAndUnknownKindFallback(t *testing.T) {
	loop := b6PanAppliedLoop()
	loop.Experiment.Admission = experiment.Admission{
		TargetRef:   map[string]any{"kind": "track", "id": "42"},
		EvidenceRefs: []string{"obs_b6_gain"},
		Hypothesis:  "底鼓电平略高于参考轨",
		TypedAction: map[string]any{"action_domain": experiment.D1S1ActionDomain, "action_kind": experiment.D1S1ActionKind, "delta_db": 0.8},
	}
	reply := d1AppliedReportReply(loop, map[string]any{
		"status": "applied", "before_readback_db": -6.2, "actual_readback_db": -5.4, "readback_verified": true,
	})
	for _, needle := range []string{"Track 42", "音量", "+0.8 dB", "针对的发现", "去哪看"} {
		if !strings.Contains(reply, needle) {
			t.Fatalf("gain applied report must carry %q: %q", needle, reply)
		}
	}
	if strings.Contains(reply, "（") && strings.Count(reply, "Track 42（") > 0 {
		t.Fatalf("track without a label must not render an empty label paren: %q", reply)
	}

	unknown := b6PanAppliedLoop()
	unknown.Experiment.Admission.TypedAction = map[string]any{"action_domain": "mystery", "action_kind": "mystery_kind", "delta_db": 0.5}
	unknown.Experiment.Admission.Hypothesis = ""
	unknown.Experiment.Admission.ExpectedEffect = "整体听感更平衡"
	fallback := d1AppliedReportReply(unknown, map[string]any{"status": "applied"})
	for _, needle := range []string{"Track 1017", "针对的发现", "去哪看"} {
		if !strings.Contains(fallback, needle) {
			t.Fatalf("unknown-kind applied report must still carry the five elements (%q missing): %q", needle, fallback)
		}
	}
	b6AssertNoInternalTerms(t, fallback)
}

// 钉①-c：术语黑名单护栏对既有模板文本的剥除（FAM/FS/D 阶段编号、投影缩写），
// 干净文本原样通过。
func TestStripInternalTerminalTerms(t *testing.T) {
	legacy := "FAM3-S1 轨道声像参数已应用并回读验证。"
	out := stripInternalTerminalTerms(legacy)
	if strings.Contains(out, "FAM3") {
		t.Fatalf("FAM code must be stripped: %q", out)
	}
	if !strings.Contains(out, "轨道声像参数已应用并回读验证") {
		t.Fatalf("strip must keep the informative wording: %q", out)
	}
	for _, poison := range []string{"FAM1-S1 齿音", "FS0 阶段", "D1-S1 增益", "D2-1.5 压缩", "CCB 观察", "DOM 投影", "MOM 投影", "PCA 门", "VSP 回执"} {
		if stripped := stripInternalTerminalTerms(poison + " 已记录"); strings.Contains(stripped, strings.Fields(poison)[0]) {
			t.Fatalf("internal term %q must be stripped, got %q", poison, stripped)
		}
	}
	clean := "实验调整已应用并完成观测评估，等待用户试听确认。"
	if stripped := stripInternalTerminalTerms(clean); stripped != clean {
		t.Fatalf("clean wording must pass unchanged: %q", stripped)
	}
}

// 钉②-a：完成态终局回复剥除「人工判定仍待完成」类承诺（取证 seq22 原文
// 整段进、剥后仍保留应用事实），可应答 waiting park 的试听等待措辞不受影响。
func TestSettledTerminalDropsPendingJudgmentClaims(t *testing.T) {
	legacy := "FAM3-S1 轨道声像参数已应用并回读验证。动作后新证据已单独记录；声学实质性、目标响应与人工判定仍待完成。"
	out := sanitizeSettledTerminalReply(legacy)
	if strings.Contains(out, "人工判定") {
		t.Fatalf("a settled terminal must not claim a pending human judgment: %q", out)
	}
	if !strings.Contains(out, "已应用并回读验证") || !strings.Contains(out, "动作后新证据已单独记录") {
		t.Fatalf("the judgment-claim strip must keep the applied facts: %q", out)
	}
	b6AssertNoInternalTerms(t, out)

	park := "实验调整已应用并完成观测评估，等待用户试听确认。"
	if stripped := stripPendingHumanJudgmentClaims(park); stripped != park {
		t.Fatalf("an answerable audition park reply is a servicable judgment entry, not a hollow claim: %q", stripped)
	}
}

// 钉②-b（wiring）：调度链完成态终局的事件 body 与落图节点都过护栏——取证
// seq22 形态（链末切片带 FAM3-S1 模板回复、goal 已 completed）整段进入
// settleAndDeliverContinuationChainEnd 后，事件与水合文本都不再含内部术语
// 与待判定承诺，且报告事实保留、投递不回退。
func TestSettledSchedulerTerminalSanitizedOnEventAndNode(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-b6", "run-b6", "improve the mix")
	s.harness.SetGoalStatus("goal-b6", agentruntime.StatusCompleted, nil)

	settled := DurableContinuation{
		ContinuationID: "cont_b6_settled", GoalID: "goal-b6", RunID: "run-b6",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationCompleted,
	}
	s.settleAndDeliverContinuationChainEnd(context.Background(), settled, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-b6", RunID: "run-b6",
		GoalStatus: string(agentruntime.StatusCompleted),
		StopReason: "d1_post_action_evaluation_required",
		Reply:      "FAM3-S1 轨道声像参数已应用并回读验证。动作后新证据已单独记录；声学实质性、目标响应与人工判定仍待完成。",
	}, true)

	events, _ := s.agentEventsSince("conversation-f2", 0, 64)
	var delivered *AgentEvent
	for index := range events {
		event := events[index]
		if event.Type != "turn.completed" || event.ItemID != "chain_result" {
			continue
		}
		delivered = &event
		break
	}
	if delivered == nil {
		t.Fatalf("settled chain end must still deliver the scheduler terminal: %+v", events)
	}
	if !strings.Contains(delivered.Body, "已应用并回读验证") {
		t.Fatalf("sanitized terminal must keep the applied facts: %q", delivered.Body)
	}
	if strings.Contains(delivered.Body, "FAM3") || strings.Contains(delivered.Body, "人工判定") {
		t.Fatalf("sanitized terminal must drop internal terms and judgment claims: %q", delivered.Body)
	}
	messages := schedulerTerminalHydratedMessages(t, s, projectPath)
	sawSanitizedNode := false
	for _, message := range messages {
		if strings.Contains(message.Content, "已应用并回读验证") {
			sawSanitizedNode = true
			if strings.Contains(message.Content, "人工判定") || strings.Contains(message.Content, "FAM3") {
				t.Fatalf("persisted terminal node must be sanitized too: %q", message.Content)
			}
		}
	}
	if !sawSanitizedNode {
		t.Fatalf("terminal node must land in the conversation graph: %+v", messages)
	}
}

// 钉③：mix_tick.pending 的 display 文案按权限模式分支——manual 承诺等待确
// 认；完全访问（自动应用）下不得承诺「等待你确认/确认前不会修改工程」，
// 改述为直接应用，title/body/display 同步分支。
func TestPendingMixTickDisplayBranchesByAuthority(t *testing.T) {
	candidate := agentloop.PendingMixTickCandidate{
		Operation: "track_pan_adjust", TrackID: "1017", DeltaPan: 0.05,
		Status: "pending_confirmation", ObservationID: "obs_b6",
	}
	manualBody := pendingMixTickEventBody(candidate)
	if !strings.Contains(manualBody, "正在等待你确认") || !strings.Contains(manualBody, "确认前不会修改工程") {
		t.Fatalf("manual-mode pending body must keep the confirmation promise: %q", manualBody)
	}
	autoBody := pendingMixTickAutoApplyEventBody(candidate)
	if strings.Contains(autoBody, "等待你确认") || strings.Contains(autoBody, "确认前不会修改工程") {
		t.Fatalf("full-access pending body must not promise a confirmation wait: %q", autoBody)
	}
	if !strings.Contains(autoBody, "完全访问") || !strings.Contains(autoBody, "Track 1017") {
		t.Fatalf("full-access pending body must state the direct-apply mode and the target: %q", autoBody)
	}

	manualPayload := pendingMixTickEventPayloadForMode(candidate, "obs_b6", false)
	autoPayload := pendingMixTickEventPayloadForMode(candidate, "obs_b6", true)
	manualDisplay, _ := manualPayload["display"].(map[string]any)
	autoDisplay, _ := autoPayload["display"].(map[string]any)
	if manualDisplay == nil || autoDisplay == nil {
		t.Fatalf("both modes must render a display block: %+v %+v", manualPayload, autoPayload)
	}
	if body, _ := manualDisplay["body"].(string); !strings.Contains(body, "正在等待你确认") {
		t.Fatalf("manual display body must keep the confirmation promise: %+v", manualDisplay)
	}
	if body, _ := autoDisplay["body"].(string); strings.Contains(body, "等待你确认") {
		t.Fatalf("full-access display body must not promise a confirmation wait: %+v", autoDisplay)
	}
	if title, _ := autoDisplay["title"].(string); !strings.Contains(title, "直接") {
		t.Fatalf("full-access display title must signal direct apply: %+v", autoDisplay)
	}
	if title, _ := manualDisplay["title"].(string); !strings.Contains(title, "待确认") {
		t.Fatalf("manual display title must keep the pending-confirmation wording: %+v", manualDisplay)
	}
}

// 钉③（wiring）：storePendingMixTickCandidateForMode 的事件发射按模式分
// 支（完全访问=直接应用文案；默认=待确认文案），事件标题与正文一致分支。
func TestStorePendingMixTickCandidateDisplayModeWiring(t *testing.T) {
	candidate := agentloop.PendingMixTickCandidate{
		Operation: "track_pan_adjust", TrackID: "1017", DeltaPan: 0.05,
		Status: "pending_confirmation", ObservationID: "obs_b6",
	}
	s := testContinuationServer()

	s.storePendingMixTickCandidateForMode("conversation-b6", "goal-b6", "run-b6", candidate, true)
	events, _ := s.agentEventsSince("conversation-b6", 0, 64)
	autoEvent := b6FindMixTickPending(events)
	if autoEvent == nil {
		t.Fatal("full-access store must still emit mix_tick.pending")
	}
	if strings.Contains(autoEvent.Body, "等待你确认") || strings.Contains(autoEvent.Title, "待确认") {
		t.Fatalf("full-access pending event must use the direct-apply wording: title=%q body=%q", autoEvent.Title, autoEvent.Body)
	}
	display, _ := autoEvent.Payload["display"].(map[string]any)
	if body, _ := display["body"].(string); display != nil && strings.Contains(body, "等待你确认") {
		t.Fatalf("full-access pending payload display must not promise a confirmation wait: %+v", display)
	}

	s.storePendingMixTickCandidateForMode("conversation-b6", "goal-b6", "run-b6", candidate, false)
	events, _ = s.agentEventsSince("conversation-b6", 0, 64)
	var manualEvent *AgentEvent
	for index := range events {
		event := events[index]
		if event.Type == "mix_tick.pending" && strings.Contains(event.Body, "正在等待你确认") {
			manualEvent = &event
			break
		}
	}
	if manualEvent == nil {
		t.Fatalf("manual-mode store must keep the confirmation-promise wording: %+v", events)
	}
}

// 钉③（goalrunner 投影面）：伴随改善提案的候选在完全访问下走直接应用文
// 案（不承诺等待确认）；无提案的普通候选即使完全访问也保持待确认文案
// （它真的在等用户的明确执行指令）。
func TestGoalRunnerPendingProjectionBranchesWithProposal(t *testing.T) {
	candidate := &agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "1007", DeltaDB: 1.5,
		Status: "pending_confirmation", ObservationID: "obs_b6_goal",
	}
	proposal := &agentprotocol.ImprovementProposal{}
	s := testContinuationServer()
	s.mu.Lock()
	s.authorityMode = authorityModeFull
	s.conversationGoals = map[string]string{}
	s.conversationMemory = map[string]agentloop.ExecutionMemory{}
	s.pendingMixTicks = map[string]agentloop.PendingMixTickCandidate{}
	s.mu.Unlock()

	_ = s.recordGoalResult("conversation-b6-goal", agentloop.Result{
		GoalID: "goal-b6-goal", RunID: "run-b6-goal",
		FreeStateDecision: &agentloop.FreeStateDecision{ImprovementProposal: proposal},
		ExecutionMemory:   agentloop.ExecutionMemory{PendingMixTickCandidate: candidate},
	})
	events, _ := s.agentEventsSince("conversation-b6-goal", 0, 64)
	autoEvent := b6FindMixTickPending(events)
	if autoEvent == nil {
		t.Fatal("goalrunner projection must emit mix_tick.pending")
	}
	if strings.Contains(autoEvent.Body, "等待你确认") {
		t.Fatalf("proposal-borne candidate under full access must use the direct-apply wording: %q", autoEvent.Body)
	}
	if !strings.Contains(autoEvent.Body, "完全访问") {
		t.Fatalf("direct-apply wording must name the mode: %q", autoEvent.Body)
	}

	_ = s.recordGoalResult("conversation-b6-plain", agentloop.Result{
		GoalID: "goal-b6-plain", RunID: "run-b6-plain",
		ExecutionMemory: agentloop.ExecutionMemory{PendingMixTickCandidate: candidate},
	})
	events, _ = s.agentEventsSince("conversation-b6-plain", 0, 64)
	plainEvent := b6FindMixTickPending(events)
	if plainEvent == nil {
		t.Fatal("plain candidate projection must emit mix_tick.pending")
	}
	if !strings.Contains(plainEvent.Body, "正在等待你确认") {
		t.Fatalf("plain mix-tick candidate keeps the confirmation wait even under full access: %q", plainEvent.Body)
	}
}

func b6FindMixTickPending(events []AgentEvent) *AgentEvent {
	for index := range events {
		event := events[index]
		if event.Type == "mix_tick.pending" {
			return &event
		}
	}
	return nil
}

// 终局节点持久化路径的守恒：sanitized 前后 recordSchedulerChainTerminalNode
// 仍按原语义落图（防 护栏改动把空回复误判为不落图）。
func TestSanitizeKeepsNonEmptyReplyLanding(t *testing.T) {
	legacy := "FAM3-S1 轨道声像参数已应用并回读验证。动作后新证据已单独记录；声学实质性、目标响应与人工判定仍待完成。"
	out := sanitizeSettledTerminalReply(legacy)
	if strings.TrimSpace(out) == "" {
		t.Fatalf("sanitize must never blank a non-empty terminal reply: %q", out)
	}
	if reply := sanitizeSettledTerminalReply("   "); strings.TrimSpace(reply) != "" {
		t.Fatalf("whitespace-only reply stays whitespace-only: %q", reply)
	}
}
