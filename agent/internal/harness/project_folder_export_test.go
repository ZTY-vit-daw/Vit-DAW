package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/shadow"
)

type folderExportKernel struct {
	targetUUID string
	mediaPath  string
	calls      []map[string]any
}

func (k *folderExportKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	k.calls = append(k.calls, cloneAnyMap(command))
	if boolValueDefault(command["preflight_only"], false) {
		return map[string]any{
			"status": "media_preflight", "referenced_audio_count": 1,
			"external_audio_count": 1, "missing_audio_count": 0, "estimated_copy_bytes": 5,
		}, "", nil
	}
	writePath := firstString(command, "write_path")
	if err := os.MkdirAll(filepath.Dir(writePath), 0o755); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(writePath, []byte("encrypted-project-copy"), 0o644); err != nil {
		return nil, "", err
	}
	projectPath := firstString(command, "file_path")
	projectBase := strings.TrimSuffix(filepath.Base(projectPath), filepath.Ext(projectPath))
	return map[string]any{
		"status": "ok", "project_lifecycle": "save_as_folder",
		"project_path": projectPath, "snapshot_path": writePath,
		"project_uuid": k.targetUUID, "source_project_uuid": "vitproj_source",
		"active_project_unchanged": true,
		"media": []any{map[string]any{
			"kind": "audio", "source_path": k.mediaPath,
			"target_path": filepath.ToSlash(filepath.Join(projectBase+"_Media", "Audio", filepath.Base(k.mediaPath))),
			"size":        5, "status": "pending_copy",
		}},
	}, "", nil
}

func TestSaveAsFolderRequiresExplicitMediaPolicy(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": sourcePath, "project_uuid": "vitproj_source"})
	kernel := &folderExportKernel{targetUUID: "vitproj_target", mediaPath: filepath.Join(root, "vocal.wav")}
	h := NewWithSender(kernel, project, nil)
	if _, err := h.ActivateProjectStore(sourcePath, "vitproj_source"); err != nil {
		t.Fatal(err)
	}
	result, err := h.saveAsFolder(context.Background(), map[string]any{"directory_path": filepath.Join(root, "Portable")})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(result, "status") != "require_media_policy" || len(kernel.calls) != 1 {
		t.Fatalf("result=%#v calls=%#v", result, kernel.calls)
	}
	if _, err := os.Stat(filepath.Join(root, "Portable")); !os.IsNotExist(err) {
		t.Fatalf("target was created during preflight: %v", err)
	}
}

func TestSaveAsFolderPublishesIndependentProjectAndKeepsSourceActive(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	audioPath := filepath.Join(root, "vocal.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": sourcePath, "project_uuid": "vitproj_source"})
	kernel := &folderExportKernel{targetUUID: "vitproj_target", mediaPath: audioPath}
	h := NewWithSender(kernel, project, nil)
	sourceRoots, err := h.ActivateProjectStore(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	sourceSession, err := history.EnsureWorkingSession(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	if err := history.WriteAgentRuntimeState(sourcePath, "vitproj_source", []byte(`{"turn":"B1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sourceRoots.Agent, "observations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoots.Agent, "observations", "obs_source.json"), []byte(`{"project_uuid":"vitproj_source"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := projectworkspace.SaveAnalysisManifest(sourcePath, "vitproj_source", map[string]any{
		"dad_fact_status": "ready",
		"track_waveform_envelopes": []any{map[string]any{
			"status": "ready", "track_id": "track_1", "clip_id": "clip_1",
			"source_path": audioPath, "rms_dbfs": -18.0, "peak_dbfs": -6.0,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "Portable")
	result, err := h.saveAsFolder(context.Background(), map[string]any{
		"directory_path": targetDir, "media_policy": "copy_referenced_audio",
	})
	if err != nil {
		t.Fatal(err)
	}
	if firstString(result, "status") != "ok" || !boolValueDefault(result["active_project_unchanged"], false) {
		t.Fatalf("result=%#v", result)
	}
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "source_Media", "Audio", filepath.Base(audioPath))); err != nil {
		t.Fatal(err)
	}
	targetRoots, err := projectstore.Resolve(targetPath, "vitproj_target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectstore.Load(targetRoots); err != nil {
		t.Fatal(err)
	}
	targetAnalysis, _, err := projectworkspace.LoadAnalysisManifest(targetPath, "vitproj_target")
	if err != nil {
		t.Fatalf("folder export omitted derived analysis manifest: %v", err)
	}
	if targetAnalysis.ProjectUUID != "vitproj_target" || !samePath(targetAnalysis.ProjectPath, targetPath) || len(targetAnalysis.Rows) != 1 {
		t.Fatalf("folder export did not rebind derived analysis manifest: %+v", targetAnalysis)
	}
	current, ok := projectstore.Current()
	if !ok || current.ProjectUUID != "vitproj_source" || !samePath(current.ProjectPath, sourcePath) {
		t.Fatalf("active store changed: %+v ok=%v", current, ok)
	}
	continuedSession, err := history.EnsureWorkingSession(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	if continuedSession.SessionID != sourceSession.SessionID || continuedSession.Status != "active" {
		t.Fatalf("folder export severed the active source history session: before=%+v after=%+v", sourceSession, continuedSession)
	}
	if err := history.WriteAgentRuntimeState(sourcePath, "vitproj_source", []byte(`{"turn":"B2"}`)); err != nil {
		t.Fatal(err)
	}
	kernel.targetUUID = "vitproj_target_2"
	secondTargetDir := filepath.Join(root, "Portable2")
	if _, err := h.saveAsFolder(context.Background(), map[string]any{
		"directory_path": secondTargetDir, "media_policy": "copy_referenced_audio",
	}); err != nil {
		t.Fatal(err)
	}
	secondRuntime, err := history.ReadAgentRuntimeState(filepath.Join(secondTargetDir, filepath.Base(sourcePath)), "vitproj_target_2")
	if err != nil {
		t.Fatal(err)
	}
	secondState := map[string]any{}
	if err := json.Unmarshal(secondRuntime, &secondState); err != nil {
		t.Fatal(err)
	}
	if firstString(secondState, "turn") != "B2" || firstString(secondState, "project_uuid") != "vitproj_target_2" || !samePath(firstString(secondState, "project_path"), filepath.Join(secondTargetDir, filepath.Base(sourcePath))) {
		t.Fatalf("second folder export lost the latest source Agent runtime or target identity: %s", secondRuntime)
	}
	// Two folder exports each perform preflight plus snapshot.
	if len(kernel.calls) != 4 {
		t.Fatalf("kernel calls=%#v", kernel.calls)
	}
}

func TestSaveAsFolderExportsUnsavedDraftFromActiveProjectStore(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	draftPath := filepath.Join(root, "drafts", "draft_123", "Unsaved.vit")
	if err := os.MkdirAll(filepath.Dir(draftPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(draftPath, []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": ""})
	kernel := &folderExportKernel{targetUUID: "vitproj_exported", mediaPath: filepath.Join(root, "unused.wav")}
	h := NewWithSender(kernel, project, nil)
	sourceRoots, err := h.ActivateProjectStore(draftPath, "vitproj_draft")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.EnsureWorkingSession(draftPath, "vitproj_draft"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(sourceRoots.Agent, projectstore.ManifestFile)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(root, "Portable")
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool:      "project.save_as_folder",
		Confirmed: true,
		Args: map[string]any{
			"directory_path": targetDir,
			"media_policy":   "reference_only",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" {
		t.Fatalf("response=%+v", response)
	}
	result := response.Result
	if firstString(result, "status") != "ok" || !boolValueDefault(result["active_project_unchanged"], false) {
		t.Fatalf("result=%#v", result)
	}
	targetPath := filepath.Join(targetDir, "Unsaved.vit")
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatal(err)
	}
	targetRoots, err := projectstore.Resolve(targetPath, "vitproj_exported")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectstore.Load(targetRoots); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "package_manifest.v2.json")); err != nil {
		t.Fatal(err)
	}
	current, ok := projectstore.Current()
	if !ok || current.ProjectUUID != "vitproj_draft" || !samePath(current.ProjectPath, draftPath) {
		t.Fatalf("active draft store changed: %+v ok=%v", current, ok)
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestAfter) != string(manifestBefore) {
		t.Fatal("source draft manifest changed during folder export")
	}
}

func TestSaveAsFolderDefersAuditToTargetAndLeavesSourceAgentStoreFrozen(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	audioPath := filepath.Join(root, "vocal.wav")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"project_path": sourcePath, "project_uuid": "vitproj_source"})
	kernel := &folderExportKernel{targetUUID: "vitproj_target", mediaPath: audioPath}
	h := NewWithSender(kernel, project, nil)
	sourceRoots, err := h.ActivateProjectStore(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.EnsureWorkingSession(sourcePath, "vitproj_source"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(sourceRoots.Agent, projectstore.ManifestFile)
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "Portable")
	response, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "project.save_as_folder", Confirmed: true,
		Args: map[string]any{"directory_path": targetDir, "media_policy": "copy_referenced_audio"},
	})
	if err != nil || response.Status != "ok" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
	manifestAfter, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(manifestAfter) != string(manifestBefore) {
		t.Fatal("source manifest changed during folder export")
	}
	if entries, readErr := os.ReadDir(filepath.Join(sourceRoots.Agent, "journal")); readErr == nil && len(entries) > 0 {
		t.Fatalf("source journal changed during folder export: %#v", entries)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	targetPath := filepath.Join(targetDir, filepath.Base(sourcePath))
	targetRoots, err := projectstore.Resolve(targetPath, "vitproj_target")
	if err != nil {
		t.Fatal(err)
	}
	targetJournal, err := journal.NewSharded(20, filepath.Join(targetRoots.Agent, "journal"), targetRoots.ProjectUUID)
	if err != nil {
		t.Fatal(err)
	}
	actions := targetJournal.Recent(10)
	if len(actions) != 1 || actions[0].CommandName != "save_as_folder" || actions[0].Status != journal.StatusSucceeded {
		t.Fatalf("target audit=%#v", actions)
	}
}
