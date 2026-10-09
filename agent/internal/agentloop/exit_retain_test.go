package agentloop

// exit_retain_test.go — L1-4-IMPL-D D1 腿 2：run 终态观察结论跨会话延伸
// （观察账本结论行 retain 进工程 L4 账本 + 语句级去重 + advisory 形态）。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/contextruntime/carriers"
	agentruntime "vit-daw-agent/internal/runtime"
)

func exitRetainTestState(projectDir string) *runState {
	context := map[string]any{}
	if projectDir != "" {
		context["project_path"] = projectDir
	}
	context["free_state_reasoning_loop"] = map[string]any{
		"observation_ledger": map[string]any{
			"available_views": map[string]any{
				"view.mix.frequency": map[string]any{
					"status":         "ready",
					"observation_id": "obs-1",
					"target_ref":     "track://drums",
					"conclusion":     "drums bus masking resolved",
				},
				"view.mix.multitrack": map[string]any{
					"status":         "ready",
					"observation_id": "obs-2",
					"target_ref":     "track://bass",
					"conclusion":     "bass level stable after trim",
				},
				"view.no.conclusion": map[string]any{
					"status":         "partial",
					"observation_id": "obs-3",
					"target_ref":     "track://vox",
				},
			},
		},
	}
	return &runState{
		input: Input{Context: context},
		goal:  agentruntime.Goal{RunID: "run-42"},
	}
}

func TestRetainRunObservationConclusionsPersistsLedger(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{}
	note := r.retainRunObservationConclusions(exitRetainTestState(dir))
	if !strings.Contains(note, "2 conclusions retained") {
		t.Fatalf("note = %q, want 2 conclusions retained", note)
	}
	entries, err := carriers.ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ledger entries = %d, want 2 (no-conclusion row skipped)", len(entries))
	}
	// 确定性序：available_views 按键排序 → view.mix.frequency 先于 view.mix.multitrack。
	if entries[0].Statement != "drums bus masking resolved" || entries[1].Statement != "bass level stable after trim" {
		t.Fatalf("entries = [%s, %s], want sorted-key order", entries[0].Statement, entries[1].Statement)
	}
	for _, entry := range entries {
		if entry.Kind != carriers.LedgerKindObservationConclusion {
			t.Fatalf("entry kind = %s, want observation_conclusion", entry.Kind)
		}
		if len(entry.EvidenceRefs) != 1 {
			t.Fatalf("entry %d evidence refs = %v, want the row target_ref", entry.EntryID, entry.EvidenceRefs)
		}
	}
}

func TestRetainRunObservationConclusionsDedup(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{}
	state := exitRetainTestState(dir)
	if note := r.retainRunObservationConclusions(state); !strings.Contains(note, "2 conclusions retained") {
		t.Fatalf("first pass note = %q, want 2 retained", note)
	}
	if note := r.retainRunObservationConclusions(exitRetainTestState(dir)); note != "" {
		t.Fatalf("second pass note = %q, want empty (statement-level dedup)", note)
	}
	entries, err := carriers.ReadLedger(dir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("after re-run entries = %d err = %v, want 2 (idempotent)", len(entries), err)
	}
}

func TestRetainRunObservationConclusionsAdvisoryWithoutProjectDir(t *testing.T) {
	r := &Runner{}
	note := r.retainRunObservationConclusions(exitRetainTestState(""))
	if !strings.Contains(note, "advisory") || !strings.Contains(note, "no project dir") {
		t.Fatalf("note = %q, want advisory form disclosure", note)
	}
}

func TestRetainRunObservationConclusionsEmptyLedger(t *testing.T) {
	r := &Runner{}
	if note := r.retainRunObservationConclusions(&runState{input: Input{Context: map[string]any{"project_path": t.TempDir()}}}); note != "" {
		t.Fatalf("note = %q, want empty for ledger-less run", note)
	}
}

func TestRunProjectDirFromStateKeyOrder(t *testing.T) {
	if dir := runProjectDirFromState(&runState{input: Input{Context: map[string]any{"current_project_path": "D:/p1", "project_path": "D:/p0"}}}); dir != "D:/p0" {
		t.Fatalf("dir = %q, want project_path precedence", dir)
	}
	if dir := runProjectDirFromState(&runState{input: Input{Context: map[string]any{"current_project_path": "D:/p1"}}}); dir != "D:/p1" {
		t.Fatalf("dir = %q, want current_project_path fallback", dir)
	}
	if dir := runProjectDirFromState(nil); dir != "" {
		t.Fatalf("nil state dir = %q, want empty", dir)
	}
}

// REVIEW-1 G-1：去重读失败支——账本 corrupt（含前缀可读段）时 ReadLedger
// 整本 fail-closed 拒读，去重必须全量保留（宁可重复入账不静默丢结论）。
func TestDedupeAgainstProjectLedgerCorruptReadKeepsAll(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ledger"), 0o755); err != nil {
		t.Fatal(err)
	}
	// 首行是合法且语句可匹配的条目——证明整本读失败时连可读前缀也不参与去重。
	corrupt := `{"entry_id":1,"kind":"observation_conclusion","statement":"drums bus masking resolved"}` + "\n" + "not-json\n"
	if err := os.WriteFile(filepath.Join(dir, carriers.LedgerRelPath), []byte(corrupt), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := carriers.ReadLedger(dir); err == nil {
		t.Fatalf("fixture must fail the ledger read (corrupt line present)")
	}
	units := []contextruntime.WindowUnit{
		{Unit: contextruntime.ExitUnit{Kind: contextruntime.ExitUnitObservationBundle, ID: "obs-1"}, RetainedStatement: "drums bus masking resolved"},
		{Unit: contextruntime.ExitUnit{Kind: contextruntime.ExitUnitObservationBundle, ID: "obs-2"}, RetainedStatement: "bass level stable after trim"},
	}
	kept := dedupeAgainstProjectLedger(dir, units)
	if len(kept) != len(units) {
		t.Fatalf("corrupt-ledger dedupe kept %d/%d units, want full retention", len(kept), len(units))
	}
	for i, unit := range kept {
		if unit.RetainedStatement != units[i].RetainedStatement {
			t.Fatalf("kept unit %d statement = %q, want %q (order and content preserved)", i, unit.RetainedStatement, units[i].RetainedStatement)
		}
	}
}

// REVIEW-1 G-4：runner 终态漏斗直测——仅终态（cont=nil 分支）触发 retain；
// 暂停面（pause→result 非终态）不触发；终态 trace 事件附加可见。
func TestRunnerResultFunnelRetainsOnlyOnTerminalStates(t *testing.T) {
	r := &Runner{}

	// 终态面：completed → 账本两条入账 + trace 附加 exit_retain 事件。
	terminalDir := t.TempDir()
	res := r.result(exitRetainTestState(terminalDir), agentruntime.StatusCompleted, "", "", "done", "", "", nil)
	entries, err := carriers.ReadLedger(terminalDir)
	if err != nil {
		t.Fatalf("ReadLedger after terminal result: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("terminal result ledger entries = %d, want 2", len(entries))
	}
	retainEvents := 0
	for _, event := range res.Trace {
		if event.Kind == "exit_retain" {
			retainEvents++
			if !strings.Contains(event.Message, "2 conclusions retained") {
				t.Fatalf("exit_retain trace message = %q, want retain count disclosure", event.Message)
			}
		}
	}
	if retainEvents != 1 {
		t.Fatalf("terminal result trace exit_retain events = %d, want 1", retainEvents)
	}

	// 暂停面：waiting_continue（pause→result，cont 存续）→ 账本零落盘、
	// 零 exit_retain 事件。
	pauseDir := t.TempDir()
	paused := r.pause(exitRetainTestState(pauseDir), agentruntime.StatusWaitingContinue, "", "", "", "", "", nil)
	if entries, err := carriers.ReadLedger(pauseDir); err != nil || len(entries) != 0 {
		t.Fatalf("pause face ledger entries = %d err = %v, want untouched empty ledger", len(entries), err)
	}
	for _, event := range paused.Trace {
		if event.Kind == "exit_retain" {
			t.Fatalf("pause face must not append exit_retain trace events, got %+v", event)
		}
	}
}
