package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/trajectory"
)

func experimentTestProposal() *agentprotocol.ImprovementProposal {
	return &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vocal"}, EvidenceRefs: []string{"obs-before"},
		ImprovementIntent: "make the vocal more forward", Hypothesis: "a bounded treatment may improve forwardness",
		ExpectedEffect: "forwardness without transient regression", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
		ActionKind: experiment.D1S1ActionKind, ParameterBounds: map[string]any{"delta_db": -1.0},
		VerificationPlan: map[string]any{"view_ids": []any{"track.timbre_frequency"}, "experiment_budget": 1}, Confidence: 0.6,
	}
}

func d1FreshObservationForTest(revision string) *agentloop.RecentObservation {
	return &agentloop.RecentObservation{Tool: "ccb.observation_request", ToolCallID: "call-before", Summary: map[string]any{
		"status": "ready", "observation_id": "obs-before", "requested_views": []any{"track.timbre_frequency"},
		"actual_executed_view_ids": []any{"track.timbre_frequency"}, "evidence_refs": []any{"obs-before"},
		"target_ref": map[string]any{"kind": "track", "id": "vocal"}, "project_binding": map[string]any{"project_revision": revision},
		"audit_receipt": map[string]any{"receipt_id": "receipt-before", "view_set_matches": true, "actual_executed_view_ids": []any{"track.timbre_frequency"}, "freshness": map[string]any{"status": "fresh"}},
	}}
}

func TestD1FreshObservedTargetAcceptsCurrentObservationClass(t *testing.T) {
	observation := d1FreshObservationForTest("7")
	observation.Summary["freshness"] = map[string]any{"status": "ready", "class": "current_observation", "project_revision": "7"}
	observation.Summary["audit_receipt"].(map[string]any)["freshness"] = map[string]any{"status": "ready", "class": "current_observation", "project_revision": "7"}
	loop := freeStateReasoningLoop{LatestObservation: observation}
	proposal := experimentTestProposal()
	if _, err := validateD1FreshObservedTarget(loop, experiment.Admission{TargetRef: proposal.Target, EvidenceRefs: []string{"obs-before"}}); err != nil {
		t.Fatalf("current_observation freshness class was rejected: %v", err)
	}
}

func TestFreeStateObservationProjectRevisionUsesFreshnessBinding(t *testing.T) {
	got := freeStateObservationProjectRevision(map[string]any{
		"freshness": map[string]any{"class": "current_observation", "project_revision": "rev-9"},
	})
	if got != "rev-9" {
		t.Fatalf("project revision = %q, want rev-9", got)
	}
}

func TestPostActionObservationRejectsReplayedPreActionBundle(t *testing.T) {
	now := time.Now().UTC()
	proposal := experimentTestProposal()
	admission, err := freeStateExperimentAdmission(freeStateReasoningLoop{LoopID: "loop-post", LatestObservation: d1FreshObservationForTest("15")}, proposal)
	if err != nil {
		t.Fatal(err)
	}
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "conv-post", GoalID: "goal-post", RunID: "run-post", TurnID: "turn-post"}, "post action", admission, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint", "15", now); err != nil {
		t.Fatal(err)
	}
	loop := freeStateReasoningLoop{Experiment: &turn, LatestProjectChange: map[string]any{"to_project": map[string]any{"project_revision": "16"}}}
	old := d1FreshObservationForTest("15")
	if freeStatePostActionObservationEligible(loop, old) {
		t.Fatal("replayed pre-action observation was accepted as post-action evidence")
	}
}

func TestFreeStateNeedsExperimentCreatesRuntimeTurnAndTrajectory(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-experiment", ConversationID: "conversation-experiment",
		GoalID: "goal-1", RunID: "run-1", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "make the vocal more forward", ActiveIntent: "make the vocal more forward", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	loop, ok := s.recordFreeStateDecision("conversation-experiment", agentloop.Result{GoalID: "goal-1", RunID: "run-1", FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "bounded hypothesis", ImprovementProposal: experimentTestProposal(), RequestedViewIDs: []string{"track.timbre_frequency"},
	}})
	if !ok || loop.Experiment == nil {
		t.Fatalf("experiment runtime not created: ok=%v loop=%+v", ok, loop)
	}
	if loop.Experiment.SchemaVersion != experiment.SchemaVersion || loop.Experiment.ID == "" {
		t.Fatalf("bad turn=%+v", loop.Experiment)
	}
	if loop.Experiment.CurrentRoundID == "" || len(loop.Experiment.Rounds) != 1 {
		t.Fatalf("round not started=%+v", loop.Experiment)
	}
	events, _ := s.agentEventsSince("conversation-experiment", 0, 100)
	foundTurn, foundIntent, foundHypothesis, foundRound := false, false, false, false
	for _, event := range events {
		switch event.Type {
		case string(trajectory.EventTurnStarted):
			foundTurn = true
		case string(trajectory.EventIntentFramed):
			foundIntent = true
		case string(trajectory.EventHypothesisProposed):
			foundHypothesis = true
		case string(trajectory.EventRoundStarted):
			foundRound = true
		}
		if event.Type == string(trajectory.EventRoundStarted) && event.Payload["schema_version"] != trajectory.SchemaVersion {
			t.Fatalf("wrong trajectory schema=%+v", event.Payload)
		}
	}
	if !foundTurn || !foundIntent || !foundHypothesis || !foundRound {
		t.Fatalf("missing trajectory events: %+v", events)
	}
}

func TestFreeStateExperimentMaterialityAndTargetResponseAreRecordedFromDecision(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-record", ConversationID: "conversation-record", Status: "awaiting_experiment", OriginalIntent: "improve", LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now}
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, "goal", "run"); err != nil {
		t.Fatal(err)
	}
	if loop.Experiment == nil {
		t.Fatal("missing experiment")
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "before", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, ProjectRevision: "rev-before", EvidenceRefs: []string{"before"}}, false, now); err != nil {
		t.Fatal(err)
	}
	s.recordFreeStateExperimentAction(&loop, "eq", "applied", map[string]any{"action_id": "action-1", "status": "readback_ok"})
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"subthreshold"}}})
	round, _ := loop.Experiment.CurrentRoundID, loop.Experiment.Rounds
	if round == "" || loop.Experiment.Rounds[0].Materiality == nil || loop.Experiment.Rounds[0].Materiality.Evaluation != trajectory.EvaluationInsufficientDose {
		t.Fatalf("materiality missing=%+v", loop.Experiment)
	}
	if loop.Experiment.Rounds[0].Decision != "" {
		t.Fatalf("D1-S1 subthreshold result must remain available for ambiguous human audition: %+v", loop.Experiment.Rounds[0])
	}
	if len(loop.Experiment.Rounds) != 1 {
		t.Fatalf("D1-S1 insufficient dose created another round: %+v", loop.Experiment.Rounds)
	}
}
