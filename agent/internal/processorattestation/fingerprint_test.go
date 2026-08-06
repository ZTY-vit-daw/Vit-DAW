package processorattestation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintPathTracksFileContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "processor.vst3")
	if err := os.WriteFile(path, []byte("version-one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version-two"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !validSHA256(first) || !validSHA256(second) {
		t.Fatalf("file fingerprints first=%q second=%q", first, second)
	}
}

func TestFingerprintPathHashesBundleDeterministically(t *testing.T) {
	firstRoot := filepath.Join(t.TempDir(), "first.vst3")
	secondRoot := filepath.Join(t.TempDir(), "second.vst3")
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.MkdirAll(filepath.Join(root, "Contents", "x86_64-win"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Contents", "moduleinfo.json"), []byte("metadata"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "Contents", "x86_64-win", "processor.vst3"), []byte("binary"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	first, err := FingerprintPath(firstRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintPath(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identical bundles produced different fingerprints: %s != %s", first, second)
	}
	if err := os.WriteFile(filepath.Join(secondRoot, "Contents", "moduleinfo.json"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := FingerprintPath(secondRoot)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("bundle content change did not change fingerprint")
	}
}
