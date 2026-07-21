package executionports

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

type fakeHistoryBaseline struct {
	GoalID       string
	RunID        string
	CapabilityID string
	CommitID     string
}

func (f *fakeHistoryBaseline) EnsureCapabilityExecutionBaseline(_ context.Context, goalID, runID, capabilityID string) (string, error) {
	f.GoalID, f.RunID, f.CapabilityID = goalID, runID, capabilityID
	return f.CommitID, nil
}

func TestProjectHistoryPersistenceReturnsDurableBaselineReference(t *testing.T) {
	history := &fakeHistoryBaseline{CommitID: "commit-1"}
	refs, err := (ProjectHistory{Harness: history, GoalID: "goal-1", RunID: "run-1"}).PrepareExecution(
		context.Background(),
		orchestration.PlanningSession{ID: "session-1"},
		orchestration.ActionSet{CapabilityID: "static_mix.static_balance.v0"},
		orchestration.ProjectCut{},
	)
	if err != nil || len(refs) != 1 || refs[0] != "project-history:commit-1" {
		t.Fatalf("refs=%#v err=%v", refs, err)
	}
	if history.GoalID != "goal-1" || history.RunID != "run-1" || history.CapabilityID != "static_mix.static_balance.v0" {
		t.Fatalf("baseline identity lost: %#v", history)
	}
}
