package agentloop

// pull_entry_test.go — 双模入口路由验收（L1-5-IMPL-D 腿3）：
// 缺省 push 旧行为零变化（push 装配形态断言）+ pull 分流（pull 装配形态+
// Session 驱动面）+恢复边界（同会话续跑/跨进程拒绝/旧载体 fail-open）。

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/pullharness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// withPullMode 覆盖模式解析缝（测试后还原）。
func withPullMode(t *testing.T, mode string) {
	t.Helper()
	previous := pullHarnessModeResolver
	pullHarnessModeResolver = func() pullharness.HarnessMode {
		return pullharness.HarnessMode{Mode: mode, Source: "test"}
	}
	t.Cleanup(func() { pullHarnessModeResolver = previous })
}

func newPullEntryLoop(client *fakeMessageCompleter, exec *pullTestExecutor) *MessageLoop {
	if client == nil {
		client = &fakeMessageCompleter{}
	}
	return &MessageLoop{
		Runtime:  loopTestRuntime(),
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 12, MaxConsecutiveErrors: 2},
		Now:      func() time.Time { return time.Date(2026, 10, 9, 21, 0, 0, 0, time.UTC) },
	}
}

// joinedText 拼接全部模型调用收到的消息文本（装配形态断言面）。
func joinedText(calls [][]llm.Message) string {
	var builder strings.Builder
	for _, call := range calls {
		for _, message := range call {
			builder.WriteString(message.Content)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func clientTextCalls(client *fakeMessageCompleter) [][]llm.Message {
	return client.calls
}

// 缺省 push：无模式标记时入口走旧路径——装配为 push 形态（Current Goal
// 用户段+全量快照注入），不经 pullSession。
func TestPullEntryDefaultPushZeroChange(t *testing.T) {
	if pullHarnessEnabled() {
		t.Fatal("default mode must be push when no env/config marker is set by this test")
	}
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"push done"}`}}
	loop := newPullEntryLoop(client, &pullTestExecutor{})
	result := loop.Start(context.Background(), Input{
		GoalID: "goal-entry-push", RunID: "run-entry-push",
		UserText: "这是一条中性测试输入，不命中任何快路径", Summary: "这是一条中性测试输入，不命中任何快路径",
	})
	if result.Status != agentruntime.StatusCompleted || result.Reply != "push done" {
		t.Fatalf("push result = %+v", result)
	}
	if len(client.calls) != 1 {
		t.Fatalf("push model calls = %d, want 1", len(client.calls))
	}
	joined := joinedText(clientTextCalls(client))
	if !strings.Contains(joined, "Current Goal:") {
		t.Error("default entry lost push assembly shape (Current Goal user section missing)")
	}
	if strings.Contains(joined, "budget_state:") {
		t.Error("default entry leaked pull dynamic zone into push assembly")
	}
	if _, ok := pullSessionLookup("goal-entry-push"); ok {
		t.Error("default push registered a pull session (must not)")
	}
}

// pull 模式：Start 经 pullSession 驱动——装配为 pull 形态（goal 行+账本
// 披露动态区），终局经宿主 Return 提交，终局后登记面释放。
func TestPullEntryStartRoutesPullSession(t *testing.T) {
	withPullMode(t, pullharness.ModePull)
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"pull done"}`}}
	loop := newPullEntryLoop(client, &pullTestExecutor{})
	result := loop.Start(context.Background(), Input{
		GoalID: "goal-entry-pull", RunID: "run-entry-pull",
		UserText: "这是一条中性测试输入，不命中任何快路径", Summary: "这是一条中性测试输入，不命中任何快路径",
	})
	if result.Status != agentruntime.StatusCompleted || result.Reply != "pull done" {
		t.Fatalf("pull result = %+v (status=%s stop=%s err=%s)", result, result.Status, result.StopReason, result.Error)
	}
	joined := joinedText(clientTextCalls(client))
	if !strings.Contains(joined, "budget_state:") {
		t.Error("pull assembly missing dynamic zone budget disclosure")
	}
	if strings.Contains(joined, "Current Goal:") {
		t.Error("pull assembly leaked push full-snapshot user section")
	}
	if _, ok := pullSessionLookup("goal-entry-pull"); ok {
		t.Error("terminal pull session not released from registry")
	}
}

// pull 确认暂停→Continue 回同一会话：确认执行一次（Confirmed=true），终局
// 提交；旧会话状态（trace/账本）不重建。
func TestPullEntryConfirmationResumeSameSession(t *testing.T) {
	withPullMode(t, pullharness.ModePull)
	exec := &pullTestExecutor{requires: map[string]bool{"pull.confirm.tool": true}}
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"proposing","tool_calls":[{"tool":"pull.confirm.tool","args":{}}]}`,
	}}
	loop := newPullEntryLoop(client, exec)
	result := loop.Start(context.Background(), Input{
		GoalID: "goal-entry-confirm", RunID: "run-entry-confirm",
		UserText: "对目标执行一次需要确认的中性操作", Summary: "对目标执行一次需要确认的中性操作",
	})
	if result.Status != agentruntime.StatusWaitingConfirmation {
		t.Fatalf("pause result = %s/%s (err=%s)", result.Status, result.StopReason, result.Error)
	}
	if result.Continuation == nil || result.Continuation.PullCheckpoint == nil {
		t.Fatalf("pause continuation missing pull checkpoint: %+v", result.Continuation)
	}
	session, ok := pullSessionLookup("goal-entry-confirm")
	if !ok {
		t.Fatal("suspended pull session not retained in registry")
	}
	ledgerBefore := session.ledger.completedCycles

	// 确认恢复：Confirmed=true 恰一次，终局提交。
	client.responses = []string{`{"final":true,"reply":"confirmed done"}`}
	resumed := loop.ResumeAfterConfirmation(context.Background(), *result.Continuation)
	if resumed.Status != agentruntime.StatusCompleted {
		t.Fatalf("resume result = %s/%s (err=%s)", resumed.Status, resumed.StopReason, resumed.Error)
	}
	if exec.applies() != 1 {
		t.Errorf("confirmed executions = %d, want exactly 1", exec.applies())
	}
	if session.ledger.completedCycles < ledgerBefore {
		t.Error("resume rebuilt ledger (must continue the same session)")
	}
	if _, still := pullSessionLookup("goal-entry-confirm"); still {
		t.Error("terminal-after-resume session not released")
	}
}

// 跨进程边界：continuation 带 pull checkpoint 但活会话丢失（登记面无）→
// 显式拒绝并报产品边界，不重放已执行动作。
func TestPullEntryCrossProcessRejection(t *testing.T) {
	withPullMode(t, pullharness.ModePull)
	exec := &pullTestExecutor{}
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"proposing","tool_calls":[{"tool":"pull.confirm.tool","args":{}}]}`,
	}}
	exec.requires = map[string]bool{"pull.confirm.tool": true}
	loop := newPullEntryLoop(client, exec)
	result := loop.Start(context.Background(), Input{
		GoalID: "goal-entry-xproc", RunID: "run-entry-xproc",
		UserText: "对目标执行一次需要确认的中性操作", Summary: "对目标执行一次需要确认的中性操作",
	})
	if result.Status != agentruntime.StatusWaitingConfirmation || result.Continuation == nil {
		t.Fatalf("pause result = %+v", result)
	}
	pullSessionRelease("goal-entry-xproc") // 模拟进程重启（活会话丢失）
	resumed := loop.ResumeAfterConfirmation(context.Background(), *result.Continuation)
	if resumed.Status != agentruntime.StatusFailed {
		t.Fatalf("cross-process resume result = %s, want failed (explicit rejection)", resumed.Status)
	}
	if !strings.Contains(resumed.Error, "cross-process") {
		t.Errorf("rejection missing product boundary reason: %s", resumed.Error)
	}
	if exec.applies() != 0 {
		t.Errorf("cross-process resume replayed %d executed actions (must be zero)", exec.applies())
	}
}

// 旧 continuation（无 pull 字段）在 pull 模式下 fail-open 走 push 路径
// （零行为变化——装配形态回归 push）。
func TestPullEntryLegacyContinuationFailOpen(t *testing.T) {
	withPullMode(t, pullharness.ModePull)
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"legacy continued"}`}}
	loop := newPullEntryLoop(client, &pullTestExecutor{})
	cont := Continuation{
		GoalID:   "goal-entry-legacy",
		RunID:    "run-entry-legacy",
		UserText: "继续任务", Summary: "继续任务",
		Budget: Budget{MaxTurns: 4, MaxToolCalls: 6, MaxConsecutiveErrors: 2},
	}
	result := loop.Continue(context.Background(), cont)
	if result.Status != agentruntime.StatusCompleted || result.Reply != "legacy continued" {
		t.Fatalf("legacy continue result = %+v (err=%s)", result, result.Error)
	}
	joined := joinedText(clientTextCalls(client))
	if !strings.Contains(joined, "Current Goal:") {
		t.Error("legacy continuation did not fail-open to push assembly shape")
	}
}

// 模式解析面：pullHarnessEnabled 消费 ResolveHarnessMode 结果（缺省 push）。
func TestPullEntryModeResolutionDefault(t *testing.T) {
	if mode := pullharness.ResolveHarnessMode(); mode.Mode != pullharness.ModePush {
		t.Errorf("default resolved mode = %q (source=%s), want push", mode.Mode, mode.Source)
	}
}

// 并发面烟测：入口分流不改变串行假设（两个独立 goal 各自完整跑通 pull）。
func TestPullEntryTwoGoalsIndependent(t *testing.T) {
	withPullMode(t, pullharness.ModePull)
	var completions atomic.Int32
	for i := 0; i < 2; i++ {
		client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"ok"}`}}
		loop := newPullEntryLoop(client, &pullTestExecutor{})
		goalID := "goal-entry-multi-" + string(rune('a'+i))
		result := loop.Start(context.Background(), Input{
			GoalID: goalID, RunID: "run-" + goalID, UserText: "测试", Summary: "测试",
		})
		if result.Status != agentruntime.StatusCompleted {
			t.Fatalf("goal %s result = %+v (err=%s)", goalID, result, result.Error)
		}
		completions.Add(1)
	}
	if completions.Load() != 2 {
		t.Errorf("completions = %d, want 2", completions.Load())
	}
}
