package capabilityadapters

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/panlayout"
)

type PanLayoutPlanRequest struct {
	SessionID  string
	Goal       string
	Mode       orchestration.InteractionMode
	ProjectCut orchestration.ProjectCut
	Input      capabilitycontext.PanLayoutInput
}

type PanLayoutPlanResult struct {
	Outcome       orchestration.CapabilityOutcome
	Bundle        orchestration.ContextBundle
	Pack          capabilitycontext.PanLayoutPack
	PackID        string
	CandidateIDs  []string
	AnalyzedCount int
}

func RunPanLayoutShadow(session orchestration.PlanningSession, cut orchestration.ProjectCut, input capabilitycontext.PanLayoutInput) (PanLayoutPlanResult, error) {
	if session.EngineOwner != orchestration.EngineV1 {
		return PanLayoutPlanResult{}, fmt.Errorf("shadow requires capability_runtime_v1 session owner, got %q", session.EngineOwner)
	}
	if session.Invocation.CapabilityID != "" && session.Invocation.CapabilityID != panlayout.CapabilityID {
		return PanLayoutPlanResult{}, fmt.Errorf("session capability %q is not B3 pan layout", session.Invocation.CapabilityID)
	}
	return PlanPanLayout(PanLayoutPlanRequest{SessionID: session.ID, Goal: session.Goal, Mode: session.Invocation.InteractionMode, ProjectCut: cut, Input: input})
}

func PlanPanLayout(request PanLayoutPlanRequest) (PanLayoutPlanResult, error) {
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" {
		return PanLayoutPlanResult{}, fmt.Errorf("session id and goal are required")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	request.Input.UserIntent = request.Goal
	pack := capabilitycontext.BuildPanLayoutPack(request.Input)
	bundle := capabilitycontext.PanLayoutOrchestrationBundle(pack, request.ProjectCut.Hash)
	result := pack.Result()
	candidateIDs := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		candidateIDs = append(candidateIDs, candidate.CandidatePlanID)
	}
	outcome := orchestration.CapabilityOutcome{
		CapabilityID: pack.CapabilityID, Summary: fmt.Sprintf("B3 analyzed %d tracks and produced %d candidate plans", pack.AnalyzedTrackCount, len(candidateIDs)),
		Context: &bundle, EvidenceRefs: append([]string(nil), pack.EvidenceRefs...),
	}
	if !pack.Readiness.CanProceed {
		outcome.Kind = orchestration.OutcomeBlocked
		outcome.Blockers = append([]string(nil), pack.Readiness.BlockedBy...)
	} else if request.Mode == orchestration.InteractionInspect {
		outcome.Kind = orchestration.OutcomeAnalysis
	} else {
		outcome.Kind = orchestration.OutcomeProposal
	}
	return PanLayoutPlanResult{Outcome: outcome, Bundle: bundle, Pack: pack, PackID: pack.PackID, CandidateIDs: candidateIDs, AnalyzedCount: pack.AnalyzedTrackCount}, nil
}

func FreezePanLayoutProposal(pack capabilitycontext.PanLayoutPack, cut orchestration.ProjectCut, candidateID string, revision int64) (orchestration.Proposal, error) {
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	if strings.TrimSpace(candidateID) == "" || revision < 1 {
		return orchestration.Proposal{}, fmt.Errorf("candidate id and positive revision are required")
	}
	for _, candidate := range pack.Result().Candidates {
		if candidate.CandidatePlanID != candidateID {
			continue
		}
		actionSet := panLayoutActionSet(pack, cut, candidate)
		targets := make([]string, 0, len(actionSet.Actions))
		for _, action := range actionSet.Actions {
			targets = append(targets, action.TargetRef)
		}
		return orchestration.Proposal{
			ID: "proposal_" + actionSet.Hash[:16], Revision: revision,
			CapabilityID: pack.CapabilityID, CapabilityVer: "v0", ProjectCutHash: cut.Hash,
			CandidateID: candidate.CandidatePlanID, ActionSetHash: actionSet.Hash, TargetScope: targets,
			Risk: "bounded_reversible", VerificationRef: "static_mix.pan_layout.verification.v0", Summary: candidate.Label,
		}, nil
	}
	return orchestration.Proposal{}, fmt.Errorf("candidate %s is not present in the solver result", candidateID)
}

func PanLayoutActionSet(pack capabilitycontext.PanLayoutPack, cut orchestration.ProjectCut, candidateID string) (orchestration.ActionSet, error) {
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	for _, candidate := range pack.Result().Candidates {
		if candidate.CandidatePlanID == candidateID {
			return panLayoutActionSet(pack, cut, candidate), nil
		}
	}
	return orchestration.ActionSet{}, fmt.Errorf("candidate %s is not present in the solver result", candidateID)
}

func panLayoutActionSet(pack capabilitycontext.PanLayoutPack, cut orchestration.ProjectCut, candidate panlayout.CandidatePlan) orchestration.ActionSet {
	actions := make([]orchestration.Action, 0, len(candidate.Actions))
	for index, action := range candidate.Actions {
		actions = append(actions, orchestration.Action{
			ID:      fmt.Sprintf("%s:%d:%s", candidate.CandidatePlanID, index, action.TrackID),
			Command: "track_pan_set", TargetRef: action.TrackID,
			BeforeFingerprint: fmt.Sprintf("track:%s:pan:%0.3f", action.TrackID, action.BeforePan),
			Args:              map[string]any{"track_id": action.TrackID, "before_pan": action.BeforePan, "target_pan": action.TargetPan, "delta_pan": action.DeltaPan},
			Compensatable:     true, IdempotencyClass: "absolute_target_with_before_fingerprint",
		})
	}
	set := orchestration.ActionSet{ID: "actionset_" + candidate.CandidatePlanID, CapabilityID: pack.CapabilityID, ProjectCutHash: cut.Hash, Actions: actions}
	set.Hash = set.ComputeHash()
	return set
}
