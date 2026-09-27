package segmentation

import (
	"encoding/json"
	"reflect"
	"testing"
)

// L2-2-SEG-2 red-first table-driven tests: deterministic linear-threshold
// boundary picking over synthetic SEG-1 primitives, neutral S1…Sn identity,
// marker command construction, and the A/B two-run consistency assertion.
//
// Red state: the stub package returns zero results, so every check fails.

// syntheticPrimitives builds a ready payload: hop=0.1s, duration seconds,
// flat baseline signals, density spikes at the given frame indices and
// novelty spikes at the given indices (spike value 1.0; baseline 0.01).
func syntheticPrimitives(durationSeconds float64, densitySpikes, noveltySpikes map[int]float64) Primitives {
	frames := int(durationSeconds / 0.1)
	p := Primitives{
		FeatureType: "segmentation_primitives",
		WindowMs:    4096.0 / 44100.0 * 1000.0,
		HopMs:       100.0,
		FrameCount:  int64(frames),
	}
	p.OnsetEvents.Status = StatusReady
	p.OnsetDensity.Status = StatusReady
	p.OnsetDensity.WindowSeconds = 1.0
	p.OnsetDensity.WindowFrames = int64(10)
	p.EnergyNovelty.Status = StatusReady
	maxNovelty := 0.0
	for i := 0; i < frames; i++ {
		start := float64(i) * 0.1
		end := start + 0.1
		density := 0.01
		if v, ok := densitySpikes[i]; ok {
			density = v
		}
		novelty := 0.01
		if v, ok := noveltySpikes[i]; ok {
			novelty = v
		}
		if novelty > maxNovelty {
			maxNovelty = novelty
		}
		p.OnsetDensity.Frames = append(p.OnsetDensity.Frames, densityFrame{
			StartSeconds: start, EndSeconds: end, OnsetCount: int64(density * 10), OnsetsPerSecond: density,
		})
		p.EnergyNovelty.Frames = append(p.EnergyNovelty.Frames, noveltyFrame{
			StartSeconds: start, EndSeconds: end, Novelty: novelty,
		})
	}
	p.EnergyNovelty.MaxNovelty = maxNovelty
	p.EnergyNovelty.MeanNovelty = 0.01
	return p
}

func TestDetectBoundariesTable(t *testing.T) {
	cases := []struct {
		name       string
		duration   float64
		density    map[int]float64
		novelty    map[int]float64
		wantAt     []float64
		wantConf   []string
	}{
		{
			name:     "two isolated spikes become two boundaries",
			duration: 30.0,
			density:  map[int]float64{100: 1.0},
			novelty:  map[int]float64{220: 1.0},
			wantAt:   []float64{10.0, 22.0},
			wantConf: []string{ConfidenceHigh, ConfidenceHigh},
		},
		{
			name:     "density-only spike above threshold",
			duration: 30.0,
			density:  map[int]float64{150: 0.9},
			wantAt:   []float64{15.0},
			wantConf: []string{ConfidenceHigh},
		},
		{
			name:     "sub-threshold spike produces no boundary",
			duration: 30.0,
			density:  map[int]float64{150: 0.6},
			wantAt:   []float64{},
		},
		{
			name:     "close candidates merge keeping the stronger",
			duration: 40.0,
			density:  map[int]float64{150: 0.7},
			novelty:  map[int]float64{180: 0.95},
			// 15.0s vs 18.0s are within MinSectionSeconds=8, so the weaker
			// density candidate is replaced by the stronger novelty one.
			wantAt:   []float64{18.0},
			wantConf: []string{ConfidenceHigh},
		},
		{
			name:     "boundary near the end is dropped by the min-section rule",
			duration: 30.0,
			density:  map[int]float64{100: 1.0},
			novelty:  map[int]float64{285: 1.0}, // 28.5s leaves only 1.5s tail
			wantAt:   []float64{10.0},
			wantConf: []string{ConfidenceHigh},
		},
		{
			name:     "medium bin confidence",
			duration: 30.0,
			density:  map[int]float64{120: 0.7},
			wantAt:   []float64{12.0},
			wantConf: []string{ConfidenceMedium},
		},
		{
			name:     "low bin confidence just above threshold",
			duration: 30.0,
			novelty:  map[int]float64{120: 0.55},
			wantAt:   []float64{12.0},
			wantConf: []string{ConfidenceLow},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectBoundaries(syntheticPrimitives(tc.duration, tc.density, tc.novelty))
			if got.Status != StatusReady {
				t.Fatalf("status = %s (%v), want ready", got.Status, got.Reasons)
			}
			if len(got.Boundaries) != len(tc.wantAt) {
				t.Fatalf("boundaries = %#v, want positions %v", got.Boundaries, tc.wantAt)
			}
			for i, want := range tc.wantAt {
				if got.Boundaries[i].AtSeconds != want {
					t.Errorf("boundary[%d].AtSeconds = %v, want %v", i, got.Boundaries[i].AtSeconds, want)
				}
			}
			for i, want := range tc.wantConf {
				if got.Boundaries[i].Confidence != want {
					t.Errorf("boundary[%d].Confidence = %q, want %q", i, got.Boundaries[i].Confidence, want)
				}
			}
			if got.DurationSeconds != tc.duration {
				t.Errorf("duration = %v, want %v", got.DurationSeconds, tc.duration)
			}
			// Constants must ride on every ready result (确定性透出).
			for _, key := range []string{"BoundaryDensityWeight", "BoundaryNoveltyWeight", "BoundaryCombinedThreshold", "MinSectionSeconds"} {
				if _, ok := got.Constants[key]; !ok {
					t.Errorf("constants missing %s: %#v", key, got.Constants)
				}
			}
		})
	}
}

func TestSectionsNeutralIDsAndPartition(t *testing.T) {
	got := DetectBoundaries(syntheticPrimitives(30.0, map[int]float64{100: 1.0}, map[int]float64{220: 1.0}))
	want := []Section{
		{SectionID: "S1", StartSeconds: 0.0, EndSeconds: 10.0, Confidence: ""},
		{SectionID: "S2", StartSeconds: 10.0, EndSeconds: 22.0, Confidence: ConfidenceHigh},
		{SectionID: "S3", StartSeconds: 22.0, EndSeconds: 30.0, Confidence: ConfidenceHigh},
	}
	if !reflect.DeepEqual(got.Sections, want) {
		t.Fatalf("sections = %#v, want %#v", got.Sections, want)
	}
}

func TestDetectBoundariesDegenerateInputs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *Primitives)
	}{
		{"density not ready", func(p *Primitives) { p.OnsetDensity.Status = "not_evaluable" }},
		{"novelty not ready", func(p *Primitives) { p.EnergyNovelty.Status = "not_evaluable" }},
		{"empty density frames", func(p *Primitives) { p.OnsetDensity.Frames = nil }},
		{"empty novelty frames", func(p *Primitives) { p.EnergyNovelty.Frames = nil }},
		{"flat zero signals", func(p *Primitives) {
			for i := range p.OnsetDensity.Frames {
				p.OnsetDensity.Frames[i].OnsetsPerSecond = 0
			}
			for i := range p.EnergyNovelty.Frames {
				p.EnergyNovelty.Frames[i].Novelty = 0
			}
			p.EnergyNovelty.MaxNovelty = 0
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := syntheticPrimitives(30.0, map[int]float64{100: 1.0}, nil)
			tc.mutate(&p)
			got := DetectBoundaries(p)
			if got.Status != StatusNotEvaluable {
				t.Fatalf("status = %s, want not_evaluable", got.Status)
			}
			if len(got.Reasons) == 0 {
				t.Errorf("NE conclusion must carry explicit reasons")
			}
			if len(got.Boundaries) != 0 || len(got.Sections) != 0 {
				t.Errorf("degenerate input must produce no boundaries/sections: %#v", got)
			}
		})
	}
}

func TestDetectBoundariesDeterministicReplay(t *testing.T) {
	p := syntheticPrimitives(60.0,
		map[int]float64{80: 1.0, 300: 0.8},
		map[int]float64{180: 0.9, 450: 1.0})
	first := DetectBoundaries(p)
	second := DetectBoundaries(p)
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("two runs on the same input differ:\n%s\n%s", a, b)
	}
	if first.Status != StatusReady || len(first.Boundaries) < 2 {
		t.Fatalf("fixture should produce >=2 boundaries: %#v", first)
	}
}

func TestParsePrimitivesKernelShape(t *testing.T) {
	fixture := `{
		"feature_type": "segmentation_primitives",
		"window_ms": 92.88, "hop_ms": 92.88, "frame_count": 3,
		"onset_events": {"status": "ready", "events": [
			{"onset_seconds": 0.5, "frame_index": 5, "onset_dbfs": -20.1, "rise_db": 3.4}
		]},
		"onset_density": {"status": "ready", "window_seconds": 1.0, "window_frames": 11, "frames": [
			{"start_seconds": 0.0, "end_seconds": 0.09288, "onset_count": 0, "onsets_per_second": 0.0},
			{"start_seconds": 0.09288, "end_seconds": 0.18576, "onset_count": 2, "onsets_per_second": 2.0}
		]},
		"energy_novelty": {"status": "ready", "max_novelty": 0.05, "mean_novelty": 0.001, "frames": [
			{"start_seconds": 0.0, "end_seconds": 0.09288, "novelty": 0.0},
			{"start_seconds": 0.09288, "end_seconds": 0.18576, "novelty": 0.05}
		]}
	}`
	p, err := ParsePrimitives([]byte(fixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.FeatureType != "segmentation_primitives" || p.FrameCount != 3 || p.HopMs != 92.88 {
		t.Fatalf("scalar fields wrong: %#v", p)
	}
	if len(p.OnsetEvents.Events) != 1 || p.OnsetEvents.Events[0].OnsetSeconds != 0.5 || p.OnsetEvents.Events[0].RiseDB != 3.4 {
		t.Fatalf("onset events wrong: %#v", p.OnsetEvents)
	}
	if len(p.OnsetDensity.Frames) != 2 || p.OnsetDensity.Frames[1].OnsetsPerSecond != 2.0 {
		t.Fatalf("density frames wrong: %#v", p.OnsetDensity)
	}
	if len(p.EnergyNovelty.Frames) != 2 || p.EnergyNovelty.Frames[1].Novelty != 0.05 || p.EnergyNovelty.MaxNovelty != 0.05 {
		t.Fatalf("novelty frames wrong: %#v", p.EnergyNovelty)
	}
	if _, err := ParsePrimitives([]byte("{not json")); err == nil {
		t.Fatalf("garbage payload must error")
	}
}

func TestBuildApplySectionMarkersCommand(t *testing.T) {
	// NE result: no command.
	ne := DetectBoundaries(Primitives{})
	if cmd := BuildApplySectionMarkersCommand(ne); cmd != nil {
		t.Fatalf("NE result must not produce a command: %#v", cmd)
	}

	got := DetectBoundaries(syntheticPrimitives(30.0, map[int]float64{100: 1.0}, map[int]float64{220: 1.0}))
	cmd := BuildApplySectionMarkersCommand(got)
	if cmd == nil {
		t.Fatalf("ready result must produce a command")
	}
	if cmd["command"] != "project.markers.apply_section_markers" {
		t.Fatalf("command name = %v", cmd["command"])
	}
	if cmd["source"] != SourceTag {
		t.Fatalf("source = %v, want %s (kernel replace filter)", cmd["source"], SourceTag)
	}
	if replace, _ := cmd["replace_existing"].(bool); !replace {
		t.Fatalf("replace_existing must be true (source-scoped replace)")
	}
	sections, ok := cmd["sections"].([]map[string]any)
	if !ok || len(sections) != 3 {
		t.Fatalf("sections = %#v, want 3 rows", cmd["sections"])
	}
	first := sections[0]
	if first["section_id"] != "S1" || first["start_seconds"] != 0.0 || first["end_seconds"] != 10.0 {
		t.Fatalf("first section wrong: %#v", first)
	}
	if _, has := first["confidence"]; has {
		t.Fatalf("opening section carries no discovery boundary: confidence must be omitted, got %#v", first)
	}
	second := sections[1]
	if second["section_id"] != "S2" || second["confidence"] != ConfidenceHigh {
		t.Fatalf("second section wrong: %#v", second)
	}
	if second["created_by"] != SourceTag {
		t.Fatalf("created_by = %v, want %s", second["created_by"], SourceTag)
	}

	// Command determinism: same input, byte-identical command JSON.
	again := BuildApplySectionMarkersCommand(DetectBoundaries(syntheticPrimitives(30.0, map[int]float64{100: 1.0}, map[int]float64{220: 1.0})))
	a, _ := json.Marshal(cmd)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Fatalf("command JSON differs between runs:\n%s\n%s", a, b)
	}
}
