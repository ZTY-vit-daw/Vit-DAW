package experimentplugins

import (
	"os"
	"path/filepath"
	"testing"
)

// THROWAWAY 328 verification (forensic artifact; card FIX-FREEFLOAT-EQ-ANCHOR-1):
// after the 328 re-derive (8c0ef051 rule already in main) + surgical apply of the
// TBTECH Kirchhoff-EQ row, the REAL live whitelist must load through
// experimentplugins.Load's strict decoder + validation and expose TBTECH in the
// static_eq section with the receipt-self-certified single-band anchor
// (apply.json verified upsert writes: freq physical 3400 Hz pid 55577, gain
// pid 55578 — anchor {3400.0, "55578", "55578"} verbatim from derive output).
// The 322 Pro-Q 3 row (first receipt-backed free_floating anchor) must remain.
func TestTBTECHLiveWhitelistLoad(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home: %v", err)
	}
	live := filepath.Join(home, ".vit", "free_state_experiment_plugins.json")
	wl, err := Load(live)
	if err != nil {
		t.Fatalf("Load rejected the live whitelist: %v", err)
	}
	foundTBTECH := false
	foundProQ3 := false
	for _, e := range wl.StaticEQ {
		switch e.PluginIdentifier {
		case "VST3-TBTECH Kirchhoff-EQ-be41c239-bdb35085":
			foundTBTECH = true
			if len(e.Bands) != 1 || e.Bands[0].CenterHz != 3400.0 ||
				e.Bands[0].GainParamIDCH1 != "55578" || e.Bands[0].GainParamIDCH2 != "55578" {
				t.Fatalf("TBTECH band anchor mismatch: %+v", e.Bands)
			}
		case "VST3-Pro-Q 3-a1aba170-ba5f7ab2":
			foundProQ3 = true
			if len(e.Bands) != 1 || e.Bands[0].CenterHz != 3400.0 {
				t.Fatalf("Pro-Q 3 band anchor regressed: %+v", e.Bands)
			}
		}
	}
	if !foundTBTECH {
		t.Fatal("TBTECH Kirchhoff-EQ missing from live static_eq section")
	}
	if !foundProQ3 {
		t.Fatal("Pro-Q 3 missing from live static_eq section (322 row regressed)")
	}
	if len(wl.StaticEQ) != 50 {
		t.Fatalf("static_eq count = %d, want 50", len(wl.StaticEQ))
	}
	if len(wl.Reverb) != 1 || len(wl.Delay) != 1 {
		t.Fatalf("reverb/delay sections changed: %+v %+v", wl.Reverb, wl.Delay)
	}
}
