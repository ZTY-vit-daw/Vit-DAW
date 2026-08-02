package projectpackage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/projectstore"
)

func TestPublishFolderV2CopiesReferencedAudioAndPublishesAtomically(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "song.vit")
	targetDir := filepath.Join(root, "portable", "Song")
	targetPath := filepath.Join(targetDir, "Song.vit")
	stagingDir := filepath.Join(filepath.Dir(targetDir), ".Song.staging_test")
	stagingPath := filepath.Join(stagingDir, "Song.vit")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceRoots, _, err := projectstore.Ensure(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagingPath, []byte("encrypted-target"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{projectstore.HistoryDirName, projectstore.DerivedDirName} {
		dir := filepath.Join(stagingDir, name, "vitproj_target")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		payload := map[string]any{"project_uuid": "vitproj_target", "project_path": stagingPath}
		data, _ := json.Marshal(payload)
		if err := os.WriteFile(filepath.Join(dir, "state.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := projectstore.ForkAgentStore(sourcePath, "vitproj_source", stagingPath, "vitproj_target"); err != nil {
		t.Fatal(err)
	}
	audio := filepath.Join(root, "audio", "vocal.wav")
	if err := os.MkdirAll(filepath.Dir(audio), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(audio, []byte("audio-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := PublishFolderV2(PublishFolderOptions{
		SourceProjectPath: sourcePath, SourceProjectUUID: "vitproj_source",
		TargetProjectPath: targetPath, TargetProjectUUID: "vitproj_target",
		StagingProjectPath: stagingPath, StagingDirectory: stagingDir, TargetDirectory: targetDir,
		MediaPolicy: MediaPolicyCopyReferenced,
		Media:       []ExportMediaSource{{SourcePath: audio, TargetPath: "Song_Media/Audio/vocal.wav"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.AudioSelfContained || manifest.PackageStatus != "complete" || len(manifest.Media) != 1 || manifest.Media[0].Status != "copied" {
		t.Fatalf("manifest=%+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "Song_Media", "Audio", "vocal.wav")); err != nil {
		t.Fatal(err)
	}
	targetRoots, err := projectstore.Resolve(targetPath, "vitproj_target")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := projectstore.Load(targetRoots)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(loaded.ProjectPath, targetPath) || loaded.OriginProjectUUID != sourceRoots.ProjectUUID {
		t.Fatalf("target agent manifest=%+v", loaded)
	}
	if _, err := os.Stat(stagingDir); !os.IsNotExist(err) {
		t.Fatalf("staging still exists: %v", err)
	}
}

func TestPublishFolderV2ReferenceOnlyDoesNotCopyAudio(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source.vit")
	targetDir := filepath.Join(root, "Reference")
	targetPath := filepath.Join(targetDir, "Reference.vit")
	stagingDir := filepath.Join(root, ".Reference.staging")
	stagingPath := filepath.Join(stagingDir, "Reference.vit")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := projectstore.Ensure(sourcePath, "vitproj_source"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stagingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagingPath, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := projectstore.ForkAgentStore(sourcePath, "vitproj_source", stagingPath, "vitproj_target"); err != nil {
		t.Fatal(err)
	}
	audio := filepath.Join(root, "vocal.wav")
	if err := os.WriteFile(audio, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := PublishFolderV2(PublishFolderOptions{
		SourceProjectPath: sourcePath, SourceProjectUUID: "vitproj_source",
		TargetProjectPath: targetPath, TargetProjectUUID: "vitproj_target",
		StagingProjectPath: stagingPath, StagingDirectory: stagingDir, TargetDirectory: targetDir,
		MediaPolicy: MediaPolicyReferenceOnly, Media: []ExportMediaSource{{SourcePath: audio}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.AudioSelfContained || manifest.Media[0].Status != "referenced" || manifest.Media[0].TargetPath != "" {
		t.Fatalf("manifest=%+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "Reference_Media")); !os.IsNotExist(err) {
		t.Fatalf("media directory was created: %v", err)
	}
}
