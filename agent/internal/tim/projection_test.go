package tim

import "testing"

func TestBuildDoesNotFabricateMissingSourceMetadata(t *testing.T) {
	proj := Build(Input{
		ObservationID: "obs_tim_test",
		MixSessionID:  "mix_tim_test",
		ProjectPackage: map[string]any{
			"track_count": 2,
			"tracks": []any{
				map[string]any{
					"track_id":   "wav_1",
					"track_name": "Wave Track",
					"clip_count": 1,
					"primary_clip": map[string]any{
						"clip_id":             "clip_w",
						"current_source_path": "D:\\Mix\\wave.wav",
						"length_seconds":      10.0,
					},
					"acoustic": map[string]any{"status": "ready", "peak_dbfs": -3.0, "rms_dbfs": -18.0},
				},
				map[string]any{
					"track_id":   "mp3_1",
					"track_name": "MP3 Track",
					"clip_count": 1,
					"primary_clip": map[string]any{
						"clip_id":             "clip_m",
						"current_source_path": "D:\\Mix\\print.mp3",
						"length_seconds":      10.0,
						"sample_rate_hz":      48000,
						"channel_count":       2,
					},
					"acoustic": map[string]any{"status": "ready", "peak_dbfs": -4.0, "rms_dbfs": -20.0},
				},
			},
		},
	})
	if proj.SchemaVersion != SchemaVersion || proj.TIMVersion != Version {
		t.Fatalf("version mismatch: %#v", proj)
	}
	if got := proj.TechnicalSummary.SampleRateCounts["48000_hz"]; got != 1 {
		t.Fatalf("sample rate should only use observed facts, got summary=%#v", proj.TechnicalSummary)
	}
	if len(proj.TechnicalSummary.BitDepthCounts) != 0 {
		t.Fatalf("bit depth should not be fabricated: %#v", proj.TechnicalSummary.BitDepthCounts)
	}
	if proj.Coverage.SampleRate.Status != StatusPartial {
		t.Fatalf("sample rate coverage = %#v", proj.Coverage.SampleRate)
	}
	if proj.Coverage.BitDepth.TotalCount != 1 || proj.Coverage.BitDepth.NotApplicable != 1 || proj.Coverage.BitDepth.Status != StatusMissing {
		t.Fatalf("bit depth coverage should apply to wav and not mp3: %#v", proj.Coverage.BitDepth)
	}
	if !hasLimitation(proj.Limitations, "source_sample_rate_not_fully_exposed_by_project_state") {
		t.Fatalf("missing sample-rate limitation: %#v", proj.Limitations)
	}
	if !hasLimitation(proj.Limitations, "compressed_audio_formats_do_not_provide_pcm_bit_depth") {
		t.Fatalf("missing compressed format limitation: %#v", proj.Limitations)
	}
}

func TestBuildTreatsAbsentCompactSourceMetadataAsUnknown(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "track-unknown", "track_name": "Imported", "clip_count": 1,
			"acoustic": map[string]any{"status": "ready", "rms_dbfs": -18.0},
		}},
	}})
	if proj.TechnicalSummary.SourceMissingCount != 0 || proj.TechnicalSummary.SourceUnknownCount != 1 {
		t.Fatalf("compact metadata gap was treated as missing: %#v", proj.TechnicalSummary)
	}
	if proj.Coverage.SourcePath.Status != StatusPartial || proj.Coverage.SourcePath.UnknownCount != 1 {
		t.Fatalf("source coverage did not expose unknown state: %#v", proj.Coverage.SourcePath)
	}
	if len(proj.Issues) != 0 {
		t.Fatalf("unknown source metadata created a false issue: %#v", proj.Issues)
	}
	if !hasLimitation(proj.Limitations, "source_path_not_exposed_by_compact_project_state") {
		t.Fatalf("unknown source limitation missing: %#v", proj.Limitations)
	}
}

func TestBuildReportsExplicitMissingSourceMetadata(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"track_count": 1,
		"tracks": []any{map[string]any{
			"track_id": "track-missing", "clip_count": 1,
			"primary_clip": map[string]any{"source_status": "missing"},
		}},
	}})
	if proj.TechnicalSummary.SourceMissingCount != 1 || proj.TechnicalSummary.SourceUnknownCount != 0 {
		t.Fatalf("explicit missing source was not preserved: %#v", proj.TechnicalSummary)
	}
	if !hasString(proj.RiskSummary.PrimaryCodes, "source_path_missing") {
		t.Fatalf("explicit missing source did not produce source_path_missing: %#v", proj.RiskSummary)
	}
}

func TestBuildUsesMatchingAuthoritativeSourceState(t *testing.T) {
	project := map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "clip_count": 1}},
	}
	auth := map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "clip_id": "c1", "source_status": "missing"}},
	}
	proj := Build(Input{ProjectPackage: project, AuthoritativeState: auth})
	if proj.TechnicalSummary.SourceMissingCount != 1 || !hasString(proj.RiskSummary.PrimaryCodes, "source_path_missing") {
		t.Fatalf("matching authoritative missing state was not applied: %#v", proj)
	}
}

func TestBuildIgnoresStaleAuthoritativeSourceState(t *testing.T) {
	project := map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "clip_count": 1}},
	}
	proj := Build(Input{ProjectPackage: project, AuthoritativeState: map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "6", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "source_status": "missing"}},
	}})
	if proj.TechnicalSummary.SourceMissingCount != 0 || proj.TechnicalSummary.SourceUnknownCount != 1 {
		t.Fatalf("stale authoritative state was borrowed: %#v", proj.TechnicalSummary)
	}
}

func TestBuildDoesNotBorrowAuthoritativeSourceAcrossClipTopologyMismatch(t *testing.T) {
	project := map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "clip_count": 1, "primary_clip": map[string]any{"clip_id": "old-clip"}}},
	}
	proj := Build(Input{ProjectPackage: project, AuthoritativeState: map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 1,
		"tracks": []any{map[string]any{"track_id": "t1", "clip_id": "new-clip", "source_status": "missing"}},
	}})
	if proj.TechnicalSummary.SourceMissingCount != 0 || proj.TechnicalSummary.SourceUnknownCount != 1 {
		t.Fatalf("clip-mismatched authoritative state was borrowed: %#v", proj.TechnicalSummary)
	}
}

func TestBuildReportsAuthoritativeDADTopologyMismatchAsLimitation(t *testing.T) {
	proj := Build(Input{ProjectPackage: map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 2,
		"tracks": []any{
			map[string]any{"track_id": "t1", "clip_count": 1},
			map[string]any{"track_id": "t2", "clip_count": 1},
		},
	}, AuthoritativeState: map[string]any{
		"project_uuid": "p1", "project_epoch": "e1", "project_revision": "7", "track_count": 2,
		"dad_fact_status": "ready", "dad_fact_ready_count": 1, "dad_fact_total_count": 1,
	}})
	if !hasLimitation(proj.Limitations, "authoritative_dad_topology_count_mismatch") {
		t.Fatalf("DAD topology mismatch limitation missing: %#v", proj.Limitations)
	}
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasLimitation(limitations []string, want string) bool {
	for _, limitation := range limitations {
		if limitation == want {
			return true
		}
	}
	return false
}
