package chat

import "testing"

func TestAgentLoopInteractionRequestsPromotesWorkflowCard(t *testing.T) {
	requests := agentLoopInteractionRequests([]map[string]any{
		{
			"result": map[string]any{
				"interaction_requests": []AgentInteractionRequest{{
					ID:     "interaction_observation_scope",
					Kind:   "form",
					Type:   "observation_scope_request",
					Status: "waiting_for_user",
				}},
			},
		},
	})
	if len(requests) != 1 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].ID != "interaction_observation_scope" || requests[0].Type != "observation_scope_request" {
		t.Fatalf("request = %#v", requests[0])
	}
}
