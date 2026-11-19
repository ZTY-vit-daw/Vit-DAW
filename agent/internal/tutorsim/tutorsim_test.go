package tutorsim

import (
	"errors"
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
		{name: "wrong schema", yaml: replace("vit.tutor_sim.v0", "vit.tutor_sim.v1"), want: "schema_version"},
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
