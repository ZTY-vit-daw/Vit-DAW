package agentloop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/mixboard"
)

func blackboardReportProjectStateFixture() map[string]any {
	return map[string]any{
		"status":       "ok",
		"project_uuid": "proj-report",
		"tracks": []map[string]any{
			{"track_id": "t1", "name": "Lead Vox", "clip_count": 3},
		},
	}
}

func TestProjectBlackboardReportShowsFreeStateConclusions(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard", "sessions"))

	settledAt := time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	store := mixboard.NewStore(filepath.Join(root, "mixboard", "sessions"))
	if _, err := store.RecordFreeStateConclusion(mixboard.FreeStateConclusionRecord{
		ProjectUUID: "proj-report", ConversationID: "conv-earlier", ExperimentID: "exp-1",
		ConclusionID: "fsc_exp-1_experiment_settle",
		Kind:         mixboard.FreeStateConclusionKindExperimentSettle, Outcome: "improved",
		Summary:   "user preferred B; treatment candidate retained",
		Judgment:  &mixboard.FreeStateJudgmentOutcome{EvidenceID: "evid-1", Preference: "b"},
		SettledAt: settledAt,
	}); err != nil {
		t.Fatalf("seed conclusion: %v", err)
	}

	report := messageLoopProjectBlackboardReportFromProjectState(nil, map[string]any{
		"project_state": blackboardReportProjectStateFixture(),
	})
	if !strings.Contains(report, "自由态结论（跨会话）") {
		t.Fatalf("cross-session conclusion section missing:\n%s", report)
	}
	if !strings.Contains(report, "已按用户人耳判定保留改动") {
		t.Fatalf("judgment outcome wording missing:\n%s", report)
	}
	if !strings.Contains(report, "user preferred B; treatment candidate retained") {
		t.Fatalf("settle summary missing:\n%s", report)
	}
	if !strings.Contains(report, "free_state_conclusions 工程级持久层") {
		t.Fatalf("data source line missing:\n%s", report)
	}
}

func TestProjectBlackboardReportOldProjectDefaultUnchanged(t *testing.T) {
	// An old project has no conclusion store at all: the report keeps its
	// legacy shape — no section, no data-source line, no error.
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard", "sessions"))

	report := messageLoopProjectBlackboardReportFromProjectState(nil, map[string]any{
		"project_state": blackboardReportProjectStateFixture(),
	})
	if strings.Contains(report, "自由态结论") {
		t.Fatalf("conclusion section leaked into an old project report:\n%s", report)
	}
	if strings.Contains(report, "free_state_conclusions") {
		t.Fatalf("conclusion data source leaked into an old project report:\n%s", report)
	}
	if !strings.Contains(report, "工程黑板状态汇报") {
		t.Fatalf("report lost its base shape:\n%s", report)
	}
}

func TestProjectBlackboardReportUnknownOutcomeShownVerbatim(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard", "sessions"))

	settledAt := time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	store := mixboard.NewStore(filepath.Join(root, "mixboard", "sessions"))
	if _, err := store.RecordFreeStateConclusion(mixboard.FreeStateConclusionRecord{
		ProjectUUID: "proj-report", ConversationID: "conv-earlier", ExperimentID: "exp-2",
		ConclusionID: "fsc_exp-2_experiment_settle",
		Kind:         mixboard.FreeStateConclusionKindExperimentSettle, Outcome: "some_future_outcome",
		Summary: "settled by a newer build", SettledAt: settledAt,
	}); err != nil {
		t.Fatalf("seed conclusion: %v", err)
	}

	report := messageLoopProjectBlackboardReportFromProjectState(nil, map[string]any{
		"project_state": blackboardReportProjectStateFixture(),
	})
	if !strings.Contains(report, "some_future_outcome") {
		t.Fatalf("unknown outcome dropped instead of shown verbatim:\n%s", report)
	}
}

func TestProjectBlackboardReportUnreadableStoreOmitted(t *testing.T) {
	// A corrupt record makes the board unreadable: the report omits the
	// section instead of failing the whole status report.
	root := t.TempDir()
	mixboardRoot := filepath.Join(root, "mixboard", "sessions")
	t.Setenv("VIT_MIXBOARD_ROOT", mixboardRoot)

	settledAt := time.Date(2026, 10, 11, 8, 30, 0, 0, time.UTC)
	store := mixboard.NewStore(mixboardRoot)
	if _, err := store.RecordFreeStateConclusion(mixboard.FreeStateConclusionRecord{
		ProjectUUID: "proj-report", ConversationID: "conv-earlier", ExperimentID: "exp-3",
		ConclusionID: "fsc_exp-3_experiment_settle",
		Kind:         mixboard.FreeStateConclusionKindExperimentSettle, Outcome: "improved",
		Summary: "seed", SettledAt: settledAt,
	}); err != nil {
		t.Fatalf("seed conclusion: %v", err)
	}
	conclusionDir := filepath.Join(root, "mixboard", "decisions", "free_state_conclusions")
	entries, err := os.ReadDir(conclusionDir)
	if err != nil {
		t.Fatalf("locate conclusion dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "fsc_") {
			if err := os.WriteFile(filepath.Join(conclusionDir, entry.Name()), []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	report := messageLoopProjectBlackboardReportFromProjectState(nil, map[string]any{
		"project_state": blackboardReportProjectStateFixture(),
	})
	if strings.Contains(report, "自由态结论（跨会话）") {
		t.Fatalf("unreadable store leaked a section into the report:\n%s", report)
	}
	if !strings.Contains(report, "工程黑板状态汇报") {
		t.Fatalf("report failed instead of omitting the section:\n%s", report)
	}
}
