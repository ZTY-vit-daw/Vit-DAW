package mixboard

import (
	"sync/atomic"

	"vit-daw-agent/internal/logx"
)

// MAT-0 observation finalize counters (docs/MATERIALIZATION_V1_DESIGN.md §5.6):
// per-kind call counts for every projection finalize step orchestrated by
// FinalizeObservationContext, emitted as [materialize.metrics] log lines.
// Discipline (Notifier-nil pattern): counting and logging must not alter any
// finalize behavior, order, or return value; a nil logger is inert while
// counting continues; the counting on/off two states must produce
// byte-identical projection output (locked by observation_metrics_test.go).

const (
	finalizeKindDOM = iota
	finalizeKindMOM
	finalizeKindTIM
	finalizeKindFXM
	finalizeKindCOM
	finalizeKindCount
)

var finalizeKindNames = [finalizeKindCount]string{"dom", "mom", "tim", "fxm", "com"}

var observationFinalizeCounts [finalizeKindCount]atomic.Uint64

var observationMetricsEnabled atomic.Bool

// observationMetricsLogger follows the Notifier-nil discipline: logx methods
// are no-ops on a nil receiver, so an unwired logger changes nothing.
var observationMetricsLogger *logx.Logger

func init() {
	observationMetricsEnabled.Store(true)
}

// SetObservationMetricsLogger wires the structured metrics log; call once at
// process startup. The metrics logger is deliberately separate from the main
// agent log so a full day of baseline counting survives its larger ring.
func SetObservationMetricsLogger(logger *logx.Logger) {
	observationMetricsLogger = logger
}

// noteObservationFinalize trails each finalize call in the assembly chain:
// the increment stays in lockstep with the call, and the log line reports the
// cumulative per-kind count. When metrics are disabled the whole note is a
// no-op bypass so the finalize path is untouched.
func noteObservationFinalize(kind int) {
	if !observationMetricsEnabled.Load() {
		return
	}
	count := observationFinalizeCounts[kind].Add(1)
	observationMetricsLogger.Info("[materialize.metrics] kind=%s finalize_count=%d", finalizeKindNames[kind], count)
}

func observationFinalizeCount(kind int) uint64 {
	return observationFinalizeCounts[kind].Load()
}

func resetObservationFinalizeCounts() {
	for i := range observationFinalizeCounts {
		observationFinalizeCounts[i].Store(0)
	}
}

func setObservationMetricsEnabled(enabled bool) {
	observationMetricsEnabled.Store(enabled)
}
