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

func TestB2UsesMOMStaticLevelProjectionAndIgnoresGenericL3ActionGate(t *testing.T) {
	input := largeProjectInput(t, 3, 0)
	input.ProjectState = map[string]any{"tracks": []any{
		map[string]any{"track_id": "track_001", "track_name": "Lead Vocal", "track_type": "audio", "volume_db": -2.0, "clips": []any{map[string]any{"id": "clip_v", "clip_gain_db": 6.0, "length_seconds": 10.0}}},
		map[string]any{"track_id": "track_002", "track_name": "Drums", "track_type": "audio", "volume_db": 0.0, "clips": []any{map[string]any{"id": "clip_d", "clip_gain_db": 3.0, "length_seconds": 10.0}}},
		map[string]any{"track_id": "track_003", "track_name": "Bass", "track_type": "audio", "volume_db": 1.0, "clips": []any{map[string]any{"id": "clip_b", "clip_gain_db": 2.0, "length_seconds": 10.0}}},
	}}
	input.MixObservation = map[string]any{"observation_id": "obs_static", "mom_projection": map[string]any{
		"trust_quality": map[string]any{"can_support_action_preflight": false},
		"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
			map[string]any{"track_id": "track_001"}, map[string]any{"track_id": "track_002"}, map[string]any{"track_id": "track_003"},
		}},
		"static_level_relationship": staticLevelProjection("ready", []any{
			staticLevelRow("track_001", "ready", -26.0, -6.0, "clip_v"),
			staticLevelRow("track_002", "ready", -24.0, -6.0, "clip_d"),
			staticLevelRow("track_003", "ready", -22.0, -5.0, "clip_b"),
		}),
	}}

	model := BuildModel(input)
	if !model.Readiness.CanProceed {
		t.Fatalf("B2-specific readiness blocked on generic MOM gate: %+v", model.Readiness)
	}
	if model.Coverage.ProjectedLevelKnownCount != 3 || model.Coverage.ComparableCount != 3 {
		t.Fatalf("coverage = %+v", model.Coverage)
	}
	track := model.Tracks[0]
	if track.LevelMetric != "effective_static_rms_dbfs" || track.LevelSource != "mom_static_level_relationship" || track.LevelDB == nil || math.Abs(*track.LevelDB-(-26.0)) > 0.001 {
		t.Fatalf("effective vocal level = %+v", track)
	}
	if track.PeakDBFS == nil || math.Abs(*track.PeakDBFS-(-6.0)) > 0.001 {
		t.Fatalf("effective vocal peak = %+v, want -10 + 6 - 2 = -6 dBFS", track.PeakDBFS)
	}
	if len(Solve(model).Candidates) == 0 {
		t.Fatal("complete B2 evidence did not produce candidates")
	}
}

func TestB2BlocksWhenTypedStaticLevelProjectionIsMissing(t *testing.T) {
	input := largeProjectInput(t, 3, 0)
	input.MixObservation = map[string]any{"observation_id": "obs_missing_static", "mom_projection": map[string]any{
		"mom_version":   "v1.5",
		"trust_quality": map[string]any{"can_support_action_preflight": true},
		"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
			map[string]any{"track_id": "track_001", "effective_static_rms_dbfs": -40.0},
			map[string]any{"track_id": "track_002", "effective_static_rms_dbfs": -24.0},
			map[string]any{"track_id": "track_003", "effective_static_rms_dbfs": -22.0},
		}},
	}}
	model := BuildModel(input)
	if model.Readiness.CanProceed {
		t.Fatalf("generic MOM levels must not replace typed projection: %+v", model.Readiness)
	}
	if candidates := Solve(model).Candidates; len(candidates) != 0 {
		t.Fatalf("blocked B2 produced candidates: %+v", candidates)
	}
}

func TestB2BlocksPartialStaticLevelProjection(t *testing.T) {
	input := largeProjectInput(t, 3, 0)
	input.MixObservation = map[string]any{"observation_id": "obs_partial", "mom_projection": map[string]any{
		"mom_version": "v1.5", "trust_quality": map[string]any{"can_support_action_preflight": true},
		"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
			map[string]any{"track_id": "track_001"}, map[string]any{"track_id": "track_002"}, map[string]any{"track_id": "track_003"},
		}},
		"static_level_relationship": staticLevelProjection("partial", []any{
			staticLevelRow("track_001", "ready", -20.0, -8.0, "clip_1"),
			staticLevelRow("track_002", "ready", -24.0, -8.0, "clip_2"),
			map[string]any{"track_id": "track_003", "status": "missing", "metric": "effective_static_rms_dbfs"},
		}),
	}}
	model := BuildModel(input)
	if model.Readiness.CanProceed || len(Solve(model).Candidates) != 0 {
		t.Fatalf("partial static relationship must block B2: %+v", model.Readiness)
	}
}

func TestB2AcceptsPartialStaticLevelProjectionAtNinetyFivePercentCoverage(t *testing.T) {
	input := largeProjectInput(t, 61, 59)
	input.ProjectState["snapshot_hash"] = "cut-1"
	projection := mapValue(input.MixObservation["mom_projection"])
	staticRelation := mapValue(projection["static_level_relationship"])
	staticRelation["status"] = "partial"
	staticRelation["tracks"] = append(rowsToAny(rowsValue(staticRelation["tracks"])),
		map[string]any{"track_id": "track_060", "status": "partial", "freshness": "fresh", "metric": "effective_static_rms_dbfs"},
		map[string]any{"track_id": "track_061", "status": "missing", "freshness": "fresh", "metric": "effective_static_rms_dbfs"},
	)

	model := BuildModel(input)
	if model.Coverage.ProjectedLevelKnownCount != 59 || model.Coverage.RoleCandidateCount != 61 || model.Coverage.EffectiveLevelCoverage < 0.95 {
		t.Fatalf("unexpected partial coverage: %+v", model.Coverage)
	}
	if !model.Readiness.CanProceed {
		t.Fatalf(">=95%% usable partial relationship should be B2-ready: %+v", model.Readiness)
	}
	if candidates := Solve(model).Candidates; len(candidates) == 0 {
		t.Fatal(">=95% usable partial relationship produced no candidates")
	}
}

func TestB2BlocksOverallStaleOrSuspectStaticLevelProjectionEvenAtFullCoverage(t *testing.T) {
	for _, status := range []string{"stale", "suspect"} {
		t.Run(status, func(t *testing.T) {
			input := largeProjectInput(t, 61, 61)
			projection := mapValue(input.MixObservation["mom_projection"])
			mapValue(projection["static_level_relationship"])["status"] = status

			model := BuildModel(input)
			if model.Coverage.EffectiveLevelCoverage != 1 {
				t.Fatalf("test requires full usable coverage: %+v", model.Coverage)
			}
			if model.Readiness.CanProceed || len(Solve(model).Candidates) != 0 {
				t.Fatalf("overall %s relationship must block B2: %+v", status, model.Readiness)
			}
		})
	}
}

func TestB2BlocksStaleOrSuspectStaticLevelProjection(t *testing.T) {
	for _, status := range []string{"stale", "suspect"} {
		t.Run(status, func(t *testing.T) {
			input := largeProjectInput(t, 3, 0)
			input.MixObservation = map[string]any{"observation_id": "obs_" + status, "mom_projection": map[string]any{
				"mom_version": "v1.5",
				"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
					map[string]any{"track_id": "track_001"}, map[string]any{"track_id": "track_002"}, map[string]any{"track_id": "track_003"},
				}},
				"static_level_relationship": staticLevelProjection(status, []any{
					staticLevelRow("track_001", status, -20.0, -8.0, "clip_1"),
					staticLevelRow("track_002", "ready", -24.0, -8.0, "clip_2"),
					staticLevelRow("track_003", "ready", -22.0, -8.0, "clip_3"),
				}),
			}}
			model := BuildModel(input)
			if model.Readiness.CanProceed || len(Solve(model).Candidates) != 0 {
				t.Fatalf("%s static relationship must block B2: readiness=%+v candidates=%+v", status, model.Readiness, Solve(model).Candidates)
			}
			if model.Coverage.BlockedLevelCount != 1 {
				t.Fatalf("%s track was not counted as blocked: %+v", status, model.Coverage)
			}
		})
	}
}

func TestB2AcceptsApproximateHomogeneousSplitProjection(t *testing.T) {
	input := largeProjectInput(t, 3, 0)
	input.MixObservation = map[string]any{"observation_id": "obs_split", "mom_projection": map[string]any{
		"mom_version": "v1.5", "trust_quality": map[string]any{"can_support_action_preflight": true},
		"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
			map[string]any{"track_id": "track_001"}, map[string]any{"track_id": "track_002"}, map[string]any{"track_id": "track_003"},
		}},
		"static_level_relationship": staticLevelProjection("approximate", []any{
			staticLevelRow("track_001", "approximate", -30.0, -8.0, "clip_a", "clip_b"),
			staticLevelRow("track_002", "ready", -24.0, -8.0, "clip_2"),
			staticLevelRow("track_003", "ready", -22.0, -8.0, "clip_3"),
		}),
	}}
	model := BuildModel(input)
	if !model.Readiness.CanProceed {
		t.Fatalf("homogeneous split projection should be B2-ready: %+v", model.Readiness)
	}
	if model.Coverage.ApproximateLevelCount != 1 || model.Tracks[0].LevelDB == nil || len(model.Tracks[0].IncludedClipIDs) != 2 {
		t.Fatalf("approximate split projection not preserved: model=%+v track=%+v", model.Coverage, model.Tracks[0])
	}
}

func TestB2BlocksStaticProjectionFromDifferentProjectCut(t *testing.T) {
	input := largeProjectInput(t, 3, 3)
	input.ProjectState["snapshot_hash"] = "current-cut"
	projection := mapValue(input.MixObservation["mom_projection"])
	mapValue(projection["static_level_relationship"])["project_cut_ref"] = "old-cut"
	model := BuildModel(input)
	if model.Readiness.CanProceed {
		t.Fatalf("stale project-cut projection must block B2: %+v", model.Readiness)
	}
	found := false
	for _, blocker := range model.Readiness.BlockedBy {
		if blocker == "mom_static_level_project_cut" {
			found = true
		}
	}
	if !found {
		t.Fatalf("project-cut blocker missing: %+v", model.Readiness)
	}
}

func TestB2BlocksStaticProjectionWithoutProjectCut(t *testing.T) {
	input := largeProjectInput(t, 3, 3)
	projection := mapValue(input.MixObservation["mom_projection"])
	delete(mapValue(projection["static_level_relationship"]), "project_cut_ref")
	model := BuildModel(input)
	if model.Readiness.CanProceed {
		t.Fatalf("projection without project cut must block B2: %+v", model.Readiness)
	}
	found := false
	for _, blocker := range model.Readiness.BlockedBy {
		if blocker == "mom_static_level_project_cut" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing project-cut blocker not reported: %+v", model.Readiness)
	}
}

func staticLevelProjection(status string, tracks []any) map[string]any {
	return map[string]any{"schema_version": "mom.static_level_relationship.v1", "status": status, "freshness": "fresh", "project_cut_ref": "cut-1", "tracks": tracks}
}

func staticLevelRow(trackID, status string, rms, peak float64, clipIDs ...string) map[string]any {
	return map[string]any{
		"track_id": trackID, "status": status, "freshness": "fresh", "metric": "effective_static_rms_dbfs",
		"tap_point": "derived_static_control_model", "effective_static_rms_dbfs": rms, "effective_static_peak_dbfs": peak,
		"included_clip_ids": clipIDs, "aggregation_method": "duration_weighted_linear_energy_source_plus_clip_gain_plus_fader",
	}
}

func rowsToAny(rows []map[string]any) []any {
	out := make([]any, len(rows))
	for index := range rows {
		out[index] = rows[index]
	}
	return out
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

func TestSolverRoleFunctionExchangeChangesTrackHierarchyPlan(t *testing.T) {
	base := BuildModel(largeProjectInput(t, 6, 6))
	if !base.Readiness.CanProceed {
		t.Fatalf("base readiness blocked: %+v", base.Readiness)
	}
	first := Solve(base)
	foregroundDelta, ok := candidateTrackDelta(first.Candidates[1], "track_001")
	if !ok {
		t.Fatalf("foreground track missing from base candidate: %+v", first.Candidates[1].Actions)
	}

	swapped := base
	swapped.Tracks = append([]Track(nil), base.Tracks...)
	swapped.Tracks[0].Role, swapped.Tracks[0].Function = "backing_vocal", "support"
	swapped.Tracks[4].Role, swapped.Tracks[4].Function = "lead_vocal", "foreground"
	second := Solve(swapped)
	supportDelta, ok := candidateTrackDelta(second.Candidates[1], "track_001")
	if !ok {
		t.Fatalf("support track missing from swapped candidate: %+v", second.Candidates[1].Actions)
	}
	if foregroundDelta <= supportDelta {
		t.Fatalf("role/function exchange did not change hierarchy direction: foreground=%.2f support=%.2f", foregroundDelta, supportDelta)
	}
}

func TestSolverVMSForegroundPriorityChangesForegroundCandidate(t *testing.T) {
	base := BuildModel(largeProjectInput(t, 6, 6))
	for index := range base.Tracks {
		if base.Tracks[index].TrackID != "track_001" && base.Tracks[index].TrackID != "track_005" {
			base.Tracks[index].Eligible = false
			continue
		}
		level := -18.0
		base.Tracks[index].LevelDB = &level
	}
	low := base
	low.Style.Dimensions.ForegroundPriority = -1
	high := base
	high.Style.Dimensions.ForegroundPriority = 1

	lowDelta, ok := candidateTrackDelta(Solve(low).Candidates[1], "track_001")
	if !ok {
		t.Fatal("low-priority VMS omitted foreground action")
	}
	highDelta, ok := candidateTrackDelta(Solve(high).Candidates[1], "track_001")
	if !ok {
		t.Fatal("high-priority VMS omitted foreground action")
	}
	if highDelta <= lowDelta {
		t.Fatalf("foreground_priority did not affect candidate: low=%.2f high=%.2f", lowDelta, highDelta)
	}
}

func candidateTrackDelta(candidate CandidatePlan, trackID string) (float64, bool) {
	for _, action := range candidate.Actions {
		if action.TrackID == trackID {
			return action.DeltaDB, true
		}
	}
	return 0, false
}

func largeProjectInput(t *testing.T, trackCount, levelCount int) Input {
	t.Helper()
	style, err := mixstyle.Builtin("modern_pop")
	if err != nil {
		t.Fatal(err)
	}
	tracks := make([]any, 0, trackCount)
	levels := make([]any, 0, levelCount)
	staticLevels := make([]any, 0, levelCount)
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
			staticLevels = append(staticLevels, staticLevelRow(id, "ready", -24.0+float64(index%9), -8.0+float64(index%3), "clip_"+id))
		}
	}
	for index, role := range roles {
		groups[index] = map[string]any{
			"group_id": role, "role_hypothesis": role, "confidence": "high", "assignments": assignments[index],
		}
	}
	staticStatus := "ready"
	if levelCount == 0 {
		staticStatus = "missing"
	} else if levelCount < trackCount {
		staticStatus = "partial"
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
				"mom_version":               "v1.5",
				"trust_quality":             map[string]any{"can_support_action_preflight": true},
				"multitrack_relation":       map[string]any{"status": "ready", "track_count": trackCount, "compared_tracks": levels},
				"static_level_relationship": staticLevelProjection(staticStatus, staticLevels),
			},
		},
	}
}
