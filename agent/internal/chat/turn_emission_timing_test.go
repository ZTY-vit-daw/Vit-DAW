package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
)

// AGENT-F4 (2026-09-12): emission-timing pins for the chat event chain.
//
// Reconnaissance finding these tests lock in (see
// queue/reports/2026-09-12-AGENT-F4-emission-timing.md):
//
//  1. The transport turn.started of an HTTP chat turn is already emitted
//     pre-semantic-entry: handleChat emits it right after the goal/run identity
//     is bound, before runAgentLoopChat can run the semantic-entry classifier.
//     The AGENT-F1 companion trajectory.turn.started follows it immediately and
//     is sequence-numbered directly behind it.
//  2. The free-state experiment stage nodes (turn.started / intent.framed /
//     hypothesis.proposed / round.started / observation.recorded) are emitted
//     synchronously inside the single admission ingest - the emission point of
//     each node is the instant its own decision lands, not a later flush. A
//     future change that defers any of them fails these pins.
//
// Both properties are consumer-visible contracts (webui messageLifecycle +
// AGENT-F1 dedup), so they are asserted on the emitted AgentEvent stream, not
// on internal state.

// TestChatTurnStartedEmittedBeforeSemanticEntryForHTTPTurn pins finding 1: a
// plain HTTP chat turn publishes turn.started even though the semantic-entry
// classification that follows can never complete on this server (no LLM, no
// harness). The event therefore cannot depend on the semantic entry.
func TestChatTurnStartedEmittedBeforeSemanticEntryForHTTPTurn(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	conversationID := "chat_f4_pre_semantic"
	body, _ := json.Marshal(ChatRequest{
		ConversationID: conversationID,
		Message:        "帮我把工程整体混得更好一点",
		Context:        map[string]any{"agent_mode": agentModeDefault},
	})
	rec := httptest.NewRecorder()
	server.handleChat(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))

	events := chatTurnTrajectoryEvents(t, server, conversationID)
	if len(events) == 0 {
		t.Fatal("no agent events recorded for the turn")
	}
	// The turn transport event is the first thing the turn publishes.
	if events[0].Type != "turn.started" {
		t.Fatalf("first event type = %q, want turn.started (events=%+v)", events[0].Type, events)
	}
	if events[0].Status != "running" || events[0].GoalID == "" || events[0].RunID == "" {
		t.Fatalf("turn.started must carry the bound goal/run identity and running status: %+v", events[0])
	}
	if len(events) < 2 {
		t.Fatalf("AGENT-F1 companion missing: %+v", events)
	}
	companion := events[1]
	if companion.Type != string(trajectory.EventTurnStarted) || companion.ItemID != chatTurnTrajectoryNodeID(events[0].RunID) {
		t.Fatalf("companion event = %+v, want %s for node turn:%s", companion, trajectory.EventTurnStarted, events[0].RunID)
	}
	if companion.Payload["status"] != string(trajectory.StatusRunning) || companion.Payload["phase"] != "framing" {
		t.Fatalf("companion payload=%+v", companion.Payload)
	}
}

// freeStateAdmissionLoopForTiming builds the minimal live loop the admission
// path needs: one fresh model-requested CCB observation is the round-1 base.
func freeStateAdmissionLoopForTiming(conversationID string) freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-f4-timing", ConversationID: conversationID,
		GoalID: "goal-f4", RunID: "run-f4", Status: "reasoning", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "make the vocal more forward", ActiveIntent: "make the vocal more forward", MaxCycles: 6,
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	}
}

// TestFreeStateExperimentAdmissionStageNodesAreEmittedOnceInCanonicalOrder pins
// finding 2 for the admission burst. ORDER and SINGLE-SHOT are both part of the
// contract: the webui renders the stage rows in arrival order, and AGENT-F1's
// duplicate suppression only holds while each decision emits its node once.
func TestFreeStateExperimentAdmissionStageNodesAreEmittedOnceInCanonicalOrder(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	conversationID := "conversation-f4-order"
	loop := freeStateAdmissionLoopForTiming(conversationID)
	server.storeFreeStateLoop(loop)

	decision := agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "bounded hypothesis",
		ImprovementProposal: experimentTestProposal(), RequestedViewIDs: []string{"track.timbre_frequency"},
	}
	admitted, ok := server.recordFreeStateDecision(conversationID, agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, FreeStateDecision: &decision,
	})
	if !ok || admitted.Experiment == nil {
		t.Fatalf("experiment runtime not created: ok=%v loop=%+v", ok, admitted)
	}
	experimentTurnID := admitted.Experiment.ID
	if experimentTurnID != "turn:"+loop.LoopID {
		t.Fatalf("experiment turn id = %q, want turn:%s", experimentTurnID, loop.LoopID)
	}

	// Read the stream immediately after the single ingest returned: every stage
	// node must already be published by its own decision point.
	events := chatTurnTrajectoryEvents(t, server, conversationID)

	wantOrder := []trajectory.EventType{
		trajectory.EventTurnStarted,
		trajectory.EventIntentFramed,
		trajectory.EventHypothesisProposed,
		trajectory.EventRoundStarted,
		trajectory.EventObservationRecorded,
	}
	gotOrder := make([]string, 0, len(wantOrder))
	counts := map[string]int{}
	for _, event := range events {
		if _, pinned := indexOfStageType(wantOrder, event.Type); !pinned {
			continue
		}
		gotOrder = append(gotOrder, event.Type)
		counts[event.Type]++
	}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("stage nodes emitted = %v, want exactly %v (events=%+v)", gotOrder, wantOrder, events)
	}
	for i, want := range wantOrder {
		if gotOrder[i] != string(want) {
			t.Fatalf("stage node order = %v, want %v", gotOrder, wantOrder)
		}
		if counts[string(want)] != 1 {
			t.Fatalf("stage node %s emitted %d times, want 1 (AGENT-F1 single-shot)", want, counts[string(want)])
		}
	}

	// Every stage node is anchored on the experiment turn, keeps the transport
	// goal/run identity, and carries the versioned trajectory schema.
	for _, event := range events {
		if _, pinned := indexOfStageType(wantOrder, event.Type); !pinned {
			continue
		}
		if event.Payload["schema_version"] != trajectory.SchemaVersion {
			t.Fatalf("%s schema_version=%v want %s", event.Type, event.Payload["schema_version"], trajectory.SchemaVersion)
		}
		if event.Payload["turn_id"] != experimentTurnID {
			t.Fatalf("%s turn_id=%v want %s (event=%+v)", event.Type, event.Payload["turn_id"], experimentTurnID, event)
		}
		if event.TrajectoryTurnID != experimentTurnID || event.SourceTurnID != loop.RunID {
			t.Fatalf("%s anchoring turn_id=%q source_turn_id=%q want %q/%q",
				event.Type, event.TrajectoryTurnID, event.SourceTurnID, experimentTurnID, loop.RunID)
		}
		if event.GoalID != loop.GoalID || event.RunID != loop.RunID {
			t.Fatalf("%s goal/run = %q/%q want %q/%q", event.Type, event.GoalID, event.RunID, loop.GoalID, loop.RunID)
		}
		if event.CreatedAt == "" {
			t.Fatalf("%s published without a transport timestamp", event.Type)
		}
	}

	// The baseline (pre-action) observation node describes the round-1 base the
	// admission bound; a post-action node must not appear at this boundary.
	for _, event := range events {
		if event.Type != string(trajectory.EventObservationRecorded) {
			continue
		}
		details, _ := event.Payload["details"].(map[string]any)
		if details == nil || details["post_action"] != false {
			t.Fatalf("admission observation node details=%+v, want pre-action base", event.Payload["details"])
		}
	}
}

func indexOfStageType(want []trajectory.EventType, value string) (int, bool) {
	for i, candidate := range want {
		if string(candidate) == value {
			return i, true
		}
	}
	return -1, false
}

// TestFreeStateExperimentStageNodesAreNotDeferredToALaterFlush pins the other
// half of finding 2: nothing about the admission burst waits for a later slice,
// mix tick, or scheduler pass. A second read of the stream with no intervening
// server activity must be byte-identical in the stage-node sequence.
func TestFreeStateExperimentStageNodesAreNotDeferredToALaterFlush(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	conversationID := "conversation-f4-noflush"
	loop := freeStateAdmissionLoopForTiming(conversationID)
	server.storeFreeStateLoop(loop)

	if err := server.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	first := chatTurnTrajectoryEvents(t, server, conversationID)
	second := chatTurnTrajectoryEvents(t, server, conversationID)
	if len(first) != len(second) {
		t.Fatalf("stage nodes changed after the admission ingest returned: %d then %d", len(first), len(second))
	}
	stageTypes := []trajectory.EventType{
		trajectory.EventTurnStarted, trajectory.EventIntentFramed, trajectory.EventHypothesisProposed,
		trajectory.EventRoundStarted, trajectory.EventObservationRecorded,
	}
	seen := 0
	for _, event := range first {
		if _, pinned := indexOfStageType(stageTypes, event.Type); pinned {
			seen++
		}
	}
	if seen != len(stageTypes) {
		t.Fatalf("admission published %d/%d stage nodes synchronously (events=%+v)", seen, len(stageTypes), first)
	}
}
