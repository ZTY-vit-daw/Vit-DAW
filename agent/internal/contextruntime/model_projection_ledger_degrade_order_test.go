package contextruntime

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

// D1-DISCLOSURE-FORENSICS-1 regression (verifyrun1 form): the observation
// ledger is the model's only durable cross-turn observation memory (message
// pairs are [system, rebuilt-user] with no assistant history), and during
// active observation it is chronically the largest warm section — the
// largest-first degradation stubbed it to a cold ref exactly when the
// proposal face needed the disclosed track bindings, leaving no visible track
// id for the proposal target. Under warm pressure the durable evidence memory
// must degrade last: process metadata yields first, and the ledger survives
// with its view rows (view keys, observation ids, track targets, conclusions).
func ledgerDegradeOrderSnapshot(t *testing.T) (Snapshot, []planner.TraceEvent, map[string]any) {
	t.Helper()
	ccb := testCCBBundleResult("ledger-order-call", "ledger-order-fact", "ledger-order-raw")
	trace := []planner.TraceEvent{{Kind: "tool_result", ToolResult: &ccb}}
	full := Build(Input{GoalTrace: trace}, fixedOptions())
	// Warm pressure mirroring the live failure shape: a growing goal trace
	// summary alongside a ledger that grew with every fresh observation into
	// the largest warm section.
	full.GoalTraceSummary = map[string]any{"entries": strings.Repeat("trace entry ", 420)}
	ledger := map[string]any{
		"schema_version": "free_state_observation_ledger.v1",
		"window_round":   3, "receipt_count": 3, "view_observation_count": 3,
		"available_views": map[string]any{
			"project.structure": map[string]any{
				"view_id": "project.structure", "status": "ready", "round": 3,
				"observation_id": "obs-struct", "target_ref": map[string]any{"kind": "project"},
				"conclusion": map[string]any{"facts": map[string]any{
					"tracks": []any{
						map[string]any{"track_id": "1007", "name": "bass"},
						map[string]any{"track_id": "1017", "name": "guitar"},
					},
				}},
			},
			"track:1017::track.stereo_space": map[string]any{
				"view_id": "track.stereo_space", "status": "ready", "round": 3,
				"observation_id": "obs-stereo", "target_ref": map[string]any{"kind": "track", "id": "1017"},
				"conclusion": map[string]any{"facts": map[string]any{"stereo": map[string]any{"correlation_estimate": 0.227}}},
			},
		},
		"receipts": []any{map[string]any{
			"receipt_id": "ccbr-obs-stereo", "observation_id": "obs-stereo", "status": "ready", "round": 3,
		}},
	}
	return full, trace, ledger
}

func TestObservationLedgerDegradesLastUnderWarmPressure(t *testing.T) {
	full, trace, ledger := ledgerDegradeOrderSnapshot(t)
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, ObservationLedger: ledger, Profile: ModelContextProfileSelection,
	}, fixedOptions())

	if overflow := ModelContextOverflow(ModelJSON(model)); overflow != "" {
		t.Fatalf("degraded projection must not report overflow: %s", overflow)
	}
	ledgerSection := modelMap(model["observation_ledger"])
	if len(ledgerSection) <= 1 {
		t.Fatalf("the observation ledger must survive warm pressure instead of degrading to a ref: %#v", ledgerSection)
	}
	views := modelMap(ledgerSection["available_views"])
	structView := modelMap(views["project.structure"])
	if modelText(structView["observation_id"]) != "obs-struct" {
		t.Fatalf("ledger view bindings lost: %#v", structView)
	}
	body := ModelJSON(model)
	if !strings.Contains(body, "\"1017\"") {
		t.Fatalf("the ledger's track bindings must stay visible to the model: %s", body)
	}
	// The process metadata degraded instead (a marker names the section that
	// yielded).
	markers := modelRows(model["compaction_markers"])
	if len(markers) == 0 {
		t.Fatalf("warm pressure must degrade process metadata with a marker: %#v", markers)
	}
	for _, marker := range markers {
		if modelText(marker["section"]) == "observation_ledger" {
			t.Fatalf("the ledger must degrade last, but a marker degraded it while metadata survived: %#v", markers)
		}
	}
}

func TestObservationLedgerStillDegradesWhenAloneOverBudget(t *testing.T) {
	full, trace, _ := ledgerDegradeOrderSnapshot(t)
	// Only the ledger in the warm layer, far beyond the 5KB budget on its own:
	// the bounded fail-closed face stays — the ledger degrades to a ref rather
	// than overflowing or silently truncating.
	hugeViews := map[string]any{}
	for i := 0; i < 24; i++ {
		hugeViews[string(rune('a'+i))] = map[string]any{
			"view_id": string(rune('a'+i)), "status": "ready", "round": 4,
			"observation_id": "obs-huge", "target_ref": map[string]any{"kind": "track", "id": "10xx"},
			"conclusion": map[string]any{"facts": map[string]any{
				"payload": strings.Repeat("conclusion fact ", 400),
			}},
		}
	}
	ledger := map[string]any{
		"schema_version": "free_state_observation_ledger.v1", "window_round": 4,
		"available_views": hugeViews,
	}
	model := ProjectModelSnapshot(ModelProjectionInput{
		Snapshot: full, GoalTrace: trace, ObservationLedger: ledger, Profile: ModelContextProfileSelection,
	}, fixedOptions())
	ledgerSection := modelMap(model["observation_ledger"])
	if modelText(ledgerSection["ref"]) == "" {
		t.Fatalf("a lone oversized ledger must still degrade to a cold ref: %#v", ledgerSection)
	}
}
