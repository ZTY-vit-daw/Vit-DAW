package chat

import (
	"errors"
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilityinteraction"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/protocolvalue"
	agentruntime "vit-daw-agent/internal/runtime"
)

const capabilityApprovalDecisionContextKey = "capability_approval_decision"

func contextWithCapabilityApprovalDecision(ctx map[string]any, decision orchestration.ApprovalDecision) map[string]any {
	out := cloneContext(ctx)
	if out == nil {
		out = map[string]any{}
	}
	out[capabilityApprovalDecisionContextKey] = decision
	return out
}

func capabilityApprovalDecisionFromContext(ctx map[string]any) (orchestration.ApprovalDecision, bool) {
	value, ok := ctx[capabilityApprovalDecisionContextKey]
	if !ok {
		return orchestration.ApprovalDecision{}, false
	}
	if decision, ok := value.(orchestration.ApprovalDecision); ok {
		return decision, true
	}
	row := protocolMap(value)
	if len(row) == 0 {
		return orchestration.ApprovalDecision{}, false
	}
	return orchestration.ApprovalDecision{
		SchemaVersion: cleanContextText(row["schema_version"]),
		Kind:          orchestration.ApprovalDecisionKind(cleanContextText(row["kind"])),
		ProposalID:    cleanContextText(row["proposal_id"]), ProposalRevision: int64(chatIntValue(row["proposal_revision"])),
		ActionSetHash: cleanContextText(row["action_set_hash"]), ProjectCutHash: cleanContextText(row["project_cut_hash"]),
		SourceTurnID: cleanContextText(row["source_turn_id"]), UserText: cleanContextText(row["user_text"]),
		Confidence: cleanContextText(row["confidence"]), Reason: cleanContextText(row["reason"]),
	}, true
}

func protocolMap(value any) map[string]any {
	return protocolvalue.Object(value)
}

func resolvePendingCapabilityTurn(req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession) orchestration.ApprovalDecision {
	return capabilityinteraction.Resolve(req.Message, firstNonEmpty(goal.RunID, "chat_turn"), session)
}

func expectedProposalMatches(ctx map[string]any, proposal *orchestration.Proposal) bool {
	hasExpectation := firstStringFromMap(ctx, "expected_proposal_id", "expected_action_set_hash", "expected_project_cut_hash") != "" || chatIntValue(ctx["expected_proposal_revision"]) > 0
	if !hasExpectation {
		return true
	}
	if proposal == nil {
		return false
	}
	checks := []struct {
		key  string
		want string
	}{
		{"expected_proposal_id", proposal.ID},
		{"expected_action_set_hash", proposal.ActionSetHash},
		{"expected_project_cut_hash", proposal.ProjectCutHash},
	}
	for _, check := range checks {
		if value := firstStringFromMap(ctx, check.key); value != "" && value != check.want {
			return false
		}
	}
	if revision := int64(chatIntValue(ctx["expected_proposal_revision"])); revision > 0 && revision != proposal.Revision {
		return false
	}
	return true
}

func proposalQuestionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	reply := renderProposalConversation(presentation)
	reply += "\n\n这是解释，不是授权；工程没有被修改，当前 Proposal 仍等待你的决定。"
	return capabilityProposalWaitingResponse(conversationID, goal, session, reply, "proposal_question")
}

func proposalAmbiguousResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	reply := "当前消息没有形成明确授权，因此不会执行工程修改。你可以直接说“执行这个方案”、提出问题、说明要排除/只处理哪些轨道，或说“取消方案”。"
	return capabilityProposalWaitingResponse(conversationID, goal, session, reply, "proposal_ambiguous")
}

func proposalRevisionClarificationResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, err error) ChatResponse {
	reply := "我识别到你想调整当前 Proposal，但还不能把要求安全地映射到确切轨道或数值。请明确说明，例如“不要动 bus”“只处理背景和声”“把 +0.50 dB 改成 +0.30 dB”或“换第二个候选方案”。"
	if err != nil && !errors.Is(err, capabilityinteraction.ErrRevisionNeedsClarification) {
		reply += " 当前修订没有保存：" + err.Error()
	}
	return capabilityProposalWaitingResponse(conversationID, goal, session, reply, "proposal_revision_clarification")
}

func capabilityProposalWaitingResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, reply, stage string) ChatResponse {
	if session.ActiveProposal == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Session 没有可讨论的 Proposal。")
	}
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: reply, NeedsConfirmation: true, PlanID: session.ActiveProposal.ID,
		Preview: firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: proposalWorkflowData(session, presentation, stage),
	}
}

func proposalRevisionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	reply := renderProposalConversation(presentation)
	reply += "\n\n旧 Proposal revision 已失效；只有当前显示的 revision 可以被授权。"
	return capabilityProposalWaitingResponse(conversationID, goal, session, reply, "proposal_revised")
}

func proposalWorkflowData(session orchestration.PlanningSession, presentation *orchestration.ProposalPresentation, stage string) map[string]any {
	data := map[string]any{
		"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
		"canary_stage": stage, "approval_mode": "conversational", "proposal_presentation": presentation,
	}
	if session.ActiveProposal != nil {
		data["proposal_id"] = session.ActiveProposal.ID
		data["proposal_revision"] = session.ActiveProposal.Revision
		data["action_set_hash"] = session.ActiveProposal.ActionSetHash
		data["project_cut_hash"] = session.ActiveProposal.ProjectCutHash
	}
	return data
}

func applyConversationalProposalRevision(session orchestration.PlanningSession, decision orchestration.ApprovalDecision) (orchestration.FrozenPlan, error) {
	if decision.ProposalID != session.ActiveProposal.ID || decision.ProposalRevision != session.ActiveProposal.Revision || decision.ActionSetHash != session.ActiveProposal.ActionSetHash || decision.ProjectCutHash != session.ActiveProposal.ProjectCutHash {
		return orchestration.FrozenPlan{}, fmt.Errorf("proposal changed before revision could be applied")
	}
	return capabilityinteraction.ReviseFrozenPlan(session, decision)
}

func proposalDecisionRequiresReplan(decision orchestration.ApprovalDecision) bool {
	return capabilityinteraction.RequiresReplan(decision)
}

func proposalDecisionStage(decision orchestration.ApprovalDecision) string {
	return strings.TrimSpace(string(decision.Kind))
}
