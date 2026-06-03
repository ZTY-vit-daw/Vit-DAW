package journal

import (
	"path/filepath"
	"testing"
)

func TestPersistentJournalReloadsActions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.json")
	j, err := NewPersistent(10, path)
	if err != nil {
		t.Fatal(err)
	}
	j.Record(Action{AgentActionID: "act_1", RunID: "run_1", GoalID: "goal_1", Domain: "workspace", Status: StatusRunning})
	j.MarkResult("act_1", StatusSucceeded, map[string]any{"ok": true}, nil)

	reloaded, err := NewPersistent(10, path)
	if err != nil {
		t.Fatal(err)
	}
	actions := reloaded.Recent(1)
	if len(actions) != 1 || actions[0].AgentActionID != "act_1" || actions[0].RunID != "run_1" || actions[0].Status != StatusSucceeded {
		t.Fatalf("actions = %+v", actions)
	}
}
