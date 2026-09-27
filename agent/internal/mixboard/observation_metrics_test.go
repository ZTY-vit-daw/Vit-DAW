package mixboard

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/logx"
)

// MAT-0 zero-behavior lock: the per-kind finalize counters and
// [materialize.metrics] logging must not change any finalize behavior, order,
// or return value. These tests pin counting-to-call sync and byte-identical
// projection output across the counting on/off two states.

func matZeroRequest(args map[string]any) Request {
	return Request{MixSessionID: "mix-mat0", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: args}
}

func matZeroCounts() map[string]uint64 {
	return map[string]uint64{
		"dom": observationFinalizeCount(finalizeKindDOM),
		"mom": observationFinalizeCount(finalizeKindMOM),
		"tim": observationFinalizeCount(finalizeKindTIM),
		"fxm": observationFinalizeCount(finalizeKindFXM),
		"com": observationFinalizeCount(finalizeKindCOM),
	}
}

func TestObservationMetricsCountsFinalizeCallsPerKind(t *testing.T) {
	resetObservationFinalizeCounts()
	defer resetObservationFinalizeCounts()
	setObservationMetricsEnabled(true)
	defer setObservationMetricsEnabled(true)

	fixedNow := "2026-09-28T00:00:00Z"
	// Plain request: DOM runs via the no-dom-mode fallback branch.
	BuildObservation(matZeroRequest(map[string]any{"feature_snapshot": comSourceFeatureSnapshot("ready")}), fixedNow)
	// Explicit dom_mode: DOM runs via the early branch; com_mode builds a real COM projection.
	BuildObservation(matZeroRequest(map[string]any{
		"dom_mode": dom.ModeSourceOnly, "com_mode": com.ModeSourceOnly,
		"feature_snapshot": comSourceFeatureSnapshot("ready"),
	}), fixedNow)
	// Band/stereo projection mode: DOM does not finalize at all.
	BuildObservation(matZeroRequest(map[string]any{
		"projection": "band_stereo", "include_raw": false,
		"feature_snapshot": comSourceFeatureSnapshot("ready"),
	}), fixedNow)

	want := map[string]uint64{"dom": 2, "mom": 3, "tim": 3, "fxm": 3, "com": 3}
	got := matZeroCounts()
	for kind, count := range want {
		if got[kind] != count {
			t.Fatalf("finalize count kind=%s = %d, want %d (all: %v)", kind, got[kind], count, got)
		}
	}
}

func TestObservationMetricsCountingTwoStatesProduceIdenticalOutput(t *testing.T) {
	resetObservationFinalizeCounts()
	defer resetObservationFinalizeCounts()

	fixedNow := "2026-09-28T01:00:00Z"
	req := matZeroRequest(map[string]any{
		"dom_mode": dom.ModeSourceOnly, "com_mode": com.ModeSourceOnly,
		"feature_snapshot": comSourceFeatureSnapshot("ready"),
	})
	base := BuildObservation(req, fixedNow)

	deepCopyObservation := func() ObservationPacket {
		data, err := json.Marshal(base)
		if err != nil {
			t.Fatal(err)
		}
		var copy ObservationPacket
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		return copy
	}

	setObservationMetricsEnabled(false)
	offObs := deepCopyObservation()
	FinalizeObservationContext(&offObs, req, fixedNow)

	setObservationMetricsEnabled(true)
	onObs := deepCopyObservation()
	FinalizeObservationContext(&onObs, req, fixedNow)
	defer setObservationMetricsEnabled(true)

	offJSON, err := json.Marshal(offObs)
	if err != nil {
		t.Fatal(err)
	}
	onJSON, err := json.Marshal(onObs)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(offJSON, onJSON) {
		t.Fatalf("counting changed projection output:\noff(%d bytes)=%s\non(%d bytes)=%s",
			len(offJSON), truncateForDiff(offJSON), len(onJSON), truncateForDiff(onJSON))
	}

	// The on-state run must actually have counted, otherwise the byte
	// comparison above proved nothing.
	got := matZeroCounts()
	for _, kind := range finalizeKindNames {
		if got[kind] == 0 {
			t.Fatalf("on-state run left kind=%s uncounted: %v", kind, got)
		}
	}
}

func truncateForDiff(data []byte) string {
	const limit = 400
	if len(data) <= limit {
		return string(data)
	}
	return string(data[:limit]) + "..."
}

func TestObservationMetricsEmitsStructuredLogLines(t *testing.T) {
	resetObservationFinalizeCounts()
	defer resetObservationFinalizeCounts()
	previousLogger := observationMetricsLogger
	defer SetObservationMetricsLogger(previousLogger)
	setObservationMetricsEnabled(true)
	defer setObservationMetricsEnabled(true)

	logPath := filepath.Join(t.TempDir(), "MaterializeMetrics.log")
	SetObservationMetricsLogger(logx.New(false, logPath, 64))

	BuildObservation(matZeroRequest(map[string]any{
		"dom_mode":         dom.ModeSourceOnly,
		"feature_snapshot": comSourceFeatureSnapshot("ready"),
	}), "2026-09-28T02:00:00Z")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(data)
	for _, want := range []string{
		"[INFO] [materialize.metrics] kind=dom finalize_count=1",
		"[materialize.metrics] kind=mom finalize_count=1",
		"[materialize.metrics] kind=tim finalize_count=1",
		"[materialize.metrics] kind=fxm finalize_count=1",
		"[materialize.metrics] kind=com finalize_count=1",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("metrics log line %q missing from:\n%s", want, logText)
		}
	}
}
