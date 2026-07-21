package capabilityadapters

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

// SPALPlanRequest is the capability-layer seam. The caller supplies only a
// finite semantic instruction; SPAL owns Provider resolution, raw preimage
// capture and physical parameter compilation.
type SPALPlanRequest struct {
	Context           context.Context
	SessionID         string
	Goal              string
	Mode              orchestration.InteractionMode
	CapabilityID      string
	CapabilityVersion string
	ProjectCut        orchestration.ProjectCut
	Instruction       spal.Instruction
	Registry          *spal.Registry
	Instances         []spal.ProviderInstance
	ResolveOptions    spal.ResolveOptions
	PreimageReader    spal.PreimageReader
}

type SPALPlanResult struct {
	Outcome     orchestration.CapabilityOutcome
	Preparation spal.Preparation
	Bundle      orchestration.ContextBundle
}

// PlanSPAL is read/compute-only. It cannot mutate a plug-in, load a Provider,
// or authorize itself. Missing Providers become an explicit Plugin Skill
// blocker rather than a raw-parameter fallback.
func PlanSPAL(request SPALPlanRequest) (SPALPlanResult, error) {
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" || strings.TrimSpace(request.CapabilityID) == "" {
		return SPALPlanResult{}, fmt.Errorf("session id, goal and capability id are required")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	if !request.ProjectCut.IsExecutable() {
		return SPALPlanResult{}, fmt.Errorf("SPAL planning requires an executable ProjectCut")
	}
	ctx := request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	prepared, err := (spal.Runtime{Registry: request.Registry}).Prepare(ctx, request.Instruction, request.Instances, request.ResolveOptions, request.PreimageReader)
	if err != nil {
		return SPALPlanResult{}, err
	}
	bundle := orchestration.ContextBundle{
		ID:             "spal_context_" + request.SessionID,
		CapabilityID:   request.CapabilityID,
		ProjectCutHash: request.ProjectCut.Hash,
		EvidenceRefs:   append([]string(nil), request.Instruction.EvidenceRefs...),
	}
	outcome := orchestration.CapabilityOutcome{CapabilityID: request.CapabilityID, Context: &bundle, EvidenceRefs: append([]string(nil), request.Instruction.EvidenceRefs...)}
	switch prepared.Resolution.Status {
	case spal.ResolutionBound:
		if prepared.Manifest == nil {
			return SPALPlanResult{}, fmt.Errorf("bound SPAL resolution omitted execution manifest")
		}
		bundle.ArtifactRefs = []string{"spal.manifest:" + prepared.Manifest.ID}
		bundle.Disclosure = "A bounded semantic parameter action is ready for proposal confirmation."
		outcome.Summary = "A verified SPAL control is bound and ready to freeze into a Proposal."
		if request.Mode == orchestration.InteractionInspect {
			outcome.Kind = orchestration.OutcomeAnalysis
		} else {
			outcome.Kind = orchestration.OutcomeProposal
		}
	case spal.ResolutionRequiresProvisioning:
		bundle.Disclosure = "A verified Provider was explicitly selected but must be provisioned in a confirmed Proposal."
		outcome.Kind = orchestration.OutcomeNeedUserInput
		outcome.Summary = prepared.Resolution.Reason
		outcome.Blockers = []string{"spal_provider_provisioning_required"}
	default:
		bundle.Disclosure = "No verified Provider instance can implement the selected semantic control."
		outcome.Kind = orchestration.OutcomeBlocked
		outcome.Summary = prepared.Resolution.Reason
		outcome.Blockers = []string{"spal_no_verified_provider", "generate_plugin_skill"}
	}
	return SPALPlanResult{Outcome: outcome, Preparation: prepared, Bundle: bundle}, nil
}

// FreezeSPALProposal turns a completed SPAL preparation into the same immutable
// Proposal/FrozenPlan input used by B2 and B3. It is intentionally separate
// from PlanSPAL so planning cannot grant a mutation.
func FreezeSPALProposal(result SPALPlanResult, capabilityID, capabilityVersion string, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if result.Preparation.Resolution.Status != spal.ResolutionBound || result.Preparation.Manifest == nil {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("only a bound SPAL preparation can freeze a proposal")
	}
	return result.Preparation.Manifest.FreezeProposal(capabilityID, capabilityVersion, cut, revision)
}
