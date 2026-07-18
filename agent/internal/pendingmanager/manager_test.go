package pendingmanager

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

func TestMemoryManagerUpsertQueryTransition(t *testing.T) {
	manager := NewMemoryManager()
	pending := agentprotocol.PendingCandidate{
		ID:              "pending_1",
		Kind:            agentprotocol.KindPendingCandidate,
		TargetRef:       "track:vocal",
		CandidateType:   "mix_treatment",
		EvidenceRefs:    []string{"observation:obs_1"},
		NeedsResolution: []string{"plugin_profile"},
		Status:          agentprotocol.PendingStatusWaitingUser,
		Source: agentprotocol.Source{
			ConversationID: "chat_1",
			GoalID:         "goal_1",
		},
	}
	manager.Upsert(pending)
	if got, ok := manager.Get("pending_1"); !ok || got.TargetRef != "track:vocal" {
		t.Fatalf("Get = %+v ok=%v", got, ok)
	}
	pending.EvidenceRefs[0] = "mutated"
	pending.NeedsResolution[0] = "mutated"
	got, ok := manager.Get("pending_1")
	if !ok || got.EvidenceRefs[0] != "observation:obs_1" || got.NeedsResolution[0] != "plugin_profile" {
		t.Fatalf("manager did not clone pending refs/needs: %+v ok=%v", got, ok)
	}
	if rows := manager.ActiveForConversation("chat_1"); len(rows) != 1 || rows[0].ID != "pending_1" {
		t.Fatalf("ActiveForConversation = %+v", rows)
	}
	if rows := manager.ActiveForGoal("goal_1"); len(rows) != 1 || rows[0].ID != "pending_1" {
		t.Fatalf("ActiveForGoal = %+v", rows)
	}
	if rows := manager.ActiveForTarget("track:vocal"); len(rows) != 1 || rows[0].ID != "pending_1" {
		t.Fatalf("ActiveForTarget = %+v", rows)
	}
	updated, ok := manager.Transition("pending_1", agentprotocol.PendingStatusRevisionRequested, "user asked for a lighter move")
	if !ok || updated.Status != agentprotocol.PendingStatusRevisionRequested || updated.Source.Metadata["transition_reason"] == "" {
		t.Fatalf("Transition = %+v ok=%v", updated, ok)
	}
	manager.Transition("pending_1", agentprotocol.PendingStatusRejected, "")
	if rows := manager.ActiveForConversation("chat_1"); len(rows) != 0 {
		t.Fatalf("rejected pending should not be active: %+v", rows)
	}
}
