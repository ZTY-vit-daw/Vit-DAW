package executionverifiers

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type sequenceHarnessInvoker struct {
	Requests  []harness.InvokeRequest
	Responses []harness.InvokeResponse
}

func (f *sequenceHarnessInvoker) Invoke(_ context.Context, request harness.InvokeRequest) (harness.InvokeResponse, error) {
	f.Requests = append(f.Requests, request)
	index := len(f.Requests) - 1
	if index >= len(f.Responses) {
		return harness.InvokeResponse{}, fmt.Errorf("unexpected invocation %d", index+1)
	}
	return f.Responses[index], nil
}

func TestHarnessAcousticRequiresFreshMOMObservation(t *testing.T) {
	invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{
		Status: "ok",
		Result: map[string]any{
			"observation_id": "obs_after",
			"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
			"mom_projection": map[string]any{
				"intent": "project_multitrack_relation_observation",
				"static_level_relationship": verifierStaticRelation("ready", []any{
					verifierStaticTrack("t1", -20), verifierStaticTrack("t2", -22),
				}),
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
				StaticLevelRelationship: &mom.StaticLevelRelationship{
					SchemaVersion: mom.StaticLevelRelationshipSchema, Status: mom.StatusReady, Freshness: "fresh",
					Coverage: map[string]any{"track_count": 2},
					Tracks: []mom.StaticLevelTrack{
						{TrackID: "t1", Status: mom.StatusReady, Freshness: "fresh", Metric: "effective_static_rms_dbfs", TapPoint: "derived_static_control_model", EffectiveStaticRMSDBFS: verifierFloatPtr(-20)},
						{TrackID: "t2", Status: mom.StatusReady, Freshness: "fresh", Metric: "effective_static_rms_dbfs", TapPoint: "derived_static_control_model", EffectiveStaticRMSDBFS: verifierFloatPtr(-22)},
					},
					EvidenceRefs: []string{"MOM:typed"},
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

func TestHarnessAcousticUntrustedStaticMOMIsInconclusive(t *testing.T) {
	for _, status := range []string{"partial", "stale", "suspect", "missing"} {
		t.Run(status, func(t *testing.T) {
			invoker := &recordingHarnessInvoker{Response: harness.InvokeResponse{Status: "ok", Result: map[string]any{
				"observation_id": "obs_after_" + status,
				"observation":    map[string]any{"created_at": "2026-07-13T12:00:00Z"},
				"mom_projection": map[string]any{
					"intent":                    "project_multitrack_relation_observation",
					"static_level_relationship": map[string]any{"status": status},
				},
			}}}
			result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), orchestration.ActionSet{ID: "as1"})
			if err != nil || result.Status != "inconclusive" || result.MOMStatus != status {
				t.Fatalf("%s MOM must not claim pass or fail: result=%#v err=%v", status, result, err)
			}
		})
	}
}

func TestHarnessAcousticB2UsesReadyStaticProjectionWhenGenericMOMIsPartial(t *testing.T) {
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
			"static_level_relationship": verifierStaticRelation("ready", compared),
		},
	}}}
	actionSet := orchestration.ActionSet{ID: "as1", Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1"}, {ID: "a2", TargetRef: "t2"}}}
	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), actionSet)
	if err != nil || result.Status != "pass" || result.MOMStatus != mom.StatusReady {
		t.Fatalf("B2 ready static projection should pass independently of generic MOM: result=%#v err=%v", result, err)
	}
	if result.Summary == "" {
		t.Fatal("B2 capability-specific verification summary is required")
	}
}

func TestHarnessAcousticB2RetriesTransientHierarchyContradiction(t *testing.T) {
	actionSet := orchestration.ActionSet{ID: "as1", Actions: []orchestration.Action{
		{ID: "rhythm", TargetRef: "1017", Args: verifierStaticActionArgs("rhythm_anchor", -44.301, 0.5)},
		{ID: "foreground", TargetRef: "vocal", Args: verifierStaticActionArgs("foreground", -20.0, 1.0)},
	}}
	observation := func(id, createdAt string, rhythmLevel float64) harness.InvokeResponse {
		return harness.InvokeResponse{Status: "ok", Result: map[string]any{
			"observation_id": id,
			"observation":    map[string]any{"created_at": createdAt},
			"mom_projection": map[string]any{
				"intent": mom.IntentProjectMultitrackObservation,
				"static_level_relationship": verifierStaticRelation("ready", []any{
					verifierStaticTrack("1017", rhythmLevel), verifierStaticTrack("vocal", -19.0),
				}),
			},
		}}
	}
	invoker := &sequenceHarnessInvoker{Responses: []harness.InvokeResponse{
		observation("obs_transient", "2026-08-01T13:01:47Z", -39.794),
		observation("obs_stable", "2026-08-01T13:01:49Z", -43.801),
	}}

	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), actionSet)
	if err != nil || result.Status != "pass" || result.RelationshipStatus != "pass" || result.ObservationID != "obs_stable" {
		t.Fatalf("transient hierarchy contradiction must be retried: result=%#v err=%v", result, err)
	}
	if len(invoker.Requests) != 2 || invoker.Requests[1].Args["previous_observation"] != "obs_transient" {
		t.Fatalf("retry must require an independently fresh observation: %#v", invoker.Requests)
	}
	if !strings.Contains(result.Summary, "not reproduced") {
		t.Fatalf("retry outcome must remain auditable: %q", result.Summary)
	}
}

func TestHarnessAcousticB2RejectsRepeatedHierarchyContradiction(t *testing.T) {
	actionSet := orchestration.ActionSet{ID: "as1", Actions: []orchestration.Action{
		{ID: "foreground", TargetRef: "vocal", Args: verifierStaticActionArgs("foreground", -20.0, 1.0)},
		{ID: "support", TargetRef: "backing", Args: verifierStaticActionArgs("support", -20.0, -1.0)},
	}}
	observation := func(id, createdAt string) harness.InvokeResponse {
		return harness.InvokeResponse{Status: "ok", Result: map[string]any{
			"observation_id": id,
			"observation":    map[string]any{"created_at": createdAt},
			"mom_projection": map[string]any{
				"intent": mom.IntentProjectMultitrackObservation,
				"static_level_relationship": verifierStaticRelation("ready", []any{
					verifierStaticTrack("vocal", -21.0), verifierStaticTrack("backing", -19.0),
				}),
			},
		}}
	}
	invoker := &sequenceHarnessInvoker{Responses: []harness.InvokeResponse{
		observation("obs_opposite_1", "2026-08-01T13:01:47Z"),
		observation("obs_opposite_2", "2026-08-01T13:01:49Z"),
	}}

	result, err := (HarnessAcoustic{Invoker: invoker, PreviousObservationID: "obs_before"}).VerifyStaticBalance(context.Background(), actionSet)
	if err != nil || result.Status != "fail" || result.RelationshipStatus != "fail" || len(invoker.Requests) != 2 {
		t.Fatalf("repeated hierarchy contradiction must still fail: result=%#v requests=%d err=%v", result, len(invoker.Requests), err)
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

func TestVerifyStaticBalanceHierarchyPassesExactCandidateDirection(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{
		{ID: "foreground", TargetRef: "vocal", Args: verifierStaticActionArgs("foreground", -20.0, 1.0)},
		{ID: "support", TargetRef: "backing", Args: verifierStaticActionArgs("support", -20.0, -1.0)},
	}}
	relation := map[string]any{"tracks": []any{
		verifierStaticTrack("vocal", -19.0),
		verifierStaticTrack("backing", -21.0),
	}}
	status, summary := verifyStaticBalanceHierarchy(relation, actionSet)
	if status != "pass" || summary == "" {
		t.Fatalf("candidate hierarchy status=%q summary=%q", status, summary)
	}
}

func TestVerifyStaticBalanceHierarchyRejectsOppositeCandidateDirection(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{
		{ID: "foreground", TargetRef: "vocal", Args: verifierStaticActionArgs("foreground", -20.0, 1.0)},
		{ID: "support", TargetRef: "backing", Args: verifierStaticActionArgs("support", -20.0, -1.0)},
	}}
	relation := map[string]any{"tracks": []any{
		verifierStaticTrack("vocal", -21.0),
		verifierStaticTrack("backing", -19.0),
	}}
	status, _ := verifyStaticBalanceHierarchy(relation, actionSet)
	if status != "fail" {
		t.Fatalf("opposite hierarchy status=%q, want fail", status)
	}
}

func TestVerifyStaticBalanceHierarchyKeepsMissingPostEvidenceInconclusive(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{
		ID: "foreground", TargetRef: "vocal", Args: verifierStaticActionArgs("foreground", -20.0, 1.0),
	}}}
	status, _ := verifyStaticBalanceHierarchy(map[string]any{"tracks": []any{}}, actionSet)
	if status != "inconclusive" {
		t.Fatalf("missing post evidence status=%q, want inconclusive", status)
	}
}

func TestVerifyStaticBalanceHierarchyDoesNotCompareDifferentMetricTap(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{
		ID: "track", TargetRef: "1007", Args: map[string]any{
			"hierarchy_function": "rhythm_anchor", "before_effective_level_db": -55.489, "delta_db": 2.0,
			"before_effective_level_metric": "source_waveform_rms_dbfs", "before_effective_level_tap_point": "source_file_pre_fx",
		},
	}}}
	relation := map[string]any{"tracks": []any{verifierStaticTrack("1007", -42.301)}}
	status, summary := verifyStaticBalanceHierarchy(relation, actionSet)
	if status != "inconclusive" || !strings.Contains(summary, "metric/tap changed") {
		t.Fatalf("cross-metric +13.188 dB sample must not become a failure: status=%q summary=%q", status, summary)
	}
}

func verifierStaticRelation(status string, tracks []any) map[string]any {
	normalized := make([]any, 0, len(tracks))
	for _, value := range tracks {
		row, _ := value.(map[string]any)
		if len(row) == 0 {
			continue
		}
		if _, ok := row["effective_static_rms_dbfs"]; !ok {
			row = map[string]any{
				"track_id": row["track_id"], "status": "ready", "freshness": "fresh",
				"metric": "effective_static_rms_dbfs", "tap_point": "derived_static_control_model",
				"effective_static_rms_dbfs": -20.0,
			}
		}
		normalized = append(normalized, row)
	}
	return map[string]any{
		"schema_version": mom.StaticLevelRelationshipSchema, "status": status, "freshness": "fresh",
		"coverage": map[string]any{"track_count": len(normalized), "usable_track_count": len(normalized)},
		"tracks":   normalized, "evidence_refs": []any{"MOM:static_level_relationship"},
	}
}

func verifierStaticTrack(trackID string, level float64) map[string]any {
	return map[string]any{
		"track_id": trackID, "status": "ready", "freshness": "fresh",
		"metric": "effective_static_rms_dbfs", "tap_point": "derived_static_control_model",
		"effective_static_rms_dbfs": level,
	}
}

func verifierStaticActionArgs(function string, before, delta float64) map[string]any {
	return map[string]any{
		"hierarchy_function": function, "before_effective_level_db": before, "delta_db": delta,
		"before_effective_level_metric":    "effective_static_rms_dbfs",
		"before_effective_level_tap_point": "derived_static_control_model",
	}
}

func verifierFloatPtr(value float64) *float64 { return &value }
