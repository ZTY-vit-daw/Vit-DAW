package pullharness

// budget_test.go — 观察预算语义（卡面验收标准 1：budget 超限→终态分类
// budget_exhausted；AGENTS §8 分记义务的常量互异锁定）。

import (
	"strings"
	"testing"
)

func TestObservationBudgetCycleExhausted(t *testing.T) {
	budget := ObservationBudget{MaxCycles: 3}
	cases := []struct {
		cycles    int
		exhausted bool
	}{
		{0, false},
		{2, false},
		{3, true},
		{4, true},
	}
	for _, tc := range cases {
		if got := budget.CycleExhausted(tc.cycles); got != tc.exhausted {
			t.Fatalf("CycleExhausted(%d) with cap 3 = %v, want %v", tc.cycles, got, tc.exhausted)
		}
	}
	// 上限 <=0 = 不设限（checkTurnBudget 既有约定）。
	uncapped := ObservationBudget{}
	if uncapped.CycleExhausted(10000) {
		t.Fatal("MaxCycles=0 must be uncapped")
	}
}

func TestObservationBudgetProbeExhausted(t *testing.T) {
	budget := ObservationBudget{MaxProbeCost: 10}
	if budget.ProbeExhausted(9.99) {
		t.Fatal("spent under line must not be exhausted")
	}
	if !budget.ProbeExhausted(10) {
		t.Fatal("spent at line must be exhausted")
	}
	if !budget.ProbeExhausted(12.5) {
		t.Fatal("spent over line must be exhausted")
	}
	uncapped := ObservationBudget{}
	if uncapped.ProbeExhausted(1e9) {
		t.Fatal("MaxProbeCost=0 must be uncapped")
	}
}

// AGENTS §8：止损终态是独立失败分类——与 no_candidate_found /
// capability_blocked / context_overflow 互异，不得混记。
func TestBudgetExhaustedIsDistinctFailureClass(t *testing.T) {
	classes := []string{
		OutcomeBudgetExhausted,
		OutcomeNoCandidateFound,
		OutcomeCapabilityBlocked,
		OutcomeContextOverflow,
		OutcomeJudgmentOK,
		OutcomeFastPath,
		OutcomeLLMError,
		OutcomeInterrupted,
	}
	seen := map[string]bool{}
	for _, class := range classes {
		if seen[class] {
			t.Fatalf("outcome class %q duplicated — failure classes must stay distinct", class)
		}
		seen[class] = true
	}
	if OutcomeBudgetExhausted != "budget_exhausted" {
		t.Fatalf("budget_exhausted literal drifted: %q", OutcomeBudgetExhausted)
	}
}

func TestObservationBudgetDisclosure(t *testing.T) {
	budget := ObservationBudget{MaxCycles: 3, MaxProbeCost: 10}

	near := budget.Disclosure(2, 5)
	if !strings.Contains(near, "cycles=2/3") || !strings.Contains(near, "probe_spent=5/10") {
		t.Fatalf("disclosure missing state numbers: %q", near)
	}
	if !strings.Contains(near, "budget_warning") {
		t.Fatalf("remaining<=1 cycle must warn: %q", near)
	}

	far := budget.Disclosure(1, 5)
	if strings.Contains(far, "budget_warning") {
		t.Fatalf("no warning expected with 2 cycles remaining: %q", far)
	}

	over := budget.Disclosure(1, 10)
	if !strings.Contains(over, "probe account over line") {
		t.Fatalf("probe over line must warn: %q", over)
	}

	uncapped := ObservationBudget{}.Disclosure(7, 1.5)
	if !strings.Contains(uncapped, "cycles=7/uncapped") || !strings.Contains(uncapped, "probe_spent=1.5/uncapped") {
		t.Fatalf("uncapped disclosure mismatch: %q", uncapped)
	}
	if strings.Contains(uncapped, "budget_warning") {
		t.Fatalf("uncapped budget must never warn: %q", uncapped)
	}
}
