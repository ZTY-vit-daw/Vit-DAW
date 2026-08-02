package chat

import (
	"context"
	"fmt"

	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/frequencycleanup"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
)

func (s *Server) recoverCapabilityExecution(ctx context.Context, conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.FrozenPlan == nil || s == nil || s.orchestrationRuntime == nil || s.kernel == nil || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "v1 execution recovery 缺少 FrozenPlan、Kernel 或 Harness。")
	}
	frozen := *session.FrozenPlan
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: appendUniqueStrings([]string{"capability-pack:" + frozen.ContextBundleID}, frozen.ProjectCut.ArtifactRefs...)},
		capabilityCanaryToolSchemas(s), nil, orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "recovery Context Envelope 无效："+err.Error())
	}
	var recovered orchestration.PlanningSession
	switch frozen.ActionSet.CapabilityID {
	case staticBalanceCapabilityID:
		recovered, err = s.orchestrationRuntime.ReconcileActionSet(
			ctx, session.ID, frozen.ActionSet,
			&executionports.StaticBalanceVSPPort{Client: s.kernel},
			executionverifiers.StaticBalance{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{
				Invoker: s.harness, PreviousObservationID: frozen.PreviousObservationID, MixSessionID: session.ID, GoalText: session.Goal,
			}},
		)
	case panLayoutCapabilityID:
		recovered, err = s.orchestrationRuntime.ReconcileActionSet(
			ctx, session.ID, frozen.ActionSet,
			&executionports.PanLayoutVSPPort{Client: s.kernel},
			executionverifiers.PanLayout{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{
				Invoker: s.harness, PreviousObservationID: frozen.PreviousObservationID, MixSessionID: session.ID, GoalText: session.Goal,
			}},
		)
	case agentSemanticEQCapabilityID:
		recovered, err = s.orchestrationRuntime.ReconcileActionSet(
			ctx, session.ID, frozen.ActionSet,
			&semanticEQMutationPort{server: s},
			semanticEQVerifier{server: s, previousObservationID: frozen.PreviousObservationID, sessionID: session.ID, goalText: session.Goal},
		)
	case lowEndRelationCapabilityID:
		var treatment lowendrelation.TreatmentPlan
		var before lowendrelation.Model
		if len(frozen.ActionSet.Actions) == 1 {
			_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["treatment_plan"], &treatment)
			_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["diagnosis_model"], &before)
		}
		recovered, err = s.orchestrationRuntime.ReconcileActionSet(
			ctx, session.ID, frozen.ActionSet,
			&b4BatchMutationPort{server: s},
			b4BatchVerifier{server: s, treatment: treatment, before: before, sessionID: session.ID, goalText: session.Goal},
		)
	case frequencyCleanupCapabilityID:
		var before frequencycleanup.TargetPostFXBaseline
		if len(frozen.ActionSet.Actions) == 1 {
			_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["verification_context"], &before)
		}
		recovered, err = s.orchestrationRuntime.ReconcileActionSet(
			ctx, session.ID, frozen.ActionSet,
			&projectEQBatchMutationPort{server: s, spec: c1BatchSpec},
			c1BatchVerifier{server: s, before: before, sessionID: session.ID, goalText: session.Goal},
		)
	default:
		err = fmt.Errorf("unsupported recovery capability %s", frozen.ActionSet.CapabilityID)
	}
	if recovered.ID == "" {
		recovered = session
	}
	var response ChatResponse
	if frozen.ActionSet.CapabilityID == agentSemanticEQCapabilityID {
		response = semanticEQExecutionResponse(conversationID, goal, recovered, err)
	} else if frozen.ActionSet.CapabilityID == lowEndRelationCapabilityID {
		response = b4ExecutionResponse(conversationID, goal, recovered, err)
	} else if frozen.ActionSet.CapabilityID == frequencyCleanupCapabilityID {
		response = c1ExecutionResponse(conversationID, goal, recovered, err)
	} else if frozen.ActionSet.CapabilityID == panLayoutCapabilityID {
		response = panLayoutCanaryExecutionResponse(conversationID, goal, recovered, envelope, err)
	} else {
		response = capabilityCanaryExecutionResponse(conversationID, goal, recovered, envelope, err)
	}
	response.WorkflowData["recovery"] = true
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	attachMixboardDecisionProjection(&response, recovered)
	return response
}

func authorizedCapabilityWaitingResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "当前 v1 Proposal 已授权但尚未进入 Execution。FrozenPlan 不会被重新规划；请明确说“继续执行”以恢复执行，或说“取消”。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingContinue),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
			"engine_owner": session.EngineOwner, "canary_stage": "authorized_recovery_waiting",
		},
	}
}
