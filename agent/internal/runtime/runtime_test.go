package runtime

import "testing"

func TestInterjectionOrderingAndCancelTick(t *testing.T) {
	rt := New()
	goal := rt.Create("mix pass")
	rt.AddInterjection(goal.GoalID, "pause", "user")
	rt.AddInterjection(goal.GoalID, "try softer", "user")
	status := rt.Status(goal.GoalID)
	if len(status.PendingInterjection) != 2 || status.PendingInterjection[0].Sequence >= status.PendingInterjection[1].Sequence {
		t.Fatalf("interjections not ordered: %+v", status.PendingInterjection)
	}
	rt.Cancel(goal.GoalID, "user cancelled")
	cancelled := rt.Tick(goal.GoalID, "after tool")
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", cancelled.Status)
	}
}

func TestSnapshotRestoreReplacesActiveGoalSet(t *testing.T) {
	source := New()
	first := source.Ensure("goal_a", "run_a", "project A")
	source.AddInterjection(first.GoalID, "pause here", "user")
	source.SetStatus(first.GoalID, StatusWaitingContinue, nil)
	snapshot := source.Snapshot()

	target := New()
	target.Ensure("goal_other", "run_other", "other project")
	target.Restore(snapshot)
	restored := target.Status("")
	if restored.GoalID != first.GoalID || restored.RunID != first.RunID || restored.Status != StatusWaitingContinue {
		t.Fatalf("restored goal=%+v", restored)
	}
	if len(restored.PendingInterjection) != 1 || restored.PendingInterjection[0].Message != "pause here" {
		t.Fatalf("restored interjections=%+v", restored.PendingInterjection)
	}
	if other := target.Status("goal_other"); other.Status != StatusIdle {
		t.Fatalf("foreign goal survived restore: %+v", other)
	}
	target.Restore(Snapshot{})
	if active := target.Status(""); active.Status != StatusIdle || active.GoalID != "" {
		t.Fatalf("empty snapshot did not clear runtime: %+v", active)
	}
}
