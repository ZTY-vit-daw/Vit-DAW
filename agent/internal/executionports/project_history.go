package executionports

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

type ProjectHistoryBaseline interface {
	EnsureCapabilityExecutionBaseline(context.Context, string, string, string) (string, error)
}

type ProjectHistory struct {
	Harness ProjectHistoryBaseline
	GoalID  string
	RunID   string
}

func (p ProjectHistory) PrepareExecution(ctx context.Context, session orchestration.PlanningSession, actionSet orchestration.ActionSet, _ orchestration.ProjectCut) ([]string, error) {
	if p.Harness == nil {
		return nil, fmt.Errorf("project history harness is required")
	}
	goalID := strings.TrimSpace(p.GoalID)
	if goalID == "" {
		goalID = session.ID
	}
	commitID, err := p.Harness.EnsureCapabilityExecutionBaseline(ctx, goalID, strings.TrimSpace(p.RunID), actionSet.CapabilityID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(commitID) == "" {
		return nil, fmt.Errorf("project history baseline returned no commit id")
	}
	return []string{"project-history:" + commitID}, nil
}
