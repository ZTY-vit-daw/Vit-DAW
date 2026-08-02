package projectstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesProjectScopedManifestAndRoots(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	roots, manifest, err := Ensure(projectPath, "vitproj_song")
	if err != nil {
		t.Fatal(err)
	}
	if roots.Agent != filepath.Join(root, AgentDirName, "vitproj_song") || roots.History != filepath.Join(root, HistoryDirName, "vitproj_song") || roots.Derived != filepath.Join(root, DerivedDirName, "vitproj_song") {
		t.Fatalf("roots = %+v", roots)
	}
	if manifest.SchemaVersion != ManifestSchemaVersion || manifest.ProjectUUID != "vitproj_song" || manifest.Budgets.ObservationMaxBytes != 2*1024*1024 {
		t.Fatalf("manifest = %+v", manifest)
	}
	loaded, err := Load(roots)
	if err != nil || loaded.ProjectUUID != manifest.ProjectUUID {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestActivatePublishesOnlyValidProjectIdentity(t *testing.T) {
	Deactivate()
	t.Cleanup(Deactivate)
	projectPath := filepath.Join(t.TempDir(), "song.vit")
	roots, _, err := Activate(projectPath, "vitproj_active")
	if err != nil {
		t.Fatal(err)
	}
	current, ok := Current()
	if !ok || current != roots {
		t.Fatalf("current=%+v ok=%v want=%+v", current, ok, roots)
	}
	if _, _, err := Activate("", ""); err == nil {
		t.Fatal("invalid activation succeeded")
	}
	current, ok = Current()
	if !ok || current != roots {
		t.Fatal("failed activation replaced the valid active identity")
	}
}

func TestForkAgentStoreCopiesAndRebindsWithoutMutatingSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "song.vit")
	targetPath := filepath.Join(root, "target", "copy.vit")
	sourceRoots, manifest, err := Ensure(sourcePath, "vitproj_source")
	if err != nil {
		t.Fatal(err)
	}
	manifest.OriginProjectUUID = "vitproj_origin"
	if err := Write(sourceRoots, manifest); err != nil {
		t.Fatal(err)
	}
	observationDir := filepath.Join(sourceRoots.Agent, "observations")
	if err := os.MkdirAll(observationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourceJSON := filepath.Join(observationDir, "obs_1.json")
	sourceData := []byte(`{"schema_version":"mix_observation.v2","project_uuid":"vitproj_source","project_path":"` + strings.ReplaceAll(sourceRoots.ProjectPath, `\`, `\\`) + `"}`)
	if err := os.WriteFile(sourceJSON, sourceData, 0o600); err != nil {
		t.Fatal(err)
	}
	journalDir := filepath.Join(sourceRoots.Agent, "journal")
	if err := os.MkdirAll(journalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, "000001.jsonl"), append(sourceData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	targetDir, err := ForkAgentStore(sourcePath, "vitproj_source", targetPath, "vitproj_target")
	if err != nil {
		t.Fatal(err)
	}
	targetRoots, err := Resolve(targetPath, "vitproj_target")
	if err != nil {
		t.Fatal(err)
	}
	if targetDir != targetRoots.Agent {
		t.Fatalf("targetDir=%q want=%q", targetDir, targetRoots.Agent)
	}
	targetManifest, err := Load(targetRoots)
	if err != nil {
		t.Fatal(err)
	}
	if targetManifest.ProjectUUID != "vitproj_target" || targetManifest.OriginProjectUUID != "vitproj_origin" || !samePath(targetManifest.ProjectPath, targetPath) {
		t.Fatalf("target manifest = %+v", targetManifest)
	}
	targetData, err := os.ReadFile(filepath.Join(targetRoots.Agent, "observations", "obs_1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var targetValue map[string]any
	if err := json.Unmarshal(targetData, &targetValue); err != nil {
		t.Fatal(err)
	}
	if targetValue["project_uuid"] != "vitproj_target" || !samePath(targetValue["project_path"].(string), targetPath) {
		t.Fatalf("target observation = %#v", targetValue)
	}
	unchanged, err := os.ReadFile(sourceJSON)
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(sourceData) {
		t.Fatalf("source changed: %s", unchanged)
	}
}

func TestLoadRejectsIdentityMismatch(t *testing.T) {
	roots, _, err := Ensure(filepath.Join(t.TempDir(), "song.vit"), "vitproj_expected")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(roots.Agent, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	value["project_uuid"] = "vitproj_wrong"
	data, _ = json.Marshal(value)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(roots); err == nil {
		t.Fatal("identity mismatch was accepted")
	}
}
