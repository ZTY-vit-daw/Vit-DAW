package pullharness

// loop_cancel_test.go — CANCEL-FIX 冻结表取消回归在 Session 面重新锚定
// （L1-5-IMPL-D 腿1：§11.1 授权随 A 占位接口退役迁移；六路径语义不变——
// 已收事实不丢/未完批不 CloseCycle/无 T2/先完批 T1 存续/不退款/不虚报完成）。
//
// 六路径（A 阶段命名→Session 面对应）：
//   entry          → 驱动 zone：Attempt 前 ctx 已取消
//   route          → 宿主 zone：Attempt 内取消（快路径面）
//   complete       → 驱动 zone：模型返回时取消（partial reply 保留）
//   plan           → 宿主 zone：Interpret 返回待执行批时取消（批不执行）
//   execute_empty  → 宿主 zone：Execute 空回执返回时取消
//   execute_result → 宿主 zone：Execute 带回执返回时取消（回执不丢）

import (
	"context"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
)

type cancelLLMFunc func(context.Context, config.EngineConfig, []llm.Message) (string, error)

func (f cancelLLMFunc) Complete(ctx context.Context, cfg config.EngineConfig, msgs []llm.Message) (string, error) {
	return f(ctx, cfg, msgs)
}

// runCancelStage 在指定 stage 同步取消 ctx（含已执行工作成功返回的组件），
// 返回驱动 Result 与宿主记录面。
func runCancelStage(t *testing.T, stage string, priorCompletedBatch bool) (*fakeSession, *fakeExit, Result) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &fakeSession{runID: "run-cancel-" + stage, goalID: "goal-1"}
	exit := &fakeExit{}
	session.exit = exit

	session.attemptFn = func(f *fakeSession, ctx context.Context) (Step, error) {
		if stage == "route" {
			cancel()
			// 宿主 zone 取消：宿主自建中断步（Source=快路径面），交 Return。
			return Step{Disposition: DispositionSuspend, Kind: ReturnInterrupted,
				Source: StepSourceFastPath, Error: "interrupted: inside_attempt"}, nil
		}
		return Step{Disposition: DispositionContinue, Source: StepSourceFastPath}, nil
	}
	session.interpretFn = func(f *fakeSession, ctx context.Context, raw string) (Step, error) {
		if stage == "plan" {
			cancel()
		}
		if priorCompletedBatch && f.budget.CompletedCycles == 0 {
			return Step{Disposition: DispositionContinue, Source: StepSourceModel,
				Calls: []ToolCall{{ID: "first-1", Tool: "existing.tool"}}}, nil
		}
		return Step{Disposition: DispositionContinue, Source: StepSourceModel,
			Calls: []ToolCall{{ID: "action-1", Tool: "existing.tool"}}}, nil
	}
	session.executeFn = func(f *fakeSession, ctx context.Context, calls []ToolCall) (Step, error) {
		if (stage == "execute_empty" || stage == "execute_result") && (!priorCompletedBatch || f.executeCalls >= 2) {
			cancel()
		}
		step := Step{Disposition: DispositionContinue, Source: StepSourceModel}
		if stage != "execute_empty" {
			ids := make([]string, 0, len(calls))
			for _, call := range calls {
				ids = append(ids, call.ID)
			}
			step.ReceiptIDs = ids
		}
		return step, nil
	}
	session.closeCycleFn = func(f *fakeSession, ctx context.Context, batchID string, exit contextruntime.ExitExecutor) error {
		f.closedBatchIDs = append(f.closedBatchIDs, batchID)
		f.budget.CompletedCycles++
		batch := []ToolResult{{ID: "first-1", Tool: "existing.tool", Status: "ok", Bytes: 100}}
		if exit != nil {
			event, _, _ := BatchTurnEvent(batchID, batch, ExitWiring{})
			exit.OnTurnBoundary(ctx, event)
		}
		return nil
	}

	in := goalInput("run-cancel-" + stage)
	in.Conversation = []llm.Message{{Role: "assistant", Content: "prior fact"}}
	if stage == "entry" {
		cancel()
	}
	loop := newSessionLoop(session, cancelLLMFunc(func(context.Context, config.EngineConfig, []llm.Message) (string, error) {
		if stage == "complete" {
			cancel()
		}
		return "model fact", nil
	}), &fakePrefix{}, exit)
	result := loop.Run(ctx, in)
	return session, exit, result
}

// 六路径：中断步恰一次交宿主 Return；未完批零 CloseCycle、零边界事件；
// 已收事实（partial reply/回执计数）保留；模型轮计数与实际调用一致。
func TestPullLoopCancellationBoundaries(t *testing.T) {
	for _, stage := range []string{"entry", "route", "complete", "plan", "execute_empty", "execute_result"} {
		t.Run(stage, func(t *testing.T) {
			session, exit, result := runCancelStage(t, stage, false)
			if session.returnCalls != 1 {
				t.Fatalf("Return calls=%d, want exactly 1 (interrupted run hands off once)", session.returnCalls)
			}
			step := session.returnedSteps[0]
			if step.Disposition != DispositionSuspend || step.Kind != ReturnInterrupted {
				t.Errorf("step=%+v, want suspend/interrupted", step)
			}
			if result.Outcome != OutcomeInterrupted {
				t.Errorf("outcome=%q, want interrupted", result.Outcome)
			}
			if session.closeCalls != 0 || len(exit.events) != 0 {
				t.Errorf("cancelled batch closed or boundary fired: close=%d events=%+v", session.closeCalls, exit.events)
			}
			// 各 stage 的推进深度（已产生事实数）：entry/route/complete 均
			// 未到 Interpret；plan 到 Interpret；execute_* 到 Execute。
			want := map[string][2]int{
				"entry": {0, 0}, "route": {0, 0}, "complete": {0, 0},
				"plan": {1, 0}, "execute_empty": {1, 1}, "execute_result": {1, 1},
			}[stage]
			if got := [2]int{session.interpretCalls, session.executeCalls}; got != want {
				t.Errorf("interpret/execute=%v, want %v (facts already produced must stand)", got, want)
			}
			// 事实保留面：complete 路径 partial reply 必须到达宿主。
			if stage == "complete" && step.Reply != "model fact" {
				t.Errorf("partial model reply lost: %q", step.Reply)
			}
		})
	}
}

// 先完批存续：第一批完整（CloseCycle 恰一 T1）后第二批取消——先完批的
// T1 与账本计数存续，取消批零新增 T1/CloseCycle。
func TestPullLoopCancellationKeepsEarlierCompletedBatch(t *testing.T) {
	session, exit, result := runCancelStage(t, "execute_result", true)
	if result.Outcome != OutcomeInterrupted {
		t.Fatalf("outcome=%q, want interrupted (result=%+v)", result.Outcome, result)
	}
	if session.budget.CompletedCycles != 1 || session.closeCalls != 1 {
		t.Errorf("completed batches=%d close=%d, want 1/1 (prior batch survives)", session.budget.CompletedCycles, session.closeCalls)
	}
	if len(exit.events) != 1 || exit.events[0].TurnID != "run-cancel-execute_result:cycle:1" {
		t.Errorf("prior T1 must survive without new boundaries: %+v", exit.events)
	}
	if result.Cycles != 1 {
		t.Errorf("result cycles=%d, want 1 (host ledger projection)", result.Cycles)
	}
	if len(session.returnedSteps) != 1 || session.returnedSteps[0].Kind != ReturnInterrupted {
		t.Errorf("interrupt step=%+v", session.returnedSteps)
	}
}
