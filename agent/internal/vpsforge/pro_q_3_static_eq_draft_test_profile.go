package vpsforge

// This file prepares a deliberately isolated local test bridge.  It is not a
// conformance implementation: its only executable surface is the already
// existing Draft-test transport, which requires a fresh live surface,
// preimage, explicit write confirmation, fresh readback and full rollback.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/vps"
)

const (
	proQ3StaticEQDraftTestProfileSchema      = "vit.vpsforge.pro_q_3_static_eq_draft_test_profile.v1"
	proQ3StaticEQDraftTestEvidenceDir        = "evidence/pro_q_3_static_eq_draft_test_profile"
	proQ3StaticEQDraftTestRuntimeLibraryFile = "draft_test_runtime/vps_library_v3.json"
	proQ3StaticEQDraftTestBindingStatus      = "agent-inferred_test_only"
	proQ3StaticEQDraftTestExecutionScope     = "local_agent_integration_test_only"
)

// ProQ3StaticEQDraftTestProfileRequest prepares one non-canonical, staging
// library for actual Vit transport tests.  The caller must explicitly launch
// a test process with VIT_VPS_LIBRARY_V3_PATH pointing to that generated
// library; the canonical user library is never read or changed here.
type ProQ3StaticEQDraftTestProfileRequest struct {
	Root string
	Now  time.Time
}

type ProQ3StaticEQDraftTestProfileResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ProfileArtifact string `json:"profile_artifact"`
	RuntimeLibrary  string `json:"runtime_library"`
	RunArtifact     string `json:"run_artifact"`
}

type proQ3StaticEQDraftTestProfile struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	ProfileID     string    `json:"profile_id"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                        `json:"authority_boundary"`
	Plugin            vps.PluginIdentity              `json:"plugin"`
	Candidate         proQ3StaticEQDraftTestCandidate `json:"candidate"`
	Mappings          []proQ3StaticEQDraftTestMapping `json:"mappings"`
	TestScenarios     []proQ3StaticEQDraftTestScene   `json:"test_scenarios"`
	Runtime           proQ3StaticEQDraftTestRuntime   `json:"runtime"`
	Deferred          []string                        `json:"deferred"`
}

type proQ3StaticEQDraftTestCandidate struct {
	VPSID              string `json:"vps_id"`
	Revision           int    `json:"revision"`
	BadgeID            string `json:"badge_id"`
	BadgeStatus        string `json:"badge_status"`
	RoutingEligible    bool   `json:"routing_eligible"`
	CredentialIssuable bool   `json:"credential_issuable"`
}

type proQ3StaticEQDraftTestMapping struct {
	Key               string   `json:"key"`
	ParameterID       string   `json:"parameter_id"`
	ObservedLabel     string   `json:"observed_label"`
	ObservedValues    []string `json:"observed_values,omitempty"`
	BindingStatus     string   `json:"binding_status"`
	ExecutionScope    string   `json:"execution_scope"`
	SelectionBoundary string   `json:"selection_boundary"`
}

type proQ3StaticEQDraftTestScene struct {
	ID              string                              `json:"id"`
	Purpose         string                              `json:"purpose"`
	ObservationMode string                              `json:"observation_mode"`
	Changes         []proQ3StaticEQDraftTestSceneChange `json:"changes"`
	Boundary        string                              `json:"boundary"`
}

type proQ3StaticEQDraftTestSceneChange struct {
	MappingKey          string  `json:"mapping_key"`
	RequestedNormalized float64 `json:"requested_normalized"`
}

type proQ3StaticEQDraftTestRuntime struct {
	LibraryPath                  string `json:"library_path"`
	CanonicalLibraryTouched      bool   `json:"canonical_library_touched"`
	CatalogVisible               bool   `json:"catalog_visible"`
	SPALDispatch                 bool   `json:"spal_dispatch"`
	RequiresExplicitWriteConfirm bool   `json:"requires_explicit_write_confirmation"`
	RequiresFreshLiveSurface     bool   `json:"requires_fresh_live_surface"`
	RequiresCompleteRollback     bool   `json:"requires_complete_rollback"`
}

// WriteProQ3StaticEQDraftTestProfile converts a very small, already observed
// Pro-Q 3 Band 1 selector set into an isolated candidate Draft library.  It is
// intentionally raw/normalized test control, not a vendor-neutral equalizer
// action implementation.  Its generated Draft cannot appear in a Provider
// Catalog because it contains no Credential.
func WriteProQ3StaticEQDraftTestProfile(request ProQ3StaticEQDraftTestProfileRequest) (ProQ3StaticEQDraftTestProfileResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, fmt.Errorf("inspect Pro-Q 3 draft-test workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return ProQ3StaticEQDraftTestProfileResult{}, fmt.Errorf("draft-test profile only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	if err := proQ3StaticEQDraftTestRequiredArtifacts(root); err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, err
	}
	var source vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &source); err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, fmt.Errorf("read staged VPS Draft: %w", err)
	}
	if source.Status != vps.VPSStatusDraft || len(source.ProviderCredentials) != 0 {
		return ProQ3StaticEQDraftTestProfileResult{}, fmt.Errorf("staged Pro-Q 3 test source must remain a credential-free Draft")
	}

	var core proQ3CoreEQProbeArtifact
	if err := readJSON(filepath.Join(root, proQ3CoreEQResultsFile), &core); err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, fmt.Errorf("read observed Pro-Q 3 core probe: %w", err)
	}
	mappings, err := proQ3StaticEQDraftTestMappings(core.ObservedControls, core.ShapeDiscovery)
	if err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, err
	}

	now := nowOrCurrent(request.Now)
	profileID := stableID("pro_q_3_static_eq_draft_test_profile", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(proQ3StaticEQDraftTestEvidenceDir, profileID+".json"))
	candidate := proQ3StaticEQDraftTestDocument(source, mappings, now)
	runtimeLibraryPath := filepath.Join(root, filepath.FromSlash(proQ3StaticEQDraftTestRuntimeLibraryFile))
	if err := proQ3StaticEQDraftTestWriteLibrary(runtimeLibraryPath, candidate); err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, err
	}
	profile := proQ3StaticEQDraftTestProfile{
		SchemaVersion: proQ3StaticEQDraftTestProfileSchema,
		Trust:         "agent-inferred",
		Status:        "candidate_test_runtime_ready_not_conformed",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		ProfileID:     profileID,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"The generated library is an isolated staging-only Draft-test runtime, never the canonical VPS Library.",
			"Each selector is agent-inferred from bounded observed Pro-Q 3 evidence and remains non-conformed.",
			"Only the local Draft-test endpoint may use these normalized selectors after a fresh live surface check and explicit write confirmation.",
			"The profile cannot grant a semantic mapping, equalizer.v2 badge, Credential, Catalog visibility, or SPAL routing.",
		},
		Plugin: status.Manifest.PluginIdentity,
		Candidate: proQ3StaticEQDraftTestCandidate{
			VPSID: candidate.ID, Revision: candidate.Revision, BadgeID: "equalizer.v2", BadgeStatus: "candidate_not_granted",
			RoutingEligible: false, CredentialIssuable: false,
		},
		Mappings: mappings,
		TestScenarios: []proQ3StaticEQDraftTestScene{{
			ID:              "pro_q_3_band_1_bell_audition_candidate",
			Purpose:         "Hold one observed Band 1 Bell test state for a human to inspect and listen to, then perform an explicit complete rollback.",
			ObservationMode: "manual_witness",
			Changes: []proQ3StaticEQDraftTestSceneChange{
				{MappingKey: "pro_q_3_test_band_1.used", RequestedNormalized: 1},
				{MappingKey: "pro_q_3_test_band_1.enabled", RequestedNormalized: 1},
				{MappingKey: "pro_q_3_test_band_1.frequency_normalized", RequestedNormalized: 0.72},
				{MappingKey: "pro_q_3_test_band_1.gain_normalized", RequestedNormalized: 0.625},
				{MappingKey: "pro_q_3_test_band_1.q_normalized", RequestedNormalized: 0.5},
				{MappingKey: "pro_q_3_test_band_1.shape", RequestedNormalized: 0},
			},
			Boundary: "This is a bounded normalized transport scene selected from observed controls. It is not a generic task instruction or proof of semantic behavior.",
		}},
		Runtime: proQ3StaticEQDraftTestRuntime{
			LibraryPath: filepath.ToSlash(proQ3StaticEQDraftTestRuntimeLibraryFile), CanonicalLibraryTouched: false,
			CatalogVisible: false, SPALDispatch: false, RequiresExplicitWriteConfirm: true,
			RequiresFreshLiveSurface: true, RequiresCompleteRollback: true,
		},
		Deferred: []string{
			"vendor-neutral equalizer.v2 action contract and feature matrix",
			"physical-unit conversion from Hz, dB and Q requests to normalized host values",
			"Dynamic EQ, automatic/match EQ, spectrum analysis and phase modes",
			"any Credential, Catalog or SPAL route",
		},
	}
	if err := persistProQ3StaticEQDraftTestProfile(root, profile); err != nil {
		return ProQ3StaticEQDraftTestProfileResult{}, err
	}
	return ProQ3StaticEQDraftTestProfileResult{
		Workspace: root, Status: profile.Status, ProfileArtifact: proQ3StaticEQDraftTestProfileFile,
		RuntimeLibrary: runtimeLibraryPath, RunArtifact: runArtifact,
	}, nil
}

func proQ3StaticEQDraftTestRequiredArtifacts(root string) error {
	for _, artifact := range []string{equalizerV2ScopeReviewFile, proQ3CoreEQResultsFile, proQ3StaticLifecycleResultsFile} {
		if _, err := filepath.Abs(filepath.Join(root, artifact)); err != nil {
			return fmt.Errorf("resolve required draft-test evidence %s: %w", artifact, err)
		}
		if _, err := os.Stat(filepath.Join(root, artifact)); err != nil {
			return fmt.Errorf("required draft-test evidence %s is unavailable: %w", artifact, err)
		}
	}
	return nil
}

func proQ3StaticEQDraftTestMappings(controls []proQ3CoreEQObservedControl, shapes []proQ3CoreEQChoice) ([]proQ3StaticEQDraftTestMapping, error) {
	roleToMapping := []struct {
		role string
		key  string
	}{
		{"band_used", "pro_q_3_test_band_1.used"},
		{"band_enabled", "pro_q_3_test_band_1.enabled"},
		{"band_frequency", "pro_q_3_test_band_1.frequency_normalized"},
		{"band_gain", "pro_q_3_test_band_1.gain_normalized"},
		{"band_q", "pro_q_3_test_band_1.q_normalized"},
		{"band_shape", "pro_q_3_test_band_1.shape"},
	}
	byRole := map[string]proQ3CoreEQObservedControl{}
	for _, control := range controls {
		byRole[control.Role] = control
	}
	shapeValues := make([]string, 0, len(shapes))
	for _, shape := range shapes {
		if strings.TrimSpace(shape.ObservedDisplay) != "" {
			shapeValues = append(shapeValues, shape.ObservedDisplay)
		}
	}
	result := make([]proQ3StaticEQDraftTestMapping, 0, len(roleToMapping))
	for _, definition := range roleToMapping {
		control, found := byRole[definition.role]
		if !found || strings.TrimSpace(control.ID) == "" {
			return nil, fmt.Errorf("observed Pro-Q 3 core probe lacks required %s selector", definition.role)
		}
		values := []string(nil)
		if definition.role == "band_shape" {
			values = append(values, shapeValues...)
		}
		result = append(result, proQ3StaticEQDraftTestMapping{
			Key: definition.key, ParameterID: control.ID, ObservedLabel: control.HostLabel, ObservedValues: values,
			BindingStatus: proQ3StaticEQDraftTestBindingStatus, ExecutionScope: proQ3StaticEQDraftTestExecutionScope,
			SelectionBoundary: "Selected from the current observed Pro-Q 3 VST3 surface and bounded staging probes. The label, ID and display text are not generic semantic authority.",
		})
	}
	return result, nil
}

func proQ3StaticEQDraftTestDocument(source vps.VPSDocument, mappings []proQ3StaticEQDraftTestMapping, now time.Time) vps.VPSDocument {
	candidate := source
	candidate.Status = vps.VPSStatusDraft
	candidate.ProviderCredentials = nil
	candidate.UpdatedAt = now
	byID := make(map[string]proQ3StaticEQDraftTestMapping, len(mappings))
	for _, mapping := range mappings {
		byID[mapping.ParameterID] = mapping
	}
	for index := range candidate.ControlSurface.Mappings {
		mapping, selected := byID[candidate.ControlSurface.Mappings[index].ParameterID]
		if !selected {
			continue
		}
		parts := strings.Split(mapping.Key, ".")
		candidate.ControlSurface.Mappings[index].ComponentID = strings.Join(parts[:len(parts)-1], ".")
		candidate.ControlSurface.Mappings[index].SemanticSlot = parts[len(parts)-1]
		candidate.ControlSurface.Mappings[index].BindingStatus = mapping.BindingStatus
		candidate.ControlSurface.Mappings[index].ExecutionScope = mapping.ExecutionScope
		candidate.ControlSurface.Mappings[index].Confirmed = false
		candidate.ControlSurface.Mappings[index].Warning = "Candidate local Draft-test selector only. It is agent-inferred, uses normalized host values, requires explicit confirmation and complete rollback, and is not conformed or dispatchable."
		candidate.ControlSurface.Mappings[index].EvidenceRefs = uniqueSorted(append(candidate.ControlSurface.Mappings[index].EvidenceRefs,
			"vpsforge:"+proQ3CoreEQResultsFile,
			"vpsforge:"+proQ3StaticLifecycleResultsFile,
		))
		if strings.HasSuffix(mapping.Key, ".shape") {
			candidate.ControlSurface.Mappings[index].EnumValues = append([]string(nil), mapping.ObservedValues...)
		}
	}
	return candidate
}

func proQ3StaticEQDraftTestWriteLibrary(path string, candidate vps.VPSDocument) error {
	library, err := vps.NewLibrary(path)
	if err != nil {
		return fmt.Errorf("open isolated Pro-Q 3 draft-test library: %w", err)
	}
	if _, _, err := library.ImportDraft(candidate); err != nil {
		return fmt.Errorf("write isolated Pro-Q 3 draft-test library: %w", err)
	}
	return nil
}

func persistProQ3StaticEQDraftTestProfile(root string, profile proQ3StaticEQDraftTestProfile) error {
	if err := writeJSON(filepath.Join(root, proQ3StaticEQDraftTestProfileFile), profile); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(root, filepath.FromSlash(profile.RunArtifact)), profile); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]any{
		"schema_version": profile.SchemaVersion, "profile_id": profile.ProfileID, "status": profile.Status,
		"mapping_count": len(profile.Mappings), "canonical_library_touched": false,
		"credential_issuable": false, "catalog_visible": false, "spal_dispatch": false,
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind: "pro_q_3_static_eq_draft_test_profile", Trust: "agent-inferred", Source: "vpsforge.pro_q_3_static_eq_draft_test_profile",
		Summary:  "Prepared an isolated Pro-Q 3 Draft-test runtime from observed selectors. It supports only explicit local transport tests with fresh readback and rollback; it is not a conformed mapping, Credential, Catalog entry or SPAL route.",
		Artifact: profile.RunArtifact, CapturedAt: profile.CapturedAt, Data: data,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["pro_q_3_static_eq_draft_test_profile"] = proQ3StaticEQDraftTestProfileFile
		manifest.UpdatedAt = profile.CapturedAt
	})
}
