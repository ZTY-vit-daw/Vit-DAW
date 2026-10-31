package projectworkspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// M3-OBS-IDENTITY-BIND-1：ForkDerived 必须把整棵 derived 树的 JSON/JSONL 再
// 绑定到 fork 的新身份（acoustic store / 观察快照随行），且容忍旧形态
// （无 acoustic store 的 derived 也能 fork——§11 兼容）。

func TestForkDerivedRebindsWholeTreeIdentities(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "src.vit")
	targetPath := filepath.Join(root, "dst.vit")
	const sourceUUID = "vitproj_fd_rebind_src"
	const targetUUID = "vitproj_fd_rebind_dst"

	sourceDir, err := DerivedDir(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 旧形态之外的现代形态：L3 log + acoustic store + 观察快照。
	l3Row := map[string]any{
		"feature_type": "band_energy_summary", "status": "ready",
		"track_id": "1007", "clip_id": "1014", "project_id": sourceUUID, "project_uuid": sourceUUID,
		"project_path": sourcePath, "file_path": filepath.Join(root, "clip.wav"),
		"duration_seconds": 6.0, "source_revision": "rev_fd",
	}
	if _, _, err := AppendL3Feature(sourcePath, sourceUUID, l3Row); err != nil {
		t.Fatal(err)
	}
	acoustic := map[string]any{
		"schema_version": "acoustic_package_status.v0",
		"packages": []any{map[string]any{
			"schema_version": "acoustic_package_status.v0", "status": "partial",
			"project_id": sourceUUID, "track_id": "1007", "clip_id": "1014",
			"source_revision": "rev_fd",
		}},
	}
	acousticBytes, _ := json.MarshalIndent(acoustic, "", "\t")
	if err := os.WriteFile(filepath.Join(sourceDir, "acoustic_package_status.json"), acousticBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"latest_request": map[string]any{"project_id": sourceUUID, "project_path": sourcePath},
	}
	snapshotBytes, _ := json.MarshalIndent(snapshot, "", "\t")
	if err := os.WriteFile(filepath.Join(sourceDir, "mixboard_feature_snapshot.json"), snapshotBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ForkDerived(sourcePath, sourceUUID, targetPath, targetUUID); err != nil {
		t.Fatal(err)
	}
	targetDir, err := DerivedDir(targetPath, targetUUID)
	if err != nil {
		t.Fatal(err)
	}
	for name := range map[string]string{
		"acoustic_package_status.json":   "",
		"mixboard_feature_snapshot.json": "",
		"l3_feature_log.jsonl":           "",
	} {
		raw, err := os.ReadFile(filepath.Join(targetDir, name))
		if err != nil {
			t.Fatalf("forked %s missing: %v", name, err)
		}
		if strings.Contains(string(raw), sourceUUID) {
			t.Fatalf("forked %s still carries source identity %s", name, sourceUUID)
		}
		if !strings.Contains(string(raw), targetUUID) {
			t.Fatalf("forked %s does not carry target identity %s", name, targetUUID)
		}
	}
	// 保源：source 树不动。
	for _, name := range []string{"acoustic_package_status.json", "mixboard_feature_snapshot.json", "l3_feature_log.jsonl"} {
		raw, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			t.Fatalf("source %s must remain: %v", name, err)
		}
		if !strings.Contains(string(raw), sourceUUID) {
			t.Fatalf("source %s lost its identity (fork must not rewrite the source)", name)
		}
	}
}

func TestForkDerivedToleratesLegacyDerivedWithoutAcousticStore(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "legacy.vit")
	targetPath := filepath.Join(root, "legacy_as.vit")
	const sourceUUID = "vitproj_fd_legacy_src"
	const targetUUID = "vitproj_fd_legacy_dst"

	sourceDir, err := DerivedDir(sourcePath, sourceUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "analysis_manifest.json"), []byte(`{"schema_version":"other"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ForkDerived(sourcePath, sourceUUID, targetPath, targetUUID); err != nil {
		t.Fatalf("legacy derived fork must not fail: %v", err)
	}
}

func TestForkDerivedWithoutSourceIsNoop(t *testing.T) {
	root := t.TempDir()
	dir, err := ForkDerived(filepath.Join(root, "none.vit"), "vitproj_fd_none", filepath.Join(root, "out.vit"), "vitproj_fd_out")
	if err != nil || dir != "" {
		t.Fatalf("missing source derived must be a no-op, dir=%q err=%v", dir, err)
	}
}
