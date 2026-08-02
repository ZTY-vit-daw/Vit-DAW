package journal

import (
	"os"
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

func TestShardedJournalAppendsCompactedUpdatesAndReloadsLatestState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	j, err := NewSharded(10, root, "vitproj_journal")
	if err != nil {
		t.Fatal(err)
	}
	j.SetShardLimits(1024*1024, 2)
	j.Record(Action{
		AgentActionID: "act_1", Domain: "daw", CommandName: "mix.observe", Status: StatusRunning,
		Command: map[string]any{"cmd": "mix.observe", "large": make([]any, 1000)},
	})
	j.MarkResult("act_1", StatusSucceeded, map[string]any{
		"status": "ok", "tracks": make([]any, 1000), "counts": map[string]any{"tracks": 8},
	}, nil)
	j.Record(Action{AgentActionID: "act_2", Domain: "daw", CommandName: "track_gain_adjust", Status: StatusSucceeded})

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("shards=%d want=2", len(entries))
	}
	reloaded, err := NewSharded(10, root, "vitproj_journal")
	if err != nil {
		t.Fatal(err)
	}
	action, ok := reloaded.Get("act_1")
	if !ok || action.Status != StatusSucceeded || action.ProjectUUID != "vitproj_journal" || action.SchemaVersion != JournalSchemaVersion {
		t.Fatalf("action=%+v ok=%v", action, ok)
	}
	if action.Command != nil {
		t.Fatalf("large DAW command persisted: %#v", action.Command)
	}
	if _, exists := action.Result["tracks"]; exists {
		t.Fatalf("large result array persisted: %#v", action.Result)
	}
	if action.Result["status"] != "ok" {
		t.Fatalf("result summary=%#v", action.Result)
	}
}

func TestShardedJournalPreservesWorkspaceRollbackFields(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	j, err := NewSharded(10, root, "vitproj_workspace")
	if err != nil {
		t.Fatal(err)
	}
	j.Record(Action{
		AgentActionID: "act_workspace", Domain: "workspace", CommandName: "workspace_apply_edit", Status: StatusRunning,
		Command: map[string]any{"project_path": `D:\Work\song.vit`, "unneeded": "drop"},
	})
	j.MarkResult("act_workspace", StatusSucceeded, map[string]any{
		"status": "ok", "path": "notes.txt", "reverse_old_text": "after", "reverse_new_text": "before",
	}, nil)
	reloaded, err := NewSharded(10, root, "vitproj_workspace")
	if err != nil {
		t.Fatal(err)
	}
	action, ok := reloaded.Get("act_workspace")
	if !ok || action.Result["reverse_old_text"] != "after" || action.Result["reverse_new_text"] != "before" || action.Result["path"] != "notes.txt" {
		t.Fatalf("rollback action=%+v ok=%v", action, ok)
	}
	if action.Command["project_path"] == "" || action.Command["unneeded"] != nil {
		t.Fatalf("workspace command=%#v", action.Command)
	}
}

func TestShardedJournalRejectsProjectIdentityMismatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	j, err := NewSharded(10, root, "vitproj_a")
	if err != nil {
		t.Fatal(err)
	}
	j.Record(Action{AgentActionID: "act_1", Status: StatusSucceeded})
	if _, err := NewSharded(10, root, "vitproj_b"); err == nil {
		t.Fatal("project identity mismatch was accepted")
	}
}

func TestShardedJournalFoldsOldPollingShardsIntoArchive(t *testing.T) {
	root := filepath.Join(t.TempDir(), "journal")
	j, err := NewSharded(20, root, "vitproj_archive")
	if err != nil {
		t.Fatal(err)
	}
	j.SetShardLimits(1024*1024, 1, 2)
	j.Record(Action{AgentActionID: "poll", Tool: "mix.observe", CommandName: "mix.observe", Status: StatusSucceeded})
	j.Record(Action{AgentActionID: "write", Tool: "track_gain_adjust", CommandName: "track_gain_adjust", Status: StatusSucceeded})
	j.Record(Action{AgentActionID: "failed", Tool: "track_gain_adjust", CommandName: "track_gain_adjust", Status: StatusFailed, Error: "failed"})
	if _, err := os.Stat(filepath.Join(root, "archive", "000001.jsonl")); err != nil {
		t.Fatalf("archive shard missing: %v", err)
	}
	reloaded, err := NewSharded(20, root, "vitproj_archive")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Get("poll"); ok {
		t.Fatal("successful polling action should be folded out")
	}
	if _, ok := reloaded.Get("write"); !ok {
		t.Fatal("write action was lost during shard folding")
	}
	if _, ok := reloaded.Get("failed"); !ok {
		t.Fatal("failed action was lost during shard folding")
	}
}
