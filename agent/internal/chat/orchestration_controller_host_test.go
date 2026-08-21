package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
)

func projectMixDecisionContext() map[string]any {
	return contextWithSemanticEntryDecision(map[string]any{"project_uuid": "project-1"}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller: string(orchestrationcontroller.ProjectMixWorkflow), TargetScope: semanticEntryScopeProjectContext,
		ControlMode: semanticEntryControlSemanticLoop, UserAuthorization: semanticEntryAuthorizationAction,
		Confidence: 0.95, Reason: "explicit whole-project fixed mix workflow",
	})
}

func TestProjectMixControllerOwnsContinuationWithoutStartingFreeState(t *testing.T) {
	server := audioClosureTestServer()
	ctx, owner, active, err := server.prepareProjectMixController("conversation-mix", projectMixDecisionContext())
	if err != nil || !active || owner.Controller != orchestrationcontroller.ProjectMixWorkflow {
		t.Fatalf("project mix owner: active=%v owner=%+v err=%v", active, owner, err)
	}
	if shouldStartFreeStateReasoningLoop("mix the whole project", ctx) {
		t.Fatal("project mix controller incorrectly entered free-state closure adapter")
	}
	waiting := agentloop.Result{
		Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{GoalID: "goal-mix"},
	}
	response := server.projectMixControllerResponse(ChatResponse{ConversationID: "conversation-mix", GoalStatus: string(agentruntime.StatusWaitingContinue)}, owner, waiting)
	if _, ok := server.controllerOwners.Active("conversation-mix"); !ok {
		t.Fatal("project mix owner was released at a continuation boundary")
	}
	if response.Workflow != "project_mix_workflow" || response.WorkflowData[orchestrationOwnerContextKey] == nil {
		t.Fatalf("project mix response omitted owner contract: %+v", response)
	}
	completed := agentloop.Result{Status: agentruntime.StatusCompleted, StopReason: agentloop.StopReasonDone}
	server.projectMixControllerResponse(ChatResponse{ConversationID: "conversation-mix", GoalStatus: string(agentruntime.StatusCompleted)}, owner, completed)
	if _, ok := server.controllerOwners.Active("conversation-mix"); ok {
		t.Fatal("terminal project mix retained controller ownership")
	}
}

func TestActiveProjectMixOwnerReconstructsTrustedSelectorContext(t *testing.T) {
	server := audioClosureTestServer()
	_, owner, _, err := server.prepareProjectMixController("conversation-mix", projectMixDecisionContext())
	if err != nil {
		t.Fatal(err)
	}
	ctx := server.bindActiveOrchestrationController("conversation-mix", map[string]any{"project_uuid": "project-1"})
	decision, ok := orchestrationControllerDecisionFromContext(ctx)
	if !ok || decision.Controller != orchestrationcontroller.ProjectMixWorkflow {
		t.Fatalf("selector context not restored: %+v ok=%v", decision, ok)
	}
	entry, ok := semanticEntryDecisionFromContext(ctx)
	if !ok || entry.Route != semanticEntryRouteOpenSemantic || entry.TargetScope != semanticEntryScopeProjectContext || entry.Controller != "" {
		t.Fatalf("pure semantic context not restored separately from controller: %+v ok=%v owner=%+v", entry, ok, owner)
	}
}

func TestUnavailableProjectMixFailsClosedWithoutOrdinaryFallback(t *testing.T) {
	server := audioClosureTestServer()
	ctx, owner, active, err := server.prepareProjectMixController("conversation-mix", projectMixDecisionContext())
	if err != nil || !active || projectMixWorkflowV1Available(ctx) {
		t.Fatalf("project mix availability setup: active=%v owner=%+v err=%v", active, owner, err)
	}
	response := server.projectMixUnavailableResponse("conversation-mix", agentModeDefault, owner, ctx)
	if response.StopReason != "capability_unavailable" || response.GoalStatus != string(agentruntime.StatusFailed) || response.Workflow != "project_mix_workflow" {
		t.Fatalf("project mix did not fail closed: %+v", response)
	}
	if _, ok := server.controllerOwners.Active("conversation-mix"); ok {
		t.Fatal("unavailable project mix retained continuation ownership")
	}
}
