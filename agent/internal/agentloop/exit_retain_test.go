package agentloop

// exit_retain_test.go — L1-4-IMPL-D D1 腿 2：run 终态观察结论跨会话延伸
// （观察账本结论行 retain 进工程 L4 账本 + 语句级去重 + advisory 形态）。

import (
	"strings"
	"testing"

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
