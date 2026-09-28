package chat

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
)

// DOSE-AUDIBLE-1 chat-layer mirror seals: the mix-tick confirmation gate and
// the plugin-candidate disclosure wording must carry the same audible-priority
// ceiling the experiment domain table enforces (constants in
// internal/experiment/dose_bounds.go).

func doseMirrorGainCandidate(delta float64) agentloop.PendingMixTickCandidate {
	return agentloop.PendingMixTickCandidate{Operation: "track_gain_adjust", TrackID: "track-1", DeltaDB: delta, Status: "pending_confirmation"}
}

func doseMirrorPanCandidate(delta float64) agentloop.PendingMixTickCandidate {
	return agentloop.PendingMixTickCandidate{Operation: "track_pan_adjust", TrackID: "track-1", DeltaPan: delta, Status: "pending_confirmation"}
}

func TestValidatePendingMixTickCandidateHoldsAudibleCeiling(t *testing.T) {
	server := &Server{}
	ctx := context.Background()
	for _, edge := range []float64{experiment.D1S1MaxAbsDeltaDB, -experiment.D1S1MaxAbsDeltaDB} {
		if err := server.validatePendingMixTickCandidate(ctx, doseMirrorGainCandidate(edge)); err != nil {
			t.Fatalf("gain ceiling endpoint %g refused: %v", edge, err)
		}
	}
	for _, edge := range []float64{experiment.D1S1MaxAbsDeltaPan, -experiment.D1S1MaxAbsDeltaPan} {
		if err := server.validatePendingMixTickCandidate(ctx, doseMirrorPanCandidate(edge)); err != nil {
			t.Fatalf("pan ceiling endpoint %g refused: %v", edge, err)
		}
	}
	beyondGain := experiment.D1S1MaxAbsDeltaDB + 0.5
	err := server.validatePendingMixTickCandidate(ctx, doseMirrorGainCandidate(beyondGain))
	if err == nil || !strings.Contains(err.Error(), experiment.D1S1DBDoseRangeText()) {
		t.Fatalf("beyond-ceiling gain %g must be refused with the constant-derived bound: %v", beyondGain, err)
	}
	beyondPan := experiment.D1S1MaxAbsDeltaPan + 0.05
	err = server.validatePendingMixTickCandidate(ctx, doseMirrorPanCandidate(beyondPan))
	if err == nil || !strings.Contains(err.Error(), experiment.D1S1PanDoseRangeText()) {
		t.Fatalf("beyond-ceiling pan %g must be refused with the constant-derived bound: %v", beyondPan, err)
	}
	if err := server.validatePendingMixTickCandidate(ctx, doseMirrorGainCandidate(0)); err == nil {
		t.Fatal("zero gain delta must be refused")
	}
	if err := server.validatePendingMixTickCandidate(ctx, doseMirrorPanCandidate(0)); err == nil {
		t.Fatal("zero pan delta must be refused")
	}
}

// Every disclosed family capability face must restate the constant-derived
// ceiling, so the proposal-time wording cannot drift from the admission gate.
func TestPluginCandidateDisclosureCapabilityTextMatchesDoseCeiling(t *testing.T) {
	if len(freeStatePluginCandidateCapabilities) == 0 {
		t.Fatal("plugin candidate capability map is empty")
	}
	for domain, capability := range freeStatePluginCandidateCapabilities {
		if !strings.Contains(capability, experiment.D1S1DBDoseRangeText()) {
			t.Fatalf("domain %s capability wording lost the constant-derived bound: %q", domain, capability)
		}
	}
}

// The D2-2 chat-side cumulative refusal mirrors the experiment-side bound;
// both must derive from the same ceiling constant.
func TestD2MultiRoundChatCumulativeBoundFollowsAudibleCeiling(t *testing.T) {
	if d2MultiRoundChatCumulativeBoundDB != experiment.D1S1MaxAbsDeltaDB {
		t.Fatalf("chat cumulative bound %g drifted from the audible single-step ceiling %g", d2MultiRoundChatCumulativeBoundDB, experiment.D1S1MaxAbsDeltaDB)
	}
	if !strings.Contains(fmt.Sprintf("%v", experiment.D1S1MaxAbsDeltaDB), "10") {
		t.Fatalf("sanity: ceiling constant unexpectedly rendered")
	}
}
