package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Natural-language automation intent layer (M3-C4-AUTO-INTENT-1, L3): a
// time-varying mix phrase ("让这条轨的存在感慢慢强起来") arms the section
// choreography chain deterministically, before any LLM turn — the same
// pre-model takeover philosophy as synthesizeClipRangeSplitCommandsPreModel
// (INTENT-WIRE precedent). The vocabulary below is the routing authority; the
// treatment verb family is the preserved FREESTATE-REGION-IMPL-1 stop-note
// list (freeStateRangeTreatmentVerbs shape) reused verbatim in spirit.

// automationTimeVaryingQualifiers is the time-variant qualifier family — the
// boundary words that separate an envelope intent from a static treatment. A
// phrase without one of these never arms the choreography (negative case:
// "把这条轨存在感调强一点" stays on the static treatment paths).
var automationTimeVaryingQualifiers = []string{
	"慢慢", "逐渐", "渐渐", "渐强", "渐弱", "越来越", "逐步", "一点点",
	"推起来", "铺进来", "铺满", "往上走", "往下走",
	"slowly", "gradually", "crescendo", "build up", "ramp up", "ramp down", "fade in", "fade out",
}

// automationParameterCategoryLevel/Space/Tone are the semantic-result-word
// categories. Level rides the track's built-in Volume & Pan plugin (always
// addressable); space/tone depend on a matching plugin actually being loaded
// on the track — when it is not, the honest boundary is reported and nothing
// is fabricated (card wording: 挂了才有，没挂如实报边界不虚写).
const (
	automationCategoryLevel = "level"
	automationCategorySpace = "space"
	automationCategoryTone  = "tone"
)

// automationSemanticResultWords maps the semantic-result family onto the
// parameter categories: 存在感/能量/音量→电平类（轨 volume ride）；
// 空间/远近/湿→空间类（send/reverb wet）；亮/闷/薄→音色类（EQ/滤波参数）.
var automationSemanticResultWords = map[string]string{
	"存在感": automationCategoryLevel, "能量": automationCategoryLevel, "音量": automationCategoryLevel,
	"声压": automationCategoryLevel, "响度": automationCategoryLevel, "推前": automationCategoryLevel,
	"靠前": automationCategoryLevel, "退后": automationCategoryLevel, "靠后": automationCategoryLevel, "存在": automationCategoryLevel,
	"空间": automationCategorySpace, "空间感": automationCategorySpace, "远近": automationCategorySpace,
	"氛围": automationCategorySpace, "湿": automationCategorySpace, "湿润": automationCategorySpace,
	"宽度": automationCategorySpace, "开阔": automationCategorySpace,
	"亮": automationCategoryTone, "闷": automationCategoryTone, "薄": automationCategoryTone,
	"暗": automationCategoryTone, "高频": automationCategoryTone, "空气感": automationCategoryTone, "音色": automationCategoryTone,
	// english mirrors of the same three categories
	"presence": automationCategoryLevel, "energy": automationCategoryLevel, "volume": automationCategoryLevel,
	"space": automationCategorySpace, "wet": automationCategorySpace, "ambience": automationCategorySpace,
	"brightness": automationCategoryTone, "bright": automationCategoryTone, "dull": automationCategoryTone, "muffled": automationCategoryTone,
}

// automationDirectionUp/Down decide the default shape's direction from the
// phrase polarity.
var automationDirectionUpWords = []string{"强起来", "强一点", "大起来", "多起来", "起来", "推前", "靠前", "渐强", "更亮", "变亮", "亮起来", "强", "up", "more"}
var automationDirectionDownWords = []string{"弱下去", "弱一点", "小下去", "少下去", "下去", "退后", "靠后", "渐弱", "更闷", "变暗", "暗下来", "收", "down", "less"}

// automationTreatmentVerbs is the preserved stop-note treatment verb family
// (freeStateRangeTreatmentVerbs shape): a time-varying phrase carrying one of
// these but no semantic-result word still arms, with the parameter category
// left for the clarification card to ask.
var automationTreatmentVerbs = []string{
	"处理", "调一", "调整", "调改", "改善", "优化", "修一", "修改",
	"不闷", "更亮", "亮一点", "暗一点", "更厚", "更薄", "干净", "好听",
	"混响", "均衡", "压缩", "激励", "响一点",
	"treat", "process", "adjust", "improve", "fix", "brighten", "eq", "compress", "reverb",
}

// automationIntent is the parsed routing decision of one phrase.
type automationIntent struct {
	TimeVarying  bool
	Category     string
	CategoryWord string
	DirectionUp  bool
	HasDirection bool
}

// parseAutomationIntent classifies the phrase against the three vocabulary
// families. TimeVarying is the arming boundary; Category may stay empty when
// only a generic treatment verb matched (the clarification card then asks).
func parseAutomationIntent(text string) automationIntent {
	lower := strings.ToLower(strings.TrimSpace(text))
	intent := automationIntent{}
	if lower == "" {
		return intent
	}
	intent.TimeVarying = textHasAny(lower, automationTimeVaryingQualifiers...)
	// Longest-match first so "空间感" wins over "空间" and "存在感" over "存在".
	words := make([]string, 0, len(automationSemanticResultWords))
	for word := range automationSemanticResultWords {
		words = append(words, word)
	}
	sortStringsByLengthDesc(words)
	for _, word := range words {
		if strings.Contains(lower, word) {
			intent.Category = automationSemanticResultWords[word]
			intent.CategoryWord = word
			break
		}
	}
	if intent.Category == "" && intent.TimeVarying && textHasAny(lower, automationTreatmentVerbs...) {
		// Generic treatment verb with no semantic-result word: the category is
		// unknown and the clarification card must ask (never guess).
		intent.CategoryWord = ""
	}
	intent.HasDirection = true
	if textHasAny(lower, automationDirectionUpWords...) {
		intent.DirectionUp = true
	} else if textHasAny(lower, automationDirectionDownWords...) {
		intent.DirectionUp = false
	} else {
		intent.HasDirection = false
		intent.DirectionUp = true
	}
	return intent
}

// automationIntentArmed is the deterministic routing gate: a time-varying
// phrase with either a semantic-result word or a treatment verb. Without a
// time-varying qualifier nothing arms (static treatment keeps its path).
func automationIntentArmed(text string) bool {
	intent := parseAutomationIntent(text)
	if !intent.TimeVarying {
		return false
	}
	return intent.Category != "" || textHasAny(strings.ToLower(strings.TrimSpace(text)), automationTreatmentVerbs...)
}

// ---- Region resolution (three channels, no guessing) ----

const (
	automationRegionSourceAbsolute  = "utterance_absolute"
	automationRegionSourceMarker    = "section_marker"
	automationRegionNeedsClarify    = "clarification"
	automationSectionWordMaxSeconds = 600.0
)

// automationRegionIntent is the region-resolution outcome of one phrase: one
// of the three channels (utterance-embedded absolute window, project section
// marker, or a clarification ask). L4 audio-inferred sections are out of
// scope by card: no guessing. BareSpanSeconds carries a bare single-ended
// span ("30 秒" with no 前/到/section word) found on the clarification
// channel: the clarify ask offers it as a confirmable candidate (start at 0)
// instead of dropping the number — the user's explicit acceptance, never a
// default (M3-C4-AUTO-CLARIFY-UX-1 faces 1+3).
type automationRegionIntent struct {
	Start, End      float64
	Source          string
	SectionName     string
	SectionSeconds  float64
	BareSpanSeconds float64
	ClarifyReason   string
}

var (
	automationAbsoluteRegionRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)?\s*(?:到|至|~|—|–|to)\s*(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)`)
	automationFromRegionRe     = regexp.MustCompile(`从\s*(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)?\s*(?:到|至|~|—|–|to)\s*(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)`)
	automationUnitDashRegionRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)\s*-\s*(\d+(?:\.\d+)?)\s*(?:秒|s|sec|seconds?)`)
	automationFirstNRe         = regexp.MustCompile(`前\s*(\d+(?:\.\d+)?)\s*(?:秒|s|sec|minutes?|分钟)`)
	automationNumberRe         = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:秒|s|sec)`)
)

// automationSectionNames are the section words the marker channel resolves.
// A section word plus "那 N 秒" (or the bare section span) reads the project's
// section markers; when the project carries no matching marker the region
// falls to the clarification channel — never an audio-inferred guess.
var automationSectionNames = []string{"副歌", "主歌", "前奏", "间奏", "尾奏", "桥段", "drop", "chorus", "verse", "bridge", "intro", "outro"}

func parseSectionName(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, name := range automationSectionNames {
		if strings.Contains(lower, strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

// parseAutomationRegionIntent resolves the region channels in order:
//  1. utterance-embedded absolute windows ("0 到 60 秒", "从 10 秒到 20 秒", "前 60 秒");
//  2. a section word ("副歌") with optional span ("那 30 秒") — resolved against
//     the project markers by the caller (channel 2 marker read);
//  3. clarification (no window, no marker) — the card asks, nothing is guessed.
func parseAutomationRegionIntent(text string) automationRegionIntent {
	lower := strings.ToLower(strings.TrimSpace(text))
	region := automationRegionIntent{Source: automationRegionNeedsClarify}
	if lower == "" {
		region.ClarifyReason = "no_region"
		return region
	}
	if match := automationFromRegionRe.FindStringSubmatch(lower); len(match) == 3 {
		start, startErr := strconv.ParseFloat(match[1], 64)
		end, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && end > start && start >= 0 {
			region.Start, region.End, region.Source = start, end, automationRegionSourceAbsolute
			return region
		}
	}
	if match := automationAbsoluteRegionRe.FindStringSubmatch(lower); len(match) == 3 {
		start, startErr := strconv.ParseFloat(match[1], 64)
		end, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && end > start && start >= 0 {
			region.Start, region.End, region.Source = start, end, automationRegionSourceAbsolute
			return region
		}
	}
	if match := automationUnitDashRegionRe.FindStringSubmatch(lower); len(match) == 3 {
		start, startErr := strconv.ParseFloat(match[1], 64)
		end, endErr := strconv.ParseFloat(match[2], 64)
		if startErr == nil && endErr == nil && end > start && start >= 0 {
			region.Start, region.End, region.Source = start, end, automationRegionSourceAbsolute
			return region
		}
	}
	if match := automationFirstNRe.FindStringSubmatch(lower); len(match) == 2 {
		span, spanErr := strconv.ParseFloat(match[1], 64)
		if spanErr == nil && span > 0 && span <= automationSectionWordMaxSeconds {
			if strings.Contains(lower[strings.Index(lower, match[0]):], "分") {
				span *= 60
			}
			region.Start, region.End, region.Source = 0, span, automationRegionSourceAbsolute
			return region
		}
	}
	if section := parseSectionName(lower); section != "" {
		region.SectionName = section
		region.Source = automationRegionSourceMarker
		// Optional span after the section word: "副歌那 30 秒".
		if idx := strings.Index(lower, strings.ToLower(section)); idx >= 0 {
			tail := lower[idx+len(section):]
			if match := automationNumberRe.FindStringSubmatch(tail); len(match) == 2 {
				if seconds, err := strconv.ParseFloat(match[1], 64); err == nil && seconds > 0 && seconds <= automationSectionWordMaxSeconds {
					region.SectionSeconds = seconds
				}
			}
		}
		return region
	}
	// No window and no section word. A bare "N 秒" is still information: it
	// rides along to the clarification channel as a confirmable candidate
	// (never a default — the card asks, the user accepts or corrects).
	if match := automationNumberRe.FindStringSubmatch(lower); len(match) == 2 {
		if seconds, err := strconv.ParseFloat(match[1], 64); err == nil && seconds > 0 && seconds <= automationSectionWordMaxSeconds {
			region.BareSpanSeconds = seconds
		}
	}
	region.ClarifyReason = "no_region_no_marker"
	return region
}

// ---- Shape generator (default library + explicit values) ----

const (
	automationShapeFadeIn  = "fade_in"
	automationShapeFadeOut = "fade_out"
	automationShapeHold    = "hold"
	automationShapePush    = "push"
)

// automationShape is one resolved envelope shape: name + start/end values in
// the parameter's raw range plus one curvature coefficient.
type automationShape struct {
	Name       string
	Label      string
	StartValue float64
	EndValue   float64
	Curve      float64
}

// automationDefaultShapeLibrary is the v1 default library for the level
// category (track volume raw range [0,1] — 201/217 kernel-verified). The
// amplitudes are deliberately conservative; the confirmation card discloses
// them and the user can edit before confirming.
func automationDefaultShape(directionUp bool) automationShape {
	if directionUp {
		return automationShape{Name: automationShapeFadeIn, Label: "渐入", StartValue: 0.30, EndValue: 0.60, Curve: 0}
	}
	return automationShape{Name: automationShapeFadeOut, Label: "渐出", StartValue: 0.60, EndValue: 0.30, Curve: 0}
}

// automationShapeLibrary lists the four library entries for the confirmation
// card's editable choice field.
func automationShapeLibrary() []map[string]any {
	return []map[string]any{
		{"id": automationShapeFadeIn, "label": "渐入（0.30 → 0.60）"},
		{"id": automationShapeFadeOut, "label": "渐出（0.60 → 0.30）"},
		{"id": automationShapeHold, "label": "保持（0.55 → 0.55）"},
		{"id": automationShapePush, "label": "推进（0.25 → 0.80，快起）"},
	}
}

// automationShapeByID resolves a library entry (confirmation-card edit face).
func automationShapeByID(id string) (automationShape, bool) {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case automationShapeFadeIn:
		return automationShape{Name: automationShapeFadeIn, Label: "渐入", StartValue: 0.30, EndValue: 0.60, Curve: 0}, true
	case automationShapeFadeOut:
		return automationShape{Name: automationShapeFadeOut, Label: "渐出", StartValue: 0.60, EndValue: 0.30, Curve: 0}, true
	case automationShapeHold:
		return automationShape{Name: automationShapeHold, Label: "保持", StartValue: 0.55, EndValue: 0.55, Curve: 0}, true
	case automationShapePush:
		return automationShape{Name: automationShapePush, Label: "推进", StartValue: 0.25, EndValue: 0.80, Curve: -0.5}, true
	default:
		return automationShape{}, false
	}
}

// automationExplicitValues extracts explicit numeric start/end values from the
// phrase ("从 0.2 涨到 0.8"): the utterance wins over the default library.
var automationExplicitValuesRe = regexp.MustCompile(`(?:从|from)?\s*(0?\.\d+|1\.0+|0\.\d+|\d+(?:\.\d+)?)\s*(?:涨到|到|升至|升到|降到|降至|ramp to|to)\s*(0?\.\d+|1\.0+|0\.\d+|\d+(?:\.\d+)?)`)

func parseAutomationExplicitValues(text string) (start, end float64, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	match := automationExplicitValuesRe.FindStringSubmatch(lower)
	if len(match) != 3 {
		return 0, 0, false
	}
	startValue, startErr := strconv.ParseFloat(match[1], 64)
	endValue, endErr := strconv.ParseFloat(match[2], 64)
	if startErr != nil || endErr != nil || startValue < 0 || startValue > 1 || endValue < 0 || endValue > 1 || startValue == endValue {
		return 0, 0, false
	}
	return startValue, endValue, true
}

// automationBuildPoints renders the resolved shape over the resolved region
// into the envelope point payload (v1: two anchor points; the curvature
// coefficient carries the in-between shape).
func automationBuildPoints(shape automationShape, start, end float64) []map[string]any {
	return []map[string]any{
		{"t": start, "v": shape.StartValue, "c": shape.Curve},
		{"t": end, "v": shape.EndValue, "c": shape.Curve},
	}
}

// automationShapeSummary renders the disclosure line for cards and replies.
func automationShapeSummary(shape automationShape) string {
	return fmt.Sprintf("%s（%s：%.2f → %.2f，曲率 %.2f）", shape.Label, shape.Name, shape.StartValue, shape.EndValue, shape.Curve)
}

func sortStringsByLengthDesc(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && len(values[j]) > len(values[j-1]); j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
