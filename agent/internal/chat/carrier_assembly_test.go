package chat

// carrier_assembly_test.go — L1-4-IMPL-D D3：chat 入口四层载体真实装载
// （L2 偏好档 + L4 工程账本渲染进稳定 system；层序与报告行）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/promptruntime"
)

func writeCarrierProfileFixture(t *testing.T, workspaceDir string) {
	t.Helper()
	doc := map[string]any{
		"schema_version": "user_profile.v1",
		"user_id":        "local",
		"entries": []map[string]any{{
			"pref_id":       "p-1",
			"statement":     "User prefers vocal-forward reference balances",
			"class":         "core",
			"status":        "active",
			"confirmed_at":  "2026-10-08T09:00:00Z",
			"source":        "confirmation-ticket-1",
			"evidence_refs": []string{"track://vox"},
		}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal profile fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, carriers.ProfileFilename), raw, 0o644); err != nil {
		t.Fatalf("write profile fixture: %v", err)
	}
}

func TestChatAssemblyCarriesFourLayerSections(t *testing.T) {
	workspaceDir := t.TempDir()
	projectDir := t.TempDir()
	writeCarrierProfileFixture(t, workspaceDir)
	if _, err := carriers.AppendLedgerEntry(projectDir, "constraint", "", "no clipping on the master bus", nil, 0, time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}

	server := newChatServerForTest(t, nil, nil, nil)
	server.carrierWorkspaceDir = workspaceDir
	requestContext := map[string]any{"project_path": projectDir}
	assembly, report, err := server.buildAssemblyWithReport(context.Background(), "conv-carriers", "四层轮。", requestContext)
	if err != nil {
		t.Fatalf("buildAssemblyWithReport: %v", err)
	}
	system := chatAssemblySystemText(t, assembly.Messages)

	anchors := []string{
		"You are Ask Vit",                               // L1 rules
		"Evidence refs and re-pull discipline:",         // L1 IMPL-C 段
		"User prefers vocal-forward reference balances", // L2 profile
		"no clipping on the master bus",                 // L4 ledger
		"Available DAW command catalog:",                // 目录排尾
	}
	last := -1
	for _, anchor := range anchors {
		at := strings.Index(system, anchor)
		if at < 0 {
			t.Fatalf("system message missing layer anchor %q", anchor)
		}
		if at < last {
			t.Fatalf("layer anchor %q out of order (at %d, previous %d)", anchor, at, last)
		}
		last = at
	}

	rendered := map[string]bool{}
	for _, layer := range report.Layers {
		if layer.LayerID == carriers.LayerProfile && layer.State == "rendered" {
			rendered["profile"] = true
		}
		if layer.LayerID == carriers.LayerLedger && layer.State == "rendered" {
			rendered["ledger"] = true
		}
	}
	if !rendered["profile"] || !rendered["ledger"] {
		t.Fatalf("report layers missing rendered profile/ledger rows: %+v", report.Layers)
	}
}

func TestChatAssemblyAbsentLayersKeepPrefixByteStable(t *testing.T) {
	// 旧工程/无工作区载体常态：L2-L4 全缺席——system 字节与 D2 单段形态
	// 逐字节一致（renderSections 连接语义；parity 主证明的配套腿）。
	server := newChatServerForTest(t, nil, nil, nil)
	server.carrierWorkspaceDir = t.TempDir()
	assembly := server.buildAssembly(context.Background(), "conv-carriers-absent", "对照轮。", map[string]any{})
	system := chatAssemblySystemText(t, assembly.Messages)

	catalog := server.harness.ModelCatalogSummary()
	if !strings.HasPrefix(system, "You are Ask Vit") || !strings.HasSuffix(system, catalog) {
		t.Fatalf("absent-layer system shape unexpected (head/tail mismatch)")
	}
	if strings.Contains(system, "User prefers") || strings.Contains(system, "master bus") {
		t.Fatalf("absent layers must not inject content")
	}
}

// REVIEW-1 G-3：入口级 L1 corrupt 回落（len(sections)==0→D2 单段形态）直接
// 测试——embed 装载失败理论不可达，回落分支以提炼后的纯函数直测锁定形态。
func TestChatSystemSectionsCorruptFallback(t *testing.T) {
	fallback := chatSystemSections(carriers.Bundle{}, "mode", "catalog")
	if len(fallback) != 1 {
		t.Fatalf("empty bundle fallback sections = %d, want single D2 section", len(fallback))
	}
	if fallback[0].ID != "chat_system" || !fallback[0].Stable {
		t.Fatalf("fallback section id/stable = %q/%v, want static stable chat_system section", fallback[0].ID, fallback[0].Stable)
	}
	if fallback[0].Content != chatSystemFromRuleset("mode", "catalog") {
		t.Fatalf("fallback content diverges from D2 single-section form")
	}
	rendered := []promptruntime.Section{{ID: carriers.LayerRules}, {ID: carriers.LayerCatalog}}
	if got := chatSystemSections(carriers.Bundle{Sections: rendered}, "mode", "catalog"); len(got) != 2 || got[0].ID != carriers.LayerRules || got[1].ID != carriers.LayerCatalog {
		t.Fatalf("non-empty bundle must pass sections through unchanged, got %+v", got)
	}
}

// L4-LEDGER-DIR-1：真栈路径形态（project_path=.vit 文件，账本落父目录）下
// chat 装配读面必须 rendered，而非永 absent。
func TestChatAssemblyRendersLedgerForVitFileProjectPath(t *testing.T) {
	parent := t.TempDir()
	projectFile := filepath.Join(parent, "carrier_fixture.vit")
	if err := os.WriteFile(projectFile, []byte("kernel project file placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := carriers.AppendLedgerEntry(parent, "constraint", "", "vit-form ledger reaches the system prefix", nil, 0, time.Date(2026, 10, 9, 9, 30, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed ledger at parent: %v", err)
	}
	server := newChatServerForTest(t, nil, nil, nil)
	server.carrierWorkspaceDir = t.TempDir()
	assembly, report, err := server.buildAssemblyWithReport(context.Background(), "conv-carriers-vit", "vit 形态轮。", map[string]any{"project_path": projectFile})
	if err != nil {
		t.Fatalf("buildAssemblyWithReport: %v", err)
	}
	system := chatAssemblySystemText(t, assembly.Messages)
	if !strings.Contains(system, "vit-form ledger reaches the system prefix") {
		t.Fatalf("ledger statement missing from system for .vit project_path form")
	}
	rendered := false
	for _, layer := range report.Layers {
		if layer.LayerID == carriers.LayerLedger && layer.State == "rendered" {
			rendered = true
		}
	}
	if !rendered {
		t.Fatalf("report layers missing rendered ledger row for .vit project_path form: %+v", report.Layers)
	}
}
