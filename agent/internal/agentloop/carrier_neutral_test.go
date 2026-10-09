package agentloop

// carrier_neutral_test.go — L1-4-IMPL-D D3：中性族入口四层载体真实装载
// （bundle 层序 Section 进稳定 system；缺层字节兼容）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/promptruntime"
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

// REVIEW-1 G-3：入口级 L1 corrupt 回落（len(sections)==0→单段骨架）直接
// 测试——embed 装载失败理论不可达，回落分支以提炼后的纯函数直测锁定形态。
func TestNeutralFamilySystemSectionsCorruptFallback(t *testing.T) {
	state := neutralFamilyTestState(nil)
	fallback := neutralFamilySystemSections(carriers.Bundle{}, state)
	if len(fallback) != 1 {
		t.Fatalf("empty bundle fallback sections = %d, want single skeleton section", len(fallback))
	}
	if fallback[0].ID != "message_loop_neutral_family_selection" || !fallback[0].Stable {
		t.Fatalf("fallback section id/stable = %q/%v, want static stable selection section", fallback[0].ID, fallback[0].Stable)
	}
	if fallback[0].Content != messageLoopNeutralFamilySystemSkeleton(state) {
		t.Fatalf("fallback content diverges from single-section skeleton form")
	}
	rendered := []promptruntime.Section{{ID: "ctx.layer.rules"}, {ID: "ctx.layer.catalog"}}
	if got := neutralFamilySystemSections(carriers.Bundle{Sections: rendered}, state); len(got) != 2 || got[0].ID != "ctx.layer.rules" || got[1].ID != "ctx.layer.catalog" {
		t.Fatalf("non-empty bundle must pass sections through unchanged, got %+v", got)
	}
}

// L4-LEDGER-DIR-1：真栈路径形态（project_path=.vit 文件，账本落父目录）下
// message_loop 装配读面必须 rendered，而非永 absent。
func TestNeutralFamilyAssemblyRendersLedgerForVitFileProjectPath(t *testing.T) {
	parent := t.TempDir()
	projectFile := filepath.Join(parent, "neutral_fixture.vit")
	if err := os.WriteFile(projectFile, []byte("kernel project file placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := carriers.AppendLedgerEntry(parent, "constraint", "", "neutral vit-form ledger line", nil, 0, time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed ledger at parent: %v", err)
	}
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	state.input.Context = map[string]any{"project_path": projectFile}
	loop := &MessageLoop{carrierWorkspaceDir: t.TempDir()}
	input := loop.neutralFamilyAssemblyInput(state, "{}")
	system := ""
	for _, section := range input.SystemSections {
		system += section.Content + "\n\n"
	}
	if !strings.Contains(system, "neutral vit-form ledger line") {
		t.Fatalf("ledger statement missing from neutral system for .vit project_path form")
	}
}
