package contextruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

// TestDumpContextGraphContractFixtures writes deterministic contract fixtures
// consumed by scripts/context_graph_v1_contract_check.py. It is a no-op unless
// DUMP_CONTEXT_FIXTURES=1 so the checked-in fixtures stay stable and the
// regular test run never touches the tree.
func TestDumpContextGraphContractFixtures(t *testing.T) {
	if os.Getenv("DUMP_CONTEXT_FIXTURES") != "1" {
		t.Skip("set DUMP_CONTEXT_FIXTURES=1 to regenerate contract fixtures")
	}
	opts := fixedOptions()
	outDir := filepath.Join("..", "..", "..", "temp", "context_graph_v1")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Round-12 projection: multi-round trace + delta-only ledger.
	trace := make([]planner.TraceEvent, 0, 12)
	for i := 1; i <= 12; i++ {
		ccb := testCCBBundleResult(fmt.Sprintf("ccb-%02d", i), fmt.Sprintf("fact-%02d", i), "raw-audit")
		trace = append(trace, planner.TraceEvent{Kind: "tool_result", ToolResult: &ccb})
	}
	receipts := make([]any, 0, 12)
	for i := 1; i <= 12; i++ {
		receipts = append(receipts, map[string]any{
			"receipt_id": fmt.Sprintf("receipt-%02d", i), "tool_call_id": fmt.Sprintf("ccb-%02d", i),
			"observation_id": fmt.Sprintf("obs-%02d", i), "status": "ready", "round": i,
		})
	}
	ledger := map[string]any{
		"schema_version": "free_state_observation_ledger.v1",
		"window_round":   12, "receipt_count": 12, "receipts": receipts,
	}
	full := Build(Input{GoalTrace: trace}, opts)
	roundModel := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, ObservationLedger: ledger, Profile: ModelContextProfileSelection,
	}, opts)
	writeFixture(t, outDir, "model_projection_round12.json", ModelJSON(roundModel))

	// Degraded projection: supporting hot section over budget, degraded to ref.
	fullDegrade := Build(Input{GoalTrace: trace[:1]}, opts)
	fullDegrade.DAWStateSummary = map[string]any{"state": strings.Repeat("wide daw state payload ", 4000)}
	degradedModel := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: fullDegrade, GoalTrace: trace[:1], Profile: ModelContextProfileSelection,
	}, opts)
	writeFixture(t, outDir, "model_projection_degraded.json", ModelJSON(degradedModel))

	// Overflow projection: blocking hot section over budget, fail closed.
	fullOverflow := Build(Input{GoalTrace: trace[:1]}, opts)
	fullOverflow.CurrentSelection = map[string]any{"payload": strings.Repeat("current selection cannot be degraded ", 4000)}
	overflowModel := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: fullOverflow, GoalTrace: trace[:1], Profile: ModelContextProfileSelection,
	}, opts)
	writeFixture(t, outDir, "model_projection_overflow.json", ModelJSON(overflowModel))
}

func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
