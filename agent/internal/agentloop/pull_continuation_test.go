package agentloop

// pull_continuation_test.go — 恢复载体边界（§11.3-4 主管裁定第 4 项）：
// schema 版本化 fail 边界 / 旧 continuation=push fail-open / 身份绑定校验 /
// checkpoint 字段往返。

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/pullharness"
)

// 旧 continuation（无 Pull 字段）→ IsPull=false：按原 push 语义解释
// （fail-open，零行为变化——入口路由据此走 legacy Continue）。
func TestPullContinuationLegacyFailOpen(t *testing.T) {
	legacy := &PullContinuation{Continuation: &Continuation{GoalID: "g", UserText: "u"}}
	if legacy.IsPull() {
		t.Error("legacy continuation without pull fields must not be treated as pull (fail-open to push)")
	}
	if err := legacy.ValidatePull("g", "r", ""); err == nil {
		t.Error("ValidatePull must reject non-pull carriers explicitly")
	}
}

// 未知 schema 版本/绑定不符 → 拒绝并报因（保留数据，不默默转 push）。
func TestPullContinuationVersionAndBindingBoundaries(t *testing.T) {
	base := func() *PullContinuation {
		return &PullContinuation{
			SchemaVersion: pullContinuationSchemaVersion,
			HarnessMode:   "pull",
			Continuation:  &Continuation{GoalID: "goal-a", RunID: "run-a"},
			Pull: &PullCheckpoint{
				GoalID: "goal-a", RunID: "run-a", SessionKey: "pullharness:run-a",
				NextCycle: 3, CompletedCycles: 2, ModelCalls: 4, ToolAttempts: 5,
				SettledReceipts: []string{"r1", "r2"}, ClosedBatchIDs: []string{"run-a:cycle:1"},
			},
		}
	}
	if err := base().ValidatePull("goal-a", "run-a", ""); err != nil {
		t.Fatalf("valid checkpoint rejected: %v", err)
	}
	future := base()
	future.SchemaVersion = pullContinuationSchemaVersion + 1
	if err := future.ValidatePull("goal-a", "run-a", ""); err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Errorf("future schema accepted: %v", err)
	}
	wrongGoal := base()
	wrongGoal.Pull.GoalID = "goal-other"
	if err := wrongGoal.ValidatePull("goal-a", "run-a", ""); err == nil || !strings.Contains(err.Error(), "goal binding") {
		t.Errorf("goal binding mismatch accepted: %v", err)
	}
	wrongRun := base()
	wrongRun.Pull.RunID = "run-other"
	if err := wrongRun.ValidatePull("goal-a", "run-a", ""); err == nil || !strings.Contains(err.Error(), "run binding") {
		t.Errorf("run binding mismatch accepted: %v", err)
	}
	wrongMode := base()
	wrongMode.HarnessMode = "push"
	if err := wrongMode.ValidatePull("goal-a", "run-a", ""); err == nil || !strings.Contains(err.Error(), "harness_mode") {
		t.Errorf("mode mismatch accepted: %v", err)
	}
}

// Suspend 提交产 checkpoint：账本/已结算回执/已关批/T1 面完整快照；恢复时
// ledger 还原（已结算不重复计费——同 receipt 二次结算零新增）。
func TestPullContinuationCheckpointRoundTrip(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, func(ctx map[string]any) {
		ctx["pull_observation_budget"] = map[string]any{"max_cycles": 5, "max_probe_cost": 2}
	})
	// 一个完整批（回执 r*=1 笔）后暂停。
	step, _ := session.Interpret(context.Background(),
		`{"final":false,"reply":"work","tool_calls":[{"tool":"pull.echo","args":{}}]}`)
	if _, err := session.Execute(context.Background(), step.Calls); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := session.CloseCycle(context.Background(), "run-pull:cycle:1", nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	// 触发暂停提交（ctx 取消面）。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	interruptStep, err := session.Attempt(ctx)
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if _, err := session.Return(ctx, interruptStep); err != nil {
		t.Fatalf("return: %v", err)
	}
	checkpoint := session.lastCheckpoint
	if checkpoint == nil || !checkpoint.IsPull() {
		t.Fatalf("interrupt commit lost checkpoint: %+v", checkpoint)
	}
	cp := checkpoint.Pull
	if cp.CompletedCycles != 1 || cp.NextCycle < 1 || cp.ModelCalls != 1 {
		t.Errorf("checkpoint ledger = %+v, want completed=1 model=1", cp)
	}
	if len(cp.SettledReceipts) != 1 || len(cp.ClosedBatchIDs) != 1 {
		t.Errorf("checkpoint receipts/batches = %v/%v, want 1/1", cp.SettledReceipts, cp.ClosedBatchIDs)
	}
	if cp.Generation != 1 || cp.SessionKey == "" || cp.GoalID == "" {
		t.Errorf("checkpoint identity = %+v", cp)
	}

	// 恢复：新会话从 checkpoint 还原账本（绑定校验通过）。
	exec2 := &pullTestExecutor{}
	session2, l2 := newPullTestSession(t, exec2, nil)
	restored, err := newPullSession(l2, l2.runner(), session2.state, checkpoint)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.ledger.completedCycles != 1 || restored.ledger.modelCalls != 1 {
		t.Errorf("restored ledger = %+v", restored.ledger)
	}
	if restored.ledger.maxCycles != 5 {
		t.Errorf("restored max cycles = %d, want 5 (budget carried)", restored.ledger.maxCycles)
	}
	// 已结算回执不重复计费（恢复面单次结算契约）。
	if fresh := restored.ledger.settle(cp.SettledReceipts); len(fresh) != 0 {
		t.Errorf("restored session re-settled %d receipts", len(fresh))
	}
	// 同 BatchID 不再重复 T1。
	if err := restored.CloseCycle(context.Background(), "run-pull:cycle:1", nil); err != nil {
		t.Fatalf("duplicate close: %v", err)
	}
	if restored.ledger.completedCycles != 1 {
		t.Errorf("restored completed cycles = %d, want 1 (same batch T1<=1)", restored.ledger.completedCycles)
	}
}

// 会话登记面：注册/查找/释放（同进程恢复的载体查找）。
func TestPullSessionRegistry(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	goalID := session.state.goal.GoalID
	pullSessionRegister(goalID, session)
	if found, ok := pullSessionLookup(goalID); !ok || found != session {
		t.Fatal("registry lookup failed")
	}
	pullSessionRelease(goalID)
	if _, ok := pullSessionLookup(goalID); ok {
		t.Fatal("released session still found")
	}
}

// 恢复载体经 Frame/ledger 投影可见：probe unknown 语义随 checkpoint 存续。
func TestPullContinuationUnknownCostCarried(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	step, _ := session.Interpret(context.Background(),
		`{"final":false,"reply":"work","tool_calls":[{"tool":"pull.echo","args":{}}]}`)
	if _, err := session.Execute(context.Background(), step.Calls); err != nil {
		t.Fatalf("execute: %v", err)
	}
	frame, _ := session.Snapshot(context.Background())
	if frame.Budget.ProbeCostKnown {
		t.Skip("metering source appeared; unknown-cost assertion not applicable")
	}
	// 暂停面：checkpoint 记录 unknown 状态（恢复后不冒充已知零成本）。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	interruptStep, _ := session.Attempt(ctx)
	session.Return(ctx, interruptStep)
	if session.lastCheckpoint == nil || session.lastCheckpoint.Pull.ProbeCostKnown {
		t.Error("checkpoint lost unknown-cost state")
	}
	_ = pullharness.OutcomeInterrupted
}
