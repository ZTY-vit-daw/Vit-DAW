package agentloop

// neutral_automation_route_test.go — M3-LLM-NEUTRAL-EMBED-1（222）：
// neutral family selection 装配分支的 automation 路由指引段。
// 三个证明面：
//  1. TestNeutralFamilyAutomationRouteMatchesChatRulesetLine——同源同文
//     锁定：指引常量与 chat 族 ruleset 段 chat.discipline.plugins_mixing
//     内该行逐字一致（chat 段文本变更时红，强制同步；不新写文案的执法面）。
//  2. TestNeutralFamilyAssemblyCarriesAutomationRoute——生产装配面在场：
//     bundle 主路径渲染的 system 消息含指引行，且指引段在 L1 域之后。
//  3. TestOrdinaryAssemblyUnaffectedByNeutralAutomationRoute——确认话术
//     分支（message_loop_system 完整装配）零回退：普通族装配不含该段。

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/rules/ruleset"
)

// TestNeutralFamilyAutomationRouteMatchesChatRulesetLine：指引常量逐字节
// 等于 chat 族 chat.discipline.plugins_mixing 段内的整行（217 既有 embed
// 文本；装载面已做 CRLF→LF 归一，按行比对）。
func TestNeutralFamilyAutomationRouteMatchesChatRulesetLine(t *testing.T) {
	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	found := false
	for _, section := range result.Manifest.Sections {
		if section.SectionID != "chat.discipline.plugins_mixing" {
			continue
		}
		found = true
		for _, line := range strings.Split(section.Content, "\n") {
			if strings.TrimSpace(line) == neutralFamilyAutomationRouteSection().Content {
				return
			}
		}
		t.Fatalf("chat.discipline.plugins_mixing no longer carries the automation route line byte-for-byte; neutral guidance constant must be re-synced (same-source lock)")
	}
	if !found {
		t.Fatalf("chat.discipline.plugins_mixing section missing from ruleset manifest")
	}
}

// TestNeutralFamilyAssemblyCarriesAutomationRoute：真实 bundle 主路径
// （rules/profile/env/ledger/catalog 层序）渲染出的 system 消息必须携带
// 指引行——220 停止条件 2 的缺陷形态（模型从未收到指引）不复现。
func TestNeutralFamilyAssemblyCarriesAutomationRoute(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	loop := &MessageLoop{carrierWorkspaceDir: t.TempDir()}
	input := loop.neutralFamilyAssemblyInput(state, "{}")

	system := ""
	sawRoute := false
	sawPostL1 := false
	for _, section := range input.SystemSections {
		if section.ID == neutralFamilyAutomationRouteSectionID {
			sawRoute = true
		} else if sawRoute {
			sawPostL1 = true
		}
		system += section.Content + "\n\n"
	}
	if !sawRoute {
		t.Fatalf("neutral family assembly missing the automation route section (220 deployment defect regression)")
	}
	if !sawPostL1 {
		t.Fatalf("automation route section must be followed by at least the catalog layer, got section order %v", sectionIDs(input.SystemSections))
	}
	if !strings.Contains(system, neutralFamilyAutomationRouteSection().Content) {
		t.Fatalf("neutral family system text lacks the automation route guidance line")
	}
	if !strings.Contains(system, "automation.set_points") || !strings.Contains(system, "automation.get_curve") {
		t.Fatalf("neutral family system text lacks the automation tool names")
	}
}

// TestOrdinaryAssemblyUnaffectedByNeutralAutomationRoute：确认话术分支
// （message_loop_system 完整装配，free-state 不活跃）零回退——装配不含
// 指引段（普通族的 automation 面来自其自身 tool catalog），形态不被本卡
// 改动。
func TestOrdinaryAssemblyUnaffectedByNeutralAutomationRoute(t *testing.T) {
	ordinary := &runState{
		input:  Input{UserText: "确认写入", Context: map[string]any{}, AllowedTools: []string{"automation.get_curve", "automation.set_points"}},
		goal:   goalForNeutralFamilyTest,
		budget: Budget{MaxToolCalls: 12},
	}
	if messageLoopNeedsNeutralFamilyProjection(ordinary) {
		t.Fatalf("ordinary state unexpectedly classified as neutral family projection")
	}
	loop := &MessageLoop{}
	input := loop.assemblyInput(ordinary, "{}")
	for _, section := range input.SystemSections {
		if section.ID == neutralFamilyAutomationRouteSectionID {
			t.Fatalf("ordinary message_loop_system assembly must not carry the neutral-only automation route section")
		}
	}
	if len(input.SystemSections) == 0 {
		t.Fatalf("ordinary assembly rendered no system sections")
	}
}

func sectionIDs(sections []promptruntime.Section) []string {
	ids := make([]string, 0, len(sections))
	for _, section := range sections {
		ids = append(ids, section.ID)
	}
	return ids
}
