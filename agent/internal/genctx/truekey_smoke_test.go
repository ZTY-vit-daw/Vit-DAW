package genctx

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestTrueKeySnapshotFromStateFile is the true-key smoke gate: it assembles a
// snapshot from a REAL kernel get_project_state response captured by
// scripts/genctx_truekey_smoke_mac.sh (which drives set_tempo and re-reads
// state over kernel ZMQ) and asserts the tempo / time-signature linkage the
// card requires. Skipped unless GENCTX_SMOKE_STATE points at the capture —
// the smoke script is the only intended driver.
func TestTrueKeySnapshotFromStateFile(t *testing.T) {
	path := os.Getenv("GENCTX_SMOKE_STATE")
	if path == "" {
		t.Skip("GENCTX_SMOKE_STATE not set — true-key smoke runs via scripts/genctx_truekey_smoke_mac.sh")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state capture: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("parse state capture: %v", err)
	}
	wantTempo := os.Getenv("GENCTX_SMOKE_TEMPO") // set by the script (e.g. 96)
	wantTimeSig := os.Getenv("GENCTX_SMOKE_TIMESIG")

	snap, err := AssembleSnapshot(state, SceneInProject, CallParams{})
	if err != nil {
		t.Fatalf("assemble from true state: %v", err)
	}
	if wantTempo != "" {
		expect, err := strconv.ParseFloat(wantTempo, 64)
		if err != nil {
			t.Fatalf("bad GENCTX_SMOKE_TEMPO %q: %v", wantTempo, err)
		}
		if snap.TempoBPM != expect {
			t.Errorf("true-key tempo linkage: snapshot=%.1f want=%.1f (set_tempo must flow into music_context)", snap.TempoBPM, expect)
		}
	}
	if wantTimeSig != "" && snap.TimeSignature != wantTimeSig {
		t.Errorf("true-key time signature: snapshot=%q want=%q", snap.TimeSignature, wantTimeSig)
	}
	prompt, err := RenderGenerationPrompt(snap, "smoke user text", RenderProfile{ConsumerID: "gen_plugin"})
	if err != nil {
		t.Fatalf("render true snapshot: %v", err)
	}
	if !strings.Contains(prompt, "time_unit:\"beats\"") {
		t.Errorf("rendered prompt lost the beats MUST line:\n%s", prompt)
	}
	// The kernel ships key_signature null until GEN-CTX-KEYFIELD-1 lands:
	// assembly must treat that as unset (visible placeholder), not an error.
	if snap.KeySignature != nil {
		t.Errorf("pre-KEYFIELD kernel state must yield nil key, got %+v", snap.KeySignature)
	}
	if !strings.Contains(prompt, "key: (not set)") {
		t.Errorf("unset key must stay visible in the true-key prompt:\n%s", prompt)
	}
}
