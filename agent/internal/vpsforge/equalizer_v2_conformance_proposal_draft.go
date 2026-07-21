package vpsforge

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	equalizerV2ProposalDraftSchema = "vit.vpsforge.equalizer_v2_conformance_proposal_draft.v1"
	equalizerV2ProposalEvidenceDir = "evidence/equalizer_v2_conformance_proposal_draft"
)

// EqualizerV2ConformanceProposalDraftRequest only creates a local, review-only
// document inside an existing staging workspace. It is not a submission,
// approval, badge freeze, Credential issuance, Catalog update, or SPAL action.
type EqualizerV2ConformanceProposalDraftRequest struct {
	Root string
	Now  time.Time
}

type EqualizerV2ConformanceProposalDraftResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
}

type equalizerV2ConformanceProposalDraft struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	ProposalID    string    `json:"proposal_id"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                           `json:"authority_boundary"`
	Target            equalizerV2ProposalTarget          `json:"target"`
	Evidence          []equalizerV2ProposalEvidence      `json:"evidence"`
	HumanWitness      equalizerV2ProposalHumanWitness    `json:"human_witness"`
	BlockingGates     []equalizerV2ProposalGate          `json:"blocking_gates"`
	RequestedReview   equalizerV2ProposalRequestedReview `json:"requested_review"`
}

type equalizerV2ProposalTarget struct {
	BadgeID          string   `json:"badge_id"`
	CategoryID       string   `json:"category_id,omitempty"`
	Task             string   `json:"task,omitempty"`
	RequiredFeatures []string `json:"required_features,omitempty"`
	CandidateTrust   string   `json:"candidate_trust"`
	CandidateStatus  string   `json:"candidate_status"`
	RoutingEligible  bool     `json:"routing_eligible"`
}

type equalizerV2ProposalEvidence struct {
	ID           string `json:"id"`
	Artifact     string `json:"artifact"`
	Trust        string `json:"trust"`
	Status       string `json:"status"`
	Availability string `json:"availability"`
	UseBoundary  string `json:"use_boundary"`
}

type equalizerV2ProposalHumanWitness struct {
	Trust                  string                    `json:"trust"`
	RecordCount            int                       `json:"record_count"`
	Rounds                 []proQ3ReviewWitnessRound `json:"rounds,omitempty"`
	AllRecordsNotConformed bool                      `json:"all_records_not_conformed"`
	UseBoundary            string                    `json:"use_boundary"`
}

type equalizerV2ProposalGate struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type equalizerV2ProposalRequestedReview struct {
	Request                  string   `json:"request"`
	ProposedStandardActions  []string `json:"proposed_standard_actions"`
	SubmissionAuthorization  string   `json:"submission_authorization"`
	BadgeFreezeAuthorized    bool     `json:"badge_freeze_authorized"`
	CredentialIssuable       bool     `json:"credential_issuable"`
	CatalogMutation          string   `json:"catalog_mutation"`
	SPALRouteMutation        string   `json:"spal_route_mutation"`
	VPSDraftMappingPromotion string   `json:"vps_draft_mapping_promotion"`
}

// WriteEqualizerV2ConformanceProposalDraft creates an explicit handoff
// document for a later human review. Crucially, it has no normative action
// mapping: a future reviewer must define and approve one separately instead
// of treating parameter names, automated evidence, or this draft as authority.
func WriteEqualizerV2ConformanceProposalDraft(request EqualizerV2ConformanceProposalDraftRequest) (EqualizerV2ConformanceProposalDraftResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return EqualizerV2ConformanceProposalDraftResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return EqualizerV2ConformanceProposalDraftResult{}, fmt.Errorf("inspect equalizer.v2 proposal workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return EqualizerV2ConformanceProposalDraftResult{}, fmt.Errorf("equalizer.v2 proposal draft only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	var review proQ3EqualizerReviewSummary
	if err := readJSON(filepath.Join(root, proQ3EqualizerReviewSummaryFile), &review); err != nil {
		return EqualizerV2ConformanceProposalDraftResult{}, fmt.Errorf("read required Pro-Q 3 review summary: %w", err)
	}
	if review.Trust != "agent-inferred" || review.Status != "evidence_summary_not_conformed" {
		return EqualizerV2ConformanceProposalDraftResult{}, fmt.Errorf("Pro-Q 3 review summary must remain agent-inferred and not-conformed")
	}
	if !strings.EqualFold(strings.TrimSpace(review.CandidateBadge.BadgeID), "equalizer.v2") {
		return EqualizerV2ConformanceProposalDraftResult{}, fmt.Errorf("review summary does not name equalizer.v2 as its candidate badge")
	}

	now := nowOrCurrent(request.Now)
	proposalID := stableID("equalizer_v2_proposal_draft", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(equalizerV2ProposalEvidenceDir, proposalID+".json"))
	draft := equalizerV2ConformanceProposalDraft{
		SchemaVersion: equalizerV2ProposalDraftSchema,
		Trust:         "agent-inferred",
		Status:        "draft_not_submitted_not_conformed",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		ProposalID:    proposalID,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"This is a staging-only review draft, not an equalizer.v2 badge definition or approval.",
			"It does not create a routeable task badge, a feature matrix, a Credential, a Catalog entry, a SPAL route, or a formal VPS Library artifact.",
			"Observed host facts and agent-inferred summaries remain non-normative; user GUI testimony remains user-confirmed and not-conformed.",
			"No standard action is proposed automatically. A future review must deliberately define vendor-neutral action requirements and a repeatable conformance protocol.",
		},
		Target: equalizerV2ProposalTarget{
			BadgeID:          review.CandidateBadge.BadgeID,
			CategoryID:       review.CandidateBadge.CategoryID,
			Task:             review.CandidateBadge.Task,
			RequiredFeatures: append([]string(nil), review.CandidateBadge.RequiredFeatures...),
			CandidateTrust:   review.CandidateBadge.Trust,
			CandidateStatus:  review.CandidateBadge.Status,
			RoutingEligible:  false,
		},
		Evidence:      equalizerV2ProposalEvidenceFromReview(review),
		HumanWitness:  equalizerV2ProposalWitnessFromReview(review.HumanWitness),
		BlockingGates: equalizerV2ProposalGates(review),
		RequestedReview: equalizerV2ProposalRequestedReview{
			Request:                  "Review this non-submitted evidence packet and decide whether a separate equalizer.v2 conformance specification should be drafted. No approval or routing action is requested by this document.",
			ProposedStandardActions:  []string{},
			SubmissionAuthorization:  "not_requested",
			BadgeFreezeAuthorized:    false,
			CredentialIssuable:       false,
			CatalogMutation:          "none",
			SPALRouteMutation:        "none",
			VPSDraftMappingPromotion: "none; existing mappings remain observed only",
		},
	}
	if err := persistEqualizerV2ProposalDraft(root, draft); err != nil {
		return EqualizerV2ConformanceProposalDraftResult{}, err
	}
	return EqualizerV2ConformanceProposalDraftResult{
		Workspace: root, Status: draft.Status, ResultsArtifact: equalizerV2ProposalDraftFile, RunArtifact: runArtifact,
	}, nil
}

func equalizerV2ProposalEvidenceFromReview(review proQ3EqualizerReviewSummary) []equalizerV2ProposalEvidence {
	evidence := make([]equalizerV2ProposalEvidence, 0, len(review.EvidenceSources))
	for _, source := range review.EvidenceSources {
		evidence = append(evidence, equalizerV2ProposalEvidence{
			ID: source.ID, Artifact: source.Artifact, Trust: source.Trust, Status: source.Status,
			Availability: source.Availability, UseBoundary: source.Boundary,
		})
	}
	return evidence
}

func equalizerV2ProposalWitnessFromReview(witness proQ3ReviewHumanWitness) equalizerV2ProposalHumanWitness {
	return equalizerV2ProposalHumanWitness{
		Trust: witness.Trust, RecordCount: witness.RecordCount,
		Rounds:                 append([]proQ3ReviewWitnessRound(nil), witness.Rounds...),
		AllRecordsNotConformed: witness.AllRecordsNotConformed,
		UseBoundary:            "The referenced raw GUI testimony is review evidence only. It cannot supply executable semantics, badge eligibility, or Credential authority by itself.",
	}
}

func equalizerV2ProposalGates(review proQ3EqualizerReviewSummary) []equalizerV2ProposalGate {
	gates := make([]equalizerV2ProposalGate, 0, len(review.ReviewGaps)+3)
	for _, gap := range review.ReviewGaps {
		gates = append(gates, equalizerV2ProposalGate{ID: gap.ID, Status: gap.Status, Reason: gap.Reason})
	}
	gates = append(gates,
		equalizerV2ProposalGate{
			ID:     "badge_contract_and_feature_matrix",
			Status: "unsupported/unknown",
			Reason: "The candidate badge is non-routeable. A vendor-neutral minimum contract and a separate feature matrix still require explicit review.",
		},
		equalizerV2ProposalGate{
			ID:     "independent_conformance_decision",
			Status: "unsupported/unknown",
			Reason: "Observed machine preparation and user-confirmed testimony have not undergone an independent conformance decision.",
		},
		equalizerV2ProposalGate{
			ID:     "separate_installation_authority",
			Status: "unsupported/unknown",
			Reason: "No Credential, Catalog registration, or SPAL routing authority has been requested or granted.",
		},
	)
	return gates
}

func persistEqualizerV2ProposalDraft(root string, draft equalizerV2ConformanceProposalDraft) error {
	if err := writeJSON(filepath.Join(root, equalizerV2ProposalDraftFile), draft); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(draft.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("equalizer.v2 proposal run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), draft); err != nil {
		return err
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": draft.SchemaVersion, "proposal_id": draft.ProposalID, "status": draft.Status,
		"badge_id": draft.Target.BadgeID, "routing_eligible": false, "submission_authorization": "not_requested",
		"badge_freeze_authorized": false, "credential_issuable": false, "catalog_mutation": "none", "spal_route_mutation": "none",
		"standard_action_count": len(draft.RequestedReview.ProposedStandardActions),
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "equalizer_v2_conformance_proposal_draft",
		Trust:      "agent-inferred",
		Source:     "vpsforge.equalizer_v2_proposal_draft",
		Summary:    "Prepared a local, non-submitted equalizer.v2 review draft from staged evidence. It does not define or freeze a badge, promote a VPS mapping, issue a Credential, update Catalog, or enable SPAL routing.",
		Artifact:   filepath.ToSlash(runArtifact),
		CapturedAt: draft.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["equalizer_v2_conformance_proposal_draft"] = equalizerV2ProposalDraftFile
		manifest.UpdatedAt = draft.CapturedAt
	})
}
