package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
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
	s.recordFreeStateExperimentAction(&loop, "eq", "applied", map[string]any{"action_id": "action-1", "status": "readback_ok", "after_revision": "rev-after"})
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "rev-after", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
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

// Experiment report fields must reach the runtime regardless of the
// decision's status: capability_blocked + materiality + user_judgment_pending
// is the documented ambiguous human-judgment settlement. Ingesting reports
// only under needs_experiment lost exactly that record (2026-08-25 D1 smoke).
func TestExperimentReportFieldsIngestedOnTerminalDecision(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-boundary", ConversationID: "conversation-boundary",
		GoalID: "goal-b", RunID: "run-b", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "improve the low-end balance", ActiveIntent: "improve the low-end balance", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	if loop, ok := s.recordFreeStateDecision("conversation-boundary", agentloop.Result{GoalID: "goal-b", RunID: "run-b", FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "bounded hypothesis", ImprovementProposal: experimentTestProposal(), RequestedViewIDs: []string{"track.timbre_frequency"},
	}}); !ok || loop.Experiment == nil {
		t.Fatalf("experiment runtime not created: ok=%v", ok)
	} else if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-boundary", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: map[string]any{"action_id": "d1-boundary", "status": "applied", "transaction_id": "tx-b", "idempotency_key": "key-b", "readback_verified": true, "after_revision": "8"}}, now); err != nil {
		t.Fatalf("apply intervention: %v", err)
	} else if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "obs-after", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"obs-after"}}, true, now); err != nil {
		t.Fatalf("record post-action observation: %v", err)
	} else {
		s.storeFreeStateLoop(loop)
	}
	loop, ok := s.recordFreeStateDecision("conversation-boundary", agentloop.Result{GoalID: "goal-b", RunID: "run-b", FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateCapabilityBlocked, EvidenceStatus: "insufficient",
		Summary: "ambiguous; awaiting human A/B judgment",
		ExperimentMateriality:      &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-after"}},
		ExperimentRoundDecision:    string(experiment.DecisionUserJudgment),
	}})
	if !ok {
		t.Fatal("boundary decision was not recorded")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Materiality == nil {
		t.Fatalf("materiality on a terminal decision never reached the experiment runtime: %+v", round)
	}
	if round.Decision != experiment.DecisionUserJudgment {
		t.Fatalf("round decision = %q, want user_judgment_pending", round.Decision)
	}
}
// The human-judgment boundary is terminal for model turns. Once the round
// decision user_judgment_pending is recorded (or a judgment is requested /
// recorded), a needs_observation/needs_action/new-admission decision must not
// revive the loop into repeated post-action observation cycles (2026-08-25
// 21:09 D1 smoke: the revived loop recorded a second post_action=true
// observation and burned the remaining continuation budget).
func TestJudgmentBoundaryRefusesLoopRevival(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := d1LoopForTest(t, "7")
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "before_revision": "7", "after_revision": "8", "applied_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "readback_verified": true}
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-action", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: receipt}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", ReceiptID: "receipt-after", RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
	// Park the round at the durable human-judgment boundary. The loop is
	// deliberately still presented as active (the exact state the 21:09
	// revival bug hit: awaiting_experiment / observing with the round decided
	// user_judgment_pending); the guard must refuse the revival and park it.
	if _, err := loop.Experiment.DecideRound(experiment.DecisionUserJudgment, "awaiting human A/B judgment", now); err != nil {
		t.Fatal(err)
	}
	loop.Status = "awaiting_experiment"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.LatestProjectChange = map[string]any{"to_project": map[string]any{"project_revision": "8"}}
	s.storeFreeStateLoop(loop)

	// A revival decision must leave the loop parked and record nothing new.
	revived, ok := s.recordFreeStateDecision(loop.ConversationID, agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "need more post-action evidence", RequestedViewIDs: []string{"track.basic_energy"},
	}})
	if !ok {
		t.Fatal("loop was not retained")
	}
	if revived.Status != "blocked" || !strings.Contains(revived.LastError, "human judgment boundary") {
		t.Fatalf("loop revived at the judgment boundary: status=%s last_error=%q", revived.Status, revived.LastError)
	}
	round, err := revived.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	// The round must retain exactly the pre-action + one post-action bundles;
	// the boundary turn must not record another observation.
	if len(round.Observations) != 2 {
		t.Fatalf("boundary turn recorded a new observation: %+v", round.Observations)
	}
	postActionCount := 0
	for _, observation := range round.Observations {
		if observation.PostAction {
			postActionCount++
		}
	}
	if postActionCount != 1 {
		t.Fatalf("boundary turn changed the post-action observation count: %+v", round.Observations)
	}

}

// The settle-family round decision (retained / rolled_back / stopped) is the
// one model output the judgment boundary still admits: it must pass the
// boundary guard and reach the experiment runtime instead of being refused
// like the revival statuses.
func TestJudgmentBoundaryAdmitsSettleDecision(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := d1LoopForTest(t, "7")
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "before_revision": "7", "after_revision": "8", "applied_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "readback_verified": true}
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-action", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: receipt}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", ReceiptID: "receipt-after", RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"material-after"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetSufficient, Outcome: trajectory.EvaluationAgentEvaluable, EvidenceRefs: []string{"target-after"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.DecideRound(experiment.DecisionUserJudgment, "awaiting human A/B judgment", now); err != nil {
		t.Fatal(err)
	}
	loop.Status = "awaiting_experiment"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.LatestProjectChange = map[string]any{"to_project": map[string]any{"project_revision": "8"}}
	s.storeFreeStateLoop(loop)

	settled, ok := s.recordFreeStateDecision(loop.ConversationID, agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateCapabilityBlocked,
		EvidenceStatus: "insufficient", Summary: "user retained the treatment",
		ExperimentRoundDecision: string(experiment.DecisionRetain),
	}})
	if !ok {
		t.Fatal("settle decision was not recorded")
	}
	if strings.Contains(settled.LastError, "human judgment boundary") || settled.Status == "observing" || settled.Status == "awaiting_experiment" {
		t.Fatalf("settle decision was mis-handled at the boundary: status=%s last_error=%q", settled.Status, settled.LastError)
	}
	round, err := settled.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Decision != experiment.DecisionRetain {
		t.Fatalf("settle round decision = %q, want retained", round.Decision)
	}
}

// An FS8 evaluation report (needs_experiment carrying materiality +
// target_response + round_decision) must not be re-audited by the G1-G7 gate:
// the gate re-checks pre-apply state that legitimately changed after Apply, so
// auditing the report converted the evaluation into capability_blocked and the
// human-judgment boundary never became durable (2026-08-25 21:09 D1 smoke).
// Mirrors the agentloop output-gate skip for report-carrying decisions.
func TestFS8EvaluationReportSkipsGateAuditAndParksAtJudgmentBoundary(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := d1LoopForTest(t, "7")
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "before_revision": "7", "after_revision": "8", "applied_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "readback_verified": true}
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-action", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: receipt}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", ReceiptID: "receipt-after", RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
	loop.Status = "awaiting_experiment"
	loop.LatestProjectChange = map[string]any{"to_project": map[string]any{"project_revision": "8"}}
	s.storeFreeStateLoop(loop)

	// The continuation context carries a post-apply closure without frontier /
	// target evidence so the G1-G7 audit would fail G5/G6 if it ran.
	result := agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, Continuation: &agentloop.Continuation{Context: map[string]any{
		"minimal_audio_closure": map[string]any{"project_uuid": "project-1", "project_revision": "8", "hypothesis_frontier": map[string]any{}},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "awaiting_experiment",
			"original_intent": "improve the low-end balance",
		},
	}}, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "post-action evaluation: awaiting human A/B judgment",
		ImprovementProposal: experimentTestProposal(),
		ExperimentMateriality:    &experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"material-after"}},
		ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"target-after"}},
		ExperimentRoundDecision:  string(experiment.DecisionUserJudgment),
	}}
	recorded, ok := s.recordFreeStateDecision(loop.ConversationID, result)
	if !ok {
		t.Fatal("evaluation decision was not recorded")
	}
	if strings.Contains(recorded.LastError, "free_state_admission_gate_failed") {
		t.Fatalf("FS8 evaluation report was re-audited and rejected: %+v", recorded.LastError)
	}
	round, err := recorded.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Materiality == nil || round.TargetResponse == nil {
		t.Fatalf("evaluation report never reached the experiment runtime: %+v", round)
	}
	if round.Decision != experiment.DecisionUserJudgment {
		t.Fatalf("round decision = %q, want user_judgment_pending", round.Decision)
	}
	if recorded.Status != "blocked" {
		t.Fatalf("loop status = %q, want blocked at the judgment boundary", recorded.Status)
	}
}

// Re-answering a stale interaction (for example the already-executed mix-tick
// confirmation) while the round is parked at the human-judgment boundary must
// not revive the loop into another observation/action cycle (2026-08-25 21:09
// D1 smoke: the re-answered mix tick revived the loop, which recorded a second
// post-action observation).
func TestInteractionResumeRefusedAtJudgmentBoundary(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := d1LoopForTest(t, "7")
	receipt := map[string]any{"action_id": "d1-action", "status": "applied", "before_revision": "7", "after_revision": "8", "applied_revision": "8", "transaction_id": "tx-d1", "idempotency_key": "key-d1", "readback_verified": true}
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{ID: "d1-action", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: receipt}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{ID: "after", ReceiptID: "receipt-after", RequestedViewIDs: []string{"mix.multitrack_relationship"}, ExecutedViewIDs: []string{"mix.multitrack_relationship"}, ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"after"}}, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.DecideRound(experiment.DecisionUserJudgment, "awaiting human A/B judgment", now); err != nil {
		t.Fatal(err)
	}
	// The loop is still presented as active (the state the revival bug hit);
	// the boundary guard must refuse the resume before any cycle accounting.
	loop.Status = "awaiting_experiment"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	s.storeFreeStateLoop(loop)

	interaction := PendingInteraction{ID: "interaction-stale-mix-tick", ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, Kind: "mix_tick_confirmation", Workflow: "mix_tick"}
	resp := ChatResponse{ConversationID: loop.ConversationID, GoalStatus: string(agentruntime.StatusCompleted), Workflow: "mix_tick", WorkflowData: map[string]any{"mutation_performed": true}}
	got := s.maybeContinueFreeStateAfterInteraction(context.Background(), interaction, resp, "approve")
	if got.Workflow != "mix_tick" || got.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("interaction resume changed the boundary response: %+v", got)
	}
	stored, _ := s.freeStateLoop(loop.ConversationID)
	if stored.Status != "awaiting_experiment" || stored.Cycle != 0 || len(stored.Actions) != 0 {
		t.Fatalf("interaction resume revived the loop at the judgment boundary: status=%s cycle=%d actions=%d", stored.Status, stored.Cycle, len(stored.Actions))
	}
}


