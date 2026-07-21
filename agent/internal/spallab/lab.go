package spallab

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

// Lab composes a ProviderStore with the existing governed orchestration path.
// It contains no B4 diagnosis logic and deliberately does not expose a plugin
// loading or provisioning operation.
type Lab struct {
	Providers *ProviderStore
	Sessions  orchestration.Store
	Executor  *executionruntime.Coordinator
}

func New(providers *ProviderStore, sessions orchestration.Store) (*Lab, error) {
	if providers == nil || sessions == nil {
		return nil, errors.New("SPAL lab provider and session stores are required")
	}
	if !orchestration.IsDurableStore(sessions) {
		return nil, errors.New("SPAL lab requires a durable orchestration store")
	}
	return &Lab{Providers: providers, Sessions: sessions, Executor: executionruntime.New(sessions)}, nil
}

// Plan captures the physical preimage and creates a frozen Proposal, but only
// after resolving one explicitly conformed instance. It never mutates a plug-in.
func (l *Lab) Plan(ctx context.Context, request PlanRequest) (PlanResult, error) {
	if l == nil || l.Providers == nil || l.Sessions == nil {
		return PlanResult{}, errors.New("SPAL lab is not initialized")
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.Goal) == "" || request.PreimageReader == nil {
		return PlanResult{}, errors.New("session_id, goal and preimage reader are required")
	}
	if !request.ProjectCut.IsExecutable() {
		return PlanResult{}, errors.New("SPAL lab planning requires an executable Project Cut")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	records, err := l.Providers.List()
	if err != nil {
		return PlanResult{}, err
	}
	for _, record := range records {
		if record.Instance.TargetRef != request.Instruction.TargetRef {
			continue
		}
		if request.ValidateRecord == nil {
			return PlanResult{}, errors.New("SPAL lab planning requires a current Provider record validator")
		}
		if err := request.ValidateRecord(ctx, record); err != nil {
			return PlanResult{}, fmt.Errorf("validate current SPAL Provider record %s: %w", record.ID, err)
		}
	}
	registry, err := l.Providers.Registry()
	if err != nil {
		return PlanResult{}, err
	}
	instances := make([]spal.ProviderInstance, 0, len(records))
	for _, record := range records {
		instances = append(instances, record.Instance)
	}
	planned, err := capabilityadapters.PlanSPAL(capabilityadapters.SPALPlanRequest{
		Context:           ctx,
		SessionID:         request.SessionID,
		Goal:              request.Goal,
		Mode:              orchestration.InteractionPropose,
		CapabilityID:      StaticBellCapability,
		CapabilityVersion: StaticBellVersion,
		ProjectCut:        request.ProjectCut,
		Instruction:       request.Instruction,
		Registry:          registry,
		Instances:         instances,
		PreimageReader:    request.PreimageReader,
	})
	if err != nil {
		return PlanResult{}, err
	}
	result := PlanResult{Outcome: planned.Outcome, Bundle: planned.Bundle}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || planned.Preparation.Manifest == nil {
		return result, nil
	}
	proposal, actionSet, err := capabilityadapters.FreezeSPALProposal(planned, StaticBellCapability, StaticBellVersion, request.ProjectCut, 1)
	if err != nil {
		return PlanResult{}, err
	}
	session, err := orchestration.NewSession(request.SessionID, request.ProjectCut.ProjectUUID, request.Goal, orchestration.EngineV1, orchestration.CapabilityInvocation{
		CapabilityID:    StaticBellCapability,
		CapabilityVer:   StaticBellVersion,
		InteractionMode: orchestration.InteractionPropose,
		ProcessingPath:  orchestration.PathCapability,
		Goal:            request.Goal,
		TargetRefs:      []string{request.Instruction.TargetRef},
		Constraints:     []string{"spal_lab_only", "no_provider_auto_provisioning"},
	})
	if err != nil {
		return PlanResult{}, err
	}
	session.Constraints = append(session.Constraints, "spal_lab_only", "no_provider_auto_provisioning")
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: request.ProjectCut,
		ContextBundleID: planned.Bundle.ID,
	})
	if err != nil {
		return PlanResult{}, err
	}
	if err := l.Sessions.Create(session); err != nil {
		return PlanResult{}, err
	}
	result.Proposal = &proposal
	result.ActionSet = &actionSet
	result.Session = &session
	return result, nil
}

// PlanRollback makes rollback another Proposal rather than an imperative
// side-channel. It refuses to overwrite a manual or concurrent change between
// the original execution and this rollback plan.
func (l *Lab) PlanRollback(ctx context.Context, request RollbackPlanRequest) (PlanResult, error) {
	if l == nil || l.Sessions == nil {
		return PlanResult{}, errors.New("SPAL lab is not initialized")
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.OriginalSessionID) == "" || request.PreimageReader == nil {
		return PlanResult{}, errors.New("session_id, original_session_id and preimage reader are required")
	}
	if !request.ProjectCut.IsExecutable() {
		return PlanResult{}, errors.New("SPAL lab rollback planning requires an executable Project Cut")
	}
	if request.ProjectCut.Hash == "" {
		request.ProjectCut.Hash = request.ProjectCut.ComputeHash()
	}
	original, ok := l.Sessions.Load(strings.TrimSpace(request.OriginalSessionID))
	if !ok || original.FrozenPlan == nil || original.Execution == nil {
		return PlanResult{}, fmt.Errorf("original SPAL lab execution %s was not found", request.OriginalSessionID)
	}
	if len(original.FrozenPlan.ActionSet.Actions) != 1 || !allApplied(original.Execution.Receipts) {
		return PlanResult{}, errors.New("only a fully applied single-action SPAL lab execution can be rolled back")
	}
	originalManifest, err := spal.ManifestFromAction(original.FrozenPlan.ActionSet.Actions[0])
	if err != nil {
		return PlanResult{}, err
	}
	records, err := l.Providers.List()
	if err != nil {
		return PlanResult{}, err
	}
	matchedRecord := false
	for _, record := range records {
		if record.Instance.ID != originalManifest.Binding.Instance.ID {
			continue
		}
		matchedRecord = true
		if request.ValidateRecord == nil {
			return PlanResult{}, errors.New("SPAL lab rollback planning requires a current Provider record validator")
		}
		if err := request.ValidateRecord(ctx, record); err != nil {
			return PlanResult{}, fmt.Errorf("validate current SPAL Provider record for rollback %s: %w", record.ID, err)
		}
		break
	}
	if !matchedRecord {
		return PlanResult{}, errors.New("original SPAL lab Provider record is no longer registered")
	}
	current, err := request.PreimageReader.CaptureSPALPreimage(ctx, originalManifest.Binding, originalManifest.Preimage)
	if err != nil {
		return PlanResult{}, fmt.Errorf("capture current parameters for SPAL rollback: %w", err)
	}
	rollback, err := spal.NewRollbackManifest(originalManifest, current)
	if err != nil {
		return PlanResult{}, err
	}
	proposal, actionSet, err := rollback.FreezeProposal(StaticBellCapability, StaticBellVersion, request.ProjectCut, 1)
	if err != nil {
		return PlanResult{}, err
	}
	goal := strings.TrimSpace(request.Goal)
	if goal == "" {
		goal = "Restore SPAL Lab preimage for " + request.OriginalSessionID
	}
	session, err := orchestration.NewSession(request.SessionID, request.ProjectCut.ProjectUUID, goal, orchestration.EngineV1, orchestration.CapabilityInvocation{
		CapabilityID: StaticBellCapability, CapabilityVer: StaticBellVersion,
		InteractionMode: orchestration.InteractionPropose, ProcessingPath: orchestration.PathCapability,
		Goal: goal, TargetRefs: []string{rollback.Instruction.TargetRef},
		Constraints: []string{"spal_lab_only", "explicit_rollback_proposal", "no_provider_auto_provisioning"},
	})
	if err != nil {
		return PlanResult{}, err
	}
	session.Constraints = append(session.Constraints, "spal_lab_only", "explicit_rollback_proposal", "no_provider_auto_provisioning")
	bundle := orchestration.ContextBundle{
		ID:             "spal_rollback_context_" + request.SessionID,
		CapabilityID:   StaticBellCapability,
		ProjectCutHash: request.ProjectCut.Hash,
		EvidenceRefs:   append([]string(nil), rollback.EvidenceRefs...),
		ArtifactRefs:   []string{"spal.rollback_manifest:" + rollback.ID, "spal.original_manifest:" + originalManifest.ID},
		Disclosure:     "A separately confirmable rollback to the original frozen physical preimage is ready.",
	}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: request.ProjectCut,
		ContextBundleID: bundle.ID,
	})
	if err != nil {
		return PlanResult{}, err
	}
	if err := l.Sessions.Create(session); err != nil {
		return PlanResult{}, err
	}
	return PlanResult{
		Outcome: orchestration.CapabilityOutcome{
			Kind: orchestration.OutcomeProposal, CapabilityID: StaticBellCapability,
			Summary:      "A bounded SPAL rollback proposal is ready for exact confirmation.",
			EvidenceRefs: append([]string(nil), rollback.EvidenceRefs...), Context: &bundle,
		},
		Bundle:   bundle,
		Proposal: &proposal, ActionSet: &actionSet, Session: &session,
	}, nil
}

// Approve is intentionally separate from Plan and Execute. The caller must
// name the exact Proposal ID that was displayed to the lab operator.
func (l *Lab) Approve(sessionID, proposalID, sourceTurnID string) (orchestration.PlanningSession, error) {
	if l == nil || l.Sessions == nil {
		return orchestration.PlanningSession{}, errors.New("SPAL lab is not initialized")
	}
	session, ok := l.Sessions.Load(strings.TrimSpace(sessionID))
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("SPAL lab session %s not found", sessionID)
	}
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return orchestration.PlanningSession{}, errors.New("SPAL lab session has no frozen proposal")
	}
	if strings.TrimSpace(proposalID) == "" || proposalID != session.ActiveProposal.ID {
		return orchestration.PlanningSession{}, errors.New("exact frozen SPAL lab proposal id is required for approval")
	}
	sourceTurnID = strings.TrimSpace(sourceTurnID)
	if sourceTurnID == "" {
		return orchestration.PlanningSession{}, errors.New("approval source turn is required")
	}
	decision := orchestration.ApprovalDecision{
		SchemaVersion:    orchestration.ApprovalDecisionSchema,
		Kind:             orchestration.ApprovalApprove,
		ProposalID:       session.ActiveProposal.ID,
		ProposalRevision: session.ActiveProposal.Revision,
		ActionSetHash:    session.ActiveProposal.ActionSetHash,
		ProjectCutHash:   session.ActiveProposal.ProjectCutHash,
		SourceTurnID:     sourceTurnID,
	}
	expected := session.Revision
	updated, err := session.Authorize(orchestration.Authorization{
		ProposalID: decision.ProposalID, ProposalRevision: decision.ProposalRevision,
		ActionSetHash: decision.ActionSetHash, ProjectCutHash: decision.ProjectCutHash,
		SourceTurnID: sourceTurnID, Sequence: 1, Decision: &decision,
	})
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := l.Sessions.Save(updated, expected); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return updated, nil
}

// Execute consumes only an already-authorized frozen plan. The existing
// Coordinator owns Project Cut checks, durable receipts and reconciliation.
func (l *Lab) Execute(ctx context.Context, sessionID string, port executionruntime.MutationPort, verifier executionruntime.Verifier, persistence executionruntime.PersistencePort) (orchestration.PlanningSession, error) {
	if l == nil || l.Sessions == nil || l.Executor == nil {
		return orchestration.PlanningSession{}, errors.New("SPAL lab is not initialized")
	}
	session, ok := l.Sessions.Load(strings.TrimSpace(sessionID))
	if !ok || session.FrozenPlan == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("SPAL lab frozen session %s not found", sessionID)
	}
	return l.Executor.ExecuteWithPersistence(ctx, session.ID, session.FrozenPlan.ActionSet, session.FrozenPlan.ProjectCut, port, verifier, persistence)
}

func (l *Lab) Session(sessionID string) (orchestration.PlanningSession, bool) {
	if l == nil || l.Sessions == nil {
		return orchestration.PlanningSession{}, false
	}
	return l.Sessions.Load(strings.TrimSpace(sessionID))
}

func allApplied(receipts []orchestration.ActionReceipt) bool {
	if len(receipts) == 0 {
		return false
	}
	for _, receipt := range receipts {
		if !strings.EqualFold(strings.TrimSpace(receipt.Status), "applied") {
			return false
		}
	}
	return true
}
