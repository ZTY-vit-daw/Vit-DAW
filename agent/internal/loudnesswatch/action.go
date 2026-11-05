package loudnesswatch

// Breach-to-action suggestion (card AF-D1-LOUDNESS-SETTLE-1). Pure function:
// a breach verdict plus the measurement rows in, one bounded fader suggestion
// out. The suggestion is advisory only — the caller parks it on the existing
// pending-confirmation chain (mix.propose_tick -> confirm -> mix.apply_tick);
// nothing here writes any project state.
//
// Design pins:
//   - only a breach verdict suggests an action (pass/unmeasurable are
//     check outcomes, not repair work);
//   - the target is the carrier of the LARGEST signed breach (the clip that
//     pushed the project out of bounds the most), mapped back to its track
//     through the same rows the comparator consumed;
//   - the suggested delta is the negated breach delta clamped to
//     maxAbsDeltaDB (v1 dose bound comes from the caller — the watch package
//     stays free of experiment-package imports);
//   - no resolvable track for the worst breach means no suggestion (an
//     unaddressable target is disclosed honestly, never invented).

import (
	"fmt"
	"math"
	"strings"
)

// ActionOperationGainAdjust is the only v1 suggestion operation: a bounded
// track gain move through the existing mix-tick confirmation chain.
const ActionOperationGainAdjust = "track_gain_adjust"

// ActionSuggestion is one bounded, confirmable repair move derived from a
// breach verdict. DeltaDB is signed: negative pulls the track down (ceiling
// breach), positive pushes it up (floor breach).
type ActionSuggestion struct {
	Operation string  `json:"operation"`
	TrackID   string  `json:"track_id"`
	ClipID    string  `json:"clip_id,omitempty"`
	DeltaDB   float64 `json:"delta_db"`
	Metric    string  `json:"metric"`
	Measured  float64 `json:"measured"`
	Threshold float64 `json:"threshold"`
	// Basis is the user-visible one-liner carrying the approximate-grade
	// qualifier (same honesty discipline as Verdict.Summary).
	Basis string `json:"basis"`
}

// SuggestGainAction derives the bounded fader suggestion for the worst
// breach, or returns nil when no actionable suggestion exists (verdict not a
// breach, no breach details, dose clamped to zero, or the worst-breach clip
// has no resolvable track in rows). maxAbsDeltaDB <= 0 disables the action
// face (nil for every verdict).
func SuggestGainAction(verdict Verdict, rows []map[string]any, maxAbsDeltaDB float64) *ActionSuggestion {
	if verdict.Verdict != VerdictBreach || len(verdict.BreachDetails) == 0 || maxAbsDeltaDB <= 0 {
		return nil
	}
	worst := verdict.BreachDetails[0]
	for _, detail := range verdict.BreachDetails[1:] {
		if math.Abs(detail.DeltaDB) > math.Abs(worst.DeltaDB) {
			worst = detail
		}
	}
	trackID := trackIDForClip(rows, worst.ClipID)
	if trackID == "" {
		return nil
	}
	delta := clampDelta(-worst.DeltaDB, maxAbsDeltaDB)
	if delta == 0 {
		return nil
	}
	return &ActionSuggestion{
		Operation: ActionOperationGainAdjust,
		TrackID:   trackID,
		ClipID:    worst.ClipID,
		DeltaDB:   delta,
		Metric:    worst.Metric,
		Measured:  worst.Measured,
		Threshold: worst.Threshold,
		Basis:     suggestionBasis(worst, delta),
	}
}

// trackIDForClip resolves the owning track of a clip through the same
// loudness_summary rows the comparator consumed. An empty clip id or a clip
// with no row leaves the target unaddressable (empty).
func trackIDForClip(rows []map[string]any, clipID string) string {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return ""
	}
	for _, row := range rows {
		if strings.TrimSpace(firstStringFromAny(row, "clip_id")) == clipID {
			if track := strings.TrimSpace(firstStringFromAny(row, "track_id")); track != "" {
				return track
			}
		}
	}
	return ""
}

func clampDelta(delta, maxAbs float64) float64 {
	if delta > maxAbs {
		return maxAbs
	}
	if delta < -maxAbs {
		return -maxAbs
	}
	return delta
}

// suggestionBasis keeps the approximate-grade qualifier on the action line:
// the suggestion inherits the verdict's honesty boundary, never promising a
// compliant re-measure (the recheck after the move is a new verdict).
func suggestionBasis(detail BreachDetail, delta float64) string {
	direction := "拉低"
	if delta > 0 {
		direction = "推高"
	}
	return fmt.Sprintf("近似口径：clip %s 的 %s %.1f 超出阈值 %.1f（%+.1f dB），建议先把所在轨道%s %.1f dB（待确认后执行，非合规承诺）",
		detail.ClipID, detail.Metric, detail.Measured, detail.Threshold, detail.DeltaDB, direction, math.Abs(delta))
}
