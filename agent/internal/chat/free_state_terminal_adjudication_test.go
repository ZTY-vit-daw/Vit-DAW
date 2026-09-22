package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
)

// FIX-F3-G4-SEMANTICS 方案乙, chat side: a terminal-locked loop whose complete
// proposal was refused only by the evidence-completeness gates (G3-G6) parks
// the proposal on the confirmation face with the open-dimensions disclosure
// instead of capability-blocking it. The agentloop already admitted the
// decision through its output gate (see the agentloop red test); the chat-side
// authoritative re-audit must reach the same parking verdict.

func terminalAdjudicationChatContext() map[string]any {
	return map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-7",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version":      "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
			"observation_ledger": map[string]any{
				"receipts": []any{
					map[string]any{
						"status": "rejected", "observation_id": "obs-mix", "requested_views": []any{"mix.frequency_relationship"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
					map[string]any{
						"status": "ready", "observation_id": "obs-target", "requested_views": []any{"track.timbre_frequency"},
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
				"available_views": map[string]any{
					"track:1007::track.timbre_frequency": map[string]any{
						"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-target",
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
			},
			// 方案甲 post-state: G4 passes via the spine-aligned OR arms (scan/
			// target evidence), so the reproducible sole evidence-completeness
			// rejection is G3 — the scan receipt is unusable.
			// (obs-mix is rejected below, after this literal.)
		},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7",
			"hypothesis_frontier": map[string]any{
				"candidate_id": "candidate-1",
				"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
			},
		},
	}
}

func terminalAdjudicationChatDecision() agentloop.FreeStateDecision {
	return agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "target evidence supports a bounded improvement hypothesis",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion:     agentprotocol.ImprovementProposalSchema,
			Target:            map[string]any{"kind": "track", "id": "1007"},
			EvidenceRefs:      []string{"obs-target"},
			ImprovementIntent: "make the bass relationship feel clearer",
			Hypothesis:        "a small bounded change may improve separation",
			ExpectedEffect:    "the relationship should be easier to compare",
			ActionDomain:      agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind:        "bounded_gain_adjustment",
			ParameterBounds:   map[string]any{"delta_db": -0.5},
			Confidence:        0.55,
		},
	}
}

func terminalAdjudicationChatLoop() freeStateReasoningLoop {
	return freeStateReasoningLoop{
		SchemaVersion:      freeStateReasoningLoopSchema,
		LoopID:             "loop-f3-adjudication",
		ConversationID:     "conv-f3-adjudication",
		Status:             "reasoning",
		OriginalIntent:     "improve the mix",
		ContinuationBudget: 6,
		ContinuationUsed:   5,
		TerminalTurnLocked: true,
		TerminalTurnReason: FreeStateTerminalReasonBudgetCritical,
		PriorityQueue: &audioclosure.PriorityQueue{
			SchemaVersion: audioclosure.PriorityQueueSchema,
			Entries: []audioclosure.PriorityQueueEntry{
				{Dimension: audioclosure.DimensionFrequencyOccupancy, PriorityReason: audioclosure.PriorityDefaultOrder, Status: audioclosure.QueueOpen},
				{Dimension: audioclosure.DimensionStereoSpace, PriorityReason: audioclosure.PriorityDefaultOrder, Status: audioclosure.QueueClosed},
			},
		},
	}
}

// adjudicationStringList reads a disclosure field as a string list whatever
// concrete slice shape the JSON round-trip left behind.
func adjudicationStringList(row map[string]any, key string) []string {
	switch typed := row[key].(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

// adjudicationRowList reads a disclosure row list whatever concrete slice
// shape the in-memory latch ([]map[string]any) or the durable JSON round-trip
// ([]any) left behind.
func adjudicationRowList(row map[string]any, key string) []map[string]any {
	switch typed := row[key].(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	}
	return nil
}

// The chat-side authoritative re-audit parks the terminal-locked
// evidence-completeness rejection instead of capability-blocking it, keeps the
// honest admission receipt, and constructs no experiment admission (the
// user's confirmation-surface decision is the adjudication).
func TestFreeStateTerminalAdjudicationParksProposalNotCapabilityBlocked(t *testing.T) {
	s := &Server{freeStateLoops: map[string]freeStateReasoningLoop{}}
	loop := terminalAdjudicationChatLoop()
	s.freeStateLoops[loop.ConversationID] = loop
	decision := terminalAdjudicationChatDecision()
	res := agentloop.Result{
		GoalID:            "goal-f3",
		RunID:             "run-f3",
		FreeStateDecision: &decision,
		ContextSnapshot:   terminalAdjudicationChatContext(),
	}
	stored, _ := s.recordFreeStateDecision(loop.ConversationID, res)
	if stored.Status != "awaiting_experiment" {
		t.Fatalf("parked terminal proposal must leave the loop awaiting_experiment, got %q (last_error=%q)", stored.Status, stored.LastError)
	}
	if stored.LatestDecision == nil || stored.LatestDecision.Status != agentloop.FreeStateNeedsExperiment {
		t.Fatalf("the parked decision must survive as the loop's latest decision, got %+v", stored.LatestDecision)
	}
	if stored.Experiment != nil {
		t.Fatal("parking must not construct an experiment admission (the user adjudicates at the confirmation face)")
	}
	if len(stored.TerminalAdjudication) == 0 {
		t.Fatal("parking must latch the terminal adjudication disclosure on the loop")
	}
	if ids := adjudicationStringList(stored.TerminalAdjudication, "failed_gate_ids"); len(ids) != 1 || ids[0] != "G3_project_scan" {
		t.Fatalf("adjudication disclosure failed_gate_ids = %v, want [G3_project_scan]", ids)
	}
	open := adjudicationRowList(stored.TerminalAdjudication, "open_dimensions")
	if len(open) != 1 {
		t.Fatalf("adjudication disclosure open_dimensions = %+v, want the single open queue dimension", open)
	}
	if firstStringFromMap(open[0], "dimension") != string(audioclosure.DimensionFrequencyOccupancy) {
		t.Fatalf("the open dimension must be the queue's open entry, got %+v", open[0])
	}
	if receipt := stored.AdmissionReceipt; len(receipt) == 0 || receipt["boundary"] != "admission_gate_failed" {
		t.Fatalf("the honest admission receipt must stay recorded, got %+v", receipt)
	}
}

// A non-terminal loop keeps today's capability_blocked semantics: parking is
// exclusive to the locked terminal turn.
func TestFreeStateGateRejectionOutsideTerminalLockStillCapabilityBlocks(t *testing.T) {
	s := &Server{freeStateLoops: map[string]freeStateReasoningLoop{}}
	loop := terminalAdjudicationChatLoop()
	loop.TerminalTurnLocked = false
	s.freeStateLoops[loop.ConversationID] = loop
	decision := terminalAdjudicationChatDecision()
	res := agentloop.Result{
		GoalID:            "goal-f3-open",
		RunID:             "run-f3-open",
		FreeStateDecision: &decision,
		ContextSnapshot:   terminalAdjudicationChatContext(),
	}
	stored, _ := s.recordFreeStateDecision(loop.ConversationID, res)
	if stored.Status != "capability_blocked" {
		t.Fatalf("unlocked gate rejection must keep the capability_blocked semantics, got %q", stored.Status)
	}
	if stored.LatestDecision == nil || stored.LatestDecision.Status != agentloop.FreeStateCapabilityBlocked {
		t.Fatalf("unlocked gate rejection must surface the blocked decision, got %+v", stored.LatestDecision)
	}
}

// The confirmation face carries the dimensions-not-closed disclosure in
// workflow_data (open_dimensions + terminal_adjudication), and a parked
// proposal is never auto-authorized away under full project access: the user
// rules on the gate-refused proposal personally.
func TestImprovementProposalResponseCarriesAdjudicationDisclosure(t *testing.T) {
	s := &Server{
		freeStateLoops:    map[string]freeStateReasoningLoop{},
		conversationGoals: map[string]string{},
		interactions:      map[string]PendingInteraction{},
	}
	loop := terminalAdjudicationChatLoop()
	loop.TerminalAdjudication = map[string]any{
		"schema_version":       "free_state_terminal_adjudication.v1",
		"failed_gate_ids":      []any{"G3_project_scan"},
		"open_dimensions":      []any{map[string]any{"dimension": string(audioclosure.DimensionFrequencyOccupancy), "status": "open"}},
		"terminal_turn_reason": FreeStateTerminalReasonBudgetCritical,
	}
	s.freeStateLoops[loop.ConversationID] = loop
	decision := terminalAdjudicationChatDecision()
	res := agentloop.Result{
		GoalID:            "goal-f3",
		RunID:             "run-f3",
		FreeStateDecision: &decision,
	}
	resp := s.improvementProposalResponse(loop.ConversationID, "default", res, map[string]any{"authority_mode": "full"})
	if !resp.NeedsConfirmation || resp.GoalStatus != "waiting_confirmation" {
		t.Fatalf("the parked proposal must park on the confirmation face even under full access, got needs_confirmation=%t status=%q stop=%q",
			resp.NeedsConfirmation, resp.GoalStatus, resp.StopReason)
	}
	if open, _ := resp.WorkflowData["open_dimensions"].([]any); len(open) != 1 {
		t.Fatalf("workflow_data.open_dimensions missing or wrong shape: %+v", resp.WorkflowData["open_dimensions"])
	}
	adj := firstMapFromAny(resp.WorkflowData["terminal_adjudication"])
	if firstStringFromMap(adj, "schema_version") != "free_state_terminal_adjudication.v1" {
		t.Fatalf("workflow_data.terminal_adjudication missing: %+v", adj)
	}
	if !strings.Contains(resp.Reply, "维度未闭合") {
		t.Fatalf("the reply must carry the dimensions-not-closed disclosure, got %q", resp.Reply)
	}
}
