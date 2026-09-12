package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

const (
	freeStateReasoningLoopSchema           = "free_state_reasoning_loop.v1"
	freeStateObservationLedgerSchema       = "free_state_observation_ledger.v1"
	freeStateObservationLedgerLimit        = 24
	freeStateDefaultMaxCycles              = 6
	freeStateMaxActionCount                = 6
	freeStateMaxFamilyActionCount          = 2
	freeStatePhaseProcessorSelection       = "processor_selection"
	freeStatePhaseProcessorMaterialization = "processor_materialization"
	freeStatePhasePostActionEvaluation     = "post_action_evaluation"
	// freeStateObservationSaturationNoticeSchema is the DIAG3-3 mechanical
	// runtime notice injected at the exhausted-budget closure boundary when
	// per-track primary coverage is complete while the diagnostic queue is
	// still open and the hypothesis frontier is empty.
	freeStateObservationSaturationNoticeSchema = "free_state_observation_saturation_notice.v1"
	// freeStateSaturationNoticeMaxRounds bounds the notice's surfacing
	// window: after two decision rounds without a proposal the notice
	// retires and the ordinary (honest) budget-exhaustion settle applies.
	freeStateSaturationNoticeMaxRounds = 2
)

type freeStateActionRecord struct {
	Cycle         int            `json:"cycle"`
	ProcessorType string         `json:"processor_type"`
	Workflow      string         `json:"workflow"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	Receipt       map[string]any `json:"receipt,omitempty"`
	RecordedAt    time.Time      `json:"recorded_at"`
}

// freeStateReasoningLoop is orchestration memory, not mutation authority. It
// keeps one user intent intact while governed processor workflows pause for
// confirmation and return their receipts on later HTTP turns.
type freeStateReasoningLoop struct {
	SchemaVersion               string                       `json:"schema_version"`
	LoopID                      string                       `json:"loop_id"`
	ConversationID              string                       `json:"conversation_id"`
	GoalID                      string                       `json:"goal_id,omitempty"`
	RunID                       string                       `json:"run_id,omitempty"`
	AuthorityMode               experiment.AuthorityMode     `json:"authority_mode"`
	Status                      string                       `json:"status"`
	DecisionPhase               string                       `json:"decision_phase"`
	OriginalIntent              string                       `json:"original_intent"`
	ActiveIntent                string                       `json:"active_intent"`
	TargetRef                   map[string]any               `json:"target_ref,omitempty"`
	Cycle                       int                          `json:"cycle"`
	MaxCycles                   int                          `json:"max_cycles"`
	CurrentRoundID              string                       `json:"current_round_id,omitempty"`
	CurrentPhase                string                       `json:"current_phase,omitempty"`
	PriorityQueue               *audioclosure.PriorityQueue  `json:"priority_queue,omitempty"`
	ContinuationBudget          int                          `json:"continuation_budget,omitempty"`
	ContinuationUsed            int                          `json:"continuation_used,omitempty"`
	ObservationIDs              []string                     `json:"observation_ids,omitempty"`
	ObservationReceipts         []map[string]any             `json:"observation_receipts,omitempty"`
	RejectedObservationRequests []map[string]any             `json:"rejected_observation_requests,omitempty"`
	ObservationLedger           map[string]any               `json:"observation_ledger,omitempty"`
	LatestProjectChange         map[string]any               `json:"latest_project_change,omitempty"`
	LatestObservation           *agentloop.RecentObservation `json:"latest_observation,omitempty"`
	Actions                     []freeStateActionRecord      `json:"actions,omitempty"`
	LatestDecision              *agentloop.FreeStateDecision `json:"latest_decision,omitempty"`
	Experiment                  *experiment.Turn             `json:"experiment,omitempty"`
	D1State                     map[string]any               `json:"d1_state,omitempty"`
	D1Receipt                   map[string]any               `json:"d1_receipt,omitempty"`
	// AdmissionReceipt is the read-only FS6/FS7 boundary audit. It is kept
	// separately from the execution receipt because a proposal may be absent
	// or rejected before an experiment Turn exists.
	AdmissionReceipt              map[string]any `json:"admission_receipt,omitempty"`
	AuditionSessionID             string         `json:"audition_session_id,omitempty"`
	AuditionSessionSnapshot       map[string]any `json:"audition_session_snapshot,omitempty"`
	RequiresPostActionObservation bool           `json:"requires_post_action_observation"`
	PostActionObservationReserved bool           `json:"post_action_observation_reserved,omitempty"`
	// PostApplyBudgetReserved marks the one-shot D1 post-apply budget floor
	// granted at the applied boundary (reserveD1PostApplySlices). It is separate
	// from PostActionObservationReserved so the generic +1 observation reserve
	// keeps its own once-only semantics on top of the floor.
	PostApplyBudgetReserved bool `json:"post_apply_budget_reserved,omitempty"`
	// TargetingCoveragePassDone marks the once-per-loop DIAG3-1 coverage
	// pass: the pre-settle gate that books per-track observations for active
	// tracks never covered by an open dimension's primary view. Done stays
	// set after the pass so a failed or still-insufficient coverage round
	// settles honestly at the next boundary instead of extending forever.
	TargetingCoveragePassDone bool `json:"targeting_coverage_pass_done,omitempty"`
	// ObservationSaturationNotice is the DIAG3-3 mechanical runtime notice
	// for the "per-track coverage complete + queue open + frontier empty"
	// shape (run 20260906_181304: full 6×5 coverage booked, every remaining
	// model turn stayed needs_observation in fs4, zero proposal attempts). It
	// carries only structural state facts — dimension list, coverage status,
	// frontier size, continuation budget — never a track identity, domain
	// name, or domain verb. The notice retires after two surfaced decision
	// rounds without a proposal or on the first needs_experiment decision;
	// the exhausted-budget honest settle semantics are untouched.
	ObservationSaturationNotice map[string]any `json:"observation_saturation_notice,omitempty"`
	// ObservationSaturationRounds counts the completed decision rounds since
	// the saturation notice was injected; it is the retirement latch.
	ObservationSaturationRounds int `json:"observation_saturation_rounds,omitempty"`
	// FrontierDecisionRoundGranted marks the once-per-loop DIAG3-3 grant of
	// one decision round after the hypothesis frontier became established at
	// an exhausted closure boundary (run 20260906_194504: the mix-level
	// conflict candidates landed on the final closure round, the boundary
	// settled capability_blocked and the model never held a turn with the
	// frontier visible). Granted stays set so the ordinary honest settle
	// applies at the next boundary if the model still returns no proposal.
	FrontierDecisionRoundGranted bool `json:"frontier_decision_round_granted,omitempty"`
	// TIMING-1 anti-abuse accounting (advisory ruling #5 rules 1-3), recorded
	// agentloop-side at the G-gate bounce boundary and persisted here like the
	// terminal-turn fields. AdmissionRejectionCount counts evidence-type
	// proposal bounces (capped at two per loop); AdmissionRejectionGaps holds
	// the structured content-blind gap records for the terminal-turn
	// disclosure; LastRejectedProposalFingerprint/…EvidenceRevision detect the
	// identical-resubmission-without-new-evidence shape that locks the
	// terminal turn directly. All fields are content-free.
	AdmissionRejectionCount         int           `json:"admission_rejection_count,omitempty"`
	AdmissionRejectionGaps          []map[string]any `json:"admission_rejection_gaps,omitempty"`
	LastRejectedProposalFingerprint string        `json:"last_rejected_proposal_fingerprint,omitempty"`
	LastRejectedEvidenceRevision    string        `json:"last_rejected_evidence_revision,omitempty"`
	// SettleRefusedRoundID records the round whose settle report was refused
	// because its fresh post-action observation had not landed yet. In that
	// race window the experiment projection can still show the round pre-action
	// (the intervention booking lags the transport receipt), so neither the
	// pending-settlement nor the spent-mutation predicate holds and a replayed
	// pending tick would pass as the round's first mutation. The marker is the
	// authoritative same-round state the recordGoalResult guard must see; it
	// retires when the settle report lands or the round advances.
	SettleRefusedRoundID string `json:"settle_refused_round_id,omitempty"`
	// TerminalTurnLocked is the BOUNDARY-1 terminal-turn reservation latch. It
	// is set once, atomically, when the window budget becomes critical (closure
	// rounds remaining <= 1 or continuations remaining <= 1) or the final
	// continuation checkpoint is reached. From that point the agentloop output
	// gate admits only a needs_experiment decision with a complete proposal or
	// the TerminalDecisions family, so ordinary observation/tool turns cannot
	// consume the reserved checkpoint. The three system settles (closure round
	// boundary / evidence ceiling / continuation exhaustion) run only after the
	// locked turn failed (honest fallback), never as a success exit.
	TerminalTurnLocked bool `json:"terminal_turn_locked,omitempty"`
	// TerminalTurnReason records which trigger fired: budget_critical or
	// last_checkpoint. Content-free.
	TerminalTurnReason string `json:"terminal_turn_reason,omitempty"`
	// TerminalRetryCount bounds the strengthened terminal retry at one per
	// loop. It is incremented agentloop-side (output-gate rejection boundary)
	// and rides the durable loop through the continuation merge.
	TerminalRetryCount int    `json:"terminal_retry_count,omitempty"`
	LastError          string `json:"last_error,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func freeStateLoopActive(loop freeStateReasoningLoop) bool {
	if loop.SchemaVersion != freeStateReasoningLoopSchema || strings.TrimSpace(loop.OriginalIntent) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(loop.Status)) {
	case "completed", "cancelled", "blocked", "capability_blocked", "no_candidate_found", "failed", "stopped":
		return false
	default:
		return true
	}
}

func shouldStartFreeStateReasoningLoop(userText string, requestContext map[string]any) bool {
	if agentModeFromContext(requestContext) == agentModePlan ||
		contextBool(requestContext, "disable_free_state_reasoning") || strings.HasPrefix(strings.TrimSpace(userText), "/") {
		return false
	}
	// Diagnostic-only replay is an explicit read-only boundary. It does not
	// need semantic-entry classification before entering the CCB observation
	// loop, and must not be mistaken for an ordinary treatment request.
	if contextBool(requestContext, "free_state_diagnostic_only") {
		return true
	}
	decision, ok := semanticEntryDecisionFromContext(requestContext)
	controller, controllerOK := orchestrationControllerDecisionFromContext(requestContext)
	return ok && controllerOK && controller.Controller == orchestrationcontroller.MinimalAudioClosure &&
		decision.Route == semanticEntryRouteOpenSemantic &&
		decision.ControlMode == semanticEntryControlSemanticLoop &&
		decision.UserAuthorization == semanticEntryAuthorizationAction
}

func (s *Server) prepareFreeStateReasoningContext(conversationID, userText string, requestContext map[string]any) (map[string]any, bool) {
	if s == nil {
		return requestContext, false
	}
	// The request context is a transport surface and can contain an older
	// response envelope.  Prefer the server's durable loop when it exists, and
	// merge the transport copy into it without allowing a shallow map overwrite
	// to discard the observation ledger accumulated by MessageLoop.
	requestLoop, requestOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"])
	persistedLoop, persistedOK := s.freeStateLoop(conversationID)
	loop := freeStateReasoningLoop{}
	switch {
	case persistedOK && freeStateLoopActive(persistedLoop):
		loop = mergeFreeStateLoops(persistedLoop, requestLoop, requestOK)
	case requestOK:
		loop = requestLoop
	}
	if !freeStateLoopActive(loop) {
		if contextBool(requestContext, "free_state_internal_resume") || !shouldStartFreeStateReasoningLoop(userText, requestContext) {
			return requestContext, false
		}
		now := time.Now().UTC()
		goalID, runID := goalIDsFromContext(requestContext)
		loop = freeStateReasoningLoop{
			SchemaVersion:  freeStateReasoningLoopSchema,
			LoopID:         "free_state_" + randomID(),
			ConversationID: conversationID,
			GoalID:         goalID,
			RunID:          runID,
			AuthorityMode:  authorityModeFromContext(requestContext),
			Status:         "reasoning",
			DecisionPhase:  freeStatePhaseProcessorSelection,
			OriginalIntent: strings.TrimSpace(userText),
			ActiveIntent:   strings.TrimSpace(userText),
			TargetRef:      freeStateTargetRef(requestContext),
			MaxCycles:      freeStateDefaultMaxCycles,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		s.storeFreeStateLoop(loop)
	}
	if loop.AuthorityMode == "" {
		loop.AuthorityMode = authorityModeFromContext(requestContext)
	}
	goalID, runID := goalIDsFromContext(requestContext)
	if goalID != "" {
		loop.GoalID = goalID
	}
	if runID != "" {
		loop.RunID = runID
	}
	// BOUNDARY-1 §1.1: evaluate the terminal-turn triggers at the one context
	// binding boundary both the HTTP turn and every scheduler resume cross, so
	// the lock lands before the next model slice runs and the reserved
	// checkpoint can only be spent on a terminal-family output.
	s.evaluateFreeStateTerminalTurnTrigger(&loop)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return mergeContext(requestContext, map[string]any{"free_state_reasoning_loop": freeStateLoopMap(loop)}), true
}

// mergeFreeStateLoops combines a durable orchestration loop with a transport
// copy.  Evidence-bearing fields are merged by identity; a missing ledger in
// the transport copy can therefore never erase observations from a previous
// HTTP turn or a restored workspace.
func mergeFreeStateLoops(base, overlay freeStateReasoningLoop, overlayOK bool) freeStateReasoningLoop {
	out := cloneFreeStateLoop(base)
	if !overlayOK {
		return out
	}
	overlayNewer := freeStateLoopOverlayNewer(out, overlay)
	if overlay.SchemaVersion != "" {
		out.SchemaVersion = overlay.SchemaVersion
	}
	for _, field := range []struct {
		dst *string
		src string
	}{
		{&out.LoopID, overlay.LoopID}, {&out.ConversationID, overlay.ConversationID},
		{&out.GoalID, overlay.GoalID}, {&out.RunID, overlay.RunID},
		{&out.DecisionPhase, overlay.DecisionPhase}, {&out.OriginalIntent, overlay.OriginalIntent},
		{&out.ActiveIntent, overlay.ActiveIntent}, {&out.LastError, overlay.LastError},
	} {
		if strings.TrimSpace(field.src) != "" {
			*field.dst = field.src
		}
	}
	// A continuation carries the input snapshot that started its slice. That
	// snapshot is commonly older than the durable loop updated after the prior
	// observation. Do not let it regress an active/terminal status.
	if strings.TrimSpace(overlay.Status) != "" &&
		(strings.TrimSpace(out.Status) == "" || (overlayNewer && !(freeStateLoopTerminal(out.Status) && !freeStateLoopTerminal(overlay.Status)))) {
		out.Status = overlay.Status
	}
	if out.AuthorityMode == "" {
		out.AuthorityMode = overlay.AuthorityMode
	}
	if out.AuthorityMode == "" {
		out.AuthorityMode = experiment.AuthorityOrdinary
	}
	if overlay.TargetRef != nil {
		out.TargetRef = cloneContext(overlay.TargetRef)
	}
	if len(overlay.LatestProjectChange) > 0 {
		out.LatestProjectChange = cloneContext(overlay.LatestProjectChange)
	}
	// The saturation notice is set at the closure boundary before the next
	// round runs, so a newer overlay owns it. The rounds counter is monotonic;
	// the overlay never resurrects a notice the durable loop already retired
	// (overlay rounds can only exceed, never restore an older notice).
	if len(overlay.ObservationSaturationNotice) > 0 && overlay.ObservationSaturationRounds >= out.ObservationSaturationRounds {
		out.ObservationSaturationNotice = cloneContext(overlay.ObservationSaturationNotice)
	}
	if overlay.ObservationSaturationRounds > out.ObservationSaturationRounds {
		out.ObservationSaturationRounds = overlay.ObservationSaturationRounds
	}
	// TIMING-1 anti-abuse accounting is one logical record written atomically
	// at the agentloop rejection boundary: the counter only moves forward, and
	// a newer rejection record (count, gaps, fingerprint, evidence revision)
	// owns the whole set. A transport copy that predates the rejections never
	// resurrects them.
	if overlay.AdmissionRejectionCount > out.AdmissionRejectionCount {
		out.AdmissionRejectionCount = overlay.AdmissionRejectionCount
		out.AdmissionRejectionGaps = cloneFreeStateGapRows(overlay.AdmissionRejectionGaps)
		out.LastRejectedProposalFingerprint = overlay.LastRejectedProposalFingerprint
		out.LastRejectedEvidenceRevision = overlay.LastRejectedEvidenceRevision
	} else if overlay.AdmissionRejectionCount == out.AdmissionRejectionCount && len(overlay.AdmissionRejectionGaps) > len(out.AdmissionRejectionGaps) {
		out.AdmissionRejectionGaps = cloneFreeStateGapRows(overlay.AdmissionRejectionGaps)
	}
	// BOUNDARY-1 terminal-turn fields are sticky the same way: a transport
	// copy that predates the lock never clears it, the reason is replaced only
	// by the locking write itself, and the agentloop-incremented retry counter
	// only moves forward.
	if overlay.TerminalTurnLocked {
		out.TerminalTurnLocked = true
		if out.TerminalTurnReason == "" {
			out.TerminalTurnReason = overlay.TerminalTurnReason
		}
	}
	if overlay.TerminalRetryCount > out.TerminalRetryCount {
		out.TerminalRetryCount = overlay.TerminalRetryCount
	}
	if len(overlay.AuditionSessionSnapshot) > 0 {
		out.AuditionSessionSnapshot = cloneContext(overlay.AuditionSessionSnapshot)
	}
	if len(overlay.D1State) > 0 {
		out.D1State = cloneContext(overlay.D1State)
	}
	if len(overlay.D1Receipt) > 0 {
		out.D1Receipt = cloneContext(overlay.D1Receipt)
	}
	if len(overlay.AdmissionReceipt) > 0 {
		out.AdmissionReceipt = cloneContext(overlay.AdmissionReceipt)
	}
	if overlay.MaxCycles > 0 {
		out.MaxCycles = overlay.MaxCycles
	}
	if overlay.CurrentRoundID != "" && (overlayNewer || out.CurrentRoundID == "") {
		out.CurrentRoundID = overlay.CurrentRoundID
	}
	if overlay.CurrentPhase != "" && (out.CurrentPhase == "" || (overlayNewer && freeStatePhaseRank(overlay.CurrentPhase) >= freeStatePhaseRank(out.CurrentPhase))) {
		out.CurrentPhase = overlay.CurrentPhase
	}
	if overlay.PriorityQueue != nil && (overlayNewer || out.PriorityQueue == nil) {
		out.PriorityQueue = overlay.PriorityQueue
	}
	if overlay.ContinuationBudget > out.ContinuationBudget {
		out.ContinuationBudget = overlay.ContinuationBudget
	}
	if overlay.ContinuationUsed > out.ContinuationUsed {
		out.ContinuationUsed = overlay.ContinuationUsed
	}
	if overlay.Cycle > out.Cycle {
		out.Cycle = overlay.Cycle
	}
	if overlay.LatestObservation != nil && (overlayNewer || out.LatestObservation == nil) {
		out.LatestObservation = overlay.LatestObservation
	}
	if overlay.LatestDecision != nil && (overlayNewer || out.LatestDecision == nil) {
		out.LatestDecision = overlay.LatestDecision
	}
	if !overlay.CreatedAt.IsZero() && (out.CreatedAt.IsZero() || overlay.CreatedAt.Before(out.CreatedAt)) {
		out.CreatedAt = overlay.CreatedAt
	}
	if overlay.UpdatedAt.After(out.UpdatedAt) {
		out.UpdatedAt = overlay.UpdatedAt
	}
	out.RequiresPostActionObservation = out.RequiresPostActionObservation || overlay.RequiresPostActionObservation
	out.ObservationIDs = appendUniqueFreeStateStrings(out.ObservationIDs, overlay.ObservationIDs)
	out.ObservationReceipts = mergeFreeStateRowsByKey(out.ObservationReceipts, overlay.ObservationReceipts, "receipt_id")
	out.RejectedObservationRequests = mergeFreeStateRowsByKey(out.RejectedObservationRequests, overlay.RejectedObservationRequests, "fingerprint")
	if len(overlay.Actions) > len(out.Actions) {
		out.Actions = append([]freeStateActionRecord(nil), overlay.Actions...)
	}
	out.ObservationLedger = mergeFreeStateLedgers(out.ObservationLedger, overlay.ObservationLedger)
	return out
}

func freeStateLoopOverlayNewer(base, overlay freeStateReasoningLoop) bool {
	if base.UpdatedAt.IsZero() {
		return true
	}
	if overlay.UpdatedAt.IsZero() {
		return false
	}
	return overlay.UpdatedAt.After(base.UpdatedAt)
}

// cloneFreeStateGapRows deep-copies the structured admission-gap records for
// the durable merge (each row is a plain JSON map).
func cloneFreeStateGapRows(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneContext(row))
	}
	return out
}

func freeStateLoopTerminal(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "cancelled", "blocked", "capability_blocked", "no_candidate_found", "failed", "stopped":
		return true
	default:
		return false
	}
}

func freeStatePhaseRank(phase string) int {
	phase = strings.ToLower(strings.TrimSpace(phase))
	for index, candidate := range []string{
		"fs0_semantic_entry", "fs1_project_bound", "fs2_capacity_assessed", "fs3_project_scan",
		"fs4_diagnostic_round", "fs5_candidate_frontier", "fs6_target_confirmed", "fs7_improvement_proposal",
		"fs8_experiment_verification", "fs9_terminal",
	} {
		if phase == candidate {
			return index
		}
	}
	return -1
}

func mergeFreeStateLedgers(base, overlay map[string]any) map[string]any {
	if len(base) == 0 {
		return cloneContext(overlay)
	}
	out := cloneContext(base)
	if len(overlay) == 0 {
		return out
	}
	out["schema_version"] = freeStateObservationLedgerSchema
	available := firstMapFromAny(out["available_views"])
	if available == nil {
		available = map[string]any{}
	}
	for viewID, row := range firstMapFromAny(overlay["available_views"]) {
		available[viewID] = mergeFreeStateAvailableViewRow(firstMapFromAny(available[viewID]), firstMapFromAny(row))
	}
	if len(available) > 0 {
		out["available_views"] = available
	}
	out["rejected_view_sets"] = mergeFreeStateRowsByKey(freeStateMapRows(out["rejected_view_sets"]), freeStateMapRows(overlay["rejected_view_sets"]), "fingerprint")
	out["receipts"] = mergeFreeStateRowsByKey(freeStateMapRows(out["receipts"]), freeStateMapRows(overlay["receipts"]), "receipt_id")
	for _, key := range []string{"rejected_view_set_count", "receipt_count"} {
		if freeStateLedgerCount(overlay[key], 0) > freeStateLedgerCount(out[key], 0) {
			out[key] = overlay[key]
		}
	}
	if len(freeStateMapRows(out["rejected_view_sets"])) == 0 {
		delete(out, "rejected_view_sets")
	}
	if len(freeStateMapRows(out["receipts"])) == 0 {
		delete(out, "receipts")
	}
	return out
}

// mergeFreeStateAvailableViewRow keeps a decision digest when a transport or
// HTTP-layer row repeats the same observation with fewer fields. A newer
// observation always owns its own conclusion; a conclusion is never carried
// across observation IDs where it could be misattributed.
func mergeFreeStateAvailableViewRow(base, overlay map[string]any) map[string]any {
	if len(base) == 0 {
		return cloneContext(overlay)
	}
	if len(overlay) == 0 {
		return cloneContext(base)
	}
	// A continuation or transport overlay can serialise a catalog row from
	// before the mutation. Evidence identity must not regress: when the base
	// row already observes a strictly higher project revision, the overlay row
	// is pre-mutation residue and must not drag the catalog back (2026-08-29
	// S3h smoke: round-2 proposals kept being handed the round-1 pre-action
	// pointer and were then refused by G7 for quoting it).
	if baseRevision, baseOK := freeStateAvailableViewRevision(base); baseOK {
		if overlayRevision, overlayOK := freeStateAvailableViewRevision(overlay); overlayOK && overlayRevision < baseRevision {
			return cloneContext(base)
		}
	}
	out := cloneContext(base)
	for key, value := range overlay {
		if value != nil {
			out[key] = value
		}
	}
	baseObservationID := firstStringFromMap(base, "observation_id")
	overlayObservationID := firstStringFromMap(overlay, "observation_id")
	if firstMapFromAny(overlay["conclusion"]) == nil &&
		(baseObservationID == "" || overlayObservationID == "" || baseObservationID == overlayObservationID) {
		if conclusion := firstMapFromAny(base["conclusion"]); len(conclusion) > 0 {
			out["conclusion"] = cloneContext(conclusion)
		}
	}
	return out
}

func appendUniqueFreeStateStrings(base, extra []string) []string {
	out := append([]string(nil), base...)
	for _, value := range extra {
		if !freeStateContainsString(out, value) && strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func mergeFreeStateRowsByKey(base, overlay []map[string]any, key string) []map[string]any {
	out := make([]map[string]any, 0, len(base)+len(overlay))
	seen := map[string]bool{}
	for _, rows := range [][]map[string]any{base, overlay} {
		for _, row := range rows {
			identity := strings.TrimSpace(firstStringFromMap(row, key))
			if identity == "" && key == "fingerprint" {
				identity = freeStateNormalizedViewFingerprint(freeStateStringSlice(row["requested_views"]))
			}
			if identity != "" && seen[identity] {
				continue
			}
			if identity != "" {
				seen[identity] = true
			}
			out = append(out, cloneContext(row))
		}
	}
	return out
}

func freeStateTargetRef(ctx map[string]any) map[string]any {
	trackID := firstStringFromMap(ctx, "selected_track_id", "selected_plugin_track_id")
	if trackID == "" {
		return nil
	}
	return map[string]any{
		"kind":       "track",
		"id":         trackID,
		"label":      firstStringFromMap(ctx, "selected_track_name"),
		"track_id":   trackID,
		"track_name": firstStringFromMap(ctx, "selected_track_name"),
	}
}

func (s *Server) storeFreeStateLoop(loop freeStateReasoningLoop) {
	if s == nil || strings.TrimSpace(loop.ConversationID) == "" {
		return
	}
	syncD1Receipt(&loop)
	loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
	// The continuation budget is the explicit ADR §10 scheduling bound; it
	// defaults to the loop's action ceiling so the scheduler Attempt semantics
	// and the loop budget cannot diverge.
	if loop.ContinuationBudget <= 0 && loop.MaxCycles > 0 {
		loop.ContinuationBudget = loop.MaxCycles
	}
	loop = cloneFreeStateLoop(loop)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.freeStateLoops == nil {
		s.freeStateLoops = map[string]freeStateReasoningLoop{}
	}
	s.freeStateLoops[loop.ConversationID] = loop
}

func (s *Server) freeStateLoop(conversationID string) (freeStateReasoningLoop, bool) {
	if s == nil {
		return freeStateReasoningLoop{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	loop, ok := s.freeStateLoops[conversationID]
	return cloneFreeStateLoop(loop), ok
}

func (s *Server) hasActiveFreeStateReasoningLoop(conversationID string) bool {
	loop, ok := s.freeStateLoop(conversationID)
	return ok && freeStateLoopActive(loop)
}

func (s *Server) recordFreeStateAdmissionReceipt(loop *freeStateReasoningLoop, decision *agentloop.FreeStateDecision, context map[string]any) {
	if s == nil || loop == nil || decision == nil {
		return
	}
	status := strings.ToLower(strings.TrimSpace(decision.Status))
	if status != agentloop.FreeStateNeedsExperiment && status != agentloop.FreeStateImprovementProposal &&
		status != agentloop.FreeStateCapabilityBlocked && status != agentloop.FreeStateBlocked {
		return
	}
	audit := agentloop.AuditFreeStateNeedsExperimentGate(context, decision)
	candidateID := ""
	frontierEvidence := ""
	if s.audioClosures != nil {
		if closure, tracked := s.audioClosures.ActiveForConversation(loop.ConversationID); tracked {
			candidateID = strings.TrimSpace(closure.Frontier.CandidateID)
			frontierEvidence = closureTargetEvidence(closure)
		}
	}
	if candidateID == "" && len(loop.ObservationLedger) > 0 {
		for _, observation := range freeStateLedgerObservations(loop.ObservationLedger) {
			if observation == nil {
				continue
			}
			frontier, _ := audioClosureFrontier(audioclosure.HypothesisFrontier{}, agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema}, []*agentloop.RecentObservation{observation})
			if frontier.CandidateID != "" {
				candidateID = frontier.CandidateID
				break
			}
		}
	}
	if candidateID == "" {
		candidateID = firstNonEmpty(firstStringFromMap(loop.TargetRef, "candidate_id"), "")
	}
	targetEvidence := ""
	if decision.ImprovementProposal != nil {
		for _, ref := range decision.ImprovementProposal.EvidenceRefs {
			if strings.TrimSpace(ref) != "" {
				targetEvidence = strings.TrimSpace(ref)
				break
			}
		}
	}
	if frontierEvidence != "" {
		targetEvidence = frontierEvidence
	} else if loop.LatestObservation != nil {
		targetEvidence = firstNonEmpty(firstStringFromMap(loop.LatestObservation.Summary, "observation_id"), targetEvidence)
	}
	gateResults := freeStateAdmissionGateResults(audit.FailedGateIDs)
	boundary := ""
	switch {
	case !audit.Proposal:
		boundary = "proposal_missing"
	case !audit.ProposalValid:
		boundary = "proposal_invalid"
	case len(audit.FailedGateIDs) > 0:
		boundary = "admission_gate_failed"
	case status == agentloop.FreeStateCapabilityBlocked || status == agentloop.FreeStateBlocked:
		boundary = firstNonEmpty(decision.StopReason, "capability_boundary")
	default:
		boundary = "admitted"
	}
	observationBinding := map[string]any{}
	if loop.LatestObservation != nil {
		observationBinding = firstMapFromAny(loop.LatestObservation.Summary["project_binding"])
	}
	loop.AdmissionReceipt = map[string]any{
		"schema_version":      "free_state_admission_receipt.v1",
		"status":              status,
		"boundary":            boundary,
		"candidate_id":        candidateID,
		"target_evidence_ref": targetEvidence,
		"failed_gate_ids":     append([]string(nil), audit.FailedGateIDs...),
		"gate_results":        gateResults,
		"proposal_present":    audit.Proposal,
		"proposal_valid":      audit.ProposalValid,
		"proposal_error":      audit.ProposalError,
		"project_revision":    firstNonEmpty(firstStringFromMap(loop.LatestProjectChange, "project_revision", "revision"), firstStringFromMap(observationBinding, "project_revision")),
		"recorded_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
}

// closureTargetEvidence returns the exact selected-track observation when the
// closure has reached FS6. Candidate source refs identify discovery evidence;
// they are intentionally only a fallback because D1-S1 admission binds to the
// fresh target observation itself.
func closureTargetEvidence(state audioclosure.State) string {
	candidateID := strings.TrimSpace(state.Frontier.CandidateID)
	selectedTracks := map[string]bool{}
	fallback := ""
	for _, candidate := range state.Frontier.Candidates {
		if candidate.ID != candidateID {
			continue
		}
		fallback = firstNonEmpty(candidate.SourceObservationID)
		if fallback == "" && len(candidate.EvidenceRefs) > 0 {
			fallback = strings.TrimSpace(candidate.EvidenceRefs[0])
		}
		for _, trackID := range candidate.TrackIDs {
			selectedTracks[strings.TrimSpace(trackID)] = true
		}
		break
	}
	for _, record := range state.Observations {
		if !selectedTracks[strings.TrimSpace(record.TargetRef)] || strings.TrimSpace(record.ObservationID) == "" {
			continue
		}
		for _, viewID := range record.ViewIDs {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "track.") {
				return record.ObservationID
			}
		}
	}
	return fallback
}

// freeStateAdmissionGateResults keeps the complete G1-G7 result vector
// auditable even when admission stops before a proposal reaches execution.
// The gate evaluator deliberately exposes only failed IDs; this projection
// adds the complementary pass/fail values without inventing evidence.
func freeStateAdmissionGateResults(failed []string) map[string]any {
	failedSet := map[string]bool{}
	for _, id := range failed {
		failedSet[strings.TrimSpace(id)] = true
	}
	results := map[string]any{}
	for _, id := range []string{
		"G1_project_binding", "G2_capacity_assessed", "G3_project_scan",
		"G4_dimension_closed", "G5_frontier_established", "G6_target_evidence",
		"G7_fresh_revision_bound_refs", "G8_target_consistency",
	} {
		results[id] = "pass"
		if failedSet[id] {
			results[id] = "fail"
		}
	}
	return results
}

func (s *Server) recordFreeStateDecision(conversationID string, res agentloop.Result) (freeStateReasoningLoop, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) {
		return loop, ok
	}
	// MessageLoop records the compact ledger in its continuation/context
	// snapshot as soon as each CCB tool returns.  Import that internal state
	// before interpreting the HTTP result so a response envelope that omits
	// history cannot erase evidence needed by the next request.
	if res.Continuation != nil {
		if continued, continuedOK := freeStateLoopFromAny(firstMapFromAny(res.Continuation.Context)["free_state_reasoning_loop"]); continuedOK {
			loop = mergeFreeStateLoops(loop, continued, true)
		}
	}
	if snapshotLoop, snapshotOK := freeStateLoopFromAny(firstMapFromAny(res.ContextSnapshot)["free_state_reasoning_loop"]); snapshotOK {
		loop = mergeFreeStateLoops(loop, snapshotLoop, true)
	}
	observations := freeStateCCBObservations(res)
	var latestUsable *agentloop.RecentObservation
	for _, current := range observations {
		if current == nil {
			continue
		}
		if freeStateUsableObservation(current) {
			latestUsable = current
			loop.LatestObservation = current
		}
		loop.ObservationLedger = mergeFreeStateObservationLedger(loop.ObservationLedger, current, loop.Cycle)
		if rejected := freeStateRejectedObservation(current); len(rejected) > 0 && !freeStateRejectedObservationRecorded(loop.RejectedObservationRequests, rejected) {
			loop.RejectedObservationRequests = append(loop.RejectedObservationRequests, rejected)
		}
		if receipt := firstMapFromAny(current.Summary["audit_receipt"]); len(receipt) > 0 {
			if receiptID := firstStringFromMap(receipt, "receipt_id"); receiptID == "" || !freeStateObservationReceiptRecorded(loop.ObservationReceipts, receiptID) {
				loop.ObservationReceipts = append(loop.ObservationReceipts, cloneContext(receipt))
			}
		}
		if observationID := firstStringFromMap(current.Summary, "observation_id"); observationID != "" &&
			!freeStateContainsString(loop.ObservationIDs, observationID) {
			loop.ObservationIDs = append(loop.ObservationIDs, observationID)
		}
	}
	// A durable continuation can retain the just-executed CCB view only in the
	// compact observation ledger while Result.Executed/RecentObservation still
	// project the previous turn. Rebuild the latest usable observation from the
	// merged ledger so closure candidate selection cannot regress to stale
	// project.structure evidence after a targeted observation completed.
	if durableLatest := freeStateLatestObservationFromLedger(loop.ObservationLedger); durableLatest != nil {
		loop.LatestObservation = durableLatest
		latestUsable = durableLatest
	}
	// A native-tool proposal turn can end in waiting_confirmation with no typed
	// decision while the decision-bearing result envelope omits its CCB
	// observation; either way the fresh observation is the recalibration round's
	// pre-action base and must reach the round or its execution gate has nothing
	// revision-matched to admit against. Self-gated and idempotent.
	s.bookFreeStateRecalibrationRoundBase(&loop, observations)
	if res.FreeStateDecision == nil {
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return loop, true
	}
	decision := *res.FreeStateDecision
	decision.RequestedViewIDs = append([]string(nil), res.FreeStateDecision.RequestedViewIDs...)
	decision.Limitations = append([]string(nil), res.FreeStateDecision.Limitations...)
	// DIAG3-3 retirement latch: a surfaced decision round counts against the
	// notice's two-round window; a proposal-bearing decision retires it
	// immediately. Either way the exhausted-budget honest settle semantics
	// downstream are untouched.
	if len(loop.ObservationSaturationNotice) > 0 {
		loop.ObservationSaturationRounds++
		switch strings.ToLower(strings.TrimSpace(decision.Status)) {
		case agentloop.FreeStateNeedsExperiment, agentloop.FreeStateImprovementProposal, agentloop.FreeStateNeedsAction:
			loop.ObservationSaturationNotice = nil
		default:
			if loop.ObservationSaturationRounds >= freeStateSaturationNoticeMaxRounds {
				loop.ObservationSaturationNotice = nil
			}
		}
	}
	// ContextSnapshot is the compact contextruntime projection and intentionally
	// omits the authoritative free-state closure/ledger fields.  Prefer the
	// continuation's full runtime context for the G1-G7 audit; otherwise a fresh
	// CCB observation is paired with an empty/stale gate snapshot and every gate
	// is reported as failed.
	auditContext := map[string]any{}
	if res.Continuation != nil {
		candidate := cloneContext(res.Continuation.Context)
		if len(firstMapFromAny(candidate["minimal_audio_closure"])) > 0 ||
			len(firstMapFromAny(candidate["free_state_reasoning_loop"])) > 0 {
			auditContext = candidate
		}
	}
	if len(auditContext) == 0 {
		auditContext = cloneContext(res.ContextSnapshot)
	}
	// The server-owned loop and closure are authoritative across scheduler
	// slices. Rebind the audit context from them before evaluating G1-G7; the
	// model continuation may legally carry only a compact/stale overlay.
	auditContext = s.authoritativeFreeStateAdmissionContext(conversationID, auditContext, loop)
	s.recordFreeStateAdmissionReceipt(&loop, &decision, auditContext)
	experimentWasActive := loop.Experiment != nil
	loop.LatestDecision = &decision
	// The durable human-judgment boundary (round decision user_judgment_pending,
	// judgment requested, or judgment evidence recorded) is terminal for model
	// turns: the loop may only settle through the audition judgment path, and a
	// needs_observation/needs_action/new-admission decision must not revive it
	// into repeated post-action observation cycles (2026-08-25 21:09 D1 smoke:
	// the revived loop recorded a second post_action=true observation and burned
	// the remaining continuation budget).
	judgmentBoundary := freeStateJudgmentBoundary(loop)
	if judgmentBoundary && !freeStateJudgmentSettleDecision(decision) {
		loop.Status = "blocked"
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
		loop.LastError = "experiment round is waiting for the human judgment boundary; only settle decisions are admitted"
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] decision status %s ignored at the human judgment boundary for %s", decision.Status, conversationID)
		}
		return loop, true
	}
	if experimentWasActive && !judgmentBoundary {
		for _, current := range observations {
			if loop.RequiresPostActionObservation && !freeStatePostActionObservationEligible(loop, current) {
				// A continuation may replay the pre-action CCB bundle after the
				// mutation. Never bind that old observation as post-action evidence;
				// wait for a new CCB bundle at the mutation's after revision.
				continue
			}
			observation, observationOK := freeStateExperimentObservation(current, loop.RequiresPostActionObservation)
			if !observationOK || freeStateExperimentHasObservation(loop.Experiment, observation.ID) {
				continue
			}
			if events, observationErr := loop.Experiment.RecordObservation(observation, loop.RequiresPostActionObservation, time.Now().UTC()); observationErr == nil {
				s.emitFreeStateExperimentEvents(events)
			} else if s.logger != nil {
				s.logger.Warn("[free-state-experiment] observation rejected: %v", observationErr)
			}
		}
	}
	if res.GoalID != "" {
		loop.GoalID = res.GoalID
	}
	if res.RunID != "" {
		loop.RunID = res.RunID
	}
	if err := s.applyFreeStateDecisionSemantic(&loop, decision); err != nil {
		loop.Status = "blocked"
		loop.LastError = "task semantic transition rejected: " + err.Error()
		blocked := decision
		blocked.Status = agentloop.FreeStateCapabilityBlocked
		blocked.EvidenceStatus = "insufficient"
		blocked.StopReason = "task_semantic_transition_rejected"
		blocked.Limitations = append(blocked.Limitations, loop.LastError)
		loop.LatestDecision = &blocked
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return loop, true
	}
	if decision.ObservationID != "" && !freeStateContainsString(loop.ObservationIDs, decision.ObservationID) {
		loop.ObservationIDs = append(loop.ObservationIDs, decision.ObservationID)
	}
	switch strings.ToLower(strings.TrimSpace(decision.Status)) {
	case agentloop.FreeStateNeedsAction:
		selected, target, resolved := freeStateResolveActionObservation(loop, decision, observations)
		if !resolved {
			// A family decision without an unambiguous cited observation must not
			// reach PCA/controller materialization. Preserve the audit state but
			// fail closed at the orchestration boundary.
			loop.Status = "blocked"
			loop.DecisionPhase = freeStatePhaseProcessorSelection
			loop.LastError = "needs_action requires an unambiguous observation_id/evidence_ref target binding"
			blocked := decision
			blocked.Status = agentloop.FreeStateBlocked
			blocked.EvidenceStatus = "insufficient"
			blocked.StopReason = "free_state_observation_target_unresolved"
			blocked.Limitations = append(blocked.Limitations, loop.LastError)
			loop.LatestDecision = &blocked
			loop.LatestObservation = latestUsable
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			return loop, true
		}
		if selected != nil {
			loop.LatestObservation = selected
		}
		if len(target) > 0 {
			loop.TargetRef = target
		}
		loop.Status = "awaiting_action"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.ActiveIntent = strings.TrimSpace(decision.RemainingIntent)
		if selected != nil {
			loop.RequiresPostActionObservation = false
		}
	case agentloop.FreeStateNeedsExperiment, agentloop.FreeStateImprovementProposal:
		// A needs_experiment status without a model-owned proposal is an
		// admission failure. Never synthesize a proposal from free-form intent:
		// doing so would fabricate the target/evidence handoff and could make the
		// FS6 -> FS7 transition appear authorized without G1-G7 evidence.
		proposalInvalid := decision.ImprovementProposal != nil && decision.ImprovementProposal.Validate() != nil
		// A decision carrying experiment report fields evaluates an already
		// admitted experiment; it is not a new admission. The G1-G7 audit
		// re-checks pre-apply state (frontier, target evidence, revision
		// binding) that legitimately changed after Apply, so auditing the
		// report here rejected every FS8 evaluation and the judgment
		// boundary never became durable (2026-08-25 21:09 D1 smoke: round
		// decision lost, loop revived and recorded a second post-action
		// observation). Mirrors the agentloop output-gate skip. The same
		// boundary covers a proposal-only decision on a running D2-2
		// multi-round experiment: that proposal is the already-admitted
		// experiment's next intra-round intervention (the owed round's), and
		// G1's contract-vs-closure revision equality is structurally false
		// once round 1 applied, so auditing it rejected every round-2+
		// proposal (20260830_085624: capability_blocked
		// free_state_admission_gate_failed, zero round-2 interventions).
		carriesExperimentReport := decision.ExperimentMateriality != nil || decision.ExperimentTargetResponse != nil ||
			strings.TrimSpace(decision.ExperimentRoundDecision) != ""
		var gateAudit agentloop.FreeStateGateAudit
		gateRejected := false
		if !carriesExperimentReport && !freeStateLoopRunningMultiRoundExperiment(&loop) {
			gateAudit = agentloop.AuditFreeStateNeedsExperimentGate(auditContext, &decision)
			gateRejected = len(auditContext) > 0 && !gateAudit.Passed
		}
		if decision.ImprovementProposal == nil || proposalInvalid || gateRejected {
			blocked := decision
			blocked.Status = agentloop.FreeStateCapabilityBlocked
			blocked.EvidenceStatus = "insufficient"
			if decision.ImprovementProposal == nil {
				blocked.StopReason = "free_state_improvement_proposal_missing"
				blocked.Limitations = append(blocked.Limitations, "needs_experiment requires a model-submitted improvement_proposal; no proposal was synthesized")
			} else {
				blocked.StopReason = "free_state_improvement_proposal_invalid"
				if err := decision.ImprovementProposal.Validate(); err != nil {
					blocked.Limitations = append(blocked.Limitations, err.Error())
				}
				if gateRejected {
					blocked.StopReason = "free_state_admission_gate_failed"
					blocked.Limitations = append(blocked.Limitations, gateAudit.FailedGateIDs...)
				}
			}
			loop.Status = "capability_blocked"
			loop.LastError = blocked.StopReason
			loop.LatestDecision = &blocked
			s.recordFreeStateAdmissionReceipt(&loop, &blocked, auditContext)
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			return loop, true
		}
		// The agentloop G1-G7 gate has accepted this decision (it would have
		// been rejected otherwise), which is the evidence that the FS7 guard
		// (GatePassed) holds. Advance the closure spine to FS7/FS8 from the
		// admitted decision.
		if s != nil && s.audioClosures != nil {
			if closure, tracked := s.audioClosures.ActiveForConversation(conversationID); tracked && !closure.Terminal() {
				// The FS2 capacity guard is derived from the durable capability
				// route record (scheduler-driven turns do not carry the HTTP
				// request context), never self-asserted here.
				advanced := s.advanceAudioClosurePhase(closure, audioclosure.PhaseGuardEvidence{
					CapacityAssessed: s.audioClosureCapacityAssessedForConversation(nil, conversationID),
					GatePassed:       true,
					// A valid proposal is the FS6 -> FS7 handoff. Do not also
					// assert AdmissionValid here: AdvancePhase walks forward and
					// would otherwise skip the auditable proposal boundary into
					// FS8 before the experiment admission has been constructed.
					AdmissionValid: false,
				})
				// The local loop is stored again at the end of this method;
				// carry the closure-owned phase into that write so it cannot
				// overwrite the synced FS7 projection with an empty/stale phase.
				if phase, valid := audioclosure.ParsePhase(string(advanced.Phase)); valid {
					loop.CurrentPhase = string(phase)
				}
			}
		}
		proposal := decision.ImprovementProposal
		loop.TargetRef = cloneContext(proposal.Target)
		loop.Status = "awaiting_experiment"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.ActiveIntent = strings.TrimSpace(proposal.ImprovementIntent)
		// A settle report rides the same preserved proposal, but it must not
		// retire the mandatory post-action observation debt: keep the flag until
		// the round actually carries the fresh observation the settle guard in
		// recordFreeStateExperimentDecision requires.
		if !loop.RequiresPostActionObservation || freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment) {
			loop.RequiresPostActionObservation = false
		}
		s.upsertPendingCandidate(proposal.ToPendingCandidate(conversationID, res.GoalID, res.RunID, time.Now().UTC().Format(time.RFC3339Nano)))
	case agentloop.FreeStateNeedsObservation:
		loop.Status = "observing"
		loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
	case agentloop.FreeStateDiagnosticComplete, agentloop.FreeStateSatisfied:
		loop.Status = "completed"
		loop.ActiveIntent = ""
		loop.RequiresPostActionObservation = false
		s.settleDiagnosticTask(&loop, decision)
	case agentloop.FreeStateNoCandidateFound:
		loop.Status = "no_candidate_found"
		loop.ActiveIntent = ""
		loop.RequiresPostActionObservation = false
	case agentloop.FreeStateBlocked, agentloop.FreeStateCapabilityBlocked:
		loop.Status = "blocked"
		loop.LastError = firstNonEmpty(decision.StopReason, strings.Join(decision.Limitations, "; "), decision.Summary)
	}
	if strings.EqualFold(strings.TrimSpace(decision.Status), agentloop.FreeStateNeedsExperiment) ||
		strings.EqualFold(strings.TrimSpace(decision.Status), agentloop.FreeStateImprovementProposal) ||
		(strings.EqualFold(strings.TrimSpace(decision.Status), agentloop.FreeStateNeedsAction) && s.hasTaskSemanticContract(loop.GoalID)) {
		if loop.Experiment == nil {
			if err := s.startFreeStateExperiment(&loop, decision, res.GoalID, res.RunID); err != nil {
				loop.Status = "blocked"
				loop.LastError = "free-state experiment admission failed: " + err.Error()
				blocked := decision
				blocked.Status = agentloop.FreeStateBlocked
				blocked.EvidenceStatus = "insufficient"
				blocked.StopReason = "free_state_experiment_admission_invalid"
				blocked.Limitations = append(blocked.Limitations, loop.LastError)
				loop.LatestDecision = &blocked
			} else {
				// A validated admission is the auditable FS7 -> FS8 boundary.
				// Keep the proposal boundary visible until the experiment runtime
				// exists, then advance the closure host before scheduling any
				// post-action verification continuation.
				if s != nil && s.audioClosures != nil {
					if closure, tracked := s.audioClosures.ActiveForConversation(conversationID); tracked && !closure.Terminal() {
						advanced := s.advanceAudioClosurePhase(closure, audioclosure.PhaseGuardEvidence{
							CapacityAssessed: s.audioClosureCapacityAssessedForConversation(nil, conversationID),
							AdmissionValid:   true,
						})
						if phase, valid := audioclosure.ParsePhase(string(advanced.Phase)); valid {
							loop.CurrentPhase = string(phase)
						}
					}
				}
				s.recordFreeStateExperimentDecision(context.Background(), &loop, decision)
			}
		} else {
			s.recordFreeStateExperimentDecision(context.Background(), &loop, decision)
		}
	} else if (decision.ExperimentMateriality != nil || decision.ExperimentTargetResponse != nil ||
		strings.TrimSpace(decision.ExperimentRoundDecision) != "") && loop.Experiment != nil {
		// Experiment report fields ride on any decision shape, terminal
		// statuses included: a capability_blocked decision carrying
		// experiment_materiality + experiment_round_decision=user_judgment_pending
		// is the documented ambiguous human-judgment settlement. Ingesting
		// reports only under needs_experiment lost exactly that record
		// (2026-08-25 D1 smoke: materiality existed on the decision, the
		// experiment runtime never recorded it).
		s.recordFreeStateExperimentDecision(context.Background(), &loop, decision)
	}
	// Recording user_judgment_pending parks the round at the durable human
	// judgment boundary. The loop must not present as awaiting_experiment or
	// observing afterwards; it waits for the audition judgment path, so any
	// further model turn is refused by the boundary guard above.
	if freeStateJudgmentBoundary(loop) && !freeStateJudgmentSettleDecision(decision) {
		loop.Status = "blocked"
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
		loop.LastError = firstNonEmpty(loop.LastError, "experiment round is waiting for the human judgment boundary")
	}
	if loop.Experiment != nil {
		switch strings.ToLower(strings.TrimSpace(decision.Status)) {
		case agentloop.FreeStateSatisfied:
			outcome := experiment.OutcomeStable
			if round, roundErr := loop.Experiment.CurrentRound(); roundErr == nil && round.TargetResponse != nil && round.TargetResponse.Response == experiment.TargetSufficient {
				outcome = experiment.OutcomeImproved
			}
			s.settleFreeStateExperiment(&loop, outcome, decision.Summary)
		case agentloop.FreeStateBlocked:
			if strings.TrimSpace(decision.ExperimentRoundDecision) != "" {
				// The round decision (e.g. user_judgment_pending) already
				// settled the round through the report ingestion above; a
				// blocked-observation settlement here would overwrite the
				// human-judgment boundary.
				break
			}
			outcome := experiment.OutcomeBlockedObservation
			if strings.Contains(strings.ToLower(firstNonEmpty(decision.StopReason, decision.Summary)), "capability") {
				outcome = experiment.OutcomeBlockedCapability
			}
			s.settleFreeStateExperiment(&loop, outcome, firstNonEmpty(decision.StopReason, decision.Summary))
		}
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return loop, true
}

func freeStatePostActionObservationEligible(loop freeStateReasoningLoop, observation *agentloop.RecentObservation) bool {
	if observation == nil || loop.Experiment == nil {
		return false
	}
	row, ok := freeStateExperimentObservation(observation, true)
	if !ok || !row.Fresh || strings.TrimSpace(row.ProjectRevision) == "" {
		return false
	}
	if latest, err := loop.Experiment.CurrentRound(); err == nil {
		for _, prior := range latest.Observations {
			if prior.ID == row.ID && !prior.PostAction {
				return false
			}
		}
	}
	changeRevision := firstStringFromMap(loop.LatestProjectChange, "project_revision")
	if to := firstMapFromAny(loop.LatestProjectChange["to_project"]); len(to) > 0 {
		changeRevision = firstNonEmpty(firstStringFromMap(to, "project_revision"), changeRevision)
	}
	return changeRevision == "" || strings.EqualFold(row.ProjectRevision, changeRevision)
}

// freeStateJudgmentBoundary reports whether the experiment is durably parked
// at the human-judgment boundary. GLM ruling 3: the boundary persists at
// experiment scope — any round that requested a user judgment without the
// judgment landing parks the whole experiment; the current round additionally
// parks while a recorded judgment or a user_judgment_pending decision awaits
// its settle decision. From this point model turns may only carry
// settle-family round decisions; observation/action/admission decisions must
// not revive the loop.
func freeStateJudgmentBoundary(loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil {
		return false
	}
	switch loop.Experiment.Status {
	case experiment.StatusSettled, experiment.StatusStopped:
		return false
	}
	for _, round := range loop.Experiment.Rounds {
		if round.UserJudgmentRequested && len(round.UserJudgmentEvidence) == 0 {
			return true
		}
	}
	current, err := loop.Experiment.CurrentRound()
	if err != nil {
		return false
	}
	return len(current.UserJudgmentEvidence) > 0 || current.Decision == experiment.DecisionUserJudgment
}

// freeStateJudgmentSettleDecision reports whether a decision carries the
// settle-family round decision that the human-judgment boundary admits
// (retained / rolled_back / stopped-ambiguous). Every other decision shape
// would revive the loop and is refused at the boundary.
func freeStateJudgmentSettleDecision(decision agentloop.FreeStateDecision) bool {
	switch experiment.RoundDecision(strings.TrimSpace(decision.ExperimentRoundDecision)) {
	case experiment.DecisionRetain, experiment.DecisionRollback, experiment.DecisionStopped:
		return true
	}
	return false
}

func (s *Server) authoritativeFreeStateAdmissionContext(conversationID string, fallback map[string]any, loop freeStateReasoningLoop) map[string]any {
	ctx := cloneContext(fallback)
	if len(ctx) == 0 {
		// Legacy/unit callers may intentionally omit admission context; preserve
		// the historical non-gating behavior rather than manufacturing a partial
		// snapshot from the server loop.
		return ctx
	}
	if ctx == nil {
		ctx = map[string]any{}
	}
	// Keep a richer test/request snapshot when the server loop has not yet
	// accumulated any ledger/frontier evidence; production scheduler loops do
	// carry that authoritative ledger and therefore replace the stale overlay.
	authoritativeLoop := freeStateLoopMap(loop)
	if len(firstMapFromAny(authoritativeLoop["observation_ledger"])) > 0 ||
		len(firstMapFromAny(authoritativeLoop["hypothesis_frontier"])) > 0 ||
		len(firstMapFromAny(ctx["free_state_reasoning_loop"])) == 0 {
		ctx["free_state_reasoning_loop"] = authoritativeLoop
	}
	if s != nil && s.audioClosures != nil {
		if closure, ok := s.audioClosures.ActiveForConversation(conversationID); ok {
			ctx["minimal_audio_closure"] = audioClosureStateMap(closure)
			ctx["free_state_phase"] = string(closure.Phase)
			if contract := firstMapFromAny(ctx["task_contract"]); len(contract) == 0 {
				ctx["task_contract"] = map[string]any{"project_uuid": closure.ProjectUUID, "project_revision": closure.ProjectRevision, "kind": "improvement"}
			}
		}
	}
	if route := s.previousCapabilityRoute("", conversationID); route.Assessment != nil {
		ctx[capacityAssessmentContextKey] = *route.Assessment
	}
	return ctx
}

func freeStateLatestObservationFromLedger(ledger map[string]any) *agentloop.RecentObservation {
	available := firstMapFromAny(ledger["available_views"])
	if len(available) == 0 {
		return nil
	}
	type ledgerObservation struct {
		id          string
		round       int
		observedAt  string
		toolCallID  string
		status      string
		targetRef   map[string]any
		freshness   map[string]any
		auditRef    map[string]any
		viewIDs     []string
		evidence    []string
		limitations []string
		views       map[string]any
	}
	grouped := map[string]*ledgerObservation{}
	for key, value := range available {
		row := firstMapFromAny(value)
		if len(row) == 0 {
			continue
		}
		status := strings.ToLower(firstStringFromMap(row, "status"))
		if status != "ready" && status != "partial" && status != "ok" {
			continue
		}
		observationID := firstStringFromMap(row, "observation_id")
		viewID := firstNonEmpty(firstStringFromMap(row, "view_id"), key)
		if observationID == "" || viewID == "" {
			continue
		}
		item := grouped[observationID]
		if item == nil {
			item = &ledgerObservation{id: observationID, status: status, views: map[string]any{}}
			grouped[observationID] = item
		}
		item.round = max(item.round, chatIntValue(row["round"]))
		freshness := firstMapFromAny(row["freshness"])
		observedAt := firstStringFromMap(freshness, "observed_at")
		if observedAt > item.observedAt {
			item.observedAt = observedAt
			item.freshness = cloneContext(freshness)
		}
		item.toolCallID = firstNonEmpty(firstStringFromMap(row, "tool_call_id"), item.toolCallID)
		if status == "partial" {
			item.status = status
		}
		if target := firstMapFromAny(row["target_ref"]); len(target) > 0 {
			item.targetRef = cloneContext(target)
		}
		if audit := firstMapFromAny(row["audit_ref"]); len(audit) > 0 {
			item.auditRef = cloneContext(audit)
		}
		item.viewIDs = freeStateNormalizedViewIDs(append(item.viewIDs, viewID))
		item.evidence = freeStateNormalizedViewIDs(append(item.evidence, freeStateStringSlice(row["evidence_refs"])...))
		item.limitations = freeStateNormalizedViewIDs(append(item.limitations, freeStateStringSlice(row["limitations"])...))
		view := map[string]any{"status": status}
		if conclusion := firstMapFromAny(row["conclusion"]); len(conclusion) > 0 {
			if facts := firstMapFromAny(conclusion["facts"]); len(facts) > 0 {
				view["facts"] = cloneContext(facts)
			}
			if projectionStatus := firstStringFromMap(conclusion, "projection_status"); projectionStatus != "" {
				view["projection_status"] = projectionStatus
			}
		}
		item.views[viewID] = view
	}
	var latest *ledgerObservation
	for _, item := range grouped {
		if latest == nil || item.round > latest.round ||
			(item.round == latest.round && item.observedAt > latest.observedAt) ||
			(item.round == latest.round && item.observedAt == latest.observedAt && item.id > latest.id) {
			latest = item
		}
	}
	if latest == nil {
		return nil
	}
	summary := map[string]any{
		"schema_version":           "ccb_observation_bundle.v1",
		"status":                   latest.status,
		"observation_id":           latest.id,
		"requested_views":          append([]string(nil), latest.viewIDs...),
		"actual_executed_view_ids": append([]string(nil), latest.viewIDs...),
		"views":                    latest.views,
	}
	if latest.round > 0 {
		summary["round"] = latest.round
	}
	if len(latest.targetRef) > 0 {
		summary["target_ref"] = latest.targetRef
	}
	if len(latest.freshness) > 0 {
		summary["freshness"] = latest.freshness
	}
	if len(latest.evidence) > 0 {
		summary["evidence_refs"] = latest.evidence
	}
	if len(latest.limitations) > 0 {
		summary["limitations"] = latest.limitations
	}
	if len(latest.auditRef) > 0 {
		summary["audit_receipt"] = map[string]any{
			"schema_version": firstStringFromMap(latest.auditRef, "receipt_schema"),
			"receipt_id":     firstStringFromMap(latest.auditRef, "receipt_id"),
			"status":         latest.status,
		}
	}
	return &agentloop.RecentObservation{
		ToolCallID: latest.toolCallID, Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
		Status: latest.status, Summary: summary,
	}
}

// freeStateLedgerObservations rebuilds CCB observations from the durable
// available_views projection. A continuation may omit Result.Executed and
// RecentObservation while retaining this ledger; frontier construction must
// therefore consume the ledger as authoritative read-only evidence rather
// than treating the result as an empty observation round.
func freeStateLedgerObservations(ledger map[string]any) []*agentloop.RecentObservation {
	available := firstMapFromAny(ledger["available_views"])
	if len(available) == 0 {
		return nil
	}
	keys := make([]string, 0, len(available))
	for key := range available {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]*agentloop.RecentObservation, 0, len(keys))
	for _, key := range keys {
		row := firstMapFromAny(available[key])
		viewID := firstNonEmpty(firstStringFromMap(row, "view_id"), key)
		if strings.Contains(viewID, "::") {
			viewID = strings.TrimSpace(strings.SplitN(viewID, "::", 2)[1])
		}
		if viewID == "" {
			continue
		}
		summary := cloneContext(row)
		summary["status"] = firstNonEmpty(firstStringFromMap(row, "status", "bundle_status"), "ready")
		summary["observation_id"] = firstStringFromMap(row, "observation_id", "receipt_id")
		summary["requested_views"] = []any{viewID}
		// The ledger stores the compact conclusion alongside freshness and
		// target binding. Re-expose it under views[view].facts so the same
		// candidate projection code handles both live bundles and compact rows.
		conclusion := firstMapFromAny(row["conclusion"])
		facts := firstMapFromAny(conclusion["facts"])
		if len(facts) == 0 {
			facts = firstMapFromAny(row["facts"])
		}
		if len(facts) > 0 {
			summary["views"] = map[string]any{viewID: map[string]any{
				"status": firstNonEmpty(firstStringFromMap(row, "status"), "ready"), "facts": facts,
				"projection_status": firstNonEmpty(firstStringFromMap(conclusion, "projection_status"), "ready"),
			}}
		}
		if len(firstMapFromAny(summary["audit_receipt"])) == 0 {
			summary["audit_receipt"] = map[string]any{
				"receipt_id": firstStringFromMap(row, "receipt_id"),
				"status":     firstNonEmpty(firstStringFromMap(row, "status"), "ready"),
				"freshness":  cloneContext(firstMapFromAny(row["freshness"])),
			}
		}
		if strings.TrimSpace(firstStringFromMap(summary, "observation_id")) == "" {
			continue
		}
		out = append(out, &agentloop.RecentObservation{
			Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
			Status: firstNonEmpty(firstStringFromMap(summary, "status"), "ready"), Summary: summary,
		})
	}
	return out
}

func appendUniqueFreeStateObservations(base, extra []*agentloop.RecentObservation) []*agentloop.RecentObservation {
	seen := map[string]bool{}
	for _, observation := range base {
		if observation == nil {
			continue
		}
		if id := firstStringFromMap(observation.Summary, "observation_id", "receipt_id"); id != "" {
			seen[strings.ToLower(id)] = true
		}
	}
	out := append([]*agentloop.RecentObservation(nil), base...)
	for _, observation := range extra {
		if observation == nil {
			continue
		}
		id := firstStringFromMap(observation.Summary, "observation_id", "receipt_id")
		if id != "" && seen[strings.ToLower(id)] {
			continue
		}
		if id != "" {
			seen[strings.ToLower(id)] = true
		}
		out = append(out, observation)
	}
	return out
}

func freeStateRejectedObservation(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	tool := strings.ToLower(strings.TrimSpace(firstNonEmpty(observation.Tool, observation.CommandName)))
	if tool != "ccb.observation_request" && tool != "ccb_observation_request" {
		return nil
	}
	status := strings.ToLower(firstStringFromMap(observation.Summary, "status", "bundle_status"))
	if status != "rejected" {
		return nil
	}
	audit := firstMapFromAny(observation.Summary["audit_receipt"])
	requested := freeStateNormalizedViewIDs(freeStateStringSlice(observation.Summary["requested_views"]))
	target := freeStateObservationTrackTarget(observation)
	return map[string]any{
		"fingerprint":           freeStateRequestFingerprint(requested, target),
		"observation_id":        firstStringFromMap(observation.Summary, "observation_id"),
		"request_id":            firstStringFromMap(observation.Summary, "request_id"),
		"requested_views":       requested,
		"reasons":               append([]string(nil), freeStateStringSlice(firstNonNil(observation.Summary["omission_reasons"], audit["rejection_reasons"]))...),
		"receipt_id":            firstStringFromMap(audit, "receipt_id"),
		"tool_call_id":          observation.ToolCallID,
		"status":                "rejected",
		"retry_policy":          "do_not_retry",
		"rejection_scope":       firstNonEmpty(firstStringFromMap(observation.Summary, "rejection_scope"), firstStringFromMap(audit, "rejection_scope")),
		"blocking_view_ids":     firstNonNil(observation.Summary["blocking_view_ids"], audit["blocking_view_ids"]),
		"non_blocking_view_ids": firstNonNil(observation.Summary["non_blocking_view_ids"], audit["non_blocking_view_ids"]),
		"target_ref":            target,
	}
}

func freeStateRejectedObservationRecorded(rows []map[string]any, candidate map[string]any) bool {
	want := freeStateRequestFingerprint(freeStateStringSlice(candidate["requested_views"]), firstMapFromAny(candidate["target_ref"]))
	if want == "" {
		return false
	}
	for _, row := range rows {
		if freeStateRequestFingerprint(freeStateStringSlice(row["requested_views"]), firstMapFromAny(row["target_ref"])) == want {
			return true
		}
	}
	return false
}

func freeStateStringSlice(value any) []string {
	rows := []string{}
	switch typed := value.(type) {
	case []string:
		return append(rows, typed...)
	case []any:
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
				rows = append(rows, text)
			}
		}
	default:
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			rows = append(rows, text)
		}
	}
	return rows
}

func freeStateNormalizedViewFingerprint(values []string) string {
	values = freeStateNormalizedViewIDs(values)
	if len(values) == 0 {
		return ""
	}
	digest := sha256.Sum256([]byte(strings.Join(values, "\x1f")))
	return "ccb_views:" + hex.EncodeToString(digest[:8])
}

func freeStateRequestFingerprint(values []string, target map[string]any) string {
	viewFingerprint := freeStateNormalizedViewFingerprint(values)
	if viewFingerprint == "" {
		return ""
	}
	kind := strings.ToLower(firstStringFromMap(target, "kind", "target_kind"))
	id := firstStringFromMap(target, "id", "target_id", "track_id", "clip_id")
	if kind == "" && id == "" {
		return viewFingerprint
	}
	digest := sha256.Sum256([]byte(viewFingerprint + "\x1f" + kind + "\x1f" + id))
	return "ccb_request:" + hex.EncodeToString(digest[:8])
}

func freeStateNormalizedViewIDs(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// freeStateObservationDeliveryReasons returns the delivery-fact reason list a
// live CCB bundle summary carries: the bundle's omission_reasons, falling back
// to the audit receipt's rejection_reasons. This is exactly the source
// freeStateObservationCompactReceipt binds to a ledger receipt row (B13-A here,
// B13-C for the compact shape), so the same trim fact reaches the ledger
// whether it is read from the summary or from the row.
func freeStateObservationDeliveryReasons(summary map[string]any) any {
	return firstNonNil(summary["omission_reasons"], firstMapFromAny(summary["audit_receipt"])["rejection_reasons"])
}

// freeStateViewOmittedByDisclosureBudget reports whether a live CCB bundle
// summary marks exactly this view id as trimmed by the disclosure budget. It
// reuses this package's B13-A predicate (freeStateScanViewOmittedByDisclosureBudget)
// instead of restating its verdict, and mirrors agentloop's B13-D guard so the
// in-flight projection and this authoritative persistence cannot disagree about
// the same trim. The comparison stays mechanical and content-blind: only the
// structural "<view_id>: <marker>" reason shape is read, never view content,
// track identity, processor or dose.
func freeStateViewOmittedByDisclosureBudget(summary map[string]any, viewID string) bool {
	return freeStateScanViewOmittedByDisclosureBudget(
		map[string]any{"rejection_reasons": freeStateObservationDeliveryReasons(summary)}, viewID)
}

func mergeFreeStateObservationLedger(ledger map[string]any, observation *agentloop.RecentObservation, round int) map[string]any {
	ledger = cloneContext(ledger)
	if len(ledger) == 0 {
		ledger = map[string]any{}
	}
	ledger["schema_version"] = freeStateObservationLedgerSchema
	if window := freeStateLedgerCount(ledger["window_round"], 0); round > window {
		ledger["window_round"] = round
	}
	if rejected := freeStateRejectedObservation(observation); len(rejected) > 0 {
		rejected["round"] = round
		rows := freeStateMapRows(ledger["rejected_view_sets"])
		if !freeStateRejectedObservationRecorded(rows, rejected) {
			rows = append(rows, rejected)
			ledger["rejected_view_sets"] = freeStateBoundedRows(rows)
			ledger["rejected_view_set_count"] = freeStateLedgerCount(ledger["rejected_view_set_count"], len(rows)-1) + 1
		}
	}
	if receipt := freeStateObservationCompactReceipt(observation); len(receipt) > 0 {
		receipt["round"] = round
		rows := freeStateMapRows(ledger["receipts"])
		key := firstNonEmpty(firstStringFromMap(receipt, "receipt_id"), firstStringFromMap(receipt, "tool_call_id"))
		if !freeStateCompactReceiptRecorded(rows, key) {
			rows = append(rows, receipt)
			ledger["receipts"] = freeStateBoundedRows(rows)
			ledger["receipt_count"] = freeStateLedgerCount(ledger["receipt_count"], len(rows)-1) + 1
		}
	}
	if !freeStateUsableObservation(observation) {
		return ledger
	}
	available := firstMapFromAny(ledger["available_views"])
	if available == nil {
		available = map[string]any{}
	}
	views := firstMapFromAny(observation.Summary["views"])
	recorded := 0
	for _, viewID := range freeStateNormalizedViewIDs(freeStateStringSlice(observation.Summary["requested_views"])) {
		// B13-D: a view the disclosure budget trimmed was never disclosed — the
		// budget drops it before it reaches the payload, so Summary["views"]
		// carries no entry for it and the status fallback below would restate
		// the bundle's own status for a view that is not in the bundle at all.
		// This is the authoritative persistence of the observation ledger (the
		// agentloop in-flight projection runs first, this merge runs at the HTTP
		// boundary and wins), so without the guard a never-delivered view stays
		// in the model-visible catalog and keeps satisfying the G6 target
		// binding agentloop's write segment no longer fabricates. The verdict is
		// deliberately status-independent: the trim marker alone decides, on any
		// bundle status (B13-C precedent). A trim is not a delivery and cannot
		// retract one: an earlier round that really delivered the view keeps its
		// own row, whose freshness/revision binding still governs G7.
		if freeStateViewOmittedByDisclosureBudget(observation.Summary, viewID) {
			continue
		}
		view := firstMapFromAny(views[viewID])
		viewStatus := firstNonEmpty(firstStringFromMap(view, "status"), firstStringFromMap(observation.Summary, "status"))
		if !strings.EqualFold(viewStatus, "ready") && !strings.EqualFold(viewStatus, "partial") {
			continue
		}
		key := freeStateObservationLedgerViewKey(viewID, observation.Summary)
		available[key] = nonEmptyFreeStateMap(map[string]any{
			"view_id":          viewID,
			"status":           viewStatus,
			"observation_id":   firstStringFromMap(observation.Summary, "observation_id"),
			"tool_call_id":     observation.ToolCallID,
			"freshness":        cloneContext(firstMapFromAny(observation.Summary["freshness"])),
			"project_revision": freeStateObservationProjectRevision(observation.Summary),
			"project_uuid":     firstStringFromMap(firstMapFromAny(observation.Summary["project_binding"]), "project_uuid"),
			"project_epoch":    firstStringFromMap(firstMapFromAny(observation.Summary["project_binding"]), "project_epoch"),
			"limitations":      firstNonNil(view["limitations"], observation.Summary["limitations"]),
			"evidence_refs":    observation.Summary["evidence_refs"],
			"audit_ref":        freeStateObservationAuditRef(observation.Summary),
			"target_ref":       freeStateObservationTrackTarget(observation),
			"round":            round,
			"conclusion": contextruntime.ProjectCCBViewConclusion(observation.Summary, viewID, contextruntime.Options{
				MaxTextRunes: 900, MaxListItems: 8, MaxPreviewBytes: 6 * 1024, SkipPluginSemanticLoad: true,
			}),
		})
		recorded++
	}
	if len(available) > 0 {
		ledger["available_views"] = available
		ledger["view_observation_count"] = freeStateLedgerCount(ledger["view_observation_count"], 0) + recorded
	}
	return ledger
}

// invalidateFreeStateObservationLedger marks observations from the previous
// project state as stale after a mutation. The old rows remain in the ledger
// for audit/comparison, but cannot support a current action until a CCB receipt
// for the new state replaces them.
func invalidateFreeStateObservationLedger(ledger map[string]any, change map[string]any) map[string]any {
	ledger = cloneContext(ledger)
	available := firstMapFromAny(ledger["available_views"])
	if len(available) == 0 {
		return ledger
	}
	changeID := firstStringFromMap(change, "change_id")
	scopes := freeStateStringSlice(change["affected_scopes"])
	for key, raw := range available {
		row := firstMapFromAny(raw)
		viewID := firstStringFromMap(row, "view_id")
		if viewID == "project.change_delta" {
			continue
		}
		if len(scopes) > 0 && !freeStateViewAffectedByChange(viewID, scopes) {
			continue
		}
		row["status"] = "stale"
		row["freshness"] = map[string]any{"status": "stale", "reason": "project_change_refresh_required"}
		row["stale_reason"] = "project_change_refresh_required"
		if changeID != "" {
			row["invalidated_by_change_id"] = changeID
		}
		available[key] = row
	}
	ledger["available_views"] = available
	if changeID != "" {
		ledger["invalidated_by_change_id"] = changeID
	}
	return ledger
}

// freeStateAvailableViewRevision returns a catalog row's project revision as
// an integer plus a presence flag. Rows without a revision binding (or with a
// non-numeric one) keep the historical overlay-wins merge.
func freeStateAvailableViewRevision(row map[string]any) (int, bool) {
	revision := firstNonEmpty(
		firstStringFromMap(row, "project_revision"),
		firstStringFromMap(firstMapFromAny(row["freshness"]), "project_revision"),
	)
	if revision == "" {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimSpace(revision))
	if err != nil {
		return 0, false
	}
	return value, true
}

func freeStateViewAffectedByChange(viewID string, scopes []string) bool {
	viewID = strings.ToLower(strings.TrimSpace(viewID))
	for _, scope := range scopes {
		scope = strings.ToLower(strings.TrimSpace(scope))
		switch {
		case scope == "project.state":
			return true
		case scope == "track.level" && (viewID == "track.basic_energy" || strings.HasPrefix(viewID, "mix.")):
			return true
		case scope == "track.stereo_space" && (viewID == "track.stereo_space" || strings.HasPrefix(viewID, "mix.")):
			return true
		case scope == "project.headroom" && viewID == "mix.multitrack_relationship":
			return true
		case scope == "project.structure" && viewID == "project.structure":
			return true
		case scope == "processor.identity_and_controls" && viewID == "processor.identity_and_controls":
			return true
		case scope == "processor.behavior" && viewID == "processor.behavior":
			return true
		case scope == "processor.change_delta" && viewID == "processor.change_delta":
			return true
		case scope == "comparison.before_after" && viewID == "comparison.before_after":
			return true
		case scope == viewID:
			return true
		}
	}
	return false
}

func freeStateObservationLedgerViewKey(viewID string, summary map[string]any) string {
	target := firstMapFromAny(summary["target_ref"])
	kind := strings.ToLower(firstStringFromMap(target, "kind", "target_kind"))
	id := firstStringFromMap(target, "id", "target_id", "track_id")
	if kind == "track" && id != "" {
		return "track:" + id + "::" + viewID
	}
	return viewID
}

func freeStateUsableObservation(observation *agentloop.RecentObservation) bool {
	if observation == nil {
		return false
	}
	status := strings.ToLower(firstStringFromMap(observation.Summary, "status", "bundle_status"))
	return status == "ready" || status == "partial"
}

// freeStateObservationProjectRevision extracts the CCB bundle's authoritative
// project_binding revision so ledger rows stay revision-bound (gate G7).
func freeStateObservationProjectRevision(summary map[string]any) string {
	return firstNonEmpty(
		firstStringFromMap(summary, "project_revision"),
		firstStringFromMap(firstMapFromAny(summary["project_binding"]), "project_revision"),
		firstStringFromMap(firstMapFromAny(summary["freshness"]), "project_revision"),
		firstStringFromMap(firstMapFromAny(firstMapFromAny(summary["audit_receipt"])["freshness"]), "project_revision"),
	)
}

func freeStateObservationCompactReceipt(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	audit := firstMapFromAny(observation.Summary["audit_receipt"])
	return nonEmptyFreeStateMap(map[string]any{
		"receipt_id":       firstStringFromMap(audit, "receipt_id"),
		"receipt_schema":   firstStringFromMap(audit, "schema_version"),
		"tool_call_id":     observation.ToolCallID,
		"observation_id":   firstStringFromMap(observation.Summary, "observation_id"),
		"request_id":       firstStringFromMap(observation.Summary, "request_id"),
		"status":           firstStringFromMap(observation.Summary, "status", "bundle_status"),
		"requested_views":  freeStateNormalizedViewIDs(freeStateStringSlice(observation.Summary["requested_views"])),
		"project_revision": freeStateObservationProjectRevision(observation.Summary),
		"project_uuid":     firstStringFromMap(firstMapFromAny(observation.Summary["project_binding"]), "project_uuid"),
		"project_epoch":    firstStringFromMap(firstMapFromAny(observation.Summary["project_binding"]), "project_epoch"),
		"freshness":        cloneContext(firstMapFromAny(observation.Summary["freshness"])),
		"limitations":      observation.Summary["limitations"],
		"evidence_refs":    observation.Summary["evidence_refs"],
		// B13-C: the disclosure-budget delivery fact must survive into this
		// ledger row, exactly as B13-A had to add it to agentloop's
		// freeStateObservationLedgerReceipt. The compact CCB bundle summary does
		// not carry audit_receipt (agentloop/message_loop.go
		// messageLoopCCBObservationSummary selects omission_reasons, not
		// audit_receipt), so the bundle's omission_reasons is the live source and
		// the full receipt is the durable fallback. Rows without any omission
		// keep their previous shape: compactSelectedKeys-style emptiness pruning
		// drops the key.
		"rejection_reasons": firstNonNil(observation.Summary["omission_reasons"], audit["rejection_reasons"]),
	})
}

func freeStateObservationAuditRef(summary map[string]any) map[string]any {
	audit := firstMapFromAny(summary["audit_receipt"])
	return nonEmptyFreeStateMap(map[string]any{
		"receipt_id":     firstStringFromMap(audit, "receipt_id"),
		"receipt_schema": firstStringFromMap(audit, "schema_version"),
		"bundle_id":      firstStringFromMap(summary, "bundle_id"),
	})
}

func freeStateCompactReceiptRecorded(rows []map[string]any, key string) bool {
	if strings.TrimSpace(key) == "" {
		return false
	}
	for _, row := range rows {
		if firstNonEmpty(firstStringFromMap(row, "receipt_id"), firstStringFromMap(row, "tool_call_id")) == key {
			return true
		}
	}
	return false
}

func freeStateBoundedRows(rows []map[string]any) []map[string]any {
	if len(rows) > freeStateObservationLedgerLimit {
		return rows[len(rows)-freeStateObservationLedgerLimit:]
	}
	return rows
}

func freeStateLedgerCount(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func nonEmptyFreeStateMap(row map[string]any) map[string]any {
	for key, value := range row {
		if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" || fmt.Sprint(value) == "[]" || fmt.Sprint(value) == "map[]" {
			delete(row, key)
		}
	}
	return row
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func freeStateObservationTrackTarget(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	target := firstMapFromAny(observation.Summary["target_ref"])
	kind := strings.ToLower(firstStringFromMap(target, "kind", "target_kind"))
	trackID := firstStringFromMap(target, "track_id")
	if kind == "track" {
		trackID = firstNonEmpty(trackID, firstStringFromMap(target, "id", "target_id"))
	}
	if trackID == "" {
		return nil
	}
	trackName := firstStringFromMap(target, "track_name", "label", "name")
	return map[string]any{
		"kind":       "track",
		"id":         trackID,
		"label":      trackName,
		"track_id":   trackID,
		"track_name": trackName,
		"source":     "ccb_observation_binding",
	}
}

// freeStateResolveActionObservation binds a model family decision to the
// exact observation that supports it. The durable loop may contain compact
// receipts for many observations, but only the cited observation is promoted
// to the downstream PCA/controller context.
func freeStateResolveActionObservation(loop freeStateReasoningLoop, decision agentloop.FreeStateDecision, current []*agentloop.RecentObservation) (*agentloop.RecentObservation, map[string]any, bool) {
	pool := make([]*agentloop.RecentObservation, 0, len(current)+1)
	seen := map[string]bool{}
	add := func(observation *agentloop.RecentObservation) {
		if !freeStateUsableObservation(observation) {
			return
		}
		id := firstStringFromMap(observation.Summary, "observation_id")
		key := firstNonEmpty(id, observation.ToolCallID)
		if key != "" && seen[key] {
			return
		}
		if key != "" {
			seen[key] = true
		}
		pool = append(pool, observation)
	}
	for _, observation := range current {
		add(observation)
	}
	add(loop.LatestObservation)
	if len(pool) == 0 {
		if target := freeStateNormalizedTrackTarget(loop.TargetRef); len(target) > 0 {
			return nil, target, true
		}
		return nil, nil, false
	}
	refs := []string(nil)
	if decision.SemanticProcessorIntent != nil {
		refs = append(refs, decision.SemanticProcessorIntent.EvidenceRefs...)
	}
	wantID := strings.TrimSpace(decision.ObservationID)
	var matches []*agentloop.RecentObservation
	for _, observation := range pool {
		id := firstStringFromMap(observation.Summary, "observation_id")
		matched := wantID != "" && id == wantID
		if !matched {
			for _, ref := range refs {
				ref = strings.TrimSpace(ref)
				if ref == id || strings.TrimPrefix(ref, "mix.observe:") == id || strings.TrimPrefix(ref, "ccb.observe:") == id {
					matched = true
					break
				}
			}
		}
		if matched {
			matches = append(matches, observation)
		}
	}
	if len(matches) != 1 {
		// A single usable observation is unambiguous even when an older client
		// omitted the explicit reference. Multi-observation loops never get this
		// fallback because observation order is not execution authority.
		if wantID == "" && len(refs) == 0 && len(pool) == 1 {
			matches = pool
		} else {
			return nil, nil, false
		}
	}
	selected := matches[0]
	target := freeStateObservationTrackTarget(selected)
	if len(target) == 0 {
		target = freeStateNormalizedTrackTarget(loop.TargetRef)
	}
	if len(target) == 0 {
		return nil, nil, false
	}
	return selected, target, true
}

func freeStateNormalizedTrackTarget(target map[string]any) map[string]any {
	kind := strings.ToLower(firstStringFromMap(target, "kind", "target_kind"))
	trackID := firstStringFromMap(target, "track_id")
	if kind == "track" {
		trackID = firstNonEmpty(trackID, firstStringFromMap(target, "id", "target_id"))
	}
	if trackID == "" {
		return nil
	}
	trackName := firstStringFromMap(target, "track_name", "label", "name")
	return nonEmptyFreeStateMap(map[string]any{
		"kind": "track", "id": trackID, "label": trackName,
		"track_id": trackID, "track_name": trackName,
		"source": firstNonEmpty(firstStringFromMap(target, "source"), "free_state_target_binding"),
	})
}

func (s *Server) bindFreeStateAuthoritativeTrack(conversationID string, requestContext map[string]any) map[string]any {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK && freeStateLoopActive(recovered) {
			loop, ok = recovered, true
		}
	}
	if !ok || !freeStateLoopActive(loop) {
		return requestContext
	}
	trackID := firstStringFromMap(loop.TargetRef, "track_id")
	if strings.EqualFold(firstStringFromMap(loop.TargetRef, "kind"), "track") {
		trackID = firstNonEmpty(trackID, firstStringFromMap(loop.TargetRef, "id"))
	}
	if trackID == "" {
		return requestContext
	}
	trackName := firstStringFromMap(loop.TargetRef, "track_name", "label", "name")
	out := mergeContext(requestContext, map[string]any{
		"selected_track_id":   trackID,
		"selected_track_name": trackName,
	})
	if pluginTrackID := firstStringFromMap(out, "selected_plugin_track_id"); pluginTrackID != "" && pluginTrackID != trackID {
		delete(out, "selected_plugin_id")
		delete(out, "selected_plugin_name")
		delete(out, "selected_plugin_track_id")
	}
	return out
}

func freeStateMaterializationRetry(loop freeStateReasoningLoop) (string, agentloop.Result, bool) {
	if !freeStateLoopActive(loop) || !strings.EqualFold(loop.Status, "awaiting_action") ||
		loop.LatestDecision == nil || !strings.EqualFold(loop.LatestDecision.Status, agentloop.FreeStateNeedsAction) {
		return "", agentloop.Result{}, false
	}
	decision := *loop.LatestDecision
	decision.RequestedViewIDs = append([]string(nil), loop.LatestDecision.RequestedViewIDs...)
	decision.Limitations = append([]string(nil), loop.LatestDecision.Limitations...)
	return firstNonEmpty(decision.RemainingIntent, loop.ActiveIntent, loop.OriginalIntent), agentloop.Result{
		GoalID:            loop.GoalID,
		RunID:             loop.RunID,
		Status:            agentruntime.StatusWaitingContinue,
		StopReason:        agentloop.StopReasonTransientLLMError,
		GoalSummary:       loop.OriginalIntent,
		RecentObservation: loop.LatestObservation,
		FreeStateDecision: &decision,
	}, true
}

func freeStateTransientServiceError(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"context deadline exceeded", "timeout", "timed out", "awaiting headers",
		"500", "502", "503", "504", "internal server error", "temporary", "stream returned no output", "connection reset",
		"wsarecv", "failed to respond", "connection attempt failed", "network is unreachable",
		"runtime configuration state unavailable", "\u8fd0\u884c\u65f6\u914d\u7f6e\u72b6\u6001\u4e0d\u53ef\u7528",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *Server) makeFreeStateMaterializationResumable(conversationID string, requestContext map[string]any,
	res agentloop.Result, response ChatResponse) ChatResponse {
	if !freeStateRouteAuthorized(requestContext) || response.GoalStatus != string(agentruntime.StatusFailed) ||
		!freeStateTransientServiceError(response.Error) {
		return response
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) || !strings.EqualFold(loop.Status, "awaiting_action") {
		return response
	}
	originalError := response.Error
	response.GoalStatus = string(agentruntime.StatusWaitingContinue)
	response.StopReason = agentloop.StopReasonTransientLLMError
	response.Error = ""
	response.Reply = "处理器选择或物化阶段遇到临时模型服务错误。原始意图、CCB 观察和待处理动作均已保留；任务尚未完成，可以继续重试这一阶段。"
	response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{
		"status": "paused_transient", "resumable": true, "transient_error": originalError, "mutation_performed": false,
	})
	goalID := firstNonEmpty(response.GoalID, res.GoalID, loop.GoalID)
	if s != nil && s.harness != nil && goalID != "" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
	}
	if s != nil && conversationID != "" && goalID != "" {
		s.mu.Lock()
		s.conversationGoals[conversationID] = goalID
		s.mu.Unlock()
	}
	return response
}

func (s *Server) resumeFreeStateMaterialization(ctx context.Context, conversationID, mode string,
	requestContext map[string]any, cfg config.EngineConfig) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	userText, res, resumable := freeStateMaterializationRetry(loop)
	if !ok || !resumable {
		return ChatResponse{}, false
	}
	bound := mergeContext(requestContext, map[string]any{
		"free_state_route_authorized": true,
		"free_state_internal_resume":  true,
		"free_state_processor_type":   strings.ToLower(strings.TrimSpace(loop.LatestDecision.ProcessorType)),
		"free_state_reasoning_loop":   freeStateLoopMap(loop),
		"goal_id":                     loop.GoalID,
		"run_id":                      loop.RunID,
	})
	if intent := freeStateProcessorIntentMap(loop.LatestDecision); len(intent) > 0 {
		bound["free_state_semantic_processor_intent"] = intent
	}
	response, routed := s.routeOrdinaryAgentTreatmentStrategy(ctx, conversationID, mode, userText, bound, res, cfg)
	if !routed {
		return ChatResponse{}, false
	}
	return s.makeFreeStateMaterializationResumable(conversationID, bound, res, response), true
}

func freeStateContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func freeStateRouteAuthorized(ctx map[string]any) bool {
	if !contextBool(ctx, "free_state_route_authorized") || !freeStateLoopActiveContext(ctx) {
		return false
	}
	if decision, verified := semanticEntryDecisionFromContext(ctx); verified {
		if decision.Route != semanticEntryRouteOpenSemantic || decision.ControlMode != semanticEntryControlSemanticLoop || decision.UserAuthorization != semanticEntryAuthorizationAction {
			return false
		}
		switch decision.TargetScope {
		case semanticEntryScopeCurrentSelection:
			return contextHasAnyValue(ctx, "selected_track_id", "selected_scene_track_id", "selected_plugin_track_id", "selected_clip_id", "piano_roll_focus_clip_id") || len(contextStringSlice(ctx["selected_clip_ids"])) > 0
		case semanticEntryScopeProjectContext:
			return true
		default:
			return false
		}
	}
	// Persisted loops created before semantic_entry_decision.v1 may continue
	// under their existing governed authorization, but they cannot create a new
	// free-state entry without the model-owned decision above.
	return true
}

func freeStateLoopActiveContext(ctx map[string]any) bool {
	loop, ok := freeStateLoopFromAny(ctx["free_state_reasoning_loop"])
	return ok && freeStateLoopActive(loop)
}

func (s *Server) bindFreeStateContextToResponse(resp ChatResponse, requestContext map[string]any) ChatResponse {
	loop, ok := s.freeStateLoop(resp.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
			s.storeFreeStateLoop(loop)
		}
	}
	if !ok {
		return resp
	}
	if reason, blocked := freeStateMaterializationBlockReason(resp); blocked && freeStateLoopActive(loop) {
		loop.Status = "blocked"
		loop.LastError = reason
		loop.RequiresPostActionObservation = false
		loop.LatestDecision = &agentloop.FreeStateDecision{
			SchemaVersion:  agentloop.FreeStateDecisionSchema,
			Status:         agentloop.FreeStateBlocked,
			EvidenceStatus: "insufficient",
			Summary:        reason,
			StopReason:     firstNonEmpty(resp.StopReason, "semantic_treatment_capability_boundary"),
			Limitations:    []string{reason},
		}
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
	}
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	ctx := cloneContext(firstMapFromAny(resp.WorkflowData["request_context"]))
	ctx = mergeContext(ctx, requestContext)
	ctx["free_state_reasoning_loop"] = freeStateLoopMap(loop)
	resp.WorkflowData["request_context"] = ctx
	resp.WorkflowData["free_state_reasoning_loop"] = freeStateLoopMap(loop)
	return resp
}

func freeStateEvidenceRefreshReason(resp ChatResponse) (string, bool) {
	if !strings.EqualFold(strings.TrimSpace(resp.Workflow), semanticCompressorExecutionWorkflow) ||
		!strings.EqualFold(firstStringFromMap(resp.WorkflowData, "status"), "materialization_rejected") {
		return "", false
	}
	execution := firstMapFromAny(resp.WorkflowData["execution"])
	detail := firstStringFromMap(execution, "message", "failure_code")
	text := strings.ToLower(firstNonEmpty(resp.Error, detail, resp.StopReason))
	if !strings.Contains(text, "com evidence") && !strings.Contains(text, "paired_io") {
		return "", false
	}
	return firstNonEmpty(detail, resp.Reply, resp.StopReason, "fresh paired COM evidence is required"), true
}

func freeStateMaterializationBlockReason(resp ChatResponse) (string, bool) {
	workflow := strings.ToLower(strings.TrimSpace(resp.Workflow))
	status := strings.ToLower(firstStringFromMap(resp.WorkflowData, "status"))
	blocked := false
	switch workflow {
	case semanticTreatmentWorkflow:
		blocked = status == "capability_boundary" || status == "qualification_failed"
	case semanticDynamicWorkflow:
		blocked = status == "rejected" || status == "failed"
	case semanticCompressorExecutionWorkflow:
		if _, recoverable := freeStateEvidenceRefreshReason(resp); recoverable {
			return "", false
		}
		blocked = status == "materialization_rejected"
	case "capability_runtime_v1":
		blocked = status == "rejected" && strings.EqualFold(firstStringFromMap(resp.WorkflowData, "stage"), "preflight_rejected")
	}
	if !blocked {
		return "", false
	}
	detail := firstStringFromMap(firstMapFromAny(resp.WorkflowData["execution"]), "message", "failure_code")
	reason := firstNonEmpty(resp.Error, detail, resp.Reply, resp.StopReason, "the selected processor path reached a governed materialization boundary")
	return reason, true
}

func (s *Server) maybeContinueFreeStateAfterInteraction(ctx context.Context, interaction PendingInteraction, resp ChatResponse, decision string) ChatResponse {
	var closureState audioclosure.State
	var closureTracked bool
	resp, closureState, closureTracked = s.recordAudioClosureCapabilityResponse(interaction.ConversationID, resp)
	if closureTracked && closureState.Terminal() {
		return resp
	}
	loop, ok := s.freeStateLoop(interaction.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(interaction.RequestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
			s.storeFreeStateLoop(loop)
		}
	}
	if !ok || !freeStateLoopActive(loop) {
		return resp
	}
	if freeStateJudgmentBoundary(loop) {
		// The experiment round is durably parked at the human-judgment boundary.
		// Answering a stale interaction (for example re-answering the already
		// executed mix-tick confirmation) must not revive the loop into another
		// observation/action cycle; only the audition judgment path settles the
		// round (2026-08-25 21:09 D1 smoke: re-answered mix tick revived the
		// loop, which recorded a second post-action observation).
		if s.logger != nil {
			s.logger.Warn("[free-state-experiment] interaction %s answer does not resume the loop at the human judgment boundary for %s", interaction.ID, interaction.ConversationID)
		}
		return resp
	}
	if isFreeStateCancellation(decision, resp) {
		s.stopFreeStateExperiment(&loop, "user cancelled the pending processor action")
		loop.Status = "cancelled"
		loop.LastError = "user cancelled the pending processor action"
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	if reason, recoverable := freeStateEvidenceRefreshReason(resp); recoverable {
		loop.Status = "awaiting_action"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.LastError = reason
		loop.RequiresPostActionObservation = false
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
		resp.StopReason = "free_state_com_evidence_refresh_required"
		resp.Error = ""
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
			"resumable": true, "free_state_status": "awaiting_evidence_refresh", "mutation_performed": false,
		})
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	processorType, actionStatus, receipt, attempted := freeStateAcousticActionOutcome(interaction, resp)
	if !attempted || resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) {
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	loop.Cycle++
	loop.Actions = append(loop.Actions, freeStateActionRecord{
		Cycle:         loop.Cycle,
		ProcessorType: processorType,
		Workflow:      resp.Workflow,
		Status:        actionStatus,
		Summary:       firstNonEmpty(resp.StopReason, resp.Reply),
		Receipt:       receipt,
		RecordedAt:    time.Now().UTC(),
	})
	s.recordFreeStateExperimentAction(&loop, processorType, actionStatus, receipt)
	// The post-action observation debt only stands while the round still lacks
	// its fresh post-action evidence: a D1 applied respond books it
	// deterministically (bookD1PostActionObservation), and resetting the flag
	// unconditionally here would force the settle turns back into an
	// observation loop the gates cannot satisfy twice.
	loop.RequiresPostActionObservation = actionStatus == "applied" &&
		!(loop.Experiment != nil && loop.Experiment.Admission.IsD1S1() && freeStateExperimentRoundHasFreshPostActionObservation(loop.Experiment))
	if loop.RequiresPostActionObservation {
		reservePostActionObservationSlice(&loop)
	}
	loop.Status = "re_evaluating"
	loop.DecisionPhase = freeStatePhaseProcessorSelection
	if loop.RequiresPostActionObservation {
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
	}
	loop.LastError = ""
	if actionStatus != "applied" {
		loop.LastError = firstNonEmpty(resp.Error, resp.StopReason, "processor action failed")
	}
	if actionStatus == "applied" {
		change, authoritative := s.freeStatePostActionProjectChange(ctx)
		loop.LatestProjectChange = cloneContext(change)
		loop.ObservationLedger = invalidateFreeStateObservationLedger(loop.ObservationLedger, change)
		if s.harness != nil && len(change) > 0 && !authoritative {
			loop.Status = "observing"
			loop.DecisionPhase = freeStatePhasePostActionEvaluation
			loop.LastError = "post-action project refresh is not authoritative yet"
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
			resp.StopReason = "free_state_project_refresh_pending"
			resp.Error = ""
			resp.Reply = "工程动作已提交，但当前工程状态尚未取得权威刷新；不会使用旧观察继续判断。"
			resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
				"free_state_project_change": cloneContext(change),
				"state_refresh":             "pending_authoritative_snapshot",
			})
			return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
		}
	}
	if loop.MaxCycles <= 0 || loop.MaxCycles > freeStateMaxActionCount {
		loop.MaxCycles = freeStateDefaultMaxCycles
	}
	if loop.Cycle >= loop.MaxCycles {
		loop.Status = "blocked"
		loop.LastError = fmt.Sprintf("free-state reasoning reached its %d-action safety limit", loop.MaxCycles)
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
		resp.StopReason = "free_state_cycle_limit"
		resp.Error = ""
		resp.Reply = "The processor reasoning loop reached its bounded action limit. The original intent remains recorded, but no further automatic action was started."
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	if freeStateContinuationBudgetExhausted(loop) {
		if !loop.TerminalTurnLocked && freeStateTerminalTurnGuarded(loop) {
			// BOUNDARY-1 总则 1: the exhaustion settle is the fallback that
			// runs only after the terminal turn failed. Reserve exactly one
			// terminal checkpoint (scheduling floor only, same mechanism as
			// reserveD1PostApplySlices) instead of settling an unlocked
			// observation loop.
			loop.ContinuationBudget = loop.ContinuationUsed
			lockFreeStateTerminalTurn(&loop, FreeStateTerminalReasonLastCheckpoint)
		} else {
			loop.Status = "blocked"
			loop.LastError = fmt.Sprintf("free-state reasoning exhausted its %d-continuation budget", loop.ContinuationBudget)
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
			resp.StopReason = freeStateContinuationBudgetExhaustedReason
			resp.Error = ""
			resp.Reply = "The free-state reasoning loop exhausted its bounded continuation budget. The original intent and the evidence gathered so far remain recorded; no further automatic continuation was started."
			return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
		}
	}
	// ContinuationUsed is accounted at the durable checkpoint boundary in
	// recordGoalResult. Do not increment here as well; doing so double-counts
	// the interaction resume and can consume the budget before the mandatory
	// post-action observation slice runs.
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		if err == nil {
			err = fmt.Errorf("AI configuration is incomplete")
		}
		loop.Status = "blocked"
		loop.LastError = err.Error()
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	resumeContext := mergeContext(interaction.RequestContext, map[string]any{
		"conversation_id":                   interaction.ConversationID,
		"goal_id":                           firstNonEmpty(interaction.GoalID, loop.GoalID),
		"run_id":                            firstNonEmpty(interaction.RunID, loop.RunID),
		"free_state_internal_resume":        true,
		"free_state_reasoning_loop":         freeStateLoopMap(loop),
		"free_state_latest_action_evidence": freeStateActionMap(loop.Actions[len(loop.Actions)-1]),
		"requires_post_action_observation":  loop.RequiresPostActionObservation,
		"free_state_project_change":         cloneContext(loop.LatestProjectChange),
	})
	if loop.LatestDecision != nil {
		if intent := freeStateProcessorIntentMap(loop.LatestDecision); len(intent) > 0 {
			resumeContext["free_state_semantic_processor_intent"] = intent
		}
	}
	resumed, handled := s.runAgentLoopChat(ctx, interaction.ConversationID, ChatRequest{
		ConversationID: interaction.ConversationID,
		Message:        loop.OriginalIntent,
		Context:        resumeContext,
	}, cfg)
	if !handled {
		return s.bindFreeStateContextToResponse(resp, resumeContext)
	}
	return resumed
}

func reservePostActionObservationSlice(loop *freeStateReasoningLoop) {
	if loop == nil || loop.PostActionObservationReserved || !loop.RequiresPostActionObservation || loop.ContinuationBudget <= 0 || loop.ContinuationUsed+1 < loop.ContinuationBudget {
		return
	}
	// One observation-only slice is mandatory after Apply; this does not permit
	// another forward action because the post-action gate still requires fresh
	// CCB evidence before any action decision.
	loop.ContinuationBudget++
	loop.PostActionObservationReserved = true
}

// freeStateD1PostApplySliceNeed is the mandatory post-apply slice count the D1
// applied boundary must reserve: one fresh post-action CCB observation slice
// plus the materiality/target evaluation slices that settle the round at the
// human judgment boundary. Pre-apply drains (observation, admission, proposal,
// execution) legitimately consume the general budget, but they must not eat the
// post-apply chain (2026-08-28 09:30 smoke: applied at 4/6, three scheduler
// enqueues burned the budget to 7/6 with no reserve on that path, blocking
// before any fresh bundle landed).
const freeStateD1PostApplySliceNeed = 3

// reserveD1PostApplySlices grants the phase-scoped D1 post-apply budget floor
// at the applied boundary: continuation_budget is raised to used+need once per
// applied round. It changes scheduling only — the post-action evidence gates
// (revision-bound eligibility, explicit-fresh CCB verification) are untouched,
// so the extra slices can still only carry governed turns.
func reserveD1PostApplySlices(loop *freeStateReasoningLoop) {
	if loop == nil || loop.PostApplyBudgetReserved || !loop.RequiresPostActionObservation || loop.ContinuationBudget <= 0 {
		return
	}
	if floor := loop.ContinuationUsed + freeStateD1PostApplySliceNeed; floor > loop.ContinuationBudget {
		loop.ContinuationBudget = floor
	}
	loop.PostApplyBudgetReserved = true
}

// postActionProjectChange establishes the shared authoritative-refresh
// barrier for every mutation owner. It deliberately contains no free-state
// policy: callers decide whether a fresh snapshot resumes a loop or reaches a
// capability terminal record.
func (s *Server) postActionProjectChange(ctx context.Context, refreshSource string) (map[string]any, bool) {
	if s == nil || s.harness == nil {
		return nil, true
	}
	state := s.harness.StateSummary(ctx)
	change := firstMapFromAny(state["latest_change"])
	if strings.EqualFold(firstStringFromMap(change, "freshness"), "current_snapshot") {
		return change, true
	}
	s.harness.RefreshShadow(ctx, refreshSource)
	state = s.harness.StateSummary(ctx)
	change = firstMapFromAny(state["latest_change"])
	if strings.EqualFold(firstStringFromMap(change, "freshness"), "current_snapshot") {
		return change, true
	}
	if confirmed := s.harness.LatestAuthoritativeProjectChange(4); len(confirmed) > 0 {
		return confirmed, true
	}
	return change, false
}

func (s *Server) freeStatePostActionProjectChange(ctx context.Context) (map[string]any, bool) {
	return s.postActionProjectChange(ctx, "free_state_post_action_refresh")
}

const freeStateContinuationBudgetExhaustedReason = "free_state_continuation_budget_exhausted"

// freeStateContinuationBudgetExhausted reports whether the loop's explicit
// cross-slice continuation budget is spent (ADR §10 scheduling semantics;
// waiting_continue is an internal bounded state, not an unlimited resume).
func freeStateContinuationBudgetExhausted(loop freeStateReasoningLoop) bool {
	return loop.ContinuationBudget > 0 && loop.ContinuationUsed >= loop.ContinuationBudget
}

// BOUNDARY-1 terminal-turn trigger reasons (content-free).
const (
	FreeStateTerminalReasonBudgetCritical = "budget_critical"
	FreeStateTerminalReasonLastCheckpoint = "last_checkpoint"
)

// freeStateTerminalTurnGuarded reports whether the terminal-turn reservation
// applies to this loop right now. The machinery targets the pre-proposal
// observation loop whose exits must pass through the model's mouth; a loop
// with a live experiment runtime or a mandatory post-action debt has its own
// protected windows (reserveD1PostApplySlices, the human-judgment boundary)
// whose legal outputs are experiment reports, not the terminal family.
func freeStateTerminalTurnGuarded(loop freeStateReasoningLoop) bool {
	return freeStateLoopActive(loop) && loop.Experiment == nil &&
		!loop.RequiresPostActionObservation && !freeStateJudgmentBoundary(loop)
}

// freeStateTerminalTurnTriggerReason evaluates the two BOUNDARY-1 §1.1
// triggers against the authoritative counters, taking the earlier one:
// budget critical (closure rounds remaining <= 1 or continuations remaining
// <= 1) or the final continuation checkpoint (continuations remaining == 0,
// i.e. the checkpoint being scheduled is the last legal resume). It returns
// the trigger reason, or "" when no trigger holds or the loop is already
// locked / outside the terminal-turn guard.
func freeStateTerminalTurnTriggerReason(loop freeStateReasoningLoop, closure audioclosure.State, closureTracked bool) string {
	if loop.TerminalTurnLocked || !freeStateTerminalTurnGuarded(loop) {
		return ""
	}
	if closureTracked && closure.ContractID != "" && closure.Policy.MaxClosureRounds > 0 &&
		closure.Policy.MaxClosureRounds-closure.RoundsStarted <= 1 {
		return FreeStateTerminalReasonBudgetCritical
	}
	if loop.ContinuationBudget > 0 {
		remaining := loop.ContinuationBudget - loop.ContinuationUsed
		if remaining <= 0 {
			return FreeStateTerminalReasonLastCheckpoint
		}
		if remaining == 1 {
			return FreeStateTerminalReasonBudgetCritical
		}
	}
	return ""
}

// lockFreeStateTerminalTurn atomically sets the terminal-turn reservation
// latch. It is idempotent and never clears: once locked, the loop's remaining
// turns are terminal-only and the system settles become the honest fallback.
func lockFreeStateTerminalTurn(loop *freeStateReasoningLoop, reason string) bool {
	if loop == nil || loop.TerminalTurnLocked || strings.TrimSpace(reason) == "" {
		return false
	}
	loop.TerminalTurnLocked = true
	loop.TerminalTurnReason = strings.TrimSpace(reason)
	loop.UpdatedAt = time.Now().UTC()
	return true
}

// evaluateFreeStateTerminalTurnTrigger loads the authoritative closure state
// for the loop's conversation and locks the terminal turn when a trigger
// holds. It reports whether the latch was set by this call.
func (s *Server) evaluateFreeStateTerminalTurnTrigger(loop *freeStateReasoningLoop) bool {
	if s == nil || loop == nil {
		return false
	}
	closure, tracked := audioclosure.State{}, false
	if s.audioClosures != nil {
		closure, tracked = s.audioClosures.ActiveForConversation(loop.ConversationID)
	}
	reason := freeStateTerminalTurnTriggerReason(*loop, closure, tracked)
	return lockFreeStateTerminalTurn(loop, reason)
}

func resolvedFreeStateDecisionPhase(loop freeStateReasoningLoop) string {
	if loop.RequiresPostActionObservation {
		return freeStatePhasePostActionEvaluation
	}
	if strings.EqualFold(strings.TrimSpace(loop.Status), "awaiting_action") {
		return freeStatePhaseProcessorMaterialization
	}
	return freeStatePhaseProcessorSelection
}

func freeStateCCBObservations(res agentloop.Result) []*agentloop.RecentObservation {
	out := make([]*agentloop.RecentObservation, 0)
	for _, record := range res.Executed {
		name := strings.ToLower(firstNonEmpty(firstStringFromMap(record, "tool"), firstStringFromMap(record, "command_name")))
		if name != "ccb.observation_request" && name != "ccb_observation_request" {
			continue
		}
		result := firstMapFromAny(record["result"])
		bundle := firstMapFromAny(result["bundle"])
		if len(bundle) == 0 && len(firstMapFromAny(result["audit_receipt"])) == 0 {
			continue
		}
		status := firstNonEmpty(firstStringFromMap(record, "status"), firstStringFromMap(result, "status"), firstStringFromMap(bundle, "status"), firstStringFromMap(firstMapFromAny(result["audit_receipt"]), "status"))
		summary := bundle
		if len(summary) == 0 {
			summary = result
		}
		out = append(out, &agentloop.RecentObservation{
			ToolCallID:  firstStringFromMap(record, "tool_call_id"),
			Tool:        firstNonEmpty(firstStringFromMap(record, "tool"), "ccb.observation_request"),
			CommandName: firstStringFromMap(record, "command_name"),
			Status:      status,
			Error:       firstStringFromMap(record, "error"),
			Summary:     cloneContext(summary),
		})
	}
	// A one-turn MessageLoop can finish immediately after a CCB observation.
	// In that path the authoritative observation is retained on Result even
	// when the per-turn Executed projection is empty. Admit that observation
	// into the controller input as well, while avoiding a duplicate when both
	// projections are present.
	if observation := res.RecentObservation; freeStateIsCCBObservation(observation) && !freeStateObservationAlreadyPresent(out, observation) {
		out = append(out, cloneRecentObservationForFreeState(observation))
	}
	return out
}

func freeStateIsCCBObservation(observation *agentloop.RecentObservation) bool {
	if observation == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(observation.Tool, observation.CommandName)))
	if name != "ccb.observation_request" && name != "ccb_observation_request" {
		return false
	}
	return len(firstMapFromAny(observation.Summary["bundle"])) > 0 ||
		len(firstMapFromAny(observation.Summary["audit_receipt"])) > 0 ||
		len(firstStringFromMap(observation.Summary, "observation_id", "receipt_id")) > 0
}

func freeStateObservationAlreadyPresent(observations []*agentloop.RecentObservation, candidate *agentloop.RecentObservation) bool {
	for _, observation := range observations {
		if observation == nil {
			continue
		}
		if candidate.ToolCallID != "" && observation.ToolCallID == candidate.ToolCallID {
			return true
		}
		candidateID := firstStringFromMap(candidate.Summary, "observation_id", "receipt_id")
		if candidateID != "" && candidateID == firstStringFromMap(observation.Summary, "observation_id", "receipt_id") {
			return true
		}
	}
	return false
}

func cloneRecentObservationForFreeState(observation *agentloop.RecentObservation) *agentloop.RecentObservation {
	if observation == nil {
		return nil
	}
	clone := *observation
	clone.Summary = cloneContext(observation.Summary)
	clone.ProducedBindings = append([]agentloop.ExecutionBinding(nil), observation.ProducedBindings...)
	return &clone
}

func freeStateCCBObservation(res agentloop.Result) *agentloop.RecentObservation {
	observations := freeStateCCBObservations(res)
	if len(observations) == 0 {
		return nil
	}
	for index := len(observations) - 1; index >= 0; index-- {
		if freeStateUsableObservation(observations[index]) {
			return observations[index]
		}
	}
	return observations[len(observations)-1]
}

func freeStateObservationReceiptRecorded(receipts []map[string]any, receiptID string) bool {
	for _, receipt := range receipts {
		if firstStringFromMap(receipt, "receipt_id") == receiptID {
			return true
		}
	}
	return false
}

func isFreeStateCancellation(decision string, resp ChatResponse) bool {
	clean := strings.ToLower(strings.TrimSpace(decision))
	return clean == "cancel" || clean == "deny" || clean == "reject" ||
		resp.GoalStatus == string(agentruntime.StatusCancelled)
}

func freeStateAcousticActionOutcome(interaction PendingInteraction, resp ChatResponse) (string, string, map[string]any, bool) {
	workflow := firstNonEmpty(resp.Workflow, interaction.Workflow)
	switch {
	case strings.EqualFold(workflow, semanticCompressorExecutionWorkflow):
		status := firstStringFromMap(resp.WorkflowData, "status")
		if status == "executed" {
			return "compressor", "applied", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
		if status == "failed" || strings.TrimSpace(resp.Error) != "" {
			return "compressor", "failed", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
	case strings.EqualFold(workflow, semanticDynamicWorkflow):
		status := firstStringFromMap(resp.WorkflowData, "status")
		processor := firstStringFromMap(resp.WorkflowData, "processor_type")
		if processor == "" {
			processor = firstStringFromMap(firstMapFromAny(resp.WorkflowData["execution_receipt"]), "processor_type")
		}
		if status == "executed" {
			return firstNonEmpty(processor, "dynamic"), "applied", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
		if status == "failed" || status == "rejected" || strings.TrimSpace(resp.Error) != "" {
			return firstNonEmpty(processor, "dynamic"), "failed", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
	case strings.EqualFold(workflow, "capability_runtime_v1") &&
		firstStringFromMap(resp.WorkflowData, "capability_id") == agentSemanticEQCapabilityID:
		for _, receipt := range freeStateMapRows(resp.WorkflowData["receipts"]) {
			if strings.EqualFold(firstStringFromMap(receipt, "status"), "applied") {
				out := cloneContext(receipt)
				if verification := resp.WorkflowData["verification"]; verification != nil {
					out["verification"] = verification
				}
				return "eq", "applied", out, true
			}
		}
		if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			return "eq", "failed", cloneContext(resp.WorkflowData), true
		}
	case strings.EqualFold(workflow, "mix_tick"):
		// Native track gain/pan proposals execute through the mix_tick
		// confirmation path rather than a semantic processor workflow. Treat
		// the committed tick as a governed action so the free-state loop can
		// enter post_action_evaluation and request fresh CCB evidence. The
		// mix.observe receipt remains execution-layer evidence; it is not
		// substituted for the model-requested CCB observation.
		if strings.EqualFold(resp.StopReason, "mix_tick_applied_reobserved") &&
			!resp.NeedsConfirmation && resp.GoalStatus == string(agentruntime.StatusCompleted) {
			receipt := cloneContext(resp.WorkflowData)
			if receipt == nil {
				receipt = map[string]any{}
			}
			receipt["stop_reason"] = resp.StopReason
			if len(resp.ExecutedKernelReply) > 0 {
				receipt["executed_kernel_reply"] = resp.ExecutedKernelReply
			}
			return "mix_tick", "applied", receipt, true
		}
		if strings.EqualFold(resp.StopReason, "mix_tick_applied_reobserve_failed") ||
			strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			return "mix_tick", "failed", cloneContext(resp.WorkflowData), true
		}
	case strings.EqualFold(workflow, "free_state_d1_s1"):
		status := strings.ToLower(firstStringFromMap(resp.WorkflowData, "status"))
		receipt := cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"]))
		if status == "applied" && boolValue(resp.WorkflowData["parameter_applied"]) && boolValue(resp.WorkflowData["readback_verified"]) {
			return experiment.D1S1ActionDomain, "applied", receipt, true
		}
		if status == "failed" || status == "blocked" || strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			return experiment.D1S1ActionDomain, "failed", receipt, true
		}
	}
	return "", "", nil, false
}

func freeStateLoopFromAny(value any) (freeStateReasoningLoop, bool) {
	if typed, ok := value.(freeStateReasoningLoop); ok {
		return cloneFreeStateLoop(typed), typed.SchemaVersion == freeStateReasoningLoopSchema
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "null" {
		return freeStateReasoningLoop{}, false
	}
	loop := freeStateReasoningLoop{}
	if err := json.Unmarshal(data, &loop); err != nil || loop.SchemaVersion != freeStateReasoningLoopSchema {
		return freeStateReasoningLoop{}, false
	}
	return loop, true
}

func freeStateLoopMap(loop freeStateReasoningLoop) map[string]any {
	data, _ := json.Marshal(loop)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

// freeStateObservationWithAuthoritativeAudit reattaches the authoritative CCB
// audit receipt from the loop's durable receipt list onto the compact
// ledger-rebuilt latest observation, keyed by receipt_id. The rebuilt
// observation is a compact projection whose audit keeps only identity
// fields; boundaries that verify the model-requested view contract
// (semanticProgressiveDisclosureRecordObservation) must see the authoritative
// receipt. Without an id-matched authoritative receipt the observation is
// returned unchanged and those boundaries keep failing closed exactly as
// before — this only restores recorded fact, it never upgrades one.
func freeStateObservationWithAuthoritativeAudit(loop freeStateReasoningLoop, observation *agentloop.RecentObservation) *agentloop.RecentObservation {
	if observation == nil {
		return nil
	}
	audit := firstMapFromAny(observation.Summary["audit_receipt"])
	receiptID := firstStringFromMap(audit, "receipt_id")
	if receiptID == "" || strings.TrimSpace(firstStringFromMap(audit, "requested_by")) != "" {
		return observation
	}
	for _, receipt := range loop.ObservationReceipts {
		if firstStringFromMap(receipt, "receipt_id") != receiptID {
			continue
		}
		if strings.TrimSpace(firstStringFromMap(receipt, "requested_by")) == "" {
			continue
		}
		restored := cloneRecentObservationForFreeState(observation)
		restored.Summary["audit_receipt"] = cloneContext(receipt)
		return restored
	}
	return observation
}

func freeStateProcessorIntentMap(decision *agentloop.FreeStateDecision) map[string]any {
	if decision == nil || decision.SemanticProcessorIntent == nil {
		return nil
	}
	intent := decision.SemanticProcessorIntent
	out := map[string]any{
		"schema_version":    intent.SchemaVersion,
		"status":            intent.Status,
		"family":            intent.Family,
		"intent":            intent.Intent,
		"required_coverage": append([]string(nil), intent.RequiredCoverage...),
		"scope":             intent.Scope,
		"control_mode":      intent.ControlMode,
		"confidence":        intent.Confidence,
		"evidence_refs":     append([]string(nil), intent.EvidenceRefs...),
	}
	if intent.Rejection != nil {
		out["rejection"] = map[string]any{"code": intent.Rejection.Code, "reason": intent.Rejection.Reason, "details": cloneContext(intent.Rejection.Details)}
	}
	return out
}

func freeStateActionMap(action freeStateActionRecord) map[string]any {
	data, _ := json.Marshal(action)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func freeStateMapRows(value any) []map[string]any {
	data, _ := json.Marshal(value)
	rows := []map[string]any{}
	_ = json.Unmarshal(data, &rows)
	return rows
}

func cloneFreeStateLoop(loop freeStateReasoningLoop) freeStateReasoningLoop {
	data, _ := json.Marshal(loop)
	out := freeStateReasoningLoop{}
	_ = json.Unmarshal(data, &out)
	return out
}

// freeStateCapabilityBoundaryDetail records the concrete boundary behind a
// capability_blocked settlement: the durable capacity routing level, the
// governed capability that was (or was not) selected, and the continuation
// budget numbers. It lets the acceptance report distinguish a real capability
// boundary from missing data. Callers must hold s.mu.
func (s *Server) freeStateCapabilityBoundaryDetailLocked(conversationID string, loop freeStateReasoningLoop) []string {
	detail := []string{}
	if s != nil {
		var record CapabilityRouteRecord
		for _, candidate := range s.capabilityRoutes {
			if candidate.ConversationID == conversationID &&
				(candidate.UpdatedAt.After(record.UpdatedAt) || record.SchemaVersion == "") {
				record = candidate
			}
		}
		if record.Assessment != nil {
			if record.Assessment.CapacityLevel != "" {
				detail = append(detail, "capacity_level="+record.Assessment.CapacityLevel)
			}
			if record.Assessment.SelectedCapability != "" {
				detail = append(detail, "selected_capability="+record.Assessment.SelectedCapability)
			} else {
				detail = append(detail, "selected_capability=none")
			}
		}
	}
	if loop.ContinuationBudget > 0 {
		detail = append(detail, fmt.Sprintf("continuation_budget=%d/%d", loop.ContinuationUsed, loop.ContinuationBudget))
	}
	return detail
}

// settleFreeStateLoopFromClosure propagates a terminal minimal-audio-closure
// settlement into the free-state reasoning loop. The scheduler-driven
// settling turn never returns over HTTP, so without this sync the loop stays
// "observing" while the closure has already settled, and no runtime interface
// exposes the terminal stop reason.
func (s *Server) settleFreeStateLoopFromClosure(conversationID string, settlement audioclosure.Settlement, closure ...audioclosure.State) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	if s.freeStateLoops == nil {
		s.mu.Unlock()
		return
	}
	loop, ok := s.freeStateLoops[conversationID]
	if !ok || !freeStateLoopActive(loop) {
		s.mu.Unlock()
		return
	}
	boundaryDetail := s.freeStateCapabilityBoundaryDetailLocked(conversationID, loop)
	decisionStatus, loopStatus := agentloop.FreeStateBlocked, "blocked"
	switch settlement.Reason {
	case audioclosure.StopNoCandidateFound:
		decisionStatus, loopStatus = agentloop.FreeStateNoCandidateFound, "no_candidate_found"
	case audioclosure.StopCapabilityBlocked:
		decisionStatus, loopStatus = agentloop.FreeStateCapabilityBlocked, "capability_blocked"
	case audioclosure.StopDiagnosticComplete:
		decisionStatus, loopStatus = agentloop.FreeStateDiagnosticComplete, "completed"
	case audioclosure.StopSatisfied:
		decisionStatus, loopStatus = agentloop.FreeStateSatisfied, "completed"
	case audioclosure.StopCancelled:
		decisionStatus, loopStatus = agentloop.FreeStateBlocked, "cancelled"
	}
	now := time.Now().UTC()
	summary := firstNonEmpty(settlement.Summary, "the bounded audio closure settled: "+string(settlement.Reason))
	loop.Status = loopStatus
	loop.LastError = firstNonEmpty(settlement.Summary, string(settlement.Reason))
	if settlement.Reason == audioclosure.StopNoCandidateFound && s.harness != nil && loop.GoalID != "" {
		goal := s.harness.RuntimeStatus(loop.GoalID)
		if goal.Task != nil && goal.Task.SemanticState != nil && goal.Task.SemanticState.State == taskstate.StateObservationInProgress {
			evidence := append([]string(nil), loop.ObservationIDs...)
			if len(evidence) == 0 {
				evidence = []string{"free_state:" + loop.LoopID}
			}
			_, _ = s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
				Event: taskstate.EventNoCandidateReported, Reason: "bounded free-state search budget exhausted", Summary: summary,
				EvidenceRefs:    evidence,
				ProjectRevision: goal.Task.SemanticState.ProjectRevision,
			})
		}
	}
	decision := agentloop.FreeStateDecision{
		SchemaVersion:  agentloop.FreeStateDecisionSchema,
		Status:         decisionStatus,
		EvidenceStatus: "insufficient",
		Summary:        summary,
		StopReason:     string(settlement.Reason),
	}
	if decisionStatus == agentloop.FreeStateCapabilityBlocked {
		decision.Limitations = append(decision.Limitations, boundaryDetail...)
	}
	// Preserve the FS6 admission boundary even when the model never emitted a
	// proposal: the closure settlement is still required to explain the
	// selected frontier/evidence and all failed G1-G7 checks. This is audit
	// metadata only and does not create a candidate or execution authority.
	if decisionStatus == agentloop.FreeStateCapabilityBlocked || decisionStatus == agentloop.FreeStateBlocked {
		candidateID, evidenceRef := "", ""
		gateResults := map[string]string{
			"G1_project_binding":           "fail",
			"G2_capacity_assessed":         "fail",
			"G3_project_scan":              "fail",
			"G4_dimension_closed":          "fail",
			"G5_frontier_established":      "fail",
			"G6_target_evidence":           "fail",
			"G7_fresh_revision_bound_refs": "fail",
			// G8 needs a model proposal to cross-check; this reconstruction is
			// exactly the no-proposal terminal boundary, so consistency stays
			// unproven rather than defaulted to pass.
			"G8_target_consistency": "fail",
		}
		if len(closure) > 0 {
			current := closure[0]
			if strings.TrimSpace(current.ProjectUUID) != "" && strings.TrimSpace(current.ProjectRevision) != "" {
				gateResults["G1_project_binding"] = "pass"
			}
			for _, route := range s.capabilityRoutes {
				if route.ConversationID == conversationID && route.Assessment != nil &&
					strings.TrimSpace(route.Assessment.SelectedCapability) != "" &&
					!strings.EqualFold(strings.TrimSpace(route.Assessment.CapacityLevel), "exceeds_free_state") {
					gateResults["G2_capacity_assessed"] = "pass"
					break
				}
			}
			for _, record := range current.Observations {
				for _, viewID := range record.ViewIDs {
					if viewID == "mix.multitrack_relationship" || viewID == "mix.frequency_relationship" || strings.HasPrefix(viewID, "project.") {
						gateResults["G3_project_scan"] = "pass"
					}
				}
			}
			for _, round := range current.DiagnosticRounds {
				if round.Closed() {
					gateResults["G4_dimension_closed"] = "pass"
				}
			}
			if len(current.Frontier.Candidates) > 0 {
				gateResults["G5_frontier_established"] = "pass"
			}
			candidateID = closure[0].Frontier.CandidateID
			for _, candidate := range closure[0].Frontier.Candidates {
				if candidate.ID == candidateID {
					evidenceRef = firstNonEmpty(candidate.SourceObservationID)
					if evidenceRef == "" && len(candidate.EvidenceRefs) > 0 {
						evidenceRef = candidate.EvidenceRefs[0]
					}
					break
				}
			}
			selectedTracks := map[string]bool{}
			for _, candidate := range current.Frontier.Candidates {
				if candidate.ID == candidateID {
					for _, trackID := range candidate.TrackIDs {
						selectedTracks[trackID] = true
					}
				}
			}
			if candidateID != "" {
				for _, record := range current.Observations {
					if selectedTracks[record.TargetRef] {
						for _, viewID := range record.ViewIDs {
							if strings.HasPrefix(viewID, "track.") {
								gateResults["G6_target_evidence"] = "pass"
								// The frontier source proves candidate discovery, but
								// FS6 admission must retain the exact selected-target
								// observation. Prefer that track-level observation ref
								// over the candidate's project/relationship source ref.
								if strings.TrimSpace(record.ObservationID) != "" {
									evidenceRef = record.ObservationID
								}
							}
						}
					}
				}
			}
		}
		// A terminal capability boundary has no model proposal to validate;
		// retain all deterministic gates as failed rather than calling it
		// no_candidate_found.
		failed := []string{}
		for _, id := range []string{"G1_project_binding", "G2_capacity_assessed", "G3_project_scan", "G4_dimension_closed", "G5_frontier_established", "G6_target_evidence", "G7_fresh_revision_bound_refs", "G8_target_consistency"} {
			if gateResults[id] == "fail" {
				failed = append(failed, id)
			}
		}
		loop.AdmissionReceipt = map[string]any{
			"schema_version": "free_state_admission_receipt.v1", "status": string(decisionStatus),
			"boundary": "proposal_missing", "candidate_id": candidateID, "target_evidence_ref": evidenceRef,
			"failed_gate_ids": failed, "proposal_present": false, "proposal_valid": false,
			"gate_results":     gateResults,
			"project_revision": loop.LatestProjectChange["project_revision"], "recorded_at": now.Format(time.RFC3339Nano),
		}
	}
	loop.LatestDecision = &decision
	loop.ActiveIntent = ""
	loop.RequiresPostActionObservation = false
	loop.UpdatedAt = now
	s.freeStateLoops[conversationID] = cloneFreeStateLoop(loop)
	s.mu.Unlock()
	_ = s.persistContinuationState()
}

// syncFreeStateSpine mirrors the closure spine (FS phase + diagnostic round)
// into the free-state loop so scheduler-driven turns are observable at the
// runtime interface and the loop persists across restarts.
func (s *Server) syncFreeStateSpine(state audioclosure.State) {
	if s == nil || strings.TrimSpace(state.ConversationID) == "" {
		return
	}
	s.mu.Lock()
	if s.freeStateLoops == nil {
		s.mu.Unlock()
		return
	}
	loop, ok := s.freeStateLoops[state.ConversationID]
	if !ok {
		s.mu.Unlock()
		return
	}
	// TIMING-1: the gate-open signal arming is retired with the phase
	// admission gate (needs_experiment is admitted in every FS phase, so
	// there is no "gate opens later" transition to signal). The spine mirror
	// keeps only the observation-organization facts.
	if phase, valid := audioclosure.ParsePhase(string(state.Phase)); valid {
		loop.CurrentPhase = string(phase)
	}
	if len(state.DiagnosticRounds) > 0 {
		loop.CurrentRoundID = state.DiagnosticRounds[len(state.DiagnosticRounds)-1].RoundID
	}
	// The priority queue is a projection of the closure's diagnostic round
	// records (audioclosure.QueueFromRounds); mirroring it into the loop keeps
	// the no_candidate_found necessity check (messageLoopFreeStateQueueStillOpen)
	// on production data while the event stream stays the single truth.
	queue := audioclosure.QueueFromRounds(state.DiagnosticRounds)
	loop.PriorityQueue = &queue
	loop.UpdatedAt = time.Now().UTC()
	s.freeStateLoops[state.ConversationID] = cloneFreeStateLoop(loop)
	s.mu.Unlock()
}
