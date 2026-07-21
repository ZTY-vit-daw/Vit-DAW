package panlayout

import (
	"testing"
	"time"

	"vit-daw-agent/internal/mixstyle"
)

func TestBuildSolveFullProjectPanLayout(t *testing.T) {
	tracks := []any{
		map[string]any{"track_id": "v", "track_name": "Lead Vocal", "track_type": "audio", "is_audio_track": true, "pan": .3, "channel_count": 1},
		map[string]any{"track_id": "g1", "track_name": "Guitar 1", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "g2", "track_name": "Guitar 2", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
	}
	tom := map[string]any{"full_assignment_manifest": map[string]any{"groups": []any{
		map[string]any{"role": "lead_vocal", "confidence": "high", "track_ids": []any{"v"}},
		map[string]any{"role": "guitar", "confidence": "high", "track_ids": []any{"g1", "g2"}},
	}}}
	mom := map[string]any{"mom_version": "v1.4", "multitrack_relation": map[string]any{"status": "ready", "track_count": 3}}
	model := BuildModel(Input{UserIntent: "modern pop B3", ProjectState: map[string]any{"tracks": tracks}, MOMProjection: mom, TOMProjection: tom, Style: mixstyle.Default(), GeneratedAt: time.Unix(1, 0)})
	if !model.Readiness.CanProceed || len(model.Tracks) != 3 {
		t.Fatalf("model readiness=%+v tracks=%+v", model.Readiness, model.Tracks)
	}
	result := Solve(model)
	if len(result.Candidates) != 3 {
		t.Fatalf("candidates=%+v", result.Candidates)
	}
	candidate, err := CompileDecision(model, result, Decision{CandidatePlanID: result.Candidates[1].CandidatePlanID, Disposition: "select"})
	if err != nil || len(candidate.Actions) != 3 {
		t.Fatalf("candidate=%+v err=%v", candidate, err)
	}
}

func TestUnknownAndRiskyStereoTracksDoNotBlockSafeLayout(t *testing.T) {
	tracks := []any{
		map[string]any{"track_id": "v", "track_name": "Lead Vocal", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "b1", "track_name": "Backing Vocal 1", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "b2", "track_name": "Backing Vocal 2", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "mystery", "track_name": "Audio 77", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 2},
	}
	tom := map[string]any{"full_assignment_manifest": map[string]any{"groups": []any{
		map[string]any{"role": "lead_vocal", "confidence": "high", "track_ids": []any{"v"}},
		map[string]any{"role": "backing_vocal", "confidence": "high", "track_ids": []any{"b1", "b2"}},
	}}}
	mom := map[string]any{"multitrack_relation": map[string]any{"status": "ready"}, "compared_tracks": []any{map[string]any{"track_id": "mystery", "correlation_estimate": .05}}}
	model := BuildModel(Input{ProjectState: map[string]any{"tracks": tracks}, MOMProjection: mom, TOMProjection: tom, Style: mixstyle.Default(), GeneratedAt: time.Unix(1, 0)})
	if !model.Readiness.CanProceed {
		t.Fatalf("readiness=%+v", model.Readiness)
	}
	result := Solve(model)
	for _, candidate := range result.Candidates {
		for _, a := range candidate.Actions {
			if a.TrackID == "mystery" {
				t.Fatalf("risky stereo action=%+v", a)
			}
		}
	}
}

func TestCandidateIDsAreDeterministicAndCannotBeInvented(t *testing.T) {
	in := Input{ProjectState: map[string]any{"tracks": []any{map[string]any{"track_id": "v", "track_name": "Lead Vocal", "track_type": "audio", "is_audio_track": true, "pan": .2}, map[string]any{"track_id": "b1", "track_name": "Backing Vocal 1", "track_type": "audio", "is_audio_track": true, "pan": 0.0}, map[string]any{"track_id": "b2", "track_name": "Backing Vocal 2", "track_type": "audio", "is_audio_track": true, "pan": 0.0}}}, TOMProjection: map[string]any{"full_assignment_manifest": map[string]any{"groups": []any{map[string]any{"role": "lead_vocal", "confidence": "high", "track_ids": []any{"v"}}, map[string]any{"role": "backing_vocal", "confidence": "high", "track_ids": []any{"b1", "b2"}}}}}, MOMProjection: map[string]any{"multitrack_relation": map[string]any{"status": "ready"}}, Style: mixstyle.Default(), GeneratedAt: time.Unix(1, 0)}
	a, b := Solve(BuildModel(in)), Solve(BuildModel(in))
	if a.Candidates[0].CandidatePlanID != b.Candidates[0].CandidatePlanID {
		t.Fatalf("ids differ %s %s", a.Candidates[0].CandidatePlanID, b.Candidates[0].CandidatePlanID)
	}
	if _, err := CompileDecision(BuildModel(in), a, Decision{CandidatePlanID: "invented"}); err == nil {
		t.Fatal("invented candidate accepted")
	}
}
