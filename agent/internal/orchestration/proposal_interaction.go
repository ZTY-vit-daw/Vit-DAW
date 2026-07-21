package orchestration

import "strings"

const ProposalPresentationSchema = "vit.proposal_presentation.v1"
const ApprovalDecisionSchema = "vit.approval_decision.v1"

// ProposalPresentation is the bounded user-facing view of an executable
// Proposal. It is durable audit data, not a second execution source: the
// FrozenPlan ActionSet remains the only mutation input.
type ProposalPresentation struct {
	SchemaVersion    string                  `json:"schema_version"`
	ProposalID       string                  `json:"proposal_id"`
	ProposalRevision int64                   `json:"proposal_revision"`
	CapabilityID     string                  `json:"capability_id"`
	Title            string                  `json:"title"`
	Conclusion       string                  `json:"conclusion,omitempty"`
	AnalysisSummary  []string                `json:"analysis_summary,omitempty"`
	Recommendation   string                  `json:"recommendation,omitempty"`
	AnalyzedTracks   int                     `json:"analyzed_tracks,omitempty"`
	ActionCount      int                     `json:"action_count"`
	Risk             string                  `json:"risk,omitempty"`
	Reversible       bool                    `json:"reversible"`
	Readiness        []ProposalMetric        `json:"readiness,omitempty"`
	ChangeGroups     []ProposalChangeGroup   `json:"change_groups,omitempty"`
	Actions          []ProposalActionPreview `json:"actions,omitempty"`
	Limitations      []string                `json:"limitations,omitempty"`
	EvidenceRefs     []string                `json:"evidence_refs,omitempty"`
	ApprovalPrompt   string                  `json:"approval_prompt,omitempty"`
}

type ProposalMetric struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Status string `json:"status,omitempty"`
}

type ProposalChangeGroup struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Role       string  `json:"role,omitempty"`
	Function   string  `json:"function,omitempty"`
	TrackCount int     `json:"track_count"`
	MoveCount  int     `json:"move_count"`
	MinValue   float64 `json:"min_value"`
	MaxValue   float64 `json:"max_value"`
	Unit       string  `json:"unit,omitempty"`
}

type ProposalActionPreview struct {
	ActionID  string  `json:"action_id"`
	TrackID   string  `json:"track_id"`
	TrackName string  `json:"track_name,omitempty"`
	Role      string  `json:"role,omitempty"`
	Function  string  `json:"function,omitempty"`
	Operation string  `json:"operation"`
	Before    float64 `json:"before"`
	Target    float64 `json:"target"`
	Delta     float64 `json:"delta"`
	Unit      string  `json:"unit,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}

type ApprovalDecisionKind string

const (
	ApprovalApprove    ApprovalDecisionKind = "approve"
	ApprovalReject     ApprovalDecisionKind = "reject"
	ApprovalQuestion   ApprovalDecisionKind = "question"
	ApprovalRevise     ApprovalDecisionKind = "revise"
	ApprovalNarrow     ApprovalDecisionKind = "narrow_scope"
	ApprovalAmbiguous  ApprovalDecisionKind = "ambiguous"
	ApprovalNoDecision ApprovalDecisionKind = "none"
)

// ApprovalDecision is the typed interpretation of one conversational turn.
// Only Kind=approve may be converted into Authorization.
type ApprovalDecision struct {
	SchemaVersion    string               `json:"schema_version"`
	Kind             ApprovalDecisionKind `json:"kind"`
	ProposalID       string               `json:"proposal_id,omitempty"`
	ProposalRevision int64                `json:"proposal_revision,omitempty"`
	ActionSetHash    string               `json:"action_set_hash,omitempty"`
	ProjectCutHash   string               `json:"project_cut_hash,omitempty"`
	ApprovedScope    []string             `json:"approved_scope,omitempty"`
	SourceTurnID     string               `json:"source_turn_id,omitempty"`
	UserText         string               `json:"user_text,omitempty"`
	Confidence       string               `json:"confidence,omitempty"`
	Reason           string               `json:"reason,omitempty"`
	Adjustments      []ProposalAdjustment `json:"adjustments,omitempty"`
	Unresolved       []string             `json:"unresolved,omitempty"`
}

type ProposalAdjustment struct {
	Kind          string   `json:"kind"`
	TargetQuery   string   `json:"target_query,omitempty"`
	TargetRefs    []string `json:"target_refs,omitempty"`
	OriginalValue float64  `json:"original_value,omitempty"`
	Value         float64  `json:"value,omitempty"`
	Unit          string   `json:"unit,omitempty"`
	Candidate     int      `json:"candidate,omitempty"`
}

func (d ApprovalDecision) ExactApprovalFor(proposal Proposal) bool {
	return d.Kind == ApprovalApprove &&
		strings.TrimSpace(d.ProposalID) == strings.TrimSpace(proposal.ID) &&
		d.ProposalRevision == proposal.Revision &&
		strings.TrimSpace(d.ActionSetHash) == strings.TrimSpace(proposal.ActionSetHash) &&
		strings.TrimSpace(d.ProjectCutHash) == strings.TrimSpace(proposal.ProjectCutHash) &&
		strings.TrimSpace(d.SourceTurnID) != ""
}

func cloneProposalPresentation(in *ProposalPresentation) *ProposalPresentation {
	if in == nil {
		return nil
	}
	out := *in
	out.AnalysisSummary = append([]string(nil), in.AnalysisSummary...)
	out.Readiness = append([]ProposalMetric(nil), in.Readiness...)
	out.ChangeGroups = append([]ProposalChangeGroup(nil), in.ChangeGroups...)
	out.Actions = append([]ProposalActionPreview(nil), in.Actions...)
	out.Limitations = append([]string(nil), in.Limitations...)
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	return &out
}

func cloneApprovalDecision(in *ApprovalDecision) *ApprovalDecision {
	if in == nil {
		return nil
	}
	out := *in
	out.ApprovedScope = append([]string(nil), in.ApprovedScope...)
	out.Unresolved = append([]string(nil), in.Unresolved...)
	out.Adjustments = append([]ProposalAdjustment(nil), in.Adjustments...)
	for index := range out.Adjustments {
		out.Adjustments[index].TargetRefs = append([]string(nil), in.Adjustments[index].TargetRefs...)
	}
	return &out
}
