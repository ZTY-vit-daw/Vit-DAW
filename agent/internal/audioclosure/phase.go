package audioclosure

import (
	"fmt"
	"strings"
	"time"
)

// Free-state phase machine (docs/FREE_STATE_PHASE_TRANSITION_TABLE_V1.md).
// audioclosure is the phase host: it owns the phase truth, the transition
// guards, and the persisted phase_transition events. The model remains the
// owner of observation choice and hypotheses; this machine only constrains
// which decision types are legal in the current phase.

// FS phases (FS0–FS9). Legacy five-value phases (declared in types.go) remain
// valid; ParsePhase
// migrates them onto the FS scale for host-side queries.
const (
	PhaseFS0SemanticEntry          Phase = "fs0_semantic_entry"
	PhaseFS1ProjectBound           Phase = "fs1_project_bound"
	PhaseFS2CapacityAssessed       Phase = "fs2_capacity_assessed"
	PhaseFS3ProjectScan            Phase = "fs3_project_scan"
	PhaseFS4DiagnosticRound        Phase = "fs4_diagnostic_round"
	PhaseFS5CandidateFrontier      Phase = "fs5_candidate_frontier"
	PhaseFS6TargetConfirmed        Phase = "fs6_target_confirmed"
	PhaseFS7ImprovementProposal    Phase = "fs7_improvement_proposal"
	PhaseFS8ExperimentVerification Phase = "fs8_experiment_verification"
	PhaseFS9Terminal               Phase = "fs9_terminal"
)

var FSPhaseOrder = []Phase{
	PhaseFS0SemanticEntry, PhaseFS1ProjectBound, PhaseFS2CapacityAssessed, PhaseFS3ProjectScan,
	PhaseFS4DiagnosticRound, PhaseFS5CandidateFrontier, PhaseFS6TargetConfirmed,
	PhaseFS7ImprovementProposal, PhaseFS8ExperimentVerification, PhaseFS9Terminal,
}

func IsFSPhase(phase Phase) bool {
	for _, candidate := range FSPhaseOrder {
		if phase == candidate {
			return true
		}
	}
	return false
}

// ParsePhase accepts both FS phases and legacy five-value phases, migrating
// legacy values per the four-layer mapping in the transition-table contract §4:
// observing→FS1, reasoning→FS4, awaiting_capability→FS6, verifying→FS8,
// settled→FS9.
func ParsePhase(value string) (Phase, bool) {
	phase := Phase(strings.ToLower(strings.TrimSpace(value)))
	for _, candidate := range FSPhaseOrder {
		if phase == candidate {
			return phase, true
		}
	}
	switch phase {
	case PhaseObserving:
		return PhaseFS1ProjectBound, true
	case PhaseReasoning:
		return PhaseFS4DiagnosticRound, true
	case PhaseAwaitingCapability:
		return PhaseFS6TargetConfirmed, true
	case PhaseVerifying:
		return PhaseFS8ExperimentVerification, true
	case PhaseSettled:
		return PhaseFS9Terminal, true
	default:
		// Legacy values also accept their bare aliases ("fs0".."fs9").
		for _, candidate := range FSPhaseOrder {
			if strings.HasPrefix(string(candidate), string(phase)+"_") {
				return candidate, true
			}
		}
		return "", false
	}
}

// legalPhaseTransitions is the contract §2 adjacency: legal forward, loop, and
// bounded-backward transitions only.
var legalPhaseTransitions = map[Phase]map[Phase]bool{
	PhaseFS0SemanticEntry:    {PhaseFS1ProjectBound: true, PhaseFS9Terminal: true},
	PhaseFS1ProjectBound:     {PhaseFS2CapacityAssessed: true, PhaseFS9Terminal: true},
	PhaseFS2CapacityAssessed: {PhaseFS3ProjectScan: true, PhaseFS9Terminal: true},
	PhaseFS3ProjectScan:      {PhaseFS4DiagnosticRound: true, PhaseFS5CandidateFrontier: true, PhaseFS9Terminal: true},
	PhaseFS4DiagnosticRound:  {PhaseFS4DiagnosticRound: true, PhaseFS5CandidateFrontier: true, PhaseFS9Terminal: true},
	PhaseFS5CandidateFrontier: map[Phase]bool{
		PhaseFS6TargetConfirmed: true, PhaseFS4DiagnosticRound: true, PhaseFS9Terminal: true,
	},
	PhaseFS6TargetConfirmed: map[Phase]bool{
		PhaseFS7ImprovementProposal: true, PhaseFS5CandidateFrontier: true, PhaseFS4DiagnosticRound: true, PhaseFS9Terminal: true,
	},
	PhaseFS7ImprovementProposal: {PhaseFS8ExperimentVerification: true, PhaseFS4DiagnosticRound: true},
	PhaseFS8ExperimentVerification: map[Phase]bool{
		PhaseFS9Terminal: true, PhaseFS8ExperimentVerification: true,
	},
	PhaseFS9Terminal: {},
}

func LegalPhaseTransition(from, to Phase) bool {
	if !IsFSPhase(from) || !IsFSPhase(to) {
		return false
	}
	return legalPhaseTransitions[from][to]
}

// PhaseGuardInput is the deterministic guard input snapshot embedded in every
// phase_transition event so that Fold can re-verify the transition on replay.
type PhaseGuardInput struct {
	// ProjectBound: taskstate.Contract and closure State agree on a non-empty
	// project_uuid/project_revision (G1).
	ProjectBound bool `json:"project_bound"`
	// CapacityAssessed: capability routing completed with an executable path,
	// not capability_blocked (G2).
	CapacityAssessed bool `json:"capacity_assessed"`
	// ScanUsable: at least one usable project/mix-level scan view (G3).
	ScanUsable bool `json:"scan_usable"`
	// QueueNonEmpty: the diagnostic priority queue still has an open dimension.
	QueueNonEmpty bool `json:"queue_non_empty"`
	// DimensionClosed: at least one diagnostic dimension closed with usable
	// evidence and no open unresolved question (G4).
	DimensionClosed bool `json:"dimension_closed"`
	// FrontierEstablished: the hypothesis frontier holds at least one candidate
	// (G5).
	FrontierEstablished bool `json:"frontier_established"`
	// TargetEvidence: the selected candidate has target-level usable evidence
	// (G6).
	TargetEvidence bool `json:"target_evidence"`
	// GatePassed: the needs_experiment admission gate G1–G7 passed (FS7 entry).
	GatePassed bool `json:"gate_passed"`
	// AdmissionValid: experiment.Admission constructed and validated (FS8 entry).
	AdmissionValid bool `json:"admission_valid"`
	// ContinueOnceRemaining: the one global continue_once allowance is still
	// unused (FS8 self-loop).
	ContinueOnceRemaining bool `json:"continue_once_remaining"`
	// TerminalStopReason is required for any transition into FS9.
	TerminalStopReason string `json:"terminal_stop_reason,omitempty"`
}

// EvaluatePhaseGuard re-derives whether a transition satisfies its contract
// guard. It is pure: the same input and phases always produce the same result,
// on the driver and on event replay.
func EvaluatePhaseGuard(from, to Phase, in PhaseGuardInput) error {
	if !IsFSPhase(from) || !IsFSPhase(to) {
		return fmt.Errorf("phase guard requires FS phases, got %s -> %s", from, to)
	}
	if !LegalPhaseTransition(from, to) {
		return fmt.Errorf("illegal phase transition %s -> %s", from, to)
	}
	switch to {
	case PhaseFS1ProjectBound:
		if !in.ProjectBound {
			return fmt.Errorf("guard failed: transition to %s requires a complete project binding", to)
		}
	case PhaseFS2CapacityAssessed:
		if !in.CapacityAssessed {
			return fmt.Errorf("guard failed: transition to %s requires a non-blocked capacity assessment", to)
		}
	case PhaseFS3ProjectScan:
		if !in.ScanUsable {
			return fmt.Errorf("guard failed: transition to %s requires a usable project/mix-level scan view", to)
		}
	case PhaseFS4DiagnosticRound:
		if !in.QueueNonEmpty {
			return fmt.Errorf("guard failed: transition to %s requires a non-empty diagnostic priority queue", to)
		}
	case PhaseFS5CandidateFrontier:
		if !in.FrontierEstablished || !(in.DimensionClosed || (from == PhaseFS3ProjectScan && in.ScanUsable)) {
			return fmt.Errorf("guard failed: transition to %s requires an established frontier and a closed dimension (or a unique scan-level candidate)", to)
		}
	case PhaseFS6TargetConfirmed:
		if !in.TargetEvidence {
			return fmt.Errorf("guard failed: transition to %s requires target-level evidence for the selected candidate", to)
		}
	case PhaseFS7ImprovementProposal:
		if !in.GatePassed {
			return fmt.Errorf("guard failed: transition to %s requires the needs_experiment gate G1-G7", to)
		}
	case PhaseFS8ExperimentVerification:
		if from == PhaseFS7ImprovementProposal && !in.AdmissionValid {
			return fmt.Errorf("guard failed: transition to %s from %s requires a validated experiment admission", to, from)
		}
		if from == PhaseFS8ExperimentVerification && !in.ContinueOnceRemaining {
			return fmt.Errorf("guard failed: the single continue_once allowance is already spent")
		}
	case PhaseFS9Terminal:
		reason := StopReason(strings.TrimSpace(in.TerminalStopReason))
		if reason == "" {
			return fmt.Errorf("guard failed: transition to %s requires a terminal stop reason", to)
		}
		if !validStopReason(reason) {
			return fmt.Errorf("guard failed: transition to %s requires a stop reason from the audioclosure enumeration, got %q", to, reason)
		}
	}
	return nil
}

// PhaseDecisionPolicy returns which free-state decision statuses are legal in
// a phase. The model turn is not the diagnostic round: a phase may contain
// many tool calls and continuations. A missing/empty answer means the status
// is not admitted by the phase machine.
type phaseDecisionPolicy struct {
	NeedsObservation  bool
	NeedsAction       bool
	NeedsExperiment   bool
	TerminalDecisions bool
}

// TIMING-1 (2026-09-09, advisory ruling #5, direction 4b): the phase machine no
// longer holds a second semantic admission standard for proposals. A
// needs_experiment / improvement_proposal decision is admissible in every FS
// phase; whether a concrete proposal is accepted is decided only by the G1-G8
// evidence gate plus the existing content-blind, evidence-binding, authority,
// and safety checks. The phase ladder keeps its observation-organization,
// progress-disclosure, and budget-scheduling roles but has no proposal veto.
// needs_observation / needs_action / terminal-family policies are unchanged.
var phaseDecisionPolicies = map[Phase]phaseDecisionPolicy{
	PhaseFS0SemanticEntry:       {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS1ProjectBound:        {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS2CapacityAssessed:    {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS3ProjectScan:         {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS4DiagnosticRound:     {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS5CandidateFrontier:   {NeedsObservation: true, NeedsExperiment: true},
	PhaseFS6TargetConfirmed:     {NeedsObservation: true, NeedsAction: true, NeedsExperiment: true},
	PhaseFS7ImprovementProposal: {NeedsAction: true, NeedsExperiment: true},
	// FS8 is the verification phase of a governed experiment.  The model may
	// still request read-only evidence here, but it must not request another
	// action. The experiment evaluation report itself (experiment_materiality,
	// experiment_target_response, experiment_round_decision incl.
	// user_judgment_pending) travels on a needs_experiment decision; without
	// admitting it here the verification phase could observe but never
	// report, burning the continuation budget on bounced evaluations
	// (2026-08-25 D1 smoke: "fs8_experiment_verification does not admit
	// decision status needs_experiment").
	PhaseFS8ExperimentVerification: {NeedsObservation: true, NeedsExperiment: true, TerminalDecisions: true},
	PhaseFS9Terminal:               {TerminalDecisions: true},
}

// AllowsDecisionStatus reports whether a free-state decision status is legal
// in the given phase. Unknown statuses are terminal-family by default.
func AllowsDecisionStatus(phase Phase, status string) bool {
	policy, ok := phaseDecisionPolicies[phase]
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "needs_observation", "observation_in_progress":
		return policy.NeedsObservation
	case "needs_action":
		return policy.NeedsAction
	case "needs_experiment", "improvement_proposal":
		return policy.NeedsExperiment
	case "satisfied", "diagnostic_complete", "no_candidate_found", "capability_blocked", "blocked":
		return policy.TerminalDecisions || policy.NeedsObservation || policy.NeedsAction || policy.NeedsExperiment
	default:
		return false
	}
}

// PhaseGuardEvidence carries the guard inputs that have no representation in
// the closure State: capability routing, the needs_experiment gate verdict,
// and admission validation. Everything else is derived from State so a caller
// cannot self-assert project binding, scan evidence, queue state, frontier,
// or target evidence (transition-table contract §2).
type PhaseGuardEvidence struct {
	CapacityAssessed bool
	GatePassed       bool
	AdmissionValid   bool
}

var projectScanViewIDs = map[string]bool{
	"mix.multitrack_relationship": true,
	"mix.frequency_relationship":  true,
}

// DerivePhaseGuardInput builds the guard snapshot from the closure State
// itself. Recorded observations are admitted facts (rejected duplicates are
// never recorded), so a recorded mix/project view proves scan usability and a
// recorded track view on the selected candidate proves target evidence.
func DerivePhaseGuardInput(state State, evidence PhaseGuardEvidence) PhaseGuardInput {
	closedDims := map[DiagnosticDimension]bool{}
	skippedDims := map[DiagnosticDimension]bool{}
	for _, round := range state.DiagnosticRounds {
		if round.Closed() {
			closedDims[round.PrimaryDimension] = true
		}
		for _, skip := range round.SkippedDimensions {
			skippedDims[skip.Dimension] = true
		}
	}
	queueNonEmpty := false
	for _, dim := range DefaultDimensionOrder {
		if !closedDims[dim] && !skippedDims[dim] {
			queueNonEmpty = true
			break
		}
	}
	selectedTracks := map[string]bool{}
	if state.Frontier.CandidateID != "" {
		for _, candidate := range state.Frontier.Candidates {
			if candidate.ID == state.Frontier.CandidateID {
				for _, trackID := range candidate.TrackIDs {
					selectedTracks[trackID] = true
				}
			}
		}
	}
	scanUsable, targetEvidence := false, false
	for _, record := range state.Observations {
		for _, viewID := range record.ViewIDs {
			if projectScanViewIDs[viewID] || strings.HasPrefix(viewID, "project.") {
				scanUsable = true
			}
			if strings.HasPrefix(viewID, "track.") && len(selectedTracks) > 0 && selectedTracks[record.TargetRef] {
				targetEvidence = true
			}
		}
	}
	policy := state.Policy
	if policy.MaxActionAttempts <= 0 {
		policy = DefaultPolicy()
	}
	return PhaseGuardInput{
		ProjectBound:          strings.TrimSpace(state.ProjectUUID) != "" && strings.TrimSpace(state.ProjectRevision) != "",
		CapacityAssessed:      evidence.CapacityAssessed,
		ScanUsable:            scanUsable,
		QueueNonEmpty:         queueNonEmpty,
		DimensionClosed:       len(closedDims) > 0,
		FrontierEstablished:   len(state.Frontier.Candidates) > 0,
		TargetEvidence:        targetEvidence,
		GatePassed:            evidence.GatePassed,
		AdmissionValid:        evidence.AdmissionValid,
		ContinueOnceRemaining: state.ActionAttempts < policy.MaxActionAttempts,
	}
}

// AdvancePhase walks the legal forward chain while the derived guard admits
// each step and returns the resulting state (unchanged when no step admits).
// It never skips phases: each admitted step appends its own event.
func (d Driver) AdvancePhase(state State, expectedRevision uint64, evidence PhaseGuardEvidence, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	current := state
	for !IsFSPhase(current.Phase) {
		next, err := d.TransitionPhase(current, current.Revision, PhaseFS0SemanticEntry, PhaseGuardInput{}, "fs entry", now)
		if err != nil {
			return current, err
		}
		current = next
	}
	for {
		guard := DerivePhaseGuardInput(current, evidence)
		var nextPhase Phase
		switch current.Phase {
		case PhaseFS0SemanticEntry:
			nextPhase = PhaseFS1ProjectBound
		case PhaseFS1ProjectBound:
			nextPhase = PhaseFS2CapacityAssessed
		case PhaseFS2CapacityAssessed:
			nextPhase = PhaseFS3ProjectScan
		case PhaseFS3ProjectScan:
			nextPhase = PhaseFS4DiagnosticRound
		case PhaseFS4DiagnosticRound:
			nextPhase = PhaseFS5CandidateFrontier
		case PhaseFS5CandidateFrontier:
			nextPhase = PhaseFS6TargetConfirmed
		case PhaseFS6TargetConfirmed:
			nextPhase = PhaseFS7ImprovementProposal
		case PhaseFS7ImprovementProposal:
			nextPhase = PhaseFS8ExperimentVerification
		default:
			return current, nil
		}
		if EvaluatePhaseGuard(current.Phase, nextPhase, guard) != nil {
			return current, nil
		}
		next, err := d.TransitionPhase(current, current.Revision, nextPhase, guard, "derived from closure evidence", now)
		if err != nil {
			return current, err
		}
		current = next
	}
}
