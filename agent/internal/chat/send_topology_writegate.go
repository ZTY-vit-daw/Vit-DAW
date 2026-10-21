package chat

import (
	"context"
	"fmt"
	"strings"
)

// TIMEFX-BUS-WRITEGATE-1（146 设计稿 §二.1 硬写门）：总线拓扑计划的确认/
// 执行期拓扑校验。提案引用的源轨必须存在于观察披露（当前工程态轨集）；
// reuse 形态的 send 目标轨必须存在；create 形态的 FX 轨属于本提案新建清单
// （内核 connect 按名幂等创建），不要求已存在。任一引用不在观察披露且不在
// 新建清单 = 幻觉拓扑，拒载——不派发任何内核命令。观察披露不可读 = 诚实
// 拒载（fail-closed，不虚构通过）。K2 显式 send 工作流（send_route_workflow.go
// 拦截面）不在本门范围：其源轨按构造来自真实态解析，语义零改变（卡面红线）。

// semanticTopologyObservedState 读取写门用的观察披露：优先内核实时
// get_project_state（与语义处理实例面同一读取模式），失败回落 harness
// 状态摘要。两形态都以 tracks[] 携带在册轨行。
func (s *Server) semanticTopologyObservedState(ctx context.Context) map[string]any {
	if s == nil || s.harness == nil {
		return nil
	}
	state := s.harness.UserStateSummary(ctx)
	if client := s.eqKernelClient(); client != nil {
		if liveState, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_project_state"}); err == nil && kernelReplyOK(liveState) {
			state = liveState
		}
	}
	return state
}

// observedTrackIDSet 从观察披露提取在册轨 ID 集。
func observedTrackIDSet(state map[string]any) map[string]bool {
	ids := map[string]bool{}
	for _, track := range sendStateTrackRows(state) {
		if id := firstStringFromMap(track, "track_id", "id"); id != "" {
			ids[id] = true
		}
	}
	return ids
}

// semanticTopologyWriteGateIssue 校验 bus 拓扑计划的引用面（纯函数，单测
// 直喂）。返回 nil = 放行；非 nil = 幻觉/不可核校引用，返回含具体 ID 的
// 拒载理由。非 bus 计划（inline 例外走既有单轨装载面）零接触。
func semanticTopologyWriteGateIssue(plan *semanticTopologyPlan, observedState map[string]any) error {
	if plan == nil || plan.Mode != semanticTopologyModeBus {
		return nil
	}
	observed := observedTrackIDSet(observedState)
	if len(observed) == 0 {
		return fmt.Errorf("拓扑写门拒载：观察披露不可读或工程无轨，无法核校总线拓扑引用；没有执行任何连接或装载")
	}
	unknownSources := []string{}
	for _, send := range plan.Sends {
		source := strings.TrimSpace(send.SourceTrackID)
		if source == "" || !observed[source] {
			unknownSources = append(unknownSources, firstNonEmpty(source, "(空源轨 ID)"))
		}
	}
	unknownTargets := []string{}
	if plan.FXTrack.Action == semanticTopologyFXActionReuse {
		fxID := strings.TrimSpace(plan.FXTrack.TrackID)
		if fxID == "" || !observed[fxID] {
			unknownTargets = append(unknownTargets, firstNonEmpty(fxID, "(reuse 缺 fx_track_id)"))
		}
	}
	if len(unknownSources) > 0 {
		return fmt.Errorf("拓扑写门拒载：源轨 [ %s ] 不在观察披露中（幻觉拓扑，仅本提案新建 FX 轨可豁免）；没有执行任何连接或装载", strings.Join(unknownSources, ", "))
	}
	if len(unknownTargets) > 0 {
		return fmt.Errorf("拓扑写门拒载：send 目标 FX 轨 [ %s ] 不在观察披露中（幻觉拓扑，reuse 必须指向既有轨）；没有执行任何连接或装载", strings.Join(unknownTargets, ", "))
	}
	return nil
}
