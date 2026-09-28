package agentloop

import (
	"testing"

	"vit-daw-agent/internal/experiment"
)

// DOSE-AUDIBLE-1 agentloop-layer mirror seal: the implicit-path pan clamp
// carries the same audible-priority ceiling the experiment domain table
// enforces (constants in internal/experiment/dose_bounds.go).

func TestClampPanDeltaHoldsAudibleCeiling(t *testing.T) {
	if got := clampPanDelta(experiment.D1S1MaxAbsDeltaPan + 0.1); got != experiment.D1S1MaxAbsDeltaPan {
		t.Fatalf("pan clamp above the ceiling: got %g, want %g", got, experiment.D1S1MaxAbsDeltaPan)
	}
	if got := clampPanDelta(-experiment.D1S1MaxAbsDeltaPan - 0.1); got != -experiment.D1S1MaxAbsDeltaPan {
		t.Fatalf("pan clamp below the ceiling: got %g, want %g", got, -experiment.D1S1MaxAbsDeltaPan)
	}
	if got := clampPanDelta(experiment.D1S1MaxAbsDeltaPan); got != experiment.D1S1MaxAbsDeltaPan {
		t.Fatalf("pan clamp must preserve the ceiling endpoint: got %g", got)
	}
	if got := clampPanDelta(experiment.D1S1MaxAbsDeltaPan / 2); got != experiment.D1S1MaxAbsDeltaPan/2 {
		t.Fatalf("pan clamp must preserve in-ceiling moves: got %g", got)
	}
}
