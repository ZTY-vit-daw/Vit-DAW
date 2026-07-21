package executionverifiers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/orchestration"
)

type recordingHarnessInvoker struct {
	Request  harness.InvokeRequest
	Response harness.InvokeResponse
	Err      error
}

func (f *recordingHarnessInvoker) Invoke(_ context.Context, request harness.InvokeRequest) (harness.InvokeResponse, error) {
	f.Request = request
	return f.Response, f.Err
}

func TestHarnessAcousticRequiresFreshMOMObservation(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{
		Status: "ok",
		Result: map[string]any{
			"observation_id": "obs_after",
			"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
			"mom_projection": map[string]any{
				"intent": "project_multitrack_relation_observation",
				"multitrack_relation": map[string]any{
					"status":      "ready",
					"track_count": 2,
					"compared_tracks": []any{
						map[string]any{"track_id": "t1"},
						map[string]any{"track_id": "t2"},
					},
					"level_distribution": map[string]any{"status": "ready"},
					"evidence_refs":      []any{"MOM:relation", "DAD:track_rows"},
				},
			},
		},
	}}
	result, err := (HarnessAcoustic{
		Invoker: invoker, PreviousObservationID: "obs_before", MixSessionID: "session_1", GoalText: "balance",
	}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
	if err != nil || result.Status != "pass" || result.ObservationID != "obs_after" || result.ObservationRevision == "" || result.MOMStatus != "ready" {
		t.Fatalf("unexpected acoustic result=%#v err=%v", result, err)
	}
	args := invoker.Request.Args
	if args["scope"] != "full_project" || args["observation_only"] != true || args["previous_observation"] != "obs_before" || args["mom_intent"] != "project_multitrack_relation_observation" {
		t.Fatalf("unsafe or incomplete mix.observe request: %#v", args)
	}
	if invoker.Request.Tool != "mix.observe" || invoker.Request.Source != "capability_runtime_v1_verifier" {
		t.Fatalf("unexpected invocation: %#v", invoker.Request)
	}
}

func TestHarnessAcousticAcceptsTypedMOMProjectionFromInProcessHarness(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{
		Status: "ok",
		Result: map[string]any{
			"observation_id": "obs_after",
			"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
			"mom_projection": &mom.Projection{
				MOMVersion: mom.Version,
				Intent:     mom.IntentProjectMultitrackObservation,
				MultitrackRelation: mom.MultitrackRelation{
					Status: mom.StatusReady, TrackCount: 2,
					ComparedTracks:    []map[string]any{{"track_id": "t1"}, {"track_id": "t2"}},
					LevelDistribution: map[string]any{"status": "ready"},
					EvidenceRefs:      []string{"MOM:typed"},
				},
			},
		},
	}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
	if err != nil || result.Status != "pass" || result.MOMStatus != mom.StatusReady {
		t.Fatalf("typed projection must verify like its JSON form: result=%#v err=%v", result, err)
	}
}

func TestHarnessAcousticRejectsReusedObservation(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_before",
		"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
	}}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
	if err != nil || result.Status != "fail" {
		t.Fatalf("reused observation must fail: result=%#v err=%v", result, err)
	}
}

func TestHarnessAcousticKeepsUnavailableObservationInconclusive(t *testing.T) {
	invoker := &recordingHarnessInvoker{Err: errors.New("feature collector unavailable")}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
	if err != nil || result.Status != "inconclusive" {
		t.Fatalf("observation infrastructure failure must remain inconclusive: result=%#v err=%v", result, err)
	}
}

func TestHarnessAcousticPartialMOMIsInconclusive(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after",
		"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
		"mom_projection": map[string]any{
			"intent":              "project_multitrack_relation_observation",
			"multitrack_relation": map[string]any{"status": "partial"},
		},
	}}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
	if err != nil || result.Status != "inconclusive" || result.MOMStatus != "partial" {
		t.Fatalf("partial MOM must not claim pass: result=%#v err=%v", result, err)
	}
}

func TestHarnessAcousticB2AcceptsPartialMOMWhenLevelRelationshipIsReady(t *testing.T) {
	compared := make([]any, 0, 61)
	for index := 1; index <= 61; index++ {
		compared = append(compared, map[string]any{"track_id": fmt.Sprintf("track_%d", index)})
	}
	compared[0] = map[string]any{"track_id": "t1"}
	compared[1] = map[string]any{"track_id": "t2"}
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after",
		"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
		"mom_projection": map[string]any{
			"intent": mom.IntentProjectMultitrackObservation,
			"multitrack_relation": map[string]any{
				"status": "partial", "track_count": 61, "compared_tracks": compared,
				"level_distribution":  map[string]any{"status": "ready"},
				"stereo_distribution": map[string]any{"status": "missing"},
				"limitations":         []any{"some_tracks_missing_band_or_stereo_evidence"},
			},
		},
	}}}
	actionSet := orchestration.ActionSet{ID: "as1", Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1"}, {ID: "a2", TargetRef: "t2"}}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), actionSet)
	if err != nil || result.Status != "pass" || result.MOMStatus != mom.StatusPartial {
		t.Fatalf("B2 level-ready partial MOM should pass: result=%#v err=%v", result, err)
	}
	if result.Summary == "" {
		t.Fatal("B2 capability-specific verification summary is required")
	}
}

func TestHarnessAcousticB3StillRequiresStereoRelationship(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
		"observation_id": "obs_after",
		"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
		"mom_projection": map[string]any{
			"intent": mom.IntentProjectMultitrackObservation,
			"multitrack_relation": map[string]any{
				"status": "partial", "track_count": 2,
				"compared_tracks":     []any{map[string]any{"track_id": "t1"}, map[string]any{"track_id": "t2"}},
				"level_distribution":  map[string]any{"status": "ready"},
				"stereo_distribution": map[string]any{"status": "missing"},
			},
		},
	}}}
	actionSet := orchestration.ActionSet{ID: "as1", Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1"}}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyPanLayout(context.Background(), actionSet)
	if err != nil || result.Status != "inconclusive" {
		t.Fatalf("B3 must not borrow B2 level-only verification: result=%#v err=%v", result, err)
	}
}
