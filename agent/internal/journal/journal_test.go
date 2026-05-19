package journal

import "testing"

func TestJournalRecordsAndReturnsNewestFirst(t *testing.T) {
	j := New(2)
	j.Record(Action{AgentActionID: "a1", CommandName: "ping", Status: StatusSucceeded})
	j.Record(Action{AgentActionID: "a2", CommandName: "play", Status: StatusSucceeded})
	j.Record(Action{AgentActionID: "a3", CommandName: "stop", Status: StatusSucceeded})

	recent := j.Recent(10)
	if len(recent) != 2 {
		t.Fatalf("len = %d, want 2", len(recent))
	}
	if recent[0].AgentActionID != "a3" || recent[1].AgentActionID != "a2" {
		t.Fatalf("unexpected order: %+v", recent)
	}
	if _, ok := j.Get("a1"); ok {
		t.Fatal("oldest action should have been trimmed")
	}
}

func TestJournalMarkResult(t *testing.T) {
	j := New(10)
	j.Record(Action{AgentActionID: "a1", CommandName: "ping", Status: StatusRunning})
	j.MarkResult("a1", StatusSucceeded, map[string]any{"status": "ok"}, nil)

	action, ok := j.Get("a1")
	if !ok {
		t.Fatal("missing action")
	}
	if action.Status != StatusSucceeded || action.Result["status"] != "ok" {
		t.Fatalf("action = %+v", action)
	}
}
