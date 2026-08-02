package projectworkspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestL3FeatureLogDeduplicatesFoldsAndForks(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "source", "song.vit")
	targetPath := filepath.Join(root, "target", "song copy.vit")
	for _, path := range []string{sourcePath, targetPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("project"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	first := testL3FeatureRow("band_energy_summary", "track_1", "clip_1", filepath.Join(root, "one.wav"), "rev_one")
	if _, appended, err := AppendL3Feature(sourcePath, "source_uuid", first); err != nil || !appended {
		t.Fatalf("first append appended=%v err=%v", appended, err)
	}
	if _, appended, err := AppendL3Feature(sourcePath, "source_uuid", first); err != nil || appended {
		t.Fatalf("duplicate append appended=%v err=%v", appended, err)
	}
	updatedDuplicate := cloneMap(first)
	updatedDuplicate["updated_at"] = "2026-07-31T10:01:00Z"
	if _, appended, err := AppendL3Feature(sourcePath, "source_uuid", updatedDuplicate); err != nil || appended {
		t.Fatalf("semantic duplicate append appended=%v err=%v", appended, err)
	}
	for _, featureType := range []string{"stereo_relation_summary", "loudness_summary"} {
		if _, appended, err := AppendL3Feature(sourcePath, "source_uuid", testL3FeatureRow(featureType, "track_1", "clip_1", filepath.Join(root, "one.wav"), "rev_one")); err != nil || !appended {
			t.Fatalf("append %s appended=%v err=%v", featureType, appended, err)
		}
	}
	for _, featureType := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		if _, appended, err := AppendL3Feature(sourcePath, "source_uuid", testL3FeatureRow(featureType, "track_2", "clip_2", filepath.Join(root, "two.wav"), "rev_two")); err != nil || !appended {
			t.Fatalf("append second identity %s appended=%v err=%v", featureType, appended, err)
		}
	}

	rows, _, err := ReadL3Features(sourcePath, "source_uuid")
	if err != nil || len(rows) != 6 {
		t.Fatalf("source rows=%d err=%v", len(rows), err)
	}
	statuses, _, err := BuildL3AcousticStatuses(sourcePath, "source_uuid")
	if err != nil || len(statuses) != 2 {
		t.Fatalf("statuses=%#v err=%v", statuses, err)
	}
	for _, status := range statuses {
		if status.ProjectID != "source_uuid" {
			t.Fatalf("status project identity not stamped: %#v", status)
		}
		features := status.PackageLayers["l3_deep"].Features
		for _, name := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
			if features[name].Status != "ready" {
				t.Fatalf("%s not ready in %#v", name, status)
			}
		}
	}
	duplicate := cloneMap(first)
	duplicate["updated_at"] = "2026-07-31T10:02:00Z"
	path, err := L3FeatureLogPath(sourcePath, "source_uuid")
	if err != nil {
		t.Fatal(err)
	}
	rowsWithDuplicate := append(append([]map[string]any{}, rows...), duplicate)
	if err := writeL3FeatureRowsAtomic(path, rowsWithDuplicate); err != nil {
		t.Fatal(err)
	}

	if _, err := ForkDerived(sourcePath, "source_uuid", targetPath, "target_uuid"); err != nil {
		t.Fatal(err)
	}
	targetRows, _, err := ReadL3Features(targetPath, "target_uuid")
	if err != nil || len(targetRows) != 6 {
		t.Fatalf("target rows=%d err=%v", len(targetRows), err)
	}
	for _, row := range targetRows {
		if text(row["project_id"]) != "target_uuid" || text(row["project_uuid"]) != "target_uuid" || text(row["project_path"]) != targetPath {
			t.Fatalf("forked row was not rebound: %#v", row)
		}
	}
}

func testL3FeatureRow(featureType, trackID, clipID, sourcePath, sourceRevision string) map[string]any {
	row := map[string]any{
		"command": "audio_feature_data_ready", "feature_type": featureType, "status": "ready", "quality_status": "ready",
		"track_id": trackID, "clip_id": clipID, "file_path": sourcePath, "source_path": sourcePath,
		"source_revision": sourceRevision, "source_fingerprint": sourceRevision,
		"clip_revision":     "clip=" + clipID + "|source=" + sourceRevision,
		"analyzer_revision": "dad_l3_offline_analyzer.v1", "duration_seconds": 12.5,
		"coverage_ratio": 1.0, "coverage_seconds": 12.5, "updated_at": "2026-07-31T10:00:00Z",
		"source_identity": map[string]any{"track_id": trackID, "clip_id": clipID, "source_path": sourcePath, "source_revision": sourceRevision},
		"target":          map[string]any{"track_id": trackID, "clip_id": clipID, "file_path": sourcePath, "source_revision": sourceRevision},
	}
	switch featureType {
	case "band_energy_summary":
		row["bands"] = map[string]any{"mid": map[string]any{"status": "ready", "energy": 1.0}}
	case "stereo_relation_summary":
		row["correlation_estimate"] = 0.5
	case "loudness_summary":
		row["rms"] = 0.1
		row["peak_abs"] = 0.3
	}
	return row
}
