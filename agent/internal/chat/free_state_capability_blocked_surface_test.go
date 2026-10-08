package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FS-CAPABILITY-BLOCKED-SURFACE-1 (FS-NL-PROPOSAL-RECON-1 fix): when the
// server-side admission gate refuses the round's improvement proposal, the
// response router used to fall through to the plain branch and deliver the
// model's prose as a successful done turn — the R3 trace promised an
// experiment ("执行后请直接 A/B 试听") that would never run while the goal
// read completed. These tests pin the explicit boundary surface: the refusal
// itself is correct enforcement and stays untouched; only its delivery face
// changes.

// gateRefusedFreeStateFixture rebuilds the R3 admission shape through the
// production recorder: an active observing loop receives a needs_experiment
// decision whose valid proposal cites target-track evidence, while the
// pre-fold frontier holds a candidate without track coverage. The gate audit
// must refuse exactly G6+G8 (the R3 receipt signature) with G7 evidence
// resolution still passing.
func gateRefusedFreeStateFixture(t *testing.T) (*Server, freeStateReasoningLoop, agentloop.Result) {
	t.Helper()
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-r3-surface",
		ConversationID: "conversation-r3-surface",
		GoalID:         "goal-r3-surface",
		RunID:          "run-r3-surface",
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
		"task_contract":                  map[string]any{"project_uuid": "uuid-r3", "project_revision": "3", "kind": "improvement"},
		"free_state_capacity_assessment": map[string]any{"capacity_level": "within_free_state", "selected_capability": "free_state"},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "uuid-r3", "project_revision": "3",
			"diagnostic_rounds": []any{map[string]any{"evidence_status": "ready"}},
			"hypothesis_frontier": map[string]any{
				"candidates": []any{map[string]any{"id": "cand-unfolded"}},
			},
		},
		"free_state_reasoning_loop": map[string]any{
			"observation_ledger": map[string]any{
				"receipts": []any{
					map[string]any{"observation_id": "obs-scan-r3", "status": "ready", "project_revision": "3", "requested_views": []any{"mix.multitrack_relationship"}},
					map[string]any{"observation_id": "obs-202-r3", "status": "ready", "project_revision": "3", "target_ref": map[string]any{"kind": "track", "id": "202"}},
				},
			},
		},
	}
	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID,
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		// The R3 leak face: a prose final answer promising an experiment that
		// the admission gate has just refused. The boundary response must never
		// deliver it.
		Reply:           "我准备对 202 号轨做一次有界实验，执行后请直接 A/B 试听。",
		ContextSnapshot: auditContext,
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion:  agentloop.FreeStateDecisionSchema,
			Status:         agentloop.FreeStateNeedsExperiment,
			EvidenceStatus: "sufficient",
			Summary:        "candidate diagnosis established",
			ImprovementProposal: &agentprotocol.ImprovementProposal{
				SchemaVersion:     agentprotocol.ImprovementProposalSchema,
				Target:            map[string]any{"kind": "track", "id": "202"},
				EvidenceRefs:      []string{"obs-202-r3"},
				ImprovementIntent: "bounded high-frequency smoothing on the target track",
				Hypothesis:        "a bounded attenuation improves the harshness",
				ExpectedEffect:    "A/B comparable candidate",
				ActionDomain:      agentprotocol.ImprovementActionDomainTrackGain,
				ActionKind:        "gain_adjust",
				ParameterBounds:   map[string]any{"delta_db": -1.5},
				Confidence:        0.5,
			},
		},
	}
	return s, loop, res
}

func gateReceiptFailedIDs(t *testing.T, receipt map[string]any) []string {
	t.Helper()
	failed := freeStateCapabilityBlockedStringList(receipt["failed_gate_ids"])
	if len(failed) == 0 {
		t.Fatalf("admission receipt carries no failed_gate_ids: %+v", receipt)
	}
	return failed
}

func TestGateRefusalSettlesCapabilityBlockedLoopShape(t *testing.T) {
	s, loop, res := gateRefusedFreeStateFixture(t)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("gate-refused decision was not retained by recordFreeStateDecision")
	}
	if stored.Status != "capability_blocked" {
		t.Fatalf("gate refusal did not settle the loop at capability_blocked: %q", stored.Status)
	}
	if stored.LatestDecision == nil || stored.LatestDecision.Status != agentloop.FreeStateCapabilityBlocked {
		t.Fatalf("latest decision is not capability_blocked: %+v", stored.LatestDecision)
	}
	if stored.LatestDecision.StopReason != "free_state_admission_gate_failed" {
		t.Fatalf("gate refusal recorded stop_reason %q", stored.LatestDecision.StopReason)
	}
	if boundary := firstStringFromMap(stored.AdmissionReceipt, "boundary"); boundary != "admission_gate_failed" {
		t.Fatalf("admission receipt boundary = %q, want admission_gate_failed", boundary)
	}
	failed := gateReceiptFailedIDs(t, stored.AdmissionReceipt)
	joined := strings.Join(failed, ",")
	if !strings.Contains(joined, "G6_target_evidence") || !strings.Contains(joined, "G8_target_consistency") {
		t.Fatalf("R3 signature gates missing from failed_gate_ids: %q", joined)
	}
	if strings.Contains(joined, "G7") {
		t.Fatalf("evidence resolution (G7) must stay passing in the R3 shape: %q", joined)
	}
}

func TestCapabilityBlockedBoundaryResponseSurface(t *testing.T) {
	s, loop, res := gateRefusedFreeStateFixture(t)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("gate-refused decision was not retained by recordFreeStateDecision")
	}
	resp, blocked := s.capabilityBlockedBoundaryResponse(loop.ConversationID, "default", res, stored)
	if !blocked {
		t.Fatal("capability_blocked boundary was not detected on the settled loop")
	}
	// Face 1: the machine-readable stop reason.
	if resp.StopReason != "capability_blocked" {
		t.Fatalf("boundary response stop_reason = %q, want capability_blocked", resp.StopReason)
	}
	// Face 2: the workflow data carries the full admission receipt.
	receipt, ok := resp.WorkflowData["free_state_admission_receipt"].(map[string]any)
	if !ok {
		t.Fatalf("workflow data lost the admission receipt: %+v", resp.WorkflowData)
	}
	if boundary := firstStringFromMap(receipt, "boundary"); boundary != "admission_gate_failed" {
		t.Fatalf("workflow receipt boundary = %q", boundary)
	}
	failed := gateReceiptFailedIDs(t, receipt)
	joined := strings.Join(failed, ",")
	if !strings.Contains(joined, "G6_target_evidence") || !strings.Contains(joined, "G8_target_consistency") {
		t.Fatalf("workflow receipt lost the R3 gate signature: %q", joined)
	}
	if resp.WorkflowData["capability_blocked"] != true || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("workflow data boundary markers wrong: %+v", resp.WorkflowData)
	}
	// Face 3: the reply is a capability boundary statement — it names the
	// refusal, never delivers the model's promise, and claims no execution.
	if !strings.Contains(resp.Reply, "准入门") || !strings.Contains(resp.Reply, "G6_target_evidence") || !strings.Contains(resp.Reply, "G8_target_consistency") {
		t.Fatalf("boundary reply does not state the admission refusal: %q", resp.Reply)
	}
	if strings.Contains(resp.Reply, "A/B 试听") || strings.Contains(resp.Reply, "执行后请") {
		t.Fatalf("boundary reply leaked the model's promised experiment: %q", resp.Reply)
	}
	for _, promise := range []string{"已执行", "已完成实验", "已应用"} {
		if strings.Contains(resp.Reply, promise) {
			t.Fatalf("boundary reply fakes success with %q: %q", promise, resp.Reply)
		}
	}
	if !strings.Contains(resp.Reply, "没有执行任何实验或修改") || !strings.Contains(resp.Reply, "不会自动重试") {
		t.Fatalf("boundary reply does not state the no-execution/no-retry boundary: %q", resp.Reply)
	}
	// The durable envelope semantics stay untouched — the boundary rides
	// stop_reason + workflow data, exactly like the terminal inactive-resume
	// response above (recordGoalResult already persisted the same envelope).
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("boundary response changed the durable envelope status: %q", resp.GoalStatus)
	}
}

func TestCapabilityBlockedBoundaryDetectionKeepsExistingPaths(t *testing.T) {
	now := time.Now().UTC()
	base := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-negative",
		ConversationID: "conversation-negative", GoalID: "goal-negative", RunID: "run-negative",
		OriginalIntent: "bounded intent", CreatedAt: now, UpdatedAt: now,
	}
	staleGateReceipt := map[string]any{"schema_version": "free_state_admission_receipt.v1", "boundary": "admission_gate_failed", "failed_gate_ids": []string{"G6_target_evidence"}}
	cases := []struct {
		name string
		loop freeStateReasoningLoop
	}{
		{"awaiting_action", func() freeStateReasoningLoop {
			loop := base
			loop.Status = "awaiting_action"
			loop.LatestDecision = &agentloop.FreeStateDecision{Status: agentloop.FreeStateNeedsAction}
			return loop
		}()},
		{"awaiting_experiment_parked_with_stale_gate_receipt", func() freeStateReasoningLoop {
			// 方案乙 parking: the receipt legitimately keeps the gate refusal
			// while the loop parks at the confirmation face — that face must
			// win, the boundary response must not hijack it.
			loop := base
			loop.Status = "awaiting_experiment"
			loop.LatestDecision = &agentloop.FreeStateDecision{Status: agentloop.FreeStateNeedsExperiment}
			loop.AdmissionReceipt = staleGateReceipt
			return loop
		}()},
		{"plain_completed_diagnostic", func() freeStateReasoningLoop {
			loop := base
			loop.Status = "completed"
			loop.LatestDecision = &agentloop.FreeStateDecision{Status: agentloop.FreeStateDiagnosticComplete}
			return loop
		}()},
		{"judgment_boundary_blocked", func() freeStateReasoningLoop {
			// The judgment boundary parks the loop as blocked with the settle
			// report as latest decision — its own settle face owns delivery.
			loop := base
			loop.Status = "blocked"
			loop.LatestDecision = &agentloop.FreeStateDecision{Status: agentloop.FreeStateNeedsExperiment, ExperimentRoundDecision: "user_judgment_pending"}
			loop.AdmissionReceipt = staleGateReceipt
			return loop
		}()},
		{"no_decision", base},
	}
	for _, testCase := range cases {
		if receipt, blocked := freeStateCapabilityBlockedBoundary(testCase.loop); blocked {
			t.Fatalf("%s was hijacked by the capability_blocked boundary (receipt=%+v)", testCase.name, receipt)
		}
	}
}

func TestCapabilityBlockedBoundaryDetectsModelTerminalConcession(t *testing.T) {
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-model-blocked",
		ConversationID: "conversation-model-blocked", GoalID: "goal-model-blocked", RunID: "run-model-blocked",
		OriginalIntent: "bounded intent", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		Status:         "blocked",
		LatestDecision: &agentloop.FreeStateDecision{Status: agentloop.FreeStateCapabilityBlocked, Summary: "no governed processor family for the requested change"},
	}
	receipt, blocked := freeStateCapabilityBlockedBoundary(loop)
	if !blocked {
		t.Fatal("model-declared terminal capability boundary was not detected")
	}
	if len(freeStateCapabilityBlockedStringList(receipt["failed_gate_ids"])) != 0 {
		t.Fatalf("model-terminal boundary must not invent gate failures: %+v", receipt)
	}
	reply := freeStateCapabilityBlockedReply(receipt, loop.LatestDecision)
	if !strings.Contains(reply, "能力边界") || !strings.Contains(reply, "没有执行任何实验或修改") {
		t.Fatalf("model-terminal boundary reply lost the boundary statement: %q", reply)
	}
	if !strings.Contains(reply, "no governed processor family") {
		t.Fatalf("model-terminal boundary reply dropped the model's stated boundary: %q", reply)
	}
}
