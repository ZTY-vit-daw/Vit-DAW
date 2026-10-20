package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FS-BLOCKED-SURFACE-CLOSURE-1: with the closure active, the settled turn used
// to return audioClosureResponse before the capability_blocked boundary check
// ran, so a loop settled at the boundary reached the user through the
// closure's face — goal_status=completed beside a capability_blocked fs_loop
// (run 203348 shape). These tests pin the closure-active seam of
// settledTurnFaceResponse: the boundary face owns the presentation whenever
// the blocked receipt is settled, the closure view still rides the response,
// and the closure settlement face never masquerades a blocked turn as a clean
// completion. The plain-branch boundary contract itself stays pinned by
// free_state_capability_blocked_surface_test.go, untouched.

// blockedClosureSurfaceServer is the union both faces need: the closure store
// and controller registry from the closure controller tests, plus the
// continuation maps the boundary response's durable envelope path writes.
func blockedClosureSurfaceServer() *Server {
	s := audioClosureTestServer()
	s.goalContinuations = map[string]agentloop.Continuation{}
	s.durableContinuations = map[string]DurableContinuation{}
	s.conversationGoals = map[string]string{}
	s.conversationMemory = map[string]agentloop.ExecutionMemory{}
	return s
}

// blockedClosureFixtureState rebuilds the R3 blocked settle on the given
// server through the production recorder: the fixture loop is stored, the
// gate-refused decision is folded in, and the stored loop must have settled at
// capability_blocked.
func blockedClosureFixtureState(t *testing.T, s *Server, loop freeStateReasoningLoop, res agentloop.Result) freeStateReasoningLoop {
	t.Helper()
	s.storeFreeStateLoop(loop)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("gate-refused decision was not retained by recordFreeStateDecision")
	}
	if stored.Status != "capability_blocked" {
		t.Fatalf("gate refusal did not settle the loop at capability_blocked: %q", stored.Status)
	}
	return stored
}

// The pin: closure active + blocked receipt settled → the boundary face owns
// the response, with the closure view bound and no settlement masquerade.
func TestCapabilityBlockedBoundaryOwnsClosureActiveSettledFace(t *testing.T) {
	_, loop, res := gateRefusedFreeStateFixture(t)
	s := blockedClosureSurfaceServer()
	_, closureState, active, err := s.prepareAudioClosureContext(loop.ConversationID, loop.OriginalIntent, audioClosureTestContext())
	if err != nil || !active {
		t.Fatalf("closure did not activate: active=%v err=%v", active, err)
	}
	stored := blockedClosureFixtureState(t, s, loop, res)
	resp := s.settledTurnFaceResponse(loop.ConversationID, "default", res, stored, true, closureState)
	// Face 1: the machine-readable stop reason, same contract as the plain
	// branch's boundary surface.
	if resp.StopReason != "capability_blocked" {
		t.Fatalf("closure-active boundary response stop_reason = %q, want capability_blocked", resp.StopReason)
	}
	// Face 2: the workflow data carries the boundary markers and the full
	// admission receipt.
	if resp.WorkflowData["capability_blocked"] != true || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("workflow data boundary markers wrong: %+v", resp.WorkflowData)
	}
	receipt, ok := resp.WorkflowData["free_state_admission_receipt"].(map[string]any)
	if !ok {
		t.Fatalf("workflow data lost the admission receipt: %+v", resp.WorkflowData)
	}
	joined := strings.Join(gateReceiptFailedIDs(t, receipt), ",")
	if !strings.Contains(joined, "G6_target_evidence") || !strings.Contains(joined, "G8_target_consistency") {
		t.Fatalf("workflow receipt lost the R3 gate signature: %q", joined)
	}
	// Face 3: the reply is the capability boundary statement — it names the
	// refusal in user language, never delivers the model's promise, and never
	// borrows the closure settlement's converged voice. SEMANTIC-CARD-COPY-1④:
	// gate codenames stay in the structured receipt, not the prose.
	if !strings.Contains(resp.Reply, "准入门") || !strings.Contains(resp.Reply, "目标证据") || !strings.Contains(resp.Reply, "目标一致性") {
		t.Fatalf("boundary reply does not state the admission refusal in user language: %q", resp.Reply)
	}
	for _, id := range gateReceiptFailedIDs(t, receipt) {
		if strings.Contains(resp.Reply, id) {
			t.Fatalf("boundary reply leaked internal gate codename %q: %q", id, resp.Reply)
		}
	}
	if !strings.Contains(resp.Reply, "没有执行任何实验或修改") || !strings.Contains(resp.Reply, "不会自动重试") {
		t.Fatalf("boundary reply does not state the no-execution/no-retry boundary: %q", resp.Reply)
	}
	for _, masquerade := range []string{"A/B 试听", "执行后请", "已收敛"} {
		if strings.Contains(resp.Reply, masquerade) {
			t.Fatalf("boundary reply masquerades the blocked turn with %q: %q", masquerade, resp.Reply)
		}
	}
	// The closure view still rides the response so the frontend keeps seeing
	// the closure it is tracking.
	if _, bound := resp.WorkflowData[audioClosureContextKey]; !bound {
		t.Fatalf("closure view was dropped from the boundary face: %+v", resp.WorkflowData)
	}
	if resp.Workflow == "minimal_audio_closure" {
		t.Fatalf("blocked turn was presented through the closure settlement face: workflow=%q reply=%q", resp.Workflow, resp.Reply)
	}
	// The durable envelope contract stays untouched — identical to the plain
	// branch (recordGoalResult already persisted the same envelope).
	if resp.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("boundary response changed the durable envelope status: %q", resp.GoalStatus)
	}
}

// The inverse control at the same seam: without a settled blocked receipt the
// closure face stays — the boundary must not hijack a plainly settled turn
// (the plain_completed_diagnostic negative shape, recomposed with closure
// activation).
func TestClosureActiveSettledFaceWithoutBlockedReceiptKeepsClosureFace(t *testing.T) {
	s := blockedClosureSurfaceServer()
	conversation := "conversation-closure-plain-face"
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-closure-plain-face",
		ConversationID: conversation,
		GoalID:         "goal-closure-plain-face",
		RunID:          "run-closure-plain-face",
		Status:         "completed",
		OriginalIntent: "把人声轨的高频毛刺处理一下",
		LatestDecision: &agentloop.FreeStateDecision{Status: agentloop.FreeStateDiagnosticComplete},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if _, blocked := freeStateCapabilityBlockedBoundary(loop); blocked {
		t.Fatal("plain completed diagnostic must not be detected as the blocked boundary")
	}
	_, closureState, active, err := s.prepareAudioClosureContext(conversation, loop.OriginalIntent, audioClosureTestContext())
	if err != nil || !active {
		t.Fatalf("closure did not activate: active=%v err=%v", active, err)
	}
	res := agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusCompleted, StopReason: agentloop.StopReasonDone}
	resp := s.settledTurnFaceResponse(conversation, "default", res, loop, true, closureState)
	if resp.StopReason == "capability_blocked" {
		t.Fatalf("plain settled turn was hijacked by the boundary face: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.WorkflowData["capability_blocked"] != nil {
		t.Fatalf("plain settled turn carries boundary markers: %+v", resp.WorkflowData)
	}
	if _, bound := resp.WorkflowData[audioClosureContextKey]; !bound {
		t.Fatalf("closure view was lost from the closure face: %+v", resp.WorkflowData)
	}
}

// The documented overlap: the round recording settled the closure terminal in
// the same turn the loop settled blocked. The settlement face must still not
// own the presentation — the boundary face wins with the terminal closure
// state bound (settleFreeStateLoopFromClosure no-ops on the already-terminal
// blocked loop, so the skipped mirroring costs nothing durable).
func TestBlockedBoundaryWinsOverTerminalClosureSettlementFace(t *testing.T) {
	_, loop, res := gateRefusedFreeStateFixture(t)
	s := blockedClosureSurfaceServer()
	ctx, state, active, err := s.prepareAudioClosureContext(loop.ConversationID, loop.OriginalIntent, audioClosureTestContext())
	if err != nil || !active {
		t.Fatalf("closure did not activate: active=%v err=%v", active, err)
	}
	for round := 0; round < 2; round++ {
		var admitted bool
		state, admitted, err = s.admitAudioClosureRound(state)
		if err != nil || !admitted {
			t.Fatalf("round %d admission: admitted=%v err=%v", round+1, admitted, err)
		}
		state, err = s.recordAudioClosureRound(state, agentloop.Result{}, ctx)
		if err != nil {
			t.Fatalf("round %d record: %v", round+1, err)
		}
	}
	if !state.Terminal() {
		t.Fatalf("precondition failed: closure did not settle terminal: %+v", state)
	}
	stored := blockedClosureFixtureState(t, s, loop, res)
	resp := s.settledTurnFaceResponse(loop.ConversationID, "default", res, stored, true, state)
	if resp.StopReason != "capability_blocked" {
		t.Fatalf("terminal-closure overlap lost the boundary face: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.Workflow == "minimal_audio_closure" {
		t.Fatalf("terminal settlement face owned the blocked turn: workflow=%q reply=%q", resp.Workflow, resp.Reply)
	}
	if _, bound := resp.WorkflowData[audioClosureContextKey]; !bound {
		t.Fatalf("terminal closure view was dropped from the boundary face: %+v", resp.WorkflowData)
	}
	if !strings.Contains(resp.Reply, "没有执行任何实验或修改") {
		t.Fatalf("terminal-closure overlap reply is not the boundary statement: %q", resp.Reply)
	}
}
