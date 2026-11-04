package agentloop

// ccb_model_prompt_action_path_test.go — M3-NEUTRAL-ACTION-PATH-1（235）：
// 时变语义→automation 行动路径教学段到达 turn_directives 层。
// 证明面（承 222 neutral_automation_route_test.go 先例）：
//  1. TestNeutralFamilyTurnDirectivesCarryTimeVaryingActionPath——教学段
//     在 directives 生成器输出中无条件在场（跨形态：普通/诊断轮）。
//  2. TestNeutralFamilyAssemblyLandsActionPathInUserDirectives——生产装配
//     面：教学段物理落在 user 节 directives Section（模型实收的显著层，
//     234 证据），system 骨架不因它变化（T-A3 骨架恒定不被破坏）。
//  3. TestTimeVaryingActionPathGuidanceVersionLock——锚短语集锁定（版本
//     纪律执法面：文案变更须同步升 freeStateTimeVaryingActionPathVersion
//     与 scripts/automation_llm_route_smoke.py 的 payload-capture needle）。
//  4. TestOrdinaryAssemblyUnaffectedByTimeVaryingActionPath——确认话术分支
//     （message_loop_system 完整装配）零回退：普通族装配不含教学段。

import (
	"strings"
	"testing"
)

// timeVaryingActionPathNeedlePhrases 是教学段的机检锚短语集（render/payload
// 工件判读与版本锁定共用；smoke 侧 needle 取标题行，见
// scripts/automation_llm_route_smoke.py TIME_VARYING_PATH_NEEDLE）。
var timeVaryingActionPathNeedlePhrases = []string{
	"Action Path Selection (time-varying goals vs static adjustments)",
	"riding a parameter up or down over time",
	"plugin.get_parameters",
	"automation.set_points",
	"automation.get_curve",
	"The discrimination is semantic, not phrase matching",
	// 243 v2 写入触发权锚：直接调用→运行时自动拦卡；纯文本征询不成卡。
	"make the mutation call directly",
	"a plain-text request for confirmation never forms a confirmation card",
	"do not pre-ask in text",
}

func TestNeutralFamilyTurnDirectivesCarryTimeVaryingActionPath(t *testing.T) {
	guidance := freeStateTimeVaryingActionPathGuidance()
	for _, phrase := range timeVaryingActionPathNeedlePhrases {
		if !strings.Contains(guidance, phrase) {
			t.Fatalf("time-varying action path guidance lacks anchor phrase %q", phrase)
		}
	}
	// 无条件在场：普通中性族轮与诊断轮的 directives 都必须携带教学段
	// （234 机理=「何时走」判别教学从未到达该层；有条件缺席会复现同型）。
	states := []*runState{
		neutralFamilyTestState(nil),
		neutralFamilyTestState(map[string]any{
			"task_contract": map[string]any{"kind": "improvement"},
			"free_state_phase": map[string]any{"diagnostic_only": true},
		}),
	}
	for index, state := range states {
		directives := messageLoopNeutralFamilyTurnDirectives(state)
		if !strings.Contains(directives, guidance) {
			t.Fatalf("state %d: turn directives missing the full time-varying action path guidance (conditional-absence regression)", index)
		}
	}
}

func TestNeutralFamilyAssemblyLandsActionPathInUserDirectives(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	loop := &MessageLoop{carrierWorkspaceDir: t.TempDir()}
	input := loop.neutralFamilyAssemblyInput(state, "{}")

	guidance := freeStateTimeVaryingActionPathGuidance()
	inDirectives := false
	for _, section := range input.UserSections {
		if section.ID == "message_loop_neutral_family_directives" && strings.Contains(section.Content, guidance) {
			inDirectives = true
		}
	}
	if !inDirectives {
		t.Fatalf("time-varying action path guidance must physically land in the per-turn directives user section (the layer the model obeys, 234 forensics)")
	}
	// T-A3 骨架恒定不被破坏：教学段不进 system 层（222 段已在 system 教
	// 工具用法；本卡的增量是 directives 层的「何时走」判别）。
	for _, section := range input.SystemSections {
		if strings.Contains(section.Content, "Action Path Selection (time-varying goals vs static adjustments)") {
			t.Fatalf("time-varying action path guidance must not ride the byte-stable system skeleton")
		}
	}
}

func TestTimeVaryingActionPathGuidanceVersionLock(t *testing.T) {
	// 版本纪律锚：常量文本变更时本锁定面强制意识同步（版本号+锚短语集+
	// smoke 侧 needle 三处）。锚短语与 222 先例同向：不再写第二份同源文本，
	// 以短语集锁定唯一权威常量。
	if freeStateTimeVaryingActionPathVersion == "" {
		t.Fatalf("time-varying action path guidance must carry a version fingerprint")
	}
	guidance := freeStateTimeVaryingActionPathGuidance()
	requiredAnchors := []string{
		// 语义特征判别（非话术映射）
		"one fixed parameter change would satisfy",
		"riding a parameter up or down over time",
		"becoming more or less prominent as the track plays",
		// 路径存在性四步
		"observe to locate the target track",
		"read the parameter value range with plugin.get_parameters",
		"call automation.set_points",
		"read back with automation.get_curve",
		// 治理卡边界（243 v2：触发权因果正序——直接调用，运行时自动拦卡）
		"make the mutation call directly once you hold every write element",
		"the runtime governance gate pauses the call into a user confirmation card automatically",
		"a plain-text request for confirmation never forms a confirmation card",
		"do not pre-ask in text",
		// 与提案面的关系
		"static action_domain proposal is the wrong instrument",
	}
	for _, anchor := range requiredAnchors {
		if !strings.Contains(guidance, anchor) {
			t.Fatalf("time-varying action path guidance v2 anchor drift: missing %q (bump the version and re-sync the anchor set + smoke needle)", anchor)
		}
	}
	// 243 v2 负断言：因果倒置旧句已替换（"passes the gate ... before any write"
	// 可被误读为「先取得用户确认再调用」，242 断点根因）。
	if strings.Contains(guidance, "before any write") {
		t.Fatal("guidance v2 must not carry the causality-inverted \"before any write\" clause (243 wording fix)")
	}
	// 红线自证：教学段不含任何具体用户话术示例（词表全量退役裁定）。
	banned := []string{"这段", "越来越突出", "帮我把", "动起来"}
	for _, phrase := range banned {
		if strings.Contains(guidance, phrase) {
			t.Fatalf("guidance must teach semantic shape discrimination, not phrase mapping: found %q", phrase)
		}
	}
}

func TestOrdinaryAssemblyUnaffectedByTimeVaryingActionPath(t *testing.T) {
	// 确认话术分支零回退：普通族（message_loop_system 完整装配）不含
	// 中性族专属教学段（同 222 先例 TestOrdinaryAssemblyUnaffectedBy-
	// NeutralAutomationRoute 形态）。
	ordinary := &runState{
		input:  Input{UserText: "确认提案", Context: map[string]any{}, AllowedTools: []string{"automation.get_curve", "automation.set_points"}},
		goal:   goalForNeutralFamilyTest,
		budget: Budget{MaxToolCalls: 12},
	}
	if messageLoopNeedsNeutralFamilyProjection(ordinary) {
		t.Fatalf("confirm-phrase state unexpectedly classified as neutral family projection (routing logic untouched red line)")
	}
	loop := &MessageLoop{}
	input := loop.assemblyInput(ordinary, "{}")
	guidance := freeStateTimeVaryingActionPathGuidance()
	for _, section := range input.SystemSections {
		if strings.Contains(section.Content, guidance) {
			t.Fatalf("ordinary message_loop_system assembly must not carry the neutral-only time-varying action path guidance")
		}
	}
	for _, section := range input.UserSections {
		if strings.Contains(section.Content, guidance) {
			t.Fatalf("ordinary assembly user sections must not carry the neutral-only time-varying action path guidance")
		}
	}
	if len(input.SystemSections) == 0 {
		t.Fatalf("ordinary assembly rendered no system sections")
	}
}
