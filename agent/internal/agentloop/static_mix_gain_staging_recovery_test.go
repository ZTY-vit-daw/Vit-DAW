package agentloop

import (
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
)

func TestGainStagingAnalysisNeedsRetryAfterClipEdits(t *testing.T) {
	status := map[string]any{
		"analysis_job": map[string]any{
			"analysis_job_id":      "audio_analysis_test",
			"status":               "submitted",
			"dad_fact_ready_count": 11,
			"dad_fact_total_count": 61,
		},
	}
	if !messageLoopGainStagingAnalysisNeedsRetry(status) {
		t.Fatal("submitted partial DAD job should be retryable")
	}
	if !messageLoopGainStagingAnalysisNeedsRecovery(nil) {
		t.Fatal("missing DAD job should be rebuilt from the current project")
	}
	call := messageLoopGainStagingAudioAnalysisRetryCall(status)
	if call.Tool != "project.audio_analysis_start" || call.Args["retry_missing"] != true || call.Args["rebuild_from_project"] != true || call.Args["analysis_job_id"] != "audio_analysis_test" {
		t.Fatalf("retry call = %+v", call)
	}

	status["analysis_job"].(map[string]any)["dad_fact_ready_count"] = 61
	if messageLoopGainStagingAnalysisNeedsRetry(status) {
		t.Fatal("fully ready DAD job must not be retried")
	}
}

func TestGainStagingBatchExpandsTrackDeltaToAllSurvivingClips(t *testing.T) {
	delta := 3.5
	target := 3.5
	batch := capabilitycontext.GainStagingSuggestion{
		ActionKind: "source_clip_gain_calibration_batch",
		Tool:       "clip.gain.set_batch",
	}
	trackAction := capabilitycontext.GainStagingSuggestion{
		ActionKind: "source_clip_gain_calibration",
		Tool:       "clip.gain.set",
		TrackID:    "track_1",
		TrackName:  "Lead",
		ClipID:     "primary",
		DeltaDB:    &delta,
		TargetDB:   &target,
	}
	projectState := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "track_1",
				"clips": []any{
					map[string]any{"clip_id": "clip_a", "clip_type": "wave", "clip_gain_db": -2.0},
					map[string]any{"clip_id": "clip_b", "clip_type": "wave", "clip_gain_db": 1.0},
				},
			},
		},
	}

	call := messageLoopGainStagingClipGainSetBatchCall(batch, []capabilitycontext.GainStagingSuggestion{trackAction}, projectState)
	actions := messageLoopMapRows(call.Args["pending_actions"])
	if call.Tool != "clip.gain.set_batch" || len(actions) != 2 {
		t.Fatalf("expanded batch = tool=%q actions=%+v", call.Tool, actions)
	}
	want := map[string]float64{"clip_a": 1.5, "clip_b": 4.5}
	for _, row := range actions {
		args := messageLoopMapValue(row["args"])
		clipID := firstMapText(args, "clip_id")
		gain, ok := firstNumericMapValue(args, "gain_db")
		if !ok || gain != want[clipID] {
			t.Fatalf("clip %q target = %v ok=%t, want %v", clipID, gain, ok, want[clipID])
		}
	}
	summary := messageLoopMapValue(call.Args["calibration_summary"])
	if summary["track_action_count"] != 1 || summary["clip_action_count"] != 2 {
		t.Fatalf("calibration summary = %+v", summary)
	}
}

func TestB12CompletionRejectsUnsafeEffectiveStaticPeak(t *testing.T) {
	peak := -23.718
	effectivePeak := 0.282
	gain := 24.0
	pack := capabilitycontext.Pack{
		Tracks: []capabilitycontext.TrackGainRow{{
			TrackID: "1062", TrackName: "Quiet metal",
			PeakDBFS: &peak, EffectiveStaticPeakDBFS: &effectivePeak,
			PrimaryClip: &capabilitycontext.ClipGainRow{ClipID: "1066", GainDB: &gain},
		}},
	}
	unsafe := messageLoopB12UnsafeTargetPeaks(pack, []messageLoopB12SourceCalibrationTarget{{TrackID: "1062", ClipID: "1066", TargetGainDB: 24}})
	if len(unsafe) != 1 || unsafe[0].TrackID != "1062" || unsafe[0].PeakDBFS != 0.282 {
		t.Fatalf("unsafe peaks = %+v", unsafe)
	}
}

func TestGainStagingBatchExpansionKeepsEverySplitClipPeakSafe(t *testing.T) {
	delta := 22.718
	target := 22.718
	action := capabilitycontext.GainStagingSuggestion{
		ActionKind: "source_clip_gain_calibration", Tool: "clip.gain.set", TrackID: "1062", ClipID: "1066",
		DeltaDB: &delta, TargetDB: &target, Metadata: map[string]any{"observed_source_peak_dbfs": -23.718},
	}
	projectState := map[string]any{"tracks": []any{map[string]any{
		"track_id": "1062", "clips": []any{
			map[string]any{"clip_id": "1066", "clip_type": "wave", "clip_gain_db": 0.0},
			map[string]any{"clip_id": "1911", "clip_type": "wave", "clip_gain_db": 5.0},
		},
	}}}
	expanded := messageLoopGainStagingExpandTrackCalibration(action, projectState)
	if len(expanded) != 2 {
		t.Fatalf("expanded = %+v", expanded)
	}
	for _, row := range expanded {
		if row.TargetDB == nil || *row.TargetDB > 22.718+0.001 {
			t.Fatalf("split clip target is not peak-safe: %+v", row)
		}
	}
	if expanded[1].DeltaDB == nil || mathAbs(*expanded[1].DeltaDB-17.718) > 0.001 {
		t.Fatalf("second split clip delta = %+v", expanded[1])
	}
}

func TestB12CompletionAcceptsPeakAtSafetyCeiling(t *testing.T) {
	peak := -23.718
	gain := 22.718
	pack := capabilitycontext.Pack{
		Tracks: []capabilitycontext.TrackGainRow{{
			TrackID: "1062", TrackName: "Quiet metal", PeakDBFS: &peak,
			PrimaryClip: &capabilitycontext.ClipGainRow{ClipID: "1066", GainDB: &gain},
		}},
	}
	unsafe := messageLoopB12UnsafeTargetPeaks(pack, []messageLoopB12SourceCalibrationTarget{{TrackID: "1062", ClipID: "1066", TargetGainDB: gain}})
	if len(unsafe) != 0 {
		t.Fatalf("unsafe peaks = %+v", unsafe)
	}
}
