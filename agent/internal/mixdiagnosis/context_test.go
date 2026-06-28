package mixdiagnosis

import (
	"strings"
	"testing"
)

func TestLowMudMissingBandUsesConservativeProbe(t *testing.T) {
	ctx := Build(Input{
		ConversationID: "chat_1",
		GoalID:         "goal_1",
		RunID:          "run_1",
		UserIntent:     "低频有点糊，帮我清一下",
		TargetRef:      "track:1007",
		ObservationSummary: map[string]any{
			"observation_id": "obs_1",
			"mix_package": map[string]any{
				"missing_metrics": []any{"band_energy_summary", "stereo_correlation"},
				"source_capabilities": map[string]any{
					"band_energy": "missing",
				},
				"current_metrics": map[string]any{
					"waveform": map[string]any{"status": "ready", "headroom_db": 6.0},
				},
			},
			"deep_package": map[string]any{
				"source_capabilities": map[string]any{"deep_band_observation": "missing"},
			},
		},
	})
	if ctx.SchemaVersion != SchemaVersion || ctx.ProblemKind != "low_mud" {
		t.Fatalf("context identity = %+v", ctx)
	}
	if ctx.Recommendation.Strategy != "conservative_probe" || ctx.LowRiskNextStep != "conservative_probe" {
		t.Fatalf("recommendation = %+v low=%q", ctx.Recommendation, ctx.LowRiskNextStep)
	}
	if !hasMissing(ctx, "band_energy_summary") || !hasMissing(ctx, "spectrogram_tiles") {
		t.Fatalf("missing evidence = %+v", ctx.MissingEvidence)
	}
	for _, fact := range ctx.ObservedFacts {
		if strings.Contains(strings.ToLower(fact.Summary), "buildup") || strings.Contains(fact.Summary, "堆积") {
			t.Fatalf("missing-band context must not claim buildup as observed: %+v", fact)
		}
	}
}

func TestLowMudReadyBandCitesBandSummary(t *testing.T) {
	ctx := Build(Input{
		UserIntent: "reduce low-mid mud",
		TargetRef:  "track:bass",
		ObservationSummary: map[string]any{
			"observation_id": "obs_ready",
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"band_energy": map[string]any{
						"status": "ready",
						"source": "feature_snapshot",
						"bands":  map[string]any{"bass": "-18.2", "low_mid": "-14.6"},
					},
				},
			},
		},
	})
	if ctx.Recommendation.Strategy != "eq_cut_low_mid" {
		t.Fatalf("strategy = %+v", ctx.Recommendation)
	}
	if !hasObserved(ctx, "band_energy_summary") {
		t.Fatalf("observed facts = %+v", ctx.ObservedFacts)
	}
	if !containsRef(ctx.EvidenceRefs, "observed.band_energy_summary") {
		t.Fatalf("evidence refs = %+v", ctx.EvidenceRefs)
	}
}

func TestVocalForwardAmbiguousRequestsTargetClarification(t *testing.T) {
	ctx := Build(Input{
		UserIntent: "让人声更靠前一点",
		TargetRef:  "project",
		ObservationSummary: map[string]any{
			"observation_id": "obs_vocal",
		},
	})
	if ctx.ProblemKind != "vocal_forward" {
		t.Fatalf("problem kind = %s", ctx.ProblemKind)
	}
	if ctx.Recommendation.Strategy != "request_target_clarification" || !hasMissing(ctx, "target_track_identity") {
		t.Fatalf("ctx = %+v", ctx)
	}
}

func TestVocalForwardNoBandAvoidsBrightBoost(t *testing.T) {
	ctx := Build(Input{
		UserIntent: "make vocal forward but not too bright",
		TargetRef:  "track:vocal",
		ObservationSummary: map[string]any{
			"mix_package": map[string]any{
				"source_capabilities": map[string]any{"band_energy": "missing"},
			},
		},
	})
	if ctx.Recommendation.Strategy != "conservative_level_or_peer_rebalance" {
		t.Fatalf("recommendation = %+v", ctx.Recommendation)
	}
	all := strings.ToLower(ctx.Recommendation.Strategy + " " + ctx.Recommendation.NextStep)
	for _, fact := range ctx.InferredFacts {
		all += " " + strings.ToLower(fact.Summary)
	}
	if strings.Contains(all, "high_shelf_boost") || strings.Contains(all, "presence_boost") {
		t.Fatalf("unsafe bright boost leaked into context: %s", all)
	}
}

func TestPanClearTargetSmallPanAdjustDoesNotDependOnBand(t *testing.T) {
	ctx := Build(Input{
		UserIntent:    "把吉他稍微靠左一点",
		TargetRef:     "track:guitar",
		ActionKind:    "pan_balance",
		ProcessorType: "utility",
		ObservationSummary: map[string]any{
			"mix_package": map[string]any{
				"missing_metrics": []any{"band_energy_summary"},
			},
		},
	})
	if ctx.ProblemKind != "pan_balance" || ctx.Recommendation.Strategy != "small_pan_adjust" {
		t.Fatalf("ctx = %+v", ctx)
	}
	if hasMissing(ctx, "band_energy_summary") {
		t.Fatalf("pan should not make band energy blocking/missing in diagnosis: %+v", ctx.MissingEvidence)
	}
}

func TestStereoStatusObservationIsNotPanBalance(t *testing.T) {
	ctx := Build(Input{
		UserIntent: "read-only observation: check stereo status, phase, and correlation; no changes",
		TargetRef:  "project",
		ObservationSummary: map[string]any{
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"stereo_relation": map[string]any{
						"status":               "ready",
						"balance_state":        "centered",
						"correlation_estimate": 0.82,
						"correlation_state":    "stable",
					},
				},
			},
		},
	})
	if ctx.ProblemKind == "pan_balance" || ctx.Recommendation.Strategy == "small_pan_adjust" {
		t.Fatalf("stereo status observation became pan diagnosis: %+v", ctx)
	}
	if ctx.Recommendation.ActionKind != "observation_only" {
		t.Fatalf("recommendation = %+v", ctx.Recommendation)
	}
	if !hasObserved(ctx, "stereo_relation_summary") {
		t.Fatalf("stereo facts = %+v", ctx.ObservedFacts)
	}
}

func TestReadOnlyObservationWithPanWordsDoesNotAttachPanDiagnosis(t *testing.T) {
	ctx := Build(Input{
		UserIntent:    "analysis only: observe whether the guitar pan position is leaning left; do not modify",
		TargetRef:     "track:guitar",
		ActionKind:    "pan_balance",
		ProcessorType: "utility",
		ObservationSummary: map[string]any{
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"stereo_relation": map[string]any{"status": "ready", "balance_state": "left_heavy"},
				},
			},
		},
	})
	if ctx.ProblemKind == "pan_balance" || ctx.Recommendation.Strategy == "small_pan_adjust" {
		t.Fatalf("read-only pan-word observation became pan diagnosis: %+v", ctx)
	}
	if ctx.Recommendation.ActionKind != "observation_only" {
		t.Fatalf("recommendation = %+v", ctx.Recommendation)
	}
}

func hasMissing(ctx Context, key string) bool {
	for _, row := range ctx.MissingEvidence {
		if row.Key == key {
			return true
		}
	}
	return false
}

func hasObserved(ctx Context, key string) bool {
	for _, row := range ctx.ObservedFacts {
		if row.Key == key {
			return true
		}
	}
	return false
}

func containsRef(refs []string, needle string) bool {
	for _, ref := range refs {
		if strings.Contains(ref, needle) {
			return true
		}
	}
	return false
}
