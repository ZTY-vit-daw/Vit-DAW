package legamedesign

// W01-B 编译器测试（G0-LEGAME-COMPILE-1）：映射表 T-1..T-21 逐行 + manifest/缓存/准入。
// §10 纪律：所有落盘写 t.TempDir()，coord/lesson-designs 原件只读不写。

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/edu"
)

// ---- 测试装置：临时工程包（设计源副本 + catalog + 两份合成 MIDI）----

const (
	testMIDIRelPrototype = "assets/midi/exercise2-prototype_demo-C_80bpm.mid"
	testMIDIRelTheme     = "assets/midi/sanshilipu-first8_rhythm-only_GM37_80bpm.mid"
)

// testOverrides 是 R5 显式覆盖参数（fixture 设计源 tempo/budget 均为 null）。
func testOverrides() CompileOverrides {
	return CompileOverrides{TempoBPM: floatPtr(80), MaxTurns: intPtr(60), MaxLLMCalls: intPtr(120)}
}

type testPackage struct {
	root       string
	designPath string
}

// setupTestPackage 建可编译工程包：设计源=testdata 副本（CRLF 原样），catalog 达标，
// 两份被引用 MIDI 合成（catalog 状态 generated_symbolic_checked）。
func setupTestPackage(t *testing.T) *testPackage {
	t.Helper()
	p := &testPackage{root: t.TempDir()}
	p.designPath = filepath.Join(p.root, "design.v1.yaml")
	writeTestPackageFile(t, p.root, "design.v1.yaml", string(readFixture(t)))
	p.writeCatalog(t, "generated_symbolic_checked", "generated_symbolic_checked")
	writeTestPackageFile(t, p.root, testMIDIRelPrototype, string(buildTestMIDI(t, [][3]int{{60, 0, 480}, {62, 480, 480}})))
	writeTestPackageFile(t, p.root, testMIDIRelTheme, string(buildTestMIDI(t, [][3]int{{37, 0, 480}, {37, 480, 480}})))
	return p
}

func (p *testPackage) input(t *testing.T) CompileInput {
	t.Helper()
	return CompileInput{
		DesignPath:   p.designPath,
		PackageRoot:  p.root,
		OutputDir:    filepath.Join(t.TempDir(), "out"),
		Overrides:    testOverrides(),
		Judgements:   FirstCourseJudgementRegistry(),
		Capabilities: FirstCourseCapabilityBridge(),
	}
}

// rewriteDesign 以 LF 归一文本做定向变异后覆盖设计源副本（只动 t.TempDir 副本）。
func (p *testPackage) rewriteDesign(t *testing.T, mutateFunc func(src string) string) {
	t.Helper()
	src := strings.ReplaceAll(string(readFixture(t)), "\r\n", "\n")
	if strings.ContainsRune(src, '\r') {
		t.Fatalf("设计源副本含孤立 CR")
	}
	out := mutateFunc(src)
	if out == src {
		t.Fatalf("设计源变异未生效")
	}
	writeTestPackageFile(t, p.root, "design.v1.yaml", out)
}

func (p *testPackage) writeCatalog(t *testing.T, statusPrototype, statusTheme string) {
	t.Helper()
	writeTestPackageFile(t, p.root, "assets/catalog.yaml", testCatalogYAML(statusPrototype, statusTheme))
}

func testCatalogYAML(statusPrototype, statusTheme string) string {
	return `schema_version: vit.lesson_material_catalog.v0
items:
  - id: develop-prototype
    file: assets/midi/exercise2-prototype_demo-C_80bpm.mid
    status: ` + statusPrototype + `
    sound_check: not_performed
  - id: reference-sanshilipu-rhythm
    file: assets/midi/sanshilipu-first8_rhythm-only_GM37_80bpm.mid
    status: ` + statusTheme + `
    sound_check: not_performed
`
}

func writeTestPackageFile(t *testing.T, pkgRoot, rel, content string) {
	t.Helper()
	abs := filepath.Join(pkgRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// replaceOnce 变异的无计数检查版本（同一锚点多处出现时用 strings.Replace 首处）。
func replaceOnce(t *testing.T, src, oldS, newS string) string {
	t.Helper()
	if !strings.Contains(src, oldS) {
		t.Fatalf("变异锚点不存在")
	}
	return strings.Replace(src, oldS, newS, 1)
}

// mustCompile 编译并断言 0 E；否则 fatal 附诊断。
func mustCompile(t *testing.T, input CompileInput) *CompileResult {
	t.Helper()
	res, rep := Compile(input)
	if rep.HasErrors() {
		t.Fatalf("期望 0 E，实际: %s", rep.String())
	}
	if res == nil {
		t.Fatalf("0 E 但返回 nil CompileResult")
	}
	return res
}

// compileCard 编译并返回现行校验门解析后的卡。
func compileCard(t *testing.T, input CompileInput) (*CompileResult, *edu.LevelCard) {
	t.Helper()
	res := mustCompile(t, input)
	card, err := edu.ParseLevelCard(res.CardYAML)
	if err != nil {
		t.Fatalf("产物未过 edu.ParseLevelCard: %v", err)
	}
	if err := edu.ValidateLevelCard(card); err != nil {
		t.Fatalf("产物未过 edu.ValidateLevelCard: %v", err)
	}
	return res, card
}

// buildTestMIDI 生成最小 SMF format 0（division=480；notes=[pitch,startTicks,lenTicks]）。
func buildTestMIDI(t *testing.T, notes [][3]int) []byte {
	t.Helper()
	var body bytes.Buffer
	var writeVLQ func(v int)
	writeVLQ = func(v int) {
		if v < 0 {
			t.Fatalf("VLQ 负值")
		}
		var stack []byte
		stack = append(stack, byte(v&0x7F))
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

// ---- 验收锚点：music-creation-unit2 fixture 编译产物过现行 ValidateLevelCard ----

func TestCompileFirstCourseZeroRejections(t *testing.T) {
	p := setupTestPackage(t)
	input := p.input(t)
	res, card := compileCard(t, input)

	// T-1 卡 schema 版本
	if card.SchemaVersion != edu.LevelCardSchemaVersion {
		t.Fatalf("card schema_version=%q", card.SchemaVersion)
	}
	// T-2
	if card.ID != "music-creation-unit2-demo" || card.Title == "" {
		t.Fatalf("T-2 id/title 映射错: id=%q title=%q", card.ID, card.Title)
	}
	// T-4/T-5
	if card.LeadDefault != "human" || len(card.Players) != 2 {
		t.Fatalf("T-4/T-5 lead=%q players=%v", card.LeadDefault, card.Players)
	}
	// T-6/T-7/T-9
	if card.Timing.TimeSignature != "2/4" || card.Timing.BeatsPerBar != 2 || card.Timing.BPMDefault != 80 || card.Timing.PickupBeats != 0 {
		t.Fatalf("T-6/7/8/9 timing 映射错: %+v", card.Timing)
	}
	// T-11
	if len(card.Timing.ClipOrigins) != 2 || card.Timing.BarSpan.FirstBar != 1 || card.Timing.BarSpan.LastBar != 8 {
		t.Fatalf("T-11 clip_origins/bar_span 映射错: origins=%v span=%+v", card.Timing.ClipOrigins, card.Timing.BarSpan)
	}
	if o, ok := card.Timing.ClipOrigins["rhythm-lab"]; !ok || o.FirstBar != 1 || o.StartBeatsGlobal != 0 {
		t.Fatalf("T-11 rhythm-lab origin=%+v ok=%v", o, ok)
	}
	// T-12 runtime_slots（5 轨；声明序坐标；kind 映射）
	if len(card.RuntimeSlots) != 5 {
		t.Fatalf("T-12 runtime_slots=%d", len(card.RuntimeSlots))
	}
	wantSlotKinds := map[string]struct {
		index int
		kind  string
	}{
		"reference-track": {1, "reference"},
		"melody-track":    {2, "workspace"},
		"rhythm-track":    {3, "blank_to_fill"},
		"theme-track":     {4, "workspace"},
		"result-track":    {5, "workspace"},
	}
	for name, want := range wantSlotKinds {
		got, ok := card.RuntimeSlots[name]
		if !ok || got.TrackIndex != want.index || got.Kind != want.kind {
			t.Fatalf("T-12 slot %s=%+v ok=%v（want index=%d kind=%s）", name, got, ok, want.index, want.kind)
		}
		if got.Preload != nil || got.InitialState != "empty" {
			t.Fatalf("T-12 slot %s initial_state/preload 错: %+v", name, got)
		}
	}
	// T-13 assets（两份达标 MIDI）+ notes 表
	if len(card.Assets) != 2 {
		t.Fatalf("T-13 assets=%d（期望 2）", len(card.Assets))
	}
	for i := range card.Assets {
		a := &card.Assets[i]
		if a.Kind != "midi_reference" || len(a.SHA256) != 64 || a.NotesRef == "" {
			t.Fatalf("T-13 asset 映射错: %+v", a)
		}
		if a.Key == nil || a.Key.Mode != "major" || a.Key.Tonic != 0 {
			t.Fatalf("T-13 asset.key 映射错（C 大调）: %+v", a.Key)
		}
		tableName := strings.TrimPrefix(a.NotesRef, "notes.")
		if len(card.Notes[tableName]) != 2 {
			t.Fatalf("T-13 notes 表 %q=%v", tableName, card.Notes[tableName])
		}
		// MAT-FILE：卡资产 sha256 = 包内真实文件 sha256
		abs := filepath.Join(p.root, filepath.FromSlash(a.File))
		data, err := os.ReadFile(abs)
		if err != nil {
			t.Fatalf("T-13 卡资产文件缺失: %v", err)
		}
		if sha256Hex(data) != a.SHA256 {
			t.Fatalf("T-13 卡资产 sha256 与文件不符: %s", a.File)
		}
	}
	// T-15 objectives：1+2+3+2+3=11 条（R4 拆分 id#k）
	if len(card.Objectives) != 11 {
		t.Fatalf("T-15 objectives=%d（期望 11）", len(card.Objectives))
	}
	ids := map[string]bool{}
	for _, o := range card.Objectives {
		ids[o.ID] = true
	}
	for _, want := range []string{"g1-understand", "g2-contours#1", "g2-contours#2", "g3-develop#1", "g3-develop#2", "g3-develop#3", "g4-theme#1", "g4-theme#2", "task-song#1", "task-song#2", "task-song#3"} {
		if !ids[want] {
			t.Fatalf("T-15 缺卡目标 id %q（实际 %v）", want, ids)
		}
	}
	// T-16 nodes：8 节点、done_when 全可运行、teaching 映射
	if len(card.Nodes) != 8 {
		t.Fatalf("T-16 nodes=%d", len(card.Nodes))
	}
	wantDone := map[string]string{
		"n0-intent":   "event:user_confirmed",
		"n1-listen":   "event:user_feedback_given",
		"n2-rhythm":   "structure:clip_exists:melody-track",
		"n3-contours": "structure:clip_exists:melody-track",
		"n4-develop":  "structure:clip_exists:melody-track",
		"n5-theme":    "structure:clip_exists:theme-track",
		"n6-song":     "structure:track_non_empty:result-track",
		"n7-reflect":  "structure:track_non_empty:result-track",
	}
	wantTeaching := map[string][]string{
		"n1-listen":   {"g1-understand"},
		"n3-contours": {"g2-contours#2"},
		"n4-develop":  {"g3-develop#2", "g3-develop#3"},
		"n5-theme":    {"g4-theme#2"},
		"n6-song":     {"task-song#2", "task-song#3"},
	}
	for i := range card.Nodes {
		n := &card.Nodes[i]
		got := describeEduDoneWhen(n.DoneWhen)
		if want := wantDone[n.ID]; got != want {
			t.Fatalf("T-16 node %s done_when=%s（want %s）", n.ID, got, want)
		}
		if n.Goal == "" {
			t.Fatalf("T-16 node %s goal 空", n.ID)
		}
		if wantObjs, ok := wantTeaching[n.ID]; ok {
			if strings.Join(n.Teaching.Objectives, ",") != strings.Join(wantObjs, ",") {
				t.Fatalf("T-16 node %s teaching.objectives 错: %+v（want %v）", n.ID, n.Teaching, wantObjs)
			}
			if n.Teaching.Note == "" {
				t.Fatalf("T-16 node %s teaching.note 空（teaching_points join 丢失）", n.ID)
			}
		}
	}
	// T-16 teaching.note join "；"
	if !strings.Contains(card.Nodes[1].Teaching.Note, "；") {
		t.Fatalf("T-16 teaching.note 未按「；」join: %+v", card.Nodes[1].Teaching)
	}
	// T-17 write_boundary（闭包表并集，排序去重；confirm_downgrade=none）
	if len(card.WriteBoundary.Allow) != 9 || card.WriteBoundary.ConfirmDowngrade != "none" {
		t.Fatalf("T-17 write_boundary=%+v", card.WriteBoundary)
	}
	for i := 1; i < len(card.WriteBoundary.Allow); i++ {
		if card.WriteBoundary.Allow[i-1] >= card.WriteBoundary.Allow[i] {
			t.Fatalf("T-17 allow 未排序去重: %v", card.WriteBoundary.Allow)
		}
	}
	// T-18 guidance
	if card.Guidance.PullbackAfterTurns != 6 || card.Guidance.Soft == "" || card.Guidance.Avoid == nil || len(card.Guidance.Avoid) != 0 {
		t.Fatalf("T-18 guidance=%+v", card.Guidance)
	}
	// T-19 finale + settle 模板
	fin := card.Finale
	if !fin.CompletionBeforeExport || !fin.ClosureCheck.StructureComplete.AllNodesDone {
		t.Fatalf("T-19 finale 基础模板错: %+v", fin)
	}
	if got := fin.ClosureCheck.StructureComplete.NoPlaceholderBlanks; len(got) != 1 || got[0] != "rhythm-track" {
		t.Fatalf("T-19 no_placeholder_blanks=%v（want blank_to_fill 槽）", got)
	}
	if !fin.ClosureCheck.SoftSignals.UserClosingLanguage.Required {
		t.Fatalf("T-19 user_confirm_closure=true → user_closing_language.required 须 true")
	}
	if fin.Settle.ObjectivesCheck != "c-q1-structure,c-q2-structure,c-q3-structure,c-generation-record,c-reports" {
		t.Fatalf("T-19 settle.objectives_check=%q", fin.Settle.ObjectivesCheck)
	}
	if !fin.Settle.TrajectoryArchive || len(fin.Settle.MemoryWrites) != 2 {
		t.Fatalf("T-19 settle=%+v", fin.Settle)
	}
	// T-20 budget
	if card.Budget.MaxTurns != 60 || card.Budget.MaxLLMCalls != 120 || card.Budget.OnStopLoss != "preserve_and_pause" {
		t.Fatalf("T-20 budget=%+v", card.Budget)
	}

	// W 清单对账（源校验 W-MAT 3 + 编译 W-MAT 10 + W-CHK 5 + W-MAP 5 = 23）
	if got := resReportRuleCount(t, res, 0); got != 23 {
		t.Fatalf("W 清单=23（实际 %d）:", got)
	}
	if got := res.Manifest.Receipt.PendingQuestions; len(got) != 9 {
		t.Fatalf("T-21 manifest pending_questions=%v", got)
	}
	if len(res.Manifest.Receipt.ExcludedMaterials) != 13 {
		t.Fatalf("T-13 待备料清单=%d（期望 13）", len(res.Manifest.Receipt.ExcludedMaterials))
	}
	if len(res.Manifest.Receipt.AdmittedAssets) != 2 {
		t.Fatalf("T-13 准入资产回执=%d", len(res.Manifest.Receipt.AdmittedAssets))
	}
	if len(res.Manifest.Receipt.TrackMapping) != 5 {
		t.Fatalf("R3 manifest track_mapping=%d", len(res.Manifest.Receipt.TrackMapping))
	}
	// T-21：卡本体不得带 pending_questions/设计层字段
	if bytes.Contains(res.CardYAML, []byte("pending_questions")) || bytes.Contains(res.CardYAML, []byte("key_signature")) {
		t.Fatalf("T-10/T-21：卡本体含设计层字段")
	}
	// 确定性：同输入两次编译产出字节一致
	res2 := mustCompile(t, input)
	if !bytes.Equal(res.CardYAML, res2.CardYAML) {
		t.Fatalf("编译不确定：两次卡字节不同")
	}
	m1, _ := json.Marshal(res.Manifest)
	m2, _ := json.Marshal(res2.Manifest)
	if !bytes.Equal(m1, m2) {
		t.Fatalf("编译不确定：两次 manifest 不同")
	}
}

// resReportRuleCount 用 manifest W 清单计数（编译结果不直接暴露 Report）。
func resReportRuleCount(t *testing.T, res *CompileResult, _ int) int {
	t.Helper()
	return len(res.Manifest.Warnings)
}

// describeEduDoneWhen 把 edu.DoneWhen 压成可比较字符串（仅本测试使用的叶形态）。
func describeEduDoneWhen(dw edu.DoneWhen) string {
	if len(dw.All) == 1 {
		return describeEduDoneWhen(dw.All[0])
	}
	if len(dw.All) > 1 {
		return "all"
	}
	if len(dw.Any) > 0 {
		return "any"
	}
	if dw.Leaf == nil {
		return "none"
	}
	leaf := dw.Leaf
	if leaf.Event != "" {
		return "event:" + leaf.Event
	}
	if leaf.Structure != "" {
		s := "structure:" + leaf.Structure
		if leaf.Clip != "" {
			s += ":" + leaf.Clip
		}
		if leaf.Track != "" {
			s += ":" + leaf.Track
		}
		return s
	}
	if leaf.Judge != "" {
		return "judge:" + leaf.Judge
	}
	return "unknown"
}

// ---- 设计源锚点（fixture 副本 LF 归一后逐条变异；内容与 revision 3 逐字节对应）----

const (
	fixtureBriefBlock = `brief:
  audience: 有基本音符时值和简短旋律阅读经验的高中生
  music_task: 试写四种旋律线，发展一个喜欢的片段，再参照给定节奏创作八小节主题并听见成曲
  final_deliverable: 四条两小节片段、一个发展稿、八小节主题、现场试听与生成版本、作业报表和过程摘要
  prerequisites: [基本时值与休止, 拍号基础, 简短旋律阅读或可用的入门引导]
  estimated_minutes: null # 不以固定课时限制学生/模型的实际进度
`

	fixtureFinalDeliverableLine = `  final_deliverable: 四条两小节片段、一个发展稿、八小节主题、现场试听与生成版本、作业报表和过程摘要
`

	fixtureTimingSignatureComment = `    time_signature: 2/4
    key_signature: C # 记谱拼写与YuE2 ABC K:的默认上下文；材料可自带覆盖（2026-11-12裁定）
`

	fixturePickupLine = `    pickup_beats: 0
`

	fixtureRhythmLabLocator = `      locator: {clip_alias: rhythm-lab, time_signature: 2/4, bars: 4}
`

	fixtureThemeLocator = `      locator: {clip_alias: eight-bar-theme, time_signature: 2/4, bars: 8}
`

	fixtureGuidanceBlock = `guidance:
  approach: 以学生作品贯穿的共同创作；短讲解和即时实践交替，三题逐步完成
  help_policy: 先问意图和具体位置，再局部对比/示范；逐渐减少帮助并记录实际帮助
  exploration_policy: 节点内允许提问/重听/重试；返回修改产生新版本，不清除前面的证据
  learner_decision_policy: 主旋律与节奏决定由学生作出；模型负责材料、定位、解释与已约定辅助编曲
`

	fixtureEvidenceRecordingBlock = `evidence_recording:
  requested_granularity: [key_action, region, saved_revision]
  record_initiator_and_executor: true
  record_help_disclosed: true
  record_verified_progression: true
  pin_submission_log_range: true
  unknown_source: mark_unverified
  checkpoints: [node_entry, meaningful_revision, submission]
`

	fixtureSubmissionBlock = `submission:
  freeze_project_and_evidence_snapshot: true
  teacher_review_required: true
  review_by_objective: true
  retain_previous_submissions: true
  on_return: preserve_history_and_continue
  reports: [assignment_report, key_context_summary]
  include_raw_evidence_index: true
  summary_claims_require_evidence_refs: true
`

	fixtureCompletionBlock = `completion:
  required_deliverables: [四条两小节谱稿, 原型与发展稿, 八小节主题, 生成阶段的真实结果记录, 作业报表与过程摘要]
  required_checks: [c-q1-structure, c-q2-structure, c-q3-structure, c-generation-record, c-reports]
  learning_evidence_to_record: [c-listening-explanation, c-q1-explanation, c-q2-explanation, c-humming, c-q3-reference-and-expression, c-version-choice]
  user_confirm_closure: true
  export: optional
`
)

// compileReport 断言编译失败并返回诊断。
func compileReport(t *testing.T, input CompileInput) *Report {
	t.Helper()
	res, rep := Compile(input)
	if rep == nil {
		t.Fatalf("Compile 返回 nil report")
	}
	if !rep.HasErrors() {
		t.Fatalf("期望编译失败（E），实际 0 E 且 res=%v", res != nil)
	}
	return rep
}

// compileOK 断言 0 E 并返回结果+诊断。
func compileOK(t *testing.T, input CompileInput) (*CompileResult, *Report) {
	t.Helper()
	res, rep := Compile(input)
	if rep.HasErrors() {
		t.Fatalf("期望 0 E，实际: %s", rep.String())
	}
	if res == nil {
		t.Fatalf("0 E 但返回 nil CompileResult")
	}
	return res, rep
}

// ---- T-2：id/title 直拷与卡拒绝空值 ----

func TestT02IDTitleRequired(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "id: music-creation-unit2-demo", "id: ''")
	})
	assertDiag(t, compileReport(t, p.input(t)), SeverityError, RuleRequired, "id")

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "title: 把旋律的起伏写成自己的作品", "title: ''")
	})
	assertDiag(t, compileReport(t, p2.input(t)), SeverityError, RuleRequired, "title")
}

// ---- T-3：brief → story 模板句（缺→W+兜底）----

func TestT03BriefStoryTemplate(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string { return mutate(t, src, fixtureBriefBlock, "") })
	res, rep := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	if card.Story != "（设计源未提供剧情句）" {
		t.Fatalf("T-3 brief 缺兜底句错: %q", card.Story)
	}
	assertDiag(t, rep, SeverityWarning, RuleDefault, "brief")

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string { return mutate(t, src, fixtureFinalDeliverableLine, "") })
	res2, rep2 := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if !strings.Contains(card2.Story, "本局任务") || strings.Contains(card2.Story, "最终交付") {
		t.Fatalf("T-3 brief 部分缺失 story=%q", card2.Story)
	}
	assertDiag(t, rep2, SeverityWarning, RuleDefault, "brief.final_deliverable")
}

// ---- T-4：lead 一致性 → lead_default（混写→E-FLOW）----

func TestT04LeadDefault(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, `      lead: human
      uses_materials: []
      uses_regions: [r-theme]`, `      lead: model
      uses_materials: []
      uses_regions: [r-theme]`)
	})
	assertDiag(t, compileReport(t, p.input(t)), SeverityError, RuleFlow, "flow.nodes")

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string { return strings.ReplaceAll(src, "lead: human", "lead: model") })
	res, _ := compileOK(t, p2.input(t))
	card, _ := res.LevelCard()
	if card.LeadDefault != "model" {
		t.Fatalf("T-4 全 model → lead_default=%q", card.LeadDefault)
	}
}

// ---- T-6/T-7/T-9：time_signature / beats_per_bar / pickup_beats ----

func TestT06TimeSignatureRequired(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		src = mutate(t, src, fixtureTimingSignatureComment, `    key_signature: C # 记谱拼写与YuE2 ABC K:的默认上下文；材料可自带覆盖（2026-11-12裁定）
`)
		src = replaceOnce(t, src, fixtureRhythmLabLocator, `      locator: {clip_alias: rhythm-lab, bars: 4}
`)
		src = mutate(t, src, "locator: {scope: four_clips, clip_aliases: [q1-free, q1-flat, q1-smooth, q1-wave], time_signature: 2/4, bars_per_clip: 4}", "locator: {scope: four_clips, clip_aliases: [q1-free, q1-flat, q1-smooth, q1-wave], bars_per_clip: 4}")
		src = mutate(t, src, `      locator: {clip_alias: eight-bar-theme, time_signature: 2/4, bars: 8}
`, `      locator: {clip_alias: eight-bar-theme, bars: 8}
`)
		return src
	})
	rep := compileReport(t, p.input(t))
	assertDiag(t, rep, SeverityError, RuleRequired, "workspace.timing.time_signature")
}

func TestT07BeatsPerBar(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return strings.ReplaceAll(src, "time_signature: 2/4", "time_signature: 4/4")
	})
	res, _ := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	if card.Timing.TimeSignature != "4/4" || card.Timing.BeatsPerBar != 4 {
		t.Fatalf("T-7 4/4 → ts=%q bpb=%d", card.Timing.TimeSignature, card.Timing.BeatsPerBar)
	}
}

func TestT09PickupBeats(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return mutate(t, src, fixturePickupLine, `    pickup_beats: 0.5
`)
	})
	res, _ := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	if card.Timing.PickupBeats != 0.5 {
		t.Fatalf("T-9 pickup_beats=%v", card.Timing.PickupBeats)
	}

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string { return mutate(t, src, fixturePickupLine, "") })
	res2, _ := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if card2.Timing.PickupBeats != 0 {
		t.Fatalf("T-9 pickup_beats 缺省须 0（got %v）", card2.Timing.PickupBeats)
	}
}

// ---- T-8/T-20/R5：tempo/budget fail-closed 与覆盖参数 ----

func TestT08TempoBudgetR5(t *testing.T) {
	p := setupTestPackage(t)
	base := p.input(t)

	noOverride := base
	noOverride.Overrides = CompileOverrides{}
	rep := compileReport(t, noOverride)
	got := 0
	for _, d := range rep.Diagnostics() {
		if d.Rule == RuleBudget && d.Severity == SeverityError {
			got++
		}
	}
	if got != 3 {
		t.Fatalf("R5 无覆盖：期望 tempo+2 budget=3 条 E（W-BUDGET），got %d: %s", got, rep.String())
	}
	for _, path := range []string{"workspace.timing.tempo_bpm", "budget.max_turns", "budget.max_llm_calls"} {
		diag := assertDiag(t, rep, SeverityError, RuleBudget, path)
		if !strings.Contains(diag.Message, "覆盖参数") {
			t.Fatalf("R5 诊断须可行动: %s", diag.Message)
		}
	}

	tempoOnly := base
	tempoOnly.Overrides = CompileOverrides{TempoBPM: floatPtr(80)}
	rep = compileReport(t, tempoOnly)
	for _, path := range []string{"budget.max_turns", "budget.max_llm_calls"} {
		assertDiag(t, rep, SeverityError, RuleBudget, path)
	}
	if rep.RuleCount(RuleBudget) != 2 {
		t.Fatalf("R5 缺 budget 两数值须 2 E，got %d", rep.RuleCount(RuleBudget))
	}

	res, rep := compileOK(t, base)
	card, _ := res.LevelCard()
	if card.Timing.BPMDefault != 80 || card.Budget.MaxTurns != 60 || card.Budget.MaxLLMCalls != 120 {
		t.Fatalf("R5 覆盖参数未落到卡: timing=%+v budget=%+v", card.Timing, card.Budget)
	}
	if len(rep.Warnings()) != 23 {
		t.Fatalf("W 清单=23（got %d）", len(rep.Warnings()))
	}

	// 源值 + 覆盖参数并存：覆盖参数优先并出 I（不静默覆盖）
	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "    tempo_bpm: null", "    tempo_bpm: 100")
	})
	res2, rep2 := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if card2.Timing.BPMDefault != 80 {
		t.Fatalf("R5 覆盖参数应优先于源值 100（got %v）", card2.Timing.BPMDefault)
	}
	found := false
	for _, d := range rep2.Infos() {
		if d.Path == "workspace.timing.tempo_bpm" && strings.Contains(d.Message, "优先") {
			found = true
		}
	}
	if !found {
		t.Fatalf("R5 覆盖参数并存时须出 I-NOTE 说明优先关系")
	}
}

func TestT20BudgetMapping(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		src = mutate(t, src, "  max_turns: null", "  max_turns: 30")
		src = mutate(t, src, "  max_llm_calls: null", "  max_llm_calls: 40")
		return mutate(t, src, "    tempo_bpm: null", "    tempo_bpm: 100")
	})
	input := p.input(t)
	input.Overrides = CompileOverrides{}
	res, _ := compileOK(t, input)
	card, _ := res.LevelCard()
	if card.Budget.MaxTurns != 30 || card.Budget.MaxLLMCalls != 40 || card.Budget.OnStopLoss != "preserve_and_pause" {
		t.Fatalf("T-20 budget 映射错: %+v", card.Budget)
	}
	if card.Timing.BPMDefault != 100 {
		t.Fatalf("T-8 源 tempo 直拷错: %v", card.Timing.BPMDefault)
	}
}

// ---- T-10/R6a：key_signature 设计层字段不进卡；30 调 derivation ----

func TestT10KeySignatureNotInCard(t *testing.T) {
	p := setupTestPackage(t)
	res, rep := compileOK(t, p.input(t))
	if bytes.Contains(res.CardYAML, []byte("key_signature")) {
		t.Fatalf("T-10：卡本体不得含 key_signature")
	}
	found := false
	for _, d := range rep.Infos() {
		if d.Path == "workspace.timing.key_signature" && d.Rule == RuleNote {
			found = true
		}
	}
	if !found {
		t.Fatalf("T-10：源校验须出 key_signature I-NOTE")
	}
}

func TestR6aAssetKeyDerivation(t *testing.T) {
	majors := []struct {
		token string
		tonic int
	}{
		{"F#", 6}, {"Bb", 10}, {"Cb", 11}, {"E", 4}, {"Db", 1},
	}
	for _, c := range majors {
		t.Run("major-"+c.token, func(t *testing.T) {
			p := setupTestPackage(t)
			p.rewriteDesign(t, func(src string) string {
				return replaceOnce(t, src, "key_signature: C #", "key_signature: "+c.token+" #")
			})
			res, _ := compileOK(t, p.input(t))
			card, _ := res.LevelCard()
			if len(card.Assets) == 0 || card.Assets[0].Key == nil {
				t.Fatalf("R6a 大调 %s：资产 key 缺失", c.token)
			}
			if card.Assets[0].Key.Tonic != c.tonic || card.Assets[0].Key.Mode != "major" {
				t.Fatalf("R6a 大调 %s：key=%+v（want tonic=%d major）", c.token, card.Assets[0].Key, c.tonic)
			}
		})
	}
	// 小调：R6a 词表合法，但卡 v0 判定 key 仅大调 → 不写 key + W-MAP（可见降级）
	t.Run("minor-visible-degradation", func(t *testing.T) {
		p := setupTestPackage(t)
		p.rewriteDesign(t, func(src string) string {
			return replaceOnce(t, src, "key_signature: C #", "key_signature: Am #")
		})
		res, rep := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if len(card.Assets) == 0 || card.Assets[0].Key != nil {
			t.Fatalf("R6a 小调：卡 v0 key 应留空（got %+v）", card.Assets[0].Key)
		}
		found := false
		for _, w := range rep.Warnings() {
			if w.Rule == RuleDefault && strings.Contains(w.Message, "R6a") {
				found = true
			}
		}
		if !found {
			t.Fatalf("R6a 小调不写 key 须出 W-MAP（可见降级）")
		}
	})
}

// ---- T-11：clip_origins/bar_span 推导（歧义→E-MAP）----

func TestT11ClipOriginsAmbiguity(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, fixtureRhythmLabLocator, `      locator: {clip_alias: rhythm-lab, time_signature: 2/4}
`)
	})
	rep := compileReport(t, p.input(t))
	assertDiag(t, rep, SeverityError, RuleMapping, "workspace.regions[1].locator.bars")

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		src = replaceOnce(t, src, fixtureRhythmLabLocator, `      locator: {scope: selected_reference_excerpt}
`)
		return mutate(t, src, fixtureThemeLocator, `      locator: {scope: generated_audio_versions}
`)
	})
	rep2 := compileReport(t, p2.input(t))
	assertDiag(t, rep2, SeverityError, RuleMapping, "workspace.regions")
}

// ---- T-12/R3：runtime_slots 映射与 initial_material preload ----

func TestT12RuntimeSlotsPreload(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, "initial_material: null, editable_by_user: false}", "initial_material: prototype-midi, editable_by_user: false}")
	})
	res, _ := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	slot := card.RuntimeSlots["reference-track"]
	if slot.Preload == nil || *slot.Preload != "prototype-midi" || slot.InitialState != "loaded" {
		t.Fatalf("T-12 initial_material→preload 映射错: %+v", slot)
	}

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, "initial_material: null, editable_by_user: false}", "initial_material: source-summary, editable_by_user: false}")
	})
	assertDiag(t, compileReport(t, p2.input(t)), SeverityError, RuleRef, "workspace.tracks[0].initial_material")
}

func TestR3TrackMappingManifest(t *testing.T) {
	p := setupTestPackage(t)
	res, _ := compileOK(t, p.input(t))
	want := []trackMappingRecord{
		{TrackID: "reference-track", TrackIndex: 1, Kind: "reference"},
		{TrackID: "melody-track", TrackIndex: 2, Kind: "workspace"},
		{TrackID: "rhythm-track", TrackIndex: 3, Kind: "blank_to_fill"},
		{TrackID: "theme-track", TrackIndex: 4, Kind: "workspace"},
		{TrackID: "result-track", TrackIndex: 5, Kind: "workspace"},
	}
	got := res.Manifest.Receipt.TrackMapping
	if len(got) != len(want) {
		t.Fatalf("R3 track_mapping=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("R3 track_mapping[%d]=%+v（want %+v）", i, got[i], want[i])
		}
	}
}

// ---- T-13：材料准入（catalog 状态 / MAT-FILE / 待备料清单）----

func TestT13MaterialAdmission(t *testing.T) {
	t.Run("catalog-unverified-excluded", func(t *testing.T) {
		p := setupTestPackage(t)
		p.writeCatalog(t, "transcribed_unverified", "generated_symbolic_checked")
		res, rep := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if len(card.Assets) != 1 || card.Assets[0].ID != "theme-reference-rhythm" {
			t.Fatalf("T-13 未达标材料应排除（assets=%d）", len(card.Assets))
		}
		if len(res.Manifest.Receipt.ExcludedMaterials) != 14 {
			t.Fatalf("T-13 待备料清单=%d（期望 14）", len(res.Manifest.Receipt.ExcludedMaterials))
		}
		diag := assertDiag(t, rep, SeverityWarning, RuleMatWarn, "materials[12].file")
		if !strings.Contains(diag.Message, "transcribed_unverified") {
			t.Fatalf("T-13 排除原因须含 catalog 状态: %s", diag.Message)
		}
	})
	t.Run("catalog-missing-excludes-all-playable", func(t *testing.T) {
		p := setupTestPackage(t)
		if err := os.Remove(filepath.Join(p.root, "assets", "catalog.yaml")); err != nil {
			t.Fatalf("remove catalog: %v", err)
		}
		res, _ := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if len(card.Assets) != 0 {
			t.Fatalf("T-13 catalog 缺失时不得准入资产（got %d）", len(card.Assets))
		}
		if len(res.Manifest.Receipt.ExcludedMaterials) != 15 {
			t.Fatalf("T-13 catalog 缺失待备料清单=%d（期望 15）", len(res.Manifest.Receipt.ExcludedMaterials))
		}
	})
	t.Run("mat-file-missing", func(t *testing.T) {
		p := setupTestPackage(t)
		if err := os.Remove(filepath.Join(p.root, filepath.FromSlash(testMIDIRelPrototype))); err != nil {
			t.Fatalf("remove midi: %v", err)
		}
		rep := compileReport(t, p.input(t))
		diag := assertDiag(t, rep, SeverityError, RuleMat, "materials[12].file")
		if !strings.Contains(diag.Message, "MAT-FILE") {
			t.Fatalf("T-13 MAT-FILE 诊断标记缺失: %s", diag.Message)
		}
	})
	t.Run("invalid-smf", func(t *testing.T) {
		p := setupTestPackage(t)
		writeTestPackageFile(t, p.root, testMIDIRelPrototype, "MThd-broken")
		rep := compileReport(t, p.input(t))
		assertDiag(t, rep, SeverityError, RuleMapping, "materials[12].file")
	})
	t.Run("empty-smf-notes", func(t *testing.T) {
		p := setupTestPackage(t)
		writeTestPackageFile(t, p.root, testMIDIRelPrototype, string(buildTestMIDI(t, nil)))
		rep := compileReport(t, p.input(t))
		diag := assertDiag(t, rep, SeverityError, RuleMapping, "materials[12].file")
		if !strings.Contains(diag.Message, "notes") {
			t.Fatalf("T-13 空 notes 诊断错: %s", diag.Message)
		}
	})
	t.Run("catalog-unregistered-file", func(t *testing.T) {
		p := setupTestPackage(t)
		writeTestPackageFile(t, p.root, "assets/catalog.yaml", `schema_version: vit.lesson_material_catalog.v0
items:
  - id: reference-sanshilipu-rhythm
    file: assets/midi/sanshilipu-first8_rhythm-only_GM37_80bpm.mid
    status: generated_symbolic_checked
`)
		res, _ := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if len(card.Assets) != 1 {
			t.Fatalf("T-13 catalog 未登记文件应排除（assets=%d）", len(card.Assets))
		}
	})
	t.Run("non-playable-files-not-read", func(t *testing.T) {
		p := setupTestPackage(t)
		// 文本/图像类材料文件不存在也不影响编译（只进材料库加载面，不进卡资产）
		if _, err := os.Stat(filepath.Join(p.root, "TEXTBOOK-NOTES.md")); !os.IsNotExist(err) {
			t.Fatalf("测试前提：非可播放材料文件不应存在")
		}
		compileOK(t, p.input(t))
	})
}

// ---- T-14：判定面三分支（白名单结构叶 / rubric→verbal_rubric / event 映射；W-CHK）----

func TestT14JudgementFaces(t *testing.T) {
	t.Run("empty-registry", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Judgements = JudgementRegistry{}
		rep := compileReport(t, input)
		if got := rep.RuleCount(RuleFlow); got != 8 {
			t.Fatalf("空登记表：8 节点应全 E-FLOW，got %d", got)
		}
		if got := rep.RuleCount(RuleCheckWarn); got != 15 {
			t.Fatalf("空登记表：W-CHK 应 15（5 deterministic objective + 8 complete_when + 2 user_judgment objective），got %d", got)
		}
	})
	t.Run("structure-predicate-whitelist", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Judgements.Deterministic["c-rhythm-preview"] = StructurePredicate{Structure: "bogus"}
		assertDiag(t, compileReport(t, input), SeverityError, RuleMapping, "checks")
	})
	t.Run("structure-clip-unknown-slot", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Judgements.Deterministic["c-rhythm-preview"] = StructurePredicate{Structure: StructureClipExists, Clip: "no-such-track"}
		assertDiag(t, compileReport(t, input), SeverityError, RuleMapping, "checks")
	})
	t.Run("event-closed-set", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Judgements.UserJudgment["c-intent"] = "explode"
		assertDiag(t, compileReport(t, input), SeverityError, RuleMapping, "checks")
	})
	t.Run("registry-drift-warning", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Judgements.Deterministic["ghost-check"] = StructurePredicate{Structure: StructureTempoSet}
		_, rep := compileOK(t, input)
		diag := assertDiag(t, rep, SeverityWarning, RuleDefault, "checks")
		if !strings.Contains(diag.Message, "ghost-check") {
			t.Fatalf("登记表漂移 W-MAP 须点名未知 check: %s", diag.Message)
		}
	})
	t.Run("no-text-inference", func(t *testing.T) {
		// 修改 criterion 文本不得改变判定面（禁止从文本生成判定）
		p := setupTestPackage(t)
		p.rewriteDesign(t, func(src string) string {
			return replaceOnce(t, src, "criterion: 2/4共8拍节奏（四小节暂定）在旋律轨clip有效保存，占位音高可经实际音源试听，Tap所选单位/BPM可回读", "criterion: 完全无关的文本（不含任何结构谓词提示）")
		})
		res, _ := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if got := describeEduDoneWhen(card.Nodes[2].DoneWhen); got != "structure:clip_exists:melody-track" {
			t.Fatalf("T-14 文本变异后 done_when 改变（存在文本推断）: %s", got)
		}
	})
	t.Run("rubric-not-judgement", func(t *testing.T) {
		p := setupTestPackage(t)
		p.rewriteDesign(t, func(src string) string {
			return replaceOnce(t, src, "      complete_when: [c-intent]", "      complete_when: [c-listening-explanation]")
		})
		rep := compileReport(t, p.input(t))
		assertDiag(t, rep, SeverityWarning, RuleCheckWarn, "checks")
		assertDiag(t, rep, SeverityError, RuleFlow, "flow.nodes[0].complete_when")
	})
	t.Run("partial-compilable", func(t *testing.T) {
		p := setupTestPackage(t)
		p.rewriteDesign(t, func(src string) string {
			return replaceOnce(t, src, "      complete_when: [c-rhythm-preview]", "      complete_when: [c-rhythm-preview, c-listening-explanation]")
		})
		res, rep := compileOK(t, p.input(t))
		card, _ := res.LevelCard()
		if got := describeEduDoneWhen(card.Nodes[2].DoneWhen); got != "structure:clip_exists:melody-track" {
			t.Fatalf("部分可编译：应保留唯一可编译叶，got %s", got)
		}
		if got := rep.RuleCount(RuleCheckWarn); got != 6 {
			t.Fatalf("部分可编译：W-CHK 应 6（5 objective + 1 不可编译项），got %d", got)
		}
	})
}

// ---- T-15/R4：objectives 映射与多证据拆分 ----

func TestT15ObjectivesR4Split(t *testing.T) {
	p := setupTestPackage(t)
	res, rep := compileOK(t, p.input(t))
	card, _ := res.LevelCard()

	// rubric → verbal_rubric（g1 单证据直映射）
	if card.Objectives[0].ID != "g1-understand" || card.Objectives[0].Check.Kind != "verbal_rubric" || len(card.Objectives[0].Check.Rubric) == 0 {
		t.Fatalf("T-15 单证据映射错: %+v", card.Objectives[0])
	}
	// user_judgment 已登记 → event 叶（g3#3 = c-humming）
	foundEvent := false
	for _, o := range card.Objectives {
		if o.ID == "g3-develop#3" {
			foundEvent = o.Check.Event == "user_feedback_given"
		}
	}
	if !foundEvent {
		t.Fatalf("T-15 user_judgment→event 映射错")
	}
	// deterministic 证据在卡 objective 面无结构叶 → verbal_rubric + W-CHK（g2#1）
	for _, o := range card.Objectives {
		if o.ID == "g2-contours#1" && o.Check.Kind != "verbal_rubric" {
			t.Fatalf("T-15 deterministic objective 降级错: %+v", o.Check)
		}
	}
	// R4 拆分 W 诊断列明拆分 id
	foundSplit := false
	for _, w := range rep.Warnings() {
		if w.Rule == RuleDefault && strings.Contains(w.Message, "g2-contours") && strings.Contains(w.Message, "g2-contours#1") {
			foundSplit = true
		}
	}
	if !foundSplit {
		t.Fatalf("T-15/R4 拆分 W-MAP 未列明拆分 id")
	}
	// 卡目标 id 唯一
	seen := map[string]bool{}
	for _, o := range card.Objectives {
		if seen[o.ID] {
			t.Fatalf("T-15 卡目标 id 重复: %s", o.ID)
		}
		seen[o.ID] = true
	}

	// 无 evidence_checks → 不进卡 + W（n1 teaching.objectives 不得悬空）
	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, "    evidence_checks: [c-listening-explanation]", "    evidence_checks: []")
	})
	res2, rep2 := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if len(card2.Objectives) != 10 {
		t.Fatalf("T-15 无证据钩目标应不进卡（objectives=%d）", len(card2.Objectives))
	}
	assertDiag(t, rep2, SeverityWarning, RuleDefault, "objectives[0].evidence_checks")
	if len(card2.Nodes[1].Teaching.Objectives) != 0 {
		t.Fatalf("T-15 目标丢弃后 teaching.objectives 不得悬空: %v", card2.Nodes[1].Teaching.Objectives)
	}
}

// ---- T-16：节点映射（goal/model_does/hint_ladder/teaching/done_when 组合子）----

func TestT16NodesMapping(t *testing.T) {
	p := setupTestPackage(t)
	res, _ := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	n1 := card.Nodes[1]
	if n1.Goal == "" || len(n1.ModelDoes) == 0 || len(n1.HintLadder) == 0 {
		t.Fatalf("T-16 节点映射缺失: %+v", n1)
	}
	if !strings.Contains(n1.Teaching.Note, "；") {
		t.Fatalf("T-16 teaching_points join 错: %q", n1.Teaching.Note)
	}
	// 多条可编译 complete_when → all 组合子
	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, "      complete_when: [c-rhythm-preview]", "      complete_when: [c-rhythm-preview, c-q1-structure]")
	})
	res2, _ := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if got := describeEduDoneWhen(card2.Nodes[2].DoneWhen); got != "all" {
		t.Fatalf("T-16 多完成条件应产出 all 组合子，got %s", got)
	}
}

// ---- T-17：write_boundary 仅由 CAP-BRIDGE 闭包表产生 ----

func TestT17CapabilityBridge(t *testing.T) {
	t.Run("empty-bridge", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		input.Capabilities = CapabilityBridge{}
		rep := compileReport(t, input)
		if got := rep.RuleCount(RuleCap); got != 8 {
			t.Fatalf("T-17 空闭包表：8 个 required 能力应全 E-CAP，got %d", got)
		}
	})
	t.Run("missing-entry", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		closures := input.Capabilities.Closures
		delete(closures, "tempo-and-meter")
		assertDiag(t, compileReport(t, input), SeverityError, RuleCap, "capability_requests[3]")
	})
	t.Run("registered-but-empty-closures", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		closures := map[string][]string{}
		for id := range input.Capabilities.Closures {
			closures[id] = nil
		}
		input.Capabilities = CapabilityBridge{Closures: closures}
		rep := compileReport(t, input)
		if rep.RuleCount(RuleCap) != 0 {
			t.Fatalf("T-17 已登记空闭包不应 E-CAP（got %d）", rep.RuleCount(RuleCap))
		}
		assertDiag(t, rep, SeverityError, RuleMapping, "write_boundary.allow")
	})
}

// ---- T-18：guidance 四字段 join + pullback 缺省注入 ----

func TestT18GuidanceDefaults(t *testing.T) {
	p := setupTestPackage(t)
	res, rep := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	wantSoft := strings.Join([]string{
		"以学生作品贯穿的共同创作；短讲解和即时实践交替，三题逐步完成",
		"先问意图和具体位置，再局部对比/示范；逐渐减少帮助并记录实际帮助",
		"节点内允许提问/重听/重试；返回修改产生新版本，不清除前面的证据",
		"主旋律与节奏决定由学生作出；模型负责材料、定位、解释与已约定辅助编曲",
	}, "；")
	if card.Guidance.Soft != wantSoft {
		t.Fatalf("T-18 soft join 错: %q", card.Guidance.Soft)
	}
	if card.Guidance.PullbackAfterTurns != 6 || card.Guidance.Avoid == nil || len(card.Guidance.Avoid) != 0 {
		t.Fatalf("T-18 guidance 缺省错: %+v", card.Guidance)
	}
	assertDiag(t, rep, SeverityWarning, RuleDefault, "guidance.pullback_after_turns")

	// 缺 guidance 段：不 panic、soft 空、pullback 仍注入 6 + W
	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string { return mutate(t, src, fixtureGuidanceBlock, "") })
	res2, rep2 := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if card2.Guidance.Soft != "" || card2.Guidance.PullbackAfterTurns != 6 {
		t.Fatalf("T-18 guidance 缺失处理错: %+v", card2.Guidance)
	}
	assertDiag(t, rep2, SeverityWarning, RuleDefault, "guidance.pullback_after_turns")
}

// ---- T-19：submission/completion/evidence_recording → finale+settle 模板 ----

func TestT19FinaleSettleTemplates(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		src = mutate(t, src, fixtureSubmissionBlock, "")
		return mutate(t, src, fixtureCompletionBlock, "")
	})
	res, rep := compileOK(t, p.input(t))
	card, _ := res.LevelCard()
	fin := card.Finale
	if fin.Settle.ObjectivesCheck != "" {
		t.Fatalf("T-19 completion 缺 → objectives_check 应为空（got %q）", fin.Settle.ObjectivesCheck)
	}
	if !fin.ClosureCheck.SoftSignals.UserClosingLanguage.Required {
		t.Fatalf("T-19 completion 缺省 user_confirm_closure=true → required 须 true")
	}
	if !fin.Settle.TrajectoryArchive || len(fin.Settle.MemoryWrites) != 2 {
		t.Fatalf("T-19 submission 缺省模板错: %+v", fin.Settle)
	}
	for _, path := range []string{"submission", "completion"} {
		assertDiag(t, rep, SeverityInfo, RuleNote, path)
	}
	// user_confirm_closure=false → required=false
	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "  user_confirm_closure: true", "  user_confirm_closure: false")
	})
	res2, _ := compileOK(t, p2.input(t))
	card2, _ := res2.LevelCard()
	if card2.Finale.ClosureCheck.SoftSignals.UserClosingLanguage.Required {
		t.Fatalf("T-19 user_confirm_closure=false → required 须 false")
	}
}

// ---- T-21：pending_questions 不进卡，进 manifest 回执 ----

func TestT21PendingQuestionsReceipt(t *testing.T) {
	p := setupTestPackage(t)
	res, _ := compileOK(t, p.input(t))
	if bytes.Contains(res.CardYAML, []byte("pending_questions")) {
		t.Fatalf("T-21：pending_questions 不得进卡")
	}
	if len(res.Manifest.Receipt.PendingQuestions) != 9 {
		t.Fatalf("T-21：manifest 回执须罗列 pending_questions（got %d）", len(res.Manifest.Receipt.PendingQuestions))
	}
	if res.Manifest.Receipt.PendingQuestions[0] != "课程按任务量与证据推进，展演5分钟录播/截图/产物；不等待课时时长确认" {
		t.Fatalf("T-21：pending_questions 回执内容错: %q", res.Manifest.Receipt.PendingQuestions[0])
	}
}

// ---- R6b：music_region 单/列表/null 归一（编译接受三态）----

func TestR6bMusicRegionNormalized(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "music_region: 第一二句", "music_region: [第一二句, 第三四句]")
	})
	compileOK(t, p.input(t))

	p2 := setupTestPackage(t)
	p2.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "music_region: 第一二句", "music_region: null")
	})
	compileOK(t, p2.input(t))
}

// ---- R6c：材料核验状态不提升进 schema（KnownFields fail-closed）----

func TestR6cCatalogStatusNotInSchema(t *testing.T) {
	p := setupTestPackage(t)
	p.rewriteDesign(t, func(src string) string {
		return replaceOnce(t, src, "    file: TEXTBOOK-NOTES.md", `    file: TEXTBOOK-NOTES.md
    status: checked`)
	})
	rep := compileReport(t, p.input(t))
	diag := assertDiag(t, rep, SeverityError, RuleSchema, "")
	if !strings.Contains(diag.Message, "status") {
		t.Fatalf("R6c：status 字段应被 KnownFields 拒绝（got: %s）", diag.Message)
	}
}

// ---- manifest 格式（§6.5：四元组键 + W 清单 + 映射表版本 + 回执）----

func compileAndWrite(t *testing.T, input CompileInput) (*CompileResult, string, string) {
	t.Helper()
	res := mustCompile(t, input)
	cardPath, manifestPath, err := WriteCompiled(res, input.OutputDir)
	if err != nil {
		t.Fatalf("WriteCompiled: %v", err)
	}
	return res, cardPath, manifestPath
}

func TestManifestFormat(t *testing.T) {
	p := setupTestPackage(t)
	input := p.input(t)
	res, rep := compileOK(t, input)
	_, cardPath, manifestPath := compileAndWrite(t, input)

	cardBytes, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("读编译卡: %v", err)
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("读 manifest: %v", err)
	}
	var m CompileManifest
	if err := json.Unmarshal(manifestRaw, &m); err != nil {
		t.Fatalf("manifest 非合法 JSON: %v", err)
	}
	if m.SchemaVersion != ManifestSchemaVersion || m.MappingTableVersion != MappingTableVersion {
		t.Fatalf("manifest schema/映射表版本错: %q/%q", m.SchemaVersion, m.MappingTableVersion)
	}
	key := m.Key
	if key.DesignSourceSHA256 == "" || key.CompilerVersion == "" || key.AssemblyInputHash == "" || key.CardSchemaVersion == "" {
		t.Fatalf("四元组键不完整: %+v", key)
	}
	if key.CompilerVersion != CompilerVersion || key.CardSchemaVersion != edu.LevelCardSchemaVersion {
		t.Fatalf("四元组键版本错: %+v", key)
	}
	if m.CardFile != res.Design.ID+".compiled.level.yaml" {
		t.Fatalf("manifest.card_file=%q", m.CardFile)
	}
	if m.CardSHA256 != sha256Hex(cardBytes) {
		t.Fatalf("manifest.card_sha256 与卡文件不符")
	}
	if len(m.Warnings) != 23 || len(rep.Warnings()) != 23 {
		t.Fatalf("W 清单=23（manifest=%d report=%d）", len(m.Warnings), len(rep.Warnings()))
	}
	if len(m.Receipt.AdmittedAssets) != 2 || len(m.Receipt.ExcludedMaterials) != 13 ||
		len(m.Receipt.TrackMapping) != 5 || len(m.Receipt.PendingQuestions) != 9 {
		t.Fatalf("manifest 回执计数错: admitted=%d excluded=%d tracks=%d pending=%d",
			len(m.Receipt.AdmittedAssets), len(m.Receipt.ExcludedMaterials), len(m.Receipt.TrackMapping), len(m.Receipt.PendingQuestions))
	}
	// 文件按 MarshalIndent 落盘：重排队列化字节一致（确定性）
	remarshaled, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("manifest 重序列化: %v", err)
	}
	remarshaled = append(remarshaled, []byte(string(rune(10)))...)
	if !bytes.Equal(remarshaled, manifestRaw) {
		t.Fatalf("manifest 序列化不确定/被额外改写")
	}
}

// ---- 缓存键变化 → 重编译判定；手改卡/manifest 损坏 → 拒绝；缺文件 → 不可用 ----

func TestCacheKeyAndVerifyCompiled(t *testing.T) {
	t.Run("valid-hit", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		res, cardPath, manifestPath := compileAndWrite(t, input)
		m, err := VerifyCompiled(cardPath, manifestPath, input)
		if err != nil {
			t.Fatalf("有效缓存应命中: %v", err)
		}
		if !m.Key.Equal(res.Manifest.Key) {
			t.Fatalf("VerifyCompiled 返回的键与编译键不一致")
		}
		current, err := BuildCacheKey(input)
		if err != nil {
			t.Fatalf("BuildCacheKey: %v", err)
		}
		if !current.Equal(res.Manifest.Key) {
			t.Fatalf("BuildCacheKey 与 manifest 键不一致")
		}
	})
	t.Run("design-change-stale", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		p.rewriteDesign(t, func(src string) string {
			return mutate(t, src, "title: 把旋律的起伏写成自己的作品", "title: 修改后的标题")
		})
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCacheStale) {
			t.Fatalf("设计源字节变化须判缓存失效（got %v）", err)
		}
	})
	t.Run("override-change-stale", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		changed := input
		changed.Overrides.MaxTurns = intPtr(61)
		if _, err := VerifyCompiled(cardPath, manifestPath, changed); !errors.Is(err, ErrCacheStale) {
			t.Fatalf("覆盖参数变化须判缓存失效（got %v）", err)
		}
	})
	t.Run("catalog-change-stale", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		p.writeCatalog(t, "transcribed_unverified", "generated_symbolic_checked")
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCacheStale) {
			t.Fatalf("catalog 变化须判缓存失效（got %v）", err)
		}
	})
	t.Run("admitted-file-change-stale", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		writeTestPackageFile(t, p.root, testMIDIRelPrototype, string(buildTestMIDI(t, [][3]int{{67, 0, 240}})))
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCacheStale) {
			t.Fatalf("准入文件字节变化须判缓存失效（got %v）", err)
		}
	})
	t.Run("non-admitted-file-change-keeps-hit", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		// 非准入材料（text 类）不在装配输入清单哈希内：内容变化不影响缓存命中
		writeTestPackageFile(t, p.root, "TEXTBOOK-NOTES.md", "未进卡材料内容变化")
		if _, err := VerifyCompiled(cardPath, manifestPath, input); err != nil {
			t.Fatalf("非准入材料变化不应使准入文件集合哈希失效: %v", err)
		}
	})
	t.Run("hand-edited-card-tampered", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		cardBytes, _ := os.ReadFile(cardPath)
		if err := os.WriteFile(cardPath, append(cardBytes, []byte("# hand edit")...), 0o644); err != nil {
			t.Fatalf("手改卡: %v", err)
		}
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCompiledTampered) {
			t.Fatalf("手改 compiled 卡须被 manifest 拒绝（got %v）", err)
		}
	})
	t.Run("manifest-broken", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		if err := os.WriteFile(manifestPath, []byte("{"), 0o644); err != nil {
			t.Fatalf("写坏 manifest: %v", err)
		}
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCompiledTampered) {
			t.Fatalf("损坏 manifest 须拒绝（got %v）", err)
		}
	})
	t.Run("mapping-table-version-stale", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		raw, _ := os.ReadFile(manifestPath)
		var m CompileManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("manifest 解码: %v", err)
		}
		m.MappingTableVersion = "vit.legame_design.v0.mapping.stale"
		patched, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(manifestPath, patched, 0o644); err != nil {
			t.Fatalf("写 patched manifest: %v", err)
		}
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCacheStale) {
			t.Fatalf("映射表版本变化须判缓存失效（got %v）", err)
		}
	})
	t.Run("manifest-missing", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		if err := os.Remove(manifestPath); err != nil {
			t.Fatalf("remove manifest: %v", err)
		}
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCompiledMissing) {
			t.Fatalf("manifest 缺失须判缓存缺失（got %v）", err)
		}
	})
	t.Run("card-missing", func(t *testing.T) {
		p := setupTestPackage(t)
		input := p.input(t)
		_, cardPath, manifestPath := compileAndWrite(t, input)
		if err := os.Remove(cardPath); err != nil {
			t.Fatalf("remove card: %v", err)
		}
		if _, err := VerifyCompiled(cardPath, manifestPath, input); !errors.Is(err, ErrCompiledMissing) {
			t.Fatalf("卡文件缺失须判缓存缺失（got %v）", err)
		}
	})
}

// ---- RefreshCompiled：缓存命中→复用；失效→重编译；重编译失败→旧产物保留但阻断使用 ----

func TestRefreshCompiled(t *testing.T) {
	p := setupTestPackage(t)
	input := p.input(t)

	res1, rep1, cached1 := RefreshCompiled(input)
	if cached1 || res1 == nil || rep1.HasErrors() || rep1 == nil {
		t.Fatalf("首次 RefreshCompiled 应重编译（cached=%v）", cached1)
	}
	cardPath, manifestPath := CompiledPaths(input.OutputDir, res1.Design.ID)
	cardBytes1, err := os.ReadFile(cardPath)
	if err != nil {
		t.Fatalf("读卡: %v", err)
	}

	res2, rep2, cached2 := RefreshCompiled(input)
	if !cached2 || res2 == nil || rep2 == nil || rep2.HasErrors() {
		t.Fatalf("第二次 RefreshCompiled 应命中缓存（cached=%v）", cached2)
	}
	if !bytes.Equal(cardBytes1, res2.CardYAML) {
		t.Fatalf("缓存命中返回卡字节与磁盘不一致")
	}

	// 设计源变化 → 缓存失效 → 重编译，磁盘卡更新
	p.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "title: 把旋律的起伏写成自己的作品", "title: 修改后的标题")
	})
	res3, rep3, cached3 := RefreshCompiled(input)
	if cached3 || res3 == nil || rep3.HasErrors() {
		t.Fatalf("设计源变化后应重编译（cached=%v）", cached3)
	}
	card3, err := res3.LevelCard()
	if err != nil || card3.Title != "修改后的标题" {
		t.Fatalf("重编译卡未更新 title: %v %+v", err, card3)
	}
	if _, err := VerifyCompiled(cardPath, manifestPath, input); err != nil {
		t.Fatalf("重编译后缓存应可校验: %v", err)
	}

	// 手改卡 → 缓存完整性失效 → 重编译修复（覆盖手改卡）
	tamperedCard := append(append([]byte{}, res3.CardYAML...), []byte("# edited")...)
	if err := os.WriteFile(cardPath, tamperedCard, 0o644); err != nil {
		t.Fatalf("手改卡: %v", err)
	}
	_, rep4, cached4 := RefreshCompiled(input)
	if cached4 || rep4.HasErrors() {
		t.Fatalf("手改卡后应重编译修复（cached=%v）", cached4)
	}
	if _, err := VerifyCompiled(cardPath, manifestPath, input); err != nil {
		t.Fatalf("重编译修复后缓存应可校验: %v", err)
	}

	// 失效 + 重编译失败：旧卡/旧 manifest 保留原样，但校验失效=阻断使用（fail-closed）
	p.rewriteDesign(t, func(src string) string {
		return mutate(t, src, "title: 把旋律的起伏写成自己的作品", "title: 第二次修改标题")
	})
	if err := os.Remove(filepath.Join(p.root, filepath.FromSlash(testMIDIRelPrototype))); err != nil {
		t.Fatalf("remove admitted midi: %v", err)
	}
	cardBefore, _ := os.ReadFile(cardPath)
	manifestBefore, _ := os.ReadFile(manifestPath)
	resFail, repFail, cachedFail := RefreshCompiled(input)
	if cachedFail || resFail != nil || repFail == nil || !repFail.HasErrors() {
		t.Fatalf("重编译失败应返回 E 且不返回结果（cached=%v res=%v）", cachedFail, resFail != nil)
	}
	assertDiag(t, repFail, SeverityError, RuleMat, "materials[12].file")
	cardAfter, _ := os.ReadFile(cardPath)
	manifestAfter, _ := os.ReadFile(manifestPath)
	if !bytes.Equal(cardBefore, cardAfter) || !bytes.Equal(manifestBefore, manifestAfter) {
		t.Fatalf("重编译失败不得改写既有产物（fail-closed 保留旧缓存）")
	}
	if _, err := VerifyCompiled(cardPath, manifestPath, input); err == nil {
		t.Fatalf("失效且重编译失败后，旧缓存必须被阻断使用")
	}

	// 设计源非法 → 缓存校验 fail-closed、不得降级使用旧卡
	p2 := setupTestPackage(t)
	input2 := p2.input(t)
	if _, _, cached := RefreshCompiled(input2); cached {
		t.Fatalf("首次应重编译")
	}
	card2Path, manifest2Path := CompiledPaths(input2.OutputDir, "music-creation-unit2-demo")
	card2Before, _ := os.ReadFile(card2Path)
	p2.rewriteDesign(t, func(src string) string {
		return mutate(t, src, fixtureObjectivesBlock, "")
	})
	resBad, repBad, cachedBad := RefreshCompiled(input2)
	if cachedBad || resBad != nil || repBad == nil || !repBad.HasErrors() {
		t.Fatalf("设计源非法时 RefreshCompiled 须 fail-closed（cached=%v）", cachedBad)
	}
	card2After, _ := os.ReadFile(card2Path)
	if !bytes.Equal(card2Before, card2After) {
		t.Fatalf("设计源非法不得改写既有产物")
	}
	if _, err := VerifyCompiled(card2Path, manifest2Path, input2); err == nil {
		t.Fatalf("设计源非法后旧缓存必须被阻断使用")
	}
}

// ---- MIDI 解析（T-13 notes 表输入；fail-closed）----

func buildRawSMF(t *testing.T, trackBody []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteString("MThd")
	out.Write([]byte{0, 0, 0, 6})
	out.Write([]byte{0, 0, 0, 1, 0x01, 0xE0})
	out.WriteString("MTrk")
	length := len(trackBody)
	out.Write([]byte{byte(length >> 24), byte(length >> 16), byte(length >> 8), byte(length)})
	out.Write(trackBody)
	return out.Bytes()
}

func TestMIDIParser(t *testing.T) {
	notes, err := parseSMFNotes(buildTestMIDI(t, [][3]int{{60, 0, 480}, {64, 240, 240}}))
	if err != nil {
		t.Fatalf("合法 SMF 解析失败: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("notes 条数=%d", len(notes))
	}
	// 排序后：60@0/1beat，64@0.5/0.5beat
	if notes[0].Pitch != 60 || notes[0].Start != 0 || notes[0].Length != 1 {
		t.Fatalf("notes[0]=%+v", notes[0])
	}
	if notes[1].Pitch != 64 || notes[1].Start != 0.5 || notes[1].Length != 0.5 {
		t.Fatalf("notes[1]=%+v", notes[1])
	}

	if _, err := parseSMFNotes([]byte("junk")); err == nil {
		t.Fatalf("非 SMF 应报错")
	}
	// SMPTE division 不支持
	smpte := buildTestMIDI(t, [][3]int{{60, 0, 480}})
	smpte[12] = 0x80
	smpte[13] = 0x00
	if _, err := parseSMFNotes(smpte); err == nil {
		t.Fatalf("SMPTE division 应 fail-closed")
	}
	// 未闭合音符
	unclosed := buildRawSMF(t, []byte{0x00, 0x90, 60, 100, 0x00, 0xFF, 0x2F, 0x00})
	if _, err := parseSMFNotes(unclosed); err == nil || !strings.Contains(err.Error(), "未闭合") {
		t.Fatalf("未闭合音符应报错（got %v）", err)
	}
	// note_off 无对应 note_on
	orphan := buildRawSMF(t, []byte{0x00, 0x80, 60, 0, 0x00, 0xFF, 0x2F, 0x00})
	if _, err := parseSMFNotes(orphan); err == nil {
		t.Fatalf("孤儿 note_off 应报错")
	}
}

// ---- 编译确定性（卡字节 + manifest 字节）----

func TestCompileDeterministic(t *testing.T) {
	p := setupTestPackage(t)
	input := p.input(t)
	res1 := mustCompile(t, input)
	res2 := mustCompile(t, input)
	if !bytes.Equal(res1.CardYAML, res2.CardYAML) {
		t.Fatalf("两次编译卡字节不同（不确定）")
	}
	manifest1, _ := json.Marshal(res1.Manifest)
	manifest2, _ := json.Marshal(res2.Manifest)
	if !bytes.Equal(manifest1, manifest2) {
		t.Fatalf("两次编译 manifest 不同（不确定）")
	}
	// 重复写盘：卡哈希稳定
	_, cardPath1, _ := compileAndWrite(t, input)
	bytes1, _ := os.ReadFile(cardPath1)
	_, cardPath2, _ := compileAndWrite(t, input)
	bytes2, _ := os.ReadFile(cardPath2)
	if !bytes.Equal(bytes1, bytes2) {
		t.Fatalf("重复写盘卡字节不同")
	}
}

const fixtureObjectivesBlock = `objectives:
  - id: g1-understand
    kind: learning
    description: 能定位音高进行与节奏差别，并用音乐证据说明自己的听感
    evidence_checks: [c-listening-explanation]
  - id: g2-contours
    kind: learning
    description: 四种旋律线各写两小节，能解释自己有意安排的走向
    evidence_checks: [c-q1-structure, c-q1-explanation]
  - id: g3-develop
    kind: learning
    description: 用至少一种原题示例手法发展喜欢的片段，唱或哼出来并说明保留和变化
    evidence_checks: [c-q2-structure, c-q2-explanation, c-humming]
  - id: g4-theme
    kind: learning
    description: 参照《三十里铺》前两句节奏，运用旋律线写出八小节自己的主题
    evidence_checks: [c-q3-structure, c-q3-reference-and-expression]
  - id: task-song
    kind: music_task
    description: 以选定学生主题生成纯器乐编曲，改变风格尝试新版本并比较
    evidence_checks: [c-generation-record, c-generated-pair, c-version-choice]
`

// ---- 产物落盘后可经现行 LoadLevelCard 加载（资产文件+sha256 对账一并过门）----

func TestCompiledProductLoadsWithCurrentLoader(t *testing.T) {
	p := setupTestPackage(t)
	input := p.input(t)
	// 加载器约定：卡所在目录名 == 卡 id（LoadLevelCard 收口）；输出目录按包布局命名。
	input.OutputDir = filepath.Join(t.TempDir(), "music-creation-unit2-demo")
	res := mustCompile(t, input)
	cardPath, _, err := WriteCompiled(res, input.OutputDir)
	if err != nil {
		t.Fatalf("WriteCompiled: %v", err)
	}
	card, err := edu.LoadLevelCard(cardPath)
	if err != nil {
		t.Fatalf("编译产物未过现行 LoadLevelCard（资产/哈希/目录约定）: %v", err)
	}
	if len(card.Assets) != 2 {
		t.Fatalf("加载后资产数=%d", len(card.Assets))
	}
	// 资产已复制进包资产区，且与卡内 sha256 一致
	for i := range card.Assets {
		a := &card.Assets[i]
		data, err := os.ReadFile(filepath.Join(input.OutputDir, filepath.FromSlash(a.File)))
		if err != nil {
			t.Fatalf("产物包缺资产文件 %s: %v", a.File, err)
		}
		if sha256Hex(data) != a.SHA256 {
			t.Fatalf("产物包资产 sha256 与卡不符: %s", a.File)
		}
	}
}
