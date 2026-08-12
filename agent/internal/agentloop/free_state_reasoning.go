package agentloop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
)

const FreeStateDecisionSchema = "free_state_decision.v1"

const (
	freeStateObservationLedgerSchema = "free_state_observation_ledger.v1"
	freeStateObservationLedgerLimit  = 24
)

const (
	FreeStateNeedsObservation     = "needs_observation"
	FreeStateNeedsAction          = "needs_action"
	FreeStateSatisfied            = "satisfied"
	FreeStateBlocked              = "blocked"
	freeStateMaxSameFamilyActions = 2
	freeStateMaxActions           = 6
)

// FreeStateDecision is the model-owned control state for an ordinary-Agent
// observation/action loop. It carries no execution authority.
type FreeStateDecision struct {
	SchemaVersion   string `json:"schema_version"`
	Status          string `json:"status"`
	EvidenceStatus  string `json:"evidence_status"`
	Summary         string `json:"summary"`
	RemainingIntent string `json:"remaining_intent,omitempty"`
	ProcessorType   string `json:"processor_type,omitempty"`
	// SemanticProcessorIntent is the model-owned family/coverage handoff. It
	// contains no plugin identity, path, parameter ID, or vendor mapping.
	SemanticProcessorIntent *processorintent.Intent `json:"semantic_processor_intent,omitempty"`
	RequestedViewIDs        []string                `json:"requested_view_ids,omitempty"`
	ObservationID           string                  `json:"observation_id,omitempty"`
	Limitations             []string                `json:"limitations,omitempty"`
	StopReason              string                  `json:"stop_reason,omitempty"`
}

func (d FreeStateDecision) Validate() error {
	if strings.TrimSpace(d.SchemaVersion) != FreeStateDecisionSchema {
		return fmt.Errorf("schema_version must be %s", FreeStateDecisionSchema)
	}
	status := strings.ToLower(strings.TrimSpace(d.Status))
	if d.SemanticProcessorIntent != nil && status != FreeStateNeedsAction {
		if err := d.SemanticProcessorIntent.Validate(); err != nil {
			return fmt.Errorf("semantic_processor_intent: %w", err)
		}
	}
	switch status {
	case FreeStateNeedsObservation:
		if len(nonEmptyFreeStateStrings(d.RequestedViewIDs)) == 0 {
			return fmt.Errorf("needs_observation requires requested_view_ids")
		}
	case FreeStateNeedsAction:
		if strings.TrimSpace(d.RemainingIntent) == "" {
			return fmt.Errorf("needs_action requires remaining_intent")
		}
		switch strings.ToLower(strings.TrimSpace(d.ProcessorType)) {
		case "eq", "compressor", "limiter", "gate_expander", "gate", "expander", "de_esser", "deesser", "de-esser", "transient_shaper", "transient", "multiband_dynamics", "multiband", "reverb", "delay", "distortion", "native":
		default:
			return fmt.Errorf("needs_action requires a governed processor_type or an explicit native handoff")
		}
		if d.SemanticProcessorIntent != nil {
			if err := validateFreeStateProcessorIntent(*d.SemanticProcessorIntent, d.ProcessorType); err != nil {
				return err
			}
		}
	case FreeStateSatisfied:
		if strings.ToLower(strings.TrimSpace(d.EvidenceStatus)) != "sufficient" {
			return fmt.Errorf("satisfied requires evidence_status=sufficient")
		}
	case FreeStateBlocked:
		// Summary is the canonical human-readable boundary. stop_reason and
		// limitations add structure when available but are not redundant gates.
	default:
		return fmt.Errorf("status must be needs_observation, needs_action, satisfied, or blocked")
	}
	if strings.TrimSpace(d.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

func cloneFreeStateDecision(in *FreeStateDecision) *FreeStateDecision {
	if in == nil {
		return nil
	}
	out := *in
	out.RequestedViewIDs = append([]string(nil), in.RequestedViewIDs...)
	out.Limitations = append([]string(nil), in.Limitations...)
	if in.SemanticProcessorIntent != nil {
		intent := *in.SemanticProcessorIntent
		intent.RequiredCoverage = append([]string(nil), in.SemanticProcessorIntent.RequiredCoverage...)
		intent.EvidenceRefs = append([]string(nil), in.SemanticProcessorIntent.EvidenceRefs...)
		if in.SemanticProcessorIntent.Rejection != nil {
			rejection := *in.SemanticProcessorIntent.Rejection
			if in.SemanticProcessorIntent.Rejection.Details != nil {
				rejection.Details = cloneMap(in.SemanticProcessorIntent.Rejection.Details)
			}
			intent.Rejection = &rejection
		}
		out.SemanticProcessorIntent = &intent
	}
	return &out
}

func validateFreeStateProcessorIntent(intent processorintent.Intent, processorType string) error {
	if err := intent.Validate(); err != nil {
		return fmt.Errorf("semantic_processor_intent: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(intent.Status), processorintent.StatusUnresolved) {
		return fmt.Errorf("semantic_processor_intent: unresolved intent cannot request needs_action")
	}
	if strings.ToLower(strings.TrimSpace(intent.ControlMode)) != processorintent.ControlModeSemantic {
		return fmt.Errorf("semantic_processor_intent: needs_action requires control_mode=semantic_loop")
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return fmt.Errorf("semantic_processor_intent: processor registry unavailable: %w", err)
	}
	if definition, ok := registry.Resolve(intent.Family); !ok {
		return fmt.Errorf("semantic_processor_intent: family is not registered")
	} else if err := registry.ValidateCoverage(definition.Family, intent.RequiredCoverage); err != nil {
		return fmt.Errorf("semantic_processor_intent: %w", err)
	} else if definition.InspectOnly {
		return fmt.Errorf("semantic_processor_intent: inspect-only family cannot enter needs_action")
	}
	if expected := processorFamilyForFreeStateType(processorType); expected != "" && expected != intent.Family {
		return fmt.Errorf("semantic_processor_intent family %s does not match processor_type %s", intent.Family, processorType)
	}
	return nil
}

func processorFamilyForFreeStateType(processorType string) string {
	switch strings.ToLower(strings.TrimSpace(processorType)) {
	case "eq", processorintent.FamilyStaticEQ:
		return processorintent.FamilyStaticEQ
	case "compressor", processorintent.FamilyBroadbandCompressor:
		return processorintent.FamilyBroadbandCompressor
	case "limiter":
		return processorintent.FamilyLimiter
	case "gate", "expander", processorintent.FamilyGateExpander:
		return processorintent.FamilyGateExpander
	case "de-esser", "deesser", "de_esser":
		return processorintent.FamilyDeEsser
	case "transient", processorintent.FamilyTransientShaper:
		return processorintent.FamilyTransientShaper
	case "multiband", processorintent.FamilyMultibandDynamics:
		return processorintent.FamilyMultibandDynamics
	default:
		return ""
	}
}

func nonEmptyFreeStateStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func messageLoopFreeStateContext(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	return messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
}

func messageLoopFreeStatePromptContext(state *runState) map[string]any {
	source := messageLoopFreeStateContext(state)
	if len(source) == 0 {
		return nil
	}
	out := compactSelectedKeys(source, []string{
		"schema_version", "loop_id", "status", "decision_phase", "original_intent", "active_intent",
		"cycle", "max_cycles", "observation_ids", "latest_decision",
		"requires_post_action_observation",
	})
	if target := messageLoopMapValue(source["target_ref"]); len(target) > 0 {
		// Family selection has no need for a loaded-instance identity. Keep only
		// the track binding needed to interpret the CCB observation scope.
		out["target_ref"] = compactSelectedKeys(target, []string{"kind", "id", "track_id", "confidence", "source"})
		if !strings.EqualFold(firstMapText(target, "kind"), "track") {
			delete(out["target_ref"].(map[string]any), "id")
		}
	}
	if decision := messageLoopMapValue(source["latest_decision"]); len(decision) > 0 {
		out["latest_decision"] = compactSelectedKeys(decision, []string{
			"schema_version", "status", "evidence_status", "summary", "remaining_intent",
			"processor_type", "semantic_processor_intent", "requested_view_ids", "observation_id", "limitations", "stop_reason",
		})
	}
	actions := messageLoopMapRows(source["actions"])
	if len(actions) > 0 {
		start := len(actions) - 3
		if start < 0 {
			start = 0
		}
		rows := make([]map[string]any, 0, len(actions)-start)
		for _, action := range actions[start:] {
			row := compactSelectedKeys(action, []string{
				"cycle", "processor_type", "workflow", "status", "recorded_at",
			})
			if receipt := messageLoopMapValue(action["receipt"]); len(receipt) > 0 {
				row["receipt_summary"] = messageLoopFreeStateReceiptPromptSummary(receipt)
			}
			rows = append(rows, row)
		}
		out["actions"] = rows
	}
	return out
}

func messageLoopFreeStateReceiptPromptSummary(receipt map[string]any) map[string]any {
	out := compactSelectedKeys(receipt, []string{
		"schema_version", "status", "track_id",
	})
	if controller := messageLoopMapValue(receipt["controller_result"]); len(controller) > 0 {
		out["controller_result"] = compactSelectedKeys(controller, []string{"status", "atomic", "control_count"})
	}
	if audit := messageLoopMapValue(receipt["parameter_audit"]); len(audit) > 0 {
		out["parameter_audit"] = compactSelectedKeys(audit, []string{"status", "target_parameter_count", "non_target_parameters_unchanged"})
	}
	if evaluation := messageLoopMapValue(receipt["com_evaluation"]); len(evaluation) > 0 {
		out["com_evaluation"] = messageLoopFreeStateCOMEvaluationPromptSummary(evaluation)
	}
	if verification := messageLoopMapValue(receipt["verification"]); len(verification) > 0 {
		out["verification"] = compactSelectedKeys(verification, []string{
			"status", "structural", "acoustic", "user_acceptance", "summary",
		})
	}
	return out
}

func messageLoopFreeStateCOMEvaluationPromptSummary(evaluation map[string]any) map[string]any {
	out := compactSelectedKeys(evaluation, []string{"com_version", "status", "mode", "projection_id"})
	change := messageLoopMapValue(evaluation["behavior_change"])
	if len(change) == 0 {
		return out
	}
	compactChange := compactSelectedKeys(change, []string{"status"})
	dimensions := messageLoopMapRows(change["dimensions"])
	if len(dimensions) > 0 {
		rows := make([]map[string]any, 0, len(dimensions))
		for _, dimension := range dimensions {
			row := compactSelectedKeys(dimension, []string{
				"dimension", "status", "classification", "confidence", "basis", "metrics",
			})
			rows = append(rows, row)
		}
		compactChange["dimensions"] = rows
	}
	out["behavior_change"] = compactChange
	return out
}

func messageLoopFreeStateActive(state *runState) bool {
	row := messageLoopFreeStateContext(state)
	if len(row) == 0 {
		return false
	}
	status := strings.ToLower(firstMapText(row, "status"))
	return status != "completed" && status != "cancelled" && status != "blocked"
}

func messageLoopFreeStateOutputIssue(state *runState, out messageLoopOutput) string {
	active := messageLoopFreeStateActive(state)
	if !active {
		if out.FreeStateDecision != nil {
			return "free_state is only valid while a free_state_reasoning_loop context is active"
		}
		return ""
	}
	if out.NeedsClarification || strings.TrimSpace(out.FailureReason) != "" {
		return ""
	}
	if out.FreeStateDecision == nil {
		return "an active free-state reasoning loop requires one free_state_decision.v1 on every reasoning turn; preserve the original intent and state whether observation, action, satisfaction, or a blocker comes next"
	}
	if err := out.FreeStateDecision.Validate(); err != nil {
		return "invalid free_state decision: " + err.Error()
	}
	status := strings.ToLower(strings.TrimSpace(out.FreeStateDecision.Status))
	switch status {
	case FreeStateNeedsObservation:
		if out.Final || len(out.ToolCalls) == 0 {
			return "needs_observation must be non-final and call ccb.observation_catalog or ccb.observation_request"
		}
		requestCalls := 0
		for _, call := range out.ToolCalls {
			if !messageLoopIsCCBObservationTool(call) {
				return "needs_observation may call only CCB observation catalog/request tools in the free-state loop"
			}
			if !messageLoopIsCCBObservationRequestName(normalizedActionName(call, executorpkg.Result{})) {
				continue
			}
			requestCalls++
			if issue := messageLoopFreeStateRequestedViewsIssue(out.FreeStateDecision.RequestedViewIDs, call.Args["view_ids"]); issue != "" {
				return issue
			}
		}
		if requestCalls > 1 {
			return "needs_observation may contain at most one ccb.observation_request so the executed view set remains exactly auditable"
		}
		if issue := messageLoopFreeStateRejectedViewSetIssue(state, out.FreeStateDecision.RequestedViewIDs); issue != "" {
			return issue
		}
	case FreeStateNeedsAction:
		if len(out.ToolCalls) != 0 {
			return "needs_action must contain no direct mutation tool calls; the local governed processor router owns materialization and confirmation"
		}
		if messageLoopFreeStateSemanticIntentRequired(state) && out.FreeStateDecision.SemanticProcessorIntent == nil {
			return "needs_action requires semantic_processor_intent.v1; family and required_coverage must be selected by the model"
		}
		if !messageLoopFreeStateProcessorGoverned(out.FreeStateDecision.ProcessorType) {
			return "the requested processor_type has no governed open-semantic execution path in this release; only eq, compressor, limiter, gate_expander, de_esser, transient_shaper, and multiband_dynamics are executable here, so return blocked for the unsupported family instead of substituting, recommending, or loading another processor"
		}
		ctx := messageLoopFreeStateContext(state)
		if phase := strings.ToLower(firstMapText(ctx, "decision_phase")); phase == "post_action_evaluation" &&
			strings.ToLower(strings.TrimSpace(out.FreeStateDecision.EvidenceStatus)) != "sufficient" {
			return "post-action evidence is inconclusive or insufficient; no further processor write is allowed until a new governed reasoning turn obtains decisive evidence"
		}
		if !messageLoopHasSuccessfulCCBObservationRequest(state) {
			if freeStateBool(ctx["requires_post_action_observation"]) {
				return "cannot select another processor action after an action until a fresh model-requested CCB observation_request has returned a usable read-only evidence bundle in this reasoning cycle"
			}
			return "cannot select the first processor action until at least one model-requested CCB observation_request has returned a usable read-only evidence bundle"
		}
		if count := messageLoopFreeStateActionCount(ctx); count >= freeStateMaxActions {
			return fmt.Sprintf("the free-state loop reached its bounded %d-action limit; return blocked instead of writing another processor action", freeStateMaxActions)
		}
		if count := messageLoopFreeStateProcessorApplyCount(ctx, out.FreeStateDecision.ProcessorType); count >= freeStateMaxSameFamilyActions {
			return fmt.Sprintf("the same processor family reached its bounded %d-action limit; return blocked or address a different unresolved clause", freeStateMaxSameFamilyActions)
		}
	case FreeStateSatisfied:
		if len(out.ToolCalls) != 0 {
			return "satisfied must contain no tool calls"
		}
		ctx := messageLoopFreeStateContext(state)
		if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
			return "cannot mark the original intent satisfied after an action until a fresh CCB observation_request has returned in this reasoning cycle"
		}
	case FreeStateBlocked:
		if len(out.ToolCalls) != 0 {
			return "blocked must contain no tool calls"
		}
		if messageLoopFreeStateClaimsUnavailableObservation(state, *out.FreeStateDecision) {
			return "cannot claim that observation is unavailable: ccb.observation_catalog and ccb.observation_request are available in this free-state turn; use the CCB catalog/request protocol, then decide the treatment family from the returned bounded evidence and limitations"
		}
	}
	return ""
}

func messageLoopFreeStateRejectedViewSetIssue(state *runState, requested []string) string {
	want := messageLoopFreeStateViewFingerprint(requested)
	if want == "" {
		return ""
	}
	if state != nil && state.recentObservation != nil &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(state.recentObservation.Tool, state.recentObservation.CommandName)) &&
		strings.EqualFold(firstMapText(state.recentObservation.Summary, "status", "bundle_status"), "rejected") &&
		messageLoopFreeStateViewFingerprint(messageLoopStringList(state.recentObservation.Summary["requested_views"])) == want {
		return "the requested CCB view set was rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
	}
	ctx := messageLoopFreeStateContext(state)
	for _, row := range messageLoopMapRows(ctx["rejected_observation_requests"]) {
		if messageLoopFreeStateViewFingerprint(messageLoopStringList(row["requested_views"])) == want {
			return "the requested CCB view set was already rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
		}
	}
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["rejected_view_sets"]) {
		if messageLoopFreeStateViewFingerprint(messageLoopStringList(row["requested_views"])) == want {
			return "the requested CCB view set was already rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
		}
	}
	return ""
}

func messageLoopFreeStateViewFingerprint(values []string) string {
	values = messageLoopNormalizedViewIDs(values)
	if len(values) == 0 {
		return ""
	}
	digest := sha256.Sum256([]byte(strings.Join(values, "\x1f")))
	return "ccb_views:" + hex.EncodeToString(digest[:8])
}

func messageLoopNormalizedViewIDs(values []string) []string {
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

// recordFreeStateCCBObservation updates the compact ledger immediately after
// each CCB execution. This makes a rejection visible to the next model turn in
// the same Runner invocation instead of waiting for the HTTP result boundary.
func recordFreeStateCCBObservation(state *runState, observation *RecentObservation) {
	if state == nil || observation == nil || !messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) {
		return
	}
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	if len(loop) == 0 {
		return
	}
	ledger := messageLoopMapValue(loop["observation_ledger"])
	if len(ledger) == 0 {
		ledger = map[string]any{"schema_version": freeStateObservationLedgerSchema}
	}
	mergeFreeStateObservationLedger(ledger, observation)
	loop["observation_ledger"] = ledger
	if rejected := freeStateRejectedLedgerEntry(observation); len(rejected) > 0 {
		rows := messageLoopMapRows(loop["rejected_observation_requests"])
		if !freeStateLedgerRowRecorded(rows, firstMapText(rejected, "fingerprint"), "fingerprint") {
			rows = append(rows, rejected)
		}
		loop["rejected_observation_requests"] = rows
	}
	state.input.Context["free_state_reasoning_loop"] = loop
}

func mergeFreeStateObservationLedger(ledger map[string]any, observation *RecentObservation) {
	if len(ledger) == 0 || observation == nil {
		return
	}
	ledger["schema_version"] = freeStateObservationLedgerSchema
	if rejected := freeStateRejectedLedgerEntry(observation); len(rejected) > 0 {
		rows := messageLoopMapRows(ledger["rejected_view_sets"])
		if !freeStateLedgerRowRecorded(rows, firstMapText(rejected, "fingerprint"), "fingerprint") {
			ledger["rejected_view_sets"] = appendBoundedFreeStateRows(rows, rejected)
			ledger["rejected_view_set_count"] = freeStateLedgerCount(ledger["rejected_view_set_count"], len(rows)) + 1
		}
	}
	if receipt := freeStateObservationLedgerReceipt(observation); len(receipt) > 0 {
		rows := messageLoopMapRows(ledger["receipts"])
		key := firstNonEmpty(firstMapText(receipt, "receipt_id"), firstMapText(receipt, "tool_call_id"))
		if !freeStateLedgerRowRecorded(rows, key, firstNonEmpty(mapKeyForReceipt(receipt), "tool_call_id")) {
			ledger["receipts"] = appendBoundedFreeStateRows(rows, receipt)
			ledger["receipt_count"] = freeStateLedgerCount(ledger["receipt_count"], len(rows)) + 1
		}
	}
	status := strings.ToLower(firstMapText(observation.Summary, "status", "bundle_status"))
	if status != "ready" && status != "partial" {
		return
	}
	available := messageLoopMapValue(ledger["available_views"])
	if available == nil {
		available = map[string]any{}
	}
	for _, viewID := range messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"])) {
		view := messageLoopMapValue(messageLoopMapValue(observation.Summary["views"])[viewID])
		viewStatus := firstNonEmpty(firstMapText(view, "status"), status)
		if !strings.EqualFold(viewStatus, "ready") && !strings.EqualFold(viewStatus, "partial") {
			continue
		}
		available[viewID] = compactSelectedKeys(map[string]any{
			"view_id":        viewID,
			"status":         viewStatus,
			"observation_id": firstMapText(observation.Summary, "observation_id"),
			"tool_call_id":   observation.ToolCallID,
			"freshness":      observation.Summary["freshness"],
			"limitations":    firstNonNilValue(view["limitations"], observation.Summary["limitations"]),
			"evidence_refs":  observation.Summary["evidence_refs"],
			"audit_ref":      freeStateObservationAuditRef(observation.Summary),
		}, []string{"view_id", "status", "observation_id", "tool_call_id", "freshness", "limitations", "evidence_refs", "audit_ref"})
	}
	if len(available) > 0 {
		ledger["available_views"] = available
	}
}

func freeStateRejectedLedgerEntry(observation *RecentObservation) map[string]any {
	if observation == nil || !strings.EqualFold(firstMapText(observation.Summary, "status", "bundle_status"), "rejected") {
		return nil
	}
	requested := messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"]))
	fingerprint := messageLoopFreeStateViewFingerprint(requested)
	if fingerprint == "" {
		return nil
	}
	audit := messageLoopMapValue(observation.Summary["audit_receipt"])
	return compactSelectedKeys(map[string]any{
		"fingerprint":     fingerprint,
		"requested_views": requested,
		"status":          "rejected",
		"reasons":         firstNonNilValue(observation.Summary["omission_reasons"], audit["rejection_reasons"]),
		"receipt_id":      firstMapText(audit, "receipt_id"),
		"tool_call_id":    observation.ToolCallID,
		"request_id":      firstMapText(observation.Summary, "request_id"),
		"observation_id":  firstMapText(observation.Summary, "observation_id"),
		"retry_policy":    "do_not_retry",
		"rejection_scope": firstNonEmpty(firstMapText(observation.Summary, "rejection_scope"), firstMapText(audit, "rejection_scope")),
		"blocking_view_ids": firstNonNilValue(observation.Summary["blocking_view_ids"], audit["blocking_view_ids"]),
		"non_blocking_view_ids": firstNonNilValue(observation.Summary["non_blocking_view_ids"], audit["non_blocking_view_ids"]),
	}, []string{"fingerprint", "requested_views", "status", "reasons", "receipt_id", "tool_call_id", "request_id", "observation_id", "retry_policy", "rejection_scope", "blocking_view_ids", "non_blocking_view_ids"})
}

func freeStateObservationLedgerReceipt(observation *RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	audit := messageLoopMapValue(observation.Summary["audit_receipt"])
	return compactSelectedKeys(map[string]any{
		"receipt_id":      firstMapText(audit, "receipt_id"),
		"receipt_schema":  firstMapText(audit, "schema_version"),
		"tool_call_id":    observation.ToolCallID,
		"observation_id":  firstMapText(observation.Summary, "observation_id"),
		"request_id":      firstMapText(observation.Summary, "request_id"),
		"status":          firstMapText(observation.Summary, "status", "bundle_status"),
		"requested_views": messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"])),
		"freshness":       observation.Summary["freshness"],
		"limitations":     observation.Summary["limitations"],
		"evidence_refs":   observation.Summary["evidence_refs"],
	}, []string{"receipt_id", "receipt_schema", "tool_call_id", "observation_id", "request_id", "status", "requested_views", "freshness", "limitations", "evidence_refs"})
}

func freeStateObservationAuditRef(summary map[string]any) map[string]any {
	audit := messageLoopMapValue(summary["audit_receipt"])
	return compactSelectedKeys(map[string]any{
		"receipt_id":     firstMapText(audit, "receipt_id"),
		"receipt_schema": firstMapText(audit, "schema_version"),
		"bundle_id":      firstMapText(summary, "bundle_id"),
	}, []string{"receipt_id", "receipt_schema", "bundle_id"})
}

func appendBoundedFreeStateRows(rows []map[string]any, row map[string]any) []map[string]any {
	rows = append(rows, row)
	if len(rows) > freeStateObservationLedgerLimit {
		rows = rows[len(rows)-freeStateObservationLedgerLimit:]
	}
	return rows
}

func freeStateLedgerRowRecorded(rows []map[string]any, want, key string) bool {
	if strings.TrimSpace(want) == "" {
		return false
	}
	for _, row := range rows {
		if firstMapText(row, key) == want {
			return true
		}
	}
	return false
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

func mapKeyForReceipt(receipt map[string]any) string {
	if firstMapText(receipt, "receipt_id") != "" {
		return "receipt_id"
	}
	return "tool_call_id"
}

func firstNonNilValue(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func messageLoopFreeStateSemanticIntentRequired(state *runState) bool {
	route, verified := messageLoopSemanticEntryRoute(state)
	return verified && route == "open_semantic"
}

func messageLoopFreeStateProcessorGoverned(processorType string) bool {
	switch strings.ToLower(strings.TrimSpace(processorType)) {
	case "eq", "compressor", "limiter", "gate", "expander", "gate_expander", "de-esser", "deesser", "de_esser", "transient", "transient_shaper", "multiband", "multiband_dynamics":
		return true
	default:
		return false
	}
}

func canonicalFreeStateProcessorType(processorType string) string {
	switch strings.ToLower(strings.TrimSpace(processorType)) {
	case "eq":
		return "eq"
	case "compressor":
		return "compressor"
	case "limiter":
		return "limiter"
	case "gate", "expander", "gate_expander":
		return "gate_expander"
	case "de-esser", "deesser", "de_esser":
		return "de_esser"
	case "transient", "transient_shaper":
		return "transient_shaper"
	case "multiband", "multiband_dynamics":
		return "multiband_dynamics"
	default:
		return ""
	}
}

func messageLoopFreeStateProcessorApplyCount(ctx map[string]any, processorType string) int {
	processorType = canonicalFreeStateProcessorType(processorType)
	if processorType == "" {
		return 0
	}
	count := 0
	for _, action := range messageLoopMapRows(ctx["actions"]) {
		if strings.EqualFold(firstMapText(action, "status"), "applied") &&
			canonicalFreeStateProcessorType(firstMapText(action, "processor_type")) == processorType {
			count++
		}
	}
	return count
}

func messageLoopFreeStateActionCount(ctx map[string]any) int {
	count := 0
	for _, action := range messageLoopMapRows(ctx["actions"]) {
		if strings.EqualFold(firstMapText(action, "status"), "applied") {
			count++
		}
	}
	return count
}

func messageLoopFreeStateClaimsUnavailableObservation(state *runState, decision FreeStateDecision) bool {
	if state == nil || !allowedTool("ccb.observation_request", state.input.AllowedTools) || messageLoopHasSuccessfulCCBObservationRequest(state) {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	if phase := strings.ToLower(firstMapText(ctx, "decision_phase")); phase != "" && phase != "processor_selection" {
		return false
	}
	text := strings.ToLower(strings.Join([]string{
		decision.Summary,
		decision.StopReason,
		strings.Join(decision.Limitations, " "),
	}, " "))
	mentionsObservation := strings.Contains(text, "observation") || strings.Contains(text, "observe") ||
		strings.Contains(text, "mix.observe") || strings.Contains(text, "\u89c2\u5bdf")
	mentionsInterface := strings.Contains(text, "interface") || strings.Contains(text, "tool") ||
		strings.Contains(text, "\u63a5\u53e3") || strings.Contains(text, "\u5de5\u5177")
	claimsUnavailable := strings.Contains(text, "unavailable") || strings.Contains(text, "not available") ||
		strings.Contains(text, "cannot access") || strings.Contains(text, "\u4e0d\u53ef\u7528") ||
		strings.Contains(text, "\u65e0\u6cd5\u4f7f\u7528")
	return mentionsObservation && mentionsInterface && claimsUnavailable
}

func messageLoopFreeStateTerminalReply(out messageLoopOutput) (string, string, bool) {
	if out.FreeStateDecision == nil || len(out.ToolCalls) != 0 {
		return "", "", false
	}
	status := strings.ToLower(strings.TrimSpace(out.FreeStateDecision.Status))
	if status != FreeStateSatisfied && status != FreeStateBlocked {
		return "", "", false
	}
	reply := strings.TrimSpace(out.Reply)
	if reply == "" {
		reply = strings.TrimSpace(out.FreeStateDecision.Summary)
	}
	if reply == "" {
		reply = "The free-state reasoning loop reached a terminal decision."
	}
	return reply, status, true
}

func coerceMessageLoopFreeStateObservationOutput(state *runState, out messageLoopOutput) messageLoopOutput {
	if !messageLoopFreeStateActive(state) || out.FreeStateDecision == nil ||
		!strings.EqualFold(strings.TrimSpace(out.FreeStateDecision.Status), FreeStateNeedsObservation) ||
		len(nonEmptyFreeStateStrings(out.FreeStateDecision.RequestedViewIDs)) == 0 {
		return out
	}
	if len(out.ToolCalls) > 0 {
		for _, call := range out.ToolCalls {
			name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
			if messageLoopIsCCBObservationName(name) {
				return out
			}
			if name != "mix.observe" && name != "mix_observe" {
				return out
			}
		}
	}
	call := planner.ToolCall{
		Tool:   "ccb.observation_request",
		Args:   map[string]any{"view_ids": nonEmptyFreeStateStrings(out.FreeStateDecision.RequestedViewIDs)},
		Reason: firstNonEmpty(out.FreeStateDecision.Summary, "fulfill the model's structured CCB observation request"),
	}
	if len(out.ToolCalls) > 0 {
		call.ID = out.ToolCalls[0].ID
		call.PlanItemID = out.ToolCalls[0].PlanItemID
		call.Reason = firstNonEmpty(out.ToolCalls[0].Reason, call.Reason)
	}
	out.ToolCalls = []planner.ToolCall{call}
	out.Final = false
	return out
}

func messageLoopFreeStateRequestedViewsIssue(decisionViews []string, callValue any) string {
	decision, decisionIssue := messageLoopFreeStateNormalizedViewSet(decisionViews)
	if decisionIssue != "" {
		return "free_state requested_view_ids " + decisionIssue
	}
	callViews, callIssue := messageLoopFreeStateNormalizedCallViewSet(callValue)
	if callIssue != "" {
		return "ccb.observation_request view_ids " + callIssue
	}
	if len(decision) != len(callViews) {
		return "ccb.observation_request view_ids must exactly match the model's free_state requested_view_ids; the server may not add, remove, replace, or infer views"
	}
	for viewID := range decision {
		if !callViews[viewID] {
			return "ccb.observation_request view_ids must exactly match the model's free_state requested_view_ids; the server may not add, remove, replace, or infer views"
		}
	}
	return ""
}

func messageLoopFreeStateNormalizedViewSet(values []string) (map[string]bool, string) {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		viewID := strings.TrimSpace(value)
		if viewID == "" {
			return nil, "must contain only non-empty identifiers"
		}
		if out[viewID] {
			return nil, "must not contain duplicate identifiers"
		}
		out[viewID] = true
	}
	if len(out) == 0 {
		return nil, "must contain at least one identifier"
	}
	return out, ""
}

func messageLoopFreeStateNormalizedCallViewSet(value any) (map[string]bool, string) {
	var values []string
	switch typed := value.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		values = make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, "must be an array of string identifiers"
			}
			values = append(values, text)
		}
	default:
		return nil, "must be an array of string identifiers"
	}
	return messageLoopFreeStateNormalizedViewSet(values)
}

func freeStateBool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func messageLoopIsCCBObservationTool(call planner.ToolCall) bool {
	return messageLoopIsCCBObservationName(normalizedActionName(call, executorpkg.Result{}))
}

func messageLoopIsCCBObservationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ccb.observation_catalog", "ccb_observation_catalog", "ccb.observation_request", "ccb_observation_request":
		return true
	default:
		return false
	}
}

func messageLoopIsCCBObservationRequestName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ccb.observation_request", "ccb_observation_request":
		return true
	default:
		return false
	}
}

func messageLoopHasSuccessfulCCBObservationRequest(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if messageLoopIsCCBObservationRequestName(name) && messageLoopExecutionSucceeded(record) && messageLoopCCBObservationResultUsable(messageLoopMapValue(record["result"])) {
			return true
		}
	}
	// Once an action has completed, an observation from a prior HTTP turn is
	// stale for the post-action gate. The fresh CCB request must execute in
	// this reasoning turn and therefore appear in state.executed.
	ctx := messageLoopFreeStateContext(state)
	if freeStateBool(ctx["requires_post_action_observation"]) {
		// A transient LLM failure can resume with the successful observation in
		// Continuation.RecentObservation rather than state.executed. Accept it
		// only when the same continuation trace proves that CCB request ran.
		return messageLoopTraceHasSuccessfulCCBObservation(state)
	}
	observation := state.recentObservation
	if observation != nil && observation.Error == "" && !toolStatusFailed(observation.Status) &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) &&
		messageLoopCCBObservationBundleUsable(observation.Summary) {
		return true
	}
	return false
}

func messageLoopTraceHasSuccessfulCCBObservation(state *runState) bool {
	if state == nil || state.recentObservation == nil ||
		!messageLoopIsCCBObservationRequestName(firstNonEmpty(state.recentObservation.Tool, state.recentObservation.CommandName)) ||
		!messageLoopCCBObservationBundleUsable(state.recentObservation.Summary) {
		return false
	}
	for _, event := range state.trace {
		if event.ToolResult == nil || event.ToolResult.ToolCallID != state.recentObservation.ToolCallID ||
			!messageLoopIsCCBObservationRequestName(event.ToolResult.Tool) || toolStatusFailed(event.ToolResult.Status) ||
			!messageLoopCCBObservationResultUsable(event.ToolResult.Result) {
			continue
		}
		return true
	}
	return false
}

func messageLoopCCBObservationResultUsable(result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	return messageLoopCCBObservationBundleUsable(messageLoopMapValue(result["bundle"]))
}

func messageLoopCCBObservationBundleUsable(bundle map[string]any) bool {
	if len(bundle) == 0 || bundle["read_only"] != true || bundle["mutation_authority"] != false {
		return false
	}
	status := strings.ToLower(firstMapText(bundle, "status"))
	if status != "ready" && status != "partial" {
		return false
	}
	return len(messageLoopMapValue(bundle["views"])) > 0
}
