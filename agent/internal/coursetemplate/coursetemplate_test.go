package coursetemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func sha256Sum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// templateDir walks up from the test working directory to the repo root and
// returns the committed melody-drum-v1 template directory. Tests consume the
// real committed asset (not a fixture copy) so manifest/template drift fails
// in CI, per the SHA256 discipline in the template README.
func templateDir(t *testing.T) string {
	t.Helper()
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := here; ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "coord", "lesson-designs", "music-creation-unit2", "templates", "melody-drum-v1")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
		if dir == filepath.Dir(dir) {
			t.Fatalf("template directory not found walking up from %s", here)
		}
	}
}

func TestLoadCommittedTemplate(t *testing.T) {
	loaded, err := Load(templateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.TemplateSHA != loaded.Manifest.TemplateSHA256 {
		t.Fatalf("hash mismatch after load: %s != %s", loaded.TemplateSHA, loaded.Manifest.TemplateSHA256)
	}
	if loaded.Edit.Numerator != 2 || loaded.Edit.Denominator != 4 {
		t.Fatalf("timesig = %d/%d, want 2/4", loaded.Edit.Numerator, loaded.Edit.Denominator)
	}
	if loaded.Edit.TempoBPM != 80.0 {
		t.Fatalf("tempo = %v, want 80", loaded.Edit.TempoBPM)
	}
	if loaded.Edit.KeySignature != "C" {
		t.Fatalf("key = %q, want C", loaded.Edit.KeySignature)
	}
	if len(loaded.Edit.Tracks) != 5 {
		t.Fatalf("tracks = %d, want 5", len(loaded.Edit.Tracks))
	}
}

func TestValidateCommittedTemplate(t *testing.T) {
	loaded, err := Load(templateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := loaded.Validate(); err != nil {
		t.Fatalf("Validate committed template: %v", err)
	}
}

// TestValidateDetectsPinnedStateDrift proves the program-pin assertion has
// teeth: mutating the manifest's pinned state hash must fail validation.
func TestValidateDetectsPinnedStateDrift(t *testing.T) {
	dir := t.TempDir()
	src := templateDir(t)
	for _, name := range []string{ManifestFile, "template.v1.xml"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load copy: %v", err)
	}
	loaded.Manifest.Sources["melody_voice"].Plugin.StateSHA256 = "deadbeef"
	if err := loaded.Validate(); err == nil {
		t.Fatal("Validate must fail when the pinned plugin state hash drifts")
	}
}

// TestValidateDetectsMediaReference proves the portability guard rejects an
// audio media path surface (machine-local or relative) in the template.
func TestValidateDetectsMediaReference(t *testing.T) {
	dir := t.TempDir()
	src := templateDir(t)
	for _, name := range []string{ManifestFile, "template.v1.xml"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	blob := []byte(`<AUDIOCLIP name="x" source="D:/machine/local.wav" start="0.0" length="1.0" offset="0.0" id="9999"/>\n</EDIT>`)
	path := filepath.Join(dir, "template.v1.xml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	idx := lastIndex(raw, []byte("</EDIT>"))
	if idx < 0 {
		t.Fatal("EDIT close tag missing")
	}
	mutated := append(raw[:idx:idx], append(blob, raw[idx:]...)...)
	if err := os.WriteFile(path, mutated, 0o644); err != nil {
		t.Fatal(err)
	}
	// Patch the manifest hash to the mutated file so Load succeeds and the
	// media-reference assertion is what Validate must catch.
	if err := patchManifestHash(dir, mutated); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load mutated copy: %v", err)
	}
	if err := loaded.Validate(); err == nil {
		t.Fatal("Validate must fail when the template carries an AUDIOCLIP media reference")
	}
}

// patchManifestHash rewrites template_sha256 in the copied manifest to the
// canonical (LF-normalized) content hash so load-time verification passes.
func patchManifestHash(dir string, data []byte) error {
	sum := sha256Sum(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	manifestPath := filepath.Join(dir, ManifestFile)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	out := bytes.Replace(raw, []byte(`"template_sha256": "a188defd2c852d57e1b18e0a2cb811c9b854c1c8063b54383d191f8dc1f0fb29"`), []byte(`"template_sha256": "`+sum+`"`), 1)
	return os.WriteFile(manifestPath, out, 0o644)
}

func TestPrepareLessonProjectCopies(t *testing.T) {
	loaded, err := Load(templateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "lesson", "lesson_project.xml")
	if err := loaded.PrepareLessonProject(dst); err != nil {
		t.Fatalf("PrepareLessonProject: %v", err)
	}
	src, err := os.ReadFile(loaded.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != string(got) {
		t.Fatal("prepared copy differs from template (byte-identity expected)")
	}
	// place-copy: original untouched.
	after, err := os.ReadFile(loaded.TemplatePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(src) {
		t.Fatal("template asset mutated by PrepareLessonProject")
	}
}

// TestW01SeamFieldMap documents the timing seam surface consumed by W01-C:
// every design timing field has a template pin row (annotation contract).
func TestW01SeamFieldMap(t *testing.T) {
	loaded, err := Load(templateDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{
		"workspace.timing.time_signature",
		"workspace.timing.key_signature",
		"workspace.timing.tempo_bpm",
		"workspace.timing.pickup_beats",
	}
	got := make(map[string]bool)
	for _, row := range loaded.Manifest.W01Seam.FieldMap {
		got[row.DesignField] = true
	}
	for _, field := range want {
		if !got[field] {
			t.Errorf("w01_seam.field_map missing %s", field)
		}
	}
	if len(loaded.Manifest.W01Seam.TrackMap) != 5 {
		t.Errorf("track map entries = %d, want 5", len(loaded.Manifest.W01Seam.TrackMap))
	}
}

func lastIndex(haystack, needle []byte) int {
	for i := len(haystack) - len(needle); i >= 0; i-- {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
