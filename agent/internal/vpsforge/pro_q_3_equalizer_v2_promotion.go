package vpsforge

// This file is the deliberately narrow admission path for the first
// production FabFilter Pro-Q 3 equalizer.v2 VPS.  It does not turn automatic
// labels into semantics: it assembles an explicit reviewed action binding from
// separately attributable observed, user-confirmed and conformance evidence.
// The canonical Library is touched only after a supplied, exact local
// authorization phrase.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const (
	proQ3EqualizerV2PromotionSchema      = "vit.vpsforge.pro_q_3_equalizer_v2_promotion.v1"
	proQ3EqualizerV2PromotionFile        = "pro_q_3_equalizer_v2_promotion.json"
	proQ3EqualizerV2PromotionEvidenceDir = "evidence/pro_q_3_equalizer_v2_promotion"
	proQ3EqualizerV2PromotionPhrase      = "PROMOTE_PRO_Q_3_EQUALIZER_V2_TO_LOCAL_CATALOG"
)

// ProQ3EqualizerV2PromotionRequest has two phases.  With Authorization empty
// it writes only a staging candidate/audit handoff.  Supplying the exact
// phrase performs the separately authorized, atomic local Library write.
type ProQ3EqualizerV2PromotionRequest struct {
	Root                 string
	WitnessWorkspace     string
	CanonicalLibraryPath string
	Authorization        string
	Now                  time.Time
}

type ProQ3EqualizerV2PromotionResult struct {
	Workspace               string `json:"workspace"`
	Status                  string `json:"status"`
	PromotionArtifact       string `json:"promotion_artifact"`
	CandidateVPSID          string `json:"candidate_vps_id"`
	CandidateCredential     string `json:"candidate_credential_id"`
	CanonicalLibraryPath    string `json:"canonical_library_path"`
	StagingActivationPath   string `json:"staging_activation_path,omitempty"`
	StagingActivationStatus string `json:"staging_activation_status,omitempty"`
	BackupPath              string `json:"backup_path,omitempty"`
	CatalogEntryCount       int    `json:"catalog_entry_count,omitempty"`
}

type proQ3PromotionGate struct {
	ID           string   `json:"id"`
	Status       string   `json:"status"`
	Trust        string   `json:"trust"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Detail       string   `json:"detail"`
}

type proQ3PromotionCommit struct {
	CommittedAt             time.Time `json:"committed_at"`
	CanonicalPath           string    `json:"canonical_library_path"`
	BackupPath              string    `json:"backup_path,omitempty"`
	StagingActivationPath   string    `json:"staging_activation_path,omitempty"`
	StagingActivationStatus string    `json:"staging_activation_status,omitempty"`
	VPSID                   string    `json:"vps_id"`
	VPSRevision             int       `json:"vps_revision"`
	CredentialID            string    `json:"credential_id"`
	CatalogEntries          int       `json:"catalog_entry_count"`
}

// proQ3PromotionStagingDeactivation is prepared before a canonical Library
// write.  The staging bridge is a separate authority boundary, so promotion
// must not leave the matching temporary bridge enabled.  It retains the exact
// prior bytes in case the following canonical upsert fails.
type proQ3PromotionStagingDeactivation struct {
	Path       string
	Prior      []byte
	Next       map[string]json.RawMessage
	WasEnabled bool
}

type proQ3EqualizerV2PromotionArtifact struct {
	SchemaVersion      string                `json:"schema_version"`
	Trust              string                `json:"trust"`
	Status             string                `json:"status"`
	CapturedAt         time.Time             `json:"captured_at"`
	WorkspaceID        string                `json:"workspace_id"`
	WitnessWorkspace   string                `json:"witness_workspace"`
	CandidateDocument  vps.VPSDocument       `json:"candidate_document"`
	Gates              []proQ3PromotionGate  `json:"gates"`
	EvidenceRefs       []string              `json:"evidence_refs"`
	AuthorityBoundary  []string              `json:"authority_boundary"`
	KnownUnsupported   []string              `json:"known_unsupported"`
	CrossWorkspaceNote string                `json:"cross_workspace_note"`
	Commit             *proQ3PromotionCommit `json:"commit,omitempty"`
}

type proQ3PromotionWitnessEvidence struct {
	Trust   string            `json:"trust"`
	Records []json.RawMessage `json:"records"`
}

type proQ3PromotionTransportReport struct {
	Status                      string `json:"status"`
	SurfaceCompatibility        string `json:"surface_compatibility"`
	SurfaceMismatchAcknowledged bool   `json:"surface_mismatch_acknowledged"`
	WriteReadback               struct {
		Passed bool `json:"passed"`
	} `json:"write_readback"`
	Rollback struct {
		Passed bool `json:"passed"`
	} `json:"rollback"`
}

// PromoteProQ3EqualizerV2 first builds a reproducible candidate from the
// current r4-style workspace plus the archived r3 GUI witness package. It
// never treats the old and new complete parameter surfaces as interchangeable:
// r4 supplies all automatic/current host evidence; r3 is linked only for the
// archived user-confirmed GUI testimony.
func PromoteProQ3EqualizerV2(request ProQ3EqualizerV2PromotionRequest) (ProQ3EqualizerV2PromotionResult, error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("inspect Pro-Q 3 promotion workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) || !status.Manifest.PluginIdentity.Fingerprint.Complete() {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("promotion requires a complete current FabFilter Pro-Q 3 VST3 workspace identity")
	}
	witnessRoot, err := preflightStagingWorkspaceRoot(request.WitnessWorkspace)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("resolve archived Pro-Q 3 witness workspace: %w", err)
	}
	witnessStatus, err := Inspect(witnessRoot)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("inspect archived Pro-Q 3 witness workspace: %w", err)
	}
	if err := proQ3PromotionCrossWorkspaceIdentity(status.Manifest.PluginIdentity, witnessStatus.Manifest.PluginIdentity); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	now := nowOrCurrent(request.Now)
	gates, evidenceRefs, hostBinding, transportReport, err := proQ3PromotionGates(root, witnessRoot)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	for _, gate := range gates {
		if gate.Status != "passed" {
			return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("Pro-Q 3 promotion gate %s is not passed: %s", gate.ID, gate.Detail)
		}
	}
	var source vps.VPSDocument
	if err := readJSON(filepath.Join(root, draftFile), &source); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("read current Pro-Q 3 VPS Draft: %w", err)
	}
	candidate, credentialID, err := proQ3EqualizerV2PromotionDocument(source, hostBinding, evidenceRefs, now)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	if err := candidate.Validate(); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("validate Pro-Q 3 promotion candidate: %w", err)
	}
	canonicalPath := strings.TrimSpace(request.CanonicalLibraryPath)
	if canonicalPath == "" {
		canonicalPath = vps.DefaultLibraryPath()
	}
	canonicalPath, err = filepath.Abs(canonicalPath)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("resolve canonical VPS Library: %w", err)
	}
	artifact := proQ3EqualizerV2PromotionArtifact{
		SchemaVersion:     proQ3EqualizerV2PromotionSchema,
		Trust:             "agent-inferred",
		Status:            "ready_for_explicit_local_promotion",
		CapturedAt:        now,
		WorkspaceID:       status.Manifest.WorkspaceID,
		WitnessWorkspace:  witnessRoot,
		CandidateDocument: candidate,
		Gates:             gates,
		EvidenceRefs:      evidenceRefs,
		AuthorityBoundary: []string{
			"The candidate distinguishes independent VST3 identity, observed Vit host projection and user GUI testimony.",
			"Observed labels, display strings and automatic probe output remain evidence; the reviewed action binding is the only executable equalizer.v2 authority proposed here.",
			"Without the exact explicit local promotion phrase this artifact is staging-only and cannot modify the VPS Library or derived Catalog.",
			"The derived Provider Catalog is not independently written; it is read from the canonical VPS Library after an atomic upsert.",
			"A matching active VPS Forge staging bridge is disabled before the canonical write, and restored only if that canonical write fails.",
		},
		KnownUnsupported: []string{
			"Dynamic EQ", "automatic/match EQ", "spectrum analyzer", "linear phase and other phase modes",
			"Band 2 and later static EQ bands", "slopes other than 12 and 24 dB/oct", "output controls and vendor-special capabilities",
		},
		CrossWorkspaceNote: "The archived r3 workspace is linked only for user-confirmed GUI testimony after exact product version, install path and installation fingerprint matching. Its older full parameter surface is never used as current automatic evidence, Adapter identity or Vit dispatch guard.",
	}
	artifactPath := filepath.Join(root, proQ3EqualizerV2PromotionFile)
	if err := writeJSON(artifactPath, artifact); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	result := ProQ3EqualizerV2PromotionResult{
		Workspace: root, Status: artifact.Status, PromotionArtifact: proQ3EqualizerV2PromotionFile,
		CandidateVPSID: candidate.ID, CandidateCredential: credentialID, CanonicalLibraryPath: canonicalPath,
	}
	if strings.TrimSpace(request.Authorization) == "" {
		if err := proQ3PromotionPersistStagingEvidence(root, artifact, now); err != nil {
			return ProQ3EqualizerV2PromotionResult{}, err
		}
		return result, nil
	}
	if strings.TrimSpace(request.Authorization) != proQ3EqualizerV2PromotionPhrase {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("canonical promotion requires the exact authorization phrase %q", proQ3EqualizerV2PromotionPhrase)
	}
	stagingDeactivation, err := proQ3PromotionPrepareStagingDeactivation(root, now)
	if err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	if stagingDeactivation != nil && stagingDeactivation.WasEnabled {
		if err := stagingDeactivation.apply(); err != nil {
			return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("disable matching Pro-Q 3 staging bridge before promotion: %w", err)
		}
	}
	backup, catalogEntries, stored, committedCredentialID, err := proQ3PromotionCommitCanonical(candidate, credentialID, canonicalPath, now)
	if err != nil {
		if stagingDeactivation != nil && stagingDeactivation.WasEnabled {
			if restoreErr := stagingDeactivation.restore(); restoreErr != nil {
				return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("promote canonical Pro-Q 3 VPS: %v; restore staging activation: %w", err, restoreErr)
			}
		}
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	artifact.Status = "promoted_to_local_catalog"
	stagingActivationPath, stagingActivationStatus := "", ""
	if stagingDeactivation != nil {
		stagingActivationPath = stagingDeactivation.Path
		stagingActivationStatus = map[bool]string{true: "disabled_matching_staging_bridge", false: "already_disabled_matching_staging_bridge"}[stagingDeactivation.WasEnabled]
	}
	artifact.Commit = &proQ3PromotionCommit{
		CommittedAt: now, CanonicalPath: canonicalPath, BackupPath: backup, VPSID: stored.ID,
		StagingActivationPath: stagingActivationPath, StagingActivationStatus: stagingActivationStatus,
		VPSRevision: stored.Revision, CredentialID: committedCredentialID, CatalogEntries: catalogEntries,
	}
	if err := writeJSON(artifactPath, artifact); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, fmt.Errorf("persist committed Pro-Q 3 promotion artifact: %w", err)
	}
	if err := proQ3PromotionPersistStagingEvidence(root, artifact, now); err != nil {
		return ProQ3EqualizerV2PromotionResult{}, err
	}
	result.Status, result.CandidateCredential, result.StagingActivationPath, result.StagingActivationStatus, result.BackupPath, result.CatalogEntryCount = artifact.Status, committedCredentialID, stagingActivationPath, stagingActivationStatus, backup, catalogEntries
	_ = transportReport // Kept explicit in the gate/evidence artifact; do not embed its large preimage twice.
	return result, nil
}

func proQ3PromotionCrossWorkspaceIdentity(current, archived vps.PluginIdentity) error {
	for _, check := range []struct{ name, current, archived string }{
		{"manufacturer", current.Manufacturer, archived.Manufacturer},
		{"name", current.Name, archived.Name},
		{"format", current.Format, archived.Format},
		{"version", current.Version, archived.Version},
		{"install path", current.InstallPath, archived.InstallPath},
		{"installation fingerprint", current.Fingerprint.Installation, archived.Fingerprint.Installation},
	} {
		if strings.TrimSpace(check.current) == "" || strings.TrimSpace(check.archived) == "" {
			return fmt.Errorf("current and archived Pro-Q 3 workspaces need %s for evidence linkage", check.name)
		}
		if check.name == "install path" {
			if !sameInstallPath(check.current, check.archived) {
				return fmt.Errorf("current and archived Pro-Q 3 witness workspaces use different install paths")
			}
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(check.current), strings.TrimSpace(check.archived)) {
			return fmt.Errorf("current and archived Pro-Q 3 witness workspaces differ in %s", check.name)
		}
	}
	return nil
}

func proQ3PromotionGates(root, witnessRoot string) ([]proQ3PromotionGate, []string, vps.HostProjectionBinding, string, error) {
	gates := []proQ3PromotionGate{}
	evidence := []string{}
	add := func(gate proQ3PromotionGate) {
		gates = append(gates, gate)
		evidence = append(evidence, gate.EvidenceRefs...)
	}

	hostBinding, found, err := proQ3StaticEQVitHostProjectionBinding(root, mustProQ3PromotionIdentity(root))
	if err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", err
	}
	if !found {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("current Pro-Q 3 workspace has no compatible Vit host projection binding")
	}
	add(proQ3PromotionGate{ID: "vit_host_projection", Status: "passed", Trust: "observed", EvidenceRefs: hostBinding.EvidenceRefs, Detail: "current Vit host projection is explicitly bound to the current independent Adapter identity"})

	var core proQ3CoreEQProbeArtifact
	if err := readJSON(filepath.Join(root, proQ3CoreEQResultsFile), &core); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("read current Pro-Q 3 core behavior probe: %w", err)
	}
	if err := proQ3PromotionValidateCoreProbe(core); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", err
	}
	add(proQ3PromotionGate{ID: "current_static_eq_behavior", Status: "passed", Trust: "observed", EvidenceRefs: []string{"vpsforge:" + proQ3CoreEQResultsFile}, Detail: "current independent Adapter measured bounded Bell, Low Cut and High Cut states with fresh readback, state roundtrip and rollback"})

	var baseline struct {
		Trust  string `json:"trust"`
		Status string `json:"status"`
	}
	if err := readJSON(filepath.Join(root, fxmBaselineFile), &baseline); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("read current Pro-Q 3 FXM baseline: %w", err)
	}
	if baseline.Trust != "observed" || baseline.Status != "observed_completed" {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("current Pro-Q 3 FXM default baseline is not observed_completed")
	}
	add(proQ3PromotionGate{ID: "fxm_default_baseline", Status: "passed", Trust: "observed", EvidenceRefs: []string{"vpsforge:" + fxmBaselineFile}, Detail: "current default-state bypass/processed Probe Audio baseline is present"})

	var witnesses proQ3PromotionWitnessEvidence
	if err := readJSON(filepath.Join(witnessRoot, humanWitnessFile), &witnesses); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("read archived Pro-Q 3 human witness evidence: %w", err)
	}
	if witnesses.Trust != "user-confirmed" || len(witnesses.Records) == 0 {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("archived Pro-Q 3 GUI evidence is not user-confirmed")
	}
	add(proQ3PromotionGate{ID: "human_static_eq_witness", Status: "passed", Trust: "user-confirmed", EvidenceRefs: []string{"vpsforge:archived/" + humanWitnessFile}, Detail: "archived user GUI testimony is retained as user-confirmed evidence, not converted into automatic parameter semantics"})

	var lifecycle proQ3StaticLifecycleProbeArtifact
	if err := readJSON(filepath.Join(root, proQ3StaticLifecycleResultsFile), &lifecycle); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("read current Pro-Q 3 lifecycle probe: %w", err)
	}
	if lifecycle.Trust != "observed" || lifecycle.Status != "observed_completed" || !lifecycle.ActiveState.FreshReadbackMatches || !lifecycle.ActiveState.StateRoundtripMatches || !lifecycle.ActiveState.Rollback.ReportedVerified || !lifecycle.ActiveState.Rollback.FreshReadback.Matches || !lifecycle.FinalRollback.InitialStateRestoreVerified || !lifecycle.FinalRollback.FreshReadbackMatchesInitial.Matches {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("current Pro-Q 3 lifecycle evidence does not prove the bounded active-state transaction")
	}
	add(proQ3PromotionGate{ID: "static_lifecycle", Status: "passed", Trust: "observed", EvidenceRefs: []string{"vpsforge:" + proQ3StaticLifecycleResultsFile}, Detail: "current direct-worker lifecycle test recorded Used/Enabled static state, state roundtrip and complete rollback"})

	var isolation proQ3ResourceIsolationProbeArtifact
	if err := readJSON(filepath.Join(root, proQ3ResourceIsolationResultsFile), &isolation); err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("read current Pro-Q 3 resource-isolation probe: %w", err)
	}
	if isolation.Trust != "observed" || isolation.Status != "observed_completed" || !isolation.ResourceIsolation.SeparateWorkerProcessesObserved || !isolation.ResourceIsolation.BPreimageAfterAWrite.Matches || !isolation.ResourceIsolation.BPreimageAtFinalReadback.Matches || !isolation.StateRoundtrip.Completed || !isolation.StateRoundtrip.ParameterReadbackMatchesWorkerPreimage || !isolation.StateRoundtrip.FreshSnapshotMatchesPostWrite.Matches || !isolation.ReloadRestore.UnloadReported || !isolation.ReloadRestore.Reloaded || !isolation.ReloadRestore.PostWriteStateRestored || !isolation.ReloadRestore.FreshSnapshotMatchesPostWrite.Matches || !isolation.FinalRollback.TransactionRollbackReportedVerified || !isolation.FinalRollback.TransactionRollbackFreshReadback.Matches || !isolation.FinalRollback.ReloadedInitialStateRestored || !isolation.FinalRollback.FinalFreshReadbackMatchesInitial.Matches {
		return nil, nil, vps.HostProjectionBinding{}, "", fmt.Errorf("current Pro-Q 3 resource-isolation evidence is incomplete")
	}
	add(proQ3PromotionGate{ID: "resource_isolation_and_reload", Status: "passed", Trust: "observed", EvidenceRefs: []string{"vpsforge:" + proQ3ResourceIsolationResultsFile}, Detail: "current two isolated workers, state restoration after reload, and full rollback passed"})

	transport, transportRef, err := proQ3PromotionFindStrictVitTransport(root)
	if err != nil {
		return nil, nil, vps.HostProjectionBinding{}, "", err
	}
	add(proQ3PromotionGate{ID: "current_vit_transport", Status: "passed", Trust: "observed", EvidenceRefs: []string{"vpsforge:" + transportRef}, Detail: "current Vit host write/readback and complete rollback passed under the bound host projection without mismatch acknowledgement"})
	return gates, uniqueSorted(evidence), hostBinding, transport, nil
}

func mustProQ3PromotionIdentity(root string) vps.PluginIdentity {
	status, err := Inspect(root)
	if err != nil {
		return vps.PluginIdentity{}
	}
	return status.Manifest.PluginIdentity
}

func proQ3PromotionValidateCoreProbe(core proQ3CoreEQProbeArtifact) error {
	if core.SchemaVersion != proQ3CoreEQProbeSchema || core.Trust != "observed" || core.Status != "observed_completed" || !core.FinalRollback.InitialStateRestoreVerified || !core.FinalRollback.FreshReadbackMatches {
		return fmt.Errorf("current Pro-Q 3 core behavior evidence is incomplete")
	}
	requiredRoles := map[string]bool{"band_used": false, "band_enabled": false, "band_frequency": false, "band_gain": false, "band_q": false, "band_shape": false, "band_slope": false}
	for _, control := range core.ObservedControls {
		if _, wanted := requiredRoles[control.Role]; wanted && strings.TrimSpace(control.ID) != "" && control.Automation == "automatable" {
			requiredRoles[control.Role] = true
		}
	}
	for role, present := range requiredRoles {
		if !present {
			return fmt.Errorf("current Pro-Q 3 core behavior evidence lacks %s", role)
		}
	}
	for _, choice := range append(append([]proQ3CoreEQChoice(nil), core.ShapeDiscovery...), core.SlopeDiscovery...) {
		if !choice.FreshReadbackMatches || !choice.StateRoundtripMatches || !choice.RollbackVerified || !choice.RollbackReadbackMatches {
			return fmt.Errorf("current Pro-Q 3 discrete-state evidence has a failed transaction")
		}
	}
	hasChoice := func(choices []proQ3CoreEQChoice, display string) bool {
		for _, choice := range choices {
			if strings.EqualFold(strings.TrimSpace(choice.ObservedDisplay), display) {
				return true
			}
		}
		return false
	}
	for _, display := range []string{"Bell", "Low Cut", "High Cut"} {
		if !hasChoice(core.ShapeDiscovery, display) {
			return fmt.Errorf("current Pro-Q 3 core behavior evidence lacks observed shape %s", display)
		}
	}
	for _, display := range []string{"12 dB/oct", "24 dB/oct"} {
		if !hasChoice(core.SlopeDiscovery, display) {
			return fmt.Errorf("current Pro-Q 3 core behavior evidence lacks observed slope %s", display)
		}
	}
	for _, scenarioID := range []string{"bell_lower_normalized_boost", "bell_higher_normalized_boost", "low_cut_candidate", "high_cut_candidate", "low_cut_24_db_per_octave_candidate"} {
		found := false
		for _, scenario := range core.Scenarios {
			if scenario.ID != scenarioID {
				continue
			}
			found = scenario.FreshReadback && scenario.StateRoundtrip && scenario.Render != nil && scenario.Rollback.Verified && scenario.Rollback.FreshReadback
		}
		if !found {
			return fmt.Errorf("current Pro-Q 3 core behavior evidence lacks a passed %s scenario", scenarioID)
		}
	}
	return nil
}

func proQ3PromotionFindStrictVitTransport(root string) (string, string, error) {
	reportRoot := filepath.Join(root, "staging_vps_runtime", "vps_draft_test_reports")
	paths, err := filepath.Glob(filepath.Join(reportRoot, "vps_draft_run_*.json"))
	if err != nil {
		return "", "", fmt.Errorf("find current Vit transport reports: %w", err)
	}
	sort.Strings(paths)
	for index := len(paths) - 1; index >= 0; index-- {
		var report proQ3PromotionTransportReport
		if err := readJSON(paths[index], &report); err != nil {
			continue
		}
		if report.Status == "passed_transport_only" && report.SurfaceCompatibility == "strict_match" && !report.SurfaceMismatchAcknowledged && report.WriteReadback.Passed && report.Rollback.Passed {
			relative, relErr := filepath.Rel(root, paths[index])
			if relErr != nil {
				return "", "", relErr
			}
			return paths[index], filepath.ToSlash(relative), nil
		}
	}
	return "", "", fmt.Errorf("current Pro-Q 3 workspace has no strict-match Vit write/readback/rollback transport report")
}

func proQ3EqualizerV2PromotionDocument(source vps.VPSDocument, hostBinding vps.HostProjectionBinding, evidenceRefs []string, now time.Time) (vps.VPSDocument, string, error) {
	if source.Status != vps.VPSStatusDraft || !isProQ3Identity(source.PluginIdentity) || !source.PluginIdentity.Fingerprint.Complete() {
		return vps.VPSDocument{}, "", fmt.Errorf("promotion source must be a complete current Pro-Q 3 Draft")
	}
	if err := hostBinding.Validate(); err != nil {
		return vps.VPSDocument{}, "", err
	}
	candidate := source
	candidate.Status = vps.VPSStatusVerified
	candidate.Revision = 1
	candidate.ProviderCredentials = nil
	candidate.HostProjectionBindings = []vps.HostProjectionBinding{hostBinding}
	candidate.UpdatedAt = now
	if candidate.CreatedAt.IsZero() {
		candidate.CreatedAt = now
	}
	selected := map[string]string{"0": "allocated", "1": "enabled", "2": "frequency_hz", "3": "gain_db", "7": "q", "8": "response_shape", "9": "slope_db_per_octave"}
	for index := range candidate.ControlSurface.Mappings {
		mapping := &candidate.ControlSurface.Mappings[index]
		slot, selectedControl := selected[strings.TrimSpace(mapping.ParameterID)]
		if !selectedControl {
			continue
		}
		mapping.ComponentID = "b1"
		mapping.SemanticSlot = slot
		mapping.Confirmed = false
		mapping.BindingStatus = "observed_physical_binding_referenced_by_reviewed_equalizer_v2_action"
		mapping.ExecutionScope = "equalizer.v2_verified_credential"
		mapping.Warning = "Observed physical selector only. Its semantic authority is limited to the reviewed equalizer.v2 Credential action binding and does not generalize from its label."
		mapping.EvidenceRefs = uniqueSorted(append(mapping.EvidenceRefs, evidenceRefs...))
	}
	profile := vps.EqualizerConformanceProfileV2()
	candidate.SemanticCapabilities = []vps.SemanticCapability{{
		ID: profile.ID, Operations: []string{vps.OperationEQBandPatch, vps.OperationEQPassFilterPatch},
		Parameters:  []string{"band_ref", "enabled", "response_shape", "frequency_hz", "gain_db", "q", "filter_kind", "cutoff_frequency_hz", "slope_db_per_octave"},
		FilterTypes: []string{"bell", "highpass", "lowpass"}, Status: string(vps.CredentialVerified), EvidenceRefs: evidenceRefs,
	}}
	candidate.CapabilityProfiles = []vps.CapabilityConformanceProfile{profile}
	candidate.BadgeActionImplementations = proQ3EqualizerV2ConformedActionImplementations(evidenceRefs)
	candidate.SpecialCapabilities = []vps.SpecialCapability{
		{ID: "fabfilter.pro_q_3.dynamic_eq", Status: "unsupported/unknown", Dispatchable: false, InvocationPolicy: "not_dispatchable", Summary: "Not admitted to equalizer.v2.", Unknowns: []string{"semantic mapping", "behavior conformance"}},
		{ID: "fabfilter.pro_q_3.auto_eq", Status: "unsupported/unknown", Dispatchable: false, InvocationPolicy: "not_dispatchable", Summary: "Not admitted to equalizer.v2.", Unknowns: []string{"schema", "conformance"}},
		{ID: "fabfilter.pro_q_3.analyzer", Status: "unsupported/unknown", Dispatchable: false, InvocationPolicy: "not_dispatchable", Summary: "Not admitted to equalizer.v2.", Unknowns: []string{"schema", "conformance"}},
		{ID: "fabfilter.pro_q_3.phase_modes", Status: "unsupported/unknown", Dispatchable: false, InvocationPolicy: "not_dispatchable", Summary: "Not admitted to equalizer.v2.", Unknowns: []string{"schema", "conformance"}},
	}
	binding := proQ3EqualizerV2ConformedBinding()
	credentialID := "credential_" + stableID("pro_q_3_equalizer_v2", candidate.ID, candidate.PluginIdentity.Fingerprint.Installation)
	credential := vps.ProviderCredential{
		ID: credentialID, Revision: 1, CapabilityID: vps.EqualizerCapabilityID, Status: vps.CredentialVerified,
		PluginFingerprint: candidate.PluginIdentity.Fingerprint,
		Conformance: vps.CredentialConformance{
			ProfileID: profile.ID, ProfileVersion: profile.Version,
			Operations:  []string{vps.OperationEQBandPatch, vps.OperationEQPassFilterPatch},
			Parameters:  []string{"band_ref", "enabled", "response_shape", "frequency_hz", "gain_db", "q", "filter_kind", "cutoff_frequency_hz", "slope_db_per_octave"},
			FilterTypes: []string{"bell", "highpass", "lowpass"}, EQV2Binding: &binding,
			ConformedSchemas:    []string{spal.EQBandPatchControlID, spal.EQPassFilterPatchControlID},
			WriteReadbackPassed: true, BoundaryTestsPassed: true, RollbackTestPassed: true,
			BehaviorTestsPassed: true, BehaviorScope: "Band 1 static Bell plus Low Cut/High Cut at 12 and 24 dB/oct only; all other EQ features remain unsupported/unknown.",
			StateRetentionPassed: true, ResourceIsolationPassed: true, EvidenceRefs: evidenceRefs, CompletedAt: now,
		},
		Safety: vps.SafetyAndRollback{
			Preconditions: []string{"match the independent VST3 installation fingerprint", "match the observed Vit host-projection binding", "fresh complete parameter preimage before every write"},
			Bounds:        []string{"Band 1 only", "Bell, Low Cut and High Cut only", "Low/High Cut slopes limited to 12 or 24 dB/oct"},
			RollbackMode:  "fresh_preimage_restore_and_full_readback_verification",
			Invalidators:  []string{"independent VST3 installation fingerprint change", "Vit host projection fingerprint change", "failure of fresh readback or rollback"},
		},
		EvidenceRefs: evidenceRefs, IssuedAt: now, UpdatedAt: now,
	}
	candidate.ProviderCredentials = []vps.ProviderCredential{credential}
	candidate.ConformanceEvidence = uniqueSorted(append(candidate.ConformanceEvidence, evidenceRefs...))
	fxmRequirement := vps.DefaultFXMMeasurementRequirement(vps.EqualizerCapabilityID)
	fxmRequirement.Operations = []string{vps.OperationEQBandPatch, vps.OperationEQPassFilterPatch}
	fxmRequirement.Status = "conformed"
	fxmRequirement.EvidenceRefs = evidenceRefs
	fxmRequirement.Limitations = append(fxmRequirement.Limitations, "Only the admitted static Band 1 action set is covered.")
	candidate.FXMMeasurements = []vps.FXMMeasurementRequirement{fxmRequirement}
	payload, _ := json.Marshal(map[string]any{"evidence_refs": evidenceRefs, "host_projection": hostBinding})
	candidate.RawObservations = append(candidate.RawObservations, vps.RawObservation{Kind: "vpsforge_equalizer_v2_admission", Source: "vpsforge.pro_q_3_equalizer_v2_promotion", Summary: "Explicit reviewed admission package; source evidence trust classes remain separately attributable.", CapturedAt: now, Data: payload})
	return candidate, credentialID, nil
}

func proQ3EqualizerV2ConformedBinding() spal.EQV2Binding {
	linear := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "linear"}
	}
	logarithmic := func(id, unit string, minimum, maximum float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: minimum, Max: maximum, Scale: "log"}
	}
	allocated := linear("0", "toggle", 0, 1)
	shape := spal.EnumParameterBinding{ParameterID: "8", Values: map[string]float64{"bell": 0, "highpass": .25, "lowpass": .5}}
	slope := spal.EnumParameterBinding{ParameterID: "9", Values: map[string]float64{"12": 1.0 / 9.0, "24": 3.0 / 9.0}}
	band := spal.EQV2BandBinding{
		ComponentID: "b1", Allocated: &allocated, Enabled: linear("1", "toggle", 0, 1),
		ResponseShape: shape, FrequencyHz: logarithmic("2", "Hz", 10, 30000), GainDB: linear("3", "dB", -30, 30), Q: logarithmic("7", "Q", .025, 40),
	}
	pass := func() *spal.EQV2PassFilterBinding {
		passShape := shape
		return &spal.EQV2PassFilterBinding{ComponentID: "b1", Allocated: &allocated, ResponseShape: &passShape, Enabled: linear("1", "toggle", 0, 1), CutoffFrequencyHz: logarithmic("2", "Hz", 10, 30000), SlopeDBPerOctave: slope}
	}
	return spal.EQV2Binding{ConformedSchemas: []string{spal.EQBandPatchControlID, spal.EQPassFilterPatchControlID}, Bands: map[string]spal.EQV2BandBinding{"b1": band}, HighPass: pass(), LowPass: pass()}
}

func proQ3EqualizerV2ConformedActionImplementations(evidenceRefs []string) []vps.VPSActionImplementation {
	return []vps.VPSActionImplementation{
		{
			BadgeID: vps.EqualizerCapabilityID, BadgeVersion: vps.EqualizerProfileVersion, ActionID: "eq.static_band.patch", SchemaID: spal.EQBandPatchControlID, Status: vps.BadgeFeatureStatusConformed, BindingRef: "credential.eq_v2_binding.b1",
			Features: []vps.BadgeFeatureMatrixEntry{{ActionID: "eq.static_band.patch", FeatureID: "static_band", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}, {ActionID: "eq.static_band.patch", FeatureID: "bell", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}}, EvidenceRefs: evidenceRefs,
		},
		{
			BadgeID: vps.EqualizerCapabilityID, BadgeVersion: vps.EqualizerProfileVersion, ActionID: "eq.pass_filter.patch", SchemaID: spal.EQPassFilterPatchControlID, Status: vps.BadgeFeatureStatusConformed, BindingRef: "credential.eq_v2_binding.b1",
			Features: []vps.BadgeFeatureMatrixEntry{{ActionID: "eq.pass_filter.patch", FeatureID: "highpass", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}, {ActionID: "eq.pass_filter.patch", FeatureID: "lowpass", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}, {ActionID: "eq.pass_filter.patch", FeatureID: "slope_12_db_per_octave", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}, {ActionID: "eq.pass_filter.patch", FeatureID: "slope_24_db_per_octave", Status: vps.BadgeFeatureStatusConformed, EvidenceRefs: evidenceRefs}}, EvidenceRefs: evidenceRefs,
		},
	}
}

func proQ3PromotionCommitCanonical(candidate vps.VPSDocument, credentialID, canonicalPath string, now time.Time) (string, int, vps.VPSDocument, string, error) {
	library, err := vps.NewLibrary(canonicalPath)
	if err != nil {
		return "", 0, vps.VPSDocument{}, "", err
	}
	if existing, found, getErr := library.FindByPluginIdentity(candidate.PluginIdentity); getErr != nil {
		return "", 0, vps.VPSDocument{}, "", fmt.Errorf("inspect canonical Pro-Q 3 VPS: %w", getErr)
	} else if found {
		recovery, recoveryCredentialID, recoveryErr := proQ3PromotionFalseStaleRecoveryDocument(existing, candidate, credentialID, now)
		if recoveryErr != nil {
			return "", 0, vps.VPSDocument{}, "", fmt.Errorf("canonical Library already contains Pro-Q 3 VPS %s and is not eligible for the narrow false-stale recovery: %w", existing.ID, recoveryErr)
		}
		backup, backupErr := proQ3PromotionBackupCanonicalLibrary(canonicalPath, now)
		if backupErr != nil {
			return "", 0, vps.VPSDocument{}, "", backupErr
		}
		stored, upsertErr := library.Upsert(recovery)
		if upsertErr != nil {
			return "", 0, vps.VPSDocument{}, "", fmt.Errorf("atomic Pro-Q 3 false-stale recovery upsert: %w", upsertErr)
		}
		catalog, catalogErr := library.Catalog()
		if catalogErr != nil {
			return "", 0, vps.VPSDocument{}, "", fmt.Errorf("derive canonical Provider Catalog after false-stale recovery: %w", catalogErr)
		}
		if !proQ3PromotionCatalogContains(catalog, stored.ID, recoveryCredentialID) {
			return "", 0, vps.VPSDocument{}, "", fmt.Errorf("canonical Library did not derive the recovered Pro-Q 3 equalizer.v2 Catalog entry")
		}
		return backup, len(catalog.Entries), stored, recoveryCredentialID, nil
	}
	backup, err := proQ3PromotionBackupCanonicalLibrary(canonicalPath, now)
	if err != nil {
		return "", 0, vps.VPSDocument{}, "", err
	}
	stored, err := library.Upsert(candidate)
	if err != nil {
		return "", 0, vps.VPSDocument{}, "", fmt.Errorf("atomic canonical Pro-Q 3 VPS upsert: %w", err)
	}
	catalog, err := library.Catalog()
	if err != nil {
		return "", 0, vps.VPSDocument{}, "", fmt.Errorf("derive canonical Provider Catalog after promotion: %w", err)
	}
	if !proQ3PromotionCatalogContains(catalog, stored.ID, credentialID) {
		return "", 0, vps.VPSDocument{}, "", fmt.Errorf("canonical Library did not derive the promoted Pro-Q 3 equalizer.v2 Catalog entry")
	}
	return backup, len(catalog.Entries), stored, credentialID, nil
}

func proQ3PromotionCatalogContains(catalog vps.ProviderCatalog, vpsID, credentialID string) bool {
	for _, entry := range catalog.Entries {
		if entry.VPSID == vpsID && entry.CredentialID == credentialID && entry.CapabilityID == vps.EqualizerCapabilityID {
			return true
		}
	}
	return false
}

// proQ3PromotionFalseStaleRecoveryDocument is intentionally narrower than a
// general VPS edit. It accepts only the exact false stale state that could be
// produced by the pre-fix mismatch between the Adapter's raw VST3 file hash
// and the host runtime's package hash. The original stale Credential is kept
// as audit history; a fresh Credential ID is issued from the already-gated
// current candidate rather than reviving that stale Credential in place.
func proQ3PromotionFalseStaleRecoveryDocument(existing, candidate vps.VPSDocument, credentialID string, now time.Time) (vps.VPSDocument, string, error) {
	if existing.ID != candidate.ID || existing.Status != vps.VPSStatusStale || !existing.PluginIdentity.Fingerprint.Equal(candidate.PluginIdentity.Fingerprint) || !sameInstallPath(existing.PluginIdentity.InstallPath, candidate.PluginIdentity.InstallPath) {
		return vps.VPSDocument{}, "", fmt.Errorf("stored document is not the exact stale Pro-Q 3 candidate")
	}
	existingBinding, existingBound := existing.HostProjectionBindingFor(vps.VitHostProjectionID)
	candidateBinding, candidateBound := candidate.HostProjectionBindingFor(vps.VitHostProjectionID)
	if !existingBound || !candidateBound || !existingBinding.Fingerprint.Equal(candidateBinding.Fingerprint) {
		return vps.VPSDocument{}, "", fmt.Errorf("stored document has no matching observed Vit host projection")
	}
	var candidateCredential vps.ProviderCredential
	for _, credential := range candidate.ProviderCredentials {
		if credential.ID == credentialID && credential.Status == vps.CredentialVerified {
			candidateCredential = credential
			break
		}
	}
	if candidateCredential.ID == "" {
		return vps.VPSDocument{}, "", fmt.Errorf("current candidate has no verified equalizer.v2 Credential")
	}
	matchedFalseStale := false
	for _, credential := range existing.ProviderCredentials {
		if credential.ID != credentialID {
			continue
		}
		if credential.Status != vps.CredentialStale || !credential.PluginFingerprint.Equal(candidate.PluginIdentity.Fingerprint) || !strings.EqualFold(strings.TrimSpace(credential.InvalidationReason), "plugin installation fingerprint changed") {
			return vps.VPSDocument{}, "", fmt.Errorf("stored original Credential is not the expected installation false-stale record")
		}
		matchedFalseStale = true
	}
	if !matchedFalseStale {
		return vps.VPSDocument{}, "", fmt.Errorf("stored document has no expected false-stale Credential")
	}
	for _, credential := range existing.ProviderCredentials {
		if credential.CapabilityID == vps.EqualizerCapabilityID && credential.Dispatchable() {
			return vps.VPSDocument{}, "", fmt.Errorf("stored document already has a dispatchable equalizer.v2 Credential")
		}
	}
	recovery := candidateCredential
	recovery.ID = credentialID + "_requalification_r2"
	for _, credential := range existing.ProviderCredentials {
		if credential.ID == recovery.ID {
			return vps.VPSDocument{}, "", fmt.Errorf("stored document already has recovery Credential %s", recovery.ID)
		}
	}
	recovery.Revision = 1
	recovery.InvalidatedAt = nil
	recovery.InvalidationReason = ""
	recovery.IssuedAt = now
	recovery.UpdatedAt = now
	recoveryEvidence := "vpsforge:" + proQ3EqualizerV2PromotionFile + "#false_stale_requalification"
	recovery.EvidenceRefs = uniqueSorted(append(recovery.EvidenceRefs, recoveryEvidence))
	recovery.Conformance.EvidenceRefs = uniqueSorted(append(recovery.Conformance.EvidenceRefs, recoveryEvidence))
	recovered := existing
	recovered.Status = vps.VPSStatusVerified
	recovered.ProviderCredentials = append(recovered.ProviderCredentials, recovery)
	recovered.ConformanceEvidence = uniqueSorted(append(recovered.ConformanceEvidence, recoveryEvidence))
	payload, _ := json.Marshal(map[string]any{
		"stale_credential_id": credentialID, "recovery_credential_id": recovery.ID,
		"reason": "runtime package-hash mismatch was corrected to the Adapter-compatible raw file hash before requalification",
	})
	recovered.RawObservations = append(recovered.RawObservations, vps.RawObservation{
		Kind: "vpsforge_false_stale_requalification", Source: "vpsforge.pro_q_3_equalizer_v2_promotion",
		Summary: "Issued a separate reviewed Credential after correcting a false runtime installation-fingerprint mismatch; the original stale Credential remains archived.", CapturedAt: now, Data: payload,
	})
	recovered.UpdatedAt = now
	if err := recovered.Validate(); err != nil {
		return vps.VPSDocument{}, "", fmt.Errorf("validate recovered Pro-Q 3 document: %w", err)
	}
	return recovered, recovery.ID, nil
}

func proQ3PromotionBackupCanonicalLibrary(path string, now time.Time) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read canonical VPS Library for promotion backup: %w", err)
	}
	backup := path + ".pre-pro-q-3-equalizer-v2." + now.UTC().Format("20060102T150405.000000000Z") + ".bak"
	if err := os.WriteFile(backup, data, 0o600); err != nil {
		return "", fmt.Errorf("write canonical VPS Library promotion backup: %w", err)
	}
	return backup, nil
}

// proQ3PromotionPrepareStagingDeactivation returns a mutation only when the
// ordinary VPS Forge activation pointer explicitly targets this workspace's
// Pro-Q 3 staging artifact.  An unrelated staging experiment is deliberately
// ignored.  A malformed matching pointer blocks admission rather than risking
// a formal Catalog record and a live temporary bridge at the same time.
func proQ3PromotionPrepareStagingDeactivation(root string, now time.Time) (*proQ3PromotionStagingDeactivation, error) {
	stagingRoot := filepath.Dir(filepath.Dir(root))
	activationPath := filepath.Join(stagingRoot, proQ3StaticEQStagingActivationFile)
	prior, err := os.ReadFile(activationPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read VPS Forge staging activation: %w", err)
	}
	var activation map[string]json.RawMessage
	if err := json.Unmarshal(prior, &activation); err != nil {
		return nil, fmt.Errorf("decode VPS Forge staging activation: %w", err)
	}
	stringValue := func(key string) string {
		var value string
		_ = json.Unmarshal(activation[key], &value)
		return strings.TrimSpace(value)
	}
	boolValue := func(key string) (bool, bool) {
		var value bool
		if err := json.Unmarshal(activation[key], &value); err != nil {
			return false, false
		}
		return value, true
	}
	if schema := stringValue("schema_version"); schema != proQ3StaticEQStagingActivationSchema {
		return nil, fmt.Errorf("VPS Forge staging activation has unsupported schema %q", schema)
	}
	stagingPath := stringValue("staging_vps_path")
	if stagingPath == "" {
		return nil, fmt.Errorf("VPS Forge staging activation has no staging_vps_path")
	}
	if !filepath.IsAbs(stagingPath) {
		stagingPath = filepath.Join(filepath.Dir(activationPath), stagingPath)
	}
	stagingPath, err = filepath.Abs(stagingPath)
	if err != nil {
		return nil, fmt.Errorf("resolve VPS Forge staging activation target: %w", err)
	}
	matchingArtifact := filepath.Join(root, proQ3StaticEQStagingVPSFile)
	if !sameInstallPath(stagingPath, matchingArtifact) {
		return nil, nil
	}
	enabled, ok := boolValue("enabled")
	if !ok {
		return nil, fmt.Errorf("matching VPS Forge staging activation has non-boolean enabled value")
	}
	plan := &proQ3PromotionStagingDeactivation{Path: activationPath, Prior: append([]byte(nil), prior...), Next: activation, WasEnabled: enabled}
	if !enabled {
		return plan, nil
	}
	setRaw := func(key string, value any) error {
		encoded, err := json.Marshal(value)
		if err == nil {
			activation[key] = encoded
		}
		return err
	}
	if err := setRaw("enabled", false); err != nil {
		return nil, err
	}
	if err := setRaw("status", "superseded_by_local_catalog"); err != nil {
		return nil, err
	}
	if err := setRaw("updated_at", now.UTC()); err != nil {
		return nil, err
	}
	if err := setRaw("superseded_by", map[string]string{
		"kind":      "pro_q_3_equalizer_v2_local_catalog_promotion",
		"workspace": root,
		"reason":    "The matching Pro-Q 3 staging artifact has an explicitly promoted local Catalog candidate.",
	}); err != nil {
		return nil, err
	}
	return plan, nil
}

func (plan *proQ3PromotionStagingDeactivation) apply() error {
	if plan == nil || !plan.WasEnabled {
		return nil
	}
	return writeJSON(plan.Path, plan.Next)
}

func (plan *proQ3PromotionStagingDeactivation) restore() error {
	if plan == nil || !plan.WasEnabled {
		return nil
	}
	temporary := plan.Path + ".promotion-restore.tmp"
	if err := os.WriteFile(temporary, plan.Prior, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, plan.Path)
}

func proQ3PromotionPersistStagingEvidence(root string, artifact proQ3EqualizerV2PromotionArtifact, now time.Time) error {
	payload, _ := json.Marshal(map[string]any{"status": artifact.Status, "candidate_vps_id": artifact.CandidateDocument.ID, "commit": artifact.Commit})
	if err := appendEvidence(root, EvidenceEntry{Kind: "pro_q_3_equalizer_v2_promotion", Trust: map[bool]string{true: "conformed", false: "agent-inferred"}[artifact.Commit != nil], Source: "vpsforge.pro_q_3_equalizer_v2_promotion", Summary: "Prepared or committed the explicit Pro-Q 3 equalizer.v2 admission package; source evidence trust classes remain separate.", Artifact: proQ3EqualizerV2PromotionFile, CapturedAt: now, Data: payload}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["pro_q_3_equalizer_v2_promotion"] = proQ3EqualizerV2PromotionFile
		manifest.UpdatedAt = now
	})
}
