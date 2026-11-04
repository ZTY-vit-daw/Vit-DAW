package agentloop

// ccb_model_prompt_terminal_route_test.go — M3-TERMINAL-ROUTE-WORDING-1
//（246）：终局轮直调形态补全四项措辞的锁面。
//  1. TestTerminalTurnSentenceCarriesDirectCallShape——项一：终局句三选
//     新形态锚+旧二选句负断言（freeStateTerminalTurnSentence 与其渲染面）。
//  2. TestTimeVaryingGuidanceCarriesShapeExampleAndBridge——项二/项三：v3
//     段时变形态示例锚（直调轮+读回终态轮）+确认语义桥接句锚；保守负断
//     言=不教 final:true+tool_calls 同轮组合（normalizeMessageLoopOutput 对
//     final:true 清空 tool_calls——照抄该形态=写入静默丢失，见卡回执偏差
//     记录）。
//  3. TestConvergenceGuidanceCarriesTimeVaryingExemption——项四：目标级规
//     则句时变豁免子句锚+旧句直断负断言（244 报告 §2.2 #3 的正面对撞消除）。
// 同步面（ruleset sections/chat server.go fallback 的 v2 触发权句原文）本卡
// 未触碰：由既有 parity 测试（TestNeutralFamilyAutomationRouteMatches-
// ChatRulesetLine / TestSkeletonMutationRuleCarriesGovernanceGateSemantics）
// 持续锁定，跑绿即四面同步证明。

import (
	"strings"
	"testing"
)

// TestTerminalTurnSentenceCarriesDirectCallShape：项一——终局二选句升级为
// 三选，第三形态（全写要素在手→直接 set_points 直调，治理门自动拦卡）与
// v2 触发权句同语义；被顶掉的旧二选精确形态必须消失。
func TestTerminalTurnSentenceCarriesDirectCallShape(t *testing.T) {
	sentence := freeStateTerminalTurnSentence
	// 新形态锚：直调形态+治理门语义与 v2 句同源。
	for _, anchor := range []string{
		"a direct automation.set_points mutation call when every write element (target track, parameter, in-range values, time range) is already in hand",
		"the runtime governance gate pauses it into a user confirmation card automatically",
	} {
		if !strings.Contains(sentence, anchor) {
			t.Fatalf("terminal sentence v3 missing direct-call anchor %q: %q", anchor, sentence)
		}
	}
	// 旧二选句负断言：proposal-or-blocked 的二选连接形态不再在场（244 §2.2 #1
	// 的终局锁死形态）。
	if strings.Contains(sentence, "a bounded improvement proposal, or an explicit terminal statement") {
		t.Fatalf("terminal sentence still carries the two-choice lock shape (244 §2.2 #1): %q", sentence)
	}
	// 渲染面：locked 轮 directive 携带三选句（含预算头）。
	state := boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["terminal_turn_locked"] = true
	}), nil)
	directive := messageLoopFreeStateTerminalTurnDirective(state)
	if !strings.Contains(directive, "a direct automation.set_points mutation call when every write element") {
		t.Fatalf("locked-turn directive missing the direct-call shape: %q", directive)
	}
	// 终局 output-gate bounce 文本同源（messageLoopFreeStateTerminalTurnIssue
	// 返回同一常量）：三选句在 bounce 面同文。
	issue := messageLoopFreeStateTerminalTurnIssue(state, messageLoopOutput{})
	if issue != freeStateTerminalTurnSentence {
		t.Fatalf("terminal bounce text must stay same-source with the directive sentence, got %q", issue)
	}
}

// TestTimeVaryingGuidanceCarriesShapeExampleAndBridge：项二/项三——v3 段补
// 时变形态示例（消除「唯一 final 示例=静态提案」邻接缺陷，244 §2.2 #6）与
// 确认语义桥接句（§2.3 #3 双确认语义无桥）。
func TestTimeVaryingGuidanceCarriesShapeExampleAndBridge(t *testing.T) {
	guidance := freeStateTimeVaryingActionPathGuidance()
	// 项二示例锚：直调轮（final:false 工具调用轮）+读回终态轮（final:true）。
	for _, anchor := range []string{
		`"tool":"automation.set_points"`,
		`"plugin_id":"<plugin id resolved from plugin.get_parameters>"`,
		`"param_id":"<parameter id>"`,
		`"points":[{"t":0,"v":<value inside the read range>}`,
		`"final":false`,
		`{"final":true,"reply":"short readback note","free_state":{"schema_version":"free_state_decision.v1","status":"satisfied","evidence_status":"sufficient","summary":"envelope written and read back"}}`,
	} {
		if !strings.Contains(guidance, anchor) {
			t.Fatalf("time-varying guidance v3 missing shape-example anchor %q", anchor)
		}
	}
	// 直调轮在前、终态读回在后（链序锚）。
	if strings.Index(guidance, `"final":false`) > strings.Index(guidance, `"final":true`) {
		t.Fatalf("shape example must show the direct-call tool turn before the readback terminal turn")
	}
	// 保守负断言：不教 final:true+tool_calls 同轮组合——运行时
	// normalizeMessageLoopOutput 对 final:true 清空 tool_calls（静默丢写），
	// 治理链合法形态是 final:false 工具调用轮（free_state_automation_surface_
	// test.go 治理链先例）。
	if strings.Contains(guidance, `"final":true,"tool_calls"`) {
		t.Fatalf("guidance must not teach a final:true turn carrying tool_calls (runtime drops them silently)")
	}
	// 项三桥接句锚：确认卡=时变写入的核对与授权面；文本预问不成卡。
	for _, anchor := range []string{
		"The confirmation card that the runtime gate raises for the direct call is itself where the time range and point values get checked",
		"a plain-text pre-question in your reply is never part of that gate and never forms a card, so do not pre-ask in text",
	} {
		if !strings.Contains(guidance, anchor) {
			t.Fatalf("time-varying guidance v3 missing confirmation-bridge anchor %q", anchor)
		}
	}
}

// TestConvergenceGuidanceCarriesTimeVaryingExemption：项四——目标级规则句
// 加时变豁免子句；旧句「目标级证据在手→必走 needs_experiment 提案」的直断
// 形态必须消失（244 §2.2 #3 与 v2 直调教学的正面对撞）。
func TestConvergenceGuidanceCarriesTimeVaryingExemption(t *testing.T) {
	guidance := freeStateImprovementConvergenceGuidance()
	for _, anchor := range []string{
		"This proposal workflow serves static adjustments only",
		"a time-varying goal is exempt from it",
		"call automation.set_points directly, and never wrap a time-varying envelope in an improvement_proposal",
	} {
		if !strings.Contains(guidance, anchor) {
			t.Fatalf("convergence guidance missing time-varying exemption anchor %q", anchor)
		}
	}
	// 旧句负断言：原直断句尾（citing it. 后直断结束、无豁免接续）不再在场。
	if strings.Contains(guidance, "improvement_proposal.v1 citing it.\n") {
		t.Fatalf("convergence guidance still carries the un-exempted target-level rule line (244 §2.2 #3 collision shape)")
	}
	// 豁免子句与原句同段同句（邻接，不是分离段落）——对撞句对必须在同一行
	// 内先后在场。
	for _, line := range strings.Split(guidance, "\n") {
		if strings.Contains(line, "Once the target-level evidence for your chosen candidate is in hand") {
			if !strings.Contains(line, "a time-varying goal is exempt from it") {
				t.Fatalf("time-varying exemption must ride the target-level rule line itself, got: %q", line)
			}
			return
		}
	}
	t.Fatal("target-level evidence rule line not found in convergence guidance")
}

// TestTerminalRouteWordingRidesDirectivesLayer：生产装配面——四项措辞全部
// 落在模型实收的 per-turn directives 层（终局句按 locked 条件、其余三项随
// improvement 契约无条件），普通族装配不受影响（缺省组行为不变）。
func TestTerminalRouteWordingRidesDirectivesLayer(t *testing.T) {
	// improvement 契约态：v3 段+豁免句在场。
	improvement := neutralFamilyTestState(map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
	})
	directives := messageLoopNeutralFamilyTurnDirectives(improvement)
	if !strings.Contains(directives, "Action Path Selection (time-varying goals vs static adjustments)") {
		t.Fatal("improvement-contract directives missing the v3 action-path guidance")
	}
	if !strings.Contains(directives, "a time-varying goal is exempt from it") {
		t.Fatal("improvement-contract directives missing the time-varying exemption")
	}
	// 终局句仅 locked 轮渲染（既有 BOUNDARY-1 语义不因三选改形而变）。
	locked := boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["terminal_turn_locked"] = true
	}), nil)
	if !strings.Contains(messageLoopNeutralFamilyTurnDirectives(locked), "a direct automation.set_points mutation call") {
		t.Fatal("locked-turn directives missing the three-choice terminal sentence")
	}
	// 缺省组（无 improvement 契约、未 locked）：终局句与豁免句不渲染——
	// 行为面与 246 前一致（豁免句挂在 improvement 收敛指引内；终局句挂
	// locked 轮）。
	plain := neutralFamilyTestState(nil)
	plainDirectives := messageLoopNeutralFamilyTurnDirectives(plain)
	if strings.Contains(plainDirectives, "This is the final turn") {
		t.Fatal("unlocked plain state must not render the terminal sentence")
	}
	if strings.Contains(plainDirectives, "a time-varying goal is exempt from it") {
		t.Fatal("plain state (no improvement contract) must not render the exemption clause")
	}
	// v3 教学段在缺省组仍无条件在场（235 语义保持：判别教学跨形态在场）。
	if !strings.Contains(plainDirectives, "Action Path Selection (time-varying goals vs static adjustments)") {
		t.Fatal("plain-state directives must still carry the v3 action-path guidance (235 unconditional rule)")
	}
}
