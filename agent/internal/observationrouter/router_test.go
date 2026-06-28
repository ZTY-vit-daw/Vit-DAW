package observationrouter

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

func TestObservationRouterWrapsMixObserve(t *testing.T) {
	source := agentprotocol.Source{ConversationID: "chat_1", GoalID: "goal_1", RunID: "run_1"}
	args := map[string]any{"track_id": "1007", "goal_text": "reduce mud"}
	req, ok := RequestFromTool("mix.observe", args, source)
	if !ok || req.Kind != agentprotocol.KindObservationRequest || req.TargetRef != "track:1007" {
		t.Fatalf("request = %+v ok=%v", req, ok)
	}
	result, ok := ResultFromToolResult("mix.observe", args, map[string]any{
		"observation_id": "obs_1",
		"summary":        "low-mid buildup",
	}, source)
	if !ok || result.Kind != agentprotocol.KindObservationResult || result.ContextPackID != "obs_1" {
		t.Fatalf("result = %+v ok=%v", result, ok)
	}
}
