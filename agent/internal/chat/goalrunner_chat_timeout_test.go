package chat

import (
	"testing"
	"time"
)

func TestAgentLoopBudgetLeavesHeadroomInsideFrozenModelTurnTimeout(t *testing.T) {
	const frozenModelTurnTimeout = 420 * time.Second
	for _, mode := range []string{agentModeGoal, agentModePlan, ""} {
		budget := agentLoopBudgetForMode(mode)
		if budget.Timeout != 3*time.Minute {
			t.Fatalf("mode %q timeout = %s, want 3m", mode, budget.Timeout)
		}
		if budget.Timeout >= frozenModelTurnTimeout {
			t.Fatalf("mode %q timeout = %s, must leave headroom below %s", mode, budget.Timeout, frozenModelTurnTimeout)
		}
		if headroom := frozenModelTurnTimeout - budget.Timeout; headroom < 3*time.Minute {
			t.Fatalf("mode %q headroom = %s, must cover three 60-second LLM attempts", mode, headroom)
		}
	}
}

func TestAgentLoopBudgetAccountsForSemanticEntryElapsedTime(t *testing.T) {
	budget := agentLoopBudgetForModeAfter(agentModeGoal, 47*time.Second)
	if budget.Timeout != 133*time.Second {
		t.Fatalf("remaining timeout = %s, want 133s", budget.Timeout)
	}
	exhausted := agentLoopBudgetForModeAfter(agentModeGoal, 3*time.Minute)
	if exhausted.Timeout != time.Nanosecond {
		t.Fatalf("exhausted timeout = %s, want immediate checkpoint", exhausted.Timeout)
	}
}

func TestDiagnosticOnlyBudgetIsBoundedAndDoesNotChangeOrdinaryBudget(t *testing.T) {
	diagnostic := agentLoopBudgetForContext(agentModeDefault, map[string]any{"free_state_diagnostic_only": true})
	if diagnostic.MaxTurns != 5 || diagnostic.MaxToolCalls != 6 || diagnostic.Timeout != 210*time.Second {
		t.Fatalf("diagnostic budget = %+v, want 5 turns, 6 tools, 210s", diagnostic)
	}
	ordinary := agentLoopBudgetForContext(agentModeDefault, map[string]any{})
	if ordinary != agentLoopBudgetForMode(agentModeDefault) {
		t.Fatalf("ordinary budget changed unexpectedly: %+v", ordinary)
	}
}
