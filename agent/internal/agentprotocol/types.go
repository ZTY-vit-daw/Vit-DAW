package agentprotocol

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const SchemaVersion = "vit_agent_protocol.v0"
const ImprovementProposalSchema = "improvement_proposal.v1"

const (
	ImprovementActionDomainTrackGain         = "track_gain"
	ImprovementActionDomainClipGain          = "clip_gain"
	ImprovementActionDomainPan               = "pan"
	ImprovementActionDomainEQ                = "eq"
	ImprovementActionDomainCompressor        = "compressor"
	ImprovementActionDomainLimiter           = "limiter"
	ImprovementActionDomainGateExpander      = "gate_expander"
	ImprovementActionDomainDeEsser           = "de_esser"
	ImprovementActionDomainTransientShaper   = "transient_shaper"
	ImprovementActionDomainMultibandDynamics = "multiband_dynamics"
	ImprovementActionDomainPlugin            = "plugin"
)

const (
	KindPendingCandidate      = "PendingCandidate"
	KindApprovalRequest       = "ApprovalRequest"
	KindUserInputRequest      = "UserInputRequest"
	KindObservationRequest    = "ObservationRequest"
	KindObservationResult     = "ObservationResult"
	KindAcousticPackageStatus = "AcousticPackageStatus"
	KindTerminalResult        = "TerminalResult"
)

const (
	PendingStatusProposed           = "proposed"
	PendingStatusWaitingUser        = "waiting_user"
	PendingStatusAccepted           = "accepted"
	PendingStatusRejected           = "rejected"
	PendingStatusRevisionRequested  = "revision_requested"
	PendingStatusAwaitingApproval   = "awaiting_approval"
	PendingStatusReadyToCommit      = "ready_to_commit"
	PendingStatusCommitting         = "committing"
	PendingStatusCommitted          = "committed"
	PendingStatusVerified           = "verified"
	PendingStatusVerificationFailed = "verification_failed"
	PendingStatusFailed             = "failed"
	PendingStatusBlocked            = "blocked"
	ApprovalStatusRequested         = "requested"
	ApprovalStatusWaitingUser       = "waiting_user"
	ApprovalStatusApproved          = "approved"
	ApprovalStatusApprovedWithScope = "approved_with_scope"
	ApprovalStatusDenied            = "denied"
	ApprovalStatusConsumed          = "consumed"
	ApprovalStatusClosed            = "closed"
	UserInputStatusWaiting          = "waiting"
	UserInputStatusWaitingUser      = "waiting_user"
	UserInputStatusAnswered         = "answered"
	UserInputStatusSkipped          = "skipped"
	UserInputStatusCancelled        = "cancelled"
	UserInputStatusConsumed         = "consumed"
)

type Source struct {
	ConversationID string         `json:"conversation_id,omitempty"`
	GoalID         string         `json:"goal_id,omitempty"`
	RunID          string         `json:"run_id,omitempty"`
	TurnID         string         `json:"turn_id,omitempty"`
	ToolCallID     string         `json:"tool_call_id,omitempty"`
	CallID         string         `json:"call_id,omitempty"`
	AgentActionID  string         `json:"agent_action_id,omitempty"`
	ArtifactIDs    []string       `json:"artifact_ids,omitempty"`
	LegacySchema   string         `json:"legacy_schema,omitempty"`
	LegacyKind     string         `json:"legacy_kind,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ImprovementProposal is the model-owned L3 handoff. It describes a bounded
// mixing hypothesis without asserting that the current mix is objectively
// wrong and without carrying mutation authority.
type ImprovementProposal struct {
	SchemaVersion     string         `json:"schema_version"`
	Target            map[string]any `json:"target"`
	EvidenceRefs      []string       `json:"evidence_refs"`
	ImprovementIntent string         `json:"improvement_intent"`
	Hypothesis        string         `json:"hypothesis"`
	ExpectedEffect    string         `json:"expected_effect"`
	ActionDomain      string         `json:"action_domain"`
	ActionKind        string         `json:"action_kind"`
	ProcessorType     string         `json:"processor_type,omitempty"`
	ParameterBounds   map[string]any `json:"parameter_bounds,omitempty"`
	VerificationPlan  map[string]any `json:"verification_plan,omitempty"`
	Confidence        float64        `json:"confidence"`
	Limitations       []string       `json:"limitations,omitempty"`
	NeedsResolution   []string       `json:"needs_resolution,omitempty"`
	RiskClass         string         `json:"risk_class,omitempty"`
}

func (p ImprovementProposal) Validate() error {
	if strings.TrimSpace(p.SchemaVersion) != ImprovementProposalSchema {
		return fmt.Errorf("schema_version must be %s", ImprovementProposalSchema)
	}
	if len(p.Target) == 0 {
		return fmt.Errorf("target is required")
	}
	if len(p.EvidenceRefs) == 0 {
		return fmt.Errorf("evidence_refs are required")
	}
	for name, value := range map[string]string{
		"improvement_intent": p.ImprovementIntent,
		"hypothesis":         p.Hypothesis,
		"expected_effect":    p.ExpectedEffect,
		"action_domain":      p.ActionDomain,
		"action_kind":        p.ActionKind,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if p.Confidence < 0 || p.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	switch strings.ToLower(strings.TrimSpace(p.ActionDomain)) {
	case ImprovementActionDomainTrackGain, ImprovementActionDomainClipGain,
		ImprovementActionDomainPan, ImprovementActionDomainEQ,
		ImprovementActionDomainCompressor, ImprovementActionDomainLimiter,
		ImprovementActionDomainGateExpander, ImprovementActionDomainDeEsser,
		ImprovementActionDomainTransientShaper, ImprovementActionDomainMultibandDynamics,
		ImprovementActionDomainPlugin:
	default:
		return fmt.Errorf("unsupported action_domain %q", p.ActionDomain)
	}
	return nil
}

func (p ImprovementProposal) ToPendingCandidate(conversationID, goalID, runID, createdAt string) PendingCandidate {
	confidence := strconv.FormatFloat(p.Confidence, 'f', -1, 64)
	action := map[string]any{
		"schema_version":     ImprovementProposalSchema,
		"target":             ToMap(p.Target),
		"evidence_refs":      append([]string(nil), p.EvidenceRefs...),
		"improvement_intent": strings.TrimSpace(p.ImprovementIntent),
		"hypothesis":         strings.TrimSpace(p.Hypothesis),
		"expected_effect":    strings.TrimSpace(p.ExpectedEffect),
		"action_domain":      strings.TrimSpace(p.ActionDomain),
		"action_kind":        strings.TrimSpace(p.ActionKind),
		"processor_type":     strings.TrimSpace(p.ProcessorType),
		"parameter_bounds":   ToMap(p.ParameterBounds),
		"verification_plan":  ToMap(p.VerificationPlan),
		"limitations":        append([]string(nil), p.Limitations...),
		"needs_resolution":   append([]string(nil), p.NeedsResolution...),
		"risk_class":         strings.TrimSpace(p.RiskClass),
	}
	removeEmptyProtocolMap(action)
	return PendingCandidate{
		ID:              NormalizeID("pending", "improvement", conversationID, goalID, p.ActionDomain, p.ActionKind),
		Kind:            KindPendingCandidate,
		Domain:          strings.TrimSpace(p.ActionDomain),
		CandidateType:   "improvement_proposal",
		TargetRef:       protocolTargetRef(p.Target),
		ActionKind:      strings.TrimSpace(p.ActionKind),
		ProcessorType:   strings.TrimSpace(p.ProcessorType),
		EvidenceRefs:    append([]string(nil), p.EvidenceRefs...),
		NeedsResolution: append([]string(nil), p.NeedsResolution...),
		Confidence:      confidence,
		Summary:         strings.TrimSpace(p.ImprovementIntent),
		Rationale:       strings.TrimSpace(p.Hypothesis),
		CandidateAction: action,
		Risk:            firstNonEmptyProtocolString(p.RiskClass, "reversible_bounded_experiment"),
		// The concrete permission domain is selected by the governed action
		// adapter; the L3 handoff must not invent one.
		RequiredPermissionDomains: nil,
		Status:                    PendingStatusWaitingUser,
		CreatedAt:                 strings.TrimSpace(createdAt),
		Source: Source{
			ConversationID: strings.TrimSpace(conversationID), GoalID: strings.TrimSpace(goalID),
			RunID: strings.TrimSpace(runID), LegacySchema: ImprovementProposalSchema,
			LegacyKind: "ImprovementProposal",
		},
	}
}

type PendingCandidate struct {
	ID                        string         `json:"id"`
	Kind                      string         `json:"kind"`
	Domain                    string         `json:"domain,omitempty"`
	CandidateType             string         `json:"candidate_type,omitempty"`
	TargetRef                 string         `json:"target_ref,omitempty"`
	ActionKind                string         `json:"action_kind,omitempty"`
	ProcessorType             string         `json:"processor_type,omitempty"`
	EvidenceRefs              []string       `json:"evidence_refs,omitempty"`
	NeedsResolution           []string       `json:"needs_resolution,omitempty"`
	Confidence                string         `json:"confidence,omitempty"`
	Summary                   string         `json:"summary,omitempty"`
	Rationale                 string         `json:"rationale,omitempty"`
	CandidateAction           map[string]any `json:"candidate_action,omitempty"`
	Risk                      string         `json:"risk,omitempty"`
	RequiredPermissionDomains []string       `json:"required_permission_domains,omitempty"`
	Status                    string         `json:"status"`
	CreatedAt                 string         `json:"created_at,omitempty"`
	Source                    Source         `json:"source,omitempty"`
}

type ApprovalRequest struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	ResourceDomain     string   `json:"resource_domain,omitempty"`
	Operation          string   `json:"operation,omitempty"`
	Scope              string   `json:"scope,omitempty"`
	TargetRef          string   `json:"target_ref,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	Status             string   `json:"status"`
	AvailableDecisions []string `json:"available_decisions,omitempty"`
	CreatedAt          string   `json:"created_at,omitempty"`
	Source             Source   `json:"source,omitempty"`
}

type UserInputRequest struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	QuestionType string   `json:"question_type,omitempty"`
	Prompt       string   `json:"prompt,omitempty"`
	Options      []string `json:"options,omitempty"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at,omitempty"`
	Source       Source   `json:"source,omitempty"`
}

type ObservationRequest struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Intent          string   `json:"intent,omitempty"`
	TargetRef       string   `json:"target_ref,omitempty"`
	RequiredSources []string `json:"required_sources,omitempty"`
	Status          string   `json:"status,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
	Source          Source   `json:"source,omitempty"`
}

type ObservationResult struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Intent        string         `json:"intent,omitempty"`
	ContextPackID string         `json:"context_pack_id,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	SourceRefs    []string       `json:"source_refs,omitempty"`
	Status        string         `json:"status,omitempty"`
	CreatedAt     string         `json:"created_at,omitempty"`
	Source        Source         `json:"source,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type AcousticPackageStatus struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	SchemaVersion  string         `json:"schema_version"`
	Status         string         `json:"status,omitempty"`
	ProjectID      string         `json:"project_id,omitempty"`
	TrackID        string         `json:"track_id,omitempty"`
	ClipID         string         `json:"clip_id,omitempty"`
	SourceHash     string         `json:"source_hash,omitempty"`
	SourceRevision string         `json:"source_revision,omitempty"`
	ArtifactPath   string         `json:"artifact_path,omitempty"`
	PackageLayers  map[string]any `json:"package_layers,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
	Source         Source         `json:"source,omitempty"`
}

type TerminalResult struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Domain    string         `json:"domain,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Status    string         `json:"status,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	Source    Source         `json:"source,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Event struct {
	SchemaVersion string `json:"schema_version"`
	EventType     string `json:"event_type"`
	StateKind     string `json:"state_kind,omitempty"`
	StateID       string `json:"state_id,omitempty"`
	Status        string `json:"status,omitempty"`
	CreatedAt     string `json:"created_at,omitempty"`
	Source        Source `json:"source,omitempty"`
	State         any    `json:"state,omitempty"`
}

func NewEvent(state any, source Source) Event {
	event := Event{SchemaVersion: SchemaVersion, Source: source, State: state}
	switch typed := state.(type) {
	case PendingCandidate:
		event.EventType = KindPendingCandidate
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *PendingCandidate:
		if typed != nil {
			event.EventType = KindPendingCandidate
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case ApprovalRequest:
		event.EventType = KindApprovalRequest
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *ApprovalRequest:
		if typed != nil {
			event.EventType = KindApprovalRequest
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case UserInputRequest:
		event.EventType = KindUserInputRequest
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *UserInputRequest:
		if typed != nil {
			event.EventType = KindUserInputRequest
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case ObservationRequest:
		event.EventType = KindObservationRequest
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *ObservationRequest:
		if typed != nil {
			event.EventType = KindObservationRequest
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case ObservationResult:
		event.EventType = KindObservationResult
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *ObservationResult:
		if typed != nil {
			event.EventType = KindObservationResult
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case AcousticPackageStatus:
		event.EventType = KindAcousticPackageStatus
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *AcousticPackageStatus:
		if typed != nil {
			event.EventType = KindAcousticPackageStatus
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	case TerminalResult:
		event.EventType = KindTerminalResult
		event.StateKind = typed.Kind
		event.StateID = typed.ID
		event.Status = typed.Status
	case *TerminalResult:
		if typed != nil {
			event.EventType = KindTerminalResult
			event.StateKind = typed.Kind
			event.StateID = typed.ID
			event.Status = typed.Status
		}
	}
	if event.EventType == "" {
		event.EventType = "Unknown"
	}
	return event
}

func ToMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func cloneProtocolMap(value map[string]any) map[string]any {
	return ToMap(value)
}

func removeEmptyProtocolMap(values map[string]any) {
	for key, value := range values {
		switch typed := value.(type) {
		case nil:
			delete(values, key)
		case string:
			if strings.TrimSpace(typed) == "" {
				delete(values, key)
			}
		case []string:
			if len(typed) == 0 {
				delete(values, key)
			}
		case map[string]any:
			if len(typed) == 0 {
				delete(values, key)
			}
		}
	}
}

func protocolTargetRef(target map[string]any) string {
	if len(target) == 0 {
		return ""
	}
	kind := FirstNonEmpty(fmt.Sprint(target["kind"]), fmt.Sprint(target["target_kind"]))
	id := FirstNonEmpty(fmt.Sprint(target["id"]), fmt.Sprint(target["target_id"]), fmt.Sprint(target["track_id"]), fmt.Sprint(target["clip_id"]), fmt.Sprint(target["bus_id"]))
	if kind == "" || id == "" || kind == "<nil>" || id == "<nil>" {
		return ""
	}
	return strings.TrimSpace(kind) + ":" + strings.TrimSpace(id)
}

func firstNonEmptyProtocolString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func NormalizeID(parts ...string) string {
	joined := strings.Join(parts, "_")
	joined = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, joined)
	joined = strings.Trim(joined, "._-")
	for strings.Contains(joined, "__") {
		joined = strings.ReplaceAll(joined, "__", "_")
	}
	if joined == "" {
		return "state"
	}
	return joined
}

func LegacyPendingStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending_confirmation", "needs_confirmation", "waiting_confirmation", "waiting_for_user":
		return PendingStatusWaitingUser
	case "accepted", "rejected", "revision_requested", "awaiting_approval", "ready_to_commit", "committing", "committed", "verified", "verification_failed", "failed", "blocked":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return PendingStatusProposed
	}
}

func LegacyApprovalStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "needs_confirmation", "waiting_confirmation", "waiting_for_user":
		return ApprovalStatusWaitingUser
	case "approved", "approved_with_scope", "denied", "consumed", "expired", "closed":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ApprovalStatusRequested
	}
}

func LegacyUserInputStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "waiting", "waiting_for_user":
		return UserInputStatusWaitingUser
	case "answered", "skipped", "cancelled", "consumed":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return UserInputStatusWaiting
	}
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func StringValue(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}
