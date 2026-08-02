package projectpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectworkspace"
)

const (
	SchemaVersion      = "vit_project_package.v1"
	ManifestFile       = "manifest.json"
	PortableDirName    = ".vit_project"
	LoosePackageSuffix = ".vit_project"
	prepareDirName     = ".project_package_prepares"
)

type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	SchemaVersion     string      `json:"schema_version"`
	ProjectUUID       string      `json:"project_uuid"`
	OriginProjectUUID string      `json:"origin_project_uuid,omitempty"`
	ProjectPath       string      `json:"project_path"`
	ProjectFile       string      `json:"project_file"`
	GenerationID      string      `json:"generation_id"`
	SaveKind          string      `json:"save_kind"`
	PackageMode       string      `json:"package_mode"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	Files             []FileEntry `json:"files"`
	AcousticPackages  int         `json:"acoustic_package_count"`
	MixboardSessions  int         `json:"mixboard_session_count"`
	MediaEntries      int         `json:"media_entry_count"`
}

type Prepared struct {
	SchemaVersion    string    `json:"schema_version"`
	PrepareID        string    `json:"prepare_id"`
	GenerationID     string    `json:"generation_id"`
	SaveKind         string    `json:"save_kind"`
	SourcePath       string    `json:"source_project_path"`
	SourceUUID       string    `json:"source_project_uuid"`
	OriginUUID       string    `json:"origin_project_uuid,omitempty"`
	HistoryWorkspace string    `json:"history_workspace"`
	PackageDir       string    `json:"package_dir"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	CommittedAt      time.Time `json:"committed_at,omitempty"`
}

type PrepareOptions struct {
	ProjectPath       string
	ProjectUUID       string
	OriginProjectUUID string
	PrepareID         string
	GenerationID      string
	SaveKind          string
	HistoryWorkspace  string
}

type CommitOptions struct {
	SourcePath     string
	SourceUUID     string
	TargetPath     string
	TargetUUID     string
	PrepareID      string
	GenerationID   string
	SaveKind       string
	PortableFolder bool
}

type RestoreResult struct {
	Status                   string `json:"status"`
	PackagePath              string `json:"package_path,omitempty"`
	ManifestVerified         bool   `json:"manifest_verified"`
	HistoryRestored          bool   `json:"history_restored"`
	AnalysisManifestRestored bool   `json:"analysis_manifest_restored"`
	FeatureSnapshotRestored  bool   `json:"feature_snapshot_restored"`
	L3FeatureLogRestored     bool   `json:"l3_feature_log_restored"`
	AcousticPackagesRestored int    `json:"acoustic_packages_restored"`
	MixboardSessionsRestored int    `json:"mixboard_sessions_restored"`
	Reason                   string `json:"reason,omitempty"`
}

func Prepare(opts PrepareOptions) (Prepared, error) {
	opts.ProjectPath = cleanAbs(opts.ProjectPath)
	opts.ProjectUUID = safeName(opts.ProjectUUID)
	opts.PrepareID = safeName(opts.PrepareID)
	opts.GenerationID = strings.TrimSpace(opts.GenerationID)
	opts.SaveKind = strings.ToLower(strings.TrimSpace(opts.SaveKind))
	opts.HistoryWorkspace = filepath.Clean(strings.TrimSpace(opts.HistoryWorkspace))
	if opts.ProjectPath == "" || opts.ProjectUUID == "" || opts.PrepareID == "" || opts.GenerationID == "" {
		return Prepared{}, errors.New("project package prepare requires project path, UUID, prepare ID and generation ID")
	}
	if opts.SaveKind == "" {
		opts.SaveKind = "save"
	}
	if opts.HistoryWorkspace == "." || !dirExists(opts.HistoryWorkspace) {
		return Prepared{}, errors.New("project package prepare requires the prepared history workspace")
	}
	if strings.TrimSpace(opts.OriginProjectUUID) == "" {
		if _, existing, discoverErr := Discover(opts.ProjectPath, opts.ProjectUUID); discoverErr == nil {
			opts.OriginProjectUUID = firstNonEmpty(existing.OriginProjectUUID, existing.ProjectUUID)
		}
	}
	root := prepareRoot(opts.ProjectPath, opts.PrepareID)
	packageDir := filepath.Join(root, "package")
	if err := removeOwnedTree(root, filepath.Join(filepath.Dir(opts.ProjectPath), ".vit_history", prepareDirName)); err != nil {
		return Prepared{}, err
	}
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return Prepared{}, err
	}
	if err := copyDir(opts.HistoryWorkspace, filepath.Join(packageDir, "history")); err != nil {
		return Prepared{}, err
	}
	copyHistoryStateFiles(opts.HistoryWorkspace, packageDir)

	acousticCount, mediaEntries, err := writeScopedAcousticPackage(packageDir, opts.ProjectPath, opts.ProjectUUID)
	if err != nil {
		return Prepared{}, err
	}
	if err := writeMediaManifest(packageDir, opts.ProjectUUID, mediaEntries); err != nil {
		return Prepared{}, err
	}
	if err := copyFeatureSnapshot(packageDir); err != nil {
		return Prepared{}, err
	}
	if err := copyDerivedAnalysisManifest(packageDir, opts.ProjectPath, opts.ProjectUUID); err != nil {
		return Prepared{}, err
	}
	if err := copyDerivedL3FeatureLog(packageDir, opts.ProjectPath, opts.ProjectUUID); err != nil {
		return Prepared{}, err
	}
	mixboardSessions, err := copyScopedMixboard(packageDir, opts.ProjectUUID)
	if err != nil {
		return Prepared{}, err
	}
	if err := writeObservationIndex(packageDir, opts.ProjectUUID); err != nil {
		return Prepared{}, err
	}

	now := time.Now().UTC()
	manifest := Manifest{
		SchemaVersion: SchemaVersion, ProjectUUID: opts.ProjectUUID,
		OriginProjectUUID: firstNonEmpty(safeName(opts.OriginProjectUUID), opts.ProjectUUID),
		ProjectPath:       opts.ProjectPath, ProjectFile: filepath.Base(opts.ProjectPath),
		GenerationID: opts.GenerationID, SaveKind: opts.SaveKind, PackageMode: "prepared",
		CreatedAt: now, UpdatedAt: now, AcousticPackages: acousticCount,
		MixboardSessions: mixboardSessions, MediaEntries: len(mediaEntries),
	}
	if err := writeManifest(packageDir, manifest); err != nil {
		return Prepared{}, err
	}
	prepared := Prepared{
		SchemaVersion: SchemaVersion, PrepareID: opts.PrepareID, GenerationID: opts.GenerationID,
		SaveKind: opts.SaveKind, SourcePath: opts.ProjectPath, SourceUUID: opts.ProjectUUID,
		OriginUUID: manifest.OriginProjectUUID, HistoryWorkspace: opts.HistoryWorkspace,
		PackageDir: packageDir, Status: "prepared", CreatedAt: now,
	}
	if err := writeJSON(filepath.Join(root, "prepare.json"), prepared); err != nil {
		return Prepared{}, err
	}
	return prepared, nil
}

func Commit(opts CommitOptions) (Manifest, string, error) {
	opts.SourcePath = cleanAbs(opts.SourcePath)
	opts.TargetPath = cleanAbs(opts.TargetPath)
	opts.SourceUUID = safeName(opts.SourceUUID)
	opts.TargetUUID = safeName(opts.TargetUUID)
	opts.PrepareID = safeName(opts.PrepareID)
	if opts.SourcePath == "" || opts.TargetPath == "" || opts.SourceUUID == "" || opts.TargetUUID == "" || opts.PrepareID == "" {
		return Manifest{}, "", errors.New("project package commit requires source and target identities plus prepare ID")
	}
	root := prepareRoot(opts.SourcePath, opts.PrepareID)
	prepared := Prepared{}
	if err := readJSON(filepath.Join(root, "prepare.json"), &prepared); err != nil {
		return Manifest{}, "", fmt.Errorf("project package prepare not found: %w", err)
	}
	if prepared.Status != "prepared" || prepared.SourceUUID != opts.SourceUUID || !samePath(prepared.SourcePath, opts.SourcePath) {
		return Manifest{}, "", errors.New("project package prepare does not match source project")
	}
	if strings.TrimSpace(opts.GenerationID) != "" && opts.GenerationID != prepared.GenerationID {
		return Manifest{}, "", errors.New("project package generation does not match prepared snapshot")
	}
	if !dirExists(prepared.PackageDir) {
		return Manifest{}, "", errors.New("prepared project package is missing")
	}

	targetDir := packagePathForCommit(opts.TargetPath, opts.TargetUUID, opts.PortableFolder)
	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Manifest{}, "", err
	}
	staging := filepath.Join(parent, "."+filepath.Base(targetDir)+".staging_"+shortID())
	if err := copyDir(prepared.PackageDir, staging); err != nil {
		return Manifest{}, "", err
	}
	if err := refreshCommittedHistory(staging, opts.TargetPath, opts.TargetUUID); err != nil {
		_ = os.RemoveAll(staging)
		return Manifest{}, "", err
	}
	if err := rebindPackageJSON(staging, opts.SourcePath, opts.SourceUUID, opts.TargetPath, opts.TargetUUID); err != nil {
		_ = os.RemoveAll(staging)
		return Manifest{}, "", err
	}
	manifest := Manifest{}
	if err := readJSON(filepath.Join(staging, ManifestFile), &manifest); err != nil {
		_ = os.RemoveAll(staging)
		return Manifest{}, "", err
	}
	now := time.Now().UTC()
	manifest.ProjectUUID = opts.TargetUUID
	manifest.OriginProjectUUID = firstNonEmpty(prepared.OriginUUID, opts.SourceUUID)
	manifest.ProjectPath = opts.TargetPath
	manifest.ProjectFile = filepath.Base(opts.TargetPath)
	manifest.GenerationID = prepared.GenerationID
	manifest.SaveKind = firstNonEmpty(strings.ToLower(strings.TrimSpace(opts.SaveKind)), prepared.SaveKind)
	manifest.PackageMode = packageModeForCommit(targetDir, opts.TargetPath, opts.PortableFolder)
	manifest.UpdatedAt = now
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = now
	}
	if err := writeManifest(staging, manifest); err != nil {
		_ = os.RemoveAll(staging)
		return Manifest{}, "", err
	}
	if err := atomicReplaceDir(staging, targetDir); err != nil {
		return Manifest{}, "", err
	}
	prepared.Status = "committed"
	prepared.CommittedAt = now
	_ = writeJSON(filepath.Join(root, "prepare.json"), prepared)
	return manifest, targetDir, nil
}

func Restore(projectPath, projectUUID string) (RestoreResult, error) {
	projectPath = cleanAbs(projectPath)
	projectUUID = safeName(projectUUID)
	if projectPath == "" || projectUUID == "" {
		return RestoreResult{}, errors.New("project package restore requires project path and UUID")
	}
	packageDir, manifest, err := Discover(projectPath, projectUUID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RestoreResult{Status: "missing", Reason: "project_package_not_found"}, nil
		}
		return RestoreResult{}, err
	}
	if err := verifyManifest(packageDir, manifest); err != nil {
		return RestoreResult{Status: "invalid", PackagePath: packageDir, Reason: err.Error()}, nil
	}
	result := RestoreResult{Status: "ok", PackagePath: packageDir, ManifestVerified: true}
	if restored, err := restoreHistoryIfMissing(packageDir, projectPath, projectUUID); err != nil {
		return result, err
	} else {
		result.HistoryRestored = restored
	}
	if restored, err := restoreFeatureSnapshot(packageDir); err != nil {
		return result, err
	} else {
		result.FeatureSnapshotRestored = restored
	}
	if restored, err := restoreAnalysisManifest(packageDir, projectPath, projectUUID); err != nil {
		return result, err
	} else {
		result.AnalysisManifestRestored = restored
	}
	if restored, err := restoreL3FeatureLog(packageDir, projectPath, projectUUID); err != nil {
		return result, err
	} else {
		result.L3FeatureLogRestored = restored
	}
	if count, err := restoreAcousticPackages(packageDir, projectUUID); err != nil {
		return result, err
	} else {
		result.AcousticPackagesRestored = count
	}
	if count, err := restoreMixboard(packageDir, projectUUID); err != nil {
		return result, err
	} else {
		result.MixboardSessionsRestored = count
	}
	return result, nil
}

// InspectLegacy discovers and verifies a v1 project package without restoring
// any of its contents. Agent store v2 uses this for read-only compatibility so
// a legacy sidecar can be reported without becoming a second authority.
func InspectLegacy(projectPath, projectUUID string) (RestoreResult, error) {
	projectPath = cleanAbs(projectPath)
	projectUUID = safeName(projectUUID)
	if projectPath == "" || projectUUID == "" {
		return RestoreResult{}, errors.New("project package inspection requires project path and UUID")
	}
	packageDir, manifest, err := Discover(projectPath, projectUUID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RestoreResult{Status: "missing", Reason: "project_package_not_found"}, nil
		}
		return RestoreResult{}, err
	}
	if err := verifyManifest(packageDir, manifest); err != nil {
		return RestoreResult{Status: "invalid", PackagePath: packageDir, Reason: err.Error()}, nil
	}
	return RestoreResult{
		Status: "legacy_read_only", PackagePath: packageDir, ManifestVerified: true,
		Reason: "legacy_project_package_read_only",
	}, nil
}

func Discover(projectPath, projectUUID string) (string, Manifest, error) {
	projectPath = cleanAbs(projectPath)
	projectUUID = safeName(projectUUID)
	base := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
	projectDir := filepath.Dir(projectPath)
	candidates := []string{
		filepath.Join(projectDir, PortableDirName),
		filepath.Join(projectDir, base+LoosePackageSuffix),
		filepath.Join(projectDir, ".vit_projects", projectUUID),
	}
	// Early "Save As Project Folder" builds placed <name>.vit directly inside
	// the loose package directory <name>.vit_project.  In that layout the
	// project directory itself is the package; looking only for a sibling or a
	// nested .vit_project silently loses the packaged Agent history on reopen.
	if strings.EqualFold(filepath.Base(projectDir), base+LoosePackageSuffix) {
		candidates = append(candidates, projectDir)
	}
	for _, candidate := range candidates {
		manifest := Manifest{}
		if err := readJSON(filepath.Join(candidate, ManifestFile), &manifest); err != nil {
			continue
		}
		if manifest.SchemaVersion == SchemaVersion && safeName(manifest.ProjectUUID) == projectUUID {
			return candidate, manifest, nil
		}
	}
	return "", Manifest{}, os.ErrNotExist
}

func PackagePath(projectPath, projectUUID string, portableFolder bool) string {
	return packagePathForCommit(cleanAbs(projectPath), safeName(projectUUID), portableFolder)
}

func prepareRoot(projectPath, prepareID string) string {
	return filepath.Join(filepath.Dir(cleanAbs(projectPath)), ".vit_history", prepareDirName, safeName(prepareID))
}

func packagePathForCommit(projectPath, projectUUID string, portableFolder bool) string {
	if existing, _, err := Discover(projectPath, projectUUID); err == nil {
		return existing
	}
	if portableFolder {
		return filepath.Join(filepath.Dir(projectPath), PortableDirName)
	}
	base := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
	return filepath.Join(filepath.Dir(projectPath), base+LoosePackageSuffix)
}

func packageModeForCommit(packageDir, projectPath string, portableFolder bool) string {
	if samePath(packageDir, filepath.Join(filepath.Dir(projectPath), PortableDirName)) {
		return "portable_folder"
	}
	if portableFolder {
		return "portable_folder"
	}
	return "loose_sidecar"
}

func writeScopedAcousticPackage(packageDir, projectPath, projectUUID string) (int, []map[string]any, error) {
	snapshot, err := acousticpackage.NewStore("").Read()
	if err != nil {
		return 0, nil, err
	}
	filtered := acousticpackage.Snapshot{SchemaVersion: snapshot.SchemaVersion, UpdatedAt: snapshot.UpdatedAt}
	byIdentity := map[string]int{}
	mediaByPath := map[string]map[string]any{}
	for _, status := range snapshot.Packages {
		if !strings.EqualFold(strings.TrimSpace(status.ProjectID), projectUUID) {
			continue
		}
		byIdentity[acousticStatusKey(status)] = len(filtered.Packages)
		filtered.Packages = append(filtered.Packages, status)
	}
	derived, _, err := projectworkspace.BuildL3AcousticStatuses(projectPath, projectUUID)
	if err != nil {
		return 0, nil, err
	}
	for _, status := range derived {
		key := acousticStatusKey(status)
		if index, ok := byIdentity[key]; ok {
			filtered.Packages[index] = acousticpackage.MergeStatus(filtered.Packages[index], status)
			continue
		}
		byIdentity[key] = len(filtered.Packages)
		filtered.Packages = append(filtered.Packages, status)
	}
	sort.Slice(filtered.Packages, func(i, j int) bool {
		return acousticStatusKey(filtered.Packages[i]) < acousticStatusKey(filtered.Packages[j])
	})
	for _, status := range filtered.Packages {
		path := strings.TrimSpace(status.SourcePath)
		if path == "" {
			continue
		}
		entry := map[string]any{
			"source_path": path, "source_hash": status.SourceHash,
			"source_fingerprint": status.SourceFingerprint, "source_revision": status.SourceRevision,
		}
		if info, statErr := os.Stat(path); statErr == nil {
			entry["size"] = info.Size()
			entry["modified_at"] = info.ModTime().UTC()
			entry["exists"] = true
		} else {
			entry["exists"] = false
		}
		mediaByPath[strings.ToLower(filepath.Clean(path))] = entry
	}
	if filtered.SchemaVersion == "" {
		filtered.SchemaVersion = acousticpackage.SchemaVersion
	}
	if err := writeJSON(filepath.Join(packageDir, "acoustic_packages.json"), filtered); err != nil {
		return 0, nil, err
	}
	media := make([]map[string]any, 0, len(mediaByPath))
	for _, row := range mediaByPath {
		media = append(media, row)
	}
	sort.Slice(media, func(i, j int) bool { return fmt.Sprint(media[i]["source_path"]) < fmt.Sprint(media[j]["source_path"]) })
	return len(filtered.Packages), media, nil
}

func acousticStatusKey(status acousticpackage.Status) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(status.ProjectID)), strings.TrimSpace(status.SessionID),
		strings.TrimSpace(status.TrackID), strings.TrimSpace(status.ClipID),
		strings.ToLower(filepath.Clean(strings.TrimSpace(status.SourcePath))), strings.TrimSpace(status.SourceHash),
		strings.TrimSpace(status.SourceFingerprint), strings.TrimSpace(status.SourceRevision),
		strings.TrimSpace(status.ClipRevision), strings.TrimSpace(status.RenderRevision),
	}, "\x1f")
}

func writeMediaManifest(packageDir, projectUUID string, rows []map[string]any) error {
	return writeJSON(filepath.Join(packageDir, "media_manifest.json"), map[string]any{
		"schema_version": "vit_project_media_manifest.v1", "project_uuid": projectUUID,
		"media_policy": "reference_only", "entries": rows, "updated_at": time.Now().UTC(),
	})
}

func copyFeatureSnapshot(packageDir string) error {
	source := mixboard.FeatureSnapshotPath(nil)
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return writeJSON(filepath.Join(packageDir, "observation_snapshot.json"), map[string]any{"schema_version": "mixboard_feature_snapshot.v1"})
		}
		return err
	}
	return copyFile(source, filepath.Join(packageDir, "observation_snapshot.json"))
}

func copyDerivedAnalysisManifest(packageDir, projectPath, projectUUID string) error {
	source := filepath.Join(filepath.Dir(projectPath), ".vit_derived", safeName(projectUUID), "analysis_manifest.json")
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyFile(source, filepath.Join(packageDir, "analysis_manifest.json"))
}

func copyDerivedL3FeatureLog(packageDir, projectPath, projectUUID string) error {
	source, err := projectworkspace.L3FeatureLogPath(projectPath, projectUUID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyFile(source, filepath.Join(packageDir, projectworkspace.L3FeatureLogFile))
}

func copyScopedMixboard(packageDir, projectUUID string) (int, error) {
	sourceRoot := mixboard.DefaultRoot()
	targetRoot := filepath.Join(packageDir, "mixboard")
	if err := os.MkdirAll(filepath.Join(targetRoot, "sessions"), 0o755); err != nil {
		return 0, err
	}
	decisionSource := filepath.Join(sourceRoot, "_projects", safeName(projectUUID))
	if dirExists(decisionSource) {
		if err := copyDir(decisionSource, filepath.Join(targetRoot, "project_decisions")); err != nil {
			return 0, err
		}
	}
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, writeMixboardIndex(packageDir, projectUUID, nil)
		}
		return 0, err
	}
	sessions := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		source := filepath.Join(sourceRoot, entry.Name())
		if !dirContainsProjectUUID(source, projectUUID) {
			continue
		}
		if err := copyDir(source, filepath.Join(targetRoot, "sessions", entry.Name())); err != nil {
			return 0, err
		}
		sessions = append(sessions, entry.Name())
	}
	sort.Strings(sessions)
	return len(sessions), writeMixboardIndex(packageDir, projectUUID, sessions)
}

func writeMixboardIndex(packageDir, projectUUID string, sessions []string) error {
	index := map[string]any{
		"schema_version": "vit_project_mixboard_index.v1", "project_uuid": projectUUID,
		"sessions": sessions, "session_count": len(sessions), "updated_at": time.Now().UTC(),
	}
	if err := writeJSON(filepath.Join(packageDir, "mixboard.json"), index); err != nil {
		return err
	}
	decisionSource := filepath.Join(packageDir, "mixboard", "project_decisions", "current_decisions.json")
	if _, err := os.Stat(decisionSource); err == nil {
		return copyFile(decisionSource, filepath.Join(packageDir, "decision_ledger.json"))
	}
	return writeJSON(filepath.Join(packageDir, "decision_ledger.json"), map[string]any{
		"schema_version": "mix_decision_board.v1", "project_uuid": projectUUID,
		"decision_count": 0, "decisions": []any{}, "updated_at": time.Now().UTC(),
	})
}

func writeObservationIndex(packageDir, projectUUID string) error {
	refs := make([]string, 0)
	root := filepath.Join(packageDir, "mixboard", "sessions")
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		rel, relErr := filepath.Rel(packageDir, path)
		if relErr == nil {
			refs = append(refs, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(refs)
	return writeJSON(filepath.Join(packageDir, "observation_index.json"), map[string]any{
		"schema_version": "vit_project_observation_index.v1", "project_uuid": projectUUID,
		"feature_snapshot": "observation_snapshot.json", "observation_refs": refs,
		"updated_at": time.Now().UTC(),
	})
}

func refreshCommittedHistory(packageDir, targetPath, targetUUID string) error {
	historySource := filepath.Join(filepath.Dir(targetPath), ".vit_history", safeName(targetUUID))
	if !dirExists(historySource) {
		return nil
	}
	historyTarget := filepath.Join(packageDir, "history")
	if err := removeOwnedTree(historyTarget, packageDir); err != nil {
		return err
	}
	if err := copyDir(historySource, historyTarget); err != nil {
		return err
	}
	copyHistoryStateFiles(historySource, packageDir)
	return nil
}

func copyHistoryStateFiles(historyWorkspace, packageDir string) {
	stateDir := filepath.Join(historyWorkspace, "state")
	for _, name := range []string{"agent_runtime_state.json", "conversation_graph.json"} {
		source := filepath.Join(stateDir, name)
		if _, err := os.Stat(source); err == nil {
			_ = copyFile(source, filepath.Join(packageDir, name))
		} else {
			_ = writeJSON(filepath.Join(packageDir, name), map[string]any{"schema_version": "missing", "reason": "not_present_in_history_workspace"})
		}
	}
}

func rebindPackageJSON(packageDir, sourcePath, sourceUUID, targetPath, targetUUID string) error {
	if err := projectworkspace.RebindL3FeatureLogFile(
		filepath.Join(packageDir, projectworkspace.L3FeatureLogFile),
		sourcePath, sourceUUID, targetPath, targetUUID,
	); err != nil {
		return err
	}
	return filepath.WalkDir(packageDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") || filepath.Base(path) == ManifestFile {
			return err
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var value any
		if json.Unmarshal(data, &value) != nil {
			return nil
		}
		value = rebindValue(value, sourcePath, sourceUUID, targetPath, targetUUID)
		return writeJSON(path, value)
	})
}

func rebindValue(value any, sourcePath, sourceUUID, targetPath, targetUUID string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = rebindValue(child, sourcePath, sourceUUID, targetPath, targetUUID)
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = rebindValue(typed[index], sourcePath, sourceUUID, targetPath, targetUUID)
		}
		return typed
	case string:
		if typed == sourceUUID {
			return targetUUID
		}
		if samePath(typed, sourcePath) {
			return targetPath
		}
	}
	return value
}

func restoreHistoryIfMissing(packageDir, projectPath, projectUUID string) (bool, error) {
	source := filepath.Join(packageDir, "history")
	if !dirExists(source) {
		return false, nil
	}
	target := filepath.Join(filepath.Dir(projectPath), ".vit_history", safeName(projectUUID))
	if historyHasData(target) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	staging := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".restore_"+shortID())
	if err := copyDir(source, staging); err != nil {
		return false, err
	}
	if err := atomicReplaceDir(staging, target); err != nil {
		return false, err
	}
	return true, nil
}

func restoreFeatureSnapshot(packageDir string) (bool, error) {
	source := filepath.Join(packageDir, "observation_snapshot.json")
	if _, err := os.Stat(source); err != nil {
		return false, nil
	}
	return true, atomicCopyFile(source, mixboard.FeatureSnapshotPath(nil))
}

func restoreAnalysisManifest(packageDir, projectPath, projectUUID string) (bool, error) {
	source := filepath.Join(packageDir, "analysis_manifest.json")
	if _, err := os.Stat(source); err != nil {
		return false, nil
	}
	target := filepath.Join(filepath.Dir(projectPath), ".vit_derived", safeName(projectUUID), "analysis_manifest.json")
	return true, atomicCopyFile(source, target)
}

func restoreL3FeatureLog(packageDir, projectPath, projectUUID string) (bool, error) {
	source := filepath.Join(packageDir, projectworkspace.L3FeatureLogFile)
	if _, err := os.Stat(source); err != nil {
		return false, nil
	}
	target, err := projectworkspace.L3FeatureLogPath(projectPath, projectUUID)
	if err != nil {
		return false, err
	}
	return true, atomicCopyFile(source, target)
}

func restoreAcousticPackages(packageDir, projectUUID string) (int, error) {
	packaged := acousticpackage.Snapshot{}
	if err := readJSON(filepath.Join(packageDir, "acoustic_packages.json"), &packaged); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	store := acousticpackage.NewStore("")
	current, err := store.Read()
	if err != nil {
		return 0, err
	}
	kept := make([]acousticpackage.Status, 0, len(current.Packages)+len(packaged.Packages))
	for _, status := range current.Packages {
		if !strings.EqualFold(strings.TrimSpace(status.ProjectID), projectUUID) {
			kept = append(kept, status)
		}
	}
	for _, status := range packaged.Packages {
		if strings.EqualFold(strings.TrimSpace(status.ProjectID), projectUUID) {
			kept = append(kept, status)
		}
	}
	current.SchemaVersion = acousticpackage.SchemaVersion
	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	current.Packages = kept
	return len(packaged.Packages), atomicWriteJSON(store.Path, current)
}

func restoreMixboard(packageDir, projectUUID string) (int, error) {
	targetRoot := mixboard.DefaultRoot()
	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		return 0, err
	}
	count := 0
	sessionsRoot := filepath.Join(packageDir, "mixboard", "sessions")
	entries, _ := os.ReadDir(sessionsRoot)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		source := filepath.Join(sessionsRoot, entry.Name())
		target := filepath.Join(targetRoot, entry.Name())
		staging := filepath.Join(targetRoot, "."+entry.Name()+".restore_"+shortID())
		if err := copyDir(source, staging); err != nil {
			return count, err
		}
		if err := atomicReplaceDir(staging, target); err != nil {
			return count, err
		}
		count++
	}
	decisionSource := filepath.Join(packageDir, "mixboard", "project_decisions")
	if dirExists(decisionSource) {
		target := filepath.Join(targetRoot, "_projects", safeName(projectUUID))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, err
		}
		staging := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".restore_"+shortID())
		if err := copyDir(decisionSource, staging); err != nil {
			return count, err
		}
		if err := atomicReplaceDir(staging, target); err != nil {
			return count, err
		}
	}
	return count, nil
}

func writeManifest(packageDir string, manifest Manifest) error {
	manifest.Files = nil
	entries, err := checksumEntries(packageDir)
	if err != nil {
		return err
	}
	manifest.Files = entries
	return writeJSON(filepath.Join(packageDir, ManifestFile), manifest)
}

func verifyManifest(packageDir string, manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported project package schema %q", manifest.SchemaVersion)
	}
	for _, entry := range manifest.Files {
		path := filepath.Join(packageDir, filepath.FromSlash(entry.Path))
		info, err := os.Stat(path)
		if err != nil || info.Size() != entry.Size {
			return fmt.Errorf("project package file mismatch: %s", entry.Path)
		}
		hash, err := hashFile(path)
		if err != nil || hash != entry.SHA256 {
			return fmt.Errorf("project package checksum mismatch: %s", entry.Path)
		}
	}
	return nil
}

func checksumEntries(root string) ([]FileEntry, error) {
	entries := make([]FileEntry, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Base(path) == ManifestFile {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, FileEntry{Path: filepath.ToSlash(rel), SHA256: hash, Size: info.Size()})
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}

func dirContainsProjectUUID(root, projectUUID string) bool {
	for _, relative := range []string{"current.json", "context_pack.json"} {
		data, err := os.ReadFile(filepath.Join(root, relative))
		if err == nil && strings.Contains(string(data), projectUUID) {
			return true
		}
	}
	// Legacy sessions can predate current.json. Inspect only their newest
	// observation instead of walking every historical payload on each save.
	observations := filepath.Join(root, "observations")
	entries, err := os.ReadDir(observations)
	if err != nil {
		return false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(observations, entry.Name()))
		return readErr == nil && strings.Contains(string(data), projectUUID)
	}
	return false
}

func historyHasData(path string) bool {
	for _, candidate := range []string{"workspace.json", filepath.Join("state", "conversation_graph.json"), "HEAD"} {
		if _, err := os.Stat(filepath.Join(path, candidate)); err == nil {
			return true
		}
	}
	return false
}

func atomicReplaceDir(staging, target string) error {
	staging = filepath.Clean(staging)
	target = filepath.Clean(target)
	if !dirExists(staging) || filepath.Dir(staging) != filepath.Dir(target) {
		return errors.New("project package staging must exist beside its target")
	}
	backup := target + ".previous_" + shortID()
	if dirExists(target) {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(staging, target); err != nil {
		if dirExists(backup) {
			_ = os.Rename(backup, target)
		}
		return err
	}
	if dirExists(backup) {
		_ = os.RemoveAll(backup)
	}
	return nil
}

func atomicCopyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temp := target + ".copy_tmp_" + shortID()
	if err := copyFile(source, temp); err != nil {
		return err
	}
	return atomicReplaceFile(temp, target)
}

func atomicReplaceFile(temp, target string) error {
	backup := target + ".previous_" + shortID()
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backup); err != nil {
			_ = os.Remove(temp)
			return err
		}
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func atomicWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "\t")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp_" + shortID()
	if err := os.WriteFile(temp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return atomicReplaceFile(temp, path)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func copyDir(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func removeOwnedTree(target, ownerRoot string) error {
	target = filepath.Clean(target)
	ownerRoot = filepath.Clean(ownerRoot)
	rel, err := filepath.Rel(ownerRoot, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("refusing to remove project package path outside owned root")
	}
	if dirExists(target) {
		return os.RemoveAll(target)
	}
	return nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cleanAbs(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Clean(abs)
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "..", "_")
	return strings.Trim(value, " .")
}

func samePath(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shortID() string {
	return fmt.Sprintf("%x", time.Now().UTC().UnixNano())
}
