package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

func TestMessageLoopAudioClosureEmptyOutputBecomesTypedProtocolFailure(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		"",
		`{"final":false,"failure_reason":"model_protocol_failure","reply":"","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "make the vocal less harsh",
		Context: map[string]any{"minimal_audio_closure": map[string]any{
			"schema_version": "minimal_audio_closure.v1", "phase": "reasoning", "original_intent": "make the vocal less harsh",
		}},
	})
	if res.StopReason != StopReasonModelProtocolFailure || !res.ModelProtocolFailure || res.ModelProtocolRepairs != 1 || res.NeedsClarification {
		t.Fatalf("empty output was not typed as protocol failure: %+v", res)
	}
	if len(client.calls) != 2 {
		t.Fatalf("model calls=%d want=2", len(client.calls))
	}
	repairPrompt := client.calls[1][0].Content
	if strings.Contains(repairPrompt, `"needs_clarification":true`) || !strings.Contains(repairPrompt, "never ask the user to restate") {
		t.Fatalf("closure repair prompt permits clarification: %s", repairPrompt)
	}
}

func TestMessageLoopAudioClosureDistinguishesSuccessfulRepair(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		"not json",
		`{"final":true,"reply":"bounded conclusion","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{}, Budget: Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText: "diagnose the issue",
		Context: map[string]any{"minimal_audio_closure": map[string]any{
			"schema_version": "minimal_audio_closure.v1", "phase": "reasoning",
		}},
	})
	if res.StopReason != StopReasonDone || res.ModelProtocolFailure || res.ModelProtocolRepairs != 1 || res.Reply != "bounded conclusion" {
		t.Fatalf("successful repair was misclassified: %+v", res)
	}
}
