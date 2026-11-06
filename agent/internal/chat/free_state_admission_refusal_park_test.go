package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
)

// EXPADMIT-EVIDENCE-RULE-1 paired tests. The 262 verifyrun1 shape: the model
// produced a structurally valid improvement proposal whose evidence_refs cited
// an earlier observation of the SAME target at the SAME revision (basic_energy
// — the acoustically most relevant evidence for a level proposal) while the
// loop's LatestObservation had just moved on to a later view of that target
// (time_dynamics). The chat-side D1-S1 admission validator pinned the citation
// to the LatestObservation's own id, refused the proposal, and the rejection
// branch rewrote the loop blocked/terminal — the response face settled
// terminal-done and the turn-close guard closed the honest proposal as an
// orphan without ever building its confirmation card.
//
// Fix ② relaxes the citation SELECTION rule (any same-revision observation of
// the same target establishes the binding; G7's revision-bound freshness
// semantics, borrowed not reinvented — the agentloop gate itself is
// untouched). Fix ① parks the residual refusal on the confirmation face
// (terminalAdjudicationParking family) instead of terminalizing it.

// expAdmitEarlierObservationLedger builds a durable observation ledger whose
// rows describe an earlier observation of the given target at the given
// revision, in both receipt and available_views shapes.
func expAdmitEarlierObservationLedger(observationID, revision string, target map[string]any) map[string]any {
	row := map[string]any{
		"receipt_id": "receipt-" + observationID, "status": "ready",
		"observation_id": observationID, "requested_views": []any{"track.basic_energy"},
		"target_ref": target, "evidence_refs": []any{observationID},
		"project_revision": revision,
		"freshness":       map[string]any{"status": "fresh", "class": "current_observation", "project_revision": revision},
	}
	view := cloneContext(row)
	view["view_id"] = "track.basic_energy"
	targetID := firstStringFromMap(target, "id", "track_id")
	return map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"receipts":       []any{row},
		"available_views": map[string]any{
			"track:" + targetID + "::track.basic_energy": view,
		},
	}
}

// expAdmitTarget is the shared admission target (track vocal), matching
// d1FreshObservationForTest's latest observation target.
func expAdmitTarget() map[string]any {
	return map[string]any{"kind": "track", "id": "vocal"}
}

// Paired test ①: a citation of an earlier same-revision observation of the
// same target — the exact 262 verifyrun1 shape — passes the relaxed admission
// binding without quoting the LatestObservation id.
func TestD1EvidenceCitationAcceptsSameRevisionEarlierTargetObservation(t *testing.T) {
	loop := freeStateReasoningLoop{
		LatestObservation: d1FreshObservationForTest("7"),
		ObservationLedger: expAdmitEarlierObservationLedger("obs-early", "7", expAdmitTarget()),
	}
	admission := experiment.Admission{TargetRef: expAdmitTarget(), EvidenceRefs: []string{"obs-early"}}
	if _, err := validateD1FreshObservedTarget(loop, admission); err != nil {
		t.Fatalf("same-revision earlier target observation citation was refused: %v", err)
	}
}

// Paired test ②: the same earlier-observation citation bound to an older
// revision is still refused — G7's revision-bound freshness semantics are not
// weakened, only the which-observation selection rule moved.
func TestD1EvidenceCitationStillRefusesCrossRevisionStaleObservation(t *testing.T) {
	loop := freeStateReasoningLoop{
		LatestObservation: d1FreshObservationForTest("7"),
		ObservationLedger: expAdmitEarlierObservationLedger("obs-early", "6", expAdmitTarget()),
	}
	admission := experiment.Admission{TargetRef: expAdmitTarget(), EvidenceRefs: []string{"obs-early"}}
	_, err := validateD1FreshObservedTarget(loop, admission)
	if err == nil || !strings.Contains(err.Error(), "must cite a fresh revision-bound observation") {
		t.Fatalf("cross-revision stale citation must keep its refusal, got %v", err)
	}
}

// Paired test ③: the refusal vocabulary survives the relaxation. The relaxed
// path still requires (a) the cited row to be an observation of the admission
// target, (b) an explicitly fresh class, and (c) a revision equal to the
// latest observation's revision — and a ref that resolves to nothing at all
// keeps the original refusal exactly like the pre-relaxation pin did.
func TestD1EvidenceCitationRelaxationKeepsRefusalVocabulary(t *testing.T) {
	cases := []struct {
		name    string
		ledger  map[string]any
		cited   string
		wantErr string
	}{
		{
			name:    "same-revision observation of another target",
			ledger:  expAdmitEarlierObservationLedger("obs-other", "7", map[string]any{"kind": "track", "id": "other-track"}),
			cited:   "obs-other",
			wantErr: "must cite a fresh revision-bound observation",
		},
		{
			name: "explicitly stale class at the same revision",
			ledger: func() map[string]any {
				ledger := expAdmitEarlierObservationLedger("obs-stale", "7", expAdmitTarget())
				for _, raw := range ledger["receipts"].([]any) {
					row := raw.(map[string]any)
					row["freshness"] = map[string]any{"status": "stale", "class": "stale", "project_revision": "7"}
				}
				for _, raw := range ledger["available_views"].(map[string]any) {
					row := raw.(map[string]any)
					row["freshness"] = map[string]any{"status": "stale", "class": "stale", "project_revision": "7"}
				}
				return ledger
			}(),
			cited:   "obs-stale",
			wantErr: "must cite a fresh revision-bound observation",
		},
		{
			name: "no revision information cannot prove freshness",
			ledger: func() map[string]any {
				ledger := expAdmitEarlierObservationLedger("obs-norev", "7", expAdmitTarget())
				for _, raw := range ledger["receipts"].([]any) {
					row := raw.(map[string]any)
					delete(row, "project_revision")
					delete(row, "freshness")
				}
				for _, raw := range ledger["available_views"].(map[string]any) {
					row := raw.(map[string]any)
					delete(row, "project_revision")
					delete(row, "freshness")
				}
				return ledger
			}(),
			cited:   "obs-norev",
			wantErr: "must cite a fresh revision-bound observation",
		},
		{
			name:    "citation resolving to no ledger row at all",
			ledger:  expAdmitEarlierObservationLedger("obs-early", "7", expAdmitTarget()),
			cited:   "obs-nowhere",
			wantErr: "must cite a fresh revision-bound observation",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			loop := freeStateReasoningLoop{
				LatestObservation: d1FreshObservationForTest("7"),
				ObservationLedger: testCase.ledger,
			}
			admission := experiment.Admission{TargetRef: expAdmitTarget(), EvidenceRefs: []string{testCase.cited}}
			_, err := validateD1FreshObservedTarget(loop, admission)
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("refusal vocabulary weakened for %q: got %v", testCase.cited, err)
			}
		})
	}
}

// The direct citation of the latest observation keeps its historical pass —
// the relaxation only widens the accepted citation set.
func TestD1EvidenceCitationLatestObservationQuoteStillPasses(t *testing.T) {
	loop := freeStateReasoningLoop{LatestObservation: d1FreshObservationForTest("7")}
	admission := experiment.Admission{TargetRef: expAdmitTarget(), EvidenceRefs: []string{"obs-before"}}
	if _, err := validateD1FreshObservedTarget(loop, admission); err != nil {
		t.Fatalf("direct latest-observation citation was refused: %v", err)
	}
}

// Paired test ④: an experiment admission refusal parks the proposal for user
// adjudication instead of terminalizing it — the loop keeps the decision's
// own shape and awaiting_experiment presentation (the historical rewrite made
// the response face settle terminal-done and the turn-close guard closed the
// honest proposal as an orphan, 262 verifyrun1), no experiment admission is
// constructed, the refusal stands in the admission receipt and LastError, and
// the TerminalAdjudication disclosure latches with the content-free refusal
// marker.
func TestAdmissionRefusalParksProposalInsteadOfTerminalizing(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-admit-park", ConversationID: "conversation-admit-park",
		GoalID: "goal-park", RunID: "run-park", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "improve", ActiveIntent: "improve", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	proposal := experimentTestProposal()
	proposal.EvidenceRefs = []string{"obs-nowhere"}
	decision := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "bounded hypothesis the admission validator refuses", ImprovementProposal: proposal, RequestedViewIDs: []string{"track.timbre_frequency"},
	}
	stored, ok := s.recordFreeStateDecision("conversation-admit-park", agentloop.Result{GoalID: "goal-park", RunID: "run-park", FreeStateDecision: &decision})
	if !ok {
		t.Fatal("parked admission refusal turn was not retained")
	}
	if stored.Status != "awaiting_experiment" {
		t.Fatalf("admission refusal must park the loop at awaiting_experiment, got %q (last_error=%q)", stored.Status, stored.LastError)
	}
	if stored.LatestDecision == nil || stored.LatestDecision.Status != agentloop.FreeStateNeedsExperiment {
		t.Fatalf("the refused decision must keep its needs_experiment shape, got %+v", stored.LatestDecision)
	}
	if stored.Experiment != nil {
		t.Fatal("admission refusal must not construct an experiment admission (the user adjudicates at the confirmation face)")
	}
	if len(stored.TerminalAdjudication) == 0 {
		t.Fatal("admission refusal must latch the adjudication park disclosure")
	}
	if firstStringFromMap(stored.TerminalAdjudication, "schema_version") != freeStateTerminalAdjudicationSchema {
		t.Fatalf("park disclosure schema = %q", firstStringFromMap(stored.TerminalAdjudication, "schema_version"))
	}
	ids := adjudicationStringList(stored.TerminalAdjudication, "failed_gate_ids")
	if len(ids) != 1 || ids[0] != freeStateAdmissionRefusedParkGateID {
		t.Fatalf("park disclosure failed_gate_ids = %v, want [%s]", ids, freeStateAdmissionRefusedParkGateID)
	}
	if receipt := stored.AdmissionReceipt; len(receipt) == 0 || receipt["status"] != agentloop.FreeStateBlocked || receipt["boundary"] == "admitted" {
		t.Fatalf("the honest refusal must stay recorded in the admission receipt, got %+v", receipt)
	}
	if !strings.Contains(stored.LastError, "parked for adjudication") {
		t.Fatalf("the refusal must stay in LastError, got %q", stored.LastError)
	}
}

// Paired test ⑤: the parked refusal reaches the confirmation face, not a
// terminal done — mirroring the goalrunner binding (res.FreeStateDecision =
// loop.LatestDecision) the response surface sees the parked loop, and the
// latched disclosure forces the confirmation card even under full project
// access (the auto-authorization policy covers admitted proposals only).
func TestAdmissionRefusalParkSurfacesConfirmationCardNotTerminalDone(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-admit-face", ConversationID: "conversation-admit-face",
		GoalID: "goal-face", RunID: "run-face", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "improve", ActiveIntent: "improve", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	proposal := experimentTestProposal()
	proposal.EvidenceRefs = []string{"obs-nowhere"}
	decision := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "bounded hypothesis the admission validator refuses", ImprovementProposal: proposal, RequestedViewIDs: []string{"track.timbre_frequency"},
	}
	stored, _ := s.recordFreeStateDecision("conversation-admit-face", agentloop.Result{GoalID: "goal-face", RunID: "run-face", FreeStateDecision: &decision})
	res := agentloop.Result{GoalID: "goal-face", RunID: "run-face", FreeStateDecision: stored.LatestDecision}
	resp := s.improvementProposalResponse("conversation-admit-face", "default", res, map[string]any{"authority_mode": string(experiment.AuthorityFull)})
	if !resp.NeedsConfirmation || resp.GoalStatus != "waiting_confirmation" {
		t.Fatalf("the parked refusal must park on the confirmation face, got needs_confirmation=%t status=%q stop=%q",
			resp.NeedsConfirmation, resp.GoalStatus, resp.StopReason)
	}
	if resp.StopReason != "improvement_proposal_confirmation_required" {
		t.Fatalf("unexpected stop reason %q", resp.StopReason)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].Kind != "improvement_proposal_confirmation" {
		t.Fatalf("the confirmation card was not built: %+v", resp.InteractionRequests)
	}
	adj := firstMapFromAny(resp.WorkflowData["terminal_adjudication"])
	if firstStringFromMap(adj, "schema_version") != freeStateTerminalAdjudicationSchema {
		t.Fatalf("workflow_data.terminal_adjudication missing on the parked refusal: %+v", adj)
	}
	if ids := adjudicationStringList(adj, "failed_gate_ids"); len(ids) != 1 || ids[0] != freeStateAdmissionRefusedParkGateID {
		t.Fatalf("park disclosure marker missing: %v", ids)
	}
}

// The park latch is per-refusal: a later decision whose proposal admits
// cleanly clears the stale disclosure and constructs the experiment — the
// park can never misdescribe a loop that carries a live admitted experiment.
func TestAdmissionParkLatchClearsOnLaterSuccessfulAdmission(t *testing.T) {
	s := New(nil, nil, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-admit-clear", ConversationID: "conversation-admit-clear",
		GoalID: "goal-clear", RunID: "run-clear", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "improve", ActiveIntent: "improve", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	})
	refused := experimentTestProposal()
	refused.EvidenceRefs = []string{"obs-nowhere"}
	first := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "refused first attempt", ImprovementProposal: refused, RequestedViewIDs: []string{"track.timbre_frequency"},
	}
	if parked, _ := s.recordFreeStateDecision("conversation-admit-clear", agentloop.Result{GoalID: "goal-clear", RunID: "run-clear", FreeStateDecision: &first}); len(parked.TerminalAdjudication) == 0 {
		t.Fatal("the first refused attempt must latch the park disclosure")
	}
	second := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "admitted retry", ImprovementProposal: experimentTestProposal(), RequestedViewIDs: []string{"track.timbre_frequency"},
	}
	stored, _ := s.recordFreeStateDecision("conversation-admit-clear", agentloop.Result{GoalID: "goal-clear", RunID: "run-clear", FreeStateDecision: &second})
	if stored.Experiment == nil {
		t.Fatalf("the admitted retry must construct the experiment, last_error=%q", stored.LastError)
	}
	if len(stored.TerminalAdjudication) != 0 {
		t.Fatalf("a successful admission must clear the stale park disclosure, got %+v", stored.TerminalAdjudication)
	}
	if stored.Status != "awaiting_experiment" {
		t.Fatalf("admitted retry loop status = %q", stored.Status)
	}
}
