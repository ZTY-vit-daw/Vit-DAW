package pullharness

// disclosure_test.go — 披露位迁移验收（设计 §4.2 表四行，卡面验收 1
// "披露位四行各就位断言"）：
//   行 1 GATE PATH 预披露 + 行 2 G1-G8 证据门指引 → 冷启动底座规则段；
//   行 3 TERMINAL TURN + 行 4 预算指令 → 动态区逐轮状态行。
// 各行以锚短语定位，正置+错位双断言（就位=在指定段、不在他段）。

import (
	"strings"
	"testing"
)

// 四行各就位：行 1/2 在冷启动规则段，行 3/4 在动态区；错位=零。
func TestDisclosureFourRowsInPlace(t *testing.T) {
	rules := coldStartRulesContent()
	if !strings.Contains(rules, anchorGatePath) {
		t.Fatalf("rules section missing GATE PATH row (§4.2 row 1): %q", rules)
	}
	if !strings.Contains(rules, anchorEvidenceGates) {
		t.Fatalf("rules section missing EVIDENCE GATES row (§4.2 row 2): %q", rules)
	}

	budget := ObservationBudget{MaxCycles: 5, MaxProbeCost: 4}
	rows := dynamicStateRows(2, 1.5, budget)
	dynamic := strings.Join(rows, "\n")
	if !strings.Contains(dynamic, anchorTerminalTurn) {
		t.Fatalf("dynamic zone missing TERMINAL TURN row (§4.2 row 3): %q", dynamic)
	}
	if !strings.Contains(dynamic, anchorBudget) {
		t.Fatalf("dynamic zone missing BUDGET row (§4.2 row 4): %q", dynamic)
	}
	if !strings.Contains(dynamic, "budget_state: cycles=2/5") {
		t.Fatalf("dynamic zone missing BudgetState disclosure: %q", dynamic)
	}

	// 错位断言：行 1/2 不逐轮复现（不再进动态区）；行 3/4 不进冷启动段。
	if strings.Contains(dynamic, anchorGatePath) || strings.Contains(dynamic, anchorEvidenceGates) {
		t.Fatalf("cold-start rows leaked into dynamic zone: %q", dynamic)
	}
	if strings.Contains(rules, anchorTerminalTurn) || strings.Contains(rules, anchorBudget) {
		t.Fatalf("dynamic rows leaked into cold start rules: %q", rules)
	}

	// 行 1 语义锚：先补证再提案（ccb.observation_request 既有语义）。
	gatePath := rules[strings.Index(rules, anchorGatePath):]
	for _, fragment := range []string{"ccb.observation_catalog", "ccb.observation_request", "machine-readable structured gap"} {
		if !strings.Contains(gatePath, fragment) {
			t.Fatalf("GATE PATH row missing semantic anchor %q: %q", fragment, gatePath)
		}
	}
	// 行 2 语义锚：G1-G8 准入只由证据门决定（:582 既有语义）。
	gates := rules[strings.Index(rules, anchorEvidenceGates):]
	for _, fragment := range []string{"G1-G8", "Do not rush the proposal", "no default view sequence"} {
		if !strings.Contains(gates, fragment) {
			t.Fatalf("EVIDENCE GATES row missing semantic anchor %q: %q", fragment, gates)
		}
	}
}

// 行 3 机械状态头：剩余循环节计数随轮次/上限变化（:358 counters 形态；
// 不设上限=uncapped）。
func TestTerminalTurnRowMechanicalCounters(t *testing.T) {
	capped := terminalTurnRow(2, ObservationBudget{MaxCycles: 5})
	if !strings.Contains(capped, "tool-call cycles remaining: 3") {
		t.Fatalf("terminal row missing remaining counter: %q", capped)
	}
	uncapped := terminalTurnRow(7, ObservationBudget{})
	if !strings.Contains(uncapped, "tool-call cycles remaining: uncapped") {
		t.Fatalf("terminal row missing uncapped form: %q", uncapped)
	}
	if !strings.Contains(capped, "no tool calls is the terminal turn") {
		t.Fatalf("terminal row missing terminal-turn instruction: %q", capped)
	}
}

// 行 4 落点：BudgetState 披露行在动态区且预警随机械判据出现（§3.4 落点）。
func TestBudgetDisclosureWarningInDynamicZone(t *testing.T) {
	rows := dynamicStateRows(4, 0, ObservationBudget{MaxCycles: 5})
	if !strings.Contains(rows[0], "budget_warning") {
		t.Fatalf("budget disclosure missing near-cap warning: %q", rows[0])
	}
	rows = dynamicStateRows(1, 0, ObservationBudget{MaxCycles: 5})
	if strings.Contains(rows[0], "budget_warning") {
		t.Fatalf("budget disclosure warned below cap: %q", rows[0])
	}
}

// 冷启动底座整体经 RenderColdStart 挂规则段（行 1/2 的实际挂载点）。
func TestDisclosureRowsMountedViaColdStartRender(t *testing.T) {
	base := RenderColdStart(coldStartFixtureEngine())
	var rules *string
	for i, section := range base.Sections {
		if section.ID == ColdStartSectionRules {
			rules = &base.Sections[i].Content
		}
	}
	if rules == nil {
		t.Fatalf("cold start base missing rules section: %v", base.AbsentIDs)
	}
	if !strings.Contains(*rules, anchorGatePath) || !strings.Contains(*rules, anchorEvidenceGates) {
		t.Fatalf("mounted rules section missing disclosure rows: %q", *rules)
	}
}
