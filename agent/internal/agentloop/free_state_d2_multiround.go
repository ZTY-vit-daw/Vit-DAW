package agentloop

import (
	"os"
	"strconv"
	"strings"

	"vit-daw-agent/internal/experiment"
)

// FreeStateD2MultiRoundBudgetEnv is the single auditable source of the D2-2
// multi-round admission tier. The chat-side admission gate and this package's
// model prompt read the same value, so a tier is never half-applied: without
// an explicit in-range injection every admission keeps the sealed single-round
// budget of 1 and the model cannot upgrade it.
const FreeStateD2MultiRoundBudgetEnv = "VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET"

// ResolveD2MultiRoundBudget returns the server-injected D2-2 multi-round
// experiment budget: 1 (single round — the default and the fail-closed value)
// or a value within 2..experiment.MaxD2MultiRoundBudget. Malformed or
// out-of-range injections fall back to 1, never to a wider tier.
func ResolveD2MultiRoundBudget() int {
	raw := strings.TrimSpace(os.Getenv(FreeStateD2MultiRoundBudgetEnv))
	if raw == "" {
		return 1
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 2 || value > experiment.MaxD2MultiRoundBudget {
		return 1
	}
	return value
}

// messageLoopFreeStateMultiRoundTier reports whether the model prompt must use
// the D2-2 multi-round wording: an effective budget within
// 2..experiment.MaxD2MultiRoundBudget. Budgets above the sealed bound fall
// back to the single-round wording, mirroring the admission tier's fail-closed
// split.
func messageLoopFreeStateMultiRoundTier(state *runState) bool {
	budget := messageLoopFreeStateEffectiveExperimentBudget(state)
	return budget > 1 && budget <= experiment.MaxD2MultiRoundBudget
}

// messageLoopFreeStateEffectiveExperimentBudget returns the experiment budget
// the prompt should assume: a live experiment's own admission budget is
// authoritative while the loop carries one; before an admission exists, the
// server-injected tier decides what the next admission would receive.
func messageLoopFreeStateEffectiveExperimentBudget(state *runState) int {
	if budget, ok := messageLoopExperimentAdmissionBudget(state); ok {
		return budget
	}
	return ResolveD2MultiRoundBudget()
}

func messageLoopExperimentAdmissionBudget(state *runState) (int, bool) {
	turn := messageLoopMapValue(messageLoopFreeStateContext(state)["experiment"])
	if len(turn) == 0 {
		return 0, false
	}
	admission := messageLoopMapValue(turn["admission"])
	if len(admission) == 0 {
		return 0, false
	}
	switch budget := admission["experiment_budget"].(type) {
	case float64:
		return int(budget), true
	case int:
		return budget, true
	case int64:
		return int(budget), true
	}
	return 0, false
}
