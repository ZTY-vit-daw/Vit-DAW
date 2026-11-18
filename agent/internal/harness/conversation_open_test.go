package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
)

// conversationOpenFixture builds one saved root project with Project History and
// one worktree copy, each carrying its own conversation node. The worktree's
// project file is left with uncheckpointed content ("wt-after") so the composite
// tool's materialization is observable, and the root project's main branch stays
// at rootCommit (same-worktree branch provenance).
type conversationOpenFixture struct {
	root             string
	rootProject      string
	rootCommitID     string
	rootNodeID       string
	worktreeName     string
	worktreePath     string
	worktreeCommitID string
	worktreeNodeID   string
}

func newConversationOpenFixture(t *testing.T) *conversationOpenFixture {
	t.Helper()
	root := t.TempDir()
	rootProject := filepath.Join(root, "Song.vit")
	if err := os.WriteFile(rootProject, []byte("root-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCheckpoint, err := history.Checkpoint(map[string]any{"project_path": rootProject, "message": "root one"})
	if err != nil {
		t.Fatalf("root checkpoint: %v", err)
	}
	rootCommit, ok := rootCheckpoint["commit"].(history.Commit)
	if !ok {
		t.Fatalf("root checkpoint commit = %T", rootCheckpoint["commit"])
	}
	rootNodeResult, err := history.AppendConversationNode(map[string]any{
		"project_path": rootProject,
		"kind":         "ask",
		"commit_id":    rootCommit.ID,
		"text":         "root ask",
	})
	if err != nil {
		t.Fatalf("root conversation node: %v", err)
	}
	rootNode, ok := rootNodeResult["node"].(history.ConversationNode)
	if !ok {
		t.Fatalf("root node = %T", rootNodeResult["node"])
	}

	const worktreeName = "wt-1"
	created, err := history.WorktreeCreate(map[string]any{
		"project_path": rootProject,
		"commit_id":    rootCommit.ID,
		"name":         worktreeName,
	})
	if err != nil {
		t.Fatalf("worktree create: %v", err)
	}
	worktreePath := fmt.Sprint(created["project_file_path"])
	if err := os.WriteFile(worktreePath, []byte("wt-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktreeCheckpoint, err := history.Checkpoint(map[string]any{"project_path": worktreePath, "message": "worktree one"})
	if err != nil {
		t.Fatalf("worktree checkpoint: %v", err)
	}
	worktreeCommit, ok := worktreeCheckpoint["commit"].(history.Commit)
	if !ok {
		t.Fatalf("worktree checkpoint commit = %T", worktreeCheckpoint["commit"])
	}
	worktreeNodeResult, err := history.AppendConversationNode(map[string]any{
		"project_path": worktreePath,
		"kind":         "ask",
		"commit_id":    worktreeCommit.ID,
		"text":         "worktree ask",
	})
	if err != nil {
		t.Fatalf("worktree conversation node: %v", err)
	}
	worktreeNode, ok := worktreeNodeResult["node"].(history.ConversationNode)
	if !ok {
		t.Fatalf("worktree node = %T", worktreeNodeResult["node"])
	}
	// Leave uncheckpointed target content so materialization must restore wt-one.
	if err := os.WriteFile(worktreePath, []byte("wt-after"), 0o644); err != nil {
		t.Fatal(err)
	}
	return &conversationOpenFixture{
		root:             root,
		rootProject:      rootProject,
		rootCommitID:     rootCommit.ID,
		rootNodeID:       rootNode.ID,
		worktreeName:     worktreeName,
		worktreePath:     worktreePath,
		worktreeCommitID: worktreeCommit.ID,
		worktreeNodeID:   worktreeNode.ID,
	}
}

func conversationOpenShadow(t *testing.T, projectPath string) *shadow.Project {
	t.Helper()
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath})
	return project
}

func conversationOpenKernelCommands(kernel *fakeKernelClient) []string {
	commands := make([]string, 0, len(kernel.commands))
	for _, command := range kernel.commands {
		commands = append(commands, firstString(command, "cmd"))
	}
	return commands
}

func conversationOpenCountCommand(kernel *fakeKernelClient, name string) int {
	count := 0
	for _, command := range kernel.commands {
		if firstString(command, "cmd") == name {
			count++
		}
	}
	return count
}

func conversationOpenReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// TestConversationOpenSameWorktreeReloadsOnce covers requirement 2: a node that
// is the head of its own branch is opened with the existing branch-checkout
// semantics (materialize commit, write active branch/node) and exactly one
// project reload (one open_project + one get_project_state) follows.
func TestConversationOpenSameWorktreeReloadsOnce(t *testing.T) {
	fixture := newConversationOpenFixture(t)
	if err := os.WriteFile(fixture.rootProject, []byte("root-dirty"), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "project_path": fixture.rootProject},
		{"status": "ok", "project_path": fixture.rootProject},
	}}
	h := New(nil, conversationOpenShadow(t, fixture.rootProject), nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.conversation.open",
		Args:      map[string]any{"name": "main", "node_id": fixture.rootNodeID, "branch": "main"},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Status != "ok" || resp.Tool != "version.conversation.open" {
		t.Fatalf("resp = %+v", resp)
	}
	if got := conversationOpenReadFile(t, fixture.rootProject); got != "root-one" {
		t.Fatalf("project content after conversation open = %q, want root-one", got)
	}
	wantCommands := []string{"open_project", "get_project_state"}
	if got := conversationOpenKernelCommands(kernel); fmt.Sprint(got) != fmt.Sprint(wantCommands) {
		t.Fatalf("kernel commands = %v, want %v", got, wantCommands)
	}
	if n := conversationOpenCountCommand(kernel, "open_project"); n != 1 {
		t.Fatalf("open_project count = %d", n)
	}
	if n := conversationOpenCountCommand(kernel, "get_project_state"); n != 1 {
		t.Fatalf("get_project_state count = %d", n)
	}
	if firstString(kernel.commands[0], "file_path") != fixture.rootProject {
		t.Fatalf("open_project file_path = %q", firstString(kernel.commands[0], "file_path"))
	}
	if resp.Result["same_worktree"] != true || resp.Result["target_materialized"] != true || resp.Result["kernel_reloaded"] != true {
		t.Fatalf("result disclosure = %+v", resp.Result)
	}
	if resp.Result["mode"] != "branch_checkout" || resp.Result["active_branch"] != "main" || resp.Result["detached"] != false {
		t.Fatalf("target state disclosure = %+v", resp.Result)
	}
	if resp.Result["active_node_id"] != fixture.rootNodeID || resp.Result["head"] != fixture.rootCommitID {
		t.Fatalf("target head/node disclosure = %+v", resp.Result)
	}
	refresh := mapFromAny(resp.Result["refresh"])
	if refresh["kernel_reloaded"] != true || refresh["project_file_path"] != fixture.rootProject {
		t.Fatalf("refresh = %+v", refresh)
	}
}

// TestConversationOpenSameWorktreeNodeCheckoutReloadsOnce covers the
// node-checkout equivalent for nodes that are not their branch head: the target
// commit is materialized detached and still only one project reload happens.
func TestConversationOpenSameWorktreeNodeCheckoutReloadsOnce(t *testing.T) {
	fixture := newConversationOpenFixture(t)
	if err := os.WriteFile(fixture.rootProject, []byte("root-two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Checkpoint(map[string]any{"project_path": fixture.rootProject, "message": "root two"}); err != nil {
		t.Fatalf("second root checkpoint: %v", err)
	}
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "project_path": fixture.rootProject},
		{"status": "ok", "project_path": fixture.rootProject},
	}}
	h := New(nil, conversationOpenShadow(t, fixture.rootProject), nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.conversation.open",
		Args:      map[string]any{"name": "main", "node_id": fixture.rootNodeID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	if got := conversationOpenReadFile(t, fixture.rootProject); got != "root-one" {
		t.Fatalf("project content after node checkout = %q, want root-one", got)
	}
	wantCommands := []string{"open_project", "get_project_state"}
	if got := conversationOpenKernelCommands(kernel); fmt.Sprint(got) != fmt.Sprint(wantCommands) {
		t.Fatalf("kernel commands = %v, want %v", got, wantCommands)
	}
	if resp.Result["mode"] != "node_checkout" || resp.Result["detached"] != true || resp.Result["active_node_id"] != fixture.rootNodeID {
		t.Fatalf("node checkout disclosure = %+v", resp.Result)
	}
}

// TestConversationOpenCrossWorktreeReloadsOnceWithAutosave covers requirement 3:
// switching to another worktree first runs the existing switch autosave, then
// materializes the target node inside the target copy, then reloads exactly
// once (one open_project + one get_project_state).
func TestConversationOpenCrossWorktreeReloadsOnceWithAutosave(t *testing.T) {
	fixture := newConversationOpenFixture(t)
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "snapshot_xml": "<EDIT root-live=\"yes\"/>", "project_path": fixture.rootProject},
		{"status": "ok", "project_path": fixture.worktreePath},
		{"status": "ok", "project_path": fixture.worktreePath},
	}}
	h := New(nil, conversationOpenShadow(t, fixture.rootProject), nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.conversation.open",
		Args:      map[string]any{"project_file_path": fixture.worktreePath, "node_id": fixture.worktreeNodeID},
		Confirmed: true,
		Source:    "test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("resp = %+v", resp)
	}
	wantCommands := []string{"project.snapshot_export", "open_project", "get_project_state"}
	if got := conversationOpenKernelCommands(kernel); fmt.Sprint(got) != fmt.Sprint(wantCommands) {
		t.Fatalf("kernel commands = %v, want %v", got, wantCommands)
	}
	if n := conversationOpenCountCommand(kernel, "open_project"); n != 1 {
		t.Fatalf("open_project count = %d", n)
	}
	if n := conversationOpenCountCommand(kernel, "get_project_state"); n != 1 {
		t.Fatalf("get_project_state count = %d", n)
	}
	if firstString(kernel.commands[1], "file_path") != fixture.worktreePath {
		t.Fatalf("open_project file_path = %q", firstString(kernel.commands[1], "file_path"))
	}
	if got := conversationOpenReadFile(t, fixture.worktreePath); got != "wt-one" {
		t.Fatalf("target worktree content after conversation open = %q, want wt-one", got)
	}
	if got := conversationOpenReadFile(t, fixture.rootProject); got != "root-one" {
		t.Fatalf("root project content must not be rewritten by autosave: %q", got)
	}
	rootList, err := history.List(map[string]any{"project_path": fixture.rootProject})
	if err != nil {
		t.Fatalf("root list: %v", err)
	}
	foundAutosave := false
	for _, commit := range rootList["commits"].([]history.Commit) {
		if commit.Source == "worktree_checkout" && commit.CheckpointKind == "auto_worktree_switch" &&
			samePath(commit.ProjectPath, fixture.rootProject) {
			foundAutosave = true
			break
		}
	}
	if !foundAutosave {
		t.Fatalf("switch autosave commit missing from root history: %+v", rootList["commits"])
	}
	autosaveID := firstString(resp.Result, "switch_autosave_commit_id")
	if autosaveID == "" {
		t.Fatalf("result did not disclose switch_autosave_commit_id: %+v", resp.Result)
	}
	if resp.Result["same_worktree"] != false || resp.Result["target_materialized"] != true || resp.Result["kernel_reloaded"] != true {
		t.Fatalf("result disclosure = %+v", resp.Result)
	}
	if resp.Result["active_worktree"] != fixture.worktreeName || resp.Result["active_branch"] != "main" {
		t.Fatalf("target worktree state = %+v", resp.Result)
	}
	if resp.Result["active_node_id"] != fixture.worktreeNodeID || resp.Result["head"] != fixture.worktreeCommitID {
		t.Fatalf("target head/node = %+v", resp.Result)
	}
	refresh := mapFromAny(resp.Result["refresh"])
	if refresh["kernel_reloaded"] != true || refresh["project_file_path"] != fixture.worktreePath {
		t.Fatalf("refresh = %+v", refresh)
	}
}

// TestConversationOpenRejectsWhileTurnRunning covers requirement 4: the active
// project-plane checkout guard rejects the composite tool exactly like the
// existing checkout tools, before any Project History or kernel work happens.
func TestConversationOpenRejectsWhileTurnRunning(t *testing.T) {
	h := New(nil, nil, nil)
	kernel := &fakeKernelClient{}
	h.kernel = kernel
	goal := h.BeginGoal("running experiment")
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.conversation.open",
		Args:      map[string]any{"name": "main", "node_id": "node-1"},
		Confirmed: true,
		GoalID:    goal.GoalID,
		RunID:     goal.RunID,
		Source:    "test",
	})
	if err == nil || resp.Status != "error" {
		t.Fatalf("running turn must be rejected: resp=%+v err=%v", resp, err)
	}
	if resp.Result["error_code"] != "checkout_blocked_while_agent_running" {
		t.Fatalf("error_code = %+v", resp.Result)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("blocked turn must not touch the kernel: %+v", kernel.commands)
	}
}

// TestConversationOpenRejectsUnknownWorktreeBranchAndNode covers requirement 4's
// unknown-identity refusals and the pre-reload failure semantics: no kernel
// project reload happens and the result discloses that nothing was materialized.
func TestConversationOpenRejectsUnknownWorktreeBranchAndNode(t *testing.T) {
	fixture := newConversationOpenFixture(t)
	kernel := &fakeKernelClient{}
	h := New(nil, conversationOpenShadow(t, fixture.rootProject), nil)
	h.kernel = kernel

	cases := []struct {
		name        string
		args        map[string]any
		wantMessage string
	}{
		{"unknown worktree", map[string]any{"name": "missing-worktree", "node_id": fixture.rootNodeID}, "worktree not found"},
		{"unknown branch", map[string]any{"name": "main", "node_id": fixture.rootNodeID, "branch": "missing-branch"}, "branch not found"},
		{"unknown node", map[string]any{"name": "main", "node_id": "node_missing"}, "conversation node not found"},
		{"missing identity", map[string]any{"node_id": fixture.rootNodeID}, "requires a target worktree identity"},
		{"missing node", map[string]any{"name": "main"}, "requires non-empty node_id"},
	}
	for _, tc := range cases {
		resp, err := h.Invoke(context.Background(), InvokeRequest{
			Tool:      "version.conversation.open",
			Args:      tc.args,
			Confirmed: true,
			Source:    "test",
		})
		if err == nil || resp.Status != "error" {
			t.Fatalf("%s: resp=%+v err=%v", tc.name, resp, err)
		}
		if !strings.Contains(strings.ToLower(resp.Error), strings.ToLower(tc.wantMessage)) {
			t.Fatalf("%s: error = %q, want %q", tc.name, resp.Error, tc.wantMessage)
		}
		if resp.Result["target_materialized"] != false || resp.Result["kernel_reloaded"] != false {
			t.Fatalf("%s: failure disclosure = %+v", tc.name, resp.Result)
		}
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("validation failures must not reload the current project: %+v", kernel.commands)
	}
	if got := conversationOpenReadFile(t, fixture.rootProject); got != "root-one" {
		t.Fatalf("current project changed by a pre-reload failure: %q", got)
	}
	if got := conversationOpenReadFile(t, fixture.worktreePath); got != "wt-after" {
		t.Fatalf("target worktree changed by a pre-reload failure: %q", got)
	}
}

// TestConversationOpenReloadFailureIsExplicit covers requirement 5: when the
// kernel open_project fails, the tool returns an explicit error that discloses
// the already-materialized target copy and the reload warning instead of
// pretending success.
func TestConversationOpenReloadFailureIsExplicit(t *testing.T) {
	fixture := newConversationOpenFixture(t)
	kernel := &fakeKernelClient{replies: []map[string]any{
		{"status": "error", "message": "open_project refused"},
		{"status": "ok", "project_path": fixture.rootProject},
	}}
	h := New(nil, conversationOpenShadow(t, fixture.rootProject), nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "version.conversation.open",
		Args:      map[string]any{"name": "main", "node_id": fixture.rootNodeID, "branch": "main"},
		Confirmed: true,
		Source:    "test",
	})
	if err == nil || resp.Status != "error" {
		t.Fatalf("reload failure must be explicit: resp=%+v err=%v", resp, err)
	}
	if !strings.Contains(resp.Error, "open_project refused") {
		t.Fatalf("reload failure reason missing: %q", resp.Error)
	}
	if resp.Result["target_materialized"] != true || resp.Result["kernel_reloaded"] != false {
		t.Fatalf("reload failure disclosure = %+v", resp.Result)
	}
	refresh := mapFromAny(resp.Result["refresh"])
	if refresh["kernel_reloaded"] != false || !strings.Contains(fmt.Sprint(refresh["warning"]), "open_project refused") {
		t.Fatalf("refresh disclosure = %+v", refresh)
	}
	wantCommands := []string{"open_project", "get_project_state"}
	if got := conversationOpenKernelCommands(kernel); fmt.Sprint(got) != fmt.Sprint(wantCommands) {
		t.Fatalf("kernel commands = %v, want %v", got, wantCommands)
	}
	if got := conversationOpenReadFile(t, fixture.rootProject); got != "root-one" {
		t.Fatalf("materialization should be disclosed as done: %q", got)
	}
}

// TestConversationOpenIsCatalogedAsConfirmTool proves the new composite seam is
// registered in the tool catalog and keeps the checkout confirmation gate.
func TestConversationOpenIsCatalogedAsConfirmTool(t *testing.T) {
	h := New(nil, nil, nil)
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:   "version.conversation.open",
		Args:   map[string]any{"name": "main", "node_id": "node-1"},
		Source: "test",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if resp.Status != "needs_confirmation" || !resp.RequiresConfirmation || resp.Tool != "version.conversation.open" {
		t.Fatalf("catalog/confirmation gate = %+v", resp)
	}
	if resp.CommandName != "version_conversation_open" || resp.RiskLevel != "confirm" {
		t.Fatalf("catalog metadata = %+v", resp)
	}
}
