package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	agentruntime "vit-daw-agent/internal/runtime"
)

// M3-CHAT-HOST-CARRY-FIX-1 (2026-11-04 forensics 卡 245). The chat host's
// typed round-trip at the run boundary silently dropped the agentloop-side
// pinned `automation_surfaces` rows (the struct had no field), so a terminal
// turn rebuilt after limit_reached -> checkpoint -> resume rendered without
// the parameter disclosure; and a scheduler-driven terminal slice's outcome
// never reached the chat face while the bare-continue swallow kept answering
// "already queued" with zero state. These tests pin the three fixes.

func carryAutomationSurfaceRow(pluginID, trackID, revision string) map[string]any {
	return map[string]any{
		"plugin_id":        pluginID,
		"track_id":         trackID,
		"plugin_name":      "TestEQ",
		"params":           []map[string]any{{"param_id": "Volume", "min": 0.0, "max": 1.0, "value": 0.740818}},
		"round":            3,
		"project_revision": revision,
	}
}

// 面一 ①: round-trip keeps the key — a transport map carrying the pinned rows
// survives freeStateLoopFromAny (typed field present) and freeStateLoopMap
// writes the key back, and cloneFreeStateLoop deep-copies the rows.
func TestFreeStateLoopRoundTripCarriesAutomationSurfaces(t *testing.T) {
	row := carryAutomationSurfaceRow("1011", "track:1010", "3")
	loopMap := map[string]any{
		"schema_version":      freeStateReasoningLoopSchema,
		"loop_id":             "loop-carry",
		"conversation_id":     "conversation-carry",
		"status":              "observing",
		"original_intent":     "balance the vocal",
		"automation_surfaces": []any{row},
	}
	typed, ok := freeStateLoopFromAny(loopMap)
	if !ok {
		t.Fatal("setup: the loop map must deserialize into the typed loop")
	}
	if len(typed.AutomationSurfaces) != 1 {
		t.Fatalf("round-trip dropped the automation_surfaces rows: %d", len(typed.AutomationSurfaces))
	}
	if got := firstStringFromMap(typed.AutomationSurfaces[0], "plugin_id"); got != "1011" {
		t.Fatalf("row identity lost in the typed round-trip: plugin_id=%q", got)
	}
	if got := firstStringFromMap(typed.AutomationSurfaces[0], "project_revision"); got != "3" {
		t.Fatalf("row revision lost in the typed round-trip: project_revision=%q", got)
	}
	// freeStateLoopMap writes the key back for the next run-boundary rebuild.
	back := freeStateLoopMap(typed)
	rows := freeStateMapRows(back["automation_surfaces"])
	if len(rows) != 1 {
		t.Fatalf("freeStateLoopMap lost the automation_surfaces key: %v", back["automation_surfaces"])
	}
	if firstStringFromMap(rows[0], "plugin_id") != "1011" {
		t.Fatalf("freeStateLoopMap row identity mismatch: %+v", rows[0])
	}
	// cloneFreeStateLoop deep-copies: mutating the clone must not touch the original.
	clone := cloneFreeStateLoop(typed)
	clone.AutomationSurfaces[0]["plugin_id"] = "mutated"
	if firstStringFromMap(typed.AutomationSurfaces[0], "plugin_id") != "1011" {
		t.Fatal("cloneFreeStateLoop shares the automation_surfaces backing array")
	}
}

// 面一 ②: the merge keeps the agentloop writer's contract — same
// track_id+plugin_id refreshes in place, strict-forward revision expiry drops
// stale rows, an absent current revision expires nothing, and the result is
// FIFO-bounded to the ledger limit.
func TestMergeFreeStateAutomationSurfaceRowsExpiryAndLimit(t *testing.T) {
	base := []map[string]any{
		carryAutomationSurfaceRow("1011", "track:1010", "3"),
		carryAutomationSurfaceRow("1012", "track:1020", "8"),
	}
	overlay := []map[string]any{
		carryAutomationSurfaceRow("1011", "track:1010", "9"), // same fingerprint: refresh
		carryAutomationSurfaceRow("1013", "track:1030", "8"), // new fingerprint: append
	}
	// Identity merge (no current revision: expiry off).
	merged := mergeFreeStateAutomationSurfaceRows(base, overlay, "")
	if len(merged) != 3 {
		t.Fatalf("merge produced %d rows, want 3 (refresh in place + append): %+v", len(merged), merged)
	}
	if got := firstStringFromMap(merged[0], "project_revision"); got != "9" {
		t.Fatalf("the overlay row must refresh the same track+plugin row in place: revision=%q", got)
	}
	if got := firstStringFromMap(merged[1], "plugin_id"); got != "1012" {
		t.Fatalf("base rows must keep their slots: plugin_id=%q", got)
	}
	if got := firstStringFromMap(merged[2], "plugin_id"); got != "1013" {
		t.Fatalf("the new fingerprint row must append: plugin_id=%q", got)
	}

	// Strict-forward expiry: revision 9 drops every strictly-older row
	// (refreshed rev-9 row survives, rev-3/rev-8 rows go).
	expired := mergeFreeStateAutomationSurfaceRows(base, overlay, "9")
	if len(expired) != 1 || firstStringFromMap(expired[0], "plugin_id") != "1011" {
		t.Fatalf("revision 9 must keep only the refreshed rev-9 row: %+v", expired)
	}
	kept := mergeFreeStateAutomationSurfaceRows([]map[string]any{
		carryAutomationSurfaceRow("1011", "track:1010", "9"),   // equal revision: kept
		carryAutomationSurfaceRow("1012", "track:1020", ""),    // no revision: kept
		carryAutomationSurfaceRow("1013", "track:1030", "abc"), // unorderable: kept
	}, nil, "9")
	if len(kept) != 3 {
		t.Fatalf("equal/absent/unorderable revisions must be kept fail-closed: %+v", kept)
	}
	noExpiry := mergeFreeStateAutomationSurfaceRows(base, nil, "")
	if len(noExpiry) != 2 {
		t.Fatalf("an absent current revision must expire nothing: %+v", noExpiry)
	}

	// FIFO bound: ten distinct rows collapse to the last eight.
	flood := make([]map[string]any, 0, 10)
	for i := 0; i < 10; i++ {
		flood = append(flood, carryAutomationSurfaceRow("p", "track:"+string(rune('a'+i)), "9"))
	}
	bounded := mergeFreeStateAutomationSurfaceRows(nil, flood, "9")
	if len(bounded) != freeStateAutomationSurfaceLedgerLimit {
		t.Fatalf("the merge must bound rows to %d: %d", freeStateAutomationSurfaceLedgerLimit, len(bounded))
	}
	if got := firstStringFromMap(bounded[0], "track_id"); got != "track:c" {
		t.Fatalf("the bound must keep the newest rows (FIFO head drop): first track=%q", got)
	}
}

// 面三 T1a: the bare-continue swallow carries the pending work's actual face
// (decision + proposal outline + latest observation) in its WorkflowData.
func TestContinueSwallowCarriesFreeStateSummary(t *testing.T) {
	s := runFencedServer()
	now := time.Now().UTC()
	storeRunFencePark(s, runFencePendingPark("cont-carry-summary", runFenceCurrentRun, now), nil)
	s.mu.Lock()
	s.freeStateLoops[runFenceConversation] = freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-carry", ConversationID: runFenceConversation,
		GoalID: runFenceGoal, RunID: runFenceCurrentRun, Status: "awaiting_experiment",
		OriginalIntent: "balance the vocal", ActiveIntent: "balance the vocal",
		MaxCycles: freeStateDefaultMaxCycles, ContinuationBudget: 6, ContinuationUsed: 2,
		LatestDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status:        agentloop.FreeStateCapabilityBlocked,
			Summary:       "无法继续：缺少该插件参数的权威写前标识与时间跨度。",
			ImprovementProposal: &agentprotocol.ImprovementProposal{
				SchemaVersion: agentprotocol.ImprovementProposalSchema,
				ActionDomain:  "track_gain", ActionKind: "static_gain_write",
				ImprovementIntent: "bump the vocal by 1 dB", Confidence: 0.7,
				Target: map[string]any{"track_id": "1010"}, EvidenceRefs: []string{"obs-1"},
			},
		},
		LatestObservation: &agentloop.RecentObservation{Tool: "plugin.get_parameters", Status: "ok"},
		CreatedAt:         now, UpdatedAt: now,
	}
	s.mu.Unlock()

	resp := runFenceNudge(t, s)
	if resp.StopReason != "automatic_continuation_already_scheduled" {
		t.Fatalf("setup: the nudge must hit the automatic-queue swallow: stop=%q", resp.StopReason)
	}
	summary, _ := resp.WorkflowData["free_state_summary"].(map[string]any)
	if len(summary) == 0 {
		t.Fatal("the swallow reply must carry free_state_summary in its WorkflowData")
	}
	if got, _ := summary["decision_status"].(string); got != agentloop.FreeStateCapabilityBlocked {
		t.Fatalf("summary decision_status = %q, want capability_blocked", got)
	}
	proposal, _ := summary["proposal"].(map[string]any)
	if got, _ := proposal["action_domain"].(string); got != "track_gain" {
		t.Fatalf("summary proposal outline lost the action_domain: %+v", proposal)
	}
	observation, _ := summary["latest_observation"].(map[string]any)
	if got, _ := observation["tool"].(string); got != "plugin.get_parameters" {
		t.Fatalf("summary latest_observation lost the tool face: %+v", observation)
	}
	if got, _ := summary["continuation_used"].(int); got != 2 {
		t.Fatalf("summary lost the continuation counter: %+v", summary["continuation_used"])
	}
}

// 面三 T1b: a scheduler-claimed terminal slice delivers the terminal summary
// to the original conversation through the existing scheduler_chain event,
// and the same terminal shape on the HTTP path delivers nothing (the caller
// renders its own response).
func carryTerminalServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := New(nil, nil, nil)
	conversationID := "conversation-carry-terminal"
	goalID := "goal-carry-terminal"
	now := time.Now().UTC()
	s.harness.EnsureGoal(goalID, "run-carry-terminal", "balance the vocal")
	s.mu.Lock()
	s.conversationGoals[conversationID] = goalID
	s.freeStateLoops[conversationID] = freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-carry-terminal", ConversationID: conversationID,
		GoalID: goalID, RunID: "run-carry-terminal", Status: agentloop.FreeStateCapabilityBlocked,
		OriginalIntent: "balance the vocal", ActiveIntent: "balance the vocal",
		MaxCycles: freeStateDefaultMaxCycles,
		LatestDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status:        agentloop.FreeStateCapabilityBlocked,
			Summary:       "缺少该插件参数的权威写前标识与时间跨度，已保留全部观察证据。",
		},
		CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Unlock()
	return s, conversationID
}

func carryTerminalResult(goalID string) agentloop.Result {
	return agentloop.Result{
		GoalID: goalID, RunID: "run-carry-terminal", TaskID: "task-carry-terminal",
		Status: agentruntime.StatusCompleted, StopReason: "capability_blocked_settled",
	}
}

func carryTerminalChainEvents(t *testing.T, s *Server, conversationID string) []AgentEvent {
	t.Helper()
	// agentEventsSince takes s.mu itself — no outer lock here.
	events, _ := s.agentEventsSince(conversationID, 0, 100)
	out := []AgentEvent{}
	for _, event := range events {
		if event.Type == "turn.completed" || event.Type == "turn.failed" {
			out = append(out, event)
		}
	}
	return out
}

func TestSchedulerTerminalSliceDeliversTerminalSummary(t *testing.T) {
	s, conversationID := carryTerminalServer(t)
	goalID := "goal-carry-terminal"
	// The scheduler's claim: the terminal slice runs under a Running durable record.
	now := time.Now().UTC()
	s.mu.Lock()
	s.durableContinuations["cont-carry-terminal"] = DurableContinuation{
		ContinuationID: "cont-carry-terminal", TaskID: "task-carry-terminal", GoalID: goalID,
		ConversationID: conversationID, RunID: "run-carry-terminal",
		OriginalIntent: "balance the vocal",
		Status:         ContinuationRunning, CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Unlock()

	if err := s.recordGoalResult(conversationID, carryTerminalResult(goalID)); err != nil {
		t.Fatal(err)
	}
	events := carryTerminalChainEvents(t, s, conversationID)
	if len(events) != 1 {
		t.Fatalf("the scheduler-driven terminal slice must deliver exactly one terminal event: %d", len(events))
	}
	event := events[0]
	if event.GoalID != goalID {
		t.Fatalf("terminal event identity mismatch: goal=%q", event.GoalID)
	}
	if !contextBool(event.Payload, "scheduler_chain") {
		t.Fatalf("the terminal delivery must ride the scheduler_chain marker: %+v", event.Payload)
	}
	if !strings.Contains(event.Body, "能力边界") {
		t.Fatalf("terminal event body must name the settlement in plain language: body=%q", event.Body)
	}
}

func TestHTTPTerminalSliceDeliversNoTerminalSummary(t *testing.T) {
	s, conversationID := carryTerminalServer(t)
	goalID := "goal-carry-terminal"
	// No Running durable record: this terminal result came from an HTTP turn,
	// whose caller renders the response itself.
	if err := s.recordGoalResult(conversationID, carryTerminalResult(goalID)); err != nil {
		t.Fatal(err)
	}
	if events := carryTerminalChainEvents(t, s, conversationID); len(events) != 0 {
		t.Fatalf("the HTTP terminal path must not emit a duplicate terminal event: %+v", events)
	}
}
