package agentloop

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/audioclosure"
)

// Free-state needs_experiment admission gate
// (docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md). The single-usable-bundle weak
// gate is fully replaced by the seven conjunctive machine checks below. The
// only legal exit when the gate fails is needs_observation.

const (
	freeStateGateG1 = "G1_project_binding"
	freeStateGateG2 = "G2_capacity_assessed"
	freeStateGateG3 = "G3_project_scan"
	freeStateGateG4 = "G4_dimension_closed"
	freeStateGateG5 = "G5_frontier_established"
	freeStateGateG6 = "G6_target_evidence"
	freeStateGateG7 = "G7_fresh_revision_bound_refs"
)

var freeStateGateOrder = []string{
	freeStateGateG1, freeStateGateG2, freeStateGateG3, freeStateGateG4,
	freeStateGateG5, freeStateGateG6, freeStateGateG7,
}

// FreeStateGateAudit is a read-only explanation of the G1-G7 admission
// evaluation. It is intentionally separate from the model decision so a
// missing or malformed proposal can still be reported at the capability
// boundary without being turned into a fabricated candidate.
type FreeStateGateAudit struct {
	FailedGateIDs []string `json:"failed_gate_ids,omitempty"`
	Passed        bool     `json:"passed"`
	Proposal      bool     `json:"proposal_present"`
	ProposalValid bool     `json:"proposal_valid"`
	ProposalError string   `json:"proposal_error,omitempty"`
}

// AuditFreeStateNeedsExperimentGate exposes the deterministic G1-G7 check to
// orchestration/reporting code without exposing the internal runState type.
// The supplied context is treated as an immutable snapshot.
func AuditFreeStateNeedsExperimentGate(context map[string]any, decision *FreeStateDecision) FreeStateGateAudit {
	audit := FreeStateGateAudit{Proposal: decision != nil && decision.ImprovementProposal != nil}
	if audit.Proposal {
		if err := decision.ImprovementProposal.Validate(); err != nil {
			audit.ProposalError = err.Error()
		} else {
			audit.ProposalValid = true
		}
	}
	state := &runState{input: Input{Context: cloneMap(context)}}
	audit.FailedGateIDs = evaluateFreeStateNeedsExperimentGate(state, decision)
	audit.Passed = len(audit.FailedGateIDs) == 0
	return audit
}

func freeStateMapRevision(value any) string {
	if m := messageLoopMapValue(value); len(m) > 0 {
		return firstMapText(m, "project_revision", "revision")
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// gateG1 requires taskstate.Contract and the closure state to agree on a
// non-empty project binding.
func gateG1(state *runState) bool {
	if state == nil {
		return false
	}
	contract := messageLoopMapValue(state.input.Context["task_contract"])
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	if len(contract) == 0 || len(closure) == 0 {
		return false
	}
	contractUUID, contractRevision := firstMapText(contract, "project_uuid"), firstMapText(contract, "project_revision")
	closureUUID, closureRevision := firstMapText(closure, "project_uuid"), firstMapText(closure, "project_revision")
	return contractUUID != "" && contractRevision != "" &&
		strings.EqualFold(contractUUID, closureUUID) && strings.EqualFold(contractRevision, closureRevision)
}

// gateG2 requires a completed, non-blocked capability routing assessment with
// at least one executable governed path. The real assessment fields are
// capacity_level (within_free_state / near_free_state_limit /
// fixed_capability_requested / exceeds_free_state) and selected_capability;
// "status" is not an assessment field.
func gateG2(state *runState) bool {
	if state == nil {
		return false
	}
	assessment := messageLoopMapValue(state.input.Context["free_state_capacity_assessment"])
	if len(assessment) == 0 {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(firstMapText(assessment, "capacity_level")), "exceeds_free_state") {
		return false
	}
	return strings.TrimSpace(firstMapText(assessment, "selected_capability")) != ""
}

var freeStateProjectScanViews = []string{"mix.multitrack_relationship", "mix.frequency_relationship"}

func freeStateReceiptUsable(row map[string]any) bool {
	switch strings.ToLower(strings.TrimSpace(firstMapText(row, "status", "bundle_status"))) {
	case "ready", "partial":
		return true
	default:
		return false
	}
}

// gateG3 requires at least one usable project/mix-level scan receipt in the
// observation ledger.
func gateG3(state *runState) bool {
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if !freeStateReceiptUsable(row) {
			continue
		}
		for _, viewID := range messageLoopStringList(row["requested_views"]) {
			for _, scan := range freeStateProjectScanViews {
				if strings.EqualFold(strings.TrimSpace(viewID), scan) {
					return true
				}
			}
		}
	}
	return false
}

// gateG4 requires at least one closed diagnostic dimension: a round record
// with usable evidence and no open unresolved question.
func gateG4(state *runState) bool {
	ctx := messageLoopFreeStateContext(state)
	rows := messageLoopMapRows(ctx["diagnostic_rounds"])
	if len(rows) == 0 {
		rows = messageLoopMapRows(state.input.Context["free_state_diagnostic_rounds"])
	}
	if len(rows) == 0 {
		rows = messageLoopMapRows(messageLoopMapValue(state.input.Context["minimal_audio_closure"])["diagnostic_rounds"])
	}
	for _, row := range rows {
		if !strings.EqualFold(strings.TrimSpace(firstMapText(row, "evidence_status", "status")), "ready") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(firstMapText(row, "evidence_status")), "ready") &&
			len(messageLoopStringList(row["unresolved_questions"])) == 0 {
			return true
		}
	}
	return false
}

// gateG5 requires an established candidate frontier.
func gateG5(state *runState) bool {
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	return len(messageLoopMapRows(frontier["candidates"])) > 0
}

// gateG6 requires target-level usable evidence for the selected candidate.
func gateG6(state *runState) bool {
	if state == nil {
		return false
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	if selected == "" {
		return false
	}
	allowedTracks := map[string]bool{}
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if !strings.EqualFold(messageLoopText(candidate["id"]), selected) {
			continue
		}
		for _, trackID := range messageLoopStringList(candidate["track_ids"]) {
			allowedTracks[trackID] = true
		}
	}
	if len(allowedTracks) == 0 {
		return false
	}
	if messageLoopFreeStateCandidateTargetObserved(state, allowedTracks) {
		return true
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	for _, raw := range messageLoopMapValue(ledger["available_views"]) {
		view := messageLoopMapValue(raw)
		target := messageLoopMapValue(view["target_ref"])
		if !strings.EqualFold(messageLoopText(target["kind"]), "track") || !allowedTracks[messageLoopText(target["id"])] {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(firstMapText(view, "status"))) {
		case "ready", "partial":
			return true
		}
	}
	return false
}

// gateG7 requires every improvement-proposal evidence ref to resolve to a
// fresh, revision-bound observation receipt.
func gateG7(state *runState, evidenceRefs []string) bool {
	if state == nil || len(evidenceRefs) == 0 {
		return false
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	closureRevision := firstMapText(closure, "project_revision")
	if closureRevision == "" {
		return false
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	receipts := messageLoopMapRows(ledger["receipts"])
	available := messageLoopMapValue(ledger["available_views"])
	for _, ref := range evidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return false
		}
		matched := false
		for _, row := range receipts {
			if !refMatchesLedgerRow(ref, row) {
				continue
			}
			matched = true
			if !freeStateReceiptFreshRevisionBound(row, closureRevision) {
				return false
			}
		}
		for _, raw := range available {
			view := messageLoopMapValue(raw)
			if !refMatchesLedgerRow(ref, view) {
				continue
			}
			matched = true
			if !freeStateViewFreshRevisionBound(view, closureRevision) {
				return false
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func refMatchesLedgerRow(ref string, row map[string]any) bool {
	if strings.EqualFold(strings.TrimSpace(firstMapText(row, "observation_id")), ref) ||
		strings.EqualFold(strings.TrimSpace(firstMapText(row, "receipt_id")), ref) {
		return true
	}
	for _, candidate := range messageLoopStringList(row["evidence_refs"]) {
		if strings.EqualFold(strings.TrimSpace(candidate), ref) {
			return true
		}
	}
	return false
}

// freeStateFreshStatus accepts the freshness statuses a real CCB bundle can
// carry: the receipt writer marks fresh observations with status "fresh", the
// production bundle freshness carries the project binding status
// ("current"/"current_snapshot") or class "current_observation". "stale" and
// any unknown value stay non-fresh.
func freeStateFreshStatus(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fresh", "current", "current_snapshot", "current_observation":
		return true
	default:
		return false
	}
}

func freeStateReceiptFreshRevisionBound(row map[string]any, closureRevision string) bool {
	freshness := messageLoopMapValue(row["freshness"])
	if len(freshness) > 0 {
		// CCB bundles use status for observation readiness (for example
		// status=ready) and class for freshness semantics.  A valid class must
		// take precedence; readiness must never be interpreted as freshness.
		freshnessClass := firstMapText(freshness, "class")
		freshnessValue := firstNonEmpty(freshnessClass, firstMapText(freshness, "status"))
		if freshnessValue != "" && !freeStateFreshStatus(freshnessValue) {
			return false
		}
		if revision := firstMapText(freshness, "project_revision"); revision != "" {
			return strings.EqualFold(revision, closureRevision)
		}
	} else if status := strings.ToLower(strings.TrimSpace(firstMapText(row, "freshness"))); status != "" && !freeStateFreshStatus(status) {
		return false
	}
	if revision := firstMapText(row, "project_revision"); revision != "" {
		return strings.EqualFold(revision, closureRevision)
	}
	// No revision information at all cannot prove revision binding.
	return false
}

func freeStateViewFreshRevisionBound(view map[string]any, closureRevision string) bool {
	freshness := messageLoopMapValue(view["freshness"])
	if len(freshness) > 0 {
		freshnessClass := firstMapText(freshness, "class")
		freshnessValue := firstNonEmpty(freshnessClass, firstMapText(freshness, "status"))
		if freshnessValue != "" && !freeStateFreshStatus(freshnessValue) {
			return false
		}
		if revision := firstMapText(freshness, "project_revision"); revision != "" {
			return strings.EqualFold(revision, closureRevision)
		}
	}
	if revision := firstMapText(view, "project_revision"); revision != "" {
		return strings.EqualFold(revision, closureRevision)
	}
	return false
}

// evaluateFreeStateNeedsExperimentGate returns the ids of every failed gate.
// An empty result admits the needs_experiment decision and the experiment
// Admission construction.
func evaluateFreeStateNeedsExperimentGate(state *runState, decision *FreeStateDecision) []string {
	var refs []string
	if decision != nil && decision.ImprovementProposal != nil {
		refs = decision.ImprovementProposal.EvidenceRefs
	}
	checks := map[string]bool{
		freeStateGateG1: gateG1(state),
		freeStateGateG2: gateG2(state),
		freeStateGateG3: gateG3(state),
		freeStateGateG4: gateG4(state),
		freeStateGateG5: gateG5(state),
		freeStateGateG6: gateG6(state),
		freeStateGateG7: gateG7(state, refs),
	}
	var failed []string
	for _, id := range freeStateGateOrder {
		if !checks[id] {
			failed = append(failed, id)
		}
	}
	return failed
}

// freeStateNeedsExperimentGateFailureMessage words the admission-gate refusal.
// When the revision-binding gates fail (G1 project binding, G7 fresh
// revision-bound refs) it appends the fresh observation reference the ledger
// currently holds — mechanical runtime state telling the retry which citation
// is quotable now, never domain guidance (20260829_205921: the model re-quoted
// a stale pointer the catalog no longer served and burnt its turns).
func freeStateNeedsExperimentGateFailureMessage(state *runState, failed []string) string {
	failedList := strings.Join(failed, ", ")
	revisionBoundFailure := false
	for _, id := range failed {
		if id == freeStateGateG1 || id == freeStateGateG7 {
			revisionBoundFailure = true
		}
	}
	if !revisionBoundFailure {
		return fmt.Sprintf("needs_experiment requires the full admission gate; failed: %s; return needs_observation with the next bounded observation instead", failedList)
	}
	if reference := freeStateLedgerFreshReference(state); reference != "" {
		return fmt.Sprintf("needs_experiment requires the full admission gate; failed: %s; the fresh quotable observation reference is %s; return needs_observation with the next bounded observation instead", failedList, reference)
	}
	return fmt.Sprintf("needs_experiment requires the full admission gate; failed: %s; return needs_observation with the next bounded observation instead", failedList)
}

// freeStateLedgerFreshReference returns the freshest quotable observation
// reference "obs_id@revision" the observation ledger holds under the closure's
// current revision: the last available_views row that is fresh and
// revision-bound (append order is recency order), falling back to the last
// such receipt. Empty when the ledger holds nothing quotable at the closure
// revision.
func freeStateLedgerFreshReference(state *runState) string {
	if state == nil {
		return ""
	}
	closureRevision := firstMapText(messageLoopMapValue(state.input.Context["minimal_audio_closure"]), "project_revision")
	if closureRevision == "" {
		return ""
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	reference := ""
	for _, raw := range messageLoopMapValue(ledger["available_views"]) {
		view := messageLoopMapValue(raw)
		if !freeStateViewFreshRevisionBound(view, closureRevision) {
			continue
		}
		if candidate := freeStateRowFreshReference(view); candidate != "" {
			reference = candidate
		}
	}
	if reference == "" {
		for _, row := range messageLoopMapRows(ledger["receipts"]) {
			if !freeStateReceiptFreshRevisionBound(row, closureRevision) {
				continue
			}
			if candidate := freeStateRowFreshReference(row); candidate != "" {
				reference = candidate
			}
		}
	}
	return reference
}

// freeStateRowFreshReference reads one ledger row's identity as the mechanical
// citation "obs_id@revision", preferring the row's explicit revision binding
// over the freshness block's.
func freeStateRowFreshReference(row map[string]any) string {
	observationID := strings.TrimSpace(firstMapText(row, "observation_id"))
	if observationID == "" {
		return ""
	}
	revision := strings.TrimSpace(firstNonEmpty(
		firstMapText(row, "project_revision"),
		firstMapText(messageLoopMapValue(row["freshness"]), "project_revision"),
	))
	if revision == "" {
		return ""
	}
	return observationID + "@" + revision
}

// messageLoopFreeStateClaimsProjectPerfect is the M05 pattern assertion: an
// exhausted diagnostic queue must not be reported as the project being
// flawless. It matches an open set of perfection patterns, not fixed copy.
var freeStatePerfectionPatterns = []string{
	"perfect", "flawless", "no issues", "nothing wrong", "no problems",
	"完美", "无任何问题", "没有任何问题", "无可挑剔", "没有问题",
}

func messageLoopFreeStateClaimsProjectPerfect(decision FreeStateDecision, reply string) bool {
	text := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(reply), decision.Summary, decision.StopReason,
		strings.Join(decision.Limitations, " "),
	}, " "))
	for _, pattern := range freeStatePerfectionPatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

// freeStateReObservationAdmitted applies the three same-target/view-set
// re-request admissions (contract §2) on top of the existing usable/rejected
// dedup: new evidence (contradiction-priority revisit), a new project
// revision, or a recorded contradiction.
func freeStateReObservationAdmitted(state *runState, fingerprint string, requestedViews []string, metadata ...map[string]any) bool {
	if state == nil || fingerprint == "" {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	var prior map[string]any
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if messageLoopFreeStateRequestFingerprint(messageLoopStringList(row["requested_views"]), messageLoopMapValue(row["target_ref"])) == fingerprint {
			prior = row
			break
		}
	}
	if prior == nil && state.recentObservation != nil {
		row := state.recentObservation.Summary
		if messageLoopFreeStateRequestFingerprint(messageLoopStringList(row["requested_views"]), messageLoopMapValue(row["target_ref"])) == fingerprint {
			prior = row
		}
	}
	if prior == nil {
		return false
	}
	// The current turn may carry explicit round metadata, or the observation
	// receipt itself may carry it when a scheduler continuation restored state.
	meta := messageLoopMapValue(state.input.Context["free_state_reobservation"])
	if len(metadata) > 0 && len(metadata[0]) > 0 {
		meta = metadata[0]
	}
	if len(meta) == 0 {
		meta = messageLoopMapValue(ctx["reobservation"])
	}
	currentRevision := firstNonEmpty(
		firstMapText(meta, "project_revision", "new_project_revision"),
		firstMapText(ctx, "project_revision"),
		firstMapText(state.input.Context, "project_revision"),
		firstMapText(messageLoopMapValue(state.input.Context["minimal_audio_closure"]), "project_revision"),
	)
	priorRevision := firstNonEmpty(firstMapText(prior, "project_revision"), firstMapText(messageLoopMapValue(prior["freshness"]), "project_revision"))
	priorStatus := audioclosure.RoundEvidenceStatus(strings.ToLower(firstNonEmpty(firstMapText(prior, "evidence_status", "status", "bundle_status"), "open")))
	priorQuestions := messageLoopStringList(firstNonNilValue(prior["unresolved_questions"], messageLoopMapValue(prior["round"])["unresolved_questions"]))
	newQuestions := messageLoopStringList(firstNonNilValue(meta["unresolved_questions"], meta["new_unresolved_questions"], ctx["unresolved_questions"]))
	priority := audioclosure.PriorityReason(strings.ToLower(firstNonEmpty(firstMapText(meta, "priority_reason", "new_priority_reason"), firstMapText(ctx, "priority_reason"))))
	declaredContradiction := freeStateBool(firstNonNilValue(meta["declared_contradiction"], meta["contradiction"]))
	if !declaredContradiction && state.recentObservation != nil {
		declaredContradiction = freeStateBool(firstNonNilValue(state.recentObservation.Summary["declared_contradiction"], state.recentObservation.Summary["contradiction"]))
	}
	requested := messageLoopStringList(meta["requested_views"])
	if len(requested) == 0 && len(requestedViews) > 0 {
		requested = requestedViews
	}
	admitted, _ := audioclosure.AdmitReObservation(audioclosure.ReObservationRequest{
		RequestedViews: requested, PriorViews: messageLoopStringList(prior["requested_views"]),
		PriorEvidenceStatus: priorStatus, PriorUnresolved: priorQuestions, NewUnresolvedQuestions: newQuestions,
		PriorProjectRevision: priorRevision, NewProjectRevision: currentRevision, NewPriorityReason: priority,
		DeclaredContradiction: declaredContradiction,
	})
	if admitted {
		return true
	}
	return false
}

func freeStateReObservationMetadata(decision *FreeStateDecision) map[string]any {
	if decision == nil {
		return nil
	}
	meta := map[string]any{}
	if strings.TrimSpace(decision.PriorityReason) != "" {
		meta["priority_reason"] = strings.TrimSpace(decision.PriorityReason)
	}
	if len(decision.UnresolvedQuestions) > 0 {
		meta["new_unresolved_questions"] = append([]string(nil), decision.UnresolvedQuestions...)
	}
	if decision.DeclaredContradiction {
		meta["declared_contradiction"] = true
	}
	return meta
}

// messageLoopFreeStatePhaseDecisionIssue is the host-phase query: when an FS
// phase is present in the host context, the decision status must be legal for
// that phase. A model turn is not a diagnostic round; phases may contain many
// tool calls and continuations, so only the decision type is checked here.
func messageLoopFreeStatePhaseDecisionIssue(state *runState, status string) string {
	if state == nil {
		return ""
	}
	raw := strings.TrimSpace(firstMapText(state.input.Context, "free_state_phase"))
	if raw == "" {
		raw = strings.TrimSpace(messageLoopText(messageLoopMapValue(state.input.Context["minimal_audio_closure"])["phase"]))
	}
	if raw == "" {
		return ""
	}
	phase, ok := audioclosure.ParsePhase(raw)
	if !ok || !audioclosure.IsFSPhase(phase) {
		return ""
	}
	if audioclosure.AllowsDecisionStatus(phase, status) {
		return ""
	}
	return fmt.Sprintf("the closure host is in phase %s which does not admit decision status %s; return needs_observation with the next bounded observation (or the phase-legal terminal boundary)", phase, status)
}
