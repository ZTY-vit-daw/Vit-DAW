package probeaudio

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateCreatesDeterministicProbeSuite(t *testing.T) {
	root := t.TempDir()
	manifest, err := Generate(root, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SuiteID != SuiteID || len(manifest.Assets) != 7 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, asset := range manifest.Assets {
		info, err := os.Stat(filepath.Join(root, asset.File))
		if err != nil || info.Size() <= 44 || asset.SHA256 == "" {
			t.Fatalf("asset %s invalid: info=%v err=%v", asset.ID, info, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}
