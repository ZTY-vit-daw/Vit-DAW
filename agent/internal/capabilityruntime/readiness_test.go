package capabilityruntime

import "testing"

func TestEvaluateBlocksOnlyRequiredMissingOrBlockedConditions(t *testing.T) {
	got := Evaluate("cap", []Condition{
		{ID: "ready", Required: true, Status: ConditionReady},
		{ID: "optional", Required: false, Status: ConditionMissing},
		{ID: "warning", Required: true, Status: ConditionWarning},
	})
	if !got.CanProceed || got.Status != ConditionWarning {
		t.Fatalf("readiness = %+v", got)
	}
	got = Evaluate("cap", append(got.Conditions, Condition{ID: "required", Required: true, Status: ConditionBlocked}))
	if got.CanProceed || got.Status != ConditionBlocked || len(got.BlockedBy) != 1 {
		t.Fatalf("blocked readiness = %+v", got)
	}
}
