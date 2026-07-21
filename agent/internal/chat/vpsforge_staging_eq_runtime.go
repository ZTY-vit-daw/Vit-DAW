package chat

// This file is the deliberately narrow bridge between a VPS Forge staging VPS
// and the ordinary Agent equalizer capability tool.  It is not a Provider
// runtime: the bridge is opt-in, performs only a temporary local transaction
// with complete rollback, and never creates a Credential, Catalog entry, or
// SPAL route.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const (
	vpsForgeStagingVPSPathEnv        = "VIT_VPS_FORGE_STAGING_VPS_PATH"
	vpsForgeStagingEQWorkflow        = "vpsforge_staging_eq"
	vpsForgeStagingEQSource          = "vpsforge_staging"
	vpsForgeStagingEQCapabilityID    = "vpsforge.staging.equalizer"
	vpsForgeStagingEQOperationSchema = "vit.vpsforge.staging_eq_operation.v1"
	vpsForgeStagingVPSSchema         = "vit.vpsforge.pro_q_3_static_eq_staging_vps.v1"
	vpsForgeStagingVPSStatus         = "staging_vps_implemented_not_conformed"
)

type vpsForgeStagingVPSArtifact struct {
	SchemaVersion              string                            `json:"schema_version"`
	Trust                      string                            `json:"trust"`
	Status                     string                            `json:"status"`
	VPSID                      string                            `json:"vps_id"`
	VPSRevision                int                               `json:"vps_revision"`
	ImplementedMappings        []vpsForgeStagingArtifactMapping  `json:"implemented_mappings"`
	BadgeActionImplementations []vps.VPSActionImplementation     `json:"badge_action_implementations"`
	ImplementedSchemas         []string                          `json:"implemented_schemas"`
	StagingBinding             *spal.EQV2Binding                 `json:"spal_eq_v2_staging_binding"`
	Runtime                    vpsForgeStagingVPSArtifactRuntime `json:"runtime"`
}

type vpsForgeStagingArtifactMapping struct {
	Key            string `json:"key"`
	ParameterID    string `json:"parameter_id"`
	BindingStatus  string `json:"binding_status"`
	ExecutionScope string `json:"execution_scope"`
}

type vpsForgeStagingVPSArtifactRuntime struct {
	LibraryPath        string `json:"library_path"`
	CanonicalTouched   bool   `json:"canonical_library_touched"`
	CredentialIssuable bool   `json:"credential_issuable"`
	CatalogVisible     bool   `json:"catalog_visible"`
	SPALDispatch       bool   `json:"spal_dispatch"`
}

type vpsForgeStagingEQConfig struct {
	Artifact     vpsForgeStagingVPSArtifact
	ArtifactPath string
	ArtifactHash string
	Library      *vps.Library
	Document     vps.VPSDocument
	Adapter      *spal.VPSEQV2StagingAdapter
}

type vpsForgeStagingDesiredChange struct {
	MappingKey          string  `json:"mapping_key"`
	RequestedNormalized float64 `json:"requested_normalized"`
}

// vpsForgeStagingEQOperation is persisted in the ordinary PendingPlan.  It
// intentionally contains no preimage: a full fresh preimage is captured only
// immediately before execution, then verified during rollback.
type vpsForgeStagingEQOperation struct {
	SchemaVersion                    string                         `json:"schema_version"`
	ArtifactPath                     string                         `json:"artifact_path"`
	ArtifactSHA256                   string                         `json:"artifact_sha256"`
	VPSID                            string                         `json:"vps_id"`
	VPSRevision                      int                            `json:"vps_revision"`
	Target                           vpsDraftTestTarget             `json:"target"`
	Instruction                      spal.Instruction               `json:"instruction"`
	DesiredChanges                   []vpsForgeStagingDesiredChange `json:"desired_changes"`
	PlannedSurfaceCompatibility      string                         `json:"planned_surface_compatibility"`
	ExpectedParameterSurface         string                         `json:"expected_parameter_surface"`
	ObservedParameterSurface         string                         `json:"observed_parameter_surface"`
	SurfaceMismatchAcknowledgedOnUse bool                           `json:"surface_mismatch_acknowledged_on_use"`
}

type vpsForgeStagingMappingSpec struct {
	Key         string
	ParameterID string
}

var vpsForgeProQ3Band1StagingMappings = []vpsForgeStagingMappingSpec{
	{Key: "pro_q_3_band_1_staging.used", ParameterID: "0"},
	{Key: "pro_q_3_band_1_staging.enabled", ParameterID: "1"},
	{Key: "pro_q_3_band_1_staging.frequency_normalized", ParameterID: "2"},
	{Key: "pro_q_3_band_1_staging.gain_normalized", ParameterID: "3"},
	{Key: "pro_q_3_band_1_staging.q_normalized", ParameterID: "7"},
	{Key: "pro_q_3_band_1_staging.shape", ParameterID: "8"},
}

func (s *Server) loadVPSForgeStagingEQConfig() (vpsForgeStagingEQConfig, bool, error) {
	configured := strings.TrimSpace(os.Getenv(vpsForgeStagingVPSPathEnv))
	if configured == "" {
		return vpsForgeStagingEQConfig{}, false, nil
	}
	path, err := filepath.Abs(configured)
	if err != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("resolve %s: %w", vpsForgeStagingVPSPathEnv, err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("read VPS Forge staging artifact: %w", err)
	}
	var artifact vpsForgeStagingVPSArtifact
	if err := json.Unmarshal(payload, &artifact); err != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("decode VPS Forge staging artifact: %w", err)
	}
	if artifact.SchemaVersion != vpsForgeStagingVPSSchema || artifact.Status != vpsForgeStagingVPSStatus || !strings.EqualFold(strings.TrimSpace(artifact.Trust), "user-confirmed") || strings.TrimSpace(artifact.VPSID) == "" || artifact.VPSRevision < 1 {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("VPS Forge staging artifact is not an implemented Pro-Q 3 staging VPS")
	}
	if artifact.Runtime.CanonicalTouched || artifact.Runtime.CredentialIssuable || artifact.Runtime.CatalogVisible || artifact.Runtime.SPALDispatch {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("VPS Forge staging artifact violates its non-routeable authority boundary")
	}
	if err := vpsForgeStagingArtifactMappingsValid(artifact); err != nil {
		return vpsForgeStagingEQConfig{}, true, err
	}
	libraryPath, err := vpsForgeStagingRuntimeLibraryPath(path, artifact.Runtime.LibraryPath)
	if err != nil {
		return vpsForgeStagingEQConfig{}, true, err
	}
	library, err := vps.NewLibrary(libraryPath)
	if err != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("open isolated staging VPS library: %w", err)
	}
	document, found, err := library.Get(artifact.VPSID)
	if err != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("read isolated staging VPS: %w", err)
	}
	if !found || document.Revision != artifact.VPSRevision || document.Status != vps.VPSStatusDraft {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("isolated staging VPS does not match the configured staging artifact")
	}
	if len(document.ProviderCredentials) != 0 {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("configured staging VPS contains Provider Credentials and cannot use the staging bridge")
	}
	if err := vpsForgeStagingDocumentMappingsValid(document); err != nil {
		return vpsForgeStagingEQConfig{}, true, err
	}
	if !strings.EqualFold(strings.TrimSpace(document.PluginIdentity.Manufacturer), "FabFilter") || !strings.EqualFold(strings.TrimSpace(document.PluginIdentity.Name), "Pro-Q 3") || !strings.EqualFold(strings.TrimSpace(document.PluginIdentity.Format), "VST3") {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("configured staging VPS is not the expected FabFilter Pro-Q 3 VST3 slice")
	}
	if err := vpsForgeStagingActionArtifactValid(artifact, document); err != nil {
		return vpsForgeStagingEQConfig{}, true, err
	}
	adapter, adapterErr := spal.NewVPSEQV2StagingAdapter(spal.EQV2StagingProviderDefinition{
		Descriptor: spal.ProviderDescriptor{
			ID: "vpsforge.staging.eq_v2:" + document.ID, AdapterVersion: "v2", PluginName: document.PluginIdentity.Name,
			PluginFormat: document.PluginIdentity.Format, ProfileSignature: document.PluginIdentity.Fingerprint.ParameterSurface,
			Status: spal.ProviderCandidate, Experimental: true, SupportedSchemas: append([]string(nil), artifact.ImplementedSchemas...),
		},
		VPSID: document.ID, StagingID: document.ID + ":" + strconv.Itoa(document.Revision), Binding: *artifact.StagingBinding,
		ImplementedSchemas: append([]string(nil), artifact.ImplementedSchemas...),
	})
	if adapterErr != nil {
		return vpsForgeStagingEQConfig{}, true, fmt.Errorf("load VPS Forge staging action compiler: %w", adapterErr)
	}
	sum := sha256.Sum256(payload)
	return vpsForgeStagingEQConfig{
		Artifact: artifact, ArtifactPath: path, ArtifactHash: "sha256:" + hex.EncodeToString(sum[:]), Library: library, Document: document, Adapter: adapter,
	}, true, nil
}

func vpsForgeStagingActionArtifactValid(artifact vpsForgeStagingVPSArtifact, document vps.VPSDocument) error {
	if artifact.StagingBinding == nil || len(artifact.ImplementedSchemas) == 0 || len(artifact.BadgeActionImplementations) == 0 {
		return fmt.Errorf("VPS Forge staging artifact predates the shared action implementation; regenerate this isolated staging VPS before use")
	}
	for _, implementation := range artifact.BadgeActionImplementations {
		contract, known := vps.BadgeContractFor(implementation.BadgeID)
		if !known || implementation.ValidateAgainst(contract) != nil {
			return fmt.Errorf("VPS Forge staging artifact has an invalid task-badge action implementation %s", implementation.ActionID)
		}
		if !containsIgnoreCase(artifact.ImplementedSchemas, implementation.SchemaID) {
			return fmt.Errorf("VPS Forge staging artifact does not advertise implementation schema %s", implementation.SchemaID)
		}
		matched := false
		for _, documentImplementation := range document.BadgeActionImplementations {
			if documentImplementation.BadgeID == implementation.BadgeID && documentImplementation.ActionID == implementation.ActionID && documentImplementation.SchemaID == implementation.SchemaID && documentImplementation.BindingRef == implementation.BindingRef && documentImplementation.Status == implementation.Status {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("isolated staging VPS does not match action implementation %s", implementation.ActionID)
		}
	}
	return nil
}

func vpsForgeStagingRuntimeLibraryPath(artifactPath, declaredPath string) (string, error) {
	declaredPath = strings.TrimSpace(declaredPath)
	if declaredPath == "" {
		return "", fmt.Errorf("VPS Forge staging artifact has no isolated runtime library path")
	}
	root := filepath.Dir(artifactPath)
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(declaredPath)))
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve isolated staging library path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("VPS Forge staging runtime library must remain inside its workspace")
	}
	return candidate, nil
}

func vpsForgeStagingArtifactMappingsValid(artifact vpsForgeStagingVPSArtifact) error {
	available := map[string]vpsForgeStagingArtifactMapping{}
	for _, mapping := range artifact.ImplementedMappings {
		if key := strings.TrimSpace(mapping.Key); key != "" {
			available[key] = mapping
		}
	}
	for _, expected := range vpsForgeProQ3Band1StagingMappings {
		mapping, found := available[expected.Key]
		if !found || strings.TrimSpace(mapping.ParameterID) != expected.ParameterID || !strings.EqualFold(strings.TrimSpace(mapping.BindingStatus), "user-confirmed_staging_executable") || !strings.EqualFold(strings.TrimSpace(mapping.ExecutionScope), "forge_staging_actual_test") {
			return fmt.Errorf("VPS Forge staging artifact lacks the expected non-routeable mapping %s", expected.Key)
		}
	}
	return nil
}

func vpsForgeStagingDocumentMappingsValid(document vps.VPSDocument) error {
	available := map[string]vps.ControlSurfaceMapping{}
	for _, mapping := range document.ControlSurface.Mappings {
		if key := vpsDraftTestMappingKey(mapping); key != "" {
			available[key] = mapping
		}
	}
	for _, expected := range vpsForgeProQ3Band1StagingMappings {
		mapping, found := available[expected.Key]
		if !found || strings.TrimSpace(mapping.ParameterID) != expected.ParameterID || !vpsDraftTestOnlyMapping(mapping) {
			return fmt.Errorf("isolated staging VPS lacks the expected executable staging mapping %s", expected.Key)
		}
	}
	return nil
}

// handleVPSForgeStagingEQ returns handled=false only when staging is not
// configured.  Once an isolated staging runtime is explicitly enabled, it
// refuses mismatched targets rather than silently falling through to an
// unrelated verified Provider.
func (s *Server) handleVPSForgeStagingEQ(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, parsed spalEQV2Request) (ChatResponse, bool) {
	config, enabled, configErr := s.loadVPSForgeStagingEQConfig()
	if !enabled {
		return ChatResponse{}, false
	}
	if configErr != nil {
		return vpsForgeStagingBlockedResponse(conversationID, goal, "VPS Forge staging bridge configuration is unavailable: "+configErr.Error(), nil), true
	}
	target := vpsForgeStagingTargetFromRequest(parsed)
	if err := target.valid(); err != nil {
		return vpsForgeStagingClarificationResponse(conversationID, goal, []string{"selected Pro-Q 3 plugin instance"}), true
	}
	fresh, err := s.readVPSDraftTestParameterSurface(ctx, target, vpsForgeStagingRequestContext(config.Artifact.VPSID, "plan"))
	if err != nil {
		return vpsForgeStagingBlockedResponse(conversationID, goal, "Unable to read the selected staging plug-in surface; no parameter write was attempted: "+err.Error(), map[string]any{"target": target}), true
	}
	digest := buildPluginParameterDigest(fresh)
	if err := vpsForgeStagingTargetMatchesDigest(target, config.Document, digest); err != nil {
		return vpsForgeStagingBlockedResponse(conversationID, goal, err.Error(), map[string]any{"target": target}), true
	}
	surface, surfaceErr := vpsDraftTestValidateLiveSurface(config.Document, digest, "")
	if surfaceErr != nil && surface.Status != vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		return vpsForgeStagingBlockedResponse(conversationID, goal, surfaceErr.Error(), map[string]any{"target": target, "surface_compatibility": surface.Status}), true
	}
	if response, supported := vpsForgeStagingInstructionResponse(conversationID, goal, config, target, parsed, surface); !supported {
		return response, true
	}
	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return vpsForgeStagingInspectionResponse(conversationID, goal, config, target, parsed.Instruction, surface), true
	}
	desired, conversionErr := vpsForgeStagingCompiledDesiredChanges(config, target, parsed.Instruction)
	if conversionErr != nil {
		return vpsForgeStagingClarificationResponse(conversationID, goal, []string{conversionErr.Error()}), true
	}
	operation := vpsForgeStagingEQOperation{
		SchemaVersion:                    vpsForgeStagingEQOperationSchema,
		ArtifactPath:                     config.ArtifactPath,
		ArtifactSHA256:                   config.ArtifactHash,
		VPSID:                            config.Artifact.VPSID,
		VPSRevision:                      config.Artifact.VPSRevision,
		Target:                           target,
		Instruction:                      cloneSPALInstruction(parsed.Instruction),
		DesiredChanges:                   desired,
		PlannedSurfaceCompatibility:      surface.Status,
		ExpectedParameterSurface:         surface.ExpectedParameterSurface,
		ObservedParameterSurface:         surface.ObservedParameterSurface,
		SurfaceMismatchAcknowledgedOnUse: surface.Status == vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged,
	}
	planID := "plan_" + randomID()
	presentation := vpsForgeStagingProposalPresentation(planID, target, parsed.Instruction, desired, surface)
	operationData, err := vpsForgeStagingOperationMap(operation)
	if err != nil {
		return vpsForgeStagingBlockedResponse(conversationID, goal, "Could not persist the staging proposal: "+err.Error(), nil), true
	}
	workflowData := map[string]any{
		"conversation_id":            conversationID,
		"execution_route":            vpsForgeStagingEQWorkflow,
		"source":                     vpsForgeStagingEQSource,
		"routing_eligible":           false,
		"catalog_visible":            false,
		"credential_issuance":        false,
		"spal_dispatch":              false,
		"vps_id":                     config.Artifact.VPSID,
		"vps_revision":               config.Artifact.VPSRevision,
		"proposal_id":                planID,
		"semantic_action":            instructionMap(parsed.Instruction),
		"surface_compatibility":      surface.Status,
		"expected_parameter_surface": surface.ExpectedParameterSurface,
		"observed_parameter_surface": surface.ObservedParameterSurface,
		"operation":                  operationData,
	}
	if surface.Status == vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		workflowData["surface_mismatch_requires_confirmation"] = true
	}
	plan := PendingPlan{
		ID: planID, CreatedAt: time.Now().UTC(), Context: contextWithGoal(contextWithConversationID(cloneStringAnyMap(req.Context), conversationID), goal.GoalID, goal.RunID),
		Preview: presentationConclusion(presentation), Workflow: vpsForgeStagingEQWorkflow, WorkflowData: workflowData,
	}
	s.mu.Lock()
	s.pending[plan.ID] = plan
	s.mu.Unlock()
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: plan.ID,
		Preview: presentationConclusion(presentation), ProposalPresentation: presentation,
		Workflow: vpsForgeStagingEQWorkflow, GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: workflowData,
	}, true
}

func vpsForgeStagingTargetFromRequest(request spalEQV2Request) vpsDraftTestTarget {
	trackID := strings.TrimSpace(request.TargetRef)
	if strings.HasPrefix(strings.ToLower(trackID), "track:") {
		trackID = strings.TrimSpace(trackID[len("track:"):])
	}
	return vpsDraftTestTarget{TrackID: trackID, PluginID: strings.TrimSpace(request.PluginID)}
}

func vpsForgeStagingTargetMatchesDigest(target vpsDraftTestTarget, document vps.VPSDocument, digest pluginParameterDigest) error {
	if digest.TrackID != "" && !strings.EqualFold(strings.TrimSpace(digest.TrackID), target.TrackID) {
		return fmt.Errorf("fresh plug-in readback track does not match the selected Pro-Q 3 target; no write was attempted")
	}
	if digest.PluginID != "" && !strings.EqualFold(strings.TrimSpace(digest.PluginID), target.PluginID) {
		return fmt.Errorf("fresh plug-in readback instance does not match the selected Pro-Q 3 target; no write was attempted")
	}
	if !vpsRuntimeIdentityMatches(document.PluginIdentity, digest) {
		return fmt.Errorf("selected plug-in is not the configured FabFilter Pro-Q 3 staging VPS; no fallback Provider was selected")
	}
	return nil
}

func vpsForgeStagingInstructionResponse(conversationID string, goal agentruntime.Goal, config vpsForgeStagingEQConfig, target vpsDraftTestTarget, request spalEQV2Request, surface vpsDraftTestSurfaceCompatibility) (ChatResponse, bool) {
	if err := vpsForgeStagingActionSupported(config, request.Instruction); err != nil {
		return ChatResponse{
			ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
			Reply:    "The requested Pro-Q 3 staging action is outside its explicit feature matrix: " + err.Error() + ". It was not redirected to another EQ Provider.",
			Workflow: vpsForgeStagingEQWorkflow, GoalStatus: string(agentruntime.StatusWaitingClarification),
			WorkflowData: vpsForgeStagingBaseWorkflowData(config, target, request.Instruction, surface, map[string]any{
				"unsupported": true, "routing_eligible": false,
			}),
		}, false
	}
	return ChatResponse{}, true
}

/* Legacy Bell-only conversion retained in source history during the
transition to the contract-driven action compiler. It must never become a
second semantic execution path. */
/*
	unsupported := ""
	if request.Instruction.SchemaID != spal.EQBandPatchControlID {
		switch request.Instruction.SchemaID {
		case spal.EQPassFilterPatchControlID:
			unsupported = "High-pass/low-pass are not implemented in the current Pro-Q 3 staging slice. They are not redirected to another EQ Provider."
		case spal.EQOutputPatchControlID:
			unsupported = "Output/bypass controls are not implemented in the current Pro-Q 3 staging slice."
		default:
			unsupported = "This EQ operation is outside the current Pro-Q 3 staging slice."
		}
	} else if !strings.EqualFold(strings.TrimSpace(request.Instruction.StringParameters["band_ref"]), "b1") {
		unsupported = "Only Pro-Q 3 Band 1 is implemented in the current staging slice; multi-band allocation is not yet available."
	} else if !strings.EqualFold(strings.TrimSpace(request.Instruction.StringParameters["response_shape"]), "bell") {
		unsupported = "Only the user-witnessed Band 1 Bell action is implemented in the current staging slice; this response shape remains unsupported here."
	} else if request.Instruction.Parameters["enabled"] < 0.5 {
		unsupported = "Disabling an existing Pro-Q 3 band is outside this temporary staging action."
	}
	if unsupported == "" {
		return ChatResponse{}, true
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: unsupported,
		Workflow: vpsForgeStagingEQWorkflow, GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: vpsForgeStagingBaseWorkflowData(config, target, request.Instruction, surface, map[string]any{
			"unsupported": true, "routing_eligible": false,
		}),
	}, false
}

func vpsForgeStagingDesiredChanges(instruction spal.Instruction) ([]vpsForgeStagingDesiredChange, error) {
	if err := instruction.Validate(); err != nil {
		return nil, err
	}
	frequency, frequencyOK := instruction.Parameters["frequency_hz"]
	gain, gainOK := instruction.Parameters["gain_db"]
	q, qOK := instruction.Parameters["q"]
	if !frequencyOK || !gainOK || !qOK {
		return nil, fmt.Errorf("frequency_hz, gain_db and q are required for the Pro-Q 3 staging Bell action")
	}
	if !vpsForgeStagingFinite(frequency) || frequency < vpsForgeProQ3FrequencyMinHz || frequency > vpsForgeProQ3FrequencyMaxHz {
		return nil, fmt.Errorf("frequency_hz must be within the observed staging range %.0f–%.0f Hz", vpsForgeProQ3FrequencyMinHz, vpsForgeProQ3FrequencyMaxHz)
	}
	if !vpsForgeStagingFinite(gain) || gain < vpsForgeProQ3GainMinDB || gain > vpsForgeProQ3GainMaxDB {
		return nil, fmt.Errorf("gain_db must be within the observed staging range %.0f–%.0f dB", vpsForgeProQ3GainMinDB, vpsForgeProQ3GainMaxDB)
	}
	if !vpsForgeStagingFinite(q) || q < vpsForgeProQ3QMin || q > vpsForgeProQ3QMax {
		return nil, fmt.Errorf("q must be within the observed staging range %.3f–%.0f", vpsForgeProQ3QMin, vpsForgeProQ3QMax)
	}
	frequencyNormalized, err := vpsForgeStagingLogNormalized(frequency, vpsForgeProQ3FrequencyMinHz, vpsForgeProQ3FrequencyMaxHz)
	if err != nil {
		return nil, err
	}
	qNormalized, err := vpsForgeStagingLogNormalized(q, vpsForgeProQ3QMin, vpsForgeProQ3QMax)
	if err != nil {
		return nil, err
	}
	gainNormalized := (gain - vpsForgeProQ3GainMinDB) / (vpsForgeProQ3GainMaxDB - vpsForgeProQ3GainMinDB)
	return []vpsForgeStagingDesiredChange{
		{MappingKey: "pro_q_3_band_1_staging.used", RequestedNormalized: 1},
		{MappingKey: "pro_q_3_band_1_staging.enabled", RequestedNormalized: 1},
		{MappingKey: "pro_q_3_band_1_staging.shape", RequestedNormalized: 0}, // observed Bell enum value
		{MappingKey: "pro_q_3_band_1_staging.frequency_normalized", RequestedNormalized: frequencyNormalized},
		{MappingKey: "pro_q_3_band_1_staging.gain_normalized", RequestedNormalized: gainNormalized},
		{MappingKey: "pro_q_3_band_1_staging.q_normalized", RequestedNormalized: qNormalized},
	}, nil
}

func vpsForgeStagingFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func vpsForgeStagingLogNormalized(value, minimum, maximum float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || minimum <= 0 || maximum <= minimum {
		return 0, fmt.Errorf("invalid observed logarithmic staging domain")
	}
	result := math.Log(value/minimum) / math.Log(maximum/minimum)
	if !vpsDraftTestNormalizedValueValid(result) {
		return 0, fmt.Errorf("value is outside the observed logarithmic staging domain")
	}
	return result, nil
}

*/

// vpsForgeStagingActionSupported reads the fixed equalizer.v2 action grammar
// and this VPS's feature matrix. It intentionally cannot fall back from a
// missing task-card feature to another plug-in or a parent category.
func vpsForgeStagingActionSupported(config vpsForgeStagingEQConfig, instruction spal.Instruction) error {
	if err := instruction.Validate(); err != nil {
		return err
	}
	contract, known := vps.BadgeContractFor(vps.EqualizerCapabilityID)
	if !known {
		return fmt.Errorf("equalizer.v2 badge contract is unavailable")
	}
	actionID := ""
	for _, action := range contract.Actions {
		if action.SchemaID == instruction.SchemaID {
			actionID = action.ID
			break
		}
	}
	if actionID == "" {
		return fmt.Errorf("no equalizer.v2 action contract exists for %s", instruction.SchemaID)
	}
	var implementation *vps.VPSActionImplementation
	for index := range config.Artifact.BadgeActionImplementations {
		candidate := &config.Artifact.BadgeActionImplementations[index]
		if candidate.BadgeID == vps.EqualizerCapabilityID && candidate.ActionID == actionID && candidate.SchemaID == instruction.SchemaID {
			implementation = candidate
			break
		}
	}
	if implementation == nil {
		return fmt.Errorf("action %s has no plugin implementation", actionID)
	}
	required := vpsForgeStagingRequiredFeatures(instruction)
	if !implementation.SupportsRequiredFeatures(required, vps.BadgeFeatureStatusStagingReady) {
		return fmt.Errorf("action %s does not provide required features %s at staging readiness", actionID, strings.Join(required, ", "))
	}
	return nil
}

func vpsForgeStagingRequiredFeatures(instruction spal.Instruction) []string {
	switch instruction.SchemaID {
	case spal.EQBandPatchControlID:
		return []string{"static_band", strings.ToLower(strings.TrimSpace(instruction.StringParameters["response_shape"]))}
	case spal.EQPassFilterPatchControlID:
		slope := strconv.FormatFloat(instruction.Parameters["slope_db_per_octave"], 'f', -1, 64)
		return []string{strings.ToLower(strings.TrimSpace(instruction.StringParameters["filter_kind"])), "slope_" + slope + "_db_per_octave"}
	}
	return nil
}

// vpsForgeStagingCompiledDesiredChanges is the only staging semantic-to-host
// conversion path. It uses the shared EQ v2 compiler and then verifies that
// every physical parameter is represented by an explicitly approved staging
// mapping in the isolated VPS.
func vpsForgeStagingCompiledDesiredChanges(config vpsForgeStagingEQConfig, target vpsDraftTestTarget, instruction spal.Instruction) ([]vpsForgeStagingDesiredChange, error) {
	if err := vpsForgeStagingActionSupported(config, instruction); err != nil {
		return nil, err
	}
	if config.Adapter == nil {
		return nil, fmt.Errorf("staging action compiler is unavailable")
	}
	descriptor := config.Adapter.Descriptor()
	binding, err := config.Adapter.Bind(spal.ProviderInstance{
		ID:         "vpsforge_staging:" + config.Document.ID + ":" + target.TrackID + ":" + target.PluginID,
		ProviderID: descriptor.ID, TargetRef: "track:" + target.TrackID, TrackID: target.TrackID, PluginID: target.PluginID,
		Status: spal.ProviderCandidate, Metadata: map[string]string{"vpsforge_staging": "true", "vps_id": config.Document.ID},
	}, instruction)
	if err != nil {
		return nil, err
	}
	writes, err := config.Adapter.Compile(instruction, binding)
	if err != nil {
		return nil, err
	}
	mappings := map[string]vps.ControlSurfaceMapping{}
	for _, mapping := range config.Document.ControlSurface.Mappings {
		if vpsDraftTestOnlyMapping(mapping) {
			mappings[mapping.ParameterID] = mapping
		}
	}
	changes := make([]vpsForgeStagingDesiredChange, 0, len(writes))
	seen := map[string]bool{}
	for _, write := range writes {
		mapping, found := mappings[write.ParameterID]
		if !found {
			return nil, fmt.Errorf("action compiler references parameter %s without an approved staging mapping", write.ParameterID)
		}
		key := vpsDraftTestMappingKey(mapping)
		if key == "" || seen[key] || !vpsDraftTestNormalizedValueValid(write.Value) {
			return nil, fmt.Errorf("action compiler produced an invalid staging write for parameter %s", write.ParameterID)
		}
		seen[key] = true
		changes = append(changes, vpsForgeStagingDesiredChange{MappingKey: key, RequestedNormalized: write.Value})
	}
	return changes, nil
}

func cloneSPALInstruction(input spal.Instruction) spal.Instruction {
	copy := input
	copy.Parameters = cloneFloatMap(input.Parameters)
	copy.StringParameters = cloneStringMap(input.StringParameters)
	copy.EvidenceRefs = append([]string(nil), input.EvidenceRefs...)
	return copy
}

func vpsForgeStagingProposalPresentation(planID string, target vpsDraftTestTarget, instruction spal.Instruction, desired []vpsForgeStagingDesiredChange, surface vpsDraftTestSurfaceCompatibility) *orchestration.ProposalPresentation {
	conclusion := "Temporarily apply the explicitly selected staging EQ action on track " + target.TrackID + ", then restore the complete preimage."
	actions := []orchestration.ProposalActionPreview{}
	if instruction.SchemaID == spal.EQBandPatchControlID {
		frequency, gain, q := instruction.Parameters["frequency_hz"], instruction.Parameters["gain_db"], instruction.Parameters["q"]
		conclusion = fmt.Sprintf("Temporarily set Pro-Q 3 Band 1 to %s, %.1f Hz, %+.2f dB, Q %.3f on track %s, then restore the complete preimage.", instruction.StringParameters["response_shape"], frequency, gain, q, target.TrackID)
		actions = append(actions,
			orchestration.ProposalActionPreview{ActionID: "pro_q_3_band_1_frequency", TrackID: target.TrackID, Operation: "set_staging_frequency", Target: frequency, Unit: "Hz"},
			orchestration.ProposalActionPreview{ActionID: "pro_q_3_band_1_gain", TrackID: target.TrackID, Operation: "set_staging_gain", Target: gain, Unit: "dB"},
			orchestration.ProposalActionPreview{ActionID: "pro_q_3_band_1_q", TrackID: target.TrackID, Operation: "set_staging_q", Target: q, Unit: "Q"},
		)
	} else if instruction.SchemaID == spal.EQPassFilterPatchControlID {
		cutoff, slope := instruction.Parameters["cutoff_frequency_hz"], instruction.Parameters["slope_db_per_octave"]
		conclusion = fmt.Sprintf("Temporarily set Pro-Q 3 Band 1 to %s at %.1f Hz / %.0f dB/oct on track %s, then restore the complete preimage.", instruction.StringParameters["filter_kind"], cutoff, slope, target.TrackID)
		actions = append(actions,
			orchestration.ProposalActionPreview{ActionID: "pro_q_3_pass_filter_cutoff", TrackID: target.TrackID, Operation: "set_staging_pass_filter_cutoff", Target: cutoff, Unit: "Hz"},
			orchestration.ProposalActionPreview{ActionID: "pro_q_3_pass_filter_slope", TrackID: target.TrackID, Operation: "set_staging_pass_filter_slope", Target: slope, Unit: "dB/oct"},
		)
	}
	limitations := []string{
		"This is an isolated VPS Forge staging transaction, not a verified Provider or SPAL route.",
		"Only action-matrix rows explicitly marked staging_ready can run; no parent category or parameter label creates an implicit route.",
		"After fresh readback, the complete normalized parameter preimage is restored and verified; the requested setting is not retained as a production edit.",
	}
	if vpsForgeStagingIsSlope24Audit(instruction) {
		limitations = append(limitations, "This is the one 24 dB/oct audit candidate. The state remains visible for 30 seconds so the user can confirm the plug-in display, then it is rolled back automatically.")
	}
	if surface.Status == vpsDraftTestSurfaceCompatibilityMismatchUnacknowledged {
		limitations = append(limitations, "The current parameter-surface fingerprint differs from the staging observation. Confirming authorizes this one temporary staging test only; it does not update any fingerprint, Credential, Catalog or SPAL route.")
	}
	return &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: planID, ProposalRevision: 1,
		CapabilityID: vpsForgeStagingEQCapabilityID, Title: "Pro-Q 3 staging EQ transport test",
		Conclusion: conclusion,
		AnalysisSummary: []string{
			"The target identity and complete parameter surface were freshly read from the selected plug-in instance.",
			"Requested physical values use observed local Pro-Q 3 display domains only; they remain staging implementation data, not conformed semantics.",
			"Every changed host parameter receives a fresh readback; full rollback is verified before success is reported.",
		},
		Recommendation: "Confirm only to run this reversible staging transaction.", AnalyzedTracks: 1, ActionCount: len(desired), Risk: "staging_only", Reversible: true,
		Actions:        actions,
		Limitations:    limitations,
		EvidenceRefs:   []string{"vpsforge.staging_vps:pro_q_3_static_eq", "vpsforge.staging_surface:observed"},
		ApprovalPrompt: "确认后执行一次 staging 写入/readback/完整 rollback；取消则不写入插件。",
	}
}

func vpsForgeStagingInspectionResponse(conversationID string, goal agentruntime.Goal, config vpsForgeStagingEQConfig, target vpsDraftTestTarget, instruction spal.Instruction, surface vpsDraftTestSurfaceCompatibility) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "The selected Pro-Q 3 instance matches the explicitly configured VPS Forge staging slice. Inspect mode performed no parameter write.",
		Workflow: vpsForgeStagingEQWorkflow, GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: vpsForgeStagingBaseWorkflowData(config, target, instruction, surface, map[string]any{"inspection_only": true}),
	}
}

func vpsForgeStagingBaseWorkflowData(config vpsForgeStagingEQConfig, target vpsDraftTestTarget, instruction spal.Instruction, surface vpsDraftTestSurfaceCompatibility, extra map[string]any) map[string]any {
	data := map[string]any{
		"execution_route":            vpsForgeStagingEQWorkflow,
		"source":                     vpsForgeStagingEQSource,
		"routing_eligible":           false,
		"catalog_visible":            false,
		"credential_issuance":        false,
		"spal_dispatch":              false,
		"vps_id":                     config.Artifact.VPSID,
		"vps_revision":               config.Artifact.VPSRevision,
		"target":                     target,
		"semantic_action":            instructionMap(instruction),
		"surface_compatibility":      surface.Status,
		"expected_parameter_surface": surface.ExpectedParameterSurface,
		"observed_parameter_surface": surface.ObservedParameterSurface,
	}
	for key, value := range extra {
		data[key] = value
	}
	return data
}

func vpsForgeStagingBlockedResponse(conversationID string, goal agentruntime.Goal, message string, data map[string]any) ChatResponse {
	if data == nil {
		data = map[string]any{}
	}
	data["execution_route"] = vpsForgeStagingEQWorkflow
	data["source"] = vpsForgeStagingEQSource
	data["routing_eligible"] = false
	data["catalog_visible"] = false
	data["credential_issuance"] = false
	data["spal_dispatch"] = false
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: message, Workflow: vpsForgeStagingEQWorkflow, GoalStatus: string(agentruntime.StatusWaitingClarification), WorkflowData: data}
}

func vpsForgeStagingClarificationResponse(conversationID string, goal agentruntime.Goal, missing []string) ChatResponse {
	return vpsForgeStagingBlockedResponse(conversationID, goal, "The Pro-Q 3 staging bridge needs: "+strings.Join(missing, ", ")+". No parameter write was attempted.", map[string]any{"missing_fields": missing})
}

func vpsForgeStagingOperationMap(operation vpsForgeStagingEQOperation) (map[string]any, error) {
	payload, err := json.Marshal(operation)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func vpsForgeStagingOperationFromPlan(plan PendingPlan) (vpsForgeStagingEQOperation, error) {
	data := mapValue(plan.WorkflowData["operation"])
	if len(data) == 0 {
		return vpsForgeStagingEQOperation{}, fmt.Errorf("staging proposal has no durable operation data")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return vpsForgeStagingEQOperation{}, err
	}
	var operation vpsForgeStagingEQOperation
	if err := json.Unmarshal(payload, &operation); err != nil {
		return vpsForgeStagingEQOperation{}, fmt.Errorf("decode staging proposal: %w", err)
	}
	if operation.SchemaVersion != vpsForgeStagingEQOperationSchema || operation.VPSID == "" || operation.VPSRevision < 1 || len(operation.DesiredChanges) == 0 {
		return vpsForgeStagingEQOperation{}, fmt.Errorf("staging proposal is incomplete or incompatible")
	}
	if err := operation.Target.valid(); err != nil {
		return vpsForgeStagingEQOperation{}, err
	}
	return operation, nil
}

func vpsForgeStagingRequestContext(vpsID, stage string) map[string]any {
	context := vpsDraftTestRequestContext(vpsID, "pro_q_3_band_1_staging", stage)
	context["vpsforge_staging_bridge"] = true
	context["source"] = vpsForgeStagingEQSource
	return context
}

// resolveVPSForgeStagingEQPlan is called only after the normal Agent
// confirmation has consumed its PendingPlan.  It re-reads every guard rather
// than trusting proposal-time state, then delegates the actual transaction to
// the existing Draft executor for full snapshot rollback.
func (s *Server) resolveVPSForgeStagingEQPlan(ctx context.Context, planID string, plan PendingPlan) (int, map[string]any) {
	goalID, runID := goalIDsFromContext(plan.Context)
	projectPath := projectPathFromChatContext(plan.Context)
	agentMode := agentModeFromContext(plan.Context)
	summary, message, executeErr := s.executeVPSForgeStagingEQPlan(ctx, plan)
	goalStatus := agentruntime.StatusCompleted
	status := "ok"
	if executeErr != nil {
		goalStatus = agentruntime.StatusFailed
		status = "error"
		message = firstNonEmpty(message, "VPS Forge staging transaction failed: "+executeErr.Error())
		s.harness.CompleteGoal(goalID, executeErr)
	} else {
		s.harness.CompleteGoal(goalID, nil)
	}
	projectHistory := s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, "vit", message, goalID, runID, map[string]any{
		"vpsforge_staging": summary,
	})
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
	}
	workflowData := map[string]any{
		"execution_route":     vpsForgeStagingEQWorkflow,
		"source":              vpsForgeStagingEQSource,
		"routing_eligible":    false,
		"catalog_visible":     false,
		"credential_issuance": false,
		"spal_dispatch":       false,
		"execution":           summary,
	}
	if value := cleanContextText(summary["vps_id"]); value != "" {
		workflowData["vps_id"] = value
	}
	if value := summary["vps_revision"]; value != nil {
		workflowData["vps_revision"] = value
	}
	if value := cleanContextText(summary["report_path"]); value != "" {
		workflowData["report_path"] = value
	}
	response := map[string]any{
		"status":            status,
		"message":           message,
		"reply":             message,
		"plan_id":           planID,
		"goal_id":           goalID,
		"run_id":            runID,
		"agent_mode":        agentMode,
		"goal_status":       string(goalStatus),
		"workflow":          vpsForgeStagingEQWorkflow,
		"workflow_data":     workflowData,
		"staging_execution": summary,
		"project_history":   projectHistory,
		"typed_events":      typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, map[bool]string{true: "denied", false: "consumed"}[executeErr != nil], firstNonEmpty(errorString(executeErr), message)),
	}
	if agentPlan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, goalStatus, "", "", errorString(executeErr), projectHistory)); agentPlan != nil {
		response["agent_plan"] = agentPlan
	}
	return httpStatusForVPSForgeStagingExecution(executeErr), response
}

func httpStatusForVPSForgeStagingExecution(_ error) int {
	// Confirmation endpoints report execution failures in their JSON envelope,
	// matching the existing PendingPlan contract.  A consumed confirmation is
	// never retried by an HTTP client just because the target plug-in rejected a
	// guarded staging transaction.
	return 200
}

func (s *Server) executeVPSForgeStagingEQPlan(ctx context.Context, plan PendingPlan) (map[string]any, string, error) {
	operation, err := vpsForgeStagingOperationFromPlan(plan)
	if err != nil {
		return vpsForgeStagingExecutionBase(nil), "The staging proposal is invalid; no parameter write was attempted.", err
	}
	config, enabled, configErr := s.loadVPSForgeStagingEQConfig()
	if !enabled || configErr != nil {
		if configErr == nil {
			configErr = fmt.Errorf("%s is no longer configured", vpsForgeStagingVPSPathEnv)
		}
		return vpsForgeStagingExecutionBase(nil), "The VPS Forge staging bridge is unavailable; no parameter write was attempted.", configErr
	}
	if operation.ArtifactPath != config.ArtifactPath || operation.ArtifactSHA256 != config.ArtifactHash || operation.VPSID != config.Artifact.VPSID || operation.VPSRevision != config.Artifact.VPSRevision {
		return vpsForgeStagingExecutionBase(&config), "The staging artifact changed after the proposal; no parameter write was attempted.", fmt.Errorf("staging artifact or VPS revision changed after proposal")
	}
	expectedDesired, desiredErr := vpsForgeStagingCompiledDesiredChanges(config, operation.Target, operation.Instruction)
	if desiredErr != nil || !vpsForgeStagingDesiredChangesMatch(operation.DesiredChanges, expectedDesired) {
		if desiredErr == nil {
			desiredErr = fmt.Errorf("staging proposal values do not match its requested EQ action")
		}
		return vpsForgeStagingExecutionBase(&config), "The staging proposal changed after it was shown; no parameter write was attempted.", desiredErr
	}
	fresh, readErr := s.readVPSDraftTestParameterSurface(ctx, operation.Target, vpsForgeStagingRequestContext(config.Artifact.VPSID, "confirm_prepare"))
	if readErr != nil {
		return vpsForgeStagingExecutionBase(&config), "The selected Pro-Q 3 surface could not be freshly read; no parameter write was attempted.", readErr
	}
	digest := buildPluginParameterDigest(fresh)
	if matchErr := vpsForgeStagingTargetMatchesDigest(operation.Target, config.Document, digest); matchErr != nil {
		return vpsForgeStagingExecutionBase(&config), "The selected plug-in no longer matches the staging proposal; no parameter write was attempted.", matchErr
	}
	acknowledgment := ""
	if operation.SurfaceMismatchAcknowledgedOnUse {
		acknowledgment = vpsDraftTestSurfaceMismatchAcknowledgmentPhrase
	}
	surface, surfaceErr := vpsDraftTestValidateLiveSurface(config.Document, digest, acknowledgment)
	if surfaceErr != nil {
		return vpsForgeStagingExecutionBase(&config), "The fresh Pro-Q 3 surface no longer satisfies the staging guard; no parameter write was attempted.", surfaceErr
	}
	wantedSurfaceStatus := operation.PlannedSurfaceCompatibility
	if operation.SurfaceMismatchAcknowledgedOnUse {
		wantedSurfaceStatus = vpsDraftTestSurfaceCompatibilityMismatchAcknowledged
	}
	if surface.Status != wantedSurfaceStatus || !strings.EqualFold(surface.ExpectedParameterSurface, operation.ExpectedParameterSurface) || !strings.EqualFold(surface.ObservedParameterSurface, operation.ObservedParameterSurface) {
		return vpsForgeStagingExecutionBase(&config), "The Pro-Q 3 parameter surface changed after the proposal; no parameter write was attempted.", fmt.Errorf("staging parameter-surface guard changed after proposal")
	}
	mappingKeys := make([]string, 0, len(operation.DesiredChanges))
	for _, desired := range operation.DesiredChanges {
		mappingKeys = append(mappingKeys, desired.MappingKey)
	}
	library, document, mappings, resolveErr := vpsDraftTestResolveMappingKeysFromLibrary(config.Library, operation.VPSID, mappingKeys)
	if resolveErr != nil {
		return vpsForgeStagingExecutionBase(&config), "The staging mappings are unavailable; no parameter write was attempted.", resolveErr
	}
	if library.Path() == "" || document.Revision != operation.VPSRevision || document.ID != operation.VPSID || document.Status != vps.VPSStatusDraft {
		return vpsForgeStagingExecutionBase(&config), "The staging VPS changed after the proposal; no parameter write was attempted.", fmt.Errorf("staging VPS changed after proposal")
	}
	if err := vpsForgeStagingDocumentMappingsValid(document); err != nil {
		return vpsForgeStagingExecutionBase(&config), "The staging VPS no longer exposes its approved staging slice; no parameter write was attempted.", err
	}
	reportRoot := vpsDraftTestReportRoot(library.Path())
	if err := vpsDraftTestEnsureReportRoot(reportRoot); err != nil {
		return vpsForgeStagingExecutionBase(&config), "The staging evidence store is unavailable; no parameter write was attempted.", err
	}
	for _, mapping := range mappings {
		if rejected, rejectedErr := vpsDraftTestMappingRejected(reportRoot, document.ID, document.Revision, vpsDraftTestMappingKey(mapping)); rejectedErr != nil {
			return vpsForgeStagingExecutionBase(&config), "The staging disposition ledger could not be read; no parameter write was attempted.", rejectedErr
		} else if rejected {
			return vpsForgeStagingExecutionBase(&config), "This staging mapping was previously rejected for the current VPS revision; no parameter write was attempted.", fmt.Errorf("staging mapping %s was rejected for this VPS revision", vpsDraftTestMappingKey(mapping))
		}
	}
	parameterIndex := vpsParameterIndex(digest)
	changes := make([]vpsDraftTestChange, 0, len(operation.DesiredChanges))
	for index, desired := range operation.DesiredChanges {
		mapping := mappings[index]
		if vpsDraftTestMappingKey(mapping) != desired.MappingKey || !vpsDraftTestOnlyMapping(mapping) {
			return vpsForgeStagingExecutionBase(&config), "The staging mapping changed after the proposal; no parameter write was attempted.", fmt.Errorf("staging mapping %s changed after proposal", desired.MappingKey)
		}
		parameter, found := parameterIndex[mapping.ParameterID]
		if !found || !parameter.HostControllable {
			return vpsForgeStagingExecutionBase(&config), "A required Pro-Q 3 parameter is unavailable or not host-controllable; no parameter write was attempted.", fmt.Errorf("staging parameter %s is absent or not host-controllable", mapping.ParameterID)
		}
		current, finite := vpsFiniteNumber(parameter.NormalizedValue)
		if !finite {
			return vpsForgeStagingExecutionBase(&config), "A required Pro-Q 3 parameter has no fresh normalized readback; no parameter write was attempted.", fmt.Errorf("staging parameter %s has no finite normalized readback", mapping.ParameterID)
		}
		if !vpsNormalizedValuesMatch(current, desired.RequestedNormalized) {
			changes = append(changes, vpsDraftTestChange{MappingKey: desired.MappingKey, ParameterID: mapping.ParameterID, RequestedNormalized: desired.RequestedNormalized})
		}
	}
	if len(changes) == 0 {
		result := vpsForgeStagingExecutionBase(&config)
		result["execution_status"] = "already_requested_state"
		result["target"] = operation.Target
		result["writes_started"] = false
		result["surface_compatibility"] = surface.Status
		return result, "The selected Pro-Q 3 Band 1 already matches the requested staging values. No parameter write was needed.", nil
	}
	ticket := vpsDraftTestTicket{
		ID: "vpsforge_staging_ticket_" + randomID(), VPSID: document.ID, VPSRevision: document.Revision,
		Target: operation.Target, MappingKey: changes[0].MappingKey, ParameterID: changes[0].ParameterID,
		RequestedNormalized: changes[0].RequestedNormalized, ObservationMode: vpsDraftTestObservationModeTimedRollback,
		ScenarioID: "vpsforge_pro_q_3_band_1_bell_agent_bridge", Changes: changes, ReportRoot: reportRoot,
		LibraryPath:          library.Path(),
		SurfaceCompatibility: surface, CreatedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(vpsDraftTestTicketTTL),
	}
	if vpsForgeStagingIsSlope24Audit(operation.Instruction) {
		// This one explicitly authorised audit remains visible long enough for
		// the user to verify the plug-in display. It still auto-rolls back the
		// complete preimage so it cannot accidentally become a production edit.
		ticket.Hold = 30 * time.Second
		ticket.ScenarioID = "vpsforge_pro_q_3_band_1_highpass_24_db_per_octave_audit"
	}
	s.vpsDraftTestRunMu.Lock()
	report, runErr := s.runVPSDraftTest(ctx, ticket)
	s.vpsDraftTestRunMu.Unlock()
	reportPath, persistErr := vpsDraftTestPersistReport(reportRoot, report)
	result := vpsForgeStagingExecutionBase(&config)
	result["execution_status"] = report.Status
	result["target"] = operation.Target
	result["requested_changes"] = changes
	result["surface_compatibility"] = report.SurfaceCompatibility
	result["report_path"] = reportPath
	result["report"] = vpsForgeStagingReportSummary(report)
	if persistErr != nil {
		return result, "The staging transaction finished but its bounded evidence report could not be persisted.", fmt.Errorf("persist staging transaction report: %w", persistErr)
	}
	if runErr != nil {
		return result, "The Pro-Q 3 staging transaction failed; the executor attempted its mandatory rollback before reporting failure.", runErr
	}
	return result, "Pro-Q 3 staging write/readback and complete rollback passed. The result remains observed staging transport evidence; no Credential, Catalog entry or SPAL route was created.", nil
}

func vpsForgeStagingIsSlope24Audit(instruction spal.Instruction) bool {
	return instruction.SchemaID == spal.EQPassFilterPatchControlID &&
		strings.EqualFold(strings.TrimSpace(instruction.StringParameters["filter_kind"]), "highpass") &&
		math.Abs(instruction.Parameters["slope_db_per_octave"]-24) < 0.000001
}

func vpsForgeStagingExecutionBase(config *vpsForgeStagingEQConfig) map[string]any {
	result := map[string]any{
		"source":              vpsForgeStagingEQSource,
		"execution_route":     vpsForgeStagingEQWorkflow,
		"routing_eligible":    false,
		"catalog_visible":     false,
		"credential_issuance": false,
		"spal_dispatch":       false,
	}
	if config != nil {
		result["vps_id"] = config.Artifact.VPSID
		result["vps_revision"] = config.Artifact.VPSRevision
	}
	return result
}

func vpsForgeStagingInstructionViolation(instruction spal.Instruction) string {
	if instruction.SchemaID != spal.EQBandPatchControlID {
		return "only the static EQ Band patch schema is supported"
	}
	if !strings.EqualFold(strings.TrimSpace(instruction.StringParameters["band_ref"]), "b1") {
		return "only Band 1 is supported"
	}
	if !strings.EqualFold(strings.TrimSpace(instruction.StringParameters["response_shape"]), "bell") {
		return "only Bell is supported"
	}
	if instruction.Parameters["enabled"] < 0.5 {
		return "the staging bridge only supports an enabled Bell band"
	}
	return ""
}

func vpsForgeStagingDesiredChangesMatch(left, right []vpsForgeStagingDesiredChange) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].MappingKey != right[index].MappingKey || !vpsNormalizedValuesMatch(left[index].RequestedNormalized, right[index].RequestedNormalized) {
			return false
		}
	}
	return true
}

func vpsForgeStagingReportSummary(report vpsDraftTestReport) map[string]any {
	result := map[string]any{
		"run_id": report.RunID, "evidence_id": report.EvidenceID, "status": report.Status,
		"trust": report.Trust, "test_scope": report.TestScope, "vps_id": report.VPSID,
		"vps_revision": report.VPSRevision, "observation_mode": report.ObservationMode,
		"surface_compatibility": report.SurfaceCompatibility, "failure_code": report.FailureCode,
		"write_readback": map[string]any{
			"passed":        report.WriteReadback.Passed,
			"receipts":      vpsForgeStagingReceiptSummary(report.WriteReadback.Receipts),
			"receipt_count": len(report.WriteReadback.Receipts),
		},
		"rollback": map[string]any{
			"status": report.Rollback.Status, "passed": report.Rollback.Passed, "rounds": report.Rollback.Rounds,
			"changed_parameter_ids":   vpsForgeStagingStringSummary(report.Rollback.ChangedParameterIDs),
			"changed_parameter_count": len(report.Rollback.ChangedParameterIDs),
			"receipts":                vpsForgeStagingReceiptSummary(report.Rollback.Receipts),
			"receipt_count":           len(report.Rollback.Receipts),
		},
	}
	if report.Error != "" {
		result["error"] = report.Error
	}
	return result
}

func vpsForgeStagingReceiptSummary(receipts []vpsDraftTestReceipt) []vpsDraftTestReceipt {
	const maximum = 32
	if len(receipts) <= maximum {
		return append([]vpsDraftTestReceipt(nil), receipts...)
	}
	return append([]vpsDraftTestReceipt(nil), receipts[:maximum]...)
}

func vpsForgeStagingStringSummary(values []string) []string {
	const maximum = 32
	if len(values) <= maximum {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:maximum]...)
}
