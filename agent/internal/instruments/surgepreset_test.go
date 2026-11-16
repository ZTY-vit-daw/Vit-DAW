package instruments

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFXP writes a minimal fixture patch file with the VST chunk preset
// header (CcnK .. FPCh) when valid, or garbage when invalid.
func writeFXP(t *testing.T, dir, name string, valid bool) {
	t.Helper()
	path := filepath.Join(dir, name)
	header := make([]byte, 56)
	if valid {
		copy(header[0:4], "CcnK")
		copy(header[8:12], "FPCh")
		copy(header[16:20], "cjs3")
	}
	if err := os.WriteFile(path, header, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func buildFixtureFactory(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"Keys", "Basses", "Percussion", "Leads", "Polysynths", "Templates"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir fixture category: %v", err)
		}
	}
	writeFXP(t, filepath.Join(root, "Keys"), "EP 1.fxp", true)
	writeFXP(t, filepath.Join(root, "Keys"), "EP 2.fxp", true)
	writeFXP(t, filepath.Join(root, "Keys"), "Organ 1.fxp", true)
	writeFXP(t, filepath.Join(root, "Keys"), "Digi Harpsi.fxp", true)
	writeFXP(t, filepath.Join(root, "Keys"), "Church.fxp", true)
	writeFXP(t, filepath.Join(root, "Keys"), "Broken.fxp", false)
	writeFXP(t, filepath.Join(root, "Basses"), "Bass 1.fxp", true)
	writeFXP(t, filepath.Join(root, "Percussion"), "Hit.fxp", true)
	writeFXP(t, filepath.Join(root, "Leads"), "Saw.fxp", true)
	writeFXP(t, filepath.Join(root, "Polysynths"), "Poly.fxp", true)
	writeFXP(t, filepath.Join(root, "Templates"), "Init.fxp", true)
	if err := os.WriteFile(filepath.Join(root, "Keys", "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-fxp fixture: %v", err)
	}
	return root
}

func TestBuildFactoryIndexCountsAndOrder(t *testing.T) {
	root := buildFixtureFactory(t)
	index, err := BuildFactoryIndex(root)
	if err != nil {
		t.Fatalf("BuildFactoryIndex: %v", err)
	}
	if index.InvalidCount != 1 {
		t.Fatalf("InvalidCount = %d, want 1", index.InvalidCount)
	}
	var got []string
	for _, patch := range index.Patches {
		got = append(got, patch.Category+"/"+patch.Name)
	}
	want := []string{
		"Basses/Bass 1", "Keys/Broken", "Keys/Church", "Keys/Digi Harpsi",
		"Keys/EP 1", "Keys/EP 2", "Keys/Organ 1", "Leads/Saw",
		"Percussion/Hit", "Polysynths/Poly", "Templates/Init",
	}
	if len(got) != len(want) {
		t.Fatalf("patch count = %d (%v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("patch[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
	for _, patch := range index.Patches {
		wantOK := patch.Name != "Broken"
		if patch.HeaderOK != wantOK {
			t.Fatalf("patch %s HeaderOK = %v, want %v", patch.Name, patch.HeaderOK, wantOK)
		}
	}
}

func TestBuildFactoryIndexMissingRoot(t *testing.T) {
	_, err := BuildFactoryIndex(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatalf("missing root must error so callers degrade honestly")
	}
}

func TestDefaultFactoryRootEnvOverride(t *testing.T) {
	t.Setenv(factoryRootEnv, "")
	if DefaultFactoryRoot() == "" && os.Getenv("ProgramData") != "" {
		t.Fatalf("default root should follow ProgramData when no override is set")
	}
	t.Setenv(factoryRootEnv, `D:\custom\surge\patches_factory`)
	if got := DefaultFactoryRoot(); got != `D:\custom\surge\patches_factory` {
		t.Fatalf("DefaultFactoryRoot override = %q", got)
	}
}

func TestCategoryCounts(t *testing.T) {
	root := buildFixtureFactory(t)
	index, err := BuildFactoryIndex(root)
	if err != nil {
		t.Fatalf("BuildFactoryIndex: %v", err)
	}
	counts := index.CategoryCounts()
	byCategory := map[string]CategoryCount{}
	for _, row := range counts {
		byCategory[row.Category] = row
	}
	if row := byCategory["Keys"]; row.Count != 6 || row.Invalid != 1 {
		t.Fatalf("Keys tally = %+v, want count 6 invalid 1", row)
	}
	if row := byCategory["Basses"]; row.Count != 1 || len(row.Semantics) != 1 || row.Semantics[0] != ClassBass {
		t.Fatalf("Basses tally = %+v", row)
	}
	if row := byCategory["Templates"]; row.Count != 1 || row.Semantics != nil {
		t.Fatalf("Templates tally = %+v, want no semantics", row)
	}
	for i := 1; i < len(counts); i++ {
		if counts[i].Category <= counts[i-1].Category {
			t.Fatalf("categories not sorted: %v", counts)
		}
	}
}

func TestPatchSemantics(t *testing.T) {
	cases := []struct {
		category, name string
		want           Class
		ok             bool
	}{
		{"Keys", "EP 1", ClassEPiano, true},
		{"Keys", "DX EP", ClassEPiano, true},
		{"Keys", "Soft Suitcase", ClassEPiano, true},
		{"Keys", "Organ 1", ClassOrgan, true},
		{"Keys", "House Organ", ClassOrgan, true},
		{"Keys", "Digi Harpsi", ClassHarpsi, true},
		{"Keys", "Church", ClassKeysMisc, true},
		{"Basses", "Bass 1", ClassBass, true},
		{"Percussion", "Hit", ClassDrums, true},
		{"Leads", "Saw", ClassLead, true},
		{"Templates", "Init", ClassUnknown, false},
		{"MPE", "Whatever", ClassUnknown, false},
	}
	for _, tc := range cases {
		class, ok := PatchSemantics(FactoryPatch{Category: tc.category, Name: tc.name})
		if class != tc.want || ok != tc.ok {
			t.Fatalf("PatchSemantics(%s/%s) = %q ok=%v, want %q ok=%v", tc.category, tc.name, class, ok, tc.want, tc.ok)
		}
	}
}

func TestFilterByClassSkipsInvalidHeaders(t *testing.T) {
	root := buildFixtureFactory(t)
	index, err := BuildFactoryIndex(root)
	if err != nil {
		t.Fatalf("BuildFactoryIndex: %v", err)
	}
	ep := index.FilterByClass(ClassEPiano)
	if len(ep) != 2 {
		t.Fatalf("epiano patches = %d, want 2 (EP 1, EP 2)", len(ep))
	}
	for _, patch := range ep {
		if !patch.HeaderOK {
			t.Fatalf("invalid-header patch leaked into filter: %+v", patch)
		}
	}
	if got := index.FilterByClass(ClassPiano); len(got) != 0 {
		t.Fatalf("piano patches = %d, want 0 (Surge Keys has no acoustic piano)", len(got))
	}
}
