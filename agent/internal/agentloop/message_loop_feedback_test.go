package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestMessageLoopNeutralFamilyFeedbackHistoryKeepsLatestGateOnly(t *testing.T) {
	conversation := []llm.Message{
		{Role: "user", Content: "original acoustic goal"},
		{Role: "assistant", Content: `{"selected_plugin":"must-not-leak"}`},
		{Role: "user", Content: "<final_gate>older gate</final_gate>"},
		{Role: "assistant", Content: `{"materialization_identity":"must-not-leak-either"}`},
		{Role: "user", Content: "<final_gate>latest gate feedback</final_gate>"},
	}

	history := messageLoopNeutralFamilyFeedbackHistory(conversation)
	if len(history) != 1 {
		t.Fatalf("feedback history length = %d, want 1: %#v", len(history), history)
	}
	if history[0].Role != "user" || history[0].Content != "<final_gate>latest gate feedback</final_gate>" {
		t.Fatalf("feedback history = %#v", history)
	}
	joined := history[0].Content
	if strings.Contains(joined, "must-not-leak") || strings.Contains(joined, "older gate") {
		t.Fatalf("feedback history retained unrelated or stale content: %q", joined)
	}
}

func TestMessageLoopNeutralFamilyAssemblyExposesFinalGateFeedback(t *testing.T) {
	state := &runState{
		input: Input{
			UserText: "find one bounded improvement",
			Conversation: []llm.Message{
				{Role: "assistant", Content: `{"selected_plugin":"must-not-leak"}`},
				{Role: "user", Content: "<final_gate>return needs_experiment with one proposal</final_gate>"},
			},
		},
		goal:   agentruntime.Goal{GoalID: "goal-feedback", RunID: "run-feedback"},
		budget: Budget{MaxToolCalls: 1},
	}
	assembly := (&MessageLoop{}).assemblyNeutralFamilySelection(state, `{"free_state_reasoning_loop":{"status":"active"}}`)
	if len(assembly.Messages) < 3 {
		t.Fatalf("neutral assembly messages = %#v, want system, feedback, runtime", assembly.Messages)
	}
	if !strings.Contains(assembly.Messages[1].Content, "<final_gate>return needs_experiment with one proposal</final_gate>") {
		t.Fatalf("final-gate feedback is not visible in neutral assembly: %#v", assembly.Messages)
	}
	for _, message := range assembly.Messages {
		if strings.Contains(message.Content, "must-not-leak") {
			t.Fatalf("neutral assembly leaked prior materialization identity: %#v", assembly.Messages)
		}
	}
}
