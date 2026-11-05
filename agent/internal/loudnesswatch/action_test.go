package loudnesswatch

import (
	"math"
	"strings"
	"testing"
)

func suggestionRows() []map[string]any {
	return []map[string]any{
		{"track_id": "trk_a", "clip_id": "clip_1", "integrated_lufs": -8.2, "sample_peak_dbfs": -0.2},
		{"track_id": "trk_b", "clip_id": "clip_2", "integrated_lufs": -15.0, "sample_peak_dbfs": -3.0},
	}
}

func TestSuggestGainActionReturnsNilForNonBreachVerdicts(t *testing.T) {
	rows := suggestionRows()
	for _, verdictKind := range []string{VerdictPass, VerdictUnmeasurable, ""} {
		verdict := Verdict{SchemaVersion: VerdictSchemaVersion, Verdict: verdictKind, Summary: "s"}
		if got := SuggestGainAction(verdict, rows, 10); got != nil {
			t.Fatalf("verdict %q: expected nil suggestion, got %+v", verdictKind, got)
		}
	}
}

func TestSuggestGainActionTargetsWorstBreachTrack(t *testing.T) {
	// clip_1 breaches both dimensions; integrated is the larger signed breach.
	verdict := Verdict{
		SchemaVersion: VerdictSchemaVersion, Verdict: VerdictBreach,
		BreachDetails: []BreachDetail{
			{ClipID: "clip_1", Metric: "integrated_lufs", Measured: -8.2, Threshold: -14, DeltaDB: 5.8},
			{ClipID: "clip_1", Metric: "sample_peak_dbfs", Measured: -0.2, Threshold: -1, DeltaDB: 0.8},
		},
	}
	got := SuggestGainAction(verdict, suggestionRows(), 10)
	if got == nil {
		t.Fatal("expected a suggestion for a breach verdict")
	}
	if got.Operation != ActionOperationGainAdjust || got.TrackID != "trk_a" || got.ClipID != "clip_1" {
		t.Fatalf("unexpected target: %+v", got)
	}
	if got.Metric != "integrated_lufs" {
		t.Fatalf("expected worst-breach metric integrated_lufs, got %s", got.Metric)
	}
	if math.Abs(got.DeltaDB-(-5.8)) > 1e-9 {
		t.Fatalf("expected delta -5.8 (negated breach), got %v", got.DeltaDB)
	}
	if !strings.Contains(got.Basis, "近似口径") || !strings.Contains(got.Basis, "待确认后执行") {
		t.Fatalf("basis must carry the approximate qualifier and the confirmation boundary: %q", got.Basis)
	}
}

func TestSuggestGainActionClampsDeltaToCallerBound(t *testing.T) {
	verdict := Verdict{
		SchemaVersion: VerdictSchemaVersion, Verdict: VerdictBreach,
		BreachDetails: []BreachDetail{{ClipID: "clip_1", Metric: "integrated_lufs", Measured: -30, Threshold: -26, DeltaDB: -4}},
	}
	// Floor breach: the fix direction is positive (push up), clamped to +2.
	got := SuggestGainAction(verdict, suggestionRows(), 2)
	if got == nil {
		t.Fatal("expected a clamped push-up suggestion")
	}
	if got.DeltaDB != 2 {
		t.Fatalf("expected clamped delta +2, got %v", got.DeltaDB)
	}
	// A bound of zero disables the action face entirely.
	if got := SuggestGainAction(verdict, suggestionRows(), 0); got != nil {
		t.Fatalf("expected nil with zero dose bound, got %+v", got)
	}
}

func TestSuggestGainActionDropsUnresolvableTrack(t *testing.T) {
	verdict := Verdict{
		SchemaVersion: VerdictSchemaVersion, Verdict: VerdictBreach,
		BreachDetails: []BreachDetail{{ClipID: "clip_unknown", Metric: "integrated_lufs", Measured: -8, Threshold: -14, DeltaDB: 6}},
	}
	if got := SuggestGainAction(verdict, suggestionRows(), 10); got != nil {
		t.Fatalf("expected nil for a clip with no resolvable track, got %+v", got)
	}
	// Empty clip id is equally unaddressable.
	verdict.BreachDetails[0].ClipID = ""
	if got := SuggestGainAction(verdict, suggestionRows(), 10); got != nil {
		t.Fatalf("expected nil for an empty clip id, got %+v", got)
	}
}

func TestSuggestGainActionZeroAfterClampMeansNoSuggestion(t *testing.T) {
	// A breach delta smaller than any representable clamp step still yields a
	// non-zero suggestion; the zero case is the empty detail list.
	verdict := Verdict{SchemaVersion: VerdictSchemaVersion, Verdict: VerdictBreach}
	if got := SuggestGainAction(verdict, suggestionRows(), 10); got != nil {
		t.Fatalf("expected nil for a breach with no details, got %+v", got)
	}
}
