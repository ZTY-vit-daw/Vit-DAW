// Package fastpath 承载 agent 循环的确定性快路径（FastPathRouter）。
//
// L1-5-IMPL-C（HARNESS_V1_DESIGN §5）：message_loop 确定性 preflight 族在此归并为
// 统一注册面——注册清单可审计（DefaultEntryNames + Router.Names 完整性契约）、
// 每循环单点尝试（TryMatch 按注册序短路）、CCB diagnostic-only 实验边界语义
// 平移为 Router 的显式开关（SetDiagnosticOnly，判定源仍由宿主传入）。
// 匹配/产出纯函数自 agentloop 逐字平移（行为零变化），状态面执行体留在宿主。
package fastpath

import "context"

// Handler 是注册项的执行体形态，与 agentloop preflight 方法签名一致：
// (ctx, runner, state) → (stopped, result)。stopped=true 表示本轮被该确定性
// 快路径短路，result 为宿主 Result 原样透传（Router 不解释）。
type Handler[RUN any, STATE any, RES any] func(ctx context.Context, runner RUN, state STATE) (stopped bool, result RES)

// Entry 是注册面最小单元：可审计名 + 执行体。
type Entry[RUN any, STATE any, RES any] struct {
	Name    string
	Handler Handler[RUN, STATE, RES]
}

// Router 按注册序逐项尝试的确定性快路径注册面。泛型参数由宿主实例化
// （agentloop 以 *Runner / *runState / Result 实例化；本包不引用宿主类型）。
type Router[RUN any, STATE any, RES any] struct {
	entries        []Entry[RUN, STATE, RES]
	diagnosticOnly bool
}

// NewRouter 以给定注册序构造 Router（注册序=宿主原 preflight 链调用序契约）。
func NewRouter[RUN any, STATE any, RES any](entries ...Entry[RUN, STATE, RES]) *Router[RUN, STATE, RES] {
	return &Router[RUN, STATE, RES]{entries: append([]Entry[RUN, STATE, RES](nil), entries...)}
}

// Register 追加一个注册项（保持调用序）。
func (rt *Router[RUN, STATE, RES]) Register(entry Entry[RUN, STATE, RES]) {
	rt.entries = append(rt.entries, entry)
}

// Names 返回当前注册清单（注册序），供完整性断言与审计。
func (rt *Router[RUN, STATE, RES]) Names() []string {
	names := make([]string, 0, len(rt.entries))
	for _, e := range rt.entries {
		names = append(names, e.Name)
	}
	return names
}

// SetDiagnosticOnly 设置 diagnostic-only 旁路开关。true 时 TryMatch 整体旁路
// （零处理机调用）——CCB 实验边界语义：普通确定性 preflight（尤其 project.state）
// 会绕过该边界、在模型观察前制造终态。判定源由宿主每轮传入，Router 不自行判定。
func (rt *Router[RUN, STATE, RES]) SetDiagnosticOnly(v bool) { rt.diagnosticOnly = v }

// DiagnosticOnly 返回当前旁路开关状态。
func (rt *Router[RUN, STATE, RES]) DiagnosticOnly() bool { return rt.diagnosticOnly }

// TryMatch 按注册序逐项尝试，首个 stopped=true 的命中即短路返回（命中项名
// 一并返回，供遥测/审计）；全部未命中返回 (false, 零值, "")。
// diagnostic-only 开启时整体旁路，返回未命中且不调用任何 Handler。
func (rt *Router[RUN, STATE, RES]) TryMatch(ctx context.Context, runner RUN, state STATE) (stopped bool, result RES, entry string) {
	var zero RES
	if rt.diagnosticOnly {
		return false, zero, ""
	}
	for _, e := range rt.entries {
		if e.Handler == nil {
			continue
		}
		if hit, res := e.Handler(ctx, runner, state); hit {
			return true, res, e.Name
		}
	}
	return false, zero, ""
}

// DefaultEntryNames 是 L1-5-IMPL-C 平移时的规范注册清单（顺序=原 agentloop
// message_loop.go loop() preflight 链调用序，清单见 HARNESS_V1_DESIGN §1.4/§5）。
// 宿主注册面以此为完整性契约：注册漂移（遗漏/多余/乱序）即显式失败。
var DefaultEntryNames = []string{
	"static_mix_capability_contract",
	"project_blackboard_status",
	"clip_fade_gain_set",
	"clip_fade_gain_read",
	"strip_silence_suggest",
	"clip_range_split",
	"stems_folder_import",
	"pending_section_markers_apply",
	"natural_mix_observation",
	"static_mix_gain_staging_context_pack",
}
