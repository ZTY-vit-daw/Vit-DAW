package capabilityadapters

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/orchestration"
)

type LowEndRelationPlanRequest struct {
	SessionID  string
	Goal       string
	Mode       orchestration.InteractionMode
	ProjectCut orchestration.ProjectCut
	Input      capabilitycontext.LowEndRelationInput
}

type LowEndRelationPlanResult struct {
	Outcome orchestration.CapabilityOutcome
	Bundle  orchestration.ContextBundle
	Pack    capabilitycontext.LowEndRelationPack
}

// PlanLowEndRelation is the read/compute-only B4 diagnosis adapter. It always
// produces OutcomeAnalysis; chat orchestration may subsequently ask the LLM
// for a treatment plan and freeze generic-EQ load/parameter Proposals.
func PlanLowEndRelation(request LowEndRelationPlanRequest) (LowEndRelationPlanResult, error) {
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" {
		return LowEndRelationPlanResult{}, fmt.Errorf("session id and goal are required")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	request.Input.UserIntent = request.Goal
	pack := capabilitycontext.BuildLowEndRelationPack(request.Input)
	bundle := capabilitycontext.LowEndRelationOrchestrationBundle(pack, request.ProjectCut.Hash)

	outcome := orchestration.CapabilityOutcome{
		CapabilityID: pack.CapabilityID,
		Context:      &bundle,
		EvidenceRefs: append([]string(nil), pack.EvidenceRefs...),
	}
	if !pack.Readiness.CanProceed {
		outcome.Kind = orchestration.OutcomeBlocked
		outcome.Summary = fmt.Sprintf("B4 低频关系分析被阻塞：%s", strings.Join(pack.Readiness.BlockedBy, "、"))
		outcome.Blockers = append([]string(nil), pack.Readiness.BlockedBy...)
	} else {
		// B4 v0 is always analysis — it never graduates to OutcomeProposal.
		outcome.Kind = orchestration.OutcomeAnalysis
		outcome.Summary = fmt.Sprintf("B4 分析了 %d 条低频相关轨道：%d 处冲突，低频倾向：%s",
			pack.AnalyzedTrackCount, pack.Summary.ConflictCount, lowEndTendencyText(pack.Summary.LowEndTendency))
	}
	return LowEndRelationPlanResult{Outcome: outcome, Bundle: bundle, Pack: pack}, nil
}

// lowEndTendencyText renders lowendrelation.LowEndSummary.LowEndTendency
// (see internal/mom/project_relation.go tendencyState) as Chinese for the
// user-facing reply; the raw value is still preserved in workflow_data.
func lowEndTendencyText(tendency string) string {
	switch tendency {
	case "prominent":
		return "明显偏多"
	case "present":
		return "存在但不明显"
	default:
		return "暂无法判断"
	}
}
