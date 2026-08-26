package agentloop

import (
	"strings"
	"testing"
)

func TestSystemPromptWiresBudgetDirectiveFromContext(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": map[string]any{
			"status": "observing", "continuation_budget": 6, "continuation_used": 5,
		},
	}}}
	l := &MessageLoop{}
	msgs := l.assembly(state, "").Messages
	sys := ""
	if len(msgs) > 0 {
		sys = msgs[0].Content
	}
	if !strings.Contains(sys, "CONTINUATION BUDGET CRITICAL") {
		t.Fatalf("critical directive missing from assembled system prompt")
	}
}
