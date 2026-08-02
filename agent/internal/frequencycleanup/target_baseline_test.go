package frequencycleanup

import (
	"testing"
	"time"
)

func TestTargetPreflightTrackIDsIncludesOnlyStaticTargetsAndRelationshipPeers(t *testing.T) {
	model := Model{Candidates: []DiagnosisCandidate{
		{ID: "c1", TrackIDs: []string{"kick", "bass"}},
		{ID: "c2", TrackIDs: []string{"vocal", "guitar"}},
	}}
	plan := TreatmentPlan{Items: []TreatmentItem{
		{TrackID: "bass", Classification: TreatmentStaticEQ},
		{TrackID: "vocal", Classification: TreatmentNoChange},
	}}
	ids := TargetPreflightTrackIDs(plan, model)
	if stableID("ids", ids) != stableID("ids", []string{"bass", "kick"}) {
		t.Fatalf("target preflight scope = %#v", ids)
	}
}

func TestTargetPreflightTrackIDsForFourTargetsExcludesUnrelatedProjectTracks(t *testing.T) {
	model := Model{Tracks: []TrackProfile{{TrackID: "kick"}, {TrackID: "bass"}, {TrackID: "vocal"}, {TrackID: "guitar"}, {TrackID: "keys"}, {TrackID: "room"}, {TrackID: "fx"}}, Candidates: []DiagnosisCandidate{
		{ID: "low", TrackIDs: []string{"kick", "bass"}},
		{ID: "mid", TrackIDs: []string{"vocal", "guitar", "keys"}},
		{ID: "unrelated", TrackIDs: []string{"room", "fx"}},
	}}
	plan := TreatmentPlan{Items: []TreatmentItem{
		{TrackID: "kick", Classification: TreatmentStaticEQ},
		{TrackID: "bass", Classification: TreatmentStaticEQ},
		{TrackID: "vocal", Classification: TreatmentStaticEQ},
		{TrackID: "guitar", Classification: TreatmentStaticEQ},
		{TrackID: "keys", Classification: TreatmentNoChange},
		{TrackID: "room", Classification: TreatmentNoChange},
		{TrackID: "fx", Classification: TreatmentDeferredSpace},
	}}
	ids := TargetPreflightTrackIDs(plan, model)
	want := []string{"bass", "guitar", "keys", "kick", "vocal"}
	if stableID("ids", ids) != stableID("ids", want) {
		t.Fatalf("four-target preflight scope = %#v want %#v", ids, want)
	}
}

func TestTargetPostFXBaselineRequiresExactCurrentRevisionSameTapCoverage(t *testing.T) {
	rows := []map[string]any{
		{"status": "ready", "track_id": "bass", "tap_point": "track_post_fader", "render_revision": "r1", "track_state_fingerprint": "s1", "evidence_ref": "probe:bass:1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}},
		{"status": "ready", "track_id": "kick", "tap_point": "track_post_fader", "render_revision": "r2", "track_state_fingerprint": "s2", "evidence_ref": "probe:kick:1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .4}}},
	}
	baseline := BuildTargetPostFXBaseline("c1", "preflight", "project", []string{"bass", "kick"}, rows, time.Unix(1, 0))
	if !baseline.Readiness.CanProceed || len(baseline.Tracks) != 2 || baseline.TapPoint != "track_post_fader" {
		t.Fatalf("ready target baseline = %#v", baseline)
	}
	rows[1]["track_state_fingerprint"] = ""
	missing := BuildTargetPostFXBaseline("c1", "preflight", "project", []string{"bass", "kick"}, rows, time.Unix(2, 0))
	if missing.Readiness.CanProceed {
		t.Fatalf("baseline without current track revision became ready: %#v", missing)
	}
}

func TestVerifyTargetPostFXBaselinesUsesExactTargetScopeAndNeverClaimsAcceptance(t *testing.T) {
	beforeRows := []map[string]any{{"status": "ready", "track_id": "bass", "tap_point": "track_post_fader", "render_revision": "r1", "track_state_fingerprint": "s1", "evidence_ref": "probe:bass:1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}}}
	afterRows := []map[string]any{{"status": "ready", "track_id": "bass", "tap_point": "track_post_fader", "render_revision": "r2", "track_state_fingerprint": "s2", "evidence_ref": "probe:bass:2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .2}}}}
	before := BuildTargetPostFXBaseline("c1", "before", "project", []string{"bass"}, beforeRows, time.Unix(1, 0))
	after := BuildTargetPostFXBaseline("c1", "after", "project", []string{"bass"}, afterRows, time.Unix(2, 0))
	verification := VerifyTargetPostFXBaselines(before, after)
	if !verification.Comparable || verification.Status != "observed" || verification.UserAcceptance != "unknown" || len(verification.TrackIDs) != 1 {
		t.Fatalf("target verification = %#v", verification)
	}
}
