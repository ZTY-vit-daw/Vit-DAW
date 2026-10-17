package chat

import (
	"strings"
)

// AGENT-SENDROUTE-SEMANTICS-1（设计件 MIX-SENDRETURN-DESIGN-1 §2.2）：
// 「给某轨加混响/延迟」类话术的拓扑决策表。决策是启发式（词法+上下文），
// 不是语义理解保证；误判走 PendingPlan 用户确认面拦截（send 结构本身必须
// 经过确认才执行）。四分支：
//  1. insert 默认——单轨+空间类效果器+无共享信号（现状不变）；
//  2. send 结构——话术含共享信号+可解析 ≥2 条源轨；
//  3. agent 主动建议——单轨请求但工程内 ≥2 轨有同类空间诉求（建议可拒，
//     不改变本单轨 insert 计划本身）；
//  4. insert 锁定——用户明说「只给这条轨」，不再建议 send。

// SendTopologyDecision 是拓扑决策表的输出分支。
type SendTopologyDecision string

const (
	// SendDecisionInsertDefault 单轨无共享信号：insert 直挂（现状）。
	SendDecisionInsertDefault SendTopologyDecision = "insert_default"
	// SendDecisionSendStructure 共享话术+≥2 源轨：send 结构计划。
	SendDecisionSendStructure SendTopologyDecision = "send_structure"
	// SendDecisionAgentSuggest 单轨请求+工程侧共享潜力：insert 计划+附建议（可拒）。
	SendDecisionAgentSuggest SendTopologyDecision = "agent_suggest"
	// SendDecisionInsertLocked 用户明说只给这条轨：insert 锁定，不再建议。
	SendDecisionInsertLocked SendTopologyDecision = "insert_locked"
)

// sendShareSignalMarkers 共享/总线发送话术标记（大小写不敏感子串）。
var sendShareSignalMarkers = []string{
	"共享", "共用", "同用", "一起用", "总线发送", "发送总线", "同一条", "同一个空间", "同一空间",
	"多条轨", "几条轨", "都加上", "都送", "share", "shared", "same reverb", "same space",
	"bus send", "send bus", "send them", "common reverb", "common space",
}

// sendLockSignalMarkers insert 锁定话术标记：用户明说只处理这一条轨。
var sendLockSignalMarkers = []string{
	"只给这条轨", "只给这轨", "只给这一轨", "只在这条轨", "只在这轨", "只加在这条", "只挂在这",
	"这条轨自己", "这条轨单独", "不要发送", "不用共享", "别共享",
	"only this track", "just this track", "this track only", "no send", "don't share",
}

// SendTopologyDecisionInput 是决策表纯函数输入。SourceTrackCount 是话术里
// 可解析（按名匹配工程轨）的源轨数；SpatialFamilyPeers 是工程内除目标轨外
// 已挂同类空间效果器的轨数（C3/机架面词法事实，建议分支证据）。
type SendTopologyDecisionInput struct {
	UserText           string
	SourceTrackCount   int
	HasSpatialIntent   bool
	SpatialFamilyPeers int
}

// decidePluginLoadTopology 是拓扑决策表（纯函数，单测四分支直喂）。
// 判定顺序：锁定 > send 结构 > 主动建议 > insert 默认。
func decidePluginLoadTopology(input SendTopologyDecisionInput) SendTopologyDecision {
	if sendTextHasAny(input.UserText, sendLockSignalMarkers) {
		return SendDecisionInsertLocked
	}
	if sendTextHasAny(input.UserText, sendShareSignalMarkers) && input.SourceTrackCount >= 2 {
		return SendDecisionSendStructure
	}
	if input.SourceTrackCount < 2 && input.HasSpatialIntent && input.SpatialFamilyPeers >= 2 {
		return SendDecisionAgentSuggest
	}
	return SendDecisionInsertDefault
}

func sendTextHasAny(text string, markers []string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, marker := range markers {
		if marker != "" && strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// SendShareSignalText 将共享信号命中词如实回传（呈现/日志用，词法命中非语义判定）。
func SendShareSignalText(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, marker := range sendShareSignalMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return marker
		}
	}
	return ""
}

// sendSourceTrackResolution 按话术解析源轨：工程轨名在话术中整词命中
// （大小写不敏感）即计入；返回保持工程轨序。轨道数少于 2 或无命中时返回空。
type sendSourceTrack struct {
	TrackID   string
	TrackName string
}

// sendStateTrackRows 提取工程态轨行（chat 侧本地形态，daw 包只读助手未导出
// 该面且不在本卡文件域）。接受 get_project_state 的 tracks[]。
func sendStateTrackRows(state map[string]any) []map[string]any {
	return mapRowsFromAny(state["tracks"])
}

func resolveSendSourceTracks(userText string, state map[string]any) []sendSourceTrack {
	tracks := sendStateTrackRows(state)
	if len(tracks) == 0 {
		return nil
	}
	lower := strings.ToLower(userText)
	out := make([]sendSourceTrack, 0, 2)
	for _, track := range tracks {
		id := firstStringFromMap(track, "track_id", "id")
		name := firstStringFromMap(track, "track_name", "name", "user_label")
		if id == "" || name == "" {
			continue
		}
		if len(name) >= 2 && strings.Contains(lower, strings.ToLower(name)) {
			out = append(out, sendSourceTrack{TrackID: id, TrackName: name})
		}
	}
	return out
}

// countSpatialFamilyPeers 统计工程内挂有空间类效果器（plugin_path 词法分类，
// 与 C3 词法标记同源）的轨数，排除已计入的源轨——agent 主动建议分支的证据面。
func countSpatialFamilyPeers(state map[string]any, excludeTrackIDs map[string]bool) int {
	count := 0
	for _, track := range sendStateTrackRows(state) {
		id := firstStringFromMap(track, "track_id", "id")
		if id == "" || excludeTrackIDs[id] {
			continue
		}
		for _, node := range mapRowsFromAny(mapValue(track["rack"])["nodes"]) {
			path := firstStringFromMap(node, "plugin_path", "path")
			if path == "" {
				continue
			}
			if c3MatchSpacePluginMarker(path) != "" {
				count++
				break
			}
		}
	}
	return count
}

// sendFXTrackNameForPluginQuery 按插件查询推断 FX 轨缺省名（用户可在计划里改）。
func sendFXTrackNameForPluginQuery(query string) string {
	lower := strings.ToLower(query)
	switch {
	case strings.Contains(lower, "delay") || strings.Contains(lower, "延迟") || strings.Contains(lower, "回声") || strings.Contains(lower, "echo"):
		return "Delay Bus"
	default:
		return "Reverb Bus"
	}
}
