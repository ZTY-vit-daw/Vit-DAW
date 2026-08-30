package chat

import (
	"context"
	"fmt"
	"math"
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
//
// The default tier is the sealed D1-S1 single-round budget of 1; the D2-2
// multi-round tier is only reachable through the explicit server-side
// injection resolved by freeStateExperimentAdmissionWithTier callers.
func freeStateExperimentAdmission(loop freeStateReasoningLoop, proposal *agentprotocol.ImprovementProposal) (experiment.Admission, error) {
	return freeStateExperimentAdmissionWithTier(loop, proposal, 1)
}

// freeStateExperimentAdmissionWithTier is the tier-aware admission builder.
// tier<=1 keeps the historical D1-S1 single-round path byte-for-byte
// (construction, ValidateD1S1, then the fresh-observed-target binding).
// tier>1 builds the D2-2 multi-round admission: the budget comes exclusively
// from the server-side tier injection (the model never upgrades it), and the
// admission carries the server-derived experiment baseline fingerprint that
// anchors the ruling-2 cumulative dose accounting.
func freeStateExperimentAdmissionWithTier(loop freeStateReasoningLoop, proposal *agentprotocol.ImprovementProposal, tier int) (experiment.Admission, error) {
	if proposal == nil {
		return experiment.Admission{}, fmt.Errorf("improvement proposal is required")
	}
	// The D2-1 domain table is the production admission boundary: track_gain
	// keeps its original shape and plugin-bound domains are admitted with
	// equally tight per-domain bounds. Domain+kind must match one table row
	// (D1S1DomainSpecFor) so a hybrid action cannot borrow a domain's bounds.
	spec, specOK := experiment.D1S1DomainSpecFor(experiment.Admission{TypedAction: map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind}})
	if !specOK {
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
	if tier > 1 {
		budget = tier
	}
	var typedAction map[string]any
	var diagnosticBounds, retainedBounds map[string]any
	if !spec.WriteBinding.PluginBound {
		deltaDB, _ := treatmentNumber(proposal.ParameterBounds, "delta_db", "db_delta", "gain_delta_db")
		typedAction = map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind, "target_db": proposal.ParameterBounds["target_db"], "delta_db": deltaDB}
		diagnosticBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "delta_db": deltaDB, "max_action_attempts": 1}
		retainedBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), "delta_db": deltaDB, "max_action_attempts": 1}
	} else {
		// Plugin-bound domains carry their single bounded parameter (named by
		// the table's AdmissionValueKey) plus the verbatim passthrough keys;
		// both dose scopes use that key instead of delta_db. Nothing here
		// relaxes the shared D1-S1 bounds — the row validators remain the
		// enforcement authority.
		valueDB, _ := treatmentNumber(proposal.ParameterBounds, spec.AdmissionValueKey)
		typedAction = map[string]any{"action_domain": proposal.ActionDomain, "action_kind": proposal.ActionKind, spec.AdmissionValueKey: valueDB}
		for _, key := range spec.AdmissionPassthroughKeys {
			if value, exists := proposal.ParameterBounds[key]; exists {
				typedAction[key] = value
			}
		}
		diagnosticBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), spec.AdmissionValueKey: valueDB, "max_action_attempts": 1}
		retainedBounds = map[string]any{"source": "proposal", "bounds": cloneContext(bounds), spec.AdmissionValueKey: valueDB, "max_action_attempts": 1}
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
	if tier <= 1 {
		if err := admission.ValidateD1S1(); err != nil {
			return experiment.Admission{}, err
		}
		if _, err := validateD1FreshObservedTarget(loop, admission); err != nil {
			return experiment.Admission{}, err
		}
		return admission, nil
	}
	// D2-2 multi-round tier: the baseline fingerprint is derived server-side
	// from the fresh target observation (GLM ruling 2 anchor) and the admission
	// runs the S1 multi-round validation variant.
	observation, err := validateD1FreshObservedTarget(loop, admission)
	if err != nil {
		return experiment.Admission{}, err
	}
	admission.BaselineFingerprint = freeStateExperimentBaselineFingerprint(loop, observation)
	if err := admission.ValidateD2MultiRound(); err != nil {
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

// freeStateAdmissionRunsSingleRound reports whether the admission runs under
// the D1-S1 single-round guard set. IsD1S1 alone is pure domain membership and
// ignores the budget (the S1 tier trap: a domain member with budget 3 still
// answers IsD1S1()==true), so every chat-side auto-continuation guard must use
// this predicate instead of IsD1S1.
func freeStateAdmissionRunsSingleRound(admission experiment.Admission) bool {
	return admission.IsD1S1() && !admission.IsD2MultiRound()
}

// validateD1TierAdmission runs the tier-correct admission guard behind every
// D1 plan builder and execution entry: the single-round tier keeps
// ValidateD1S1 byte-for-byte (budget 1, sealed domain bounds), while the D2-2
// multi-round tier runs its S1-frozen variant (per-scope
// max_action_attempts==1, budget 2..4, baseline fingerprint). Budgets above
// the D2-2 ceiling fall back to the single-round guards exactly like the
// experiment runtime's isD1S1SingleRound, so an unknown tier fails closed.
func validateD1TierAdmission(admission experiment.Admission) error {
	if freeStateAdmissionRunsSingleRound(admission) {
		return admission.ValidateD1S1()
	}
	return admission.ValidateD2MultiRound()
}

// d1MultiRoundRoundScopeSuffix returns the per-round disambiguator for the
// D2-2 tier's durable identities (durable session id and journal action id).
// Round 1 keeps the historical experiment-level strings byte-for-byte; every
// later round appends _round_<n> so each round owns a distinct durable
// session, a distinct journal action, and a distinct idempotent transaction
// chain (the multi-round probe asserts per-round transaction-identity
// uniqueness and before/after revision chaining). The single-round tier
// always resolves to the empty suffix.
func d1MultiRoundRoundScopeSuffix(loop freeStateReasoningLoop) string {
	if loop.Experiment == nil || !loop.Experiment.Admission.IsD2MultiRound() {
		return ""
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || round.Number <= 1 {
		return ""
	}
	return fmt.Sprintf("_round_%d", round.Number)
}

// d2MultiRoundChatCumulativeBoundDB mirrors the S1-frozen experiment-lifetime
// cumulative displacement bound (2 dB, equal to every domain's single-action
// absolute bound; sealed on the experiment side by
// TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound). The chat-side
// pre-mutation refusal must not widen it independently of the frozen ruling.
const d2MultiRoundChatCumulativeBoundDB = 2.0

// d2MultiRoundCumulativeDeltaDB sums the signed achieved deltas recorded on
// every applied intervention of the experiment. It mirrors the experiment
// runtime's cumulative-dose accounting (GLM ruling 2): failed or ambiguous
// attempts record no trusted displacement and are skipped.
func d2MultiRoundCumulativeDeltaDB(turn *experiment.Turn, valueKey string) float64 {
	if turn == nil {
		return 0
	}
	total := 0.0
	for _, round := range turn.Rounds {
		for _, prior := range round.Interventions {
			if prior.TechnicalApplication != experiment.TechnicalApplied {
				continue
			}
			if value, ok := treatmentNumber(prior.AchievedDelta, valueKey); ok {
				total += value
			}
		}
	}
	return total
}

// d2MultiRoundCheckProjectedCumulativeDelta refuses a plan whose apply would
// push the experiment-lifetime cumulative displacement past the S1-frozen 2dB
// bound. It mirrors the experiment runtime's post-apply accounting at the
// plan boundary so the refusal lands before the kernel mutation instead of
// after it (a post-apply rejection would diverge the experiment ledger from
// the kernel state); the runtime check stays authoritative.
func d2MultiRoundCheckProjectedCumulativeDelta(turn *experiment.Turn, valueKey string, deltaDB float64) error {
	projected := d2MultiRoundCumulativeDeltaDB(turn, valueKey) + deltaDB
	if math.Abs(projected) > d2MultiRoundChatCumulativeBoundDB {
		return fmt.Errorf("D2-2 cumulative %s displacement %.4g dB would exceed the experiment-lifetime bound %.4g dB; refusing before the mutation", valueKey, projected, d2MultiRoundChatCumulativeBoundDB)
	}
	return nil
}

// freeStateExperimentBudgetExhausted reports whether the experiment has
// already spent its full intervention budget (one governed mutation per
// round, ExperimentBudget rounds). The frozen multi-round acceptance scope
// treats more rounds than budget as fatal, so the chat layer must stop
// opening calibration rounds here and settle instead.
func freeStateExperimentBudgetExhausted(turn *experiment.Turn) bool {
	return turn != nil && turn.InterventionCount() >= turn.Admission.ExperimentBudget
}

// freeStateD2RoundContinuationAllowance is the pre-apply continuation slice
// count a freshly opened D2-2 calibration round needs: one target
// observation, one bounded proposal, one mix-tick confirmation. The
// post-apply chain keeps its own phase-scoped floor via
// reserveD1PostApplySlices.
const freeStateD2RoundContinuationAllowance = 3

// grantD2CalibrationRoundContinuation makes a freshly opened D2-2 calibration
// round schedulable: the continuation budget gains one pre-apply allowance
// beyond what earlier rounds consumed, and the one-shot post-apply reserves
// re-arm — their unit is the applied round, not the experiment, so round 2's
// mandatory post-action chain must be able to reserve its own slices.
func grantD2CalibrationRoundContinuation(loop *freeStateReasoningLoop) {
	if loop == nil {
		return
	}
	if floor := loop.ContinuationUsed + freeStateD2RoundContinuationAllowance; floor > loop.ContinuationBudget {
		loop.ContinuationBudget = floor
	}
	loop.PostActionObservationReserved = false
	loop.PostApplyBudgetReserved = false
}

// freeStateExperimentJudgmentPending mirrors the experiment runtime's
// experiment-scope judgment boundary (GLM ruling 3): any round that requested
// a user judgment without the judgment landing parks the whole experiment
// until a settle-family decision or an explicit settlement.
func freeStateExperimentJudgmentPending(turn *experiment.Turn) bool {
	if turn == nil {
		return false
	}
	switch turn.Status {
	case experiment.StatusSettled, experiment.StatusStopped:
		return false
	}
	for _, round := range turn.Rounds {
		if round.UserJudgmentRequested && len(round.UserJudgmentEvidence) == 0 {
			return true
		}
	}
	return false
}

// freeStateExperimentBaselineFingerprint anchors the D2-2 cumulative dose
// accounting (GLM ruling 2) at the experiment baseline revision. It is derived
// server-side from the fresh target observation; a model-supplied fingerprint
// is never trusted.
func freeStateExperimentBaselineFingerprint(loop freeStateReasoningLoop, observation experiment.Observation) map[string]any {
	revision := firstNonEmpty(observation.ProjectRevision, firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"))
	return map[string]any{"revision": revision, "source": "free_state_admission", "observation_id": observation.ID}
}

func (s *Server) startFreeStateExperiment(loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision, goalID, runID string) error {
	if loop == nil || decision.ImprovementProposal == nil {
		return fmt.Errorf("experiment proposal is missing")
	}
	// GLM ruling 3 chat layer: a pending judgment on an existing experiment
	// refuses any new admission in the same loop. The error is the S1 runtime
	// sentinel so the boundary reads identically at both layers.
	if freeStateExperimentJudgmentPending(loop.Experiment) {
		return experiment.ErrJudgmentPending
	}
	// The D2-2 multi-round tier is a server-side injection only. Without it
	// (budget resolves to 1) every path below is byte-identical to the
	// historical single-round admission.
	tier := agentloop.ResolveD2MultiRoundBudget()
	var admission experiment.Admission
	var err error
	if decision.ExperimentAdmission != nil {
		admission = *decision.ExperimentAdmission
		if tier > 1 {
			// The model never upgrades (or narrows) the budget itself, and the
			// baseline fingerprint is always derived server-side from the
			// fresh target observation.
			admission.ExperimentBudget = tier
			var observation experiment.Observation
			if observation, err = validateD1FreshObservedTarget(*loop, admission); err == nil {
				admission.BaselineFingerprint = freeStateExperimentBaselineFingerprint(*loop, observation)
				err = admission.ValidateD2MultiRound()
			}
		} else {
			err = admission.ValidateD1S1()
			if err == nil {
				_, err = validateD1FreshObservedTarget(*loop, admission)
			}
		}
	} else {
		admission, err = freeStateExperimentAdmissionWithTier(*loop, decision.ImprovementProposal, tier)
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
	if tier > 1 {
		if err := admission.ValidateD2MultiRound(); err != nil {
			return err
		}
		if s != nil && s.logger != nil {
			s.logger.Info("[free-state-experiment] D2-2 multi-round admission tier budget=%d source=%s", tier, agentloop.FreeStateD2MultiRoundBudgetEnv)
		}
	} else if err := admission.ValidateD1S1(); err != nil {
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

// neutralizeFreeStateBoundaryResidue strips the durable judgment-boundary
// signal pair — the user_judgment_pending round decision and the
// human_audition_ready target response, which travel together per the
// settle-report contract — from the loop's latest decision projection. The
// pair is classification input for
// continuationRequiresUserInteraction: while it rides a LatestDecision that
// no longer describes the loop's live state, the next limit stop parks its
// checkpoint at an unanswerable empty-shell waiting_interaction and the owed
// round never runs again. Two sites own a stale pair: the refused settle
// report (S3f) and the round opening that follows a landed judgment or an
// insufficient-dose calibration decision (S3h5: the settled round-1 pair
// survives LatestDecision although the judgment landed, round 1 decided
// next_round and round 2 opened). A genuinely pending boundary keeps the
// pair — callers invoke this only after the boundary is factually released.
func neutralizeFreeStateBoundaryResidue(loop *freeStateReasoningLoop) {
	if loop == nil || loop.LatestDecision == nil {
		return
	}
	boundaryResidue := strings.EqualFold(strings.TrimSpace(loop.LatestDecision.ExperimentRoundDecision), string(experiment.DecisionUserJudgment)) ||
		(loop.LatestDecision.ExperimentTargetResponse != nil &&
			strings.EqualFold(strings.TrimSpace(string(loop.LatestDecision.ExperimentTargetResponse.Outcome)), string(trajectory.EvaluationHumanAuditionReady)))
	if !boundaryResidue {
		return
	}
	cleared := *loop.LatestDecision
	cleared.ExperimentRoundDecision = ""
	cleared.ExperimentTargetResponse = nil
	loop.LatestDecision = &cleared
}

func (s *Server) recordFreeStateExperimentDecision(ctx context.Context, loop *freeStateReasoningLoop, decision agentloop.FreeStateDecision) {
	if loop == nil || loop.Experiment == nil {
		return
	}
	// A D1 settle report (materiality/target response/round decision) may only
	// land after the mandatory fresh post-action CCB observation is recorded on
	// the round. Recording it earlier parks the round at the human judgment
	// boundary without the evidence that boundary is supposed to arbitrate
	// (2026-08-28 10:27/10:42 smokes: the settle turn skipped the observation,
	// the round parked on user_judgment_pending with zero post-action evidence).
	// The round state is the ground truth here, not RequiresPostActionObservation:
	// the needs_experiment branch of recordFreeStateDecision legitimately clears
	// that flag for proposal-carrying decisions before this ingest runs.
	// Refusing keeps the loop live with its reserved post-apply budget so the
	// next slice observes first; no evidence gate is weakened.
	settleReportCarried := decision.ExperimentMateriality != nil || decision.ExperimentTargetResponse != nil ||
		strings.TrimSpace(decision.ExperimentRoundDecision) != ""
	if loop.Experiment.Admission.IsD1S1() && settleReportCarried &&
		!freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment) {
		if s.logger != nil {
			// The refusal copy splits by round type (S3h2): a round that never
			// acted cannot discharge a post-action-observation debt, so saying
			// "wait for the observation" there sends the model into settle
			// replays (20260829_205921 trace: zero tool calls, budget burnt).
			if freeStateLoopRoundNeverActed(*loop) {
				s.logger.Warn("[free-state-experiment] settle report refused on a round that still owes its bounded intervention (fresh base %s); the round must propose against that base, not settle",
					freeStateOwedRoundBaseReference(*loop))
			} else {
				s.logger.Warn("[free-state-experiment] settle report refused until the fresh post-action observation is recorded on the round")
			}
		}
		// The refused report must not park the driving continuation at a
		// judgment boundary it failed to form: strip its full boundary signal
		// pair — the user_judgment_pending round decision AND the
		// human_audition_ready target response, which travel together per the
		// settle-report contract — from the loop projection so the settle chain
		// stays schedulable and retries once the deterministic post-action
		// booking lands (2026-08-29 S3c smoke: the refused round-2 report parked
		// its continuation at waiting_interaction and the settle turn never ran
		// again; 2026-08-29 S3f 183540: clearing only the round decision left
		// the target response on the envelope, and the continuation
		// scheduler's decision double-check still parked the turn behind an
		// unanswerable empty-shell interaction).
		neutralizeFreeStateBoundaryResidue(loop)
		// A settle report exists only for a round that already spent its
		// single mutation, but in this race window the experiment projection
		// can still show the round pre-action (the intervention booking lags
		// the transport receipt), so neither the pending-settlement nor the
		// spent-mutation predicate will hold when the same envelope's pending
		// tick reaches the recordGoalResult guard. Mark the round durably:
		// the refused settle is owed, the round owes no new mutation, and the
		// guard must see this post-refusal state instead of the pre-race
		// snapshot (2026-08-29 17:00:50 smoke: refused settle and the replayed
		// tick bind in the same second, one waiting_interaction leftover).
		if round, roundErr := loop.Experiment.CurrentRound(); roundErr == nil {
			loop.SettleRefusedRoundID = round.ID
		}
		return
	}
	if settleReportCarried {
		// The report landed (the round carries its fresh post-action
		// observation): the race window is closed, the round decided or parked
		// at its judgment boundary by the ingest below.
		loop.SettleRefusedRoundID = ""
	}
	if decision.ExperimentMateriality != nil {
		if events, err := loop.Experiment.EvaluateMateriality(*decision.ExperimentMateriality, time.Now().UTC()); err == nil {
			s.emitFreeStateExperimentEvents(events)
			// Insufficient dose is an effect-size signal, never ambiguity: the
			// non-single-round tiers may open one calibration round, while the
			// D1-S1 single-round tier keeps intercepting it (GLM ruling 1).
			// Opening a round never raises a dose by itself; any dose change
			// still passes the admission domain-table validation. With the
			// experiment budget already spent, the frozen multi-round contract
			// (rounds may never exceed budget) settles the experiment instead
			// of opening a beyond-budget calibration round.
			if decision.ExperimentMateriality.Evaluation == trajectory.EvaluationInsufficientDose && !freeStateAdmissionRunsSingleRound(loop.Experiment.Admission) {
				if freeStateExperimentBudgetExhausted(loop.Experiment) {
					s.settleFreeStateExperiment(loop, experiment.OutcomeBudgetExhausted, "multi-round budget exhausted; no calibration round may open")
					if loop.Experiment.Status == experiment.StatusSettled {
						loop.Status = "completed"
						loop.RequiresPostActionObservation = false
					}
				} else if decisionEvents, decisionErr := loop.Experiment.DecideRound(experiment.DecisionNextRound, "insufficient dose; calibrate in next round", time.Now().UTC()); decisionErr == nil {
					s.emitFreeStateExperimentEvents(decisionEvents)
					views := freeStateExperimentViews(*loop, decision, decision.ImprovementProposal)
					if len(views) == 0 {
						views = append([]string(nil), loop.Experiment.Rounds[len(loop.Experiment.Rounds)-1].RequestedViewIDs...)
					}
					if roundEvents, roundErr := loop.Experiment.StartRound(views, loop.Experiment.Admission.CheckpointRef, firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), time.Now().UTC()); roundErr == nil {
						s.emitFreeStateExperimentEvents(roundEvents)
						grantD2CalibrationRoundContinuation(loop)
						// The freshly opened calibration round makes the settled
						// report's boundary signals factually stale; a residue on
						// LatestDecision would misclassify the round's next limit
						// stop as an interaction boundary (S3h5, same family as
						// the refused-settle neutralization above).
						neutralizeFreeStateBoundaryResidue(loop)
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
	// The D2-2 tier's cumulative-dose accounting (GLM ruling 2) requires every
	// applied intervention to carry its achieved displacement at the domain's
	// value key; the intervention receipt also states the numeric applied
	// delta because the VSP port receipts carry absolute readbacks (not
	// deltas) and the multi-round probe doses from the intervention receipt.
	// The single-round tier keeps its historical intervention shape.
	if technical == experiment.TechnicalApplied && loop.Experiment.Admission.IsD2MultiRound() {
		if spec, ok := experiment.D1S1DomainSpecFor(loop.Experiment.Admission); ok {
			if value, valueOK := treatmentNumber(loop.Experiment.Admission.TypedAction, spec.AdmissionValueKey); valueOK {
				intervention.AchievedDelta = map[string]any{spec.AdmissionValueKey: value}
				intervention.Receipt["applied_delta_db"] = value
			}
		}
		// A round's mutation can advance the live revision more than once
		// (PluginBound domains: the round-scoped plugin instance creation plus
		// the parameter batch), and the port receipt brackets only the final
		// write. The frozen multi-round contract chains each round's receipt
		// onto the previous round's after_revision, so on this tier the
		// receipt's before_revision states the round's full mutation envelope:
		// the revision this round started from (2026-08-29 S3c smoke: round 2
		// instantiated its plugin at 4->5 and wrote the parameter at 5->6,
		// leaving a receipt that began at 5 instead of the round base 4).
		if len(round.Interventions) == 0 && len(loop.Experiment.Rounds) >= 2 {
			previous := loop.Experiment.Rounds[len(loop.Experiment.Rounds)-2]
			if interventions := previous.Interventions; len(interventions) > 0 {
				if base := firstStringFromMap(interventions[len(interventions)-1].Receipt, "after_revision", "applied_revision"); base != "" &&
					firstStringFromMap(intervention.Receipt, "before_revision") != base {
					intervention.Receipt["before_revision"] = base
				}
			}
		}
	}
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

// freeStateExperimentRoundHasFreshPostActionObservation reports whether the
// current experiment round already carries the mandatory fresh post-action CCB
// observation. It is the round-state ground truth behind the D1 settle-report
// admission guard in recordFreeStateExperimentDecision.
func freeStateExperimentRoundHasFreshPostActionObservation(turn *experiment.Turn) bool {
	if turn == nil {
		return false
	}
	round, err := turn.CurrentRound()
	if err != nil {
		return false
	}
	for index := len(round.Observations) - 1; index >= 0; index-- {
		if round.Observations[index].PostAction && round.Observations[index].Fresh {
			return true
		}
	}
	return false
}

// freeStateLoopRoundSpentMutation reports whether the current experiment
// round has spent its single mutation budget without its round decision
// landing yet — including the post-action-observation race window where the
// deterministic booking has not arrived. Every further pending mix
// tick/treatment is an illegal second mutation in that state.
func freeStateLoopRoundSpentMutation(loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil || !strings.EqualFold(strings.TrimSpace(string(loop.Experiment.Status)), string(experiment.StatusRunning)) {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.Interventions) == 0 {
		return false
	}
	return strings.TrimSpace(string(round.Decision)) == ""
}

// freeStateLoopRoundPendingSettlement reports whether the loop's applied
// experiment round carries fresh post-action evidence but no settlement
// decision. The round state is authoritative: storeFreeStateLoop rewrites
// DecisionPhase from RequiresPostActionObservation, so a booked observation
// can legitimately leave the persisted phase at processor_selection while the
// round still owes its settle report. While this holds the round's single
// mutation budget is spent: any further pending mix tick/treatment interaction
// is an illegal second mutation, and projecting the goal completed would
// permanently lose the settlement (S2e, 2026-08-28 121306→130901 smokes).
func freeStateLoopRoundPendingSettlement(loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil || !strings.EqualFold(strings.TrimSpace(string(loop.Experiment.Status)), string(experiment.StatusRunning)) {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || strings.TrimSpace(string(round.Decision)) != "" {
		return false
	}
	return freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment)
}

// freeStateLoopRoundSettleRefused reports whether the current round's settle
// report was refused because its fresh post-action observation had not landed
// yet — the post-action-observation race window. The experiment projection in
// that window can still show the round pre-action (the intervention booking
// lags the transport receipt), so the spent-mutation and pending-settlement
// predicates both read false on it; the refusal marker recorded by
// recordFreeStateExperimentDecision is the authoritative same-round state.
// While it holds the round owes its settle retry and no new pending mix
// tick/treatment may be accepted (an illegal second mutation mid-round).
func freeStateLoopRoundSettleRefused(loop freeStateReasoningLoop) bool {
	if strings.TrimSpace(loop.SettleRefusedRoundID) == "" || loop.Experiment == nil ||
		!strings.EqualFold(strings.TrimSpace(string(loop.Experiment.Status)), string(experiment.StatusRunning)) {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || round.ID != loop.SettleRefusedRoundID {
		return false
	}
	return strings.TrimSpace(string(round.Decision)) == ""
}

// freeStateLoopRoundOwesIntervention reports whether the loop's freshly opened
// D2-2 recalibration/calibration round still owes its single governed
// intervention. The closure round limit is an observation-window bound and must
// not kill a cross-judgment recalibration round whose intervention budget is
// unspent — the same asymmetry the settle window already extends for (a round
// that owes its settle report). Exactly that recalibration boundary satisfies
// neither legacy condition: the previous round's post-action observation is
// booked (RequiresPostActionObservation=false) and the new round carries no
// fresh post-action evidence (freeStateLoopRoundPendingSettlement=false), so
// without this branch any scheduler re-entry settles capability_blocked before
// round 2 can act (2026-08-29 S3b smoke: "closure observation round boundary
// reached" between the recalibration decision and round 2's intervention). The
// single-round D1 tier is excluded — its boundary behavior is sealed and it
// never opens a recalibration round — and the budget guard keeps the frozen
// contract intact: one intervention per round, at most ExperimentBudget per
// experiment, and the extension grants neither.
func freeStateLoopRoundOwesIntervention(loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil || !strings.EqualFold(strings.TrimSpace(string(loop.Experiment.Status)), string(experiment.StatusRunning)) {
		return false
	}
	if !loop.Experiment.Admission.IsD2MultiRound() || freeStateExperimentBudgetExhausted(loop.Experiment) {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.Interventions) != 0 {
		return false
	}
	return true
}

// freeStateLoopRoundNeverActed separates the two states that both project as a
// zero-intervention round: a freshly opened recalibration round that genuinely
// owes its bounded intervention, and the settle-report race window where the
// round already executed but the intervention booking lags the transport
// receipt. RequiresPostActionObservation is the applied-boundary debt bit —
// true exactly while an executed mutation awaits its post-action booking — so
// it is the discriminator: the race window carries the debt, the never-acted
// round does not (20260829_205921 trace: the misdirected settle report was
// refused on a never-acted round-2 whose guidance then said "wait for the
// observation", a debt that cannot discharge there).
func freeStateLoopRoundNeverActed(loop freeStateReasoningLoop) bool {
	return freeStateLoopRoundOwesIntervention(loop) && !loop.RequiresPostActionObservation
}

// freeStateOwedRoundBaseReference restates the never-acted round's fresh base
// as the mechanical citation "obs_id@revision" — the same base observation
// S3h1's recalibrationRoundBaseRecent projects into the model-visible catalog,
// so the refusal guidance quotes exactly the reference the ledger presents.
func freeStateOwedRoundBaseReference(loop freeStateReasoningLoop) string {
	if loop.Experiment == nil {
		return ""
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return ""
	}
	for index := len(round.Observations) - 1; index >= 0; index-- {
		observation := round.Observations[index]
		if observation.PostAction || !observation.Fresh {
			continue
		}
		obsID := strings.TrimSpace(observation.ID)
		revision := strings.TrimSpace(observation.ProjectRevision)
		if obsID == "" || revision == "" {
			continue
		}
		return obsID + "@" + revision
	}
	return ""
}

// armFreeStateRecalibrationContinuation makes the freshly opened recalibration
// round drivable. The judgment boundary is an out-of-band POST: the driving
// continuation chain was already drained when the loop parked at the audition
// boundary, so after the recalibration decision opens the next round a continue
// nudge would find nothing to continue ("当前没有可继续的暂停任务") and the
// owed round could never act (2026-08-29 S3b/S3c smoke: round 2 opened, every
// nudge returned no_continuation, the intervention never executed). Arm the
// goal continuation with an internal-resume context so the ordinary continue
// path runs the recalibration round's slice chain. Gated by the
// owed-intervention predicate (multi-round tier only, budget unspent), refuses
// to clobber an existing continuation, and never arms the sealed single-round
// tier.
func (s *Server) armFreeStateRecalibrationContinuation(loop *freeStateReasoningLoop) {
	if s == nil || loop == nil || !freeStateLoopRoundOwesIntervention(*loop) || freeStateContinuationBudgetExhausted(*loop) {
		return
	}
	goalID := strings.TrimSpace(loop.GoalID)
	if goalID == "" || strings.TrimSpace(loop.ConversationID) == "" {
		return
	}
	s.mu.Lock()
	if s.goalContinuations == nil || s.conversationGoals == nil {
		s.mu.Unlock()
		return
	}
	if _, exists := s.goalContinuations[goalID]; exists {
		s.mu.Unlock()
		return
	}
	s.goalContinuations[goalID] = agentloop.Continuation{
		GoalID: goalID, RunID: loop.RunID,
		UserText:       firstNonEmpty(loop.ActiveIntent, loop.OriginalIntent),
		OriginalIntent: loop.OriginalIntent,
		Context: map[string]any{
			"conversation_id":             loop.ConversationID,
			"goal_id":                     goalID,
			"run_id":                      loop.RunID,
			"free_state_internal_resume":  true,
			"free_state_route_authorized": true,
			"free_state_reasoning_loop":   freeStateLoopMap(*loop),
			"free_state_project_change":   cloneContext(loop.LatestProjectChange),
		},
	}
	s.conversationGoals[loop.ConversationID] = goalID
	s.mu.Unlock()
}

// bookFreeStateRecalibrationRoundBase books a fresh CCB observation as the
// freshly opened recalibration round's pre-action base. Round 1 gets its base
// deterministically at admission; a later round must take its own fresh
// observation, and the model turn that produces it ends in a native-tool
// proposal confirmation carrying no typed decision — so the decision-gated
// booking site never runs, the confirmed action then dies at the "D1-S1 VSP
// base revision does not match the fresh admitted observation" gate, and the
// owed round can never act (2026-08-29 S3c smoke). Gated by the
// owed-intervention predicate: only a multi-round round that has not acted
// yet, with the judgment boundary released.
func (s *Server) bookFreeStateRecalibrationRoundBase(loop *freeStateReasoningLoop, observations []*agentloop.RecentObservation) {
	if s == nil || loop == nil || loop.Experiment == nil || !freeStateLoopRoundOwesIntervention(*loop) || freeStateJudgmentBoundary(*loop) {
		return
	}
	candidates := append([]*agentloop.RecentObservation{}, observations...)
	if loop.LatestObservation != nil {
		candidates = append(candidates, loop.LatestObservation)
	}
	for _, current := range candidates {
		if current == nil {
			continue
		}
		observation, ok := freeStateExperimentObservation(current, false)
		if !ok || freeStateExperimentHasObservation(loop.Experiment, observation.ID) {
			continue
		}
		if base := freeStateRecalibrationBaseRevision(*loop); base != "" && observation.ProjectRevision != base {
			continue
		}
		if events, err := loop.Experiment.RecordObservation(observation, false, time.Now().UTC()); err == nil {
			s.emitFreeStateExperimentEvents(events)
			return
		} else if s.logger != nil {
			s.logger.Warn("[free-state-experiment] recalibration base observation rejected: %v", err)
		}
	}
}

// bookRecalibrationRoundBaseFromLoop mirrors the admission's deterministic
// before-observation booking for a freshly opened recalibration round: re-base
// the round on the judged round's freshest post-action observation — the
// authoritative bundle at the mutation's after revision, which is still the
// live project state. The deterministic post-action booking writes that bundle
// onto the experiment round without ever flowing through
// loop.LatestObservation (which stays at the pre-action bundle), so the base
// must be sourced from the experiment's own rounds. The execution gate admits
// the round's bounded action only against a revision-matched non-post-action
// base on the round itself, and the proposing model turn ends in a native-tool
// confirmation whose result envelope never reaches the decision-gated booking
// path — so the base must be booked at the round boundary (2026-08-29 S3c
// smoke: the confirmed round-2 action died at "D1-S1 VSP base revision does
// not match the fresh admitted observation"). Re-basing keeps the original
// evidence identity: the runtime has no cross-round observation-id dedup, and
// the bundle genuinely is the freshest observation of the base revision.
func (s *Server) bookRecalibrationRoundBaseFromLoop(loop *freeStateReasoningLoop) {
	if s == nil || loop == nil || loop.Experiment == nil || !freeStateLoopRoundOwesIntervention(*loop) {
		return
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.Observations) > 0 || len(loop.Experiment.Rounds) < 2 {
		return
	}
	previous := loop.Experiment.Rounds[len(loop.Experiment.Rounds)-2]
	base, found := experiment.Observation{}, false
	for index := len(previous.Observations) - 1; index >= 0; index-- {
		if candidate := previous.Observations[index]; candidate.PostAction && candidate.Fresh && strings.TrimSpace(candidate.ProjectRevision) != "" {
			base, found = candidate, true
			break
		}
	}
	if !found {
		return
	}
	base.PostAction = false
	if baseRevision := freeStateRecalibrationBaseRevision(*loop); baseRevision != "" && base.ProjectRevision != baseRevision {
		return
	}
	events, bookErr := loop.Experiment.RecordObservation(base, false, time.Now().UTC())
	if bookErr != nil {
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] recalibration base observation rejected: %v", bookErr)
		}
		return
	}
	s.emitFreeStateExperimentEvents(events)
	// The booked base is also the round's model-visible evidence baseline.
	// Project it into the observation ledger so the proposal turn's catalog
	// presents the fresh obs id + revision instead of the pre-action pointer
	// the model would otherwise quote and be refused for (2026-08-29 S3h
	// smoke: available_views kept serving the round-1 pre-action observation
	// while the post-action bundle only ever reached the receipts).
	loop.ObservationLedger = supersedeFreeStateRoundBaseReceipts(loop.ObservationLedger, base)
	loop.ObservationLedger = mergeFreeStateObservationLedger(
		loop.ObservationLedger,
		recalibrationRoundBaseRecent(loop.Experiment, base),
		loop.Cycle)
}

// recalibrationRoundBaseRecent restates the booked round base in the compact
// CCB observation shape the observation ledger consumes. Only booking-carried
// facts are restated — identity, executed view set, revision binding, evidence
// refs, and the booking receipt identity; the original bundle's per-view
// conclusions are not reconstructed, so the row points at the observation
// instead of fabricating conclusions.
func recalibrationRoundBaseRecent(exp *experiment.Turn, base experiment.Observation) *agentloop.RecentObservation {
	executed := base.ExecutedViewIDs
	if len(executed) == 0 {
		executed = base.RequestedViewIDs
	}
	return &agentloop.RecentObservation{
		ToolCallID: "d1_round_base:" + sanitizeCanaryID(exp.ID), Tool: "ccb.observation_request",
		CommandName: "ccb_observation_request", Status: "ready",
		Summary: map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready",
			"observation_id": base.ID,
			"requested_views": append([]string(nil), base.RequestedViewIDs...),
			"actual_executed_view_ids": append([]string(nil), executed...),
			"evidence_refs":            append([]string(nil), base.EvidenceRefs...),
			"target_ref":               cloneContext(exp.Admission.TargetRef),
			"project_binding":          map[string]any{"project_revision": base.ProjectRevision},
			"freshness":                map[string]any{"status": "ready", "class": "current_observation", "project_revision": base.ProjectRevision},
			"audit_receipt": map[string]any{
				"receipt_id": firstNonEmpty(base.ReceiptID, "d1_ccb:"+base.ID),
				"freshness":  map[string]any{"class": "current_observation", "project_revision": base.ProjectRevision},
			},
		},
	}
}

// supersedeFreeStateRoundBaseReceipts restates the ledger's receipt rows for
// the round base observation at the boundary: the base is the judged round's
// post-action bundle re-based as the live revision's current observation, so
// its receipts must present the revision-bound current_observation restatement
// instead of the deterministic booking's post_action class (which the G7
// freshness vocabulary refuses), and duplicate booking echoes of the same
// observation collapse into a single restated row. Foreign observations and
// the row's own history fields (round, tool_call_id) are preserved.
func supersedeFreeStateRoundBaseReceipts(ledger map[string]any, base experiment.Observation) map[string]any {
	rows := freeStateMapRows(ledger["receipts"])
	matched := false
	for _, row := range rows {
		if firstStringFromMap(row, "observation_id") == base.ID {
			matched = true
			break
		}
	}
	if !matched {
		return ledger
	}
	ledger = cloneContext(ledger)
	out := make([]map[string]any, 0, len(rows))
	superseded := false
	for _, row := range rows {
		if firstStringFromMap(row, "observation_id") != base.ID {
			out = append(out, row)
			continue
		}
		if superseded {
			continue
		}
		restated := cloneContext(row)
		restated["status"] = firstNonEmpty(firstStringFromMap(restated, "status"), "ready")
		restated["project_revision"] = base.ProjectRevision
		restated["receipt_id"] = firstNonEmpty(
			firstStringFromMap(restated, "receipt_id"), firstNonEmpty(base.ReceiptID, "d1_ccb:"+base.ID))
		if freshness := firstMapFromAny(restated["freshness"]); len(freshness) > 0 {
			freshness["class"] = "current_observation"
			freshness["status"] = firstNonEmpty(firstStringFromMap(freshness, "status"), "ready")
			freshness["project_revision"] = base.ProjectRevision
			restated["freshness"] = freshness
		} else {
			restated["freshness"] = map[string]any{
				"class": "current_observation", "status": "ready", "project_revision": base.ProjectRevision,
			}
		}
		out = append(out, restated)
		superseded = true
	}
	ledger["receipts"] = out
	if count := freeStateLedgerCount(ledger["receipt_count"], 0); count > len(out) {
		ledger["receipt_count"] = len(out)
	}
	return ledger
}

// freeStateRecalibrationBaseRevision is the live project revision the freshly
// opened recalibration round re-bases on: the judged round's mutation after
// revision (booked in LatestProjectChange), falling back to the previous
// round's recorded revision.
func freeStateRecalibrationBaseRevision(loop freeStateReasoningLoop) string {
	if revision := firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"); revision != "" {
		return revision
	}
	if loop.Experiment == nil || len(loop.Experiment.Rounds) < 2 {
		return ""
	}
	previous := loop.Experiment.Rounds[len(loop.Experiment.Rounds)-2]
	if interventions := previous.Interventions; len(interventions) > 0 {
		if revision := firstStringFromMap(interventions[len(interventions)-1].Receipt, "after_revision", "applied_revision"); revision != "" {
			return revision
		}
	}
	return previous.ProjectRevision
}

// freeStateRoundPendingSettlementForConversation is the server-facing lookup
// used by the interaction-storage refusals. It prefers the durable loop.
func (s *Server) freeStateRoundPendingSettlementForConversation(conversationID string) bool {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return false
	}
	// Same triple condition as the recordGoalResult guard: a round that spent
	// its mutation (projection-booked or refusal-marked) and still owes its
	// settle report must not surface or accept another mutation.
	return freeStateLoopRoundPendingSettlement(loop) || freeStateLoopRoundSpentMutation(loop) ||
		freeStateLoopRoundSettleRefused(loop)
}
