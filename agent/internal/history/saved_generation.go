package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const savedGenerationsDirName = ".saved_generations"
const savePreparesDirName = ".save_prepares"
const savedHeadFileName = "SAVED_HEAD.json"
const legacySavedBoundaryBackupsDirName = ".legacy_saved_boundary_backups"

type SavedHead struct {
	SchemaVersion     string    `json:"schema_version"`
	ProjectPath       string    `json:"project_path"`
	ProjectUUID       string    `json:"project_uuid"`
	GenerationID      string    `json:"generation_id"`
	HistoryHead       string    `json:"history_head,omitempty"`
	SavedAt           time.Time `json:"saved_at"`
	ProjectSHA256     string    `json:"project_sha256,omitempty"`
	ProjectSize       int64     `json:"project_size,omitempty"`
	ProjectModifiedAt time.Time `json:"project_modified_at,omitempty"`
}

type PreparedSave struct {
	SchemaVersion string    `json:"schema_version"`
	PrepareID     string    `json:"prepare_id"`
	GenerationID  string    `json:"generation_id"`
	SaveKind      string    `json:"save_kind"`
	ProjectPath   string    `json:"project_path"`
	ProjectUUID   string    `json:"project_uuid"`
	SessionID     string    `json:"session_id,omitempty"`
	WorkspaceDir  string    `json:"workspace_dir"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	CommittedAt   time.Time `json:"committed_at,omitempty"`
}

// PreparedSaveWorkspace returns the immutable history workspace frozen by a
// successful save-prepare. Project-package persistence uses this exact draft
// boundary so the portable Agent state and the .vit file share one generation.
func PreparedSaveWorkspace(projectPath, projectUUID, prepareID string) (string, error) {
	prepareID = strings.TrimSpace(prepareID)
	if prepareID == "" {
		return "", errors.New("history save prepare_id is required")
	}
	repo, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return "", err
	}
	prepared := PreparedSave{}
	path := filepath.Join(filepath.Dir(repo.HistoryDir), savePreparesDirName, safeName(prepareID), "prepare.json")
	if err := readJSON(path, &prepared); err != nil {
		return "", err
	}
	if prepared.Status != "prepared" || prepared.ProjectUUID != repo.ProjectUUID || !sameProjectPath(prepared.ProjectPath, repo.ProjectPath) {
		return "", errors.New("history save prepare does not match project identity")
	}
	if !dirExists(prepared.WorkspaceDir) {
		return "", errors.New("prepared history workspace is missing")
	}
	return prepared.WorkspaceDir, nil
}

func PrepareWorkingSessionSave(projectPath, projectUUID, saveKind string) (map[string]any, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	if projectPath == "" || strings.TrimSpace(projectUUID) == "" {
		return nil, errors.New("project path and UUID are required to prepare history save")
	}
	if _, err := EnsureWorkingSession(projectPath, projectUUID); err != nil {
		return nil, err
	}
	binding := boundProjectWorkspace(projectPath)
	if binding.WorkspaceDir == "" || !dirExists(binding.WorkspaceDir) {
		return nil, errors.New("active working session is required to prepare history save")
	}
	now := time.Now().UTC()
	generationID := "save_" + now.Format("20060102T150405") + "_" + shortID()
	prepareID := "prepare_" + shortID()
	repo, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return nil, err
	}
	prepareRoot := filepath.Join(filepath.Dir(repo.HistoryDir), savePreparesDirName, prepareID)
	workspaceDir := filepath.Join(prepareRoot, "workspace")
	if err := copyDir(binding.WorkspaceDir, workspaceDir); err != nil {
		return nil, err
	}
	prepared := PreparedSave{
		SchemaVersion: "vit_project_history_save_prepare.v1", PrepareID: prepareID,
		GenerationID: generationID, SaveKind: strings.ToLower(strings.TrimSpace(saveKind)),
		ProjectPath: repo.ProjectPath, ProjectUUID: repo.ProjectUUID,
		SessionID: binding.SessionID, WorkspaceDir: workspaceDir,
		Status: "prepared", CreatedAt: now,
	}
	if prepared.SaveKind == "" {
		prepared.SaveKind = "save"
	}
	if err := writeJSON(filepath.Join(prepareRoot, "prepare.json"), prepared); err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "ok", "prepared": true, "prepare_id": prepareID,
		"generation_id": generationID, "agent_history_generation": generationID,
		"project_path": repo.ProjectPath, "project_uuid": repo.ProjectUUID,
		"save_kind": prepared.SaveKind, "working_session_id": binding.SessionID,
	}, nil
}

func CommitPreparedWorkingSession(sourcePath, sourceUUID, targetPath, targetUUID, prepareID, generationID, saveKind string) (map[string]any, error) {
	prepareID = strings.TrimSpace(prepareID)
	generationID = strings.TrimSpace(generationID)
	if prepareID == "" {
		return nil, errors.New("history save prepare_id is required")
	}
	sourceRepo, err := canonicalRepo(sourcePath, sourceUUID)
	if err != nil {
		return nil, err
	}
	prepareRoot := filepath.Join(filepath.Dir(sourceRepo.HistoryDir), savePreparesDirName, safeName(prepareID))
	prepared := PreparedSave{}
	if err := readJSON(filepath.Join(prepareRoot, "prepare.json"), &prepared); err != nil {
		return nil, fmt.Errorf("history save prepare not found: %w", err)
	}
	if prepared.Status != "prepared" || !sameProjectPath(prepared.ProjectPath, sourceRepo.ProjectPath) || prepared.ProjectUUID != sourceRepo.ProjectUUID {
		return nil, errors.New("history save prepare does not match source project")
	}
	if generationID == "" {
		generationID = prepared.GenerationID
	}
	if generationID != prepared.GenerationID {
		return nil, errors.New("history generation does not match prepared save")
	}
	if !dirExists(prepared.WorkspaceDir) {
		return nil, errors.New("prepared history workspace is missing")
	}
	saveKind = strings.ToLower(strings.TrimSpace(saveKind))
	if saveKind == "" {
		saveKind = prepared.SaveKind
	}
	forkTarget := saveKind == "save_as" || saveKind == "save_as_folder"
	sourceRemainsActive := saveKind == "save_as_folder"
	if forkTarget {
		if targetPath == "" || targetUUID == "" || targetUUID == sourceUUID {
			return nil, errors.New("forked history commit requires a new target identity")
		}
	} else {
		targetPath, targetUUID = sourcePath, sourceUUID
	}
	targetRepo, err := canonicalRepoWithMigration(targetPath, targetUUID, false)
	if err != nil {
		return nil, err
	}
	if err := replaceWorkspaceFromPrepared(prepared.WorkspaceDir, targetRepo.HistoryDir); err != nil {
		return nil, err
	}
	preparedRepo := sourceRepo
	preparedRepo.HistoryDir = prepared.WorkspaceDir
	preparedRepo.StateDir = filepath.Join(prepared.WorkspaceDir, "state")
	if forkTarget {
		if err := adoptVisibleWorktrees(targetRepo, preparedRepo); err != nil {
			return nil, err
		}
		if err := rewriteAdoptedDraftHistory(targetRepo, preparedRepo); err != nil {
			return nil, err
		}
	} else if err := writeWorkspaceIdentity(targetRepo.HistoryDir, targetRepo.ProjectPath, targetRepo.ProjectUUID); err != nil {
		return nil, err
	}
	head, err := createSavedGeneration(targetRepo, generationID)
	if err != nil {
		return nil, err
	}
	prepared.Status = "committed"
	prepared.CommittedAt = head.SavedAt
	_ = writeJSON(filepath.Join(prepareRoot, "prepare.json"), prepared)
	if session, readErr := readWorkingSession(filepath.Dir(boundProjectWorkspace(sourcePath).WorkspaceDir)); readErr == nil {
		switch {
		case saveKind == "save_as":
			session.Status = "forked"
			session.ForkedToPath = targetRepo.ProjectPath
			session.ForkedToUUID = targetRepo.ProjectUUID
		case sourceRemainsActive:
			// Folder export is a snapshot, not a project switch. Keep the exact
			// source draft active so subsequent Agent turns and another folder
			// export continue from the same conversation/runtime workspace.
			session.Status = "active"
			session.ForkedToPath = ""
			session.ForkedToUUID = ""
		default:
			session.Status = "active"
			session.CommittedAt = head.SavedAt
		}
		session.UpdatedAt = head.SavedAt
		_ = writeWorkingSession(session)
	}
	bindProjectCanonical(targetRepo.ProjectPath, targetRepo.ProjectUUID)
	targetSession, err := EnsureWorkingSessionAtGeneration(targetRepo.ProjectPath, targetRepo.ProjectUUID, generationID)
	if err != nil {
		return nil, err
	}
	commits, _ := listCommits(targetRepo)
	out := historyState(targetRepo, len(commits))
	out["status"] = "ok"
	out["working_session_committed"] = true
	out["prepare_id"] = prepareID
	out["generation_id"] = generationID
	out["agent_history_generation"] = generationID
	out["working_session_id"] = targetSession.SessionID
	if forkTarget {
		out["working_session_forked"] = true
		out["source_project_path"] = sourceRepo.ProjectPath
		out["source_project_uuid"] = sourceRepo.ProjectUUID
	}
	if sourceRemainsActive {
		out["active_project_unchanged"] = true
		out["source_working_session_id"] = WorkingSessionID(sourceRepo.ProjectPath)
	}
	return out, nil
}

// RecoverPreparedSaveOnOpen closes the crash window between a successful
// project-file save and the host's post-save Agent notification. The embedded
// generation is authoritative; only an exact prepared snapshot is promoted.
func RecoverPreparedSaveOnOpen(targetPath, targetUUID, sourcePath, sourceUUID, generationID, prepareID string) (map[string]any, error) {
	targetPath = strings.TrimSpace(targetPath)
	targetUUID = strings.TrimSpace(targetUUID)
	generationID = strings.TrimSpace(generationID)
	prepareID = strings.TrimSpace(prepareID)
	if targetPath == "" || targetUUID == "" || generationID == "" {
		return map[string]any{"status": "ok", "recovered": false, "reason": "recovery_identity_incomplete"}, nil
	}
	if head, err := SavedHeadForProject(targetPath, targetUUID); err == nil &&
		head.GenerationID == generationID && head.ProjectUUID == targetUUID && sameProjectPath(head.ProjectPath, targetPath) {
		return map[string]any{"status": "ok", "recovered": false, "reason": "generation_already_committed", "generation_id": generationID}, nil
	}
	if sourcePath == "" && sourceUUID != "" {
		sourcePath = ProjectPathForUUID(targetPath, sourceUUID)
	}
	prepareRoots := []string{filepath.Join(filepath.Dir(targetPath), DirName, savePreparesDirName)}
	if sourcePath != "" && !sameProjectPath(sourcePath, targetPath) {
		prepareRoots = append(prepareRoots, filepath.Join(filepath.Dir(sourcePath), DirName, savePreparesDirName))
	}
	prepared, err := findPreparedSave(prepareRoots, prepareID, generationID, sourceUUID)
	if err != nil {
		return nil, err
	}
	if prepared.PrepareID == "" {
		return map[string]any{"status": "ok", "recovered": false, "reason": "matching_prepare_not_found", "generation_id": generationID}, nil
	}
	saveKind := prepared.SaveKind
	if prepared.ProjectUUID != targetUUID {
		saveKind = "save_as"
	}
	result, err := CommitPreparedWorkingSession(
		prepared.ProjectPath, prepared.ProjectUUID, targetPath, targetUUID,
		prepared.PrepareID, prepared.GenerationID, saveKind,
	)
	if result != nil {
		result["recovered"] = err == nil
		result["recovery_reason"] = "prepared_save_committed_on_open"
	}
	return result, err
}

func findPreparedSave(roots []string, prepareID, generationID, sourceUUID string) (PreparedSave, error) {
	seen := map[string]bool{}
	for _, root := range roots {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "." || seen[strings.ToLower(root)] {
			continue
		}
		seen[strings.ToLower(root)] = true
		if prepareID != "" {
			prepared := PreparedSave{}
			if err := readJSON(filepath.Join(root, safeName(prepareID), "prepare.json"), &prepared); err == nil {
				if prepared.Status == "prepared" && prepared.GenerationID == generationID && (sourceUUID == "" || prepared.ProjectUUID == sourceUUID) {
					return prepared, nil
				}
			}
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return PreparedSave{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			prepared := PreparedSave{}
			if err := readJSON(filepath.Join(root, entry.Name(), "prepare.json"), &prepared); err != nil {
				continue
			}
			if prepared.Status == "prepared" && prepared.GenerationID == generationID && (sourceUUID == "" || prepared.ProjectUUID == sourceUUID) {
				return prepared, nil
			}
		}
	}
	return PreparedSave{}, nil
}

func SavedHeadForProject(projectPath, projectUUID string) (SavedHead, error) {
	repo, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return SavedHead{}, err
	}
	head := SavedHead{}
	if err := readJSON(filepath.Join(repo.HistoryDir, savedHeadFileName), &head); err != nil {
		return SavedHead{}, err
	}
	return head, nil
}

// RepairLegacyProjectSavedBoundary converts a migrated legacy workspace into
// the first immutable Saved Generation using the .vit file's save timestamp.
func RepairLegacyProjectSavedBoundary(projectPath, projectUUID string) (map[string]any, error) {
	projectPath = BindProjectIdentity(projectPath, projectUUID)
	repo, err := canonicalRepo(projectPath, projectUUID)
	if err != nil {
		return nil, err
	}
	generationID, backupDir, repaired, err := initializeLegacySavedGenerationAtProjectBoundary(repo)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "ok", "repaired": repaired, "project_path": repo.ProjectPath,
		"project_uuid": repo.ProjectUUID, "generation_id": generationID,
		"agent_history_generation": generationID, "backup_dir": backupDir,
	}, nil
}

func initializeLegacySavedGenerationAtProjectBoundary(repo Repo) (string, string, bool, error) {
	if head, err := SavedHeadForProject(repo.ProjectPath, repo.ProjectUUID); err == nil {
		return head.GenerationID, "", false, nil
	}
	metadata := map[string]any{}
	if err := readJSON(filepath.Join(repo.HistoryDir, "workspace.json"), &metadata); err != nil {
		return "", "", false, nil
	}
	migrated, _ := metadata["migrated_from_legacy"].(bool)
	if !migrated {
		return "", "", false, nil
	}
	info, err := os.Stat(repo.ProjectPath)
	if err != nil {
		return "", "", false, err
	}
	cutoff := info.ModTime().UTC()
	historyRoot := filepath.Dir(repo.HistoryDir)
	backupDir := filepath.Join(historyRoot, legacySavedBoundaryBackupsDirName,
		repo.ProjectUUID+"_"+time.Now().UTC().Format("20060102T150405"))
	if err := copyDir(repo.HistoryDir, filepath.Join(backupDir, "canonical")); err != nil {
		return "", "", false, err
	}
	sessionsRoot := filepath.Join(historyRoot, workingSessionsDirName, safeName(repo.ProjectUUID))
	if dirExists(sessionsRoot) {
		if err := copyDir(sessionsRoot, filepath.Join(backupDir, "sessions")); err != nil {
			return "", "", false, err
		}
	}
	if err := pruneCanonicalWorkspaceAfter(repo, cutoff); err != nil {
		return "", backupDir, false, err
	}
	if err := writeWorkspaceIdentity(repo.HistoryDir, repo.ProjectPath, repo.ProjectUUID); err != nil {
		return "", backupDir, false, err
	}
	generationID := "legacy_save_" + cutoff.Format("20060102T150405") + "_" + shortID()
	if _, err := createSavedGeneration(repo, generationID); err != nil {
		return "", backupDir, false, err
	}
	return generationID, backupDir, true, nil
}

func savedGenerationWorkspace(repo Repo, generationID string) string {
	return filepath.Join(filepath.Dir(repo.HistoryDir), savedGenerationsDirName, safeName(repo.ProjectUUID), safeName(generationID), "workspace")
}

func createSavedGeneration(repo Repo, generationID string) (SavedHead, error) {
	if generationID == "" {
		return SavedHead{}, errors.New("generation ID is required")
	}
	now := time.Now().UTC()
	head := SavedHead{
		SchemaVersion: "vit_project_saved_head.v1", ProjectPath: repo.ProjectPath,
		ProjectUUID: repo.ProjectUUID, GenerationID: generationID,
		HistoryHead: currentHead(repo), SavedAt: now,
	}
	if info, err := os.Stat(repo.ProjectPath); err == nil {
		head.ProjectSize = info.Size()
		head.ProjectModifiedAt = info.ModTime().UTC()
		if hash, hashErr := hashFile(repo.ProjectPath); hashErr == nil {
			head.ProjectSHA256 = hash
		}
	}
	if err := writeJSON(filepath.Join(repo.HistoryDir, savedHeadFileName), head); err != nil {
		return SavedHead{}, err
	}
	generationWorkspace := savedGenerationWorkspace(repo, generationID)
	if dirExists(generationWorkspace) {
		return SavedHead{}, fmt.Errorf("saved generation already exists: %s", generationID)
	}
	if err := copyDir(repo.HistoryDir, generationWorkspace); err != nil {
		return SavedHead{}, err
	}
	return head, nil
}

func replaceWorkspaceFromPrepared(sourceDir, targetDir string) error {
	if !dirExists(sourceDir) {
		return errors.New("prepared workspace is missing")
	}
	root := filepath.Dir(targetDir)
	temp := filepath.Join(root, "."+filepath.Base(targetDir)+".save_"+shortID())
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
