package agentloop

// carrier_neutral_test.go — L1-4-IMPL-D D3：中性族入口四层载体真实装载
// （bundle 层序 Section 进稳定 system；缺层字节兼容）。

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/contextruntime/carriers"
)

func TestNeutralFamilyAssemblyCarriesFourLayerSections(t *testing.T) {
	workspaceDir := t.TempDir()
	projectDir := t.TempDir()
	if _, err := carriers.AppendLedgerEntry(projectDir, "constraint", "", "keep true-peak below -1 dBTP on delivery", nil, 0, time.Date(2026, 10, 8, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	state.input.Context = map[string]any{"project_path": projectDir}

	loop := &MessageLoop{carrierWorkspaceDir: workspaceDir}
	input := loop.neutralFamilyAssemblyInput(state, "{}")
	if len(input.SystemSections) == 0 {
		t.Fatalf("neutral family assembly rendered no system sections")
	}
	joined := make([]string, 0, len(input.SystemSections))
	for _, section := range input.SystemSections {
		joined = append(joined, section.ID)
	}
	order := strings.Join(joined, ",")
	if !strings.HasPrefix(order, carriers.LayerRules) || !strings.HasSuffix(order, carriers.LayerCatalog) {
		t.Fatalf("layer order = %q, want rules-first catalog-last", order)
	}

	system := ""
	for _, section := range input.SystemSections {
		system += section.Content + "\n\n"
	}
	if !strings.Contains(system, "keep true-peak below -1 dBTP on delivery") {
		t.Fatalf("ledger statement missing from neutral system")
	}
	if !strings.Contains(system, "Evidence refs and re-pull discipline:") {
		t.Fatalf("discipline section missing from neutral system")
	}
	if input.CarrierLayerStates[carriers.LayerProfile] != carriers.StateAbsent {
		t.Fatalf("profile layer expected absent (no fixture), got %+v", input.CarrierLayerStates)
	}
	if input.CarrierLayerStates[carriers.LayerLedger] != "" {
		t.Fatalf("rendered ledger layer must not carry a declared state, got %+v", input.CarrierLayerStates)
	}
}

func TestNeutralFamilyAssemblyAbsentLayersFallbackShape(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	loop := &MessageLoop{carrierWorkspaceDir: t.TempDir()}
	input := loop.neutralFamilyAssemblyInput(state, "{}")
	// 全缺席形态：bundle 至少含 rules+catalog 两段；字节兼容性由
	// TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte（全链 parity）锁定。
	if len(input.SystemSections) < 2 {
		t.Fatalf("expected at least rules+catalog sections, got %d", len(input.SystemSections))
	}
}
