package chat

import (
	"context"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
)

// DIAG3-1 targeting-coverage closure: the free-state loop owns per-track
// observation coverage for the diagnostic dimensions it declares open. The
// model chooses per-track targets freely (CCB never selects views), so a run
// can exhaust its budget with per-track evidence on a few salient tracks
// only; the exhausted-budget closure boundary must then not settle
// no_candidate_found while an active track has never been observed by the
// primary per-track view of any open dimension. This gate closes that gap at
// the boundary: it issues the missing observation requests deterministically
// (the loop is the requester — CCB only executes explicit requests), merges
// the bundles into the durable observation ledger, and extends the closure
// window so the next model turn decides from full coverage. If the evidence
// still admits no domain, no_candidate_found remains the honest outcome.
//
// DIAG3-2 widens the duty to closed dimensions: a dimension that closed
// before the model reached every track still owes its missing tracks at the
// boundary (its per-track primary coverage is incomplete), so a dynamics
// dimension closed early on one salient track cannot starve a broadband
// proposal of dynamics evidence on the fault-carrier tracks. The duty stays
// bounded by the ledger's actual gaps: a closed dimension fully covered on
// every active track never re-triggers, and skipped dimensions never enter.
const (
	// freeStateTargetingCoverageMaxRequests bounds one coverage pass; a
	// project larger than the cap leaves the residual gap open and the next
	// boundary settles honestly.
	freeStateTargetingCoverageMaxRequests = 8
	// freeStateTargetingCoverageInvokeTimeout bounds one deterministic
	// observation request (a bounded L2 render probe on the real kernel).
	freeStateTargetingCoverageInvokeTimeout = 90 * time.Second
	// freeStateTargetingCoverageSliceNeed is the once-only continuation
	// floor granted with the coverage round: the coverage-informed decision
	// slice, the proposal/admission slices, and headroom to reach the applied
	// boundary, which reserves its own post-apply slices
	// (freeStateD1PostApplySliceNeed). Scheduling only — the honesty gates
	// are untouched.
	freeStateTargetingCoverageSliceNeed = 5
	// freeStateDutyObservationToolCallPrefix marks the tool call IDs of
	// observation requests the loop booked deterministically (targeting
	// coverage, frontier feed). The marker is the provenance signal that
	// survives the durable ledger round-trip and routes those bundles onto
	// the duty side of the dual evidence budget.
	freeStateDutyObservationToolCallPrefix = "targeting_coverage:"
)

// freeStateMixCandidateProbeViewIDs is the closed, static set of mix view
// families whose candidate rows the frontier extractor consumes
// (audioClosureCandidateRows). The bounded frontier-feed probe probes
// exactly these families and nothing else: the mapping is the extractor's
// own declared dependency — never a model choice, a domain preference, or a
// sealed hint. A probe that yields no candidates leaves the frontier empty
// and the honest closure unchanged.
var freeStateMixCandidateProbeViewIDs = []string{"mix.multitrack_relationship", "mix.frequency_relationship"}

// freeStateObservationIsDutyBooked reports whether an observation was booked
// by the loop's deterministic duty pass. Live bookings carry the marker on
// the RecentObservation; ledger-rehydrated ones only inside the durable
// summary row, so both are checked.
func freeStateObservationIsDutyBooked(observation *agentloop.RecentObservation) bool {
	if observation == nil {
		return false
	}
	toolCallID := firstNonEmpty(observation.ToolCallID, firstStringFromMap(observation.Summary, "tool_call_id"))
	return strings.HasPrefix(strings.TrimSpace(toolCallID), freeStateDutyObservationToolCallPrefix)
}

// freeStateMixCandidateProbeBookObservation issues one deterministic
// project-level observation request for one candidate-extractor view family
// through the same execution path the coverage duty uses, and returns the
// CCB bundle, or nil when the kernel cannot produce the evidence. Swapped in
// tests.
var freeStateMixCandidateProbeBookObservation = func(s *Server, loop freeStateReasoningLoop, viewID string) map[string]any {
	if s == nil || s.harness == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), freeStateTargetingCoverageInvokeTimeout)
	defer cancel()
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:       "ccb.observation_request",
		Args:       map[string]any{"view_ids": []string{viewID}},
		Context:    map[string]any{"observation_only": true},
		Source:     "free_state_frontier_feed_probe",
		GoalID:     loop.GoalID,
		RunID:      loop.RunID,
		ToolCallID: freeStateDutyObservationToolCallPrefix + sanitizeCanaryID(viewID),
	})
	if err != nil || !strings.EqualFold(strings.TrimSpace(response.Status), "ok") {
		if s.logger != nil {
			s.logger.Warn("[free-state] frontier-feed probe unavailable for %s: %v %s", viewID, err, firstNonEmpty(response.Error, response.Status))
		}
		return nil
	}
	bundle := structMap(response.Result["bundle"])
	status := strings.ToLower(firstStringFromMap(bundle, "status"))
	if status != "ready" && status != "partial" {
		if s.logger != nil {
			s.logger.Warn("[free-state] frontier-feed probe not usable for %s: status=%s", viewID, status)
		}
		return nil
	}
	return bundle
}

// bookFreeStateFrontierFeedProbe probes each candidate-extractor view family
// once, merging usable bundles into the durable ledger under duty-marked
// tool call IDs, and returns the merged ledger, the number of booked
// bundles, and the number of candidates the extractor derives from those
// rows. It is a no-op when the frontier already holds candidates — the feed
// exists only to give the extractor its declared input rows, never to select
// a view for CCB or to push a proposal.
func (s *Server) bookFreeStateFrontierFeedProbe(loop *freeStateReasoningLoop, state audioclosure.State, merged map[string]any) (map[string]any, int, int) {
	if s == nil || loop == nil || len(state.Frontier.Candidates) > 0 {
		return merged, 0, 0
	}
	booked := 0
	recentObservations := make([]*agentloop.RecentObservation, 0, len(freeStateMixCandidateProbeViewIDs))
	for _, viewID := range freeStateMixCandidateProbeViewIDs {
		bundle := freeStateMixCandidateProbeBookObservation(s, *loop, viewID)
		if bundle == nil {
			continue
		}
		observationID := firstStringFromMap(bundle, "observation_id")
		if observationID == "" {
			continue
		}
		status := strings.ToLower(firstStringFromMap(bundle, "status"))
		executed := freeStateStringSlice(firstMapFromAny(firstMapFromAny(bundle["audit_receipt"])["actual_executed_view_ids"]))
		if len(executed) == 0 {
			executed = freeStateStringSlice(bundle["requested_views"])
		}
		if len(executed) == 0 {
			executed = []string{viewID}
		}
		evidence := freeStateStringSlice(bundle["evidence_refs"])
		if len(evidence) == 0 {
			evidence = []string{observationID}
		}
		toolCallID := freeStateDutyObservationToolCallPrefix + sanitizeCanaryID(viewID)
		recent := &agentloop.RecentObservation{
			ToolCallID: toolCallID, Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
			Status: status,
			Summary: map[string]any{
				"schema_version":           "ccb_observation_bundle.v1",
				"status":                   status,
				"observation_id":           observationID,
				"requested_views":          []string{viewID},
				"actual_executed_view_ids": executed,
				"views":                    cloneContext(firstMapFromAny(bundle["views"])),
				"freshness":                cloneContext(firstMapFromAny(bundle["freshness"])),
				"bundle":                   cloneContext(bundle),
				"evidence_refs":            evidence,
				"source":                   "free_state_frontier_feed_probe",
			},
		}
		merged = mergeFreeStateObservationLedger(merged, recent, loop.Cycle)
		if !freeStateContainsString(loop.ObservationIDs, observationID) {
			loop.ObservationIDs = append(loop.ObservationIDs, observationID)
		}
		recentObservations = append(recentObservations, recent)
		booked++
	}
	return merged, booked, len(audioClosureCandidates(nil, recentObservations))
}

// freeStateTargetingCoverageRequest is one deterministic per-track observation
// request: an uncovered track plus the primary views of the eligible
// dimensions it has never been observed with.
type freeStateTargetingCoverageRequest struct {
	TrackID   string
	TrackName string
	ViewIDs   []string
}

// targetingCoverageDimensionScope is one dimension's slice of the coverage
// duty: the dimension plus its track-level primary views, in queue order.
type targetingCoverageDimensionScope struct {
	Dimension audioclosure.DiagnosticDimension
	Views     []string
}

// freeStateTargetingLedgerTrackViews projects the durable observation ledger
// into per-track observed-view sets. Track-targeted rows are keyed
// "track:<id>::<view>"; a row's target_ref is the fallback identity source.
func freeStateTargetingLedgerTrackViews(ledger map[string]any) map[string]map[string]bool {
	observed := map[string]map[string]bool{}
	for key, raw := range firstMapFromAny(ledger["available_views"]) {
		row := firstMapFromAny(raw)
		viewID := firstStringFromMap(row, "view_id")
		trackID := ""
		if strings.HasPrefix(key, "track:") {
			if rest, _, found := strings.Cut(strings.TrimPrefix(key, "track:"), "::"); found {
				trackID = rest
			}
		}
		if trackID == "" {
			target := firstMapFromAny(row["target_ref"])
			if strings.EqualFold(firstStringFromMap(target, "kind", "target_kind"), "track") {
				trackID = firstStringFromMap(target, "id", "target_id", "track_id")
			}
		}
		if trackID == "" || viewID == "" {
			continue
		}
		if observed[trackID] == nil {
			observed[trackID] = map[string]bool{}
		}
		observed[trackID][strings.TrimSpace(viewID)] = true
	}
	return observed
}

// freeStateTargetingCoverageDimensionScope selects the dimensions whose
// primary per-track views the coverage duty covers, in queue order. An open
// dimension is always in scope. A closed dimension joins when its per-track
// primary coverage is incomplete — some active track has never been observed
// with its primary views — so a dimension closed before the model reached
// every track still owes its missing tracks at the boundary (DIAG3-2: the p03
// broadband-compression entry starvation). A closed dimension fully covered
// on every active track stays out of scope, and skipped dimensions never
// enter: the duty closes actual ledger gaps; it neither re-opens settled
// dimensions nor expands beyond them.
func freeStateTargetingCoverageDimensionScope(queue *audioclosure.PriorityQueue, ledger map[string]any, tracks []harness.ActiveAudioTrack) []targetingCoverageDimensionScope {
	if queue == nil {
		return nil
	}
	observed := freeStateTargetingLedgerTrackViews(ledger)
	activeTracks := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if track.ID != "" {
			activeTracks = append(activeTracks, track.ID)
		}
	}
	scope := make([]targetingCoverageDimensionScope, 0, len(queue.Entries))
	for _, entry := range queue.Entries {
		views := make([]string, 0, 1)
		for _, viewID := range audioclosure.DimensionPrimaryViews(entry.Dimension) {
			if strings.HasPrefix(strings.TrimSpace(viewID), "track.") {
				views = append(views, viewID)
			}
		}
		if len(views) == 0 {
			continue
		}
		switch entry.Status {
		case audioclosure.QueueOpen:
		case audioclosure.QueueClosed:
			if !targetingCoverageDimensionUndercovered(views, observed, activeTracks) {
				continue
			}
		default:
			// Skipped dimensions left the diagnostic spine by policy.
			continue
		}
		scope = append(scope, targetingCoverageDimensionScope{Dimension: entry.Dimension, Views: views})
	}
	return scope
}

// targetingCoverageDimensionUndercovered reports whether any active track has
// never been observed with all of the dimension's primary per-track views.
func targetingCoverageDimensionUndercovered(views []string, observed map[string]map[string]bool, activeTracks []string) bool {
	for _, trackID := range activeTracks {
		seen := observed[trackID]
		covered := true
		for _, viewID := range views {
			if !seen[viewID] {
				covered = false
				break
			}
		}
		if !covered {
			return true
		}
	}
	return false
}

// freeStateTargetingCoverageRequests computes the missing per-track coverage:
// eligible dimensions in queue order (open dimensions plus closed dimensions
// with incomplete per-track primary coverage), each dimension's primary
// per-track views, active tracks from the shadow projection. The result
// carries at most one request per uncovered track in stable track-ID order,
// bounded by freeStateTargetingCoverageMaxRequests. No track ordering signal
// beyond the ID enters this computation — it is a coverage duty, not a
// ranking.
func freeStateTargetingCoverageRequests(queue *audioclosure.PriorityQueue, ledger map[string]any, tracks []harness.ActiveAudioTrack) []freeStateTargetingCoverageRequest {
	if queue == nil || len(tracks) == 0 {
		return nil
	}
	scope := freeStateTargetingCoverageDimensionScope(queue, ledger, tracks)
	if len(scope) == 0 {
		return nil
	}
	observed := freeStateTargetingLedgerTrackViews(ledger)
	requests := make([]freeStateTargetingCoverageRequest, 0, len(tracks))
	for _, track := range tracks {
		if track.ID == "" {
			continue
		}
		seen := observed[track.ID]
		uncovered := make([]string, 0, len(scope))
		for _, dim := range scope {
			for _, viewID := range dim.Views {
				if seen[viewID] {
					continue
				}
				duplicate := false
				for _, existing := range uncovered {
					if existing == viewID {
						duplicate = true
						break
					}
				}
				if !duplicate {
					uncovered = append(uncovered, viewID)
				}
			}
		}
		if len(uncovered) == 0 {
			continue
		}
		requests = append(requests, freeStateTargetingCoverageRequest{TrackID: track.ID, TrackName: track.Name, ViewIDs: uncovered})
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].TrackID < requests[j].TrackID })
	if len(requests) > freeStateTargetingCoverageMaxRequests {
		requests = requests[:freeStateTargetingCoverageMaxRequests]
	}
	return requests
}

// freeStateTargetingCoverageBookObservation issues one deterministic
// observation request through the capability harness — the same execution
// path model-requested observations and the D1 post-action booking use — and
// returns the CCB bundle, or nil when the kernel cannot produce the evidence.
// Swapped in tests.
var freeStateTargetingCoverageBookObservation = func(s *Server, loop freeStateReasoningLoop, req freeStateTargetingCoverageRequest) map[string]any {
	if s == nil || s.harness == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), freeStateTargetingCoverageInvokeTimeout)
	defer cancel()
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "ccb.observation_request",
		Args: map[string]any{
			"view_ids":   append([]string(nil), req.ViewIDs...),
			"target_ref": map[string]any{"kind": "track", "id": req.TrackID, "label": req.TrackName},
		},
		Context:    map[string]any{"observation_only": true},
		Source:     "free_state_targeting_coverage",
		GoalID:     loop.GoalID,
		RunID:      loop.RunID,
		ToolCallID: "targeting_coverage:" + sanitizeCanaryID(req.TrackID),
	})
	if err != nil || !strings.EqualFold(strings.TrimSpace(response.Status), "ok") {
		if s.logger != nil {
			s.logger.Warn("[free-state] targeting-coverage observation unavailable for %s: %v %s", req.TrackID, err, firstNonEmpty(response.Error, response.Status))
		}
		return nil
	}
	bundle := structMap(response.Result["bundle"])
	status := strings.ToLower(firstStringFromMap(bundle, "status"))
	if status != "ready" && status != "partial" {
		if s.logger != nil {
			s.logger.Warn("[free-state] targeting-coverage observation not usable for %s: status=%s", req.TrackID, status)
		}
		return nil
	}
	return bundle
}

// grantFreeStateFrontierDecisionRoundAtBoundary is the proposal-side mirror
// of the coverage diversion: with the frontier established, the loop still
// pre-proposal, and continuation budget remaining, the boundary grants one
// decision round (window +1) instead of settling. The grant is once-per-loop
// — a spent grant falls through to the ordinary honest settle, and a
// proposal already admitted (Experiment != nil) never needs it.
func (s *Server) grantFreeStateFrontierDecisionRoundAtBoundary(loop freeStateReasoningLoop, state audioclosure.State) (audioclosure.State, bool, error) {
	if loop.FrontierDecisionRoundGranted || loop.Experiment != nil || freeStateContinuationBudgetExhausted(loop) {
		return state, false, nil
	}
	loop.FrontierDecisionRoundGranted = true
	if loop.ContinuationBudget > 0 && loop.ContinuationUsed+freeStateTargetingCoverageSliceNeed > loop.ContinuationBudget {
		loop.ContinuationBudget = loop.ContinuationUsed + freeStateTargetingCoverageSliceNeed
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	next, err := (audioclosure.Driver{}).ExtendClosureRounds(state, state.Revision, state.Policy.MaxClosureRounds+1, time.Now().UTC())
	if err != nil {
		return state, false, err
	}
	if err := s.audioClosures.Save(next, state.Revision); err != nil {
		return state, false, err
	}
	s.persistCurrentProjectWorkspace()
	if s.logger != nil {
		s.logger.Info("[free-state] frontier decision round granted on %s: %d frontier candidate(s) established with continuation budget remaining; closure window extended to %d round(s)", state.ClosureID, len(state.Frontier.Candidates), next.Policy.MaxClosureRounds)
	}
	return next, true, nil
}

// freeStateObservationSaturationNotice builds the DIAG3-3 mechanical runtime
// notice: per-track primary observation coverage is complete while the
// diagnostic queue is still open and the hypothesis frontier is empty. Only
// structural state facts enter the map — dimension names from the queue
// record, coverage status, frontier size, continuation budget. No track
// identity, domain name, or domain verb ever enters: the notice tells the
// model face that the observation obligation is closed, never what to
// propose.
func freeStateObservationSaturationNotice(loop freeStateReasoningLoop, state audioclosure.State, openDimensions []string, coverageDimensions int) map[string]any {
	dimensions := append([]string(nil), openDimensions...)
	if dimensions == nil {
		dimensions = []string{}
	}
	return map[string]any{
		"schema_version":      freeStateObservationSaturationNoticeSchema,
		"coverage_status":     "per_track_primary_complete",
		"coverage_dimensions": coverageDimensions,
		"open_dimensions":     dimensions,
		"frontier_candidates": len(state.Frontier.Candidates),
		"continuation_used":   loop.ContinuationUsed,
		"continuation_budget": loop.ContinuationBudget,
	}
}

// setFreeStateObservationSaturationNotice stamps the once-only notice onto
// the loop. The retirement latch (ObservationSaturationRounds bounded by
// freeStateSaturationNoticeMaxRounds) keeps a retired notice retired, so the
// exhausted-budget honest settle stays the ordinary outcome afterwards.
func (s *Server) setFreeStateObservationSaturationNotice(loop *freeStateReasoningLoop, state audioclosure.State, coverageDimensions int) {
	if loop == nil || len(loop.ObservationSaturationNotice) > 0 || loop.ObservationSaturationRounds >= freeStateSaturationNoticeMaxRounds {
		return
	}
	loop.ObservationSaturationNotice = freeStateObservationSaturationNotice(*loop, state, freeStateQueueOpenDimensions(loop.PriorityQueue), coverageDimensions)
	if s != nil && s.logger != nil {
		s.logger.Info("[free-state] observation saturation notice injected on %s: coverage complete, queue open, frontier empty (surfacing budget %d decision round(s))", state.ClosureID, freeStateSaturationNoticeMaxRounds)
	}
}

// freeStateQueueOpenDimensions lists the queue record's open dimension names
// in queue order — the same disclosure the prompt summary projects, never an
// invented list.
func freeStateQueueOpenDimensions(queue *audioclosure.PriorityQueue) []string {
	if queue == nil {
		return nil
	}
	open := make([]string, 0, len(queue.Entries))
	for _, entry := range queue.Entries {
		if entry.Status == audioclosure.QueueOpen {
			open = append(open, string(entry.Dimension))
		}
	}
	return open
}

// closeFreeStateTargetingCoverageAtBoundary is the pre-settle gate at the
// exhausted-budget closure boundary, hosting the two DIAG3-3 propulsion
// diversions. It returns (nextState, diverted, err): diverted=true means the
// boundary granted another round instead of settling. First the coverage
// diversion (DIAG3-1/3-2): the pass books the missing per-track observation
// requests and extends the closure window so the next model turn decides
// from full coverage. Second the frontier diversion: an established
// hypothesis frontier with continuation budget remaining is a proposal
// opportunity, never a settle point — the model must hold at least one
// decision round with the candidates visible before an honest settle may run
// (run 20260906_194504: the mix-level conflict candidates landed on the
// final closure round and the boundary settled capability_blocked with zero
// proposal attempts). Both grants are once-per-loop; every degraded path
// returns diverted=false so the honest settle runs unchanged.
func (s *Server) closeFreeStateTargetingCoverageAtBoundary(state audioclosure.State) (audioclosure.State, bool, error) {
	if s == nil || state.Terminal() || state.ContractID == "" {
		return state, false, nil
	}
	loop, ok := s.freeStateLoop(state.ConversationID)
	if !ok || !freeStateLoopActive(loop) || loop.PriorityQueue == nil || !loop.PriorityQueue.HasOpen() {
		return state, false, nil
	}
	if !s.freeStateQueueStillOpenWithinCapacity(state.ConversationID) {
		return state, false, nil
	}
	if len(state.Frontier.Candidates) > 0 {
		return s.grantFreeStateFrontierDecisionRoundAtBoundary(loop, state)
	}
	if loop.TargetingCoveragePassDone {
		return state, false, nil
	}
	var tracks []harness.ActiveAudioTrack
	if s.harness != nil {
		tracks = s.harness.ActiveAudioTracks(context.Background())
	}
	coverageDimensions := len(freeStateTargetingCoverageDimensionScope(loop.PriorityQueue, loop.ObservationLedger, tracks))
	requests := freeStateTargetingCoverageRequests(loop.PriorityQueue, loop.ObservationLedger, tracks)
	loop.TargetingCoveragePassDone = true
	if len(requests) == 0 {
		// DIAG3-3: coverage is complete while the queue is still open and the
		// frontier is empty (both guards above passed). The observation duty
		// is closed — tell the model face once, mechanically, so the
		// remaining budget can be spent on a bounded proposal or an honest
		// refusal instead of repeated covered-observation requests.
		s.setFreeStateObservationSaturationNotice(&loop, state, coverageDimensions)
		// F5 bounded frontier feed: the frontier candidates can only come
		// from the candidate extractor's declared view families, which no
		// per-track duty covers. With per-track coverage complete and the
		// frontier still empty, those families are probed once each here —
		// duty-accounted — so the frontier gate stops depending on the model
		// spontaneously requesting them. A probe without usable rows leaves
		// the honest closure unchanged.
		merged := loop.ObservationLedger
		if merged == nil {
			merged = map[string]any{}
		}
		merged, bookedMix, feedCandidates := s.bookFreeStateFrontierFeedProbe(&loop, state, merged)
		if bookedMix == 0 || feedCandidates == 0 {
			// The probe produced no extractor-derived candidates: the
			// frontier stays empty and the honest closure runs unchanged.
			// Any usable rows remain durable duty evidence on the model face.
			s.storeFreeStateLoop(loop)
			return state, false, nil
		}
		loop.ObservationLedger = merged
		if durableLatest := freeStateLatestObservationFromLedger(merged); durableLatest != nil {
			loop.LatestObservation = durableLatest
		}
		if loop.ContinuationBudget > 0 && loop.ContinuationUsed+freeStateTargetingCoverageSliceNeed > loop.ContinuationBudget {
			loop.ContinuationBudget = loop.ContinuationUsed + freeStateTargetingCoverageSliceNeed
		}
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		next, err := (audioclosure.Driver{}).ExtendClosureRounds(state, state.Revision, state.Policy.MaxClosureRounds+1, time.Now().UTC())
		if err != nil {
			return state, false, err
		}
		if err := s.audioClosures.Save(next, state.Revision); err != nil {
			return state, false, err
		}
		s.persistCurrentProjectWorkspace()
		if s.logger != nil {
			s.logger.Info("[free-state] frontier-feed probe booked %d extractor-declared duty observation request(s) on %s; closure window extended to %d round(s)", bookedMix, state.ClosureID, next.Policy.MaxClosureRounds)
		}
		return next, true, nil
	}
	merged := loop.ObservationLedger
	if merged == nil {
		merged = map[string]any{}
	}
	booked := 0
	for _, req := range requests {
		bundle := freeStateTargetingCoverageBookObservation(s, loop, req)
		if bundle == nil {
			continue
		}
		observationID := firstStringFromMap(bundle, "observation_id")
		if observationID == "" {
			continue
		}
		status := strings.ToLower(firstStringFromMap(bundle, "status"))
		executed := freeStateStringSlice(firstMapFromAny(firstMapFromAny(bundle["audit_receipt"])["actual_executed_view_ids"]))
		if len(executed) == 0 {
			executed = freeStateStringSlice(bundle["requested_views"])
		}
		if len(executed) == 0 {
			executed = append([]string(nil), req.ViewIDs...)
		}
		evidence := freeStateStringSlice(bundle["evidence_refs"])
		if len(evidence) == 0 {
			evidence = []string{observationID}
		}
		toolCallID := "targeting_coverage:" + sanitizeCanaryID(req.TrackID)
		recent := &agentloop.RecentObservation{
			ToolCallID: toolCallID, Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
			Status: status,
			Summary: map[string]any{
				"schema_version":           "ccb_observation_bundle.v1",
				"status":                   status,
				"observation_id":           observationID,
				"requested_views":          append([]string(nil), req.ViewIDs...),
				"actual_executed_view_ids": executed,
				"views":                    cloneContext(firstMapFromAny(bundle["views"])),
				"freshness":                cloneContext(firstMapFromAny(bundle["freshness"])),
				"bundle":                   cloneContext(bundle),
				"evidence_refs":            evidence,
				"target_ref":               map[string]any{"kind": "track", "id": req.TrackID, "label": req.TrackName},
				"source":                   "free_state_targeting_coverage",
			},
		}
		merged = mergeFreeStateObservationLedger(merged, recent, loop.Cycle)
		if !freeStateContainsString(loop.ObservationIDs, observationID) {
			loop.ObservationIDs = append(loop.ObservationIDs, observationID)
		}
		booked++
	}
	// F5 bounded frontier feed: the frontier is empty on this path (an
	// established frontier diverted above), so the pass probes the candidate
	// extractor's declared view families once each — duty-accounted — before
	// the next decision round extracts candidates from the merged ledger.
	merged, bookedMix, feedCandidates := s.bookFreeStateFrontierFeedProbe(&loop, state, merged)
	if booked == 0 && (bookedMix == 0 || feedCandidates == 0) {
		s.storeFreeStateLoop(loop)
		if s.logger != nil {
			s.logger.Warn("[free-state] targeting-coverage pass booked no usable evidence for %s; the boundary settles honestly", state.ClosureID)
		}
		return state, false, nil
	}
	loop.ObservationLedger = merged
	if durableLatest := freeStateLatestObservationFromLedger(merged); durableLatest != nil {
		loop.LatestObservation = durableLatest
	}
	if loop.ContinuationBudget > 0 && loop.ContinuationUsed+freeStateTargetingCoverageSliceNeed > loop.ContinuationBudget {
		loop.ContinuationBudget = loop.ContinuationUsed + freeStateTargetingCoverageSliceNeed
	}
	// DIAG3-2 coverage re-check: the booked bundles may have closed the
	// per-track gap (the request list is capped at
	// freeStateTargetingCoverageMaxRequests, so only a re-check against the
	// merged ledger proves completion). Complete coverage with the queue
	// still open is exactly the DIAG3-3 saturation shape, and the
	// coverage-informed decision round is its first audience.
	if len(freeStateTargetingCoverageRequests(loop.PriorityQueue, merged, tracks)) == 0 {
		s.setFreeStateObservationSaturationNotice(&loop, state, coverageDimensions)
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	next, err := (audioclosure.Driver{}).ExtendClosureRounds(state, state.Revision, state.Policy.MaxClosureRounds+1, time.Now().UTC())
	if err != nil {
		return state, false, err
	}
	if err := s.audioClosures.Save(next, state.Revision); err != nil {
		return state, false, err
	}
	s.persistCurrentProjectWorkspace()
	if s.logger != nil {
		s.logger.Info("[free-state] targeting-coverage pass booked %d per-track observation request(s) for %d coverage dimension(s) (open or closed with a per-track gap) on %s; closure window extended to %d round(s)",
			booked, coverageDimensions, state.ClosureID, next.Policy.MaxClosureRounds)
		if bookedMix > 0 {
			s.logger.Info("[free-state] frontier-feed probe booked %d extractor-declared duty observation request(s) on %s alongside the coverage pass", bookedMix, state.ClosureID)
		}
	}
	return next, true, nil
}
