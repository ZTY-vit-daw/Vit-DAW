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
	if loop.Experiment.Admission.IsD1S1() &&
		(decision.ExperimentMateriality != nil || decision.ExperimentTargetResponse != nil || strings.TrimSpace(decision.ExperimentRoundDecision) != "") &&
		!freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment) {
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] settle report refused until the fresh post-action observation is recorded on the round")
		}
		return
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
	return freeStateLoopRoundPendingSettlement(loop)
}
