// Package capabilityadapters bridges existing domain capability implementations
// into the orchestration contracts without moving their solver logic.
package capabilityadapters

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/staticbalance"
)

type StaticBalancePlanRequest struct {
	SessionID  string
	Goal       string
	Mode       orchestration.InteractionMode
	ProjectCut orchestration.ProjectCut
	Input      capabilitycontext.StaticBalanceInput
}

type StaticBalancePlanResult struct {
	Outcome       orchestration.CapabilityOutcome
	Bundle        orchestration.ContextBundle
	Pack          capabilitycontext.StaticBalancePack
	PackID        string
	CandidateIDs  []string
	AnalyzedCount int
}

// RunStaticBalanceShadow binds the adapter to a v1-owned Session without
// persisting proposal or execution state. It is the first strangler seam used
// to compare the new plan with the legacy B2 path.
func RunStaticBalanceShadow(session orchestration.PlanningSession, cut orchestration.ProjectCut, input capabilitycontext.StaticBalanceInput) (StaticBalancePlanResult, error) {
	if session.EngineOwner != orchestration.EngineV1 {
		return StaticBalancePlanResult{}, fmt.Errorf("shadow requires capability_runtime_v1 session owner, got %q", session.EngineOwner)
	}
	if session.Invocation.CapabilityID != "" && session.Invocation.CapabilityID != "static_mix.static_balance.v0" {
		return StaticBalancePlanResult{}, fmt.Errorf("session capability %q is not B2 static balance", session.Invocation.CapabilityID)
	}
	return PlanStaticBalance(StaticBalancePlanRequest{
		SessionID:  session.ID,
		Goal:       session.Goal,
		Mode:       session.Invocation.InteractionMode,
		ProjectCut: cut,
		Input:      input,
	})
}

// PlanStaticBalance is a read/compute-only adapter. It never creates a grant
// and never calls a mutation port.
func PlanStaticBalance(request StaticBalancePlanRequest) (StaticBalancePlanResult, error) {
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" {
		return StaticBalancePlanResult{}, fmt.Errorf("session id and goal are required")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	request.Input.UserIntent = request.Goal
	pack := capabilitycontext.BuildStaticBalancePack(request.Input)
	bundle := capabilitycontext.OrchestrationBundle(pack, request.ProjectCut.Hash)
	result := pack.Result()
	candidates := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		candidates = append(candidates, candidate.CandidatePlanID)
	}
	outcome := orchestration.CapabilityOutcome{
		CapabilityID: pack.CapabilityID,
		Summary:      fmt.Sprintf("B2 analyzed %d tracks and produced %d candidate plans", pack.AnalyzedTrackCount, len(candidates)),
		Context:      &bundle,
		EvidenceRefs: append([]string(nil), pack.EvidenceRefs...),
	}
	if !pack.Readiness.CanProceed {
		outcome.Kind = orchestration.OutcomeBlocked
		outcome.Blockers = append([]string(nil), pack.Readiness.BlockedBy...)
	} else if request.Mode == orchestration.InteractionInspect {
		outcome.Kind = orchestration.OutcomeAnalysis
	} else {
		outcome.Kind = orchestration.OutcomeProposal
	}
	return StaticBalancePlanResult{
		Outcome:       outcome,
		Bundle:        bundle,
		Pack:          pack,
		PackID:        pack.PackID,
		CandidateIDs:  candidates,
		AnalyzedCount: pack.AnalyzedTrackCount,
	}, nil
}

// FreezeStaticBalanceProposal turns one solver-produced candidate into an
// immutable proposal. The caller still needs to obtain user authorization.
func FreezeStaticBalanceProposal(pack capabilitycontext.StaticBalancePack, cut orchestration.ProjectCut, candidateID string, revision int64) (orchestration.Proposal, error) {
	if strings.TrimSpace(cut.Hash) == "" {
		cut.Hash = cut.ComputeHash()
	}
	if strings.TrimSpace(candidateID) == "" || revision < 1 {
		return orchestration.Proposal{}, fmt.Errorf("candidate id and positive revision are required")
	}
	for _, candidate := range pack.Result().Candidates {
		if candidate.CandidatePlanID != candidateID {
			continue
		}
		actionSet := staticBalanceActionSet(pack, cut, candidate)
		targets := make([]string, 0, len(actionSet.Actions))
		for _, action := range actionSet.Actions {
			targets = append(targets, action.TargetRef)
		}
		return orchestration.Proposal{
			ID:              "proposal_" + actionSet.Hash[:16],
			Revision:        revision,
			CapabilityID:    pack.CapabilityID,
			CapabilityVer:   "v0",
			ProjectCutHash:  cut.Hash,
			CandidateID:     candidate.CandidatePlanID,
			ActionSetHash:   actionSet.Hash,
			TargetScope:     targets,
			Risk:            "bounded_reversible",
			VerificationRef: "static_mix.static_balance.verification.v0",
			Summary:         candidate.Label,
		}, nil
	}
	return orchestration.Proposal{}, fmt.Errorf("candidate %s is not present in the solver result", candidateID)
}

func StaticBalanceActionSet(pack capabilitycontext.StaticBalancePack, cut orchestration.ProjectCut, candidateID string) (orchestration.ActionSet, error) {
	if strings.TrimSpace(cut.Hash) == "" {
		cut.Hash = cut.ComputeHash()
	}
	for _, candidate := range pack.Result().Candidates {
		if candidate.CandidatePlanID == candidateID {
			return staticBalanceActionSet(pack, cut, candidate), nil
		}
	}
	return orchestration.ActionSet{}, fmt.Errorf("candidate %s is not present in the solver result", candidateID)
}

func staticBalanceActionSet(pack capabilitycontext.StaticBalancePack, cut orchestration.ProjectCut, candidate staticbalance.CandidatePlan) orchestration.ActionSet {
	actions := make([]orchestration.Action, 0, len(candidate.Actions))
	for index, action := range candidate.Actions {
		actions = append(actions, orchestration.Action{
			ID:                fmt.Sprintf("%s:%d:%s", candidate.CandidatePlanID, index, action.TrackID),
			Command:           "track_gain_adjust",
			TargetRef:         action.TrackID,
			BeforeFingerprint: fmt.Sprintf("track:%s:fader_db:%0.3f", action.TrackID, action.BeforeDB),
			Args: map[string]any{
				"track_id":                         action.TrackID,
				"before_db":                        action.BeforeDB,
				"target_db":                        action.TargetDB,
				"delta_db":                         action.DeltaDB,
				"hierarchy_role":                   action.Role,
				"hierarchy_function":               action.Function,
				"before_effective_level_db":        action.Evidence["level_db"],
				"before_effective_level_source":    action.Evidence["level_source"],
				"before_effective_level_metric":    action.Evidence["level_metric"],
				"before_effective_level_tap_point": action.Evidence["level_tap_point"],
				"before_effective_level_status":    action.Evidence["level_status"],
				"function_current_relative_db":     action.Evidence["function_current_relative_db"],
				"function_target_relative_db":      action.Evidence["function_target_relative_db"],
			},
			Compensatable:    true,
			IdempotencyClass: "absolute_target_with_before_fingerprint",
		})
	}
	set := orchestration.ActionSet{
		ID:             "actionset_" + candidate.CandidatePlanID,
		CapabilityID:   pack.CapabilityID,
		ProjectCutHash: cut.Hash,
		Actions:        actions,
	}
	set.Hash = set.ComputeHash()
	return set
}
