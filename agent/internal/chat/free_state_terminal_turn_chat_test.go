package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
)

// BOUNDARY-1 v2 Phase 1, chat side: terminal-turn trigger matrix, atomic lock,
// §11 persistence defaults for the new loop fields, and the reserved slice's
// in-slice retry budget.

func terminalTriggerLoop() freeStateReasoningLoop {
	return freeStateReasoningLoop{
		SchemaVersion:      freeStateReasoningLoopSchema,
		LoopID:             "loop-terminal",
		ConversationID:     "conversation-terminal",
		Status:             "observing",
		OriginalIntent:     "improve the mix",
		ContinuationBudget: 6,
		ContinuationUsed:   1,
	}
}

func terminalClosure(maxRounds, started int) audioclosure.State {
	return audioclosure.State{
		ContractID: "contract-terminal", Policy: audioclosure.Policy{MaxClosureRounds: maxRounds},
		RoundsStarted: started,
	}
}

func TestFreeStateTerminalTurnTriggerMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(loop *freeStateReasoningLoop, closure *audioclosure.State, tracked *bool)
		want   string
	}{
		{name: "healthy", want: ""},
		{name: "closure_rounds_remaining_one", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			c.Policy.MaxClosureRounds, c.RoundsStarted = 6, 5
		}, want: FreeStateTerminalReasonBudgetCritical},
		{name: "closure_rounds_exhausted", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			c.Policy.MaxClosureRounds, c.RoundsStarted = 6, 6
		}, want: FreeStateTerminalReasonBudgetCritical},
		{name: "continuation_remaining_one", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			l.ContinuationBudget, l.ContinuationUsed = 6, 5
		}, want: FreeStateTerminalReasonBudgetCritical},
		{name: "continuation_last_checkpoint", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			l.ContinuationBudget, l.ContinuationUsed = 6, 6
		}, want: FreeStateTerminalReasonLastCheckpoint},
		{name: "already_locked", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			l.TerminalTurnLocked = true
			l.ContinuationBudget, l.ContinuationUsed = 6, 6
		}, want: ""},
		{name: "closure_rounds_win_over_continuation", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			l.ContinuationBudget, l.ContinuationUsed = 6, 6
			c.Policy.MaxClosureRounds, c.RoundsStarted = 6, 5
		}, want: FreeStateTerminalReasonBudgetCritical},
		{name: "untracked_closure_ignored", mutate: func(l *freeStateReasoningLoop, c *audioclosure.State, t *bool) {
			*t = false
			c.Policy.MaxClosureRounds, c.RoundsStarted = 6, 6
		}, want: ""},
	}
	for _, tc := range cases {
		loop := terminalTriggerLoop()
		closure := terminalClosure(0, 0)
		tracked := true
		if tc.mutate != nil {
			tc.mutate(&loop, &closure, &tracked)
		}
		if got := freeStateTerminalTurnTriggerReason(loop, closure, tracked); got != tc.want {
			t.Fatalf("%s: trigger reason = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestFreeStateTerminalTurnTriggerGuardedScopes(t *testing.T) {
	base := terminalTriggerLoop()
	base.ContinuationBudget, base.ContinuationUsed = 6, 6 // last checkpoint
	// A loop with a live experiment runtime has its own protected windows.
	withExperiment := base
	withExperiment.Experiment = &experiment.Turn{
		SchemaVersion: "experiment.turn.v1", ID: "exp-terminal", ConversationID: base.ConversationID,
		Status: experiment.StatusRunning,
	}
	if got := freeStateTerminalTurnTriggerReason(withExperiment, audioclosure.State{}, false); got != "" {
		t.Fatalf("experiment loop must not lock the terminal turn, got %q", got)
	}
	// A loop with a mandatory post-action observation debt is guarded out.
	withDebt := base
	withDebt.RequiresPostActionObservation = true
	if got := freeStateTerminalTurnTriggerReason(withDebt, audioclosure.State{}, false); got != "" {
		t.Fatalf("post-action loop must not lock the terminal turn, got %q", got)
	}
	// An inactive loop never locks.
	inactive := base
	inactive.Status = "blocked"
	if got := freeStateTerminalTurnTriggerReason(inactive, audioclosure.State{}, false); got != "" {
		t.Fatalf("inactive loop must not lock the terminal turn, got %q", got)
	}
}

func TestLockFreeStateTerminalTurnAtomic(t *testing.T) {
	loop := terminalTriggerLoop()
	if !lockFreeStateTerminalTurn(&loop, FreeStateTerminalReasonBudgetCritical) {
		t.Fatal("first lock must succeed")
	}
	if !loop.TerminalTurnLocked || loop.TerminalTurnReason != FreeStateTerminalReasonBudgetCritical {
		t.Fatalf("lock state = %+v", loop)
	}
	// Idempotent: a second lock with another reason never rewrites the first.
	if lockFreeStateTerminalTurn(&loop, FreeStateTerminalReasonLastCheckpoint) {
		t.Fatal("second lock must be a no-op")
	}
	if loop.TerminalTurnReason != FreeStateTerminalReasonBudgetCritical {
		t.Fatalf("lock reason rewritten: %q", loop.TerminalTurnReason)
	}
	// Empty reason never locks.
	fresh := terminalTriggerLoop()
	if lockFreeStateTerminalTurn(&fresh, "  ") {
		t.Fatal("empty reason must not lock")
	}
}

// §11: records persisted before BOUNDARY-1 deserialize with zero-value new
// fields and stay active; the merge keeps the lock sticky and the retry
// counter monotonic across transport overlays.
func TestFreeStateTerminalTurnFieldsBackwardCompatibility(t *testing.T) {
	legacy := `{"schema_version":"free_state_reasoning_loop.v1","loop_id":"loop-old","conversation_id":"c-old","status":"observing","original_intent":"improve the mix","continuation_budget":4,"continuation_used":2}`
	var loop freeStateReasoningLoop
	if err := json.Unmarshal([]byte(legacy), &loop); err != nil {
		t.Fatal(err)
	}
	if loop.TerminalTurnLocked || loop.TerminalTurnReason != "" || loop.TerminalRetryCount != 0 ||
		loop.AdmissionRejectionCount != 0 {
		t.Fatalf("legacy record must default the new fields to zero values: %+v", loop)
	}
	if !freeStateLoopActive(loop) {
		t.Fatal("legacy record must stay active")
	}
	roundTrip, err := json.Marshal(loop)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(roundTrip), "terminal_turn_locked") ||
		strings.Contains(string(roundTrip), "terminal_retry_count") {
		t.Fatalf("zero-value terminal fields must stay omitted: %s", roundTrip)
	}
	// The lock survives an older overlay that predates it; the retry counter
	// only moves forward.
	locked := loop
	locked.TerminalTurnLocked = true
	locked.TerminalTurnReason = FreeStateTerminalReasonLastCheckpoint
	locked.TerminalRetryCount = 1
	olderOverlay := loop // predates the lock
	merged := mergeFreeStateLoops(locked, olderOverlay, true)
	if !merged.TerminalTurnLocked || merged.TerminalTurnReason != FreeStateTerminalReasonLastCheckpoint || merged.TerminalRetryCount != 1 {
		t.Fatalf("overlay erased terminal fields: %+v", merged)
	}
	newerCount := loop
	newerCount.TerminalRetryCount = 2
	merged = mergeFreeStateLoops(locked, newerCount, true)
	if merged.TerminalRetryCount != 2 {
		t.Fatalf("retry counter must move forward: %+v", merged)
	}
}

// The reserved slice carries its strengthened retry inside the same
// checkpoint: a locked loop gets at least two model turns so the retry never
// pauses into a rescheduled continuation (the reservation-eating failure the
// card forbids).
func TestAudioClosureMessageLoopBudgetReservesTerminalRetry(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 1, MaxToolCalls: 4}
	locked := audioClosureMessageLoopBudget(base, map[string]any{
		"free_state_reasoning_loop": map[string]any{"terminal_turn_locked": true},
	})
	if locked.MaxTurns != 2 {
		t.Fatalf("locked slice must reserve the in-slice retry turn, got MaxTurns=%d", locked.MaxTurns)
	}
	unlocked := audioClosureMessageLoopBudget(base, map[string]any{
		"free_state_reasoning_loop": map[string]any{"terminal_turn_locked": false},
	})
	if unlocked.MaxTurns != 1 {
		t.Fatalf("unlocked slice keeps the ordinary budget, got MaxTurns=%d", unlocked.MaxTurns)
	}
}

// TIMING-1: the gate-open signal is retired with the phase admission gate.
// The spine mirror keeps only the observation-organization facts (current
// phase/round/queue); syncing any FS phase arms nothing.
func TestSyncFreeStateSpineArmsNoGateSignalAfterTimingGateRemoval(t *testing.T) {
	s := &Server{freeStateLoops: map[string]freeStateReasoningLoop{}}
	loop := terminalTriggerLoop()
	s.freeStateLoops[loop.ConversationID] = loop
	for _, phase := range []audioclosure.Phase{
		audioclosure.PhaseFS4DiagnosticRound, audioclosure.PhaseFS6TargetConfirmed, audioclosure.PhaseFS7ImprovementProposal,
	} {
		s.syncFreeStateSpine(audioclosure.State{ConversationID: loop.ConversationID, Phase: phase})
		stored, _ := s.freeStateLoop(loop.ConversationID)
		if stored.CurrentPhase != string(phase) {
			t.Fatalf("spine mirror dropped the phase %s: %+v", phase, stored)
		}
	}
	stored, _ := s.freeStateLoop(loop.ConversationID)
	data, _ := json.Marshal(freeStateLoopMap(stored))
	for _, retired := range []string{"needs_experiment_gate_open_pending", "needs_experiment_gate_open_signaled"} {
		if strings.Contains(string(data), retired) {
			t.Fatalf("retired gate-open key %q written back: %s", retired, data)
		}
	}
}

