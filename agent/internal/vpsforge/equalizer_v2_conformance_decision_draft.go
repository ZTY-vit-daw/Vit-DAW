package vpsforge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	equalizerV2DecisionDraftSchema = "vit.vpsforge.equalizer_v2_conformance_decision_draft.v1"
	equalizerV2DecisionEvidenceDir = "evidence/equalizer_v2_conformance_decision_draft"
)

// EqualizerV2ConformanceDecisionDraftRequest creates a read-only decision
// handoff from staging evidence. It cannot approve, freeze, route or install a
// badge, regardless of the evidence currently available.
type EqualizerV2ConformanceDecisionDraftRequest struct {
	Root string
	Now  time.Time
}

type EqualizerV2ConformanceDecisionDraftResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
}

type equalizerV2ConformanceDecisionDraft struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	DecisionID    string    `json:"decision_id"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                       `json:"authority_boundary"`
	Target            equalizerV2DecisionTarget      `json:"target"`
	EvidenceFindings  []equalizerV2DecisionFinding   `json:"evidence_findings"`
	BlockingFindings  []equalizerV2DecisionFinding   `json:"blocking_findings"`
	LifecycleHistory  equalizerV2LifecycleRunHistory `json:"lifecycle_run_history"`
	Decision          equalizerV2DecisionOutcome     `json:"decision"`
}

type equalizerV2DecisionTarget struct {
	BadgeID         string `json:"badge_id"`
	CandidateStatus string `json:"candidate_status"`
	RoutingEligible bool   `json:"routing_eligible"`
}

type equalizerV2DecisionFinding struct {
	ID        string   `json:"id"`
	Trust     string   `json:"trust"`
	Status    string   `json:"status"`
	Artifacts []string `json:"artifacts"`
	Boundary  string   `json:"boundary"`
}

type equalizerV2LifecycleRunHistory struct {
	ArtifactDirectory string   `json:"artifact_directory"`
	TotalRuns         int      `json:"total_runs"`
	CompletedRuns     int      `json:"completed_runs"`
	FailedRuns        int      `json:"failed_runs"`
	OtherRuns         int      `json:"other_runs"`
	RunArtifacts      []string `json:"run_artifacts"`
}

type equalizerV2DecisionOutcome struct {
	ConformanceStatus        string `json:"conformance_status"`
	Recommendation           string `json:"recommendation"`
	ExecutableActionMappings int    `json:"executable_action_mappings"`
	BadgeFreezeAuthorized    bool   `json:"badge_freeze_authorized"`
	CredentialIssuable       bool   `json:"credential_issuable"`
	CatalogMutation          string `json:"catalog_mutation"`
	SPALRouteMutation        string `json:"spal_route_mutation"`
	VPSDraftMappingPromotion string `json:"vps_draft_mapping_promotion"`
}

// WriteEqualizerV2ConformanceDecisionDraft derives the current negative
// conformance outcome explicitly: useful staging evidence exists, but the
// badge remains non-conformed until a reviewed generic contract, action
// mappings and functional FXM have been completed.
func WriteEqualizerV2ConformanceDecisionDraft(request EqualizerV2ConformanceDecisionDraftRequest) (EqualizerV2ConformanceDecisionDraftResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("inspect equalizer.v2 decision workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("equalizer.v2 decision draft only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	var scope equalizerV2ScopeReview
	if err := readJSON(filepath.Join(root, equalizerV2ScopeReviewFile), &scope); err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("read equalizer.v2 scope review: %w", err)
	}
	var matrix equalizerV2StaticEQReviewMatrix
	if err := readJSON(filepath.Join(root, equalizerV2StaticEQMatrixFile), &matrix); err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("read equalizer.v2 static EQ review matrix: %w", err)
	}
	var review proQ3EqualizerReviewSummary
	if err := readJSON(filepath.Join(root, proQ3EqualizerReviewSummaryFile), &review); err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("read Pro-Q 3 review summary: %w", err)
	}
	if !strings.EqualFold(review.CandidateBadge.BadgeID, "equalizer.v2") || review.CandidateBadge.RoutingEligible {
		return EqualizerV2ConformanceDecisionDraftResult{}, fmt.Errorf("review summary must retain a non-routeable equalizer.v2 candidate")
	}
	lifecycleSource := proQ3ReviewSource(root, "static_lifecycle_probe", proQ3StaticLifecycleResultsFile, "observed lifecycle probe only")
	history := equalizerV2ReadLifecycleRunHistory(root)
	now := nowOrCurrent(request.Now)
	decisionID := stableID("equalizer_v2_decision_draft", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(equalizerV2DecisionEvidenceDir, decisionID+".json"))
	draft := equalizerV2ConformanceDecisionDraft{
		SchemaVersion: equalizerV2DecisionDraftSchema,
		Trust:         "agent-inferred",
		Status:        "decision_draft_not_conformed",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		DecisionID:    decisionID,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"This is a staging decision draft, not a conformance approval or a badge freeze.",
			"Observed host behavior, user-confirmed scope and agent-inferred coverage remain separate and non-executable evidence classes.",
			"The draft cannot issue a Credential, update Catalog, enable SPAL routing or promote a VPS Draft mapping.",
		},
		Target: equalizerV2DecisionTarget{
			BadgeID: review.CandidateBadge.BadgeID, CandidateStatus: review.CandidateBadge.Status, RoutingEligible: false,
		},
		EvidenceFindings: []equalizerV2DecisionFinding{
			{
				ID: "user_confirmed_first_version_scope", Trust: scope.Trust, Status: scope.Status,
				Artifacts: []string{equalizerV2ScopeReviewFile},
				Boundary:  "The v1 static-EQ boundary is confirmed, but it is not a generic action contract.",
			},
			{
				ID: "static_eq_evidence_coverage", Trust: matrix.Trust, Status: matrix.Status,
				Artifacts: []string{equalizerV2StaticEQMatrixFile, humanWitnessFile, proQ3CoreEQResultsFile},
				Boundary:  "The matrix identifies assembled evidence for static frequency/gain/Q and shape review; it has zero executable mappings.",
			},
			{
				ID: "observed_static_lifecycle_probe", Trust: lifecycleSource.Trust, Status: lifecycleSource.Status,
				Artifacts: []string{proQ3StaticLifecycleResultsFile},
				Boundary:  "Bounded Used/Enabled candidates completed write/readback, state-roundtrip, render and rollback checks; test labels are not semantic mappings.",
			},
			{
				ID: "worker_resource_isolation", Trust: "observed", Status: "observed_completed",
				Artifacts: []string{proQ3ResourceIsolationResultsFile},
				Boundary:  "Separate-worker isolation and reload restoration were observed; this does not establish a semantic EQ capability.",
			},
		},
		BlockingFindings: []equalizerV2DecisionFinding{
			{
				ID: "vendor_neutral_badge_contract", Trust: "unsupported/unknown", Status: "blocking",
				Artifacts: []string{equalizerV2ScopeReviewFile},
				Boundary:  "The user-confirmed scope still needs an independently reviewed vendor-neutral minimum contract.",
			},
			{
				ID: "executable_action_mapping", Trust: "unsupported/unknown", Status: "blocking",
				Artifacts: []string{equalizerV2StaticEQMatrixFile, draftFile},
				Boundary:  "No observed Pro-Q 3 parameter is promoted to an executable action mapping.",
			},
			{
				ID: "functional_fxm", Trust: "unsupported/unknown", Status: "blocking",
				Artifacts: []string{fxmBaselineFile, equalizerV2StaticEQMatrixFile},
				Boundary:  "Only default-state FXM exists. Function-level FXM must follow a reviewed action contract and test design.",
			},
			{
				ID: "lifecycle_run_history_review", Trust: "observed", Status: equalizerV2LifecycleHistoryStatus(history),
				Artifacts: history.RunArtifacts,
				Boundary:  "All archived lifecycle runs, including failed exploratory attempts, remain part of the audit trail and require review before any conformance decision.",
			},
			{
				ID: "independent_conformance_authority", Trust: "unsupported/unknown", Status: "blocking",
				Artifacts: []string{equalizerV2ProposalDraftFile},
				Boundary:  "No independent approval, badge freeze or installation authority has been granted.",
			},
		},
		LifecycleHistory: history,
		Decision: equalizerV2DecisionOutcome{
			ConformanceStatus:        "not_conformed",
			Recommendation:           "Defer equalizer.v2 conformance. Continue only with a separately reviewed vendor-neutral action contract, then action-scoped FXM and boundary tests.",
			ExecutableActionMappings: 0,
			BadgeFreezeAuthorized:    false,
			CredentialIssuable:       false,
			CatalogMutation:          "none",
			SPALRouteMutation:        "none",
			VPSDraftMappingPromotion: "none; observed mappings remain observed only",
		},
	}
	if err := persistEqualizerV2DecisionDraft(root, draft); err != nil {
		return EqualizerV2ConformanceDecisionDraftResult{}, err
	}
	return EqualizerV2ConformanceDecisionDraftResult{
		Workspace: root, Status: draft.Status, ResultsArtifact: equalizerV2DecisionDraftFile, RunArtifact: runArtifact,
	}, nil
}

func equalizerV2ReadLifecycleRunHistory(root string) equalizerV2LifecycleRunHistory {
	history := equalizerV2LifecycleRunHistory{ArtifactDirectory: proQ3StaticLifecycleEvidenceDir, RunArtifacts: []string{}}
	directory := filepath.Join(root, proQ3StaticLifecycleEvidenceDir)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return history
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		artifact := filepath.ToSlash(filepath.Join(proQ3StaticLifecycleEvidenceDir, entry.Name()))
		history.RunArtifacts = append(history.RunArtifacts, artifact)
		var header proQ3ReviewArtifactHeader
		if readJSON(filepath.Join(directory, entry.Name()), &header) != nil {
			history.OtherRuns++
			continue
		}
		history.TotalRuns++
		switch header.Status {
		case "observed_completed":
			history.CompletedRuns++
		case "observed_failed":
			history.FailedRuns++
		default:
			history.OtherRuns++
		}
	}
	sort.Strings(history.RunArtifacts)
	return history
}

func equalizerV2LifecycleHistoryStatus(history equalizerV2LifecycleRunHistory) string {
	if history.TotalRuns == 0 {
		return "missing"
	}
	if history.FailedRuns > 0 {
		return "review_required_with_archived_failures"
	}
	return "review_required"
}

func persistEqualizerV2DecisionDraft(root string, draft equalizerV2ConformanceDecisionDraft) error {
	if err := writeJSON(filepath.Join(root, equalizerV2DecisionDraftFile), draft); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(draft.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("equalizer.v2 decision run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), draft); err != nil {
		return err
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": draft.SchemaVersion, "decision_id": draft.DecisionID, "status": draft.Status,
		"badge_id": draft.Target.BadgeID, "conformance_status": draft.Decision.ConformanceStatus,
		"executable_action_mappings": 0, "badge_freeze_authorized": false, "credential_issuable": false,
		"catalog_mutation": "none", "spal_route_mutation": "none",
		"lifecycle_total_runs": draft.LifecycleHistory.TotalRuns, "lifecycle_failed_runs": draft.LifecycleHistory.FailedRuns,
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "equalizer_v2_conformance_decision_draft",
		Trust:      "agent-inferred",
		Source:     "vpsforge.equalizer_v2_decision_draft",
		Summary:    "Prepared a non-conformed equalizer.v2 decision draft from staging evidence. It defers badge approval and does not promote mappings, issue Credentials, update Catalog or enable SPAL routing.",
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
		manifest.Artifacts["equalizer_v2_conformance_decision_draft"] = equalizerV2DecisionDraftFile
		manifest.UpdatedAt = draft.CapturedAt
	})
}
