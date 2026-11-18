package legamedesign

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// 测试夹具：最小合法设计源按段分片，便于构造缺段/空段/逐字段变异负例。
// `10 纪律：测试运行时副本一律写 t.TempDir()，不改 coord/lesson-designs 原件。
// ============================================================================

const (
	yamlHead = `schema_version: vit.legame_design.v0
id: demo-unit
revision: 1
title: 测试乐局
`

	yamlBrief = `brief:
  audience: 高中生
  music_task: 写两小节旋律
  final_deliverable: 两小节谱稿
  prerequisites: [基本时值]
  estimated_minutes: null
`

	yamlObjectives = `objectives:
  - id: g1
    kind: learning
    description: 能听出音高差别
    evidence_checks: [c1]
`

	yamlMaterials = `materials:
  - id: m1
    kind: text
    file: notes.md
    purpose: 教材页
    source: 用户提供
    locator: {pdf_pages: [1, 2], printed_pages: [1, 2], music_region: 第一句}
`

	yamlWorkspace = `workspace:
  timing:
    time_signature: 2/4
    key_signature: C
    tempo_bpm: 80
    pickup_beats: 0
  tracks:
    - {id: t1, name: 旋律, role: user_work, initial_material: m1, editable_by_user: true}
  regions:
    - id: r1
      track: t1
      locator: {clip_alias: clip-1, time_signature: 2/4, bars: 4}
      purpose: 学生片段
`

	yamlWorkspaceNoRegions = `workspace:
  timing:
    time_signature: 2/4
    key_signature: C
    tempo_bpm: 80
    pickup_beats: 0
  tracks:
    - {id: t1, name: 旋律, role: user_work, initial_material: m1, editable_by_user: true}
  regions: []
`

	yamlCapabilities = `capability_requests:
  - {id: cap1, request: 谱面编辑与试听, required: true, fallback: null}
`

	yamlChecks = `checks:
  - id: c1
    purpose: task_progress
    method: deterministic
    target: r1
    criterion: clip 存在
    evidence: 工程数据
    on_unavailable: pause_and_report
`

	yamlFlow = `flow:
  progression: ordered
  nodes:
    - id: n1
      title: 第一步
      objective_refs: [g1]
      task: 写两小节
      lead: human
      uses_materials: [m1]
      uses_regions: [r1]
      uses_capabilities: [cap1]
      human_decides: [写法]
      model_may_do: [示范]
      freedom: {fixed: [两小节], open: [音高]}
      guidance:
        introduction: 先写两小节
        teaching_points: [时值]
        hint_ladder: [示范]
        on_stuck: 缩短
      complete_when: [c1]
      collect_evidence: [c1]
      leaves_behind: 两小节谱稿
      transition: 继续
`

	yamlFlowEmptyNodes = `flow:
  progression: ordered
  nodes: []
`

	yamlTail = `evidence_recording:
  requested_granularity: [key_action, region, saved_revision]
  record_initiator_and_executor: true
  record_help_disclosed: true
  record_verified_progression: true
  pin_submission_log_range: true
  unknown_source: mark_unverified
  checkpoints: [node_entry, meaningful_revision, submission]
submission:
  freeze_project_and_evidence_snapshot: true
  teacher_review_required: true
  review_by_objective: true
  retain_previous_submissions: true
  on_return: preserve_history_and_continue
  reports: [assignment_report, key_context_summary]
  include_raw_evidence_index: true
  summary_claims_require_evidence_refs: true
completion:
  required_deliverables: [两小节谱稿]
  required_checks: [c1]
  learning_evidence_to_record: [c1]
  user_confirm_closure: true
  export: optional
budget:
  max_turns: 8
  max_llm_calls: 16
  on_limit: preserve_and_pause
pending_questions:
  - 待确认
`
)

// yamlRequired 是仅含 L1 必填段的最小文档（用于缺可选段 I-NOTE 测试）。
const yamlRequired = yamlHead + yamlBrief + yamlObjectives + yamlMaterials + yamlWorkspace + yamlCapabilities + yamlChecks + yamlFlow

// yamlMinimal 是含全部可选段的最小合法设计源（0 E 基线）。
const yamlMinimal = yamlRequired + yamlTail

// ============================================================================
// 测试辅助
// ============================================================================

func readFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "design.v1.yaml"))
	if err != nil {
		t.Fatalf("读取 testdata fixture 失败: %v", err)
	}
	return data
}

func readSourceBytes(t *testing.T, src string) []byte { return []byte(src) }

// parseOK 断言解析+校验 0 E 并返回结构。
func parseOK(t *testing.T, src string) (*Design, *Report) {
	t.Helper()
	d, r := Parse(readSourceBytes(t, src))
	if r == nil {
		t.Fatalf("Parse 返回 nil report")
	}
	if r.HasErrors() {
		t.Fatalf("期望 0 E，实际:\n%s", r.String())
	}
	if d == nil {
		t.Fatalf("0 E 但返回 nil Design")
	}
	return d, r
}

// reportOf 返回诊断（不要求 0 E）。
func reportOf(t *testing.T, src string, opts Options) *Report {
	t.Helper()
	_, r := ParseWithOptions(readSourceBytes(t, src), opts)
	if r == nil {
		t.Fatalf("ParseWithOptions 返回 nil report")
	}
	return r
}

// mutate 以唯一锚点替换构造负例（锚点出现次数须恰好 1，防止变异漂移）。
func mutate(t *testing.T, src, oldStr, newStr string) string {
	t.Helper()
	if n := strings.Count(src, oldStr); n != 1 {
		t.Fatalf("变异锚点 %q 出现 %d 次（须恰好 1 次）", oldStr, n)
	}
	return strings.Replace(src, oldStr, newStr, 1)
}

// assertDiag 断言存在指定 分级/规则号/路径 的诊断并返回它。
func assertDiag(t *testing.T, r *Report, sev Severity, rule, path string) Diagnostic {
	t.Helper()
	for _, d := range r.Diagnostics() {
		if d.Severity == sev && d.Rule == rule && d.Path == path {
			return d
		}
	}
	t.Fatalf("期望诊断 %s %s %q 未出现；实际:\n%s", sev, rule, path, r.String())
	return Diagnostic{}
}

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

// ============================================================================
// 正例：设计源 music-creation-unit2 副本 0 E（含 fixture 副本落 t.TempDir 纪律）
// ============================================================================

func TestFixtureDesignSourceZeroErrors(t *testing.T) {
	data := readFixture(t)

	// `10：fixture 副本落 t.TempDir() 后解析，不写 coord/lesson-designs 原件。
	tmpPath := filepath.Join(t.TempDir(), "design.v1.yaml")
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		t.Fatalf("fixture 副本落盘失败: %v", err)
	}
	copied, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("读取副本失败: %v", err)
	}

	d, r := Parse(copied)
	if d == nil {
		t.Fatalf("正例解析返回 nil Design（report:\n%s）", r.String())
	}
	if r.HasErrors() {
		t.Fatalf("正例（设计源 revision 3 副本）须 0 E，实际:\n%s", r.String())
	}
	errors, warnings, infos := r.Counts()
	t.Logf("fixture 诊断聚合: %d E / %d W / %d I", errors, warnings, infos)

	// 结构对账（revision 3 实例计数）
	if d.SchemaVersion != SchemaVersionLegameDesign {
		t.Fatalf("schema_version=%q", d.SchemaVersion)
	}
	if d.Revision == nil || *d.Revision != 3 {
		t.Fatalf("revision=%v（期望 3）", d.Revision)
	}
	if len(d.Objectives) != 5 || len(d.Materials) != 15 || len(d.Checks) != 15 {
		t.Fatalf("段计数不符: objectives=%d materials=%d checks=%d", len(d.Objectives), len(d.Materials), len(d.Checks))
	}
	if d.Flow == nil || len(d.Flow.Nodes) != 8 {
		t.Fatalf("flow.nodes 计数不符")
	}
	if d.Workspace == nil || len(d.Workspace.Regions) != 6 || len(d.Workspace.Tracks) != 5 {
		t.Fatalf("workspace 计数不符")
	}
	if len(d.CapabilityRequests) != 8 {
		t.Fatalf("capability_requests=%d（期望 8）", len(d.CapabilityRequests))
	}

	// W 清单：三份 file:null 未备料材料→W-MAT（不阻塞；须逐条进入回执）
	if got := r.RuleCount(RuleMatWarn); got != 3 {
		t.Fatalf("W-MAT=%d（期望 3：listening-xiaocao / listening-huainian / theme-reference-melody）；实际:\n%s", got, r.String())
	}
	for _, w := range r.Warnings() {
		if w.Rule != RuleMatWarn || w.Path == "" {
			t.Fatalf("W 诊断须带规则号与字段路径（got %s）", w.String())
		}
	}

	// I 提示：设计层字段/estimated_minutes=null/pending_questions 计数
	for _, p := range []string{
		"brief.estimated_minutes",
		"pending_questions",
		"workspace.timing.key_signature",
		"workspace.tracks[*].editable_by_user",
		"flow.nodes[*].transition",
	} {
		assertDiag(t, r, SeverityInfo, RuleNote, p)
	}

	// R6b：music_region 单值/列表/ null 三态归一
	findMaterial := func(id string) *Material {
		for i := range d.Materials {
			if d.Materials[i].ID == id {
				return &d.Materials[i]
			}
		}
		return nil
	}
	if m := findMaterial("source-practice3"); m == nil || len(m.Locator.MusicRegion) != 1 || m.Locator.MusicRegion[0] != "第一二句" {
		t.Fatalf("R6b 单值 music_region 归一失败: %+v", m)
	}
	if m := findMaterial("pitch-examples"); m == nil || len(m.Locator.MusicRegion) != 5 {
		t.Fatalf("R6b 列表 music_region 归一失败: %+v", m)
	}
	if m := findMaterial("source-summary"); m == nil || m.Locator.MusicRegion != nil {
		t.Fatalf("R6b null music_region 归一失败: %+v", m)
	}
}

// TestFixtureMatchesDesignSource 是副本漂移守卫：设计源原件可达时核对副本字节一致
// （只读；不存在时跳过，不阻塞纯包测试）。
func TestFixtureMatchesDesignSource(t *testing.T) {
	origPath := filepath.Join("..", "..", "..", "coord", "lesson-designs", "music-creation-unit2", "design.v1.yaml")
	orig, err := os.ReadFile(origPath)
	if err != nil {
		t.Skipf("设计源原件不可达（%v）：跳过 fixture 漂移核对", err)
	}
	fixture := readFixture(t)
	if !bytes.Equal(orig, fixture) {
		t.Fatalf("testdata/design.v1.yaml 与设计源原件不一致（原件已修订？请同步副本）")
	}
}

// ============================================================================
// schema_version / 判层（R1/R2）
// ============================================================================

func TestSchemaVersionGate(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		r := reportOf(t, `title: 无版本键
`, Options{})
		assertDiag(t, r, SeverityError, RuleSchema, "schema_version")
	})
	t.Run("unknown", func(t *testing.T) {
		r := reportOf(t, `schema_version: vit.legame_design.v9
title: 未知版本
`, Options{})
		assertDiag(t, r, SeverityError, RuleSchema, "schema_version")
	})
	t.Run("reserved-project-intent", func(t *testing.T) {
		src := `schema_version: vit.project_intent.v0
title: 我的混音练习
`
		d, r := Parse([]byte(src))
		if d == nil {
			t.Fatalf("预留版本仅登记：不应拒绝解码（report:\n%s）", r.String())
		}
		if r.HasErrors() {
			t.Fatalf("预留版本 vit.project_intent.v0 仅登记，不应按乐局报错:\n%s", r.String())
		}
		got := assertDiag(t, r, SeverityInfo, RuleNote, "schema_version")
		if !strings.Contains(got.Message, "仅登记") {
			t.Fatalf("预留版本诊断须说明仅登记，got: %s", got.Message)
		}
	})
}

// ============================================================================
// E-SCHEMA：未知字段 / 类型错 / 枚举外 / 格式与取值域
// ============================================================================

func TestKnownFieldsFailClosed(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"top-level", mutate(t, yamlMinimal, "id: demo-unit", "id: demo-unit\nmystery_field: 1")},
		{"objective-nested", mutate(t, yamlMinimal, "    description: 能听出音高差别", "    description: 能听出音高差别\n    mystery: x")},
		{"timing-nested", mutate(t, yamlMinimal, "    pickup_beats: 0", "    pickup_beats: 0\n    mystery: x")},
		{"locator-nested", mutate(t, yamlMinimal, "music_region: 第一句}", "music_region: 第一句, bogus: 1}")},
		{"node-nested", mutate(t, yamlMinimal, "      task: 写两小节", "      task: 写两小节\n      mystery: x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, c.src, Options{})
			d := assertDiag(t, r, SeverityError, RuleSchema, "")
			if !strings.Contains(d.Message, "KnownFields") {
				t.Fatalf("未知字段诊断须说明 fail-closed 口径，got: %s", d.Message)
			}
		})
	}
	t.Run("multiple-documents", func(t *testing.T) {
		src := yamlMinimal + "---\nschema_version: vit.legame_design.v0\n"
		r := reportOf(t, src, Options{})
		assertDiag(t, r, SeverityError, RuleSchema, "")
	})
	t.Run("type-error", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, "revision: 1", "revision: three"), Options{})
		d := assertDiag(t, r, SeverityError, RuleSchema, "")
		if !strings.Contains(d.Message, "cannot unmarshal") {
			t.Fatalf("类型错诊断须含解码原因，got: %s", d.Message)
		}
	})
}

func TestEnumClosedSets(t *testing.T) {
	cases := []struct {
		name   string
		anchor string
		repl   string
		path   string
	}{
		{"objective.kind", "kind: learning", "kind: dance", "objectives[0].kind"},
		{"material.kind", "kind: text", "kind: video", "materials[0].kind"},
		{"track.role", "role: user_work", "role: master", "workspace.tracks[0].role"},
		{"check.purpose", "purpose: task_progress", "purpose: grading", "checks[0].purpose"},
		{"check.method", "method: deterministic", "method: magic", "checks[0].method"},
		{"check.on_unavailable", "on_unavailable: pause_and_report", "on_unavailable: ignore", "checks[0].on_unavailable"},
		{"node.lead", "lead: human", "lead: robot", "flow.nodes[0].lead"},
		{"flow.progression", "progression: ordered", "progression: free", "flow.progression"},
		{"budget.on_limit", "on_limit: preserve_and_pause", "on_limit: stop", "budget.on_limit"},
		{"submission.on_return", "on_return: preserve_history_and_continue", "on_return: reset", "submission.on_return"},
		{"submission.reports", "reports: [assignment_report, key_context_summary]", "reports: [assignment_report, mystery_report]", "submission.reports[1]"},
		{"completion.export", "export: optional", "export: required", "completion.export"},
		{"evidence.unknown_source", "unknown_source: mark_unverified", "unknown_source: trust", "evidence_recording.unknown_source"},
		{"evidence.granularity", "requested_granularity: [key_action, region, saved_revision]", "requested_granularity: [key_action, video]", "evidence_recording.requested_granularity[1]"},
		{"evidence.checkpoints", "checkpoints: [node_entry, meaningful_revision, submission]", "checkpoints: [node_entry, mystery]", "evidence_recording.checkpoints[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, c.anchor, c.repl), Options{})
			assertDiag(t, r, SeverityError, RuleSchema, c.path)
		})
	}
	t.Run("locator.scope", func(t *testing.T) {
		src := mutate(t, yamlMinimal, "locator: {clip_alias: clip-1, time_signature: 2/4, bars: 4}", "locator: {scope: anything}")
		r := reportOf(t, src, Options{})
		assertDiag(t, r, SeverityError, RuleSchema, "workspace.regions[0].locator.scope")
	})
}

func TestSchemaDomainsAndFormats(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		path   string
		rule   string
		ruleID string
	}{
		{"revision-zero", mutate(t, yamlMinimal, "revision: 1", "revision: 0"), "revision", "", ""},
		{"time-signature-zero-denominator", mutate(t, yamlMinimal, `    time_signature: 2/4
    key_signature: C
`, `    time_signature: 2/0
    key_signature: C
`), "workspace.timing.time_signature", "", ""},
		{"time-signature-shape", mutate(t, yamlMinimal, `    time_signature: 2/4
    key_signature: C
`, `    time_signature: 2-4
    key_signature: C
`), "workspace.timing.time_signature", "", ""},
		{"tempo-not-positive", mutate(t, yamlMinimal, "tempo_bpm: 80", "tempo_bpm: 0"), "workspace.timing.tempo_bpm", "", ""},
		{"pickup-negative", mutate(t, yamlMinimal, "pickup_beats: 0", "pickup_beats: -1"), "workspace.timing.pickup_beats", "", ""},
		{"budget-max-turns", mutate(t, yamlMinimal, "max_turns: 8", "max_turns: 0"), "budget.max_turns", "", ""},
		{"budget-max-calls", mutate(t, yamlMinimal, "max_llm_calls: 16", "max_llm_calls: -2"), "budget.max_llm_calls", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, c.src, Options{})
			assertDiag(t, r, SeverityError, RuleSchema, c.path)
		})
	}
}

// ============================================================================
// E-REQ：L1 五必填段 / 必填字段 / FLOW-CW / REG-* 违例
// ============================================================================

func TestRequiredSections(t *testing.T) {
	cases := []struct {
		name string
		src  string
		path string
	}{
		{"objectives-missing", mutate(t, yamlMinimal, yamlObjectives, ""), "objectives"},
		{"objectives-empty", mutate(t, yamlMinimal, yamlObjectives, "objectives: []\n"), "objectives"},
		{"materials-missing", mutate(t, yamlMinimal, yamlMaterials, ""), "materials"},
		{"materials-empty", mutate(t, yamlMinimal, yamlMaterials, "materials: []\n"), "materials"},
		{"checks-missing", mutate(t, yamlMinimal, yamlChecks, ""), "checks"},
		{"checks-empty", mutate(t, yamlMinimal, yamlChecks, "checks: []\n"), "checks"},
		{"flow-missing", mutate(t, yamlMinimal, yamlFlow, ""), "flow"},
		{"flow-nodes-empty", mutate(t, yamlMinimal, yamlFlow, yamlFlowEmptyNodes), "flow.nodes"},
		{"workspace-missing", mutate(t, yamlMinimal, yamlWorkspace, ""), "workspace.regions"},
		{"regions-empty", mutate(t, yamlMinimal, yamlWorkspace, yamlWorkspaceNoRegions), "workspace.regions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, c.src, Options{})
			assertDiag(t, r, SeverityError, RuleRequired, c.path)
		})
	}
}

func TestRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		anchor string
		repl   string
		path   string
	}{
		{"objective-id-empty", "  - id: g1", "  - id: ''", "objectives[0].id"},
		{"objective-kind-missing", "    kind: learning\n", "", "objectives[0].kind"},
		{"objective-description-empty", "description: 能听出音高差别", "description: ''", "objectives[0].description"},
		{"material-id-empty", "  - id: m1", "  - id: ''", "materials[0].id"},
		{"material-purpose-empty", "purpose: 教材页", "purpose: ''", "materials[0].purpose"},
		{"material-source-empty", "source: 用户提供", "source: ''", "materials[0].source"},
		{"track-name-empty", "name: 旋律", "name: ''", "workspace.tracks[0].name"},
		{"region-id-empty", "    - id: r1", "    - id: ''", "workspace.regions[0].id"},
		{"region-track-empty", "track: t1", "track: ''", "workspace.regions[0].track"},
		{"region-purpose-empty", "purpose: 学生片段", "purpose: ''", "workspace.regions[0].purpose"},
		{"region-locator-missing", "      locator: {clip_alias: clip-1, time_signature: 2/4, bars: 4}\n", "", "workspace.regions[0].locator"},
		{"capability-id-empty", "  - {id: cap1,", "  - {id: '',", "capability_requests[0].id"},
		{"capability-request-empty", "request: 谱面编辑与试听", "request: ''", "capability_requests[0].request"},
		{"capability-required-missing", ", required: true", "", "capability_requests[0].required"},
		{"check-id-empty", "  - id: c1", "  - id: ''", "checks[0].id"},
		{"check-target-empty", "target: r1", "target: ''", "checks[0].target"},
		{"check-criterion-empty", "criterion: clip 存在", "criterion: ''", "checks[0].criterion"},
		{"check-evidence-empty", "evidence: 工程数据", "evidence: ''", "checks[0].evidence"},
		{"node-id-empty", "    - id: n1", "    - id: ''", "flow.nodes[0].id"},
		{"node-title-empty", "title: 第一步", "title: ''", "flow.nodes[0].title"},
		{"node-task-empty", "      task: 写两小节", "      task: ''", "flow.nodes[0].task"},
		{"flow-progression-missing", "  progression: ordered\n", "", "flow.progression"},
		{"flow-complete-when-empty", "complete_when: [c1]", "complete_when: []", "flow.nodes[0].complete_when"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, c.anchor, c.repl), Options{})
			assertDiag(t, r, SeverityError, RuleRequired, c.path)
		})
	}
}

func TestRegMeterLocAlias(t *testing.T) {
	locAnchor := "locator: {clip_alias: clip-1, time_signature: 2/4, bars: 4}"

	t.Run("REG-METER-mismatch", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {clip_alias: clip-1, time_signature: 4/4, bars: 4}"), Options{})
		d := assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator.time_signature")
		if !strings.Contains(d.Message, "REG-METER") {
			t.Fatalf("诊断须带 REG-METER 标记，got: %s", d.Message)
		}
	})
	t.Run("REG-METER-workspace-missing", func(t *testing.T) {
		src := mutate(t, yamlMinimal, "    time_signature: 2/4\n    key_signature: C", "    key_signature: C")
		r := reportOf(t, src, Options{})
		assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator.time_signature")
	})
	t.Run("REG-LOC-mixed", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {scope: four_clips, clip_alias: clip-1}"), Options{})
		d := assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator")
		if !strings.Contains(d.Message, "REG-LOC") {
			t.Fatalf("诊断须带 REG-LOC 标记，got: %s", d.Message)
		}
	})
	t.Run("REG-LOC-none", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {time_signature: 2/4}"), Options{})
		assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator")
	})
	t.Run("REG-LOC-bars-in-scope-form", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {scope: generated_audio_versions, bars: 2}"), Options{})
		assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator.bars")
	})
	t.Run("REG-LOC-bars-per-clip-non-four-clips", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {scope: prototype_and_variant, clip_aliases: [a, b], bars_per_clip: 2}"), Options{})
		assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator.bars_per_clip")
	})
	t.Run("REG-LOC-clip-alias-and-aliases", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, locAnchor, "locator: {clip_alias: clip-1, clip_aliases: [a]}"), Options{})
		assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[0].locator")
	})
	t.Run("REG-ALIAS-duplicate", func(t *testing.T) {
		src := mutate(t, yamlMinimal, "      purpose: 学生片段", `      purpose: 学生片段
    - id: r2
      track: t1
      locator: {clip_alias: clip-1}
      purpose: 第二片段
`)
		r := reportOf(t, src, Options{})
		d := assertDiag(t, r, SeverityError, RuleRequired, "workspace.regions[1].locator.clip_alias")
		if !strings.Contains(d.Message, "REG-ALIAS") {
			t.Fatalf("诊断须带 REG-ALIAS 标记，got: %s", d.Message)
		}
	})
	t.Run("four-clips-locator-accepted", func(t *testing.T) {
		src := mutate(t, yamlMinimal, locAnchor, "locator: {scope: four_clips, clip_aliases: [q1-a, q1-b], time_signature: 2/4, bars_per_clip: 4}")
		parseOK(t, src)
	})
}

// ============================================================================
// E-REF：REF-* 引用悬空 + 各列表 id 重复
// ============================================================================

func TestReferenceIntegrity(t *testing.T) {
	cases := []struct {
		name     string
		anchor   string
		repl     string
		path     string
		ruleName string // 空=与 name 相同
	}{
		{"REF-OBJ", "evidence_checks: [c1]", "evidence_checks: [c-nope]", "objectives[0].evidence_checks[0]", ""},
		{"REF-CHK", "target: r1", "target: r-nope", "checks[0].target", ""},
		{"REF-NODE-OBJ", "objective_refs: [g1]", "objective_refs: [g-nope]", "flow.nodes[0].objective_refs[0]", ""},
		{"REF-NODE-MAT", "uses_materials: [m1]", "uses_materials: [m-nope]", "flow.nodes[0].uses_materials[0]", ""},
		{"REF-NODE-REG", "uses_regions: [r1]", "uses_regions: [r-nope]", "flow.nodes[0].uses_regions[0]", ""},
		{"REF-NODE-CAP", "uses_capabilities: [cap1]", "uses_capabilities: [cap-nope]", "flow.nodes[0].uses_capabilities[0]", ""},
		{"REF-NODE-CHK", "complete_when: [c1]", "complete_when: [c-nope]", "flow.nodes[0].complete_when[0]", ""},
		{"REF-NODE-COLLECT", "collect_evidence: [c1]", "collect_evidence: [c-nope]", "flow.nodes[0].collect_evidence[0]", "REF-NODE-CHK"},
		{"REF-COMP-required", "required_checks: [c1]", "required_checks: [c-nope]", "completion.required_checks[0]", "REF-COMP"},
		{"REF-COMP-learning", "learning_evidence_to_record: [c1]", "learning_evidence_to_record: [c-nope]", "completion.learning_evidence_to_record[0]", "REF-COMP"},
		{"REF-REGION", "track: t1", "track: t-nope", "workspace.regions[0].track", ""},
		{"REF-MAT", "initial_material: m1", "initial_material: m-nope", "workspace.tracks[0].initial_material", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, c.anchor, c.repl), Options{})
			d := assertDiag(t, r, SeverityError, RuleRef, c.path)
			ruleName := c.ruleName
			if ruleName == "" {
				ruleName = c.name
			}
			if !strings.Contains(d.Message, ruleName) {
				t.Fatalf("诊断须带规则名 %s，got: %s", ruleName, d.Message)
			}
		})
	}

	t.Run("submission_snapshot-target-accepted", func(t *testing.T) {
		parseOK(t, mutate(t, yamlMinimal, "target: r1", "target: submission_snapshot"))
	})
}

func TestDuplicateIDs(t *testing.T) {
	cases := []struct {
		name   string
		anchor string
		repl   string
		path   string
	}{
		{"objective", "    evidence_checks: [c1]\n", `    evidence_checks: [c1]
  - id: g1
    kind: learning
    description: 重复目标
    evidence_checks: [c1]
`, "objectives[1].id"},
		{"check", "    on_unavailable: pause_and_report\n", `    on_unavailable: pause_and_report
  - id: c1
    purpose: task_progress
    method: rubric
    target: submission_snapshot
    criterion: 重复检查
    evidence: 重复证据
    on_unavailable: request_review
`, "checks[1].id"},
		{"track", "    - {id: t1, name: 旋律, role: user_work, initial_material: m1, editable_by_user: true}\n", `    - {id: t1, name: 旋律, role: user_work, initial_material: m1, editable_by_user: true}
    - {id: t1, name: 第二旋律, role: user_work, initial_material: null}
`, "workspace.tracks[1].id"},
		{"capability", "  - {id: cap1, request: 谱面编辑与试听, required: true, fallback: null}\n", `  - {id: cap1, request: 谱面编辑与试听, required: true, fallback: null}
  - {id: cap1, request: 重复能力, required: false, fallback: null}
`, "capability_requests[1].id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, c.anchor, c.repl), Options{})
			assertDiag(t, r, SeverityError, RuleRef, c.path)
		})
	}
}

// ============================================================================
// E-MAT（MAT-PATH）/ W-MAT
// ============================================================================

func TestMaterialPathRules(t *testing.T) {
	cases := []struct {
		name string
		repl string
	}{
		{"empty-string", "file: ''"},
		{"absolute-posix", "file: /abs/notes.md"},
		{"absolute-windows", "file: C:/abs/notes.md"},
		{"backslash", "file: 'assets\\\\notes.mid'"},
		{"parent-escape", "file: ../outside.mid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, "file: notes.md", c.repl), Options{})
			d := assertDiag(t, r, SeverityError, RuleMat, "materials[0].file")
			if !strings.Contains(d.Message, "MAT-PATH") {
				t.Fatalf("诊断须带 MAT-PATH 标记，got: %s", d.Message)
			}
		})
	}
	t.Run("valid-relative-path", func(t *testing.T) {
		parseOK(t, mutate(t, yamlMinimal, "file: notes.md", "file: assets/midi/demo.mid"))
	})
}

func TestMaterialWarnings(t *testing.T) {
	t.Run("file-null-not-prepared", func(t *testing.T) {
		d, r := Parse([]byte(mutate(t, yamlMinimal, "file: notes.md", "file: null")))
		if d == nil {
			t.Fatalf("W 不阻塞解析，应返回设计源；report:\n%s", r.String())
		}
		dg := assertDiag(t, r, SeverityWarning, RuleMatWarn, "materials[0].file")
		if !strings.Contains(dg.Message, "未备料") {
			t.Fatalf("W-MAT 提示须说明未备料，got: %s", dg.Message)
		}
	})
	for _, keyword := range []string{"待转写", "未听核"} {
		t.Run("source-"+keyword, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, "source: 用户提供", "source: 教材页"+keyword), Options{})
			assertDiag(t, r, SeverityWarning, RuleMatWarn, "materials[0].source")
		})
	}
	t.Run("no-warning-when-prepared", func(t *testing.T) {
		_, r := parseOK(t, yamlMinimal)
		if got := r.RuleCount(RuleMatWarn); got != 0 {
			t.Fatalf("已备料+已核 source 不应有 W-MAT（got %d）", got)
		}
	})
}

// ============================================================================
// E-CAP / W-CHK 表接口 stub（闭包表与判定面映射表由 W01-B/C 接线）
// ============================================================================

func TestCapabilityBridgeStub(t *testing.T) {
	t.Run("unwired-no-diagnostic", func(t *testing.T) {
		_, r := parseOK(t, yamlMinimal)
		if got := r.RuleCount(RuleCap); got != 0 {
			t.Fatalf("闭包表未接线（nil）不应产生 E-CAP（got %d）", got)
		}
	})
	t.Run("wired-missing-entry", func(t *testing.T) {
		r := reportOf(t, yamlMinimal, Options{CapabilityClosure: CapabilityClosureSet{}})
		d := assertDiag(t, r, SeverityError, RuleCap, "capability_requests[0]")
		if !strings.Contains(d.Message, "CAP-BRIDGE") {
			t.Fatalf("诊断须带 CAP-BRIDGE 标记，got: %s", d.Message)
		}
	})
	t.Run("wired-with-entry", func(t *testing.T) {
		_, r := ParseWithOptions([]byte(yamlMinimal), Options{CapabilityClosure: CapabilityClosureSet{"cap1": true}})
		if r.HasErrors() {
			t.Fatalf("闭包表已登记不应 E-CAP:\n%s", r.String())
		}
	})
	t.Run("not-required-no-diagnostic", func(t *testing.T) {
		src := mutate(t, yamlMinimal, ", required: true", ", required: false")
		_, r := ParseWithOptions([]byte(src), Options{CapabilityClosure: CapabilityClosureSet{}})
		if r.HasErrors() {
			t.Fatalf("required=false 且缺闭包表条目不应报错:\n%s", r.String())
		}
	})
}

func TestCheckJudgementMappingStub(t *testing.T) {
	t.Run("unwired-no-diagnostic", func(t *testing.T) {
		_, r := parseOK(t, yamlMinimal)
		if got := r.RuleCount(RuleCheckWarn); got != 0 {
			t.Fatalf("判定面映射表未接线（nil）不应产生 W-CHK（got %d）", got)
		}
	})
	t.Run("wired-unmapped", func(t *testing.T) {
		r := reportOf(t, yamlMinimal, Options{CheckMapping: CheckJudgementSet{}})
		assertDiag(t, r, SeverityWarning, RuleCheckWarn, "checks[0]")
		// complete_when 全部不可映射 → 节点级 W-CHK
		assertDiag(t, r, SeverityWarning, RuleCheckWarn, "flow.nodes[0].complete_when")
	})
	t.Run("wired-mapped", func(t *testing.T) {
		r := reportOf(t, yamlMinimal, Options{CheckMapping: CheckJudgementSet{"c1": true}})
		if got := r.RuleCount(RuleCheckWarn); got != 0 {
			t.Fatalf("判定面已映射不应 W-CHK（got %d）", got)
		}
	})
}

// ============================================================================
// R6a：30 调封闭枚举（合法 30 / 集合外拒绝）
// ============================================================================

func TestKeySignatureClosedSetR6a(t *testing.T) {
	all := append(append([]string{}, MajorKeySignatures...), MinorKeySignatures...)
	if len(all) != 30 {
		t.Fatalf("R6a 闭集须为 30 调（15 大调+15 关系小调），got %d", len(all))
	}
	for _, key := range all {
		t.Run("valid-"+key, func(t *testing.T) {
			parseOK(t, mutate(t, yamlMinimal, "key_signature: C", "key_signature: "+key))
		})
	}
	outside := []string{"H", "c", "Cmin", "C minor", "Fbm", "G#", "Am#", "Db-", "C#m#"}
	for _, key := range outside {
		t.Run("outside-"+key, func(t *testing.T) {
			r := reportOf(t, mutate(t, yamlMinimal, "key_signature: C", "key_signature: "+key), Options{})
			d := assertDiag(t, r, SeverityError, RuleSchema, "workspace.timing.key_signature")
			if !strings.Contains(d.Message, "R6a") {
				t.Fatalf("诊断须说明 R6a 闭集口径，got: %s", d.Message)
			}
		})
	}
}

// ============================================================================
// R6b：locator.music_region 单值/列表/null 归一为 list
// ============================================================================

func TestMusicRegionNormalizationR6b(t *testing.T) {
	d, _ := parseOK(t, yamlMinimal)
	if got := d.Materials[0].Locator.MusicRegion; len(got) != 1 || got[0] != "第一句" {
		t.Fatalf("单值 music_region 应归一为 [\"第一句\"]，got %v", got)
	}
	t.Run("list", func(t *testing.T) {
		d, _ := parseOK(t, mutate(t, yamlMinimal, "music_region: 第一句", "music_region: [第一句, 第二句]"))
		if got := d.Materials[0].Locator.MusicRegion; len(got) != 2 || got[1] != "第二句" {
			t.Fatalf("列表 music_region 应原样保留，got %v", got)
		}
	})
	t.Run("null", func(t *testing.T) {
		d, _ := parseOK(t, mutate(t, yamlMinimal, "music_region: 第一句", "music_region: null"))
		if got := d.Materials[0].Locator.MusicRegion; got != nil {
			t.Fatalf("null music_region 应归一为 nil，got %v", got)
		}
	})
	t.Run("invalid-mapping", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, "music_region: 第一句", "music_region: {a: b}"), Options{})
		d := assertDiag(t, r, SeverityError, RuleSchema, "")
		if !strings.Contains(d.Message, "music_region") {
			t.Fatalf("music_region 类型错诊断须说明字段，got: %s", d.Message)
		}
	})
	t.Run("invalid-scalar-int", func(t *testing.T) {
		r := reportOf(t, mutate(t, yamlMinimal, "music_region: 第一句", "music_region: 123"), Options{})
		assertDiag(t, r, SeverityError, RuleSchema, "")
	})
}

// ============================================================================
// I-NOTE：设计层字段不进卡 / estimated_minutes=null / pending_questions 计数 / 缺省注入提示
// ============================================================================

func TestInfoNotes(t *testing.T) {
	_, r := parseOK(t, yamlMinimal)
	estimated := assertDiag(t, r, SeverityInfo, RuleNote, "brief.estimated_minutes")
	if !strings.Contains(estimated.Message, "null") {
		t.Fatalf("estimated_minutes=null 提示缺失，got: %s", estimated.Message)
	}
	pending := assertDiag(t, r, SeverityInfo, RuleNote, "pending_questions")
	if !strings.Contains(pending.Message, "1 条") {
		t.Fatalf("pending_questions 计数提示缺失，got: %s", pending.Message)
	}
	for _, p := range []string{
		"workspace.timing.key_signature",
		"workspace.tracks[*].editable_by_user",
		"flow.nodes[*].uses_materials",
		"flow.nodes[*].uses_regions",
		"flow.nodes[*].uses_capabilities",
		"flow.nodes[*].human_decides",
		"flow.nodes[*].freedom",
		"flow.nodes[*].leaves_behind",
		"flow.nodes[*].transition",
	} {
		assertDiag(t, r, SeverityInfo, RuleNote, p)
	}

	t.Run("estimated-minutes-declared", func(t *testing.T) {
		_, r := parseOK(t, mutate(t, yamlMinimal, "estimated_minutes: null", "estimated_minutes: 45"))
		for _, d := range r.Infos() {
			if d.Path == "brief.estimated_minutes" {
				t.Fatalf("estimated_minutes 已声明时不应再出 null I-NOTE")
			}
		}
	})
	t.Run("pending-questions-empty", func(t *testing.T) {
		src := mutate(t, yamlMinimal, `pending_questions:
  - 待确认
`, "pending_questions: []\n")
		_, r := parseOK(t, src)
		for _, d := range r.Infos() {
			if d.Path == "pending_questions" {
				t.Fatalf("pending_questions 为空时不应出计数 I-NOTE")
			}
		}
	})
	t.Run("optional-segments-absent", func(t *testing.T) {
		_, r := parseOK(t, yamlRequired)
		for _, p := range []string{"evidence_recording", "submission", "completion"} {
			d := assertDiag(t, r, SeverityInfo, RuleNote, p)
			if !strings.Contains(d.Message, "缺省") {
				t.Fatalf("%s 缺省注入提示须说明缺省，got: %s", p, d.Message)
			}
		}
	})
}

// ============================================================================
// R5：tempo/budget 缺失→E（fail-closed，不注入缺省，诊断可行动）
// ============================================================================

// r5MissingSource 构造 tempo_bpm/budget 两数值均显式 null 的设计源。
func r5MissingSource(t *testing.T) string {
	t.Helper()
	src := mutate(t, yamlMinimal, "tempo_bpm: 80", "tempo_bpm: null")
	src = mutate(t, src, "max_turns: 8", "max_turns: null")
	src = mutate(t, src, "max_llm_calls: 16", "max_llm_calls: null")
	return src
}

func TestR5CompileReadiness(t *testing.T) {
	src := r5MissingSource(t)

	// 设计期显式 null 在源校验入口合法（契约 `4.6/`4.14：null=未定，编译 E+覆盖参数）。
	d, srcReport := parseOK(t, src)
	if srcReport.HasErrors() {
		t.Fatalf("源校验不应把设计期 null 判 E")
	}

	t.Run("no-overrides-fails-closed", func(t *testing.T) {
		r := ValidateCompileReadiness(d, CompileOverrides{})
		if !r.HasErrors() {
			t.Fatalf("R5：tempo/budget 缺失且无覆盖参数须 E，实际:\n%s", r.String())
		}
		for _, p := range []string{"workspace.timing.tempo_bpm", "budget.max_turns", "budget.max_llm_calls"} {
			diag := assertDiag(t, r, SeverityError, RuleBudget, p)
			if !strings.Contains(diag.Message, "R5") || !strings.Contains(diag.Message, "覆盖参数") {
				t.Fatalf("R5 诊断须可行动（R5+覆盖参数提示），got: %s", diag.Message)
			}
		}
		if got := r.RuleCount(RuleBudget); got != 3 {
			t.Fatalf("R5 缺失项须逐项出 E（tempo+budget 两数值=3），got %d", got)
		}
	})

	t.Run("explicit-overrides-pass", func(t *testing.T) {
		r := ValidateCompileReadiness(d, CompileOverrides{
			TempoBPM:    floatPtr(78),
			MaxTurns:    intPtr(6),
			MaxLLMCalls: intPtr(12),
		})
		if r.HasErrors() {
			t.Fatalf("提供显式覆盖参数后 R5 应通过，实际:\n%s", r.String())
		}
		for _, p := range []string{"workspace.timing.tempo_bpm", "budget.max_turns", "budget.max_llm_calls"} {
			diag := assertDiag(t, r, SeverityInfo, RuleNote, p)
			if !strings.Contains(diag.Message, "覆盖参数") {
				t.Fatalf("覆盖参数 I-NOTE 须说明来源，got: %s", diag.Message)
			}
		}
	})

	t.Run("invalid-override", func(t *testing.T) {
		r := ValidateCompileReadiness(d, CompileOverrides{TempoBPM: floatPtr(0), MaxTurns: intPtr(0)})
		assertDiag(t, r, SeverityError, RuleSchema, "workspace.timing.tempo_bpm")
		assertDiag(t, r, SeverityError, RuleSchema, "budget.max_turns")
	})

	t.Run("non-design-version", func(t *testing.T) {
		r := ValidateCompileReadiness(&Design{SchemaVersion: SchemaVersionProjectIntent}, CompileOverrides{})
		assertDiag(t, r, SeverityError, RuleSchema, "schema_version")
	})

	// 设计源实例（revision 3）本身 tempo/budget 均 null：编译就绪判定应 3 E，
	// 这正是 R5 与「正例源校验 0 E」并存的边界（源=设计期文档，编译=目标卡要求）。
	t.Run("fixture-fails-r5-without-overrides", func(t *testing.T) {
		fixture := readFixture(t)
		fd, fr := Parse(fixture)
		if fd == nil {
			t.Fatalf("fixture 解析应通过（源校验 0 E）:\n%s", fr.String())
		}
		r := ValidateCompileReadiness(fd, CompileOverrides{})
		if got := r.RuleCount(RuleBudget); got != 3 {
			t.Fatalf("设计源实例 R5 缺失项应为 3（tempo + budget 两数值），got %d\n%s", got, r.String())
		}
	})
}

// ============================================================================
// 聚合 / 确定性 / 诊断格式
// ============================================================================

func TestAggregatedDiagnostics(t *testing.T) {
	src := yamlMinimal
	src = mutate(t, src, "kind: learning", "kind: dance")
	src = mutate(t, src, "target: r1", "target: r-nope")
	src = mutate(t, src, "file: notes.md", "file: /abs/x")
	r := reportOf(t, src, Options{})
	errors, _, _ := r.Counts()
	if errors < 3 {
		t.Fatalf("聚合须一次列出全部 E（≥3），got %d:\n%s", errors, r.String())
	}
	for _, rule := range []string{RuleSchema, RuleRef, RuleMat} {
		if r.RuleCount(rule) < 1 {
			t.Fatalf("聚合诊断缺少规则 %s:\n%s", rule, r.String())
		}
	}
	err := r.Err()
	if err == nil {
		t.Fatalf("存在 E 时 Err() 须非 nil")
	}
	if !strings.Contains(err.Error(), "E-SCHEMA") {
		t.Fatalf("Err() 聚合文本须含规则号，got: %v", err)
	}
}

func TestDiagnosticFormatAndDeterminism(t *testing.T) {
	src := mutate(t, yamlMinimal, "target: r1", "target: r-nope")
	r1 := reportOf(t, src, Options{})
	r2 := reportOf(t, src, Options{})
	if r1.String() != r2.String() {
		t.Fatalf("同一输入两次校验的诊断须完全一致（确定性）")
	}
	d := assertDiag(t, r1, SeverityError, RuleRef, "checks[0].target")
	if !strings.HasPrefix(d.String(), "[E-REF] checks[0].target ") {
		t.Fatalf("诊断形态须为 \"[规则号] 字段路径 提示\"，got: %s", d.String())
	}
	for _, dg := range r1.Diagnostics() {
		if dg.Path == "" && dg.Rule == RuleRef {
			t.Fatalf("引用类 E 诊断须带字段路径: %s", dg.String())
		}
	}
	// 正例：0 E → Err()==nil；W/I 不构成错误
	_, ok := parseOK(t, mutate(t, yamlMinimal, "file: notes.md", "file: null"))
	if ok.Err() != nil {
		t.Fatalf("仅 W 不应构成错误: %v", ok.Err())
	}
}

// ============================================================================
// 规则负例覆盖对账：契约 `5.2 规则号每条至少 1 个负例
// ============================================================================

func TestEveryRuleHasNegativeCase(t *testing.T) {
	cases := []struct {
		name string
		src  string
		opts Options
		rule string
		sev  Severity
	}{
		{"E-SCHEMA-enum", mutate(t, yamlMinimal, "kind: learning", "kind: dance"), Options{}, RuleSchema, SeverityError},
		{"E-REQ-missing-section", mutate(t, yamlMinimal, yamlObjectives, ""), Options{}, RuleRequired, SeverityError},
		{"E-REF-dangling", mutate(t, yamlMinimal, "target: r1", "target: r-nope"), Options{}, RuleRef, SeverityError},
		{"E-MAT-path", mutate(t, yamlMinimal, "file: notes.md", "file: ../escape.md"), Options{}, RuleMat, SeverityError},
		{"E-CAP-bridge", yamlMinimal, Options{CapabilityClosure: CapabilityClosureSet{}}, RuleCap, SeverityError},
		{"W-CHK-unmapped", yamlMinimal, Options{CheckMapping: CheckJudgementSet{}}, RuleCheckWarn, SeverityWarning},
		{"W-MAT-null-file", mutate(t, yamlMinimal, "file: notes.md", "file: null"), Options{}, RuleMatWarn, SeverityWarning},
		{"I-NOTE-info", yamlMinimal, Options{}, RuleNote, SeverityInfo},
	}
	covered := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := reportOf(t, c.src, c.opts)
			found := false
			for _, d := range r.Diagnostics() {
				if d.Rule == c.rule && d.Severity == c.sev {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("规则 %s 负例未产生 %s 诊断:\n%s", c.rule, c.sev, r.String())
			}
			covered[c.rule] = true
		})
	}
	// R5：规则号 W-BUDGET 的 E 级出口（契约 `5.2 原文：缺失→E）
	t.Run("W-BUDGET-R5", func(t *testing.T) {
		d, _ := parseOK(t, r5MissingSource(t))
		r := ValidateCompileReadiness(d, CompileOverrides{})
		found := false
		for _, diag := range r.Diagnostics() {
			if diag.Rule == RuleBudget && diag.Severity == SeverityError {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("R5 未产生 E 级 %s 诊断:\n%s", RuleBudget, r.String())
		}
		covered[RuleBudget] = true
	})
	for _, rule := range AllRules() {
		if !covered[rule] {
			t.Fatalf("契约 `5.2 规则 %s 缺少负例覆盖", rule)
		}
	}
}
