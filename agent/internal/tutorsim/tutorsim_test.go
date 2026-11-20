package tutorsim

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validExperimentYAML = `schema_version: vit.tutor_sim.v0
id: test-3step
persona: |
  You are a concise tutor.
  Never answer for the learner.
steps:
  - id: step-1
    ask: "Ask one"
    expect: "7"
    retry: "Retry one"
    pass: "Pass one"
  - id: step-2
    ask: "Ask two"
    expect: "42"
    retry: "Retry two"
    pass: "Pass two"
  - id: step-3
    ask: "Ask three"
    expect: "123"
    retry: "Retry three"
    pass: "Pass three"
`

func mustParse(t *testing.T, data string) *Experiment {
	t.Helper()
	exp, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return exp
}

func TestParseValidExperiment(t *testing.T) {
	exp := mustParse(t, validExperimentYAML)
	if exp.SchemaVersion != SchemaVersion || exp.ID != "test-3step" {
		t.Fatalf("header = %+v", exp)
	}
	if !strings.Contains(exp.Persona, "concise tutor") {
		t.Fatalf("persona = %q", exp.Persona)
	}
	if len(exp.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(exp.Steps))
	}
	if exp.Steps[0].ID != "step-1" || exp.Steps[1].Expect != "42" {
		t.Fatalf("steps = %+v", exp.Steps)
	}
}

func TestStepMatchesTrimsSurroundingWhitespaceOnly(t *testing.T) {
	step := Step{Expect: "42"}
	for _, input := range []string{"42", " 42 ", "\t42\n"} {
		if !step.Matches(input) {
			t.Errorf("Matches(%q) = false, want true", input)
		}
	}
	for _, input := range []string{"", "4 2", "42x", "四十二", "042"} {
		if step.Matches(input) {
			t.Errorf("Matches(%q) = true, want false", input)
		}
	}
}

func TestParseRejectsInvalidExperiment(t *testing.T) {
	replace := func(old, new string) string {
		if !strings.Contains(validExperimentYAML, old) {
			t.Fatalf("fixture missing %q", old)
		}
		return strings.Replace(validExperimentYAML, old, new, 1)
	}
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{name: "wrong schema", yaml: replace("vit.tutor_sim.v0", "vit.tutor_sim.v2"), want: "schema_version"},
		{name: "missing id", yaml: replace("id: test-3step", "id: \"\""), want: "id is required"},
		{name: "missing persona", yaml: replace("persona: |\n  You are a concise tutor.\n  Never answer for the learner.", "persona: \"\""), want: "persona is required"},
		{name: "empty expect", yaml: replace("expect: \"7\"", "expect: \"\""), want: "expect is required"},
		{name: "empty retry", yaml: replace("retry: \"Retry one\"", "retry: \"\""), want: "retry is required"},
		{name: "empty pass", yaml: replace("pass: \"Pass one\"", "pass: \"\""), want: "pass is required"},
		{name: "duplicate step id", yaml: replace("id: step-2", "id: step-1"), want: "duplicate step id"},
		{name: "unknown top-level field", yaml: validExperimentYAML + "extra: true\n", want: "field extra not found"},
		{name: "unknown step field", yaml: replace("retry: \"Retry one\"", "retry: \"Retry one\"\n    hint: nope"), want: "field hint not found"},
		{name: "trailing document", yaml: validExperimentYAML + "---\nid: second\n", want: "multiple YAML documents"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(tc.yaml)); err == nil {
				t.Fatal("Parse() succeeded, want error")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestParseEmptyDocument(t *testing.T) {
	if _, err := Parse(nil); err == nil || !strings.Contains(err.Error(), "empty experiment") {
		t.Fatalf("Parse(nil) error = %v", err)
	}
}

func writeProject(t *testing.T, withSidecar bool) (projectPath, sidecarPath string) {
	t.Helper()
	dir := t.TempDir()
	projectPath = filepath.Join(dir, "blank.vit")
	if err := os.WriteFile(projectPath, []byte("<project/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	sidecarPath = filepath.Join(dir, filepath.FromSlash(SidecarRelPath))
	if withSidecar {
		if err := os.MkdirAll(filepath.Dir(sidecarPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sidecarPath, []byte(validExperimentYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return projectPath, sidecarPath
}

func TestLoadProjectMissingSidecar(t *testing.T) {
	projectPath, sidecarPath := writeProject(t, false)
	exp, gotSidecar, err := LoadProject(projectPath)
	if !errors.Is(err, ErrSidecarMissing) {
		t.Fatalf("LoadProject() error = %v, want ErrSidecarMissing", err)
	}
	if exp != nil || gotSidecar != sidecarPath {
		t.Fatalf("exp=%v sidecar=%q, want nil/%q", exp, gotSidecar, sidecarPath)
	}
}

func TestLoadProjectLoadsSidecarFromFileAndDirectory(t *testing.T) {
	projectPath, _ := writeProject(t, true)
	for _, input := range []string{projectPath, filepath.Dir(projectPath)} {
		exp, sidecar, err := LoadProject(input)
		if err != nil {
			t.Fatalf("LoadProject(%q) error = %v", input, err)
		}
		if exp == nil || exp.ID != "test-3step" {
			t.Fatalf("LoadProject(%q) experiment = %+v", input, exp)
		}
		if sidecar == "" || !strings.HasSuffix(sidecar, filepath.Join(".vit_agent", "tutor_sim.yaml")) {
			t.Fatalf("LoadProject(%q) sidecar = %q", input, sidecar)
		}
	}
}

func TestShippedExperimentParses(t *testing.T) {
	path := filepath.Join("..", "..", "..", "coord", "experiments", "tutor-sim-1", "tutor_sim.yaml")
	exp, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%s) error = %v", path, err)
	}
	if exp.SchemaVersion != SchemaVersion || exp.ID != "tutor-sim-3step" {
		t.Fatalf("shipped experiment header = %+v", exp)
	}
	if len(exp.Steps) != 3 {
		t.Fatalf("shipped steps = %d, want 3", len(exp.Steps))
	}
	want := []string{"7", "42", "123"}
	for i, step := range exp.Steps {
		if step.Expect != want[i] {
			t.Fatalf("shipped step %d expect = %q, want %q", i+1, step.Expect, want[i])
		}
	}
}

func startSession(t *testing.T, store *Store) (*Experiment, string) {
	t.Helper()
	exp := mustParse(t, validExperimentYAML)
	conversationID := "conv-1"
	if _, err := store.Start(conversationID, filepath.Join(t.TempDir(), "blank.vit"), exp); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	return exp, conversationID
}

func TestStoreWrongAnswerDoesNotAdvance(t *testing.T) {
	store := NewStore()
	_, conversationID := startSession(t, store)
	verdict, _, ok := store.Submit(conversationID, " 6 ")
	if !ok {
		t.Fatal("Submit() missed the session")
	}
	if verdict.Passed || verdict.Completed || verdict.AlreadyCompleted {
		t.Fatalf("wrong answer verdict = %+v", verdict)
	}
	if verdict.StepIndex != 1 || verdict.StepID != "step-1" || verdict.Attempts != 1 || verdict.Failures != 1 {
		t.Fatalf("wrong answer state = %+v", verdict)
	}
	snap, ok := store.Status(conversationID)
	if !ok || snap.StepIndex != 1 || snap.Completed || snap.Attempts != 1 || snap.Failures != 1 {
		t.Fatalf("status after wrong answer = %+v ok=%t", snap, ok)
	}
}

func TestStoreCorrectAnswersAdvanceAndComplete(t *testing.T) {
	store := NewStore()
	_, conversationID := startSession(t, store)

	verdict, exp, ok := store.Submit(conversationID, " 7 ")
	if !ok || !verdict.Passed || verdict.Completed {
		t.Fatalf("step 1 verdict = %+v ok=%t", verdict, ok)
	}
	if verdict.StepIndex != 2 || verdict.StepID != "step-2" || verdict.AnsweredStep.ID != "step-1" {
		t.Fatalf("step 1 advance = %+v", verdict)
	}
	if exp.Steps[1].Ask == "" {
		t.Fatal("experiment lost step 2")
	}

	verdict, _, _ = store.Submit(conversationID, "41")
	if verdict.Passed || verdict.StepIndex != 2 || verdict.Failures != 1 {
		t.Fatalf("step 2 wrong verdict = %+v", verdict)
	}

	verdict, _, _ = store.Submit(conversationID, "42")
	if !verdict.Passed || verdict.Completed || verdict.StepIndex != 3 || verdict.StepID != "step-3" {
		t.Fatalf("step 2 verdict = %+v", verdict)
	}

	verdict, _, _ = store.Submit(conversationID, "123")
	if !verdict.Passed || !verdict.Completed || verdict.AlreadyCompleted {
		t.Fatalf("step 3 verdict = %+v", verdict)
	}
	if verdict.StepIndex != 3 || verdict.StepID != "step-3" || verdict.AnsweredStep.ID != "step-3" {
		t.Fatalf("step 3 state = %+v", verdict)
	}

	snap, ok := store.Status(conversationID)
	if !ok || !snap.Completed || snap.Status != StatusCompleted || snap.StepIndex != 3 || snap.Advances != 3 {
		t.Fatalf("completed status = %+v ok=%t", snap, ok)
	}

	before := snap
	verdict, _, ok = store.Submit(conversationID, "7")
	if !ok || !verdict.Completed || !verdict.AlreadyCompleted || verdict.Passed {
		t.Fatalf("post-completion verdict = %+v ok=%t", verdict, ok)
	}
	after, _ := store.Status(conversationID)
	if after.Attempts != before.Attempts || after.StepIndex != before.StepIndex {
		t.Fatalf("post-completion input changed state: before=%+v after=%+v", before, after)
	}
}

func TestStoreSessionsAreIsolated(t *testing.T) {
	store := NewStore()
	exp := mustParse(t, validExperimentYAML)
	for _, id := range []string{"conv-a", "conv-b"} {
		if _, err := store.Start(id, "proj.vit", exp); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, ok := store.Submit("conv-a", "7"); !ok {
		t.Fatal("conv-a submit missed")
	}
	a, _ := store.Status("conv-a")
	b, _ := store.Status("conv-b")
	if a.StepIndex != 2 || b.StepIndex != 1 {
		t.Fatalf("isolation failed: a=%+v b=%+v", a, b)
	}
}

func TestStoreUnknownConversationAndValidation(t *testing.T) {
	store := NewStore()
	if _, _, ok := store.Submit("missing", "7"); ok {
		t.Fatal("Submit(missing) = ok, want miss")
	}
	if _, ok := store.Status("missing"); ok {
		t.Fatal("Status(missing) = ok, want miss")
	}
	if _, err := store.Start("", "p.vit", mustParse(t, validExperimentYAML)); err == nil {
		t.Fatal("Start with empty conversation id succeeded")
	}
	if _, err := store.Start("conv", "p.vit", &Experiment{ID: "no-steps"}); err == nil {
		t.Fatal("Start with no steps succeeded")
	}
}

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（池序 438）：v1 schema（brief/materials）+ MIDI 材料面      */
/* ------------------------------------------------------------------ */

const v1ExperimentYAML = `schema_version: vit.tutor_sim.v1
id: test-v1-material
persona: |
  Concise tutor.
steps:
  - id: step-listen
    ask: "Listen and answer"
    expect: "42"
    retry: "Retry listen"
    pass: "Pass listen"
    brief: |
      教学简报：先装载材料，再听。
    materials:
      - kind: midi_clip
        title: 主题材料
        path: assets/theme.mid
  - id: step-confirm
    ask: "Confirm"
    expect: "完成"
    retry: "Retry confirm"
    pass: "Pass confirm"
`

func TestParseV1KeepsBriefAndMaterials(t *testing.T) {
	exp := mustParse(t, v1ExperimentYAML)
	if exp.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("schema_version = %q", exp.SchemaVersion)
	}
	if exp.Steps[0].Brief == "" || !strings.Contains(exp.Steps[0].Brief, "教学简报") {
		t.Fatalf("brief lost: %q", exp.Steps[0].Brief)
	}
	if exp.Steps[1].Brief != "" || len(exp.Steps[1].Materials) != 0 {
		t.Fatalf("step without brief/materials should default empty: %+v", exp.Steps[1])
	}
	materials := exp.Steps[0].Materials
	if len(materials) != 1 || materials[0].Kind != MaterialKindMidiClip || materials[0].Title != "主题材料" || materials[0].Path != "assets/theme.mid" {
		t.Fatalf("materials = %+v", materials)
	}
}

func TestParseV0DefaultsBriefAndMaterialsEmpty(t *testing.T) {
	// v0 文档仍按原样解析：brief/materials 缺省空（UI 不渲染卡的契约面）。
	exp := mustParse(t, validExperimentYAML)
	if exp.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", exp.SchemaVersion)
	}
	for i, step := range exp.Steps {
		if step.Brief != "" || len(step.Materials) != 0 {
			t.Fatalf("v0 step %d carries v1 fields: %+v", i+1, step)
		}
	}
}

func TestParseV1RejectsBadMaterials(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"empty kind", strings.Replace(v1ExperimentYAML, "kind: midi_clip", "kind: \"\"", 1), "materials[0].kind is required"},
		{"unknown kind", strings.Replace(v1ExperimentYAML, "kind: midi_clip", "kind: video", 1), "materials[0].kind must be"},
		{"missing title", strings.Replace(v1ExperimentYAML, "title: 主题材料", "title: \"\"", 1), "materials[0].title is required"},
		{"midi_clip without path", strings.Replace(v1ExperimentYAML, "path: assets/theme.mid", "path: \"\"", 1), "path is required for kind \"midi_clip\""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Parse() error = %v, want substring %q", err, tc.want)
			}
		})
	}
	// audio 是 schema 位：无 path 合法（本轮无内容）。
	audioYAML := `schema_version: vit.tutor_sim.v1
id: test-audio-slot
persona: |
  Concise tutor.
steps:
  - id: step-audio
    ask: "Listen"
    expect: "ok"
    retry: "Retry"
    pass: "Pass"
    materials:
      - kind: audio
        title: 音频材料位
`
	if _, err := Parse([]byte(audioYAML)); err != nil {
		t.Fatalf("audio slot without path should parse, error = %v", err)
	}
}

func TestV1MaterialsAreFailClosedOnUnknownFields(t *testing.T) {
	yaml := strings.Replace(v1ExperimentYAML, "path: assets/theme.mid", "path: assets/theme.mid\n        format: wav", 1)
	if _, err := Parse([]byte(yaml)); err == nil || !strings.Contains(err.Error(), "field format not found") {
		t.Fatalf("unknown material field accepted: %v", err)
	}
}

func TestShippedTutorSim2ParsesWithMaterialSummary(t *testing.T) {
	path := filepath.Join("..", "..", "..", "coord", "experiments", "tutor-sim-2", "tutor_sim.yaml")
	exp, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(%s) error = %v", path, err)
	}
	if exp.SchemaVersion != SchemaVersionV1 || exp.ID != "tutor-sim-2-twinkle" {
		t.Fatalf("shipped experiment header = %+v", exp)
	}
	if len(exp.Steps) != 4 {
		t.Fatalf("shipped steps = %d, want 4", len(exp.Steps))
	}
	for i, step := range exp.Steps {
		if step.Brief == "" {
			t.Fatalf("shipped step %d has no brief", i+1)
		}
	}
	if exp.Steps[0].Expect != "42" || exp.Steps[1].Expect != "模进" || exp.Steps[2].Expect != "已装载" || exp.Steps[3].Expect != "完成" {
		t.Fatalf("shipped expects drifted: %v", []string{exp.Steps[0].Expect, exp.Steps[1].Expect, exp.Steps[2].Expect, exp.Steps[3].Expect})
	}
	// 材料=冻结主题 MIDI 的原样复制：摘要必须落在 42 音 / 48 拍 / 100 BPM。
	seen := 0
	for _, step := range exp.Steps {
		if len(step.Materials) == 0 {
			continue
		}
		seen++
		material := step.Materials[0]
		if material.Kind != MaterialKindMidiClip || material.SummaryError != "" {
			t.Fatalf("material summary failed: %+v", material)
		}
		if material.NoteCount != 42 || math.Abs(material.DurationBeats-48) > 1e-9 || math.Abs(material.TempoBPM-100) > 1e-6 {
			t.Fatalf("material summary = count %d beats %f tempo %f, want 42/48/100", material.NoteCount, material.DurationBeats, material.TempoBPM)
		}
		if material.AbsPath == "" || !filepath.IsAbs(material.AbsPath) {
			t.Fatalf("material AbsPath not resolved: %q", material.AbsPath)
		}
	}
	if seen != 2 {
		t.Fatalf("materials on %d steps, want 2", seen)
	}
}

/* -------------------- 最小 SMF 解析 -------------------- */

// buildTestSMF 手工构造 format 0 SMF：480 ticks/qn、tempo meta 与 note
// on/off 对（delta VLQ 显式给字节）。
func buildTestSMF(t *testing.T, trackBody []byte) []byte {
	t.Helper()
	out := []byte{}
	header := make([]byte, 0, 14)
	header = append(header, 'M', 'T', 'h', 'd')
	header = binary.BigEndian.AppendUint32(header, 6)
	header = binary.BigEndian.AppendUint16(header, 0)
	header = binary.BigEndian.AppendUint16(header, 1)
	header = binary.BigEndian.AppendUint16(header, 480)
	out = append(out, header...)
	out = append(out, 'M', 'T', 'r', 'k')
	out = binary.BigEndian.AppendUint32(out, uint32(len(trackBody)))
	out = append(out, trackBody...)
	return out
}

func TestParseMidiNotesAndSummary(t *testing.T) {
	// tempo 600000 µs/qn（100 BPM）；delta 全部 ≤127 保证单字节 VLQ。
	doc, err := ParseMidi(buildTestSMF(t, []byte{
		0x00, 0xFF, 0x51, 0x03, 0x09, 0x27, 0xC0, // tempo 100 BPM
		0x00, 0x90, 60, 90,  // tick 0：note on 60
		0x7F, 0x80, 60, 64,  // tick 127：note off（未满一拍）
		0x51, 0x90, 67, 100, // tick 208：note on 67
		0x7F, 0x80, 67, 64,  // tick 335：note off
		0x00, 0xFF, 0x2F, 0x00,
	}))
	if err != nil {
		t.Fatalf("ParseMidi() error = %v", err)
	}
	if doc.Summary.NoteCount != 2 {
		t.Fatalf("note count = %d, want 2", doc.Summary.NoteCount)
	}
	if math.Abs(doc.Summary.TempoBPM-100) > 1e-6 || doc.Summary.Division != 480 {
		t.Fatalf("summary = %+v", doc.Summary)
	}
	if len(doc.Notes) != 2 || doc.Notes[0].Pitch != 60 || doc.Notes[1].Pitch != 67 {
		t.Fatalf("notes = %+v", doc.Notes)
	}
	if doc.Notes[0].Start != 0 || math.Abs(doc.Notes[0].Length-127.0/480.0) > 1e-9 {
		t.Fatalf("note 0 = %+v", doc.Notes[0])
	}
	if math.Abs(doc.Notes[1].Start-208.0/480.0) > 1e-9 || math.Abs(doc.Notes[1].Length-127.0/480.0) > 1e-9 {
		t.Fatalf("note 1 = %+v", doc.Notes[1])
	}
	// 时长=最后一个音符的结束点。
	if math.Abs(doc.Summary.DurationBeats-335.0/480.0) > 1e-9 {
		t.Fatalf("duration = %f, want %f", doc.Summary.DurationBeats, 335.0/480.0)
	}
}

func TestParseMidiRunningStatusAndDefaultTempo(t *testing.T) {
	// running status：首事件显式 0x90，随后 delta+数据字节复用 0x90；
	// note-on velocity 0 作为关闭语义（SMF 惯例）。无 tempo meta → 缺省 120。
	doc, err := ParseMidi(buildTestSMF(t, []byte{
		0x00, 0x90, 60, 64, // note on 60 vel 100（状态字节显式）
		0x40, 60, 0x00, // delta 64，running status note-on vel 0 = 关闭
		0x00, 0xFF, 0x2F, 0x00,
	}))
	if err != nil {
		t.Fatalf("ParseMidi() error = %v", err)
	}
	if doc.Summary.NoteCount != 1 || doc.Summary.TempoBPM != 120 || math.Abs(doc.Notes[0].Length-64.0/480.0) > 1e-9 {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestParseMidiFailClosed(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"missing header", []byte("not a midi file......"), "MThd"},
		{"smpte division", func() []byte {
			out := []byte{'M', 'T', 'h', 'd'}
			out = binary.BigEndian.AppendUint32(out, 6)
			out = binary.BigEndian.AppendUint16(out, 0)
			out = binary.BigEndian.AppendUint16(out, 1)
			out = binary.BigEndian.AppendUint16(out, 0xE250)
			return out
		}(), "SMPTE"},
		{"format 2", func() []byte {
			out := []byte{'M', 'T', 'h', 'd'}
			out = binary.BigEndian.AppendUint32(out, 6)
			out = binary.BigEndian.AppendUint16(out, 2)
			out = binary.BigEndian.AppendUint16(out, 1)
			out = binary.BigEndian.AppendUint16(out, 480)
			out = append(out, 'M', 'T', 'r', 'k')
			out = binary.BigEndian.AppendUint32(out, 4)
			out = append(out, 0x00, 0xFF, 0x2F, 0x00)
			return out
		}(), "format 2"},
		{"unclosed note", buildTestSMF(t, []byte{
			0x00, 0x90, 60, 90,
			0x00, 0xFF, 0x2F, 0x00,
		}), "未闭合"},
		{"no tracks", func() []byte {
			out := []byte{'M', 'T', 'h', 'd'}
			out = binary.BigEndian.AppendUint32(out, 6)
			out = binary.BigEndian.AppendUint16(out, 0)
			out = binary.BigEndian.AppendUint16(out, 0)
			out = binary.BigEndian.AppendUint16(out, 480)
			return out
		}(), "无 MTrk"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseMidi(tc.data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseMidi() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestSummarizeMaterialsRecordsErrorWithoutFailing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken.mid"), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	exp := &Experiment{
		SchemaVersion: SchemaVersionV1,
		ID:            "sum",
		Persona:       "p",
		Steps: []Step{{
			ID: "s1", Ask: "a", Expect: "e", Retry: "r", Pass: "p",
			Materials: []Material{{Kind: MaterialKindMidiClip, Title: "m", Path: "broken.mid"}},
		}},
		SourceDir: dir,
	}
	exp.SummarizeMaterials()
	material := exp.Steps[0].Materials[0]
	if material.SummaryError == "" || material.NoteCount != 0 {
		t.Fatalf("summary error not recorded: %+v", material)
	}
	if material.AbsPath != filepath.Join(dir, "broken.mid") {
		t.Fatalf("AbsPath = %q", material.AbsPath)
	}
	// 缺文件同样只记录，不 panic 不阻塞。
	exp2 := &Experiment{
		SchemaVersion: SchemaVersionV1, ID: "sum2", Persona: "p",
		Steps: []Step{{ID: "s", Ask: "a", Expect: "e", Retry: "r", Pass: "p",
			Materials: []Material{{Kind: MaterialKindMidiClip, Title: "m", Path: "missing.mid"}}}},
		SourceDir: dir,
	}
	exp2.SummarizeMaterials()
	if exp2.Steps[0].Materials[0].SummaryError == "" {
		t.Fatal("missing file summary error not recorded")
	}
}

/* -------------------- 装载状态生命周期 -------------------- */

func TestStoreMaterialLoadLifecycle(t *testing.T) {
	store := NewStore()
	exp := mustParse(t, v1ExperimentYAML)
	conversationID := "conv-mat"
	if _, err := store.Start(conversationID, "p.vit", exp); err != nil {
		t.Fatal(err)
	}
	materials, loaded, ok := store.CurrentMaterials(conversationID)
	if !ok || len(materials) != 1 || materials[0].Title != "主题材料" || len(loaded) != 0 {
		t.Fatalf("current materials = %+v loaded = %+v ok = %t", materials, loaded, ok)
	}
	if _, canLoad := store.ReserveMaterial("unknown", "x"); canLoad {
		t.Fatal("reserve on unknown conversation succeeded")
	}
	if _, canLoad := store.ReserveMaterial(conversationID, "不存在"); canLoad {
		t.Fatal("reserve of a title not declared on the current step succeeded")
	}
	// 占位成功；重复占位失败。
	if _, canLoad := store.ReserveMaterial(conversationID, "主题材料"); !canLoad {
		t.Fatal("first reserve failed")
	}
	if _, canLoad := store.ReserveMaterial(conversationID, "主题材料"); canLoad {
		t.Fatal("double reserve succeeded")
	}
	loaded = store.LoadedMaterials(conversationID)
	if !loaded["主题材料"].Loading || loaded["主题材料"].Loaded {
		t.Fatalf("reserved state = %+v", loaded["主题材料"])
	}
	// 失败收尾：Loading 清除、Error 记录。
	if !store.FinalizeMaterial(conversationID, "主题材料", MaterialLoadState{Error: "kernel boom"}) {
		t.Fatal("finalize failed")
	}
	state := store.LoadedMaterials(conversationID)["主题材料"]
	if state.Loading || state.Loaded || state.Error != "kernel boom" {
		t.Fatalf("state after failed finalize = %+v", state)
	}
	// 失败后可重试：重新占位 → 成功装载。
	if _, canLoad := store.ReserveMaterial(conversationID, "主题材料"); !canLoad {
		t.Fatal("reserve after failure should succeed")
	}
	if !store.FinalizeMaterial(conversationID, "主题材料", MaterialLoadState{Loaded: true, TrackID: "t1", ClipID: "c1"}) {
		t.Fatal("success finalize failed")
	}
	if _, canLoad := store.ReserveMaterial(conversationID, "主题材料"); canLoad {
		t.Fatal("reserve after success succeeded")
	}
	state = store.LoadedMaterials(conversationID)["主题材料"]
	if !state.Loaded || state.TrackID != "t1" || state.ClipID != "c1" {
		t.Fatalf("final state = %+v", state)
	}
	// 装载态进 Verdict（答案推进时同步带出）。
	verdict, _, ok := store.Submit(conversationID, "42")
	if !ok || !verdict.Passed {
		t.Fatalf("verdict = %+v", verdict)
	}
	if got := verdict.MaterialsLoaded["主题材料"]; !got.Loaded || got.ClipID != "c1" {
		t.Fatalf("verdict materials loaded = %+v", got)
	}
	// 会话表深拷贝：外部改动不影响会话。
	verdict.MaterialsLoaded["主题材料"] = MaterialLoadState{Loaded: false}
	if store.LoadedMaterials(conversationID)["主题材料"].Loaded != true {
		t.Fatal("verdict map aliases session state")
	}
}

func TestLoadFileSetsSourceDir(t *testing.T) {
	path := filepath.Join("..", "..", "..", "coord", "experiments", "tutor-sim-2", "tutor_sim.yaml")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	exp, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if exp.SourceDir != filepath.Dir(abs) {
		t.Fatalf("SourceDir = %q, want %q", exp.SourceDir, filepath.Dir(abs))
	}
	if _, err := os.Stat(filepath.Join(exp.SourceDir, "assets", "theme_twinkle_v0.mid")); err != nil {
		t.Fatalf("device asset missing: %v", err)
	}
}
