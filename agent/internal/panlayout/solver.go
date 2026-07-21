package panlayout

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type strategy struct {
	ID, Label           string
	Strength, MaxAbsPan float64
	Recommended         bool
}

func Solve(model Model) Result {
	result := Result{SchemaVersion: ResultSchemaVersion, ResultID: stableID("plr", map[string]any{"model": model.ModelID, "style": model.Style.Hash}), ModelID: model.ModelID, CapabilityID: CapabilityID, GeneratedAt: model.GeneratedAt, ObservationID: model.ObservationID, Style: model.Style, Readiness: model.Readiness, AnalyzedTrackCount: len(model.Tracks), EligibleTrackCount: model.Coverage.EligibleTrackCount, EvidenceRefs: append([]string(nil), model.EvidenceRefs...), Limitations: append([]string(nil), model.Limitations...)}
	if !model.Readiness.CanProceed {
		return result
	}
	for _, s := range []strategy{{"conservative", "Conservative", .68, .55, false}, {"balanced", "Style Target", 1, .78, true}, {"expressive", "Wide", 1.14, .90, false}} {
		candidate := solveCandidate(model, s)
		if len(candidate.Actions) > 0 {
			result.Candidates = append(result.Candidates, candidate)
		}
	}
	if len(result.Candidates) == 0 {
		result.Limitations = unique(append(result.Limitations, "no_material_pan_moves_after_bounded_solver"))
	}
	return result
}

func solveCandidate(model Model, s strategy) CandidatePlan {
	c := CandidatePlan{CandidatePlanID: stableID("plp", map[string]any{"model": model.ModelID, "style": model.Style.Hash, "strategy": s.ID}), Strategy: s.ID, Label: s.Label, Recommended: s.Recommended, Confidence: confidence(model), Strength: s.Strength, MaxAbsPan: s.MaxAbsPan, Assumptions: []string{"Current project pan/routing state satisfies B3 readiness.", "Stereo width is not modified; stereo/mono evidence only constrains track-pan actions."}, Limitations: append([]string(nil), model.Limitations...)}
	groups := map[string][]Track{}
	for _, t := range model.Tracks {
		if t.Eligible && t.CurrentPan != nil {
			if t.CenterAnchor {
				target := 0.0
				c.Actions = appendAction(c.Actions, model, t, target, s, "center anchor prior")
			}
			if !t.CenterAnchor && t.PairKey != "" {
				groups[t.PairKey] = append(groups[t.PairKey], t)
			}
		}
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		if len(group) < 2 {
			continue
		}
		sort.SliceStable(group, func(i, j int) bool { return group[i].TrackID < group[j].TrackID })
		base := roleSpread(group[0].Role, model.Style.Dimensions) * model.Style.Dimensions.SpreadAmount * s.Strength
		base = clamp(base, .12, s.MaxAbsPan)
		for i, t := range group {
			sign := -1.0
			if i%2 == 1 {
				sign = 1
			}
			rank := float64(i / 2)
			target := sign * clamp(base+.08*rank, .1, s.MaxAbsPan)
			if t.SourceFormat == "stereo" {
				target *= clamp(1-.55*model.Style.Dimensions.StereoSourceConservatism, .35, .85)
			} else if t.SourceFormat == "unknown" {
				target *= .55
			}
			target += model.Style.Dimensions.SymmetryBias * .08
			c.Actions = appendAction(c.Actions, model, t, target, s, "complementary role/pair layout")
		}
	}
	sort.SliceStable(c.Actions, func(i, j int) bool {
		ai, aj := math.Abs(c.Actions[i].DeltaPan), math.Abs(c.Actions[j].DeltaPan)
		if ai == aj {
			return c.Actions[i].TrackID < c.Actions[j].TrackID
		}
		return ai > aj
	})
	c.GroupSummary = summarizeGroups(c.Actions)
	return c
}

func appendAction(actions []Action, model Model, t Track, target float64, s strategy, reason string) []Action {
	target = roundPan(clamp(target, -s.MaxAbsPan, s.MaxAbsPan))
	before := roundPan(*t.CurrentPan)
	delta := roundPan(target - before)
	if math.Abs(delta) < .05 {
		return actions
	}
	return append(actions, Action{Operation: "track_pan_set", TrackID: t.TrackID, TrackName: t.TrackName, Role: t.Role, Function: t.Function, BeforePan: before, TargetPan: target, DeltaPan: delta, Reason: fmt.Sprintf("%s under %s (%s)", reason, model.Style.Name, s.Label), Evidence: map[string]any{"role_source": t.RoleSource, "role_confidence": t.RoleConfidence, "source_format": t.SourceFormat, "correlation": t.Correlation, "pair_key": t.PairKey, "style_id": model.Style.ID, "style_hash": model.Style.Hash, "model_id": model.ModelID}})
}

func roleSpread(role string, d interface{}) float64 {
	_ = d
	switch strings.ToLower(role) {
	case "backing_vocal", "harmony_vocal", "double_vocal":
		return .70
	case "electric_guitar", "acoustic_guitar", "guitar":
		return .78
	case "percussion", "hi_hat", "shaker":
		return .56
	case "strings", "orchestra", "brass":
		return .68
	case "synth", "keys", "piano", "pad":
		return .58
	case "fx", "effects":
		return .72
	default:
		return .52
	}
}

func confidence(model Model) string {
	if model.Coverage.RoleCoverage >= .85 && model.Coverage.PanCoverage >= .95 && model.Coverage.PairEligibleTrackCount >= 4 {
		return "high"
	}
	return "medium"
}
func summarizeGroups(actions []Action) []GroupSummary {
	by := map[string][]float64{}
	for _, a := range actions {
		role := a.Role
		if role == "" {
			role = "unknown"
		}
		by[role] = append(by[role], a.TargetPan)
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []GroupSummary{}
	for _, k := range keys {
		v := by[k]
		s := GroupSummary{Role: k, TrackCount: len(v), MoveCount: len(v), MinPan: v[0], MaxPan: v[0]}
		for _, x := range v {
			if x < s.MinPan {
				s.MinPan = x
			}
			if x > s.MaxPan {
				s.MaxPan = x
			}
		}
		out = append(out, s)
	}
	return out
}
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func roundPan(v float64) float64 { return math.Round(v*100) / 100 }
