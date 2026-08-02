package levelsafety

import "math"

const StaticPeakCeilingDBFS = -1.0

type ClipGainConstraint struct {
	TargetClipGainDB     float64
	AppliedDeltaDB       float64
	ClipGainBoundClamped bool
	PeakSafetyClamped    bool
	ProjectedPeakDBFS    *float64
	PeakSafetyAchieved   *bool
}

// ConstrainSourceClipGain keeps an absolute clip-gain target within both the
// writable clip-gain range and the static source peak ceiling. sourcePeakDBFS
// is the pre-clip-gain source peak, so projected static peak is source peak
// plus the absolute target clip gain.
func ConstrainSourceClipGain(currentClipGainDB, requestedDeltaDB, clipGainBoundDB float64, sourcePeakDBFS *float64) ClipGainConstraint {
	target := currentClipGainDB + requestedDeltaDB
	boundClamped := false
	if clipGainBoundDB > 0 {
		if target > clipGainBoundDB {
			target = clipGainBoundDB
			boundClamped = true
		} else if target < -clipGainBoundDB {
			target = -clipGainBoundDB
			boundClamped = true
		}
	}

	peakClamped := false
	var projectedPeak *float64
	var peakSafetyAchieved *bool
	if sourcePeakDBFS != nil && !math.IsNaN(*sourcePeakDBFS) && !math.IsInf(*sourcePeakDBFS, 0) {
		maxSafeTarget := StaticPeakCeilingDBFS - *sourcePeakDBFS
		if target > maxSafeTarget {
			target = maxSafeTarget
			peakClamped = true
			if clipGainBoundDB > 0 && target < -clipGainBoundDB {
				target = -clipGainBoundDB
				boundClamped = true
			}
		}
		projected := *sourcePeakDBFS + target
		achieved := projected <= StaticPeakCeilingDBFS+1e-9
		projectedPeak = &projected
		peakSafetyAchieved = &achieved
	}

	return ClipGainConstraint{
		TargetClipGainDB:     target,
		AppliedDeltaDB:       target - currentClipGainDB,
		ClipGainBoundClamped: boundClamped,
		PeakSafetyClamped:    peakClamped,
		ProjectedPeakDBFS:    projectedPeak,
		PeakSafetyAchieved:   peakSafetyAchieved,
	}
}
