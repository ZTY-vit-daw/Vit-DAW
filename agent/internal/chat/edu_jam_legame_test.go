package chat

// W01-C binding 读取点零栈单测：UUID 目录隔离（A/B 不串）、新格式带工程身份、
// UUID 缺省/旧父目录兼容、M1 导入命令参数解析、legacy 卡内容寻址缓存（禁 level_id 单键）。
// 全部落盘 t.TempDir()；不启真栈。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eduJamLegameProject 建伪工程路径（共享父目录，验证同目录 A/B 不串）。
func eduJamLegameProject(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name+".vit")
	if err := os.WriteFile(path, []byte("fake project"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	return path
}

func TestEduJamLegameBindingUUIDIsolation(t *testing.T) {
	shared := t.TempDir()
	projectA := eduJamLegameProject(t, shared, "A")
	projectB := eduJamLegameProject(t, shared, "B")

	// 同目录 A/B 两工程写 binding：路径必须按 project UUID 隔离（H3 串工程修复）。
	if err := writeLegameBindingForProject(projectA, "uuid-a", legameBinding{
		SchemaVersion: legameBindingSchemaVersion, LevelID: "level-alpha", BoundSource: "test",
	}); err != nil {
		t.Fatalf("write A binding: %v", err)
	}
	if err := writeLegameBindingForProject(projectB, "uuid-b", legameBinding{
		SchemaVersion: legameBindingSchemaVersion, LevelID: "level-beta", BoundSource: "test",
	}); err != nil {
		t.Fatalf("write B binding: %v", err)
	}
	pathA := legameBindingPathForProject(projectA, "uuid-a")
	pathB := legameBindingPathForProject(projectB, "uuid-b")
	if pathA == pathB {
		t.Fatalf("A/B binding 路径相同（串工程）: %s", pathA)
	}
	legacy := legameBindingPath(projectA)
	if pathA == legacy {
		t.Fatalf("新写入不得落旧父目录路径: %s", pathA)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("旧父目录 binding 不应存在: %v", err)
	}

	bindingA, resolvedA, ok := loadLegameBindingForProject(projectA, "uuid-a")
	if !ok || bindingA.LevelID != "level-alpha" || resolvedA != pathA {
		t.Fatalf("A binding 读取错: %+v path=%q ok=%v", bindingA, resolvedA, ok)
	}
	bindingB, _, ok := loadLegameBindingForProject(projectB, "uuid-b")
	if !ok || bindingB.LevelID != "level-beta" {
		t.Fatalf("B binding 读取错: %+v ok=%v", bindingB, ok)
	}
	// 新格式带工程身份：用错 uuid 读同一工程必须拒绝（防跨工程误用）。
	if _, _, ok := loadLegameBindingForProject(projectA, "uuid-wrong"); ok {
		t.Fatalf("binding.project_uuid 校验缺失（错误 uuid 仍读到）")
	}
	// 开场标记回写只动 opened 两字段并写回命中路径。
	if err := writeLegameBindingOpenedAt(resolvedA, bindingA, "conv-xyz"); err != nil {
		t.Fatalf("write opened: %v", err)
	}
	after, ok := loadLegameBinding(projectA)
	_ = after
	_ = ok
	reloaded, _, ok := loadLegameBindingForProject(projectA, "uuid-a")
	if !ok || reloaded.OpenedConversationID == nil || *reloaded.OpenedConversationID != "conv-xyz" {
		t.Fatalf("opened 回写未生效: %+v ok=%v", reloaded, ok)
	}
	if reloaded.LevelID != "level-alpha" {
		t.Fatalf("opened 回写不得改绑定本体: %+v", reloaded)
	}
}

func TestEduJamLegameBindingLegacyCompat(t *testing.T) {
	shared := t.TempDir()
	project := eduJamLegameProject(t, shared, "legacy")
	legacyPath := legameBindingPath(project)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	data, _ := json.MarshalIndent(legameBinding{
		SchemaVersion: legameBindingSchemaVersion, LevelID: "first-run-twinkle", BoundSource: "placement_script",
	}, "", "  ")
	if err := os.WriteFile(legacyPath, data, 0o644); err != nil {
		t.Fatalf("write legacy binding: %v", err)
	}
	// UUID 可用但 UUID 目录缺失 → 旧父目录兜底可读（旧布位脚本兼容）。
	binding, resolved, ok := loadLegameBindingForProject(project, "uuid-new")
	if !ok || binding.LevelID != "first-run-twinkle" || resolved != legacyPath {
		t.Fatalf("legacy 兜底读取失败: %+v path=%q ok=%v", binding, resolved, ok)
	}
	// UUID 缺省（旧调用）只认旧父目录。
	legacy, ok := loadLegameBinding(project)
	if !ok || legacy.LevelID != "first-run-twinkle" {
		t.Fatalf("legacy 旧入口读取失败: %+v ok=%v", legacy, ok)
	}
	if legameBindingPathForProject(project, "") != legacyPath {
		t.Fatalf("UUID 缺省路径须回退旧父目录")
	}
}

func TestEduJamLegameImportCommandParsing(t *testing.T) {
	face, args, _, _, _, ok := parseEduJamCommand("导入乐局设计源 D:/some/unit2 tempo=90 turns=40 calls=80")
	if !ok || face != "import" {
		t.Fatalf("导入命令未识别: face=%q ok=%v", face, ok)
	}
	if !strings.Contains(args, "D:/some/unit2") || !strings.Contains(args, "tempo=90") {
		t.Fatalf("导入命令参数透传错: %q", args)
	}

	dir := t.TempDir()
	sourceFile := filepath.Join(dir, "design.v1.yaml")
	if err := os.WriteFile(sourceFile, []byte("schema_version: vit.legame_design.v0\n"), 0o644); err != nil {
		t.Fatalf("write design: %v", err)
	}
	sourceDir, designFile, overrides, err := parseEduJamImportArgs(sourceFile + " tempo=80 turns=60 calls=120")
	if err != nil {
		t.Fatalf("parse import args: %v", err)
	}
	if sourceDir != dir || designFile != "design.v1.yaml" {
		t.Fatalf("设计源文件路径拆分错: dir=%q file=%q", sourceDir, designFile)
	}
	if overrides.TempoBPM == nil || *overrides.TempoBPM != 80 || overrides.MaxTurns == nil || *overrides.MaxTurns != 60 || overrides.MaxLLMCalls == nil || *overrides.MaxLLMCalls != 120 {
		t.Fatalf("R5 覆盖参数解析错: %+v", overrides)
	}
	if _, _, _, err := parseEduJamImportArgs("D:/not/exist/unit2"); err == nil {
		t.Fatalf("不存在路径应 usage 报错")
	}
	if _, _, _, err := parseEduJamImportArgs(""); err == nil {
		t.Fatalf("空参数应 usage 报错")
	}
	if _, _, _, err := parseEduJamImportArgs(sourceFile + " tempo=0"); err == nil {
		t.Fatalf("非法 tempo 覆盖参数应报错")
	}
}

func TestEduJamLegacyCardCacheReloadsByContent(t *testing.T) {
	levelsRoot := t.TempDir()
	levelDir := filepath.Join(levelsRoot, "cache-content")
	if err := os.MkdirAll(levelDir, 0o755); err != nil {
		t.Fatalf("mkdir level: %v", err)
	}
	cardYAML := `schema_version: vit.level_card.v0
id: cache-content
title: 缓存内容版一
lead_default: human
players: [human, model]
timing:
  bpm_default: 80
  time_signature: "2/4"
  beats_per_bar: 2
  pickup_beats: 0
  bar_span: { first_bar: 1, last_bar: 1 }
runtime_slots:
  t1: { track_index: 1, kind: workspace, preload: null, initial_state: empty }
objectives: []
nodes:
  - id: n1
    title: 第一步
    lead: human
    goal: 做点什么
    done_when: { structure: track_non_empty, track: t1 }
guidance: { pullback_after_turns: 6 }
write_boundary: { allow: [create_midi_clip], confirm_downgrade: none }
finale:
  completion_before_export: true
  closure_check:
    structure_complete: { all_nodes_done: true }
    soft_signals:
      user_closing_language: { role: weighted, required: true }
  settle: { objectives_check: all, trajectory_archive: true }
budget: { max_turns: 60, max_llm_calls: 120, on_stop_loss: preserve_and_pause }
`
	cardPath := filepath.Join(levelDir, "level.yaml")
	if err := os.WriteFile(cardPath, []byte(cardYAML), 0o644); err != nil {
		t.Fatalf("write card: %v", err)
	}
	// 直接走文件装载（禁 level_id 单键：缓存按路径+内容键，内容变化后重读）。
	first, err := loadEduJamCardFile(cardPath)
	if err != nil {
		t.Fatalf("load card v1: %v", err)
	}
	if first.Title != "缓存内容版一" {
		t.Fatalf("card v1 title=%q", first.Title)
	}
	// 修改内容（标题）→同一 level id/路径内容变化必须重载出新版本。
	revised := strings.Replace(cardYAML, "缓存内容版一", "缓存内容版二", 1)
	if err := os.WriteFile(cardPath, []byte(revised), 0o644); err != nil {
		t.Fatalf("write revised card: %v", err)
	}
	second, err := loadEduJamCardFile(cardPath)
	if err != nil {
		t.Fatalf("load card v2: %v", err)
	}
	if second.Title != "缓存内容版二" {
		t.Fatalf("同 level id 内容变化未重载（缓存串版本）: %q", second.Title)
	}
}
