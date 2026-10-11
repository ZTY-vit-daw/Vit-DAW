package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/projectstore"
)

func freeStateConclusionStoreFixture(now time.Time) FreeStateConclusionRecord {
	return FreeStateConclusionRecord{
		ConclusionID:    "fsc_exp-1_experiment_settle",
		ProjectUUID:     "proj-m1",
		ConversationID:  "conv-a",
		GoalID:          "goal-1",
		RunID:           "run-1",
		LoopID:          "loop-1",
		ExperimentID:    "exp-1",
		Kind:            FreeStateConclusionKindExperimentSettle,
		Outcome:         "improved",
		DecisionStatus:  "completed",
		Summary:         "user preferred B; treatment candidate retained",
		CheckpointRef:   "ckpt-9",
		ProjectRevision: "rev-12",
		Judgment: &FreeStateJudgmentOutcome{
			EvidenceID: "evid-1", AuditionSessionID: "aud-1",
			Preference: "b", HeardDifference: "yes",
		},
		SettledAt: now.Add(-time.Minute),
	}
}

func TestFreeStateConclusionRecordRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return now }

	record, err := store.RecordFreeStateConclusion(freeStateConclusionStoreFixture(now))
	if err != nil {
		t.Fatalf("RecordFreeStateConclusion: %v", err)
	}
	if record.ConclusionID == "" {
		t.Fatal("conclusion id not assigned")
	}
	if !record.RecordedAt.Equal(now) {
		t.Fatalf("recorded at = %v, want %v", record.RecordedAt, now)
	}

	// A second store instance over the same root models a later session:
	// the conclusion must read back with its terminal facts intact.
	board, err := NewStore(store.Root).ReadFreeStateConclusionBoard("proj-m1")
	if err != nil {
		t.Fatalf("ReadFreeStateConclusionBoard: %v", err)
	}
	if board.ConclusionCount != 1 || len(board.Conclusions) != 1 {
		t.Fatalf("board count = %d, want 1 (%+v)", board.ConclusionCount, board)
	}
	view := board.Conclusions[0]
	if view.ConclusionID != record.ConclusionID || view.Kind != FreeStateConclusionKindExperimentSettle ||
		view.Outcome != "improved" || view.Summary != record.Summary ||
		view.ConversationID != "conv-a" || view.ExperimentID != "exp-1" || view.CheckpointRef != "ckpt-9" {
		t.Fatalf("round-trip view mismatch: %+v", view)
	}
	if view.Judgment == nil || view.Judgment.EvidenceID != "evid-1" || view.Judgment.Preference != "b" {
		t.Fatalf("judgment projection mismatch: %+v", view.Judgment)
	}
	if !view.SettledAt.Equal(record.SettledAt) {
		t.Fatalf("settled at = %v, want %v", view.SettledAt, record.SettledAt)
	}
}

func TestFreeStateConclusionRecordIdempotentRewrite(t *testing.T) {
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	root := t.TempDir()
	store := NewStore(root)
	store.Now = func() time.Time { return now }
	first, err := store.RecordFreeStateConclusion(freeStateConclusionStoreFixture(now))
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Same immutable content replayed later (a second settle funnel seeing the
	// same terminal state) keeps the original RecordedAt instead of erroring.
	// The replay shares the terminal fact — same settled-at — only the wall
	// clock (Now) moved.
	later := now.Add(time.Hour)
	store.Now = func() time.Time { return later }
	replay := freeStateConclusionStoreFixture(later)
	replay.SettledAt = first.SettledAt
	second, err := store.RecordFreeStateConclusion(replay)
	if err != nil {
		t.Fatalf("idempotent rewrite: %v", err)
	}
	if !second.RecordedAt.Equal(first.RecordedAt) {
		t.Fatalf("recorded at drifted on replay: %v != %v", second.RecordedAt, first.RecordedAt)
	}

	// Same ConclusionID with different content is refused.
	conflict := freeStateConclusionStoreFixture(later)
	conflict.SettledAt = first.SettledAt
	conflict.Outcome = "rolled_back"
	if _, err := store.RecordFreeStateConclusion(conflict); err == nil {
		t.Fatal("conflicting rewrite accepted")
	}
}

func TestFreeStateConclusionBoardOldProjectDefault(t *testing.T) {
	store := NewStore(t.TempDir())
	board, err := store.ReadFreeStateConclusionBoard("proj-never")
	if err != nil {
		t.Fatalf("old project read must not error: %v", err)
	}
	if board.ConclusionCount != 0 || len(board.Conclusions) != 0 {
		t.Fatalf("old project board not empty: %+v", board)
	}
	if board.ProjectUUID != "proj-never" {
		t.Fatalf("board project uuid = %q", board.ProjectUUID)
	}
}

func TestFreeStateConclusionOldRecordDeserializes(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	dir := filepath.Join(store.projectDecisionDir("proj-old"), "free_state_conclusions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A record written by a earlier revision of this schema (no judgment, no
	// checkpoint/revision, no decision status) and carrying an outcome value
	// this build has no wording for: it must deserialize and display verbatim.
	legacy := map[string]any{
		"schema_version":  FreeStateConclusionRecordSchemaVersion,
		"conclusion_id":   "fsc_exp-legacy_experiment_settle",
		"project_uuid":    "proj-old",
		"conversation_id": "conv-legacy",
		"kind":            FreeStateConclusionKindExperimentSettle,
		"outcome":         "some_future_outcome",
		"summary":         "settled by an older build",
		"settled_at":      "2026-10-10T08:00:00Z",
		"recorded_at":     "2026-10-10T08:00:00Z",
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fsc_exp-legacy_experiment_settle.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	board, err := NewStore(root).ReadFreeStateConclusionBoard("proj-old")
	if err != nil {
		t.Fatalf("legacy record read: %v", err)
	}
	if board.ConclusionCount != 1 {
		t.Fatalf("legacy conclusion dropped: %+v", board)
	}
	view := board.Conclusions[0]
	if view.Outcome != "some_future_outcome" || view.Summary != "settled by an older build" {
		t.Fatalf("legacy values not preserved verbatim: %+v", view)
	}
	if view.Judgment != nil || view.CheckpointRef != "" {
		t.Fatalf("legacy optional fields not defaulted: %+v", view)
	}
}

func TestFreeStateConclusionBoardFileSkippedAsRecord(t *testing.T) {
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return now }
	if _, err := store.RecordFreeStateConclusion(freeStateConclusionStoreFixture(now)); err != nil {
		t.Fatalf("write: %v", err)
	}
	board, err := store.ReadFreeStateConclusionBoard("proj-m1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if board.ConclusionCount != 1 {
		t.Fatalf("board file double-counted as a record: %+v", board)
	}
}

func TestFreeStateConclusionValidation(t *testing.T) {
	store := NewStore(t.TempDir())
	missingUUID := freeStateConclusionStoreFixture(time.Now())
	missingUUID.ProjectUUID = ""
	if _, err := store.RecordFreeStateConclusion(missingUUID); err == nil || !strings.Contains(err.Error(), "project uuid") {
		t.Fatalf("missing project uuid not rejected: %v", err)
	}
	missingKind := freeStateConclusionStoreFixture(time.Now())
	missingKind.Kind = "bogus"
	if _, err := store.RecordFreeStateConclusion(missingKind); err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("unknown kind not rejected: %v", err)
	}
	missingOutcome := freeStateConclusionStoreFixture(time.Now())
	missingOutcome.Outcome = ""
	if _, err := store.RecordFreeStateConclusion(missingOutcome); err == nil || !strings.Contains(err.Error(), "outcome") {
		t.Fatalf("missing outcome not rejected: %v", err)
	}
}

func TestFreeStateConclusionReadRequiresUUIDWhenUnbound(t *testing.T) {
	projectstore.Deactivate()
	store := NewStore(t.TempDir())
	if _, err := store.ReadFreeStateConclusionBoard("  "); err == nil || !strings.Contains(err.Error(), "project uuid is required") {
		t.Fatalf("empty uuid read not rejected: %v", err)
	}
}
