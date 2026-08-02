package capabilityadapters

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/orchestration"
)

type FrequencyCleanupPlanRequest struct {
	SessionID  string
	Goal       string
	Mode       orchestration.InteractionMode
	ProjectCut orchestration.ProjectCut
	Input      capabilitycontext.FrequencyCleanupInput
}
type FrequencyCleanupPlanResult struct {
	Outcome orchestration.CapabilityOutcome
	Bundle  orchestration.ContextBundle
	Pack    capabilitycontext.FrequencyCleanupPack
}

func PlanFrequencyCleanup(request FrequencyCleanupPlanRequest) (FrequencyCleanupPlanResult, error) {
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" {
		return FrequencyCleanupPlanResult{}, fmt.Errorf("session id and goal are required")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	request.Input.UserIntent = request.Goal
	pack := capabilitycontext.BuildFrequencyCleanupPack(request.Input)
	bundle := capabilitycontext.FrequencyCleanupOrchestrationBundle(pack, request.ProjectCut.Hash)
	outcome := orchestration.CapabilityOutcome{CapabilityID: pack.CapabilityID, Context: &bundle, EvidenceRefs: append([]string(nil), pack.EvidenceRefs...)}
	if !pack.Readiness.Diagnosis.CanProceed {
		outcome.Kind = orchestration.OutcomeBlocked
		outcome.Summary = "C1 frequency diagnosis readiness is blocked"
		outcome.Blockers = append([]string(nil), pack.Readiness.Diagnosis.BlockedBy...)
	} else {
		outcome.Kind = orchestration.OutcomeAnalysis
		outcome.Summary = fmt.Sprintf("C1 analyzed %d project tracks and found %d observable frequency candidates", pack.AnalyzedTrackCount, len(pack.Candidates))
	}
	return FrequencyCleanupPlanResult{Outcome: outcome, Bundle: bundle, Pack: pack}, nil
}
