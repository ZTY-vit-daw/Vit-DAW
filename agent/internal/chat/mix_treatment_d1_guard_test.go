package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// D1-MIXTREAT-KIND-GUARD-1: the 2026-10-24 forensic (reverb admission armed,
// pan_balance proposal stored and published as an executable confirmation
// card, approve, frozen-gate reason "D1-S1 requires one bounded
// reverb_decay_adjust" leaked verbatim into the chat reply) pins two failure
// shapes these tests lock out: the mismatched-treatment card must never be
// stored or published, and the blocked reply must stay localized.

func d1GuardAdmissionLoop(t *testing.T, conversationID, domain, kind, targetID string) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-d1-guard",
		ConversationID: conversationID,
		GoalID:         "goal-d1-guard",
		RunID:          "run-d1-guard",
		Status:         "awaiting_experiment",
		OriginalIntent: "给人声加点空间感",
		CreatedAt:      now,
		UpdatedAt:      now,
		Experiment: &experiment.Turn{
			SchemaVersion: experiment.SchemaVersion,
			ID:            "turn-d1-guard",
			Admission: experiment.Admission{
				SchemaVersion: experiment.SchemaVersion,
				TargetRef:     map[string]any{"kind": "track", "id": targetID},
				EvidenceRefs:  []string{"obs_guard_1"},
				Hypothesis:    "a bounded dose may improve the mix",
				ExpectedEffect: "clearer relationship",
				CheckpointRef: "ckpt_guard_1",
				TypedAction:   map[string]any{"action_domain": domain, "action_kind": kind},
				DiagnosticDoseBounds: map[string]any{"max_action_attempts": 1},
				RetainedDoseBounds:   map[string]any{"max_action_attempts": 1},
				ExperimentBudget:     1,
				VerificationPlan:     map[string]any{"views": []any{"track.stereo_space"}},
				RollbackPlan:         map[string]any{"strategy": "project_history"},
				AuthorityMode:        experiment.AuthorityOrdinary,
			},
		},
	}
}

func d1GuardPanTreatment(targetTrack string) agentloop.MixTreatmentPending {
	return agentloop.MixTreatmentPending{
		SchemaVersion:  "mix_treatment_pending.v0",
		Status:         "pending_confirmation",
		ConversationID: "chat_d1_guard",
		ObservationID:  "obs_guard_1",
		TargetRef:      "track:" + targetTrack,
		ActionKind:     "pan_balance",
		ProcessorType:  "utility",
		DeltaPan:       0.1,
		Confidence:     "medium",
	}
}

func d1GuardGainTreatment(targetTrack string) agentloop.MixTreatmentPending {
	return agentloop.MixTreatmentPending{
		SchemaVersion:  "mix_treatment_pending.v0",
		Status:         "pending_confirmation",
		ConversationID: "chat_d1_guard",
		ObservationID:  "obs_guard_1",
		TargetRef:      "track:" + targetTrack,
		ActionKind:     "gain_balance",
		ProcessorType:  "utility",
		DeltaDB:        -1.5,
		Confidence:     "medium",
	}
}

func d1GuardAssertNoExecutableTreatmentCard(t *testing.T, resp ChatResponse) {
	t.Helper()
	if resp.NeedsConfirmation {
		t.Fatalf("refused treatment still set NeedsConfirmation")
	}
	if resp.Workflow == "mix_treatment" {
		t.Fatalf("refused treatment still overrode the response workflow: %q", resp.Workflow)
	}
	for _, req := range resp.InteractionRequests {
		if req.Kind == "mix_treatment_confirmation" {
			t.Fatalf("refused treatment still published a confirmation card: %+v", req)
		}
	}
}

func d1GuardAssertLocalizedGuidance(t *testing.T, reply string) {
	t.Helper()
	if !strings.Contains(reply, "实验轮") || !strings.Contains(reply, "没有修改工程") {
		t.Fatalf("refusal reply is not the honest localized guidance: %q", reply)
	}
	for _, needle := range []string{"D1-S1", "D2-1", "FAM3-S1", "reverb", "pan_balance", "gain_balance", "requires"} {
		if strings.Contains(reply, needle) {
			t.Fatalf("refusal reply leaked the internal signature %q: %q", needle, reply)
		}
	}
}

// The forensic shape: a PluginBound reverb admission plus a pan_balance
// proposal whose only end state was the frozen gate. The guard must refuse
// the card and the durable store, demote the confirmation wait, and settle
// the turn with the localized guidance.
func TestPendingMixTreatmentRefusedUnderMismatchedD1S1Admission(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "reverb", "reverb_decay_adjust", "1032"))
	treatment := d1GuardPanTreatment("1032")
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID:       "goal_1",
		RunID:        "run_1",
		Status:       agentruntime.StatusWaitingConfirmation,
		Reply:        "我建议把 1032 的声像微调 0.10。",
		Continuation: &agentloop.Continuation{GoalID: "goal_1", RunID: "run_1"},
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})

	d1GuardAssertNoExecutableTreatmentCard(t, resp)
	d1GuardAssertLocalizedGuidance(t, resp.Reply)
	if !strings.Contains(resp.Reply, "混响") || !strings.Contains(resp.Reply, "声像") {
		t.Fatalf("refusal reply omitted the admitted domain or refused family: %q", resp.Reply)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("refused confirmation wait was not demoted to waiting_continue: %q", resp.GoalStatus)
	}
	if resp.StopReason != mixTreatmentRefusedD1AdmissionMismatchReason {
		t.Fatalf("stop reason = %q", resp.StopReason)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_d1_guard"); ok {
		t.Fatal("mismatched treatment was durably stored")
	}
}

// A treatment riding a completed result (the published-card fixture shape)
// must be refused just the same: no card, guidance reply, completed goal.
func TestPendingMixTreatmentRefusedOnCompletedResultWithoutCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "reverb", "reverb_decay_adjust", "1032"))
	treatment := d1GuardPanTreatment("1032")
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "已根据声像/立体声观察生成一个待确认的声像处理候选。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})

	d1GuardAssertNoExecutableTreatmentCard(t, resp)
	d1GuardAssertLocalizedGuidance(t, resp.Reply)
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("completed result was reshaped by the refusal: %q", resp.GoalStatus)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_d1_guard"); ok {
		t.Fatal("mismatched treatment was durably stored")
	}
}

// Stop-condition control: without a D1-S1 admission the same pan_balance
// treatment keeps the ordinary executable confirmation card, byte for byte
// with the pre-guard behavior.
func TestPendingMixTreatmentWithoutAdmissionKeepsExecutableCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	treatment := d1GuardPanTreatment("1032")
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "已根据声像/立体声观察生成一个待确认的声像处理候选。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})

	if !resp.NeedsConfirmation || resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) || resp.Workflow != "mix_treatment" {
		t.Fatalf("no-admission pan card lost its confirmation shape: needs=%v status=%q workflow=%q", resp.NeedsConfirmation, resp.GoalStatus, resp.Workflow)
	}
	if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].Kind != "mix_treatment_confirmation" {
		t.Fatalf("interaction requests = %+v", resp.InteractionRequests)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_d1_guard"); !ok {
		t.Fatal("no-admission treatment was not stored")
	}
}

// Matching kinds keep their executable cards under their own admissions: the
// classic gain flow (gain_balance under track_gain) and the pan delta flow
// (pan_balance under pan). The guard is scoped to the mismatch branch only.
func TestPendingMixTreatmentMatchingKindKeepsExecutableCard(t *testing.T) {
	cases := []struct {
		name      string
		domain    string
		kind      string
		treatment agentloop.MixTreatmentPending
	}{
		{"gain under track_gain admission", "track_gain", "track_gain_adjust", d1GuardGainTreatment("1007")},
		{"pan delta under pan admission", "pan", "track_pan_adjust", d1GuardPanTreatment("1007")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := New(nil, shadow.New(nil), nil)
			server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", tc.domain, tc.kind, "1007"))
			resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
				GoalID: "goal_1",
				RunID:  "run_1",
				Status: agentruntime.StatusCompleted,
				Reply:  "已生成一个待确认的处理候选。",
				ExecutionMemory: agentloop.ExecutionMemory{
					PendingMixTreatment: &tc.treatment,
				},
			})
			if !resp.NeedsConfirmation || resp.Workflow != "mix_treatment" || len(resp.InteractionRequests) != 1 {
				t.Fatalf("matching-kind card was refused: needs=%v workflow=%q requests=%d", resp.NeedsConfirmation, resp.Workflow, len(resp.InteractionRequests))
			}
			if _, ok := server.pendingMixTreatmentForConversation("chat_d1_guard"); !ok {
				t.Fatal("matching-kind treatment was not stored")
			}
		})
	}
}

// A pan_balance treatment carrying an explicit target pan resolves to
// track_pan_set, which the pan admission's frozen plan can never accept —
// the kind dimension catches it even inside the pan family.
func TestPendingMixTreatmentPanSetRefusedUnderPanAdmission(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "pan", "track_pan_adjust", "1007"))
	treatment := d1GuardPanTreatment("1007")
	targetPan := 0.2
	treatment.TargetPan = &targetPan
	treatment.DeltaPan = 0
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "已生成一个待确认的声像设置候选。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})
	d1GuardAssertNoExecutableTreatmentCard(t, resp)
	d1GuardAssertLocalizedGuidance(t, resp.Reply)
	if !strings.Contains(resp.Reply, "声像") {
		t.Fatalf("pan-set refusal omitted the refused family: %q", resp.Reply)
	}
}

// The target dimension (the 157 target-consistency sibling): a matching kind
// against a different track than the admitted observation target can never
// pass the frozen gate's target check, so it is refused at issuance too.
func TestPendingMixTreatmentTargetMismatchRefusedUnderAdmission(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "track_gain", "track_gain_adjust", "1007"))
	treatment := d1GuardGainTreatment("1032")
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "已生成一个待确认的电平调整候选。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})
	d1GuardAssertNoExecutableTreatmentCard(t, resp)
	d1GuardAssertLocalizedGuidance(t, resp.Reply)
	if !strings.Contains(resp.Reply, "目标") {
		t.Fatalf("target-mismatch refusal omitted the target wording: %q", resp.Reply)
	}
}

// Non-executable treatments (the resolver would answer needs_* honestly) are
// out of the guard's scope: their card keeps the pre-guard behavior even
// under an admission, because approval never reaches the frozen gate.
func TestPendingMixTreatmentNonExecutableKindKeepsCardUnderAdmission(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "reverb", "reverb_decay_adjust", "1032"))
	treatment := d1GuardPanTreatment("1032")
	treatment.ActionKind = "plugin_treatment"
	treatment.DeltaPan = 0
	resp := server.chatResponseFromAgentLoopResult("chat_d1_guard", agentModeDefault, agentloop.Result{
		GoalID: "goal_1",
		RunID:  "run_1",
		Status: agentruntime.StatusCompleted,
		Reply:  "已生成一个待确认的插件处理候选。",
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTreatment: &treatment,
		},
	})
	if !resp.NeedsConfirmation || len(resp.InteractionRequests) != 1 {
		t.Fatalf("non-executable treatment card was refused by the admission guard: needs=%v requests=%d", resp.NeedsConfirmation, len(resp.InteractionRequests))
	}
}

// The approve-face defense: a card issued before the admission armed (the
// race window) must be answered with the same honest refusal instead of
// dispatching into the frozen gate.
func TestPendingMixTreatmentApproveRefusedUnderAdmissionRace(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	server.storeFreeStateLoop(d1GuardAdmissionLoop(t, "chat_d1_guard", "reverb", "reverb_decay_adjust", "1032"))
	treatment := d1GuardPanTreatment("1032")
	server.mu.Lock()
	if server.pendingTreatments == nil {
		server.pendingTreatments = map[string]agentloop.MixTreatmentPending{}
	}
	server.pendingTreatments["chat_d1_guard"] = treatment
	server.mu.Unlock()

	resp, handled := server.handlePendingMixTreatmentChat(context.Background(), "chat_d1_guard", ChatRequest{Message: "可以执行"}, agentModeDefault)
	if !handled {
		t.Fatal("approve was not handled by the pending treatment surface")
	}
	d1GuardAssertLocalizedGuidance(t, resp.Reply)
	if resp.StopReason != mixTreatmentRefusedD1AdmissionMismatchReason {
		t.Fatalf("stop reason = %q", resp.StopReason)
	}
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("goal status = %q", resp.GoalStatus)
	}
	if _, ok := server.pendingMixTreatmentForConversation("chat_d1_guard"); ok {
		t.Fatal("approved mismatched treatment survived the refusal")
	}
}

// The blocked-reply face: the structured Error keeps the machine reason
// verbatim while the Reply stays localized and actionable — the forensic
// signature must never reach the user surface again.
func TestD1BlockedResponseReplyLocalizedErrorPreserved(t *testing.T) {
	loop := d1GuardAdmissionLoop(t, "chat_d1_guard", "reverb", "reverb_decay_adjust", "1032")
	cases := []struct {
		name       string
		reason     string
		wantNeedle string
	}{
		{"kind mismatch", "D1-S1 requires one bounded reverb_decay_adjust", "混响"},
		{"target mismatch", "D1-S1 candidate target does not match admitted observation target", "目标"},
		{"generic boundary", "D1-S1 durable session has no frozen plan", "执行边界"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := d1BlockedResponse(loop, tc.reason)
			if resp.Error != tc.reason {
				t.Fatalf("structured Error was rewritten: %q", resp.Error)
			}
			if resp.Reply == tc.reason {
				t.Fatalf("Reply leaked the internal reason verbatim")
			}
			if !strings.Contains(resp.Reply, "没有修改工程") || !strings.Contains(resp.Reply, tc.wantNeedle) {
				t.Fatalf("blocked reply is not the localized actionable copy: %q", resp.Reply)
			}
			for _, needle := range []string{"D1-S1", "D2-1", "FAM3-S1", "reverb_decay_adjust", "frozen plan", "requires"} {
				if strings.Contains(resp.Reply, needle) {
					t.Fatalf("blocked reply leaked the internal signature %q: %q", needle, resp.Reply)
				}
			}
			if resp.StopReason != "d1_execution_blocked" || resp.GoalStatus != string(agentruntime.StatusFailed) {
				t.Fatalf("blocked response contract fields changed: stop=%q status=%q", resp.StopReason, resp.GoalStatus)
			}
		})
	}
}
