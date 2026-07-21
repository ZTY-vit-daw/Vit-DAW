package vpsforge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const (
	proQ3StaticEQStagingVPSSchema        = "vit.vpsforge.pro_q_3_static_eq_staging_vps.v1"
	proQ3StaticEQStagingVPSEvidenceDir   = "evidence/pro_q_3_static_eq_staging_vps"
	proQ3StaticEQStagingVPSRuntimeFile   = "staging_vps_runtime/vps_library_v3.json"
	proQ3StaticEQStagingActivationFile   = "active_vpsforge_staging_runtime.json"
	proQ3StaticEQStagingActivationSchema = "vit.vpsforge.staging_activation.v1"
	proQ3StaticEQStagingBindingStatus    = "user-confirmed_staging_executable"
	proQ3StaticEQStagingExecutionScope   = "forge_staging_actual_test"
)

// ProQ3StaticEQStagingVPSRequest creates the directly usable first Pro-Q 3
// VPS implementation. It intentionally stops at the already witnessed static
// Band 1 surface; it neither grants the broad equalizer.v2 badge nor enters
// Credential, Catalog or SPAL routing.
type ProQ3StaticEQStagingVPSRequest struct {
	Root string
	Now  time.Time
}

type ProQ3StaticEQStagingVPSResult struct {
	Workspace      string `json:"workspace"`
	Status         string `json:"status"`
	VPSArtifact    string `json:"vps_artifact"`
	RuntimeLibrary string `json:"runtime_library"`
	RunArtifact    string `json:"run_artifact"`
	ActivationFile string `json:"activation_file"`
}

// proQ3StaticEQStagingBinding is the reviewed, bounded physical realization
// of the two equalizer.v2 actions for which evidence exists. It deliberately
// stays outside Credential conformance: the artifact declares its implemented
// schemas separately, and only the explicit staging resolver may compile it.
//
// The 24 dB/oct enum value below is an agent-inferred *audit candidate*
// derived from the observed discrete slope surface (12 dB/oct = 1/9). It
// is usable only in the one staging audit below, which requires a fresh
// GUI/display witness and complete rollback. It is not conformed and is not
// exposed to Catalog or ordinary SPAL routing.
func proQ3StaticEQStagingBinding() (spal.EQV2Binding, []string) {
	linear := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "linear"}
	}
	log := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "log"}
	}
	allocated := linear("0", "toggle", 0, 1)
	band := spal.EQV2BandBinding{
		ComponentID: "b1", Allocated: &allocated, Enabled: linear("1", "toggle", 0, 1),
		ResponseShape: spal.EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"bell": 0}},
		FrequencyHz:   log("2", "Hz", 10, 30000), GainDB: linear("3", "dB", -30, 30), Q: log("7", "Q", .025, 40),
	}
	shape := spal.EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"highpass": .25, "lowpass": .5}}
	slope := spal.EnumParameterBinding{ParameterID: "9", Values: map[string]float64{"12": 1.0 / 9.0, "24": 3.0 / 9.0}}
	highPass := &spal.EQV2PassFilterBinding{
		ComponentID: "b1", Allocated: &allocated, ResponseShape: &shape, Enabled: linear("1", "toggle", 0, 1),
		CutoffFrequencyHz: log("2", "Hz", 10, 30000), SlopeDBPerOctave: slope,
	}
	lowPass := &spal.EQV2PassFilterBinding{
		ComponentID: "b1", Allocated: &allocated, ResponseShape: &shape, Enabled: linear("1", "toggle", 0, 1),
		CutoffFrequencyHz: log("2", "Hz", 10, 30000), SlopeDBPerOctave: slope,
	}
	return spal.EQV2Binding{Bands: map[string]spal.EQV2BandBinding{"b1": band}, HighPass: highPass, LowPass: lowPass}, []string{spal.EQBandPatchControlID, spal.EQPassFilterPatchControlID}
}

func proQ3StaticEQStagingActionImplementations() []vps.VPSActionImplementation {
	const bindingRef = "spal.eq_v2.staging_binding"
	return []vps.VPSActionImplementation{
		{
			BadgeID: vps.EqualizerCapabilityID, BadgeVersion: vps.EqualizerProfileVersion,
			ActionID: "eq.static_band.patch", SchemaID: spal.EQBandPatchControlID, Status: vps.BadgeFeatureStatusStagingReady, BindingRef: bindingRef,
			Features: []vps.BadgeFeatureMatrixEntry{
				{ActionID: "eq.static_band.patch", FeatureID: "static_band", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + proQ3StaticLifecycleResultsFile}},
				{ActionID: "eq.static_band.patch", FeatureID: "bell", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + humanWitnessFile}},
			},
			EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + proQ3StaticLifecycleResultsFile, "vpsforge:" + humanWitnessFile},
		},
		{
			BadgeID: vps.EqualizerCapabilityID, BadgeVersion: vps.EqualizerProfileVersion,
			ActionID: "eq.pass_filter.patch", SchemaID: spal.EQPassFilterPatchControlID, Status: vps.BadgeFeatureStatusStagingReady, BindingRef: bindingRef,
			Features: []vps.BadgeFeatureMatrixEntry{
				{ActionID: "eq.pass_filter.patch", FeatureID: "highpass", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + humanWitnessFile}},
				{ActionID: "eq.pass_filter.patch", FeatureID: "lowpass", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + humanWitnessFile}},
				{ActionID: "eq.pass_filter.patch", FeatureID: "slope_12_db_per_octave", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile}},
				{ActionID: "eq.pass_filter.patch", FeatureID: "slope_24_db_per_octave", Status: vps.BadgeFeatureStatusStagingReady, EvidenceRefs: []string{"vpsforge:" + surfaceFile}}, // This row means only that an explicit, rollback-only audit can
				// test the inferred selector. It is upgraded neither to a
				// semantic claim nor to conformance by generating this VPS.

			},
			EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile, "vpsforge:" + humanWitnessFile},
		},
	}
}

// proQ3StaticEQSlopeAuditMapping deliberately does not use the parameter
// label as semantic authority. ID 9 was already captured as an observed,
// automatable member of the stable Pro-Q 3 surface; this returns a raw,
// test-only selector so the shared compiler can exercise the one 24 dB/oct
// audit candidate under full preimage/readback/rollback control.
func proQ3StaticEQSlopeAuditMapping(source vps.VPSDocument) (proQ3StaticEQDraftTestMapping, error) {
	for _, mapping := range source.ControlSurface.Mappings {
		if strings.TrimSpace(mapping.ParameterID) != "9" {
			continue
		}
		return proQ3StaticEQDraftTestMapping{
			Key:               "pro_q_3_band_1_staging.slope",
			ParameterID:       "9",
			ObservedLabel:     mapping.Label,
			BindingStatus:     proQ3StaticEQDraftTestBindingStatus,
			ExecutionScope:    proQ3StaticEQDraftTestExecutionScope,
			SelectionBoundary: "Agent-inferred raw selector for a one-time 24 dB/oct staging audit. It cannot grant task semantics, conformance, Credential or routing authority.",
		}, nil
	}
	return proQ3StaticEQDraftTestMapping{}, fmt.Errorf("observed Pro-Q 3 source VPS lacks parameter 9 for the bounded slope audit")
}

// WriteProQ3StaticEQStagingVPS uses the user-confirmed control relationships
// plus the observed surface to produce a real staging VPS that can be applied
// to a loaded Pro-Q 3 during actual use tests. The mappings stay explicitly
// non-conformed and are accepted only by the isolated staging execution path.
func WriteProQ3StaticEQStagingVPS(request ProQ3StaticEQStagingVPSRequest) (ProQ3StaticEQStagingVPSResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, fmt.Errorf("inspect Pro-Q 3 staging VPS workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return ProQ3StaticEQStagingVPSResult{}, fmt.Errorf("staging VPS only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	if err := proQ3StaticEQDraftTestRequiredArtifacts(root); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	var source vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &source); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, fmt.Errorf("read staged VPS Draft: %w", err)
	}
	if source.Status != vps.VPSStatusDraft || len(source.ProviderCredentials) != 0 {
		return ProQ3StaticEQStagingVPSResult{}, fmt.Errorf("staged Pro-Q 3 source must remain a credential-free Draft")
	}
	var core proQ3CoreEQProbeArtifact
	if err := readJSON(filepath.Join(root, proQ3CoreEQResultsFile), &core); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, fmt.Errorf("read observed Pro-Q 3 core probe: %w", err)
	}
	mappings, err := proQ3StaticEQDraftTestMappings(core.ObservedControls, core.ShapeDiscovery)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	slopeAudit, err := proQ3StaticEQSlopeAuditMapping(source)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	mappings = append(mappings, slopeAudit)
	for index := range mappings {
		mappings[index].Key = strings.Replace(mappings[index].Key, "pro_q_3_test_band_1", "pro_q_3_band_1_staging", 1)
		if mappings[index].ParameterID == slopeAudit.ParameterID {
			mappings[index].SelectionBoundary = "Agent-inferred 24 dB/oct slope selector for one explicit staging audit only. Fresh readback, user GUI/display confirmation and complete rollback are required; it is neither a conformed mapping nor a normal SPAL action."
			continue
		}
		mappings[index].BindingStatus = proQ3StaticEQStagingBindingStatus
		mappings[index].ExecutionScope = proQ3StaticEQStagingExecutionScope
		mappings[index].SelectionBoundary = "The user witnessed this Pro-Q 3 control relationship; its raw host selector remains observed. It is implemented only for explicit staging use and is not a Credential-backed generic EQ binding."
	}
	now := nowOrCurrent(request.Now)
	vpsID := stableID("vps_pro_q_3_static_eq_staging", status.Manifest.WorkspaceID, source.PluginIdentity.Fingerprint.ParameterSurface)
	candidate := proQ3StaticEQDraftTestDocument(source, mappings, now)
	if binding, found, bindingErr := proQ3StaticEQVitHostProjectionBinding(root, source.PluginIdentity); bindingErr != nil {
		return ProQ3StaticEQStagingVPSResult{}, bindingErr
	} else if found {
		// This is an observed Vit transport guard, not a replacement for the
		// independent Adapter identity held in candidate.PluginIdentity.
		candidate.HostProjectionBindings = []vps.HostProjectionBinding{binding}
	}
	candidate.BadgeActionImplementations = proQ3StaticEQStagingActionImplementations()
	candidate.ID = vpsID
	candidate.Revision = 1
	for index := range candidate.SemanticCapabilities {
		if strings.EqualFold(candidate.SemanticCapabilities[index].ID, "equalizer.v2") {
			candidate.SemanticCapabilities[index].Status = "user-confirmed_staging_implemented_not_conformed"
		}
	}
	for index := range candidate.ControlSurface.Mappings {
		mapping := &candidate.ControlSurface.Mappings[index]
		if mapping.BindingStatus == proQ3StaticEQStagingBindingStatus && mapping.ExecutionScope == proQ3StaticEQStagingExecutionScope {
			mapping.Warning = "User-confirmed Pro-Q 3 static-EQ staging mapping. It is implemented for actual tests, remains non-conformed, and cannot enter Credential, Catalog or SPAL routing."
		}
	}
	runtimeLibrary := filepath.Join(root, filepath.FromSlash(proQ3StaticEQStagingVPSRuntimeFile))
	stored, err := proQ3StaticEQStagingWriteLibrary(runtimeLibrary, candidate)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	candidate = stored
	profileID := stableID("pro_q_3_static_eq_staging_vps", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(proQ3StaticEQStagingVPSEvidenceDir, profileID+".json"))
	stagingBinding, implementedSchemas := proQ3StaticEQStagingBinding()
	artifact := map[string]any{
		"schema_version": proQ3StaticEQStagingVPSSchema,
		"trust":          "user-confirmed",
		"status":         "staging_vps_implemented_not_conformed",
		"captured_at":    now,
		"workspace_id":   status.Manifest.WorkspaceID,
		"vps_id":         candidate.ID,
		"vps_revision":   candidate.Revision,
		"run_artifact":   runArtifact,
		"authority_boundary": []string{
			"This is the implemented Pro-Q 3 static-EQ staging VPS, not a new preflight gate.",
			"It may be used in an explicit actual Vit test with fresh readback and rollback.",
			"It remains non-conformed, has no Credential, is absent from Catalog, and cannot be selected by SPAL.",
		},
		"implemented_mappings":         mappings,
		"badge_action_implementations": candidate.BadgeActionImplementations,
		"implemented_schemas":          implementedSchemas,
		"spal_eq_v2_staging_binding":   stagingBinding,
		"actual_test_scene": map[string]any{
			"id": "pro_q_3_band_1_bell_actual_test", "observation_mode": "manual_witness",
			"changes": []map[string]any{
				{"mapping_key": "pro_q_3_band_1_staging.used", "requested_normalized": 1.0},
				{"mapping_key": "pro_q_3_band_1_staging.enabled", "requested_normalized": 1.0},
				{"mapping_key": "pro_q_3_band_1_staging.frequency_normalized", "requested_normalized": 0.72},
				{"mapping_key": "pro_q_3_band_1_staging.gain_normalized", "requested_normalized": 0.625},
				{"mapping_key": "pro_q_3_band_1_staging.q_normalized", "requested_normalized": 0.5},
				{"mapping_key": "pro_q_3_band_1_staging.shape", "requested_normalized": 0.0},
			},
		},
		"action_audit_scene": map[string]any{
			"id": "pro_q_3_band_1_highpass_24_db_per_octave_audit", "observation_mode": "timed_rollback",
			"boundary": "The 24 dB/oct slope selector is agent-inferred for this single action audit. It remains observed/agent-inferred until a fresh GUI witness, behavior probe and rollback result are reviewed.",
			"changes": []map[string]any{
				{"mapping_key": "pro_q_3_band_1_staging.used", "requested_normalized": 1.0},
				{"mapping_key": "pro_q_3_band_1_staging.enabled", "requested_normalized": 1.0},
				{"mapping_key": "pro_q_3_band_1_staging.frequency_normalized", "requested_normalized": 0.575188457965851},
				{"mapping_key": "pro_q_3_band_1_staging.shape", "requested_normalized": 0.25},
				{"mapping_key": "pro_q_3_band_1_staging.slope", "requested_normalized": 3.0 / 9.0},
			},
		},
		"runtime": map[string]any{
			"library_path":              filepath.ToSlash(proQ3StaticEQStagingVPSRuntimeFile),
			"canonical_library_touched": false, "credential_issuable": false,
			"catalog_visible": false, "spal_dispatch": false,
		},
		"deferred": []string{
			"review of the one 24 dB/oct pass-filter staging audit",
			"Dynamic EQ, automatic/match EQ, analyzer and phase modes",
			"Credential, Catalog and SPAL routing",
		},
	}
	if err := writeJSON(filepath.Join(root, proQ3StaticEQStagingVPSFile), artifact); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	if err := writeJSON(filepath.Join(root, filepath.FromSlash(runArtifact)), artifact); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	ledgerData, _ := json.Marshal(map[string]any{"schema_version": proQ3StaticEQStagingVPSSchema, "status": artifact["status"], "vps_id": candidate.ID, "mapping_count": len(mappings), "credential_issuable": false, "catalog_visible": false, "spal_dispatch": false})
	if err := appendEvidence(root, EvidenceEntry{Kind: "pro_q_3_static_eq_staging_vps", Trust: "user-confirmed", Source: "vpsforge.pro_q_3_static_eq_staging_vps", Summary: "Implemented the user-witnessed Pro-Q 3 static-EQ staging VPS for actual tests. It remains non-conformed and non-routeable.", Artifact: runArtifact, CapturedAt: now, Data: ledgerData}); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	if err := updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["pro_q_3_static_eq_staging_vps"] = proQ3StaticEQStagingVPSFile
		manifest.UpdatedAt = now
	}); err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	activationPath, err := proQ3StaticEQWriteStagingActivation(root, proQ3StaticEQStagingVPSFile, candidate.ID, candidate.Revision, now)
	if err != nil {
		return ProQ3StaticEQStagingVPSResult{}, err
	}
	return ProQ3StaticEQStagingVPSResult{Workspace: root, Status: "staging_vps_implemented_not_conformed", VPSArtifact: proQ3StaticEQStagingVPSFile, RuntimeLibrary: runtimeLibrary, RunArtifact: runArtifact, ActivationFile: activationPath}, nil
}

// proQ3StaticEQVitHostProjectionBinding imports only the observed guard from
// a Forge Vit-binding artifact.  The imported structure remains marked
// observed and is used solely to let the local transport executor compare
// like-for-like Vit host projections instead of acknowledging an artificial
// mismatch against the independent VST3 Adapter's full surface.
func proQ3StaticEQVitHostProjectionBinding(root string, identity vps.PluginIdentity) (vps.HostProjectionBinding, bool, error) {
	path := filepath.Join(root, vitHostBindingFile)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return vps.HostProjectionBinding{}, false, nil
		}
		return vps.HostProjectionBinding{}, false, fmt.Errorf("read Pro-Q 3 Vit host binding: %w", err)
	}
	var artifact vitHostBindingArtifact
	if err := readJSON(path, &artifact); err != nil {
		return vps.HostProjectionBinding{}, false, fmt.Errorf("decode Pro-Q 3 Vit host binding: %w", err)
	}
	if artifact.SchemaVersion != VitHostBindingSchema || artifact.Trust != "observed" || artifact.Status != "observed_host_projection_compatible" {
		return vps.HostProjectionBinding{}, false, fmt.Errorf("Pro-Q 3 Vit host binding is not an observed compatible projection")
	}
	if !artifact.AdapterFullFingerprint.Equal(identity.Fingerprint) || !artifact.VitHostFingerprint.Complete() {
		return vps.HostProjectionBinding{}, false, fmt.Errorf("Pro-Q 3 Vit host binding does not match the independent staged plug-in identity")
	}
	required := make([]string, 0, len(artifact.RequiredParameters))
	for _, parameter := range artifact.RequiredParameters {
		if strings.TrimSpace(parameter.ID) == "" || parameter.Status != "matched_observed" || !parameter.AdapterStableID || !parameter.AdapterControllable || !parameter.VitControllable {
			return vps.HostProjectionBinding{}, false, fmt.Errorf("Pro-Q 3 Vit host binding has an unavailable required control")
		}
		required = append(required, parameter.ID)
	}
	binding := vps.HostProjectionBinding{
		SchemaVersion:        vps.HostProjectionBindingSchemaVersion,
		HostID:               vps.VitHostProjectionID,
		Fingerprint:          artifact.VitHostFingerprint,
		RequiredParameterIDs: uniqueSorted(required),
		EvidenceRefs:         []string{"vpsforge:" + vitHostBindingFile, "vpsforge:" + vitHostSurfaceFile},
		Trust:                "observed",
		Status:               "observed_host_projection_compatible",
		CapturedAt:           artifact.CapturedAt,
	}
	if err := binding.Validate(); err != nil {
		return vps.HostProjectionBinding{}, false, fmt.Errorf("validate Pro-Q 3 Vit host projection binding: %w", err)
	}
	return binding, true, nil
}

// proQ3StaticEQWriteStagingActivation is a local pointer under the staging
// root, never under the canonical VPS Library.  The normal VitAgent reads it
// only as an opt-in bridge configuration and continues to keep Catalog and
// Credentials in its usual user Library.
func proQ3StaticEQWriteStagingActivation(workspaceRoot, artifactFile, vpsID string, revision int, now time.Time) (string, error) {
	stagingRoot := filepath.Dir(filepath.Dir(workspaceRoot))
	artifactPath := filepath.Join(workspaceRoot, artifactFile)
	relativeArtifact, err := filepath.Rel(stagingRoot, artifactPath)
	if err != nil {
		return "", fmt.Errorf("resolve staging activation artifact: %w", err)
	}
	activationPath := filepath.Join(stagingRoot, proQ3StaticEQStagingActivationFile)
	activation := map[string]any{
		"schema_version":   proQ3StaticEQStagingActivationSchema,
		"enabled":          true,
		"staging_vps_path": filepath.ToSlash(relativeArtifact),
		"vps_id":           vpsID,
		"vps_revision":     revision,
		"status":           "staging_vps_implemented_not_conformed",
		"updated_at":       now,
		"authority_boundary": []string{
			"This pointer enables an isolated staging bridge only.",
			"It cannot issue a Credential, modify Catalog, or enable SPAL dispatch.",
		},
	}
	if err := writeJSON(activationPath, activation); err != nil {
		return "", fmt.Errorf("write staging activation: %w", err)
	}
	return activationPath, nil
}

// proQ3StaticEQStagingWriteLibrary updates only the disposable workspace
// library. This permits a revised staging implementation without writing the
// canonical user library.
func proQ3StaticEQStagingWriteLibrary(path string, candidate vps.VPSDocument) (vps.VPSDocument, error) {
	library, err := vps.NewLibrary(path)
	if err != nil {
		return vps.VPSDocument{}, fmt.Errorf("open isolated Pro-Q 3 staging VPS library: %w", err)
	}
	stored, err := library.Upsert(candidate)
	if err != nil {
		return vps.VPSDocument{}, fmt.Errorf("write isolated Pro-Q 3 staging VPS library: %w", err)
	}
	return stored, nil
}
