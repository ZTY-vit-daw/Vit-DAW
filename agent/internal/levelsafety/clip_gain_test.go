package levelsafety

import (
	"math"
	"testing"
)

func TestConstrainSourceClipGainPrioritizesPeakSafetyOverRMSRaise(t *testing.T) {
	peak := -23.718
	got := ConstrainSourceClipGain(0, 24, 24, &peak)
	if math.Abs(got.TargetClipGainDB-22.718) > 0.0001 {
		t.Fatalf("target clip gain = %.6f, want 22.718", got.TargetClipGainDB)
	}
	if math.Abs(got.AppliedDeltaDB-22.718) > 0.0001 {
		t.Fatalf("applied delta = %.6f, want 22.718", got.AppliedDeltaDB)
	}
	if !got.PeakSafetyClamped || got.ClipGainBoundClamped {
		t.Fatalf("clamp flags = %+v", got)
	}
	if got.ProjectedPeakDBFS == nil || math.Abs(*got.ProjectedPeakDBFS-StaticPeakCeilingDBFS) > 0.0001 {
		t.Fatalf("projected peak = %+v", got.ProjectedPeakDBFS)
	}
	if got.PeakSafetyAchieved == nil || !*got.PeakSafetyAchieved {
		t.Fatalf("peak safety = %+v", got.PeakSafetyAchieved)
	}
}

func TestConstrainSourceClipGainRepairsUnsafeExistingAbsoluteTarget(t *testing.T) {
	peak := -23.718
	got := ConstrainSourceClipGain(24, 0, 24, &peak)
	if math.Abs(got.TargetClipGainDB-22.718) > 0.0001 || !got.PeakSafetyClamped {
		t.Fatalf("constraint = %+v", got)
	}
}

func TestConstrainSourceClipGainAccountsForExistingDownstreamFader(t *testing.T) {
	peak := -23.718
	got := ConstrainSourceClipGainWithDownstreamGain(0, 24, 24, &peak, 1.5)
	if math.Abs(got.TargetClipGainDB-21.218) > 0.0001 {
		t.Fatalf("target clip gain = %.6f, want 21.218", got.TargetClipGainDB)
	}
	if got.ProjectedPeakDBFS == nil || math.Abs(*got.ProjectedPeakDBFS-StaticPeakCeilingDBFS) > 0.0001 {
		t.Fatalf("projected peak = %+v", got.ProjectedPeakDBFS)
	}
	if !got.PeakSafetyClamped || got.PeakSafetyAchieved == nil || !*got.PeakSafetyAchieved {
		t.Fatalf("constraint = %+v", got)
	}
}
