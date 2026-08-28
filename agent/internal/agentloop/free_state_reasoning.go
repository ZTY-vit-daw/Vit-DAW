package agentloop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/contextruntime"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
)

const FreeStateDecisionSchema = "free_state_decision.v1"
const FreeStateDiagnosticSchema = "free_state_diagnostic.v1"

const (
	freeStateObservationLedgerSchema = "free_state_observation_ledger.v1"
	freeStateObservationLedgerLimit  = 24
	freeStateMaxObservationRequests  = 3
)

const (
	FreeStateNeedsObservation     = "needs_observation"
	FreeStateNeedsAction          = "needs_action"
	FreeStateNeedsExperiment      = "needs_experiment"
	FreeStateDiagnosticComplete   = "diagnostic_complete"
	FreeStateNoCandidateFound     = "no_candidate_found"
	FreeStateImprovementProposal  = "improvement_proposal"
	FreeStateCapabilityBlocked    = "capability_blocked"
	FreeStateSatisfied            = "satisfied"
	FreeStateBlocked              = "blocked"
	freeStateMaxSameFamilyActions = 2
	freeStateMaxActions           = 6
)

// FreeStateDecision is the model-owned control state for an ordinary-Agent
// observation/action loop. It carries no execution authority.
type FreeStateDecision struct {
	SchemaVersion            string                             `json:"schema_version"`
	Status                   string                             `json:"status"`
	EvidenceStatus           string                             `json:"evidence_status"`
	Summary                  string                             `json:"summary"`
	RemainingIntent          string                             `json:"remaining_intent,omitempty"`
	ProcessorType            string                             `json:"processor_type,omitempty"`
	ImprovementProposal      *agentprotocol.ImprovementProposal `json:"improvement_proposal,omitempty"`
	ExperimentAdmission      *experiment.Admission              `json:"experiment_admission,omitempty"`
	ExperimentMateriality    *experiment.MaterialityEvaluation  `json:"experiment_materiality,omitempty"`
	ExperimentTargetResponse *experiment.TargetEvaluation       `json:"experiment_target_response,omitempty"`
	ExperimentRoundDecision  string                             `json:"experiment_round_decision,omitempty"`
	// SemanticProcessorIntent is the model-owned family/coverage handoff. It
	// contains no plugin identity, path, parameter ID, or vendor mapping.
	SemanticProcessorIntent *processorintent.Intent `json:"semantic_processor_intent,omitempty"`
	RequestedViewIDs        []string                `json:"requested_view_ids,omitempty"`
	// Diagnostic round metadata is model-authored evidence context. It grants
	// no execution authority; it only explains why a same-target/view-set
	// observation may be revisited.
	PriorityReason        string               `json:"priority_reason,omitempty"`
	UnresolvedQuestions   []string             `json:"unresolved_questions,omitempty"`
	DeclaredContradiction bool                 `json:"declared_contradiction,omitempty"`
	ObservationID         string               `json:"observation_id,omitempty"`
	Limitations           []string             `json:"limitations,omitempty"`
	StopReason            string               `json:"stop_reason,omitempty"`
	Diagnostic            *FreeStateDiagnostic `json:"diagnostic,omitempty"`
}

// carriesExperimentReport reports whether the decision evaluates an already
// admitted experiment (experiment report fields present) instead of
// proposing a new one. Evaluation reports bind to the experiment through the
// runtime's admission/round identity, not through a re-echoed proposal, so
// they must not be rejected for omitting improvement_proposal (2026-08-25 D1
// smoke: the FS8 evaluation report died on generic validation).
func (d FreeStateDecision) carriesExperimentReport() bool {
	return d.ExperimentMateriality != nil || d.ExperimentTargetResponse != nil ||
		strings.TrimSpace(d.ExperimentRoundDecision) != ""
}

// FreeStateDiagnostic is a model-owned, read-only diagnosis result. The
// runtime verifies only evidence references and never supplies evaluator
// truth or interprets the finding statement.
type FreeStateDiagnostic struct {
	SchemaVersion string                       `json:"schema_version"`
	Status        string                       `json:"status"`
	Findings      []FreeStateDiagnosticFinding `json:"findings,omitempty"`
	Limitations   []string                     `json:"limitations,omitempty"`
}

type FreeStateDiagnosticFinding struct {
	Statement    string         `json:"statement"`
	Scope        map[string]any `json:"scope,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs"`
	Confidence   float64        `json:"confidence,omitempty"`
	Limitation   string         `json:"limitation,omitempty"`
}

func (d *FreeStateDiagnostic) Validate() error {
	if d == nil {
		return nil
	}
	if strings.TrimSpace(d.SchemaVersion) != FreeStateDiagnosticSchema {
		return fmt.Errorf("diagnostic schema_version must be %s", FreeStateDiagnosticSchema)
	}
	switch strings.ToLower(strings.TrimSpace(d.Status)) {
	case "confirmed", "ruled_out", "unresolved":
	default:
		return fmt.Errorf("diagnostic status must be confirmed, ruled_out, or unresolved")
	}
	if len(d.Findings) == 0 && strings.ToLower(strings.TrimSpace(d.Status)) != "unresolved" {
		return fmt.Errorf("diagnostic requires at least one finding unless status=unresolved")
	}
	for i, finding := range d.Findings {
		if strings.TrimSpace(finding.Statement) == "" {
			return fmt.Errorf("diagnostic finding %d requires statement", i)
		}
		if len(finding.EvidenceRefs) == 0 {
			return fmt.Errorf("diagnostic finding %d requires evidence_refs", i)
		}
		if finding.Confidence < 0 || finding.Confidence > 1 {
			return fmt.Errorf("diagnostic finding %d confidence must be between 0 and 1", i)
		}
	}
	return nil
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
	if d.ImprovementProposal != nil {
		if err := d.ImprovementProposal.Validate(); err != nil {
			return fmt.Errorf("improvement_proposal: %w", err)
		}
		if status != FreeStateNeedsExperiment && status != FreeStateImprovementProposal {
			return fmt.Errorf("improvement_proposal requires status=%s or %s", FreeStateImprovementProposal, FreeStateNeedsExperiment)
		}
	}
	if d.ExperimentAdmission != nil {
		if err := d.ExperimentAdmission.Validate(); err != nil {
			return fmt.Errorf("experiment_admission: %w", err)
		}
	}
	if d.ExperimentMateriality != nil {
		if err := d.ExperimentMateriality.Validate(); err != nil {
			return fmt.Errorf("experiment_materiality: %w", err)
		}
	}
	if d.ExperimentTargetResponse != nil {
		if err := d.ExperimentTargetResponse.Validate(); err != nil {
			return fmt.Errorf("experiment_target_response: %w", err)
		}
	}
	if d.ExperimentRoundDecision != "" && status != FreeStateNeedsExperiment {
		return fmt.Errorf("experiment_round_decision requires status=%s", FreeStateNeedsExperiment)
	}
	if d.Diagnostic != nil {
		if err := d.Diagnostic.Validate(); err != nil {
			return err
		}
	}
	switch status {
	case FreeStateNeedsObservation, "observation_in_progress":
		// Catalog discovery is a model-visible observation step but does not
		// request a view set.  An empty requested_view_ids is therefore valid
		// only when the turn calls ccb.observation_catalog; a concrete
		// ccb.observation_request is still required to carry a non-empty set and
		// is checked by the message-loop output gate.
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
	case FreeStateNeedsExperiment, FreeStateImprovementProposal:
		if d.ImprovementProposal == nil && !d.carriesExperimentReport() {
			return fmt.Errorf("improvement proposal state requires improvement_proposal")
		}
		if strings.TrimSpace(d.EvidenceStatus) == "" {
			return fmt.Errorf("improvement proposal state requires evidence_status")
		}
	case FreeStateDiagnosticComplete, FreeStateNoCandidateFound:
		if strings.ToLower(strings.TrimSpace(d.EvidenceStatus)) != "sufficient" {
			return fmt.Errorf("%s requires evidence_status=sufficient", status)
		}
		if d.Diagnostic == nil {
			return fmt.Errorf("%s requires an evidence-backed diagnostic", status)
		}
	case FreeStateSatisfied:
		if strings.ToLower(strings.TrimSpace(d.EvidenceStatus)) != "sufficient" {
			return fmt.Errorf("satisfied requires evidence_status=sufficient")
		}
	case FreeStateBlocked, FreeStateCapabilityBlocked:
		// Summary is the canonical human-readable boundary. stop_reason and
		// limitations add structure when available but are not redundant gates.
	default:
		return fmt.Errorf("status must be needs_observation, needs_action, improvement_proposal, needs_experiment, diagnostic_complete, no_candidate_found, capability_blocked, or a supported legacy state")
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
	out.UnresolvedQuestions = append([]string(nil), in.UnresolvedQuestions...)
	if in.ExperimentAdmission != nil {
		out.ExperimentAdmission = cloneExperimentAdmission(in.ExperimentAdmission)
	}
	if in.ExperimentMateriality != nil {
		out.ExperimentMateriality = cloneExperimentMateriality(in.ExperimentMateriality)
	}
	if in.ExperimentTargetResponse != nil {
		out.ExperimentTargetResponse = cloneExperimentTargetResponse(in.ExperimentTargetResponse)
	}
	out.Limitations = append([]string(nil), in.Limitations...)
	if in.ImprovementProposal != nil {
		proposal := *in.ImprovementProposal
		proposal.EvidenceRefs = append([]string(nil), in.ImprovementProposal.EvidenceRefs...)
		proposal.Limitations = append([]string(nil), in.ImprovementProposal.Limitations...)
		proposal.NeedsResolution = append([]string(nil), in.ImprovementProposal.NeedsResolution...)
		proposal.Target = cloneMap(in.ImprovementProposal.Target)
		proposal.ParameterBounds = cloneMap(in.ImprovementProposal.ParameterBounds)
		proposal.VerificationPlan = cloneMap(in.ImprovementProposal.VerificationPlan)
		out.ImprovementProposal = &proposal
	}
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
	if in.Diagnostic != nil {
		diagnostic := *in.Diagnostic
		diagnostic.Limitations = append([]string(nil), in.Diagnostic.Limitations...)
		diagnostic.Findings = make([]FreeStateDiagnosticFinding, len(in.Diagnostic.Findings))
		for i, finding := range in.Diagnostic.Findings {
			diagnostic.Findings[i] = finding
			diagnostic.Findings[i].EvidenceRefs = append([]string(nil), finding.EvidenceRefs...)
			diagnostic.Findings[i].Scope = cloneMap(finding.Scope)
		}
		out.Diagnostic = &diagnostic
	}
	return &out
}

func cloneExperimentAdmission(in *experiment.Admission) *experiment.Admission {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	out := &experiment.Admission{}
	if json.Unmarshal(data, out) != nil {
		return nil
	}
	return out
}

func cloneExperimentMateriality(in *experiment.MaterialityEvaluation) *experiment.MaterialityEvaluation {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	out := &experiment.MaterialityEvaluation{}
	if json.Unmarshal(data, out) != nil {
		return nil
	}
	return out
}

func cloneExperimentTargetResponse(in *experiment.TargetEvaluation) *experiment.TargetEvaluation {
	if in == nil {
		return nil
	}
	data, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	out := &experiment.TargetEvaluation{}
	if json.Unmarshal(data, out) != nil {
		return nil
	}
	return out
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
		"cycle", "max_cycles", "continuation_budget", "continuation_used", "observation_ids", "latest_decision",
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
			"processor_type", "improvement_proposal", "experiment_admission", "experiment_materiality", "experiment_target_response", "experiment_round_decision", "semantic_processor_intent", "diagnostic", "requested_view_ids", "priority_reason", "unresolved_questions", "declared_contradiction", "observation_id", "limitations", "stop_reason",
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

// messageLoopFreeStateJudgmentBoundary reports whether the experiment in the
// loop context is durably parked at the human-judgment boundary. GLM ruling 3:
// the boundary persists at experiment scope — any earlier round that requested
// a user judgment without the judgment landing keeps the whole experiment
// parked (only the settle family is admitted afterwards); the current round
// additionally parks while a recorded judgment awaits its settle decision.
func messageLoopFreeStateJudgmentBoundary(state *runState) bool {
	if state == nil {
		return false
	}
	loop := messageLoopFreeStateContext(state)
	experimentRow := messageLoopMapValue(loop["experiment"])
	status := strings.ToLower(firstMapText(experimentRow, "status"))
	if status == string(experiment.StatusSettled) || status == string(experiment.StatusStopped) {
		return false
	}
	rounds := messageLoopMapRows(experimentRow["rounds"])
	if len(rounds) == 0 {
		return false
	}
	for _, round := range rounds {
		if freeStateBool(round["user_judgment_requested"]) && len(messageLoopMapRows(round["user_judgment_evidence"])) == 0 {
			return true
		}
	}
	round := rounds[len(rounds)-1]
	if len(messageLoopMapRows(round["user_judgment_evidence"])) > 0 {
		return true
	}
	return strings.ToLower(strings.TrimSpace(firstMapText(round, "decision"))) == string(experiment.DecisionUserJudgment)
}

// messageLoopFreeStateJudgmentBoundaryIssue is the final-gate defense-in-depth
// for the human-judgment boundary. Even if a scheduler continuation slips
// through the chat-side guards, a model decision that would revive the loop
// (observation / action / new admission) is rejected here; only the
// settle-family round decisions remain legal.
func messageLoopFreeStateJudgmentBoundaryIssue(state *runState, decision *FreeStateDecision) string {
	if state == nil || decision == nil || !messageLoopFreeStateJudgmentBoundary(state) {
		return ""
	}
	if strings.TrimSpace(decision.ExperimentRoundDecision) != "" {
		switch experiment.RoundDecision(strings.TrimSpace(decision.ExperimentRoundDecision)) {
		case experiment.DecisionRetain, experiment.DecisionRollback, experiment.DecisionStopped:
			return ""
		}
	}
	return "the experiment round is parked at the human-judgment boundary; the model may only report the final settle round decision (retained / rolled_back / stopped-ambiguous) — no further observation, action, or new admission is permitted"
}

// A successful catalog call is a durable observation boundary. Repeating it
// cannot add evidence and previously caused scheduler continuations to spend
// their entire bounded budget rediscovering the same view IDs.
func messageLoopFreeStateCatalogAlreadyObserved(state *runState) bool {
	if state == nil {
		return false
	}
	for _, event := range state.trace {
		if event.ToolResult == nil || !messageLoopIsCCBObservationCatalogName(event.ToolResult.Tool) || toolStatusFailed(event.ToolResult.Status) {
			continue
		}
		catalog := messageLoopMapValue(event.ToolResult.Result["catalog"])
		if len(messageLoopMapRows(catalog["views"])) > 0 {
			return true
		}
	}
	return false
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
	if issue := messageLoopFreeStateJudgmentBoundaryIssue(state, out.FreeStateDecision); issue != "" {
		return issue
	}
	if err := out.FreeStateDecision.Validate(); err != nil {
		return "invalid free_state decision: " + err.Error()
	}
	diagnosticOnly := messageLoopFreeStateDiagnosticOnly(state)
	status := strings.ToLower(strings.TrimSpace(out.FreeStateDecision.Status))
	if issue := messageLoopFreeStatePhaseDecisionIssue(state, status); issue != "" {
		return issue
	}
	switch status {
	case FreeStateNeedsObservation:
		if diagnosticOnly && messageLoopFreeStateDiagnosticEvidenceWindowClosed(state) {
			return "diagnostic-only observation window is closed after a usable non-structural CCB observation; do not call another tool and return your own terminal free_state_diagnostic.v1 conclusion now (use unresolved with limitations when the available evidence is insufficient)"
		}
		if out.Final || len(out.ToolCalls) == 0 {
			return "needs_observation must be non-final and call ccb.observation_catalog or ccb.observation_request"
		}
		requestCalls := make([]planner.ToolCall, 0, len(out.ToolCalls))
		requestFingerprints := map[string]bool{}
		for _, call := range out.ToolCalls {
			if !messageLoopIsCCBObservationTool(call) {
				return "needs_observation may call only CCB observation catalog/request tools in the free-state loop"
			}
			if messageLoopIsCCBObservationCatalogName(normalizedActionName(call, executorpkg.Result{})) && messageLoopFreeStateCatalogAlreadyObserved(state) {
				return "ccb.observation_catalog already returned a successful catalog in this loop; request a cataloged observation view or return the concrete evidence boundary"
			}
			if !messageLoopIsCCBObservationRequestName(normalizedActionName(call, executorpkg.Result{})) {
				continue
			}
			requestCalls = append(requestCalls, call)
			if issue := messageLoopFreeStateTrackTargetIssue(state, call); issue != "" {
				return issue
			}
			callViews := messageLoopStringList(call.Args["view_ids"])
			if _, issue := messageLoopFreeStateNormalizedCallViewSet(call.Args["view_ids"]); issue != "" {
				return "ccb.observation_request view_ids " + issue
			}
			fingerprint := messageLoopFreeStateRequestFingerprint(callViews, messageLoopCCBTargetFromCall(call))
			if fingerprint == "" {
				return "ccb.observation_request must have a deterministic view-set and target fingerprint"
			}
			if requestFingerprints[fingerprint] {
				return "needs_observation must not repeat the same CCB view set and target in one turn"
			}
			requestFingerprints[fingerprint] = true
			if issue := messageLoopFreeStateRejectedViewSetIssue(state, callViews, messageLoopCCBTargetFromCall(call)); issue != "" {
				return issue
			}
			if issue := messageLoopFreeStateAlreadyObservedIssue(state, callViews, messageLoopCCBTargetFromCall(call), freeStateReObservationMetadata(out.FreeStateDecision)); issue != "" {
				return issue
			}
		}
		if len(requestCalls) > freeStateMaxObservationRequests {
			return fmt.Sprintf("needs_observation may contain at most %d distinct ccb.observation_request calls in one turn", freeStateMaxObservationRequests)
		}
		if issue := messageLoopFreeStateRequestedCallsIssue(out.FreeStateDecision.RequestedViewIDs, requestCalls); issue != "" {
			return issue
		}
		if issue := messageLoopFreeStateCandidateProgressionIssue(state, status, requestCalls); issue != "" {
			return issue
		}
	case FreeStateNeedsAction:
		if diagnosticOnly {
			return "diagnostic-only free-state turns must end with a diagnostic conclusion and may not enter a processor action"
		}
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
		if issue := messageLoopFreeStateCandidateProgressionIssue(state, status, nil); issue != "" {
			return issue
		}
		if count := messageLoopFreeStateActionCount(ctx); count >= freeStateMaxActions {
			return fmt.Sprintf("the free-state loop reached its bounded %d-action limit; return blocked instead of writing another processor action", freeStateMaxActions)
		}
		if count := messageLoopFreeStateProcessorApplyCount(ctx, out.FreeStateDecision.ProcessorType); count >= freeStateMaxSameFamilyActions {
			return fmt.Sprintf("the same processor family reached its bounded %d-action limit; return blocked or address a different unresolved clause", freeStateMaxSameFamilyActions)
		}
	case FreeStateNeedsExperiment, FreeStateImprovementProposal:
		if diagnosticOnly {
			return "diagnostic-only free-state turns must end with a diagnostic conclusion and may not enter an improvement experiment"
		}
		if len(out.ToolCalls) != 0 {
			return "needs_experiment must contain no direct mutation tool calls; the existing governed execution layer owns materialization and confirmation"
		}
		// A decision carrying experiment report fields evaluates an already
		// admitted experiment; it is not a new admission. The G1-G7 gate
		// re-checks pre-apply state (project binding revision, frontier,
		// target evidence) that legitimately changed after Apply, so running
		// it again rejected every evaluation report at FS8 (2026-08-25 D1
		// smoke: "failed: G1_project_binding, G5_frontier_established,
		// G6_target_evidence"). The report itself is validated by the
		// experiment schema validators; the settlement path still enforces
		// the one-round/single-mutation budget.
		if out.FreeStateDecision.ExperimentMateriality != nil || out.FreeStateDecision.ExperimentTargetResponse != nil ||
			strings.TrimSpace(out.FreeStateDecision.ExperimentRoundDecision) != "" {
			if m := out.FreeStateDecision.ExperimentMateriality; m != nil {
				if err := m.Validate(); err != nil {
					return "invalid experiment_materiality: " + err.Error()
				}
			}
			if tr := out.FreeStateDecision.ExperimentTargetResponse; tr != nil {
				if err := tr.Validate(); err != nil {
					return "invalid experiment_target_response: " + err.Error()
				}
			}
			// Symmetric with the satisfied/blocked branches: after an applied
			// action the settle report is only legal once the fresh post-action
			// CCB observation has returned in this reasoning cycle. Without this
			// gate the settle turn skipped the observation and parked the round
			// at the human judgment boundary with zero post-action evidence
			// (2026-08-28 10:27/10:42 smokes).
			ctx := messageLoopFreeStateContext(state)
			if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
				return "cannot report experiment materiality/target response after an applied action until a fresh CCB observation_request has returned in this reasoning cycle; request the post-action observation first, then settle the experiment on its evidence"
			}
		} else if failed := evaluateFreeStateNeedsExperimentGate(state, out.FreeStateDecision); len(failed) > 0 {
			// The single-usable-bundle weak gate is replaced by the seven-part
			// admission gate (docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md). Gate
			// failure has exactly one legal exit: needs_observation.
			return fmt.Sprintf("needs_experiment requires the full admission gate; failed: %s; return needs_observation with the next bounded observation instead", strings.Join(failed, ", "))
		}
	case FreeStateSatisfied, FreeStateDiagnosticComplete, FreeStateNoCandidateFound:
		if len(out.ToolCalls) != 0 {
			return status + " must contain no tool calls"
		}
		if status == FreeStateNoCandidateFound {
			if issue := messageLoopFreeStateNoCandidateIssue(state); issue != "" {
				return issue
			}
			if messageLoopFreeStateQueueStillOpen(state) {
				return "no_candidate_found is not valid while the diagnostic priority queue still has open dimensions; observe the next queued dimension or return blocked with the concrete evidence boundary"
			}
			if messageLoopFreeStateClaimsProjectPerfect(*out.FreeStateDecision, out.Reply) {
				return "no_candidate_found is an exhausted-queue boundary, not a clean bill of health; describe the bounded evidence searched and the questions left open instead of asserting the project has no problems"
			}
		}
		if status == FreeStateSatisfied && strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
			return "satisfied is a legacy local conclusion and cannot settle an open improvement contract; return needs_experiment with an evidence-backed improvement_proposal, no_candidate_found with a bounded diagnostic, or capability_blocked with the concrete boundary"
		}
		// The improvement contract settles only through a governed experiment
		// outcome, no_candidate_found, or a capability boundary (its own
		// completion criteria). A confirmed diagnostic is exactly the moment
		// an improvement task must convert that finding into a bounded
		// proposal; letting diagnostic_complete settle here silently turns
		// the open improvement task into an unrequested diagnostic-only run
		// (2026-08-25 D1 smoke: sufficient masking evidence closed as
		// diagnostic_complete and the experiment chain never started).
		if status == FreeStateDiagnosticComplete && !diagnosticOnly && strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
			return "diagnostic_complete cannot settle an open improvement contract; convert the confirmed diagnostic into needs_experiment with one bounded improvement_proposal citing the confirmed findings, or return no_candidate_found with a bounded ruled-out diagnostic, or capability_blocked with the concrete boundary"
		}
		ctx := messageLoopFreeStateContext(state)
		if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
			return "cannot mark the original intent satisfied after an action until a fresh CCB observation_request has returned in this reasoning cycle"
		}
		if issue := messageLoopFreeStateRoundPendingSettlementIssue(ctx); issue != "" {
			return issue
		}
		if diagnosticOnly {
			if issue := messageLoopFreeStateDiagnosticIssue(state, out.FreeStateDecision); issue != "" {
				return issue
			}
		}
	case FreeStateBlocked, FreeStateCapabilityBlocked:
		if len(out.ToolCalls) != 0 {
			return "blocked must contain no tool calls"
		}
		if messageLoopFreeStateClaimsUnavailableObservation(state, *out.FreeStateDecision) {
			return "cannot claim that observation is unavailable: ccb.observation_catalog and ccb.observation_request are available in this free-state turn; use the CCB catalog/request protocol, then decide the treatment family from the returned bounded evidence and limitations"
		}
		// Symmetric with the satisfied/no_candidate gate above: after an
		// applied action the loop must not settle on any terminal — blocked
		// included — until a fresh post-action CCB observation has returned
		// in this reasoning turn. Without this, blocked is an escape hatch
		// that skips the mandatory post-action evidence step (2026-08-25 D1
		// smoke: the post-action turn settled capability_blocked on a
		// pre-action authorization objection without ever observing rev 20).
		ctx := messageLoopFreeStateContext(state)
		if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
			return "cannot return blocked after an applied action until a fresh CCB observation_request has returned in this reasoning cycle; request the post-action observation first, then settle the experiment on its evidence"
		}
		if issue := messageLoopFreeStateRoundPendingSettlementIssue(ctx); issue != "" {
			return issue
		}
		if diagnosticOnly {
			if issue := messageLoopFreeStateDiagnosticIssue(state, out.FreeStateDecision); issue != "" {
				return issue
			}
		}
	}
	return ""
}

// messageLoopFreeStateRoundPendingSettlementIssue refuses bare terminal
// decisions while an applied D1 experiment round has its fresh post-action
// observation recorded but no settlement yet. Without this the post-action
// turn strands the round on a bare capability_blocked and the human judgment
// boundary never opens (2026-08-28 12:41/12:34 smokes: settle evidence booked
// deterministically, the model still returned bare terminals).
func messageLoopFreeStateRoundPendingSettlementIssue(ctx map[string]any) string {
	if len(ctx) == 0 || !strings.EqualFold(strings.TrimSpace(messageLoopText(ctx["decision_phase"])), "post_action_evaluation") {
		return ""
	}
	experiment := messageLoopMapValue(ctx["experiment"])
	if len(experiment) == 0 || !strings.EqualFold(strings.TrimSpace(messageLoopText(experiment["status"])), "running") {
		return ""
	}
	rounds := messageLoopMapRows(experiment["rounds"])
	if len(rounds) == 0 {
		return ""
	}
	round := rounds[len(rounds)-1]
	if strings.TrimSpace(messageLoopText(round["decision"])) != "" {
		return ""
	}
	for _, observation := range messageLoopMapRows(round["observations"]) {
		if freeStateBool(observation["post_action"]) && freeStateBool(observation["fresh"]) {
			return "the applied experiment round has fresh post-action evidence recorded and is pending settlement; report experiment_materiality + experiment_target_response + experiment_round_decision=user_judgment_pending on the preserved proposal (or the concrete blocked boundary that prevents evaluating the recorded evidence), not a bare terminal"
		}
	}
	return ""
}

func messageLoopTaskContractKind(state *runState) string {
	if state == nil {
		return ""
	}
	contract := messageLoopMapValue(state.input.Context["task_contract"])
	return strings.ToLower(messageLoopText(contract["kind"]))
}

func messageLoopFreeStateCandidateProgressionIssue(state *runState, status string, calls []planner.ToolCall) string {
	if state == nil {
		return ""
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	candidates := messageLoopMapRows(frontier["candidates"])
	if len(candidates) == 0 {
		return ""
	}
	selected := messageLoopText(frontier["candidate_id"])
	allowedTracks := map[string]bool{}
	for _, candidate := range candidates {
		if selected != "" && messageLoopText(candidate["id"]) != selected {
			continue
		}
		for _, trackID := range messageLoopStringList(candidate["track_ids"]) {
			allowedTracks[trackID] = true
		}
	}
	if len(allowedTracks) == 0 {
		return "closure candidate frontier is malformed; return blocked with the evidence boundary"
	}
	targetObservedNow := messageLoopFreeStateCandidateTargetObserved(state, allowedTracks)
	if status == FreeStateNeedsAction {
		if selected == "" && !targetObservedNow {
			return "closure candidates require an explicit target-level observation before needs_action; select one listed candidate track first"
		}
		return ""
	}
	if status != FreeStateNeedsObservation {
		return ""
	}
	// CandidateID is written only after an admitted track-level observation
	// lands on one of the candidate tracks. At that point the minimal closure
	// has completed its one narrowing step: another observation would turn the
	// bounded closure back into an open-ended inspection loop. The model must
	// either hand off an evidence-supported action, propose a bounded
	// improvement experiment, or state the concrete evidence/capability
	// boundary.
	if selected != "" || targetObservedNow {
		if status == FreeStateNeedsObservation && messageLoopFreeStateCandidateTargetEvidencePartial(state, allowedTracks) {
			// Partial target evidence is a continuation point, not a bounded
			// closure. Keep the candidate frontier active so the model can
			// inspect another candidate or request a more discriminating view.
			return ""
		}
		return "the selected closure candidate already has target-level evidence; return needs_action when sufficient or blocked with the concrete action-preflight boundary now"
	}
	if len(calls) == 0 {
		return "closure candidate frontier requires a target-level ccb.observation_request"
	}
	for _, call := range calls {
		if !messageLoopIsCCBObservationRequestName(normalizedActionName(call, executorpkg.Result{})) {
			return "closure candidate frontier forbids catalog discovery until the candidate is resolved"
		}
		for _, viewID := range messageLoopStringList(call.Args["view_ids"]) {
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "track.") {
				return "closure candidate frontier requires only target-level track.* observations; project.* and mix.* cannot be retried now"
			}
		}
		target := messageLoopCCBTargetFromCall(call)
		if !strings.EqualFold(messageLoopText(target["kind"]), "track") || !allowedTracks[messageLoopText(target["id"])] {
			return "closure candidate frontier requires target_ref.kind=track and an id from the candidate track set"
		}
	}
	return ""
}

func messageLoopFreeStateCandidateTargetObserved(state *runState, allowedTracks map[string]bool) bool {
	if state == nil || state.recentObservation == nil {
		return false
	}
	observation := state.recentObservation
	if !messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(messageLoopText(observation.Summary["status"])))
	if status != "ready" && status != "partial" {
		return false
	}
	target := messageLoopMapValue(observation.Summary["target_ref"])
	return strings.EqualFold(messageLoopText(target["kind"]), "track") && allowedTracks[messageLoopText(target["id"])]
}

func messageLoopFreeStateCandidateTargetEvidencePartial(state *runState, allowedTracks map[string]bool) bool {
	if !messageLoopFreeStateCandidateTargetObserved(state, allowedTracks) {
		return false
	}
	observation := state.recentObservation
	if strings.EqualFold(strings.TrimSpace(messageLoopText(observation.Summary["status"])), "partial") {
		return true
	}
	for _, raw := range messageLoopMapValue(observation.Summary["views"]) {
		view := messageLoopMapValue(raw)
		if strings.EqualFold(strings.TrimSpace(messageLoopText(view["status"])), "partial") {
			return true
		}
	}
	return false
}

// A partial target observation is still a valid bounded hypothesis input. Once
// it is present in the durable observation ledger, repeatedly seeking another
// view can consume the closure discovery budget without increasing the
// decision quality. The model should hand off to needs_experiment instead.
func messageLoopFreeStatePartialTargetEvidenceAlreadyAvailable(state *runState, allowedTracks map[string]bool) bool {
	if state == nil {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	for _, raw := range messageLoopMapValue(ledger["available_views"]) {
		view := messageLoopMapValue(raw)
		target := messageLoopMapValue(view["target_ref"])
		if !strings.EqualFold(messageLoopText(target["kind"]), "track") || !allowedTracks[messageLoopText(target["id"])] {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(messageLoopText(view["status"])))
		if status == "partial" && (messageLoopText(view["observation_id"]) != "" || len(messageLoopStringList(view["evidence_refs"])) > 0) {
			return true
		}
	}
	return false
}

func messageLoopFreeStateNoCandidateIssue(state *runState) string {
	if state == nil || !strings.EqualFold(messageLoopTaskContractKind(state), "improvement") {
		return ""
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	if len(messageLoopMapRows(frontier["candidates"])) == 0 {
		return ""
	}
	return "no_candidate_found is not valid while the bounded candidate frontier still contains unresolved candidates; return needs_experiment with one bounded proposal or capability_blocked with the concrete evidence boundary"
}

// messageLoopFreeStateQueueStillOpen consumes the diagnostic priority queue:
// no_candidate_found is only admissible once the queue has no open dimension
// left (or no queue is tracked at all).
func messageLoopFreeStateQueueStillOpen(state *runState) bool {
	if state == nil {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	raw, ok := ctx["priority_queue"]
	if !ok {
		if closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"]); closure != nil {
			raw, ok = closure["priority_queue"]
		}
		if !ok {
			return false
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return true
	}
	queue := audioclosure.PriorityQueue{}
	// Fail closed: a queue that is present but unparseable or invalid cannot
	// prove the diagnostic dimensions are exhausted.
	if json.Unmarshal(data, &queue) != nil || queue.SchemaVersion != audioclosure.PriorityQueueSchema || queue.Validate() != nil {
		return true
	}
	return queue.HasOpen()
}

// messageLoopFreeStateDiagnosticEvidenceWindowClosed bounds only the
// diagnostic-only experiment. A structure-only observation may be needed to
// discover visible track identities, but the first usable acoustic/project
// relationship view closes the observation window. The model still owns the
// finding and may conclude unresolved; the runtime supplies no diagnosis.
func messageLoopFreeStateDiagnosticEvidenceWindowClosed(state *runState) bool {
	if !messageLoopFreeStateDiagnosticOnly(state) || state == nil {
		return false
	}
	for _, record := range state.executed {
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if !messageLoopIsCCBObservationRequestName(name) || !messageLoopExecutionSucceeded(record) {
			continue
		}
		if messageLoopFreeStateBundleHasDiagnosticEvidence(messageLoopMapValue(messageLoopMapValue(record["result"])["bundle"])) {
			return true
		}
	}
	if observation := state.recentObservation; observation != nil && observation.Error == "" && !toolStatusFailed(observation.Status) &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) &&
		messageLoopFreeStateBundleHasDiagnosticEvidence(observation.Summary) {
		return true
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	for viewID, raw := range messageLoopMapValue(ledger["available_views"]) {
		if messageLoopFreeStateDiagnosticViewUsable(viewID, messageLoopMapValue(raw)) {
			return true
		}
	}
	return false
}

func messageLoopFreeStateBundleHasDiagnosticEvidence(bundle map[string]any) bool {
	if !messageLoopCCBObservationBundleUsable(bundle) {
		return false
	}
	for viewID, raw := range messageLoopMapValue(bundle["views"]) {
		if messageLoopFreeStateDiagnosticViewUsable(viewID, messageLoopMapValue(raw)) {
			return true
		}
	}
	return false
}

func messageLoopFreeStateDiagnosticViewUsable(viewID string, view map[string]any) bool {
	if strings.EqualFold(strings.TrimSpace(viewID), "project.structure") {
		return false
	}
	projectionStatus := strings.ToLower(strings.TrimSpace(firstMapText(view, "projection_status")))
	if projectionStatus == "unsupported_projection" || projectionStatus == "omitted" || projectionStatus == "deferred" {
		return false
	}
	switch strings.ToLower(firstMapText(view, "status")) {
	case "ready", "partial":
		return true
	default:
		return false
	}
}

func messageLoopFreeStateDiagnosticOnly(state *runState) bool {
	if state == nil {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	for _, key := range []string{"diagnostic_only", "free_state_diagnostic_only"} {
		if freeStateBool(ctx[key]) || freeStateBool(state.input.Context[key]) {
			return true
		}
	}
	return false
}

func messageLoopFreeStateDiagnosticIssue(state *runState, decision *FreeStateDecision) string {
	if decision == nil || decision.Diagnostic == nil {
		return "diagnostic-only free-state terminal decisions require free_state_diagnostic.v1"
	}
	if err := decision.Diagnostic.Validate(); err != nil {
		return "invalid free_state diagnostic: " + err.Error()
	}
	known := messageLoopFreeStateKnownDiagnosticRefs(state)
	for _, finding := range decision.Diagnostic.Findings {
		for _, ref := range finding.EvidenceRefs {
			ref = strings.TrimSpace(ref)
			if ref == "" || !known[ref] {
				return "diagnostic evidence_refs must reference an observation or evidence returned in the current free-state loop"
			}
		}
	}
	return ""
}

func messageLoopFreeStateKnownDiagnosticRefs(state *runState) map[string]bool {
	known := map[string]bool{}
	if state == nil {
		return known
	}
	add := func(value any) {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			known[text] = true
		}
	}
	addObservation := func(summary map[string]any) {
		if len(summary) == 0 {
			return
		}
		add(summary["observation_id"])
		for _, ref := range messageLoopStringList(summary["evidence_refs"]) {
			add(ref)
		}
	}
	if state.recentObservation != nil && !toolStatusFailed(state.recentObservation.Status) {
		if messageLoopFreeStateObservationStatusUsable(state.recentObservation.Summary) {
			addObservation(state.recentObservation.Summary)
		}
	}
	ctx := messageLoopFreeStateContext(state)
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	for _, row := range messageLoopMapValue(ledger["available_views"]) {
		view := messageLoopMapValue(row)
		if messageLoopFreeStateObservationStatusUsable(view) {
			addObservation(view)
		}
	}
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if messageLoopFreeStateObservationStatusUsable(row) {
			addObservation(row)
		}
	}
	return known
}

func messageLoopFreeStateObservationStatusUsable(value map[string]any) bool {
	if len(value) == 0 {
		return false
	}
	for _, key := range []string{"status", "bundle_status"} {
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value[key]))) {
		case "ready", "partial":
			return true
		}
	}
	return false
}

func messageLoopFreeStateRejectedViewSetIssue(state *runState, requested []string, target map[string]any) string {
	want := messageLoopFreeStateRequestFingerprint(requested, target)
	if want == "" {
		return ""
	}
	if state != nil && state.recentObservation != nil &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(state.recentObservation.Tool, state.recentObservation.CommandName)) &&
		strings.EqualFold(firstMapText(state.recentObservation.Summary, "status", "bundle_status"), "rejected") &&
		messageLoopFreeStateRequestFingerprint(messageLoopStringList(state.recentObservation.Summary["requested_views"]), messageLoopMapValue(state.recentObservation.Summary["target_ref"])) == want {
		return "the requested CCB view set was rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
	}
	ctx := messageLoopFreeStateContext(state)
	for _, row := range messageLoopMapRows(ctx["rejected_observation_requests"]) {
		if messageLoopFreeStateRequestFingerprint(messageLoopStringList(row["requested_views"]), messageLoopMapValue(row["target_ref"])) == want {
			return "the requested CCB view set was already rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
		}
	}
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["rejected_view_sets"]) {
		if messageLoopFreeStateRequestFingerprint(messageLoopStringList(row["requested_views"]), messageLoopMapValue(row["target_ref"])) == want {
			return "the requested CCB view set was already rejected; choose a different catalog view set or return blocked instead of retrying the same rejected/deferred views"
		}
	}
	return ""
}

func messageLoopFreeStateAlreadyObservedIssue(state *runState, requested []string, target map[string]any, metadata ...map[string]any) string {
	want := messageLoopFreeStateRequestFingerprint(requested, target)
	if want == "" || state == nil {
		return ""
	}
	if freeStateReObservationAdmitted(state, want, requested, metadata...) {
		return ""
	}
	if observation := state.recentObservation; observation != nil &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) &&
		messageLoopFreeStateObservationStatusUsable(observation.Summary) &&
		messageLoopFreeStateRequestFingerprint(messageLoopStringList(observation.Summary["requested_views"]), messageLoopMapValue(observation.Summary["target_ref"])) == want {
		return "the requested CCB view set and target already returned usable evidence; choose a different cataloged view or target instead of repeating it"
	}
	ctx := messageLoopFreeStateContext(state)
	ledger := messageLoopMapValue(ctx["observation_ledger"])
	if allProjectScopedCCBViews(requested) {
		available := messageLoopMapValue(ledger["available_views"])
		allAvailable := true
		for _, viewID := range requested {
			if len(messageLoopMapValue(available[strings.TrimSpace(viewID)])) == 0 {
				allAvailable = false
				break
			}
		}
		if allAvailable {
			return "the requested project-level CCB view set already returned usable evidence; choose a different cataloged view or target instead of repeating it"
		}
	}
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if !messageLoopFreeStateObservationStatusUsable(row) {
			continue
		}
		if messageLoopFreeStateRequestFingerprint(messageLoopStringList(row["requested_views"]), messageLoopMapValue(row["target_ref"])) == want {
			return "the requested CCB view set and target already returned usable evidence; choose a different cataloged view or target instead of repeating it"
		}
		// Project-level views are scoped to the whole project. Treat an omitted
		// target and an explicit current-project target as the same observation;
		// otherwise a continuation can evade the duplicate-view guard by changing
		// only the redundant target encoding.
		priorViews := messageLoopStringList(row["requested_views"])
		if sameStringSet(priorViews, requested) && allProjectScopedCCBViews(requested) {
			return "the requested project-level CCB view set already returned usable evidence; choose a different cataloged view or target instead of repeating it"
		}
	}
	return ""
}

func allProjectScopedCCBViews(viewIDs []string) bool {
	if len(viewIDs) == 0 {
		return false
	}
	for _, viewID := range viewIDs {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "project.") &&
			!strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "mix.") {
			return false
		}
	}
	return true
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]bool, len(left))
	for _, value := range left {
		seen[strings.TrimSpace(value)] = true
	}
	for _, value := range right {
		if !seen[strings.TrimSpace(value)] {
			return false
		}
	}
	return true
}

func messageLoopFreeStateRequestFingerprint(values []string, target map[string]any) string {
	viewFingerprint := messageLoopFreeStateViewFingerprint(values)
	if viewFingerprint == "" {
		return ""
	}
	kind, id := messageLoopCCBTargetIdentity(target)
	if kind == "" && id == "" {
		return viewFingerprint
	}
	digest := sha256.Sum256([]byte(viewFingerprint + "\x1f" + kind + "\x1f" + id))
	return "ccb_request:" + hex.EncodeToString(digest[:8])
}

func messageLoopCCBTargetIdentity(target map[string]any) (string, string) {
	kind := strings.ToLower(strings.TrimSpace(firstMapText(target, "kind", "target_kind")))
	id := strings.TrimSpace(firstMapText(target, "id", "target_id", "track_id", "clip_id"))
	return kind, id
}

func messageLoopCCBTargetFromCall(call planner.ToolCall) map[string]any {
	if target := messageLoopMapValue(call.Args["target_ref"]); len(target) > 0 {
		return compactSelectedKeys(target, []string{"kind", "id", "label", "track_id"})
	}
	kind := firstMapText(call.Args, "target_kind")
	id := firstMapText(call.Args, "target_id", "track_id", "clip_id")
	if kind == "" && id == "" {
		return nil
	}
	return compactSelectedKeys(map[string]any{
		"kind": kind, "id": id, "label": firstMapText(call.Args, "target_label"),
	}, []string{"kind", "id", "label"})
}

func messageLoopFreeStateTrackTargetIssue(state *runState, call planner.ToolCall) string {
	// Legacy/unit free-state callers may intentionally exercise an unbound
	// observation request. The stricter target contract applies to the real
	// open-semantic entry, where project-wide track evidence must be auditable.
	route, verified := messageLoopSemanticEntryRoute(state)
	if !verified || route != "open_semantic" {
		return ""
	}
	hasTrackView := false
	for _, viewID := range messageLoopStringList(call.Args["view_ids"]) {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(viewID)), "track.") {
			hasTrackView = true
			break
		}
	}
	if !hasTrackView {
		return ""
	}
	kind, id := messageLoopCCBTargetIdentity(messageLoopCCBTargetFromCall(call))
	if kind == "track" && id != "" {
		return ""
	}
	// A pre-bound single-track task remains compatible with the existing
	// deterministic binding path. Project-wide free-state inspection has no
	// such authority and must receive a model-selected visible track target.
	loopTarget := messageLoopMapValue(messageLoopFreeStateContext(state)["target_ref"])
	boundKind, boundID := messageLoopCCBTargetIdentity(loopTarget)
	if state != nil && firstMapText(state.input.Context, "selected_track_id", "selected_plugin_track_id") != "" {
		return ""
	}
	if boundKind == "track" && boundID != "" {
		return ""
	}
	return "track.* observation views require args.target_ref with kind=track and an exact model-visible track id; request project structure first if track identities are not yet visible"
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
	mergeFreeStateObservationLedger(ledger, observation, state.turnsUsed)
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

// syncFreeStateRuntimeAfterObservation keeps the in-flight MessageLoop
// snapshot coherent with a CCB result.  The chat controller remains the
// authoritative closure owner and persists the same observation at the HTTP
// boundary; this projection only prevents the next final-gate evaluation in
// this Runner invocation from combining a new ledger with an old closure
// revision/phase.
func syncFreeStateRuntimeAfterObservation(state *runState, observation *RecentObservation) {
	if state == nil || observation == nil || !messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) ||
		!messageLoopFreeStateObservationStatusUsable(observation.Summary) {
		return
	}
	ctx := state.input.Context
	if len(ctx) == 0 {
		return
	}
	closure := messageLoopMapValue(ctx["minimal_audio_closure"])
	if len(closure) == 0 {
		return
	}
	obsRevision := firstNonEmpty(firstMapText(observation.Summary, "project_revision"), firstMapText(messageLoopMapValue(observation.Summary["project_binding"]), "project_revision"))
	if obsRevision != "" {
		// Only a project-bound observation may refresh the closure binding.
		obsUUID := firstMapText(messageLoopMapValue(observation.Summary["project_binding"]), "project_uuid")
		closureUUID := firstMapText(closure, "project_uuid")
		if obsUUID == "" || closureUUID == "" || strings.EqualFold(obsUUID, closureUUID) {
			closure["project_revision"] = obsRevision
			if contract := messageLoopMapValue(ctx["task_contract"]); len(contract) > 0 {
				contract["project_revision"] = obsRevision
			}
			if assessment := messageLoopMapValue(ctx["free_state_capacity_assessment"]); len(assessment) > 0 {
				assessment["project_revision"] = obsRevision
			}
			if binding := messageLoopMapValue(ctx["project_binding"]); len(binding) > 0 {
				binding["project_revision"] = obsRevision
			}
		}
	}
	// A target-level usable observation closes the candidate-confirmation
	// boundary.  Project-wide scans alone must not advance the host phase.
	if gateG6(state) || freeStateObservationClosesTarget(state, observation) {
		phaseText := firstNonEmpty(firstMapText(ctx, "free_state_phase"), firstMapText(closure, "phase"))
		if phase, ok := audioclosure.ParsePhase(phaseText); ok &&
			(phase == audioclosure.PhaseFS4DiagnosticRound || phase == audioclosure.PhaseFS5CandidateFrontier) {
			ctx["free_state_phase"] = string(audioclosure.PhaseFS6TargetConfirmed)
			closure["phase"] = string(audioclosure.PhaseFS6TargetConfirmed)
			loop := messageLoopMapValue(ctx["free_state_reasoning_loop"])
			if len(loop) > 0 {
				loop["current_phase"] = string(audioclosure.PhaseFS6TargetConfirmed)
				loop["phase"] = string(audioclosure.PhaseFS6TargetConfirmed)
				ctx["free_state_reasoning_loop"] = loop
			}
		}
	}
}

func freeStateObservationClosesTarget(state *runState, observation *RecentObservation) bool {
	if state == nil || observation == nil {
		return false
	}
	target := messageLoopMapValue(observation.Summary["target_ref"])
	kind, id := messageLoopCCBTargetIdentity(target)
	if kind != "track" || id == "" {
		return false
	}
	closure := messageLoopMapValue(state.input.Context["minimal_audio_closure"])
	frontier := messageLoopMapValue(closure["hypothesis_frontier"])
	selected := strings.TrimSpace(messageLoopText(frontier["candidate_id"]))
	if selected == "" {
		return false
	}
	for _, candidate := range messageLoopMapRows(frontier["candidates"]) {
		if !strings.EqualFold(firstMapText(candidate, "id"), selected) {
			continue
		}
		for _, trackID := range messageLoopStringList(candidate["track_ids"]) {
			if strings.TrimSpace(trackID) == id {
				return messageLoopFreeStateObservationStatusUsable(observation.Summary)
			}
		}
	}
	return false
}

// mergeFreeStateObservationLedger appends the current model round to every
// row so the model projection can fold older rounds into history instead of
// replaying them. round is the model turn (state.turnsUsed) that recorded the
// observation; the chat orchestration path uses its loop cycle.
func mergeFreeStateObservationLedger(ledger map[string]any, observation *RecentObservation, round int) {
	if len(ledger) == 0 || observation == nil {
		return
	}
	ledger["schema_version"] = freeStateObservationLedgerSchema
	if window := freeStateLedgerCount(ledger["window_round"], 0); round > window {
		ledger["window_round"] = round
	}
	if rejected := freeStateRejectedLedgerEntry(observation); len(rejected) > 0 {
		rejected["round"] = round
		rows := messageLoopMapRows(ledger["rejected_view_sets"])
		if !freeStateLedgerRowRecorded(rows, firstMapText(rejected, "fingerprint"), "fingerprint") {
			ledger["rejected_view_sets"] = appendBoundedFreeStateRows(rows, rejected)
			ledger["rejected_view_set_count"] = freeStateLedgerCount(ledger["rejected_view_set_count"], len(rows)) + 1
		}
	}
	if receipt := freeStateObservationLedgerReceipt(observation); len(receipt) > 0 {
		receipt["round"] = round
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
	recorded := 0
	for _, viewID := range messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"])) {
		view := messageLoopMapValue(messageLoopMapValue(observation.Summary["views"])[viewID])
		viewStatus := firstNonEmpty(firstMapText(view, "status"), status)
		if !strings.EqualFold(viewStatus, "ready") && !strings.EqualFold(viewStatus, "partial") {
			continue
		}
		available[viewID] = compactSelectedKeys(map[string]any{
			"view_id":          viewID,
			"status":           viewStatus,
			"observation_id":   firstMapText(observation.Summary, "observation_id"),
			"tool_call_id":     observation.ToolCallID,
			"project_revision": firstNonEmpty(firstMapText(observation.Summary, "project_revision"), firstMapText(messageLoopMapValue(observation.Summary["project_binding"]), "project_revision")),
			"freshness":        observation.Summary["freshness"],
			"limitations":      firstNonNilValue(view["limitations"], observation.Summary["limitations"]),
			"evidence_refs":    observation.Summary["evidence_refs"],
			"audit_ref":        freeStateObservationAuditRef(observation.Summary),
			"target_ref":       compactFreeStateObservationTarget(observation.Summary),
			"round":            round,
			"conclusion":       contextruntime.ProjectCCBViewConclusion(observation.Summary, viewID, messageLoopCompactOptions()),
		}, []string{"view_id", "status", "observation_id", "tool_call_id", "project_revision", "freshness", "limitations", "evidence_refs", "audit_ref", "target_ref", "round", "conclusion"})
		if key := freeStateObservationLedgerViewKey(viewID, observation.Summary); key != viewID {
			available[key] = available[viewID]
			delete(available, viewID)
		}
		recorded++
	}
	if len(available) > 0 {
		ledger["available_views"] = available
		ledger["view_observation_count"] = freeStateLedgerCount(ledger["view_observation_count"], 0) + recorded
	}
}

func freeStateRejectedLedgerEntry(observation *RecentObservation) map[string]any {
	if observation == nil || !strings.EqualFold(firstMapText(observation.Summary, "status", "bundle_status"), "rejected") {
		return nil
	}
	requested := messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"]))
	target := compactFreeStateObservationTarget(observation.Summary)
	fingerprint := messageLoopFreeStateRequestFingerprint(requested, target)
	if fingerprint == "" {
		return nil
	}
	audit := messageLoopMapValue(observation.Summary["audit_receipt"])
	return compactSelectedKeys(map[string]any{
		"fingerprint":           fingerprint,
		"requested_views":       requested,
		"status":                "rejected",
		"reasons":               firstNonNilValue(observation.Summary["omission_reasons"], audit["rejection_reasons"]),
		"receipt_id":            firstMapText(audit, "receipt_id"),
		"tool_call_id":          observation.ToolCallID,
		"request_id":            firstMapText(observation.Summary, "request_id"),
		"observation_id":        firstMapText(observation.Summary, "observation_id"),
		"retry_policy":          "do_not_retry",
		"rejection_scope":       firstNonEmpty(firstMapText(observation.Summary, "rejection_scope"), firstMapText(audit, "rejection_scope")),
		"blocking_view_ids":     firstNonNilValue(observation.Summary["blocking_view_ids"], audit["blocking_view_ids"]),
		"non_blocking_view_ids": firstNonNilValue(observation.Summary["non_blocking_view_ids"], audit["non_blocking_view_ids"]),
		"target_ref":            target,
	}, []string{"fingerprint", "requested_views", "status", "reasons", "receipt_id", "tool_call_id", "request_id", "observation_id", "retry_policy", "rejection_scope", "blocking_view_ids", "non_blocking_view_ids", "target_ref"})
}

func freeStateObservationLedgerReceipt(observation *RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	audit := messageLoopMapValue(observation.Summary["audit_receipt"])
	return compactSelectedKeys(map[string]any{
		"receipt_id":       firstMapText(audit, "receipt_id"),
		"receipt_schema":   firstMapText(audit, "schema_version"),
		"tool_call_id":     observation.ToolCallID,
		"observation_id":   firstMapText(observation.Summary, "observation_id"),
		"request_id":       firstMapText(observation.Summary, "request_id"),
		"status":           firstMapText(observation.Summary, "status", "bundle_status"),
		"requested_views":  messageLoopNormalizedViewIDs(messageLoopStringList(observation.Summary["requested_views"])),
		"project_revision": firstNonEmpty(firstMapText(observation.Summary, "project_revision"), firstMapText(messageLoopMapValue(observation.Summary["project_binding"]), "project_revision")),
		"freshness":        observation.Summary["freshness"],
		"limitations":      observation.Summary["limitations"],
		"evidence_refs":    observation.Summary["evidence_refs"],
		"target_ref":       compactFreeStateObservationTarget(observation.Summary),
	}, []string{"receipt_id", "receipt_schema", "tool_call_id", "observation_id", "request_id", "status", "requested_views", "project_revision", "freshness", "limitations", "evidence_refs", "target_ref"})
}

func compactFreeStateObservationTarget(summary map[string]any) map[string]any {
	target := messageLoopMapValue(summary["target_ref"])
	return compactSelectedKeys(target, []string{"kind", "id", "label", "track_id", "track_name"})
}

func freeStateObservationLedgerViewKey(viewID string, summary map[string]any) string {
	kind, id := messageLoopCCBTargetIdentity(compactFreeStateObservationTarget(summary))
	if kind == "track" && id != "" {
		return "track:" + id + "::" + viewID
	}
	return viewID
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
	if status != FreeStateSatisfied && status != FreeStateBlocked && status != FreeStateDiagnosticComplete && status != FreeStateNoCandidateFound && status != FreeStateCapabilityBlocked {
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

func messageLoopFreeStateRequestedCallsIssue(decisionViews []string, calls []planner.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	if len(calls) == 1 {
		return messageLoopFreeStateRequestedViewsIssue(decisionViews, calls[0].Args["view_ids"])
	}
	decision := map[string]bool{}
	for _, value := range decisionViews {
		viewID := strings.TrimSpace(value)
		if viewID == "" {
			return "free_state requested_view_ids must contain only non-empty identifiers"
		}
		decision[viewID] = true
	}
	if len(decision) == 0 {
		return "free_state requested_view_ids must contain at least one identifier"
	}
	covered := map[string]bool{}
	for _, call := range calls {
		callViews, issue := messageLoopFreeStateNormalizedCallViewSet(call.Args["view_ids"])
		if issue != "" {
			return "ccb.observation_request view_ids " + issue
		}
		for viewID := range callViews {
			if !decision[viewID] {
				return "each ccb.observation_request view_ids must be a subset of the model's free_state requested_view_ids"
			}
			covered[viewID] = true
		}
	}
	if len(covered) != len(decision) {
		return "the combined ccb.observation_request view_ids must exactly cover the model's free_state requested_view_ids"
	}
	for viewID := range decision {
		if !covered[viewID] {
			return "the combined ccb.observation_request view_ids must exactly cover the model's free_state requested_view_ids"
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
