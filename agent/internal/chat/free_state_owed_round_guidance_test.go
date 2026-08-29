package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// D2-2-S3h2 (20260829_205921 trace): the settle report the model emitted on the
// freshly opened round-2 was refused (the round never acted, so no post-action
// observation can exist on it), but every refusal surface answered "wait until
// the fresh post-action observation is recorded" — a debt that can never
// discharge on a round that owes its intervention. The model read the guidance,
// made zero tool calls for two turns, replayed the settle report on every nudge
// and burnt the continuation budget. The refusal copy must split by round type:
// a round that never acted hears the executable pointer (this round owes one
// bounded intervention proposal; the fresh base is obs@rev, cite it), while a
// round that acted and truly owes settlement keeps the observation guidance.

// owedRoundSettleReplayResult builds the misdirected settle turn's envelope:
// the boundary-signal pair riding the preserved proposal, the 205921 turn-7
// shape arriving on a round that never acted.
func owedRoundSettleReplayResult(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3h2", SliceID: "slice-s3h2", TurnID: "turn-s3h2",
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:              agentloop.FreeStateNeedsExperiment,
			Summary:             "settle report replayed on the round that never acted",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationAgentEvaluable,
				Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3h2", SliceID: "slice-s3h2", TurnID: "turn-s3h2",
			OriginalIntent: loop.OriginalIntent,
			Context:        map[string]any{"free_state_reasoning_loop": map[string]any{"status": "observing"}},
		},
	}
}

// driveOwedRoundRefusal opens the recalibration round (driveNoDifferenceRecalibration:
// round-1 acted and was judged, round-2 freshly opened with its S3h1 fresh base
// obs-d2-action-1@8), replays the misdirected settle report through the full
// decision ingest (refusal + boundary-pair strip + stored-decision copy-back,
// mirroring goalrunner_chat.go), and returns the envelope the response builder
// sees — the 205921 post-turn-7 state.
func driveOwedRoundRefusal(t *testing.T, s *Server, loop *freeStateReasoningLoop) agentloop.Result {
	t.Helper()
	driveNoDifferenceRecalibration(t, s, loop)
	res := owedRoundSettleReplayResult(*loop)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("misdirected settle turn was not retained by recordFreeStateDecision")
	}
	if stored.SettleRefusedRoundID == "" {
		t.Fatal("misdirected settle report on the never-acted round was not refused")
	}
	if !freeStateLoopRoundNeverActed(stored) {
		t.Fatal("fixture does not describe a never-acted round: the copy split cannot be exercised")
	}
	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	return res
}

// The refusal answer for a never-acted round must carry the executable pointer:
// the round owes one bounded intervention proposal and the fresh base reference
// is named — never "wait for the observation".
func TestRefusedSettleOnNeverActedRoundGuidesToInterventionProposal(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res := driveOwedRoundRefusal(t, s, &loop)

	resp := s.improvementProposalResponse(loop.ConversationID, "default", res, map[string]any{})
	if resp.NeedsConfirmation || len(resp.InteractionRequests) != 0 {
		t.Fatalf("refused settle replay surfaced a confirmation: needs=%v requests=%d", resp.NeedsConfirmation, len(resp.InteractionRequests))
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("refused settle replay answered %q, want waiting_continue", resp.GoalStatus)
	}
	if resp.StopReason != "round_owes_intervention_proposal" {
		t.Fatalf("never-acted round refusal kept the awaiting-observation classification: stop=%q", resp.StopReason)
	}
	if !strings.Contains(resp.Reply, "欠一次有界干预提案") {
		t.Fatalf("refusal reply does not state the owed intervention proposal: %q", resp.Reply)
	}
	if !strings.Contains(resp.Reply, "obs-d2-action-1@8") {
		t.Fatalf("refusal reply does not name the fresh base reference: %q", resp.Reply)
	}
	if data := resp.WorkflowData; data["round_owes_intervention"] != true || data["round_base_reference"] != "obs-d2-action-1@8" {
		t.Fatalf("refusal workflow data lost the owed-round facts: %+v", data)
	}
}

// The guidance must be truthful: a bare proposal from the never-acted round is
// the owed work itself, so the settle-refused swallow may not eat it — it must
// flow into the recalibration routing (S3c pathway) instead of answering
// waiting_continue with "no further changes will be proposed".
func TestNeverActedRoundBareProposalRoutesPastSettleRefusal(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	proposal := driveOwedRoundRefusal(t, s, &loop)
	proposal.FreeStateDecision.ExperimentMateriality = nil
	proposal.FreeStateDecision.ExperimentRoundDecision = ""
	resp := s.improvementProposalResponse(loop.ConversationID, "default", proposal, map[string]any{})
	if resp.WorkflowData["recalibration_proposal_routed"] != true {
		t.Fatalf("never-acted round's bare proposal was swallowed by the settle refusal: status=%q stop=%q reply=%q",
			resp.GoalStatus, resp.StopReason, resp.Reply)
	}
	if resp.WorkflowData["round_owes_intervention_proposal"] == true {
		t.Fatalf("bare proposal was answered with the settle-refusal guidance: %+v", resp.WorkflowData)
	}
}

// Control (real judgment boundary intact): a round that acted and truly owes
// settlement keeps the observation guidance unchanged — the split must not
// weaken the settle-race refusal semantics.
func TestActedRoundSettleRefusalKeepsObservationGuidance(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	loop.SettleRefusedRoundID = round.ID
	loop.RequiresPostActionObservation = true
	s.storeFreeStateLoop(loop)

	resp := s.improvementProposalResponse(loop.ConversationID, "default", settleRaceTurn(loop), map[string]any{})
	if resp.StopReason != "settle_report_refused_awaiting_observation" {
		t.Fatalf("acted round's refusal lost the awaiting-observation classification: stop=%q", resp.StopReason)
	}
	if !strings.Contains(resp.Reply, "尚未入账") {
		t.Fatalf("acted round's refusal reply changed: %q", resp.Reply)
	}
	if strings.Contains(resp.Reply, "干预提案") {
		t.Fatalf("acted round's refusal reply drifted into owed-proposal guidance: %q", resp.Reply)
	}
}

// The refused settle's completed-residue demotion (S3g) keeps its mechanism on
// a never-acted round but carries the owed-intervention classification — the
// envelope must not re-teach "awaiting observation" to a round that owes a
// proposal.
func TestRefusedSettleResidueOnNeverActedRoundCarriesOwedClassification(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res := driveOwedRoundRefusal(t, s, &loop)

	resp := s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("completed residue was not demoted on the never-acted round: %q", resp.GoalStatus)
	}
	if resp.StopReason != "round_owes_intervention_proposal" {
		t.Fatalf("never-acted round's demoted residue kept the awaiting-observation stop reason: %q", resp.StopReason)
	}
	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 || continuations[0].Status != ContinuationPending {
		t.Fatalf("owed-round retry checkpoint lost schedulability: %+v", continuations)
	}
}
