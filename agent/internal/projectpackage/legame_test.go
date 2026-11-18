package projectpackage

// W01-C 工程绑定零栈单测：R2 元数据启用/缺省兼容、M1 导入装配、A/B 不串，
// 重开/移动缓存命中、revise 失效重编译、手改卡 fail-closed、enabled=false 惰性。
// §10 纪律：全部落盘在 t.TempDir()；不启真栈、不碰 webview/内核。

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/legamedesign"
)

// ---- 测试装置 ----

func legameFloatPtr(v float64) *float64 { return &v }
func legameIntPtr(v int) *int           { return &v }

func legameTestOverrides() legamedesign.CompileOverrides {
	return legamedesign.CompileOverrides{
		TempoBPM:    legameFloatPtr(80),
		MaxTurns:    legameIntPtr(60),
		MaxLLMCalls: legameIntPtr(120),
	}
}

// legameTestProject 建伪工程文件路径（t.TempDir 下 <name>.vit）。
func legameTestProject(t *testing.T, name string) (string, string) {
	t.Helper()
	projectPath := filepath.Join(t.TempDir(), name+".vit")
	if err := os.WriteFile(projectPath, []byte("fake project bytes"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	return projectPath, "uuid-" + name
}

// legameTestSourceDir 建设计源目录（fixture 设计源 + catalog + 两份合成 MIDI）。
func legameTestSourceDir(t *testing.T) string {
	t.Helper()
	sourceDir := t.TempDir()
	designBytes, err := os.ReadFile(filepath.Join("..", "legamedesign", "testdata", "design.v1.yaml"))
	if err != nil {
		t.Fatalf("read design fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, LegameDesignFile), designBytes, 0o644); err != nil {
		t.Fatalf("write design: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "assets", "midi"), 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	catalog := `schema_version: vit.lesson_material_catalog.v0
items:
  - id: develop-prototype
    file: assets/midi/exercise2-prototype_demo-C_80bpm.mid
    status: generated_symbolic_checked
    sound_check: not_performed
  - id: reference-sanshilipu-rhythm
    file: assets/midi/sanshilipu-first8_rhythm-only_GM37_80bpm.mid
    status: generated_symbolic_checked
    sound_check: not_performed
`
	if err := os.WriteFile(filepath.Join(sourceDir, "assets", "catalog.yaml"), []byte(catalog), 0o644); err != nil {
		t.Fatalf("write catalog: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "assets", "midi", "exercise2-prototype_demo-C_80bpm.mid"), legameTestSMF(t, [][3]int{{60, 0, 480}, {62, 480, 480}}), 0o644); err != nil {
		t.Fatalf("write midi 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "assets", "midi", "sanshilipu-first8_rhythm-only_GM37_80bpm.mid"), legameTestSMF(t, [][3]int{{37, 0, 480}, {37, 480, 480}}), 0o644); err != nil {
		t.Fatalf("write midi 2: %v", err)
	}
	return sourceDir
}

// legameTestSMF 生成最小 SMF format 0（division=480）。
func legameTestSMF(t *testing.T, notes [][3]int) []byte {
	t.Helper()
	var body bytes.Buffer
	writeVLQ := func(v int) {
		stack := []byte{byte(v & 0x7F)}
		v >>= 7
		for v > 0 {
			stack = append(stack, byte(v&0x7F)|0x80)
			v >>= 7
		}
		for i := len(stack) - 1; i >= 0; i-- {
			body.WriteByte(stack[i])
		}
	}
	type event struct {
		tick  int
		on    bool
		pitch int
	}
	events := make([]event, 0, len(notes)*2)
	for _, n := range notes {
		events = append(events, event{tick: n[1], on: true, pitch: n[0]})
		events = append(events, event{tick: n[1] + n[2], on: false, pitch: n[0]})
	}
	for i := 1; i < len(events); i++ {
		for j := i; j > 0; j-- {
			if events[j].tick < events[j-1].tick || (events[j].tick == events[j-1].tick && !events[j].on && events[j-1].on) {
				events[j], events[j-1] = events[j-1], events[j]
			} else {
				break
			}
		}
	}
	last := 0
	for _, ev := range events {
		writeVLQ(ev.tick - last)
		last = ev.tick
		if ev.on {
			body.WriteByte(0x90)
		} else {
			body.WriteByte(0x80)
		}
		body.WriteByte(byte(ev.pitch))
		if ev.on {
			body.WriteByte(100)
		} else {
			body.WriteByte(0)
		}
	}
	body.WriteByte(0x00)
	body.WriteByte(0xFF)
	body.WriteByte(0x2F)
	body.WriteByte(0x00)
	var out bytes.Buffer
	out.WriteString("MThd")
	out.Write([]byte{0, 0, 0, 6})
	out.Write([]byte{0, 0, 0, 1, 0x01, 0xE0})
	out.WriteString("MTrk")
	length := body.Len()
	out.Write([]byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)})
	out.Write(body.Bytes())
	return out.Bytes()
}

func legameTestImport(t *testing.T, projectPath, projectUUID string) (*LegameImportResult, *legamedesign.Report) {
	t.Helper()
	result, report, err := ImportLegameDesign(LegameImportOptions{
		ProjectPath: projectPath,
		ProjectUUID: projectUUID,
		SourceDir:   legameTestSourceDir(t),
		Overrides:   legameTestOverrides(),
	})
	if err != nil {
		t.Fatalf("ImportLegameDesign: %v", err)
	}
	if report.HasErrors() {
		t.Fatalf("导入不应有 E: %s", report.String())
	}
	return result, report
}

// ---- binding 路径：按 project UUID 目录隔离（A/B 不串）；UUID 缺省兼容旧路径 ----

func TestLegameBindingPathUUIDIsolation(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "shared", "A.vit")
	projectB := filepath.Join(root, "shared", "B.vit")
	if err := os.MkdirAll(filepath.Dir(projectA), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pathAU := LegameBindingPath(projectA, "uuid-a")
	pathBU := LegameBindingPath(projectB, "uuid-b")
	pathA2 := LegameBindingPath(projectA, "uuid-a2")
	if pathAU == pathBU || pathAU == pathA2 {
		t.Fatalf("binding 路径未按 project UUID 隔离: %q %q %q", pathAU, pathBU, pathA2)
	}
	wantDir := filepath.Join(filepath.Dir(projectA), LegameBindingDirName, "uuid-a")
	if filepath.Dir(pathAU) != wantDir {
		t.Fatalf("binding 路径=%q want dir %q", pathAU, wantDir)
	}
	legacy := LegameLegacyBindingPath(projectA)
	if got := LegameBindingPath(projectA, ""); got != legacy {
		t.Fatalf("UUID 缺省兼容失败: %q != %q", got, legacy)
	}
	if filepath.Dir(legacy) != filepath.Join(filepath.Dir(projectA), LegameBindingDirName) {
		t.Fatalf("legacy binding 路径=%q", legacy)
	}
}

// ---- R2 元数据：缺省/写回/旧包兼容/enabled=false 惰性 ----

func TestLegameMetadataDefaultsAndLegacy(t *testing.T) {
	t.Run("no-package-is-disabled", func(t *testing.T) {
		projectPath, _ := legameTestProject(t, "plain")
		meta, found, err := LoadLegameMetadata(projectPath, "uuid-plain")
		if err != nil || found || meta.Enabled {
			t.Fatalf("无项目包应为普通工程（普通工程=乐局未启用）: meta=%+v found=%v err=%v", meta, found, err)
		}
	})
	t.Run("package-without-legame-is-disabled", func(t *testing.T) {
		projectPath, uuid := legameTestProject(t, "legacyplain")
		if _, err := ensureProjectPackage(projectPath, uuid); err != nil {
			t.Fatalf("ensure package: %v", err)
		}
		meta, found, err := LoadLegameMetadata(projectPath, uuid)
		if err != nil || !found || meta.Enabled {
			t.Fatalf("旧包缺 legame 段=普通工程: meta=%+v found=%v err=%v", meta, found, err)
		}
	})
	t.Run("manifest-mirror-compat", func(t *testing.T) {
		projectPath, uuid := legameTestProject(t, "mirror")
		packageDir, err := ensureProjectPackage(projectPath, uuid)
		if err != nil {
			t.Fatalf("ensure package: %v", err)
		}
		// 旧实现只写 manifest 镜像（无 legame.json）：读侧须兼容
		designDir := filepath.Join(packageDir, LegameDirName)
		if err := os.MkdirAll(designDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		designBytes, _ := os.ReadFile(filepath.Join("..", "legamedesign", "testdata", "design.v1.yaml"))
		if err := os.WriteFile(filepath.Join(designDir, LegameDesignFile), designBytes, 0o644); err != nil {
			t.Fatalf("write design: %v", err)
		}
		_, manifest, err := Discover(projectPath, uuid)
		if err != nil {
			t.Fatalf("discover: %v", err)
		}
		manifest.Legame = &LegameMetadata{SchemaVersion: LegameSchemaVersion, Enabled: true, Design: LegameDirName + "/" + LegameDesignFile}
		if err := writeManifest(packageDir, manifest); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		meta, found, err := LoadLegameMetadata(projectPath, uuid)
		if err != nil || !found || !meta.Enabled || meta.Design != LegameDirName+"/"+LegameDesignFile {
			t.Fatalf("manifest 镜像兼容读取失败: meta=%+v found=%v err=%v", meta, found, err)
		}
	})
	t.Run("enabled-requires-design-inside-package", func(t *testing.T) {
		projectPath, uuid := legameTestProject(t, "invalidmeta")
		err := SaveLegameMetadata(projectPath, uuid, LegameMetadata{Enabled: true, Design: "../../outside.yaml"})
		if err == nil {
			t.Fatalf("enabled=true 且 design 越出包根必须 fail-closed")
		}
	})
	t.Run("enabled-false-lazy-no-compile", func(t *testing.T) {
		projectPath, uuid := legameTestProject(t, "lazy")
		if err := SaveLegameMetadata(projectPath, uuid, LegameMetadata{Enabled: false}); err != nil {
			t.Fatalf("save disabled metadata: %v", err)
		}
		result, report, err := LoadLegameCard(projectPath, uuid)
		if err != nil || report != nil && report.HasErrors() {
			t.Fatalf("enabled=false 不得报错/编译: err=%v", err)
		}
		if result == nil || result.Enabled {
			t.Fatalf("enabled=false 应为惰性普通工程: %+v", result)
		}
		packageDir, _, err := Discover(projectPath, uuid)
		if err != nil {
			t.Fatalf("discover: %v", err)
		}
		if dirExists(filepath.Join(packageDir, LegameDirName, LegameCardsDirName)) {
			t.Fatalf("enabled=false 不得产生编译卡（惰性）")
		}
	})
}

// ---- M1 导入 → 编译装配进包 → 过现行卡装载；A/B 两工程独立 ----

func TestLegameImportAssembly(t *testing.T) {
	projectA, uuidA := legameTestProject(t, "A")
	resultA, reportA := legameTestImport(t, projectA, uuidA)
	if !resultA.Enabled || resultA.CardID == "" {
		t.Fatalf("导入结果=%+v", resultA)
	}
	if reportA == nil {
		t.Fatalf("导入须返回 W/I 诊断回执")
	}
	packageA, manifestA, err := Discover(projectA, uuidA)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if err := verifyManifest(packageA, manifestA); err != nil {
		t.Fatalf("导入后项目包 manifest 校验失败: %v", err)
	}
	if manifestA.Legame == nil || !manifestA.Legame.Enabled {
		t.Fatalf("manifest 须带 legame 启用镜像: %+v", manifestA.Legame)
	}
	for _, rel := range []string{resultA.DesignRel, resultA.CardRel, resultA.ManifestRel} {
		if !fileExistsTest(filepath.Join(packageA, filepath.FromSlash(rel))) {
			t.Fatalf("包内产物缺失: %s", rel)
		}
	}
	meta, found, err := LoadLegameMetadata(projectA, uuidA)
	if err != nil || !found || !meta.Enabled || meta.Design != resultA.DesignRel {
		t.Fatalf("元数据回读失败: meta=%+v found=%v err=%v", meta, found, err)
	}
	if meta.Overrides.TempoBPM == nil || *meta.Overrides.TempoBPM != 80 {
		t.Fatalf("R5 覆盖参数须随包持久化（重开/移动可复现）: %+v", meta.Overrides)
	}
	loaded, loadReport, err := LoadLegameCard(projectA, uuidA)
	if err != nil {
		t.Fatalf("LoadLegameCard: %v", err)
	}
	if loadReport != nil && loadReport.HasErrors() {
		t.Fatalf("Load 不应有 E")
	}
	if loaded == nil || !loaded.Enabled || loaded.Card == nil {
		t.Fatalf("装配卡装载失败: %+v", loaded)
	}
	if loaded.Card.ID != resultA.CardID || loaded.CacheKeyID == "" || !loaded.Cached {
		t.Fatalf("装配结果错: id=%s key=%s cached=%v", loaded.Card.ID, loaded.CacheKeyID, loaded.Cached)
	}
	if len(loaded.Card.Assets) != 2 || len(loaded.Card.Nodes) != 8 {
		t.Fatalf("装配卡内容错: assets=%d nodes=%d", len(loaded.Card.Assets), len(loaded.Card.Nodes))
	}

	// A/B 两工程独立：各自 binding 路径分离、装配产物分离、卡互不影响
	projectB, uuidB := legameTestProject(t, "B")
	resultB, _ := legameTestImport(t, projectB, uuidB)
	if LegameBindingPath(projectA, uuidA) == LegameBindingPath(projectB, uuidB) {
		t.Fatalf("A/B binding 路径不得相同")
	}
	packageB, _, err := Discover(projectB, uuidB)
	if err != nil {
		t.Fatalf("discover B: %v", err)
	}
	if packageA == packageB {
		t.Fatalf("A/B 项目包路径不得相同")
	}
	loadedB, _, err := LoadLegameCard(projectB, uuidB)
	if err != nil || loadedB.Card == nil {
		t.Fatalf("B 装配失败: %v", err)
	}
	if loadedB.CardPath == loaded.CardPath {
		t.Fatalf("A/B 卡文件路径不得相同")
	}
	if loadedB.CacheKeyID == "" {
		t.Fatalf("B 四元组缓存键为空")
	}
	_ = resultB
}

// ---- R5 fail-closed：无覆盖参数导入必须 E 回执且不写启用元数据 ----

func TestLegameImportR5FailClosed(t *testing.T) {
	projectPath, uuid := legameTestProject(t, "r5")
	sourceDir := legameTestSourceDir(t)
	result, report, err := ImportLegameDesign(LegameImportOptions{
		ProjectPath: projectPath, ProjectUUID: uuid, SourceDir: sourceDir,
	})
	if err == nil || result != nil {
		t.Fatalf("R5 无覆盖参数导入必须 fail-closed（无静默成功）")
	}
	if report == nil || !report.HasErrors() {
		t.Fatalf("须如实回执 E 诊断")
	}
	if got := report.RuleCount(legamedesign.RuleBudget); got != 3 {
		t.Fatalf("R5 期望 3 条 W-BUDGET E（tempo+2 budget），got %d:\n%s", got, report.String())
	}
	meta, _, metaErr := LoadLegameMetadata(projectPath, uuid)
	if metaErr != nil {
		t.Fatalf("load meta: %v", metaErr)
	}
	if meta.Enabled {
		t.Fatalf("导入失败不得写启用元数据（fail-closed）")
	}
	packageDir, _, derr := Discover(projectPath, uuid)
	if derr != nil {
		return // 未创建包亦可
	}
	if dirExists(legameCardsPath(packageDir)) {
		t.Fatalf("导入失败不得落编译卡")
	}
}

// ---- revise：源修订→缓存失效→重编译新卡；非法 revise/手改卡→fail-closed ----

func TestLegameReviseAndTamper(t *testing.T) {
	projectPath, uuid := legameTestProject(t, "revise")
	_, _ = legameTestImport(t, projectPath, uuid)
	packageDir, _, err := Discover(projectPath, uuid)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	first, _, err := LoadLegameCard(projectPath, uuid)
	if err != nil || first == nil || !first.Cached {
		t.Fatalf("首次重开应命中缓存: %+v err=%v", first, err)
	}
	firstKey := first.CacheKeyID
	if first.Card.Title != "把旋律的起伏写成自己的作品" {
		t.Fatalf("初始卡标题错: %q", first.Card.Title)
	}

	// revise 源（包内设计源副本）
	designAbs := filepath.Join(packageDir, LegameDirName, LegameDesignFile)
	designBytes, err := os.ReadFile(designAbs)
	if err != nil {
		t.Fatalf("read design: %v", err)
	}
	revised := strings.Replace(string(designBytes), "title: 把旋律的起伏写成自己的作品", "title: 修订后的标题", 1)
	if revised == string(designBytes) {
		t.Fatalf("revise 变异未生效")
	}
	if err := os.WriteFile(designAbs, []byte(revised), 0o644); err != nil {
		t.Fatalf("write revised design: %v", err)
	}
	second, report, err := LoadLegameCard(projectPath, uuid)
	if err != nil {
		t.Fatalf("revise 后 Load 应重编译成功: %v\n%s", err, reportString(report))
	}
	if second.Card.Title != "修订后的标题" {
		t.Fatalf("revise 后新卡未生效: %q", second.Card.Title)
	}
	if second.CacheKeyID == firstKey {
		t.Fatalf("revise 后缓存键必须变化（新卡生效）")
	}

	// 手改 compiled 卡 → manifest 拒绝 → 重编译修复
	cardBytes, err := os.ReadFile(second.CardPath)
	if err != nil {
		t.Fatalf("read card: %v", err)
	}
	tampered := append(append([]byte{}, cardBytes...), []byte("# hand edit")...)
	if err := os.WriteFile(second.CardPath, tampered, 0o644); err != nil {
		t.Fatalf("tamper card: %v", err)
	}
	repaired, _, err := LoadLegameCard(projectPath, uuid)
	if err != nil {
		t.Fatalf("手改卡后应 fail-closed 重编译修复: %v", err)
	}
	afterBytes, _ := os.ReadFile(repaired.CardPath)
	if bytes.Contains(afterBytes, []byte("# hand edit")) {
		t.Fatalf("手改卡未被 manifest 拒绝/修复")
	}

	// 非法 revise：E 回执 + 旧卡保留且不得视为可用
	if err := os.WriteFile(designAbs, []byte("schema_version: vit.legame_design.v0\nbogus_field: 1\n"), 0o644); err != nil {
		t.Fatalf("write invalid design: %v", err)
	}
	broken, brokenReport, err := LoadLegameCard(projectPath, uuid)
	if err == nil || broken != nil {
		t.Fatalf("非法 revise 必须 fail-closed（不得返回旧卡）")
	}
	if brokenReport == nil || !brokenReport.HasErrors() {
		t.Fatalf("非法 revise 须如实回执 E")
	}
	if !fileExistsTest(second.CardPath) {
		t.Fatalf("非法 revise 不得删除旧产物（保留但阻断使用）")
	}
	if _, verifyErr := legamedesign.VerifyCompiled(second.CardPath, second.ManifestPath, legamedesign.CompileInput{
		DesignPath: designAbs, PackageRoot: filepath.Dir(designAbs),
		Overrides:    legameTestOverrides(),
		Judgements:   legamedesign.FirstCourseJudgementRegistry(),
		Capabilities: legamedesign.FirstCourseCapabilityBridge(),
	}); verifyErr == nil {
		t.Fatalf("非法源下旧缓存不得通过校验（fail-closed）")
	}
}

func reportString(report *legamedesign.Report) string {
	if report == nil {
		return ""
	}
	return report.String()
}

// ---- 重开/移动：相对路径不变 → 四元组键命中（不切别版） ----

func TestLegameReopenAfterMove(t *testing.T) {
	root := t.TempDir()
	sourceDir := legameTestSourceDir(t)
	projectPath := filepath.Join(root, "before", "moved.vit")
	uuid := "uuid-moved"
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	if _, _, err := ImportLegameDesign(LegameImportOptions{
		ProjectPath: projectPath, ProjectUUID: uuid, SourceDir: sourceDir, Overrides: legameTestOverrides(),
	}); err != nil {
		t.Fatalf("import: %v", err)
	}
	before, _, err := LoadLegameCard(projectPath, uuid)
	if err != nil || before == nil || !before.Cached {
		t.Fatalf("移动前装配失败: %+v err=%v", before, err)
	}

	movedRoot := filepath.Join(t.TempDir(), "after")
	if err := os.Rename(filepath.Join(root, "before"), movedRoot); err != nil {
		t.Fatalf("move project dir: %v", err)
	}
	movedProject := filepath.Join(movedRoot, "moved.vit")
	after, report, err := LoadLegameCard(movedProject, uuid)
	if err != nil {
		t.Fatalf("移动后装配失败: %v\n%s", err, reportString(report))
	}
	if after.Card == nil || after.Card.ID != before.Card.ID {
		t.Fatalf("移动后切错版本: before=%s after=%v", before.Card.ID, after.Card)
	}
	if !after.Cached || after.CacheKeyID != before.CacheKeyID {
		t.Fatalf("移动后应命中同一四元组缓存键（相对路径不变）: cached=%v key=%s→%s", after.Cached, before.CacheKeyID, after.CacheKeyID)
	}
	// 装配目录随包移动且资产相对路径仍可解析
	if _, statErr := os.Stat(after.CardPath); statErr != nil {
		t.Fatalf("移动后卡文件不可达: %v", statErr)
	}
}

// ---- 保存/另存：乐局装配随项目包携带（Prepare/Commit 链路） ----

func TestLegameSaveCommitCarriesAssembly(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "dev")
	if err := os.MkdirAll(devRoot, 0o755); err != nil {
		t.Fatalf("mkdir devroot: %v", err)
	}
	t.Setenv("VIT_DAW_DEV_ROOT", devRoot)
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(devRoot, "mixboard"))
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(devRoot, "acoustic.json"))

	sourcePath := filepath.Join(root, "source", "song.vit")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("project"), 0o644); err != nil {
		t.Fatalf("write project: %v", err)
	}
	if _, _, err := ImportLegameDesign(LegameImportOptions{
		ProjectPath: sourcePath, ProjectUUID: "uuid-src", SourceDir: legameTestSourceDir(t), Overrides: legameTestOverrides(),
	}); err != nil {
		t.Fatalf("import: %v", err)
	}
	historyWorkspace := filepath.Join(root, "prepare", "workspace")
	if err := os.MkdirAll(filepath.Join(historyWorkspace, "state"), 0o755); err != nil {
		t.Fatalf("mkdir history: %v", err)
	}
	writeTestJSON(t, filepath.Join(historyWorkspace, "workspace.json"), map[string]any{"project_path": sourcePath})
	writeTestJSON(t, filepath.Join(historyWorkspace, "state", "agent_runtime_state.json"), map[string]any{"project_uuid": "uuid-src"})
	writeTestJSON(t, filepath.Join(historyWorkspace, "state", "conversation_graph.json"), map[string]any{"project_uuid": "uuid-src", "nodes": []any{}})

	if _, err := Prepare(PrepareOptions{
		ProjectPath: sourcePath, ProjectUUID: "uuid-src", PrepareID: "prepare_legame",
		GenerationID: "save_legame", SaveKind: "save", HistoryWorkspace: historyWorkspace,
	}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	targetPath := filepath.Join(root, "target", "song-saved.vit")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("project saved"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	targetHistory := filepath.Join(filepath.Dir(targetPath), ".vit_history", "uuid-tgt")
	if err := os.MkdirAll(filepath.Join(targetHistory, "state"), 0o755); err != nil {
		t.Fatalf("mkdir target history: %v", err)
	}
	writeTestJSON(t, filepath.Join(targetHistory, "workspace.json"), map[string]any{"project_path": targetPath})
	writeTestJSON(t, filepath.Join(targetHistory, "state", "agent_runtime_state.json"), map[string]any{"project_uuid": "uuid-tgt"})
	writeTestJSON(t, filepath.Join(targetHistory, "state", "conversation_graph.json"), map[string]any{"project_uuid": "uuid-tgt", "nodes": []any{}})
	manifest, _, err := Commit(CommitOptions{
		SourcePath: sourcePath, SourceUUID: "uuid-src", TargetPath: targetPath, TargetUUID: "uuid-tgt",
		PrepareID: "prepare_legame", GenerationID: "save_legame", SaveKind: "save_as",
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if manifest.Legame == nil || !manifest.Legame.Enabled {
		t.Fatalf("另存后项目包须携带 legame 元数据: %+v", manifest.Legame)
	}
	loaded, report, err := LoadLegameCard(targetPath, "uuid-tgt")
	if err != nil {
		t.Fatalf("另存后装配失败: %v\n%s", err, reportString(report))
	}
	if loaded == nil || !loaded.Enabled || loaded.Card == nil || loaded.Card.ID != "music-creation-unit2-demo" {
		t.Fatalf("另存后乐局卡丢失/切版: %+v", loaded)
	}
}

// ---- 普通工程转乐局：只写元数据+指向设计源，装载时按需编译（R2） ----

func TestLegameEnableOnDemandCompiles(t *testing.T) {
	projectPath, uuid := legameTestProject(t, "ondemand")
	packageDir, err := ensureProjectPackage(projectPath, uuid)
	if err != nil {
		t.Fatalf("ensure package: %v", err)
	}
	sourceDir := legameTestSourceDir(t)
	if err := copyDir(sourceDir, filepath.Join(packageDir, LegameDirName)); err != nil {
		t.Fatalf("copy design tree: %v", err)
	}
	meta := LegameMetadata{
		Enabled:   true,
		Design:    LegameDirName + "/" + LegameDesignFile,
		Overrides: LegameOverrides{TempoBPM: legameFloatPtr(80), MaxTurns: legameIntPtr(60), MaxLLMCalls: legameIntPtr(120)},
	}
	if err := SaveLegameMetadata(projectPath, uuid, meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	loaded, report, err := LoadLegameCard(projectPath, uuid)
	if err != nil {
		t.Fatalf("按需编译失败: %v\n%s", err, reportString(report))
	}
	if loaded == nil || !loaded.Enabled || loaded.Card == nil || loaded.Cached {
		t.Fatalf("按需编译应产出新卡: %+v", loaded)
	}
	if dirExists(filepath.Join(packageDir, LegameDirName, LegameCardsDirName)) == false {
		t.Fatalf("按需编译后须有装配产物")
	}
}

// ---- 静态 helper ----

func fileExistsTest(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// compile-time pin: legame metadata JSON 形态（manifest.legame 字段名稳定）
func TestLegameManifestJSONShape(t *testing.T) {
	projectPath, uuid := legameTestProject(t, "json")
	if err := SaveLegameMetadata(projectPath, uuid, LegameMetadata{Enabled: false}); err != nil {
		t.Fatalf("save: %v", err)
	}
	packageDir, _, err := Discover(projectPath, uuid)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(packageDir, ManifestFile))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	legame, ok := decoded["legame"].(map[string]any)
	if !ok {
		t.Fatalf("manifest 缺 legame 段: %v", decoded["legame"])
	}
	if enabled, ok := legame["enabled"].(bool); !ok || enabled {
		t.Fatalf("manifest.legame.enabled 形态错: %v", legame["enabled"])
	}
}
