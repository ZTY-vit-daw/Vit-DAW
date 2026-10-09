package pullharness

// exit_wiring.go — 真实退场接线（HARNESS_V1_DESIGN §3.1 步骤⑥/[T2] 与
// §2 触发点映射，L1-5-IMPL-B）。A 卡经 fake ExitExecutor 验证循环时序；
// 本文件落三件事：
//
//  1. TurnBoundaryEvent.Window/Budget 构造落位（A 阶段散在 loop.go 的
//     内联构造收拢为本文件两个构造函数，可测面）：T1=批轮事件
//    （TurnID=<RunID>:cycle:<n>，单元=工具结果行），T2=终局事件
//    （TurnID=RunID，沿 agentloop/exit_retain.go 先例；窗面=run 累计
//     工具单元）。
//  2. 生产注入面：NewWiredExitExecutor 返回真实
//     contextruntime.ExitExecutor（既有机械判据零新造），依赖经
//     ExitExecutorConfig 注入（CAS 例外路径写入器/WARN 遥测/时钟）。
//  3. 结论级 retain 供给通道：ToolResult.RetainedStatement（上游显式
//     提供，WindowUnit 同名语义）——T1 批轮内观察结论经此进三态判据；
//     报告消费（WriteRetains 落盘/遥测）维持 advisory 形态，生产消费
//     切换归 IMPL-D（exit_executor.go 头注先例）。
//
// 判据零新造：三态（retain/ref/drop）与四判据（turn_end/semantic_expiry/
// window_slide/budget）全部是 contextruntime 既有机械规则，本文件只构造
// 事件输入（HistoryLimit/HotLimitBytes 两个线位经 ExitWiring 落位；<=0
// =不设线，A 阶段缺省形态不变）。

import (
	"time"

	"vit-daw-agent/internal/contextruntime"
)

// ExitWiring 是边界事件构造的线位配置（零值=不设线，A 阶段形态）。
type ExitWiring struct {
	// HistoryLimit >0：WindowState.HistoryLimit（history_message 超限部分
	// 为 window_slide 候选——chat limit=12 既有先例）。pull v1 工具单元
	// 不含 history_message，线位预留。
	HistoryLimit int
	// HotLimitBytes >0：BudgetState.HotLimitBytes（动态窗字节线，budget
	// 判据；宁可超预算不丢证据链的执法面）。
	HotLimitBytes int64
	// Now 边界事件时间戳注入（确定性测试）；nil=time.Now。
	Now func() time.Time
}

func (w ExitWiring) now() time.Time {
	if w.Now != nil {
		return w.Now()
	}
	return time.Now()
}

// NewWiredExitExecutor 生产构造：真实 contextruntime.ExitExecutor（既有
// 机械判据零新造）。PullLoop 消费方（IMPL-D 双模入口接线）经此注入，
// 不再经测试 fake。
func NewWiredExitExecutor(cfg contextruntime.ExitExecutorConfig) contextruntime.ExitExecutor {
	return contextruntime.NewExitExecutor(cfg)
}

// BatchTurnEvent 构造单批 T1 事件（§2 触发点映射：工具批执行完毕 →
// OnTurnBoundary，每循环恰一次）：单元=工具结果行（TurnID 标本批轮次 →
// turn_end 判据按既有机械规则触发），热字节=批字节合计；线位经
// ExitWiring。返回事件与单元/字节（终局 T2 窗面的累计输入）。
func BatchTurnEvent(turnID string, batch []ToolResult, wiring ExitWiring) (contextruntime.TurnBoundaryEvent, []contextruntime.WindowUnit, int64) {
	units := make([]contextruntime.WindowUnit, 0, len(batch))
	var hotBytes int64
	for _, toolResult := range batch {
		units = append(units, contextruntime.WindowUnit{
			Unit: contextruntime.ExitUnit{
				Kind: contextruntime.ExitUnitToolResult,
				ID:   toolResult.ID,
			},
			Bytes:             toolResult.Bytes,
			TurnID:            turnID,
			HandleRef:         toolResult.HandleRef,
			EvidenceRefs:      toolResult.EvidenceRefs,
			RetainedStatement: toolResult.RetainedStatement,
		})
		hotBytes += toolResult.Bytes
	}
	event := contextruntime.TurnBoundaryEvent{
		TurnID: turnID,
		Window: contextruntime.WindowState{Units: units, HistoryLimit: wiring.HistoryLimit},
		Budget: contextruntime.BudgetState{HotBytes: hotBytes, HotLimitBytes: wiring.HotLimitBytes},
		Now:    wiring.now(),
	}
	return event, units, hotBytes
}

// FinalTurnEvent 构造终局 T2 事件（TurnID=RunID，exit_retain 先例；窗面
// =run 累计工具单元）。单元切片复制（事件与累计窗所有权分离）。
func FinalTurnEvent(runID string, units []contextruntime.WindowUnit, hotBytes int64, wiring ExitWiring) contextruntime.TurnBoundaryEvent {
	return contextruntime.TurnBoundaryEvent{
		TurnID: runID,
		Window: contextruntime.WindowState{
			Units:        append([]contextruntime.WindowUnit(nil), units...),
			HistoryLimit: wiring.HistoryLimit,
		},
		Budget: contextruntime.BudgetState{HotBytes: hotBytes, HotLimitBytes: wiring.HotLimitBytes},
		Now:    wiring.now(),
	}
}
