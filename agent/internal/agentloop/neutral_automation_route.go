package agentloop

import (
	"strings"

	"vit-daw-agent/internal/promptruntime"
)

// M3-LLM-NEUTRAL-EMBED-1（220 停止条件 2 部署缺陷修复）：词表未命中的
// 变体话术进 message loop 走 neutral family selection 装配，该装配的
// system prompt 原先不含 217 的 automation 路由指引行——模型从未收到
// 指引（220 render 逐行证据：coord/runs/M3-C4-AUTO-LLM-ROUTE-VERIFY-1/
// llmroute_20261030_{1,2}）。本段把该指引行以独立 Section 追加进 neutral
// 装配：文本与 chat 族 ruleset 段 chat.discipline.plugins_mixing 内的
// 该行逐字同源同文（不新写文案；同源由
// TestNeutralFamilyAutomationRouteMatchesChatRulesetLine 持续锁定——
// chat 段文本变更时该测试红，强制本常量同步）。判断权冻结令 2026-10-30：
// 本卡只让既有指引到达 LLM，不新增判断类 Go 固定话术。
const neutralFamilyAutomationRouteGuidance = `For parameter automation envelope requests such as riding a parameter up or down over a time range, use automation.get_curve, automation.set_points, and automation.clear_region. Resolve plugin_id and param_id first: external plugin parameters, or the track's built-in Volume & Pan plugin (plugin type=volume) for track-level volume/pan rides. Read the parameter value range with get_plugin_parameters before writing, keep every point value inside that range, and read back with automation.get_curve after the write. Clip-edge fade in/out (clip boundary fades) is clip.fade.set, not parameter automation: route clip-boundary fades to clip.fade and time-range parameter envelopes to automation.*.`

// neutralFamilyAutomationRouteSectionID 是指引段的稳定 Section ID
// （进 render 取证与层序诊断面）。
const neutralFamilyAutomationRouteSectionID = "message_loop_neutral_family_automation_route"

// neutralFamilyAutomationRouteCacheKey 指引段内容指纹自治：文本变更时必须
// 改版本号（与 ruleset 版本纪律同向，避免 CacheKey 与内容漂移）。
const neutralFamilyAutomationRouteCacheKey = "neutral-family-automation-route:v1"

// neutralFamilyAutomationRouteSection 构造指引段。Stable=true：文本跨轮
// 字节恒定（T-A3 骨架恒定要求），进稳定前缀面；Kind=SectionStatic。
// 挂载位形对齐 carriers 侧 semantic_topology_convention 先例（L1 域邻位
// 的自治惯例段），但本段落在 agentloop 装配组装层——卡面文件域内的
// 最小侵入面（M3-LLM-NEUTRAL-EMBED-1 勘察定案）。
func neutralFamilyAutomationRouteSection() promptruntime.Section {
	return promptruntime.Section{
		ID:       neutralFamilyAutomationRouteSectionID,
		Kind:     promptruntime.SectionStatic,
		Title:    "",
		Content:  strings.TrimSpace(neutralFamilyAutomationRouteGuidance),
		CacheKey: neutralFamilyAutomationRouteCacheKey,
		Stable:   true,
	}
}
