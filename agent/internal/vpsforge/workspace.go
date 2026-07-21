package vpsforge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/vps"
)

const (
	manifestFile                      = "authoring_manifest.json"
	ledgerFile                        = "evidence_ledger.json"
	surfaceFile                       = "surface_snapshot.json"
	fxmPlanFile                       = "fxm_measurement_plan.json"
	fxmProjectionFile                 = "fxm_projection.json"
	draftFile                         = "vps_draft.json"
	pluginIdentityFile                = "plugin_identity.json"
	parameterProbeFile                = "parameter_probe_results.json"
	stateRoundtripFile                = "state_roundtrip_results.json"
	fxmBaselineFile                   = "fxm_default_baseline.json"
	groupsFile                        = "inferred_parameter_groups.json"
	humanGapsFile                     = "human_evidence_gaps.json"
	witnessRoundsFile                 = "witness_rounds.json"
	humanWitnessFile                  = "human_witness_evidence.json"
	stereoPlacementResultsFile        = "stereo_placement_probe_results.json"
	stereoPlacementGuideFile          = "stereo_placement_listening_guide.md"
	proQ3ResourceIsolationResultsFile = "pro_q_3_resource_isolation_probe_results.json"
	proQ3EqualizerReviewSummaryFile   = "pro_q_3_equalizer_review_summary.json"
	equalizerV2ProposalDraftFile      = "equalizer_v2_conformance_proposal_draft.json"
	equalizerV2ScopeReviewFile        = "equalizer_v2_scope_review.json"
	equalizerV2StaticEQMatrixFile     = "equalizer_v2_static_eq_review_matrix.json"
	proQ3StaticLifecycleResultsFile   = "pro_q_3_static_lifecycle_probe_results.json"
	equalizerV2DecisionDraftFile      = "equalizer_v2_conformance_decision_draft.json"
	proQ3StaticEQDraftTestProfileFile = "pro_q_3_static_eq_draft_test_profile.json"
	proQ3StaticEQStagingVPSFile       = "pro_q_3_static_eq_staging_vps.json"
	vitHostBindingFile                = "vit_host_binding.json"
	vitHostSurfaceFile                = "vit_host_surface_snapshot.json"
	conformanceFile                   = "conformance_skeleton.json"
	candidateBadgesFile               = "candidate_task_badges.json"
	badgeActionContractFile           = "badge_action_contract.json"
	featureGapsFile                   = "required_feature_gaps.json"
	badgeSkeletonFile                 = "badge_conformance_skeleton.json"
)

type InitRequest struct {
	Root         string
	Identity     vps.PluginIdentity
	Capabilities []string
	HostProtocol string
	HostEndpoint string
	Now          time.Time
}

func Init(req InitRequest) (Status, error) {
	root := filepath.Clean(strings.TrimSpace(req.Root))
	if root == "" || root == "." {
		return Status{}, fmt.Errorf("authoring workspace root is required")
	}
	if _, err := os.Stat(filepath.Join(root, manifestFile)); err == nil {
		return Status{}, fmt.Errorf("authoring workspace already exists: %s", root)
	}
	format := strings.ToUpper(strings.TrimSpace(req.Identity.Format))
	if !supportedAuthoringFormat(format) {
		return Status{}, fmt.Errorf("unsupported authoring format %q; expected VST3, CLAP, AAX or AU", req.Identity.Format)
	}
	req.Identity.Format = format
	if strings.TrimSpace(req.Identity.Name) == "" {
		return Status{}, fmt.Errorf("plugin name is required")
	}
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	capabilities := uniqueSorted(req.Capabilities)
	if len(capabilities) == 0 {
		capabilities = []string{"unknown"}
	}
	workspaceID := stableID("vps_author", req.Identity.Manufacturer, req.Identity.Name, req.Identity.Format, req.Identity.Version, now.Format(time.RFC3339Nano))
	manifest := Manifest{
		SchemaVersion: ManifestSchema, WorkspaceID: workspaceID, PluginIdentity: req.Identity, CapabilityIDs: capabilities, Status: "discovery",
		HostAdapter: HostAdapter{Protocol: firstNonEmpty(req.HostProtocol, "vit.vps_host_adapter.v1"), Endpoint: strings.TrimSpace(req.HostEndpoint), PluginFormat: format, Status: "unconnected"},
		Artifacts: map[string]string{
			"evidence_ledger": ledgerFile, "plugin_identity": pluginIdentityFile, "surface_snapshot": surfaceFile,
			"parameter_probe_results": parameterProbeFile, "state_roundtrip_results": stateRoundtripFile,
			"fxm_default_baseline": fxmBaselineFile, "inferred_parameter_groups": groupsFile,
			"human_evidence_gaps": humanGapsFile, "witness_rounds": witnessRoundsFile, "human_witness_evidence": humanWitnessFile,
			"stereo_placement_probe_results": stereoPlacementResultsFile, "stereo_placement_listening_guide": stereoPlacementGuideFile,
			"pro_q_3_eq_core_probe_results":            proQ3CoreEQResultsFile,
			"pro_q_3_resource_isolation_probe_results": proQ3ResourceIsolationResultsFile,
			"pro_q_3_equalizer_review_summary":         proQ3EqualizerReviewSummaryFile,
			"equalizer_v2_conformance_proposal_draft":  equalizerV2ProposalDraftFile,
			"equalizer_v2_scope_review":                equalizerV2ScopeReviewFile,
			"equalizer_v2_static_eq_review_matrix":     equalizerV2StaticEQMatrixFile,
			"pro_q_3_static_lifecycle_probe_results":   proQ3StaticLifecycleResultsFile,
			"equalizer_v2_conformance_decision_draft":  equalizerV2DecisionDraftFile,
			"pro_q_3_static_eq_draft_test_profile":     proQ3StaticEQDraftTestProfileFile,
			"pro_q_3_static_eq_staging_vps":            proQ3StaticEQStagingVPSFile,
			"vit_host_binding":                         vitHostBindingFile,
			"vit_host_surface_snapshot":                vitHostSurfaceFile,
			"conformance_skeleton":                     conformanceFile, "candidate_task_badges": candidateBadgesFile,
			"badge_action_contract": badgeActionContractFile,
			"required_feature_gaps": featureGapsFile, "badge_conformance_skeleton": badgeSkeletonFile,
			"fxm_measurement_plan": fxmPlanFile, "fxm_projection": fxmProjectionFile, "vps_draft": draftFile,
			"workspace_readme": "README.md", "agent_protocol": "agent_protocol.json",
		},
		CreatedAt: now, UpdatedAt: now,
		Limitations: []string{"staging-only workspace; no VPS Library, Credential, Catalog or SPAL mutation", "automatic host facts are observed evidence and never semantic authority", "AAX hosting requires a licensed SDK and compatible host adapter"},
	}
	draft := vps.NewDraft(req.Identity, now)
	for _, capabilityID := range capabilities {
		draft.SemanticCapabilities = append(draft.SemanticCapabilities, vps.SemanticCapability{ID: capabilityID, Status: "unknown"})
		if profile, ok := vps.ProfileFor(capabilityID); ok {
			draft.CapabilityProfiles = append(draft.CapabilityProfiles, profile)
		}
		draft.FXMMeasurements = append(draft.FXMMeasurements, vps.DefaultFXMMeasurementRequirement(capabilityID))
	}
	ledger := EvidenceLedger{SchemaVersion: LedgerSchema, WorkspaceID: workspaceID, Entries: []EvidenceEntry{{ID: stableID("evidence", workspaceID, "authoring-init"), Kind: "authoring_intent", Trust: "user-confirmed", Source: "vpsforge.init", Summary: "Created an isolated Vit VPS Forge workspace; no canonical library mutation was requested.", CapturedAt: now}}}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, manifestFile), manifest); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, ledgerFile), ledger); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, draftFile), draft); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, fxmPlanFile), draft.FXMMeasurements); err != nil {
		return Status{}, err
	}
	if err := writeAgentGuide(root, manifest); err != nil {
		return Status{}, err
	}
	return Inspect(root)
}

func Inspect(root string) (Status, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	var manifest Manifest
	var ledger EvidenceLedger
	var draft vps.VPSDocument
	if err := readJSON(filepath.Join(root, manifestFile), &manifest); err != nil {
		return Status{}, err
	}
	if err := readJSON(filepath.Join(root, ledgerFile), &ledger); err != nil {
		return Status{}, err
	}
	if err := readJSON(filepath.Join(root, draftFile), &draft); err != nil {
		return Status{}, err
	}
	status := Status{Workspace: root, Manifest: manifest, EvidenceCount: len(ledger.Entries), Draft: draft}
	if _, err := os.Stat(filepath.Join(root, surfaceFile)); err == nil {
		status.SurfaceAvailable = true
	}
	var artifact FXMArtifact
	if readJSON(filepath.Join(root, fxmProjectionFile), &artifact) == nil {
		status.FXMAvailable = true
		status.FXMStatus = artifact.Projection.Status
	}
	status.Validation = Validate(root)
	return status, nil
}

func Validate(root string) ValidationResult {
	errorsOut, warnings := []string{}, []string{}
	var manifest Manifest
	var ledger EvidenceLedger
	var draft vps.VPSDocument
	if err := readJSON(filepath.Join(root, manifestFile), &manifest); err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	if err := readJSON(filepath.Join(root, ledgerFile), &ledger); err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	if err := readJSON(filepath.Join(root, draftFile), &draft); err != nil {
		errorsOut = append(errorsOut, err.Error())
	} else if err := draft.Validate(); err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	if manifest.SchemaVersion != ManifestSchema && manifest.SchemaVersion != LegacyManifestSchema {
		errorsOut = append(errorsOut, "authoring manifest schema mismatch")
	} else if manifest.SchemaVersion == LegacyManifestSchema {
		warnings = append(warnings, "legacy VPS Authoring manifest accepted read-only; new workspaces use "+ManifestSchema)
	}
	if ledger.SchemaVersion != LedgerSchema {
		errorsOut = append(errorsOut, "evidence ledger schema mismatch")
	}
	if len(draft.ProviderCredentials) > 0 {
		errorsOut = append(errorsOut, "authoring draft must not contain Provider Credentials")
	}
	if _, err := os.Stat(filepath.Join(root, surfaceFile)); err != nil {
		warnings = append(warnings, "host parameter/display surface has not been captured")
	}
	var artifact FXMArtifact
	if err := readJSON(filepath.Join(root, fxmProjectionFile), &artifact); err != nil {
		warnings = append(warnings, "FXM counterfactual measurement has not been captured")
	} else if !artifact.Projection.TrustQuality.CanSupportObservation {
		warnings = append(warnings, "FXM projection cannot support observation: "+strings.Join(artifact.Projection.TrustQuality.BlockedReasons, ", "))
	}
	next := []string{}
	if len(errorsOut) == 0 {
		next = append(next, "capture a real host surface through an adapter", "record user-confirmed semantic mappings", "run write/readback, rollback and FXM measurement conformance", "request explicit installation only after Credential conformance")
	}
	status := "valid"
	if len(errorsOut) > 0 {
		status = "invalid"
	}
	return ValidationResult{Status: status, Errors: errorsOut, Warnings: warnings, InstallReady: false, NextSteps: next}
}

func IngestSurface(root string, snapshot SurfaceSnapshot, source string, now time.Time) (Status, error) {
	if snapshot.SchemaVersion == "" {
		snapshot.SchemaVersion = SurfaceSchema
	}
	if snapshot.SchemaVersion != SurfaceSchema {
		return Status{}, fmt.Errorf("surface snapshot schema must be %s", SurfaceSchema)
	}
	if snapshot.CapturedAt.IsZero() {
		snapshot.CapturedAt = nowOrCurrent(now)
	}
	var manifest Manifest
	if err := readJSON(filepath.Join(root, manifestFile), &manifest); err != nil {
		return Status{}, err
	}
	if err := samePluginFamily(manifest.PluginIdentity, snapshot.PluginIdentity); err != nil {
		return Status{}, err
	}
	seenParameters := map[string]bool{}
	for _, parameter := range snapshot.Parameters {
		id := strings.TrimSpace(parameter.ID)
		if id == "" || seenParameters[id] {
			return Status{}, fmt.Errorf("surface snapshot contains a missing or duplicate parameter id %q", parameter.ID)
		}
		seenParameters[id] = true
	}
	if err := writeJSON(filepath.Join(root, surfaceFile), snapshot); err != nil {
		return Status{}, err
	}
	data, _ := json.Marshal(snapshot)
	if err := appendEvidence(root, EvidenceEntry{Kind: "host_surface", Trust: "observed", Source: firstNonEmpty(source, "host_adapter"), Summary: fmt.Sprintf("Captured %d host parameters and display-surface facts.", len(snapshot.Parameters)), Artifact: surfaceFile, CapturedAt: snapshot.CapturedAt, Data: data}); err != nil {
		return Status{}, err
	}
	if err := updateManifest(root, func(m *Manifest) {
		m.Status = "semantic_witness"
		m.HostAdapter.Status = "connected"
		m.UpdatedAt = snapshot.CapturedAt
	}); err != nil {
		return Status{}, err
	}
	var draft vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &draft); err != nil {
		return Status{}, err
	}
	if strings.TrimSpace(snapshot.PluginIdentity.Manufacturer) != "" {
		draft.PluginIdentity.Manufacturer = snapshot.PluginIdentity.Manufacturer
	}
	if strings.TrimSpace(snapshot.PluginIdentity.Name) != "" {
		draft.PluginIdentity.Name = snapshot.PluginIdentity.Name
	}
	if strings.TrimSpace(snapshot.PluginIdentity.Format) != "" {
		draft.PluginIdentity.Format = strings.ToUpper(snapshot.PluginIdentity.Format)
	}
	if strings.TrimSpace(snapshot.PluginIdentity.Version) != "" {
		draft.PluginIdentity.Version = snapshot.PluginIdentity.Version
	}
	if strings.TrimSpace(snapshot.PluginIdentity.InstallPath) != "" {
		draft.PluginIdentity.InstallPath = snapshot.PluginIdentity.InstallPath
	}
	if snapshot.PluginIdentity.Fingerprint.Complete() || snapshot.PluginIdentity.Fingerprint.ParameterSurface != "" || snapshot.PluginIdentity.Fingerprint.DisplaySurface != "" {
		draft.PluginIdentity.Fingerprint = snapshot.PluginIdentity.Fingerprint
	}
	draft.ControlSurface.Mappings = make([]vps.ControlSurfaceMapping, 0, len(snapshot.Parameters))
	for _, parameter := range snapshot.Parameters {
		draft.ControlSurface.Mappings = append(draft.ControlSurface.Mappings, vps.ControlSurfaceMapping{
			ParameterID: parameter.ID, Label: parameter.Name, DisplayDomain: parameter.DisplayDomain,
			Confirmed: false, BindingStatus: "observed", ExecutionScope: "vps_authoring_only",
			Warning:      "Host surface observation is not executable semantics until user confirmation and conformance.",
			EvidenceRefs: []string{"vpsforge.surface:" + stableID("surface", manifest.WorkspaceID, parameter.ID)},
		})
	}
	if err := draft.Validate(); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, draftFile), draft); err != nil {
		return Status{}, err
	}
	return Inspect(root)
}

func RecordFXM(root string, input fxm.Input, now time.Time) (Status, error) {
	if input.CreatedAt == "" {
		input.CreatedAt = nowOrCurrent(now).Format(time.RFC3339Nano)
	}
	projection := fxm.Build(input)
	artifact := FXMArtifact{Input: input, Projection: projection}
	if err := writeJSON(filepath.Join(root, fxmProjectionFile), artifact); err != nil {
		return Status{}, err
	}
	data, _ := json.Marshal(projection)
	trust := "observed"
	if projection.TrustQuality.CanSupportActionPreflight {
		trust = "conformed"
	}
	if err := appendEvidence(root, EvidenceEntry{Kind: "fxm_projection", Trust: trust, Source: "vpsforge.measure", Summary: projection.LLMContext.SummaryMD, Artifact: fxmProjectionFile, CapturedAt: nowOrCurrent(now), Data: data}); err != nil {
		return Status{}, err
	}
	var draft vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &draft); err != nil {
		return Status{}, err
	}
	for i := range draft.FXMMeasurements {
		draft.FXMMeasurements[i].Status = "observed"
		draft.FXMMeasurements[i].EvidenceRefs = uniqueSorted(append(draft.FXMMeasurements[i].EvidenceRefs, "fxm:"+projection.ProjectionID))
	}
	if err := writeJSON(filepath.Join(root, draftFile), draft); err != nil {
		return Status{}, err
	}
	if err := writeJSON(filepath.Join(root, fxmPlanFile), draft.FXMMeasurements); err != nil {
		return Status{}, err
	}
	if err := updateManifest(root, func(m *Manifest) { m.Status = "conformance_pending"; m.UpdatedAt = nowOrCurrent(now) }); err != nil {
		return Status{}, err
	}
	return Inspect(root)
}

func ProbeHost(ctx context.Context, root, baseURL string, client *http.Client) (Status, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return Status{}, fmt.Errorf("host adapter endpoint is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/plugin/snapshot", nil)
	if err != nil {
		return Status{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Status{}, fmt.Errorf("host adapter returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var snapshot HostSnapshot
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&snapshot); err != nil {
		return Status{}, err
	}
	if snapshot.Surface.SchemaVersion == "" {
		snapshot.Surface.SchemaVersion = SurfaceSchema
	}
	if snapshot.Surface.PluginIdentity.Name == "" {
		snapshot.Surface.PluginIdentity = snapshot.Identity
	}
	status, err := IngestSurface(root, snapshot.Surface, baseURL, time.Now().UTC())
	if err != nil {
		return Status{}, err
	}
	_ = updateManifest(root, func(m *Manifest) { m.HostAdapter.Endpoint = baseURL; m.HostAdapter.Status = "connected" })
	return status, nil
}

func appendEvidence(root string, entry EvidenceEntry) error {
	var ledger EvidenceLedger
	if err := readJSON(filepath.Join(root, ledgerFile), &ledger); err != nil {
		return err
	}
	if entry.CapturedAt.IsZero() {
		entry.CapturedAt = time.Now().UTC()
	}
	entry.ID = stableID("evidence", ledger.WorkspaceID, entry.Kind, entry.Source, entry.CapturedAt.Format(time.RFC3339Nano))
	ledger.Entries = append(ledger.Entries, entry)
	return writeJSON(filepath.Join(root, ledgerFile), ledger)
}

func updateManifest(root string, mutate func(*Manifest)) error {
	var manifest Manifest
	if err := readJSON(filepath.Join(root, manifestFile), &manifest); err != nil {
		return err
	}
	mutate(&manifest)
	if manifest.UpdatedAt.IsZero() {
		manifest.UpdatedAt = time.Now().UTC()
	}
	return writeJSON(filepath.Join(root, manifestFile), manifest)
}

func readJSON(path string, output any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func supportedAuthoringFormat(value string) bool {
	switch value {
	case "VST3", "CLAP", "AAX", "AU":
		return true
	}
	return false
}

func samePluginFamily(expected, observed vps.PluginIdentity) error {
	for _, item := range []struct {
		label    string
		expected string
		observed string
	}{
		{"manufacturer", expected.Manufacturer, observed.Manufacturer},
		{"name", expected.Name, observed.Name},
		{"format", strings.ToUpper(expected.Format), strings.ToUpper(observed.Format)},
	} {
		if strings.TrimSpace(item.expected) != "" && strings.TrimSpace(item.observed) != "" && !strings.EqualFold(strings.TrimSpace(item.expected), strings.TrimSpace(item.observed)) {
			return fmt.Errorf("host snapshot %s %q does not match authoring target %q", item.label, item.observed, item.expected)
		}
	}
	return nil
}
func nowOrCurrent(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}
func stableID(prefix string, values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:])[:20]
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
