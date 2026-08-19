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
		ExpectedEffect: "forwardness without transient regression", ActionDomain: agentprotocol.ImprovementActionDomainEQ,
		ActionKind: "bounded_tonal_adjustment", ParameterBounds: map[string]any{"max_db": 2},
		VerificationPlan: map[string]any{"view_ids": []any{"track.timbre_frequency"}}, Confidence: 0.6,
	}
}

func TestFreeStateNeedsExperimentCreatesRuntimeTurnAndTrajectory(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-experiment", ConversationID: "conversation-experiment",
		GoalID: "goal-1", RunID: "run-1", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "make the vocal more forward", ActiveIntent: "make the vocal more forward", MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", ToolCallID: "call-before", Summary: map[string]any{
			"status": "ready", "observation_id": "obs-before", "requested_views": []any{"track.timbre_frequency"},
			"actual_executed_view_ids": []any{"track.timbre_frequency"}, "evidence_refs": []any{"obs-before"},
			"audit_receipt": map[string]any{"receipt_id": "receipt-before", "view_set_matches": true, "actual_executed_view_ids": []any{"track.timbre_frequency"}},
		}}, CreatedAt: now, UpdatedAt: now,
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
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-record", ConversationID: "conversation-record", Status: "awaiting_experiment", OriginalIntent: "improve", CreatedAt: now, UpdatedAt: now}
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, "goal", "run"); err != nil {
		t.Fatal(err)
	}
	if loop.Experiment == nil {
		t.Fatal("missing experiment")
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "before", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, EvidenceRefs: []string{"before"}}, false, now); err != nil {
		t.Fatal(err)
	}
	s.recordFreeStateExperimentAction(&loop, "eq", "applied", map[string]any{"action_id": "action-1", "status": "readback_ok"})
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"subthreshold"}}})
	round, _ := loop.Experiment.CurrentRoundID, loop.Experiment.Rounds
	if round == "" || loop.Experiment.Rounds[0].Materiality == nil || loop.Experiment.Rounds[0].Materiality.Evaluation != trajectory.EvaluationInsufficientDose {
		t.Fatalf("materiality missing=%+v", loop.Experiment)
	}
	if loop.Experiment.Rounds[0].Status != experiment.RoundDoseCalibrating {
		t.Fatalf("wrong dose status=%+v", loop.Experiment.Rounds[0])
	}
}
