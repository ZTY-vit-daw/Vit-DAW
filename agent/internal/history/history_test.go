package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDefaultDraftProjectPathUsesEditRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	draftProject.Lock()
	draftProject.root = ""
	draftProject.path = ""
	draftProject.Unlock()

	path, err := defaultDraftProjectPath()
	if err != nil {
		t.Fatalf("defaultDraftProjectPath: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read draft project: %v", err)
	}
	if !strings.Contains(string(b), "<EDIT ") {
		t.Fatalf("draft project root = %q, want EDIT", string(b))
	}
}

func TestProjectNewStartsFreshDraftHistory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", root)
	draftProject.Lock()
	draftProject.root = ""
	draftProject.path = ""
	draftProject.Unlock()

	first, err := ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("first project new: %v", err)
	}
	firstPath := fmt.Sprint(first["project_path"])
	firstCommit, err := Checkpoint(map[string]any{
		"message":              "first draft",
		"project_snapshot_xml": "<EDIT draft_state=\"first\"/>",
	})
	if err != nil {
		t.Fatalf("first checkpoint: %v", err)
	}
	if _, err := AppendConversationNode(map[string]any{
		"kind":         "ask",
		"commit_id":    firstCommit["commit_id"],
		"text_preview": "first draft ask",
	}); err != nil {
		t.Fatalf("first conversation node: %v", err)
	}

	second, err := ProjectNew(map[string]any{})
	if err != nil {
		t.Fatalf("second project new: %v", err)
	}
	secondPath := fmt.Sprint(second["project_path"])
	if firstPath == "" || secondPath == "" || sameProjectPath(firstPath, secondPath) {
		t.Fatalf("draft paths should be different: first=%q second=%q", firstPath, secondPath)
	}
	secondStatus, err := Status(map[string]any{})
	if err != nil {
		t.Fatalf("second status: %v", err)
	}
	if messages := secondStatus["conversation_messages"].([]ConversationMessage); len(messages) != 0 {
		t.Fatalf("second draft should start without first draft messages: %+v", messages)
	}
	firstStatus, err := Status(map[string]any{"project_path": firstPath})
	if err != nil {
		t.Fatalf("first status: %v", err)
	}
	if messages := firstStatus["conversation_messages"].([]ConversationMessage); len(messages) != 1 || messages[0].Content != "first draft ask" {
		t.Fatalf("first draft messages should stay scoped: %+v", messages)
	}
}

func TestProjectSavedAdoptsDraftHistory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_HISTORY_DRAFT_ROOT", filepath.Join(root, "drafts"))
	draftProject.Lock()
	draftProject.root = ""
	draftProject.path = ""
	draftProject.Unlock()

	draftCheckpoint, err := Checkpoint(map[string]any{
		"message":              "draft baseline",
		"project_snapshot_xml": "<EDIT draft_state=\"one\"/>",
	})
	if err != nil {
		t.Fatalf("draft checkpoint: %v", err)
	}
	draftCommit := draftCheckpoint["commit"].(Commit)
	if _, err := AppendConversationNode(map[string]any{
		"kind":         "vit",
		"commit_id":    draftCommit.ID,
		"node_id":      "vit_draft_1",
		"text_preview": "created before save",
	}); err != nil {
		t.Fatalf("append draft node: %v", err)
	}
	draftWorktree, err := WorktreeCreate(map[string]any{"name": "A", "commit_id": draftCommit.ID})
	if err != nil {
		t.Fatalf("draft worktree create: %v", err)
	}
	draftWorktreePath := fmt.Sprint(draftWorktree["project_file_path"])

	project := filepath.Join(root, "Saved Song.vit")
	if err := os.WriteFile(project, []byte("<EDIT draft_state=\"one\"/>"), 0o644); err != nil {
		t.Fatalf("write saved project: %v", err)
	}
	adopted, err := ProjectSaved(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("project saved: %v", err)
	}
	if adopted["adopted_draft"] != true {
		t.Fatalf("adopted_draft = %v, result=%+v", adopted["adopted_draft"], adopted)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	graph := status["conversation_graph"].(ConversationGraph)
	if len(graph.Nodes) != 1 || graph.Nodes[0].ID != "vit_draft_1" {
		t.Fatalf("adopted graph nodes = %+v", graph.Nodes)
	}
	list, err := List(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	commits := list["commits"].([]Commit)
	if len(commits) != 1 {
		t.Fatalf("commits = %d, want 1", len(commits))
	}
	if commits[0].ProjectPath != project || commits[0].ProjectFile.Path != filepath.ToSlash(filepath.Base(project)) {
		t.Fatalf("adopted commit path = project %q file %q", commits[0].ProjectPath, commits[0].ProjectFile.Path)
	}
	worktrees := status["worktrees"].([]map[string]any)
	var adoptedWorktree map[string]any
	for _, item := range worktrees {
		if fmt.Sprint(item["name"]) == "A" {
			adoptedWorktree = item
			break
		}
	}
	if adoptedWorktree == nil {
		t.Fatalf("adopted worktree A missing: %+v", worktrees)
	}
	adoptedWorktreePath := fmt.Sprint(adoptedWorktree["project_file_path"])
	if adoptedWorktreePath == "" || adoptedWorktreePath == draftWorktreePath || !fileExists(adoptedWorktreePath) {
		t.Fatalf("adopted worktree path = %q old=%q", adoptedWorktreePath, draftWorktreePath)
	}
	if !strings.Contains(filepath.ToSlash(adoptedWorktreePath), "/Worktrees/A/") || filepath.Base(adoptedWorktreePath) != "Saved Song_A.vit" {
		t.Fatalf("adopted worktree should be visible and renamed, got %q", adoptedWorktreePath)
	}
	if err := os.WriteFile(project, []byte("<EDIT stale=\"true\"/>"), 0o644); err != nil {
		t.Fatalf("write stale project: %v", err)
	}
	if _, err := Checkout(map[string]any{"project_path": project, "commit_id": draftCommit.ID}); err != nil {
		t.Fatalf("checkout adopted commit: %v", err)
	}
	b, err := os.ReadFile(project)
	if err != nil {
		t.Fatalf("read restored project: %v", err)
	}
	if !strings.Contains(string(b), `draft_state="one"`) {
		t.Fatalf("restored project = %q", string(b))
	}
}

func TestCheckpointDedupesObjects(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("<EDIT/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	media := filepath.Join(root, "Song_Media")
	if err := os.MkdirAll(media, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "rack_control_macros.json"), []byte(`{"macros":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Checkpoint(map[string]any{"project_path": project, "message": "one"})
	if err != nil {
		t.Fatalf("checkpoint 1: %v", err)
	}
	second, err := Checkpoint(map[string]any{"project_path": project, "message": "two"})
	if err != nil {
		t.Fatalf("checkpoint 2: %v", err)
	}
	if first["commit"] == nil || second["commit"] == nil {
		t.Fatalf("missing commits: %+v %+v", first, second)
	}
	objects := 0
	err = filepath.WalkDir(filepath.Join(root, DirName, "objects"), func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			objects++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if objects != 2 {
		t.Fatalf("objects = %d, want 2 deduped project+macro objects", objects)
	}
}

func TestRestorePreviewReportsModified(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	commit := result["commit"].(Commit)
	if err := os.WriteFile(project, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := RestorePreview(map[string]any{"project_path": project, "commit_id": commit.ID})
	if err != nil {
		t.Fatal(err)
	}
	changes := preview["changes"].([]map[string]any)
	if len(changes) != 1 || changes[0]["status"] != "modified" {
		t.Fatalf("changes = %+v", changes)
	}
}

func TestProjectHistoryVisibilityTools(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project, "message": "first"})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstCommit := firstResult["commit"].(Commit)
	if err := os.WriteFile(project, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project, "message": "second"})
	if err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	secondCommit := secondResult["commit"].(Commit)

	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["initialized"] != true || status["head"] != secondCommit.ID || status["commit_count"] != 2 {
		t.Fatalf("status = %+v", status)
	}
	if status["active_branch"] != "main" || status["detached"] != false {
		t.Fatalf("branch status = %+v", status)
	}
	if status["project_path"] == "" || status["history_dir"] == "" {
		t.Fatalf("status missing paths: %+v", status)
	}

	list, err := List(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	commits := list["commits"].([]Commit)
	if len(commits) != 2 || commits[0].ID != secondCommit.ID || commits[1].ID != firstCommit.ID {
		t.Fatalf("commits = %+v", commits)
	}
	refs := list["refs"].(map[string]string)
	if refs["refs/heads/main"] != secondCommit.ID {
		t.Fatalf("refs = %+v", refs)
	}
	branches := list["branches"].(map[string]string)
	if branches["main"] != secondCommit.ID {
		t.Fatalf("branches = %+v", branches)
	}

	show, err := Show(map[string]any{"project_path": project, "commit_id": firstCommit.ID})
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if show["commit"].(Commit).ID != firstCommit.ID || show["head"] != secondCommit.ID {
		t.Fatalf("show = %+v", show)
	}

	if err := os.WriteFile(project, []byte("three"), 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := Diff(map[string]any{"project_path": project, "commit_id": secondCommit.ID})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	changes := diff["changes"].([]map[string]any)
	if diff["commit_id"] != secondCommit.ID || len(changes) != 1 || changes[0]["status"] != "modified" {
		t.Fatalf("diff = %+v", diff)
	}

	preview, err := RestorePreview(map[string]any{"project_path": project, "commit_id": firstCommit.ID})
	if err != nil {
		t.Fatalf("restore preview: %v", err)
	}
	if preview["commit_id"] != firstCommit.ID || preview["project_path"] == "" || preview["history_dir"] == "" {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestBranchCheckoutMakesCheckpointAdvanceActiveBranch(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("main-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project, "message": "main one"})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstCommit := firstResult["commit"].(Commit)
	if _, err := BranchCreate(map[string]any{"project_path": project, "branch": "mix-a"}); err != nil {
		t.Fatalf("branch create: %v", err)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["active_branch"] != "mix-a" || status["detached"] != false || status["head"] != firstCommit.ID {
		t.Fatalf("branch create should switch to the new active branch: %+v", status)
	}
	if err := os.WriteFile(project, []byte("mix-a two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project, "message": "mix a"})
	if err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	secondCommit := secondResult["commit"].(Commit)
	status, err = Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status after branch checkpoint: %v", err)
	}
	branches := status["branches"].(map[string]string)
	if status["active_branch"] != "mix-a" || status["head"] != secondCommit.ID {
		t.Fatalf("status = %+v", status)
	}
	if branches["main"] != firstCommit.ID || branches["mix-a"] != secondCommit.ID {
		t.Fatalf("branches = %+v; first=%s second=%s", branches, firstCommit.ID, secondCommit.ID)
	}
	if secondCommit.Branch != "mix-a" || len(secondCommit.Parents) != 1 || secondCommit.Parents[0] != firstCommit.ID {
		t.Fatalf("second commit metadata = %+v", secondCommit)
	}
}

func TestCheckoutCommitEntersDetachedState(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	firstCommit := firstResult["commit"].(Commit)
	if err := os.WriteFile(project, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	secondCommit := secondResult["commit"].(Commit)
	if _, err := Checkout(map[string]any{"project_path": project, "commit_id": firstCommit.ID}); err != nil {
		t.Fatalf("checkout commit: %v", err)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["detached"] != true || status["active_branch"] != "detached" || status["head"] != firstCommit.ID {
		t.Fatalf("detached status = %+v", status)
	}
	if err := os.WriteFile(project, []byte("detached work"), 0o644); err != nil {
		t.Fatal(err)
	}
	detachedResult, err := Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("detached checkpoint: %v", err)
	}
	detachedCommit := detachedResult["commit"].(Commit)
	status, err = Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status after detached checkpoint: %v", err)
	}
	branches := status["branches"].(map[string]string)
	if status["head"] != detachedCommit.ID || branches["main"] != secondCommit.ID {
		t.Fatalf("detached checkpoint should not advance main: status=%+v branches=%+v", status, branches)
	}
	if detachedCommit.Branch != "" {
		t.Fatalf("detached commit should not record branch: %+v", detachedCommit)
	}
}

func TestWorktreeCreateAndListReturnsMetadata(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Checkpoint(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	commit := result["commit"].(Commit)
	created, err := WorktreeCreate(map[string]any{"project_path": project, "commit_id": commit.ID, "name": "copy-a"})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	if created["name"] != "copy-a" || created["commit_id"] != commit.ID || created["created_at"] == nil {
		t.Fatalf("created = %+v", created)
	}
	if fmt.Sprint(created["path"]) != filepath.Join(root, worktreesDirName, "copy-a") {
		t.Fatalf("worktree path = %q", created["path"])
	}
	if filepath.Base(fmt.Sprint(created["project_file_path"])) != "Song_copy-a.vit" {
		t.Fatalf("worktree project file = %q", created["project_file_path"])
	}
	if _, err := os.Stat(fmt.Sprint(created["project_file_path"])); err != nil {
		t.Fatalf("worktree project missing: %v", err)
	}
	list, err := WorktreeList(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	items := list["worktrees"].([]map[string]any)
	if len(items) != 2 || items[0]["name"] != "main" || items[0]["is_root"] != true {
		t.Fatalf("worktrees = %+v", items)
	}
	copyItem := map[string]any{}
	for _, item := range items {
		if item["name"] == "copy-a" {
			copyItem = item
			break
		}
	}
	if copyItem["name"] != "copy-a" || copyItem["commit_id"] != commit.ID {
		t.Fatalf("copy worktree missing = %+v", items)
	}
	if copyItem["project_file_path"] == "" || copyItem["origin_commit_id"] != commit.ID {
		t.Fatalf("worktree metadata missing project_file_path/origin_commit_id: %+v", copyItem)
	}
	checkout, err := WorktreeCheckout(map[string]any{"project_path": project, "name": "copy-a"})
	if err != nil {
		t.Fatalf("worktree checkout: %v", err)
	}
	if checkout["project_file_path"] != copyItem["project_file_path"] || checkout["active_worktree"] != "copy-a" {
		t.Fatalf("worktree checkout = %+v; item=%+v", checkout, copyItem)
	}
	targetHistory, ok := checkout["target_project_history"].(map[string]any)
	if !ok || targetHistory["active_worktree"] != "copy-a" {
		t.Fatalf("target history = %+v", checkout["target_project_history"])
	}
	targetProject := fmt.Sprint(copyItem["project_file_path"])
	if err := os.WriteFile(targetProject, []byte("copy-two"), 0o644); err != nil {
		t.Fatal(err)
	}
	targetCheckpoint, err := Checkpoint(map[string]any{"project_path": targetProject})
	if err != nil {
		t.Fatalf("target checkpoint: %v", err)
	}
	if err := os.WriteFile(targetProject, []byte("stale-copy-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	checkout, err = WorktreeCheckout(map[string]any{"project_path": project, "name": "copy-a"})
	if err != nil {
		t.Fatalf("worktree checkout materialize: %v", err)
	}
	if got, err := os.ReadFile(targetProject); err != nil || string(got) != "copy-two" {
		t.Fatalf("target project after materialize = %q err=%v", string(got), err)
	}
	if checkout["worktree"].(map[string]any)["worktree_head_commit_id"] != targetCheckpoint["commit_id"] {
		t.Fatalf("worktree head not refreshed: %+v", checkout["worktree"])
	}
	rootCheckout, err := WorktreeCheckout(map[string]any{"project_path": copyItem["project_file_path"], "name": "main"})
	if err != nil {
		t.Fatalf("root worktree checkout: %v", err)
	}
	if rootCheckout["project_file_path"] != project || rootCheckout["active_worktree"] != "" {
		t.Fatalf("root checkout = %+v", rootCheckout)
	}
	if err := os.WriteFile(project, []byte("stale-root-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WorktreeCheckout(map[string]any{"project_path": copyItem["project_file_path"], "name": "main"}); err != nil {
		t.Fatalf("root worktree checkout materialize: %v", err)
	}
	if got, err := os.ReadFile(project); err != nil || string(got) != "one" {
		t.Fatalf("root project after materialize = %q err=%v", string(got), err)
	}
}

func TestConversationGraphNodeCheckoutAndBranchFromNode(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project, "message": "one"})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstCommit := firstResult["commit"].(Commit)
	askResult, err := AppendConversationNode(map[string]any{
		"project_path": project,
		"kind":         "ask",
		"commit_id":    firstCommit.ID,
		"text_preview": "make a clip",
		"goal_id":      "goal_a",
		"run_id":       "run_a",
	})
	if err != nil {
		t.Fatalf("append ask: %v", err)
	}
	askNode := askResult["node"].(ConversationNode)

	if err := os.WriteFile(project, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project, "message": "two"})
	if err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	secondCommit := secondResult["commit"].(Commit)
	vitResult, err := AppendConversationNode(map[string]any{
		"project_path": project,
		"kind":         "vit",
		"commit_id":    secondCommit.ID,
		"text_preview": "done",
	})
	if err != nil {
		t.Fatalf("append vit: %v", err)
	}
	vitNode := vitResult["node"].(ConversationNode)
	if vitNode.ParentNodeID != askNode.ID {
		t.Fatalf("vit parent = %q, want ask %q", vitNode.ParentNodeID, askNode.ID)
	}

	if _, err := NodeCheckout(map[string]any{"project_path": project, "node_id": askNode.ID}); err != nil {
		t.Fatalf("node checkout: %v", err)
	}
	if got, err := os.ReadFile(project); err != nil || string(got) != "one" {
		t.Fatalf("project after node checkout = %q err=%v", string(got), err)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["detached"] != true || status["active_node_id"] != askNode.ID || status["head"] != firstCommit.ID {
		t.Fatalf("node checkout status = %+v", status)
	}

	branch, err := BranchCreate(map[string]any{
		"project_path": project,
		"branch":       "mix-b",
		"commit_id":    firstCommit.ID,
		"from_node_id": askNode.ID,
		"goal_id":      "goal_b",
		"run_id":       "run_b",
	})
	if err != nil {
		t.Fatalf("branch from node: %v", err)
	}
	if branch["active_branch"] != "mix-b" || branch["head"] != firstCommit.ID {
		t.Fatalf("branch result = %+v", branch)
	}
	graph := branch["conversation_graph"].(ConversationGraph)
	if graph.ActiveBranch != "mix-b" || graph.ActiveNodeID == "" {
		t.Fatalf("branch graph = %+v", graph)
	}
	var marker ConversationNode
	for _, node := range graph.Nodes {
		if node.Kind == "branch_marker" && node.ParentNodeID == askNode.ID && node.Branch == "mix-b" {
			marker = node
			break
		}
	}
	if marker.ID == "" || graph.ActiveNodeID != marker.ID {
		t.Fatalf("missing active branch marker: graph=%+v marker=%+v", graph, marker)
	}
}

func TestConversationMessagesFollowActiveNodePath(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project, "message": "one"})
	if err != nil {
		t.Fatalf("checkpoint first: %v", err)
	}
	firstID := firstResult["commit"].(Commit).ID
	if _, err := AppendConversationNode(map[string]any{
		"project_path": project,
		"kind":         "ask",
		"commit_id":    firstID,
		"node_id":      "ask_1",
		"text_preview": strings.Repeat("用户完整问题", 30),
	}); err != nil {
		t.Fatalf("append ask: %v", err)
	}
	if err := os.WriteFile(project, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project, "message": "two"})
	if err != nil {
		t.Fatalf("checkpoint second: %v", err)
	}
	secondID := secondResult["commit"].(Commit).ID
	if _, err := AppendConversationNode(map[string]any{
		"project_path":   project,
		"kind":           "vit",
		"commit_id":      secondID,
		"node_id":        "vit_1",
		"parent_node_id": "ask_1",
		"text_preview":   "主线回复",
	}); err != nil {
		t.Fatalf("append vit: %v", err)
	}
	if _, err := BranchCreate(map[string]any{"project_path": project, "branch": "mix-b", "commit_id": firstID, "from_node_id": "ask_1"}); err != nil {
		t.Fatalf("branch: %v", err)
	}
	if err := os.WriteFile(project, []byte("branch"), 0o644); err != nil {
		t.Fatal(err)
	}
	branchResult, err := Checkpoint(map[string]any{"project_path": project, "message": "branch"})
	if err != nil {
		t.Fatalf("checkpoint branch: %v", err)
	}
	branchID := branchResult["commit"].(Commit).ID
	if _, err := AppendConversationNode(map[string]any{
		"project_path": project,
		"kind":         "vit",
		"commit_id":    branchID,
		"node_id":      "vit_branch",
		"text_preview": "分支回复",
	}); err != nil {
		t.Fatalf("append branch vit: %v", err)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	messages := status["conversation_messages"].([]ConversationMessage)
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Content != "分支回复" {
		t.Fatalf("conversation messages = %+v", messages)
	}
	if messages[0].Content != strings.Repeat("用户完整问题", 30) {
		t.Fatalf("ask content was truncated: %q", messages[0].Content)
	}
}

func TestCompactPreviewKeepsUnicodeBoundaries(t *testing.T) {
	preview := compactPreview(strings.Repeat("低频冲突观察", 40))
	if !utf8.ValidString(preview) {
		t.Fatalf("preview is not valid utf8: %q", preview)
	}
	if strings.ContainsRune(preview, '\uFFFD') {
		t.Fatalf("preview contains replacement rune: %q", preview)
	}
	if len([]rune(preview)) > 160 {
		t.Fatalf("preview too long: %d %q", len([]rune(preview)), preview)
	}
}

func TestConversationMessagesIncludeArtifacts(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Checkpoint(map[string]any{"project_path": project, "message": "one"})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := result["commit"].(Commit)
	if _, err := AppendConversationNode(map[string]any{
		"project_path": project,
		"kind":         "vit",
		"commit_id":    commit.ID,
		"text_preview": "found media",
		"artifacts": []map[string]any{
			{
				"id":    "art_video",
				"kind":  "video",
				"title": "demo.mp4",
				"path":  filepath.Join(root, "demo.mp4"),
				"mime":  "video/mp4",
			},
		},
	}); err != nil {
		t.Fatalf("append vit: %v", err)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	messages := status["conversation_messages"].([]ConversationMessage)
	if len(messages) != 1 {
		t.Fatalf("conversation messages = %+v", messages)
	}
	if len(messages[0].Artifacts) != 1 || messages[0].Artifacts[0]["id"] != "art_video" {
		t.Fatalf("message artifacts = %+v", messages[0].Artifacts)
	}
}

func TestConversationMessagesAreScopedPerWorktreeProject(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCheckpoint, err := Checkpoint(map[string]any{"project_path": project, "message": "root"})
	if err != nil {
		t.Fatalf("root checkpoint: %v", err)
	}
	rootCommitID := rootCheckpoint["commit"].(Commit).ID
	if _, err := AppendConversationNode(map[string]any{"project_path": project, "kind": "ask", "commit_id": rootCommitID, "node_id": "root_ask", "text_preview": "root ask"}); err != nil {
		t.Fatalf("root ask: %v", err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": project, "kind": "vit", "commit_id": rootCommitID, "node_id": "root_vit", "parent_node_id": "root_ask", "text_preview": "root reply"}); err != nil {
		t.Fatalf("root vit: %v", err)
	}

	worktree, err := WorktreeCreate(map[string]any{"project_path": project, "name": "wt-a", "commit_id": rootCommitID})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	worktreeProject := fmt.Sprint(worktree["project_file_path"])
	worktreeStatus, err := Status(map[string]any{"project_path": worktreeProject})
	if err != nil {
		t.Fatalf("worktree status: %v", err)
	}
	worktreeCommitID := fmt.Sprint(worktreeStatus["head"])
	if _, err := AppendConversationNode(map[string]any{"project_path": worktreeProject, "kind": "ask", "commit_id": worktreeCommitID, "node_id": "wt_ask", "text_preview": "worktree ask"}); err != nil {
		t.Fatalf("worktree ask: %v", err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": worktreeProject, "kind": "vit", "commit_id": worktreeCommitID, "node_id": "wt_vit", "parent_node_id": "wt_ask", "text_preview": "worktree reply"}); err != nil {
		t.Fatalf("worktree vit: %v", err)
	}

	rootStatus, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatalf("root status: %v", err)
	}
	rootMessages := rootStatus["conversation_messages"].([]ConversationMessage)
	if len(rootMessages) != 2 || rootMessages[0].Content != "root ask" || rootMessages[1].Content != "root reply" {
		t.Fatalf("root messages = %+v", rootMessages)
	}
	worktreeStatus, err = Status(map[string]any{"project_path": worktreeProject})
	if err != nil {
		t.Fatalf("worktree status after append: %v", err)
	}
	worktreeMessages := worktreeStatus["conversation_messages"].([]ConversationMessage)
	if len(worktreeMessages) != 2 || worktreeMessages[0].Content != "worktree ask" || worktreeMessages[1].Content != "worktree reply" {
		t.Fatalf("worktree messages = %+v", worktreeMessages)
	}
}

func TestProjectHistoryRefsAndWorktreesAreScopedPerProjectFile(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "A.vit")
	projectB := filepath.Join(root, "B.vit")
	if err := os.WriteFile(projectA, []byte("a-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectB, []byte("b-one"), 0o644); err != nil {
		t.Fatal(err)
	}

	resultA, err := Checkpoint(map[string]any{"project_path": projectA, "message": "a"})
	if err != nil {
		t.Fatalf("checkpoint a: %v", err)
	}
	commitA := resultA["commit"].(Commit)
	if _, err := BranchCreate(map[string]any{"project_path": projectA, "branch": "mix-a"}); err != nil {
		t.Fatalf("branch a: %v", err)
	}
	if _, err := WorktreeCreate(map[string]any{"project_path": projectA, "name": "copy-a"}); err != nil {
		t.Fatalf("worktree a: %v", err)
	}

	// Simulate old root-level refs/worktrees from another project in the same folder.
	legacyRef := filepath.Join(root, DirName, "refs", "heads", "legacy-a")
	if err := os.MkdirAll(filepath.Dir(legacyRef), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyRef, []byte(commitA.ID), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyWorktree := filepath.Join(root, DirName, "worktrees", "legacy-copy-a")
	if err := writeJSON(filepath.Join(legacyWorktree, ".vit_worktree.json"), map[string]any{
		"name":      "legacy-copy-a",
		"commit_id": commitA.ID,
		"path":      legacyWorktree,
	}); err != nil {
		t.Fatal(err)
	}

	resultB, err := Checkpoint(map[string]any{"project_path": projectB, "message": "b"})
	if err != nil {
		t.Fatalf("checkpoint b: %v", err)
	}
	commitB := resultB["commit"].(Commit)
	if _, err := BranchCreate(map[string]any{"project_path": projectB, "branch": "mix-b"}); err != nil {
		t.Fatalf("branch b: %v", err)
	}
	if _, err := WorktreeCreate(map[string]any{"project_path": projectB, "name": "copy-b"}); err != nil {
		t.Fatalf("worktree b: %v", err)
	}

	listB, err := List(map[string]any{"project_path": projectB})
	if err != nil {
		t.Fatalf("list b: %v", err)
	}
	branchesB := listB["branches"].(map[string]string)
	if branchesB["main"] != commitB.ID || branchesB["mix-b"] != commitB.ID {
		t.Fatalf("project b branches = %+v", branchesB)
	}
	if _, ok := branchesB["mix-a"]; ok {
		t.Fatalf("project b leaked project a branch: %+v", branchesB)
	}
	if _, ok := branchesB["legacy-a"]; ok {
		t.Fatalf("project b leaked legacy project a branch: %+v", branchesB)
	}
	worktreesB := listB["worktrees"].([]map[string]any)
	if len(worktreesB) != 2 || worktreesB[0]["name"] != "main" || worktreesB[0]["is_root"] != true {
		t.Fatalf("project b worktrees = %+v", worktreesB)
	}
	foundCopyB := false
	for _, item := range worktreesB {
		if item["name"] == "copy-b" {
			foundCopyB = true
		}
	}
	if !foundCopyB {
		t.Fatalf("project b missing copy-b worktree = %+v", worktreesB)
	}
	commitsB := listB["commits"].([]Commit)
	if len(commitsB) != 1 || commitsB[0].ID != commitB.ID {
		t.Fatalf("project b commits = %+v", commitsB)
	}

	listA, err := List(map[string]any{"project_path": projectA})
	if err != nil {
		t.Fatalf("list a: %v", err)
	}
	branchesA := listA["branches"].(map[string]string)
	if branchesA["main"] != commitA.ID || branchesA["mix-a"] != commitA.ID || branchesA["legacy-a"] != commitA.ID {
		t.Fatalf("project a branches = %+v", branchesA)
	}
	if _, ok := branchesA["mix-b"]; ok {
		t.Fatalf("project a leaked project b branch: %+v", branchesA)
	}
	worktreesA := listA["worktrees"].([]map[string]any)
	if len(worktreesA) != 3 {
		t.Fatalf("project a worktrees = %+v", worktreesA)
	}
}

func TestDiffPrefersMemorySnapshotForProjectFile(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("disk"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Checkpoint(map[string]any{
		"project_path":         project,
		"project_snapshot_xml": "memory-one",
	})
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	commit := result["commit"].(Commit)
	diff, err := Diff(map[string]any{
		"project_path":         project,
		"commit_id":            commit.ID,
		"project_snapshot_xml": "memory-two",
	})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	changes := diff["changes"].([]map[string]any)
	if len(changes) != 1 || changes[0]["status"] != "modified" || changes[0]["source"] != "memory_snapshot" {
		t.Fatalf("changes = %+v", changes)
	}
}

func TestNodeDeleteRemovesSubtree(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(project, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstResult, err := Checkpoint(map[string]any{"project_path": project, "message": "one"})
	if err != nil {
		t.Fatal(err)
	}
	firstID := firstResult["commit"].(Commit).ID
	askResult, err := AppendConversationNode(map[string]any{"project_path": project, "kind": "ask", "commit_id": firstID, "node_id": "ask_1", "text_preview": "ask"})
	if err != nil {
		t.Fatal(err)
	}
	if askResult["active_node_id"] != "ask_1" {
		t.Fatalf("ask result = %+v", askResult)
	}
	if _, err := BranchCreate(map[string]any{"project_path": project, "branch": "mix-a", "commit_id": firstID, "from_node_id": "ask_1"}); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(project, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	secondResult, err := Checkpoint(map[string]any{"project_path": project, "message": "two"})
	if err != nil {
		t.Fatal(err)
	}
	secondID := secondResult["commit"].(Commit).ID
	if _, err := AppendConversationNode(map[string]any{"project_path": project, "kind": "vit", "commit_id": secondID, "node_id": "vit_1", "parent_node_id": "ask_1", "branch": "mix-a", "text_preview": "vit"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := NodeDelete(map[string]any{"project_path": project, "node_id": "vit_1"})
	if err != nil {
		t.Fatalf("node delete: %v", err)
	}
	if deleted["deleted_node_count"] != 2 {
		t.Fatalf("deleted = %+v", deleted)
	}
	status, err := Status(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	graph := status["conversation_graph"].(ConversationGraph)
	if len(graph.Nodes) != 1 || graph.Nodes[0].ID != "ask_1" {
		t.Fatalf("graph = %+v", graph)
	}
	if _, err := readCommit(OpenForTest(t, project), secondID); err == nil {
		t.Fatalf("deleted child commit still exists: %s", secondID)
	}
	if _, err := readCommit(OpenForTest(t, project), firstID); err != nil {
		t.Fatalf("parent commit should remain: %v", err)
	}
}

func TestProjectUUIDWorkspacesKeepConversationsIsolated(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "A.vit")
	projectB := filepath.Join(root, "B.vit")
	if err := os.WriteFile(projectA, []byte("<EDIT projectID=\"a\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectB, []byte("<EDIT projectID=\"b\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	BindProjectIdentity(projectA, "vitproj_a")
	BindProjectIdentity(projectB, "vitproj_b")
	appendTestConversation(t, projectA, "A only")
	appendTestConversation(t, projectB, "B only")
	a, _ := ConversationMessages(map[string]any{"project_path": projectA})
	b, _ := ConversationMessages(map[string]any{"project_path": projectB})
	if got := conversationContents(a); len(got) != 1 || got[0] != "A only" {
		t.Fatalf("project A messages = %#v", got)
	}
	if got := conversationContents(b); len(got) != 1 || got[0] != "B only" {
		t.Fatalf("project B messages = %#v", got)
	}
	repoA, _ := Open(projectA)
	repoB, _ := Open(projectB)
	if repoA.HistoryDir == repoB.HistoryDir || filepath.Base(repoA.HistoryDir) != "vitproj_a" || filepath.Base(repoB.HistoryDir) != "vitproj_b" {
		t.Fatalf("UUID history dirs not isolated: A=%s B=%s", repoA.HistoryDir, repoB.HistoryDir)
	}
}

func TestProjectUUIDWorkspaceMigratesLegacyPathHistory(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "Legacy.vit")
	if err := os.WriteFile(project, []byte("<EDIT projectID=\"legacy\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	appendTestConversation(t, project, "legacy conversation")
	legacyRepo, _ := Open(project)
	BindProjectIdentity(project, "vitproj_migrated")
	migratedRepo, err := Open(project)
	if err != nil {
		t.Fatal(err)
	}
	if migratedRepo.HistoryDir == legacyRepo.HistoryDir || !fileExists(filepath.Join(migratedRepo.HistoryDir, "workspace.json")) {
		t.Fatalf("legacy workspace was not migrated: legacy=%s migrated=%s", legacyRepo.HistoryDir, migratedRepo.HistoryDir)
	}
	messages, _ := ConversationMessages(map[string]any{"project_path": project})
	if got := conversationContents(messages); len(got) != 1 || got[0] != "legacy conversation" {
		t.Fatalf("migrated messages = %#v", got)
	}
}

func TestProjectForkCopiesConversationThenDiverges(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Source.vit")
	target := filepath.Join(root, "Target.vit")
	if err := os.WriteFile(source, []byte("<EDIT projectID=\"source\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("<EDIT projectID=\"target\"/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	BindProjectIdentity(source, "vitproj_source")
	appendTestConversation(t, source, "before save as")
	result, err := ProjectFork(map[string]any{
		"source_project_path": source,
		"source_project_uuid": "vitproj_source",
		"project_path":        target,
		"project_uuid":        "vitproj_target",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["forked_history"] != true {
		t.Fatalf("fork result = %#v", result)
	}
	appendTestConversation(t, target, "target only")
	sourceMessages, _ := ConversationMessages(map[string]any{"project_path": source})
	targetMessages, _ := ConversationMessages(map[string]any{"project_path": target})
	if got := conversationContents(sourceMessages); len(got) != 1 || got[0] != "before save as" {
		t.Fatalf("source messages after fork = %#v", got)
	}
	if got := conversationContents(targetMessages); len(got) != 2 || got[0] != "before save as" || got[1] != "target only" {
		t.Fatalf("target messages after fork = %#v", got)
	}
}

func appendTestConversation(t *testing.T, projectPath, text string) {
	t.Helper()
	checkpoint, err := Checkpoint(map[string]any{"project_path": projectPath, "message": text})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{
		"project_path": projectPath,
		"commit_id":    checkpoint["commit_id"],
		"kind":         "ask",
		"text":         text,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestConversationMessageLifecycleProtocol(t *testing.T) {
	project := filepath.Join(t.TempDir(), "message-protocol.vit")
	if err := os.WriteFile(project, []byte(`<EDIT projectID="message-protocol"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := Checkpoint(map[string]any{"project_path": project, "message": "proposal"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := AppendConversationNode(map[string]any{
		"project_path":       project,
		"commit_id":          checkpoint["commit_id"],
		"kind":               "vit",
		"text":               "B2 static balance proposal",
		"message_kind":       "proposal",
		"turn_id":            "run-1",
		"logical_message_id": "proposal-1",
		"supersedes":         []string{"proposal-0"},
		"message_data": map[string]any{
			"schema_version":     "vit.message_data.v1",
			"needs_confirmation": true,
			"plan_id":            "plan-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	node := result["node"].(ConversationNode)
	if node.Lifecycle != "durable" || node.Persistence != "project_history" || node.MessageKind != "proposal" {
		t.Fatalf("node protocol = %#v", node)
	}
	messages, err := ConversationMessages(map[string]any{"project_path": project})
	if err != nil {
		t.Fatal(err)
	}
	rows := messages["conversation_messages"].([]ConversationMessage)
	if len(rows) != 1 {
		t.Fatalf("message count = %d", len(rows))
	}
	message := rows[0]
	if message.Lifecycle != "durable" || message.Persistence != "project_history" || message.MessageKind != "proposal" || message.TurnID != "run-1" || message.LogicalMessageID != "proposal-1" {
		t.Fatalf("message protocol = %#v", message)
	}
	if len(message.Supersedes) != 1 || message.Supersedes[0] != "proposal-0" {
		t.Fatalf("message supersedes = %#v", message.Supersedes)
	}
	if message.MessageData["schema_version"] != "vit.message_data.v1" || message.MessageData["plan_id"] != "plan-1" {
		t.Fatalf("message data = %#v", message.MessageData)
	}
}

func TestConversationHistoryRejectsTransientActivity(t *testing.T) {
	project := filepath.Join(t.TempDir(), "transient-rejected.vit")
	if err := os.WriteFile(project, []byte(`<EDIT projectID="transient-rejected"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := Checkpoint(map[string]any{"project_path": project, "message": "activity"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = AppendConversationNode(map[string]any{
		"project_path": project,
		"commit_id":    checkpoint["commit_id"],
		"kind":         "vit",
		"text":         "reading tracks",
		"lifecycle":    "transient",
		"persistence":  "none",
	})
	if err == nil || !strings.Contains(err.Error(), "transient conversation activity") {
		t.Fatalf("expected transient persistence rejection, got %v", err)
	}
}

func conversationContents(result map[string]any) []string {
	rows, _ := result["conversation_messages"].([]ConversationMessage)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Content)
	}
	return out
}

func OpenForTest(t *testing.T, project string) Repo {
	t.Helper()
	repo, err := Open(project)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
