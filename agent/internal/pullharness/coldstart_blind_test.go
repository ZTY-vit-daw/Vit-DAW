package pullharness

// coldstart_blind_test.go — 内容盲审查（TIMING-2 / CONTEXT_LAYERING §8.1，
// 卡面验收 1"内容盲审查：底座+披露段无域处理知识"）。
//
// 机械键形态与 T-C1 的 RuleSection.Purpose 等价（卡面"RuleSection.Purpose
// 机械键思路同 T-C1，本包等价断言"）：
//   1. 审查键封闭枚举：每个底座段必须持有效 ColdStartSectionPurpose，
//      规则段=discipline、事实段=facts（本包封闭枚举）；
//   2. 域处理知识禁词扫描：底座+动态区披露段的渲染文本不得含域处理词表
//      任一项（词表取 push 侧 Pattern Recognition / preflight 域动词族；
//      "consider/apply" 类引导词同禁——处理知识归模型参数，D12）。
// 正对照（阳性样本必须被扫描器命中）防词表空转。

import (
	"strings"
	"testing"
)

// blindForbiddenTokens 是域处理知识禁词表（小写匹配）。来源=push 侧既有
// 域处理文本的词族（agentloop Pattern Recognition 段与 preflight 域动词）：
// 领域效果器/处理动作/引导式建议词。"gate" 不入列——证据门（admission
// gate）语义与音频噪声门同形异义，前者是本 harness 准入词汇。目录结构词
// （track.*、mix.* 视图 ID、LUFS/dBTP 参数事实）不在禁列——它们是 CCB
// 目录与 RLM 引用面，非处理知识。
var blindForbiddenTokens = []string{
	"track_gain", "static_eq", "broadband_compression", "compressor",
	"equalizer", "reverb", "limiter", "de-ess",
	"gain staging", "gain reduction", "gain adjustment", "normalize",
	"normalization", "fade", "stems", "stem split", "silence",
	"crest", "headroom", "pumping", "stereo width", "low band",
	"high band", "consider ", "suggests", "apply ", "often indicates",
}

func blindScan(t *testing.T, label string, texts ...string) {
	t.Helper()
	for _, text := range texts {
		lowered := strings.ToLower(text)
		for _, token := range blindForbiddenTokens {
			if strings.Contains(lowered, token) {
				t.Fatalf("%s contains domain processing knowledge %q (content-blind violation): %q",
					label, token, text)
			}
		}
	}
}

// 审查键封闭枚举 + 域处理知识禁词扫描：底座（含规则段）+动态区披露段。
func TestColdStartAndDisclosureContentBlind(t *testing.T) {
	base := RenderColdStart(coldStartFixtureEngine())
	contents := make([]string, 0, len(base.Sections))
	for _, section := range base.Sections {
		purpose := coldStartSectionPurpose(section.ID)
		if !purpose.Valid() {
			t.Fatalf("cold start section %s purpose %q not in closed enum", section.ID, purpose)
		}
		switch section.ID {
		case ColdStartSectionRules:
			if purpose != ColdStartPurposeDiscipline {
				t.Fatalf("rules section purpose = %q, want discipline", purpose)
			}
		case ColdStartSectionTOM, ColdStartSectionBus, ColdStartSectionDelivery:
			if purpose != ColdStartPurposeFacts {
				t.Fatalf("facts section %s purpose = %q, want facts", section.ID, purpose)
			}
		}
		contents = append(contents, section.Content)
	}
	blindScan(t, "cold start base", contents...)

	// 动态区披露段（行 3/4 迁移文本；预算计数行随参）。
	rows := dynamicStateRows(2, 1.5, ObservationBudget{MaxCycles: 6, MaxProbeCost: 4})
	blindScan(t, "dynamic disclosure rows", rows...)

	// 正对照：阳性域处理样本必须命中（扫描器非空转）。
	positive := "mix.masking_relationship margins often indicates level-imbalance candidates; consider track_gain adjustment"
	lowered := strings.ToLower(positive)
	hit := false
	for _, token := range blindForbiddenTokens {
		if strings.Contains(lowered, token) {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatalf("positive control not flagged — forbidden token list is vacuous")
	}
}
