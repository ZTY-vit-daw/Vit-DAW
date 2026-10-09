package pullharness

// coldstart.go — 冷启动底座（HARNESS_V1_DESIGN §4.1，L1-5-IMPL-B）。
//
// 会话首装一次性 session 层 Section（之后字节恒定）：三事实组与 L4 genesis
// 头部段（harness/project_genesis.go，L4-GENESIS-1）同源——经既有只读消费
// 面重渲染。不复制 harness 包私有函数、不改 harness 包（文本化在本接线层
// 完成，与 genesis 接线层同责同款投影纪律）：
//   - TOM 概览 = capabilitycontext.BuildProjectTOMProjection（chat 侧既有
//     消费面）的 llm_context.summary_md + full_assignment_manifest 摘要行；
//   - 交付目标 = rlm.BuiltinDeliveryProfiles（内建引用面；工程态 render→
//     profile 绑定快照面归 IMPL-D 接线时供给）；
//   - 总线拓扑 = 同源内核快照 parent/routing_bus/vit_type 文本化。
//
// 缺失投影 → 该事实组 absent 不臆造（L4-GENESIS-1 投影纪律同款）：缺席组
// 不渲染 Section，经 PrefixRequest.LayerStates 声明进装配报告（§2.0
// fail-open 但显式）。
//
// 披露位迁移落点（§4.2 表行 1/2）：GATE PATH 预披露与 G1-G8 证据门指引进
// 底座规则段（语义沿 ccb_model_prompt.go:220/:273/:582 既有文本，旧
// harness 零改动）——一次性指引，不再逐轮复现；文本见 disclosure.go。
//
// 内容盲（TIMING-2 / §8.1）：底座段审查键 ColdStartSectionPurpose 封闭
// 枚举（RuleSection.Purpose 机械键思路同 T-C1，本包等价断言）；域处理
// 知识（push 侧 Pattern Recognition 段一族）不迁移——机械断言见
// coldstart_blind_test.go。
//
// 摘要行封顶与 genesis 接线层同款（防前缀膨胀；超限注记 "+N more"，不
// 静默丢）。

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/rlm"
)

// 冷启动底座 Section ID（稳定段身份；LayerStates 声明与报告定位键）。
const (
	ColdStartSectionTOM      = "pullharness.coldstart.tom"
	ColdStartSectionBus      = "pullharness.coldstart.bus"
	ColdStartSectionDelivery = "pullharness.coldstart.delivery"
	ColdStartSectionRules    = "pullharness.coldstart.rules"
)

// 摘要行封顶（与 genesis 接线层同量级）。
const (
	coldStartMaxTrackLines    = 32
	coldStartMaxDeliveryLines = 8
	coldStartMaxRouteEdges    = 32
)

// coldStartLayerAbsent 是缺席事实组的 LayerStates 声明值（§2.0 显式缺席）。
const coldStartLayerAbsent = "absent"

// ColdStartSectionPurpose 是底座段的内容盲审查键（§8.1 RuleSection.Purpose
// 思路，封闭枚举）。facts=机械事实段（投影只读渲染）；discipline/catalog/
// output_format 与 ruleset 包三值同义（规则段只用前两值）。
type ColdStartSectionPurpose string

const (
	ColdStartPurposeFacts        ColdStartSectionPurpose = "facts"
	ColdStartPurposeDiscipline   ColdStartSectionPurpose = "discipline"
	ColdStartPurposeCatalog      ColdStartSectionPurpose = "catalog"
	ColdStartPurposeOutputFormat ColdStartSectionPurpose = "output_format"
)

// Valid 报告审查键是否在封闭枚举内（等价断言的机械面）。
func (p ColdStartSectionPurpose) Valid() bool {
	switch p {
	case ColdStartPurposeFacts, ColdStartPurposeDiscipline,
		ColdStartPurposeCatalog, ColdStartPurposeOutputFormat:
		return true
	}
	return false
}

// coldStartSectionPurpose 返回段 ID 的审查键；未知段 ID 返回空（无效值，
// 由内容盲断言拦截）。
func coldStartSectionPurpose(id string) ColdStartSectionPurpose {
	switch id {
	case ColdStartSectionTOM, ColdStartSectionBus, ColdStartSectionDelivery:
		return ColdStartPurposeFacts
	case ColdStartSectionRules:
		return ColdStartPurposeDiscipline
	}
	return ""
}

// ColdStartSource 是底座装配的只读输入缝：内核项目状态快照（与 TOM 消费
// 面同源的 engine_snapshot 面；harness 侧由 shadow.Snapshot
// ["engine_snapshot"] 供给，pull 侧真实接线归 IMPL-D）。nil 快照=三事实组
// 按 absent 处理。
type ColdStartSource interface {
	ColdStartEngineSnapshot() map[string]any
}

// ColdStartSourceFunc 适配函数形态的 ColdStartSource。
type ColdStartSourceFunc func() map[string]any

// ColdStartEngineSnapshot 实现 ColdStartSource。
func (f ColdStartSourceFunc) ColdStartEngineSnapshot() map[string]any {
	if f == nil {
		return nil
	}
	return f()
}

// ColdStartBase 是一次首装的底座产物：Sections 按 TOM→总线→交付→规则
// 固定序（层序固定精神，T-C1）；AbsentIDs 是缺席事实组的声明清单
// （LayerStates "absent"）。规则段与快照无关，恒在。
type ColdStartBase struct {
	Sections  []promptruntime.Section
	AbsentIDs []string
}

// LayerStates 把缺席声明转成 PrefixRequest.LayerStates 形态（空缺席=空表）。
func (b ColdStartBase) LayerStates() map[string]string {
	if len(b.AbsentIDs) == 0 {
		return nil
	}
	states := make(map[string]string, len(b.AbsentIDs))
	for _, id := range b.AbsentIDs {
		states[id] = coldStartLayerAbsent
	}
	return states
}

// RenderColdStart 从内核项目状态快照渲染冷启动底座（确定性：同输入同
// 字节——消费面均为数组序/固定目录序，无 map 迭代序与时间戳进段）。
func RenderColdStart(engine map[string]any) ColdStartBase {
	base := ColdStartBase{}
	if lines := coldStartTOMLines(engine); len(lines) > 0 {
		base.Sections = append(base.Sections, promptruntime.TextSection(
			promptruntime.SectionSession, ColdStartSectionTOM,
			"cold start — project TOM overview", strings.Join(lines, "\n"), true))
	} else {
		base.AbsentIDs = append(base.AbsentIDs, ColdStartSectionTOM)
	}
	if topology := coldStartBusTopology(engine); topology != "" {
		base.Sections = append(base.Sections, promptruntime.TextSection(
			promptruntime.SectionSession, ColdStartSectionBus,
			"cold start — bus topology", topology, true))
	} else {
		base.AbsentIDs = append(base.AbsentIDs, ColdStartSectionBus)
	}
	if lines := coldStartDeliveryLines(); len(lines) > 0 {
		base.Sections = append(base.Sections, promptruntime.TextSection(
			promptruntime.SectionSession, ColdStartSectionDelivery,
			"cold start — delivery targets", strings.Join(lines, "\n"), true))
	} else {
		base.AbsentIDs = append(base.AbsentIDs, ColdStartSectionDelivery)
	}
	base.Sections = append(base.Sections, promptruntime.TextSection(
		promptruntime.SectionSession, ColdStartSectionRules,
		"cold start — admission gates & gate path (standing rule disclosure)",
		coldStartRulesContent(), true))
	return base
}

// coldStartTOMLines 消费 TOM 投影的披露面：概览行（llm_context.summary_md）
// + 逐轨 "name → group [confidence]" 行（full_assignment_manifest 既序列，
// 不回读 raw rows）。
func coldStartTOMLines(engine map[string]any) []string {
	if len(engine) == 0 {
		return nil
	}
	projection := capabilitycontext.BuildProjectTOMProjection(engine, "", "")
	if len(projection) == 0 {
		return nil
	}
	lines := make([]string, 0, 8)
	if llmContext, ok := projection["llm_context"].(map[string]any); ok {
		if summary := coldStartFirstText(llmContext, "summary_md"); summary != "" {
			lines = append(lines, summary)
		}
	}
	trackLines := coldStartManifestTrackLines(projection)
	if len(lines)+len(trackLines) > coldStartMaxTrackLines {
		kept := coldStartMaxTrackLines - len(lines)
		if kept < 0 {
			kept = 0
		}
		trackLines = append(trackLines[:kept:kept],
			fmt.Sprintf("…+%d more", len(trackLines)-kept))
	}
	lines = append(lines, trackLines...)
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// coldStartManifestTrackLines 从 TOM full_assignment_manifest 抽逐轨摘要
// 行（投影披露面内的既序列化行；组序/行内序即快照序，确定性）。
func coldStartManifestTrackLines(projection map[string]any) []string {
	manifest, ok := projection["full_assignment_manifest"].(map[string]any)
	if !ok {
		return nil
	}
	groups, ok := manifest["groups"].([]any)
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(groups))
	for _, groupAny := range groups {
		group, ok := groupAny.(map[string]any)
		if !ok {
			continue
		}
		label := coldStartFirstText(group, "label")
		assignments, _ := group["assignments"].([]any)
		for _, assignmentAny := range assignments {
			assignment, ok := assignmentAny.(map[string]any)
			if !ok {
				continue
			}
			name := coldStartFirstText(assignment, "track_name", "track_id")
			if name == "" {
				continue
			}
			line := name
			if label != "" {
				line += " → " + label
			}
			if confidence := coldStartFirstText(assignment, "confidence"); confidence != "" {
				line += " [" + confidence + "]"
			}
			lines = append(lines, line)
			if len(lines) >= coldStartMaxTrackLines {
				return lines
			}
		}
	}
	return lines
}

// coldStartBusTopology 把内核项目状态快照的轨道父路由文本化成一行
// （tracks 计数 + child→parent 边 + 总线/子混标记；tracks 数组序即输出序，
// 确定性）。无轨道/无路由字段=留空（absent，不臆造）。
func coldStartBusTopology(engine map[string]any) string {
	tracks := coldStartTrackRows(engine)
	if len(tracks) == 0 {
		return ""
	}
	nameByID := make(map[string]string, len(tracks))
	for _, track := range tracks {
		if id := coldStartFirstText(track, "track_id", "id"); id != "" {
			nameByID[id] = coldStartTrackName(track)
		}
	}
	edges := make([]string, 0, len(tracks))
	overflow := 0
	var buses []string
	for _, track := range tracks {
		name := coldStartTrackName(track)
		if name == "" {
			continue
		}
		if coldStartTrackIsBus(track) {
			buses = append(buses, name)
		}
		parentID := coldStartFirstText(track, "parent_track_id", "parent_folder_track_id")
		parent, known := nameByID[parentID]
		if parentID == "" || !known || parent == name {
			continue
		}
		if len(edges) >= coldStartMaxRouteEdges {
			overflow++
			continue
		}
		edges = append(edges, name+"→"+parent)
	}
	if len(edges) == 0 && len(buses) == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("tracks=%d", len(tracks))}
	if len(edges) > 0 {
		parts = append(parts, "routes: "+strings.Join(edges, ", "))
	}
	if overflow > 0 {
		parts = append(parts, fmt.Sprintf("…+%d more", overflow))
	}
	if len(buses) > 0 {
		parts = append(parts, "bus/submix: "+strings.Join(buses, ", "))
	}
	return strings.Join(parts, "; ")
}

// coldStartTrackIsBus 汇总线语义标记（vit_type=bus / 子混文件夹 / 路由
// 总线开关——与内核快照同源字段面）。
func coldStartTrackIsBus(track map[string]any) bool {
	if strings.EqualFold(coldStartFirstText(track, "vit_type"), "bus") {
		return true
	}
	for _, key := range []string{"is_submix_folder", "routing_bus_enabled"} {
		if value, ok := track[key].(bool); ok && value {
			return true
		}
	}
	return false
}

func coldStartTrackName(track map[string]any) string {
	return coldStartFirstText(track, "track_name", "name", "track_id", "id")
}

func coldStartTrackRows(engine map[string]any) []map[string]any {
	raw, ok := engine["tracks"].([]any)
	if !ok {
		return nil
	}
	rows := make([]map[string]any, 0, len(raw))
	for _, rowAny := range raw {
		if row, ok := rowAny.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

// coldStartDeliveryLines 渲染 RLM 内建交付 profile 引用行（固定目录序，
// 封顶截断）。工程态绑定面（render→profile）归 IMPL-D 接线供给，不在
// 本渲染面。
func coldStartDeliveryLines() []string {
	profiles := rlm.BuiltinDeliveryProfiles()
	lines := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		if len(lines) >= coldStartMaxDeliveryLines {
			lines = append(lines, fmt.Sprintf("…+%d more", len(profiles)-coldStartMaxDeliveryLines))
			break
		}
		lines = append(lines, coldStartDeliveryProfileLine(profile))
	}
	return lines
}

func coldStartDeliveryProfileLine(profile rlm.DeliveryProfile) string {
	line := profile.ProfileID
	if profile.DisplayName != "" {
		line += " (" + profile.DisplayName + ")"
	}
	if profile.Integrated.Max != 0 {
		if profile.Integrated.Min != nil {
			line += fmt.Sprintf(" %g…%g LUFS", *profile.Integrated.Min, profile.Integrated.Max)
		} else {
			line += fmt.Sprintf(" ≤%g LUFS", profile.Integrated.Max)
		}
	}
	if profile.TruePeakMaxDBTP != 0 {
		line += fmt.Sprintf(" ≤%gdBTP", profile.TruePeakMaxDBTP)
	}
	return line
}

func coldStartFirstText(source map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := source[key].(string)
		if !ok {
			continue
		}
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}
