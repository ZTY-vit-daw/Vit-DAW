package vpsforge

import (
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/probeaudio"
)

func TestStereoPlacementMetricsRecognizeDeterministicStereoProbeWindows(t *testing.T) {
	root := t.TempDir()
	if _, err := probeaudio.Generate(root, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	metrics, err := stereoPlacementMetrics(filepath.Join(root, "stereo_phase_probe_48k_10s.wav"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]stereoPlacementSegmentMetric{}
	for _, metric := range metrics {
		byName[metric.Segment] = metric
	}
	leftOnly, ok := byName["left_only"]
	if !ok || leftOnly.LeftRMSDBFS-leftOnly.RightRMSDBFS < 100 {
		t.Fatalf("left-only metrics = %#v", leftOnly)
	}
	rightOnly, ok := byName["right_only"]
	if !ok || rightOnly.RightRMSDBFS-rightOnly.LeftRMSDBFS < 100 {
		t.Fatalf("right-only metrics = %#v", rightOnly)
	}
	inPhase, ok := byName["in_phase"]
	if !ok || inPhase.MidRMSDBFS-inPhase.SideRMSDBFS < 100 {
		t.Fatalf("in-phase metrics = %#v", inPhase)
	}
	oppositePhase, ok := byName["opposite_phase"]
	if !ok || oppositePhase.SideRMSDBFS-oppositePhase.MidRMSDBFS < 100 {
		t.Fatalf("opposite-phase metrics = %#v", oppositePhase)
	}
}

func TestStereoPlacementMetricDeltasAlignByWindow(t *testing.T) {
	bypass := []stereoPlacementSegmentMetric{{Segment: "left_only", LeftRMSDBFS: -20, RightRMSDBFS: -30, MidRMSDBFS: -22, SideRMSDBFS: -25}}
	processed := []stereoPlacementSegmentMetric{{Segment: "left_only", LeftRMSDBFS: -10, RightRMSDBFS: -29, MidRMSDBFS: -14, SideRMSDBFS: -19}}
	deltas := stereoMetricDeltas(bypass, processed)
	if len(deltas) != 1 || deltas[0].LeftDeltaDB == nil || deltas[0].RightDeltaDB == nil || deltas[0].MidDeltaDB == nil || deltas[0].SideDeltaDB == nil || *deltas[0].LeftDeltaDB != 10 || *deltas[0].RightDeltaDB != 1 || *deltas[0].MidDeltaDB != 8 || *deltas[0].SideDeltaDB != 6 {
		t.Fatalf("deltas = %#v", deltas)
	}
}

func TestStereoPlacementMetricDeltaMarksSilentReferenceAsNotApplicable(t *testing.T) {
	deltas := stereoMetricDeltas(
		[]stereoPlacementSegmentMetric{{Segment: "left_only", LeftRMSDBFS: -180, RightRMSDBFS: -20, MidRMSDBFS: -180, SideRMSDBFS: -20}},
		[]stereoPlacementSegmentMetric{{Segment: "left_only", LeftRMSDBFS: -10, RightRMSDBFS: -10, MidRMSDBFS: -10, SideRMSDBFS: -10}},
	)
	if len(deltas) != 1 || deltas[0].LeftDeltaDB != nil || deltas[0].MidDeltaDB != nil || len(deltas[0].SilentReferenceFields) != 2 {
		t.Fatalf("silent-reference deltas = %#v", deltas)
	}
}
