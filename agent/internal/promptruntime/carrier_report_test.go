package promptruntime

// carrier_report_test.go — L1-4-IMPL-D D3：装配输入随附载体声明与 WARN 的
// 报告并入（CarrierLayerStates/CarrierWarnings → AssemblyReport）。

import (
	"context"
	"testing"
)

func TestAssembleMergesCarrierLayerStatesAndWarnings(t *testing.T) {
	service := NewPrefixService()
	input := AssemblyInput{
		SystemSections: []Section{
			TextSection(SectionStatic, "stable_a", "", "alpha", true),
		},
		CarrierLayerStates: map[string]string{"ctx.layer.profile": "absent"},
		CarrierWarnings:    []string{"profile layer absent", "second warning"},
	}
	_, report, err := service.Assemble(context.Background(), PrefixRequest{AssemblyInput: input, SessionKey: "carrier-merge"})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	found := false
	for _, layer := range report.Layers {
		if layer.LayerID == "ctx.layer.profile" && layer.State == "absent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("declared absent layer missing from report.Layers: %+v", report.Layers)
	}
	if len(report.CarrierWarnings) != 2 {
		t.Fatalf("CarrierWarnings = %v, want both warnings surfaced", report.CarrierWarnings)
	}
	extras := report.PromptStatsExtras()
	if extras["carrier_warnings"] != 2 {
		t.Fatalf("extras carrier_warnings = %v, want 2", extras["carrier_warnings"])
	}
}

func TestAssembleRequestLevelLayerStatesStillWork(t *testing.T) {
	// 请求级 LayerStates（IMPL-B 既有面）不因 D3 合并路径退化；同名时请求级优先。
	service := NewPrefixService()
	input := AssemblyInput{
		SystemSections:     []Section{TextSection(SectionStatic, "stable_a", "", "alpha", true)},
		CarrierLayerStates: map[string]string{"ctx.layer.env": "absent"},
	}
	_, report, err := service.Assemble(context.Background(), PrefixRequest{
		AssemblyInput: input,
		SessionKey:    "carrier-merge-2",
		LayerStates:   map[string]string{"ctx.layer.env": "corrupt"},
	})
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	state := ""
	for _, layer := range report.Layers {
		if layer.LayerID == "ctx.layer.env" {
			state = layer.State
		}
	}
	if state != "corrupt" {
		t.Fatalf("request-level layer state = %q, want corrupt (request precedence)", state)
	}
}
