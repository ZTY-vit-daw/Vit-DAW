package experimentplugins

import "testing"

// THROWAWAY 322 sufficiency proof (never merged; forensic artifact): a
// static_eq entry for FabFilter Pro-Q 3 whose single band anchor is backed by
// the 2026-11-09 EQ-channel certification receipt (apply.json verified writes:
// freq param 2 @3400 Hz, gain param 3 @-3 dB; dual-cycle probe stable_id both
// cycles, zero identity drift) must pass Load's strict decoder + validation so
// the free-state candidate disclosure can list it.
func TestProQ3SufficiencyProofWhitelistLoad(t *testing.T) {
	wl, err := Load(`C:\Users\timoz\AppData\Local\Temp\proq3_whitelist_sufficiency.json`)
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
