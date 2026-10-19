package agentloop

import (
	"strings"
	"testing"
)

// CAP-BOUNDARY-1: the governed-remediation contract must ride on every
// free-state turn directive surface — the model may only explain or let the
// user choose among the runtime-supplied next_actions; the no-fallback law
// itself stays in force.
func TestCapabilityBoundaryRemediationPromptRuleInTurnDirectives(t *testing.T) {
	rule := capabilityBoundaryRemediationPromptRule()
	for _, needle := range []string{"capability_boundary.v1", "never invent a fallback", "approximate write"} {
		if !strings.Contains(rule, needle) {
			t.Fatalf("remediation rule lost the governed-option contract marker %q: %q", needle, rule)
		}
	}
	directives := messageLoopNeutralFamilyTurnDirectives(&runState{})
	if !strings.Contains(directives, rule) {
		t.Fatalf("turn directives do not carry the capability boundary remediation rule")
	}
	full := messageLoopNeutralFamilySystemPrompt(&runState{})
	if !strings.Contains(full, rule) {
		t.Fatalf("system prompt surface lost the capability boundary remediation rule")
	}
}
