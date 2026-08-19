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
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
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
	SchemaVersion                 string                       `json:"schema_version"`
	LoopID                        string                       `json:"loop_id"`
	ConversationID                string                       `json:"conversation_id"`
	GoalID                        string                       `json:"goal_id,omitempty"`
	RunID                         string                       `json:"run_id,omitempty"`
	Status                        string                       `json:"status"`
	DecisionPhase                 string                       `json:"decision_phase"`
	OriginalIntent                string                       `json:"original_intent"`
	ActiveIntent                  string                       `json:"active_intent"`
	TargetRef                     map[string]any               `json:"target_ref,omitempty"`
	Cycle                         int                          `json:"cycle"`
	MaxCycles                     int                          `json:"max_cycles"`
	ObservationIDs                []string                     `json:"observation_ids,omitempty"`
	ObservationReceipts           []map[string]any             `json:"observation_receipts,omitempty"`
	RejectedObservationRequests   []map[string]any             `json:"rejected_observation_requests,omitempty"`
	ObservationLedger             map[string]any               `json:"observation_ledger,omitempty"`
	LatestProjectChange           map[string]any               `json:"latest_project_change,omitempty"`
	LatestObservation             *agentloop.RecentObservation `json:"latest_observation,omitempty"`
	Actions                       []freeStateActionRecord      `json:"actions,omitempty"`
	LatestDecision                *agentloop.FreeStateDecision `json:"latest_decision,omitempty"`
	RequiresPostActionObservation bool                         `json:"requires_post_action_observation"`
	LastError                     string                       `json:"last_error,omitempty"`
	CreatedAt                     time.Time                    `json:"created_at"`
	UpdatedAt                     time.Time                    `json:"updated_at"`
}

func freeStateLoopActive(loop freeStateReasoningLoop) bool {
	if loop.SchemaVersion != freeStateReasoningLoopSchema || strings.TrimSpace(loop.OriginalIntent) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(loop.Status)) {
	case "completed", "cancelled", "blocked":
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
	goalID, runID := goalIDsFromContext(requestContext)
	if goalID != "" {
		loop.GoalID = goalID
	}
	if runID != "" {
		loop.RunID = runID
	}
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
	if overlay.SchemaVersion != "" {
		out.SchemaVersion = overlay.SchemaVersion
	}
	for _, field := range []struct {
		dst *string
		src string
	}{
		{&out.LoopID, overlay.LoopID}, {&out.ConversationID, overlay.ConversationID},
		{&out.GoalID, overlay.GoalID}, {&out.RunID, overlay.RunID}, {&out.Status, overlay.Status},
		{&out.DecisionPhase, overlay.DecisionPhase}, {&out.OriginalIntent, overlay.OriginalIntent},
		{&out.ActiveIntent, overlay.ActiveIntent}, {&out.LastError, overlay.LastError},
	} {
		if strings.TrimSpace(field.src) != "" {
			*field.dst = field.src
		}
	}
	if overlay.TargetRef != nil {
		out.TargetRef = cloneContext(overlay.TargetRef)
	}
	if len(overlay.LatestProjectChange) > 0 {
		out.LatestProjectChange = cloneContext(overlay.LatestProjectChange)
	}
	if overlay.MaxCycles > 0 {
		out.MaxCycles = overlay.MaxCycles
	}
	if overlay.Cycle > out.Cycle {
		out.Cycle = overlay.Cycle
	}
	if overlay.LatestObservation != nil {
		out.LatestObservation = overlay.LatestObservation
	}
	if overlay.LatestDecision != nil {
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
	loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
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
	if res.FreeStateDecision == nil {
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return loop, true
	}
	decision := *res.FreeStateDecision
	decision.RequestedViewIDs = append([]string(nil), res.FreeStateDecision.RequestedViewIDs...)
	decision.Limitations = append([]string(nil), res.FreeStateDecision.Limitations...)
	loop.LatestDecision = &decision
	if res.GoalID != "" {
		loop.GoalID = res.GoalID
	}
	if res.RunID != "" {
		loop.RunID = res.RunID
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
	case agentloop.FreeStateNeedsExperiment:
		if decision.ImprovementProposal == nil {
			loop.Status = "blocked"
			loop.LastError = "needs_experiment requires improvement_proposal"
			blocked := decision
			blocked.Status = agentloop.FreeStateBlocked
			blocked.EvidenceStatus = "insufficient"
			blocked.StopReason = "free_state_improvement_proposal_missing"
			blocked.Limitations = append(blocked.Limitations, loop.LastError)
			loop.LatestDecision = &blocked
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			return loop, true
		}
		proposal := decision.ImprovementProposal
		loop.TargetRef = cloneContext(proposal.Target)
		loop.Status = "awaiting_experiment"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.ActiveIntent = strings.TrimSpace(proposal.ImprovementIntent)
		loop.RequiresPostActionObservation = false
		s.upsertPendingCandidate(proposal.ToPendingCandidate(conversationID, res.GoalID, res.RunID, time.Now().UTC().Format(time.RFC3339Nano)))
	case agentloop.FreeStateNeedsObservation:
		loop.Status = "observing"
		loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
	case agentloop.FreeStateSatisfied:
		loop.Status = "completed"
		loop.ActiveIntent = ""
		loop.RequiresPostActionObservation = false
	case agentloop.FreeStateBlocked:
		loop.Status = "blocked"
		loop.LastError = firstNonEmpty(decision.StopReason, strings.Join(decision.Limitations, "; "), decision.Summary)
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return loop, true
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
		view := firstMapFromAny(views[viewID])
		viewStatus := firstNonEmpty(firstStringFromMap(view, "status"), firstStringFromMap(observation.Summary, "status"))
		if !strings.EqualFold(viewStatus, "ready") && !strings.EqualFold(viewStatus, "partial") {
			continue
		}
		key := freeStateObservationLedgerViewKey(viewID, observation.Summary)
		available[key] = nonEmptyFreeStateMap(map[string]any{
			"view_id":        viewID,
			"status":         viewStatus,
			"observation_id": firstStringFromMap(observation.Summary, "observation_id"),
			"tool_call_id":   observation.ToolCallID,
			"freshness":      cloneContext(firstMapFromAny(observation.Summary["freshness"])),
			"limitations":    firstNonNil(view["limitations"], observation.Summary["limitations"]),
			"evidence_refs":  observation.Summary["evidence_refs"],
			"audit_ref":      freeStateObservationAuditRef(observation.Summary),
			"target_ref":     freeStateObservationTrackTarget(observation),
			"round":          round,
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

func freeStateObservationCompactReceipt(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	audit := firstMapFromAny(observation.Summary["audit_receipt"])
	return nonEmptyFreeStateMap(map[string]any{
		"receipt_id":      firstStringFromMap(audit, "receipt_id"),
		"receipt_schema":  firstStringFromMap(audit, "schema_version"),
		"tool_call_id":    observation.ToolCallID,
		"observation_id":  firstStringFromMap(observation.Summary, "observation_id"),
		"request_id":      firstStringFromMap(observation.Summary, "request_id"),
		"status":          firstStringFromMap(observation.Summary, "status", "bundle_status"),
		"requested_views": freeStateNormalizedViewIDs(freeStateStringSlice(observation.Summary["requested_views"])),
		"freshness":       cloneContext(firstMapFromAny(observation.Summary["freshness"])),
		"limitations":     observation.Summary["limitations"],
		"evidence_refs":   observation.Summary["evidence_refs"],
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
		"502", "503", "504", "temporary", "stream returned no output", "connection reset",
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
	if isFreeStateCancellation(decision, resp) {
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
	loop.RequiresPostActionObservation = actionStatus == "applied"
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
