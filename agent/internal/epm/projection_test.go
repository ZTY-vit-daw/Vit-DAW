package epm

import "testing"

func TestBuildDetectsFullLengthStemsAndProtectsAlignment(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Summary: map[string]any{"tracks_created": 3, "edit_length_seconds": 180.0},
		Rows: []map[string]any{
			stemRow("track_drums", "Drums", "clip_drums", 0, 180, 2),
			stemRow("track_bass", "Bass", "clip_bass", 0, 179.8, 2),
			stemRow("track_vocal", "Lead Vocal", "clip_vocal", 0, 181, 2),
		},
		DADWaveformRows: []map[string]any{
			dadRow("track_drums", "clip_drums", -18, -3),
			dadRow("track_bass", "clip_bass", -20, -4),
			dadRow("track_vocal", "clip_vocal", -16, -2),
		},
	})

	if proj.SchemaVersion != SchemaVersion || proj.EPMVersion != Version {
		t.Fatalf("version mismatch: %+v", proj)
	}
	if !proj.ClipCleanup.FullLengthStemDetected || !proj.ClipCleanup.PreserveStemAlignment {
		t.Fatalf("expected full-length stem protection: %+v", proj.ClipCleanup)
	}
	if proj.ClipCleanup.TrimRecommendation != "not_recommended_for_stems" {
		t.Fatalf("trim recommendation = %q", proj.ClipCleanup.TrimRecommendation)
	}
	if proj.ClipCleanup.FadeRecommendation != "not_recommended_by_default" {
		t.Fatalf("fade recommendation = %q", proj.ClipCleanup.FadeRecommendation)
	}
	if proj.SectionMap.Status != "suggested_low_confidence" || proj.SectionMap.ReferenceStrategy != "few_track_proxy_mix" {
		t.Fatalf("section map = %+v", proj.SectionMap)
	}
	if len(proj.SectionMap.Sections) == 0 || proj.SectionMap.MarkerWriteSupport != "ready" {
		t.Fatalf("section recommendations not generated safely: %+v", proj.SectionMap)
	}
	ctx := ContextProjectionMap(proj)
	if ctx["schema_version"] != SchemaVersion {
		t.Fatalf("context schema = %+v", ctx)
	}
}

func TestBuildLimitedWhenClipTimingMissing(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Rows: []map[string]any{
			{"track_id": "track_1", "track_name": "Audio 1", "clip_id": "clip_1"},
			{"track_id": "track_2", "track_name": "Audio 2", "clip_id": "clip_2"},
		},
	})

	if proj.ClipCleanup.Status != statusLimited {
		t.Fatalf("cleanup status = %q, want limited", proj.ClipCleanup.Status)
	}
	if proj.ClipCleanup.TrimRecommendation != "not_enough_data" {
		t.Fatalf("trim recommendation = %q", proj.ClipCleanup.TrimRecommendation)
	}
	if len(proj.Limitations) == 0 {
		t.Fatalf("expected limitations for missing timing/DAD facts")
	}
}

func TestBuildRaisesFadeCandidateOnlyForAcousticRisk(t *testing.T) {
	proj := BuildFromImportRows(ImportInput{
		Rows: []map[string]any{
			stemRow("track_hot", "Hot Print", "clip_hot", 0, 60, 2),
			stemRow("track_clean", "Clean Print", "clip_clean", 0, 60, 2),
		},
		DADWaveformRows: []map[string]any{
			dadRow("track_hot", "clip_hot", -12, -0.05),
			dadRow("track_clean", "clip_clean", -18, -5),
		},
	})

	if proj.ClipCleanup.CandidateSummary.FadeCandidateCount != 1 {
		t.Fatalf("fade candidates = %+v", proj.ClipCleanup.CandidateSummary)
	}
	if proj.ClipCleanup.FadeRecommendation != "review_boundary_risk_only" {
		t.Fatalf("fade recommendation = %q", proj.ClipCleanup.FadeRecommendation)
	}
	if len(proj.ClipCleanup.Candidates) != 1 || proj.ClipCleanup.Candidates[0].TrackID != "track_hot" {
		t.Fatalf("candidates = %+v", proj.ClipCleanup.Candidates)
	}
	action := proj.ClipCleanup.Candidates[0].PendingAction
	if action == nil || action.ToolName != "clip.fade.set" {
		t.Fatalf("pending fade action missing: %+v", proj.ClipCleanup.Candidates[0])
	}
	if action.Args["clip_id"] != "clip_hot" || action.Args["fade_in_seconds"] == nil || action.Args["fade_out_seconds"] == nil {
		t.Fatalf("pending fade args = %+v", action.Args)
	}
	if action.Risk != "confirm" || len(action.EvidenceRefs) == 0 {
		t.Fatalf("pending fade action risk/evidence = %+v", action)
	}
}

func TestBuildMultitrackSectionMapFramework(t *testing.T) {
	rows := []map[string]any{}
	for i := 0; i < 10; i++ {
		rows = append(rows, stemRow("track", "Stem", "clip", 0, 120, 2))
	}
	proj := BuildFromImportRows(ImportInput{Rows: rows})

	if proj.SectionMap.ReferenceStrategy != "multitrack_group_activity" {
		t.Fatalf("strategy = %q", proj.SectionMap.ReferenceStrategy)
	}
	if proj.SectionMap.MarkerWriteSupport != "ready" {
		t.Fatalf("marker support = %q", proj.SectionMap.MarkerWriteSupport)
	}
}

func TestBuildSectionMapFromDADTimeSegments(t *testing.T) {
	rows := []map[string]any{}
	dadRows := []map[string]any{}
	for i := 0; i < 6; i++ {
		trackID := "track_" + string(rune('a'+i))
		clipID := "clip_" + string(rune('a'+i))
		rows = append(rows, stemRow(trackID, "Stem", clipID, 0, 96, 2))
		dadRows = append(dadRows, map[string]any{
			"status":   "ready",
			"track_id": trackID,
			"clip_id":  clipID,
			"time_segments": []any{
				map[string]any{"start_seconds": 0, "end_seconds": 24, "rms": 0.03, "energy_state": "low"},
				map[string]any{"start_seconds": 24, "end_seconds": 48, "rms": 0.16, "energy_state": "high"},
				map[string]any{"start_seconds": 48, "end_seconds": 72, "rms": 0.05, "energy_state": "medium"},
				map[string]any{"start_seconds": 72, "end_seconds": 96, "rms": 0.18, "energy_state": "high"},
			},
		})
	}
	proj := BuildFromImportRows(ImportInput{Rows: rows, DADWaveformRows: dadRows})

	if proj.SectionMap.Status != "recommended" {
		t.Fatalf("section status = %q map=%+v", proj.SectionMap.Status, proj.SectionMap)
	}
	if proj.SectionMap.Confidence == "low" || proj.SectionMap.CandidateCount < 2 {
		t.Fatalf("expected DAD-backed section candidates: %+v", proj.SectionMap)
	}
	if proj.SectionMap.EvidenceSummary.BoundarySource != "dad_time_segment_activity" {
		t.Fatalf("boundary source = %+v", proj.SectionMap.EvidenceSummary)
	}
	if proj.SectionMap.Sections[0].StartSeconds != 0 || proj.SectionMap.Sections[len(proj.SectionMap.Sections)-1].EndSeconds != 96 {
		t.Fatalf("sections do not cover project: %+v", proj.SectionMap.Sections)
	}
}

func stemRow(trackID, trackName, clipID string, start, duration float64, channels int) map[string]any {
	return map[string]any{
		"track_id":         trackID,
		"track_name":       trackName,
		"clip_id":          clipID,
		"clip_name":        clipID + ".wav",
		"source_file_path": "E:/stems/" + clipID + ".wav",
		"start_seconds":    start,
		"duration_seconds": duration,
		"channel_count":    channels,
	}
}

func dadRow(trackID, clipID string, rms, peak float64) map[string]any {
	return map[string]any{
		"status":    "ready",
		"track_id":  trackID,
		"clip_id":   clipID,
		"rms_dbfs":  rms,
		"peak_dbfs": peak,
	}
}
