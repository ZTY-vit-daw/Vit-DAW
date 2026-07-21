package projectworkspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalysisManifestRoundTrip(t *testing.T) {
	project := filepath.Join(t.TempDir(), "Song.vit")
	if err := os.WriteFile(project, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	status := map[string]any{
		"dad_fact_status": "ready",
		"track_waveform_envelopes": []any{map[string]any{
			"status": "ready", "track_id": "track_1", "clip_id": "clip_1",
			"source_revision": "source_rev", "clip_revision": "clip_rev", "rms_dbfs": -24.5,
		}},
	}
	manifest, path, err := SaveAnalysisManifest(project, "vitproj_test", status)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Rows) != 1 || filepath.Base(filepath.Dir(path)) != "vitproj_test" {
		t.Fatalf("saved manifest = %#v path=%s", manifest, path)
	}
	loaded, loadedPath, err := LoadAnalysisManifest(project, "vitproj_test")
	if err != nil {
		t.Fatal(err)
	}
	recovered := loaded.AudioAnalysisStatus(loadedPath)
	if recovered["dad_fact_status"] != "ready" || recovered["dad_fact_ready_count"] != 1 || loadedPath != path {
		t.Fatalf("recovered status = %#v", recovered)
	}
}
