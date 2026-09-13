package chat

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FALLBACK-2 单测钉（阶段 A：RED）——**只用既有 API** 表达断言，先在未改动的
// schedulerChainFailureResponse 上跑出真实断言失败，再实现分级。
//
// 现场逐字（FALLBACK-1 回执 §4 工件 B/C/D，本卡不重查）：
//   - conversation_graph.json 两条 vit 节点正文最后一行 = 「失败详情：failed」；
//   - agent_runtime_state.json：loop.last_error = 决策层完整句、
//     minimal_audio_closures[…].events[17].settlement = {reason:"failed",
//     summary:<同一句>}、durable_continuations[*].last_error =
//     "durable continuation failed: failed"。
//
// 终局三态（决策侧裁定口径）：
//
//	① JSON 解不开        = 模型协议失败（可重试措辞）
//	② 门拒绝 G4/G5/G6/G8 = 决定不可采（gap 人话 + 不是执行故障 + 下一步指引）
//	③ 真执行失败         = 既有 F5 语义逐字不变
const (
	// fallbackDecisionLayerSentence 是 agentloop 终局 fallback 的完整英文句
	// （free_state_reasoning.go:898 的 r.fail 文案），三处原文逐字同源。
	fallbackDecisionLayerSentence = "terminal turn produced no admissible final decision after one strengthened retry"
	// fallbackSliceReply 是 912.vit 落图节点里「这一步收到的汇报」逐字原文。
	fallbackSliceReply = "任务在形成有效结算前失败；失败原因与已有证据已保留。"
	// bareEnumFailureBody 是缺陷本体：状态枚举字面量冒充用户面失败详情。
	bareEnumFailureBody = "失败详情：failed"
	// 阶段 A 用字面量键构造判别输入；生产侧同名常量在实现后由
	// TestChainFailureDecisionReceiptKeysAreStable 钉住两端一致。
	familyRefusedLiteral  = "proposal_not_admitted"
	familyUnparsedLiteral = "terminal_output_unparsed"
	receiptKeyLiteral     = "chain_failure_decision"
)

// fallbackFreeStateLoopFixture 复刻 912.vit goal_303a5ad8002ebe6c 的终局循环：
// 终端轮已锁定、结构化门拒绝 gap 在案（G5+G6+G8）、admission_receipt 记 G7/G8。
func fallbackFreeStateLoopFixture() freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "free_state_fallback2_test",
		ConversationID: "conversation-fallback2",
		GoalID:         "goal_303a5ad8002ebe6c",
		RunID:          "run_ea4ad10fbb3a3f99",
		Status:         "blocked",
		OriginalIntent: "检查当前工程有什么问题",
		// 终局轮锁定 + 锁定原因（agentloop 侧写入，chat 只读）
		TerminalTurnLocked: true,
		TerminalTurnReason: "budget_critical",
		// 决策层原文（工件 B 三处之一）
		LastError: fallbackDecisionLayerSentence,
		// TIMING-1 结构化 gap：门拒绝的 condition/binding 原文
		AdmissionRejectionCount: 1,
		AdmissionRejectionGaps: []map[string]any{{
			"schema_version":  "free_state_admission_gap.v1",
			"failed_gate_ids": []any{"G5_frontier_established", "G6_target_evidence", "G8_target_consistency"},
			"missing": []any{
				map[string]any{"gate_id": "G5_frontier_established", "condition": "no_frontier_candidates", "binding": "minimal_audio_closure.hypothesis_frontier.candidates non-empty"},
				map[string]any{"gate_id": "G6_target_evidence", "condition": "no_selected_candidate", "binding": "a usable target-level observation for the frontier-selected candidate"},
				map[string]any{"gate_id": "G8_target_consistency", "condition": "no_target_own_observation_cited", "binding": "proposal target and evidence refs are consistent with the frontier-selected candidate and its target-level observations"},
			},
		}},
		AdmissionReceipt: map[string]any{
			"status":          "blocked",
			"boundary":        "proposal_missing",
			"failed_gate_ids": []any{"G7_fresh_revision_bound_refs", "G8_target_consistency"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// fallbackChainFailureResponse 复刻调度器在 912.vit 终局看到的那一份响应：
// audioClosureResponse 的终局形状（Error 为空、StopReason 是结算 reason "failed"）
// 经 bindFreeStateContextToResponse 挂上 free_state_reasoning_loop 之后的结果。
func fallbackChainFailureResponse() ChatResponse {
	return fallbackChainFailureResponseFamily("")
}

func fallbackChainFailureResponseFamily(family string) ChatResponse {
	workflowData := map[string]any{
		"status": "settled",
		"settlement": map[string]any{
			"reason": "failed", "summary": fallbackDecisionLayerSentence,
		},
		"free_state_reasoning_loop": freeStateLoopMap(fallbackFreeStateLoopFixture()),
	}
	if family != "" {
		// detail 是 agentloop fallback 自己的短标签（message_loop.go:633 的
		// detail 参数）；决策层完整句在 settlement.summary / loop.last_error 上。
		workflowData[receiptKeyLiteral] = map[string]any{
			"schema_version": "scheduler_chain_failure_decision.v1",
			"family":         family,
			"detail":         "no admissible final decision after one strengthened retry",
		}
	}
	return ChatResponse{
		ConversationID: "conversation-fallback2",
		GoalID:         "goal_303a5ad8002ebe6c",
		RunID:          "run_ea4ad10fbb3a3f99",
		GoalStatus:     string(agentruntime.StatusFailed),
		StopReason:     "failed",
		Workflow:       "minimal_audio_closure",
		Reply:          fallbackSliceReply,
		WorkflowData:   workflowData,
	}
}

func fallbackChainFailureContinuation() DurableContinuation {
	return DurableContinuation{
		ContinuationID: "cont_303a5ad8_fallback2",
		GoalID:         "goal_303a5ad8002ebe6c",
		RunID:          "run_ea4ad10fbb3a3f99",
		ConversationID: "conversation-fallback2",
		OriginalIntent: "检查当前工程有什么问题",
		Status:         ContinuationFailed,
		// 现状：调度器把结构化 detail 压成 Go error 后落到 durable.LastError
		LastError: "durable continuation failed: failed",
	}
}

func fallbackFailureErr() error {
	return errors.New("durable continuation failed: failed")
}

// 钉 ①：门拒绝终局不得渲染成执行器故障。用户面必须给出 gap 人话、明确「不是
// 执行故障」、给出下一步指引，且不出现枚举字面量裸串。
func TestChainFailureTerminalGradesAdmissionRefusalAsDecisionNotExecution(t *testing.T) {
	resp := schedulerChainFailureResponse(fallbackChainFailureContinuation(),
		fallbackChainFailureResponseFamily(familyRefusedLiteral), fallbackFailureErr())
	body := resp.Reply
	// 修后用户面正文（回执 §3 引用；-v 可见）。
	t.Logf("graded refusal body: %q", body)

	if !strings.Contains(body, "没有被采纳") {
		t.Fatalf("a gate-refused terminal must say the proposal was not adopted: %q", body)
	}
	if !strings.Contains(body, "前沿") {
		t.Fatalf("the refusal must carry the structured gap in human words (G5 frontier): %q", body)
	}
	if !strings.Contains(body, "这不是执行故障") {
		t.Fatalf("a gate-refused terminal must not be framed as an execution failure: %q", body)
	}
	if !strings.Contains(body, "需要更多观察") {
		t.Fatalf("the refusal must carry next-step guidance: %q", body)
	}
	if strings.Contains(body, schedulerChainFailureLead) {
		t.Fatalf("the execution-failure lead must not head a decision refusal: %q", body)
	}
	if strings.Contains(body, bareEnumFailureBody) {
		t.Fatalf("the bare status enum must never be the user-facing failure detail: %q", body)
	}

	// 第二条真实终局（912.vit goal_7f3d41a3cbe7a307）：聚合 gap 是
	// G4_dimension_closed / no_closed_diagnostic_dimension，admission_receipt 另记
	// G7+G8。两条现场必须走同一条分级路径，gap 人话取自聚合 gap 原文。
	loop := fallbackFreeStateLoopFixture()
	loop.AdmissionRejectionGaps = []map[string]any{{
		"schema_version":  "free_state_admission_gap.v1",
		"failed_gate_ids": []any{"G4_dimension_closed"},
		"missing": []any{
			map[string]any{"gate_id": "G4_dimension_closed", "condition": "no_closed_diagnostic_dimension", "binding": "a diagnostic round with evidence_status=ready and no open unresolved_questions"},
		},
	}}
	loop.AdmissionReceipt = map[string]any{"failed_gate_ids": []any{"G7_fresh_revision_bound_refs", "G8_target_consistency"}}
	second := fallbackChainFailureResponseFamily(familyRefusedLiteral)
	second.WorkflowData["free_state_reasoning_loop"] = freeStateLoopMap(loop)
	secondBody := schedulerChainFailureResponse(fallbackChainFailureContinuation(), second, fallbackFailureErr()).Reply
	if !strings.Contains(secondBody, "诊断维度") {
		t.Fatalf("the archived G4 refusal must reach the user in human words: %q", secondBody)
	}
	if strings.Contains(secondBody, "前沿") {
		t.Fatalf("the G4 refusal must not borrow the G5 frontier wording: %q", secondBody)
	}
	if !strings.Contains(secondBody, "这不是执行故障") || !strings.Contains(secondBody, "需要更多观察") {
		t.Fatalf("the G4 refusal must stay in the decision-not-execution grade: %q", secondBody)
	}
	if strings.Contains(secondBody, schedulerChainFailureLead) || strings.Contains(secondBody, bareEnumFailureBody) {
		t.Fatalf("the G4 refusal must not read as an execution failure: %q", secondBody)
	}

	// 载荷错误位保留完整决策层原文（用户面被分级，内部原文一字不丢）。
	if !strings.Contains(resp.Error, fallbackDecisionLayerSentence) {
		t.Fatalf("the payload error must keep the decision layer's complete text: %q", resp.Error)
	}
}

// 钉 ②：JSON 解不开（真协议失败）与门拒绝（决定不可采）必须可区分。
func TestChainFailureTerminalGradesProtocolFailureDistinctFromRefusal(t *testing.T) {
	refusal := schedulerChainFailureResponse(fallbackChainFailureContinuation(),
		fallbackChainFailureResponseFamily(familyRefusedLiteral), fallbackFailureErr()).Reply
	unparsed := schedulerChainFailureResponse(fallbackChainFailureContinuation(),
		fallbackChainFailureResponseFamily(familyUnparsedLiteral), fallbackFailureErr()).Reply

	t.Logf("graded protocol-failure body: %q", unparsed)
	if unparsed == refusal {
		t.Fatalf("a protocol failure and a decision refusal must not share one user-facing body: %q", refusal)
	}
	if !strings.Contains(refusal, "没有被采纳") {
		t.Fatalf("refusal body missing the not-adopted wording: %q", refusal)
	}
	if strings.Contains(refusal, "输出格式无法解析") {
		t.Fatalf("the refusal body must not read as a format/protocol failure: %q", refusal)
	}
	if !strings.Contains(unparsed, "输出格式无法解析") {
		t.Fatalf("the protocol-failure body must name the unparseable output: %q", unparsed)
	}
	if !strings.Contains(unparsed, "可以重试") {
		t.Fatalf("the protocol-failure body must offer the retry: %q", unparsed)
	}
	if strings.Contains(unparsed, "没有被采纳") {
		t.Fatalf("the protocol-failure body must not claim the proposal was refused: %q", unparsed)
	}
	if strings.Contains(unparsed, schedulerChainFailureLead) {
		t.Fatalf("the execution-failure lead must not head a protocol failure: %q", unparsed)
	}
}

// 钉 ③：真执行失败维持既有 F5 语义逐字不变。
func TestChainFailureTerminalKeepsF5ExecutionFailureVerbatim(t *testing.T) {
	sliceReply := "本片已完成两轮观察。"
	executionDetail := "dsp_stage_write_timeout"
	resp := schedulerChainFailureResponse(DurableContinuation{
		ContinuationID: "cont_f5_verbatim", GoalID: "goal-f5v", RunID: "run-f5v", ConversationID: "conversation-f5v",
	}, ChatResponse{
		ConversationID: "conversation-f5v", GoalID: "goal-f5v", RunID: "run-f5v",
		GoalStatus: string(agentruntime.StatusFailed), StopReason: executionDetail, Reply: sliceReply,
	}, errors.New("dsp stage write timed out"))

	want := schedulerChainFailureReply(sliceReply, executionDetail)
	if resp.Reply != want {
		t.Fatalf("a real execution failure body must stay byte-identical to F5's: got %q want %q", resp.Reply, want)
	}
	if !strings.Contains(resp.Reply, schedulerChainFailureLead) {
		t.Fatalf("the F5 execution-failure lead must stay in place: %q", resp.Reply)
	}
	if !strings.Contains(resp.Reply, executionDetail) {
		t.Fatalf("the F5 execution failure detail must stay in place: %q", resp.Reply)
	}

	// 同族守卫：自由态循环在场但没有终局 fallback 判别回执时，链仍然是普通执行
	// 失败——判别权归回执，不归「循环在场」。
	withLoop := fallbackChainFailureResponse()
	withLoop.Reply = sliceReply
	withLoop.Error = "mix tick apply failed: kernel write timeout"
	withLoop.StopReason = ""
	withLoop.GoalStatus = string(agentruntime.StatusFailed)
	withLoopFailure := schedulerChainFailureResponse(fallbackChainFailureContinuation(), withLoop, errors.New("mix tick apply failed: kernel write timeout"))
	if strings.Contains(withLoopFailure.Reply, "没有被采纳") {
		t.Fatalf("an execution failure inside a free-state chain must not be graded as a decision refusal: %q", withLoopFailure.Reply)
	}
	if !strings.Contains(withLoopFailure.Reply, schedulerChainFailureLead) {
		t.Fatalf("an execution failure inside a free-state chain keeps the F5 lead: %q", withLoopFailure.Reply)
	}
	if !strings.Contains(withLoopFailure.Reply, "kernel write timeout") {
		t.Fatalf("an execution failure inside a free-state chain keeps its real detail: %q", withLoopFailure.Reply)
	}
}

// 钉 ④：状态枚举字面量不得充当用户面失败详情；兜底必须是中文状态词。
func TestChainFailureTerminalNeverUsesBareStatusEnumAsDetail(t *testing.T) {
	// (a) 取值链终点：只有结算状态可用（Error 空、StopReason 就是枚举值本身）
	enumOnly := schedulerChainFailureResponse(fallbackChainFailureContinuation(), ChatResponse{
		ConversationID: "conversation-fallback2", GoalID: "goal_303a5ad8002ebe6c", RunID: "run_ea4ad10fbb3a3f99",
		GoalStatus: string(agentruntime.StatusFailed), StopReason: "failed", Reply: fallbackSliceReply,
	}, fallbackFailureErr())
	if strings.Contains(enumOnly.Reply, bareEnumFailureBody) {
		t.Fatalf("a bare status enum must never be the user-facing failure detail: %q", enumOnly.Reply)
	}
	if !strings.Contains(enumOnly.Reply, "失败详情：") {
		t.Fatalf("the failure detail line must survive the enum guard: %q", enumOnly.Reply)
	}
	if !strings.Contains(enumOnly.Reply, schedulerChainFailureDetailFallbackStatus) {
		t.Fatalf("the enum fallback must be a Chinese status word: %q", enumOnly.Reply)
	}

	terminals := map[string]ChatResponse{
		"gate-refusal": schedulerChainFailureResponse(fallbackChainFailureContinuation(),
			fallbackChainFailureResponseFamily(familyRefusedLiteral), fallbackFailureErr()),
		"protocol-failure": schedulerChainFailureResponse(fallbackChainFailureContinuation(),
			fallbackChainFailureResponseFamily(familyUnparsedLiteral), fallbackFailureErr()),
		"enum-only": enumOnly,
		"executor-error": schedulerChainFailureResponse(fallbackChainFailureContinuation(), ChatResponse{
			ConversationID: "conversation-fallback2", GoalID: "goal_303a5ad8002ebe6c", RunID: "run_ea4ad10fbb3a3f99",
		}, errors.New("executor unavailable")),
	}
	for name, resp := range terminals {
		if strings.Contains(resp.Reply, bareEnumFailureBody) {
			t.Fatalf("%s terminal must not render the bare enum detail: %q", name, resp.Reply)
		}
		if strings.Contains(resp.Reply, "失败详情：failed") {
			t.Fatalf("%s terminal must not render the bare enum detail: %q", name, resp.Reply)
		}
	}
}

// 钉 ⑤：durable.LastError 保留完整决策层原文（不因分级被截断、不回落成枚举）。
func TestChainFailureReasonKeepsFullDecisionLayerTextForDurableLastError(t *testing.T) {
	resp := fallbackChainFailureResponseFamily(familyRefusedLiteral)
	reason := schedulerChainFailureReason(resp)
	if reason != fallbackDecisionLayerSentence {
		t.Fatalf("the chain failure reason must be the decision layer's complete original text: got %q want %q", reason, fallbackDecisionLayerSentence)
	}
	if schedulerChainStatusEnumLiteral(reason) {
		t.Fatalf("the chain failure reason must never collapse to a status enum: %q", reason)
	}
	// 完整句优先于回执自己的短标签（settlement.summary / loop.last_error 是
	// 承载完整原文的两处既有面）。
	shortLabelOnly := ChatResponse{WorkflowData: map[string]any{
		schedulerChainDecisionReceiptKey: map[string]any{
			"schema_version": schedulerChainDecisionReceiptSchema,
			"family":         schedulerChainDecisionFamilyRefused,
			"detail":         "no admissible final decision after one strengthened retry",
		},
	}}
	if got := schedulerChainFailureReason(shortLabelOnly); got != "no admissible final decision after one strengthened retry" {
		t.Fatalf("without a fuller surface the receipt detail stands in: %q", got)
	}

	// 落盘链路：setContinuationStatus 是 ContinuationFailed 的唯一 durable 写入点，
	// 它把 err.Error() 原样写进 last_error —— 所以完整原文必须活到这一跳。
	s := testContinuationServer()
	s.continuationPersist = func() error { return nil }
	s.durableContinuations["cont_712"] = DurableContinuation{
		ContinuationID: "cont_712", GoalID: "goal_303a5ad8002ebe6c", RunID: "run_ea4ad10fbb3a3f99",
		ConversationID: "conversation-fallback2", Status: ContinuationRunning,
	}
	s.setContinuationStatus("cont_712", ContinuationFailed, fmt.Errorf("durable continuation failed: %s", reason))
	lastError := s.durableContinuations["cont_712"].LastError
	if lastError != "durable continuation failed: "+fallbackDecisionLayerSentence {
		t.Fatalf("durable.LastError must keep the complete decision-layer text: %q", lastError)
	}
	if strings.Contains(lastError, "failed: failed") {
		t.Fatalf("durable.LastError must not collapse to the status enum: %q", lastError)
	}
}

// 钉 ⑥：判别回执从既有的 agentloop 终局 fallback 轨迹派生——两条真实终局不再
// 共用一个停机级；真执行失败与非失败回合一律不产回执（F5 语义零回退的保证）。
func TestSchedulerChainDecisionReceiptReadsTerminalFallbackTrace(t *testing.T) {
	cases := []struct {
		name   string
		detail string
		want   string
	}{
		// message_loop.go:631-633 的终局准入门拒绝族（G4/G5/G6/G8 现场）
		{"admission-refused", "no admissible final decision after one strengthened retry", schedulerChainDecisionFamilyRefused},
		// message_loop.go:544/562 的解析/修复层失败族
		{"repair-unparsed", "terminal turn output was unparseable after the strengthened JSON repair retry", schedulerChainDecisionFamilyUnparsed},
		// message_loop.go:598-600 的严格解析族
		{"strict-unparsed", "terminal turn raw output was not one clean JSON object after one strengthened retry", schedulerChainDecisionFamilyUnparsed},
	}
	for _, tc := range cases {
		bound := bindSchedulerChainDecisionReceipt(ChatResponse{}, agentloop.Result{
			Status: agentruntime.StatusFailed,
			Trace:  []planner.TraceEvent{{Kind: "final_gate", Message: schedulerChainTerminalFallbackTracePrefix + tc.detail}},
		})
		receipt, ok := schedulerChainDecisionReceiptFromResponse(bound)
		if !ok {
			t.Fatalf("%s: the terminal fallback trace must produce a receipt: %+v", tc.name, bound.WorkflowData)
		}
		if receipt.Family != tc.want {
			t.Fatalf("%s: family = %q, want %q", tc.name, receipt.Family, tc.want)
		}
		if receipt.Detail != tc.detail {
			t.Fatalf("%s: receipt must keep the agentloop fallback reason verbatim: %q", tc.name, receipt.Detail)
		}
	}

	// 词表之外的 fallback 措辞不猜：不产回执，退回 F5 文案（宁可少分级，
	// 不可凭空声称「提案没有被采纳」）。
	unknown := bindSchedulerChainDecisionReceipt(ChatResponse{}, agentloop.Result{
		Status: agentruntime.StatusFailed,
		Trace:  []planner.TraceEvent{{Kind: "final_gate", Message: schedulerChainTerminalFallbackTracePrefix + "terminal turn gave up for an unlisted reason"}},
	})
	if len(firstMapFromAny(unknown.WorkflowData[schedulerChainDecisionReceiptKey])) != 0 {
		t.Fatalf("an unrecognized fallback reason must not be graded: %+v", unknown.WorkflowData)
	}

	// 真执行失败（trace 里没有终局 fallback 事件）不产回执
	execution := bindSchedulerChainDecisionReceipt(ChatResponse{}, agentloop.Result{
		Status: agentruntime.StatusFailed,
		Trace:  []planner.TraceEvent{{Kind: "planner_error", Message: "mix tick apply failed: kernel write timeout"}},
	})
	if len(firstMapFromAny(execution.WorkflowData[schedulerChainDecisionReceiptKey])) != 0 {
		t.Fatalf("a real execution failure must not carry a decision receipt: %+v", execution.WorkflowData)
	}
	// 非失败回合即使带 final_gate 轨迹也不分级
	completed := bindSchedulerChainDecisionReceipt(ChatResponse{}, agentloop.Result{
		Status: agentruntime.StatusCompleted,
		Trace:  []planner.TraceEvent{{Kind: "final_gate", Message: schedulerChainTerminalFallbackTracePrefix + "no admissible final decision after one strengthened retry"}},
	})
	if len(firstMapFromAny(completed.WorkflowData[schedulerChainDecisionReceiptKey])) != 0 {
		t.Fatalf("a non-failed turn must not carry a decision receipt: %+v", completed.WorkflowData)
	}
	// 畸形回执不产生分级（fail-safe：退回 F5 文案）
	malformed := ChatResponse{WorkflowData: map[string]any{
		schedulerChainDecisionReceiptKey: map[string]any{"family": "something_else"},
	}}
	if _, ok := schedulerChainDecisionReceiptFromResponse(malformed); ok {
		t.Fatal("an unrecognized receipt family must not grade the terminal")
	}
}

// 阶段 A 的字面量键与生产常量必须一致（两侧都不许悄悄漂移）。
func TestSchedulerChainDecisionReceiptKeysAreStable(t *testing.T) {
	if schedulerChainDecisionReceiptKey != receiptKeyLiteral {
		t.Fatalf("receipt key drift: %q", schedulerChainDecisionReceiptKey)
	}
	if schedulerChainDecisionFamilyRefused != familyRefusedLiteral {
		t.Fatalf("refused family drift: %q", schedulerChainDecisionFamilyRefused)
	}
	if schedulerChainDecisionFamilyUnparsed != familyUnparsedLiteral {
		t.Fatalf("unparsed family drift: %q", schedulerChainDecisionFamilyUnparsed)
	}
	if !schedulerChainStatusEnumLiteral("failed") || !schedulerChainStatusEnumLiteral(string(agentruntime.StatusFailed)) {
		t.Fatal("the status enum literal must be recognized as an enum, never as a failure detail")
	}
	if schedulerChainStatusEnumLiteral(fallbackDecisionLayerSentence) {
		t.Fatal("the decision layer's original sentence must not be mistaken for an enum")
	}
}
