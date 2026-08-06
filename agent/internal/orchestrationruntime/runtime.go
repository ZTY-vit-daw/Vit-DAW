// Package orchestrationruntime composes the first v1 migration seam. It is
// intentionally small: planning is read/compute-only until a later phase adds
// the Execution Coordinator.
package orchestrationruntime

import (
	"context"
	"encoding/json"
	"fmt"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
)

const StaticBalanceCapabilityID = "static_mix.static_balance.v0"
const PanLayoutCapabilityID = "static_mix.pan_layout.v0"
const LowEndRelationCapabilityID = "static_mix.low_end_relation.v0"
const FrequencyCleanupCapabilityID = "fine_mix.frequency_cleanup.v1"
const AgentSemanticEQCapabilityID = "agent.effect.eq_control.v0"

const capabilityRuntimeSystemContext = "Vit Project-aware Capability Runtime v1. Use the fixed PlanningSession engine owner, typed ContextBundle, ProjectCut, Proposal, Authorization, ActionSet, Execution and Verification contracts. Readiness is not authorization. Full derived models remain behind evidence or artifact references unless explicitly requested and admitted by budget."

type Runtime struct {
	Registry *orchestration.Registry
	Store    orchestration.Store
	Executor *executionruntime.Coordinator
}

func New() *Runtime {
	return NewWithStore(orchestration.NewMemoryStore())
}

func NewWithStore(store orchestration.Store) *Runtime {
	return &Runtime{Registry: orchestration.DefaultRegistry(), Store: store, Executor: executionruntime.New(store)}
}

func (r *Runtime) StartB2Session(sessionID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, "", projectUUID, goal, mode, StaticBalanceCapabilityID, "v0", nil)
}

func (r *Runtime) StartB3Session(sessionID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, "", projectUUID, goal, mode, PanLayoutCapabilityID, "v0", nil)
}

func (r *Runtime) StartB2ChatSession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, conversationID, projectUUID, goal, mode, StaticBalanceCapabilityID, "v0", nil)
}

func (r *Runtime) StartB3ChatSession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, conversationID, projectUUID, goal, mode, PanLayoutCapabilityID, "v0", nil)
}

// StartB4ChatSession starts the full-project B4 relationship workflow. Its
// mutation phases reuse the ordinary generic-EQ contract and executor.
func (r *Runtime) StartB4ChatSession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, conversationID, projectUUID, goal, mode, LowEndRelationCapabilityID, "v0", []string{
		"full_project_observation_and_treatment",
		"plugin_load_and_eq_parameters_separately_confirmed",
		"exact_track_and_plugin_identity_at_execution_leaves",
		"project_wide_all_or_rollback",
		"reuse_ordinary_generic_eq_runtime",
		"no_profile_learn_spal_network_or_b5",
	})
}

func (r *Runtime) StartC1ChatSession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, conversationID, projectUUID, goal, mode, FrequencyCleanupCapabilityID, "v1", []string{
		"full_project_frequency_relationship_diagnosis",
		"explicit_classification_for_every_project_track",
		"static_eq_only_dynamic_space_automation_deferred",
		"full_project_same_tap_post_fx_baseline_required_for_mutation",
		"plugin_load_and_eq_parameters_separately_confirmed",
		"project_wide_all_or_rollback",
		"reuse_ordinary_generic_eq_runtime",
		"no_new_observer_recognizer_topology_or_executor",
	})
}

// StartAgentSemanticEQChatSession starts the horizontal ordinary-Agent EQ
// governance path. It is intentionally separate from B4 and every A-F
// specialist capability even though those layers may later reuse its frozen
// semantic-action contract.
func (r *Runtime) StartAgentSemanticEQChatSession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode) (orchestration.PlanningSession, error) {
	return r.startCapabilitySession(sessionID, conversationID, projectUUID, goal, mode, AgentSemanticEQCapabilityID, "v0", []string{
		"generic_static_eq_only",
		"llm_acoustic_judgement_must_be_frozen",
		"no_stored_mapping_or_plugin_loading",
		"frozen_topology_and_parameter_preimage_required",
		"structural_readback_required",
		"acoustic_verification_requires_fresh_same_tap_post_fx_evidence",
	})
}

func (r *Runtime) startCapabilitySession(sessionID, conversationID, projectUUID, goal string, mode orchestration.InteractionMode, capabilityID, version string, constraints []string) (orchestration.PlanningSession, error) {
	if r == nil || r.Registry == nil || r.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime is not initialized")
	}
	definition, ok := r.Registry.Resolve(capabilityID, version)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("capability %s@%s is not registered", capabilityID, version)
	}
	session, err := orchestration.NewSession(sessionID, projectUUID, goal, orchestration.EngineV1, orchestration.CapabilityInvocation{
		CapabilityID:    definition.ID,
		CapabilityVer:   definition.Version,
		ConversationID:  conversationID,
		InteractionMode: mode,
		ProcessingPath:  orchestration.PathCapability,
		Goal:            goal,
		Constraints:     append([]string(nil), constraints...),
	})
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := r.Store.Create(session); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return session, nil
}

func (r *Runtime) ShadowB2(sessionID string, cut orchestration.ProjectCut, input capabilitycontext.StaticBalanceInput) (capabilityadapters.StaticBalancePlanResult, error) {
	if r == nil || r.Store == nil {
		return capabilityadapters.StaticBalancePlanResult{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return capabilityadapters.StaticBalancePlanResult{}, fmt.Errorf("session %s not found", sessionID)
	}
	return capabilityadapters.RunStaticBalanceShadow(session, cut, input)
}

func (r *Runtime) ShadowB3(sessionID string, cut orchestration.ProjectCut, input capabilitycontext.PanLayoutInput) (capabilityadapters.PanLayoutPlanResult, error) {
	if r == nil || r.Store == nil {
		return capabilityadapters.PanLayoutPlanResult{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return capabilityadapters.PanLayoutPlanResult{}, fmt.Errorf("session %s not found", sessionID)
	}
	return capabilityadapters.RunPanLayoutShadow(session, cut, input)
}

func (r *Runtime) ShadowB3FromVSP(sessionID string, state *kernel.VSPStateResult, request projectcut.BuildRequest, input capabilitycontext.PanLayoutInput) (capabilityadapters.PanLayoutPlanResult, orchestration.ProjectCut, error) {
	request.State = state
	cut, err := projectcut.Build(request)
	if err != nil {
		return capabilityadapters.PanLayoutPlanResult{}, orchestration.ProjectCut{}, err
	}
	planned, err := r.ShadowB3(sessionID, cut, input)
	if err != nil {
		return capabilityadapters.PanLayoutPlanResult{}, orchestration.ProjectCut{}, err
	}
	return planned, cut, nil
}

// ShadowB2FromVSP makes the current VSP snapshot lineage explicit. With the
// reference adapter this returns a bounded Cut; callers must not execute it
// until the Kernel provides an apply-point barrier.
func (r *Runtime) ShadowB2FromVSP(sessionID string, state *kernel.VSPStateResult, request projectcut.BuildRequest, input capabilitycontext.StaticBalanceInput) (capabilityadapters.StaticBalancePlanResult, orchestration.ProjectCut, error) {
	request.State = state
	cut, err := projectcut.Build(request)
	if err != nil {
		return capabilityadapters.StaticBalancePlanResult{}, orchestration.ProjectCut{}, err
	}
	planned, err := r.ShadowB2(sessionID, cut, input)
	if err != nil {
		return capabilityadapters.StaticBalancePlanResult{}, orchestration.ProjectCut{}, err
	}
	return planned, cut, nil
}

func (r *Runtime) ShadowB4(sessionID string, cut orchestration.ProjectCut, input capabilitycontext.LowEndRelationInput) (capabilityadapters.LowEndRelationPlanResult, error) {
	if r == nil || r.Store == nil {
		return capabilityadapters.LowEndRelationPlanResult{}, fmt.Errorf("runtime is not initialized")
	}
	if _, ok := r.Store.Load(sessionID); !ok {
		return capabilityadapters.LowEndRelationPlanResult{}, fmt.Errorf("session %s not found", sessionID)
	}
	return capabilityadapters.PlanLowEndRelation(capabilityadapters.LowEndRelationPlanRequest{
		SessionID:  sessionID,
		Goal:       input.UserIntent,
		Mode:       orchestration.InteractionInspect,
		ProjectCut: cut,
		Input:      input,
	})
}

func (r *Runtime) ShadowB4FromVSP(sessionID string, state *kernel.VSPStateResult, request projectcut.BuildRequest, input capabilitycontext.LowEndRelationInput) (capabilityadapters.LowEndRelationPlanResult, orchestration.ProjectCut, error) {
	request.State = state
	cut, err := projectcut.Build(request)
	if err != nil {
		return capabilityadapters.LowEndRelationPlanResult{}, orchestration.ProjectCut{}, err
	}
	planned, err := r.ShadowB4(sessionID, cut, input)
	if err != nil {
		return capabilityadapters.LowEndRelationPlanResult{}, orchestration.ProjectCut{}, err
	}
	return planned, cut, nil
}

func (r *Runtime) ShadowC1(sessionID string, cut orchestration.ProjectCut, input capabilitycontext.FrequencyCleanupInput) (capabilityadapters.FrequencyCleanupPlanResult, error) {
	if r == nil || r.Store == nil {
		return capabilityadapters.FrequencyCleanupPlanResult{}, fmt.Errorf("runtime is not initialized")
	}
	if _, ok := r.Store.Load(sessionID); !ok {
		return capabilityadapters.FrequencyCleanupPlanResult{}, fmt.Errorf("session %s not found", sessionID)
	}
	return capabilityadapters.PlanFrequencyCleanup(capabilityadapters.FrequencyCleanupPlanRequest{SessionID: sessionID, Goal: input.UserIntent, Mode: orchestration.InteractionInspect, ProjectCut: cut, Input: input})
}

func (r *Runtime) ShadowC1FromVSP(sessionID string, state *kernel.VSPStateResult, request projectcut.BuildRequest, input capabilitycontext.FrequencyCleanupInput) (capabilityadapters.FrequencyCleanupPlanResult, orchestration.ProjectCut, error) {
	request.State = state
	cut, err := projectcut.Build(request)
	if err != nil {
		return capabilityadapters.FrequencyCleanupPlanResult{}, orchestration.ProjectCut{}, err
	}
	planned, err := r.ShadowC1(sessionID, cut, input)
	if err != nil {
		return capabilityadapters.FrequencyCleanupPlanResult{}, orchestration.ProjectCut{}, err
	}
	return planned, cut, nil
}

func (r *Runtime) AttachProposal(sessionID string, proposal orchestration.Proposal) (orchestration.PlanningSession, error) {
	if r == nil || r.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s not found", sessionID)
	}
	expected := session.Revision
	updated, err := session.SetProposal(proposal)
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := r.Store.Save(updated, expected); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return updated, nil
}

func (r *Runtime) AttachFrozenPlan(sessionID string, plan orchestration.FrozenPlan) (orchestration.PlanningSession, error) {
	if r == nil || r.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s not found", sessionID)
	}
	expected := session.Revision
	updated, err := session.SetFrozenPlan(plan)
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := r.Store.Save(updated, expected); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return updated, nil
}

// AuthorizeProposal persists a semantic, version-bound authorization. It does
// not execute the ActionSet; Execution Coordinator will consume it later.
func (r *Runtime) AuthorizeProposal(sessionID string, authorization orchestration.Authorization) (orchestration.PlanningSession, error) {
	if r == nil || r.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s not found", sessionID)
	}
	expected := session.Revision
	updated, err := session.Authorize(authorization)
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := r.Store.Save(updated, expected); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return updated, nil
}

func (r *Runtime) CancelPlanningSession(sessionID string) (orchestration.PlanningSession, error) {
	if r == nil || r.Store == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s not found", sessionID)
	}
	if session.Terminal() {
		return session, nil
	}
	if session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying {
		return orchestration.PlanningSession{}, fmt.Errorf("session %s is %s; reconcile or coordinator cancellation is required", sessionID, session.Status)
	}
	expected := session.Revision
	updated, err := session.Transition(orchestration.StatusCancelled)
	if err != nil {
		return orchestration.PlanningSession{}, err
	}
	if err := r.Store.Save(updated, expected); err != nil {
		return orchestration.PlanningSession{}, err
	}
	return updated, nil
}

func (r *Runtime) ExecuteActionSet(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, currentCut orchestration.ProjectCut, port executionruntime.MutationPort, verifier executionruntime.Verifier) (orchestration.PlanningSession, error) {
	if r == nil || r.Executor == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime executor is not initialized")
	}
	return r.Executor.Execute(ctx, sessionID, actionSet, currentCut, port, verifier)
}

func (r *Runtime) ExecuteActionSetWithPersistence(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, currentCut orchestration.ProjectCut, port executionruntime.MutationPort, verifier executionruntime.Verifier, persistence executionruntime.PersistencePort) (orchestration.PlanningSession, error) {
	if r == nil || r.Executor == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime executor is not initialized")
	}
	return r.Executor.ExecuteWithPersistence(ctx, sessionID, actionSet, currentCut, port, verifier, persistence)
}

func (r *Runtime) ReconcileActionSet(ctx context.Context, sessionID string, actionSet orchestration.ActionSet, port executionruntime.ReconcilePort, verifier executionruntime.Verifier) (orchestration.PlanningSession, error) {
	if r == nil || r.Executor == nil {
		return orchestration.PlanningSession{}, fmt.Errorf("runtime executor is not initialized")
	}
	return r.Executor.Reconcile(ctx, sessionID, actionSet, port, verifier)
}

func (r *Runtime) HasDurableStore() bool {
	return r != nil && orchestration.IsDurableStore(r.Store)
}

// BuildB2ContextEnvelope admits every model-facing component through one
// bounded envelope. Callers may supply arbitrarily long turn history; only the
// configured recent window can enter the model context.
func (r *Runtime) BuildB2ContextEnvelope(sessionID string, bundle orchestration.ContextBundle, toolSchemas, turns []orchestration.ContextEntry, budget orchestration.ContextWindowBudget) (orchestration.ContextEnvelope, error) {
	return r.BuildCapabilityContextEnvelope(sessionID, bundle, toolSchemas, turns, budget)
}

func (r *Runtime) BuildCapabilityContextEnvelope(sessionID string, bundle orchestration.ContextBundle, toolSchemas, turns []orchestration.ContextEntry, budget orchestration.ContextWindowBudget) (orchestration.ContextEnvelope, error) {
	if r == nil || r.Store == nil || r.Registry == nil {
		return orchestration.ContextEnvelope{}, fmt.Errorf("runtime is not initialized")
	}
	session, ok := r.Store.Load(sessionID)
	if !ok {
		return orchestration.ContextEnvelope{}, fmt.Errorf("session %s not found", sessionID)
	}
	registryEntries := make([]orchestration.ContextEntry, 0)
	for _, definition := range r.Registry.List() {
		data, _ := json.Marshal(definition)
		priority := 10
		if definition.ID == session.Invocation.CapabilityID {
			priority = 100
		}
		registryEntries = append(registryEntries, orchestration.ContextEntry{
			ID: definition.ID + "@" + definition.Version, Content: string(data),
			Reference: "capability-registry:" + definition.ID + "@" + definition.Version, Priority: priority,
		})
	}
	envelope := orchestration.BuildContextEnvelope(orchestration.ContextEnvelopeRequest{
		System: capabilityRuntimeSystemContext, Registry: registryEntries, ToolSchemas: toolSchemas,
		Session: session, Bundle: bundle, ConversationTurns: turns, Budget: budget,
	})
	if !envelope.Valid {
		return envelope, fmt.Errorf("context envelope admission failed: %v", envelope.Blockers)
	}
	return envelope, nil
}
