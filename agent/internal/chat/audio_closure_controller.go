package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

const audioClosureContextKey = "minimal_audio_closure"

func orchestrationControllerDecisionMap(decision orchestrationcontroller.Decision) map[string]any {
	return map[string]any{
		"schema_version": decision.SchemaVersion, "controller": string(decision.Controller),
		"target_scope": decision.TargetScope, "authorization": decision.Authorization,
		"source_route": decision.SourceRoute, "reason": decision.Reason,
	}
}

func orchestrationControllerDecisionFromContext(requestContext map[string]any) (orchestrationcontroller.Decision, bool) {
	row := firstMapFromAny(requestContext[orchestrationDecisionContextKey])
	if len(row) == 0 {
		return orchestrationcontroller.Decision{}, false
	}
	decision := orchestrationcontroller.Decision{
		SchemaVersion: firstStringFromMap(row, "schema_version"),
		Controller:    orchestrationcontroller.Kind(firstStringFromMap(row, "controller")),
		TargetScope:   firstStringFromMap(row, "target_scope"), Authorization: firstStringFromMap(row, "authorization"),
		SourceRoute: firstStringFromMap(row, "source_route"), Reason: firstStringFromMap(row, "reason"),
	}
	return decision, decision.SchemaVersion == orchestrationcontroller.DecisionSchema
}

func (s *Server) hasActiveAudioClosure(conversationID string) bool {
	if s == nil || s.audioClosures == nil {
		return false
	}
	_, ok := s.audioClosures.ActiveForConversation(conversationID)
	return ok
}

func (s *Server) prepareAudioClosureContext(conversationID, userText string, requestContext map[string]any) (map[string]any, audioclosure.State, bool, error) {
	if s == nil {
		return requestContext, audioclosure.State{}, false, nil
	}
	if s.audioClosures == nil {
		s.audioClosures = audioclosure.NewMemoryStore()
	}
	if state, ok := s.audioClosures.ActiveForConversation(conversationID); ok {
		requestContext = s.bindCurrentTaskSemantics(requestContext, state.GoalID)
		requestRevision := audioClosureRequestProjectRevision(s, requestContext)
		if requestRevision != "" && state.ProjectRevision != "" && requestRevision != state.ProjectRevision {
			driver := audioclosure.Driver{}
			previous := state
			var err error
			if state.ActiveCapability != nil {
				_, _ = s.transitionTaskSemantic(state.GoalID, taskstate.TransitionRequest{Event: taskstate.EventCapabilityBlocked, Reason: "project revision changed during an active capability", Summary: "active capability evidence became stale", ProjectRevision: requestRevision})
				state, _, err = driver.SettleCapability(state, state.Revision, audioclosure.CapabilitySettlement{
					SessionID: state.ActiveCapability.SessionID, ActionID: state.ActiveCapability.ActionID,
					Status: "stale", Reason: "project revision changed before the next closure round",
				}, time.Now().UTC())
			} else {
				if s.hasTaskSemanticContract(state.GoalID) {
					if _, err = s.transitionTaskSemantic(state.GoalID, taskstate.TransitionRequest{Event: taskstate.EventProjectRevisionChanged, Reason: "authoritative project revision changed", Summary: "previous observation evidence was invalidated", ProjectRevision: requestRevision}); err == nil {
						state, _, err = driver.RevalidateProjectRevision(state, state.Revision, requestRevision, time.Now().UTC())
						if err == nil {
							state, err = s.projectAudioClosureTaskState(state)
						}
					}
				} else {
					state, err = driver.Settle(state, state.Revision, audioclosure.StopProjectRevisionStale,
						"project revision changed before the next closure round", false, time.Now().UTC())
				}
			}
			if err != nil {
				return requestContext, previous, true, err
			}
			if err := s.audioClosures.Save(state, previous.Revision); err != nil {
				return requestContext, previous, true, err
			}
			if state.Terminal() {
				s.settleAudioClosureOwner(state)
			}
			s.persistCurrentProjectWorkspace()
			return bindAudioClosureContext(requestContext, state), state, true, nil
		}
		if err := s.ensureAudioClosureOwner(state); err != nil {
			return requestContext, state, true, err
		}
		return bindAudioClosureContext(requestContext, state), state, true, nil
	}
	decision, ok := orchestrationControllerDecisionFromContext(requestContext)
	if !ok || decision.Controller != orchestrationcontroller.MinimalAudioClosure {
		return requestContext, audioclosure.State{}, false, nil
	}
	entry, verified := semanticEntryDecisionFromContext(requestContext)
	if !verified {
		return requestContext, audioclosure.State{}, false, fmt.Errorf("minimal audio closure requires a verified semantic entry")
	}
	mode := audioclosure.ModeTreatment
	if entry.Route == semanticEntryRouteObservation {
		mode = audioclosure.ModeDiagnostic
	}
	projectUUID := firstNonEmpty(firstStringFromMap(requestContext, "project_uuid", "project_id"), s.activeWorkspaceUUID)
	if projectUUID == "" {
		// Conversation-scoped fallback is explicit and stable. It allows tests
		// and an unsaved new project to use the controller without pretending
		// that observations from a later saved project share its identity.
		projectUUID = "unsaved:" + conversationID
	}
	projectRevision := audioClosureRequestProjectRevision(s, requestContext)
	scope := audioClosureScope(entry.TargetScope, requestContext, projectUUID)
	requestContext, err := s.ensureAudioTaskContract(conversationID, mode, scope, projectUUID, projectRevision, requestContext)
	if err != nil {
		return requestContext, audioclosure.State{}, false, err
	}
	goal := agentruntime.Goal{}
	if s.harness != nil {
		goal = s.harness.RuntimeStatus(firstStringFromMap(requestContext, "goal_id"))
	}
	originalIntent := strings.TrimSpace(userText)
	taskID, contractID := "", ""
	var semanticState taskstate.State
	var semanticRevision uint64
	if goal.Task != nil {
		originalIntent = firstNonEmpty(goal.Task.OriginalIntent, originalIntent)
		taskID = goal.Task.TaskID
		if goal.Task.Contract != nil && goal.Task.SemanticState != nil {
			contractID, semanticState, semanticRevision = goal.Task.Contract.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision
		}
	}
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "audio_closure_" + randomID(), ConversationID: conversationID,
		TaskID: taskID, GoalID: firstStringFromMap(requestContext, "goal_id"), RunID: firstStringFromMap(requestContext, "run_id"),
		ContractID: contractID, TaskState: semanticState, TaskStateRevision: semanticRevision,
		ProjectUUID: projectUUID, ProjectRevision: projectRevision, OriginalIntent: originalIntent,
		Mode: mode, Scope: scope, Now: time.Now().UTC(),
	})
	if err != nil {
		return requestContext, audioclosure.State{}, false, err
	}
	if err := s.audioClosures.Create(state); err != nil {
		return requestContext, audioclosure.State{}, false, err
	}
	if err := s.ensureAudioClosureOwner(state); err != nil {
		settled, settleErr := (audioclosure.Driver{}).Settle(state, state.Revision, audioclosure.StopCancelled, err.Error(), false, time.Now().UTC())
		if settleErr == nil {
			_ = s.audioClosures.Save(settled, state.Revision)
		}
		return requestContext, state, false, err
	}
	s.persistCurrentProjectWorkspace()
	return bindAudioClosureContext(requestContext, state), state, true, nil
}

func (s *Server) projectAudioClosureTaskState(state audioclosure.State) (audioclosure.State, error) {
	if s == nil || s.harness == nil || state.ContractID == "" {
		return state, nil
	}
	goal := s.harness.RuntimeStatus(state.GoalID)
	if goal.Task == nil || goal.Task.TaskID != state.TaskID || goal.Task.Contract == nil || goal.Task.SemanticState == nil || goal.Task.Contract.ContractID != state.ContractID {
		return state, fmt.Errorf("audio closure task identity does not match canonical runtime")
	}
	next, _, err := (audioclosure.Driver{}).ProjectTaskState(state, state.Revision, state.ContractID, goal.Task.SemanticState.State, goal.Task.SemanticState.Revision, time.Now().UTC())
	return next, err
}

func audioClosureRequestProjectRevision(s *Server, requestContext map[string]any) string {
	// The closure/task contract must bind to the authoritative Shadow snapshot,
	// which is populated from VSPStateResult.Revision. A UI/session value is
	// only a fallback for unsaved/test projects and must never override a live
	// kernel revision (notably the historical "0" placeholder).
	if s != nil && s.harness != nil {
		if state := s.harness.UserStateSummary(context.Background()); len(state) > 0 {
			if revision := firstStringFromMap(state, "project_revision"); revision != "" && revision != "0" {
				return revision
			}
		}
	}
	revision := firstStringFromMap(requestContext,
		"project_revision", "project_state_revision", "project_cut_hash", "state_token", "vsp_state_token")
	if revision == "0" {
		// Zero is the historical unbound placeholder, not a VSP revision.
		return ""
	}
	if revision == "" && s != nil {
		revision = s.activeWorkspaceSessionID
	}
	return revision
}

func audioClosureScope(targetScope string, requestContext map[string]any, projectUUID string) audioclosure.Scope {
	if targetScope == semanticEntryScopeCurrentSelection {
		id := firstStringFromMap(requestContext, "selected_track_id", "selected_scene_track_id", "selected_plugin_track_id", "selected_clip_id", "piano_roll_focus_clip_id")
		kind := "selection"
		if contextHasAnyValue(requestContext, "selected_track_id", "selected_scene_track_id", "selected_plugin_track_id") {
			kind = "track"
		} else if contextHasAnyValue(requestContext, "selected_clip_id", "piano_roll_focus_clip_id") {
			kind = "clip"
		}
		return audioclosure.Scope{Kind: kind, ID: id, Label: firstStringFromMap(requestContext, "selected_track_name", "selected_clip_name")}
	}
	return audioclosure.Scope{Kind: "project", ID: projectUUID}
}

func (s *Server) ensureAudioClosureOwner(state audioclosure.State) error {
	if s.controllerOwners == nil {
		s.controllerOwners = orchestrationcontroller.NewRegistry()
	}
	if owner, ok := s.controllerOwners.Active(state.ConversationID); ok {
		if owner.Controller == orchestrationcontroller.MinimalAudioClosure && owner.ControllerID == state.ClosureID {
			return nil
		}
		return fmt.Errorf("conversation is already owned by %s controller %s", owner.Controller, owner.ControllerID)
	}
	_, _, err := s.controllerOwners.Acquire(state.ConversationID, state.ClosureID, orchestrationcontroller.Decision{
		SchemaVersion: orchestrationcontroller.DecisionSchema, Controller: orchestrationcontroller.MinimalAudioClosure,
		TargetScope: audioClosureSemanticScope(state.Scope), Authorization: audioClosureAuthorization(state.Mode),
		SourceRoute: audioClosureSourceRoute(state.Mode), Reason: "persistent minimal audio closure",
	}, time.Now().UTC())
	return err
}

func audioClosureSemanticScope(scope audioclosure.Scope) string {
	if scope.Kind == "project" {
		return semanticEntryScopeProjectContext
	}
	return semanticEntryScopeCurrentSelection
}
func audioClosureAuthorization(mode audioclosure.Mode) string {
	if mode == audioclosure.ModeDiagnostic {
		return semanticEntryAuthorizationObserve
	}
	return semanticEntryAuthorizationAction
}
func audioClosureSourceRoute(mode audioclosure.Mode) string {
	if mode == audioclosure.ModeDiagnostic {
		return semanticEntryRouteObservation
	}
	return semanticEntryRouteOpenSemantic
}

func bindAudioClosureContext(requestContext map[string]any, state audioclosure.State) map[string]any {
	bound := map[string]any{audioClosureContextKey: audioClosureStateMap(state)}
	// The FS phase truth lives on the closure; expose it under the dedicated
	// context key so agentloop's phase-aware decision check consumes the same
	// source as minimal_audio_closure.phase.
	if phase, ok := audioclosure.ParsePhase(string(state.Phase)); ok {
		bound["free_state_phase"] = string(phase)
	}
	out := mergeContext(requestContext, bound)
	if _, ok := orchestrationControllerDecisionFromContext(out); !ok {
		decision := orchestrationcontroller.Decision{
			SchemaVersion: orchestrationcontroller.DecisionSchema, Controller: orchestrationcontroller.MinimalAudioClosure,
			TargetScope: audioClosureSemanticScope(state.Scope), Authorization: audioClosureAuthorization(state.Mode),
			SourceRoute: audioClosureSourceRoute(state.Mode), Reason: "restored minimal audio closure",
		}
		out[orchestrationDecisionContextKey] = orchestrationControllerDecisionMap(decision)
	}
	return out
}

func audioClosureStateMap(state audioclosure.State) map[string]any {
	raw, _ := json.Marshal(state)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func audioClosureMessageLoopBudget(base agentloop.Budget, context map[string]any) agentloop.Budget {
	// Candidate selection is the one closure phase that needs two adjacent
	// model turns: request the bounded target observation, then decide from
	// the returned target evidence. Keeping both turns in the same request
	// prevents the outer continuation/round budget from consuming the final
	// action-or-boundary decision slot.
	base.MaxTurns = 1
	frontier := firstMapFromAny(firstMapFromAny(context[audioClosureContextKey])["hypothesis_frontier"])
	if len(freeStateMapRows(frontier["candidates"])) > 0 && firstStringFromMap(frontier, "candidate_id") == "" {
		base.MaxTurns = 2
	}
	if base.MaxToolCalls <= 0 || base.MaxToolCalls > 6 {
		base.MaxToolCalls = 6
	}
	return base
}

func (s *Server) admitAudioClosureRound(state audioclosure.State) (audioclosure.State, bool, error) {
	if state.ClosureID == "" || s == nil || s.audioClosures == nil {
		return state, false, nil
	}
	current, ok := s.audioClosures.Load(state.ClosureID)
	if !ok {
		return state, false, fmt.Errorf("minimal audio closure %s is missing", state.ClosureID)
	}
	// A transient provider failure leaves the admitted closure round active so
	// the next invocation can retry the same semantic move. The round boundary
	// applies only after that round has completed; checking it first would turn
	// a provider retry at MaxClosureRounds into a fabricated terminal outcome.
	if current.RoundInProgress {
		return current, true, nil
	}
	if current.ContractID != "" && current.RoundsStarted >= current.Policy.MaxClosureRounds {
		// A governed action can complete on the final diagnostic round. The
		// post-action CCB observation is part of that same experiment contract,
		// so grant one durable verification round before applying the ordinary
		// closure boundary. Without this extension the old round limit settles
		// the task as capability_blocked immediately after a real mutation.
		if loop, loopOK := s.freeStateLoop(current.ConversationID); loopOK && loop.Experiment != nil && loop.RequiresPostActionObservation {
			next, err := (audioclosure.Driver{}).ExtendClosureRounds(current, current.Revision, current.RoundsStarted+1, time.Now().UTC())
			if err != nil {
				return current, false, err
			}
			current = next
			if err := s.audioClosures.Save(current, state.Revision); err != nil {
				return current, false, err
			}
			s.persistCurrentProjectWorkspace()
		} else {
			next, err := s.settleTaskAtAudioClosureBoundary(current, "closure observation round boundary reached")
			if err != nil {
				return current, false, err
			}
			if next.Revision != current.Revision {
				if err := s.audioClosures.Save(next, current.Revision); err != nil {
					return current, false, err
				}
				s.persistCurrentProjectWorkspace()
			}
			if next.Terminal() {
				s.settleAudioClosureOwner(next)
			}
			return next, false, nil
		}
	}
	next, admitted, err := (audioclosure.Driver{}).AdmitRound(current, current.Revision, time.Now().UTC())
	if err != nil {
		return current, false, err
	}
	if next.Revision != current.Revision {
		if err := s.audioClosures.Save(next, current.Revision); err != nil {
			return current, false, err
		}
		s.persistCurrentProjectWorkspace()
	}
	// Round boundary: enter/advance the FS phase machine from closure facts
	// (binding at FS1; later phases advance on the record boundary where the
	// capacity/observation evidence exists).
	next = s.advanceAudioClosurePhase(next, audioclosure.PhaseGuardEvidence{})
	if next.Terminal() {
		s.settleAudioClosureOwner(next)
	}
	return next, admitted, nil
}

func (s *Server) recordAudioClosureRound(state audioclosure.State, res agentloop.Result, requestContext map[string]any) (audioclosure.State, error) {
	if s == nil || s.audioClosures == nil || state.ClosureID == "" {
		return state, nil
	}
	current, ok := s.audioClosures.Load(state.ClosureID)
	if !ok {
		return state, fmt.Errorf("minimal audio closure %s is missing", state.ClosureID)
	}
	if current.Terminal() {
		return current, nil
	}
	driver := audioclosure.Driver{}
	if changeID := audioClosureAuthoritativeProjectChangeID(requestContext); changeID != "" && changeID != current.LastProjectChangeID {
		// A governed Apply closes the prior diagnostic round. Before recording
		// its authoritative project change, admit the dedicated post-action
		// verification round; otherwise RecordProjectChange correctly rejects
		// the transition as an unadmitted round.
		if loop, loopOK := s.freeStateLoop(current.ConversationID); loopOK && loop.RequiresPostActionObservation && !current.RoundInProgress {
			admitted, _, admitErr := driver.AdmitRound(current, current.Revision, time.Now().UTC())
			if admitErr != nil {
				return current, admitErr
			}
			current = admitted
		}
		next, _, err := driver.RecordProjectChange(current, current.Revision, changeID, time.Now().UTC())
		if err != nil {
			return current, err
		}
		current = next
	}
	if res.StopReason == agentloop.StopReasonTransientLLMError {
		// The result can still carry the previous durable free-state decision.
		// Do not project that stale decision into the frontier or no-progress
		// counters. Only an authoritative project change discovered above is
		// allowed to advance closure state on a provider retry.
		stored, _ := s.audioClosures.Load(state.ClosureID)
		if current.Revision != stored.Revision {
			if err := s.audioClosures.Save(current, stored.Revision); err != nil {
				return stored, err
			}
			s.persistCurrentProjectWorkspace()
		}
		return current, nil
	}
	observations := freeStateCCBObservations(res)
	postActionObservationRequired := false
	if loop, loopOK := s.freeStateLoop(current.ConversationID); loopOK {
		postActionObservationRequired = loop.RequiresPostActionObservation
	}
	// Scheduler continuations can carry the authoritative CCB bundles only in
	// the durable free-state ledger. Rehydrate those bundles before recording
	// the closure round so candidate/frontier state cannot regress to empty
	// merely because this Result has no Executed projection.
	if loop, loopOK := s.freeStateLoop(current.ConversationID); loopOK {
		observations = appendUniqueFreeStateObservations(observations, freeStateLedgerObservations(loop.ObservationLedger))
	}
	for _, observation := range observations {
		if observation == nil || current.Terminal() {
			continue
		}
		if postActionObservationRequired {
			observationRevision := firstNonEmpty(
				firstStringFromMap(observation.Summary, "project_revision"),
				firstStringFromMap(firstMapFromAny(observation.Summary["project_binding"]), "project_revision"),
				firstStringFromMap(firstMapFromAny(observation.Summary["freshness"]), "project_revision"),
			)
			if observationRevision == "" || !strings.EqualFold(observationRevision, current.ProjectRevision) {
				// Replayed pre-action evidence must not close a post-action round.
				continue
			}
		}
		observationID := firstStringFromMap(observation.Summary, "observation_id")
		// One CCB bundle can be projected through several compact view sets as
		// continuations merge their durable ledgers. The closure budget is for
		// unique observation bundles, not repeated projections of the same
		// observation_id. Replaying those projections must not consume the
		// evidence ceiling before the next genuinely new observation arrives.
		if audioClosureHasObservationID(current, observationID) {
			continue
		}
		if current.ContractID != "" && len(current.Observations) >= current.Policy.MaxUniqueObservations {
			next, boundaryErr := s.settleTaskAtAudioClosureBoundary(current, "closure evidence ceiling reached")
			if boundaryErr != nil {
				return current, boundaryErr
			}
			current = next
			break
		}
		key := audioClosureObservationKey(current, observation, requestContext)
		outcome, err := driver.RecordObservation(current, current.Revision, key, observationID, time.Now().UTC())
		if err != nil {
			return current, err
		}
		current = outcome.State
	}
	if !current.Terminal() && res.FreeStateDecision != nil {
		frontier, actionability := audioClosureFrontier(current.Frontier, *res.FreeStateDecision, observations)
		next, _, err := driver.UpdateFrontier(current, current.Revision, frontier, actionability, time.Now().UTC())
		if err != nil {
			return current, err
		}
		current = next
	}
	if !current.Terminal() && current.ContractID != "" {
		next, err := s.projectAudioClosureTaskState(current)
		if err != nil {
			return current, err
		}
		current = next
	}
	repairCount, plannerError := audioClosureProtocolTrace(res)
	for repair := 0; !current.Terminal() && repair < repairCount; repair++ {
		next, _, err := driver.RecordProtocolRepair(current, current.Revision, time.Now().UTC())
		if err != nil {
			return current, err
		}
		current = next
	}
	if !current.Terminal() && (plannerError || (repairCount > 0 && res.NeedsClarification)) {
		var next audioclosure.State
		var err error
		if current.ContractID != "" {
			_, err = s.transitionTaskSemantic(current.GoalID, taskstate.TransitionRequest{Event: taskstate.EventTaskFailed,
				Reason: "model protocol failure", Summary: "MessageLoop did not produce a valid closure move after its protocol repair", ProjectRevision: current.ProjectRevision})
			if err == nil {
				next, err = s.projectAudioClosureTaskState(current)
			}
			if err == nil {
				next = audioClosureSettleFromResult(driver, next, res)
			}
		} else {
			next, err = driver.Settle(current, current.Revision, audioclosure.StopModelProtocolFailure,
				"MessageLoop did not produce a valid closure move after its protocol repair", false, time.Now().UTC())
		}
		if err != nil {
			return current, err
		}
		current = next
	}
	if !current.Terminal() {
		current = audioClosureSettleFromResult(driver, current, res)
	}
	if !current.Terminal() && current.RoundInProgress {
		// Round close boundary: advance the FS spine from the evidence this
		// round produced, then persist the diagnostic round record (the G4
		// data source) before closing the round.
		evidence := audioclosure.PhaseGuardEvidence{
			CapacityAssessed: s.audioClosureCapacityAssessedForConversation(requestContext, current.ConversationID),
		}
		advanced, advanceErr := advancePhaseState(current, evidence)
		if advanceErr != nil && s.logger != nil {
			s.logger.Warn("[audio-closure] phase advance failed for %s at %s: %v", current.ClosureID, current.Phase, advanceErr)
		}
		current = advanced
		if record, ok := audioClosureRoundRecord(current); ok {
			if recorded, err := driver.RecordDiagnosticRound(current, current.Revision, record, time.Now().UTC()); err == nil {
				current = recorded
			}
		}
		next, err := driver.CompleteRound(current, current.Revision, time.Now().UTC())
		if err != nil {
			return current, err
		}
		current = next
	}
	if !current.Terminal() && current.ContractID != "" && current.NoProgressStreak >= current.Policy.MaxNoProgressRounds &&
		(current.Scope.Kind != "project" || len(current.Frontier.Candidates) > 0) {
		next, err := s.settleTaskAtAudioClosureBoundary(current, "closure made no material progress within its bounded observation window")
		if err != nil {
			return current, err
		}
		current = next
	}
	stored, _ := s.audioClosures.Load(state.ClosureID)
	if current.Revision != stored.Revision {
		if err := s.audioClosures.Save(current, stored.Revision); err != nil {
			return stored, err
		}
		s.persistCurrentProjectWorkspace()
	}
	// Mirror the (possibly advanced) spine into the loop snapshot so the
	// scheduler-driven progression is observable at the runtime interface.
	s.syncFreeStateSpine(current)
	if current.Terminal() {
		s.settleAudioClosureOwner(current)
	}
	return current, nil
}

func audioClosureHasObservationID(state audioclosure.State, observationID string) bool {
	observationID = strings.TrimSpace(observationID)
	if observationID == "" {
		return false
	}
	for _, record := range state.Observations {
		if strings.TrimSpace(record.ObservationID) == observationID {
			return true
		}
	}
	return false
}

func (s *Server) settleTaskAtAudioClosureBoundary(state audioclosure.State, reason string) (audioclosure.State, error) {
	if state.ContractID == "" {
		return state, nil
	}
	// A free-state capacity route with an open diagnostic queue is a bounded
	// continuation point, not a capability terminal.  In particular, the
	// legacy no-pending-mix-tick boundary must not collapse FS2/FS3 into
	// capability_blocked while the runtime still owns free-state observation.
	queueOpenWithinCapacity := s.freeStateQueueStillOpenWithinCapacity(state.ConversationID)
	frontierOpen := len(state.Frontier.Candidates) > 0
	if queueOpenWithinCapacity && !frontierOpen {
		// An open queue is normally a continuation point. Once the explicit
		// continuation budget is exhausted, the open-intent contract requires a
		// bounded no-candidate conclusion rather than an empty non-terminal
		// response or a fabricated capability boundary.
		loop, ok := s.freeStateLoop(state.ConversationID)
		if !ok || !freeStateContinuationBudgetExhausted(loop) {
			return state, nil
		}
	}
	stopReason := audioclosure.StopCapabilityBlocked
	if queueOpenWithinCapacity && !frontierOpen {
		stopReason = audioclosure.StopNoCandidateFound
	}
	if audioclosure.IsFSPhase(state.Phase) && state.Phase != audioclosure.PhaseFS9Terminal {
		terminal, err := (audioclosure.Driver{}).TransitionPhase(state, state.Revision, audioclosure.PhaseFS9Terminal,
			audioclosure.PhaseGuardInput{TerminalStopReason: string(stopReason)},
			"explicit capability boundary", time.Now().UTC())
		if err != nil {
			return state, err
		}
		state = terminal
	}
	goal := s.harness.RuntimeStatus(state.GoalID)
	if goal.Task == nil || goal.Task.SemanticState == nil {
		return state, fmt.Errorf("closure boundary has no canonical task state")
	}
	evidence := make([]string, 0, len(state.ObservationOrder))
	for _, fingerprint := range state.ObservationOrder {
		record := state.Observations[fingerprint]
		evidence = append(evidence, firstNonEmpty(record.ObservationID, record.Fingerprint))
	}
	// A round/evidence boundary proves only that the current capability window
	// cannot continue. no_candidate_found is a model-authored, validator-backed
	// semantic decision applied by applyFreeStateDecisionSemantic; absence of a
	// projected frontier is not evidence that no candidate exists.
	event := taskstate.EventCapabilityBlocked
	if queueOpenWithinCapacity && !frontierOpen {
		event = taskstate.EventNoCandidateReported
	}
	if _, err := s.transitionTaskSemantic(state.GoalID, taskstate.TransitionRequest{
		Event: event, Reason: reason, Summary: reason, EvidenceRefs: evidence, ProjectRevision: state.ProjectRevision,
	}); err != nil {
		return state, err
	}
	next, err := s.projectAudioClosureTaskState(state)
	if err != nil {
		return state, err
	}
	next = audioClosureSettleFromResult(audioclosure.Driver{}, next, agentloop.Result{Reply: reason, StopReason: string(stopReason)})
	if !next.Terminal() {
		return state, fmt.Errorf("canonical closure boundary did not produce a terminal projection")
	}
	return next, nil
}

func (s *Server) freeStateQueueStillOpenWithinCapacity(conversationID string) bool {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var route CapabilityRouteRecord
	for _, candidate := range s.capabilityRoutes {
		if candidate.ConversationID == conversationID &&
			(candidate.UpdatedAt.After(route.UpdatedAt) || route.SchemaVersion == "") {
			route = candidate
		}
	}
	if route.Assessment == nil ||
		!strings.EqualFold(strings.TrimSpace(route.Assessment.CapacityLevel), "within_free_state") ||
		!strings.EqualFold(strings.TrimSpace(route.Assessment.SelectedCapability), "free_state") {
		return false
	}
	loop, ok := s.freeStateLoops[conversationID]
	return ok && loop.PriorityQueue != nil && loop.PriorityQueue.HasOpen()
}

func audioClosureAuthoritativeProjectChangeID(requestContext map[string]any) string {
	if len(requestContext) == 0 {
		return ""
	}
	changes := []map[string]any{
		firstMapFromAny(requestContext["free_state_project_change"]),
		firstMapFromAny(requestContext["latest_project_change"]),
	}
	if loop := firstMapFromAny(requestContext["free_state_reasoning_loop"]); len(loop) > 0 {
		changes = append(changes, firstMapFromAny(loop["latest_project_change"]))
	}
	for _, change := range changes {
		if !strings.EqualFold(firstStringFromMap(change, "freshness"), "current_snapshot") {
			continue
		}
		if changeID := firstStringFromMap(change, "change_id"); changeID != "" {
			return changeID
		}
	}
	return ""
}

func audioClosureObservationKey(state audioclosure.State, observation *agentloop.RecentObservation, requestContext map[string]any) audioclosure.ObservationKey {
	summary := observation.Summary
	viewIDs := freeStateStringSlice(firstNonNil(summary["view_ids"], summary["admitted_views"], summary["included_views"], summary["requested_views"]))
	if len(viewIDs) == 0 {
		viewIDs = []string{firstNonEmpty(observation.Tool, observation.CommandName, "ccb_observation")}
	}
	targetRef := firstNonEmpty(firstStringFromMap(firstMapFromAny(summary["target_ref"]), "id", "target_id", "track_id"), state.Scope.ID)
	return audioclosure.ObservationKey{
		ProjectUUID: state.ProjectUUID,
		ProjectRevision: firstNonEmpty(
			firstStringFromMap(summary, "project_revision", "project_state_revision", "state_token"),
			firstStringFromMap(firstMapFromAny(summary["project_binding"]), "project_revision"),
			state.ProjectRevision),
		Scope: state.Scope, TargetRef: targetRef, ViewIDs: viewIDs,
		ObservationMode: firstNonEmpty(firstStringFromMap(summary, "observation_mode", "mode"), observation.Tool, observation.CommandName),
		Tap:             firstNonEmpty(firstStringFromMap(summary, "tap", "tap_point", "measurement_tap"), firstStringFromMap(requestContext, "tap", "observation_tap")),
		TimeWindow:      firstNonEmpty(firstStringFromMap(summary, "time_window", "window_ref", "range_ref"), firstStringFromMap(requestContext, "time_window", "observation_window")),
	}
}

func audioClosureFrontier(existing audioclosure.HypothesisFrontier, decision agentloop.FreeStateDecision, observations []*agentloop.RecentObservation) (audioclosure.HypothesisFrontier, audioclosure.Actionability) {
	status := strings.ToLower(strings.TrimSpace(decision.Status))
	frontier := existing
	frontier.Candidates = audioClosureCandidates(existing.Candidates, observations)
	if selected := audioClosureSelectedCandidate(frontier.Candidates, observations); selected != "" {
		frontier.CandidateID = selected
	}
	hypotheses := append([]string(nil), frontier.HypothesisIDs...)
	for _, candidate := range frontier.Candidates {
		hypotheses = append(hypotheses, "candidate:"+candidate.ID)
	}
	if processor := strings.ToLower(strings.TrimSpace(decision.ProcessorType)); processor != "" {
		hypotheses = append(hypotheses, "processor:"+processor)
	}
	frontier.HypothesisIDs = hypotheses
	frontier.Blockers = append([]string(nil), decision.Limitations...)
	actionability := audioclosure.ActionabilityUnknown
	if status == agentloop.FreeStateNeedsAction || status == agentloop.FreeStateNeedsExperiment {
		actionability = audioclosure.ActionabilityActionable
		if frontier.CandidateID == "" {
			frontier.CandidateID = firstNonEmpty(decision.ObservationID, "actionable:"+strings.ToLower(strings.TrimSpace(decision.ProcessorType)))
		}
	} else if status == agentloop.FreeStateSatisfied || status == agentloop.FreeStateBlocked {
		actionability = audioclosure.ActionabilityNonActionable
	}
	return frontier, actionability
}

func audioClosureCandidates(existing []audioclosure.Candidate, observations []*agentloop.RecentObservation) []audioclosure.Candidate {
	byID := make(map[string]audioclosure.Candidate, len(existing))
	for _, candidate := range existing {
		if strings.TrimSpace(candidate.ID) != "" {
			byID[candidate.ID] = candidate
		}
	}
	for _, observation := range observations {
		if !freeStateUsableObservation(observation) {
			continue
		}
		observationID := firstStringFromMap(observation.Summary, "observation_id")
		for _, viewID := range audioClosureCandidateViewIDs(observation.Summary) {
			rows := audioClosureCandidateRows(observation.Summary, viewID)
			for _, row := range rows {
				trackIDs, trackNames := audioClosureCandidateTracks(row)
				if len(trackIDs) == 0 {
					continue
				}
				issueType := firstStringFromMap(row, "type", "issue_type", "status")
				region := firstStringFromMap(row, "region", "band")
				candidateID := audioClosureCandidateID(observationID, viewID, issueType, region, trackIDs)
				evidenceRefs := freeStateStringSlice(firstNonNil(row["evidence_refs"], row["evidence_ref"], observation.Summary["evidence_refs"]))
				byID[candidateID] = audioclosure.Candidate{
					ID: candidateID, SourceObservationID: observationID, ViewID: viewID,
					IssueType: issueType, Region: region, TrackIDs: trackIDs, TrackNames: trackNames,
					EvidenceRefs: evidenceRefs,
				}
			}
		}
	}
	out := make([]audioclosure.Candidate, 0, len(byID))
	for _, candidate := range byID {
		out = append(out, candidate)
	}
	return out
}

// audioClosureCandidateViewIDs accepts both the compact CCB bundle emitted by
// the live loop and the durable full observation package written by the
// harness. The latter intentionally omits requested_views/views, so its MOM
// and project-package candidate projections are the source of truth.
func audioClosureCandidateViewIDs(summary map[string]any) []string {
	ids := freeStateNormalizedViewIDs(freeStateStringSlice(summary["requested_views"]))
	if len(ids) > 0 {
		return ids
	}
	if relation := firstMapFromAny(firstMapFromAny(summary["mom_projection"])["multitrack_relation"]); len(freeStateMapRows(relation["band_conflict_candidates"])) > 0 {
		return []string{"mix.multitrack_relationship"}
	}
	if len(firstMapFromAny(summary["project_package"])) > 0 {
		return []string{"mix.frequency_relationship"}
	}
	return nil
}

func audioClosureCandidateRows(summary map[string]any, viewID string) []map[string]any {
	conclusion := contextruntime.ProjectCCBViewConclusion(summary, viewID, contextruntime.Options{
		MaxTextRunes: 900, MaxListItems: 8, MaxPreviewBytes: 6 * 1024, SkipPluginSemanticLoad: true,
	})
	facts := firstMapFromAny(conclusion["facts"])
	rows := append(freeStateMapRows(facts["conflict_candidates"]), freeStateMapRows(facts["band_conflict_candidates"])...)
	if len(rows) > 0 {
		return rows
	}
	// Durable continuation compaction stores the authoritative conclusion
	// directly under views[view_id].facts. Read that structured projection
	// before falling back to full acoustic-package shapes.
	view := firstMapFromAny(firstMapFromAny(summary["views"])[viewID])
	directFacts := firstMapFromAny(view["facts"])
	rows = append(freeStateMapRows(directFacts["conflict_candidates"]), freeStateMapRows(directFacts["band_conflict_candidates"])...)
	if len(rows) > 0 {
		return rows
	}
	// Persisted observations are full acoustic packages rather than CCB view
	// envelopes. Read only the already-produced candidate rows; no target,
	// processor, or parameter is introduced here.
	switch viewID {
	case "mix.multitrack_relationship":
		relation := firstMapFromAny(firstMapFromAny(summary["mom_projection"])["multitrack_relation"])
		return freeStateMapRows(relation["band_conflict_candidates"])
	case "mix.frequency_relationship":
		conflicts := firstMapFromAny(firstMapFromAny(summary["project_package"])["conflict_candidates"])
		if rows := freeStateMapRows(conflicts["candidates"]); len(rows) > 0 {
			return rows
		}
		return freeStateMapRows(firstMapFromAny(summary["project_package"])["conflict_candidates"])
	default:
		return nil
	}
}

func audioClosureCandidateTracks(row map[string]any) ([]string, []string) {
	ids := freeStateNormalizedViewIDs(freeStateStringSlice(row["track_ids"]))
	names := []string{}
	if id := firstStringFromMap(row, "track_id", "id"); id != "" {
		ids = freeStateNormalizedViewIDs(append(ids, id))
	}
	for _, track := range freeStateMapRows(row["tracks"]) {
		if id := firstStringFromMap(track, "track_id", "id"); id != "" {
			ids = freeStateNormalizedViewIDs(append(ids, id))
		}
		if name := firstStringFromMap(track, "track_name", "name", "label"); name != "" {
			names = append(names, name)
		}
	}
	return ids, freeStateNormalizedViewIDs(names)
}

func audioClosureCandidateID(observationID, viewID, issueType, region string, trackIDs []string) string {
	digest := sha256.Sum256([]byte(strings.Join(append([]string{observationID, viewID, issueType, region}, freeStateNormalizedViewIDs(trackIDs)...), "\x1f")))
	return "candidate:" + hex.EncodeToString(digest[:8])
}

func audioClosureSelectedCandidate(candidates []audioclosure.Candidate, observations []*agentloop.RecentObservation) string {
	for _, observation := range observations {
		if !freeStateUsableObservation(observation) {
			continue
		}
		target := firstMapFromAny(observation.Summary["target_ref"])
		if !strings.EqualFold(firstStringFromMap(target, "kind", "target_kind"), "track") {
			continue
		}
		trackID := firstStringFromMap(target, "id", "track_id", "target_id")
		for _, candidate := range candidates {
			if freeStateContainsString(candidate.TrackIDs, trackID) {
				return candidate.ID
			}
		}
	}
	return ""
}

func audioClosureProtocolTrace(res agentloop.Result) (repairs int, plannerError bool) {
	if res.ModelProtocolRepairs > 0 || res.ModelProtocolFailure || res.StopReason == agentloop.StopReasonModelProtocolFailure {
		return res.ModelProtocolRepairs, res.ModelProtocolFailure || res.StopReason == agentloop.StopReasonModelProtocolFailure
	}
	// Compatibility for continuations created before typed protocol metadata.
	pendingError := false
	for _, event := range res.Trace {
		switch strings.ToLower(strings.TrimSpace(event.Kind)) {
		case "planner_repair":
			repairs++
			pendingError = false
		case "planner_error":
			pendingError = true
		}
	}
	return repairs, pendingError
}

func audioClosureSettleFromResult(driver audioclosure.Driver, state audioclosure.State, res agentloop.Result) audioclosure.State {
	reason := audioclosure.StopReason("")
	summary := firstNonEmpty(res.FailureReason, res.Error, res.Reply, res.StopReason)
	needsClarification := false
	if state.ContractID != "" {
		switch state.TaskState {
		case taskstate.StateNoCandidateFound:
			reason = audioclosure.StopNoCandidateFound
		case taskstate.StateCapabilityBlocked:
			reason = audioclosure.StopCapabilityBlocked
		case taskstate.StateSettled:
			reason = audioclosure.StopTaskSettled
		case taskstate.StateCancelled:
			reason = audioclosure.StopCancelled
		case taskstate.StateFailed:
			reason = audioclosure.StopTaskFailed
		default:
			return state
		}
		settled, err := driver.Settle(state, state.Revision, reason, summary, false, time.Now().UTC())
		if err != nil {
			return state
		}
		return settled
	}
	if res.NeedsClarification {
		reason, needsClarification = audioclosure.StopUserChoiceRequired, true
	} else if state.Mode == audioclosure.ModeDiagnostic && res.Status == agentruntime.StatusCompleted && res.FreeStateDecision == nil {
		reason = audioclosure.StopDiagnosticComplete
	} else if res.FreeStateDecision != nil {
		switch strings.ToLower(strings.TrimSpace(res.FreeStateDecision.Status)) {
		case agentloop.FreeStateSatisfied:
			if state.Mode == audioclosure.ModeDiagnostic {
				reason = audioclosure.StopDiagnosticComplete
			} else {
				reason = audioclosure.StopSatisfied
			}
		case agentloop.FreeStateBlocked:
			reason = audioClosureBlockedReason(*res.FreeStateDecision)
			summary = firstNonEmpty(res.FreeStateDecision.Summary, res.FreeStateDecision.StopReason, summary)
		}
	}
	if reason == "" {
		return state
	}
	settled, err := driver.Settle(state, state.Revision, reason, summary, needsClarification, time.Now().UTC())
	if err != nil {
		return state
	}
	return settled
}

func audioClosureBlockedReason(decision agentloop.FreeStateDecision) audioclosure.StopReason {
	text := strings.ToLower(strings.Join(append(append([]string{}, decision.StopReason, decision.Summary), decision.Limitations...), " "))
	switch {
	case strings.Contains(text, "pca"):
		return audioclosure.StopPCAUnavailable
	case strings.Contains(text, "capability") || strings.Contains(text, "adapter"):
		return audioclosure.StopCapabilityUnavailable
	case strings.Contains(text, "revision") || strings.Contains(text, "stale"):
		return audioclosure.StopProjectRevisionStale
	default:
		return audioclosure.StopInsufficientEvidence
	}
}

func (s *Server) settleAudioClosureOwner(state audioclosure.State) {
	if s == nil || s.controllerOwners == nil || !state.Terminal() {
		return
	}
	owner, ok := s.controllerOwners.Active(state.ConversationID)
	if !ok || owner.ControllerID != state.ClosureID {
		return
	}
	_, _ = s.controllerOwners.Settle(state.ConversationID, state.ClosureID, owner.Revision, string(state.Settlement.Reason), time.Now().UTC())
}

func (s *Server) audioClosureResponse(conversationID, mode string, state audioclosure.State, base agentloop.Result) ChatResponse {
	if !state.Terminal() {
		resp := s.chatResponseFromAgentLoopResult(conversationID, mode, base)
		return bindAudioClosureToResponse(resp, state)
	}
	if state.GoalID != "" {
		s.clearGoalContinuation(state.GoalID)
	} else if base.GoalID != "" {
		s.clearGoalContinuation(base.GoalID)
	}
	s.settleFreeStateLoopFromClosure(conversationID, *state.Settlement, state)
	// The closure is the phase host. Mirror its terminal FS9 state before the
	// response is projected so the durable loop and runtime row cannot retain
	// the pre-settlement FS2/FS4 phase.
	s.syncFreeStateSpine(state)
	admissionReceipt := map[string]any{}
	if loop, ok := s.freeStateLoop(conversationID); ok {
		admissionReceipt = cloneContext(loop.AdmissionReceipt)
	}
	reply := audioClosureSettlementReply(state.Settlement)
	status := agentruntime.StatusCompleted
	if state.Settlement.NeedsUserClarification {
		status = agentruntime.StatusWaitingClarification
	}
	if state.Settlement.Reason == audioclosure.StopTransportFailure || state.Settlement.Reason == audioclosure.StopModelProtocolFailure || state.Settlement.Reason == audioclosure.StopActionFailed || state.Settlement.Reason == audioclosure.StopTaskFailed {
		status = agentruntime.StatusFailed
	}
	resp := ChatResponse{
		ConversationID: conversationID, TaskID: base.TaskID, GoalID: firstNonEmpty(base.GoalID, state.GoalID), RunID: firstNonEmpty(base.RunID, state.RunID),
		SliceID: base.SliceID, TurnID: base.TurnID, OriginalIntent: firstNonEmpty(base.OriginalIntent, state.OriginalIntent),
		AgentMode: mode, Reply: reply, GoalStatus: string(status), StopReason: string(state.Settlement.Reason), Workflow: "minimal_audio_closure",
		WorkflowData: map[string]any{"schema_version": audioclosure.SchemaVersion, "status": "settled", "settlement": state.Settlement, "minimal_audio_closure": audioClosureStateMap(state), "free_state_admission_receipt": admissionReceipt, "mutation_performed": false},
	}
	return resp
}

func bindAudioClosureToResponse(resp ChatResponse, state audioclosure.State) ChatResponse {
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData[audioClosureContextKey] = audioClosureStateMap(state)
	requestContext := firstMapFromAny(resp.WorkflowData["request_context"])
	requestContext = bindAudioClosureContext(requestContext, state)
	resp.WorkflowData["request_context"] = requestContext
	return resp
}

func (s *Server) bindAudioClosureCapabilityHandoff(resp ChatResponse, state audioclosure.State) (ChatResponse, audioclosure.State) {
	if s == nil || s.audioClosures == nil || state.Terminal() {
		return bindAudioClosureToResponse(resp, state), state
	}
	current, ok := s.audioClosures.Load(state.ClosureID)
	if !ok || current.Terminal() {
		return bindAudioClosureToResponse(resp, state), state
	}
	driver := audioclosure.Driver{}
	sessionID := firstStringFromMap(resp.WorkflowData, "session_id", "capability_session_id")
	capabilityID := firstStringFromMap(resp.WorkflowData, "capability_id")
	if sessionID != "" && capabilityID != "" {
		actionID := firstNonEmpty(firstStringFromMap(resp.WorkflowData, "action_id", "proposal_id", "candidate_id"), resp.PlanID, sessionID+":action:1")
		next, _, err := driver.BeginCapability(current, current.Revision, audioclosure.CapabilityLink{
			SessionID: sessionID, CapabilityID: capabilityID, ActionID: actionID,
			ExpectedProjectRevision: current.ProjectRevision,
		}, time.Now().UTC())
		if err == nil && next.Revision != current.Revision {
			if saveErr := s.audioClosures.Save(next, current.Revision); saveErr == nil {
				current = next
				s.persistCurrentProjectWorkspace()
				if s.orchestrationRuntime != nil && current.ActiveCapability != nil {
					_, linkErr := s.orchestrationRuntime.AttachParentController(sessionID, orchestration.ParentControllerLink{
						SchemaVersion: orchestration.ParentControllerLinkSchema,
						ControllerID:  current.ClosureID, ControllerType: string(orchestrationcontroller.MinimalAudioClosure),
						ClosureID: current.ClosureID, ClosureRevision: current.ActiveCapability.ClosureRevision,
						ActionID: current.ActiveCapability.ActionID, ExpectedProjectRevision: current.ProjectRevision,
					})
					if linkErr != nil {
						failed, _, settleErr := driver.SettleCapability(current, current.Revision, audioclosure.CapabilitySettlement{
							SessionID: sessionID, ActionID: current.ActiveCapability.ActionID, Status: "failed", Reason: "parent link failed: " + linkErr.Error(),
						}, time.Now().UTC())
						if settleErr == nil && s.audioClosures.Save(failed, current.Revision) == nil {
							current = failed
							s.settleAudioClosureOwner(current)
							s.persistCurrentProjectWorkspace()
						}
						resp.Error = "capability parent link failed: " + linkErr.Error()
						resp.StopReason = string(audioclosure.StopCapabilityUnavailable)
						resp.GoalStatus = string(agentruntime.StatusFailed)
					}
				}
			}
		}
	} else if resp.NeedsConfirmation || strings.Contains(strings.ToLower(resp.GoalStatus), "waiting") {
		// Native typed-tool confirmations remain under this closure. Settling
		// before their receipt returns would restart the next model turn with a
		// new closure, losing the accumulated revision/delta and observation
		// ledger needed for post-action evaluation.
	} else if resp.Error != "" || strings.EqualFold(firstStringFromMap(resp.WorkflowData, "status"), "unavailable") || strings.EqualFold(firstStringFromMap(resp.WorkflowData, "status"), "rejected") {
		reason := audioclosure.StopCapabilityUnavailable
		if strings.Contains(strings.ToLower(firstNonEmpty(resp.Error, resp.StopReason, resp.Reply)), "pca") {
			reason = audioclosure.StopPCAUnavailable
		}
		var next audioclosure.State
		var err error
		if current.ContractID != "" {
			_, err = s.transitionTaskSemantic(current.GoalID, taskstate.TransitionRequest{Event: taskstate.EventCapabilityBlocked,
				Reason: firstNonEmpty(resp.StopReason, "governed capability unavailable"), Summary: firstNonEmpty(resp.Error, resp.Reply), ProjectRevision: current.ProjectRevision})
			if err == nil {
				next, err = s.projectAudioClosureTaskState(current)
			}
			if err == nil {
				next = audioClosureSettleFromResult(driver, next, agentloop.Result{Reply: firstNonEmpty(resp.Error, resp.Reply)})
			}
		} else {
			next, err = driver.Settle(current, current.Revision, reason, firstNonEmpty(resp.Error, resp.Reply), false, time.Now().UTC())
		}
		if err == nil {
			if saveErr := s.audioClosures.Save(next, current.Revision); saveErr == nil {
				current = next
				s.settleAudioClosureOwner(current)
				s.persistCurrentProjectWorkspace()
			}
		}
	}
	if current.Terminal() && resp.StopReason == "" {
		resp.StopReason = string(current.Settlement.Reason)
	}
	return bindAudioClosureToResponse(resp, current), current
}

// recordAudioClosureCapabilityResponse converts the governed child transaction
// result into one exactly-once parent receipt. A completed child moves the
// closure to verification; it does not itself claim acoustic satisfaction.
func (s *Server) recordAudioClosureCapabilityResponse(conversationID string, resp ChatResponse) (ChatResponse, audioclosure.State, bool) {
	if s == nil || s.audioClosures == nil {
		return resp, audioclosure.State{}, false
	}
	state, ok := s.audioClosures.ActiveForConversation(conversationID)
	if !ok || state.ActiveCapability == nil {
		return resp, state, ok
	}
	storedRevision := state.Revision
	if resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) || resp.GoalStatus == string(agentruntime.StatusWaitingContinue) {
		return bindAudioClosureToResponse(resp, state), state, true
	}
	status := "failed"
	switch resp.GoalStatus {
	case string(agentruntime.StatusCompleted):
		status = "completed"
	case string(agentruntime.StatusCancelled):
		status = "cancelled"
	}
	if strings.EqualFold(firstStringFromMap(resp.WorkflowData, "status"), "stale") {
		status = "stale"
	}
	link := state.ActiveCapability
	if state.ContractID != "" && status != "completed" && status != "needs_review" {
		event := taskstate.EventCapabilityBlocked
		if status == "cancelled" {
			event = taskstate.EventTaskCancelled
		}
		if _, transitionErr := s.transitionTaskSemantic(state.GoalID, taskstate.TransitionRequest{Event: event,
			Reason: firstNonEmpty(resp.StopReason, "capability did not complete"), Summary: firstNonEmpty(resp.Error, resp.Reply), ProjectRevision: state.ProjectRevision}); transitionErr != nil {
			resp.Error = firstNonEmpty(resp.Error, transitionErr.Error())
			return bindAudioClosureToResponse(resp, state), state, true
		}
		projected, projectionErr := s.projectAudioClosureTaskState(state)
		if projectionErr != nil {
			resp.Error = firstNonEmpty(resp.Error, projectionErr.Error())
			return bindAudioClosureToResponse(resp, state), state, true
		}
		state = projected
	}
	next, _, err := (audioclosure.Driver{}).SettleCapability(state, state.Revision, audioclosure.CapabilitySettlement{
		SessionID: link.SessionID, ActionID: link.ActionID, Status: status,
		Reason: firstNonEmpty(resp.Error, resp.StopReason),
	}, time.Now().UTC())
	if err != nil {
		return bindAudioClosureToResponse(resp, state), state, true
	}
	if saveErr := s.audioClosures.Save(next, storedRevision); saveErr != nil {
		return bindAudioClosureToResponse(resp, state), state, true
	}
	state = next
	if state.Terminal() {
		s.settleAudioClosureOwner(state)
	}
	s.persistCurrentProjectWorkspace()
	return bindAudioClosureToResponse(resp, state), state, true
}

func audioClosureControllerErrorResponse(conversationID, mode string, err error) ChatResponse {
	message := "minimal audio closure controller failed"
	if err != nil {
		message += ": " + err.Error()
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode,
		Reply:      "声学闭环控制器无法建立一致的持久状态，因此没有继续观察或修改工程。",
		GoalStatus: string(agentruntime.StatusFailed), StopReason: "audio_closure_controller_failure", Error: message,
		Workflow: "minimal_audio_closure", WorkflowData: map[string]any{"schema_version": audioclosure.SchemaVersion, "status": "controller_failure", "mutation_performed": false},
	}
}

func audioClosureSettlementReply(settlement *audioclosure.Settlement) string {
	if settlement == nil {
		return "本次声学闭环已结束。"
	}
	if settlement.NeedsUserClarification && strings.TrimSpace(settlement.Summary) != "" {
		return settlement.Summary
	}
	switch settlement.Reason {
	case audioclosure.StopSatisfied:
		return "本次声学问题已经完成闭环并通过结论检查。"
	case audioclosure.StopDiagnosticComplete:
		return "本次只读声学诊断已经完成；没有修改工程。"
	case audioclosure.StopNoCandidateFound:
		return "在已声明的观察范围内没有发现可信改善候选；结论保留证据引用和未覆盖边界。"
	case audioclosure.StopCapabilityBlocked:
		return "任务已到达明确的能力边界；没有把能力不足解释为改善完成。"
	case audioclosure.StopTaskSettled:
		return "任务已经满足其持久化契约中的证据与结算条件。"
	case audioclosure.StopTaskFailed:
		return "任务在形成有效结算前失败；失败原因与已有证据已保留。"
	case audioclosure.StopEvidenceCeilingReached:
		return "已达到本次闭环的唯一观察上限，现有证据仍不足以支持可靠动作；没有修改工程。"
	case audioclosure.StopNoProgress:
		return "连续两轮没有获得新的有效证据或缩小判断范围，本次闭环已停止；没有修改工程。"
	case audioclosure.StopRoundLimit:
		return "本次声学闭环已达到六轮上限，未继续重复观察或自动执行。"
	case audioclosure.StopModelProtocolFailure:
		return "模型在一次格式修复后仍未给出合法的闭环决策，本次任务已按协议失败结算；无需重述原任务。"
	case audioclosure.StopTransportFailure:
		return "模型传输链路失败，本次闭环已按基础设施故障结算；没有修改工程。"
	default:
		if strings.TrimSpace(settlement.Summary) != "" {
			return settlement.Summary
		}
		return "本次声学闭环已结束；没有启动额外观察或工程修改。"
	}
}

// audioClosureCapacityAssessed derives the FS2 guard from the capacity
// routing assessment: the request context first, then the server's durable
// capability route record (scheduler-driven turns do not carry the request
// context of the original HTTP turn).
func audioClosureCapacityAssessed(requestContext map[string]any) bool {
	assessment := firstMapFromAny(requestContext["free_state_capacity_assessment"])
	if len(assessment) == 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(firstStringFromMap(assessment, "capacity_level")), "exceeds_free_state") {
		return false
	}
	return strings.TrimSpace(firstStringFromMap(assessment, "selected_capability")) != ""
}

func (s *Server) audioClosureCapacityAssessedForConversation(requestContext map[string]any, conversationID string) bool {
	if audioClosureCapacityAssessed(requestContext) {
		return true
	}
	if s == nil {
		return false
	}
	s.mu.Lock()
	var record CapabilityRouteRecord
	for _, candidate := range s.capabilityRoutes {
		if candidate.ConversationID == conversationID &&
			(candidate.UpdatedAt.After(record.UpdatedAt) || record.SchemaVersion == "") {
			record = candidate
		}
	}
	s.mu.Unlock()
	if record.SchemaVersion == "" || record.Assessment == nil {
		return false
	}
	assessment := record.Assessment
	if strings.EqualFold(strings.TrimSpace(assessment.CapacityLevel), "exceeds_free_state") {
		return false
	}
	return strings.TrimSpace(assessment.SelectedCapability) != ""
}

// advancePhaseState computes the FS spine advancement without persisting:
// callers that accumulate further events before their own save must use this
// variant to avoid a revision conflict against the store.
func advancePhaseState(current audioclosure.State, evidence audioclosure.PhaseGuardEvidence) (audioclosure.State, error) {
	next, err := (audioclosure.Driver{}).AdvancePhase(current, current.Revision, evidence, time.Now().UTC())
	if err != nil {
		return current, err
	}
	return next, nil
}

// advanceAudioClosurePhase drives the FS phase machine at closure round
// boundaries and persists the result. Guard inputs are derived from the
// closure State itself plus the non-State evidence struct; the caller never
// self-asserts binding, scan, queue, frontier, or target evidence.
func (s *Server) advanceAudioClosurePhase(current audioclosure.State, evidence audioclosure.PhaseGuardEvidence) audioclosure.State {
	if s == nil || s.audioClosures == nil || current.Terminal() || current.ClosureID == "" {
		return current
	}
	next, advanceErr := advancePhaseState(current, evidence)
	if advanceErr != nil && s.logger != nil {
		s.logger.Warn("[audio-closure] phase advance failed for %s at %s: %v", current.ClosureID, current.Phase, advanceErr)
	}
	if next.Revision == current.Revision {
		return current
	}
	if err := s.audioClosures.Save(next, current.Revision); err != nil {
		return current
	}
	s.syncFreeStateSpine(next)
	s.persistCurrentProjectWorkspace()
	return next
}

// audioClosureRoundRecord derives the diagnostic round record for the round
// that is about to close, from the observations recorded in that round.
func audioClosureRoundRecord(state audioclosure.State) (audioclosure.DiagnosticRoundRecord, bool) {
	views := map[string]bool{}
	usable := false
	for _, record := range state.Observations {
		if record.Round != state.RoundsStarted {
			continue
		}
		for _, viewID := range record.ViewIDs {
			views[viewID] = true
		}
		usable = true
	}
	if len(views) == 0 {
		return audioclosure.DiagnosticRoundRecord{}, false
	}
	// Pick the primary dimension whose allowed views cover the observed set;
	// primary-view hits win over supporting-view hits.
	primary := audioclosure.DiagnosticDimension("")
	for _, dim := range audioclosure.DefaultDimensionOrder {
		allowed := audioclosure.ValidDimensionViews(dim)
		hit := false
		for viewID := range views {
			for _, candidate := range allowed {
				if candidate == viewID {
					hit = true
					break
				}
			}
		}
		if !hit {
			continue
		}
		if primary == "" {
			primary = dim
		}
		if len(views) == 1 {
			for _, candidate := range audioclosure.DimensionPrimaryViews(dim) {
				if views[candidate] {
					primary = dim
				}
			}
		}
	}
	if primary == "" {
		// Cross-dimension supporting views (project.change_delta,
		// comparison.before_after, ...) support no primary dimension alone.
		return audioclosure.DiagnosticRoundRecord{}, false
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s", state.ClosureID, state.RoundsStarted, state.ProjectRevision)))
	status := audioclosure.RoundEvidenceOpen
	if usable {
		status = audioclosure.RoundEvidenceReady
	}
	return audioclosure.DiagnosticRoundRecord{
		SchemaVersion: audioclosure.DiagnosticRoundSchema,
		RoundID:       "r_" + hex.EncodeToString(digest[:6]),
		GoalID:        state.GoalID, RunID: state.RunID, ConversationID: state.ConversationID,
		PrimaryDimension: primary,
		PriorityReason:   audioclosure.PriorityDefaultOrder,
		ViewsRequested:   sortedMapKeys(views),
		EvidenceStatus:   status,
		ProjectRevision:  state.ProjectRevision,
	}, true
}

func sortedMapKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
