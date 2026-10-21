package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

// REPLY-FACE-BOUNDARY-LAYER-1 (146-L3 fix): the boundary reply must follow the
// admission receipt's boundary layer. The forensics F4/E5 shape (model
// self-declared the boundary before any governed proposal existed) still
// carries failed_gate_ids from the gate audit of the proposal-less round, and
// the pre-layering order rendered it as a refusal of a proposal that was never
// made while burying the model's actionable summary.

// proposalMissingReceipt rebuilds the E5 receipt signature: boundary
// proposal_missing, proposal_present=false, and the gate audit's failed ids
// for the empty round shape.
func proposalMissingReceipt(boundary string) map[string]any {
	return map[string]any{
		"schema_version":   "free_state_admission_receipt.v1",
		"status":           "capability_blocked",
		"boundary":         boundary,
		"failed_gate_ids":  []string{"G3_project_scan", "G5_frontier_established", "G6_target_evidence", "G7_fresh_revision_bound_refs", "G8_target_consistency"},
		"proposal_present": false,
		"proposal_valid":   false,
	}
}

const replyFaceModelSummary = "工程信息没有披露人声轨，唯一可见轨道是 bass（id 1007）"

func assertBoundaryFaceReply(t *testing.T, reply string, wantSummaryFace bool) {
	t.Helper()
	const boundaryTail = "任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。"
	if !strings.Contains(reply, boundaryTail) {
		t.Fatalf("reply lost the no-execution/no-retry boundary tail: %q", reply)
	}
	if !wantSummaryFace {
		return
	}
	if !strings.Contains(reply, replyFaceModelSummary) {
		t.Fatalf("reply buried the model's actionable summary: %q", reply)
	}
	if strings.Contains(reply, "准入门") {
		t.Fatalf("proposal-less boundary must not state a proposal refusal: %q", reply)
	}
	for _, id := range []string{"G3_", "G5_", "G6_", "G7_", "G8_"} {
		if strings.Contains(reply, id) {
			t.Fatalf("reply leaked gate codename %q: %q", id, reply)
		}
	}
}

func TestCapabilityBlockedReplyProposalMissingTakesSummaryFace(t *testing.T) {
	decision := &agentloop.FreeStateDecision{Summary: replyFaceModelSummary}
	reply := freeStateCapabilityBlockedReply(proposalMissingReceipt("proposal_missing"), decision)
	assertBoundaryFaceReply(t, reply, true)

	// Without a summary the face degrades to the neutral boundary statement —
	// never to the gate refusal of a proposal that does not exist.
	silent := freeStateCapabilityBlockedReply(proposalMissingReceipt("proposal_missing"), &agentloop.FreeStateDecision{})
	assertBoundaryFaceReply(t, silent, false)
	if !strings.Contains(silent, "自由态推理在能力边界停止。") {
		t.Fatalf("proposal_missing without summary lost the neutral face: %q", silent)
	}
}

func TestCapabilityBlockedReplyProposalInvalidTakesSummaryFace(t *testing.T) {
	decision := &agentloop.FreeStateDecision{Summary: replyFaceModelSummary}
	reply := freeStateCapabilityBlockedReply(proposalMissingReceipt("proposal_invalid"), decision)
	assertBoundaryFaceReply(t, reply, true)
}

func TestCapabilityBlockedReplyAdmissionGateFailedKeepsGateFace(t *testing.T) {
	const boundaryTail = "任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。"
	receipt := proposalMissingReceipt("admission_gate_failed")
	receipt["failed_gate_ids"] = []string{"G6_target_evidence", "G8_target_consistency"}
	receipt["proposal_present"] = true
	receipt["proposal_valid"] = true
	// The gate face wins even when the model also supplied a summary: the
	// refusal of a real proposal is the honest face (148④ surface contract).
	decision := &agentloop.FreeStateDecision{Summary: "candidate diagnosis established"}
	reply := freeStateCapabilityBlockedReply(receipt, decision)
	want := "本次有界改进提案未通过自由态准入门（目标证据/目标一致性 两项准入检查未过），" + boundaryTail
	if reply != want {
		t.Fatalf("gate reply = %q, want %q", reply, want)
	}
}

func TestCapabilityBlockedReplyBoundaryDefaultKeepsCurrentBehavior(t *testing.T) {
	const boundaryTail = "任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。"
	failed := []string{"G6_target_evidence", "G8_target_consistency"}
	decision := &agentloop.FreeStateDecision{Summary: replyFaceModelSummary}

	// Legacy receipts carry no boundary key at all — the pre-layering order
	// (gate ids, then summary, then neutral) must keep ruling them.
	legacy := freeStateCapabilityBlockedReply(map[string]any{"failed_gate_ids": failed}, decision)
	if !strings.Contains(legacy, "准入门") {
		t.Fatalf("boundary-less receipt lost the gate face: %q", legacy)
	}

	// capability_boundary with no failed gates keeps the summary face.
	modelBlocked := freeStateCapabilityBlockedReply(
		map[string]any{"boundary": "capability_boundary", "failed_gate_ids": []string{}}, decision)
	if !strings.Contains(modelBlocked, replyFaceModelSummary) || strings.Contains(modelBlocked, "准入门") {
		t.Fatalf("capability_boundary summary face broken: %q", modelBlocked)
	}

	// capability_boundary with neither gates nor a summary stays neutral.
	neutral := freeStateCapabilityBlockedReply(
		map[string]any{"boundary": "capability_boundary"}, &agentloop.FreeStateDecision{})
	if neutral != "自由态推理在能力边界停止。"+boundaryTail {
		t.Fatalf("capability_boundary neutral face = %q", neutral)
	}
}

// TestCapabilityBlockedBoundaryResponseProposalMissingFace pins the full
// response face for the forensics F4 turn: a loop settled blocked by the
// model's own concession, carrying the proposal_missing admission receipt.
func TestCapabilityBlockedBoundaryResponseProposalMissingFace(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-rfbl-1",
		ConversationID: "conversation-rfbl-1",
		GoalID:         "goal-rfbl-1",
		RunID:          "run-rfbl-1",
		OriginalIntent: "把人声轨的高频毛刺处理一下",
		CreatedAt:      now,
		UpdatedAt:      now,
		Status:         "blocked",
		LatestDecision: &agentloop.FreeStateDecision{
			Status:  agentloop.FreeStateCapabilityBlocked,
			Summary: replyFaceModelSummary,
		},
		AdmissionReceipt: proposalMissingReceipt("proposal_missing"),
	}
	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID,
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		// The model prose the pre-layering face suppressed — it must now ride
		// the summary face instead of a fabricated gate refusal.
		Reply: "我准备对 202 号轨做一次有界实验，执行后请直接 A/B 试听。",
	}
	resp, blocked := s.capabilityBlockedBoundaryResponse(loop.ConversationID, "default", res, loop)
	if !blocked {
		t.Fatal("proposal_missing blocked loop was not detected as the capability boundary")
	}
	if resp.StopReason != "capability_blocked" {
		t.Fatalf("boundary response stop_reason = %q", resp.StopReason)
	}
	assertBoundaryFaceReply(t, resp.Reply, true)
	if strings.Contains(resp.Reply, "A/B 试听") || strings.Contains(resp.Reply, "执行后请") {
		t.Fatalf("boundary reply leaked the model's promised experiment: %q", resp.Reply)
	}
	receipt, ok := resp.WorkflowData["free_state_admission_receipt"].(map[string]any)
	if !ok {
		t.Fatalf("workflow data lost the admission receipt: %+v", resp.WorkflowData)
	}
	if boundary := firstStringFromMap(receipt, "boundary"); boundary != "proposal_missing" {
		t.Fatalf("workflow receipt boundary = %q", boundary)
	}
	if resp.WorkflowData["capability_blocked"] != true || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("workflow data boundary markers wrong: %+v", resp.WorkflowData)
	}
}
