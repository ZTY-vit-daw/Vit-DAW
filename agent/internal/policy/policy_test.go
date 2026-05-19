package policy

import "testing"

func TestClassifyDirect(t *testing.T) {
	d := Classify(map[string]any{"cmd": "get_project_state"})
	if d.Risk != RiskDirect {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskDirect)
	}
}

func TestClassifyConfirmByDefault(t *testing.T) {
	d := Classify(map[string]any{"cmd": "delete_track", "track_id": "1007"})
	if d.Risk != RiskConfirm {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskConfirm)
	}
}

func TestClassifyUndoableDoesNotRequireConfirmation(t *testing.T) {
	d := Classify(map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true})
	if d.Risk != RiskUndoable {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskUndoable)
	}
	if NeedsConfirmation([]Decision{d}) {
		t.Fatal("undoable command should not require confirmation")
	}
}

func TestCommandNameFallback(t *testing.T) {
	name := CommandName(map[string]any{"action": "play"})
	if name != "play" {
		t.Fatalf("name = %q, want play", name)
	}
}
