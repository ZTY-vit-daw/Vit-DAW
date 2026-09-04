package chat

import (
	"testing"
)

// AGENT-F5：调度侧切片没有 HTTP 响应通道，park 的交互此前对一切事件消费
// 者不可见（2026-09-04 手测：第 6 片 park 的确认卡双端浮不出）。park 必须
// 补发 interaction.pending；应答收口（checkpoint 退休）补发
// interaction.resolved，供 UI 即时浮卡/撤卡。
func TestSchedulerInteractionParkEmitsPendingAndResolvedEvents(t *testing.T) {
	s := testContinuationServer()
	item := DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: "cont_park_evt",
		TaskID: "task-park", GoalID: "goal-park", RunID: "run-park", ConversationID: "conversation-park-evt",
		CurrentSliceID: "slice-park", OriginalIntent: "inspect the project", Status: ContinuationClaimed,
	}
	s.durableContinuations[item.ContinuationID] = item
	pending := map[string]any{
		"status": "waiting_confirmation", "stop_reason": "improvement_proposal_confirmation_required",
		"interaction_id": "interaction-park-evt",
		"requests": []any{map[string]any{
			"id": "interaction-park-evt", "kind": "improvement_proposal_confirmation", "status": "waiting_for_user",
			"actions": []any{map[string]any{"id": "approve", "style": "primary", "recommended": true}},
		}},
	}
	s.parkClaimedContinuationAtInteraction(item, pending)

	events, _ := s.agentEventsSince(item.ConversationID, 0, 32)
	pendingSeen := false
	for _, event := range events {
		if event.Type != "interaction.pending" {
			continue
		}
		pendingSeen = true
		if firstStringFromMap(event.Payload, "interaction_id") != "interaction-park-evt" {
			t.Fatalf("pending event lost the interaction id: %+v", event.Payload)
		}
		if firstStringFromMap(event.Payload, "continuation_id") != item.ContinuationID {
			t.Fatalf("pending event lost the continuation id: %+v", event.Payload)
		}
		if event.GoalID != item.GoalID || event.ItemType != "interaction" {
			t.Fatalf("pending event identity is wrong: %+v", event)
		}
	}
	if !pendingSeen {
		t.Fatalf("scheduler interaction park did not emit interaction.pending: %d events", len(events))
	}

	s.completePendingInteractionContinuation(PendingInteraction{
		ID: "interaction-park-evt", ConversationID: item.ConversationID, GoalID: item.GoalID, RunID: item.RunID,
		Kind: "improvement_proposal_confirmation",
	})
	events, _ = s.agentEventsSince(item.ConversationID, 0, 32)
	resolvedSeen := false
	for _, event := range events {
		if event.Type != "interaction.resolved" {
			continue
		}
		resolvedSeen = true
		if firstStringFromMap(event.Payload, "interaction_id") != "interaction-park-evt" {
			t.Fatalf("resolved event lost the interaction id: %+v", event.Payload)
		}
	}
	if !resolvedSeen {
		t.Fatalf("answered interaction did not emit interaction.resolved: %d events", len(events))
	}
}

// 非 park 状态（已终态/已 park 过）不得重复发 pending 事件——防重复浮卡。
func TestSchedulerInteractionParkDoesNotEmitForSettledCheckpoint(t *testing.T) {
	s := testContinuationServer()
	item := DurableContinuation{
		SchemaVersion: continuationRuntimeSchema, ContinuationID: "cont_park_settled",
		TaskID: "task-park", GoalID: "goal-park", RunID: "run-park", ConversationID: "conversation-park-settled",
		CurrentSliceID: "slice-park", OriginalIntent: "inspect the project", Status: ContinuationCompleted,
	}
	s.durableContinuations[item.ContinuationID] = item
	s.parkClaimedContinuationAtInteraction(item, map[string]any{"interaction_id": "interaction-settled"})
	events, _ := s.agentEventsSince(item.ConversationID, 0, 32)
	for _, event := range events {
		if event.Type == "interaction.pending" {
			t.Fatalf("settled checkpoint must not emit a pending event: %+v", event)
		}
	}
}
