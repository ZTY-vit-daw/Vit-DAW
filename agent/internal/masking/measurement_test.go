package masking

import "testing"

func TestBuildProducesDirectionalCandidateWithoutClaimingDefect(t *testing.T) {
	bands := []BandDefinition{{ID: "bass", MinHz: 60, MaxHz: 250}, {ID: "mid", MinHz: 500, MaxHz: 2000}}
	frames := func(bass, mid float64) []Frame {
		return []Frame{
			{StartSeconds: 0, EndSeconds: .1, LevelsDBFS: map[string]float64{"bass": bass, "mid": mid}},
			{StartSeconds: .1, EndSeconds: .2, LevelsDBFS: map[string]float64{"bass": bass + 1, "mid": mid}},
			{StartSeconds: .2, EndSeconds: .3, LevelsDBFS: map[string]float64{"bass": bass, "mid": mid - 1}},
		}
	}
	measurement := Build(BuildInput{ProjectUUID: "project-1", ProjectRevision: "42", ProjectStateHash: "hash-42", Tracks: []TrackEvidence{
		{TrackID: "drums", TrackName: "Drums", TapPoint: "track_post_fader", RenderRevision: "render-drums", AnalyzerRevision: "l2.v2", EvidenceRef: "dad.l2:drums", SampleRate: 48000, RangeStart: 0, RangeEnd: .3, Bands: bands, Frames: frames(-18, -42)},
		{TrackID: "bass", TrackName: "Bass", TapPoint: "track_post_fader", RenderRevision: "render-bass", AnalyzerRevision: "l2.v2", EvidenceRef: "dad.l2:bass", SampleRate: 48000, RangeStart: 0, RangeEnd: .3, Bands: bands, Frames: frames(-34, -35)},
	}})
	if measurement.Status != "ready" || measurement.SchemaVersion != MeasurementSchema || measurement.MeasurementID == "" {
		t.Fatalf("measurement header = %+v", measurement)
	}
	var forward, reverse *PairBand
	for index := range measurement.PairBands {
		row := &measurement.PairBands[index]
		if row.MaskerTrackID == "drums" && row.TargetTrackID == "bass" && row.BandID == "bass" {
			forward = row
		}
		if row.MaskerTrackID == "bass" && row.TargetTrackID == "drums" && row.BandID == "bass" {
			reverse = row
		}
	}
	if forward == nil || forward.Status != "candidate" || forward.CoverageRatio <= 0 || forward.P90MarginDB <= 0 {
		t.Fatalf("forward masking candidate = %+v", forward)
	}
	if reverse == nil || reverse.Status == "candidate" {
		t.Fatalf("reverse relationship was not directional: %+v", reverse)
	}
	if measurement.Model["candidate_only"] != true {
		t.Fatalf("model boundary missing: %+v", measurement.Model)
	}
}

func TestBuildRejectsUnsynchronizedEvidence(t *testing.T) {
	band := []BandDefinition{{ID: "mid", MinHz: 500, MaxHz: 2000}}
	frame := []Frame{{StartSeconds: 0, EndSeconds: .1, LevelsDBFS: map[string]float64{"mid": -20}}}
	measurement := Build(BuildInput{Tracks: []TrackEvidence{
		{TrackID: "a", TapPoint: "track_post_fader", RenderRevision: "a", AnalyzerRevision: "v1", SampleRate: 48000, RangeStart: 0, RangeEnd: 1, Bands: band, Frames: frame},
		{TrackID: "b", TapPoint: "track_post_fader", RenderRevision: "b", AnalyzerRevision: "v1", SampleRate: 48000, RangeStart: 1, RangeEnd: 2, Bands: band, Frames: frame},
	}})
	if measurement.Status != "suspect" || len(measurement.PairBands) != 0 {
		t.Fatalf("unsynchronized evidence was promoted: %+v", measurement)
	}
}

func TestTrackEvidenceFromProbeRowReadsBoundedFrames(t *testing.T) {
	row := map[string]any{
		"status": "ready", "track_id": "1012", "tap_point": "track_post_fader", "render_revision": "render-1", "evidence_ref": "dad.l2:render-1",
		"analyzed_range": map[string]any{"start_seconds": 10.0, "end_seconds": 10.1},
		"masking_frames": map[string]any{
			"schema_version": FrameSchema, "status": "ready", "analyzer_revision": "probe.masking.v1", "sample_rate": 48000.0,
			"bands":  []any{map[string]any{"id": "mid", "min_hz": 500.0, "max_hz": 2000.0}},
			"frames": []any{map[string]any{"start_seconds": 10.0, "end_seconds": 10.1, "levels_dbfs": map[string]any{"mid": -24.0}}},
		},
	}
	row["tail_seconds"] = 0.25
	evidence, ok := TrackEvidenceFromProbeRow(row, "Drums")
	if !ok || evidence.TrackID != "1012" || evidence.TrackName != "Drums" || evidence.TailSeconds != 0.25 || len(evidence.Frames) != 1 || evidence.Frames[0].LevelsDBFS["mid"] != -24 {
		t.Fatalf("probe evidence = %+v ok=%v", evidence, ok)
	}
}
