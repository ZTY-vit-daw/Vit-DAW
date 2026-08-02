package artifacts

import (
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/projectstore"
)

func TestDefaultRootUsesActiveV2ProjectStore(t *testing.T) {
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	projectPath := filepath.Join(t.TempDir(), "song.vit")
	if err := os.WriteFile(projectPath, []byte("vit"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots, _, err := projectstore.Activate(projectPath, "vitproj_artifacts")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(roots.Agent, "artifacts")
	if got := DefaultRoot(); got != want {
		t.Fatalf("DefaultRoot=%q want=%q", got, want)
	}
}
