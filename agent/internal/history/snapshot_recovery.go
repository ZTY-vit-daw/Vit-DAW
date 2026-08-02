package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RecoverMatchingProjectHistory repairs a legacy project whose file identity
// predates (or no longer agrees with) its Agent history UUID. It only adopts a
// sibling history when an immutable project snapshot object exactly matches
// the currently opened project file.
func RecoverMatchingProjectHistory(projectPath, projectUUID string) (map[string]any, error) {
	target, err := canonicalRepoWithMigration(projectPath, projectUUID, false)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"status":       "ok",
		"recovered":    false,
		"project_path": target.ProjectPath,
		"project_uuid": target.ProjectUUID,
	}
	if historyHasUserData(target) {
		result["reason"] = "target_history_exists"
		return result, nil
	}
	projectHash, err := hashFile(target.ProjectPath)
	if err != nil {
		return nil, err
	}
	source, commitID, found, err := matchingSiblingHistory(target, projectHash)
	if err != nil {
		return nil, err
	}
	if !found {
		result["reason"] = "matching_snapshot_history_not_found"
		result["project_sha256"] = projectHash
		return result, nil
	}
	backupDir, err := installRecoveredHistory(source.HistoryDir, target.HistoryDir)
	if err != nil {
		return nil, err
	}
	if err := rewriteAdoptedDraftHistory(target, source); err != nil {
		_ = rollbackRecoveredHistory(target.HistoryDir, backupDir)
		return nil, err
	}
	ensureRootWorktreeItem(target)
	commits, _ := listCommits(target)
	result = historyState(target, len(commits))
	result["status"] = "ok"
	result["recovered"] = true
	result["recovery_reason"] = "matching_project_snapshot"
	result["source_project_uuid"] = source.ProjectUUID
	result["source_history_dir"] = source.HistoryDir
	result["matching_commit_id"] = commitID
	result["project_sha256"] = projectHash
	if backupDir != "" {
		result["previous_empty_history_backup"] = backupDir
	}
	return result, nil
}

type snapshotHistoryCandidate struct {
	repo      Repo
	commitID  string
	commitCnt int
	score     int
}

func matchingSiblingHistory(target Repo, projectHash string) (Repo, string, bool, error) {
	historyRoot := filepath.Dir(target.HistoryDir)
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Repo{}, "", false, nil
		}
		return Repo{}, "", false, err
	}
	candidates := make([]snapshotHistoryCandidate, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || strings.EqualFold(entry.Name(), target.ProjectUUID) {
			continue
		}
		historyDir := filepath.Join(historyRoot, entry.Name())
		workspace := map[string]any{}
		_ = readJSON(filepath.Join(historyDir, "workspace.json"), &workspace)
		sourceProjectPath := strings.TrimSpace(fmt.Sprint(workspace["project_path"]))
		if sourceProjectPath == "" || sourceProjectPath == "<nil>" {
			sourceProjectPath = target.ProjectPath
		}
		sourceProjectUUIDText := strings.TrimSpace(fmt.Sprint(workspace["project_uuid"]))
		if sourceProjectUUIDText == "<nil>" {
			sourceProjectUUIDText = ""
		}
		sourceProjectUUID := safeName(sourceProjectUUIDText)
		if sourceProjectUUID == "" {
			sourceProjectUUID = safeName(entry.Name())
		}
		source := Repo{
			ProjectPath: sourceProjectPath,
			ProjectUUID: sourceProjectUUID,
			ProjectDir:  filepath.Dir(sourceProjectPath),
			MediaDir: filepath.Join(filepath.Dir(sourceProjectPath),
				strings.TrimSuffix(filepath.Base(sourceProjectPath), filepath.Ext(sourceProjectPath))+"_Media"),
			HistoryDir: historyDir,
			StateDir:   filepath.Join(historyDir, "state"),
			Draft:      target.Draft,
		}
		commits, readErr := rawHistoryCommits(source)
		if readErr != nil || len(commits) == 0 {
			continue
		}
		matchingCommit := ""
		for _, commit := range commits {
			if strings.EqualFold(strings.TrimSpace(commit.ProjectFile.Hash), projectHash) && verifiedHistoryObject(source, projectHash) {
				matchingCommit = commit.ID
				break
			}
		}
		if matchingCommit == "" {
			continue
		}
		score := len(commits)
		if strings.EqualFold(safeName(entry.Name()), source.ProjectUUID) {
			score += 1000000
		}
		candidates = append(candidates, snapshotHistoryCandidate{repo: source, commitID: matchingCommit, commitCnt: len(commits), score: score})
	}
	if len(candidates) == 0 {
		return Repo{}, "", false, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].commitCnt != candidates[j].commitCnt {
			return candidates[i].commitCnt > candidates[j].commitCnt
		}
		return candidates[i].repo.ProjectUUID < candidates[j].repo.ProjectUUID
	})
	return candidates[0].repo, candidates[0].commitID, true, nil
}

func rawHistoryCommits(repo Repo) ([]Commit, error) {
	dir := filepath.Join(repo.HistoryDir, "commits")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	commits := make([]Commit, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		commit := Commit{}
		if err := readJSON(filepath.Join(dir, entry.Name()), &commit); err == nil && strings.TrimSpace(commit.ID) != "" {
			commits = append(commits, commit)
		}
	}
	sort.Slice(commits, func(i, j int) bool { return commits[i].CreatedAt.After(commits[j].CreatedAt) })
	return commits, nil
}

func verifiedHistoryObject(repo Repo, expectedHash string) bool {
	path := objectPath(repo, expectedHash)
	actual, err := hashFile(path)
	return err == nil && strings.EqualFold(actual, expectedHash)
}

func installRecoveredHistory(sourceDir, targetDir string) (string, error) {
	parent := filepath.Dir(targetDir)
	staging := filepath.Join(parent, ".snapshot_recovery_staging_"+safeName(filepath.Base(targetDir))+"_"+shortID())
	if err := copyDir(sourceDir, staging); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	backupDir := ""
	if dirExists(targetDir) {
		backupRoot := filepath.Join(parent, ".snapshot_recovery_backups")
		if err := os.MkdirAll(backupRoot, 0o755); err != nil {
			_ = os.RemoveAll(staging)
			return "", err
		}
		backupDir = filepath.Join(backupRoot, safeName(filepath.Base(targetDir))+"_"+shortID())
		if err := os.Rename(targetDir, backupDir); err != nil {
			_ = os.RemoveAll(staging)
			return "", err
		}
	}
	if err := os.Rename(staging, targetDir); err != nil {
		if backupDir != "" {
			_ = os.Rename(backupDir, targetDir)
		}
		_ = os.RemoveAll(staging)
		return "", err
	}
	return backupDir, nil
}

func rollbackRecoveredHistory(targetDir, backupDir string) error {
	failedDir := targetDir + ".failed_" + shortID()
	if dirExists(targetDir) {
		if err := os.Rename(targetDir, failedDir); err != nil {
			return err
		}
	}
	if backupDir != "" && dirExists(backupDir) {
		if err := os.Rename(backupDir, targetDir); err != nil {
			_ = os.Rename(failedDir, targetDir)
			return err
		}
	}
	return nil
}
