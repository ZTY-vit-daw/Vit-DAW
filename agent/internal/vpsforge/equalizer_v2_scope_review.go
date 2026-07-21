package vpsforge

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	equalizerV2ScopeReviewSchema      = "vit.vpsforge.equalizer_v2_scope_review.v1"
	equalizerV2ScopeReviewEvidenceDir = "evidence/equalizer_v2_scope_review"
	maxEqualizerV2ScopeStatementBytes = 32 * 1024
)

// EqualizerV2ScopeReviewRequest records an explicit user decision about the
// first review boundary. It cannot turn that boundary into an executable
// mapping, a conformance result, or a routeable task badge.
type EqualizerV2ScopeReviewRequest struct {
	Root          string
	UserStatement string
	Now           time.Time
}

type EqualizerV2ScopeReviewResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
}

type equalizerV2ScopeReview struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	ReviewID      string    `json:"review_id"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                  `json:"authority_boundary"`
	CandidateTarget   equalizerV2ScopeCandidate `json:"candidate_target"`
	UserStatement     string                    `json:"user_statement"`
	IncludedScope     []equalizerV2ScopeArea    `json:"included_scope"`
	ExcludedScope     []equalizerV2ScopeArea    `json:"excluded_scope"`
	VendorSpecial     []equalizerV2ScopeArea    `json:"vendor_special_candidates"`
	ConformanceGates  []equalizerV2ScopeGate    `json:"conformance_gates"`
	ExecutionBoundary equalizerV2ScopeExecution `json:"execution_boundary"`
}

type equalizerV2ScopeCandidate struct {
	BadgeID         string `json:"badge_id"`
	CategoryID      string `json:"category_id,omitempty"`
	Task            string `json:"task,omitempty"`
	CandidateTrust  string `json:"candidate_trust"`
	CandidateStatus string `json:"candidate_status"`
	RoutingEligible bool   `json:"routing_eligible"`
}

type equalizerV2ScopeArea struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Boundary string `json:"boundary"`
}

type equalizerV2ScopeGate struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type equalizerV2ScopeExecution struct {
	ExecutableActionMappings int    `json:"executable_action_mappings"`
	BadgeFreezeAuthorized    bool   `json:"badge_freeze_authorized"`
	CredentialIssuable       bool   `json:"credential_issuable"`
	CatalogMutation          string `json:"catalog_mutation"`
	SPALRouteMutation        string `json:"spal_route_mutation"`
	VPSDraftMappingPromotion string `json:"vps_draft_mapping_promotion"`
}

// RecordEqualizerV2ScopeReview preserves the user's confirmation verbatim,
// while retaining the distinction between a review boundary and an executable
// semantic contract. The output is deliberately staging-only.
func RecordEqualizerV2ScopeReview(request EqualizerV2ScopeReviewRequest) (EqualizerV2ScopeReviewResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return EqualizerV2ScopeReviewResult{}, err
	}
	statement := strings.TrimSpace(request.UserStatement)
	if statement == "" {
		return EqualizerV2ScopeReviewResult{}, fmt.Errorf("a non-empty user scope confirmation is required")
	}
	if len([]byte(statement)) > maxEqualizerV2ScopeStatementBytes {
		return EqualizerV2ScopeReviewResult{}, fmt.Errorf("user scope confirmation exceeds %d bytes", maxEqualizerV2ScopeStatementBytes)
	}
	status, err := Inspect(root)
	if err != nil {
		return EqualizerV2ScopeReviewResult{}, fmt.Errorf("inspect equalizer.v2 scope workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return EqualizerV2ScopeReviewResult{}, fmt.Errorf("equalizer.v2 scope review only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	candidate := readProQ3ReviewCandidate(root)
	if !strings.EqualFold(candidate.BadgeID, "equalizer.v2") || candidate.RoutingEligible {
		return EqualizerV2ScopeReviewResult{}, fmt.Errorf("scope review requires the non-routeable equalizer.v2 candidate")
	}
	now := nowOrCurrent(request.Now)
	reviewID := stableID("equalizer_v2_scope_review", status.Manifest.WorkspaceID, statement, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(equalizerV2ScopeReviewEvidenceDir, reviewID+".json"))
	review := equalizerV2ScopeReview{
		SchemaVersion: equalizerV2ScopeReviewSchema,
		Trust:         "user-confirmed",
		Status:        "scope_confirmed_not_conformed",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		ReviewID:      reviewID,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"User confirmation records only the intended first-version review boundary; it is not executable semantic authority.",
			"The scope does not define a routeable badge, an EQ action schema, a feature matrix, or vendor-independent parameter mappings.",
			"Excluded items are deferred for separate review, not declared absent from Pro-Q 3 or unavailable in future versions.",
			"No Credential, Catalog entry, SPAL route, canonical VPS Library artifact, or VPS Draft mapping is modified.",
		},
		CandidateTarget: equalizerV2ScopeCandidate{
			BadgeID: candidate.BadgeID, CategoryID: candidate.CategoryID, Task: candidate.Task,
			CandidateTrust: candidate.Trust, CandidateStatus: candidate.Status, RoutingEligible: false,
		},
		UserStatement: statement,
		IncludedScope: []equalizerV2ScopeArea{
			{
				ID:       "static_band_lifecycle",
				Status:   "in_scope_pending_conformance",
				Boundary: "Review static parametric-band creation/selection plus the distinction between enable, bypass, delete and state restoration. Exact parameter bindings remain non-executable until conformance.",
			},
			{
				ID:       "static_frequency_gain_q",
				Status:   "in_scope_pending_conformance",
				Boundary: "Review host-controlled static frequency, gain and Q behavior with fresh readback and rollback. User listening observations remain evidence, not runtime rules.",
			},
			{
				ID:       "common_filter_shape_selection",
				Status:   "in_scope_pending_conformance",
				Boundary: "Review static selection and state persistence of common filter shapes. Display labels and any vendor-specific shape list remain observed rather than normative.",
			},
		},
		ExcludedScope: []equalizerV2ScopeArea{
			{ID: "dynamic_eq", Status: "out_of_scope_for_v1_pending_separate_review", Boundary: "Dynamic, time-varying EQ behavior is deferred pending its own semantic, temporal and conformance review."},
			{ID: "automatic_eq_or_matching", Status: "out_of_scope_for_v1_pending_separate_review", Boundary: "Automatic analysis, matching or recommendation workflows are deferred pending separate observed workflow and authorization review."},
			{ID: "spectrum_analyzer", Status: "out_of_scope_for_v1_pending_separate_review", Boundary: "Spectrum-display or analysis features are deferred; a visual measurement surface is not a routeable audio-processing task by itself."},
			{ID: "linear_phase_and_other_phase_modes", Status: "out_of_scope_for_v1_pending_separate_review", Boundary: "Phase-processing modes are deferred pending latency, state, boundary and listening review."},
		},
		VendorSpecial: []equalizerV2ScopeArea{
			{
				ID:       "pro_q_3_stereo_placement",
				Status:   "vendor_special_candidate_not_dispatchable",
				Boundary: "The observed Pro-Q 3 Placement surface is retained as a vendor-special candidate. It is not included in the equalizer.v2 minimum and cannot be dispatched without its own schema and conformance.",
			},
		},
		ConformanceGates: []equalizerV2ScopeGate{
			{ID: "vendor_neutral_badge_contract", Status: "unsupported/unknown", Reason: "The confirmed boundary still needs a deliberately authored, vendor-neutral minimum contract and independent review."},
			{ID: "executable_action_mapping", Status: "unsupported/unknown", Reason: "No parameter mapping is promoted by this scope confirmation; mappings remain observed."},
			{ID: "functional_fxm", Status: "unsupported/unknown", Reason: "Only a default baseline is available; function-level FXM waits for confirmed mapping and test design."},
			{ID: "behavior_and_boundary_conformance", Status: "unsupported/unknown", Reason: "The reviewed scope still requires repeatable behavior, edge-case, state and rollback conformance decisions."},
			{ID: "separate_authorization", Status: "unsupported/unknown", Reason: "No authorization has been requested or granted for badge freeze, Credential, Catalog or SPAL changes."},
		},
		ExecutionBoundary: equalizerV2ScopeExecution{
			ExecutableActionMappings: 0, BadgeFreezeAuthorized: false, CredentialIssuable: false,
			CatalogMutation: "none", SPALRouteMutation: "none", VPSDraftMappingPromotion: "none; existing mappings remain observed only",
		},
	}
	if err := persistEqualizerV2ScopeReview(root, review); err != nil {
		return EqualizerV2ScopeReviewResult{}, err
	}
	return EqualizerV2ScopeReviewResult{
		Workspace: root, Status: review.Status, ResultsArtifact: equalizerV2ScopeReviewFile, RunArtifact: runArtifact,
	}, nil
}

func persistEqualizerV2ScopeReview(root string, review equalizerV2ScopeReview) error {
	if err := writeJSON(filepath.Join(root, equalizerV2ScopeReviewFile), review); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(review.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("equalizer.v2 scope review run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), review); err != nil {
		return err
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": review.SchemaVersion, "review_id": review.ReviewID, "status": review.Status,
		"badge_id": review.CandidateTarget.BadgeID, "routing_eligible": false,
		"included_scope_count": len(review.IncludedScope), "excluded_scope_count": len(review.ExcludedScope),
		"vendor_special_candidate_count": len(review.VendorSpecial), "executable_action_mappings": 0,
		"badge_freeze_authorized": false, "credential_issuable": false, "catalog_mutation": "none", "spal_route_mutation": "none",
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "equalizer_v2_scope_review",
		Trust:      "user-confirmed",
		Source:     "user_scope_confirmation",
		Summary:    "Archived the user's first-version equalizer.v2 scope confirmation. It remains a non-conformed review boundary and does not create executable mappings, badge eligibility, Credentials, Catalog changes, or SPAL routing.",
		Artifact:   filepath.ToSlash(runArtifact),
		CapturedAt: review.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["equalizer_v2_scope_review"] = equalizerV2ScopeReviewFile
		manifest.UpdatedAt = review.CapturedAt
	})
}
