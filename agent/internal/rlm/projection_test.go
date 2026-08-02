package rlm

import (
	"fmt"
	"math"
	"testing"
)

func TestBuildReferenceLevelModelUsesActiveRMSAsStrictMetric(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "active_rms_dbfs": -24.0, "rms_dbfs": -35.0},
					{"track_id": "track_b", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
					{"track_id": "track_c", "active_rms_dbfs": -20.0, "rms_dbfs": -30.0},
				},
			},
		},
	})

	if proj.Status != StatusReady || proj.Mode != ModeStrict || proj.SelectedMetric != "active_rms_dbfs" {
		t.Fatalf("projection status/mode/metric = %s/%s/%s", proj.Status, proj.Mode, proj.SelectedMetric)
	}
	if proj.ReferenceLevel == nil || *proj.ReferenceLevel != -20 {
		t.Fatalf("reference = %+v", proj.ReferenceLevel)
	}
	if len(proj.Calibration) != 1 || proj.Calibration[0].ClipID != "clip_a" {
		t.Fatalf("calibration rows = %+v", proj.Calibration)
	}
	if proj.Calibration[0].TargetClipGainDB != 4 {
		t.Fatalf("target gain = %+v", proj.Calibration[0])
	}
	if !proj.Actionable {
		t.Fatalf("projection should be actionable: %+v", proj)
	}
}

func TestMergeSourceRowsKeepsTrackAggregateWhenDADUsesAnotherClip(t *testing.T) {
	merged := mergeSourceRows(
		sourceRows([]map[string]any{{
			"track_id": "track_a",
			"clips":    []map[string]any{{"clip_id": "clip_primary", "clip_gain_db": 2.0, "duration_seconds": 10.0}, {"clip_id": "clip_other", "clip_gain_db": -3.0, "duration_seconds": 4.0}},
		}}, "project.state:tracks"),
		sourceRows([]map[string]any{{
			"track_id":                  "track_a",
			"primary_clip":              map[string]any{"clip_id": "clip_primary"},
			"rms_dbfs":                  -20.0,
			"effective_static_rms_dbfs": -18.0,
		}}, "mix.observe:project_package.tracks"),
		sourceRows([]map[string]any{{
			"track_id":  "track_a",
			"clip_id":   "clip_other",
			"rms_dbfs":  -40.0,
			"peak_dbfs": -10.0,
		}}, "project.audio_analysis_status:track_waveform_envelopes"),
	)
	if len(merged) != 1 {
		t.Fatalf("merged rows = %+v", merged)
	}
	row := referenceRowFromMap(merged[0].Data, merged[0].EvidenceRefs)
	if row.PrimaryClipID != "clip_primary" {
		t.Fatalf("primary clip identity = %q, want clip_primary; data=%+v", row.PrimaryClipID, merged[0].Data)
	}
	if row.RMSDBFS == nil || math.Abs(*row.RMSDBFS-(-20.0)) > 0.001 {
		t.Fatalf("different DAD clip overwrote track aggregate: row=%+v data=%+v", row, merged[0].Data)
	}
}

func TestBuildReferenceLevelModelUsesFullRMSAsCoarseWhenActiveMissing(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"track_waveform_envelopes": []map[string]any{
				{"track_id": "track_a", "rms_dbfs": -24.5, "peak_dbfs": -10.0},
				{"track_id": "track_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
				{"track_id": "track_c", "rms_dbfs": -20.5, "peak_dbfs": -8.5},
			},
		},
	})

	if proj.Status != StatusReady || proj.Mode != ModeCoarse || proj.SelectedMetric != "rms_dbfs" {
		t.Fatalf("projection status/mode/metric = %s/%s/%s", proj.Status, proj.Mode, proj.SelectedMetric)
	}
	if len(proj.Calibration) != 2 || proj.Calibration[0].TargetClipGainDB != 4 || proj.Calibration[1].TargetClipGainDB != -0.5 {
		t.Fatalf("coarse calibration rows = %+v", proj.Calibration)
	}
	if len(proj.Limitations) == 0 || proj.Limitations[0] != "rlm_strict_active_or_lufs_missing_coarse_reference" {
		t.Fatalf("limitations = %+v", proj.Limitations)
	}
}

func TestBuildReferenceLevelModelBlocksPartialCoverage(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_c", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"track_waveform_envelopes": []map[string]any{
				{"track_id": "track_a", "rms_dbfs": -24.0},
				{"track_id": "track_b", "rms_dbfs": -20.0},
			},
		},
	})

	if proj.Status != StatusPartial || proj.Actionable {
		t.Fatalf("partial projection should not be actionable: status=%s actionable=%v rows=%+v", proj.Status, proj.Actionable, proj.Calibration)
	}
	if len(proj.Calibration) != 0 {
		t.Fatalf("partial coverage should not create calibration rows: %+v", proj.Calibration)
	}
	if proj.Summary.MissingMetricCount != 1 {
		t.Fatalf("missing count = %+v", proj.Summary)
	}
}

func TestBuildReferenceLevelModelExcludesExplicitSilentWaveformFromCoverage(t *testing.T) {
	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				testTrack("track_a", "clip_a", 0),
				testTrack("track_b", "clip_b", 0),
				testTrack("track_silent", "clip_silent", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "rms_dbfs": -24.0, "peak_dbfs": -10.0},
					{"track_id": "track_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
					{"track_id": "track_silent", "clip_id": "clip_silent", "rms": 0.0, "peak_abs": 0.0, "status": "ready"},
				},
			},
		},
	})

	if proj.Status != StatusReady || proj.Summary.EligibleTrackCount != 2 || proj.Summary.CandidateCount != 2 {
		t.Fatalf("silent source should not block strict coverage: status=%s summary=%+v limitations=%+v", proj.Status, proj.Summary, proj.Limitations)
	}
	foundSilent := false
	for _, row := range proj.Rows {
		if row.TrackID == "track_silent" {
			foundSilent = row.SilentSource && !row.Eligible && row.EligibilityReason == "silent_source_no_level_evidence"
		}
	}
	if !foundSilent {
		t.Fatalf("silent row was not marked/excluded: %+v", proj.Rows)
	}
}

func TestBuildReferenceLevelModelUsesLinearDADRowsAndExcludesOneSilentSource(t *testing.T) {
	tracks := make([]map[string]any, 0, 61)
	waveforms := make([]map[string]any, 0, 61)
	for i := 0; i < 60; i++ {
		trackID := fmt.Sprintf("track_%02d", i)
		clipID := fmt.Sprintf("clip_%02d", i)
		tracks = append(tracks, testTrack(trackID, clipID, 0))
		levelDB := -21.0
		if i%2 == 0 {
			levelDB = -20.0
		}
		waveforms = append(waveforms, map[string]any{
			"track_id": trackID,
			"clip_id":  clipID,
			"rms":      ampFromDB(levelDB),
			"peak_abs": ampFromDB(-8.0),
			"status":   "ready",
		})
	}
	tracks = append(tracks, testTrack("track_silent", "clip_silent", 0))
	waveforms = append(waveforms, map[string]any{
		"track_id": "track_silent",
		"clip_id":  "clip_silent",
		"rms":      0.0,
		"peak_abs": 0.0,
		"status":   "ready",
	})

	proj := Build(Input{
		GeneratedAt: "2026-07-10T00:00:00Z",
		ProjectState: map[string]any{
			"tracks": tracks,
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": waveforms,
			},
		},
	})

	if proj.Status != StatusReady || proj.SelectedMetric != "rms_dbfs" || proj.Summary.CandidateCount != 60 || proj.Summary.EligibleTrackCount != 60 {
		t.Fatalf("realistic DAD rows should produce full non-silent coverage: status=%s metric=%s summary=%+v limitations=%+v", proj.Status, proj.SelectedMetric, proj.Summary, proj.Limitations)
	}
	if len(proj.Calibration) != 60 {
		t.Fatalf("calibration rows = %d, want 60", len(proj.Calibration))
	}
}

func TestBuildReferenceLevelModelCreatesManyRowsForBatchUse(t *testing.T) {
	tracks := make([]map[string]any, 0, 64)
	waveforms := make([]map[string]any, 0, 64)
	for i := 0; i < 64; i++ {
		trackID := fmt.Sprintf("track_%02d", i)
		clipID := fmt.Sprintf("clip_%02d", i)
		tracks = append(tracks, testTrack(trackID, clipID, 0))
		level := -20.0
		if i%2 == 0 {
			level = -21.0
		}
		waveforms = append(waveforms, map[string]any{"track_id": trackID, "active_rms_dbfs": level})
	}
	proj := Build(Input{
		GeneratedAt:         "2026-07-10T00:00:00Z",
		ProjectState:        map[string]any{"tracks": tracks},
		AudioAnalysisStatus: map[string]any{"track_waveform_envelopes": waveforms},
	})

	if proj.Status != StatusReady || proj.Summary.CandidateCount != 64 {
		t.Fatalf("projection summary = %+v status=%s", proj.Summary, proj.Status)
	}
	if len(proj.Calibration) != 64 {
		t.Fatalf("strict reference should keep all actionable rows, got %d: %+v", len(proj.Calibration), proj.Calibration)
	}
}

func testTrack(trackID string, clipID string, gainDB float64) map[string]any {
	return map[string]any{
		"track_id":       trackID,
		"track_name":     trackID,
		"is_audio_track": true,
		"track_type":     "audio",
		"volume_db":      0.0,
		"clips": []map[string]any{{
			"clip_id":          clipID,
			"clip_name":        clipID + ".wav",
			"type":             "audio",
			"clip_gain_db":     gainDB,
			"duration_seconds": 12.0,
		}},
	}
}

func ampFromDB(db float64) float64 {
	return math.Pow(10, db/20)
}
