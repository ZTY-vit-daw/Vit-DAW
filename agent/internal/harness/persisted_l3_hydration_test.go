package harness

import (
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/projectworkspace"
)

func TestHydratePersistedProjectL3PackagesRestoresCurrentStore(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "fixture.vit")
	if err := os.WriteFile(projectPath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, _, err := projectstore.Activate(projectPath, "project-l3")
	if err != nil {
		t.Fatal(err)
	}
	defer projectstore.Deactivate()

	_, _, err = projectworkspace.AppendL3Feature(projectPath, roots.ProjectUUID, map[string]any{
		"feature_type":    "band_energy_summary",
		"track_id":        "track-1",
		"clip_id":         "clip-1",
		"source_path":     filepath.Join(t.TempDir(), "stem.wav"),
		"source_revision": "rev-1",
		"status":          "ready",
		"bands": map[string]any{
			"bass": map[string]any{"energy_db": -18.0},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	storePath := filepath.Join(roots.Derived, "acoustic_package_status.json")
	store := acousticpackage.NewStore(storePath)
	hydratePersistedProjectL3Packages(store, nil)
	snapshot, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Packages) != 1 {
		t.Fatalf("restored packages = %d, want 1", len(snapshot.Packages))
	}
	feature := snapshot.Packages[0].PackageLayers["l3_deep"].Features["band_energy_summary"]
	if feature.Status != acousticpackage.StatusReady {
		t.Fatalf("restored L3 status = %q, want ready", feature.Status)
	}
}
