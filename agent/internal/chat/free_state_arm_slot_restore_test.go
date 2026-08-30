package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
)

// D2-2-S3h7: the judgment-driven arm slot versus the workspace restore's
// legacy migration. The 20260830_093308 terminal run armed the round-2 driver
// inside the judgment POST (an ID-less internal-resume continuation — the arm
// deliberately writes no durable record; the first driven slice's
// recordGoalResult creates it), the POST persisted the snapshot, and the
// scheduler cycle's disk reload then ran the pre-C legacy migration over that
// snapshot. The migration found no durable record for the ID-less armed
// continuation, classified it as a legacy checkpoint, and materialized it into
// a waiting_interaction durable whose only interaction surface is the
// unanswerable "legacy_waiting_continue" marker. That shell occupied the arm
// slot and every later nudge died at pending_interaction_requires_response
// without reaching a model turn — round 2 starved before its first model turn.

// TestWorkspaceRestorePreservesArmedRecalibrationContinuation locks the root
// fix: the legacy migration must not shell-ify a modern armed internal resume.
// The armed continuation crosses the restore boundary as the compatibility
// index entry it was persisted as, still drivable by the ordinary continue
// nudge, with no fabricated durable record.
func TestWorkspaceRestorePreservesArmedRecalibrationContinuation(t *testing.T) {
	armed := agentloop.Continuation{
		GoalID: "goal-d2", RunID: "run-d2", UserText: "improve the mix",
		Context: map[string]any{
			"conversation_id": "conversation-d2", "goal_id": "goal-d2", "run_id": "run-d2",
			"free_state_internal_resume": true, "free_state_route_authorized": true,
		},
	}
	restored := &Server{}
	restored.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		GoalContinuations: map[string]agentloop.Continuation{"goal-d2": armed},
		ConversationGoals: map[string]string{"conversation-d2": "goal-d2"},
	})
	kept, ok := restored.goalContinuations["goal-d2"]
	if !ok || kept.UserText != armed.UserText || !contextBool(kept.Context, "free_state_internal_resume") {
		t.Fatalf("restore lost the armed continuation: kept=%+v ok=%v", kept, ok)
	}
	if shell, exists := restored.durableContinuations[continuationIDForContinuation(kept)]; exists {
		t.Fatalf("restore fabricated a durable record for the armed continuation: %+v", shell)
	}
	// The nudge's interaction gate scans durable waiting_interaction records;
	// with no shell materialized the bare continue must fall through to the
	// resumable slot instead of dying at an unanswerable boundary.
	if pending, parked := restored.interactionContinuationForConversation("conversation-d2"); parked {
		t.Fatalf("restored snapshot parks the armed round behind an interaction: %+v", pending)
	}
}

// TestWorkspaceRestoreStillMaterializesLegacyCheckpointShells locks the red
// line: pre-C legacy checkpoints (no internal-resume marker) keep their exact
// migration shape — the durable shell exists so a restarted agent can still
// surface the legacy boundary, and the compatibility index keeps the slot.
func TestWorkspaceRestoreStillMaterializesLegacyCheckpointShells(t *testing.T) {
	legacy := agentloop.Continuation{GoalID: "goal-legacy", RunID: "run-legacy", UserText: "legacy settle checkpoint"}
	restored := &Server{}
	restored.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		GoalContinuations: map[string]agentloop.Continuation{"goal-legacy": legacy},
		ConversationGoals: map[string]string{"conversation-legacy": "goal-legacy"},
	})
	id := continuationIDForContinuation(legacy)
	shell, exists := restored.durableContinuations[id]
	if !exists || shell.Status != ContinuationWaitingInteraction ||
		firstStringFromMap(shell.PendingInteraction, "status") != "legacy_waiting_continue" {
		t.Fatalf("legacy checkpoint migration changed shape: shell=%+v exists=%v", shell, exists)
	}
	if shell.ConversationID != "conversation-legacy" {
		t.Fatalf("legacy shell lost its conversation binding: %+v", shell)
	}
	if cont, ok := restored.goalContinuations["goal-legacy"]; !ok || cont.GoalID != "goal-legacy" {
		t.Fatalf("legacy compatibility index lost the slot: cont=%+v ok=%v", cont, ok)
	}
}

// TestArmDisplacesUndrivableShellOccupantAndSurvivesReload locks the arm-side
// half: when the slot's occupant is an unanswerable legacy shell, arming the
// freshly opened recalibration round displaces it (instead of the historical
// "slot exists, yield" refusal), terminalizes the displaced shell durable so a
// later disk reload cannot refill the slot from it, and the armed driver
// survives the reload round-trip.
func TestArmDisplacesUndrivableShellOccupantAndSurvivesReload(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	driveNoDifferenceRecalibration(t, s, &loop)
	shell := agentloop.Continuation{GoalID: loop.GoalID, RunID: loop.RunID, UserText: "legacy settle checkpoint"}
	shellID := continuationIDForContinuation(shell)
	now := time.Now().UTC()
	s.mu.Lock()
	s.goalContinuations[loop.GoalID] = shell
	s.durableContinuations[shellID] = DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: shellID,
		GoalID: loop.GoalID, ConversationID: loop.ConversationID, RunID: loop.RunID,
		Continuation: shell, Status: ContinuationWaitingInteraction,
		PendingInteraction: map[string]any{
			"status": "legacy_waiting_continue",
			"reason": "legacy checkpoint has no authoritative stop reason",
		},
		CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Unlock()

	s.armFreeStateRecalibrationContinuation(&loop)
	cont, ok := s.goalContinuationForConversation(loop.ConversationID)
	if !ok || !contextBool(cont.Context, "free_state_internal_resume") {
		t.Fatalf("arm yielded to an unanswerable shell occupant: cont=%+v ok=%v", cont, ok)
	}
	displaced, exists := s.durableContinuations[shellID]
	if !exists || displaced.Status == ContinuationWaitingInteraction {
		t.Fatalf("displaced shell durable stayed parkable: status=%+v exists=%v", displaced.Status, exists)
	}

	// Reload round-trip: the armed driver must cross the restore boundary and
	// no unanswerable boundary may shadow the conversation's continue path.
	s.mu.Lock()
	snapshot := s.projectAgentRuntimeStateLocked()
	s.mu.Unlock()
	reloaded := &Server{}
	reloaded.restoreProjectAgentRuntimeStateLocked(snapshot)
	kept, keptOK := reloaded.goalContinuations[loop.GoalID]
	if !keptOK || !contextBool(kept.Context, "free_state_internal_resume") {
		t.Fatalf("armed driver did not survive the reload: kept=%+v ok=%v", kept, keptOK)
	}
	if pending, parked := reloaded.interactionContinuationForConversation(loop.ConversationID); parked {
		t.Fatalf("reload re-parked the conversation behind an interaction: %+v", pending)
	}
}

// TestArmNeverDisplacesDrivableOccupant locks the arm red line: an occupant
// carrying a live resumable checkpoint (the scheduler's pending durable) is
// drivable and must never be clobbered by a recalibration arm.
func TestArmNeverDisplacesDrivableOccupant(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	driveNoDifferenceRecalibration(t, s, &loop)
	drivable := agentloop.Continuation{
		GoalID: loop.GoalID, RunID: loop.RunID, SliceID: "slice-live", TurnID: "turn-live",
		UserText: "live checkpoint", OriginalIntent: loop.OriginalIntent,
	}
	drivableID := continuationIDForContinuation(drivable)
	now := time.Now().UTC()
	s.mu.Lock()
	s.goalContinuations[loop.GoalID] = drivable
	s.durableContinuations[drivableID] = DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: drivableID,
		GoalID: loop.GoalID, ConversationID: loop.ConversationID, RunID: loop.RunID,
		CurrentSliceID: "slice-live", Continuation: drivable, Status: ContinuationPending,
		CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Unlock()

	s.armFreeStateRecalibrationContinuation(&loop)
	if kept := s.goalContinuations[loop.GoalID]; kept.SliceID != "slice-live" || kept.UserText != "live checkpoint" {
		t.Fatalf("arm clobbered a drivable continuation: %+v", kept)
	}
}

// TestContinuationOccupantDrivablePredicate locks the named CLEAN1 seed
// predicate itself: an arm-slot occupant is drivable exactly when it carries a
// resumable checkpoint or parks at an interaction a user can actually answer;
// unanswerable legacy shells and terminal residue are the displaceable forms.
func TestContinuationOccupantDrivablePredicate(t *testing.T) {
	occupant := agentloop.Continuation{GoalID: "goal-x", RunID: "run-x"}
	cases := []struct {
		name   string
		durable *DurableContinuation
		want   bool
	}{
		{"no durable record (armed internal resume)", nil, true},
		{"pending checkpoint", &DurableContinuation{Status: ContinuationPending}, true},
		{"claimed checkpoint", &DurableContinuation{Status: ContinuationClaimed}, true},
		{"running checkpoint", &DurableContinuation{Status: ContinuationRunning}, true},
		{
			"answerable waiting interaction",
			&DurableContinuation{Status: ContinuationWaitingInteraction,
				PendingInteraction: map[string]any{"status": "waiting_confirmation", "interaction_id": "ix-1"}},
			true,
		},
		{
			"unanswerable legacy shell",
			&DurableContinuation{Status: ContinuationWaitingInteraction,
				PendingInteraction: map[string]any{"status": "legacy_waiting_continue"}},
			false,
		},
		{"completed residue", &DurableContinuation{Status: ContinuationCompleted}, false},
		{"cancelled residue", &DurableContinuation{Status: ContinuationCancelled}, false},
		{"failed residue", &DurableContinuation{Status: ContinuationFailed}, false},
	}
	for _, tc := range cases {
		if got := continuationOccupantDrivable(occupant, tc.durable); got != tc.want {
			t.Fatalf("%s: continuationOccupantDrivable=%v want %v", tc.name, got, tc.want)
		}
	}
}
