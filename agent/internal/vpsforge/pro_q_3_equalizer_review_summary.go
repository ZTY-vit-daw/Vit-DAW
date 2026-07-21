package vpsforge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const proQ3EqualizerReviewSummarySchema = "vit.vpsforge.pro_q_3_equalizer_review_summary.v1"

// ProQ3EqualizerReviewSummaryRequest asks for a read-only review handoff from
// evidence already captured inside one isolated Pro-Q 3 preflight workspace.
// It must not be used as a Credential, Catalog or SPAL promotion path.
type ProQ3EqualizerReviewSummaryRequest struct {
	Root string
	Now  time.Time
}

type ProQ3EqualizerReviewSummaryResult struct {
	Workspace string `json:"workspace"`
	Status    string `json:"status"`
	Artifact  string `json:"artifact"`
}

type proQ3EqualizerReviewSummary struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`

	AuthorityBoundary []string                      `json:"authority_boundary"`
	CandidateBadge    proQ3ReviewCandidate          `json:"candidate_badge"`
	EvidenceSources   []proQ3ReviewEvidenceSource   `json:"evidence_sources"`
	HumanWitness      proQ3ReviewHumanWitness       `json:"human_witness"`
	MachineReadiness  []proQ3ReviewMachineReadiness `json:"machine_readiness"`
	ReviewGaps        []proQ3ReviewGap              `json:"review_gaps"`
	Decision          proQ3ReviewDecision           `json:"decision"`
}

type proQ3ReviewCandidate struct {
	BadgeID          string   `json:"badge_id"`
	CategoryID       string   `json:"category_id,omitempty"`
	Task             string   `json:"task,omitempty"`
	RequiredFeatures []string `json:"required_features,omitempty"`
	Trust            string   `json:"trust"`
	Status           string   `json:"status"`
	RoutingEligible  bool     `json:"routing_eligible"`
	SourceArtifact   string   `json:"source_artifact"`
}

type proQ3ReviewEvidenceSource struct {
	ID           string    `json:"id"`
	Artifact     string    `json:"artifact"`
	Trust        string    `json:"trust"`
	Status       string    `json:"status"`
	CapturedAt   time.Time `json:"captured_at,omitempty"`
	Availability string    `json:"availability"`
	Boundary     string    `json:"boundary"`
}

type proQ3ReviewHumanWitness struct {
	Artifact               string                    `json:"artifact"`
	Trust                  string                    `json:"trust"`
	RecordCount            int                       `json:"record_count"`
	Rounds                 []proQ3ReviewWitnessRound `json:"rounds,omitempty"`
	AllRecordsNotConformed bool                      `json:"all_records_not_conformed"`
	InterpretationBoundary string                    `json:"interpretation_boundary"`
}

type proQ3ReviewWitnessRound struct {
	RoundID string `json:"round_id"`
	Status  string `json:"status"`
	Records int    `json:"record_count"`
}

type proQ3ReviewMachineReadiness struct {
	ID       string `json:"id"`
	Trust    string `json:"trust"`
	Status   string `json:"status"`
	Artifact string `json:"artifact"`
	Boundary string `json:"boundary"`
}

type proQ3ReviewGap struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type proQ3ReviewDecision struct {
	CredentialIssuable bool   `json:"credential_issuable"`
	CatalogMutation    string `json:"catalog_mutation"`
	SPALRouteMutation  string `json:"spal_route_mutation"`
	VPSDraftPromotion  string `json:"vps_draft_promotion"`
	NextReviewAction   string `json:"next_review_action"`
}

type proQ3ReviewArtifactHeader struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
}

// WriteProQ3EqualizerReviewSummary collates references and explicitly marks
// what remains outside automated authority. It never edits vps_draft.json,
// submits a badge Proposal, or changes an authorization-bearing artifact.
func WriteProQ3EqualizerReviewSummary(request ProQ3EqualizerReviewSummaryRequest) (ProQ3EqualizerReviewSummaryResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return ProQ3EqualizerReviewSummaryResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return ProQ3EqualizerReviewSummaryResult{}, fmt.Errorf("inspect Pro-Q 3 review workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return ProQ3EqualizerReviewSummaryResult{}, fmt.Errorf("review summary only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	now := nowOrCurrent(request.Now)
	candidate := readProQ3ReviewCandidate(root)
	sources := []proQ3ReviewEvidenceSource{
		proQ3ReviewSource(root, "plugin_identity", pluginIdentityFile, "observed identity and class/fingerprint facts only"),
		proQ3ReviewSource(root, "surface_snapshot", surfaceFile, "observed VST3 parameter surface only; labels are not semantic authority"),
		proQ3ReviewSource(root, "parameter_probe", parameterProbeFile, "observed write/readback/rollback result only"),
		proQ3ReviewSource(root, "state_roundtrip", stateRoundtripFile, "observed state serialization/reload result only"),
		proQ3ReviewSource(root, "fxm_default_baseline", fxmBaselineFile, "default-state machine baseline only; no functional FXM claim"),
		proQ3ReviewSource(root, "stereo_placement_probe", stereoPlacementResultsFile, "observed bounded render experiment only"),
		proQ3ReviewSource(root, "core_eq_probe", proQ3CoreEQResultsFile, "observed static response experiment only"),
		proQ3ReviewSource(root, "resource_isolation_probe", proQ3ResourceIsolationResultsFile, "observed direct-worker isolation/reload experiment only"),
		proQ3ReviewSource(root, "equalizer_v2_scope_review", equalizerV2ScopeReviewFile, "user-confirmed first-version review boundary only; it is not an executable action contract"),
		proQ3ReviewSource(root, "equalizer_v2_static_eq_review_matrix", equalizerV2StaticEQMatrixFile, "agent-inferred evidence coverage audit only; it is not a conformance decision"),
		proQ3ReviewSource(root, "static_lifecycle_probe", proQ3StaticLifecycleResultsFile, "observed bounded static lifecycle candidate probe only"),
	}
	humanWitness := readProQ3ReviewHumanWitness(root)
	machineReadiness := []proQ3ReviewMachineReadiness{
		proQ3ReviewMachineSource("parameter_write_readback_rollback", parameterProbeFile, sources),
		proQ3ReviewMachineSource("state_serialization_reload_restore", stateRoundtripFile, sources),
		proQ3ReviewMachineSource("default_fxm_baseline", fxmBaselineFile, sources),
		proQ3ReviewMachineSource("stereo_placement_behavior_probe", stereoPlacementResultsFile, sources),
		proQ3ReviewMachineSource("core_eq_behavior_probe", proQ3CoreEQResultsFile, sources),
		proQ3ReviewMachineSource("resource_isolation_and_reloaded_restore", proQ3ResourceIsolationResultsFile, sources),
		proQ3ReviewMachineSource("static_lifecycle_behavior_probe", proQ3StaticLifecycleResultsFile, sources),
	}
	summary := proQ3EqualizerReviewSummary{
		SchemaVersion: proQ3EqualizerReviewSummarySchema,
		Trust:         "agent-inferred",
		Status:        "evidence_summary_not_conformed",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		AuthorityBoundary: []string{
			"This is an agent-inferred review index over staging evidence, not an executable VPS mapping.",
			"Observed machine facts, display text, numeric renders and user-confirmed testimony remain distinct evidence classes.",
			"No automatic result, parameter name, plug-in name or summary inference grants equalizer.v2 or any other routeable badge.",
			"No Credential, Catalog entry, SPAL route or formal VPS Library artifact is created or changed by this summary.",
		},
		CandidateBadge:   candidate,
		EvidenceSources:  sources,
		HumanWitness:     humanWitness,
		MachineReadiness: machineReadiness,
		ReviewGaps:       proQ3ReviewGaps(sources),
		Decision: proQ3ReviewDecision{
			CredentialIssuable: false,
			CatalogMutation:    "none",
			SPALRouteMutation:  "none",
			VPSDraftPromotion:  "none; observed mappings remain observed and user testimony remains user-confirmed/not-conformed",
			NextReviewAction:   "Prepare, but do not submit automatically, an explicit equalizer.v2 conformance/freeze Proposal after the remaining semantic and behavior gates are deliberately reviewed.",
		},
	}
	if err := persistProQ3EqualizerReviewSummary(root, summary); err != nil {
		return ProQ3EqualizerReviewSummaryResult{}, err
	}
	return ProQ3EqualizerReviewSummaryResult{Workspace: root, Status: summary.Status, Artifact: proQ3EqualizerReviewSummaryFile}, nil
}

func proQ3ReviewGaps(sources []proQ3ReviewEvidenceSource) []proQ3ReviewGap {
	scopeStatus := "unsupported/unknown"
	scopeReason := "A candidate task badge and raw witness evidence do not yet establish a vendor-neutral, executable task scope."
	for _, source := range sources {
		if source.ID == "equalizer_v2_scope_review" && source.Availability == "present" && source.Trust == "user-confirmed" && source.Status == "scope_confirmed_not_conformed" {
			scopeStatus = "scope_confirmed_not_conformed"
			scopeReason = "The user confirmed a first-version review boundary, but it remains non-conformed and still needs a vendor-neutral contract plus executable-action review."
		}
	}
	return []proQ3ReviewGap{
		{ID: "equalizer_v2_semantic_scope", Status: scopeStatus, Reason: scopeReason},
		{ID: "standard_action_mapping", Status: "unsupported/unknown", Reason: "Observed IDs, labels and static measurements have not been promoted to reviewed action ownership or an executable VPS mapping."},
		{ID: "mode_and_vendor_special_capabilities", Status: "unsupported/unknown", Reason: "Placement, dynamic behavior, display/routing modes and vendor-specific controls require capability-scoped review; special capabilities remain non-dispatchable."},
		{ID: "functional_fxm", Status: "unsupported/unknown", Reason: "Only the default FXM baseline is present; functional-level FXM waits for confirmed semantic mappings."},
		{ID: "conformance_boundaries", Status: "unsupported/unknown", Reason: "Additional independent behavior, edge-case, version-boundary and human semantic review is required before any formal conformance decision."},
	}
}

func readProQ3ReviewCandidate(root string) proQ3ReviewCandidate {
	candidate := proQ3ReviewCandidate{
		BadgeID: "unknown", Trust: "agent-inferred", Status: "unsupported/unknown", RoutingEligible: false, SourceArtifact: candidateBadgesFile,
	}
	var document struct {
		Trust      string `json:"trust"`
		Candidates []struct {
			BadgeID          string   `json:"badge_id"`
			CategoryID       string   `json:"category_id"`
			Task             string   `json:"task"`
			RequiredFeatures []string `json:"required_features"`
			Status           string   `json:"status"`
			RoutingEligible  bool     `json:"routing_eligible"`
		} `json:"candidates"`
	}
	if readJSON(filepath.Join(root, candidateBadgesFile), &document) != nil || len(document.Candidates) == 0 {
		return candidate
	}
	item := document.Candidates[0]
	candidate.BadgeID = firstNonEmpty(strings.TrimSpace(item.BadgeID), "unknown")
	candidate.CategoryID = strings.TrimSpace(item.CategoryID)
	candidate.Task = strings.TrimSpace(item.Task)
	candidate.RequiredFeatures = uniqueSorted(item.RequiredFeatures)
	candidate.Status = firstNonEmpty(strings.TrimSpace(item.Status), "unsupported/unknown")
	candidate.RoutingEligible = false
	candidate.Trust = firstNonEmpty(strings.TrimSpace(document.Trust), "agent-inferred")
	return candidate
}

func proQ3ReviewSource(root, id, artifact, boundary string) proQ3ReviewEvidenceSource {
	source := proQ3ReviewEvidenceSource{
		ID: id, Artifact: artifact, Trust: "unsupported/unknown", Status: "unsupported/unknown", Availability: "missing", Boundary: boundary,
	}
	var header proQ3ReviewArtifactHeader
	if err := readJSON(filepath.Join(root, artifact), &header); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			source.Availability = "unreadable"
			source.Status = "unsupported/unknown"
		}
		return source
	}
	source.Availability = "present"
	source.Trust = firstNonEmpty(strings.TrimSpace(header.Trust), "observed")
	source.Status = firstNonEmpty(strings.TrimSpace(header.Status), "observed")
	source.CapturedAt = header.CapturedAt
	return source
}

func readProQ3ReviewHumanWitness(root string) proQ3ReviewHumanWitness {
	result := proQ3ReviewHumanWitness{
		Artifact: humanWitnessFile, Trust: "user-confirmed", AllRecordsNotConformed: true,
		InterpretationBoundary: "Raw human GUI testimony remains user-confirmed evidence. It is neither transformed into runtime rules nor used as a conformance or Credential grant.",
	}
	var evidence HumanWitnessEvidence
	if err := readJSON(filepath.Join(root, humanWitnessFile), &evidence); err == nil {
		result.Trust = firstNonEmpty(strings.TrimSpace(evidence.Trust), "user-confirmed")
		result.RecordCount = len(evidence.Records)
		roundCounts := map[string]int{}
		for _, record := range evidence.Records {
			roundCounts[record.RoundID]++
			if record.ConformanceStatus != "not-conformed" {
				result.AllRecordsNotConformed = false
			}
		}
		result.Rounds = proQ3ReviewRounds(root, roundCounts)
		return result
	}
	result.Rounds = proQ3ReviewRounds(root, map[string]int{})
	return result
}

func proQ3ReviewRounds(root string, recordCounts map[string]int) []proQ3ReviewWitnessRound {
	var document struct {
		Rounds []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"rounds"`
	}
	if readJSON(filepath.Join(root, witnessRoundsFile), &document) != nil {
		return nil
	}
	rounds := make([]proQ3ReviewWitnessRound, 0, len(document.Rounds))
	for _, round := range document.Rounds {
		if id := strings.TrimSpace(round.ID); id != "" {
			rounds = append(rounds, proQ3ReviewWitnessRound{RoundID: id, Status: strings.TrimSpace(round.Status), Records: recordCounts[id]})
		}
	}
	sort.Slice(rounds, func(i, j int) bool { return rounds[i].RoundID < rounds[j].RoundID })
	return rounds
}

func proQ3ReviewMachineSource(id, artifact string, sources []proQ3ReviewEvidenceSource) proQ3ReviewMachineReadiness {
	for _, source := range sources {
		if source.Artifact != artifact {
			continue
		}
		return proQ3ReviewMachineReadiness{
			ID: id, Trust: source.Trust, Status: source.Status, Artifact: artifact, Boundary: source.Boundary,
		}
	}
	return proQ3ReviewMachineReadiness{ID: id, Trust: "unsupported/unknown", Status: "unsupported/unknown", Artifact: artifact}
}

func persistProQ3EqualizerReviewSummary(root string, summary proQ3EqualizerReviewSummary) error {
	if err := writeJSON(filepath.Join(root, proQ3EqualizerReviewSummaryFile), summary); err != nil {
		return err
	}
	sources := make([]map[string]any, 0, len(summary.EvidenceSources))
	for _, source := range summary.EvidenceSources {
		sources = append(sources, map[string]any{
			"id": source.ID, "artifact": source.Artifact, "trust": source.Trust, "status": source.Status, "availability": source.Availability,
		})
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": summary.SchemaVersion, "status": summary.Status, "candidate_badge": summary.CandidateBadge.BadgeID,
		"candidate_routing_eligible": false, "credential_issuable": false, "catalog_mutation": "none", "spal_route_mutation": "none",
		"human_witness_record_count": summary.HumanWitness.RecordCount, "sources": sources,
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "pro_q_3_equalizer_review_summary",
		Trust:      "agent-inferred",
		Source:     "vpsforge.review_summary",
		Summary:    "Indexed Pro-Q 3 staging evidence for a later explicit review. The summary does not freeze equalizer.v2, promote mappings, issue a Credential, update Catalog, or enable SPAL routing.",
		Artifact:   proQ3EqualizerReviewSummaryFile,
		CapturedAt: summary.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["pro_q_3_equalizer_review_summary"] = proQ3EqualizerReviewSummaryFile
		manifest.UpdatedAt = summary.CapturedAt
	})
}
