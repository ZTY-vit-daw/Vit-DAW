package lowendrelation

import (
	"testing"
	"time"
)

func TestTargetPostFXTrackIDsIncludesTreatmentTargetsAndReferencedPeers(t *testing.T) {
	plan := TreatmentPlan{Targets: []TreatmentTarget{{TrackID: "bass", RelationshipRefs: []string{"conflict-low"}}}}
	model := Model{Conflicts: []LowEndConflict{
		{ID: "conflict-low", Tracks: []map[string]any{{"track_id": "kick"}, {"track_id": "bass"}}},
		{ID: "unrelated", Tracks: []map[string]any{{"track_id": "synth"}}},
	}}
	ids := TargetPostFXTrackIDs(plan, model)
	if stableID("ids", ids) != stableID("ids", []string{"bass", "kick"}) {
		t.Fatalf("B4 post-FX scope=%v", ids)
	}
}

func TestVerifyTargetPostFXBaselinesRequiresFreshObservedChange(t *testing.T) {
	beforeRows := []map[string]any{{"status": "ready", "track_id": "bass", "tap_point": "track_post_fader", "render_revision": "r1", "track_state_fingerprint": "s1", "evidence_ref": "probe:bass:1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .4}}}}
	afterRows := []map[string]any{{"status": "ready", "track_id": "bass", "tap_point": "track_post_fader", "render_revision": "r2", "track_state_fingerprint": "s2", "evidence_ref": "probe:bass:2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .2}}}}
	before := BuildTargetPostFXBaseline("b4", "before", "project", []string{"bass"}, beforeRows, time.Unix(1, 0))
	after := BuildTargetPostFXBaseline("b4", "after", "project", []string{"bass"}, afterRows, time.Unix(2, 0))
	verification := VerifyTargetPostFXBaselines(before, after)
	if !verification.Comparable || verification.Status != "observed_change" || verification.UserAcceptance != "unknown" || len(verification.ChangedDimensions) == 0 {
		t.Fatalf("B4 post-FX verification=%#v", verification)
	}
	after.Tracks[0].Bands = before.Tracks[0].Bands
	after.Tracks[0].Metrics = before.Tracks[0].Metrics
	after.BaselineID = stableID("b4fx", map[string]any{"forced": "fresh-without-acoustic-change"})
	unchanged := VerifyTargetPostFXBaselines(before, after)
	if unchanged.Status != "inconclusive" || len(unchanged.Reasons) == 0 {
		t.Fatalf("unchanged B4 evidence passed: %#v", unchanged)
	}
}
