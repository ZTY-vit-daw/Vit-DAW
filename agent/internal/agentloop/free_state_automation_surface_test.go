package agentloop

// free_state_automation_surface_test.go — M3-LLM-GATE-OPEN-1（闸 1 开放）
// 探针面：
//  1. 名字归一/规范名/mutation 分类；
//  2. 准入 union（free-state 活跃=并入四工具；诊断专用/非 free-state=零变化；
//     幂等）；
//  3. needs_observation 校验器放行 automation 面（读直放+写合法）、拒绝面外
//     mutation、拒绝同轮双写、拒绝诊断专用轮；
//  4. 执行守卫：面内 mutation 放行至治理门，面外 mutation 仍弹回；
//  5. 显示面：Allowed tools/目录行含四工具与治理门语义；骨架 twin 含新
//     禁令语义且旧一刀切句已消失；
//  6. 闸 2 headroom：首次命中 +4 turn，一次为限；
//  7. 零裸写端到端负例：free-state 轮模型 set_points 调用→executor 首见
//     Confirmed=false+needs_confirmation 暂停（WaitingConfirmation）→确认
//     恢复才 Confirmed=true 执行——任何路径不出现未经确认的写。

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func automationSurfaceTestState(mutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":  "free_state_reasoning_loop.v1",
			"status":          "reasoning",
			"original_intent": "ride the track volume up over the section",
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

func automationSurfaceDecision(status string) *FreeStateDecision {
	return &FreeStateDecision{
		SchemaVersion:  FreeStateDecisionSchema,
		Status:         status,
		EvidenceStatus: "insufficient",
		Summary:        "resolve the volume parameter range first",
	}
}

func automationReadCall() planner.ToolCall {
	return planner.ToolCall{ID: "c1", Tool: "get_plugin_parameters",
		Args: map[string]any{"track_id": "1012", "plugin_id": "plg_1"}, Reason: "read the value range"}
}

func automationWriteCall() planner.ToolCall {
	return planner.ToolCall{ID: "c2", Tool: "automation.set_points",
		Args: map[string]any{"plugin_id": "plg_1", "param_id": "volume",
			"points": []any{map[string]any{"t": 0.0, "v": 0.3}, map[string]any{"t": 30.0, "v": 0.6}}},
		Reason: "write the envelope"}
}

func automationClearCall() planner.ToolCall {
	return planner.ToolCall{ID: "c3", Tool: "automation.clear_region",
		Args: map[string]any{"plugin_id": "plg_1", "param_id": "volume",
			"region": map[string]any{"start": 0.0, "end": 30.0}}, Reason: "clear"}
}

func TestFreeStateAutomationSurfaceNameClassification(t *testing.T) {
	for name, canonical := range map[string]string{
		"plugin.get_parameters":   "plugin.get_parameters",
		"plugin_get_parameters":   "plugin.get_parameters",
		"get_plugin_parameters":   "plugin.get_parameters",
		"automation.get_curve":    "automation.get_curve",
		"automation_get_curve":    "automation.get_curve",
		"automation.set_points":   "automation.set_points",
		"automation_set_points":   "automation.set_points",
		"automation.clear_region": "automation.clear_region",
	} {
		if !messageLoopIsFreeStateAutomationSurfaceName(name) {
			t.Fatalf("%s should be on the admitted surface", name)
		}
		if got := freeStateAutomationSurfaceCanonicalName(name); got != canonical {
			t.Fatalf("canonical(%s) = %q, want %q", name, got, canonical)
		}
	}
	for _, outside := range []string{"set_plugin_param", "plugin.set_parameter", "set_volume", "clip.fade.set", "ccb.observation_request", ""} {
		if messageLoopIsFreeStateAutomationSurfaceName(outside) {
			t.Fatalf("%s must stay off the surface", outside)
		}
	}
	if !messageLoopIsFreeStateAutomationMutationName("automation.set_points") || !messageLoopIsFreeStateAutomationMutationName("automation_clear_region") {
		t.Fatal("set_points/clear_region are the governance-gate mutations")
	}
	if messageLoopIsFreeStateAutomationMutationName("automation.get_curve") || messageLoopIsFreeStateAutomationMutationName("get_plugin_parameters") {
		t.Fatal("read tools must not be classified as mutations")
	}
}

func TestAdmitFreeStateDecisionRoundToolSurface(t *testing.T) {
	state := automationSurfaceTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	admitFreeStateDecisionRoundToolSurface(state)
	for _, want := range freeStateAutomationSurfaceTools {
		if !allowedTool(want, state.input.AllowedTools) {
			t.Fatalf("admitted surface missing %q: %v", want, state.input.AllowedTools)
		}
	}
	before := len(state.input.AllowedTools)
	admitFreeStateDecisionRoundToolSurface(state)
	if len(state.input.AllowedTools) != before {
		t.Fatalf("admission must be idempotent: %d -> %d", before, len(state.input.AllowedTools))
	}

	diagnostic := automationSurfaceTestState(func(ctx map[string]any) {
		ctx["free_state_diagnostic_only"] = true
	})
	diagnostic.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	admitFreeStateDecisionRoundToolSurface(diagnostic)
	if allowedTool("automation.set_points", diagnostic.input.AllowedTools) {
		t.Fatal("diagnostic-only turns must not gain the automation surface")
	}

	plain := &runState{input: Input{Context: map[string]any{}, AllowedTools: []string{"mix.observe"}}}
	admitFreeStateDecisionRoundToolSurface(plain)
	if len(plain.input.AllowedTools) != 1 {
		t.Fatalf("non-free-state rounds must stay unchanged: %v", plain.input.AllowedTools)
	}
}

func TestNeedsObservationAdmitsAutomationSurfaceCalls(t *testing.T) {
	state := automationSurfaceTestState(nil)

	read := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{automationReadCall()}}
	if issue := messageLoopFreeStateOutputIssue(state, read); issue != "" {
		t.Fatalf("read-only surface call refused: %q", issue)
	}

	write := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{automationWriteCall()}}
	if issue := messageLoopFreeStateOutputIssue(state, write); issue != "" {
		t.Fatalf("governed mutation call refused: %q", issue)
	}

	mixed := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{
			{ID: "c0", Tool: "ccb.observation_catalog", Args: map[string]any{}, Reason: "discover views"},
			automationWriteCall(),
		}}
	if issue := messageLoopFreeStateOutputIssue(state, mixed); issue != "" {
		t.Fatalf("catalog+mutation turn refused: %q", issue)
	}
}

func TestNeedsObservationRejectsOutOfSurfaceAndDoubleMutation(t *testing.T) {
	state := automationSurfaceTestState(nil)

	outside := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{{ID: "x1", Tool: "set_plugin_param",
			Args: map[string]any{"plugin_id": "plg_1", "param_id": "volume", "value": 0.5}}}}
	if issue := messageLoopFreeStateOutputIssue(state, outside); !strings.Contains(issue, "may call only") {
		t.Fatalf("out-of-surface mutation must bounce, got %q", issue)
	}

	double := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{automationWriteCall(), automationClearCall()}}
	if issue := messageLoopFreeStateOutputIssue(state, double); !strings.Contains(issue, "at most one automation mutation") {
		t.Fatalf("two mutations in one turn must bounce, got %q", issue)
	}
}

func TestDiagnosticOnlyNeedsObservationRejectsAutomationSurface(t *testing.T) {
	state := automationSurfaceTestState(func(ctx map[string]any) {
		ctx["free_state_diagnostic_only"] = true
	})
	out := messageLoopOutput{FreeStateDecision: automationSurfaceDecision(FreeStateNeedsObservation),
		ToolCalls: []planner.ToolCall{automationReadCall()}}
	if issue := messageLoopFreeStateOutputIssue(state, out); !strings.Contains(issue, "diagnostic-only") {
		t.Fatalf("diagnostic-only automation call must bounce, got %q", issue)
	}
}

func TestFreeStateMutationGuardAdmitsSurfaceAndBouncesOthers(t *testing.T) {
	state := automationSurfaceTestState(nil)
	if issue := messageLoopToolGuardIssue(state, automationWriteCall(), false); issue != "" {
		t.Fatalf("admitted-surface mutation must ride the governance gate, got %q", issue)
	}
	if issue := messageLoopToolGuardIssue(state, automationReadCall(), false); issue != "" {
		t.Fatalf("read-only surface call must pass, got %q", issue)
	}
	outside := planner.ToolCall{ID: "x", Tool: "set_plugin_param",
		Args: map[string]any{"plugin_id": "plg_1", "param_id": "volume", "value": 0.5}}
	if issue := messageLoopToolGuardIssue(state, outside, false); !strings.Contains(issue, "forbidden") {
		t.Fatalf("out-of-surface mutation must stay forbidden, got %q", issue)
	}

	diagnostic := automationSurfaceTestState(func(ctx map[string]any) {
		ctx["free_state_diagnostic_only"] = true
	})
	if issue := messageLoopToolGuardIssue(diagnostic, automationWriteCall(), false); !strings.Contains(issue, "diagnostic-only") {
		t.Fatalf("diagnostic-only mutation must bounce, got %q", issue)
	}
}

func TestNeutralFamilyDisplayCarriesAutomationSurface(t *testing.T) {
	tools := []string{
		"ccb.observation_catalog", "ccb.observation_request",
		"plugin.get_parameters", "automation.get_curve",
		"automation.set_points", "automation.clear_region",
		"get_plugin_parameters", // alias form canonicalizes, no duplicate row
	}
	allowed := messageLoopNeutralFamilyAllowedTools(tools)
	want := []string{"ccb.observation_catalog", "ccb.observation_request",
		"plugin.get_parameters", "automation.get_curve", "automation.set_points", "automation.clear_region"}
	if len(allowed) != len(want) {
		t.Fatalf("allowed tools = %v, want %v", allowed, want)
	}
	for i, name := range want {
		if allowed[i] != name {
			t.Fatalf("allowed tools = %v, want %v", allowed, want)
		}
	}
	catalog := messageLoopNeutralFamilyCatalog(tools)
	for _, marker := range []string{
		"plugin.get_parameters:", "automation.get_curve:", "automation.set_points:", "automation.clear_region:",
		"governance gate", "value range", "readback",
	} {
		if !strings.Contains(catalog, marker) {
			t.Fatalf("catalog missing %q:\n%s", marker, catalog)
		}
	}
}

func TestSkeletonMutationRuleCarriesGovernanceGateSemantics(t *testing.T) {
	state := automationSurfaceTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	admitFreeStateDecisionRoundToolSurface(state)
	skeleton := messageLoopNeutralFamilySystemSkeleton(state)
	for _, marker := range []string{
		"The automation surface is admitted in this phase",
		"always pass the runtime governance gate",
		"at most one such mutation per turn",
		"Do not call any other mutation, plug-in loading, or typed control tool in this phase",
	} {
		if !strings.Contains(skeleton, marker) {
			t.Fatalf("skeleton missing %q", marker)
		}
	}
	if strings.Contains(skeleton, "Do not call any mutation, plug-in loading, or typed control tool in this phase.") {
		t.Fatal("the blanket mutation prohibition must be gone from the skeleton")
	}
	// twin：legacy 回落模板与 embed 渲染同文（字节级由
	// TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte 锁定；此处钉
	// 语义同在）。
	for _, marker := range []string{
		"The automation surface is admitted in this phase",
		"always pass the runtime governance gate",
	} {
		if !strings.Contains(legacyNeutralFamilySkeletonTemplate, marker) {
			t.Fatalf("legacy template missing %q", marker)
		}
	}
	if strings.Contains(legacyNeutralFamilySkeletonTemplate, "Do not call any mutation, plug-in loading, or typed control tool in this phase.") {
		t.Fatal("the blanket mutation prohibition must be gone from the legacy template")
	}
}

func TestNoteFreeStateAutomationEngagementHeadroom(t *testing.T) {
	state := automationSurfaceTestState(nil)
	state.budget = Budget{MaxTurns: 8, MaxToolCalls: 12, Timeout: 3 * time.Minute}
	noteFreeStateAutomationEngagement(state)
	if state.budget.MaxTurns != 12 || state.budget.Timeout != 5*time.Minute {
		t.Fatalf("headroom = %+v, want MaxTurns 12 / +120s", state.budget)
	}
	noteFreeStateAutomationEngagement(state)
	if state.budget.MaxTurns != 12 {
		t.Fatalf("headroom must be granted once, got MaxTurns=%d", state.budget.MaxTurns)
	}
}

// automationSurfaceTestExecutor 模拟 harness 治理门形态：未确认的
// set_points/clear_region 回 needs_confirmation（零写入），确认后执行 ok；
// 其余工具走共享 fake。记录每次 Confirmed 供零裸写断言。
type automationSurfaceTestExecutor struct {
	fake    *fakeMessageExecutor
	calls   []planner.ToolCall
	confirm []bool
}

func (e *automationSurfaceTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	name := strings.TrimSpace(in.ToolCall.Tool)
	if name == "automation.set_points" || name == "automation.clear_region" {
		e.calls = append(e.calls, in.ToolCall)
		e.confirm = append(e.confirm, in.Confirmed)
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID: in.ToolCall.ID, Tool: name, CommandName: name,
				Status: "needs_confirmation", RequiresConfirmation: true,
				Preview: "Replace volume automation points (2 points, 0-30s).",
			}, nil
		}
		return executorpkg.Result{
			ToolCallID: in.ToolCall.ID, Tool: name, CommandName: name,
			Status: "ok",
			Result: map[string]any{"status": "ok", "written": 2, "point_count": 2},
		}, nil
	}
	return e.fake.RunToolCall(context.Background(), in)
}

// TestFreeStateSetPointsZeroBareWriteGovernanceChain 是卡面安全面主负例：
// free-state 决策轮的模型 set_points 调用必须经治理门——首轮 executor 只见
// Confirmed=false 的 needs_confirmation 提案（零裸写），确认恢复后才以
// Confirmed=true 执行，随后模型读回报文终态。
func TestFreeStateSetPointsZeroBareWriteGovernanceChain(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"我先把这条轨的音量包络写上去。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"parameter range already read; write the envelope now"},"tool_calls":[{"tool":"automation.set_points","args":{"plugin_id":"plg_1","param_id":"volume","points":[{"t":0,"v":0.3},{"t":30,"v":0.6}]},"reason":"ride the volume up"}]}`,
		`{"final":true,"reply":"已写入 0→30 秒的音量爬升包络并逐点读回验证。","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"envelope written and read back point by point"}}`,
	}}
	exec := &automationSurfaceTestExecutor{fake: &fakeMessageExecutor{}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 12, MaxConsecutiveErrors: 2},
	}

	input := Input{
		UserText: "这段让它越来越突出一点",
		Context: map[string]any{
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1",
				"status":         "reasoning",
				"original_intent": "让这条轨的存在感慢慢强起来",
			},
		},
		AllowedTools: []string{"ccb.observation_catalog", "ccb.observation_request"},
	}
	first := loop.Start(context.Background(), input)

	if first.Status != "waiting_confirmation" || first.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("first turn = status=%q stop=%q reply=%q error=%q", first.Status, first.StopReason, first.Reply, first.Error)
	}
	if len(exec.calls) != 1 || exec.confirm[0] {
		t.Fatalf("zero bare write violated: executor saw calls=%+v confirmed=%v", exec.calls, exec.confirm)
	}
	if first.Continuation == nil || first.Continuation.PendingToolCall == nil ||
		first.Continuation.PendingToolCall.Tool != "automation.set_points" {
		t.Fatalf("pending set_points continuation expected, got %+v", first.Continuation)
	}

	second := loop.ResumeAfterConfirmation(context.Background(), *first.Continuation)
	if second.Status != "completed" {
		t.Fatalf("confirmed resume = status=%q stop=%q reply=%q error=%q", second.Status, second.StopReason, second.Reply, second.Error)
	}
	if len(exec.calls) != 2 || !exec.confirm[1] {
		t.Fatalf("confirmed execution missing: calls=%+v confirmed=%v", exec.calls, exec.confirm)
	}
	if !strings.Contains(second.Reply, "读回") {
		t.Fatalf("final reply should carry the readback report, got %q", second.Reply)
	}
}
