package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// FIX-CARRIES-REPORT-TIGHTEN-1: the carriesExperimentReport audit skip is
// grounded in loop.Experiment, not in the decision's own field echo. The 109
// rerun2 proposal turns copied the skeleton's settle placeholder triple
// {materiality:none,not_ready}+target_response+round_decision verbatim with no
// experiment admitted; the field-presence skip let that shape bypass the G1-G7
// admission audit and advance on an unbacked GatePassed. These tests pin the
// tightened semantics: placeholder fields with no admitted experiment are
// audited as the new admission they actually are, while the same fields on an
// admitted experiment keep the settle-report exemption.

// placeholderTripleGateRefusedFixture mirrors gateRefusedFreeStateFixture's
// no-experiment loop and G6/G8-refusing audit context, with the decision
// additionally carrying the settle placeholder triple.
func placeholderTripleGateRefusedFixture(t *testing.T, loopID, conversationID, goalID, runID string) (*Server, freeStateReasoningLoop, agentloop.Result) {
	t.Helper()
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         loopID,
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Status:         "observing",
		DecisionPhase:  freeStatePhaseProcessorSelection,
		OriginalIntent: "把人声轨的高频毛刺处理一下",
		ActiveIntent:   "把人声轨的高频毛刺处理一下",
		MaxCycles:      freeStateDefaultMaxCycles,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.storeFreeStateLoop(loop)
	auditContext := map[string]any{
		"task_contract":                  map[string]any{"project_uuid": "uuid-crrt", "project_revision": "3", "kind": "improvement"},
		"free_state_capacity_assessment": map[string]any{"capacity_level": "within_free_state", "selected_capability": "free_state"},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "uuid-crrt", "project_revision": "3",
			"diagnostic_rounds": []any{map[string]any{"evidence_status": "ready"}},
			"hypothesis_frontier": map[string]any{
				"candidates": []any{map[string]any{"id": "cand-crrt"}},
			},
		},
		"free_state_reasoning_loop": map[string]any{
			"observation_ledger": map[string]any{
				"receipts": []any{
					map[string]any{"observation_id": "obs-scan-crrt", "status": "ready", "project_revision": "3", "requested_views": []any{"mix.multitrack_relationship"}},
					map[string]any{"observation_id": "obs-202-crrt", "status": "ready", "project_revision": "3", "target_ref": map[string]any{"kind": "track", "id": "202"}},
				},
			},
		},
	}
	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID,
		Status:          agentruntime.StatusCompleted,
		StopReason:      agentloop.StopReasonDone,
		ContextSnapshot: auditContext,
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion:  agentloop.FreeStateDecisionSchema,
			Status:         agentloop.FreeStateNeedsExperiment,
			EvidenceStatus: "sufficient",
			Summary:        "proposal turn echoing the settle placeholder triple",
			ImprovementProposal: &agentprotocol.ImprovementProposal{
				SchemaVersion:     agentprotocol.ImprovementProposalSchema,
				Target:            map[string]any{"kind": "track", "id": "202"},
				EvidenceRefs:      []string{"obs-202-crrt"},
				ImprovementIntent: "bounded high-frequency smoothing on the target track",
				Hypothesis:        "a bounded attenuation improves the harshness",
				ExpectedEffect:    "A/B comparable candidate",
				ActionDomain:      agentprotocol.ImprovementActionDomainTrackGain,
				ActionKind:        "gain_adjust",
				ParameterBounds:   map[string]any{"delta_db": -1.5},
				Confidence:        0.5,
			},
			// The skeleton placeholder triple, copied verbatim with no
			// experiment admitted (109 rerun2 proposal-turn shape).
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityNone, Evaluation: trajectory.EvaluationNotReady,
				Attempt: 1, EvidenceRefs: []string{"obs-202-crrt"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationAmbiguous,
				Summary: "placeholder echo", EvidenceRefs: []string{"obs-202-crrt"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
	}
	return s, loop, res
}

// No admitted experiment + placeholder triple = no audit skip: the decision is
// audited as a new admission, and the same G6/G8 refusal that rejects the bare
// proposal must reject the placeholder-triple echo instead of admitting an
// experiment on an unbacked GatePassed.
func TestPlaceholderReportTripleWithoutExperimentIsAuditedAsNewAdmission(t *testing.T) {
	s, loop, res := placeholderTripleGateRefusedFixture(t, "loop-crrt-tighten", "conversation-crrt-tighten", "goal-crrt-tighten", "run-crrt-tighten")
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("placeholder-triple decision was not retained by recordFreeStateDecision")
	}
	if stored.Status != "capability_blocked" {
		t.Fatalf("placeholder triple without an experiment escaped the admission audit: status=%q experiment=%v", stored.Status, stored.Experiment != nil)
	}
	if stored.LatestDecision == nil || stored.LatestDecision.StopReason != "free_state_admission_gate_failed" {
		t.Fatalf("latest decision stop_reason = %+v, want free_state_admission_gate_failed", stored.LatestDecision)
	}
	if boundary := firstStringFromMap(stored.AdmissionReceipt, "boundary"); boundary != "admission_gate_failed" {
		t.Fatalf("admission receipt boundary = %q, want admission_gate_failed", boundary)
	}
	failed := gateReceiptFailedIDs(t, stored.AdmissionReceipt)
	if joined := strings.Join(failed, ","); !strings.Contains(joined, "G6_target_evidence") {
		t.Fatalf("placeholder triple changed the audit verdict face: failed_gate_ids=%q", joined)
	}
	if stored.Experiment != nil {
		t.Fatal("placeholder triple admitted an experiment through the refused gate")
	}
}

// Control: the same placeholder-carrying decision on a loop with an admitted
// experiment keeps the settle-report exemption — the audit must not re-audit
// an already-admitted experiment's report as a new admission (2026-08-25 D1
// smoke boundary).
func TestReportTripleOnAdmittedExperimentKeepsSettleExemption(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 1)
	s.storeFreeStateLoop(loop)
	_, _, res := placeholderTripleGateRefusedFixture(t, "loop-crrt-control", "conversation-crrt-control", "goal-crrt-control", "run-crrt-control")
	res.GoalID, res.RunID = loop.GoalID, loop.RunID
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("report decision was not retained by recordFreeStateDecision")
	}
	if strings.Contains(stored.LastError, "free_state_admission_gate_failed") {
		t.Fatalf("admitted experiment's report was re-audited as a new admission: %q", stored.LastError)
	}
	if stored.Status == "capability_blocked" {
		t.Fatalf("settle-report exemption lost on an admitted experiment: status=%q last_error=%q", stored.Status, stored.LastError)
	}
	if stored.Experiment == nil {
		t.Fatal("admitted experiment disappeared while recording its report")
	}
}
