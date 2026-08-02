package staticbalance

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/mixstyle"
)

type solverStrategy struct {
	ID          string
	Label       string
	Strength    float64
	MaxDeltaDB  float64
	Recommended bool
}

func Solve(model Model) Result {
	result := Result{
		SchemaVersion:      ResultSchemaVersion,
		ResultID:           stableID("sbr", model.ModelID, model.Style.ID),
		ModelID:            model.ModelID,
		CapabilityID:       CapabilityID,
		GeneratedAt:        model.GeneratedAt,
		ObservationID:      model.ObservationID,
		Style:              model.Style,
		Readiness:          model.Readiness,
		AnalyzedTrackCount: len(model.Tracks),
		EligibleTrackCount: model.Coverage.EligibleTrackCount,
		EvidenceRefs:       append([]string(nil), model.EvidenceRefs...),
		Limitations:        append([]string(nil), model.Limitations...),
	}
	if !model.Readiness.CanProceed {
		return result
	}
	strategies := []solverStrategy{
		{ID: "conservative", Label: "Conservative", Strength: 0.70, MaxDeltaDB: 1.25},
		{ID: "balanced", Label: "Balanced", Strength: 1.00, MaxDeltaDB: 2.00, Recommended: true},
		{ID: "expressive", Label: "Expressive", Strength: 1.20, MaxDeltaDB: 2.00},
	}
	for _, strategy := range strategies {
		candidate := solveCandidate(model, strategy)
		if len(candidate.Actions) > 0 {
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	if len(result.Candidates) == 0 {
		result.Limitations = uniqueStrings(append(result.Limitations, "no_material_fader_moves_after_bounded_solver"))
	}
	return result
}

func solveCandidate(model Model, strategy solverStrategy) CandidatePlan {
	eligible := make([]Track, 0, model.Coverage.ComparableCount)
	levelsByFunction := map[string][]float64{}
	for _, track := range model.Tracks {
		if !track.Eligible || track.FaderDB == nil || track.LevelDB == nil || track.Function == "" {
			continue
		}
		eligible = append(eligible, track)
		levelsByFunction[track.Function] = append(levelsByFunction[track.Function], *track.LevelDB)
	}
	candidate := CandidatePlan{
		CandidatePlanID: stableID("sbp", model.ModelID, model.Style.ID, strategy.ID),
		Strategy:        strategy.ID,
		Label:           strategy.Label,
		Recommended:     strategy.Recommended,
		Confidence:      solverConfidence(model),
		Strength:        strategy.Strength,
		MaxAbsDeltaDB:   strategy.MaxDeltaDB,
		Assumptions: []string{
			"The current project state, not prior capability history, satisfies the B2 readiness contract.",
			"Track-level effective static RMS evidence is comparable within this project observation.",
		},
		Limitations: append([]string(nil), model.Limitations...),
	}
	if len(eligible) < 2 {
		return candidate
	}
	functions := make([]string, 0, len(levelsByFunction))
	functionMedian := map[string]float64{}
	functionLevels := make([]float64, 0, len(levelsByFunction))
	for function, values := range levelsByFunction {
		functions = append(functions, function)
		functionMedian[function] = median(values)
		functionLevels = append(functionLevels, functionMedian[function])
	}
	sort.Strings(functions)
	projectFunctionMedian := median(functionLevels)
	rawByFunction := map[string]float64{}
	rawValues := make([]float64, 0, len(functions))
	for _, function := range functions {
		currentRelative := clamp(functionMedian[function]-projectFunctionMedian, -3, 3)
		targetRelative := functionalTarget(function, model.Style.Dimensions)
		response := clamp(0.55-0.20*model.Style.Dimensions.NaturalDynamicTolerance, 0.30, 0.75)
		rawByFunction[function] = (targetRelative - currentRelative*response) * strategy.Strength
		rawValues = append(rawValues, rawByFunction[function])
	}
	center := median(rawValues)
	for _, track := range eligible {
		delta := roundQuarter(clamp(rawByFunction[track.Function]-center, -strategy.MaxDeltaDB, strategy.MaxDeltaDB))
		if delta > 0 && track.HeadroomDB != nil {
			maxRaise := roundQuarter(math.Max(0, *track.HeadroomDB-1.0))
			if delta > maxRaise {
				delta = maxRaise
			}
		}
		if math.Abs(delta) < 0.25 {
			continue
		}
		before := *track.FaderDB
		candidate.Actions = append(candidate.Actions, Action{
			Operation: "track_gain_adjust",
			TrackID:   track.TrackID,
			TrackName: track.TrackName,
			Role:      track.Role,
			Function:  track.Function,
			DeltaDB:   delta,
			BeforeDB:  round3(before),
			TargetDB:  round3(before + delta),
			Reason:    fmt.Sprintf("place %s/%s in the %s relationship under %s", track.TrackName, track.Role, track.Function, model.Style.Name),
			Evidence: map[string]any{
				"role_source":                  track.RoleSource,
				"role_confidence":              track.RoleConfidence,
				"level_metric":                 track.LevelMetric,
				"level_source":                 track.LevelSource,
				"level_tap_point":              track.LevelTapPoint,
				"level_status":                 track.LevelStatus,
				"level_db":                     round3(*track.LevelDB),
				"function_median_level_db":     round3(functionMedian[track.Function]),
				"project_function_median_db":   round3(projectFunctionMedian),
				"function_current_relative_db": round3(clamp(functionMedian[track.Function]-projectFunctionMedian, -3, 3)),
				"function_target_relative_db":  round3(functionalTarget(track.Function, model.Style.Dimensions)),
				"style_id":                     model.Style.ID,
				"model_id":                     model.ModelID,
				"candidate_plan_id":            candidate.CandidatePlanID,
			},
		})
	}
	sort.SliceStable(candidate.Actions, func(i, j int) bool {
		ai, aj := math.Abs(candidate.Actions[i].DeltaDB), math.Abs(candidate.Actions[j].DeltaDB)
		if ai == aj {
			return candidate.Actions[i].TrackID < candidate.Actions[j].TrackID
		}
		return ai > aj
	})
	candidate.FunctionSummary = summarizeFunctions(candidate.Actions)
	return candidate
}

func solverConfidence(model Model) string {
	if model.Coverage.RoleCoverage >= 0.85 && model.Coverage.LevelCoverage >= 0.85 && model.Coverage.FunctionCount >= 3 {
		return "high"
	}
	return "medium"
}

func functionalTarget(function string, d mixstyle.Dimensions) float64 {
	switch function {
	case "foreground":
		return 0.90 + 0.80*d.ForegroundPriority
	case "rhythm_anchor":
		return 0.30 + 0.70*d.RhythmAnchorWeight + 0.20*d.TransientControlBias
	case "low_end_anchor":
		return 0.10 + 0.80*d.LowEndAnchorWeight
	case "harmonic_bed":
		return -0.50 + 0.60*d.HarmonicBedDepth - 0.30*d.DensityClearance
	case "support":
		return -1.00 - 0.65*d.SupportLayerDistance - 0.35*d.DensityClearance + 0.20*d.HarmonicBedDepth
	case "effects":
		return -1.40 - 0.50*d.SupportLayerDistance - 0.40*d.DensityClearance
	default:
		return 0
	}
}

func summarizeFunctions(actions []Action) []FunctionSummary {
	byFunction := map[string][]float64{}
	for _, action := range actions {
		function := strings.TrimSpace(action.Function)
		if function == "" {
			function = "unknown"
		}
		byFunction[function] = append(byFunction[function], action.DeltaDB)
	}
	functions := make([]string, 0, len(byFunction))
	for function := range byFunction {
		functions = append(functions, function)
	}
	sort.Strings(functions)
	out := make([]FunctionSummary, 0, len(functions))
	for _, function := range functions {
		values := byFunction[function]
		summary := FunctionSummary{Function: function, TrackCount: len(values), MoveCount: len(values), MinDeltaDB: values[0], MaxDeltaDB: values[0]}
		for _, value := range values {
			summary.MeanDeltaDB += value
			if value < summary.MinDeltaDB {
				summary.MinDeltaDB = value
			}
			if value > summary.MaxDeltaDB {
				summary.MaxDeltaDB = value
			}
		}
		summary.MeanDeltaDB = round3(summary.MeanDeltaDB / float64(len(values)))
		out = append(out, summary)
	}
	return out
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func roundQuarter(value float64) float64 { return math.Round(value*4) / 4 }
