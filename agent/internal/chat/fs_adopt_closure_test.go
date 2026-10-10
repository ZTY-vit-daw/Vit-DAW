package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/orchestrationcontroller"
)

// FS-ADOPT-CLOSURE-1 regression fixtures: the 2026-10-01 20:33–20:36 M1 live
// run (conversation webui_mupimj6f) — park → new input adopts by continuation
// (round.decision + settled=adopted_by_continuation landed, FS-PARK semantics
// held) → the next new goal (goal_65e6beee, "可以听听看drums音轨的clip低频怎么样吗")
// opened normally but settled immediately with zero observation, replying the
// StopTaskSettled canned sentence 「任务已经满足其持久化契约中的证据与结算条件。」.
// Mechanism: the adoption settles experiment+task (EventTaskSettled) but not
// the session audio closure; prepareAudioClosureContext bound the old active
// closure to the new goal; the run's projectAudioClosureTaskState +
// audioClosureSettleFromResult mapped the settled TaskState to StopTaskSettled
// and closed the turn. The two pins below nail the two fix layers separately;
// the legal same-goal StopTaskSettled presentation (explicit judgment POST
// shape) must not degrade.

// Layer 1 (source): the adoption settlement must close the session closure
// with the honest adoption reason and release the controller owner. Pre-fix
// red: the closure stayed non-terminal (fs7) and the owner stayed active.
func TestAdoptionSettlesSessionClosureAndReleasesOwner(t *testing.T) {
	server, goal, loop, closure := fsStopApplyFixtureServer(t)
	if !judgmentParkPendingRound(loop) {
		t.Fatal("setup: the fixture loop must park a pending judgment round")
	}
	if !server.settleJudgmentParkOnUserContinuation(fsStopApplyConversation, "可以听听看drums音轨的clip低频怎么样吗") {
		t.Fatal("setup: the continuation message must adopt the parked round")
	}
	settled, ok := server.audioClosures.Load(closure.ClosureID)
	if !ok || !settled.Terminal() {
		t.Fatalf("the adopted park must settle the session closure: ok=%v settlement=%+v", ok, settled.Settlement)
	}
	if settled.Settlement == nil || settled.Settlement.Reason != audioclosure.StopAdoptedByContinuation {
		t.Fatalf("adoption settlement reason = %+v, want adopted_by_continuation", settled.Settlement)
	}
	// 诚实边界：采纳≠问题解决——绝不落 satisfied / diagnostic_complete。
	if settled.Settlement.Reason == audioclosure.StopSatisfied || settled.Settlement.Reason == audioclosure.StopDiagnosticComplete {
		t.Fatalf("adoption must not fabricate a completion reason: %s", settled.Settlement.Reason)
	}
	if _, active := server.controllerOwners.Active(fsStopApplyConversation); active {
		t.Fatal("the controller owner must be released by the adoption settlement")
	}
	// The adopted goal itself stays closed with its task settled (FS-PARK pin,
	// re-asserted to prove this layer did not disturb the adoption semantics).
	if status := server.harness.RuntimeStatus(goal.GoalID).Status; !isTerminalGoalStatus(status) {
		t.Fatalf("the parked goal must stay closed after adoption: %s", status)
	}

	// The tail the live run never reached: the NEXT new goal runs on its own
	// fresh closure instead of inheriting the adopted one.
	newGoal := server.harness.BeginGoal("可以听听看drums音轨的clip低频怎么样吗")
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-fs-stop-apply-1", "project_revision": "rev-1",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		Controller:  string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := server.prepareAudioClosureContext(fsStopApplyConversation, "可以听听看drums音轨的clip低频怎么样吗", entryContext)
	if err != nil {
		t.Fatalf("the post-adoption goal must not inherit the old closure's conflict: %v", err)
	}
	if !active || newState.Terminal() || newState.GoalID != newGoal.GoalID || newState.ClosureID == closure.ClosureID {
		t.Fatalf("the post-adoption goal must run on its own fresh closure: active=%v goal=%q closure=%q", active, newState.GoalID, newState.ClosureID)
	}
}

// Layer 2 (binding guard): with the source layer bypassed (the exact pre-fix
// durable shape — the task settled, the closure still active, the owner still
// held), prepareAudioClosureContext must refuse to hand the finished task's
// closure to a different goal: it settles the orphaned closure, releases the
// owner, and lets the new goal open a fresh closure. Pre-fix red: the call
// returned the old closure bound (state.GoalID = the settled goal), which is
// the zero-execution StopTaskSettled turn of the live run.
func TestPrepareAudioClosureContextRefusesFinishedTaskClosureForNewGoal(t *testing.T) {
	server, goal, loop, closure := fsStopApplyFixtureServer(t)
	// The pre-fix adoption shape, source layer aside: the canonical task
	// settles while the closure (and its owner) stay untouched.
	if err := server.settleTaskFromExperiment(&loop, judgmentParkAdoptionSummary, freeStateExperimentEvidence(loop.Experiment)); err != nil {
		t.Fatal(err)
	}
	server.harness.CompleteGoal(goal.GoalID, nil)
	if task := server.harness.RuntimeStatus(goal.GoalID).Task; task == nil || task.SemanticState == nil || !task.SemanticState.Terminal {
		t.Fatalf("setup: the fixture task must sit in a terminal state: %+v", task)
	}

	newGoal := server.harness.BeginGoal("可以听听看drums音轨的clip低频怎么样吗")
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-fs-stop-apply-1", "project_revision": "rev-1",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		Controller:  string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := server.prepareAudioClosureContext(fsStopApplyConversation, "可以听听看drums音轨的clip低频怎么样吗", entryContext)
	if err != nil {
		t.Fatalf("the guard must unwedge the new goal instead of failing it: %v", err)
	}
	// Pre-fix red face: the old closure came back bound to the new goal.
	if newState.ClosureID == closure.ClosureID || newState.GoalID == goal.GoalID {
		t.Fatalf("a finished task's closure must not become a new goal's working closure: closure=%q goal=%q", newState.ClosureID, newState.GoalID)
	}
	if !active || newState.Terminal() || newState.GoalID != newGoal.GoalID {
		t.Fatalf("the new goal must open its own fresh closure: active=%v goal=%q", active, newState.GoalID)
	}
	// The guard settled the orphan honestly and released the owner.
	orphanned, ok := server.audioClosures.Load(closure.ClosureID)
	if !ok || !orphanned.Terminal() {
		t.Fatalf("the guard must settle the finished task's closure: ok=%v settlement=%+v", ok, orphanned.Settlement)
	}
	if orphanned.Settlement != nil && orphanned.Settlement.Reason == audioclosure.StopSatisfied {
		t.Fatalf("the guard must not fabricate a satisfied settle: %+v", orphanned.Settlement)
	}
	// The guard released the old owner; the fresh intake then acquires the
	// conversation for the NEW closure — the active owner must never still be
	// the finished task's closure.
	if owner, ownerActive := server.controllerOwners.Active(fsStopApplyConversation); ownerActive && owner.ControllerID == closure.ClosureID {
		t.Fatal("the guard must release the finished task's controller owner (the fresh closure owns the conversation now)")
	}
}

// The legal sibling (card pin d): the same goal re-entering its own closure
// with a settled task is the honest StopTaskSettled presentation (the explicit
// audition-judgment shape) — the guard must not take that binding away.
func TestPrepareAudioClosureContextKeepsSameGoalSettledTaskBinding(t *testing.T) {
	server, goal, loop, _ := fsStopApplyFixtureServer(t)
	if err := server.settleTaskFromExperiment(&loop, judgmentParkAdoptionSummary, freeStateExperimentEvidence(loop.Experiment)); err != nil {
		t.Fatal(err)
	}
	_, state, active, err := server.prepareAudioClosureContext(fsStopApplyConversation, "继续", map[string]any{
		"goal_id": goal.GoalID, "run_id": goal.RunID,
		"project_uuid": "project-fs-stop-apply-1", "project_revision": "rev-1",
	})
	if err != nil {
		t.Fatalf("the same-goal binding must keep working: %v", err)
	}
	if !active || state.GoalID != goal.GoalID {
		t.Fatalf("the same goal must keep its own closure bound: active=%v goal=%q", active, state.GoalID)
	}
	if state.Terminal() {
		t.Fatal("the same-goal binding must not settle the closure at bind time (the run path settles it with StopTaskSettled)")
	}
}

// The new stop reason's user-facing wording states the adoption honestly —
// never the StopTaskSettled canned sentence of the defect, never a
// satisfaction claim.
func TestAdoptedByContinuationSettlementReplyWording(t *testing.T) {
	reply := audioClosureSettlementReply(&audioclosure.Settlement{Reason: audioclosure.StopAdoptedByContinuation}, -1, freeStateCandidateSupply{})
	if reply == "" || reply == audioClosureSettlementReply(&audioclosure.Settlement{Reason: audioclosure.StopTaskSettled}, -1, freeStateCandidateSupply{}) {
		t.Fatalf("adopted_by_continuation must carry its own wording, got %q", reply)
	}
	if containsAny(reply, []string{"已经满足", "完成闭环"}) {
		t.Fatalf("the adoption wording must not claim completion: %q", reply)
	}
	if !strings.Contains(reply, "采纳") {
		t.Fatalf("the adoption wording must state the adoption itself: %q", reply)
	}
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
