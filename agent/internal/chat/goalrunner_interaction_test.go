package chat

import "testing"

func TestAgentLoopInteractionRequestsPromotesWorkflowCard(t *testing.T) {
	requests := agentLoopInteractionRequests([]map[string]any{
		{
			"result": map[string]any{
				"interaction_requests": []AgentInteractionRequest{{
					ID:     "interaction_plugin_learning",
					Kind:   "form",
					Type:   "plugin_learning_ui_reference_request",
					Status: "waiting_for_user",
				}},
			},
		},
	})
	if len(requests) != 1 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].ID != "interaction_plugin_learning" || requests[0].Type != "plugin_learning_ui_reference_request" {
		t.Fatalf("request = %#v", requests[0])
	}
}
