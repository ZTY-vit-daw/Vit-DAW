package contextruntime

// exit_hook_consumption_test.go — L1-4-IMPL-D D1 腿 1：挂点生产消费面
// （retain 决策落盘 / advisory 形态 / 写失败可见性）。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/llm"
)

func retentionTestUnits(turnID string) []WindowUnit {
	return []WindowUnit{
		{
			Unit:              ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs-1"},
			TurnID:            turnID,
			RetainedStatement: "conclusion A",
			EvidenceRefs:      []string{"track://drums"},
			Bytes:             42,
		},
		{
			Unit: ExitUnit{Kind: ExitUnitToolResult, ID: "tool-7"},
			// 无结论无证据 → drop（不变式无负担）。
			TurnID: turnID,
			Bytes:  10,
		},
	}
}

func TestRunTurnBoundaryHookConsumesRetains(t *testing.T) {
	dir := t.TempDir()
	result := RunTurnBoundaryHook(context.Background(), TurnBoundaryHookInput{
		TurnID:     "run-1",
		ExtraUnits: retentionTestUnits("run-1"),
		ProjectDir: dir,
	})
	if result.RetainsWritten != 1 {
		t.Fatalf("RetainsWritten = %d, want 1 (single conclusion-bearing unit)", result.RetainsWritten)
	}
	retains, drops := 0, 0
	for _, decision := range result.ExitReport.Decisions {
		switch decision.Action {
		case ExitRetain:
			retains++
		case ExitDrop:
			drops++
		}
	}
	if retains != 1 || drops != 1 {
		t.Fatalf("decisions retain/drop = %d/%d, want 1/1", retains, drops)
	}
	entries, err := carriers.ReadLedger(dir)
	if err != nil {
		t.Fatalf("ReadLedger: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ledger entries = %d, want 1", len(entries))
	}
	if entries[0].Kind != carriers.LedgerKindObservationConclusion || entries[0].Statement != "conclusion A" {
		t.Fatalf("entry = %+v, want observation_conclusion/conclusion A", entries[0])
	}
	if len(entries[0].EvidenceRefs) != 1 || entries[0].EvidenceRefs[0] != "track://drums" {
		t.Fatalf("evidence refs = %v, want [track://drums]", entries[0].EvidenceRefs)
	}
	if len(result.ExitViolations) != 0 {
		t.Fatalf("violations = %v, want empty", result.ExitViolations)
	}
}

func TestRunTurnBoundaryHookAdvisoryWithoutProjectDir(t *testing.T) {
	dir := t.TempDir()
	result := RunTurnBoundaryHook(context.Background(), TurnBoundaryHookInput{
		TurnID:     "run-1",
		History:    []llm.Message{{Role: "user", Content: "hi"}},
		ExtraUnits: retentionTestUnits("run-1"),
	})
	if result.RetainsWritten != 0 {
		t.Fatalf("RetainsWritten = %d, want 0 (advisory form)", result.RetainsWritten)
	}
	if _, err := os.Stat(filepath.Join(dir, carriers.LedgerRelPath)); !os.IsNotExist(err) {
		t.Fatalf("ledger file must not exist in advisory form, stat err = %v", err)
	}
	if len(result.ExitReport.Decisions) == 0 {
		t.Fatalf("advisory form still reports decisions (执法面不缺席)")
	}
}

func TestRunTurnBoundaryHookRetainWriteFailureVisible(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	result := RunTurnBoundaryHook(context.Background(), TurnBoundaryHookInput{
		TurnID:     "run-1",
		ExtraUnits: retentionTestUnits("run-1"),
		ProjectDir: blocker,
	})
	visible := false
	for _, violation := range result.ExitViolations {
		if strings.Contains(violation, "retain write failed") {
			visible = true
		}
	}
	if !visible {
		t.Fatalf("retain write failure must surface in ExitViolations, got %v", result.ExitViolations)
	}
}
