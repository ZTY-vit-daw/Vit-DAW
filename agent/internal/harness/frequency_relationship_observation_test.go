package harness

import (
	"testing"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/mom"
)

func TestFrequencyRelationshipIntentCanonicalizesToFreshFullProjectRead(t *testing.T) {
	cmd := map[string]any{"mom_intent": mom.IntentProjectFrequencyObservation, "observation_only": true}
	if !mixObservationScopeNeedsFreshProjectState(cmd) {
		t.Fatal("frequency relationship intent must refresh full project state")
	}
	canonical := canonicalizeMixObservationCommand(cmd, mixboard.TargetRef{}, nil)
	if firstString(canonical, "scope") != "full_project" {
		t.Fatalf("scope = %q command=%#v", firstString(canonical, "scope"), canonical)
	}
	target := mapAnyFromAny(canonical["target_ref"])
	if firstString(target, "kind") != "project" || firstString(target, "id") != "current" {
		t.Fatalf("project target = %#v", target)
	}
	listen := mapAnyFromAny(canonical["listen_scope"])
	if firstString(mapAnyFromAny(listen["source"]), "mode") != "full_project" || firstString(mapAnyFromAny(listen["time"]), "mode") != "full_song" {
		t.Fatalf("listen scope = %#v", listen)
	}
	if required := mixObservationReadyGateRequiredFeatures(mom.IntentProjectFrequencyObservation); len(required) != 0 {
		t.Fatalf("project relation projection must report partial coverage instead of blocking on one target gate: %v", required)
	}
}

func TestC1PerTrackObservationReusesExistingL2RenderProbeTrigger(t *testing.T) {
	cmd := map[string]any{"mom_intent": mom.IntentRealtimeBandStereoObservation, "observation_only": true, "track_id": "track-1", "tap_point": "track_post_fader"}
	if !mixObservationShouldRequestL2RenderProbe(cmd) {
		t.Fatal("C1 per-track baseline did not enter the existing L2 Render Probe path")
	}
	canonical := canonicalizeMixObservationCommand(cmd, mixboard.TargetRef{Kind: "track", ID: "track-1"}, map[string]any{"track_id": "track-1"})
	if firstString(canonical, "track_id") != "track-1" || firstString(canonical, "tap_point") != "track_post_fader" {
		t.Fatalf("C1 L2 identity/tap changed: %+v", canonical)
	}
}
