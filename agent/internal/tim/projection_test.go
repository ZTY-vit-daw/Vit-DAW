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

func hasLimitation(limitations []string, want string) bool {
	for _, limitation := range limitations {
		if limitation == want {
			return true
		}
	}
	return false
}
