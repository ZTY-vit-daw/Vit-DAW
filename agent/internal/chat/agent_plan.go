package chat

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type AgentPlan struct {
	GoalID                string          `json:"goal_id,omitempty"`
	RunID                 string          `json:"run_id,omitempty"`
	Status                string          `json:"status,omitempty"`
	Summary               string          `json:"summary,omitempty"`
	CurrentStep           string          `json:"current_step,omitempty"`
	Steps                 []AgentPlanStep `json:"steps,omitempty"`
	NeedsClarification    bool            `json:"needs_clarification,omitempty"`
	ClarificationQuestion string          `json:"clarification_question,omitempty"`
	FailureReason         string          `json:"failure_reason,omitempty"`
	ProjectHistory        map[string]any  `json:"project_history,omitempty"`
}

type AgentPlanStep struct {
	ID          string         `json:"id,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Evidence    string         `json:"evidence,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

func agentPlanFromAgentLoopResult(res agentloop.Result) *AgentPlan {
	if res.GoalID == "" && res.RunID == "" && res.Status == "" && len(res.PlanItems) == 0 {
		return nil
	}
	plan := &AgentPlan{
		GoalID:                res.GoalID,
		RunID:                 res.RunID,
		Status:                string(res.Status),
		Summary:               firstNonEmpty(res.GoalSummary, res.Goal.Summary),
		CurrentStep:           firstNonEmpty(res.CurrentStep, currentStepFromPlannerItems(res.PlanItems)),
		Steps:                 agentPlanStepsFromPlannerItems(res.PlanItems),
		NeedsClarification:    res.NeedsClarification,
		ClarificationQuestion: strings.TrimSpace(res.ClarificationQuestion),
		FailureReason:         firstNonEmpty(res.FailureReason, res.Error),
		ProjectHistory:        cloneContext(res.ProjectHistory),
	}
	return plan
}

func agentPlanFromRuntimeGoal(goal agentruntime.Goal, projectHistory map[string]any, cont *agentloop.Continuation) *AgentPlan {
	if goal.GoalID == "" && goal.RunID == "" && goal.Status == "" && cont == nil {
		return nil
	}
	if goal.Status == agentruntime.StatusIdle &&
		strings.TrimSpace(goal.GoalID) == "" &&
		strings.TrimSpace(goal.RunID) == "" &&
		strings.TrimSpace(goal.Summary) == "" &&
		strings.TrimSpace(goal.Error) == "" &&
		cont == nil {
		return nil
	}
	var steps []planner.PlanItem
	var currentStep string
	if cont != nil {
		steps = append([]planner.PlanItem(nil), cont.PlanItems...)
		if cont.PendingToolCall != nil {
			currentStep = firstNonEmpty(cont.PendingToolCall.Reason, cont.PendingToolCall.Tool)
		}
	}
	if currentStep == "" {
		currentStep = currentStepFromPlannerItems(steps)
	}
	plan := &AgentPlan{
		GoalID:         firstNonEmpty(goal.GoalID, continuationGoalID(cont)),
		RunID:          firstNonEmpty(goal.RunID, continuationRunID(cont)),
		Status:         string(goal.Status),
		Summary:        firstNonEmpty(goal.Summary, continuationSummary(cont)),
		CurrentStep:    currentStep,
		Steps:          agentPlanStepsFromPlannerItems(steps),
		FailureReason:  goal.Error,
		ProjectHistory: cloneContext(projectHistory),
	}
	if goal.Status == agentruntime.StatusWaitingClarification {
		plan.NeedsClarification = true
	}
	return plan
}

func agentPlanStepsFromPlannerItems(items []planner.PlanItem) []AgentPlanStep {
	if len(items) == 0 {
		return nil
	}
	out := make([]AgentPlanStep, 0, len(items))
	for _, item := range items {
		step := AgentPlanStep{
			ID:          strings.TrimSpace(item.ID),
			Description: strings.TrimSpace(item.Description),
			Status:      strings.TrimSpace(item.Status),
			Evidence:    strings.TrimSpace(item.Evidence),
			Metadata:    cloneContext(item.Metadata),
		}
		if step.Status == "" {
			step.Status = "pending"
		}
		if step.ID == "" && step.Description == "" {
			continue
		}
		out = append(out, step)
	}
	return out
}

func currentStepFromPlannerItems(items []planner.PlanItem) string {
	for _, status := range []string{"running", "waiting_confirmation", "pending"} {
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item.Status), status) {
				return firstNonEmpty(item.Description, item.ID)
			}
		}
	}
	return ""
}

func (s *Server) activeGoalPlan(goal agentruntime.Goal, projectHistory map[string]any) *AgentPlan {
	if s == nil {
		return agentPlanFromRuntimeGoal(goal, projectHistory, nil)
	}
	s.mu.Lock()
	cont, ok := s.goalContinuations[goal.GoalID]
	s.mu.Unlock()
	if !ok {
		return agentPlanFromRuntimeGoal(goal, projectHistory, nil)
	}
	return agentPlanFromRuntimeGoal(goal, projectHistory, &cont)
}

func syncAgentPlanProjectHistory(resp *ChatResponse) {
	if resp == nil || resp.AgentPlan == nil {
		return
	}
	resp.AgentPlan.ProjectHistory = cloneContext(resp.ProjectHistory)
	if resp.AgentPlan.GoalID == "" {
		resp.AgentPlan.GoalID = resp.GoalID
	}
	if resp.AgentPlan.RunID == "" {
		resp.AgentPlan.RunID = resp.RunID
	}
	if resp.AgentPlan.Status == "" {
		resp.AgentPlan.Status = resp.GoalStatus
	}
	if resp.AgentPlan.Summary == "" {
		resp.AgentPlan.Summary = resp.GoalSummary
	}
	if resp.AgentPlan.CurrentStep == "" {
		resp.AgentPlan.CurrentStep = resp.CurrentStep
	}
}

func simpleAgentPlan(goalID, runID string, status agentruntime.GoalStatus, summary, currentStep, failureReason string, projectHistory map[string]any) *AgentPlan {
	if strings.TrimSpace(goalID) == "" && strings.TrimSpace(runID) == "" && status == "" {
		return nil
	}
	return &AgentPlan{
		GoalID:         strings.TrimSpace(goalID),
		RunID:          strings.TrimSpace(runID),
		Status:         string(status),
		Summary:        strings.TrimSpace(summary),
		CurrentStep:    strings.TrimSpace(currentStep),
		FailureReason:  strings.TrimSpace(failureReason),
		ProjectHistory: cloneContext(projectHistory),
	}
}

func agentPlanFromPendingPlan(plan PendingPlan, status agentruntime.GoalStatus, projectHistory map[string]any) *AgentPlan {
	goalID, runID := goalIDsFromContext(plan.Context)
	if plan.GoalContinuation != nil {
		base := agentPlanFromRuntimeGoal(agentruntime.Goal{
			GoalID:  firstNonEmpty(goalID, plan.GoalContinuation.GoalID),
			RunID:   firstNonEmpty(runID, plan.GoalContinuation.RunID),
			Status:  status,
			Summary: firstNonEmpty(plan.GoalContinuation.Summary, plan.GoalContinuation.UserText),
		}, projectHistory, plan.GoalContinuation)
		if base != nil {
			base.ProjectHistory = cloneContext(projectHistory)
		}
		return base
	}
	return simpleAgentPlan(goalID, runID, status, "", "", "", projectHistory)
}

func inferredGoalStatus(resp ChatResponse) string {
	switch {
	case resp.NeedsConfirmation:
		return string(agentruntime.StatusWaitingConfirmation)
	case strings.TrimSpace(resp.Error) != "":
		return string(agentruntime.StatusFailed)
	default:
		return string(agentruntime.StatusCompleted)
	}
}

func continuationGoalID(cont *agentloop.Continuation) string {
	if cont == nil {
		return ""
	}
	return cont.GoalID
}

func continuationRunID(cont *agentloop.Continuation) string {
	if cont == nil {
		return ""
	}
	return cont.RunID
}

func continuationSummary(cont *agentloop.Continuation) string {
	if cont == nil {
		return ""
	}
	return firstNonEmpty(cont.Summary, cont.UserText)
}

func agentPlanStatusText(plan *AgentPlan) string {
	if plan == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(plan.Status))
}
