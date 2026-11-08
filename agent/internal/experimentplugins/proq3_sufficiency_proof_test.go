package experimentplugins

import (
	"os"
	"path/filepath"
	"testing"
)

// THROWAWAY 322 verification (never merged; forensic artifact): after the
// 322 in-card derive fix + surgical apply, the REAL live whitelist must load
// through experimentplugins.Load's strict decoder + validation AND expose
// Pro-Q 3 in the static_eq section (the free-state candidate disclosure
// projects identity fields from exactly this list).
func TestProQ3LiveWhitelistLoad(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home: %v", err)
	}
	live := filepath.Join(home, ".vit", "free_state_experiment_plugins.json")
	wl, err := Load(live)
	if err != nil {
		t.Fatalf("Load rejected the live whitelist: %v", err)
	}
	found := false
	for _, e := range wl.StaticEQ {
		if e.PluginIdentifier == "VST3-Pro-Q 3-a1aba170-ba5f7ab2" {
			found = true
			if len(e.Bands) != 1 || e.Bands[0].CenterHz != 3400.0 {
				t.Fatalf("band anchor mismatch: %+v", e.Bands)
			}
		}
	}
	if !found {
		t.Fatal("Pro-Q 3 missing from live static_eq section")
	}
	if len(wl.StaticEQ) != 49 {
		t.Fatalf("static_eq count = %d, want 49", len(wl.StaticEQ))
	}
	if len(wl.Reverb) != 1 || len(wl.Delay) != 1 {
		t.Fatalf("reverb/delay sections changed: %+v %+v", wl.Reverb, wl.Delay)
	}
}

// The original sufficiency proof (temp-file form) stays below unchanged.
func TestProQ3SufficiencyProofWhitelistLoad(t *testing.T) {
	path := `C:\Users\timoz\AppData\Local\Temp\proq3_whitelist_sufficiency.json`
	if _, err := os.Stat(path); err != nil {
		t.Skipf("temp candidate missing: %v", err)
	}
	wl, err := Load(path)
	if err != nil {
		t.Fatalf("Load rejected the candidate whitelist: %v", err)
	}
	found := false
	for _, e := range wl.StaticEQ {
		if e.PluginIdentifier == "VST3-Pro-Q 3-a1aba170-ba5f7ab2" {
			found = true
			if len(e.Bands) != 1 || e.Bands[0].CenterHz != 3400.0 {
				t.Fatalf("band anchor mismatch: %+v", e.Bands)
			}
		}
	}
	if !found {
		t.Fatal("Pro-Q 3 not present in loaded static_eq section")
	}
	if len(wl.StaticEQ) != 49 {
		t.Fatalf("static_eq count = %d, want 49", len(wl.StaticEQ))
	}
}
