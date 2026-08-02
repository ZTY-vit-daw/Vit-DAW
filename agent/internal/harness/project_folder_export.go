package harness

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/projectpackage"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
)

func (h *Harness) saveAsFolder(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, errors.New("save_as_folder requires a kernel connection")
	}
	sourcePath, sourceUUID := h.CurrentProjectIdentity(ctx)
	if roots, ok := projectstore.Current(); ok {
		if strings.TrimSpace(sourcePath) == "" {
			sourcePath = roots.ProjectPath
		}
		if strings.TrimSpace(sourceUUID) == "" {
			sourceUUID = roots.ProjectUUID
		}
	}
	if strings.TrimSpace(sourcePath) == "" || strings.TrimSpace(sourceUUID) == "" {
		return nil, errors.New("save_as_folder requires an active project identity")
	}
	targetDir := strings.TrimSpace(firstString(cmd, "directory_path", "folder_path", "target_directory"))
	if targetDir == "" {
		return nil, errors.New("save_as_folder requires directory_path")
	}
	targetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, err
	}
	targetDir = filepath.Clean(targetDir)
	if info, statErr := os.Stat(targetDir); statErr == nil {
		if info.IsDir() {
			return nil, errors.New("save_as_folder target directory already exists")
		}
		return nil, errors.New("save_as_folder target path is not a directory")
	} else if !os.IsNotExist(statErr) {
		return nil, statErr
	}
	projectFileName := strings.TrimSpace(firstString(cmd, "project_file_name"))
	if projectFileName == "" {
		projectFileName = filepath.Base(sourcePath)
	}
	projectFileName = filepath.Base(projectFileName)
	if !strings.EqualFold(filepath.Ext(projectFileName), ".vit") {
		projectFileName = strings.TrimSuffix(projectFileName, filepath.Ext(projectFileName)) + ".vit"
	}
	logicalTargetPath := filepath.Join(targetDir, projectFileName)
	mediaPolicy := strings.ToLower(strings.TrimSpace(firstString(cmd, "media_policy")))

	preflight, _, preflightErr := h.kernel.SendCommand(ctx, map[string]any{
		"cmd": "save_project_copy", "file_path": logicalTargetPath, "preflight_only": true,
		"media_policy": mediaPolicy,
	})
	if preflightErr != nil {
		return nil, preflightErr
	}
	if strings.EqualFold(firstString(preflight, "status"), "error") {
		return preflight, fmt.Errorf("%s", firstNonEmpty(firstString(preflight, "message", "error"), "folder export preflight failed"))
	}
	referencedAudioCount := int(numberFromAny(preflight["referenced_audio_count"]))
	if referencedAudioCount > 0 && mediaPolicy == "" {
		preflight["status"] = "require_media_policy"
		preflight["directory_path"] = targetDir
		preflight["project_path"] = logicalTargetPath
		return preflight, nil
	}
	if mediaPolicy == "" {
		mediaPolicy = projectpackage.MediaPolicyReferenceOnly
	}
	if mediaPolicy != projectpackage.MediaPolicyReferenceOnly && mediaPolicy != projectpackage.MediaPolicyCopyReferenced {
		return nil, fmt.Errorf("unsupported media_policy %q", mediaPolicy)
	}
	if mediaPolicy == projectpackage.MediaPolicyCopyReferenced && int(numberFromAny(preflight["missing_audio_count"])) > 0 {
		preflight["status"] = "media_missing"
		return preflight, errors.New("referenced audio is missing; complete folder export was not created")
	}

	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	stagingDir := filepath.Join(parent, "."+filepath.Base(targetDir)+".staging_"+randomID())
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(stagingDir)
		}
	}()
	stagingProjectPath := filepath.Join(stagingDir, projectFileName)
	prepared, err := history.PrepareWorkingSessionSave(sourcePath, sourceUUID, "save_as_folder")
	if err != nil {
		return nil, err
	}
	prepareID := firstString(prepared, "prepare_id")
	generationID := firstString(prepared, "agent_history_generation", "generation_id")
	snapshot, _, snapshotErr := h.kernel.SendCommand(ctx, map[string]any{
		"cmd": "save_project_copy", "file_path": logicalTargetPath, "write_path": stagingProjectPath,
		"media_policy": mediaPolicy, "history_prepare_id": prepareID, "agent_history_generation": generationID,
	})
	if snapshotErr != nil {
		return nil, snapshotErr
	}
	status := strings.ToLower(firstString(snapshot, "status"))
	if status != "ok" {
		return snapshot, fmt.Errorf("%s", firstNonEmpty(firstString(snapshot, "message", "error"), "folder project snapshot failed"))
	}
	targetUUID := firstString(snapshot, "project_uuid", "project_id")
	if targetUUID == "" || strings.EqualFold(targetUUID, sourceUUID) {
		return snapshot, errors.New("folder project snapshot did not create a new project UUID")
	}
	if !boolValueDefault(snapshot["active_project_unchanged"], false) {
		return snapshot, errors.New("kernel did not preserve the active source project during folder export")
	}
	workspace, err := history.CommitPreparedWorkingSession(
		sourcePath, sourceUUID, stagingProjectPath, targetUUID,
		prepareID, generationID, "save_as",
	)
	if err != nil {
		return workspace, err
	}
	if _, err := projectworkspace.ForkDerived(sourcePath, sourceUUID, stagingProjectPath, targetUUID); err != nil {
		return workspace, err
	}
	if _, err := projectstore.ForkAgentStore(sourcePath, sourceUUID, stagingProjectPath, targetUUID); err != nil {
		return workspace, err
	}
	media := exportMediaSources(snapshot["media"])
	manifest, err := projectpackage.PublishFolderV2(projectpackage.PublishFolderOptions{
		SourceProjectPath: sourcePath, SourceProjectUUID: sourceUUID,
		TargetProjectPath: logicalTargetPath, TargetProjectUUID: targetUUID,
		StagingProjectPath: stagingProjectPath, StagingDirectory: stagingDir, TargetDirectory: targetDir,
		MediaPolicy: mediaPolicy, Media: media,
	})
	if err != nil {
		return workspace, err
	}
	published = true
	return map[string]any{
		"status": "ok", "project_lifecycle": "save_as_folder",
		"project_path": logicalTargetPath, "project_uuid": targetUUID,
		"source_project_path": sourcePath, "source_project_uuid": sourceUUID,
		"directory_path": targetDir, "media_policy": mediaPolicy,
		"audio_self_contained":     manifest.AudioSelfContained,
		"package_status":           manifest.PackageStatus,
		"package_manifest_path":    filepath.Join(targetDir, "package_manifest.v2.json"),
		"active_project_unchanged": true,
	}, nil
}

func exportMediaSources(value any) []projectpackage.ExportMediaSource {
	rows := mapRowsFromAny(value)
	out := make([]projectpackage.ExportMediaSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectpackage.ExportMediaSource{
			SourcePath: firstString(row, "source_path"),
			TargetPath: firstString(row, "target_path"),
			Size:       int64(numberFromAny(row["size"])),
		})
	}
	return out
}
