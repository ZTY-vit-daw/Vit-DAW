package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
)

// M2-TIMEFX-2 producer wiring tests: the post-write differential probe is
// best-effort evidence enrichment. Without a kernel-backed harness it must
// disclose unavailability; it must never mutate a nil result map or panic on
// an empty recognizer identity.

func TestAttachTimefxFXMProbeDisclosesUnavailableHarness(t *testing.T) {
	server := &Server{}
	result := map[string]any{"status": "exact"}
	server.attachTimefxFXMProbe(context.Background(), result, "1007", "1021", "reverb", "decay_time_reverb", "gen-1")
	probe, ok := result["fxm_probe"].(map[string]any)
	if !ok {
		t.Fatalf("fxm_probe missing: %+v", result)
	}
	if probe["status"] != "failed" || probe["reason"] != "harness_unavailable" {
		t.Fatalf("outcome = %+v", probe)
	}
}

func TestAttachTimefxFXMProbeRequiresRecognizerIdentity(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil)}
	result := map[string]any{"status": "exact"}
	server.attachTimefxFXMProbe(context.Background(), result, "1007", "1021", "reverb", "", "")
	probe, ok := result["fxm_probe"].(map[string]any)
	if !ok {
		t.Fatalf("fxm_probe missing: %+v", result)
	}
	if probe["status"] != "failed" || !strings.Contains(probe["reason"].(string), "recognizer_identity_unavailable") {
		t.Fatalf("outcome = %+v", probe)
	}
	if probe["family"] != "reverb" || probe["track_id"] != "1007" {
		t.Fatalf("identity not disclosed: %+v", probe)
	}
}

func TestAttachTimefxFXMProbeToleratesNilResult(t *testing.T) {
	server := &Server{}
	server.attachTimefxFXMProbe(context.Background(), nil, "1007", "1021", "delay", "time_delay", "gen-1")
}
