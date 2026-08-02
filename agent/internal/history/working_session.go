package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const workingSessionsDirName = ".sessions"

type WorkingSession struct {
	SchemaVersion  string    `json:"schema_version"`
	SessionID      string    `json:"session_id"`
	ProjectPath    string    `json:"project_path"`
	ProjectUUID    string    `json:"project_uuid"`
	WorkspaceDir   string    `json:"workspace_dir"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	CommittedAt    time.Time `json:"committed_at,omitempty"`
	ForkedToPath   string    `json:"forked_to_path,omitempty"`
	ForkedToUUID   string    `json:"forked_to_uuid,omitempty"`
	BaseGeneration string    `json:"base_generation,omitempty"`
}

func EnsureWorkingSession(projectPath, projectUUID string) (WorkingSession, error) {
	return EnsureWorkingSessionAtGeneration(projectPath, projectUUID, "")
}

func EnsureWorkingSessionAtGeneration(projectPath, projectUUID, generationID string) (WorkingSession, error) {
	return ensureWorkingSessionAtGeneration(projectPath, projectUUID, generationID, false)
}

// OpenWorkingSessionAtGeneration always starts a fresh draft from the saved
// generation. Existing active drafts remain available for explicit recovery.
func OpenWorkingSessionAtGeneration(projectPath, projectUUID, generationID string) (WorkingSession, error) {
	return ensureWorkingSessionAtGeneration(projectPath, projectUUID, generationID, true)
}

func ensureWorkingSessionAtGeneration(projectPath, projectUUID, generationID string, forceNew bool) (WorkingSession, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	if projectPath == "" || strings.TrimSpace(projectUUID) == "" {
		return WorkingSession{}, errors.New("project path and UUID are required for working session")
	}
	if !forceNew {
		binding := boundProjectWorkspace(projectPath)
		if binding.ProjectUUID == safeName(projectUUID) && binding.WorkspaceDir != "" && dirExists(binding.WorkspaceDir) {
			if session, err := readWorkingSession(filepath.Dir(binding.WorkspaceDir)); err == nil && session.Status == "active" {
				return session, nil
			}
		}
	}
	canonical, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return WorkingSession{}, err
	}
	if forceNew && generationID == "" {
		if repairedGeneration, _, _, repairErr := initializeLegacySavedGenerationAtProjectBoundary(canonical); repairErr != nil {
			return WorkingSession{}, repairErr
		} else if repairedGeneration != "" {
			generationID = repairedGeneration
		}
	}
	markRecoverableWorkingSessions(canonical)
	baseDir := canonical.HistoryDir
	if generationID == "" {
		if head, headErr := SavedHeadForProject(projectPath, projectUUID); headErr == nil {
			generationID = head.GenerationID
		}
	}
	if generationID != "" {
		candidate := savedGenerationWorkspace(canonical, generationID)
		if dirExists(candidate) {
			baseDir = candidate
		}
	}
	sessionID := "session_" + time.Now().UTC().Format("20060102T150405") + "_" + shortID()
	sessionRoot := filepath.Join(filepath.Dir(canonical.HistoryDir), workingSessionsDirName, safeName(projectUUID), sessionID)
	workspaceDir := filepath.Join(sessionRoot, "workspace")
	if dirExists(baseDir) {
		if err := copyDir(baseDir, workspaceDir); err != nil {
			return WorkingSession{}, err
		}
	} else if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return WorkingSession{}, err
	}
	now := time.Now().UTC()
	session := WorkingSession{
		SchemaVersion: "vit_project_working_session.v1", SessionID: sessionID,
		ProjectPath: canonical.ProjectPath, ProjectUUID: canonical.ProjectUUID,
		WorkspaceDir: workspaceDir, Status: "active", CreatedAt: now, UpdatedAt: now, BaseGeneration: generationID,
	}
	if err := writeWorkspaceIdentity(workspaceDir, canonical.ProjectPath, canonical.ProjectUUID); err != nil {
		return WorkingSession{}, err
	}
	if err := writeWorkingSession(session); err != nil {
		return WorkingSession{}, err
	}
	bindProjectWorkingSession(projectPath, projectUUID, session)
	return session, nil
}

func markRecoverableWorkingSessions(repo Repo) {
	entries, err := os.ReadDir(workingSessionsRoot(repo))
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, readErr := readWorkingSession(filepath.Join(workingSessionsRoot(repo), entry.Name()))
		if readErr != nil || session.Status != "active" || !sameProjectPath(session.ProjectPath, repo.ProjectPath) {
			continue
		}
		session.Status = "recovery_available"
		session.UpdatedAt = now
		_ = writeWorkingSession(session)
	}
}

func WorkingSessionID(projectPath string) string {
	return boundProjectWorkspace(projectPath).SessionID
}

func CommitWorkingSession(projectPath, projectUUID string) (map[string]any, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	binding := boundProjectWorkspace(projectPath)
	if binding.WorkspaceDir == "" {
		return Status(map[string]any{"project_path": projectPath})
	}
	canonical, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return nil, err
	}
	if err := replaceCanonicalWorkspace(binding.WorkspaceDir, canonical.HistoryDir); err != nil {
		return nil, err
	}
	session, err := readWorkingSession(filepath.Dir(binding.WorkspaceDir))
	if err == nil {
		session.Status = "active"
		session.UpdatedAt = time.Now().UTC()
		session.CommittedAt = session.UpdatedAt
		_ = writeWorkingSession(session)
	}
	commits, _ := listCommits(canonical)
	out := historyState(canonical, len(commits))
	out["status"] = "ok"
	out["working_session_committed"] = true
	out["working_session_id"] = binding.SessionID
	return out, nil
}

func ForkWorkingSession(sourcePath, sourceUUID, targetPath, targetUUID string) (map[string]any, error) {
	if sourceUUID == targetUUID {
		return nil, errors.New("working session fork requires a new target project UUID")
	}
	sourcePath = BindProjectIdentity(sourcePath, sourceUUID)
	if _, err := EnsureWorkingSession(sourcePath, sourceUUID); err != nil {
		return nil, err
	}
	BindProjectIdentity(targetPath, targetUUID)
	result, err := ProjectFork(map[string]any{
		"source_project_path": sourcePath, "source_project_uuid": sourceUUID,
		"project_path": targetPath, "project_uuid": targetUUID,
	})
	if err != nil {
		return nil, err
	}
	sourceBinding := boundProjectWorkspace(sourcePath)
	if sourceBinding.WorkspaceDir != "" {
		if session, readErr := readWorkingSession(filepath.Dir(sourceBinding.WorkspaceDir)); readErr == nil {
			session.Status = "forked"
			session.UpdatedAt = time.Now().UTC()
			session.ForkedToPath = targetPath
			session.ForkedToUUID = safeName(targetUUID)
			_ = writeWorkingSession(session)
		}
	}
	targetSession, err := EnsureWorkingSession(targetPath, targetUUID)
	if err != nil {
		return nil, err
	}
	result["working_session_forked"] = true
	result["working_session_id"] = targetSession.SessionID
	return result, nil
}

func RecoverMissingProjectFork(sourcePath, sourceUUID, targetPath, targetUUID string) (map[string]any, error) {
	if sourceUUID == "" || targetUUID == "" || sourceUUID == targetUUID {
		return nil, errors.New("valid parent and target UUIDs are required for project fork recovery")
	}
	targetCanonical, err := canonicalRepoWithMigration(targetPath, targetUUID, false)
	if err != nil {
		return nil, err
	}
	if historyHasUserData(targetCanonical) {
		return map[string]any{"status": "ok", "recovered": false, "reason": "target_workspace_exists"}, nil
	}
	if strings.TrimSpace(sourcePath) == "" {
		sourcePath = ProjectPathForUUID(targetPath, sourceUUID)
	}
	if strings.TrimSpace(sourcePath) == "" {
		return nil, fmt.Errorf("parent project path not found for UUID %s", sourceUUID)
	}
	sourceCanonical, err := canonicalRepo(sourcePath, sourceUUID)
	if err != nil {
		return nil, err
	}
	metadata := map[string]any{}
	_ = readJSON(filepath.Join(sourceCanonical.HistoryDir, "workspace.json"), &metadata)
	if migrated, _ := metadata["migrated_from_legacy"].(bool); migrated {
		if cutoff, ok := timeValue(metadata["migration_completed_at"]); ok {
			return recoverLegacySharedWorkspace(sourceCanonical, targetCanonical, cutoff)
		}
	}
	return ForkWorkingSession(sourcePath, sourceUUID, targetPath, targetUUID)
}

func ProjectPathForUUID(referenceProjectPath, projectUUID string) string {
	referenceProjectPath = strings.TrimSpace(referenceProjectPath)
	projectUUID = safeName(projectUUID)
	if referenceProjectPath == "" || projectUUID == "" {
		return ""
	}
	abs, err := filepath.Abs(referenceProjectPath)
	if err != nil {
		return ""
	}
	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		metadata := map[string]any{}
		path := filepath.Join(dir, DirName, projectUUID, "workspace.json")
		if err := readJSON(path, &metadata); err == nil {
			if projectPath := strings.TrimSpace(fmt.Sprint(metadata["project_path"])); projectPath != "" {
				return projectPath
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func recoverLegacySharedWorkspace(source, target Repo, cutoff time.Time) (map[string]any, error) {
	historyRoot := filepath.Dir(source.HistoryDir)
	backupDir := filepath.Join(historyRoot, ".legacy_shared_backups", source.ProjectUUID+"_"+time.Now().UTC().Format("20060102T150405"))
	if err := copyDir(source.HistoryDir, backupDir); err != nil {
		return nil, err
	}
	if dirExists(target.HistoryDir) {
		if err := os.RemoveAll(target.HistoryDir); err != nil {
			return nil, err
		}
	}
	if err := copyDir(source.HistoryDir, target.HistoryDir); err != nil {
		return nil, err
	}
	if err := rewriteAdoptedDraftHistory(target, source); err != nil {
		return nil, err
	}
	if err := pruneCanonicalWorkspaceAfter(source, cutoff); err != nil {
		return nil, err
	}
	targetSession, err := EnsureWorkingSession(target.ProjectPath, target.ProjectUUID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "ok", "recovered": true, "legacy_shared_workspace_split": true,
		"source_project_path": source.ProjectPath, "source_project_uuid": source.ProjectUUID,
		"project_path": target.ProjectPath, "project_uuid": target.ProjectUUID,
		"backup_dir": backupDir, "migration_cutoff": cutoff.UTC(), "working_session_id": targetSession.SessionID,
	}, nil
}

func pruneCanonicalWorkspaceAfter(repo Repo, cutoff time.Time) error {
	graph := ConversationGraph{}
	_ = readJSON(filepath.Join(repo.StateDir, conversationGraphFile), &graph)
	commits, err := listCommits(repo)
	if err != nil {
		return err
	}
	kept := make([]Commit, 0, len(commits))
	removedIDs := map[string]bool{}
	for _, commit := range commits {
		if commit.CreatedAt.IsZero() || !commit.CreatedAt.After(cutoff) {
			kept = append(kept, commit)
			continue
		}
		removedIDs[commit.ID] = true
		_ = os.Remove(filepath.Join(repo.HistoryDir, "commits", commit.ID+".json"))
	}
	nodes := make([]ConversationNode, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if (node.CreatedAt.IsZero() || !node.CreatedAt.After(cutoff)) && !removedIDs[node.CommitID] {
			nodes = append(nodes, node)
		}
	}
	graph.Nodes = nodes
	graph.ActiveNodeID = ""
	if len(nodes) > 0 {
		graph.ActiveNodeID = nodes[len(nodes)-1].ID
	}
	if err := writeConversationGraph(repo, graph); err != nil {
		return err
	}
	if err := pruneLegacyAgentRuntime(repo, cutoff, nodes); err != nil {
		return err
	}
	refsRoot := filepath.Join(repo.StateDir, "refs")
	if err := os.RemoveAll(refsRoot); err != nil {
		return err
	}
	latestByBranch := map[string]Commit{}
	var latest Commit
	for _, commit := range kept {
		if latest.ID == "" || commit.CreatedAt.After(latest.CreatedAt) {
			latest = commit
		}
		branch := firstNonEmpty(commit.Branch, rootWorktreeName)
		if current := latestByBranch[branch]; current.ID == "" || commit.CreatedAt.After(current.CreatedAt) {
			latestByBranch[branch] = commit
		}
	}
	for branch, commit := range latestByBranch {
		if err := writeRef(repo, filepath.Join("refs", "heads", branch), commit.ID); err != nil {
			return err
		}
	}
	if latest.ID != "" {
		if err := writeRef(repo, "HEAD", latest.ID); err != nil {
			return err
		}
		if _, ok := latestByBranch[activeBranch(repo)]; !ok {
			_ = writeActiveBranch(repo, firstNonEmpty(latest.Branch, rootWorktreeName), false)
		}
	}
	return nil
}

func pruneLegacyAgentRuntime(repo Repo, cutoff time.Time, nodes []ConversationNode) error {
	runtimePath := filepath.Join(repo.StateDir, agentRuntimeStateFile)
	state := map[string]any{}
	if err := readJSON(runtimePath, &state); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	allowed := map[string]int{}
	for _, node := range nodes {
		role := ""
		switch strings.ToLower(strings.TrimSpace(node.Kind)) {
		case "ask":
			role = "user"
		case "vit":
			role = "assistant"
		}
		content := firstNonEmpty(node.Text, node.TextPreview)
		if role != "" && content != "" {
			allowed[role+"\x00"+content]++
		}
	}
	keptConversations := map[string]any{}
	if conversations, ok := state["conversations"].(map[string]any); ok {
		for conversationID, value := range conversations {
			rows, _ := value.([]any)
			kept := make([]any, 0, len(rows))
			for _, value := range rows {
				row, _ := value.(map[string]any)
				key := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["role"]))) + "\x00" + strings.TrimSpace(fmt.Sprint(row["content"]))
				if allowed[key] <= 0 {
					continue
				}
				allowed[key]--
				kept = append(kept, row)
			}
			if len(kept) > 0 {
				keptConversations[conversationID] = kept
			}
		}
	}
	pruned := map[string]any{
		"schema_version": firstNonEmpty(strings.TrimSpace(fmt.Sprint(state["schema_version"])), "vit_project_agent_runtime.v1"),
		"project_path":   repo.ProjectPath,
		"project_uuid":   repo.ProjectUUID,
		"saved_at":       cutoff.UTC(),
	}
	if len(keptConversations) > 0 {
		pruned["conversations"] = keptConversations
	}
	if goals, ok := state["conversation_goals"].(map[string]any); ok {
		keptGoals := map[string]any{}
		for conversationID := range keptConversations {
			if goal, exists := goals[conversationID]; exists {
				keptGoals[conversationID] = goal
			}
		}
		if len(keptGoals) > 0 {
			pruned["conversation_goals"] = keptGoals
		}
	}
	return writeJSON(runtimePath, pruned)
}

func timeValue(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, !typed.IsZero()
	case string:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(fmt.Sprint(value)))
		return parsed, err == nil
	}
}

func canonicalRepo(projectPath, projectUUID string) (Repo, error) {
	return canonicalRepoWithMigration(projectPath, projectUUID, true)
}

func canonicalRepoWithMigration(projectPath, projectUUID string, migrateLegacy bool) (Repo, error) {
	projectPath = strings.TrimSpace(projectPath)
	projectUUID = safeName(projectUUID)
	if projectPath == "" || projectUUID == "" {
		return Repo{}, errors.New("project path and UUID are required")
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return Repo{}, err
	}
	abs = filepath.Clean(abs)
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	historyRoot := filepath.Join(filepath.Dir(abs), DirName)
	repo := Repo{
		ProjectPath: abs, ProjectUUID: projectUUID, ProjectDir: filepath.Dir(abs),
		MediaDir:   filepath.Join(filepath.Dir(abs), base+"_Media"),
		HistoryDir: filepath.Join(historyRoot, projectUUID),
		StateDir:   filepath.Join(historyRoot, projectUUID, "state"),
		Draft:      IsDraftProjectPath(abs),
	}
	if migrateLegacy {
		if err := migrateLegacyHistoryWorkspace(repo, historyRoot); err != nil {
			return Repo{}, err
		}
	}
	return repo, nil
}

func bindProjectWorkingSession(projectPath, projectUUID string, session WorkingSession) {
	projectIdentityRegistry.Lock()
	projectIdentityRegistry.byPath[normalizeProjectPath(projectPath)] = projectWorkspaceBinding{
		ProjectUUID: safeName(projectUUID), WorkspaceDir: filepath.Clean(session.WorkspaceDir), SessionID: session.SessionID,
	}
	projectIdentityRegistry.Unlock()
}

func bindProjectCanonical(projectPath, projectUUID string) {
	projectIdentityRegistry.Lock()
	projectIdentityRegistry.byPath[normalizeProjectPath(projectPath)] = projectWorkspaceBinding{ProjectUUID: safeName(projectUUID)}
	projectIdentityRegistry.Unlock()
}

func workingSessionsRoot(repo Repo) string {
	return filepath.Join(filepath.Dir(repo.HistoryDir), workingSessionsDirName, safeName(repo.ProjectUUID))
}

func latestResumableWorkingSession(repo Repo) (WorkingSession, bool) {
	entries, err := os.ReadDir(workingSessionsRoot(repo))
	if err != nil {
		return WorkingSession{}, false
	}
	sessions := make([]WorkingSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, err := readWorkingSession(filepath.Join(workingSessionsRoot(repo), entry.Name()))
		if err != nil || session.Status != "active" || !sameProjectPath(session.ProjectPath, repo.ProjectPath) || !dirExists(session.WorkspaceDir) {
			continue
		}
		sessions = append(sessions, session)
	}
	if len(sessions) == 0 {
		return WorkingSession{}, false
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	return sessions[0], true
}

func readWorkingSession(sessionRoot string) (WorkingSession, error) {
	session := WorkingSession{}
	if err := readJSON(filepath.Join(sessionRoot, "session.json"), &session); err != nil {
		return session, err
	}
	return session, nil
}

func writeWorkingSession(session WorkingSession) error {
	if session.SessionID == "" || session.WorkspaceDir == "" {
		return errors.New("invalid working session")
	}
	return writeJSON(filepath.Join(filepath.Dir(session.WorkspaceDir), "session.json"), session)
}

func writeWorkspaceIdentity(workspaceDir, projectPath, projectUUID string) error {
	path := filepath.Join(workspaceDir, "workspace.json")
	metadata := map[string]any{}
	_ = readJSON(path, &metadata)
	metadata["schema_version"] = "vit_project_workspace.v2"
	metadata["project_path"] = projectPath
	metadata["project_uuid"] = safeName(projectUUID)
	metadata["updated_at"] = time.Now().UTC()
	return writeJSON(path, metadata)
}

func replaceCanonicalWorkspace(sourceDir, targetDir string) error {
	sourceDir = filepath.Clean(sourceDir)
	targetDir = filepath.Clean(targetDir)
	if sourceDir == targetDir || !dirExists(sourceDir) {
		return fmt.Errorf("invalid working session workspace: %s", sourceDir)
	}
	root := filepath.Dir(targetDir)
	if !strings.EqualFold(filepath.Clean(filepath.Dir(sourceDir)), filepath.Join(root, workingSessionsDirName, filepath.Base(targetDir))) && !strings.HasPrefix(strings.ToLower(sourceDir), strings.ToLower(filepath.Join(root, workingSessionsDirName)+string(os.PathSeparator))) {
		return errors.New("working session is outside the project history root")
	}
	temp := filepath.Join(root, "."+filepath.Base(targetDir)+".commit_"+shortID())
	if err := copyDir(sourceDir, temp); err != nil {
		return err
	}
	backup := targetDir + ".previous_" + shortID()
	if dirExists(targetDir) {
		if err := os.Rename(targetDir, backup); err != nil {
			_ = os.RemoveAll(temp)
			return err
		}
	}
	if err := os.Rename(temp, targetDir); err != nil {
		if dirExists(backup) {
			_ = os.Rename(backup, targetDir)
		}
		return err
	}
	if dirExists(backup) {
		_ = os.RemoveAll(backup)
	}
	return nil
}
