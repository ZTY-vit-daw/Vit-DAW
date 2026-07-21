package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestRecordGoalResultPersistsExecutionMemoryAcrossCompletedGoals(t *testing.T) {
	s := &Server{
		conversationGoals:  map[string]string{},
		conversationMemory: map[string]agentloop.ExecutionMemory{},
		goalContinuations:  map[string]agentloop.Continuation{},
	}
	res := agentloop.Result{
		GoalID: "goal_import",
		RunID:  "run_import",
		Status: agentruntime.StatusCompleted,
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingTrackOrganization: map[string]any{
				"coverage_status":           "complete",
				"assignment_coverage_count": 61,
			},
		},
	}

	s.recordGoalResult("conversation_1", res)

	memory := s.agentLoopExecutionMemoryForConversation("conversation_1")
	if len(memory.PendingTrackOrganization) == 0 {
		t.Fatal("completed goal execution memory was not persisted for the conversation")
	}
	if memory.PendingTrackOrganization["assignment_coverage_count"] != 61 {
		t.Fatalf("persisted memory mismatch: %#v", memory.PendingTrackOrganization)
	}
	memory.PendingTrackOrganization["assignment_coverage_count"] = 20
	again := s.agentLoopExecutionMemoryForConversation("conversation_1")
	if again.PendingTrackOrganization["assignment_coverage_count"] != 61 {
		t.Fatalf("conversation memory should be cloned on read, got %#v", again.PendingTrackOrganization)
	}
}
