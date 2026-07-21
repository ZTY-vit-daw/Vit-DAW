package staticbalance

import (
	"fmt"
	"math"
	"testing"
	"time"

	"vit-daw-agent/internal/mixstyle"
)

func TestAllTrackModelAndSolverDoNotUseDisclosureCaps(t *testing.T) {
	for _, trackCount := range []int{61, 128, 256} {
		t.Run(fmt.Sprint(trackCount), func(t *testing.T) {
			input := largeProjectInput(t, trackCount, trackCount)
			model := BuildModel(input)
			if got := len(model.Tracks); got != trackCount {
				t.Fatalf("analyzed tracks = %d, want %d", got, trackCount)
			}
			if model.Coverage.AnalyzedTrackCount != trackCount || model.Coverage.ComparableCount != trackCount {
				t.Fatalf("coverage = %+v", model.Coverage)
			}
			if !model.Readiness.CanProceed {
				t.Fatalf("readiness blocked: %+v", model.Readiness)
			}
			result := Solve(model)
			if result.AnalyzedTrackCount != trackCount || len(result.Candidates) != 3 {
				t.Fatalf("result = tracks=%d candidates=%d limitations=%v", result.AnalyzedTrackCount, len(result.Candidates), result.Limitations)
			}
			for _, candidate := range result.Candidates {
				if err := ValidateCandidate(model, candidate); err != nil {
					t.Fatalf("candidate %s: %v", candidate.Strategy, err)
				}
			}
		})
	}
}

func TestReadinessBlocksSparseTrackLevelEvidenceWithoutTruncatingModel(t *testing.T) {
	input := largeProjectInput(t, 61, 1)
	model := BuildModel(input)
	if len(model.Tracks) != 61 || model.Coverage.ComparableCount != 1 {
		t.Fatalf("model coverage = %+v tracks=%d", model.Coverage, len(model.Tracks))
	}
	if model.Readiness.CanProceed {
		t.Fatalf("sparse evidence should block readiness: %+v", model.Readiness)
	}
	result := Solve(model)
	if len(result.Candidates) != 0 {
		t.Fatalf("blocked readiness produced candidates: %+v", result.Candidates)
	}
}

func TestB2UsesDADEffectiveLevelsAndIgnoresGenericL3ActionGate(t *testing.T) {
	input := largeProjectInput(t, 3, 0)
	input.ProjectState = map[string]any{"tracks": []any{
		map[string]any{"track_id": "track_001", "track_name": "Lead Vocal", "track_type": "audio", "volume_db": -2.0, "clips": []any{map[string]any{"id": "clip_v", "clip_gain_db": 6.0, "length_seconds": 10.0}}},
		map[string]any{"track_id": "track_002", "track_name": "Drums", "track_type": "audio", "volume_db": 0.0, "clips": []any{map[string]any{"id": "clip_d", "clip_gain_db": 3.0, "length_seconds": 10.0}}},
		map[string]any{"track_id": "track_003", "track_name": "Bass", "track_type": "audio", "volume_db": 1.0, "clips": []any{map[string]any{"id": "clip_b", "clip_gain_db": 2.0, "length_seconds": 10.0}}},
	}}
	input.MixObservation = map[string]any{"observation_id": "obs_dad", "mom_projection": map[string]any{
		"trust_quality": map[string]any{"can_support_action_preflight": false},
		"multitrack_relation": map[string]any{"status": "partial", "compared_tracks": []any{
			map[string]any{"track_id": "track_001"}, map[string]any{"track_id": "track_002"}, map[string]any{"track_id": "track_003"},
		}},
	}}
	input.AudioAnalysisStatus = map[string]any{"analysis_job": map[string]any{"track_waveform_envelopes": []any{
		map[string]any{"status": "ready", "track_id": "track_001", "clip_id": "clip_v", "rms_dbfs": -30.0, "peak_dbfs": -10.0, "duration_seconds": 10.0},
		map[string]any{"status": "ready", "track_id": "track_002", "clip_id": "clip_d", "rms_dbfs": -27.0, "peak_dbfs": -9.0, "duration_seconds": 10.0},
		map[string]any{"status": "ready", "track_id": "track_003", "clip_id": "clip_b", "rms_dbfs": -25.0, "peak_dbfs": -8.0, "duration_seconds": 10.0},
	}}}

	model := BuildModel(input)
	if !model.Readiness.CanProceed {
		t.Fatalf("B2-specific readiness blocked on generic MOM gate: %+v", model.Readiness)
	}
	if model.Coverage.DADLevelKnownCount != 3 || model.Coverage.ComparableCount != 3 {
		t.Fatalf("coverage = %+v", model.Coverage)
	}
	track := model.Tracks[0]
	if track.LevelMetric != "effective_static_rms_dbfs" || track.LevelSource != "dad_waveform_effective_static" || track.LevelDB == nil || math.Abs(*track.LevelDB-(-26.0)) > 0.001 {
		t.Fatalf("effective vocal level = %+v, want -30 + 6 - 2 = -26 dBFS", track)
	}
	if track.PeakDBFS == nil || math.Abs(*track.PeakDBFS-(-6.0)) > 0.001 {
		t.Fatalf("effective vocal peak = %+v, want -10 + 6 - 2 = -6 dBFS", track.PeakDBFS)
	}
	if len(Solve(model).Candidates) == 0 {
		t.Fatal("complete B2 evidence did not produce candidates")
	}
}

func TestKeysOrSynthRoleMapsToHarmonicBed(t *testing.T) {
	if got := roleFunction("keys_or_synth"); got != "harmonic_bed" {
		t.Fatalf("keys_or_synth function = %q", got)
	}
}

func TestDirectMOMProjectionInputTakesProjectionBody(t *testing.T) {
	input := largeProjectInput(t, 8, 8)
	input.MOMProjection = mapValue(input.MixObservation["mom_projection"])
	input.MixObservation = map[string]any{"observation_id": "obs_direct_mom"}
	model := BuildModel(input)
	if !model.Readiness.CanProceed || len(model.MOMSummary) == 0 {
		t.Fatalf("direct MOM projection was not admitted: readiness=%+v summary=%+v", model.Readiness, model.MOMSummary)
	}
}

func TestCompileDecisionOnlyResolvesExistingValidatedCandidate(t *testing.T) {
	model := BuildModel(largeProjectInput(t, 16, 16))
	result := Solve(model)
	if len(result.Candidates) == 0 {
		t.Fatal("expected solver candidates")
	}
	selected, err := CompileDecision(model, result, Decision{Disposition: "select", CandidatePlanID: result.Candidates[1].CandidatePlanID})
	if err != nil || selected.CandidatePlanID != result.Candidates[1].CandidatePlanID {
		t.Fatalf("compile selected=%+v err=%v", selected, err)
	}
	if _, err := CompileDecision(model, result, Decision{Disposition: "select", CandidatePlanID: "invented"}); err == nil {
		t.Fatal("invented candidate id was accepted")
	}
}

func TestSolverPreservesWithinFunctionBalanceWithCommonGroupDelta(t *testing.T) {
	model := BuildModel(largeProjectInput(t, 48, 48))
	result := Solve(model)
	if len(result.Candidates) < 2 {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	byFunction := map[string]float64{}
	for _, action := range result.Candidates[1].Actions {
		if previous, ok := byFunction[action.Function]; ok && previous != action.DeltaDB {
			t.Fatalf("function %s received inconsistent deltas %.2f and %.2f", action.Function, previous, action.DeltaDB)
		}
		byFunction[action.Function] = action.DeltaDB
	}
	if len(byFunction) < 2 {
		t.Fatalf("function deltas = %+v", byFunction)
	}
}

func largeProjectInput(t *testing.T, trackCount, levelCount int) Input {
	t.Helper()
	style, err := mixstyle.Builtin("modern_pop")
	if err != nil {
		t.Fatal(err)
	}
	tracks := make([]any, 0, trackCount)
	levels := make([]any, 0, levelCount)
	roles := []string{"lead_vocal", "drums", "bass", "strings", "backing_vocal", "effect"}
	groups := make([]any, len(roles))
	assignments := make([][]any, len(roles))
	for index := 0; index < trackCount; index++ {
		id := fmt.Sprintf("track_%03d", index+1)
		tracks = append(tracks, map[string]any{
			"track_id": id, "track_name": fmt.Sprintf("Stem %03d", index+1), "track_type": "audio", "volume_db": 0.0,
		})
		roleIndex := index % len(roles)
		assignments[roleIndex] = append(assignments[roleIndex], map[string]any{"track_id": id, "confidence": "high"})
		if index < levelCount {
			levels = append(levels, map[string]any{
				"track_id": id, "rms_dbfs": -24.0 + float64(index%9), "peak_dbfs": -8.0 + float64(index%3), "headroom_db": 8.0,
			})
		}
	}
	for index, role := range roles {
		groups[index] = map[string]any{
			"group_id": role, "role_hypothesis": role, "confidence": "high", "assignments": assignments[index],
		}
	}
	return Input{
		UserIntent:    "modern pop B2 static balance",
		GeneratedAt:   time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC),
		Style:         style,
		StyleExplicit: true,
		ProjectState:  map[string]any{"tracks": tracks},
		ContextSnapshot: map[string]any{"tom_projection": map[string]any{
			"tom_version":              "v0.2",
			"full_assignment_manifest": map[string]any{"track_count": trackCount, "groups": groups},
		}},
		MixObservation: map[string]any{
			"observation_id": "obs_large",
			"mom_projection": map[string]any{
				"mom_version":         "v1.4",
				"trust_quality":       map[string]any{"can_support_action_preflight": true},
				"multitrack_relation": map[string]any{"track_count": trackCount, "compared_tracks": levels},
			},
		},
	}
}
