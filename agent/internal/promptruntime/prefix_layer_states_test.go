package promptruntime

// L1-4-IMPL-B：PrefixRequest.LayerStates——载体层 absent/corrupt 的显式
// 报告行（§2.0 fail-open 但显式）。已渲染层不受声明影响；声明不改指纹。

import (
	"context"
	"testing"
)

func TestLayerStatesDeclareAbsentAndCorrupt(t *testing.T) {
	service := NewPrefixService()
	section := TextSection(SectionStatic, "ctx.layer.rules", "", "rules text", true)
	req := PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{section}},
		SessionKey:    "layer-states",
		LayerStates: map[string]string{
			"ctx.layer.env":     "corrupt",
			"ctx.layer.profile": "absent",
		},
	}
	_, report, err := service.Assemble(context.Background(), req)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	states := map[string]string{}
	for _, layer := range report.Layers {
		states[layer.LayerID] = layer.State
	}
	if states["ctx.layer.rules"] != "rendered" {
		t.Fatalf("rendered layer affected by declarations: %+v", report.Layers)
	}
	if states["ctx.layer.profile"] != "absent" || states["ctx.layer.env"] != "corrupt" {
		t.Fatalf("declared layer rows missing: %+v", report.Layers)
	}

	// 渲染中的层重复声明不产生第二行。
	dup := req
	dup.LayerStates = map[string]string{"ctx.layer.rules": "absent"}
	_, dupReport, err := service.Assemble(context.Background(), dup)
	if err != nil {
		t.Fatalf("dup assemble: %v", err)
	}
	rows := 0
	for _, layer := range dupReport.Layers {
		if layer.LayerID == "ctx.layer.rules" {
			rows++
		}
	}
	if rows != 1 {
		t.Fatalf("rendered layer must keep exactly one row, got %d", rows)
	}

	// 声明行不进指纹（无内容层，指纹只由渲染层级联）。
	without := PrefixRequest{AssemblyInput: req.AssemblyInput, SessionKey: "layer-states-no-decl"}
	_, withoutReport, err := service.Assemble(context.Background(), without)
	if err != nil {
		t.Fatalf("without assemble: %v", err)
	}
	if withoutReport.PrefixFingerprint != report.PrefixFingerprint {
		t.Fatalf("declarations must not change the prefix fingerprint")
	}

	// 上一轮渲染、本轮声明缺席：diff 记 layer removed + 对应封闭枚举归因。
	second := PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{}},
		SessionKey:    "layer-states",
		LayerStates:   map[string]string{"ctx.layer.rules": "corrupt"},
	}
	_, secondReport, err := service.Assemble(context.Background(), second)
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	if len(secondReport.Breaks) != 1 || secondReport.Breaks[0].LayerID != "ctx.layer.rules" ||
		secondReport.Breaks[0].Reason != BreakRulesetChanged {
		t.Fatalf("expected layer-removed break with ruleset_changed, got %+v", secondReport.Breaks)
	}
}
