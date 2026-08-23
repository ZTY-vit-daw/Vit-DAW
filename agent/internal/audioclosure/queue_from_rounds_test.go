package audioclosure

import (
	"strings"
	"testing"
	"time"
)

// Queue derivation from persisted diagnostic rounds plus the FS9 stop-reason
// enumeration negative (docs/FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md).

func TestQueueFromRoundsClosesSkipsAndLeavesPartialOpen(t *testing.T) {
	readyRound := DiagnosticRoundRecord{
		SchemaVersion: DiagnosticRoundSchema, RoundID: "r_qfr00000001",
		PrimaryDimension: DimensionLevelHeadroom, PriorityReason: PriorityDefaultOrder,
		ViewsRequested: []string{"track.basic_energy"},
		EvidenceStatus: RoundEvidenceReady, ProjectRevision: "rev-7",
		SkippedDimensions: []SkippedDimension{{Dimension: DimensionStereoSpace, Reason: SkipNotApplicable}},
	}
	partialRound := DiagnosticRoundRecord{
		SchemaVersion: DiagnosticRoundSchema, RoundID: "r_qfr00000002",
		PrimaryDimension: DimensionFrequencyOccupancy, PriorityReason: PriorityDefaultOrder,
		ViewsRequested: []string{"track.timbre_frequency"},
		EvidenceStatus: RoundEvidencePartial, ProjectRevision: "rev-7",
	}
	openRound := DiagnosticRoundRecord{
		SchemaVersion: DiagnosticRoundSchema, RoundID: "r_qfr00000003",
		PrimaryDimension: DimensionDynamics, PriorityReason: PriorityDefaultOrder,
		ViewsRequested: []string{"track.time_dynamics"},
		EvidenceStatus: RoundEvidenceReady, ProjectRevision: "rev-7",
		UnresolvedQuestions: []string{"which stage compresses"},
	}
	// An invalid record must not corrupt the derived queue.
	badRound := DiagnosticRoundRecord{SchemaVersion: DiagnosticRoundSchema, RoundID: "r_qfr00000004"}

	queue := QueueFromRounds([]DiagnosticRoundRecord{readyRound, partialRound, openRound, badRound})
	if err := queue.Validate(); err != nil {
		t.Fatalf("derived queue invalid: %v", err)
	}
	status := map[DiagnosticDimension]QueueEntryStatus{}
	skip := map[DiagnosticDimension]SkipReason{}
	for _, entry := range queue.Entries {
		status[entry.Dimension] = entry.Status
		skip[entry.Dimension] = entry.SkipReason
	}
	if status[DimensionLevelHeadroom] != QueueClosed {
		t.Fatalf("ready round without unresolved questions left %s open", DimensionLevelHeadroom)
	}
	if status[DimensionStereoSpace] != QueueSkipped || skip[DimensionStereoSpace] != SkipNotApplicable {
		t.Fatalf("recorded skip not applied: %s/%s", status[DimensionStereoSpace], skip[DimensionStereoSpace])
	}
	if status[DimensionFrequencyOccupancy] != QueueOpen {
		t.Fatalf("partial round closed %s", DimensionFrequencyOccupancy)
	}
	if status[DimensionDynamics] != QueueOpen {
		t.Fatalf("round with unresolved questions closed %s", DimensionDynamics)
	}
	if !queue.HasOpen() {
		t.Fatal("queue with open dimensions reported exhausted")
	}
}

func TestQueueFromRoundsExhaustedWhenAllDimensionsClosed(t *testing.T) {
	var rounds []DiagnosticRoundRecord
	for i, dim := range DefaultDimensionOrder {
		rounds = append(rounds, DiagnosticRoundRecord{
			SchemaVersion: DiagnosticRoundSchema, RoundID: "r_qfx" + strings.Repeat("0", 8) + string(rune('a'+i)),
			PrimaryDimension: dim, PriorityReason: PriorityDefaultOrder,
			ViewsRequested: ValidDimensionViews(dim)[:1],
			EvidenceStatus: RoundEvidenceReady, ProjectRevision: "rev-7",
		})
	}
	queue := QueueFromRounds(rounds)
	if err := queue.Validate(); err != nil {
		t.Fatalf("exhausted queue invalid: %v", err)
	}
	if queue.HasOpen() {
		t.Fatal("all dimensions closed but queue still open")
	}
}

func TestFS9SettleRejectsStopReasonOutsideEnumeration(t *testing.T) {
	state, err := Start(StartRequest{
		ClosureID: "closure-fs9neg", ConversationID: "conversation-fs9neg",
		ProjectUUID: "proj-fs9neg", OriginalIntent: "inspect the mix", Mode: ModeDiagnostic,
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if _, err := (Driver{}).Settle(state, state.Revision, StopReason("everything_is_fine_now"), "fabricated terminal", false, time.Now().UTC()); err == nil {
		t.Fatal("stop reason outside the enumeration settled the closure")
	}
	if _, err := (Driver{}).Settle(state, state.Revision, StopNoCandidateFound, "bounded diagnostic", false, time.Now().UTC()); err != nil {
		t.Fatalf("enum stop reason rejected: %v", err)
	}
}
