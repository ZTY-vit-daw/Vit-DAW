package projectpackage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/projectworkspace"
)

func TestPrepareCommitRestorePortableFolder(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "dev")
	if err := os.MkdirAll(filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "mixboard"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_DEV_ROOT", devRoot)
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "mixboard"))
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "acoustic_package_status.json"))

	sourceDir := filepath.Join(root, "source")
	targetDir := filepath.Join(root, "Portable Song")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "source.vit")
	targetPath := filepath.Join(targetDir, "Portable Song.vit")
	if err := os.WriteFile(sourcePath, []byte("source project"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("target project"), 0o644); err != nil {
		t.Fatal(err)
	}

	historyWorkspace := filepath.Join(sourceDir, ".vit_history", ".save_prepares", "prepare_1", "workspace")
	if err := os.MkdirAll(filepath.Join(historyWorkspace, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(historyWorkspace, "workspace.json"), map[string]any{"project_path": sourcePath, "project_uuid": "source_uuid"})
	writeTestJSON(t, filepath.Join(historyWorkspace, "state", "agent_runtime_state.json"), map[string]any{"project_uuid": "source_uuid"})
	writeTestJSON(t, filepath.Join(historyWorkspace, "state", "conversation_graph.json"), map[string]any{"project_uuid": "source_uuid", "nodes": []any{}})

	featurePath := filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "mixboard_feature_snapshot.json")
	writeTestJSON(t, featurePath, map[string]any{
		"schema_version":        "mixboard_feature_snapshot.v1",
		"band_energy_summaries": []any{map[string]any{"project_id": "source_uuid", "track_id": "t1", "status": "ready"}},
	})
	store := acousticpackage.NewStore("")
	if _, err := store.Upsert(acousticpackage.Status{
		ProjectID: "source_uuid", TrackID: "t1", ClipID: "c1", SourceRevision: "rev1",
		PackageLayers: map[string]acousticpackage.LayerStatus{"l3_deep": {Status: acousticpackage.StatusReady, Features: map[string]acousticpackage.FeatureStatus{
			"band_energy_summary": {Status: acousticpackage.StatusReady, Ref: map[string]any{"status": "ready"}},
		}}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, featureType := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		row := packageTestL3Row(featureType, "t2", "c2", filepath.Join(root, "two.wav"), "rev2")
		if _, appended, err := projectworkspace.AppendL3Feature(sourcePath, "source_uuid", row); err != nil || !appended {
			t.Fatalf("append project L3 %s appended=%v err=%v", featureType, appended, err)
		}
	}

	prepared, err := Prepare(PrepareOptions{
		ProjectPath: sourcePath, ProjectUUID: "source_uuid", PrepareID: "prepare_1",
		GenerationID: "save_1", SaveKind: "save_as", HistoryWorkspace: historyWorkspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Status != "prepared" {
		t.Fatalf("prepared = %#v", prepared)
	}

	targetHistory := filepath.Join(targetDir, ".vit_history", "target_uuid")
	if err := os.MkdirAll(filepath.Join(targetHistory, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(targetHistory, "workspace.json"), map[string]any{"project_path": targetPath, "project_uuid": "target_uuid"})
	writeTestJSON(t, filepath.Join(targetHistory, "state", "agent_runtime_state.json"), map[string]any{"project_uuid": "target_uuid"})
	writeTestJSON(t, filepath.Join(targetHistory, "state", "conversation_graph.json"), map[string]any{"project_uuid": "target_uuid", "nodes": []any{}})

	manifest, packageDir, err := Commit(CommitOptions{
		SourcePath: sourcePath, SourceUUID: "source_uuid", TargetPath: targetPath, TargetUUID: "target_uuid",
		PrepareID: "prepare_1", GenerationID: "save_1", SaveKind: "save_as", PortableFolder: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if packageDir != filepath.Join(targetDir, PortableDirName) || manifest.ProjectUUID != "target_uuid" || manifest.OriginProjectUUID != "source_uuid" {
		t.Fatalf("manifest=%#v packageDir=%s", manifest, packageDir)
	}
	if _, _, err := Discover(targetPath, "target_uuid"); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(targetHistory); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(featurePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.Path); err != nil {
		t.Fatal(err)
	}
	inspected, err := InspectLegacy(targetPath, "target_uuid")
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Status != "legacy_read_only" || !inspected.ManifestVerified {
		t.Fatalf("inspect = %#v", inspected)
	}
	for _, path := range []string{targetHistory, featurePath, store.Path} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("read-only inspection restored %s: %v", path, statErr)
		}
	}
	restored, err := Restore(targetPath, "target_uuid")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.AcousticPackages != 2 {
		t.Fatalf("manifest acoustic packages = %d, want store plus project log", manifest.AcousticPackages)
	}
	if !restored.ManifestVerified || !restored.HistoryRestored || !restored.FeatureSnapshotRestored || !restored.L3FeatureLogRestored || restored.AcousticPackagesRestored != 2 {
		t.Fatalf("restore = %#v", restored)
	}
	snapshot, err := store.Read()
	if err != nil || len(snapshot.Packages) != 2 || snapshot.Packages[0].ProjectID != "target_uuid" || snapshot.Packages[1].ProjectID != "target_uuid" {
		t.Fatalf("acoustic restore snapshot=%#v err=%v", snapshot, err)
	}
	rows, _, err := projectworkspace.ReadL3Features(targetPath, "target_uuid")
	if err != nil || len(rows) != 3 {
		t.Fatalf("restored L3 log rows=%d err=%v", len(rows), err)
	}
}

func packageTestL3Row(featureType, trackID, clipID, sourcePath, sourceRevision string) map[string]any {
	row := map[string]any{
		"command": "audio_feature_data_ready", "feature_type": featureType, "status": "ready", "quality_status": "ready",
		"track_id": trackID, "clip_id": clipID, "file_path": sourcePath, "source_path": sourcePath,
		"source_revision": sourceRevision, "source_fingerprint": sourceRevision,
		"clip_revision":     "clip=" + clipID + "|source=" + sourceRevision,
		"analyzer_revision": "dad_l3_offline_analyzer.v1", "duration_seconds": 10.0,
		"coverage_ratio": 1.0, "coverage_seconds": 10.0, "updated_at": "2026-07-31T10:00:00Z",
	}
	switch featureType {
	case "band_energy_summary":
		row["bands"] = map[string]any{"mid": map[string]any{"status": "ready", "energy": 1.0}}
	case "stereo_relation_summary":
		row["correlation_estimate"] = 0.5
	case "loudness_summary":
		row["rms"] = 0.1
	}
	return row
}

func TestLoosePackagesDoNotCollideInSharedDirectory(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "A.vit")
	b := filepath.Join(root, "B.vit")
	if got := PackagePath(a, "uuid_a", false); got != filepath.Join(root, "A.vit_project") {
		t.Fatalf("A package = %s", got)
	}
	if got := PackagePath(b, "uuid_b", false); got != filepath.Join(root, "B.vit_project") {
		t.Fatalf("B package = %s", got)
	}
}

func TestRestoreLegacyProjectStoredInsideLoosePackageFolder(t *testing.T) {
	root := t.TempDir()
	projectUUID := "legacy_uuid"
	packageDir := filepath.Join(root, "Legacy Song"+LoosePackageSuffix)
	projectPath := filepath.Join(packageDir, "Legacy Song.vit")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("legacy project"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestJSON(t, filepath.Join(packageDir, "history", "workspace.json"), map[string]any{
		"project_path": projectPath,
		"project_uuid": projectUUID,
	})
	writeTestJSON(t, filepath.Join(packageDir, "history", "state", "conversation_graph.json"), map[string]any{
		"project_path":   projectPath,
		"project_uuid":   projectUUID,
		"active_node_id": "a5_node",
		"nodes":          []any{map[string]any{"id": "a5_node", "kind": "vit", "text": "A5 complete"}},
	})
	writeTestJSON(t, filepath.Join(packageDir, "history", "state", "HEAD"), "a5_commit")
	if err := writeManifest(packageDir, Manifest{
		SchemaVersion: SchemaVersion,
		ProjectUUID:   projectUUID,
		ProjectPath:   projectPath,
		ProjectFile:   filepath.Base(projectPath),
		PackageMode:   "loose_sidecar",
	}); err != nil {
		t.Fatal(err)
	}

	discovered, _, err := Discover(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if discovered != packageDir {
		t.Fatalf("legacy package discovered at %q, want %q", discovered, packageDir)
	}
	restored, err := Restore(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != "ok" || !restored.ManifestVerified || !restored.HistoryRestored {
		t.Fatalf("legacy restore = %#v", restored)
	}
	restoredGraph := filepath.Join(packageDir, ".vit_history", projectUUID, "state", "conversation_graph.json")
	graph := map[string]any{}
	if err := readJSON(restoredGraph, &graph); err != nil {
		t.Fatal(err)
	}
	if graph["active_node_id"] != "a5_node" {
		t.Fatalf("restored graph = %#v", graph)
	}
}

func TestNormalSavePreservesPortablePackageMode(t *testing.T) {
	root := t.TempDir()
	devRoot := filepath.Join(root, "dev")
	t.Setenv("VIT_DAW_DEV_ROOT", devRoot)
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "mixboard"))
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", filepath.Join(devRoot, "VitApp", "Workspace", "Artifacts", "acoustic_package_status.json"))

	projectDir := filepath.Join(root, "Portable Song")
	projectPath := filepath.Join(projectDir, "Portable Song.vit")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}

	prepare := func(prepareID, generationID, saveKind string) {
		t.Helper()
		workspace := filepath.Join(projectDir, ".vit_history", ".save_prepares", prepareID, "workspace")
		writeTestJSON(t, filepath.Join(workspace, "workspace.json"), map[string]any{
			"project_path": projectPath,
			"project_uuid": "portable_uuid",
		})
		if _, err := Prepare(PrepareOptions{
			ProjectPath: projectPath, ProjectUUID: "portable_uuid", PrepareID: prepareID,
			GenerationID: generationID, SaveKind: saveKind, HistoryWorkspace: workspace,
		}); err != nil {
			t.Fatal(err)
		}
	}

	prepare("prepare_save_as", "generation_save_as", "save_as")
	first, packageDir, err := Commit(CommitOptions{
		SourcePath: projectPath, SourceUUID: "portable_uuid", TargetPath: projectPath, TargetUUID: "portable_uuid",
		PrepareID: "prepare_save_as", GenerationID: "generation_save_as", SaveKind: "save_as", PortableFolder: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.PackageMode != "portable_folder" || packageDir != filepath.Join(projectDir, PortableDirName) {
		t.Fatalf("initial manifest=%#v packageDir=%s", first, packageDir)
	}

	prepare("prepare_save", "generation_save", "save")
	second, secondPackageDir, err := Commit(CommitOptions{
		SourcePath: projectPath, SourceUUID: "portable_uuid", TargetPath: projectPath, TargetUUID: "portable_uuid",
		PrepareID: "prepare_save", GenerationID: "generation_save", SaveKind: "save",
	})
	if err != nil {
		t.Fatal(err)
	}
	if secondPackageDir != packageDir || second.PackageMode != "portable_folder" {
		t.Fatalf("normal save changed portable mode: manifest=%#v packageDir=%s", second, secondPackageDir)
	}
	stored := Manifest{}
	if err := readJSON(filepath.Join(secondPackageDir, ManifestFile), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.PackageMode != "portable_folder" {
		t.Fatalf("stored package mode = %q", stored.PackageMode)
	}
}

func TestManifestRejectsTamperedFile(t *testing.T) {
	root := t.TempDir()
	writeTestJSON(t, filepath.Join(root, "payload.json"), map[string]any{"ok": true})
	manifest := Manifest{SchemaVersion: SchemaVersion, ProjectUUID: "p", ProjectPath: "x", ProjectFile: "x.vit"}
	if err := writeManifest(root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "payload.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := readJSON(filepath.Join(root, ManifestFile), &manifest); err != nil {
		t.Fatal(err)
	}
	if err := verifyManifest(root, manifest); err == nil {
		t.Fatal("tampered package unexpectedly verified")
	}
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
