package chat

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

// B3-ABLATION-1: the test-only ablation hook pair (264 B4 hook-family
// pattern). The fixture phrase is the same B3 pan-layout wording in both
// legs — only the env differs — so the pair pins the exact contract: default
// unset keeps the deterministic lexicon takeover and canary dispatch
// byte-for-byte; VIT_PAN_LAYOUT_ABLATION=1 bypasses the B3 family lexicon
// (every pan-layout phrase falls through to the LLM turn) and disables the
// canary dispatch branch even for an explicit B3 capability context (the
// governance chain — pending proposal confirmation — is not bypassed).
func TestPanLayoutAblationHookPair(t *testing.T) {
	const phrase = "分析一下当前工程的声像布局"

	t.Run("default unset keeps lexicon arming and canary dispatch", func(t *testing.T) {
		if panLayoutAblationBypassed() {
			t.Fatalf("ablation hook must be off by default")
		}
		if got := inferProjectAwareCapability(phrase); got != panLayoutCapabilityID {
			t.Fatalf("default state must keep the B3 lexicon arming for the pan-layout phrase, got %q", got)
		}
		server := &Server{orchestrationRuntime: orchestrationruntime.New()}
		resp, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conv-b3-ablation-pair", ChatRequest{
			Message: phrase,
			Context: map[string]any{"agent_mode": agentModeDefault},
		}, agentruntime.Goal{})
		if !handled {
			t.Fatalf("default state must keep the deterministic canary dispatch for the B3 phrase")
		}
		if resp.Workflow != "capability_runtime_v1" {
			t.Fatalf("default workflow = %q, want capability_runtime_v1", resp.Workflow)
		}
	})

	t.Run("env=1 disarms the lexicon and the canary dispatch", func(t *testing.T) {
		t.Setenv(panLayoutAblationEnv, "1")
		if !panLayoutAblationBypassed() {
			t.Fatalf("hook env must register as bypassed")
		}
		// Lexicon face: the B3 phrase no longer infers the capability.
		if got := inferProjectAwareCapability(phrase); got != "" {
			t.Fatalf("ablation hook must let the pan-layout phrase fall through inference, got %q", got)
		}
		server := &Server{orchestrationRuntime: orchestrationruntime.New()}
		// Natural-phrase face: inference disabled -> nothing claims the turn.
		resp, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conv-b3-ablation-pair", ChatRequest{
			Message: phrase,
			Context: map[string]any{"agent_mode": agentModeDefault},
		}, agentruntime.Goal{})
		if handled {
			t.Fatalf("ablation hook must let the B3 phrase fall through to the LLM turn, got takeover: %+v", resp)
		}
		// Dispatch face: even an explicit B3 capability context must not reach
		// the pan-layout canary runtime (the dispatch branch itself is
		// disabled).
		resp, handled = server.handleCapabilityRuntimeCanary(context.Background(), "conv-b3-ablation-pair", ChatRequest{
			Message: phrase,
			Context: map[string]any{"agent_mode": agentModeDefault, "capability_id": panLayoutCapabilityID},
		}, agentruntime.Goal{})
		if handled {
			t.Fatalf("ablation hook must disable the B3 canary dispatch for explicit capability context, got takeover: %+v", resp)
		}
	})

	t.Run("only exact 1 arms the hook", func(t *testing.T) {
		for _, value := range []string{"", "0", "true", "off"} {
			t.Setenv(panLayoutAblationEnv, value)
			if panLayoutAblationBypassed() {
				t.Fatalf("env value %q must not bypass the gate", value)
			}
			if got := inferProjectAwareCapability(phrase); got != panLayoutCapabilityID {
				t.Fatalf("env value %q must keep the B3 lexicon armed, got %q", value, got)
			}
		}
	})
}
