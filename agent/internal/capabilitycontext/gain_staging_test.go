package capabilitycontext

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildGainStagingPackRanksCapsAndExcludesRawFields(t *testing.T) {
	projectState := map[string]any{
		"tracks": []map[string]any{
			{
				"track_id":         "vocal_1",
				"track_name":       "Lead Vocal",
				"user_track_index": 1,
				"volume_db":        -7.5,
				"clips": []map[string]any{{
					"clip_id":          "clip_vocal",
					"clip_name":        strings.Repeat("Vocal ", 40),
					"type":             "audio",
					"clip_gain_db":     7.2,
					"duration_seconds": 12.0,
					"source_path":      `D:\Mix\Lead Vocal.wav`,
				}},
			},
			{
				"track_id":   "bass_1",
				"track_name": "Bass",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_bass",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			},
			{
				"track_id":   "pad_1",
				"track_name": "Pad",
				"clips": []map[string]any{{
					"clip_id":          "clip_pad",
					"type":             "audio",
					"duration_seconds": 12.0,
				}},
			},
		},
	}
	mixResult := map[string]any{
		"observation_id": "obs_1",
		"observation": map[string]any{
			"observation_id": "obs_1",
			"target_ref":     map[string]any{"kind": "project", "id": "current"},
			"project_package": map[string]any{
				"tracks": []map[string]any{
					{
						"track_id":    "vocal_1",
						"role_guess":  "vocal",
						"peak_dbfs":   -0.05,
						"rms_dbfs":    -14.0,
						"headroom_db": 0.05,
						"time_segments": []map[string]any{{
							"start_seconds": 0.0,
							"rms":           0.2,
						}},
					},
					{
						"track_id":    "bass_1",
						"role_guess":  "bass",
						"peak_dbfs":   -4.0,
						"rms_dbfs":    -22.0,
						"headroom_db": 4.0,
					},
					{
						"track_id":    "pad_1",
						"role_guess":  "pad",
						"peak_dbfs":   -55.0,
						"rms_dbfs":    -48.0,
						"headroom_db": 55.0,
					},
				},
				"limitations": []any{"kernel_l3_loudness_summary_missing"},
			},
		},
	}

	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent:     "检查 B1 gain staging",
		ProjectState:   projectState,
		MixObservation: mixResult,
		GeneratedAt:    time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC),
		Budget: Budget{
			MaxTracks:        2,
			MaxRankingRows:   2,
			MaxClipsPerTrack: 2,
			MaxStringRunes:   32,
		},
	})

	if pack.SchemaVersion != SchemaVersion || pack.CapabilityID != GainStagingCapabilityID {
		t.Fatalf("unexpected pack identity: %#v", pack)
	}
	if pack.ContextManifestID != "static_mix.gain_staging.context_manifest.v0" || pack.ContextBuilder != CapabilityContextBuilderVersion {
		t.Fatalf("pack is not context-manifest driven: manifest=%q builder=%q", pack.ContextManifestID, pack.ContextBuilder)
	}
	if got := pack.Summary["track_count"]; got != float64(3) && got != 3 {
		t.Fatalf("track_count = %#v", got)
	}
	if len(pack.Tracks) != 2 {
		t.Fatalf("tracks were not capped: %d", len(pack.Tracks))
	}
	if pack.Tracks[0].TrackID != "vocal_1" {
		t.Fatalf("highest risk track should be first, got %#v", pack.Tracks[0])
	}
	if len(pack.Rankings["headroom_risk"]) == 0 || pack.Rankings["headroom_risk"][0].TrackID != "vocal_1" {
		t.Fatalf("headroom ranking missing vocal risk: %#v", pack.Rankings["headroom_risk"])
	}
	if len(pack.Rankings["clip_gain_outliers"]) == 0 || pack.Rankings["clip_gain_outliers"][0].ClipID != "clip_vocal" {
		t.Fatalf("clip gain outlier missing: %#v", pack.Rankings["clip_gain_outliers"])
	}
	if pack.EvidenceStatus["track_acoustic"].Status != "ready" {
		t.Fatalf("track acoustic status = %#v", pack.EvidenceStatus["track_acoustic"])
	}
	data, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{`"time_segments":`, `"track_waveform_envelopes":`, `D:\Mix`, `"source_path":`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("pack leaked forbidden %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "Vocal Vocal Vocal Vocal Vocal Vo...") {
		t.Fatalf("long clip name was not truncated:\n%s", text)
	}
}

func TestBuildGainStagingPackMarksMissingEvidence(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent:   "gain staging",
		ProjectState: map[string]any{"tracks": []map[string]any{{"track_id": "track_1", "track_name": "Track 1"}}},
		GeneratedAt:  time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC),
	})

	if pack.EvidenceStatus["mix_observation"].Status != "missing" {
		t.Fatalf("mix observation should be missing: %#v", pack.EvidenceStatus["mix_observation"])
	}
	if pack.EvidenceStatus["track_acoustic"].Status != "missing" {
		t.Fatalf("track acoustic should be missing: %#v", pack.EvidenceStatus["track_acoustic"])
	}
	if len(pack.Limitations) == 0 {
		t.Fatalf("missing evidence should produce limitations")
	}
}

func TestBuildGainStagingSuggestionsNoActionForHealthyPack(t *testing.T) {
	suggestions := BuildGainStagingSuggestions(Pack{
		CapabilityID: GainStagingCapabilityID,
		PackID:       "pack_healthy",
		Rankings:     map[string][]RankRow{},
	})

	if suggestions.Status != "no_action" || suggestions.PrimaryAction != nil || len(suggestions.Actions) != 0 {
		t.Fatalf("healthy suggestions = %+v", suggestions)
	}
}

func TestBuildGainStagingSuggestionsClipGainOutlier(t *testing.T) {
	suggestions := BuildGainStagingSuggestions(Pack{
		CapabilityID: GainStagingCapabilityID,
		PackID:       "pack_clip",
		Rankings: map[string][]RankRow{
			"clip_gain_outliers": {{
				TrackID:      "track_vocal",
				TrackName:    "Lead Vocal",
				ClipID:       "clip_vocal",
				ClipName:     "Lead Vocal.wav",
				Kind:         "clip_gain_db",
				Value:        7.25,
				Unit:         "dB",
				Risk:         "medium",
				EvidenceRefs: []string{"project.state:tracks"},
			}},
		},
	})

	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("clip suggestions = %+v", suggestions)
	}
	action := suggestions.PrimaryAction
	if action.ActionKind != "clip_gain_set" || action.Tool != "clip.gain.set" || action.ClipID != "clip_vocal" {
		t.Fatalf("clip action = %+v", action)
	}
	if action.TargetDB == nil || *action.TargetDB != 0 {
		t.Fatalf("target db = %+v", action.TargetDB)
	}
	if action.DeltaDB == nil || *action.DeltaDB != -7.25 {
		t.Fatalf("delta db = %+v", action.DeltaDB)
	}
	if !action.RequiresConfirmation {
		t.Fatalf("clip gain action must require confirmation")
	}
}

func TestBuildGainStagingSuggestionsSourceLevelOutlierUsesClipGainCalibration(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{{
				"track_id":   "track_quiet",
				"track_name": "Quiet",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":   "track_ref_a",
				"track_name": "Reference A",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_a",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":   "track_ref_b",
				"track_name": "Reference B",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_b",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_b12",
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "rms_dbfs": -30.0, "peak_dbfs": -12.0},
						{"track_id": "track_ref_a", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
						{"track_id": "track_ref_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 1 || rows[0].ClipID != "clip_quiet" {
		t.Fatalf("source level rankings = %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("source calibration suggestions = %+v", suggestions)
	}
	action := suggestions.PrimaryAction
	if action.ActionKind != "source_clip_gain_calibration" || action.Tool != "clip.gain.set" {
		t.Fatalf("source calibration action = %+v", action)
	}
	if action.TargetDB == nil || *action.TargetDB != 10 {
		t.Fatalf("target db = %+v", action.TargetDB)
	}
	if action.DeltaDB == nil || *action.DeltaDB != 10 {
		t.Fatalf("delta db = %+v", action.DeltaDB)
	}
	if metric := action.Metadata["reference_metric"]; metric != "rms_dbfs" {
		t.Fatalf("reference metric = %#v metadata=%+v", metric, action.Metadata)
	}
	if sameMetric := action.Metadata["same_metric_comparison"]; sameMetric != true {
		t.Fatalf("same metric comparison not marked: %+v", action.Metadata)
	}
	if !action.RequiresConfirmation {
		t.Fatalf("source calibration action must require confirmation")
	}
}

func TestBuildGainStagingPackB12AdmissionUsesActiveRMSWhenFullyComparable(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet", "Quiet", "clip_quiet", 0),
				b1TestTrackWithClip("track_ref_a", "Reference A", "clip_ref_a", 0),
				b1TestTrackWithClip("track_ref_b", "Reference B", "clip_ref_b", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_active_rms",
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "active_rms_dbfs": -31.0, "peak_dbfs": -10.0},
						{"track_id": "track_ref_a", "active_rms_dbfs": -21.0, "peak_dbfs": -8.0},
						{"track_id": "track_ref_b", "active_rms_dbfs": -21.0, "peak_dbfs": -8.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	admission := b1AdmissionSummary(t, pack)
	if admission["status"] != "ready" || admission["selected_metric"] != "active_rms_dbfs" {
		t.Fatalf("admission = %+v", admission)
	}
	if admission["candidate_count"] != 3 || admission["eligible_track_count"] != 3 {
		t.Fatalf("admission counts = %+v", admission)
	}
	status := pack.EvidenceStatus["b1_2_source_level_admission"]
	if status.Status != "ready" || status.KnownCount != 3 || status.TotalCount != 3 {
		t.Fatalf("source level admission evidence = %+v", status)
	}
	rows := pack.Rankings["source_level_outliers"]
	if len(rows) != 1 || rows[0].TrackID != "track_quiet" || rows[0].Kind != "active_rms_dbfs_delta_to_reference" {
		t.Fatalf("source level rankings = %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("suggestions = %+v", suggestions)
	}
	if metric := suggestions.PrimaryAction.Metadata["reference_metric"]; metric != "active_rms_dbfs" {
		t.Fatalf("reference metric = %#v metadata=%+v", metric, suggestions.PrimaryAction.Metadata)
	}
}

func TestBuildGainStagingPackB12AdmissionBlocksSelectedClipOnlyCoverage(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet", "Quiet", "clip_quiet", 0),
				b1TestTrackWithClip("track_ref_a", "Reference A", "clip_ref_a", 0),
				b1TestTrackWithClip("track_ref_b", "Reference B", "clip_ref_b", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_selected_only",
				"target_ref":     map[string]any{"kind": "selected_clip", "id": "clip_quiet"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "active_rms_dbfs": -31.0, "peak_dbfs": -10.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	admission := b1AdmissionSummary(t, pack)
	if admission["status"] != "partial" || admission["selected_metric"] != "active_rms_dbfs" {
		t.Fatalf("admission should block partial selected-only coverage, got %+v", admission)
	}
	if admission["candidate_count"] != 1 || admission["eligible_track_count"] != 3 || admission["missing_candidate_count"] != 2 {
		t.Fatalf("admission counts = %+v", admission)
	}
	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 0 {
		t.Fatalf("source level rankings should be blocked by missing full-project coverage: %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "no_action" || suggestions.PrimaryAction != nil {
		t.Fatalf("suggestions should not calibrate selected-only observation: %+v", suggestions)
	}
}

func TestBuildGainStagingPackB12AdmissionUsesAudioAnalysisStatusForFullProjectMetrics(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet", "Quiet", "clip_quiet", 0),
				b1TestTrackWithClip("track_ref_a", "Reference A", "clip_ref_a", 0),
				b1TestTrackWithClip("track_ref_b", "Reference B", "clip_ref_b", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_selected_only",
				"target_ref":     map[string]any{"kind": "selected_clip", "id": "clip_quiet"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "rms_dbfs": -31.0, "peak_dbfs": -10.0},
					},
				},
			},
		},
		AudioAnalysisStatus: map[string]any{
			"status": "ok",
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_quiet", "clip_id": "clip_quiet", "rms_dbfs": -30.0, "peak_dbfs": -12.0, "headroom_db": 12.0},
					{"track_id": "track_ref_a", "clip_id": "clip_ref_a", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
					{"track_id": "track_ref_b", "clip_id": "clip_ref_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	admission := b1AdmissionSummary(t, pack)
	if admission["status"] != "ready" || admission["selected_metric"] != "rms_dbfs" {
		t.Fatalf("audio-analysis-backed admission = %+v", admission)
	}
	if admission["candidate_count"] != 3 || admission["eligible_track_count"] != 3 || admission["missing_candidate_count"] != 0 {
		t.Fatalf("audio-analysis-backed admission counts = %+v", admission)
	}
	status := pack.EvidenceStatus["b1_2_source_level_admission"]
	if status.Status != "ready" || status.KnownCount != 3 || status.TotalCount != 3 {
		t.Fatalf("source level admission evidence = %+v", status)
	}
	if !strings.Contains(strings.Join(pack.EvidenceRefs, ","), "project.audio_analysis_status:track_waveform_envelopes") {
		t.Fatalf("pack evidence should cite project.audio_analysis_status: %+v", pack.EvidenceRefs)
	}
	rows := pack.Rankings["source_level_outliers"]
	if len(rows) != 1 || rows[0].TrackID != "track_quiet" {
		t.Fatalf("source level rankings = %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil || suggestions.PrimaryAction.Tool != "clip.gain.set" {
		t.Fatalf("suggestions = %+v", suggestions)
	}
	if suggestions.PrimaryAction.TargetDB == nil || *suggestions.PrimaryAction.TargetDB != 10 {
		t.Fatalf("target clip gain = %+v", suggestions.PrimaryAction)
	}
}

func TestBuildGainStagingSuggestionsSourceLevelOutliersUseBatchWhenMultipleTargets(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet_a", "Quiet A", "clip_quiet_a", 0),
				b1TestTrackWithClip("track_quiet_b", "Quiet B", "clip_quiet_b", 0),
				b1TestTrackWithClip("track_ref_a", "Reference A", "clip_ref_a", 0),
				b1TestTrackWithClip("track_ref_b", "Reference B", "clip_ref_b", 0),
				b1TestTrackWithClip("track_ref_c", "Reference C", "clip_ref_c", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_batch",
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet_a", "active_rms_dbfs": -34.0},
						{"track_id": "track_quiet_b", "active_rms_dbfs": -33.0},
						{"track_id": "track_ref_a", "active_rms_dbfs": -20.0},
						{"track_id": "track_ref_b", "active_rms_dbfs": -20.0},
						{"track_id": "track_ref_c", "active_rms_dbfs": -20.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 2 {
		t.Fatalf("source level rows = %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("suggestions = %+v", suggestions)
	}
	if suggestions.PrimaryAction.ActionKind != "source_clip_gain_calibration_batch" || suggestions.PrimaryAction.Tool != "clip.gain.set_batch" {
		t.Fatalf("primary action = %+v", suggestions.PrimaryAction)
	}
	if len(suggestions.Actions) != 3 {
		t.Fatalf("actions = %+v", suggestions.Actions)
	}
	if suggestions.Actions[1].ActionKind != "source_clip_gain_calibration" || suggestions.Actions[2].ActionKind != "source_clip_gain_calibration" {
		t.Fatalf("child actions = %+v", suggestions.Actions)
	}
}

func TestBuildGainStagingPackStrictReferenceCalibrationUsesRLMRowsBelowOutlierThreshold(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 strict reference level calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_a", "A", "clip_a", 0),
				b1TestTrackWithClip("track_b", "B", "clip_b", 0),
				b1TestTrackWithClip("track_c", "C", "clip_c", 0),
				b1TestTrackWithClip("track_d", "D", "clip_d", 0),
				b1TestTrackWithClip("track_e", "E", "clip_e", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "active_rms_dbfs": -21.0},
					{"track_id": "track_b", "active_rms_dbfs": -20.5},
					{"track_id": "track_c", "active_rms_dbfs": -20.0},
					{"track_id": "track_d", "active_rms_dbfs": -19.5},
					{"track_id": "track_e", "active_rms_dbfs": -19.0},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 0 {
		t.Fatalf("legacy outlier rows should stay empty for <3 dB deviations: %+v", rows)
	}
	strictRows := pack.Rankings["source_level_reference_calibration"]
	if len(strictRows) != 4 {
		t.Fatalf("strict RLM calibration rows = %+v", strictRows)
	}
	rlmSummary := pack.Summary["b1_2_reference_level_model"].(map[string]any)
	if rlmSummary["selected_metric"] != "active_rms_dbfs" || rlmSummary["mode"] != "strict" {
		t.Fatalf("RLM summary = %+v", rlmSummary)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("suggestions = %+v", suggestions)
	}
	if suggestions.PrimaryAction.ActionKind != "source_clip_gain_calibration_batch" || suggestions.PrimaryAction.Tool != "clip.gain.set_batch" {
		t.Fatalf("primary action = %+v", suggestions.PrimaryAction)
	}
	if len(suggestions.Actions) != 5 {
		t.Fatalf("batch plus 4 child actions expected, got %+v", suggestions.Actions)
	}
	if suggestions.PrimaryAction.Metadata["calibration_mode"] != "strict" || suggestions.PrimaryAction.Metadata["rlm_projection_id"] == "" {
		t.Fatalf("batch metadata = %+v", suggestions.PrimaryAction.Metadata)
	}
}

func TestGainStagingStrictReferenceCalibrationTreatsB12AsContract(t *testing.T) {
	for _, userIntent := range []string{"B1.2", "\u8fdb\u884cB1.2", "B1_2"} {
		if !gainStagingStrictReferenceCalibrationRequest(userIntent) {
			t.Fatalf("B1.2 must imply full-project same-reference calibration: %q", userIntent)
		}
	}
}

func TestBuildGainStagingPackChineseStrictReferenceCalibrationUsesRLMRows(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 \u4e25\u683c\u53c2\u8003\u7535\u5e73\u6821\u51c6\uff0c\u8ba9\u6240\u6709\u8f68\u9053\u8d34\u8fd1\u540c\u4e00\u53c2\u8003\u7535\u5e73",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_a", "A", "clip_a", 0),
				b1TestTrackWithClip("track_b", "B", "clip_b", 0),
				b1TestTrackWithClip("track_c", "C", "clip_c", 0),
			},
		},
		AudioAnalysisStatus: map[string]any{
			"analysis_job": map[string]any{
				"track_waveform_envelopes": []map[string]any{
					{"track_id": "track_a", "active_rms_dbfs": -22.0},
					{"track_id": "track_b", "active_rms_dbfs": -20.0},
					{"track_id": "track_c", "active_rms_dbfs": -18.0},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	strictRows := pack.Rankings["source_level_reference_calibration"]
	if len(strictRows) != 2 {
		t.Fatalf("Chinese strict RLM rows = %+v", strictRows)
	}
}

func TestBuildGainStagingPackB12AdmissionDoesNotMixRMSAndPeak(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet", "Quiet", "clip_quiet", 0),
				b1TestTrackWithClip("track_ref", "Reference", "clip_ref", 0),
				b1TestTrackWithClip("track_peak_only", "Peak Only", "clip_peak_only", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_mixed_metrics",
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "rms_dbfs": -30.0},
						{"track_id": "track_ref", "rms_dbfs": -20.0},
						{"track_id": "track_peak_only", "peak_dbfs": -8.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	admission := b1AdmissionSummary(t, pack)
	if admission["status"] != "partial" || admission["selected_metric"] != "rms_dbfs" {
		t.Fatalf("admission should be partial RMS, got %+v", admission)
	}
	if admission["candidate_count"] != 2 || admission["eligible_track_count"] != 3 || admission["missing_candidate_count"] != 1 {
		t.Fatalf("admission counts = %+v", admission)
	}
	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 0 {
		t.Fatalf("source level ranking should not mix RMS and peak: %+v", rows)
	}
	suggestions := BuildGainStagingSuggestions(pack)
	if suggestions.Status != "no_action" || suggestions.PrimaryAction != nil {
		t.Fatalf("suggestions should be blocked by partial same-metric coverage: %+v", suggestions)
	}
	limits := strings.Join(pack.Limitations, ",")
	if !strings.Contains(limits, "b1_2_source_level_same_metric_partial_coverage") ||
		!strings.Contains(limits, "b1_2_source_level_metrics_not_mixable") {
		t.Fatalf("limitations = %+v", pack.Limitations)
	}
}

func TestBuildGainStagingPackB12AdmissionMarksApproximateLUFS(t *testing.T) {
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{
				b1TestTrackWithClip("track_quiet", "Quiet", "clip_quiet", 0),
				b1TestTrackWithClip("track_ref_a", "Reference A", "clip_ref_a", 0),
				b1TestTrackWithClip("track_ref_b", "Reference B", "clip_ref_b", 0),
			},
		},
		MixObservation: map[string]any{
			"observation": map[string]any{
				"observation_id": "obs_approx_lufs",
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "approximate_lufs": -31.0},
						{"track_id": "track_ref_a", "approximate_lufs": -21.0},
						{"track_id": "track_ref_b", "approximate_lufs": -21.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	admission := b1AdmissionSummary(t, pack)
	if admission["status"] != "ready" || admission["selected_metric"] != "approximate_lufs" || admission["metric_approximate"] != true {
		t.Fatalf("admission = %+v", admission)
	}
	if !strings.Contains(strings.Join(pack.Limitations, ","), "b1_2_source_level_selected_metric_approximate") {
		t.Fatalf("limitations should mark approximate LUFS: %+v", pack.Limitations)
	}
	rows := pack.Rankings["source_level_outliers"]
	if len(rows) != 1 || rows[0].Metadata["metric_approximate"] != true {
		t.Fatalf("source level rankings = %+v", rows)
	}
}

func TestBuildGainStagingPackReadsStructMixObservation(t *testing.T) {
	type observationPacket struct {
		ObservationID  string         `json:"observation_id"`
		ProjectPackage map[string]any `json:"project_package"`
	}
	pack := BuildGainStagingPack(GainStagingInput{
		UserIntent: "B1.2 source calibration",
		ProjectState: map[string]any{
			"tracks": []map[string]any{{
				"track_id":  "track_quiet",
				"volume_db": 0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":  "track_ref_a",
				"volume_db": 0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_a",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":  "track_ref_b",
				"volume_db": 0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_b",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		MixObservation: map[string]any{
			"status": "ok",
			"observation": observationPacket{
				ObservationID: "obs_struct",
				ProjectPackage: map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "rms_dbfs": -30.0, "peak_dbfs": -12.0},
						{"track_id": "track_ref_a", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
						{"track_id": "track_ref_b", "rms_dbfs": -20.0, "peak_dbfs": -8.0},
					},
				},
			},
		},
		GeneratedAt: time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC),
	})

	if got := pack.Summary["tracks_with_acoustics"]; got != 3 {
		t.Fatalf("tracks_with_acoustics = %v summary=%+v", got, pack.Summary)
	}
	if rows := pack.Rankings["source_level_outliers"]; len(rows) != 1 || rows[0].ClipID != "clip_quiet" {
		t.Fatalf("source level rankings = %+v", rows)
	}
}

func TestBuildGainStagingSuggestionsTrackFaderOutlierUsesGroupAbsoluteReset(t *testing.T) {
	suggestions := BuildGainStagingSuggestions(Pack{
		CapabilityID: GainStagingCapabilityID,
		PackID:       "pack_fader",
		Rankings: map[string][]RankRow{
			"track_gain_outliers": {{
				TrackID:      "track_pad",
				TrackName:    "Pad",
				Kind:         "volume_db",
				Value:        -60,
				Unit:         "dB",
				Risk:         "medium",
				EvidenceRefs: []string{"project.state:tracks"},
			}, {
				TrackID:   "track_fx",
				TrackName: "FX",
				Kind:      "volume_db",
				Value:     -12,
				Unit:      "dB",
				Risk:      "medium",
			}},
		},
	})

	if suggestions.Status != "suggested" || suggestions.PrimaryAction == nil {
		t.Fatalf("fader suggestions = %+v", suggestions)
	}
	action := suggestions.PrimaryAction
	if action.ActionKind != "track_fader_unity_reset" || action.Tool != "track.group.apply_control" {
		t.Fatalf("fader action = %+v", action)
	}
	if len(action.TrackIDs) != 2 || action.TrackIDs[0] != "track_pad" || action.TrackIDs[1] != "track_fx" {
		t.Fatalf("fader action track IDs = %+v", action.TrackIDs)
	}
	if action.TargetDB == nil || *action.TargetDB != 0 {
		t.Fatalf("target db = %+v", action.TargetDB)
	}
	if action.DeltaDB == nil || *action.DeltaDB != 60 {
		t.Fatalf("delta db = %+v", action.DeltaDB)
	}
	if !action.RequiresConfirmation {
		t.Fatalf("fader reset action must require confirmation")
	}
}

func TestBuildGainStagingSuggestionsHeadroomRiskIsReportOnlyInV1(t *testing.T) {
	suggestions := BuildGainStagingSuggestions(Pack{
		CapabilityID: GainStagingCapabilityID,
		PackID:       "pack_headroom",
		Rankings: map[string][]RankRow{
			"headroom_risk": {{
				TrackID:   "track_drums",
				TrackName: "Drums",
				Kind:      "headroom_db",
				Value:     0.05,
				Unit:      "dB",
				Risk:      "high",
			}},
		},
	})

	if suggestions.Status != "no_action" || suggestions.PrimaryAction != nil {
		t.Fatalf("headroom suggestions = %+v", suggestions)
	}
	if !strings.Contains(strings.Join(suggestions.Limitations, ","), "headroom_risk_requires_source_or_clip_trim_calibration") {
		t.Fatalf("headroom limitations = %+v", suggestions.Limitations)
	}
}

func TestBuildGainStagingSuggestionsLowLevelRiskIsReportOnlyInV1(t *testing.T) {
	suggestions := BuildGainStagingSuggestions(Pack{
		CapabilityID: GainStagingCapabilityID,
		PackID:       "pack_low",
		Rankings: map[string][]RankRow{
			"low_level": {{
				TrackID:   "track_pad",
				TrackName: "Pad",
				Kind:      "rms_dbfs",
				Value:     -48,
				Unit:      "dBFS",
				Risk:      "medium",
			}},
		},
	})

	if suggestions.Status != "no_action" || suggestions.PrimaryAction != nil {
		t.Fatalf("low-level suggestions = %+v", suggestions)
	}
	if !strings.Contains(strings.Join(suggestions.Limitations, ","), "low_level_risk_requires_source_or_clip_trim_calibration") {
		t.Fatalf("low-level limitations = %+v", suggestions.Limitations)
	}
}

func b1TestTrackWithClip(trackID, trackName, clipID string, clipGainDB float64) map[string]any {
	return map[string]any{
		"track_id":   trackID,
		"track_name": trackName,
		"volume_db":  0.0,
		"clips": []map[string]any{{
			"clip_id":          clipID,
			"type":             "audio",
			"clip_gain_db":     clipGainDB,
			"duration_seconds": 12.0,
		}},
	}
}

func b1AdmissionSummary(t *testing.T, pack Pack) map[string]any {
	t.Helper()
	admission, ok := pack.Summary["b1_2_source_level_admission"].(map[string]any)
	if !ok || len(admission) == 0 {
		t.Fatalf("pack has no B1.2 source level admission summary: %+v", pack.Summary)
	}
	return admission
}
