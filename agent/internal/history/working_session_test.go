package history

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkingSessionSaveAsForksCurrentHistoryWithoutMutatingSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "B1.vit")
	targetPath := filepath.Join(root, "B2.vit")
	if err := os.WriteFile(sourcePath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_b1_working"
	const targetUUID = "vitproj_b2_working"
	BindProjectIdentity(sourcePath, sourceUUID)
	base, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B1 saved baseline", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	baseCommit := strings.TrimSpace(base["commit_id"].(string))
	if _, err := AppendConversationNode(map[string]any{
		"project_path": sourcePath, "kind": "vit", "commit_id": baseCommit, "text": "B1 complete", "branch": "main",
	}); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentRuntimeState(sourcePath, sourceUUID, []byte(`{"stage":"b1"}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	working, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B2 working", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	workingCommit := strings.TrimSpace(working["commit_id"].(string))
	if _, err := AppendConversationNode(map[string]any{
		"project_path": sourcePath, "kind": "vit", "commit_id": workingCommit, "text": "B2 complete", "branch": "main",
	}); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentRuntimeState(sourcePath, sourceUUID, []byte(`{"stage":"b2","pending":"plan"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := BranchCreate(map[string]any{"project_path": sourcePath, "name": "b2-work", "commit_id": workingCommit}); err != nil {
		t.Fatal(err)
	}
	worktree, err := WorktreeCreate(map[string]any{"project_path": sourcePath, "name": "b2-audition", "commit_id": workingCommit})
	if err != nil {
		t.Fatal(err)
	}
	sourceWorktreePath := fmt.Sprint(worktree["path"])

	canonicalSource, err := canonicalRepo(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(canonicalSource), []string{"B1 complete"})
	if data, err := os.ReadFile(filepath.Join(canonicalSource.StateDir, agentRuntimeStateFile)); err != nil || string(data) != `{"stage":"b1"}` {
		t.Fatalf("source canonical runtime changed before save-as: data=%s err=%v", data, err)
	}

	result, err := ForkWorkingSession(sourcePath, sourceUUID, targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	if result["working_session_forked"] != true {
		t.Fatalf("fork result=%+v", result)
	}
	canonicalSource, _ = canonicalRepo(sourcePath, sourceUUID)
	canonicalTarget, err := canonicalRepo(targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(canonicalSource), []string{"B1 complete"})
	assertConversationTexts(t, readConversationGraphOrDefault(canonicalTarget), []string{"B1 complete", "B2 complete"})
	if currentHead(canonicalSource) != baseCommit {
		t.Fatalf("source HEAD moved: got=%s want=%s", currentHead(canonicalSource), baseCommit)
	}
	if currentHead(canonicalTarget) != workingCommit {
		t.Fatalf("target HEAD=%s want=%s", currentHead(canonicalTarget), workingCommit)
	}
	if _, ok := listBranches(canonicalSource)["b2-work"]; ok {
		t.Fatalf("source inherited B2 branch: %+v", listBranches(canonicalSource))
	}
	if listBranches(canonicalTarget)["b2-work"] != workingCommit || publicActiveBranch(canonicalTarget) != "b2-work" {
		t.Fatalf("target branch state missing: branches=%+v active=%s", listBranches(canonicalTarget), publicActiveBranch(canonicalTarget))
	}
	targetWorktrees := listWorktreeItems(canonicalTarget)
	var targetWorktree map[string]any
	for _, item := range targetWorktrees {
		if item["name"] == "b2-audition" {
			targetWorktree = item
			break
		}
	}
	if targetWorktree == nil || !pathWithin(fmt.Sprint(targetWorktree["path"]), visibleWorktreeRoot(canonicalTarget)) {
		t.Fatalf("target worktree was not copied into target namespace: %+v", targetWorktrees)
	}
	if sameProjectPath(fmt.Sprint(targetWorktree["path"]), sourceWorktreePath) {
		t.Fatalf("target reused source worktree path: source=%s target=%+v", sourceWorktreePath, targetWorktree)
	}
	sourceMeta := map[string]any{}
	if err := readJSON(filepath.Join(sourceWorktreePath, ".vit_worktree.json"), &sourceMeta); err != nil {
		t.Fatal(err)
	}
	if !sameProjectPath(fmt.Sprint(sourceMeta["root_project_path"]), sourcePath) {
		t.Fatalf("source worktree metadata was mutated by Save As: %+v", sourceMeta)
	}
	if data, err := os.ReadFile(filepath.Join(canonicalTarget.StateDir, agentRuntimeStateFile)); err != nil || !strings.Contains(string(data), `"stage": "b2"`) {
		t.Fatalf("target runtime missing working state: data=%s err=%v", data, err)
	}
	if _, err := EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, sourcePath)), []string{"B1 complete"})
}

func TestWorkingSessionSaveCommitsToCurrentProject(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "same.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_same_save"
	BindProjectIdentity(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "working save", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "ask", "commit_id": checkpoint["commit_id"], "text": "saved conversation", "branch": "main"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CommitWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(canonical), []string{"saved conversation"})
}

func TestWorkingSessionRegistryLossStartsFromSavedCanonicalAndLeavesRecoveryDraft(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "recover.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_crash_recovery"
	BindProjectIdentity(projectPath, projectUUID)
	first, err := EnsureWorkingSession(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "unsaved session", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "ask", "commit_id": checkpoint["commit_id"], "text": "recover me"}); err != nil {
		t.Fatal(err)
	}
	bindProjectCanonical(projectPath, projectUUID)
	resumed, err := EnsureWorkingSession(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.SessionID == first.SessionID {
		t.Fatalf("unsaved session was automatically resumed: first=%s resumed=%s", first.SessionID, resumed.SessionID)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, projectPath)), nil)
	recovery, err := readWorkingSession(filepath.Dir(first.WorkspaceDir))
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Status != "recovery_available" {
		t.Fatalf("unsaved session status=%q want recovery_available", recovery.Status)
	}
}

func TestWorkingSessionPrefersRecoveredCanonicalOverEmptyActiveSession(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "recovered.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_recovered_canonical"
	historyPath := BindProjectIdentity(projectPath, projectUUID)
	empty, err := EnsureWorkingSession(historyPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	bindProjectCanonical(projectPath, projectUUID)
	checkpoint, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "recovered baseline", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{
		"project_path": projectPath, "kind": "vit", "commit_id": checkpoint["commit_id"], "text": "recovered history",
	}); err != nil {
		t.Fatal(err)
	}
	bindProjectCanonical(projectPath, projectUUID)

	recovered, err := EnsureWorkingSession(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.SessionID == empty.SessionID {
		t.Fatalf("empty session was resumed over recovered canonical: %+v", recovered)
	}
	old, err := readWorkingSession(filepath.Dir(empty.WorkspaceDir))
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != "recovery_available" {
		t.Fatalf("empty session status=%q", old.Status)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, projectPath)), []string{"recovered history"})
}

func TestSavedGenerationIsOnlyAdvancedAfterSuccessfulCommit(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "saved.vit")
	if err := os.WriteFile(projectPath, []byte("saved project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_saved_generation"
	BindProjectIdentity(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	savedCommit, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "saved chat", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "ask", "commit_id": savedCommit["commit_id"], "text": "saved chat"}); err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareWorkingSessionSave(projectPath, projectUUID, "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPreparedWorkingSession(projectPath, projectUUID, projectPath, projectUUID,
		fmt.Sprint(prepared["prepare_id"]), fmt.Sprint(prepared["generation_id"]), "save"); err != nil {
		t.Fatal(err)
	}
	head, err := SavedHeadForProject(projectPath, projectUUID)
	if err != nil || head.GenerationID != prepared["generation_id"] {
		t.Fatalf("saved head=%+v err=%v prepared=%+v", head, err, prepared)
	}
	unsavedCommit, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "unsaved chat", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "ask", "commit_id": unsavedCommit["commit_id"], "text": "unsaved chat"}); err != nil {
		t.Fatal(err)
	}
	bindProjectCanonical(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, projectPath)), []string{"saved chat"})
}

func TestPreparedButFailedSaveDoesNotAdvanceConversationHistory(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "failed.vit")
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_failed_save"
	BindProjectIdentity(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	commit, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "must stay unsaved", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "ask", "commit_id": commit["commit_id"], "text": "must stay unsaved"}); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareWorkingSessionSave(projectPath, projectUUID, "save"); err != nil {
		t.Fatal(err)
	}
	bindProjectCanonical(projectPath, projectUUID)
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, projectPath)), nil)
	if _, err := SavedHeadForProject(projectPath, projectUUID); !os.IsNotExist(err) {
		t.Fatalf("failed save created Saved HEAD: %v", err)
	}
}

func TestOpenRecoversPreparedSaveAsAfterPostSaveNotificationIsLost(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "B1.vit")
	targetPath := filepath.Join(root, "B2.vit")
	if err := os.WriteFile(sourcePath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_recovery_b1"
	const targetUUID = "vitproj_recovery_b2"
	BindProjectIdentity(sourcePath, sourceUUID)
	if _, err := EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		t.Fatal(err)
	}
	b1, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B1", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": sourcePath, "kind": "vit", "commit_id": b1["commit_id"], "text": "B1 complete"}); err != nil {
		t.Fatal(err)
	}
	b1Prepared, err := PrepareWorkingSessionSave(sourcePath, sourceUUID, "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CommitPreparedWorkingSession(sourcePath, sourceUUID, sourcePath, sourceUUID,
		fmt.Sprint(b1Prepared["prepare_id"]), fmt.Sprint(b1Prepared["generation_id"]), "save"); err != nil {
		t.Fatal(err)
	}
	b2, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B2", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": sourcePath, "kind": "vit", "commit_id": b2["commit_id"], "text": "B2 complete"}); err != nil {
		t.Fatal(err)
	}
	b2Prepared, err := PrepareWorkingSessionSave(sourcePath, sourceUUID, "save_as")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate opening after the Kernel saved the target but before the host
	// could send version.project_saved. The old fallback may already exist.
	if _, err := RecoverMissingProjectFork(sourcePath, sourceUUID, targetPath, targetUUID); err != nil {
		t.Fatal(err)
	}
	recovered, err := RecoverPreparedSaveOnOpen(
		targetPath, targetUUID, sourcePath, sourceUUID,
		fmt.Sprint(b2Prepared["generation_id"]), "",
	)
	if err != nil {
		t.Fatal(err)
	}
	if recovered["recovered"] != true || recovered["recovery_reason"] != "prepared_save_committed_on_open" {
		t.Fatalf("recovery result=%+v", recovered)
	}
	head, err := SavedHeadForProject(targetPath, targetUUID)
	if err != nil || head.GenerationID != fmt.Sprint(b2Prepared["generation_id"]) || head.ProjectUUID != targetUUID {
		t.Fatalf("target head=%+v err=%v prepared=%+v", head, err, b2Prepared)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, targetPath)), []string{"B1 complete", "B2 complete"})
}

func TestRecoverLegacySharedWorkspaceSplitsPostMigrationHistoryIntoChild(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "B1.vit")
	targetPath := filepath.Join(root, "B2.vit")
	if err := os.WriteFile(sourcePath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_legacy_parent"
	const targetUUID = "vitproj_child"
	BindProjectIdentity(sourcePath, sourceUUID)
	b1, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B1", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": sourcePath, "kind": "vit", "commit_id": b1["commit_id"], "text": "B1 complete"}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC()
	sourceCanonical, err := canonicalRepo(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(sourceCanonical.HistoryDir, "workspace.json"), map[string]any{
		"schema_version": "vit_project_workspace.v1", "project_path": sourcePath, "project_uuid": sourceUUID,
		"migrated_from_legacy": true, "migration_completed_at": cutoff,
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	b2, err := Checkpoint(map[string]any{"project_path": sourcePath, "message": "B2", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": sourcePath, "kind": "vit", "commit_id": b2["commit_id"], "text": "B2 complete"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentRuntimeState(sourcePath, sourceUUID, []byte(`{
		"schema_version":"vit_project_agent_runtime.v1",
		"conversations":{"chat_b1":[{"role":"assistant","content":"B1 complete"}],"chat_b2":[{"role":"assistant","content":"B2 complete"}]},
		"conversation_goals":{"chat_b1":"goal_b1","chat_b2":"goal_b2"},
		"pending_static_balance_plans":{"chat_b2":{"plan_id":"b2_pending"}}
	}`)); err != nil {
		t.Fatal(err)
	}

	result, err := RecoverMissingProjectFork("", sourceUUID, targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	if result["legacy_shared_workspace_split"] != true || !dirExists(result["backup_dir"].(string)) {
		t.Fatalf("recovery result=%+v", result)
	}
	sourceCanonical, _ = canonicalRepo(sourcePath, sourceUUID)
	targetCanonical, err := canonicalRepo(targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(sourceCanonical), []string{"B1 complete"})
	assertConversationTexts(t, readConversationGraphOrDefault(targetCanonical), []string{"B1 complete", "B2 complete"})
	sourceRuntime := map[string]any{}
	if err := readJSON(filepath.Join(sourceCanonical.StateDir, agentRuntimeStateFile), &sourceRuntime); err != nil {
		t.Fatal(err)
	}
	sourceConversations, _ := sourceRuntime["conversations"].(map[string]any)
	if len(sourceConversations) != 1 || sourceConversations["chat_b1"] == nil || sourceConversations["chat_b2"] != nil {
		t.Fatalf("B1 runtime was not pruned at migration boundary: %+v", sourceRuntime)
	}
	if sourceRuntime["pending_static_balance_plans"] != nil {
		t.Fatalf("B2 pending state remained in B1: %+v", sourceRuntime)
	}
	if data, err := os.ReadFile(filepath.Join(targetCanonical.StateDir, agentRuntimeStateFile)); err != nil || !strings.Contains(string(data), `"b2_pending"`) {
		t.Fatalf("B2 runtime missing: data=%s err=%v", data, err)
	}
	targetWorkspace := map[string]any{}
	if err := readJSON(filepath.Join(targetCanonical.HistoryDir, "workspace.json"), &targetWorkspace); err != nil {
		t.Fatal(err)
	}
	if !sameProjectPath(fmt.Sprint(targetWorkspace["project_path"]), targetPath) || targetWorkspace["project_uuid"] != targetUUID {
		t.Fatalf("target workspace identity not rewritten: %+v", targetWorkspace)
	}
}

func TestOpenLegacyProjectRepairsSavedBoundaryFromProjectTimestamp(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "B1.vit")
	if err := os.WriteFile(projectPath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_legacy_saved_boundary"
	BindProjectIdentity(projectPath, projectUUID)
	b1, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "B1", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "vit", "commit_id": b1["commit_id"], "text": "B1 complete"}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC()
	if err := os.Chtimes(projectPath, cutoff, cutoff); err != nil {
		t.Fatal(err)
	}
	repo, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(repo.HistoryDir, "workspace.json"), map[string]any{
		"schema_version": "vit_project_workspace.v1", "project_path": "mojibake.vit", "project_uuid": projectUUID,
		"migrated_from_legacy": true, "migration_completed_at": cutoff.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	b2, err := Checkpoint(map[string]any{"project_path": projectPath, "message": "B2", "source": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AppendConversationNode(map[string]any{"project_path": projectPath, "kind": "vit", "commit_id": b2["commit_id"], "text": "B2 complete"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteAgentRuntimeState(projectPath, projectUUID, []byte(`{
		"schema_version":"vit_project_agent_runtime.v1",
		"conversations":{"chat_b1":[{"role":"assistant","content":"B1 complete"}],"chat_b2":[{"role":"assistant","content":"B2 complete"}]}
	}`)); err != nil {
		t.Fatal(err)
	}
	session, err := OpenWorkingSessionAtGeneration(projectPath, projectUUID, "")
	if err != nil {
		t.Fatal(err)
	}
	if session.BaseGeneration == "" {
		t.Fatalf("legacy project did not receive a Saved Generation: %+v", session)
	}
	assertConversationTexts(t, readConversationGraphOrDefault(mustOpenRepo(t, projectPath)), []string{"B1 complete"})
	head, err := SavedHeadForProject(projectPath, projectUUID)
	if err != nil || head.GenerationID != session.BaseGeneration {
		t.Fatalf("saved head=%+v session=%+v err=%v", head, session, err)
	}
	backups, err := filepath.Glob(filepath.Join(root, DirName, legacySavedBoundaryBackupsDirName, projectUUID+"_*"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("legacy repair backup missing: %v err=%v", backups, err)
	}
}

func assertConversationTexts(t *testing.T, graph ConversationGraph, want []string) {
	t.Helper()
	got := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if strings.TrimSpace(node.Text) != "" {
			got = append(got, node.Text)
		}
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("conversation texts=%v want=%v graph=%+v", got, want, graph)
	}
}

func mustOpenRepo(t *testing.T, projectPath string) Repo {
	t.Helper()
	repo, err := Open(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}
