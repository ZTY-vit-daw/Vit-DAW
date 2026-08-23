package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/audioclosure"
)

// Production wiring negatives for the no_candidate_found necessity check
// (docs/FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md): while the
// diagnostic priority queue still has an open dimension, no_candidate_found is
// rejected; a malformed queue fails closed (treated as still open).

func noCandidateDecision(summary string) messageLoopOutput {
	return messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNoCandidateFound, EvidenceStatus: "sufficient",
		Summary:    summary,
		Diagnostic: &FreeStateDiagnostic{SchemaVersion: FreeStateDiagnosticSchema, Status: "unresolved", Findings: nil, Limitations: []string{"queue exhausted"}},
	}}
}

func queueStillOpenState(queue any) *runState {
	return &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect the project",
			"priority_queue": queue,
		},
	}}}
}

// productionShapedQueue mirrors the queue the chat loop derives from the
// closure's diagnostic rounds: one dimension closed, one skipped, rest open.
func productionShapedQueue() map[string]any {
	entries := []any{}
	for _, dim := range audioclosure.DefaultDimensionOrder {
		entry := map[string]any{"dimension": string(dim), "priority_reason": "default_order", "status": "open"}
		switch dim {
		case audioclosure.DimensionLevelHeadroom:
			entry["status"] = "closed"
		case audioclosure.DimensionStereoSpace:
			entry["status"] = "skipped"
			entry["skip_reason"] = "not_applicable"
		}
		entries = append(entries, entry)
	}
	return map[string]any{"schema_version": audioclosure.PriorityQueueSchema, "entries": entries}
}

func TestNoCandidateRejectedWhileQueueHasOpenDimension(t *testing.T) {
	state := queueStillOpenState(productionShapedQueue())
	if !messageLoopFreeStateQueueStillOpen(state) {
		t.Fatal("queue with open dimensions reported exhausted")
	}
	issue := messageLoopFreeStateOutputIssue(state, noCandidateDecision("the bounded diagnostic covered the searched dimensions and left the rest open"))
	if !strings.Contains(issue, "still has open dimensions") {
		t.Fatalf("no_candidate_found accepted while queue open: %q", issue)
	}

	// Once every dimension is closed or skipped the necessity check passes.
	exhausted := productionShapedQueue()
	for _, raw := range exhausted["entries"].([]any) {
		entry := raw.(map[string]any)
		if entry["status"] == "open" {
			entry["status"] = "closed"
		}
	}
	done := queueStillOpenState(exhausted)
	if messageLoopFreeStateQueueStillOpen(done) {
		t.Fatal("fully closed queue reported open")
	}
	if issue := messageLoopFreeStateOutputIssue(done, noCandidateDecision("the bounded diagnostic covered the searched dimensions and left the rest open")); issue != "" {
		t.Fatalf("no_candidate_found with exhausted queue rejected: %q", issue)
	}
}

func TestMalformedQueueFailsClosed(t *testing.T) {
	// Missing entries: unparseable/invalid queue cannot prove exhaustion.
	if !messageLoopFreeStateQueueStillOpen(queueStillOpenState(map[string]any{"schema_version": audioclosure.PriorityQueueSchema})) {
		t.Fatal("entry-less queue treated as exhausted")
	}
	// A skip without an enum reason invalidates the queue.
	badSkip := productionShapedQueue()
	for _, raw := range badSkip["entries"].([]any) {
		entry := raw.(map[string]any)
		if entry["status"] == "skipped" {
			delete(entry, "skip_reason")
		}
	}
	if !messageLoopFreeStateQueueStillOpen(queueStillOpenState(badSkip)) {
		t.Fatal("queue with reason-less skip treated as exhausted")
	}
	// Non-object queue payload.
	if !messageLoopFreeStateQueueStillOpen(queueStillOpenState("not-a-queue")) {
		t.Fatal("non-object queue payload treated as exhausted")
	}
	// No queue tracked at all stays admissible (legacy loops).
	if messageLoopFreeStateQueueStillOpen(&runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "inspect"},
	}}}) {
		t.Fatal("absent queue treated as open")
	}
}
