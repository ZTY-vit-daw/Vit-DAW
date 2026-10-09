package pullharness

// session_test.go — Session 协议类型验收（L1-5-IMPL-D 腿1，§11.1 冻结
// 签名的机械面）：Disposition/ReturnKind 封闭枚举（未知拒绝）、步协议
// 有效性（Suspend/Terminal 必带合法 Kind；Continue 零值合法）、账本披露
// 的 unknown 成本语义（不填 0 冒充免费）。

import (
	"strings"
	"testing"
)

func TestDispositionClosedEnum(t *testing.T) {
	valid := map[Disposition]string{
		DispositionContinue: "continue",
		DispositionSuspend:  "suspend",
		DispositionTerminal: "terminal",
	}
	for value, name := range valid {
		if !value.Valid() {
			t.Errorf("disposition %d must be valid", value)
		}
		if value.String() != name {
			t.Errorf("disposition %d String()=%q, want %q", value, value.String(), name)
		}
	}
	for _, invalid := range []Disposition{0, 4, 255} {
		if invalid.Valid() {
			t.Errorf("disposition %d must be rejected", invalid)
		}
		if invalid.String() != "unknown" {
			t.Errorf("disposition %d String()=%q, want unknown", invalid, invalid.String())
		}
	}
}

func TestReturnKindClosedEnum(t *testing.T) {
	names := map[ReturnKind]string{
		ReturnDone:                       "done",
		ReturnFailed:                     "failed",
		ReturnConfirmation:               "confirmation",
		ReturnClarification:              "clarification",
		ReturnSliceLimit:                 "slice_limit",
		ReturnInterjection:               "interjection",
		ReturnTransient:                  "transient",
		ReturnInterrupted:                "interrupted",
		ReturnUserCancelled:              "user_cancelled",
		ReturnObservationBudgetExhausted: "observation_budget_exhausted",
	}
	if len(names) != 10 {
		t.Fatalf("ReturnKind enum count=%d, want 10 (§11.1 frozen set)", len(names))
	}
	for value, name := range names {
		if !value.Valid() {
			t.Errorf("kind %d must be valid", value)
		}
		if value.String() != name {
			t.Errorf("kind %d String()=%q, want %q", value, value.String(), name)
		}
	}
	for _, invalid := range []ReturnKind{0, 11, 255} {
		if invalid.Valid() {
			t.Errorf("kind %d must be rejected", invalid)
		}
	}
}

func TestValidateStepProtocol(t *testing.T) {
	cases := []struct {
		name  string
		step  Step
		valid bool
	}{
		{"continue_zero_kind", Step{Disposition: DispositionContinue}, true},
		{"terminal_with_kind", Step{Disposition: DispositionTerminal, Kind: ReturnDone}, true},
		{"suspend_with_kind", Step{Disposition: DispositionSuspend, Kind: ReturnInterrupted}, true},
		{"terminal_without_kind", Step{Disposition: DispositionTerminal}, false},
		{"suspend_without_kind", Step{Disposition: DispositionSuspend}, false},
		{"unknown_disposition", Step{Disposition: Disposition(9), Kind: ReturnDone}, false},
		{"unknown_kind", Step{Disposition: DispositionTerminal, Kind: ReturnKind(42)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateStep(tc.step, "test")
			if tc.valid && got != "" {
				t.Errorf("step rejected: %s", got)
			}
			if !tc.valid && got == "" {
				t.Error("invalid step accepted")
			}
		})
	}
}

// 账本披露：known 成本数值披露；unknown 成本显式标注（不填 0 冒充免费）。
func TestLedgerDisclosureRows(t *testing.T) {
	known := ledgerDisclosureRows(LedgerView{CompletedCycles: 2, ProbeSpent: 1.5, ProbeCostKnown: true},
		ObservationBudget{MaxCycles: 5, MaxProbeCost: 3})
	joined := ""
	for _, row := range known {
		joined += row + "\n"
	}
	for _, want := range []string{"cycles=2/5", "probe_spent=1.5/3"} {
		if !strings.Contains(joined, want) {
			t.Errorf("known-cost disclosure missing %q in:\n%s", want, joined)
		}
	}

	unknown := ledgerDisclosureRows(LedgerView{CompletedCycles: 1, ProbeCostKnown: false},
		ObservationBudget{MaxCycles: 5})
	joined = ""
	for _, row := range unknown {
		joined += row + "\n"
	}
	if !strings.Contains(joined, "probe_spent=unknown") {
		t.Errorf("unknown-cost must not render as 0:\n%s", joined)
	}
	if !strings.Contains(joined, "probe cost accounting unavailable") {
		t.Errorf("unknown-cost note missing:\n%s", joined)
	}
}
