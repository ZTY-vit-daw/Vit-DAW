package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MIXLAYER-M1-5 unit coverage: the mix_report.v1 projection folds the
// free-state settle conclusion board (M1-1 persistence layer, consumed
// read-only) into the report timeline — newest-conclusion selection,
// old-project empty-board default, unreadable-board fail-open limitation,
// and the AGENTS §11 old-JSON deserialization default for the new fields.

func m1FiveConclusionRecord(id, projectUUID, outcome string, settledAt time.Time) FreeStateConclusionRecord {
	return FreeStateConclusionRecord{
		ConclusionID: id, ProjectUUID: projectUUID, ConversationID: "conv-" + id,
		Kind: FreeStateConclusionKindExperimentSettle, Outcome: outcome,
		Summary: "m1-5 test settle", SettledAt: settledAt,
	}
}

func TestMixReportFoldsFreeStateConclusionTimeline(t *testing.T) {
	store := NewStore(t.TempDir())
	base := time.Unix(1_700_000_000, 0).UTC()
	if _, err := store.RecordFreeStateConclusion(m1FiveConclusionRecord("fsc-1", "proj-m1-5", "rolled_back", base)); err != nil {
		t.Fatalf("record first conclusion: %v", err)
	}
	if _, err := store.RecordFreeStateConclusion(m1FiveConclusionRecord("fsc-2", "proj-m1-5", "improved", base.Add(time.Hour))); err != nil {
		t.Fatalf("record second conclusion: %v", err)
	}
	report, err := store.BuildMixReport(MixReportRequest{ProjectUUID: "proj-m1-5"})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if len(report.FreeStateConclusionTimeline) != 2 {
		t.Fatalf("conclusion timeline = %#v, want 2 entries", report.FreeStateConclusionTimeline)
	}
	if report.FreeStateConclusionTimeline[0].ConclusionID != "fsc-1" || report.FreeStateConclusionTimeline[1].ConclusionID != "fsc-2" {
		t.Fatalf("conclusion timeline not SettledAt-ascending: %#v", report.FreeStateConclusionTimeline)
	}
	if report.LatestSettleConclusion == nil || report.LatestSettleConclusion.ConclusionID != "fsc-2" {
		t.Fatalf("latest settle conclusion = %#v, want fsc-2 (newest by SettledAt)", report.LatestSettleConclusion)
	}
	if report.LatestSettleConclusion.Outcome != "improved" {
		t.Fatalf("latest outcome = %q, want improved", report.LatestSettleConclusion.Outcome)
	}
	for _, limitation := range report.Limitations {
		if strings.Contains(limitation, "free_state_conclusions") {
			t.Fatalf("unexpected conclusion limitation: %#v", report.Limitations)
		}
	}
}

func TestMixReportOldProjectEmptyConclusionDefault(t *testing.T) {
	store := NewStore(t.TempDir())
	if _, err := store.RecordCapabilitySession(terminalMixDecisionSession("b2", "static_mix.static_balance.v0", "cut", time.Now().UTC())); err != nil {
		t.Fatal(err)
	}
	report, err := store.BuildMixReport(MixReportRequest{ProjectUUID: "proj-m1-5-old"})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	if len(report.FreeStateConclusionTimeline) != 0 || report.LatestSettleConclusion != nil {
		t.Fatalf("old-project conclusions = %#v / %#v, want empty/nil", report.FreeStateConclusionTimeline, report.LatestSettleConclusion)
	}
	for _, limitation := range report.Limitations {
		if strings.Contains(limitation, "free_state_conclusions") {
			t.Fatalf("empty board must not read as a limitation: %#v", report.Limitations)
		}
	}
}

func TestMixReportUnreadableConclusionBoardFailsOpen(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if _, err := store.RecordFreeStateConclusion(m1FiveConclusionRecord("fsc-good", "proj-m1-5-bad", "improved", time.Unix(1_700_000_000, 0).UTC())); err != nil {
		t.Fatalf("seed conclusion: %v", err)
	}
	dir := store.freeStateConclusionDir("proj-m1-5-bad")
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("plant malformed record: %v", err)
	}
	report, err := NewStore(root).BuildMixReport(MixReportRequest{ProjectUUID: "proj-m1-5-bad"})
	if err != nil {
		t.Fatalf("unreadable conclusion board must fail open, got error: %v", err)
	}
	if len(report.FreeStateConclusionTimeline) != 0 || report.LatestSettleConclusion != nil {
		t.Fatalf("unreadable board must not leak partial conclusions: %#v", report.FreeStateConclusionTimeline)
	}
	found := false
	for _, limitation := range report.Limitations {
		if limitation == "free_state_conclusions_unreadable" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unreadable board must record the honest limitation: %#v", report.Limitations)
	}
}

func TestMixReportOldJSONRoundTripsNewFields(t *testing.T) {
	// AGENTS §11: a pre-M1-5 report JSON (no conclusion fields) must
	// deserialize into the extended struct with the documented defaults.
	oldJSON := `{"schema_version":"mix_report.v1","project_uuid":"proj-old","generated_at":"2026-01-01T00:00:00Z","capability_summary":[],"decision_timeline":[],"verification_summary":{},"export_readiness":"not_assessed"}`
	var report MixReport
	if err := json.Unmarshal([]byte(oldJSON), &report); err != nil {
		t.Fatalf("old report JSON rejected: %v", err)
	}
	if len(report.FreeStateConclusionTimeline) != 0 || report.LatestSettleConclusion != nil {
		t.Fatalf("old JSON must default conclusions to empty/nil: %#v / %#v", report.FreeStateConclusionTimeline, report.LatestSettleConclusion)
	}

	// New report JSON round-trips the conclusion fields verbatim.
	store := NewStore(t.TempDir())
	settled := time.Unix(1_700_000_100, 0).UTC()
	if _, err := store.RecordFreeStateConclusion(m1FiveConclusionRecord("fsc-rt", "proj-m1-5-rt", "improved", settled)); err != nil {
		t.Fatalf("seed conclusion: %v", err)
	}
	fresh, err := store.BuildMixReport(MixReportRequest{ProjectUUID: "proj-m1-5-rt"})
	if err != nil {
		t.Fatalf("build report: %v", err)
	}
	data, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var reloaded MixReport
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if reloaded.LatestSettleConclusion == nil || reloaded.LatestSettleConclusion.ConclusionID != "fsc-rt" {
		t.Fatalf("round-trip lost the latest settle conclusion: %#v", reloaded.LatestSettleConclusion)
	}
	if !reloaded.LatestSettleConclusion.SettledAt.Equal(settled) {
		t.Fatalf("round-trip settled_at = %v, want %v", reloaded.LatestSettleConclusion.SettledAt, settled)
	}
}
