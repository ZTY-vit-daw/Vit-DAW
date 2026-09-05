package history

import (
	"path/filepath"
	"testing"
)

func TestHeadCheckpointRevisionReportsRecordedRevision(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "helpers.vit")
	BindProjectIdentity(projectPath, "vitproj_helpers")
	if _, err := EnsureWorkingSession(projectPath, "vitproj_helpers"); err != nil {
		t.Fatal(err)
	}

	revision, commitID, err := HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil || revision != "" || commitID != "" {
		t.Fatalf("empty repo must report unknown revision: rev=%q commit=%q err=%v", revision, commitID, err)
	}

	if _, err := Checkpoint(map[string]any{
		"project_path": projectPath, "message": "old style", "source": "test",
		"project_snapshot_xml": "<p/>",
	}); err != nil {
		t.Fatal(err)
	}
	revision, commitID, err = HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil || revision != "" || commitID == "" {
		t.Fatalf("legacy checkpoint without revision must read as unknown-over-HEAD: rev=%q commit=%q err=%v", revision, commitID, err)
	}

	secondCheckpoint, err := Checkpoint(map[string]any{
		"project_path": projectPath, "message": "with revision", "source": "test",
		"project_snapshot_xml": "<p/>", "project_revision": "9",
	})
	if err != nil {
		t.Fatal(err)
	}
	revision, commitID, err = HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil || revision != "9" || commitID != value(secondCheckpoint, "commit_id") {
		t.Fatalf("HEAD revision=%q commit=%q want rev=9 commit=%s err=%v", revision, commitID, value(secondCheckpoint, "commit_id"), err)
	}
}

func TestRebindConversationNodeCommitMovesNodesToLaterCommit(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "rebind.vit")
	BindProjectIdentity(projectPath, "vitproj_rebind")
	if _, err := EnsureWorkingSession(projectPath, "vitproj_rebind"); err != nil {
		t.Fatal(err)
	}
	first, err := Checkpoint(map[string]any{
		"project_path": projectPath, "message": "first", "source": "test",
		"project_snapshot_xml": "<p/>", "project_revision": "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstID := value(first, "commit_id")
	node, err := AppendConversationNode(map[string]any{
		"project_path": projectPath, "kind": "vit", "commit_id": firstID, "text": "bound to first",
	})
	if err != nil {
		t.Fatal(err)
	}
	nodeRow, ok := node["node"].(ConversationNode)
	if !ok {
		t.Fatalf("node result missing: %+v", node)
	}

	second, err := Checkpoint(map[string]any{
		"project_path": projectPath, "message": "second", "source": "test",
		"project_snapshot_xml": "<p/>", "project_revision": "2",
	})
	if err != nil {
		t.Fatal(err)
	}
	secondID := value(second, "commit_id")

	if err := RebindConversationNodeCommit(map[string]any{"project_path": projectPath}, []string{nodeRow.ID}, secondID); err != nil {
		t.Fatal(err)
	}
	graph := readConversationGraphOrDefault(mustOpen(t, projectPath))
	found := false
	for _, n := range graph.Nodes {
		if n.ID == nodeRow.ID {
			found = true
			if n.CommitID != secondID {
				t.Fatalf("node not rebound: %s want %s", n.CommitID, secondID)
			}
		}
	}
	if !found {
		t.Fatalf("rebound node %s missing from graph", nodeRow.ID)
	}

	if err := RebindConversationNodeCommit(map[string]any{"project_path": projectPath}, []string{nodeRow.ID}, "c_missing"); err == nil {
		t.Fatal("rebind to a non-existent commit must fail")
	}
}

func mustOpen(t *testing.T, projectPath string) Repo {
	t.Helper()
	repo, err := Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
