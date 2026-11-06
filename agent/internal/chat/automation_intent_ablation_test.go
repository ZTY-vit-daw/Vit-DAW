package chat

import (
	"context"
	"testing"
)

// C4-ABLATION-1: the test-only ablation hook pair. The fixture phrase is the
// same armed time-window wording in both legs — only the env differs — so the
// pair pins the exact contract: default unset keeps the deterministic
// pre-model takeover byte-for-byte; VIT_AUTOMATION_INTENT_ABLATION=1 disarms
// the vocabulary gate so the phrase falls through to the LLM turn with no
// choreography state stored (the pending/governance chain stays untouched —
// nothing armed means nothing pending).

func TestAutomationIntentAblationHookPair(t *testing.T) {
	const phrase = "30 秒之后动起来"
	request := func() ChatRequest {
		return ChatRequest{
			ConversationID: "conv-ablation-pair",
			Message:        phrase,
			Context:        map[string]any{"agent_mode": agentModeDefault, "selected_track_id": "1007"},
		}
	}

	t.Run("default unset keeps pre-model arming", func(t *testing.T) {
		model := clarifyHandoffTestLLM(t, "要动哪一类参数？电平可以直接做；空间或音色需要轨上挂插件。", nil, nil)
		server := clarifyHandoffTestServer()
		clarifyHandoffPointServer(t, server, model)
		if automationIntentAblationBypassed() {
			t.Fatalf("ablation hook must be off by default")
		}
		resp, handled := server.handleAutomationChoreoChat(context.Background(), "conv-ablation-pair", request(), agentModeDefault)
		if !handled {
			t.Fatalf("default state must keep the deterministic takeover for the armed phrase")
		}
		if resp.Workflow != automationChoreoWorkflow {
			t.Fatalf("default workflow = %q, want %q", resp.Workflow, automationChoreoWorkflow)
		}
		if _, ok := server.automationChoreoForConversation("conv-ablation-pair"); !ok {
			t.Fatalf("default state must store the pending choreography state")
		}
	})

	t.Run("env=1 disarms the vocabulary gate", func(t *testing.T) {
		t.Setenv(automationIntentAblationEnv, "1")
		if !automationIntentAblationBypassed() {
			t.Fatalf("hook env must register as bypassed")
		}
		// No engine double is needed: the armed leg below must return before
		// any LLM participation, so a stray engine call would fail the test
		// by hanging on the unset real engine config instead of passing.
		server := clarifyHandoffTestServer()
		resp, handled := server.handleAutomationChoreoChat(context.Background(), "conv-ablation-pair", request(), agentModeDefault)
		if handled {
			t.Fatalf("ablation hook must let the armed phrase fall through to the LLM turn, got takeover: %+v", resp)
		}
		if _, ok := server.automationChoreoForConversation("conv-ablation-pair"); ok {
			t.Fatalf("ablation hook must not leave choreography state behind")
		}
	})

	t.Run("only exact 1 arms the hook", func(t *testing.T) {
		for _, value := range []string{"", "0", "true", "off"} {
			t.Setenv(automationIntentAblationEnv, value)
			if automationIntentAblationBypassed() {
				t.Fatalf("env value %q must not bypass the gate", value)
			}
		}
	})
}
