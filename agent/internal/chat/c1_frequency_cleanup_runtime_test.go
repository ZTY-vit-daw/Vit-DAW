package chat

import (
	"testing"
)

func TestC1MutationRequestedHonorsExplicitExecutionContext(t *testing.T) {
	if !c1MutationRequested("C1", map[string]any{"c1_execute": true}) {
		t.Fatal("explicit C1 execution context was ignored")
	}
	if c1MutationRequested("C1", map[string]any{"c1_execute": false}) {
		t.Fatal("plain C1 label should not imply mutation")
	}
}
