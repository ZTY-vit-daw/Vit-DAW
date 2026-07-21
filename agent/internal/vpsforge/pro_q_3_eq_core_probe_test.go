package vpsforge

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/probeaudio"
)

func TestProQ3CoreEQImpulseResponseIsNeutralForMatchingFiles(t *testing.T) {
	root := t.TempDir()
	if _, err := probeaudio.Generate(root, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "impulse_stereo_48k_2s.wav")
	copyPath := filepath.Join(root, "matching-copy.wav")
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	response, err := proQ3CoreEQImpulseResponse(input, copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(response) != len(proQ3CoreEQResponseFrequencies) {
		t.Fatalf("response count = %d", len(response))
	}
	for _, point := range response {
		if math.Abs(point.DeltaDB) > 0.00001 {
			t.Fatalf("matching files produced a non-neutral response at %#v", point)
		}
	}
}

func TestProQ3CoreEQChoiceSelectionStaysDisplayBounded(t *testing.T) {
	choices := []proQ3CoreEQChoice{
		{RequestedNormalized: 0, ObservedDisplay: "Bell"},
		{RequestedNormalized: 0.5, ObservedDisplay: "Low Cut"},
		{RequestedNormalized: 1, ObservedDisplay: "High Cut"},
	}
	if bell, ok := proQ3CoreEQChoiceByDisplay(choices, "bell"); !ok || bell.RequestedNormalized != 0 {
		t.Fatalf("Bell choice = %#v ok=%v", bell, ok)
	}
	if low, ok := proQ3CoreEQChoiceByContains(choices, "low", "cut"); !ok || low.RequestedNormalized != 0.5 {
		t.Fatalf("low-cut candidate = %#v ok=%v", low, ok)
	}
	if _, ok := proQ3CoreEQChoiceByContains(choices, "side"); ok {
		t.Fatal("unexpected unrelated display match")
	}
}

func TestProQ3CoreEQPassFilterScenarioKeepsSlopeAsObservedControl(t *testing.T) {
	shape := proQ3CoreEQChoice{RequestedNormalized: .25, ObservedDisplay: "Low Cut"}
	slope := proQ3CoreEQChoice{RequestedNormalized: 3.0 / 9.0, ObservedDisplay: "24 dB/oct"}
	scenario := proQ3CoreEQPassFilterScenarioTemplate("low_cut_24", "test", shape, slope, .575188457965851, .5, .5)
	if scenario.SelectionBasis == "" || len(scenario.Configuration) != 8 {
		t.Fatalf("pass-filter scenario = %#v", scenario)
	}
	last := scenario.Configuration[len(scenario.Configuration)-1]
	if last.Role != "band_slope" || math.Abs(last.RequestedNormalized-slope.RequestedNormalized) > 0.0000001 {
		t.Fatalf("pass-filter slope configuration = %#v", last)
	}
}

func TestProQ3CoreEQResponseBandwidthUsesObservedResponseOnly(t *testing.T) {
	response := []proQ3CoreEQFrequencyResponse{
		{FrequencyHz: 100, DeltaDB: 1},
		{FrequencyHz: 200, DeltaDB: 8},
		{FrequencyHz: 400, DeltaDB: 10},
		{FrequencyHz: 800, DeltaDB: 8},
		{FrequencyHz: 1600, DeltaDB: 1},
	}
	bandwidth := proQ3CoreEQResponseBandwidth(response)
	if bandwidth == nil || bandwidth.LowerHz != 200 || bandwidth.UpperHz != 800 || bandwidth.Ratio != 4 {
		t.Fatalf("bandwidth = %#v", bandwidth)
	}
}
