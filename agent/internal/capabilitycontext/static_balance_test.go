package capabilitycontext

import (
	"fmt"
	"testing"
	"time"

	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/staticbalance"
)

func TestBuildStaticBalancePackAndMultiTrackPlan(t *testing.T) {
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent:    "按现代流行做 B2 静态平衡",
		GeneratedAt:   time.Unix(1700000000, 0),
		Style:         mustTestMixStyle(t, "modern_pop"),
		StyleExplicit: true,
		ProjectState: map[string]any{"tracks": []any{
			map[string]any{"track_id": "v", "track_name": "Lead Vocal", "volume_db": 0.0},
			map[string]any{"track_id": "d", "track_name": "Drums", "volume_db": 0.0},
			map[string]any{"track_id": "b", "track_name": "Bass", "volume_db": 0.0},
			map[string]any{"track_id": "s", "track_name": "Strings", "volume_db": 0.0},
		}},
		ContextSnapshot: map[string]any{"tom_projection": map[string]any{
			"tom_version": "v0.2",
			"group_proposals": []any{
				map[string]any{"role_hypothesis": "lead_vocal", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "v"}}},
				map[string]any{"role_hypothesis": "drums_or_percussion", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "d"}}},
				map[string]any{"role_hypothesis": "bass", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "b"}}},
				map[string]any{"role_hypothesis": "strings", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "s"}}},
			},
		}},
		MixObservation: map[string]any{"observation_id": "obs_b2", "mom_projection": map[string]any{
			"mom_version":   "v1.4",
			"trust_quality": map[string]any{"can_support_action_preflight": true},
			"multitrack_relation": map[string]any{"compared_tracks": []any{
				map[string]any{"track_id": "v", "rms_dbfs": -18.0, "headroom_db": 8.0},
				map[string]any{"track_id": "d", "rms_dbfs": -18.0, "headroom_db": 6.0},
				map[string]any{"track_id": "b", "rms_dbfs": -18.0, "headroom_db": 7.0},
				map[string]any{"track_id": "s", "rms_dbfs": -18.0, "headroom_db": 9.0},
			}},
		}},
	})
	if pack.SchemaVersion != StaticBalanceContextPackSchema || pack.ContextManifestID != "static_mix.static_balance.context_manifest.v1" || pack.ContextBuilder != CapabilityContextBuilderVersion || pack.Style.ID != "modern_pop" || len(pack.Tracks) != 4 {
		t.Fatalf("pack = %+v", pack)
	}
	actions := recommendedStaticBalanceActions(pack)
	if len(actions) < 2 {
		t.Fatalf("solver actions = %+v, limitations=%v", actions, pack.Limitations)
	}
	for _, action := range actions {
		if action.Operation != "track_gain_adjust" || action.DeltaDB == 0 || action.DeltaDB < -2 || action.DeltaDB > 2 {
			t.Fatalf("unsafe action: %+v", action)
		}
	}
}

func TestStaticBalanceDisclosureCapNeverCapsAnalysisOrPlan(t *testing.T) {
	const trackCount = 61
	tracks := make([]any, 0, trackCount)
	levels := make([]any, 0, trackCount)
	roles := []string{"lead_vocal", "drums", "bass", "strings"}
	assignments := make([][]any, len(roles))
	for index := 0; index < trackCount; index++ {
		id := fmt.Sprintf("t_%02d", index+1)
		tracks = append(tracks, map[string]any{"track_id": id, "track_name": id, "track_type": "audio", "volume_db": 0.0})
		levels = append(levels, map[string]any{"track_id": id, "rms_dbfs": -24.0 + float64(index%9), "peak_dbfs": -8.0})
		assignments[index%len(roles)] = append(assignments[index%len(roles)], map[string]any{"track_id": id, "confidence": "high"})
	}
	groups := make([]any, len(roles))
	for index, role := range roles {
		groups[index] = map[string]any{"role_hypothesis": role, "confidence": "high", "assignments": assignments[index]}
	}
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent: "B2 static balance", GeneratedAt: time.Unix(1700000000, 0), Style: mustTestMixStyle(t, "modern_pop"),
		Budget:          Budget{MaxDisclosedTracks: 7, MaxRankingRows: 3},
		ProjectState:    map[string]any{"tracks": tracks},
		ContextSnapshot: map[string]any{"tom_projection": map[string]any{"full_assignment_manifest": map[string]any{"track_count": trackCount, "groups": groups}}},
		MixObservation: map[string]any{"observation_id": "obs_61", "mom_projection": map[string]any{
			"trust_quality":       map[string]any{"can_support_action_preflight": true},
			"multitrack_relation": map[string]any{"track_count": trackCount, "compared_tracks": levels},
		}},
	})
	if pack.AnalyzedTrackCount != trackCount || pack.DisclosedTrackCount != 7 || len(pack.Tracks) != 7 {
		t.Fatalf("analysis/disclosure counts = %d/%d tracks=%d", pack.AnalyzedTrackCount, pack.DisclosedTrackCount, len(pack.Tracks))
	}
	actions := recommendedStaticBalanceActions(pack)
	if len(actions) <= len(pack.Tracks) {
		t.Fatalf("full solver plan was capped by disclosure: actions=%d disclosed=%d", len(actions), len(pack.Tracks))
	}
	if len(pack.Candidates) != 3 || len(pack.Candidates[0].ActionExamples) > 3 {
		t.Fatalf("candidate disclosure = %+v", pack.Candidates)
	}
}

func TestStaticBalancePlanBlocksUntrustedMOM(t *testing.T) {
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent: "B2 static balance", GeneratedAt: time.Unix(1700000000, 0), Style: mixstyle.Default(),
		ProjectState: map[string]any{"tracks": []any{
			map[string]any{"track_id": "v", "track_name": "Vocal", "volume_db": 0.0, "rms_dbfs": -18.0},
			map[string]any{"track_id": "d", "track_name": "Drums", "volume_db": 0.0, "rms_dbfs": -18.0},
		}},
		MixObservation: map[string]any{"mom_projection": map[string]any{"trust_quality": map[string]any{"can_support_action_preflight": false}}},
	})
	if candidates := pack.Result().Candidates; len(candidates) != 0 {
		t.Fatalf("untrusted MOM produced candidates: %+v", candidates)
	}
}

func TestStaticBalancePlanBlocksLowConfidenceRoleCoverage(t *testing.T) {
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent: "B2 static balance", GeneratedAt: time.Unix(1700000000, 0), Style: mixstyle.Default(),
		ProjectState: map[string]any{"tracks": []any{
			map[string]any{"track_id": "v", "track_name": "Track 1", "volume_db": 0.0},
			map[string]any{"track_id": "d", "track_name": "Track 2", "volume_db": 0.0},
		}},
		ContextSnapshot: map[string]any{"tom_projection": map[string]any{"group_proposals": []any{
			map[string]any{"role_hypothesis": "lead_vocal", "confidence": "low", "assignment_excerpt": []any{map[string]any{"track_id": "v"}}},
			map[string]any{"role_hypothesis": "drums", "confidence": "low", "assignment_excerpt": []any{map[string]any{"track_id": "d"}}},
		}}},
		MixObservation: map[string]any{"mom_projection": map[string]any{
			"trust_quality": map[string]any{"can_support_action_preflight": true},
			"multitrack_relation": map[string]any{"compared_tracks": []any{
				map[string]any{"track_id": "v", "rms_dbfs": -18.0}, map[string]any{"track_id": "d", "rms_dbfs": -18.0},
			}},
		}},
	})
	if candidates := pack.Result().Candidates; len(candidates) != 0 {
		t.Fatalf("low-confidence roles produced candidates: %+v", candidates)
	}
}

func recommendedStaticBalanceActions(pack StaticBalancePack) []staticbalance.Action {
	for _, candidate := range pack.Result().Candidates {
		if candidate.Recommended {
			return candidate.Actions
		}
	}
	return nil
}

func mustTestMixStyle(t *testing.T, id string) mixstyle.MixStyle {
	t.Helper()
	style, err := mixstyle.Builtin(id)
	if err != nil {
		t.Fatal(err)
	}
	return style
}
