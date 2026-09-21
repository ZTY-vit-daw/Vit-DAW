package agentloop

import (
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
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
	freeStateGateG8 = "G8_target_consistency"
)

var freeStateGateOrder = []string{
	freeStateGateG1, freeStateGateG2, freeStateGateG3, freeStateGateG4,
	freeStateGateG5, freeStateGateG6, freeStateGateG7, freeStateGateG8,
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

// freeStateDisclosureBudgetMarker is the verbatim suffix the disclosure budget
// appends when it drops a view to fit max_disclosure_bytes
// (capabilitycontext/free_state_observation.go: viewID+": omitted by disclosure
// budget"). The same strings reach this package as the bundle summary's
// omission_reasons and as a receipt's rejection_reasons.
const freeStateDisclosureBudgetMarker = "omitted by disclosure budget"

// freeStateScanViewOmittedByDisclosureBudget reports whether a receipt marks
// exactly this view id as trimmed by the disclosure budget. The comparison is
// mechanical and content-blind: it reads the structural "<view_id>: <marker>"
// reason shape only, so no view content, track identity, processor or dose is
// inspected. Reasons carrying any other marker (stale/missing/deferred source
// evidence, non-budget rejections) never disqualify a delivery.
func freeStateScanViewOmittedByDisclosureBudget(row map[string]any, viewID string) bool {
	viewID = strings.TrimSpace(viewID)
	if viewID == "" {
		return false
	}
	for _, reason := range messageLoopStringList(row["rejection_reasons"]) {
		if !strings.Contains(reason, freeStateDisclosureBudgetMarker) {
			continue
		}
		owner := reason
		if idx := strings.Index(reason, ":"); idx >= 0 {
			owner = reason[:idx]
		}
		if strings.EqualFold(strings.TrimSpace(owner), viewID) {
			return true
		}
	}
	return false
}

// freeStateLedgerHasUsableScan reports whether the fresh observation ledger
// holds at least one usable qualified project/mix scan receipt. Extracted from
// gateG3 so gateG5's same-slice fallback (FIX-GATE-FRESHNESS-1) shares the
// exact semantics.
func freeStateLedgerHasUsableScan(state *runState) bool {
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if !freeStateReceiptUsable(row) {
			continue
		}
		for _, viewID := range messageLoopStringList(row["requested_views"]) {
			for _, scan := range freeStateProjectScanViews {
				if !strings.EqualFold(strings.TrimSpace(viewID), scan) {
					continue
				}
				if freeStateScanViewOmittedByDisclosureBudget(row, scan) {
					continue
				}
				return true
			}
		}
	}
	return false
}

// gateG3 requires at least one usable project/mix-level scan receipt whose
// qualified scan view was actually delivered. B13-A (2026-09-12): a receipt
// whose only qualified scan view was trimmed by the CCB disclosure budget used
// to pass G3 on the nominal requested_views hit while the frontier it feeds
// (G5) could never pass — the same receipt yielded opposite facts. The gate now
// requires delivery, so a status=partial receipt with every qualified scan view
// trimmed fails both gates consistently.
func gateG3(state *runState) bool {
	return freeStateLedgerHasUsableScan(state)
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
	if len(messageLoopMapRows(frontier["candidates"])) > 0 {
		return true
	}
	// FIX-GATE-FRESHNESS-1 (2026-09-21): same-slice lag completion — when the
	// qualifying scan itself was delivered inside the current slice, its
	// candidate rows have not folded into the frontier projection yet (the
	// fold runs at the slice boundary). A usable qualified scan receipt in the
	// fresh ledger means the fold will derive candidates from that same
	// receipt (audioClosureCandidates), so the frontier is established for
	// admission purposes. Over-admission stays bounded: G6/G8 still require
	// the target-level evidence regardless of this fallback.
	return freeStateLedgerHasUsableScan(state)
}

// messageLoopFreshTrackObservationTarget returns the track id of the freshest
// usable track-targeted observation, or "" — the track whose candidate the
// slice-boundary fold is about to select (FIX-GATE-FRESHNESS-1's lag
// completion view over state.recentObservation, mirroring
// messageLoopFreeStateCandidateTargetObserved's usability semantics).
func messageLoopFreshTrackObservationTarget(state *runState) string {
	if state == nil || state.recentObservation == nil {
		return ""
	}
	observation := state.recentObservation
	if !messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) {
		return ""
	}
	status := strings.ToLower(strings.TrimSpace(messageLoopText(observation.Summary["status"])))
	if status != "ready" && status != "partial" {
		return ""
	}
	target := messageLoopMapValue(observation.Summary["target_ref"])
	if !strings.EqualFold(messageLoopText(target["kind"]), "track") {
		return ""
	}
	return strings.TrimSpace(messageLoopText(target["id"]))
}

// gateG6 requires target-level usable evidence for the selected candidate.
func gateG6(state *runState) bool {
	if state == nil {
		return false
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	allowedTracks := map[string]bool{}
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if selected != "" && !strings.EqualFold(messageLoopText(candidate["id"]), selected) {
			continue
		}
		for _, trackID := range messageLoopStringList(candidate["track_ids"]) {
			allowedTracks[trackID] = true
		}
	}
	if len(allowedTracks) == 0 {
		// FIX-GATE-FRESHNESS-1 (2026-09-21): an empty selection (or an empty
		// pre-fold frontier) is the same-slice lag window — the server-side
		// fold writes the candidate_id selection only at the slice boundary,
		// so a proposal following its target-level observation inside one
		// slice reads a pre-fold snapshot here (PORT-PS1-N5-1 conv
		// mix_single_tick_e2e_20260921_102317: chain-complete proposal bounced
		// with no_selected_candidate one slice before the fold landed). The
		// fresh track-targeted observation the fold will consume establishes
		// the candidate-to-be track set instead — the same rescue the
		// progression check applies via targetObservedNow
		// (messageLoopFreeStateCandidateProgressionIssue). With a live
		// selection this branch never weakens the gate: the selected
		// candidate's tracks were collected by the loop above.
		if target := messageLoopFreshTrackObservationTarget(state); target != "" {
			allowedTracks[target] = true
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

// gateG8 (AGENT-2 A3 target-consistency assertions, 2026-09-05) requires the
// improvement proposal's target to agree with the candidate frontier, the
// target-level observation ledger, and its own evidence citations:
//
//  1. the proposal target must be a track covered by the frontier-selected
//     candidate (proposal target comes from the candidate frontier);
//  2. the ledger must hold a usable target-level observation whose target_ref
//     is that same track (target-level observation consistency);
//  3. every proposal evidence ref must resolve to ledger rows that are
//     project-level or bound to that same track — never to another track's
//     observation — and at least one ref must cite the target's own
//     observation (evidence refs trace back to the target).
//
// Ground-truth target correctness stays evaluator-side only (sealed truth
// never enters agent context); this gate enforces the internal consistency
// the agent-side surfaces can prove. The M5 wrong-target class (proposal
// narrating one track while the evidence observes another) fails here
// fail-closed and routes back to needs_observation.
func gateG8(state *runState, proposal *agentprotocol.ImprovementProposal) bool {
	if state == nil || proposal == nil {
		return false
	}
	target := messageLoopMapValue(proposal.Target)
	if !strings.EqualFold(strings.TrimSpace(messageLoopText(target["kind"])), "track") {
		return false
	}
	trackID := strings.TrimSpace(firstMapText(target, "id", "track_id"))
	if trackID == "" {
		return false
	}
	// Assertion 1: the proposal target is one of the selected candidate's tracks.
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	fromFrontier := false
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if selected != "" && !strings.EqualFold(messageLoopText(candidate["id"]), selected) {
			continue
		}
		for _, id := range messageLoopStringList(candidate["track_ids"]) {
			if strings.TrimSpace(id) == trackID {
				fromFrontier = true
			}
		}
	}
	if !fromFrontier && selected == "" {
		// FIX-GATE-FRESHNESS-1 (2026-09-21): empty pre-fold frontier — the
		// fresh track-targeted observation is the candidate-to-be (the fold
		// derives its candidate from the same observation rows), so a
		// proposal on that exact track is frontier-consistent. A proposal on
		// any other track stays rejected here, and cross-track citation
		// contamination stays rejected by assertion 3 below.
		if trackID == messageLoopFreshTrackObservationTarget(state) {
			fromFrontier = true
		}
	}
	if !fromFrontier {
		return false
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	rows := append([]map[string]any(nil), messageLoopMapRows(ledger["receipts"])...)
	for _, raw := range messageLoopMapValue(ledger["available_views"]) {
		rows = append(rows, messageLoopMapValue(raw))
	}
	// Assertion 2: a usable target-level observation is bound to the same track.
	targetLevelBound := false
	for _, row := range rows {
		if freeStateReceiptUsable(row) && ledgerRowTrackTarget(row) == trackID {
			targetLevelBound = true
			break
		}
	}
	if !targetLevelBound {
		return false
	}
	// Assertion 3: evidence refs resolve without cross-track contamination and
	// at least one cites the target's own observation.
	citedTarget := false
	for _, ref := range proposal.EvidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return false
		}
		matched := false
		for _, row := range rows {
			if !refMatchesLedgerRow(ref, row) {
				continue
			}
			matched = true
			owner := ledgerRowTrackTarget(row)
			if owner != "" && owner != trackID {
				return false
			}
			if owner == trackID {
				citedTarget = true
			}
		}
		if !matched {
			return false
		}
	}
	return citedTarget
}

// ledgerRowTrackTarget returns the track id a ledger row is bound to, or ""
// for project-level rows (no track target_ref).
func ledgerRowTrackTarget(row map[string]any) string {
	targetRef := messageLoopMapValue(row["target_ref"])
	if !strings.EqualFold(strings.TrimSpace(messageLoopText(targetRef["kind"])), "track") {
		return ""
	}
	return strings.TrimSpace(firstMapText(targetRef, "id", "track_id"))
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
	var proposal *agentprotocol.ImprovementProposal
	if decision != nil && decision.ImprovementProposal != nil {
		refs = decision.ImprovementProposal.EvidenceRefs
		proposal = decision.ImprovementProposal
	}
	checks := map[string]bool{
		freeStateGateG1: gateG1(state),
		freeStateGateG2: gateG2(state),
		freeStateGateG3: gateG3(state),
		freeStateGateG4: gateG4(state),
		freeStateGateG5: gateG5(state),
		freeStateGateG6: gateG6(state),
		freeStateGateG7: gateG7(state, refs),
		freeStateGateG8: gateG8(state, proposal),
	}
	var failed []string
	for _, id := range freeStateGateOrder {
		if !checks[id] {
			failed = append(failed, id)
		}
	}
	return failed
}

// FreeStateAdmissionGapSchema is the TIMING-1 machine-readable G-gate
// rejection gap schema. The gap carries only failure categories, the missing
// evidence/freshness/binding condition fields, and evidence references the
// model itself submitted or the ledger currently serves — never a domain,
// track, plug-in, or dosage hint (content-blind red line, advisory ruling #5).
const FreeStateAdmissionGapSchema = "free_state_admission_gap.v1"

// FreeStateAdmissionMissingCondition is one gate's machine-readable gap: why
// the gate failed (closed condition vocabulary) plus the evidence/freshness/
// binding slots relevant to that condition.
type FreeStateAdmissionMissingCondition struct {
	GateID       string   `json:"gate_id"`
	Condition    string   `json:"condition"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Freshness    string   `json:"freshness,omitempty"`
	Binding      string   `json:"binding,omitempty"`
}

// FreeStateAdmissionGap is the structured refusal a bounced evidence proposal
// receives: the failed gate ids plus the per-gate missing conditions. It is
// read-only feedback derived from the same context the gate itself checked;
// it introduces no new verdict.
type FreeStateAdmissionGap struct {
	SchemaVersion string                            `json:"schema_version"`
	FailedGateIDs []string                          `json:"failed_gate_ids"`
	Missing       []FreeStateAdmissionMissingCondition `json:"missing,omitempty"`
	// QuotableFreshReference is the ledger's current fresh observation
	// reference ("obs_id@revision") when a revision-binding gate failed: which
	// citation is quotable right now, as mechanical runtime state.
	QuotableFreshReference string `json:"quotable_fresh_reference,omitempty"`
}

// freeStateAdmissionGap derives the structured gap for the failed gates. The
// condition derivations re-read the same context fields the gate checks and
// never change any verdict; gates without a dedicated derivation still report
// their category through Missing with the generic condition.
func freeStateAdmissionGap(state *runState, decision *FreeStateDecision, failed []string) FreeStateAdmissionGap {
	gap := FreeStateAdmissionGap{SchemaVersion: FreeStateAdmissionGapSchema, FailedGateIDs: append([]string(nil), failed...)}
	var proposal *agentprotocol.ImprovementProposal
	if decision != nil {
		proposal = decision.ImprovementProposal
	}
	for _, id := range failed {
		switch id {
		case freeStateGateG1:
			condition, binding := gateG1MissingCondition(state)
			if condition != "" {
				gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: condition, Binding: binding})
			}
		case freeStateGateG2:
			if condition := gateG2MissingCondition(state); condition != "" {
				gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: condition, Binding: "free_state_capacity_assessment.capacity_level!=exceeds_free_state and selected_capability non-empty"})
			}
		case freeStateGateG3:
			// TIMING-2 ③: the binding names the two qualified mix scan views and
			// states that a project-level structure view does not satisfy the
			// gate — view ids and gate conditions are CCB catalog structure
			// vocabulary, not domain content (advisory #6 question 2; the 7/7
			// project.structure semantic mismatch BEHAVIOR-1 measured).
			// B13-A: the binding also states the delivery requirement, so the
			// bounce names the same fact the gate now judges (a disclosure-budget
			// omission is a structural receipt reason string, not content).
			gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: "no_delivered_project_scan_receipt", Binding: "observation_ledger receipt status ready|partial with a qualified mix scan view in requested_views: mix.multitrack_relationship or mix.frequency_relationship; a project-level structure view such as project.structure does not satisfy this gate, and the view must have been delivered — a receipt rejection reason marking that view as omitted by disclosure budget disqualifies it"})
		case freeStateGateG4:
			gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: "no_closed_diagnostic_dimension", Binding: "a diagnostic round with evidence_status=ready and no open unresolved_questions"})
		case freeStateGateG5:
			gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: "no_frontier_candidates", Binding: "minimal_audio_closure.hypothesis_frontier.candidates non-empty"})
		case freeStateGateG6:
			gap.Missing = append(gap.Missing, FreeStateAdmissionMissingCondition{GateID: id, Condition: gateG6MissingCondition(state), Binding: "a usable target-level observation for the frontier-selected candidate"})
		case freeStateGateG7:
			for _, missing := range gateG7MissingConditions(state, decision, failed) {
				gap.Missing = append(gap.Missing, missing)
			}
		case freeStateGateG8:
			for _, missing := range gateG8MissingConditions(state, proposal) {
				gap.Missing = append(gap.Missing, missing)
			}
		}
	}
	for _, id := range failed {
		if id == freeStateGateG1 || id == freeStateGateG7 {
			if reference := freeStateLedgerFreshReference(state); reference != "" {
				gap.QuotableFreshReference = reference
				break
			}
		}
	}
	return gap
}

func gateG1MissingCondition(state *runState) (string, string) {
	binding := "task_contract and minimal_audio_closure agree on project_uuid+project_revision"
	if state == nil {
		return "task_contract_missing", binding
	}
	contract := messageLoopMapValue(state.input.Context["task_contract"])
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	if len(contract) == 0 || len(closure) == 0 {
		return "binding_context_missing", binding
	}
	contractUUID, contractRevision := firstMapText(contract, "project_uuid"), firstMapText(contract, "project_revision")
	closureUUID, closureRevision := firstMapText(closure, "project_uuid"), firstMapText(closure, "project_revision")
	if contractUUID == "" || contractRevision == "" || closureUUID == "" || closureRevision == "" {
		return "binding_fields_empty", binding
	}
	if !strings.EqualFold(contractUUID, closureUUID) {
		return "project_uuid_mismatch", binding
	}
	if !strings.EqualFold(contractRevision, closureRevision) {
		return "project_revision_mismatch", binding
	}
	return "binding_failed", binding
}

func gateG2MissingCondition(state *runState) string {
	if state == nil {
		return "assessment_missing"
	}
	assessment := messageLoopMapValue(state.input.Context["free_state_capacity_assessment"])
	if len(assessment) == 0 {
		return "assessment_missing"
	}
	if strings.EqualFold(strings.TrimSpace(firstMapText(assessment, "capacity_level")), "exceeds_free_state") {
		return "capacity_exceeds_free_state"
	}
	if strings.TrimSpace(firstMapText(assessment, "selected_capability")) == "" {
		return "selected_capability_missing"
	}
	return "assessment_failed"
}

func gateG6MissingCondition(state *runState) string {
	if state == nil {
		return "no_target_level_observation_for_selected_candidate"
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	if selected == "" {
		return "no_selected_candidate"
	}
	allowed := 0
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if strings.EqualFold(messageLoopText(candidate["id"]), selected) {
			allowed += len(messageLoopStringList(candidate["track_ids"]))
		}
	}
	if allowed == 0 {
		return "no_target_tracks_for_selected_candidate"
	}
	return "no_target_level_observation_for_selected_candidate"
}

// gateG7MissingConditions mirrors gateG7's per-ref walk: every submitted ref
// must resolve to ledger rows and every matched row must be fresh and bound to
// the closure revision. The gap names the offending refs (the model's own
// citations) and the revision the gate requires.
func gateG7MissingConditions(state *runState, decision *FreeStateDecision, failed []string) []FreeStateAdmissionMissingCondition {
	binding := "every improvement_proposal.evidence_refs entry resolves to a fresh, revision-bound observation receipt"
	if state == nil {
		return []FreeStateAdmissionMissingCondition{{GateID: freeStateGateG7, Condition: "gate_state_unavailable", Binding: binding}}
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	closureRevision := firstMapText(closure, "project_revision")
	if closureRevision == "" {
		return []FreeStateAdmissionMissingCondition{{GateID: freeStateGateG7, Condition: "closure_revision_missing", Freshness: "minimal_audio_closure.project_revision is empty", Binding: binding}}
	}
	var refs []string
	if decision != nil && decision.ImprovementProposal != nil {
		refs = decision.ImprovementProposal.EvidenceRefs
	}
	if len(refs) == 0 {
		// The gap derivation runs against the same bounced decision the gate
		// evaluated; with no refs on it the gate fails on emptiness.
		return []FreeStateAdmissionMissingCondition{{GateID: freeStateGateG7, Condition: "evidence_refs_missing", Binding: binding}}
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	receipts := messageLoopMapRows(ledger["receipts"])
	available := messageLoopMapValue(ledger["available_views"])
	var missing []FreeStateAdmissionMissingCondition
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG7, Condition: "evidence_ref_empty", EvidenceRefs: []string{ref}, Binding: binding})
			continue
		}
		matched, fresh := false, true
		for _, row := range receipts {
			if !refMatchesLedgerRow(ref, row) {
				continue
			}
			matched = true
			if !freeStateReceiptFreshRevisionBound(row, closureRevision) {
				fresh = false
			}
		}
		for _, raw := range available {
			view := messageLoopMapValue(raw)
			if !refMatchesLedgerRow(ref, view) {
				continue
			}
			matched = true
			if !freeStateViewFreshRevisionBound(view, closureRevision) {
				fresh = false
			}
		}
		switch {
		case !matched:
			missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG7, Condition: "unresolved_evidence_ref", EvidenceRefs: []string{ref}, Freshness: "requires project_revision=" + closureRevision, Binding: binding})
		case !fresh:
			missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG7, Condition: "stale_or_unbound_evidence_ref", EvidenceRefs: []string{ref}, Freshness: "requires project_revision=" + closureRevision, Binding: binding})
		}
	}
	if len(missing) == 0 {
		missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG7, Condition: "revision_binding_failed", Freshness: "requires project_revision=" + closureRevision, Binding: binding})
	}
	return missing
}

// gateG8MissingConditions reports which of G8's target-consistency conditions
// failed, as condition vocabulary only. Track identities are deliberately not
// disclosed: the gap stays content-blind (advisory ruling #5 anti-abuse rule 1).
func gateG8MissingConditions(state *runState, proposal *agentprotocol.ImprovementProposal) []FreeStateAdmissionMissingCondition {
	binding := "proposal target and evidence refs are consistent with the frontier-selected candidate and its target-level observations"
	if state == nil {
		return []FreeStateAdmissionMissingCondition{{GateID: freeStateGateG8, Condition: "gate_state_unavailable", Binding: binding}}
	}
	var conditions []string
	var refs []string
	if proposal == nil {
		return []FreeStateAdmissionMissingCondition{{GateID: freeStateGateG8, Condition: "proposal_missing", Binding: binding}}
	}
	target := messageLoopMapValue(proposal.Target)
	if !strings.EqualFold(strings.TrimSpace(messageLoopText(target["kind"])), "track") {
		conditions = append(conditions, "target_not_track_kind")
	}
	trackID := strings.TrimSpace(firstMapText(target, "id", "track_id"))
	if trackID == "" {
		conditions = append(conditions, "target_id_missing")
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	fromFrontier := false
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if !strings.EqualFold(messageLoopText(candidate["id"]), selected) {
			continue
		}
		for _, id := range messageLoopStringList(candidate["track_ids"]) {
			if strings.TrimSpace(id) == trackID {
				fromFrontier = true
			}
		}
	}
	if !fromFrontier {
		conditions = append(conditions, "target_not_from_frontier_candidate")
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	rows := append([]map[string]any(nil), messageLoopMapRows(ledger["receipts"])...)
	for _, raw := range messageLoopMapValue(ledger["available_views"]) {
		rows = append(rows, messageLoopMapValue(raw))
	}
	targetLevelBound := false
	for _, row := range rows {
		if freeStateReceiptUsable(row) && ledgerRowTrackTarget(row) == trackID {
			targetLevelBound = true
			break
		}
	}
	if !targetLevelBound {
		conditions = append(conditions, "no_target_level_observation_bound_to_target")
	}
	citedTarget := false
	for _, ref := range proposal.EvidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			refs = append(refs, ref)
			conditions = append(conditions, "evidence_ref_empty")
			continue
		}
		matched := false
		for _, row := range rows {
			if !refMatchesLedgerRow(ref, row) {
				continue
			}
			matched = true
			owner := ledgerRowTrackTarget(row)
			if owner != "" && owner != trackID {
				refs = append(refs, ref)
				conditions = append(conditions, "cross_track_evidence_contamination")
			}
			if owner == trackID {
				citedTarget = true
			}
		}
		if !matched {
			refs = append(refs, ref)
			conditions = append(conditions, "unresolved_evidence_ref")
		}
	}
	if !citedTarget {
		conditions = append(conditions, "no_target_own_observation_cited")
	}
	var missing []FreeStateAdmissionMissingCondition
	seen := map[string]bool{}
	for _, condition := range conditions {
		if seen[condition] {
			continue
		}
		seen[condition] = true
		missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG8, Condition: condition, EvidenceRefs: refs, Binding: binding})
	}
	if len(missing) == 0 {
		missing = append(missing, FreeStateAdmissionMissingCondition{GateID: freeStateGateG8, Condition: "target_consistency_failed", Binding: binding})
	}
	return missing
}

// freeStateNeedsExperimentGateFailureMessage words the admission-gate refusal
// as a closed template over the structured gap (TIMING-1 change 2): the
// machine-readable gap JSON rides the message so both the model retry and the
// artifact diagnostics can parse it. When a revision-binding gate failed
// (G1/G7) the message keeps the fresh quotable observation reference and the
// literal improvement_proposal.evidence_refs citation shape — mechanical
// runtime state telling the retry which citation is quotable now and how to
// write it, never domain guidance (20260829_205921: the model re-quoted a
// stale pointer the catalog no longer served and burnt its turns;
// MILESTONE-E2E 20260906: two admitted-path proposals were rejected on ref
// format alone). G8 discloses its failed consistency conditions only; track
// identities are no longer named (content-blind red line).
func freeStateNeedsExperimentGateFailureMessage(state *runState, decision *FreeStateDecision, failed []string) string {
	gap := freeStateAdmissionGap(state, decision, failed)
	data, err := json.Marshal(gap)
	if err != nil {
		data = []byte(fmt.Sprintf("%+v", gap.FailedGateIDs))
	}
	guidance := ""
	if gap.QuotableFreshReference != "" {
		// DIAG3-2: the retry needs the literal field shape the gate matches,
		// not just the ref value. Mechanical runtime state, never domain
		// guidance.
		guidance = fmt.Sprintf("; the fresh quotable observation reference is %s — cite it verbatim as an improvement_proposal.evidence_refs entry, e.g. [\"%s\"]", gap.QuotableFreshReference, gap.QuotableFreshReference)
	}
	return fmt.Sprintf("needs_experiment requires the full admission gate; structured gap=%s%s; return needs_observation with the next bounded observation instead", data, guidance)
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
	// TIMING-1: needs_experiment/improvement_proposal are admitted in every FS
	// phase (phaseDecisionPolicies), so this bounce can only fire for the
	// remaining status families (needs_observation/needs_action outside their
	// phases, unknown statuses). The refusal states the procedural fact only —
	// which phase is active and which status it does not admit — with no
	// proposal-timing recovery semantics (the gate-open clause is retired).
	// Closed template; pinned by test.
	return fmt.Sprintf("the closure host is in phase %s which does not admit decision status %s; return needs_observation with the next bounded observation (or the phase-legal terminal boundary)", phase, status)
}
