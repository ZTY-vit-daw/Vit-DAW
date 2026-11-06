package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D1-DISCLOSURE-FORENSICS-1 regression (verifyrun2 form): an external writer
// advanced the authoritative project revision mid-request (a concurrent PCA
// certification's guard-track add/delete churned 2→3→4) while the closure
// kept its request-entry revision 2. Recording the then-fresh observation hit
// the cross-revision guard and settled the closure dead with
// project_revision_stale. A strictly newer kernel-minted observation revision
// must heal the tracked revision first (the request-entry revalidation
// semantics applied at the drift's detection point), so recording fresh
// post-drift evidence continues the closure instead of killing it.
func externalDriftHealServer(t *testing.T) (*Server, audioclosure.State) {
	t.Helper()
	server := &Server{
		harness:          harness.NewWithSender(nil, nil, nil),
		audioClosures:    audioclosure.NewMemoryStore(),
		freeStateLoops:   map[string]freeStateReasoningLoop{},
		capabilityRoutes: map[string]CapabilityRouteRecord{},
	}
	started, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "audio_closure_external_drift_heal", ConversationID: "conversation-external-drift-heal",
		GoalID: "goal-external-drift-heal", RunID: "run-external-drift-heal",
		ProjectUUID: "project-external-drift-heal", ProjectRevision: "2",
		OriginalIntent: "inspect the project", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-external-drift-heal"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("closure start failed: %v", err)
	}
	admitted, _, err := (audioclosure.Driver{}).AdmitRound(started, started.Revision, time.Now().UTC())
	if err != nil {
		t.Fatalf("round admit failed: %v", err)
	}
	if err := server.audioClosures.Create(admitted); err != nil {
		t.Fatalf("closure create failed: %v", err)
	}
	return server, admitted
}

func externalDriftResult(revision string) agentloop.Result {
	return agentloop.Result{
		GoalID: "goal-external-drift-heal", RunID: "run-external-drift-heal", Status: agentruntime.StatusCompleted,
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "tool_call_id": "call-external-drift-1", "status": "ok",
			"result": map[string]any{"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready",
				"observation_id": "obs-external-drift-fresh", "project_revision": revision,
				"requested_views": []any{"track.stereo_space"},
				"views": map[string]any{"track.stereo_space": map[string]any{
					"view_id": "track.stereo_space", "status": "ready", "facts": map[string]any{},
				}},
			}},
		}},
	}
}

func TestExternalDriftObservationHealsClosureInsteadOfSettling(t *testing.T) {
	server, state := externalDriftHealServer(t)

	recorded, err := server.recordAudioClosureRound(state, externalDriftResult("4"), map[string]any{})

	if err != nil {
		t.Fatalf("the post-drift recording failed the run: %v", err)
	}
	if recorded.Terminal() {
		t.Fatalf("fresh post-drift evidence machine-settled the closure: %+v", recorded.Settlement)
	}
	if recorded.Settlement != nil && recorded.Settlement.Reason == audioclosure.StopProjectRevisionStale {
		t.Fatalf("settlement must not be project_revision_stale: %+v", recorded.Settlement)
	}
	if recorded.ProjectRevision != "4" {
		t.Fatalf("tracked revision = %q, want the healed 4", recorded.ProjectRevision)
	}
	if !audioClosureHasObservationID(recorded, "obs-external-drift-fresh") {
		t.Fatal("the fresh post-drift observation must be booked into the healed closure")
	}
	healedEvent := false
	for _, event := range recorded.Events {
		if event.Type == audioclosure.EventProjectRevisionChanged &&
			strings.Contains(string(event.Data), `"4"`) {
			healedEvent = true
		}
	}
	if !healedEvent {
		t.Fatalf("the heal must append an auditable project_revision_changed event: %+v", recorded.Events)
	}
}

func TestExternalDriftHealPredicateGuards(t *testing.T) {
	base := audioclosure.State{ProjectRevision: "2"}
	cases := []struct {
		name      string
		state     audioclosure.State
		revision  string
		wantHeals bool
	}{
		{"newer numeric heals", base, "4", true},
		{"older numeric never heals", base, "1", false},
		{"equal never heals", base, "2", false},
		{"empty never heals", base, "", false},
		{"non-numeric never heals", base, "rev-4", false},
		{"superseded never heals", audioclosure.State{
			ProjectRevision: "2", SupersededProjectRevisions: map[string]bool{"4": true},
		}, "4", false},
		{"active capability pins", audioclosure.State{
			ProjectRevision: "2", ActiveCapability: &audioclosure.CapabilityLink{SessionID: "cap-1"},
		}, "4", false},
	}
	for _, tc := range cases {
		if got := audioClosureObservationRevisionHeals(tc.state, tc.revision); got != tc.wantHeals {
			t.Fatalf("%s: heals = %v, want %v", tc.name, got, tc.wantHeals)
		}
	}
}
