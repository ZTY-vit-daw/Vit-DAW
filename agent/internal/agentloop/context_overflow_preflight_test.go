package agentloop

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	agentruntime "vit-daw-agent/internal/runtime"
)

// TestMessageLoopContextOverflowFailsClosedBeforeLLM is the short runtime
// preflight for the Graph Context v1 fail-closed boundary: an over-budget
// model context must never reach the LLM. The runner refuses the request and
// returns an explicit context_overflow error instead of silently truncating
// facts.
func TestMessageLoopContextOverflowFailsClosedBeforeLLM(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"unexpected"}`}}
	loop := &MessageLoop{
		Runtime: loopTestRuntime(),
		Client:  client,
		Config:  config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget:  Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	// A blocking hot section (current_selection) over the 10KB hot budget that
	// cannot be degraded must fail closed before any LLM call.
	ranges := make([]map[string]any, 0, 20)
	for i := 0; i < 20; i++ {
		ranges = append(ranges, map[string]any{
			"range_id":      fmt.Sprintf("range-%02d", i),
			"clip_name":     strings.Repeat(fmt.Sprintf("wide clip name %02d ", i), 60),
			"source":        "project_selection",
			"start_seconds": float64(i),
			"end_seconds":   float64(i) + 1,
		})
	}
	result := loop.Start(context.Background(), Input{
		GoalID:   "goal_context_overflow_preflight",
		RunID:    "run_context_overflow_preflight",
		UserText: "run the context overflow preflight",
		Summary:  "run the context overflow preflight",
		Context:  map[string]any{"initialized": true, "selected_clip_ranges": ranges},
	})
	if result.Status != agentruntime.StatusFailed {
		t.Fatalf("overflow result = status=%q stop=%q error=%q", result.Status, result.StopReason, result.Error)
	}
	if !strings.Contains(result.Error, "context_overflow") {
		t.Fatalf("overflow error missing explicit marker: %q", result.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM was called despite context overflow: %d calls", len(client.calls))
	}
	if !strings.Contains(result.Error, "audit_snapshot://") {
		t.Fatalf("overflow error missing cold ref: %q", result.Error)
	}
}
