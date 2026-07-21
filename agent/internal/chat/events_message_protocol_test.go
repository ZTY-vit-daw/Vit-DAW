package chat

import "testing"

func TestAgentEventProtocolIsTransientByDefault(t *testing.T) {
	server := &Server{}
	event := server.emitAgentEvent("conversation-1", AgentEvent{
		Type:     "item.started",
		GoalID:   "goal-1",
		RunID:    "run-1",
		ItemID:   "item-1",
		ItemType: "daw_action",
		Status:   "running",
	})
	if event.Lifecycle != "transient" || event.Persistence != "none" || event.MessageKind != "activity" {
		t.Fatalf("event protocol = %#v", event)
	}
	if event.TurnID != "run-1" || event.LogicalMessageID == "" {
		t.Fatalf("event identity = %#v", event)
	}
}

func TestChatResponseProtocolUsesDurableHistoryIdentity(t *testing.T) {
	resp := ChatResponse{
		GoalID:            "goal-2",
		RunID:             "run-2",
		Reply:             "B2 proposal",
		NeedsConfirmation: true,
		ProjectHistory: map[string]any{
			"conversation_messages": []map[string]any{
				{"role": "assistant", "content": "B2 proposal", "node_id": "node-2", "logical_message_id": "proposal-2"},
			},
		},
	}
	ensureChatResponseMessageProtocol(&resp)
	syncChatResponseMessageIdentityFromHistory(&resp)
	if resp.Lifecycle != "durable" || resp.Persistence != "project_history" || resp.MessageKind != "proposal" {
		t.Fatalf("response protocol = %#v", resp)
	}
	if resp.TurnID != "run-2" || resp.LogicalMessageID != "proposal-2" {
		t.Fatalf("response identity = %#v", resp)
	}
}

func TestCapabilityExecutionResponseUsesVerificationMessageKind(t *testing.T) {
	resp := ChatResponse{
		Reply:    "B2 executed and verified",
		Workflow: "capability_runtime_v1",
		WorkflowData: map[string]any{
			"canary_stage":        "executed_verified",
			"execution_id":        "execution-1",
			"verification_result": map[string]any{"status": "pass"},
		},
	}
	ensureChatResponseMessageProtocol(&resp)
	if resp.MessageKind != "verification" {
		t.Fatalf("message kind = %q, want verification", resp.MessageKind)
	}
}
