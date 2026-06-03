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
