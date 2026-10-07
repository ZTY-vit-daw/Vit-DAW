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
	joined := ""
	for _, message := range msgs {
		joined += message.Content + "\n"
	}
	if !strings.Contains(joined, "CONTINUATION BUDGET CRITICAL") {
		t.Fatalf("critical directive missing from the assembled request")
	}
	sys := ""
	if len(msgs) > 0 {
		sys = msgs[0].Content
	}
	if strings.Contains(sys, "CONTINUATION BUDGET CRITICAL") {
		t.Fatalf("per-turn budget directive must live in the user turn, not the stable system message (L1-4-IMPL-A §3.3)")
	}
}
