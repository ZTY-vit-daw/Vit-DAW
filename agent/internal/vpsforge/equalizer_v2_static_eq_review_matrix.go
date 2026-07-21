package vpsforge

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const equalizerV2StaticEQMatrixSchema = "vit.vpsforge.equalizer_v2_static_eq_review_matrix.v1"

// EqualizerV2StaticEQReviewMatrixRequest produces an agent-inferred audit of
// existing staging evidence. It is deliberately a coverage index, not a
// semantic command schema or an executable VPS mapping.
type EqualizerV2StaticEQReviewMatrixRequest struct {
	Root string
	Now  time.Time
}

type EqualizerV2StaticEQReviewMatrixResult struct {
	Workspace string `json:"workspace"`
	Status    string `json:"status"`
	Artifact  string `json:"artifact"`
}

type equalizerV2StaticEQReviewMatrix struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`

	AuthorityBoundary []string                    `json:"authority_boundary"`
	ScopeReview       equalizerV2MatrixScopeRef   `json:"scope_review"`
	Rows              []equalizerV2MatrixRow      `json:"rows"`
	DeferredItems     []equalizerV2ScopeArea      `json:"deferred_items"`
	AuditConclusion   equalizerV2MatrixConclusion `json:"audit_conclusion"`
}

type equalizerV2MatrixScopeRef struct {
	Artifact string `json:"artifact"`
	Trust    string `json:"trust"`
	Status   string `json:"status"`
}

type equalizerV2MatrixRow struct {
	ID                        string                        `json:"id"`
	ScopeID                   string                        `json:"scope_id"`
	CoverageStatus            string                        `json:"coverage_status"`
	UserEvidence              equalizerV2MatrixUserEvidence `json:"user_evidence"`
	MachineEvidence           []equalizerV2MatrixEvidence   `json:"machine_evidence"`
	FunctionalTestEligibility string                        `json:"functional_test_eligibility"`
	RemainingGates            []string                      `json:"remaining_gates"`
	ExecutionBoundary         string                        `json:"execution_boundary"`
}

type equalizerV2MatrixUserEvidence struct {
	Artifact  string   `json:"artifact"`
	Trust     string   `json:"trust"`
	RecordIDs []string `json:"record_ids,omitempty"`
	Coverage  string   `json:"coverage"`
	Boundary  string   `json:"boundary"`
}

type equalizerV2MatrixEvidence struct {
	Artifact     string `json:"artifact"`
	Trust        string `json:"trust"`
	Status       string `json:"status"`
	Availability string `json:"availability"`
	Boundary     string `json:"boundary"`
}

type equalizerV2MatrixConclusion struct {
	ReadyForMachineLifecycleProbe bool   `json:"ready_for_machine_lifecycle_probe"`
	ReadyForFunctionalFXM         bool   `json:"ready_for_functional_fxm"`
	ExecutableActionMappings      int    `json:"executable_action_mappings"`
	BadgeFreezeAuthorized         bool   `json:"badge_freeze_authorized"`
	CredentialIssuable            bool   `json:"credential_issuable"`
	CatalogMutation               string `json:"catalog_mutation"`
	SPALRouteMutation             string `json:"spal_route_mutation"`
}

// WriteEqualizerV2StaticEQReviewMatrix combines the already-confirmed scope
// with user and machine evidence. It only determines whether a non-destructive
// staging test may be planned; it never promotes an observed plug-in parameter
// to a semantic action.
func WriteEqualizerV2StaticEQReviewMatrix(request EqualizerV2StaticEQReviewMatrixRequest) (EqualizerV2StaticEQReviewMatrixResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return EqualizerV2StaticEQReviewMatrixResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return EqualizerV2StaticEQReviewMatrixResult{}, fmt.Errorf("inspect static EQ review workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return EqualizerV2StaticEQReviewMatrixResult{}, fmt.Errorf("static EQ review matrix only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	var scope equalizerV2ScopeReview
	if err := readJSON(filepath.Join(root, equalizerV2ScopeReviewFile), &scope); err != nil {
		return EqualizerV2StaticEQReviewMatrixResult{}, fmt.Errorf("read required equalizer.v2 scope review: %w", err)
	}
	if scope.Trust != "user-confirmed" || scope.Status != "scope_confirmed_not_conformed" {
		return EqualizerV2StaticEQReviewMatrixResult{}, fmt.Errorf("equalizer.v2 scope review must remain user-confirmed and not-conformed")
	}
	var witnesses HumanWitnessEvidence
	if err := readJSON(filepath.Join(root, humanWitnessFile), &witnesses); err != nil {
		return EqualizerV2StaticEQReviewMatrixResult{}, fmt.Errorf("read human witness evidence: %w", err)
	}

	core := equalizerV2MatrixArtifact(root, proQ3CoreEQResultsFile, "bounded static frequency/gain/Q/shape response probe; observed only")
	parameterProbe := equalizerV2MatrixArtifact(root, parameterProbeFile, "generic host write/readback/rollback safety evidence; no EQ semantics")
	stateRoundtrip := equalizerV2MatrixArtifact(root, stateRoundtripFile, "generic state serialization/reload evidence; no EQ semantics")
	lifecycle := equalizerV2MatrixArtifact(root, proQ3StaticLifecycleResultsFile, "static Band Used/Enabled lifecycle behavior probe; observed only")

	lifecycleWitnesses := equalizerV2WitnessRecordIDs(witnesses, "创建", "旁通", "删除", "撤回")
	frequencyWitnesses := equalizerV2WitnessRecordIDs(witnesses, "freq", "dB", "Q", "3198", "Hz")
	shapeWitnesses := equalizerV2WitnessRecordIDs(witnesses, "类型", "滤波", "High Cut", "Low Cut")

	rows := []equalizerV2MatrixRow{
		{
			ID: "static_band_lifecycle", ScopeID: "static_band_lifecycle",
			CoverageStatus: equalizerV2MatrixLifecycleStatus(lifecycle, lifecycleWitnesses),
			UserEvidence: equalizerV2MatrixUserEvidence{
				Artifact: humanWitnessFile, Trust: "user-confirmed", RecordIDs: lifecycleWitnesses,
				Coverage: equalizerV2MatrixCoverage(lifecycleWitnesses),
				Boundary: "GUI evidence supports a review of lifecycle distinctions, but does not create an executable meaning for any observed parameter.",
			},
			MachineEvidence:           []equalizerV2MatrixEvidence{parameterProbe, stateRoundtrip, lifecycle},
			FunctionalTestEligibility: equalizerV2MatrixLifecycleEligibility(lifecycleWitnesses),
			RemainingGates: []string{
				"complete lifecycle behavior probe with fresh readback, state roundtrip and rollback",
				"independent review of create/enable/bypass/delete semantics before mapping",
			},
			ExecutionBoundary: "no executable lifecycle mapping; any parameter IDs remain observed testing selectors only",
		},
		{
			ID: "static_frequency_gain_q", ScopeID: "static_frequency_gain_q",
			CoverageStatus: equalizerV2MatrixStaticCoreStatus(core, frequencyWitnesses),
			UserEvidence: equalizerV2MatrixUserEvidence{
				Artifact: humanWitnessFile, Trust: "user-confirmed", RecordIDs: frequencyWitnesses,
				Coverage: equalizerV2MatrixCoverage(frequencyWitnesses),
				Boundary: "User interactions and listening reports are scoped evidence; they are not generalized acoustic rules or executable bindings.",
			},
			MachineEvidence:           []equalizerV2MatrixEvidence{parameterProbe, stateRoundtrip, core},
			FunctionalTestEligibility: "machine static-response evidence exists; function-level FXM remains blocked pending a reviewed action contract",
			RemainingGates: []string{
				"vendor-neutral action contract",
				"reviewed parameter/action mapping",
				"function-level FXM test design after mapping approval",
			},
			ExecutionBoundary: "frequency, gain and Q labels/IDs remain observed and non-executable",
		},
		{
			ID: "common_filter_shape_selection", ScopeID: "common_filter_shape_selection",
			CoverageStatus: equalizerV2MatrixStaticCoreStatus(core, shapeWitnesses),
			UserEvidence: equalizerV2MatrixUserEvidence{
				Artifact: humanWitnessFile, Trust: "user-confirmed", RecordIDs: shapeWitnesses,
				Coverage: equalizerV2MatrixCoverage(shapeWitnesses),
				Boundary: "Screenshots and display text support review only; no vendor shape label becomes a universal standard-action value.",
			},
			MachineEvidence:           []equalizerV2MatrixEvidence{stateRoundtrip, core},
			FunctionalTestEligibility: "bounded observed Shape discovery exists; semantic shape taxonomy and action mapping remain pending review",
			RemainingGates: []string{
				"vendor-neutral definition of any common shape subset",
				"reviewed selection semantics and edge behavior",
			},
			ExecutionBoundary: "observed Shape display choices remain non-executable",
		},
	}
	matrix := equalizerV2StaticEQReviewMatrix{
		SchemaVersion: equalizerV2StaticEQMatrixSchema,
		Trust:         "agent-inferred",
		Status:        "coverage_audit_not_conformed",
		CapturedAt:    nowOrCurrent(request.Now),
		WorkspaceID:   status.Manifest.WorkspaceID,
		AuthorityBoundary: []string{
			"This matrix indexes evidence coverage and test eligibility only; it is not a badge contract or executable VPS mapping.",
			"User-confirmed records, observed machine results and agent-inferred coverage conclusions remain separate evidence classes.",
			"An available machine probe never grants semantic authority, conformance, a Credential, Catalog mutation or SPAL route.",
		},
		ScopeReview:   equalizerV2MatrixScopeRef{Artifact: equalizerV2ScopeReviewFile, Trust: scope.Trust, Status: scope.Status},
		Rows:          rows,
		DeferredItems: append([]equalizerV2ScopeArea(nil), scope.ExcludedScope...),
		AuditConclusion: equalizerV2MatrixConclusion{
			ReadyForMachineLifecycleProbe: len(lifecycleWitnesses) > 0 && lifecycle.Availability == "missing",
			ReadyForFunctionalFXM:         false,
			ExecutableActionMappings:      0,
			BadgeFreezeAuthorized:         false,
			CredentialIssuable:            false,
			CatalogMutation:               "none",
			SPALRouteMutation:             "none",
		},
	}
	if err := persistEqualizerV2StaticEQReviewMatrix(root, matrix); err != nil {
		return EqualizerV2StaticEQReviewMatrixResult{}, err
	}
	return EqualizerV2StaticEQReviewMatrixResult{Workspace: root, Status: matrix.Status, Artifact: equalizerV2StaticEQMatrixFile}, nil
}

func equalizerV2MatrixArtifact(root, artifact, boundary string) equalizerV2MatrixEvidence {
	source := proQ3ReviewSource(root, artifact, artifact, boundary)
	return equalizerV2MatrixEvidence{
		Artifact: artifact, Trust: source.Trust, Status: source.Status, Availability: source.Availability, Boundary: boundary,
	}
}

func equalizerV2WitnessRecordIDs(evidence HumanWitnessEvidence, keywords ...string) []string {
	ids := []string{}
	for _, record := range evidence.Records {
		text := strings.ToLower(record.UserStatement + "\n" + record.PromptContext)
		for _, keyword := range keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				ids = append(ids, record.ID)
				break
			}
		}
	}
	return uniqueSorted(ids)
}

func equalizerV2MatrixCoverage(recordIDs []string) string {
	if len(recordIDs) == 0 {
		return "no action-specific user witness match was found; do not infer coverage"
	}
	return "user-confirmed records are available for later review; they are not conformance"
}

func equalizerV2MatrixStaticCoreStatus(core equalizerV2MatrixEvidence, recordIDs []string) string {
	if core.Availability == "present" && core.Trust == "observed" && core.Status == "observed_completed" && len(recordIDs) > 0 {
		return "user_and_machine_evidence_assembled_not_conformed"
	}
	return "evidence_incomplete_not_conformed"
}

func equalizerV2MatrixLifecycleStatus(lifecycle equalizerV2MatrixEvidence, recordIDs []string) string {
	if lifecycle.Availability == "present" && lifecycle.Trust == "observed" && lifecycle.Status == "observed_completed" && len(recordIDs) > 0 {
		return "user_and_machine_evidence_assembled_not_conformed"
	}
	if len(recordIDs) > 0 {
		return "user_evidence_present_machine_lifecycle_probe_pending"
	}
	return "evidence_incomplete_not_conformed"
}

func equalizerV2MatrixLifecycleEligibility(recordIDs []string) string {
	if len(recordIDs) == 0 {
		return "not eligible until action-specific user evidence is identified"
	}
	return "eligible for a non-destructive staging lifecycle probe using observed selectors; results remain observed"
}

func persistEqualizerV2StaticEQReviewMatrix(root string, matrix equalizerV2StaticEQReviewMatrix) error {
	if err := writeJSON(filepath.Join(root, equalizerV2StaticEQMatrixFile), matrix); err != nil {
		return err
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": matrix.SchemaVersion, "status": matrix.Status,
		"row_count": len(matrix.Rows), "ready_for_machine_lifecycle_probe": matrix.AuditConclusion.ReadyForMachineLifecycleProbe,
		"ready_for_functional_fxm": false, "executable_action_mappings": 0,
		"badge_freeze_authorized": false, "credential_issuable": false, "catalog_mutation": "none", "spal_route_mutation": "none",
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "equalizer_v2_static_eq_review_matrix",
		Trust:      "agent-inferred",
		Source:     "vpsforge.equalizer_v2_static_eq_matrix",
		Summary:    "Indexed static equalizer.v2 evidence coverage and bounded test eligibility. The matrix does not define executable action mappings or grant badge, Credential, Catalog or SPAL authority.",
		Artifact:   equalizerV2StaticEQMatrixFile,
		CapturedAt: matrix.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["equalizer_v2_static_eq_review_matrix"] = equalizerV2StaticEQMatrixFile
		manifest.UpdatedAt = matrix.CapturedAt
	})
}
