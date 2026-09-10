package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

const taskSemanticContextKey = "task_semantic_state"
const taskContractContextKey = "task_contract"

func (s *Server) ensureAudioTaskContract(conversationID string, mode audioclosure.Mode, scope audioclosure.Scope, projectUUID, projectRevision string, requestContext map[string]any) (map[string]any, error) {
	if s == nil || s.harness == nil {
		// Isolated controller tests and domain adapters can run without the
		// product harness. The product chat path always supplies it and is
		// covered by contract/recovery integration tests.
		return requestContext, nil
	}
	goalID := firstStringFromMap(requestContext, "goal_id")
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task == nil || goal.GoalID == "" || goal.RunID == "" {
		if goalID == "" {
			return requestContext, nil
		}
		// Project activation can restore the workspace before the in-memory
		// runtime has rehydrated the task row. The route already carries the
		// durable goal/run identity; hydrate that exact identity instead of
		// inventing a replacement. If it still cannot be recovered, fail closed.
		goal = s.harness.EnsureGoal(goalID, firstStringFromMap(requestContext, "run_id"), firstStringFromMap(requestContext, "original_intent"))
		if goal.Task == nil || goal.GoalID == "" || goal.RunID == "" {
			return requestContext, fmt.Errorf("audio task contract requires durable Task/Goal/Run identity")
		}
	}
	kind := taskstate.ContractImprovement
	authorization := "governed_experiment"
	criteria := []string{
		"settle only after a governed experiment outcome",
		"or report no_candidate_found or a distinct capability boundary with evidence",
	}
	if mode == audioclosure.ModeDiagnostic {
		kind = taskstate.ContractDiagnostic
		authorization = "observe_and_propose_only"
		criteria = []string{
			"return an evidence-backed bounded diagnosis or no_candidate_found",
			"a bounded improvement proposal may settle without project mutation",
		}
	}
	contract := taskstate.Contract{
		ContractID: "contract_" + goal.Task.TaskID, ConversationID: conversationID,
		Kind: kind, Scope: taskstate.Scope{Kind: scope.Kind, ID: scope.ID, Label: scope.Label},
		Temporary: scope.Kind == "project", TargetDiscovery: "agent_observation",
		AuthorizationBoundary: authorization, CompletionCriteria: criteria,
		EvidenceRequirements: []string{
			"exact observation or audit receipt references",
			"project revision matching the evaluated evidence",
			"explicit limitations for every bounded conclusion",
		},
		ProjectUUID: projectUUID, ProjectRevision: projectRevision, CreatedAt: time.Now().UTC(),
	}
	goal, err := s.harness.EnsureTaskContract(goalID, contract)
	if err != nil {
		return requestContext, err
	}
	return bindTaskSemanticGoal(requestContext, goal.Task.Contract, goal.Task.SemanticState), nil
}

// bindTaskSemanticGoal avoids placing runtime internals or model reasoning in
// the prompt surface. Only the immutable contract and canonical projection are
// copied into the continuation checkpoint.
func bindTaskSemanticGoal(context map[string]any, goalTask *taskstate.Contract, state *taskstate.Snapshot) map[string]any {
	out := cloneContext(context)
	if out == nil {
		out = map[string]any{}
	}
	if goalTask != nil {
		out[taskContractContextKey] = structMap(*goalTask)
	}
	if state != nil {
		out[taskSemanticContextKey] = structMap(*state)
	}
	return out
}

func (s *Server) bindCurrentTaskSemantics(context map[string]any, goalID string) map[string]any {
	if s == nil || s.harness == nil {
		return context
	}
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task == nil {
		return context
	}
	return bindTaskSemanticGoal(context, goal.Task.Contract, goal.Task.SemanticState)
}

func (s *Server) hasTaskSemanticContract(goalID string) bool {
	if s == nil || s.harness == nil || strings.TrimSpace(goalID) == "" {
		return false
	}
	goal := s.harness.RuntimeStatus(goalID)
	return goal.Task != nil && goal.Task.Contract != nil && goal.Task.SemanticState != nil
}

func (s *Server) transitionTaskSemantic(goalID string, request taskstate.TransitionRequest) (taskstate.Snapshot, error) {
	if s == nil || s.harness == nil {
		return taskstate.Snapshot{}, fmt.Errorf("task runtime is unavailable")
	}
	goal, err := s.harness.TransitionTask(goalID, request)
	if err != nil {
		return taskstate.Snapshot{}, err
	}
	if goal.Task == nil || goal.Task.SemanticState == nil {
		return taskstate.Snapshot{}, fmt.Errorf("task semantic transition produced no state")
	}
	s.persistCurrentProjectWorkspace()
	return *goal.Task.SemanticState, nil
}

func (s *Server) applyFreeStateDecisionSemantic(loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision) error {
	if loop == nil || loop.GoalID == "" || s == nil || s.harness == nil {
		return nil
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil {
		return nil
	}
	contract, current := *goal.Task.Contract, *goal.Task.SemanticState
	status := strings.ToLower(strings.TrimSpace(decision.Status))
	evidence := freeStateDecisionEvidence(decision)
	summary := firstNonEmpty(decision.Summary, decision.StopReason)
	revision := firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), current.ProjectRevision)
	apply := func(request taskstate.TransitionRequest) error {
		request.ProjectRevision = firstNonEmpty(request.ProjectRevision, revision)
		next, err := s.transitionTaskSemantic(loop.GoalID, request)
		if err == nil {
			current = next
		}
		return err
	}
	switch status {
	case agentloop.FreeStateNeedsObservation, string(taskstate.StateObservationInProgress):
		return nil
	case agentloop.FreeStateDiagnosticComplete, agentloop.FreeStateSatisfied:
		if contract.Kind == taskstate.ContractImprovement && status == agentloop.FreeStateSatisfied {
			return fmt.Errorf("satisfied cannot settle an open improvement contract")
		}
		if current.State == taskstate.StateObservationInProgress {
			return apply(taskstate.TransitionRequest{Event: taskstate.EventDiagnosticCompleted, Reason: "runtime validated diagnostic evidence", Summary: summary, EvidenceRefs: evidence})
		}
		return nil
	case agentloop.FreeStateNoCandidateFound:
		return apply(taskstate.TransitionRequest{Event: taskstate.EventNoCandidateReported, Reason: "runtime validated bounded candidate search", Summary: summary, EvidenceRefs: evidence})
	case agentloop.FreeStateNeedsAction, agentloop.FreeStateNeedsExperiment, agentloop.FreeStateImprovementProposal:
		if decision.ImprovementProposal == nil {
			return fmt.Errorf("improvement proposal is missing")
		}
		if current.State == taskstate.StateNeedsExperiment && current.Proposal != nil {
			return nil
		}
		if current.State == taskstate.StateObservationInProgress {
			if err := apply(taskstate.TransitionRequest{Event: taskstate.EventDiagnosticCompleted, Reason: "candidate diagnosis established", Summary: summary, EvidenceRefs: evidence}); err != nil {
				return err
			}
		}
		proposal := canonicalProposal(loop, decision)
		return apply(taskstate.TransitionRequest{Event: taskstate.EventImprovementProposed, Reason: "runtime admitted bounded improvement proposal", Summary: summary, EvidenceRefs: evidence, CandidateID: canonicalCandidateID(decision), Proposal: &proposal})
	case agentloop.FreeStateCapabilityBlocked, agentloop.FreeStateBlocked:
		return apply(taskstate.TransitionRequest{Event: taskstate.EventCapabilityBlocked, Reason: firstNonEmpty(decision.StopReason, "free-state capability boundary"), Summary: summary, EvidenceRefs: evidence})
	default:
		return fmt.Errorf("free-state status %q has no canonical transition", decision.Status)
	}
}

func (s *Server) settleDiagnosticTask(loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision) error {
	if loop == nil {
		return nil
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil || goal.Task.Contract.Kind != taskstate.ContractDiagnostic {
		return nil
	}
	if goal.Task.SemanticState.State != taskstate.StateDiagnosticComplete && goal.Task.SemanticState.State != taskstate.StateImprovementProposal {
		return nil
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventTaskSettled, Reason: "diagnostic-only contract fulfilled",
		Summary: decision.Summary, EvidenceRefs: freeStateDecisionEvidence(decision),
		ProjectRevision: goal.Task.SemanticState.ProjectRevision,
	})
	return err
}

func (s *Server) bindExperimentSemantic(loop *freeStateReasoningLoop) error {
	if loop == nil || loop.Experiment == nil {
		return fmt.Errorf("experiment identity is unavailable")
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	if goal.Task == nil || goal.Task.SemanticState == nil {
		return nil
	}
	current := goal.Task.SemanticState
	if current.State == taskstate.StateNeedsExperiment && current.ExperimentID == loop.Experiment.ID {
		return nil
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventExperimentRequired, Reason: "free-state experiment runtime admitted",
		Summary: "bounded experiment admitted", ExperimentID: loop.Experiment.ID,
		ProjectRevision: firstNonEmpty(loop.Experiment.ProjectRevision, current.ProjectRevision),
	})
	if err == nil {
		goal := s.harness.RuntimeStatus(loop.GoalID)
		if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil {
			return fmt.Errorf("task semantic state disappeared before experiment settlement")
		}
		err = loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
	}
	return err
}

// updateTaskExperimentPendingInteraction keeps confirmation ownership in the
// canonical Task state while the experiment remains in needs_experiment.
func (s *Server) updateTaskExperimentPendingInteraction(conversationID, goalID, interactionID, kind, reason string) error {
	if !s.hasTaskSemanticContract(goalID) {
		return nil
	}
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task == nil || goal.Task.SemanticState == nil || goal.Task.SemanticState.State != taskstate.StateNeedsExperiment {
		return nil
	}
	current := goal.Task.SemanticState
	request := taskstate.TransitionRequest{
		Event: taskstate.EventExperimentRequired, Reason: reason, Summary: reason,
		ExperimentID: current.ExperimentID, ProjectRevision: current.ProjectRevision,
	}
	if strings.TrimSpace(interactionID) != "" {
		request.PendingInteraction = &taskstate.PendingInteraction{InteractionID: interactionID, Kind: kind, Reason: reason}
	}
	next, err := s.transitionTaskSemantic(goalID, request)
	if err != nil {
		return err
	}
	if loop, ok := s.freeStateLoop(conversationID); ok && loop.Experiment != nil {
		if goal.Task.Contract == nil {
			return fmt.Errorf("task contract disappeared while binding pending interaction")
		}
		if err := loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, next.State, next.Revision); err != nil {
			return err
		}
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
	}
	return nil
}

func (s *Server) requireTaskHumanJudgment(loop *freeStateReasoningLoop, interactionID, reason string) error {
	if loop == nil || loop.Experiment == nil || !s.hasTaskSemanticContract(loop.GoalID) {
		return nil
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	// audition.ready reaches requestAuditionJudgment twice (prepare path and
	// kernel telemetry path) inside the judgment request's write window. The
	// transition table admits human_judgment_requested only from
	// needs_experiment/improvement_proposal, so the latecomer used to die as a
	// benign WARN. A task already parked at this experiment's judgment
	// boundary is the success shape: rebind the projection and return.
	if goal.Task != nil && goal.Task.SemanticState != nil &&
		goal.Task.SemanticState.State == taskstate.StateHumanJudgmentRequired &&
		goal.Task.SemanticState.ExperimentID == loop.Experiment.ID {
		return loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventHumanJudgmentRequested, Reason: reason, Summary: reason,
		ExperimentID: loop.Experiment.ID, ProjectRevision: loop.Experiment.ProjectRevision,
		PendingInteraction: &taskstate.PendingInteraction{InteractionID: interactionID, Kind: "audition_judgment", Reason: reason},
	})
	if err != nil {
		return err
	}
	goal = s.harness.RuntimeStatus(loop.GoalID)
	return loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
}

func (s *Server) resumeTaskExperimentAfterJudgment(loop *freeStateReasoningLoop, reason string) error {
	if loop == nil || loop.Experiment == nil || !s.hasTaskSemanticContract(loop.GoalID) {
		return nil
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventExperimentRequired, Reason: reason, Summary: reason,
		ExperimentID: loop.Experiment.ID, ProjectRevision: loop.Experiment.ProjectRevision,
	})
	if err != nil {
		return err
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	return loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
}

func (s *Server) settleTaskFromExperiment(loop *freeStateReasoningLoop, summary string, evidence []string) error {
	if loop == nil || loop.Experiment == nil {
		return fmt.Errorf("experiment identity is unavailable")
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventTaskSettled, Reason: "experiment runtime produced a governed outcome",
		Summary: summary, EvidenceRefs: evidence, ExperimentID: loop.Experiment.ID,
		ProjectRevision: loop.Experiment.ProjectRevision,
	})
	if err != nil {
		return err
	}
	// Turn.Settle validates the canonical task state, so the experiment's
	// projection must be rebound to the revision the settlement just produced.
	goal := s.harness.RuntimeStatus(loop.GoalID)
	if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil {
		return fmt.Errorf("task semantic state disappeared before experiment settlement")
	}
	return loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
}

func (s *Server) completeTaskExperimentOutcome(loop *freeStateReasoningLoop, outcome experiment.SettlementOutcome, summary string) error {
	if loop == nil || loop.Experiment == nil || !s.hasTaskSemanticContract(loop.GoalID) {
		return nil
	}
	evidence := freeStateExperimentEvidence(loop.Experiment)
	event := taskstate.EventTaskSettled
	switch outcome {
	case experiment.OutcomeNeedsJudgment:
		return fmt.Errorf("needs_user_judgment must use the non-terminal human judgment transition")
	case experiment.OutcomeBlockedCapability, experiment.OutcomeBlockedObservation, experiment.OutcomeBudgetExhausted:
		event = taskstate.EventCapabilityBlocked
	case experiment.OutcomeUnsafe:
		event = taskstate.EventTaskFailed
	case experiment.OutcomeStopped:
		event = taskstate.EventTaskCancelled
	}
	_, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: event, Reason: "experiment outcome: " + string(outcome), Summary: summary,
		EvidenceRefs: evidence, ExperimentID: loop.Experiment.ID, ProjectRevision: loop.Experiment.ProjectRevision,
	})
	if err != nil {
		return err
	}
	goal := s.harness.RuntimeStatus(loop.GoalID)
	return loop.Experiment.BindTaskState(goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision)
}

func freeStateExperimentEvidence(turn *experiment.Turn) []string {
	if turn == nil {
		return nil
	}
	out := append([]string(nil), turn.Admission.EvidenceRefs...)
	for _, round := range turn.Rounds {
		for _, observation := range round.Observations {
			out = append(out, observation.EvidenceRefs...)
		}
		for _, intervention := range round.Interventions {
			out = append(out, intervention.EvidenceRefs...)
		}
		if round.Materiality != nil {
			out = append(out, round.Materiality.EvidenceRefs...)
		}
		if round.TargetResponse != nil {
			out = append(out, round.TargetResponse.EvidenceRefs...)
		}
		for _, judgment := range round.UserJudgmentEvidence {
			out = append(out, judgment.ID)
			out = append(out, judgment.AnalyticalEvidenceRefs...)
		}
	}
	return freeStateNormalizedViewIDs(out)
}

func freeStateDecisionEvidence(decision agentloop.FreeStateDecision) []string {
	out := []string{decision.ObservationID}
	if decision.ImprovementProposal != nil {
		out = append(out, decision.ImprovementProposal.EvidenceRefs...)
	}
	if decision.ExperimentMateriality != nil {
		out = append(out, decision.ExperimentMateriality.EvidenceRefs...)
	}
	if decision.ExperimentTargetResponse != nil {
		out = append(out, decision.ExperimentTargetResponse.EvidenceRefs...)
	}
	if decision.SemanticProcessorIntent != nil {
		out = append(out, decision.SemanticProcessorIntent.EvidenceRefs...)
	}
	if decision.Diagnostic != nil {
		for _, finding := range decision.Diagnostic.Findings {
			out = append(out, finding.EvidenceRefs...)
		}
	}
	return freeStateNormalizedViewIDs(out)
}

func canonicalProposal(loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision) taskstate.BoundedProposal {
	proposal := decision.ImprovementProposal
	seed := strings.Join([]string{loop.GoalID, proposal.ImprovementIntent, proposal.Hypothesis, strings.Join(proposal.EvidenceRefs, "|")}, "\x1f")
	digest := sha256.Sum256([]byte(seed))
	return taskstate.BoundedProposal{
		ProposalID:         "proposal_" + hex.EncodeToString(digest[:10]),
		Summary:            firstNonEmpty(proposal.ImprovementIntent, decision.Summary),
		EvidenceRefs:       append([]string(nil), proposal.EvidenceRefs...),
		Bounds:             append(append([]string(nil), proposal.Limitations...), proposal.NeedsResolution...),
		RequiresExperiment: true,
	}
}

func canonicalCandidateID(decision agentloop.FreeStateDecision) string {
	if decision.ImprovementProposal == nil {
		return ""
	}
	return firstStringFromMap(decision.ImprovementProposal.Target, "id", "target_id", "track_id")
}

func structMap(value any) map[string]any {
	raw, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// reconcileRestoredTaskSemanticProjections verifies that domain projections
// still belong to the restored canonical Task. It may advance a stale
// projection to a newer canonical revision, but it never reconstructs task
// meaning from a closure, experiment, prompt, or continuation.
func (s *Server) reconcileRestoredTaskSemanticProjectionsLocked() {
	if s == nil || s.harness == nil {
		return
	}
	failClosed := func(conversationID, goalID, reason string) {
		delete(s.goalContinuations, goalID)
		if loop, ok := s.freeStateLoops[conversationID]; ok {
			loop.Status = "blocked"
			loop.LastError = "semantic recovery validation required: " + reason
			loop.UpdatedAt = time.Now().UTC()
			s.freeStateLoops[conversationID] = loop
		}
		now := time.Now().UTC()
		for continuationID, item := range s.durableContinuations {
			if item.GoalID != goalID || item.Status == ContinuationCompleted || item.Status == ContinuationCancelled || item.Status == ContinuationFailed {
				continue
			}
			item.Status = ContinuationWaitingInteraction
			item.LeaseOwner = ""
			item.LeaseExpiresAt = time.Time{}
			item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": reason}
			item.UpdatedAt = now
			s.durableContinuations[continuationID] = cloneDurableContinuation(item)
		}
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingClarification, fmt.Errorf("semantic recovery validation required: %s", reason))
	}
	for conversationID, loop := range s.freeStateLoops {
		if loop.Experiment == nil || loop.GoalID == "" {
			continue
		}
		goal := s.harness.RuntimeStatus(loop.GoalID)
		if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil {
			// Legacy pre-E experiments remain readable and use their existing
			// compatibility lifecycle. Canonical product experiments never do.
			continue
		}
		contract, semantic := goal.Task.Contract, goal.Task.SemanticState
		experimentState := loop.Experiment
		if err := experimentState.Validate(); err != nil {
			failClosed(conversationID, loop.GoalID, "experiment state is invalid: "+err.Error())
			continue
		}
		if loop.RunID != goal.RunID || loop.OriginalIntent != goal.Task.OriginalIntent || experimentState.ConversationID != contract.ConversationID ||
			experimentState.GoalID != goal.GoalID || experimentState.RunID != goal.RunID || experimentState.OriginalIntent != goal.Task.OriginalIntent ||
			experimentState.ContractID != contract.ContractID || semantic.ExperimentID != experimentState.ID {
			failClosed(conversationID, loop.GoalID, "experiment identity does not match canonical Task/Run/contract")
			continue
		}
		// The canonical Task projection is authoritative across interaction
		// boundaries. A routed confirmation may durably advance the experiment
		// snapshot before the Task snapshot is persisted; when the semantic state
		// itself is identical, rebase that ahead-of-canonical revision instead of
		// treating a recoverable ordering race as a conflicting experiment.
		if experimentState.TaskStateRevision > semantic.Revision && experimentState.TaskState == semantic.State {
			experimentState.TaskStateRevision = semantic.Revision
		}
		if err := experimentState.BindTaskState(contract.ContractID, semantic.State, semantic.Revision); err != nil {
			// The canonical Task/semantic snapshot is the authority. If the
			// experiment has the same validated identity but an interaction-boundary
			// state/revision overlay, rebase its projection to the canonical state;
			// do not let a stale continuation strand an otherwise admitted action.
			experimentState.TaskState = semantic.State
			experimentState.TaskStateRevision = semantic.Revision
			if retryErr := experimentState.BindTaskState(contract.ContractID, semantic.State, semantic.Revision); retryErr != nil {
				failClosed(conversationID, loop.GoalID, "experiment task projection is conflicting: "+retryErr.Error())
				continue
			}
		}
		loop.Experiment = experimentState
		s.freeStateLoops[conversationID] = loop
	}
	if s.audioClosures == nil {
		return
	}
	for _, closure := range s.audioClosures.Snapshot() {
		if closure.ContractID == "" {
			continue
		}
		goal := s.harness.RuntimeStatus(closure.GoalID)
		conversationID := closure.ConversationID
		if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil || closure.TaskID != goal.Task.TaskID || closure.RunID != goal.RunID ||
			closure.OriginalIntent != goal.Task.OriginalIntent || closure.ContractID != goal.Task.Contract.ContractID || conversationID != goal.Task.Contract.ConversationID {
			failClosed(conversationID, closure.GoalID, "audio closure identity does not match canonical Task/Run/contract")
			continue
		}
		// A terminal closure is already the authoritative folded result for its
		// bounded lifecycle. Re-projecting it during recovery would append an
		// event to a settled event stream and incorrectly turn a valid restart
		// into a recovery conflict. Only verify identity above; non-terminal
		// closures still need projection to the canonical task revision.
		if closure.Terminal() {
			continue
		}
		originalRevision := closure.Revision
		semantic := goal.Task.SemanticState
		var err error
		if semantic.ProjectRevision != "" && semantic.ProjectRevision != closure.ProjectRevision {
			closure, _, err = (audioclosure.Driver{}).RevalidateProjectRevision(closure, closure.Revision, semantic.ProjectRevision, time.Now().UTC())
		}
		if err == nil {
			closure, _, err = (audioclosure.Driver{}).ProjectTaskState(closure, closure.Revision, closure.ContractID, semantic.State, semantic.Revision, time.Now().UTC())
		}
		if err != nil {
			failClosed(conversationID, closure.GoalID, "audio closure task projection is conflicting: "+err.Error())
			continue
		}
		if semantic.State.Terminal() && !closure.Terminal() {
			closure = audioClosureSettleFromResult(audioclosure.Driver{}, closure, agentloop.Result{Reply: firstNonEmpty(semantic.Summary, semantic.TransitionReason)})
		}
		if closure.Revision != originalRevision {
			if err := s.audioClosures.Save(closure, originalRevision); err != nil {
				failClosed(conversationID, closure.GoalID, "audio closure recovery save failed: "+err.Error())
				continue
			}
		}
	}
}
