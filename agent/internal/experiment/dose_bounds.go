package experiment

import "fmt"

// Single-point dose-ceiling constants for the bounded free-state gate
// (DOSE-AUDIBLE-1, user ruling 2026-09-28:
// decisions/2026-09-28-dose-calibration-ruling.md). Every adjustable dose
// axis is calibrated so one admitted step can land inside the listener's
// audible range; the ceiling value lives here once and the domain table
// (d1s1_domains.go), the chat/agentloop execution mirrors, the D2-2
// cumulative bound, and every hint/error wording derive from it. Editing a
// bound anywhere else recreates the drift this file removed.
const (
	// D1S1MaxAbsDeltaDB is the single-step dose ceiling shared by every
	// dB-family dose axis (track gain delta_db, band gain gain_db, the
	// threshold/attack/ceiling/range plugin rows). 10 dB ≈ perceived loudness
	// halving/doubling, so any single move inside the ceiling stays audible.
	D1S1MaxAbsDeltaDB = 10.0
	// D1S1MaxAbsDeltaPan is the single-step dose ceiling for the normalized
	// pan axis (-1..1 linear): a half-scale horizontal move is clearly
	// distinguishable in A/B comparison.
	D1S1MaxAbsDeltaPan = 0.5
)

// D1S1DBDoseRangeText renders the dB-family ceiling as wording ("+/-10 dB")
// for hints, errors, and mirror-side texts.
func D1S1DBDoseRangeText() string {
	return fmt.Sprintf("+/-%g dB", D1S1MaxAbsDeltaDB)
}

// D1S1DBDoseHintFragment renders the model-facing bound fragment for a
// dB-family dose key ("nonzero number within +/-10").
func D1S1DBDoseHintFragment() string {
	return fmt.Sprintf("nonzero number within +/-%g", D1S1MaxAbsDeltaDB)
}

// D1S1PanDoseRangeText renders the pan ceiling as wording ("+/-0.5").
func D1S1PanDoseRangeText() string {
	return fmt.Sprintf("+/-%g", D1S1MaxAbsDeltaPan)
}

// D1S1PanDoseHintFragment renders the model-facing bound fragment for the
// pan axis ("nonzero number within +/-0.5").
func D1S1PanDoseHintFragment() string {
	return fmt.Sprintf("nonzero number within +/-%g", D1S1MaxAbsDeltaPan)
}
