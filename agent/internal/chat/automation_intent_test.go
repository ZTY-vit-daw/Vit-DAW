package chat

import (
	"strings"
	"testing"
)

// M3-C4-AUTO-INTENT-1: the three vocabulary families, region channels,
// shape generator and revision merge of the deterministic choreography
// routing layer.

func TestAutomationIntentArmingBoundary(t *testing.T) {
	cases := []struct {
		text  string
		armed bool
	}{
		{"副歌那 30 秒，让这条轨的存在感慢慢强起来", true},
		{"0 到 60 秒，把这条轨的空间感逐渐铺进来", true},
		{"让能量慢慢推起来", true},
		{"把这条轨存在感调强一点", false},                         // no time-varying word: static treatment path
		{"把低频降下来", false},                              // static
		{"轨道音量是多少", false},                             // fact question
		{"帮我把这段拆出来", false},                            // structural, no treatment verb with time variance
		{"fade in the vocal presence over time", true}, // english time-varying + semantic word
	}
	for _, tc := range cases {
		if got := automationIntentArmed(tc.text); got != tc.armed {
			t.Errorf("automationIntentArmed(%q) = %v, want %v", tc.text, got, tc.armed)
		}
	}
}

func TestParseAutomationIntentCategoryMapping(t *testing.T) {
	cases := []struct {
		text     string
		category string
	}{
		{"让存在感慢慢强起来", automationCategoryLevel},
		{"能量逐渐推起来", automationCategoryLevel},
		{"音量慢慢涨上去", automationCategoryLevel},
		{"空间感渐渐铺进来", automationCategorySpace},
		{"远近慢慢拉开", automationCategorySpace},
		{"声音越来越亮", automationCategoryTone},
		{"这段听着闷，慢慢变亮", automationCategoryTone},
	}
	for _, tc := range cases {
		intent := parseAutomationIntent(tc.text)
		if !intent.TimeVarying {
			t.Errorf("parseAutomationIntent(%q).TimeVarying = false", tc.text)
		}
		if intent.Category != tc.category {
			t.Errorf("parseAutomationIntent(%q).Category = %q, want %q", tc.text, intent.Category, tc.category)
		}
	}
	// Time-varying phrase with a treatment verb but no semantic-result word:
	// the category stays unknown and the clarification card must ask.
	intent := parseAutomationIntent("把这条轨处理得慢慢好听一点")
	if !intent.TimeVarying || intent.Category != "" {
		t.Errorf("treatment-only phrase: TimeVarying=%v Category=%q, want true/empty", intent.TimeVarying, intent.Category)
	}
}

func TestParseAutomationRegionThreeChannels(t *testing.T) {
	// Channel 1: utterance-embedded absolute windows.
	for _, tc := range []struct {
		text       string
		start, end float64
		source     string
	}{
		{"0 到 60 秒", 0, 60, automationRegionSourceAbsolute},
		{"从 10 秒到 25 秒", 10, 25, automationRegionSourceAbsolute},
		{"10s-40s", 10, 40, automationRegionSourceAbsolute},
		{"前 30 秒", 0, 30, automationRegionSourceAbsolute},
	} {
		region := parseAutomationRegionIntent(tc.text)
		if region.Source != tc.source || region.Start != tc.start || region.End != tc.end {
			t.Errorf("parseAutomationRegionIntent(%q) = %+v, want %s %.0f-%.0f", tc.text, region, tc.source, tc.start, tc.end)
		}
	}
	// Channel 2: section word with optional span (marker resolution is
	// server-side; the parse only extracts the ask).
	region := parseAutomationRegionIntent("副歌那 30 秒让存在感慢慢强起来")
	if region.Source != automationRegionSourceMarker || region.SectionName != "副歌" || region.SectionSeconds != 30 {
		t.Errorf("section phrase parse = %+v, want marker/副歌/30s", region)
	}
	region = parseAutomationRegionIntent("verse 这段把能量推起来")
	if region.Source != automationRegionSourceMarker || region.SectionName != "verse" || region.SectionSeconds != 0 {
		t.Errorf("bare section parse = %+v, want marker/verse/0", region)
	}
	// Channel 3: no window, no marker -> clarification, never a guess.
	region = parseAutomationRegionIntent("让存在感慢慢强起来")
	if region.Source != automationRegionNeedsClarify {
		t.Errorf("windowless phrase source = %q, want %q", region.Source, automationRegionNeedsClarify)
	}
}

func TestAutomationShapeGenerator(t *testing.T) {
	// Default library by direction.
	up := automationDefaultShape(true)
	if up.Name != automationShapeFadeIn || up.EndValue <= up.StartValue {
		t.Errorf("default up shape = %+v", up)
	}
	down := automationDefaultShape(false)
	if down.Name != automationShapeFadeOut || down.EndValue >= down.StartValue {
		t.Errorf("default down shape = %+v", down)
	}
	// All four library entries resolve.
	for _, name := range []string{automationShapeFadeIn, automationShapeFadeOut, automationShapeHold, automationShapePush} {
		if _, ok := automationShapeByID(name); !ok {
			t.Errorf("automationShapeByID(%q) missing", name)
		}
	}
	// Explicit utterance values win over the library.
	start, end, ok := parseAutomationExplicitValues("从 0.2 涨到 0.8")
	if !ok || start != 0.2 || end != 0.8 {
		t.Errorf("parseAutomationExplicitValues = %.2f-%.2f ok=%v", start, end, ok)
	}
	if _, _, ok := parseAutomationExplicitValues("让它慢慢强起来"); ok {
		t.Errorf("no explicit values should parse from a qualitative phrase")
	}
	// Points render over the region.
	points := automationBuildPoints(up, 12.5, 42.5)
	if len(points) != 2 || points[0]["t"].(float64) != 12.5 || points[1]["t"].(float64) != 42.5 {
		t.Errorf("automationBuildPoints = %+v", points)
	}
}

func TestAutomationChoreoRevisionMerge(t *testing.T) {
	base := automationChoreoCandidate{
		Shape:       automationDefaultShape(true),
		RegionStart: 0, RegionEnd: 30,
	}
	base.Points = automationBuildPoints(base.Shape, base.RegionStart, base.RegionEnd)
	// Value revision.
	revised, err := automationChoreoApplyRevision(base, "改成 0.4 到 0.9")
	if err != nil {
		t.Fatalf("value revision: %v", err)
	}
	if revised.Shape.StartValue != 0.4 || revised.Shape.EndValue != 0.9 {
		t.Errorf("revised values = %.2f-%.2f", revised.Shape.StartValue, revised.Shape.EndValue)
	}
	if revised.Points[0]["v"].(float64) != 0.4 || revised.Points[1]["v"].(float64) != 0.9 {
		t.Errorf("revised points not rebuilt: %+v", revised.Points)
	}
	// Region revision.
	revised, err = automationChoreoApplyRevision(base, "改成 10 到 40 秒")
	if err != nil {
		t.Fatalf("region revision: %v", err)
	}
	if revised.RegionStart != 10 || revised.RegionEnd != 40 {
		t.Errorf("revised region = %.0f-%.0f", revised.RegionStart, revised.RegionEnd)
	}
	// Unreadable revision fails honestly.
	if _, err := automationChoreoApplyRevision(base, "就按刚才说的"); err == nil {
		t.Errorf("unreadable revision should fail")
	}
	// Field edits: shape swap + values + region.
	edited, err := automationChoreoApplyFieldEdits(base, map[string]any{
		"shape": automationShapePush, "start_value": "0.25", "end_value": "0.80", "curve": "-0.50",
		"region_start": "5", "region_end": "35",
	})
	if err != nil {
		t.Fatalf("field edits: %v", err)
	}
	if edited.Shape.StartValue != 0.25 || edited.Shape.EndValue != 0.80 || edited.Shape.Curve != -0.5 || edited.RegionStart != 5 || edited.RegionEnd != 35 {
		t.Errorf("edited candidate = %+v", edited)
	}
	if _, err := automationChoreoApplyFieldEdits(base, map[string]any{"curve": "2.5"}); err == nil {
		t.Errorf("curve out of [-1,1] should fail")
	}
	if _, err := automationChoreoApplyFieldEdits(base, map[string]any{"region_start": "40", "region_end": "10"}); err == nil {
		t.Errorf("inverted region should fail")
	}
}

func TestAutomationChoreoHumanSummaryShapeWord(t *testing.T) {
	summary := automationChoreoHumanSummary(automationChoreoCandidate{
		TrackID: "t1", ParamID: "volume", Category: automationCategoryLevel,
		Shape: automationDefaultShape(true), RegionStart: 0, RegionEnd: 30,
		Points: automationBuildPoints(automationDefaultShape(true), 0, 30),
	})
	for _, want := range []string{"Track t1", "volume", "电平类", "0.0s-30.0s", "渐入"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q missing %q", summary, want)
		}
	}
}
