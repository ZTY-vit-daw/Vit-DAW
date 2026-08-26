package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/trajectory"
)

// freeStateExperimentAdmission converts the existing model-owned proposal to
// the runtime admission without granting mutation authority. Optional legacy
// proposal bounds are kept compatible while the runtime records that they are
// proposal-derived until a governed action supplies the real checkpoint.
func freeStateExperimentAdmission(loop freeStateReasoningLoop, proposal *agentprotocol.ImprovementProposal) (experiment.Admission, error) {
	if proposal == nil {
		return experiment.Admission{}, fmt.Errorf("improvement proposal is required")
	}
	// The D2-1 domain table is the production admission boundary: track_gain
	// keeps its original shape and static_eq is admitted with equally tight
	// per-domain bounds. Domain+kind must match one table row (D1S1DomainSpecFor)
	// so a hybrid action cannot borrow a domain's bounds.
	if _, ok := experiment.D1S1DomainSpecFor(experiment.Admission{TypedAction: map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind}}); !ok {
		return experiment.Admission{}, fmt.Errorf("D1-S1 production entry admits only the D2-1 domain table domains with matching action_kind (%s)", strings.Join(experiment.D1S1AdmittedDomains(), ", "))
	}
	bounds := cloneContext(proposal.ParameterBounds)
	if len(bounds) == 0 {
		bounds = map[string]any{"source": "proposal", "mode": "bounded"}
	}
	verification := cloneContext(proposal.VerificationPlan)
	if len(verification) == 0 {
		verification = map[string]any{"evidence_refs": append([]string(nil), proposal.EvidenceRefs...)}
	}
	checkpoint := firstStringFromMap(loop.LatestProjectChange, "checkpoint_ref", "commit_id", "project_revision")
	if checkpoint == "" {
		checkpoint = "pending:" + firstNonEmpty(loop.LoopID, "free-state-experiment")
	}
	budget := 1
	deltaDB, _ := treatmentNumber(proposal.ParameterBounds, "delta_db", "db_delta", "gain_delta_db")
	typedAction := map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind, "target_db": proposal.ParameterBounds["target_db"], "delta_db": deltaDB}
	diagnosticBounds := map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "delta_db": deltaDB, "max_action_attempts": 1}
	retainedBounds := map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "delta_db": deltaDB, "max_action_attempts": 1}
	if strings.EqualFold(strings.TrimSpace(proposal.ActionDomain), d1StaticEQDomain) {
		// D2-1 static_eq: the typed action carries the bounded band parameters
		// from the proposal (gain_db is the moved parameter; frequency/q/band
		// are pinned from the domain table), and both dose scopes use gain_db
		// instead of delta_db. Nothing here relaxes the shared D1-S1 bounds.
		gainDB, _ := treatmentNumber(proposal.ParameterBounds, "gain_db")
		typedAction = map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind, "gain_db": gainDB}
		for _, key := range []string{"frequency_hz", "q", "band_index", "plugin_identifier"} {
			if value, exists := proposal.ParameterBounds[key]; exists {
				typedAction[key] = value
			}
		}
		diagnosticBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "gain_db": gainDB, "max_action_attempts": 1}
		retainedBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "gain_db": gainDB, "max_action_attempts": 1}
	}
	admission := experiment.Admission{
		SchemaVersion:        experiment.SchemaVersion,
		TargetRef:            cloneContext(proposal.Target),
		EvidenceRefs:         append([]string(nil), proposal.EvidenceRefs...),
		Hypothesis:           proposal.Hypothesis,
		TypedAction:          typedAction,
		DiagnosticDoseBounds: diagnosticBounds,
		RetainedDoseBounds:   retainedBounds,
		ExperimentBudget:     budget,
		ExpectedEffect:       proposal.ExpectedEffect,
		ProtectedDimensions:  freeStateStringSlice(verification["protected_dimensions"]),
		VerificationPlan:     verification,
		CheckpointRef:        checkpoint,
		RollbackPlan:         map[string]any{"kind": "agent_rollback_action", "source": "existing_governed_rollback"},
		AuthorityMode:        map[bool]experiment.AuthorityMode{true: experiment.AuthorityFull, false: experiment.AuthorityOrdinary}[loop.AuthorityMode == experiment.AuthorityFull],
	}
	if err := admission.ValidateD1S1(); err != nil {
		return experiment.Admission{}, err
	}
	if _, err := validateD1FreshObservedTarget(loop, admission); err != nil {
		return experiment.Admission{}, err
	}
	return admission, nil
}

func validateD1FreshObservedTarget(loop freeStateReasoningLoop, admission experiment.Admission) (experiment.Observation, error) {
	observation, ok := freeStateExperimentObservation(loop.LatestObservation, false)
	if !ok || loop.LatestObservation == nil {
		return experiment.Observation{}, fmt.Errorf("D1-S1 target requires a usable CCB observation")
	}
	summary := cloneContext(loop.LatestObservation.Summary)
	if bundle := firstMapFromAny(summary["bundle"]); len(bundle) > 0 {
		for key, value := range bundle {
			if _, exists := summary[key]; !exists {
				summary[key] = value
			}
		}
	}
	audit := firstMapFromAny(summary["audit_receipt"])
	freshnessMap := firstMapFromAny(audit["freshness"])
	if len(freshnessMap) == 0 {
		freshnessMap = firstMapFromAny(summary["freshness"])
	}
	// CCB status=ready describes observation availability. Freshness is carried
	// by class (current_observation/current_snapshot/fresh); do not reject a
	// valid current observation merely because its readiness status is ready.
	freshness := firstNonEmpty(
		firstStringFromMap(freshnessMap, "class"),
		firstStringFromMap(freshnessMap, "status"),
	)
	if !freeStateFreshObservationClass(freshness) || !observation.Fresh {
		return experiment.Observation{}, fmt.Errorf("D1-S1 target requires an explicitly fresh CCB observation")
	}
	if strings.TrimSpace(observation.ProjectRevision) == "" {
		return experiment.Observation{}, fmt.Errorf("D1-S1 target observation must be revision-bound")
	}
	target := firstMapFromAny(summary["target_ref"])
	wantKind := strings.ToLower(firstStringFromMap(admission.TargetRef, "kind"))
	wantID := firstStringFromMap(admission.TargetRef, "id", "track_id")
	if strings.ToLower(firstStringFromMap(target, "kind", "target_kind")) != wantKind || firstStringFromMap(target, "id", "target_id", "track_id") != wantID {
		return experiment.Observation{}, fmt.Errorf("D1-S1 target must match the fresh observation target")
	}
	// The proposal must cite the observation itself, not only its receipt or a
	// secondary evidence ref.  Receipt IDs remain useful audit metadata, but
	// they do not establish the model's target-observation binding required by
	// the FS6 -> FS7 handoff.
	if !containsStringFold(admission.EvidenceRefs, observation.ID) {
		return experiment.Observation{}, fmt.Errorf("D1-S1 evidence_refs must include the target observation ID %q", observation.ID)
	}
	return observation, nil
}

func freeStateFreshObservationClass(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fresh", "current", "current_snapshot", "current_observation":
		return true
	default:
		return false
	}
}

func containsStringFold(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func stringSlicesIntersect(left, right []string) bool {
	seen := map[string]bool{}
	for _, value := range left {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	for _, value := range right {
		if seen[strings.TrimSpace(value)] {
			return true
		}
	}
	return false
}

func freeStateExperimentViews(loop freeStateReasoningLoop, decision agentloop.FreeStateDecision, proposal *agentprotocol.ImprovementProposal) []string {
	views := append([]string(nil), decision.RequestedViewIDs...)
	if len(views) == 0 && loop.LatestObservation != nil {
		views = freeStateStringSlice(loop.LatestObservation.Summary["requested_views"])
		if len(views) == 0 {
			views = freeStateStringSlice(loop.LatestObservation.Summary["actual_executed_view_ids"])
		}
	}
	if len(views) == 0 && proposal != nil {
		plan := firstMapFromAny(proposal.VerificationPlan)
		views = freeStateStringSlice(plan["view_ids"])
	}
	return freeStateNormalizedViewIDs(views)
}

func freeStateExperimentObservation(observation *agentloop.RecentObservation, postAction bool) (experiment.Observation, bool) {
	if !freeStateIsCCBObservation(observation) || !freeStateUsableObservation(observation) {
		return experiment.Observation{}, false
	}
	summary := cloneContext(observation.Summary)
	bundle := firstMapFromAny(summary["bundle"])
	if len(bundle) > 0 {
		for key, value := range bundle {
			if _, exists := summary[key]; !exists {
				summary[key] = value
			}
		}
	}
	receipt := firstMapFromAny(summary["audit_receipt"])
	requested := freeStateStringSlice(summary["requested_views"])
	if len(requested) == 0 {
		requested = freeStateStringSlice(bundle["requested_views"])
	}
	executed := freeStateStringSlice(summary["actual_executed_view_ids"])
	if len(executed) == 0 {
		executed = freeStateStringSlice(receipt["actual_executed_view_ids"])
	}
	if len(executed) == 0 {
		executed = freeStateStringSlice(bundle["actual_executed_view_ids"])
	}
	freshness := firstMapFromAny(receipt["freshness"])
	fresh := !strings.EqualFold(firstStringFromMap(freshness, "status"), "stale") &&
		!strings.EqualFold(firstStringFromMap(summary, "freshness"), "stale")
	evidence := freeStateStringSlice(summary["evidence_refs"])
	if len(evidence) == 0 {
		evidence = freeStateStringSlice(bundle["evidence_refs"])
	}
	if len(evidence) == 0 {
		evidence = []string{firstNonEmpty(firstStringFromMap(summary, "observation_id"), observation.ToolCallID)}
	}
	row := experiment.Observation{
		ID:        firstNonEmpty(firstStringFromMap(summary, "observation_id", "receipt_id"), observation.ToolCallID),
		ReceiptID: firstStringFromMap(receipt, "receipt_id"), RequestedViewIDs: requested, ExecutedViewIDs: executed,
		ViewSetMatches: boolValue(receipt["view_set_matches"]) || (receipt["view_set_matches"] == nil && len(requested) > 0 && sameStringSet(requested, executed)),
		Fresh:          fresh, PostAction: postAction, ProjectRevision: firstNonEmpty(firstStringFromMap(receipt, "project_revision"),
			firstStringFromMap(firstMapFromAny(receipt["project_binding"]), "project_revision"), freeStateObservationProjectRevision(summary)),
		EvidenceRefs: evidence, Limitations: freeStateStringSlice(summary["limitations"]), Summary: summary, RecordedAt: time.Now().UTC(),
	}
	return row, row.ID != ""
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, value := range a {
		found := false
		for _, candidate := range b {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func freeStateExperimentHasObservation(turn *experiment.Turn, observationID string) bool {
	if turn == nil || strings.TrimSpace(observationID) == "" {
		return false
	}
	for _, round := range turn.Rounds {
		for _, observation := range round.Observations {
			if observation.ID == observationID {
				return true
			}
		}
	}
	return false
}

func (s *Server) emitFreeStateExperimentEvents(events []trajectory.Event) {
	for _, event := range events {
		if _, err := s.emitTrajectoryEvent(event.ConversationID, event); err != nil && s.logger != nil {
			s.logger.Warn("[free-state-experiment] trajectory event rejected type=%s error=%v", event.Type, err)
		}
	}
}

func (s *Server) ensureFreeStateExperimentCheckpoint(ctx context.Context, loop *freeStateReasoningLoop, goalID, runID string) string {
	if loop == nil {
		return ""
	}
	if checkpoint := firstStringFromMap(loop.LatestProjectChange, "checkpoint_ref", "commit_id"); checkpoint != "" {
		return checkpoint
	}
	if s == nil || s.harness == nil {
		return ""
	}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Command: map[string]any{"cmd": "version_checkpoint", "message": "free-state experiment baseline", "source": "free_state_experiment", "checkpoint_kind": "manual"},
		Source:  "free_state_experiment", Confirmed: true, GoalID: goalID, RunID: runID,
	})
	if err != nil || strings.EqualFold(response.Status, "error") {
		return ""
	}
	return firstStringFromMap(response.ProjectHistory, "commit_id", "checkpoint_ref")
}

func (s *Server) startFreeStateExperiment(loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision, goalID, runID string) error {
	if loop == nil || decision.ImprovementProposal == nil {
		return fmt.Errorf("experiment proposal is missing")
	}
	var admission experiment.Admission
	var err error
	if decision.ExperimentAdmission != nil {
		admission = *decision.ExperimentAdmission
		err = admission.ValidateD1S1()
		if err == nil {
			_, err = validateD1FreshObservedTarget(*loop, admission)
		}
	} else {
		admission, err = freeStateExperimentAdmission(*loop, decision.ImprovementProposal)
	}
	if err != nil {
		return err
	}
	if loop.AuthorityMode == "" {
		loop.AuthorityMode = experiment.AuthorityOrdinary
	}
	admission.AuthorityMode = loop.AuthorityMode
	if checkpoint := s.ensureFreeStateExperimentCheckpoint(context.Background(), loop, goalID, runID); checkpoint != "" {
		admission.CheckpointRef = checkpoint
	}
	if err := admission.ValidateD1S1(); err != nil {
		return err
	}
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: loop.ConversationID, GoalID: goalID, RunID: runID, TurnID: "turn:" + loop.LoopID}, loop.OriginalIntent, admission, time.Now().UTC())
	if err != nil {
		return err
	}
	loop.Experiment = &turn
	if err := s.bindExperimentSemantic(loop); err != nil {
		loop.Experiment = nil
		return err
	}
	beforeObservation, err := validateD1FreshObservedTarget(*loop, admission)
	if err != nil {
		return err
	}
	events := turn.StartEvents(time.Now().UTC())
	views := freeStateExperimentViews(*loop, decision, decision.ImprovementProposal)
	if len(views) == 0 {
		return fmt.Errorf("D1-S1 requires the model-requested CCB view set")
	}
	roundEvents, roundErr := turn.StartRound(views, admission.CheckpointRef, beforeObservation.ProjectRevision, time.Now().UTC())
	if roundErr != nil {
		return roundErr
	}
	events = append(events, roundEvents...)
	s.emitFreeStateExperimentEvents(events)
	observationEvents, observationErr := loop.Experiment.RecordObservation(beforeObservation, false, time.Now().UTC())
	if observationErr != nil {
		return observationErr
	}
	s.emitFreeStateExperimentEvents(observationEvents)
	return nil
}

func (s *Server) recordFreeStateExperimentDecision(ctx context.Context, loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision) {
	if loop == nil || loop.Experiment == nil {
		return
	}
	if decision.ExperimentMateriality != nil {
		if events, err := loop.Experiment.EvaluateMateriality(*decision.ExperimentMateriality, time.Now().UTC()); err == nil {
			s.emitFreeStateExperimentEvents(events)
			if decision.ExperimentMateriality.Evaluation == trajectory.EvaluationInsufficientDose && !loop.Experiment.Admission.IsD1S1() {
				if decisionEvents, decisionErr := loop.Experiment.DecideRound(experiment.DecisionNextRound, "insufficient dose; calibrate in next round", time.Now().UTC()); decisionErr == nil {
					s.emitFreeStateExperimentEvents(decisionEvents)
					views := freeStateExperimentViews(*loop, decision, decision.ImprovementProposal)
					if len(views) == 0 {
						views = append([]string(nil), loop.Experiment.Rounds[len(loop.Experiment.Rounds)-1].RequestedViewIDs...)
					}
					if roundEvents, roundErr := loop.Experiment.StartRound(views, loop.Experiment.Admission.CheckpointRef, firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), time.Now().UTC()); roundErr == nil {
						s.emitFreeStateExperimentEvents(roundEvents)
					}
				}
			}
		} else if s.logger != nil {
			s.logger.Warn("[free-state-experiment] materiality rejected: %v", err)
		}
	}
	if decision.ExperimentTargetResponse != nil {
		if events, err := loop.Experiment.RecordTargetResponse(*decision.ExperimentTargetResponse, time.Now().UTC()); err == nil {
			s.emitFreeStateExperimentEvents(events)
			if decision.ExperimentTargetResponse.Outcome == trajectory.EvaluationHumanAuditionReady {
				if auditionErr := s.prepareFreeStateAudition(ctx, loop); auditionErr != nil && s.logger != nil {
					s.logger.Warn("[free-state-experiment] audition prepare failed: %v", auditionErr)
				}
			}
		} else if s.logger != nil {
			s.logger.Warn("[free-state-experiment] target response rejected: %v", err)
		}
	}
	if decision.ExperimentRoundDecision != "" {
		roundDecision := experiment.RoundDecision(decision.ExperimentRoundDecision)
		if events, err := loop.Experiment.DecideRound(roundDecision, decision.Summary, time.Now().UTC()); err == nil {
			s.emitFreeStateExperimentEvents(events)
			if roundDecision == experiment.DecisionRollback {
				if events, rollbackErr := s.rollbackFreeStateExperiment(ctx, loop); rollbackErr == nil {
					s.emitFreeStateExperimentEvents(events)
				} else if s.logger != nil {
					s.logger.Warn("[free-state-experiment] rollback failed: %v", rollbackErr)
				}
			}
		} else if s.logger != nil {
			s.logger.Warn("[free-state-experiment] round decision rejected: %v", err)
		}
	}
}

func (s *Server) recordFreeStateExperimentAction(loop *freeStateReasoningLoop, processorType, status string, receipt map[string]any) {
	if loop == nil || loop.Experiment == nil || len(loop.Experiment.Rounds) == 0 {
		return
	}
	round := loop.Experiment.Rounds[len(loop.Experiment.Rounds)-1]
	actionID := firstStringFromMap(receipt, "agent_action_id", "action_id", "receipt_id")
	if actionID == "" {
		actionID = fmt.Sprintf("free-state-action-%d", loop.Cycle)
	}
	for _, intervention := range round.Interventions {
		if intervention.ID == actionID {
			return
		}
	}
	if loop.Experiment.Admission.IsD1S1() && len(round.Interventions) > 0 {
		return
	}
	attempt := len(round.Interventions) + 1
	technical := experiment.TechnicalApplied
	if status != "applied" {
		technical = experiment.TechnicalFailed
	}
	intervention := experiment.Intervention{ID: actionID, Attempt: attempt, TechnicalApplication: technical,
		UserConfirmed:    loop.Experiment.Admission.AuthorityMode == experiment.AuthorityOrdinary,
		PolicyAuthorized: loop.Experiment.Admission.AuthorityMode == experiment.AuthorityFull,
		Receipt:          cloneContext(receipt), ProcessorResponse: map[string]any{"processor_type": processorType}, AppliedAt: time.Now().UTC()}
	if events, err := loop.Experiment.ApplyIntervention(intervention, time.Now().UTC()); err == nil {
		s.emitFreeStateExperimentEvents(events)
	} else if s.logger != nil {
		s.logger.Warn("[free-state-experiment] intervention rejected: %v", err)
	}
}

// settleFreeStateExperiment writes the runtime settlement projection through
// the transient trajectory transport; Project History remains owned by the
// existing conversation/history layer.
func (s *Server) settleFreeStateExperiment(loop *freeStateReasoningLoop, outcome experiment.SettlementOutcome, summary string) {
	if loop == nil || loop.Experiment == nil || loop.Experiment.Status == experiment.StatusSettled || loop.Experiment.Status == experiment.StatusStopped {
		return
	}
	if err := s.completeTaskExperimentOutcome(loop, outcome, summary); err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-experiment] canonical settlement rejected: %v", err)
		}
		return
	}
	events, err := loop.Experiment.Settle(outcome, summary, time.Now().UTC())
	if err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-experiment] settlement rejected: %v", err)
		}
		return
	}
	s.emitFreeStateExperimentEvents(events)
}

func (s *Server) stopFreeStateExperiment(loop *freeStateReasoningLoop, summary string) {
	if loop == nil || loop.Experiment == nil || loop.Experiment.Status == experiment.StatusSettled || loop.Experiment.Status == experiment.StatusStopped {
		return
	}
	if err := s.completeTaskExperimentOutcome(loop, experiment.OutcomeStopped, summary); err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-experiment] canonical stop rejected: %v", err)
		}
		return
	}
	events, err := loop.Experiment.Stop(summary, time.Now().UTC())
	if err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[free-state-experiment] stop rejected: %v", err)
		}
		return
	}
	s.emitFreeStateExperimentEvents(events)
}

// rollbackFreeStateExperiment delegates actual restoration to the existing
// governed rollback command. The runtime only records its receipt and emits
// the trajectory projection; it does not implement a second undo mechanism.
func (s *Server) rollbackFreeStateExperiment(ctx context.Context, loop *freeStateReasoningLoop) ([]trajectory.Event, error) {
	if s == nil || s.harness == nil || loop == nil || loop.Experiment == nil {
		return nil, fmt.Errorf("rollback dependencies are unavailable")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return nil, err
	}
	if len(round.Interventions) == 0 {
		return nil, fmt.Errorf("rollback requires an intervention")
	}
	targetID := round.Interventions[len(round.Interventions)-1].ID
	response, invokeErr := s.harness.Invoke(ctx, harness.InvokeRequest{Command: map[string]any{"cmd": "agent_rollback_action", "target_action_id": targetID}, Context: map[string]any{"free_state_experiment": true}, Source: "free_state_experiment", Confirmed: true, GoalID: loop.GoalID, RunID: loop.RunID})
	if invokeErr != nil || strings.EqualFold(response.Status, "error") || strings.TrimSpace(response.Error) != "" {
		if invokeErr != nil {
			return nil, invokeErr
		}
		return nil, fmt.Errorf("rollback action failed: %s", firstNonEmpty(response.Error, response.Status))
	}
	receipt := cloneContext(response.Result)
	if receipt == nil {
		receipt = map[string]any{}
	}
	receipt["agent_action_id"] = response.AgentActionID
	receipt["status"] = firstNonEmpty(response.Status, "succeeded")
	return loop.Experiment.MarkRollback(time.Now().UTC(), receipt, []string{targetID})
}
